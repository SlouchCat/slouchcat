package playback

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strings"
)

// Stream tokens (plan 5.4). A token is
//
//	st = <playback session id> "." base64url(HMAC-SHA256(key, id + "\n" + item id))
//
// It carries no expiry, because Safari keeps playlist URLs for the whole playback and a pause
// over night must not break it; expiry and revocation live in the playback_sessions row, which
// every token request reads by its primary key. The key is derived from the secret key file
// (secrets.Keyring.Derive("stream-token")) and kept in memory only.

const (
	idLength  = 26 // ULID
	macLength = sha256.Size
)

// SignToken returns the stream token of a playback session for an item.
func SignToken(key []byte, id, itemID string) string {
	return id + "." + base64.RawURLEncoding.EncodeToString(tokenMAC(key, id, itemID))
}

func tokenMAC(key []byte, id, itemID string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(id + "\n" + itemID))
	return m.Sum(nil)
}

// parseToken splits a token into the playback session id and the MAC; ok is false for anything
// that cannot be a token.
func parseToken(st string) (id string, mac []byte, ok bool) {
	id, enc, found := strings.Cut(st, ".")
	if !found || len(id) != idLength {
		return "", nil, false
	}
	mac, err := base64.RawURLEncoding.DecodeString(enc)
	if err != nil || len(mac) != macLength {
		return "", nil, false
	}
	return id, mac, true
}
