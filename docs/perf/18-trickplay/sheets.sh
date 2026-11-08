#!/bin/bash
# Sheet size test: grab every 10 s point of FILE as PNG (width W), then build 10x10 sheets in
# JPEG and WebP at several qualities and report the total size per hour of video.
# usage: sheets.sh FILE W   (frames and sheets go to $OUT, default /tmp/trickplay-sheets; never into the repository)
set -eu
f=$1; w=$2
d=${OUT:-/tmp/trickplay-sheets}/sheets-$(basename "$f" .mkv)-$w; rm -rf "$d"; mkdir -p "$d/png" "$d/s"
dur=$(ffprobe -v error -show_entries format=duration -of csv=p=0 "$f")
n=$(awk -v d="$dur" 'BEGIN{print int(d/10)}')
seq 0 $((n-1)) | xargs -P 8 -I{} sh -c 'ffmpeg -nostdin -loglevel error -threads 1 -noaccurate_seek -ss $(({}*10)) -i "$0" -map 0:v:0 -frames:v 1 -fps_mode passthrough -vf scale='"$w"':-2:flags=lanczos -y '"$d"'/png/$(printf %05d {}).png' "$f"
sheets=$(( (n+99)/100 ))
hours=$(awk -v d="$dur" 'BEGIN{print d/3600}')
for enc in "jpg -q:v 3" "jpg -q:v 5" "jpg -q:v 8" "webp -quality 60" "webp -quality 75" "webp -quality 85"; do
  ext=${enc%% *}; opt=${enc#* }
  rm -f "$d"/s/*
  for s in $(seq 0 $((sheets-1))); do
    ffmpeg -nostdin -loglevel error -start_number $((s*100)) -i "$d/png/%05d.png" -frames:v 1 -vf "tile=10x10" $opt -y "$d/s/$s.$ext" 2>/dev/null || true
  done
  kb=$(du -bc "$d"/s/* | tail -1 | cut -f1)
  echo "$(basename "$f") w=$w points=$n sheets=$sheets $enc total_kb=$((kb/1000)) per_hour_mb=$(awk -v b=$kb -v h=$hours 'BEGIN{printf "%.2f", b/1e6/h}') sheet_kb=$((kb/1000/sheets))"
done

