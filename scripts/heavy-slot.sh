#!/usr/bin/env bash
# Runs one heavy local check -- the whole Go suite under -race, a Playwright e2e run, the Storybook
# screenshots -- in one of a few machine-wide slots, and waits while all of them are taken.
#
#   scripts/heavy-slot.sh <command> [args...]
#
# Why: several agents work on this machine at the same time, each in a worktree of its own. When
# four of them start the e2e suite at once, four servers, four browsers and their transcodes fight
# for the same cores, every run gets slower and the time budgets of the playback specs start to
# fail. Waiting for a slot is cheaper than that, and the result of a run that had the machine to
# itself is one to trust (docs/dev/SETUP.md, "Teststufen").
#
# The slots are flock(1) locks on files in SLOUCHCAT_HEAVY_DIR (default
# $XDG_RUNTIME_DIR/slouchcat-heavy, else /tmp/slouchcat-heavy). The lock is held by this script, not
# by the command: the command runs as a child with the lock descriptor closed, so a server it left
# behind cannot keep a slot taken after the run. The lock goes away with this script, also when it
# is killed.
#
# SLOUCHCAT_HEAVY_SLOTS  number of slots (default 2); 0 turns the waiting off
# SLOUCHCAT_HEAVY_HELD   set for the command; a heavy check started inside another one (`task ci`
#                        runs several) does not wait for a second slot
# CI                     set on GitHub runners: every job has its runner to itself, so no slot
set -euo pipefail

if [ "$#" -eq 0 ]; then
  echo "usage: scripts/heavy-slot.sh <command> [args...]" >&2
  exit 2
fi

slots=${SLOUCHCAT_HEAVY_SLOTS:-2}
case $slots in
  '' | *[!0-9]*)
    echo "heavy-slot: SLOUCHCAT_HEAVY_SLOTS must be a number, not '$slots'" >&2
    exit 2
    ;;
esac

if [ -n "${CI:-}" ] || [ -n "${SLOUCHCAT_HEAVY_HELD:-}" ] || [ "$slots" -eq 0 ]; then
  exec "$@"
fi

dir=${SLOUCHCAT_HEAVY_DIR:-${XDG_RUNTIME_DIR:-/tmp}/slouchcat-heavy}
mkdir -p "$dir"

waiting_since=""
while :; do
  for i in $(seq 1 "$slots"); do
    file="$dir/slot-$i.lock"
    # Read-write without truncating: opening with > would wipe the holder line of a slot that is
    # taken, and the waiting message below reads it.
    exec 9<>"$file"
    if flock -n 9; then
      printf '%s pid %s in %s: %s\n' "$(date '+%H:%M:%S')" "$$" "$PWD" "$*" >"$file"
      if [ -n "$waiting_since" ]; then
        echo "heavy-slot: slot $i free after $(($(date +%s) - waiting_since)) s" >&2
      fi
      set +e
      SLOUCHCAT_HEAVY_HELD=$i "$@" 9>&-
      status=$?
      set -e
      : >"$file"
      exit "$status"
    fi
    exec 9>&-
  done
  if [ -z "$waiting_since" ]; then
    waiting_since=$(date +%s)
    echo "heavy-slot: all $slots slots are taken, waiting (SLOUCHCAT_HEAVY_SLOTS, docs/dev/SETUP.md):" >&2
    for i in $(seq 1 "$slots"); do
      echo "  slot $i: $(head -n 1 "$dir/slot-$i.lock" 2>/dev/null || true)" >&2
    done
  fi
  sleep 2
done
