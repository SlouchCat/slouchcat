package notify

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// Announcements of the main user (A-7, plan 5.2). The administration writes, publishes and deletes
// them through admin (admin.Announcements); publishing sends one to every account that chose the
// event announcements (guests have e-mail on by default) and every profile sees it at its next
// start until it acknowledges it.

const (
	maxAnnouncementTitle = 120
	maxAnnouncementBody  = 4000
	// MaxOpenAnnouncements is the most open announcements a profile gets at once (contract).
	MaxOpenAnnouncements = 20
)

func validateAnnouncement(title, body string) (string, string, error) {
	title, body = strings.TrimSpace(title), strings.TrimSpace(body)
	var verr *model.ValidationError
	switch n := utf8.RuneCountInString(title); {
	case n == 0:
		verr = verr.Add("title", "required")
	case n > maxAnnouncementTitle:
		verr = verr.Add("title", "too_long")
	}
	switch n := utf8.RuneCountInString(body); {
	case n == 0:
		verr = verr.Add("body", "required")
	case n > maxAnnouncementBody:
		verr = verr.Add("body", "too_long")
	}
	if verr != nil {
		return "", "", verr
	}
	return title, body, nil
}

// ListAnnouncements returns every announcement, newest first, with how many profiles acknowledged
// it.
func (s *Service) ListAnnouncements(ctx context.Context) ([]model.Announcement, error) {
	rows, err := s.read.AnnouncementList(ctx)
	if err != nil {
		return nil, fmt.Errorf("list announcements: %w", err)
	}
	out := make([]model.Announcement, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAnnouncement(systemq.AnnouncementGetRow(r)))
	}
	return out, nil
}

func toAnnouncement(r systemq.AnnouncementGetRow) model.Announcement {
	a := model.Announcement{
		ID: r.ID, Title: r.Title, Body: r.Body, CreatedAt: time.UnixMilli(r.CreatedAt).UTC(), UpdatedAt: time.UnixMilli(r.UpdatedAt).UTC(),
		CreatedBy: r.CreatedByEmail, AcknowledgedCount: int(r.AcknowledgedCount), ProfileCount: int(r.ProfileCount),
	}
	if r.PublishedAt.Valid {
		t := time.UnixMilli(r.PublishedAt.Int64).UTC()
		a.PublishedAt = &t
	}
	return a
}

func (s *Service) announcement(ctx context.Context, id int64) (model.Announcement, error) {
	r, err := s.write.AnnouncementGet(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Announcement{}, model.ErrNotFound
	}
	if err != nil {
		return model.Announcement{}, fmt.Errorf("announcement %d: %w", id, err)
	}
	return toAnnouncement(r), nil
}

// CreateAnnouncement writes an announcement of creator and publishes it when in.Publish
// (*model.ValidationError: title 1 to 120, body 1 to 4000 characters).
func (s *Service) CreateAnnouncement(ctx context.Context, creator model.Account, in model.AnnouncementInput) (model.Announcement, error) {
	title, body, err := validateAnnouncement(in.Title, in.Body)
	if err != nil {
		return model.Announcement{}, err
	}
	now := s.now().UnixMilli()
	id, err := s.write.AnnouncementInsert(ctx, systemq.AnnouncementInsertParams{
		Title: title, Body: body, CreatedBy: sql.NullInt64{Int64: creator.ID, Valid: creator.ID != 0}, Now: now,
		PublishedAt: sql.NullInt64{Int64: now, Valid: in.Publish},
	})
	if err != nil {
		return model.Announcement{}, fmt.Errorf("store announcement: %w", err)
	}
	if in.Publish {
		s.sendAnnouncement(ctx, id, title, body)
	}
	return s.announcement(ctx, id)
}

// UpdateAnnouncement changes a draft: model.ErrNotFound, model.ErrAnnouncementPublished,
// *model.ValidationError.
func (s *Service) UpdateAnnouncement(ctx context.Context, id int64, title, body string) (model.Announcement, error) {
	title, body, err := validateAnnouncement(title, body)
	if err != nil {
		return model.Announcement{}, err
	}
	n, err := s.write.AnnouncementUpdate(ctx, systemq.AnnouncementUpdateParams{Title: title, Body: body, Now: s.now().UnixMilli(), ID: id})
	if err != nil {
		return model.Announcement{}, fmt.Errorf("update announcement: %w", err)
	}
	if n == 0 {
		a, err := s.announcement(ctx, id)
		if err != nil {
			return model.Announcement{}, err
		}
		if a.PublishedAt != nil {
			return model.Announcement{}, model.ErrAnnouncementPublished
		}
	}
	return s.announcement(ctx, id)
}

// PublishAnnouncement publishes a draft and sends it on the channels; an announcement published
// before stays as it is and is not sent again (model.ErrNotFound for an unknown one).
func (s *Service) PublishAnnouncement(ctx context.Context, id int64) (model.Announcement, error) {
	n, err := s.write.AnnouncementPublish(ctx, systemq.AnnouncementPublishParams{Now: sql.NullInt64{Int64: s.now().UnixMilli(), Valid: true}, ID: id})
	if err != nil {
		return model.Announcement{}, fmt.Errorf("publish announcement: %w", err)
	}
	a, err := s.announcement(ctx, id)
	if err != nil {
		return model.Announcement{}, err
	}
	if n == 1 {
		s.sendAnnouncement(ctx, id, a.Title, a.Body)
	}
	return a, nil
}

// sendAnnouncement emits the announcement to every account that chose it. A failure is logged: the
// announcement is published and every profile sees it in the app.
func (s *Service) sendAnnouncement(ctx context.Context, id int64, title, body string) {
	// Accounts whose channel rows are missing get the defaults of 5.2 first (guests: e-mail on).
	if err := s.write.ChannelEnsureDefaultsAll(ctx, s.now().UnixMilli()); err != nil {
		s.log.WarnContext(ctx, "default channels before an announcement", "announcement_id", id, "err", err)
	}
	if err := s.Emit(ctx, model.Event{Type: model.EventAnnouncements, Subject: title, Body: body, Link: "/"}); err != nil {
		s.log.WarnContext(ctx, "send announcement", "announcement_id", id, "err", err)
	}
}

// DeleteAnnouncement deletes an announcement with its acknowledgements (model.ErrNotFound).
func (s *Service) DeleteAnnouncement(ctx context.Context, id int64) error {
	n, err := s.write.AnnouncementDelete(ctx, id)
	if err != nil {
		return fmt.Errorf("delete announcement: %w", err)
	}
	if n == 0 {
		return model.ErrNotFound
	}
	return nil
}

// Announcements returns the published announcements the profile has not acknowledged, newest first,
// at most MaxOpenAnnouncements.
func (s *Service) Announcements(ctx context.Context, profile model.Profile) ([]model.ProfileAnnouncement, error) {
	rows, err := s.read.AnnouncementsOpenForProfile(ctx, systemq.AnnouncementsOpenForProfileParams{
		ProfileID: profile.ID, LimitCount: MaxOpenAnnouncements,
	})
	if err != nil {
		return nil, fmt.Errorf("open announcements: %w", err)
	}
	out := make([]model.ProfileAnnouncement, 0, len(rows))
	for _, r := range rows {
		out = append(out, model.ProfileAnnouncement{ID: r.ID, Title: r.Title, Body: r.Body, PublishedAt: time.UnixMilli(r.PublishedAt.Int64).UTC()})
	}
	return out, nil
}

// AckAnnouncement acknowledges a published announcement for the profile (idempotent;
// model.ErrNotFound for an unknown or unpublished one).
func (s *Service) AckAnnouncement(ctx context.Context, profile model.Profile, id int64) error {
	n, err := s.write.AnnouncementAck(ctx, systemq.AnnouncementAckParams{ProfileID: profile.ID, Now: s.now().UnixMilli(), ID: id})
	if err != nil {
		return fmt.Errorf("acknowledge announcement: %w", err)
	}
	if n == 1 {
		return nil
	}
	published, err := s.write.AnnouncementIsPublished(ctx, id)
	if err != nil {
		return fmt.Errorf("announcement %d: %w", id, err)
	}
	if !published {
		return model.ErrNotFound
	}
	return nil // acknowledged before
}
