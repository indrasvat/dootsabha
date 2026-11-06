#!/usr/bin/env bash
# L4 evidence for task 712 / GitHub issue #34: what an agent that put a REAL
# codex + grok council in the background can see while it runs, for two builds.
#
# Each run is one shux window: the caller on top, and below it an independent
# observer (pr-evidence's observe.sh) with a clock and three read-only probes —
# the bytes on stderr, the progress file, and `ps`. Those are everything a
# caller can read mid-run; the observer never asks the caller anything.
#
#   .shux/scripts/issue-34-evidence.sh <before-binary> <after-binary> [scenario]
#
#   scenario  run   (default) the council runs to the end
#             term  SIGTERM to dootsabha once both agents are in dispatch
#
# Writes .shux/out/712/<scenario>-{before,after}.mp4 and keeps every frame
# under /tmp/ev712/<scenario>-<label>-frames for stills. The run happens in /tmp
# so no home path reaches a frame.
set -euo pipefail

BEFORE="${1:?usage: issue-34-evidence.sh <before-binary> <after-binary> [run|term]}"
AFTER="${2:?usage: issue-34-evidence.sh <before-binary> <after-binary> [run|term]}"
SCENARIO="${3:-run}"
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SKILL="$HOME/.claude/skills/pr-evidence/scripts"
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

# Probe 2: the progress file, one row per event, coloured by status.
cat >"$WORK/progress-probe.sh" <<'EOF'
#!/usr/bin/env bash
f="$1/run.ndjson"
G=$'\e[32m' R=$'\e[31m' Y=$'\e[33m' D=$'\e[2m' B=$'\e[1m' X=$'\e[0m'
if [ -e "$1/no-flag" ]; then printf '%snone — this build has no --progress-file%s\n' "$R" "$X"; exit 0; fi
if [ ! -s "$f" ]; then printf '%s(not written yet)%s\n' "$D" "$X"; exit 0; fi
jq -r 'select(.event != "run_started") |
  [(.ts | sub("\\.[0-9]+Z$"; "Z") | fromdate | strflocaltime("%H:%M:%S")), .event, (.stage // ""), (.agent // ""), (.status // ""),
   (if .duration_ms then "\(.duration_ms / 1000 | floor)s" else "" end),
   (if .exit_code != null then "exit_code=\(.exit_code)" else "" end), (.signal // "")] | @tsv' "$f" |
while IFS=$'\t' read -r ts ev st ag stat dur code sig; do
  c=$X
  case "$stat" in ok) c=$G ;; error | timeout) c=$R ;; interrupted) c=$Y ;; esac
  [ "$ev" = run_finished ] && c=$B
  printf '%s%s%s  %-13s %-9s %-6s %s%-11s%s %5s  %s%s %s%s\n' "$D" "$ts" "$X" "$ev" "$st" "$ag" "$c" "$stat" "$X" "$dur" "$B" "$code" "$sig" "$X"
done
EOF

# Probe 1: bytes on stderr, once the redirect has created the file.
cat >"$WORK/stderr-probe.sh" <<'EOF'
#!/usr/bin/env bash
if [ -e "$1/err" ]; then printf '%s bytes\n' "$(wc -c < "$1/err" | tr -d ' ')"; else echo "0 bytes"; fi
EOF

# Probe 3: the agents of THIS run — the children of its dootsabha process. A
# prompt needle would miss them after dispatch: review and synthesis prompts
# do not carry it.
cat >"$WORK/ps-probe.sh" <<'EOF'
#!/usr/bin/env bash
Y=$'\e[33m' D=$'\e[2m' X=$'\e[0m' any=0
for p in $(pgrep -f "dootsabha-$1 council --agents"); do
  for c in $(pgrep -P "$p"); do
    printf '%s%s is running%s %s— which stage? how long left? ps cannot say%s\n' "$Y" "$(basename "$(ps -o comm= -p "$c")")" "$X" "$D" "$X"; any=1
  done
done
[ "$any" = 0 ] && printf '%sno agent process%s\n' "$D" "$X"
exit 0
EOF

cat >"$WORK/drive.sh" <<'EOF'
#!/usr/bin/env bash
DS="$1" DIR="$2" STOP="$3" FLAG="$4" N="$5" SCENARIO="$6"
B=$'\e[1m' C=$'\e[36m' Y=$'\e[33m' G=$'\e[32m' X=$'\e[0m'
step() { printf '\n%s%s▶ %s%s\n' "$B" "$C" "$*" "$X"; }
cmd()  { printf '%s$ %s%s\n' "$Y" "$*" "$X"; }
cd "$DIR" || exit
P="In two sentences: what does a Unix process group let a parent do? ($N)"
step "Build under test: $("$DS" --version)"
step "A council outlasts a caller's foreground limit, so the caller backgrounds it"
if [ "$FLAG" = yes ]; then
  cmd "dootsabha council --agents codex,grok --chair codex --json --progress-file run.ndjson \"…\" > result.json 2> err &"
  "$DS" council --agents codex,grok --chair codex --json --progress-file run.ndjson "$P" > result.json 2> err &
else
  cmd "dootsabha council … --progress-file run.ndjson \"…\""
  rc=0; "$DS" council --agents codex,grok --json --progress-file run.ndjson "$P" > /dev/null 2>&1 || rc=$?
  printf '%s exit %s: unknown flag%s\n' "$Y" "$rc" "$X"; touch no-flag
  cmd "dootsabha council --agents codex,grok --chair codex --json \"…\" > result.json 2> err &"
  "$DS" council --agents codex,grok --chair codex --json "$P" > result.json 2> err &
fi
pid=$!
if [ "$SCENARIO" = term ]; then
  for _ in $(seq 1 240); do [ "$(pgrep -P "$pid" | wc -l)" -ge 2 ] && break; sleep 0.25; done
  sleep 8
  step "The caller gives up: SIGTERM to dootsabha"
  cmd "kill -TERM $pid"; kill -TERM "$pid"
fi
step "The caller waits; all it can look at is below"
rc=0; wait "$pid" || rc=$?
step "Done"
printf '%s $? = %s%s\n' "$G" "$rc" "$X"
sleep 8
touch "$STOP"
EOF

# record <label> <binary> <flag:yes|no>
record() {
  local label="$1" bin="$2" flag="$3" s dir stop pane n
  n="EV712$(date +%s)$RANDOM"
  s="ev712$SCENARIO$label"
  dir="$WORK/$SCENARIO-$label"
  stop="$dir/stop"
  rm -rf "$dir" "$WORK/$SCENARIO-$label-frames"
  mkdir -p "$dir"
  cp "$bin" "$WORK/dootsabha-$label"
  SESSIONS+=("$s")
  pane=$(shux --format json session create "$s" -d --cwd "$dir" -- bash -l | jq -r .pane_id)
  shux pane split -s "$s" -p "$pane" -d horizontal -- bash "$SKILL/observe.sh" -u "$stop" -i 1 -- \
    "1. stderr::bash $WORK/stderr-probe.sh $dir" \
    "2. progress file::bash $WORK/progress-probe.sh $dir" \
    "3. ps::bash $WORK/ps-probe.sh $label" >/dev/null
  shux pane set-size -s "$s" -p "$pane" --cols 140 --rows 13 >/dev/null
  shux pane wait-for -s "$s" -p "$pane" --text "$label" --timeout-ms 30000 >/dev/null
  shux pane send-keys -s "$s" -p "$pane" --text "clear; bash $WORK/drive.sh $WORK/dootsabha-$label $dir $stop $flag $n $SCENARIO" >/dev/null
  shux pane send-keys -s "$s" -p "$pane" --data Cg== >/dev/null
  # Film from the driver's first heading on, so no frame shows the prompt.
  shux pane wait-for -s "$s" -p "$pane" --text "Build under test" --timeout-ms 30000 >/dev/null
  # record.sh reads $SHUX as the shux binary; inside a shux pane it is "1".
  SHUX=shux "$SKILL/record.sh" -s "$s" -c 140x44 -o "$OUT/$SCENARIO-$label.mp4" -u "$stop" -t 2400 -f 1 -x 8 \
    -k "$WORK/$SCENARIO-$label-frames"
  shux session kill "$s" >/dev/null
  echo "$SCENARIO-$label -> $OUT/$SCENARIO-$label.mp4"
}

record before "$BEFORE" no
record after "$AFTER" yes
echo "Read every frame and poster in $OUT before attaching any."
