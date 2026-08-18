# Task 706: Bump the default Grok model to `grok-4.6`

## Status: DONE

## Depends On
704 (grok provider)

## Problem

`grok` 1.0.5 now ships `grok-4.6` as its own default; दूतसभा still pinned
`grok-4.5`. Because `-m` is a **pinned** flag (704), दूतसभा was actively forcing
every grok call onto the older model — the user could not get 4.6 even by
configuring the CLI, only by editing `providers.grok.model`.

Two things made this riskier than a one-constant edit:

- The literal appeared in **six** places — the provider constant, the viper
  default, the plugin's `DefaultModel` *and* `SupportedModels[0]`,
  `configs/default.yaml`, and `internal/plugin/context_file.go`. Unit tests
  covered the constant and the viper default, but **`configs/default.yaml` was
  guarded by nothing** — breaking it alone failed zero tests. And while L5 drove
  grok through the binary extensively, it did so only on **failure** paths, so no
  integration test exercised model forwarding at all.
- `grok-4.5` is **not retired** (`grok models` still lists it). Unlike the
  gemini→agy sunset (703) this must not migrate anyone: a user who pinned 4.5
  keeps 4.5.

## PRD Ref
§6.1 (exit codes unchanged), §7 (provider config)

## Files

| File | Change |
|---|---|
| `internal/providers/grok.go` | exported `GrokDefaultModel` = `grok-4.6`; blank-effort fallback; attached short-flag stripping |
| `internal/core/config.go` | viper default → `grok-4.6`; effort comment |
| `plugins/grok/main.go` | reads `providers.GrokDefaultModel`; `SupportedModels` keeps 4.5 |
| `configs/default.yaml` | model + `xhigh` in the effort comment |
| `internal/plugin/context_file.go` | extension-context model reads the constant (LIVE path — extensions read this JSON) |
| `testdata/mock-providers/mock-grok` | **echoes the `-m` it receives** as `<model>-build` |
| `scripts/test-binary.sh` | L3: end-to-end forwarding + pin-survival |
| `scripts/test-agent-workflow.sh` | L5: grok success path (only failure paths existed) |
| `internal/providers/grok_model_test.go` | NEW — drift guard, pin guard, xhigh passthrough |
| `plugins/grok/capabilities_test.go` | NEW — plugin capability assertions |
| `internal/providers/grok_test.go`, `grok_internal_test.go`, `internal/core/grok_config_test.go` | assertions → 4.6; empty-effort cases |
| `README.md`, `skill/SKILL.md`, `skill/references/command-reference.md`, `docs/PROGRESS.md` | docs |
| `docs/grok-cli-findings.md` | dated addendum; 704 body untouched |

## Steps

1. Verify against the real binary and docs.x.ai — never infer a model id.
2. RED: flip the default assertions; add plugin-capability and empty-effort tests.
3. GREEN: bump the four sources; fix `--reasoning-effort=` forwarding an empty value.
4. Make `mock-grok` echo its `-m`, so integration tests can see forwarding at all.
5. Add L3 end-to-end + pin-survival and L5 success-path coverage.
6. Docs, addendum, grep gate.

## Verification

```
make ci-fast · make test · make test-binary · make test-plugins · make test-agent
```

L4 is captured by `.shux/scripts/grok-4-6-evidence.sh` (stills) and
`.shux/scripts/grok-4-6-video.sh` (motion), following the `issue-20-evidence.sh`
precedent — real `grok` 1.0.5 on grok-4.6, no mock providers in any frame.
`make test-visual` covers the iTerm2 L4 suite, including the `status` model-column
check in `.claude/automations/test_dootsabha_status.py`, which now expects grok.

Plus parallel adversarial agents driving the real binary — they found two real
defects (blank-effort forwarding, attached short-flag smuggling), both fixed with
regression tests.

## Done Criteria

- [x] `grok models` default (`grok-4.6`) == दूतसभा's shipped default
- [x] Two of the four sources deduplicated into `providers.GrokDefaultModel`; the
      other two (viper default, YAML skeleton) fail a drift test when broken alone
- [x] `--config /dev/null` → `consult --agent grok` reports `grok-4.6-build`
- [x] A pinned `providers.grok.model: grok-4.5` still yields `grok-4.5-build`
- [x] A *malformed* pin (non-string `model`/`binary`) is a loud config error,
      not a silent fallback to the bumped default
- [x] `xhigh` reaches argv in all four spellings; every blank spelling falls back to `high`
- [x] `SupportedModels` still offers 4.5 — it is a live model
- [x] L1–L5 green; adversarial agents find nothing unaddressed
- [x] L4 evidence from the REAL grok CLI doing real work (no mocks in frames)

## Commit

```
feat(grok): bump the default model to grok-4.6

grok 1.0.5 ships grok-4.6 as its own default; -m is a pinned flag, so दूतसभा was
forcing every call back onto grok-4.5. Not a migration: grok-4.5 is still live,
stays in SupportedModels, and an explicit pin is never rewritten.

The literal appeared in six places. GrokDefaultModel is now the single source of
truth the plugin and extension context read; the viper default and the YAML
skeleton are covered by a drift test (the skeleton previously failed nothing).
mock-grok echoes the -m it receives, so L3/L5 can see forwarding at all.
```

## Session Protocol

1. `cm context`
2. Read `CLAUDE.md`, `docs/PROGRESS.md`, this file
3. Mark this task **IN PROGRESS** before any code change
4. Work the TDD steps in order, capturing each RED
5. L1–L5 + adversarial agents + shux L4 before DONE

## Visual Test Results

Captured with `.shux/scripts/grok-4-6-evidence.sh` — real `grok` 1.0.5, real work,
no mocks. Frames in `.shux/out/706/final/` (gitignored; attached to the PR):

| Frame | Shows |
|---|---|
| `01-grok-models` | the premise — the CLI's own default is `grok-4.6`, with `grok-4.5` still listed |
| `02-status` | `status` reports grok `1.0.5` / `grok-4.6`, healthy, alongside the other three |
| `03-consult-default` | a real answer about this repo; footer `grok-4.6-build` |
| `04-consult-json` | JSON contract: `grok-4.6-build`, real tokens, real cost, session id |
| `05-pin-4-5` | a pinned `grok-4.5` still yields `grok-4.5-build` — not a migration |
| `06-council` | 3-agent council (codex, agy, grok) with grok as chair |
| `07-review` | grok-4.6 reviewing this branch and finding the drift-guard overclaim |
| `08-review-after` | grok-4.6 re-reviewing after the fixes |
| `grok-4-6-council.mp4` | live council, 2160×1292 |
