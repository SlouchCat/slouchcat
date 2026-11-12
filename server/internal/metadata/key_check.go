package metadata

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/logging"
	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/trakt"
)

// The check of a stored provider key (checkProviderKey, concept 20 P-12): one cheap request per
// provider with the key the service uses, through the existing clients. Neither the answer nor an
// error carries the key; the message is the provider's error, scrubbed of it.

// Providers of CheckProviderKey (the contract's enum).
const (
	KeyCheckTMDB        = "tmdb"
	KeyCheckTVDB        = "tvdb"
	KeyCheckFanart      = "fanart"
	KeyCheckYouTube     = "youtube"
	KeyCheckTrakt       = "trakt"
	KeyCheckGoogleBooks = "googlebooks"
)

// The cheap requests: a film Fanart.tv knows, a public video (one quota unit), an ISBN.
const (
	keyCheckFanartPath = "/movies/603"
	keyCheckYouTubeID  = "jNQXAC9IVRw"
	keyCheckISBN       = "9780140449136"
	keyCheckMaxMessage = 300
)

// errFanartKeyRefused: Fanart.tv answered 401 or 403 to the key.
var errFanartKeyRefused = errors.New("fanart: key refused")

// CheckProviderKey asks the provider with the configured key whether it holds (model.KeyCheck);
// an unknown provider is a validation error. A provider without key is ok=false, not_configured.
func (s *Service) CheckProviderKey(ctx context.Context, provider string) (model.KeyCheck, error) {
	var secrets []string
	var err error
	switch provider {
	case KeyCheckTMDB:
		var key string
		if key, err = s.keys.TMDBKey(ctx); err == nil {
			secrets = []string{key}
			err = s.checkWith(key, func() error { return s.client.CheckKey(ctx, key) })
		}
	case KeyCheckTVDB:
		var key, pin string
		if key, pin, err = s.keys.TVDBKey(ctx); err == nil {
			secrets = []string{key, pin}
			err = s.checkWith(key, func() error {
				_, err := s.tvdb.bearer(ctx, key, pin, true)
				return err
			})
		}
	case KeyCheckFanart:
		var key, client string
		if key, client, err = s.keys.FanartKeys(ctx); err == nil {
			secrets = []string{key, client}
			err = s.checkWith(key, func() error {
				var raw map[string]any
				if err := s.fanart.getJSON(ctx, keyCheckFanartPath, &raw); !errors.Is(err, ErrNotFound) {
					return err
				}
				return nil // the key was taken; the film is simply unknown
			})
		}
	case KeyCheckYouTube:
		var key string
		if key, err = s.keys.YouTubeKey(ctx); err == nil {
			secrets = []string{key}
			err = s.checkWith(key, func() error { return s.checkYouTube(ctx, key) })
		}
	case KeyCheckTrakt:
		var id string
		if id, err = s.traktClientID(ctx); err == nil {
			secrets = []string{id}
			err = s.checkWith(id, func() error {
				c := trakt.New(id, trakt.WithBaseURL(s.trakt.base), trakt.WithHTTPClient(s.trakt.http),
					trakt.WithLimiter(s.trakt.limiter), trakt.WithUserAgent(s.trakt.agent))
				_, err := c.Fetch(ctx, trakt.ListRef{Kind: trakt.KindTrending, Media: trakt.MediaMovie}, 1)
				return err
			})
		}
	case KeyCheckGoogleBooks:
		var key string
		if key, err = s.books.google.apiKey(ctx); err == nil {
			secrets = []string{key}
			err = s.checkWith(key, func() error { return s.checkGoogleBooks(ctx, key) })
		}
	default:
		return model.KeyCheck{}, (&model.ValidationError{}).Add("provider", "invalid_value")
	}
	return keyCheckResult(ctx, err, secrets)
}

// checkWith runs check unless key is empty (errNotConfigured).
func (s *Service) checkWith(key string, check func() error) error {
	if strings.TrimSpace(key) == "" {
		return errNotConfigured
	}
	return check()
}

// checkYouTube reads one public video: one unit of the daily quota instead of a search's 100.
func (s *Service) checkYouTube(ctx context.Context, key string) error {
	y, ok := s.trailersYouTube.(*youtubeTrailers)
	if !ok {
		return errors.New("youtube: no client")
	}
	if !y.quota.Take(ctx, 1) {
		return errQuotaExhausted
	}
	var out struct{}
	return y.getJSON(ctx, "/videos", url.Values{"part": {"id"}, "id": {keyCheckYouTubeID}, "key": {key}}, &out)
}

// checkGoogleBooks searches one ISBN past the search cache; 400, 401 and 403 refuse the key.
func (s *Service) checkGoogleBooks(ctx context.Context, key string) error {
	p := s.books.google
	_, status, err := p.fetch(ctx, p.http, p.limiter,
		queryURL(p.base, "/volumes", url.Values{"q": {"isbn:" + keyCheckISBN}, "maxResults": {"1"}, "key": {key}}), bookJSON)
	if status == http.StatusBadRequest || status == http.StatusUnauthorized || status == http.StatusForbidden {
		return fmt.Errorf("%w: googlebooks: status %d", model.ErrTMDBKeyInvalid, status)
	}
	return err
}

// keyCheckResult maps the outcome of a check; a database or context error stays an error.
func keyCheckResult(ctx context.Context, err error, secrets []string) (model.KeyCheck, error) {
	switch {
	case err == nil:
		return model.KeyCheck{OK: true}, nil
	case ctx.Err() != nil:
		return model.KeyCheck{}, ctx.Err()
	case errors.Is(err, errNotConfigured):
		return model.KeyCheck{Problem: model.KeyCheckNotConfigured}, nil
	}
	check := model.KeyCheck{Problem: model.KeyCheckFailed, Message: keyCheckMessage(err, secrets)}
	switch {
	case errors.Is(err, model.ErrTMDBKeyInvalid), errors.Is(err, errYouTubeKeyInvalid), errors.Is(err, errFanartKeyRefused),
		errors.Is(err, trakt.ErrUnauthorized):
		check.Problem = model.KeyCheckInvalid
	case errors.Is(err, model.ErrTMDBUnreachable), errors.Is(err, trakt.ErrUnavailable), errors.Is(err, errQuotaExhausted), errors.Is(err, errYouTubeUnavailable),
		errors.Is(err, errSourceBlocked):
		check.Problem = model.KeyCheckUnreachable
	}
	return check, nil
}

// keyCheckMessage is the error in plain text without any secret: every key is cut out, then the
// redaction of the log runs over it, and it is cut to a short line.
func keyCheckMessage(err error, secrets []string) string {
	msg := err.Error()
	for _, v := range secrets {
		if v = strings.TrimSpace(v); v != "" {
			msg = strings.ReplaceAll(msg, v, logging.Redacted)
		}
	}
	msg = logging.RedactString(msg)
	if r := []rune(msg); len(r) > keyCheckMaxMessage {
		msg = string(r[:keyCheckMaxMessage]) + "…"
	}
	return msg
}
