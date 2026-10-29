-- A podcast episode is available with an enclosure or with a present file (ADR 0015, concept 4.5).
--
-- The trigger items_enclosure_update of 00003 set the availability of an episode from its
-- enclosure alone: an episode read from a podcast folder, which has a file and no enclosure,
-- became unavailable the moment its enclosure was cleared, although its file is there. The
-- media_files triggers of 00014 already know both sources; this trigger now follows the same
-- rule. Triggers are replaced, never edited in place (CLAUDE.md rule 5).

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

DROP TRIGGER items_enclosure_update;

-- +goose StatementBegin
CREATE TRIGGER items_enclosure_update AFTER UPDATE OF enclosure_url ON items
WHEN NEW.kind = 'podcast_episode'
BEGIN
    UPDATE items SET available = (NEW.enclosure_url IS NOT NULL
        OR EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = NEW.id AND m.missing_since IS NULL))
    WHERE id = NEW.id;
END;
-- +goose StatementEnd

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
