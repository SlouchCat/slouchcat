package admin

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

type fakeTrakt struct {
	status    model.TraktStatus
	refreshes int
	err       error
}

func (f *fakeTrakt) TraktStatus(context.Context) (model.TraktStatus, error) { return f.status, f.err }

func (f *fakeTrakt) RefreshTrakt(context.Context) error {
	f.refreshes++
	return f.err
}

// The stored Trakt client id is read decrypted and never logged; status and refresh go to the
// metadata service (E-7).
func TestTrakt(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	if id, err := f.svc.TraktClientID(ctx); err != nil || id != "" {
		t.Fatalf("TraktClientID unset = %q, %v", id, err)
	}
	enc, err := f.ring.Encrypt("FAKE-trakt-id")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Write.ExecContext(ctx, `INSERT INTO settings (key, value, updated_at) VALUES (?, ?, 0)`, SettingTraktClientID, enc); err != nil {
		t.Fatal(err)
	}
	if id, err := f.svc.TraktClientID(ctx); err != nil || id != "FAKE-trakt-id" {
		t.Fatalf("TraktClientID from settings = %q, %v", id, err)
	}

	if _, err := f.svc.TraktStatus(ctx); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("status without WithTrakt: %v", err)
	}
	if err := f.svc.RefreshTrakt(ctx); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("refresh without WithTrakt: %v", err)
	}
	tr := &fakeTrakt{status: model.TraktStatus{Configured: true, ListKey: "trending", Entries: 10}}
	f.svc.WithTrakt(tr)
	if st, err := f.svc.TraktStatus(ctx); err != nil || st != tr.status {
		t.Errorf("status = %+v, %v", st, err)
	}
	if err := f.svc.RefreshTrakt(ctx); err != nil || tr.refreshes != 1 {
		t.Errorf("refresh: %v, %d calls", err, tr.refreshes)
	}
	tr.err = model.ErrProviderNotConfigured
	if err := f.svc.RefreshTrakt(ctx); !errors.Is(err, model.ErrProviderNotConfigured) {
		t.Errorf("refresh without client id: %v", err)
	}
	if strings.Contains(f.logs.String(), "FAKE-trakt-id") {
		t.Errorf("log names the client id:\n%s", f.logs.String())
	}
}
