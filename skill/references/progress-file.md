# Progress File Reference

**TL;DR** — pass `--progress-file <path>` and read that file whenever you like
while the run goes. One JSON object per line. The last line of a finished run is
`run_finished`, carrying the exit code the process returns.

```bash
dootsabha council --json --progress-file run.ndjson "…" > result.json &
tail -n1 run.ndjson                                  # what is it doing now?
jq -c 'select(.event=="call_started" or .event=="call_finished")' run.ndjson
```

- [Usage](#usage)
- [Events](#events) — schema version **1**
- [Reading a file whose run may have died](#reading-a-file-whose-run-may-have-died)
- [Guarantees](#guarantees)

## Usage

| | |
|---|---|
| Commands | `consult`, `council`, `review`, `refine` |
| Flag | `--progress-file <path>` (alias `--pragati-patra`) |
| Env | `DOOTSABHA_PROGRESS_FILE=<path>`, used when the flag is not given (`--progress-file ""` opts out) |

A run's stdout and exit code are what they would be without it; stderr gains
only a line if writing the file fails. The file is emptied when the run starts — one file, one run — so give each run its own
path. A symlink is followed, and its target emptied.

These exit `2` before any agent runs and leave the file untouched:

- a path that cannot be opened, or is not a regular file (a FIFO, a device,
  `/dev/null` — to turn the file off, pass `--progress-file ""`)
- the file stdout or stderr is redirected to, or the config file (`--config`, or
  `~/.config/dootsabha/config.yaml` without it)
- a file another dootsabha run is still writing (it holds a lock until it exits)

## Events

Every line has `ts` (RFC 3339, UTC, nanoseconds) and `event`. Fields may be added
within a schema version; none is renamed or removed, and no value changes meaning.

### `run_started` — once, before the first call

```json
{"ts":"…","event":"run_started","schema_version":1,"command":"council","pid":4242,
 "agents":["codex","grok"],"chair":"codex","timeout_ms":900000,"session_timeout_ms":4500000}
```

| Field | |
|---|---|
| `command` | `consult` · `council` · `review` · `refine` |
| `pid` | dootsabha's process id — see [below](#reading-a-file-whose-run-may-have-died) |
| `agents` | every agent the run may call (`review`/`refine`: the author first) |
| `chair` | `council` only |
| `author` | `review` and `refine` only |
| `timeout_ms` | budget for one call |
| `session_timeout_ms` | ceiling for the whole run; `0` = none (`consult` has none) |

A run that fails before it begins — a config error (`6`), an unknown agent (`2`) —
writes no `run_started`; its file holds `run_finished` alone.

### `call_started` / `call_finished` — one pair per provider call

```json
{"ts":"…","event":"call_started","call":3,"stage":"dispatch","agent":"grok"}
{"ts":"…","event":"call_finished","call":3,"stage":"dispatch","agent":"grok","status":"timeout","scope":"invocation","duration_ms":900004}
```

`call` numbers the run's calls from 1 and pairs a start with its finish. Calls in
one stage run in parallel, so their lines interleave.

| `stage` | Command |
|---|---|
| `consult` | `consult` |
| `dispatch` · `review` · `synthesis` | `council` (a fallback chair is `synthesis` too) |
| `author` · `review` | `review` |
| `author` · `review` · `incorporate` | `refine` |

| `status` | Meaning |
|---|---|
| `ok` | produced output |
| `error` | failed — the reason is in the command's own output, never here |
| `timeout` | hit a deadline; `scope` is `invocation` (`--timeout`) or `session` (`--session-timeout`) |
| `interrupted` | stopped because dootsabha was signalled, or still running when it exited |

`scope` is present exactly when `status` is `timeout`. These are the same
outcomes the exit code is computed from. A call attempted after the session
ceiling has passed still gets its pair — `timeout`, `session`, `duration_ms` near
0 — because it was attempted, and counted.

### `run_finished` — last, exactly once

```json
{"ts":"…","event":"run_finished","pid":4242,"exit_code":5,"duration_ms":1210000}
{"ts":"…","event":"run_finished","pid":4242,"exit_code":143,"signal":"SIGTERM","duration_ms":61000}
```

`exit_code` is the code the process exits with ([exit codes](exit-codes.md)). On
SIGINT, SIGTERM or SIGHUP the agents are stopped first, every open call is closed
as `interrupted`, and the line carries `signal` and `128 + N` — what a shell
reports in `$?` (130, 143, 129).

A reader that closes dootsabha's stdout or stderr ends the run with SIGPIPE, and
dootsabha exits `0` as it always has. The line then says `"signal":"SIGPIPE"`
with `exit_code` 0, and any call it cut short is `interrupted`: the run did not
finish, whatever the code says. The `signal` field is best effort here: if the
run was already exiting when the pipe broke, the line has the exit code alone.

## Reading a file whose run may have died

| Last line | Process `pid` | Means |
|---|---|---|
| `run_finished` | — | done; branch on `exit_code` |
| anything else | alive (`kill -0 $pid`) | still running; the open calls say what it waits on |
| anything else | gone | died without reporting: SIGKILL, a crash, power loss |
| no file, or a `pid` that is not the process you started | gone | killed in its first milliseconds, before it could report — the file, if any, is an earlier run's; use `$?` |

A file without `run_finished` whose process is gone means one of two things: the
process died (SIGKILL, a crash, power loss), or writing the file failed partway —
that run logged `progress file: write failed` on stderr and exited normally, so
its real code is wherever you captured `$?`. If you have neither, treat it like
exit `1`, and check stdout before trusting it.

## Guarantees

- Each line is written with one `write(2)` to a file opened `O_APPEND`, and a
  line the disk takes only part of is cut back. Lines are visible to readers as
  they are written. Parse only lines that end in a newline: a reader that
  catches a cut-back line mid-write sees a fragment without one.
- Every `call_started` has exactly one `call_finished` in a file with
  `run_finished`.
- No prompt, response or error text — names, stages, numbers and status only.
- The env variable is removed before agents start, so an agent that runs
  dootsabha itself does not write to its parent's file.
- A write that fails mid-run (disk full) is logged once on stderr at Error level;
  the run continues, and the file stops growing without `run_finished` — read it
  as the third row above once the process is gone.
