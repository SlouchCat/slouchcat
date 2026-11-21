// Command paircheck plays a TV and a phone against a running meeting point service (concept 27
// sections 7.1, 7.2 and 13, stage 2): it opens a meeting point, drops a real HPKE packet, picks it
// up and decrypts it, and checks the answers 409, 410 and 404, the wrong pickup secret, the
// cancel, the health check and the help page. It is a tool of the move rehearsal (7.5, step 3).
//
//	paircheck -url https://pair.example.test [-ca test-ca.pem] [-connect 127.0.0.1:8443]
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/relay/internal/checkclient"
	"github.com/slouchcat/slouchcat/relay/internal/pairing"
)

func main() {
	base := flag.String("url", "http://127.0.0.1:8180", "base URL of the meeting point service")
	ca := flag.String("ca", "", "PEM file of an extra CA to trust (a local test CA)")
	connect := flag.String("connect", "", "dial this host:port instead of resolving the name in -url")
	flag.Parse()
	client, err := checkclient.New(*ca, *connect, 15*time.Second)
	if err != nil {
		fmt.Fprintln(os.Stderr, "paircheck:", err)
		os.Exit(2)
	}
	n, err := Run(client, strings.TrimRight(*base, "/"), os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "paircheck: FAILED after %d checks: %v\n", n, err)
		os.Exit(1)
	}
	fmt.Printf("paircheck: %d checks passed against %s\n", n, *base)
}

type checker struct {
	c    *http.Client
	base string
	out  io.Writer
	n    int
}

func (k *checker) do(method, path, pickup string, body any) (int, []byte, http.Header, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, k.base+path, rd) //nolint:noctx // the client has a timeout
	if err != nil {
		return 0, nil, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if pickup != "" {
		req.Header.Set("Authorization", "Pickup "+pickup)
	}
	resp, err := k.c.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	return resp.StatusCode, b, resp.Header, err
}

func (k *checker) expect(what, method, path, pickup string, body any, want int) ([]byte, http.Header, error) {
	status, b, h, err := k.do(method, path, pickup, body)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", what, err)
	}
	if status != want {
		return nil, nil, fmt.Errorf("%s: status %d, want %d (%s)", what, status, want, strings.TrimSpace(string(b)))
	}
	k.n++
	fmt.Fprintf(k.out, "ok   %-46s %d\n", what, status)
	return b, h, nil
}

type opened struct {
	ID        string    `json:"id"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func (k *checker) open(what, secret string) (string, error) {
	hash, err := pairing.PickupHash(secret)
	if err != nil {
		return "", err
	}
	b, _, err := k.expect(what, http.MethodPost, "/v1/pair", "", map[string]string{"pickupHash": hash}, http.StatusCreated)
	if err != nil {
		return "", err
	}
	var o opened
	if err := json.Unmarshal(b, &o); err != nil {
		return "", fmt.Errorf("%s: %w", what, err)
	}
	if raw, err := pairing.B64.DecodeString(o.ID); err != nil || len(raw) != 16 {
		return "", fmt.Errorf("%s: id %q is not 16 bytes in base64url", what, o.ID)
	}
	if d := time.Until(o.ExpiresAt); d <= 0 || d > 6*time.Minute {
		return "", fmt.Errorf("%s: expiresAt %s is not within five minutes", what, o.ExpiresAt)
	}
	return o.ID, nil
}

// Run performs every check and returns how many passed.
func Run(c *http.Client, base string, out io.Writer) (int, error) {
	k := &checker{c: c, base: base, out: out}
	err := k.run()
	return k.n, err
}

func (k *checker) run() error {
	if _, _, err := k.expect("health check", http.MethodGet, "/v1/healthz", "", nil, http.StatusOK); err != nil {
		return err
	}
	if err := k.helpPage(); err != nil {
		return err
	}

	// The TV: a one-time key and a pickup secret that never leaves it.
	tv, err := pairing.NewTVKey()
	if err != nil {
		return err
	}
	secret := pairing.NewSecret(32)
	id, err := k.open("TV opens a meeting point", secret)
	if err != nil {
		return err
	}
	path := "/v1/pair/" + id
	if _, _, err := k.expect("TV polls, nothing there yet", http.MethodGet, path, secret, nil, http.StatusNoContent); err != nil {
		return err
	}
	if _, _, err := k.expect("pick-up with a wrong secret", http.MethodGet, path, pairing.NewSecret(32), nil, http.StatusForbidden); err != nil {
		return err
	}
	if _, _, err := k.expect("pick-up without a secret", http.MethodGet, path, "", nil, http.StatusUnauthorized); err != nil {
		return err
	}

	// The phone: it read id and key from the QR code and seals the packet for them.
	serverPub, serverKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	want := pairing.Packet{V: pairing.Version, Server: "https://media.example.test", Voucher: pairing.NewSecret(32),
		ServerKey: pairing.B64.EncodeToString(serverPub)}
	sealed, err := pairing.SealPacket(tv.PublicKey().Bytes(), id, want)
	if err != nil {
		return err
	}
	packet := map[string]string{"packet": pairing.B64.EncodeToString(sealed)}
	if _, _, err := k.expect("phone drops the packet", http.MethodPut, path, "", packet, http.StatusNoContent); err != nil {
		return err
	}
	if _, _, err := k.expect("second packet is refused (occupied)", http.MethodPut, path, "", packet, http.StatusConflict); err != nil {
		return err
	}
	b, _, err := k.expect("TV picks the packet up", http.MethodGet, path, secret, nil, http.StatusOK)
	if err != nil {
		return err
	}
	var got struct {
		Packet string `json:"packet"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		return err
	}
	raw, err := pairing.B64.DecodeString(got.Packet)
	if err != nil {
		return err
	}
	p, err := pairing.OpenPacket(tv, id, raw)
	if err != nil {
		return fmt.Errorf("TV decrypts the packet: %w", err)
	}
	if p != want {
		return fmt.Errorf("TV decrypts the packet: got %+v, want %+v", p, want)
	}
	k.n++
	fmt.Fprintf(k.out, "ok   %-46s\n", "TV decrypts the packet")
	// The user's server answers the redeem (played here, the relay never sees it): the token sealed
	// for the TV with the account's e-mail, signed with the server key the packet named.
	answer := pairing.RedeemAnswer{Token: pairing.NewSecret(32), DeviceID: "0123456789abcdef0123456789abcdef", AccountEmail: "owner@example.test"}
	sealedToken, err := pairing.SealRedeem(tv.PublicKey().Bytes(), answer)
	if err != nil {
		return err
	}
	signature := pairing.SignRedeem(serverKey, sealedToken)
	packetKey, err := pairing.B64.DecodeString(p.ServerKey)
	if err != nil || !pairing.VerifyRedeem(packetKey, sealedToken, signature) {
		return errors.New("TV checks the redeem answer: the signature does not verify with the packet's serverKey")
	}
	opened, err := pairing.OpenRedeem(tv, sealedToken)
	if err != nil || opened != answer {
		return fmt.Errorf("TV opens the redeem answer: %w", errors.Join(err, errors.New("another answer")))
	}
	k.n++
	fmt.Fprintf(k.out, "ok   %-46s\n", "TV checks and opens the redeem answer")
	if _, _, err := k.expect("second pick-up (consumed)", http.MethodGet, path, secret, nil, http.StatusGone); err != nil {
		return err
	}
	if _, _, err := k.expect("packet after the pick-up (consumed)", http.MethodPut, path, "", packet, http.StatusGone); err != nil {
		return err
	}
	if _, _, err := k.expect("TV tidies up after the pick-up", http.MethodDelete, path, secret, nil, http.StatusNoContent); err != nil {
		return err
	}
	if _, _, err := k.expect("packet after pick-up and tidy-up (consumed)", http.MethodPut, path, "", packet, http.StatusGone); err != nil {
		return err
	}
	unknown := "/v1/pair/" + pairing.NewSecret(16)
	if _, _, err := k.expect("packet for an unknown id", http.MethodPut, unknown, "", packet, http.StatusNotFound); err != nil {
		return err
	}
	big := map[string]string{"packet": pairing.B64.EncodeToString(make([]byte, 4097))}
	if _, _, err := k.expect("packet over 4 KB", http.MethodPut, path, "", big, http.StatusRequestEntityTooLarge); err != nil {
		return err
	}

	// Cancel: only with the pickup secret, and afterwards the id is unknown.
	secret2 := pairing.NewSecret(32)
	id2, err := k.open("TV opens a second meeting point", secret2)
	if err != nil {
		return err
	}
	path2 := "/v1/pair/" + id2
	if _, _, err := k.expect("cancel with a wrong secret", http.MethodDelete, path2, secret, nil, http.StatusForbidden); err != nil {
		return err
	}
	if _, _, err := k.expect("TV cancels", http.MethodDelete, path2, secret2, nil, http.StatusNoContent); err != nil {
		return err
	}
	if _, _, err := k.expect("packet after the cancel (unknown)", http.MethodPut, path2, "", packet, http.StatusNotFound); err != nil {
		return err
	}
	return nil
}

func (k *checker) helpPage() error {
	for _, c := range []struct{ lang, text string }{
		{"de-CH,de;q=0.9", "Öffne die Slouchcat-App"},
		{"en-GB,en;q=0.8", "Open the Slouchcat app"},
	} {
		req, err := http.NewRequest(http.MethodGet, k.base+"/tv", nil) //nolint:noctx // the client has a timeout
		if err != nil {
			return err
		}
		req.Header.Set("Accept-Language", c.lang)
		resp, err := k.c.Do(req)
		if err != nil {
			return err
		}
		b, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		_ = resp.Body.Close()
		if err != nil {
			return err
		}
		csp := resp.Header.Get("Content-Security-Policy")
		switch {
		case resp.StatusCode != http.StatusOK:
			return fmt.Errorf("help page: status %d", resp.StatusCode)
		case !strings.Contains(string(b), c.text):
			return fmt.Errorf("help page (%s): text %q missing", c.lang, c.text)
		case strings.Contains(strings.ToLower(string(b)), "<script"):
			return errors.New("help page carries a script")
		case !strings.Contains(csp, "default-src 'none'") || strings.Contains(csp, "script-src"):
			return fmt.Errorf("help page: weak Content-Security-Policy %q", csp)
		case resp.Header.Get("Referrer-Policy") != "no-referrer":
			return errors.New("help page: Referrer-Policy is not no-referrer")
		}
		k.n++
		fmt.Fprintf(k.out, "ok   %-46s %d\n", "help page "+c.lang, resp.StatusCode)
	}
	return nil
}
