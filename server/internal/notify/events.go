package notify

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/i18n"
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
		Body: text(acc.Language, "invitation_body", vars{"email": guestEmail}), Link: "/admin/access",
	})
}

// DeviceLinked tells the account that a device was paired with it (event device_linked, concept 27
// K-7; auth.WithDeviceLinked): on every switched-on channel of the account, in the language of its
// primary profile. The text names what the server knows itself (the time and the kind of the
// device) and marks name and system version as the device's own claim, since whoever pairs chooses
// them.
func (s *Service) DeviceLinked(ctx context.Context, accountID int64, kind, name, system string, at time.Time) error {
	acc, err := s.write.NotifyAccount(ctx, accountID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil // the account is gone
	}
	if err != nil {
		return fmt.Errorf("paired account: %w", err)
	}
	return s.Emit(ctx, model.Event{
		Type: model.EventDeviceLinked, AccountID: acc.ID, Subject: text(acc.Language, "device_linked_subject"),
		Body: text(acc.Language, "device_linked_body", vars{
			"time": at.UTC().Format("2006-01-02 15:04") + " UTC", "kind": text(acc.Language, "device_kind_"+kind),
			"name": name, "system": system,
		}),
		Link: "/settings/devices", // «Meine Geräte» of the web app
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
		Type: model.EventErrors, Source: "job:" + j.Type, Subject: text(lang, "job_failed_subject", vars{"job": j.Type}),
		Body: text(lang, "job_failed_body", vars{"job": j.Type, "attempts": j.Attempts, "reason": reason}), Link: "/admin",
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
		Type: model.EventErrors, Source: "scan:" + libraryName, Subject: text(lang, "scan_aborted_subject", vars{"library": libraryName}),
		Body: text(lang, "scan_aborted_body", vars{"library": libraryName, "reason": text(lang, reason)}), Link: "/admin/libraries",
	})
}

// RootRemoved tells the main account that an administrator removed a folder from a library and
// how many titles and files went with it (event errors; library.WithRootRemoved, idea 02, E12).
// Removing 4000 titles should draw at least as much attention as an unplugged NAS cable, and the
// project keeps no audit table.
func (s *Service) RootRemoved(ctx context.Context, libraryName, path string, items, files int64) error {
	lang := s.ownerLanguage(ctx)
	return s.Emit(ctx, model.Event{
		Type: model.EventErrors, Source: "library:" + libraryName, Subject: text(lang, "root_removed_subject", vars{"library": libraryName}),
		Body: text(lang, "root_removed_body", vars{"path": path, "library": libraryName, "items": items, "files": files}), Link: "/admin/libraries",
	})
}

// RootContentChanged tells the main account that an administrator changed the content of a
// folder (ADR 0015): the folder is read afresh with another parser, and its titles left the
// library exactly as with a removal (event errors; library.WithRootContentChanged).
func (s *Service) RootContentChanged(ctx context.Context, libraryName, path, from, to string, items, files int64) error {
	lang := s.ownerLanguage(ctx)
	return s.Emit(ctx, model.Event{
		Type: model.EventErrors, Source: "library:" + libraryName, Subject: text(lang, "root_content_changed_subject", vars{"library": libraryName}),
		Body: text(lang, "root_content_changed_body", vars{
			"path": path, "library": libraryName, "from": text(lang, "content_"+from), "to": text(lang, "content_"+to), "items": items, "files": files,
		}),
		Link: "/admin/libraries",
	})
}

// UpdateInstallFailed tells the administration that an install by the updater companion ended
// rolled back or failed (event errors; admin.WithUpdater, concept 15 9.3, ADR 0024): from and to,
// what happened, and when the database was migrated already, the backup to go back with. The
// administration reports each install once; the source (target version and finish time) keeps a
// second send of the same install within the hour out, not the next failed install to the same
// version.
func (s *Service) UpdateInstallFailed(ctx context.Context, in model.UpdateInstall) error {
	lang := s.ownerLanguage(ctx)
	outcome := "update_failed"
	if in.Phase == "rolled_back" {
		outcome = "update_rolled_back"
	}
	from, to := orUnknown(lang, in.From), orUnknown(lang, in.To)
	parts := []string{text(lang, outcome+"_body", vars{"from": from, "to": to})}
	if in.Code != "" {
		parts = append(parts, text(lang, "update_code", vars{"code": in.Code}))
	}
	if in.Message != "" {
		parts = append(parts, in.Message)
	}
	switch {
	case in.Migrated && in.Backup != "":
		parts = append(parts, text(lang, "update_migrated_backup", vars{"backup": in.Backup}))
	case in.Migrated:
		parts = append(parts, text(lang, "update_migrated"))
	}
	return s.Emit(ctx, model.Event{
		Type: model.EventErrors, Source: updateSource(in), Subject: text(lang, outcome+"_subject", vars{"to": to}),
		Body: strings.Join(parts, "\n\n"), Link: "/admin",
	})
}

// updateSource is the event source of an install: its target version and finish time.
func updateSource(in model.UpdateInstall) string {
	finished := ""
	if in.FinishedAt != nil {
		finished = in.FinishedAt.UTC().Format(time.RFC3339Nano)
	}
	return "update:" + in.To + ":" + finished
}

func orUnknown(lang, version string) string {
	if version == "" {
		return text(lang, "update_version_unknown")
	}
	return version
}

// ownerLanguage is the language of the owner's primary profile (English without one, the last
// link of the text chain; concept 22 section 4.5). The texts of
// the event errors are written in one language, and with several administrators that language has
// to be a particular one, so it follows the owner (ADR 0014).
func (s *Service) ownerLanguage(ctx context.Context) string {
	owner, err := s.write.NotifyOwner(ctx)
	if err != nil {
		return i18n.Fallback
	}
	return owner.Language
}
