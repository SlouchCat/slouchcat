#!/usr/bin/env bash
# Checks a release tag of the Android apps against the version file of its app and prints the
# app (`app` or `tv`, the APPS of scripts/android-dist.sh). Used by .github/workflows/release-android.yml.
#
#   scripts/android-tag-check.sh app-v1.0.0   -> app   (android/app-mobile/version.properties)
#   scripts/android-tag-check.sh tv-v1.0.0    -> tv    (android/app-tv/version.properties)
#
# Fails when the tag has another form or its version is not the file's versionName.
set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "usage: scripts/android-tag-check.sh <app-vX.Y.Z|tv-vX.Y.Z>" >&2
  exit 2
fi
tag=$1
case "$tag" in
  app-v*) app=app; module=app-mobile ;;
  tv-v*) app=tv; module=app-tv ;;
  *) echo "tag $tag is neither app-vX.Y.Z nor tv-vX.Y.Z" >&2; exit 1 ;;
esac
version=${tag#"$app"-v}
if ! [[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "tag $tag: $version is not X.Y.Z" >&2
  exit 1
fi
file="$(cd "$(dirname "$0")/.." && pwd)/android/$module/version.properties"
name=$(sed -n 's/^versionName=//p' "$file" | tr -d '[:space:]')
if [ "$name" != "$version" ]; then
  echo "tag $tag, but android/$module/version.properties has versionName=$name" >&2
  exit 1
fi
echo "$app"
