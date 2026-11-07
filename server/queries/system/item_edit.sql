-- The administrator's metadata editor (concept 15 7.5, P-6; package admin): texts in the server's
-- metadata language, the title columns and their locks, and the genre suggestions. Times are Unix
-- milliseconds.

-- name: EditItemText :exec
-- Writes the administrator's text row of one language as given, sort_key and the typed sort_title
-- included (the lock sort_title:<lang> that keeps them from the other writers is set in the same
-- transaction); an existing row keeps its rank.
INSERT INTO item_texts (item_id, lang, title, overview, tagline, sort_key, sort_title, rank)
VALUES (@item_id, @lang, @title, @overview, @tagline, @sort_key, @sort_title, @rank)
ON CONFLICT (item_id, lang) DO UPDATE
SET title = excluded.title, overview = excluded.overview, tagline = excluded.tagline, sort_key = excluded.sort_key,
    sort_title = excluded.sort_title;

-- name: EditItemColumns :exec
UPDATE items
SET original_title = @original_title,
    year           = @year,
    release_date   = @release_date,
    genres         = @genres,
    locked_fields  = @locked_fields,
    updated_at     = @now
WHERE id = @id;

-- name: DeleteItemFieldSourceLang :exec
-- The provenance of a field the administrator wrote goes (the value is his).
DELETE FROM item_field_sources WHERE item_id = @item_id AND field = @field AND lang = @lang;

-- name: ListLiveGenres :many
-- The genres column of every live film and series ({"de": [...], "en": [...]}, longer than an
-- empty list or object); the editor collects the names of one language from it for its suggestions.
SELECT DISTINCT genres FROM items
WHERE deleted_at IS NULL AND kind IN ('movie', 'series') AND length(genres) > 2;
