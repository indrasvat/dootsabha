# Task 702: Provider Default Model Refresh

## Status: DONE

## Depends On
- Task 105: Codex + Gemini Providers
- Task 501: Default Config + Embedded Docs

## Problem
The repo still defaults provider models that have moved forward in upstream
vendor docs.

Per current official OpenAI docs, `gpt-5.4` is the GPT-5 flagship starting
point for complex reasoning and coding. Per current official Google Gemini API
docs, the current Gemini 3 Pro API model string is `gemini-3.1-pro-preview`.

Both the Codex and Gemini providers originally reported configured models
without reliably passing them through to the underlying CLI subprocess, so
changing config alone did not guarantee runtime behavior matched the configured
defaults.

## Files
| File | Action |
|------|--------|
| `internal/core/config.go` | Update built-in Codex default model |
| `internal/providers/codex.go` | Pass configured/overridden model to `codex exec` |
| `internal/providers/gemini.go` | Pass configured/overridden model to `gemini` |
| `plugins/codex/main.go` | Update plugin capability metadata |
| `plugins/gemini/main.go` | Update plugin capability metadata |
| `configs/default.yaml` | Update documented default config |
| `README.md` | Update user-facing examples/defaults |
| `commands/dootsabha.md` | Update command examples |
| `skill/SKILL.md` | Update bundled skill examples |
| `skill/examples/council-deliberation.md` | Update example output |

## Steps
1. Confirm the target model strings against official OpenAI and Gemini docs
2. Update the Codex and Gemini defaults in config and plugin metadata
3. Fix the Codex and Gemini invocation paths so models are actually passed through
4. Update tests and docs that assert the old models
5. Verify with `make check` and real binary runs

## Done Criteria
- [x] Default Codex model is `gpt-5.4`
- [x] `codex exec` receives the configured model when no override is provided
- [x] Codex `--model` override still wins over the configured default
- [x] Default Gemini model is `gemini-3.1-pro-preview`
- [x] `gemini` receives the configured model when no override is provided
- [x] Gemini `--model` override still wins over the configured default
- [x] Docs/examples reflect both refreshed defaults
- [x] `make check` passes after the Gemini change
- [x] Real binary validation confirms the visible Gemini default is refreshed

## Visual Test Results
- `./bin/dootsabha status --json` reported Codex as healthy with model `gpt-5.4`
- `./bin/dootsabha consult --agent codex --json "Say exactly PONG"` succeeded and returned `"Model": "gpt-5.4"`
- `DOOTSABHA_PROVIDERS_CODEX_MODEL=definitely-invalid-model ./bin/dootsabha consult --agent codex "Say exactly PONG"` failed with the real Codex 400 invalid-model error, proving configured-model passthrough
- `./bin/dootsabha council --json --agents codex,gemini --chair codex "Write a Go function func Add(a, b int) int and mention one table-driven test case."` succeeded; Codex dispatch and Codex chair synthesis both used `gpt-5.4`
- `./bin/dootsabha review --json "Write a Go function func Add(a, b int) int and keep the answer concise."` succeeded; Codex author used `gpt-5.4`
- `./bin/dootsabha refine --json "Write a Go function func Clamp(n, min, max int) int and keep it concise."` succeeded; Codex review stage completed successfully inside the real refine pipeline
- `./bin/dootsabha status --json` reported Gemini as healthy with model `gemini-3.1-pro-preview`
- `./bin/dootsabha consult --agent gemini --json "Say exactly PONG"` succeeded and returned `"Model": "gemini-3.1-pro-preview"`
- `DOOTSABHA_PROVIDERS_GEMINI_MODEL=definitely-invalid-model ./bin/dootsabha consult --agent gemini "Say exactly PONG"` failed with Gemini’s real `ModelNotFoundError`, proving configured-model passthrough
- `./bin/dootsabha council --json --agents codex,gemini --chair codex "Write a Go function func Multiply(a, b int) int and mention one table-driven test case."` succeeded; Gemini dispatch used `gemini-3.1-pro-preview`
- `./bin/dootsabha review --json --author codex --reviewer gemini "Write a Go function func Add(a, b int) int and keep the answer concise."` succeeded; Gemini reviewer used `gemini-3.1-pro-preview`
- `./bin/dootsabha refine --json "Write a Go function func Clamp(n, min, max int) int and keep it concise."` succeeded after the Gemini refresh; Gemini review stage completed successfully inside the real refine pipeline
