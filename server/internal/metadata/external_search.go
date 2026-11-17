package metadata

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// The search for titles outside the library (concept 25, F-11, architecture 16): the library
// search asks TMDB with Slouchcat's own key. This file returns TMDB's hits as they came; package
// wishes lets visibility decide which of them a profile sees and fetches the details of the first
// ones. The raw hits are kept 10 minutes per medium and query.

const externalSearchTTL = 10 * time.Minute

// ExternalHit is one hit of a TMDB search: TMDB's English title and the year.
type ExternalHit struct {
	Media         string // MediaMovie or MediaShow
	TMDBID        int64
	Title         string
	OriginalTitle string
	Year          int // 0 = unknown
}

// externalSearchCache keeps the raw hits per medium and query.
type externalSearchCache struct {
	mu      sync.Mutex
	entries map[string]externalSearchEntry
}

type externalSearchEntry struct {
	hits []ExternalHit
	at   time.Time
}

func (c *externalSearchCache) get(key string, now time.Time) ([]ExternalHit, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || now.Sub(e.at) >= externalSearchTTL {
		return nil, false
	}
	return e.hits, true
}

func (c *externalSearchCache) put(key string, hits []ExternalHit, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[string]externalSearchEntry{}
	}
	for k, e := range c.entries { // the old ones go on every write; the map stays small
		if now.Sub(e.at) >= externalSearchTTL {
			delete(c.entries, k)
		}
	}
	c.entries[key] = externalSearchEntry{hits: hits, at: now}
}

// SearchExternalTitles asks TMDB for films (media movie), series (show) or both (""), films first,
// each in TMDB's order (the first page). It answers model.ErrProviderNotConfigured without a TMDB
// key, model.ErrTMDBKeyInvalid and model.ErrTMDBUnreachable as TMDB answered.
func (s *Service) SearchExternalTitles(ctx context.Context, media, query string) ([]ExternalHit, error) {
	query = strings.Join(strings.Fields(query), " ")
	key := media + "\x00" + strings.ToLower(query)
	if hits, ok := s.externalSearch.get(key, s.now()); ok {
		return hits, nil
	}
	c, err := s.tmdb.keyed(ctx)
	if errors.Is(err, errNotConfigured) {
		return nil, model.ErrProviderNotConfigured
	}
	if err != nil {
		return nil, err
	}
	var hits []ExternalHit
	for _, m := range []string{MediaMovie, MediaShow} {
		if media != "" && media != m {
			continue
		}
		kind := "movie"
		if m == MediaShow {
			kind = "tv"
		}
		rs, err := c.Search(ctx, kind, query, nil, s.tmdb.adult(ctx))
		if err != nil {
			return nil, err
		}
		for _, r := range rs {
			h := ExternalHit{Media: m, TMDBID: r.ID, Title: r.Title, OriginalTitle: r.OriginalTitle, Year: hitYear(r.ReleaseDate)}
			if m == MediaShow {
				h.Title, h.OriginalTitle, h.Year = r.Name, r.OriginalName, hitYear(r.FirstAirDate)
			}
			if h.TMDBID > 0 {
				hits = append(hits, h)
			}
		}
	}
	s.externalSearch.put(key, hits, s.now())
	return hits, nil
}

// hitYear is the year of a TMDB date, 0 when it has none.
func hitYear(date string) int {
	y, _ := yearOf(date)
	return y
}
