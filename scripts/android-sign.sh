#!/usr/bin/env bash
# Signs the unsigned release files of a folder of scripts/android-dist.sh (or of a GitHub release)
# with the upload key, without Gradle and without anything of the workspace: zipalign and apksigner
# from the Android build tools, jarsigner from the JDK. The job "sign" of release-android.yml runs
# exactly this; locally it signs what `task android:dist` built without the key.
#
#   scripts/android-sign.sh <folder>
#
# The key comes from four environment variables, never from arguments (they would show in `ps`):
#   SLOUCHCAT_SIGNING_STORE_FILE      the keystore (upload.jks)
#   SLOUCHCAT_SIGNING_STORE_PASSWORD  its password
#   SLOUCHCAT_SIGNING_KEY_ALIAS       the key's alias (upload)
#   SLOUCHCAT_SIGNING_KEY_PASSWORD    the key's password (in a PKCS12 keystore the same)
# All four set: every *-release-unsigned.apk and *-release-unsigned.aab becomes *-release.apk/.aab
# (aligned to 16 KB pages, signed, verified), the unsigned file goes. None set: nothing is signed.
# Some set: stops and names the missing ones, never a value. In every case SHA256SUMS is written
# again over the *.apk and *.aab of the folder.
set -euo pipefail
shopt -s inherit_errexit nullglob

if [ "$#" -ne 1 ] || [ ! -d "$1" ]; then
  echo "usage: scripts/android-sign.sh <folder with the files of android:dist>" >&2
  exit 2
fi
dir=$(cd "$1" && pwd)

names=(SLOUCHCAT_SIGNING_STORE_FILE SLOUCHCAT_SIGNING_STORE_PASSWORD SLOUCHCAT_SIGNING_KEY_ALIAS SLOUCHCAT_SIGNING_KEY_PASSWORD)
missing=()
for name in "${names[@]}"; do
  [ -n "${!name:-}" ] || missing+=("$name")
done

if [ "${#missing[@]}" -eq "${#names[@]}" ]; then
  echo "no signing values: the release files stay unsigned"
elif [ "${#missing[@]}" -gt 0 ]; then
  echo "signing needs all four values; missing: ${missing[*]}" >&2
  exit 1
else
  if [ ! -f "$SLOUCHCAT_SIGNING_STORE_FILE" ]; then
    echo "SLOUCHCAT_SIGNING_STORE_FILE names no file" >&2
    exit 1
  fi
  sdk=${ANDROID_HOME:-$HOME/.local/share/android-sdk}
  tools=$(find "$sdk/build-tools" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' 2>/dev/null | sort -V | tail -n 1)
  if [ -z "$tools" ]; then
    echo "no build-tools under $sdk/build-tools" >&2
    exit 1
  fi
  bt="$sdk/build-tools/$tools"
  # apksigner is a Java program; jarsigner is the JDK's.
  if [ -n "${JAVA_HOME:-}" ]; then PATH="$JAVA_HOME/bin:$PATH"; fi
  umask 077
  work=$(mktemp -d)
  trap 'rm -rf "$work"' EXIT

  for unsigned in "$dir"/*-release-unsigned.apk; do
    signed="${unsigned%-unsigned.apk}.apk"
    "$bt/zipalign" -P 16 -f 4 "$unsigned" "$work/aligned.apk"
    "$bt/apksigner" sign --ks "$SLOUCHCAT_SIGNING_STORE_FILE" --ks-key-alias "$SLOUCHCAT_SIGNING_KEY_ALIAS" \
      --ks-pass env:SLOUCHCAT_SIGNING_STORE_PASSWORD --key-pass env:SLOUCHCAT_SIGNING_KEY_PASSWORD \
      --out "$work/signed.apk" "$work/aligned.apk"
    "$bt/apksigner" verify "$work/signed.apk"
    "$bt/zipalign" -c -P 16 4 "$work/signed.apk"
    mv "$work/signed.apk" "$signed"
    rm -f "$unsigned" "$work/signed.apk.idsig"
    echo "signed $(basename "$signed")"
  done

  for unsigned in "$dir"/*-release-unsigned.aab; do
    signed="${unsigned%-unsigned.aab}.aab"
    jarsigner -keystore "$SLOUCHCAT_SIGNING_STORE_FILE" \
      -storepass:env SLOUCHCAT_SIGNING_STORE_PASSWORD -keypass:env SLOUCHCAT_SIGNING_KEY_PASSWORD \
      -signedjar "$work/signed.aab" "$unsigned" "$SLOUCHCAT_SIGNING_KEY_ALIAS" >/dev/null
    # jarsigner -verify also says "jar is unsigned" with status 0: the signature block decides.
    jarsigner -verify "$work/signed.aab" >/dev/null
    # The listing first: `unzip | grep -q` fails under pipefail when grep stops reading early.
    grep -qE '^META-INF/[^/]+\.(RSA|EC|DSA)$' <<<"$(unzip -Z1 "$work/signed.aab")" \
      || { echo "$(basename "$unsigned"): no signature after jarsigner" >&2; exit 1; }
    mv "$work/signed.aab" "$signed"
    rm -f "$unsigned"
    echo "signed $(basename "$signed")"
  done
fi

cd "$dir"
files=(./*.apk ./*.aab)
if [ "${#files[@]}" -eq 0 ]; then
  echo "no *.apk or *.aab in $dir" >&2
  exit 1
fi
sha256sum -- "${files[@]#./}" >SHA256SUMS
cat SHA256SUMS
