#!/usr/bin/env bash
# L4 real-CLI evidence for GitHub issue #20 — same command, old binary vs new.
#
# Runs `dootsabha review` against the REAL codex and claude CLIs with a
# per-invocation budget that is smaller than the two calls take together. Under
# the old shared deadline the reviewer is starved and the run exits 4; under
# per-invocation budgets both agents finish and it exits 0.
#
# Requires: a `dootsabha-old` binary built from main (see BUILD below) and
# working codex + claude CLIs. Real API calls — this costs money and minutes.
#
#   git worktree add /tmp/ds-main-20 main
#   (cd /tmp/ds-main-20 && go build -o /tmp/ds-main-20/dootsabha-old ./cmd/dootsabha)
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT="$REPO/.shux/out/20"
OLD="${OLD_BIN:-/tmp/ds-main-20/dootsabha-old}"
BUDGET="${BUDGET:-60s}"
PROMPT="${PROMPT:-Name one benefit of per-invocation timeouts. One sentence.}"

export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR_OVERRIDE:-/tmp/shux20}"
mkdir -p "$XDG_RUNTIME_DIR" "$OUT"

[[ -x "$OLD" ]] || { echo "missing old binary at $OLD — see BUILD in the header" >&2; exit 1; }

SESS="i20-real"
cleanup() {
  shux session kill "$SESS" >/dev/null 2>&1 || true
  shux daemon stop >/dev/null 2>&1 || true
}
trap cleanup EXIT

# Both runs, one frame: identical flags, identical real providers, one binary
# each. `cp` keeps the old binary's path short so it does not wrap the line.
read -r -d '' SCRIPT <<EOF || true
cd '$REPO'
cp '$OLD' ./dootsabha-old
clear
printf '\033[1msame command, same real CLIs (codex author, claude reviewer), two binaries\033[0m\n'
printf '\033[2m  review --author codex --reviewer claude --timeout $BUDGET --session-timeout 20m\033[0m\n\n'
set +e
printf '\033[1;31mbefore\033[0m (main, one deadline shared by both calls)\n'
time ./dootsabha-old review --author codex --reviewer claude --timeout $BUDGET --session-timeout 20m "$PROMPT" >/dev/null 2>/tmp/i20-old.err
printf '  \033[2mexit %s —\033[0m %s\n\n' "\$?" "\$(tail -1 /tmp/i20-old.err)"
printf '\033[1;32mafter\033[0m (this branch, one budget per call)\n'
time ./bin/dootsabha review --author codex --reviewer claude --timeout $BUDGET --session-timeout 20m "$PROMPT" >/tmp/i20-new.out 2>/dev/null
printf '  \033[2mexit %s —\033[0m %s\n' "\$?" "\$(tail -2 /tmp/i20-new.out | head -1)"
set -e
rm -f ./dootsabha-old
echo __FRAME_DONE__
sleep 300
EOF

echo "Running real-CLI before/after (this makes real API calls)…"
shux session create "$SESS" -d --title "real" -- bash -c "$SCRIPT" >/dev/null
shux pane set-size -s "$SESS" --cols 120 --rows 30 >/dev/null
if ! shux pane wait-for -s "$SESS" --text '__FRAME_DONE__' --timeout-ms 900000 >/dev/null; then
  echo "  ✗ never reached the end marker" >&2
  shux pane capture -s "$SESS" >&2
  exit 1
fi
shux pane snapshot -s "$SESS" -o "$OUT/09-real-cli-before-after.png" >/dev/null
shux pane capture -s "$SESS" > "$OUT/09-real-cli-before-after.txt"
echo "  ✓ 09-real-cli-before-after.png"
