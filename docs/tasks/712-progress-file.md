# Task 712: A progress file for runs nobody is watching

## Status: DONE

## Depends On
705 (`cli.Outcome`, per-call budget), 711 (signal shutdown). Closes #34.

## Problem

A council takes minutes (median 5.8m, p90 14.8m, max 33.4m over 71 real runs).
Stage progress is drawn only when stderr is a TTY, and `--json` holds stderr at
Error, so an agent that starts a run sees nothing until it ends and guesses from
`ps`. Long runs go to the background, so the silent run is the normal case.

## PRD Ref
§6 global flags, §6.1 (exit codes — unchanged)

## Design

`--progress-file <path>` on `consult`, `council`, `review` and `refine`
(`DOOTSABHA_PROGRESS_FILE` when the flag is absent). The file is truncated when
the run starts and gets one JSON object per line, each written with a single
`write(2)` so a reader never sees half a line. stdout, stderr and exit codes are
untouched. Contract: `skill/references/progress-file.md`.

- **One emitter per call.** `Outcome.Invoke` (review, refine), the engine's
  `providerAgent` (council) and consult each wrap their provider call in
  `progressLog.call`, which writes `call_started`, runs it, and writes
  `call_finished` from the same `err` the ledger records. Status and scope come
  from the same classification `Outcome.Exit` uses.
- **Stage is the engine's to name.** `Dispatch`, `PeerReview` and `Synthesize`
  tag the context they hand their agents; the adapter reads it. No shared
  "current stage" variable.
- **`run_finished` is written by `exit`**, the single exit path, after agents are
  stopped and with the code passed to `os.Exit` — it cannot disagree with `$?`.
- **One target, two possible owners.** The path is resolved from argv in
  `Execute`, before any signal handler exists — Cobra never parses a command line
  it rejects. `progressMu` decides who opens it: the command at the start of
  `RunE`, or `exit`/the signal handler if `RunE` never got there. The end of the
  run keeps the lock, so nothing opens the file after its `run_finished`. A
  rejected command line therefore reports its own exit 2 instead of leaving the
  previous run's success.
- **A signal is reported, not a gap.** The signal handler, after stopping the
  agents, closes every open call as `interrupted` and writes `run_finished` with
  `exit_code` 128+N and `signal`, then ignores the other signals so it dies by
  the one it reported. Only SIGKILL, a crash, power loss, or a signal in the
  first milliseconds leave no `run_finished`; `pid` tells a stale file apart.
- **Status follows the ledger.** A deadline is `timeout`, even after a signal;
  council's engine calls carry the scope measured at the call (`scopedError`),
  so its exit message and the file name the same limit.
- **Every `call_started` gets a `call_finished`** in any file that has
  `run_finished`: calls still open at exit are closed as `interrupted`.
- **No text from the run.** Names, stages, durations, status — never prompt,
  response or error text (an error can quote a response).
- **The env form is not inherited.** The variable is unset at startup, so an
  agent that itself runs dootsabha does not truncate its parent's file.
- **A bad path is a bad command** (exit 2), checked before any agent runs and
  before the file is emptied: not a regular file (opening a FIFO blocked
  forever), the file stdout/stderr goes to, the `--config` file, or a file another
  run holds (`flock`). A write that fails mid-run is cut back to the last whole
  line, reported once at Error level, and the run continues — aborting a
  30-minute council over its progress log would lose the result.

## Files

| File | Change |
|---|---|
| `internal/cli/progress.go` | the log: open, events, `call`, `finish` |
| `internal/cli/outcome.go` | `Invoke` takes a stage and goes through the log |
| `internal/cli/agent.go` | engine calls go through the log |
| `internal/core/engine.go`, `review.go`, `synthesis.go` | stage on the context |
| `internal/cli/{consult,council,review,refine}.go` | flag, `run_started` |
| `internal/cli/interrupt.go` | `run_finished` on exit and on a signal |
| `internal/cli/root.go` | resolve the target from argv before any handler |
| `skill/references/progress-file.md` | the versioned contract |
| `scripts/test-agent-workflow.sh`, `scripts/progress-check.py` | L5 Workflow 12: every case against the contract |
| `.shux/scripts/issue-34-evidence.sh` | L4 capture, real codex + grok, before/after |

## Review

Four rounds, then the PR bot. Both councils ran this branch's own binary with `--progress-file`
against real codex and grok, and were watched from the file.

| Round | Reviewers | Outcome |
|---|---|---|
| 1 | codex + grok council (23m), takshaka, 3 adversarial agents (exit/signal path, file path surface, events vs outcome) | 14 reproduced defects fixed in `d184808`, `0347336`: FIFO hang, config/stdout clobber, two runs on one path, stale file after a rejected command line, missing `run_finished` after an early signal, wrong-signal death, torn last line, empty flag not opting out, council blaming the session for an invocation timeout, a real timeout relabelled `interrupted`, SIGPIPE looking like success |
| 2 | the 3 agents re-running their own repros, codex + grok council (43m) | all closed except one signal race (4/260) and the default config; argv scan disagreed with pflag; `--help` wrote a success record; a chair deadline + fallback failure took the wrong scope — `3395a85`, `12fd8fc` |
| 3 | codex, takshaka | `--config ""` left the default config unprotected (both found it); `--help=false` wrote no file — `09cd4d5` |
| 4 | codex, fix commit only | both CLOSED, nothing broken — SHIP |
| PR bot | codex review bot on #37 | a config path that does not exist yet was created (poisoning the default config); SIGPIPE recorded a cut-short call as `error` (3/3) — both fixed red-first in `4316a63` |

## Found, not fixed

- SIGPIPE racing a normal exit: `run_finished` has the true exit code but may
  lack `"signal":"SIGPIPE"`.
- A signal in the first milliseconds, before the handler is installed, leaves
  no file (or the previous run's; `pid` tells them apart). Documented.
- The `--json` result explains less than the file (`meta.providers` says
  `error` for a timeout and covers dispatch only; a failed reviewer or
  incorporate has no `error`; `--rounds` keeps only the last synthesis). #35.
- A write blocked by a hung filesystem would hold the log's lock at shutdown.
  FIFOs are refused, so only a stalled mount can do it. Not reproduced.

## Done Criteria

- [x] `--progress-file` on consult, council, review, refine; NDJSON, one write per line
- [x] every call: `call_started` + `call_finished` (`ok`/`error`/`timeout`+`scope`/`interrupted`)
- [x] `run_finished.exit_code` equals the process exit code, signals included
- [x] events come from the same error the ledger records
- [x] no prompt or response text in the file
- [x] contract documented in `skill/references/`
- [x] L1+L2+L3+L5 green; L4 real agents in shux

## Visual Test Results

Captured with `.shux/scripts/issue-34-evidence.sh` — real `codex` and `grok`, a
real `council --json` put in the background as a caller would. BEFORE is the
published v0.11.0, AFTER is `09cd4d5`. Each run is one shux window: the caller
above, an independent observer below (pr-evidence's `observe.sh`: a clock, the
bytes on stderr, the progress file, and the run's agent processes from `ps`).
Every frame was read back; videos pass `check_video.sh` (H.264 High@4.2, 15 fps).

| Scenario | before (v0.11.0) | after (this PR) |
|---|---|---|
| +52s into a council | stderr 0 bytes, no file; `ps`: codex and grok alive, stage unknown | file: both dispatches `ok` (9s, 11s), codex's review `ok` 34s, grok's review open |
| run ends | `$?` 0, nothing else | `run_finished exit_code=0` after 100s, matching `$?` |
| SIGTERM mid-review | `$?` 143, a 59-byte stderr notice | both open review calls `interrupted`, `run_finished exit_code=143 SIGTERM` |

Also dogfooded: both review councils of this branch (23m and 43m, grok's dispatch
15m and 22m) were watched from their progress files. L5 275/275 (Workflow 12, 28
cases), `make check` 0 issues, L3 40/40.
