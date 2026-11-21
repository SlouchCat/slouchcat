package pairing

import (
	"bytes"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hpke"
	"crypto/rand"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The shared test vectors (format slouchcat-crypto-vectors-v1, concept 27 sections 7.2 and 12):
// written once by the stage 0 generator and fixed since, because HPKE seals at random. Go reads
// them here, Kotlin reads the same files in the app's tests. hpke.json carries the two cases of
// the protocol and the RFC 9180 vector A.1.1, which checks the implementation against values it
// did not produce itself (security review L-7).

type vectorFile struct {
	Format string `json:"format"`
	Kind   string `json:"kind"`
	Cases  []struct {
		Name     string         `json:"name"`
		Input    map[string]any `json:"input"`
		Expected map[string]any `json:"expected"`
	} `json:"cases"`
}

func readVectors(t *testing.T, name, kind string) vectorFile {
	t.Helper()
	data, err := os.ReadFile("../../../testdata/crypto/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var f vectorFile
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	if f.Format != "slouchcat-crypto-vectors-v1" || f.Kind != kind || len(f.Cases) == 0 {
		t.Fatalf("%s: format %q, kind %q, %d cases", name, f.Format, f.Kind, len(f.Cases))
	}
	return f
}

func raw(t *testing.T, v any) []byte {
	t.Helper()
	s, ok := v.(string)
	if !ok {
		t.Fatalf("%v is not a base64url string", v)
	}
	b, err := B64.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func num(t *testing.T, v any) uint16 {
	t.Helper()
	f, ok := v.(float64)
	if !ok {
		t.Fatalf("%v is not a number", v)
	}
	return uint16(f)
}

func TestHPKEVectors(t *testing.T) {
	f := readVectors(t, "hpke.json", "hpke-base-single-shot")
	var protocol, rfc int
	for _, c := range f.Cases {
		t.Run(c.Name, func(t *testing.T) {
			in := c.Input
			if num(t, in["mode"]) != 0 {
				t.Fatal("only the base mode is used")
			}
			kem, err := hpke.NewKEM(num(t, in["kem"]))
			if err != nil {
				t.Fatal(err)
			}
			kdf, err := hpke.NewKDF(num(t, in["kdf"]))
			if err != nil {
				t.Fatal(err)
			}
			aead, err := hpke.NewAEAD(num(t, in["aead"]))
			if err != nil {
				t.Fatal(err)
			}
			sk, err := kem.NewPrivateKey(raw(t, in["recipientPrivateKey"]))
			if err != nil {
				t.Fatal(err)
			}
			if pub := sk.PublicKey().Bytes(); !bytes.Equal(pub, raw(t, in["recipientPublicKey"])) {
				t.Fatal("recipientPublicKey does not belong to recipientPrivateKey")
			}
			sealed := raw(t, in["sealed"])
			r, err := hpke.NewRecipient(sealed[:32], sk, kdf, aead, raw(t, in["info"]))
			if err != nil {
				t.Fatal(err)
			}
			plain, err := r.Open(raw(t, in["aad"]), sealed[32:])
			if err != nil {
				t.Fatal(err)
			}
			if want := raw(t, c.Expected["plaintext"]); !bytes.Equal(plain, want) {
				t.Fatalf("plaintext = %q, want %q", plain, want)
			}
			// The protocol cases also open through the package's own functions.
			info := string(raw(t, in["info"]))
			tv, err := ecdh.X25519().NewPrivateKey(raw(t, in["recipientPrivateKey"]))
			if err != nil {
				t.Fatal(err)
			}
			switch {
			case strings.HasPrefix(info, PacketInfoPrefix):
				protocol++
				id := strings.TrimPrefix(info, PacketInfoPrefix)
				p, err := OpenPacket(tv, id, sealed)
				if err != nil || p.Server == "" || p.Voucher == "" {
					t.Fatalf("OpenPacket = %+v, %v", p, err)
				}
				if _, err := OpenPacket(tv, id+"x", sealed); err == nil {
					t.Fatal("the packet opened for another meeting point")
				}
			case info == RedeemInfo && len(plain) > 0:
				protocol++
				a, err := OpenRedeem(tv, sealed)
				if err != nil || a.Token == "" || a.DeviceID == "" || a.AccountEmail == "" {
					t.Fatalf("OpenRedeem = %+v, %v", a, err)
				}
			case strings.HasPrefix(c.Name, "RFC 9180"):
				rfc++
			}
		})
	}
	if protocol != 2 || rfc != 1 {
		t.Fatalf("%d protocol cases and %d RFC cases, want 2 and 1", protocol, rfc)
	}
}

func TestPickupVectors(t *testing.T) {
	f := readVectors(t, "pickup.json", "pickup-sha256")
	for _, c := range f.Cases {
		secret, _ := c.Input["secret"].(string)
		want, _ := c.Expected["hash"].(string)
		if got, err := PickupHash(secret); err != nil || got != want {
			t.Fatalf("%s: PickupHash = %q, %v; want %q", c.Name, got, err, want)
		}
	}
}

func TestRoundTripAndTamper(t *testing.T) {
	tv, err := NewTVKey()
	if err != nil {
		t.Fatal(err)
	}
	id := NewSecret(16)
	sealed, err := SealPacket(tv.PublicKey().Bytes(), id, Packet{V: 1, Server: "http://192.0.2.7:8096", Voucher: NewSecret(32)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OpenPacket(tv, id, sealed); err != nil {
		t.Fatal(err)
	}
	sealed[len(sealed)-1] ^= 1
	if _, err := OpenPacket(tv, id, sealed); err == nil {
		t.Fatal("a tampered packet opened")
	}
	other, _ := NewTVKey()
	if _, err := OpenPacket(other, id, sealed[:len(sealed)-1]); err == nil {
		t.Fatal("another key opened the packet")
	}
	if _, err := PickupHash("short"); err == nil {
		t.Fatal("a short pickup secret was accepted")
	}
}

// The redeem answer of today's server: the token, the device id and the account's e-mail sealed
// for the TV, signed with the server's key, which the TV checks with the serverKey of the packet.
func TestRedeemAnswerSignedAndSealed(t *testing.T) {
	tv, err := NewTVKey()
	if err != nil {
		t.Fatal(err)
	}
	serverPub, server, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	want := RedeemAnswer{Token: NewSecret(32), DeviceID: "0123456789abcdef0123456789abcdef", AccountEmail: "owner@example.test"}
	sealed, err := SealRedeem(tv.PublicKey().Bytes(), want)
	if err != nil {
		t.Fatal(err)
	}
	sig := SignRedeem(server, sealed)
	if !VerifyRedeem(serverPub, sealed, sig) {
		t.Fatal("the signature does not verify")
	}
	got, err := OpenRedeem(tv, sealed)
	if err != nil || got != want {
		t.Fatalf("OpenRedeem = %+v, %v; want %+v", got, err, want)
	}
	otherPub, _, _ := ed25519.GenerateKey(rand.Reader)
	swapped := bytes.Clone(sealed)
	swapped[40] ^= 1
	for name, ok := range map[string]bool{
		"another server key": VerifyRedeem(otherPub, sealed, sig),
		"a swapped token":    VerifyRedeem(serverPub, swapped, sig),
		"a short key":        VerifyRedeem(serverPub[:31], sealed, sig),
	} {
		if ok {
			t.Errorf("%s verifies", name)
		}
	}
}
