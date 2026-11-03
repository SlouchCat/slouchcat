#!/usr/bin/env bash
# Prints the import paths of the server packages a change touches, for `task server:check:fast`.
#
# A package is touched when a file in its directory -- or in a directory below it that is no
# package of its own, such as testdata/ -- differs from the merge base with BASE: committed,
# staged, unstaged or new and not ignored. The packages of this module that import a touched one
# directly, in their code or in their tests, are added; their importers are not, which is what
# keeps this fast. A change to go.mod or go.sum touches every package.
#
# Prints nothing when no package is touched.
#
# Usage: scripts/changedpkgs.sh [BASE]   (default origin/main)
set -euo pipefail

cd "$(dirname "$0")/.."
base=${1:-origin/main}
fork=$(git merge-base "$base" HEAD)

changed=$({
  git diff --name-only --relative "$fork" -- .
  git ls-files --others --exclude-standard
} | LC_ALL=C sort -u)
[ -n "$changed" ] || exit 0

module=$(go list -m)
# "<import path> <its imports, test imports and external test imports>", one package per line.
graph=$(go list -e -f '{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./...)

declare -A is_package
while read -r pkg _; do is_package[$pkg]=1; done <<<"$graph"

# The package of each changed file: its directory, or the nearest parent directory that is one.
touched=""
while read -r file; do
  case "$file" in
    go.mod | go.sum)
      printf '%s\n' "${!is_package[@]}" | LC_ALL=C sort
      exit 0
      ;;
  esac
  dir=$(dirname "$file")
  while [ "$dir" != "." ]; do
    if [ -n "${is_package[$module/$dir]:-}" ]; then
      touched+="$module/$dir"$'\n'
      break
    fi
    dir=$(dirname "$dir")
  done
done <<<"$changed"
[ -n "$touched" ] || exit 0

{
  printf '%s' "$touched"
  # Through the environment: awk -v would choke on the newlines.
  printf '%s\n' "$graph" | TOUCHED=$touched awk '
    BEGIN { n = split(ENVIRON["TOUCHED"], t, "\n"); for (i = 1; i <= n; i++) if (t[i] != "") hit[t[i]] = 1 }
    { for (i = 2; i <= NF; i++) if ($i in hit) { print $1; next } }'
} | LC_ALL=C sort -u
