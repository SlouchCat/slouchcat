package fetch

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/netguard"
)

// A fetch loads the body into Final with the file mode given, reports its progress, keeps the
// answer's media type, and reuses Final without a request; a body cut short leaves the partial
// file for the next attempt, which continues it (concept 19 P-7). The offline download keeps its
// own tests of the resume checks (P-18).
func TestFetchLoadsResumesAndReuses(t *testing.T) {
	t.Parallel()
	content := bytes.Repeat([]byte("0123456789"), 300_000) // 3 MB, three progress steps
	var requests atomic.Int32
	var cut atomic.Bool
	cut.Store(true)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("ETag", `"v1"`)
		if rng := r.Header.Get("Range"); rng != "" {
			from, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(rng, "bytes="), "-"))
			w.Header().Set("Content-Range", "bytes "+strconv.Itoa(from)+"-"+strconv.Itoa(len(content)-1)+"/"+strconv.Itoa(len(content)))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(content[from:])
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(content)))
		if cut.Load() {
			_, _ = w.Write(content[:1_500_000])
			panic(http.ErrAbortHandler) // the connection breaks mid-body
		}
		_, _ = w.Write(content)
	}))
	defer srv.Close()
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	var reports []int64
	req := Request{
		Client: Clients().For(context.Background(), srv.URL+"/feed.xml", "admin"), URL: srv.URL + "/ep.mp3?key=s3cr3t", Root: root,
		Part: "sub/.x.part", Validator: "sub/.x.validator", Final: "sub/.x.done", FileMode: 0o600, DirMode: 0o700,
		Progress: func(have, _ int64) { reports = append(reports, have) },
	}
	if _, err := Fetch(context.Background(), req); !errors.Is(err, ErrUnavailable) || strings.Contains(err.Error(), "s3cr3t") {
		t.Fatalf("cut body: %v", err)
	}
	if info, err := os.Stat(filepath.Join(dir, "sub/.x.part")); err != nil || info.Size() == 0 || info.Mode().Perm() != 0o600 {
		t.Fatalf("partial file after the cut: %v %v", info, err)
	}
	cut.Store(false)
	res, err := Fetch(context.Background(), req)
	if err != nil || res.Reused || res.ContentType != "audio/mpeg" || res.Bytes != int64(len(content)) {
		t.Fatalf("resumed fetch = %+v %v", res, err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "sub/.x.done")); err != nil || !bytes.Equal(got, content) {
		t.Fatalf("final file has %d bytes, %v", len(got), err)
	}
	for _, name := range []string{"sub/.x.part", "sub/.x.validator"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s kept after the fetch: %v", name, err)
		}
	}
	if len(reports) == 0 || reports[len(reports)-1] != int64(len(content)) {
		t.Errorf("progress reports %v, want the last at %d", reports, len(content))
	}
	before := requests.Load()
	if res, err := Fetch(context.Background(), req); err != nil || !res.Reused || requests.Load() != before {
		t.Errorf("reuse = %+v %v after %d requests", res, err, requests.Load()-before)
	}
}

// Another scheme and a public feed whose enclosure lies in the local network are refused, and the
// error names the host only.
func TestFetchRefuses(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("x")) }))
	defer srv.Close()
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	req := Request{Root: root, Part: ".p", Validator: ".v", Final: ".f", FileMode: 0o600}
	req.Client, req.URL = Clients().For(context.Background(), "https://203.0.113.5/feed.xml", "admin"), "ftp://example.test/x.mp3"
	if _, err := Fetch(context.Background(), req); !errors.Is(err, ErrUnavailable) {
		t.Errorf("ftp: %v", err)
	}
	req.URL = srv.URL + "/ep.mp3?auth=s3cr3t"
	if _, err := Fetch(context.Background(), req); !errors.Is(err, ErrUnavailable) || !errors.Is(err, netguard.ErrRefused) || strings.Contains(err.Error(), "s3cr3t") {
		t.Errorf("local enclosure of a public feed: %v", err)
	}
	// A feed read from a file never gets the LAN exception, even on the host of the enclosure.
	req.Client = Clients().For(context.Background(), srv.URL+"/feed.xml", netguard.FeedOriginFile)
	if _, err := Fetch(context.Background(), req); !errors.Is(err, netguard.ErrRefused) {
		t.Errorf("local feed from a file: %v", err)
	}
}

func TestContentRangeAndExt(t *testing.T) {
	t.Parallel()
	if first, total, ok := ContentRange("bytes */3000"); !ok || first != -1 || total != 3000 {
		t.Errorf("ContentRange(*/3000) = %d %d %v", first, total, ok)
	}
	if first, total, ok := ContentRange("bytes 500-2999/*"); !ok || first != 500 || total != -1 {
		t.Errorf("ContentRange(500-2999/*) = %d %d %v", first, total, ok)
	}
	for _, bad := range []string{"", "bytes", "bytes 9-1/10", "items 0-1/2", "bytes a-b/c"} {
		if _, _, ok := ContentRange(bad); ok {
			t.Errorf("ContentRange(%q) accepted", bad)
		}
	}
	for p, want := range map[string]string{"/a/ep.MP3": "mp3", "/ep.m4a": "m4a", "/ep": "audio", "/ep.php": "audio"} {
		if got := Ext(p); got != want {
			t.Errorf("Ext(%q) = %q, want %q", p, got, want)
		}
	}
}

// With NoMkdir a fetch never makes the directory of its partial file: a folder that vanished
// meanwhile stays gone and the fetch fails with fs.ErrNotExist before any request.
func TestFetchNoMkdir(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { requests.Add(1) }))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	_, err = Fetch(context.Background(), Request{Client: netguard.NewClients(http.Client{}, HeaderTimeout).For(context.Background(), srv.URL, ""),
		URL: srv.URL + "/a.mp3", Root: root, Part: "gone/a.part", Validator: "gone/a.validator", Final: "gone/a.mp3",
		FileMode: 0o664, DirMode: 0o775, NoMkdir: true})
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("fetch into a missing folder = %v, want not exist", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "gone")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the folder was made: %v", err)
	}
	if n := requests.Load(); n != 0 {
		t.Errorf("requests = %d", n)
	}
}
