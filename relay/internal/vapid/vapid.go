// Package vapid checks and makes the VAPID signature of a Web Push request (RFC 8292; concept 27
// rule D-4): "Authorization: vapid t=<JWT>, k=<public key>", the JWT signed with ES256 by the
// server's VAPID key.
//
// The push forwarder accepts a request only when k is the VAPID key sealed into the push address,
// the signature verifies with it, aud is one of the forwarder's own origins, and exp lies in the
// future but no more than 24 hours ahead (RFC 8292 section 2).
package vapid

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// MaxLifetime is the longest a token may still be valid (RFC 8292 section 2).
const MaxLifetime = 24 * time.Hour

// Errors of Verify. All of them are answered with 403.
var (
	ErrMissing   = errors.New("no VAPID authorization")
	ErrKey       = errors.New("VAPID key does not match the push address")
	ErrSignature = errors.New("VAPID signature does not verify")
	ErrAudience  = errors.New("VAPID token is not for this service")
	ErrExpired   = errors.New("VAPID token expired or too long-lived")
	ErrMalformed = errors.New("malformed VAPID token")
)

var b64 = base64.RawURLEncoding

type header struct {
	Typ string `json:"typ"`
	Alg string `json:"alg"`
}

// Claims of a VAPID token.
type Claims struct {
	Aud string `json:"aud"`
	Exp int64  `json:"exp"`
	Sub string `json:"sub,omitempty"`
}

// Verify checks the Authorization header against the sealed key (base64url of the uncompressed
// P-256 point), the own origins and the time now.
func Verify(authorization, sealedKey string, origins []string, now time.Time) error {
	scheme, rest, ok := strings.Cut(strings.TrimSpace(authorization), " ")
	if !ok || !strings.EqualFold(scheme, "vapid") {
		return ErrMissing
	}
	var token, key string
	for _, part := range strings.Split(rest, ",") {
		name, value, _ := strings.Cut(strings.TrimSpace(part), "=")
		switch strings.ToLower(name) {
		case "t":
			token = value
		case "k":
			key = value
		}
	}
	if token == "" || key == "" {
		return ErrMissing
	}
	// The key in the header must be the one the push address carries; only that server may send
	// to this phone (7.3). The comparison ignores padding, which some senders add.
	if subtle.ConstantTimeCompare([]byte(strings.TrimRight(key, "=")), []byte(strings.TrimRight(sealedKey, "="))) != 1 {
		return ErrKey
	}
	pub, err := ParsePublicKey(sealedKey)
	if err != nil {
		return ErrKey
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ErrMalformed
	}
	var h header
	if err := decodeJSON(parts[0], &h); err != nil || h.Alg != "ES256" {
		return ErrMalformed
	}
	sig, err := b64.DecodeString(parts[2])
	if err != nil || len(sig) != 64 {
		return ErrSignature
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	r, s := new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])
	if !ecdsa.Verify(pub, digest[:], r, s) {
		return ErrSignature
	}
	var c Claims
	if err := decodeJSON(parts[1], &c); err != nil {
		return ErrMalformed
	}
	audOK := false
	for _, o := range origins {
		if c.Aud == o {
			audOK = true
		}
	}
	if !audOK {
		return ErrAudience
	}
	exp := time.Unix(c.Exp, 0)
	if !now.Before(exp) || exp.Sub(now) > MaxLifetime+time.Minute {
		return ErrExpired
	}
	return nil
}

// ParsePublicKey reads a base64url uncompressed P-256 point (65 bytes, 0x04 || X || Y).
func ParsePublicKey(s string) (*ecdsa.PublicKey, error) {
	raw, err := b64.DecodeString(strings.TrimRight(s, "="))
	if err != nil {
		return nil, err
	}
	return ecdsa.ParseUncompressedPublicKey(elliptic.P256(), raw)
}

// PublicKeyString is the base64url uncompressed form of k, as a server publishes it.
func PublicKeyString(k *ecdsa.PublicKey) (string, error) {
	raw, err := k.Bytes()
	if err != nil {
		return "", err
	}
	return b64.EncodeToString(raw), nil
}

// Authorization signs claims with k and returns the header value (what a server does; pushcheck
// and the tests use it).
func Authorization(k *ecdsa.PrivateKey, c Claims) (string, error) {
	hb, _ := json.Marshal(header{Typ: "JWT", Alg: "ES256"})
	cb, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	signing := b64.EncodeToString(hb) + "." + b64.EncodeToString(cb)
	digest := sha256.Sum256([]byte(signing))
	r, s, err := ecdsa.Sign(rand.Reader, k, digest[:])
	if err != nil {
		return "", err
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	pub, err := PublicKeyString(&k.PublicKey)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("vapid t=%s.%s, k=%s", signing, b64.EncodeToString(sig), pub), nil
}

func decodeJSON(s string, v any) error {
	raw, err := b64.DecodeString(s)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}
