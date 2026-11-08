package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

const (
	// MaxManifestBytes bounds the release manifest.
	MaxManifestBytes = 64 << 10
	// MaxReleaseFileBytes bounds the other files of a release the updater reads (the compose
	// files) and the GitHub release record of the fallback, which carries the notes.
	MaxReleaseFileBytes = 1 << 20

	// githubAPI is the GitHub REST API of the fallback.
	githubAPI = "https://api.github.com"
	// attemptTimeout bounds one request to the download address; attempts is how often it is
	// asked before the fallback, attemptPause the wait in between. Together they leave the API
	// fallback at least half of a 20 s budget.
	attemptTimeout = 4 * time.Second
	attempts       = 2
	attemptPause   = time.Second
)

// StatusError is an HTTP answer other than 200 for a release file.
type StatusError struct{ Code int }

func (e *StatusError) Error() string { return fmt.Sprintf("status %d", e.Code) }

// FetchManifest returns the body of the release manifest at manifestURL, at most
// MaxManifestBytes+1 bytes, so the caller sees an oversized one. Both the server's update check
// and the updater read it here (FetchReleaseFile).
func FetchManifest(ctx context.Context, client *http.Client, manifestURL, userAgent string) ([]byte, error) {
	return FetchReleaseFile(ctx, client, manifestURL, userAgent, MaxManifestBytes)
}

// FetchReleaseFile returns the body of a release file, at most limit+1 bytes.
//
// GitHub serves release downloads (github.com/<owner>/<repo>/releases/latest/download/<name> and
// …/releases/download/<tag>/<name>) apart from its API, and on 8 November 2026 the download
// address answered 504 for hours, also for other projects, while the API and the asset storage
// worked. So a network error or a 5xx answer is asked again once after a second; when it persists
// and the address has one of those forms, the same asset is read through the API (the release,
// then the asset by name, only an asset address of the same repository). If that fails too, the
// error names both. Any other answer (404, 403) is final. ctx bounds the whole call.
func FetchReleaseFile(ctx context.Context, client *http.Client, fileURL, userAgent string, limit int64) ([]byte, error) {
	var err error
	for i := range attempts {
		if i > 0 {
			select {
			case <-ctx.Done():
				return nil, err
			case <-time.After(attemptPause):
			}
		}
		actx, cancel := context.WithTimeout(ctx, attemptTimeout)
		var body []byte
		body, err = get(actx, client, fileURL, userAgent, "application/json, application/octet-stream", limit)
		cancel()
		if err == nil {
			return body, nil
		}
		if !retryable(err) {
			return nil, err
		}
	}
	ref, ok := githubAsset(fileURL)
	if !ok {
		return nil, err
	}
	body, ferr := fetchViaAPI(ctx, client, ref, userAgent, limit)
	if ferr != nil {
		return nil, fmt.Errorf("%w (API fallback: %v)", err, ferr)
	}
	return body, nil
}

// retryable reports whether err may pass on another try: a network error or a 5xx answer.
func retryable(err error) bool {
	var se *StatusError
	if errors.As(err, &se) {
		return se.Code >= http.StatusInternalServerError
	}
	return true
}

// ReleaseFileURL is the download address of the file name of release version (X.Y.Z), derived
// from the manifest address: …/releases/download/v<version>/<name> for a GitHub release address,
// otherwise the file next to the manifest (a mirror or a test server).
func ReleaseFileURL(manifestURL, version, name string) (string, error) {
	u, err := url.Parse(manifestURL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return "", fmt.Errorf("manifest address %q is not https", manifestURL)
	}
	if !ValidRelease(version) || name == "" || strings.ContainsAny(name, "/\\") || name == "." || name == ".." {
		return "", fmt.Errorf("no release file %q of %q", name, version)
	}
	if ref, ok := githubAsset(manifestURL); ok {
		u.Path = "/" + ref.owner + "/" + ref.repo + "/releases/download/v" + version + "/" + name
		return u.String(), nil
	}
	u.Path = path.Join(path.Dir(u.Path), name)
	u.RawQuery = ""
	return u.String(), nil
}

// assetRef is a release file on GitHub: tag is empty for the latest release.
type assetRef struct {
	owner, repo, tag, name string
}

// githubAsset splits https://github.com/<owner>/<repo>/releases/latest/download/<name> and
// https://github.com/<owner>/<repo>/releases/download/<tag>/<name>.
func githubAsset(raw string) (assetRef, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || !strings.EqualFold(u.Host, "github.com") || u.RawQuery != "" {
		return assetRef{}, false
	}
	p := strings.Split(strings.Trim(u.Path, "/"), "/")
	var r assetRef
	switch {
	case len(p) == 6 && p[2] == "releases" && p[3] == "latest" && p[4] == "download":
		r = assetRef{owner: p[0], repo: p[1], name: p[5]}
	case len(p) == 6 && p[2] == "releases" && p[3] == "download":
		r = assetRef{owner: p[0], repo: p[1], tag: p[4], name: p[5]}
	default:
		return assetRef{}, false
	}
	for _, s := range []string{r.owner, r.repo, r.name, r.tag} {
		if s == "." || s == ".." {
			return assetRef{}, false
		}
	}
	if r.owner == "" || r.repo == "" || r.name == "" {
		return assetRef{}, false
	}
	return r, true
}

// fetchViaAPI reads the asset through the GitHub API. It follows only an asset address of the
// same repository (compared case-insensitively, as GitHub treats names); GitHub redirects that
// to its asset storage.
func fetchViaAPI(ctx context.Context, client *http.Client, ref assetRef, userAgent string, limit int64) ([]byte, error) {
	base := githubAPI + "/repos/" + url.PathEscape(ref.owner) + "/" + url.PathEscape(ref.repo) + "/releases"
	release := base + "/latest"
	if ref.tag != "" {
		release = base + "/tags/" + url.PathEscape(ref.tag)
	}
	raw, err := get(ctx, client, release, userAgent, "application/vnd.github+json", MaxReleaseFileBytes)
	if err != nil {
		return nil, fmt.Errorf("release record: %w", err)
	}
	if int64(len(raw)) > MaxReleaseFileBytes {
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
	prefix := strings.ToLower(base + "/assets/")
	for _, a := range rel.Assets {
		lower := strings.ToLower(a.URL)
		if a.Name == ref.name && strings.HasPrefix(lower, prefix) && !strings.Contains(lower[len(prefix):], "/") {
			body, err := get(ctx, client, a.URL, userAgent, "application/octet-stream", limit)
			if err != nil {
				return nil, fmt.Errorf("asset: %w", err)
			}
			return body, nil
		}
	}
	return nil, fmt.Errorf("release has no asset %q", ref.name)
}

// get reads at most limit+1 bytes of a 200 answer.
func get(ctx context.Context, client *http.Client, address, userAgent, accept string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", accept)
	if strings.HasPrefix(address, githubAPI+"/") {
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, &StatusError{Code: resp.StatusCode}
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit+1))
}
