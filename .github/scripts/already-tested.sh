#!/usr/bin/env bash
# Prints `true` when the commit just pushed to main has exactly the content that a green
# pull-request run of this workflow already checked in full, else `false` (ADR 0013, addendums of
# 15 November and 28 November 2026). The job "changes" of ci.yml calls it on a push to main; `true`
# lets the heavy jobs skip, and the gate takes the pull-request run's result instead.
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
#     succeeded -- none skipped -- except the informational jobs of .github/ci-informational-jobs.txt,
#     which the gate does not ask for either (their result is reported, not judged). A
#     documentation-only pull request skips its heavy jobs; its merge is then checked in full on
#     main, as before (docs-only.sh is not trusted twice).
#  5. That run recorded the tree it checked (annotation `tested-tree` of its job "changes", written
#     from the merge commit it checked out) and it is HEAD's tree. This binds the run to the content
#     now on main: a run of the same P2 against another base branch checked another merge and does
#     not count. The run's `pull_requests` field cannot do this, because GitHub lists only open pull
#     requests there and the pull request is already merged when this runs.
#  6. The merge does not change .github/. The pull-request run checked with the pull request's own
#     workflow, scripts and job list; a change there is a change of the check itself, and main
#     then checks it in full with what is now merged (security review of 28 November 2026).
#
# Needs: a checkout with both parents (fetch-depth 2), `gh` with GH_TOKEN (actions: read,
# checks: read), GITHUB_REPOSITORY and GITHUB_SERVER_URL. Any API error prints `false`: what cannot
# be judged runs everything. Diagnostics go to stderr, the answer to stdout; with GITHUB_OUTPUT set,
# `run=<id>` and `note=<informational results>` go there.
set -euo pipefail

say() { echo "already-tested: $*" >&2; }
# Job names and annotation texts come from the pull request (its ci.yml, its steps). Printed into
# this log, a line starting with `::` would be a workflow command of this job, and the old form
# `##[` works anywhere in a line; only printable characters of a single line pass, nothing printed
# here starts a line, and `##[` is broken up.
clean() { printf '%s' "$1" | tr -d '\r\n' | tr -cd '[:print:]' | sed 's/##\[/# #[/g'; }
answer() {
  echo "$1"
  exit 0
}

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
informational_file=${CI_INFORMATIONAL_JOBS:-$here/../ci-informational-jobs.txt}
informational=$(grep -v '^[[:space:]]*#' "$informational_file" 2>/dev/null | sed '/^[[:space:]]*$/d') || informational=

parents=$(git rev-list --parents -n 1 HEAD | cut -d' ' -f2-)
# One positional parameter per parent is the point of the unquoted expansion.
# shellcheck disable=SC2086
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

head_tree=$(git rev-parse "HEAD^{tree}")
if [ "$head_tree" != "$(git rev-parse "$p2^{tree}")" ]; then
  say "HEAD's tree differs from the tree of $p2"
  answer false
fi

if ! git diff --quiet "$p1" HEAD -- .github; then
  say "the merge changes .github/: main checks it in full"
  answer false
fi

# The list is not filtered with status=success: that filter missed a finished run once (the merge
# of #178 on 16 November 2026: the run was complete and green, and status=success did not list
# it, not even twenty minutes later; apparently GitHub answers the filter from an index). The
# run's own fields decide.
runs=$(gh api --paginate "repos/$GITHUB_REPOSITORY/actions/workflows/ci.yml/runs?event=pull_request&head_sha=$p2&per_page=100" \
  --jq '.workflow_runs[] | select(.status == "completed" and .conclusion == "success") | .id' 2>/dev/null) || {
  say "listing the runs of $p2 failed"
  answer false
}
for run in $runs; do
  # Every job of the run's latest attempt: id, conclusion and name, one line each (@tsv escapes a
  # tab or line break inside a name, so a name is always one field).
  jobs=$(gh api --paginate "repos/$GITHUB_REPOSITORY/actions/runs/$run/jobs" \
    --jq '.jobs[] | [.id, (.conclusion // "null"), .name] | @tsv' 2>/dev/null) || continue
  [ -n "$jobs" ] || continue
  bad=
  note=
  changes_job=
  while IFS=$'\t' read -r id conclusion name; do
    if [ -n "$informational" ] && grep -qxF -- "$name" <<<"$informational"; then
      note="${note:+$note; }$(clean "$name"): $(clean "$conclusion")"
      continue
    fi
    if [ "$name" = "changes (docs only?)" ]; then
      # Two jobs of that name would leave the choice to the order; that is no proof.
      [ -z "$changes_job" ] && changes_job=$id || changes_job=ambiguous
    fi
    [ "$conclusion" = success ] || bad="${bad:+$bad, }$(clean "$name"): $(clean "$conclusion")"
  done <<<"$jobs"
  if [ -n "$bad" ]; then
    say "run $run of $p2 has jobs that did not succeed: $bad"
    continue
  fi
  if [ -z "$changes_job" ] || [ "$changes_job" = ambiguous ]; then
    say "run $run of $p2 has no single job \"changes (docs only?)\""
    continue
  fi
  tested=$(gh api --paginate "repos/$GITHUB_REPOSITORY/check-runs/$changes_job/annotations" \
    --jq '.[] | select(.title == "tested-tree") | .message' 2>/dev/null) || {
    say "reading the annotations of run $run failed"
    continue
  }
  # Exactly one annotation with exactly main's tree: a second one (a file name of the pull request
  # printed as a workflow command, say) makes this two lines and therefore no match.
  if [ "$tested" != "$head_tree" ]; then
    say "run $run checked tree '$(clean "$tested" | tr -cd '0-9a-f' | cut -c1-40)' (none recorded if empty), main has $head_tree"
    continue
  fi
  say "run $GITHUB_SERVER_URL/$GITHUB_REPOSITORY/actions/runs/$run checked $p2 in full${note:+ (informational: $note)}"
  if [ -n "${GITHUB_OUTPUT:-}" ]; then
    echo "run=$run" >>"$GITHUB_OUTPUT"
    echo "note=$note" >>"$GITHUB_OUTPUT"
  fi
  answer true
done
say "no pull-request run checked $p2 in full"
answer false
