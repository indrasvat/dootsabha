#!/usr/bin/env bash
# L4 visual evidence for task 710 (#31) — grok's read-only sandbox refused to
# start under दूतसभा's isolated HOME when /var/run/docker.sock links into the
# real home (Docker Desktop's link).
#
# Every frame is the REAL `grok` CLI through a real dootsabha binary in a real PTY,
# rasterised headless by shux. BEFORE = the installed release (v0.10.1),
# AFTER = this branch's bin/dootsabha.
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT="$REPO/.shux/out/710"
BEFORE="${BEFORE_BIN:-$(command -v dootsabha)}"
AFTER="bin/dootsabha"   # relative — an absolute path would leak $HOME into frames

export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR_OVERRIDE:-/tmp/shux710}"
mkdir -p "$XDG_RUNTIME_DIR" "$OUT"

COLS="${COLS:-100}"
TILDE="sed \"s#\$HOME#~#g\""   # keep the real home path out of the frames

SESSIONS=()
cleanup() {
  for s in "${SESSIONS[@]:-}"; do
    [[ -n "$s" ]] && shux session kill "$s" >/dev/null 2>&1 || true
  done
  shux daemon stop >/dev/null 2>&1 || true
}
trap cleanup EXIT

DONE_DIR="$OUT/.done"
mkdir -p "$DONE_DIR"
find "$DONE_DIR" -type f -name '[0-9]*' -delete

# wait_done <name> <max-seconds> — poll for a completion FILE, not an on-screen
# marker: a marker printed into the pane would appear in the evidence itself.
wait_done() {
  local name="$1" max="$2" waited=0
  while [[ "$waited" -lt "$max" ]]; do
    [[ -e "$DONE_DIR/$name" ]] && { sleep 1; return 0; }
    sleep 2
    waited=$((waited + 2))
  done
  return 1
}

# frame <name> <max-seconds> <rows> <shell-command>
frame() {
  local name="$1" max="$2" rows="$3" cmd="$4"
  local sess="f710-$name"
  printf '  ▸ %-28s ' "$name"
  shux session create "$sess" -d --title "$name" --cwd "$REPO" \
    -- bash -lc "cd '$REPO' && clear && { $cmd; } ; touch '$DONE_DIR/$name'; sleep 3600" \
    >/dev/null
  SESSIONS+=("$sess")
  shux pane set-size -s "$sess" --cols "$COLS" --rows "$rows" >/dev/null
  if wait_done "$name" "$max"; then
    shux pane snapshot -s "$sess" -o "$OUT/$name.png" >/dev/null
    printf 'ok  (%s)\n' "$(du -h "$OUT/$name.png" | cut -f1)"
  else
    shux pane snapshot -s "$sess" -o "$OUT/$name.png" >/dev/null
    printf 'TIMEOUT after %ss — partial frame kept\n' "$max"
  fi
  shux session kill "$sess" >/dev/null 2>&1 || true
}

# hdr <text> — a cyan header line; <text> must not contain a single quote.
hdr() { printf "printf '\\\\033[1;36m%%s\\\\033[0m\\\\n' '%s'" "$1"; }

echo "L4 evidence → $OUT  (${COLS} cols)"

# 1. The premise: Docker Desktop's socket link points into the real home.
frame 01-premise 30 6 \
  "$(hdr '$ ls -l /var/run/docker.sock'); ls -l /var/run/docker.sock | $TILDE; grok --version"

# 2. BEFORE: the released binary — grok refuses to start under the empty HOME.
frame 02-before 240 12 \
  "$(hdr "\$ dootsabha consult --agent grok \"Reply with exactly: PONG\"   # v0.10.1"); \
   '$BEFORE' consult --agent grok --config /dev/null 'Reply with exactly: PONG'; echo \"exit=\$?\""

# 3. AFTER: this branch — the same call answers.
frame 03-after 240 8 \
  "$(hdr "\$ bin/dootsabha consult --agent grok \"Reply with exactly: PONG\""); \
   $AFTER consult --agent grok --config /dev/null 'Reply with exactly: PONG'; echo \"exit=\$?\""

# 4. Still isolated: the HOME grok was handed holds only the two socket-dir links.
frame 04-isolated-home 30 12 \
  "$(hdr '$ ls -la <isolated grok HOME> <isolated grok HOME>/.docker'); \
   d=\$(ls -td \"\${TMPDIR:-/tmp}\"/dootsabha-grok-home-* | head -1); \
   ls -la \"\$d\" | tail -n +2 | $TILDE; ls -la \"\$d/.docker\" | tail -n +2 | $TILDE"

# 5. Still read-only: grok's own write to the working directory is refused.
frame 05-read-only 300 12 \
  "$(hdr '$ bin/dootsabha consult --agent grok  # asked to write .shux/out/710/probe.txt'); \
   $AFTER consult --agent grok --config /dev/null \
     'Run exactly this with your bash tool: echo hi > .shux/out/710/probe.txt — then reply with only the exact error text or OK.'; \
   $(hdr '$ ls .shux/out/710/probe.txt'); ls .shux/out/710/probe.txt"

# 6. The motivating case: a council that includes grok.
frame 06-council 900 34 \
  "$(hdr '$ bin/dootsabha council --agents claude,codex,grok --chair claude'); \
   $AFTER council --agents claude,codex,grok --chair claude \
     'One short sentence each: should a CLI wrapper fail loud or fall back silently when a sandbox cannot be applied?'; \
   echo \"exit=\$?\""

echo
echo "Frames:"
find "$OUT" -name '*.png' -exec du -h {} + | sort -k2
