#!/bin/sh
# Concept 18 (trickplay), section 3; see docs/perf/18-trickplay.md.
# Runs on the CT 190 host. 1) HDR: per-point cost with the CPU tonemap of GrabFrameArgs.
# 2) Playback next to the generation: a QSV transcode (HEVC 1080p -> H.264 8 Mbit/s) alone and
#    while seek extraction with 2 workers runs on another file; reports the transcode speed.
set -u
H=/media/calib/dutton-4k-hdr10plus-120s.mkv
TM="zscale=w=320:h=trunc(320/dar/2)*2,setsar=1,zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709,tonemap=tonemap=hable:desat=0,zscale=t=bt709:m=bt709:r=tv,format=yuv420p"
docker exec slouchcat sh -c '
  t0=$(date +%s.%N)
  for t in 5 15 25 35 45 55 65 75 85 95 105; do
    ffmpeg -nostdin -hide_banner -benchmark -loglevel info -threads 1 -noaccurate_seek -ss $t -i "'"$H"'" -map 0:v:0 -frames:v 1 -fps_mode passthrough -vf "'"$TM"'" -q:v 4 -y /tmp/tp/hdr-$t.jpg 2>&1 | grep -o "utime=[0-9.]*s"
  done | sed "s/utime=//;s/s//" | awk -v a=$t0 -v b=$(date +%s.%N) "{s+=\$1;n++} END{printf \"hdr points=%d cpu_per_point=%.2f\n\", n, s/n}"
  t1=$(date +%s.%N); echo "hdr wall_per_point=$(awk -v a=$t0 -v b=$t1 "BEGIN{printf \"%.2f\", (b-a)/11}")"
'
F=/media/nas/Filme/"21 Jump Street (2012) [imdb-tt1232829]"/"21 Jump Street (2012) [imdb-tt1232829] - [Bluray-1080p][AAC 2.0][x265]-2BA.mkv"
G=/media/nas/Filme/"2012 (2009) [imdb-tt1190080]"/"2012 (2009) [imdb-tt1190080] - [Bluray-1080p][AAC 2.0][x265]-2BA.mkv"
tc() {
  docker exec slouchcat sh -c "ffmpeg -nostdin -hide_banner -loglevel info -stats -init_hw_device qsv=hw:/dev/dri/renderD128 -hwaccel qsv -hwaccel_output_format qsv -c:v hevc_qsv -ss 1200 -t 120 -i \"$F\" -map 0:v:0 -c:v h264_qsv -b:v 6M -maxrate 8M -bufsize 16M -f null - 2>&1" | tr '\r' '\n' | grep -o "speed=[ 0-9.]*x" | tail -1
}
echo "transcode alone: $(tc)"
docker exec slouchcat sh /tmp/ct-bench.sh "$G" seek 2 > /tmp/ct-extra-seek.txt &
sleep 2
echo "transcode with seek x2: $(tc)"
wait
echo "seek x2 next to the transcode: $(cat /tmp/ct-extra-seek.txt)"
