#!/usr/bin/env bash
# L4 evidence for task 711 / GitHub issue #33: SIGTERM sent to dootsabha alone,
# with REAL `claude` and `grok` agents mid-call, recorded for two builds.
#
# Each run is one shux window: the driver on top, and below it an independent
# observer — a clock plus `ps` of the agents carrying this run's needle, with
# their parent pid (1 = orphaned). The observer reads only `ps`, so it reports
# what the system did, not what the driver claims.
#
#   .shux/scripts/issue-33-evidence.sh <before-binary> <after-binary>
#
# Writes .shux/out/711/{before,after}.mp4 (real time) and the last frame of
# each as PNG. Copy the binaries under /tmp first: their path is never shown,
# but the run happens in /tmp so no home path reaches a frame.
set -euo pipefail

BEFORE="${1:?usage: issue-33-evidence.sh <before-binary> <after-binary>}"
AFTER="${2:?usage: issue-33-evidence.sh <before-binary> <after-binary>}"
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT="$REPO/.shux/out/711"
WORK="/tmp/ev711"
export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR_OVERRIDE:-/tmp/shux711}"
mkdir -p "$XDG_RUNTIME_DIR" "$OUT" "$WORK"

SESSIONS=()
cleanup() {
  for s in "${SESSIONS[@]:-}"; do
    [[ -n "$s" ]] && shux session kill "$s" >/dev/null 2>&1 || true
  done
  shux daemon stop >/dev/null 2>&1 || true
}
trap cleanup EXIT

# The observer's probe: agents with this run's needle, their parent and age.
cat >"$WORK/probe.sh" <<'EOF'
#!/usr/bin/env bash
n="$1"
G=$'\e[32m' R=$'\e[31m' D=$'\e[2m' X=$'\e[0m'
pids=$(pgrep -f "(claude|grok) -p .*$n" | paste -sd, -)
if [ -z "$pids" ]; then printf '  %sno agents running%s\n' "$G" "$X"; exit 0; fi
printf '%s  PID  PPID  AGE    AGENT                      CHILDREN%s\n' "$D" "$X"
ps -ww -o pid=,ppid=,etime=,command= -p "$pids" | while read -r pid ppid etime cmd _; do
  c=$X; [ "$ppid" = 1 ] && c=$R
  printf '%s%6s %5s %6s  %-26s %s%s\n' "$c" "$pid" "$ppid" "$etime" "$cmd -p \"Write an essay…\"" \
    "$(pgrep -P "$pid" | wc -l | tr -d ' ')" "$X"
done
EOF

cat >"$WORK/observe.sh" <<'EOF'
#!/usr/bin/env bash
n="$1" stop="$2" t0=$(date +%s)
B=$'\e[1m' C=$'\e[36m' X=$'\e[0m'
while [ ! -e "$stop" ]; do
  # Build the whole frame first: clearing, then probing, tears the snapshot.
  frame=$(
    printf '%s⏱  %s  +%ss%s\n\n' "$C" "$(date +%H:%M:%S)" "$(( $(date +%s) - t0 ))" "$X"
    printf '%sdootsabha processes%s\n  %s running\n\n' "$B" "$X" "$(pgrep -f 'dootsabha-[^ ]* consult' | wc -l | tr -d ' ')"
    printf '%sagents, independent ps (PPID 1 = orphaned)%s\n' "$B" "$X"
    bash "${0%/*}/probe.sh" "$n"
  )
  printf '\e[H\e[2J%s\n' "$frame"
  sleep 0.5
done
EOF

cat >"$WORK/drive.sh" <<'EOF'
#!/usr/bin/env bash
DS="$1" N="$2" STOP="$3"
B=$'\e[1m' C=$'\e[36m' Y=$'\e[33m' G=$'\e[32m' X=$'\e[0m'
step() { printf '\n%s%s▶ %s%s\n' "$B" "$C" "$*" "$X"; }
cmd()  { printf '%s$ %s%s\n' "$Y" "$*" "$X"; }
P="Write a 3000-word essay on how Unix process groups and signals work. Tag $N"
step "Build under test"; cmd "dootsabha --version"; "$DS" --version
step "Real claude and grok calls, each with a 10m budget"
declare -A pid
for a in claude grok; do
  cmd "dootsabha consult --agent $a --timeout 10m \"Write a 3000-word essay…\" &"
  "$DS" consult --agent "$a" --timeout 10m "$P" >/dev/null 2>"${STOP%/*}/$a.$N.err" &
  pid[$a]=$!
done
for _ in $(seq 1 120); do
  [ "$(pgrep -f "(claude|grok) -p .*$N" | wc -l)" -ge 2 ] && break
  sleep 0.25
done
sleep 4
step "SIGTERM to dootsabha only (what timeout(1) or a stopped background task sends)"
for a in claude grok; do cmd "kill -TERM ${pid[$a]}    # dootsabha running $a"; kill -TERM "${pid[$a]}"; done
for a in claude grok; do
  rc=0; wait "${pid[$a]}" || rc=$?
  printf '%s dootsabha (%s) exit status %s%s\n' "$G" "$a" "$rc" "$X"
  sed 's/^/   stderr: /' "${STOP%/*}/$a.$N.err" | grep -v '^   stderr: $' || true
done
step "Watch the agents below for 15 seconds"
sleep 15
touch "$STOP"
EOF

# record <label> <binary>
record() {
  local label="$1" bin="$2" n s stop frames pane i=0
  n="EV711$(date +%s)$RANDOM"
  s="ev711$label"
  stop="$WORK/$label.stop"
  frames="$WORK/$label-frames"
  rm -rf "$stop" "$frames"
  mkdir -p "$frames"
  cp "$bin" "$WORK/dootsabha-$label"
  SESSIONS+=("$s")
  pane=$(shux --format json session create "$s" -d --cwd "$WORK" -- bash -l | jq -r .pane_id)
  shux pane split -s "$s" -p "$pane" -d horizontal -- bash "$WORK/observe.sh" "$n" "$stop" >/dev/null
  shux pane set-size -s "$s" -p "$pane" --cols 132 --rows 24 >/dev/null
  # Keys sent before the login shell draws its prompt are lost.
  shux pane wait-for -s "$s" -p "$pane" --text "ev711" --timeout-ms 30000 >/dev/null
  shux pane send-keys -s "$s" -p "$pane" --text "clear; bash $WORK/drive.sh $WORK/dootsabha-$label $n $stop" >/dev/null
  shux pane send-keys -s "$s" -p "$pane" --data Cg== >/dev/null
  # Start filming once the driver has cleared the screen: the prompt shows the
  # user's name and path, and the first frame becomes the player's poster.
  shux pane wait-for -s "$s" -p "$pane" --text "Build under test" --timeout-ms 30000 >/dev/null
  local start=$SECONDS
  while [[ ! -e "$stop" && $((SECONDS - start)) -lt 150 ]]; do
    shux window snapshot -s "$s" --cols 132 --rows 40 -o "$(printf '%s/%05d.png' "$frames" "$i")" >/dev/null
    i=$((i + 1))
    sleep 0.2
  done
  local fps
  fps=$(awk -v n="$i" -v t="$((SECONDS - start))" 'BEGIN { printf "%.2f", (t > 0 ? n / t : 4) }')
  ffmpeg -v error -y -framerate "$fps" -i "$frames/%05d.png" -vf "pad=ceil(iw/2)*2:ceil(ih/2)*2" \
    -c:v libx264 -profile:v high -level:v 4.2 -preset slow -crf 16 -pix_fmt yuv420p \
    -movflags +faststart "$OUT/$label.mp4" # iOS will not decode above level 5.x
  cp "$(printf '%s/%05d.png' "$frames" "$((i - 1))")" "$OUT/$label-end.png"
  shux session kill "$s" >/dev/null
  # Stop whatever the build left running, now that it is on film.
  for p in $(pgrep -f "(claude|grok) -p .*$n" || true); do kill -KILL -- "-$p" 2>/dev/null || kill -KILL "$p" 2>/dev/null || true; done
  echo "$label: $i frames at $fps fps -> $OUT/$label.mp4"
}

record before "$BEFORE"
record after "$AFTER"
echo "Read $OUT/before-end.png and $OUT/after-end.png before attaching them."
