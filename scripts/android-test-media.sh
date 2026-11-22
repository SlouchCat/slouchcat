#!/usr/bin/env bash
# A small media tree for the Android tests against a real server (task android:contract,
# scripts/android-e2e.sh): a film and a series with two seasons, named as Plex and Jellyfin name
# them, with NFO files for summary, tagline and genres. No ffmpeg: the "videos" are text files, so
# the server lists every title with a file it cannot play (B-9), enough for the detail pages.
#
#   scripts/android-test-media.sh DIR     # writes DIR/Movies and DIR/TV (DIR must exist)
set -euo pipefail

dir=${1:?usage: android-test-media.sh DIR}
[ -d "$dir" ] || { echo "android-test-media: $dir is no directory" >&2; exit 1; }

file() { mkdir -p "$(dirname "$1")"; printf '%s\n' "$2" >"$1"; }

film="$dir/Movies/Night Train (2024)"
file "$film/Night Train (2024).mkv" "Slouchcat test file: text, not a video."
file "$film/Night Train (2024).nfo" '<?xml version="1.0" encoding="UTF-8"?>
<movie>
  <title>Night Train</title>
  <year>2024</year>
  <tagline>A new day begins.</tagline>
  <plot>A train runs through the night and nobody on board knows where it stops. The conductor keeps a list of every passenger, and one name on it belongs to somebody who never got on. Long enough to need more than four lines on a phone.</plot>
  <runtime>104</runtime>
  <genre>Drama</genre>
  <genre>Mystery</genre>
</movie>'

show="$dir/TV/Quiet Ring (2022)"
file "$show/tvshow.nfo" '<?xml version="1.0" encoding="UTF-8"?>
<tvshow>
  <title>Quiet Ring</title>
  <year>2022</year>
  <plot>Evil has had many names. A ring that nobody hears changes hands.</plot>
  <genre>Fantasy</genre>
</tvshow>'
for episode in "1 1 Arrival" "1 2 Cowardice" "2 1 Return"; do
  set -- $episode
  name=$(printf 'Quiet Ring S%02dE%02d' "$1" "$2")
  season=$(printf 'Season %02d' "$1")
  file "$show/$season/$name.mkv" "Slouchcat test file: text, not a video."
  file "$show/$season/$name.nfo" "<?xml version=\"1.0\" encoding=\"UTF-8\"?>
<episodedetails>
  <title>$3</title>
  <season>$1</season>
  <episode>$2</episode>
  <plot>What happens in episode $2 of season $1.</plot>
  <runtime>33</runtime>
</episodedetails>"
done
