# दूतसभा — Agent Conventions

दूतसभा orchestrates AI coding CLIs (`claude`, `codex`, `agy`, `grok`) as a
council: dispatch → peer review → synthesis. It is command-run-exit, not a TUI.
The structured JSON and the exit codes **are** the product — other agents consume
them, so they are contracts, not output formatting.

## Session start
1. `cm context "<task>"` (CASS memory — the argument is required)
2. This file → `docs/PROGRESS.md` → the task file (`docs/tasks/NNN-*.md`)
3. Mark the task **IN PROGRESS** before the first code change
4. **Branch before the first edit.** Never commit to `main`.

## Commands
```bash
make build        # bin/dootsabha (auto-installs git hooks)
make ci           # lint + unit tests — the pre-push gate
make check        # everything: fmt + fix + lint + vet + test + L3 + installer
make pre-commit   # fast: fmt-check + vet + fix-check
make test-binary  # L3 smoke (mock providers)
make test-agent   # L5 JSON / exit codes / perf
make test-plugins # gRPC plugin smoke
make help         # all targets
```

**Build with Go 1.26, not the system Go:**
`export GOROOT="$HOME/sdk/go1.26.0"; export PATH="$GOROOT/bin:$PATH"`
Export it in the *same* shell call as `git commit` / `git push` — lefthook
inherits the environment, and under Go 1.27 `go fix` rewrites unrelated files and
fails the hook. `make fix-check` diffs against the **index**, so stage first.

## Testing levels
| Level | Command | What |
|---|---|---|
| L1 | `make ci-fast` | compile + lint + vet |
| L2 | `make test` | unit (mocks) |
| L3 | `make test-binary` | real binary + mock providers |
| L4 | `.shux/scripts/*.sh` | **shux** — real CLIs, headless PTY, PNG frames |
| L5 | `make test-agent` | JSON contract, exit codes, perf |

Never mark a task DONE without L1+L2+L3 green. L4 is required for
output-visible changes.

**L4 is shux.** The iTerm2 suite (`.claude/automations/`, `make test-visual`) is
retired — do not run or extend it. Copy `.shux/scripts/agy-3-7-evidence.sh`:
signal completion with a **file** marker (an on-screen marker lands in the
evidence), size each frame to its own output, **read every PNG back** before
attaching it, and stop every daemon you started
(`XDG_RUNTIME_DIR=<dir> shux daemon stop`). Never edit a script while it runs —
bash re-reads it by byte offset and dies mid-run.

## Adversarial pass — before the convergence review
Spawn **3–4 narrow parallel agents, never 1–2 sprawling ones**: give each a
disjoint attack surface, a hard step budget, an explicit *"if you run out, report
what you have — partial beats nothing"*, and instructions to return findings as
**text in the final message** (subagents are blocked from writing report files).
Reproduce every finding yourself before believing it, and if an agent returns
nothing, say so — never claim coverage it did not deliver.

## Cloud sessions
A cloud session has **no provider CLIs installed or authenticated**. Run
`dootsabha status` before planning any real-provider work: `council` / `review` /
`refine` against live agents will not run, and neither will L4 shux capture.
Use the host's own parallel agents for multi-perspective review instead, and
record L4 as *N/A — no provider CLIs in this environment* in the task file.
**Never fabricate evidence you could not capture.** L1/L2/L3/L5 run anywhere —
they use `testdata/mock-providers/`.

## PR review — MANDATORY
As soon as a PR exists, **and again after every fix push**, load the `gh-ghent`
skill and follow its decision order. Do not hand-roll `gh pr` / `gh api` polling,
and do not switch to bare `--watch` while review comments may still arrive.

## Exit codes (PRD §6.1)
One code, one caller action. Precedence `2 > 6 > 4 > 3 > 5 > 1 > 0`:

| | | |
|---|---|---|
| `2` | Usage | bad flags/args, unknown agent or chair — **fix the command** |
| `6` | Config | missing, unreadable, invalid — **fix the config** |
| `4` | Timeout | at least one agent ran out of time |
| `3` | Provider | every requested agent failed — nothing usable |
| `5` | Partial | some agents failed, output still usable |
| `1` | Error | a genuine internal bug — never a bad command line |
| `0` | Success | complete and usable |

**Pipelines do not decide their own exit code.** `review`, `refine` and `council`
record every provider call into `cli.Outcome` — `Outcome.Invoke` for their own,
`AddDispatches`/`AddReviews`/`AddSynthesis` for the engine's — and end with
`outcome.Exit()`. Three commands each deriving their own answer is how four
separate "a timed-out agent exits 0" bugs shipped (#20). Guards in
`internal/cli/outcome_test.go` fail the build if a command mints
`ExitProvider`/`ExitTimeout`/`ExitPartial` itself, returns `nil` from `RunE`,
calls a provider outside `Outcome.Invoke`, builds its own `context.WithTimeout`,
or adds a result type whose error no `Add*` reads.

## Timeouts
`--timeout` is the budget for **one agent call**; `--session-timeout` is the
ceiling for the **whole pipeline**. Every call takes a fresh window from
`core.Budget`, clipped by the session. Both exit `4`, and the message names which
fired. Never reuse one context across calls — that is the #20 regression.

Derive any per-call duration from `ctx.Deadline()`, never from `cfg.Timeout`: the
context already reflects both bounds.

## Anti-hallucination
1. Never claim DONE without terminal output as proof.
2. `make check` before every commit; `make ci` must pass before DONE.
3. Every task file needs `## Visual Test Results` with real evidence.
4. Mock providers are for L2/L3/L5 only — L4 uses the real CLIs.
5. Enumerate every path on `git add`. Never `git add -A` / `.` / `-u`.

## Conventions
- All command output through `internal/output.Renderer` — never `fmt.Print` in a command.
- `NO_COLOR`: test **presence**, not value — `_, set := os.LookupEnv("NO_COLOR")`.
- Every command needs a Devanagari alias (`sabha` → `council`); key flags too
  (`--dootas` → `--agents`). Root needs `Args: cobra.ArbitraryArgs` or extension
  discovery never fires.
- Log via `internal/observability`; `-v` Warn/Info, `-vv` Debug, `-vvv` +source.
  Warn is the default level in text mode and is raised to Error under `--json`,
  so stdout stays exactly one document.
- Do **not** append `2>&1` to commands by default — no dootsabha command needs it.

Go-specific rules load from `.claude/rules/go.md`; plugin and proto rules from
`.claude/rules/plugins.md`.

## Reading path
`CLAUDE.md` → `docs/PROGRESS.md` → task file → `docs/PRD.md §X.Y` → detail docs
(`docs/agy-cli-findings.md`, `docs/grok-cli-findings.md`, `docs/testing-strategy.md`).
