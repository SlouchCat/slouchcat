-- Titles outside the library (concept 25, 08-datenmodell-api.md section 2.9, migration 00068).
-- Package store/externalq; importing it is allowed only in package metadata (depguard rule
-- "externalq"). Times are Unix milliseconds.

-- name: GetExternalTitle :one
SELECT * FROM external_titles WHERE media = @media AND tmdb_id = @tmdb_id;

-- name: UpsertExternalTitle :exec
INSERT INTO external_titles (media, tmdb_id, tvdb_id, imdb_id, original_title, year, release_date,
    runtime_minutes, genres, certifications, min_age, poster_path, backdrop_path, logo_path, trailers,
    status, seasons, next_episode, last_episode, fetched_at)
VALUES (@media, @tmdb_id, @tvdb_id, @imdb_id, @original_title, @year, @release_date, @runtime_minutes,
    @genres, @certifications, @min_age, @poster_path, @backdrop_path, @logo_path, @trailers, @status,
    @seasons, @next_episode, @last_episode, @fetched_at)
ON CONFLICT (media, tmdb_id) DO UPDATE SET
    tvdb_id = excluded.tvdb_id, imdb_id = excluded.imdb_id, original_title = excluded.original_title,
    year = excluded.year, release_date = excluded.release_date, runtime_minutes = excluded.runtime_minutes,
    genres = excluded.genres, certifications = excluded.certifications, min_age = excluded.min_age,
    poster_path = excluded.poster_path, backdrop_path = excluded.backdrop_path, logo_path = excluded.logo_path,
    trailers = excluded.trailers, status = excluded.status, seasons = excluded.seasons,
    next_episode = excluded.next_episode, last_episode = excluded.last_episode, fetched_at = excluded.fetched_at;

-- name: TouchExternalTitle :exec
-- A refresh that found nothing new to write (TMDB no longer has the title): the row keeps its
-- values and waits for the next due date like any other.
UPDATE external_titles SET fetched_at = @fetched_at WHERE media = @media AND tmdb_id = @tmdb_id;

-- name: ListStaleExternalTitles :many
-- The rows fetched before cutoff, the oldest first.
SELECT media, tmdb_id FROM external_titles
WHERE fetched_at < @cutoff
ORDER BY fetched_at, media, tmdb_id
LIMIT @limit_count;

-- name: ListExternalTitleAges :many
-- The certifications and ages in key order after (media, tmdb_id), for the new ages after a change
-- of the rating country.
SELECT media, tmdb_id, certifications, min_age FROM external_titles
WHERE media > @after_media OR (media = @after_media AND tmdb_id > @after_tmdb_id)
ORDER BY media, tmdb_id
LIMIT @limit_count;

-- name: SetExternalTitleMinAge :exec
UPDATE external_titles SET min_age = @min_age WHERE media = @media AND tmdb_id = @tmdb_id;

-- name: ListExternalTitleTexts :many
SELECT language, title, overview, tagline FROM external_title_texts
WHERE media = @media AND tmdb_id = @tmdb_id
ORDER BY language;

-- name: DeleteExternalTitleTexts :exec
DELETE FROM external_title_texts WHERE media = @media AND tmdb_id = @tmdb_id;

-- name: InsertExternalTitleText :exec
INSERT INTO external_title_texts (media, tmdb_id, language, title, overview, tagline)
VALUES (@media, @tmdb_id, @language, @title, @overview, @tagline);

-- name: ListExternalTitleImages :many
SELECT * FROM external_title_images
WHERE media = @media AND tmdb_id = @tmdb_id AND kind = @kind
ORDER BY size, format;

-- name: ListExternalTitleImageSizes :many
-- The stored sizes of every kind of a title in one format per kind (the JPEG, a logo's PNG): the
-- width, height and hash of the image references.
SELECT kind, size, width, height, content_hash FROM external_title_images
WHERE media = @media AND tmdb_id = @tmdb_id AND format = CASE kind WHEN 'logo' THEN 'png' ELSE 'jpeg' END
ORDER BY kind, width;

-- name: UpsertExternalTitleImage :exec
INSERT INTO external_title_images (media, tmdb_id, kind, size, format, lang, source_url, rel_path,
    content_hash, media_type, width, height, bytes, fetched_at)
VALUES (@media, @tmdb_id, @kind, @size, @format, @lang, @source_url, @rel_path, @content_hash,
    @media_type, @width, @height, @bytes, @fetched_at)
ON CONFLICT (media, tmdb_id, kind, size, format) DO UPDATE SET
    lang = excluded.lang, source_url = excluded.source_url, rel_path = excluded.rel_path,
    content_hash = excluded.content_hash, media_type = excluded.media_type, width = excluded.width,
    height = excluded.height, bytes = excluded.bytes, fetched_at = excluded.fetched_at;

-- name: DeleteExternalTitleImages :many
-- Every image of a kind (its TMDB path changed or is gone); returns the files to remove.
DELETE FROM external_title_images
WHERE media = @media AND tmdb_id = @tmdb_id AND kind = @kind
RETURNING rel_path;

-- name: DeleteExternalTitleImage :exec
DELETE FROM external_title_images WHERE id = @id;
