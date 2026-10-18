#!/usr/bin/env bash
# Measures the M0 DoD values of the container (plan M0: healthy in < 5 s, idle RSS < 60 MB).
# Starts a throwaway container from IMAGE with a temporary /config and /cache, polls the
# health status every 100 ms, prints the time until "healthy", then the idle RSS.
#
# Env: IMAGE (default slouchcat:dev), MAX_HEALTHY_SECONDS (5), MAX_RSS_MB (60),
#      IDLE_SECONDS (3), DOCKER (docker).
set -euo pipefail

IMAGE="${IMAGE:-slouchcat:dev}"
MAX_HEALTHY_SECONDS="${MAX_HEALTHY_SECONDS:-5}"
MAX_RSS_MB="${MAX_RSS_MB:-60}"
IDLE_SECONDS="${IDLE_SECONDS:-3}"
DOCKER="${DOCKER:-docker}"

if ! command -v "$DOCKER" >/dev/null 2>&1; then
  echo "error: '$DOCKER' not found" >&2
  exit 1
fi

workdir="$(mktemp -d)"
name="slouchcat-healthy-$$"
# shellcheck disable=SC2317 # invoked via trap
cleanup() {
  "$DOCKER" rm -f "$name" >/dev/null 2>&1 || true
  rm -rf "$workdir"
}
trap cleanup EXIT

mkdir -p "$workdir/config" "$workdir/cache"
chmod 0777 "$workdir/config" "$workdir/cache"

start_ns="$(date +%s%N)"
"$DOCKER" run -d --name "$name" \
  --user "$(id -u):$(id -g)" \
  -v "$workdir/config:/config" \
  -v "$workdir/cache:/cache" \
  "$IMAGE" >/dev/null

status=""
deadline_ns=$((start_ns + 30 * 1000000000))
while :; do
  status="$("$DOCKER" inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$name")"
  now_ns="$(date +%s%N)"
  if [[ "$status" == "healthy" ]]; then
    break
  fi
  if [[ "$status" == "unhealthy" || "$status" == "none" ]] || ((now_ns > deadline_ns)); then
    echo "error: container did not become healthy (status: $status)" >&2
    "$DOCKER" logs "$name" >&2 || true
    exit 1
  fi
  if [[ "$("$DOCKER" inspect --format '{{.State.Running}}' "$name")" != "true" ]]; then
    echo "error: container exited" >&2
    "$DOCKER" logs "$name" >&2 || true
    exit 1
  fi
  sleep 0.1
done

elapsed_ms=$(((now_ns - start_ns) / 1000000))
printf 'healthy after %d.%03d s (limit %s s)\n' $((elapsed_ms / 1000)) $((elapsed_ms % 1000)) "$MAX_HEALTHY_SECONDS"

sleep "$IDLE_SECONDS"
rss_line="$("$DOCKER" exec "$name" cat /proc/1/status | grep VmRSS)"
rss_kb="$(awk '{print $2}' <<<"$rss_line")"
echo "idle after ${IDLE_SECONDS} s: $(tr -s '[:space:]' ' ' <<<"$rss_line")"
printf 'idle RSS: %d.%01d MB (limit %s MB)\n' $((rss_kb / 1024)) $(((rss_kb % 1024) * 10 / 1024)) "$MAX_RSS_MB"
"$DOCKER" stats --no-stream --format 'table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.PIDs}}' "$name"

fail=0
if ((elapsed_ms > MAX_HEALTHY_SECONDS * 1000)); then
  echo "FAIL: healthy took longer than ${MAX_HEALTHY_SECONDS} s" >&2
  fail=1
fi
if ((rss_kb > MAX_RSS_MB * 1024)); then
  echo "FAIL: idle RSS above ${MAX_RSS_MB} MB" >&2
  fail=1
fi
exit "$fail"
