-- Person and collection images with scope (concept 17 stage 3, ADR 0026). The rows come from the
-- job people.images (person_images, collection_images, migration 00040).

-- The portrait of a person with a role on a title the profile may see: the wanted format when
-- stored, else the JPEG. No role on a visible title: no row (404).
-- name: GetVisiblePersonImage :one
SELECT pi.* FROM person_images AS pi
WHERE pi.person_id = @person_id AND pi.status = 'ok' AND pi.format IN (@format, 'jpeg')
  AND EXISTS (SELECT 1 FROM credits AS c
              JOIN visible_items AS v ON v.item_id = c.item_id AND v.profile_id = @profile_id
              WHERE c.person_id = pi.person_id)
ORDER BY pi.format = 'webp' DESC
LIMIT 1;

-- The 'ok' rows (JPEG and WebP) of the collections among @collection_ids that are fully visible to
-- the profile: at least one visible member and no member that is not deleted and not visible. The
-- TMDB poster may show any member, so it is shown only to a profile that may see all of them; the
-- one rule for the endpoint and the fields of CollectionRef and Collection. visible_items holds only
-- available titles, so a member whose file is missing makes its collection not fully visible.
-- @profile_id comes before the slice: sqlc numbers it, and a numbered parameter after the expanded
-- slice would name one of its values.
-- name: ListFullyVisibleCollectionImages :many
SELECT ci.* FROM collection_images AS ci
WHERE EXISTS (SELECT 1 FROM collection_items AS m
              JOIN visible_items AS v ON v.item_id = m.item_id AND v.profile_id = @profile_id
              WHERE m.collection_id = ci.collection_id)
  AND ci.collection_id IN (sqlc.slice('collection_ids')) AND ci.status = 'ok'
  AND NOT EXISTS (SELECT 1 FROM collection_items AS m
                  JOIN items AS i ON i.id = m.item_id
                  WHERE m.collection_id = ci.collection_id AND i.deleted_at IS NULL
                    AND NOT EXISTS (SELECT 1 FROM visible_items AS vh
                                    WHERE vh.item_id = m.item_id AND vh.profile_id = @profile_id))
ORDER BY ci.collection_id, ci.format;
