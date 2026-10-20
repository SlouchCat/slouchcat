-- Unfiltered queries of the offline package (package store/systemq, M5 design 3.3 to 3.5): downloads,
-- their tombstones (the revocation list), the auto-download rules and the source data of the
-- preparation job. Owner: offline package. Visibility for viewers always goes through package
-- visibility; these queries read and write the download rows themselves. Times are Unix milliseconds.

-- name: InsertDownload :one
-- One download per device and title: a second request for the same title on the device inserts
-- nothing (no row returned) and the caller reads the existing row.
INSERT INTO downloads (id, account_id, profile_id, device_id, item_id, media_file_id, quality, audio_stream, auto,
                       duration_ms, estimated_bytes, allowed_profile_ids, created_at, updated_at, allowed_at)
VALUES (@id, @account_id, @profile_id, @device_id, @item_id, @media_file_id, @quality, @audio_stream, @auto,
        @duration_ms, @estimated_bytes, @allowed_profile_ids, @now, @now, @now)
ON CONFLICT (device_id, item_id) DO NOTHING
RETURNING *;

-- name: GetDownload :one
SELECT * FROM downloads WHERE id = @id;

-- name: GetDeviceDownload :one
-- The download of a title on a device (UNIQUE device_id, item_id).
SELECT * FROM downloads WHERE device_id = @device_id AND item_id = @item_id;

-- name: ListDeviceDownloads :many
-- Every download of an account on a device, oldest first (sync).
SELECT * FROM downloads WHERE device_id = @device_id AND account_id = @account_id
ORDER BY created_at, id;

-- name: DeleteAccountDownload :execrows
-- Deletes a download of the account; the trigger downloads_tombstone writes its tombstone.
DELETE FROM downloads WHERE id = @id AND account_id = @account_id;

-- name: SetDownloadsFetched :execrows
-- The device reports complete files (sync): the server copy is removed 24 h later (AN-20).
UPDATE downloads SET updated_at = @now, fetched_at = @now
WHERE device_id = @device_id AND account_id = @account_id AND fetched_at IS NULL
  AND state IN ('ready', 'evicted') AND id IN (sqlc.slice('ids'));

-- name: RequeueDownload :execrows
-- An evicted download the device still lacks is prepared again from the start.
UPDATE downloads
SET state = 'queued', error = NULL, segments_done = 0, size_bytes = NULL, ready_at = NULL, fetched_at = NULL,
    files_removed_at = NULL, updated_at = @now
WHERE id = @id AND state = 'evicted';

-- name: SetDownloadAllowed :exec
UPDATE downloads SET allowed_profile_ids = @allowed_profile_ids, allowed_at = @now WHERE id = @id;

-- name: SetDownloadState :execrows
-- queued (waiting, error budget_full or a retry), paused (preempted by live playback) or failed.
UPDATE downloads SET state = @state, error = @error, updated_at = @now WHERE id = @id;

-- name: StartDownloadPreparation :execrows
UPDATE downloads
SET state = 'preparing', error = NULL, mode = @mode, width = @width, height = @height, video_bitrate = @video_bitrate,
    audio_bitrate = @audio_bitrate, duration_ms = @duration_ms, segments_total = @segments_total,
    segments_done = @segments_done, updated_at = @now
WHERE id = @id;

-- name: SetDownloadProgress :execrows
-- After every appended segment; no row means the download was deleted meanwhile.
UPDATE downloads SET segments_done = @segments_done, updated_at = @now WHERE id = @id;

-- name: SetDownloadReady :execrows
UPDATE downloads
SET state = 'ready', error = NULL, size_bytes = @size_bytes, segments_done = @segments_done, updated_at = @now,
    ready_at = @now, files_removed_at = NULL
WHERE id = @id;

-- name: SumServerCopies :one
-- Space the server copies take or will take (budget, AN-20): prepared files by their size, started
-- preparations by their estimate. Queued downloads that have not started do not count.
SELECT CAST(COALESCE(SUM(CASE WHEN state = 'ready' THEN COALESCE(size_bytes, estimated_bytes)
                              ELSE estimated_bytes END), 0) AS INTEGER) AS bytes
FROM downloads
WHERE id <> @id AND files_removed_at IS NULL
  AND (state IN ('ready', 'preparing', 'paused') OR (state = 'queued' AND segments_done > 0));

-- name: ListDownloadsToEvict :many
-- Prepared copies the server removes: 24 h after the device fetched them, 7 days after they were
-- ready without a fetch (AN-20). At most 500 per batch.
SELECT id FROM downloads
WHERE state = 'ready' AND files_removed_at IS NULL
  AND ((fetched_at IS NOT NULL AND fetched_at <= @fetched_before)
       OR (fetched_at IS NULL AND ready_at <= @ready_before))
ORDER BY id
LIMIT 500;

-- name: EvictDownload :execrows
UPDATE downloads SET state = 'evicted', updated_at = @now, files_removed_at = @now
WHERE id = @id AND state = 'ready';

-- name: ListFailedDownloadFiles :many
-- Failed preparations whose partial files are still on disk. At most 500 per batch.
SELECT id FROM downloads WHERE state = 'failed' AND files_removed_at IS NULL ORDER BY id LIMIT 500;

-- name: SetDownloadFilesRemoved :exec
UPDATE downloads SET files_removed_at = @now WHERE id = @id;

-- name: ListExistingDownloadIDs :many
-- Which of the ids have a row (orphaned directories of the cleanup).
SELECT id FROM downloads WHERE id IN (sqlc.slice('ids'));

-- name: ListDeviceTombstones :many
-- Reported ids of a device that were deleted (revocation list, AN-4, K-7).
SELECT id FROM download_tombstones WHERE device_id = @device_id AND id IN (sqlc.slice('ids')) ORDER BY id;

-- name: DeleteOldTombstones :execrows
-- Tombstones older than 400 days go, like devices (at most 500 per batch).
DELETE FROM download_tombstones
WHERE id IN (SELECT t.id FROM download_tombstones AS t WHERE t.removed_at < @before LIMIT 500);

-- name: CountOpenAutoDownloads :one
-- Auto-downloads of a profile on a device that the profile has not finished watching (O-3).
SELECT count(*) FROM downloads AS d
LEFT JOIN progress AS p ON p.profile_id = d.profile_id AND p.item_id = d.item_id
WHERE d.profile_id = @profile_id AND d.device_id = @device_id AND d.auto = 1 AND COALESCE(p.completed, 0) = 0;

-- name: GetDownloadRule :one
SELECT * FROM download_rules WHERE profile_id = @profile_id AND device_id = @device_id;

-- name: UpsertDownloadRule :exec
INSERT INTO download_rules (profile_id, device_id, enabled, next_episodes, new_podcast_episodes, wifi_only, max_count,
                            quality, updated_at)
VALUES (@profile_id, @device_id, @enabled, @next_episodes, @new_podcast_episodes, @wifi_only, @max_count, @quality, @now)
ON CONFLICT (profile_id, device_id) DO UPDATE SET
    enabled = excluded.enabled, next_episodes = excluded.next_episodes,
    new_podcast_episodes = excluded.new_podcast_episodes, wifi_only = excluded.wifi_only,
    max_count = excluded.max_count, quality = excluded.quality, updated_at = excluded.updated_at;

-- name: ListActiveDeviceRules :many
-- Enabled next-episode rules of the account's profiles on a device (sync, O-3).
SELECT r.* FROM download_rules AS r
JOIN profiles AS p ON p.id = r.profile_id
WHERE r.device_id = @device_id AND p.account_id = @account_id AND r.enabled = 1 AND r.next_episodes = 1
ORDER BY r.profile_id;

-- name: ListRecentEpisodeProgress :many
-- Episodes the profile watched since a moment, newest first, with their series: the last watched
-- episode of every series is the first row of the series (auto-downloads, O-3). Visibility is
-- applied by the caller through the profile scope of package visibility.
SELECT p.item_id, p.updated_at, s.parent_id AS series_id
FROM progress AS p
JOIN items AS e ON e.id = p.item_id AND e.kind = 'episode'
JOIN items AS s ON s.id = e.parent_id AND s.kind = 'season'
WHERE p.profile_id = @profile_id AND p.updated_at >= @since AND s.parent_id IS NOT NULL
ORDER BY p.updated_at DESC, p.item_id;

-- name: GetDownloadSource :one
-- The source of a preparation: library root, path, stored probe and keyframe index of the file.
SELECT m.id, m.item_id, m.rel_path, m.size, m.video_codec, m.audio_codec, m.width, m.height, m.duration_ms,
       m.probe, m.keyframes, r.path AS root_path
FROM media_files AS m
JOIN library_roots AS r ON r.id = m.root_id
WHERE m.id = @id;

-- name: ListDownloadSubtitleTracks :many
-- Subtitle tracks of a media file; the job converts the text tracks to WebVTT sidecars.
SELECT * FROM tracks WHERE media_file_id = @media_file_id AND kind = 'subtitle'
ORDER BY source, stream_index, rel_path;

-- name: GetProfileAccount :one
SELECT account_id FROM profiles WHERE id = @id;
