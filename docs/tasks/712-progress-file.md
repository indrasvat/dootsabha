# Task 712: A progress file for runs nobody is watching

## Status: IN PROGRESS

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
| `skill/references/progress-file.md` | the versioned contract |

## Done Criteria

- [ ] `--progress-file` on consult, council, review, refine; NDJSON, one write per line
- [ ] every call: `call_started` + `call_finished` (`ok`/`error`/`timeout`+`scope`/`interrupted`)
- [ ] `run_finished.exit_code` equals the process exit code, signals included
- [ ] events come from the same error the ledger records
- [ ] no prompt or response text in the file
- [ ] contract documented in `skill/references/`
- [ ] L1+L2+L3+L5 green; L4 real agents in shux

## Visual Test Results

_Pending._
