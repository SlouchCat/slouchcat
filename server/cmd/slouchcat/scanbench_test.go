package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

func TestScanbenchSmallTree(t *testing.T) {
	root := t.TempDir()
	files := []string{
		"Movies/Alien (1979)/Alien (1979).mkv",
		"Movies/Alien (1979)/Trailers/Alien Trailer.mkv",
		"Movies/Heat (1995)/Heat (1995) - 1080p.mkv",
		"TV/Show (2020)/Season 01/Show - S01E01.mkv",
		"TV/Show (2020)/Season 01/Show - S01E02.mkv",
		"TV/Show (2020)/Specials/Show - S00E01.mkv",
	}
	for _, f := range files {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o600); err != nil { // empty placeholders, like task testdata:tree
			t.Fatal(err)
		}
	}
	o := scanbenchOptions{
		root: root, workers: 2, interrupt: time.Millisecond, timeout: time.Minute,
		runner: transcode.Runner{FFprobePath: "ffprobe"}, log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		libs: []benchLibrary{
			{name: "Filme", kind: model.LibraryVideo, content: model.RootMovie, dir: filepath.Join(root, "Movies")},
			{name: "Serien", kind: model.LibraryVideo, content: model.RootSeries, dir: filepath.Join(root, "TV")},
		},
	}
	rep, err := runScanbench(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Libraries) != 2 || rep.Libraries[0].Items != 3 || rep.Libraries[1].Items != 1+2+3 {
		t.Fatalf("libraries = %+v", rep.Libraries)
	}
	if rep.Libraries[0].ProbeErrors != 3 || rep.Libraries[1].ProbeErrors != 3 {
		t.Errorf("probe errors = %d, %d; want every empty file", rep.Libraries[0].ProbeErrors, rep.Libraries[1].ProbeErrors)
	}
	if rep.Jobs.Repeated != 0 || rep.Jobs.LeaseExpired != 0 || rep.Jobs.DuplicateKeys != 0 || rep.Jobs.Done != rep.Jobs.Total {
		t.Errorf("jobs = %+v", rep.Jobs)
	}
	if rep.Home.Requests == 0 || rep.Home.Errors != 0 {
		t.Errorf("home = %+v", rep.Home)
	}
	if rep.RescanOps != 0 || rep.RescanBatches != 0 {
		t.Errorf("rescan wrote %d operations in %d transactions", rep.RescanOps, rep.RescanBatches)
	}
	if !rep.Interrupt.Converged || rep.Interrupt.FinishedBefore {
		t.Errorf("interrupt = %+v", rep.Interrupt)
	}
	for _, l := range rep.Interrupt.Libraries {
		if l.State != "aborted" || l.LastError != "cancelled" {
			t.Errorf("interrupted %s: %s/%s", l.Name, l.State, l.LastError)
		}
	}
	var out bytes.Buffer
	printScanbench(&out, rep)
	if !strings.Contains(out.String(), "rescan without changes") {
		t.Errorf("table:\n%s", out.String())
	}
}
