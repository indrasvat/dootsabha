# Task 707: Bump the default Antigravity model to `Gemini 3.7 Flash (High)`

## Status: IN PROGRESS

## Depends On
703 (agy provider)

## Problem

Google shipped **Gemini 3.7 Flash** on 2026-08-13 and made it the default model
behind the Antigravity agent. `agy` 1.1.17 lists all three of its effort tiers;
दूतसभा still pinned `Gemini 3.5 Flash (High)`.

`--model` is a flag दूतसभा *always* emits (`stripAgyModelFlags` removes any user
`--model` from `flags` and re-adds the resolved one), so like grok's `-m` in 706
this was not a soft default — दूतसभा was actively forcing every `agy` call onto
3.5. Configuring the Antigravity CLI itself did nothing.

Two things made this more than a one-constant edit:

- The literal appeared in **six** places, none of which kept each other in sync:
  `internal/providers/agy.go` (fallback config), `internal/core/config.go` (viper
  default), `internal/core/migrate.go` (`agyModel`, what a gemini→agy migration
  writes into a user's file), `plugins/agy/main.go` (`DefaultModel` **and**
  `SupportedModels`), `configs/default.yaml`, and
  `internal/plugin/context_file.go` (the LIVE JSON extensions read).
  `configs/default.yaml` and `context_file.go` were guarded by **nothing** —
  breaking either alone failed zero tests.
- `mock-agy` **discarded** `--model` entirely (`--model) shift 2 ;;`), so no L3 or
  L5 test could observe model forwarding at all. The one L3 assertion that
  mentioned a model only checked the mock still printed plain text.

## PRD Ref
§6.1 (exit codes unchanged), §7 (provider config), §5.3 (agy provider)

## Not a migration

`agy models` still lists 3.6 Flash, 3.5 Flash and 3.1 Pro. Unlike the
gemini→agy sunset (703) this migrates nobody: the older tiers stay in
`SupportedModels`, and an explicit `providers.agy.model` is never rewritten.

## Real-CLI findings (agy 1.1.17)

Verified by driving the binary, not inferred:

| Probe | Result |
|---|---|
| `--model "Gemini 3.7 Flash (High)"` | accepted |
| `--model "gemini-3.7-flash-high"` (the id from `agy models` col 1) | **also** accepted |
| `--model "not-a-real-model-xyz"` | exit 1, and the error lists **display names** |
| `--model "gemini 3.7 flash (high)"` | accepted — matching is case-insensitive |
| `--model "Gemini 3.7 Flash (Low)" --effort high` | **exit 1** — `--effort is not supported for model "…(Low)"` |
| `--effort xhigh` | exit 1 — `valid: low, medium, high` |

So the display-name suffix **is** the effort selector; `--effort` is not a
separate axis दूतसभा should start emitting. Keeping display names (rather than
switching to ids) also keeps the change additive: `status`, `config show` and the
PRD all already speak display names, and the CLI's own error message prints that
spelling back.

Separately: **`agy --output-format json` now works** (1.1.17 emits
`conversation_id`, `status`, `duration_seconds`, `num_turns`, `usage.*`). The
provider comment claiming it does not exist is stale. Consuming it is a real
feature with its own tests and is **out of scope here** — filed as task 708. This
task only stops the docs from asserting something false.

## Files

| File | Change |
|---|---|
| `internal/providers/agy.go` | exported `AgyDefaultModel` = `Gemini 3.7 Flash (High)`; stale-JSON comment corrected |
| `internal/core/config.go` | viper default reads `core.defaultAgyModel` |
| `internal/core/migrate.go` | `agyModel` reads the same `core` constant — one core-side source, not two |
| `plugins/agy/main.go` | reads `providers.AgyDefaultModel`; `SupportedModels` gains 3.7/3.6, keeps 3.5 |
| `configs/default.yaml` | model bumped; comment names the effort-suffix rule |
| `internal/plugin/context_file.go` | reads `providers.AgyDefaultModel` (LIVE path) |
| `testdata/mock-providers/mock-agy` | **echoes the `--model` it receives**; sentinel default |
| `scripts/test-binary.sh` | L3: end-to-end forwarding + pin-survival |
| `scripts/test-agent-workflow.sh` | L5: agy model forwarding through the binary |
| `internal/providers/agy_model_test.go` | NEW — drift guard, pin guard, migrate guard |
| `plugins/agy/capabilities_test.go` | NEW — plugin capability assertions |
| `internal/providers/agy_test.go`, `internal/core/config_test.go`, `internal/core/migrate_test.go`, `internal/plugin/context_file_test.go`, `internal/cli/config_cmd_test.go` | assertions → 3.7 |
| `.claude/automations/test_dootsabha_status.py` | model column expects `Gemini 3.7 Flash` |
| `README.md`, `skill/SKILL.md`, `skill/references/command-reference.md`, `skill/examples/council-deliberation.md`, `commands/dootsabha.md`, `docs/PRD.md`, `docs/PROGRESS.md` | docs |
| `docs/agy-cli-findings.md` | NEW — dated findings from the real binary |
| `docs/tasks/708-agy-json-output.md` | NEW — the deferred JSON work, PENDING |

## Steps

1. Probe the real binary + official docs — never infer a model id.
2. RED: flip the default assertions; add drift/pin/capability tests.
3. GREEN: collapse six sites to three sources (provider const, core const, YAML).
4. Make `mock-agy` echo its `--model`, so integration tests can see forwarding.
5. Add L3 forwarding + pin-survival and L5 forwarding coverage.
6. Docs, findings doc, follow-up task, grep gate.

## Verification

```
make ci-fast · make test · make test-binary · make test-plugins · make test-agent
```

L4 is captured by `.shux/scripts/agy-3-7-evidence.sh` (stills) and
`.shux/scripts/agy-3-7-video.sh` (motion), following the `grok-4-6-evidence.sh`
precedent — real `agy` 1.1.17 on Gemini 3.7 Flash, no mock providers in any frame.
`make test-visual` covers the iTerm2 L4 suite.

Plus parallel adversarial agents driving the real binary, and a `codex,agy,grok`
दूतसभा council reviewing the branch.

## Done Criteria

- [ ] `agy models` top entry == दूतसभा's shipped default
- [ ] Three of the six sites read `providers.AgyDefaultModel`; the core-side
      viper default and migration share one `core` constant; the YAML skeleton
      fails a drift test when broken alone
- [ ] `--config /dev/null` → `consult --agent agy` forwards `Gemini 3.7 Flash (High)`
- [ ] A pinned `providers.agy.model` still yields that pin — bump, not migration
- [ ] `config migrate` writes the bumped model, not the stale one
- [ ] `SupportedModels` still offers 3.6/3.5/3.1 — they are live models
- [ ] L1–L5 green; adversarial agents find nothing unaddressed
- [ ] L4 evidence from the REAL agy CLI doing real work (no mocks in frames)

## Commit

```
feat(agy): bump the default model to Gemini 3.7 Flash (High)

Google shipped Gemini 3.7 Flash on 2026-08-13 and made it Antigravity's own
default; agy 1.1.17 lists all three effort tiers. dootsabha always emits
--model, so it was forcing every call back onto 3.5. Not a migration: 3.6/3.5/
3.1 stay live, stay in SupportedModels, and an explicit pin is never rewritten.

The literal appeared in six unsynced places. AgyDefaultModel is now the source
the plugin and the extension context read, and the viper default and the
migration writer share one core constant; a drift test covers the YAML skeleton,
which previously failed nothing. mock-agy echoes the --model it receives, so
L3/L5 can see forwarding at all.
```

## Session Protocol

1. `cm context`
2. Read `CLAUDE.md`, `docs/PROGRESS.md`, this file
3. Mark this task **IN PROGRESS** before any code change
4. Work the TDD steps in order, capturing each RED
5. L1–L5 + adversarial agents + shux L4 before DONE

## Visual Test Results

_Pending — captured with `.shux/scripts/agy-3-7-evidence.sh`._
