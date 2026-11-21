package webpush

import (
	"bytes"
	"crypto/ecdh"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// testdata/crypto/webpush.json (format slouchcat-crypto-vectors-v1): two messages encrypted by
// the server's own library at record sizes 4096 and 2700 (stage 0), and the example of RFC 8291
// section 5, which checks Decrypt against values it did not produce itself (security review L-7).
func TestWebPushVectors(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/crypto/webpush.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Format string `json:"format"`
		Kind   string `json:"kind"`
		Cases  []struct {
			Name  string `json:"name"`
			Input struct {
				AuthSecret   string `json:"authSecret"`
				Body         string `json:"body"`
				RecordSize   int    `json:"recordSize"`
				UAPrivateKey string `json:"uaPrivateKey"`
				UAPublicKey  string `json:"uaPublicKey"`
			} `json:"input"`
			Expected struct {
				Plaintext string `json:"plaintext"`
			} `json:"expected"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	if f.Format != "slouchcat-crypto-vectors-v1" || f.Kind != "webpush-aes128gcm-decrypt" {
		t.Fatalf("format %q, kind %q", f.Format, f.Kind)
	}
	b64 := base64.RawURLEncoding
	dec := func(s string) []byte {
		b, err := b64.DecodeString(s)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	rfc := 0
	for _, c := range f.Cases {
		t.Run(c.Name, func(t *testing.T) {
			k, err := ecdh.P256().NewPrivateKey(dec(c.Input.UAPrivateKey))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(k.PublicKey().Bytes(), dec(c.Input.UAPublicKey)) {
				t.Fatal("uaPublicKey does not belong to uaPrivateKey")
			}
			s := &Subscriber{Key: k, Auth: dec(c.Input.AuthSecret)}
			plain, err := s.Decrypt(dec(c.Input.Body))
			if err != nil {
				t.Fatal(err)
			}
			if want := dec(c.Expected.Plaintext); !bytes.Equal(plain, want) {
				t.Fatalf("plaintext = %q, want %q", plain, want)
			}
			if strings.HasPrefix(c.Name, "RFC 8291") {
				rfc++
			}
		})
	}
	if rfc != 1 {
		t.Fatalf("%d RFC cases, want 1", rfc)
	}
}
