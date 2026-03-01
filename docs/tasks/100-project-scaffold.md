# Task 1.1: Project Scaffold + Makefile + Gating Hooks

## Status: DONE

## Depends On
- Phase 0 complete (all spikes validated assumptions)

## Parallelizable With
- None (all P1 tasks depend on this)

## Problem

दूतसभा needs a Go project skeleton with build tooling, linting, git hooks, gating hooks, mock providers, and CI configuration before any feature code can be written.

## PRD Reference
- §4 (Technology stack — all dependency versions)
- §5.1 (Directory structure — full layout)
- §10.2 (Make targets — full set)
- §10.3 (Testing infrastructure locations)

## Also Read
- `testing-strategy.md §1` (Mock providers for L3)
- `testing-strategy.md §3` (L4 gating hooks)
- `testing-strategy.md §7` (Session protocol — 12 steps)
- `testing-strategy.md §8` (Lefthook git hooks — lefthook.yml, idempotent `make hooks`, `make build` dependency)

## Files to Create
- `go.mod` — Module `github.com/indrasvat/dootsabha`, Go 1.26
- `cmd/dootsabha/main.go` — Entry point calling `cli.Execute()`
- `internal/cli/root.go` — Placeholder root command with bilingual help
- `internal/version/version.go` — Version/commit/date via ldflags
- `Makefile` — Full target set per §10.2
- `.golangci.yml` — golangci-lint v2 config
- `lefthook.yml` — Pre-commit (fmt+vet+go-fix-check) + pre-push (make ci) per testing-strategy.md §8.1
- `.github/workflows/ci.yml` — Lint + test on push/PR
- `.github/workflows/release.yml` — Build on tag push
- `CLAUDE.md` — Agent conventions doc (≤200 lines)
- `docs/PROGRESS.md` — Phase/task tracking
- `configs/default.yaml` — Skeleton config
- `testdata/mock-providers/mock-claude` — Mock CLI (per testing-strategy.md §1)
- `testdata/mock-providers/mock-codex` — Mock CLI (per testing-strategy.md §1)
- `testdata/mock-providers/mock-gemini` — Mock CLI (per testing-strategy.md §1)
- `scripts/test-binary.sh` — L3 smoke test stub
- `scripts/test-agent-workflow.sh` — L5 stub (exits 0)
- `scripts/hooks/pre-task-done-gate.sh` — Blocks DONE without L4 evidence
- `scripts/hooks/pre-push-visual-gate.sh` — Blocks push without screenshots
- `scripts/verify-visual-tests.sh` — L4 verification runner
- `.claude/automations/.gitkeep` — Dir for L4 scripts

## Files to Modify
- `.gitignore` — Add Go entries (bin/, dist/, coverage/, _spikes/)

## Execution Steps

### Step 1: Initialize Go module
- `go mod init github.com/indrasvat/dootsabha`
- Add deps: cobra v1.10.2, viper v1.21.0, lipgloss v1.1.0, huh v0.8.0, errgroup, retry-go, testify
- `go mod tidy`

### Step 2: Create main.go + root command
- Minimal entry point, root command with `--version` flag

### Step 3: Create Makefile (full set per §10.2)
- Build targets: `build` (depends on `hooks`), `install`, `clean`
- Test targets: `test`, `test-race`, `coverage`, `test-integration`, `test-binary`, `test-visual`, `test-agent`, `test-all`
- Lint: `lint`, `lint-fix`, `fmt`, `vet`, `fix` (`go fix ./...`)
- CI: `ci`, `ci-fast`, `check` (fmt+fix+lint+vet+test+smoke)
- Tools: `tools`, `hooks` (idempotent per §8.2), `version`, `help`
- **Critical:** `build` depends on `hooks` — every `make build` auto-installs lefthook

### Step 4: Create idempotent `make hooks` + lefthook.yml
- `make hooks`: check lefthook on PATH → install if missing → `lefthook install` → no-op if already done (per testing-strategy.md §8.2)
- `lefthook.yml` with pre-commit (gofumpt check + go vet + go fix dry-run) and pre-push (make ci) per testing-strategy.md §8.1
- Verify: `make build` triggers hooks install, `git commit` triggers pre-commit checks

### Step 5: Create mock providers + gating hooks
- Mock providers per testing-strategy.md §1 (chmod +x)
- Gating hooks per testing-strategy.md §3

### Step 6: Create CI workflows, linter config

### Step 7: Create CLAUDE.md + PROGRESS.md

## Verification

### L1: Build + lint
```bash
make ci
```

### L3: Binary execution + hooks
```bash
make build
./bin/dootsabha --help
./bin/dootsabha --version
make test-binary

# Verify hooks installed (make build should have done this)
test -f .git/hooks/pre-commit && grep -q lefthook .git/hooks/pre-commit && echo "hooks OK"

# Verify idempotent
make hooks  # Should print "Hooks already installed"
make hooks  # Same — no error, no reinstall

# Verify make check runs full suite
make check
```

## Completion Criteria

1. `make build` produces `bin/dootsabha` AND installs hooks (verify `.git/hooks/pre-commit` exists)
2. `./bin/dootsabha --help` shows bilingual help placeholder
3. `make ci` passes
4. `go mod tidy` reports no changes
5. Mock providers are executable and produce expected output
6. `make hooks` is idempotent (run twice — second run is a no-op)
7. Pre-commit hook runs gofumpt check + go vet + go fix dry-run
8. Pre-push hook runs `make ci`
9. `make check` runs full quality suite (fmt+fix+lint+vet+test+smoke)
10. PROGRESS.md updated

## Commit

```
feat(scaffold): initialize repository with build tooling and gating

- Go module with cobra, viper, lipgloss, huh, errgroup, retry-go
- Comprehensive Makefile with 20+ targets (§10.2)
- make build auto-installs lefthook hooks (idempotent)
- lefthook: pre-commit (gofumpt+vet+go-fix) + pre-push (make ci)
- Mock providers for claude/codex/gemini (L3)
- Gating hooks: pre-task-done, pre-push-visual
- golangci-lint v2, GitHub Actions CI
```

## Session Protocol

1. Read CLAUDE.md (create it in this task)
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §4, §5.1, §10.2
5. Read testing-strategy.md §1, §3, §7, §8
6. Execute steps 1-7
7. Run verification (L1 → L3)
8. **Change status to `DONE`**
9. Update `docs/PROGRESS.md`
10. Commit
