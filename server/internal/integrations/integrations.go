// Package integrations receives the webhooks of Radarr and Sonarr (plan M9, decision 5.6, spec
// I-8) and turns them into a targeted scan of one directory, so a new or changed file appears
// without waiting for the next full comparison (AN-12).
//
// The connection is incoming only: Slouchcat never calls Radarr or Sonarr and registers nothing
// there (controlling the Arr services is a non-goal). The administration creates an integration,
// Slouchcat answers once with the full address POST {BasePath}/webhooks/{token}, and the
// administration pastes it into "Connect → Webhook". Only the SHA-256 of the token is stored
// (3.8 point 2), so the address can never be shown again; renewing replaces it.
//
// A received payload names a directory (movie.folderPath for Radarr, series.path for Sonarr).
// The integration's path mapping translates the prefix the Arr service sees into the prefix this
// server sees (longest match wins), and package paths resolves the result against the registered
// library roots. The effect is a scan.path job of package library — the same job the inotify
// watcher queues (3.4), with the same coalescing: one trigger more, never a second scan path.
//
// This package never touches items, media_files or tracks (CLAUDE.md rule 2): it reads the
// library roots and writes its own two tables through store/appq.
package integrations

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// tokenBytes is the size of a webhook token: 256 bits (3.8 point 2), 43 characters in base64url.
const tokenBytes = 32

// TokenChars is the length of a token in its textual form; the route rejects anything else
// before a database lookup happens.
const TokenChars = 43

// MaxEvents is the number of log entries an integration keeps (5.6); a trigger of migration
// 00017 enforces it.
const MaxEvents = 20

// PathScanner queues a targeted scan of directories of a library root. Package library
// implements it; the webhook joins the coalescing of the watcher (3.4).
type PathScanner interface {
	RequestPathScan(ctx context.Context, libraryID, rootID int64, dirs []string)
}

// Service implements the administration of the integrations and the receiver of their webhooks.
type Service struct {
	write *appq.Queries
	read  *appq.Queries
	scan  PathScanner
	log   *slog.Logger
	now   func() time.Time

	// baseURL is the public address of the server (setting public_url) without a trailing
	// slash; it prefixes the address shown once after creating or renewing a token. Empty
	// leaves the address relative.
	baseURL func(ctx context.Context) string

	limits *limiters
}

// Option configures New.
type Option func(*Service)

// WithClock replaces time.Now (tests).
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// WithBaseURL sets the lookup of the public address that prefixes the webhook address.
func WithBaseURL(fn func(ctx context.Context) string) Option {
	return func(s *Service) { s.baseURL = fn }
}

// New returns the service. scan may be nil in tests that only administer integrations.
func New(db *store.DB, scan PathScanner, log *slog.Logger, opts ...Option) *Service {
	s := &Service{
		write: appq.New(db.Write), read: appq.New(db.Read),
		scan: scan, log: log, now: time.Now, limits: newLimiters(),
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// List returns every integration, oldest first, without tokens.
func (s *Service) List(ctx context.Context) ([]model.Integration, error) {
	rows, err := s.read.ListWebhookIntegrations(ctx)
	if err != nil {
		return nil, fmt.Errorf("list integrations: %w", err)
	}
	out := make([]model.Integration, 0, len(rows))
	for _, r := range rows {
		m, err := toModel(r)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

// Get returns one integration without its token.
func (s *Service) Get(ctx context.Context, id int64) (model.Integration, error) {
	row, err := s.read.GetWebhookIntegration(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Integration{}, model.ErrNotFound
	}
	if err != nil {
		return model.Integration{}, fmt.Errorf("get integration: %w", err)
	}
	return toModel(row)
}

// Create adds an integration and returns it with the token in clear and the full address; both
// are gone after this answer (3.8 point 2).
func (s *Service) Create(ctx context.Context, name, kind string) (model.Integration, error) {
	name = strings.TrimSpace(name)
	var verr *model.ValidationError
	if name == "" || len([]rune(name)) > 80 {
		verr = verr.Add("name", "required")
	}
	if kind != model.IntegrationRadarr && kind != model.IntegrationSonarr {
		verr = verr.Add("kind", "invalid")
	}
	if verr != nil {
		return model.Integration{}, verr
	}
	token, hash, tail, err := newToken()
	if err != nil {
		return model.Integration{}, err
	}
	now := s.now()
	row, err := s.write.InsertWebhookIntegration(ctx, appq.InsertWebhookIntegrationParams{
		Name: name, Kind: kind, TokenHash: hash, TokenTail: tail, PathMap: "[]", Now: now.UnixMilli(),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return model.Integration{}, (&model.ValidationError{}).Add("name", "taken")
		}
		return model.Integration{}, fmt.Errorf("insert integration: %w", err)
	}
	m, err := toModel(row)
	if err != nil {
		return model.Integration{}, err
	}
	s.reveal(ctx, &m, token)
	return m, nil
}

// RenewToken replaces the token of an integration and reveals the new address once. The old
// address stops working immediately.
func (s *Service) RenewToken(ctx context.Context, id int64) (model.Integration, error) {
	token, hash, tail, err := newToken()
	if err != nil {
		return model.Integration{}, err
	}
	row, err := s.write.UpdateWebhookIntegrationToken(ctx, appq.UpdateWebhookIntegrationTokenParams{
		ID: id, TokenHash: hash, TokenTail: tail, Now: s.now().UnixMilli(),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return model.Integration{}, model.ErrNotFound
	}
	if err != nil {
		return model.Integration{}, fmt.Errorf("renew integration token: %w", err)
	}
	m, err := toModel(row)
	if err != nil {
		return model.Integration{}, err
	}
	s.reveal(ctx, &m, token)
	return m, nil
}

// SetPathMap replaces the path mapping of an integration. Every pair needs two absolute paths;
// duplicate sources are rejected, because the longest match must stay unambiguous.
func (s *Service) SetPathMap(ctx context.Context, id int64, pairs []model.PathMapping) (model.Integration, error) {
	clean, verr := validateMapping(pairs)
	if verr != nil {
		return model.Integration{}, verr
	}
	encoded, err := json.Marshal(toStored(clean))
	if err != nil {
		return model.Integration{}, fmt.Errorf("encode path map: %w", err)
	}
	row, err := s.write.UpdateWebhookIntegrationPathMap(ctx, appq.UpdateWebhookIntegrationPathMapParams{
		ID: id, PathMap: string(encoded), Now: s.now().UnixMilli(),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return model.Integration{}, model.ErrNotFound
	}
	if err != nil {
		return model.Integration{}, fmt.Errorf("update path map: %w", err)
	}
	return toModel(row)
}

// Delete removes an integration and its event log.
func (s *Service) Delete(ctx context.Context, id int64) error {
	if _, err := s.read.GetWebhookIntegration(ctx, id); errors.Is(err, sql.ErrNoRows) {
		return model.ErrNotFound
	} else if err != nil {
		return fmt.Errorf("get integration: %w", err)
	}
	if err := s.write.DeleteWebhookIntegration(ctx, id); err != nil {
		return fmt.Errorf("delete integration: %w", err)
	}
	s.limits.drop(id)
	return nil
}

// Events returns the log of an integration, newest first.
func (s *Service) Events(ctx context.Context, id int64) ([]model.IntegrationEvent, error) {
	if _, err := s.read.GetWebhookIntegration(ctx, id); errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("get integration: %w", err)
	}
	rows, err := s.read.ListWebhookEvents(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list webhook events: %w", err)
	}
	out := make([]model.IntegrationEvent, 0, len(rows))
	for _, r := range rows {
		out = append(out, model.IntegrationEvent{
			ID: r.ID, ReceivedAt: time.UnixMilli(r.ReceivedAt).UTC(), Event: r.Event,
			ReportedPath: r.ReportedPath.String, ResolvedPath: r.ResolvedPath.String, Result: r.Result,
		})
	}
	return out, nil
}

// reveal fills the clear token and the full address of a just created or renewed integration.
func (s *Service) reveal(ctx context.Context, m *model.Integration, token string) {
	m.Token = token
	base := ""
	if s.baseURL != nil {
		base = strings.TrimRight(s.baseURL(ctx), "/")
	}
	m.URL = base + Path(token)
}

// Path returns the route of a token below the API base path.
func Path(token string) string { return "/webhooks/" + token }

// newToken returns a fresh token, its SHA-256 and its last four characters.
func newToken() (token, hash, tail string, err error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", "", "", fmt.Errorf("webhook token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, HashToken(token), token[len(token)-4:], nil
}

// HashToken returns the stored form of a token.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// storedMapping is one prefix pair as it is stored in the column path_map.
type storedMapping struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func toStored(pairs []model.PathMapping) []storedMapping {
	out := make([]storedMapping, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, storedMapping{From: p.From, To: p.To})
	}
	return out
}

func toModel(r appq.WebhookIntegration) (model.Integration, error) {
	var stored []storedMapping
	if err := json.Unmarshal([]byte(r.PathMap), &stored); err != nil {
		return model.Integration{}, fmt.Errorf("decode path map of integration %d: %w", r.ID, err)
	}
	m := model.Integration{
		ID: r.ID, Name: r.Name, Kind: r.Kind, TokenTail: r.TokenTail,
		PathMap: make([]model.PathMapping, 0, len(stored)), CreatedAt: time.UnixMilli(r.CreatedAt).UTC(),
	}
	for _, p := range stored {
		m.PathMap = append(m.PathMap, model.PathMapping{From: p.From, To: p.To})
	}
	if r.LastSeenAt.Valid {
		m.LastSeen = time.UnixMilli(r.LastSeenAt.Int64).UTC()
	}
	return m, nil
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// Authenticate finds the integration a token belongs to. The token is hashed before the lookup,
// so the stored form never leaves the database (3.8 point 2); it is never logged (point 8).
func (s *Service) Authenticate(ctx context.Context, token string) (model.Integration, error) {
	if len(token) != TokenChars {
		return model.Integration{}, model.ErrWebhookToken
	}
	row, err := s.read.GetWebhookIntegrationByTokenHash(ctx, HashToken(token))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Integration{}, model.ErrWebhookToken
	}
	if err != nil {
		return model.Integration{}, fmt.Errorf("find integration by token: %w", err)
	}
	return toModel(row)
}

// EventsSince counts the webhooks of every integration received since a point in time; the
// dashboard tile shows the last 24 hours (5.6).
func (s *Service) EventsSince(ctx context.Context, since time.Time) (int, error) {
	n, err := s.read.CountWebhookEventsSince(ctx, since.UnixMilli())
	if err != nil {
		return 0, fmt.Errorf("count webhook events: %w", err)
	}
	return int(n), nil
}
