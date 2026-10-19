// Package secrets encrypts outgoing credentials at rest (plan 3.8 point 3): TMDB keys and later
// other integration secrets are stored in the database as AES-256-GCM ciphertext. The key lives in
// <config>/secret.key (32 random bytes, mode 0600), never in the database, and is created on the
// first start.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// KeyFile is the name of the key file inside the config directory.
const KeyFile = "secret.key"

const (
	keySize = 32 // AES-256
	version = "v1:"
)

// Errors returned by Decrypt. Messages never contain the ciphertext or the plaintext.
var (
	ErrUnknownVersion = errors.New("secrets: unknown ciphertext version")
	ErrCorrupt        = errors.New("secrets: ciphertext corrupt, tampered with or encrypted with another key")
)

// Keyring encrypts and decrypts secrets with the key of the config directory.
type Keyring struct {
	aead cipher.AEAD
	key  []byte // master key, only used to derive purpose keys (Derive)
}

// Load reads <configDir>/secret.key or creates it with a fresh random key. An existing key file
// that group or others may read is used, but logged as a warning.
func Load(configDir string, log *slog.Logger) (*Keyring, error) {
	path := filepath.Join(configDir, KeyFile)
	key, err := os.ReadFile(path) //nolint:gosec // G304: fixed file name inside the operator-configured config directory
	switch {
	case errors.Is(err, fs.ErrNotExist):
		key, err = create(configDir, path)
		if err != nil {
			return nil, err
		}
		log.Info("secret key created", "file", path)
	case err != nil:
		return nil, fmt.Errorf("read secret key %s: %w", path, err)
	default:
		if fi, statErr := os.Stat(path); statErr == nil && fi.Mode().Perm()&0o077 != 0 {
			log.Warn("secret key file is readable by group or others; restrict it to 0600",
				"file", path, "mode", fi.Mode().Perm().String())
		}
	}
	if len(key) != keySize {
		return nil, fmt.Errorf("secret key %s has %d bytes, want %d", path, len(key), keySize)
	}
	return newKeyring(key)
}

func newKeyring(key []byte) (*Keyring, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("secrets: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secrets: %w", err)
	}
	return &Keyring{aead: aead, key: key}, nil
}

// create writes a new random key atomically: temp file with mode 0600 in the same directory,
// sync, rename.
func create(dir, path string) (key []byte, err error) {
	key = make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate secret key: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".secret.key-*")
	if err != nil {
		return nil, fmt.Errorf("create secret key: %w", err)
	}
	defer func() {
		if err != nil {
			_ = os.Remove(tmp.Name())
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return nil, errors.Join(fmt.Errorf("create secret key: %w", err), tmp.Close())
	}
	if _, err := tmp.Write(key); err != nil {
		return nil, errors.Join(fmt.Errorf("write secret key: %w", err), tmp.Close())
	}
	if err := tmp.Sync(); err != nil {
		return nil, errors.Join(fmt.Errorf("write secret key: %w", err), tmp.Close())
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("write secret key: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return nil, fmt.Errorf("store secret key: %w", err)
	}
	return key, nil
}

// Encrypt returns "v1:" + base64url(nonce || ciphertext).
func (k *Keyring) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, k.aead.NonceSize(), k.aead.NonceSize()+len(plaintext)+k.aead.Overhead())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("secrets: nonce: %w", err)
	}
	sealed := k.aead.Seal(nonce, nonce, []byte(plaintext), []byte(version))
	return version + base64.RawURLEncoding.EncodeToString(sealed), nil
}

// Decrypt reverses Encrypt. Other versions, tampered data and data from another key fail.
func (k *Keyring) Decrypt(ciphertext string) (string, error) {
	body, ok := strings.CutPrefix(ciphertext, version)
	if !ok {
		return "", ErrUnknownVersion
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil || len(raw) < k.aead.NonceSize()+k.aead.Overhead() {
		return "", ErrCorrupt
	}
	n := k.aead.NonceSize()
	plain, err := k.aead.Open(nil, raw[:n], raw[n:], []byte(version))
	if err != nil {
		return "", ErrCorrupt
	}
	return string(plain), nil
}

// Derive returns a 32-byte key for one purpose, for example "stream-token" (HMAC key of the stream
// tokens, plan 5.4). It is HKDF-SHA256 over the master key with the label as info, so purpose keys
// are independent of each other and of the encryption key, and the same label always yields the
// same key for the same key file. The result lives in memory only.
func (k *Keyring) Derive(label string) []byte {
	out, err := hkdf.Key(sha256.New, k.key, nil, "slouchcat/"+label, keySize)
	if err != nil {
		panic(err) // only fails for lengths above 255 * 32 bytes
	}
	return out
}
