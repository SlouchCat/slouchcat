#!/usr/bin/env bash
# Generates the audio of the radio test station (concept 16, build plan S4) with the ffmpeg lavfi
# source sine only: no music, no copyrighted material. The files are small and committed; run this
# script again only to change them.
#
# Env: FFMPEG (binary, default from PATH).
set -euo pipefail

FFMPEG="${FFMPEG:-ffmpeg}"
dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ff=("$FFMPEG" -nostdin -hide_banner -loglevel error -y)
tone() { printf '%s\n' -f lavfi -i "sine=frequency=$1:sample_rate=44100:duration=$2"; }

mapfile -t t440 < <(tone 440 16)
"${ff[@]}" "${t440[@]}" -ac 2 -c:a libmp3lame -b:a 128k -f mp3 "$dir/tone-mp3-128.mp3"
"${ff[@]}" "${t440[@]}" -ac 2 -c:a aac -b:a 64k -f adts "$dir/tone-aac-64.aac"
"${ff[@]}" "${t440[@]}" -ac 2 -c:a libvorbis -q:a 0 -metadata TITLE="Test Tone" -metadata ARTIST="Slouchcat" \
  -f ogg "$dir/tone-ogg.ogg"
mapfile -t t440s < <(tone 440 8)
"${ff[@]}" "${t440s[@]}" -ac 2 -c:a libopus -b:a 48k -metadata TITLE="Opus Tone" -metadata ARTIST="Slouchcat" \
  -f ogg "$dir/tone-opus.ogg"

# Two chained Ogg Vorbis streams (how Icecast Ogg stations change the title): two complete files
# with their own serial number and comments, one after the other.
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mapfile -t t660 < <(tone 660 6)
"${ff[@]}" "${t440s[@]}" -t 6 -ac 2 -c:a libvorbis -q:a 0 -serial_offset 1 -metadata TITLE="First Tone" \
  -metadata ARTIST="Slouchcat" -f ogg "$tmp/a.ogg"
"${ff[@]}" "${t660[@]}" -ac 2 -c:a libvorbis -q:a 0 -serial_offset 2 -metadata TITLE="Second Tone" \
  -metadata ARTIST="Slouchcat" -f ogg "$tmp/b.ogg"
cat "$tmp/a.ogg" "$tmp/b.ogg" >"$dir/tone-ogg-chained.ogg"

# HLS: 6 segments of 4 s MPEG-TS with AAC 64 kbit/s.
mkdir -p "$dir/hls"
rm -f "$dir/hls/"*.ts "$dir/hls/"*.m3u8
mapfile -t t24 < <(tone 523 24)
"${ff[@]}" "${t24[@]}" -ac 2 -c:a aac -b:a 64k -f hls -hls_time 4 -hls_list_size 0 -hls_segment_type mpegts \
  -hls_segment_filename "$dir/hls/seg%d.ts" "$dir/hls/media.m3u8"
rm -f "$dir/hls/media.m3u8" "$dir/hls/seg6.ts" # the AAC priming leaves a few ms for a seventh segment
