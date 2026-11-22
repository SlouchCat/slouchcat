#!/usr/bin/env bash
# The flow of the phone app in an emulator against a real server (task android:e2e, concept 27 12
# «Gerät»): starts bin/slouchcat with fresh data on PORT, starts its own emulator AVD on EMU_PORT
# and runs the instrumentation tests of app-mobile there (SignInFlowTest, DetailFlowTest, PlayerFlowTest).
#
#   scripts/android-e2e.sh [--shots DIR]
#
# PORT        port of the test server (default 8597); it listens on 127.0.0.1 only, the emulator
#             reaches the host's loopback as http://10.0.2.2:PORT
# AVD         emulator to start (default sc_phone); EMU_PORT its console port (default 5580)
# EMU_OPTS    more options for the emulator, split at spaces
# SHOTS=DIR   or --shots DIR: run through adb instead of Gradle, photograph every page and sheet
#             (the test's uiAutomation screenshots) and copy them to DIR
# LOCALE      language of the app for the photos (de-CH, en …; default: the device's)
# CLASS       only this test class (com.slouchcat.app.mobile.PlayerFlowTest), else all
# WAIT_MAX    seconds to wait while another emulator runs (default 600; 0 fails at once)
#
# The script only ever touches the emulator it started itself (adb -s emulator-EMU_PORT): never
# another agent's emulator and never a phone on USB. On exit it stops that emulator and the
# server, nothing else. One emulator at a time on a shared machine: while another one runs, it
# waits. On a failure the logs (logcat, server, emulator) stay in app-mobile/build/e2e-logs/.
# Run from android/ (the Taskfile does). Needs server/bin/slouchcat (task server:build).
set -euo pipefail

port=${PORT:-8597}
avd=${AVD:-sc_phone}
emu_port=${EMU_PORT:-5580}
shots=${SHOTS:-}
only=${CLASS:-}
wait_max=${WAIT_MAX:-600}
if [ "${1:-}" = "--shots" ]; then shots=$2; fi

sdk=${ANDROID_HOME:-$HOME/.local/share/android-sdk}
serial="emulator-$emu_port"
adb() { "$sdk/platform-tools/adb" -s "$serial" "$@"; }
# For the cleanup: adb waits without end when the emulator has died, so these calls get a limit.
adb_brief() { timeout 30 "$sdk/platform-tools/adb" -s "$serial" "$@"; }
app=com.slouchcat.app.dev
root=$(mktemp -d -t slouchcat-e2e-XXXXXX)
logs=app-mobile/build/e2e-logs
server_pid=""
emulator_pid=""

cleanup() {
  status=$?
  if [ "$status" -ne 0 ]; then
    mkdir -p "$logs"
    [ -f "$root/server.log" ] && cp "$root/server.log" "$logs/"
    [ -f "$root/emulator.log" ] && cp "$root/emulator.log" "$logs/"
    if [ -n "$emulator_pid" ]; then
      adb_brief logcat -d >"$logs/logcat.txt" 2>/dev/null || true
      # The photos of a failed wait (FlowWaits.kt: semantics tree in the message, photo here).
      adb_brief pull "/sdcard/Android/data/$app/files/failures" "$logs/" >/dev/null 2>&1 || true
    fi
  fi
  if [ -n "$server_pid" ]; then kill "$server_pid" 2>/dev/null || true; wait "$server_pid" 2>/dev/null || true; fi
  if [ -n "$emulator_pid" ]; then
    adb_brief emu kill >/dev/null 2>&1 || true
    # Also before the emulator reached adb: the process itself (code review minor 31).
    for _ in $(seq 1 20); do kill -0 "$emulator_pid" 2>/dev/null || break; sleep 1; done
    kill "$emulator_pid" 2>/dev/null || true
  fi
  rm -rf "$root"
}
trap cleanup EXIT

fail() {
  echo "android-e2e: $1" >&2
  if [ -f "$root/server.log" ]; then
    echo "--- server log, last 40 lines ---" >&2
    tail -n 40 "$root/server.log" >&2
  fi
  exit 1
}

started=$(date +%s)

# The server: fresh data, no TMDB key, on the loopback only (code review minor 30). Browsable roots:
# the small media tree of scripts/android-test-media.sh (DetailFlowTest), real video for the
# player's flow (PlayerFlowTest, needs ffmpeg) and, when `task testdata:media` made them, the
# synthetic media that apicheck seeds (stage 4 browses a library).
mkdir -p "$root/media"
../scripts/android-test-media.sh "$root/media"
../scripts/android-test-video.sh "$root/media"
media=""
if [ -d ../testdata/media/Movies ]; then media=$(cd ../testdata/media && pwd); fi
SLOUCHCAT_BROWSE_ROOTS="$root/media${media:+,$media}" SLOUCHCAT_TMDB_API_KEY= ../server/bin/slouchcat serve \
  --listen "127.0.0.1:$port" \
  --config-dir "$root/config" --cache-dir "$root/cache" \
  --dev --log-format text --log-level info >"$root/server.log" 2>&1 &
server_pid=$!
for _ in $(seq 1 120); do
  kill -0 "$server_pid" 2>/dev/null || fail "the server stopped while starting (port $port taken?)"
  if grep -q 'msg=listening' "$root/server.log"; then break; fi
  sleep 0.5
done
grep -q 'msg=listening' "$root/server.log" || fail "the server reported no address within 60 s"

# With media: apicheck sets the server up as its fixture (account, primary profile with PIN,
# films, audiobooks, podcasts, scanned); the flow signs in with that account. Without media
# (CI) the flow sets up its own account and sees the empty start page.
flow_args=()
if [ -n "$media" ]; then
  (cd ../server && go build -o bin/apicheck ./cmd/apicheck)
  ../server/bin/apicheck -base "http://127.0.0.1:$port" -spec ../api/openapi.yaml -testdata ../testdata -devices \
    >"$root/apicheck.log" 2>&1 || { cat "$root/apicheck.log" >&2; fail "apicheck could not seed the server"; }
  flow_args=(slouchcatEmail apicheck-owner@slouchcat.test slouchcatPassword apicheck-password-0123)
  echo "android-e2e: server seeded with the test media"
fi

# One emulator at a time: wait while another one runs (another agent's).
waited=0
while pgrep -f '^[^ ]*qemu-system' >/dev/null; do
  [ "$waited" -lt "$wait_max" ] || fail "another emulator runs; one at a time on this machine"
  echo "android-e2e: another emulator runs, waiting ($waited s)"
  sleep 60
  waited=$((waited + 60))
done

boot() {
  # EMU_OPTS: more options for the emulator, split at spaces (for example -feature -Vulkan).
  # shellcheck disable=SC2086
  "$sdk/emulator/emulator" -avd "$avd" -port "$emu_port" -no-window -no-audio -no-boot-anim -no-snapshot-save \
    -gpu swiftshader_indirect ${EMU_OPTS:-} >"$root/emulator.log" 2>&1 &
  emulator_pid=$!
  for _ in $(seq 1 240); do
    kill -0 "$emulator_pid" 2>/dev/null || return 1
    if [ "$(adb shell getprop sys.boot_completed 2>/dev/null | tr -d '\r')" = "1" ]; then return 0; fi
    sleep 1
  done
  return 1
}
if ! boot; then
  # One more try: a cold boot on a busy runner sometimes hangs (code review minor 28).
  echo "android-e2e: the emulator did not boot, trying once more"
  adb emu kill >/dev/null 2>&1 || true
  kill "$emulator_pid" 2>/dev/null || true
  wait "$emulator_pid" 2>/dev/null || true
  boot || fail "the emulator did not boot"
fi
# No animations: the test waits for states, the animations only cost time.
for setting in window_animation_scale transition_animation_scale animator_duration_scale; do
  adb shell settings put global "$setting" 0
done
# No touch sounds: the flow needs none, and on the hosted runner every write of the guest's audio
# device failed with an I/O error, the last lines of the log before the emulator died there.
adb shell settings put system sound_effects_enabled 0
# The player hides the system bars: no system hint «Viewing full screen» over the photos.
adb shell settings put secure immersive_mode_confirmations confirmed
export ANDROID_SERIAL=$serial
booted=$(date +%s)
echo "android-e2e: $serial ready after $((booted - started)) s"

# A fresh app on the own emulator: the flow starts at the server address. The AVD keeps its data
# between boots; right after the boot the package manager may not answer yet.
for _ in $(seq 1 60); do
  if adb shell pm list packages >/dev/null 2>&1; then break; fi
  sleep 1
done
adb uninstall "$app" >/dev/null 2>&1 || true
adb uninstall "$app.test" >/dev/null 2>&1 || true
# Photos of an earlier run belong to the uid of the old install: the new one could not write there.
adb shell rm -rf "/sdcard/Android/data/$app" >/dev/null 2>&1 || true
# The network of the emulator comes up after the boot: without it the first request fails
# with ENETUNREACH.
for _ in $(seq 1 60); do
  if adb shell ping -c 1 -W 1 10.0.2.2 >/dev/null 2>&1; then break; fi
  sleep 1
done

server_url="http://10.0.2.2:$port"
if [ -z "$shots" ]; then
  gradle_args=()
  for ((i = 0; i < ${#flow_args[@]}; i += 2)); do
    gradle_args+=("-Pandroid.testInstrumentationRunnerArguments.${flow_args[i]}=${flow_args[i + 1]}")
  done
  ../scripts/heavy-slot.sh ./gradlew --console=plain :app-mobile:connectedPlayDebugAndroidTest \
    "-Pandroid.testInstrumentationRunnerArguments.slouchcatServer=$server_url" \
    "-Pandroid.testInstrumentationRunnerArguments.slouchcatMedia=$root/media" "${gradle_args[@]}" \
    ${only:+"-Pandroid.testInstrumentationRunnerArguments.class=$only"} || fail "the flow failed"
else
  ../scripts/heavy-slot.sh ./gradlew --console=plain -q :app-mobile:assemblePlayDebug :app-mobile:assemblePlayDebugAndroidTest
  adb install -r -t app-mobile/build/outputs/apk/play/debug/app-mobile-play-debug.apk >/dev/null
  adb install -r -t app-mobile/build/outputs/apk/androidTest/play/debug/app-mobile-play-debug-androidTest.apk >/dev/null
  if [ -n "${LOCALE:-}" ]; then adb shell cmd locale set-app-locales "$app" --locales "$LOCALE"; fi
  am_args=()
  for ((i = 0; i < ${#flow_args[@]}; i += 2)); do am_args+=(-e "${flow_args[i]}" "${flow_args[i + 1]}"); done
  out=$(adb shell am instrument -w -e slouchcatServer "$server_url" -e slouchcatMedia "$root/media" -e slouchcatShots 1 "${am_args[@]}" ${only:+-e class "$only"} \
    "$app.test/androidx.test.runner.AndroidJUnitRunner")
  echo "$out" | grep -E "^(OK|FAILURES|Tests run)|Exception|Error" | head -n 12
  echo "$out" | grep -q '^OK (' || fail "the flow failed"
  rm -f "$shots"/*.png "$shots"/*.txt
  mkdir -p "$shots"
  adb pull "/sdcard/Android/data/$app/files/shots/." "$shots" >/dev/null 2>&1
  echo "android-e2e: photos in $shots"
fi
echo "android-e2e: done after $(($(date +%s) - started)) s ($(($(date +%s) - booted)) s with the device)"
