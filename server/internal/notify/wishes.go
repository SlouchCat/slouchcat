package notify

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/i18n"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// Notifications about wishes (concept 25, 06-benachrichtigungen.md). Package wishes decides when a
// wish changes and calls one function per event; notify finds the requesters (the owner and the
// profiles that joined, F-7), checks per profile what it may see (F-9, F-13), writes one inbox
// entry per profile under a dedupe key and sends the event on the channels of the profile's
// account, prefixed with the profile's name. A profile with a PIN gets external messages without
// the title (F-15, setting wishes.pin_external_hide_title); its inbox shows the title behind the PIN.

// Inbox kinds of wishes (notifications.kind, migration 00070).
const (
	KindWishAvailable       = "wish_available"
	KindWishSeasonAvailable = "wish_season_available"
	KindWishApproved        = "wish_approved"
	KindWishDeclined        = "wish_declined"
	KindWishFailed          = "wish_failed"
)

// settingPINHideTitle is admin.SettingWishesPINExternalHideTitle: "0" lets external messages of
// PIN profiles name the title; absent or any other value hides it (default on, F-15).
const settingPINHideTitle = "wishes.pin_external_hide_title"

// WishOutcome says what a call did per requester: Notified are the profiles that got the message
// now, Waiting those that may not see the title yet (package wishes asks again on its next run).
// Profiles notified before are in neither.
type WishOutcome struct {
	Notified []int64
	Waiting  []int64
}

// WishAvailable announces that wish wishID can be played: itemID is the library title that
// fulfils it, a film or, for a series, the series once every wished season is available (section
// 3.3). Each open requester that may see itemID and was not notified before (wish_requesters.
// notified_at, also after its inbox entry was cleared away) gets an inbox entry with the key
// wish:<id>:available and the event wish_available; notified_at is set in the same transaction.
// A requester that may not see the title yet is returned as waiting. For a series, once nobody
// waits, every wished season is marked notified too, so no season message follows (section 7).
// model.ErrNotFound for an unknown wish. A failure to send is logged; the entry and notified_at
// stay.
func (s *Service) WishAvailable(ctx context.Context, wishID int64, itemID string) (WishOutcome, error) {
	wish, err := s.write.NotifyWish(ctx, wishID)
	if errors.Is(err, sql.ErrNoRows) {
		return WishOutcome{}, model.ErrNotFound
	}
	if err != nil {
		return WishOutcome{}, fmt.Errorf("wish %d: %w", wishID, err)
	}
	requesters, err := s.write.NotifyWishRequesters(ctx, wishID)
	if err != nil {
		return WishOutcome{}, fmt.Errorf("requesters of wish %d: %w", wishID, err)
	}
	hide, err := s.hideTitleForPIN(ctx)
	if err != nil {
		return WishOutcome{}, err
	}
	var out WishOutcome
	var errs []error
	for _, r := range requesters {
		if r.NotifiedAt.Valid {
			continue
		}
		p := requesterProfile(r)
		item, err := s.vis.Summary(ctx, visibility.Profile(p), itemID)
		if errors.Is(err, model.ErrNotFound) {
			out.Waiting = append(out.Waiting, p.ID)
			continue
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("visibility of %s for profile %d: %w", itemID, p.ID, err))
			continue
		}
		created, err := s.claimAvailable(ctx, wish.ID, p.ID, itemID)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !created {
			continue // notified by a concurrent run
		}
		out.Notified = append(out.Notified, p.ID)
		ev := model.Event{Type: model.EventWishAvailable, AccountID: p.AccountID, ProfileID: p.ID, ItemID: itemID,
			Subject: text(p.Language, "wish_available_subject", vars{"title": item.Title}),
			Body:    text(p.Language, "wish_available_"+wish.Media+"_body"), Link: "/items/" + itemID}
		if hide && r.HasPin {
			ev = hiddenWishEvent(ev, p.Language)
		}
		if err := s.Emit(ctx, ev); err != nil {
			s.log.WarnContext(ctx, "send wish available", "wish_id", wish.ID, "profile_id", p.ID, "err", err)
		}
	}
	if wish.Media == visibility.MediaShow && len(out.Waiting) == 0 && len(errs) == 0 {
		if err := s.write.NotifyWishSeasonsNotifiedAll(ctx, systemq.NotifyWishSeasonsNotifiedAllParams{
			Now: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}, WishID: wish.ID,
		}); err != nil {
			errs = append(errs, fmt.Errorf("seasons of wish %d notified: %w", wish.ID, err))
		}
	}
	return out, errors.Join(errs...)
}

// WishSeason is one wished season of a series wish that may have become available (section 3.3).
type WishSeason struct {
	WishID   int64
	SeriesID string // the library series of the wish
	Season   int    // season number, 1 or more
	// Episodes is the number of episodes that makes the season complete: the season's episode
	// count by series_catalog (before it exists: external_titles.seasons), for a season still on
	// air the episodes aired so far.
	Episodes int
	Airing   bool // the season is still on air: «the episodes aired so far are here»
}

// WishSeasonAvailable announces one season of a series wish that does not complete the wish (the
// last missing season is WishAvailable of the series). Each open requester not notified about the
// whole wish whose playable, visible episodes of the season reach in.Episodes gets an inbox entry
// wish_season_available (key wish:<id>:season:<n>, the season as its title) and the event
// wish_available; a requester with fewer is waiting. Once nobody waits, wish_seasons.notified_at is
// set and later calls send nothing. model.ErrNotFound for an unknown wish, a film wish or a season
// the wish lacks.
func (s *Service) WishSeasonAvailable(ctx context.Context, in WishSeason) (WishOutcome, error) {
	if in.Season < 1 || in.Episodes < 1 {
		return WishOutcome{}, fmt.Errorf("season %d with %d episodes", in.Season, in.Episodes)
	}
	wish, err := s.write.NotifyWish(ctx, in.WishID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && wish.Media != visibility.MediaShow) {
		return WishOutcome{}, model.ErrNotFound
	}
	if err != nil {
		return WishOutcome{}, fmt.Errorf("wish %d: %w", in.WishID, err)
	}
	season, err := s.write.NotifyWishSeason(ctx, systemq.NotifyWishSeasonParams{WishID: wish.ID, SeasonNumber: int64(in.Season)})
	if errors.Is(err, sql.ErrNoRows) {
		return WishOutcome{}, model.ErrNotFound
	}
	if err != nil {
		return WishOutcome{}, fmt.Errorf("season %d of wish %d: %w", in.Season, wish.ID, err)
	}
	if season.NotifiedAt.Valid {
		return WishOutcome{}, nil
	}
	requesters, err := s.write.NotifyWishRequesters(ctx, wish.ID)
	if err != nil {
		return WishOutcome{}, fmt.Errorf("requesters of wish %d: %w", wish.ID, err)
	}
	seasonID, err := s.write.NotifySeasonItem(ctx, systemq.NotifySeasonItemParams{
		SeriesID: sql.NullString{String: in.SeriesID, Valid: true}, SeasonNumber: sql.NullInt64{Int64: int64(in.Season), Valid: true},
	})
	missing := errors.Is(err, sql.ErrNoRows)
	if err != nil && !missing {
		return WishOutcome{}, fmt.Errorf("season %d of %s: %w", in.Season, in.SeriesID, err)
	}
	hide, err := s.hideTitleForPIN(ctx)
	if err != nil {
		return WishOutcome{}, err
	}
	key := wishKey(wish.ID, "season:"+strconv.Itoa(in.Season))
	textKey := "wish_season"
	if in.Airing {
		textKey = "wish_season_aired"
	}
	var out WishOutcome
	var errs []error
	for _, r := range requesters {
		if r.NotifiedAt.Valid {
			continue // the whole wish was announced
		}
		p := requesterProfile(r)
		if missing {
			out.Waiting = append(out.Waiting, p.ID)
			continue
		}
		series, complete, err := s.seasonComplete(ctx, p, in, seasonID)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !complete {
			out.Waiting = append(out.Waiting, p.ID)
			continue
		}
		res, err := s.write.NotifyWishInsert(ctx, systemq.NotifyWishInsertParams{
			ProfileID: p.ID, Kind: KindWishSeasonAvailable, ItemID: sql.NullString{String: seasonID, Valid: true},
			ParentID: sql.NullString{String: in.SeriesID, Valid: true}, WishID: sql.NullInt64{Int64: wish.ID, Valid: true},
			DedupeKey: key, Now: s.now().UnixMilli(),
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("store notification: %w", err))
			continue
		}
		if n, err := res.RowsAffected(); err != nil || n == 0 {
			if err != nil {
				errs = append(errs, err)
			}
			continue // notified before
		}
		out.Notified = append(out.Notified, p.ID)
		ev := model.Event{Type: model.EventWishAvailable, AccountID: p.AccountID, ProfileID: p.ID, ItemID: seasonID,
			Subject: text(p.Language, textKey+"_subject", vars{"title": series.Title, "season": in.Season}),
			Body:    text(p.Language, textKey+"_body"), Link: "/items/" + seasonID}
		if hide && r.HasPin {
			ev = hiddenWishEvent(ev, p.Language)
		}
		if err := s.Emit(ctx, ev); err != nil {
			s.log.WarnContext(ctx, "send wish season", "wish_id", wish.ID, "season", in.Season, "profile_id", p.ID, "err", err)
		}
	}
	if len(out.Waiting) == 0 && len(errs) == 0 {
		if err := s.write.NotifyWishSeasonNotified(ctx, systemq.NotifyWishSeasonNotifiedParams{
			Now: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}, WishID: wish.ID, SeasonNumber: int64(in.Season),
		}); err != nil {
			errs = append(errs, fmt.Errorf("season %d of wish %d notified: %w", in.Season, wish.ID, err))
		}
	}
	return out, errors.Join(errs...)
}

// seasonComplete reports whether the profile can play in.Episodes episodes of the season, with
// the series summary for the texts; a series or season the profile does not see is not complete.
func (s *Service) seasonComplete(ctx context.Context, p model.Profile, in WishSeason, seasonID string) (model.ItemSummary, bool, error) {
	scope := visibility.Profile(p)
	counts, err := s.vis.VisibleSeasons(ctx, scope, in.SeriesID)
	if errors.Is(err, model.ErrNotFound) {
		return model.ItemSummary{}, false, nil
	}
	if err != nil {
		return model.ItemSummary{}, false, fmt.Errorf("seasons of %s for profile %d: %w", in.SeriesID, p.ID, err)
	}
	if counts[in.Season] < in.Episodes {
		return model.ItemSummary{}, false, nil
	}
	if err := s.vis.Visible(ctx, scope, seasonID); errors.Is(err, model.ErrNotFound) {
		return model.ItemSummary{}, false, nil
	} else if err != nil {
		return model.ItemSummary{}, false, fmt.Errorf("visibility of %s for profile %d: %w", seasonID, p.ID, err)
	}
	series, err := s.vis.Summary(ctx, scope, in.SeriesID)
	if errors.Is(err, model.ErrNotFound) {
		return model.ItemSummary{}, false, nil
	}
	if err != nil {
		return model.ItemSummary{}, false, fmt.Errorf("series %s for profile %d: %w", in.SeriesID, p.ID, err)
	}
	return series, true, nil
}

// WishApproved announces that Seerr approved wish wishID (only the change from pending to
// processing, not an approval when the wish was created; section 2). WishDeclined and WishFailed
// are the other two status events. Each open requester gets an inbox entry without a title (key
// wish:<id>:approved, …, read through the wish) and the event wish_status, which the channels
// choose only by hand (off by default, F-14). A second call sends nothing. model.ErrNotFound for an
// unknown wish.
func (s *Service) WishApproved(ctx context.Context, wishID int64) error {
	return s.wishStatus(ctx, wishID, KindWishApproved, "wish_approved_body")
}

// WishDeclined announces that Seerr declined wish wishID (Seerr gives no reason); see WishApproved.
func (s *Service) WishDeclined(ctx context.Context, wishID int64) error {
	return s.wishStatus(ctx, wishID, KindWishDeclined, "wish_declined_body")
}

// WishFailed announces that wish wishID failed for good; reason is wishes.fail_reason (quota,
// blocklisted, …) and selects the plain text of the message, an unknown one a general text; see
// WishApproved.
func (s *Service) WishFailed(ctx context.Context, wishID int64, reason string) error {
	body := "wish_failed_body"
	if _, ok := texts.Raw(i18n.Fallback, "wish_failed_"+reason); ok && reason != "" {
		body = "wish_failed_" + reason
	}
	return s.wishStatus(ctx, wishID, KindWishFailed, body)
}

// wishStatus stores the status entry of every open requester and sends wish_status for each new one.
func (s *Service) wishStatus(ctx context.Context, wishID int64, kind, bodyKey string) error {
	wish, err := s.write.NotifyWish(ctx, wishID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("wish %d: %w", wishID, err)
	}
	requesters, err := s.write.NotifyWishRequesters(ctx, wishID)
	if err != nil {
		return fmt.Errorf("requesters of wish %d: %w", wishID, err)
	}
	hide, err := s.hideTitleForPIN(ctx)
	if err != nil {
		return err
	}
	what := strings.TrimPrefix(kind, "wish_")
	var errs []error
	for _, r := range requesters {
		p := requesterProfile(r)
		res, err := s.write.NotifyWishInsert(ctx, systemq.NotifyWishInsertParams{
			ProfileID: p.ID, Kind: kind, WishID: sql.NullInt64{Int64: wish.ID, Valid: true},
			DedupeKey: wishKey(wish.ID, what), Now: s.now().UnixMilli(),
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("store notification: %w", err))
			continue
		}
		if n, err := res.RowsAffected(); err != nil || n == 0 {
			if err != nil {
				errs = append(errs, err)
			}
			continue // notified before
		}
		ev := model.Event{Type: model.EventWishStatus, AccountID: p.AccountID, ProfileID: p.ID,
			Subject: text(p.Language, kind+"_subject", vars{"title": wish.Title}), Body: text(p.Language, bodyKey),
			Link: "/wishes?focus=" + strconv.FormatInt(wish.ID, 10)}
		if hide && r.HasPin {
			ev.Subject, ev.Body, ev.Link = text(p.Language, "wish_status_hidden_subject"), text(p.Language, "wish_hidden_body"), "/wishes"
		}
		if err := s.Emit(ctx, ev); err != nil {
			s.log.WarnContext(ctx, "send wish status", "wish_id", wish.ID, "kind", kind, "profile_id", p.ID, "err", err)
		}
	}
	return errors.Join(errs...)
}

// WishReview tells the administrators that wish wishID waits for approval in Seerr (event
// wish_review, chosen by administrators only; no inbox entry, no profile prefix, section 8). The
// text names the title and «account · profile» of the owner, in the owner's language like the
// event errors. Once per wish: a second call sends nothing while the deliveries of the first are
// kept. model.ErrNotFound for an unknown wish or one without owner.
func (s *Service) WishReview(ctx context.Context, wishID int64) error {
	wish, err := s.write.NotifyWish(ctx, wishID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("wish %d: %w", wishID, err)
	}
	owner, err := s.write.NotifyWishOwner(ctx, wishID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("owner of wish %d: %w", wishID, err)
	}
	source := wishKey(wish.ID, "review")
	s.errorsMu.Lock() // the same lock as the dedupe of errors: two calls in the same moment send once
	defer s.errorsMu.Unlock()
	sent, err := s.write.DeliveryRecentExists(ctx, systemq.DeliveryRecentExistsParams{
		Event: model.EventWishReview, Source: sql.NullString{String: source, Valid: true}, Since: 0,
	})
	if err != nil {
		return fmt.Errorf("dedupe wish review: %w", err)
	}
	if sent {
		return nil
	}
	lang := s.ownerLanguage(ctx)
	return s.Emit(ctx, model.Event{
		Type: model.EventWishReview, Source: source, Subject: text(lang, "wish_review_subject", vars{"title": wish.Title}),
		Body: text(lang, "wish_review_"+wish.Media+"_body", vars{"account": owner.Email, "profile": owner.Name}),
		Link: "/admin/wishes?state=pending",
	})
}

// claimAvailable sets notified_at of the requester and stores its inbox entry in one transaction;
// false when another run set notified_at first or the entry exists already.
func (s *Service) claimAvailable(ctx context.Context, wishID, profileID int64, itemID string) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.write.WithTx(tx)
	now := s.now().UnixMilli()
	claimed, err := q.NotifyWishRequesterClaim(ctx, systemq.NotifyWishRequesterClaimParams{
		Now: sql.NullInt64{Int64: now, Valid: true}, WishID: wishID, ProfileID: profileID,
	})
	if err != nil {
		return false, fmt.Errorf("notified_at of wish %d, profile %d: %w", wishID, profileID, err)
	}
	if claimed == 0 {
		return false, nil
	}
	res, err := q.NotifyWishInsert(ctx, systemq.NotifyWishInsertParams{
		ProfileID: profileID, Kind: KindWishAvailable, ItemID: sql.NullString{String: itemID, Valid: true},
		WishID: sql.NullInt64{Int64: wishID, Valid: true}, DedupeKey: wishKey(wishID, "available"), Now: now,
	})
	if err != nil {
		return false, fmt.Errorf("store notification: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("store notification: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit: %w", err)
	}
	return n > 0, nil
}

// wishKey is the dedupe key of a wish entry: wish:<id>:<what> (section 7).
func wishKey(wishID int64, what string) string {
	return "wish:" + strconv.FormatInt(wishID, 10) + ":" + what
}

// hiddenWishEvent is the external message of a PIN profile (F-15): no title in subject, body and
// link; the link leads to the wishes behind the PIN.
func hiddenWishEvent(ev model.Event, lang string) model.Event {
	ev.Subject, ev.Body, ev.Link, ev.ItemID = text(lang, "wish_hidden_subject"), text(lang, "wish_hidden_body"), "/wishes", ""
	return ev
}

// hideTitleForPIN reads wishes.pin_external_hide_title (default on).
func (s *Service) hideTitleForPIN(ctx context.Context) (bool, error) {
	v, err := s.read.GetSystemSetting(ctx, settingPINHideTitle)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("setting %s: %w", settingPINHideTitle, err)
	}
	return v != "0", nil
}

// requesterProfile is the profile of a requester row, as far as visibility and the texts need it.
func requesterProfile(r systemq.NotifyWishRequestersRow) model.Profile {
	return model.Profile{ID: r.ProfileID, AccountID: r.AccountID, Name: r.Name, MaxRating: int(r.MaxRating),
		Language: r.Language, MetadataLanguage: r.MetadataLanguage.String, HasPin: r.HasPin}
}
