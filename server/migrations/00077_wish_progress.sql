-- Concept 25 stage 4, step 4.6d (08-datenmodell-api.md section 2.16): the download progress from
-- the queue of Radarr and Sonarr. Only package wishes reads and writes it (store/wishq); only the
-- job wishes.progress writes it, the pages read it and never call Radarr or Sonarr (N-84). Times
-- are Unix milliseconds.
--
--   wish_progress  one row per wish in procurement (processing, partial). arr_title_id is the id of
--                  the film resp. series in Radarr resp. Sonarr, looked up once. stage is what the
--                  viewer sees (search, download, import, stalled; NULL = nothing to show); percent
--                  and eta_seconds only with download. episodes_done and episodes_total only for
--                  Sonarr, over the wished seasons. admin_message is the service's message for the
--                  administration, at most 500 characters and without paths. error says why
--                  nothing or only a part was readable. has_file and title_read_at keep the file
--                  state of the last lookup of the title (at most every 5 minutes, architecture
--                  22.4). read_at is the last run that read the queue for the row; older than 5
--                  minutes is stale (N-85).
--
-- A trigger deletes the row when the wish leaves procurement (architecture 22.4).
--
-- A new table only, nothing is rebuilt; the frame of the template stays.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE wish_progress (
    wish_id        INTEGER NOT NULL PRIMARY KEY REFERENCES wishes(id) ON DELETE CASCADE,
    service        TEXT    NOT NULL CHECK (service IN ('radarr', 'sonarr')),
    arr_title_id   INTEGER CHECK (arr_title_id IS NULL OR arr_title_id > 0),
    stage          TEXT    CHECK (stage IS NULL OR stage IN ('search', 'download', 'import', 'stalled')),
    percent        INTEGER CHECK (percent IS NULL OR percent BETWEEN 0 AND 100),
    eta_seconds    INTEGER CHECK (eta_seconds IS NULL OR eta_seconds >= 0),
    episodes_done  INTEGER CHECK (episodes_done IS NULL OR episodes_done >= 0),
    episodes_total INTEGER CHECK (episodes_total IS NULL OR episodes_total >= 0),
    admin_message  TEXT    CHECK (admin_message IS NULL OR (admin_message <> '' AND length(admin_message) <= 500)),
    error          TEXT    CHECK (error IS NULL OR error IN ('unreachable', 'not_found', 'incomplete')),
    has_file       INTEGER CHECK (has_file IS NULL OR has_file IN (0, 1)),
    title_read_at  INTEGER,
    read_at        INTEGER NOT NULL,
    CHECK (COALESCE(stage, '') = 'download' OR (percent IS NULL AND eta_seconds IS NULL)),
    CHECK ((episodes_done IS NULL) = (episodes_total IS NULL)),
    CHECK (service = 'sonarr' OR episodes_total IS NULL)
) STRICT;

CREATE INDEX wish_progress_service ON wish_progress (service);

-- A closed wish keeps no progress.
-- +goose StatementBegin
CREATE TRIGGER wish_progress_closed AFTER UPDATE OF state ON wishes
WHEN NEW.state NOT IN ('processing', 'partial')
BEGIN
    DELETE FROM wish_progress WHERE wish_id = NEW.id;
END;
-- +goose StatementEnd

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
