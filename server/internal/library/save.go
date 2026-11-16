package library

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// Saving the form «Bibliothek bearbeiten» in one go (saveLibrary, concept 24 4.3). The folder
// steps are the building blocks of RemoveRoot and UpdateRoot (removeRootRowsTx,
// changeRootContentTx, changeRootPathTx, writeRootProviders, insertRoot), here run in one shared
// transaction together with the settings of the library, which package admin writes (AN-2). The
// order inside the transaction is fixed: remove, other content, other path, new folders -- a new
// folder may take the path of one removed in the same request. Watchers, scan state, queue
// clean-up and notifications follow the commit.

// revision derives the state the form edits from the stored rows: the library's name, kind,
// default rating and trickplay setting, and per folder its id, path, content, providers and own
// languages and images. A scan changes none of them, so a form stays valid while a scan runs; any
// change through createLibrary, updateLibrary, the folder operations or saveLibrary changes it. It
// needs no column of its own (stage 3 has no migration): a hash over the rows is enough to tell
// "the same as loaded" from "changed since".
func revision(l appq.Library, roots []appq.LibraryRoot) string {
	type rootState struct {
		ID        int64
		Path      string
		Content   string
		Providers string
		TextLangs sql.NullString
		Images    sql.NullString
	}
	state := struct {
		ID            int64
		Name          string
		Kind          string
		DefaultRating sql.NullInt64
		Trickplay     [5]any
		Roots         []rootState
	}{
		ID: l.ID, Name: l.Name, Kind: l.Kind, DefaultRating: l.DefaultRating,
		Trickplay: [5]any{l.TrickplayMovies, l.TrickplayMoviesKeep, l.TrickplaySeries, l.TrickplaySeriesKeep, l.TrickplaySeriesScope},
		Roots:     make([]rootState, 0, len(roots)),
	}
	for _, r := range roots { // ListLibraryRoots orders by id
		state.Roots = append(state.Roots, rootState{ID: r.ID, Path: r.Path, Content: r.Content, Providers: r.Providers, TextLangs: r.TextLangs, Images: r.Images})
	}
	raw, _ := json.Marshal(state) // plain values only; cannot fail
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:12])
}

// savePlan is a checked LibrarySave: what to remove, what changes on the kept folders and what is
// new, each with the index of the request for field errors.
type savePlan struct {
	removals []appq.LibraryRoot
	kept     []keptRoot
	added    []addedRoot
}

// keptRoot is a stored folder the request keeps, with its new state.
type keptRoot struct {
	index     int
	stored    appq.LibraryRoot
	content   string
	old       []string
	providers []string
	moved     *preparedRoot // nil = the same path
	meta      model.RootMetadata
	metaSet   bool // the own languages or images differ from the stored ones
}

func (k keptRoot) contentChanged() bool { return k.content != k.stored.Content }

// addedRoot is a new folder of the request.
type addedRoot struct {
	index int
	root  preparedRoot
}

// heavy reports whether the plan removes a folder or reads one afresh: those steps take the scan
// lock, as RemoveRoot and UpdateRoot do.
func (p savePlan) heavy() bool {
	if len(p.removals) > 0 {
		return true
	}
	for _, k := range p.kept {
		if k.contentChanged() || k.moved != nil {
			return true
		}
	}
	return false
}

// scans reports whether the plan needs a scan: a new folder, or one with another path or content.
func (p savePlan) scans() bool {
	if len(p.added) > 0 {
		return true
	}
	for _, k := range p.kept {
		if k.contentChanged() || k.moved != nil {
			return true
		}
	}
	return false
}

// Save stores the form «Bibliothek bearbeiten» (saveLibrary, concept 24 4.3) in one transaction.
// settings writes the library's own settings into the same transaction (package admin: name,
// default rating with AN-2, trickplay); nil writes none. Nothing is written when any step fails.
//
// Errors: model.ErrNotFound, model.ErrStale (req.Revision is not the stored state, checked
// before and inside the transaction), model.ErrRootUnlisted (a stored folder is neither in
// req.Roots nor in req.Remove), *model.ValidationError with the folder's index (roots[1].path,
// removeRoots[0].confirmPath), model.ErrScanRunning (a removal, content or path change while a
// scan holds the lock), and whatever settings returns.
func (s *Service) Save(ctx context.Context, libraryID int64, req model.LibrarySave, settings func(context.Context, *sql.Tx) error) (model.LibrarySaved, error) {
	rq := appq.New(s.db.Read)
	lib, err := rq.GetLibrary(ctx, libraryID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.LibrarySaved{}, model.ErrNotFound
	}
	if err != nil {
		return model.LibrarySaved{}, fmt.Errorf("get library %d: %w", libraryID, err)
	}
	stored, err := rq.ListLibraryRoots(ctx, libraryID)
	if err != nil {
		return model.LibrarySaved{}, fmt.Errorf("list roots of library %d: %w", libraryID, err)
	}
	if revision(lib, stored) != req.Revision {
		return model.LibrarySaved{}, model.ErrStale
	}
	plan, err := s.planSave(ctx, lib, stored, req)
	if err != nil {
		return model.LibrarySaved{}, err
	}
	if plan.heavy() {
		unlock, ok := s.tryLockScan(libraryID)
		if !ok {
			return model.LibrarySaved{}, fmt.Errorf("%w: library %d", model.ErrScanRunning, libraryID)
		}
		defer unlock()
	}

	done, err := s.applySave(ctx, libraryID, req.Revision, plan, settings)
	if err != nil {
		return model.LibrarySaved{}, err
	}
	s.afterSave(ctx, lib, plan, done)

	var out model.Library
	if plan.scans() {
		out, err = s.QueueScan(ctx, libraryID)
	} else {
		out, err = s.Get(ctx, libraryID)
	}
	if err != nil {
		return model.LibrarySaved{}, err
	}
	saved := model.LibrarySaved{Library: out, MetadataChanged: []model.RootTitles{}}
	for _, k := range plan.kept {
		if !k.metaSet {
			continue
		}
		for _, r := range out.Roots {
			if r.ID == k.stored.ID {
				saved.MetadataChanged = append(saved.MetadataChanged, model.RootTitles{RootID: r.ID, HomeTitleCount: r.HomeTitleCount})
			}
		}
	}
	return saved, nil
}

// planSave checks a request against the stored folders without writing anything.
func (s *Service) planSave(ctx context.Context, lib appq.Library, stored []appq.LibraryRoot, req model.LibrarySave) (savePlan, error) {
	var plan savePlan
	var verr *model.ValidationError
	byID := make(map[int64]appq.LibraryRoot, len(stored))
	ids := make([]int64, 0, len(stored))
	for _, r := range stored {
		byID[r.ID] = r
		ids = append(ids, r.ID)
	}

	// The ids first: every stored folder is named once, kept or removed. An id error ends the
	// check here, because the rest would compare against the wrong folders.
	var idErr *model.ValidationError
	named := make(map[int64]bool, len(stored))
	for j, r := range req.Remove {
		name := fmt.Sprintf("removeRoots[%d].", j)
		root, ok := byID[r.ID]
		switch {
		case !ok:
			idErr = idErr.Add(name+"id", "unknown_root")
			continue
		case named[r.ID]:
			idErr = idErr.Add(name+"id", "duplicate")
			continue
		}
		named[r.ID] = true
		switch c := strings.TrimSpace(r.ConfirmPath); {
		case c == "":
			verr = verr.Add(name+"confirmPath", "required")
		case c != root.Path:
			verr = verr.Add(name+"confirmPath", "confirm_mismatch")
		}
		plan.removals = append(plan.removals, root)
	}
	for i, r := range req.Roots {
		if r.ID == 0 {
			continue
		}
		name := fmt.Sprintf("roots[%d].", i)
		switch {
		case byID[r.ID].ID == 0:
			idErr = idErr.Add(name+"id", "unknown_root")
		case named[r.ID]:
			idErr = idErr.Add(name+"id", "duplicate")
		}
		named[r.ID] = true
	}
	if idErr != nil {
		return savePlan{}, idErr
	}
	for _, r := range stored {
		if !named[r.ID] {
			return savePlan{}, fmt.Errorf("%w: root %d", model.ErrRootUnlisted, r.ID)
		}
	}

	// The paths the library holds afterwards: a kept folder's new or stored path. A changed path
	// is compared with these and with the folders of other libraries, never with the stored
	// folders of this library, which the request names itself.
	final := make([]string, len(req.Roots))
	for i, r := range req.Roots {
		p := strings.TrimSpace(r.Path)
		if r.ID != 0 && samePath(p, byID[r.ID].Path) {
			p = byID[r.ID].Path
		}
		final[i] = filepath.Clean(p)
	}
	keptPaths := func(except int) []string {
		var out []string
		for i, r := range req.Roots {
			if r.ID != 0 && i != except {
				out = append(out, final[i])
			}
		}
		return out
	}

	for i, r := range req.Roots {
		if r.ID == 0 {
			continue
		}
		name := fmt.Sprintf("roots[%d].", i)
		root := byID[r.ID]
		k := keptRoot{index: i, stored: root, content: r.Content, old: model.ParseProviders(root.Providers), providers: r.Providers, meta: r.Metadata}
		if k.contentChanged() {
			switch {
			case !model.ValidRootContent(r.Content):
				verr = verr.Add(name+"content", "invalid_value")
			case !model.ContentAllowed(lib.Kind, r.Content):
				verr = verr.Add(name+"content", model.CodeContentNotAllowed)
			}
			switch c := strings.TrimSpace(r.ConfirmPath); {
			case c == "":
				verr = verr.Add(name+"confirmPath", "required")
			case c != root.Path:
				verr = verr.Add(name+"confirmPath", "confirm_mismatch")
			}
		}
		if !model.ValidRootProviders(r.Content, r.Providers) {
			verr = verr.Add(name+"providers", model.CodeProvidersNotAllowed)
		}
		// Compared in the encoding RootMetadataColumns writes, so a stored value in another
		// spelling (key order, spaces) is no change.
		storedMeta, err := model.ParseRootMetadata(root.TextLangs, root.Images)
		if err != nil {
			s.log.WarnContext(ctx, "folder metadata unreadable, replaced by the request", "root_id", root.ID, "err", err)
		}
		oldText, oldImages := model.RootMetadataColumns(storedMeta)
		newText, newImages := model.RootMetadataColumns(r.Metadata)
		k.metaSet = err != nil || newText != oldText || newImages != oldImages
		if k.metaSet {
			for _, f := range s.checkMetadata(r.Content, r.Metadata, name) {
				verr = verr.Add(f.Field, f.Code)
			}
		}
		if !samePath(strings.TrimSpace(r.Path), root.Path) {
			p, err := s.prepareMovedRoot(ctx, root, r.Path, k.contentChanged(), keptPaths(i), ids...)
			var field *model.ValidationError
			switch {
			case errors.As(err, &field):
				verr = verr.Add(name+"path", field.Fields[0].Code)
			case err != nil:
				return savePlan{}, err
			}
			k.moved = p
		}
		plan.kept = append(plan.kept, k)
	}

	earlier := keptPaths(-1)
	for i, r := range req.Roots {
		if r.ID != 0 {
			continue
		}
		p, ok, err := s.prepareRoot(ctx, lib.Kind, r.LibraryRootRequest, fmt.Sprintf("roots[%d].", i), &verr, earlier, ids...)
		if err != nil {
			return savePlan{}, err
		}
		if ok {
			earlier = append(earlier, p.path)
			plan.added = append(plan.added, addedRoot{index: i, root: p})
		}
	}
	if verr != nil {
		return savePlan{}, verr
	}
	return plan, nil
}

// samePath reports whether a path of the request names the stored path of a folder.
func samePath(path, stored string) bool {
	return filepath.IsAbs(path) && filepath.Clean(path) == stored
}

// saveDone is what applySave wrote, for the work after the commit.
type saveDone struct {
	removed   []removedRoot
	content   map[int64][2]int64 // root id: items soft-deleted, media files deleted
	moves     map[int64]rootMove
	providers map[int64]providerChange
	inserted  []appq.LibraryRoot
}

type removedRoot struct {
	root         appq.LibraryRoot
	items, files int64
}

// applySave runs the plan in one transaction, in the order of concept 24 4.3. The revision is
// checked again inside it: another save that committed between the check in Save and this
// transaction makes this one stale instead of overwriting it.
func (s *Service) applySave(ctx context.Context, libraryID int64, rev string, plan savePlan, settings func(context.Context, *sql.Tx) error) (saveDone, error) {
	done := saveDone{content: map[int64][2]int64{}, moves: map[int64]rootMove{}, providers: map[int64]providerChange{}}
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return saveDone{}, err
	}
	defer func() { _ = tx.Rollback() }()
	q := appq.New(tx)
	lib, err := q.GetLibrary(ctx, libraryID)
	if errors.Is(err, sql.ErrNoRows) {
		return saveDone{}, model.ErrNotFound
	}
	if err != nil {
		return saveDone{}, fmt.Errorf("get library %d: %w", libraryID, err)
	}
	roots, err := q.ListLibraryRoots(ctx, libraryID)
	if err != nil {
		return saveDone{}, fmt.Errorf("list roots of library %d: %w", libraryID, err)
	}
	if revision(lib, roots) != rev {
		return saveDone{}, model.ErrStale
	}
	if settings != nil {
		if err := settings(ctx, tx); err != nil {
			return saveDone{}, err
		}
	}

	for _, root := range plan.removals {
		items, files, err := s.removeRootRowsTx(ctx, tx, libraryID, root.ID)
		if err != nil {
			return saveDone{}, err
		}
		done.removed = append(done.removed, removedRoot{root: root, items: items, files: files})
	}
	for _, k := range plan.kept {
		if !k.contentChanged() {
			continue
		}
		items, files, err := s.changeRootContentTx(ctx, tx, k.stored, k.content, k.providers)
		if err != nil {
			return saveDone{}, err
		}
		done.content[k.stored.ID] = [2]int64{items, files}
	}
	// The folders of other libraries may have changed since the plan was checked; this library's
	// own cannot (the revision above).
	others, err := q.ListAllLibraryRoots(ctx)
	if err != nil {
		return saveDone{}, fmt.Errorf("list roots: %w", err)
	}
	otherLibrary := func(r appq.LibraryRoot) bool { return r.LibraryID == libraryID }
	var verr *model.ValidationError
	for _, k := range plan.kept {
		if k.moved == nil {
			continue
		}
		if err := overlapping(others, k.moved.path, otherLibrary); err != nil {
			verr = verr.Add(fmt.Sprintf("roots[%d].path", k.index), model.CodeRootOverlap)
			continue
		}
		m, err := s.changeRootPathTx(ctx, tx, k.stored, *k.moved, k.old, k.providers)
		if err != nil {
			return saveDone{}, err
		}
		done.moves[k.stored.ID] = m
	}
	for _, k := range plan.kept {
		if k.contentChanged() || k.moved != nil {
			continue
		}
		ch, err := s.writeRootProviders(ctx, tx, k.stored, k.old, k.providers)
		if err != nil {
			return saveDone{}, err
		}
		done.providers[k.stored.ID] = ch
	}
	for _, k := range plan.kept {
		if !k.metaSet {
			continue
		}
		textLangs, images := model.RootMetadataColumns(k.meta)
		if err := q.UpdateLibraryRootMetadata(ctx, appq.UpdateLibraryRootMetadataParams{
			TextLangs: textLangs, Images: images, ID: k.stored.ID, LibraryID: libraryID,
		}); err != nil {
			return saveDone{}, fmt.Errorf("update own metadata of root %d: %w", k.stored.ID, err)
		}
	}
	now := s.now().UnixMilli()
	for _, a := range plan.added {
		if err := overlapping(others, a.root.path, otherLibrary); err != nil {
			verr = verr.Add(fmt.Sprintf("roots[%d].path", a.index), model.CodeRootOverlap)
			continue
		}
		row, err := insertRoot(ctx, q, libraryID, a.root, now)
		if err != nil {
			return saveDone{}, fmt.Errorf("insert library root: %w", err)
		}
		done.inserted = append(done.inserted, row)
	}
	if verr != nil {
		return saveDone{}, verr
	}
	if err := tx.Commit(); err != nil {
		return saveDone{}, fmt.Errorf("save library %d: %w", libraryID, err)
	}
	return done, nil
}

// afterSave does what lies outside the database, in the order of the transaction: watchers, the
// scanner's state, the queue and the notifications.
func (s *Service) afterSave(ctx context.Context, lib appq.Library, plan savePlan, done saveDone) {
	for _, r := range done.removed {
		s.afterRootRemoved(ctx, lib, r.root, r.items, r.files)
	}
	for _, k := range plan.kept {
		if c, ok := done.content[k.stored.ID]; ok {
			s.afterRootContentChanged(ctx, lib, k.stored, k.content, c[0], c[1])
		}
		if m, ok := done.moves[k.stored.ID]; ok {
			s.afterRootPathChanged(ctx, lib, k.stored, m, k.old, k.providers)
		}
		if ch, ok := done.providers[k.stored.ID]; ok {
			s.logProviderChange(ctx, k.stored, k.old, k.providers, ch)
		}
	}
	for _, row := range done.inserted {
		s.log.InfoContext(ctx, "library root added", "library_id", lib.ID, "root_id", row.ID, "root", row.Path,
			"content", row.Content, "providers", row.Providers, "fs_type", row.FsType, "watch", row.Watch)
		s.startWatcher(row)
	}
	s.log.InfoContext(ctx, "library saved", "library_id", lib.ID, "removed", len(done.removed),
		"content_changed", len(done.content), "moved", len(done.moves), "added", len(done.inserted))
}
