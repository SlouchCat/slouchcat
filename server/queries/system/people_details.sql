-- The facts of a person for the person page (concept 21 P-8, migration 00058; package
-- store/systemq). Owner: metadata package, job people.details. Times are Unix milliseconds.
--
-- A person is a candidate when it has a TMDB id and at least one role on a title that is not
-- deleted. A candidate is open when it was never fetched, was fetched before @stale_before or in
-- another main language than @lang. The open condition is written out in both queries; keep them
-- equal.

-- name: ListOpenPersonDetails :many
-- The open candidates by id after the cursor @after_id, @limit_count at a time.
SELECT p.id, p.tmdb_id
FROM people AS p
WHERE p.id > @after_id AND p.tmdb_id IS NOT NULL
  AND EXISTS (SELECT 1 FROM credits AS c JOIN items AS i ON i.id = c.item_id
              WHERE c.person_id = p.id AND i.deleted_at IS NULL)
  AND (p.details_fetched_at IS NULL OR p.details_fetched_at < CAST(@stale_before AS INTEGER)
       OR p.details_lang IS NULL OR p.details_lang <> CAST(@lang AS TEXT))
ORDER BY p.id
LIMIT @limit_count;

-- name: CountOpenPersonDetails :one
-- The count of ListOpenPersonDetails without a cursor: the total of the job's progress.
SELECT count(*)
FROM people AS p
WHERE p.tmdb_id IS NOT NULL
  AND EXISTS (SELECT 1 FROM credits AS c JOIN items AS i ON i.id = c.item_id
              WHERE c.person_id = p.id AND i.deleted_at IS NULL)
  AND (p.details_fetched_at IS NULL OR p.details_fetched_at < CAST(@stale_before AS INTEGER)
       OR p.details_lang IS NULL OR p.details_lang <> CAST(@lang AS TEXT));

-- name: StorePersonDetails :exec
-- The facts of one fetch; NULL for what TMDB does not know (a person TMDB does not know at all
-- gets NULL everywhere).
UPDATE people
SET biography = @biography, biography_lang = @biography_lang, birthday = @birthday, deathday = @deathday,
    place_of_birth = @place_of_birth, known_for_department = @known_for_department,
    total_credits = @total_credits, details_lang = CAST(@details_lang AS TEXT), details_fetched_at = CAST(@now AS INTEGER)
WHERE id = @id;

-- name: MarkPersonDetailsFetched :exec
-- A fetch that would fail the same way again (a 4xx other than 401 and 404): the facts stay as
-- they are and the person waits for the next refresh.
UPDATE people SET details_lang = CAST(@details_lang AS TEXT), details_fetched_at = CAST(@now AS INTEGER) WHERE id = @id;

-- The descriptions of TMDB collections (concept 21 O-18, migration 00060), fetched by the same job
-- after the people. A collection is a candidate when it comes from TMDB and has a member that is
-- not deleted; it is open like a person (never fetched, fetched before @stale_before or in another
-- main language than @lang). Manual collections are never candidates. Keep both queries equal.

-- name: ListOpenCollectionDetails :many
-- The open candidates by id after the cursor @after_id, @limit_count at a time.
SELECT c.id, c.tmdb_id
FROM collections AS c
WHERE c.id > CAST(@after_id AS TEXT) AND c.source = 'tmdb' AND c.tmdb_id IS NOT NULL
  AND EXISTS (SELECT 1 FROM collection_items AS ci JOIN items AS i ON i.id = ci.item_id
              WHERE ci.collection_id = c.id AND i.deleted_at IS NULL)
  AND (c.details_fetched_at IS NULL OR c.details_fetched_at < CAST(@stale_before AS INTEGER)
       OR c.details_lang IS NULL OR c.details_lang <> CAST(@lang AS TEXT))
ORDER BY c.id
LIMIT @limit_count;

-- name: CountOpenCollectionDetails :one
-- The count of ListOpenCollectionDetails without a cursor: part of the total of the job's progress.
SELECT count(*)
FROM collections AS c
WHERE c.source = 'tmdb' AND c.tmdb_id IS NOT NULL
  AND EXISTS (SELECT 1 FROM collection_items AS ci JOIN items AS i ON i.id = ci.item_id
              WHERE ci.collection_id = c.id AND i.deleted_at IS NULL)
  AND (c.details_fetched_at IS NULL OR c.details_fetched_at < CAST(@stale_before AS INTEGER)
       OR c.details_lang IS NULL OR c.details_lang <> CAST(@lang AS TEXT));

-- name: StoreCollectionDetails :exec
-- The description of one fetch; NULL when TMDB has none (or does not know the collection).
UPDATE collections
SET description = @description, description_lang = @description_lang,
    details_lang = CAST(@details_lang AS TEXT), details_fetched_at = CAST(@now AS INTEGER)
WHERE id = @id;

-- name: MarkCollectionDetailsFetched :exec
-- A fetch that would fail the same way again: the description stays, the collection waits for
-- the next refresh.
UPDATE collections SET details_lang = CAST(@details_lang AS TEXT), details_fetched_at = CAST(@now AS INTEGER) WHERE id = @id;
