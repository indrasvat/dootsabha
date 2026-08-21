# दूतसभा — Testing Strategy

> **Authoritative reference for all testing implementation details.**
> PRD §10 contains the summary; this document has the full specs.
> Task files reference specific sections as `testing-strategy.md §N`.

---

## §1 Mock Providers for L3

Mock providers are tiny bash scripts that simulate CLI behavior for offline testing. One per provider, placed in `testdata/mock-providers/`:

**`testdata/mock-providers/mock-claude`:**
```bash
#!/usr/bin/env bash
# Simulates claude CLI for smoke tests — no API calls
set -euo pipefail
PROMPT="" FORMAT="" MODEL="claude-opus-4-8" ERROR=""
while [[ $# -gt 0 ]]; do
  case $1 in
    -p) PROMPT="$2"; shift 2 ;;
    --output-format) FORMAT="$2"; shift 2 ;;
    --model) MODEL="$2"; shift 2 ;;
    --dangerously-skip-permissions) shift ;;
    --error) ERROR="$2"; shift 2 ;;  # test hook: force error
    *) PROMPT="${PROMPT:-$1}"; shift ;;
  esac
done
[[ -n "$ERROR" ]] && { echo "Error: $ERROR" >&2; exit 3; }
if [ "$FORMAT" = "json" ]; then
  echo '{"result":"Mock: '"$PROMPT"'","session_id":"mock_123","cost_usd":0.001,"model":"'"$MODEL"'","duration_ms":150}'
else
  echo "Mock response to: $PROMPT"
fi
```

**`testdata/mock-providers/mock-codex`:** (emits JSONL event stream)
```bash
#!/usr/bin/env bash
set -euo pipefail
PROMPT=""
while [[ $# -gt 0 ]]; do
  case $1 in
    exec) shift ;;
    --json) shift ;;
    --sandbox) shift 2 ;;
    --skip-git-repo-check) shift ;;
    *) PROMPT="${PROMPT:-$1}"; shift ;;
  esac
done
echo '{"type":"thread.started","thread_id":"mock-thread-1"}'
echo '{"type":"turn.started"}'
echo '{"type":"item.completed","item":{"id":"item_0","type":"agent_message","text":"Mock: '"$PROMPT"'"}}'
echo '{"type":"turn.completed","usage":{"input_tokens":100,"output_tokens":50}}'
```

**`testdata/mock-providers/mock-agy`:** (JSON envelope; echoes the model it was given)
```bash
#!/usr/bin/env bash
# Simulates agy (Antigravity CLI) for smoke tests — no API calls
set -euo pipefail
PROMPT=""; FORMAT="text"; MODEL="unset"   # sentinel: a dropped --model must fail loudly
while [[ $# -gt 0 ]]; do
  case $1 in
    --model) MODEL="$2"; shift 2 ;;
    --output-format) FORMAT="$2"; shift 2 ;;
    -p|--print|--prompt) PROMPT="$2"; shift 2 ;;
    --dangerously-skip-permissions) shift ;;
    *) PROMPT="${PROMPT:-$1}"; shift ;;
  esac
done
# The model is echoed into the body — agy's envelope has no model field, so the
# response text is the only channel an integration test can assert forwarding on.
```

**`testdata/mock-providers/mock-grok`:** (streaming-messages-json NDJSON)
```bash
#!/usr/bin/env bash
# Simulates the xAI Grok CLI — emits the NDJSON stream, not a single object.
set -euo pipefail
PROMPT=""
MODEL="unset"        # SENTINEL, not the shipped default — see below
while [[ $# -gt 0 ]]; do
  case $1 in
    --version) echo "grok 1.0.5 (mock) [stable]"; exit 0 ;;
    -p|--single) PROMPT="$2"; shift 2 ;;
    -m|--model) MODEL="$2"; shift 2 ;;   # captured, not discarded — see below
    --output-format|--reasoning-effort|--effort) shift 2 ;;
    --sandbox|--permission-mode|--max-turns|--cwd|--prompt-file) shift 2 ;;
    --always-approve|--no-plan|--no-subagents|--no-auto-update) shift ;;
    *) PROMPT="${PROMPT:-$1}"; shift ;;
  esac
done
# Emits a system line, an assistant preamble block that is NOT the answer, then
# the final result event. The preamble proves dootsabha reads `result` rather
# than concatenating assistant text (grok's `--output-format json` .text trap).
```

`mock-grok` **echoes the model it was handed**: `-m` is captured into `MODEL`
and emitted both in the system-init line and as the `modelUsage` key
`"<model>-build"`, mirroring the real CLI's backend-id convention. That is what
lets L3/L5 prove दूतसभा actually forwards the configured model, instead of
asserting a hardcoded string against itself. `MODEL` defaults to the sentinel
`unset` rather than the shipped default, so a regression that dropped `-m`
entirely fails loudly instead of echoing the right answer by accident.


> **Why the preamble block matters:** grok's `--output-format json` merges every
> assistant text block into `.text` with no separator, so tool-call preambles leak
> into the answer. dootsabha uses `--output-format streaming-messages-json` and
> reads the last `type=="result"` line. `mock-grok` reproduces that shape so the
> L3 smoke test would catch a regression back to `.text` parsing.

Mock providers are activated via config override: `DOOTSABHA_CLAUDE_BIN=testdata/mock-providers/mock-claude` etc.

---

## §2 shux Automation (L4 Visual Verification)

> Unit tests cannot verify terminal rendering. Only frames prove visual correctness.

L4 is [shux](https://github.com/indrasvat/shux): it rasterises a real PTY to PNG
headless — no terminal emulator, no display server, no window focus to lose. The
iTerm2-driver suite it replaced was removed in `chore/claude-md-refresh`; recover
it from git history if ever needed.

### §2.1 Canonical capture script

Capture scripts are committed to `.shux/scripts/<topic>-evidence.sh`; frames go to
`.shux/out/<task>/`, which is gitignored and attached to the PR instead. Copy
`.shux/scripts/agy-3-7-evidence.sh` — it is the reference implementation. Its
shape:

```bash
export XDG_RUNTIME_DIR=/tmp/shux<task>     # private socket dir, short path
trap cleanup EXIT                          # kill every session, then daemon stop

frame() {                                  # name, max-seconds, rows, command
  shux session create "$sess" -d --cwd "$REPO" \
    -- bash -lc "cd '$REPO' && clear && { $cmd; }; touch '$DONE_DIR/$name'; sleep 3600"
  shux pane set-size -s "$sess" --cols 100 --rows "$rows"
  wait_done "$name" "$max"                 # poll a FILE, never an on-screen marker
  shux pane snapshot -s "$sess" -o "$OUT/$name.png"
  shux session kill "$sess"
}
```

### §2.2 Rules that came from getting these wrong

- **Signal completion with a file**, not `echo __DONE__` — an on-screen marker is
  captured into the evidence itself.
- **Size rows per frame.** A six-line result framed in 24 blank rows reads as sloppy.
- **Read every PNG back** before attaching it. This is how a `$HOME` path leak and
  an incorrect code comment were both caught after the fact.
- **Never edit a script while it is running** — bash re-reads it by byte offset and
  dies mid-run, losing frames.
- **Stop every daemon you start:** `XDG_RUNTIME_DIR=<dir> shux daemon stop`, once
  per runtime dir. Killing sessions does not stop the daemon.
- Motion evidence: snapshot the live pane on a cadence and stitch with `ffmpeg` —
  shux has no video primitive by design. See `.shux/scripts/agy-3-7-video.sh`.

### §2.3 Cloud sessions

A cloud session has no provider CLIs installed or authenticated, so L4 cannot be
captured there. Record `N/A` in the task file **with the reason**; the gate in §3
accepts that and rejects a bare `N/A`. Never fabricate evidence.

## §3 Enforcement Layers

> A rule only holds if it lives in a layer that can enforce it. Put it in the
> wrong one and it fails silently — as this section's predecessor did.

### §3.1 Which layer enforces what

| Layer | Answers | Fails loudly? |
|---|---|---|
| **Hook** (`.claude/hooks/`) | "is this action about to do something hard to undo?" | only when it fires |
| **Test** (`make check`, CI) | "is this true of the repo right now?" | yes — every push |
| **CLAUDE.md** | anything requiring judgment | no — guidance only |

State questions belong in tests. `internal/cli/outcome_test.go` is the model: it
fails the build if a command mints its own exit code.

### §3.2 What the hooks do

Wired repo-scoped in `.claude/settings.json`. All three **fail open** — anything
they cannot determine (no `git`, no `python3`, detached HEAD, a shallow cloud
checkout with no `origin/HEAD`) is allowed through. Missing a violation is
cheaper than blocking legitimate work; see §3.4.

| Hook | Event | Blocks? |
|---|---|---|
| `branch-guard.sh` | `PreToolUse(Edit\|Write)` | yes — editing a **tracked** file on the default branch. Untracked files pass. |
| `staging-guard.sh` | `PreToolUse(Bash)` | yes — `git add -A/./-u`, `git commit -a`. `git` must be the command, so `echo git add -A` passes. |
| `session-hygiene.sh` | `Stop` | **never.** Warns about leftover shux daemons and a debug browser bridge. |

### §3.3 What the test does

`internal/tasks/evidence_test.go`: a task marked DONE must carry a
`## Visual Test Results` section naming a `.shux/scripts/*.sh` that exists — or a
reasoned `N/A` (a cloud session has no provider CLIs; some tasks render nothing).
A bare `N/A` fails. The section is bounded by the next `##` heading, so a later
section cannot stand in as evidence.

Tasks predating this check are listed in `grandfathered`. That list may only
shrink, and a stale entry fails its own test. Retrofitting evidence onto work
whose frames were deleted would mean inventing it.

**The check has fixtures asserting its FAILURE path.** That is the difference
between a gate and the appearance of one — see §3.4.

### §3.4 Why the previous version is worth remembering

`scripts/verify-visual-tests.sh` was called *"the single most important mechanism
for preventing agent hallucinations."* It never ran. Three independent reasons:

1. Registered nowhere — no `.claude/settings.json` existed.
2. It read `$1/$2/$3`; a `PreToolUse` hook receives **JSON on stdin**. Wired as
   written, it would have exited 0 on everything.
3. Its checks used `grep -oP`, unsupported by BSD grep, behind a `|| true`.

Two lessons are now structural. **A gate needs a test of its own failure path**,
or it reports success while checking nothing. And **a gate must not misfire**: the
old pre-push hook gated every push on any IN PROGRESS task, and when it blocked
unrelated work, the agent flipped two tasks to DONE to satisfy it — violating the
very rule it protected. A gate that cries wolf does not stop an agent; it teaches
the agent to route around it.

**Evidence quality is deliberately not gated.** A check can confirm a capture
script exists; it cannot confirm anyone looked at the frames. Honesty is what
review is for — during #27–#29 दूतसभा and Codex caught four factual errors and a
bug inside a fix. No gate could have.


## §4 L5 Agent Workflow Tests

Tests that validate दूतसभा is consumable by other AI agents:

```bash
#!/usr/bin/env bash
# scripts/test-agent-workflow.sh
set -euo pipefail

BINARY="bin/dootsabha"
PASS=0 FAIL=0

run_test() {
  local name="$1" cmd="$2" check="$3"
  if eval "$check"; then
    printf "  ✓ %s\n" "$name"; ((PASS++))
  else
    printf "  ✗ %s\n" "$name"; ((FAIL++))
  fi
}

# Workflow 1: JSON output is valid and parseable
run_test "consult JSON valid" \
  "$BINARY consult --json 'PONG'" \
  "$BINARY consult --json 'PONG' | python3 -m json.tool >/dev/null 2>&1"

# Workflow 2: Exit codes reflect state
run_test "consult success exit 0" \
  "" \
  "$BINARY consult 'PONG' >/dev/null 2>&1; [ \$? -eq 0 ]"

# Workflow 3: No ANSI in piped output
run_test "consult no ANSI when piped" \
  "" \
  "! $BINARY consult 'PONG' | grep -qP '\x1b\['"

# Workflow 4: JSON fields exist
run_test "consult JSON has required fields" \
  "" \
  "$BINARY consult --json 'PONG' | python3 -c \"import json,sys; d=json.load(sys.stdin); assert 'content' in d and 'meta' in d\""

# Workflow 5: Status JSON is valid
run_test "status JSON valid" \
  "" \
  "$BINARY status --json | python3 -m json.tool >/dev/null 2>&1"

# Workflow 6: Error produces structured JSON
run_test "error produces JSON with exit 3" \
  "" \
  "$BINARY consult --json --agent nonexistent 'test'; [ \$? -eq 3 ]"

# Workflow 7: Performance (<2s startup)
run_test "startup under 2s" \
  "" \
  "timeout 2 $BINARY --version >/dev/null 2>&1"

printf "\nResults: %d passed, %d failed\n" "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
```

---

## §5 Anti-Hallucination Rules

> These rules exist because agents WILL try to skip verification. Every rule here was learned from real failures in gh-ghent.

1. **NEVER claim a task is DONE without showing actual terminal output.** Terminal output is proof. Assertions are not proof.
2. **Screenshots are mandatory for any output-visible change.** If a human would look at the terminal to verify, you need a screenshot.
3. **`make ci` MUST pass before marking any task DONE.** No exceptions.
4. **Every task file MUST have a `## Visual Test Results` section** with:
   - L4 script name and pass/fail count
   - Each screenshot reviewed with specific observations
   - Any findings or deviations noted
5. **Every phase must show:** (a) help output, (b) command output, (c) JSON piped to `jq`, (d) piped through `cat` (no ANSI).
6. **L4 tests run against REAL CLIs** with tiny prompts ("PONG") to minimize cost. Never mock at L4.
7. **`make check` before every commit:** `gofumpt` + `go vet` + `golangci-lint` + `go test` + smoke. `go fix` runs only during Go toolchain migrations.
8. **Pre-push hook blocks** if any IN PROGRESS task fails L4 gate.
9. **Pre-task-done gate blocks** if task status changes to DONE without L4 evidence.
10. **Mock providers for L2/L3 only.** L4 and L5 use real CLIs. Token cost is controlled via tiny prompts.

---

## §6 Task File Verification Checklist

Every task file in `docs/tasks/` MUST include these two sections. This is a hard requirement — `internal/tasks/evidence_test.go` fails `make check` without them (§3.3).

**Section 1: `## Verification`** — must contain ALL applicable levels:

| Level | Required Content | Example |
|-------|-----------------|---------|
| **L1** | `make test` — expected: all pass | Always required |
| **L2** | `make test-integration` — expected: all pass | If integration tests exist |
| **L3** | `make build` + actual binary commands with expected output + `--json \| jq .` + `\| cat` (no ANSI) | Always required |
| **L4** | `.shux/scripts/<topic>-evidence.sh` + what each frame shows | Required for any output-visible change |
| **L5** | `make test-agent` | Required for commands with `--json` output |

**Section 2: `## Visual Test Results`** — must contain actual evidence (not a placeholder):

| Field | Required? | Description |
|-------|-----------|-------------|
| Capture script | YES | `.shux/scripts/<topic>-evidence.sh` — must exist; the gate checks it |
| Frame table | YES | One row per frame: filename + what it proves |
| Findings | YES | Deviations, learnings, or "No issues found" |

A cloud session has no provider CLIs and cannot capture L4. Record
`_N/A — <reason>_` instead; the gate accepts a reasoned N/A and rejects a bare one.

**Minimum content:** it must name a `.shux/scripts/*.sh` that exists, or give a reasoned `N/A`. Enforced by `internal/tasks/evidence_test.go` in `make check`; the section ends at the next `##` heading, so padding from a later section does not count (§3.3).

---

## §7 Session Protocol (Per-Task Execution)

Agents MUST follow this protocol for every task:

```
 1. Read CLAUDE.md (conventions, build commands, pitfalls)
 2. Read this task file
 3. Change task status to IN PROGRESS
 4. Read referenced PRD sections (§X.Y)
 5. Read referenced research docs
 6. Execute implementation steps
 7. Run verification ladder (L1 → L2 → L3 → L4 → L5)
 8. Fill in Visual Test Results section with evidence
 9. Change task status to DONE
10. Update docs/PROGRESS.md — mark task done + session notes
11. Update CLAUDE.md Learnings section if new insights
12. Commit with prescribed message
```

**Hard rules:**
- Step 7 CANNOT be skipped — L4 is mandatory for any visible output change
- Step 8 CANNOT be skipped — empty Visual Test Results = task is NOT done
- If any L-level fails, task stays IN PROGRESS with failure details noted
- Agent MUST run `cm context "<task description>" --json` at step 1 to pull relevant playbook rules

---

## §8 Git Hooks via Lefthook (Code Quality Gates)

> Git hooks run *your code* through automated checks before it leaves the machine. They are one of the three layers in §3: lefthook catches code issues at commit/push, the Claude Code hooks catch actions in flight, and the tests catch repo state.

### §8.1 `lefthook.yml` Specification

```yaml
# lefthook.yml — Git hooks for code quality
# pre-commit: fast checks (<3s) via make pre-commit
# pre-push: full CI (<30s) via make ci

pre-commit:
  commands:
    pre-commit:
      run: make pre-commit
      fail_text: "Pre-commit checks failed! Run 'make pre-commit' to see details."

pre-push:
  commands:
    ci:
      run: make ci
      fail_text: "CI failed! Fix issues before pushing."
```

**Design rationale:**
- **lefthook.yml is minimal** — delegates all logic to Makefile targets. One entry point per hook, no inline shell.
- **`make pre-commit`** (<3s): `gofumpt` check (not write) + `go vet` + `go fix` dry-run. Fast enough for every commit. Doesn't lint (too slow for commit frequency).
- **`make ci`** (<30s): lint + test + vet. The full quality gate before code leaves the machine.
- **All commands live in the Makefile** — visible via `make help`, runnable standalone, testable in CI. Lefthook is just the trigger mechanism.

### §8.2 Idempotent `make hooks` Target

```makefile
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
```

**Key properties:**
1. **Idempotent**: safe to call on every `make build` — no-ops if hooks already installed
2. **Auto-installs lefthook**: if not on PATH, installs via `go install`
3. **Checks actual hook content**: verifies the hook file exists AND contains lefthook (not just that a file exists)
4. **Never fails the build**: if lefthook install fails (e.g., CI without git), the build continues

### §8.3 `make build` Depends on `make hooks`

```makefile
.PHONY: build
build: hooks ## Build binary (auto-installs hooks)
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/dootsabha
```

**Why this matters:** Any agent that runs `make build` — even if it has never set up the repo — gets hooks installed automatically. This eliminates the "forgot to install hooks" failure mode. The dependency is visible in the Makefile, so agents can see it in `make help`.

### §8.4 `make pre-commit`, `make fix`, `make check`

```makefile
.PHONY: pre-commit
pre-commit: fmt-check vet fix-check ## Fast pre-commit gate (<3s): format + vet + go fix dry-run
	@printf "$(COLOR_GREEN)>> Pre-commit passed$(COLOR_RESET)\n"

.PHONY: fmt-check
fmt-check: ## Check formatting without writing (gofumpt -l -d)
	@printf "$(COLOR_BLUE)>> Checking format...$(COLOR_RESET)\n"
	@UNFORMATTED=$$(gofumpt -l . 2>/dev/null); \
	if [ -n "$$UNFORMATTED" ]; then \
		printf "$(COLOR_RED)>> Unformatted files:$(COLOR_RESET)\n$$UNFORMATTED\n"; \
		printf "Run: make fmt\n"; \
		exit 1; \
	fi
	@printf "$(COLOR_GREEN)>> Format OK$(COLOR_RESET)\n"

.PHONY: fix
fix: ## Run go fix ./... (applies changes)
	@printf "$(COLOR_BLUE)>> Running go fix...$(COLOR_RESET)\n"
	go fix ./...
	@printf "$(COLOR_GREEN)>> go fix complete$(COLOR_RESET)\n"

.PHONY: fix-check
fix-check: ## Check if go fix would change anything (dry-run, no writes)
	@printf "$(COLOR_BLUE)>> Checking go fix...$(COLOR_RESET)\n"
	@TMPDIR=$$(mktemp -d) && \
	cp -r . "$$TMPDIR/src" 2>/dev/null && \
	cd "$$TMPDIR/src" && go fix ./... 2>/dev/null && \
	if ! diff -rq "$$TMPDIR/src" . --exclude=.git --exclude=bin --exclude=coverage >/dev/null 2>&1; then \
		rm -rf "$$TMPDIR"; \
		printf "$(COLOR_RED)>> go fix has pending changes. Run: make fix$(COLOR_RESET)\n"; \
		exit 1; \
	fi; \
	rm -rf "$$TMPDIR"
	@printf "$(COLOR_GREEN)>> go fix OK$(COLOR_RESET)\n"

.PHONY: check
check: fmt vet fix lint test test-binary ## Full quality suite (pre-commit + CI + smoke)
	@printf "$(COLOR_GREEN)>> All checks passed$(COLOR_RESET)\n"
```

**Target hierarchy:**
- `make pre-commit` — fast gate called by lefthook pre-commit hook: `fmt-check` + `vet` + `fix-check` (<3s)
- `make ci` — standard CI gate called by lefthook pre-push hook: `lint` + `test` + `vet` (<30s)
- `make check` — belt AND suspenders: `fmt` + `vet` + `fix` + `lint` + `test` + `test-binary`. Run manually when you want everything.

### §8.5 Two-Layer Quality System

| Layer | Mechanism | What It Catches | When |
|-------|-----------|----------------|------|
| **Git hooks (lefthook)** | pre-commit, pre-push | Format, vet, go fix, lint, tests | Every commit/push |
| **Claude Code hooks (§3.2)** | PreToolUse on Edit/Write/Bash | Editing a tracked file on the default branch; bulk staging | Before the action runs |
| **Repo-state tests (§3.3)** | `make check`, CI | A DONE task with no evidence; a command minting its own exit code | Every push |

The three are independent and catch different things: lefthook catches code that
does not build, the hooks catch an action about to happen, and the tests catch the
state of the repo — including in CI, where the hooks do not run at all.
