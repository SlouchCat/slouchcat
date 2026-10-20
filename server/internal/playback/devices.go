package playback

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/auth"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// Device capability profiles (W-7, M4 design 3.6): every start records the device of the browser
// session (sessions.device_id) with its owner (the account and profile of the start) and, when the
// client reported it, its capability profile, name and type. A client without a device on its
// session (a session from before migration 00007) names its device with PlaybackRequest.deviceId,
// which counts only in the form of the device cookie and only for a device that is new or already
// belongs to the account (claimDevice).

// devicePattern is the form of a device id (the device cookie, 128 random bits as hex, like
// package auth).
var devicePattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// recordDevice records the device of a start and returns its reference for PlaybackInfo (nil when
// the start has no device) and the device id of the playback row: the device, else the client's
// deviceId as it is (progress merging of M1) unless it has the form of a device id that was not
// claimed. A failure to record is logged; it does not stop the playback.
func (s *Service) recordDevice(ctx context.Context, session model.Session, req model.PlaybackRequest, caps model.PlaybackCapabilities, now time.Time) (*model.DeviceRef, string) {
	id := session.DeviceID
	if id == "" && devicePattern.MatchString(req.DeviceID) {
		if !s.claimDevice(ctx, req.DeviceID, session.Account.ID) {
			return nil, "" // never attach the playback to a device of someone else
		}
		id = req.DeviceID
	}
	if id == "" {
		return nil, req.DeviceID
	}
	p := appq.RecordDeviceProfileParams{
		ID: id, Now: now.UnixMilli(),
		AccountID: sql.NullInt64{Int64: session.Account.ID, Valid: true}, ProfileID: sql.NullInt64{Int64: session.Profile.ID, Valid: true},
	}
	if req.Capabilities != nil {
		raw, err := json.Marshal(caps)
		if err != nil {
			s.log.WarnContext(ctx, "encode device capabilities", "err", err)
		} else {
			p.Capabilities = sql.NullString{String: string(raw), Valid: true}
			p.CapabilitiesAt = sql.NullInt64{Int64: now.UnixMilli(), Valid: true}
		}
		p.Name, p.Type = nullString(caps.Name), nullString(caps.Type)
	}
	if err := s.write.RecordDeviceProfile(ctx, p); err != nil {
		s.log.WarnContext(ctx, "record device profile", "device_id", id, "err", err)
		return &model.DeviceRef{ID: id}, id
	}
	ref := &model.DeviceRef{ID: id}
	if d, err := s.read.GetDevice(ctx, id); err == nil {
		ref.Name = d.Name.String
	}
	return ref, id
}

// claimDevice reports whether a client-named device id may become the device of a start: not the
// reserved device of the PIN ceiling, and either without a row yet or already owned by the account.
// A row of another account (or of a browser without playback, owner unknown) is not taken over:
// its owner and capabilities stay, and forgetting it does not end this playback.
func (s *Service) claimDevice(ctx context.Context, id string, accountID int64) bool {
	if id == auth.ReservedDeviceID {
		return false
	}
	d, err := s.read.GetDevice(ctx, id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return true
	case err != nil:
		s.log.WarnContext(ctx, "get device", "device_id", id, "err", err)
		return false
	}
	return d.AccountID.Valid && d.AccountID.Int64 == accountID
}

// ListStreams returns the running playback sessions for the administration (W-6), newest first:
// neither revoked nor expired, a request within ActiveWindow. ItemTitle and DurationMs stay empty;
// the administration fills them from the unfiltered title tables.
func (s *Service) ListStreams(ctx context.Context) ([]model.AdminStream, error) {
	now := s.now()
	rows, err := s.read.ListActivePlaybackSessions(ctx, appq.ListActivePlaybackSessionsParams{
		Now: now.UnixMilli(), ActiveAfter: now.Add(-ActiveWindow).UnixMilli(),
	})
	if err != nil {
		return nil, fmt.Errorf("list active playback sessions: %w", err)
	}
	out := make([]model.AdminStream, 0, len(rows))
	for _, r := range rows {
		st := model.AdminStream{
			ID: r.ID, AccountEmail: r.AccountEmail, ProfileName: r.ProfileName, ItemID: r.ItemID, MediaFileID: r.MediaFileID,
			Mode: r.Mode, Backend: r.Backend.String, Hardware: r.Hardware == 1, Quality: r.Quality.String,
			Width: int(r.Width.Int64), Height: int(r.Height.Int64), Bitrate: int(r.Bitrate.Int64), Reason: r.Reason.String,
			StartedAt: time.UnixMilli(r.StartedAt).UTC(), LastActivityAt: time.UnixMilli(r.LastActivityAt).UTC(),
		}
		if r.DeviceID.Valid {
			st.Device = &model.DeviceRef{ID: r.DeviceID.String, Name: r.DeviceName.String}
		}
		if r.PositionMs.Valid {
			pos := r.PositionMs.Int64
			st.PositionMs = &pos
		}
		out = append(out, st)
	}
	return out, nil
}

// StopStream ends a running playback session of any browser session for the administration
// (W-6): it revokes the row (stopped_by admin), closes its open Range responses and stops its
// ffmpeg process. An unknown or already ended playback session is model.ErrNotFound.
func (s *Service) StopStream(ctx context.Context, id string) error {
	n, err := s.write.StopPlaybackSession(ctx, appq.StopPlaybackSessionParams{
		ID: id, Now: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}, StoppedBy: nullString(model.StoppedByAdmin),
	})
	if err != nil {
		return fmt.Errorf("stop playback session: %w", err)
	}
	if n == 0 {
		return model.ErrNotFound
	}
	s.end(id)
	s.log.InfoContext(ctx, "playback ended by the administration", "playback_id", id)
	return nil
}

// EndDevice ends every playback session of a device (forgotten by the administration, W-7): those
// recorded for it and those its browser sessions started (stopped_by admin).
func (s *Service) EndDevice(ctx context.Context, deviceID string) error {
	ids, err := s.write.RevokePlaybackSessionsByDevice(ctx, appq.RevokePlaybackSessionsByDeviceParams{
		DeviceID: nullString(deviceID), Now: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}, StoppedBy: nullString(model.StoppedByAdmin),
	})
	if err != nil {
		return fmt.Errorf("revoke playback sessions of device: %w", err)
	}
	for _, id := range ids {
		s.end(id)
	}
	return nil
}

func notFoundOr(err error, what string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrNotFound
	}
	return fmt.Errorf("%s: %w", what, err)
}

func nullPositive(v int) sql.NullInt64 { return sql.NullInt64{Int64: int64(v), Valid: v > 0} }
