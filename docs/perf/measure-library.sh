#!/usr/bin/env bash
# The sister of measure-home.sh for the library page (concept 08, section 8; docs/perf/startseite.md):
# runs measure-home.sh against the library pages that matter, each 60 requests after five warm-ups,
# with the same session handling and the same output per line (p50, p95, max, not-200).
#
#   SLOUCHCAT_URL=http://localhost:8096 SLOUCHCAT_EMAIL=… SLOUCHCAT_PASSWORD=… \
#     SLOUCHCAT_PROFILE_ID=1 SLOUCHCAT_MOVIES_ID=1 SLOUCHCAT_SERIES_ID=2 bash docs/perf/measure-library.sh
#
# SLOUCHCAT_MOVIES_ID and SLOUCHCAT_SERIES_ID are the ids of the film and series libraries
# (GET /api/v1/libraries/visible); the cases are the paper's: the side navigation, the first page of
# the film library in every default view, and the heaviest one, "series, status started, sorted by
# title" (the EXISTS over the episodes meets the large library). Since concept 17 section 6 the grid
# reads pages of 100 and the letter bar jumps: page 0 sorted by title carries letterStarts, a page
# further down (offset 900) carries none; the letter filter of before 1.21.0 is gone from the app.
set -euo pipefail

here=$(cd "$(dirname "$0")" && pwd)
movies=${SLOUCHCAT_MOVIES_ID:-1}
series=${SLOUCHCAT_SERIES_ID:-2}

for path in \
  "/api/v1/libraries/visible" \
  "/api/v1/libraries/$movies/items" \
  "/api/v1/libraries/$movies/items?sort=added" \
  "/api/v1/libraries/$movies/items?sort=vote" \
  "/api/v1/libraries/$movies/items?sort=title&limit=100" \
  "/api/v1/libraries/$movies/items?sort=title&offset=900&limit=100" \
  "/api/v1/libraries/$series/items" \
  "/api/v1/libraries/$series/items?sort=played" \
  "/api/v1/libraries/$series/items?sort=title&status=started&limit=100" \
  "/api/v1/picks"; do
  bash "$here/measure-home.sh" "$path"
done
