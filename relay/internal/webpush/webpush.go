// Package webpush is the message encryption of Web Push (RFC 8291, content coding aes128gcm of
// RFC 8188), both ways: Encrypt is what a Slouchcat server does, Decrypt what the Android app
// does. The push forwarder never decrypts and never imports this package; pushcheck and the tests
// use it to prove that a message crosses the forwarder and Google unchanged.
package webpush

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

// Subscriber is the receiving side: the app's P-256 key and its 16-byte auth secret.
type Subscriber struct {
	Key  *ecdh.PrivateKey
	Auth []byte
}

// NewSubscriber makes fresh keys.
func NewSubscriber() (*Subscriber, error) {
	k, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	auth := make([]byte, 16)
	_, _ = rand.Read(auth)
	return &Subscriber{Key: k, Auth: auth}, nil
}

func derive(auth, secret, uaPublic, asPublic, salt []byte) (cek, nonce []byte, err error) {
	info := append(append([]byte("WebPush: info\x00"), uaPublic...), asPublic...)
	ikm, err := hkdf.Key(sha256.New, secret, auth, string(info), 32)
	if err != nil {
		return nil, nil, err
	}
	prk, err := hkdf.Extract(sha256.New, ikm, salt)
	if err != nil {
		return nil, nil, err
	}
	if cek, err = hkdf.Expand(sha256.New, prk, "Content-Encoding: aes128gcm\x00", 16); err != nil {
		return nil, nil, err
	}
	nonce, err = hkdf.Expand(sha256.New, prk, "Content-Encoding: nonce\x00", 12)
	return cek, nonce, err
}

// Encrypt seals plaintext for the subscriber's public key and auth secret into one aes128gcm
// record of recordSize bytes (padding fills it).
func Encrypt(uaPublic *ecdh.PublicKey, auth, plaintext []byte, recordSize int) ([]byte, error) {
	as, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	secret, err := as.ECDH(uaPublic)
	if err != nil {
		return nil, err
	}
	salt := make([]byte, 16)
	_, _ = rand.Read(salt)
	asPub := as.PublicKey().Bytes()
	cek, nonce, err := derive(auth, secret, uaPublic.Bytes(), asPub, salt)
	if err != nil {
		return nil, err
	}
	headerLen := 16 + 4 + 1 + len(asPub)
	maxPlain := recordSize - headerLen - 16 - 1
	if len(plaintext) > maxPlain {
		return nil, errors.New("webpush: plaintext too long for the record size")
	}
	padded := make([]byte, maxPlain+1)
	copy(padded, plaintext)
	padded[len(plaintext)] = 2 // last-record delimiter, then zero padding
	gcm, err := newGCM(cek)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, recordSize)
	out = append(out, salt...)
	out = binary.BigEndian.AppendUint32(out, uint32(recordSize)) //nolint:gosec // record sizes are small
	out = append(out, byte(len(asPub)))                          //nolint:gosec // a P-256 point is 65 bytes
	out = append(out, asPub...)
	return gcm.Seal(out, nonce, padded, nil), nil
}

// Decrypt opens a single-record aes128gcm message.
func (s *Subscriber) Decrypt(body []byte) ([]byte, error) {
	if len(body) < 21 {
		return nil, errors.New("webpush: body too short")
	}
	salt := body[:16]
	idLen := int(body[20])
	if len(body) < 21+idLen+16 {
		return nil, errors.New("webpush: body too short")
	}
	asPubRaw := body[21 : 21+idLen]
	asPub, err := ecdh.P256().NewPublicKey(asPubRaw)
	if err != nil {
		return nil, err
	}
	secret, err := s.Key.ECDH(asPub)
	if err != nil {
		return nil, err
	}
	cek, nonce, err := derive(s.Auth, secret, s.Key.PublicKey().Bytes(), asPubRaw, salt)
	if err != nil {
		return nil, err
	}
	gcm, err := newGCM(cek)
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, nonce, body[21+idLen:], nil)
	if err != nil {
		return nil, err
	}
	// Strip the padding: zeros back to the delimiter 2.
	i := len(plain) - 1
	for i >= 0 && plain[i] == 0 {
		i--
	}
	if i < 0 || plain[i] != 2 {
		return nil, errors.New("webpush: no last-record delimiter")
	}
	return plain[:i], nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
