#!/usr/bin/env bash
# L4 evidence for task 709 — the enforcement layers.
#
# What is under test here is BEHAVIOUR, not rendering: whether a guard fires, what
# it says, and — just as important — what it lets through. So the frames drive the
# hooks with real Claude Code hook JSON and show the verdicts side by side.
#
# branch-guard runs against a throwaway repo in $TMPDIR, because proving it blocks
# on the default branch would otherwise mean checking this repo out onto main.
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT="$REPO/.shux/out/709"

export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR_OVERRIDE:-/tmp/shux709}"
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
  local sess="f709-$name"
  printf '  ▸ %-24s ' "$name"
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

# 1. The staging guard: what it stops, and what it must NOT stop.
frame 01-staging-guard 120 20 \
  "printf '%-32s %s\\n' COMMAND VERDICT; printf '%.0s─' {1..46}; echo
   for c in 'git add -A' 'git add .' 'git commit -am wip' 'cd /tmp && git add -A' \\
            'git add internal/foo.go' 'echo git add -A'; do
     j=\$(python3 -c 'import json,sys; print(json.dumps({\"tool_name\":\"Bash\",\"tool_input\":{\"command\":sys.argv[1]}}))' \"\$c\")
     rc=0; printf '%s' \"\$j\" | .claude/hooks/staging-guard.sh >/dev/null 2>&1 || rc=\$?
     [ \"\$rc\" = 2 ] && v='BLOCKED' || v='allowed'
     printf '%-32s %s\\n' \"\$c\" \"\$v\"
   done"

# 2. The branch guard, in a throwaway repo so `main` can actually be tested.
frame 02-branch-guard 120 18 ".shux/scripts/gates-709-branch-demo.sh"

# 3. The evidence test — including its own failure fixtures, which is the part the
#    old gate never had.
frame 03-evidence-test 300 26 \
  "export GOROOT=\$HOME/sdk/go1.26.0; export PATH=\$GOROOT/bin:\$PATH
   go test ./internal/tasks/ -v 2>&1 | grep -E '^(=== RUN|--- PASS|--- FAIL|ok|PASS)' | head -20"

echo
echo "Frames:"
find "$OUT" -name '*.png' -exec du -h {} + | sort -k2
