#!/bin/sh
# Sets up the demo server without manual steps (concept 27, F-10). Runs in the service "seed" of
# compose.yaml once the server is healthy, and again on every `docker compose up`: each step
# checks first whether it is done.
#
#   1. First-start wizard over the API (runSetup): main account, its profile and the library
#      "Films" on /media/Movies (and "Series" on /media/TV if that folder exists).
#   2. Waits until the scan is done and at least one title is visible.
#   3. Review account for Google's reviewers: an invitation of the main account (all libraries,
#      rating ceiling 18) accepted with DEMO_REVIEW_EMAIL and DEMO_REVIEW_PASSWORD. It gets its own
#      primary profile (DEMO_REVIEW_PROFILE) and no administration rights.
set -eu

BASE="${DEMO_SERVER:-http://slouchcat:8096}/api/v1"
CSRF='X-Requested-With: Slouchcat'
JSON='Content-Type: application/json'
OWNER_JAR=/tmp/owner.jar
REVIEW_JAR=/tmp/review.jar

fail() {
  echo "seed: $*" >&2
  exit 1
}

# The values go into JSON as they are: no quote, no backslash (README, «Umgebungsdatei»).
check_value() { # <name> <value> <min length>
  [ "${#2}" -ge "$3" ] || fail "$1 must have at least $3 characters"
  case "$2" in *'"'* | *'\'* | *' '*) fail "$1 must not contain quotes, backslashes or spaces" ;; esac
}

check_value DEMO_OWNER_EMAIL "${DEMO_OWNER_EMAIL:-}" 3
check_value DEMO_OWNER_PASSWORD "${DEMO_OWNER_PASSWORD:-}" 10
check_value DEMO_REVIEW_EMAIL "${DEMO_REVIEW_EMAIL:-}" 3
check_value DEMO_REVIEW_PASSWORD "${DEMO_REVIEW_PASSWORD:-}" 10
REVIEW_PROFILE="${DEMO_REVIEW_PROFILE:-Demo}"
LANGUAGE="${DEMO_LANGUAGE:-en}"

# post <jar> <path> <json>: prints the body, returns non-zero on an HTTP error.
post() {
  curl -sS -f -b "$1" -c "$1" -H "$CSRF" -H "$JSON" -X POST -d "$3" "$BASE$2"
}

i=0
until curl -fsS "$BASE/healthz" >/dev/null 2>&1; do
  i=$((i + 1))
  [ "$i" -le 120 ] || fail "server not healthy after 120 s"
  sleep 1
done

owner_login="{\"email\":\"$DEMO_OWNER_EMAIL\",\"password\":\"$DEMO_OWNER_PASSWORD\"}"
if curl -fsS "$BASE/setup" | grep -q '"setupRequired":true'; then
  library='{"name":"Films","kind":"video","roots":[{"path":"/media/Movies","content":"movie","providers":["tmdb"]}]}'
  post "$OWNER_JAR" /setup "{\"email\":\"$DEMO_OWNER_EMAIL\",\"password\":\"$DEMO_OWNER_PASSWORD\",\"language\":\"$LANGUAGE\",\"profileName\":\"Owner\",\"library\":$library}" >/dev/null ||
    fail "runSetup failed"
  echo "seed: main account and library Films created"
fi
post "$OWNER_JAR" /auth/login "$owner_login" >/dev/null || fail "login of the main account failed"

if [ -d /media/TV ] && ! curl -fsS -b "$OWNER_JAR" "$BASE/admin/libraries" | grep -q '"name":"Series"'; then
  post "$OWNER_JAR" /admin/libraries '{"name":"Series","kind":"video","roots":[{"path":"/media/TV","content":"series","providers":["tmdb"]}]}' >/dev/null ||
    fail "library Series not created"
  echo "seed: library Series created"
fi

i=0
while :; do
  libraries="$(curl -fsS -b "$OWNER_JAR" "$BASE/admin/libraries")"
  case "$libraries" in
  *'"state":"failed"'* | *'"state":"aborted"'*) fail "a library scan failed: $libraries" ;;
  esac
  if ! printf '%s' "$libraries" | grep -Eq '"state":"(never|queued|running)"'; then
    break
  fi
  i=$((i + 1))
  [ "$i" -le 900 ] || fail "scan not done after 900 s"
  sleep 1
done

review_login="{\"email\":\"$DEMO_REVIEW_EMAIL\",\"password\":\"$DEMO_REVIEW_PASSWORD\"}"
if post "$REVIEW_JAR" /auth/login "$review_login" >/dev/null 2>&1; then
  echo "seed: review account exists"
else
  created="$(post "$OWNER_JAR" /admin/invitations '{"note":"Google Play review","maxRating":18,"allLibraries":true}')" ||
    fail "invitation not created"
  token="$(printf '%s' "$created" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')"
  [ -n "$token" ] || fail "invitation without token"
  post "$REVIEW_JAR" "/invitations/$token/accept" "{\"email\":\"$DEMO_REVIEW_EMAIL\",\"password\":\"$DEMO_REVIEW_PASSWORD\",\"profileName\":\"$REVIEW_PROFILE\",\"language\":\"$LANGUAGE\"}" >/dev/null ||
    fail "review account not created (is the e-mail already used by another account?)"
  echo "seed: review account created"
fi

# The review account must see titles: the same check as the reviewer's first page.
post "$REVIEW_JAR" /auth/login "$review_login" >/dev/null || fail "login of the review account failed"
home="$(curl -fsS -b "$REVIEW_JAR" "$BASE/home")" || fail "start page of the review account failed"
count="$(printf '%s' "$home" | grep -o '"title":"' | wc -l | tr -d ' ')"
[ "$count" -gt 0 ] || fail "the review account sees no title"
echo "seed: done, the review account sees $count titles on the start page"
