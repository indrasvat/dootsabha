#!/usr/bin/env bash
# L4 evidence for task 712 / GitHub issue #34: what an agent that put a REAL
# codex + grok council in the background can see while it runs, for two builds.
#
# Each run is one shux window: the caller on top, and below it an observer that
# shows everything a caller can read mid-run — the bytes on stderr, the progress
# file, and `ps`. The observer reads only those, never the caller's state.
#
#   .shux/scripts/issue-34-evidence.sh <before-binary> <after-binary>
#
# Writes .shux/out/712/{before,after}.mp4 (sped up 1.5x after a cut of the idle
# middle) and the last frame of each as PNG. The run happens in /tmp so no home
# path reaches a frame.
set -euo pipefail

BEFORE="${1:?usage: issue-34-evidence.sh <before-binary> <after-binary>}"
AFTER="${2:?usage: issue-34-evidence.sh <before-binary> <after-binary>}"
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT="$REPO/.shux/out/712"
WORK="/tmp/ev712"
export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR_OVERRIDE:-/tmp/shux712}"
mkdir -p "$XDG_RUNTIME_DIR" "$OUT" "$WORK"

SESSIONS=()
cleanup() {
  for s in "${SESSIONS[@]:-}"; do
    [[ -n "$s" ]] && shux session kill "$s" >/dev/null 2>&1 || true
  done
  shux daemon stop >/dev/null 2>&1 || true
}
trap cleanup EXIT

# The observer: what a caller can read while the council runs.
cat >"$WORK/observe.sh" <<'EOF'
#!/usr/bin/env bash
dir="$1" stop="$2" t0=$(date +%s)
B=$'\e[1m' C=$'\e[36m' D=$'\e[2m' G=$'\e[32m' R=$'\e[31m' Y=$'\e[33m' X=$'\e[0m'
while [ ! -e "$stop" ]; do
  frame=$(
    printf '%s⏱  +%ss%s   %swhat the calling agent can read right now%s\n\n' "$C" "$(( $(date +%s) - t0 ))" "$X" "$B" "$X"
    printf '%s1. stderr%s  %s bytes\n\n' "$B" "$X" "$(wc -c < "$dir/err" 2>/dev/null | tr -d ' ' || echo 0)"
    printf '%s2. progress file%s\n' "$B" "$X"
    if [ -s "$dir/run.ndjson" ]; then
      jq -r 'select(.event != "run_started") |
        [.ts[11:19], .event, (.stage // ""), (.agent // ""), (.status // ""),
         (if .duration_ms then "\(.duration_ms / 1000 | floor)s" else "" end),
         (if .exit_code != null then "exit_code=\(.exit_code)" else "" end)] | @tsv' "$dir/run.ndjson" |
      while IFS=$'\t' read -r ts ev st ag stat dur code; do
        c=$X; case "$stat" in ok) c=$G;; error|timeout) c=$R;; esac
        [ "$ev" = run_finished ] && c=$B$G
        printf '   %s%s  %-13s %-9s %-6s %s%-8s%s %5s %s%s\n' "$D" "$ts" "$ev" "$st" "$ag" "$c" "$stat" "$X" "$dur" "$code" "$X"
      done
    elif [ -e "$dir/no-flag" ]; then
      printf '   %snone — this build has no --progress-file%s\n' "$R" "$X"
    else
      printf '   %s(not yet)%s\n' "$D" "$X"
    fi
    printf '\n%s3. ps — the only other signal%s\n' "$B" "$X"
    found=0
    for a in "codex exec" "grok -p"; do
      if pgrep -f "^$a|/$a" >/dev/null 2>&1; then
        printf '   %s%s is running%s %s(which stage? unknown)%s\n' "$Y" "${a%% *}" "$X" "$D" "$X"; found=1
      fi
    done
    [ "$found" = 0 ] && printf '   %sno agent process%s\n' "$D" "$X"
  )
  printf '\e[H\e[2J%s\n' "$frame"
  sleep 1
done
EOF

cat >"$WORK/drive.sh" <<'EOF'
#!/usr/bin/env bash
DS="$1" DIR="$2" STOP="$3" FLAG="$4"
B=$'\e[1m' C=$'\e[36m' Y=$'\e[33m' G=$'\e[32m' X=$'\e[0m'
step() { printf '\n%s%s▶ %s%s\n' "$B" "$C" "$*" "$X"; }
cmd()  { printf '%s$ %s%s\n' "$Y" "$*" "$X"; }
cd "$DIR" || exit
P="In one sentence: what does a Unix process group let a parent do?"
step "Build under test"; cmd "dootsabha --version"; "$DS" --version
step "A council outlasts the caller's foreground limit, so it goes to the background"
if [ "$FLAG" = yes ]; then
  cmd "dootsabha council --agents codex,grok --chair codex --json --progress-file run.ndjson \"…\" > result.json 2> err &"
  "$DS" council --agents codex,grok --chair codex --json --progress-file run.ndjson "$P" > result.json 2> err &
else
  cmd "dootsabha council --json --progress-file run.ndjson …"
  rc=0; "$DS" council --json --progress-file run.ndjson "$P" > /dev/null 2>&1 || rc=$?
  printf '%s exit %s — unknown flag%s\n' "$Y" "$rc" "$X"; touch no-flag
  cmd "dootsabha council --agents codex,grok --chair codex --json \"…\" > result.json 2> err &"
  "$DS" council --agents codex,grok --chair codex --json "$P" > result.json 2> err &
fi
pid=$!
step "While it runs, the caller can only look at what is below"
rc=0; wait "$pid" || rc=$?
step "It finished"
printf '%s $? = %s%s\n' "$G" "$rc" "$X"
[ -s run.ndjson ] && printf '%s run_finished.exit_code = %s%s\n' "$G" "$(tail -n1 run.ndjson | jq .exit_code)" "$X"
sleep 6
touch "$STOP"
EOF

# record <label> <binary> <flag:yes|no>
record() {
  local label="$1" bin="$2" flag="$3" s dir stop frames pane i=0
  s="ev712$label"
  dir="$WORK/$label"
  stop="$dir/stop"
  frames="$WORK/$label-frames"
  rm -rf "$dir" "$frames"
  mkdir -p "$dir" "$frames"
  cp "$bin" "$WORK/dootsabha-$label"
  SESSIONS+=("$s")
  pane=$(shux --format json session create "$s" -d --cwd "$dir" -- bash -l | jq -r .pane_id)
  shux pane split -s "$s" -p "$pane" -d horizontal -- bash "$WORK/observe.sh" "$dir" "$stop" >/dev/null
  shux pane set-size -s "$s" -p "$pane" --cols 132 --rows 16 >/dev/null
  shux pane wait-for -s "$s" -p "$pane" --text "$label" --timeout-ms 30000 >/dev/null
  shux pane send-keys -s "$s" -p "$pane" --text "clear; bash $WORK/drive.sh $WORK/dootsabha-$label $dir $stop $flag" >/dev/null
  shux pane send-keys -s "$s" -p "$pane" --data Cg== >/dev/null
  shux pane wait-for -s "$s" -p "$pane" --text "Build under test" --timeout-ms 30000 >/dev/null
  local start=$SECONDS
  while [[ ! -e "$stop" && $((SECONDS - start)) -lt 1500 ]]; do
    shux window snapshot -s "$s" --cols 132 --rows 36 -o "$(printf '%s/%05d.png' "$frames" "$i")" >/dev/null
    i=$((i + 1))
    sleep 1
  done
  # One frame a second, played at 6 fps: a 6x time-lapse, with a clock on screen.
  ffmpeg -v error -y -framerate 6 -i "$frames/%05d.png" -vf "pad=ceil(iw/2)*2:ceil(ih/2)*2" \
    -c:v libx264 -profile:v high -level:v 4.2 -preset slow -crf 16 -pix_fmt yuv420p \
    -movflags +faststart "$OUT/$label.mp4" # iOS will not decode above level 5.x
  cp "$(printf '%s/%05d.png' "$frames" "$((i - 1))")" "$OUT/$label-end.png"
  shux session kill "$s" >/dev/null
  echo "$label: $i frames ($((SECONDS - start))s) -> $OUT/$label.mp4"
}

record before "$BEFORE" no
record after "$AFTER" yes
echo "Read every PNG in $OUT before attaching any."
