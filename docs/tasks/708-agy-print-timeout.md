# Task 708: Forward the per-call budget to `agy --print-timeout`

## Status: DONE

## Depends On
707 (agy JSON + model bump), 705 (timeout scoping)

## Problem

`agy --print-timeout` defaults to **5m** — exactly दूतसभा's default per-call
`--timeout`. दूतसभा starts its budget marginally before the spawn, so on defaults
its own timeout wins and the user gets the named exit `4`.

Raise `--timeout` past 5m and that stops being true: agy self-terminates at its
own 5m and reports it as an **ordinary ERROR envelope with exit 1** —

```json
{"status":"ERROR","response":"","error":"timeout waiting for response", …}
```

— which is indistinguishable from any other provider failure. दूतसभा then charged
the caller exit **3** ("nothing usable, try another agent") for what was really
exit **4** ("raise the timeout"). One code, one caller action; this handed over
the wrong one. Found while probing the real binary for 707.

## PRD Ref
§6.1 (exit-code precedence), §5.3 (agy provider)

## Real-CLI findings (agy 1.1.17)

| Probe | Result |
|---|---|
| `--print-timeout 1ms` on a live prompt | exit 1, `error: "timeout waiting for response"` — the flag is honoured, and its failure is shaped like any other |
| **`--print-timeout 0`** | **also times out instantly** — `0` means zero, *not* "disabled" |
| `--print-timeout 15m30s` | accepted; Go duration strings parse |

The second one is the trap: a naive "no budget → send 0" would kill every call.

## Files

| File | Change |
|---|---|
| `internal/providers/agy.go` | `agyPrintTimeout(ctx)` derives the value from the step deadline; `--print-timeout` joins the pinned set when emitted |
| `testdata/mock-providers/mock-agy` | **honours `--print-timeout`**, defaulting to 1s as a stand-in for agy's 5m, and fails with the real timeout envelope |
| `scripts/test-binary.sh` | L3: agy's own timeout no longer preempts the budget; दूतसभा's own timer still exits 4 |
| `internal/providers/agy_test.go` | forwarding across window sizes; unbounded emits nothing; every pinned spelling |
| `configs/default.yaml`, `docs/agy-cli-findings.md`, `skill/references/command-reference.md`, `docs/PROGRESS.md` | docs |

## Design

The value comes from **`ctx.Deadline()`**, never from `cfg.Timeout`: `core.Budget`
already clips each step to the earlier of the per-invocation window and the
session ceiling, so the deadline on the context is the single source of truth.
Nothing here builds a `context.WithTimeout` — `outcome_test.go`'s guard stands.

Emitted value is `remaining + 30s`, rounded to the second. The margin makes
दूतसभा's timer fire first (so it keeps the exit code) and clears the
`SIGTERM`→grace→`SIGKILL` sequence, which is 5s by default.

A non-positive window emits **nothing** — never `0`, which agy reads as zero.

## Verification

```
make ci · make test-binary · make test-plugins · make test-agent
```

- `make ci` — lint **0 issues**, all unit tests pass
- L3 **40/40** (was 38) · L5 **238/238** · plugins **9/9** · shellcheck clean
- Mutation-tested: reverting the forwarding fails
  `TestAgyProviderForwardsStepBudgetAsPrintTimeout` **and** both new L3 cases.
- Forwarded values confirmed against an argv recorder:
  `--timeout 30s → 1m0s`, `15m → 15m30s`, `45m → 45m30s`
- The real `agy` 1.1.17 accepts the emitted duration string (`--print-timeout
  15m30s` → `status: SUCCESS`).

## Done Criteria

- [x] `--timeout` past agy's own default on a hanging agy exits `4`, not `3`
- [x] The emitted `--print-timeout` exceeds the context deadline, never trails it
- [x] A context with no deadline emits no `--print-timeout` (and never `0`)
- [x] `outcome_test.go` guards still pass — no new timeout arithmetic in the CLI layer
- [x] L3 covers the case with a stub that honours `--print-timeout`

## Residual (by design)

With **both** `--timeout 0` and `--session-timeout 0` the step has no deadline,
so दूतसभा emits nothing and agy's own 5m default applies. That is the one case
where a user's own `--print-timeout` in `providers.agy.flags` survives — the
escape hatch. Detecting agy's timeout by its message would mean string-matching
error text, which this repo does not do.

## Commit

```
fix(agy): forward the step budget to --print-timeout

agy's own print timeout defaults to 5m -- exactly dootsabha's default --timeout
-- and reports as a plain ERROR envelope with exit 1, so a timeout arrived
looking like any other provider failure and the caller got exit 3 instead of 4.
Raising --timeout past 5m did not raise agy's.

dootsabha now forwards the step's remaining window plus a 30s margin, taken from
ctx.Deadline() (core.Budget already clips it to the session ceiling), so its own
timer fires first and keeps the exit code. --print-timeout 0 means ZERO, not
disabled, so a non-positive window emits nothing at all.
```

## Session Protocol

1. `cm context`
2. Read `CLAUDE.md`, `docs/PROGRESS.md`, this file
3. Mark this task **IN PROGRESS** before any code change
4. Work the TDD steps in order, capturing each RED
5. L1–L5 + shux L4 before DONE

## Visual Test Results

Captured with `.shux/scripts/agy-708-evidence.sh` — real `agy` 1.1.17, real work,
no mocks in the frames that show live calls. Frames in `.shux/out/708/`
(gitignored; attached to the PR):

| Frame | Shows |
|---|---|
| `01-argv` | the forwarded values for `--timeout 30s / 15m / 45m`, from an argv recorder |
| `02-real-accepts` | the real CLI accepting `--print-timeout 15m30s` and answering |
| `03-exit-code` | the contract: a call slower than agy's own default now succeeds, and दूतसभा's own timer still exits `4` |
