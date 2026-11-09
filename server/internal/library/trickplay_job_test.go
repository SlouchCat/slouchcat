package library

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

const trickplayProbe = `{"format":{"duration":7200000000000},"streams":[{"index":0,"type":"video","codec":"h264","width":1920,"height":1080}]}`

// tpEnv is an env with a cache directory, fake trickplay calls that record what they made, and a
// film library of placeholder files that carry a stored probe.
type tpEnv struct {
	*env
	cache, dir string
	lib        int64
	mu         sync.Mutex
	seeks      []time.Duration
	sheets     []string // base names of the sheets written, in order
	frameErr   func(at time.Duration) error
}

func newTPEnv(t *testing.T) *tpEnv {
	t.Helper()
	e := &tpEnv{env: newEnv(t), cache: t.TempDir(), dir: t.TempDir()}
	e.svc = New(e.db, e.queue, transcode.Runner{FFprobePath: ffprobePath()}, discard, WithClock(e.clock.Now), WithCacheDir(e.cache))
	e.svc.diskSpace = func(string) (int64, int64, error) { return 1 << 40, 1 << 39, nil }
	e.svc.trickplayFrameFn = func(_ context.Context, _ string, at time.Duration, height int, _ transcode.Stream) ([]byte, error) {
		e.mu.Lock()
		e.seeks = append(e.seeks, at)
		fail := e.frameErr
		e.mu.Unlock()
		if fail != nil {
			if err := fail(at); err != nil {
				return nil, err
			}
		}
		return transcode.BlackTrickplayTile(height), nil
	}
	e.svc.trickplaySheetFn = func(_ context.Context, tiles [][]byte, height int, out string) error {
		for _, tile := range tiles {
			if len(tile) != transcode.TrickplayTileBytes(height) {
				return errors.New("short tile")
			}
		}
		e.mu.Lock()
		name := filepath.Base(out)
		e.sheets = append(e.sheets, name[len(trickplayTempPrefix):strings.Index(name[len(trickplayTempPrefix):], "-")+len(trickplayTempPrefix)])
		e.mu.Unlock()
		return os.WriteFile(out, []byte("RIFF"+strconv.Itoa(len(tiles))), 0o600)
	}
	req := testRequest("Filme", e.dir)
	ahead := trickplayAheadSetting
	req.Trickplay = &ahead
	lib, err := e.svc.Create(e.ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	e.lib = lib.ID
	return e
}

// trickplayAheadSetting is «Im Voraus» for films and series with «Speichern» and the whole series
// (the default before 1.23.1, now «Aus»): the setting of the trickplay tests that generate on
// import, backfill and start.
var trickplayAheadSetting = model.LibraryTrickplay{
	Movies: model.TrickplaySetting{Mode: model.TrickplayAhead, Keep: true},
	Series: model.TrickplaySetting{Mode: model.TrickplayAhead, Keep: true, Scope: model.TrickplayScopeSeries},
}

// film adds a placeholder film with a stored probe and the given duration and returns its file id.
func (e *tpEnv) film(name string, durationMs int64) string {
	e.t.Helper()
	rel := name + "/" + name + ".mkv"
	writeFile(e.t, e.dir, rel, "placeholder "+name)
	e.scan(e.lib)
	id := e.str(`SELECT id FROM media_files WHERE rel_path = ?`, rel)
	e.exec(`UPDATE media_files SET probe = ?, probed_at = 1, duration_ms = ? WHERE id = ?`, trickplayProbe, durationMs, id)
	return id
}

func (e *tpEnv) setting(movies string, keep bool) {
	e.t.Helper()
	e.exec(`UPDATE libraries SET trickplay_movies = ?, trickplay_movies_keep = ? WHERE id = ?`, movies, boolInt(keep), e.lib)
}

func (e *tpEnv) version(id string) FileVersion {
	e.t.Helper()
	f, err := systemq.New(e.db.Read).GetMediaFileSystem(e.ctx, id)
	if err != nil {
		e.t.Fatal(err)
	}
	return VersionOf(f)
}

// run claims and runs the next trickplay.generate job like a worker of the pool does.
func (e *tpEnv) run() (*jobs.Job, error) {
	e.t.Helper()
	j, err := e.queue.Claim(e.ctx, "trickplay-1", []string{JobTrickplay})
	if err != nil || j == nil {
		e.t.Fatalf("claim: %v, %v", j, err)
	}
	err = e.svc.handleTrickplay(e.ctx, j)
	var later *jobs.RetryLaterError
	switch {
	case errors.As(err, &later):
		if derr := e.queue.Defer(e.ctx, "trickplay-1", j.ID, later.After); derr != nil {
			e.t.Fatal(derr)
		}
	case err != nil:
		if ferr := e.queue.Fail(e.ctx, "trickplay-1", j, err); ferr != nil {
			e.t.Fatal(ferr)
		}
	default:
		if cerr := e.queue.Complete(e.ctx, "trickplay-1", j.ID); cerr != nil {
			e.t.Fatal(cerr)
		}
	}
	return j, err
}

func (e *tpEnv) setFiles(dir, id string) []string {
	e.t.Helper()
	entries, err := os.ReadDir(filepath.Join(e.cache, dir, TrickplaySetName(id, e.version(id))))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		e.t.Fatal(err)
	}
	var out []string
	for _, en := range entries {
		out = append(out, en.Name())
	}
	slices.Sort(out)
	return out
}

func (e *tpEnv) status(id string) (TrickplayManifest, []int, string) {
	e.t.Helper()
	m, sheets, state, ok, err := TrickplayStatus(e.ctx, e.db.Read, e.cache, TrickplayFile{
		ID: id, Version: e.version(id), DurationMs: e.int(`SELECT duration_ms FROM media_files WHERE id = ?`, id),
		Video: &transcode.Stream{Type: "video", Width: 1920, Height: 1080},
	})
	if err != nil || !ok {
		e.t.Fatalf("status: %v %v", ok, err)
	}
	return m, sheets, state
}

func (e *tpEnv) int(query string, args ...any) int64 {
	e.t.Helper()
	var n int64
	if err := e.db.Read.QueryRowContext(e.ctx, query, args...).Scan(&n); err != nil {
		e.t.Fatalf("%s: %v", query, err)
	}
	return n
}

func (e *tpEnv) start(id string, startMs int64, setting model.TrickplaySetting) {
	e.t.Helper()
	if err := QueueTrickplayForPlayback(e.ctx, e.queue, e.cache, TrickplayStart{
		MediaFileID: id, Version: e.version(id), Kind: "movie", StartMs: startMs, Setting: setting,
	}); err != nil {
		e.t.Fatal(err)
	}
}

// A start queues trickplay.generate at priority 9 with the window override and never the pause
// override; the job writes the manifest first, the sheet of the start position before the others
// and the marker last; the state follows; a second start of a complete set queues nothing.
func TestTrickplayStartJob(t *testing.T) {
	t.Parallel()
	e := newTPEnv(t)
	id := e.film("Film (2020)", 2_500_000) // 250 points, 3 sheets
	ahead := trickplayAheadSetting.Movies
	if _, _, state := e.status(id); state != TrickplayQueued {
		t.Errorf("state before the start %q", state)
	}
	e.start(id, 1_250_000, ahead) // point 125: sheet 1
	if prio, win, pause := e.int(`SELECT priority FROM jobs WHERE type = ?`, JobTrickplay), e.int(`SELECT override_window FROM jobs WHERE type = ?`, JobTrickplay),
		e.int(`SELECT override_pause FROM jobs WHERE type = ?`, JobTrickplay); prio != 9 || win != 1 || pause != 0 {
		t.Errorf("start job: priority %d, window override %d, pause override %d", prio, win, pause)
	}
	if max := e.int(`SELECT max_attempts FROM jobs WHERE type = ?`, JobTrickplay); max != trickplayAttempts {
		t.Errorf("max attempts %d", max)
	}
	if _, err := e.run(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(e.sheets, []string{"1", "2", "0"}) {
		t.Errorf("sheet order %v, want 1 2 0", e.sheets)
	}
	if got := e.setFiles(TrickplayDir, id); !slices.Equal(got, []string{"0.webp", "1.webp", "2.webp", "complete", "manifest.json"}) {
		t.Errorf("set files %v", got)
	}
	m, sheets, state := e.status(id)
	if state != TrickplayComplete || !slices.Equal(sheets, []int{0, 1, 2}) || len(m.PointsMs) != 250 || m.Source != TrickplaySourceFallback {
		t.Errorf("status %q %v, %d points, source %q", state, sheets, len(m.PointsMs), m.Source)
	}
	// The two workers of a sheet grab its points in either order; the first 100 grabbed are
	// those of sheet 1 (points 100 to 199), since a sheet ends before the next one starts.
	if len(e.seeks) != 250 {
		t.Errorf("%d points grabbed", len(e.seeks))
	} else if first := e.seeks[:transcode.TrickplayColumns*transcode.TrickplayRows]; slices.Min(first) != 1_000_000*time.Millisecond || slices.Max(first) != 1_990_000*time.Millisecond {
		t.Errorf("first sheet grabbed %v to %v", slices.Min(first), slices.Max(first))
	}
	e.start(id, 0, ahead)
	if n := e.int(`SELECT count(*) FROM jobs WHERE type = ? AND state = 'queued'`, JobTrickplay); n != 0 {
		t.Errorf("a complete set queued %d jobs", n)
	}
	// Off, and kinds without trickplay, queue nothing.
	other := e.film("Zweiter (2021)", 60_000)
	e.start(other, 0, model.TrickplaySetting{Mode: model.TrickplayOff})
	if err := QueueTrickplayForPlayback(e.ctx, e.queue, e.cache, TrickplayStart{MediaFileID: other, Kind: "audiobook", Setting: ahead}); err != nil {
		t.Fatal(err)
	}
	if n := e.int(`SELECT count(*) FROM jobs WHERE type = ? AND state = 'queued'`, JobTrickplay); n != 0 {
		t.Errorf("off or an audiobook queued %d jobs", n)
	}
}

// An import job yields to a waiting start job after its sheet without an attempt, keeps its
// sheets, and finishes later; a start of the same file takes the job over (origin and position).
func TestTrickplayPreemption(t *testing.T) {
	t.Parallel()
	e := newTPEnv(t)
	first := e.film("Erster (2020)", 2_500_000)
	second := e.film("Zweiter (2021)", 50_000)
	if _, err := enqueueTrickplay(e.ctx, e.queue, trickplayPayload{MediaFileID: first, Origin: TrickplayOriginImport}, prioTrickplayImport, false); err != nil {
		t.Fatal(err)
	}
	j, err := e.queue.Claim(e.ctx, "trickplay-1", []string{JobTrickplay})
	if err != nil || j == nil {
		t.Fatal(err)
	}
	// While it runs, a viewer starts the second film.
	e.start(second, 0, trickplayAheadSetting.Movies)
	err = e.svc.handleTrickplay(e.ctx, j)
	var later *jobs.RetryLaterError
	if !errors.As(err, &later) || later.After != 0 {
		t.Fatalf("import job: %v, want RetryLater(0)", err)
	}
	if err := e.queue.Defer(e.ctx, "trickplay-1", j.ID, 0); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(e.sheets, []string{"0"}) {
		t.Errorf("sheets before yielding %v", e.sheets)
	}
	// The start job runs first (priority 9), then the import job makes the two missing sheets.
	if j, err := e.run(); err != nil || !strings.Contains(string(j.Payload), second) {
		t.Fatalf("first claimed %s: %v", j.Payload, err)
	}
	if j, err := e.run(); err != nil || !strings.Contains(string(j.Payload), first) || j.Attempts != 1 {
		t.Fatalf("then %s (attempts %d): %v", j.Payload, j.Attempts, err)
	}
	if !slices.Equal(e.sheets, []string{"0", "0", "1", "2"}) {
		t.Errorf("sheets %v", e.sheets)
	}

	// A start takes a waiting import job over.
	third := e.film("Dritter (2022)", 30_000)
	if _, err := enqueueTrickplay(e.ctx, e.queue, trickplayPayload{MediaFileID: third, Origin: TrickplayOriginImport}, prioTrickplayImport, false); err != nil {
		t.Fatal(err)
	}
	e.start(third, 20_000, trickplayAheadSetting.Movies)
	var p trickplayPayload
	if err := json.Unmarshal([]byte(e.str(`SELECT payload FROM jobs WHERE idempotency_key = ? AND state = 'queued'`, trickplayKey(third))), &p); err != nil {
		t.Fatal(err)
	}
	if p.Origin != TrickplayOriginStart || p.StartMs != 20_000 || e.int(`SELECT priority FROM jobs WHERE idempotency_key = ? AND state = 'queued'`, trickplayKey(third)) != 9 {
		t.Errorf("taken over: %+v", p)
	}
	// A later import does not take a start job back.
	if _, err := enqueueTrickplay(e.ctx, e.queue, trickplayPayload{MediaFileID: third, Origin: TrickplayOriginImport}, prioTrickplayImport, false); err != nil {
		t.Fatal(err)
	}
	if got := e.str(`SELECT payload FROM jobs WHERE idempotency_key = ? AND state = 'queued'`, trickplayKey(third)); !strings.Contains(got, `"origin":"start"`) {
		t.Errorf("payload after an import %s", got)
	}
}

// A paused trickplay.generate stays paused for a start job (no pause override, ADR 0022 unchanged).
func TestTrickplayPauseHolds(t *testing.T) {
	t.Parallel()
	e := newTPEnv(t)
	id := e.film("Film (2020)", 30_000)
	e.exec(`INSERT INTO job_types (type, paused_at, updated_at) VALUES (?, 1, 1)`, JobTrickplay)
	e.start(id, 0, trickplayAheadSetting.Movies)
	if j, err := e.queue.Claim(e.ctx, "trickplay-1", []string{JobTrickplay}); err != nil || j != nil {
		t.Errorf("claimed %v while paused (%v)", j, err)
	}
	if n := e.int(`SELECT count(*) FROM jobs WHERE override_pause = 1`); n != 0 {
		t.Errorf("%d pause overrides", n)
	}
}

// The setting decides (5.2): off and a setting that does not store without an open playback end
// a job without work, a download makes a temporary set for a setting that does not store, an
// import ends after a change from «Im Voraus» to «Beim Start».
func TestTrickplaySettingDecides(t *testing.T) {
	t.Parallel()
	e := newTPEnv(t)
	id := e.film("Film (2020)", 30_000)
	enq := func(origin string) {
		t.Helper()
		if _, err := enqueueTrickplay(e.ctx, e.queue, trickplayPayload{MediaFileID: id, Origin: origin}, 5, true); err != nil {
			t.Fatal(err)
		}
	}
	e.setting(model.TrickplayOnStart, true)
	enq(TrickplayOriginImport)
	if _, err := e.run(); err != nil || len(e.sheets) != 0 {
		t.Errorf("import at on_start: %v, %v", e.sheets, err)
	}
	e.setting(model.TrickplayOnStart, false)
	enq(TrickplayOriginStart)
	if _, err := e.run(); err != nil || len(e.sheets) != 0 {
		t.Errorf("start without keep and without playback: %v, %v", e.sheets, err)
	}
	enq(TrickplayOriginDownload)
	if _, err := e.run(); err != nil {
		t.Fatal(err)
	}
	if got := e.setFiles(TrickplaySessionDir, id); !slices.Contains(got, "complete") || e.setFiles(TrickplayDir, id) != nil {
		t.Errorf("download without keep: session %v, stored %v", got, e.setFiles(TrickplayDir, id))
	}
	// Back to a storing setting: the next run moves the temporary set instead of making it again.
	e.setting(model.TrickplayAhead, true)
	e.sheets = nil
	enq(TrickplayOriginStart)
	if _, err := e.run(); err != nil {
		t.Fatal(err)
	}
	if got := e.setFiles(TrickplayDir, id); !slices.Contains(got, "complete") || len(e.sheets) != 0 || e.setFiles(TrickplaySessionDir, id) != nil {
		t.Errorf("after the change: stored %v, sheets %v", got, e.sheets)
	}
	// Below the free-space floor an import ends, a start writes a temporary set.
	low := e.film("Voll (2021)", 30_000)
	e.svc.diskSpace = func(string) (int64, int64, error) { return 100 << 30, 5 << 30, nil }
	if _, err := enqueueTrickplay(e.ctx, e.queue, trickplayPayload{MediaFileID: low, Origin: TrickplayOriginImport}, 2, false); err != nil {
		t.Fatal(err)
	}
	if _, err := e.run(); err != nil || e.setFiles(TrickplayDir, low) != nil {
		t.Errorf("import below the floor wrote %v (%v)", e.setFiles(TrickplayDir, low), err)
	}
	if _, err := enqueueTrickplay(e.ctx, e.queue, trickplayPayload{MediaFileID: low, Origin: TrickplayOriginDownload}, 5, true); err != nil {
		t.Fatal(err)
	}
	if _, err := e.run(); err != nil || !slices.Contains(e.setFiles(TrickplaySessionDir, low), "complete") {
		t.Errorf("download below the floor: %v (%v)", e.setFiles(TrickplaySessionDir, low), err)
	}
}

// A broken point is filled black; five in a row end the job as failed, the sheets stay; the state
// says failed.
func TestTrickplayBrokenPoints(t *testing.T) {
	t.Parallel()
	e := newTPEnv(t)
	id := e.film("Film (2020)", 1_100_000) // 110 points, 2 sheets
	e.frameErr = func(at time.Duration) error {
		if at == 30*time.Second {
			return transcode.ErrNoFrame
		}
		return nil
	}
	e.start(id, 0, trickplayAheadSetting.Movies)
	if _, err := e.run(); err != nil {
		t.Fatalf("one broken point: %v", err)
	}
	broken := e.film("Kaputt (2021)", 1_100_000)
	e.frameErr = func(at time.Duration) error {
		if at >= 1_000*time.Second {
			return transcode.ErrNoFrame
		}
		return nil
	}
	e.sheets = nil
	e.start(broken, 0, trickplayAheadSetting.Movies)
	if _, err := e.run(); !errors.Is(err, errTrickplayFailures) {
		t.Fatalf("five broken points: %v", err)
	}
	if got := e.setFiles(TrickplayDir, broken); !slices.Equal(got, []string{"0.webp", "manifest.json"}) {
		t.Errorf("files after the failure %v", got)
	}
	if _, sheets, state := e.status(broken); state != TrickplayFailed || !slices.Equal(sheets, []int{0}) {
		t.Errorf("state %q %v", state, sheets)
	}
}

// The import trigger: a file added after a complete scan at «Im Voraus» is queued with priority 2.
func TestTrickplayAfterProbe(t *testing.T) {
	t.Parallel()
	e := newTPEnv(t)
	id := e.film("Film (2020)", 30_000)
	f, err := systemq.New(e.db.Read).GetMediaFileSystem(e.ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	item, err := systemq.New(e.db.Read).GetItemSystem(e.ctx, f.ItemID)
	if err != nil {
		t.Fatal(err)
	}
	e.svc.queueTrickplayAfterProbe(e.ctx, f, item, false)
	if n := e.jobCount(JobTrickplay); n != 0 {
		t.Errorf("a file of the first scan queued %d jobs", n)
	}
	e.svc.queueTrickplayAfterProbe(e.ctx, f, item, true)
	if prio, payload := e.int(`SELECT priority FROM jobs WHERE type = ?`, JobTrickplay), e.str(`SELECT payload FROM jobs WHERE type = ?`, JobTrickplay); prio != 2 || !strings.Contains(payload, `"origin":"import"`) {
		t.Errorf("import job: %d %s", prio, payload)
	}
	e.exec(`DELETE FROM jobs`)
	e.setting(model.TrickplayOnStart, true)
	e.svc.queueTrickplayAfterProbe(e.ctx, f, item, true)
	if n := e.jobCount(JobTrickplay); n != 0 {
		t.Errorf("on_start queued %d import jobs", n)
	}
	// A stored set of an earlier version at «Beim Start» with «Speichern»: an import job would end at
	// once, so the old set goes now and nothing is queued; the next start makes the set.
	old := filepath.Join(e.cache, TrickplayDir, id+"-1-1")
	if err := os.MkdirAll(old, 0o750); err != nil {
		t.Fatal(err)
	}
	e.svc.queueTrickplayAfterProbe(e.ctx, f, item, false)
	if _, err := os.Stat(old); err == nil || e.jobCount(JobTrickplay) != 0 {
		t.Errorf("changed file at on_start: old set stays %v, %d jobs", err == nil, e.jobCount(JobTrickplay))
	}
	// At «Im Voraus» a changed file is queued.
	e.setting(model.TrickplayAhead, true)
	if err := os.MkdirAll(old, 0o750); err != nil {
		t.Fatal(err)
	}
	e.svc.queueTrickplayAfterProbe(e.ctx, f, item, false)
	if n := e.jobCount(JobTrickplay); n != 1 {
		t.Errorf("changed file queued %d jobs", n)
	}
}

// The look-ahead (6.4): order and scope.
func TestTrickplayLookAhead(t *testing.T) {
	t.Parallel()
	all := []trickplayEpisode{
		{itemID: "s0e1", season: 0, episode: 1},
		{itemID: "s1e1", season: 1, episode: 1}, {itemID: "s1e2", season: 1, episode: 2}, {itemID: "s1e3", season: 1, episode: 3},
		{itemID: "s2e1", season: 2, episode: 1}, {itemID: "s2e2", season: 2, episode: 2},
	}
	ids := func(es []trickplayEpisode) []string {
		var out []string
		for _, e := range es {
			out = append(out, e.itemID)
		}
		return out
	}
	for _, c := range []struct {
		start, scope string
		want         []string
	}{
		{"s1e2", model.TrickplayScopeSeries, []string{"s1e3", "s2e1", "s2e2", "s1e1", "s0e1"}},
		{"s1e2", model.TrickplayScopeSeasonRest, []string{"s1e3"}},
		{"s1e3", model.TrickplayScopeSeasonRest, []string{"s2e1", "s2e2"}},
		{"s2e2", model.TrickplayScopeSeasonRest, nil},
		{"s1e3", model.TrickplayScopeNextEpisode, []string{"s2e1"}},
		{"s2e2", model.TrickplayScopeNextEpisode, nil},
		{"s0e1", model.TrickplayScopeSeries, []string{"s1e1", "s1e2", "s1e3", "s2e1", "s2e2"}},
	} {
		if got := ids(trickplayLookAhead(all, c.start, c.scope)); !slices.Equal(got, c.want) {
			t.Errorf("%s %s: %v, want %v", c.start, c.scope, got, c.want)
		}
	}
}

// The look-ahead job queues the other episodes of a series at series_start: the next two at
// priority 3, the rest at 2 throttled; at another setting nothing.
func TestTrickplaySeriesJob(t *testing.T) {
	t.Parallel()
	e := newTPEnv(t)
	dir := t.TempDir()
	show := "Serie (2020)"
	for _, rel := range []string{"Season 01/Serie - S01E01.mkv", "Season 01/Serie - S01E02.mkv", "Season 01/Serie - S01E03.mkv",
		"Season 02/Serie - S02E01.mkv", "Season 02/Serie - S02E02.mkv"} {
		writeFile(t, dir, show+"/"+rel, rel)
	}
	lib := e.createSeries(dir)
	e.scan(lib.ID)
	e.exec(`UPDATE media_files SET probe = ?, probed_at = 1, duration_ms = 60000`, trickplayProbe)
	series := e.str(`SELECT id FROM items WHERE kind = 'series'`)
	ep := func(s, n int) string {
		return e.str(`SELECT ep.id FROM items AS ep JOIN items AS se ON se.id = ep.parent_id WHERE se.parent_id = ? AND ep.kind = 'episode'
		              AND coalesce(ep.season_number, se.season_number) = ? AND ep.episode_number = ?`, series, s, n)
	}
	run := func() {
		t.Helper()
		raw, _ := json.Marshal(trickplaySeriesPayload{SeriesID: series, EpisodeID: ep(1, 1)})
		if err := e.svc.handleTrickplaySeries(e.ctx, &jobs.Job{Type: JobTrickplaySeries, Payload: raw}); err != nil {
			t.Fatal(err)
		}
	}
	run()
	if n := e.jobCount(JobTrickplay); n != 0 {
		t.Errorf("ahead queued %d look-ahead jobs", n)
	}
	e.exec(`UPDATE libraries SET trickplay_series = 'series_start', trickplay_series_scope = 'series' WHERE id = ?`, lib.ID)
	run()
	rows, err := e.db.Read.QueryContext(e.ctx, `SELECT priority, payload FROM jobs WHERE type = ? ORDER BY id`, JobTrickplay)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var prio int
		var payload string
		if err := rows.Scan(&prio, &payload); err != nil {
			t.Fatal(err)
		}
		got = append(got, strconv.Itoa(prio)+strconv.FormatBool(strings.Contains(payload, `"throttled":true`)))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"3false", "3false", "2true", "2true"}) {
		t.Errorf("look-ahead jobs %v", got)
	}
	// The start trigger queues the look-ahead under the key of the series; the newest episode wins.
	setting := model.TrickplaySetting{Mode: model.TrickplaySeriesStart, Scope: model.TrickplayScopeSeries}
	for _, episode := range []string{ep(1, 2), ep(2, 1)} {
		if err := QueueTrickplayForPlayback(e.ctx, e.queue, e.cache, TrickplayStart{MediaFileID: "01ZZZZZZZZZZZZZZZZZZZZZZZZ", Kind: "episode",
			Setting: setting, SeriesID: series, EpisodeID: episode}); err != nil {
			t.Fatal(err)
		}
	}
	if payload := e.str(`SELECT payload FROM jobs WHERE type = ? AND state = 'queued'`, JobTrickplaySeries); !strings.Contains(payload, ep(2, 1)) {
		t.Errorf("look-ahead payload %s", payload)
	}
}

// The backfill queues files at «Im Voraus» without a complete set, and a complete fallback set
// once the file has a keyframe index: the job makes the index set beside the fallback set, which
// serves until the new one replaces it under a new version (new sheet URLs).
func TestTrickplayBackfill(t *testing.T) {
	t.Parallel()
	e := newTPEnv(t)
	done := e.film("Fertig (2020)", 30_000)
	open := e.film("Offen (2021)", 30_000)
	e.start(done, 0, trickplayAheadSetting.Movies)
	if _, err := e.run(); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.handleTrickplayBackfill(e.ctx, &jobs.Job{}); err != nil {
		t.Fatal(err)
	}
	if got := e.str(`SELECT group_concat(idempotency_key) FROM jobs WHERE type = ? AND state = 'queued'`, JobTrickplay); got != trickplayKey(open) {
		t.Errorf("backfill queued %q", got)
	}
	e.exec(`DELETE FROM jobs WHERE state = 'queued'`)
	e.exec(`UPDATE media_files SET keyframes_source = 'cues', keyframes = ? WHERE id = ?`, EncodeKeyframes(ms(0, 10_000, 20_000)), done)
	if err := e.svc.handleTrickplayBackfill(e.ctx, &jobs.Job{}); err != nil {
		t.Fatal(err)
	}
	before, _, _ := e.status(done)
	if n := e.int(`SELECT count(*) FROM jobs WHERE idempotency_key = ? AND state = 'queued'`, trickplayKey(done)); n != 1 ||
		!slices.Contains(e.setFiles(TrickplayDir, done), "complete") {
		t.Fatalf("fallback set: %d jobs, files %v", n, e.setFiles(TrickplayDir, done))
	}
	// The backfill queues the file without a set again, at the same priority and (fake clock) the
	// same time; the claim order of the two is then the order of the media file ids, ULIDs of one
	// millisecond whose shared entropy tests in parallel reset. Only the job of done is under test.
	if n := e.int(`SELECT count(*) FROM jobs WHERE idempotency_key = ? AND state = 'queued'`, trickplayKey(open)); n != 1 {
		t.Fatalf("open file: %d jobs", n)
	}
	e.exec(`DELETE FROM jobs WHERE idempotency_key = ? AND state = 'queued'`, trickplayKey(open))
	// While the job makes the index set, the fallback set serves.
	e.svc.trickplaySheetFn = func(_ context.Context, _ [][]byte, _ int, out string) error {
		if m, _, state := e.status(done); state != TrickplayComplete || m.Source != TrickplaySourceFallback {
			t.Errorf("during the rebuild: %q, source %q", state, m.Source)
		}
		if !strings.Contains(out, TrickplaySetName(done, e.version(done))+trickplayNextSuffix+"/") {
			t.Errorf("rebuild writes %s", out)
		}
		return os.WriteFile(out, []byte("RIFF"), 0o600)
	}
	if _, err := e.run(); err != nil {
		t.Fatal(err)
	}
	after, sheets, state := e.status(done)
	if state != TrickplayComplete || after.Source != TrickplaySourceIndex || after.Version == before.Version || !slices.Equal(sheets, []int{0}) {
		t.Errorf("after the rebuild: %q %v, source %q, version %q (was %q)", state, sheets, after.Source, after.Version, before.Version)
	}
	if _, err := os.Stat(filepath.Join(e.cache, TrickplayDir, TrickplaySetName(done, e.version(done))+trickplayNextSuffix)); err == nil {
		t.Error("the folder of the rebuild stays")
	}
	if v, ok := TrickplaySetVersionOf(e.cache, done, e.version(done)); !ok || v != after.Version {
		t.Errorf("set version %q %v, want %q", v, ok, after.Version)
	}
}

// The maintenance (6.7): other versions, gone files and sets of a setting that does not store go;
// sets of the current version stay and are counted; old temporary sheets go; temporary sets of a
// file without playback for 10 minutes go.
func TestTrickplaySweep(t *testing.T) {
	t.Parallel()
	e := newTPEnv(t)
	id := e.film("Film (2020)", 30_000)
	e.start(id, 0, trickplayAheadSetting.Movies)
	if _, err := e.run(); err != nil {
		t.Fatal(err)
	}
	set := filepath.Join(e.cache, TrickplayDir, TrickplaySetName(id, e.version(id)))
	mk := func(path string, age time.Duration) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		at := e.clock.Now().Add(-age)
		if err := os.Chtimes(path, at, at); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(filepath.Dir(path), at, at); err != nil {
			t.Fatal(err)
		}
	}
	mk(filepath.Join(set, trickplayTempPrefix+"5-1.webp"), 7*time.Hour)
	mk(filepath.Join(set, trickplayTempPrefix+"6-1.webp"), time.Hour)
	old := filepath.Join(e.cache, TrickplayDir, id+"-1-1")
	gone := filepath.Join(e.cache, TrickplayDir, "01ZZZZZZZZZZZZZZZZZZZZZZZZ-1-1")
	session := filepath.Join(e.cache, TrickplaySessionDir, TrickplaySetName(id, e.version(id)))
	mk(filepath.Join(old, "0.webp"), 0)
	mk(filepath.Join(gone, "0.webp"), 0)
	mk(filepath.Join(session, "0.webp"), 20*time.Minute)
	if err := e.svc.sweepTrickplay(e.ctx); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]bool{
		set: true, old: false, gone: false, session: false,
		filepath.Join(set, trickplayTempPrefix+"5-1.webp"): false, filepath.Join(set, trickplayTempPrefix+"6-1.webp"): true,
	} {
		if _, err := os.Stat(path); (err == nil) != want {
			t.Errorf("%s exists %v, want %v", path, err == nil, want)
		}
	}
	movies, series, free := e.svc.TrickplayUsage(e.lib)
	if movies == 0 || series != 0 || free != 1<<39 {
		t.Errorf("usage %d %d %d", movies, series, free)
	}
	// A setting that does not store: the stored set goes at the next maintenance.
	e.setting(model.TrickplayOff, true)
	if err := e.svc.sweepTrickplay(e.ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(set); err == nil {
		t.Error("stored set of a setting off stays")
	}
}

// trickplay.generate with the real ffmpeg on a copy of the generated H.264 test film (MP4, keyframe
// index from the sample tables): one sheet, a WebP of 10 tiles of 320 px in width.
func TestTrickplayJobRealFFmpeg(t *testing.T) {
	t.Parallel()
	if ffprobePath() == "" || ffmpegPath() == "" {
		t.Fatal("ffmpeg and ffprobe are required for the trickplay tests (install ffmpeg)")
	}
	e := newTPEnv(t)
	runner := transcode.Runner{FFprobePath: ffprobePath(), FFmpegPath: ffmpegPath()}
	e.svc = New(e.db, e.queue, runner, discard, WithClock(e.clock.Now), WithCacheDir(e.cache))
	src := filepath.Join(testMedia(t), "Slouchcat Test H264 (2024)", "Slouchcat Test H264 (2024).mp4")
	copyTestFile(t, src, e.dir, "Echt (2024)/Echt (2024).mp4")
	e.scan(e.lib)
	id := e.str(`SELECT id FROM media_files WHERE rel_path LIKE 'Echt%'`)
	if err := e.svc.handleProbe(e.ctx, e.probeJobOf(id)); err != nil {
		t.Fatal(err)
	}
	e.start(id, 0, trickplayAheadSetting.Movies)
	if _, err := e.run(); err != nil {
		t.Fatal(err)
	}
	m, sheets, state := e.status(id)
	if state != TrickplayComplete || !slices.Equal(sheets, []int{0}) || m.Source != TrickplaySourceIndex || m.TileHeight != 180 {
		t.Fatalf("state %q, sheets %v, source %q, height %d", state, sheets, m.Source, m.TileHeight)
	}
	sheet := filepath.Join(e.cache, TrickplayDir, TrickplaySetName(id, e.version(id)), TrickplaySheetName(0))
	p, err := runner.Probe(e.ctx, sheet)
	if err != nil {
		t.Fatal(err)
	}
	rows := (len(m.PointsMs) + 9) / 10
	if v := mainVideo(p.Streams); v == nil || v.Width != 3200 || v.Height != rows*180 {
		t.Errorf("sheet %+v, want 3200 x %d", v, rows*180)
	}
}
