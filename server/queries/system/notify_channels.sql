-- Notification channels, Telegram links, push subscriptions and deliveries (M8, A-6, plan 5.5;
-- package notify). Times are Unix milliseconds. Events and channel settings are JSON (see migration
-- 00016); an event is matched as its quoted name inside the events array (the names carry no quote
-- or escape). Endpoint, p256dh and auth of a push subscription are capability secrets and never
-- logged.

-- name: ChannelEnsureDefaults :exec
-- The three channel rows of an account with the defaults of 5.2 (e-mail on for guests with
-- announcements; everything else off); existing rows stay as they are.
INSERT OR IGNORE INTO account_channels (account_id, channel, enabled, events, config, updated_at)
SELECT a.id, c.channel,
       CASE WHEN c.channel = 'email' AND a.role = 'guest' THEN 1 ELSE 0 END,
       CASE WHEN a.role = 'guest' THEN '["announcements"]'
            ELSE '["new_content","errors","invitations","announcements"]' END,
       '{}', @now
FROM accounts AS a
JOIN (SELECT 'email' AS channel UNION ALL SELECT 'push' UNION ALL SELECT 'telegram') AS c
WHERE a.id = @account_id;

-- name: ChannelsByAccount :many
SELECT account_id, channel, enabled, events, config, updated_at
FROM account_channels
WHERE account_id = @account_id
ORDER BY CASE channel WHEN 'email' THEN 0 WHEN 'push' THEN 1 ELSE 2 END;

-- name: ChannelGet :one
SELECT account_id, channel, enabled, events, config, updated_at
FROM account_channels
WHERE account_id = @account_id AND channel = @channel;

-- name: ChannelUpsert :exec
INSERT INTO account_channels (account_id, channel, enabled, events, config, updated_at)
VALUES (@account_id, @channel, @enabled, @events, @config, @now)
ON CONFLICT (account_id, channel) DO UPDATE
SET enabled = excluded.enabled, events = excluded.events, config = excluded.config, updated_at = excluded.updated_at;

-- name: ChannelSetEnabled :execrows
UPDATE account_channels SET enabled = @enabled, updated_at = @now
WHERE account_id = @account_id AND channel = @channel;

-- name: ChannelSummaries :many
-- Every channel row of every account (account overview of the administration).
SELECT account_id, channel, enabled
FROM account_channels
ORDER BY account_id, CASE channel WHEN 'email' THEN 0 WHEN 'push' THEN 1 ELSE 2 END;

-- name: ChannelTargetsByEvent :many
-- Switched-on channels of every account that chose the event (announcements).
SELECT c.account_id, c.channel, c.config, a.email, a.role
FROM account_channels AS c
JOIN accounts AS a ON a.id = c.account_id
WHERE c.enabled = 1 AND instr(c.events, '"' || CAST(@event AS TEXT) || '"') > 0
ORDER BY c.account_id, c.channel;

-- name: ChannelTargetsByAccountEvent :many
-- Switched-on channels of one account that chose the event.
SELECT c.account_id, c.channel, c.config, a.email, a.role
FROM account_channels AS c
JOIN accounts AS a ON a.id = c.account_id
WHERE c.account_id = @account_id AND c.enabled = 1
  AND instr(c.events, '"' || CAST(@event AS TEXT) || '"') > 0
ORDER BY c.channel;

-- name: ChannelTargetsOwners :many
-- Switched-on channels of the main account that chose the event (the event errors goes to the main
-- account only).
SELECT c.account_id, c.channel, c.config, a.email, a.role
FROM account_channels AS c
JOIN accounts AS a ON a.id = c.account_id
WHERE a.role = 'owner' AND c.enabled = 1
  AND instr(c.events, '"' || CAST(@event AS TEXT) || '"') > 0
ORDER BY c.account_id, c.channel;

-- name: TelegramLinkUpsert :exec
-- A new link code for the account; an older open code of the account stops working.
INSERT INTO telegram_links (account_id, code, expires_at, created_at)
VALUES (@account_id, @code, @expires_at, @now)
ON CONFLICT (account_id) DO UPDATE SET code = excluded.code, expires_at = excluded.expires_at, created_at = excluded.created_at;

-- name: TelegramLinkGet :one
SELECT account_id, code, expires_at, created_at FROM telegram_links WHERE account_id = @account_id;

-- name: TelegramLinkByCode :one
-- The account of an open code.
SELECT account_id FROM telegram_links WHERE code = @code AND expires_at > @now;

-- name: TelegramLinkDelete :exec
DELETE FROM telegram_links WHERE account_id = @account_id;

-- name: TelegramLinksOpen :one
-- Whether any code is open (the bot is polled only then).
SELECT count(*) FROM telegram_links WHERE expires_at > @now;

-- name: TelegramLinksDeleteExpired :execrows
DELETE FROM telegram_links WHERE expires_at <= @now;

-- name: PushSubscriptionUpsert :one
-- One row per endpoint: the same endpoint again (also after a sign-in with another account in the
-- same browser) takes the new account, keys and device.
INSERT INTO push_subscriptions (account_id, device_id, endpoint, p256dh, auth, user_agent, device_name, created_at)
VALUES (@account_id, sqlc.narg('device_id'), @endpoint, @p256dh, @auth, sqlc.narg('user_agent'), sqlc.narg('device_name'), @now)
ON CONFLICT (endpoint) DO UPDATE
SET account_id = excluded.account_id, device_id = excluded.device_id, p256dh = excluded.p256dh, auth = excluded.auth,
    user_agent = excluded.user_agent, device_name = excluded.device_name, failures = 0
RETURNING id, account_id, device_id, device_name, created_at;

-- name: PushSubscriptionsByAccount :many
-- The subscriptions of an account for the settings (no secrets) and for sending (with keys).
SELECT id, account_id, device_id, endpoint, p256dh, auth, device_name, created_at, last_used_at, failures
FROM push_subscriptions
WHERE account_id = @account_id
ORDER BY created_at, id;

-- name: PushSubscriptionDelete :execrows
-- Removes a subscription of the account (0 rows for an unknown one or one of another account).
DELETE FROM push_subscriptions WHERE id = @id AND account_id = @account_id;

-- name: PushSubscriptionDeleteByID :exec
-- Removes a subscription the push service answered 404 or 410 for.
DELETE FROM push_subscriptions WHERE id = @id;

-- name: PushSubscriptionsDeleteAll :execrows
-- A new VAPID key pair invalidates every subscription.
DELETE FROM push_subscriptions;

-- name: PushSubscriptionUsed :exec
UPDATE push_subscriptions SET last_used_at = @now, failures = 0 WHERE id = @id;

-- name: PushSubscriptionFailed :exec
UPDATE push_subscriptions SET failures = failures + 1 WHERE id = @id;

-- name: DeliveryInsert :one
INSERT INTO deliveries (account_id, profile_id, channel, event, source, subject, body, link, created_at)
VALUES (@account_id, sqlc.narg('profile_id'), @channel, @event, sqlc.narg('source'), @subject, @body, sqlc.narg('link'), @now)
RETURNING id;

-- name: DeliveryGet :one
SELECT * FROM deliveries WHERE id = @id;

-- name: DeliveryClaim :execrows
-- Counts an attempt of a queued delivery; 0 rows when it was sent, failed or skipped meanwhile.
UPDATE deliveries SET attempts = attempts + 1 WHERE id = @id AND state = 'queued';

-- name: DeliverySent :exec
UPDATE deliveries SET state = 'sent', sent_at = @now, last_error = NULL WHERE id = @id;

-- name: DeliveryFailed :exec
-- Records the error of an attempt; state stays queued while the job retries and becomes failed after
-- the last attempt.
UPDATE deliveries SET state = @state, last_error = @last_error WHERE id = @id;

-- name: DeliverySkipped :exec
UPDATE deliveries SET state = 'skipped', last_error = sqlc.narg('reason') WHERE id = @id;

-- name: DeliveriesByAccount :many
-- The newest deliveries of an account.
SELECT id, account_id, profile_id, channel, event, subject, state, attempts, last_error, created_at, sent_at
FROM deliveries
WHERE account_id = @account_id
ORDER BY created_at DESC, id DESC
LIMIT @limit_count;

-- name: DeliveryRecentExists :one
-- Dedupe of the event errors: whether the same event of the same source went out since a time.
SELECT EXISTS (
    SELECT 1 FROM deliveries WHERE event = @event AND source = @source AND created_at >= @since
) AS recent;

-- name: DeliveriesDeleteOld :execrows
-- Finished deliveries older than before go (at most 500 per batch).
DELETE FROM deliveries
WHERE id IN (SELECT d.id FROM deliveries AS d WHERE d.state <> 'queued' AND d.created_at < @before LIMIT 500);
