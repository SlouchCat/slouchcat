package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// appLoginRequest and appLogin are the body and the answer of appLogin (contract 1.42.0).
type appLoginRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	DeviceName string `json:"deviceName"`
	Kind       string `json:"kind"`
}

type appLogin struct {
	Token    string      `json:"token"`
	DeviceID string      `json:"deviceId"`
	Session  sessionInfo `json:"session"`
}

type healthVersion struct {
	ContractVersion string `json:"contractVersion"`
}

// checkAppLogin signs in like the phone app (concept 27, ADR 0040): appLogin answers a bearer
// token without a cookie, the token opens the account's session, the administration stays closed
// to it (403 client_not_allowed), getHealth names the contract version, and logout ends the token.
func checkAppLogin(ctx context.Context, s *state) (string, error) {
	resp, err := s.c.sendApp(ctx, http.MethodPost, "/auth/app-login", "",
		appLoginRequest{Email: ownerEmail, Password: ownerPassword, DeviceName: "apicheck", Kind: "phone"})
	if err != nil {
		return "", err
	}
	if err := resp.expect(http.MethodPost, "/auth/app-login", http.StatusOK); err != nil {
		return "", err
	}
	if len(resp.header.Values("Set-Cookie")) != 0 {
		return "", errors.New("appLogin set a cookie")
	}
	var login appLogin
	if err := json.Unmarshal(resp.body, &login); err != nil {
		return "", fmt.Errorf("decode appLogin: %w", err)
	}
	if login.Token == "" || len(login.DeviceID) != 32 || login.Session.Account.Email != ownerEmail {
		return "", fmt.Errorf("appLogin answered device %q for %q", login.DeviceID, login.Session.Account.Email)
	}
	if resp, err = s.c.sendApp(ctx, http.MethodGet, "/auth/session", login.Token, nil); err != nil {
		return "", err
	}
	if err := resp.expect(http.MethodGet, "/auth/session", http.StatusOK); err != nil {
		return "", err
	}
	if resp, err = s.c.sendApp(ctx, http.MethodGet, "/admin/devices", login.Token, nil); err != nil {
		return "", err
	}
	var p problem
	if err := json.Unmarshal(resp.body, &p); resp.status != http.StatusForbidden || err != nil || p.Code != "client_not_allowed" {
		return "", fmt.Errorf("listDevices with the app token answered %d %s", resp.status, p.Code)
	}
	if resp, err = s.c.sendApp(ctx, http.MethodGet, "/healthz", "", nil); err != nil {
		return "", err
	}
	var h healthVersion
	if err := json.Unmarshal(resp.body, &h); err != nil || h.ContractVersion != s.c.contract.version {
		return "", fmt.Errorf("getHealth names the contract %q, the file says %q", h.ContractVersion, s.c.contract.version)
	}
	if resp, err = s.c.sendApp(ctx, http.MethodPost, "/auth/logout", login.Token, nil); err != nil {
		return "", err
	}
	if err := resp.expect(http.MethodPost, "/auth/logout", http.StatusNoContent); err != nil {
		return "", err
	}
	if resp, err = s.c.sendApp(ctx, http.MethodGet, "/auth/session", login.Token, nil); err != nil {
		return "", err
	}
	if err := resp.expect(http.MethodGet, "/auth/session", http.StatusUnauthorized); err != nil {
		return "", fmt.Errorf("the token still works after logout: %w", err)
	}
	return fmt.Sprintf("App-Sitzung für %s, Vertrag %s, Verwaltung gesperrt, abgemeldet", login.Session.Account.Email, h.ContractVersion), nil
}
