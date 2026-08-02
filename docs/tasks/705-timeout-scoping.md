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
| L2 `go test ./...` | pass; `-race -count=2` clean |
| L2 unit | 17 `Budget`/`StepContext` cases, 9 engine cases, 13 CLI cases |
| L3 `make test-binary` | 21/21 (9 new timeout-scoping cases) |
| L5 `make test-agent` | 222/222 (17 new) |
| L4 `shux` | screenshots under `.shux/out/20/` |

RED evidence: before the fix, 8 of the 9 new L3 cases failed — `review`, `refine`
and `council` all exited 4 on pipelines whose every individual step fitted its
budget.

## Regression guards

- `TestPipelineCommandsDoNotShareOneDeadline` — bans
  `context.WithTimeout(context.Background()` in review/refine/council, the exact
  shape of the bug.
- `TestBudgetStepIsFreshEachTime` — five sequential steps each get a full window.
- `TestEngineReleasesStepContexts` — no timer leak per agent per stage.
- L3 pipelines whose total runtime exceeds one invocation budget while no single
  step does: the combination fails under a shared deadline and passes under
  per-invocation budgets.
