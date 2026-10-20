package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/secrets"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// fakeMetadata stands in for package metadata: it records the calls and applies the documented
// effects of the manual match and of manual trailers to the database, so the administration view
// can be checked end to end.
type fakeMetadata struct {
	db        *sql.DB
	matched   []string // itemID/provider/providerID
	refreshed []string
	searchErr error
	searches  []string
}

func (m *fakeMetadata) Search(_ context.Context, provider, kind, query string, _ *int) ([]model.MetadataCandidate, error) {
	m.searches = append(m.searches, provider+"/"+kind+"/"+query)
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	year := 1999
	return []model.MetadataCandidate{{Provider: provider, ProviderID: "603", Title: query, Year: &year}}, nil
}

// AssignMatch follows design 3.2: the provider id is set, the other ids are cleared, the
// provider ids are locked and the title is pending again.
func (m *fakeMetadata) AssignMatch(ctx context.Context, itemID, provider, providerID string) error {
	m.matched = append(m.matched, itemID+"/"+provider+"/"+providerID)
	var locked string
	if err := m.db.QueryRowContext(ctx, `SELECT locked_fields FROM items WHERE id = ?`, itemID).Scan(&locked); err != nil {
		return model.ErrNotFound
	}
	var fields []string
	_ = json.Unmarshal([]byte(locked), &fields)
	for _, f := range []string{"tmdb_id", "tvdb_id", "imdb_id"} {
		if !slices.Contains(fields, f) {
			fields = append(fields, f)
		}
	}
	raw, _ := json.Marshal(fields)
	col := map[string]string{"tmdb": "tmdb_id", "tvdb": "tvdb_id"}[provider]
	_, err := m.db.ExecContext(ctx, `UPDATE items SET tmdb_id = NULL, tvdb_id = NULL, imdb_id = NULL, metadata_state = 'pending', locked_fields = ? WHERE id = ?`, string(raw), itemID)
	if err == nil {
		_, err = m.db.ExecContext(ctx, `UPDATE items SET `+col+` = ? WHERE id = ?`, providerID, itemID)
	}
	return err
}

func (m *fakeMetadata) Refresh(_ context.Context, itemID string) error {
	m.refreshed = append(m.refreshed, itemID)
	return nil
}

func (m *fakeMetadata) AddManualTrailer(ctx context.Context, itemID, lang, youtubeID string) (model.Trailer, error) {
	res, err := m.db.ExecContext(ctx, `INSERT INTO trailers (item_id, lang, youtube_id, source, created_at) VALUES (?, ?, ?, 'manual', 1)`, itemID, lang, youtubeID)
	if err != nil {
		return model.Trailer{}, err
	}
	id, _ := res.LastInsertId()
	return model.Trailer{ID: id, Lang: lang, YouTubeID: youtubeID, Source: "manual"}, nil
}

func (m *fakeMetadata) DeleteTrailer(ctx context.Context, itemID string, trailerID int64) error {
	res, err := m.db.ExecContext(ctx, `DELETE FROM trailers WHERE id = ? AND item_id = ?`, trailerID, itemID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (m *fakeMetadata) ListTrailersSystem(ctx context.Context, itemID string) ([]model.Trailer, error) {
	rows, err := m.db.QueryContext(ctx, `SELECT id, lang, youtube_id, source, official FROM trailers WHERE item_id = ? ORDER BY lang, id`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Trailer
	for rows.Next() {
		var t model.Trailer
		if err := rows.Scan(&t.ID, &t.Lang, &t.YouTubeID, &t.Source, &t.Official); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// fakeMaintenance stands in for package library.
type fakeMaintenance struct {
	cancelled []int64
	ignored   []string
	reason    string
}

func (m *fakeMaintenance) CancelScan(_ context.Context, id int64) (model.Library, error) {
	m.cancelled = append(m.cancelled, id)
	if id != 1 {
		return model.Library{}, model.ErrNotFound
	}
	return model.Library{ID: 1, Scan: model.LibraryScan{State: "aborted", LastError: "cancelled"}}, nil
}

func (m *fakeMaintenance) IgnoreFiles(_ context.Context, ids []string, reason string) (files, itemsDeleted int, err error) {
	m.ignored, m.reason = append(m.ignored, ids...), reason
	return len(ids), 1, nil
}

type maintFixture struct {
	f     *vistest.Fixture
	svc   *Service
	meta  *fakeMetadata
	maint *fakeMaintenance
	ctx   context.Context
}

func newMaintFixture(t *testing.T) maintFixture {
	t.Helper()
	f := vistest.New(t)
	f.AddEdgeCases(t)
	f.AddSeries(t)
	log := vistest.Logger()
	ring, err := secrets.Load(t.TempDir(), log)
	if err != nil {
		t.Fatal(err)
	}
	meta, maint := &fakeMetadata{db: f.DB.Write}, &fakeMaintenance{}
	svc := New(f.DB, auth.New(f.DB, log), &fakeLibraries{}, ring, &fakeTMDB{}, func(string) string { return "" }, log).
		WithMetadata(meta).WithLibraryMaintenance(maint)
	return maintFixture{f: f, svc: svc, meta: meta, maint: maint, ctx: context.Background()}
}

func (m maintFixture) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := m.f.DB.Write.ExecContext(m.ctx, query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

func (m maintFixture) issues(t *testing.T, kind string) []model.Issue {
	t.Helper()
	page, err := m.svc.ListIssues(m.ctx, kind, nil, maxIssueLimit, 0)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != len(page.Items) {
		t.Errorf("%s: total %d, %d items on one page", kind, page.Total, len(page.Items))
	}
	return page.Items
}

func issueIDs(list []model.Issue) []string {
	out := []string{}
	for _, is := range list {
		out = append(out, is.ItemID)
	}
	slices.Sort(out)
	return out
}

func sorted(ids ...string) []string {
	slices.Sort(ids)
	return ids
}

// B-9: every issue kind lists exactly the prepared cases.
func TestAdminMaintenanceIssues(t *testing.T) {
	m := newMaintFixture(t)
	f := m.f
	lone, loneExtra := f.AddLoneExtraSeries(t)
	missing := edge(f, func(x vistest.Movie) bool { return x.Missing })
	other := edge(f, func(x vistest.Movie) bool { return x.OtherLibrary })

	// The clean fixture has no derived column out of step with the trigger rules.
	if got, err := m.svc.VerifyRatings(m.ctx); err != nil || len(got) != 0 {
		t.Fatalf("VerifyRatings on the fixture = %+v, %v; want none", got, err)
	}

	m.exec(t, `UPDATE items SET metadata_state = 'failed', metadata_error = 'tmdb_no_result' WHERE id = ?`, f.Unrated().ID)
	m.exec(t, `UPDATE items SET metadata_state = 'pending' WHERE id = ?`, f.Show.E3.ID) // created long ago
	m.exec(t, `UPDATE items SET metadata_state = 'pending', created_at = ? WHERE id = ?`, time.Now().UnixMilli(), f.Show.E1.ID)
	m.exec(t, `DELETE FROM images WHERE item_id = ? AND kind = 'poster' AND size = 'w400'`, f.Rated(0).ID)
	m.exec(t, `UPDATE media_files SET probe_error = 'Invalid data found when processing input', probed_at = 5 WHERE id = ?`, f.Rated(16).FileID)
	const emptySeason = "01J9000000000000000000R0ES"
	m.exec(t, `INSERT INTO items (id, kind, parent_id, library_id, title, sort_title, added_at, created_at, updated_at)
	           VALUES (?, 'season', ?, ?, 'Staffel 9', 'staffel 9', 1, 1, 1)`, emptySeason, f.Show.Series.ID, f.LibraryID())

	if got := issueIDs(m.issues(t, IssueUnmatched)); !slices.Equal(got, sorted(f.Unrated().ID, f.Show.E3.ID)) {
		t.Errorf("unmatched = %v", got)
	}
	if got := m.issues(t, IssueUnmatched); got[0].Detail == "" {
		t.Errorf("unmatched detail empty: %+v", got)
	}
	if got := issueIDs(m.issues(t, IssueNoImages)); !slices.Equal(got, []string{f.Rated(0).ID}) {
		t.Errorf("no_images = %v", got)
	}
	if got := issueIDs(m.issues(t, IssueUnrated)); !slices.Equal(got, []string{f.Unrated().ID}) {
		t.Errorf("unrated = %v", got)
	}
	orphaned := m.issues(t, IssueOrphaned)
	if got := issueIDs(orphaned); !slices.Equal(got, sorted(missing.ID, lone.ID, emptySeason)) {
		t.Errorf("orphaned = %v", got)
	}
	for _, is := range orphaned {
		want := map[string]string{missing.ID: "files_missing", lone.ID: "no_seasons", emptySeason: "no_episodes"}[is.ItemID]
		if is.Detail != want {
			t.Errorf("orphaned %s detail %q, want %q", is.ItemID, is.Detail, want)
		}
	}
	unplayable := m.issues(t, IssueUnplayable)
	if len(unplayable) != 1 || unplayable[0].MediaFileID != f.Rated(16).FileID || unplayable[0].Path != f.Rated(16).RelPath ||
		!strings.Contains(unplayable[0].Detail, "Invalid data") || unplayable[0].ItemKind != "movie" {
		t.Errorf("unplayable = %+v", unplayable)
	}
	// Library filter and paging.
	lib := other.LibraryID
	if page, err := m.svc.ListIssues(m.ctx, IssueUnrated, &lib, 0, 0); err != nil || page.Total != 0 || len(page.Items) != 0 {
		t.Errorf("unrated in library X = %+v, %v", page, err)
	}
	if page, err := m.svc.ListIssues(m.ctx, IssueOrphaned, nil, 1, 1); err != nil || page.Total != 3 || len(page.Items) != 1 {
		t.Errorf("orphaned page 2 = %+v, %v", page, err)
	}
	// An offset near MaxInt is an empty page, not an overflow (review finding 11).
	for _, kind := range []string{IssueRatingMismatch, IssueUnrated} {
		if page, err := m.svc.ListIssues(m.ctx, kind, nil, maxIssueLimit, math.MaxInt); err != nil || len(page.Items) != 0 {
			t.Errorf("%s at offset MaxInt = %+v, %v", kind, page, err)
		}
	}
	for _, bad := range []struct {
		kind          string
		limit, offset int
	}{{"broken", 0, 0}, {IssueUnrated, 201, 0}, {IssueUnrated, 0, -1}} {
		var verr *model.ValidationError
		if _, err := m.svc.ListIssues(m.ctx, bad.kind, nil, bad.limit, bad.offset); !errors.As(err, &verr) {
			t.Errorf("ListIssues(%q, %d, %d) = %v, want a validation error", bad.kind, bad.limit, bad.offset, err)
		}
	}

	// A series that is itself unmatched or unrated stands for its episodes (review finding 14).
	m.exec(t, `UPDATE items SET metadata_state = 'unmatched' WHERE id = ?`, f.Show.Series.ID)
	if got := issueIDs(m.issues(t, IssueUnmatched)); !slices.Equal(got, sorted(f.Unrated().ID, f.Show.Series.ID)) {
		t.Errorf("unmatched with an unmatched series = %v, want the series instead of its episode", got)
	}
	m.exec(t, `UPDATE items SET metadata_state = 'matched' WHERE id = ?`, f.Show.Series.ID)
	m.exec(t, `UPDATE items SET own_rating = NULL WHERE id = ?`, f.Show.Series.ID)
	if got := issueIDs(m.issues(t, IssueUnrated)); !slices.Equal(got, sorted(f.Unrated().ID, f.Show.Series.ID)) {
		t.Errorf("unrated with an unrated series = %v, want the series without its episodes", got)
	}
	m.exec(t, `UPDATE items SET own_rating = 12 WHERE id = ?`, f.Show.Series.ID)

	// rating_mismatch: a derived column written behind the triggers' back.
	m.exec(t, `UPDATE items SET effective_rating = 6 WHERE id = ?`, f.Show.E1.ID)
	mismatch := m.issues(t, IssueRatingMismatch)
	if got := issueIDs(mismatch); !slices.Contains(got, f.Show.E1.ID) || !slices.Contains(got, f.Show.Season.ID) || len(got) != 2 {
		t.Errorf("rating_mismatch = %v, want E1 and its season", got)
	}
	for _, is := range mismatch {
		if is.ItemID == f.Show.E1.ID && is.Detail != "effective_rating 6 != 12" {
			t.Errorf("E1 mismatch detail %q", is.Detail)
		}
	}
	m.exec(t, `UPDATE items SET own_rating = own_rating WHERE id = ?`, f.Show.Series.ID) // the triggers repair the tree
	m.exec(t, `UPDATE items SET available = 1 WHERE id = ?`, missing.ID)
	if got := issueIDs(m.issues(t, IssueRatingMismatch)); !slices.Equal(got, []string{missing.ID}) {
		t.Errorf("rating_mismatch after the repair = %v, want only the falsely available title", got)
	}
	_ = loneExtra
}

// B-9: duplicates by provider id, by title and year, and by content hash.
func TestAdminMaintenanceDuplicates(t *testing.T) {
	m := newMaintFixture(t)
	f := m.f
	m16, m18, m0, m6, m12 := f.Rated(16), f.Rated(18), f.Rated(0), f.Rated(6), f.Rated(12)
	m.exec(t, `UPDATE items SET tmdb_id = (SELECT tmdb_id FROM items WHERE id = ?) WHERE id = ?`, m18.ID, m16.ID)
	m.exec(t, `UPDATE items SET tmdb_id = NULL, sort_title = 'same', year = 2020 WHERE id IN (?, ?)`, m0.ID, m6.ID)
	m.exec(t, `UPDATE media_files SET partial_hash = 'abc', size = 2097152 WHERE id IN (?, ?)`, m12.FileID, f.Unrated().FileID)
	m.exec(t, `UPDATE media_files SET partial_hash = 'solo' WHERE id = ?`, f.Version2FileID)
	// Identical small and empty files (placeholders) are no duplicates (review finding 15).
	m.exec(t, `UPDATE media_files SET partial_hash = 'tiny' WHERE id IN (?, ?)`, m0.FileID, m6.FileID)
	m.exec(t, `UPDATE media_files SET partial_hash = 'empty', size = 0 WHERE id IN (?, ?)`, m16.FileID, m18.FileID)

	groups, err := m.svc.ListDuplicates(m.ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	for _, g := range groups {
		var ids []string
		for _, it := range g.Items {
			ids = append(ids, it.ItemID)
			if len(it.Files) == 0 || it.Files[0].Path == "" || it.LibraryID == 0 {
				t.Errorf("%s group: item %s without files or library: %+v", g.Reason, it.ItemID, it)
			}
			if g.Reason == DuplicateFileHash && len(it.Files) != 1 {
				t.Errorf("hash group lists %d files of %s, want only the shared one", len(it.Files), it.ItemID)
			}
		}
		slices.Sort(ids)
		got[g.Reason] = append(got[g.Reason], strings.Join(ids, ","))
	}
	want := map[string][]string{
		DuplicateProviderID: {strings.Join(sorted(m16.ID, m18.ID), ",")},
		DuplicateTitleYear:  {strings.Join(sorted(m0.ID, m6.ID), ",")},
		DuplicateFileHash:   {strings.Join(sorted(m12.ID, f.Unrated().ID), ",")},
	}
	for reason, w := range want {
		if !slices.Equal(got[reason], w) {
			t.Errorf("%s groups = %v, want %v", reason, got[reason], w)
		}
	}
	other := f.OtherLibraryID()
	if groups, err := m.svc.ListDuplicates(m.ctx, &other); err != nil || len(groups) != 0 {
		t.Errorf("duplicates of library X = %+v, %v", groups, err)
	}
}

// B-9: a merge moves files, trailers, credits and the further progress to the target and deletes
// the source for good.
func TestAdminMaintenanceMerge(t *testing.T) {
	m := newMaintFixture(t)
	f := m.f
	target, source := f.Rated(16), f.Rated(18)
	m.exec(t, `INSERT INTO progress (profile_id, item_id, position_ms, duration_ms, completed, updated_at) VALUES
	           (?, ?, 1000, 7200000, 0, 10), (?, ?, 3000000, 7200000, 0, 20),
	           (?, ?, 5000, 7200000, 1, 30), (?, ?, 6000000, 7200000, 0, 40)`,
		f.P18.ID, target.ID, f.P18.ID, source.ID, f.P16.ID, target.ID, f.P16.ID, source.ID)

	var verr *model.ValidationError
	if _, err := m.svc.MergeItems(m.ctx, target.ID, []string{f.Show.Series.ID}); !errors.As(err, &verr) {
		t.Errorf("merge of another kind: %v, want a validation error", err)
	}
	other := edge(f, func(x vistest.Movie) bool { return x.OtherLibrary })
	if _, err := m.svc.MergeItems(m.ctx, target.ID, []string{other.ID}); !errors.As(err, &verr) {
		t.Errorf("merge across libraries: %v, want a validation error", err)
	}
	if _, err := m.svc.MergeItems(m.ctx, "01J0000000000000000000XXXX", []string{source.ID}); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown target: %v", err)
	}
	if _, err := m.svc.MergeItems(m.ctx, target.ID, []string{target.ID}); !errors.As(err, &verr) {
		t.Errorf("target among the sources: %v", err)
	}

	it, err := m.svc.MergeItems(m.ctx, target.ID, []string{source.ID})
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, mf := range it.MediaFiles {
		files = append(files, mf.ID)
	}
	slices.Sort(files)
	if !slices.Equal(files, sorted(target.FileID, source.FileID)) || len(it.Trailers) != 1 || it.Trailers[0].YouTubeID != "EnTrailer18" {
		t.Errorf("merged item = files %v, trailers %+v", files, it.Trailers)
	}
	var n int
	if err := f.DB.Read.QueryRow(`SELECT count(*) FROM items WHERE id = ?`, source.ID).Scan(&n); err != nil || n != 0 {
		t.Errorf("source still exists (%d, %v)", n, err)
	}
	if err := f.DB.Read.QueryRow(`SELECT count(*) FROM credits WHERE item_id = ? AND role = 'director'`, target.ID).Scan(&n); err != nil || n != 1 {
		t.Errorf("director credit on the target: %d, %v", n, err)
	}
	progress := func(profileID int64) (pos int64, completed int) {
		t.Helper()
		if err := f.DB.Read.QueryRow(`SELECT position_ms, completed FROM progress WHERE profile_id = ? AND item_id = ?`, profileID, target.ID).Scan(&pos, &completed); err != nil {
			t.Fatal(err)
		}
		return pos, completed
	}
	if pos, done := progress(f.P18.ID); pos != 3000000 || done != 0 {
		t.Errorf("P18 progress after the merge = %d (completed %d), want the larger position", pos, done)
	}
	if pos, done := progress(f.P16.ID); pos != 5000 || done != 1 {
		t.Errorf("P16 progress after the merge = %d (completed %d), want the completed row", pos, done)
	}
	if it.ID != target.ID {
		t.Errorf("merge answered %s", it.ID)
	}
}

// M5 review W5: a merge moves the source's downloads to the target, so devices keep their copies
// (O-6); a device that holds both keeps the target's download and gets a tombstone for the
// source's one.
func TestAdminMaintenanceMergeMovesDownloads(t *testing.T) {
	m := newMaintFixture(t)
	f := m.f
	target, source := f.Rated(16), f.Rated(18)
	devA, devB := strings.Repeat("a", 32), strings.Repeat("b", 32)
	m.exec(t, `INSERT INTO devices (id, created_at, last_seen_at) VALUES (?, 1, 1), (?, 1, 1)`, devA, devB)
	const (
		onlySource = "01J9MERGE00000000000000AAA"
		bothSource = "01J9MERGE00000000000000BBB"
		bothTarget = "01J9MERGE00000000000000CCC"
	)
	for _, d := range []struct {
		id, device string
		movie      vistest.Movie
	}{{onlySource, devA, source}, {bothSource, devB, source}, {bothTarget, devB, target}} {
		m.exec(t, `INSERT INTO downloads (id, account_id, profile_id, device_id, item_id, media_file_id, quality, audio_stream,
		           state, estimated_bytes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 'low', 1, 'ready', 1000, 1, 1)`,
			d.id, f.Account.ID, f.P18.ID, d.device, d.movie.ID, d.movie.FileID)
	}
	if _, err := m.svc.MergeItems(m.ctx, target.ID, []string{source.ID}); err != nil {
		t.Fatal(err)
	}
	items := map[string]string{}
	rows, err := f.DB.Read.Query(`SELECT id, item_id FROM downloads`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, item string
		if err := rows.Scan(&id, &item); err != nil {
			t.Fatal(err)
		}
		items[id] = item
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[onlySource] != target.ID || items[bothTarget] != target.ID {
		t.Errorf("downloads after the merge = %v, want %s and %s on the target", items, onlySource, bothTarget)
	}
	var tombstones []string
	trows, err := f.DB.Read.Query(`SELECT id FROM download_tombstones ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer trows.Close()
	for trows.Next() {
		var id string
		if err := trows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		tombstones = append(tombstones, id)
	}
	if err := trows.Err(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(tombstones, []string{bothSource}) {
		t.Errorf("tombstones = %v, want only the duplicate %s", tombstones, bothSource)
	}
}

// B-9: cleanup soft-deletes only orphaned titles and hands files to package library.
func TestAdminMaintenanceCleanup(t *testing.T) {
	m := newMaintFixture(t)
	f := m.f
	lone, _ := f.AddLoneExtraSeries(t)
	missing := edge(f, func(x vistest.Movie) bool { return x.Missing })
	items, files, err := m.svc.CleanupItems(m.ctx, []string{missing.ID, f.Rated(0).ID, lone.ID, f.Show.Season.ID, "01J0000000000000000000XXXX"}, nil)
	if err != nil || items != 2 || files != 0 {
		t.Fatalf("cleanup items = %d, %d, %v; want 2 titles", items, files, err)
	}
	for id, want := range map[string]bool{missing.ID: true, lone.ID: true, f.Rated(0).ID: false, f.Show.Season.ID: false} {
		var deleted sql.NullInt64
		if err := f.DB.Read.QueryRow(`SELECT deleted_at FROM items WHERE id = ?`, id).Scan(&deleted); err != nil {
			t.Fatal(err)
		}
		if deleted.Valid != want {
			t.Errorf("item %s soft-deleted = %v, want %v", id, deleted.Valid, want)
		}
	}
	items, files, err = m.svc.CleanupItems(m.ctx, nil, []string{f.Rated(16).FileID})
	if err != nil || files != 1 || items != 1 || !slices.Equal(m.maint.ignored, []string{f.Rated(16).FileID}) || m.maint.reason != "unplayable" {
		t.Errorf("cleanup files = %d, %d, %v; ignored %v (%s)", items, files, err, m.maint.ignored, m.maint.reason)
	}
	var verr *model.ValidationError
	if _, _, err := m.svc.CleanupItems(m.ctx, nil, nil); !errors.As(err, &verr) {
		t.Errorf("empty cleanup: %v", err)
	}
}

// B-9/AN-1: the mass rating has the semantics of SetItemRating per title.
func TestAdminMassRating(t *testing.T) {
	m := newMaintFixture(t)
	f := m.f
	m.svc.WithQueue(jobs.NewQueue(f.DB.Write, nil))
	twelve := 12
	ids := []string{f.Unrated().ID, f.Show.E3.ID, "01J0000000000000000000XXXX", f.Unrated().ID}
	n, err := m.svc.SetItemsRating(m.ctx, ids, &twelve)
	if err != nil || n != 2 {
		t.Fatalf("SetItemsRating = %d, %v; want 2", n, err)
	}
	for _, id := range []string{f.Unrated().ID, f.Show.E3.ID} {
		locked, source := f.ItemFlags(t, id)
		if source != "manual" || !strings.Contains(locked, "own_rating") {
			t.Errorf("%s: locked %s, source %s", id, locked, source)
		}
	}
	var eff sql.NullInt64
	if err := f.DB.Read.QueryRow(`SELECT effective_rating FROM items WHERE id = ?`, f.Unrated().ID).Scan(&eff); err != nil || eff.Int64 != 12 {
		t.Errorf("effective rating of the formerly unrated movie = %v, %v", eff, err)
	}
	if n, err := m.svc.SetItemsRating(m.ctx, []string{f.Unrated().ID}, nil); err != nil || n != 1 {
		t.Fatalf("clear = %d, %v", n, err)
	}
	var jobsQueued int
	if err := f.DB.Read.QueryRow(`SELECT count(*) FROM jobs WHERE type = 'metadata.match'`).Scan(&jobsQueued); err != nil || jobsQueued != 1 {
		t.Errorf("metadata.match jobs after clearing = %d, %v", jobsQueued, err)
	}
	var verr *model.ValidationError
	seven := 7
	if _, err := m.svc.SetItemsRating(m.ctx, ids, &seven); !errors.As(err, &verr) {
		t.Errorf("rating 7: %v", err)
	}
	if _, err := m.svc.SetItemsRating(m.ctx, nil, &twelve); !errors.As(err, &verr) {
		t.Errorf("no ids: %v", err)
	}
	// More than one transaction of 500: unknown ids around a known one.
	many := make([]string, 0, 1201)
	for i := range 1200 {
		many = append(many, "01J0000000000000000000"+string(rune('A'+i%26))+string(rune('A'+i/26%26))+"ZZ")
	}
	many = append(many, f.Rated(0).ID)
	if n, err := m.svc.SetItemsRating(m.ctx, many, &twelve); err != nil || n != 1 {
		t.Errorf("large batch = %d, %v", n, err)
	}
}

// B-6: the manual match validates the request, delegates to package metadata and answers with
// the administration view, whose provider ids are now locked.
func TestSetItemMatchLocksFields(t *testing.T) {
	m := newMaintFixture(t)
	f := m.f
	film := f.Rated(6)
	it, err := m.svc.SetItemMatch(m.ctx, film.ID, "tmdb", "603")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(m.meta.matched, []string{film.ID + "/tmdb/603"}) {
		t.Errorf("metadata calls = %v", m.meta.matched)
	}
	if it.TMDBID == nil || *it.TMDBID != 603 || it.IMDBID != "" || it.MetadataState != "pending" {
		t.Errorf("view after the match = %+v", it)
	}
	for _, field := range []string{"tmdb_id", "tvdb_id", "imdb_id"} {
		if !slices.Contains(it.LockedFields, field) {
			t.Errorf("locked fields %v lack %s", it.LockedFields, field)
		}
	}
	if len(it.MediaFiles) != 1 || it.MediaFiles[0].Path != film.RelPath || it.MediaFiles[0].RootID != f.RootID() || it.Extras != 2 || len(it.Trailers) != 2 {
		t.Errorf("view of the film: files %+v, extras %d, trailers %d", it.MediaFiles, it.Extras, len(it.Trailers))
	}
	if it, err := m.svc.SetItemMatch(m.ctx, f.Show.Series.ID, "tvdb", "81189"); err != nil || it.TVDBID == nil || *it.TVDBID != 81189 || it.Seasons != 2 || it.Episodes != 4 || it.Extras != 1 {
		t.Errorf("tvdb match of the series = %+v, %v", it, err)
	}
	for _, bad := range []struct{ id, provider, providerID, field string }{
		{f.Show.Season.ID, "tmdb", "1", "provider"}, // only films and series
		{film.ID, "tvdb", "1", "provider"},          // TVDB only for series
		{film.ID, "imdb", "1", "provider"},
		{film.ID, "tmdb", "abc", "providerId"},
		{film.ID, "tmdb", "", "providerId"},
	} {
		var verr *model.ValidationError
		_, err := m.svc.SetItemMatch(m.ctx, bad.id, bad.provider, bad.providerID)
		if !errors.As(err, &verr) || verr.Fields[0].Field != bad.field {
			t.Errorf("SetItemMatch(%s, %s, %q) = %v, want a validation error on %s", bad.id, bad.provider, bad.providerID, err, bad.field)
		}
	}
	if _, err := m.svc.SetItemMatch(m.ctx, "01J0000000000000000000XXXX", "tmdb", "1"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown title: %v", err)
	}
	if len(m.meta.matched) != 2 {
		t.Errorf("invalid requests reached package metadata: %v", m.meta.matched)
	}
	if _, err := m.svc.RefreshItem(m.ctx, film.ID); err != nil || !slices.Equal(m.meta.refreshed, []string{film.ID}) {
		t.Errorf("refresh = %v, %v", err, m.meta.refreshed)
	}
	deleted := edge(f, func(x vistest.Movie) bool { return x.Deleted })
	if _, err := m.svc.GetAdminItem(m.ctx, deleted.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("deleted title: %v", err)
	}
}

func TestAdminTrailers(t *testing.T) {
	m := newMaintFixture(t)
	film := m.f.Rated(0)
	tr, err := m.svc.CreateItemTrailer(m.ctx, film.ID, "de", "", "https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=10")
	if err != nil || tr.YouTubeID != "dQw4w9WgXcQ" || tr.Source != "manual" {
		t.Fatalf("create = %+v, %v", tr, err)
	}
	if err := m.svc.DeleteItemTrailer(m.ctx, film.ID, tr.ID); err != nil {
		t.Errorf("delete: %v", err)
	}
	if err := m.svc.DeleteItemTrailer(m.ctx, film.ID, tr.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("delete twice: %v", err)
	}
	for _, bad := range []struct{ lang, id, url, field string }{
		{"german", "dQw4w9WgXcQ", "", "lang"},
		{"de", "", "", "youtubeId"},
		{"de", "bad id!", "", "youtubeId"},
		{"de", "", "https://vimeo.com/123456789", "url"},
		{"de", "dQw4w9WgXcQ", "https://youtu.be/aaaaaaaaaaa", "youtubeId"},
	} {
		var verr *model.ValidationError
		if _, err := m.svc.CreateItemTrailer(m.ctx, film.ID, bad.lang, bad.id, bad.url); !errors.As(err, &verr) || verr.Fields[0].Field != bad.field {
			t.Errorf("CreateItemTrailer(%q, %q, %q) = %v, want a validation error on %s", bad.lang, bad.id, bad.url, err, bad.field)
		}
	}
	if _, err := m.svc.CreateItemTrailer(m.ctx, "01J0000000000000000000XXXX", "de", "dQw4w9WgXcQ", ""); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("unknown title: %v", err)
	}
}

func TestSearchMetadataAndCancelScan(t *testing.T) {
	m := newMaintFixture(t)
	year := 1999
	got, err := m.svc.SearchMetadata(m.ctx, "tmdb", "movie", " The Matrix ", &year)
	if err != nil || len(got) != 1 || !slices.Equal(m.meta.searches, []string{"tmdb/movie/The Matrix"}) {
		t.Errorf("search = %+v, %v (%v)", got, err, m.meta.searches)
	}
	m.meta.searchErr = model.ErrProviderNotConfigured
	if _, err := m.svc.SearchMetadata(m.ctx, "tvdb", "series", "Show", nil); !errors.Is(err, model.ErrProviderNotConfigured) {
		t.Errorf("unconfigured provider: %v", err)
	}
	var verr *model.ValidationError
	if _, err := m.svc.SearchMetadata(m.ctx, "imdb", "season", " ", &[]int{3000}[0]); !errors.As(err, &verr) || len(verr.Fields) != 4 {
		t.Errorf("invalid search: %v", err)
	}
	if lib, err := m.svc.CancelLibraryScan(m.ctx, 1); err != nil || lib.Scan.State != "aborted" {
		t.Errorf("cancel = %+v, %v", lib, err)
	}
	if _, err := m.svc.CancelLibraryScan(m.ctx, 9); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("cancel unknown: %v", err)
	}
	// Without the collaborators the operations fail loudly instead of pretending success.
	bare := New(m.f.DB, auth.New(m.f.DB, vistest.Logger()), &fakeLibraries{}, nil, &fakeTMDB{}, func(string) string { return "" }, vistest.Logger())
	if _, err := bare.CancelLibraryScan(m.ctx, 1); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("cancel without library maintenance: %v", err)
	}
	if _, err := bare.GetAdminItem(m.ctx, m.f.Rated(0).ID); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("admin item without metadata: %v", err)
	}
}

func edge(f *vistest.Fixture, pick func(vistest.Movie) bool) vistest.Movie {
	for _, m := range f.Movies {
		if pick(m) {
			return m
		}
	}
	panic("admin test: edge case missing from the fixture")
}
