#!/usr/bin/env bash
# L4 visual evidence for task 706 — default grok model bumped to grok-4.6.
#
# Every frame is the REAL `grok` CLI (1.0.5) doing REAL work through a real
# `bin/dootsabha` in a real PTY, rasterised headless by shux. No mock providers
# appear in any frame — the whole point is to show grok-4.6 actually running.
#
# Panes are deliberately narrow (100x30) so each cell rasterises large: the PNGs
# read as zoomed-in, not as a wall of 200-column text.
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT="$REPO/.shux/out/706"
BIN="bin/dootsabha"   # relative — an absolute path would leak $HOME into frames

export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR_OVERRIDE:-/tmp/shux706}"
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
# `pane wait-for --timeout-ms` caps at 60s, shorter than a real grok council.
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
  local sess="f706-$name"
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

# 1. The premise: the real CLI's own default is grok-4.6.
frame 01-grok-models 60 'grok models'

# 2. dootsabha agrees — status reports the grok row as grok-4.6, live.
frame 02-status 120 "$BIN status"

# 3. The shipped default, with NO config file, is what actually runs.
frame 03-consult-default 300 \
  "$BIN consult --agent grok --config /dev/null \
     'In one sentence, what does internal/providers/grok.go do?'"

# 4. JSON contract: the backend id comes back as grok-4.6-build.
frame 04-consult-json 300 \
  "$BIN consult --agent grok --json --config /dev/null 'Reply with exactly: PONG' \
   | python3 -m json.tool"

# 5. A pinned grok-4.5 is NOT rewritten — the bump is a default, not a migration.
frame 05-pin-4-5 300 \
  "printf 'providers:\\n  grok:\\n    model: grok-4.5\\n' > /tmp/pin45.yaml && \
   $BIN consult --agent grok --json --config /tmp/pin45.yaml 'Reply with exactly: PONG' \
   | python3 -c 'import json,sys; d=json.load(sys.stdin)[\"data\"]; print(\"Model :\", d[\"Model\"]); print(\"Cost  :\", d[\"CostUSD\"])'"

# 6. Real multi-agent council with grok-4.6 deliberating alongside codex and agy.
frame 06-council 900 \
  "$BIN council --agents codex,agy,grok --chair grok \
     'One sentence each: is bumping a default model across four unsynced declaration sites risky? Be terse.'"

# 7. grok-4.6 reviewing this very change.
frame 07-review 900 \
  "$BIN review --author codex --reviewer grok \
     'Summarise in 3 bullets what changed in this branch (feat/grok-4-6-default) and whether it is safe.'"

echo
echo "Frames:"
find "$OUT" -name '*.png' -exec du -h {} + | sort -k2
