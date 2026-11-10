-- Unfiltered title queries for metadata enrichment and image storage (package store/systemq).
-- Owner: metadata package. Times are Unix milliseconds.

-- name: UpdateItemMatch :exec
-- Writes the matched provider data of a movie or series. The caller has already kept the values
-- of locked fields.
UPDATE items
SET original_title     = @original_title,
    year               = @year,
    release_date       = @release_date,
    runtime_minutes    = @runtime_minutes,
    genres             = @genres,
    tmdb_id            = @tmdb_id,
    tvdb_id            = @tvdb_id,
    imdb_id            = @imdb_id,
    own_rating         = @own_rating,
    rating_source      = @rating_source,
    tmdb_poster_path   = @tmdb_poster_path,
    tmdb_backdrop_path = @tmdb_backdrop_path,
    vote_average       = @vote_average,
    original_language  = @original_language,
    metadata_state     = 'matched',
    metadata_error     = NULL,
    matched_at         = @now,
    updated_at         = @now
WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateChildMatch :exec
-- Writes the provider data of a season or episode (series job or episode match). The caller has
-- already kept the values of locked fields.
UPDATE items
SET release_date       = @release_date,
    runtime_minutes    = @runtime_minutes,
    tmdb_poster_path   = @tmdb_poster_path,
    tmdb_backdrop_path = @tmdb_backdrop_path,
    metadata_state     = 'matched',
    metadata_error     = NULL,
    matched_at         = @now,
    updated_at         = @now
WHERE id = @id AND deleted_at IS NULL;

-- name: ListChildItems :many
-- The children of a series (seasons, episodes without season folder, extras) or of a season.
SELECT * FROM items
WHERE parent_id = @parent_id AND deleted_at IS NULL
ORDER BY season_number, episode_number, id;

-- name: ListEpisodesToMatch :many
-- Episodes of a series (below a season or directly below the series) that still wait for their
-- metadata: pending, or skipped because the series was not matched yet.
SELECT e.id FROM items AS e
WHERE e.kind = 'episode' AND e.deleted_at IS NULL
  AND (e.metadata_state = 'pending' OR (e.metadata_state = 'skipped' AND e.metadata_error = 'series_unmatched'))
  AND (e.parent_id = @series_id
       OR e.parent_id IN (SELECT s.id FROM items AS s WHERE s.parent_id = @series_id AND s.kind = 'season'))
ORDER BY e.season_number, e.episode_number, e.id;

-- name: ListPendingMatchItems :many
-- Titles of a library whose metadata is pending without an active match job (QueuePendingMetadata):
-- movies and series, and episodes whose series is matched (the series job fills the others).
SELECT i.id, i.kind FROM items AS i
LEFT JOIN items AS p ON p.id = i.parent_id
LEFT JOIN items AS g ON g.id = p.parent_id
WHERE i.library_id = @library_id AND i.deleted_at IS NULL AND i.metadata_state = 'pending'
  AND (i.kind IN ('movie', 'series')
       OR (i.kind = 'episode' AND ((p.kind = 'series' AND p.metadata_state = 'matched')
                                   OR (g.kind = 'series' AND g.metadata_state = 'matched'))))
  AND NOT EXISTS (SELECT 1 FROM jobs AS j
                  WHERE j.idempotency_key = 'match:' || i.id AND j.state IN ('queued', 'running'))
ORDER BY i.id;

-- name: ListUpgradeMatchItems :many
-- Matched TMDB movies and series of a library without image paths and without a finished trailer
-- check: titles matched before M3, which lack the image paths, credits and trailers only a match
-- writes (QueuePendingMetadata queues one match each, at most @limit_count per run). The M3 match
-- stores the paths or queues trailer.find, whose check sets trailer_checked_at, so a title is
-- queued once.
SELECT i.id FROM items AS i
WHERE i.library_id = @library_id AND i.deleted_at IS NULL
  AND i.kind IN ('movie', 'series') AND i.metadata_state = 'matched' AND i.tmdb_id IS NOT NULL
  AND i.tmdb_poster_path IS NULL AND i.tmdb_backdrop_path IS NULL AND i.trailer_checked_at IS NULL
  AND instr(i.locked_fields, '"trailer"') = 0
  AND NOT EXISTS (SELECT 1 FROM jobs AS j
                  WHERE j.idempotency_key IN ('match:' || i.id, 'trailer:' || i.id) AND j.state IN ('queued', 'running'))
ORDER BY i.id
LIMIT @limit_count;

-- name: AssignItemMatch :exec
-- Manual match (B-6): the chosen provider ID, the other provider ID, imdb_id and the AniList id
-- cleared (another title at the provider is another anime, the fill looks it up again), the IDs
-- locked, metadata and trailers pending until the queued match has run; the provenance rows of
-- the old identity go with it (the caller deletes them in the same transaction).
UPDATE items
SET tmdb_id        = @tmdb_id,
    tvdb_id        = @tvdb_id,
    imdb_id        = NULL,
    anilist_id     = NULL,
    filled_providers = NULL,
    locked_fields  = @locked_fields,
    metadata_state = 'pending',
    metadata_error = NULL,
    trailer_state  = 'pending',
    updated_at     = @now
WHERE id = @id AND deleted_at IS NULL;

-- name: ListMediaFileIDsByItem :many
-- The files of a title (refresh probes them again).
SELECT id FROM media_files WHERE item_id = @item_id ORDER BY id;

-- name: SetItemMetadataState :exec
UPDATE items
SET metadata_state = @metadata_state,
    metadata_error = @metadata_error,
    matched_at     = @matched_at,
    updated_at     = @now
WHERE id = @id;

-- name: ListItemTextsByItem :many
SELECT * FROM item_texts WHERE item_id = @item_id ORDER BY lang;

-- name: UpsertItemText :exec
-- sort_key is model.SortKey(lang, title): every writer of a text row sets it (ADR 0016), except
-- that a sort title the administrator set (the lock sort_title:<lang> in items.locked_fields,
-- concept 15 7.5) keeps the stored key for every writer. rank is the position of lang in the
-- effective metadata language list of the title (ADR 0018, 0 = first).
INSERT INTO item_texts (item_id, lang, title, overview, tagline, sort_key, rank)
VALUES (@item_id, @lang, @title, @overview, @tagline, @sort_key, @rank)
ON CONFLICT (item_id, lang) DO UPDATE
SET title = excluded.title, overview = excluded.overview, tagline = excluded.tagline,
    sort_key = CASE
        WHEN EXISTS (SELECT 1 FROM items AS li WHERE li.id = excluded.item_id
                       AND instr(li.locked_fields, '"sort_title:' || excluded.lang || '"') > 0)
        THEN item_texts.sort_key ELSE excluded.sort_key END,
    rank = excluded.rank;

-- name: DeleteItemText :exec
-- metadata.fill removes the row of a language that left the effective list (ADR 0018).
DELETE FROM item_texts WHERE item_id = @item_id AND lang = @lang;

-- name: GetSystemSetting :one
-- A settings row for the metadata settings (metadata.Settings, ADR 0018); sql.ErrNoRows = default.
SELECT value FROM settings WHERE key = @key;

-- name: ListSystemSettingsByPrefix :many
-- Every settings row whose key starts with the prefix (metadata.*), for metadata.Settings.
SELECT key, value FROM settings WHERE key LIKE @prefix ORDER BY key;

-- name: CountItemsForMetadataApply :one
-- Matched titles (movies and series) that metadata.apply would queue, optionally of some libraries.
SELECT count(*) FROM items
WHERE deleted_at IS NULL AND kind IN ('movie', 'series') AND metadata_state = 'matched'
  AND (sqlc.arg('all_libraries') = 1 OR library_id IN (sqlc.slice('library_ids')));

-- name: ListImagesByItem :many
-- Every stored image row of a title, originals included (image.fetch compares them with TMDB).
SELECT * FROM images WHERE item_id = @item_id ORDER BY kind, size, format;

-- name: UpsertImage :exec
INSERT INTO images (item_id, kind, size, format, lang, provider, source_url, rel_path, content_hash, media_type, width,
                    height, bytes, accent, created_at)
VALUES (@item_id, @kind, @size, @format, @lang, @provider, @source_url, @rel_path, @content_hash, @media_type, @width,
        @height, @bytes, @accent, @now)
ON CONFLICT (item_id, kind, size, format) DO UPDATE
SET lang         = excluded.lang,
    provider     = excluded.provider,
    source_url   = excluded.source_url,
    rel_path     = excluded.rel_path,
    content_hash = excluded.content_hash,
    media_type   = excluded.media_type,
    width        = excluded.width,
    height       = excluded.height,
    bytes        = excluded.bytes,
    accent       = excluded.accent,
    created_at   = excluded.created_at;

-- name: DeleteImage :exec
-- Removes a row the pipeline no longer produces (for example the M1a poster in w500).
DELETE FROM images WHERE id = @id;

-- The decided image source per title and kind (ADR 0018, concept 10 4.4): the single input of
-- image.fetch. chosen = 1 is the administrator's choice (the kind is in locked_fields too).

-- name: ListItemImagesByItem :many
SELECT * FROM item_images WHERE item_id = @item_id ORDER BY kind;

-- name: UpsertItemImage :exec
INSERT INTO item_images (item_id, kind, provider, lang, source, chosen, updated_at)
VALUES (@item_id, @kind, @provider, @lang, @source, @chosen, @now)
ON CONFLICT (item_id, kind) DO UPDATE
SET provider = excluded.provider, lang = excluded.lang, source = excluded.source, chosen = excluded.chosen,
    updated_at = excluded.updated_at;

-- name: DeleteItemImage :exec
DELETE FROM item_images WHERE item_id = @item_id AND kind = @kind;

-- Image files the scanner found next to the media (concept 10 4.6): the local candidates of fill
-- and of the image choice; rel_path is relative to the root root_id and read through paths only.

-- name: ListItemLocalImages :many
SELECT * FROM item_local_images WHERE item_id = @item_id ORDER BY kind, rel_path;

-- name: ListLocalImagesByRoot :many
SELECT * FROM item_local_images WHERE root_id = @root_id ORDER BY item_id, kind, rel_path;

-- name: UpsertItemLocalImage :exec
INSERT INTO item_local_images (item_id, kind, rel_path, root_id, width, height, size, mtime)
VALUES (@item_id, @kind, @rel_path, @root_id, @width, @height, @size, @mtime)
ON CONFLICT (item_id, kind, rel_path) DO UPDATE
SET root_id = excluded.root_id, width = excluded.width, height = excluded.height, size = excluded.size, mtime = excluded.mtime;

-- name: DeleteItemLocalImage :exec
DELETE FROM item_local_images WHERE item_id = @item_id AND kind = @kind AND rel_path = @rel_path;

-- name: CountItemsWithLocalImages :one
SELECT count(DISTINCT item_id) FROM item_local_images;

-- name: GetLibraryRootPath :one
SELECT path FROM library_roots WHERE id = @id;

-- name: ListMatchedItemImages :many
-- Stored images of the matched titles of a library, for the image check after a scan
-- (QueueIncompleteImages). Titles without any image row are not listed here (ListItemsMissingImages
-- covers those with a stored path).
SELECT i.id AS item_id, i.locked_fields, im.kind, im.size, im.format, im.source_url, im.rel_path, im.accent, im.width
FROM items AS i
JOIN images AS im ON im.item_id = i.id
WHERE i.library_id = @library_id
  AND i.deleted_at IS NULL
  AND i.metadata_state = 'matched'
ORDER BY i.id, im.kind, im.size, im.format;

-- name: ListItemsMissingImages :many
-- Matched titles of a library with a decided image source (item_images, ADR 0018) but no row of
-- that kind yet (their image.fetch failed or never ran); the cover (poster) of audiobooks,
-- podcasts and episodes with a cover_path or cover_url (M6).
SELECT i.id, i.kind, i.locked_fields,
       CAST((EXISTS (SELECT 1 FROM item_images AS ii WHERE ii.item_id = i.id AND ii.kind = 'poster')
             OR ((i.cover_path IS NOT NULL OR i.cover_url IS NOT NULL) AND i.kind IN ('audiobook', 'podcast', 'podcast_episode')))
            AND NOT EXISTS (SELECT 1 FROM images AS im WHERE im.item_id = i.id AND im.kind = 'poster') AS INTEGER) AS poster_missing,
       CAST(EXISTS (SELECT 1 FROM item_images AS ii WHERE ii.item_id = i.id AND ii.kind = 'backdrop')
            AND NOT EXISTS (SELECT 1 FROM images AS im WHERE im.item_id = i.id AND im.kind = 'backdrop') AS INTEGER) AS backdrop_missing,
       CAST(EXISTS (SELECT 1 FROM item_images AS ii WHERE ii.item_id = i.id AND ii.kind = 'logo')
            AND NOT EXISTS (SELECT 1 FROM images AS im WHERE im.item_id = i.id AND im.kind = 'logo') AS INTEGER) AS logo_missing
FROM items AS i
WHERE i.library_id = @library_id
  AND i.deleted_at IS NULL
  AND i.metadata_state = 'matched'
  AND (((EXISTS (SELECT 1 FROM item_images AS ii WHERE ii.item_id = i.id AND ii.kind = 'poster')
          OR ((i.cover_path IS NOT NULL OR i.cover_url IS NOT NULL) AND i.kind IN ('audiobook', 'podcast', 'podcast_episode')))
        AND NOT EXISTS (SELECT 1 FROM images AS im WHERE im.item_id = i.id AND im.kind = 'poster'))
    OR (EXISTS (SELECT 1 FROM item_images AS ii WHERE ii.item_id = i.id AND ii.kind = 'backdrop')
        AND NOT EXISTS (SELECT 1 FROM images AS im WHERE im.item_id = i.id AND im.kind = 'backdrop'))
    OR (EXISTS (SELECT 1 FROM item_images AS ii WHERE ii.item_id = i.id AND ii.kind = 'logo')
        AND NOT EXISTS (SELECT 1 FROM images AS im WHERE im.item_id = i.id AND im.kind = 'logo')))
ORDER BY i.id;

-- Provenance per field (item_field_sources, ADR 0015): which provider of the list won a field.

-- name: ListItemFieldSources :many
SELECT field, lang, provider FROM item_field_sources WHERE item_id = @item_id ORDER BY field, lang;

-- name: UpsertItemFieldSource :exec
INSERT INTO item_field_sources (item_id, field, lang, provider) VALUES (@item_id, @field, @lang, @provider)
ON CONFLICT (item_id, field, lang) DO UPDATE SET provider = excluded.provider;

-- name: DeleteItemFieldSources :exec
DELETE FROM item_field_sources WHERE item_id = @item_id;

-- name: UpdateItemAniListID :exec
UPDATE items SET anilist_id = @anilist_id, updated_at = @now WHERE id = @id;

-- name: UpdateItemFill :exec
-- Writes the fields the fill job decided for a movie or series (metadata.fill, ADR 0015); the
-- caller keeps the current value of every field it does not fill. The match state stays.
UPDATE items
SET original_title     = @original_title,
    year               = @year,
    release_date       = @release_date,
    runtime_minutes    = @runtime_minutes,
    genres             = @genres,
    own_rating         = @own_rating,
    rating_source      = @rating_source,
    tmdb_poster_path   = @tmdb_poster_path,
    tmdb_backdrop_path = @tmdb_backdrop_path,
    vote_average       = @vote_average,
    original_language  = @original_language,
    anilist_id         = @anilist_id,
    filled_providers   = @filled_providers,
    updated_at         = @now
WHERE id = @id AND deleted_at IS NULL;

-- name: SetItemFilledProviders :exec
-- Records the list a fill ran with (a run with one provider writes provenance only).
UPDATE items SET filled_providers = @filled_providers, updated_at = @now WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateChildFill :exec
-- Writes the fields the fill job decided for a season or episode; the match state stays.
UPDATE items
SET release_date       = @release_date,
    runtime_minutes    = @runtime_minutes,
    tmdb_poster_path   = @tmdb_poster_path,
    tmdb_backdrop_path = @tmdb_backdrop_path,
    updated_at         = @now
WHERE id = @id AND deleted_at IS NULL;

-- name: ListFillItems :many
-- Matched movies and series of a library with more than one provider in their list (a JSON list
-- with a comma; provider names carry none) whose last fill ran with another list or never ran
-- (items.filled_providers), without an active fill job: the convergence of QueuePendingMetadata
-- (ADR 0015). The caller drops the titles whose only difference is a provider that is not
-- configured right now (QueuePendingFills), so a switched-off AniList queues nothing and a
-- switched-on one queues every title once.
SELECT i.id, i.providers, i.filled_providers FROM items AS i
WHERE i.library_id = @library_id AND i.deleted_at IS NULL
  AND i.kind IN ('movie', 'series') AND i.metadata_state = 'matched'
  AND instr(i.providers, ',') > 0
  AND (i.filled_providers IS NULL OR i.filled_providers <> i.providers)
  AND NOT EXISTS (SELECT 1 FROM jobs AS j
                  WHERE j.idempotency_key = 'fill:' || i.id AND j.state IN ('queued', 'running'))
ORDER BY i.id;

-- name: ListVoteBackfillItems :many
-- One batch of metadata.vote_backfill (concept 08 4.2, ADR 0016): matched movies and series
-- without a vote average whose list names a provider that delivers one (tmdb, anilist), whose
-- vote_average is not locked and that carry no provenance row for vote_average (a title whose
-- providers gave no vote gets one naming its identifier, so it is not asked again), in id order
-- after @after_id. Provider names and field names carry no quote, so instr on the JSON lists is
-- exact (like ListFillItems).
SELECT * FROM items
WHERE deleted_at IS NULL AND kind IN ('movie', 'series') AND metadata_state = 'matched'
  AND vote_average IS NULL AND id > @after_id
  AND (instr(providers, '"tmdb"') > 0 OR instr(providers, '"anilist"') > 0)
  AND instr(locked_fields, '"vote_average"') = 0
  AND NOT EXISTS (SELECT 1 FROM item_field_sources AS s WHERE s.item_id = items.id AND s.field = 'vote_average')
ORDER BY id
LIMIT @limit_count;

-- name: SetItemVoteAverage :exec
-- Writes the vote average metadata.vote_backfill decided (NULL keeps it unknown).
UPDATE items SET vote_average = @vote_average, updated_at = @now WHERE id = @id AND deleted_at IS NULL;

-- name: ListOriginalLanguageBackfillItems :many
-- One batch of metadata.original_language_backfill (concept 13 11.7): matched movies and series
-- whose original language was never looked up (NULL; '' is "looked up, unknown") and is not
-- locked, in id order after @after_id.
SELECT * FROM items
WHERE deleted_at IS NULL AND kind IN ('movie', 'series') AND metadata_state = 'matched'
  AND original_language IS NULL AND id > @after_id
  AND instr(locked_fields, '"original_language"') = 0
ORDER BY id
LIMIT @limit_count;

-- name: SetItemOriginalLanguage :exec
-- Writes the original language the backfill decided ('' = looked up, unknown).
UPDATE items SET original_language = @original_language, updated_at = @now WHERE id = @id AND deleted_at IS NULL;

-- name: SetItemOriginalLanguageOverride :exec
-- The administrator's original language (setItemOriginalLanguage): the value (NULL = look it up
-- again) and the lock together.
UPDATE items SET original_language = @original_language, locked_fields = @locked_fields, updated_at = @now
WHERE id = @id AND deleted_at IS NULL;

-- name: DeleteItemFieldSource :exec
DELETE FROM item_field_sources WHERE item_id = @item_id AND field = @field;

-- Metadata settings (concept 10, ADR 0018): the server's keys live in settings (metadata.*), the
-- deviation of a library in library_metadata (NULL = like the server).

-- name: UpsertSystemSetting :exec
INSERT INTO settings (key, value, updated_at) VALUES (@key, @value, @now)
ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at;

-- name: GetLibraryMetadata :one
SELECT * FROM library_metadata WHERE library_id = @library_id;

-- name: ListLibraryMetadata :many
SELECT * FROM library_metadata ORDER BY library_id;

-- name: UpsertLibraryMetadata :exec
INSERT INTO library_metadata (library_id, text_langs, images, updated_at)
VALUES (@library_id, @text_langs, @images, @now)
ON CONFLICT (library_id) DO UPDATE SET text_langs = excluded.text_langs, images = excluded.images, updated_at = excluded.updated_at;

-- name: DeleteLibraryMetadata :exec
DELETE FROM library_metadata WHERE library_id = @library_id;

-- name: CountLibraryItemsForApply :one
-- Matched films and series of one library, the titles metadata.apply queues for it.
SELECT count(*) FROM items WHERE library_id = @library_id AND deleted_at IS NULL AND kind IN ('movie', 'series') AND metadata_state = 'matched';

-- name: ListApplyItems :many
-- The matched films and series of a library in id order, for metadata.apply (one match per title).
SELECT id, kind FROM items WHERE library_id = @library_id AND deleted_at IS NULL AND kind IN ('movie', 'series') AND metadata_state = 'matched' ORDER BY id;

-- name: ListLibraryIDs :many
SELECT id FROM libraries ORDER BY id;

-- name: SetItemLockedFields :exec
-- The administrator's lock of a kind (setItemImage, clearItemImage; concept 10 6.3).
UPDATE items SET locked_fields = @locked_fields, updated_at = @now WHERE id = @id;

-- name: CountForeignMediaFilesInDir :one
-- The present media files of other titles directly in the folder @dir of a root: an audiobook's
-- folder images are its cover candidates only while no other title's file lies beside its parts
-- (concept 19 D5.3).
SELECT count(*) FROM media_files
WHERE root_id = @root_id AND item_id <> @item_id AND missing_since IS NULL
  AND substr(rel_path, 1, length(CAST(@dir AS TEXT)) + 1) = CAST(@dir AS TEXT) || '/'
  AND instr(substr(rel_path, length(CAST(@dir AS TEXT)) + 2), '/') = 0;
