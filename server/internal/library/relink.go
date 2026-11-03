package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Late relinking of soft-deleted titles (ADR 0023, idea 04). A live film or series and a
// soft-deleted one are twins when they lie in the same library, have the same kind, share at least
// one provider id (TMDB, TVDB, IMDb) and no provider id contradicts. The deleted twin survives with
// its id, so ids on devices, in links and in offline packages stay valid; the live copy merges
// into it and is deleted for good.

// ErrNotTwins is returned by Relink when the two titles are not a soft-deleted title and its live
// copy (anymore).
var ErrNotTwins = errors.New("not a deleted title and its live copy")

// RelinkPair is a live film or series and the soft-deleted twin it would merge into.
type RelinkPair struct {
	LiveID, DeletedID string
	LibraryID         int64
	Kind              string // movie or series
	Title             string // of the live copy
	DeletedAt         time.Time
}

// RelinkUserData counts the user data rows of a title and its descendants, per table.
type RelinkUserData struct {
	Progress         int64
	ProgressEvents   int64
	ProgressDays     int64
	PlaybackSessions int64
	Watchlist        int64
	Favorites        int64
	CollectionItems  int64
	Notifications    int64
	TopPicks         int64
	Downloads        int64
	ActivityEvents   int64
}

// RelinkResult describes one relink: the kept (formerly deleted) and the removed (live copy) id,
// the nodes of the copy that merged into a counterpart (the title, seasons, episodes) and its
// children that moved below the twin for want of one.
type RelinkResult struct {
	KeptID, RemovedID string
	Merged, Moved     int
}

// ListRelinkPairs lists the live films and series with a soft-deleted twin, of one library or of
// all libraries (libraryID 0). Several live copies may name the same twin.
func (s *Service) ListRelinkPairs(ctx context.Context, libraryID int64) ([]RelinkPair, error) {
	rows, err := systemq.New(s.db.Read).ListRelinkPairs(ctx, libraryID)
	if err != nil {
		return nil, fmt.Errorf("list relink pairs: %w", err)
	}
	pairs := make([]RelinkPair, 0, len(rows))
	for _, r := range rows {
		pairs = append(pairs, RelinkPair{
			LiveID: r.LiveID, DeletedID: r.DeletedID, LibraryID: r.LibraryID, Kind: r.Kind, Title: r.Title,
			DeletedAt: time.UnixMilli(r.DeletedAt.Int64).UTC(),
		})
	}
	return pairs, nil
}

// CountRelinkUserData counts the user data of a title and its descendants per table.
func (s *Service) CountRelinkUserData(ctx context.Context, itemID string) (RelinkUserData, error) {
	row, err := systemq.New(s.db.Read).CountRelinkUserData(ctx, itemID)
	if err != nil {
		return RelinkUserData{}, fmt.Errorf("count user data of %s: %w", itemID, err)
	}
	return RelinkUserData(row), nil
}

// Relink merges the live copy liveID into its soft-deleted twin deletedID (see relink). It
// returns model.ErrNotFound when one of them does not exist, ErrNotTwins when they are no twins
// and model.ErrScanRunning while a scan of the library runs.
func (s *Service) Relink(ctx context.Context, deletedID, liveID string) (RelinkResult, error) {
	live, err := systemq.New(s.db.Read).GetItemSystem(ctx, liveID)
	if errors.Is(err, sql.ErrNoRows) {
		return RelinkResult{}, fmt.Errorf("%w: item %s", model.ErrNotFound, liveID)
	}
	if err != nil {
		return RelinkResult{}, fmt.Errorf("load item %s: %w", liveID, err)
	}
	unlock, ok := s.tryLockScan(live.LibraryID)
	if !ok {
		return RelinkResult{}, fmt.Errorf("%w: library %d", model.ErrScanRunning, live.LibraryID)
	}
	defer unlock()
	return s.relink(ctx, deletedID, liveID)
}

// RelinkDeletedTwin merges a live film or series into its soft-deleted twin, if it has one
// (metadata.WithRelink: called after metadata.match wrote the provider ids). It returns the id
// that survives and whether the copy merged; a title without twin stays as it is. While a scan of
// the library holds the scan lock, whose item index still names the copy, the relink waits until
// the lock is released (unlockScan) and the copy is reported as not merged.
func (s *Service) RelinkDeletedTwin(ctx context.Context, liveID string) (string, bool, error) {
	live, err := systemq.New(s.db.Read).GetItemSystem(ctx, liveID)
	if errors.Is(err, sql.ErrNoRows) {
		return liveID, false, nil
	}
	if err != nil {
		return liveID, false, fmt.Errorf("load item %s: %w", liveID, err)
	}
	if live.DeletedAt.Valid || (live.Kind != "movie" && live.Kind != "series") {
		return liveID, false, nil
	}
	// Taking the lock or queueing behind its holder happens under s.mu, like the holder's check
	// for queued relinks before it releases the lock, so no relink is left behind.
	s.mu.Lock()
	ch := s.scanLockLocked(live.LibraryID)
	select {
	case ch <- struct{}{}:
		s.mu.Unlock()
	default:
		s.relinkPending[live.LibraryID] = append(s.relinkPending[live.LibraryID], liveID)
		s.mu.Unlock()
		s.log.InfoContext(ctx, "relink waits for the scan lock", "library_id", live.LibraryID, "item_id", liveID)
		return liveID, false, nil
	}
	defer s.unlockScan(live.LibraryID, ch)
	return s.relinkTwin(ctx, liveID)
}

// relinkTwin merges liveID into its most recently deleted twin, if any; the caller holds the scan
// lock of the library.
func (s *Service) relinkTwin(ctx context.Context, liveID string) (string, bool, error) {
	deletedID, err := systemq.New(s.db.Read).FindDeletedTwin(ctx, liveID)
	if errors.Is(err, sql.ErrNoRows) {
		return liveID, false, nil
	}
	if err != nil {
		return liveID, false, fmt.Errorf("find deleted twin of %s: %w", liveID, err)
	}
	if _, err := s.relink(ctx, deletedID, liveID); err != nil {
		if errors.Is(err, ErrNotTwins) || errors.Is(err, model.ErrNotFound) {
			return liveID, false, nil // changed since the lookup
		}
		return liveID, false, err
	}
	return deletedID, true, nil
}

// relink merges the live copy liveID into its soft-deleted twin deletedID in one transaction; the
// caller holds the scan lock of the library. The twin comes back with its id. The copy's media
// files move to it, and its children merge by number: a season into the twin's season with the
// same number, an episode into the season's episode with the same number (both restored when the
// copy's node is live). A child without counterpart (and every extra) moves below the twin. The
// user data of every merged node follows with the rules of the duplicate merge (B-9): progress per
// profile (completed wins, then the larger position), events, days, top picks, watchlist,
// favourites, collections, notifications and downloads, plus playback sessions and activity
// events. The emptied nodes of the copy are deleted for good, the copy last; a deleted child of
// the twin without counterpart stays deleted. The triggers recompute availability and ratings. The
// twin gets a metadata.match, which describes the episodes that moved in.
func (s *Service) relink(ctx context.Context, deletedID, liveID string) (RelinkResult, error) {
	now := s.now()
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return RelinkResult{}, fmt.Errorf("begin relink: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	load := func(id string) (systemq.Item, error) {
		it, err := q.GetItemSystem(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return it, fmt.Errorf("%w: item %s", model.ErrNotFound, id)
		}
		if err != nil {
			return it, fmt.Errorf("load item %s: %w", id, err)
		}
		return it, nil
	}
	dead, err := load(deletedID)
	if err != nil {
		return RelinkResult{}, err
	}
	live, err := load(liveID)
	if err != nil {
		return RelinkResult{}, err
	}
	if !twins(dead, live) {
		return RelinkResult{}, fmt.Errorf("%w: %s and %s", ErrNotTwins, deletedID, liveID)
	}
	r := &relinker{q: q, now: now.UnixMilli()}
	if err := r.restore(ctx, deletedID); err != nil {
		return RelinkResult{}, err
	}
	if err := r.merge(ctx, deletedID, liveID); err != nil {
		return RelinkResult{}, err
	}
	if err := enqueueTx(ctx, q, now, matchJob(deletedID)); err != nil {
		return RelinkResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return RelinkResult{}, fmt.Errorf("commit relink: %w", err)
	}
	res := RelinkResult{KeptID: deletedID, RemovedID: liveID, Merged: r.merged, Moved: r.moved}
	s.log.InfoContext(ctx, "deleted title relinked", "library_id", live.LibraryID, "kind", live.Kind,
		"kept_id", deletedID, "removed_id", liveID, "merged", res.Merged, "moved", res.Moved)
	return res, nil
}

// twins reports whether dead is a soft-deleted film or series and live its live copy.
func twins(dead, live systemq.Item) bool {
	if dead.ID == live.ID || !dead.DeletedAt.Valid || live.DeletedAt.Valid || dead.LibraryID != live.LibraryID ||
		dead.Kind != live.Kind || (live.Kind != "movie" && live.Kind != "series") {
		return false
	}
	shared := (dead.TmdbID.Valid && dead.TmdbID == live.TmdbID) || (dead.TvdbID.Valid && dead.TvdbID == live.TvdbID) ||
		(dead.ImdbID.Valid && dead.ImdbID == live.ImdbID)
	ref := itemRef{id: dead.ID, tmdb: dead.TmdbID.Int64, tvdb: dead.TvdbID.Int64, imdb: dead.ImdbID.String}
	return shared && ref.fits(titleInfo{tmdb: live.TmdbID.Int64, tvdb: live.TvdbID.Int64, imdb: live.ImdbID.String})
}

// relinker writes one relink in its transaction.
type relinker struct {
	q             *systemq.Queries
	now           int64
	merged, moved int
}

func (r *relinker) restore(ctx context.Context, id string) error {
	if err := r.q.RestoreItem(ctx, systemq.RestoreItemParams{Now: r.now, ID: id}); err != nil {
		return fmt.Errorf("restore item %s: %w", id, err)
	}
	return nil
}

// merge moves the files, the user data and the children of the copy's node source into the
// twin's node target, then deletes source. target is restored by the caller when it should be.
func (r *relinker) merge(ctx context.Context, target, source string) error {
	if err := r.moveRows(ctx, target, source); err != nil {
		return err
	}
	if err := r.mergeChildren(ctx, target, source); err != nil {
		return err
	}
	// The children of source have moved or merged; nothing cascades from it any more.
	if err := r.q.AdminDeleteItem(ctx, source); err != nil {
		return fmt.Errorf("relink: delete %s: %w", source, err)
	}
	r.merged++
	return nil
}

// mergeChildren merges the seasons and episodes of source into those of target with the same
// number and moves the other children below target.
func (r *relinker) mergeChildren(ctx context.Context, target, source string) error {
	kids, err := r.q.ListRelinkChildren(ctx, sql.NullString{String: target, Valid: true})
	if err != nil {
		return fmt.Errorf("relink: list children of %s: %w", target, err)
	}
	// Live children come first, so a live counterpart wins over a deleted one of the same number.
	seasons, episodes := map[int64]string{}, map[int64]string{}
	for _, k := range kids {
		switch {
		case k.Kind == "season" && k.SeasonNumber.Valid:
			if _, ok := seasons[k.SeasonNumber.Int64]; !ok {
				seasons[k.SeasonNumber.Int64] = k.ID
			}
		case k.Kind == "episode" && k.EpisodeNumber.Valid:
			if _, ok := episodes[k.EpisodeNumber.Int64]; !ok {
				episodes[k.EpisodeNumber.Int64] = k.ID
			}
		}
	}
	own, err := r.q.ListRelinkChildren(ctx, sql.NullString{String: source, Valid: true})
	if err != nil {
		return fmt.Errorf("relink: list children of %s: %w", source, err)
	}
	for _, k := range own {
		counterpart := ""
		switch {
		case k.Kind == "season" && k.SeasonNumber.Valid:
			counterpart = seasons[k.SeasonNumber.Int64]
		case k.Kind == "episode" && k.EpisodeNumber.Valid:
			counterpart = episodes[k.EpisodeNumber.Int64]
		}
		if counterpart == "" {
			if err := r.q.RelinkSetParent(ctx, systemq.RelinkSetParentParams{
				ParentID: sql.NullString{String: target, Valid: true}, Now: r.now, ID: k.ID,
			}); err != nil {
				return fmt.Errorf("relink: move %s below %s: %w", k.ID, target, err)
			}
			r.moved++
			continue
		}
		// Restored before its files arrive, so the availability triggers count it for its parents.
		if !k.DeletedAt.Valid {
			if err := r.restore(ctx, counterpart); err != nil {
				return err
			}
		}
		if err := r.merge(ctx, counterpart, k.ID); err != nil {
			return err
		}
	}
	return nil
}

// moveRows moves the media files and the user data of source to target, with the rules of the
// duplicate merge (admin.MergeItems, B-9) plus playback sessions and activity events.
func (r *relinker) moveRows(ctx context.Context, target, source string) error {
	q, now := r.q, r.now
	tn, sn := sql.NullString{String: target, Valid: true}, sql.NullString{String: source, Valid: true}
	steps := []struct {
		what string
		run  func() error
	}{
		{"media files", func() error {
			return q.AdminMoveMediaFiles(ctx, systemq.AdminMoveMediaFilesParams{TargetID: target, SourceID: source, Now: now})
		}},
		{"progress", func() error {
			return q.AdminMergeProgress(ctx, systemq.AdminMergeProgressParams{TargetID: target, SourceID: source})
		}},
		{"progress events", func() error {
			return q.AdminMoveProgressEvents(ctx, systemq.AdminMoveProgressEventsParams{TargetID: target, SourceID: source})
		}},
		{"progress days", func() error {
			return q.AdminMergeProgressDays(ctx, systemq.AdminMergeProgressDaysParams{TargetID: target, SourceID: source})
		}},
		{"top picks", func() error {
			return q.AdminMoveTopPicks(ctx, systemq.AdminMoveTopPicksParams{TargetID: target, SourceID: source})
		}},
		{"watchlist", func() error {
			return q.AdminMoveWatchlist(ctx, systemq.AdminMoveWatchlistParams{TargetID: target, SourceID: source})
		}},
		{"favorites", func() error {
			return q.AdminMoveFavorites(ctx, systemq.AdminMoveFavoritesParams{TargetID: target, SourceID: source})
		}},
		{"collection items", func() error {
			return q.AdminMoveCollectionItems(ctx, systemq.AdminMoveCollectionItemsParams{TargetID: target, SourceID: source})
		}},
		{"notifications", func() error {
			return q.AdminMoveNotifications(ctx, systemq.AdminMoveNotificationsParams{TargetID: target, SourceID: source})
		}},
		{"notification parents", func() error {
			return q.AdminMoveNotificationParents(ctx, systemq.AdminMoveNotificationParentsParams{TargetID: tn, SourceID: sn})
		}},
		{"downloads", func() error {
			return q.AdminMoveDownloads(ctx, systemq.AdminMoveDownloadsParams{TargetID: target, SourceID: source, Now: now})
		}},
		{"playback sessions", func() error {
			return q.RelinkMovePlaybackSessions(ctx, systemq.RelinkMovePlaybackSessionsParams{TargetID: target, SourceID: source})
		}},
		{"activity events", func() error {
			return q.RelinkMoveActivityEvents(ctx, systemq.RelinkMoveActivityEventsParams{TargetID: tn, SourceID: sn})
		}},
	}
	for _, step := range steps {
		if err := step.run(); err != nil {
			return fmt.Errorf("relink %s of %s: %w", step.what, source, err)
		}
	}
	return nil
}
