package main

import (
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hpke"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// The TV alone (concept 27 stage 9, the flow of the phone app in the emulator and the acceptance with
// a real phone): this mode does what the TV app does when it shows its QR code (concept 26 4.3). It
// opens a meeting point at the relay, writes the content of the QR code to a file (-pair-tv) for the
// phone's scanner (or for a QR encoder in a terminal), waits for the phone's packet, opens it, checks
// the server with proveServer against the packet's key, redeems the voucher, checks the server's
// signature and opens the device token. Nothing secret is printed: neither the meeting point, the
// pickup secret, the voucher nor the token.

// pairTVOptions are the flags of -pair-tv.
type pairTVOptions struct {
	codeFile   string        // where the QR code's content goes (mode 0600)
	relay      string        // the relay as this program reaches it
	codeRelay  string        // the relay the code names, as the phone reaches it (default: relay)
	wantServer string        // the server address the packet must carry (empty: any)
	wantEmail  string        // the account the sealed answer must name (empty: any)
	expiry     time.Duration // how long to wait for the packet at most (the meeting point lives 5 minutes)
}

// runPairTV plays the TV until the voucher is redeemed and the device token opened.
func runPairTV(ctx context.Context, s *state, o pairTVOptions) error {
	if o.relay == "" {
		return errors.New("-pair-tv needs -relay")
	}
	if o.codeRelay == "" {
		o.codeRelay = o.relay
	}
	tv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	tvKey := b64url.EncodeToString(tv.PublicKey().Bytes())
	s.relay = o.relay
	meeting, err := s.openMeetingPoint(ctx)
	if err != nil {
		return err
	}
	code := strings.TrimRight(o.codeRelay, "/") + "/tv#v=1&i=" + meeting.id + "&k=" + tvKey
	if err := os.WriteFile(o.codeFile, []byte(code+"\n"), 0o600); err != nil {
		return fmt.Errorf("write the code: %w", err)
	}
	fmt.Println("apicheck: TV shows its code (written to the file)")

	sealed, err := meeting.awaitPacket(ctx, o.expiry)
	if err != nil {
		return err
	}
	sk, err := hpke.NewDHKEMPrivateKey(tv)
	if err != nil {
		return err
	}
	opened, err := hpke.Open(sk, pairKDF, pairAEAD, []byte(packetInfoPrefix+meeting.id), sealed)
	if err != nil {
		return fmt.Errorf("TV: the packet does not open with the TV's key and meeting point: %w", err)
	}
	var packet pairingPacket
	if err := json.Unmarshal(opened, &packet); err != nil || packet.V != 1 || len(packet.Voucher) != 43 {
		// Never the packet itself: it carries the voucher (security review L-7).
		return fmt.Errorf("TV: packet v=%d with a voucher of %d characters", packet.V, len(packet.Voucher))
	}
	fmt.Printf("apicheck: TV opened the packet: server %s\n", packet.Server)
	if o.wantServer != "" && strings.TrimRight(packet.Server, "/") != strings.TrimRight(o.wantServer, "/") {
		return fmt.Errorf("TV: the packet names the server %q, want %q", packet.Server, o.wantServer)
	}
	if err := s.proveServer(ctx, packet.ServerKey); err != nil {
		return err
	}
	redeem := map[string]string{"voucher": packet.Voucher, "tvKey": tvKey, "deviceName": "apicheck TV", "systemVersion": "Android TV 14"}
	resp, err := s.c.sendTV(ctx, http.MethodPost, "/auth/pairing/redeem", "", redeem)
	if err != nil {
		return err
	}
	if err := resp.expectSecret(http.MethodPost, "/auth/pairing/redeem", http.StatusOK); err != nil {
		return err
	}
	var redeemed pairingRedeemed
	if err := json.Unmarshal(resp.body, &redeemed); err != nil {
		return errors.New("redeemPairingVoucher answered no sealed token")
	}
	raw, err := b64url.DecodeString(redeemed.SealedToken)
	if err != nil {
		return fmt.Errorf("sealedToken: %w", err)
	}
	keyBytes, _ := b64url.DecodeString(packet.ServerKey)
	sig, _ := b64url.DecodeString(redeemed.Signature)
	if len(keyBytes) != ed25519.PublicKeySize || !ed25519.Verify(keyBytes, append([]byte(redeemSignaturePrefix), raw...), sig) {
		return errors.New("TV: the redeem answer's signature does not verify with the packet's serverKey")
	}
	tokenPlain, err := hpke.Open(sk, pairKDF, pairAEAD, []byte(redeemInfo), raw)
	if err != nil {
		return fmt.Errorf("TV: open the sealed token: %w", err)
	}
	var device sealedAnswer
	if err := json.Unmarshal(tokenPlain, &device); err != nil || device.Token == "" || len(device.DeviceID) != 32 {
		return errors.New("TV: sealed token without token or device")
	}
	if o.wantEmail != "" && device.AccountEmail != o.wantEmail {
		return fmt.Errorf("TV: the sealed answer names another account (%d characters)", len(device.AccountEmail))
	}
	// The token works: the session of the new device answers.
	if resp, err = s.c.sendTV(ctx, http.MethodGet, "/auth/session", device.Token, nil); err != nil {
		return err
	}
	if err := resp.expectSecret(http.MethodGet, "/auth/session", http.StatusOK); err != nil {
		return fmt.Errorf("TV: the new device token: %w", err)
	}
	fmt.Printf("apicheck: TV paired with the account %s\n", device.AccountEmail)
	return nil
}

// awaitPacket asks the relay every 2 seconds, as the TV does (build log «Protokolle»), until the
// phone's packet is there or within has passed.
func (m *meetingPoint) awaitPacket(ctx context.Context, within time.Duration) ([]byte, error) {
	at := m.s.relay + "/v1/pair/" + m.id
	auth := "Pickup " + m.secret
	deadline := time.Now().Add(within)
	for {
		var picked struct {
			Packet string `json:"packet"`
		}
		status, err := relayPoll(ctx, at, auth, &picked)
		if err != nil {
			return nil, fmt.Errorf("relay: ask for the packet: %w", err)
		}
		if status == http.StatusOK {
			out, err := b64url.DecodeString(picked.Packet)
			if err != nil {
				return nil, errors.New("relay: the packet is no base64url")
			}
			return out, nil
		}
		if status != http.StatusNoContent {
			return nil, fmt.Errorf("relay: the meeting point answered %d", status)
		}
		if time.Now().After(deadline) {
			_ = relayCall(ctx, http.MethodDelete, at, auth, nil, http.StatusNoContent, nil)
			return nil, fmt.Errorf("no packet within %s", within)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

// relayPoll is one GET of the meeting point: its status, with 200 the packet in out.
func relayPoll(ctx context.Context, target, authorization string, out any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", authorization)
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, nil
	}
	return http.StatusOK, json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(out)
}
