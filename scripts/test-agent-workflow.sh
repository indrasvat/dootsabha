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

# A provider that never returns, for timeout tests.
SCRATCH_HANG="$(mktemp -t dootsabha-hang)"
# shellcheck disable=SC2016  # $1 is the generated script's own arg, not ours
printf '#!/usr/bin/env bash\n[[ "$1" == "--version" ]] && { echo "0.0.0"; exit 0; }\nsleep 300\n' > "$SCRATCH_HANG"
chmod +x "$SCRATCH_HANG"

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
# (removed: asserted only "non-zero" where the contract says exactly 2;
#  `expect_exit 2 "unknown provider"` covers this precisely.)

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
if "$BINARY" status --json | python3 -c "
import json,sys
d = json.load(sys.stdin)
rows = d['data']
assert isinstance(rows, list) and rows, 'data is not a non-empty list'
need = {'Name','Healthy','Version','Model','Reachable','Installed'}
for r in rows:
    missing = need - set(r)
    assert not missing, 'row %r missing %s' % (r.get('Name'), missing)
assert {r['Name'] for r in rows} >= {'claude','codex','agy','grok'}, 'built-in provider missing'
"; then
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

# 5e. A REQUIRED provider that is absent MUST still fail status — but as
# DEGRADED (5), because the other agents still work. Only a setup with no usable
# agent at all is 3.
RC=0; DOOTSABHA_PROVIDERS_AGY_BINARY=/nonexistent/agy "$BINARY" status >/dev/null 2>&1 || RC=$?
if [ "$RC" -eq 5 ]; then
  pass "absent required provider degrades status to 5"
else
  fail "absent required provider exited $RC (want 5, degraded)"
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

# ── Workflow 5p: exit-code contract ─────────────────────────────────────────
#
# One code, one caller action:
#   0 proceed · 1 internal bug · 2 fix the command · 3 retry/other agent
#   4 raise --timeout · 5 usable but incomplete · 6 fix the config
# Precedence: 2 > 6 > 4 > 3 > 5 > 1 > 0

echo ""
echo "--- Exit-code contract ---"

# Asserts the exit code in BOTH text and --json mode.
#
# These were previously tested on separate axes — expect_exit never passed
# --json, and the JSON-document tests never checked an exit code. That blind
# spot let `status --json` return 0 for every health state, and then let the
# identical defect ship in `refine --json` in the very run that claimed the
# class was closed. --json is the mode agents are told to use; it must never
# be the more forgiving one.
expect_exit() {
  local want="$1" desc="$2"; shift 2
  local rc=0
  "$@" >/dev/null 2>&1 || rc=$?
  if [ "$rc" -eq "$want" ]; then
    pass "exit $want — $desc"
  else
    fail "exit $rc (want $want) — $desc"
  fi

  # Same invocation, plus --json: the code must be identical AND stdout must be
  # exactly one JSON document. Asserting only the code left code x payload
  # uncrossed — that gap is why an empty stdout with a correct exit code went
  # unnoticed.
  local jrc=0 jout
  jout=$("$@" --json 2>/dev/null) || jrc=$?
  if [ "$jrc" -eq "$want" ]; then
    pass "exit $want — $desc [--json]"
  else
    fail "exit $jrc (want $want) — $desc [--json] — JSON mode disagrees with text mode"
  fi
  if printf '%s' "$jout" | one_json; then
    pass "one JSON document — $desc [--json]"
  else
    fail "stdout was not exactly one JSON document — $desc [--json]"
  fi
}

expect_exit 0 "successful consult"            "$BINARY" consult --agent claude "hi"
expect_exit 2 "unknown flag"                  "$BINARY" consult --definitely-not-a-flag
expect_exit 2 "missing prompt arg"            "$BINARY" consult
expect_exit 2 "missing --agent"               "$BINARY" consult "hi"
expect_exit 2 "unknown provider"              "$BINARY" consult --agent definitely-not-an-agent "hi"
expect_exit 2 "unknown chair"                 "$BINARY" council "hi" --chair definitely-not-an-agent
expect_exit 2 "unknown agent in --agents"     "$BINARY" council "hi" --agents claude,definitely-not-an-agent
expect_exit 2 "unknown --reviewers (refine)"  "$BINARY" refine "hi" --reviewers definitely-not-an-agent
expect_exit 2 "unknown --author (refine)"     "$BINARY" refine "hi" --author definitely-not-an-agent
expect_exit 2 "unknown --author (review)"     "$BINARY" review "hi" --author definitely-not-an-agent
expect_exit 2 "stray comma in --agents"       "$BINARY" council "hi" --agents claude,,codex
expect_exit 2 "stray comma in --reviewers"    "$BINARY" refine "hi" --reviewers codex,,agy

# --json must carry the SAME exit code as human mode. It is the mode agents are
# told to always use, so a status that reports 0 with zero healthy providers
# breaks every health gate built on it.
expect_exit 3 "status --json: nothing usable" \
  env DOOTSABHA_PROVIDERS_CLAUDE_BINARY=/x DOOTSABHA_PROVIDERS_CODEX_BINARY=/x DOOTSABHA_PROVIDERS_AGY_BINARY=/x DOOTSABHA_PROVIDERS_GROK_BINARY=/x "$BINARY" status --json
expect_exit 5 "status --json: degraded" \
  env DOOTSABHA_PROVIDERS_AGY_BINARY=/x "$BINARY" status --json
expect_exit 6 "plugin list: bad --config"     "$BINARY" plugin list --config /nope/nope.yaml
expect_exit 2 "council: --rounds above max"   "$BINARY" council "hi" --agents claude --rounds 99

# CLAUDE.md mandates a Devanagari alias for every command. An alias that diverges
# from its ASCII form on an error path is a silent trap, so parity is asserted on
# BOTH success and failure.
echo ""
echo "--- Bilingual alias parity ---"

alias_parity() {
  local ascii="$1" deva="$2"; shift 2
  local a=0 d=0
  "$BINARY" "$ascii" "$@" >/dev/null 2>&1 || a=$?
  "$BINARY" "$deva"  "$@" >/dev/null 2>&1 || d=$?
  if [ "$a" -eq "$d" ]; then
    pass "$ascii / $deva agree (exit $a)"
  else
    fail "$ascii exited $a but $deva exited $d"
  fi
}

alias_parity status  "स्थिति"
alias_parity status  "स्थिति" --config /nope/nope.yaml
alias_parity council "सभा" "hi" --agents claude
alias_parity council "सभा" "hi" --agents definitely-not-an-agent
alias_parity consult "परामर्श" "hi" --agent claude
alias_parity consult "परामर्श" "hi" --agent definitely-not-an-agent
alias_parity refine  "संशोधन" "hi" --reviewers definitely-not-an-agent

# A provider's output must never break the JSON contract, whatever it emits.
echo ""
echo "--- Hostile provider output ---"

HOSTILE_DIR="$(mktemp -d -t dootsabha-hostile)"
trap 'rm -rf "$HOSTILE_DIR"' EXIT
# shellcheck disable=SC2016  # $1 belongs to the generated script
printf '#!/usr/bin/env bash\n[[ "$1" == "--version" ]] && { echo "1.0"; exit 0; }\nprintf "\\033[31mRED\\033[0m answer\\n"\n' > "$HOSTILE_DIR/ansi"
# shellcheck disable=SC2016
printf '#!/usr/bin/env bash\n[[ "$1" == "--version" ]] && { echo "1.0"; exit 0; }\nprintf %%s "{\\"type\\":\\"result\\",\\"result\\":\\"trunc"\n' > "$HOSTILE_DIR/trunc"
chmod +x "$HOSTILE_DIR/ansi" "$HOSTILE_DIR/trunc"

for hostile in ansi trunc; do
  if { DOOTSABHA_PROVIDERS_GROK_BINARY="$HOSTILE_DIR/$hostile" \
       "$BINARY" consult --agent grok "hi" --json 2>/dev/null || true; } | one_json; then
    pass "provider emitting $hostile output still yields one JSON document"
  else
    fail "provider emitting $hostile output broke the JSON contract"
  fi
done

# config show --json must emit a JSON document on failure like every other
# command; it was emitting zero bytes, and `jq` exits 0 on empty input.
if { "$BINARY" config show --json --config /nope/nope.yaml 2>/dev/null || true; } | one_json; then
  pass "config show --json emits one document on config error"
else
  fail "config show --json emitted no/invalid JSON on config error"
fi
expect_exit 2 "unknown command"               "$BINARY" definitely-not-a-command
expect_exit 6 "config file missing"           "$BINARY" consult --agent claude "hi" --config /nope/nope.yaml
expect_exit 6 "config missing (council)"      "$BINARY" council "hi" --config /nope/nope.yaml
expect_exit 3 "single agent unavailable"      env DOOTSABHA_PROVIDERS_GROK_BINARY=/nonexistent/grok "$BINARY" consult --agent grok "hi"
expect_exit 3 "ALL agents failed"             env DOOTSABHA_PROVIDERS_CLAUDE_BINARY=/x DOOTSABHA_PROVIDERS_CODEX_BINARY=/x "$BINARY" council "hi" --agents claude,codex
expect_exit 5 "some agents failed"            env DOOTSABHA_PROVIDERS_GROK_BINARY=/nonexistent/grok "$BINARY" council "hi" --agents claude,grok

# Exit 5 is the least-covered non-zero code and it changed meaning in three
# places, so each path gets an explicit case. A stage failing AFTER usable output
# exists is 5, never 3 — callers follow `rc == 0 || rc == 5` and would otherwise
# discard content they already paid for.
FAILN="$(mktemp -t ds-failn)"
CTR="$(mktemp -t ds-ctr)"
trap 'rm -f "$CFG_DUR" "$CFG_HUMAN" "$CFG_PROV" "$CFG_ROUNDS" "$SCRATCH_HANG" "$FAILN" "$CTR"' EXIT
# shellcheck disable=SC2016  # $1/$C belong to the generated script
printf '#!/usr/bin/env bash\n[[ "$1" == "--version" ]] && { echo "1.0"; exit 0; }\nC="$MOCK_CTR"; n=$(cat "$C" 2>/dev/null || echo 0); n=${n:-0}; echo $((n+1)) > "$C"\nif [ "$n" -ge "${MOCK_OK:-1}" ]; then echo gone >&2; exit 1; fi\necho "{\\"result\\":\\"r\\",\\"session_id\\":\\"s\\",\\"cost_usd\\":0,\\"model\\":\\"m\\",\\"duration_ms\\":1}"\n' > "$FAILN"
chmod +x "$FAILN"

echo 0 > "$CTR"
# expect_exit runs each command twice (text, then --json). A counter-backed mock
# is stateful, so the counter must be reset before EACH invocation or the second
# run starts mid-sequence and reports a different code.
expect_exit_stateful() {
  local want="$1" desc="$2"; shift 2
  local rc=0 jrc=0 jout
  echo 0 > "$CTR"
  "$@" >/dev/null 2>&1 || rc=$?
  if [ "$rc" -eq "$want" ]; then
    pass "exit $want — $desc"
  else
    fail "exit $rc (want $want) — $desc"
  fi

  echo 0 > "$CTR"
  jout=$("$@" --json 2>/dev/null) || jrc=$?
  if [ "$jrc" -eq "$want" ]; then
    pass "exit $want — $desc [--json]"
  else
    fail "exit $jrc (want $want) — $desc [--json] — JSON mode disagrees with text mode"
  fi

  if printf '%s' "$jout" | one_json; then
    pass "one JSON document — $desc [--json]"
  else
    fail "stdout was not exactly one JSON document — $desc [--json]"
  fi
}

expect_exit_stateful 5 "council: synthesis fails after good dispatch" \
  env MOCK_OK=1 MOCK_CTR="$CTR" DOOTSABHA_PROVIDERS_CLAUDE_BINARY="$FAILN" \
  "$BINARY" council "hi" --agents claude --chair claude

expect_exit_stateful 5 "council: multi-round, later round loses all agents" \
  env MOCK_OK=2 MOCK_CTR="$CTR" DOOTSABHA_PROVIDERS_CLAUDE_BINARY="$FAILN" \
  "$BINARY" council "hi" --agents claude --chair claude --rounds 2

expect_exit_stateful 3 "council: every agent fails from the start" \
  env MOCK_OK=0 MOCK_CTR="$CTR" DOOTSABHA_PROVIDERS_CLAUDE_BINARY="$FAILN" \
  "$BINARY" council "hi" --agents claude --rounds 2

expect_exit 5 "review: reviewer fails, author content usable" \
  env DOOTSABHA_PROVIDERS_GROK_BINARY=/nonexistent/grok \
  "$BINARY" review "hi" --author claude --reviewer grok

expect_exit 3 "review: author fails, nothing to review" \
  env DOOTSABHA_PROVIDERS_GROK_BINARY=/nonexistent/grok \
  "$BINARY" review "hi" --author grok --reviewer claude

expect_exit 5 "refine: reviewer fails" \
  env DOOTSABHA_PROVIDERS_GROK_BINARY=/nonexistent/grok \
  "$BINARY" refine "hi" --author claude --reviewers grok

expect_exit 2 "plugin: unknown subcommand"   "$BINARY" plugin definitely-not-a-subcommand
expect_exit 2 "plugin inspect: unknown name" "$BINARY" plugin inspect definitely-not-a-plugin
expect_exit 2 "config: no subcommand"        "$BINARY" config
expect_exit 0 "config /dev/null is allowed"  "$BINARY" status --config /dev/null
expect_exit 5 "status degraded"               env DOOTSABHA_PROVIDERS_AGY_BINARY=/x "$BINARY" status
expect_exit 3 "status nothing usable"         env DOOTSABHA_PROVIDERS_CLAUDE_BINARY=/x DOOTSABHA_PROVIDERS_CODEX_BINARY=/x DOOTSABHA_PROVIDERS_AGY_BINARY=/x DOOTSABHA_PROVIDERS_GROK_BINARY=/x "$BINARY" status
expect_exit 6 "config error (council)"        "$BINARY" council "hi" --config /nope/nope.yaml
expect_exit 6 "config error (status)"         "$BINARY" status --config /nope/nope.yaml

# Invalid config VALUES, not just unparseable YAML. Viper coerces garbage to zero
# values, so `timeout: 5 minutes` silently ran with the built-in default.
CFG_DUR="$(mktemp -t ds-dur)"; CFG_HUMAN="$(mktemp -t ds-human)"
CFG_PROV="$(mktemp -t ds-prov)"; CFG_ROUNDS="$(mktemp -t ds-rounds)"
printf 'timeout: "not a duration"\n'       > "$CFG_DUR"
printf 'timeout: 5 minutes\n'              > "$CFG_HUMAN"
printf 'providers:\n  claude: "scalar"\n' > "$CFG_PROV"
printf 'council:\n  rounds: "three"\n'    > "$CFG_ROUNDS"
expect_exit 6 "config: unparseable duration"  "$BINARY" status --config "$CFG_DUR"
expect_exit 6 "config: human duration typo"   "$BINARY" status --config "$CFG_HUMAN"
expect_exit 6 "config: provider not a map"    "$BINARY" status --config "$CFG_PROV"
expect_exit 6 "config: rounds not a number"   "$BINARY" status --config "$CFG_ROUNDS"

# A character device is read to EOF by viper and never returns. `--config <(...)`
# is a real idiom, so this must fail fast rather than hang.
if [ -c /dev/zero ]; then
  RC=0; timeout 10 "$BINARY" status --config /dev/zero >/dev/null 2>&1 || RC=$?
  if [ "$RC" -eq 6 ]; then
    pass "config: character device rejected (no hang)"
  else
    fail "config: /dev/zero gave $RC (want 6; 124 means it hung)"
  fi
fi
expect_exit 2 "precedence: bad flag beats bad config" "$BINARY" council "hi" --zzz --config /nope/nope.yaml

# A timeout during a council must report 4, not be masked by the downstream
# synthesis failure it causes. Precedence says 4 > 5 > 3, and consult/review/
# refine already map DeadlineExceeded to 4 — council did not.
HANG="$SCRATCH_HANG"
expect_exit 4 "council: agent hangs (timeout beats partial)" \
  env DOOTSABHA_PROVIDERS_GROK_BINARY="$HANG" "$BINARY" council "hi" --agents claude,grok --timeout 3s
expect_exit 4 "council: ALL agents hang" \
  env DOOTSABHA_PROVIDERS_CLAUDE_BINARY="$HANG" DOOTSABHA_PROVIDERS_CODEX_BINARY="$HANG" "$BINARY" council "hi" --agents claude,codex --timeout 3s
expect_exit 4 "refine: reviewer hangs" \
  env DOOTSABHA_PROVIDERS_GROK_BINARY="$HANG" "$BINARY" refine "hi" --author claude --reviewers grok --timeout 3s

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
