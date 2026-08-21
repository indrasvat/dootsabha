#!/usr/bin/env bash
# L4 visual evidence for task 707 — default agy model bumped to Gemini 3.7 Flash
# (High), and `--output-format json` now parsed.
#
# Every frame is the REAL `agy` CLI (1.1.17) doing REAL work through a real
# `bin/dootsabha` in a real PTY, rasterised headless by shux. No mock providers
# appear in any frame — the whole point is to show 3.7 Flash actually running.
#
# Panes are deliberately narrow (100x30) so each cell rasterises large: the PNGs
# read as zoomed-in, not as a wall of 200-column text.
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT="$REPO/.shux/out/707"
BIN="bin/dootsabha"   # relative — an absolute path would leak $HOME into frames

export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR_OVERRIDE:-/tmp/shux707}"
mkdir -p "$XDG_RUNTIME_DIR" "$OUT"

COLS="${COLS:-100}"
ROWS="${ROWS:-30}"

SESSIONS=()
cleanup() {
  for s in "${SESSIONS[@]:-}"; do
    [[ -n "$s" ]] && shux session kill "$s" >/dev/null 2>&1 || true
  done
  shux daemon stop >/dev/null 2>&1 || true
}
trap cleanup EXIT

# wait_marker <session> <max-seconds> — poll for the end marker. shux's
# `pane wait-for --timeout-ms` caps at 60s, shorter than a real agy council.
wait_marker() {
  local sess="$1" max="$2" waited=0
  while [[ "$waited" -lt "$max" ]]; do
    if shux pane capture -s "$sess" --lines 500 2>/dev/null | grep -q '__FRAME_DONE__'; then
      return 0
    fi
    sleep 3
    waited=$((waited + 3))
  done
  return 1
}

# frame <name> <max-seconds> <shell-command>
# Runs the command in a fresh pane, waits for it to finish, snapshots to PNG.
frame() {
  local name="$1" max="$2" cmd="$3"
  local sess="f707-$name"
  printf '  ▸ %-28s ' "$name"
  shux session create "$sess" -d --title "$name" --cwd "$REPO" \
    -- bash -lc "cd '$REPO' && clear && { $cmd; } ; echo; echo __FRAME_DONE__; sleep 3600" \
    >/dev/null
  SESSIONS+=("$sess")
  shux pane set-size -s "$sess" --cols "$COLS" --rows "$ROWS" >/dev/null
  if wait_marker "$sess" "$max"; then
    shux pane snapshot -s "$sess" -o "$OUT/$name.png" >/dev/null
    printf 'ok  (%s)\n' "$(du -h "$OUT/$name.png" | cut -f1)"
  else
    shux pane snapshot -s "$sess" -o "$OUT/$name.png" >/dev/null
    printf 'TIMEOUT after %ss — partial frame kept\n' "$max"
  fi
  shux session kill "$sess" >/dev/null 2>&1 || true
}

echo "L4 evidence → $OUT  (${COLS}x${ROWS})"

# 1. The premise: the CLI's own list leads with 3.7 Flash, and still offers 3.6/3.5.
frame 01-agy-models 90 'agy models | head -12'

# 2. दूतसभा agrees — status reports agy 1.1.17 on Gemini 3.7 Flash (High).
frame 02-status 120 "$BIN status --config /dev/null"

# 3. The shipped default, with NO config file, answers a real question about this repo.
frame 03-consult-default 400 \
  "$BIN consult --agent agy --config /dev/null \
     'In one sentence, what does stripAgyPinnedFlags in internal/providers/agy.go protect against?'"

# 4. THE GAP THIS CLOSES: tokens and a session id, which were 0/empty before 707.
frame 04-consult-json 400 \
  "$BIN consult --agent agy --json --config /dev/null 'Reply with exactly: PONG' \
   | python3 -c 'import json,sys; d=json.load(sys.stdin)[\"data\"]; \
       print(\"Model    :\", d[\"Model\"]); print(\"TokensIn :\", d[\"TokensIn\"]); \
       print(\"TokensOut:\", d[\"TokensOut\"]); print(\"SessionID:\", d[\"SessionID\"]); \
       print(\"CostUSD  :\", d[\"CostUSD\"], \"(agy reports none — never estimated)\")'"

# 5. A pinned 3.5 is NOT rewritten — the bump is a default, not a migration.
frame 05-pin-3-5 400 \
  "printf 'providers:\\n  agy:\\n    model: Gemini 3.5 Flash (High)\\n' > /tmp/pin35.yaml && \
   $BIN consult --agent agy --json --config /tmp/pin35.yaml 'Reply with exactly: PONG' \
   | python3 -c 'import json,sys; print(\"Model:\", json.load(sys.stdin)[\"data\"][\"Model\"])'"

# 6. In JSON mode agy writes failures to STDOUT and leaves stderr EMPTY. Before 707
#    dootsabha read stderr and showed a bare exit code, losing the reason entirely.
frame 06-error-envelope 200 \
  "printf 'providers:\\n  agy:\\n    model: not-a-real-model-xyz\\n' > /tmp/badmodel.yaml && \
   $BIN consult --agent agy --config /tmp/badmodel.yaml 'hi'; echo \"exit=\$?\""

# 7. Real multi-agent council with 3.7 Flash chairing alongside codex and grok.
frame 07-council 900 \
  "$BIN council --agents codex,agy,grok --chair agy \
     'One sentence each: when a CLI reports a per-turn status of ERROR but exits 0 with a usable answer, which should a wrapper trust? Be terse.'"

# 8. 3.7 Flash reviewing this very change.
frame 08-review 900 \
  "$BIN review --author codex --reviewer agy \
     'Summarise in 3 bullets what changed in this branch (feat/agy-gemini-3-7-flash) and whether it is safe.'"

echo
echo "Frames:"
find "$OUT" -name '*.png' -exec du -h {} + | sort -k2
