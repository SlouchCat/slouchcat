#!/bin/sh
# Concept 14, step 8: rate control of QSV and VAAPI at 480p and 360p (O-2, O-13).
# Runs inside the Slouchcat container on CT 190:
#   ssh root@192.168.1.43 'docker exec -i slouchcat sh -s' < docs/perf/14-wiedergabestart-qsv.sh
# Encodes a 30 s clip per variant into /tmp, prints the chosen rate control, the mean and the
# highest 4 s bitrate and the SSIM against the scaled source, then deletes its files.
set -u
IN="/media/nas/TestSerien/Silo/Season 01/Silo (2023) - S01E03 - Machines [WEBDL-1080p][EAC3 Atmos 5.1][h265]-ZeroTwo.mkv"
SS=600
T=30
OUT=/tmp/k14-measure
mkdir -p "$OUT"

# variant name, height, encoder arguments
run() {
	name=$1 h=$2
	shift 2
	f="$OUT/$name.mp4"
	case "$name" in
	vaapi*) hw="-vaapi_device /dev/dri/renderD128" vf="scale=-2:$h,format=nv12,hwupload" ;;
	*) hw="-init_hw_device qsv=hw -filter_hw_device hw" vf="scale=-2:$h,format=nv12,hwupload=extra_hw_frames=16" ;;
	esac
	start=$(date +%s.%N)
	# shellcheck disable=SC2086
	ffmpeg -hide_banner -nostdin -y -loglevel verbose $hw -ss $SS -t $T -i "$IN" -an -sn \
		-vf "$vf" "$@" -g 9999 -f mp4 "$f" 2>"$OUT/$name.log"
	end=$(date +%s.%N)
	rc=$(grep -oE 'RateControlMethod: [A-Z_]+|rc_mode[^,]*|Using VBR|RC mode: [A-Z]+' "$OUT/$name.log" | head -1)
	tgt=$(grep -oE 'TargetKbps: [0-9]+; MaxKbps: [0-9]+' "$OUT/$name.log" | head -1)
	mean=$(ffprobe -v error -show_entries format=bit_rate -of csv=p=0 "$f")
	peak=$(ffprobe -v error -select_streams v:0 -show_entries packet=pts_time,size -of csv=p=0 "$f" |
		awk -F, '{b=int($1/4); s[b]+=$2} END {m=0; for (k in s) if (s[k]>m) m=s[k]; printf "%.0f", m*8/4}')
	ssim=$(ffmpeg -hide_banner -nostdin -ss $SS -t $T -i "$IN" -i "$f" \
		-lavfi "[0:v]scale=-2:$h,format=yuv420p[r];[1:v]format=yuv420p[d];[d][r]ssim" -f null - 2>&1 |
		grep -oE 'All:[0-9.]+' | tail -1)
	echo "$name | $rc | $tgt | mean $mean bit/s | peak4s $peak bit/s | SSIM $ssim | $(awk -v t=$T -v a="$start" -v b="$end" 'BEGIN {printf "%.1f", t/(b-a)}')x"
}

# before (CQP by accident) and after (capped VBR) per rung; R = 2 Mbit/s (480p), 1 Mbit/s (360p)
run qsv-480-old 480 -c:v h264_qsv -preset veryfast -global_quality 23 -look_ahead 0 -maxrate 2000000 -bufsize 4000000 -profile:v high
run qsv-480-new 480 -c:v h264_qsv -preset faster -b:v 1500000 -maxrate 2000000 -bufsize 4000000 -look_ahead 0 -profile:v high
run qsv-360-old 360 -c:v h264_qsv -preset veryfast -global_quality 23 -look_ahead 0 -maxrate 1000000 -bufsize 2000000 -profile:v high
run qsv-360-new 360 -c:v h264_qsv -preset faster -b:v 999000 -maxrate 1000000 -bufsize 2000000 -look_ahead 0 -profile:v high
run qsv-1080-new 1080 -c:v h264_qsv -preset faster -b:v 6000000 -maxrate 8000000 -bufsize 16000000 -look_ahead 0 -profile:v high
run vaapi-480-new 480 -c:v h264_vaapi -rc_mode VBR -b:v 1500000 -maxrate 2000000 -bufsize 4000000 -profile:v high
run vaapi-1080-new 1080 -c:v h264_vaapi -rc_mode VBR -b:v 6000000 -maxrate 8000000 -bufsize 16000000 -profile:v high

rm -rf "$OUT"
