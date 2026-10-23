package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

// The podcast fixture: the server needs a feed it can subscribe to, and no run may reach the
// internet (H-2). This is the same fixture the Playwright suite uses (web/e2e/feed-server.mjs)
// with the same files — testdata/podcasts/feed.xml.tmpl and testdata/media/Podcasts — served from
// this process so the check needs nothing but the server and the test data.

// itemBlock matches one <item> element of the feed template.
var itemBlock = regexp.MustCompile(`(?s)[ \t]*<item>.*?</item>\n?`)

// episodeFile matches the enclosure paths of the template.
var episodeFile = regexp.MustCompile(`\A/episode-([1-3])\.mp3\z`)

// feedServer serves the test feed on the loopback. episodes says how many of the template's items
// the feed currently shows; raising it is "a new episode appeared" (E-8).
type feedServer struct {
	url      string
	template string
	files    http.Handler
	episodes atomic.Int32
	srv      *http.Server
	done     chan struct{}
}

// startFeed reads the feed template and serves it on a free loopback port with count episodes.
func startFeed(ctx context.Context, testdata string, count int32) (*feedServer, error) {
	template, err := os.ReadFile(filepath.Join(testdata, "podcasts", "feed.xml.tmpl")) //nolint:gosec // G304: test data directory from a flag of this developer tool
	if err != nil {
		return nil, fmt.Errorf("read feed template: %w", err)
	}
	var config net.ListenConfig
	listener, err := config.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen for the feed: %w", err)
	}
	f := &feedServer{
		url:      "http://" + listener.Addr().String(),
		template: string(template),
		files:    http.FileServerFS(os.DirFS(filepath.Join(testdata, "media", "Podcasts"))),
		episodes: atomic.Int32{},
		srv:      nil,
		done:     make(chan struct{}),
	}
	f.episodes.Store(count)
	f.srv = &http.Server{Handler: f, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		defer close(f.done)
		if err := f.srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "feed server: %v\n", err)
		}
	}()
	return f, nil
}

// feedURL is the address the server subscribes to.
func (f *feedServer) feedURL() string { return f.url + "/feed.xml" }

// setEpisodes publishes the first n items of the template.
func (f *feedServer) setEpisodes(n int32) { f.episodes.Store(n) }

// stop shuts the fixture down.
func (f *feedServer) stop(ctx context.Context) {
	_ = f.srv.Shutdown(ctx)
	<-f.done
}

func (f *feedServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	switch {
	case r.URL.Path == "/feed.xml":
		w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(f.render()))
	case episodeFile.MatchString(r.URL.Path), r.URL.Path == "/cover.jpg":
		// Only the enclosures and the cover of the template reach the file server, and it serves
		// nothing above testdata/media/Podcasts (Range requests included).
		f.files.ServeHTTP(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// render returns the template with its first episodes items and the links pointing at this server.
func (f *feedServer) render() string {
	limit := f.episodes.Load()
	index := int32(0)
	body := itemBlock.ReplaceAllStringFunc(f.template, func(item string) string {
		index++
		if index <= limit {
			return item
		}
		return ""
	})
	return strings.ReplaceAll(body, "{{BASE}}", f.url)
}
