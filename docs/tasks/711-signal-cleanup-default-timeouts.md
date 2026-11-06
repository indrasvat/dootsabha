# Task 711: Stop agents when dootsabha is signalled; size the default budgets from real runs

## Status: DONE

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
start until its leader is reaped. `cli.setupInterrupt` traps SIGINT, SIGTERM and
SIGHUP; the first one runs `core.StopAll` — SIGTERM to every group, up to 5s for
the leaders, then SIGKILL to every group in that snapshot — and re-raises the
signal with its default action. The caller sees the death it always did
(`-15`/`-2`/`-1` to a waiting parent, 143/130/129 to a shell); PRD §6.1 is
untouched.

- **One shutdown.** A signal, a normal or error exit, and SIGPIPE's exit 0 all
  run inside one `sync.Once`, and each stops every agent before the process
  ends. Without it the killed agent's failure reaches `os.Exit(3)` (or a dead
  pipe reaches `exit 0`) mid-cleanup.
- **No new agents once stopping.** The stopping check, the fork and the insert
  share one lock, so a council's next stage cannot start an agent during
  cleanup.
- **Hurrying keeps the snapshot.** A second signal (signal path) or any signal
  (exit path) ends the grace early through `StopAll`'s `hurry` channel; the
  SIGKILL still reaches the original snapshot, so a group whose leader was
  reaped mid-grace keeps no survivor.
- **A call cut short takes its group.** The per-call timeout reaper SIGKILLs the
  group even when the leader exits within the grace, so a member that ignored
  SIGTERM goes too (v0.10.2 left it). A call that finished on its own leaves
  what it started alone.
- **Bounded on a stuck reader.** The "stopped N agent process group(s)" notice
  waits at most 250ms for stderr; the re-raise gets 1s to land before the
  `128+N` fallback.
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

## Review

Five rounds, each dogfooding this branch's own binary as a codex+grok council.

| Round | Reviewers | Outcome |
|---|---|---|
| 1 | codex council (grok timed out at 20m), takshaka, 3 adversarial agents | 8 findings fixed in `0002a19`: stderr-stall hang, re-raise race (2/60 exited 143), SIGPIPE/normal exits orphaning or racing cleanup, swallowed second signal, groups leaving the registry with live members, start/stop window, stale 5m/30m in `configs/default.yaml` and docs, misleading error after an interrupt |
| 2 | codex council (grok timed out), 2 agents re-running their repros | all round-1 findings CLOSED by the agents; codex: killing every finished group broke intentional background work, and a signal during exit cleanup waited the grace — `a04d8b2` |
| 3 | codex + grok council | the first signal never hurried the exit, and a fresh hurry sweep missed reaped leaders' children — `6b766fd` |
| 4 | codex + grok council, fix commits only | both: A1, A2 CLOSED, nothing broken — SHIP |

## Found, not fixed

- An exit already stopping agents keeps its own code; a signal arriving then
  hurries it but is not re-raised.
- A finished agent's background task that writes to its inherited stdout costs
  the 2s output drain and dies of SIGPIPE on its next write — as in v0.10.2.
- `cmd.Start()` runs under the registry lock; an exec stalled on a hung
  filesystem would delay `StopAll` until it returns. Not reproduced.
- Extensions (`dootsabha <ext>`) run in dootsabha's own group and are not
  tracked; a SIGTERM sent to dootsabha alone still leaves one running.
- SIGPIPE mid-run still exits 0 for an unfinished pipeline (its agents are now
  stopped first).
- `council --rounds 2` now warns that it may be cut short (6 × 15m > 75m); no
  recorded call used `--rounds`.

## Done Criteria

- [x] SIGTERM/SIGINT/SIGHUP stop every agent group (TERM, grace, KILL) before exit
- [x] dootsabha then dies by the same signal (143/130/129); exit codes unchanged
- [x] a second signal ends it immediately; no agent starts once stopping began
- [x] a signal ignored at startup stays ignored
- [x] stdout empty or one JSON document after an interrupt; SIGPIPE stops agents before exit 0
- [x] defaults `timeout: 15m`, `session_timeout: 75m`; README, PRD, skill, help updated
- [x] L1+L2+L3+L5 green; L4 real `claude`/`grok` before/after

## Visual Test Results

Captured with `.shux/scripts/issue-33-evidence.sh` — real `claude` and `grok`,
real calls, no mocks. BEFORE is the published v0.10.2, AFTER is `6b766fd` built
from a clean tree. Each run is one shux window: the driver above, an independent
observer below (a clock plus `ps` of the agents, PPID 1 = orphaned). Videos and
end frames in `.shux/out/711/` (gitignored; attached to the PR), each read back.

| Run | 19–21s after `kill -TERM` to dootsabha |
|---|---|
| before | dootsabha gone (exit 143); `claude -p` (3 children) and `grok -p` still running, PPID 1 |
| after | exit 143, stderr `stopped 1 agent process group(s)` each; observer: no agents running |

Also measured, final binary: 60/60 runs died by SIGTERM (`-15` to a waiting
parent; the first cut exited 143 in 2/60). L5 247/247 with the signal cases of
Workflow 11; `make check` 0 issues, L3 40/40.
