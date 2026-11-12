-- Audiobooks of a viewing profile (M6, design 3.1; package store/scopedq, read by the visibility
-- package only): the book-wide chapters and the parts in playback order.

-- Book-wide chapters of a visible audiobook, in order.
-- name: ListVisibleChapters :many
SELECT c.idx, c.title, c.start_ms, c.end_ms, c.media_file_id, c.file_start_ms
FROM chapters AS c
JOIN visible_items AS v ON v.item_id = c.item_id AND v.profile_id = @profile_id
WHERE c.item_id = @item_id
ORDER BY c.idx;

-- Present parts of a visible audiobook with the path of their library root, in playback order
-- (part_index; a single-file book has none). A part whose probe failed is left out, like in the
-- chapters and the downloads.
-- name: ListVisibleAudiobookParts :many
SELECT m.id, m.item_id, m.rel_path, m.size, m.mtime, m.container, m.video_codec, m.audio_codec,
       m.width, m.height, m.duration_ms, m.version_label, m.missing_since, m.created_at, m.part_index,
       r.path AS root_path
FROM media_files AS m
JOIN visible_items AS v ON v.item_id = m.item_id AND v.profile_id = @profile_id
JOIN library_roots AS r ON r.id = m.root_id
WHERE m.item_id = @item_id AND m.missing_since IS NULL AND m.probe_error IS NULL
ORDER BY COALESCE(m.part_index, 0), m.rel_path, m.id;

-- Audiobook series (concept 19 stage D2, D4.1): the visible books of a library that belong to a
-- series, with what the row "Reihen und Sammlungen" and the page of a series compute from them
-- (count, spellings, author, narrator, duration, order, completed). sort_key is the title sort key
-- in the metadata language, then the interface language, then the file name's, the visibility
-- package groups and orders in Go. completed leads to the next volume (it continues), watched
-- counts the volumes heard (it shows, watched_at, concept 20 section 6.3).
-- name: ListVisibleAudioSeriesVolumes :many
SELECT i.id, CAST(i.audio_series_id AS TEXT) AS series_id, s.name AS series_display, s.name_locked,
       i.series_name, i.series_index, COALESCE(i.author_display, i.author) AS author, i.narrator, i.duration_ms,
       CAST(COALESCE(NULLIF(tm.sort_key, ''), NULLIF(tu.sort_key, ''), i.sort_title, '') AS TEXT) AS sort_key,
       COALESCE(p.completed, 0) AS completed, CAST(p.watched_at IS NOT NULL AS INTEGER) AS watched
FROM items AS i
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
JOIN audio_series AS s ON s.id = i.audio_series_id AND s.library_id = i.library_id
LEFT JOIN item_texts AS tm ON tm.item_id = i.id AND tm.lang = @meta_lang
LEFT JOIN item_texts AS tu ON tu.item_id = i.id AND tu.lang = @ui_lang
LEFT JOIN progress AS p ON p.profile_id = @profile_id AND p.item_id = i.id
WHERE i.library_id = @library_id AND i.kind = 'audiobook';

-- The same rows for one series of a library (the page of a series).
-- name: ListVisibleAudioSeriesVolumesOf :many
SELECT i.id, CAST(i.audio_series_id AS TEXT) AS series_id, s.name AS series_display, s.name_locked,
       i.series_name, i.series_index, COALESCE(i.author_display, i.author) AS author, i.narrator, i.duration_ms,
       CAST(COALESCE(NULLIF(tm.sort_key, ''), NULLIF(tu.sort_key, ''), i.sort_title, '') AS TEXT) AS sort_key,
       COALESCE(p.completed, 0) AS completed, CAST(p.watched_at IS NOT NULL AS INTEGER) AS watched
FROM items AS i
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
JOIN audio_series AS s ON s.id = i.audio_series_id AND s.library_id = i.library_id
LEFT JOIN item_texts AS tm ON tm.item_id = i.id AND tm.lang = @meta_lang
LEFT JOIN item_texts AS tu ON tu.item_id = i.id AND tu.lang = @ui_lang
LEFT JOIN progress AS p ON p.profile_id = @profile_id AND p.item_id = i.id
WHERE i.audio_series_id = @series_id AND i.library_id = @library_id AND i.kind = 'audiobook';

-- The number of visible books of a series (ItemDetail.seriesId needs two), counted like the page.
-- name: CountVisibleAudioSeriesVolumes :one
SELECT count(*)
FROM items AS i
JOIN visible_items AS v ON v.item_id = i.id AND v.profile_id = @profile_id
JOIN audio_series AS s ON s.id = i.audio_series_id AND s.library_id = i.library_id
WHERE i.audio_series_id = @series_id AND i.kind = 'audiobook';

-- The audiobook collections of a library (D4.1): manual collections whose visible members are all
-- audiobooks, with at least one visible member in this library. Count, heard members (watched_at,
-- concept 20 section 6.3) and cover over every visible member, as the collection page and
-- ListVisibleCollections count them.
-- name: ListVisibleAudioCollections :many
SELECT c.id, c.name, c.name_de, count(*) AS member_count,
       CAST(sum(p.watched_at IS NOT NULL) AS INTEGER) AS completed,
       CAST((SELECT ci2.item_id FROM collection_items AS ci2
             JOIN items AS i2 ON i2.id = ci2.item_id
             JOIN visible_items AS v2 ON v2.item_id = ci2.item_id AND v2.profile_id = @profile_id
             WHERE ci2.collection_id = c.id
             ORDER BY ci2.position, i2.release_date IS NULL, i2.release_date, i2.id
             LIMIT 1) AS TEXT) AS cover_id
FROM collections AS c
JOIN collection_items AS ci ON ci.collection_id = c.id
JOIN visible_items AS v ON v.item_id = ci.item_id AND v.profile_id = @profile_id
JOIN items AS i ON i.id = ci.item_id
LEFT JOIN progress AS p ON p.profile_id = @profile_id AND p.item_id = ci.item_id
WHERE c.source = 'manual'
GROUP BY c.id
HAVING sum(i.kind <> 'audiobook') = 0 AND sum(i.library_id = @library_id) > 0
ORDER BY c.name, c.id;

-- The visible members of the given collections with their library, in every library (the filter
-- count "2 von 6" counts the members like the badge).
-- name: ListVisibleCollectionMembers :many
SELECT ci.collection_id, ci.item_id, i.library_id
FROM collection_items AS ci
JOIN visible_items AS v ON v.item_id = ci.item_id AND v.profile_id = @profile_id
JOIN items AS i ON i.id = ci.item_id
WHERE ci.collection_id IN (sqlc.slice('collection_ids'));
