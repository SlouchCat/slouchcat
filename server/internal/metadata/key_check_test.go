package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/jobs"
	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/ratelimit"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/store/storetest"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// checkKeys is a KeySource for the key check; every field is a key, "" is none.
type checkKeys struct {
	tmdb, tvdb, pin, fanart, client, youtube, trakt, google string
}

func (k checkKeys) TMDBKey(context.Context) (string, error)         { return k.tmdb, nil }
func (k checkKeys) TVDBKey(context.Context) (string, string, error) { return k.tvdb, k.pin, nil }
func (k checkKeys) YouTubeKey(context.Context) (string, error)      { return k.youtube, nil }
func (k checkKeys) TraktClientID(context.Context) (string, error)   { return k.trakt, nil }
func (k checkKeys) FanartKeys(context.Context) (string, string, error) {
	return k.fanart, k.client, nil
}
func (k checkKeys) GoogleBooksKey(context.Context) (string, error) { return k.google, nil }

// keyCheckGood is the prefix of every key the fake providers accept.
const keyCheckGood = "good-"

// newKeyCheckProviders is one fake of every provider's cheap request: a key with keyCheckGood
// holds, any other is refused the way the provider refuses it; a key "down-…" answers 503.
func newKeyCheckProviders(t *testing.T) *httptest.Server {
	t.Helper()
	answer := func(w http.ResponseWriter, key string, refused int, body string) {
		switch {
		case strings.HasPrefix(key, keyCheckGood):
			_, _ = w.Write([]byte(body))
		case strings.HasPrefix(key, "down-"):
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			w.WriteHeader(refused)
			_, _ = w.Write([]byte(`{"error":{"errors":[{"reason":"keyInvalid"}],"message":"key ` + key + ` refused"}}`))
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /3/configuration", func(w http.ResponseWriter, r *http.Request) {
		answer(w, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), http.StatusUnauthorized, `{"images":{}}`)
	})
	mux.HandleFunc("POST /v4/login", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			APIKey string `json:"apikey"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		answer(w, body.APIKey, http.StatusUnauthorized, `{"status":"success","data":{"token":"t"}}`)
	})
	mux.HandleFunc("GET /fanart/movies/603", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.Header.Get("api-key"), keyCheckGood) {
			w.WriteHeader(http.StatusNotFound) // a key that holds, a film Fanart.tv does not know
			return
		}
		answer(w, r.Header.Get("api-key"), http.StatusUnauthorized, `{}`)
	})
	mux.HandleFunc("GET /yt/videos", func(w http.ResponseWriter, r *http.Request) {
		answer(w, r.URL.Query().Get("key"), http.StatusBadRequest, `{"items":[]}`)
	})
	mux.HandleFunc("GET /trakt/movies/trending", func(w http.ResponseWriter, r *http.Request) {
		answer(w, r.Header.Get("trakt-api-key"), http.StatusForbidden, `[]`)
	})
	mux.HandleFunc("GET /google/volumes", func(w http.ResponseWriter, r *http.Request) {
		answer(w, r.URL.Query().Get("key"), http.StatusBadRequest, `{"items":[]}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// Concept 20, P-12: one cheap request per provider with its configured key. A good key holds, a
// refused one is «invalid», a provider that is down «unreachable», none «not_configured»; neither
// the answer nor the log ever carries a key.
func TestCheckProviderKey(t *testing.T) {
	ctx := context.Background()
	srv := newKeyCheckProviders(t)
	logs := &bytes.Buffer{}
	log := slog.New(logging.NewRedactingHandler(slog.NewTextHandler(&lockedWriter{w: logs}, nil)))
	db, err := storetest.Open(ctx, store.Options{ConfigDir: t.TempDir(), Version: "test", Logger: log})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	service := func(keys checkKeys) *Service {
		return New(db, jobs.NewQueue(db.Write, nil), keys, t.TempDir(), transcode.Runner{}, log,
			WithBaseURL(srv.URL+"/3", srv.URL+"/t/p"), WithHTTPClient(srv.Client()),
			WithTVDBBaseURL(srv.URL+"/v4", srv.URL+"/art"), WithFanartBaseURL(srv.URL+"/fanart", srv.URL+"/assets"),
			WithYouTubeBaseURL(srv.URL+"/yt"),
			WithYouTubeQuota(ratelimit.NewQuota("youtube-test", youtubeDailyUnits, ratelimit.MidnightPacific)),
			WithTrakt("", WithTraktBaseURL(srv.URL+"/trakt"), WithTraktHTTPClient(srv.Client()),
				WithTraktLimiter(ratelimit.New("trakt-test", 1000, 100))),
			WithBookSourceURLs(BookSourceURLsFromBase(srv.URL)))
	}
	all := func(prefix string) checkKeys {
		return checkKeys{tmdb: prefix + "tmdb-1234", tvdb: prefix + "tvdb-1234", pin: "pin-9876", fanart: prefix + "fanart-1234",
			client: "client-5678", youtube: prefix + "youtube-1234", trakt: prefix + "trakt-1234", google: prefix + "google-1234"}
	}
	providers := []string{KeyCheckTMDB, KeyCheckTVDB, KeyCheckFanart, KeyCheckYouTube, KeyCheckTrakt, KeyCheckGoogleBooks}
	var secrets []string
	for _, c := range []struct {
		name    string
		keys    checkKeys
		problem string
	}{
		{"good keys", all(keyCheckGood), ""},
		{"refused keys", all("bad-"), model.KeyCheckInvalid},
		{"providers down", all("down-"), model.KeyCheckUnreachable},
		{"no keys", checkKeys{}, model.KeyCheckNotConfigured},
	} {
		k := c.keys
		secrets = append(secrets, k.tmdb, k.tvdb, k.pin, k.fanart, k.client, k.youtube, k.trakt, k.google)
		svc := service(k)
		for _, p := range providers {
			got, err := svc.CheckProviderKey(ctx, p)
			if err != nil {
				t.Fatalf("%s, %s: %v", c.name, p, err)
			}
			if got.OK != (c.problem == "") || got.Problem != c.problem {
				t.Errorf("%s, %s = %+v, want problem %q", c.name, p, got, c.problem)
			}
			if (c.problem == model.KeyCheckInvalid || c.problem == model.KeyCheckUnreachable) && got.Message == "" {
				t.Errorf("%s, %s: no message", c.name, p)
			}
			for _, s := range []string{k.tmdb, k.tvdb, k.pin, k.fanart, k.client, k.youtube, k.trakt, k.google} {
				if s != "" && strings.Contains(got.Message, s) {
					t.Errorf("%s, %s: message %q carries a key", c.name, p, got.Message)
				}
			}
		}
	}
	if _, err := service(all(keyCheckGood)).CheckProviderKey(ctx, "nobody"); err == nil {
		t.Error("an unknown provider was checked")
	}
	for _, s := range secrets {
		if s != "" && strings.Contains(logs.String(), s) {
			t.Errorf("key %q in the log", s)
		}
	}
}

// The message of a failed check never carries a key, even when a provider's error repeats it.
func TestKeyCheckMessageCutsTheKey(t *testing.T) {
	msg := keyCheckMessage(errors.New("tvdb /login: key sec-4242 refused, pin 7777"), []string{"sec-4242", "7777", ""})
	if strings.Contains(msg, "sec-4242") || strings.Contains(msg, "7777") || !strings.Contains(msg, logging.Redacted) {
		t.Errorf("message = %q", msg)
	}
	if long := keyCheckMessage(errors.New(strings.Repeat("x", 1000)), nil); len([]rune(long)) != keyCheckMaxMessage+1 {
		t.Errorf("long message has %d characters", len([]rune(long)))
	}
}
