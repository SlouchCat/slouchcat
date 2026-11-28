#!/usr/bin/env bash
# Self-test of affected-jobs.sh (task ci:workflows), in two parts:
#
#  1. A table: paths -> the groups that must run. It pins the map, including the fail-safe cases.
#  2. The map against the code: every file a test or a build reads across a module boundary
#     (Go tests of server/ and relay/, the Gradle builds of android/, the e2e helpers of web/) is
#     found by a search, and a change of that file must run the reading module's jobs. A new
#     reading the map does not cover turns this red, before a pull request skips a job it needs.
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
expect "a mixed pull request" "server android android_server web e2e image" server/x.go android/y.kt web/z.ts docs/x.md
expect "an Android-only pull request leaves the e2e path out" "android android_server" android/app-mobile/version.properties docs/dev/CURRENT_STATUS.md

# --- 2. the map against the files read across modules ---------------------------------------------
# needs <reading group> <path that is read>: a change of that path must run the group.
needs() {
  local group=$1 path=$2 got
  # A directory stands for any file in it; a file (a dot in its last part) for itself.
  case "${path##*/}" in *.*) ;; *) path=$path/probe ;; esac
  got=" $(run "$path") "
  if [[ "$got" != *" $group "* ]]; then
    echo "FAIL $path is read by $group (found by the search), but a change of it does not run $group"
    failures=$((failures + 1))
  fi
}
cd "$root"
edges=0
# Go tests of the server and the relay: "../../../testdata/x", filepath.Join("..", "..", "api", ...).
for module in server relay; do
  group=$module
  while IFS= read -r target; do
    [ -n "$target" ] || continue
    needs "$group" "$target"
    edges=$((edges + 1))
  done < <(grep -rhoE '"(\.\./)+(api|web|testdata|deploy|packages|android|relay|server|scripts)/[A-Za-z0-9_./-]*"|filepath\.Join\(("\.\.", *)+"(api|web|testdata|deploy|packages|android|relay|server|scripts)"(, *"[A-Za-z0-9_.-]+")*' \
    "$module" --include='*_test.go' --include='*.go' |
    sed -E 's/filepath\.Join\(("\.\.", *)+//; s/"//g; s/, */\//g; s#^(\.\./)+##' |
    awk -F/ '{ if ($1 == "web") print; else if ($1 == "testdata" || $1 == "packages") print $1 "/" $2; else print $1 }' | sort -u)
done
# The Android builds: dir("../packages/design-tokens"), file("../api/openapi.yaml"), "web/src/locales".
while IFS= read -r target; do
  [ -n "$target" ] || continue
  needs android "$target"
  edges=$((edges + 1))
done < <(grep -rhoE '"(\.\./)*(api|web|testdata|packages|deploy|relay|server|scripts)/[A-Za-z0-9_./-]*"' android --include='*.kts' |
  sed -E 's/"//g; s#^(\.\./)+##' | awk -F/ '{ if ($1 == "web") print; else print $1 "/" $2 }' | sort -u)
# The e2e helpers: ../../testdata/media, ../../server/bin/slouchcat-e2e.
while IFS= read -r target; do
  [ -n "$target" ] || continue
  needs e2e "$target"
  edges=$((edges + 1))
done < <(grep -rhoE "\.\./\.\./(api|testdata|server|relay|packages|deploy|scripts)/[A-Za-z0-9_./-]*" web/e2e web/playwright.config.ts |
  sed -E 's#^(\.\./)+##' | awk -F/ '{ print $1 "/" $2 }' | sort -u)
# The Android contract and emulator jobs build and start the server and the relay.
grep -q ':server:build' android/Taskfile.yml && { needs android_server server; edges=$((edges + 1)); }
grep -q ':relay:build' android/Taskfile.yml && { needs android_server relay; edges=$((edges + 1)); }
echo "     $edges readings across modules checked against the map"
[ "$edges" -ge 10 ] || { echo "FAIL the search found only $edges readings; it no longer sees the code"; failures=$((failures + 1)); }

if [ "$failures" -gt 0 ]; then
  echo "affected-jobs.test.sh: $failures case(s) failed" >&2
  exit 1
fi
echo "affected-jobs.test.sh: all cases passed"
