-- Concept 25, review M-5: the request id of a film locks a wish only while the wish is open. The
-- unique index wishes_request of migration 00069 covered closed wishes too: a withdrawn wish that
-- still carried the id of its request (the withdrawal waiting for Seerr) or a wish closed with
-- arr_failed made the next wish of another profile fail on UNIQUE when it attached to the same
-- request after a 409, and the outbox retried it forever. Now the index holds one id once among
-- the open wishes (queued, pending, processing; a film is never partial). Several closed wishes and
-- one open wish may share an id; GetWishBySeerrRequest takes the open one first.
--
-- An index only, nothing is rebuilt; the frame of the template stays.

-- +goose Up
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
BEGIN;

DROP INDEX wishes_request;
CREATE UNIQUE INDEX wishes_request ON wishes (seerr_request_id)
    WHERE seerr_request_id IS NOT NULL AND state IN ('queued', 'pending', 'processing');

COMMIT;
PRAGMA foreign_key_check;
PRAGMA foreign_keys=ON;
