package library

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// A new video library starts at «Aus» for films and series (1.23.1; before «Im Voraus», F-11)
// unless the request names a setting, stored in the row and not left to the column default 'ahead'
// of 00042; an audio library refuses one (concept 18, 8.2).
func TestCreateLibraryTrickplay(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	lib := e.create(t.TempDir())
	off := model.LibraryTrickplay{
		Movies: model.TrickplaySetting{Mode: model.TrickplayOff, Keep: true},
		Series: model.TrickplaySetting{Mode: model.TrickplayOff, Keep: true, Scope: model.TrickplayScopeSeries},
	}
	if lib.Trickplay == nil || *lib.Trickplay != off || *lib.Trickplay != model.DefaultLibraryTrickplay() {
		t.Errorf("default setting %+v", lib.Trickplay)
	}
	if got, err := e.svc.Get(e.ctx, lib.ID); err != nil || got.Trickplay == nil || *got.Trickplay != off {
		t.Errorf("stored default setting %+v, %v", got.Trickplay, err)
	}
	req := testRequest("Serien", t.TempDir())
	set := model.LibraryTrickplay{
		Movies: model.TrickplaySetting{Mode: model.TrickplayOff, Keep: true},
		Series: model.TrickplaySetting{Mode: model.TrickplaySeriesStart, Keep: false, Scope: model.TrickplayScopeSeasonRest},
	}
	req.Trickplay = &set
	lib, err := e.svc.Create(e.ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := e.svc.Get(e.ctx, lib.ID); err != nil || got.Trickplay == nil || *got.Trickplay != set {
		t.Errorf("stored setting %+v, %v", got.Trickplay, err)
	}
	books := model.LibraryRequest{Name: "Hörbücher", Kind: model.LibraryAudiobooks, Roots: rootReq(t.TempDir(), model.RootAudiobook), Trickplay: &set}
	var verr *model.ValidationError
	if _, err := e.svc.Create(e.ctx, books); !errors.As(err, &verr) || verr.Fields[0].Field != "trickplay" || verr.Fields[0].Code != "not_allowed" {
		t.Errorf("audio library with a trickplay setting: %v", err)
	}
	bad := testRequest("Kaputt", t.TempDir())
	bad.Trickplay = &model.LibraryTrickplay{Movies: model.TrickplaySetting{Mode: "series_start"}, Series: set.Series}
	if _, err := e.svc.Create(e.ctx, bad); !errors.As(err, &verr) || verr.Fields[0].Field != "trickplay.movies.mode" {
		t.Errorf("films with series_start: %v", err)
	}
	if got, err := e.svc.Create(e.ctx, model.LibraryRequest{Name: "Hörbücher 2", Kind: model.LibraryAudiobooks, Roots: rootReq(t.TempDir(), model.RootAudiobook)}); err != nil || got.Trickplay != nil {
		t.Errorf("audio library reports %+v, %v", got.Trickplay, err)
	}
}

func ms(v ...int64) []time.Duration {
	out := make([]time.Duration, 0, len(v))
	for _, x := range v {
		out = append(out, time.Duration(x)*time.Millisecond)
	}
	return out
}

// The points of a set (concept 18, 5.4): the first keyframe of each 10-s section that no section
// took, seeked 1 ms after it; a section without its own keyframe repeats the point before it;
// without an index the start of every section.
func TestTrickplayGrid(t *testing.T) {
	t.Parallel()
	v := transcode.Stream{Type: "video", Width: 1920, Height: 800}
	// Keyframes every 4.4 s on average, one gap of 25 s (20.0 → 45.0).
	kf := ms(0, 4400, 9999, 12000, 20000, 45000, 47000, 51000)
	m := TrickplayGrid(55_000, kf, v, "1-2")
	if m.Source != TrickplaySourceIndex || m.TileWidth != 320 || m.TileHeight != 132 || m.Columns != 10 || m.Rows != 10 || m.IntervalMs != 10_000 || m.Version != "1-2-index132" {
		t.Errorf("grid %+v", m)
	}
	wantPoints := []int64{0, 12000, 20000, 20000, 45000, 51000}
	wantSeek := []int64{1, 12001, 20001, 20001, 45001, 51001}
	if !slices.Equal(m.PointsMs, wantPoints) || !slices.Equal(m.SeekMs, wantSeek) {
		t.Errorf("points %v seek %v, want %v %v", m.PointsMs, m.SeekMs, wantPoints, wantSeek)
	}
	// No keyframe in the first section: its first frame.
	if m := TrickplayGrid(20_000, ms(15000), v, "v"); !slices.Equal(m.PointsMs, []int64{0, 15000}) || !slices.Equal(m.SeekMs, []int64{0, 15001}) {
		t.Errorf("late first keyframe: %v %v", m.PointsMs, m.SeekMs)
	}
	f := TrickplayGrid(25_001, nil, transcode.Stream{Width: 720, Height: 480, SAR: "32:27"}, "v")
	if f.Source != TrickplaySourceFallback || f.TileHeight != 180 || !slices.Equal(f.PointsMs, []int64{0, 10000, 20000}) || !slices.Equal(f.SeekMs, f.PointsMs) {
		t.Errorf("fallback grid %+v", f)
	}
	if e := TrickplayGrid(0, kf, v, "v"); len(e.PointsMs) != 0 || e.Sheets() != 0 {
		t.Errorf("no duration: %+v", e)
	}
	// 2 h: 720 points in 8 sheets, the last with 20 points.
	long := TrickplayGrid(7_200_000, nil, v, "v")
	if long.Sheets() != 8 {
		t.Errorf("sheets of 2 h: %d", long.Sheets())
	}
	if from, to := long.SheetPoints(7); from != 700 || to != 720 {
		t.Errorf("last sheet points %d..%d", from, to)
	}
	// From 40 minutes (point 240, sheet 2): 2, 3 … 7, then 1, 0.
	if got := TrickplaySheetOrder(long, 2_400_000); !slices.Equal(got, []int{2, 3, 4, 5, 6, 7, 1, 0}) {
		t.Errorf("sheet order %v", got)
	}
	if got := TrickplaySheetOrder(long, 99_999_999); !slices.Equal(got, []int{7, 6, 5, 4, 3, 2, 1, 0}) {
		t.Errorf("sheet order past the end %v", got)
	}
	if got := TrickplaySheetOrder(long, 0); got[0] != 0 || len(got) != 8 {
		t.Errorf("sheet order from the start %v", got)
	}
}

func TestTrickplaySheetIndex(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]int{"0.webp": 0, "12.webp": 12} {
		if n, ok := trickplaySheetIndex(name); !ok || n != want {
			t.Errorf("%s: %d %v", name, n, ok)
		}
	}
	for _, name := range []string{".tmp-3-1.webp", "01.webp", "-1.webp", "manifest.json", "complete", ".webp", "x.webp"} {
		if _, ok := trickplaySheetIndex(name); ok {
			t.Errorf("%s counts as a sheet", name)
		}
	}
}
