# Task 713: Make the skill lead to the right call, fast

## Status: DONE

## Depends On
712 (the progress file the skill now leads with).

## Problem

The skill loads in 13 of 14 sessions that use dootsabha from other repos, but
over four months its references were opened twice and its examples never: only
the SKILL.md body is ever read, and it loads once, so it can drop out of context.
In those sessions the same failures recurred with the skill loaded:

- a foreground council killed at the Bash tool's 10-minute cap (exit 143);
- about 180 `ps`/`pgrep`/sleep checks on runs in flight; `--progress-file` unused;
- ~15 consecutive one-voice councils on an agy out of quota (default agents);
- guessed fields: `.result.content`, `.final // .final_content`, `['data']` on a council;
- a council cancelled with `pkill -f "dootsabha council"`, killing any other;
- grok sandbox errors driving 59 direct `grok -p` / `codex exec` calls.

The body also had defects of its own: one extraction row sent `review` and
`refine` to `.synthesis.content` (null for both), it never said that codex,
claude and agy can write in the caller's directory, and the skill and its
references hardcoded model names and dates that had gone stale.

## Design

Progressive disclosure fitted to how the skill is actually read:

- **Description** (always in context): what it is for and not for (opinions and
  reviews, not delegation), trigger words, and the most-violated rule —
  name `--agents`, run councils in the background with `--progress-file`.
- **SKILL.md** (read once): the every-time path only — pick the command and the
  field holding its answer; run it from Claude Code (background, watch the
  file, budgets, cancel); write the prompt (one argument, agents can read *and
  write* the repo); branch on the exit code; agent caveats; review-loop scope.
- **References** (opened only when a step sends you): each is linked at its
  point of need with "open when …". Config keys come from the live CLI
  (`config show --commented`), not a list that can go stale.
- **Examples removed**: never opened; their unique content (`.dispatch[]`,
  `.reviews[]`) is now in SKILL.md.
- **No model names or dates** anywhere in the skill or `/dootsabha` command.

## Files

| File | Change |
|---|---|
| `skill/SKILL.md` | rewritten: 158 → ~135 lines, description 448 → 841 chars |
| `skill/references/command-reference.md` | no model column; write access per agent; grok timing from real runs; config keys via `config show --commented` |
| `skill/examples/*` | removed |
| `commands/dootsabha.md` | model names → `<model>` |

## Visual Test Results

N/A — docs-only: no command's output changed. Evidence for the change:

- Every extraction path in the new command table was run against real output
  (consult, council, review, refine, status, the failure envelope, the
  synthesis fallback) and returned the answer.
- Blind eval, as the skills docs prescribe: two Haiku agents, given only the old
  or the new skill and the same four tasks drawn from the failures above, graded
  on a rubric fixed before reading either answer. Old **13/17** (no timeout for a
  large grok review, nothing stopping agents writing the tree, "the skill does
  not give the field" for refine's answer). New **17/17**. Neither opened a
  reference; none of the four tasks needed one. Both flagged that the skill never
  named `run_finished.exit_code` — fixed.
