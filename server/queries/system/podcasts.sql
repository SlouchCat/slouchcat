-- Unfiltered podcast queries (package store/systemq, M6 design 3.4). Owner: metadata package
-- (subscriptions and the job podcast.fetch). Every query name starts with Podcast so the files of the
-- other owners never collide. Times are Unix milliseconds.

-- name: PodcastFindByFeed :one
-- A live subscription of the library to the feed URL (feed_exists; the unique index
-- items_podcast_feed refuses a second one).
SELECT id FROM items
WHERE kind = 'podcast' AND library_id = @library_id AND feed_url = @feed_url AND deleted_at IS NULL
LIMIT 1;

-- name: PodcastInsert :one
-- A new subscription after its first successful fetch; the episodes follow in the same transaction.
-- feed_origin names who chose the feed: 'admin' for a subscription made by hand, 'file' for a feed
-- read from the metadata.json of a podcast folder (ADR 0015).
INSERT INTO items (id, kind, library_id, title, sort_title, author, explicit, cover_url, feed_url, feed_etag,
                   feed_modified, feed_checked_at, feed_origin, own_rating, rating_source, providers, metadata_state,
                   matched_at, added_at, created_at, updated_at)
VALUES (@id, 'podcast', @library_id, @title, @sort_title, @author, @explicit, @cover_url, @feed_url, @feed_etag,
        @feed_modified, @now, @feed_origin, @own_rating, @rating_source, '["local"]', 'matched', @now, @now, @now, @now)
RETURNING *;

-- name: PodcastUpdateChannel :exec
-- The channel after a fetch with content (200): the caller keeps locked fields and manual ratings.
UPDATE items
SET title = @title, sort_title = @sort_title, author = @author, explicit = @explicit, cover_url = @cover_url,
    own_rating = @own_rating, rating_source = @rating_source, feed_etag = @feed_etag, feed_modified = @feed_modified,
    feed_checked_at = @now, feed_error = NULL, updated_at = @now
WHERE id = @id AND kind = 'podcast';

-- name: PodcastSetChecked :exec
-- A fetch without new content (304) or a failed one (feed_error set, NULL after a success).
UPDATE items SET feed_checked_at = @now, feed_error = @feed_error, updated_at = @now
WHERE id = @id AND kind = 'podcast';

-- name: PodcastGet :one
-- A podcast with its library (live or deleted; the caller decides).
SELECT i.*, l.default_rating AS library_default_rating
FROM items AS i JOIN libraries AS l ON l.id = i.library_id
WHERE i.id = @id AND i.kind = 'podcast';

-- name: PodcastList :many
-- Every live subscription with the number of its live episodes, newest first (listPodcasts).
SELECT i.id, i.library_id, i.title, i.author, i.feed_url, i.feed_origin, i.explicit, i.feed_checked_at, i.feed_error, i.created_at,
       (SELECT count(*) FROM items AS e
        WHERE e.parent_id = i.id AND e.kind = 'podcast_episode' AND e.deleted_at IS NULL) AS episode_count,
       (SELECT count(*) FROM items AS e JOIN media_files AS m ON m.item_id = e.id
        WHERE e.parent_id = i.id AND e.kind = 'podcast_episode' AND e.deleted_at IS NULL) AS local_episode_count,
       CAST(COALESCE((SELECT m.root_id FROM items AS e JOIN media_files AS m ON m.item_id = e.id
                      WHERE e.parent_id = i.id AND e.kind = 'podcast_episode' ORDER BY m.id LIMIT 1), 0) AS INTEGER) AS root_id
FROM items AS i
WHERE i.kind = 'podcast' AND i.deleted_at IS NULL AND (sqlc.narg('only_id') IS NULL OR i.id = sqlc.narg('only_id'))
ORDER BY i.created_at DESC, i.id DESC;

-- name: PodcastListIDsByLibrary :many
-- Live subscriptions of a library (scanLibrary of a podcast library refreshes all).
SELECT id FROM items WHERE kind = 'podcast' AND library_id = @library_id AND deleted_at IS NULL ORDER BY id;

-- name: PodcastListDue :many
-- Live subscriptions whose last fetch is older than the moment (the poller, H-2); never fetched first.
SELECT id FROM items
WHERE kind = 'podcast' AND deleted_at IS NULL AND (feed_checked_at IS NULL OR feed_checked_at <= @checked_before)
ORDER BY feed_checked_at IS NOT NULL, feed_checked_at, id
LIMIT 500;

-- name: PodcastListEpisodes :many
-- The stored episodes of a podcast with what an update compares and keeps (locked fields),
-- soft-deleted ones included: their guid stays taken, so the fetch neither inserts nor restores them.
SELECT id, guid, title, sort_title, cover_url, own_rating, rating_source, locked_fields, deleted_at, published_at FROM items
WHERE parent_id = @podcast_id AND kind = 'podcast_episode' AND guid IS NOT NULL;

-- name: PodcastInsertEpisode :exec
INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, guid, published_at, duration_ms,
                   enclosure_url, enclosure_type, enclosure_length, explicit, cover_url, season_number, episode_number,
                   own_rating, rating_source, providers, metadata_state, matched_at, added_at, created_at, updated_at)
VALUES (@id, 'podcast_episode', @parent_id, @library_id, @title, @sort_title, @guid, @published_at, @duration_ms,
        @enclosure_url, @enclosure_type, @enclosure_length, @explicit, @cover_url, @season_number, @episode_number,
        @own_rating, @rating_source, '["local"]', 'matched', @now, @now, @now, @now);

-- name: PodcastUpdateEpisode :exec
-- An episode the feed still lists (a soft-deleted one stays deleted).
UPDATE items
SET title = @title, sort_title = @sort_title, published_at = @published_at, duration_ms = @duration_ms,
    enclosure_url = @enclosure_url, enclosure_type = @enclosure_type, enclosure_length = @enclosure_length,
    explicit = @explicit, cover_url = @cover_url, season_number = @season_number, episode_number = @episode_number,
    own_rating = @own_rating, rating_source = @rating_source, updated_at = @now
WHERE id = @id AND kind = 'podcast_episode' AND deleted_at IS NULL;

-- name: PodcastSoftDelete :execrows
-- Unsubscribe: the podcast and its episodes (deletePodcast).
UPDATE items SET deleted_at = @now, updated_at = @now
WHERE deleted_at IS NULL AND ((id = @id AND kind = 'podcast') OR (parent_id = @id AND kind = 'podcast_episode'));
