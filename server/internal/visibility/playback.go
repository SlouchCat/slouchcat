package visibility

import (
	"context"

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
