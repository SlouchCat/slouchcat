package arr

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// The calls of the client (architecture 2.6, 08-datenmodell-api 6). There are no others.

// Tags returns GET /api/v3/tag. It is also the connection test: 200 means reachable with a valid
// key, ErrKeyInvalid a wrong key (08-datenmodell-api 6, first row).
func (c *Client) Tags(ctx context.Context) ([]Tag, error) {
	var out []Tag
	err := c.call(ctx, http.MethodGet, "/tag", nil, nil, &out)
	return out, err
}

// EnsureTag returns the tag with label and whether it was created. label must be valid
// (ValidLabel; Radarr refuses anything else). It reads the tag list first and compares in lower
// case; only a missing label is created with POST /api/v3/tag. Creating is idempotent on both
// services as well (I-50): a concurrent run that creates the same label gets the same id.
func (c *Client) EnsureTag(ctx context.Context, label string) (Tag, bool, error) {
	if err := ValidLabel(label); err != nil {
		return Tag{}, false, c.fail("POST "+apiPrefix+"/tag", 0, "", []error{ErrInvalid}, err)
	}
	tags, err := c.Tags(ctx)
	if err != nil {
		return Tag{}, false, err
	}
	for _, t := range tags {
		if strings.ToLower(t.Label) == label {
			return t, false, nil
		}
	}
	var out Tag
	if err := c.call(ctx, http.MethodPost, "/tag", nil, Tag{Label: label}, &out); err != nil {
		return Tag{}, false, err
	}
	if out.ID < 1 {
		return Tag{}, false, c.fail("POST "+apiPrefix+"/tag", http.StatusCreated, "", []error{ErrServer}, errors.New("answer has no tag id"))
	}
	return out, true, nil
}

// MovieByTMDB returns the Radarr movie with the TMDB id from GET /api/v3/movie?tmdbId=. An empty
// answer is ErrNotFound: the movie is not yet in Radarr. Radarr clients only.
func (c *Client) MovieByTMDB(ctx context.Context, tmdbID int) (Movie, error) {
	const op = "GET " + apiPrefix + "/movie"
	if c.kind != Radarr || tmdbID < 1 {
		return Movie{}, c.fail(op, 0, "", []error{ErrInvalid}, errors.New("movies only on radarr, with a tmdb id"))
	}
	var raw []struct {
		ID      int   `json:"id"`
		TMDBID  int   `json:"tmdbId"`
		Tags    []int `json:"tags"`
		HasFile bool  `json:"hasFile"`
	}
	if err := c.call(ctx, http.MethodGet, "/movie", url.Values{"tmdbId": {strconv.Itoa(tmdbID)}}, nil, &raw); err != nil {
		return Movie{}, err
	}
	for _, m := range raw { // the TMDB id is compared as well: an old Radarr may ignore the filter
		if m.TMDBID == tmdbID && m.ID > 0 {
			return Movie{ID: m.ID, TMDBID: m.TMDBID, Tags: m.Tags, HasFile: m.HasFile}, nil
		}
	}
	return Movie{}, c.fail(op, http.StatusOK, "", []error{ErrNotFound}, errors.New("no movie with this tmdb id"))
}

// SeriesByTVDB returns the Sonarr series with the TVDB id from GET /api/v3/series?tvdbId=. An
// empty answer is ErrNotFound: the series is not yet in Sonarr. Sonarr clients only.
func (c *Client) SeriesByTVDB(ctx context.Context, tvdbID int) (Series, error) {
	const op = "GET " + apiPrefix + "/series"
	if c.kind != Sonarr || tvdbID < 1 {
		return Series{}, c.fail(op, 0, "", []error{ErrInvalid}, errors.New("series only on sonarr, with a tvdb id"))
	}
	var raw []struct {
		ID         int        `json:"id"`
		TVDBID     int        `json:"tvdbId"`
		Tags       []int      `json:"tags"`
		Statistics Statistics `json:"statistics"`
		Seasons    []struct {
			SeasonNumber int         `json:"seasonNumber"`
			Monitored    bool        `json:"monitored"`
			Statistics   *Statistics `json:"statistics"`
		} `json:"seasons"`
	}
	if err := c.call(ctx, http.MethodGet, "/series", url.Values{"tvdbId": {strconv.Itoa(tvdbID)}}, nil, &raw); err != nil {
		return Series{}, err
	}
	for _, s := range raw {
		if s.TVDBID != tvdbID || s.ID < 1 {
			continue
		}
		out := Series{ID: s.ID, TVDBID: s.TVDBID, Tags: s.Tags, Statistics: s.Statistics}
		for _, se := range s.Seasons {
			out.Seasons = append(out.Seasons, Season{Number: se.SeasonNumber, Monitored: se.Monitored, Statistics: se.Statistics})
		}
		return out, nil
	}
	return Series{}, c.fail(op, http.StatusOK, "", []error{ErrNotFound}, errors.New("no series with this tvdb id"))
}

// applyTagsAdd is the only mode the editor is sent with: never "remove", never "replace" (N-26).
const applyTagsAdd = "add"

// AttachTag adds the tag to the titles (Radarr movie ids or Sonarr series ids) with
// PUT /api/v3/movie/editor resp. /api/v3/series/editor and applyTags "add"; the other tags of the
// titles stay. Not verified against an instance (appendix I, writing calls).
func (c *Client) AttachTag(ctx context.Context, tagID int, titleIDs ...int) error {
	path, idsKey := "/movie/editor", "movieIds"
	if c.kind == Sonarr {
		path, idsKey = "/series/editor", "seriesIds"
	}
	if tagID < 1 || len(titleIDs) == 0 {
		return c.fail("PUT "+apiPrefix+path, 0, "", []error{ErrInvalid}, errors.New("tag id and title ids required"))
	}
	for _, id := range titleIDs {
		if id < 1 {
			return c.fail("PUT "+apiPrefix+path, 0, "", []error{ErrInvalid}, errors.New("title id below 1"))
		}
	}
	body := map[string]any{idsKey: titleIDs, "tags": []int{tagID}, "applyTags": applyTagsAdd}
	return c.call(ctx, http.MethodPut, path, nil, body, nil)
}

// Queue paging (K-8): the client asks for QueuePageSize records per page and reads pages until
// totalRecords are read. The largest pageSize the services accept is not verified, so the client
// follows the pages instead of relying on one large page.
const (
	QueuePageSize = 200
	maxQueuePages = 50
)

type queueRecordJSON struct {
	ID                      int      `json:"id"`
	MovieID                 *int     `json:"movieId"`
	SeriesID                *int     `json:"seriesId"`
	EpisodeID               *int     `json:"episodeId"`
	SeasonNumber            *int     `json:"seasonNumber"`
	Size                    *float64 `json:"size"`
	SizeLeft                *float64 `json:"sizeleft"`
	TimeLeft                string   `json:"timeleft"`
	EstimatedCompletionTime string   `json:"estimatedCompletionTime"`
	Status                  string   `json:"status"`
	TrackedDownloadState    string   `json:"trackedDownloadState"`
	TrackedDownloadStatus   string   `json:"trackedDownloadStatus"`
	ErrorMessage            string   `json:"errorMessage"`
	StatusMessages          []struct {
		Title    string   `json:"title"`
		Messages []string `json:"messages"`
	} `json:"statusMessages"`
	DownloadClient string `json:"downloadClient"`
	Protocol       string `json:"protocol"`
}

// Queue reads GET /api/v3/queue completely, page by page (K-8). Records without movieId (Radarr)
// resp. seriesId (Sonarr) are counted in Queue.Skipped and left out. Output paths are not read.
func (c *Client) Queue(ctx context.Context) (Queue, error) {
	var out Queue
	seen := map[int]bool{}
	read := 0
	for page := 1; ; page++ {
		if page > maxQueuePages {
			return Queue{}, c.fail("GET "+apiPrefix+"/queue", http.StatusOK, "", []error{ErrServer}, fmt.Errorf("queue longer than %d pages", maxQueuePages))
		}
		var p struct {
			Page         int               `json:"page"`
			PageSize     int               `json:"pageSize"`
			TotalRecords int               `json:"totalRecords"`
			Records      []queueRecordJSON `json:"records"`
		}
		q := url.Values{"page": {strconv.Itoa(page)}, "pageSize": {strconv.Itoa(QueuePageSize)}}
		if err := c.call(ctx, http.MethodGet, "/queue", q, nil, &p); err != nil {
			return Queue{}, err
		}
		out.Total = p.TotalRecords
		for _, r := range p.Records {
			read++
			if r.ID != 0 && seen[r.ID] { // the queue moved between two pages
				continue
			}
			seen[r.ID] = true
			rec, ok := c.queueRecord(r)
			if !ok {
				out.Skipped++
				continue
			}
			out.Records = append(out.Records, rec)
		}
		if len(p.Records) == 0 || read >= p.TotalRecords {
			return out, nil
		}
	}
}

func (c *Client) queueRecord(r queueRecordJSON) (QueueRecord, bool) {
	title := r.MovieID
	if c.kind == Sonarr {
		title = r.SeriesID
	}
	if title == nil || *title < 1 {
		return QueueRecord{}, false
	}
	rec := QueueRecord{
		ID: r.ID, TitleID: *title,
		Status: r.Status, TrackedDownloadState: r.TrackedDownloadState, TrackedDownloadStatus: r.TrackedDownloadStatus,
		ErrorMessage: cleanMessage(r.ErrorMessage), DownloadClient: cleanMessage(r.DownloadClient), Protocol: r.Protocol,
	}
	if c.kind == Sonarr {
		if r.EpisodeID != nil {
			rec.EpisodeID = *r.EpisodeID
		}
		if r.SeasonNumber != nil && *r.SeasonNumber >= 0 {
			rec.SeasonNumber, rec.SeasonKnown = *r.SeasonNumber, true
		}
	}
	if r.Size != nil && r.SizeLeft != nil && *r.Size > 0 && *r.SizeLeft >= 0 {
		rec.Size, rec.SizeLeft, rec.SizeKnown = int64(*r.Size), int64(*r.SizeLeft), true
	}
	rec.TimeLeft, rec.TimeLeftKnown = ParseTimeLeft(r.TimeLeft)
	if t, err := time.Parse(time.RFC3339Nano, r.EstimatedCompletionTime); err == nil {
		rec.EstimatedEnd = t
	}
	for _, m := range r.StatusMessages {
		sm := StatusMessage{Title: cleanMessage(m.Title)}
		for _, line := range m.Messages {
			sm.Messages = append(sm.Messages, cleanMessage(line))
		}
		rec.StatusMessages = append(rec.StatusMessages, sm)
	}
	return rec, true
}
