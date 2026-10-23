package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The data every check needs: an account, a profile with a PIN, a library of films, one of
// audiobooks and a podcast with episodes. There is no ready-made fixture for a third-party
// client — the Playwright suite builds the same state through the same API (web/e2e/global-setup.ts)
// — so the client sets the installation up itself through the setup wizard and the admin
// operations of the contract. The media are the generated test media of testdata/media
// (`task testdata:media`), the feed is the local fixture of feed.go.

// Fixed credentials of the run. The server is a throw-away instance of this check; these are not
// secrets, and no other installation knows them.
const (
	ownerEmail    = "apicheck-owner@slouchcat.test"
	ownerPassword = "apicheck-password-0123"
	ownerPin      = "246810"
)

// Titles of the generated test media the checks work with.
const (
	movieTitle     = "Slouchcat Test H264 (2024)"
	multiAudio     = "Slouchcat Test Multi Audio (2024)"
	audiobookTitle = "Slouchcat Test Audiobook M4B"
)

// Timings of the fixtures.
const (
	scanTimeout   = 5 * time.Minute
	searchTimeout = 2 * time.Minute
	notifyTimeout = 2 * time.Minute
	pollInterval  = time.Second
)

// state holds everything the checks found while the fixtures were built.
type state struct {
	c        *client
	testdata string
	feed     *feedServer

	profileID int64
	movie     itemRef // film in MP4 with H.264 and AAC: direct play
	multi     itemRef // film in MKV with two audio tracks and two subtitle tracks
	audiobook itemRef // M4B with chapter marks
	podcast   itemRef
	podcastID string // the podcast as the administration knows it
	announce  int64

	// downloadID is set by the download check and used by the offline events that follow it.
	downloadID string
}

// mediaDir is the directory of the generated test media. The server resolves library paths on its
// own file system and takes absolute ones only, so the client sends what it resolved itself.
func (s *state) mediaDir() string {
	absolute, err := filepath.Abs(s.testdata)
	if err != nil {
		absolute = s.testdata
	}
	return filepath.Join(absolute, "media")
}

// waitForServer polls the health endpoint of the contract until the server answers.
func (s *state) waitForServer(ctx context.Context, wait time.Duration) error {
	deadline := time.Now().Add(wait)
	var last error
	for {
		var h health
		err := s.c.get(ctx, "/healthz", nil, &h)
		if err == nil {
			return nil
		}
		if errors.Is(err, errUnknownOperation) || ctx.Err() != nil {
			return err
		}
		last = err
		if time.Now().After(deadline) {
			return fmt.Errorf("server did not become healthy within %s: %w", wait, last)
		}
		if err := sleep(ctx, pollInterval); err != nil {
			return err
		}
	}
}

// prepare runs the setup wizard (or logs in on a server that already ran it), adds the audio
// libraries, subscribes to the local feed and collects the ids the checks need.
func (s *state) prepare(ctx context.Context) error {
	var status setupStatus
	if err := s.c.get(ctx, "/setup", nil, &status); err != nil {
		return err
	}
	if status.SetupRequired {
		body := setupRequest{
			Email:    ownerEmail,
			Password: ownerPassword,
			Language: "de",
			Library:  libraryRequest{Name: "Filme", Kind: "movies", Path: filepath.Join(s.mediaDir(), "Movies")},
		}
		if err := s.c.call(ctx, http.MethodPost, "/setup", nil, body, nil, http.StatusCreated); err != nil {
			return err
		}
	}
	if err := s.login(ctx); err != nil {
		return err
	}
	if err := s.ensureLibraries(ctx); err != nil {
		return err
	}
	if err := s.waitForScans(ctx); err != nil {
		return err
	}
	if err := s.ensurePodcast(ctx); err != nil {
		return err
	}
	return s.findItems(ctx)
}

// login signs the main account in and makes sure its primary profile is active, with the PIN when
// an earlier run already set one.
func (s *state) login(ctx context.Context) error {
	var session sessionInfo
	err := s.c.call(ctx, http.MethodPost, "/auth/login", nil,
		loginRequest{Email: ownerEmail, Password: ownerPassword}, &session, http.StatusOK)
	if err != nil {
		return err
	}
	if session.Profile == nil {
		session, err = s.selectPrimary(ctx)
		if err != nil {
			return err
		}
	}
	if session.Profile == nil {
		return errors.New("login left the session without a profile")
	}
	s.profileID = session.Profile.ID
	return nil
}

// selectPrimary activates the primary profile of the account, with its PIN when it has one.
func (s *state) selectPrimary(ctx context.Context) (sessionInfo, error) {
	var profiles []profileSummary
	if err := s.c.get(ctx, "/auth/profiles", nil, &profiles); err != nil {
		return sessionInfo{}, err
	}
	for _, p := range profiles {
		if !p.IsPrimary {
			continue
		}
		var body any
		if p.HasPin {
			body = pinRequest{Pin: ownerPin}
		}
		var session sessionInfo
		path := "/auth/profiles/" + strconv.FormatInt(p.ID, 10) + "/select"
		if err := s.c.call(ctx, http.MethodPost, path, nil, body, &session, http.StatusOK); err != nil {
			return sessionInfo{}, err
		}
		return session, nil
	}
	return sessionInfo{}, errors.New("the account has no primary profile")
}

// ensureLibraries adds the audiobook library and the podcast library next to the films.
func (s *state) ensureLibraries(ctx context.Context) error {
	libraries, err := s.libraries(ctx)
	if err != nil {
		return err
	}
	has := func(name string) bool {
		for _, l := range libraries {
			if l.Name == name {
				return true
			}
		}
		return false
	}
	rating := 0
	if !has("Hörbücher") {
		body := libraryRequest{
			Name: "Hörbücher", Kind: "audiobooks",
			Path:          filepath.Join(s.mediaDir(), "Audiobooks"),
			DefaultRating: &rating,
		}
		if err := s.c.call(ctx, http.MethodPost, "/admin/libraries", nil, body, nil, http.StatusCreated, http.StatusAccepted); err != nil {
			return err
		}
	}
	if !has("Podcasts") {
		body := libraryRequest{Name: "Podcasts", Kind: "podcasts", DefaultRating: &rating}
		if err := s.c.call(ctx, http.MethodPost, "/admin/libraries", nil, body, nil, http.StatusCreated, http.StatusAccepted); err != nil {
			return err
		}
	}
	return nil
}

// libraries lists the libraries with their scan state.
func (s *state) libraries(ctx context.Context) ([]library, error) {
	var libraries []library
	if err := s.c.get(ctx, "/admin/libraries", nil, &libraries); err != nil {
		return nil, err
	}
	return libraries, nil
}

// waitForScans waits until every library with a directory finished its scan.
func (s *state) waitForScans(ctx context.Context) error {
	deadline := time.Now().Add(scanTimeout)
	for {
		libraries, err := s.libraries(ctx)
		if err != nil {
			return err
		}
		done, running := true, ""
		for _, l := range libraries {
			if l.Kind == "podcasts" { // a podcast library has no directory to scan
				continue
			}
			switch l.Scan.State {
			case "done":
			case "failed", "aborted":
				return fmt.Errorf("scan of library %q %s: %s", l.Name, l.Scan.State, l.Scan.LastError)
			default:
				done = false
				running = l.Name + " " + l.Scan.State
			}
		}
		if done {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("library scans unfinished after %s (%s)", scanTimeout, running)
		}
		if err := sleep(ctx, pollInterval); err != nil {
			return err
		}
	}
}

// ensurePodcast subscribes the podcast library to the local feed fixture.
func (s *state) ensurePodcast(ctx context.Context) error {
	var existing []adminPodcast
	if err := s.c.get(ctx, "/admin/podcasts", nil, &existing); err != nil {
		return err
	}
	for _, p := range existing {
		if p.FeedURL == s.feed.feedURL() {
			s.podcastID = p.ID
			return nil
		}
	}
	libraries, err := s.libraries(ctx)
	if err != nil {
		return err
	}
	var libraryID int64
	for _, l := range libraries {
		if l.Kind == "podcasts" {
			libraryID = l.ID
		}
	}
	if libraryID == 0 {
		return errors.New("no podcast library")
	}
	var created adminPodcast
	body := podcastCreate{LibraryID: libraryID, FeedURL: s.feed.feedURL()}
	if err := s.c.call(ctx, http.MethodPost, "/admin/podcasts", nil, body, &created, http.StatusCreated); err != nil {
		return err
	}
	s.podcastID = created.ID
	return nil
}

// findItems looks the titles of the checks up through the search of the contract. A finished scan
// does not mean the title is searchable in the same instant, so each title is given a little
// time, the way the Playwright setup waits for its audiobooks.
func (s *state) findItems(ctx context.Context) error {
	finds := []struct {
		title string
		kind  string
		into  *itemRef
	}{
		{movieTitle, "movie", &s.movie},
		{multiAudio, "movie", &s.multi},
		{audiobookTitle, "audiobook", &s.audiobook},
	}
	for _, find := range finds {
		item, err := s.search(ctx, find.title, find.kind)
		if err != nil {
			return err
		}
		*find.into = item
	}
	// The podcast is taken by its id, not by its title: the subscription names the item, and a
	// server that already carries other feeds must not hand the checks the wrong one.
	podcast, err := s.detail(ctx, s.podcastID)
	if err != nil {
		return fmt.Errorf("read the subscribed podcast: %w", err)
	}
	s.podcast = podcast.itemRef
	return nil
}

// search returns the title of the given kind whose name matches, or an error naming what the
// library holds instead.
func (s *state) search(ctx context.Context, title, kind string) (itemRef, error) {
	want := titleOf(title)
	deadline := time.Now().Add(searchTimeout)
	for {
		var result searchResult
		body := searchRequest{Query: want, MediaTypes: []string{kind}, Limit: 50}
		if err := s.c.call(ctx, http.MethodPost, "/search", nil, body, &result, http.StatusOK); err != nil {
			return itemRef{}, err
		}
		seen := make([]string, 0, len(result.Items))
		for _, item := range result.Items {
			if item.Kind == kind && item.Title == want {
				return item, nil
			}
			seen = append(seen, item.Title)
		}
		if time.Now().After(deadline) {
			return itemRef{}, fmt.Errorf("the library holds no %s %q within %s (found: %s); are the test media of testdata/media generated?",
				kind, want, searchTimeout, strings.Join(seen, ", "))
		}
		if err := sleep(ctx, pollInterval); err != nil {
			return itemRef{}, err
		}
	}
}

// titleOf strips the year suffix the file name carries, e.g. "Film (2024)" → "Film".
func titleOf(name string) string {
	if i := len(name) - len(" (2024)"); i > 0 && name[i:] == " (2024)" {
		return name[:i]
	}
	return name
}

// seedNotification makes the server create a real inbox entry (E-8): the profile marks the
// podcast as a favourite, a new episode appears in the feed and the administration fetches it.
func (s *state) seedNotification(ctx context.Context) error {
	path := "/items/" + s.podcast.ID + "/favorite"
	if err := s.c.call(ctx, http.MethodPut, path, nil, nil, nil, http.StatusNoContent, http.StatusOK); err != nil {
		return err
	}
	s.feed.setEpisodes(3)
	refresh := "/admin/podcasts/" + s.podcastID + "/refresh"
	if err := s.c.call(ctx, http.MethodPost, refresh, nil, nil, nil, http.StatusAccepted); err != nil {
		return err
	}
	deadline := time.Now().Add(notifyTimeout)
	for {
		var page notificationPage
		if err := s.c.get(ctx, "/notifications", nil, &page); err != nil {
			return err
		}
		if page.Total > 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("no notification of a new episode within %s", notifyTimeout)
		}
		if err := sleep(ctx, pollInterval); err != nil {
			return err
		}
	}
}

// seedAnnouncement publishes one announcement for every profile (A-7).
func (s *state) seedAnnouncement(ctx context.Context) error {
	body := announcementRequest{
		Title:   "Fremd-Client-Test P-16",
		Body:    "Diese Ankündigung stammt aus dem Fremd-Client-Test der Abnahme.",
		Publish: true,
	}
	var created announcement
	err := s.c.call(ctx, http.MethodPost, "/admin/announcements", nil, body, &created, http.StatusCreated)
	if err != nil {
		return err
	}
	s.announce = created.ID
	if created.PublishedAt != "" {
		return nil
	}
	publish := "/admin/announcements/" + strconv.FormatInt(created.ID, 10) + "/publish"
	return s.c.call(ctx, http.MethodPost, publish, nil, nil, nil, http.StatusOK)
}

// seedTrailer stores a manual trailer for the film, so the trailer info of the viewer has
// content without a TMDB or YouTube key (B-8 "manueller Ersatz").
func (s *state) seedTrailer(ctx context.Context) error {
	path := "/admin/items/" + s.movie.ID + "/trailers"
	body := trailerRequest{Lang: "de", YoutubeID: "dQw4w9WgXcQ"}
	err := s.c.call(ctx, http.MethodPost, path, nil, body, nil, http.StatusCreated)
	if err != nil {
		var existing []trailer
		if s.c.get(ctx, "/items/"+s.movie.ID+"/trailers", nil, &existing) == nil && len(existing) > 0 {
			return nil // an earlier run already stored one
		}
		return err
	}
	return nil
}

// lockProfile gives the only profile a PIN and ends the session, so the login check starts where
// a viewer starts: at the picker of a PIN-protected profile (K-4).
func (s *state) lockProfile(ctx context.Context) error {
	path := "/profiles/" + strconv.FormatInt(s.profileID, 10) + "/pin"
	err := s.c.call(ctx, http.MethodPut, path, nil, pinRequest{Pin: ownerPin}, nil, http.StatusNoContent)
	if err != nil {
		return err
	}
	return s.c.call(ctx, http.MethodPost, "/auth/logout", nil, nil, nil, http.StatusNoContent)
}

// sleep waits unless the context ends first.
func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("run cancelled: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

// query builds a query string from name/value pairs.
func query(pairs ...string) url.Values {
	values := url.Values{}
	for i := 0; i+1 < len(pairs); i += 2 {
		values.Set(pairs[i], pairs[i+1])
	}
	return values
}
