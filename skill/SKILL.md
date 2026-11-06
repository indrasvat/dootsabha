---
name: dootsabha
description: >
  Get a second opinion or a review from other AI coding agents — codex, grok,
  agy (Antigravity) or claude — through the dootsabha CLI: one command, one JSON
  document on stdout, an exit code to branch on. Use to review a diff, branch or
  PR before merging; for an adversarial or convergence review; to validate a PRD,
  design or plan; to compare approaches; to ask "codex/grok, check my work"; or to
  see which agent CLIs are healthy. Triggers: dootsabha, दूतसभा, council, sabha,
  second opinion, cross-review, peer review, ask codex, ask grok, review with
  codex and grok, multi-agent review, refine with reviewers. Prefer it over
  calling `codex exec`, `grok -p` or `agy -p` for opinions and reviews; it is not
  for delegating implementation work. Councils take many minutes: always name
  --agents and run them in the background with --progress-file.
allowed-tools: Bash(dootsabha:*), Bash(./bin/dootsabha:*)
---

# दूतसभा — other agents' opinions, as JSON

## 1. Pick the command

| You want | Run | The answer is at |
|---|---|---|
| One agent's view | `dootsabha consult --agent codex --json "…"` | `.data.Content` |
| Several views, cross-reviewed, merged | `dootsabha council --agents codex,grok --chair codex --json "…"` | `.synthesis.content` |
| Which agents work right now | `dootsabha status --json` | `.data[] \| select(.Healthy) \| .Name` |
| A reviewed draft: A writes, B critiques | `dootsabha review --author codex --reviewer grok --json "…"` | `.review.content` (draft: `.author.content`) |
| A draft revised through each reviewer in turn | `dootsabha refine --author codex --reviewers grok,agy --json "…"` | `.final.content` |

Always `--json`: stdout is exactly one JSON document, on failure too.

**Name the agents.** Without `--agents`, a council runs the config's default
(`codex,agy` inside Claude Code), and `grok` joins only when named. When a run
exits 5 because an agent failed on quota or auth (`.meta.providers` says
`error`; `.dispatch[].error` says why), **drop it from the next run's
`--agents`** — repeating it gives a one-voice council every time.

Reading the wrong field prints `null` and exits 0. `consult`/`status` wrap the
answer as `{meta, data}` with **PascalCase** fields (a failed consult has
`.data.error`); the others are top-level, **snake_case**. When a council's
synthesis failed (`.synthesis == null`), the agents' own answers are still there:

```bash
jq -r '.synthesis.content // (.dispatch[] | select(.error == null) | .content)' result.json
```

A council's parts: `.dispatch[]` (`provider`, `content`, `error`) is each agent's
own answer; `.reviews[]` (`reviewer`, `reviewed`, `content`) is the peer review;
`.meta` holds `providers`, durations, tokens and `total_cost_usd`.

**Need a flag or field not shown here** (`--model`, `--rounds`, `--parallel=false`,
`--anonymous=false`, refine's `.versions[]`)? Open
[references/command-reference.md](references/command-reference.md) and jump to
that command's section from its table of contents — don't read it whole.

## 2. Run it from Claude Code

A council takes minutes — a 30 KB review prompt has taken grok 15–22 minutes per
call — and prints nothing until it ends. The Bash tool stops at 10 minutes.

- **Run `council`, `refine`, and anything with grok in the background**
  (`run_in_background: true`), redirecting stdout to a file, with
  `--progress-file`. Foreground is fine only for a `consult`/`review` whose
  `--timeout` is under 10m — past that the Bash tool kills it (exit 143) and the
  answer is lost.
- **Watch the progress file, not `ps`.** One JSON line per call start and finish
  (`stage`, `agent`, `status`, `duration_ms`); the last line of a finished run is
  `run_finished`, whose `exit_code` is the process's exit code.
  ```bash
  dootsabha council --agents codex,grok --chair codex --timeout 25m --json \
    --progress-file run.ndjson "$(cat prompt.md)" > result.json
  # Monitor: tail -n +1 -F run.ndjson | grep --line-buffered -E '"(call_finished|run_finished)"'
  ```
  Never sleep-poll, and never `ps | grep` for liveness: the agents' argv carries
  your prompt, so a substring match finds the wrong process. Open
  [references/progress-file.md](references/progress-file.md) only to parse other
  event fields, or when the process is gone and the file has no `run_finished`.
- **Budgets:** `--timeout` (default 15m) is per agent call; `--session-timeout`
  (default 75m) is the whole run. With grok and a large prompt, use `--timeout 25m`.
- **To cancel**, send SIGTERM to the dootsabha pid you started (`kill $pid`): it
  stops its agents, exits 143, and the progress file records it. Never
  `pkill -f "dootsabha …"` — that kills every other council running too.

## 3. Write the prompt

- The prompt is **one argument**. For anything long: `"$(cat prompt.md)"`.
- Agents run **in your current directory** and can read the repo. Point them at
  files and commits ("review `git diff main...HEAD`; read `internal/cli/`")
  instead of pasting everything.
- **They can also write.** codex, claude and agy run with full permissions in
  that directory; only grok is read-only. For a review, say "Do not modify any
  file", or run from a throwaway `git worktree add --detach /tmp/x <sha>`.
- Ask for findings in a shape you can act on: `file:line`, a concrete failing
  scenario, the fix, and a final `SHIP` or `FIX`.

## 4. Branch on the exit code

| Code | Means | Do |
|---|---|---|
| 0 | complete | use it |
| 5 | some agents failed, output usable | use it; the failures are in `.dispatch[].error` / `.meta.providers` |
| 4 | an agent ran out of time | the message names the budget (`invocation` → `--timeout`, `session` → `--session-timeout`); a council's other answers are still in `.dispatch` |
| 3 | every agent failed, nothing usable | read `.data.error` / `.dispatch[].error`; retry with another agent |
| 2 | bad command (flag, agent name, chair) | fix the command |
| 6 | config missing or invalid | fix the config |
| 1 | internal bug | report it |

If several apply, the highest wins: `2 > 6 > 4 > 3 > 5 > 1 > 0`. Writing a
script that branches on them: [references/exit-codes.md](references/exit-codes.md).

## 5. Agents

- **Never assume model names.** `dootsabha config show --json` shows what is in
  effect; a response's `Model` is the backend's id and can differ from it.
  To change a model, flag or default: `dootsabha config show --commented` lists
  every key with its meaning and value; any key can be set for one run as
  `DOOTSABHA_<KEY>` (dots → underscores, e.g. `DOOTSABHA_PROVIDERS_GROK_MODEL`).
- **`status` proves the CLI starts, not that it has quota.** A healthy agent can
  still fail mid-call on a rate limit — agy most often. Exit 3 or 5 names it;
  swap the agent.
- **grok:** opt-in, read-only, and the slowest: plan for it in budgets. If it
  fails with `sandbox could not be applied`, update dootsabha rather than
  calling `grok -p` directly — that bypasses the read-only sandbox.

## 6. Review loops

Re-review **only the fix commits** ("did each fix close its finding, what did
it break?"), never the whole diff again, and stop after a few rounds: a council
asked to find bugs always finds more.

## References — open only when the step above sends you there

| File | Open it when |
|---|---|
| [command-reference.md](references/command-reference.md) | you need a flag, field or config key not shown above |
| [progress-file.md](references/progress-file.md) | you parse progress events beyond `call_finished`/`run_finished`, or a run died without `run_finished` |
| [exit-codes.md](references/exit-codes.md) | you write a script that branches on exit codes |
