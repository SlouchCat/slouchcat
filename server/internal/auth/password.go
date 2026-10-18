package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// argonParams are the argon2id cost parameters (plan 3.8 point 2). They are stored in every hash,
// so raising them later only affects new hashes.
type argonParams struct {
	memoryKiB uint32
	time      uint32
	threads   uint8
	saltLen   int
	keyLen    uint32
}

var defaultParams = argonParams{memoryKiB: 64 * 1024, time: 3, threads: 4, saltLen: 16, keyLen: 32}

// Upper bounds for parameters read from a stored hash, so a corrupted row cannot make a login
// allocate gigabytes.
const (
	maxMemoryKiB = 1 << 20 // 1 GiB
	maxTime      = 16
)

var errBadHash = errors.New("auth: malformed password hash")

// hashPassword returns the PHC string $argon2id$v=19$m=…,t=…,p=…$<salt>$<hash>.
func hashPassword(password string, p argonParams) (string, error) {
	salt := make([]byte, p.saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, p.time, p.memoryKiB, p.threads, p.keyLen)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.memoryKiB, p.time, p.threads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// verifyPassword checks password against a PHC string, with the parameters taken from the string.
func verifyPassword(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return false, errBadHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, errBadHash
	}
	var p argonParams
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memoryKiB, &p.time, &p.threads); err != nil {
		return false, errBadHash
	}
	if p.memoryKiB == 0 || p.memoryKiB > maxMemoryKiB || p.time == 0 || p.time > maxTime || p.threads == 0 {
		return false, errBadHash
	}
	b64 := base64.RawStdEncoding
	salt, err := b64.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return false, errBadHash
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil || len(want) == 0 || len(want) > 1024 {
		return false, errBadHash
	}
	got := argon2.IDKey([]byte(password), salt, p.time, p.memoryKiB, p.threads, uint32(len(want))) //nolint:gosec // G115: len(want) is bounded to 1024 above
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
