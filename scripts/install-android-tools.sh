#!/usr/bin/env bash
# Installs what the Android build needs, without root, under ~/.local/share (docs/dev/SETUP.md,
# section Android): Temurin JDK 21 (checksum checked), the Android command line tools (checksum
# checked), the SDK packages of the build (compileSdk 37.2, build tools 37.0.0) and the four
# emulators the pages are checked on: sc_phone (API 36), sc_tablet (API 36), sc_tv (API 34),
# sc_phone28 (API 28, minSdk). Installing the SDK asks once to accept Google's SDK licence.
# Run it again any time; what is there stays. PACKAGES="…" installs other packages instead.
set -euo pipefail

JDK_ROOT="$HOME/.local/share/jdk"
SDK_ROOT="$HOME/.local/share/android-sdk"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# The JDK pinned in .tool-versions (`java temurin-21.0.13.1+2`), not whatever is newest.
here="$(cd "$(dirname "$0")/.." && pwd)"
pinned="$(sed -n 's/^java temurin-//p' "$here/.tool-versions")"
[ -n "$pinned" ] || { echo "no java line in .tool-versions" >&2; exit 1; }

if [ ! -x "$JDK_ROOT/current/bin/java" ]; then
  release="jdk-$pinned"
  meta="$(curl -fsSL "https://api.adoptium.net/v3/assets/release_name/eclipse/${release//+/%2B}?os=linux&architecture=x64&image_type=jdk&heap_size=normal&project=jdk")"
  url="$(printf '%s' "$meta" | python3 -c 'import json,sys; print(json.load(sys.stdin)["binaries"][0]["package"]["link"])')"
  sum="$(printf '%s' "$meta" | python3 -c 'import json,sys; print(json.load(sys.stdin)["binaries"][0]["package"]["checksum"])')"
  name="$(printf '%s' "$meta" | python3 -c 'import json,sys; print(json.load(sys.stdin)["release_name"])')"
  echo "JDK: $name"
  curl -fsSL -o "$TMP/jdk.tar.gz" "$url"
  # Two sources: the checksum of the API and the one published next to the archive on GitHub.
  published="$(curl -fsSL "$url.sha256.txt" | cut -d' ' -f1)"
  [ "$sum" = "$published" ] || { echo "JDK checksums differ: API $sum, release $published" >&2; exit 1; }
  echo "$sum  $TMP/jdk.tar.gz" | sha256sum -c -
  mkdir -p "$JDK_ROOT"
  tar -xzf "$TMP/jdk.tar.gz" -C "$JDK_ROOT"
  ln -sfn "$JDK_ROOT/$name" "$JDK_ROOT/current"
fi
export JAVA_HOME="$JDK_ROOT/current"
export PATH="$JAVA_HOME/bin:$PATH"
java -version 2>&1 | head -1

if [ ! -x "$SDK_ROOT/cmdline-tools/latest/bin/sdkmanager" ]; then
  xml="$(curl -fsSL https://dl.google.com/android/repository/repository2-3.xml)"
  zip="$(printf '%s' "$xml" | grep -o 'commandlinetools-linux-[0-9]*_latest.zip' | sort -t- -k3 -n -u | tail -1)"
  sha="$(printf '%s' "$xml" | python3 -c '
import re,sys
x=sys.stdin.read(); z=sys.argv[1]
i=x.index(z); seg=x[max(0,i-600):i]
print(re.findall(r"<checksum[^>]*>([0-9a-f]{40})</checksum>", seg)[-1])' "$zip")"
  echo "cmdline-tools: $zip"
  curl -fsSL -o "$TMP/cmdline.zip" "https://dl.google.com/android/repository/$zip"
  echo "$sha  $TMP/cmdline.zip" | sha1sum -c -
  mkdir -p "$SDK_ROOT/cmdline-tools"
  unzip -q "$TMP/cmdline.zip" -d "$TMP/cl"
  rm -rf "$SDK_ROOT/cmdline-tools/latest"
  mv "$TMP/cl/cmdline-tools" "$SDK_ROOT/cmdline-tools/latest"
fi
export ANDROID_HOME="$SDK_ROOT"
SDKM="$SDK_ROOT/cmdline-tools/latest/bin/sdkmanager"

ANDROID="$SDK_ROOT/cmdline-tools/latest/bin/android"
# `sdkmanager` is deprecated; `android sdk install` takes the package paths with slashes.
PACKAGES="${PACKAGES:-platform-tools emulator platforms/android-37.2 platforms/android-36 platforms/android-34 build-tools/37.0.0 \
  system-images/android-36/google_apis/x86_64 system-images/android-36/google_apis_tablet/x86_64 \
  system-images/android-34/google-tv/x86 system-images/android-28/google_apis/x86_64}"
# shellcheck disable=SC2086
"$ANDROID" --sdk="$SDK_ROOT" sdk install $PACKAGES

AVD="$SDK_ROOT/cmdline-tools/latest/bin/avdmanager"
mk() { echo no | "$AVD" create avd -f -n "$1" -k "$2" -d "$3" >/dev/null; }
mk sc_phone "system-images;android-36;google_apis;x86_64" pixel_9
mk sc_tablet "system-images;android-36;google_apis_tablet;x86_64" pixel_tablet
mk sc_tv "system-images;android-34;google-tv;x86" tv_1080p
mk sc_phone28 "system-images;android-28;google_apis;x86_64" pixel_9
"$SDK_ROOT/emulator/emulator" -list-avds
