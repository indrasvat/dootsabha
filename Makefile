# दूतसभा Makefile
# Full target set per PRD §10.2

BINARY_NAME   := dootsabha
BIN_DIR       := bin
CMD_PATH      := ./cmd/dootsabha
MODULE        := github.com/indrasvat/dootsabha

VERSION       ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT        ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE          ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS       := -s -w \
                 -X $(MODULE)/internal/version.Version=$(VERSION) \
                 -X $(MODULE)/internal/version.Commit=$(COMMIT) \
                 -X $(MODULE)/internal/version.Date=$(DATE)

COLOR_RESET   := \033[0m
COLOR_BLUE    := \033[34m
COLOR_GREEN   := \033[32m
COLOR_YELLOW  := \033[33m
COLOR_RED     := \033[31m

COVERAGE_DIR  := coverage

.DEFAULT_GOAL := help

# ── Build ────────────────────────────────────────────────────────────────────

.PHONY: build
build: hooks ## Build binary to bin/dootsabha (auto-installs hooks)
	@mkdir -p $(BIN_DIR)
	@printf "$(COLOR_BLUE)>> Building $(BINARY_NAME)...$(COLOR_RESET)\n"
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME) $(CMD_PATH)
	@printf "$(COLOR_GREEN)>> Built: $(BIN_DIR)/$(BINARY_NAME)$(COLOR_RESET)\n"

.PHONY: install
install: ## Install binary to GOPATH/bin
	go install -ldflags "$(LDFLAGS)" $(CMD_PATH)

.PHONY: clean
clean: ## Remove build artifacts
	@rm -rf $(BIN_DIR) $(COVERAGE_DIR)
	@printf "$(COLOR_GREEN)>> Cleaned$(COLOR_RESET)\n"

# ── Test ─────────────────────────────────────────────────────────────────────

.PHONY: test
test: build-mock-plugins ## Run unit tests (excludes _spikes)
	go test $(GO_DIRS)

.PHONY: test-race
test-race: ## Run unit tests with race detector (excludes _spikes)
	go test -race -shuffle=on $(GO_DIRS)

.PHONY: coverage
coverage: ## Generate coverage report
	@mkdir -p $(COVERAGE_DIR)
	go test -coverprofile=$(COVERAGE_DIR)/coverage.out $(GO_DIRS)
	go tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@printf "$(COLOR_GREEN)>> Coverage report: $(COVERAGE_DIR)/coverage.html$(COLOR_RESET)\n"

.PHONY: test-integration
test-integration: ## Run integration tests
	go test -tags=integration ./...

.PHONY: test-binary
test-binary: build ## L3: Binary smoke tests (mock providers)
	@printf "$(COLOR_BLUE)>> Running L3 smoke tests...$(COLOR_RESET)\n"
	@bash scripts/test-binary.sh

.PHONY: test-plugins
test-plugins: build-mock-plugins build-plugins ## L3: Plugin smoke tests (mock + provider plugins)
	@printf "$(COLOR_BLUE)>> Running plugin smoke tests...$(COLOR_RESET)\n"
	@bash scripts/test-plugin-smoke.sh

.PHONY: test-visual
test-visual: ## L4: Visual integration tests via iTerm2-driver
	@printf "$(COLOR_BLUE)>> Running L4 visual tests...$(COLOR_RESET)\n"
	@bash scripts/verify-visual-tests.sh

.PHONY: test-agent
test-agent: build ## L5: Agent workflow tests (real CLIs)
	@printf "$(COLOR_BLUE)>> Running L5 agent workflow tests...$(COLOR_RESET)\n"
	@bash scripts/test-agent-workflow.sh

.PHONY: test-all
test-all: test test-race test-binary test-visual test-agent ## Run all test layers

# ── Lint & Format ─────────────────────────────────────────────────────────────

.PHONY: lint
lint: ## Run golangci-lint (excludes _spikes)
	golangci-lint run $(GO_DIRS)

.PHONY: lint-fix
lint-fix: ## Run golangci-lint with auto-fix (excludes _spikes)
	golangci-lint run --fix $(GO_DIRS)

# Directories to format/check (excludes _spikes — separate modules)
GO_DIRS := ./cmd/... ./internal/... ./proto/... ./plugins/...

.PHONY: fmt
fmt: ## Format code with gofumpt (excludes _spikes)
	gofumpt -w $(shell go list -f '{{.Dir}}' $(GO_DIRS))

.PHONY: fmt-check
fmt-check: ## Check formatting (non-destructive, excludes _spikes)
	@if ! command -v gofumpt >/dev/null 2>&1; then \
		printf "$(COLOR_YELLOW)>> gofumpt not found, installing...$(COLOR_RESET)\n"; \
		go install mvdan.cc/gofumpt@latest; \
	fi
	@UNFORMATTED=$$(gofumpt -l $(shell go list -f '{{.Dir}}' $(GO_DIRS))); \
	if [ -n "$$UNFORMATTED" ]; then \
		printf "$(COLOR_RED)>> Formatting issues in:$(COLOR_RESET)\n$$UNFORMATTED\n"; \
		exit 1; \
	fi
	@printf "$(COLOR_GREEN)>> Formatting OK$(COLOR_RESET)\n"

.PHONY: vet
vet: ## Run go vet (excludes _spikes)
	go vet $(GO_DIRS)

.PHONY: fix
fix: ## Run go fix (applies API migrations, excludes _spikes)
	go fix $(GO_DIRS)

.PHONY: fix-check
fix-check: ## Run go fix and fail if any files changed (pre-commit check, excludes _spikes)
	@go fix $(GO_DIRS)
	@if ! git diff --quiet -- '*.go' 2>/dev/null; then \
		printf "$(COLOR_YELLOW)>> go fix changed files — re-stage and retry:$(COLOR_RESET)\n"; \
		git diff --name-only -- '*.go'; \
		exit 1; \
	fi
	@printf "$(COLOR_GREEN)>> go fix: no changes needed$(COLOR_RESET)\n"

# ── Dependencies ─────────────────────────────────────────────────────────────

.PHONY: tidy
tidy: ## Run go mod tidy
	go mod tidy

.PHONY: verify
verify: ## Run go mod verify
	go mod verify

# ── CI ───────────────────────────────────────────────────────────────────────

.PHONY: pre-commit
pre-commit: fmt-check vet fix-check ## Fast pre-commit checks (<3s): fmt+vet+fix-check

.PHONY: ci
ci: lint vet test ## Full CI gate: lint+vet+test (<30s)

.PHONY: ci-fast
ci-fast: fmt vet test ## Fast CI: fmt+vet+test

.PHONY: check
check: fmt-check fix-check lint vet test test-binary ## Full quality suite: fmt+fix+lint+vet+test+smoke

# ── Extensions ───────────────────────────────────────────────────────────────

EXTENSION_DIR := examples/extensions
LOCAL_BIN     := $(HOME)/.local/bin

.PHONY: install-extensions
install-extensions: ## Symlink extensions from examples/ into ~/.local/bin
	@mkdir -p $(LOCAL_BIN)
	@for ext in $(EXTENSION_DIR)/dootsabha-*; do \
		name=$$(basename "$$ext"); \
		ln -sf "$$(pwd)/$$ext" "$(LOCAL_BIN)/$$name"; \
		printf "$(COLOR_GREEN)>> Linked: $(LOCAL_BIN)/$$name$(COLOR_RESET)\n"; \
	done

# ── Provider Plugins ─────────────────────────────────────────────────────────

PLUGIN_BIN := plugins/bin

.PHONY: build-plugins
build-plugins: ## Build plugin binaries (providers + strategy)
	@mkdir -p $(PLUGIN_BIN)
	@printf "$(COLOR_BLUE)>> Building plugins...$(COLOR_RESET)\n"
	go build -o $(PLUGIN_BIN)/claude-provider ./plugins/claude
	go build -o $(PLUGIN_BIN)/codex-provider ./plugins/codex
	go build -o $(PLUGIN_BIN)/gemini-provider ./plugins/gemini
	go build -o $(PLUGIN_BIN)/council-strategy ./plugins/council-strategy
	@printf "$(COLOR_GREEN)>> Plugins built$(COLOR_RESET)\n"

# ── Mock Plugins ─────────────────────────────────────────────────────────────

MOCK_PLUGIN_DIR := testdata/mock-plugins
MOCK_PLUGIN_BIN := $(MOCK_PLUGIN_DIR)/bin

.PHONY: build-mock-plugins
build-mock-plugins: ## Build mock plugin binaries for integration tests
	@mkdir -p $(MOCK_PLUGIN_BIN)
	@printf "$(COLOR_BLUE)>> Building mock plugins...$(COLOR_RESET)\n"
	go build -o $(MOCK_PLUGIN_BIN)/mock-provider ./$(MOCK_PLUGIN_DIR)/mock-provider
	go build -o $(MOCK_PLUGIN_BIN)/mock-strategy ./$(MOCK_PLUGIN_DIR)/mock-strategy
	go build -o $(MOCK_PLUGIN_BIN)/mock-hook ./$(MOCK_PLUGIN_DIR)/mock-hook
	@printf "$(COLOR_GREEN)>> Mock plugins built$(COLOR_RESET)\n"

# ── Proto ─────────────────────────────────────────────────────────────────────

.PHONY: proto
proto: ## Regenerate protobuf Go code from .proto files
	@printf "$(COLOR_BLUE)>> Generating protobuf Go code...$(COLOR_RESET)\n"
	protoc --go_out=. --go_opt=module=$(MODULE) \
	       --go-grpc_out=. --go-grpc_opt=module=$(MODULE) \
	       proto/provider.proto proto/strategy.proto proto/hook.proto
	@printf "$(COLOR_GREEN)>> Proto generation complete$(COLOR_RESET)\n"

# ── Tools ─────────────────────────────────────────────────────────────────────

.PHONY: tools
tools: ## Install dev tools (golangci-lint, gofumpt, lefthook)
	@printf "$(COLOR_BLUE)>> Installing golangci-lint...$(COLOR_RESET)\n"
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v2.9.0
	@printf "$(COLOR_BLUE)>> Installing gofumpt...$(COLOR_RESET)\n"
	go install mvdan.cc/gofumpt@latest
	@printf "$(COLOR_BLUE)>> Installing lefthook...$(COLOR_RESET)\n"
	go install github.com/evilmartians/lefthook@latest
	@printf "$(COLOR_GREEN)>> Tools installed$(COLOR_RESET)\n"

.PHONY: hooks
hooks: ## Install git hooks via lefthook (idempotent, safe to call repeatedly)
	@if ! command -v lefthook >/dev/null 2>&1; then \
		printf "$(COLOR_BLUE)>> Installing lefthook...$(COLOR_RESET)\n"; \
		go install github.com/evilmartians/lefthook@latest; \
	fi
	@if [ ! -f .git/hooks/pre-commit ] || ! grep -q lefthook .git/hooks/pre-commit 2>/dev/null; then \
		printf "$(COLOR_BLUE)>> Installing git hooks...$(COLOR_RESET)\n"; \
		lefthook install; \
	else \
		printf "$(COLOR_GREEN)>> Hooks already installed$(COLOR_RESET)\n"; \
	fi

.PHONY: version
version: ## Show version info
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT)"
	@echo "Date:    $(DATE)"

# ── Help ──────────────────────────────────────────────────────────────────────

.PHONY: help
help: ## Show this help
	@printf "$(COLOR_BLUE)दूतसभा — AI Council Orchestrator$(COLOR_RESET)\n\n"
	@printf "$(COLOR_YELLOW)Usage:$(COLOR_RESET) make [target]\n\n"
	@awk 'BEGIN {FS = ":.*##"; printf ""} /^[a-zA-Z_-]+:.*?##/ { printf "  $(COLOR_GREEN)%-20s$(COLOR_RESET) %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
