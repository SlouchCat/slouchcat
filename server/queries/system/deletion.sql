-- Deleting a library and titles (concept 20 sections 9 to 11, stage 5). Owner: library package.
-- Times are Unix milliseconds.

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
