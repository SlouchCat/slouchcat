// Package youtubeid reads YouTube video ids from the URLs and ids an administrator enters for a
// manual trailer (B-8). Packages admin (input validation) and metadata (storage) share it, so
// both accept the same forms.
package youtubeid

import (
	"net/url"
	"regexp"
	"strings"
)

// pattern is the shape of a YouTube video id (trailers.youtube_id, 8 to 16 characters).
var pattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,16}$`)

// Valid reports whether id has the shape of a YouTube video id.
func Valid(id string) bool { return pattern.MatchString(id) }

// Parse returns the video id of a YouTube URL or of a bare id. URLs: youtube.com/watch?v=,
// youtu.be/<id>, youtube.com/embed|shorts|live|v/<id>, music.youtube.com/watch?v=,
// youtube-nocookie.com/embed/<id>; with or without scheme (http or https), www. and m.
func Parse(urlOrID string) (string, bool) {
	in := strings.TrimSpace(urlOrID)
	if Valid(in) {
		return in, true
	}
	if !strings.Contains(in, "://") {
		in = "https://" + in
	}
	u, err := url.Parse(in)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return "", false
	}
	host := strings.TrimPrefix(strings.TrimPrefix(strings.ToLower(u.Hostname()), "www."), "m.")
	segs := strings.Split(strings.Trim(u.Path, "/"), "/")
	id := ""
	switch host {
	case "youtu.be":
		id = segs[0]
	case "youtube.com", "music.youtube.com":
		switch {
		case segs[0] == "watch":
			id = u.Query().Get("v")
		case len(segs) == 2 && (segs[0] == "embed" || segs[0] == "shorts" || segs[0] == "live" || segs[0] == "v"):
			id = segs[1]
		}
	case "youtube-nocookie.com":
		if len(segs) == 2 && segs[0] == "embed" {
			id = segs[1]
		}
	}
	if !Valid(id) {
		return "", false
	}
	return id, true
}
