-- Cast and crew from the metadata providers (package store/systemq). Owner: metadata package.
-- Times are Unix milliseconds.

-- name: UpsertPerson :one
INSERT INTO people (tmdb_id, name, profile_path, created_at)
VALUES (@tmdb_id, @name, @profile_path, @now)
ON CONFLICT (tmdb_id) DO UPDATE SET name = excluded.name, profile_path = excluded.profile_path
RETURNING id;

-- name: DeleteItemCredits :exec
DELETE FROM credits WHERE item_id = @item_id;

-- name: InsertCredit :exec
-- A person with two crew jobs of the same role (screenplay and story) keeps the first.
INSERT INTO credits (item_id, person_id, role, character, ord)
VALUES (@item_id, @person_id, @role, @character, @ord)
ON CONFLICT (item_id, person_id, role) DO NOTHING;

-- name: ListItemCreditsSystem :many
-- The stored credits of a title in display order (tests, administration).
SELECT c.person_id, p.tmdb_id, p.name, c.role, c.character, c.ord
FROM credits AS c
JOIN people AS p ON p.id = c.person_id
WHERE c.item_id = @item_id
ORDER BY c.role, c.ord, p.name;
