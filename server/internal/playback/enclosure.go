package playback

import (
	"context"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Podcast episodes without a library file stream through the server from their enclosure URL
// (M6, design 3.4): the same stream token and visibility rules as every other stream, the
// client's address never reaches the podcast host, and the download job reuses the fetch. Stream B
// implements startEnclosure (a direct playback session with media_file_id "" and the item's
// duration) and ProxyEnclosure (Range pass-through to the enclosure).

// startEnclosure starts the playback of a podcast episode from its enclosure.
func (s *Service) startEnclosure(_ context.Context, _ model.Session, _ model.ItemSummary, _ model.PlaybackRequest) (model.PlaybackInfo, error) {
	return model.PlaybackInfo{}, model.ErrNotImplemented
}
