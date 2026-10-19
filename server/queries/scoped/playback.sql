-- Playback data of a media file of a visible title (package store/scopedq, read by the visibility
-- package only): the stored ffprobe result and keyframe index the playback decision and the HLS
-- plan need (M1b).

-- name: GetVisibleMediaFileProbe :one
SELECT m.id, m.probe, m.keyframes
FROM media_files AS m
JOIN visible_items AS v ON v.item_id = m.item_id AND v.profile_id = @profile_id
WHERE m.item_id = @item_id AND m.id = @media_file_id;
