#!/usr/bin/env bash
# The flow of the phone app in an emulator against a real server (task android:e2e, concept 27 12
# «Gerät»): starts bin/slouchcat with fresh data on PORT (0.0.0.0, the emulator reaches it as
# http://10.0.2.2:PORT), uses the running device or starts the AVD, and runs the instrumentation
# test SignInFlowTest of app-mobile.
#
#   scripts/android-e2e.sh [--shots DIR]
#
# PORT        port of the test server (default 8597)
# AVD         emulator to start when no device is connected (default sc_phone); CI brings its own
# SHOTS=DIR   or --shots DIR: run through adb instead of Gradle, photograph every page and sheet
#             (the test's uiAutomation screenshots) and copy them to DIR
# LOCALE      language of the app for the photos (de-CH, en …; default: the device's)
#
# Memory: one emulator at a time on a shared machine. When no device is connected and another
# emulator runs (someone else's), the script stops instead of starting a second one.
# Run from android/ (the Taskfile does). Needs server/bin/slouchcat (task server:build).
set -euo pipefail

port=${PORT:-8597}
avd=${AVD:-sc_phone}
shots=${SHOTS:-}
if [ "${1:-}" = "--shots" ]; then shots=$2; fi

sdk=${ANDROID_HOME:-$HOME/.local/share/android-sdk}
adb="$sdk/platform-tools/adb"
app=com.slouchcat.app.dev
root=$(mktemp -d -t slouchcat-e2e-XXXXXX)
server_pid=""
started_emulator=""

cleanup() {
  if [ -n "$server_pid" ]; then kill "$server_pid" 2>/dev/null || true; wait "$server_pid" 2>/dev/null || true; fi
  if [ -n "$started_emulator" ]; then "$adb" -s "$started_emulator" emu kill >/dev/null 2>&1 || true; fi
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

# The server: fresh data, no TMDB key, reachable from the emulator.
SLOUCHCAT_TMDB_API_KEY= ../server/bin/slouchcat serve \
  --listen "0.0.0.0:$port" \
  --config-dir "$root/config" --cache-dir "$root/cache" \
  --dev --log-format text --log-level info >"$root/server.log" 2>&1 &
server_pid=$!
for _ in $(seq 1 120); do
  kill -0 "$server_pid" 2>/dev/null || fail "the server stopped while starting (port $port taken?)"
  if grep -q 'msg=listening' "$root/server.log"; then break; fi
  sleep 0.5
done
grep -q 'msg=listening' "$root/server.log" || fail "the server reported no address within 60 s"

# The device: the one connected, else the AVD.
device=$("$adb" devices | awk 'NR > 1 && $2 == "device" { print $1; exit }')
if [ -z "$device" ]; then
  if pgrep -f '^[^ ]*qemu-system' >/dev/null; then
    fail "no device connected, but an emulator runs (another agent's?); one emulator at a time"
  fi
  "$sdk/emulator/emulator" -avd "$avd" -port 5580 -no-window -no-audio -no-boot-anim -no-snapshot-save \
    -gpu swiftshader_indirect >"$root/emulator.log" 2>&1 &
  device=emulator-5580
  started_emulator=$device
  "$adb" -s "$device" wait-for-device
  for _ in $(seq 1 180); do
    if [ "$("$adb" -s "$device" shell getprop sys.boot_completed 2>/dev/null | tr -d '\r')" = "1" ]; then break; fi
    sleep 1
  done
  [ "$("$adb" -s "$device" shell getprop sys.boot_completed | tr -d '\r')" = "1" ] || fail "the emulator did not boot within 3 min"
  # No animations: the test waits for states, the animations only cost time.
  for setting in window_animation_scale transition_animation_scale animator_duration_scale; do
    "$adb" -s "$device" shell settings put global "$setting" 0
  done
fi
export ANDROID_SERIAL=$device
booted=$(date +%s)
echo "android-e2e: device $device ready after $((booted - started)) s"

# A fresh app: the flow starts at the server address. The AVD keeps its data between boots, so an
# app of an earlier run would start signed in; right after the boot the package manager may not
# answer yet, hence the wait.
for _ in $(seq 1 60); do
  if "$adb" shell pm list packages >/dev/null 2>&1; then break; fi
  sleep 1
done
"$adb" uninstall "$app" >/dev/null 2>&1 || true
"$adb" uninstall "$app.test" >/dev/null 2>&1 || true

server_url="http://10.0.2.2:$port"
if [ -z "$shots" ]; then
  ../scripts/heavy-slot.sh ./gradlew --console=plain :app-mobile:connectedPlayDebugAndroidTest \
    "-Pandroid.testInstrumentationRunnerArguments.slouchcatServer=$server_url" || fail "the flow failed"
else
  ../scripts/heavy-slot.sh ./gradlew --console=plain -q :app-mobile:assemblePlayDebug :app-mobile:assemblePlayDebugAndroidTest
  "$adb" install -r -t app-mobile/build/outputs/apk/play/debug/app-mobile-play-debug.apk >/dev/null
  "$adb" install -r -t app-mobile/build/outputs/apk/androidTest/play/debug/app-mobile-play-debug-androidTest.apk >/dev/null
  if [ -n "${LOCALE:-}" ]; then "$adb" shell cmd locale set-app-locales "$app" --locales "$LOCALE"; fi
  out=$("$adb" shell am instrument -w -e slouchcatServer "$server_url" -e slouchcatShots 1 \
    "$app.test/androidx.test.runner.AndroidJUnitRunner")
  echo "$out" | grep -E "^(OK|FAILURES|Tests run)|Exception|Error" | head -n 12
  echo "$out" | grep -q '^OK (' || fail "the flow failed"
  mkdir -p "$shots"
  "$adb" pull "/sdcard/Android/data/$app/files/shots/." "$shots" >/dev/null
  echo "android-e2e: photos in $shots"
fi
echo "android-e2e: done after $(($(date +%s) - started)) s ($(($(date +%s) - booted)) s with the device)"
