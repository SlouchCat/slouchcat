#!/usr/bin/env bash
# Installs the toolbox for brand assets and visual checks, without root, under
# ~/.local/share/slouchcat-design (docs/dev/DESIGN-WERKZEUGE.md):
#   node/      npm packages pinned by scripts/design-tools/package-lock.json (sharp, svgo, resvg,
#              Lighthouse, axe-core, playwright-core, pixelmatch, the two browser MCP servers, ...)
#              and the sc-* tools from scripts/design-tools/tools
#   browsers/  Chrome for Testing, one pinned version for Lighthouse, axe, sc-shot and the MCP servers
#   bin/       Inkscape and ImageMagick as AppImages (SHA-256 checked) and small start scripts,
#              among them the two browser MCP servers (sc-mcp-playwright, sc-mcp-chrome-devtools)
#   media/     real media pictures for mockups (scripts/design-media.sh, never in git)
# fonttools comes through pipx. Links to the commands are placed in ~/.local/bin.
# Run it again any time; what is there stays. `--uninstall` removes everything except media/,
# `--uninstall --purge` removes media/ too.
set -euo pipefail

ROOT="${SLOUCHCAT_DESIGN_HOME:-$HOME/.local/share/slouchcat-design}"
LINKS="$HOME/.local/bin"
here="$(cd "$(dirname "$0")" && pwd)"

INKSCAPE_VERSION="1.4.4"
INKSCAPE_URL="https://media.inkscape.org/dl/resources/file/Inkscape-1.4.4.AppImage"
INKSCAPE_SHA256="d217fe05c92faaed5c3af46856b6299c98933dc722819cb5bc2ca052c5522166"
MAGICK_VERSION="7.1.2-36"
MAGICK_URL="https://github.com/ImageMagick/ImageMagick/releases/download/7.1.2-36/ImageMagick-7.1.2-36-x86_64.AppImage"
MAGICK_SHA256="68cca0053a8f82206e72400014f32f4b654e9807f45928a2325a1d14d695fb3a"
CHROME_VERSION="158.0.8127.88"
# SHA-256 of chrome-linux64/chrome as unpacked on 28 November 2026; @puppeteer/browsers checks only TLS.
CHROME_SHA256="1ce7865c612bd8a55b4a07b2df89350e408d603a52763b5902b48143b8033903"
FONTTOOLS_VERSION="4.69.1"
BROTLI_VERSION="1.2.1"
ZOPFLI_VERSION="0.4.3"
MARKER=".slouchcat-design-toolbox"

# Commands linked into ~/.local/bin: name and target below $ROOT.
LINKED=(
  "inkscape bin/inkscape"
  "magick bin/magick"
  "lighthouse bin/lighthouse"
  "svgo node/node_modules/.bin/svgo"
  "pwa-assets-generator node/node_modules/.bin/pwa-assets-generator"
  "s2v node/node_modules/.bin/s2v"
  "png-to-ico node/node_modules/.bin/png-to-ico"
  "sc-shot node/tools/sc-shot.mjs"
  "sc-axe node/tools/sc-axe.mjs"
  "sc-pixeldiff node/tools/sc-pixeldiff.mjs"
  "sc-svg2png node/tools/sc-svg2png.mjs"
  "sc-mcp-check node/tools/sc-mcp-check.mjs"
  "sc-mcp-playwright bin/sc-mcp-playwright"
  "sc-mcp-chrome-devtools bin/sc-mcp-chrome-devtools"
)

# A link is ours when it points below $ROOT; anything else with the same name is left alone.
is_ours() { [ -L "$1" ] && case "$(readlink "$1")" in "$ROOT"/*) true ;; *) false ;; esac; }

if [ "${1:-}" = "--uninstall" ]; then
  # Deletes only a folder this script created (marker file), never $HOME or /.
  case "$ROOT" in "" | / | "$HOME" | "$HOME/") echo "refusing to remove $ROOT" >&2; exit 1 ;; esac
  [ -f "$ROOT/$MARKER" ] || { echo "no toolbox marker in $ROOT; nothing removed" >&2; exit 1; }
  for entry in "${LINKED[@]}"; do
    name="${entry%% *}"
    if is_ours "$LINKS/$name"; then rm -f "$LINKS/$name"; fi
  done
  rm -rf "$ROOT/node" "$ROOT/browsers" "$ROOT/bin"
  if [ "${2:-}" = "--purge" ]; then rm -rf "$ROOT"; fi
  if command -v pipx >/dev/null 2>&1; then pipx uninstall fonttools >/dev/null 2>&1 || true; fi
  echo "removed: toolbox in $ROOT, links in $LINKS, fonttools (pipx)"
  exit 0
fi

command -v node >/dev/null 2>&1 || { echo "node is missing (docs/dev/SETUP.md)" >&2; exit 1; }
node_bin="$(command -v node)"
case "$ROOT" in "" | / | "$HOME" | "$HOME/") echo "SLOUCHCAT_DESIGN_HOME must be its own folder" >&2; exit 1 ;; esac
mkdir -p "$ROOT/node" "$ROOT/bin" "$ROOT/browsers" "$ROOT/media" "$LINKS"
touch "$ROOT/$MARKER"

# 1. npm packages, exactly as locked. No install scripts run; none of the packages needs one.
lock_sum="$(sha256sum "$here/design-tools/package-lock.json" | cut -d' ' -f1)"
if [ "$(cat "$ROOT/node/.lock-sha256" 2>/dev/null || true)" != "$lock_sum" ] || [ ! -d "$ROOT/node/node_modules" ]; then
  cp "$here/design-tools/package.json" "$here/design-tools/package-lock.json" "$ROOT/node/"
  (cd "$ROOT/node" && npm ci --ignore-scripts --no-audit --no-fund)
  printf '%s\n' "$lock_sum" >"$ROOT/node/.lock-sha256"
fi
rm -rf "$ROOT/node/tools"
cp -r "$here/design-tools/tools" "$ROOT/node/tools"
chmod +x "$ROOT/node/tools"/sc-*.mjs

# 2. Chrome for Testing, Google's build for automation, from its official download bucket.
chrome="$ROOT/browsers/chrome/linux-$CHROME_VERSION/chrome-linux64/chrome"
if [ ! -x "$chrome" ]; then
  "$ROOT/node/node_modules/.bin/browsers" install "chrome@$CHROME_VERSION" --path "$ROOT/browsers"
fi
if ! echo "$CHROME_SHA256  $chrome" | sha256sum -c --quiet -; then
  rm -rf "$ROOT/browsers/chrome/linux-$CHROME_VERSION"
  echo "Chrome binary does not match the pinned SHA-256; removed" >&2
  exit 1
fi
ln -sfn "$chrome" "$ROOT/bin/chrome"

# 3. Inkscape and ImageMagick: the projects' own AppImages, checked against the published SHA-256.
fetch_appimage() { # name version url sha256
  local file="$ROOT/bin/$1-$2.AppImage"
  if [ ! -x "$file" ]; then
    curl -fL --retry 2 -o "$file.part" "$3"
    echo "$4  $file.part" | sha256sum -c -
    chmod +x "$file.part"
    mv "$file.part" "$file"
  fi
}
fetch_appimage Inkscape "$INKSCAPE_VERSION" "$INKSCAPE_URL" "$INKSCAPE_SHA256"
fetch_appimage ImageMagick "$MAGICK_VERSION" "$MAGICK_URL" "$MAGICK_SHA256"

write_script() { # path, then the script on stdin
  cat >"$1"
  chmod +x "$1"
}
write_script "$ROOT/bin/inkscape" <<EOF
#!/bin/sh
exec "$ROOT/bin/Inkscape-$INKSCAPE_VERSION.AppImage" "\$@"
EOF
write_script "$ROOT/bin/magick" <<EOF
#!/bin/sh
exec "$ROOT/bin/ImageMagick-$MAGICK_VERSION.AppImage" "\$@"
EOF
# Lighthouse with the toolbox's Chrome, never sending error reports (Sentry), headless unless the
# caller passes its own --chrome-flags.
write_script "$ROOT/bin/lighthouse" <<EOF
#!/bin/sh
export CHROME_PATH="\${CHROME_PATH:-$ROOT/bin/chrome}"
case " \$* " in
  *" --chrome-flags"*) exec "$node_bin" "$ROOT/node/node_modules/lighthouse/cli/index.js" --no-enable-error-reporting "\$@" ;;
  *) exec "$node_bin" "$ROOT/node/node_modules/lighthouse/cli/index.js" --no-enable-error-reporting --chrome-flags="--headless=new" "\$@" ;;
esac
EOF

# The two browser MCP servers, started the same way everywhere (agent definitions in .claude/agents,
# sc-mcp-check, a session's own --mcp-config): headless, a fresh profile per start, the toolbox's
# Chrome. Chrome DevTools MCP runs without usage statistics, CrUX lookups and update checks, so no
# page address leaves the machine. Extra arguments are passed on (for example --device "Pixel 9").
write_script "$ROOT/bin/sc-mcp-playwright" <<EOF
#!/bin/sh
mkdir -p "$ROOT/out/playwright-mcp"
exec "$node_bin" "$ROOT/node/node_modules/.bin/playwright-mcp" --headless --isolated \\
  --executable-path "$ROOT/bin/chrome" --viewport-size 1440x900 \\
  --output-dir "$ROOT/out/playwright-mcp" "\$@"
EOF
write_script "$ROOT/bin/sc-mcp-chrome-devtools" <<EOF
#!/bin/sh
export CHROME_DEVTOOLS_MCP_NO_UPDATE_CHECKS=1
exec "$node_bin" "$ROOT/node/node_modules/.bin/chrome-devtools-mcp" --headless --isolated \\
  --executablePath "$ROOT/bin/chrome" --viewport 1440x900 --no-file-navigations \\
  --no-usage-statistics --no-performance-crux "\$@"
EOF

# 4. fonttools with Brotli and Zopfli (WOFF2) through pipx, all three pinned, wheels only.
if command -v pipx >/dev/null 2>&1; then
  if ! pipx list --short 2>/dev/null | grep -qx "fonttools $FONTTOOLS_VERSION"; then
    pins="$(mktemp)"
    printf 'brotli==%s\nzopfli==%s\n' "$BROTLI_VERSION" "$ZOPFLI_VERSION" >"$pins"
    pipx install --force --pip-args="--only-binary=:all: --constraint $pins" \
      "fonttools[woff]==$FONTTOOLS_VERSION"
    rm -f "$pins"
  fi
else
  echo "pipx is missing: fonttools not installed (sudo apt install pipx, then run again)" >&2
fi

# 5. Links in ~/.local/bin. A foreign file of the same name stays and is reported.
for entry in "${LINKED[@]}"; do
  name="${entry%% *}"
  target="$ROOT/${entry#* }"
  if { [ -e "$LINKS/$name" ] || [ -L "$LINKS/$name" ]; } && ! is_ours "$LINKS/$name"; then
    echo "kept existing $LINKS/$name (not from this toolbox); use $target directly" >&2
    continue
  fi
  ln -sfn "$target" "$LINKS/$name"
done

echo "toolbox ready in $ROOT; check it with scripts/design-tools-check.sh"
