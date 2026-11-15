#!/usr/bin/env bash
# Prints `true` when the commit just pushed to main has exactly the content that a green
# pull-request run of this workflow already checked in full, else `false` (ADR 0013, addendum of
# 15 November 2026). The job "changes" of ci.yml calls it on a push to main; `true` lets the heavy
# jobs skip, and the gate takes the pull-request run's result instead.
#
# All of this must hold, or the answer is `false` and main checks everything as before:
#
#  1. HEAD is a merge commit with two parents: P1 (main before the merge) and P2 (the head of the
#     pull request). A squash or rebase merge has one parent and is checked in full.
#  2. P2 contains P1 (GitHub compare: P2 is not behind P1). The pull-request run checked the merge
#     of P2 into the main of its time; main only moves forward, so that main is contained in P1 and
#     therefore in P2, and the run checked the tree of P2.
#  3. HEAD has the tree of P2, so what is now on main is that tree, byte for byte.
#  4. A pull_request run of this workflow for P2 finished with success, and every one of its jobs
#     succeeded -- none skipped. A documentation-only pull request skips its heavy jobs; its merge
#     is then checked in full on main, as before (docs-only.sh is not trusted twice).
#
# Needs: a checkout with both parents (fetch-depth 2), `gh` with GH_TOKEN (actions: read),
# GITHUB_REPOSITORY and GITHUB_SERVER_URL. Any API error prints `false`: what cannot be judged runs
# everything. Diagnostics go to stderr, the answer to stdout.
set -euo pipefail

say() { echo "already-tested: $*" >&2; }
answer() {
  echo "$1"
  exit 0
}

parents=$(git rev-list --parents -n 1 HEAD | cut -d' ' -f2-)
set -- $parents
if [ "$#" -ne 2 ]; then
  say "HEAD has $# parent(s), not a merge commit"
  answer false
fi
p1=$1
p2=$2

behind=$(gh api "repos/$GITHUB_REPOSITORY/compare/$p1...$p2" --jq .behind_by 2>/dev/null) || {
  say "compare $p1...$p2 failed"
  answer false
}
if [ "$behind" != 0 ]; then
  say "the pull request head $p2 is $behind commit(s) behind main $p1"
  answer false
fi

if [ "$(git rev-parse "HEAD^{tree}")" != "$(git rev-parse "$p2^{tree}")" ]; then
  say "HEAD's tree differs from the tree of $p2"
  answer false
fi

runs=$(gh api "repos/$GITHUB_REPOSITORY/actions/workflows/ci.yml/runs?event=pull_request&head_sha=$p2&status=success" \
  --jq '.workflow_runs[].id' 2>/dev/null) || {
  say "listing the runs of $p2 failed"
  answer false
}
for run in $runs; do
  # Every job of the run's latest attempt, by conclusion; one line each.
  conclusions=$(gh api --paginate "repos/$GITHUB_REPOSITORY/actions/runs/$run/jobs" \
    --jq '.jobs[].conclusion' 2>/dev/null) || continue
  [ -n "$conclusions" ] || continue
  if ! grep -qv '^success$' <<<"$conclusions"; then
    say "run $GITHUB_SERVER_URL/$GITHUB_REPOSITORY/actions/runs/$run checked $p2 in full"
    echo "run=$run" >>"${GITHUB_OUTPUT:-/dev/null}"
    answer true
  fi
  say "run $run of $p2 has jobs that did not succeed: $(sort -u <<<"$conclusions" | tr '\n' ' ')"
done
say "no pull-request run checked $p2 in full"
answer false
