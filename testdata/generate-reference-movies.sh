#!/usr/bin/env bash
# Creates empty placeholder files for the 25 reference films of reference-movies.txt in
# Plex/Jellyfin naming (one folder per film), so a scan with a real TMDB key can match them, fetch
# posters (B-3) and find trailers (B-8). No media content is involved; ffprobe fails on the empty
# files, which is expected.
#
# Usage: ./generate-reference-movies.sh [OUT_DIR]   (default: media/Reference next to this script)
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
out="${1:-$here/media/Reference}"
mkdir -p "$out"
n=0
while IFS= read -r line; do
  [[ -z "$line" || "$line" == \#* ]] && continue
  name="${line//\//-}" # a slash would make a directory
  name="${name//:/ -}"  # colons are awkward on SMB shares
  dir="$out/$name"
  mkdir -p "$dir"
  : > "$dir/$name.mkv"
  n=$((n + 1))
done < "$here/reference-movies.txt"
echo "created $n reference films under $out"
