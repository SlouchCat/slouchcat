-- Person and collection images (concept 17 stage 3, ADR 0026; package store/systemq). Owner:
-- metadata package, job people.images. Times are Unix milliseconds.
--
-- A person is a candidate when it has a profile_path and at least one role on a title that is not
-- deleted, or when it has rows but no profile_path any more (the rows and files go). A collection
-- is a candidate when it is a TMDB collection with a poster_path and at least one member that is
-- not deleted, or when it has rows but no poster_path. A candidate is open when it has no path
-- (rows to drop), no JPEG row of the current path, a 'missing' row older than @missing_before, or
-- an 'ok' JPEG row without a WebP row of the same path. An 'ok' row whose file is gone is open as
-- well; the job sees that through the file system, not here (QueuePeopleImages samples the files
-- through SampleStored*Images). The open condition is written out in
-- both the candidate listing (column is_open) and the open listing; keep them equal.

-- name: ListPersonImageCandidates :many
-- The candidates by id after the cursor @after_id, @limit_count at a time, with the JPEG and WebP
-- rows they have.
SELECT p.id, p.profile_path,
       pj.status AS jpeg_status, pj.rel_path AS jpeg_rel_path,
       pw.rel_path AS webp_rel_path,
       CAST(p.profile_path IS NULL
            OR pj.person_id IS NULL OR pj.source <> p.profile_path
            OR (pj.status = 'missing' AND pj.fetched_at < @missing_before)
            OR (pj.status = 'ok' AND (pw.person_id IS NULL OR pw.source <> p.profile_path)) AS INTEGER) AS is_open
FROM people AS p
LEFT JOIN person_images AS pj ON pj.person_id = p.id AND pj.format = 'jpeg'
LEFT JOIN person_images AS pw ON pw.person_id = p.id AND pw.format = 'webp'
WHERE p.id > @after_id
  AND ((p.profile_path IS NOT NULL
        AND EXISTS (SELECT 1 FROM credits AS c JOIN items AS i ON i.id = c.item_id
                    WHERE c.person_id = p.id AND i.deleted_at IS NULL))
       OR (p.profile_path IS NULL AND (pj.person_id IS NOT NULL OR pw.person_id IS NOT NULL)))
ORDER BY p.id
LIMIT @limit_count;

-- name: CountPersonImageCandidates :one
-- The count of ListPersonImageCandidates without a cursor: the total of the job's progress.
SELECT count(*)
FROM people AS p
WHERE (p.profile_path IS NOT NULL
       AND EXISTS (SELECT 1 FROM credits AS c JOIN items AS i ON i.id = c.item_id
                   WHERE c.person_id = p.id AND i.deleted_at IS NULL))
   OR (p.profile_path IS NULL AND EXISTS (SELECT 1 FROM person_images AS pi WHERE pi.person_id = p.id));

-- name: ListOpenPersonImages :many
-- The open candidates by id, at most @limit_count (QueuePendingMetadata asks for one, a run for
-- one more than it saw fail).
SELECT p.id
FROM people AS p
LEFT JOIN person_images AS pj ON pj.person_id = p.id AND pj.format = 'jpeg'
LEFT JOIN person_images AS pw ON pw.person_id = p.id AND pw.format = 'webp'
WHERE ((p.profile_path IS NOT NULL
        AND EXISTS (SELECT 1 FROM credits AS c JOIN items AS i ON i.id = c.item_id
                    WHERE c.person_id = p.id AND i.deleted_at IS NULL))
       OR (p.profile_path IS NULL AND (pj.person_id IS NOT NULL OR pw.person_id IS NOT NULL)))
  AND (p.profile_path IS NULL
       OR pj.person_id IS NULL OR pj.source <> p.profile_path
       OR (pj.status = 'missing' AND pj.fetched_at < @missing_before)
       OR (pj.status = 'ok' AND (pw.person_id IS NULL OR pw.source <> p.profile_path)))
ORDER BY p.id
LIMIT @limit_count;

-- name: SampleStoredPersonImages :many
-- The files of up to @limit_count 'ok' rows of candidates with a path, at random: QueuePeopleImages
-- looks for them on disk, since a restore or a wiped cache leaves rows that the open listing takes
-- as done.
SELECT pi.rel_path
FROM person_images AS pi
JOIN people AS p ON p.id = pi.person_id
WHERE pi.status = 'ok' AND pi.rel_path IS NOT NULL AND p.profile_path IS NOT NULL
  AND EXISTS (SELECT 1 FROM credits AS c JOIN items AS i ON i.id = c.item_id
              WHERE c.person_id = p.id AND i.deleted_at IS NULL)
ORDER BY random()
LIMIT @limit_count;

-- name: UpsertPersonImage :exec
INSERT INTO person_images (person_id, format, status, source, size, rel_path, content_hash, media_type,
                           width, height, bytes, fetched_at)
VALUES (@person_id, @format, @status, @source, @size, @rel_path, @content_hash, @media_type,
        @width, @height, @bytes, @now)
ON CONFLICT (person_id, format) DO UPDATE
SET status = excluded.status, source = excluded.source, size = excluded.size, rel_path = excluded.rel_path,
    content_hash = excluded.content_hash, media_type = excluded.media_type, width = excluded.width,
    height = excluded.height, bytes = excluded.bytes, fetched_at = excluded.fetched_at;

-- name: DeletePersonImage :exec
DELETE FROM person_images WHERE person_id = @person_id AND format = @format;

-- name: DeletePersonImages :exec
DELETE FROM person_images WHERE person_id = @person_id;

-- name: DeleteMissingPersonImages :execrows
-- people.images_backfill: the failed fetches are tried again by the next run.
DELETE FROM person_images WHERE status = 'missing';

-- name: ListCollectionImageCandidates :many
-- The collection candidates by id after the cursor @after_id, @limit_count at a time.
SELECT c.id, c.poster_path,
       cj.status AS jpeg_status, cj.rel_path AS jpeg_rel_path,
       cw.rel_path AS webp_rel_path,
       CAST(c.poster_path IS NULL
            OR cj.collection_id IS NULL OR cj.source <> c.poster_path
            OR (cj.status = 'missing' AND cj.fetched_at < @missing_before)
            OR (cj.status = 'ok' AND (cw.collection_id IS NULL OR cw.source <> c.poster_path)) AS INTEGER) AS is_open
FROM collections AS c
LEFT JOIN collection_images AS cj ON cj.collection_id = c.id AND cj.format = 'jpeg'
LEFT JOIN collection_images AS cw ON cw.collection_id = c.id AND cw.format = 'webp'
WHERE c.id > @after_id
  AND ((c.source = 'tmdb' AND c.poster_path IS NOT NULL
        AND EXISTS (SELECT 1 FROM collection_items AS ci JOIN items AS i ON i.id = ci.item_id
                    WHERE ci.collection_id = c.id AND i.deleted_at IS NULL))
       OR (c.poster_path IS NULL AND (cj.collection_id IS NOT NULL OR cw.collection_id IS NOT NULL)))
ORDER BY c.id
LIMIT @limit_count;

-- name: CountCollectionImageCandidates :one
SELECT count(*)
FROM collections AS c
WHERE (c.source = 'tmdb' AND c.poster_path IS NOT NULL
       AND EXISTS (SELECT 1 FROM collection_items AS ci JOIN items AS i ON i.id = ci.item_id
                   WHERE ci.collection_id = c.id AND i.deleted_at IS NULL))
   OR (c.poster_path IS NULL AND EXISTS (SELECT 1 FROM collection_images AS cimg WHERE cimg.collection_id = c.id));

-- name: ListOpenCollectionImages :many
SELECT c.id
FROM collections AS c
LEFT JOIN collection_images AS cj ON cj.collection_id = c.id AND cj.format = 'jpeg'
LEFT JOIN collection_images AS cw ON cw.collection_id = c.id AND cw.format = 'webp'
WHERE ((c.source = 'tmdb' AND c.poster_path IS NOT NULL
        AND EXISTS (SELECT 1 FROM collection_items AS ci JOIN items AS i ON i.id = ci.item_id
                    WHERE ci.collection_id = c.id AND i.deleted_at IS NULL))
       OR (c.poster_path IS NULL AND (cj.collection_id IS NOT NULL OR cw.collection_id IS NOT NULL)))
  AND (c.poster_path IS NULL
       OR cj.collection_id IS NULL OR cj.source <> c.poster_path
       OR (cj.status = 'missing' AND cj.fetched_at < @missing_before)
       OR (cj.status = 'ok' AND (cw.collection_id IS NULL OR cw.source <> c.poster_path)))
ORDER BY c.id
LIMIT @limit_count;

-- name: SampleStoredCollectionImages :many
-- The files of up to @limit_count 'ok' rows of collection candidates with a path, at random (like
-- SampleStoredPersonImages).
SELECT cimg.rel_path
FROM collection_images AS cimg
JOIN collections AS c ON c.id = cimg.collection_id
WHERE cimg.status = 'ok' AND cimg.rel_path IS NOT NULL AND c.source = 'tmdb' AND c.poster_path IS NOT NULL
  AND EXISTS (SELECT 1 FROM collection_items AS ci JOIN items AS i ON i.id = ci.item_id
              WHERE ci.collection_id = c.id AND i.deleted_at IS NULL)
ORDER BY random()
LIMIT @limit_count;

-- name: UpsertCollectionImage :exec
INSERT INTO collection_images (collection_id, format, status, source, size, rel_path, content_hash, media_type,
                               width, height, bytes, fetched_at)
VALUES (@collection_id, @format, @status, @source, @size, @rel_path, @content_hash, @media_type,
        @width, @height, @bytes, @now)
ON CONFLICT (collection_id, format) DO UPDATE
SET status = excluded.status, source = excluded.source, size = excluded.size, rel_path = excluded.rel_path,
    content_hash = excluded.content_hash, media_type = excluded.media_type, width = excluded.width,
    height = excluded.height, bytes = excluded.bytes, fetched_at = excluded.fetched_at;

-- name: DeleteCollectionImage :exec
DELETE FROM collection_images WHERE collection_id = @collection_id AND format = @format;

-- name: DeleteCollectionImages :exec
DELETE FROM collection_images WHERE collection_id = @collection_id;

-- name: DeleteMissingCollectionImages :execrows
DELETE FROM collection_images WHERE status = 'missing';
