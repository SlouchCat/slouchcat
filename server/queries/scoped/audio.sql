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
