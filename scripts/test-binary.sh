#!/usr/bin/env bash
# L3 smoke test: build binary and run basic commands with mock providers
set -euo pipefail

BINARY="bin/dootsabha"
MOCK_DIR="testdata/mock-providers"
PASS=0
FAIL=0

pass() { printf "  ✓ %s\n" "$1"; PASS=$((PASS+1)); }
fail() { printf "  ✗ %s\n" "$1"; FAIL=$((FAIL+1)); }

echo "Running L3 smoke tests..."

# Test 1: binary exists
if [[ -x "$BINARY" ]]; then
  pass "binary exists and is executable"
else
  fail "binary not found: $BINARY"
fi

# Test 2: --help exits 0
if "$BINARY" --help >/dev/null 2>&1; then
  pass "--help exits 0"
else
  fail "--help failed"
fi

# Test 3: --version exits 0
if "$BINARY" --version >/dev/null 2>&1; then
  pass "--version exits 0"
else
  fail "--version failed"
fi

# Test 4: --version shows version string (dev, semver, or git hash)
VERSION_OUT=$("$BINARY" --version 2>&1)
if echo "$VERSION_OUT" | grep -qE "dev|[0-9]+\.[0-9]+|[0-9a-f]{7}"; then
  pass "--version shows version string"
else
  fail "--version output unexpected: $VERSION_OUT"
fi

# Test 5: unknown command exits non-zero
if ! "$BINARY" unknown-command-xyz >/dev/null 2>&1; then
  pass "unknown command exits non-zero"
else
  fail "unknown command should exit non-zero"
fi

# Test 6: mock-claude works
if [[ -x "$MOCK_DIR/mock-claude" ]]; then
  RESULT=$("$MOCK_DIR/mock-claude" -p "PONG" --output-format json 2>&1)
  if echo "$RESULT" | grep -q '"result"'; then
    pass "mock-claude produces JSON"
  else
    fail "mock-claude JSON unexpected: $RESULT"
  fi
else
  fail "mock-claude not found/executable"
fi

# Test 7: mock-codex works
if [[ -x "$MOCK_DIR/mock-codex" ]]; then
  RESULT=$("$MOCK_DIR/mock-codex" --json "PONG" 2>&1)
  if echo "$RESULT" | grep -q '"type"'; then
    pass "mock-codex produces JSONL"
  else
    fail "mock-codex JSONL unexpected: $RESULT"
  fi
else
  fail "mock-codex not found/executable"
fi

# Test 8: mock-agy works (Antigravity print mode — JSON envelope)
if [[ -x "$MOCK_DIR/mock-agy" ]]; then
  RESULT=$("$MOCK_DIR/mock-agy" --dangerously-skip-permissions --model "Gemini 3.7 Flash (High)" \
    --output-format json -p "PONG" 2>&1)
  if python3 -c '
import json,sys
d = json.load(sys.stdin)
assert d["status"] == "SUCCESS", d
assert "PONG" in d["response"], d
assert d["usage"]["input_tokens"] + d["usage"]["output_tokens"] == d["usage"]["total_tokens"], d
' <<<"$RESULT" 2>/dev/null; then
    pass "mock-agy emits a parseable JSON envelope"
  else
    fail "mock-agy output unexpected: $RESULT"
  fi
  # The mock ECHOES the --model it was handed. A SYNTHETIC model is used
  # deliberately: asserting the shipped default here would move both sides
  # together every bump and prove nothing.
  if "$MOCK_DIR/mock-agy" --model "probe-9.9" -p "PONG" 2>&1 | grep -q 'model=probe-9.9'; then
    pass "mock-agy echoes the model it was given"
  else
    fail "mock-agy did not echo --model"
  fi
  # ...and with no --model at all it must NOT invent a plausible one, or a
  # regression that dropped the flag would read as a pass.
  if "$MOCK_DIR/mock-agy" -p "PONG" 2>&1 | grep -q 'model=unset'; then
    pass "mock-agy reports a sentinel when --model is absent"
  else
    fail "mock-agy masks a missing --model"
  fi
else
  fail "mock-agy not found/executable"
fi

# Test 8b: dootsabha FORWARDS the configured model to agy, and parses the JSON
# envelope back into session id and token counts.
#
# REGRESSION GUARD. mock-agy previously discarded --model outright, so no
# integration test could observe a forwarding regression at all.
# --config /dev/null so a developer's ~/.dootsabha config cannot mask the default.
AGY_JSON=$(DOOTSABHA_PROVIDERS_AGY_BINARY="$MOCK_DIR/mock-agy" \
  "$BINARY" consult --agent agy --json --config /dev/null "PONG" 2>/dev/null || true)
if python3 -c '
import json,sys
d = json.load(sys.stdin)["data"]
assert "model=Gemini 3.7 Flash (High)" in d["Content"], d["Content"]
assert d["Model"] == "Gemini 3.7 Flash (High)", d["Model"]
assert d["SessionID"], "conversation_id was not parsed out of the JSON envelope"
assert d["TokensIn"] > 0 and d["TokensOut"] > 0, d
' <<<"$AGY_JSON" 2>/dev/null; then
  pass "consult --agent agy forwards the shipped default and parses JSON telemetry"
else
  fail "agy default not forwarded end-to-end: $AGY_JSON"
fi

# Test 8c: an explicitly pinned model is never rewritten. 3.6/3.5/3.1 are still
# live, so bumping the DEFAULT must not move a user who chose one.
AGY_PIN_CFG=$(mktemp -t dootsabha-agy-pin-XXXXXX)
trap 'rm -f "$AGY_PIN_CFG"' EXIT
printf 'providers:\n  agy:\n    binary: %s\n    model: Gemini 3.5 Flash (High)\n' \
  "$PWD/$MOCK_DIR/mock-agy" > "$AGY_PIN_CFG"
AGY_PIN_JSON=$("$BINARY" consult --agent agy --json --config "$AGY_PIN_CFG" "PONG" 2>/dev/null || true)
if python3 -c '
import json,sys
d = json.load(sys.stdin)["data"]
assert "model=Gemini 3.5 Flash (High)" in d["Content"], d["Content"]
' <<<"$AGY_PIN_JSON" 2>/dev/null; then
  pass "an explicitly pinned agy model survives the default bump"
else
  fail "pinned agy model was rewritten: $AGY_PIN_JSON"
fi

# Test 8d: a user cannot break the parser or smuggle a model from config.
#
# agy parses argv with Go's stdlib flag package, so `-model` IS `--model` and a
# repeat is LAST-WINS. A stripper matching only the double-dash spelling let
# `-output-format text` break every parse, and `-model X` SILENTLY run a different
# model than the one dootsabha reports — undetectable, because agy's envelope
# never echoes the model back.
AGY_FMT_CFG=$(mktemp -t dootsabha-agy-fmt-XXXXXX)
trap 'rm -f "$AGY_PIN_CFG" "$AGY_FMT_CFG"' EXIT
for SPELLING in "--output-format" "-output-format"; do
  printf 'providers:\n  agy:\n    binary: %s\n    flags:\n      - %s\n      - text\n' \
    "$PWD/$MOCK_DIR/mock-agy" "$SPELLING" > "$AGY_FMT_CFG"
  if "$BINARY" consult --agent agy --json --config "$AGY_FMT_CFG" "PONG" >/dev/null 2>&1; then
    pass "a configured '$SPELLING text' cannot break the agy parser"
  else
    fail "config-supplied '$SPELLING' reached agy and broke the parse"
  fi
done

for SPELLING in "--model" "-model"; do
  printf 'providers:\n  agy:\n    binary: %s\n    model: Gemini 3.7 Flash (High)\n    flags:\n      - %s\n      - SMUGGLED\n' \
    "$PWD/$MOCK_DIR/mock-agy" "$SPELLING" > "$AGY_FMT_CFG"
  SMUGGLE_JSON=$("$BINARY" consult --agent agy --json --config "$AGY_FMT_CFG" "PONG" 2>/dev/null || true)
  if grep -qF "model=Gemini 3.7 Flash (High)" <<<"$SMUGGLE_JSON" && ! grep -qF "SMUGGLED" <<<"$SMUGGLE_JSON"; then
    pass "a configured '$SPELLING SMUGGLED' cannot override the pinned model"
  else
    fail "'$SPELLING' smuggled a model past the pin: $SMUGGLE_JSON"
  fi
done

# Test 8e: a stray NON-FLAG token in flags must not swallow the prompt. agy stops
# parsing at the first non-flag token, so with -p last the prompt was never sent
# and agy tried to open an interactive TUI instead.
printf 'providers:\n  agy:\n    binary: %s\n    flags:\n      - --dangerously-skip-permissions\n      - "true"\n' \
  "$PWD/$MOCK_DIR/mock-agy" > "$AGY_FMT_CFG"
STRAY_JSON=$("$BINARY" consult --agent agy --json --config "$AGY_FMT_CFG" "KEEPTHISPROMPT" 2>/dev/null || true)
if grep -qF "KEEPTHISPROMPT" <<<"$STRAY_JSON"; then
  pass "a stray non-flag token in flags does not swallow the prompt"
else
  fail "the prompt was lost to a stray flags token: $STRAY_JSON"
fi

# Test 8f: a malformed `flags` is a loud config error, not a silent empty list.
printf 'providers:\n  agy:\n    binary: agy\n    flags: 42\n' > "$AGY_FMT_CFG"
RC=0; "$BINARY" consult --agent agy --config "$AGY_FMT_CFG" "PONG" >/dev/null 2>&1 || RC=$?
if [[ "$RC" -eq 6 ]]; then
  pass "a non-list providers.agy.flags exits 6"
else
  fail "malformed flags exited $RC, want 6"
fi

# Test 9: mock-grok works (streaming-messages-json NDJSON)
if [[ -x "$MOCK_DIR/mock-grok" ]]; then
  RESULT=$("$MOCK_DIR/mock-grok" --output-format streaming-messages-json -m grok-4.6 \
    --reasoning-effort high --sandbox read-only --permission-mode bypassPermissions \
    --always-approve --no-plan --no-subagents --no-auto-update -p "PONG" 2>&1)
  # The answer must come from the result event, not the assistant preamble block.
  if echo "$RESULT" | grep -q '"type":"result"' && echo "$RESULT" | grep -q '"result":"Mock response to: PONG"'; then
    pass "mock-grok emits a parseable result event"
  else
    fail "mock-grok output unexpected: $RESULT"
  fi
  # The mock ECHOES the -m it was handed as "<model>-build", mirroring the real
  # CLI's backend-id convention. A SYNTHETIC model is used deliberately: asserting
  # grok-4.6-build here would move both sides together every bump and prove
  # nothing. probe-9.9 can only appear if the flag really was read.
  ECHO_RESULT=$("$MOCK_DIR/mock-grok" -m probe-9.9 -p "PONG" 2>&1)
  if echo "$ECHO_RESULT" | grep -q '"probe-9.9-build"'; then
    pass "mock-grok echoes the model it was given as the backend id"
  else
    fail "mock-grok did not echo -m into modelUsage: $ECHO_RESULT"
  fi
  # ...and with no -m at all it must NOT invent a plausible model, or a
  # regression that dropped the flag would read as a pass.
  if "$MOCK_DIR/mock-grok" -p "PONG" 2>&1 | grep -q '"unset-build"'; then
    pass "mock-grok reports a sentinel when -m is absent"
  else
    fail "mock-grok masks a missing -m"
  fi
else
  fail "mock-grok not found/executable"
fi

# Test 9b: dootsabha actually FORWARDS the configured model to grok, and surfaces
# the backend id it gets back.
#
# REGRESSION GUARD. L5 drove grok through the binary extensively, but only on
# FAILURE paths — every grok assertion pointed at /nonexistent/grok or a hostile
# stub. And mock-grok hardcoded its own model string, so even a success-path test
# could not have seen a model-forwarding regression. The mock now echoes the -m
# it was handed, and this asserts the whole chain end-to-end.
# --config /dev/null so a developer's ~/.dootsabha config cannot mask the default.
GROK_JSON=$(DOOTSABHA_PROVIDERS_GROK_BINARY="$MOCK_DIR/mock-grok" \
  "$BINARY" consult --agent grok --json --config /dev/null "PONG" 2>/dev/null || true)
if python3 -c '
import json,sys
d = json.load(sys.stdin)
m = d["data"]["Model"]
assert m == "grok-4.6-build", f"model={m!r}, want grok-4.6-build"
assert "PONG" in d["data"]["Content"], d["data"]["Content"]
' <<<"$GROK_JSON" 2>/dev/null; then
  pass "consult --agent grok forwards the shipped default and reports the backend id"
else
  fail "grok default not forwarded end-to-end: $GROK_JSON"
fi

# Test 9c: an explicitly pinned model is never rewritten. grok-4.5 is still a live
# model, so bumping the DEFAULT must not move a user who chose 4.5.
GROK_PIN_CFG=$(mktemp -t dootsabha-grok-pin-XXXXXX)
trap 'rm -f "$GROK_PIN_CFG"' EXIT
printf 'providers:\n  grok:\n    binary: %s\n    model: grok-4.5\n' "$PWD/$MOCK_DIR/mock-grok" > "$GROK_PIN_CFG"
GROK_PIN_JSON=$("$BINARY" consult --agent grok --json --config "$GROK_PIN_CFG" "PONG" 2>/dev/null || true)
if python3 -c '
import json,sys
d = json.load(sys.stdin)
m = d["data"]["Model"]
assert m == "grok-4.5-build", f"model={m!r}, want grok-4.5-build (a pinned model must survive)"
' <<<"$GROK_PIN_JSON" 2>/dev/null; then
  pass "an explicitly pinned grok-4.5 survives the default bump"
else
  fail "pinned grok model was rewritten: $GROK_PIN_JSON"
fi

# Test 9d: a MALFORMED provider pin is a loud config error, not a silent fallback.
#
# PRD §6.1 makes this an exit-code contract, so assert the PROCESS exit code, not
# just the error text: `model: [grok-4.5]` used to load with exit 0 while the
# bumped default actually ran, which defeats the one guarantee the grok-4.6 bump
# makes. `flags` IS a list, so writing `model` as one is a plausible typo.
BAD_PIN_CFG=$(mktemp -t dootsabha-grok-badpin-XXXXXX)
printf 'providers:\n  grok:\n    model: [grok-4.5]\n' > "$BAD_PIN_CFG"
RC=0
BAD_OUT=$("$BINARY" consult --agent grok --config "$BAD_PIN_CFG" "hi" 2>&1) || RC=$?
rm -f "$BAD_PIN_CFG"
if [[ "$RC" -eq 6 ]] && grep -q 'providers.grok.model' <<<"$BAD_OUT"; then
  pass "a malformed provider pin exits 6 and names the key"
else
  fail "malformed pin gave exit $RC (want 6): $BAD_OUT"
fi

# Test 10: mocks stay valid JSON for prompts containing quotes and newlines.
#
# REGRESSION GUARD. council/refine/review build multi-line prompts (they embed
# prior agent responses), so a mock that naively interpolates $PROMPT into a JSON
# string emits invalid JSON — and those pipelines were being exercised against
# garbage rather than a faithful stand-in for the real CLI.
NASTY='Line one
Line "two" with quotes and \backslash'

if "$MOCK_DIR/mock-claude" -p "$NASTY" --output-format json 2>/dev/null \
   | python3 -c 'import json,sys; json.load(sys.stdin)' 2>/dev/null; then
  pass "mock-claude emits valid JSON for a quoted multi-line prompt"
else
  fail "mock-claude emits INVALID JSON for a quoted multi-line prompt"
fi

if "$MOCK_DIR/mock-codex" "$NASTY" 2>/dev/null \
   | python3 -c 'import json,sys; [json.loads(l) for l in sys.stdin if l.strip()]' 2>/dev/null; then
  pass "mock-codex emits valid JSONL for a quoted multi-line prompt"
else
  fail "mock-codex emits INVALID JSONL for a quoted multi-line prompt"
fi

if "$MOCK_DIR/mock-grok" -p "$NASTY" --output-format streaming-messages-json 2>/dev/null \
   | python3 -c 'import json,sys; [json.loads(l) for l in sys.stdin if l.strip()]' 2>/dev/null; then
  pass "mock-grok emits valid NDJSON for a quoted multi-line prompt"
else
  fail "mock-grok emits INVALID NDJSON for a quoted multi-line prompt"
fi

# ---------------------------------------------------------------------------
# Timeout scoping (GitHub issue #20)
#
# `--timeout` is the budget for ONE provider invocation; `--session-timeout` is
# the ceiling for the whole pipeline. The bug was a single deadline shared by
# every call, so a slow author starved the reviewer and the reviewer took the
# blame. Each case below runs a pipeline whose TOTAL runtime exceeds the
# per-invocation budget while no SINGLE step does — that combination fails under
# the shared-deadline model and passes under per-invocation budgets.
#
# Mock delays are driven by MOCK_<PROVIDER>_DELAY (seconds).
# ---------------------------------------------------------------------------
export DOOTSABHA_PROVIDERS_CLAUDE_BINARY="$MOCK_DIR/mock-claude"
export DOOTSABHA_PROVIDERS_CODEX_BINARY="$MOCK_DIR/mock-codex"
export DOOTSABHA_PROVIDERS_AGY_BINARY="$MOCK_DIR/mock-agy"

# Each step sleeps STEP_DELAY; each step is allowed STEP_BUDGET. A pipeline of
# 2-3 steps therefore outlives one budget without any step exceeding it.
STEP_DELAY=0.7
STEP_BUDGET=1200ms

# run_pipeline <desc> <expected-exit> <expected-stdout-substring> -- <args...>
# Runs the binary with mock delays applied, checks exit code and output.
run_pipeline() {
  local desc="$1" want_exit="$2" want_text="$3"; shift 4  # shift past the "--"
  local out rc=0
  out=$(MOCK_CLAUDE_DELAY="$STEP_DELAY" MOCK_CODEX_DELAY="$STEP_DELAY" MOCK_AGY_DELAY="$STEP_DELAY" \
        "$BINARY" "$@" --config /dev/null 2>&1) || rc=$?
  if [[ "$rc" -ne "$want_exit" ]]; then
    fail "$desc (exit $rc, want $want_exit)"
    return
  fi
  if [[ -n "$want_text" ]] && ! grep -qF "$want_text" <<<"$out"; then
    fail "$desc (output missing '$want_text')"
    return
  fi
  pass "$desc"
}

# Test 11: review — a slow author must not starve the reviewer. THE issue #20 case.
run_pipeline "review: author and reviewer each get their own budget" 0 '"claude": "ok"' -- \
  review --json --author codex --reviewer claude --timeout "$STEP_BUDGET" --session-timeout 60s \
  "say something"

# Test 12: review — reviewer content actually lands in the JSON, not just exit 0.
REVIEW_JSON=$(MOCK_CLAUDE_DELAY="$STEP_DELAY" MOCK_CODEX_DELAY="$STEP_DELAY" \
  "$BINARY" review --json --author codex --reviewer claude --timeout "$STEP_BUDGET" \
  --session-timeout 60s --config /dev/null "say something" 2>/dev/null || true)
if python3 -c '
import json,sys
d = json.load(sys.stdin)
assert d["author"]["content"], "author content empty"
assert d["review"]["content"], "reviewer content empty"
assert d["meta"]["providers"]["claude"] == "ok", "reviewer not ok"
' <<<"$REVIEW_JSON" 2>/dev/null; then
  pass "review: both agents produce content under a per-invocation budget"
else
  fail "review: reviewer produced no content — starved by the author"
fi

# Test 13: review — the session ceiling still fires, and says so.
run_pipeline "review: session ceiling fires and is named" 4 "session timeout" -- \
  review --author codex --reviewer claude --timeout 60s --session-timeout 1s \
  "say something"

# Test 14: review — a genuinely slow single call is still an invocation timeout.
OUT=$(MOCK_CODEX_DELAY=5 "$BINARY" review --author codex --reviewer claude \
  --timeout 400ms --session-timeout 60s --config /dev/null "say something" 2>&1 || true)
if grep -qF "invocation timeout" <<<"$OUT"; then
  pass "review: invocation timeout is named as such"
else
  fail "review: expected 'invocation timeout' in: $OUT"
fi

# Test 15: refine — v1 → review → incorporate are three separate budgets.
run_pipeline "refine: every pipeline step gets its own budget" 0 "" -- \
  refine --json --author claude --reviewers codex --timeout "$STEP_BUDGET" \
  --session-timeout 60s "say something"

# Test 16: refine — the incorporation step actually ran (v2 exists).
REFINE_JSON=$(MOCK_CLAUDE_DELAY="$STEP_DELAY" MOCK_CODEX_DELAY="$STEP_DELAY" \
  "$BINARY" refine --json --author claude --reviewers codex --timeout "$STEP_BUDGET" \
  --session-timeout 60s --config /dev/null "say something" 2>/dev/null || true)
if python3 -c '
import json,sys
d = json.load(sys.stdin)
assert d["final"]["version"] >= 2, "final version is not >= 2 — incorporation never ran"
assert d["meta"]["providers"]["codex"] == "ok", "reviewer not ok"
' <<<"$REFINE_JSON" 2>/dev/null; then
  pass "refine: reviewer + incorporation both completed"
else
  fail "refine: pipeline stopped early — a later step inherited a spent budget"
fi

# Test 16b: refine — a reviewer that blows its OWN window must not end the run.
#
# The old code broke out of the loop on any DeadlineExceeded, which was right
# when one deadline bounded everything. With per-invocation budgets it is wrong:
# reviewer B still has a full window of its own. codex hangs past --timeout, agy
# must still review, and the run must exit 4 (a timeout outranks the partial).
REFINE_PARTIAL=$(MOCK_CODEX_DELAY=5 MOCK_CLAUDE_DELAY=0 MOCK_AGY_DELAY=0 \
  "$BINARY" refine --json --author claude --reviewers codex,agy --timeout 500ms \
  --session-timeout 60s --config /dev/null "say something" 2>/dev/null || true)
if python3 -c '
import json,sys
d = json.load(sys.stdin)
p = d["meta"]["providers"]
assert p.get("codex") == "error", "the hung reviewer should be marked error, got %r" % p
assert p.get("agy") == "ok", "the second reviewer never ran — the loop broke out early: %r" % p
assert d["final"]["version"] >= 2, "no incorporation ran after the surviving review"
' <<<"$REFINE_PARTIAL" 2>/dev/null; then
  pass "refine: a reviewer blowing its own window does not end the pipeline"
else
  fail "refine: one reviewer timing out killed the rest of the pipeline"
fi

RC=0
MOCK_CODEX_DELAY=5 "$BINARY" refine --author claude --reviewers codex,agy --timeout 500ms \
  --session-timeout 60s --config /dev/null "say something" >/dev/null 2>&1 || RC=$?
if [[ "$RC" -eq 4 ]]; then
  pass "refine: a timed-out reviewer still exits 4 (timeout outranks partial)"
else
  fail "refine: reviewer timeout exited $RC, want 4"
fi

# Test 17: council — dispatch, peer review and synthesis are separate budgets.
run_pipeline "council: dispatch/review/synthesis each get their own budget" 0 "" -- \
  council --json --agents claude,codex --chair claude --rounds 1 \
  --timeout "$STEP_BUDGET" --session-timeout 60s "say something"

# Test 18: council — synthesis produced content rather than dying of starvation.
COUNCIL_JSON=$(MOCK_CLAUDE_DELAY="$STEP_DELAY" MOCK_CODEX_DELAY="$STEP_DELAY" \
  "$BINARY" council --json --agents claude,codex --chair claude --rounds 1 \
  --timeout "$STEP_BUDGET" --session-timeout 60s --config /dev/null "say something" 2>/dev/null || true)
if python3 -c '
import json,sys
d = json.load(sys.stdin)
assert d["synthesis"] and d["synthesis"]["content"], "no synthesis content"
assert all(v == "ok" for v in d["meta"]["providers"].values()), d["meta"]["providers"]
' <<<"$COUNCIL_JSON" 2>/dev/null; then
  pass "council: synthesis ran after dispatch and peer review"
else
  fail "council: synthesis starved by the earlier stages"
fi

# Test 19: a timeout in JSON mode still emits exactly ONE JSON document.
TIMEOUT_JSON=$(MOCK_CLAUDE_DELAY="$STEP_DELAY" MOCK_CODEX_DELAY="$STEP_DELAY" \
  "$BINARY" review --json --author codex --reviewer claude --timeout 60s \
  --session-timeout 1s --config /dev/null "say something" 2>/dev/null || true)
if python3 -c 'import json,sys; json.load(sys.stdin)' <<<"$TIMEOUT_JSON" 2>/dev/null; then
  pass "session timeout emits exactly one JSON document"
else
  fail "session timeout produced unparseable stdout: $TIMEOUT_JSON"
fi

printf "\nResults: %d passed, %d failed\n" "$PASS" "$FAIL"
[[ "$FAIL" -eq 0 ]]
