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
# One package can be split over several groups by test name, when it alone is longer than a job
# should take (internal/store since 26 November 2026, testgroups.txt says why). Every line of such a
# package but one carries a regular expression over the names of its top-level tests; the line
# without one is the package's own "rest". A line with a pattern runs `-run=<pattern>` and skips
# what the lines above it already took, the line without one skips every pattern:
#
#   2   internal/store  ^TestMigration   -> -run=^TestMigration                       internal/store
#   2b  internal/store                   -> -skip=^TestMigration                      internal/store
#
# Each top-level test therefore lands in exactly one group, also a test added tomorrow, by the
# same construction as the group "rest" for packages. `--check` lists the package's tests and
# prints how many each group takes, which must add up to the whole and be more than none per
# group: a pattern that matches nothing would be a job that quietly tests nothing.
#
# Usage:
#   scripts/testgroups.sh            -> ./...            (every package)
#   scripts/testgroups.sh all        -> ./...
#   scripts/testgroups.sh 1          -> the import paths of group 1 (with -run/-skip for a split package)
#   scripts/testgroups.sh rest       -> every package no group names
#   scripts/testgroups.sh --matrix   -> ["1","2","rest"], the CI matrix; needs no Go toolchain
#   scripts/testgroups.sh --check    -> prints the groups, fails on a package that does not exist
set -euo pipefail

cd "$(dirname "$0")/.."
file=testgroups.txt

# "<group> <package suffix> [<test pattern>]", comments and empty lines dropped.
entries() {
  sed -e 's/#.*//' "$file" | awk 'NF >= 2 { print $1, $2, $3 }'
}

group_names() { entries | awk '{ print $1 }' | LC_ALL=C sort -u; }
named_suffixes() { entries | awk '{ print $2 }' | LC_ALL=C sort -u; }
group_suffixes() { entries | awk -v g="$1" '$1 == g { print $2 }'; }
# Packages split by test name, and the patterns of one of them in file order.
split_suffixes() { entries | awk 'NF == 3 { print $2 }' | LC_ALL=C sort -u; }
patterns_of() { entries | awk -v s="$1" '$2 == s && NF == 3 { print $3 }'; }
# Not `grep -q`: it stops reading at the first match, and pipefail would count the SIGPIPE of the
# producer as a failure (see all_suffixes below).
is_split() { [ "$(split_suffixes | grep -cxF "$1")" -gt 0 ]; }

# The go test flags of one line of a split package: its own pattern minus the patterns above it,
# or, for the line without a pattern, minus all of them.
split_flags() {
  local suffix=$1 pattern=$2 before
  if [ -n "$pattern" ]; then
    before=$(patterns_of "$suffix" | awk -v p="$pattern" '$0 == p { found = 1 } !found { print }' | paste -sd'|' -)
    printf -- '-run=%s' "$pattern"
    [ -z "$before" ] || printf -- ' -skip=%s' "$before"
  else
    printf -- '-skip=%s' "$(patterns_of "$suffix" | paste -sd'|' -)"
  fi
}

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
  # Named twice without patterns, a package would run in full in two jobs.
  while read -r suffix; do
    [ -n "$suffix" ] || continue
    if ! is_split "$suffix"; then
      echo "testgroups.txt names '$suffix' twice; split it by test name or name it once" >&2
      missing=1
    fi
  done < <(entries | awk '{ print $2 }' | LC_ALL=C sort | uniq -d)
  [ "$missing" -eq 0 ] || exit 1
  local group
  while read -r group; do
    echo "group $group: $(group_suffixes "$group" | tr '\n' ' ')"
  done < <(group_names)
  echo "group rest: $(rest_suffixes | tr '\n' ' ')"
  while read -r suffix; do
    [ -n "$suffix" ] || continue
    check_split "$suffix"
  done < <(split_suffixes)
}

# A split package: alone in each of its groups (-run and -skip apply to every package of a go test
# call), exactly one line without a pattern, patterns without "/" (a pattern with one would reach
# into subtests, and the groups would no longer be each other's complement), and every group takes
# at least one test. The counts are printed; they add up to the package's tests by construction.
check_split() {
  local suffix=$1 bad=0 group pattern names total taken run skip n sum=0
  while read -r group; do
    if [ "$(group_suffixes "$group" | wc -l)" -ne 1 ]; then
      echo "testgroups.txt: group '$group' holds the split package $suffix and something else" >&2
      bad=1
    fi
  done < <(entries | awk -v s="$suffix" '$2 == s { print $1 }')
  if [ "$(entries | awk -v s="$suffix" '$2 == s && NF == 2' | wc -l)" -ne 1 ]; then
    echo "testgroups.txt: the split package $suffix needs exactly one line without a pattern" >&2
    bad=1
  fi
  if [ "$(patterns_of "$suffix" | grep -c / || true)" -gt 0 ]; then
    echo "testgroups.txt: a pattern of $suffix contains '/'" >&2
    bad=1
  fi
  [ "$bad" -eq 0 ] || exit 1
  # Top-level test names, as go test runs them (examples and fuzz tests included).
  names=$(go test -list '.*' "./$suffix" | grep -E '^(Test|Example|Fuzz)')
  total=$(printf '%s\n' "$names" | grep -c .)
  while read -r group pattern; do
    run=$pattern
    skip=$(split_flags "$suffix" "$pattern" | sed -n 's/.*-skip=//p')
    n=$(printf '%s\n' "$names" | { if [ -n "$run" ]; then grep -E -- "$run"; else cat; fi; } |
      { if [ -n "$skip" ]; then grep -vE -- "$skip"; else cat; fi; } | grep -c . || true)
    echo "  $suffix group $group: $n tests ($(split_flags "$suffix" "$pattern"))"
    if [ "$n" -eq 0 ]; then
      echo "testgroups.txt: group '$group' takes no test of $suffix" >&2
      bad=1
    fi
    sum=$((sum + n))
  done < <(entries | awk -v s="$suffix" '$2 == s { print $1, $3 }')
  echo "  $suffix: $sum of $total tests in its groups"
  if [ "$sum" -ne "$total" ]; then
    echo "testgroups.txt: the groups of $suffix take $sum of its $total tests" >&2
    bad=1
  fi
  [ "$bad" -eq 0 ] || exit 1
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
    line=$(entries | awk -v g="$1" '$1 == g && !done { print $2, $3; done = 1 }')
    suffix=${line%% *}
    if is_split "$suffix"; then
      pattern=${line#"$suffix"}
      pattern=${pattern# }
      echo "$(split_flags "$suffix" "$pattern") $(echo "$suffix" | as_paths)"
    else
      fail_if_empty "$(group_suffixes "$1" | as_paths | tr '\n' ' ')" "group '$1'"
    fi
    ;;
esac
