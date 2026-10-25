package store

import (
	"strings"
	"testing"
)

// Migration 00021 (M11, plan 5.3): the partial unique index spotlight_overrides_rank allows one
// entry per rank and list. Without it two entries could sit on rank 1 and the server would resolve
// the tie by position, silently. Unpinned overrides (pinned_rank NULL) are not limited, and the
// same rank in another list is free. (The 409 rank_taken answer of the administration is step 4.)
func TestSpotlightOverrideRankIsUniquePerList(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	const insert = `INSERT INTO spotlight_overrides (list_key, media, tmdb_id, hidden, pinned_rank, note, updated_at)
	                VALUES (?, ?, ?, 0, ?, NULL, 1)`
	mustExec(t, db.Write, insert, "trending", "movie", 1, 1)
	// The same rank in another list is free.
	mustExec(t, db.Write, insert, "popular", "movie", 1, 1)
	// Any number of unpinned overrides in one list.
	mustExec(t, db.Write, insert, "trending", "movie", 2, nil)
	mustExec(t, db.Write, insert, "trending", "show", 3, nil)

	// A second entry on rank 1 of trending is refused, also for the other medium: the rank counts
	// per list, because the row is a single row for media 'both'.
	for _, media := range []string{"movie", "show"} {
		_, err := db.Write.Exec(insert, "trending", media, 4, 1)
		if err == nil || !strings.Contains(err.Error(), "UNIQUE constraint failed") {
			t.Errorf("second %s on rank 1 of trending: err = %v, want a UNIQUE constraint error", media, err)
		}
	}
	// Moving an unpinned override onto a taken rank is refused as well.
	if _, err := db.Write.Exec(`UPDATE spotlight_overrides SET pinned_rank = 1 WHERE list_key = 'trending' AND tmdb_id = 2`); err == nil ||
		!strings.Contains(err.Error(), "UNIQUE constraint failed") {
		t.Errorf("update onto rank 1 of trending: err = %v, want a UNIQUE constraint error", err)
	}
	if n := queryInt(t, db.Read, `SELECT count(*) FROM spotlight_overrides WHERE pinned_rank = 1`); n.Int64 != 2 {
		t.Errorf("%d overrides on rank 1, want 2 (one per list)", n.Int64)
	}
}
