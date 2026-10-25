-- The folder list and the removal confirmation of a library name the files and the titles of one
-- folder (idea 02, E12, CountLibraryRootContent). Both numbers come from media_files by root_id,
-- and the list of libraries is polled every five seconds while a scan runs.
--
-- media_files_identity (root_id, dev, inode, size) answers the root_id part but does not carry
-- item_id, so count(DISTINCT item_id) had to read every matching row of the table and build a
-- temporary tree. With this index both numbers come out of the index alone.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

CREATE INDEX media_files_root_item ON media_files (root_id, item_id);

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
