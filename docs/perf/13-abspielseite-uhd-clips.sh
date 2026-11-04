#!/bin/bash
# Concept 13, step 12: synthetic 4K clips for hlsbench on CT 190 (test data only, no library).
# SDR with libx264 (libx265 in 4K exceeds the 4 GB of the CT, OOM rc=137; hevc_qsv refuses the
# encode). HDR10 with libx265 and small buffers, 10 bit, BT.2020/PQ.
set -uo pipefail
OUT=/root/uhd-test
mkdir -p "$OUT"
IMG=ghcr.io/slouchcat/slouchcat:1.10.0
run() {
  echo ">> $(date +%T) ${*: -1}"
  docker run --rm --user 0:0 --entrypoint ffmpeg -v "$OUT:/out" "$IMG" -hide_banner -loglevel warning -y "$@"
  echo "<< rc=$?"
}
# Moving test pattern plus temporal noise, so the encoder has real work to do.
src() { printf 'testsrc2=size=%s:rate=%s,noise=alls=12:allf=t+u' "$1" "$2"; }
AUD="sine=frequency=440:sample_rate=48000"
SDR=(-c:v libx264 -preset ultrafast -crf 20 -pix_fmt yuv420p -threads 4 -color_primaries bt709 -color_trc bt709 -colorspace bt709)
HDR=(-c:v libx265 -preset ultrafast -crf 22 -pix_fmt yuv420p10le
  -x265-params "log-level=error:frame-threads=1:rc-lookahead=5:bframes=2:colorprim=bt2020:transfer=smpte2084:colormatrix=bt2020nc:hdr10=1"
  -color_primaries bt2020 -color_trc smpte2084 -colorspace bt2020nc)
AAC=(-c:a aac -b:a 192k -shortest)
for f in "$OUT"/*.mkv; do [ -s "$f" ] || rm -f "$f"; done
[ -f "$OUT/sdr-2160p24.mkv" ] || run -f lavfi -i "$(src 3840x2160 24)" -f lavfi -i "$AUD" -t 90 "${SDR[@]}" "${AAC[@]}" /out/sdr-2160p24.mkv
[ -f "$OUT/sdr-scope24.mkv" ] || run -f lavfi -i "$(src 3840x1600 24)" -f lavfi -i "$AUD" -t 90 "${SDR[@]}" "${AAC[@]}" /out/sdr-scope24.mkv
[ -f "$OUT/sdr-2160p60.mkv" ] || run -f lavfi -i "$(src 3840x2160 60)" -f lavfi -i "$AUD" -t 60 "${SDR[@]}" "${AAC[@]}" /out/sdr-2160p60.mkv
# HDR10 from the finished SDR clip: the 10-bit test pattern goes through a 48-bit conversion that
# alone exceeds the memory. Only the signalling matters for the measurement (HDR → tonemapping).
[ -f "$OUT/hdr10-2160p24.mkv" ] || run -i /out/sdr-2160p24.mkv -t 60 -map 0 "${HDR[@]}" -c:a copy /out/hdr10-2160p24.mkv
ls -la "$OUT"
for f in sdr-2160p24 sdr-scope24 sdr-2160p60 hdr10-2160p24; do
  docker run --rm --user 0:0 --entrypoint ffprobe -v "$OUT:/out" "$IMG" -v error -count_packets -select_streams v:0 \
    -show_entries stream=codec_name,width,height,r_frame_rate,pix_fmt,color_transfer,nb_read_packets -of csv=p=0 "/out/$f.mkv"
done
