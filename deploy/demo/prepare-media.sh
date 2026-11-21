#!/bin/sh
# Prepares the media folder of the demo server (concept 27, F-10). Runs once in the service
# "media" of compose.yaml (busybox sh of the curl image), before the server starts.
#
# DEMO_SOURCE=blender (default): downloads the films of films.tsv into /media/Movies/<folder>/,
#   checks each download against its SHA-256 and unpacks zips. Films already in place are kept.
# DEMO_SOURCE=local: downloads nothing and only checks that /media/Movies holds a file, for
#   example synthetic media from testdata/generate-media.sh (README, «Ohne Download»).
set -eu

MEDIA=/media
MOVIES="$MEDIA/Movies"
INCOMING="$MEDIA/.incoming" # hidden: the scanner skips it
LIST=/demo/films.tsv
SOURCE="${DEMO_SOURCE:-blender}"

fail() {
  echo "prepare-media: $*" >&2
  exit 1
}

case "$SOURCE" in
blender) ;;
local)
  [ -n "$(find "$MOVIES" -type f 2>/dev/null | head -n 1)" ] ||
    fail "DEMO_SOURCE=local, but $MOVIES holds no file"
  echo "prepare-media: DEMO_SOURCE=local, $MOVIES is in place"
  exit 0
  ;;
*) fail "unknown DEMO_SOURCE '$SOURCE' (blender or local)" ;;
esac

mkdir -p "$MOVIES" "$INCOMING"
tab="$(printf '\t')"
failed=0

while IFS="$tab" read -r folder suffix url sha member; do
  case "$folder" in '' | '#'*) continue ;; esac
  target="$MOVIES/$folder/$folder.$suffix"
  if [ -f "$target" ]; then
    echo "prepare-media: keep $folder.$suffix"
    continue
  fi
  echo "prepare-media: download $url"
  part="$INCOMING/download.part"
  rm -f "$part"
  if ! curl -fsSL --retry 3 --retry-delay 5 -o "$part" "$url"; then
    echo "prepare-media: download failed: $url" >&2
    failed=1
    continue
  fi
  got="$(sha256sum "$part" | cut -d ' ' -f 1)"
  if [ "$got" != "$sha" ]; then
    echo "prepare-media: SHA-256 mismatch for $url (got $got)" >&2
    rm -f "$part"
    failed=1
    continue
  fi
  mkdir -p "$MOVIES/$folder"
  if [ "$member" = "-" ]; then
    mv "$part" "$target.part"
  else
    unzip -p "$part" "$member" >"$target.part"
    rm -f "$part"
  fi
  mv "$target.part" "$target"
  echo "prepare-media: ready $folder.$suffix"
done <"$LIST"

rmdir "$INCOMING" 2>/dev/null || true

# Attribution (CC BY): the films carry their credits; this file names them once more.
cat >"$MEDIA/CREDITS.txt" <<'EOF'
Films of the Slouchcat demo server, open movies of the Blender Foundation:
- Elephants Dream (2006): (c) copyright 2006, Blender Foundation / Netherlands Media Art Institute / www.elephantsdream.org, CC BY 2.5
- Big Buck Bunny (2008): (c) copyright 2008, Blender Foundation / www.bigbuckbunny.org, CC BY 3.0
- Sintel (2010): (c) copyright Blender Foundation | durian.blender.org, CC BY 3.0
- Tears of Steel (2012): (CC) Blender Foundation | mango.blender.org, CC BY 3.0
Licences: https://creativecommons.org/licenses/by/2.5/ and https://creativecommons.org/licenses/by/3.0/
EOF

[ "$failed" = 0 ] || fail "at least one film is missing; run 'docker compose up media' again"
echo "prepare-media: done"
