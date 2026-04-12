# Task 702: Codex Default Model (`gpt-5.4`)

## Status: DONE

## Depends On
- Task 105: Codex + Gemini Providers
- Task 501: Default Config + Embedded Docs

## Problem
The repo still defaults Codex to `gpt-5.3-codex`.

Per current official OpenAI docs, `gpt-5.4` is the flagship model for complex
reasoning and coding, so the app default should move forward accordingly.

The current Codex provider also reports a configured model without passing it to
the `codex exec` subprocess, so changing the config alone does not guarantee the
runtime actually uses the configured default.

## Files
| File | Action |
|------|--------|
| `internal/core/config.go` | Update built-in Codex default model |
| `internal/providers/codex.go` | Pass configured/overridden model to `codex exec` |
| `plugins/codex/main.go` | Update plugin capability metadata |
| `configs/default.yaml` | Update documented default config |
| `README.md` | Update user-facing examples/defaults |
| `commands/dootsabha.md` | Update command examples |
| `skill/SKILL.md` | Update bundled skill examples |
| `skill/examples/council-deliberation.md` | Update example output |

## Steps
1. Confirm the target model against official OpenAI docs
2. Update the Codex default model in config and plugin metadata
3. Fix the Codex invocation path so the model is actually passed through
4. Update tests and docs that assert the old model
5. Verify with `make ci`, `make test-binary`, and a real binary run

## Done Criteria
- [x] Default Codex model is `gpt-5.4`
- [x] `codex exec` receives the configured model when no override is provided
- [x] `--model` override still wins over the configured default
- [x] Docs/examples reflect the new default
- [x] `make ci` passes
- [x] `make test-binary` passes
- [x] Real binary validation confirms the visible default is `gpt-5.4`

## Visual Test Results
- `./bin/dootsabha status --json` reported Codex as healthy with model `gpt-5.4`
- `./bin/dootsabha consult --agent codex --json "Say exactly PONG"` succeeded and returned `"Model": "gpt-5.4"`
- `DOOTSABHA_PROVIDERS_CODEX_MODEL=definitely-invalid-model ./bin/dootsabha consult --agent codex "Say exactly PONG"` failed with the real Codex 400 invalid-model error, proving configured-model passthrough
- `./bin/dootsabha council --json --agents codex,gemini --chair codex "Write a Go function func Add(a, b int) int and mention one table-driven test case."` succeeded; Codex dispatch and Codex chair synthesis both used `gpt-5.4`
- `./bin/dootsabha review --json "Write a Go function func Add(a, b int) int and keep the answer concise."` succeeded; Codex author used `gpt-5.4`
- `./bin/dootsabha refine --json "Write a Go function func Clamp(n, min, max int) int and keep it concise."` succeeded; Codex review stage completed successfully inside the real refine pipeline
