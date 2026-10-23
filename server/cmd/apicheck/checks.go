package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// The viewer functions of criterion P-16 (plan section 8), one check per function and in the
// order the criterion lists them. "Wünschen" is missing on purpose: the M9 cut of 22.10.2026
// dropped the viewer page and its endpoints without replacement (decision 10 in
// docs/planning/04-entscheidungen.md), so the contract describes no operation for it.

// check is one row of the report.
type check struct {
	name string
	run  func(context.Context, *state) (string, error)
}

// checks returns the viewer functions in the order of P-16.
func checks() []check {
	return []check{
		{"Login", checkLogin},
		{"Profilwahl mit PIN", checkProfilePin},
		{"Startseiten-Reihen", checkHome},
		{"Suche mit Filtern", checkSearch},
		{"Detail", checkDetail},
		{"Merkliste", checkWatchlist},
		{"Favoriten", checkFavorites},
		{"Abspielstart mit Fähigkeitsprofil", checkPlaybackStart},
		{"Abruf Direct Play/HLS mit Stream-Token", checkStreams},
		{"Spur- und Untertitelwahl", checkTracks},
		{"Fortschritt melden", checkProgress},
		{"Download vorbereiten und abrufen", checkDownload},
		{"Offline-Ereignisse hochladen", checkOfflineEvents},
		{"Benachrichtigungen lesen", checkNotifications},
		{"Ankündigungen lesen", checkAnnouncements},
		{"Trailer-Info", checkTrailers},
		{"Hörbuch-Kapitel", checkAudiobookChapters},
		{"Podcast-Episoden", checkPodcastEpisodes},
	}
}

// checkLogin signs in with e-mail and password. The fixtures gave the only profile a PIN and
// logged out again, so the answer carries the account without an active profile: the client has
// to show the picker.
func checkLogin(ctx context.Context, s *state) (string, error) {
	var session sessionInfo
	body := loginRequest{Email: ownerEmail, Password: ownerPassword}
	if err := s.c.call(ctx, http.MethodPost, "/auth/login", nil, body, &session, http.StatusOK); err != nil {
		return "", err
	}
	if session.Account.Email != ownerEmail {
		return "", fmt.Errorf("login answered with the account %q", session.Account.Email)
	}
	if session.Profile != nil {
		return "", errors.New("login activated a profile although the only profile has a PIN")
	}
	var current sessionInfo
	if err := s.c.get(ctx, "/auth/session", nil, &current); err != nil {
		return "", err
	}
	if current.Account.ID != session.Account.ID {
		return "", errors.New("the session does not belong to the account that logged in")
	}
	return fmt.Sprintf("Sitzung für %s (Rolle %s), noch ohne Profil",
		session.Account.Email, session.Account.Role), nil
}

// checkProfilePin walks the profile picker: without the PIN the server refuses, with it the
// profile becomes active.
func checkProfilePin(ctx context.Context, s *state) (string, error) {
	var profiles []profileSummary
	if err := s.c.get(ctx, "/auth/profiles", nil, &profiles); err != nil {
		return "", err
	}
	var primary profileSummary
	for _, p := range profiles {
		if p.IsPrimary {
			primary = p
		}
	}
	if primary.ID == 0 {
		return "", errors.New("the picker shows no primary profile")
	}
	if !primary.HasPin {
		return "", fmt.Errorf("profile %q carries no PIN", primary.Name)
	}
	path := "/auth/profiles/" + strconv.FormatInt(primary.ID, 10) + "/select"
	resp, err := s.c.send(ctx, http.MethodPost, path, nil, nil)
	if err != nil {
		return "", err
	}
	if err := resp.expect(http.MethodPost, path, http.StatusForbidden); err != nil {
		return "", fmt.Errorf("selection without a PIN was not refused: %w", err)
	}
	var session sessionInfo
	err = s.c.call(ctx, http.MethodPost, path, nil, pinRequest{Pin: ownerPin}, &session, http.StatusOK)
	if err != nil {
		return "", err
	}
	if session.Profile == nil || session.Profile.ID != primary.ID {
		return "", errors.New("the session did not switch to the chosen profile")
	}
	s.profileID = session.Profile.ID
	return fmt.Sprintf("ohne PIN 403 %s, mit PIN aktives Profil %q (FSK %d)",
		"pin_required", session.Profile.Name, session.Profile.MaxRating), nil
}

// checkHome reads the rows of the start page.
func checkHome(ctx context.Context, s *state) (string, error) {
	var page homePage
	if err := s.c.get(ctx, "/home", nil, &page); err != nil {
		return "", err
	}
	if len(page.Rows) == 0 {
		return "", errors.New("the start page has no rows")
	}
	parts := make([]string, 0, len(page.Rows))
	titles := 0
	for _, row := range page.Rows {
		titles += len(row.Items)
		parts = append(parts, fmt.Sprintf("%s %d", row.Key, len(row.Items)+len(row.Genres)))
	}
	if titles == 0 {
		return "", errors.New("no row of the start page holds a title")
	}
	return fmt.Sprintf("%d Reihen: %s", len(page.Rows), strings.Join(parts, ", ")), nil
}

// checkSearch searches with filters and browses by filters alone.
func checkSearch(ctx context.Context, s *state) (string, error) {
	var genres []struct {
		Name string `json:"name"`
	}
	if err := s.c.get(ctx, "/genres", nil, &genres); err != nil {
		return "", err
	}
	var found searchResult
	body := searchRequest{
		Query:        "Slouchcat",
		MediaTypes:   []string{"movie"},
		YearFrom:     2000,
		YearTo:       2100,
		Availability: []string{"local"},
		Limit:        50,
	}
	if err := s.c.call(ctx, http.MethodPost, "/search", nil, body, &found, http.StatusOK); err != nil {
		return "", err
	}
	if found.Total == 0 {
		return "", errors.New("the filtered search found no film")
	}
	var browsed searchResult
	browse := searchRequest{MediaTypes: []string{"audiobook"}, Limit: 50}
	if err := s.c.call(ctx, http.MethodPost, "/search", nil, browse, &browsed, http.StatusOK); err != nil {
		return "", err
	}
	if browsed.Total == 0 {
		return "", errors.New("browsing by the filter audiobook found nothing")
	}
	return fmt.Sprintf("Text + Filter (movie, 2000–2100, local): %d Treffer; nur Filter (audiobook): %d; %d Genre(s) im Vertrag abgefragt",
		found.Total, browsed.Total, len(genres)), nil
}

// checkDetail reads the detail of a film.
func checkDetail(ctx context.Context, s *state) (string, error) {
	detail, err := s.detail(ctx, s.movie.ID)
	if err != nil {
		return "", err
	}
	if len(detail.MediaFiles) == 0 {
		return "", fmt.Errorf("the detail of %q names no media file", detail.Title)
	}
	file := detail.MediaFiles[0]
	if !file.Available {
		return "", fmt.Errorf("the media file of %q is not available", detail.Title)
	}
	return fmt.Sprintf("%q: %d Datei(en), primär %s %s/%s, %d ms",
		detail.Title, len(detail.MediaFiles), file.Container, file.VideoCodec, file.AudioCodec, file.DurationMs), nil
}

// checkWatchlist puts a title on the watchlist, finds it there and takes it off again.
func checkWatchlist(ctx context.Context, s *state) (string, error) {
	path := "/items/" + s.movie.ID + "/watchlist"
	if err := s.c.call(ctx, http.MethodPut, path, nil, nil, nil, http.StatusNoContent, http.StatusOK); err != nil {
		return "", err
	}
	var page struct {
		Items []itemRef `json:"items"`
		Total int       `json:"total"`
	}
	if err := s.c.get(ctx, "/watchlist", nil, &page); err != nil {
		return "", err
	}
	if !contains(page.Items, s.movie.ID) {
		return "", errors.New("the title is not on the watchlist")
	}
	detail, err := s.detail(ctx, s.movie.ID)
	if err != nil {
		return "", err
	}
	if !detail.InWatchlist {
		return "", errors.New("the detail does not show the title as on the watchlist")
	}
	if err := s.c.call(ctx, http.MethodDelete, path, nil, nil, nil, http.StatusNoContent, http.StatusOK); err != nil {
		return "", err
	}
	if err := s.c.get(ctx, "/watchlist", nil, &page); err != nil {
		return "", err
	}
	if contains(page.Items, s.movie.ID) {
		return "", errors.New("the title stayed on the watchlist")
	}
	return fmt.Sprintf("%q aufgenommen (Merkliste %d Einträge), Detail inWatchlist=true, wieder entfernt",
		s.movie.Title, page.Total+1), nil
}

// checkFavorites reads the favourites, removes one and marks it again. The fixtures already made
// the podcast a favourite, which is what gave the inbox its entry.
func checkFavorites(ctx context.Context, s *state) (string, error) {
	path := "/items/" + s.podcast.ID + "/favorite"
	var favorites []itemRef
	if err := s.c.get(ctx, "/favorites", nil, &favorites); err != nil {
		return "", err
	}
	if !contains(favorites, s.podcast.ID) {
		return "", errors.New("the podcast is not among the favourites")
	}
	if err := s.c.call(ctx, http.MethodDelete, path, nil, nil, nil, http.StatusNoContent, http.StatusOK); err != nil {
		return "", err
	}
	if err := s.c.get(ctx, "/favorites", nil, &favorites); err != nil {
		return "", err
	}
	if contains(favorites, s.podcast.ID) {
		return "", errors.New("the podcast stayed among the favourites")
	}
	if err := s.c.call(ctx, http.MethodPut, path, nil, nil, nil, http.StatusNoContent, http.StatusOK); err != nil {
		return "", err
	}
	return fmt.Sprintf("%q als Favorit gelesen, entfernt und wieder gesetzt", s.podcast.Title), nil
}

// directCapabilities is the capability profile of a browser that plays H.264 and AAC in MP4.
func directCapabilities() *playbackCapabilities {
	return &playbackCapabilities{
		Containers: []string{"mp4"},
		Video:      []string{"h264"},
		Audio:      []string{"aac"},
		HLS:        "mse",
		MaxWidth:   1920,
		MaxHeight:  1080,
		Name:       "apicheck P-16",
		Type:       "other",
	}
}

// checkPlaybackStart starts the same film twice with two capability profiles and shows that the
// profile decides between direct play and HLS.
func checkPlaybackStart(ctx context.Context, s *state) (string, error) {
	direct, err := s.play(ctx, s.movie.ID, playbackRequest{Capabilities: directCapabilities()})
	if err != nil {
		return "", err
	}
	defer s.stop(ctx, direct.PlaybackSessionID)
	if direct.Mode != "direct" {
		return "", fmt.Errorf("a device with MP4, H.264 and AAC got %q instead of direct play", direct.Mode)
	}

	webm := directCapabilities()
	webm.Containers = []string{"webm"} // the same codecs, but the MP4 container is out of reach
	hls, err := s.play(ctx, s.movie.ID, playbackRequest{Capabilities: webm})
	if err != nil {
		return "", err
	}
	defer s.stop(ctx, hls.PlaybackSessionID)
	if hls.Mode != "hls" {
		return "", fmt.Errorf("a device without the MP4 container got %q instead of HLS", hls.Mode)
	}
	if hls.Transcoding == nil {
		return "", errors.New("the HLS answer says nothing about how it is produced")
	}
	return fmt.Sprintf("MP4/H.264/AAC → direct; ohne MP4-Container → hls (%s, Grund %s); %d Qualitätsstufen",
		hls.Transcoding.Mode, hls.Transcoding.Reason, len(hls.Qualities)), nil
}

// checkStreams fetches the media itself: the file through the Range endpoint of a direct play and
// the playlists, the init segment and the first segment of an HLS playback — all authorised by
// the stream token alone.
func checkStreams(ctx context.Context, s *state) (string, error) {
	direct, err := s.play(ctx, s.movie.ID, playbackRequest{Capabilities: directCapabilities()})
	if err != nil {
		return "", err
	}
	defer s.stop(ctx, direct.PlaybackSessionID)

	const want = 65536
	header := http.Header{"Range": []string{fmt.Sprintf("bytes=0-%d", want-1)}}
	part, err := s.c.sendRaw(ctx, http.MethodGet, direct.StreamURL, header)
	if err != nil {
		return "", err
	}
	if err := part.expect(http.MethodGet, direct.StreamURL, http.StatusPartialContent); err != nil {
		return "", err
	}
	if len(part.body) != want {
		return "", fmt.Errorf("the Range request returned %d instead of %d bytes", len(part.body), want)
	}

	// Without the token the same path must not deliver anything (plan 5.4).
	bare := "/items/" + s.movie.ID + "/stream"
	denied, err := s.c.send(ctx, http.MethodGet, bare, nil, nil)
	if err != nil {
		return "", err
	}
	if err := denied.expect(http.MethodGet, bare, http.StatusUnauthorized); err != nil {
		return "", fmt.Errorf("the stream endpoint answered without a stream token: %w", err)
	}

	webm := directCapabilities()
	webm.Containers = []string{"webm"}
	hls, err := s.play(ctx, s.movie.ID, playbackRequest{Capabilities: webm})
	if err != nil {
		return "", err
	}
	defer s.stop(ctx, hls.PlaybackSessionID)

	master, err := s.text(ctx, hls.StreamURL)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(master, "#EXTM3U") {
		return "", errors.New("the master playlist is no M3U8")
	}
	mediaRef := firstURI(master)
	if mediaRef == "" {
		return "", errors.New("the master playlist names no media playlist")
	}
	mediaURL := resolve(hls.StreamURL, mediaRef)
	media, err := s.text(ctx, mediaURL)
	if err != nil {
		return "", err
	}
	initRef := attribute(media, "#EXT-X-MAP:", "URI")
	if initRef == "" {
		return "", errors.New("the media playlist names no init segment")
	}
	initSegment, err := s.bytes(ctx, resolve(mediaURL, initRef))
	if err != nil {
		return "", err
	}
	segmentRef := firstURI(media)
	if segmentRef == "" {
		return "", errors.New("the media playlist names no segment")
	}
	segment, err := s.bytes(ctx, resolve(mediaURL, segmentRef))
	if err != nil {
		return "", err
	}
	if len(initSegment) == 0 || len(segment) == 0 {
		return "", errors.New("init segment or first segment is empty")
	}
	return fmt.Sprintf("Direct Play 206 mit %d Bytes, ohne Token 401; HLS: Master, Medienliste, init.mp4 (%d Bytes), erstes Segment (%d Bytes)",
		len(part.body), len(initSegment), len(segment)), nil
}

// checkTracks chooses a non-default audio track and a subtitle track and fetches the WebVTT the
// server announces for it.
func checkTracks(ctx context.Context, s *state) (string, error) {
	var tracks trackList
	if err := s.c.get(ctx, "/items/"+s.multi.ID+"/tracks", nil, &tracks); err != nil {
		return "", err
	}
	if len(tracks.Audio) < 2 {
		return "", fmt.Errorf("%q carries %d audio tracks, two are needed", s.multi.Title, len(tracks.Audio))
	}
	var audio track
	chosen := false
	for _, t := range tracks.Audio {
		if !t.Default && !chosen {
			audio, chosen = t, true
		}
	}
	if !chosen {
		return "", errors.New("every audio track is the default one")
	}
	var subtitle track
	found := false
	for _, t := range tracks.Subtitles {
		if t.Text && !found {
			subtitle, found = t, true
		}
	}
	if !found {
		return "", errors.New("the file carries no text subtitle track")
	}
	index := audio.Index
	id := subtitle.ID
	info, err := s.play(ctx, s.multi.ID, playbackRequest{
		Capabilities:     directCapabilities(),
		AudioTrack:       &index,
		SubtitleTrack:    &id,
		SubtitleDelivery: "text",
	})
	if err != nil {
		return "", err
	}
	defer s.stop(ctx, info.PlaybackSessionID)
	if info.Selection.SubtitleTrack == nil || *info.Selection.SubtitleTrack != id {
		return "", errors.New("the playback does not deliver the chosen subtitle track")
	}
	if info.Selection.SubtitleDelivery != "text" {
		return "", fmt.Errorf("the subtitle is delivered as %q instead of text", info.Selection.SubtitleDelivery)
	}
	if info.SubtitleURL == "" {
		return "", errors.New("the playback names no subtitle URL")
	}
	vtt, err := s.text(ctx, info.SubtitleURL)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(vtt, "WEBVTT") {
		return "", errors.New("the subtitle track is no WebVTT")
	}
	return fmt.Sprintf("%d Ton-, %d Untertitelspuren; gewählt Ton %q (%s) und Untertitel %q (%s), WebVTT mit %d Zeichen",
		len(tracks.Audio), len(tracks.Subtitles), audio.Title, audio.Codec, subtitle.Title, subtitle.Codec, len(vtt)), nil
}

// checkProgress reports a playback position and reads it back from the detail.
func checkProgress(ctx context.Context, s *state) (string, error) {
	detail, err := s.detail(ctx, s.movie.ID)
	if err != nil {
		return "", err
	}
	duration := detail.MediaFiles[0].DurationMs
	if duration == 0 {
		return "", errors.New("the media file has no duration")
	}
	const position = 3000
	event := progressEvent{
		MediaFileID: detail.MediaFiles[0].ID,
		PositionMs:  position,
		DurationMs:  duration,
		PlayedMs:    position,
		Seq:         1,
		State:       "paused",
		ClientTime:  time.Now().UTC().Format(time.RFC3339),
	}
	var merged progress
	path := "/items/" + s.movie.ID + "/progress"
	if err := s.c.call(ctx, http.MethodPost, path, nil, event, &merged, http.StatusOK); err != nil {
		return "", err
	}
	if merged.PositionMs != position {
		return "", fmt.Errorf("the server merged the position to %d ms", merged.PositionMs)
	}
	again, err := s.detail(ctx, s.movie.ID)
	if err != nil {
		return "", err
	}
	if again.Progress == nil || again.Progress.PositionMs != position {
		return "", errors.New("the detail does not show the reported position")
	}
	return fmt.Sprintf("Position %d ms von %d ms gemeldet (%.1f %%) und im Detail wiedergefunden",
		merged.PositionMs, merged.DurationMs, merged.Percent), nil
}

// checkDownload prepares a download, waits for the manifest and fetches the file with the
// download token.
func checkDownload(ctx context.Context, s *state) (string, error) {
	var options downloadOptions
	if err := s.c.get(ctx, "/items/"+s.movie.ID+"/download-options", nil, &options); err != nil {
		return "", err
	}
	if len(options.Qualities) == 0 {
		return "", errors.New("the title offers no download profile")
	}
	var created downloadList
	body := downloadRequest{Quality: "low"}
	path := "/items/" + s.movie.ID + "/downloads"
	if err := s.c.call(ctx, http.MethodPost, path, nil, body, &created, http.StatusCreated); err != nil {
		return "", err
	}
	if len(created.Downloads) != 1 {
		return "", fmt.Errorf("the request created %d downloads", len(created.Downloads))
	}
	download := created.Downloads[0]
	manifest, err := s.waitForDownload(ctx, download.ID)
	if err != nil {
		return "", err
	}
	if manifest.InitLength == 0 || len(manifest.Segments) == 0 {
		return "", errors.New("the manifest names no init segment or no media segment")
	}
	header := http.Header{"Range": []string{fmt.Sprintf("bytes=0-%d", manifest.InitLength-1)}}
	resp, err := s.c.sendRaw(ctx, http.MethodGet, manifest.FileURL, header)
	if err != nil {
		return "", err
	}
	if err := resp.expect(http.MethodGet, manifest.FileURL, http.StatusPartialContent); err != nil {
		return "", err
	}
	if int64(len(resp.body)) != manifest.InitLength {
		return "", fmt.Errorf("the init segment returned %d instead of %d bytes", len(resp.body), manifest.InitLength)
	}
	s.downloadID = download.ID
	return fmt.Sprintf("Profil low vorbereitet: %d Bytes, %d Segmente, Codecs %s; init.mp4 (%d Bytes) mit Download-Token geholt; sichtbar für die Profile %v",
		manifest.SizeBytes, len(manifest.Segments), manifest.Codecs, manifest.InitLength, download.AllowedProfileIDs), nil
}

// waitForDownload polls the manifest until the server prepared the file.
func (s *state) waitForDownload(ctx context.Context, id string) (downloadManifest, error) {
	path := "/downloads/" + id + "/manifest"
	deadline := time.Now().Add(downloadTimeout)
	for {
		resp, err := s.c.send(ctx, http.MethodGet, path, nil, nil)
		if err != nil {
			return downloadManifest{}, err
		}
		if resp.status == http.StatusOK {
			var manifest downloadManifest
			if err := unmarshal(resp.body, &manifest); err != nil {
				return downloadManifest{}, err
			}
			return manifest, nil
		}
		if resp.status != http.StatusConflict {
			return downloadManifest{}, resp.expect(http.MethodGet, path, http.StatusOK, http.StatusConflict)
		}
		if time.Now().After(deadline) {
			return downloadManifest{}, fmt.Errorf("the download was not ready within %s", downloadTimeout)
		}
		if err := sleep(ctx, pollInterval); err != nil {
			return downloadManifest{}, err
		}
	}
}

// checkOfflineEvents uploads the progress a device buffered while it played a download.
func checkOfflineEvents(ctx context.Context, s *state) (string, error) {
	if s.downloadID == "" {
		return "", errors.New("no prepared download: the download check has to run first")
	}
	detail, err := s.detail(ctx, s.movie.ID)
	if err != nil {
		return "", err
	}
	playback, err := newPlaybackID()
	if err != nil {
		return "", err
	}
	const position = 7000
	batch := offlineEventBatch{
		SentAt: time.Now().UTC().Format(time.RFC3339),
		Events: []offlineEvent{{
			ProfileID:  s.profileID,
			ItemID:     s.movie.ID,
			PlaybackID: playback,
			DownloadID: s.downloadID,
			PositionMs: position,
			DurationMs: detail.MediaFiles[0].DurationMs,
			PlayedMs:   2000,
			Seq:        1,
			State:      "paused",
			ClientTime: time.Now().UTC().Format(time.RFC3339),
		}},
	}
	var result offlineEventResult
	err = s.c.call(ctx, http.MethodPost, "/offline/events", nil, batch, &result, http.StatusOK)
	if err != nil {
		return "", err
	}
	if result.Accepted != 1 || result.Rejected != 0 {
		return "", fmt.Errorf("the server answered %d accepted, %d duplicates, %d rejected",
			result.Accepted, result.Duplicates, result.Rejected)
	}
	again, err := s.detail(ctx, s.movie.ID)
	if err != nil {
		return "", err
	}
	if again.Progress == nil || again.Progress.PositionMs != position {
		return "", errors.New("the offline position did not reach the stored progress")
	}
	return fmt.Sprintf("1 Ereignis hochgeladen (%s), Ergebnis %q, gespeicherte Position %d ms",
		playback, strings.Join(result.Results, ","), again.Progress.PositionMs), nil
}

// checkNotifications reads the inbox and marks it read. The fixtures created the entry through a
// favourite podcast that got a new episode (E-8).
func checkNotifications(ctx context.Context, s *state) (string, error) {
	var page notificationPage
	if err := s.c.get(ctx, "/notifications", query("limit", "50"), &page); err != nil {
		return "", err
	}
	if len(page.Notifications) == 0 {
		return "", errors.New("the inbox is empty although a new episode appeared")
	}
	first := page.Notifications[0]
	read := "/notifications/" + strconv.FormatInt(first.ID, 10) + "/read"
	if err := s.c.call(ctx, http.MethodPut, read, nil, nil, nil, http.StatusNoContent, http.StatusOK); err != nil {
		return "", err
	}
	if err := s.c.call(ctx, http.MethodPost, "/notifications/read-all", nil, nil, nil, http.StatusNoContent, http.StatusOK); err != nil {
		return "", err
	}
	var after notificationPage
	if err := s.c.get(ctx, "/notifications", nil, &after); err != nil {
		return "", err
	}
	if after.Unread != 0 {
		return "", fmt.Errorf("%d notifications stayed unread", after.Unread)
	}
	return fmt.Sprintf("%d Benachrichtigung(en), neueste %q zu %q; einzeln und alle als gelesen markiert",
		page.Total, first.Kind, first.Item.Title), nil
}

// checkAnnouncements reads the open announcements and acknowledges one.
func checkAnnouncements(ctx context.Context, s *state) (string, error) {
	var open []announcement
	if err := s.c.get(ctx, "/announcements", nil, &open); err != nil {
		return "", err
	}
	var mine announcement
	for _, a := range open {
		if a.ID == s.announce {
			mine = a
		}
	}
	if mine.ID == 0 {
		return "", errors.New("the published announcement is not among the open ones")
	}
	ack := "/announcements/" + strconv.FormatInt(mine.ID, 10) + "/ack"
	if err := s.c.call(ctx, http.MethodPost, ack, nil, nil, nil, http.StatusNoContent, http.StatusOK); err != nil {
		return "", err
	}
	var after []announcement
	if err := s.c.get(ctx, "/announcements", nil, &after); err != nil {
		return "", err
	}
	for _, a := range after {
		if a.ID == mine.ID {
			return "", errors.New("the acknowledged announcement is still open")
		}
	}
	return fmt.Sprintf("%d offene Ankündigung(en), %q gelesen und bestätigt", len(open), mine.Title), nil
}

// checkTrailers reads the trailer info of a film.
func checkTrailers(ctx context.Context, s *state) (string, error) {
	var trailers []trailer
	if err := s.c.get(ctx, "/items/"+s.movie.ID+"/trailers", nil, &trailers); err != nil {
		return "", err
	}
	if len(trailers) == 0 {
		return "", errors.New("the film has no trailer")
	}
	first := trailers[0]
	if !strings.Contains(first.EmbedURL, "youtube-nocookie.com/embed/") {
		return "", fmt.Errorf("the trailer points at %q instead of the IFrame player", first.EmbedURL)
	}
	return fmt.Sprintf("%d Trailer, erster %s/%s über %s",
		len(trailers), first.Lang, first.YoutubeID, first.EmbedURL), nil
}

// checkAudiobookChapters reads the chapters of an audiobook, from the detail and from a playback.
func checkAudiobookChapters(ctx context.Context, s *state) (string, error) {
	detail, err := s.detail(ctx, s.audiobook.ID)
	if err != nil {
		return "", err
	}
	if len(detail.Chapters) < 2 {
		return "", fmt.Errorf("%q has %d chapters", detail.Title, len(detail.Chapters))
	}
	info, err := s.play(ctx, s.audiobook.ID, playbackRequest{Capabilities: directCapabilities()})
	if err != nil {
		return "", err
	}
	defer s.stop(ctx, info.PlaybackSessionID)
	if info.MediaKind != "audio" {
		return "", fmt.Errorf("the audiobook plays as %q", info.MediaKind)
	}
	if len(info.Chapters) < 2 {
		return "", fmt.Errorf("the playback carries %d chapters", len(info.Chapters))
	}
	return fmt.Sprintf("%q: %d Kapitel im Detail, %d auf der Abspielzeitachse, erstes %q ab %d ms, Modus %s",
		detail.Title, len(detail.Chapters), len(info.Chapters),
		detail.Chapters[0].Title, detail.Chapters[0].StartMs, info.Mode), nil
}

// checkPodcastEpisodes reads the podcast with its episodes and pages through them.
func checkPodcastEpisodes(ctx context.Context, s *state) (string, error) {
	detail, err := s.detail(ctx, s.podcast.ID)
	if err != nil {
		return "", err
	}
	if len(detail.Episodes) == 0 {
		return "", errors.New("the podcast detail names no episode")
	}
	var page struct {
		Episodes []itemRef `json:"episodes"`
		Total    int       `json:"total"`
		Offset   int       `json:"offset"`
		Limit    int       `json:"limit"`
	}
	path := "/items/" + s.podcast.ID + "/episodes"
	if err := s.c.get(ctx, path, query("offset", "0", "limit", "2"), &page); err != nil {
		return "", err
	}
	if len(page.Episodes) == 0 || page.Total < len(detail.Episodes) {
		return "", fmt.Errorf("the paged list answered %d of %d episodes", len(page.Episodes), page.Total)
	}
	return fmt.Sprintf("%q: %d Episoden im Detail, Seite mit %d von %d, neueste %q",
		detail.Title, len(detail.Episodes), len(page.Episodes), page.Total, page.Episodes[0].Title), nil
}

// detail reads the detail of a title.
func (s *state) detail(ctx context.Context, id string) (itemDetail, error) {
	var detail itemDetail
	if err := s.c.get(ctx, "/items/"+id, nil, &detail); err != nil {
		return itemDetail{}, err
	}
	return detail, nil
}

// play starts a playback.
func (s *state) play(ctx context.Context, id string, req playbackRequest) (playbackInfo, error) {
	var info playbackInfo
	path := "/items/" + id + "/play"
	if err := s.c.call(ctx, http.MethodPost, path, nil, req, &info, http.StatusOK); err != nil {
		return playbackInfo{}, err
	}
	return info, nil
}

// stop ends a playback session, so its stream token dies and the transcode budget is free again.
func (s *state) stop(ctx context.Context, id string) {
	if id == "" {
		return
	}
	path := "/playback/sessions/" + id
	_ = s.c.call(ctx, http.MethodDelete, path, nil, nil, nil, http.StatusNoContent, http.StatusOK, http.StatusNotFound)
}

// text fetches a URL the server handed out and returns its body as text.
func (s *state) text(ctx context.Context, raw string) (string, error) {
	body, err := s.bytes(ctx, raw)
	return string(body), err
}

// bytes fetches a URL the server handed out.
func (s *state) bytes(ctx context.Context, raw string) ([]byte, error) {
	resp, err := s.c.sendRaw(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	if err := resp.expect(http.MethodGet, raw, http.StatusOK, http.StatusPartialContent); err != nil {
		return nil, err
	}
	return resp.body, nil
}

// contains reports whether the list holds the title.
func contains(items []itemRef, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

// firstURI returns the first line of a playlist that is not a comment.
func firstURI(playlist string) string {
	for _, line := range strings.Split(playlist, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			return line
		}
	}
	return ""
}

// attribute returns the value of a quoted attribute of the first playlist tag with the prefix,
// for example URI of #EXT-X-MAP.
func attribute(playlist, tag, name string) string {
	for _, line := range strings.Split(playlist, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, tag) {
			continue
		}
		key := name + `="`
		start := strings.Index(line, key)
		if start < 0 {
			return ""
		}
		rest := line[start+len(key):]
		if end := strings.Index(rest, `"`); end >= 0 {
			return rest[:end]
		}
	}
	return ""
}

// resolve joins a playlist entry with the URL of the playlist it came from.
func resolve(base, ref string) string {
	if strings.HasPrefix(ref, "/") || strings.Contains(ref, "://") {
		return ref
	}
	path := base
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		path = path[:i]
	}
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[:i+1] + ref
	}
	return ref
}

// crockford is the alphabet of a ULID, as the contract's pattern for playback ids spells it.
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// newPlaybackID returns an id for one offline playback: 26 characters of the ULID alphabet, the
// first one low enough to keep the timestamp part in range.
func newPlaybackID() (string, error) {
	raw := make([]byte, 26)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("random playback id: %w", err)
	}
	id := make([]byte, 26)
	for i, b := range raw {
		id[i] = crockford[int(b)%len(crockford)]
	}
	id[0] = crockford[int(raw[0])%8]
	return string(id), nil
}
