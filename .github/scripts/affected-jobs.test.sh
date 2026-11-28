#!/usr/bin/env bash
# Self-test of affected-jobs.sh (task ci:workflows), in two parts:
#
#  1. A table: paths -> the groups that must run. It pins the map, including the fail-safe cases.
#  2. The map against the code: every path a test, a build or a script of the CI reads across a
#     module boundary (Go, Kotlin, Gradle, TypeScript, shell and the Taskfiles) is found by a
#     search, and a change of that path must run the job group of the reading file. A new reading
#     the map does not cover turns this red, before a pull request skips a job it needs.
set -euo pipefail

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
root=$(cd "$here/../.." && pwd)
script=$here/affected-jobs.sh
failures=0

run() { printf '%s\n' "$@" | bash "$script" | awk -F= '$2 == "true" { sub(/^run_/, "", $1); printf "%s ", $1 }' | sed 's/ $//'; }
expect() { # expect <name> <expected groups, space separated, in script order> <paths...>
  local name=$1 want=$2 got
  shift 2
  got=$(run "$@")
  if [ "$got" = "$want" ]; then
    echo "ok   $name"
  else
    echo "FAIL $name: want '$want', got '$got'"
    failures=$((failures + 1))
  fi
}

all="server relay android android_server web e2e image"

# --- 1. the table --------------------------------------------------------------------------------
expect "documentation only runs nothing" "" docs/dev/SETUP.md README.md docs/adr/0001-x.md
expect "no input runs everything" "$all"
expect "an unknown path runs everything" "$all" somewhere/new.txt
expect "the workflows run everything" "$all" .github/workflows/ci.yml
expect "a Taskfile runs everything" "$all" web/Taskfile.yml
expect "the contract runs everything" "$all" api/openapi.yaml
expect "the lock file runs everything" "$all" pnpm-lock.yaml
expect "the tool versions run everything" "$all" .tool-versions
expect "server code" "server android_server e2e image" server/internal/api/x.go
expect "server go.sum adds relay" "server relay android_server e2e image" server/go.sum
expect "relay code" "relay android_server" relay/internal/pair/x.go
expect "relay images" "relay" deploy/relay/pair/Dockerfile
expect "android code" "android android_server" android/app-mobile/src/main/kotlin/X.kt
expect "android scripts" "android android_server" scripts/android-e2e.sh
expect "web code" "web e2e image" web/src/features/home/HomePage.tsx
expect "web e2e spec" "web e2e image" web/e2e/home.spec.ts
expect "web markdown is checked by prettier" "web e2e image" web/README.md
expect "web locales reach the Android strings" "android android_server web e2e image" web/src/locales/de.json
expect "the player labels reach the server tests" "server web e2e image" web/src/features/player/labels.ts
expect "design tokens reach Android" "android android_server web e2e image" packages/design-tokens/tokens/color.json
expect "the API client" "web e2e image" packages/api-client/src/index.ts
expect "test data" "server relay android android_server e2e" testdata/crypto/webpush.json
expect "the Dockerfile" "server image" deploy/Dockerfile
expect "the ffmpeg notice is read by the image" "server image" deploy/licenses/FFMPEG-NOTICE.md
expect "the i18n scripts" "web" scripts/i18n/check.ts
expect "the CI failure reader" "server" scripts/ci-failures.sh
expect "the launcher icons are held to the logo by a web test" "android android_server web" android/app-tv/src/main/res/drawable/banner.xml
expect "the store graphics are held to the logo" "web" docs/store/grafiken/app-icon.svg
expect "other store documents are documentation" "" docs/store/listing.md
expect "the app icon is read by the Android launcher test" "android web e2e image" web/public/icon.svg
expect "the e2e mocks run in the Android jobs too" "android_server web e2e image" web/e2e/seerr-server.mjs
expect "an e2e spec does not reach Android" "web e2e image" web/e2e/wishes.spec.ts
expect "the licence is copied into every image" "server relay image" LICENSE
expect "deploy is mapped below its first folder" "relay" deploy/relay/push/Dockerfile
expect "a README in the test data is not documentation" "server relay android android_server e2e" testdata/db/README.md
expect "a README in the server is not documentation" "server android_server e2e image" server/queries/scoped/README.md
expect "a README in the scripts is not documentation" "$all" scripts/README.md
expect "release notes at the top level are documentation" "" release-notes-1.0.0.txt
expect "release notes inside a module are not" "web e2e image" web/release-notes.txt
expect "a folder named like release notes is not documentation" "$all" release-notes/build.sh
expect "a mixed pull request" "server android android_server web e2e image" server/x.go android/y.kt web/z.ts docs/x.md
expect "an Android-only pull request leaves the e2e path out" "android android_server" android/app-mobile/version.properties docs/dev/CURRENT_STATUS.md

# --- 2. the map against the files read across modules ---------------------------------------------
# A search for every path a test, a build or a script of the CI names across a module boundary, and
# for each the job groups that run the reading file: a change of that path must run those groups.
#
# The search sees relative paths that leave a module and name a top-level path of the repository
# ("../../testdata/x", File("../../web/public/icon.svg"), node ../web/e2e/x.mjs),
# filepath.Join("..", "..", "testdata", ...), two shell idioms (`$(cd ../x && pwd)/file` and
# `go -C ../server tool`), and, in files that compute the repository's root
# (join(import.meta.dirname, '../../..'), resolve(dirname(fileURLToPath(import.meta.url)), '../..'),
# projectDirectory.dir(".."), slouchcat.repo), every string that starts with a top-level directory
# ('docs/store/...'), and the sources of COPY and ADD in the Dockerfiles. It does not see a path put
# together from variables in another form, or one a tool reads from its own configuration; ADR
# 0013, addendum M9, names that rest. The anchors below hold the search itself: one known reading
# per form, so a search that stops seeing a form turns this red instead of passing with fewer
# readings.
cd "$root"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# reader_group <file>: the job group that runs <file>; "-" for files the CI does not run. A file
# another reading file names (scripts/android-helpers.sh starts web/e2e/radio-server.mjs) also gets
# the groups of that file, below.
reader_group() {
  case "$1" in
    server/*) echo server ;;
    relay/*) echo relay ;;
    # The contract tests and the emulator flows run against bin/slouchcat and the relay; the
    # Taskfile's readings across modules are those tasks' (server, relay, scripts/android-*).
    android/*/src/contractTest/* | android/*/src/androidTest/* | android/Taskfile.yml) echo android_server ;;
    android/*) echo android ;;
    web/e2e/* | web/playwright.config.ts) echo e2e ;;
    web/* | packages/* | scripts/i18n/*) echo web ;;
    scripts/android-*) echo android_server ;;
    scripts/ci-failures*) echo server ;;
    # The images: the relay services in the job "relay", the server and the updater in "image".
    deploy/relay/*) echo relay ;;
    deploy/*) echo image ;;
    # Run by hand: the test server on Proxmox, the design toolbox, the store's screenshots.
    scripts/proxmox/* | scripts/design-tools/* | scripts/store-screenshots.sh) echo - ;;
    *) echo '?' ;;
  esac
}
# Paths the search finds that the reading file writes, not reads: <file><TAB><path>.
written=$'web/Taskfile.yml\tserver/internal/webassets/dist'

# The patterns of the map's case, for a reading of a directory: a change anywhere below it must run
# the reader's groups, also where a narrower line of the map decides (web/ and web/src/locales/*).
# The sed address matches the literal text `case "$path" in` of the map.
# shellcheck disable=SC2016
mapfile -t labels < <(sed -n '/^  case "\$path" in$/,/^  esac$/p' "$script" | grep -v '^ *#' |
  sed -e ':a' -e '/\\$/N; s/\\\n//; ta' | grep -oE '^ +[^ ][^)]*\)' | sed -E 's/\)$//' |
  tr '|' '\n' | sed -E 's/^ +| +$//g' | grep -v '^$')
[ "${#labels[@]}" -ge 20 ] || { echo "FAIL read only ${#labels[@]} patterns from the map's case"; failures=$((failures + 1)); }

# probes <path>: the paths whose change stands for a change of <path>. A directory stands for a
# file in it and for every pattern of the map below it, a file for itself; a path that is not there
# (a build output such as server/bin/slouchcat) is a file when its last part has a dot.
probes() {
  local path=$1 label
  if [ -d "$path" ] || { [ ! -e "$path" ] && [[ "${path##*/}" != *.* ]]; }; then
    echo "$path/probe"
    for label in "${labels[@]}"; do
      if [[ "$label" == "$path/"* ]]; then echo "${label//\*/probe}"; fi
    done
  else
    echo "$path"
  fi
}

# needs <group> <path that is read> <reading file>: a change of that path must run the group.
needs() {
  local group=$1 path=$2 reader=$3 probe got
  while IFS= read -r probe; do
    got=" $(run "$probe") "
    if [[ "$got" != *" $group "* ]]; then
      echo "FAIL $probe (for $path, read by $reader, $group) does not run $group"
      failures=$((failures + 1))
    fi
  done < <(probes "$path")
}

tops=$(git ls-files | cut -d/ -f1 | sort -u | grep -v '^[.]' | sed 's/[.]/[.]/g' | paste -sd'|')
seg='[A-Za-z0-9_.@{}$-]+'
up="(\.\./)+($tops)(/$seg)*"
search="$up( \&\& pwd\)/$seg)?|go -C (\.\./)+server tool|filepath\.Join\((\"\.\.\", *)+\"($tops)\"(, *\"[A-Za-z0-9_. ()-]+\")*"
roothelper="import\.meta\.dirname, '\.\./\.\.(/\.\.)*'|fileURLToPath\(import\.meta\.url\)\), *'\.\./\.\.(/\.\.)*'|projectDirectory\.dir\(\"\.\.\"\)|slouchcat\.repo"
readers=('server/*.go' 'relay/*.go' 'android/*.kt' 'android/*.kts' 'web/*.ts' 'web/*.tsx' 'web/*.mjs'
  'web/*.js' 'packages/*.ts' 'packages/*.mjs' 'packages/*.js' 'scripts/*.ts' 'scripts/*.mjs'
  'scripts/*.sh' 'server/*.sh' 'relay/*.sh' 'android/*.sh' 'web/*.sh' '*/Taskfile.yml'
  ':!:*/generated/*' ':!:*/gen/*' ':!:*/dist/*' ':!:*.d.ts')

# git grep that fails on an error (exit 2 and up), not on "nothing found" (exit 1).
grep_ok() {
  local rc=0
  git grep "$@" || rc=$?
  [ "$rc" -le 1 ] || { echo "FAIL git grep $* ended with exit $rc" >&2; return 1; }
}

found() {
  local f helpers
  grep_ok -oE "$search" -- "${readers[@]}" >"$tmp/raw" || return 1
  helpers=$(grep_ok -lE "$roothelper" -- "${readers[@]}") || return 1
  while IFS= read -r f; do
    [ -n "$f" ] || continue
    grep_ok -oE "['\"]($tops)/$seg(/$seg)*['\"]" -- "$f" >>"$tmp/raw" || return 1
  done <<<"$helpers"
  # The sources the Dockerfiles copy from the build context, the repository's root.
  while IFS= read -r f; do
    awk -v f="$f" '($1 == "COPY" || $1 == "ADD") && !/--from=/ {
      for (i = 2; i < NF; i++) if ($i !~ /^--/) print f ":" $i
    }' "$f" >>"$tmp/raw" || return 1
  done < <(git ls-files 'deploy/*Dockerfile*')
  sed -E \
    -e 's/:/\t/' \
    -e 's/\tfilepath\.Join\(("\.\.", *)+/\t/' -e 's/", *"/\//g' -e "s/[\"']//g" \
    -e 's#\tgo -C ((\.\./)+)server tool#\t\1server/go.mod#' \
    -e 's# && pwd\)/#/#' \
    -e 's#\t(\.\./)+#\t#' -e 's#/$##' "$tmp/raw" | sort -u
}

if ! found >"$tmp/found"; then
  echo "FAIL the search for readings across modules failed"
  failures=$((failures + 1))
fi

# One known reading per form the search must see.
anchors=(
  $'server/internal/trackpick/languages_test.go\tweb/src/features/player/labels.ts'
  $'server/internal/library/probe_test.go\ttestdata/media/Movies'
  $'android/app-mobile/src/test/kotlin/com/slouchcat/app/mobile/LauncherIconTest.kt\tweb/public/icon.svg'
  $'android/core/strings/build.gradle.kts\tweb/src/locales'
  $'web/src/ui/logoMark.test.ts\tdocs/store/grafiken/app-icon.svg'
  $'scripts/i18n/lib.ts\tweb/src/locales'
  $'scripts/android-helpers.sh\tweb/e2e/seerr-server.mjs'
  $'scripts/android-helpers.sh\tweb/e2e/radio-server.mjs'
  $'relay/Taskfile.yml\tserver/go.mod'
  $'deploy/relay/pair/Dockerfile\tLICENSE'
)
for anchor in "${anchors[@]}"; do
  grep -qxF "$anchor" "$tmp/found" || {
    echo "FAIL the search no longer finds the reading ${anchor/$'\t'/ -> }; it lost a form (or the file moved: update the anchor)"
    failures=$((failures + 1))
  }
done

# The groups of every reading file, then those of the files it runs, until nothing changes.
declare -A groups_of=() by_hand=()
edge_list=()
while IFS=$'\t' read -r reader target; do
  [ -n "$target" ] || continue
  [ "$reader"$'\t'"$target" != "$written" ] || continue
  edge_list+=("$reader"$'\t'"$target")
  if [ -z "${groups_of[$reader]+set}" ]; then
    case "$(reader_group "$reader")" in
      -) groups_of[$reader]=""; by_hand[$reader]=1 ;;
      '?') groups_of[$reader]="" ;;
      *) groups_of[$reader]=$(reader_group "$reader") ;;
    esac
  fi
done <"$tmp/found"
changed=1
while [ "$changed" = 1 ]; do
  changed=0
  for edge in "${edge_list[@]}"; do
    reader=${edge%%$'\t'*} target=${edge#*$'\t'}
    [ -n "${groups_of[$target]+set}" ] || continue
    for group in ${groups_of[$reader]}; do
      if [[ " ${groups_of[$target]} " != *" $group "* ]]; then
        groups_of[$target]="${groups_of[$target]} $group"
        changed=1
      fi
    done
  done
done

edges=0
for edge in "${edge_list[@]}"; do
  reader=${edge%%$'\t'*} target=${edge#*$'\t'}
  if [ -z "${groups_of[$reader]// /}" ]; then
    [ -n "${by_hand[$reader]:-}" ] && continue
    echo "FAIL $reader reads $target across modules, but reader_group knows no job group for $reader"
    failures=$((failures + 1))
    continue
  fi
  for group in ${groups_of[$reader]}; do
    needs "$group" "$target" "$reader"
    edges=$((edges + 1))
  done
done
# The Android contract and emulator jobs build and start the server and the relay (task names, no paths).
grep -q ':server:build' android/Taskfile.yml && { needs android_server server android/Taskfile.yml; edges=$((edges + 1)); }
grep -q ':relay:build' android/Taskfile.yml && { needs android_server relay android/Taskfile.yml; edges=$((edges + 1)); }
echo "     $edges readings across modules checked against the map (${#anchors[@]} anchors, ${#labels[@]} patterns)"
[ "$edges" -ge 100 ] || { echo "FAIL the search found only $edges readings; it no longer sees the code"; failures=$((failures + 1)); }

if [ "$failures" -gt 0 ]; then
  echo "affected-jobs.test.sh: $failures case(s) failed" >&2
  exit 1
fi
echo "affected-jobs.test.sh: all cases passed"
