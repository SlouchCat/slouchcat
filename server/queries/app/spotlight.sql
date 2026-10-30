-- Spotlight configuration (M11, E-7, AN-3, ADR 0011): one row, id 1. Package admin writes it;
-- discover reads item_count and min_hits for the row, visibility reads interval_hours for the
-- staleness limit (twice the interval). The table names no title, so it lives in appq, which
-- discover may import (systemq it may not, .golangci.yml rule "systemq").

-- name: GetSpotlightConfig :one
SELECT * FROM spotlight_config WHERE id = 1;

-- name: SetSpotlightConfig :exec
UPDATE spotlight_config
SET enabled = @enabled, list_key = @list_key, list_url = @list_url, media = @media,
    item_count = @item_count, min_hits = @min_hits, interval_hours = @interval_hours,
    rotation_seconds = @rotation_seconds, updated_at = @now
WHERE id = 1;

-- name: SetSpotlightManualRefresh :exec
UPDATE spotlight_config SET last_manual_refresh = @now WHERE id = 1;
