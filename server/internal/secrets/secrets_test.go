package secrets

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func load(t *testing.T, dir string) *Keyring {
	t.Helper()
	k, err := Load(dir, discard())
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestRoundTrip(t *testing.T) {
	k := load(t, t.TempDir())
	for _, plain := range []string{"", "eyJhbGciOiJIUzI1NiJ9.fake-token", strings.Repeat("ä", 1000)} {
		ct, err := k.Encrypt(plain)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(ct, "v1:") {
			t.Fatalf("ciphertext %q lacks the version prefix", ct)
		}
		if plain != "" && strings.Contains(ct, plain) {
			t.Fatal("ciphertext contains the plaintext")
		}
		got, err := k.Decrypt(ct)
		if err != nil {
			t.Fatal(err)
		}
		if got != plain {
			t.Fatalf("round trip = %q, want %q", got, plain)
		}
	}
	a, _ := k.Encrypt("same")
	b, _ := k.Encrypt("same")
	if a == b {
		t.Fatal("two encryptions of the same plaintext must differ (random nonce)")
	}
}

func TestTamperingFails(t *testing.T) {
	k := load(t, t.TempDir())
	ct, err := k.Encrypt("FAKE-secret-value")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(ct, "v1:"))
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)-1] ^= 0x01
	tampered := "v1:" + base64.RawURLEncoding.EncodeToString(raw)
	cases := map[string]error{
		tampered:                      ErrCorrupt,
		"v1:" + "!!!":                 ErrCorrupt,
		"v1:" + "AAAA":                ErrCorrupt,
		"v2:" + ct[3:]:                ErrUnknownVersion,
		strings.TrimPrefix(ct, "v1:"): ErrUnknownVersion,
	}
	for in, want := range cases {
		_, err := k.Decrypt(in)
		if !errors.Is(err, want) {
			t.Errorf("Decrypt(%q) error = %v, want %v", in, err, want)
		}
		if err != nil && strings.Contains(err.Error(), "FAKE-secret-value") {
			t.Errorf("error leaks the plaintext: %v", err)
		}
	}
}

func TestWrongKeyFails(t *testing.T) {
	a := load(t, t.TempDir())
	b := load(t, t.TempDir())
	ct, err := a.Encrypt("FAKE-secret-value")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Decrypt(ct); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("decrypt with another key: %v, want ErrCorrupt", err)
	}
}

func TestKeyFileCreatedPrivateAndReused(t *testing.T) {
	dir := t.TempDir()
	k1 := load(t, dir)
	path := filepath.Join(dir, KeyFile)
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("key file mode = %v, want 0600", fi.Mode().Perm())
	}
	if fi.Size() != keySize {
		t.Fatalf("key file has %d bytes", fi.Size())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("temp files left behind: %v", entries)
	}
	ct, err := k1.Encrypt("persisted")
	if err != nil {
		t.Fatal(err)
	}
	k2 := load(t, dir)
	got, err := k2.Decrypt(ct)
	if err != nil || got != "persisted" {
		t.Fatalf("second Load does not reuse the key: %q, %v", got, err)
	}
}

func TestLoadWarnsOnOpenPermissions(t *testing.T) {
	dir := t.TempDir()
	load(t, dir)
	if err := os.Chmod(filepath.Join(dir, KeyFile), 0o644); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	if _, err := Load(dir, slog.New(slog.NewTextHandler(&logs, nil))); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs.String(), "level=WARN") {
		t.Fatalf("no warning for a world-readable key file: %s", logs.String())
	}
}

func TestLoadRejectsShortKey(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, KeyFile), []byte("short"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir, discard()); err == nil {
		t.Fatal("Load accepted a key file of the wrong size")
	}
}

func TestDeriveIsStablePerKeyAndLabel(t *testing.T) {
	dir := t.TempDir()
	a := load(t, dir)
	again := load(t, dir)
	other := load(t, t.TempDir())
	stream := a.Derive("stream-token")
	if len(stream) != keySize {
		t.Fatalf("derived key has %d bytes", len(stream))
	}
	if !bytes.Equal(stream, again.Derive("stream-token")) {
		t.Error("the same key file and label yield different keys")
	}
	if bytes.Equal(stream, a.Derive("other-purpose")) {
		t.Error("different labels yield the same key")
	}
	if bytes.Equal(stream, other.Derive("stream-token")) {
		t.Error("different key files yield the same key")
	}
	if bytes.Equal(stream, a.key) {
		t.Error("the derived key equals the master key")
	}
}
