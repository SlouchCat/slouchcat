#!/usr/bin/env bash
# Generates synthetic test media (plan M0 step 6, test matrix 10.3) with ffmpeg lavfi
# sources only (testsrc2, smptehdbars, sine), PGS captions from make-pgs.py and the HDR chart
# from hdr-quality.py: no copyrighted material.
# Output: testdata/media/ (gitignored) in Plex/Jellyfin naming.
#
# Env: FFMPEG, FFPROBE (binaries, default from PATH), OUT_DIR (default testdata/media),
#      FORCE=1 regenerates existing files, LONG=1 adds the 120-s 4K HDR10 clip (slow, ~260 MB).
set -euo pipefail

FFMPEG="${FFMPEG:-ffmpeg}"
FFPROBE="${FFPROBE:-ffprobe}"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT_DIR="${OUT_DIR:-$script_dir/media}"
FORCE="${FORCE:-0}"

die() {
  echo "error: $*" >&2
  exit 1
}

for bin in "$FFMPEG" "$FFPROBE"; do
  command -v "$bin" >/dev/null 2>&1 \
    || die "'$bin' not found. Install ffmpeg or set FFMPEG/FFPROBE, e.g. FFMPEG=/usr/lib/jellyfin-ffmpeg/ffmpeg"
done

encoders="$("$FFMPEG" -hide_banner -encoders 2>/dev/null)"
for enc in libx264 libx265 aac ac3 dca srt ass libmp3lame mjpeg; do
  grep -Eq "^ [A-Z.]{6} ${enc} " <<<"$encoders" || die "ffmpeg encoder '$enc' is missing in $FFMPEG"
done

ff=("$FFMPEG" -nostdin -hide_banner -loglevel error -y)
# veryfast instead of ultrafast: ultrafast yields Constrained Baseline, real files are High profile.
x264=(-c:v libx264 -preset veryfast -crf 30 -g 48 -pix_fmt yuv420p
  -color_primaries bt709 -color_trc bt709 -colorspace bt709)
aac=(-c:a aac -b:a 128k -ac 2)

# Lavfi inputs: 1080p24 test pattern and a 48 kHz sine tone of the given duration.
video_1080() { printf '%s\n' -f lavfi -i "testsrc2=size=1920x1080:rate=24:duration=$1"; }
tone() { printf '%s\n' -f lavfi -i "sine=frequency=$2:sample_rate=48000:duration=$1"; }

generated=()

# want <file>: returns 0 when the file must be (re)generated and creates its folder.
want() {
  generated+=("$1")
  if [[ -f "$1" && "$FORCE" != "1" ]]; then
    echo "skip (exists): ${1#"$OUT_DIR"/}"
    return 1
  fi
  mkdir -p "$(dirname "$1")"
  echo "generate: ${1#"$OUT_DIR"/}"
}

movie_path() { printf '%s/Movies/%s/%s.%s' "$OUT_DIR" "$1" "$1" "$2"; }

mapfile -t v10 < <(video_1080 10)
mapfile -t a10 < <(tone 10 440)

# (a) H.264/AAC MP4 1080p (Direct Play everywhere).
f="$(movie_path "Slouchcat Test H264 (2024)" mp4)"
if want "$f"; then
  "${ff[@]}" "${v10[@]}" "${a10[@]}" "${x264[@]}" "${aac[@]}" -movflags +faststart \
    -metadata title="Slouchcat Test H264" "$f"
fi

# (b) HEVC 10-bit SDR MKV 1080p.
f="$(movie_path "Slouchcat Test HEVC 10bit (2024) {tmdb-7003}" mkv)"
if want "$f"; then
  "${ff[@]}" "${v10[@]}" "${a10[@]}" \
    -c:v libx265 -preset ultrafast -crf 32 -g 48 -pix_fmt yuv420p10le \
    -color_primaries bt709 -color_trc bt709 -colorspace bt709 \
    -x265-params log-level=error \
    "${aac[@]}" -metadata title="Slouchcat Test HEVC 10bit" "$f"
fi

# (c) HDR10 HEVC 4K, 30 s. The pixels are SDR test bars; the stream carries HDR10
# signalling (BT.2020, PQ, mastering display, MaxCLL/MaxFALL) for detection and tonemapping tests.
f="$(movie_path "Slouchcat Test HDR10 4K (2024)" mkv)"
if want "$f"; then
  "${ff[@]}" -f lavfi -i "smptehdbars=size=3840x2160:rate=24:duration=30" \
    -f lavfi -i "sine=frequency=440:sample_rate=48000:duration=30" \
    -vf "format=yuv420p10le" \
    -c:v libx265 -preset ultrafast -crf 34 -g 48 -pix_fmt yuv420p10le \
    -color_primaries bt2020 -color_trc smpte2084 -colorspace bt2020nc \
    -x265-params "log-level=error:hdr10=1:repeat-headers=1:colorprim=bt2020:transfer=smpte2084:colormatrix=bt2020nc:master-display=G(13250,34500)B(7500,3000)R(34000,16000)WP(15635,16450)L(10000000,1):max-cll=1000,400" \
    "${aac[@]}" -metadata title="Slouchcat Test HDR10 4K" "$f"
fi

# Six-channel tone (mono sine on every channel) for the surround formats.
surround=(-af "pan=5.1(side)|FL=c0|FR=c0|FC=c0|LFE=c0|SL=c0|SR=c0")

# (d) H.264 + AC3 5.1 MKV.
f="$(movie_path "Slouchcat Test AC3 51 (2024) {tmdb-7001}" mkv)"
if want "$f"; then
  "${ff[@]}" "${v10[@]}" "${a10[@]}" "${x264[@]}" "${surround[@]}" \
    -c:a ac3 -b:a 384k -metadata title="Slouchcat Test AC3 51" "$f"
fi

# (e) H.264 + DTS 5.1 MKV (ffmpeg's dca encoder is experimental: -strict -2).
f="$(movie_path "Slouchcat Test DTS (2024) {tmdb-7002}" mkv)"
if want "$f"; then
  "${ff[@]}" "${v10[@]}" "${a10[@]}" "${x264[@]}" "${surround[@]}" \
    -c:a dca -strict -2 -b:a 768k -metadata title="Slouchcat Test DTS" "$f"
fi

# (f) H.264 MKV with embedded SRT and ASS tracks (de/en), plus a copy without
# embedded subtitles that has the same subtitles as sidecar files.
subs_dir="$(mktemp -d)"
trap 'rm -rf "$subs_dir"' EXIT

write_srt() { # <file> <line1> <line2> <line3>
  cat >"$1" <<SRT
1
00:00:01,000 --> 00:00:03,000
$2

2
00:00:04,000 --> 00:00:06,000
$3

3
00:00:07,000 --> 00:00:09,500
$4
SRT
}

write_ass() { # <file> <title> <line1> <line2> <line3>
  cat >"$1" <<ASS
[Script Info]
Title: $2
ScriptType: v4.00+
PlayResX: 1920
PlayResY: 1080
WrapStyle: 0
ScaledBorderAndShadow: yes

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Default,Arial,64,&H00FFFFFF,&H000000FF,&H00000000,&H80000000,0,0,0,0,100,100,0,0,1,3,1,2,40,40,60,1
Style: Top,Arial,56,&H0000FFFF,&H000000FF,&H00000000,&H80000000,1,0,0,0,100,100,0,0,1,3,1,8,40,40,60,1

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:01.00,0:00:03.00,Default,,0,0,0,,$3
Dialogue: 0,0:00:04.00,0:00:06.00,Top,,0,0,0,,{\\i1}$4{\\i0}
Dialogue: 0,0:00:07.00,0:00:09.50,Default,,0,0,0,,{\\an5\\fad(200,200)}$5
ASS
}

write_srt "$subs_dir/de.srt" "Erster Untertitel (SRT, Deutsch)" "Zweiter Untertitel mit Umlauten: äöü" "Dritter Untertitel"
write_srt "$subs_dir/en.srt" "First subtitle (SRT, English)" "Second subtitle" "Third subtitle"
write_ass "$subs_dir/de.ass" "Slouchcat Test ASS de" "Erster Untertitel (ASS, Deutsch)" "Oben und kursiv" "Mitte mit Einblendung"
write_ass "$subs_dir/en.ass" "Slouchcat Test ASS en" "First subtitle (ASS, English)" "Top and italic" "Centre with fade"

f="$(movie_path "Slouchcat Test Subtitles (2024)" mkv)"
if want "$f"; then
  "${ff[@]}" "${v10[@]}" "${a10[@]}" \
    -i "$subs_dir/de.srt" -i "$subs_dir/en.srt" -i "$subs_dir/de.ass" -i "$subs_dir/en.ass" \
    -map 0:v -map 1:a -map 2 -map 3 -map 4 -map 5 \
    "${x264[@]}" "${aac[@]}" -c:s:0 srt -c:s:1 srt -c:s:2 ass -c:s:3 ass \
    -metadata:s:s:0 language=ger -metadata:s:s:0 title="Deutsch (SRT)" \
    -metadata:s:s:1 language=eng -metadata:s:s:1 title="English (SRT)" \
    -metadata:s:s:2 language=ger -metadata:s:s:2 title="Deutsch (ASS)" \
    -metadata:s:s:3 language=eng -metadata:s:s:3 title="English (ASS)" \
    -disposition:s:0 default -metadata title="Slouchcat Test Subtitles" "$f"
fi
embedded="$f"

name="Slouchcat Test Sidecar Subtitles (2024)"
f="$(movie_path "$name" mkv)"
if want "$f"; then
  "${ff[@]}" -i "$embedded" -map 0:v -map 0:a -c copy -metadata title="Slouchcat Test Sidecar Subtitles" "$f"
  for lang in de en; do
    cp "$subs_dir/$lang.srt" "$(dirname "$f")/$name.$lang.srt"
    cp "$subs_dir/$lang.ass" "$(dirname "$f")/$name.$lang.ass"
  done
fi

# (g) Chapters from an ffmetadata file (3 chapters).
cat >"$subs_dir/chapters.ffmeta" <<'META'
;FFMETADATA1
title=Slouchcat Test Chapters

[CHAPTER]
TIMEBASE=1/1000
START=0
END=3000
title=Kapitel 1: Anfang

[CHAPTER]
TIMEBASE=1/1000
START=3000
END=6500
title=Kapitel 2: Mitte

[CHAPTER]
TIMEBASE=1/1000
START=6500
END=10000
title=Kapitel 3: Ende
META

f="$(movie_path "Slouchcat Test Chapters (2024) {tmdb-7004}" mkv)"
if want "$f"; then
  "${ff[@]}" "${v10[@]}" "${a10[@]}" -i "$subs_dir/chapters.ffmeta" \
    -map 0:v -map 1:a -map_metadata 2 -map_chapters 2 "${x264[@]}" "${aac[@]}" "$f"
fi

# (h) Long clips for the HLS seek test (M1b, web/e2e/hls.spec.ts): 90 s at 720p keeps generation
# fast. HEVC 10-bit → transcode in Chromium/Firefox; H.264/AAC in MKV → remux, with irregular
# keyframes so the remux segment boundaries follow the keyframe index (plan 3.5).
mapfile -t v90 < <(printf '%s\n' -f lavfi -i "testsrc2=size=1280x720:rate=24:duration=90")
mapfile -t a90 < <(tone 90 440)

f="$(movie_path "Slouchcat Test HEVC Long (2024)" mkv)"
if want "$f"; then
  "${ff[@]}" "${v90[@]}" "${a90[@]}" \
    -c:v libx265 -preset ultrafast -crf 32 -g 48 -pix_fmt yuv420p10le \
    -color_primaries bt709 -color_trc bt709 -colorspace bt709 \
    -x265-params log-level=error \
    "${aac[@]}" -metadata title="Slouchcat Test HEVC Long" "$f"
fi

f="$(movie_path "Slouchcat Test H264 MKV Long (2024) {tmdb-7005}" mkv)"
if want "$f"; then
  "${ff[@]}" "${v90[@]}" "${a90[@]}" \
    -c:v libx264 -preset veryfast -crf 30 -pix_fmt yuv420p \
    -color_primaries bt709 -color_trc bt709 -colorspace bt709 \
    -x264-params "keyint=infinite:scenecut=0" \
    -force_key_frames 0,2,7,8,15,21,30,31,45,60,61,75 \
    "${aac[@]}" -metadata title="Slouchcat Test H264 MKV Long" "$f"
fi

# Series example: two episodes.
show="Slouchcat Test Show (2024)"
for ep in 1 2; do
  f="$(printf '%s/TV/%s/Season 01/%s S01E%02d.mp4' "$OUT_DIR" "$show" "$show" "$ep")"
  if want "$f"; then
    if ((ep == 1)); then src="testsrc2"; else src="smptebars"; fi
    "${ff[@]}" -f lavfi -i "$src=size=1920x1080:rate=24:duration=10" "${a10[@]}" \
      "${x264[@]}" "${aac[@]}" -movflags +faststart \
      -metadata title="Episode $ep" -metadata show="Slouchcat Test Show" \
      -metadata season_number=1 -metadata episode_sort="$ep" "$f"
  fi
done

# M3 library cases (B-5, B-7, B-9). The web e2e specs use these names.

# clip <file> <seconds> <lavfi video source> <tone Hz> <title>: short H.264/AAC 1080p MP4.
clip() {
  "${ff[@]}" -f lavfi -i "$3=size=1920x1080:rate=24:duration=$2" \
    -f lavfi -i "sine=frequency=$4:sample_rate=48000:duration=$2" \
    "${x264[@]}" "${aac[@]}" -movflags +faststart -metadata title="$5" "$1"
}

# Specials and a file with two episodes.
f="$OUT_DIR/TV/$show/Specials/$show S00E01.mp4"
if want "$f"; then
  clip "$f" 10 smptebars 660 "Special 1"
fi
f="$OUT_DIR/TV/$show/Season 01/$show S01E03-E04.mp4"
if want "$f"; then
  clip "$f" 10 testsrc2 550 "Episode 3 and 4"
fi

# Second version of the H.264 movie (B-5): the same source at 1280x720.
h264="$(movie_path "Slouchcat Test H264 (2024)" mp4)"
f="$OUT_DIR/Movies/Slouchcat Test H264 (2024)/Slouchcat Test H264 (2024) - 720p.mp4"
if want "$f"; then
  "${ff[@]}" "${v10[@]}" "${a10[@]}" -vf scale=1280:720 "${x264[@]}" "${aac[@]}" -movflags +faststart \
    -metadata title="Slouchcat Test H264 720p" "$f"
fi

# A movie with extras in every supported form (B-7): five-second clips.
extras_dir="$OUT_DIR/Movies/Slouchcat Test Extras (2024)"
f="$(movie_path "Slouchcat Test Extras (2024)" mp4)"
if want "$f"; then
  clip "$f" 10 testsrc2 440 "Slouchcat Test Extras"
fi
for extra in "Trailers/Slouchcat Test Extras Trailer" "Featurettes/Making Of" "Deleted Scenes/Scene 1" \
  "Behind The Scenes/On Set" "Slouchcat Test Extras (2024)-trailer"; do
  f="$extras_dir/$extra.mp4"
  if want "$f"; then
    clip "$f" 5 smptebars 880 "$(basename "$extra")"
  fi
done

# Unplayable file (B-9): a text file with a video extension; ffprobe fails on it. Not part of the
# ffprobe summary below.
broken="$(movie_path "Slouchcat Test Broken (2024)" mkv)"
if [[ ! -f "$broken" || "$FORCE" == "1" ]]; then
  mkdir -p "$(dirname "$broken")"
  echo "generate: ${broken#"$OUT_DIR"/}"
  printf 'Slouchcat test file: this is text, not a Matroska video.\n' >"$broken"
else
  echo "skip (exists): ${broken#"$OUT_DIR"/}"
fi

# Duplicate (B-9): a byte copy of the H.264 movie, so both have the same partial hash.
f="$(movie_path "Slouchcat Test H264 Copy (2024)" mp4)"
if want "$f"; then
  cp "$h264" "$f"
fi

# A movie without year and without rating (B-9 "unrated").
f="$(movie_path "Slouchcat Test Unrated" mp4)"
if want "$f"; then
  clip "$f" 10 testsrc2 330 "Slouchcat Test Unrated"
fi

# M4 playback cases (W-2, W-4, test matrix 10.3).

# Multiple audio tracks and embedded SRT, 20 s so a track switch mid-play is testable:
# 0 video, 1 AAC stereo German (default), 2 AC3 5.1 English, 3 SRT German, 4 SRT English.
write_srt20() { # <file> <word>: five captions "<word> 1" ... "<word> 5", every 4 s
  local i
  : >"$1"
  for i in 1 2 3 4 5; do
    printf '%d\n00:00:%02d,000 --> 00:00:%02d,000\n%s %d\n\n' "$i" $((i * 4 - 3)) $((i * 4 - 1)) "$2" "$i" >>"$1"
  done
}
write_srt20 "$subs_dir/multi-de.srt" "Deutsch"
write_srt20 "$subs_dir/multi-en.srt" "English"
mapfile -t v20 < <(video_1080 20)
f="$(movie_path "Slouchcat Test Multi Audio (2024)" mkv)"
if want "$f"; then
  "${ff[@]}" "${v20[@]}" \
    -f lavfi -i "sine=frequency=440:sample_rate=48000:duration=20" \
    -f lavfi -i "sine=frequency=660:sample_rate=48000:duration=20" \
    -i "$subs_dir/multi-de.srt" -i "$subs_dir/multi-en.srt" \
    -map 0:v -map 1:a -map 2:a -map 3 -map 4 \
    "${x264[@]}" -c:a:0 aac -b:a:0 128k -ac:a:0 2 \
    -filter:a:1 "pan=5.1(side)|FL=c0|FR=c0|FC=c0|LFE=c0|SL=c0|SR=c0" -c:a:1 ac3 -b:a:1 384k \
    -c:s srt \
    -metadata:s:a:0 language=ger -metadata:s:a:0 title="Deutsch (AAC 2.0)" \
    -metadata:s:a:1 language=eng -metadata:s:a:1 title="English (AC3 5.1)" \
    -metadata:s:s:0 language=ger -metadata:s:s:0 title="Deutsch (SRT)" \
    -metadata:s:s:1 language=eng -metadata:s:s:1 title="English (SRT)" \
    -disposition:a:0 default -disposition:a:1 0 -disposition:s:0 0 -disposition:s:1 0 \
    -metadata title="Slouchcat Test Multi Audio" "$f"
fi

# PGS image subtitles (ffmpeg has no PGS encoder): two captions from make-pgs.py,
# "Slouchcat PGS 1" at 2-6 s and "Slouchcat PGS 2" at 8-12 s; stream 2 is hdmv_pgs_subtitle.
f="$(movie_path "Slouchcat Test PGS (2024)" mkv)"
if want "$f"; then
  python3 -I "$script_dir/make-pgs.py" "$subs_dir/captions.sup"
  "${ff[@]}" "${v20[@]}" -f lavfi -i "sine=frequency=440:sample_rate=48000:duration=20" \
    -i "$subs_dir/captions.sup" -map 0:v -map 1:a -map 2 \
    "${x264[@]}" "${aac[@]}" -c:s copy \
    -metadata:s:s:0 language=eng -metadata:s:s:0 title="English (PGS)" \
    -metadata title="Slouchcat Test PGS" "$f"
  [[ "$("$FFPROBE" -v error -select_streams s:0 -show_entries stream=codec_name -of csv=p=0 "$f")" == hdmv_pgs_subtitle ]] ||
    die "$f has no hdmv_pgs_subtitle stream"
fi

hdr10=(-c:v libx265 -preset ultrafast -g 48 -pix_fmt yuv420p10le
  -color_primaries bt2020 -color_trc smpte2084 -colorspace bt2020nc)
hdr10_params="log-level=error:hdr10=1:repeat-headers=1:colorprim=bt2020:transfer=smpte2084:colormatrix=bt2020nc:master-display=G(13250,34500)B(7500,3000)R(34000,16000)WP(15635,16450)L(10000000,1):max-cll=1000,400"

# HDR10 test chart for P-6 (testdata/hdr-quality.sh): 3840x2160 PQ/BT.2020, 10 s, a grey wedge
# of 11 PQ steps (0-100 %) across the top half and six skin-tone patches in the bottom half;
# geometry and values in hdr-quality.py and README.md.
f="$(movie_path "Slouchcat Test HDR10 Chart (2024)" mkv)"
if want "$f"; then
  python3 -I "$script_dir/hdr-quality.py" chart "$subs_dir/chart.ppm"
  "${ff[@]}" -loop 1 -framerate 24 -t 10 -i "$subs_dir/chart.ppm" \
    -f lavfi -i "sine=frequency=440:sample_rate=48000:duration=10" \
    -vf "scale=out_color_matrix=bt2020:out_range=tv:flags=accurate_rnd+full_chroma_int,format=yuv420p10le" \
    "${hdr10[@]}" -crf 16 -x265-params "$hdr10_params" \
    "${aac[@]}" -metadata title="Slouchcat Test HDR10 Chart" "$f"
  rm -f "$subs_dir/chart.ppm"
fi

# The 4K HDR10 pattern at 120 s for the 5-s start measurement (W-3); slow, only with LONG=1.
if [[ "${LONG:-0}" == "1" ]]; then
  f="$(movie_path "Slouchcat Test HDR10 4K Long (2024)" mkv)"
  if want "$f"; then
    "${ff[@]}" -f lavfi -i "smptehdbars=size=3840x2160:rate=24:duration=120" \
      -f lavfi -i "sine=frequency=440:sample_rate=48000:duration=120" \
      -vf "format=yuv420p10le" "${hdr10[@]}" -crf 34 -x265-params "$hdr10_params" \
      "${aac[@]}" -metadata title="Slouchcat Test HDR10 4K Long" "$f"
  fi
fi

# Intro and end-credits season (M4, W-9): five episodes of about 100 s with the same 20-s intro
# melody at different offsets and the same 25-s outro melody at the end, between tones of their
# own; E03 also carries the chapters "Intro" and "Credits". Lengths follow the detection rules
# (package segments): the intro lies in the first 40 %, an outro is at least 20 s long.

# melody <out.wav> <seconds per tone> <Hz>...: sine tones one after the other.
melody() {
  local out="$1" len="$2" in=() labels="" n=0
  shift 2
  for hz in "$@"; do
    in+=(-f lavfi -i "sine=frequency=$hz:sample_rate=48000:duration=$len")
    labels+="[$n:a]"
    n=$((n + 1))
  done
  "${ff[@]}" "${in[@]}" -filter_complex "${labels}concat=n=$n:v=0:a=1" "$out"
}

# own_tones <out.wav> <seconds> <seed>: tones of 0.5 to 1.5 s at pseudo-random 150 to 1800 Hz,
# deterministic for a seed (a linear congruential generator, no $RANDOM).
own_tones() {
  local out="$1" left=$(($2 * 1000)) seed="$3" in=() labels="" n=0 ms hz
  while ((left > 0)); do
    seed=$(((seed * 1103515245 + 12345) % 2147483648))
    ms=$((500 + seed % 1000))
    ((ms > left)) && ms=$left
    seed=$(((seed * 1103515245 + 12345) % 2147483648))
    hz=$((150 + seed % 1650))
    in+=(-f lavfi -i "sine=frequency=$hz:sample_rate=48000:duration=$((ms / 1000)).$(printf '%03d' $((ms % 1000)))")
    labels+="[$n:a]"
    n=$((n + 1))
    left=$((left - ms))
  done
  "${ff[@]}" "${in[@]}" -filter_complex "${labels}concat=n=$n:v=0:a=1" "$out"
}

intro_show="Slouchcat Test Intro Show (2024) {tmdb-7102}"
intro_starts=(5 12 8 15 10) # seconds of own tones before the intro, per episode
intro_wav="$subs_dir/intro.wav"
outro_wav="$subs_dir/outro.wav"
for ep in 1 2 3 4 5; do
  f="$(printf '%s/TV/%s/Season 01/%s - S01E%02d.mkv' "$OUT_DIR" "$intro_show" "$intro_show" "$ep")"
  if ! want "$f"; then
    continue
  fi
  if [[ ! -f "$intro_wav" ]]; then
    melody "$intro_wav" 2.5 523.25 659.25 783.99 1046.5 880 698.46 587.33 493.88
    melody "$outro_wav" 2.5 392 440 493.88 523.25 587.33 659.25 698.46 783.99 880 987.77
  fi
  start=${intro_starts[ep - 1]}
  dur=$((96 + 2 * ep))
  credits=$((dur - 25))
  own_tones "$subs_dir/pre.wav" "$start" $((ep * 7919))
  own_tones "$subs_dir/mid.wav" $((credits - start - 20)) $((ep * 104729))
  chapters=()
  if ((ep == 3)); then
    cat >"$subs_dir/intro-chapters.ffmeta" <<META
;FFMETADATA1

[CHAPTER]
TIMEBASE=1/1000
START=0
END=$((start * 1000))
title=Prolog

[CHAPTER]
TIMEBASE=1/1000
START=$((start * 1000))
END=$(((start + 20) * 1000))
title=Intro

[CHAPTER]
TIMEBASE=1/1000
START=$(((start + 20) * 1000))
END=$((credits * 1000))
title=Handlung

[CHAPTER]
TIMEBASE=1/1000
START=$((credits * 1000))
END=$((dur * 1000))
title=Credits
META
    chapters=(-i "$subs_dir/intro-chapters.ffmeta" -map_chapters 5)
  fi
  "${ff[@]}" -f lavfi -i "testsrc2=size=640x360:rate=24:duration=$dur" \
    -i "$subs_dir/pre.wav" -i "$intro_wav" -i "$subs_dir/mid.wav" -i "$outro_wav" "${chapters[@]}" \
    -filter_complex "[1:a][2:a][3:a][4:a]concat=n=4:v=0:a=1[a]" -map 0:v -map "[a]" \
    "${x264[@]}" "${aac[@]}" -metadata title="Episode $ep" "$f"
done

# M5 offline (O-2 season download): six short episodes, 1280x720 H.264/AAC MP4 of 8 s each, a
# tone of its own per episode; episode 1 has a German SRT next to it (a WebVTT sidecar of the
# download). The web e2e specs use these names.
download_show="Slouchcat Test Download Show (2024) {tmdb-7101}"
write_srt "$subs_dir/download-de.srt" "Erster Untertitel der Download-Serie" \
  "Zweiter Untertitel mit Umlauten: äöü" "Dritter Untertitel"
for ep in 1 2 3 4 5 6; do
  f="$(printf '%s/TV/%s/Season 01/%s S01E%02d.mp4' "$OUT_DIR" "$download_show" "$download_show" "$ep")"
  if want "$f"; then
    "${ff[@]}" -f lavfi -i "testsrc2=size=1280x720:rate=24:duration=8" \
      -f lavfi -i "sine=frequency=$((300 + 50 * ep)):sample_rate=48000:duration=8" \
      "${x264[@]}" "${aac[@]}" -movflags +faststart \
      -metadata title="Episode $ep" -metadata show="Slouchcat Test Download Show" \
      -metadata season_number=1 -metadata episode_sort="$ep" "$f"
    if ((ep == 1)); then
      cp "$subs_dir/download-de.srt" "${f%.mp4}.de.srt"
    fi
  fi
done

# M6 audio (H-1, H-2, O-1 audio): an audiobook library with one M4B of 12 chapters (chapter marks
# from an ffmetadata file, cover art embedded as attached picture, author/narrator/series in tags
# like real M4B files: artist = author, composer = narrator, show = series, episode_id = volume)
# and one book as a folder of 10 MP3 files (one chapter each, cover.jpg next to them), plus three
# podcast episodes and a feed template for the local mock feed server (no network in CI).
audio_dir="$OUT_DIR/Audiobooks"
audio_author="Slouchcat Test Author"
m4b_title="Slouchcat Test Audiobook M4B"
mp3_title="Slouchcat Test Audiobook MP3"
audio_series="Slouchcat Test Audio Series"

# cover <out.jpg> <text colour>: a 600x600 cover from lavfi colour bars with a solid frame.
cover() {
  "${ff[@]}" -f lavfi -i "smptebars=size=600x600:rate=1:duration=1" \
    -vf "drawbox=x=20:y=20:w=560:h=560:color=$2@0.8:t=24" -frames:v 1 -q:v 3 "$1"
}

f="$audio_dir/$audio_author/$audio_series/01 - $m4b_title/$m4b_title.m4b"
if want "$f"; then
  cover "$subs_dir/m4b-cover.jpg" red
  {
    echo ";FFMETADATA1"
    for ch in $(seq 1 12); do
      printf '\n[CHAPTER]\nTIMEBASE=1/1000\nSTART=%d\nEND=%d\ntitle=Kapitel %d\n' \
        $(((ch - 1) * 5000)) $((ch * 5000)) "$ch"
    done
  } >"$subs_dir/m4b-chapters.ffmeta"
  own_tones "$subs_dir/m4b.wav" 60 424242
  "${ff[@]}" -i "$subs_dir/m4b.wav" -i "$subs_dir/m4b-cover.jpg" -i "$subs_dir/m4b-chapters.ffmeta" \
    -map 0:a -map 1:v -map_chapters 2 -c:a aac -b:a 64k -ac 1 -c:v:0 mjpeg -disposition:v:0 attached_pic \
    -metadata title="$m4b_title" -metadata artist="$audio_author" -metadata album_artist="$audio_author" \
    -metadata composer="Slouchcat Test Narrator" -metadata album="$m4b_title" -metadata date=2024 \
    -metadata genre="Test" -metadata show="$audio_series" -metadata episode_id=1 \
    -metadata description="Ein Testhoerbuch mit zwoelf Kapiteln aus Sinustoenen." \
    -f mp4 -movflags +faststart "$f"
fi

mp3_dir="$audio_dir/$audio_author/$audio_series/02 - $mp3_title"
for ch in $(seq 1 10); do
  f="$(printf '%s/%02d - Kapitel %d.mp3' "$mp3_dir" "$ch" "$ch")"
  if want "$f"; then
    if [[ ! -f "$mp3_dir/cover.jpg" ]]; then cover "$mp3_dir/cover.jpg" blue; fi
    "${ff[@]}" -f lavfi -i "sine=frequency=$((200 + 40 * ch)):sample_rate=44100:duration=6" \
      -c:a libmp3lame -b:a 64k -ac 1 -id3v2_version 3 \
      -metadata title="Kapitel $ch" -metadata artist="$audio_author" -metadata album_artist="$audio_author" \
      -metadata composer="Slouchcat Test Narrator" -metadata album="$mp3_title" -metadata track="$ch/10" \
      -metadata date=2023 -metadata genre="Test" "$f"
  fi
done

# Podcast episodes for the mock feed (testdata/podcasts/feed.xml.tmpl has {{BASE}} placeholders).
# The tags are what Audiobookshelf writes when it downloads an episode (ADR 0015, concept 4.5):
# title, album = podcast, date = the feed's pubDate in RFC 2822, comment = the description. The
# dates repeat the template, so a file and its feed episode pair by the publication minute.
podcast_dir="$OUT_DIR/Podcasts"
podcast_dates=("" "Mon, 06 Oct 2025 06:00:00 +0000" "Mon, 13 Oct 2025 06:00:00 +0000" "Mon, 20 Oct 2025 06:00:00 +0000")
for ep in 1 2 3; do
  f="$podcast_dir/episode-$ep.mp3"
  if want "$f"; then
    if [[ ! -f "$podcast_dir/cover.jpg" ]]; then cover "$podcast_dir/cover.jpg" green; fi
    "${ff[@]}" -f lavfi -i "sine=frequency=$((500 + 100 * ep)):sample_rate=44100:duration=$((6 + 2 * ep))" \
      -c:a libmp3lame -b:a 64k -ac 1 -id3v2_version 3 \
      -metadata title="Slouchcat Test Episode $ep" -metadata artist="Slouchcat Test Podcast" \
      -metadata album="Slouchcat Test Podcast" -metadata date="${podcast_dates[$ep]}" \
      -metadata comment="Episode $ep aus dem Ordner." "$f"
  fi
done

# A podcast folder as Audiobookshelf lays it out (ADR 0015): metadata.json without feedURL (the
# Go tests write a feedURL of their own when they need one), cover.jpg, two episodes with tags,
# one of them with the podcast's title as its title tag (Audiobookshelf does that for some
# episodes; the name of the file counts then), and one audio file directly in the root, which a
# scan passes over.
folder_dir="$OUT_DIR/PodcastFolder"
show_dir="$folder_dir/Slouchcat Test Folder Podcast"
f="$show_dir/metadata.json"
if [[ ! -f "$f" || "$FORCE" == "1" ]]; then
  mkdir -p "$show_dir"
  cover "$show_dir/cover.jpg" yellow
  cat >"$f" <<'JSON'
{
  "tags": [],
  "title": "Slouchcat Test Folder Podcast",
  "author": "Slouchcat Test Author",
  "description": "Ein Podcast aus einem Ordner, wie Audiobookshelf ihn ablegt.",
  "releaseDate": "2025-10-20T06:00:00Z",
  "genres": ["Education", "Podcasts"],
  "language": "de",
  "explicit": false,
  "podcastType": "episodic"
}
JSON
fi
folder_dates=("" "Mon, 03 Nov 2025 06:00:00 +0000" "Mon, 10 Nov 2025 06:00:00 +0000")
folder_titles=("" "Ordnerfolge Eins" "Slouchcat Test Folder Podcast")
folder_names=("" "Ordnerfolge Eins.mp3" "Slouchcat Test Folder Podcast (3f5a2c1e-0000-4000-8000-000000000002).mp3")
for ep in 1 2; do
  f="$show_dir/${folder_names[$ep]}"
  if want "$f"; then
    "${ff[@]}" -f lavfi -i "sine=frequency=$((900 + 100 * ep)):sample_rate=44100:duration=$((5 + ep))" \
      -c:a libmp3lame -b:a 64k -ac 1 -id3v2_version 3 \
      -metadata title="${folder_titles[$ep]}" -metadata artist="Slouchcat Test Author" \
      -metadata album="Slouchcat Test Folder Podcast" -metadata date="${folder_dates[$ep]}" \
      -metadata comment="Ordnerfolge $ep." "$f"
  fi
done
f="$folder_dir/loose.mp3"
if want "$f"; then
  "${ff[@]}" -f lavfi -i "sine=frequency=1200:sample_rate=44100:duration=3" -c:a libmp3lame -b:a 64k -ac 1 "$f"
fi

# Summary.
echo
echo "== ffprobe summary ($OUT_DIR)"
for f in "${generated[@]}"; do
  echo "-- ${f#"$OUT_DIR"/}"
  "$FFPROBE" -v error \
    -show_entries "format=duration,format_name:stream=index,codec_type,codec_name,profile,pix_fmt,width,height,channels,channel_layout,color_primaries,color_transfer:stream_tags=language" \
    -of compact=nokey=0 "$f"
  chapters="$("$FFPROBE" -v error -show_chapters -of csv=p=0 "$f" | wc -l)"
  if ((chapters > 0)); then echo "chapters=$chapters"; fi
done

hdr="$(movie_path "Slouchcat Test HDR10 4K (2024)" mkv)"
echo "-- HDR10 side data (first frame) of ${hdr#"$OUT_DIR"/}"
"$FFPROBE" -v error -select_streams v:0 -read_intervals "%+#1" \
  -show_entries "frame_side_data=side_data_type,min_luminance,max_luminance,max_content,max_average" \
  -of default=noprint_wrappers=1 "$hdr" | grep -v "User Data Unregistered" || echo "no HDR side data found"
