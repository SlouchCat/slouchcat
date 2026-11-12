-- Deleting a library and titles (concept 20 sections 9 to 11, stage 5). Owner: library package.
-- Times are Unix milliseconds. A sqlc.slice always comes after every named parameter of its
-- query: sqlc numbers the named ones (?1, ?2) before it expands the slice, so a named parameter
-- behind a slice would read an element of the slice.

-- name: ListPlayingProfilesOfLibrary :many
-- The profiles that play a title of the library right now (O-13: a delete is refused while one
-- does), as the stream overview counts a stream: not revoked, not expired, a request within the
-- activity window, the browser session valid. By name, each once.
SELECT DISTINCT p.name FROM playback_sessions AS ps
JOIN sessions AS s ON s.id = ps.session_id
JOIN profiles AS p ON p.id = ps.profile_id
JOIN items AS i ON i.id = ps.item_id
WHERE i.library_id = @library_id
  AND ps.revoked_at IS NULL AND ps.expires_at > @now AND ps.last_activity_at >= @active_after
  AND s.revoked_at IS NULL AND s.expires_at > @now
ORDER BY p.name;

-- name: ListActiveJobsOfLibrary :many
-- Queued and running jobs that work on the library, its folders, its titles, their files or their
-- downloads, by the ids their payloads carry (library.*, metadata.*, image.fetch, probe.file,
-- keyframes, trickplay, chapters, podcasts, offline). Matching, trickplay, keyframes and images
-- hang on the title, not on the library; left alone they would fail after the delete and fill
-- the failed runs. The subqueries are not correlated, so each runs once.
SELECT j.id FROM jobs AS j
WHERE j.state IN ('queued', 'running')
  AND (json_extract(j.payload, '$.rootId') IN (SELECT r.id FROM library_roots AS r WHERE r.library_id = @library_id)
       OR json_extract(j.payload, '$.libraryId') = @library_id
       OR json_extract(j.payload, '$.itemId') IN (SELECT i.id FROM items AS i WHERE i.library_id = @library_id)
       OR json_extract(j.payload, '$.episodeId') IN (SELECT i.id FROM items AS i WHERE i.library_id = @library_id)
       OR json_extract(j.payload, '$.podcastId') IN (SELECT i.id FROM items AS i WHERE i.library_id = @library_id)
       OR json_extract(j.payload, '$.seriesId') IN (SELECT i.id FROM items AS i WHERE i.library_id = @library_id)
       OR json_extract(j.payload, '$.seasonId') IN (SELECT i.id FROM items AS i WHERE i.library_id = @library_id)
       OR json_extract(j.payload, '$.mediaFileId') IN (
            SELECT m.id FROM media_files AS m WHERE m.root_id IN (SELECT r.id FROM library_roots AS r WHERE r.library_id = @library_id))
       OR json_extract(j.payload, '$.downloadId') IN (
            SELECT d.id FROM downloads AS d WHERE d.item_id IN (SELECT i.id FROM items AS i WHERE i.library_id = @library_id)))
ORDER BY j.id;

-- name: ListLibraryItemIDs :many
-- Every title of the library, deleted or not: their image folders go after the delete.
SELECT id FROM items WHERE library_id = @library_id ORDER BY id;

-- name: CountLibraryItemsAll :one
-- Every title of the library, deleted or not, for the result and the log line of the delete.
SELECT count(*) FROM items WHERE library_id = @library_id;

-- name: ListTMDBCollectionsOfLibrary :many
-- The TMDB collections with a member in the library; those left without a member go with it.
SELECT DISTINCT c.id FROM collections AS c
JOIN collection_items AS ci ON ci.collection_id = c.id
JOIN items AS i ON i.id = ci.item_id
WHERE i.library_id = @library_id AND c.source = 'tmdb'
ORDER BY c.id;

-- name: DeleteGrantsOfLibrary :exec
-- The grants of the profiles, in the transaction of the delete (the foreign key of 00055 would
-- take them as well; this one does not depend on it).
DELETE FROM profile_libraries WHERE library_id = @library_id;

-- name: ClearActivityLibrary :exec
-- The activity log keeps its entries of the library; their subject holds its name.
UPDATE activity_events SET library_id = NULL WHERE library_id = @library_id;

-- name: DeleteLibraryRow :execrows
-- The foreign keys take the folders, titles, files and everything that hangs on them: progress,
-- watchlist, picks, own images and texts, the home row of the library, the grants, the downloads
-- (their tombstones tell the devices).
DELETE FROM libraries WHERE id = @id;

-- name: DeleteEmptyTMDBCollections :execrows
-- TMDB collections of the list that have no member any more; manual ones stay, empty or not.
DELETE FROM collections
WHERE id IN (sqlc.slice('ids')) AND source = 'tmdb'
  AND NOT EXISTS (SELECT 1 FROM collection_items AS ci WHERE ci.collection_id = collections.id);

-- Deleting a title (concept 20 sections 10 and 11).

-- name: GetDeletionItem :one
-- The title a deletion asks about, with its title in the given language (the text the dialog
-- shows and the administrator types; items.title without one) and the folder of its library.
SELECT i.id, i.kind, i.parent_id, i.library_id, i.deleted_at, i.removed_at, i.enclosure_url,
       CAST(COALESCE((SELECT t.title FROM item_texts AS t WHERE t.item_id = i.id AND t.lang = @lang), i.title) AS TEXT) AS display_title
FROM items AS i WHERE i.id = @id;

-- name: ListItemTree :many
-- A title and every title below it (seasons, episodes, podcast episodes, extras), deleted ones
-- too, the title first. Four levels are the deepest tree (series, season, episode; an extra of a
-- season).
WITH RECURSIVE tree(id, depth) AS (
    SELECT i.id, 0 FROM items AS i WHERE i.id = @id
    UNION ALL
    SELECT c.id, t.depth + 1 FROM items AS c JOIN tree AS t ON c.parent_id = t.id WHERE t.depth < 4
)
SELECT i.id, i.kind, i.parent_id, i.deleted_at, i.removed_at, i.enclosure_url FROM tree AS t
JOIN items AS i ON i.id = t.id
ORDER BY t.depth, i.id;

-- name: ListMediaFilesOfItems :many
-- The media files of the titles with the folder of their library root, by path.
SELECT m.id, m.item_id, m.root_id, r.path AS root_path, m.rel_path, m.size, m.dev, m.inode, m.missing_since
FROM media_files AS m
JOIN library_roots AS r ON r.id = m.root_id
WHERE m.item_id IN (sqlc.slice('item_ids'))
ORDER BY r.path, m.rel_path;

-- name: ListPlayingProfilesOfItems :many
-- The profiles that play one of the titles right now, as ListPlayingProfilesOfLibrary counts.
SELECT DISTINCT p.name FROM playback_sessions AS ps
JOIN sessions AS s ON s.id = ps.session_id
JOIN profiles AS p ON p.id = ps.profile_id
WHERE ps.revoked_at IS NULL AND ps.expires_at > @now AND ps.last_activity_at >= @active_after
  AND s.revoked_at IS NULL AND s.expires_at > @now
  AND ps.item_id IN (sqlc.slice('item_ids'))
ORDER BY p.name;

-- name: ListActiveJobsOfItems :many
-- Queued and running jobs of the titles, their files or their downloads (ListActiveJobsOfLibrary
-- for one tree).
WITH tree AS (SELECT i.id FROM items AS i WHERE i.id IN (sqlc.slice('item_ids')))
SELECT j.id FROM jobs AS j
WHERE j.state IN ('queued', 'running')
  AND (json_extract(j.payload, '$.itemId') IN (SELECT id FROM tree)
       OR json_extract(j.payload, '$.episodeId') IN (SELECT id FROM tree)
       OR json_extract(j.payload, '$.podcastId') IN (SELECT id FROM tree)
       OR json_extract(j.payload, '$.seriesId') IN (SELECT id FROM tree)
       OR json_extract(j.payload, '$.seasonId') IN (SELECT id FROM tree)
       OR json_extract(j.payload, '$.mediaFileId') IN (SELECT m.id FROM media_files AS m WHERE m.item_id IN (SELECT id FROM tree))
       OR json_extract(j.payload, '$.downloadId') IN (SELECT d.id FROM downloads AS d WHERE d.item_id IN (SELECT id FROM tree)))
ORDER BY j.id;

-- name: MarkItemsRemoved :execrows
-- "Nur aus Slouchcat entfernen": the titles leave every list (deleted_at) and are marked as taken
-- out (removed_at). A title taken out before keeps its own stamp, so it stays out when the title
-- above comes back; one soft-deleted before keeps its deletion time.
UPDATE items SET removed_at = COALESCE(removed_at, @now), deleted_at = COALESCE(deleted_at, @now), updated_at = @now
WHERE id IN (sqlc.slice('item_ids'));

-- name: DeleteDownloadsOfItems :execrows
-- The downloads of the titles; the tombstones of the delete trigger tell the devices to drop them.
DELETE FROM downloads WHERE item_id IN (sqlc.slice('item_ids'));

-- name: ListRemovedItems :many
-- The titles taken out of Slouchcat that can come back on their own: those whose parent is not
-- taken out too (a season of a removed series comes back with the series). Newest first, with
-- the library and the series or podcast above a season or episode.
SELECT i.id, i.kind, i.title, i.library_id, l.name AS library_name, i.removed_at,
       CAST(COALESCE(p.title, '') AS TEXT) AS parent_title, CAST(COALESCE(g.title, '') AS TEXT) AS grandparent_title
FROM items AS i INDEXED BY items_removed
JOIN libraries AS l ON l.id = i.library_id
LEFT JOIN items AS p ON p.id = i.parent_id
LEFT JOIN items AS g ON g.id = p.parent_id
WHERE i.removed_at IS NOT NULL AND (p.id IS NULL OR p.removed_at IS NULL)
ORDER BY i.removed_at DESC, i.id DESC
LIMIT @limit_count OFFSET @offset_count;

-- name: CountRemovedItems :one
SELECT count(*) FROM items AS i INDEXED BY items_removed
LEFT JOIN items AS p ON p.id = i.parent_id
WHERE i.removed_at IS NOT NULL AND (p.id IS NULL OR p.removed_at IS NULL);

-- name: RestoreRemovedItems :execrows
-- "Wieder aufnehmen": the titles of the tree taken out with the stamp of the title come back, the
-- same titles with all their user data. A title below taken out on its own before keeps its stamp
-- and stays out.
UPDATE items SET removed_at = NULL, deleted_at = NULL, updated_at = @now
WHERE removed_at = @removed_at AND id IN (sqlc.slice('item_ids'));

-- "Auch die Dateien loeschen" (concept 20 section 11, ADR 0032).

-- name: ListSharedMediaFiles :many
-- The media files of the titles whose file (device and inode, a hard link) is also the file of a
-- title outside them: such a file stays and the result names it.
WITH tree AS (SELECT i.id FROM items AS i WHERE i.id IN (sqlc.slice('item_ids')))
SELECT DISTINCT m.id FROM media_files AS m
JOIN media_files AS o ON o.dev = m.dev AND o.inode = m.inode AND o.id <> m.id
WHERE m.item_id IN (SELECT id FROM tree) AND m.dev <> 0 AND m.inode <> 0
  AND o.item_id NOT IN (SELECT id FROM tree);

-- name: ListLocalImagesOfItems :many
-- The image files of the folder that Slouchcat read and recorded for the titles.
SELECT DISTINCT root_id, rel_path FROM item_local_images WHERE item_id IN (sqlc.slice('item_ids'))
ORDER BY root_id, rel_path;

-- name: ListCoverPathsOfItems :many
-- The folder covers Slouchcat read for audiobooks and podcasts (relative to the root of their
-- files; an embedded picture is no file of its own).
SELECT id, CAST(cover_path AS TEXT) AS cover_path FROM items
WHERE cover_path IS NOT NULL AND cover_path <> '' AND id IN (sqlc.slice('item_ids'))
ORDER BY id;

-- name: CountTitleFilesAtPath :one
-- Whether the path is a media file of any title, or a recorded image of a title outside the
-- given ones: such a file is never deleted as a companion or a recorded file.
SELECT (SELECT count(*) FROM media_files AS m WHERE m.root_id = @root_id AND m.rel_path = @rel_path)
     + (SELECT count(*) FROM item_local_images AS l
        WHERE l.root_id = @root_id AND l.rel_path = @rel_path AND l.item_id NOT IN (sqlc.slice('item_ids'))) AS n;

-- name: CountMediaFilesBelow :one
-- Media files of the root at or below the folder prefix (ending with "/", or "" for the whole
-- root), other than the given ones: a fixed-name metadata file goes only with the last of them.
SELECT count(*) FROM media_files
WHERE root_id = @root_id AND substr(rel_path, 1, length(CAST(@prefix AS TEXT))) = CAST(@prefix AS TEXT)
  AND id NOT IN (sqlc.slice('file_ids'));

-- name: DeleteMediaFilesByID :execrows
-- The rows of the files that are gone: no player offers a deleted version.
DELETE FROM media_files WHERE id IN (sqlc.slice('ids'));

-- name: DeleteLocalImageRows :exec
DELETE FROM item_local_images WHERE root_id = @root_id AND rel_path = @rel_path;

-- name: CountMediaFilesOfItems :one
SELECT count(*) FROM media_files WHERE item_id IN (sqlc.slice('item_ids'));

-- name: ListItemsWithoutFiles :many
-- Of the given titles, the playable ones that no media file holds any more (their files went);
-- a podcast episode with an enclosure stays as a reference from its feed.
SELECT i.id FROM items AS i
WHERE i.id IN (sqlc.slice('item_ids'))
  AND (i.kind IN ('movie', 'episode', 'extra', 'audiobook') OR (i.kind = 'podcast_episode' AND i.enclosure_url IS NULL))
  AND NOT EXISTS (SELECT 1 FROM media_files AS m WHERE m.item_id = i.id);

-- name: ListEmptyContainers :many
-- Of the given titles, the seasons and series without a child left (a podcast stays as a
-- subscription).
SELECT i.id FROM items AS i
WHERE i.id IN (sqlc.slice('item_ids')) AND i.kind IN ('season', 'series')
  AND NOT EXISTS (SELECT 1 FROM items AS c WHERE c.parent_id = i.id);

-- name: HardDeleteItems :execrows
-- Deleted for good (O-14): the foreign keys take everything on the titles and below them,
-- progress, check marks and watchlist entries included.
DELETE FROM items WHERE id IN (sqlc.slice('ids'));

-- name: CountMediaFilesWithIdentity :one
-- Media files of any title with this device and inode: a companion candidate that is one (a hard
-- link, or a file moved under a fitting name) is never deleted as a companion.
SELECT count(*) FROM media_files WHERE dev = @dev AND inode = @inode AND dev <> 0 AND inode <> 0;
