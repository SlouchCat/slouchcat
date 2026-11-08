package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// MaxManifestBytes bounds the release manifest and the GitHub release record of the fallback.
const MaxManifestBytes = 64 << 10

// githubAPI is the GitHub REST API of the fallback.
const githubAPI = "https://api.github.com"

// StatusError is an HTTP answer other than 200 for the release manifest.
type StatusError struct{ Code int }

func (e *StatusError) Error() string { return fmt.Sprintf("status %d", e.Code) }

// FetchManifest returns the body of the release manifest at manifestURL, at most
// MaxManifestBytes+1 bytes, so the caller sees an oversized one. Both the server's update check
// and the updater read it here.
//
// GitHub serves release downloads (github.com/<owner>/<repo>/releases/latest/download/<name>)
// separately from its API, and on 8 November 2026 the download address answered 504 for hours
// while the API and the asset storage worked. When the address has that form and the request fails
// or answers 5xx, FetchManifest reads the same asset through the API: the latest release, then
// the asset by name. If that fails too, the error of the first request is returned.
func FetchManifest(ctx context.Context, client *http.Client, manifestURL, userAgent string) ([]byte, error) {
	body, err := get(ctx, client, manifestURL, userAgent, "application/json")
	if err == nil {
		return body, nil
	}
	var se *StatusError
	if errors.As(err, &se) && se.Code < http.StatusInternalServerError {
		return nil, err
	}
	owner, repo, name, ok := latestAsset(manifestURL)
	if !ok {
		return nil, err
	}
	if fb, ferr := fetchViaAPI(ctx, client, owner, repo, name, userAgent); ferr == nil {
		return fb, nil
	}
	return nil, err
}

// latestAsset splits https://github.com/<owner>/<repo>/releases/latest/download/<name>.
func latestAsset(raw string) (owner, repo, name string, ok bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.RawQuery != "" {
		return "", "", "", false
	}
	p := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(p) != 6 || p[2] != "releases" || p[3] != "latest" || p[4] != "download" {
		return "", "", "", false
	}
	for _, s := range []string{p[0], p[1], p[5]} {
		if s == "" || s == "." || s == ".." {
			return "", "", "", false
		}
	}
	return p[0], p[1], p[5], true
}

// fetchViaAPI reads the asset name of the latest release through the GitHub API. It follows only
// an asset address of the same repository; GitHub redirects that to its asset storage.
func fetchViaAPI(ctx context.Context, client *http.Client, owner, repo, name, userAgent string) ([]byte, error) {
	base := githubAPI + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/releases"
	raw, err := get(ctx, client, base+"/latest", userAgent, "application/vnd.github+json")
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxManifestBytes {
		return nil, errors.New("release record too large")
	}
	var rel struct {
		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(raw, &rel); err != nil {
		return nil, fmt.Errorf("release record: %w", err)
	}
	for _, a := range rel.Assets {
		if a.Name == name && strings.HasPrefix(a.URL, base+"/assets/") {
			return get(ctx, client, a.URL, userAgent, "application/octet-stream")
		}
	}
	return nil, fmt.Errorf("release has no asset %q", name)
}

// get reads at most MaxManifestBytes+1 bytes of a 200 answer.
func get(ctx context.Context, client *http.Client, address, userAgent, accept string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", accept)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, &StatusError{Code: resp.StatusCode}
	}
	return io.ReadAll(io.LimitReader(resp.Body, MaxManifestBytes+1))
}
