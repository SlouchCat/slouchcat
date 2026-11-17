package wishes

import (
	"context"

	"github.com/slouchcat/slouchcat/server/internal/notify"
)

// NotifyAdapter is the Notifier over package notify (architecture 12): notify builds the messages,
// checks the visibility per profile and writes notified_at; this adapter only translates the
// types. A season with a target below 1 is never handed on.
func NotifyAdapter(n *notify.Service) Notifier { return notifyAdapter{n: n} }

type notifyAdapter struct{ n *notify.Service }

func (a notifyAdapter) WishAvailable(ctx context.Context, wishID int64, itemID string) (NotifyOutcome, error) {
	out, err := a.n.WishAvailable(ctx, wishID, itemID)
	return NotifyOutcome{Notified: out.Notified, Waiting: out.Waiting}, err
}

func (a notifyAdapter) WishSeasonAvailable(ctx context.Context, s SeasonNotice) (NotifyOutcome, error) {
	if s.Season < 1 || s.Episodes < 1 {
		return NotifyOutcome{}, nil
	}
	out, err := a.n.WishSeasonAvailable(ctx, notify.WishSeason{
		WishID: s.WishID, SeriesID: s.SeriesID, Season: s.Season, Episodes: s.Episodes, Airing: s.Airing,
	})
	return NotifyOutcome{Notified: out.Notified, Waiting: out.Waiting}, err
}

func (a notifyAdapter) WishApproved(ctx context.Context, wishID int64) error {
	return a.n.WishApproved(ctx, wishID)
}

func (a notifyAdapter) WishDeclined(ctx context.Context, wishID int64) error {
	return a.n.WishDeclined(ctx, wishID)
}

func (a notifyAdapter) WishFailed(ctx context.Context, wishID int64, reason string) error {
	return a.n.WishFailed(ctx, wishID, reason)
}

func (a notifyAdapter) WishReview(ctx context.Context, wishID int64) error {
	return a.n.WishReview(ctx, wishID)
}
