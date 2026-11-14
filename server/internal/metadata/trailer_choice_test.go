package metadata

import (
	"context"
	"slices"
	"strconv"
	"testing"
)

// The rank table of concept 21 section 2.2 (Marco, 12 November 2026), ranks 1 to 10 for TMDB.
func TestTMDBTrailerRank(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		size     int
		official bool
		want     int
	}{
		{2160, true, 1}, {1080, true, 2}, {720, true, 3},
		{2160, false, 4}, {1080, false, 5}, {720, false, 6},
		{480, true, 7}, {360, true, 8}, {0, true, 8},
		{480, false, 9}, {360, false, 10}, {0, false, 10},
	} {
		if got := tmdbTrailerRank(c.size, c.official); got != c.want {
			t.Errorf("tmdbTrailerRank(%d, %v) = %d, want %d", c.size, c.official, got, c.want)
		}
	}
}

func tmdbVideo(lang, key string, size int, official bool, published string) Video {
	return Video{Lang: lang, Key: key, Name: key, Site: "YouTube", Type: "Trailer", Official: official, PublishedAt: published, Size: size}
}

// pickTMDBTrailers picks one trailer per language by the rank, the newer on a tie, and carries the
// size into the hit.
func TestPickTMDBTrailersByResolution(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name   string
		videos []Video
		want   string
	}{
		{"below 720 official loses to unofficial from 720 up", []Video{
			tmdbVideo("de", "offic480000", 480, true, "2024-01-01T00:00:00.000Z"),
			tmdbVideo("de", "unoff720000", 720, false, "2020-01-01T00:00:00.000Z"),
		}, "unoff720000"},
		{"from 720 up official stays in front", []Video{
			tmdbVideo("de", "unoff216000", 2160, false, "2024-01-01T00:00:00.000Z"),
			tmdbVideo("de", "offic720000", 720, true, "2020-01-01T00:00:00.000Z"),
		}, "offic720000"},
		{"higher resolution first among official", []Video{
			tmdbVideo("de", "offic108000", 1080, true, "2024-01-01T00:00:00.000Z"),
			tmdbVideo("de", "offic216000", 2160, true, "2020-01-01T00:00:00.000Z"),
		}, "offic216000"},
		{"360 official before 480 unofficial", []Video{
			tmdbVideo("de", "unoff480000", 480, false, "2024-01-01T00:00:00.000Z"),
			tmdbVideo("de", "offic360000", 360, true, "2020-01-01T00:00:00.000Z"),
		}, "offic360000"},
		{"tie: the newer wins", []Video{
			tmdbVideo("de", "older108000", 1080, true, "2020-01-01T00:00:00.000Z"),
			tmdbVideo("de", "newer108000", 1080, true, "2022-01-01T00:00:00.000Z"),
		}, "newer108000"},
		{"360 and unknown share a rank: the newer wins", []Video{
			tmdbVideo("de", "known360000", 360, false, "2020-01-01T00:00:00.000Z"),
			tmdbVideo("de", "unknown0000", 0, false, "2021-01-01T00:00:00.000Z"),
		}, "unknown0000"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			hits := pickTMDBTrailers(c.videos, []string{"de"}, nil)
			if len(hits) != 1 || hits[0].YouTubeID != c.want {
				t.Fatalf("hits %+v, want %s", hits, c.want)
			}
			for _, v := range c.videos {
				if v.Key == c.want && hits[0].Size != v.Size {
					t.Errorf("size %d, want %d", hits[0].Size, v.Size)
				}
			}
		})
	}
}

// YouTube: hd before sd (ranks 11 and 12), within the same definition the official channel first.
func TestPickYouTubeVideoPrefersHD(t *testing.T) {
	t.Parallel()
	video := func(id, title, channel, definition string) youtubeVideo {
		v := youtubeVideo{ID: id}
		v.Snippet.Title, v.Snippet.ChannelTitle = title, channel
		v.ContentDetails.Duration, v.ContentDetails.Definition = "PT2M", definition
		return v
	}
	titles := []string{"Searched Film"}
	officialSD := video("officSD0001", "Searched Film Official Trailer", "Studio Official", "sd")
	plainHD := video("plainHD0001", "Searched Film Trailer", "Movie Clips", "hd")
	officialHD := video("officHD0001", "Searched Film Official Trailer", "Studio Official", "hd")
	rank := map[string]int{"officSD0001": 0, "plainHD0001": 1, "officHD0001": 2}
	if v, ok := pickYouTubeVideo([]youtubeVideo{officialSD, plainHD}, rank, titles, "en"); !ok || v.ID != "plainHD0001" {
		t.Errorf("sd official vs hd plain: %q, want plainHD0001", v.ID)
	}
	if v, ok := pickYouTubeVideo([]youtubeVideo{officialSD, plainHD, officialHD}, rank, titles, "en"); !ok || v.ID != "officHD0001" {
		t.Errorf("two hd: %q, want the official channel officHD0001", v.ID)
	}
	if v, ok := pickYouTubeVideo([]youtubeVideo{officialSD}, rank, titles, "en"); !ok || v.ID != "officSD0001" {
		t.Errorf("sd only: %q, want officSD0001", v.ID)
	}
}

// trailerSizes returns lang/youtube_id/source/size of the stored trailers of a title.
func (e *env) trailerSizes(t *testing.T, id string) []string {
	t.Helper()
	rows, err := e.svc.read.ListTrailersByItem(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, r := range rows {
		size := "null"
		if r.Size.Valid {
			size = strconv.FormatInt(r.Size.Int64, 10)
		}
		out = append(out, r.Lang+"/"+r.YoutubeID+"/"+r.Source+"/"+size)
	}
	return out
}

// The one-time run (O-3) queues a recheck for the titles with found trailers and an automatic
// trailer only, once; the recheck takes TMDB by resolution with its size, keeps the YouTube hit of a
// language TMDB lacks and the manual rows, and searches nothing on YouTube.
func TestTrailerRecheckOnce(t *testing.T) {
	t.Parallel()
	e := newEnvWith(t, testKeys{tmdb: goodToken, youtube: ytKey})
	ctx := context.Background()
	e.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	e.addItem(t, itemSeed{id: idBySearch, title: "Searched Film", year: 2002, tmdbID: 601})
	e.addItem(t, itemSeed{id: idByIMDB, title: "Manual only", tmdbID: 550})
	e.addItem(t, itemSeed{id: idNoMatch, title: "Pending", tmdbID: 550})
	e.addItem(t, itemSeed{id: idLocked, title: "Locked", tmdbID: 550, locked: `["trailer"]`})
	mustExec(t, e.db.Write, `UPDATE items SET trailer_state = 'found'`)
	mustExec(t, e.db.Write, `UPDATE items SET trailer_state = 'pending' WHERE id = ?`, idNoMatch)
	mustExec(t, e.db.Write, `INSERT INTO trailers (item_id, lang, youtube_id, title, source, official, created_at) VALUES
	    (?, 'de', 'deTrailer01', 'Trailer (neu)', 'tmdb', 0, 1),
	    (?, 'en', 'manualEn001', NULL, 'manual', 0, 1),
	    (?, 'de', 'deTrail0001', 'Searched Film Trailer Deutsch German', 'youtube', 0, 1),
	    (?, 'en', 'enOffic0001', 'SEARCHED FILM – Official Trailer [HD]', 'youtube', 1, 1),
	    (?, 'de', 'manualDe001', NULL, 'manual', 0, 1),
	    (?, 'de', 'pendingDe01', NULL, 'tmdb', 0, 1),
	    (?, 'de', 'lockedDe001', NULL, 'tmdb', 0, 1)`,
		idByTMDB, idByTMDB, idBySearch, idBySearch, idByIMDB, idNoMatch, idLocked)

	n, err := e.svc.QueueTrailerRecheckOnce(ctx)
	if err != nil || n != 2 {
		t.Fatalf("QueueTrailerRecheckOnce = %d, %v; want 2", n, err)
	}
	if got, want := e.jobList(t, JobTrailerFind), []string{"trailer:" + idByTMDB + " p1", "trailer:" + idBySearch + " p1"}; !slices.Equal(got, want) {
		t.Fatalf("trailer jobs %q, want %q", got, want)
	}
	var rechecks int
	if err := e.db.Read.QueryRow(`SELECT count(*) FROM jobs WHERE type = ? AND json_extract(payload, '$.recheck') = 1`, JobTrailerFind).Scan(&rechecks); err != nil {
		t.Fatal(err)
	}
	if rechecks != 2 {
		t.Fatalf("%d jobs marked recheck, want 2", rechecks)
	}
	mustExec(t, e.db.Write, `DELETE FROM jobs`)
	if n, err := e.svc.QueueTrailerRecheckOnce(ctx); err != nil || n != 0 || len(e.jobList(t, JobTrailerFind)) != 0 {
		t.Fatalf("second QueueTrailerRecheckOnce = %d, %v; want nothing queued", n, err)
	}

	for _, id := range []string{idByTMDB, idBySearch} {
		if err := e.svc.recheckTrailers(ctx, id); err != nil {
			t.Fatal(err)
		}
		if state, _ := e.trailerState(t, id); state != "found" {
			t.Errorf("%s: state %s, want found", id, state)
		}
	}
	// TMDB has both languages for 550 (every video 1080 in the fake): the official ones with size.
	if got, want := e.trailerSizes(t, idByTMDB), []string{"de/deOfficial1/tmdb/1080", "en/enOfficial1/tmdb/1080", "en/manualEn001/manual/null"}; !slices.Equal(got, want) {
		t.Errorf("rechecked trailers %q, want %q", got, want)
	}
	// TMDB has nothing for 601: the YouTube hits stay.
	if got, want := e.trailerSizes(t, idBySearch), []string{"de/deTrail0001/youtube/null", "en/enOffic0001/youtube/null"}; !slices.Equal(got, want) {
		t.Errorf("rechecked trailers %q, want %q", got, want)
	}
	if n := len(e.yt.searches()); n != 0 {
		t.Errorf("%d YouTube searches during the recheck, want none", n)
	}
	if got := e.trailerSizes(t, idByIMDB); !slices.Equal(got, []string{"de/manualDe001/manual/null"}) {
		t.Errorf("manual-only title changed: %q", got)
	}

	// A rejected TMDB key leaves a rechecked title as it is (no failed state).
	e2 := newEnvWith(t, testKeys{tmdb: "revoked"})
	e2.addItem(t, itemSeed{id: idByTMDB, title: "First", tmdbID: 550})
	mustExec(t, e2.db.Write, `UPDATE items SET trailer_state = 'found'`)
	mustExec(t, e2.db.Write, `INSERT INTO trailers (item_id, lang, youtube_id, source, official, created_at) VALUES (?, 'de', 'deTrailer01', 'tmdb', 0, 1)`, idByTMDB)
	if err := e2.svc.recheckTrailers(ctx, idByTMDB); err != nil {
		t.Fatal(err)
	}
	if state, _ := e2.trailerState(t, idByTMDB); state != "found" {
		t.Errorf("state after a rejected key %s, want found", state)
	}
	if got := e2.trailerSizes(t, idByTMDB); !slices.Equal(got, []string{"de/deTrailer01/tmdb/null"}) {
		t.Errorf("trailers after a rejected key %q", got)
	}
}
