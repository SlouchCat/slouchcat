#!/usr/bin/env bash
# Generates the two 1.5 GB download files of the offline spike (plan M1a step 1, 10.4, AN-20):
#   spike-faststart.mp4  progressive MP4 with the moov atom in front (+faststart)
#   spike-fmp4.mp4       fragmented MP4 with a global sidx index (seekable without the moov table)
# Both hold the same H.264/AAC stream (1080p30 testsrc2, CBR 4 Mbit/s, keyframe every 2 s), encoded
# once and remuxed. Output: testdata/media/spike/ (gitignored). No copyrighted material.
#
# Env: FFMPEG (binary), OUT_DIR (default testdata/media/spike), MINUTES (default 52 → about 1.56 GB),
#      FORCE=1 regenerates existing files.
set -euo pipefail

FFMPEG="${FFMPEG:-ffmpeg}"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT_DIR="${OUT_DIR:-$script_dir/media/spike}"
MINUTES="${MINUTES:-52}"
FORCE="${FORCE:-0}"

command -v "$FFMPEG" >/dev/null 2>&1 || { echo "error: '$FFMPEG' not found" >&2; exit 1; }
mkdir -p "$OUT_DIR"

faststart="$OUT_DIR/spike-faststart.mp4"
fmp4="$OUT_DIR/spike-fmp4.mp4"
seconds=$((MINUTES * 60))

if [[ ! -f "$faststart" || "$FORCE" == "1" ]]; then
  echo "encode: ${faststart#"$OUT_DIR"/} (${MINUTES} min, CBR 4 Mbit/s)"
  "$FFMPEG" -nostdin -hide_banner -loglevel error -stats -y \
    -f lavfi -i "testsrc2=size=1920x1080:rate=30:duration=$seconds" \
    -f lavfi -i "sine=frequency=440:sample_rate=48000:duration=$seconds" \
    -c:v libx264 -preset ultrafast -tune zerolatency -g 60 -keyint_min 60 -sc_threshold 0 \
    -b:v 4M -minrate 4M -maxrate 4M -bufsize 8M -x264-params nal-hrd=cbr:force-cfr=1 \
    -pix_fmt yuv420p -c:a aac -b:a 128k -ac 2 \
    -movflags +faststart -metadata title="Slouchcat Offline Spike" "$faststart"
else
  echo "skip (exists): ${faststart#"$OUT_DIR"/}"
fi

if [[ ! -f "$fmp4" || "$FORCE" == "1" ]]; then
  echo "remux: ${fmp4#"$OUT_DIR"/} (fragmented MP4 with global sidx)"
  "$FFMPEG" -nostdin -hide_banner -loglevel error -y -i "$faststart" -c copy \
    -movflags frag_keyframe+empty_moov+default_base_moof+global_sidx -frag_duration 4000000 \
    "$fmp4"
else
  echo "skip (exists): ${fmp4#"$OUT_DIR"/}"
fi

echo
ls -l --block-size=M "$faststart" "$fmp4"
for f in "$faststart" "$fmp4"; do
  size=$(stat -c %s "$f")
  if ((size < 1500000000)); then
    echo "warning: ${f#"$OUT_DIR"/} is below 1.5 GB (${size} bytes); raise MINUTES" >&2
  fi
done
