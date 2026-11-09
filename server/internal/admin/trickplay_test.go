package admin

import (
	"errors"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// fakeTrickplayUsage reports fixed stored bytes and free space.
type fakeTrickplayUsage struct{ movies, series, free int64 }

func (u fakeTrickplayUsage) TrickplayUsage(int64) (movies, series, cacheFree int64) {
	return u.movies, u.series, u.free
}

// UpdateLibrary stores the trickplay setting of a video library (concept 18, 6.1), refuses it for an
// audio library and refuses unknown modes; GetLibrary of a video library carries the estimate.
func TestUpdateLibraryTrickplay(t *testing.T) {
	t.Parallel()
	a := newAudioFixture(t)
	a.svc.WithTrickplay(fakeTrickplayUsage{movies: 4_000_000, series: 0, free: 35_000_000_000})
	video := a.f.LibraryID()
	set := model.LibraryTrickplay{
		Movies: model.TrickplaySetting{Mode: model.TrickplayOnStart, Keep: false},
		Series: model.TrickplaySetting{Mode: model.TrickplaySeriesStart, Keep: true, Scope: model.TrickplayScopeNextEpisode},
	}
	if _, err := a.svc.UpdateLibrary(a.ctx, video, model.LibraryUpdate{Trickplay: &set}); err != nil {
		t.Fatal(err)
	}
	row, err := appq.New(a.f.DB.Read).GetLibrary(a.ctx, video)
	if err != nil {
		t.Fatal(err)
	}
	if row.TrickplayMovies != "on_start" || row.TrickplayMoviesKeep != 0 || row.TrickplaySeries != "series_start" ||
		row.TrickplaySeriesKeep != 1 || row.TrickplaySeriesScope != "next_episode" {
		t.Errorf("stored setting %+v", row)
	}

	var verr *model.ValidationError
	_, err = a.svc.UpdateLibrary(a.ctx, a.f.Audio.AudiobookLibraryID, model.LibraryUpdate{Trickplay: &set})
	if !errors.As(err, &verr) || len(verr.Fields) != 1 || verr.Fields[0].Field != "trickplay" || verr.Fields[0].Code != "not_allowed" {
		t.Errorf("audio library: %v", err)
	}
	bad := model.LibraryTrickplay{
		Movies: model.TrickplaySetting{Mode: model.TrickplaySeriesStart},
		Series: model.TrickplaySetting{Mode: "later", Scope: "season"},
	}
	_, err = a.svc.UpdateLibrary(a.ctx, video, model.LibraryUpdate{Trickplay: &bad})
	if !errors.As(err, &verr) || len(verr.Fields) != 3 {
		t.Errorf("invalid modes: %v", err)
	}

	lib, err := a.svc.GetLibrary(a.ctx, video)
	if err != nil {
		t.Fatal(err)
	}
	e := lib.TrickplayEstimate
	if e == nil || e.Movies.Titles == 0 || e.Movies.Files == 0 || e.Movies.DurationMs == 0 || e.Movies.StoredBytes != 4_000_000 ||
		e.CacheFreeBytes != 35_000_000_000 {
		t.Fatalf("estimate %+v", e)
	}
	// 1.5 MB per hour of video: the library estimate follows the running time, per title the mean.
	if want := e.Movies.DurationMs * trickplayBytesPerHour / 3_600_000; e.Movies.LibraryBytes != want {
		t.Errorf("library bytes %d, want %d", e.Movies.LibraryBytes, want)
	}
	if want := e.Movies.DurationMs / int64(e.Movies.Files) * trickplayBytesPerHour / 3_600_000; e.Movies.PerTitleBytes != want {
		t.Errorf("per title bytes %d, want %d", e.Movies.PerTitleBytes, want)
	}
	if audio, err := a.svc.GetLibrary(a.ctx, a.f.Audio.AudiobookLibraryID); err != nil || audio.TrickplayEstimate != nil {
		t.Errorf("audio library estimate %+v, %v", audio.TrickplayEstimate, err)
	}
}
