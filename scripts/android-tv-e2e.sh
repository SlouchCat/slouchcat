#!/usr/bin/env bash
# The flow of the TV app in a TV emulator against a real server and a local relay (task
# android:e2e:tv, concept 26 stage 2a): starts bin/slouchcat with fresh data on PORT and the relay
# relay/bin/pair on RELAY_PORT, starts its own emulator AVD on EMU_PORT and runs the
# instrumentation test TvFlowTest of app-tv there: (a) set up by hand, profile with PIN, browse
# (start, a title, a library, search, watchlist, «Profile verwalten»),
# «Einstellungen», sign out; (b) set up by scan: the test hands out the content of the QR code, this
# script lets `apicheck -pair-phone` play the phone, the TV redeems and shows server and account;
# (c) the player of stage 3 with real video: play, pause, spool, sound and subtitles, leave,
# resume, an episode up to the countdown, «Wiedergabe» of the settings.
#
#   scripts/android-tv-e2e.sh [--shots DIR]
#
# PORT        port of the test server (default 8697); 127.0.0.1 only, the emulator reaches it as
#             http://10.0.2.2:PORT
# RELAY_PORT  port of the relay (default 8180), the same way
# AVD         emulator to start (default sc_tv); EMU_PORT its console port (default 5584)
# SHOTS=DIR   or --shots DIR: photograph every page and menu and copy the photos to DIR
# LOCALE      language of the app (de-CH, en …; default: the device's)
# EMU_OPTS    more options for the emulator, split at spaces (CI: -feature -Vulkan)
# WAIT_MAX    seconds to wait while another emulator runs (default 600; 0 fails at once)
# PHONE       who plays the phone: apicheck (default, Go) or kotlin (the phone app's own Kotlin of
#             :core:pairing as a JVM program, :core:pairing:phoneCheck; task android:e2e:pair)
#
# Like scripts/android-e2e.sh it only ever touches the emulator it started itself (adb -s
# emulator-EMU_PORT), waits while another emulator runs, and on exit stops that emulator, the
# server and the relay. Logs of a failure stay in app-tv/build/e2e-logs/. Run from android/ (the
# Taskfile does). Needs server/bin/slouchcat, server/bin/apicheck and relay/bin/pair.
set -euo pipefail

port=${PORT:-8697}
relay_port=${RELAY_PORT:-8180}
avd=${AVD:-sc_tv}
emu_port=${EMU_PORT:-5584}
shots=${SHOTS:-}
wait_max=${WAIT_MAX:-600}
if [ "${1:-}" = "--shots" ]; then shots=$2; fi

sdk=${ANDROID_HOME:-$HOME/.local/share/android-sdk}
serial="emulator-$emu_port"
adb() { "$sdk/platform-tools/adb" -s "$serial" "$@"; }
# For the cleanup: adb waits without end when the emulator has died, so these calls get a limit.
adb_brief() { timeout 30 "$sdk/platform-tools/adb" -s "$serial" "$@"; }
app=com.slouchcat.app.dev
root=$(mktemp -d -t slouchcat-tv-e2e-XXXXXX)
logs=app-tv/build/e2e-logs
server_pid=""
relay_pid=""
emulator_pid=""
instrument_pid=""
# The account of the test (TvFlowTest's defaults): the phone of apicheck signs in with it.
email=e2e-owner@example.test
password=e2e-password-0123
pin=246810

cleanup() {
  status=$?
  if [ -n "$instrument_pid" ]; then kill "$instrument_pid" 2>/dev/null || true; fi
  if [ "$status" -ne 0 ]; then
    mkdir -p "$logs"
    for f in server.log relay.log emulator.log instrument.txt apicheck.log; do
      [ -f "$root/$f" ] && cp "$root/$f" "$logs/"
    done
    if [ -n "$emulator_pid" ]; then adb_brief logcat -d >"$logs/logcat.txt" 2>/dev/null || true; fi
  fi
  for pid in "$server_pid" "$relay_pid"; do
    if [ -n "$pid" ]; then kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true; fi
  done
  if [ -n "$emulator_pid" ]; then
    adb_brief emu kill >/dev/null 2>&1 || true
    for _ in $(seq 1 20); do kill -0 "$emulator_pid" 2>/dev/null || break; sleep 1; done
    kill "$emulator_pid" 2>/dev/null || true
  fi
  rm -rf "$root"
}
trap cleanup EXIT

fail() {
  echo "android-tv-e2e: $1" >&2
  for f in server.log relay.log apicheck.log; do
    if [ -f "$root/$f" ]; then
      echo "--- $f, last 30 lines ---" >&2
      tail -n 30 "$root/$f" >&2
    fi
  done
  exit 1
}

started=$(date +%s)

# The server with fresh data and the relay, both on the loopback only. The small media tree of
# scripts/android-test-media.sh is the only browsable root: TvFlowTest adds it as a library and
# browses it (stage 2b).
mkdir -p "$root/media"
../scripts/android-test-media.sh "$root/media"
# Real video for the player of stage 3 (a film, a series of two episodes; needs ffmpeg).
../scripts/android-test-video.sh "$root/media"
SLOUCHCAT_BROWSE_ROOTS="$root/media" SLOUCHCAT_TMDB_API_KEY= ../server/bin/slouchcat serve \
  --listen "127.0.0.1:$port" \
  --config-dir "$root/config" --cache-dir "$root/cache" \
  --dev --log-format text --log-level info >"$root/server.log" 2>&1 &
server_pid=$!
RELAY_LISTEN="127.0.0.1:$relay_port" ../relay/bin/pair >"$root/relay.log" 2>&1 &
relay_pid=$!
for _ in $(seq 1 120); do
  kill -0 "$server_pid" 2>/dev/null || fail "the server stopped while starting (port $port taken?)"
  kill -0 "$relay_pid" 2>/dev/null || fail "the relay stopped while starting (port $relay_port taken?)"
  if grep -q 'msg=listening' "$root/server.log" && curl -fsS "http://127.0.0.1:$relay_port/v1/healthz" >/dev/null 2>&1; then break; fi
  sleep 0.5
done
grep -q 'msg=listening' "$root/server.log" || fail "the server reported no address within 60 s"

# One emulator at a time: wait while another one runs (another agent's).
waited=0
while pgrep -f '^[^ ]*qemu-system' >/dev/null; do
  [ "$waited" -lt "$wait_max" ] || fail "another emulator runs; one at a time on this machine"
  echo "android-tv-e2e: another emulator runs, waiting ($waited s)"
  sleep 60
  waited=$((waited + 60))
done

boot() {
  "$sdk/emulator/emulator" -avd "$avd" -port "$emu_port" -no-window -no-audio -no-boot-anim -no-snapshot-save \
    -gpu swiftshader_indirect ${EMU_OPTS:-} >"$root/emulator.log" 2>&1 &
  emulator_pid=$!
  for _ in $(seq 1 300); do
    kill -0 "$emulator_pid" 2>/dev/null || return 1
    if [ "$(adb shell getprop sys.boot_completed 2>/dev/null | tr -d '\r')" = "1" ]; then return 0; fi
    sleep 1
  done
  return 1
}
if ! boot; then
  echo "android-tv-e2e: the emulator did not boot, trying once more"
  adb_brief emu kill >/dev/null 2>&1 || true
  kill "$emulator_pid" 2>/dev/null || true
  wait "$emulator_pid" 2>/dev/null || true
  boot || fail "the emulator did not boot"
fi
for setting in window_animation_scale transition_animation_scale animator_duration_scale; do
  adb shell settings put global "$setting" 0
done
booted=$(date +%s)
echo "android-tv-e2e: $serial ready after $((booted - started)) s"

for _ in $(seq 1 60); do
  if adb shell pm list packages >/dev/null 2>&1; then break; fi
  sleep 1
done
adb uninstall "$app" >/dev/null 2>&1 || true
adb uninstall "$app.test" >/dev/null 2>&1 || true
for _ in $(seq 1 60); do
  if adb shell ping -c 1 -W 1 10.0.2.2 >/dev/null 2>&1; then break; fi
  sleep 1
done

../scripts/heavy-slot.sh ./gradlew --console=plain -q :app-tv:assembleDebug :app-tv:assembleDebugAndroidTest
# PHONE=kotlin: the phone's program compiled before the TV shows its code (the code lives 5 minutes).
if [ "${PHONE:-apicheck}" = kotlin ]; then ./gradlew --console=plain -q :core:pairing:testClasses; fi
adb install -r -t app-tv/build/outputs/apk/debug/app-tv-debug.apk >/dev/null
adb install -r -t app-tv/build/outputs/apk/androidTest/debug/app-tv-debug-androidTest.apk >/dev/null
if [ -n "${LOCALE:-}" ]; then adb shell cmd locale set-app-locales "$app" --locales "$LOCALE"; fi

extra=()
if [ -n "$shots" ]; then extra=(-e slouchcatShots 1); fi
adb shell am instrument -w -r -e slouchcatServer "http://10.0.2.2:$port" -e slouchcatRelay "http://10.0.2.2:$relay_port" \
  -e slouchcatEmail "$email" -e slouchcatPassword "$password" -e slouchcatPin "$pin" -e slouchcatMedia "$root/media" "${extra[@]}" \
  "$app.test/androidx.test.runner.AndroidJUnitRunner" >"$root/instrument.txt" 2>&1 &
instrument_pid=$!

# The photos of the run, also after a failure: they show where it stopped. They lie in the app's
# own files (debug build: run-as reads them).
photos() {
  [ -n "$shots" ] || return 0
  rm -f "$shots"/*.png
  mkdir -p "$shots"
  adb exec-out run-as "$app" tar -cf - -C files shots | tar -xf - -C "$shots" --strip-components=1 || true
  echo "android-tv-e2e: photos in $shots"
}

# The phone: as soon as the TV shows its code, apicheck lays the packet down at the relay.
code=""
for _ in $(seq 1 600); do
  code=$(sed -n 's/^INSTRUMENTATION_STATUS: pairCode=//p' "$root/instrument.txt" | head -n 1 | tr -d '\r')
  [ -n "$code" ] && break
  kill -0 "$instrument_pid" 2>/dev/null || break
  sleep 0.5
done
if [ -z "$code" ]; then
  wait "$instrument_pid" || true
  instrument_pid=""
  grep -E "^(OK|FAILURES|Tests run)|Exception|Error|at com\.slouchcat" "$root/instrument.txt" | head -n 40 >&2
  photos
  fail "the TV showed no code"
fi
if [ "${PHONE:-apicheck}" = kotlin ]; then
  # The phone app's own Kotlin (:core:pairing) as a JVM program (task android:e2e:pair).
  ./gradlew --console=plain -q :core:pairing:phoneCheck --args="server=http://127.0.0.1:$port packetServer=http://10.0.2.2:$port \
relay=http://127.0.0.1:$relay_port codeRelay=http://10.0.2.2:$relay_port code=$code email=$email password=$password pin=$pin" \
    >"$root/apicheck.log" 2>&1 || { cat "$root/apicheck.log" >&2; fail "the phone's Kotlin could not pair"; }
  grep '^phonecheck:' "$root/apicheck.log"
else
  ../server/bin/apicheck -base "http://127.0.0.1:$port" -spec ../api/openapi.yaml -pair-phone "$code" \
    -relay "http://127.0.0.1:$relay_port" -pair-server "http://10.0.2.2:$port" \
    -pair-email "$email" -pair-password "$password" -pair-pin "$pin" >"$root/apicheck.log" 2>&1 || fail "apicheck could not play the phone"
  grep '^apicheck:' "$root/apicheck.log"
fi

wait "$instrument_pid" || true
instrument_pid=""
grep -E "^(OK|FAILURES|Tests run)|Exception|Error|at com\.slouchcat" "$root/instrument.txt" | head -n 40
photos
grep -q '^OK (' "$root/instrument.txt" || fail "the flow failed"
echo "android-tv-e2e: done after $(($(date +%s) - started)) s ($(($(date +%s) - booted)) s with the device)"
