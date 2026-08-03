# दूतसभा — Agent Conventions

## Session Start Protocol
1. Run `cm context` (CASS Memory — session context)
2. Read this file
3. Read `docs/PROGRESS.md` to see current phase/task
4. Read the specific task file (`docs/tasks/NNN-task-name.md`)
5. Mark task `IN PROGRESS` before any code changes

## Build Commands
```bash
make build        # Build bin/dootsabha (auto-installs hooks)
make ci           # Lint + test (pre-push gate, <30s)
make test         # Unit tests only
make test-binary  # L3 smoke (binary + mock providers)
make check        # Full suite: fmt+fix+lint+vet+test+smoke
make pre-commit   # Fast gate: fmt-check+vet+fix-check (<3s)
make help         # All targets
```

## PR Review Workflow (MANDATORY)
**As soon as a PR is created — and after every review-fix push — load the
`gh-ghent` skill and run the PR review loop.** This is a required step of this
repo's development workflow, not optional.

1. Invoke the `gh-ghent` skill (Skill tool) to load the current command contract.
2. Run the single blessed command (personal repo → keep `--solo`):
   ```bash
   PR=$(gh pr view --json number -q .number)
   gh ghent status --pr "$PR" --await-review --solo --logs --format json --no-tui
   ```
   It waits for CI + bounded review monitoring (understands Codex PR-body
   signals) and returns threads, checks, reviews, and `is_merge_ready` in one shot.
3. Act on the **first** matching condition (skill's decision order): fix CI
   failures → answer/​resolve bot threads (`gh ghent reply --resolve`) → dismiss
   stale blockers → re-run the **same** command after each fix push.
4. Repeat until `is_merge_ready == true` with non-low `review_monitor.confidence`.

Do NOT hand-roll `gh pr`/`gh api` polling or switch to bare `--watch` while
review comments may still arrive — always re-run the `--await-review` command.

## Directory Structure
```
cmd/dootsabha/main.go     Entry point → cli.Execute()
internal/cli/             Cobra commands (root, council, consult, ...)
internal/core/            Engine, config, subprocess, retry
internal/output/          Renderer, styles, formatters
internal/version/         Version via ldflags
internal/providers/       Claude/Codex/Agy (Phase 1-2, mostly replaced by plugins)
internal/plugin/          gRPC plugin manager & handshake logic
plugins/                  Provider and Strategy plugins (Phase 3+)
proto/                    gRPC service definitions (.proto + generated)
testdata/mock-providers/  Mock CLIs for L3 tests
scripts/                  Smoke tests, agent tests, gating hooks
configs/default.yaml      Skeleton config
docs/tasks/               Per-task files (NNN-name.md)
docs/PROGRESS.md          Phase/task status tracker
.claude/automations/      iTerm2-driver L4 scripts
.claude/screenshots/      L4 evidence screenshots
```

## Testing Levels
| Level | Command | Speed | What |
|-------|---------|-------|------|
| L1 | `make ci-fast` | <5s | Compile + lint + vet |
| L2 | `make test` | <2s | Unit tests (mocks) |
| L3 | `make test-binary` | <10s | Binary + mock providers |
| L4 | `make test-visual` | 30-60s | Real CLIs + iTerm2 screenshots |
| L5 | `make test-agent` | 2-5min | JSON/exit codes/perf |

**Never mark a task DONE without L1+L2+L3 passing. L4 required for output-visible changes.**

## Exit Codes (PRD §6.1)
One code, one caller action. Precedence: `2 > 6 > 4 > 3 > 5 > 1 > 0`.
- `2` (ExitUsage): Bad flags/args, unknown agent or chair — **fix the command** (highest)
- `6` (ExitConfig): Config missing, unreadable, invalid — **fix the config**
- `4` (ExitTimeout): At least one agent timed out
- `3` (ExitProvider): Every requested agent failed — nothing usable
- `5` (ExitPartial): Some agents failed, output still usable
- `1` (ExitError): Unexpected internal error — a bug
- `0` (ExitSuccess): Complete and usable (lowest)

Read most-blocking first: never valid → couldn't load config → ran out of time →
nothing usable → partially usable. `1` is reserved for genuine internal failures,
never for a bad command line.

**Pipelines do not decide their own exit code.** `review`, `refine` and `council`
record what every provider call did into `cli.Outcome` — `Outcome.Invoke` for
their own calls, `AddDispatches`/`AddReviews`/`AddSynthesis` for the engine's —
and end with `outcome.Exit()`. Three of them each deriving their own answer is
how four separate "a timed-out agent exits 0" bugs shipped (#20). Guards in
`outcome_test.go` fail the build if a command mints `ExitProvider`/`ExitTimeout`/
`ExitPartial` itself, returns `nil` from `RunE`, calls a provider outside
`Outcome.Invoke`, builds its own `context.WithTimeout`, or adds a result type
with an error no `Add*` reads.

## Timeouts
`--timeout` is the budget for **one agent call**; `--session-timeout` is the
ceiling for the **whole pipeline**. Every call gets a fresh window from
`core.Budget`, clipped by the session. Both exit `4`; the message names which
fired. Never reuse one context across calls — that is the #20 regression.

## Plugin Architecture
- **gRPC based:** Uses `hashicorp/go-plugin`.
- **Handshake Cookies:**
  - Provider: `dootsabha-provider-v1`
  - Strategy: `dootsabha-strategy-v1`
  - Hook: `dootsabha-hook-v1`
- **Plugin Registry:** Manager handles discovery and graceful shutdown.
- **Extensions:** `dootsabha-{name}` binaries on $PATH or `~/.local/bin` (user-local wins).
- **Extension Context:** JSON passed via `DOOTSABHA_CONTEXT_FILE` env var.

## Bilingual Interface (Devanagari/Hindi)
- **Aliases:** Every command MUST have a Hindi/Devanagari alias (e.g., `sabha` for `council`).
- **Flags:** Key flags should have bilingual equivalents (e.g., `--dootas` for `--agents`).
- **Discovery:** `cobra.ArbitraryArgs` required on root for extension discovery to trigger.

## Observability
- **Structured Logging:** Use `internal/observability.Logger` (wraps `slog`).
- **Trace ID:** All logs include `ds_{random5}` session trace ID.
- **Verbosity:** `-v` (Warn/Info), `-vv` (Debug), `-vvv` (Debug+Source).
- **Metrics:** Thread-safe `InProcessCollector` for tokens, costs, and durations.

## Critical Spike Findings

### CLAUDECODE env var (Spike 0.2 + env-minimal spike)
Only `CLAUDECODE` needs to be unset — it is the sole var Claude CLI checks for
nested session detection. All other `CLAUDE_CODE_*` vars (USE_BEDROCK, USE_VERTEX,
ENTRYPOINT, etc.) are left untouched. This is done ONCE at startup via
`core.DetectAndCleanClaude()` in `init()` — no per-invocation sanitization needed.
When inside Claude Code, `core.InsideClaude` is true and council defaults to
`codex,agy` (Claude is already the host).

### No huh spinner — use raw goroutine (Spike 0.7)
`huh v0.8.0` removed `NewSpinner()`. Use:
```go
func runSpinner(ctx context.Context, msg string) func() {
    frames := []string{"⠋","⠙","⠹","⠸","⠼","⠴","⠦","⠧","⠇","⠏"}
    done := make(chan struct{})
    go func() {
        i := 0
        for {
            select {
            case <-done:
                fmt.Fprintf(os.Stderr, "\r\033[K")
                return
            default:
                fmt.Fprintf(os.Stderr, "\r%s %s", frames[i%len(frames)], msg)
                time.Sleep(80 * time.Millisecond)
                i++
            }
        }
    }()
    return func() { close(done); time.Sleep(50 * time.Millisecond) }
}
```

### exec.Command not CommandContext (Spike 0.5)
Use `exec.Command` (not `exec.CommandContext`) — CommandContext sends SIGKILL immediately,
bypassing SIGTERM→grace→SIGKILL sequence. Manage lifecycle manually.

### cobra.ArbitraryArgs for extension discovery (Spike 0.5)
Root command MUST have `Args: cobra.ArbitraryArgs` or `RunE` is never called for unknown commands.

## Shell Command Conventions
- Do NOT append `2>&1` to commands by default. Only use stderr redirect when you specifically need to capture or suppress stderr. No dootsabha command needs it.
- Run `make`, `go`, and `dootsabha` commands without output redirection unless there is a concrete reason.

## Coding Conventions
- Go 1.26 idioms — no bare returns, always wrap errors with `fmt.Errorf("...: %w", err)`
- All output through `internal/output.Renderer` — never `fmt.Print` directly in commands
- `NO_COLOR` env: use `_, set := os.LookupEnv("NO_COLOR")` (presence not value)
- TTY detection: `isatty.IsTerminal(os.Stdout.Fd())`
- No `huh` dependency — drop it unless forms are needed

## L4 Script Convention (iTerm2-driver)
All `.claude/automations/test_*.py` scripts MUST follow the canonical pattern:
- `try/except/finally` wrapping all test logic in `main()`
- `created_sessions = [session]` — track every session, append new ones
- 4-level `cleanup_session()`: Ctrl+C → "q" → "exit\n" → `session.async_close()`
- `finally` iterates `created_sessions` and cleans up all of them (tabs closed, not left open)
- Entry point: `if __name__ == "__main__": exit_code = iterm2.run_until_complete(main); exit(...)`
- `main()` returns exit code from `print_summary()` — never call `sys.exit()` inside main

## Anti-Hallucination Rules
1. Never claim DONE without terminal output as proof
2. `make ci` MUST pass before marking DONE
3. Every task needs `## Visual Test Results` with actual evidence
4. Mock providers for L2/L3 only — L4+ uses real CLIs
5. `make check` before every commit

## Progressive Disclosure Reading Path
`CLAUDE.md` → `docs/PROGRESS.md` → task file → `docs/PRD.md §X.Y` → detail docs
Per-task total: ~500 lines.
