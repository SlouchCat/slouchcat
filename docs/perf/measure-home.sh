#!/usr/bin/env bash
# P-11 measurement of the start page (docs/perf/M10.md section 3, docs/perf/M11.md): 60 requests of
# GET /api/v1/home after five warm-ups, from the host the server runs on (no network path), with a
# session that has a profile selected. Prints p50, p95 and max in milliseconds and the number of
# answers that were not 200.
#
#   SLOUCHCAT_URL=http://localhost:8096 SLOUCHCAT_EMAIL=… SLOUCHCAT_PASSWORD=… \
#     SLOUCHCAT_PROFILE_ID=1 bash docs/perf/measure-home.sh [path]
#
# path defaults to /api/v1/home. SLOUCHCAT_BODY='{"query":"Se"}' turns the request into a POST with
# that JSON body (the search of P-11, M10 section 3). The password is read from the environment only
# and never printed.
set -euo pipefail

url=${SLOUCHCAT_URL:-http://localhost:8096}
path=${1:-/api/v1/home}
runs=${SLOUCHCAT_RUNS:-60}
jar=$(mktemp)
trap 'rm -f "$jar"' EXIT

curl -fsS -o /dev/null -c "$jar" -b "$jar" -H 'X-Requested-With: Slouchcat' -H 'Content-Type: application/json' \
  --data-binary "{\"email\":\"$SLOUCHCAT_EMAIL\",\"password\":\"$SLOUCHCAT_PASSWORD\"}" "$url/api/v1/auth/login"
curl -fsS -o /dev/null -c "$jar" -b "$jar" -H 'X-Requested-With: Slouchcat' -H 'Content-Type: application/json' \
  -X POST --data-binary '{}' "$url/api/v1/auth/profiles/${SLOUCHCAT_PROFILE_ID:-1}/select"

req=(-b "$jar")
if [ -n "${SLOUCHCAT_BODY:-}" ]; then
  req+=(-X POST -H 'X-Requested-With: Slouchcat' -H 'Content-Type: application/json' --data-binary "$SLOUCHCAT_BODY")
fi

for _ in 1 2 3 4 5; do
  curl -sS -o /dev/null "${req[@]}" "$url$path"
done

times=()
bad=0
for _ in $(seq "$runs"); do
  out=$(curl -sS -o /dev/null "${req[@]}" -w '%{http_code} %{time_total}' "$url$path")
  code=${out%% *}
  t=${out##* }
  [ "$code" = 200 ] || bad=$((bad + 1))
  times+=("$t")
done

sorted=$(printf '%s\n' "${times[@]}" | sort -n)
pick() { printf '%s\n' "$sorted" | awk -v p="$1" -v n="$runs" 'NR == int((p * n + 99) / 100) { printf "%.1f", $1 * 1000 }'; }
echo "$path: n=$runs p50=$(pick 50) ms p95=$(pick 95) ms max=$(printf '%s\n' "$sorted" | tail -1 | awk '{ printf "%.1f", $1 * 1000 }') ms not-200=$bad"
