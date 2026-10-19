#!/usr/bin/env bash
# Regenerates the artwork the stories show (public/*.jpg) with ffmpeg. Abstract gradients stand in
# for TMDB posters and backdrops; the hex values are fixture data, like ItemRef.accent in the
# stories. Run from web/: `bash .storybook/make-fixtures.sh`. The output is committed.
set -euo pipefail
cd "$(dirname "$0")/public"

art() { # name width height colour0 colour1 colour2 seed type
  ffmpeg -hide_banner -loglevel error -y -f lavfi \
    -i "gradients=s=$2x$3:c0=$4:c1=$5:c2=$6:n=3:type=${8:-linear}:seed=$7:duration=1" \
    -vf "noise=alls=6:allf=u,format=yuvj420p" -frames:v 1 -q:v 4 "$1.jpg"
}

art poster-dusk 400 600 0x0f1d3a 0x2f6fb3 0xe0a35a 7
art poster-ember 400 600 0x1a0c08 0xb5482a 0xf2c27a 11 radial
art poster-moss 400 600 0x0c1a12 0x3d7d4f 0xc9d98a 3
art backdrop-dusk 1280 720 0x0b1630 0x2b5d9c 0xd99a52 5
