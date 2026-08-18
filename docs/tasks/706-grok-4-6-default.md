# Task 706: Bump the default Grok model to `grok-4.6`

## Status: IN PROGRESS

## Depends On
704 (grok provider)

## Problem

`grok` 1.0.5 now ships `grok-4.6` as its own default; दूतसभा still pinned
`grok-4.5`. Because `-m` is a **pinned** flag (704), दूतसभा was actively forcing
every grok call onto the older model — the user could not get 4.6 even by
configuring the CLI, only by editing `providers.grok.model`.

Two things made this riskier than a one-constant edit:

- The default is declared in **four unsynced places** — `grokDefaultModel`,
  the viper default, the plugin's `Capabilities`, and `configs/default.yaml` —
  and **nothing drove grok through the binary**, so a partial bump would have
  been invisible to every existing test.
- `grok-4.5` is **not retired** (`grok models` still lists it). Unlike the
  gemini→agy sunset (703) this must not migrate anyone: a user who pinned 4.5
  keeps 4.5.

## PRD Ref
§6.1 (exit codes unchanged), §7 (provider config)

## Files

| File | Change |
|---|---|
| `internal/providers/grok.go` | `grokDefaultModel` → `grok-4.6`; empty-`=`-effort fallback fix |
| `internal/core/config.go` | viper default → `grok-4.6`; effort comment |
| `plugins/grok/main.go` | `DefaultModel` → `grok-4.6`; `SupportedModels` → `["grok-4.6","grok-4.5"]` |
| `configs/default.yaml` | model + `xhigh` in the effort comment |
| `internal/plugin/context_file.go` | extension-context example model |
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
plus parallel adversarial agents driving the real binary, and shux L4 frames of
real `grok-4.6` work.

## Done Criteria

- [ ] `grok models` default (`grok-4.6`) == दूतसभा's shipped default
- [ ] All four default sources agree, guarded by a test that fails on drift
- [ ] `--config /dev/null` → `consult --agent grok` reports `grok-4.6-build`
- [ ] A pinned `providers.grok.model: grok-4.5` still yields `grok-4.5-build`
- [ ] `xhigh` reaches argv in all four spellings; empty `=` value falls back to `high`
- [ ] `SupportedModels` still offers 4.5 — it is a live model
- [ ] L1–L5 green; adversarial agents find nothing unaddressed
- [ ] L4 evidence from the REAL grok CLI doing real work (no mocks in frames)

## Visual Test Results

_Pending — L4 shux capture._
