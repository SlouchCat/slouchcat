package playback

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// blockingConverter counts conversions, records how many run at once and blocks each one until
// release is closed.
type blockingConverter struct {
	mu      sync.Mutex
	calls   int
	running int
	peak    int
	release chan struct{}
	entered chan struct{}
}

func newBlockingConverter() *blockingConverter {
	return &blockingConverter{release: make(chan struct{}), entered: make(chan struct{}, 64)}
}

func (c *blockingConverter) embedded(ctx context.Context, _ string, _ int) ([]byte, error) {
	c.mu.Lock()
	c.calls++
	c.running++
	c.peak = max(c.peak, c.running)
	c.mu.Unlock()
	c.entered <- struct{}{}
	defer func() {
		c.mu.Lock()
		c.running--
		c.mu.Unlock()
	}()
	select {
	case <-c.release:
		return []byte("WEBVTT\n"), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *blockingConverter) sidecar(ctx context.Context, path string) ([]byte, error) {
	return c.embedded(ctx, path, -1)
}

func (c *blockingConverter) stats() (calls, peak int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls, c.peak
}

// subtitleGrant starts a playback with the embedded text track in a new browser session and
// returns its grant.
func subtitleGrant(t *testing.T, s *Service, f *vistest.Fixture, m vistest.Movie, track int64) model.PlaybackGrant {
	t.Helper()
	caps := capsChromeLinux
	info, err := s.Start(context.Background(), browserSession(t, f, f.P18), m.ID, model.PlaybackRequest{Capabilities: &caps, SubtitleTrack: track})
	if err != nil {
		t.Fatal(err)
	}
	g, err := s.Authorize(context.Background(), tokenOf(t, info.SubtitleURL))
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// Ten concurrent requests of one playback and track run one conversion; a request whose client
// goes away does not cancel it for the others.
func TestSubtitleConversionShared(t *testing.T) {
	f := vistest.New(t)
	tracks := f.AddTracks(t)
	m := f.Rated(6)
	f.SetMediaFile(t, m.FileID, "mp4", "h264", "aac", trackProbe(t, "h264", "bt709", 1920, 1080, 6_000_000), testKeyframes)
	conv := newBlockingConverter()
	s := New(f.DB, vistest.Logger(), WithTokenKey(testKey), WithSubtitleConverter(conv.embedded, conv.sidecar), WithCacheDir(t.TempDir()))
	g := subtitleGrant(t, s, f, m, tracks[m.FileID].Text)

	gone, cancel := context.WithCancel(context.Background())
	goneErr := make(chan error, 1)
	go func() {
		_, err := s.Subtitle(gone, g, g.SubtitleTrack)
		goneErr <- err
	}()
	<-conv.entered
	cancel()
	if err := <-goneErr; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled request: %v", err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			src, err := s.Subtitle(context.Background(), g, g.SubtitleTrack)
			if err == nil && readAll(t, src) != "WEBVTT\n" {
				err = errors.New("wrong content")
			}
			errs <- err
		}()
	}
	time.Sleep(50 * time.Millisecond) // let the requests join the running conversion
	close(conv.release)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
	if calls, _ := conv.stats(); calls != 1 {
		t.Errorf("conversions = %d, want 1", calls)
	}
	// Later requests read the cache.
	if _, err := s.Subtitle(context.Background(), g, g.SubtitleTrack); err != nil {
		t.Fatal(err)
	}
	if calls, _ := conv.stats(); calls != 1 {
		t.Errorf("conversions after the cache = %d, want 1", calls)
	}
}

// At most maxSubtitleConversions conversions run at once.
func TestSubtitleConversionsBounded(t *testing.T) {
	f := vistest.New(t)
	tracks := f.AddTracks(t)
	m := f.Rated(6)
	f.SetMediaFile(t, m.FileID, "mp4", "h264", "aac", trackProbe(t, "h264", "bt709", 1920, 1080, 6_000_000), testKeyframes)
	conv := newBlockingConverter()
	s := New(f.DB, vistest.Logger(), WithTokenKey(testKey), WithSubtitleConverter(conv.embedded, conv.sidecar))
	const n = maxSubtitleConversions + 3
	var wg sync.WaitGroup
	for range n {
		g := subtitleGrant(t, s, f, m, tracks[m.FileID].Text)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Subtitle(context.Background(), g, g.SubtitleTrack); err != nil {
				t.Error(err)
			}
		}()
	}
	for range maxSubtitleConversions {
		<-conv.entered
	}
	time.Sleep(50 * time.Millisecond)
	if _, peak := conv.stats(); peak != maxSubtitleConversions {
		t.Errorf("conversions at once = %d, want %d", peak, maxSubtitleConversions)
	}
	close(conv.release)
	wg.Wait()
	if calls, peak := conv.stats(); calls != n || peak > maxSubtitleConversions {
		t.Errorf("conversions = %d, peak %d", calls, peak)
	}
}

// A missing source answers 404 instead of a failed conversion: the sidecar file was deleted, the
// media file was deleted.
func TestSubtitleMissingSource(t *testing.T) {
	f := vistest.New(t)
	tracks := f.AddTracks(t)
	m := f.Rated(6)
	ft := tracks[m.FileID]
	f.SetMediaFile(t, m.FileID, "mp4", "h264", "aac", trackProbe(t, "h264", "bt709", 1920, 1080, 6_000_000), testKeyframes)
	conv := &fakeConverter{}
	s := New(f.DB, vistest.Logger(), WithTokenKey(testKey), WithSubtitleConverter(conv.embedded, conv.sidecar))
	sidecar := subtitleGrant(t, s, f, m, ft.Sidecar)
	if err := os.Remove(filepath.Join(f.MediaDir, filepath.FromSlash(ft.SidecarRel))); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Subtitle(context.Background(), sidecar, ft.Sidecar); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("deleted sidecar: %v", err)
	}
	embedded := subtitleGrant(t, s, f, m, ft.Text)
	if err := os.Remove(filepath.Join(f.MediaDir, filepath.FromSlash(m.RelPath))); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Subtitle(context.Background(), embedded, ft.Text); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("deleted media file: %v", err)
	}
	if conv.count() != 0 {
		t.Errorf("conversions = %d, want 0", conv.count())
	}
}
