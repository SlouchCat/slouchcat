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
expect "a mixed pull request" "server android android_server web e2e image" server/x.go android/y.kt web/z.ts docs/x.md
expect "an Android-only pull request leaves the e2e path out" "android android_server" android/app-mobile/version.properties docs/dev/CURRENT_STATUS.md

# --- 2. the map against the files read across modules ---------------------------------------------
# A search for every path a test, a build or a script of the CI names across a module boundary, and
# for each the job group that runs the reading file: a change of that path must run that group.
#
# The search sees relative paths that leave a module and name a top-level path of the repository
# ("../../testdata/x", File("../../web/public/icon.svg"), node ../web/e2e/x.mjs),
# filepath.Join("..", "..", "testdata", ...), two shell idioms (`$(cd ../x && pwd)/file` and
# `go -C ../server tool`), and, in files that compute the repository's root
# (join(import.meta.dirname, '../../..'), slouchcat.repo), every string that starts with a
# top-level directory ('docs/store/...'), and the sources of COPY and ADD in the Dockerfiles. It
# does not see a path put together from variables in another form, or one a tool reads from its
# own configuration; ADR 0013, addendum M9, names that rest.
cd "$root"

# reader_group <file>: the job group that runs <file>; "-" for files the CI does not run.
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

# needs <reading group> <path that is read> <reading file>: a change of that path must run the group.
needs() {
  local group=$1 path=$2 reader=$3 got
  # A directory stands for any file in it, a file for itself; a path that is not there (a build
  # output such as server/bin/slouchcat) is a file when its last part has a dot.
  if [ -d "$path" ] || { [ ! -e "$path" ] && [[ "${path##*/}" != *.* ]]; }; then
    path=$path/probe
  fi
  got=" $(run "$path") "
  if [[ "$got" != *" $group "* ]]; then
    echo "FAIL $path is read by $reader ($group), but a change of it does not run $group"
    failures=$((failures + 1))
  fi
}

tops=$(git ls-files | cut -d/ -f1 | sort -u | grep -v '^[.]' | sed 's/[.]/[.]/g' | paste -sd'|')
seg='[A-Za-z0-9_.@{}$-]+'
up="(\.\./)+($tops)(/$seg)*"
search="$up( \&\& pwd\)/$seg)?|go -C (\.\./)+server tool|filepath\.Join\((\"\.\.\", *)+\"($tops)\"(, *\"[A-Za-z0-9_. ()-]+\")*"
roothelper="import\.meta\.dirname, '\.\./\.\.(/\.\.)*'|slouchcat\.repo"
readers=('server/*.go' 'relay/*.go' 'android/*.kt' 'android/*.kts' 'web/*.ts' 'web/*.tsx' 'web/*.mjs'
  'packages/*.ts' 'packages/*.mjs' 'scripts/*.sh' 'scripts/*.ts' 'scripts/*.mjs' '*/Taskfile.yml'
  ':!:*/generated/*' ':!:*/gen/*' ':!:*/dist/*' ':!:*.d.ts')

found() {
  {
    git grep -oE "$search" -- "${readers[@]}" || true
    git grep -lE "$roothelper" -- "${readers[@]}" | while IFS= read -r f; do
      git grep -oE "['\"]($tops)/$seg(/$seg)*['\"]" -- "$f" || true
    done
    # The sources the Dockerfiles copy from the build context, the repository's root.
    git ls-files 'deploy/*Dockerfile*' | while IFS= read -r f; do
      awk -v f="$f" '($1 == "COPY" || $1 == "ADD") && !/--from=/ {
        for (i = 2; i < NF; i++) if ($i !~ /^--/) print f ":" $i
      }' "$f"
    done
  } | sed -E \
    -e 's/:/\t/' \
    -e 's/\tfilepath\.Join\(("\.\.", *)+/\t/' -e 's/", *"/\//g' -e "s/[\"']//g" \
    -e 's#\tgo -C ((\.\./)+)server tool#\t\1server/go.mod#' \
    -e 's# && pwd\)/#/#' \
    -e 's#\t(\.\./)+#\t#' -e 's#/$##' |
    sort -u
}

edges=0
while IFS=$'\t' read -r reader target; do
  [ -n "$target" ] || continue
  [ "$reader"$'\t'"$target" != "$written" ] || continue
  group=$(reader_group "$reader")
  case "$group" in
    -) continue ;;
    '?')
      echo "FAIL $reader reads $target across modules, but reader_group knows no job group for $reader"
      failures=$((failures + 1))
      continue
      ;;
  esac
  needs "$group" "$target" "$reader"
  edges=$((edges + 1))
done < <(found)
# The Android contract and emulator jobs build and start the server and the relay (task names, no paths).
grep -q ':server:build' android/Taskfile.yml && { needs android_server server android/Taskfile.yml; edges=$((edges + 1)); }
grep -q ':relay:build' android/Taskfile.yml && { needs android_server relay android/Taskfile.yml; edges=$((edges + 1)); }
echo "     $edges readings across modules checked against the map"
[ "$edges" -ge 60 ] || { echo "FAIL the search found only $edges readings; it no longer sees the code"; failures=$((failures + 1)); }

if [ "$failures" -gt 0 ]; then
  echo "affected-jobs.test.sh: $failures case(s) failed" >&2
  exit 1
fi
echo "affected-jobs.test.sh: all cases passed"
