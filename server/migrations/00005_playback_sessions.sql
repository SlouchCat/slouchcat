-- Playback sessions and stream tokens (plan 5.4, M1b step 2). A row exists for every started
-- playback (direct and HLS); the stream token `st` names its id and is an HMAC over id and item, so
-- expiry and revocation live here: every playlist, segment and Range request reads the row by its
-- primary key. expires_at = last_activity_at + 12 h (touched at most once per minute), revoked_at
-- is set on stop, logout and admin abort. Times are Unix milliseconds (UTC).

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;

CREATE TABLE playback_sessions (
    id               TEXT    PRIMARY KEY CHECK (length(id) = 26), -- ULID
    session_id       INTEGER NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    account_id       INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    profile_id       INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    item_id          TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    media_file_id    TEXT    NOT NULL,
    device_id        TEXT,
    mode             TEXT    NOT NULL CHECK (mode IN ('direct', 'remux', 'transcode')),
    backend          TEXT,                                         -- transcode backend, NULL for direct
    reason           TEXT,                                         -- why not direct play
    started_at       INTEGER NOT NULL,
    last_activity_at INTEGER NOT NULL,
    expires_at       INTEGER NOT NULL,
    revoked_at       INTEGER
) STRICT;

CREATE INDEX playback_sessions_session ON playback_sessions (session_id);
CREATE INDEX playback_sessions_expires ON playback_sessions (expires_at);

PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
