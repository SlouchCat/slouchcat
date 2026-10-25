#!/usr/bin/env bash
# Resolves one group of server test packages for CI (`task server:test GROUP=...`).
#
# `go test -race ./...` is the whole server suite and takes about 19 minutes on a four-core
# runner -- more than the rest of the workflow put together. CI therefore runs the groups named in
# testgroups.txt as parallel jobs. Locally nothing changes: `task server:test` without GROUP still
# runs every package in one go, and so does `task ci`.
#
# The groups must never lose a package. Only the heavy packages are named in testgroups.txt; the
# group "rest" is computed as everything `go list ./...` reports and the file does not name, so a
# new package is tested by "rest" from its first day without anyone having to remember this file.
# `--check` additionally fails when the file names a package that no longer exists, which is how a
# rename turns into a red lint instead of a group that quietly tests nothing.
#
# Usage:
#   scripts/testgroups.sh            -> ./...            (every package)
#   scripts/testgroups.sh all        -> ./...
#   scripts/testgroups.sh 1          -> the import paths of group 1
#   scripts/testgroups.sh rest       -> every package no group names
#   scripts/testgroups.sh --check    -> prints the groups, fails on a package that does not exist
set -euo pipefail

cd "$(dirname "$0")/.."
file=testgroups.txt

# "<group> <package suffix>", comments and empty lines dropped.
entries() {
  sed -e 's/#.*//' "$file" | awk 'NF >= 2 { print $1, $2 }'
}

module=$(go list -m)
all_suffixes() { go list ./... | sed "s|^${module}/||;s|^${module}\$|.|"; }

named_suffixes() { entries | awk '{ print $2 }' | sort -u; }

group_suffixes() {
  local wanted=$1
  entries | awk -v g="$wanted" '$1 == g { print $2 }'
}

rest_suffixes() {
  comm -23 <(all_suffixes | sort -u) <(named_suffixes)
}

as_paths() {
  while read -r suffix; do
    [ -n "$suffix" ] || continue
    if [ "$suffix" = "." ]; then echo "$module"; else echo "${module}/${suffix}"; fi
  done
}

check() {
  local missing=0 suffix
  while read -r suffix; do
    [ -n "$suffix" ] || continue
    if ! all_suffixes | grep -qxF "$suffix"; then
      echo "testgroups.txt names '$suffix', which is not a package of this module" >&2
      missing=1
    fi
  done < <(named_suffixes)
  [ "$missing" -eq 0 ] || exit 1
  local group
  for group in $(entries | awk '{ print $1 }' | sort -u); do
    echo "group $group: $(group_suffixes "$group" | tr '\n' ' ')"
  done
  echo "group rest: $(rest_suffixes | tr '\n' ' ')"
}

case "${1:-all}" in
  --check) check ;;
  all) echo "./..." ;;
  rest) rest_suffixes | as_paths | tr '\n' ' ' ;;
  *)
    paths=$(group_suffixes "$1" | as_paths | tr '\n' ' ')
    if [ -z "${paths// /}" ]; then
      echo "testgroups.txt has no group '$1'" >&2
      exit 1
    fi
    echo "$paths"
    ;;
esac
