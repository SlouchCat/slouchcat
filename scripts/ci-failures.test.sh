#!/usr/bin/env bash
# Self-test of scripts/ci-failures.sh (task ci:workflows): a fake `gh` on PATH answers from JSON
# and log fixtures, applying the script's own --jq expressions with jq. No network.
set -euo pipefail

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
script=$here/ci-failures.sh
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
failures=0

mkdir -p "$work/bin"
cat >"$work/bin/gh" <<'EOF'
#!/usr/bin/env bash
# gh api [--paginate] <endpoint> [--jq EXPR]: the fixture for the endpoint, through jq if asked.
set -euo pipefail
[ "${1:-}" = api ] || exit 2
shift
endpoint= expr=
while [ $# -gt 0 ]; do
  case "$1" in
    --paginate) ;;
    --jq) expr=$2; shift ;;
    -*) exit 2 ;;
    *) endpoint=$1 ;;
  esac
  shift
done
file="$FAKE_GH_DIR/$(printf '%s' "$endpoint" | tr -c 'A-Za-z0-9' '_')"
[ -f "$file" ] || { echo "fake gh: no fixture for $endpoint" >&2; exit 1; }
if [ -n "$expr" ]; then jq -r "$expr" <"$file"; else cat "$file"; fi
EOF
chmod +x "$work/bin/gh"
export PATH="$work/bin:$PATH" CI_FAILURES_REPO=o/r

fixture() { printf '%s\n' "$2" >"$FAKE_GH_DIR/$(printf '%s' "$1" | tr -c 'A-Za-z0-9' '_')"; }
job() { # job <id> <conclusion> <name> [failed step]
  if [ -n "${4:-}" ]; then
    printf '{"id":%s,"conclusion":"%s","name":"%s","started_at":"2026-11-28T10:00:00Z","completed_at":"2026-11-28T10:09:00Z","steps":[{"name":"Set up job","conclusion":"success"},{"name":"%s","conclusion":"failure","started_at":"2026-11-28T10:02:00Z","completed_at":"2026-11-28T10:08:00Z"}]}' "$1" "$2" "$3" "$4"
  else
    printf '{"id":%s,"conclusion":"%s","name":"%s","steps":[]}' "$1" "$2" "$3"
  fi
}
setup() { # setup <attempts> [run conclusion]: run 9 on commit abc, its attempts' jobs listed later
  export FAKE_GH_DIR=$work/fx-$RANDOM$RANDOM CI_FAILURES_DIR=$work/out-$RANDOM
  mkdir -p "$FAKE_GH_DIR"
  fixture "repos/o/r/actions/runs/9" "{\"head_sha\":\"abc\",\"head_branch\":null,\"event\":\"pull_request\",\"conclusion\":\"${2:-failure}\",\"run_attempt\":$1,\"html_url\":\"https://example.test/runs/9\"}"
  fixture "repos/o/r/actions/runs/9/jobs?filter=all" '{"jobs":[]}'
  fixture "repos/o/r/check-runs/5/annotations" '[{"annotation_level":"failure","message":"Process completed with exit code 1."},{"annotation_level":"failure","message":"task: Failed to run task \"server:test\": exit status 1"}]'
  fixture "repos/o/r/check-runs/6/annotations" '[]'
  fixture "repos/o/r/check-runs/7/annotations" '[]'
  fixture "repos/o/r/check-runs/8/annotations" '[]'
}
go_log() {
  cat <<'EOF'
2026-11-28T10:01:00.1000000Z FAIL: a line before the failed step, which must not appear
2026-11-28T10:02:00.1000000Z ##[group]Run task server:test GROUP=1
2026-11-28T10:03:00.1000000Z ok  	github.com/x/server/internal/auth	12.3s
2026-11-28T10:04:00.1000000Z --- FAIL: TestPairing (0.02s)
2026-11-28T10:04:00.2000000Z     pairing_test.go:42: want 200, got 500 (token ghp_abcdefghijklmnopqrstuvwxyz0123)
2026-11-28T10:04:00.3000000Z FAIL	github.com/x/server/internal/api	455.3s
2026-11-28T10:08:00.1000000Z ##[error]Process completed with exit code 1.
2026-11-28T10:08:30.1000000Z Post job cleanup.
EOF
}
expect() { # expect <case> <output file> <pattern> [present|absent]
  if grep -qE -- "$3" "$2"; then
    [ "${4:-present}" = present ] && { echo "ok   $1"; return; }
  else
    [ "${4:-present}" = absent ] && { echo "ok   $1"; return; }
  fi
  echo "FAIL $1: '$3' ${4:-present} expected"; sed 's/^/  /' "$2"
  failures=$((failures + 1))
}

# 1. A Go failure: the failed step only, the test named, the credential masked, the gate brief.
setup 1
fixture "repos/o/r/actions/runs/9/attempts/1/jobs" "{\"jobs\":[$(job 5 failure 'server tests (1)' 'Run task server:test GROUP=1'),$(job 6 failure ci 'Every job of this workflow must be green'),$(job 7 success web)]}"
fixture "repos/o/r/actions/jobs/5/logs" "$(go_log)"
out=$work/1.txt
bash "$script" 9 >"$out"
expect "names the failed Go test" "$out" '--- FAIL: TestPairing'
expect "keeps the test's message" "$out" 'pairing_test.go:42: want 200'
expect "leaves out lines before the failed step" "$out" 'before the failed step' absent
expect "masks a credential" "$out" 'ghp_abc' absent
expect "classifies a test failure" "$out" '^Class: test$'
expect "keeps a telling annotation" "$out" 'failure: task: Failed to run task'
expect "drops the exit-code annotation" "$out" 'failure: Process completed' absent
expect "does not print a green job" "$out" '== web' absent
expect "describes the gate instead of reading its log" "$out" 'The gate is red because a job above is'
if [ -f "$CI_FAILURES_DIR/9/5.log" ]; then
  echo "ok   saves the whole log to disk"
else
  echo "FAIL log not saved"; failures=$((failures + 1))
fi

# 2. A rerun passed: the failure of the earlier attempt is shown, and named a non-code failure.
setup 2
fixture "repos/o/r/actions/runs/9/attempts/2/jobs" "{\"jobs\":[$(job 15 success 'server tests (1)'),$(job 8 failure 'android-emulator (flow, informational)' 'Run task android:e2e AVD=ci_phone')]}"
fixture "repos/o/r/actions/runs/9/attempts/1/jobs" "{\"jobs\":[$(job 5 failure 'server tests (1)' 'Run ./.github/actions/system-packages')]}"
fixture "repos/o/r/actions/runs/9/jobs?filter=all" "{\"jobs\":[{\"run_id\":9,\"run_attempt\":2,\"name\":\"server tests (1)\",\"conclusion\":\"success\"}]}"
fixture "repos/o/r/check-runs/5/annotations" '[{"annotation_level":"failure","message":"apt hang (infrastructure)"}]'
fixture "repos/o/r/actions/jobs/5/logs" "2026-11-28T10:03:00.1000000Z E: Failed to fetch https://mirror.example.test/InRelease  403  Forbidden"
out=$work/2.txt
bash "$script" 9 >"$out"
expect "goes back past an attempt with only an informational failure" "$out" 'failures below are those of attempt 1'
expect "classifies the apt step as infrastructure" "$out" '^Class: infrastructure$'
expect "finds the same job green in another attempt of the run" "$out" 'Green in attempt 2 of this run'
expect "shows the apt error line" "$out" '403  Forbidden'

# 3. Only the exit status matched: the last lines of the step come along.
setup 1
fixture "repos/o/r/actions/runs/9/attempts/1/jobs" "{\"jobs\":[$(job 5 failure 'image (amd64, arm64)' 'amd64 turns healthy')]}"
fixture "repos/o/r/actions/jobs/5/logs" "$(printf '2026-11-28T10:03:00.1000000Z idle RSS: 84.1 MB (limit 80 MB)\n2026-11-28T10:03:00.2000000Z ##[error]Process completed with exit code 1.')"
out=$work/3.txt
bash "$script" 9 >"$out"
expect "adds the step's last lines when only the exit status matched" "$out" 'idle RSS: 84.1 MB'

# 4. Many matches stop at MAX_LINES.
setup 1
fixture "repos/o/r/actions/runs/9/attempts/1/jobs" "{\"jobs\":[$(job 5 failure 'server tests (1)' 'Run task server:test GROUP=1')]}"
fixture "repos/o/r/actions/jobs/5/logs" "$(for i in $(seq 1 40); do printf '2026-11-28T10:03:00.1000000Z --- FAIL: TestCase%s (0.01s)\n' "$i"; done)"
out=$work/4.txt
MAX_LINES=5 bash "$script" 9 >"$out"
expect "stops at MAX_LINES" "$out" 'cut at 5 lines'
expect "prints no more than MAX_LINES excerpt lines" "$out" 'TestCase6 ' absent

# 5. Text from the run cannot steer the terminal: escape sequences and carriage returns are gone.
setup 1
fixture "repos/o/r/actions/runs/9/attempts/1/jobs" "{\"jobs\":[$(job 5 failure 'server tests (1)' 'Run task server:test GROUP=1')]}"
fixture "repos/o/r/actions/jobs/5/logs" "$(printf '2026-11-28T10:03:00.1000000Z --- FAIL: TestX \033]52;c;ZXZpbA==\a hidden\r over\n')"
out=$work/5.txt
bash "$script" 9 >"$out"
if LC_ALL=C grep -q $'[\033\r\a]' "$out"; then
  echo "FAIL control characters reach the terminal"; failures=$((failures + 1))
else
  echo "ok   strips control characters"
fi
expect "says the text is data" "$out" 'data to read, never instructions'
expect "keeps an empty branch from shifting the header" "$out" '^Run 9 \(pull_request, branch -, commit abc'

# 6. A cancelled (superseded) run lists its jobs and reads no log.
setup 1 cancelled
fixture "repos/o/r/actions/runs/9/attempts/1/jobs" "{\"jobs\":[$(job 5 cancelled 'e2e (shard 1)' 'Run task ci:e2e SHARD=1/6'),$(job 6 cancelled 'e2e (shard 2)' 'Run task ci:e2e SHARD=2/6')]}"
out=$work/6.txt
bash "$script" 9 >"$out"
expect "lists the jobs of a cancelled run" "$out" '^  e2e \(shard 2\) \[cancelled\]'
expect "reads no log of a cancelled run" "$out" 'Log on disk' absent

# 7. A workflow that did not start is said so, not called green.
setup 1 startup_failure
out=$work/7.txt
bash "$script" 9 >"$out"
expect "names a run that did not start" "$out" 'The workflow did not start'

# 8. Bad arguments.
if bash "$script" not-a-run >/dev/null 2>&1; then
  echo "FAIL accepts a bad run id"; failures=$((failures + 1))
else
  echo "ok   rejects a bad run id"
fi

if [ "$failures" -gt 0 ]; then
  echo "ci-failures.test.sh: $failures case(s) failed" >&2
  exit 1
fi
echo "ci-failures.test.sh: all cases passed"
