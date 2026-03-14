# Task 700: Council JSON Robustness & Env Sanitization Fix

## Status: DONE

## Depends On
None (bugfix, all referenced code exists)

## Parallelizable With
None

## Problem
GitHub Issue #4 reports three bugs, plus an implicit fourth affecting Bedrock users:

1. **Codex `bufio.Scanner` overflow**: `parseCodexJSONL` uses default 64KB scanner
   buffer. Large prompts (e.g., `$(git diff)`) produce JSONL lines exceeding this,
   causing `bufio.Scanner: token too long`.
2. **No JSON on exit 1**: When all agents fail, `council.go` returns `ExitError`
   before reaching `renderCouncilJSON`. Programmatic callers get empty stdout.
   Same bug exists in `consult.go`, `review.go` (author failure), and `refine.go`
   (author v1 failure).
3. **stderr leaks with `--json`**: slog WARN lines + Cobra `Error:` line + stderr
   progress dots all leak to stderr even with `--json`.
4. **Bedrock/Vertex/Foundry env stripping**: `SanitizeEnvForClaude` uses prefix match
   on `CLAUDE_CODE*`, stripping routing vars like `CLAUDE_CODE_USE_BEDROCK`.
   Bedrock-only users get "Credit balance is too low".

## PRD Ref
§6.1 (Exit Codes), §6.2 (JSON Output Schema), §10.7 (Anti-Hallucination)

## Files
| File | Action |
|------|--------|
| `internal/providers/codex.go` | Replace `bufio.Scanner` with `bytes.Split` in `parseCodexJSONL` |
| `internal/cli/council.go` | JSON on all error paths; suppress progress in JSON mode; exit code fix |
| `internal/cli/consult.go` | JSON error output on provider failure |
| `internal/cli/review.go` | JSON error output on author failure |
| `internal/cli/refine.go` | JSON error output on author v1 failure |
| `internal/cli/root.go` | Conditional SilenceErrors (JSON mode only); print errors in Execute() |
| `internal/observability/logger.go` | Suppress WARN in JSON mode (raise to Error) |
| `internal/core/subprocess.go` | Explicit blocklist for env stripping (session + credential vars) |
| `internal/core/subprocess_test.go` | Tests: Bedrock/Vertex/Foundry preserved, session tokens stripped |
| `internal/providers/codex_test.go` | Test for large JSONL lines (>64KB, >1MB) |

## Steps

### 1. Fix env sanitization (`subprocess.go`)
Replace prefix match with explicit blocklist of vars that cause nested-session errors
OR leak credentials. Preserve routing/config vars.

**Strip** (session detection + credentials):
```go
var claudeEnvStriplist = map[string]bool{
    // Session detection (cause nested-session error)
    "CLAUDECODE":                                  true,
    "CLAUDE_CODE_ENTRYPOINT":                      true,
    "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS":         true,
    // Session credentials (should not leak to subprocess)
    "CLAUDE_CODE_REMOTE_SESSION_ID":               true,
    "CLAUDE_CODE_SESSION_ACCESS_TOKEN":             true,
    "CLAUDE_CODE_WEBSOCKET_AUTH_FILE_DESCRIPTOR":   true,
    "CLAUDE_CODE_OAUTH_TOKEN":                     true,
    "CLAUDE_CODE_OAUTH_TOKEN_FILE_DESCRIPTOR":     true,
    "CLAUDE_CODE_OAUTH_REFRESH_TOKEN":             true,
    "CLAUDE_CODE_API_KEY_FILE_DESCRIPTOR":          true,
}
```

**Preserve** (routing/config — not exhaustive, these are just the test cases):
- `CLAUDE_CODE_USE_BEDROCK`, `CLAUDE_CODE_USE_VERTEX`, `CLAUDE_CODE_USE_FOUNDRY`
- `CLAUDE_CODE_SKIP_BEDROCK_AUTH`, `CLAUDE_CODE_SKIP_VERTEX_AUTH`, `CLAUDE_CODE_SKIP_FOUNDRY_AUTH`
- `CLAUDE_CODE_MODEL`, `CLAUDE_CODE_MAX_OUTPUT_TOKENS`, all `CLAUDE_CODE_DISABLE_*`, etc.

Tests:
- Stripped vars are absent from output
- Routing vars (Bedrock, Vertex, Foundry) and auth-skip vars are preserved
- Non-CLAUDE vars unchanged

### 2. Fix JSONL parser (`codex.go`)
`res.Stdout` is already fully buffered in a `bytes.Buffer`. Using `bufio.Scanner`
on an in-memory buffer is the wrong primitive — replace with `bytes.Split(data, '\n')`
which has no line-length limit:

```go
func parseCodexJSONL(data []byte) (agentMsg string, usage *codexUsage, err error) {
    for _, line := range bytes.Split(data, []byte("\n")) {
        line = bytes.TrimSpace(line)
        if len(line) == 0 { continue }
        var ev codexEvent
        if jsonErr := json.Unmarshal(line, &ev); jsonErr != nil { continue }
        // ... same switch logic ...
    }
    return agentMsg, usage, nil
}
```

Tests: L2 test with a single JSONL line >64KB and >1MB.

**Note on argv limits**: The prompt is still passed as a CLI arg (`ARG_MAX` ~1MB on
macOS). Fixing via stdin (`codex exec -`) requires adding `WithStdin` to
`SubprocessRunner.Run` — deferred to a follow-up task (broader change affecting all
providers). For now, prompts >32KB are already truncated by `core.TruncateString`
in review/refine, but council/consult pass raw. Document this limitation.

### 3. Fix JSON output on ALL error paths

**council.go** — three classes of early return need JSON:
- `successes == 0` (all agents failed): emit `renderCouncilJSON(allDispatches, nil, nil)`
- Config/provider construction errors: emit minimal error JSON
- Dispatch/peer-review/synthesis errors: emit what we have so far

Exit code for `successes == 0`: use `HighestExitCode` from per-dispatch errors
(e.g., if all failed due to timeout, exit 4 not 1). Fall back to exit 1 if no
specific code.

**JSON mode exit code masking**: Currently `renderCouncilJSON` returns nil (exit 0)
even for partial results. After JSON render, check for partial/failed dispatches and
return the appropriate `ExitError` with the correct code.

Schema fix: Change `councilJSON.Synthesis` from `councilSynthesisJSON` to
`*councilSynthesisJSON` so nil serializes as `null`, not an empty object.

**consult.go**: On provider error, emit error JSON before `ExitError`.
**review.go**: On author failure, emit partial JSON (author=nil, reviewer=nil).
**refine.go**: On author v1 failure, emit error JSON with empty versions.

Handle encoder failure: if `renderXxxJSON` itself fails (broken pipe), return
that error, not the original `ExitError`.

### 4. Fix stderr leaks

**a) slog level** (`logger.go`):
When `jsonMode && verbosity == 0`, set level to `slog.LevelError` instead of
`slog.LevelWarn`. This suppresses the `slog.Warn("agent invocation failed", ...)`
lines. When `-v` is explicit, keep the user's requested level.

**b) Cobra Error: line** (`root.go`):
Do NOT use global `SilenceErrors = true` — that breaks TTY error reporting for
unknown commands, flag parse errors, etc. Instead:
- In `Execute()`, set `rootCmd.SilenceErrors = jsonOutput` before `rootCmd.Execute()`
- OR: always `SilenceErrors = true` but have `Execute()` print the error to stderr
  for non-JSON mode: `fmt.Fprintf(os.Stderr, "Error: %s\n", err)`

**c) Progress dots** (`council.go`):
The `stderrProgress(...)` callback is installed without checking JSON mode
(line 139). Guard with `!rc.IsJSON()`:
```go
if stderrIsTTY && !quiet && !rc.IsJSON() {
    eng.SetProgress(stderrProgress("dispatch", rc.HasColor))
}
```
Same for review progress line 168.

### 5. Verify

```bash
make ci          # L1+L2 pass
make test-binary # L3 smoke pass
make check       # Full suite clean
```

Manual verification (using mock providers to induce failures):
- `dootsabha council --json --agents claude,codex "test"` with mock that exits 1 →
  valid JSON on stdout with per-agent errors, correct exit code, empty stderr
- `dootsabha consult --json --agent claude "test"` with mock failure → JSON error
- `CLAUDE_CODE_USE_BEDROCK=1 dootsabha consult --agent claude "test"` →
  Bedrock env preserved (verify via `env` mock provider)

## Done Criteria
- [ ] `parseCodexJSONL` uses `bytes.Split` — no line-length limit
- [ ] ALL commands (council, consult, review, refine) produce valid JSON on every
      error path when `--json` is set
- [ ] JSON mode exit codes match PRD §6.1 (not always 0 or always 1)
- [ ] `councilJSON.Synthesis` is `*councilSynthesisJSON` (nil = `null` in JSON)
- [ ] `--json` without `-v` produces zero stderr output (no logs, no progress, no Error:)
- [ ] `--json -v` restores logs to stderr (user opt-in)
- [ ] `CLAUDE_CODE_USE_BEDROCK`, `CLAUDE_CODE_USE_VERTEX`, `CLAUDE_CODE_USE_FOUNDRY`
      preserved in subprocess env
- [ ] `CLAUDE_CODE_SKIP_BEDROCK_AUTH`, `CLAUDE_CODE_SKIP_VERTEX_AUTH`,
      `CLAUDE_CODE_SKIP_FOUNDRY_AUTH` preserved
- [ ] Session vars (`CLAUDE_CODE_OAUTH_TOKEN`, `CLAUDE_CODE_SESSION_ACCESS_TOKEN`,
      etc.) stripped
- [ ] `CLAUDECODE`, `CLAUDE_CODE_ENTRYPOINT`, `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS`
      still stripped
- [ ] Non-JSON TTY errors still display correctly (SilenceErrors not global)
- [ ] `make ci` passes, no lint issues
- [ ] L4 iTerm2 visual tests verify JSON error output and stderr cleanliness
- [ ] PR created referencing issue #4 with `Fixes #4`
- [ ] CI green on PR
- [ ] Codex bot review addressed (all comments resolved, thumbs-up received)
- [ ] Evidence documented for every bug fixed

## Out of Scope (follow-up tasks)
- **Stdin support for large prompts**: Adding `WithStdin` to `SubprocessRunner.Run`
  and using `codex exec -` / `claude -p -` for prompts >32KB. Broader change
  affecting Runner interface and all 3 providers. Track as Task 701.
- **status/plugin JSON error paths**: `status --json` returns 0 for unhealthy
  providers, `plugin inspect` returns plain error. Not reported in issue #4.
- **Review prompt truncation**: `review.go` embeds full author output into reviewer
  prompt without truncation (unlike refine.go which uses `TruncateString`).

## Commit
```
fix(council): JSON output on error, JSONL parser, Bedrock env (#4)

- Replace bufio.Scanner with bytes.Split in parseCodexJSONL (no line limit)
- Emit JSON on all error paths for council/consult/review/refine
- Fix JSON mode exit codes (use HighestExitCode, not always 0)
- Suppress stderr in JSON mode (slog level, progress dots, Cobra errors)
- Switch env sanitization to explicit striplist (preserve Bedrock/Vertex/Foundry)

Fixes #4
```

## Session Protocol
1. Read this file
2. Mark IN PROGRESS
3. Fix in order: env sanitization → JSONL parser → JSON error paths → stderr leaks
4. `make ci` after each fix
5. `make check` before commit
