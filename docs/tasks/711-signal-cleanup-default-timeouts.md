# Task 711: Stop agents when dootsabha is signalled; size the default budgets from real runs

## Status: IN PROGRESS

## Depends On
705 (per-call budget, session ceiling). Closes #33.

## Problem

Each agent runs in its own process group, so dootsabha's own timeout can stop it
cleanly — and so a signal sent to dootsabha never reaches it. dootsabha caught
only SIGPIPE: on SIGTERM, SIGINT or SIGHUP it died at once and left its agents
running. With the real CLIs (v0.10.2, macOS), `claude` (8 processes with its MCP
servers) and `grok` kept running reparented to PID 1; `codex` exited by itself
within 15s.

The usual sender is the caller's own workaround for the per-call default: 5m is
shorter than 13% of real calls, so callers raise it, outrun their own foreground
limit, and wrap dootsabha in `timeout` or a background task — 50 of 196 recorded
invocations.

## PRD Ref
§6.1 (exit codes — unchanged), §6 global flags (defaults)

## Data (from #33)

145 successful real calls: median 2.6m, p90 5.3m, p95 7.5m, max 23.3m; 19 over
5m, 1 over 15m. 71 councils: median 5.8m, p90 14.8m, max 33.4m. 127 of 196
invocations passed `--timeout`.

## Design

`core.Run` records each agent's process group in a registry (`liveGroups`) from
start until it is reaped. `cli.setupInterrupt` traps SIGINT, SIGTERM and SIGHUP;
on the first one it calls `core.StopAll` — SIGTERM to every group, up to 5s for
the leaders, then SIGKILL to each group so a descendant that ignored SIGTERM goes
too — and re-raises the signal with its default action. The caller sees the
status it always did (143/130/129); PRD §6.1 is untouched.

- **No new agents once stopping.** The registry's stopping flag and its insert
  share one lock, and `Run` checks it before spawning, so a council's next stage
  cannot start an agent during cleanup.
- **No exit can win the race.** Every `os.Exit` in `cli` — the result code,
  normal completion, the SIGPIPE→0 path — goes through `exit`, which blocks once
  a signal is being handled. Otherwise the killed agent's failure reaches
  `os.Exit(3)` (or a dead pipe reaches `exit 0`) before cleanup finishes.
- **Ignored stays ignored.** A shell starts background jobs with SIGINT ignored;
  `signal.Notify` on it would un-ignore it, so only signals not ignored at
  startup are trapped.
- **Defaults.** `timeout: 15m` covers 144 of 145 observed calls. `session_timeout:
  75m` is the longest default pipeline — `refine` with its two default reviewers,
  5 calls — at 15m each; 45m made every default `refine` warn
  (`TestShippedDefaultsDoNotWarn`). It does not loosen `council`: 3 steps are
  capped at 45m by the per-call bound.

Rejected: threading a signal-aware context through every command. It changes
each pipeline's error path (a cancelled call would be classified and printed as
a provider failure) for no gain over stopping the groups directly.

## Files

| File | Change |
|---|---|
| `internal/core/subprocess.go` | process-group registry, `StopAll`, `ErrStopping` |
| `internal/cli/interrupt.go` | trap, stop, re-raise; interrupt-aware `exit` |
| `internal/cli/root.go` | install the trap; route every exit through `exit` |
| `internal/core/config.go`, `budget.go`, `internal/cli/consult.go` | 15m / 75m defaults |
| `scripts/test-agent-workflow.sh` | L5: SIGTERM/INT/HUP on consult and council, ignored SIGINT |
| `internal/core/subprocess_stop_test.go` | whole group, SIGTERM-ignoring group, refusal once stopping |

## Done Criteria

- [ ] SIGTERM/SIGINT/SIGHUP stop every agent group (TERM, grace, KILL) before exit
- [ ] dootsabha then dies by the same signal (143/130/129); exit codes unchanged
- [ ] a second signal ends it immediately; no agent starts once stopping began
- [ ] a signal ignored at startup stays ignored
- [ ] stdout empty or one JSON document; SIGPIPE cannot turn an interrupt into 0
- [ ] defaults `timeout: 15m`, `session_timeout: 75m`; README, PRD, skill, help updated
- [ ] L1+L2+L3+L5 green; L4 real `claude`/`grok` before/after

## Visual Test Results

Pending.
