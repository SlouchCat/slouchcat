#!/usr/bin/env bash
# Proves that the toolbox of scripts/install-design-tools.sh works, by using it: builds a test mark
# and a small test page with planted defects, serves the page on 127.0.0.1 and runs every tool
# against them. One line per check, exit code 1 when a check fails.
#   KEEP=1  keeps the working directory and prints its path
#   SKIP_MCP=1 / SKIP_LIGHTHOUSE=1  leave out the slow browser checks
set -uo pipefail

ROOT="${SLOUCHCAT_DESIGN_HOME:-$HOME/.local/share/slouchcat-design}"
WORK="$(mktemp -d)"
server=""
cleanup() {
  if [ -n "$server" ]; then kill "$server" 2>/dev/null; fi
  if [ "${KEEP:-}" = "1" ]; then echo "kept: $WORK"; else rm -rf "$WORK"; fi
}
trap cleanup EXIT
cd "$WORK" || exit 1

failed=0
check() { # label, then the command; its output is shown only when it fails
  local label="$1"
  shift
  if "$@" >"$WORK/.last" 2>&1; then
    echo "ok    $label"
  else
    echo "FAIL  $label"
    sed 's/^/        /' "$WORK/.last" | head -15
    failed=$((failed + 1))
  fi
}
# PNG width and height from the header, without any image library.
png_size() { python3 -c '
import struct,sys
d=open(sys.argv[1],"rb").read(26)
assert d[:8]==b"\x89PNG\r\n\x1a\n", "not a PNG"
print("%dx%d" % struct.unpack(">II", d[16:24]))' "$1"; }
is_size() { [ "$(png_size "$1")" = "$2" ]; }
png_has_alpha() { python3 -c '
import sys
d=open(sys.argv[1],"rb").read(26)
sys.exit(0 if d[25] in (4,6) else 1)' "$1"; }
ico_images() { python3 -c '
import struct,sys
d=open(sys.argv[1],"rb").read(6)
assert d[:4]==b"\x00\x00\x01\x00", "not an ICO"
print(struct.unpack("<H", d[4:6])[0])' "$1"; }
ico_holds() { [ "$(ico_images "$1")" = "$2" ]; }

# --- test material -------------------------------------------------------------------------------
cat >mark.svg <<'EOF'
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" width="512" height="512">
  <!-- test mark of the tool check, not a Slouchcat logo -->
  <circle cx="256" cy="256" r="200" fill="#e8794a"/>
  <path d="M176 176h160v160H176z" fill="#101014"/>
  <text x="256" y="470" font-family="DejaVu Sans, sans-serif" font-size="48" text-anchor="middle" fill="#e8794a">Probe</text>
</svg>
EOF
cat >banner.svg <<'EOF'
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 320 180" width="320" height="180">
  <rect width="320" height="180" fill="#101014"/>
  <circle cx="90" cy="90" r="50" fill="#e8794a"/>
</svg>
EOF
page() { # file name, body additions
  cat >"$1" <<EOF
<!doctype html>
<html lang="de">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Werkzeugprobe</title>
<style>
  body { font: 16px system-ui, sans-serif; margin: 2rem; background: #101014; color: #f2f2f2; }
  button, input { font: inherit; padding: 0.5rem 0.75rem; }
  .faint { color: #2b2b30; }
</style>
</head>
<body>
<main>
<h1>Werkzeugprobe</h1>
<p><label for="name">Name</label> <input id="name" name="name" autocomplete="off"></p>
<p><button id="open" type="button">Open dialog</button></p>
<dialog id="dialog" aria-label="Probe"><p>Dialog open</p><form method="dialog"><button>Close</button></form></dialog>
<p><img src="mark-512.png" width="128" height="128" alt="Probezeichen"></p>
$2
</main>
<script>
  document.getElementById('open').addEventListener('click', () => document.getElementById('dialog').showModal());
</script>
</body>
</html>
EOF
}
page clean.html ""
page defect.html '<p class="faint">Kontrast zu schwach</p><p><img src="missing.png" width="64" height="64"></p>'

# --- brand assets --------------------------------------------------------------------------------
check "svgo: SVG optimised and still valid" bash -c '
  svgo -q mark.svg -o mark.min.svg &&
  python3 -c "import xml.dom.minidom,sys; xml.dom.minidom.parse(\"mark.min.svg\")" &&
  [ "$(wc -c <mark.min.svg)" -le "$(wc -c <mark.svg)" ]'
check "inkscape: text converted to paths (SVG)" bash -c '
  inkscape mark.svg --export-text-to-path --export-plain-svg -o mark-paths.svg 2>/dev/null &&
  ! grep -q "<text" mark-paths.svg && grep -q "<path" mark-paths.svg'
check "inkscape: SVG rendered to PNG 512" bash -c 'inkscape mark.svg -w 512 -h 512 -o ink-512.png 2>/dev/null && [ -s ink-512.png ]'
check "inkscape: PNG is 512x512" is_size ink-512.png 512x512
check "resvg (sc-svg2png): transparent PNG 512" bash -c 'sc-svg2png mark.svg mark-512.png --width 512 >/dev/null'
check "resvg: size 512x512" is_size mark-512.png 512x512
check "resvg: alpha channel present" png_has_alpha mark-512.png
check "imagemagick: monochrome variant" bash -c '
  magick mark-512.png -alpha extract -background "#ffffff" -alpha shape mark-mono.png &&
  [ "$(magick identify -format "%[colors]" mark-mono.png)" -le 256 ]'
check "imagemagick: ICO with 16, 32 and 48 px" bash -c '
  magick mark-512.png -define icon:auto-resize=16,32,48 favicon.ico'
check "imagemagick: ICO holds three images" ico_holds favicon.ico 3
check "png-to-ico: ICO from PNG" bash -c 'magick mark-512.png -resize 256x256 mark-256.png && png-to-ico mark-256.png >favicon-2.ico && [ -s favicon-2.ico ]'
check "sharp (pwa-assets-generator): PWA, maskable and Apple icons" bash -c '
  mkdir -p pwa && cp mark.svg pwa/logo.svg && (cd pwa && pwa-assets-generator --preset minimal-2023 logo.svg >/dev/null 2>&1) &&
  ls pwa/pwa-192x192.png pwa/pwa-512x512.png pwa/maskable-icon-512x512.png pwa/apple-touch-icon-180x180.png pwa/favicon.ico >/dev/null'
check "sharp: maskable icon is 512x512" is_size pwa/maskable-icon-512x512.png 512x512
check "sharp: Apple touch icon is 180x180" is_size pwa/apple-touch-icon-180x180.png 180x180
check "svg2vectordrawable: Android vector drawable" bash -c '
  s2v -i mark-paths.svg -o ic_launcher_foreground.xml >/dev/null 2>&1 && grep -q "<vector" ic_launcher_foreground.xml'
check "android: adaptive icon layer 432 px (xxxhdpi)" bash -c 'sc-svg2png mark.svg ic_foreground-xxxhdpi.png --width 432 >/dev/null'
check "android: layer is 432x432" is_size ic_foreground-xxxhdpi.png 432x432
check "android tv: banner 320x180" bash -c 'sc-svg2png banner.svg tv-banner.png --width 320 >/dev/null'
check "android tv: banner is 320x180" is_size tv-banner.png 320x180
check "imagemagick: splash screen 1170x2532" bash -c '
  magick -size 1170x2532 "xc:#101014" \( mark-512.png -resize 384x384 \) -gravity center -composite splash.png'
check "imagemagick: splash is 1170x2532" is_size splash.png 1170x2532
check "fonttools: WOFF2 subset of a system font" bash -c '
  font="$(fc-list : file | cut -d: -f1 | grep -m1 -E "DejaVuSans\.ttf$")" && [ -n "$font" ] &&
  pyftsubset "$font" --unicodes=U+0020-007E --flavor=woff2 --output-file=subset.woff2 &&
  [ "$(head -c 4 subset.woff2)" = "wOF2" ] && [ "$(wc -c <subset.woff2)" -lt "$(wc -c <"$font")" ]'

# --- the test page in a browser ------------------------------------------------------------------
port="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1])')"
python3 -m http.server "$port" --bind 127.0.0.1 >/dev/null 2>&1 &
server=$!
base="http://127.0.0.1:$port"
for _ in 1 2 3 4 5 6 7 8 9 10; do curl -fs "$base/clean.html" >/dev/null 2>&1 && break; sleep 0.3; done
# The port was free a moment ago; if another process took it, stop rather than test a foreign page.
if ! kill -0 "$server" 2>/dev/null || ! curl -fs "$base/defect.html" | grep -q 'Werkzeugprobe'; then
  echo "FAIL  test page server on 127.0.0.1:$port did not start"
  exit 1
fi

check "sc-shot: clean page, phone and desktop, no findings" sc-shot "$base/clean.html" --out shots --name clean --viewports 390x844,1440x900
check "sc-shot: phone screenshot is 390x844" is_size shots/clean-390x844.png 390x844
check "sc-shot: desktop screenshot is 1440x900" is_size shots/clean-1440x900.png 1440x900
check "sc-shot: defect page reported (broken image, failed request)" bash -c "
  ! sc-shot '$base/defect.html' --out shots --name defect --viewports 1440x900 >/dev/null &&
  python3 -c '
import json
v=json.load(open(\"shots/defect-report.json\"))[\"viewports\"][0]
assert len(v[\"brokenImages\"])==1 and any(\"missing.png\" in r[\"url\"] for r in v[\"failedRequests\"]), v'"
check "sc-axe: clean page has no violations" sc-axe "$base/clean.html" --fail-on minor
check "sc-axe: defect page reports contrast and missing alt" bash -c "
  ! sc-axe '$base/defect.html' --out axe.json >/dev/null &&
  python3 -c '
import json
ids={v[\"id\"] for v in json.load(open(\"axe.json\"))[\"violations\"]}
assert {\"color-contrast\",\"image-alt\"} <= ids, ids'"
check "sc-pixeldiff: identical screenshots match" sc-pixeldiff shots/clean-1440x900.png shots/clean-1440x900.png
check "sc-pixeldiff: different screenshots differ" bash -c '! sc-pixeldiff shots/clean-1440x900.png shots/defect-1440x900.png --out diff.png >/dev/null && [ -s diff.png ]'
if [ "${SKIP_LIGHTHOUSE:-}" != "1" ]; then
  check "lighthouse: performance and accessibility scores" bash -c "
    lighthouse '$base/clean.html' --only-categories=performance,accessibility --output=json --output-path=lighthouse.json --quiet >/dev/null 2>&1
    python3 -c '
import json
c=json.load(open(\"lighthouse.json\"))[\"categories\"]
p,a=c[\"performance\"][\"score\"],c[\"accessibility\"][\"score\"]
assert p is not None and a is not None, c
print(\"performance\",p,\"accessibility\",a)'"
fi
if [ "${SKIP_MCP:-}" != "1" ]; then
  sc-mcp-check "$base/defect.html"
  failed=$((failed + $?))
fi

echo
if [ "$failed" -eq 0 ]; then echo "all checks passed"; else echo "$failed check(s) failed"; fi
[ "$failed" -eq 0 ]
