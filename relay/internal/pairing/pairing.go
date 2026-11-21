// Package pairing is the encryption of the TV set-up (concept 27 section 7.2, "Protokolle" of the
// build log): the phone seals the packet for the TV's one-time X25519 key with HPKE (RFC 9180),
// and the user's server seals the device token of the redeem answer for the same key and signs it.
//
// Suite: DHKEM(X25519, HKDF-SHA256), HKDF-SHA256, AES-256-GCM, base mode, one message, empty aad.
// The sealed form is enc (32 bytes) followed by the ciphertext. The info of the packet is
// "slouchcat-pair-v1:" followed by the meeting point id as text, so a packet does not fit another
// meeting point; the info of the redeem answer is "slouchcat-redeem-v1".
//
// The meeting point service never imports this package: it sees only ciphertext. paircheck and
// the test vectors in testdata/crypto/hpke.json and pickup.json check it, and the server and the apps
// implement the same.
package pairing

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hpke"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

// Info prefixes and the version of the packet.
const (
	PacketInfoPrefix = "slouchcat-pair-v1:"
	RedeemInfo       = "slouchcat-redeem-v1"
	Version          = 1
	// RedeemSignaturePrefix precedes the bytes of sealedToken in what the server signs with its
	// Ed25519 key (the serverKey of the packet); the TV checks the signature before it opens the
	// token, so nobody on the way can swap in a token sealed for the TV's public key.
	RedeemSignaturePrefix = "slouchcat-redeem-signature-v1:"
)

// Packet is the plaintext the phone hands to the TV.
type Packet struct {
	V         int    `json:"v"`
	Server    string `json:"server"`
	Voucher   string `json:"voucher"`
	ServerKey string `json:"serverKey,omitempty"`
}

// RedeemAnswer is the plaintext of sealedToken in the server's redeem answer: the device token, its
// device id and the account the TV shows next to the server address (K-9).
type RedeemAnswer struct {
	Token        string `json:"token"`
	DeviceID     string `json:"deviceId"`
	AccountEmail string `json:"accountEmail"`
}

// SignRedeem is the server's signature over sealedToken (what the server does).
func SignRedeem(server ed25519.PrivateKey, sealed []byte) []byte {
	return ed25519.Sign(server, append([]byte(RedeemSignaturePrefix), sealed...))
}

// VerifyRedeem checks the signature of the redeem answer with the serverKey of the packet (32 bytes).
func VerifyRedeem(serverKey, sealed, signature []byte) bool {
	if len(serverKey) != ed25519.PublicKeySize {
		return false
	}
	return ed25519.Verify(serverKey, append([]byte(RedeemSignaturePrefix), sealed...), signature)
}

var (
	kem  = hpke.DHKEM(ecdh.X25519())
	kdf  = hpke.HKDFSHA256()
	aead = hpke.AES256GCM()
)

// B64 is the encoding of every binary value of the protocol: base64url without padding.
var B64 = base64.RawURLEncoding

// NewTVKey returns a fresh one-time X25519 key of the TV.
func NewTVKey() (*ecdh.PrivateKey, error) {
	return ecdh.X25519().GenerateKey(rand.Reader)
}

// NewSecret returns n random bytes in base64url (meeting point id 16, pickup secret 32, voucher 32).
func NewSecret(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return B64.EncodeToString(b)
}

// PickupHash is what the TV sends when it opens a meeting point: SHA-256 over the 32 raw bytes of
// the pickup secret (not over its base64url text), in base64url.
func PickupHash(secret string) (string, error) {
	raw, err := B64.DecodeString(secret)
	if err != nil || len(raw) != 32 {
		return "", errors.New("pickup secret: want 32 bytes in base64url")
	}
	sum := sha256.Sum256(raw)
	return B64.EncodeToString(sum[:]), nil
}

// SealPacket encrypts p for the TV's public key (32 raw bytes) and the meeting point id.
func SealPacket(tvPublic []byte, id string, p Packet) ([]byte, error) {
	plain, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return seal(tvPublic, []byte(PacketInfoPrefix+id), plain)
}

// OpenPacket decrypts a packet with the TV's private key.
func OpenPacket(tv *ecdh.PrivateKey, id string, sealed []byte) (Packet, error) {
	var p Packet
	plain, err := open(tv, []byte(PacketInfoPrefix+id), sealed)
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal(plain, &p); err != nil {
		return p, fmt.Errorf("packet: %w", err)
	}
	if p.V != Version {
		return p, fmt.Errorf("packet: version %d, want %d", p.V, Version)
	}
	return p, nil
}

// SealRedeem encrypts the redeem answer for the TV's public key (what the server does).
func SealRedeem(tvPublic []byte, a RedeemAnswer) ([]byte, error) {
	plain, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	return seal(tvPublic, []byte(RedeemInfo), plain)
}

// OpenRedeem decrypts sealedToken with the TV's private key.
func OpenRedeem(tv *ecdh.PrivateKey, sealed []byte) (RedeemAnswer, error) {
	var a RedeemAnswer
	plain, err := open(tv, []byte(RedeemInfo), sealed)
	if err != nil {
		return a, err
	}
	return a, json.Unmarshal(plain, &a)
}

// Seal and Open with an explicit info, for the test vectors.
func Seal(tvPublic, info, plain []byte) ([]byte, error) { return seal(tvPublic, info, plain) }

// Open is the counterpart of Seal.
func Open(tv *ecdh.PrivateKey, info, sealed []byte) ([]byte, error) { return open(tv, info, sealed) }

func seal(tvPublic, info, plain []byte) ([]byte, error) {
	pk, err := kem.NewPublicKey(tvPublic)
	if err != nil {
		return nil, fmt.Errorf("tv key: %w", err)
	}
	return hpke.Seal(pk, kdf, aead, info, plain)
}

func open(tv *ecdh.PrivateKey, info, sealed []byte) ([]byte, error) {
	sk, err := hpke.NewDHKEMPrivateKey(tv)
	if err != nil {
		return nil, err
	}
	if len(sealed) < 32+16 {
		return nil, errors.New("sealed message too short")
	}
	return hpke.Open(sk, kdf, aead, info, sealed)
}
