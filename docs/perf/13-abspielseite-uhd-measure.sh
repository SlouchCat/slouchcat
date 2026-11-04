#!/bin/bash
# Concept 13, step 12: 2160p measurement with hlsbench on CT 190 (QSV, CPU tonemapping).
# One-off containers of the stage-2 image; the running server and its database stay untouched.
set -uo pipefail
IMG=ghcr.io/slouchcat/slouchcat:stufe2-8428d7f
OUT=/root/uhd-test/results; rm -rf "$OUT"
mkdir -p "$OUT"
RG=$(grep -oP "^RENDER_GID=\K[0-9]+" /opt/slouchcat/deploy/.env)  # the GID of the device, as the server uses it
bench() { # name, then hlsbench flags
  local name=$1; shift
  echo "== $name"
  docker run --rm --device /dev/dri:/dev/dri --group-add "$RG" -v /root/uhd-test:/uhd:ro \
    --entrypoint slouchcat "$IMG" hlsbench --hw qsv --json "$@" >"$OUT/$name.json" 2>"$OUT/$name.err"
  echo "rc=$? $(head -c 1500 "$OUT/$name.json")"
  tail -3 "$OUT/$name.err"
}
uptime
bench 1-sdr2160-1stream   --file /uhd/sdr-2160p24.mkv --mode transcode --max-height 2160 --streams 1 --seconds 45
bench 1b-scope-1stream    --file /uhd/sdr-scope24.mkv --mode transcode --max-height 2160 --streams 1 --seconds 45
bench 1c-sdr2160p60       --file /uhd/sdr-2160p60.mkv --mode transcode --max-height 2160 --streams 1 --seconds 30
bench 2-sdr2160-2streams  --file /uhd/sdr-2160p24.mkv --mode transcode --max-height 2160 --streams 2 --seconds 45
echo "== 3-mixed (2160p next to 1080p, two runs side by side)"
docker run --rm --device /dev/dri:/dev/dri --group-add "$RG" -v /root/uhd-test:/uhd:ro --entrypoint slouchcat "$IMG" \
  hlsbench --hw qsv --json --file /uhd/sdr-2160p24.mkv --mode transcode --max-height 2160 --streams 1 --seconds 45 \
  >"$OUT/3-mixed-2160.json" 2>"$OUT/3-mixed-2160.err" &
docker run --rm --device /dev/dri:/dev/dri --group-add "$RG" -v /root/uhd-test:/uhd:ro --entrypoint slouchcat "$IMG" \
  hlsbench --hw qsv --json --file /uhd/sdr-2160p24.mkv --mode transcode --max-height 1080 --streams 1 --seconds 45 \
  >"$OUT/3-mixed-1080.json" 2>"$OUT/3-mixed-1080.err"
wait
echo "2160: $(head -c 1200 "$OUT/3-mixed-2160.json")"
echo "1080: $(head -c 1200 "$OUT/3-mixed-1080.json")"
bench 4-admission-budget8 --file /uhd/sdr-2160p24.mkv --mode transcode --max-height 2160 --streams 2 --seconds 30 --budget 8
bench 5-hdr-counter       --file /uhd/hdr10-2160p24.mkv --mode transcode --max-height 2160 --streams 1 --seconds 30
uptime
ls -la "$OUT"
