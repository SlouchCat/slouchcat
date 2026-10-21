package admin

import (
	"context"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// SettingTraktClientID is the settings key of the encrypted Trakt client id (M7, AN-3). The
// environment (SLOUCHCAT_TRAKT_CLIENT_ID, config.TraktClientID) wins; the settings page that stores
// it follows in M8.
const SettingTraktClientID = "trakt_client_id"

// Trakt is implemented by package metadata (M7, E-7, I-7): the state of the daily Trakt lists and a
// refresh out of schedule.
type Trakt interface {
	TraktStatus(ctx context.Context) (model.TraktStatus, error)
	// RefreshTrakt queues a refresh; model.ErrProviderNotConfigured without a client id.
	RefreshTrakt(ctx context.Context) error
}

// WithTrakt sets the Trakt lists and returns the receiver.
func (s *Service) WithTrakt(t Trakt) *Service {
	s.trakt = t
	return s
}

// TraktClientID returns the stored Trakt client id; "" when none is stored. The value never
// reaches the log or an error.
func (s *Service) TraktClientID(ctx context.Context) (string, error) {
	return s.secretKey(ctx, "", SettingTraktClientID)
}

// TraktStatus returns the state of the Trakt lists (ErrNotConfigured without WithTrakt).
func (s *Service) TraktStatus(ctx context.Context) (model.TraktStatus, error) {
	if s.trakt == nil {
		return model.TraktStatus{}, ErrNotConfigured
	}
	return s.trakt.TraktStatus(ctx)
}

// RefreshTrakt queues a refresh of the Trakt lists; model.ErrProviderNotConfigured without a
// client id, ErrNotConfigured without WithTrakt.
func (s *Service) RefreshTrakt(ctx context.Context) error {
	if s.trakt == nil {
		return ErrNotConfigured
	}
	return s.trakt.RefreshTrakt(ctx)
}
