#!/usr/bin/env bash
# Resolves the server test packages for one CI job (`task server:test GROUP=...`) and, with
# --matrix, the list of jobs itself.
#
# `go test -race ./...` is the whole server suite and takes about 19 minutes on a four-core
# runner -- more than the rest of the workflow put together. CI therefore runs the groups named in
# testgroups.txt as parallel jobs. Locally nothing changes: `task server:test` without GROUP still
# runs every package in one go, and so does `task ci`.
#
# The groups must never lose a package, and that has two halves:
#
#  - No package without a group. Only the heavy packages are named in testgroups.txt; the group
#    "rest" is computed as everything `go list ./...` reports and the file does not name, so a new
#    package is tested by "rest" from its first day. `--check` additionally fails when the file
#    names a package that no longer exists, which turns a rename into a red lint instead of a group
#    that quietly tests nothing.
#  - No group without a job. The workflow does not list the groups; it asks `--matrix` for them and
#    builds its matrix from the answer (ci.yml, job "server-groups"). A new group therefore gets a
#    job by itself. Writing the list into ci.yml as well would be the same name in two places, and
#    the half that was forgotten would be the half that runs no tests.
#
# Usage:
#   scripts/testgroups.sh            -> ./...            (every package)
#   scripts/testgroups.sh all        -> ./...
#   scripts/testgroups.sh 1          -> the import paths of group 1
#   scripts/testgroups.sh rest       -> every package no group names
#   scripts/testgroups.sh --matrix   -> ["1","2","rest"], the CI matrix; needs no Go toolchain
#   scripts/testgroups.sh --check    -> prints the groups, fails on a package that does not exist
set -euo pipefail

cd "$(dirname "$0")/.."
file=testgroups.txt

# "<group> <package suffix>", comments and empty lines dropped.
entries() {
  sed -e 's/#.*//' "$file" | awk 'NF >= 2 { print $1, $2 }'
}

group_names() { entries | awk '{ print $1 }' | LC_ALL=C sort -u; }
named_suffixes() { entries | awk '{ print $2 }' | LC_ALL=C sort -u; }
group_suffixes() { entries | awk -v g="$1" '$1 == g { print $2 }'; }

# --matrix is answered from testgroups.txt alone, so the job that builds the matrix needs neither
# Go nor the module cache. Everything below this line does need `go list`.
if [ "${1:-all}" = "--matrix" ]; then
  { group_names; echo rest; } |
    awk 'BEGIN { printf "[" } { printf "%s\"%s\"", (NR > 1 ? "," : ""), $0 } END { print "]" }'
  exit 0
fi

module=$(go list -m)
# Read once: `grep -q` closes the pipe at the first match, the producer dies of SIGPIPE, and with
# `set -o pipefail` the pipeline would report a failure that is none.
all_suffixes=$(go list ./... | sed "s|^${module}/||;s|^${module}\$|.|")

rest_suffixes() {
  # LC_ALL=C on both sides: `comm` compares bytes while `sort` collates by locale, and a locale
  # that ignores "/" would order internal/job/x and internal/jobs differently from comm's view.
  comm -23 <(printf '%s\n' "$all_suffixes" | LC_ALL=C sort -u) <(named_suffixes)
}

as_paths() {
  while read -r suffix; do
    [ -n "$suffix" ] || continue
    if [ "$suffix" = "." ]; then echo "$module"; else echo "${module}/${suffix}"; fi
  done
}

fail_if_empty() {
  local paths=$1 what=$2
  if [ -z "${paths// /}" ]; then
    echo "$what resolves to no package" >&2
    exit 1
  fi
  echo "$paths"
}

check() {
  local missing=0 suffix
  while read -r suffix; do
    [ -n "$suffix" ] || continue
    if ! printf '%s\n' "$all_suffixes" | grep -qxF "$suffix"; then
      echo "testgroups.txt names '$suffix', which is not a package of this module" >&2
      missing=1
    fi
  done < <(named_suffixes)
  [ "$missing" -eq 0 ] || exit 1
  local group
  while read -r group; do
    echo "group $group: $(group_suffixes "$group" | tr '\n' ' ')"
  done < <(group_names)
  echo "group rest: $(rest_suffixes | tr '\n' ' ')"
}

case "${1:-all}" in
  --check) check ;;
  all) echo "./..." ;;
  rest) fail_if_empty "$(rest_suffixes | as_paths | tr '\n' ' ')" "group 'rest'" ;;
  *)
    if ! group_names | grep -qxF "$1"; then
      echo "testgroups.txt has no group '$1'" >&2
      exit 1
    fi
    fail_if_empty "$(group_suffixes "$1" | as_paths | tr '\n' ' ')" "group '$1'"
    ;;
esac
