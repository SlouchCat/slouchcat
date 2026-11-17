package wishes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/seerr"
	"github.com/slouchcat/slouchcat/server/internal/store/wishq"
)

// The limit per account (concept 25, architecture 19, build step 1.5c): «Standard» (no row, the
// default of the settings), «Eigenes» (custom) or «Unbegrenzt» (unlimited, limit 0). Slouchcat
// always writes numbers into the four limit fields of the Seerr user, never null (null would be the
// instance's default, appendix I, I-32), and nothing for a user with MANAGE_USERS («Kein Limit»,
// I-34). When an account is linked to an existing user and has no own limit yet, Slouchcat takes
// over the user's limit (N-112). The state comes from GET /user/{id}/quota.

// Bounds of a limit (data model 2.13).
const (
	limitMax   = 100
	daysMin    = 1
	daysMax    = 90
	nextFreeOf = 1 // read «nächster Platz frei» when at most this many places are left
	requestsOf = 50
)

// SetAccountLimit sets the limit of an account (setAccountWishLimit): default deletes its row,
// custom needs both halves (limit 0 to 100, days 1 to 90), unlimited none. A ready Seerr user gets
// it written by the job seerr.provision. model.ErrNotFound for an unknown account.
func (s *Service) SetAccountLimit(ctx context.Context, accountID int64, mode string, movie, tv *model.WishLimit, by int64) (model.AccountWishLimit, error) {
	verr := &model.ValidationError{}
	switch mode {
	case model.WishLimitDefault, model.WishLimitUnlimited:
		if movie != nil {
			_ = verr.Add("movie", "not_allowed")
		}
		if tv != nil {
			_ = verr.Add("tv", "not_allowed")
		}
	case model.WishLimitCustom:
		checkLimit(verr, "movie", movie)
		checkLimit(verr, "tv", tv)
	default:
		_ = verr.Add("mode", "invalid_value")
	}
	if len(verr.Fields) > 0 {
		return model.AccountWishLimit{}, verr
	}
	ready := false
	err := s.inTx(ctx, func(q *wishq.Queries) error {
		if n, err := q.AccountExists(ctx, accountID); err != nil {
			return fmt.Errorf("account: %w", err)
		} else if n == 0 {
			return model.ErrNotFound
		}
		if mode == model.WishLimitDefault {
			if err := q.DeleteAccountWishLimit(ctx, accountID); err != nil {
				return fmt.Errorf("delete limit: %w", err)
			}
		} else {
			p := wishq.UpsertAccountWishLimitParams{AccountID: accountID, Mode: mode, Now: s.now().UnixMilli(), SetBy: sql.NullInt64{Int64: by, Valid: by > 0}}
			if mode == model.WishLimitCustom {
				p.MovieLimit, p.MovieDays = num(movie.Limit), num(movie.Days)
				p.TvLimit, p.TvDays = num(tv.Limit), num(tv.Days)
			}
			if err := q.UpsertAccountWishLimit(ctx, p); err != nil {
				return fmt.Errorf("store limit: %w", err)
			}
		}
		if err := q.ClearMappingLimitWritten(ctx, accountID); err != nil {
			return fmt.Errorf("limit written: %w", err)
		}
		m, err := q.GetAccountMapping(ctx, accountID)
		ready = err == nil && m.State == StateReady
		return nil
	})
	if err != nil {
		return model.AccountWishLimit{}, err
	}
	s.log.InfoContext(ctx, "wish limit of account set", "account_id", accountID, "mode", mode)
	if ready {
		if err := s.queueProvision(ctx, provisionPayload{AccountID: accountID, Intent: intentSync}, jobs.ManualPriority); err != nil {
			s.log.WarnContext(ctx, "limit of account not queued", "account_id", accountID, "err", err)
		}
	}
	a, err := s.accountMapping(ctx, accountID)
	if err != nil {
		return model.AccountWishLimit{}, err
	}
	return a.Limit, nil
}

func checkLimit(verr *model.ValidationError, field string, l *model.WishLimit) {
	switch {
	case l == nil:
		_ = verr.Add(field, "required")
	case l.Limit < 0 || l.Limit > limitMax:
		_ = verr.Add(field+".limit", "out_of_range")
	case l.Days < daysMin || l.Days > daysMax:
		_ = verr.Add(field+".days", "out_of_range")
	}
}

func num(n int) sql.NullInt64 { return sql.NullInt64{Int64: int64(n), Valid: true} }

// adoptLimit takes over the limit a linked or chosen user has in Seerr as the limit of the account
// (N-112), unless the account has an own limit or the user has MANAGE_USERS: all four fields empty
// stays «Standard»; both limits 0 is «Unbegrenzt»; otherwise «Eigenes» with Seerr's values, an
// empty field from the default, clamped to 0..100 and 1..90 days. The limit is written back by the
// caller (writeLimit), so a clamped or filled value reaches Seerr too.
func (s *Service) adoptLimit(ctx context.Context, q *wishq.Queries, accountID int64, raw model.SeerrUserQuota, manageUsers bool, def model.WishLimits, by int64) error {
	if manageUsers {
		return nil
	}
	if _, err := q.GetAccountWishLimit(ctx, accountID); err == nil {
		return nil // an own limit in Slouchcat wins
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("limit: %w", err)
	}
	if raw.MovieLimit == nil && raw.MovieDays == nil && raw.TVLimit == nil && raw.TVDays == nil {
		return nil
	}
	p := wishq.UpsertAccountWishLimitParams{AccountID: accountID, Now: s.now().UnixMilli(), SetBy: sql.NullInt64{Int64: by, Valid: by > 0}}
	if raw.MovieLimit != nil && *raw.MovieLimit == 0 && raw.TVLimit != nil && *raw.TVLimit == 0 {
		p.Mode = model.WishLimitUnlimited
	} else {
		p.Mode = model.WishLimitCustom
		p.MovieLimit = num(clamp(orDefault(raw.MovieLimit, def.Movie.Limit), 0, limitMax))
		p.MovieDays = num(clamp(orDefault(raw.MovieDays, def.Movie.Days), daysMin, daysMax))
		p.TvLimit = num(clamp(orDefault(raw.TVLimit, def.TV.Limit), 0, limitMax))
		p.TvDays = num(clamp(orDefault(raw.TVDays, def.TV.Days), daysMin, daysMax))
	}
	if err := q.UpsertAccountWishLimit(ctx, p); err != nil {
		return fmt.Errorf("adopt limit: %w", err)
	}
	s.log.InfoContext(ctx, "limit of the seerr user taken over", "account_id", accountID, "mode", p.Mode)
	return nil
}

func orDefault(p *int, def int) int {
	if p != nil {
		return *p
	}
	return def
}

func clamp(n, lo, hi int) int { return min(max(n, lo), hi) }

// rawQuota is the four limit fields of a Seerr user as Seerr holds them.
func rawQuota(u seerr.User) model.SeerrUserQuota {
	return model.SeerrUserQuota{MovieLimit: u.MovieQuotaLimit, MovieDays: u.MovieQuotaDays, TVLimit: u.TVQuotaLimit, TVDays: u.TVQuotaDays}
}

// refreshQuota reads GET /user/{id}/quota and keeps what is used and, when the limit is (nearly)
// reached, when the next place is free (N-35): the oldest counting request in the window plus the
// days. Users with MANAGE_USERS have no limit and are skipped.
func (s *Service) refreshQuota(ctx context.Context, admin seerr.Session, seerrUserID int) error {
	row, err := s.read.GetSeerrUser(ctx, int64(seerrUserID))
	if err != nil {
		return fmt.Errorf("seerr user: %w", err)
	}
	if row.IsAdmin == 1 {
		return nil
	}
	q, err := admin.Quota(ctx, seerrUserID)
	if err != nil {
		return err
	}
	var movieFree, tvFree sql.NullInt64
	if near(q.Movie) || near(q.TV) {
		page, err := admin.Requests(ctx, seerr.RequestQuery{RequestedBy: seerrUserID, Sort: "added", SortDirection: "asc", Take: requestsOf})
		if err != nil {
			return err
		}
		if near(q.Movie) {
			movieFree = nextFree(page.Results, seerr.MediaTypeMovie, *q.Movie.Days, s.now())
		}
		if near(q.TV) {
			tvFree = nextFree(page.Results, seerr.MediaTypeTV, *q.TV.Days, s.now())
		}
	}
	return s.write.SetSeerrUserQuotaState(ctx, wishq.SetSeerrUserQuotaStateParams{
		MovieQuotaUsed: num(q.Movie.Used), TvQuotaUsed: num(q.TV.Used), MovieNextFreeAt: movieFree, TvNextFreeAt: tvFree,
		QuotaAt: num64(s.now().UnixMilli()), SeerrID: int64(seerrUserID),
	})
}

func num64(n int64) sql.NullInt64 { return sql.NullInt64{Int64: n, Valid: true} }

// near: a limited half with at most nextFreeOf places left.
func near(p seerr.QuotaPart) bool {
	return p.Limit != nil && *p.Limit > 0 && p.Days != nil && p.Remaining != nil && *p.Remaining <= nextFreeOf
}

// nextFree is the oldest request of the media type within the window that counts (declined ones
// do not) plus the days.
func nextFree(reqs []seerr.Request, mediaType string, days int, now time.Time) sql.NullInt64 {
	window := time.Duration(days) * 24 * time.Hour
	for _, r := range reqs { // oldest first
		if r.Type != mediaType || r.Status == seerr.RequestDeclined || !r.CreatedAt.After(now.Add(-window)) {
			continue
		}
		return num64(r.CreatedAt.Add(window).UnixMilli())
	}
	return sql.NullInt64{}
}

// wishQuota is the limit state of a user as the administration shows it: the effective limit with
// what Seerr counts, «Kein Limit» for MANAGE_USERS; nil while the state was never read.
func wishQuota(r wishq.SeerrUser, limit model.WishLimits) *model.WishQuota {
	if r.IsAdmin == 1 {
		return &model.WishQuota{Unlimited: true, Reason: model.WishQuotaReasonAdmin}
	}
	if !r.QuotaAt.Valid {
		return nil
	}
	at := time.UnixMilli(r.QuotaAt.Int64).UTC()
	out := &model.WishQuota{At: &at}
	if limit.Movie.Limit == 0 && limit.TV.Limit == 0 {
		out.Unlimited, out.Reason = true, model.WishQuotaReasonNoLimit
		return out
	}
	part := func(l model.WishLimit, used, next sql.NullInt64) *model.WishQuotaPart {
		if l.Limit == 0 {
			return nil
		}
		p := &model.WishQuotaPart{Limit: l.Limit, Days: l.Days, Used: int(used.Int64), Remaining: max(0, l.Limit-int(used.Int64))}
		p.NextFreeAt = timeOf(next)
		return p
	}
	out.Movie = part(limit.Movie, r.MovieQuotaUsed, r.MovieNextFreeAt)
	out.TV = part(limit.TV, r.TvQuotaUsed, r.TvNextFreeAt)
	return out
}

// defaultLimitChanged queues the accounts that follow the default and the shared user, so the new
// default reaches Seerr.
func (s *Service) defaultLimitChanged(ctx context.Context) error {
	ids, err := s.write.ClearDefaultLimitWritten(ctx)
	if err != nil {
		return fmt.Errorf("clear limits: %w", err)
	}
	for _, id := range ids {
		if err := s.queueProvision(ctx, provisionPayload{AccountID: id, Intent: intentSync}, provisionPriority); err != nil {
			return err
		}
	}
	return s.queueProvision(ctx, provisionPayload{Shared: true, SharedLimit: true}, provisionPriority)
}

// refreshQuotas reads the limit state of every ready user and of the shared user (the users job);
// a failure of one user is logged.
func (s *Service) refreshQuotas(ctx context.Context, admin seerr.Session) {
	rows, err := s.read.ListReadyMappings(ctx)
	if err != nil {
		s.log.WarnContext(ctx, "ready mappings not read", "err", err)
		return
	}
	ids := make([]int, 0, len(rows)+1)
	for _, r := range rows {
		ids = append(ids, int(r.SeerrUserID.Int64))
	}
	if sh, err := s.SharedUser(ctx); err == nil && sh != nil {
		ids = append(ids, sh.ID)
	}
	for _, id := range ids {
		if err := s.refreshQuota(ctx, admin, id); err != nil {
			s.log.WarnContext(ctx, "seerr quota not read", "seerr_user_id", id, "reason", errorClass(err))
		}
	}
}
