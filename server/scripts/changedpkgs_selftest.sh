#!/usr/bin/env bash
# Checks scripts/changedpkgs.sh against a throwaway repository (`task server:check:fast:selftest`).
#
# The repository is built in a temporary directory: a small module below server/ with the same
# layout as the real one where it matters (internal/store, internal/feed, internal/api, testdata/
# and api/ at the top) and no dependency, so the test needs neither the network nor the module
# cache. Each case starts from the same base commit, changes something and compares the printed
# packages with the expected ones.
set -euo pipefail

script=$(cd "$(dirname "$0")" && pwd)/changedpkgs.sh
go_version=$(awk '$1 == "go" { print $2; exit }' "$(dirname "$0")/../go.mod")
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

export GOWORK=off GOFLAGS= GOPROXY=off GOTOOLCHAIN=local GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
repo=$work/repo
mod=example.com/m

pkg() { # pkg <dir below server/> [import ...]
  local dir=$repo/server/$1 name imports="" i
  name=$(basename "$1")
  shift
  for i in "$@"; do imports+="import _ \"$mod/$i\""$'\n'; done
  mkdir -p "$dir"
  printf 'package %s\n\n%s' "$name" "$imports" >"$dir/$name.go"
}

mkdir -p "$repo/server/scripts" "$repo/testdata/db" "$repo/testdata/podcasts" "$repo/api"
git -C "$repo" init -q -b main
git -C "$repo" config user.email selftest@example.com
git -C "$repo" config user.name selftest
printf 'module %s\n\ngo %s\n' "$mod" "$go_version" >"$repo/server/go.mod"
cp "$script" "$repo/server/scripts/changedpkgs.sh"
pkg a
pkg b a
pkg c
pkg internal/store
pkg internal/feed
pkg internal/api internal/store
pkg internal/admin internal/store
mkdir -p "$repo/server/a/testdata" "$repo/server/c/testdata"
echo x >"$repo/server/a/testdata/kept.txt"
echo x >"$repo/server/c/testdata/moved.txt"
echo "-- M1" >"$repo/testdata/db/M1.sql"
echo feed >"$repo/testdata/podcasts/feed.xml.tmpl"
echo "openapi: 3.1.0" >"$repo/api/openapi.yaml"
git -C "$repo" add -A
git -C "$repo" commit -q -m base
git -C "$repo" tag base

failed=0
check() { # check <name> <expected packages, space-separated>
  local got want
  got=$(cd "$repo/server" && scripts/changedpkgs.sh base | sed "s|^$mod/||" | tr '\n' ' ' | sed 's/ $//')
  want=$2
  if [ "$got" = "$want" ]; then
    echo "ok   $1"
  else
    echo "FAIL $1: want '$want', got '$got'" >&2
    failed=1
  fi
  git -C "$repo" reset -q --hard base
  git -C "$repo" clean -q -fdx
}

check "no change" ""

echo y >"$repo/server/a/testdata/größe ü.txt"
check "new file with a non-ASCII name (untracked)" "a b"

echo y >"$repo/server/a/testdata/größe ü.txt"
git -C "$repo" add -A
git -C "$repo" commit -q -m non-ascii
check "committed file with a non-ASCII name" "a b"

git -C "$repo" mv server/c/testdata/moved.txt server/a/testdata/moved.txt
git -C "$repo" commit -q -m rename
check "rename across packages touches both" "a b c"

git -C "$repo" rm -q -r server/a
git -C "$repo" commit -q -m "delete a"
check "deleted package still finds its importers" "b"

echo "-- M2" >"$repo/testdata/db/M2.sql"
check "frozen database state outside server/" "internal/admin internal/api internal/store"

echo changed >>"$repo/testdata/podcasts/feed.xml.tmpl"
git -C "$repo" commit -q -am feed
check "podcast feed template outside server/" "internal/feed"

echo "info: {}" >>"$repo/api/openapi.yaml"
check "OpenAPI contract outside server/" "internal/api"

echo "readme" >"$repo/README.md"
check "file outside server/ that no test reads" ""

echo "// x" >>"$repo/server/go.mod"
check "go.mod touches everything" "a b c internal/admin internal/api internal/feed internal/store"

exit "$failed"
