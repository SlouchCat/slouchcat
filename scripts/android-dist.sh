#!/usr/bin/env bash
# The files of an Android release in one folder (task android:dist): builds the APKs and bundles of
# the apps, copies them to android/dist/ under names with the version, checks them and writes
# SHA256SUMS.
#
#   scripts/android-dist.sh            both apps
#   APPS=app scripts/android-dist.sh   only the phone app (tag app-v…)
#   APPS=tv scripts/android-dist.sh    only the TV app (tag tv-v…)
#
# Per app: debug APKs to install with `adb install` (signed with the debug key, application id with
# `.dev`), the release APKs and the bundles (AAB; `foss` has none, it never goes to the store),
# signed with the upload key when the four slouchcat.signing.* values are set (build-logic,
# Signing.kt), else unsigned and named so. Checks: every debug APK verifies with apksigner and its
# badging names the package, the version of version.properties and an icon; the TV APKs carry the
# leanback launcher and the banner; a release APK named signed verifies as well.
#
# JAVA_HOME and ANDROID_HOME as in android/Taskfile.yml. Gradle waits for a heavy slot.
set -euo pipefail

android=$(cd "$(dirname "$0")/../android" && pwd)
apps=${APPS:-app tv}
dist="$android/dist"
sdk=${ANDROID_HOME:-$HOME/.local/share/android-sdk}
tools=$(find "$sdk/build-tools" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' 2>/dev/null | sort -V | tail -n 1)
if [ -z "$tools" ]; then
  echo "no build-tools under $sdk/build-tools (scripts/install-android-tools.sh)" >&2
  exit 1
fi
# apksigner is a Java program: the JDK of the build first on PATH.
if [ -n "${JAVA_HOME:-}" ]; then PATH="$JAVA_HOME/bin:$PATH"; fi
apksigner="$sdk/build-tools/$tools/apksigner"
aapt2="$sdk/build-tools/$tools/aapt2"

version() { # <module> <key>
  sed -n "s/^$2=//p" "$android/$1/version.properties" | tr -d '[:space:]'
}

tasks=()
for app in $apps; do
  case "$app" in
    app) tasks+=(:app-mobile:assemblePlayDebug :app-mobile:assembleFossDebug :app-mobile:assemblePlayRelease
      :app-mobile:assembleFossRelease :app-mobile:bundlePlayRelease) ;;
    tv) tasks+=(:app-tv:assembleDebug :app-tv:assembleRelease :app-tv:bundleRelease) ;;
    *) echo "APPS: unknown app $app (app, tv)" >&2; exit 2 ;;
  esac
done

cd "$android"
# A release APK of an earlier run with or without the key would sit beside the new one under the
# other name; Gradle writes the release outputs again.
rm -rf app-mobile/build/outputs/apk/play/release app-mobile/build/outputs/apk/foss/release \
  app-mobile/build/outputs/bundle app-tv/build/outputs/apk/release app-tv/build/outputs/bundle
../scripts/heavy-slot.sh ./gradlew --console=plain "${tasks[@]}"

rm -rf "$dist"
mkdir -p "$dist"

# The one file of a Gradle output folder; Gradle names a release APK "-unsigned" without a key.
one() { # <dir> <pattern>
  local found
  found=$(find "$1" -maxdepth 1 -name "$2" -print)
  if [ "$(printf '%s\n' "$found" | grep -c .)" -ne 1 ]; then
    echo "expected one $2 in $1, found: ${found:-none}" >&2
    exit 1
  fi
  printf '%s\n' "$found"
}

# Signed or not, by the file itself: apksigner for an APK, a signature block in META-INF for a bundle.
signed_apk() { "$apksigner" verify "$1" >/dev/null 2>&1; }
signed_aab() { unzip -Z1 "$1" | grep -qE '^META-INF/[^/]+\.(RSA|EC|DSA)$'; }

copy() { # <source> <name without suffix> <ext> <signed: yes|no>
  local target="$2"
  [ "$4" = yes ] || target="$target-unsigned"
  cp "$1" "$dist/$target.$3"
  echo "$target.$3"
}

badging() { "$aapt2" dump badging "$1"; }

# A debug APK: installable (signature), package, version and icon as expected.
check_debug() { # <apk> <package> <versionName> <versionCode> <tv: yes|no>
  local apk="$dist/$1" out
  "$apksigner" verify "$apk" >/dev/null || { echo "$1: apksigner verify failed" >&2; exit 1; }
  out=$(badging "$apk")
  grep -q "^package: name='$2' versionCode='$4' versionName='$3'" <<<"$out" \
    || { echo "$1: badging names another package or version: $(grep '^package:' <<<"$out")" >&2; exit 1; }
  grep -qE "^application: .*icon='[^']+'" <<<"$out" || { echo "$1: no application icon" >&2; exit 1; }
  if [ "$5" = yes ]; then
    grep -q "^leanback-launchable-activity:" <<<"$out" || { echo "$1: no leanback launcher" >&2; exit 1; }
    grep -qE "^application: .*banner='[^']+'" <<<"$out" || { echo "$1: no banner" >&2; exit 1; }
  else
    grep -q "^launchable-activity:" <<<"$out" || { echo "$1: no launcher activity" >&2; exit 1; }
  fi
  echo "checked $1: $(grep '^package:' <<<"$out" | cut -d' ' -f2-4)"
}

for app in $apps; do
  if [ "$app" = app ]; then
    name=$(version app-mobile versionName)
    code=$(version app-mobile versionCode)
    out=app-mobile/build/outputs
    base="slouchcat-app-$name"
    for flavour in play foss; do
      file=$(copy "$(one "$out/apk/$flavour/debug" "*.apk")" "$base-$flavour-debug" apk yes)
      check_debug "$file" "com.slouchcat.app$([ "$flavour" = foss ] && echo .foss).dev" "$name" "$code" no
      release=$(one "$out/apk/$flavour/release" "*.apk")
      if signed_apk "$release"; then copy "$release" "$base-$flavour-release" apk yes >/dev/null
      else copy "$release" "$base-$flavour-release" apk no >/dev/null; fi
    done
    bundle=$(one "$out/bundle/playRelease" "*.aab")
    if signed_aab "$bundle"; then copy "$bundle" "$base-play-release" aab yes >/dev/null
    else copy "$bundle" "$base-play-release" aab no >/dev/null; fi
  else
    name=$(version app-tv versionName)
    code=$(version app-tv versionCode)
    out=app-tv/build/outputs
    base="slouchcat-tv-$name"
    file=$(copy "$(one "$out/apk/debug" "*.apk")" "$base-debug" apk yes)
    check_debug "$file" com.slouchcat.app.dev "$name" "$code" yes
    release=$(one "$out/apk/release" "*.apk")
    if signed_apk "$release"; then copy "$release" "$base-release" apk yes >/dev/null
    else copy "$release" "$base-release" apk no >/dev/null; fi
    bundle=$(one "$out/bundle/release" "*.aab")
    if signed_aab "$bundle"; then copy "$bundle" "$base-release" aab yes >/dev/null
    else copy "$bundle" "$base-release" aab no >/dev/null; fi
  fi
done

cd "$dist"
sha256sum -- *.apk *.aab >SHA256SUMS
echo "android/dist:"
ls -l -- *.apk *.aab SHA256SUMS
cat SHA256SUMS
