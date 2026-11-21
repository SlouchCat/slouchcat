package auth

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hpke"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/secrets"
)

// Concept 27 sections 6 and 10, K26 4.6 (ADR 0040): the pairing of a TV with a voucher and the
// proof of the server's identity.

// linked is one call of the WithDeviceLinked hook.
type linked struct {
	accountID          int64
	kind, name, system string
	at                 time.Time
}

// pairingOwner is appOwner with a phone app session of the owner (primary profile active) and the
// calls of the device-linked hook.
func pairingOwner(t *testing.T) (*Service, *fakeClock, model.Session, *[]linked) {
	t.Helper()
	s, clock, _ := appOwner(t)
	var mu sync.Mutex
	calls := &[]linked{}
	s.WithDeviceLinked(func(_ context.Context, accountID int64, kind, name, system string, at time.Time) error {
		mu.Lock()
		defer mu.Unlock()
		*calls = append(*calls, linked{accountID, kind, name, system, at})
		return nil
	})
	phone, _, err := s.AppLogin(context.Background(), testEmail, testPassword, "Pixel", AppDevicePhone, "", "okhttp/5")
	if err != nil {
		t.Fatal(err)
	}
	if phone.Kind != model.SessionApp || !phone.IsPrimary() {
		t.Fatalf("phone session %+v, want an app session with the primary profile", phone)
	}
	return s, clock, phone, calls
}

// newTVKey returns a TV's one-time key and its public half in base64url.
func newTVKey(t *testing.T) (*ecdh.PrivateKey, string) {
	t.Helper()
	k, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return k, b64.EncodeToString(k.PublicKey().Bytes())
}

// openRedeem opens sealedToken with the TV's key like the TV app does.
func openRedeem(t *testing.T, tv *ecdh.PrivateKey, sealed []byte) (token, deviceID string) {
	t.Helper()
	sk, err := hpke.NewDHKEMPrivateKey(tv)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := hpke.Open(sk, hpke.HKDFSHA256(), hpke.AES256GCM(), []byte("slouchcat-redeem-v1"), sealed)
	if err != nil {
		t.Fatalf("open sealed token: %v", err)
	}
	var answer struct {
		Token    string `json:"token"`
		DeviceID string `json:"deviceId"`
	}
	if err := json.Unmarshal(plain, &answer); err != nil {
		t.Fatal(err)
	}
	return answer.Token, answer.DeviceID
}

func TestPairingRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, clock, phone, calls := pairingOwner(t)
	tv, tvKey := newTVKey(t)
	v, err := s.CreatePairingVoucher(ctx, phone, tvKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Voucher) != 43 || !v.ExpiresAt.Equal(clock.now().Add(VoucherLifetime).UTC()) {
		t.Fatalf("voucher %d characters, expires %v", len(v.Voucher), v.ExpiresAt)
	}
	// The server keeps only the hash.
	var stored string
	if err := s.db.QueryRowContext(ctx, `SELECT voucher_hash FROM pairing_vouchers WHERE account_id = ?`, phone.Account.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != hashToken(v.Voucher) || stored == v.Voucher {
		t.Fatalf("stored %q, want the hash of the voucher", stored)
	}
	out, err := s.RedeemPairingVoucher(ctx, v.Voucher, tvKey, " Wohnzimmer\u202e-TV\u200b ", "Android TV 14")
	if err != nil {
		t.Fatal(err)
	}
	if out.AccountEmail != NormalizeEmail(testEmail) || out.Session.Kind != model.SessionDevice || !out.Session.ExpiresAt.Equal(NeverExpires) {
		t.Fatalf("redeemed %+v", out)
	}
	token, deviceID := openRedeem(t, tv, out.SealedToken)
	if deviceID != out.Session.DeviceID || deviceID == phone.DeviceID || !devicePattern.MatchString(deviceID) {
		t.Fatalf("sealed device %q, session device %q, phone %q", deviceID, out.Session.DeviceID, phone.DeviceID)
	}
	got, _, err := s.Resolve(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != model.SessionDevice || got.Account.ID != phone.Account.ID || got.DeviceID != deviceID {
		t.Fatalf("sealed token resolves to %+v", got)
	}
	dev, err := s.read.GetDevice(ctx, deviceID)
	if err != nil {
		t.Fatal(err)
	}
	if dev.Name.String != "Wohnzimmer-TV" || dev.Type.String != AppDeviceTV || dev.UserAgent.String != "Android TV 14" {
		t.Errorf("device row %q %q %q", dev.Name.String, dev.Type.String, dev.UserAgent.String)
	}
	want := []linked{{phone.Account.ID, AppDeviceTV, "Wohnzimmer-TV", "Android TV 14", clock.now()}}
	if len(*calls) != 1 || (*calls)[0] != want[0] {
		t.Errorf("device linked %+v, want %+v", *calls, want)
	}
	// Only the holder of the private key reads the token.
	other, _ := newTVKey(t)
	sk, _ := hpke.NewDHKEMPrivateKey(other)
	if _, err := hpke.Open(sk, hpke.HKDFSHA256(), hpke.AES256GCM(), []byte(RedeemInfo), out.SealedToken); err == nil {
		t.Error("another key opened the sealed token")
	}
}

// Only an app session issues a voucher, only without a profile or with the primary one (F-23),
// only for a valid X25519 key.
func TestPairingVoucherRules(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, _, phone, _ := pairingOwner(t)
	_, tvKey := newTVKey(t)
	browser, _, err := s.StartSession(ctx, phone.Account, phone.Profile, "Firefox")
	if err != nil {
		t.Fatal(err)
	}
	tvSession, _, err := s.AppLogin(ctx, testEmail, testPassword, "TV", AppDeviceTV, "", "okhttp/5")
	if err != nil {
		t.Fatal(err)
	}
	child := phone
	child.Profile = model.Profile{ID: phone.Profile.ID + 100, AccountID: phone.Account.ID, Name: "Kind"}
	noProfile := phone
	noProfile.Profile = model.Profile{}
	for name, c := range map[string]struct {
		session model.Session
		key     string
		want    error
	}{
		"browser":         {browser, tvKey, model.ErrAppSessionRequired},
		"device":          {tvSession, tvKey, model.ErrAppSessionRequired},
		"child profile":   {child, tvKey, model.ErrPrimaryProfileRequired},
		"no base64url":    {phone, "not a key!", nil},
		"short key":       {phone, "AAAA", nil},
		"without profile": {noProfile, tvKey, nil},
		"primary profile": {phone, tvKey, nil},
	} {
		_, err := s.CreatePairingVoucher(ctx, c.session, c.key)
		var verr *model.ValidationError
		switch {
		case c.want != nil && !errors.Is(err, c.want):
			t.Errorf("%s: %v, want %v", name, err, c.want)
		case c.want == nil && c.key != tvKey && !errors.As(err, &verr):
			t.Errorf("%s: %v, want a validation error", name, err)
		case c.want == nil && c.key == tvKey && err != nil:
			t.Errorf("%s: %v", name, err)
		}
	}
}

// A voucher redeems once, before it expires and with exactly the key it is bound to; every other
// case answers the same ErrVoucherInvalid, and a voucher shown with another key is used up.
func TestPairingRedeemRefusals(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, clock, phone, calls := pairingOwner(t)
	_, tvKey := newTVKey(t)
	_, otherKey := newTVKey(t)
	issue := func() string {
		t.Helper()
		v, err := s.CreatePairingVoucher(ctx, phone, tvKey)
		if err != nil {
			t.Fatal(err)
		}
		return v.Voucher
	}
	redeem := func(voucher, key string) error {
		_, err := s.RedeemPairingVoucher(ctx, voucher, key, "TV", "Android TV 14")
		return err
	}

	unknown := b64.EncodeToString(make([]byte, 32))
	if err := redeem(unknown, tvKey); !errors.Is(err, model.ErrVoucherInvalid) {
		t.Errorf("unknown: %v", err)
	}

	v := issue()
	if err := redeem(v, otherKey); !errors.Is(err, model.ErrVoucherInvalid) {
		t.Errorf("other key: %v", err)
	}
	if err := redeem(v, tvKey); !errors.Is(err, model.ErrVoucherInvalid) {
		t.Errorf("right key after another key: %v, want the voucher used up", err)
	}

	v = issue()
	clock.advance(VoucherLifetime)
	if err := redeem(v, tvKey); !errors.Is(err, model.ErrVoucherInvalid) {
		t.Errorf("expired: %v", err)
	}

	// A new voucher replaces the open one.
	old := issue()
	v = issue()
	if err := redeem(old, tvKey); !errors.Is(err, model.ErrVoucherInvalid) {
		t.Errorf("replaced voucher: %v", err)
	}
	if err := redeem(v, tvKey); err != nil {
		t.Fatalf("fresh voucher: %v", err)
	}
	if err := redeem(v, tvKey); !errors.Is(err, model.ErrVoucherInvalid) {
		t.Errorf("second redeem: %v", err)
	}
	if len(*calls) != 1 {
		t.Errorf("device linked %d times, want once", len(*calls))
	}

	// The shape is checked before the lookup.
	var verr *model.ValidationError
	if _, err := s.RedeemPairingVoucher(ctx, "short", "short", "", "\u202e\x00"); !errors.As(err, &verr) || len(verr.Fields) != 4 {
		t.Errorf("bad shape: %v", err)
	}
}

// Of concurrent redeems of one voucher exactly one wins.
func TestPairingRedeemConcurrent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, _, phone, calls := pairingOwner(t)
	_, tvKey := newTVKey(t)
	v, err := s.CreatePairingVoucher(ctx, phone, tvKey)
	if err != nil {
		t.Fatal(err)
	}
	const n = 8
	var wins, refused atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ipCtx := model.WithClientIP(ctx, "192.0.2."+string(rune('1'+i)))
			_, err := s.RedeemPairingVoucher(ipCtx, v.Voucher, tvKey, "TV", "Android TV 14")
			switch {
			case err == nil:
				wins.Add(1)
			case errors.Is(err, model.ErrVoucherInvalid):
				refused.Add(1)
			default:
				t.Errorf("redeem: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()
	if wins.Load() != 1 || refused.Load() != n-1 || len(*calls) != 1 {
		t.Fatalf("%d wins, %d refused, %d messages; want 1, %d, 1", wins.Load(), refused.Load(), len(*calls), n-1)
	}
	var sessions int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM sessions WHERE kind = 'device'`).Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if sessions != 1 {
		t.Errorf("%d device sessions, want 1", sessions)
	}
}

// Redeem attempts count per client address, vouchers per account.
func TestPairingLimits(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, clock, phone, _ := pairingOwner(t)
	_, tvKey := newTVKey(t)
	unknown := b64.EncodeToString(make([]byte, 32))
	ip := model.WithClientIP(ctx, "192.0.2.10")
	for i := range redeemsPerMin {
		if _, err := s.RedeemPairingVoucher(ip, unknown, tvKey, "TV", "Android TV 14"); !errors.Is(err, model.ErrVoucherInvalid) {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
	}
	if _, err := s.RedeemPairingVoucher(ip, unknown, tvKey, "TV", "Android TV 14"); !errors.Is(err, model.ErrTooManyAttempts) {
		t.Errorf("attempt %d: %v, want too many attempts", redeemsPerMin+1, err)
	}
	if _, err := s.RedeemPairingVoucher(model.WithClientIP(ctx, "192.0.2.11"), unknown, tvKey, "TV", "Android TV 14"); !errors.Is(err, model.ErrVoucherInvalid) {
		t.Errorf("another address: %v", err)
	}
	for i := range vouchersPerMin {
		if _, err := s.CreatePairingVoucher(ctx, phone, tvKey); err != nil {
			t.Fatalf("voucher %d: %v", i+1, err)
		}
	}
	if _, err := s.CreatePairingVoucher(ctx, phone, tvKey); !errors.Is(err, model.ErrTooManyAttempts) {
		t.Errorf("voucher %d: %v, want too many attempts", vouchersPerMin+1, err)
	}
	clock.advance(time.Minute)
	if _, err := s.CreatePairingVoucher(ctx, phone, tvKey); err != nil {
		t.Errorf("after a minute: %v", err)
	}
}

// The cleanup removes expired vouchers.
func TestPairingCleanup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, clock, phone, _ := pairingOwner(t)
	_, tvKey := newTVKey(t)
	if _, err := s.CreatePairingVoucher(ctx, phone, tvKey); err != nil {
		t.Fatal(err)
	}
	clock.advance(VoucherLifetime)
	if _, err := s.DeleteExpiredSessions(ctx); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM pairing_vouchers`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("%d vouchers left after the cleanup", n)
	}
}

// vectorFile is the shared format of the crypto test vectors (slouchcat-crypto-vectors-v1).
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

func vectorBytes(t *testing.T, v any) []byte {
	t.Helper()
	s, ok := v.(string)
	if !ok {
		t.Fatalf("%v is not a base64url string", v)
	}
	b, err := b64.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// The sealed token of testdata/crypto/hpke.json opens with the suite and info of sealRedeem, and
// what sealRedeem seals opens with the vector's key into the same shape.
func TestRedeemVector(t *testing.T) {
	t.Parallel()
	f := readVectors(t, "hpke.json", "hpke-base-single-shot")
	found := false
	for _, c := range f.Cases {
		if string(vectorBytes(t, c.Input["info"])) != RedeemInfo {
			continue
		}
		found = true
		priv, err := ecdh.X25519().NewPrivateKey(vectorBytes(t, c.Input["recipientPrivateKey"]))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(priv.PublicKey().Bytes(), vectorBytes(t, c.Input["recipientPublicKey"])) {
			t.Fatal("public key does not belong to the private key")
		}
		sk, _ := hpke.NewDHKEMPrivateKey(priv)
		plain, err := hpke.Open(sk, pairingKDF, pairingAEAD, []byte(RedeemInfo), vectorBytes(t, c.Input["sealed"]))
		if err != nil {
			t.Fatalf("%s: %v", c.Name, err)
		}
		if !bytes.Equal(plain, vectorBytes(t, c.Expected["plaintext"])) {
			t.Fatalf("%s: plaintext %s", c.Name, plain)
		}
		if len(plain) == 0 {
			continue // the case of the empty plaintext checks the suite only
		}
		sealed, err := sealRedeem(priv.PublicKey().Bytes(), "example-device-token-0123456789", "dev_example")
		if err != nil {
			t.Fatal(err)
		}
		token, device := openRedeem(t, priv, sealed)
		var want struct {
			Token    string `json:"token"`
			DeviceID string `json:"deviceId"`
		}
		if err := json.Unmarshal(plain, &want); err != nil {
			t.Fatal(err)
		}
		if token != want.Token || device != want.DeviceID {
			t.Errorf("round trip %q %q, vector %q %q", token, device, want.Token, want.DeviceID)
		}
	}
	if !found {
		t.Fatal("hpke.json has no case with info slouchcat-redeem-v1")
	}
}

// proveServer signs prefix and nonce; the signature checks against getHealth's key, a flipped bit
// does not, a nonce of another length is refused, and proofs count per client address.
func TestServerProof(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, _ := newService(t)
	if s.ServerKey() != "" {
		t.Fatal("a key without identity")
	}
	seed := make([]byte, ed25519.SeedSize)
	_, _ = rand.Read(seed)
	s.WithServerIdentity(seed)
	pub := ed25519.PublicKey(vectorBytes(t, s.ServerKey()))
	nonce := make([]byte, NonceBytes)
	_, _ = rand.Read(nonce)
	ip := model.WithClientIP(ctx, "192.0.2.20")
	sig, err := s.ProveServer(ip, nonce)
	if err != nil {
		t.Fatal(err)
	}
	msg := append([]byte("slouchcat-server-proof-v1:"), nonce...)
	if !ed25519.Verify(pub, msg, sig) {
		t.Fatal("signature does not verify")
	}
	bad := bytes.Clone(sig)
	bad[0] ^= 1
	if ed25519.Verify(pub, msg, bad) {
		t.Error("flipped signature verifies")
	}
	other := bytes.Clone(nonce)
	other[0] ^= 1
	if ed25519.Verify(pub, append([]byte(ServerProofPrefix), other...), sig) {
		t.Error("signature verifies for another nonce")
	}
	var verr *model.ValidationError
	if _, err := s.ProveServer(ip, nonce[:31]); !errors.As(err, &verr) {
		t.Errorf("short nonce: %v", err)
	}
	for range proofsPerMin - 2 {
		if _, err := s.ProveServer(ip, nonce); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.ProveServer(ip, nonce); !errors.Is(err, model.ErrTooManyAttempts) {
		t.Errorf("proof %d: %v, want too many attempts", proofsPerMin+1, err)
	}
	if _, err := s.ProveServer(model.WithClientIP(ctx, "192.0.2.21"), nonce); err != nil {
		t.Errorf("another address: %v", err)
	}
}

// The key follows secret.key: the same file gives the same key after a restart, a new file a new
// key (concept 27 section 6: a new installation or a backup without secrets changes the key).
func TestServerKeyFollowsTheSecret(t *testing.T) {
	t.Parallel()
	key := func(dir string) string {
		t.Helper()
		s, _ := newService(t)
		ring, err := secrets.Load(dir, s.log)
		if err != nil {
			t.Fatal(err)
		}
		return s.WithServerIdentity(ring.Derive("server-identity")).ServerKey()
	}
	dir := t.TempDir()
	first := key(dir)
	if len(vectorBytes(t, first)) != ed25519.PublicKeySize {
		t.Fatalf("key %q", first)
	}
	if again := key(dir); again != first {
		t.Errorf("after a restart %q, before %q", again, first)
	}
	if fresh := key(t.TempDir()); fresh == first {
		t.Error("a new secret kept the key")
	}
}

// testdata/crypto/ed25519.json: the key of the seed, the message of proveServer and the signature,
// which Ed25519 makes deterministically.
func TestEd25519Vectors(t *testing.T) {
	t.Parallel()
	f := readVectors(t, "ed25519.json", "ed25519-verify")
	for _, c := range f.Cases {
		t.Run(c.Name, func(t *testing.T) {
			s, _ := newService(t)
			s.WithServerIdentity(vectorBytes(t, c.Input["privateSeed"]))
			pub, nonce := vectorBytes(t, c.Input["publicKey"]), vectorBytes(t, c.Input["nonce"])
			if s.ServerKey() != b64.EncodeToString(pub) {
				t.Fatalf("key %s, vector %s", s.ServerKey(), b64.EncodeToString(pub))
			}
			msg := vectorBytes(t, c.Input["message"])
			if !bytes.Equal(msg, append([]byte(ServerProofPrefix), nonce...)) {
				t.Fatal("the vector's message is not prefix and nonce")
			}
			sig := vectorBytes(t, c.Input["signature"])
			valid, _ := c.Expected["valid"].(bool)
			if got := ed25519.Verify(pub, msg, sig); got != valid {
				t.Fatalf("verify = %v, want %v", got, valid)
			}
			mine, err := s.ProveServer(context.Background(), nonce)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(mine, sig) != valid {
				t.Errorf("proveServer signed %x, vector %x (valid %v)", mine, sig, valid)
			}
		})
	}
}

// testdata/crypto/pbkdf2.json: the verifier of the offline PIN (web/src/features/offline/pin.ts)
// is the same in the server.
func TestPBKDF2Vectors(t *testing.T) {
	t.Parallel()
	f := readVectors(t, "pbkdf2.json", "pbkdf2-hmac-sha256")
	for _, c := range f.Cases {
		t.Run(c.Name, func(t *testing.T) {
			if it, _ := c.Input["iterations"].(float64); int(it) != model.OfflinePinIterations {
				t.Fatalf("iterations %v, the server uses %d", it, model.OfflinePinIterations)
			}
			if n, _ := c.Input["length"].(float64); int(n) != offlineKeyBytes {
				t.Fatalf("length %v, the server uses %d", n, offlineKeyBytes)
			}
			v, err := offlinePinVerifier(string(vectorBytes(t, c.Input["password"])), vectorBytes(t, c.Input["salt"]))
			if err != nil {
				t.Fatal(err)
			}
			if want := b64.EncodeToString(vectorBytes(t, c.Expected["hash"])); v.Hash != want {
				t.Errorf("hash %s, vector %s", v.Hash, want)
			}
		})
	}
}
