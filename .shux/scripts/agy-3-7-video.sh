#!/usr/bin/env bash
# L4 motion evidence for task 707 — a real Gemini 3.7 Flash council, as video.
#
# shux has no video primitive by design: it rasterises single frames. So we
# snapshot the live pane on a fixed cadence while a REAL council runs, then let
# ffmpeg stitch the frames. What you see is genuinely 3.7 Flash working, not a
# replayed transcript.
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT="$REPO/.shux/out/707"
FRAMES="$OUT/video-frames"
BIN="bin/dootsabha"

export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR_OVERRIDE:-/tmp/shux707v}"
mkdir -p "$XDG_RUNTIME_DIR" "$FRAMES"
rm -f "$FRAMES"/*.png

SESS="f707-video"
cleanup() {
  shux session kill "$SESS" >/dev/null 2>&1 || true
  shux daemon stop >/dev/null 2>&1 || true
}
trap cleanup EXIT

PROMPT='In exactly two sentences: why is a per-turn status field a poor failure discriminator for a CLI wrapper, when the process exit code disagrees with it?'

echo "Recording a live council (codex, agy, grok — chair agy) …"
shux session create "$SESS" -d --title council --cwd "$REPO" \
  -- bash -lc "cd '$REPO' && clear && $BIN council --agents codex,agy,grok --chair agy '$PROMPT'; echo; echo __FRAME_DONE__; sleep 600" \
  >/dev/null
shux pane set-size -s "$SESS" --cols 120 --rows 34 >/dev/null

i=0
max=400   # 400 * 1.5s ≈ 10 min ceiling
while [[ "$i" -lt "$max" ]]; do
  shux pane snapshot -s "$SESS" -o "$(printf '%s/f%04d.png' "$FRAMES" "$i")" >/dev/null 2>&1 || true
  if shux pane capture -s "$SESS" --lines 600 2>/dev/null | grep -q '__FRAME_DONE__'; then
    # a few tail frames so the final state is readable in playback
    for _ in 1 2 3 4 5 6 7 8; do
      i=$((i + 1))
      shux pane snapshot -s "$SESS" -o "$(printf '%s/f%04d.png' "$FRAMES" "$i")" >/dev/null 2>&1 || true
    done
    break
  fi
  sleep 1.5
  i=$((i + 1))
  [[ $((i % 20)) -eq 0 ]] && printf '  … %ss elapsed\n' "$((i * 3 / 2))"
done

COUNT=$(find "$FRAMES" -name '*.png' | wc -l | tr -d ' ')
echo "  captured $COUNT frames"

ffmpeg -y -loglevel error -framerate 8 -pattern_type glob -i "$FRAMES/*.png" \
  -vf "pad=ceil(iw/2)*2:ceil(ih/2)*2" -c:v libx264 -pix_fmt yuv420p -crf 18 \
  "$OUT/agy-3-7-council.mp4"

ffmpeg -y -loglevel error -framerate 8 -pattern_type glob -i "$FRAMES/*.png" \
  -vf "fps=8,scale=1000:-1:flags=lanczos,split[a][b];[a]palettegen[p];[b][p]paletteuse" \
  "$OUT/agy-3-7-council.gif"

du -h "$OUT/agy-3-7-council.mp4" "$OUT/agy-3-7-council.gif"
