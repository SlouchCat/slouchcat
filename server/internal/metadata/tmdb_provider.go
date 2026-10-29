package metadata

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// tmdbProvider implements Provider on top of Client with the key of the KeySource, read per
// call so a changed key applies without a restart.
type tmdbProvider struct {
	client *Client
	keys   KeySource
}

// tmdbLangs maps the text languages to the TMDB request languages.
var tmdbLangs = map[string]string{"de": langDE, "en": langEN}

func tmdbLang(lang string) string {
	if l, ok := tmdbLangs[lang]; ok {
		return l
	}
	return lang
}

func (p *tmdbProvider) Name() string { return ProviderTMDB }

func (p *tmdbProvider) Configured(ctx context.Context) bool {
	key, err := p.keys.TMDBKey(ctx)
	return err == nil && key != ""
}

// keyed returns the client with the current key or errNotConfigured.
func (p *tmdbProvider) keyed(ctx context.Context) (*Client, error) {
	key, err := p.keys.TMDBKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("read tmdb key: %w", err)
	}
	if key == "" {
		return nil, errNotConfigured
	}
	return p.client.WithKey(key), nil
}

func parseTMDBID(id string) (int64, error) {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("tmdb id %q: %w", id, ErrNotFound)
	}
	return n, nil
}

// candidate converts a search or find result.
func (p *tmdbProvider) candidate(r SearchResult, series bool) Candidate {
	c := Candidate{Provider: ProviderTMDB, ID: strconv.FormatInt(r.ID, 10), Title: r.Title,
		OriginalTitle: r.OriginalTitle, Overview: r.Overview}
	date := r.ReleaseDate
	if series {
		c.Title, c.OriginalTitle, date = r.Name, r.OriginalName, r.FirstAirDate
	}
	if y, ok := yearOf(date); ok {
		c.Year = &y
	}
	if r.PosterPath != "" {
		c.PosterURL = p.client.imageURL("w200", r.PosterPath)
	}
	return c
}

func (p *tmdbProvider) candidates(rs []SearchResult, series bool) []Candidate {
	out := make([]Candidate, 0, len(rs))
	for _, r := range rs {
		out = append(out, p.candidate(r, series))
	}
	return out
}

func (p *tmdbProvider) SearchMovie(ctx context.Context, q MovieQuery) ([]Candidate, error) {
	c, err := p.keyed(ctx)
	if err != nil {
		return nil, err
	}
	if q.IMDBID != "" {
		movies, _, err := c.Find(ctx, q.IMDBID, "imdb_id")
		return p.candidates(movies, false), err
	}
	rs, err := c.Search(ctx, "movie", q.Title, q.Year)
	return p.candidates(rs, false), err
}

func (p *tmdbProvider) SearchSeries(ctx context.Context, q SeriesQuery) ([]Candidate, error) {
	c, err := p.keyed(ctx)
	if err != nil {
		return nil, err
	}
	switch {
	case q.TVDBID > 0:
		_, series, err := c.Find(ctx, strconv.FormatInt(q.TVDBID, 10), "tvdb_id")
		return p.candidates(series, true), err
	case q.IMDBID != "":
		_, series, err := c.Find(ctx, q.IMDBID, "imdb_id")
		return p.candidates(series, true), err
	}
	rs, err := c.Search(ctx, "tv", q.Title, q.Year)
	return p.candidates(rs, true), err
}

// recordLang returns the language whose detail fills the language-independent fields: English
// when requested (the M1b behaviour), else the first.
func recordLang(langs []string) string {
	for _, l := range langs {
		if l == "en" {
			return l
		}
	}
	if len(langs) > 0 {
		return langs[0]
	}
	return "en"
}

func (p *tmdbProvider) Movie(ctx context.Context, id string, langs []string) (MovieRecord, error) {
	c, err := p.keyed(ctx)
	if err != nil {
		return MovieRecord{}, err
	}
	n, err := parseTMDBID(id)
	if err != nil {
		return MovieRecord{}, err
	}
	byLang := map[string]Movie{}
	for _, l := range langs {
		m, err := c.Movie(ctx, n, tmdbLang(l))
		if err != nil {
			return MovieRecord{}, err
		}
		byLang[l] = m
	}
	m := byLang[recordLang(langs)]
	rec := MovieRecord{TitleRecord: TitleRecord{
		Provider: ProviderTMDB, ID: strconv.FormatInt(m.ID, 10), TMDBID: m.ID, IMDBID: m.IMDBID,
		OriginalTitle: m.OriginalTitle, Date: m.ReleaseDate, Runtime: m.Runtime, VoteAverage: m.VoteAverage,
		Texts: map[string]Text{}, Genres: map[string][]string{},
		Certifications: releaseCertifications(m.ReleaseDates),
		Credits:        tmdbCredits(m.Credits, nil),
		PosterPath:     m.PosterPath, BackdropPath: m.BackdropPath,
	}}
	for l, lm := range byLang {
		rec.Texts[l] = Text{Title: lm.Title, Overview: lm.Overview, Tagline: lm.Tagline}
		rec.Genres[l] = genreNames(lm.Genres)
		if c := lm.BelongsToCollection; c != nil && c.ID > 0 {
			if rec.Collection == nil {
				rec.Collection = &CollectionRecord{TMDBID: c.ID, Name: map[string]string{}, Overview: map[string]string{}}
			}
			rec.Collection.Name[l], rec.Collection.Overview[l] = c.Name, c.Overview
			if c.PosterPath != "" && (l == recordLang(langs) || rec.Collection.PosterPath == "") {
				rec.Collection.PosterPath = c.PosterPath
			}
		}
	}
	return rec, nil
}

func (p *tmdbProvider) Series(ctx context.Context, id string, langs []string) (SeriesRecord, error) {
	c, err := p.keyed(ctx)
	if err != nil {
		return SeriesRecord{}, err
	}
	n, err := parseTMDBID(id)
	if err != nil {
		return SeriesRecord{}, err
	}
	byLang := map[string]TV{}
	for _, l := range langs {
		tv, err := c.TV(ctx, n, tmdbLang(l))
		if err != nil {
			return SeriesRecord{}, err
		}
		byLang[l] = tv
	}
	tv := byLang[recordLang(langs)]
	certs := map[string]string{}
	for _, r := range tv.ContentRatings.Results {
		if code, rating := strings.ToUpper(strings.TrimSpace(r.Country)), strings.TrimSpace(r.Rating); rating != "" {
			certs[code] = rating
		}
	}
	runtime := 0
	for _, rt := range tv.EpisodeRunTime {
		if rt > 0 {
			runtime = rt
			break
		}
	}
	var creators []Credit
	for _, cr := range tv.CreatedBy {
		creators = append(creators, Credit{Person: Person{TMDBID: cr.ID, Name: cr.Name, ProfilePath: cr.ProfilePath}, Role: RoleCreator})
	}
	rec := SeriesRecord{TitleRecord: TitleRecord{
		Provider: ProviderTMDB, ID: strconv.FormatInt(tv.ID, 10), TMDBID: tv.ID, TVDBID: tv.ExternalIDs.TVDBID,
		IMDBID: tv.ExternalIDs.IMDBID, OriginalTitle: tv.OriginalName, Date: tv.FirstAirDate, Runtime: runtime,
		VoteAverage: tv.VoteAverage, Texts: map[string]Text{}, Genres: map[string][]string{}, Certifications: certs,
		Credits:    tmdbCredits(tv.Credits, creators),
		PosterPath: tv.PosterPath, BackdropPath: tv.BackdropPath,
	}}
	for l, ltv := range byLang {
		rec.Texts[l] = Text{Title: ltv.Name, Overview: ltv.Overview, Tagline: ltv.Tagline}
		rec.Genres[l] = genreNames(ltv.Genres)
	}
	for _, s := range tv.Seasons {
		rec.Seasons = append(rec.Seasons, SeasonInfo{Number: s.SeasonNumber, EpisodeCount: s.EpisodeCount, PosterPath: s.PosterPath})
	}
	return rec, nil
}

func (p *tmdbProvider) Season(ctx context.Context, seriesID string, n int, langs []string) (SeasonRecord, error) {
	c, err := p.keyed(ctx)
	if err != nil {
		return SeasonRecord{}, err
	}
	id, err := parseTMDBID(seriesID)
	if err != nil {
		return SeasonRecord{}, err
	}
	rec := SeasonRecord{Number: n, Texts: map[string]Text{}}
	episodes := map[int]*EpisodeRecord{}
	var order []int
	main := recordLang(langs)
	for _, l := range langs {
		s, err := c.TVSeason(ctx, id, n, tmdbLang(l))
		if err != nil {
			return SeasonRecord{}, err
		}
		rec.Texts[l] = Text{Title: s.Name, Overview: s.Overview}
		if l == main {
			rec.AirDate, rec.PosterPath = s.AirDate, s.PosterPath
		}
		for _, e := range s.Episodes {
			ep := episodes[e.EpisodeNumber]
			if ep == nil {
				ep = &EpisodeRecord{Season: n, Number: e.EpisodeNumber, Texts: map[string]Text{}}
				episodes[e.EpisodeNumber] = ep
				order = append(order, e.EpisodeNumber)
			}
			ep.Texts[l] = Text{Title: e.Name, Overview: e.Overview}
			if l == main || ep.AirDate == "" {
				ep.AirDate, ep.Runtime, ep.StillPath = e.AirDate, e.Runtime, e.StillPath
			}
		}
	}
	for _, num := range order {
		rec.Episodes = append(rec.Episodes, *episodes[num])
	}
	return rec, nil
}

func (p *tmdbProvider) Episode(ctx context.Context, seriesID string, s, e int, langs []string) (EpisodeRecord, error) {
	c, err := p.keyed(ctx)
	if err != nil {
		return EpisodeRecord{}, err
	}
	id, err := parseTMDBID(seriesID)
	if err != nil {
		return EpisodeRecord{}, err
	}
	rec := EpisodeRecord{Season: s, Number: e, Texts: map[string]Text{}}
	main := recordLang(langs)
	for _, l := range langs {
		ep, err := c.TVEpisode(ctx, id, s, e, tmdbLang(l))
		if err != nil {
			return EpisodeRecord{}, err
		}
		rec.Texts[l] = Text{Title: ep.Name, Overview: ep.Overview}
		if l == main || rec.AirDate == "" {
			rec.AirDate, rec.Runtime, rec.StillPath = ep.AirDate, ep.Runtime, ep.StillPath
		}
	}
	return rec, nil
}

// tmdbCredits converts TMDB credits: the first maxCast cast members by order, directors and
// writers (department Writing) once per person, after the given creators.
func tmdbCredits(c Credits, creators []Credit) []Credit {
	out := append([]Credit(nil), creators...)
	cast := c.Cast
	if len(cast) > maxCast {
		cast = cast[:maxCast]
	}
	for _, m := range cast {
		out = append(out, Credit{Person: Person{TMDBID: m.ID, Name: m.Name, ProfilePath: m.ProfilePath},
			Role: RoleCast, Character: m.Character, Order: m.Order})
	}
	seen := map[string]bool{}
	for _, m := range c.Crew {
		role := ""
		switch {
		case m.Job == "Director":
			role = RoleDirector
		case m.Department == "Writing":
			role = RoleWriter
		default:
			continue
		}
		key := role + "/" + strconv.FormatInt(m.ID, 10)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, Credit{Person: Person{TMDBID: m.ID, Name: m.Name, ProfilePath: m.ProfilePath}, Role: role, Order: len(seen) - 1})
	}
	return out
}
