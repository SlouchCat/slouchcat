-- Playback data of a media file of a visible title (package store/scopedq, read by the visibility
-- package only): the stored ffprobe result and keyframe index the playback decision and the HLS
-- plan need (M1b).

-- name: GetVisibleMediaFileProbe :one
SELECT m.id, m.probe, m.keyframes
FROM media_files AS m
JOIN visible_items AS v ON v.item_id = m.item_id AND v.profile_id = @profile_id
WHERE m.item_id = @item_id AND m.id = @media_file_id;

-- Tracks, segments and file details of M4 (W-4, W-9, I-5).

-- Audio and subtitle tracks of a media file of a visible title: embedded streams by index, then
-- sidecar files by path.
-- name: ListVisibleMediaFileTracks :many
SELECT t.* FROM tracks AS t
JOIN media_files AS m ON m.id = t.media_file_id
JOIN visible_items AS v ON v.item_id = m.item_id AND v.profile_id = @profile_id
WHERE m.item_id = @item_id AND m.id = @media_file_id AND t.kind IN ('audio', 'subtitle')
ORDER BY t.kind, t.source, t.stream_index, t.rel_path;

-- Audio and subtitle tracks of every media file of a visible title (detail page, versions).
-- name: ListVisibleItemTracks :many
SELECT t.* FROM tracks AS t
JOIN media_files AS m ON m.id = t.media_file_id
JOIN visible_items AS v ON v.item_id = m.item_id AND v.profile_id = @profile_id
WHERE m.item_id = @item_id AND t.kind IN ('audio', 'subtitle')
ORDER BY t.media_file_id, t.kind, t.source, t.stream_index, t.rel_path;

-- Detected intro and outro of a visible title (W-9).
-- name: ListVisibleSegments :many
SELECT s.kind, s.start_ms, s.end_ms, s.source FROM media_segments AS s
JOIN visible_items AS v ON v.item_id = s.item_id AND v.profile_id = @profile_id
WHERE s.item_id = @item_id
ORDER BY s.kind;

-- Stored probes of the media files of a visible title (bit rate, HDR format and audio channels of
-- the detail page).
-- name: ListVisibleMediaFileProbes :many
SELECT m.id, m.probe FROM media_files AS m
JOIN visible_items AS v ON v.item_id = m.item_id AND v.profile_id = @profile_id
WHERE m.item_id = @item_id;
