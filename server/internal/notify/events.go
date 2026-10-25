package notify

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Hooks of the packages that raise events (design 3.1): auth (an invitation was accepted), jobs (a
// job failed for good) and library (the loss protection aborted a scan). Each renders its texts in
// the recipient's language and calls Emit. NewEpisodes (notify.go) is the hook of library and
// metadata for new content.

// InvitationAccepted tells the account that created the invitation that a guest accepted it (event
// invitations; auth.WithInvitationAccepted).
func (s *Service) InvitationAccepted(ctx context.Context, inviterAccountID int64, guestEmail string) error {
	acc, err := s.write.NotifyAccount(ctx, inviterAccountID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil // the inviting account is gone
	}
	if err != nil {
		return fmt.Errorf("inviting account: %w", err)
	}
	return s.Emit(ctx, model.Event{
		Type: model.EventInvitations, AccountID: acc.ID, Subject: text(acc.Language, "invitation_subject"),
		Body: text(acc.Language, "invitation_body", guestEmail), Link: "/admin/access",
	})
}

// JobFailed tells the main account that a job failed for good (event errors, once per job type and
// hour; jobs.Worker.OnFinalFailure). A failed notify.deliver raises nothing: the channel that failed
// would carry the message about itself.
func (s *Service) JobFailed(ctx context.Context, j jobs.Job, cause error) {
	if j.Type == JobDeliver {
		return
	}
	lang := s.ownerLanguage(ctx)
	reason := ""
	if cause != nil {
		reason = errorText(cause)
	}
	if err := s.Emit(ctx, model.Event{
		Type: model.EventErrors, Source: "job:" + j.Type, Subject: text(lang, "job_failed_subject", j.Type),
		Body: text(lang, "job_failed_body", j.Type, j.Attempts, reason), Link: "/admin",
	}); err != nil {
		s.log.WarnContext(ctx, "notify job failure", "job_id", j.ID, "job_type", j.Type, "err", err)
	}
}

// ScanAborted tells the main account that the loss protection aborted a scan of a library (event
// errors, once per library and hour; library.WithScanAborted). reason is the abort code
// (root_missing, root_empty, loss_over_threshold).
func (s *Service) ScanAborted(ctx context.Context, libraryName, reason string) error {
	lang := s.ownerLanguage(ctx)
	return s.Emit(ctx, model.Event{
		Type: model.EventErrors, Source: "scan:" + libraryName, Subject: text(lang, "scan_aborted_subject", libraryName),
		Body: text(lang, "scan_aborted_body", libraryName, text(lang, reason)), Link: "/admin/libraries",
	})
}

// RootRemoved tells the main account that an administrator removed a folder from a library and
// how many titles and files went with it (event errors; library.WithRootRemoved, idea 02, E12).
// Removing 4000 titles should draw at least as much attention as an unplugged NAS cable, and the
// project keeps no audit table.
func (s *Service) RootRemoved(ctx context.Context, libraryName, path string, items, files int64) error {
	lang := s.ownerLanguage(ctx)
	return s.Emit(ctx, model.Event{
		Type: model.EventErrors, Source: "library:" + libraryName, Subject: text(lang, "root_removed_subject", libraryName),
		Body: text(lang, "root_removed_body", path, libraryName, items, files), Link: "/admin/libraries",
	})
}

// ownerLanguage is the language of the main account's primary profile (German without one).
func (s *Service) ownerLanguage(ctx context.Context) string {
	owner, err := s.write.NotifyOwner(ctx)
	if err != nil {
		return "de"
	}
	return owner.Language
}
