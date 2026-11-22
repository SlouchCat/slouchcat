#!/usr/bin/env bash
# Real, small videos for the player of the Android apps (K27 stage 5): the contract test
# (task android:contract) and the flow in the emulator (scripts/android-e2e.sh) play them. Made
# with ffmpeg from its test sources (no film pictures), a few seconds of work.
#
#   scripts/android-test-video.sh DIR     # writes DIR/Playback/Movies and DIR/Playback/TV
#
# - «Two Voices» (MKV, 60 s): H.264 1280×720, two AAC tracks (deu, eng), two SRT tracks (deu, eng)
#   and chapters: plays directly for the app's profile, as an HLS remux without `mkv`.
# - «Clip Show» S01E01 and S01E02 (MKV, 70 s each): chapters «Intro» (5–22 s) and «Credits»
#   (45–70 s), from which the server takes the intro and the end credits.
# - Sound (K27 stage 7), in DIR/Playback/Audiobooks and DIR/Playback/Podcasts: «The Quiet Harbour»
#   by «Mira Lindqvist», an audiobook of two AAC files of 60 s (one HLS timeline of 120 s, one
#   chapter per file), and the podcast folder «Night Shift Stories» (metadata.json without feed)
#   with two episodes of 40 s.
# FFMPEG=... names another binary.
set -euo pipefail
dir=${1:?usage: android-test-video.sh DIR}
[ -d "$dir" ] || { echo "android-test-video: $dir is no directory" >&2; exit 1; }
ff=${FFMPEG:-ffmpeg}
command -v "$ff" >/dev/null || { echo "android-test-video: no ffmpeg (FFMPEG=$ff); the player tests need real video" >&2; exit 1; }
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

srt() { # file language-word
  {
    for i in 1 2 3 4 5 6; do
      s=$(((i - 1) * 10 + 1))
      printf '%d\n00:00:%02d,000 --> 00:00:%02d,000\n%s %d\n\n' "$i" "$s" "$((s + 4))" "$2" "$i"
    done
  } >"$1"
}

chapters() { # file  start:end:title ...
  out=$1
  shift
  printf ';FFMETADATA1\n' >"$out"
  for c in "$@"; do
    IFS=: read -r start end title <<<"$c"
    printf '[CHAPTER]\nTIMEBASE=1/1000\nSTART=%d\nEND=%d\ntitle=%s\n' "$((start * 1000))" "$((end * 1000))" "$title" >>"$out"
  done
}

video() { # seconds out [extra args...]
  secs=$1
  out=$2
  shift 2
  "$ff" -nostdin -loglevel error -y \
    -f lavfi -i "testsrc2=size=640x360:rate=25:duration=$secs" \
    -f lavfi -i "sine=frequency=440:sample_rate=48000:duration=$secs" \
    "$@" "$out"
}

film="$dir/Playback/Movies/Two Voices (2024)"
mkdir -p "$film"
srt "$work/de.srt" "Untertitel"
srt "$work/en.srt" "Subtitle"
chapters "$work/film.txt" "0:20:Anfang" "20:40:Mitte" "40:60:Ende"
"$ff" -nostdin -loglevel error -y \
  -f lavfi -i "testsrc2=size=1280x720:rate=25:duration=60" \
  -f lavfi -i "sine=frequency=440:sample_rate=48000:duration=60" \
  -f lavfi -i "sine=frequency=880:sample_rate=48000:duration=60" \
  -i "$work/de.srt" -i "$work/en.srt" -i "$work/film.txt" \
  -map 0:v -map 1:a -map 2:a -map 3:s -map 4:s -map_metadata 5 -map_chapters 5 \
  -c:v libx264 -preset ultrafast -pix_fmt yuv420p -g 50 -c:a aac -b:a 128k -c:s srt \
  -metadata:s:a:0 language=deu -metadata:s:a:0 title=Deutsch \
  -metadata:s:a:1 language=eng -metadata:s:a:1 title=English \
  -metadata:s:s:0 language=deu -metadata:s:s:0 title=Deutsch \
  -metadata:s:s:1 language=eng -metadata:s:s:1 title=English \
  -disposition:a:0 default -disposition:a:1 0 -disposition:s:0 0 -disposition:s:1 0 \
  -metadata title="Two Voices" "$film/Two Voices (2024).mkv"

show="$dir/Playback/TV/Clip Show (2024)/Season 01"
mkdir -p "$show"
chapters "$work/episode.txt" "0:5:Prolog" "5:22:Intro" "22:45:Story" "45:70:Credits"
for ep in 1 2; do
  video 70 "$show/Clip Show S01E0$ep.mkv" -i "$work/episode.txt" -map 0:v -map 1:a -map_chapters 2 \
    -c:v libx264 -preset ultrafast -pix_fmt yuv420p -g 50 -c:a aac -b:a 128k \
    -metadata title="Episode $ep" -metadata:s:a:0 language=deu
done

# Sound (stage 7): a tone of a few seconds per file, tagged as the scanner reads it.
tone() { # seconds frequency out [metadata args...]
  secs=$1
  freq=$2
  out=$3
  shift 3
  "$ff" -nostdin -loglevel error -y -f lavfi -i "sine=frequency=$freq:sample_rate=44100:duration=$secs" \
    -c:a aac -b:a 64k -ac 1 "$@" "$out"
}

book="$dir/Playback/Audiobooks/Mira Lindqvist/The Quiet Harbour"
mkdir -p "$book"
for part in 1 2; do
  tone 60 $((300 + 100 * part)) "$book/0$part - Teil $part.m4a" \
    -metadata title="Teil $part" -metadata album="The Quiet Harbour" -metadata artist="Mira Lindqvist" \
    -metadata composer="Jonas Brenner" -metadata track="$part/2"
done

podcast="$dir/Playback/Podcasts/Night Shift Stories"
mkdir -p "$podcast"
printf '%s\n' '{' '  "title": "Night Shift Stories",' '  "author": "Studio North",' \
  '  "description": "Ein Testpodcast aus einem Ordner.",' '  "genres": ["Podcasts"],' '  "language": "de",' \
  '  "explicit": false' '}' >"$podcast/metadata.json"
dates=("" "Mon, 03 Nov 2025 06:00:00 +0000" "Mon, 10 Nov 2025 06:00:00 +0000")
for ep in 1 2; do
  tone 40 $((700 + 100 * ep)) "$podcast/Folge $ep.m4a" \
    -metadata title="Folge $ep" -metadata album="Night Shift Stories" -metadata artist="Studio North" \
    -metadata date="${dates[$ep]}" -metadata comment="Testfolge $ep."
done
