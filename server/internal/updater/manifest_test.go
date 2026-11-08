package updater

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

const (
	testManifestURL = "https://github.com/SlouchCat/slouchcat/releases/latest/download/manifest.json"
	testLatest      = "https://api.github.com/repos/SlouchCat/slouchcat/releases/latest"
	testAsset       = "https://api.github.com/repos/SlouchCat/slouchcat/releases/assets/7"
	testManifest    = `{"version":"1.17.0"}`
)

// stubTransport answers each address with a status and a body; an address without an entry is a
// transport error. It records the addresses asked for.
type stubTransport struct {
	answers map[string]stubAnswer
	asked   []string
}

type stubAnswer struct {
	status int
	body   string
}

func (s *stubTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	s.asked = append(s.asked, r.URL.String()+" "+r.Header.Get("Accept"))
	a, ok := s.answers[r.URL.String()]
	if !ok {
		return nil, errors.New("connection refused")
	}
	return &http.Response{StatusCode: a.status, Body: io.NopCloser(strings.NewReader(a.body)), Header: http.Header{}, Request: r}, nil
}

func releaseRecord(assetURL string) string {
	return `{"tag_name":"v1.17.0","assets":[{"name":"FFMPEG-NOTICE.md","url":"` + testAsset + `0"},{"name":"manifest.json","url":"` + assetURL + `"}]}`
}

// A failing download address of a GitHub release falls back to the API (8 November 2026: 504 for
// hours); any other answer, address or asset does not.
func TestFetchManifest(t *testing.T) {
	for name, c := range map[string]struct {
		url     string
		answers map[string]stubAnswer
		want    string // body, or the error text
		asked   int
	}{
		"direct": {testManifestURL, map[string]stubAnswer{testManifestURL: {200, testManifest}}, testManifest, 1},
		"504 falls back to the API": {testManifestURL, map[string]stubAnswer{
			testManifestURL: {504, ""}, testLatest: {200, releaseRecord(testAsset)}, testAsset: {200, testManifest},
		}, testManifest, 3},
		"unreachable falls back to the API": {testManifestURL, map[string]stubAnswer{
			testLatest: {200, releaseRecord(testAsset)}, testAsset: {200, testManifest},
		}, testManifest, 3},
		"404 is final": {testManifestURL, map[string]stubAnswer{testManifestURL: {404, ""}}, "status 404", 1},
		"other address has no fallback": {"https://example.org/manifest.json", map[string]stubAnswer{
			"https://example.org/manifest.json": {504, ""},
		}, "status 504", 1},
		"failed fallback returns the first error": {testManifestURL, map[string]stubAnswer{
			testManifestURL: {504, ""}, testLatest: {403, ""},
		}, "status 504", 2},
		"asset of another repository is not followed": {testManifestURL, map[string]stubAnswer{
			testManifestURL: {502, ""}, testLatest: {200, releaseRecord("https://api.github.com/repos/other/repo/releases/assets/7")},
		}, "status 502", 2},
	} {
		t.Run(name, func(t *testing.T) {
			tr := &stubTransport{answers: c.answers}
			body, err := FetchManifest(context.Background(), &http.Client{Transport: tr}, c.url, "test")
			got := string(body)
			if err != nil {
				got = err.Error()
			}
			if got != c.want || len(tr.asked) != c.asked {
				t.Errorf("got %q after %v, want %q after %d requests", got, tr.asked, c.want, c.asked)
			}
		})
	}
}

func TestLatestAsset(t *testing.T) {
	for raw, want := range map[string]bool{
		testManifestURL: true,
		"https://github.com/SlouchCat/slouchcat/releases/download/v1.17.0/manifest.json":  false,
		"http://github.com/SlouchCat/slouchcat/releases/latest/download/manifest.json":    false,
		"https://github.com/SlouchCat/slouchcat/releases/latest/download/..":              false,
		"https://github.com/SlouchCat/slouchcat/releases/latest/download/manifest.json?x": false,
	} {
		if _, _, _, ok := latestAsset(raw); ok != want {
			t.Errorf("latestAsset(%q) = %v, want %v", raw, ok, want)
		}
	}
}
