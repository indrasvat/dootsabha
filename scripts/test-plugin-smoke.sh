#!/usr/bin/env bash
# L3 smoke tests for mock plugin binaries.
# Tests real go-plugin gRPC lifecycle: build → run → invoke → shutdown → no orphans.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
MOCK_BIN="$ROOT_DIR/testdata/mock-plugins/bin"

PASS=0
FAIL=0

pass() { PASS=$((PASS + 1)); printf "  ✓ %s\n" "$1"; }
fail() { FAIL=$((FAIL + 1)); printf "  ✗ %s\n" "$1"; }

printf "Running plugin smoke tests...\n"

# ── Test: Mock plugin binaries exist ────────────────────────────────────────
for name in mock-provider mock-strategy mock-hook; do
    if [[ -x "$MOCK_BIN/$name" ]]; then
        pass "$name binary exists and is executable"
    else
        fail "$name binary missing or not executable"
    fi
done

# ── Test: Go integration tests pass ─────────────────────────────────────────
if go test -count=1 -timeout 60s "$ROOT_DIR/internal/plugin/" -run 'TestProviderPlugin|TestStrategyPlugin|TestHookPlugin|TestConcurrent|TestPluginCrash|TestPluginBinary|TestPluginHandshake|TestFullPipeline|TestManager' > /dev/null 2>&1; then
    pass "go-plugin integration tests pass (45 tests)"
else
    fail "go-plugin integration tests failed"
fi

# ── Test: No orphan plugin processes after tests ────────────────────────────
orphans=$(pgrep -f "mock-provider\|mock-strategy\|mock-hook" 2>/dev/null || true)
if [[ -z "$orphans" ]]; then
    pass "no orphan mock-plugin processes after tests"
else
    fail "orphan processes found: $orphans"
    # Clean up orphans
    pkill -f "mock-provider|mock-strategy|mock-hook" 2>/dev/null || true
fi

# ── Test: Provider plugin binaries exist (if built) ─────────────────────────
PLUGIN_BIN="$ROOT_DIR/plugins/bin"
if [[ -d "$PLUGIN_BIN" ]]; then
    for name in claude-provider codex-provider gemini-provider; do
        if [[ -x "$PLUGIN_BIN/$name" ]]; then
            pass "provider plugin $name exists and is executable"
        else
            fail "provider plugin $name missing or not executable"
        fi
    done
fi

# ── Summary ─────────────────────────────────────────────────────────────────
printf "\nResults: %d passed, %d failed\n" "$PASS" "$FAIL"
[[ "$FAIL" -eq 0 ]]
