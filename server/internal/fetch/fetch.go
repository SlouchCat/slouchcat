// Package fetch loads the enclosure of a podcast episode into a file below an *os.Root, with
// resume (concept 19 part A, 6.2; ADR 0029). It is the loader the offline download had for itself
// (M6, offline.fetchSource) and that the stored podcast episodes share now: a partial file that
// continues with a Range request guarded by If-Range, a validator file next to it, at most 2 GiB,
// http and https only. The caller chooses the names, the file mode and the guarded client
// (Clients().For with the podcast's feed, package netguard); errors name the host of a URL only,
// never its path or query (CLAUDE.md rule 10).
package fetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/netguard"
)

// Limits of a fetch.
const (
	MaxBytes      = 2 << 30
	HeaderTimeout = 30 * time.Second
	MaxRedirects  = 5
)

// ErrUnavailable marks a fetch that failed (an invalid URL, a refused or failed request, an
// answer that is neither 200 nor a fitting 206 or 416, a body cut short): the next attempt may
// succeed.
var ErrUnavailable = errors.New("enclosure not available")

// ErrTooLarge marks a body over MaxBytes; it always comes wrapped together with ErrUnavailable.
var ErrTooLarge = errors.New("enclosure larger than 2 GiB")

// clients are the guarded clients of every fetch: the response headers within HeaderTimeout, at
// most MaxRedirects redirects, http and https only, no overall limit (the body is capped at
// MaxBytes), no address in the local network unless the podcast's feed is there itself.
var clients = netguard.NewClients(http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > MaxRedirects || (req.URL.Scheme != "http" && req.URL.Scheme != "https") {
			return fmt.Errorf("%w: too many redirects or another scheme", ErrUnavailable)
		}
		return nil
	},
}, HeaderTimeout)

// Clients returns the guarded clients of the fetches (netguard.Clients.For picks the one for a
// feed and its origin).
func Clients() netguard.Clients { return clients }

// Request describes one fetch. Part, Validator and Final are slash-separated paths below Root;
// the directory of Part must exist or is created with DirMode.
type Request struct {
	Client    *http.Client // the guarded client of the podcast's feed (Clients().For)
	URL       string
	Root      *os.Root
	Part      string      // the partial file; continued when it is there
	Validator string      // the strong ETag, else the Last-Modified of the answer that started Part
	Final     string      // the complete file: Part is renamed to it; reused when it is there
	FileMode  fs.FileMode // of Part and Validator when they are created (before the umask)
	DirMode   fs.FileMode // of the directory of Part when it is created (before the umask)
	// NoMkdir: the directory of Part must be there; it is never made (the fetch fails with
	// fs.ErrNotExist before any request). A stored podcast episode never makes a known folder anew.
	NoMkdir bool
	// Progress hears the bytes of Part while the body is read (optional); total is the size the
	// answer announced (-1 when unknown).
	Progress func(have, total int64)
	// Log hears a validator that could not be removed (optional).
	Log *slog.Logger
}

// Result is what a fetch left: the size of Final, the media type the answer named (empty when
// Final was there already) and whether Final was reused without a request.
type Result struct {
	Bytes       int64
	ContentType string
	Reused      bool
}

// progressStep is how many bytes the body reader reads between two Progress calls.
const progressStep = 1 << 20

// Fetch loads r.URL into r.Final. A complete file is reused; a partial one continues with a Range
// request guarded by If-Range with the stored validator, and only when the answer resumes at the
// byte the partial file ends with; anything else starts over (the partial file is emptied and the
// error asks for the next attempt). Failures wrap ErrUnavailable (a body over MaxBytes also
// ErrTooLarge); file system errors come as they are.
func Fetch(ctx context.Context, r Request) (Result, error) {
	u, err := url.Parse(r.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Result{}, fmt.Errorf("%w: invalid enclosure URL", ErrUnavailable)
	}
	if info, err := r.Root.Stat(r.Final); err == nil && !info.IsDir() {
		return Result{Bytes: info.Size(), Reused: true}, nil
	}
	if dir := path.Dir(r.Part); dir != "." {
		if r.NoMkdir {
			if info, err := r.Root.Stat(dir); err != nil || !info.IsDir() {
				return Result{}, fmt.Errorf("directory of the partial file: %w", fs.ErrNotExist)
			}
		} else if err := r.Root.MkdirAll(dir, r.DirMode); err != nil {
			return Result{}, err
		}
	}
	f, err := r.Root.OpenFile(r.Part, os.O_RDWR|os.O_CREATE, r.FileMode)
	if err != nil {
		return Result{}, err
	}
	defer f.Close()
	have, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrUnavailable, netguard.Clean(err))
	}
	req.Header.Set("User-Agent", "Slouchcat")
	// The bytes of a resume must count the same representation: no transparent gzip.
	req.Header.Set("Accept-Encoding", "identity")
	if have > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(have, 10)+"-")
		if v, err := r.Root.ReadFile(r.Validator); err == nil && len(v) > 0 {
			req.Header.Set("If-Range", string(v))
		}
	}
	resp, err := r.Client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrUnavailable, netguard.Clean(err))
	}
	defer resp.Body.Close()
	res := Result{ContentType: resp.Header.Get("Content-Type")}
	first, total, rangeOK := ContentRange(resp.Header.Get("Content-Range"))
	switch {
	case resp.StatusCode == http.StatusPartialContent && have > 0 && rangeOK && first == have:
	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable && have > 0 && rangeOK && total == have:
		// The partial file already holds everything.
		res.Bytes = have
		return res, complete(r)
	case resp.StatusCode == http.StatusPartialContent || resp.StatusCode == http.StatusRequestedRangeNotSatisfiable:
		// The host resumed elsewhere or the file changed size: start over with the next attempt.
		if err := f.Truncate(0); err != nil {
			return Result{}, err
		}
		return Result{}, fmt.Errorf("%w: resume at byte %d answered HTTP %d with range %q", ErrUnavailable, have,
			resp.StatusCode, resp.Header.Get("Content-Range"))
	case resp.StatusCode == http.StatusOK:
		if err := f.Truncate(0); err != nil {
			return Result{}, err
		}
		if have, err = f.Seek(0, io.SeekStart); err != nil {
			return Result{}, err
		}
		total = resp.ContentLength
		v := resp.Header.Get("ETag")
		if v == "" || strings.HasPrefix(v, "W/") {
			v = resp.Header.Get("Last-Modified")
		}
		if err := r.Root.WriteFile(r.Validator, []byte(v), r.FileMode); err != nil {
			return Result{}, err
		}
	default:
		return Result{}, fmt.Errorf("%w: enclosure host answered HTTP %d", ErrUnavailable, resp.StatusCode)
	}
	body := io.LimitReader(resp.Body, MaxBytes-have+1)
	if r.Progress != nil {
		body = &progressReader{r: body, have: have, total: total, fn: r.Progress}
	}
	n, err := io.Copy(f, body)
	switch {
	case err != nil:
		return Result{}, fmt.Errorf("%w: %w", ErrUnavailable, netguard.Clean(err))
	case have+n > MaxBytes:
		return Result{}, fmt.Errorf("%w: %w", ErrUnavailable, ErrTooLarge)
	}
	if err := f.Sync(); err != nil {
		return Result{}, err
	}
	if err := complete(r); err != nil {
		return Result{}, err
	}
	res.Bytes = have + n
	return res, nil
}

// complete renames the partial file to its final name and drops its validator.
func complete(r Request) error {
	if err := r.Root.Rename(r.Part, r.Final); err != nil {
		return err
	}
	if err := r.Root.Remove(r.Validator); err != nil && !errors.Is(err, fs.ErrNotExist) && r.Log != nil {
		r.Log.Warn("remove fetch validator", "path", r.Validator, "err", err)
	}
	return nil
}

// progressReader reports the bytes of the partial file every progressStep bytes.
type progressReader struct {
	r         io.Reader
	have      int64
	total     int64
	sinceLast int64
	fn        func(have, total int64)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.have += int64(n)
	p.sinceLast += int64(n)
	if p.sinceLast >= progressStep || err != nil {
		p.sinceLast = 0
		p.fn(p.have, p.total)
	}
	return n, err
}

// ContentRange parses a Content-Range header, "bytes first-last/total" or "bytes */total"; first
// is -1 in the second form, total -1 when it is "*".
func ContentRange(v string) (first, total int64, ok bool) {
	spec, found := strings.CutPrefix(strings.TrimSpace(v), "bytes ")
	rng, size, found2 := strings.Cut(spec, "/")
	if !found || !found2 {
		return 0, 0, false
	}
	total = -1
	if size != "*" {
		if total, ok = parseBytes(size); !ok {
			return 0, 0, false
		}
	}
	if rng == "*" {
		return -1, total, true
	}
	lo, hi, found := strings.Cut(rng, "-")
	if !found {
		return 0, 0, false
	}
	first, ok = parseBytes(lo)
	last, ok2 := parseBytes(hi)
	if !ok || !ok2 || last < first {
		return 0, 0, false
	}
	return first, total, true
}

func parseBytes(v string) (int64, bool) {
	n, err := strconv.ParseInt(v, 10, 64)
	return n, err == nil && n >= 0
}

// Ext is the file extension of an enclosure from its URL path ("audio" when it has none of the
// known audio extensions).
func Ext(p string) string {
	switch ext := strings.ToLower(strings.TrimPrefix(path.Ext(p), ".")); ext {
	case "mp3", "m4a", "m4b", "mp4", "aac", "ogg", "opus", "flac", "wav":
		return ext
	}
	return "audio"
}
