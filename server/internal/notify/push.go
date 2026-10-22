package notify

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/notify/push"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

const (
	maxDeviceName  = 80
	maxUserAgent   = 512
	maxEndpointLen = 2048
	// pushSubscriber is the contact in the VAPID token when the server has no https address.
	pushSubscriber = "https://github.com/SlouchCat/slouchcat"
)

// vapid returns the server's VAPID pair with the contact of the token, creating and storing the
// pair on first use (model.ErrPushUnavailable without Config).
func (s *Service) vapid(ctx context.Context) (push.VAPID, error) {
	if s.cfg == nil {
		return push.VAPID{}, model.ErrPushUnavailable
	}
	s.vapidMu.Lock()
	defer s.vapidMu.Unlock()
	pub, priv, err := s.cfg.VAPIDKeys(ctx)
	if err != nil {
		return push.VAPID{}, fmt.Errorf("read VAPID keys: %w", err)
	}
	if pub == "" || priv == "" {
		if pub, priv, err = push.GenerateVAPID(); err != nil {
			return push.VAPID{}, err
		}
		if err := s.cfg.SetVAPIDKeys(ctx, pub, priv); err != nil {
			return push.VAPID{}, fmt.Errorf("store VAPID keys: %w", err)
		}
		s.log.InfoContext(ctx, "web push key pair created")
	}
	subscriber := pushSubscriber
	if u, err := s.cfg.PublicURL(ctx); err == nil && strings.HasPrefix(u, "https://") {
		subscriber = strings.TrimRight(u, "/")
	}
	return push.VAPID{PublicKey: pub, PrivateKey: priv, Subscriber: subscriber}, nil
}

// PushPublicKey returns the VAPID public key the browsers subscribe with (model.ErrPushUnavailable
// without Config).
func (s *Service) PushPublicKey(ctx context.Context) (string, error) {
	v, err := s.vapid(ctx)
	if err != nil {
		return "", err
	}
	return v.PublicKey, nil
}

// RotateVAPID replaces the VAPID pair and deletes every push subscription, which the old pair
// signed (admin.PushRotator); the browsers subscribe again with the new key.
func (s *Service) RotateVAPID(ctx context.Context) error {
	if s.cfg == nil {
		return model.ErrPushUnavailable
	}
	s.vapidMu.Lock()
	defer s.vapidMu.Unlock()
	pub, priv, err := push.GenerateVAPID()
	if err != nil {
		return err
	}
	if err := s.cfg.SetVAPIDKeys(ctx, pub, priv); err != nil {
		return fmt.Errorf("store VAPID keys: %w", err)
	}
	n, err := s.write.PushSubscriptionsDeleteAll(ctx)
	if err != nil {
		return fmt.Errorf("delete push subscriptions: %w", err)
	}
	s.log.InfoContext(ctx, "web push key pair rotated", "subscriptions_deleted", n)
	return nil
}

// deviceOf is the device of the request, else the session's.
func deviceOf(ctx context.Context, session model.Session) string {
	if id := model.DeviceID(ctx); id != "" {
		return id
	}
	return session.DeviceID
}

// pushSubscriptions lists the subscriptions of the session's account without their secrets.
func (s *Service) pushSubscriptions(ctx context.Context, session model.Session) ([]model.PushSubscription, error) {
	rows, err := s.read.PushSubscriptionsByAccount(ctx, session.Account.ID)
	if err != nil {
		return nil, fmt.Errorf("push subscriptions: %w", err)
	}
	device := deviceOf(ctx, session)
	out := make([]model.PushSubscription, 0, len(rows))
	for _, r := range rows {
		out = append(out, model.PushSubscription{
			ID: r.ID, DeviceName: r.DeviceName.String, CreatedAt: time.UnixMilli(r.CreatedAt).UTC(),
			ThisDevice: device != "" && r.DeviceID.String == device,
		})
	}
	return out, nil
}

// CreatePushSubscription stores the push subscription of a browser for the session's account,
// one per endpoint (the same endpoint again takes the new keys, device and account).
// model.ErrPrimaryProfileRequired (requireManager); model.ErrPushUnavailable without Config;
// *model.ValidationError for an endpoint that is not an https URL (http too in development) or
// keys that are no P-256 point and 16-byte secret.
func (s *Service) CreatePushSubscription(ctx context.Context, session model.Session, in model.PushSubscriptionInput) (model.PushSubscription, error) {
	if err := requireManager(session); err != nil {
		return model.PushSubscription{}, err
	}
	if s.cfg == nil {
		return model.PushSubscription{}, model.ErrPushUnavailable
	}
	var verr *model.ValidationError
	if len(in.Endpoint) > maxEndpointLen || push.CheckEndpoint(in.Endpoint, s.devPush) != nil {
		verr = verr.Add("endpoint", "invalid_value")
	}
	badKey, badAuth := push.CheckKeys(in.P256dh, in.Auth)
	if badKey {
		verr = verr.Add("keys.p256dh", "invalid_value")
	}
	if badAuth {
		verr = verr.Add("keys.auth", "invalid_value")
	}
	name := strings.TrimSpace(in.DeviceName)
	if utf8.RuneCountInString(name) > maxDeviceName {
		verr = verr.Add("deviceName", "too_long")
	}
	if verr != nil {
		return model.PushSubscription{}, verr
	}
	device := deviceOf(ctx, session)
	row, err := s.write.PushSubscriptionUpsert(ctx, systemq.PushSubscriptionUpsertParams{
		AccountID: session.Account.ID, DeviceID: sql.NullString{String: device, Valid: device != ""},
		Endpoint: in.Endpoint, P256dh: in.P256dh, Auth: in.Auth,
		UserAgent:  sql.NullString{String: truncate(in.UserAgent, maxUserAgent), Valid: in.UserAgent != ""},
		DeviceName: sql.NullString{String: name, Valid: name != ""}, Now: s.now().UnixMilli(),
	})
	if err != nil {
		return model.PushSubscription{}, fmt.Errorf("store push subscription: %w", err)
	}
	s.log.InfoContext(ctx, "push subscription stored", "account_id", session.Account.ID, "subscription_id", row.ID)
	return model.PushSubscription{
		ID: row.ID, DeviceName: row.DeviceName.String, CreatedAt: time.UnixMilli(row.CreatedAt).UTC(),
		ThisDevice: device != "" && row.DeviceID.String == device,
	}, nil
}

// DeletePushSubscription removes a subscription of the session's account (model.ErrNotFound for an
// unknown one or one of another account; model.ErrPrimaryProfileRequired, see requireManager).
func (s *Service) DeletePushSubscription(ctx context.Context, session model.Session, id int64) error {
	if err := requireManager(session); err != nil {
		return err
	}
	n, err := s.write.PushSubscriptionDelete(ctx, systemq.PushSubscriptionDeleteParams{ID: id, AccountID: session.Account.ID})
	if err != nil {
		return fmt.Errorf("delete push subscription: %w", err)
	}
	if n == 0 {
		return model.ErrNotFound
	}
	return nil
}
