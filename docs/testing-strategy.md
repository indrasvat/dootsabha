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
PROMPT="" FORMAT="" MODEL="sonnet-4-6" ERROR=""
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

**`testdata/mock-providers/mock-gemini`:**
```bash
#!/usr/bin/env bash
set -euo pipefail
PROMPT="" FORMAT=""
while [[ $# -gt 0 ]]; do
  case $1 in
    --yolo) shift ;;
    -p|--prompt) PROMPT="$2"; shift 2 ;;
    --output-format) FORMAT="$2"; shift 2 ;;
    *) PROMPT="${PROMPT:-$1}"; shift ;;
  esac
done
if [ "$FORMAT" = "json" ]; then
  echo '{"result":"Mock: '"$PROMPT"'","model":"gemini-3-pro","duration_ms":120}'
else
  echo "Mock response to: $PROMPT"
fi
```

Mock providers are activated via config override: `DOOTSABHA_CLAUDE_BIN=testdata/mock-providers/mock-claude` etc.

---

## §2 iTerm2-driver Automation (L4 Visual Verification)

> Unit tests cannot verify terminal rendering. Only screenshots prove visual correctness.

### §2.1 Canonical Script Template

All iTerm2-driver scripts live in `.claude/automations/` and follow this exact template:

```python
# /// script
# requires-python = ">=3.14"
# dependencies = ["iterm2", "pyobjc", "pyobjc-framework-Quartz"]
# ///
"""
L4 Visual Test: dootsabha {command}
Tests: {list of numbered tests}
Screenshots: {list of expected screenshot names}
"""
import asyncio, iterm2, subprocess, time, os, sys
from datetime import datetime

# ─── Result Tracking ────────────────────────────────────────────
results = {
    "passed": 0, "failed": 0, "unverified": 0,
    "tests": [],
    "screenshots": [],
    "start_time": None, "end_time": None,
}

def log_result(test_name: str, status: str, details: str = "", screenshot: str = None):
    """status: PASS, FAIL, UNVERIFIED"""
    results["tests"].append({
        "name": test_name, "status": status,
        "details": details, "screenshot": screenshot,
    })
    results[{"PASS": "passed", "FAIL": "failed", "UNVERIFIED": "unverified"}[status]] += 1
    icon = {"PASS": "✓", "FAIL": "✗", "UNVERIFIED": "?"}[status]
    print(f"  {icon} {test_name}: {details}")

# ─── Screenshot Capture ─────────────────────────────────────────
SCREENSHOT_DIR = os.path.join(os.path.dirname(__file__), "..", "screenshots")

def get_iterm2_window_id():
    import Quartz
    windows = Quartz.CGWindowListCopyWindowInfo(
        Quartz.kCGWindowListOptionOnScreenOnly, Quartz.kCGNullWindowID
    )
    for w in windows:
        if w.get("kCGWindowOwnerName") == "iTerm2":
            return w.get("kCGWindowNumber")
    return None

def capture_screenshot(name: str) -> str:
    os.makedirs(SCREENSHOT_DIR, exist_ok=True)
    ts = datetime.now().strftime("%Y%m%d_%H%M%S")
    filepath = os.path.join(SCREENSHOT_DIR, f"{name}_{ts}.png")
    wid = get_iterm2_window_id()
    if wid:
        subprocess.run(["screencapture", "-x", "-l", str(wid), filepath], check=True)
    else:
        subprocess.run(["screencapture", "-x", filepath], check=True)
    results["screenshots"].append(filepath)
    return filepath

# ─── Screen Verification ────────────────────────────────────────
async def verify_screen_contains(session, expected: str, description: str, timeout: float = 10.0) -> bool:
    """Poll screen content until expected text appears or timeout."""
    start = time.monotonic()
    while (time.monotonic() - start) < timeout:
        screen = await session.async_get_screen_contents()
        for i in range(screen.number_of_lines):
            if expected in screen.line(i).string:
                return True
        await asyncio.sleep(0.3)
    return False

async def get_all_screen_text(session) -> list[str]:
    """Return all non-empty screen lines."""
    screen = await session.async_get_screen_contents()
    return [screen.line(i).string for i in range(screen.number_of_lines) if screen.line(i).string.strip()]

async def dump_screen(session, label: str):
    """Debug: print all screen lines with line numbers."""
    lines = await get_all_screen_text(session)
    print(f"\n--- SCREEN DUMP: {label} ---")
    for i, line in enumerate(lines):
        print(f"  {i:3d} | {line}")
    print(f"--- END DUMP ---\n")

# ─── Cleanup ────────────────────────────────────────────────────
async def cleanup_session(session):
    """Exit cleanly: Ctrl+C, then q, then wait."""
    try:
        await session.async_send_text("\x03")  # Ctrl+C
        await asyncio.sleep(0.5)
        await session.async_send_text("q")
        await asyncio.sleep(0.5)
    except Exception:
        pass

# ─── Summary ────────────────────────────────────────────────────
def print_summary() -> int:
    results["end_time"] = datetime.now().isoformat()
    total = results["passed"] + results["failed"] + results["unverified"]
    print(f"\n{'='*60}")
    print(f"Results: {results['passed']}/{total} PASS, {results['failed']} FAIL, {results['unverified']} UNVERIFIED")
    print(f"Screenshots: {len(results['screenshots'])} captured")
    if results["failed"] > 0:
        print("\nFailed tests:")
        for t in results["tests"]:
            if t["status"] == "FAIL":
                print(f"  ✗ {t['name']}: {t['details']}")
    return 1 if results["failed"] > 0 else 0
```

### §2.2 Running L4 Tests

```bash
# Individual test
uv run .claude/automations/test_dootsabha_consult.py

# All visual tests (via Makefile target)
make test-visual   # runs scripts/verify-visual-tests.sh
```

### §2.3 Screenshot Naming Convention

Format: `dootsabha_{command}_{state}_{timestamp}.png`

Examples:
- `dootsabha_consult_launch_20260301_143000.png`
- `dootsabha_council_dispatch_20260301_143012.png`
- `dootsabha_council_synthesis_20260301_143025.png`
- `dootsabha_status_healthy_20260301_143030.png`
- `dootsabha_status_degraded_20260301_143035.png`

Screenshots saved to `.claude/screenshots/` (gitignored). Matched by prefix (timestamp optional).

---

## §3 L4 Gating Hooks (Anti-Hallucination)

> These hooks prevent agents from claiming work is done without proof. This is the single most important mechanism for preventing agent hallucinations.

### §3.1 Task Verification Script (`scripts/verify-visual-tests.sh`)

Verifies L4 requirements for task completion. Called by both pre-task-done gate and pre-push hook.

**Checks:**
1. L4 test scripts referenced in task file "Files to Create" section exist on disk
2. Expected screenshots (listed in L4 verification section) exist in `.claude/screenshots/`
3. Task file contains `## Visual Test Results` section with actual review content (not just heading)

```bash
#!/usr/bin/env bash
set -euo pipefail
TASK_FILE="$1"
ERRORS=()

# 1. Check L4 scripts exist
SCRIPTS=$(grep -oP '\.claude/automations/test_dootsabha_\w+\.py' "$TASK_FILE" || true)
for script in $SCRIPTS; do
  [[ -f "$script" ]] || ERRORS+=("L4 script missing: $script")
done

# 2. Check screenshots exist (match by prefix)
SCREENSHOTS=$(grep -oP 'dootsabha_\w+\.png' "$TASK_FILE" || true)
for shot in $SCREENSHOTS; do
  prefix="${shot%.png}"
  found=$(find .claude/screenshots -name "${prefix}*" 2>/dev/null | head -1)
  [[ -n "$found" ]] || ERRORS+=("Screenshot missing: $shot")
done

# 3. Check Visual Test Results section exists with content
if ! grep -q '^## Visual Test Results' "$TASK_FILE"; then
  ERRORS+=("Missing '## Visual Test Results' section in task file")
elif [[ $(sed -n '/^## Visual Test Results/,/^## /p' "$TASK_FILE" | wc -l) -lt 5 ]]; then
  ERRORS+=("Visual Test Results section is too thin (needs actual findings)")
fi

if [[ ${#ERRORS[@]} -gt 0 ]]; then
  echo "❌ L4 GATE FAILED for $(basename "$TASK_FILE"):"
  for err in "${ERRORS[@]}"; do echo "  • $err"; done
  exit 2
fi
echo "✓ L4 gate passed for $(basename "$TASK_FILE")"
```

### §3.2 Pre-Task-Done Gate (`scripts/hooks/pre-task-done-gate.sh`)

Claude Code PreToolUse hook that intercepts Edit/Write on task files. If status is being changed to DONE, runs L4 verification:

```bash
#!/usr/bin/env bash
# Hook: PreToolUse (Edit, Write)
# Blocks DONE status on task files if L4 requirements not met
TOOL="$1"
FILE="$2"

# Only intercept task file edits
[[ "$FILE" == *docs/tasks/*.md ]] || exit 0

# Check if edit changes status to DONE
if grep -qi 'Status:.*DONE' <<< "$3" 2>/dev/null; then
  bash scripts/verify-visual-tests.sh "$FILE"
fi
```

### §3.3 Pre-Push Visual Gate (`scripts/hooks/pre-push-visual-gate.sh`)

Claude Code PreToolUse hook that intercepts `git push`. Finds all IN PROGRESS tasks and verifies each has passed L4:

```bash
#!/usr/bin/env bash
# Hook: PreToolUse (Bash) — matches git push
ERRORS=()
for task in docs/tasks/*.md; do
  if grep -q 'Status:.*IN PROGRESS' "$task"; then
    if ! bash scripts/verify-visual-tests.sh "$task"; then
      ERRORS+=("$task")
    fi
  fi
done
if [[ ${#ERRORS[@]} -gt 0 ]]; then
  echo "❌ PRE-PUSH GATE: L4 requirements unmet for:"
  for task in "${ERRORS[@]}"; do echo "  • $task"; done
  exit 2
fi
```

---

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

Every task file in `docs/tasks/` MUST include these two sections. This is a hard requirement — gating hooks enforce it.

**Section 1: `## Verification`** — must contain ALL applicable levels:

| Level | Required Content | Example |
|-------|-----------------|---------|
| **L1** | `make test` — expected: all pass | Always required |
| **L2** | `make test-integration` — expected: all pass | If integration tests exist |
| **L3** | `make build` + actual binary commands with expected output + `--json \| jq .` + `\| cat` (no ANSI) | Always required |
| **L4** | `uv run .claude/automations/test_dootsabha_{command}.py` + list of expected screenshot names | Required for any output-visible change |
| **L5** | `make test-agent` | Required for commands with `--json` output |

**Section 2: `## Visual Test Results`** — must contain actual evidence (not a placeholder):

| Field | Required? | Description |
|-------|-----------|-------------|
| L4 Script path | YES | `.claude/automations/test_dootsabha_{command}.py` |
| Date | YES | `YYYY-MM-DD` |
| Status | YES | `PASS (N/M)` or `FAIL (N/M)` |
| Test result table | YES | Each test with PASS/FAIL + detail column |
| Screenshots reviewed | YES | Each screenshot name + specific observation about what's visible |
| Findings | YES | Deviations, learnings, or "No issues found" |

**Minimum content:** The Visual Test Results section must be at least 5 lines long (enforced by gating hook). Empty or placeholder-only sections will be rejected.

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

> Git hooks are the last line of defense. They run *your code* through automated checks before it leaves the local machine. Combined with §3 gating hooks (Claude Code level), they create a two-layer quality system: lefthook catches code issues, §3 hooks catch process issues.

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
| **Claude Code hooks (§3)** | PreToolUse on Edit/Write/Bash | Task status violations, missing L4 evidence, missing screenshots | During agent execution |

Both layers are independent — either alone is insufficient. Lefthook catches code problems; §3 hooks catch process problems. Together they prevent both "code doesn't compile" and "agent claimed DONE without running tests."
