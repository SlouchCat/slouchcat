-- Scan state of libraries beyond libraries.sql (package store/appq). Owner: library package.
-- Times are Unix milliseconds.

-- name: MarkLibraryScanQueued :exec
-- A scan was queued; a running scan keeps its state until it finishes.
UPDATE libraries SET last_scan_state = 'queued', updated_at = @now
WHERE id = @id AND last_scan_state <> 'running';

-- name: ClearLibraryLossAccepted :exec
-- The accepted loss was applied by a scan. Only the acceptance that scan read is cleared, so an
-- acceptance given while the scan ran stays for the next one.
UPDATE libraries SET loss_accepted_at = NULL, updated_at = @now
WHERE id = @id AND loss_accepted_at = @accepted_at;

-- name: SetLibraryScanStatus :exec
-- Like SetLibraryScanState in libraries.sql, written without "IN (...)" on a parameter, which sqlc
-- leaves unrewritten there ("missing named argument state" at run time).
UPDATE libraries
SET last_scan_state = @state, last_scan_error = @error, updated_at = @now,
    last_scan_started_at = CASE WHEN @state = 'running' THEN @now ELSE last_scan_started_at END,
    last_scan_finished_at = CASE WHEN @state = 'done' OR @state = 'failed' OR @state = 'aborted'
                                 THEN @now ELSE last_scan_finished_at END
WHERE id = @id;
