#!/usr/bin/env bash
# The screenshots of the store listing (docs/store/grafiken.md) from the emulators, against a demo
# server with the four open films of the Blender Foundation (deploy/demo/), into a folder OUTSIDE
# the repository: the photos never go into the repository.
#
#   scripts/store-screenshots.sh <target folder>
#
# Writes <target>/{phone,tablet7,tablet10,tv}-{de,en}/NN-<motif>.png (PNG, 24 bit RGB) and checks
# size, format and aspect ratio of every file against docs/store/grafiken.md.
#
# What it does:
#   1. The films: deploy/demo/prepare-media.sh into MEDIA_CACHE (default ~/.cache/slouchcat-demo),
#      downloaded once (1.6 GB from download.blender.org, checked against films.tsv). The audiobook
#      is the made-up one of scripts/android-test-video.sh («The Quiet Harbour»), with a cover drawn
#      here. Per language a tree of hard links. Poster and backdrop are frames of the films
#      themselves: no TMDB key, no picture from outside.
#   2. Per language one server of this repository (bin/slouchcat, software encoding) with fresh
#      data on the loopback (de on PORT, en on PORT+2), set up over the API: account
#      demo@example.com, the primary profile «Demo» with a PIN and a second profile, the film
#      library with trickplay «Im Voraus» (one backfill run), the audiobooks; per film a summary in
#      that language, genres, poster and backdrop through the administration.
#      The helpers of scripts/android-helpers.sh (the pairing relay on PORT+85 among them).
#   3. Per device one emulator after the other (the machine-wide lock of android-helpers.sh):
#      sc_phone (1080 × 2424; the player with a display of 1080 × 1920, so its landscape photo is
#      1920 × 1080), sc_tablet twice with a 16:9 display (1920 × 1080 for 7 inch, 2560 × 1440 for
#      10 inch; the AVD itself is 16:10) and sc_tv (1920 × 1080). Per language: the app's data
#      cleared, the language set (de-CH, en), the test class StoreShotsTest of the app with the
#      switch slouchcatStore, the photos copied. The status bar in its demo mode (12:00, full).
#   4. The relay goes with the run: the QR code on the TV photo names a meeting point that no
#      longer exists.
#
# PORT        port of the German server (default 8897; the English one PORT+2, the helpers PORT-12
#             to PORT+85), all on 127.0.0.1; never 8096
# DEVICES     which devices (default "phone tablet tv"); LANGS which languages (default "de en")
# MEDIA_CACHE folder for the films and the trees (default ~/.cache/slouchcat-demo), outside the repo
# WAIT_MAX    seconds to wait for the emulator lock (default 2400)
#
# Needs: Go, Node (the helpers), ffmpeg, python3, the Android SDK of scripts/install-android-tools.sh
# with the AVDs sc_phone, sc_tablet and sc_tv, curl, flock. Builds bin/slouchcat (task server:build),
# relay/bin (task relay:build) and the debug APKs itself.
set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "usage: scripts/store-screenshots.sh <target folder outside the repository>" >&2
  exit 2
fi
repo=$(cd "$(dirname "$0")/.." && pwd -P)
target=$(realpath -m -- "$1")
case "$target/" in
  "$repo"/*)
    echo "refusing $target: the store photos stay outside the repository ($repo)" >&2
    exit 1
    ;;
esac

port=${PORT:-8897}
devices=${DEVICES:-phone tablet tv}
langs=${LANGS:-de en}
cache=$(realpath -m -- "${MEDIA_CACHE:-$HOME/.cache/slouchcat-demo}")
wait_max=${WAIT_MAX:-2400}
case "$cache/" in
  "$repo"/*)
    echo "refusing MEDIA_CACHE=$cache: the films stay outside the repository" >&2
    exit 1
    ;;
esac
if [ "$port" = 8096 ] || [ "$((port + 2))" = 8096 ]; then
  echo "refusing PORT=$port: 8096 is not ours" >&2
  exit 1
fi

sdk=${ANDROID_HOME:-$HOME/.local/share/android-sdk}
app=com.slouchcat.app.dev
email=demo@example.com
password=demo-password-0123
pin=246810
relay_port=$((port + 85))
run=$(mktemp -d -t slouchcat-store-XXXXXX)
server_pids=()
emulator_pid=""
serial=""

say() { echo "store-screenshots: $*"; }
fail() {
  echo "store-screenshots: $*" >&2
  exit 1
}

# shellcheck source=android-helpers.sh
cd "$repo/android"
. ../scripts/android-helpers.sh

adb_to() { timeout "${ADB_LIMIT:-120}" "$sdk/platform-tools/adb" -s "$serial" "$@"; }

emulator_stop() {
  [ -n "$emulator_pid" ] || return 0
  timeout 30 "$sdk/platform-tools/adb" -s "$serial" emu kill >/dev/null 2>&1 || true
  for _ in $(seq 1 30); do kill -0 "$emulator_pid" 2>/dev/null || break; sleep 1; done
  kill "$emulator_pid" 2>/dev/null || true
  wait "$emulator_pid" 2>/dev/null || true
  emulator_pid=""
  emulator_unlock
}

cleanup() {
  status=$?
  if [ "$status" -ne 0 ]; then
    mkdir -p "$target/logs"
    cp "$run"/*.log "$target/logs/" 2>/dev/null || true
    helpers_save_logs "$target/logs"
    if [ -n "$emulator_pid" ]; then timeout 30 "$sdk/platform-tools/adb" -s "$serial" logcat -d >"$target/logs/logcat.txt" 2>/dev/null || true; fi
    echo "store-screenshots: logs in $target/logs" >&2
  fi
  emulator_stop
  local pid
  for pid in "${server_pids[@]}"; do kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true; done
  # The relay goes: the meeting point of the QR code on the TV photo is gone with it.
  helpers_stop
  rm -rf "$run"
}
trap cleanup EXIT

command -v ffmpeg >/dev/null || fail "ffmpeg is missing"
command -v python3 >/dev/null || fail "python3 is missing"

# ---- 1. the media ------------------------------------------------------------------------------

say "films into $cache/media (downloaded once)"
mkdir -p "$cache/media"
DEMO_MEDIA="$cache/media" DEMO_LIST="$repo/deploy/demo/films.tsv" sh "$repo/deploy/demo/prepare-media.sh" >"$run/prepare-media.log" 2>&1 ||
  { cat "$run/prepare-media.log" >&2; fail "the films are not complete"; }

# Pictures from the films themselves: a frame cropped to a poster (2:3) and one as the backdrop.
# folder|seconds of the poster|centre of the poster (share of the width)|seconds of the backdrop
art() {
  case "$1" in
    "Tears of Steel (2012)") echo "510 0.30 330" ;;
    "Sintel (2010)") echo "60 0.44 330" ;;
    "Elephants Dream (2006)") echo "60 0.60 330" ;;
    "Big Buck Bunny (2008)") echo "195 0.47 285" ;;
  esac
}
mkdir -p "$cache/art"
for dir in "$cache/media/Movies"/*/; do
  folder=$(basename "$dir")
  read -r at centre back <<<"$(art "$folder")"
  [ -n "${at:-}" ] || fail "no pictures defined for $folder"
  video=$(find "$dir" -maxdepth 1 \( -name '*.mov' -o -name '*.mkv' \) | head -n 1)
  if [ ! -s "$cache/art/$folder-poster.jpg" ]; then
    ffmpeg -nostdin -loglevel error -y -ss "$at" -i "$video" -frames:v 1 \
      -vf "crop=ih*2/3:ih:max(0\,min(iw-ih*2/3\,iw*$centre-ih/3)):0,scale=1000:1500:flags=lanczos" -q:v 3 "$cache/art/$folder-poster.jpg"
    ffmpeg -nostdin -loglevel error -y -ss "$back" -i "$video" -frames:v 1 -vf "scale=1920:-2:flags=lanczos" -q:v 3 "$cache/art/$folder-fanart.jpg"
  fi
done

# The audiobook: the made-up one of the emulator flows, with a cover drawn here (no picture from outside).
if [ ! -d "$cache/sound/Playback/Audiobooks" ]; then
  rm -rf "$cache/sound"
  mkdir -p "$cache/sound"
  ../scripts/android-test-video.sh "$cache/sound"
fi
book=$(find "$cache/sound/Playback/Audiobooks" -mindepth 2 -maxdepth 2 -type d | head -n 1)
[ -n "$book" ] || fail "android-test-video.sh wrote no audiobook"
if [ ! -s "$book/cover.jpg" ]; then
  font=/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf
  draw=""
  if [ -f "$font" ]; then
    draw=",drawtext=fontfile=$font:text='The Quiet Harbour':fontcolor=white:fontsize=96:x=(w-text_w)/2:y=h*0.62,drawtext=fontfile=$font:text='Mira Lindqvist':fontcolor=white@0.85:fontsize=56:x=(w-text_w)/2:y=h*0.74"
  fi
  ffmpeg -nostdin -loglevel error -y -f lavfi -i "gradients=s=1200x1200:c0=0x0b2a3c:c1=0x1f6f78:c2=0xe3a857:n=3:type=linear:seed=7,format=rgb24" \
    -frames:v 1 -vf "boxblur=8$draw" -q:v 3 "$book/cover.jpg"
fi

# The texts of the films (own words). The servers get them, and the pictures, over the
# administration as an administrator without a TMDB key would: without a key metadata.match skips a
# film, so neither NFO texts nor local images reach it, but what «updateItemMetadata» and
# «uploadItemImage» write stays (locked).
film_overview() { # folder lang
  case "$1|$2" in
    "Tears of Steel (2012)|de") echo "Im Amsterdam der Zukunft wollen Wissenschaftler und Kämpfer die Welt vor zerstörerischen Robotern retten: Sie stellen einen entscheidenden Moment aus der Vergangenheit nach." ;;
    "Tears of Steel (2012)|en") echo "In a future Amsterdam, scientists and fighters try to save the world from destructive robots by staging a decisive moment from the past once more." ;;
    "Sintel (2010)|de") echo "Eine junge Frau sucht den kleinen Drachen, den sie einst gerettet und aufgezogen hat. Ihre Suche führt sie durch Schnee, Städte und Wüsten." ;;
    "Sintel (2010)|en") echo "A young woman searches for the little dragon she once rescued and raised. Her search leads her through snow, cities and deserts." ;;
    "Elephants Dream (2006)|de") echo "Zwei Männer durchstreifen eine seltsame, endlose Maschine. Der Ältere führt den Jüngeren, doch die beiden sehen darin ganz verschiedene Welten." ;;
    "Elephants Dream (2006)|en") echo "Two men roam a strange, endless machine. The older one guides the younger, but the two see very different worlds in it." ;;
    "Big Buck Bunny (2008)|de") echo "Ein grosses, gutmütiges Kaninchen rächt sich an drei Nagern, die ihm den ruhigen Tag auf der Wiese verderben." ;;
    "Big Buck Bunny (2008)|en") echo "A big, good-natured rabbit gets even with three rodents who spoil his quiet day in the meadow." ;;
  esac
}
film_genres() { # folder lang: a JSON list
  case "$1|$2" in
    "Tears of Steel (2012)|"*) echo '["Science-Fiction","Action"]' ;;
    "Sintel (2010)|de") echo '["Fantasy","Abenteuer","Animation"]' ;;
    "Sintel (2010)|en") echo '["Fantasy","Adventure","Animation"]' ;;
    "Elephants Dream (2006)|"*) echo '["Science-Fiction","Animation"]' ;;
    "Big Buck Bunny (2008)|de") echo '["Komödie","Animation"]' ;;
    "Big Buck Bunny (2008)|en") echo '["Comedy","Animation"]' ;;
  esac
}
# Per language a tree of hard links (no copy, no symbolic link out of the root).
for lang in $langs; do
  tree="$cache/lang-$lang"
  rm -rf "$tree"
  mkdir -p "$tree/Movies" "$tree/Audiobooks"
  for dir in "$cache/media/Movies"/*/; do
    folder=$(basename "$dir")
    mkdir -p "$tree/Movies/$folder"
    for file in "$dir"*; do ln -f "$file" "$tree/Movies/$folder/"; done
  done
  cp -al "$(dirname "$book")" "$tree/Audiobooks/"
done

# ---- 2. the servers ----------------------------------------------------------------------------

say "building the server, the relay and the APKs"
(cd "$repo" && task server:build relay:build) >"$run/build.log" 2>&1 || { tail -n 30 "$run/build.log" >&2; fail "the build failed"; }
../scripts/heavy-slot.sh ./gradlew --console=plain -q :app-mobile:assemblePlayDebug :app-mobile:assemblePlayDebugAndroidTest \
  :app-tv:assembleDebug :app-tv:assembleDebugAndroidTest >"$run/gradle.log" 2>&1 || { tail -n 30 "$run/gradle.log" >&2; fail "the APKs did not build"; }

helpers_start "$run" "$port" "$relay_port" || fail "a helper did not come up"

api() { # jar method path [json]: prints the body, fails on an HTTP error
  local jar=$1 method=$2 path=$3 body=${4:-}
  curl -sS -f -b "$jar" -c "$jar" -H 'X-Requested-With: Slouchcat' -H 'Content-Type: application/json' -X "$method" \
    ${body:+--data-binary "$body"} "$base/api/v1/$path"
}

declare -A server_port library_name
idx=0
for lang in $langs; do
  lport=$((port + 2 * idx))
  idx=$((idx + 1))
  server_port[$lang]=$lport
  base="http://127.0.0.1:$lport"
  tree="$cache/lang-$lang"
  mkdir -p "$run/$lang"
  # Software encoding as on the small rented server of deploy/demo (and no GPU of this machine).
  SLOUCHCAT_HW=software SLOUCHCAT_BROWSE_ROOTS="$tree" SLOUCHCAT_TMDB_API_KEY= "$repo/server/bin/slouchcat" serve --listen "127.0.0.1:$lport" \
    --config-dir "$run/$lang/config" --cache-dir "$run/$lang/cache" --dev --log-format text --log-level info >"$run/server-$lang.log" 2>&1 &
  server_pids+=($!)
  for _ in $(seq 1 120); do
    kill -0 "${server_pids[-1]}" 2>/dev/null || fail "the $lang server stopped while starting (port $lport taken?)"
    if grep -q 'msg=listening' "$run/server-$lang.log"; then break; fi
    sleep 0.5
  done
  grep -q 'msg=listening' "$run/server-$lang.log" || fail "the $lang server reported no address within 60 s"

  case "$lang" in
    de) films=Filme books=Hörbücher guest=Gast ;;
    *) films=Films books=Audiobooks guest=Guest ;;
  esac
  library_name[$lang]=$films
  jar="$run/$lang/owner.jar"
  trickplay='{"movies":{"mode":"ahead","keep":true},"series":{"mode":"off","keep":true,"scope":"series"}}'
  api "$jar" POST setup "{\"email\":\"$email\",\"password\":\"$password\",\"language\":\"$lang\",\"profileName\":\"Demo\",\"accent\":\"ember\",\"library\":{\"name\":\"$films\",\"kind\":\"video\",\"roots\":[{\"path\":\"$tree/Movies\",\"content\":\"movie\",\"providers\":[\"tmdb\"]}],\"trickplay\":$trickplay}}" >/dev/null ||
    fail "runSetup failed on the $lang server"
  api "$jar" POST auth/login "{\"email\":\"$email\",\"password\":\"$password\"}" >/dev/null || fail "login failed on the $lang server"
  primary=$(api "$jar" GET auth/profiles | python3 -c 'import json,sys; print(next(p["id"] for p in json.load(sys.stdin) if p["isPrimary"]))')
  api "$jar" POST "auth/profiles/$primary/select" >/dev/null || fail "profile choice failed on the $lang server"
  api "$jar" POST admin/libraries "{\"name\":\"$books\",\"kind\":\"audiobooks\",\"roots\":[{\"path\":\"$tree/Audiobooks\",\"content\":\"audiobook\",\"providers\":[\"local\"]}]}" >/dev/null ||
    fail "the audiobook library failed on the $lang server"
  api "$jar" POST profiles "{\"name\":\"$guest\",\"accent\":\"teal\",\"language\":\"$lang\",\"allLibraries\":true}" >/dev/null || fail "the second profile failed on the $lang server"
  api "$jar" PUT "profiles/$primary/pin" "{\"pin\":\"$pin\"}" >/dev/null || fail "the PIN failed on the $lang server"
done
for lang in $langs; do
  base="http://127.0.0.1:${server_port[$lang]}"
  for _ in $(seq 1 600); do
    libraries=$(api "$run/$lang/owner.jar" GET admin/libraries)
    case "$libraries" in *'"state":"failed"'* | *'"state":"aborted"'*) fail "a scan failed on the $lang server: $libraries" ;; esac
    printf '%s' "$libraries" | grep -Eq '"state":"(never|queued|running)"' || break
    sleep 1
  done
  # Texts and pictures of the films, as the administration writes them.
  jar="$run/$lang/owner.jar"
  api "$jar" GET "libraries/1/items" | python3 -c 'import json, sys
for i in json.load(sys.stdin)["items"]: print(i["id"] + "\t" + i["title"])' >"$run/$lang/films.tsv"
  for dir in "$cache/media/Movies"/*/; do
    folder=$(basename "$dir")
    id=$(awk -F '\t' -v t="${folder% (*}" '$2 == t { print $1 }' "$run/$lang/films.tsv")
    [ -n "$id" ] || fail "no title $folder on the $lang server"
    body=$(python3 -c 'import json, sys; print(json.dumps({"overview": sys.argv[1], "genres": json.loads(sys.argv[2])}))' \
      "$(film_overview "$folder" "$lang")" "$(film_genres "$folder" "$lang")")
    api "$jar" PATCH "admin/items/$id/metadata" "$body" >/dev/null || fail "the texts of $folder failed ($lang)"
    for pair in poster:poster backdrop:fanart; do
      curl -sS -f -b "$jar" -H 'X-Requested-With: Slouchcat' -X PUT -F "file=@$cache/art/$folder-${pair#*:}.jpg;type=image/jpeg" \
        "$base/api/v1/admin/items/$id/images/${pair%%:*}/upload" >/dev/null || fail "the ${pair%%:*} of $folder failed ($lang)"
    done
  done
  # «Im Voraus» makes the previews of files that come after the first scan; the films of the first
  # scan get theirs from one run of the backfill (StoreShotsTest waits for the set of its film).
  api "$jar" POST admin/job-types/trickplay.backfill/run >/dev/null || fail "the trickplay backfill failed ($lang)"
  say "$lang server on port ${server_port[$lang]} is set up"
done

# ---- 3. the emulators --------------------------------------------------------------------------

boot() { # avd emu_port gpu
  local avd=$1 emu_port=$2 gpu=$3
  serial="emulator-$emu_port"
  emulator_lock "$wait_max" "$run" || fail "another run holds the emulator; one at a time on this machine"
  local waited=0
  while pgrep -f '^[^ ]*qemu-system' >/dev/null; do
    [ "$waited" -lt "$wait_max" ] || fail "another emulator runs; one at a time on this machine"
    say "another emulator runs without the lock, waiting ($waited s)"
    sleep 60
    waited=$((waited + 60))
  done
  "$sdk/emulator/emulator" -avd "$avd" -port "$emu_port" -no-window -no-audio -no-boot-anim -no-snapshot-save \
    -gpu "$gpu" >"$run/emulator-$avd.log" 2>&1 &
  emulator_pid=$!
  for _ in $(seq 1 300); do
    kill -0 "$emulator_pid" 2>/dev/null || fail "the emulator $avd stopped while booting"
    if [ "$(ADB_LIMIT=10 adb_to shell getprop sys.boot_completed 2>/dev/null | tr -d '\r')" = "1" ]; then break; fi
    sleep 1
  done
  [ "$(ADB_LIMIT=10 adb_to shell getprop sys.boot_completed 2>/dev/null | tr -d '\r')" = "1" ] || fail "the emulator $avd did not boot"
  for setting in window_animation_scale transition_animation_scale animator_duration_scale; do
    adb_to shell settings put global "$setting" 0
  done
  adb_to shell settings put system sound_effects_enabled 0
  adb_to shell settings put secure immersive_mode_confirmations confirmed
  for _ in $(seq 1 60); do adb_to shell pm list packages >/dev/null 2>&1 && break; sleep 1; done
  for _ in $(seq 1 60); do adb_to shell ping -c 1 -W 1 10.0.2.2 >/dev/null 2>&1 && break; sleep 1; done
  say "$avd ready"
}

# The status bar of a phone or tablet in its demo mode: a fixed clock, full signal and battery, no icons.
demo_bar() {
  adb_to shell settings put global sysui_demo_allowed 1
  local b=(shell am broadcast -a com.android.systemui.demo -e command)
  adb_to "${b[@]}" enter >/dev/null
  adb_to "${b[@]}" clock -e hhmm 1200 >/dev/null
  adb_to "${b[@]}" network -e wifi show -e level 4 -e fully true >/dev/null
  adb_to "${b[@]}" network -e mobile hide >/dev/null
  adb_to "${b[@]}" battery -e level 100 -e plugged false >/dev/null
  adb_to "${b[@]}" notifications -e visible false >/dev/null
}

install() { # apk...
  local apk
  adb_to uninstall "$app" >/dev/null 2>&1 || true
  adb_to uninstall "$app.test" >/dev/null 2>&1 || true
  for apk in "$@"; do ADB_LIMIT=300 adb_to install -r -t "$apk" >/dev/null || fail "install of $apk failed"; done
}

fresh_app() { # lang: the app's data gone, its language set
  local lang=$1 locale
  case "$lang" in de) locale=de-CH ;; *) locale=$lang ;; esac
  adb_to shell pm clear "$app" >/dev/null
  adb_to shell cmd locale set-app-locales "$app" --locales "$locale"
}

instrument() { # lang method-or-class...: runs the store photos of the app
  local lang=$1
  shift
  local classes
  classes=$(IFS=,; echo "$*")
  local out
  out=$(ADB_LIMIT=1800 adb_to shell am instrument -w -e slouchcatStore 1 -e slouchcatServer "http://10.0.2.2:${server_port[$lang]}" \
    -e slouchcatRelay "http://10.0.2.2:$relay_port" -e slouchcatEmail "$email" -e slouchcatPassword "$password" -e slouchcatPin "$pin" \
    -e slouchcatLibrary "${library_name[$lang]}" -e class "$classes" "$app.test/androidx.test.runner.AndroidJUnitRunner" 2>&1) || true
  printf '%s\n' "$out" >>"$run/instrument.log"
  printf '%s\n' "$out" | grep -E "^(OK|FAILURES|Tests run)" | head -n 3
  if ! printf '%s\n' "$out" | grep -q '^OK ('; then
    printf '%s\n' "$out" | grep -E "Exception|Error|at com\.slouchcat" | head -n 20 >&2
    # What the screen showed: the photos so far and those of a failed wait (FlowWaits.kt).
    mkdir -p "$target/logs"
    adb_to pull "/sdcard/Android/data/$app/files/." "$target/logs/device-files" >/dev/null 2>&1 || true
    mkdir -p "$target/logs/device-files-internal"
    adb_to exec-out run-as "$app" tar -cf - -C files . 2>/dev/null | tar -xf - -C "$target/logs/device-files-internal" 2>/dev/null || true
    fail "the photos of $classes ($lang) failed"
  fi
}

pull_phone() { # dir
  mkdir -p "$1"
  adb_to pull "/sdcard/Android/data/$app/files/store/." "$1" >/dev/null || fail "no photos to copy"
  adb_to shell rm -rf "/sdcard/Android/data/$app/files/store"
}

pull_tv() { # dir
  mkdir -p "$1"
  adb_to exec-out run-as "$app" tar -cf - -C files store | tar -xf - -C "$1" --strip-components=1 || fail "no photos to copy"
}

mobile_class=com.slouchcat.app.mobile.store.StoreShotsTest
mobile_apks=(app-mobile/build/outputs/apk/play/debug/app-mobile-play-debug.apk app-mobile/build/outputs/apk/androidTest/play/debug/app-mobile-play-debug-androidTest.apk)
tv_apks=(app-tv/build/outputs/apk/debug/app-tv-debug.apk app-tv/build/outputs/apk/androidTest/debug/app-tv-debug-androidTest.apk)
mkdir -p "$target"

for device in $devices; do
  case "$device" in
    phone)
      boot sc_phone 5580 swangle_indirect
      install "${mobile_apks[@]}"
      demo_bar
      for lang in $langs; do
        dir="$target/phone-$lang"
        rm -rf "$dir"
        fresh_app "$lang"
        instrument "$lang" "$mobile_class#phone" "$mobile_class#phoneTvLink"
        # The player in landscape: a display of 1080 × 1920, the photo 1920 × 1080.
        adb_to shell wm size 1080x1920
        instrument "$lang" "$mobile_class#player"
        adb_to shell wm size reset
        pull_phone "$dir"
      done
      ;;
    tablet)
      boot sc_tablet 5582 swangle_indirect
      install "${mobile_apks[@]}"
      demo_bar
      # The AVD is 16:10 (2560 × 1600); the store wants 16:9: the display set to 16:9, no cropping.
      for size in "tablet7 1920x1080 280" "tablet10 2560x1440 320"; do
        read -r name pixels density <<<"$size"
        adb_to shell wm size "$pixels"
        adb_to shell wm density "$density"
        demo_bar
        for lang in $langs; do
          dir="$target/$name-$lang"
          rm -rf "$dir"
          fresh_app "$lang"
          instrument "$lang" "$mobile_class#tablet" "$mobile_class#player"
          pull_phone "$dir"
          # The tablet's fourth motif: the player (named 03 by the test, the third on the phone).
          mv "$dir/03-player.png" "$dir/04-player.png"
        done
      done
      adb_to shell wm size reset
      adb_to shell wm density reset
      ;;
    tv)
      boot sc_tv 5584 swiftshader_indirect
      install "${tv_apks[@]}"
      for lang in $langs; do
        dir="$target/tv-$lang"
        rm -rf "$dir"
        fresh_app "$lang"
        instrument "$lang" com.slouchcat.app.tv.store.StoreShotsTest
        pull_tv "$dir"
      done
      ;;
    *) fail "unknown device $device (phone, tablet, tv)" ;;
  esac
  emulator_stop
done

# ---- 4. format and check -----------------------------------------------------------------------

# The screenshots carry an alpha channel; the store wants PNG with 24 bit RGB.
find "$target" -path "$target/logs" -prune -o -name '*.png' -print | while read -r file; do
  ffmpeg -nostdin -loglevel error -y -i "$file" -pix_fmt rgb24 "$file.rgb.png" && mv "$file.rgb.png" "$file"
done
python3 -I "$repo/scripts/store-screenshots-check.py" "$target"
say "done: photos in $target"
