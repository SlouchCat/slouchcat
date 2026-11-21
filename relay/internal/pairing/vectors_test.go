package pairing

import (
	"bytes"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"testing/cryptotest"
)

// The shared test vectors of the TV set-up (concept 27 sections 7.2 and 12): Go writes them once,
// Go and Kotlin both read them in CI. `go test ./internal/pairing -run TestVectors -update`
// writes the file anew; the content is deterministic, so a rerun changes nothing.

var update = flag.Bool("update", false, "write testdata/crypto/pairing-v1.json")

const vectorFile = "../../../testdata/crypto/pairing-v1.json"

type vectorSuite struct {
	KEM    string `json:"kem"`
	KEMID  int    `json:"kemId"`
	KDF    string `json:"kdf"`
	KDFID  int    `json:"kdfId"`
	AEAD   string `json:"aead"`
	AEADID int    `json:"aeadId"`
	Mode   string `json:"mode"`
}

type vectorCase struct {
	Name                string `json:"name"`
	TVPrivateKey        string `json:"tvPrivateKey"`
	TVPublicKey         string `json:"tvPublicKey"`
	MeetingPointID      string `json:"meetingPointId,omitempty"`
	Info                string `json:"info"`
	AAD                 string `json:"aad"`
	Plaintext           string `json:"plaintext"`
	EphemeralPrivateKey string `json:"ephemeralPrivateKey"`
	Enc                 string `json:"enc"`
	Sealed              string `json:"sealed"`
}

type vectorPickup struct {
	Secret string `json:"secret"`
	Hash   string `json:"hash"`
}

type vectorFileT struct {
	Description string       `json:"description"`
	Encoding    string       `json:"encoding"`
	Suite       vectorSuite  `json:"suite"`
	Pickup      vectorPickup `json:"pickup"`
	Cases       []vectorCase `json:"cases"`
}

func fixed(label string, n int) []byte {
	sum := sha256.Sum256([]byte("slouchcat test vector " + label))
	return sum[:n]
}

// sealDeterministic seals with the global random source fixed to seed and returns the sealed
// message and the ephemeral private key HPKE drew from that source.
func sealDeterministic(t *testing.T, seed uint64, pub, info, plain []byte) (sealed, eph []byte) {
	t.Helper()
	var out []byte
	t.Run("seal", func(t *testing.T) {
		cryptotest.SetGlobalRandom(t, seed)
		var err error
		out, err = Seal(pub, info, plain)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("ephemeral", func(t *testing.T) {
		cryptotest.SetGlobalRandom(t, seed)
		eph = make([]byte, 32)
		_, _ = rand.Read(eph)
	})
	k, err := ecdh.X25519().NewPrivateKey(eph)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(k.PublicKey().Bytes(), out[:32]) {
		t.Fatal("the ephemeral key drawn from the fixed random source does not match enc")
	}
	return out, eph
}

func writeVectors(t *testing.T) {
	tv, err := ecdh.X25519().NewPrivateKey(fixed("tv key v1", 32))
	if err != nil {
		t.Fatal(err)
	}
	id := B64.EncodeToString(fixed("meeting point id v1", 16))
	packet, err := json.Marshal(Packet{
		V: 1, Server: "https://media.example.test", Voucher: B64.EncodeToString(fixed("voucher v1", 32)),
		ServerKey: B64.EncodeToString(fixed("server key v1", 32)),
	})
	if err != nil {
		t.Fatal(err)
	}
	redeem, err := json.Marshal(RedeemAnswer{Token: "sc_" + B64.EncodeToString(fixed("device token v1", 32)), DeviceID: "dev_" + B64.EncodeToString(fixed("device id v1", 12))})
	if err != nil {
		t.Fatal(err)
	}
	secret := B64.EncodeToString(fixed("pickup secret v1", 32))
	hash, err := PickupHash(secret)
	if err != nil {
		t.Fatal(err)
	}
	f := vectorFileT{
		Description: "Concept 27 section 7.2: the packet of the TV set-up and the sealed token of the redeem answer. " +
			"Open each sealed value with tvPrivateKey and info and compare with plaintext. " +
			"A library that takes a fixed ephemeral key can also reproduce sealed from ephemeralPrivateKey.",
		Encoding: "every key, id and sealed value is base64url without padding; plaintext and info are UTF-8 text",
		Suite: vectorSuite{
			KEM: "DHKEM(X25519, HKDF-SHA256)", KEMID: 0x20, KDF: "HKDF-SHA256", KDFID: 1,
			AEAD: "AES-256-GCM", AEADID: 2, Mode: "base",
		},
		Pickup: vectorPickup{Secret: secret, Hash: hash},
	}
	for i, c := range []struct {
		name, id, info string
		plain          []byte
	}{
		{"packet", id, PacketInfoPrefix + id, packet},
		{"redeem", "", RedeemInfo, redeem},
	} {
		sealed, eph := sealDeterministic(t, uint64(27+i), tv.PublicKey().Bytes(), []byte(c.info), c.plain)
		f.Cases = append(f.Cases, vectorCase{
			Name: c.name, TVPrivateKey: B64.EncodeToString(tv.Bytes()), TVPublicKey: B64.EncodeToString(tv.PublicKey().Bytes()),
			MeetingPointID: c.id, Info: c.info, AAD: "", Plaintext: string(c.plain),
			EphemeralPrivateKey: B64.EncodeToString(eph), Enc: B64.EncodeToString(sealed[:32]), Sealed: B64.EncodeToString(sealed),
		})
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(vectorFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(vectorFile, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestVectors(t *testing.T) {
	if *update {
		writeVectors(t)
	}
	data, err := os.ReadFile(vectorFile)
	if err != nil {
		t.Fatal(err)
	}
	var f vectorFileT
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	if got, err := PickupHash(f.Pickup.Secret); err != nil || got != f.Pickup.Hash {
		t.Fatalf("pickup hash = %q, %v; want %q", got, err, f.Pickup.Hash)
	}
	if len(f.Cases) != 2 {
		t.Fatalf("%d cases, want 2", len(f.Cases))
	}
	for _, c := range f.Cases {
		t.Run(c.Name, func(t *testing.T) {
			raw := func(s string) []byte {
				b, err := B64.DecodeString(s)
				if err != nil {
					t.Fatal(err)
				}
				return b
			}
			tv, err := ecdh.X25519().NewPrivateKey(raw(c.TVPrivateKey))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(tv.PublicKey().Bytes(), raw(c.TVPublicKey)) {
				t.Fatal("tvPublicKey does not belong to tvPrivateKey")
			}
			sealed := raw(c.Sealed)
			if !bytes.Equal(sealed[:32], raw(c.Enc)) {
				t.Fatal("sealed does not start with enc")
			}
			eph, err := ecdh.X25519().NewPrivateKey(raw(c.EphemeralPrivateKey))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(eph.PublicKey().Bytes(), raw(c.Enc)) {
				t.Fatal("enc is not the public key of ephemeralPrivateKey")
			}
			plain, err := Open(tv, []byte(c.Info), sealed)
			if err != nil {
				t.Fatal(err)
			}
			if string(plain) != c.Plaintext {
				t.Fatalf("plaintext = %s, want %s", plain, c.Plaintext)
			}
			switch c.Name {
			case "packet":
				if c.Info != PacketInfoPrefix+c.MeetingPointID {
					t.Fatalf("info %q does not name the meeting point", c.Info)
				}
				p, err := OpenPacket(tv, c.MeetingPointID, sealed)
				if err != nil || p.Server == "" || p.Voucher == "" {
					t.Fatalf("OpenPacket = %+v, %v", p, err)
				}
				// Bound to its meeting point: another id does not open it.
				if _, err := OpenPacket(tv, c.MeetingPointID+"x", sealed); err == nil {
					t.Fatal("the packet opened for another meeting point")
				}
			case "redeem":
				a, err := OpenRedeem(tv, sealed)
				if err != nil || a.Token == "" || a.DeviceID == "" {
					t.Fatalf("OpenRedeem = %+v, %v", a, err)
				}
			}
		})
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
