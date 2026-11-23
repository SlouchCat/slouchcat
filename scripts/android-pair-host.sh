#!/usr/bin/env bash
# The pairing across languages on the host, without an emulator (task android:pair:host, concept 27
# stage 9, code review m-6): the phone app's own Kotlin (:core:pairing, ScannedCode and PhonePairing,
# run as :core:pairing:phoneCheck) seals the packet, the Go TV of `apicheck -pair-tv` opens it,
# proves the server, redeems and signs out again. A fresh server on PORT and the relay on RELAY_PORT,
# both on the loopback only.
#
# PORT        port of the test server (default 8797)
# RELAY_PORT  port of the relay (default 8182)
#
# Run from android/ (the Taskfile does). Needs server/bin/slouchcat, server/bin/apicheck and
# relay/bin/pair.
set -euo pipefail

port=${PORT:-8797}
relay_port=${RELAY_PORT:-8182}
email=pair-owner@example.test
password=pair-password-0123
root=$(mktemp -d -t slouchcat-pair-host-XXXXXX)
server_pid=""
relay_pid=""
tv_pid=""

cleanup() {
  for pid in "$tv_pid" "$server_pid" "$relay_pid"; do
    if [ -n "$pid" ]; then kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true; fi
  done
  rm -rf "$root"
}
trap cleanup EXIT

fail() {
  echo "android-pair-host: $1" >&2
  for f in server.log relay.log tv.log phone.log; do
    if [ -f "$root/$f" ]; then
      echo "--- $f, last 20 lines ---" >&2
      tail -n 20 "$root/$f" >&2
    fi
  done
  exit 1
}

for bin in ../server/bin/slouchcat ../server/bin/apicheck ../relay/bin/pair; do
  [ -x "$bin" ] || fail "$bin is missing: task server:build relay:build, go build -o bin/apicheck ./cmd/apicheck"
done

SLOUCHCAT_TMDB_API_KEY= ../server/bin/slouchcat serve --listen "127.0.0.1:$port" \
  --config-dir "$root/config" --cache-dir "$root/cache" --dev --log-format text >"$root/server.log" 2>&1 &
server_pid=$!
RELAY_LISTEN="127.0.0.1:$relay_port" ../relay/bin/pair >"$root/relay.log" 2>&1 &
relay_pid=$!
for _ in $(seq 1 120); do
  kill -0 "$server_pid" 2>/dev/null || fail "the server stopped while starting (port $port taken?)"
  kill -0 "$relay_pid" 2>/dev/null || fail "the relay stopped while starting (port $relay_port taken?)"
  if grep -q 'msg=listening' "$root/server.log" && curl -fsS "http://127.0.0.1:$relay_port/v1/healthz" >/dev/null 2>&1; then break; fi
  sleep 0.5
done
grep -q 'msg=listening' "$root/server.log" || fail "the server reported no address within 60 s"

# The account of the phone: the setup of a fresh server.
status=$(curl -s -o /dev/null -w '%{http_code}' -X POST -H 'Content-Type: application/json' -H 'X-Requested-With: Slouchcat' \
  --data "{\"email\":\"$email\",\"password\":\"$password\"}" "http://127.0.0.1:$port/api/v1/setup")
[ "$status" = 201 ] || fail "setup answered $status"

# The TV (Go): its code goes to a file the phone reads.
../server/bin/apicheck -base "http://127.0.0.1:$port" -spec ../api/openapi.yaml -relay "http://127.0.0.1:$relay_port" \
  -pair-tv "$root/code.txt" -pair-want-server "http://127.0.0.1:$port" -pair-want-email "$email" >"$root/tv.log" 2>&1 &
tv_pid=$!
for _ in $(seq 1 60); do
  [ -s "$root/code.txt" ] && break
  kill -0 "$tv_pid" 2>/dev/null || fail "apicheck stopped before it showed a code"
  sleep 0.5
done
[ -s "$root/code.txt" ] || fail "apicheck showed no code within 30 s"

# The phone (Kotlin): the voucher, the packet sealed for the Go TV's key, the wait for the TV.
SLOUCHCAT_PAIR_PASSWORD=$password ./gradlew --console=plain -q :core:pairing:phoneCheck --args="server=http://127.0.0.1:$port \
packetServer=http://127.0.0.1:$port relay=http://127.0.0.1:$relay_port codeRelay=http://127.0.0.1:$relay_port \
code=$(cat "$root/code.txt") email=$email" >"$root/phone.log" 2>&1 || fail "the phone's Kotlin could not pair"
grep '^phonecheck:' "$root/phone.log"
wait "$tv_pid" || { tv_pid=""; fail "the Go TV could not open the Kotlin packet or redeem"; }
tv_pid=""
grep '^apicheck:' "$root/tv.log"
echo "android-pair-host: Kotlin sealed, Go opened, redeemed and signed out"
