-- The seasons of a series the profile can play (concept 25, data model 2.5 "Staffelstatus",
-- architecture section 11): per season number the episodes of the series the profile sees. An
-- episode file that holds several episodes (episode_end) counts each of them. The series must be
-- visible itself, else no row (404). The season number is the episode's, else its season's;
-- episodes without one are left out.
-- name: ListVisibleSeasonEpisodes :many
SELECT CAST(coalesce(e.season_number, se.season_number) AS INTEGER) AS season_number,
       CAST(sum(CASE WHEN e.episode_number IS NOT NULL AND e.episode_end IS NOT NULL AND e.episode_end >= e.episode_number
                     THEN e.episode_end - e.episode_number + 1 ELSE 1 END) AS INTEGER) AS episodes
FROM items AS sr
JOIN visible_items AS vsr ON vsr.item_id = sr.id AND vsr.profile_id = @profile_id
JOIN items AS se ON se.parent_id = sr.id AND se.kind = 'season'
JOIN items AS e ON e.parent_id = se.id AND e.kind = 'episode'
JOIN visible_items AS v ON v.item_id = e.id AND v.profile_id = @profile_id
WHERE sr.id = @series_id AND sr.kind = 'series'
  AND coalesce(e.season_number, se.season_number) IS NOT NULL
GROUP BY coalesce(e.season_number, se.season_number)
ORDER BY 1;

-- Whether the series exists for the profile (VisibleSeasons answers 404 for an unseen series and an
-- empty map for a visible one without playable episodes).
-- name: VisibleSeriesExists :one
SELECT EXISTS (
    SELECT 1 FROM items AS sr
    JOIN visible_items AS v ON v.item_id = sr.id AND v.profile_id = @profile_id
    WHERE sr.id = @series_id AND sr.kind = 'series'
) AS found;
