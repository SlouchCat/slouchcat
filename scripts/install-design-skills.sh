#!/usr/bin/env bash
# Installs the third-party design skills for Claude Code into ~/.claude/skills
# (docs/dev/DESIGN-WERKZEUGE.md). Every skill is copied from its source repository at the commit
# that was reviewed on 28 November 2026; nothing from these repositories is executed, and none of
# their hooks, commands or MCP settings is installed. Each copy carries a `.source` file with
# repository, commit and licence. In ~/.claude/settings.json they are switched off
# (`skillOverrides: "off"`), so other projects do not see them; the committed .claude/settings.json
# of this repository switches them on again as "name-only" (listed without description, loaded on
# demand). A project setting wins over the user setting (checked 28 November 2026).
#   --uninstall   removes the copies and the settings entries again
#   IMPECCABLE_ENGINE=1   also copies impeccable's `scripts/` folder; its launcher then downloads
#                         and runs the project's engine binary on first use (off by default)
# Run it again any time; it replaces its own copies and leaves foreign folders alone.
set -euo pipefail

SKILLS_DIR="${CLAUDE_CONFIG_DIR:-$HOME/.claude}/skills"
SETTINGS="${CLAUDE_CONFIG_DIR:-$HOME/.claude}/settings.json"

# repository | commit | licence | licence file in the repository (- = none) | path of the skill | name
SOURCES='
nextlevelbuilder/ui-ux-pro-max-skill|11ca2b3bdd555649f278ee348c32a9302b24dcf1|MIT|LICENSE|.claude/skills/ui-ux-pro-max|ui-ux-pro-max
pbakaus/impeccable|2fc5e1459efbbf06d92c7306517c286bf12490cf|Apache-2.0|LICENSE|plugin/skills/impeccable|impeccable
emilkowalski/skills|64eb3deb9a5fbfa041faf783e0bc3fd635872e9c|MIT|LICENSE|skills/emil-design-eng|emil-design-eng
emilkowalski/skills|64eb3deb9a5fbfa041faf783e0bc3fd635872e9c|MIT|LICENSE|skills/animate|animate
emilkowalski/skills|64eb3deb9a5fbfa041faf783e0bc3fd635872e9c|MIT|LICENSE|skills/animation-performance|animation-performance
emilkowalski/skills|64eb3deb9a5fbfa041faf783e0bc3fd635872e9c|MIT|LICENSE|skills/gestures|gestures
emilkowalski/skills|64eb3deb9a5fbfa041faf783e0bc3fd635872e9c|MIT|LICENSE|skills/mobile-native|mobile-native
emilkowalski/skills|64eb3deb9a5fbfa041faf783e0bc3fd635872e9c|MIT|LICENSE|skills/break-ui|break-ui
UXLana/design-skills-pack|298a510c2f4f643973ba226516835b4869d38338|MIT|LICENSE|skills/design-accessibility|design-accessibility
UXLana/design-skills-pack|298a510c2f4f643973ba226516835b4869d38338|MIT|LICENSE|skills/token-audit|token-audit
UXLana/design-skills-pack|298a510c2f4f643973ba226516835b4869d38338|MIT|LICENSE|skills/responsive-audit|responsive-audit
UXLana/design-skills-pack|298a510c2f4f643973ba226516835b4869d38338|MIT|LICENSE|skills/ui-edge-case-audit|ui-edge-case-audit
UXLana/design-skills-pack|298a510c2f4f643973ba226516835b4869d38338|MIT|LICENSE|skills/visual-qa|visual-qa
UXLana/design-skills-pack|298a510c2f4f643973ba226516835b4869d38338|MIT|LICENSE|skills/icon-generator|icon-generator
UXLana/design-skills-pack|298a510c2f4f643973ba226516835b4869d38338|MIT|LICENSE|skills/theme-generator|theme-generator
anthropics/skills|e4bb5d960d87126ff3b3a8af1a104b7db67560ba|Apache-2.0, fonts OFL-1.1|-|skills/canvas-design|canvas-design
vercel-labs/agent-skills|5a37a6329fbad3666c3207392a6a7c0110e92efb|MIT (README, no licence file)|-|skills/web-design-guidelines|web-design-guidelines
vercel-labs/agent-skills|5a37a6329fbad3666c3207392a6a7c0110e92efb|MIT (README, no licence file)|-|skills/react-best-practices|vercel-react-best-practices
vercel-labs/agent-skills|5a37a6329fbad3666c3207392a6a7c0110e92efb|MIT (README, no licence file)|-|skills/composition-patterns|vercel-composition-patterns
android/skills|7f7b21730a605d112ca9f79b6e40105ce7b670ee|Apache-2.0|LICENSE.txt|jetpack-compose/jetpack-compose-performance|jetpack-compose-performance
android/skills|7f7b21730a605d112ca9f79b6e40105ce7b670ee|Apache-2.0|LICENSE.txt|jetpack-compose/adaptive|adaptive
android/skills|7f7b21730a605d112ca9f79b6e40105ce7b670ee|Apache-2.0|LICENSE.txt|system/edge-to-edge|edge-to-edge
android/skills|7f7b21730a605d112ca9f79b6e40105ce7b670ee|Apache-2.0|LICENSE.txt|tv/leanback-to-compose-tv-migration|leanback-to-compose-tv-migration
ChromeDevTools/chrome-devtools-mcp|597b029dc9f1c1bbb48b71dc3d8abf25efec6f8c|Apache-2.0|LICENSE|skills/a11y-debugging|a11y-debugging
ChromeDevTools/chrome-devtools-mcp|597b029dc9f1c1bbb48b71dc3d8abf25efec6f8c|Apache-2.0|LICENSE|skills/debug-optimize-lcp|debug-optimize-lcp
'

names() { printf '%s\n' "$SOURCES" | awk -F'|' 'NF==6 {print $6}'; }

# Sets or removes "off" for our skills in the user's skillOverrides; other settings stay as they are.
settings() { # add | remove
  [ -f "$SETTINGS" ] || echo '{}' >"$SETTINGS"
  cp -p "$SETTINGS" "$SETTINGS.bak-design-skills"
  names | python3 -c '
import json, os, sys
path, mode = sys.argv[1], sys.argv[2]
data = json.load(open(path))
overrides = data.get("skillOverrides", {})
for name in sys.stdin.read().split():
    if mode == "add":
        if overrides.get(name) in (None, "name-only"):
            overrides[name] = "off"
    elif overrides.get(name) in ("off", "name-only"):
        del overrides[name]
if overrides:
    data["skillOverrides"] = overrides
else:
    data.pop("skillOverrides", None)
tmp = path + ".tmp"
with open(tmp, "w") as out:
    json.dump(data, out, indent=2, ensure_ascii=False)
    out.write("\n")
os.replace(tmp, path)' "$SETTINGS" "$1"
}

if [ "${1:-}" = "--uninstall" ]; then
  for name in $(names); do
    if [ -f "$SKILLS_DIR/$name/.source" ]; then rm -rf "${SKILLS_DIR:?}/$name"; fi
  done
  settings remove
  echo "removed: design skills in $SKILLS_DIR and their skillOverrides entries"
  exit 0
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
mkdir -p "$SKILLS_DIR"

printf '%s\n' "$SOURCES" | while IFS='|' read -r repo commit licence licence_file path name; do
  [ -n "$name" ] || continue
  clone="$TMP/${repo//\//__}"
  if [ ! -d "$clone" ]; then
    git init -q "$clone"
    git -C "$clone" remote add origin "https://github.com/$repo.git"
    git -C "$clone" fetch -q --depth 1 origin "$commit"
    git -C "$clone" -c advice.detachedHead=false checkout -q FETCH_HEAD
    [ "$(git -C "$clone" rev-parse HEAD)" = "$commit" ] || { echo "$repo: not at $commit" >&2; exit 1; }
  fi
  [ -f "$clone/$path/SKILL.md" ] || { echo "$repo: no SKILL.md in $path" >&2; exit 1; }

  target="$SKILLS_DIR/$name"
  if [ -e "$target" ] && [ ! -f "$target/.source" ]; then
    echo "kept existing $target (not installed by this script)" >&2
    continue
  fi
  rm -rf "$target.new"
  cp -r "$clone/$path" "$target.new"
  # impeccable: without scripts/ its launcher is absent and the skill follows its own
  # "Launcher unavailable" path (reads the project context directly). No binary is fetched or run.
  if [ "$name" = "impeccable" ] && [ "${IMPECCABLE_ENGINE:-}" != "1" ]; then
    rm -rf "$target.new/scripts"
  fi
  if [ "$licence_file" != "-" ] && [ ! -e "$target.new/$(basename "$licence_file")" ]; then
    cp "$clone/$licence_file" "$target.new/"
  fi
  if [ "$name" = "impeccable" ]; then cp "$clone/NOTICE.md" "$target.new/"; fi
  cat >"$target.new/.source" <<EOF
repository: https://github.com/$repo
commit: $commit
path: $path
licence: $licence
installed-by: scripts/install-design-skills.sh (Slouchcat)
EOF
  rm -rf "$target"
  mv "$target.new" "$target"
  echo "installed $name ($repo@${commit:0:10})"
done

settings add
echo "skills ready in $SKILLS_DIR; new Slouchcat sessions list them by name (.claude/settings.json)"
