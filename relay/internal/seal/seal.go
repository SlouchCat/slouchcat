// Package seal turns the Google token of a phone and the VAPID key of its server into the sealed
// part of a push address, and back (concept 27 section 7.3, rule D-6; "Protokolle" of the build
// log). The push forwarder needs no database: everything it must know to deliver travels inside
// the address, sealed with a key only it holds.
//
// Address: https://<name>/v1/push/<version>.<blob>
// Blob:    base64url without padding of nonce (12 bytes) || AES-256-GCM ciphertext and tag of the
//
//	JSON {"t": <Google token>, "k": <VAPID key, base64url>}
//
// The additional data is "slouchcat-push-v1:" + version, so a blob does not open under another
// version's name. Versions let the sealing key change without invalidating every address at once:
// the file holds every version still in use, and the current one seals.
//
// Key file (JSON, outside the image, mounted read-only):
//
//	{"current": "2", "keys": {"1": "<32 bytes base64url>", "2": "<32 bytes base64url>"}}
package seal

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Errors of Open. Every one of them is the forwarder's own failure and ends in 503, never in 410
// (D-10): an address the forwarder cannot open may still be good at an instance that has the key.
var (
	ErrUnknownVersion = errors.New("unknown sealing key version")
	ErrMalformed      = errors.New("malformed push address")
	ErrOpen           = errors.New("push address does not open with this key")
	ErrNoKey          = errors.New("no sealing key loaded")
)

const aadPrefix = "slouchcat-push-v1:"

var versionRE = regexp.MustCompile(`^[a-z0-9]{1,16}$`)

// Content is what a push address carries.
type Content struct {
	Token    string `json:"t"`
	VAPIDKey string `json:"k"`
}

// File is the key file.
type File struct {
	Current string            `json:"current"`
	Keys    map[string]string `json:"keys"`
}

// Keys holds the parsed keys.
type Keys struct {
	current string
	aeads   map[string]cipher.AEAD
}

// Load reads the key file at path.
func Load(path string) (*Keys, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the operator's configured key file
	if err != nil {
		return nil, err
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("sealing key file: %w", err)
	}
	return Parse(f)
}

// Parse checks f and builds the ciphers.
func Parse(f File) (*Keys, error) {
	k := &Keys{current: f.Current, aeads: map[string]cipher.AEAD{}}
	for v, s := range f.Keys {
		if !versionRE.MatchString(v) {
			return nil, fmt.Errorf("sealing key version %q: want 1 to 16 of a-z and 0-9", v)
		}
		raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
		if err != nil || len(raw) != 32 {
			return nil, fmt.Errorf("sealing key version %q: want 32 bytes in base64url", v)
		}
		block, err := aes.NewCipher(raw)
		if err != nil {
			return nil, err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}
		k.aeads[v] = gcm
	}
	if _, ok := k.aeads[f.Current]; !ok {
		return nil, fmt.Errorf("sealing key file: current version %q has no key", f.Current)
	}
	return k, nil
}

// Versions is the number of versions loaded.
func (k *Keys) Versions() int {
	if k == nil {
		return 0
	}
	return len(k.aeads)
}

// Current is the version that seals.
func (k *Keys) Current() string { return k.current }

// Seal returns "<version>.<blob>" for c.
func (k *Keys) Seal(c Content) (string, error) {
	if k == nil {
		return "", ErrNoKey
	}
	plain, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	gcm := k.aeads[k.current]
	nonce := make([]byte, gcm.NonceSize())
	_, _ = rand.Read(nonce)
	out := gcm.Seal(nonce, nonce, plain, []byte(aadPrefix+k.current))
	return k.current + "." + base64.RawURLEncoding.EncodeToString(out), nil
}

// Open reverses Seal.
func (k *Keys) Open(sealed string) (Content, error) {
	var c Content
	if k == nil {
		return c, ErrNoKey
	}
	version, blob, ok := strings.Cut(sealed, ".")
	if !ok || !versionRE.MatchString(version) {
		return c, ErrMalformed
	}
	gcm, ok := k.aeads[version]
	if !ok {
		return c, ErrUnknownVersion
	}
	raw, err := base64.RawURLEncoding.DecodeString(blob)
	if err != nil || len(raw) < gcm.NonceSize()+gcm.Overhead() {
		return c, ErrMalformed
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], []byte(aadPrefix+version))
	if err != nil {
		return c, ErrOpen
	}
	if err := json.Unmarshal(plain, &c); err != nil || c.Token == "" || c.VAPIDKey == "" {
		return Content{}, ErrMalformed
	}
	return c, nil
}

// NewKey returns 32 random bytes in base64url, a new key for the file.
func NewKey() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// AddVersion adds a new key under version to the file at path (creating it when missing) and makes
// it current. Old versions stay, so addresses sealed with them keep working.
func AddVersion(path, version string) error {
	if !versionRE.MatchString(version) {
		return fmt.Errorf("version %q: want 1 to 16 of a-z and 0-9", version)
	}
	f := File{Keys: map[string]string{}}
	data, err := os.ReadFile(path) //nolint:gosec // the operator's key file
	switch {
	case err == nil:
		if err := json.Unmarshal(data, &f); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if f.Keys == nil {
			f.Keys = map[string]string{}
		}
	case !errors.Is(err, os.ErrNotExist):
		return err
	}
	if _, ok := f.Keys[version]; ok {
		return fmt.Errorf("version %q exists already", version)
	}
	f.Keys[version] = NewKey()
	f.Current = version
	out, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".new"
	if err := os.WriteFile(tmp, append(out, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
