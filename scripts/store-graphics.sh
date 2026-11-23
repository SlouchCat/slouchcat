#!/usr/bin/env bash
# Renders the three store graphics of docs/store/grafiken.md (app icon 512 x 512, feature graphic
# 1024 x 500, TV banner 1280 x 720) from the SVG sources in docs/store/grafiken/ into a folder
# OUTSIDE the repository: the images never go into the repository.
#
#   scripts/store-graphics.sh <target folder>
#
# Needs the workspace (`pnpm install`) and Playwright's Chromium (`pnpm --dir web exec playwright
# install chromium`), as the e2e tests do.
set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "usage: scripts/store-graphics.sh <target folder outside the repository>" >&2
  exit 2
fi
repo=$(cd "$(dirname "$0")/.." && pwd -P)
# Resolved before anything is created, symbolic links included.
target=$(realpath -m -- "$1")
case "$target/" in
  "$repo"/*)
    echo "refusing $target: the store graphics stay outside the repository ($repo)" >&2
    exit 1
    ;;
esac
mkdir -p "$target"
node "$repo/scripts/store-graphics.mjs" "$target"
