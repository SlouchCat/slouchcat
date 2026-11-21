package auth

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// «Meine Geräte» (concept 27, 4.5, ADR 0040): the devices of the session's account and its
// sign-out. The account of a session or a download decides which devices are listed, never
// devices.account_id (the owner of the last playback start).

// ListMyDevices returns every device that holds a living session or a download of the session's
// account, last contact first; Current marks the session's own device. Every profile may read it.
func (s *Service) ListMyDevices(ctx context.Context, session model.Session) ([]model.MyDevice, error) {
	rows, err := s.read.ListMyDevices(ctx, appq.ListMyDevicesParams{AccountID: session.Account.ID, Now: s.now().UnixMilli()})
	if err != nil {
		return nil, fmt.Errorf("list my devices: %w", err)
	}
	out := make([]model.MyDevice, 0, len(rows))
	for _, r := range rows {
		d := model.MyDevice{
			ID: r.ID, Name: r.Name.String, Type: r.Type.String, SessionKind: r.SessionKind,
			LastSeenAt: time.UnixMilli(r.LastSeenAt).UTC(), Downloads: int(r.Downloads),
			Current: session.DeviceID != "" && r.ID == session.DeviceID,
		}
		if r.SessionKind != "" && r.SignedInAt > 0 {
			t := time.UnixMilli(r.SignedInAt).UTC()
			d.SignedInAt = &t
		}
		out = append(out, d)
	}
	return out, nil
}

// SignOutMyDevice removes everything the session's account has on a device and nothing of another
// account: its sessions (end runs first with their ids, so their streams stop before the sessions
// go), any playback left of them, its downloads (the trigger downloads_tombstone writes the
// revocation list), the push subscriptions of the device and the auto-download rules of the
// account's profiles on it, in one transaction. The device row stays. Only a session without an
// active profile or with the primary profile may do it (model.ErrPrimaryProfileRequired, the rule
// of the notification channels); a device without a living session or download of the account and
// the reserved device of the PIN ceiling are model.ErrNotFound.
func (s *Service) SignOutMyDevice(ctx context.Context, session model.Session, deviceID string,
	end func(ctx context.Context, sessionIDs []int64) error) error {
	if session.HasProfile() && !session.IsPrimary() {
		return model.ErrPrimaryProfileRequired
	}
	if !devicePattern.MatchString(deviceID) || deviceID == ReservedDeviceID {
		return model.ErrNotFound
	}
	account := session.Account.ID
	device := sql.NullString{String: deviceID, Valid: true}
	held, err := s.write.DeviceHeldByAccount(ctx, appq.DeviceHeldByAccountParams{ID: device, AccountID: account, Now: s.now().UnixMilli()})
	if err != nil {
		return fmt.Errorf("device of the account: %w", err)
	}
	if !held.Bool {
		return model.ErrNotFound
	}
	ids, err := s.write.ListAccountDeviceSessions(ctx, appq.ListAccountDeviceSessionsParams{DeviceID: device, AccountID: account})
	if err != nil {
		return fmt.Errorf("list device sessions: %w", err)
	}
	if end != nil && len(ids) > 0 {
		if err := end(ctx, ids); err != nil {
			return err
		}
	}

	now := nullInt(s.now().UnixMilli())
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin sign-out: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.write.WithTx(tx)
	sessions, err := q.RevokeAccountDeviceSessions(ctx, appq.RevokeAccountDeviceSessionsParams{Now: now, DeviceID: device, AccountID: account})
	if err != nil {
		return fmt.Errorf("revoke device sessions: %w", err)
	}
	if err := q.RevokePlaybackSessionsByAccountDevice(ctx, appq.RevokePlaybackSessionsByAccountDeviceParams{
		Now: now, StoppedBy: sql.NullString{String: model.StoppedByLogout, Valid: true}, DeviceID: device, AccountID: account,
	}); err != nil {
		return fmt.Errorf("revoke playback sessions: %w", err)
	}
	pushes, err := q.DeleteAccountDevicePushSubscriptions(ctx, appq.DeleteAccountDevicePushSubscriptionsParams{DeviceID: device, AccountID: account})
	if err != nil {
		return fmt.Errorf("delete push subscriptions: %w", err)
	}
	rules, err := q.DeleteAccountDeviceRules(ctx, appq.DeleteAccountDeviceRulesParams{DeviceID: deviceID, AccountID: account})
	if err != nil {
		return fmt.Errorf("delete download rules: %w", err)
	}
	downloads, err := q.DeleteAccountDeviceDownloads(ctx, appq.DeleteAccountDeviceDownloadsParams{DeviceID: deviceID, AccountID: account})
	if err != nil {
		return fmt.Errorf("delete downloads: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sign-out: %w", err)
	}
	// A playback the device started between the first end and the commit has its row revoked, but
	// its HLS output runs on: end the sessions' streams once more (idempotent; code review m-2).
	if end != nil && len(ids) > 0 {
		if err := end(context.WithoutCancel(ctx), ids); err != nil {
			s.log.WarnContext(ctx, "end streams after the sign-out", "account_id", account, "err", err)
		}
	}
	s.log.InfoContext(ctx, "account signed out of a device", "account_id", account, "own_device", deviceID == session.DeviceID,
		"sessions_revoked", sessions, "push_subscriptions", pushes, "download_rules", rules, "downloads", downloads)
	return nil
}
