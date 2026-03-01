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

# Test 8: mock-gemini works
if [[ -x "$MOCK_DIR/mock-gemini" ]]; then
  RESULT=$("$MOCK_DIR/mock-gemini" --yolo --output-format json "PONG" 2>&1)
  if echo "$RESULT" | grep -q '"result"'; then
    pass "mock-gemini produces JSON"
  else
    fail "mock-gemini JSON unexpected: $RESULT"
  fi
else
  fail "mock-gemini not found/executable"
fi

printf "\nResults: %d passed, %d failed\n" "$PASS" "$FAIL"
[[ "$FAIL" -eq 0 ]]
