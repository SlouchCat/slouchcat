#!/usr/bin/env bash
# The emulator flow of stage 8 «Wünsche und Meldungen» (concept 27): starts the helpers the flow
# needs next to the test server of scripts/android-e2e.sh, then that script with
# CLASS=com.slouchcat.app.mobile.MessagesFlowTest, and stops the helpers again.
#
#   PORT        port of the test server (default 8597); the helpers sit beside it as in task
#               android:contract: the Seerr mock PORT-11, the TMDB mock PORT-12 (web/e2e), the push
#               relay PORT+83 and the fake of Google's push service PORT+84 (relay/bin)
#   EMU_PORT, AVD, SHOTS, LOCALE, EMU_OPTS: as scripts/android-e2e.sh
#
# The server of android-e2e.sh inherits SLOUCHCAT_TEST_TMDB_BASE_URL (the TMDB mock, --dev only)
# and SLOUCHCAT_PUSH_RELAY (the local relay) from here. Run from android/ (task android:e2e:messages).
# Needs server/bin/slouchcat and relay/bin (task server:build, task relay:build) and Node.
set -euo pipefail

port=${PORT:-8597}
seerr_port=$((port - 11))
tmdb_port=$((port - 12))
relay_port=$((port + 83))
fcm_port=$((port + 84))
root=$(mktemp -d -t slouchcat-messages-XXXXXX)
helpers=()

cleanup() {
  for h in "${helpers[@]}"; do kill "$h" 2>/dev/null || true; wait "$h" 2>/dev/null || true; done
  rm -rf "$root"
}
trap cleanup EXIT

SLOUCHCAT_E2E_SEERR_PORT=$seerr_port node ../web/e2e/seerr-server.mjs >"$root/seerr.log" 2>&1 &
helpers+=($!)
SLOUCHCAT_E2E_TMDB_PORT=$tmdb_port node ../web/e2e/tmdb-server.mjs >"$root/tmdb.log" 2>&1 &
helpers+=($!)
../relay/bin/fcmfake -listen "127.0.0.1:$fcm_port" -credentials "$root/firebase.json" >"$root/fcmfake.log" 2>&1 &
helpers+=($!)
../relay/bin/push -add-key 1 -keyfile "$root/keys.json" >/dev/null
for _ in $(seq 1 40); do
  if [ -s "$root/firebase.json" ]; then break; fi
  sleep 0.25
done
[ -s "$root/firebase.json" ] || { echo "android-e2e-messages: fcmfake wrote no credentials" >&2; exit 1; }
RELAY_LISTEN="127.0.0.1:$relay_port" RELAY_PUSH_ORIGINS="http://127.0.0.1:$relay_port" RELAY_PUSH_KEYFILE="$root/keys.json" \
  RELAY_FCM_CREDENTIALS="$root/firebase.json" RELAY_FCM_BASE_URL="http://127.0.0.1:$fcm_port" ../relay/bin/push >"$root/relay.log" 2>&1 &
helpers+=($!)

export SLOUCHCAT_TEST_TMDB_BASE_URL="http://127.0.0.1:$tmdb_port/3"
export SLOUCHCAT_TEST_TMDB_IMAGE_BASE_URL="http://127.0.0.1:$tmdb_port/t/p"
export SLOUCHCAT_PUSH_RELAY="http://127.0.0.1:$relay_port"
CLASS=com.slouchcat.app.mobile.MessagesFlowTest PORT=$port ../scripts/android-e2e.sh
