package notify

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

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
const KindWishAvailable = "wish_available"

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
// A requester that may not see the title yet is returned as waiting. model.ErrNotFound for an
// unknown wish. A failure to send is logged; the entry and notified_at stay.
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
	return out, errors.Join(errs...)
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
