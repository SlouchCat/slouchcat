-- Concept 25 stage 2 «Anfordern» (08-datenmodell-api.md sections 2.4 to 2.7): the wishes, their
-- seasons, their requesters and their history. Only package wishes reads and writes them
-- (store/wishq). A wish never joins items: item_id is the value package visibility returned
-- (CLAUDE.md rule 2). Times are Unix milliseconds.
--
--   wishes           one film or one set of seasons of a series (media 'show'). A film has one
--                    request in Seerr (seerr_request_id); a series keeps the request per season in
--                    wish_seasons. seerr_user_id freezes the Seerr user the wish goes out as: the
--                    user of the account, else the shared user «slouchcat-gaeste» (F-27, N-100,
--                    N-102); a later mapping change moves nothing. state queued = waiting in the
--                    outbox; partial only for a show; failed always carries a fail_reason.
--                    withdraw_pending = still to be deleted in Seerr after a withdrawal.
--   wish_seasons     the seasons of a series wish (never season 0). tmdb_id is a copy of the
--                    wish's for the partial unique index «one open row per TMDB id and season»; a
--                    trigger refuses a copy that differs from the wish or a season of a film wish.
--   wish_requesters  the owner and the profiles that joined (F-7); notified_at is the guard against
--                    a second «verfügbar» for the profile (06-benachrichtigungen.md section 7).
--   wish_events      the history of a wish: kind ^[a-z_]+$ without an enum (pattern of the
--                    activity log, ADR 0019), detail JSON without secrets.
--
-- Partial unique indexes: wishes_open_movie (at most one open film wish per TMDB id, Anhang F 2.11),
-- wish_seasons_open (at most one open row per TMDB id and season), wishes_request (a film request
-- belongs to one wish at most), wish_requesters_owner (one owner per wish).
--
-- New tables only, nothing is rebuilt; the frame of the template stays.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE wishes (
    id               INTEGER PRIMARY KEY,
    media            TEXT    NOT NULL CHECK (media IN ('movie', 'show')),
    tmdb_id          INTEGER NOT NULL CHECK (tmdb_id > 0),
    tvdb_id          INTEGER CHECK (tvdb_id IS NULL OR tvdb_id > 0),
    imdb_id          TEXT    CHECK (imdb_id IS NULL OR imdb_id <> ''),
    title            TEXT    NOT NULL CHECK (title <> ''),
    year             INTEGER,
    origin           TEXT    NOT NULL CHECK (origin IN ('slouchcat', 'jellyseerr')),
    seerr_request_id INTEGER CHECK (seerr_request_id IS NULL OR seerr_request_id > 0),
    seerr_media_id   INTEGER,
    seerr_user_id    INTEGER REFERENCES seerr_users(seerr_id) ON DELETE SET NULL,
    state            TEXT    NOT NULL CHECK (state IN ('queued', 'pending', 'processing', 'partial',
                                                       'available', 'declined', 'failed', 'withdrawn')),
    state_detail     TEXT    CHECK (state_detail IS NULL OR state_detail IN ('importing', 'not_found_in_library')),
    fail_reason      TEXT    CHECK (fail_reason IS NULL OR fail_reason IN (
                                 'quota', 'blocklisted', 'no_permission', 'arr_failed', 'unreachable_24h',
                                 'seerr_request_gone', 'seerr_media_deleted', 'seerr_user_gone',
                                 'tvdb_id_missing', 'account_deleted')),
    attempts         INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at  INTEGER,
    withdraw_pending INTEGER NOT NULL DEFAULT 0 CHECK (withdraw_pending IN (0, 1)),
    checked_at       INTEGER,
    item_id          TEXT    REFERENCES items(id) ON DELETE SET NULL,
    created_at       INTEGER NOT NULL,
    updated_at       INTEGER NOT NULL,
    submitted_at     INTEGER,
    approved_at      INTEGER,
    available_at     INTEGER,
    closed_at        INTEGER,
    seerr_updated_at INTEGER,
    CHECK (media = 'movie' OR seerr_request_id IS NULL),
    CHECK (state <> 'partial' OR media = 'show'),
    CHECK ((state = 'failed') = (fail_reason IS NOT NULL))
) STRICT;

CREATE UNIQUE INDEX wishes_open_movie ON wishes (tmdb_id)
    WHERE media = 'movie' AND state IN ('queued', 'pending', 'processing');
CREATE UNIQUE INDEX wishes_request ON wishes (seerr_request_id) WHERE seerr_request_id IS NOT NULL;
CREATE INDEX wishes_withdraw ON wishes (next_attempt_at) WHERE withdraw_pending = 1;
CREATE INDEX wishes_open ON wishes (media, tmdb_id)
    WHERE state IN ('queued', 'pending', 'processing', 'partial');
CREATE INDEX wishes_open_tvdb ON wishes (tvdb_id)
    WHERE media = 'show' AND state IN ('queued', 'pending', 'processing', 'partial');
CREATE INDEX wishes_due ON wishes (next_attempt_at) WHERE state = 'queued';
CREATE INDEX wishes_admin ON wishes (state, updated_at DESC);
CREATE INDEX wishes_item ON wishes (item_id) WHERE item_id IS NOT NULL;

CREATE TABLE wish_seasons (
    wish_id          INTEGER NOT NULL REFERENCES wishes(id) ON DELETE CASCADE,
    season_number    INTEGER NOT NULL CHECK (season_number >= 1),
    tmdb_id          INTEGER NOT NULL,
    seerr_request_id INTEGER CHECK (seerr_request_id IS NULL OR seerr_request_id > 0),
    seerr_updated_at INTEGER,
    state            TEXT    NOT NULL CHECK (state IN ('queued', 'pending', 'processing', 'available',
                                                       'declined', 'failed', 'withdrawn')),
    available_at     INTEGER,
    notified_at      INTEGER,
    PRIMARY KEY (wish_id, season_number)
) STRICT, WITHOUT ROWID;

CREATE UNIQUE INDEX wish_seasons_open ON wish_seasons (tmdb_id, season_number)
    WHERE state IN ('queued', 'pending', 'processing');
CREATE INDEX wish_seasons_request ON wish_seasons (seerr_request_id);

-- The copy of the TMDB id must be the wish's, and only a series wish has seasons. SQLite cannot
-- set NEW in a BEFORE trigger, so the service writes the copy and the trigger refuses a wrong one.
-- +goose StatementBegin
CREATE TRIGGER wish_seasons_copy_insert BEFORE INSERT ON wish_seasons
WHEN NOT EXISTS (SELECT 1 FROM wishes WHERE id = NEW.wish_id AND media = 'show' AND tmdb_id = NEW.tmdb_id)
BEGIN
    SELECT RAISE(ABORT, 'wish_season_mismatch');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER wish_seasons_copy_update BEFORE UPDATE OF wish_id, tmdb_id ON wish_seasons
WHEN NOT EXISTS (SELECT 1 FROM wishes WHERE id = NEW.wish_id AND media = 'show' AND tmdb_id = NEW.tmdb_id)
BEGIN
    SELECT RAISE(ABORT, 'wish_season_mismatch');
END;
-- +goose StatementEnd

-- A wish with seasons keeps its title: the copies would drift otherwise.
-- +goose StatementBegin
CREATE TRIGGER wishes_title_fixed BEFORE UPDATE OF media, tmdb_id ON wishes
WHEN (NEW.media <> OLD.media OR NEW.tmdb_id <> OLD.tmdb_id)
 AND EXISTS (SELECT 1 FROM wish_seasons WHERE wish_id = OLD.id)
BEGIN
    SELECT RAISE(ABORT, 'wish_season_mismatch');
END;
-- +goose StatementEnd

CREATE TABLE wish_requesters (
    wish_id     INTEGER NOT NULL REFERENCES wishes(id) ON DELETE CASCADE,
    profile_id  INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    account_id  INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    role        TEXT    NOT NULL CHECK (role IN ('owner', 'joined')),
    created_at  INTEGER NOT NULL,
    notified_at INTEGER,
    closed_at   INTEGER,
    PRIMARY KEY (wish_id, profile_id)
) STRICT, WITHOUT ROWID;

CREATE UNIQUE INDEX wish_requesters_owner ON wish_requesters (wish_id) WHERE role = 'owner';
CREATE INDEX wish_requesters_profile ON wish_requesters (profile_id, created_at DESC);
CREATE INDEX wish_requesters_account ON wish_requesters (account_id);

CREATE TABLE wish_events (
    id      INTEGER PRIMARY KEY,
    wish_id INTEGER NOT NULL REFERENCES wishes(id) ON DELETE CASCADE,
    at      INTEGER NOT NULL,
    kind    TEXT    NOT NULL CHECK (kind <> '' AND kind NOT GLOB '*[^a-z_]*'),
    detail  TEXT    CHECK (detail IS NULL OR json_valid(detail))
) STRICT;

CREATE INDEX wish_events_wish ON wish_events (wish_id, at);

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
