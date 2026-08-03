#!/usr/bin/env bash
# L4 real-CLI evidence for GitHub issue #20 — same command, old binary vs new.
#
# Runs a real `dootsabha council` (codex + agy, 5 provider calls across dispatch,
# peer review and synthesis) with a per-invocation budget larger than any single
# call but smaller than all of them together. Under the old shared deadline the
# later stages are starved and the run exits 4; under per-invocation budgets
# every stage finishes and it exits 0.
#
# Measured on this machine: each call ~7-10s, whole council ~50s. The 25s budget
# leaves every individual call ~2.5x headroom while the pipeline as a whole
# cannot fit inside one budget — which is exactly the shape issue #20 broke on.
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
BUDGET="${BUDGET:-25s}"
PROMPT="${PROMPT:-Name one benefit of per-invocation timeouts. One sentence.}"
AGENTS="${AGENTS:-codex,agy}"

export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR_OVERRIDE:-/tmp/shux20}"
mkdir -p "$XDG_RUNTIME_DIR" "$OUT"

[[ -x "$OLD" ]] || { echo "missing old binary at $OLD — see BUILD in the header" >&2; exit 1; }


# wait_marker <session> <max-seconds> — block until the pane shows the end
# marker. shux's `pane wait-for --timeout-ms` caps at 60s, which is shorter than
# a real-CLI council takes, so poll it instead of asking for one long wait.
wait_marker() {
  local sess="$1" max="$2" waited=0
  while [[ "$waited" -lt "$max" ]]; do
    if shux pane capture -s "$sess" --lines 400 2>/dev/null | grep -q '__FRAME_DONE__'; then
      return 0
    fi
    sleep 3
    waited=$((waited + 3))
  done
  return 1
}

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
printf '\033[1msame command, same real CLIs ($AGENTS), two binaries\033[0m\n'
printf '\033[2m  council --agents $AGENTS --chair codex --timeout $BUDGET --session-timeout 20m\033[0m\n'
printf '\033[2m  5 provider calls: 2 dispatch, 2 peer review, 1 synthesis\033[0m\n\n'
set +e
printf '\033[1;31mbefore\033[0m (main — one deadline shared by all five calls)\n'
time ./dootsabha-old council --agents $AGENTS --chair codex --timeout $BUDGET --session-timeout 20m -q "$PROMPT" >/dev/null 2>/tmp/i20-old.err
printf '  \033[1;31mexit %s\033[0m — %s\n\n' "\$?" "\$(tail -1 /tmp/i20-old.err)"
printf '\033[1;32mafter\033[0m (this branch — one budget per call)\n'
time ./bin/dootsabha council --agents $AGENTS --chair codex --timeout $BUDGET --session-timeout 20m -q "$PROMPT" >/tmp/i20-new.out 2>/dev/null
printf '  \033[1;32mexit %s\033[0m — synthesis: %s\n' "\$?" "\$(grep -v '^\\s*$' /tmp/i20-new.out | tail -2 | head -1 | cut -c1-80)"
set -e
rm -f ./dootsabha-old
echo __FRAME_DONE__
sleep 300
EOF

echo "Running real-CLI before/after (this makes real API calls)…"
shux session create "$SESS" -d --title "real" -- bash -c "$SCRIPT" >/dev/null
shux pane set-size -s "$SESS" --cols 120 --rows 30 >/dev/null
if ! wait_marker "$SESS" 600; then
  echo "  ✗ never reached the end marker" >&2
  shux pane capture -s "$SESS" >&2
  exit 1
fi
shux pane snapshot -s "$SESS" -o "$OUT/09-real-cli-before-after.png" >/dev/null
shux pane capture -s "$SESS" > "$OUT/09-real-cli-before-after.txt"
echo "  ✓ 09-real-cli-before-after.png"
