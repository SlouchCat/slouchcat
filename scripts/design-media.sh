#!/usr/bin/env bash
# Keeps the real media pictures for mockups in one place outside git
# (~/.local/share/slouchcat-design/media/<set>) and links a set into a mockup folder as `img`, so
# the pictures survive a removed worktree and never reach the public repository
# (docs/dev/DESIGN-WERKZEUGE.md, section «Echte Medienbilder»).
#   design-media.sh status                  the sets, with number of files and size
#   design-media.sh seed <folder> <set>     copies an existing picture folder into a new set
#   design-media.sh link <mockup> [<set>]   links the set as <mockup>/img and keeps `img` out of git
set -euo pipefail

MEDIA="${SLOUCHCAT_DESIGN_HOME:-$HOME/.local/share/slouchcat-design}/media"
mkdir -p "$MEDIA"

usage() {
  sed -n '6,8s/^# *//p' "$0" >&2
  exit 2
}
# A set name is one plain folder name below $MEDIA.
check_name() {
  case "$1" in "" | . | .. | *[!A-Za-z0-9._-]*) echo "invalid set name: $1" >&2; exit 2 ;; esac
}

case "${1:-}" in
  status)
    found=0
    for set in "$MEDIA"/*/; do
      [ -d "$set" ] || continue
      found=1
      printf '%-24s %5s files  %s\n' "$(basename "$set")" \
        "$(find "$set" -type f ! -name 'README.txt' | wc -l)" "$(du -sh "$set" | cut -f1)"
    done
    [ "$found" = 1 ] || echo "no picture sets in $MEDIA"
    ;;
  seed)
    [ $# -eq 3 ] || usage
    check_name "$3"
    from="$2"
    set="$MEDIA/$3"
    [ -d "$from" ] || { echo "no such folder: $from" >&2; exit 1; }
    [ ! -e "$set" ] || { echo "set exists already: $set" >&2; exit 1; }
    mkdir -p "$set"
    cp -rL "$from"/. "$set"/
    cat >"$set/README.txt" <<EOF
Picture set "$3" for Slouchcat mockups.
Copied from: $(cd "$from" && pwd)
Copied on:   $(date +%F)
Covers, stills and portraits belong to their rights holders. Use them on this machine only:
never commit them, never publish a page that contains them.
EOF
    echo "set $3: $(find "$set" -type f ! -name 'README.txt' | wc -l) files"
    ;;
  link)
    [ $# -ge 2 ] || usage
    mockup="$2"
    [ -d "$mockup" ] || { echo "no such mockup folder: $mockup" >&2; exit 1; }
    if [ $# -ge 3 ]; then
      check_name "$3"
      set="$MEDIA/$3"
    else
      set="$(find "$MEDIA" -mindepth 1 -maxdepth 1 -type d | sort | tail -1)"
    fi
    [ -n "$set" ] && [ -d "$set" ] || { echo "no such set; see: $0 status" >&2; exit 1; }
    if [ -e "$mockup/img" ] && [ ! -L "$mockup/img" ]; then
      echo "$mockup/img is a real folder; move it away first (or seed a set from it)" >&2
      exit 1
    fi
    ln -sfn "$set" "$mockup/img"
    # `img/` in a .gitignore matches a folder only; the link needs the name without the slash.
    touch "$mockup/.gitignore"
    grep -qx 'img' "$mockup/.gitignore" || echo 'img' >>"$mockup/.gitignore"
    echo "$mockup/img -> $set"
    ;;
  *) usage ;;
esac
