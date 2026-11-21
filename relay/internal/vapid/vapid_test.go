package vapid

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"strings"
	"testing"
	"time"
)

var origins = []string{"https://push.example.test", "https://push-probe.example.test:8443"}

func TestVerify(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	pub, _ := PublicKeyString(&key.PublicKey)
	other, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	otherPub, _ := PublicKeyString(&other.PublicKey)
	now := time.Unix(2_000_000_000, 0)
	sign := func(k *ecdsa.PrivateKey, aud string, exp time.Time) string {
		a, err := Authorization(k, Claims{Aud: aud, Exp: exp.Unix(), Sub: "mailto:ops@example.test"})
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	good := sign(key, origins[0], now.Add(12*time.Hour))
	cases := []struct {
		name      string
		auth, key string
		want      error
	}{
		{"valid", good, pub, nil},
		{"valid for the probe name", sign(key, origins[1], now.Add(time.Hour)), pub, nil},
		{"padding in k", good + "=", pub, nil},
		{"key of another server", sign(other, origins[0], now.Add(time.Hour)), pub, ErrKey},
		{"signed by another key, k replaced", strings.Replace(sign(other, origins[0], now.Add(time.Hour)), otherPub, pub, 1), pub, ErrSignature},
		{"foreign aud", sign(key, "https://fcm.googleapis.com", now.Add(time.Hour)), pub, ErrAudience},
		{"aud with a path", sign(key, origins[0]+"/v1", now.Add(time.Hour)), pub, ErrAudience},
		{"expired", sign(key, origins[0], now.Add(-time.Second)), pub, ErrExpired},
		{"expires now", sign(key, origins[0], now), pub, ErrExpired},
		{"valid for more than 24 hours", sign(key, origins[0], now.Add(25*time.Hour)), pub, ErrExpired},
		{"no header", "", pub, ErrMissing},
		{"other scheme", "WebPush " + good[6:], pub, ErrMissing},
		{"no k", strings.Split(good, ", k=")[0], pub, ErrMissing},
		{"not a JWT", "vapid t=abc, k=" + pub, pub, ErrMalformed},
		{"sealed key is garbage", good, "garbage", ErrKey},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := Verify(c.auth, c.key, origins, now)
			if !errors.Is(err, c.want) && (err != nil || c.want != nil) {
				t.Fatalf("Verify = %v, want %v", err, c.want)
			}
		})
	}
}

func TestVerifyRejectsTamperedClaims(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	pub, _ := PublicKeyString(&key.PublicKey)
	now := time.Now()
	a, _ := Authorization(key, Claims{Aud: "https://elsewhere.example.test", Exp: now.Add(time.Hour).Unix()})
	// Swap in claims for this service and keep the old signature.
	_, ok, _ := strings.Cut(a, "t=")
	parts := strings.Split(strings.Split(ok, ",")[0], ".")
	forged, _ := Authorization(key, Claims{Aud: origins[0], Exp: now.Add(time.Hour).Unix()})
	fparts := strings.Split(strings.Split(strings.SplitN(forged, "t=", 2)[1], ",")[0], ".")
	tampered := "vapid t=" + parts[0] + "." + fparts[1] + "." + parts[2] + ", k=" + pub
	if err := Verify(tampered, pub, origins, now); !errors.Is(err, ErrSignature) {
		t.Fatalf("tampered claims: %v", err)
	}
}
