-- Incoming Radarr/Sonarr webhooks (M9, I-8, decision 5.6; package integrations). Times are Unix
-- milliseconds. The token is never stored: an address is found by the SHA-256 of the token in its
-- path. The event log is trimmed to 20 rows per integration by a trigger (migration 00017).

-- name: InsertWebhookIntegration :one
INSERT INTO webhook_integrations (name, kind, token_hash, token_tail, path_map, created_at, updated_at)
VALUES (@name, @kind, @token_hash, @token_tail, @path_map, @now, @now)
RETURNING *;

-- name: ListWebhookIntegrations :many
SELECT * FROM webhook_integrations ORDER BY id;

-- name: GetWebhookIntegration :one
SELECT * FROM webhook_integrations WHERE id = @id;

-- name: GetWebhookIntegrationByTokenHash :one
SELECT * FROM webhook_integrations WHERE token_hash = @token_hash;

-- name: UpdateWebhookIntegrationToken :one
UPDATE webhook_integrations SET token_hash = @token_hash, token_tail = @token_tail, updated_at = @now
WHERE id = @id
RETURNING *;

-- name: UpdateWebhookIntegrationPathMap :one
UPDATE webhook_integrations SET path_map = @path_map, updated_at = @now WHERE id = @id
RETURNING *;

-- name: TouchWebhookIntegration :exec
UPDATE webhook_integrations SET last_seen_at = @now WHERE id = @id;

-- name: DeleteWebhookIntegration :exec
DELETE FROM webhook_integrations WHERE id = @id;

-- name: InsertWebhookEvent :one
INSERT INTO webhook_events (integration_id, received_at, event, reported_path, resolved_path, result)
VALUES (@integration_id, @now, @event, @reported_path, @resolved_path, @result)
RETURNING *;

-- name: ListWebhookEvents :many
SELECT * FROM webhook_events WHERE integration_id = @integration_id ORDER BY id DESC;

-- name: CountWebhookEventsSince :one
-- Reports for the dashboard tile: how many events arrived since a point in time (5.6).
SELECT COUNT(*) FROM webhook_events WHERE received_at >= @since;
