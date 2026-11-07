package radio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/netguard"
	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
)

// The Radio Browser client (concept 16 section 5.1, F-14): one GET of the whole station list per
// sync, streamed into the merge. Every request goes through the guarded dialer of the relay part
// (dialer.go: no address in the local network; the radiotest build allows the loopback), the rate
// limiter of the service and the User-Agent Slouchcat/<version>.
// The server never reports clicks or votes back.

const (
	// catalogBodyLimit caps the station list (today ~40 MB).
	catalogBodyLimit = 200 << 20
	// catalogTimeout bounds one whole fetch, mirrors included.
	catalogTimeout = 10 * time.Minute
	// catalogMaxMirrors is the number of mirrors one sync tries after the base failed.
	catalogMaxMirrors = 3
	// catalogMirrorSuffix is the host suffix a mirror must have: the wildcard certificate of
	// all.api covers *.api.radio-browser.info, and a mirror is dialled by name, never by IP.
	catalogMirrorSuffix = ".api.radio-browser.info"
	// catalogStationsPath lists every station, broken ones included (the merge decides).
	catalogStationsPath = "/json/stations?hidebroken=false"
)

// errCatalogTransport marks a failure that a mirror may stand in for (connection, TLS, 5xx).
var errCatalogTransport = errors.New("radio browser unreachable")

// rbStation is one entry of /json/stations (the fields the merge reads).
type rbStation struct {
	StationUUID    string `json:"stationuuid"`
	Name           string `json:"name"`
	URL            string `json:"url"`
	URLResolved    string `json:"url_resolved"`
	Homepage       string `json:"homepage"`
	Favicon        string `json:"favicon"`
	Tags           string `json:"tags"`
	CountryCode    string `json:"countrycode"`
	ISO31662       string `json:"iso_3166_2"`
	State          string `json:"state"`
	Language       string `json:"language"`
	LanguageCodes  string `json:"languagecodes"`
	Codec          string `json:"codec"`
	Bitrate        int64  `json:"bitrate"`
	HLS            int64  `json:"hls"`
	LastCheckOK    int64  `json:"lastcheckok"`
	Votes          int64  `json:"votes"`
	ClickCount     int64  `json:"clickcount"`
	LastChangeTime string `json:"lastchangetime"`
}

// catalogClient fetches the station list from the base or, after a transport failure, from up to
// catalogMaxMirrors mirrors.
type catalogClient struct {
	client    *http.Client        // nil: lazy
	lazy      func() *http.Client // the service's guarded client (netClient)
	limiter   *ratelimit.Limiter
	userAgent string
	base      string
	// mirrors is false in the radiotest build (the base is the e2e catalog).
	mirrors bool
	// serversURL lists the mirrors ([{"name": "de1.api.radio-browser.info"}, …]).
	serversURL string
	// lookupSRV resolves the mirrors from DNS when serversURL fails (_api._tcp.radio-browser.info).
	lookupSRV func(ctx context.Context) ([]string, error)
	// mirrorBase returns the base URL of a mirror host (https://<name>).
	mirrorBase func(name string) string
}

// newCatalogClient returns the client of the Radio Browser. Its HTTP client is the guarded one
// of the service (lazy); a test sets client instead.
func newCatalogClient(userAgent string) *catalogClient {
	base, mirrors := catalogBase()
	if userAgent == "" {
		userAgent = "Slouchcat"
	}
	return &catalogClient{
		limiter: ratelimit.New("radio-browser", 1, 1), userAgent: userAgent,
		base: base, mirrors: mirrors, serversURL: base + "/json/servers",
		lookupSRV: func(ctx context.Context) ([]string, error) {
			_, srvs, err := net.DefaultResolver.LookupSRV(ctx, "api", "tcp", "radio-browser.info")
			if err != nil {
				return nil, err
			}
			out := make([]string, 0, len(srvs))
			for _, s := range srvs {
				out = append(out, strings.TrimSuffix(s.Target, "."))
			}
			return out, nil
		},
		mirrorBase: func(name string) string { return "https://" + name },
	}
}

// httpClient returns the client of the requests: client when a test set one, else the guarded
// client of the service.
func (c *catalogClient) httpClient() *http.Client {
	if c.client != nil {
		return c.client
	}
	return c.lazy()
}

// fetch streams every station of the catalog into fn, from the base or a mirror. A mirror is only
// asked when the base failed before the first entry arrived.
func (c *catalogClient) fetch(ctx context.Context, fn func(*rbStation) error) error {
	ctx, cancel := context.WithTimeout(ctx, catalogTimeout)
	defer cancel()
	err := c.fetchFrom(ctx, c.base, fn)
	if err == nil || !errors.Is(err, errCatalogTransport) || !c.mirrors {
		return err
	}
	names, merr := c.mirrorNames(ctx)
	if merr != nil {
		return errors.Join(err, merr)
	}
	errs := []error{err}
	for _, name := range names {
		err := c.fetchFrom(ctx, c.mirrorBase(name), fn)
		if err == nil || !errors.Is(err, errCatalogTransport) {
			return err
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// mirrorNames returns up to catalogMaxMirrors mirror host names from /json/servers, else from DNS
// SRV; only names below catalogMirrorSuffix count.
func (c *catalogClient) mirrorNames(ctx context.Context) ([]string, error) {
	names, err := c.serverList(ctx)
	if err != nil || len(names) == 0 {
		srv, srvErr := c.lookupSRV(ctx)
		if srvErr != nil {
			return nil, fmt.Errorf("radio browser mirrors: %w", errors.Join(err, srvErr))
		}
		names = srv
	}
	out := make([]string, 0, catalogMaxMirrors)
	seen := map[string]bool{}
	for _, n := range names {
		n = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(n), "."))
		if !strings.HasSuffix(n, catalogMirrorSuffix) || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
		if len(out) == catalogMaxMirrors {
			break
		}
	}
	if len(out) == 0 {
		return nil, errors.New("radio browser mirrors: none found")
	}
	return out, nil
}

// serverList reads /json/servers.
func (c *catalogClient) serverList(ctx context.Context) ([]string, error) {
	resp, err := c.get(ctx, c.serversURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var list []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&list); err != nil {
		return nil, fmt.Errorf("radio browser servers: %w", err)
	}
	names := make([]string, 0, len(list))
	for _, s := range list {
		names = append(names, s.Name)
	}
	return names, nil
}

// get sends one GET through the limiter; a transport error or a 5xx is errCatalogTransport,
// another non-2xx a plain error. The error names the host only (netguard.Clean).
func (c *catalogClient) get(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("radio browser request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := c.limiter.Do(ctx, c.httpClient(), req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w: %w", errCatalogTransport, netguard.Clean(err))
	}
	if resp.StatusCode/100 != 2 {
		_ = resp.Body.Close()
		err := fmt.Errorf("radio browser %s answered %d", netguard.Site(rawURL), resp.StatusCode)
		if resp.StatusCode >= 500 {
			return nil, fmt.Errorf("%w: %w", errCatalogTransport, err)
		}
		return nil, err
	}
	return resp, nil
}

// fetchFrom streams /json/stations of base into fn. A failure after the first entry is no
// transport failure any more: a mirror would deliver the entries a second time.
func (c *catalogClient) fetchFrom(ctx context.Context, base string, fn func(*rbStation) error) error {
	resp, err := c.get(ctx, base+catalogStationsPath)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	dec := json.NewDecoder(&cappedReader{r: resp.Body, left: catalogBodyLimit})
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("%w: read station list: %w", errCatalogTransport, err)
	}
	if d, ok := tok.(json.Delim); !ok || d != '[' {
		return errors.New("radio browser station list is no array")
	}
	for dec.More() {
		var e rbStation
		if err := dec.Decode(&e); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("radio browser station list: %w", err)
		}
		if err := fn(&e); err != nil {
			return err
		}
	}
	if _, err := dec.Token(); err != nil {
		return fmt.Errorf("radio browser station list: %w", err)
	}
	return nil
}

// errCatalogTooLarge is the read error past catalogBodyLimit.
var errCatalogTooLarge = errors.New("radio browser station list larger than 200 MiB")

// cappedReader fails once more than left bytes were read.
type cappedReader struct {
	r    io.Reader
	left int64
}

func (c *cappedReader) Read(p []byte) (int, error) {
	if c.left <= 0 {
		return 0, errCatalogTooLarge
	}
	if int64(len(p)) > c.left {
		p = p[:c.left]
	}
	n, err := c.r.Read(p)
	c.left -= int64(n)
	return n, err
}
