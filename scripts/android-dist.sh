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
# Signing.kt), else unsigned and named so; scripts/android-sign.sh signs those afterwards without
# Gradle (the job "sign" of release-android.yml). Checks: every debug APK verifies with apksigner
# and its badging names the package, the version of version.properties and an icon; the TV APKs
# carry the leanback launcher and the banner; the TV release APK requires leanback and not the
# touchscreen, the phone's release APKs do not require leanback.
#
# JAVA_HOME and ANDROID_HOME as in android/Taskfile.yml. Gradle waits for a heavy slot.
set -euo pipefail
shopt -s inherit_errexit

android=$(cd "$(dirname "$0")/../android" && pwd)
# APPS is a list of words; no globbing on it.
set -f
read -r -a apps <<<"${APPS:-app tv}"
set +f
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
for app in "${apps[@]}"; do
  case "$app" in
    app) tasks+=(:app-mobile:assemblePlayDebug :app-mobile:assembleFossDebug :app-mobile:assemblePlayRelease
      :app-mobile:assembleFossRelease :app-mobile:bundlePlayRelease) ;;
    tv) tasks+=(:app-tv:assembleDebug :app-tv:assembleRelease :app-tv:bundleRelease) ;;
    *) echo "APPS: unknown app $app (app, tv)" >&2; exit 2 ;;
  esac
done

# With the upload key in play, nothing of it goes into Gradle's configuration cache.
gradle_flags=(--console=plain)
gradle_props="${GRADLE_USER_HOME:-$HOME/.gradle}/gradle.properties"
if [ -n "${SLOUCHCAT_SIGNING_STORE_FILE:-}${SLOUCHCAT_SIGNING_STORE_PASSWORD:-}${SLOUCHCAT_SIGNING_KEY_ALIAS:-}${SLOUCHCAT_SIGNING_KEY_PASSWORD:-}" ] \
  || { [ -f "$gradle_props" ] && grep -q '^[[:space:]]*slouchcat\.signing\.' "$gradle_props"; }; then
  gradle_flags+=(--no-configuration-cache)
fi

cd "$android"
# A release APK of an earlier run with or without the key would sit beside the new one under the
# other name; Gradle writes the release outputs again.
rm -rf app-mobile/build/outputs/apk/play/release app-mobile/build/outputs/apk/foss/release \
  app-mobile/build/outputs/bundle app-tv/build/outputs/apk/release app-tv/build/outputs/bundle
../scripts/heavy-slot.sh ./gradlew "${gradle_flags[@]}" "${tasks[@]}"

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
signed() { # <file>
  case "$1" in
    *.apk) "$apksigner" verify "$1" >/dev/null 2>&1 ;;
    # The listing first: `unzip | grep -q` fails under pipefail when grep stops reading early.
    *.aab) grep -qE '^META-INF/[^/]+\.(RSA|EC|DSA)$' <<<"$(unzip -Z1 "$1")" ;;
  esac
}

# Copies <source> to dist as <name>.<ext>, or <name>-unsigned.<ext> when the file has no signature;
# the name it got lands in the variable `copied`.
copied=""
copy() { # <source> <name without suffix> <ext>
  local target="$2"
  signed "$1" || target="$target-unsigned"
  cp "$1" "$dist/$target.$3"
  copied="$target.$3"
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

# The devices a release APK reaches (Google Play filters by required features): the TV app only TVs
# (leanback required, touchscreen not), the phone app never only TVs (leanback not required).
check_features() { # <apk> <tv: yes|no>
  local out
  out=$(badging "$dist/$1")
  if [ "$2" = yes ]; then
    grep -qE "^[[:space:]]*uses-feature: name='android.software.leanback'$" <<<"$out" \
      || { echo "$1: leanback is not required, the TV app would reach phones" >&2; exit 1; }
    grep -qE "^[[:space:]]*uses-feature-not-required: name='android.hardware.touchscreen'$" <<<"$out" \
      || { echo "$1: the touchscreen is not declared as not required" >&2; exit 1; }
    echo "checked $1: requires leanback, not the touchscreen"
  else
    if grep -qE "^[[:space:]]*uses-feature: name='android.software.leanback'$" <<<"$out"; then
      echo "$1: requires leanback, the phone app would reach only TVs" >&2
      exit 1
    fi
    echo "checked $1: does not require leanback"
  fi
}

for app in "${apps[@]}"; do
  if [ "$app" = app ]; then
    name=$(version app-mobile versionName)
    code=$(version app-mobile versionCode)
    out=app-mobile/build/outputs
    base="slouchcat-app-$name"
    for flavour in play foss; do
      src=$(one "$out/apk/$flavour/debug" "*.apk")
      copy "$src" "$base-$flavour-debug" apk
      check_debug "$copied" "com.slouchcat.app$([ "$flavour" = foss ] && echo .foss).dev" "$name" "$code" no
      src=$(one "$out/apk/$flavour/release" "*.apk")
      copy "$src" "$base-$flavour-release" apk
      check_features "$copied" no
    done
    src=$(one "$out/bundle/playRelease" "*.aab")
    copy "$src" "$base-play-release" aab
  else
    name=$(version app-tv versionName)
    code=$(version app-tv versionCode)
    out=app-tv/build/outputs
    base="slouchcat-tv-$name"
    src=$(one "$out/apk/debug" "*.apk")
    copy "$src" "$base-debug" apk
    check_debug "$copied" com.slouchcat.app.dev "$name" "$code" yes
    src=$(one "$out/apk/release" "*.apk")
    copy "$src" "$base-release" apk
    check_features "$copied" yes
    src=$(one "$out/bundle/release" "*.aab")
    copy "$src" "$base-release" aab
  fi
done

cd "$dist"
sha256sum -- *.apk *.aab >SHA256SUMS
echo "android/dist:"
ls -l -- *.apk *.aab SHA256SUMS
cat SHA256SUMS
