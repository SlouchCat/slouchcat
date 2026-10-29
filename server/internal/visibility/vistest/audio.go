package vistest

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// Audio is the audio fixture of AddAudio (M6, design section 5).
type Audio struct {
	AudiobookLibraryID int64 // library "Hörbücher", kind audiobooks, default rating 0
	PodcastLibraryID   int64 // library "Podcasts", kind podcasts, default rating 0
	Dir                string
	// Books are the audiobooks rated 0, 6, 12, 16, 18 (manual) and one unrated, one file each.
	Books []Movie
	// Parts is a book of three files (part_index 1..3) rated 0 by the library default; FileID is
	// its first part, PartIDs all three in order.
	Parts Movie
	// Podcast is subscribed to a feed (rated 0 by the library default) with the episodes below.
	Podcast Movie
	// Episodes have a file and an enclosure, own ratings 0, 6, 12 and 16 (manual).
	Episodes []Movie
	// Explicit is the itunes:explicit episode (FSK 18, rating_source feed) with file and enclosure.
	Explicit Movie
	// EnclosureOnly is an episode without file and without images of its own (it shows the podcast's
	// cover); its rating comes from the podcast (0).
	EnclosureOnly Movie
	// Empty is a podcast without episodes: no content, so it is unavailable and invisible.
	Empty Movie
	// ExplicitPodcast is a podcast whose channel is itunes:explicit (FSK 18, rating_source feed,
	// AN-2) with one episode that has no explicit flag of its own and inherits the 18.
	ExplicitPodcast Movie
	// ExplicitPodcastEpisode is that episode (file and enclosure).
	ExplicitPodcastEpisode Movie
	// EnclosureServer serves every episode's Content at its Enclosure URL with Range support.
	EnclosureServer *httptest.Server
}

// Titles returns every audio title, containers included.
func (a Audio) Titles() []Movie {
	if a.Podcast.ID == "" {
		return nil
	}
	out := append([]Movie{}, a.Books...)
	out = append(out, a.Parts, a.Podcast)
	out = append(out, a.Episodes...)
	return append(out, a.Explicit, a.EnclosureOnly, a.Empty, a.ExplicitPodcast, a.ExplicitPodcastEpisode)
}

// EnclosureType is the MIME type of every fixture enclosure.
const EnclosureType = "audio/mpeg"

// AddAudio adds an audiobook library and a podcast library with default rating 0 (AN-2) and the
// titles of Audio: audiobooks with one file rated 0, 6, 12, 16, 18 and unrated, a book of three
// parts, a podcast with episodes rated 0, 6, 12, 16, an explicit episode and an episode with only an
// enclosure, and a podcast without episodes. Every title has texts and images like the movies
// (except the enclosure-only episode, which shows its podcast's images). An httptest server serves
// the enclosures; it stops with the test.
func (f *Fixture) AddAudio(t testing.TB) {
	t.Helper()
	ctx := context.Background()
	q := appq.New(f.DB.Write)
	zero := ptr(0)
	books, err := q.InsertLibrary(ctx, appq.InsertLibraryParams{Name: "Hörbücher", Kind: model.LibraryAudiobooks, DefaultRating: nullInt(zero), Now: 1})
	if err != nil {
		t.Fatal(err)
	}
	pods, err := q.InsertLibrary(ctx, appq.InsertLibraryParams{Name: "Podcasts", Kind: model.LibraryPodcasts, DefaultRating: nullInt(zero), Now: 1})
	if err != nil {
		t.Fatal(err)
	}
	a := Audio{AudiobookLibraryID: books.ID, PodcastLibraryID: pods.ID, Dir: t.TempDir()}
	// Podcast libraries have no root (H-2); the episode files of the fixture stand for the local
	// episode files of a later milestone and need one, so both libraries share the fixture's root.
	root, err := q.InsertLibraryRoot(ctx, appq.InsertLibraryRootParams{LibraryID: books.ID, Path: a.Dir, FsType: "ext4", Watch: "inotify",
		Content: model.RootAudiobook, Providers: model.ProvidersJSON(model.DefaultProviders(model.RootAudiobook)), Now: 1})
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	contents := map[string][]byte{} // enclosure path → content
	mux := http.NewServeMux()
	mux.HandleFunc("GET /episodes/{name}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		data, ok := contents[r.PathValue("name")]
		mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", EnclosureType)
		http.ServeContent(w, r, "", time.Unix(1, 0), bytes.NewReader(data))
	})
	a.EnclosureServer = httptest.NewServer(mux)
	t.Cleanup(a.EnclosureServer.Close)

	n := 40
	add := func(m Movie, library int64) Movie {
		n++
		m.Container = true // addMovie writes no video file; the audio files follow below
		out := f.addMovie(t, m, library, root.ID, a.Dir, n)
		out.Container = m.Kind == "podcast"
		return out
	}
	file := func(m *Movie, fileID, rel, container, codec string, part int) {
		content := make([]byte, FileSize)
		if _, err := rand.Read(content); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(a.Dir, filepath.FromSlash(rel)), content)
		exec(t, f.DB, `INSERT INTO media_files (id, item_id, root_id, rel_path, size, mtime, container, audio_codec, duration_ms,
		               part_index, probe, probed_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, 1, ?, ?, ?, ?, ?, 1, 1, 1)`,
			fileID, m.ID, root.ID, rel, FileSize, container, codec, DurationMs, nullInt(partIndex(part)), audioProbeJSON(container, codec))
		if m.FileID == fileID {
			m.RelPath, m.Content = rel, content
		}
	}

	for _, r := range []struct {
		label  string
		rating *int
	}{{"A0", ptr(0)}, {"A6", ptr(6)}, {"AC", ptr(12)}, {"AG", ptr(16)}, {"AJ", ptr(18)}, {"AN", nil}} {
		m := add(Movie{Kind: "audiobook", Label: r.label, Rating: r.rating, Effective: r.rating}, books.ID)
		source := "manual"
		if r.rating == nil {
			source = ""
		} else if *r.rating == 0 {
			source = "library"
		}
		exec(t, f.DB, `UPDATE items SET rating_source = NULLIF(?, ''), author = 'Autorin '||?, duration_ms = ? WHERE id = ?`,
			source, r.label, DurationMs, m.ID)
		file(&m, m.FileID, "Autorin/Buch "+r.label+"/Buch "+r.label+".m4b", "m4b", "aac", 0)
		a.Books = append(a.Books, m)
	}
	a.Parts = add(Movie{Kind: "audiobook", Label: "AP", Rating: zero, Effective: zero}, books.ID)
	exec(t, f.DB, `UPDATE items SET rating_source = 'library', duration_ms = ? WHERE id = ?`, 3*DurationMs, a.Parts.ID)
	for k := range 3 {
		id := a.Parts.FileID
		if k > 0 {
			id = fmt.Sprintf("01J9000000000000000000F%dAP", k)
		}
		a.Parts.PartIDs = append(a.Parts.PartIDs, id)
		file(&a.Parts, id, fmt.Sprintf("Autorin/Teile/%02d - Teil.mp3", k+1), "mp3", "mp3", k+1)
	}

	a.Podcast = add(Movie{Kind: "podcast", Label: "PC", Rating: zero, Effective: zero, MinChild: zero, Container: true}, pods.ID)
	exec(t, f.DB, `UPDATE items SET rating_source = 'library', author = 'Podcasterin', feed_url = ?, feed_checked_at = 1 WHERE id = ?`,
		a.EnclosureServer.URL+"/feed.xml", a.Podcast.ID)
	day := time.Date(2026, 10, 1, 6, 0, 0, 0, time.UTC)
	parent := a.Podcast.ID
	episode := func(label string, own, effective *int, withFile, explicit bool) Movie {
		m := add(Movie{Kind: "podcast_episode", Label: label, ParentID: parent, Rating: own, Effective: effective}, pods.ID)
		day = day.Add(24 * time.Hour)
		name := "episode-" + strings.ToLower(label) + ".mp3"
		m.Enclosure = a.EnclosureServer.URL + "/episodes/" + name
		source := ""
		switch {
		case explicit:
			source = "feed"
		case own != nil:
			source = "manual"
		}
		exec(t, f.DB, `UPDATE items SET guid = ?, published_at = ?, duration_ms = ?, enclosure_type = ?, explicit = ?,
		               rating_source = NULLIF(?, ''), enclosure_url = ? WHERE id = ?`,
			"guid-"+label, day.UnixMilli(), DurationMs, EnclosureType, explicit, source, m.Enclosure, m.ID)
		if withFile {
			file(&m, m.FileID, "Podcast/"+name, "mp3", "mp3", 0)
		} else {
			m.FileID = ""
			m.Content = make([]byte, FileSize)
			if _, err := rand.Read(m.Content); err != nil {
				t.Fatal(err)
			}
		}
		mu.Lock()
		contents[name] = m.Content
		mu.Unlock()
		return m
	}
	for _, r := range []struct {
		label  string
		rating int
	}{{"Q0", 0}, {"Q6", 6}, {"QC", 12}, {"QG", 16}} {
		a.Episodes = append(a.Episodes, episode(r.label, ptr(r.rating), ptr(r.rating), true, false))
	}
	a.Explicit = episode("QX", ptr(18), ptr(18), true, true)
	a.EnclosureOnly = episode("QE", nil, zero, false, false)
	// The enclosure-only episode has no images of its own: it shows the podcast's.
	exec(t, f.DB, `DELETE FROM images WHERE item_id = ?`, a.EnclosureOnly.ID)
	a.EnclosureOnly.PosterRel, a.EnclosureOnly.PosterWebPRel = a.Podcast.PosterRel, a.Podcast.PosterWebPRel
	a.EnclosureOnly.BackdropRel, a.EnclosureOnly.BackdropSmallRel = a.Podcast.BackdropRel, a.Podcast.BackdropSmallRel

	a.Empty = add(Movie{Kind: "podcast", Label: "PE", Rating: zero, Effective: zero, MinChild: zero, Container: true, Unavailable: true}, pods.ID)
	exec(t, f.DB, `UPDATE items SET rating_source = 'library', feed_url = ?, feed_checked_at = 1 WHERE id = ?`,
		a.EnclosureServer.URL+"/empty.xml", a.Empty.ID)

	eighteen := ptr(18)
	a.ExplicitPodcast = add(Movie{Kind: "podcast", Label: "PY", Rating: eighteen, Effective: eighteen, MinChild: eighteen, Container: true}, pods.ID)
	exec(t, f.DB, `UPDATE items SET rating_source = 'feed', explicit = 1, author = 'Podcaster', feed_url = ?, feed_checked_at = 1 WHERE id = ?`,
		a.EnclosureServer.URL+"/explicit.xml", a.ExplicitPodcast.ID)
	parent = a.ExplicitPodcast.ID
	a.ExplicitPodcastEpisode = episode("QY", nil, eighteen, true, false)
	f.Audio = a
}

func nullInt(v *int) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}

func partIndex(part int) *int {
	if part <= 0 {
		return nil
	}
	return &part
}

// SetEnclosure points the enclosure of a podcast episode at another URL (a failing host in tests).
func (f *Fixture) SetEnclosure(t testing.TB, itemID, enclosureURL string) {
	t.Helper()
	exec(t, f.DB, `UPDATE items SET enclosure_url = ? WHERE id = ?`, enclosureURL, itemID)
}

// SetEnclosureType sets the MIME type the feed names for the enclosure of an episode ("" for none).
func (f *Fixture) SetEnclosureType(t testing.TB, itemID, mime string) {
	t.Helper()
	exec(t, f.DB, `UPDATE items SET enclosure_type = NULLIF(?, '') WHERE id = ?`, mime, itemID)
}

// SetFeedURL points a podcast at another feed URL (its host decides whether enclosures and covers
// may lie in the local network, package netguard).
func (f *Fixture) SetFeedURL(t testing.TB, podcastID, feedURL string) {
	t.Helper()
	exec(t, f.DB, `UPDATE items SET feed_url = ? WHERE id = ?`, feedURL, podcastID)
}

// SetProbeError marks the probe of a media file as failed.
func (f *Fixture) SetProbeError(t testing.TB, fileID, msg string) {
	t.Helper()
	exec(t, f.DB, `UPDATE media_files SET probe_error = ? WHERE id = ?`, msg, fileID)
}

// audioProbeJSON is the stored ffprobe result of a fixture audio file: one audio stream, no video,
// the fixture duration at 256 kbit/s, so every download profile encodes (an AAC source within 1.1× of
// a profile would be copied). Multi-part books need a probe per part before they can play (the server
// concatenates the probes of the parts into the book-wide one).
func audioProbeJSON(container, codec string) string {
	return fmt.Sprintf(`{"format":{"name":%q,"duration":%d,"size":%d,"bitRate":256000},"streams":[{"index":0,"type":"audio","codec":%q,"channels":1,"default":true}],"chapters":[]}`,
		container, int64(DurationMs)*1_000_000, FileSize, codec)
}
