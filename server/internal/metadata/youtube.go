package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
)

// YouTube Data API v3 (plan 5.3, design 3.2): the second step of the trailer chain, for de and en
// when TMDB has no trailer. Only video IDs are stored; nothing is ever downloaded from YouTube.
const (
	DefaultYouTubeBase = "https://www.googleapis.com/youtube/v3"
	youtubeDailyUnits  = 10000 // free daily quota, at most 100 searches
	youtubeSearchCost  = 100
	youtubeVideosCost  = 1
	youtubeMaxResults  = 5
	minTrailerSeconds  = 60
	maxTrailerSeconds  = 300
)

var (
	// errQuotaExhausted: the daily YouTube quota is spent; the title stays pending until the next
	// run after the reset.
	errQuotaExhausted = errors.New("youtube: daily quota exhausted")
	// errYouTubeKeyInvalid: Google rejected the key; YouTube is skipped.
	errYouTubeKeyInvalid = errors.New("youtube: api key rejected")
)

// youtubeTrailers searches trailers on YouTube. The API key travels in the query parameter key
// (the only way the Data API accepts a plain API key); the logging handler masks it, and errors
// of this client never carry the request URL.
type youtubeTrailers struct {
	base    string
	http    *http.Client
	limiter *ratelimit.Limiter
	quota   *ratelimit.Quota
	keys    KeySource
}

func newYouTubeTrailers(base string, hc *http.Client, quota *ratelimit.Quota, keys KeySource) *youtubeTrailers {
	if hc == nil {
		hc = &http.Client{Timeout: httpTimeout}
	}
	return &youtubeTrailers{base: strings.TrimRight(base, "/"), http: hc, limiter: ratelimit.New("youtube", 5, 5), quota: quota, keys: keys}
}

func (y *youtubeTrailers) Name() string { return sourceYouTube }

// FindTrailers searches one trailer per language of q (de and en only). It stops with
// errQuotaExhausted when the quota runs out and returns the hits found until then.
func (y *youtubeTrailers) FindTrailers(ctx context.Context, q TrailerQuery) ([]TrailerHit, error) {
	key, err := y.keys.YouTubeKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("read youtube key: %w", err)
	}
	if key == "" {
		return nil, errNotConfigured
	}
	var hits []TrailerHit
	for _, lang := range q.Langs {
		if lang != "de" && lang != "en" {
			continue
		}
		hit, ok, err := y.find(ctx, key, q, lang)
		if err != nil {
			return hits, err
		}
		if ok {
			hits = append(hits, hit)
		}
	}
	return hits, nil
}

// find runs search.list and videos.list for one language and picks the best video.
func (y *youtubeTrailers) find(ctx context.Context, key string, q TrailerQuery, lang string) (TrailerHit, bool, error) {
	title := firstNonEmpty(q.Titles[lang], q.Titles["en"], q.OriginalTitle)
	if title == "" {
		return TrailerHit{}, false, nil
	}
	query := title
	if q.Year > 0 {
		query += " " + strconv.Itoa(q.Year)
	}
	query += " trailer"
	if lang == "de" {
		query += " deutsch"
	}
	if !y.quota.Take(ctx, youtubeSearchCost) {
		return TrailerHit{}, false, errQuotaExhausted
	}
	var search struct {
		Items []struct {
			ID struct {
				VideoID string `json:"videoId"`
			} `json:"id"`
		} `json:"items"`
	}
	err := y.getJSON(ctx, "/search", url.Values{
		"part": {"snippet"}, "q": {query}, "type": {"video"}, "videoDuration": {"short"},
		"relevanceLanguage": {lang}, "maxResults": {strconv.Itoa(youtubeMaxResults)}, "key": {key},
	}, &search)
	if err != nil {
		return TrailerHit{}, false, err
	}
	var ids []string
	for _, it := range search.Items {
		if youtubeIDPattern.MatchString(it.ID.VideoID) {
			ids = append(ids, it.ID.VideoID)
		}
	}
	if len(ids) == 0 {
		return TrailerHit{}, false, nil
	}
	if !y.quota.Take(ctx, youtubeVideosCost) {
		return TrailerHit{}, false, errQuotaExhausted
	}
	var videos struct {
		Items []youtubeVideo `json:"items"`
	}
	err = y.getJSON(ctx, "/videos", url.Values{
		"part": {"contentDetails,snippet"}, "id": {strings.Join(ids, ",")}, "key": {key},
	}, &videos)
	if err != nil {
		return TrailerHit{}, false, err
	}
	rank := map[string]int{}
	for i, id := range ids {
		rank[id] = i
	}
	best, ok := pickYouTubeVideo(videos.Items, rank, []string{title, q.Titles["en"], q.OriginalTitle}, lang)
	if !ok {
		return TrailerHit{}, false, nil
	}
	return TrailerHit{Lang: lang, YouTubeID: best.ID, Title: best.Snippet.Title, Source: sourceYouTube,
		Official: officialChannel(best, []string{title, q.OriginalTitle})}, true, nil
}

type youtubeVideo struct {
	ID      string `json:"id"`
	Snippet struct {
		Title        string `json:"title"`
		ChannelTitle string `json:"channelTitle"`
	} `json:"snippet"`
	ContentDetails struct {
		Duration string `json:"duration"`
	} `json:"contentDetails"`
}

// pickYouTubeVideo keeps the videos of 60 to 300 seconds whose title names one of the titles and
// "trailer", and prefers an official channel, then a video title in the language, then the search
// rank.
func pickYouTubeVideo(items []youtubeVideo, rank map[string]int, titles []string, lang string) (youtubeVideo, bool) {
	type scored struct {
		v     youtubeVideo
		score int
	}
	var keep []scored
	for _, v := range items {
		secs, ok := parseISODuration(v.ContentDetails.Duration)
		if !ok || secs < minTrailerSeconds || secs > maxTrailerSeconds || !youtubeIDPattern.MatchString(v.ID) {
			continue
		}
		name := normalizeTitle(v.Snippet.Title)
		if !strings.Contains(name, "trailer") || !containsAny(name, titles) {
			continue
		}
		score := 0
		if officialChannel(v, titles) {
			score += 2
		}
		german := containsAny(name, []string{"deutsch", "german"})
		if (lang == "de") == german {
			score++
		}
		keep = append(keep, scored{v, score})
	}
	if len(keep) == 0 {
		return youtubeVideo{}, false
	}
	sort.SliceStable(keep, func(i, j int) bool {
		if keep[i].score != keep[j].score {
			return keep[i].score > keep[j].score
		}
		return rank[keep[i].v.ID] < rank[keep[j].v.ID]
	})
	return keep[0].v, true
}

// officialChannel reports whether the channel looks like the studio's: "official"/"offiziell" in
// its name or the title itself.
func officialChannel(v youtubeVideo, titles []string) bool {
	ch := normalizeTitle(v.Snippet.ChannelTitle)
	return containsAny(ch, []string{"official", "offiziell"}) || containsAny(ch, titles)
}

func containsAny(normalized string, needles []string) bool {
	for _, n := range needles {
		if n = normalizeTitle(n); n != "" && strings.Contains(" "+normalized+" ", " "+n+" ") {
			return true
		}
	}
	return false
}

// normalizeTitle lower-cases s and turns every run of characters other than letters and digits
// into one blank.
func normalizeTitle(s string) string {
	var b strings.Builder
	blank := true
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			blank = false
			continue
		}
		if !blank {
			b.WriteByte(' ')
			blank = true
		}
	}
	return strings.TrimSpace(b.String())
}

// isoDuration matches the ISO 8601 durations of the Data API (PT2M31S, PT1H2M, P1DT2H).
var isoDuration = regexp.MustCompile(`^P(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?)?$`)

func parseISODuration(s string) (int, bool) {
	m := isoDuration.FindStringSubmatch(s)
	if m == nil || s == "P" || s == "PT" {
		return 0, false
	}
	total := 0
	for i, mult := range []int{0, 86400, 3600, 60, 1} {
		if i == 0 || m[i] == "" {
			continue
		}
		n, err := strconv.Atoi(m[i])
		if err != nil {
			return 0, false
		}
		total += n * mult
	}
	return total, true
}

func (y *youtubeTrailers) getJSON(ctx context.Context, path string, q url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, y.base+path+"?"+q.Encode(), nil)
	if err != nil {
		return fmt.Errorf("youtube request %s: %w", path, errors.New("invalid request"))
	}
	req.Header.Set("Accept", "application/json")
	resp, err := y.limiter.Do(ctx, y.http, req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		// *url.Error carries the URL with the key: keep only the cause.
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return fmt.Errorf("youtube %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxJSONBytes))
	if err != nil {
		return fmt.Errorf("youtube %s: read: %w", path, err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("youtube %s: decode: %w", path, err)
		}
		return nil
	}
	var problem struct {
		Error struct {
			Errors []struct {
				Reason string `json:"reason"`
			} `json:"errors"`
			Details []struct {
				Reason string `json:"reason"`
			} `json:"details"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &problem)
	reasons := map[string]bool{}
	for _, e := range problem.Error.Errors {
		reasons[e.Reason] = true
	}
	for _, d := range problem.Error.Details {
		reasons[d.Reason] = true
	}
	switch {
	case reasons["quotaExceeded"] || reasons["dailyLimitExceeded"]:
		y.quota.Exhaust()
		return errQuotaExhausted
	case reasons["keyInvalid"] || reasons["API_KEY_INVALID"] || reasons["accessNotConfigured"] || reasons["forbidden"]:
		return fmt.Errorf("%w: status %d", errYouTubeKeyInvalid, resp.StatusCode)
	default:
		return fmt.Errorf("youtube %s: status %d", path, resp.StatusCode)
	}
}
