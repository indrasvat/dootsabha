#!/usr/bin/env bash
# L4 visual evidence for GitHub issue #20 — per-provider timeouts.
#
# Each frame is a real `bin/dootsabha` run in a real PTY, rasterised headless by
# shux. Deterministic frames use the mock providers with MOCK_<NAME>_DELAY so a
# starved reviewer can be demonstrated without spending minutes on a real CLI;
# the real-CLI frames are captured separately by issue-20-real.sh.
set -euo pipefail
# shellcheck disable=SC2016  # inner-shell $? / ${PIPESTATUS} must not expand out here

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT="$REPO/.shux/out/20"
BIN="bin/dootsabha"   # relative: absolute paths would leak $HOME into every frame

export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR_OVERRIDE:-/tmp/shux20}"
mkdir -p "$XDG_RUNTIME_DIR" "$OUT"


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

SESSIONS=()
cleanup() {
  for s in "${SESSIONS[@]:-}"; do
    [[ -n "$s" ]] && shux session kill "$s" >/dev/null 2>&1 || true
  done
  shux daemon stop >/dev/null 2>&1 || true
}
trap cleanup EXIT

# Provider env shared by every deterministic frame. Relative to $REPO, which
# each frame cds into — keeps the operator's home directory out of the PNGs.
MOCKENV=(
  "DOOTSABHA_PROVIDERS_CLAUDE_BINARY=testdata/mock-providers/mock-claude"
  "DOOTSABHA_PROVIDERS_CODEX_BINARY=testdata/mock-providers/mock-codex"
  "DOOTSABHA_PROVIDERS_AGY_BINARY=testdata/mock-providers/mock-agy"
  "DOOTSABHA_PROVIDERS_GROK_BINARY=testdata/mock-providers/mock-grok"
)

# frame <name> <cols>x<rows> <shell-snippet>
# Runs the snippet in a PTY, waits for the screen to settle, saves a PNG.
frame() {
  local name="$1" size="$2" snippet="$3"
  local cols="${size%x*}" rows="${size#*x}"
  local sess="i20-$name"

  # `set -x`-style echo of the command, then the command, then its exit code —
  # the three things a reviewer needs to trust the frame. The trailing sleep
  # holds the pane open so the snapshot catches the finished screen.
  local script="cd '$REPO'; export ${MOCKENV[*]}; export PS1=''; clear; $snippet; sleep 300"

  shux session create "$sess" -d --title "$name" -- bash -c "$script" >/dev/null
  SESSIONS+=("$sess")
  shux pane set-size -s "$sess" --cols "$cols" --rows "$rows" >/dev/null
  # Marker-driven, not settle-driven: the screen is legitimately quiet while a
  # provider subprocess runs, so "stopped repainting" would fire mid-pipeline.
  if ! wait_marker "$sess" 120; then
    echo "  ✗ $name: never reached the end marker" >&2
    shux pane capture -s "$sess" >&2
    return 1
  fi
  shux pane snapshot -s "$sess" -o "$OUT/$name.png" >/dev/null
  shux session kill "$sess" >/dev/null 2>&1 || true
  printf '  ✓ %s\n' "$name.png"
}

# show <label> <command...> — prints the command, runs it, prints the exit code.
show() {
  printf 'printf "\\033[1;36m$\\033[0m %%s\\n" %q; %s; printf "\\033[2m→ exit %%s\\033[0m\\n" "$?"; echo __FRAME_DONE__' \
    "$1" "$2"
}

echo "Capturing issue #20 evidence → $OUT"

# ---------------------------------------------------------------------------
# 1. The bug, and the fix. Author and reviewer each need 0.7s; the budget is
#    1.2s per call. Under one shared deadline the reviewer was killed at 0.5s.
# ---------------------------------------------------------------------------
frame "01-review-fixed" "120x34" "$(show \
  'dootsabha review --author codex --reviewer claude --timeout 1200ms --session-timeout 60s "…"' \
  'MOCK_CODEX_DELAY=0.7 MOCK_CLAUDE_DELAY=0.7 '"$BIN"' review --author codex --reviewer claude --timeout 1200ms --session-timeout 60s --config /dev/null "Explain per-provider timeouts in one line."')"

# 2. Session ceiling fires — and says so.
frame "02-session-timeout" "120x18" "$(show \
  'dootsabha review --timeout 60s --session-timeout 1s "…"' \
  'MOCK_CODEX_DELAY=0.7 MOCK_CLAUDE_DELAY=0.7 '"$BIN"' review --author codex --reviewer claude --timeout 60s --session-timeout 1s --config /dev/null "Explain per-provider timeouts in one line."')"

# 3. A genuinely slow single call is still an invocation timeout.
frame "03-invocation-timeout" "120x18" "$(show \
  'dootsabha review --timeout 400ms --session-timeout 60s "…"' \
  'MOCK_CODEX_DELAY=5 '"$BIN"' review --author codex --reviewer claude --timeout 400ms --session-timeout 60s --config /dev/null "Explain per-provider timeouts in one line."')"

# 4. refine — five calls under one ceiling (v1 → review → incorporate ×2).
frame "04-refine-pipeline" "120x48" "$(show \
  'dootsabha refine --author claude --reviewers codex,agy --timeout 1200ms --session-timeout 60s "…"' \
  'MOCK_CLAUDE_DELAY=0.7 MOCK_CODEX_DELAY=0.7 MOCK_AGY_DELAY=0.7 '"$BIN"' refine --author claude --reviewers codex,agy --timeout 1200ms --session-timeout 60s --config /dev/null "Name one benefit of per-call timeouts."')"

# 5. council — dispatch, peer review and synthesis are three separate budgets.
frame "05-council-pipeline" "120x40" "$(show \
  'dootsabha council --agents claude,codex --chair claude --timeout 1200ms --session-timeout 60s "…"' \
  'MOCK_CLAUDE_DELAY=0.7 MOCK_CODEX_DELAY=0.7 '"$BIN"' council --agents claude,codex --chair claude --timeout 1200ms --session-timeout 60s --config /dev/null "Name one benefit of per-call timeouts."')"

# 6. A --timeout larger than the ceiling warns instead of silently truncating.
frame "06-budget-inversion-warning" "120x26" "$(show \
  'dootsabha review --timeout 40m --session-timeout 30s "…"' \
  "$BIN"' review --author codex --reviewer claude --timeout 40m --session-timeout 30s --config /dev/null "hi"')"

# 7. Exit-code contract: both scopes are 4, and the message differs.
frame "07-exit-codes" "120x20" "$(show \
  'exit-code contract for both timeout scopes' \
  'set +e; printf "\033[1msession ceiling:\033[0m "; MOCK_CODEX_DELAY=0.7 MOCK_CLAUDE_DELAY=0.7 '"$BIN"' review --author codex --reviewer claude --timeout 60s --session-timeout 1s --config /dev/null "hi" 2>&1 >/dev/null | tail -1; printf "\033[2m  exit %s\033[0m\n" "${PIPESTATUS[0]}"; printf "\033[1minvocation:\033[0m      "; MOCK_CODEX_DELAY=5 '"$BIN"' review --author codex --reviewer claude --timeout 400ms --session-timeout 60s --config /dev/null "hi" 2>&1 >/dev/null | tail -1; printf "\033[2m  exit %s\033[0m\n" "${PIPESTATUS[0]}"; printf "\033[1mwithin budget:\033[0m  "; MOCK_CODEX_DELAY=0.7 MOCK_CLAUDE_DELAY=0.7 '"$BIN"' review --author codex --reviewer claude --timeout 1200ms --session-timeout 60s --config /dev/null "hi" >/dev/null 2>&1; printf "ok\033[2m — exit %s\033[0m\n" "$?"; set -e')"

# 8. Both flags are discoverable and describe different jobs.
frame "08-help-flags" "120x28" "$(show \
  'dootsabha review --help' \
  "$BIN"' review --help')"

echo "Done → $OUT"
