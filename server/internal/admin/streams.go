package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Device and stream overviews of the administration (W-6, W-7, M4 design section 8).

// Streams is implemented by package playback: the running playback sessions and how they end.
type Streams interface {
	// ListStreams returns the running playback sessions, newest first, without titles.
	ListStreams(ctx context.Context) ([]model.AdminStream, error)
	// StopStream revokes a playback session (stopped_by admin), closes its open responses and
	// stops its ffmpeg; model.ErrNotFound for an unknown or ended one.
	StopStream(ctx context.Context, id string) error
	// EndDevice ends every playback session of a device.
	EndDevice(ctx context.Context, deviceID string) error
}

// activeWindow matches playback.ActiveWindow: streams with a request within it count as running.
const activeWindow = 2 * time.Minute

// WithStreams sets the playback service behind the stream overview and the device removal, and
// returns the receiver.
func (s *Service) WithStreams(st Streams) *Service {
	s.streams = st
	return s
}

// ListDevices returns every device, last contact first, with the owner of its last playback start
// (account e-mail and profile name), its capability profile and its running streams (W-7).
func (s *Service) ListDevices(ctx context.Context) ([]model.AdminDevice, error) {
	now := s.now()
	rows, err := s.read.ListDevicesAdmin(ctx, appq.ListDevicesAdminParams{Now: now.UnixMilli(), ActiveAfter: now.Add(-activeWindow).UnixMilli()})
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}
	out := make([]model.AdminDevice, 0, len(rows))
	for _, r := range rows {
		if r.ID == auth.ReservedDeviceID {
			continue // the counter of the PIN ceiling, no browser
		}
		d := model.AdminDevice{
			ID: r.ID, Name: r.Name.String, Type: r.Type.String, UserAgent: r.UserAgent.String, OwnerEmail: r.OwnerEmail.String,
			OwnerProfile: r.OwnerProfile.String, CreatedAt: time.UnixMilli(r.CreatedAt).UTC(), LastSeenAt: time.UnixMilli(r.LastSeenAt).UTC(),
			ActiveStreams: int(r.ActiveStreams),
		}
		if r.Capabilities.Valid {
			var c model.PlaybackCapabilities
			if err := json.Unmarshal([]byte(r.Capabilities.String), &c); err != nil {
				s.log.WarnContext(ctx, "stored device capabilities unreadable", "device_id", r.ID, "err", err)
			} else {
				d.Capabilities = &c
			}
		}
		if r.CapabilitiesAt.Valid {
			t := time.UnixMilli(r.CapabilitiesAt.Int64).UTC()
			d.CapabilitiesAt = &t
		}
		out = append(out, d)
	}
	return out, nil
}

// DeleteDevice forgets a device (W-7): its browser sessions are revoked first (no new stream can
// start), then its playback ends, its PIN counters and its row are deleted. The browser keeps its
// device cookie: its next sign-in recreates the row under the same id, without name, capabilities
// and owner until its next playback start. An unknown device and the reserved device of the PIN
// ceiling (auth.ReservedDeviceID, whose counters must not be reset) are model.ErrNotFound.
func (s *Service) DeleteDevice(ctx context.Context, id string) error {
	if id == auth.ReservedDeviceID {
		return model.ErrNotFound
	}
	if _, err := s.read.GetDevice(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.ErrNotFound
		}
		return fmt.Errorf("get device: %w", err)
	}
	now := sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}
	device := sql.NullString{String: id, Valid: true}
	if _, err := s.write.RevokeDeviceSessions(ctx, appq.RevokeDeviceSessionsParams{DeviceID: device, Now: now}); err != nil {
		return fmt.Errorf("revoke device sessions: %w", err)
	}
	if s.streams != nil {
		if err := s.streams.EndDevice(ctx, id); err != nil {
			return err
		}
	}
	if err := s.write.DeletePinAttemptsByDevice(ctx, id); err != nil {
		return fmt.Errorf("delete pin attempts: %w", err)
	}
	if _, err := s.write.DeleteDevice(ctx, id); err != nil {
		return fmt.Errorf("delete device: %w", err)
	}
	s.log.InfoContext(ctx, "device forgotten", "device_id", id)
	return nil
}

// ListStreams returns the running playback sessions (W-6), newest first, with the title in plain
// text ("Series · S01E02 · Title" for an episode) and the duration of the media file.
func (s *Service) ListStreams(ctx context.Context) ([]model.AdminStream, error) {
	if s.streams == nil {
		return nil, ErrNotConfigured
	}
	streams, err := s.streams.ListStreams(ctx)
	if err != nil || len(streams) == 0 {
		return streams, err
	}
	itemIDs := make([]string, 0, len(streams))
	fileIDs := make([]string, 0, len(streams))
	for _, st := range streams {
		itemIDs = append(itemIDs, st.ItemID)
		fileIDs = append(fileIDs, st.MediaFileID)
	}
	items, err := s.sys.AdminStreamItems(ctx, itemIDs)
	if err != nil {
		return nil, fmt.Errorf("stream titles: %w", err)
	}
	titles := make(map[string]string, len(items))
	for _, it := range items {
		titles[it.ID] = streamTitle(it)
	}
	durations, err := s.sys.AdminMediaFileDurations(ctx, fileIDs)
	if err != nil {
		return nil, fmt.Errorf("stream durations: %w", err)
	}
	byFile := make(map[string]int64, len(durations))
	for _, d := range durations {
		if d.DurationMs.Valid {
			byFile[d.ID] = d.DurationMs.Int64
		}
	}
	for i := range streams {
		st := &streams[i]
		st.ItemTitle = titles[st.ItemID]
		if d, ok := byFile[st.MediaFileID]; ok {
			st.DurationMs = &d
		}
	}
	return streams, nil
}

// streamTitle names a title for the overview: an episode as "Series · S01E02 · Title" (a
// multi-episode file "S01E03-E04"), everything else by its title.
func streamTitle(it systemq.AdminStreamItemsRow) string {
	if it.Kind != "episode" || it.SeriesTitle == "" {
		return it.Title
	}
	parts := []string{it.SeriesTitle}
	season := it.SeasonNumber
	if !season.Valid {
		season = it.ParentSeasonNumber
	}
	if it.EpisodeNumber.Valid {
		code := fmt.Sprintf("E%02d", it.EpisodeNumber.Int64)
		if season.Valid {
			code = fmt.Sprintf("S%02d", season.Int64) + code
		}
		if it.EpisodeEnd.Valid && it.EpisodeEnd.Int64 > it.EpisodeNumber.Int64 {
			code += fmt.Sprintf("-E%02d", it.EpisodeEnd.Int64)
		}
		parts = append(parts, code)
	}
	parts = append(parts, it.Title)
	return strings.Join(parts, " · ")
}

// StopStream ends a running stream (W-6); model.ErrNotFound for an unknown or ended one.
func (s *Service) StopStream(ctx context.Context, id string) error {
	if s.streams == nil {
		return ErrNotConfigured
	}
	return s.streams.StopStream(ctx, id)
}
