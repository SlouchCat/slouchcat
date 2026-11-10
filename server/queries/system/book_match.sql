-- Book sources (concept 19 stage D3, D6.3 to D6.6): the match of an audiobook, its candidates, the
-- numbers and the list "Pruefen" of the page "Hoerbuecher: Quellen", and the unified author name
-- (D6.4). Package metadata writes the match, package library the author name, package admin reads.

-- The state of a match run; provider, id, region, time and score are NULL unless matched, error
-- holds the reason of the state error.
-- name: SetBookMatch :exec
UPDATE items
SET book_match_state = @state,
    book_match_provider = sqlc.narg('provider'),
    book_match_id = sqlc.narg('provider_id'),
    book_match_region = sqlc.narg('region'),
    book_matched_at = sqlc.narg('matched_at'),
    book_match_score = sqlc.narg('score'),
    book_match_error = sqlc.narg('error'),
    book_match_manual = @manual,
    updated_at = @now
WHERE id = @id AND kind = 'audiobook';

-- Back to "not tried yet" (pending).
-- name: ClearBookMatch :exec
UPDATE items
SET book_match_state = NULL, book_match_provider = NULL, book_match_id = NULL, book_match_region = NULL,
    book_matched_at = NULL, book_match_score = NULL, book_match_error = NULL, book_match_manual = 0,
    updated_at = @now
WHERE id = @id AND kind = 'audiobook';

-- The match of one book (the refresh from the provider, the editor).
-- name: GetBookMatch :one
SELECT id, book_match_state, book_match_provider, book_match_id, book_match_region, book_matched_at,
       book_match_score, book_match_error, book_match_manual
FROM items
WHERE id = @id AND kind = 'audiobook';

-- Replacing the candidates of a book: delete them, then insert rank 1 to 3.
-- name: DeleteBookMatchCandidates :exec
DELETE FROM book_match_candidates WHERE item_id = @item_id;

-- name: InsertBookMatchCandidate :exec
INSERT INTO book_match_candidates (item_id, rank, provider, provider_id, score, candidate)
VALUES (@item_id, @rank, @provider, @provider_id, @score, @candidate);

-- The candidates of the given books, best first per book.
-- name: ListBookMatchCandidates :many
SELECT item_id, rank, provider, provider_id, score, candidate
FROM book_match_candidates
WHERE item_id IN (sqlc.slice('item_ids'))
ORDER BY item_id, rank;

-- The numbers of the page: books that are not deleted per state, NULL counted as pending.
-- name: CountAudiobooksByMatchState :many
SELECT CAST(coalesce(book_match_state, 'pending') AS TEXT) AS state, count(*) AS n
FROM items
WHERE kind = 'audiobook' AND deleted_at IS NULL
GROUP BY coalesce(book_match_state, 'pending');

-- "Jetzt alle ohne Zuordnung zuordnen": every book that is not deleted and not matched (pending,
-- review, none, error) with a file in a folder whose list holds a filler after local.
-- name: ListAudiobooksToMatch :many
SELECT i.id
FROM items AS i
WHERE i.kind = 'audiobook' AND i.deleted_at IS NULL
  AND (i.book_match_state IS NULL OR i.book_match_state IN ('review', 'none', 'error'))
  AND EXISTS (SELECT 1 FROM media_files AS m JOIN library_roots AS r ON r.id = m.root_id
              WHERE m.item_id = i.id AND json_array_length(r.providers) > 1)
ORDER BY i.id;

-- The list "Pruefen": books in review or error, by title.
-- name: ListAudiobookReview :many
SELECT i.id, i.title, CAST(coalesce(i.author_display, i.author, '') AS TEXT) AS author, i.library_id,
       i.duration_ms, CAST(i.book_match_state AS TEXT) AS state, i.book_match_error, i.book_matched_at
FROM items AS i
WHERE i.kind = 'audiobook' AND i.deleted_at IS NULL AND i.book_match_state IN ('review', 'error')
ORDER BY i.sort_title, i.title, i.id;

-- The unified author name (D6.4); NULL = like author. Only a changed value is written.
-- name: UpdateAuthorDisplay :execrows
UPDATE items
SET author_display = sqlc.narg('author_display'), updated_at = @now
WHERE id = @id AND kind = 'audiobook' AND author_display IS NOT sqlc.narg('author_display');
