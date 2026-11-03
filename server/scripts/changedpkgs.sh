#!/usr/bin/env bash
# Prints the import paths of the server packages a change touches, for `task server:check:fast`.
#
# A package is touched when a file in its directory -- or in a directory below it that is no
# package of its own, such as testdata/ -- differs from the merge base with BASE: committed,
# staged, unstaged or new and not ignored. The packages of this module that import a touched one
# directly, in their code or in their tests, are added; their importers are not, which is what
# keeps this fast. A change to go.mod or go.sum touches every package.
#
# Three cases need more than the directory of a changed file:
#
#  - A rename counts twice (--no-renames): the package the file left is touched as well as the
#    one it went to.
#  - A deleted package is gone from `go list`, but the packages that still import it are not, and
#    those are the ones that break. Its directory is therefore looked up among the imports all the
#    same; it is not printed itself, because `go test` cannot run a package that does not exist.
#  - Some tests read files outside their package directory, most of them outside server/. Those
#    files are mapped to the reading package in the case statement below (outside_package). Not
#    mapped: testdata/media, which is generated and ignored by git, so no diff ever shows it.
#
# File names are read NUL-separated, so names with spaces or non-ASCII characters (which git would
# otherwise quote) arrive as they are.
#
# Prints nothing when no package is touched. scripts/changedpkgs_selftest.sh checks these rules
# (`task server:check:fast:selftest`).
#
# Usage: scripts/changedpkgs.sh [BASE]   (default origin/main)
set -euo pipefail

cd "$(dirname "$0")/.."
base=${1:-origin/main}
fork=$(git merge-base "$base" HEAD)
top=$(git rev-parse --show-toplevel)
# This module's directory relative to the repository root, with a trailing slash ("server/").
prefix=$(git rev-parse --show-prefix)

module=$(go list -m)
# "<import path> <its imports, test imports and external test imports>", one package per line.
graph=$(go list -e -f '{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./...)

declare -A is_package
while read -r pkg _; do is_package[$pkg]=1; done <<<"$graph"

# The package below server/ that reads a file outside its own directory, or nothing. Paths are
# relative to the repository root.
outside_package() {
  case "$1" in
    testdata/db/*) echo internal/store ;;
    testdata/podcasts/*) echo internal/feed ;;
    testdata/webhooks/* | api/openapi.yaml) echo internal/api ;;
    "${prefix}migrations/00021_spotlight.sql") echo internal/api ;;
  esac
}

touched=""
# Looked up among the imports, but not printed: directories that are no package (any more).
gone=""
while IFS= read -r -d '' file; do
  case "$file" in
    "${prefix}go.mod" | "${prefix}go.sum")
      printf '%s\n' "${!is_package[@]}" | LC_ALL=C sort
      exit 0
      ;;
  esac
  extra=$(outside_package "$file")
  [ -z "$extra" ] || touched+="$module/$extra"$'\n'
  case "$file" in
    "$prefix"*) file=${file#"$prefix"} ;;
    *) continue ;;
  esac
  # The package of the file: its directory, or the nearest parent directory that is one.
  dir=$(dirname "$file")
  while [ "$dir" != "." ]; do
    if [ -n "${is_package[$module/$dir]:-}" ]; then
      touched+="$module/$dir"$'\n'
      break
    fi
    [ -d "$dir" ] || gone+="$module/$dir"$'\n'
    dir=$(dirname "$dir")
  done
done < <({
  git -C "$top" diff --name-only --no-renames -z "$fork"
  git -C "$top" ls-files --others --exclude-standard -z
} | LC_ALL=C sort -z -u)
[ -n "$touched$gone" ] || exit 0

{
  printf '%s' "$touched"
  # Through the environment: awk -v would choke on the newlines.
  printf '%s\n' "$graph" | TOUCHED=$touched$gone awk '
    BEGIN { n = split(ENVIRON["TOUCHED"], t, "\n"); for (i = 1; i <= n; i++) if (t[i] != "") hit[t[i]] = 1 }
    { for (i = 2; i <= NF; i++) if ($i in hit) { print $1; next } }'
} | LC_ALL=C sort -u | while read -r pkg; do
  [ -z "${is_package[$pkg]:-}" ] || echo "$pkg"
done
