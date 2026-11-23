# shellcheck shell=bash
# The helpers of the Android flows and the contract test, started and stopped in one place
# (sourced by scripts/android-e2e.sh, scripts/android-tv-e2e.sh and task android:contract), and
# the machine-wide lock of the emulator.
#
#   pairing relay     relay/bin/pair     PAIR_PORT   (the caller's RELAY_PORT, default PORT+85; stage 9, K26)
#   push relay        relay/bin/push     PUSH_PORT   (default PORT+83; stage 8)
#   Google's push     relay/bin/fcmfake  FCM_PORT    (default PORT+84; stands in for FCM)
#   Seerr mock        web/e2e/seerr-server.mjs   SEERR_PORT  (default PORT-11; wishes)
#   TMDB mock         web/e2e/tmdb-server.mjs    TMDB_PORT   (default PORT-12; titles outside the library)
#
# Ports: every default is a fixed distance from the server's PORT (PORT-12 to PORT+85), so two runs
# whose PORTs lie at least 100 apart (android:e2e 8597, android:e2e:tv 8697, the web suite's
# E2E_PORT series 8197, 8297 ...) never take the same helper port. A value set by hand (RELAY_PORT,
# PUSH_PORT, FCM_PORT, SEERR_PORT, TMDB_PORT) goes first. All on the loopback only; an emulator
# reaches them as http://10.0.2.2:<port>. Every helper is waited for, and each must still run
# afterwards (an answer may come from another run's helper on a taken port); one that does not
# come up stops the run with the tail of its log.
#
#   . ../scripts/android-helpers.sh
#   helpers_start "$root" "$port" "$pair_port"   # exports SLOUCHCAT_TEST_TMDB_* and SLOUCHCAT_PUSH_RELAY
#   ...                                          # start the server after this: it inherits them
#   emulator_lock "$wait_max" "$root"            # before the emulator starts; held until emulator_unlock
#   helpers_save_logs "$dir"                     # on a failure
#   helpers_stop; emulator_unlock                # after the emulator is gone
#
# The emulator lock: one emulator at a time on a shared machine. flock on EMULATOR_LOCK (default
# ${XDG_RUNTIME_DIR:-/tmp}/slouchcat-emulator.lock) from before the emulator starts until it has
# stopped; it waits up to wait_max seconds (WAIT_MAX of the callers; 0 gives up at once). A small
# holder process keeps the lock, not the script: adb's server and Gradle's daemon, started by the
# run and living on after it, must not inherit it. The holder ends with the run, also when the run
# is killed. In CI one run per runner: the lock is free at once.
#
# Needs relay/bin (task relay:build), Node and flock (util-linux). Paths are relative to android/
# (where the Taskfile runs).

helper_pids=()
helper_root=""

helper_wait() { # name url [file]: waits until url answers at all (any HTTP status) and file exists
  local name=$1 url=$2 file=${3:-}
  for _ in $(seq 1 120); do
    if curl -s -o /dev/null --max-time 2 "$url" && { [ -z "$file" ] || [ -s "$file" ]; }; then return 0; fi
    sleep 0.25
  done
  echo "android-helpers: $name did not come up at $url" >&2
  if [ -f "$helper_root/$name.log" ]; then
    echo "--- $name log, last 20 lines ---" >&2
    tail -n 20 "$helper_root/$name.log" >&2
  fi
  return 1
}

helpers_start() {
  local root=$1 port=$2 pair_port=$3
  helper_root=$root
  HELPER_PAIR_PORT=$pair_port
  HELPER_PUSH_PORT=${PUSH_PORT:-$((port + 83))}
  HELPER_FCM_PORT=${FCM_PORT:-$((port + 84))}
  HELPER_SEERR_PORT=${SEERR_PORT:-$((port - 11))}
  HELPER_TMDB_PORT=${TMDB_PORT:-$((port - 12))}
  export HELPER_PAIR_PORT HELPER_PUSH_PORT HELPER_FCM_PORT HELPER_SEERR_PORT HELPER_TMDB_PORT

  RELAY_LISTEN="127.0.0.1:$HELPER_PAIR_PORT" RELAY_LOG_FORMAT=text ../relay/bin/pair >"$root/pair.log" 2>&1 &
  helper_pids+=($!)
  ../relay/bin/fcmfake -listen "127.0.0.1:$HELPER_FCM_PORT" -credentials "$root/firebase.json" >"$root/fcmfake.log" 2>&1 &
  helper_pids+=($!)
  ../relay/bin/push -add-key 1 -keyfile "$root/keys.json" >/dev/null
  SLOUCHCAT_E2E_SEERR_PORT=$HELPER_SEERR_PORT node ../web/e2e/seerr-server.mjs >"$root/seerr.log" 2>&1 &
  helper_pids+=($!)
  SLOUCHCAT_E2E_TMDB_PORT=$HELPER_TMDB_PORT node ../web/e2e/tmdb-server.mjs >"$root/tmdb.log" 2>&1 &
  helper_pids+=($!)
  helper_wait fcmfake "http://127.0.0.1:$HELPER_FCM_PORT/messages" "$root/firebase.json" || return 1
  RELAY_LISTEN="127.0.0.1:$HELPER_PUSH_PORT" RELAY_LOG_FORMAT=text RELAY_PUSH_ORIGINS="http://127.0.0.1:$HELPER_PUSH_PORT" \
    RELAY_PUSH_KEYFILE="$root/keys.json" RELAY_FCM_CREDENTIALS="$root/firebase.json" \
    RELAY_FCM_BASE_URL="http://127.0.0.1:$HELPER_FCM_PORT" ../relay/bin/push >"$root/push.log" 2>&1 &
  helper_pids+=($!)
  helper_wait pair "http://127.0.0.1:$HELPER_PAIR_PORT/v1/healthz" || return 1
  helper_wait push "http://127.0.0.1:$HELPER_PUSH_PORT/v1/healthz" || return 1
  helper_wait seerr "http://127.0.0.1:$HELPER_SEERR_PORT/api/v1/status" || return 1
  helper_wait tmdb "http://127.0.0.1:$HELPER_TMDB_PORT/3/configuration" || return 1
  # An answer may come from another run's helper on a taken port while ours died at its bind:
  # every own helper must still be running.
  local pid
  for pid in "${helper_pids[@]}"; do
    if ! kill -0 "$pid" 2>/dev/null; then
      echo "android-helpers: a helper exited (port taken?); its logs are in $root" >&2
      grep -h "address already in use" "$root"/*.log >&2 || true
      return 1
    fi
  done

  # The server started after this inherits them: TMDB (only asked with a key, --dev only) and the
  # local push relay (http and the loopback only with --dev).
  export SLOUCHCAT_TEST_TMDB_BASE_URL="http://127.0.0.1:$HELPER_TMDB_PORT/3"
  export SLOUCHCAT_TEST_TMDB_IMAGE_BASE_URL="http://127.0.0.1:$HELPER_TMDB_PORT/t/p"
  export SLOUCHCAT_PUSH_RELAY="http://127.0.0.1:$HELPER_PUSH_PORT"
}

helpers_save_logs() { # dir: copies the helpers' logs there
  local dir=$1
  [ -n "$helper_root" ] || return 0
  mkdir -p "$dir"
  for name in pair push fcmfake seerr tmdb; do
    if [ -f "$helper_root/$name.log" ]; then cp "$helper_root/$name.log" "$dir/helper-$name.log"; fi
  done
}

helpers_stop() {
  local pid
  for pid in "${helper_pids[@]}"; do
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
  done
  helper_pids=()
}

emulator_lock_pid=""

emulator_lock() { # wait_max dir: takes the machine-wide lock of the emulator, waiting up to wait_max s
  local wait_max=$1 dir=$2 lock=${EMULATOR_LOCK:-${XDG_RUNTIME_DIR:-/tmp}/slouchcat-emulator.lock}
  local held="$dir/emulator-lock.held" parent=$BASHPID
  rm -f "$held"
  (
    exec 9>>"$lock"
    if ! flock -n 9; then
      echo "android-helpers: another run holds the emulator ($lock); waiting up to ${wait_max} s" >&2
      flock -w "$wait_max" 9 || exit 1
    fi
    : >"$held"
    while kill -0 "$parent" 2>/dev/null; do sleep 1 9>&-; done
  ) </dev/null >/dev/null &
  emulator_lock_pid=$!
  while [ ! -e "$held" ]; do
    if ! kill -0 "$emulator_lock_pid" 2>/dev/null; then
      emulator_lock_pid=""
      echo "android-helpers: gave up after ${wait_max} s: another run still holds the emulator ($lock)" >&2
      return 1
    fi
    sleep 0.5
  done
  echo "android-helpers: emulator lock taken ($lock)"
}

emulator_unlock() { # releases the lock of emulator_lock (the emulator must be gone)
  if [ -n "$emulator_lock_pid" ]; then
    kill "$emulator_lock_pid" 2>/dev/null || true
    wait "$emulator_lock_pid" 2>/dev/null || true
    emulator_lock_pid=""
  fi
}
