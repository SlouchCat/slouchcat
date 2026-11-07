package radio

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
)

// Listen tokens (concept 16 section 7.5). A token is
//
//	rt = <listen session id> "." base64url(HMAC-SHA256(key, id + "\n" + station id))
//
// so it opens only the live routes of its own listen session and only the relay of its station.
// Expiry and revocation live in the listen session, not in the token. The key is derived from the
// secret key file (secrets.Keyring.Derive("radio-token")) and kept in memory only.

const listenIDLength = 26 // ULID

// SignToken returns the listen token of a listen session for a station (exported for the fake of
// the api contract tests, like playback.SignToken).
func SignToken(key []byte, listenID string, stationID int64) string {
	return listenID + "." + base64.RawURLEncoding.EncodeToString(listenMAC(key, listenID, stationID))
}

func listenMAC(key []byte, listenID string, stationID int64) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(listenID + "\n" + strconv.FormatInt(stationID, 10)))
	return m.Sum(nil)
}

// checkToken reports whether token is the listen token of listenID for stationID (constant time).
func checkToken(key []byte, token, listenID string, stationID int64) bool {
	id, enc, ok := strings.Cut(token, ".")
	if !ok || len(id) != listenIDLength || id != listenID {
		return false
	}
	mac, err := base64.RawURLEncoding.DecodeString(enc)
	return err == nil && hmac.Equal(mac, listenMAC(key, listenID, stationID))
}
