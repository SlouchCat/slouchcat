#!/bin/sh
# Concept 18 (trickplay), section 3; see docs/perf/18-trickplay.md.
# Trickplay benchmark inside the slouchcat container on CT 190.
# usage: ct-bench.sh FILE VARIANT PAR [INTERVAL] [LIMIT]
# prints one line: variant par points images wall_s cpu_s (cpu = sum of ffmpeg utime+stime)
# variants: seek (input seek, keyframe, one ffmpeg per point), key (whole file, keyframes only),
#           fullqsv (whole file, QSV decode, all frames), full (whole file, CPU decode, all frames)
set -u
f=$1; v=$2; par=$3; iv=${4:-10}; limit=${5:-0}
w=/tmp/tp/$v-$par; rm -rf "$w"; mkdir -p "$w/img" "$w/log"
dur=$(ffprobe -v error -show_entries format=duration -of csv=p=0 "$f")
n=$(awk -v d="$dur" -v i="$iv" 'BEGIN{print int(d/i)}')
[ "$limit" -gt 0 ] && [ "$limit" -lt "$n" ] && n=$limit
t0=$(date +%s.%N)
case $v in
seek)
  i=0; while [ $i -lt $n ]; do echo $i; i=$((i+1)); done |
    xargs -P "$par" -I{} sh -c 'ffmpeg -nostdin -hide_banner -benchmark -loglevel info -threads 1 -noaccurate_seek -ss $(({}*'"$iv"')) -i "$0" -map 0:v:0 -frames:v 1 -fps_mode passthrough -vf scale=320:-2 -q:v 4 -y '"$w"'/img/$(printf %05d {}).jpg > '"$w"'/log/{}.log 2>&1' "$f"
  ;;
key)
  ffmpeg -nostdin -hide_banner -benchmark -loglevel info -threads "$par" -skip_frame nokey -i "$f" -map 0:v:0 \
    -vf "fps=1/$iv:round=down,scale=320:-2" -q:v 4 "$w/img/%05d.jpg" > "$w/log/0.log" 2>&1
  ;;
fullqsv)
  ffmpeg -nostdin -hide_banner -benchmark -loglevel info -init_hw_device vaapi=va:/dev/dri/renderD128 \
    -hwaccel vaapi -hwaccel_device va -hwaccel_output_format vaapi -i "$f" -map 0:v:0 \
    -vf "fps=1/$iv:round=down,scale_vaapi=w=320:h=-2,hwdownload,format=nv12" -q:v 4 "$w/img/%05d.jpg" > "$w/log/0.log" 2>&1
  ;;
full)
  ffmpeg -nostdin -hide_banner -benchmark -loglevel info -threads "$par" -i "$f" -map 0:v:0 \
    -vf "fps=1/$iv:round=down,scale=320:-2" -q:v 4 "$w/img/%05d.jpg" > "$w/log/0.log" 2>&1
  ;;
esac
t1=$(date +%s.%N)
imgs=$(ls "$w/img" | wc -l)
kb=$(du -sk "$w/img" | cut -f1)
cpu=$(cat "$w"/log/*.log | sed -n 's/.*utime=\([0-9.]*\)s stime=\([0-9.]*\)s.*/\1 \2/p' | awk '{s+=$1+$2} END{printf "%.1f", s}')
errs=$(cat "$w"/log/*.log | grep -ci "error")
awk -v v="$v" -v p="$par" -v n="$n" -v im="$imgs" -v a="$t0" -v b="$t1" -v c="$cpu" -v kb="$kb" -v e="$errs" \
  'BEGIN{printf "variant=%s par=%s points=%s images=%s wall_s=%.1f cpu_s=%s jpg_kb=%s errlines=%s\n", v, p, n, im, b-a, c, kb, e}'
