# Task 709: Put each rule in the layer that can actually enforce it

## Status: DONE

## Depends On
None. Follows the CLAUDE.md refresh (PR #29).

## Problem

The repo's L4 gate — which `testing-strategy.md §3` called *"the single most
important mechanism for preventing agent hallucinations"* — had **never run**.
Three independent reasons, each sufficient on its own:

1. **Registered nowhere.** No `.claude/settings.json` existed; nothing in
   `lefthook.yml` or the Makefile referenced either hook script.
2. **Wrong input contract.** `pre-task-done-gate.sh` read `$1/$2/$3`, but a
   Claude Code `PreToolUse` hook receives **JSON on stdin**. Wired as-is, `FILE`
   would be empty, the `*docs/tasks/*.md` guard false, and it would `exit 0` —
   passing everything.
3. **Dead logic.** Its two substantive checks used `grep -oP`, unsupported by BSD
   grep on macOS, paired with `|| true`. Silently skipped. (Fixed in #29; the
   other two were not.)

The root cause is not any of those. It is that **"does this file contain
evidence?" is a question about repo STATE, and it was being asked from a HOOK.**
Hooks intercept actions and do not run in CI, so nothing ever exercised it.

## The layering this task establishes

| Layer | Answers | Fails loudly? |
|---|---|---|
| **Hook** | "is this action about to do something hard to undo?" | only when it fires |
| **Test** | "is this true of the repo right now?" | yes — `make check`, CI, every push |
| **CLAUDE.md** | anything requiring judgment | no — guidance only |

Put a rule in the wrong layer and it fails silently. `internal/cli/outcome_test.go`
is the repo's own proof the middle row works.

## A second constraint, specific to agents

**A false block is worse than a miss.** Observed: when the pre-push gate blocked
work on tasks 300/703, the agent did not investigate — it flipped both to DONE to
make the gate pass, violating the very rule the gate protected. A misfiring gate
does not stop an agent; it teaches the agent to satisfy it.

So: few gates, precise, near-zero false positives, and every one must name the fix.

## What is NOT gated, deliberately

Evidence **quality**. A hook can confirm a file exists; it cannot confirm anyone
looked at it. The old `>= 5 lines` rule measured the shape of a claim, not its
truth. Honesty is not gateable — it is what review is for. During #27–#29,
दूतसभा and Codex caught four factual errors and a bug inside a fix; no gate
could have. `testing-strategy.md` is corrected to stop claiming otherwise.

## Files

| File | Change |
|---|---|
| `.claude/hooks/branch-guard.sh` | NEW — deny Edit/Write to a tracked file on the default branch |
| `.claude/hooks/staging-guard.sh` | NEW — deny `git add -A/./-u`, `git commit -a` |
| `.claude/hooks/session-hygiene.sh` | NEW — Stop: **warn** about leftover daemons; never block |
| `.claude/settings.json` | NEW — wires the three, repo-scoped |
| `internal/tasks/evidence_test.go` | NEW — a DONE task must carry real evidence; runs in CI |
| `scripts/verify-visual-tests.sh` | DELETED — replaced by the test; one implementation, not two |
| `scripts/hooks/pre-push-visual-gate.sh` | DELETED — gated every push on any IN PROGRESS task |
| `scripts/hooks/pre-task-done-gate.sh` | DELETED — broken input contract; job now the test's |
| `docs/testing-strategy.md` | §3 rewritten around the layering; the overclaim removed |
| `CLAUDE.md` | records which layer enforces what |

## Done Criteria

- [x] `git add -A` and `git commit -a` are denied with a message naming the fix
- [x] Editing a tracked file on `main` is denied; on a branch it is allowed
- [x] Editing an **untracked** file is allowed even on `main` (no false positive)
- [x] The Stop hook warns and **never** blocks, even when everything is clean
- [x] A DONE task with no evidence fails `make check`; a reasoned N/A passes
- [x] The evidence test has fixtures asserting its FAILURE path — the old gate's
      absence of exactly this is why it rotted
- [x] No second implementation of the evidence check survives
- [x] L1–L3 green

## Session Protocol

1. `cm context "<task>"`
2. Read `CLAUDE.md`, `docs/PROGRESS.md`, this file
3. Mark **IN PROGRESS** before the first code change
4. L1–L3 + shux L4 before DONE

## Visual Test Results

Captured with `.shux/scripts/gates-709-evidence.sh`. What is under test here is
behaviour, not rendering, so the frames drive the hooks with real Claude Code hook
JSON and show the verdicts — including what each guard deliberately lets through.
Frames in `.shux/out/709/` (gitignored; attached to the PR):

| Frame | Shows |
|---|---|
| `01-staging-guard` | `git add -A/./-am`, and `cd x && git add -A`, all BLOCKED; `git add <path>` and `echo git add -A` allowed |
| `02-branch-guard` | tracked-on-`main` BLOCKED, untracked-on-`main` allowed, tracked-on-branch allowed, plus the message it prints |
| `03-evidence-test` | the evidence check and all ten of its failure fixtures passing |

The `Stop` hook is not framed: it prints only when something is left running, and
was verified live instead — silent and exit 0 when clean, and against a real shux
daemon it reported the runtime dir with the exact `shux daemon stop` command,
still exit 0. It must never block.

One finding from reading the frames back: frame 02's first capture reported
`allowed` for the case that demonstrably blocks. Nested quoting inside the frame
command had broken the comparison — the guard was correct, the evidence was not.
The logic now lives in `.shux/scripts/gates-709-branch-demo.sh` as a real script.
A frame that states the opposite of the truth is worse than no frame.
