package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Concept 22 stage 3 (contract 1.36.0) over HTTP: getSession carries the rating system of the
// server's rating country (DE with the FSK labels, FR with «-12» after the setting changed);
// getDashboard shows the one-time minimum-age hint until dismissMinimumAgeNotice closes it (204,
// again 204) and the hint that the ratings are read anew while the job is queued.
func TestRatingSystemAndHintsOverHTTP(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f, h, ctx := pf.f, pf.h, context.Background()
	session := func() gen.SessionInfo {
		t.Helper()
		rec := send(t, h, http.MethodGet, BasePath+"/auth/session", "p18", "", nil)
		wantStatus(t, rec, http.StatusOK)
		var info gen.SessionInfo
		decode(t, rec, &info)
		return info
	}
	labelOf := func(rs *gen.RatingSystem, age int) string {
		for _, st := range rs.Steps {
			if st.Age == age && st.Label != nil {
				return *st.Label
			}
		}
		return ""
	}
	rs := session().RatingSystem
	if rs == nil || rs.Country != "DE" || len(rs.Steps) != 5 || labelOf(rs, 12) != "FSK 12" || rs.Name == nil || *rs.Name != "FSK" {
		t.Fatalf("rating system of a German server = %+v", rs)
	}
	if _, err := f.DB.Write.ExecContext(ctx, `INSERT INTO settings (key, value, updated_at) VALUES (?, 'FR', 1)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value`, model.SettingRatingCountry); err != nil {
		t.Fatal(err)
	}
	if rs := session().RatingSystem; rs == nil || rs.Country != "FR" || labelOf(rs, 12) != "-12" || labelOf(rs, 10) != "-10" {
		t.Errorf("rating system after the change to FR = %+v", rs)
	}

	dashboard := func() (gen.Dashboard, string) {
		t.Helper()
		rec := send(t, h, http.MethodGet, BasePath+"/admin/dashboard", "p18", "", nil)
		wantStatus(t, rec, http.StatusOK)
		var d gen.Dashboard
		decode(t, rec, &d)
		return d, rec.Body.String()
	}
	if d, body := dashboard(); d.Attention.MinimumAgeNotice != nil || d.Attention.RatingsRereading != nil ||
		strings.Contains(body, "minimumAgeNotice") {
		t.Errorf("a fresh server shows a rating hint: %+v", d.Attention)
	}
	if _, err := f.DB.Write.ExecContext(ctx, `INSERT INTO settings (key, value, updated_at) VALUES (?, '1', 1)`, model.SettingMinimumAgeNotice); err != nil {
		t.Fatal(err)
	}
	if _, err := pf.queue.Enqueue(ctx, jobs.NewJob{Type: model.JobRatingsReread}); err != nil {
		t.Fatal(err)
	}
	if d, _ := dashboard(); d.Attention.MinimumAgeNotice == nil || !*d.Attention.MinimumAgeNotice ||
		d.Attention.RatingsRereading == nil || !*d.Attention.RatingsRereading {
		t.Errorf("after the update: attention %+v, want both hints", d.Attention)
	}
	for range 2 {
		wantStatus(t, send(t, h, http.MethodDelete, BasePath+"/admin/dashboard/minimum-age-notice", "p18", "", nil), http.StatusNoContent)
	}
	if d, _ := dashboard(); d.Attention.MinimumAgeNotice != nil {
		t.Errorf("the closed hint is still shown: %+v", d.Attention)
	}
}
