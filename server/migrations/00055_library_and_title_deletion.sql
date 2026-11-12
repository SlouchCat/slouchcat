-- Deleting a library and taking a title out of Slouchcat (concept 20 sections 9 and 10, stage 5,
-- contract 1.31.0, 12 November 2026).
--
-- profile_libraries had no foreign key to libraries (00002), and libraries.id is given without
-- AUTOINCREMENT, so SQLite hands the highest id out again. A grant left behind by a deleted
-- library would then grant the next new library to a restricted profile, without anybody having
-- granted it (visible_items does not ask whether the library exists). The table is rebuilt with
-- ON DELETE CASCADE to libraries; grants of a library or profile that no longer exists are dropped
-- on the way. profile_libraries_library serves the cascade, which looks the grants up by library.
--
-- items.removed_at (Unix milliseconds): the administrator took the title out of Slouchcat
-- («Nur aus Slouchcat entfernen», section 10.2). It always comes with deleted_at, so every list
-- that asks for deleted_at IS NULL leaves the title out; the scanner never brings a removed title
-- back and puts new files under it into it, removed as well. «Wieder aufnehmen» clears both.
-- items_removed serves the page «Entfernte Titel», newest first.
--
-- Indexes on the child columns of the foreign keys to items, media_files and library_roots that
-- had none (measurement M-1, docs/perf/20-bedienung.md): deleting a row looks its children up by
-- that column, and without an index every deleted title walked the whole table once. The delete
-- of a library with 10 000 films took 42 s that way, all of it in progress_events, playback_sessions
-- and progress; a title deleted for good (O-14) walks them as well.
--
-- visible_items reads profile_libraries and is dropped and created again around the rebuild, word
-- for word as in 00022.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

DROP VIEW visible_items;

CREATE TABLE profile_libraries_new (
    profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    library_id INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    PRIMARY KEY (profile_id, library_id)
) STRICT, WITHOUT ROWID;

INSERT INTO profile_libraries_new (profile_id, library_id)
SELECT pl.profile_id, pl.library_id FROM profile_libraries AS pl
WHERE EXISTS (SELECT 1 FROM libraries AS l WHERE l.id = pl.library_id)
  AND EXISTS (SELECT 1 FROM profiles AS p WHERE p.id = pl.profile_id);

DROP TABLE profile_libraries;
ALTER TABLE profile_libraries_new RENAME TO profile_libraries;
CREATE INDEX profile_libraries_library ON profile_libraries (library_id);

ALTER TABLE items ADD COLUMN removed_at INTEGER
    CHECK (removed_at IS NULL OR (removed_at >= 0 AND deleted_at IS NOT NULL));
CREATE INDEX items_removed ON items (removed_at DESC) WHERE removed_at IS NOT NULL;

CREATE INDEX progress_item ON progress (item_id);
CREATE INDEX progress_events_item ON progress_events (item_id);
CREATE INDEX progress_days_item ON progress_days (item_id);
CREATE INDEX playback_sessions_item ON playback_sessions (item_id);
CREATE INDEX downloads_media_file ON downloads (media_file_id);
CREATE INDEX notifications_parent ON notifications (parent_id);
CREATE INDEX item_local_images_root ON item_local_images (root_id);

-- Visibility (plan 3.3): the profile's own ceiling and library rights, capped by the account's; an
-- extra only below a visible parent. Word for word from 00022.
CREATE VIEW visible_items AS
SELECT p.id AS profile_id, i.id AS item_id
FROM profiles AS p
JOIN accounts AS a ON a.id = p.account_id
JOIN items AS i
  ON i.deleted_at IS NULL
 AND i.available = 1
 AND COALESCE(i.effective_rating, 18) <= min(p.max_rating, a.max_rating)
 AND (i.kind NOT IN ('series', 'season', 'podcast') OR COALESCE(i.min_child_rating, 18) <= min(p.max_rating, a.max_rating))
 AND (i.kind <> 'extra' OR EXISTS (
        SELECT 1 FROM items AS pa
        WHERE pa.id = i.parent_id AND pa.deleted_at IS NULL AND pa.available = 1
          AND COALESCE(pa.effective_rating, 18) <= min(p.max_rating, a.max_rating)
          AND (pa.kind NOT IN ('series', 'season', 'podcast') OR COALESCE(pa.min_child_rating, 18) <= min(p.max_rating, a.max_rating))))
WHERE (p.all_libraries = 1
       OR EXISTS (SELECT 1 FROM profile_libraries AS pl
                  WHERE pl.profile_id = p.id AND pl.library_id = i.library_id))
  AND (a.all_libraries = 1
       OR EXISTS (SELECT 1 FROM account_libraries AS al
                  WHERE al.account_id = a.id AND al.library_id = i.library_id));

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
