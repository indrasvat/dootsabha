# Task 705 — Per-provider timeouts, session timeout for the pipeline

**Status:** DONE
**Depends On:** —
**Parallelizable With:** —
**Issue:** [#20](https://github.com/indrasvat/dootsabha/issues/20)

## Problem

`review`, `refine` and `council` created ONE `context.WithTimeout` up front and
reused it for every provider call. `--timeout` therefore behaved as a whole-command
deadline, not the per-invocation budget the docs and config described.

Observed in a real run:

- `dootsabha review --json --timeout 8m …`
- author (codex) completed after ~5m38s
- reviewer (claude) inherited the remaining ~2m22s
- reported failure: `timeout after 8m0s: claude invoke: … context deadline exceeded`

Claude was healthy throughout — a direct `consult --agent claude --timeout 60s`
answered in ~6.6s. The slow author was the cost; the reviewer was the symptom.

`session_timeout` existed in config and as a hidden flag but was never enforced
(`TODO(704)` in `root.go`).

## PRD Ref

§6.1 FR-ROOT-05 — `--timeout` applies per-agent invocation; `--session-timeout`
caps total session time.

## Files

- `internal/core/budget.go` — `Budget` (session ctx + fresh per-invocation `Step`), `StepContext`
- `internal/core/engine.go`, `review.go`, `synthesis.go` — derive a step context from `opts.Timeout`
- `internal/cli/budget.go` — timeout resolution, scope-aware messages, deadline detection
- `internal/cli/review.go`, `refine.go`, `council.go` — one budget per pipeline, one step per call
- `internal/cli/root.go` — `--session-timeout` unhidden and enforced; `--satra-seema` alias

## Semantics

| Knob | Scope | 0 means |
|------|-------|---------|
| `timeout` / `--timeout` / `--kaalseema` | one agent call | the 5m built-in default |
| `session_timeout` / `--session-timeout` / `--satra-seema` | whole pipeline | unbounded |

The session ceiling always wins — a step's effective deadline is the earlier of
the two. `--timeout` larger than `--session-timeout` warns on stderr rather than
silently truncating every call.

Both scopes exit `4`. The message names which fired:

```
Error: invocation timeout after 8m0s: codex invoke: …
Error: session timeout after 20m0s: claude invoke: …
```

A single agent hitting its own deadline no longer ends the pipeline: `refine`
continues to the next reviewer, and `council` keeps its healthy agents' output.
Exit `4` still outranks the partial result (precedence `4 > 5`).

## Verification

| Level | Result |
|-------|--------|
| L1 `make ci-fast` | 0 lint issues |
| L2 `go test ./...` | pass; `-race` clean |
| L2 unit | `Budget`/`StepContext`/`ScopeOf`, engine per-stage windows, CLI resolution + messages, proto round-trip |
| L3 `make test-binary` | 23/23 (11 new timeout-scoping cases) |
| L5 `make test-agent` | 228/228 (23 new) |
| L4 `shux` | 9 frames under `.shux/out/20/`, incl. a real-CLI old-vs-new run |

RED evidence: before the fix, 8 of the 9 original L3 cases failed — `review`,
`refine` and `council` all exited 4 on pipelines whose every individual step
fitted its budget. The multi-reviewer case was verified the same way against a
binary built from `main`: it abandoned the second reviewer entirely and returned
v1, where this branch runs it and returns v2.

Real CLIs, same command, two binaries (`.shux/out/20/09-real-cli-before-after.png`):

```
council --agents codex,agy --chair codex --timeout 25s --session-timeout 20m
  main         25.5s  exit 4  synthesis: synthesize fallback agy: context deadline exceeded
  this branch  33.0s  exit 0  synthesis complete
```

## One exit-code decision for every pipeline

Six defects landed in this change; four were the same one — a failure recorded
somewhere the aggregator did not look. The cause was structural: `review`,
`refine` and `council` each derived their own exit code from their own
bookkeeping, so every new stage or result type multiplied the places that had to
be updated together and nothing failed loudly when one was missed.

`internal/cli/outcome.go` is now the single input to every pipeline's exit code.
Commands record what each call did — `Outcome.Invoke` for their own calls,
`AddDispatches`/`AddReviews`/`AddSynthesis` for the engine's — and finish with
`outcome.Exit()`. "Nothing usable" is derived (no call succeeded) rather than
judged per command.

The design was reviewed by codex, agy and grok **before** implementation. All
three rejected its first shape — a `Recorder` callback on the engine — on two
grounds: an optional `SetRecorder` a new command forgets is worse than the
status quo because it looks like the design was used, and a Go callback cannot
reach an out-of-process strategy plugin, leaving two aggregation systems. Their
counter-point — that the result types already carry every error, so retention
was solved and only the scanning was inconsistent — is what shipped. Outcomes
travel as data, so a plugin's gRPC response converts to the same types and
reaches the same decision.

Net effect in `internal/cli`: 90 insertions, 367 deletions. Deleted:
`stageExitCode`, `firstDeadline`, `councilErrors`, `councilDeadline`,
`councilExit`, `invokeStep`, `stageFailureMessage`, and refine's
`partial`/`deadlineErr`/`deadlineScope` bookkeeping.

Three guards, each negative-tested by reintroducing the regression it forbids:

| Guard | Fails the build when |
|-------|----------------------|
| `TestOutcomeConsumesEveryResultError` | a result type gains an error field no `Add*` reads |
| `TestPipelineCommandsReachExitThroughOutcome` | a command mints `ExitProvider`/`ExitTimeout`/`ExitPartial` itself |
| `TestPipelineCommandsInvokeOnlyThroughOutcome` | a command calls a provider outside the choke point |
| `TestPipelineCommandsDoNotShareOneDeadline` | a command bakes one deadline for the whole pipeline |

They find pipeline commands by what they do — building a budget — rather than by
a hard-coded list, so a fourth command is covered without editing the test.

## Council review findings (codex · agy · grok, addressed)

- A chair that timed out was dropped once the fallback succeeded, so a council
  that hit a deadline could exit `0`. `SynthesisResult.ChairError` now carries
  it, including across the strategy-plugin gRPC boundary (`chair_error`).
- council's early returns printed bare stage errors; they now name the budget.
- Non-timeout peer-review failures reported `0`; they are `5`.
- The ceiling warning is pipeline-aware — `refine --reviewers a,b,c` is 7 calls,
  which overruns the shipped 5m/30m defaults — and is no longer suppressed in
  `--json` (stderr, so stdout stays one document).
- Timeout scope is decided when the step is created (`Budget.ScopeOf`) rather
  than by asking afterwards, closing a race where a subprocess in its 5s SIGTERM
  grace let a late session expiry rewrite the diagnosis.
- refine's continue-past-a-timed-out-reviewer path was untested; covered at L3+L5.

## Adversarial findings (parallel agents driving the real binary)

- **BLOCKER.** A provider whose grandchild calls `setsid()` escapes the process
  group the reaper signals and keeps the inherited stdout pipe, so `cmd.Wait()`
  never returned and the post-SIGKILL drain was an unbounded receive. Measured:
  `review --timeout 2s --session-timeout 30s` ran until killed at 25s — a
  timeout that never fires, in the very change that promises one. `Run` now owns
  its pipes so `Wait` depends on the process alone, and every wait is bounded.
  Same command now exits 4 in 4.0s; `main` still hangs.
- A one-agent council whose chair timed out exited 5 on this branch and 4 on
  main: `Synthesize` replaced the chair's error before returning, and with
  per-invocation budgets the session context is no longer a second witness.
- `Run` forked the child before checking the context, so an already-spent budget
  still cost a fork/exec.
- The ceiling warning counted invocations, not wall clock, so every council on
  the shipped defaults warned it might be cut short when it comfortably fits.
- `plugins/council-strategy` dropped `AgentConfig.timeout_ms` entirely, leaving
  every call in a plugin-run council unbounded — issue #20 out-of-process.

## Regression guards

- `TestPipelineCommandsDoNotShareOneDeadline` — bans
  `context.WithTimeout(context.Background()` in review/refine/council, the exact
  shape of the bug.
- `TestSequentialCommandsInvokeOnlyThroughInvokeStep` — review/refine may not
  call a provider directly; every call must go through `invokeStep`. Catches the
  variant the string ban misses (reusing `budget.Session()` for every call), and
  was itself negative-tested by reintroducing the bug.
- `TestBudgetStepIsFreshEachTime` — five sequential steps each get a full window.
- `TestEngineReleasesStepContexts` — no timer leak per agent per stage.
- L3 pipelines whose total runtime exceeds one invocation budget while no single
  step does: the combination fails under a shared deadline and passes under
  per-invocation budgets.
