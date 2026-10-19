package admin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Maintenance tools of B-9 (design 4.2): issue lists, the rating check, duplicates, merge and
// cleanup. Everything here reads and writes unfiltered through store/systemq; the answers carry
// ids and paths for the administrator, never an ItemRef.

// Issue kinds of listMaintenanceIssues.
const (
	IssueUnmatched      = "unmatched"
	IssueNoImages       = "no_images"
	IssueUnrated        = "unrated"
	IssueOrphaned       = "orphaned"
	IssueUnplayable     = "unplayable"
	IssueRatingMismatch = "rating_mismatch"
)

const (
	defaultIssueLimit = 50
	maxIssueLimit     = 200
	// pendingAfter: a title still pending after this long counts as unmatched.
	pendingAfter  = 24 * time.Hour
	maxMergeIDs   = 100
	maxCleanupIDs = 5000
	// cleanupReason is the ignored_files reason of files the administrator removes.
	cleanupReason = "unplayable"
)

// ListIssues returns one page of the issues of a kind, optionally of one library, with the total.
// limit 0 means 50; limit is capped at 200.
func (s *Service) ListIssues(ctx context.Context, kind string, libraryID *int64, limit, offset int) (model.IssuePage, error) {
	var verr *model.ValidationError
	if !slices.Contains([]string{IssueUnmatched, IssueNoImages, IssueUnrated, IssueOrphaned, IssueUnplayable, IssueRatingMismatch}, kind) {
		verr = verr.Add("kind", "invalid_value")
	}
	if limit < 0 || limit > maxIssueLimit {
		verr = verr.Add("limit", "invalid_value")
	}
	if offset < 0 {
		verr = verr.Add("offset", "invalid_value")
	}
	if verr != nil {
		return model.IssuePage{}, verr
	}
	if limit == 0 {
		limit = defaultIssueLimit
	}
	lib := sql.NullInt64{}
	if libraryID != nil {
		lib = sql.NullInt64{Int64: *libraryID, Valid: true}
	}
	lim, off := int64(limit), int64(offset)
	page := model.IssuePage{Items: []model.Issue{}}
	issue := func(id, itemKind, title string, year sql.NullInt64, libraryID int64) model.Issue {
		return model.Issue{Kind: kind, ItemID: id, ItemKind: itemKind, Title: title, Year: nullInt(year), LibraryID: libraryID}
	}
	var total int64
	var err error
	switch kind {
	case IssueUnmatched:
		before := s.now().Add(-pendingAfter).UnixMilli()
		if total, err = s.sys.AdminCountUnmatched(ctx, systemq.AdminCountUnmatchedParams{PendingBefore: before, LibraryID: lib}); err != nil {
			break
		}
		var rows []systemq.AdminListUnmatchedRow
		rows, err = s.sys.AdminListUnmatched(ctx, systemq.AdminListUnmatchedParams{PendingBefore: before, LibraryID: lib, LimitCount: lim, OffsetCount: off})
		for _, r := range rows {
			is := issue(r.ID, r.Kind, r.Title, r.Year, r.LibraryID)
			is.Detail = r.MetadataState
			if r.MetadataError.String != "" {
				is.Detail = r.MetadataError.String
			}
			page.Items = append(page.Items, is)
		}
	case IssueNoImages:
		if total, err = s.sys.AdminCountNoImages(ctx, lib); err != nil {
			break
		}
		var rows []systemq.AdminListNoImagesRow
		rows, err = s.sys.AdminListNoImages(ctx, systemq.AdminListNoImagesParams{LibraryID: lib, LimitCount: lim, OffsetCount: off})
		for _, r := range rows {
			page.Items = append(page.Items, issue(r.ID, r.Kind, r.Title, r.Year, r.LibraryID))
		}
	case IssueUnrated:
		if total, err = s.sys.AdminCountUnrated(ctx, lib); err != nil {
			break
		}
		var rows []systemq.AdminListUnratedRow
		rows, err = s.sys.AdminListUnrated(ctx, systemq.AdminListUnratedParams{LibraryID: lib, LimitCount: lim, OffsetCount: off})
		for _, r := range rows {
			page.Items = append(page.Items, issue(r.ID, r.Kind, r.Title, r.Year, r.LibraryID))
		}
	case IssueOrphaned:
		if total, err = s.sys.AdminCountOrphaned(ctx, lib); err != nil {
			break
		}
		var rows []systemq.AdminListOrphanedRow
		rows, err = s.sys.AdminListOrphaned(ctx, systemq.AdminListOrphanedParams{LibraryID: lib, LimitCount: lim, OffsetCount: off})
		for _, r := range rows {
			is := issue(r.ID, r.Kind, r.Title, r.Year, r.LibraryID)
			is.Detail = r.Detail
			page.Items = append(page.Items, is)
		}
	case IssueUnplayable:
		if total, err = s.sys.AdminCountUnplayable(ctx, lib); err != nil {
			break
		}
		var rows []systemq.AdminListUnplayableRow
		rows, err = s.sys.AdminListUnplayable(ctx, systemq.AdminListUnplayableParams{LibraryID: lib, LimitCount: lim, OffsetCount: off})
		for _, r := range rows {
			is := issue(r.ID, r.Kind, r.Title, r.Year, r.LibraryID)
			is.MediaFileID, is.Path, is.Detail = r.MediaFileID, r.RelPath, r.ProbeError.String
			page.Items = append(page.Items, is)
		}
	case IssueRatingMismatch:
		var all []model.Issue
		if all, err = s.VerifyRatings(ctx); err != nil {
			break
		}
		all = slices.DeleteFunc(all, func(i model.Issue) bool { return libraryID != nil && i.LibraryID != *libraryID })
		total = int64(len(all))
		page.Items = append(page.Items, all[min(offset, len(all)):min(offset+limit, len(all))]...)
	}
	if err != nil {
		return model.IssuePage{}, fmt.Errorf("list %s issues: %w", kind, err)
	}
	page.Total = int(total)
	return page, nil
}

// contentKinds are the children that make a container available (migration 00008).
var contentKinds = []string{"season", "episode", "podcast_episode"}

// VerifyRatings recomputes effective_rating (own rating against the parent chain),
// min_child_rating and available of every title by the trigger rules (plan 3.2, migration 00008)
// and returns one rating_mismatch issue per live title whose stored columns differ (rating.verify;
// the nightly job follows in M8). Podcast episodes keep their stored availability (enclosure).
func (s *Service) VerifyRatings(ctx context.Context) ([]model.Issue, error) {
	rows, err := s.sys.AdminListRatingState(ctx)
	if err != nil {
		return nil, fmt.Errorf("list rating state: %w", err)
	}
	byID := make(map[string]*systemq.AdminListRatingStateRow, len(rows))
	children := map[string][]*systemq.AdminListRatingStateRow{}
	for i := range rows {
		r := &rows[i]
		byID[r.ID] = r
		if r.ParentID.Valid {
			children[r.ParentID.String] = append(children[r.ParentID.String], r)
		}
	}

	// effective: own rating against the parent's effective rating (unknown parent = unknown).
	effective := map[string]sql.NullInt64{}
	var eff func(r *systemq.AdminListRatingStateRow, depth int) sql.NullInt64
	eff = func(r *systemq.AdminListRatingStateRow, depth int) sql.NullInt64 {
		if v, ok := effective[r.ID]; ok {
			return v
		}
		v := r.OwnRating
		if p, ok := byID[r.ParentID.String]; r.ParentID.Valid && ok && depth < 8 {
			pe := eff(p, depth+1)
			switch {
			case !pe.Valid:
				v = sql.NullInt64{}
			case !v.Valid || pe.Int64 > v.Int64:
				v = pe
			}
		}
		effective[r.ID] = v
		return v
	}
	// available and min_child_rating: leaves follow their files, containers their live, available
	// content children.
	available := map[string]bool{}
	var avail func(r *systemq.AdminListRatingStateRow, depth int) bool
	avail = func(r *systemq.AdminListRatingStateRow, depth int) bool {
		if v, ok := available[r.ID]; ok {
			return v
		}
		var v bool
		switch r.Kind {
		case "series", "season", "podcast":
			for _, c := range children[r.ID] {
				if depth < 8 && !c.DeletedAt.Valid && slices.Contains(contentKinds, c.Kind) && avail(c, depth+1) {
					v = true
				}
			}
		case "podcast_episode":
			v = r.Available == 1
		default:
			v = r.HasFile
		}
		available[r.ID] = v
		return v
	}
	minChild := func(r *systemq.AdminListRatingStateRow) sql.NullInt64 {
		var out sql.NullInt64
		for _, c := range children[r.ID] {
			if c.DeletedAt.Valid || !slices.Contains(contentKinds, c.Kind) || !avail(c, 1) {
				continue
			}
			v := int64(18)
			if e := eff(c, 1); e.Valid {
				v = e.Int64
			}
			if !out.Valid || v < out.Int64 {
				out = sql.NullInt64{Int64: v, Valid: true}
			}
		}
		return out
	}

	var issues []model.Issue
	for i := range rows {
		r := &rows[i]
		if r.DeletedAt.Valid {
			continue
		}
		var diffs []string
		if e := eff(r, 0); e != r.EffectiveRating {
			diffs = append(diffs, fmt.Sprintf("effective_rating %s != %s", ratingText(r.EffectiveRating), ratingText(e)))
		}
		if m := minChild(r); m != r.MinChildRating {
			diffs = append(diffs, fmt.Sprintf("min_child_rating %s != %s", ratingText(r.MinChildRating), ratingText(m)))
		}
		if a := avail(r, 0); a != (r.Available == 1) {
			diffs = append(diffs, fmt.Sprintf("available %d != %d", r.Available, boolInt(a)))
		}
		if len(diffs) > 0 {
			issues = append(issues, model.Issue{
				Kind: IssueRatingMismatch, ItemID: r.ID, ItemKind: r.Kind, Title: r.Title, Year: nullInt(r.Year),
				LibraryID: r.LibraryID, Detail: strings.Join(diffs, "; "),
			})
		}
	}
	return issues, nil
}

func ratingText(v sql.NullInt64) string {
	if !v.Valid {
		return "NULL"
	}
	return fmt.Sprint(v.Int64)
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Duplicate reasons.
const (
	DuplicateProviderID = "provider_id"
	DuplicateTitleYear  = "title_year"
	DuplicateFileHash   = "file_hash"
)

// ListDuplicates returns the duplicate groups (B-9): films and series of one library with the same
// provider id, with the same title and year (without provider id), and media files with the same
// content hash across titles (each title of such a group lists only the files with that hash).
func (s *Service) ListDuplicates(ctx context.Context, libraryID *int64) ([]model.DuplicateGroup, error) {
	lib := sql.NullInt64{}
	if libraryID != nil {
		lib = sql.NullInt64{Int64: *libraryID, Valid: true}
	}
	out := []model.DuplicateGroup{}

	type key struct {
		library int64
		kind    string
		value   string
	}
	var order []key
	members := map[key][]model.DuplicateItem{}
	add := func(k key, it model.DuplicateItem) {
		if _, ok := members[k]; !ok {
			order = append(order, k)
		}
		members[k] = append(members[k], it)
	}
	provider, err := s.sys.AdminListProviderDuplicates(ctx, lib)
	if err != nil {
		return nil, fmt.Errorf("list provider duplicates: %w", err)
	}
	for _, r := range provider {
		add(key{r.LibraryID, r.Kind, "p:" + r.GroupKey}, model.DuplicateItem{ItemID: r.ID, Kind: r.Kind, Title: r.Title, Year: nullInt(r.Year), LibraryID: r.LibraryID})
	}
	titled, err := s.sys.AdminListTitleYearDuplicates(ctx, lib)
	if err != nil {
		return nil, fmt.Errorf("list title duplicates: %w", err)
	}
	for _, r := range titled {
		add(key{r.LibraryID, r.Kind, fmt.Sprintf("t:%s\x00%d\x00%v", r.SortTitle, r.Year.Int64, r.Year.Valid)},
			model.DuplicateItem{ItemID: r.ID, Kind: r.Kind, Title: r.Title, Year: nullInt(r.Year), LibraryID: r.LibraryID})
	}
	var ids []string
	for _, k := range order {
		for _, it := range members[k] {
			ids = append(ids, it.ItemID)
		}
	}
	files, err := s.mediaFiles(ctx, ids)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{} // a pair that shares two provider ids is listed once
	for _, k := range order {
		group := model.DuplicateGroup{Reason: DuplicateProviderID, Items: members[k]}
		if strings.HasPrefix(k.value, "t:") {
			group.Reason = DuplicateTitleYear
		}
		var set []string
		for i := range group.Items {
			group.Items[i].Files = files[group.Items[i].ItemID]
			if group.Items[i].Files == nil {
				group.Items[i].Files = []model.AdminMediaFile{}
			}
			set = append(set, group.Items[i].ItemID)
		}
		slices.Sort(set)
		if sig := group.Reason + ":" + strings.Join(set, ","); !seen[sig] {
			seen[sig] = true
			out = append(out, group)
		}
	}

	hashes, err := s.sys.AdminListHashDuplicates(ctx, lib)
	if err != nil {
		return nil, fmt.Errorf("list hash duplicates: %w", err)
	}
	var current *model.DuplicateGroup
	var currentHash string
	for _, r := range hashes {
		if current == nil || r.PartialHash.String != currentHash {
			out = append(out, model.DuplicateGroup{Reason: DuplicateFileHash})
			current, currentHash = &out[len(out)-1], r.PartialHash.String
		}
		f := model.AdminMediaFile{ID: r.MediaFileID, RootID: r.RootID, Path: r.RelPath, SizeBytes: r.Size,
			Available: !r.MissingSince.Valid, VersionLabel: r.VersionLabel.String}
		if n := len(current.Items); n > 0 && current.Items[n-1].ItemID == r.ID {
			current.Items[n-1].Files = append(current.Items[n-1].Files, f)
			continue
		}
		current.Items = append(current.Items, model.DuplicateItem{ItemID: r.ID, Kind: r.Kind, Title: r.Title, Year: nullInt(r.Year),
			LibraryID: r.LibraryID, Files: []model.AdminMediaFile{f}})
	}
	return out, nil
}

// MergeItems merges duplicate titles into the target (B-9): media files, children (seasons,
// episodes, extras), trailers and credits of the sources move to the target (rows the target
// already has stay behind), progress merges per profile (completed wins, then the larger
// position), progress events and top picks follow, then the sources are deleted for good. Sources
// must have the kind and the library of the target. One transaction; the triggers recompute
// availability and ratings. The answer is the administration view of the target.
func (s *Service) MergeItems(ctx context.Context, targetID string, sourceIDs []string) (model.AdminItem, error) {
	var verr *model.ValidationError
	switch {
	case len(sourceIDs) == 0:
		verr = verr.Add("sourceIds", "required")
	case len(sourceIDs) > maxMergeIDs:
		verr = verr.Add("sourceIds", "too_long")
	case slices.Contains(sourceIDs, targetID):
		verr = verr.Add("sourceIds", "invalid_value")
	}
	if verr != nil {
		return model.AdminItem{}, verr
	}
	sources := slices.Clone(sourceIDs)
	slices.Sort(sources)
	sources = slices.Compact(sources)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.AdminItem{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	target, err := s.liveItem(ctx, q, targetID)
	if err != nil {
		return model.AdminItem{}, err
	}
	for _, id := range sources {
		src, err := s.liveItem(ctx, q, id)
		if err != nil {
			return model.AdminItem{}, err
		}
		if src.Kind != target.Kind || src.LibraryID != target.LibraryID {
			return model.AdminItem{}, (&model.ValidationError{}).Add("sourceIds", "invalid_value")
		}
	}
	now := s.now().UnixMilli()
	tid := sql.NullString{String: targetID, Valid: true}
	for _, id := range sources {
		sid := sql.NullString{String: id, Valid: true}
		steps := []struct {
			what string
			run  func() error
		}{
			{"media files", func() error {
				return q.AdminMoveMediaFiles(ctx, systemq.AdminMoveMediaFilesParams{TargetID: targetID, SourceID: id, Now: now})
			}},
			{"children", func() error {
				return q.AdminMoveChildren(ctx, systemq.AdminMoveChildrenParams{TargetID: tid, SourceID: sid, Now: now})
			}},
			{"trailers", func() error {
				return q.AdminMoveTrailers(ctx, systemq.AdminMoveTrailersParams{TargetID: targetID, SourceID: id})
			}},
			{"credits", func() error {
				return q.AdminMoveCredits(ctx, systemq.AdminMoveCreditsParams{TargetID: targetID, SourceID: id})
			}},
			{"progress", func() error {
				return q.AdminMergeProgress(ctx, systemq.AdminMergeProgressParams{TargetID: targetID, SourceID: id})
			}},
			{"progress events", func() error {
				return q.AdminMoveProgressEvents(ctx, systemq.AdminMoveProgressEventsParams{TargetID: targetID, SourceID: id})
			}},
			{"top picks", func() error {
				return q.AdminMoveTopPicks(ctx, systemq.AdminMoveTopPicksParams{TargetID: targetID, SourceID: id})
			}},
			{"delete source", func() error { return q.AdminDeleteItem(ctx, id) }},
		}
		for _, step := range steps {
			if err := step.run(); err != nil {
				return model.AdminItem{}, fmt.Errorf("merge %s: %w", step.what, err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return model.AdminItem{}, fmt.Errorf("commit: %w", err)
	}
	s.log.InfoContext(ctx, "items merged", "target_id", targetID, "sources", len(sources))
	return s.GetAdminItem(ctx, targetID)
}

// CleanupItems removes orphaned titles and unplayable files (B-9). A title in itemIDs is
// soft-deleted when it has no present file (films, episodes, extras: none or only missing ones)
// or no content child (series, seasons); others are skipped. The media files in mediaFileIDs are
// removed from the library and recorded as ignored with the reason "unplayable" by package
// library, which also soft-deletes titles left without a file. It returns the removed titles
// (both ways) and files.
func (s *Service) CleanupItems(ctx context.Context, itemIDs, mediaFileIDs []string) (items, files int, err error) {
	var verr *model.ValidationError
	switch {
	case len(itemIDs) == 0 && len(mediaFileIDs) == 0:
		verr = verr.Add("itemIds", "required")
	case len(itemIDs) > maxCleanupIDs:
		verr = verr.Add("itemIds", "too_long")
	case len(mediaFileIDs) > maxCleanupIDs:
		verr = verr.Add("mediaFileIds", "too_long")
	}
	if verr != nil {
		return 0, 0, verr
	}
	if len(mediaFileIDs) > 0 && s.maint == nil {
		return 0, 0, ErrNotConfigured
	}
	if len(itemIDs) > 0 {
		if items, err = s.softDeleteOrphans(ctx, itemIDs); err != nil {
			return 0, 0, err
		}
	}
	if len(mediaFileIDs) > 0 {
		removed, deleted, err := s.maint.IgnoreFiles(ctx, mediaFileIDs, cleanupReason)
		if err != nil {
			return items, 0, fmt.Errorf("ignore files: %w", err)
		}
		files, items = removed, items+deleted
	}
	s.log.InfoContext(ctx, "maintenance cleanup", "items", items, "files", files)
	return items, files, nil
}

// softDeleteOrphans soft-deletes the orphaned titles of ids in one transaction.
func (s *Service) softDeleteOrphans(ctx context.Context, ids []string) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := systemq.New(tx)
	now := s.now().UnixMilli()
	n := 0
	for _, id := range ids {
		st, err := q.AdminCleanupState(ctx, id)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && st.DeletedAt.Valid) {
			continue
		}
		if err != nil {
			return 0, fmt.Errorf("cleanup state: %w", err)
		}
		orphan := st.PresentFiles == 0
		if st.Kind == "series" || st.Kind == "season" || st.Kind == "podcast" {
			orphan = st.ContentChildren == 0
		}
		if !orphan {
			continue
		}
		rows, err := q.AdminSoftDeleteItem(ctx, systemq.AdminSoftDeleteItemParams{ID: id, Now: sql.NullInt64{Int64: now, Valid: true}})
		if err != nil {
			return 0, fmt.Errorf("soft-delete item: %w", err)
		}
		n += int(rows)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return n, nil
}
