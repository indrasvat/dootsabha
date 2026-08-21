# Task 703: Replace Gemini Provider with Antigravity (agy)

**Status:** DONE
**Depends On:** 105 (codex+gemini providers), 302 (extract providers)
**Parallelizable With:** —

## Problem

Google is sunsetting the **Gemini CLI** on **June 18, 2026** — it stops serving
Google AI Pro/Ultra requests on that date. The official replacement is the
**Antigravity CLI** (`agy`), a Go-built, agent-first, multi-model CLI. दूतसभा must
drop the dead `gemini` provider and integrate `agy` as the third council agent.

Sources:
- https://developers.googleblog.com/an-important-update-transitioning-gemini-cli-to-antigravity-cli/
- https://www.theregister.com/ai-ml/2026/05/20/bye-bye-gemini-cli-google-nudges-devs-toward-antigravity/

## Key Findings (agy 1.0.8, verified on real binary)

- Binary: `agy` (installed at `~/.local/bin/agy`).
- **Print mode:** `agy --dangerously-skip-permissions -p "<prompt>"` → plain text on
  stdout, exit 0. No `--output-format`/JSON option exists.
- **No structured output:** unlike `gemini` (which emitted a JSON stats envelope),
  `agy` print mode gives plain text only — **no session ID, token counts, or cost**.
  The provider populates `Content` + `Duration`; tokens/cost stay 0.
- **Model flag:** `--model "Gemini 3.5 Flash (High)"` — display-name identifiers, matching
  `~/.gemini/antigravity-cli/settings.json`. Available models: Gemini 3.5 Flash
  (Low/Medium/High), Gemini 3.1 Pro (Low/High), Claude Sonnet 4.6, Claude Opus 4.6,
  GPT-OSS 120B.
- **Version:** `agy --version` → `1.0.8` (bare version, `parseVersion` handles it).
- **Default model chosen:** `Gemini 3.5 Flash (High)` (agy's own settings default; fast).

## Decision

**Replace `gemini` entirely.** `agy` becomes the 3rd default agent (`claude,codex,agy`).
The `gemini` provider name, plugin, config block, mock, and color are removed.

## Files

- `internal/providers/agy.go` (new; replaces `gemini.go`) — plain-text provider
- `internal/providers/agy_test.go` (new; replaces `gemini_test.go`)
- `internal/providers/types.go` — doc comment
- `internal/core/config.go` — defaults, `defaultProviderNames`, comment map
- `internal/core/config_test.go` — expectations
- `internal/cli/consult.go` — `getProvider`, `providerColor`, error message
- `internal/cli/council.go` — default agents (`claude,codex,agy` / `codex,agy`)
- `internal/cli/root.go` — long description
- `internal/output/styles.go` — `GeminiColor` → `AgyColor`
- `configs/default.yaml` — `gemini` block → `agy` block
- `plugins/agy/main.go` (new; replaces `plugins/gemini/main.go`)
- `Makefile` — plugin build target
- `testdata/mock-providers/mock-agy` (new; replaces `mock-gemini`)
- `scripts/test-binary.sh`, `scripts/test-plugin-smoke.sh`, `scripts/test-agent-workflow.sh`
- Test fixtures referencing `gemini` across `internal/**/*_test.go`
- `README.md`, `skill/**`, `docs/PROGRESS.md`

## Verification

- L1 `make ci-fast`, L2 `make test`, L3 `make test-binary` must pass.
- L4 visual tests for status/council where output-visible.
- Real `agy` smoke: `dootsabha consult --agent agy "Reply with OK"`.

## Criteria

- No `gemini` references remain in code/config/scripts (docs note the migration).
- `dootsabha status` shows `agy` healthy.
- `dootsabha council "..."` uses `claude,codex,agy` by default.
