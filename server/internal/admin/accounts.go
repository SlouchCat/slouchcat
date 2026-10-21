package admin

import (
	"context"
	"fmt"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// ListAccounts returns every account with its profiles, library rights, last use and the switch of
// each notification channel (A-2): the main account first, then the guests, newest first.
func (s *Service) ListAccounts(ctx context.Context) ([]model.AdminAccount, error) {
	accounts, err := s.read.ListAccountsAdmin(ctx)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	profiles, err := s.read.ListProfilesAdmin(ctx)
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}
	libraries, err := s.read.ListAccountLibrariesAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list account libraries: %w", err)
	}
	channels, err := s.sys.ChannelSummaries(ctx)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	byAccount := make(map[int64]*model.AdminAccount, len(accounts))
	out := make([]model.AdminAccount, len(accounts))
	for i, a := range accounts {
		out[i] = model.AdminAccount{
			ID: a.ID, Email: a.Email, Role: a.Role, MaxRating: int(a.MaxRating), AllLibraries: a.AllLibraries == 1,
			LibraryIDs: []int64{}, Profiles: []model.ProfileRef{}, Channels: []model.ChannelSummary{},
			CreatedAt: time.UnixMilli(a.CreatedAt).UTC(),
		}
		if a.LastSeenAt.Valid {
			t := time.UnixMilli(a.LastSeenAt.Int64).UTC()
			out[i].LastSeenAt = &t
		}
		byAccount[a.ID] = &out[i]
	}
	for _, p := range profiles {
		if a := byAccount[p.AccountID]; a != nil {
			a.Profiles = append(a.Profiles, model.ProfileRef{ID: p.ID, Name: p.Name, MaxRating: int(p.MaxRating), IsPrimary: p.IsDefault == 1})
		}
	}
	for _, l := range libraries {
		if a := byAccount[l.AccountID]; a != nil && !a.AllLibraries {
			a.LibraryIDs = append(a.LibraryIDs, l.LibraryID)
		}
	}
	for _, c := range channels {
		if a := byAccount[c.AccountID]; a != nil {
			a.Channels = append(a.Channels, model.ChannelSummary{Channel: c.Channel, Enabled: c.Enabled == 1})
		}
	}
	return out, nil
}
