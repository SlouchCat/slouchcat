package admin

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// User picks in the administration (concept 18 part B, 14.7, ADR 0028): the list of the current
// picks, anonymous, and the retention. The profiles of the owner's account recommend on the title
// page (discover); the administration only removes and sets how long a pick stays.

const pickDay = 24 * time.Hour

// ListUserPicks returns every title with a pick that has not expired, newest pick first: the number
// of those picks, the day of the newest one in the server zone and the whole days until it expires,
// rounded up (1 = within the next 24 hours).
// Titles are plain text, never an ItemRef and never a profile; the poster is the content hash of the
// image that getAdminItemImage serves.
func (s *Service) ListUserPicks(ctx context.Context) ([]model.AdminUserPick, error) {
	cfg, err := s.sys.AdminGetPicksConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("get picks config: %w", err)
	}
	now := s.now()
	rows, err := s.sys.AdminListUserPicks(ctx, now.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("list user picks: %w", err)
	}
	retention := time.Duration(cfg.RetentionDays) * pickDay
	out := make([]model.AdminUserPick, 0, len(rows))
	for _, r := range rows {
		picked := time.UnixMilli(r.PickedAt)
		left := picked.Add(retention).Sub(now)
		out = append(out, model.AdminUserPick{
			ItemID: r.ItemID, Kind: r.Kind, Title: r.Title, Year: optInt(r.Year), Count: int(r.PickCount),
			PickedOn:      picked.In(s.zone).Format(time.DateOnly),
			ExpiresInDays: max(int((left+pickDay-1)/pickDay), 1), // listed means not expired: at least 1
		})
	}
	if err := withPosters(ctx, s.sys, out, userPickPoster); err != nil {
		return nil, err
	}
	return out, nil
}

// RemoveUserPick removes every pick of a title; model.ErrNotFound when it has none.
func (s *Service) RemoveUserPick(ctx context.Context, itemID string) error {
	n, err := systemq.New(s.db).AdminDeleteUserPicksOfItem(ctx, itemID)
	if err != nil {
		return fmt.Errorf("delete user picks of %s: %w", itemID, err)
	}
	if n == 0 {
		return model.ErrNotFound
	}
	s.log.InfoContext(ctx, "user picks removed", "item_id", itemID, "picks", n)
	return nil
}

// PicksRetention returns how many days a title stays after its newest pick.
func (s *Service) PicksRetention(ctx context.Context) (int, error) {
	cfg, err := s.sys.AdminGetPicksConfig(ctx)
	if err != nil {
		return 0, fmt.Errorf("get picks config: %w", err)
	}
	return int(cfg.RetentionDays), nil
}

// SetPicksRetention stores the retention: 7..365 days (retentionDays invalid_value). A shorter one
// takes effect at once, because every read filters by time.
func (s *Service) SetPicksRetention(ctx context.Context, days int) (int, error) {
	if days < model.PicksRetentionMin || days > model.PicksRetentionMax {
		return 0, (&model.ValidationError{}).Add("retentionDays", "invalid_value")
	}
	if err := systemq.New(s.db).AdminSetPicksConfig(ctx, systemq.AdminSetPicksConfigParams{
		RetentionDays: int64(days), Now: s.now().UnixMilli(),
	}); err != nil {
		return 0, fmt.Errorf("set picks config: %w", err)
	}
	return days, nil
}

// userPickPoster names the title of a pick and its poster hash for withPosters.
func userPickPoster(p *model.AdminUserPick) (string, *string) { return p.ItemID, &p.PosterHash }

func optInt(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int64)
	return &n
}

func nullText(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
