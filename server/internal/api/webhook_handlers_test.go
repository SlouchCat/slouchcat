package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/integrations"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// M9 (I-8, decision 5.6): the receiver is exercised against the payloads that were recorded from
// running Radarr and Sonarr instances, through the real route, so the generated decoder of the
// contract has to cope with every field they send. testdata/webhooks/README.md says how a
// recording is made; the file name carries the service, the pinned version and the event.

// recordingDir is the directory of the recorded payloads, relative to this package.
const recordingDir = "../../../testdata/webhooks"

// webhookRoot is the library root the recordings are mapped onto.
const webhookRoot = "/media/Recorded"

// actingEvents are the events of 5.6 that must queue a scan; every other event is ignored.
var recordedActing = map[string]bool{
	"download": true, "rename": true, "moviefiledelete": true,
	"episodefiledelete": true, "moviedelete": true, "seriesdelete": true,
}

// recordingScanner records the path scans the receiver asks for.
type recordingScanner struct {
	mu    sync.Mutex
	calls [][]string // directories per call
}

func (r *recordingScanner) RequestPathScan(_ context.Context, _, _ int64, dirs []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, dirs)
	return nil
}

func (r *recordingScanner) taken() [][]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.calls
	r.calls = nil
	return out
}

// recording is one file of testdata/webhooks.
type recording struct {
	file    string
	service string // radarr, sonarr
	version string
	event   string // from the file name, lower case
	body    []byte
	payload map[string]any
}

// loadRecordings reads every recorded payload. It fails when a service has no recording at all:
// the acceptance row I-8 asks for tests against real Radarr and Sonarr payloads, and a test that
// silently covers nothing would be worse than none.
func loadRecordings(t *testing.T) []recording {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(recordingDir, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	out := make([]recording, 0, len(files))
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".json")
		parts := strings.SplitN(name, "-", 3)
		if len(parts) != 3 {
			t.Errorf("%s: the name must be <service>-<version>-<event>.json (see the README)", filepath.Base(f))
			continue
		}
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("%s: %v", filepath.Base(f), err)
			continue
		}
		out = append(out, recording{
			file: filepath.Base(f), service: parts[0], version: parts[1], event: parts[2],
			body: body, payload: payload,
		})
	}
	byService := map[string]int{}
	for _, r := range out {
		byService[r.service]++
	}
	for _, service := range []string{"radarr", "sonarr"} {
		if byService[service] == 0 {
			t.Fatalf("no recorded %s payload in %s; record one as testdata/webhooks/README.md describes", service, recordingDir)
		}
	}
	return out
}

// reportedPath returns the directory a recording names, the way the receiver reads it.
func (r recording) reportedPath(t *testing.T) string {
	t.Helper()
	field := "movie"
	key := "folderPath"
	if r.service == "sonarr" {
		field, key = "series", "path"
	}
	obj, ok := r.payload[field].(map[string]any)
	if !ok {
		return ""
	}
	p, _ := obj[key].(string)
	return p
}

// webhookFixture is the contract fixture with a receiver whose scans are observable.
type webhookFixture struct {
	h       http.Handler
	svc     *integrations.Service
	scanner *recordingScanner
}

func newWebhookFixture(t *testing.T) webhookFixture {
	return newWebhookFixtureAt(t, &stepClock{now: time.Now()})
}

// newWebhookFixtureAt is newWebhookFixture with a clock the caller owns: the rate limit is ten
// requests per second, and only a still clock makes "inside the same second" a property of the
// test instead of a property of how busy the machine is.
func newWebhookFixtureAt(t *testing.T, clock *stepClock) webhookFixture {
	t.Helper()
	pf := newProfileFixture(t)
	scanner := &recordingScanner{}
	svc := integrations.New(pf.f.DB, scanner, slog.New(slog.NewTextHandler(io.Discard, nil)),
		integrations.WithBaseURL(func(context.Context) string { return "https://slouchcat.example.ch" + BasePath }),
		integrations.WithClock(clock.Now))
	o := pf.opts
	o.Integrations = svc
	// The recordings are mapped onto one root, so every reported directory resolves.
	if _, err := pf.f.DB.Write.ExecContext(context.Background(),
		`INSERT INTO libraries (name, kind, default_rating, created_at, updated_at) VALUES ('Recorded', 'video', 0, 0, 0)`); err != nil {
		t.Fatal(err)
	}
	var libraryID int64
	if err := pf.f.DB.Read.QueryRowContext(context.Background(),
		`SELECT id FROM libraries WHERE name = 'Recorded'`).Scan(&libraryID); err != nil {
		t.Fatal(err)
	}
	if _, err := pf.f.DB.Write.ExecContext(context.Background(),
		`INSERT INTO library_roots (library_id, path, fs_type, watch, created_at) VALUES (?, ?, 'ext4', 'inotify', 0)`,
		libraryID, webhookRoot); err != nil {
		t.Fatal(err)
	}
	return webhookFixture{h: NewHandler(o), svc: svc, scanner: scanner}
}

// integration creates an integration of a kind whose mapping sends dir onto webhookRoot.
func (wf webhookFixture) integration(t *testing.T, name, kind, dir string) model.Integration {
	t.Helper()
	ctx := context.Background()
	created, err := wf.svc.Create(ctx, name, kind)
	if err != nil {
		t.Fatal(err)
	}
	if dir != "" {
		if _, err := wf.svc.SetPathMap(ctx, created.ID, []model.PathMapping{{From: dir, To: webhookRoot}}); err != nil {
			t.Fatal(err)
		}
	}
	return created
}

// postWebhook sends a raw body to the receiver, the way Radarr does: no CSRF header, no session.
func postWebhook(t *testing.T, h http.Handler, token string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, BasePath+"/webhooks/"+token, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestWebhookHandlesTheRecordedRadarrAndSonarrPayloads(t *testing.T) {
	t.Parallel()
	recordings := loadRecordings(t)
	acting := map[string]int{}
	for _, r := range recordings {
		t.Run(r.file, func(t *testing.T) {
			wf := newWebhookFixture(t)
			reported := r.reportedPath(t)
			// Only an acting event names a real directory. The Test payload of both services
			// carries the dummy "C:\testpath", which must never become a path mapping.
			dir := ""
			if reported != "" && recordedActing[r.event] {
				dir = path.Dir(reported)
			}
			kind := model.IntegrationRadarr
			if r.service == "sonarr" {
				kind = model.IntegrationSonarr
			}
			integ := wf.integration(t, "Recorded", kind, dir)

			rec := postWebhook(t, wf.h, integ.Token, r.body)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d (%s), want 200", rec.Code, rec.Body.String())
			}
			var got struct {
				Result string `json:"result"`
				Path   string `json:"path"`
			}
			decode(t, rec, &got)

			// The event in the file name is the eventType of the recording.
			if ev, _ := r.payload["eventType"].(string); !strings.EqualFold(strings.ReplaceAll(ev, " ", ""), r.event) {
				t.Errorf("file name says event %q, the payload says %q", r.event, ev)
			}

			scans := wf.scanner.taken()
			switch {
			case recordedActing[r.event]:
				if got.Result != model.ResultScanQueued {
					t.Fatalf("result %q, want %q", got.Result, model.ResultScanQueued)
				}
				if reported == "" {
					t.Fatalf("an acting %s event without a directory in the payload", r.service)
				}
				want := webhookRoot + "/" + path.Base(reported)
				if got.Path != want {
					t.Errorf("scanned path %q, want %q", got.Path, want)
				}
				if len(scans) != 1 || len(scans[0]) != 1 || scans[0][0] != path.Base(reported) {
					t.Errorf("queued scans %v, want one of %q", scans, path.Base(reported))
				}
			case r.event == "test":
				if got.Result != model.ResultAcknowledged {
					t.Errorf("result %q, want %q", got.Result, model.ResultAcknowledged)
				}
				if len(scans) != 0 {
					t.Errorf("Test queued %v", scans)
				}
			default:
				if got.Result != model.ResultIgnored {
					t.Errorf("result %q, want %q", got.Result, model.ResultIgnored)
				}
				if len(scans) != 0 {
					t.Errorf("%s queued %v", r.event, scans)
				}
			}

			// Every call lands in the event log of its integration.
			events, err := wf.svc.Events(context.Background(), integ.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(events) != 1 {
				t.Fatalf("%d log entries, want one", len(events))
			}
		})
		if recordedActing[r.event] {
			acting[r.service]++
		}
	}
	for _, service := range []string{"radarr", "sonarr"} {
		if acting[service] == 0 {
			t.Errorf("no recorded %s payload of an acting event (import, rename or delete)", service)
		}
	}
}

func TestWebhookAnswersWithoutTheCSRFHeaderAndNeverLogsTheToken(t *testing.T) {
	t.Parallel()
	wf := newWebhookFixture(t)
	integ := wf.integration(t, "Radarr", model.IntegrationRadarr, "/movies")

	// Radarr sends no CSRF header; the route is exempt (the secret in the path authorises it).
	rec := postWebhook(t, wf.h, integ.Token, []byte(`{"eventType":"Test"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d (%s), want 200", rec.Code, rec.Body.String())
	}

	// A wrong token is 401, and the problem document carries no token (A-9).
	wrong := strings.Repeat("Z", 43)
	rec = postWebhook(t, wf.h, wrong, []byte(`{"eventType":"Test"}`))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d (%s), want 401", rec.Code, rec.Body.String())
	}
	for _, token := range []string{integ.Token, wrong} {
		if strings.Contains(rec.Body.String(), token) {
			t.Errorf("the 401 answer contains a token")
		}
	}
}

// The limit is ten requests per second and per integration, so the eleventh has to arrive inside
// the same second. The clock stands still for the whole test: with the real one the eleven
// requests took longer than a second on the CI runner, the bucket refilled in the middle and the
// eleventh came back 200 (run 39956652341 of 23 October 2026).
func TestWebhookRefusesTheEleventhRequestInASecond(t *testing.T) {
	t.Parallel()
	wf := newWebhookFixtureAt(t, &stepClock{now: time.Now()})
	integ := wf.integration(t, "Radarr", model.IntegrationRadarr, "/movies")
	for i := range integrations.RequestsPerSecond {
		if rec := postWebhook(t, wf.h, integ.Token, []byte(`{"eventType":"Test"}`)); rec.Code != http.StatusOK {
			t.Fatalf("request %d: status %d (%s)", i+1, rec.Code, rec.Body.String())
		}
	}
	rec := postWebhook(t, wf.h, integ.Token, []byte(`{"eventType":"Test"}`))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("request %d: status %d (%s), want 429", integrations.RequestsPerSecond+1, rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Retry-After"); got != "1" {
		t.Errorf("Retry-After %q, want \"1\"", got)
	}
}

func TestWebhookAnswers422ForAPathOutsideEveryRoot(t *testing.T) {
	t.Parallel()
	wf := newWebhookFixture(t)
	integ := wf.integration(t, "Radarr", model.IntegrationRadarr, "")
	rec := postWebhook(t, wf.h, integ.Token,
		[]byte(`{"eventType":"Download","movie":{"folderPath":"/elsewhere/Dune (2021)"}}`))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d (%s), want 422", rec.Code, rec.Body.String())
	}
	var problem struct {
		Code string `json:"code"`
	}
	decode(t, rec, &problem)
	if problem.Code != "path_outside_roots" {
		t.Errorf("code %q, want path_outside_roots", problem.Code)
	}
	events, err := wf.svc.Events(context.Background(), integ.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("%d log entries, want one", len(events))
	}
	if events[0].ReportedPath != "/elsewhere/Dune (2021)" || events[0].ResolvedPath != "/elsewhere/Dune (2021)" {
		t.Errorf("the log entry does not carry both paths: %+v", events[0])
	}
}

// fmtVersion keeps the pinned versions visible in a failure message.
func (r recording) String() string { return fmt.Sprintf("%s %s %s", r.service, r.version, r.event) }
