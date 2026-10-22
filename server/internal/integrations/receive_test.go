package integrations

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// receiveEnv is an environment with one library root and one integration of each kind.
type receiveEnv struct {
	*env
	radarr, sonarr model.Integration
	libraryID      int64
	movieRoot      int64
	seriesRoot     int64
}

func newReceiveEnv(t *testing.T) *receiveEnv {
	t.Helper()
	e := newEnv(t)
	ctx := context.Background()
	movieLib, movieRoot := e.addLibrary(t, "Filme", "/media/Filme")
	_, seriesRoot := e.addLibrary(t, "Serien", "/media/Serien")
	radarr, err := e.svc.Create(ctx, "Radarr", model.IntegrationRadarr)
	if err != nil {
		t.Fatal(err)
	}
	sonarr, err := e.svc.Create(ctx, "Sonarr", model.IntegrationSonarr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.SetPathMap(ctx, radarr.ID, []model.PathMapping{{From: "/movies", To: "/media/Filme"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.SetPathMap(ctx, sonarr.ID, []model.PathMapping{{From: "/tv", To: "/media/Serien"}}); err != nil {
		t.Fatal(err)
	}
	return &receiveEnv{env: e, radarr: radarr, sonarr: sonarr, libraryID: movieLib, movieRoot: movieRoot, seriesRoot: seriesRoot}
}

func TestReceiveQueuesOneScanForEveryActingEvent(t *testing.T) {
	e := newReceiveEnv(t)
	ctx := context.Background()
	for _, event := range []string{"Download", "Rename", "MovieFileDelete", "MovieDelete"} {
		t.Run(event, func(t *testing.T) {
			before := len(e.scan.calls)
			res, err := e.svc.Receive(ctx, e.radarr.Token, model.WebhookPayload{
				Event: event, MoviePath: "/movies/Dune (2021)",
			})
			if err != nil {
				t.Fatal(err)
			}
			if res.Result != model.ResultScanQueued {
				t.Fatalf("result %q, want %q", res.Result, model.ResultScanQueued)
			}
			if res.Path != "/media/Filme/Dune (2021)" {
				t.Errorf("path %q", res.Path)
			}
			if len(e.scan.calls) != before+1 {
				t.Fatalf("%d scans queued, want one", len(e.scan.calls)-before)
			}
			call := e.scan.calls[len(e.scan.calls)-1]
			if call.libraryID != e.libraryID || call.rootID != e.movieRoot {
				t.Errorf("scan of library %d root %d, want %d and %d", call.libraryID, call.rootID, e.libraryID, e.movieRoot)
			}
			if len(call.dirs) != 1 || call.dirs[0] != "Dune (2021)" {
				t.Errorf("directories %v", call.dirs)
			}
		})
	}
}

func TestReceiveReadsTheSonarrPathAndTheRadarrPathPerKind(t *testing.T) {
	e := newReceiveEnv(t)
	ctx := context.Background()

	// Every acting event of a series, including the two deletions Sonarr sends (5.6).
	for _, event := range []string{"Download", "Rename", "EpisodeFileDelete", "SeriesDelete"} {
		before := len(e.scan.calls)
		res, err := e.svc.Receive(ctx, e.sonarr.Token, model.WebhookPayload{
			Event: event, SeriesPath: "/tv/Slouchcat Test Show",
		})
		if err != nil {
			t.Fatalf("%s: %v", event, err)
		}
		if res.Result != model.ResultScanQueued {
			t.Errorf("%s: result %q, want %q", event, res.Result, model.ResultScanQueued)
		}
		if res.Path != "/media/Serien/Slouchcat Test Show" {
			t.Errorf("%s: path %q", event, res.Path)
		}
		if len(e.scan.calls) != before+1 {
			t.Fatalf("%s: %d scans queued, want one", event, len(e.scan.calls)-before)
		}
		if call := e.scan.calls[len(e.scan.calls)-1]; call.rootID != e.seriesRoot {
			t.Errorf("%s: root %d, want the series root %d", event, call.rootID, e.seriesRoot)
		}
	}

	// A Sonarr body on a Radarr address carries no movie.folderPath: 422, logged, no scan.
	before := len(e.scan.calls)
	if _, err := e.svc.Receive(ctx, e.radarr.Token, model.WebhookPayload{
		Event: "Download", SeriesPath: "/tv/Slouchcat Test Show",
	}); !isValidation(t, err, "movie.folderPath") {
		t.Errorf("error %v, want a validation error on movie.folderPath", err)
	}
	if len(e.scan.calls) != before {
		t.Error("a scan was queued anyway")
	}
	if last := e.lastEvent(t, e.radarr.ID); last.Result != model.ResultInvalidPayload {
		t.Errorf("logged result %q, want %q", last.Result, model.ResultInvalidPayload)
	}
}

func TestReceiveAcknowledgesTestAndIgnoresEverythingElse(t *testing.T) {
	e := newReceiveEnv(t)
	ctx := context.Background()

	res, err := e.svc.Receive(ctx, e.radarr.Token, model.WebhookPayload{Event: "Test"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Result != model.ResultAcknowledged {
		t.Errorf("result %q, want %q", res.Result, model.ResultAcknowledged)
	}
	if last := e.lastEvent(t, e.radarr.ID); last.Event != "Test" || last.Result != model.ResultAcknowledged {
		t.Errorf("log entry %+v", last)
	}

	for _, event := range []string{"Grab", "Health", "ApplicationUpdate", ""} {
		before := len(e.scan.calls)
		res, err := e.svc.Receive(ctx, e.radarr.Token, model.WebhookPayload{
			Event: event, MoviePath: "/movies/Dune (2021)",
		})
		if err != nil {
			t.Fatalf("%q: %v", event, err)
		}
		if res.Result != model.ResultIgnored {
			t.Errorf("%q: result %q, want %q", event, res.Result, model.ResultIgnored)
		}
		if len(e.scan.calls) != before {
			t.Errorf("%q queued a scan", event)
		}
	}
}

func TestReceiveRejectsAnUnknownTokenWithoutTouchingAnything(t *testing.T) {
	e := newReceiveEnv(t)
	ctx := context.Background()
	unknown := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	if _, err := e.svc.Receive(ctx, unknown, model.WebhookPayload{Event: "Download", MoviePath: "/movies/X"}); !errors.Is(err, model.ErrWebhookToken) {
		t.Fatalf("error %v, want ErrWebhookToken", err)
	}
	if len(e.scan.calls) != 0 {
		t.Error("a scan was queued for an unknown token")
	}
	for _, id := range []int64{e.radarr.ID, e.sonarr.ID} {
		events, err := e.svc.Events(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 0 {
			t.Errorf("integration %d logged %d entries for an unknown token", id, len(events))
		}
	}
}

func TestReceiveRejectsAPathOutsideEveryRootAndLogsBothPaths(t *testing.T) {
	e := newReceiveEnv(t)
	ctx := context.Background()
	// The mapping applies but lands nowhere: the two columns must hold two different paths, so
	// the administration sees what was reported and what Slouchcat made of it (5.6).
	if _, err := e.svc.SetPathMap(ctx, e.radarr.ID, []model.PathMapping{
		{From: "/movies", To: "/media/Filme"},
		{From: "/elsewhere", To: "/nicht/eingehaengt"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Receive(ctx, e.radarr.Token, model.WebhookPayload{
		Event: "Download", MoviePath: "/elsewhere/Dune (2021)",
	}); !errors.Is(err, model.ErrWebhookPathOutside) {
		t.Fatalf("error %v, want ErrWebhookPathOutside", err)
	}
	if len(e.scan.calls) != 0 {
		t.Error("a scan was queued for a path outside every root")
	}
	last := e.lastEvent(t, e.radarr.ID)
	if last.Result != model.ResultOutsideRoots {
		t.Errorf("result %q, want %q", last.Result, model.ResultOutsideRoots)
	}
	if last.ReportedPath != "/elsewhere/Dune (2021)" {
		t.Errorf("reported path %q", last.ReportedPath)
	}
	if last.ResolvedPath != "/nicht/eingehaengt/Dune (2021)" {
		t.Errorf("resolved path %q, want the mapped one", last.ResolvedPath)
	}
	if last.ReportedPath == last.ResolvedPath {
		t.Error("both columns hold the same path, so the mapping is not visible in the log")
	}

	// Without a mapping both columns hold the reported path, which is correct too.
	if _, err := e.svc.Receive(ctx, e.sonarr.Token, model.WebhookPayload{
		Event: "Download", SeriesPath: "/woanders/Show",
	}); !errors.Is(err, model.ErrWebhookPathOutside) {
		t.Fatalf("sonarr: error %v, want ErrWebhookPathOutside", err)
	}
	if last := e.lastEvent(t, e.sonarr.ID); last.ReportedPath != "/woanders/Show" || last.ResolvedPath != "/woanders/Show" {
		t.Errorf("without a mapping: reported %q, resolved %q", last.ReportedPath, last.ResolvedPath)
	}
}

func TestReceiveRefusesTheEleventhRequestInASecond(t *testing.T) {
	e := newReceiveEnv(t)
	ctx := context.Background()
	for i := range RequestsPerSecond {
		if _, err := e.svc.Receive(ctx, e.radarr.Token, model.WebhookPayload{Event: "Test"}); err != nil {
			t.Fatalf("request %d: %v", i+1, err)
		}
	}
	if _, err := e.svc.Receive(ctx, e.radarr.Token, model.WebhookPayload{Event: "Test"}); !errors.Is(err, model.ErrWebhookRateLimited) {
		t.Fatalf("request %d: error %v, want ErrWebhookRateLimited", RequestsPerSecond+1, err)
	}
	// The limit is per integration: the other one is unaffected.
	if _, err := e.svc.Receive(ctx, e.sonarr.Token, model.WebhookPayload{Event: "Test"}); err != nil {
		t.Errorf("the other integration was limited too: %v", err)
	}
}

// The event log must not claim a scan that never reached the queue (review finding): a failing
// RequestPathScan answers with an error and leaves no entry, so Radarr retries.
func TestReceiveWithoutAQueuedScanRecordsNothing(t *testing.T) {
	e := newReceiveEnv(t)
	ctx := context.Background()
	e.scan.err = errors.New("queue is gone")

	if _, err := e.svc.Receive(ctx, e.radarr.Token, model.WebhookPayload{
		Event: "Download", MoviePath: "/movies/Dune (2021)",
	}); err == nil {
		t.Fatal("a failing scan request was answered as a success")
	}
	events, err := e.svc.Events(ctx, e.radarr.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Errorf("%d log entries for a scan that was never queued: %+v", len(events), events)
	}

	// Once the queue works again, the same webhook is recorded as it should be.
	e.scan.err = nil
	if _, err := e.svc.Receive(ctx, e.radarr.Token, model.WebhookPayload{
		Event: "Download", MoviePath: "/movies/Dune (2021)",
	}); err != nil {
		t.Fatal(err)
	}
	if last := e.lastEvent(t, e.radarr.ID); last.Result != model.ResultScanQueued {
		t.Errorf("result %q, want %q", last.Result, model.ResultScanQueued)
	}
}

// A service that sends an over-long event name keeps its log entry, shortened (the column holds
// 80 characters); losing the entry would blind the one surface that answers "did it arrive".
func TestReceiveKeepsTheLogEntryOfAnOverlongEventName(t *testing.T) {
	e := newReceiveEnv(t)
	ctx := context.Background()
	long := strings.Repeat("Ü", 200)
	if _, err := e.svc.Receive(ctx, e.radarr.Token, model.WebhookPayload{Event: long}); err != nil {
		t.Fatal(err)
	}
	last := e.lastEvent(t, e.radarr.ID)
	if got := len([]rune(last.Event)); got != 80 {
		t.Errorf("stored event name has %d characters, want 80", got)
	}
	if last.Result != model.ResultIgnored {
		t.Errorf("result %q, want %q", last.Result, model.ResultIgnored)
	}
}

func TestReceiveRecordsTheLastContact(t *testing.T) {
	e := newReceiveEnv(t)
	ctx := context.Background()
	if before, err := e.svc.Get(ctx, e.radarr.ID); err != nil {
		t.Fatal(err)
	} else if !before.LastSeen.IsZero() {
		t.Errorf("last contact %s before the first webhook", before.LastSeen)
	}
	e.now = e.now.Add(90 * time.Second)
	if _, err := e.svc.Receive(ctx, e.radarr.Token, model.WebhookPayload{Event: "Test"}); err != nil {
		t.Fatal(err)
	}
	after, err := e.svc.Get(ctx, e.radarr.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !after.LastSeen.Equal(e.now) {
		t.Errorf("last contact %s, want %s", after.LastSeen, e.now)
	}
}

// lastEvent returns the newest log entry of an integration.
func (e *receiveEnv) lastEvent(t *testing.T, id int64) model.IntegrationEvent {
	t.Helper()
	events, err := e.svc.Events(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatal("no log entry")
	}
	return events[0]
}
