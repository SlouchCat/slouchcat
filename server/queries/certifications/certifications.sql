-- Certificates per title, provider and rating country (concept 22 section 4.11, migration 00064).
-- Package store/certq; importing it is allowed only in package metadata (depguard rule "certq").

-- name: DeleteItemCertifications :exec
DELETE FROM item_certifications WHERE item_id = @item_id AND provider = @provider;

-- name: InsertItemCertification :exec
INSERT INTO item_certifications (item_id, provider, country, certification, fetched_at)
VALUES (@item_id, @provider, @country, @certification, @now);

-- name: ListItemCertifications :many
SELECT provider, country, certification FROM item_certifications
WHERE item_id = @item_id
ORDER BY provider, country;

-- name: DeleteAllItemCertifications :exec
-- Every certificate of a title: the match found another title at a provider.
DELETE FROM item_certifications WHERE item_id = @item_id;
