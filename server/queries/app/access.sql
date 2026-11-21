-- Devices, PIN locks, profile and account management, invitations (package store/appq). Owner:
-- auth (all of it) and admin (accounts, invitations). Never touches title tables. Times are Unix
-- milliseconds.

-- name: UpsertDevice :exec
INSERT INTO devices (id, user_agent, created_at, last_seen_at) VALUES (@id, @user_agent, @now, @now)
ON CONFLICT (id) DO UPDATE SET last_seen_at = excluded.last_seen_at;

-- name: GetPinAttempt :one
SELECT * FROM pin_attempts WHERE profile_id = @profile_id AND device_id = @device_id;

-- name: UpsertPinAttempt :exec
INSERT INTO pin_attempts (profile_id, device_id, failures, locked_until, updated_at)
VALUES (@profile_id, @device_id, @failures, @locked_until, @now)
ON CONFLICT (profile_id, device_id) DO UPDATE
SET failures = excluded.failures, locked_until = excluded.locked_until, updated_at = excluded.updated_at;

-- name: DeletePinAttempt :exec
DELETE FROM pin_attempts WHERE profile_id = @profile_id AND device_id = @device_id;

-- name: DeletePinAttemptsByProfile :exec
DELETE FROM pin_attempts WHERE profile_id = @profile_id;

-- name: ListProfilesByAccount :many
SELECT * FROM profiles WHERE account_id = @account_id ORDER BY is_default DESC, id;

-- name: GetProfileOfAccount :one
SELECT * FROM profiles WHERE id = @id AND account_id = @account_id;

-- name: CountProfilesByAccount :one
SELECT count(*) FROM profiles WHERE account_id = @account_id;

-- name: UpdateProfile :exec
UPDATE profiles
SET name = @name, max_rating = @max_rating, language = @language, metadata_language = sqlc.narg('metadata_language'), accent = @accent,
    all_libraries = @all_libraries, max_bitrate_kbps = sqlc.narg('max_bitrate_kbps'), max_height = sqlc.narg('max_height'),
    max_streams = sqlc.narg('max_streams'), watch_limit_minutes = sqlc.narg('watch_limit_minutes'),
    audio_languages = @audio_languages, subtitle_mode = @subtitle_mode, subtitle_languages = @subtitle_languages,
    updated_at = @now
WHERE id = @id;

-- name: SetProfilePlaybackPrefs :execrows
-- The language and subtitle preferences of a profile of the account (JSON arrays of ISO 639-1
-- codes); 0 rows = no such profile in the account.
UPDATE profiles
SET audio_languages = @audio_languages, subtitle_mode = @subtitle_mode, subtitle_languages = @subtitle_languages,
    updated_at = @now
WHERE id = @id AND account_id = @account_id;

-- name: SetProfilePin :exec
UPDATE profiles SET pin_hash = @pin_hash, updated_at = @now WHERE id = @id;

-- name: DeleteProfile :exec
DELETE FROM profiles WHERE id = @id;

-- name: ListProfileLibrariesByAccount :many
SELECT pl.profile_id, pl.library_id FROM profile_libraries AS pl
JOIN profiles AS p ON p.id = pl.profile_id
WHERE p.account_id = @account_id
ORDER BY pl.profile_id, pl.library_id;

-- name: ListProfileLibraries :many
SELECT library_id FROM profile_libraries WHERE profile_id = @profile_id ORDER BY library_id;

-- name: DeleteProfileLibraries :exec
DELETE FROM profile_libraries WHERE profile_id = @profile_id;

-- name: InsertProfileLibrary :exec
INSERT INTO profile_libraries (profile_id, library_id) VALUES (@profile_id, @library_id);

-- name: SetSessionProfile :execrows
-- The device of a session is fixed at its start (PIN locks must not depend on a cookie the client
-- can drop); the profile must still exist in the session's account.
UPDATE sessions SET profile_id = @profile_id, last_seen_at = @now
WHERE sessions.id = @id AND sessions.revoked_at IS NULL
  AND EXISTS (SELECT 1 FROM profiles AS p WHERE p.id = @profile_id AND p.account_id = sessions.account_id);

-- name: ClearSessionsProfile :execrows
-- Sessions that have the profile active fall back to the picker (before the profile is deleted).
UPDATE sessions SET profile_id = NULL WHERE profile_id = @profile_id;

-- name: AssignSessionDevice :exec
UPDATE sessions SET device_id = @device_id WHERE id = @id AND device_id IS NULL;

-- name: ClampProfileRatings :execrows
UPDATE profiles SET max_rating = @max_rating, updated_at = @now WHERE account_id = @account_id AND max_rating > @max_rating;

-- name: DeleteStaleDevices :execrows
-- A device with a living app or device session stays however long it was silent: those sessions
-- never expire, and losing the device would drop its downloads and give it a new id (ADR 0040).
DELETE FROM devices
WHERE devices.last_seen_at < @before
  AND NOT EXISTS (SELECT 1 FROM sessions AS s
                  WHERE s.device_id = devices.id AND s.kind <> 'browser' AND s.revoked_at IS NULL AND s.expires_at > @now);

-- name: DeviceHeldByOtherAccount :one
-- Whether another account than @account_id holds a living session or a download on the device:
-- then a device id a client brings along to appLogin is not taken (concept 27, 4.2).
SELECT EXISTS (SELECT 1 FROM sessions AS s
               WHERE s.device_id = @id AND s.account_id <> @account_id AND s.revoked_at IS NULL AND s.expires_at > @now)
    OR EXISTS (SELECT 1 FROM downloads AS d WHERE d.device_id = @id AND d.account_id <> @account_id);

-- name: UpsertNamedDevice :exec
-- The device of an appLogin with the name and type the app reported; a later playback start of an
-- app or device session keeps them (RecordDeviceProfile is called without name and type).
INSERT INTO devices (id, user_agent, created_at, last_seen_at, name, type)
VALUES (@id, sqlc.narg('user_agent'), @now, @now, @name, @type)
ON CONFLICT (id) DO UPDATE SET last_seen_at = excluded.last_seen_at, name = excluded.name, type = excluded.type;

-- name: DeleteStalePinAttempts :execrows
DELETE FROM pin_attempts WHERE updated_at < @before;

-- name: TouchAccount :exec
UPDATE accounts SET last_seen_at = @now WHERE id = @id;

-- name: InsertInvitedAccount :one
-- An invitation never creates an administrator: the role is a literal in this file, not a
-- parameter, so no request body and no slip in a handler can raise it (ADR 0014, lock 4).
INSERT INTO accounts (email, password_hash, role, max_rating, all_libraries, can_request, invited_by, created_at, updated_at)
VALUES (@email, @password_hash, 'user', @max_rating, @all_libraries, @can_request, sqlc.narg('invited_by'), @now, @now)
RETURNING *;

-- name: CountOtherAdmins :one
-- How many administrators would be left without this account. Read inside the write transaction
-- of SetAccountRole and DeleteAccount, so two concurrent demotions cannot both win (ADR 0014).
SELECT count(*) FROM accounts WHERE role = 'admin' AND id <> @id;

-- name: SetAccountRole :execrows
-- The appointment lifts the account to 18 and all libraries (the table CHECK demands it, and the
-- administration views read system-wide anyway); taking the role away leaves both as they are
-- (decision 12 f). The owner is never touched here: the service answers owner_protected first,
-- and `is_owner = 0` is the second lock in case it ever does not.
UPDATE accounts
SET role = @role,
    max_rating = CASE WHEN @role = 'admin' THEN 18 ELSE max_rating END,
    all_libraries = CASE WHEN @role = 'admin' THEN 1 ELSE all_libraries END,
    updated_at = @now
WHERE id = @id AND is_owner = 0;

-- name: UpdateAccountLimits :exec
UPDATE accounts
SET max_rating = @max_rating, all_libraries = @all_libraries, max_bitrate_kbps = sqlc.narg('max_bitrate_kbps'),
    max_height = sqlc.narg('max_height'), max_streams = sqlc.narg('max_streams'), updated_at = @now
WHERE id = @id;

-- name: DeleteAccount :execrows
DELETE FROM accounts WHERE id = @id;

-- name: ListAccountLibraries :many
SELECT library_id FROM account_libraries WHERE account_id = @account_id ORDER BY library_id;

-- name: DeleteAccountLibraries :exec
DELETE FROM account_libraries WHERE account_id = @account_id;

-- name: InsertAccountLibrary :exec
INSERT INTO account_libraries (account_id, library_id) VALUES (@account_id, @library_id);

-- name: RevokePlaybackSessionsByAccount :many
UPDATE playback_sessions SET revoked_at = @now, stopped_by = @stopped_by
WHERE account_id = @account_id AND revoked_at IS NULL
RETURNING id;

-- name: RevokePlaybackSessionsByProfile :many
-- stopped_by: revoked when the profile is deleted, limit when its daily watch time is used up.
UPDATE playback_sessions SET revoked_at = @now, stopped_by = @stopped_by
WHERE profile_id = @profile_id AND revoked_at IS NULL
RETURNING id;

-- Device overview of the administration (W-7, M4).

-- name: ListDevicesAdmin :many
-- Devices, last contact first, with the owner of the last playback start and the number of running
-- streams (the activity window of CountActivePlaybackSessions).
-- session_kind is the kind of the device's living session with the latest contact, '' without one
-- (ADR 0040).
SELECT d.id, d.user_agent, d.created_at, d.last_seen_at, d.name, d.type, d.capabilities, d.capabilities_at,
       a.email AS owner_email, p.name AS owner_profile,
       CAST(COALESCE((SELECT s.kind FROM sessions AS s
                      WHERE s.device_id = d.id AND s.revoked_at IS NULL AND s.expires_at > @now
                      ORDER BY s.last_seen_at DESC, s.id DESC LIMIT 1), '') AS TEXT) AS session_kind,
       (SELECT count(*) FROM playback_sessions AS ps
        JOIN sessions AS s ON s.id = ps.session_id
        WHERE ps.device_id = d.id AND ps.revoked_at IS NULL AND ps.expires_at > @now
          AND ps.last_activity_at >= @active_after AND s.revoked_at IS NULL AND s.expires_at > @now) AS active_streams
FROM devices AS d
LEFT JOIN accounts AS a ON a.id = d.account_id
LEFT JOIN profiles AS p ON p.id = d.profile_id
ORDER BY d.last_seen_at DESC, d.id;

-- name: RevokeDeviceSessions :many
UPDATE sessions SET revoked_at = @now WHERE device_id = @device_id AND revoked_at IS NULL
RETURNING id;

-- name: DeletePinAttemptsByDevice :exec
DELETE FROM pin_attempts WHERE device_id = @device_id;

-- name: DeleteDevice :execrows
DELETE FROM devices WHERE id = @id;

-- name: DeleteDevicePushSubscriptions :execrows
-- The push subscriptions of a device, of every account: deleteDevice removes them before the device
-- row, whose deletion would only clear their device_id (concept 27, 4.5).
DELETE FROM push_subscriptions WHERE device_id = @device_id;

-- My devices (concept 27, 4.5, ADR 0040): the devices of an account and its sign-out. The
-- account of a session or a download decides, never devices.account_id (the owner of the last
-- playback start).

-- name: ListMyDevices :many
-- The devices that hold a living session or a download of the account, last contact first, with the
-- kind and start of the account's living session with the latest contact ('' and 0 without one) and
-- the account's downloads on the device.
SELECT d.id, d.name, d.type, d.last_seen_at,
       CAST(COALESCE((SELECT s.kind FROM sessions AS s
                      WHERE s.device_id = d.id AND s.account_id = @account_id AND s.revoked_at IS NULL AND s.expires_at > @now
                      ORDER BY s.last_seen_at DESC, s.id DESC LIMIT 1), '') AS TEXT) AS session_kind,
       CAST(COALESCE((SELECT s.created_at FROM sessions AS s
                      WHERE s.device_id = d.id AND s.account_id = @account_id AND s.revoked_at IS NULL AND s.expires_at > @now
                      ORDER BY s.last_seen_at DESC, s.id DESC LIMIT 1), 0) AS INTEGER) AS signed_in_at,
       (SELECT count(*) FROM downloads AS dl WHERE dl.device_id = d.id AND dl.account_id = @account_id) AS downloads
FROM devices AS d
WHERE EXISTS (SELECT 1 FROM sessions AS s
              WHERE s.device_id = d.id AND s.account_id = @account_id AND s.revoked_at IS NULL AND s.expires_at > @now)
   OR EXISTS (SELECT 1 FROM downloads AS dl WHERE dl.device_id = d.id AND dl.account_id = @account_id)
ORDER BY d.last_seen_at DESC, d.id;

-- name: DeviceHeldByAccount :one
-- Whether the account holds a living session or a download on the device (signOutMyDevice: 404 otherwise).
SELECT EXISTS (SELECT 1 FROM sessions AS s
               WHERE s.device_id = @id AND s.account_id = @account_id AND s.revoked_at IS NULL AND s.expires_at > @now)
    OR EXISTS (SELECT 1 FROM downloads AS d WHERE d.device_id = @id AND d.account_id = @account_id);

-- name: ListAccountDeviceSessions :many
-- The unrevoked sessions of the account on the device: their streams end before the sign-out.
SELECT id FROM sessions WHERE device_id = @device_id AND account_id = @account_id AND revoked_at IS NULL ORDER BY id;

-- name: RevokeAccountDeviceSessions :execrows
UPDATE sessions SET revoked_at = @now WHERE device_id = @device_id AND account_id = @account_id AND revoked_at IS NULL;

-- name: RevokePlaybackSessionsByAccountDevice :exec
-- Any playback left of the account's sessions on the device, in the transaction of the sign-out.
UPDATE playback_sessions SET revoked_at = @now, stopped_by = @stopped_by
WHERE revoked_at IS NULL
  AND session_id IN (SELECT s.id FROM sessions AS s WHERE s.device_id = @device_id AND s.account_id = @account_id);

-- name: DeleteAccountDevicePushSubscriptions :execrows
DELETE FROM push_subscriptions WHERE device_id = @device_id AND account_id = @account_id;

-- name: DeleteAccountDeviceRules :execrows
-- The auto-download rules of the account's profiles on the device.
DELETE FROM download_rules
WHERE download_rules.device_id = @device_id AND download_rules.profile_id IN (SELECT p.id FROM profiles AS p WHERE p.account_id = @account_id);

-- name: DeleteAccountDeviceDownloads :execrows
-- The account's downloads on the device; the trigger downloads_tombstone writes their tombstones,
-- which the device reads from the revocation list. Their files go with the next cleanup of package
-- offline (directories without a download row, every 10 minutes), as after deleteDevice.
DELETE FROM downloads WHERE device_id = @device_id AND account_id = @account_id;

-- name: DeleteSessionPushSubscriptions :execrows
-- logout of an app or device session: the account's push subscriptions of the session's device go
-- with it (concept 27, 4.5). A browser session keeps them (a browser does not subscribe again by
-- itself, F-40).
DELETE FROM push_subscriptions
WHERE push_subscriptions.device_id = (SELECT s.device_id FROM sessions AS s WHERE s.id = @session_id AND s.kind <> 'browser')
  AND push_subscriptions.account_id = (SELECT s.account_id FROM sessions AS s WHERE s.id = @session_id);

-- name: DeleteOtherAppDevicePushSubscriptions :execrows
-- A new password: the account's push subscriptions of the devices whose app or device session it
-- revokes (every living one but the calling session's device). Browsers keep theirs (F-40).
DELETE FROM push_subscriptions
WHERE push_subscriptions.account_id = @account_id
  AND push_subscriptions.device_id IN (SELECT s.device_id FROM sessions AS s
                    WHERE s.account_id = @account_id AND s.id <> @keep_id AND s.kind <> 'browser'
                      AND s.revoked_at IS NULL AND s.expires_at > @now AND s.device_id IS NOT NULL)
  AND push_subscriptions.device_id NOT IN (SELECT s.device_id FROM sessions AS s WHERE s.id = @keep_id AND s.device_id IS NOT NULL);

-- name: InsertInvitation :one
INSERT INTO invitations (token_hash, note, max_rating, all_libraries, can_request, wish_limit_mode,
                         wish_movie_limit, wish_movie_days, wish_tv_limit, wish_tv_days, created_by, created_at, expires_at)
VALUES (@token_hash, @note, @max_rating, @all_libraries, @can_request, @wish_limit_mode,
        sqlc.narg('wish_movie_limit'), sqlc.narg('wish_movie_days'), sqlc.narg('wish_tv_limit'), sqlc.narg('wish_tv_days'),
        sqlc.narg('created_by'), @now, @expires_at)
RETURNING *;

-- name: InsertInvitationLibrary :exec
INSERT INTO invitation_libraries (invitation_id, library_id) VALUES (@invitation_id, @library_id);

-- name: GetInvitationByTokenHash :one
SELECT * FROM invitations WHERE token_hash = @token_hash;

-- name: GetInvitation :one
SELECT * FROM invitations WHERE id = @id;

-- name: ListInvitations :many
SELECT i.*, a.email AS used_by_email FROM invitations AS i
LEFT JOIN accounts AS a ON a.id = i.used_by
ORDER BY i.created_at DESC, i.id DESC;

-- name: ListInvitationLibraries :many
SELECT invitation_id, library_id FROM invitation_libraries ORDER BY invitation_id, library_id;

-- name: ListInvitationLibraryIDs :many
SELECT library_id FROM invitation_libraries WHERE invitation_id = @invitation_id ORDER BY library_id;

-- name: MarkInvitationUsed :execrows
-- One-time: only an open (unused, unrevoked, unexpired) invitation can be taken.
UPDATE invitations SET used_at = @now, used_by = @used_by
WHERE id = @id AND used_at IS NULL AND revoked_at IS NULL AND expires_at > @now;

-- name: RevokeInvitation :execrows
UPDATE invitations SET revoked_at = @now WHERE id = @id AND used_at IS NULL AND revoked_at IS NULL;

-- name: SetAccountCanRequest :execrows
-- The request right of an account, the cap of its profiles (concept 25, data model 2.3); set by an
-- administrator.
UPDATE accounts SET can_request = @can_request, updated_at = @now WHERE id = @id;

-- name: SetProfileCanRequest :exec
-- The request right of a profile, set by the primary profile; effective only with the account's.
UPDATE profiles SET can_request = @can_request, updated_at = @now WHERE id = @id;
