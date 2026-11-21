package main

import (
	"context"
	"crypto/hpke"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// The phone alone (concept 26 stage 2a, the flow of the TV app in the emulator): the TV app shows
// its QR code, this mode reads its content (-pair-phone) and does what the phone app of stage 9 of
// concept 27 will do: appLogin, profile with PIN, createPairingVoucher for the TV's key, the packet
// sealed for that key and the meeting point, laid down at the relay. The TV takes it from there.
// When the TV redeemed the voucher, «Meine Geräte» of the account lists it; the run waits for that.

// pairCode is what the QR code of the TV carries: https://<relay>/tv#v=1&i=<id>&k=<key>.
type pairCode struct {
	relay string
	id    string
	tvKey string
}

// parsePairCode reads the content of the TV's QR code.
func parsePairCode(text string) (pairCode, error) {
	address, fragment, found := strings.Cut(text, "#")
	if !found {
		return pairCode{}, errors.New("the code has no part after #")
	}
	u, err := url.Parse(address)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || !strings.HasSuffix(u.Path, "/tv") {
		return pairCode{}, fmt.Errorf("the code is no address of a relay's /tv page: %q", address)
	}
	fields, err := url.ParseQuery(fragment)
	if err != nil || fields.Get("v") != "1" {
		return pairCode{}, errors.New("the code is not of version 1")
	}
	c := pairCode{relay: strings.TrimSuffix(address, "/tv"), id: fields.Get("i"), tvKey: fields.Get("k")}
	if len(c.id) != 22 || len(c.tvKey) != 43 {
		return pairCode{}, fmt.Errorf("the code names an id of %d and a key of %d characters", len(c.id), len(c.tvKey))
	}
	return c, nil
}

// phoneAccount is the account the phone signs in with (-pair-email, -pair-password, -pair-pin).
type phoneAccount struct {
	email, password, pin string
}

// runPairPhone plays the phone for a TV that shows code. relay (may be empty) replaces the relay of
// the code (the TV in the emulator reaches the host as 10.0.2.2, this program as 127.0.0.1);
// packetServer is the server address the packet carries (the one the TV reaches).
func runPairPhone(ctx context.Context, s *state, text, relay, packetServer string, account phoneAccount, wait time.Duration) error {
	code, err := parsePairCode(text)
	if err != nil {
		return err
	}
	if relay == "" {
		relay = code.relay
	}
	if err := s.waitForServer(ctx, wait); err != nil {
		return err
	}
	var h healthKey
	resp, err := s.c.sendTV(ctx, http.MethodGet, "/healthz", "", nil)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(resp.body, &h); err != nil || len(h.ServerKey) != 43 {
		return errors.New("getHealth names no serverKey")
	}
	phone, err := phoneSignIn(ctx, s, account)
	if err != nil {
		return err
	}
	before, err := s.deviceIDs(ctx, phone.Token)
	if err != nil {
		return err
	}
	if resp, err = s.c.sendApp(ctx, http.MethodPost, "/account/pairing-vouchers", phone.Token, map[string]string{"tvKey": code.tvKey}); err != nil {
		return err
	}
	if err := resp.expect(http.MethodPost, "/account/pairing-vouchers", http.StatusCreated); err != nil {
		return err
	}
	var v pairingVoucher
	if err := json.Unmarshal(resp.body, &v); err != nil || len(v.Voucher) != 43 {
		return errors.New("createPairingVoucher answered no voucher")
	}
	key, err := b64url.DecodeString(code.tvKey)
	if err != nil {
		return fmt.Errorf("the TV's key: %w", err)
	}
	pk, err := pairKEM.NewPublicKey(key)
	if err != nil {
		return fmt.Errorf("the TV's key: %w", err)
	}
	plain, _ := json.Marshal(pairingPacket{V: 1, Server: packetServer, Voucher: v.Voucher, ServerKey: h.ServerKey})
	sealed, err := hpke.Seal(pk, pairKDF, pairAEAD, []byte(packetInfoPrefix+code.id), plain)
	if err != nil {
		return err
	}
	if err := relayCall(ctx, http.MethodPut, relay+"/v1/pair/"+code.id, "", map[string]string{"packet": b64url.EncodeToString(sealed)}, http.StatusNoContent, nil); err != nil {
		return fmt.Errorf("relay: lay the packet down: %w", err)
	}
	fmt.Println("apicheck: packet laid down for the TV")

	// The TV redeems on its own; then «Meine Geräte» lists a new device of the type tv.
	deadline := time.Now().Add(2 * time.Minute)
	for {
		now, err := s.deviceIDs(ctx, phone.Token)
		if err != nil {
			return err
		}
		for id, kind := range now {
			if _, known := before[id]; !known && kind == "tv" {
				fmt.Println("apicheck: the TV redeemed the voucher and is listed under «Meine Geräte»")
				_, _ = s.c.sendApp(ctx, http.MethodPost, "/auth/logout", phone.Token, nil)
				return nil
			}
		}
		if time.Now().After(deadline) {
			return errors.New("the TV did not appear under «Meine Geräte» within 2 minutes")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

// deviceIDs is «Meine Geräte» of the account: device id to type.
func (s *state) deviceIDs(ctx context.Context, appToken string) (map[string]string, error) {
	resp, err := s.c.sendApp(ctx, http.MethodGet, "/account/devices", appToken, nil)
	if err != nil {
		return nil, err
	}
	if err := resp.expect(http.MethodGet, "/account/devices", http.StatusOK); err != nil {
		return nil, err
	}
	var rows []deviceRow
	if err := json.Unmarshal(resp.body, &rows); err != nil {
		return nil, fmt.Errorf("decode listMyDevices: %w", err)
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.ID] = r.Type
	}
	return out, nil
}

// phoneSignIn signs the phone in with account and chooses the primary profile with its PIN.
func phoneSignIn(ctx context.Context, s *state, account phoneAccount) (appLogin, error) {
	resp, err := s.c.sendApp(ctx, http.MethodPost, "/auth/app-login", "",
		appLoginRequest{Email: account.email, Password: account.password, DeviceName: "apicheck Handy", Kind: "phone"})
	if err != nil {
		return appLogin{}, err
	}
	if err := resp.expect(http.MethodPost, "/auth/app-login", http.StatusOK); err != nil {
		return appLogin{}, err
	}
	var login appLogin
	if err := json.Unmarshal(resp.body, &login); err != nil || login.Token == "" {
		return appLogin{}, errors.New("appLogin answered no token")
	}
	if resp, err = s.c.sendApp(ctx, http.MethodGet, "/auth/profiles", login.Token, nil); err != nil {
		return appLogin{}, err
	}
	var profiles []profileSummary
	if err := json.Unmarshal(resp.body, &profiles); err != nil {
		return appLogin{}, fmt.Errorf("decode listMyProfiles: %w", err)
	}
	for _, p := range profiles {
		if !p.IsPrimary {
			continue
		}
		var body any = map[string]string{}
		if p.HasPin {
			body = pinRequest{Pin: account.pin}
		}
		path := fmt.Sprintf("/auth/profiles/%d/select", p.ID)
		if resp, err = s.c.sendApp(ctx, http.MethodPost, path, login.Token, body); err != nil {
			return appLogin{}, err
		}
		return login, resp.expect(http.MethodPost, path, http.StatusOK)
	}
	return appLogin{}, errors.New("the account has no primary profile")
}
