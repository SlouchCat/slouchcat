#!/bin/sh
# Concept 18 (trickplay), section 3; see docs/perf/18-trickplay.md.
# Runs on the CT 190 host: each benchmark inside the container, NFS read bytes from mountstats.
set -u
nfsbytes() { awk '/192.168.1.50:\/mnt\/nerdnas\/medien/{m=1} m && /bytes:/{print $2; exit}' /proc/self/mountstats; }
run() {
  b0=$(nfsbytes)
  r=$(docker exec slouchcat sh /tmp/ct-bench.sh "$@")
  b1=$(nfsbytes)
  echo "$(basename "$1") $r nfs_mb=$(( (b1-b0)/1000000 ))"
}
F=/media/nas/Filme/"28 Days Later (2002) [imdb-tt0289043]"/"28 Days Later (2002) [imdb-tt0289043] - [German DL][Bluray-1080p][DTS 5.1][x265]-iNCEPTiON.mkv"
E=/media/nas/Serien/"3 Body Problem (2024) [tvdb-411959]"/"Season 01"/"3 Body Problem (2024) - S01E01 - Countdown [WEBDL-1080p][EAC3 5.1][x264]-ZeroTwo.mkv"
# cold NAS reads first: seek on fresh files, then the whole-file variants
run "$E" seek 1 10 60     # first 60 points, one worker: latency per point
run "$E" seek 2
run "$E" seek 4           # warm: same points as before
run "$F" seek 4           # cold
run "$F" seek 2           # warm
for f in "$E" "$F"; do
  run "$f" key 2
  run "$f" fullqsv 1
done
run "$E" full 6
