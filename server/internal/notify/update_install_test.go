package notify

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Concept 15 9.3 (ADR 0024): a rolled back or failed install reaches the administration as the
// event errors, with from and to, the reason, and when the database was migrated the backup that
// leads back. The administration reports each install once; this send is the event itself.
func TestUpdateInstallFailed(t *testing.T) {
	t.Parallel()
	e := newChanEnv(t)
	ctx := context.Background()
	e.enable(t, e.owner(), model.ChannelEmail, model.EventErrors)
	if err := e.svc.UpdateInstallFailed(ctx, model.UpdateInstall{Phase: "rolled_back", From: "1.15.0", To: "1.16.0",
		Code: "unhealthy", Message: "the new server did not become healthy in 10m0s"}); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.UpdateInstallFailed(ctx, model.UpdateInstall{Phase: "failed", From: "1.15.0", To: "1.16.1", Migrated: true,
		Backup: "pre-migration-1.16.1-20261107T100200.000Z.db", Code: "migrated_no_rollback"}); err != nil {
		t.Fatal(err)
	}
	e.drain(t)
	mails := e.mailer.sent()
	if len(mails) != 2 {
		t.Fatalf("mails = %+v, want two", mails)
	}
	bySubject := map[string]string{}
	for _, m := range mails {
		bySubject[m.subject] = m.text
	}
	rolled, ok := bySubject["Update auf 1.16.0 zurückgenommen"]
	if !ok || !strings.Contains(rolled, "von 1.15.0 auf 1.16.0") || !strings.Contains(rolled, "Grund: unhealthy") ||
		!strings.Contains(rolled, "did not become healthy") || strings.Contains(rolled, "migriert") {
		t.Errorf("rolled back mail = %q (subjects %v)", rolled, mails)
	}
	failed, ok := bySubject["Update auf 1.16.1 fehlgeschlagen"]
	if !ok || !strings.Contains(failed, "Grund: migrated_no_rollback") ||
		!strings.Contains(failed, "Sicherung pre-migration-1.16.1-20261107T100200.000Z.db") {
		t.Errorf("failed mail = %q", failed)
	}
	for _, d := range e.deliveries(t) {
		if d.event != model.EventErrors {
			t.Errorf("delivery %+v", d)
		}
	}
}

// Every text of the update event exists in both languages.
func TestUpdateInstallTexts(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"update_rolled_back_subject", "update_rolled_back_body", "update_failed_subject", "update_failed_body",
		"update_code", "update_migrated", "update_migrated_backup", "update_version_unknown"} {
		for _, lang := range []string{"de", "en"} {
			if _, ok := texts[lang][key]; !ok {
				t.Errorf("%s: no text %q", lang, key)
			}
		}
	}
	if got := text("en", "update_failed_body", orUnknown("en", ""), "1.16.0"); got != "The update from (unknown) to 1.16.0 failed." {
		t.Errorf("en body = %q", got)
	}
}

// A second failed install to the same version within the hour is a new install and reaches the
// administration as well; the source carries the finish time (only the same install is kept out).
func TestUpdateInstallFailedTwiceToTheSameVersion(t *testing.T) {
	t.Parallel()
	e := newChanEnv(t)
	ctx := context.Background()
	e.enable(t, e.owner(), model.ChannelEmail, model.EventErrors)
	first := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	second := first.Add(10 * time.Minute)
	for _, at := range []time.Time{first, second} {
		if err := e.svc.UpdateInstallFailed(ctx, model.UpdateInstall{Phase: "rolled_back", From: "1.15.0", To: "1.16.0",
			Code: "unhealthy", FinishedAt: &at}); err != nil {
			t.Fatal(err)
		}
	}
	e.drain(t)
	if mails := e.mailer.sent(); len(mails) != 2 {
		t.Fatalf("mails = %+v, want one per install", mails)
	}
}
