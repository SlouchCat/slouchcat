-- Libraries and their roots (package store/appq). Owner: admin (CRUD) and library (scan state).
-- Times are Unix milliseconds.

-- name: InsertLibrary :one
INSERT INTO libraries (name, kind, created_at, updated_at) VALUES (@name, @kind, @now, @now)
RETURNING *;

-- name: InsertLibraryRoot :one
INSERT INTO library_roots (library_id, path, fs_type, watch, created_at)
VALUES (@library_id, @path, @fs_type, @watch, @now)
RETURNING *;

-- name: ListLibraries :many
SELECT * FROM libraries ORDER BY id;

-- name: GetLibrary :one
SELECT * FROM libraries WHERE id = @id;

-- name: ListLibraryRoots :many
SELECT * FROM library_roots WHERE library_id = @library_id ORDER BY id;

-- name: ListAllLibraryRoots :many
SELECT * FROM library_roots ORDER BY library_id, id;

-- name: GetLibraryRoot :one
SELECT * FROM library_roots WHERE id = @id;

-- name: SetLibraryScanState :exec
UPDATE libraries
SET last_scan_state = @state, last_scan_error = @error, updated_at = @now,
    last_scan_started_at = CASE WHEN @state = 'running' THEN @now ELSE last_scan_started_at END,
    last_scan_finished_at = CASE WHEN @state IN ('done', 'failed', 'aborted') THEN @now ELSE last_scan_finished_at END
WHERE id = @id;

-- name: SetLibraryScanProgress :exec
UPDATE libraries SET scan_files_seen = @seen, scan_files_total = @total, updated_at = @now WHERE id = @id;

-- name: AcceptLibraryLoss :exec
UPDATE libraries SET loss_accepted_at = @now, updated_at = @now WHERE id = @id;
