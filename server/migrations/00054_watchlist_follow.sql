-- The watchlist takes over from the favourite (concept 20 P-2, contract 1.31.0; breaking).
--
-- The favourite of series and podcasts (E-8, 00015) goes away: a series or podcast on the watchlist
-- of a profile now triggers the notification «neue Folge» and the offline rule «neue
-- Podcast-Folgen automatisch laden». Every favourite becomes a watchlist entry (no duplicates), so
-- nobody loses a notification, then the table favorites is dropped.
--
-- New column watchlist.follow_since (Unix milliseconds): the offline rule counts an episode as new
-- from this moment. Taking created_at instead would make every episode of a year new for a podcast
-- that has stood on the watchlist for a year, and each device with the rule would fetch old
-- episodes on its next sync. Therefore:
--   * a migrated favourite: the date of the favourite, also when the title was on the watchlist
--     already (its created_at stays, it orders the list),
--   * an existing entry without favourite: the moment of this migration,
--   * a new entry: created_at (AddWatchlist).
-- The table is rebuilt because SQLite adds a NOT NULL column only with a default.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE TABLE watchlist_new (
    profile_id   INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    item_id      TEXT    NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    created_at   INTEGER NOT NULL,
    follow_since INTEGER NOT NULL,
    PRIMARY KEY (profile_id, item_id)
) STRICT, WITHOUT ROWID;

INSERT INTO watchlist_new (profile_id, item_id, created_at, follow_since)
SELECT w.profile_id, w.item_id, w.created_at,
       COALESCE(f.created_at, CAST(unixepoch('subsec') * 1000 AS INTEGER))
FROM watchlist AS w
LEFT JOIN favorites AS f ON f.profile_id = w.profile_id AND f.item_id = w.item_id;

INSERT INTO watchlist_new (profile_id, item_id, created_at, follow_since)
SELECT f.profile_id, f.item_id, f.created_at, f.created_at
FROM favorites AS f
WHERE NOT EXISTS (SELECT 1 FROM watchlist AS w WHERE w.profile_id = f.profile_id AND w.item_id = f.item_id);

DROP TABLE watchlist;
ALTER TABLE watchlist_new RENAME TO watchlist;
CREATE INDEX watchlist_item ON watchlist (item_id);

DROP TABLE favorites;

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
