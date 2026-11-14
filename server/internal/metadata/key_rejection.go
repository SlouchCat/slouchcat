package metadata

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// A rejected provider key (concept 21 P-2). A fill that meets a key TMDB or TVDB rejects makes
// no further request with it: the provider counts as not configured for the fills
// (fetchTitleRecords, QueuePendingFills) until the key changes. The server remembers the
// rejection per provider and key in memory only; after a restart the first job asks again. A
// check of the new key that passes (CheckProviderKey, also on saving it) lifts the rejection and
// queues the fills that waited, once.

// keyRejectedError is model.ErrTMDBKeyInvalid with the name of the provider that rejected the
// key: the sentinel's own text names TMDB for every provider. The contract code stays
// tmdb_key_invalid (errors.Is matches the sentinel).
type keyRejectedError struct{ provider string }

func (e *keyRejectedError) Error() string { return e.provider + " key invalid" }
func (e *keyRejectedError) Unwrap() error { return model.ErrTMDBKeyInvalid }

// keyRejected returns the error of a key the provider rejected.
func keyRejected(provider string) error { return &keyRejectedError{provider: provider} }

// rejectedBy names the provider of a keyRejectedError in err ("" for none).
func rejectedBy(err error) string {
	var e *keyRejectedError
	if errors.As(err, &e) {
		return e.provider
	}
	return ""
}

// rejectedKeys holds per provider the hash of the key it rejected.
type rejectedKeys struct {
	mu sync.Mutex
	m  map[string][32]byte
}

// keyHash is the hash of the configured key of a provider whose rejection is remembered (TMDB,
// TVDB with its PIN); ok is false without key or for another provider.
func (s *Service) keyHash(ctx context.Context, provider string) (sum [32]byte, ok bool) {
	switch provider {
	case ProviderTMDB:
		key, err := s.keys.TMDBKey(ctx)
		if err != nil || key == "" {
			return sum, false
		}
		return sha256.Sum256([]byte(key)), true
	case ProviderTVDB:
		key, pin, err := s.keys.TVDBKey(ctx)
		if err != nil || key == "" {
			return sum, false
		}
		return sha256.Sum256([]byte(key + "\x00" + pin)), true
	}
	return sum, false
}

// rejectKey remembers that the provider rejected its configured key; the first time for that key
// it logs a warning.
func (s *Service) rejectKey(ctx context.Context, provider string) {
	sum, ok := s.keyHash(ctx, provider)
	if !ok {
		return
	}
	s.rejected.mu.Lock()
	prev, had := s.rejected.m[provider]
	if s.rejected.m == nil {
		s.rejected.m = map[string][32]byte{}
	}
	s.rejected.m[provider] = sum
	s.rejected.mu.Unlock()
	if !had || prev != sum {
		s.log.WarnContext(ctx, "metadata provider rejected its key, no more requests with it until a new key",
			"provider", provider)
	}
}

// KeyRejected reports whether the provider rejected the key configured now (TMDB, TVDB). A new
// key is not rejected until a provider says so.
func (s *Service) KeyRejected(ctx context.Context, provider string) bool {
	sum, ok := s.keyHash(ctx, provider)
	if !ok {
		return false
	}
	s.rejected.mu.Lock()
	defer s.rejected.mu.Unlock()
	prev, had := s.rejected.m[provider]
	return had && prev == sum
}

// acceptKey lifts a remembered rejection of the provider after its key passed a check and queues
// the fills of every library once (the titles that waited for the key).
func (s *Service) acceptKey(ctx context.Context, provider string) error {
	s.rejected.mu.Lock()
	_, had := s.rejected.m[provider]
	delete(s.rejected.m, provider)
	s.rejected.mu.Unlock()
	if !had {
		return nil
	}
	ids, err := s.read.ListLibraryIDs(ctx)
	if err != nil {
		return fmt.Errorf("list libraries: %w", err)
	}
	queued := 0
	for _, id := range ids {
		n, err := s.QueuePendingFills(ctx, id)
		queued += n
		if err != nil {
			return err
		}
	}
	s.log.InfoContext(ctx, "metadata provider takes its key again, fills queued", "provider", provider, "fills", queued)
	return nil
}

// SettingKeyRejectedFillsDeleted marks that the failed fills of a rejected key were removed once
// (concept 21 O-4).
const SettingKeyRejectedFillsDeleted = "metadata.key_rejected_fills_deleted"

// DeleteKeyRejectedFillsOnce removes, once at the first start after the update to concept 21,
// the failed metadata.fill runs a rejected provider key ended (on CT 190 4 899 runs, each retried
// five times; decision O-4 of 14 November 2026). Fills since then end without failure, so the
// setting remembers a complete run; a failed run starts again at the next start. It returns the
// number of runs removed.
func (s *Service) DeleteKeyRejectedFillsOnce(ctx context.Context) (int64, error) {
	if _, err := s.read.GetSystemSetting(ctx, SettingKeyRejectedFillsDeleted); err == nil {
		return 0, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("read %s: %w", SettingKeyRejectedFillsDeleted, err)
	}
	n, err := s.write.DeleteKeyRejectedFailures(ctx, JobFill)
	if err != nil {
		return 0, fmt.Errorf("delete the failed fills of a rejected key: %w", err)
	}
	if err := s.write.UpsertSystemSetting(ctx, systemq.UpsertSystemSettingParams{
		Key: SettingKeyRejectedFillsDeleted, Value: "1", Now: s.now().UnixMilli(),
	}); err != nil {
		return n, fmt.Errorf("write %s: %w", SettingKeyRejectedFillsDeleted, err)
	}
	if n > 0 {
		s.log.InfoContext(ctx, "failed fills of a rejected key removed", "runs", n)
	}
	return n, nil
}
