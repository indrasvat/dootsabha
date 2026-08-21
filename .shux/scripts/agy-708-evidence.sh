#!/usr/bin/env bash
# L4 visual evidence for task 708 — दूतसभा forwards its per-call budget to
# `agy --print-timeout`, so an agy timeout exits 4 rather than 3.
#
# Frames 01 and 03 use recorder/stub binaries deliberately: the point is the
# ARGV दूतसभा emits and the EXIT CODE it returns, neither of which a live model
# call can show. Frame 02 is the real `agy` 1.1.17 accepting what we emit.
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT="$REPO/.shux/out/708"
BIN="bin/dootsabha"   # relative — an absolute path would leak $HOME into frames

export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR_OVERRIDE:-/tmp/shux708}"
mkdir -p "$XDG_RUNTIME_DIR" "$OUT/.done"
rm -f "$OUT/.done"/*

COLS="${COLS:-100}"

SESSIONS=()
cleanup() {
  for s in "${SESSIONS[@]:-}"; do
    [[ -n "$s" ]] && shux session kill "$s" >/dev/null 2>&1 || true
  done
  shux daemon stop >/dev/null 2>&1 || true
}
trap cleanup EXIT

# wait_done <name> <max-seconds> — poll for a completion FILE, not an on-screen
# marker: a marker printed into the pane would appear in the evidence itself.
wait_done() {
  local name="$1" max="$2" waited=0
  while [[ "$waited" -lt "$max" ]]; do
    [[ -e "$OUT/.done/$name" ]] && { sleep 1; return 0; }
    sleep 2
    waited=$((waited + 2))
  done
  return 1
}

# frame <name> <max-seconds> <rows> <shell-command>
frame() {
  local name="$1" max="$2" rows="$3" cmd="$4"
  local sess="f708-$name"
  printf '  ▸ %-22s ' "$name"
  shux session create "$sess" -d --title "$name" --cwd "$REPO" \
    -- bash -lc "cd '$REPO' && clear && { $cmd; } ; touch '$OUT/.done/$name'; sleep 3600" \
    >/dev/null
  SESSIONS+=("$sess")
  shux pane set-size -s "$sess" --cols "$COLS" --rows "$rows" >/dev/null
  wait_done "$name" "$max" || printf '(timeout) '
  shux pane snapshot -s "$sess" -o "$OUT/$name.png" >/dev/null
  printf 'ok  (%s)\n' "$(du -h "$OUT/$name.png" | cut -f1)"
  shux session kill "$sess" >/dev/null 2>&1 || true
}

echo "L4 evidence → $OUT  (${COLS} cols)"

# An argv recorder: prints what दूतसभा actually emitted, then a valid envelope.
REC="$OUT/argv-agy"
cat > "$REC" <<'EOF'
#!/usr/bin/env bash
[[ "$1" == "--version" ]] && { echo "1.1.17"; exit 0; }
printf '%s\n' "$@" > "$REC_FILE"
printf '{"conversation_id":"x","status":"SUCCESS","response":"ok","usage":{"input_tokens":1,"output_tokens":1}}\n'
EOF
chmod +x "$REC"

# 1. The fix: the per-call budget reaches agy's own timeout flag, always ahead of it.
frame 01-argv 120 14 \
  "for T in 30s 15m 45m; do
     REC_FILE='$OUT/rec.txt' DOOTSABHA_PROVIDERS_AGY_BINARY='$REC' \\
       $BIN consult --agent agy --config /dev/null --timeout \"\$T\" --session-timeout 2h 'hi' >/dev/null 2>&1
     printf '  dootsabha --timeout %-5s ->  agy --print-timeout %s\\n' \"\$T\" \"\$(grep -A1 -- '--print-timeout' '$OUT/rec.txt' | tail -1)\"
   done"

# 2. The REAL CLI accepts the duration string we emit.
frame 02-real-accepts 300 10 \
  "agy --dangerously-skip-permissions --model 'Gemini 3.7 Flash (High)' \\
     --output-format json --print-timeout 15m30s -p 'Reply with exactly: PONG' \\
   | python3 -c 'import json,sys; d=json.load(sys.stdin); print(\"agy --print-timeout 15m30s\"); print(\"  status  :\", d[\"status\"]); print(\"  response:\", repr(d[\"response\"]))'"

# 3. The contract, both directions. mock-agy stands in for agy's own 5m default
#    with 1s, and fails with the real timeout envelope.
frame 03-exit-code 180 16 \
  "echo 'agy slower than its OWN default (1s), well inside dootsabha 20s budget:'
   MOCK_AGY_DELAY=2 DOOTSABHA_PROVIDERS_AGY_BINARY=testdata/mock-providers/mock-agy \\
     $BIN consult --agent agy --config /dev/null --timeout 20s 'PONG' >/dev/null 2>&1 \\
     && echo '  exit=0  — agy no longer self-kills first' || echo '  FAILED'
   echo
   echo 'dootsabha the SHORTER timer (700ms) — it must keep the exit code:'
   MOCK_AGY_DELAY=5 DOOTSABHA_PROVIDERS_AGY_BINARY=testdata/mock-providers/mock-agy \\
     $BIN consult --agent agy --config /dev/null --timeout 700ms 'PONG' >/dev/null 2>&1
   echo \"  exit=\$?  — 4 is 'raise the timeout', not 3 'try another agent'\""

echo
echo "Frames:"
find "$OUT" -name '*.png' -exec du -h {} + | sort -k2
