-- Spotlight configuration (M11, E-7, AN-3, ADR 0011): one row, id 1. Package admin writes it;
-- discover reads item_count and min_hits for the row, visibility reads interval_hours for the
-- staleness limit (twice the interval). The table names no title, so it lives in appq, which
-- discover may import (systemq it may not, .golangci.yml rule "systemq").

-- name: GetSpotlightConfig :one
SELECT * FROM spotlight_config WHERE id = 1;
