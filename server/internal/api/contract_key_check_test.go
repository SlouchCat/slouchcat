package api

import (
	"net/http"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
)

// Contract 1.31.0 (concept 20, P-12): checkProviderKey answers ok, or the problem with the
// provider's message; an unknown provider is outside the contract's enum.
func TestCheckProviderKeyOverHTTP(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	check := func(provider string) gen.ProviderKeyCheck {
		t.Helper()
		rec := send(t, pf.h, http.MethodPost, BasePath+"/admin/settings/keys/"+provider+"/check", "p18", "", nil)
		wantStatus(t, rec, http.StatusOK)
		var out gen.ProviderKeyCheck
		decode(t, rec, &out)
		return out
	}
	if got := check("tmdb"); !got.Ok || got.Problem != nil || got.Message != nil {
		t.Errorf("tmdb = %+v, want ok alone", got)
	}
	if got := check("youtube"); got.Ok || got.Problem == nil || *got.Problem != gen.ProviderKeyCheckProblemNotConfigured {
		t.Errorf("youtube = %+v, want not_configured", got)
	}
	if got := check("trakt"); got.Ok || got.Problem == nil || *got.Problem != gen.ProviderKeyCheckProblemInvalid ||
		got.Message == nil || *got.Message != "trakt: status 403" {
		t.Errorf("trakt = %+v, want invalid with the message", got)
	}
	rec := send(t, pf.h, http.MethodPost, BasePath+"/admin/settings/keys/nobody/check", "p18", "", nil)
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("unknown provider: status %d, want 400 or 422", rec.Code)
	}
}
