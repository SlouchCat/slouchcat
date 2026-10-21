package logging

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"testing"
)

// fakeSecrets must never appear in log output.
var fakeSecrets = []string{
	"hunter2-FAKE", "tok_FAKE_123", "pin-FAKE-0815", "apikey-FAKE-xyz", "Bearer FAKE.jwt.value",
	"xapikey-FAKE", "s3cr3t-FAKE", "query-FAKE-key", "st-FAKE-token", "err-FAKE-key", "with-FAKE",
	"group-FAKE-pw", "url-FAKE-key", "msg-FAKE-key", "4711", "pl-FAKE-media", "pl-FAKE-init", "pl-FAKE-seg",
	"yt-FAKE-key", "ytq-FAKE-key", "tvdb-FAKE-key", "tvdbpin-FAKE", "apikeyq-FAKE",
	"dt-FAKE-token", "dtsub-FAKE-token", "dterr-FAKE-token", "verifier-FAKE-hash",
	"auth-FAKE-feed", "token-FAKE-feed", "access-FAKE-feed", "secret-FAKE-feed", "sig-FAKE-enc", "signature-FAKE-enc",
}

func logAll(t *testing.T, format Format) string {
	t.Helper()
	var buf bytes.Buffer
	log, err := New(&buf, format, slog.LevelDebug)
	if err != nil {
		t.Fatal(err)
	}
	log = log.With("token", "with-FAKE", "component", "test")

	log.Info("top-level keys",
		"password", "hunter2-FAKE",
		"Token", "tok_FAKE_123",
		"pin", "pin-FAKE-0815",
		slog.Group("profile", slog.Int("PIN", 4711)), // numeric secret, listed in fakeSecrets
		"apiKey", "apikey-FAKE-xyz",
		"Authorization", "Bearer FAKE.jwt.value",
		"X-Api-Key", "xapikey-FAKE",
		"client_secret", "s3cr3t-FAKE",
		"youtube_api_key", "yt-FAKE-key",
		"tvdbApiKey", "tvdb-FAKE-key",
		"TVDB_PIN", "tvdbpin-FAKE",
		"offline_pin_verifier", "verifier-FAKE-hash",
		"user", "marco",
	)
	log.Info("nested group",
		slog.Group("request",
			slog.String("method", "GET"),
			slog.Group("auth", slog.String("Password", "group-FAKE-pw"), slog.Int("attempt", 2)),
		),
	)
	log.WithGroup("tmdb").Info("url strings",
		"url", "https://api.themoviedb.org/3/movie/1?language=de&api_key=query-FAKE-key",
		"stream", "/api/v1/items/1/hls/main.m3u8?st=st-FAKE-token&segment=3",
		"parsed", &url.URL{Scheme: "https", Host: "example.org", RawQuery: "api_key=url-FAKE-key"},
		"err", fmt.Errorf("fetch: %w", errors.New(`Get "https://x.test/?api_key=err-FAKE-key": timeout`)),
		"first", "first=kept",
		"youtube", "https://www.googleapis.com/youtube/v3/search?part=snippet&q=x&key=ytq-FAKE-key&maxResults=5",
		"other", "https://x.test/login?apikey=apikeyq-FAKE",
		// Download token of M5 (getDownloadFile, getDownloadSubtitle).
		"download", "/api/v1/downloads/01JZZZZZZZZZZZZZZZZZZZZZZZ/file?dt=dt-FAKE-token",
		"subtitle", "/api/v1/downloads/01JZZZZZZZZZZZZZZZZZZZZZZZ/subtitles/3.vtt?lang=de&dt=dtsub-FAKE-token",
		"fetch", fmt.Errorf("fetch: %w", errors.New(`Get "http://x.test/api/v1/downloads/1/file?DT=dterr-FAKE-token": EOF`)),
		// An HLS playlist quoted in an error: every line carries the stream token (plan 5.4).
		// Private podcast feeds and their enclosures (M6).
		"feed", "https://feeds.example.org/show.rss?auth=auth-FAKE-feed&format=mp3",
		"feeds", "https://x.test/rss?Token=token-FAKE-feed;access_token=access-FAKE-feed&secret=secret-FAKE-feed",
		"enclosure", fmt.Errorf("fetch: %w", errors.New(`Get "https://cdn.test/ep.mp3?sig=sig-FAKE-enc&Signature=signature-FAKE-enc&expires=99": EOF`)),
		"playlist", fmt.Errorf("parse playlist: %w", errors.New("#EXTM3U\n#EXT-X-MAP:URI=\"init.mp4?st=pl-FAKE-init\"\nmedia.m3u8?st=pl-FAKE-media\nseg/0?st=pl-FAKE-seg\n")),
	)
	log.Warn("message with https://x.test/?api_key=msg-FAKE-key inside")
	return buf.String()
}

func TestRedaction(t *testing.T) {
	for _, format := range []Format{FormatJSON, FormatText} {
		t.Run(string(format), func(t *testing.T) {
			out := logAll(t, format)
			t.Log("\n" + out)
			for _, secret := range fakeSecrets {
				if strings.Contains(out, secret) {
					t.Errorf("output contains secret %q", secret)
				}
			}
			for _, keep := range []string{
				"marco", "GET", "attempt", "language=de", "segment=3", "first=kept", "component",
				"api_key=" + Redacted, "st=" + Redacted, "api.themoviedb.org", "example.org",
				"key=" + Redacted, "apikey=" + Redacted, "maxResults=5", "q=x",
				"dt=" + Redacted, "lang=de", "/subtitles/3.vtt",
				"auth=" + Redacted, "format=mp3", "access_token=" + Redacted, "sig=" + Redacted, "expires=99",
			} {
				if !strings.Contains(out, keep) {
					t.Errorf("output lost non-secret %q", keep)
				}
			}
		})
	}
}

func TestIsSensitiveKey(t *testing.T) {
	for key, want := range map[string]bool{
		"password": true, "PIN": true, "offline_pin": true, "access_token": true, "api_key": true, "youtube_api_key": true, "tvdbApiKey": true, "TVDB_PIN": true,
		"apiKey": true, "X-Api-Key": true, "authorization": true, "secret": true,
		"pin_hash": true, "profile_pin": true, "pinVerifier": true, "offline_pin_verifier": true, "invitation_token": true, "token_hash": true, "password_hash": true, "device_id": false, "content_hash": false,
		"user": false, "path": false, "max_tokens": false, "mapping": false, "status": false,
	} {
		if got := isSensitiveKey(key); got != want {
			t.Errorf("isSensitiveKey(%q) = %v, want %v", key, got, want)
		}
	}
}

func TestNewRejectsUnknownFormat(t *testing.T) {
	if _, err := New(&bytes.Buffer{}, "xml", slog.LevelInfo); err == nil {
		t.Fatal("expected an error for format xml")
	}
}
