package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Contract 1.14.0 (concept 15, 4.3): the time window of the heavy event types. The fixture's
// registry has no windowable type (those live in package library), so this test hands the admin
// service a registry with one windowable event type next to download.prepare.
func TestJobTypeWindowOverHTTP(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	ctx := context.Background()
	log := vistest.Logger()
	const heavy = "fake.heavy"
	noop := func(context.Context, *jobs.Job) error { return nil }
	reg := jobs.NewRegistry(pf.queue, log)
	pf.off.RegisterTypes(reg)
	reg.Register(jobs.Type{Name: heavy, Group: jobs.GroupLibrary, Pool: jobs.PoolGeneral, Trigger: jobs.TriggerEvent, Priority: 1,
		Handler: noop, Windowable: true})
	sched := jobs.NewScheduler(pf.queue, reg, log)
	if err := sched.Start(ctx); err != nil {
		t.Fatal(err)
	}
	pf.admin.WithJobControl(reg, sched, map[jobs.Pool]int{jobs.PoolGeneral: 1, jobs.PoolDownload: 1, jobs.PoolNotify: 1, jobs.PoolIndex: 1})

	put := func(typ, body string, status int) *gen.JobTypeInfo {
		t.Helper()
		rec := send(t, pf.h, http.MethodPut, BasePath+"/admin/job-types/"+typ+"/window", "p18", body, nil)
		wantStatus(t, rec, status)
		if status != http.StatusOK {
			var p gen.Problem
			decode(t, rec, &p)
			switch status {
			case http.StatusConflict:
				if p.Code != "job_type_not_windowable" {
					t.Errorf("%s %s: code %s, want job_type_not_windowable", typ, body, p.Code)
				}
			case http.StatusUnprocessableEntity:
				if p.Code != "validation_failed" || p.Errors == nil || len(*p.Errors) != 1 ||
					(*p.Errors)[0] != (gen.FieldError{Field: "window", Code: "invalid_value"}) {
					t.Errorf("%s %s: problem %s, want window invalid_value", typ, body, rec.Body.String())
				}
			}
			return nil
		}
		var info gen.JobTypeInfo
		decode(t, rec, &info)
		return &info
	}
	put("download.prepare", `{"clearWindow":true}`, http.StatusConflict)
	put("nobody.home", `{"clearWindow":true}`, http.StatusNotFound)
	for _, w := range []string{`{"startMinute":60,"endMinute":60}`, `{"startMinute":0,"endMinute":1440}`, `{"startMinute":-1,"endMinute":60}`} {
		put(heavy, `{"window":`+w+`}`, http.StatusUnprocessableEntity)
	}

	// A window that is closed now: start two hours ahead, so it opens at that minute.
	loc := sched.Location()
	now := time.Now().In(loc)
	minute := now.Hour()*60 + now.Minute()
	start, end := (minute+120)%1440, (minute+180)%1440
	info := put(heavy, fmt.Sprintf(`{"window":{"startMinute":%d,"endMinute":%d}}`, start, end), http.StatusOK)
	if info.Windowable == nil || !*info.Windowable || info.Window == nil || *info.Window != (gen.JobWindow{StartMinute: start, EndMinute: end}) ||
		info.WindowTimeZone == nil || *info.WindowTimeZone != loc.String() {
		t.Fatalf("after setting a window = %+v", info)
	}
	// No run waits for the window yet: no windowOpensAt.
	if info.WindowOpensAt != nil {
		t.Errorf("windowOpensAt without a waiting run = %v, want none", info.WindowOpensAt)
	}
	// A waiting run without override_window waits until the window opens.
	if _, err := pf.queue.Enqueue(ctx, jobs.NewJob{Type: heavy, IdempotencyKey: "chapters:0"}); err != nil {
		t.Fatal(err)
	}
	info = put(heavy, fmt.Sprintf(`{"window":{"startMinute":%d,"endMinute":%d}}`, start, end), http.StatusOK)
	if at := info.WindowOpensAt; at == nil || at.Location() != time.UTC || at.Before(time.Now().Add(110*time.Minute)) || at.After(time.Now().Add(121*time.Minute)) {
		t.Errorf("windowOpensAt = %v, want about two hours from now in UTC", at)
	}
	// A window that holds now: no windowOpensAt.
	info = put(heavy, fmt.Sprintf(`{"window":{"startMinute":%d,"endMinute":%d}}`, (minute+1380)%1440, (minute+60)%1440), http.StatusOK)
	if info.Window == nil || info.WindowOpensAt != nil {
		t.Errorf("window around now = %+v, want no windowOpensAt", info)
	}

	// The listing: only the windowable type carries the fields; a run with override_window says so.
	id, err := pf.queue.Enqueue(ctx, jobs.NewJob{Type: heavy, IdempotencyKey: "chapters:1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pf.f.DB.Write.ExecContext(ctx, `UPDATE jobs SET override_window = 1 WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	rec := send(t, pf.h, http.MethodGet, BasePath+"/admin/job-types", "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	var raw struct {
		Types []map[string]json.RawMessage `json:"types"`
	}
	decode(t, rec, &raw)
	for _, typ := range raw.Types {
		var name string
		_ = json.Unmarshal(typ["type"], &name)
		_, windowable := typ["windowable"]
		_, zone := typ["windowTimeZone"]
		if want := name == heavy; windowable != want || zone != want {
			t.Errorf("type %s: windowable %v, windowTimeZone %v, want %v", name, windowable, zone, want)
		}
	}
	rec = send(t, pf.h, http.MethodGet, BasePath+"/admin/jobs?type="+heavy+"&state=queued", "p18", "", nil)
	wantStatus(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"overridesWindow":true`) {
		t.Errorf("listJobRuns = %s, want the run with overridesWindow", rec.Body.String())
	}

	// clearWindow wins over window and removes it; the type stays windowable.
	info = put(heavy, `{"clearWindow":true,"window":{"startMinute":1,"endMinute":2}}`, http.StatusOK)
	if info.Window != nil || info.WindowOpensAt != nil || info.Windowable == nil || !*info.Windowable {
		t.Errorf("after clearWindow = %+v", info)
	}
}
