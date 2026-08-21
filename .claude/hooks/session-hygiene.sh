#!/usr/bin/env sh
# Stop — report anything this session left running.
#
# WARNS ONLY. This hook must never block: it fires when work is finished, and a
# block there is pure obstruction. Everything it reports is something an agent can
# leave behind without noticing — shux daemons outlive `session kill`, and a debug
# browser bridge holds a loopback port open.
#
# Every probe is guarded: on a cloud runner none of these tools exist, and their
# absence is not a finding.
set -u

FINDINGS=""

# shux daemons are per-runtime-dir and survive `session kill`.
if command -v pgrep >/dev/null 2>&1; then
    DIRS=$(pgrep -fl "shux __daemon" 2>/dev/null |
        sed -n 's|.*--socket \(.*\)/shux/shux\.sock.*|\1|p' || true)
    if [ -n "${DIRS:-}" ]; then
        COUNT=$(printf '%s\n' "$DIRS" | grep -c . || echo 0)
        LIST=$(printf '%s\n' "$DIRS" | sed 's/^/      XDG_RUNTIME_DIR=/;s/$/ shux daemon stop/')
        FINDINGS="${FINDINGS}  • ${COUNT} shux daemon(s) still running:
${LIST}
"
    fi
fi

# A Chrome remote-debugging bridge left listening on the loopback port.
if command -v curl >/dev/null 2>&1 &&
    curl -s -m 2 http://127.0.0.1:9222/json/version >/dev/null 2>&1; then
    FINDINGS="${FINDINGS}  • Chrome debug bridge up on :9222 — stop it: chrome-agent.sh stop
"
fi

[ -n "$FINDINGS" ] || exit 0

printf '\nSession hygiene — still running:\n%s\n' "$FINDINGS" >&2
exit 0
