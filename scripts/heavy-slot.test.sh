#!/usr/bin/env bash
# Self-test of scripts/heavy-slot.sh (`task heavy-slot:check`, part of `task ci:web`). The slot
# lock only does something on a developer machine -- CI skips it -- so without this test a broken
# lock would go unnoticed until several agents ran their e2e suites at once again.
set -euo pipefail

script=$(cd "$(dirname "$0")" && pwd)/heavy-slot.sh
work=$(mktemp -d -t slouchcat-heavy-slot-XXXXXX)
trap 'rm -rf "$work"' EXIT

# Every case runs as on a developer machine: no CI, no slot already held, its own lock folder.
run() { env -u CI -u SLOUCHCAT_HEAVY_HELD SLOUCHCAT_HEAVY_DIR="$work/locks" "$@"; }
fail() { echo "heavy-slot test: $1" >&2; exit 1; }

# 1. The exit status of the command comes through unchanged, success and failure.
run "$script" true || fail "true did not exit 0"
set +e
run "$script" sh -c 'exit 7'
status=$?
set -e
[ "$status" -eq 7 ] || fail "exit status 7 came back as $status"

# 2. The command sees which slot it holds, so a heavy check inside it does not wait again.
held=$(run "$script" sh -c 'echo "$SLOUCHCAT_HEAVY_HELD"')
[ "$held" = 1 ] || fail "SLOUCHCAT_HEAVY_HELD was '$held', not 1"

# 3. Two slots: two commands run at once, the third waits until one of them is done.
start=$(date +%s)
for i in 1 2 3; do
  run env SLOUCHCAT_HEAVY_SLOTS=2 "$script" sh -c "sleep 2; date +%s > '$work/done-$i'" 2>"$work/err-$i" &
done
wait
end=$(date +%s)
elapsed=$((end - start))
[ "$elapsed" -ge 4 ] || fail "three 2-s runs on two slots took ${elapsed} s, so the third did not wait"
[ "$elapsed" -le 9 ] || fail "three 2-s runs on two slots took ${elapsed} s, so two did not run side by side"
grep -q "all 2 slots are taken" "$work"/err-* || fail "the waiting run did not say why it waited"

# 4. CI, an already held slot and SLOUCHCAT_HEAVY_SLOTS=0 never wait, even with every slot taken.
run env SLOUCHCAT_HEAVY_SLOTS=1 "$script" sleep 3 &
holder=$!
sleep 0.5
for case in "CI=true" "SLOUCHCAT_HEAVY_HELD=1" "SLOUCHCAT_HEAVY_SLOTS=0"; do
  t0=$(date +%s)
  run env SLOUCHCAT_HEAVY_SLOTS=1 "$case" "$script" true
  [ $(($(date +%s) - t0)) -le 1 ] || fail "$case waited for the taken slot"
done
wait "$holder"

# 5. A process the command leaves behind does not keep the slot taken.
run env SLOUCHCAT_HEAVY_SLOTS=1 "$script" sh -c "sleep 5 >/dev/null 2>&1 &"
t0=$(date +%s)
run env SLOUCHCAT_HEAVY_SLOTS=1 "$script" true
[ $(($(date +%s) - t0)) -le 1 ] || fail "a leftover background process kept the slot taken"

# 6. Nonsense in SLOUCHCAT_HEAVY_SLOTS is an error, not a silent run without a lock.
if run env SLOUCHCAT_HEAVY_SLOTS=two "$script" true 2>/dev/null; then
  fail "SLOUCHCAT_HEAVY_SLOTS=two was accepted"
fi

echo "heavy-slot: all checks passed"
