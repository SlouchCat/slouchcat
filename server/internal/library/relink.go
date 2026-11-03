package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Late relinking of soft-deleted titles (ADR 0023, idea 04). A live film or series and a
// soft-deleted one are twins when they lie in the same library, have the same kind, share at least
// one provider id (TMDB, TVDB, IMDb) and no provider id contradicts. The deleted twin survives with
// its id, so ids on devices, in links and in offline packages stay valid; the live copy merges
// into it and is deleted for good.
//
// After metadata.match wrote the provider ids of a title, QueueRelink (metadata.WithRelink) queues
// the job library.relink for a title with a deleted twin; the job merges it under the scan lock of
// the library (handleRelink). This automatic path only takes a fresh copy (relinkGrace). The
// maintenance action (ListRelinkPairs, CheckRelinkPair, Relink) knows no freshness rule: the
// administrator sees both titles and decides.

// relinkGrace is how long before its twin's deletion a live copy may have been created and still
// merge on its own (decision 21, ADR 0023): a new folder is added and scanned first, the old one
// removed afterwards. Two titles that lived side by side for longer stay apart unless the
// administrator merges them with the maintenance action.
const relinkGrace = 7 * 24 * time.Hour

// autoGrace is relinkGrace for FindDeletedTwin; anyAge lets a copy of any age count.
var (
	autoGrace = sql.NullInt64{Int64: relinkGrace.Milliseconds(), Valid: true}
	anyAge    = sql.NullInt64{}
)

// ErrNotTwins is returned by Relink when the two titles are not a soft-deleted title and its live
// copy (anymore).
var ErrNotTwins = errors.New("not a deleted title and its live copy")

// RelinkPair is a live film or series and the soft-deleted twin it would merge into.
type RelinkPair struct {
	LiveID, DeletedID string
	LibraryID         int64
	Kind              string // movie or series
	Title             string // of the live copy
	Year              *int   // of the live copy
	DeletedTitle      string
	DeletedYear       *int
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
	// Folded counts the older soft-deleted twins of the kept title that merged into it as well.
	Folded int
}

// ListRelinkPairs lists the live films and series with a soft-deleted twin of any age, of one
// library or of all libraries (libraryID 0), for the maintenance action. Several live copies may
// name the same twin.
func (s *Service) ListRelinkPairs(ctx context.Context, libraryID int64) ([]RelinkPair, error) {
	rows, err := systemq.New(s.db.Read).ListRelinkPairs(ctx, libraryID)
	if err != nil {
		return nil, fmt.Errorf("list relink pairs: %w", err)
	}
	year := func(v sql.NullInt64) *int {
		if !v.Valid {
			return nil
		}
		y := int(v.Int64)
		return &y
	}
	pairs := make([]RelinkPair, 0, len(rows))
	for _, r := range rows {
		pairs = append(pairs, RelinkPair{
			LiveID: r.LiveID, DeletedID: r.DeletedID, LibraryID: r.LibraryID, Kind: r.Kind, Title: r.Title,
			Year: year(r.Year), DeletedTitle: r.DeletedTitle, DeletedYear: year(r.DeletedYear),
			DeletedAt: time.UnixMilli(r.DeletedAt.Int64).UTC(),
		})
	}
	return pairs, nil
}

// CountRelinkUserData counts the user data of the titles and their descendants per table, in one
// statement; a title without user data is missing from the map.
func (s *Service) CountRelinkUserData(ctx context.Context, itemIDs []string) (map[string]RelinkUserData, error) {
	out := make(map[string]RelinkUserData, len(itemIDs))
	if len(itemIDs) == 0 {
		return out, nil
	}
	rows, err := systemq.New(s.db.Read).CountRelinkUserData(ctx, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("count user data of %d titles: %w", len(itemIDs), err)
	}
	for _, r := range rows {
		out[r.ID] = RelinkUserData{
			Progress: r.Progress, ProgressEvents: r.ProgressEvents, ProgressDays: r.ProgressDays,
			PlaybackSessions: r.PlaybackSessions, Watchlist: r.Watchlist, Favorites: r.Favorites,
			CollectionItems: r.CollectionItems, Notifications: r.Notifications, TopPicks: r.TopPicks,
			Downloads: r.Downloads, ActivityEvents: r.ActivityEvents,
		}
	}
	return out, nil
}

// CheckRelinkPair checks a pair of the maintenance action before anything moves and returns the
// library of the live title. It returns model.ErrNotFound when an id is unknown or names neither a
// film nor a series, and ErrNotTwins when deletedID is not (any more) the twin ListRelinkPairs
// would show for liveID: the deleted title came back, the copy is deleted or the titles changed.
func (s *Service) CheckRelinkPair(ctx context.Context, deletedID, liveID string) (int64, error) {
	q := systemq.New(s.db.Read)
	var libraryID int64
	for _, id := range []string{deletedID, liveID} {
		it, err := q.GetItemSystem(ctx, id)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && it.Kind != "movie" && it.Kind != "series") {
			return 0, fmt.Errorf("%w: film or series %s", model.ErrNotFound, id)
		}
		if err != nil {
			return 0, fmt.Errorf("load item %s: %w", id, err)
		}
		libraryID = it.LibraryID
	}
	twin, err := q.FindDeletedTwin(ctx, systemq.FindDeletedTwinParams{LiveID: liveID, GraceMs: anyAge})
	if errors.Is(err, sql.ErrNoRows) || (err == nil && twin != deletedID) {
		return libraryID, fmt.Errorf("%w: %s and %s", ErrNotTwins, deletedID, liveID)
	}
	if err != nil {
		return libraryID, fmt.Errorf("find deleted twin of %s: %w", liveID, err)
	}
	return libraryID, nil
}

// Relink is the maintenance action's merge of the live copy liveID into its soft-deleted twin
// deletedID (see relink), without the freshness rule of the automatic path. It returns
// model.ErrNotFound when one of them does not exist, ErrNotTwins when they are no twins and
// model.ErrScanRunning while a scan of the library runs.
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
	return s.relink(ctx, deletedID, liveID, anyAge)
}

// relinkRetry is how long library.relink waits when a scan of the library holds the scan lock.
const relinkRetry = time.Minute

func relinkKey(itemID string) string { return "relink:" + itemID }

// QueueRelink is the hook after metadata.match wrote the provider ids of a title
// (metadata.WithRelink, ADR 0023). It reports whether the title is a live film or series with a
// soft-deleted twin and then queues library.relink for it (key relink:<id>), which merges the
// title into its twin; the caller's match ends there. The job waits for the debounce window, so
// the match that queued it has finished before the job may queue the title's match again.
func (s *Service) QueueRelink(ctx context.Context, liveID string) (bool, error) {
	if _, err := systemq.New(s.db.Read).FindDeletedTwin(ctx, systemq.FindDeletedTwinParams{LiveID: liveID, GraceMs: autoGrace}); errors.Is(err, sql.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("find deleted twin of %s: %w", liveID, err)
	}
	payload, err := json.Marshal(itemPayload{liveID})
	if err != nil {
		return false, fmt.Errorf("encode %s payload: %w", JobRelink, err)
	}
	if _, err := s.queue.Enqueue(ctx, jobs.NewJob{
		Type: JobRelink, Priority: prioMatch, IdempotencyKey: relinkKey(liveID), Payload: payload, Delay: jobs.DebounceWindow,
	}); err != nil {
		return false, err
	}
	return true, nil
}

// QueueRelinksTx queues library.relink in the caller's transaction for every live film or series
// that became the fresh twin (relinkGrace) of a title soft-deleted at now in that transaction. A
// copy matched while its original still lived found no twin then (QueueRelink); this catches the
// order "add the new folder, scan, remove the old one" (ADR 0023). Every soft delete of titles
// calls it: RemoveRoot, a content change of a folder, the sweep, IgnoreFiles and the cleanup of
// package admin.
func QueueRelinksTx(ctx context.Context, q *systemq.Queries, now time.Time) error {
	ids, err := q.ListFreshCopiesOfDeleted(ctx, systemq.ListFreshCopiesOfDeletedParams{
		Now: sql.NullInt64{Int64: now.UnixMilli(), Valid: true}, GraceMs: relinkGrace.Milliseconds(),
	})
	if err != nil {
		return fmt.Errorf("list fresh copies of deleted titles: %w", err)
	}
	for _, id := range ids {
		if err := enqueueTx(ctx, q, now, newJob{JobRelink, prioMatch, relinkKey(id), itemPayload{id}}); err != nil {
			return err
		}
	}
	return nil
}

// handleRelink runs library.relink: it merges a live film or series into its soft-deleted twin
// under the scan lock of the library, because a running scan's item index still names the copy.
// While a scan holds the lock, the job waits relinkRetry without using an attempt
// (jobs.RetryLater). A title that is gone has merged already (the maintenance action). After a
// merge the open streams are rechecked (WithVisibilityRecheck). When the twin is gone meanwhile
// (another copy of it merged first), the title's match runs again: it stopped after the title and
// now completes seasons, images and the rest. Database errors are returned for a retry; when the
// last attempt fails, the title's match runs again without the relink hook (queueCopyMatch), so
// the copy is still completed and the maintenance action can merge it later.
func (s *Service) handleRelink(ctx context.Context, j *jobs.Job) error {
	var p itemPayload
	if err := decodePayload(j, &p); err != nil {
		return err
	}
	err := s.runRelink(ctx, p.ItemID)
	var later *jobs.RetryLaterError
	if err != nil && !errors.As(err, &later) && j.Attempts >= j.MaxAttempts {
		if qerr := s.queueCopyMatch(context.WithoutCancel(ctx), p.ItemID, true); qerr != nil {
			return errors.Join(err, qerr)
		}
	}
	return err
}

func (s *Service) runRelink(ctx context.Context, itemID string) error {
	live, err := systemq.New(s.db.Read).GetItemSystem(ctx, itemID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load item %s: %w", itemID, err)
	}
	if live.DeletedAt.Valid {
		return nil
	}
	unlock, ok := s.tryLockScan(live.LibraryID)
	if !ok {
		return jobs.RetryLater(relinkRetry, fmt.Sprintf("a scan of library %d holds the scan lock", live.LibraryID))
	}
	defer unlock()
	_, merged, err := s.relinkTwin(ctx, live.ID)
	if err != nil {
		return err
	}
	if !merged {
		return s.queueCopyMatch(ctx, live.ID, false)
	}
	if s.recheck != nil {
		// The merge is stored; a failed recheck only leaves streams open until the next change.
		if err := s.recheck(ctx); err != nil {
			s.log.WarnContext(ctx, "visibility recheck after a relink failed", "item_id", live.ID, "err", err)
		}
	}
	return nil
}

// cancelRelink is the cancel hook of library.relink (jobs.Type.Cancel): the administration's
// cancel leaves the copy unmerged, so its match runs again without the relink hook and completes
// the title (queueCopyMatch).
func (s *Service) cancelRelink(ctx context.Context, j jobs.Job) error {
	var p itemPayload
	if err := decodePayload(&j, &p); err != nil {
		return err
	}
	return s.queueCopyMatch(ctx, p.ItemID, true)
}

// queueCopyMatch queues metadata.match for a copy whose match stopped after the title for
// library.relink (metadata.WithRelink). noRelink (the payload's noRelink) makes the match skip the
// relink hook: after a failed or cancelled relink it would otherwise hand the title straight back.
// A title that is gone meanwhile ends the match at once.
func (s *Service) queueCopyMatch(ctx context.Context, itemID string, noRelink bool) error {
	payload, err := json.Marshal(struct {
		ItemID   string `json:"itemId"`
		NoRelink bool   `json:"noRelink,omitempty"`
	}{itemID, noRelink})
	if err != nil {
		return fmt.Errorf("encode %s payload: %w", JobMetadataMatch, err)
	}
	m := matchJob(itemID)
	_, err = s.queue.Enqueue(ctx, jobs.NewJob{Type: m.typ, Priority: m.priority, IdempotencyKey: m.key, Payload: payload})
	return err
}

// relinkTwin merges liveID into its most recently deleted twin within relinkGrace, if any; the
// caller holds the scan lock of the library.
func (s *Service) relinkTwin(ctx context.Context, liveID string) (string, bool, error) {
	deletedID, err := systemq.New(s.db.Read).FindDeletedTwin(ctx, systemq.FindDeletedTwinParams{LiveID: liveID, GraceMs: autoGrace})
	if errors.Is(err, sql.ErrNoRows) {
		return liveID, false, nil
	}
	if err != nil {
		return liveID, false, fmt.Errorf("find deleted twin of %s: %w", liveID, err)
	}
	if _, err := s.relink(ctx, deletedID, liveID, autoGrace); err != nil {
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
// events. The copy's curated state comes along (relinker.carry): locked provider ids, an age
// rating never milder than the copy's (its locked one wins), the administrator's images, local
// images, chapters and segments. The emptied nodes of the copy are deleted for good, the copy last;
// a deleted child of the twin without counterpart stays deleted. Then every other soft-deleted twin
// of the survivor (an older generation of the title) folds into it the same way, user data only,
// so a later match of the survivor finds no older twin. The triggers recompute availability and
// ratings. The twin gets a metadata.match, which describes the episodes that moved in. grace is the
// freshness rule of the twins (autoGrace or anyAge).
func (s *Service) relink(ctx context.Context, deletedID, liveID string, grace sql.NullInt64) (RelinkResult, error) {
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
	if !twins(dead, live, grace) {
		return RelinkResult{}, fmt.Errorf("%w: %s and %s", ErrNotTwins, deletedID, liveID)
	}
	r := &relinker{q: q, now: now.UnixMilli(), copy: true}
	if err := r.restore(ctx, deletedID); err != nil {
		return RelinkResult{}, err
	}
	if err := r.merge(ctx, deletedID, liveID, live.Kind); err != nil {
		return RelinkResult{}, err
	}
	folded, err := r.foldOlderTwins(ctx, dead, live)
	if err != nil {
		return RelinkResult{}, err
	}
	if err := enqueueTx(ctx, q, now, matchJob(deletedID)); err != nil {
		return RelinkResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return RelinkResult{}, fmt.Errorf("commit relink: %w", err)
	}
	res := RelinkResult{KeptID: deletedID, RemovedID: liveID, Merged: r.merged, Moved: r.moved, Folded: folded}
	s.log.InfoContext(ctx, "deleted title relinked", "library_id", live.LibraryID, "kind", live.Kind,
		"kept_id", deletedID, "removed_id", liveID, "merged", res.Merged, "moved", res.Moved, "folded", res.Folded)
	return res, nil
}

// twins reports whether dead is a soft-deleted film or series and live its live copy; with a valid
// grace, live was created after dead's deletion less grace (FindDeletedTwin).
func twins(dead, live systemq.Item, grace sql.NullInt64) bool {
	if dead.ID == live.ID || !dead.DeletedAt.Valid || live.DeletedAt.Valid || dead.LibraryID != live.LibraryID ||
		dead.Kind != live.Kind || (live.Kind != "movie" && live.Kind != "series") ||
		(grace.Valid && live.CreatedAt <= dead.DeletedAt.Int64-grace.Int64) {
		return false
	}
	shared := (dead.TmdbID.Valid && dead.TmdbID == live.TmdbID) || (dead.TvdbID.Valid && dead.TvdbID == live.TvdbID) ||
		(dead.ImdbID.Valid && dead.ImdbID == live.ImdbID)
	ref := itemRef{id: dead.ID, tmdb: dead.TmdbID.Int64, tvdb: dead.TvdbID.Int64, imdb: dead.ImdbID.String}
	return shared && ref.fits(titleInfo{tmdb: live.TmdbID.Int64, tvdb: live.TvdbID.Int64, imdb: live.ImdbID.String})
}

// relinker writes one relink in its transaction.
type relinker struct {
	q   *systemq.Queries
	now int64
	// copy is set while the live copy merges: its curated state, chapters and segments come along
	// (carry). An older twin that folds in brings its user data only.
	copy          bool
	merged, moved int
}

// foldOlderTwins merges every other soft-deleted twin of the survivor dead (restored by now) into
// it: same library and kind, a shared provider id of the survivor or its copy live, none
// contradicting. It returns their number.
func (r *relinker) foldOlderTwins(ctx context.Context, dead, live systemq.Item) (int, error) {
	pick := func(a, b sql.NullInt64) sql.NullInt64 {
		if a.Valid {
			return a
		}
		return b
	}
	imdb := dead.ImdbID
	if !imdb.Valid {
		imdb = live.ImdbID
	}
	ids, err := r.q.ListRelinkChainTwins(ctx, systemq.ListRelinkChainTwinsParams{
		LibraryID: dead.LibraryID, Kind: dead.Kind,
		TmdbID: pick(dead.TmdbID, live.TmdbID), TvdbID: pick(dead.TvdbID, live.TvdbID), ImdbID: imdb,
	})
	if err != nil {
		return 0, fmt.Errorf("relink: list older twins of %s: %w", dead.ID, err)
	}
	fold := &relinker{q: r.q, now: r.now}
	for _, id := range ids {
		if err := fold.merge(ctx, dead.ID, id, dead.Kind); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

func (r *relinker) restore(ctx context.Context, id string) error {
	if err := r.q.RestoreItem(ctx, systemq.RestoreItemParams{Now: r.now, ID: id}); err != nil {
		return fmt.Errorf("restore item %s: %w", id, err)
	}
	return nil
}

// merge moves the files, the user data and the children of the copy's node source (of kind
// kind) into the twin's node target, then deletes source. target is restored by the caller when it
// should be.
func (r *relinker) merge(ctx context.Context, target, source, kind string) error {
	if r.copy {
		if err := r.carry(ctx, target, source, kind); err != nil {
			return err
		}
	}
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
		if err := r.merge(ctx, counterpart, k.ID, k.Kind); err != nil {
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
		{"dead downloads", func() error {
			return q.RelinkDropDeadDownloads(ctx, systemq.RelinkDropDeadDownloadsParams{TargetID: target, SourceID: source})
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

// carriedLocks are the locked fields whose values carry moves from the copy to the survivor; other
// locks stay with the copy.
var carriedLocks = map[string]bool{
	"tmdb_id": true, "tvdb_id": true, "imdb_id": true, "own_rating": true, "poster": true, "backdrop": true, "logo": true,
}

// carry moves the curated state of the copy's node source to the survivor's node target before
// source is deleted:
//   - locked fields the survivor lacks (carriedLocks) with their values: provider ids and the image
//     kinds whose choice moves below;
//   - the age rating (decision 21): the merged title is never milder than the copy, unless the
//     administrator locked the copy's rating. A locked rating of the copy wins with its source and
//     its lock, even over a locked rating of the survivor. Otherwise the stricter of the two (the
//     higher FSK) wins with its source, even over a locked rating of the survivor; that lock stays
//     and now holds the stricter value, so the next match cannot lower it again. A copy without a
//     rating brings no information and leaves the survivor's alone;
//   - the administrator's image choices (item_images chosen = 1) over the survivor's automatic
//     ones, and image choices and local images of a kind the survivor has none of;
//   - for a film or an episode, whose media file moved: the copy's chapters replace the
//     survivor's, and its segments replace the survivor's detected ones (manual ones stay). An
//     episode without segments sets its season back to pending, so the detection runs again.
func (r *relinker) carry(ctx context.Context, target, source, kind string) error {
	t, err := r.q.GetItemSystem(ctx, target)
	if err != nil {
		return fmt.Errorf("relink: load %s: %w", target, err)
	}
	src, err := r.q.GetItemSystem(ctx, source)
	if err != nil {
		return fmt.Errorf("relink: load %s: %w", source, err)
	}
	var tLocks, sLocks []string
	_ = json.Unmarshal([]byte(t.LockedFields), &tLocks) // a broken list counts as empty
	_ = json.Unmarshal([]byte(src.LockedFields), &sLocks)
	p := systemq.RelinkSetCuratedParams{
		TmdbID: t.TmdbID, TvdbID: t.TvdbID, ImdbID: t.ImdbID, OwnRating: t.OwnRating, RatingSource: t.RatingSource,
		LockedFields: t.LockedFields, Now: r.now, ID: target,
	}
	locks := slices.Clone(tLocks)
	for _, f := range sLocks {
		if !carriedLocks[f] || slices.Contains(locks, f) {
			continue
		}
		locks = append(locks, f)
		switch f {
		case "tmdb_id":
			p.TmdbID = src.TmdbID
		case "tvdb_id":
			p.TvdbID = src.TvdbID
		case "imdb_id":
			p.ImdbID = src.ImdbID
		}
	}
	if slices.Contains(sLocks, "own_rating") ||
		(src.OwnRating.Valid && (!p.OwnRating.Valid || p.OwnRating.Int64 < src.OwnRating.Int64)) {
		p.OwnRating, p.RatingSource = src.OwnRating, src.RatingSource
	}
	if len(locks) > len(tLocks) {
		raw, err := json.Marshal(locks)
		if err != nil {
			return fmt.Errorf("relink: locked fields of %s: %w", target, err)
		}
		p.LockedFields = string(raw)
	}
	q := r.q
	steps := []struct {
		what string
		run  func() error
	}{
		{"curated fields", func() error { return q.RelinkSetCurated(ctx, p) }},
		{"unchosen images", func() error {
			return q.RelinkDropUnchosenImages(ctx, systemq.RelinkDropUnchosenImagesParams{TargetID: target, SourceID: source})
		}},
		{"cached images", func() error {
			return q.RelinkDropCachedImages(ctx, systemq.RelinkDropCachedImagesParams{TargetID: target, SourceID: source})
		}},
		{"move cached images", func() error {
			return q.RelinkMoveCachedImages(ctx, systemq.RelinkMoveCachedImagesParams{TargetID: target, SourceID: source})
		}},
		{"images", func() error {
			return q.RelinkMoveImages(ctx, systemq.RelinkMoveImagesParams{TargetID: target, SourceID: source})
		}},
		{"local images", func() error {
			return q.RelinkMoveLocalImages(ctx, systemq.RelinkMoveLocalImagesParams{TargetID: target, SourceID: source})
		}},
		{"chapters", func() error {
			if kind != "movie" && kind != "episode" {
				return nil
			}
			if err := q.RelinkReplaceChapters(ctx, systemq.RelinkReplaceChaptersParams{TargetID: target, SourceID: source}); err != nil {
				return err
			}
			return q.RelinkMoveChapters(ctx, systemq.RelinkMoveChaptersParams{TargetID: target, SourceID: source})
		}},
		{"segments", func() error { return r.carrySegments(ctx, target, source, kind) }},
	}
	for _, step := range steps {
		if err := step.run(); err != nil {
			return fmt.Errorf("relink %s of %s: %w", step.what, source, err)
		}
	}
	return nil
}

// carrySegments moves the segments of the copy's film or episode source to target, replacing the
// detected ones of target; an episode without segments sets the season of target to pending. Other
// kinds have no segments.
func (r *relinker) carrySegments(ctx context.Context, target, source, kind string) error {
	if kind != "movie" && kind != "episode" {
		return nil
	}
	n, err := r.q.RelinkCountSegments(ctx, source)
	if err != nil {
		return err
	}
	if n == 0 {
		if kind != "episode" {
			return nil
		}
		return r.q.RelinkSeasonSegmentsPending(ctx, systemq.RelinkSeasonSegmentsPendingParams{Now: r.now, EpisodeID: target})
	}
	if err := r.q.RelinkDropDetectedSegments(ctx, target); err != nil {
		return err
	}
	return r.q.RelinkMoveSegments(ctx, systemq.RelinkMoveSegmentsParams{TargetID: target, SourceID: source})
}
