package main

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hpke"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// The device check plays a phone and a TV (concept 27 sections 6, 7.2, 10 and 13, stage 1; ADR
// 0040): the phone signs in with appLogin, chooses its profile, sees its device, and issues a
// voucher for the TV's one-time key; the TV takes the packet (through a running pairing relay with
// -relay, else handed over in memory), checks the server with proveServer, redeems the voucher,
// opens the sealed token, chooses a profile, calls what a TV may and gets 403 client_not_allowed on
// what it may not. Only the standard library: the formats are those of «Protokolle» in the build
// log, which relay/internal/pairing and the server implement as well.

// Protocol constants of the pairing and the proof.
const (
	packetInfoPrefix  = "slouchcat-pair-v1:"
	redeemInfo        = "slouchcat-redeem-v1"
	serverProofPrefix = "slouchcat-server-proof-v1:"
	// redeemSignaturePrefix precedes sealedToken in what the server signs for the TV.
	redeemSignaturePrefix = "slouchcat-redeem-signature-v1:"
)

// deviceCheckName is the row of the device check (-devices runs it alone).
const deviceCheckName = "Handy und Fernseher koppeln (Bearer)"

var b64url = base64.RawURLEncoding

type pairingVoucher struct {
	Voucher   string    `json:"voucher"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type pairingRedeemed struct {
	SealedToken string `json:"sealedToken"`
	Signature   string `json:"signature"`
}

// sealedAnswer is the plaintext of sealedToken.
type sealedAnswer struct {
	Token        string `json:"token"`
	DeviceID     string `json:"deviceId"`
	AccountEmail string `json:"accountEmail"`
}

type pairingPacket struct {
	V         int    `json:"v"`
	Server    string `json:"server"`
	Voucher   string `json:"voucher"`
	ServerKey string `json:"serverKey,omitempty"`
}

type healthKey struct {
	ServerKey string `json:"serverKey"`
}

type deviceRow struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

var (
	pairKEM  = hpke.DHKEM(ecdh.X25519())
	pairKDF  = hpke.HKDFSHA256()
	pairAEAD = hpke.AES256GCM()
)

// checkDevices runs the whole pairing as phone and TV.
func checkDevices(ctx context.Context, s *state) (string, error) {
	var h healthKey
	resp, err := s.c.sendTV(ctx, http.MethodGet, "/healthz", "", nil)
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(resp.body, &h); err != nil || len(h.ServerKey) != 43 {
		return "", fmt.Errorf("getHealth names no serverKey: %s", snippet(resp.body))
	}

	// The phone: appLogin, profile with PIN, its own device.
	phone, err := appSignIn(ctx, s, "apicheck Handy", "phone")
	if err != nil {
		return "", err
	}
	if err := s.chooseProfile(ctx, s.c.sendApp, phone.Token); err != nil {
		return "", fmt.Errorf("phone: %w", err)
	}

	// The TV opens a meeting point and shows its key (the QR code).
	tv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return "", err
	}
	tvKey := b64url.EncodeToString(tv.PublicKey().Bytes())
	meeting, err := s.openMeetingPoint(ctx)
	if err != nil {
		return "", err
	}

	// The phone issues the voucher and seals the packet for the TV.
	if resp, err = s.c.sendApp(ctx, http.MethodPost, "/account/pairing-vouchers", phone.Token, map[string]string{"tvKey": tvKey}); err != nil {
		return "", err
	}
	if err := resp.expect(http.MethodPost, "/account/pairing-vouchers", http.StatusCreated); err != nil {
		return "", err
	}
	var v pairingVoucher
	if err := json.Unmarshal(resp.body, &v); err != nil || len(v.Voucher) != 43 {
		return "", fmt.Errorf("createPairingVoucher answered no voucher of 43 characters (%d bytes)", len(resp.body))
	}
	plain, _ := json.Marshal(pairingPacket{V: 1, Server: s.c.base.String(), Voucher: v.Voucher, ServerKey: h.ServerKey})
	pk, err := pairKEM.NewPublicKey(tv.PublicKey().Bytes())
	if err != nil {
		return "", err
	}
	sealed, err := hpke.Seal(pk, pairKDF, pairAEAD, []byte(packetInfoPrefix+meeting.id), plain)
	if err != nil {
		return "", err
	}
	got, err := meeting.handOver(ctx, sealed)
	if err != nil {
		return "", err
	}

	// The TV opens the packet, checks the server and redeems.
	sk, err := hpke.NewDHKEMPrivateKey(tv)
	if err != nil {
		return "", err
	}
	opened, err := hpke.Open(sk, pairKDF, pairAEAD, []byte(packetInfoPrefix+meeting.id), got)
	if err != nil {
		return "", fmt.Errorf("TV: open the packet: %w", err)
	}
	var packet pairingPacket
	if err := json.Unmarshal(opened, &packet); err != nil || packet.V != 1 || packet.Voucher != v.Voucher {
		// Never the packet itself: it carries the voucher (security review L-7).
		return "", fmt.Errorf("TV: packet v=%d, voucher matches %t, server %q", packet.V, packet.Voucher == v.Voucher, packet.Server)
	}
	if err := s.proveServer(ctx, packet.ServerKey); err != nil {
		return "", err
	}
	redeem := map[string]string{"voucher": packet.Voucher, "tvKey": tvKey, "deviceName": "apicheck TV", "systemVersion": "Android TV 14"}
	if resp, err = s.c.sendTV(ctx, http.MethodPost, "/auth/pairing/redeem", "", redeem); err != nil {
		return "", err
	}
	if err := resp.expect(http.MethodPost, "/auth/pairing/redeem", http.StatusOK); err != nil {
		return "", err
	}
	if len(resp.header.Values("Set-Cookie")) != 0 {
		return "", errors.New("redeemPairingVoucher set a cookie")
	}
	var redeemed pairingRedeemed
	if err := json.Unmarshal(resp.body, &redeemed); err != nil || strings.Contains(string(resp.body), "@") {
		return "", errors.New("redeemPairingVoucher answered no sealed token, or the e-mail in the clear")
	}
	raw, err := b64url.DecodeString(redeemed.SealedToken)
	if err != nil {
		return "", fmt.Errorf("sealedToken: %w", err)
	}
	// The TV checks the server's signature over the sealed answer with the key of the packet.
	keyBytes, _ := b64url.DecodeString(packet.ServerKey)
	sig, _ := b64url.DecodeString(redeemed.Signature)
	if len(keyBytes) != ed25519.PublicKeySize || !ed25519.Verify(keyBytes, append([]byte(redeemSignaturePrefix), raw...), sig) {
		return "", errors.New("TV: the redeem answer's signature does not verify with the packet's serverKey")
	}
	tokenPlain, err := hpke.Open(sk, pairKDF, pairAEAD, []byte(redeemInfo), raw)
	if err != nil {
		return "", fmt.Errorf("TV: open the sealed token: %w", err)
	}
	var device sealedAnswer
	if err := json.Unmarshal(tokenPlain, &device); err != nil || device.Token == "" || len(device.DeviceID) != 32 {
		return "", fmt.Errorf("TV: sealed token without token or device")
	}
	if device.AccountEmail != ownerEmail {
		return "", fmt.Errorf("TV: the sealed answer names another account (%d characters)", len(device.AccountEmail))
	}
	if resp, err = s.c.sendTV(ctx, http.MethodPost, "/auth/pairing/redeem", "", redeem); err != nil {
		return "", err
	}
	if err := expectProblem(resp, http.StatusGone, "voucher_invalid"); err != nil {
		return "", fmt.Errorf("second redeem: %w", err)
	}

	// The TV chooses its profile and calls what it may, and not more.
	if err := s.chooseProfile(ctx, s.c.sendTV, device.Token); err != nil {
		return "", fmt.Errorf("TV: %w", err)
	}
	if resp, err = s.c.sendTV(ctx, http.MethodGet, "/home", device.Token, nil); err != nil {
		return "", err
	}
	if err := resp.expect(http.MethodGet, "/home", http.StatusOK); err != nil {
		return "", fmt.Errorf("TV: %w", err)
	}
	for _, forbidden := range []struct{ method, path string }{
		{http.MethodPost, "/account/pairing-vouchers"}, {http.MethodGet, "/me/channels"}, {http.MethodGet, "/admin/devices"},
	} {
		var body any
		if forbidden.method == http.MethodPost {
			body = map[string]string{"tvKey": tvKey}
		}
		if resp, err = s.c.sendTV(ctx, forbidden.method, forbidden.path, device.Token, body); err != nil {
			return "", err
		}
		if err := expectProblem(resp, http.StatusForbidden, "client_not_allowed"); err != nil {
			return "", fmt.Errorf("TV %s %s: %w", forbidden.method, forbidden.path, err)
		}
	}
	if err := s.seeDevices(ctx, phone.Token, map[string]string{phone.DeviceID: "phone", device.DeviceID: "tv"}); err != nil {
		return "", fmt.Errorf("the phone's account: %w", err)
	}

	// The phone signs the TV out under «Meine Geräte»; its token stops working at once.
	tvPath := "/account/devices/" + device.DeviceID
	if resp, err = s.c.sendApp(ctx, http.MethodDelete, tvPath, phone.Token, nil); err != nil {
		return "", err
	}
	if err := resp.expect(http.MethodDelete, tvPath, http.StatusNoContent); err != nil {
		return "", err
	}
	if resp, err = s.c.sendTV(ctx, http.MethodGet, "/auth/session", device.Token, nil); err != nil {
		return "", err
	}
	if err := resp.expect(http.MethodGet, "/auth/session", http.StatusUnauthorized); err != nil {
		return "", fmt.Errorf("the TV token after signOutMyDevice: %w", err)
	}
	if resp, err = s.c.sendApp(ctx, http.MethodPost, "/auth/logout", phone.Token, nil); err != nil {
		return "", err
	}
	if err := resp.expect(http.MethodPost, "/auth/logout", http.StatusNoContent); err != nil {
		return "", err
	}
	via := "Übergabe im Speicher"
	if s.relay != "" {
		via = "Vermittlung " + s.relay
	}
	return fmt.Sprintf("Handy und Fernseher für %s gekoppelt (%s), Server geprüft, Fernseher nur mit seiner Erlaubnisliste, unter «Meine Geräte» abgemeldet (401)",
		device.AccountEmail, via), nil
}

// appSignIn signs in with appLogin.
func appSignIn(ctx context.Context, s *state, name, kind string) (appLogin, error) {
	resp, err := s.c.sendApp(ctx, http.MethodPost, "/auth/app-login", "",
		appLoginRequest{Email: ownerEmail, Password: ownerPassword, DeviceName: name, Kind: kind})
	if err != nil {
		return appLogin{}, err
	}
	if err := resp.expect(http.MethodPost, "/auth/app-login", http.StatusOK); err != nil {
		return appLogin{}, err
	}
	var login appLogin
	if err := json.Unmarshal(resp.body, &login); err != nil || login.Token == "" {
		return appLogin{}, fmt.Errorf("decode appLogin: no token in %d bytes", len(resp.body))
	}
	return login, nil
}

// chooseProfile picks the primary profile of the account (the fixtures gave it the PIN).
func (s *state) chooseProfile(ctx context.Context, send func(context.Context, string, string, string, any) (*response, error), token string) error {
	resp, err := send(ctx, http.MethodGet, "/auth/profiles", token, nil)
	if err != nil {
		return err
	}
	if err := resp.expect(http.MethodGet, "/auth/profiles", http.StatusOK); err != nil {
		return err
	}
	var profiles []profileSummary
	if err := json.Unmarshal(resp.body, &profiles); err != nil {
		return fmt.Errorf("decode listMyProfiles: %w", err)
	}
	for _, p := range profiles {
		if !p.IsPrimary {
			continue
		}
		var body any = map[string]string{}
		if p.HasPin {
			body = pinRequest{Pin: ownerPin}
		}
		path := "/auth/profiles/" + strconv.FormatInt(p.ID, 10) + "/select"
		if resp, err = send(ctx, http.MethodPost, path, token, body); err != nil {
			return err
		}
		return resp.expect(http.MethodPost, path, http.StatusOK)
	}
	return errors.New("the account has no primary profile")
}

// seeDevices checks that «Meine Geräte» (listMyDevices) of the phone's account lists the devices
// (id to type).
func (s *state) seeDevices(ctx context.Context, appToken string, want map[string]string) error {
	resp, err := s.c.sendApp(ctx, http.MethodGet, "/account/devices", appToken, nil)
	if err != nil {
		return err
	}
	if err := resp.expect(http.MethodGet, "/account/devices", http.StatusOK); err != nil {
		return err
	}
	var rows []deviceRow
	if err := json.Unmarshal(resp.body, &rows); err != nil {
		return fmt.Errorf("decode listMyDevices: %w", err)
	}
	for id, kind := range want {
		found := false
		for _, r := range rows {
			if r.ID == id {
				if r.Type != kind {
					return fmt.Errorf("the %s device has type %q", kind, r.Type)
				}
				found = true
			}
		}
		if !found {
			return fmt.Errorf("«Meine Geräte» lacks the %s device", kind)
		}
	}
	return nil
}

// proveServer checks the server like the TV before it sends the voucher: a fresh nonce, signed
// with the key from the packet.
func (s *state) proveServer(ctx context.Context, serverKey string) error {
	pub, err := b64url.DecodeString(serverKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("TV: the packet's serverKey %q", serverKey)
	}
	nonce := make([]byte, 32)
	_, _ = rand.Read(nonce)
	resp, err := s.c.sendTV(ctx, http.MethodPost, "/server/prove", "", map[string]string{"nonce": b64url.EncodeToString(nonce)})
	if err != nil {
		return err
	}
	if err := resp.expect(http.MethodPost, "/server/prove", http.StatusOK); err != nil {
		return err
	}
	var proof struct {
		Signature string `json:"signature"`
		ServerKey string `json:"serverKey"`
	}
	if err := json.Unmarshal(resp.body, &proof); err != nil {
		return fmt.Errorf("decode proveServer: %w", err)
	}
	sig, err := b64url.DecodeString(proof.Signature)
	if err != nil || proof.ServerKey != serverKey || !ed25519.Verify(pub, append([]byte(serverProofPrefix), nonce...), sig) {
		return errors.New("TV: proveServer does not verify with the key of the packet")
	}
	return nil
}

func expectProblem(resp *response, status int, code string) error {
	var p problem
	if err := json.Unmarshal(resp.body, &p); resp.status != status || err != nil || p.Code != code {
		return fmt.Errorf("answered %d %s, want %d %s", resp.status, snippet(resp.body), status, code)
	}
	return nil
}

// meetingPoint is the hand-over of the packet: at a running pairing relay (relay set) or in memory.
type meetingPoint struct {
	s      *state
	id     string
	secret string // pickup secret of the TV
}

// openMeetingPoint opens a meeting point like the TV: POST /v1/pair with the hash of the pickup
// secret. Without a relay the id is a fresh random one.
func (s *state) openMeetingPoint(ctx context.Context) (*meetingPoint, error) {
	secret := make([]byte, 32)
	_, _ = rand.Read(secret)
	m := &meetingPoint{s: s, secret: b64url.EncodeToString(secret)}
	if s.relay == "" {
		id := make([]byte, 16)
		_, _ = rand.Read(id)
		m.id = b64url.EncodeToString(id)
		return m, nil
	}
	sum := sha256.Sum256(secret)
	var opened struct {
		ID string `json:"id"`
	}
	if err := relayCall(ctx, http.MethodPost, s.relay+"/v1/pair", "", map[string]string{"pickupHash": b64url.EncodeToString(sum[:])}, http.StatusCreated, &opened); err != nil {
		return nil, fmt.Errorf("relay: open the meeting point: %w", err)
	}
	if len(opened.ID) != 22 {
		return nil, fmt.Errorf("relay: meeting point id of %d characters", len(opened.ID))
	}
	m.id = opened.ID
	return m, nil
}

// handOver lays the phone's packet down and picks it up as the TV; through the relay the TV first
// sees 204 (nothing yet), then the packet, and deletes the meeting point.
func (m *meetingPoint) handOver(ctx context.Context, sealed []byte) ([]byte, error) {
	if m.s.relay == "" {
		return sealed, nil
	}
	at := m.s.relay + "/v1/pair/" + m.id
	auth := "Pickup " + m.secret
	if err := relayCall(ctx, http.MethodGet, at, auth, nil, http.StatusNoContent, nil); err != nil {
		return nil, fmt.Errorf("relay: empty meeting point: %w", err)
	}
	if err := relayCall(ctx, http.MethodPut, at, "", map[string]string{"packet": b64url.EncodeToString(sealed)}, http.StatusNoContent, nil); err != nil {
		return nil, fmt.Errorf("relay: lay the packet down: %w", err)
	}
	var picked struct {
		Packet string `json:"packet"`
	}
	if err := relayCall(ctx, http.MethodGet, at, auth, nil, http.StatusOK, &picked); err != nil {
		return nil, fmt.Errorf("relay: pick the packet up: %w", err)
	}
	if err := relayCall(ctx, http.MethodDelete, at, auth, nil, http.StatusNoContent, nil); err != nil {
		return nil, fmt.Errorf("relay: delete the meeting point: %w", err)
	}
	out, err := b64url.DecodeString(picked.Packet)
	if err != nil || !bytes.Equal(out, sealed) {
		return nil, errors.New("relay: the packet changed on the way")
	}
	return out, nil
}

// relayCall is one JSON request to the pairing relay (not part of the server's contract).
func relayCall(ctx context.Context, method, target, authorization string, body any, want int, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, _ := json.Marshal(body)
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode != want {
		var problem struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(data, &problem) // the code only: a body may carry the packet
		return fmt.Errorf("%s answered %d %q, want %d", method, resp.StatusCode, problem.Error, want)
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}
