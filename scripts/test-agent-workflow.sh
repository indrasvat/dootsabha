#!/usr/bin/env bash
# L5 agent workflow tests — validates दूतसभा is consumable by AI agents.
# Tests: JSON valid, exit codes, no ANSI, required fields, status, errors, performance.
# Uses mock providers for deterministic, offline testing.
set -euo pipefail

BINARY="bin/dootsabha"
MOCK_DIR="testdata/mock-providers"
PASS=0
FAIL=0

pass() { printf "  ✓ %s\n" "$1"; PASS=$((PASS+1)); }
fail() { printf "  ✗ %s\n" "$1"; FAIL=$((FAIL+1)); }

# Mock provider env vars — override real CLIs with mock scripts.
export DOOTSABHA_PROVIDERS_CLAUDE_BINARY="$MOCK_DIR/mock-claude"
export DOOTSABHA_PROVIDERS_CODEX_BINARY="$MOCK_DIR/mock-codex"
export DOOTSABHA_PROVIDERS_AGY_BINARY="$MOCK_DIR/mock-agy"
export DOOTSABHA_PROVIDERS_GROK_BINARY="$MOCK_DIR/mock-grok"

echo "Running L5 agent workflow tests..."
echo "  Binary: $BINARY"
echo "  Mocks:  $MOCK_DIR"
echo ""

# ── Workflow 1: JSON output is valid and parseable ───────────────────────────

echo "--- JSON validity ---"

# 1a. consult --json produces valid JSON
OUTPUT=$("$BINARY" consult --agent claude --json "PONG")
if echo "$OUTPUT" | python3 -m json.tool >/dev/null; then
  pass "consult --json produces valid JSON"
else
  fail "consult --json invalid JSON: $OUTPUT"
fi

# 1b. status --json produces valid JSON
OUTPUT=$("$BINARY" status --json)
if echo "$OUTPUT" | python3 -m json.tool >/dev/null; then
  pass "status --json produces valid JSON"
else
  fail "status --json invalid JSON: $OUTPUT"
fi

# 1c. config show --json produces valid JSON
OUTPUT=$("$BINARY" config show --json)
if echo "$OUTPUT" | python3 -m json.tool >/dev/null; then
  pass "config show --json produces valid JSON"
else
  fail "config show --json invalid JSON: $OUTPUT"
fi

# 1d. plugin list --json produces valid JSON
OUTPUT=$("$BINARY" plugin list --json)
if echo "$OUTPUT" | python3 -m json.tool >/dev/null; then
  pass "plugin list --json produces valid JSON"
else
  fail "plugin list --json invalid JSON: $OUTPUT"
fi

# ── Workflow 2: Exit codes reflect state ─────────────────────────────────────

echo ""
echo "--- Exit codes ---"

# 2a. consult success exits 0
if "$BINARY" consult --agent claude "PONG" >/dev/null; then
  pass "consult success exits 0"
else
  fail "consult success should exit 0"
fi

# 2b. unknown provider exits 1
if "$BINARY" consult --agent nonexistent "test" >/dev/null 2>&1; then
  fail "unknown provider should exit non-zero"
else
  pass "unknown provider exits non-zero"
fi

# 2c. bad flag exits non-zero
if "$BINARY" --badFlag >/dev/null 2>&1; then
  fail "bad flag should exit non-zero"
else
  pass "bad flag exits non-zero"
fi

# 2d. missing required arg exits non-zero
if "$BINARY" consult >/dev/null 2>&1; then
  fail "missing arg should exit non-zero"
else
  pass "missing required arg exits non-zero"
fi

# 2e. --help exits 0
if "$BINARY" --help >/dev/null; then
  pass "--help exits 0"
else
  fail "--help should exit 0"
fi

# 2f. --version exits 0
if "$BINARY" --version >/dev/null; then
  pass "--version exits 0"
else
  fail "--version should exit 0"
fi

# ── Workflow 3: No ANSI in piped output ──────────────────────────────────────

echo ""
echo "--- No ANSI in piped output ---"

# 3a. consult piped has no ANSI escapes
OUTPUT=$("$BINARY" consult --agent claude "PONG" | cat)
if printf '%s' "$OUTPUT" | od -c | grep -q '033'; then
  fail "consult piped output contains ANSI escapes"
else
  pass "consult piped output has no ANSI"
fi

# 3b. status piped has no ANSI escapes
OUTPUT=$("$BINARY" status | cat)
if printf '%s' "$OUTPUT" | od -c | grep -q '033'; then
  fail "status piped output contains ANSI escapes"
else
  pass "status piped output has no ANSI"
fi

# ── Workflow 4: JSON has required fields ─────────────────────────────────────

echo ""
echo "--- Required JSON fields ---"

# 4a. consult JSON has content field (in data envelope)
if "$BINARY" consult --agent claude --json "PONG" | python3 -c "import json,sys; d=json.load(sys.stdin); data=d.get('data',d); assert 'Content' in data or 'content' in data, f'keys: {list(data.keys())}'"; then
  pass "consult JSON has content field"
else
  fail "consult JSON missing content field"
fi

# 4b. consult JSON has model field (in data envelope)
if "$BINARY" consult --agent claude --json "PONG" | python3 -c "import json,sys; d=json.load(sys.stdin); data=d.get('data',d); assert 'Model' in data or 'model' in data, f'keys: {list(data.keys())}'"; then
  pass "consult JSON has model field"
else
  fail "consult JSON missing model field"
fi

# 4c. status JSON has providers
if "$BINARY" status --json | python3 -c "import json,sys; d=json.load(sys.stdin); assert len(d) >= 1, 'no providers'"; then
  pass "status JSON has providers"
else
  fail "status JSON missing providers"
fi

# ── Workflow 5: Status shows all providers ───────────────────────────────────

echo ""
echo "--- Status provider coverage ---"

# 5a. status mentions all 4 providers
STATUS_OUT=$("$BINARY" status)
FOUND=0
for prov in claude codex agy grok; do
  if echo "$STATUS_OUT" | grep -qi "$prov"; then
    FOUND=$((FOUND+1))
  fi
done
if [ "$FOUND" -eq 4 ]; then
  pass "status lists all 4 providers"
else
  fail "status only lists $FOUND/4 providers"
fi

# ── Workflow 5b: Absent providers degrade gracefully ─────────────────────────
#
# REGRESSION GUARD. Every other test here supplies a mock binary for every
# provider, so the "provider not installed" path was never exercised. Adding
# grok as a built-in then made `status` exit 3 on any machine without the grok
# CLI — for an agent the user never opted into. These tests pin the contract.

echo ""
echo "--- Absent provider handling ---"

# 5b. An OPT-IN provider that is absent must NOT fail status.
RC=0; OUT=$(DOOTSABHA_PROVIDERS_GROK_BINARY=/nonexistent/grok "$BINARY" status 2>&1) || RC=$?
if [ "$RC" -eq 0 ]; then
  pass "absent opt-in provider (grok) keeps status exit 0"
else
  fail "absent opt-in provider made status exit $RC (want 0)"
fi

# 5c. ...and it is still listed, described as not installed rather than FAIL.
if echo "$OUT" | grep -qi 'grok' && echo "$OUT" | grep -qi 'not installed'; then
  pass "absent opt-in provider is listed as not installed"
else
  fail "absent opt-in provider not described as not installed: $OUT"
fi

# 5d. No raw exec plumbing leaks into the user-facing table.
if echo "$OUT" | grep -q 'fork/exec'; then
  fail "raw fork/exec error leaked into status output"
else
  pass "no raw exec error in status output for absent opt-in provider"
fi

# 5e. A REQUIRED provider that is absent MUST still fail status.
RC=0; DOOTSABHA_PROVIDERS_AGY_BINARY=/nonexistent/agy "$BINARY" status >/dev/null 2>&1 || RC=$?
if [ "$RC" -eq 3 ]; then
  pass "absent required provider (agy) still exits 3"
else
  fail "absent required provider exited $RC (want 3)"
fi

# 5f. Explicitly consulting an absent provider is a provider error (exit 3).
RC=0; DOOTSABHA_PROVIDERS_GROK_BINARY=/nonexistent/grok "$BINARY" consult --agent grok "hi" >/dev/null 2>&1 || RC=$?
if [ "$RC" -eq 3 ]; then
  pass "consulting an absent provider exits 3"
else
  fail "consulting an absent provider exited $RC (want 3)"
fi

# 5g. status --json stays valid JSON when a provider is absent.
if DOOTSABHA_PROVIDERS_GROK_BINARY=/nonexistent/grok "$BINARY" status --json 2>/dev/null \
     | python3 -c "import json,sys; d=json.load(sys.stdin); g=[p for p in d['data'] if p['Name']=='grok'][0]; assert g['Healthy'] is False; assert g['Installed'] is False"; then
  pass "status --json stays valid and marks absent provider uninstalled"
else
  fail "status --json invalid or mislabelled for an absent provider"
fi

# ── Workflow 5h: --json emits EXACTLY ONE document, even on failure ──────────
#
# REGRESSION GUARD. Commands render their own JSON envelope, and Execute() also
# emits an error envelope for any ExitError in JSON mode. Together they wrote
# TWO documents to stdout, so `... --json | jq` failed with "Extra data" on
# every failure path — precisely when automation most needs to parse the output.

echo ""
echo "--- JSON single-document guarantee ---"

# Parses stdin as exactly one JSON value; fails on trailing documents.
one_json() {
  python3 -c "
import json,sys
raw = sys.stdin.read()
try:
    json.JSONDecoder().raw_decode(raw.lstrip())
except Exception as e:
    sys.exit('not JSON: %s' % e)
obj, end = json.JSONDecoder().raw_decode(raw.lstrip())
if raw.lstrip()[end:].strip():
    sys.exit('extra document after the first')
"
}

# 5h. consult failure
if { DOOTSABHA_PROVIDERS_GROK_BINARY=/nonexistent/grok "$BINARY" consult --agent grok "hi" --json 2>/dev/null || true; } | one_json; then
  pass "consult --json emits one document on provider failure"
else
  fail "consult --json emitted malformed/multiple documents on failure"
fi

# 5i. council partial failure — successful agents must still be reported.
if { DOOTSABHA_PROVIDERS_GROK_BINARY=/nonexistent/grok \
   "$BINARY" council "hi" --agents claude,codex,grok --json 2>/dev/null || true; } | one_json; then
  pass "council --json emits one document on partial failure"
else
  fail "council --json emitted malformed/multiple documents on partial failure"
fi

# 5j. review failure
if { DOOTSABHA_PROVIDERS_GROK_BINARY=/nonexistent/grok \
   "$BINARY" review "hi" --author grok --reviewer claude --json 2>/dev/null || true; } | one_json; then
  pass "review --json emits one document on author failure"
else
  fail "review --json emitted malformed/multiple documents on author failure"
fi

# 5k. refine failure
if { DOOTSABHA_PROVIDERS_GROK_BINARY=/nonexistent/grok \
   "$BINARY" refine "hi" --author claude --reviewers grok --json 2>/dev/null || true; } | one_json; then
  pass "refine --json emits one document on reviewer failure"
else
  fail "refine --json emitted malformed/multiple documents on reviewer failure"
fi

# 5l. council partial failure must PRESERVE the successful agents' output.
PARTIAL=$(DOOTSABHA_PROVIDERS_GROK_BINARY=/nonexistent/grok \
  "$BINARY" council "hi" --agents claude,codex,grok --json 2>/dev/null || true)
if echo "$PARTIAL" | python3 -c "
import json,sys
d = json.JSONDecoder().raw_decode(sys.stdin.read().lstrip())[0]
ok = [x for x in d.get('dispatch', []) if not x.get('error')]
assert len(ok) >= 2, 'successful agents lost: %r' % d.get('dispatch')
" 2>/dev/null; then
  pass "council preserves successful agents' output on partial failure"
else
  fail "council lost successful agents' output on partial failure"
fi

# ── Workflow 5m: chair validation and fallback visibility ───────────────────
#
# REGRESSION GUARD. `--chair bogus` used to be silently accepted: synthesis fell
# back to another agent and the command exited 0, so a typo looked like success
# while a different agent wrote the answer.

echo ""
echo "--- Chair handling ---"

# 5m. An unknown chair name is rejected, like an unknown --agent.
RC=0; OUT=$("$BINARY" council "hi" --agents claude,codex --chair definitely-not-an-agent 2>&1) || RC=$?
if [ "$RC" -ne 0 ] && echo "$OUT" | grep -q 'unknown chair'; then
  pass "unknown --chair is rejected with a clear error"
else
  fail "unknown --chair not rejected (exit $RC): $OUT"
fi

# 5n. A valid-but-unavailable chair still works, but says so.
RC=0; ERROUT=$(DOOTSABHA_PROVIDERS_GROK_BINARY=/nonexistent/grok \
  "$BINARY" council "hi" --agents claude,codex --chair grok 2>&1 >/dev/null) || RC=$?
if echo "$ERROUT" | grep -qi 'chair .* unavailable'; then
  pass "chair fallback is surfaced to the user"
else
  fail "chair fallback happened silently: $ERROUT"
fi

# 5o. ...and the fallback is recorded in JSON for programmatic callers.
if DOOTSABHA_PROVIDERS_GROK_BINARY=/nonexistent/grok \
   "$BINARY" council "hi" --agents claude,codex --chair grok --json 2>/dev/null \
   | python3 -c "
import json,sys
d = json.JSONDecoder().raw_decode(sys.stdin.read().lstrip())[0]
assert d['synthesis']['chair_fallback'], 'chair_fallback not recorded'
"; then
  pass "chair fallback recorded in JSON"
else
  fail "chair fallback missing from JSON"
fi

# ── Workflow 6: Error produces structured output ─────────────────────────────

echo ""
echo "--- Error handling ---"

# 6a. unknown provider exits non-zero with message
ERROR_OUT=$("$BINARY" consult --agent nonexistent "test" 2>&1 || true)
if echo "$ERROR_OUT" | grep -qi "unknown"; then
  pass "unknown provider shows error message"
else
  fail "unknown provider error message missing: $ERROR_OUT"
fi

# 6b. unknown command shows helpful error
ERROR_OUT=$("$BINARY" unknown-cmd-xyz 2>&1 || true)
if echo "$ERROR_OUT" | grep -qi "unknown command"; then
  pass "unknown command shows helpful error"
else
  fail "unknown command error missing: $ERROR_OUT"
fi

# ── Workflow 7: Performance ──────────────────────────────────────────────────

echo ""
echo "--- Performance ---"

# 7a. startup under 2s (--version is cheapest)
START_MS=$(python3 -c "import time; print(int(time.time()*1000))")
"$BINARY" --version >/dev/null
END_MS=$(python3 -c "import time; print(int(time.time()*1000))")
ELAPSED=$((END_MS - START_MS))
if [ "$ELAPSED" -lt 2000 ]; then
  pass "startup under 2s (${ELAPSED}ms)"
else
  fail "startup took ${ELAPSED}ms (>2000ms)"
fi

# 7b. --help under 2s
START_MS=$(python3 -c "import time; print(int(time.time()*1000))")
"$BINARY" --help >/dev/null
END_MS=$(python3 -c "import time; print(int(time.time()*1000))")
ELAPSED=$((END_MS - START_MS))
if [ "$ELAPSED" -lt 2000 ]; then
  pass "--help under 2s (${ELAPSED}ms)"
else
  fail "--help took ${ELAPSED}ms (>2000ms)"
fi

# 7c. consult with mock provider under 3s
START_MS=$(python3 -c "import time; print(int(time.time()*1000))")
"$BINARY" consult --agent claude "PONG" >/dev/null
END_MS=$(python3 -c "import time; print(int(time.time()*1000))")
ELAPSED=$((END_MS - START_MS))
if [ "$ELAPSED" -lt 3000 ]; then
  pass "consult (mock) under 3s (${ELAPSED}ms)"
else
  fail "consult (mock) took ${ELAPSED}ms (>3000ms)"
fi

# ── Workflow 8: Bilingual aliases ────────────────────────────────────────────

echo ""
echo "--- Bilingual aliases ---"

# 8a. paraamarsh alias works
if "$BINARY" paraamarsh --agent claude "PONG" >/dev/null; then
  pass "paraamarsh alias works"
else
  fail "paraamarsh alias failed"
fi

# 8b. sthiti alias works
if "$BINARY" sthiti >/dev/null; then
  pass "sthiti alias works"
else
  fail "sthiti alias failed"
fi

# 8c. vinyaas alias works
if "$BINARY" vinyaas show >/dev/null; then
  pass "vinyaas alias works"
else
  fail "vinyaas alias failed"
fi

# ── Workflow 9: Context file for extensions ──────────────────────────────────

echo ""
echo "--- Context file ---"

# 9a. Extension receives context file
cat > /tmp/dootsabha-ctxtest <<'EXTEOF'
#!/bin/bash
if [ -n "$DOOTSABHA_CONTEXT_FILE" ] && [ -f "$DOOTSABHA_CONTEXT_FILE" ]; then
    python3 -m json.tool "$DOOTSABHA_CONTEXT_FILE" >/dev/null
    echo "CONTEXT_OK"
else
    echo "CONTEXT_MISSING"
    exit 1
fi
EXTEOF
chmod +x /tmp/dootsabha-ctxtest

CTX_OUT=$(PATH="/tmp:$PATH" "$BINARY" ctxtest)
if echo "$CTX_OUT" | grep -q "CONTEXT_OK"; then
  pass "extension receives valid context file"
else
  fail "extension context file: $CTX_OUT"
fi
rm -f /tmp/dootsabha-ctxtest

# 9b. Context file cleaned up after extension exits
if ls /tmp/dootsabha-context-*.json >/dev/null 2>&1; then
  fail "context file not cleaned up"
else
  pass "context file cleaned up after extension"
fi

# ── Workflow 10: SIGPIPE handling ────────────────────────────────────────────

echo ""
echo "--- SIGPIPE ---"

# 10a. piped to head exits cleanly (not broken pipe error)
"$BINARY" --help | head -1 >/dev/null
RC=$?
if [ "$RC" -eq 0 ]; then
  pass "SIGPIPE exits 0 when piped to head"
else
  fail "SIGPIPE exit code: $RC (expected 0)"
fi

# ── Summary ──────────────────────────────────────────────────────────────────

printf "\nResults: %d passed, %d failed\n" "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
