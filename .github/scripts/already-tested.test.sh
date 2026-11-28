#!/usr/bin/env bash
# Self-test of already-tested.sh (task ci:workflows). A throwaway git repository supplies the
# merge commit, and a fake `gh` on PATH answers each API call from a JSON fixture, applying the
# script's own --jq expression with jq. No network.
set -euo pipefail

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
script=$here/already-tested.sh
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
failures=0

# --- fake gh ----------------------------------------------------------------------------------
mkdir -p "$work/bin"
cat >"$work/bin/gh" <<'EOF'
#!/usr/bin/env bash
# gh api [--paginate] <endpoint> [--jq EXPR]: prints the fixture for the endpoint through jq.
set -euo pipefail
[ "${1:-}" = api ] || { echo "fake gh: only 'api'" >&2; exit 2; }
shift
endpoint= expr=.
while [ $# -gt 0 ]; do
  case "$1" in
    --paginate) ;;
    --jq) expr=$2; shift ;;
    -*) echo "fake gh: unknown flag $1" >&2; exit 2 ;;
    *) endpoint=$1 ;;
  esac
  shift
done
file="$FAKE_GH_DIR/$(printf '%s' "$endpoint" | tr -c 'A-Za-z0-9' '_').json"
[ -f "$file" ] || { echo "fake gh: no fixture for $endpoint" >&2; exit 1; }
jq -r "$expr" <"$file"
EOF
chmod +x "$work/bin/gh"
export PATH="$work/bin:$PATH" GITHUB_REPOSITORY=o/r GITHUB_SERVER_URL=https://example.test

fixture() { # fixture <endpoint> <json>
  printf '%s' "$2" >"$FAKE_GH_DIR/$(printf '%s' "$1" | tr -c 'A-Za-z0-9' '_').json"
}

# --- repository: main A, branch B on top of A, merge commit M with B's tree ----------------------
repo=$work/repo
git init -q -b main "$repo"
cd "$repo"
git config user.email t@example.test
git config user.name test
echo a >a && git add a && git commit -q -m A
git checkout -q -b feature
echo b >b && git add b && git commit -q -m B
git checkout -q main
git merge -q --no-ff feature -m M
p1=$(git rev-parse HEAD^1)
p2=$(git rev-parse HEAD^2)
tree=$(git rev-parse 'HEAD^{tree}')
runs_endpoint="repos/o/r/actions/workflows/ci.yml/runs?event=pull_request&head_sha=$p2&per_page=100"

emulator='android-emulator (flow, informational)'
green_jobs() { # green_jobs <emulator conclusion> [extra job json]
  printf '{"jobs":[{"id":1,"conclusion":"success","name":"changes (docs only?)"},{"id":2,"conclusion":"success","name":"server tests (1)"},{"id":3,"conclusion":"%s","name":"%s"}%s]}' \
    "$1" "$emulator" "${2:+,$2}"
}

setup() { # fresh fixtures: everything green, emulator red, tree recorded
  export FAKE_GH_DIR=$work/fx-$RANDOM$RANDOM
  mkdir -p "$FAKE_GH_DIR"
  fixture "repos/o/r/compare/$p1...$p2" '{"behind_by":0}'
  fixture "$runs_endpoint" '{"workflow_runs":[{"id":11,"status":"completed","conclusion":"success"}]}'
  fixture "repos/o/r/actions/runs/11/jobs" "$(green_jobs failure)"
  fixture "repos/o/r/check-runs/1/annotations" "[{\"title\":\"tested-tree\",\"message\":\"$tree\"}]"
}

expect() { # expect <name> <true|false> [grep pattern in GITHUB_OUTPUT]
  local out status=0
  export GITHUB_OUTPUT=$FAKE_GH_DIR/output
  : >"$GITHUB_OUTPUT"
  out=$(bash "$script" 2>"$FAKE_GH_DIR/stderr") || status=$?
  if [ "$status" != 0 ] || [ "$out" != "$2" ]; then
    echo "FAIL $1: expected '$2', got '$out' (exit $status)"; sed 's/^/  /' "$FAKE_GH_DIR/stderr"
    failures=$((failures + 1)); return
  fi
  if [ -n "${3:-}" ] && ! grep -q -- "$3" "$GITHUB_OUTPUT"; then
    echo "FAIL $1: GITHUB_OUTPUT lacks '$3'"; sed 's/^/  /' "$GITHUB_OUTPUT"
    failures=$((failures + 1)); return
  fi
  echo "ok   $1"
}

# --- cases --------------------------------------------------------------------------------------
setup
expect "red informational job does not block, and is reported" true "note=$emulator: failure"
grep -qx 'run=11' "$GITHUB_OUTPUT" || { echo "FAIL run id not written"; failures=$((failures + 1)); }

setup
fixture "repos/o/r/actions/runs/11/jobs" "$(green_jobs success '{"id":4,"conclusion":"failure","name":"e2e (shard 2)"}')"
expect "a red gate job blocks" false

setup
fixture "repos/o/r/actions/runs/11/jobs" "$(green_jobs success '{"id":4,"conclusion":"skipped","name":"build (server binary with the web assets)"}')"
expect "a skipped gate job blocks (documentation-only run)" false

setup
fixture "repos/o/r/actions/runs/11/jobs" "$(green_jobs success '{"id":4,"conclusion":"failure","name":"android-emulator (flow)"}')"
expect "only the exact informational name is exempt" false

setup
fixture "repos/o/r/check-runs/1/annotations" '[]'
expect "a run without a recorded tree blocks (runs before this check)" false

setup
fixture "repos/o/r/check-runs/1/annotations" '[{"title":"tested-tree","message":"0000000000000000000000000000000000000000"}]'
expect "a run that checked another tree (another base branch) blocks" false

setup
fixture "repos/o/r/check-runs/1/annotations" "[{\"title\":\"tested-tree\",\"message\":\"$tree\"},{\"title\":\"tested-tree\",\"message\":\"$tree\"}]"
expect "two recorded trees block" false

setup
fixture "repos/o/r/compare/$p1...$p2" '{"behind_by":1}'
expect "a head behind main blocks" false

setup
rm "$FAKE_GH_DIR"/repos_o_r_compare_*
expect "an API error blocks" false

setup
fixture "$runs_endpoint" '{"workflow_runs":[{"id":11,"status":"completed","conclusion":"failure"},{"id":12,"status":"in_progress","conclusion":null}]}'
expect "no completed green run blocks" false

setup
fixture "repos/o/r/actions/runs/11/jobs" '{"jobs":[{"id":2,"conclusion":"success","name":"server tests (1)"}]}'
expect "a run without the job changes blocks" false

setup
git checkout -q --detach "$p2"
expect "a commit with one parent blocks" false
git checkout -q main

if [ "$failures" -gt 0 ]; then
  echo "already-tested.test.sh: $failures case(s) failed" >&2
  exit 1
fi
echo "already-tested.test.sh: all cases passed"
