package visibility

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/slouchcat/slouchcat/server/internal/store/scopedq"
)

// Reads for playback (M1b): the per-request check of stream-token requests and the stored probe of
// a media file that the playback decision and the HLS plan need.

// Visible returns nil when the profile of s may see the title, else model.ErrNotFound. Stream-token
// requests (playlists, segments, Range) call it on every request, so a title that becomes
// invisible during playback answers 404 from the next request on.
func (r *Reader) Visible(ctx context.Context, s Scope, itemID string) error {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return err
	}
	_, err = r.q.GetVisibleItem(ctx, scopedq.GetVisibleItemParams{ProfileID: pid, ItemID: itemID})
	return notFound(err)
}

// MediaFileProbe is the stored ffprobe result (JSON of transcode.Probe, empty when not probed yet)
// and the encoded keyframe index (nil when not indexed yet) of a media file.
type MediaFileProbe struct {
	Probe     string
	Keyframes []byte
}

// MediaFileProbe returns the probe data of a media file of a visible title, or model.ErrNotFound.
func (r *Reader) MediaFileProbe(ctx context.Context, s Scope, itemID, fileID string) (MediaFileProbe, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return MediaFileProbe{}, err
	}
	row, err := r.q.GetVisibleMediaFileProbe(ctx, scopedq.GetVisibleMediaFileProbeParams{ProfileID: pid, ItemID: itemID, MediaFileID: fileID})
	if err != nil {
		return MediaFileProbe{}, notFound(err)
	}
	return MediaFileProbe{Probe: row.Probe.String, Keyframes: row.Keyframes}, nil
}

// TrackContext is what the track choice of a playback or a download needs about a visible title
// (concept 13 section 11.3, package trackpick): the series of an episode, reached through its
// season, and the original language of the title, for an episode the series' (ISO 639-1; "" when
// unknown or not looked up yet).
type TrackContext struct {
	SeriesID         string // empty for every title but an episode of a series
	OriginalLanguage string
}

// TrackContext returns the track context of a visible title; an invisible or unknown title is
// model.ErrNotFound. An episode whose season or series the profile cannot see has no series.
func (r *Reader) TrackContext(ctx context.Context, s Scope, itemID string) (TrackContext, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return TrackContext{}, err
	}
	it, err := r.q.GetVisibleItem(ctx, scopedq.GetVisibleItemParams{ProfileID: pid, ItemID: itemID})
	if err != nil {
		return TrackContext{}, notFound(err)
	}
	out := TrackContext{OriginalLanguage: it.OriginalLanguage.String}
	if it.Kind != "episode" || !it.ParentID.Valid {
		return out, nil
	}
	parent := func(id, kind string) (scopedq.Item, bool, error) {
		p, err := r.q.GetVisibleItem(ctx, scopedq.GetVisibleItemParams{ProfileID: pid, ItemID: id})
		if errors.Is(err, sql.ErrNoRows) {
			return scopedq.Item{}, false, nil
		}
		if err != nil {
			return scopedq.Item{}, false, fmt.Errorf("get %s: %w", kind, err)
		}
		return p, p.Kind == kind, nil
	}
	season, ok, err := parent(it.ParentID.String, "season")
	if err != nil || !ok || !season.ParentID.Valid {
		return out, err
	}
	series, ok, err := parent(season.ParentID.String, "series")
	if err != nil || !ok {
		return out, err
	}
	out.SeriesID = series.ID
	if series.OriginalLanguage.Valid {
		out.OriginalLanguage = series.OriginalLanguage.String
	}
	return out, nil
}
