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
	testTagURL      = "https://github.com/SlouchCat/slouchcat/releases/download/v1.17.0/manifest.json"
	testTag         = "https://api.github.com/repos/SlouchCat/slouchcat/releases/tags/v1.17.0"
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

// A failing download address of a GitHub release is asked twice, then read through the API
// (8 November 2026: 504 for hours); any other answer, address or asset is final.
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
		}, testManifest, 4},
		"unreachable falls back to the API": {testManifestURL, map[string]stubAnswer{
			testLatest: {200, releaseRecord(testAsset)}, testAsset: {200, testManifest},
		}, testManifest, 4},
		"404 is final": {testManifestURL, map[string]stubAnswer{testManifestURL: {404, ""}}, "status 404", 1},
		"other address has no fallback": {"https://example.org/manifest.json", map[string]stubAnswer{
			"https://example.org/manifest.json": {504, ""},
		}, "status 504", 2},
		"failed fallback names both errors": {testManifestURL, map[string]stubAnswer{
			testManifestURL: {504, ""}, testLatest: {403, ""},
		}, "status 504 (API fallback: release record: status 403)", 3},
		"asset of another repository is not followed": {testManifestURL, map[string]stubAnswer{
			testManifestURL: {502, ""}, testLatest: {200, releaseRecord("https://api.github.com/repos/other/repo/releases/assets/7")},
		}, `status 502 (API fallback: release has no asset "manifest.json")`, 3},
		"a tag address reads that release": {testTagURL, map[string]stubAnswer{
			testTagURL: {503, ""}, testTag: {200, releaseRecord(testAsset)}, testAsset: {200, testManifest},
		}, testManifest, 4},
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

func TestGithubAsset(t *testing.T) {
	for raw, want := range map[string]bool{
		testManifestURL: true,
		testTagURL:      true,
		"https://GitHub.com/SlouchCat/slouchcat/releases/latest/download/manifest.json":   true,
		"http://github.com/SlouchCat/slouchcat/releases/latest/download/manifest.json":    false,
		"https://github.com/SlouchCat/slouchcat/releases/latest/download/..":              false,
		"https://github.com/SlouchCat/slouchcat/releases/latest/download/manifest.json?x": false,
		"https://github.com/SlouchCat/slouchcat/archive/main.zip":                         false,
	} {
		if _, ok := githubAsset(raw); ok != want {
			t.Errorf("githubAsset(%q) = %v, want %v", raw, ok, want)
		}
	}
}

func TestReleaseFileURL(t *testing.T) {
	for _, c := range []struct{ manifest, version, name, want string }{
		{testManifestURL, "1.18.0", "compose.yaml", "https://github.com/SlouchCat/slouchcat/releases/download/v1.18.0/compose.yaml"},
		{"https://mirror.example/slouchcat/manifest.json", "1.18.0", "compose.yaml", "https://mirror.example/slouchcat/compose.yaml"},
		{testManifestURL, "1.18.0-rc1", "compose.yaml", ""},
		{testManifestURL, "1.18.0", "../x", ""},
		{"http://mirror.example/manifest.json", "1.18.0", "compose.yaml", ""},
	} {
		got, err := ReleaseFileURL(c.manifest, c.version, c.name)
		if got != c.want || (err == nil) != (c.want != "") {
			t.Errorf("ReleaseFileURL(%q, %q, %q) = %q, %v; want %q", c.manifest, c.version, c.name, got, err, c.want)
		}
	}
}
