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

# Test 8: mock-agy works (Antigravity print mode — plain text)
if [[ -x "$MOCK_DIR/mock-agy" ]]; then
  RESULT=$("$MOCK_DIR/mock-agy" --dangerously-skip-permissions --model "Gemini 3.5 Flash (High)" -p "PONG" 2>&1)
  if echo "$RESULT" | grep -q 'PONG'; then
    pass "mock-agy produces plain-text response"
  else
    fail "mock-agy output unexpected: $RESULT"
  fi
else
  fail "mock-agy not found/executable"
fi

# Test 9: mock-grok works (streaming-messages-json NDJSON)
if [[ -x "$MOCK_DIR/mock-grok" ]]; then
  RESULT=$("$MOCK_DIR/mock-grok" --output-format streaming-messages-json -m grok-4.5 \
    --reasoning-effort high --sandbox read-only --permission-mode bypassPermissions \
    --always-approve --no-plan --no-subagents --no-auto-update -p "PONG" 2>&1)
  # The answer must come from the result event, not the assistant preamble block.
  if echo "$RESULT" | grep -q '"type":"result"' && echo "$RESULT" | grep -q '"result":"Mock response to: PONG"'; then
    pass "mock-grok emits a parseable result event"
  else
    fail "mock-grok output unexpected: $RESULT"
  fi
else
  fail "mock-grok not found/executable"
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

printf "\nResults: %d passed, %d failed\n" "$PASS" "$FAIL"
[[ "$FAIL" -eq 0 ]]
