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

## Directory Structure
```
cmd/dootsabha/main.go     Entry point → cli.Execute()
internal/cli/             Cobra commands (root, council, consult, ...)
internal/core/            Engine, config, subprocess, retry
internal/output/          Renderer, styles, formatters
internal/version/         Version via ldflags
internal/providers/       Claude/Codex/Gemini wrappers (Phase 1-2)
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

## Critical Spike Findings

### CLAUDECODE env var stripping (Spike 0.2)
When launching `claude -p` as subprocess, you MUST remove — not just empty — these env vars:
```go
for _, key := range []string{"CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT", "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS"} {
    os.Unsetenv(key) // or filter from cmd.Env
}
```
Setting to `""` is NOT sufficient — the key must be absent entirely.

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

## Coding Conventions
- Go 1.26 idioms — no bare returns, always wrap errors with `fmt.Errorf("...: %w", err)`
- All output through `internal/output.Renderer` — never `fmt.Print` directly in commands
- `NO_COLOR` env: use `_, set := os.LookupEnv("NO_COLOR")` (presence not value)
- TTY detection: `isatty.IsTerminal(os.Stdout.Fd())`
- No `huh` dependency — drop it unless forms are needed

## Anti-Hallucination Rules
1. Never claim DONE without terminal output as proof
2. `make ci` MUST pass before marking DONE
3. Every task needs `## Visual Test Results` with actual evidence
4. Mock providers for L2/L3 only — L4+ uses real CLIs
5. `make check` before every commit

## Progressive Disclosure Reading Path
`CLAUDE.md` → `docs/PROGRESS.md` → task file → `docs/PRD.md §X.Y` → detail docs
Per-task total: ~500 lines.
