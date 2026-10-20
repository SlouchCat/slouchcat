#!/usr/bin/env bash
# P-6 (plan 8): HDR10 → SDR quality of one transcode backend.
#
#   testdata/hdr-quality.sh BACKEND [FFMPEG]      BACKEND: software, nvenc, qsv or vaapi
#
# Renders 10 frames (one per second) of "Slouchcat Test HDR10 Chart (2024)" through exactly the
# filter chain the server runs on that backend (`slouchcat hlsbench --print-chain`, which calls
# transcode.Detect and transcode.HDRChain), writes testdata/out/hdr/<backend>/frame-NN.png and
# compares them:
#   (a) SSIM against the approved reference testdata/reference/hdr/<backend>/ (the first run
#       creates it from the current frames: approve it by inspection, then commit it);
#   (b) mean CIEDE2000 of the grey wedge and the skin patches against testdata/out/hdr/software/
#       (run the software backend first).
# Prints JSON (hdr-quality.py report); pass = SSIM min >= 0.95 and both dE means <= 5.
#
# Env: FFPROBE (default: ffprobe next to FFMPEG, else from PATH), SLOUCHCAT_BIN (a built
# slouchcat binary; default `go run ./cmd/slouchcat` in server/), OUT_DIR (default testdata/out/hdr),
# MEDIA_DIR (default testdata/media).
set -euo pipefail

backend="${1:-}"
case "$backend" in
software | nvenc | qsv | vaapi) ;;
*)
  echo "usage: $0 software|nvenc|qsv|vaapi [FFMPEG]" >&2
  exit 2
  ;;
esac
FFMPEG="${2:-ffmpeg}"
FFMPEG="$(command -v "$FFMPEG")" || {
  echo "error: ffmpeg '${2:-ffmpeg}' not found" >&2
  exit 1
}
if [[ -z "${FFPROBE:-}" ]]; then
  FFPROBE="$(dirname "$FFMPEG")/ffprobe"
  [[ -x "$FFPROBE" ]] || FFPROBE="$(command -v ffprobe)"
fi

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo="$(dirname "$script_dir")"
MEDIA_DIR="${MEDIA_DIR:-$script_dir/media}"
OUT_DIR="${OUT_DIR:-$script_dir/out/hdr}"
chart="$MEDIA_DIR/Movies/Slouchcat Test HDR10 Chart (2024)/Slouchcat Test HDR10 Chart (2024).mkv"
[[ -f "$chart" ]] || {
  echo "error: $chart missing; run task testdata:media" >&2
  exit 1
}
out="$OUT_DIR/$backend"
ref="$script_dir/reference/hdr/$backend"

if [[ -n "${SLOUCHCAT_BIN:-}" ]]; then
  slouchcat=("$SLOUCHCAT_BIN")
else
  slouchcat=(go -C "$repo/server" run ./cmd/slouchcat)
fi
chain_json="$("${slouchcat[@]}" hlsbench --print-chain --file "$chart" --hw "$backend" \
  --ffmpeg "$FFMPEG" --ffprobe "$FFPROBE")"

# The chain as NUL-separated fields: backend, tonemapper, filter, then the input options.
mapfile -d '' -t fields < <(python3 -I -c '
import json, sys
c = json.load(sys.stdin)
sys.stdout.write("\0".join([c["backend"], c["tonemapper"], c["filter"]] + c["input"]) + "\0")
' <<<"$chain_json")
((${#fields[@]} >= 3)) || {
  echo "error: cannot read the chain JSON: $chain_json" >&2
  exit 1
}
got="${fields[0]}" tonemapper="${fields[1]}" filter="${fields[2]}"
input=("${fields[@]:3}")
if [[ "$got" != "$backend" ]]; then
  echo "error: transcode.Detect chose $got instead of $backend (see the log above)" >&2
  exit 1
fi
echo "backend $backend, tonemapper $tonemapper" >&2
echo "chain: ${input[*]} -vf $filter" >&2

rm -rf "$out"
mkdir -p "$out"
"$FFMPEG" -nostdin -hide_banner -loglevel error -y "${input[@]}" -i "$chart" -map 0:v:0 \
  -vf "$filter,fps=1,scale=in_color_matrix=bt709:in_range=limited:out_range=full:flags=accurate_rnd+full_chroma_int,format=rgb24" \
  -frames:v 10 -start_number 0 "$out/frame-%02d.png"

ssim=()
if compgen -G "$ref/frame-*.png" >/dev/null; then
  for f in "$out"/frame-*.png; do
    r="$ref/$(basename "$f")"
    v="$("$FFMPEG" -nostdin -hide_banner -i "$f" -i "$r" -lavfi ssim -f null - 2>&1 |
      sed -n 's/.* All:\([0-9.]*\).*/\1/p' | tail -1)"
    [[ -n "$v" ]] || {
      echo "error: no SSIM for $f against $r" >&2
      exit 1
    }
    ssim+=("$v")
  done
else
  mkdir -p "$ref"
  cp "$out"/frame-*.png "$ref/"
  echo "reference created, approve by inspection: $ref" >&2
fi

report=(python3 -I "$script_dir/hdr-quality.py" report --backend "$backend" --test "$out")
if ((${#ssim[@]} > 0)); then
  report+=(--ssim "${ssim[@]}")
fi
if [[ "$backend" != software ]]; then
  compgen -G "$OUT_DIR/software/frame-*.png" >/dev/null || {
    echo "error: no software frames in $OUT_DIR/software; run $0 software first" >&2
    exit 1
  }
  report+=(--ref "$OUT_DIR/software")
fi
"${report[@]}"
