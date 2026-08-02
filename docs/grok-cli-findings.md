# Grok CLI — Verified Integration Findings (detail doc for Task 704)

Evidence backing `docs/tasks/704-grok-provider.md`. Every claim here was captured
from the real `grok` binary (0.2.117 → 0.2.118, macOS arm64, 2026-08-01), not inferred.
Screenshots proving these live in `.shux/out/grok-provider/` (gitignored — attached to
the PR as review evidence, not committed).

## Key Findings (grok 0.2.117 → 0.2.118, verified on the real binary)

> The binary **self-updated 0.2.117 → 0.2.118 mid-research**, which is itself the argument
> for `--no-auto-update`: without it, a council run can mutate its own toolchain between
> agents. Findings below were captured across both builds; nothing observed differed.

Evidence: `.shux/out/grok-provider/` (frames) and the raw captures referenced below.

- **Model:** `grok models` → `grok-4.5` is the only available model and the default.
  The `modelUsage` key in responses is **`grok-4.5-build`** — deliberately *not* the same
  string as the `-m` value. Do not assume they match.
- **Reasoning effort is a FLAG, not part of the model ID** (contrast agy's
  `Gemini 3.5 Flash (High)`). `--reasoning-effort` accepts exactly **`high|medium|low`**,
  verified by feeding an invalid value:
  `--effort/--reasoning-effort: unknown effort level 'ultramax'; use one of: high, medium, low`
- **`grok -p` is the headless contract — NOT `grok agent`.** Every `grok agent`
  subcommand (`stdio`/`headless`/`serve`/`leader`) is a long-lived ACP server for IDE/SDK
  embedding; `headless` runs over the Grok WebSocket relay. `-p/--single` is the documented
  "print to stdout and exit" path (`~/.grok/docs/user-guide/14-headless-mode.md`).
- **Verified invocation** (exit 0, pure JSON on stdout, **0 bytes** on stderr):

```
grok -p <PROMPT> --output-format streaming-messages-json --always-approve \
     --permission-mode bypassPermissions -m grok-4.5 --reasoning-effort high \
     --no-plan --no-subagents --no-auto-update --sandbox read-only \
     --max-turns <N> --cwd <DIR>
```
  …run with `HOME=<empty dir>` and `GROK_HOME=<real ~/.grok>` (see R1 solve).

  | Flag | Why it is required |
  |---|---|
  | `--output-format streaming-messages-json` | **NOT `json`** — see the `.text` trap below |
  | `--sandbox read-only` | containment + big token saving — see R5 |
  | `--always-approve` + `--permission-mode bypassPermissions` | full-access equivalent; belt-and-braces, independent of user config |
  | `-m grok-4.5` | **must** pin — the user's `config.toml` default is `grok-composer-2.5-fast`, a model that no longer exists (R3) |
  | `--reasoning-effort high` | pin explicitly rather than inherit (`high` is already grok-4.5's default, but do not rely on it) |
  | `--no-plan` | plan mode would return a *plan* instead of the answer — correctness-critical |
  | `--no-subagents` | removes cost, latency, nondeterminism from a single-shot consult |
  | `--no-auto-update` | no surprise network call / self-mutation mid-review (works, though absent from `--help`) |
  | `--max-turns` / `--cwd` | bound runaway loops; explicit working directory |

  **`--verbatim` is deliberately NOT passed.** Its name misleads. Source-authoritative
  (`xai-org/grok-build`, `prompt_parser.rs:19-21` + `metadata_types.rs::prompt_verbatim`):
  it *only* controls whether the query is wrapped in `<user_query>…</user_query>` tags.
  grok does **not** rewrite or expand prompts at all, so दूतसभा's "send my prompt
  unmodified" goal is already satisfied by default — and stripping the delimiter that
  grok's own system prompt is written against would more likely *degrade*
  instruction-following. It also does **not** suppress `CLAUDE.md`/skill injection
  (separate assembly slots). Record this so a future reviewer does not "helpfully" add it.

- **Context window is 500,000 tokens** (`api_backend: "responses"`) — दूतसभा's 32 KB
  prompt truncation is far inside the limit.
- **Doc authority order:** `~/.grok/docs/user-guide/*.md` (vendored from upstream, current)
  > `grok --help` > ~~bundled `README.md`~~ > ~~`docs.x.ai/build/cli/headless-scripting`~~.
  The last two are **stale** and disagree with the shipped binary. Do not cite them.
- **Documented exit codes:** `0` success, `1` error, `130` SIGINT, `143` SIGTERM. There is
  **no** distinct code for timeout/provider/partial. Note `2` is also observable for
  clap-level argument errors (before grok's own handling). दूतसभा maps these onto its own
  precedence (`2 > 4 > 3 > 5 > 1 > 0`) in the CLI layer, not the provider.

- **Richest telemetry of any provider** — content **plus** session id, full token breakdown
  *and* cost, beating codex (tokens only) and agy (none).

### ✅ THE SHAPE TO IMPLEMENT AGAINST — `streaming-messages-json`, last `result` line

NDJSON: **one JSON object per line**, not a pretty-printed document. Take the **last** line
with `type == "result"`. Real capture:

```json
{"type":"result","subtype":"success","is_error":false,
 "result":"# Critique: …",  "stop_reason":"end_turn",
 "session_id":"019fbc10-…","duration_ms":106781,"duration_api_ms":105084,"num_turns":1,
 "total_cost_usd":0.1049568,
 "usage":{"input_tokens":25956,"output_tokens":5724,
          "cache_read_input_tokens":62336,"cache_creation_input_tokens":0},
 "modelUsage":{"grok-4.5-build":{…}},"uuid":"…"}
```

Exactly these keys, verified: `duration_api_ms, duration_ms, is_error, modelUsage,
num_turns, result, session_id, stop_reason, subtype, total_cost_usd, type, usage, uuid`.
Note the **snake_case** — close to `claudeResponse`, so the struct mirrors `claude.go`.

### 🔴 Why NOT `--output-format json` — the `.text` trap

`--output-format json` looks ideal on a trivial prompt, but on any **real** review its
`.text` field **concatenates every assistant text block — tool-call preambles included —
with no separator**. Verified on a real `agy.go` review:

```
"I'll read `agy.go` and `types.go` carefully and review them for correctness, error
handling, edge cases, contract violations, and Go idioms.I'll compare sibling providers
and the PRD/docs so contract violations…# Code review: `agy.go`…"
```

Two preambles run together (`…Go idioms.` `I'll compare…`) before the answer starts. Any
provider parsing `.text` would emit that verbatim into council/review output.

**Use `--output-format streaming-messages-json` and take the last line with
`type == "result"`.** Verified clean — begins directly at `# Critique:` — and that line is
**exactly the Claude envelope shape** (snake_case, with a real `is_error` bool):

```
keys: type, subtype, is_error, result, stop_reason, session_id,
      duration_ms, duration_api_ms, num_turns, total_cost_usd, usage, modelUsage, uuid
```

⇒ `grokResult` mirrors `claudeResponse` closely (`is_error`/`result`/`session_id`/
`total_cost_usd`/`usage`/`modelUsage`), but is parsed as **NDJSON, last `result` line wins**
— not as a single object. The earlier camelCase `json`-mode envelope
(`text`/`stopReason`/`sessionId`) is **not** the shape to implement against.
- **Error contract in STREAMING mode** (verified — this is the shape that matters):

```json
{"type":"result","subtype":"error_during_execution","is_error":true,
 "duration_ms":0,"num_turns":0,"stop_reason":null,"total_cost_usd":0.0,
 "usage":{…zeros…},"modelUsage":{},
 "errors":["Couldn't set model 'no-such-model-xyz': Invalid params: …"],
 "session_id":"019fbc2d-…","uuid":"…"}
```

  ⚠ **A runtime error is still `type == "result"`.** Discriminate on **`is_error == true`**
  (and/or `subtype == "error_during_execution"`) — **not** on a `{"type":"error"}` object,
  which only appears in `--output-format json` mode. In the error case there is **no
  `result` field at all**; the message lives in the **`errors[]` array** (join it).

  | Case | shape | exit |
  |---|---|---|
  | success | `subtype:success`, `is_error:false`, `result:"…"` | 0 |
  | model/runtime error | `subtype:error_during_execution`, `is_error:true`, `errors:[…]`, no `result` | **1** |
  | bad flag | *(empty stdout)*, clap usage on stderr | **2** |

  **Error precedence:** provider failure must win over content — non-zero exit **or**
  `is_error:true` **or** a later error result appearing after content. Test mixed streams.
- **Piped stdout is clean** — `grep -c $'\x1b'` → 0. No ANSI, no spinner.
- **Nested sessions are fine.** grok ran successfully from *inside* a live Claude Code
  session, and `env | grep -i GROK` is empty. **No analogue of `core.DetectAndCleanClaude()`
  is needed — `internal/core/subprocess.go` gets ZERO changes.**
- **`parseVersion()` already works.** `grok --version` → `grok 0.2.117 (f1c06093089f)` →
  `0.2.117` via the existing helper at `internal/providers/claude.go:170`. No change.
- **⚠ Token/cost fields are OPTIONAL and subtle:**
  - `usage.input_tokens` is **uncached only** — it is *not* the full prompt size. The full
    picture needs `cache_read_input_tokens` + `cache_creation_input_tokens`, or `total_tokens`.
  - ⚠ **`total_cost_usd_ticks` does NOT exist in `streaming-messages-json`.** Verified: the
    streaming `result` line carries exactly `duration_api_ms, duration_ms, is_error,
    modelUsage, num_turns, result, session_id, stop_reason, subtype, total_cost_usd, type,
    usage, uuid`. The `_ticks` field is a `--output-format json`-only artefact — do not
    reference it.
  - `total_cost_usd` may be omitted upstream (`cost_is_partial`) — but **do not
    over-engineer this.** `ProviderResult.CostUSD` is a plain `float64` and cannot
    distinguish "unreported" from "genuinely zero". **Parse `total_cost_usd` when present,
    else leave 0, and claim no semantic distinction.** A nullable telemetry shape is a
    schema change and belongs in its own task, not here (additive constraint).
- **⚠ Cost floor ~$0.05/call** (~22–25 k input tokens for a one-word answer) — see R1.

## Risks (verified on the real binary)

**R1 — grok inherits Claude Code's ENTIRE harness (highest impact).** `grok inspect` in a
*fresh empty temp dir* reports **141 skills, 25 agents, 18 plugins, 6 MCP servers, 53
hooks**, plus `~/.claude/CLAUDE.md` injected as project instructions. Grok deliberately
reads `~/.claude/**`, `~/.claude.json`, and `.mcp.json` ("Claude Code Compatibility").
Consequences: the user's own hooks (RTK, DCG, peon-ping) fire inside a grok review; MCP
servers are spawned as child processes on **every** invocation; and the ~22 k-token floor
follows directly.

**🔒 It is also a privacy leak.** `~/.claude/CLAUDE.md` — the user's *private global
instructions* — is injected as project instructions (~686 tokens) and therefore **sent to
xAI on every grok invocation**. Observed directly: a review closed with *"If Ārya wants a
follow-up…"* even though that honorific appears nowhere in the reviewed material. It could
only have come from the global CLAUDE.md. This alone justifies the R1 solve.
*What does NOT work (both measured, do not retry):*
- **`GROK_*_ENABLED=false` compat env vars are inert in 0.2.117.** All 13 cells set to
  `false` (claude + cursor + codex) changed `grok inspect --json` by **exactly nothing**:
  141 skills / 25 agents / 18 plugins / 6 MCP / 53 hooks, identical to baseline.
- **`[compat.*]` blocks in a `config.toml` are equally inert.** An isolated `GROK_HOME`
  with every cell `false` still reported 118 Claude-sourced skills.

### ✅ R1 SOLVE — `HOME` isolation (verified end-to-end)

Claude-compat discovery is keyed off **`$HOME`**, not off grok's own config. So point
`HOME` at an empty directory while keeping `GROK_HOME` on the **real** `~/.grok` (which is
where auth lives):

```
HOME=<empty dir>  GROK_HOME=/Users/<user>/.grok  grok -p …
```

Measured, `grok inspect --json` and a real model call:

| | Baseline | Isolated | Δ |
|---|---|---|---|
| MCP servers spawned per call | 6 | **0** | eliminated |
| Claude hooks live | 53 | 0 (17 grok-own) | eliminated |
| Plugins / agents | 18 / 25 | 0 / 3 | eliminated |
| `CLAUDE.md` project instructions | 1 | **0** | eliminated |
| Skills | 141 | 32 (grok-own) | −109 |
| `usage.input_tokens` | 22 066 | **13 431** | **−39%** |
| `total_cost_usd` | 0.0459 | **0.0273** | **−40%** |

**Auth survives** — the isolated run returned exit 0 with a valid envelope in 2.99 s.
This removes the hook-firing and MCP-spawning hazards entirely *and* cuts cost 40%.

**Implementation:** build the env in `grokIsolatedEnv()` and pass via `core.WithEnv`
(`internal/core/subprocess.go:35`). Resolve the real grok home as `$GROK_HOME` if set,
else `$HOME/.grok`, **before** overriding `HOME`. Create the empty HOME once under
`os.TempDir()`.

*Residual caveats to document:* an empty `$HOME` also hides `~/.gitconfig` and similar
dotfiles from tools grok runs. For दूतसभा's single-shot read-and-review usage this is
acceptable — and arguably desirable — but it must be stated. If it ever bites, the refinement
is a minimal HOME seeded with selected dotfiles rather than an empty one.

**R2 — session pollution on disk (worse than it first looks).** Every `-p` run writes a
persistent session under `~/.grok/sessions/<url-encoded-cwd>/<uuid>/`. Measured across 7
headless runs: `~/.grok/sessions/` went **97 → 393 files (+296, ~4 MB)** — roughly **42
files per run**, not the ~2 initially estimated. **There is no `--no-session` flag**
(contrast `claude --no-session-persistence`). `grok sessions delete <id>` exists and the
envelope returns `session_id`, so reaping is possible. **Decision: document in v1, do not
implement reaping** (additive scope); revisit as a follow-up given the real growth rate.

**R3 — `config.toml` default model is stale.** `[models] default = "grok-composer-2.5-fast"`
no longer exists; runtime silently falls back. This is exactly why `-m grok-4.5` must be
passed explicitly. Also `[ui] permission_mode = "always-approve"` already yields
`bypassPermissions` — दूतसभा must not rely on that, pass the flags.

**R4 — bundled README contradicts runtime.** It lists 7 reasoning-effort values (runtime
accepts 3) and `"EndTurn"` (runtime emits `end_turn`). Trust
`~/.grok/docs/user-guide/*.md` and the runtime over the README.

**R5 — `--cwd` is NOT a sandbox (containment escape, observed).** With `--always-approve`
and no `--sandbox`, grok **escaped the working directory** during a real review: it ran
`find` across the whole session tree and read other agents' notes and prior review output.
`--cwd` sets the working directory; it restricts nothing.

*Solve:* pass **`--sandbox read-only`** — documented use case *"Exploration, code review"*,
which is precisely what दूतसभा asks grok to do. Reads anywhere, writes only to `~/.grok/`
and temp dirs, kernel-enforced (Seatbelt/Landlock) for the process lifetime and
irreversible at runtime. `workspace` (writes to CWD) is the alternative but grants write
access दूतसभा never needs — consult/council/review/refine only ever produce **text**.

*Verified compatible with the R1 HOME-isolation solve* (a plausible conflict, since the
sandbox whitelists `~/.grok` while `GROK_HOME` points at the real one — it works, exit 0).
The two compound dramatically:

| Config | `usage.input_tokens` | cost/call |
|---|---|---|
| Baseline | 22 066 | $0.0459 |
| + HOME isolation (R1) | 13 431 | $0.0273 |
| + `--sandbox read-only` (R5) | **3 322** | **$0.0102** |

**−85% tokens, −78% cost** versus a naive integration, plus containment. The sandbox also
strips write/edit tools from the injected system prompt, which is where the extra saving
comes from.

*Note:* grok being strictly read-only makes it **more constrained than its siblings**
(claude/codex/agy all run with full write access). That is deliberate and correct for
दूतसभा's text-producing commands — but it is a real difference and is called out here so
it is a conscious choice, not an accident.


## Timing, timeout, and why दूतसभा's usage is the cheap path

Measured on real reviews (8 runs):

| Run | Effort | Wall clock | Cost |
|---|---|---|---|
| Go code review | high | 125.6 s | — |
| same prompt | low | 78.4 s | $0.118 |
| task-spec critique | high | 105.6 s | $0.166 |
| PRD critique | high | 107.8 s | $0.105 |
| **17 KB prompt, all material inlined** | high | **24.8 s** (`num_turns: 1`) | $0.071 |

- **p50 for a real review at high reasoning ≈ 106 s**; worst observed 125.6 s.
- **Timeout: keep the existing 5 m (300 s) global default — no config change needed.**
  That is ~2.4× headroom over p50. Do **not** go below 180 s. This is an additive win:
  `timeout: 5m` in `configs/default.yaml` already suits grok.
- **high vs low reasoning:** 1.6× slower for *better prioritisation, not more findings* —
  the low-effort run surfaced the same top three defects. `high` is Ārya's call and is the
  default here, but `--reasoning-effort` stays config-overridable.
- **⭐ दूतसभा's usage is the fast path.** The 24.8 s outlier had `num_turns: 1` because all
  material was **inlined in the prompt**, so grok made no tool calls. दूतसभा already inlines
  content into council/review/refine prompts — so in real use grok should be ~4× faster and
  cheaper than the filesystem-reading runs above, **and the containment problem (R5)
  largely disappears because there is nothing to go looking for.** `--sandbox read-only`
  remains the belt-and-braces guarantee.

## Quality bar for `grok.go` (do not clone sibling defects)

Grok's own dogfood review of `agy.go` surfaced four latent defects shared by **all three**
existing providers. Each was verified against the real source before being accepted:

| Finding | Verdict after verification |
|---|---|
| `opts.Timeout` declared but never read by any provider | **Confirmed** (`grep` → no usage in `internal/providers/*.go`). But grok's "unbounded runs" claim is **overstated**: `consult.go:64` and `council.go:87` both wrap `ctx` with `context.WithTimeout`. The real issue is subtler — council applies **one budget to the entire run**, so a single slow agent can consume it all. |
| `providerConfig()` nil-derefs when `p.cfg == nil` | **Confirmed** — no nil guard in any provider |
| A partial `providers.<name>` config block silently drops defaults (`Binary`/`Flags` end up empty) | ❌ **FALSE POSITIVE for the production path.** `collectProviderNames` (`config.go:136`) always includes the built-ins and `buildConfig` (`config.go:177-179`) reads all three fields from viper, which have `SetDefault` values — so `LoadConfig` always populates them. True only for a **hand-constructed `*core.Config`** (i.e. tests), which is not the mechanism grok claimed. Kept as a cheap defensive measure, not billed as a bug fix. |
| *(missed by grok)* Once defaults are registered, the `providerConfig()` fallback branch is **dead code in production** | Corollary the review did not draw — noted so 705 can decide whether to keep it as test-only defence or delete it |
| `HealthCheck` reports `AuthValid: true` after only `--version` | **Confirmed** — it proves the binary exists, not that auth works |

**Decision:** `grok.go` is written **correct from the start** — nil-safe `providerConfig`,
merge-with-defaults rather than replace, and honouring `opts.Timeout`. Shipping a knowingly
defective new file for the sake of symmetry is indefensible. This stays additive: **no
sibling provider is modified by this task.** Bringing `claude.go`/`codex.go`/`agy.go` up to
the same standard is a separate follow-up (propose task **705**), since that *is* a
behaviour change to existing code and belongs in its own reviewed diff.

Add explicit RED tests for each: nil config, partial config block, and `opts.Timeout` honoured.


## Council review — findings incorporated

This spec was reviewed by `dootsabha council --agents codex,agy` (792 s, both agents ok).
Findings triaged against the real binary before acceptance:

| # | Finding | Verdict |
|---|---|---|
| 1 | Streaming errors are `type:"result"` + `subtype:"error_during_execution"` + `is_error:true` + `errors:[]`, **not** `{"type":"error"}` | ✅ **ACCEPTED — verified.** Spec corrected; this was a real defect in the original error ladder |
| 2 | `total_cost_usd_ticks` absent from `streaming-messages-json` | ✅ **ACCEPTED — verified.** Removed |
| 3 | `ProviderResult.CostUSD float64` can't distinguish unreported vs zero — don't claim it can | ✅ **ACCEPTED.** Requirement relaxed; nullable telemetry deferred |
| 4 | Config flags must not override correctness-critical pinned flags | ✅ **ACCEPTED.** New TDD step 2b (`stripPinnedFlags`) |
| 5 | Harden HOME isolation: 0700, race-safe, testable from a passed env slice | ✅ **ACCEPTED.** Folded into TDD 3c |
| 6 | Don't assert exact patch versions | ✅ **ACCEPTED.** Binary self-updated mid-research — proves the point |
| 7 | Register in `plugins/council-strategy/main.go` too | ✅ Already specced |
| 8 | Commit message still described the forbidden `json` shape | ✅ **ACCEPTED.** Rewritten |
| 9 | **Don't add grok to `defaultProviderNames`; `status` must keep 3-provider behaviour** | ⚠️ **OVERRIDDEN — see below** |
| 10 | Drop `plugins/grok/` and `plugin inspect grok` | ❌ **REJECTED.** All three existing providers ship a gRPC plugin (`plugins/{claude,codex,agy}`) built by `make build-plugins`. Omitting grok's would make it the odd one out. Parity wins |

### Decision on #9 (explicit, because it bends the additive rule)

Ārya's instruction was *"all relevant dootsabha commands must be updated and properly
tested (status, council, review and so on)"*. A provider absent from `dootsabha status` is
undiscoverable, so **grok IS added to `defaultProviderNames`** and `status` gains a 4th row.

Consequence, stated plainly: existing assertions that count exactly 3 providers
(`scripts/test-agent-workflow.sh:163-171`, `internal/cli/status_test.go:73`,
`internal/cli/consult_test.go:69`) **must be updated to 4**. That is *updating an
expectation for an intentional additive change*, not weakening a test — but it is the one
place the additive rule genuinely bends, so it is recorded here rather than slipped in.

Everything else stays opt-in: default council, refine reviewers, and review defaults are
untouched. `status` listing a provider ≠ that provider joining any pipeline by default.

