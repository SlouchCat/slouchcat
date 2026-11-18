package visibility

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/scopedq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Titles outside the library (concept 25, architecture sections 3, 5 and 11). A title is named by
// its external ids; it is in the library when a library title of its kind that is not deleted
// carries one of them, whether the profile sees it or not. Matching follows the library: a film by
// TMDB id, then IMDb id; a show by TVDB id first, then TMDB id, then IMDb id.

// The media of a title outside the library (external_titles.media, wishes.media).
const (
	MediaMovie = "movie"
	MediaShow  = "show"
)

// unknownAge is the age of a title without a known rating (architecture section 3).
const unknownAge = 18

// ExternalRef names a title by its external ids. TMDBID is required; TVDBID 0 and IMDBID "" are
// unknown and taken from external_titles when the title is stored there.
type ExternalRef struct {
	Media  string // MediaMovie or MediaShow
	TMDBID int64
	TVDBID int64
	IMDBID string
}

func (r ExternalRef) valid() bool {
	return (r.Media == MediaMovie || r.Media == MediaShow) && r.TMDBID > 0
}

// RequestRight reports whether the profile may see titles outside the library of a medium: the
// request right of package wishes for that medium (F-9: a profile that may not request sees no
// missing title), or, for a single title, a wish of the profile's own for it. visibility decides
// the age, the caller the right; nil means no right.
type RequestRight func(media string) bool

// ExternalMatch is the answer for one ExternalRef.
type ExternalMatch struct {
	Ref ExternalRef
	// ItemID is the library title the profile sees (the best rung among its visible titles), ""
	// when it sees none.
	ItemID string
	// InLibrary is true when a library title carries one of the ids, visible or not. Such a title
	// is never missing: for a profile that may not see it, it does not exist (404).
	InLibrary bool
	// Missing is true when the title is outside the library and may be shown to the profile: it has
	// the right and the age of the title (unknown counts as 18) is within min(profile, account).
	Missing bool
	// NeedsDetails is true when the title is outside the library, the profile has the right and an
	// age ceiling below 18, and external_titles does not hold the title yet: its age is unknown
	// until the details are fetched (the search fetches the first 20, then asks again).
	NeedsDetails bool
	// Stored is true when external_titles holds the title: its age and its TVDB and IMDb ids are
	// known to the match. The discover pages fetch the details of a series that is neither in the
	// library nor stored, so a series of the library matched TVDB first is found (architecture 21.3).
	Stored bool
}

// refJSON is one element of the refs array of MatchExternalTitles.
type refJSON struct {
	Media string `json:"media"`
	TMDB  int64  `json:"tmdb"`
	TVDB  int64  `json:"tvdb,omitempty"`
	IMDB  string `json:"imdb,omitempty"`
}

// TitleByExternalIDs returns the library title of the ids without a profile (for package wishes,
// which reads no title table itself): the best rung, an available title before a missing one.
// found is false when no library title carries one of the ids or ref is invalid.
func (r *Reader) TitleByExternalIDs(ctx context.Context, ref ExternalRef) (itemID string, found bool, err error) {
	if !ref.valid() {
		return "", false, nil
	}
	id, err := r.sys.TitleByExternalIDs(ctx, systemq.TitleByExternalIDsParams{
		Media: ref.Media, TmdbID: ref.TMDBID,
		TvdbID: sql.NullInt64{Int64: ref.TVDBID, Valid: ref.TVDBID > 0},
		ImdbID: sql.NullString{String: ref.IMDBID, Valid: ref.IMDBID != ""},
	})
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("title by external ids: %w", err)
	}
	return id, true, nil
}

// MatchExternalTitles answers for each ref, in order, whether the profile sees it as a library
// title or as a missing one (search, discover pages). An invalid ref is neither. right decides
// the request right per medium; visibility decides the age. A profile that does not exist sees
// nothing.
func (r *Reader) MatchExternalTitles(ctx context.Context, s Scope, right RequestRight, refs []ExternalRef) ([]ExternalMatch, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, err
	}
	out := make([]ExternalMatch, len(refs))
	in := make([]refJSON, 0, len(refs))
	pos := make([]int, 0, len(refs)) // index into refs of every element of in
	for i, ref := range refs {
		out[i].Ref = ref
		if ref.valid() {
			in = append(in, refJSON{Media: ref.Media, TMDB: ref.TMDBID, TVDB: max(ref.TVDBID, 0), IMDB: ref.IMDBID})
			pos = append(pos, i)
		}
	}
	if len(in) == 0 {
		return out, nil
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("external refs: %w", err)
	}
	rows, err := r.sys.MatchExternalTitles(ctx, systemq.MatchExternalTitlesParams{Refs: string(raw), ProfileID: pid})
	if err != nil {
		return nil, fmt.Errorf("match external titles: %w", err)
	}
	for _, row := range rows {
		if row.Pos < 0 || int(row.Pos) >= len(pos) {
			continue
		}
		m := &out[pos[row.Pos]]
		m.ItemID = row.ItemID
		m.InLibrary = row.InLibrary == 1
		m.Stored = row.HasRow == 1
		if m.InLibrary || right == nil || !right(m.Ref.Media) {
			continue
		}
		age := int64(unknownAge)
		if row.MinAge.Valid {
			age = row.MinAge.Int64
		}
		m.Missing = age <= row.MaxAge
		m.NeedsDetails = row.HasRow == 0 && row.MaxAge < unknownAge
	}
	return out, nil
}

// MissingTitle returns nil when the title is outside the library and the profile may see it as a
// missing title (preview page, its images, the request): right for its medium, age within the
// profile's ceiling, unknown counting as 18. Every other case answers model.ErrNotFound: a library
// title (also one rated too high for the profile), a title rated too high, a profile without the
// right.
func (r *Reader) MissingTitle(ctx context.Context, s Scope, right RequestRight, ref ExternalRef) error {
	if !ref.valid() {
		if _, err := r.profile(ctx, s); err != nil {
			return err
		}
		return model.ErrNotFound
	}
	m, err := r.MatchExternalTitles(ctx, s, right, []ExternalRef{ref})
	if err != nil {
		return err
	}
	if !m[0].Missing {
		return model.ErrNotFound
	}
	return nil
}

// HiddenSeasons returns the season numbers (1 or more) of a series the profile sees that the
// library has (a live, available episode) but the profile can play none of, for instance a
// season rated above it. Its catalog and its request dialog leave them out: neither «fehlt» nor
// «anfordern» tells it they exist. A series the profile does not see answers model.ErrNotFound.
func (r *Reader) HiddenSeasons(ctx context.Context, s Scope, seriesID string) ([]int, error) {
	visible, err := r.VisibleSeasons(ctx, s, seriesID)
	if err != nil {
		return nil, err
	}
	return r.hiddenSeasons(ctx, seriesID, visible)
}

// hiddenSeasons are the library seasons of seriesID without an episode in visible.
func (r *Reader) hiddenSeasons(ctx context.Context, seriesID string, visible map[int]int) ([]int, error) {
	rows, err := r.sys.ListLibrarySeasonNumbers(ctx, sql.NullString{String: seriesID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("library seasons: %w", err)
	}
	var out []int
	for _, n := range rows {
		if visible[int(n)] == 0 {
			out = append(out, int(n))
		}
	}
	return out, nil
}

// VisibleSeasons returns, per season number of a series the profile sees, the number of episodes
// it can play (an episode file with several episodes counts each). Seasons without such an episode
// are absent; season 0 (specials) is included when it has one. A series the profile does not see
// answers model.ErrNotFound.
func (r *Reader) VisibleSeasons(ctx context.Context, s Scope, seriesID string) (map[int]int, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, err
	}
	ok, err := r.q.VisibleSeriesExists(ctx, scopedq.VisibleSeriesExistsParams{ProfileID: pid, SeriesID: seriesID})
	if err != nil {
		return nil, fmt.Errorf("visible series: %w", err)
	}
	if !ok {
		return nil, model.ErrNotFound
	}
	rows, err := r.q.ListVisibleSeasonEpisodes(ctx, scopedq.ListVisibleSeasonEpisodesParams{ProfileID: pid, SeriesID: seriesID})
	if err != nil {
		return nil, fmt.Errorf("visible seasons: %w", err)
	}
	out := make(map[int]int, len(rows))
	for _, row := range rows {
		out[int(row.SeasonNumber)] = int(row.Episodes)
	}
	return out, nil
}
