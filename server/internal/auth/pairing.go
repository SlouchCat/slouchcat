package auth

import (
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hpke"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
)

// Pairing of a TV (concept 27 section 10, K26 4.3 and 4.6, ADR 0040 «Kopplung») and the proof of
// the server's identity (concept 27 section 6, F-38, ADR 0040 «Server weist sich aus»).
//
// The phone app issues a voucher bound to the TV's one-time X25519 key; the TV redeems it with the
// same key and gets the token of a new device session back, sealed for that key with HPKE (RFC 9180).
// The formats are those of «Protokolle» in the build log; relay/internal/pairing implements the
// same and testdata/crypto/hpke.json pins them.

// Protocol constants («Protokolle»).
const (
	// RedeemInfo is the HPKE info of sealedToken.
	RedeemInfo = "slouchcat-redeem-v1"
	// ServerProofPrefix precedes the nonce in what proveServer signs.
	ServerProofPrefix = "slouchcat-server-proof-v1:"
	// VoucherLifetime is how long a pairing voucher can be redeemed (K-3).
	VoucherLifetime = 2 * time.Minute
	// NonceBytes is the length of the nonce of proveServer.
	NonceBytes = 32
	// maxSystemVersion is the length limit of the TV's system version (the user_agent of its device).
	maxSystemVersion = 80
)

// Rate limits of the pairing and the proof, each a pot of its own (auth.Limiter): vouchers per
// account, redeem attempts and proofs per client address. A phone issues one voucher per scan and a
// TV redeems once; an app proves the server once per new network.
const (
	vouchersPerMin = 6
	redeemsPerMin  = 10
	proofsPerMin   = 30
)

var (
	b64         = base64.RawURLEncoding
	pairingKEM  = hpke.DHKEM(ecdh.X25519())
	pairingKDF  = hpke.HKDFSHA256()
	pairingAEAD = hpke.AES256GCM()
)

// pairingLimits are the pots of the pairing and the proof.
type pairingLimits struct {
	vouchers, redeems, proofs *Limiter
}

func newPairingLimits() pairingLimits {
	return pairingLimits{
		vouchers: NewLimiter(vouchersPerMin, vouchersPerMin),
		redeems:  NewLimiter(redeemsPerMin, redeemsPerMin),
		proofs:   NewLimiter(proofsPerMin, proofsPerMin),
	}
}

// WithServerIdentity sets the seed of the server's Ed25519 key (32 bytes; main passes
// secrets.Keyring.Derive("server-identity"), so the key follows secret.key) and returns the
// receiver.
func (s *Service) WithServerIdentity(seed []byte) *Service {
	s.identity = ed25519.NewKeyFromSeed(seed)
	return s
}

// WithDeviceLinked runs fn after a TV was paired with an account (notify.DeviceLinked: the message
// «Neues Gerät verbunden» on the account's channels, K-7), with the device kind, the name and the
// system version the device reported and the time of the pairing. fn must not block; an error is
// logged. It returns the receiver.
func (s *Service) WithDeviceLinked(fn func(ctx context.Context, accountID int64, kind, name, system string, at time.Time) error) *Service {
	s.deviceLinked = fn
	return s
}

// ServerKey returns the server's Ed25519 public key in base64url; empty without an identity.
func (s *Service) ServerKey() string {
	if s.identity == nil {
		return ""
	}
	pub, _ := s.identity.Public().(ed25519.PublicKey)
	return b64.EncodeToString(pub)
}

// ProveServer signs ServerProofPrefix followed by nonce (NonceBytes bytes) with the server's key.
// Every call counts against the client address in ctx (model.ErrTooManyAttempts).
// *model.ValidationError for a nonce of another length.
func (s *Service) ProveServer(ctx context.Context, nonce []byte) ([]byte, error) {
	if err := s.pairing.proofs.Allow(model.ClientIP(ctx), s.now()); err != nil {
		return nil, err
	}
	if len(nonce) != NonceBytes {
		return nil, (&model.ValidationError{}).Add("nonce", "invalid_value")
	}
	if s.identity == nil {
		return nil, errors.New("server identity not set")
	}
	msg := append([]byte(ServerProofPrefix), nonce...)
	return ed25519.Sign(s.identity, msg), nil
}

// CreatePairingVoucher issues the voucher that pairs a TV with the session's account (K-3): only an
// app session (model.ErrAppSessionRequired), only without an active profile or with the primary
// profile (model.ErrPrimaryProfileRequired), bound to tvKey (the TV's X25519 key, base64url;
// *model.ValidationError otherwise), valid for VoucherLifetime and once. A new voucher replaces the
// account's open one. Vouchers count per account (model.ErrTooManyAttempts).
func (s *Service) CreatePairingVoucher(ctx context.Context, session model.Session, tvKey string) (model.PairingVoucher, error) {
	if session.Kind != model.SessionApp {
		return model.PairingVoucher{}, model.ErrAppSessionRequired
	}
	if session.HasProfile() && !session.IsPrimary() {
		return model.PairingVoucher{}, model.ErrPrimaryProfileRequired
	}
	if _, err := parseTVKey(tvKey); err != nil {
		return model.PairingVoucher{}, (&model.ValidationError{}).Add("tvKey", "invalid_value")
	}
	now := s.now()
	if err := s.pairing.vouchers.Allow("account:"+strconv.FormatInt(session.Account.ID, 10), now); err != nil {
		return model.PairingVoucher{}, err
	}
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return model.PairingVoucher{}, fmt.Errorf("pairing voucher: %w", err)
	}
	voucher := b64.EncodeToString(raw)
	expires := now.Add(VoucherLifetime)
	if err := s.write.UpsertPairingVoucher(ctx, appq.UpsertPairingVoucherParams{
		AccountID: session.Account.ID, VoucherHash: hashToken(voucher), TvKey: tvKey, DeviceKind: AppDeviceTV,
		Now: now.UnixMilli(), ExpiresAt: expires.UnixMilli(),
	}); err != nil {
		return model.PairingVoucher{}, fmt.Errorf("store pairing voucher: %w", err)
	}
	s.log.InfoContext(ctx, "pairing voucher issued", "account_id", session.Account.ID, "session_id", session.ID)
	return model.PairingVoucher{Voucher: voucher, ExpiresAt: expires.UTC()}, nil
}

// RedeemPairingVoucher redeems a voucher for the TV that holds tvKey: the voucher must be open,
// unexpired and bound to exactly this key; it is used up in any case it was found, also when the key
// or the expiry does not fit. It starts a device session for the voucher's account, with the device
// row named deviceName and systemVersion as its user agent, seals the token and the device id for
// tvKey (RedeemInfo) and reports the pairing (WithDeviceLinked). Every call counts against the
// client address in ctx (model.ErrTooManyAttempts). Errors: *model.ValidationError (voucher,
// tvKey, deviceName, systemVersion), model.ErrVoucherInvalid for every voucher that does not
// redeem, whatever the reason.
func (s *Service) RedeemPairingVoucher(ctx context.Context, voucher, tvKey, deviceName, systemVersion string) (model.PairingRedeemed, error) {
	now := s.now()
	if err := s.pairing.redeems.Allow(model.ClientIP(ctx), now); err != nil {
		return model.PairingRedeemed{}, err
	}
	name, system := strings.TrimSpace(deviceName), strings.TrimSpace(systemVersion)
	var verr *model.ValidationError
	if raw, err := b64.DecodeString(voucher); err != nil || len(raw) != tokenBytes {
		verr = verr.Add("voucher", "invalid_value")
	}
	pub, err := parseTVKey(tvKey)
	if err != nil {
		verr = verr.Add("tvKey", "invalid_value")
	}
	if code := deviceNameProblem(name, maxDeviceName); code != "" {
		verr = verr.Add("deviceName", code)
	}
	if code := deviceNameProblem(system, maxSystemVersion); code != "" {
		verr = verr.Add("systemVersion", code)
	}
	if verr != nil {
		return model.PairingRedeemed{}, verr
	}
	row, err := s.write.TakePairingVoucher(ctx, hashToken(voucher))
	if errors.Is(err, sql.ErrNoRows) {
		s.log.InfoContext(ctx, "pairing voucher refused", "reason", "unknown")
		return model.PairingRedeemed{}, model.ErrVoucherInvalid
	}
	if err != nil {
		return model.PairingRedeemed{}, fmt.Errorf("take pairing voucher: %w", err)
	}
	reason := ""
	switch {
	case row.ExpiresAt <= now.UnixMilli():
		reason = "expired"
	case subtle.ConstantTimeCompare([]byte(row.TvKey), []byte(tvKey)) != 1:
		reason = "other_key"
	}
	if reason != "" {
		s.log.InfoContext(ctx, "pairing voucher refused", "account_id", row.AccountID, "reason", reason)
		return model.PairingRedeemed{}, model.ErrVoucherInvalid
	}
	acc, err := s.write.GetAccount(ctx, row.AccountID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.PairingRedeemed{}, model.ErrVoucherInvalid // the account went meanwhile
	}
	if err != nil {
		return model.PairingRedeemed{}, fmt.Errorf("account of the voucher: %w", err)
	}
	account, profile, err := s.signInProfile(ctx, acc)
	if err != nil {
		return model.PairingRedeemed{}, err
	}
	session, token, err := s.StartAppSession(ctx, account, profile, name, row.DeviceKind, "", system)
	if err != nil {
		return model.PairingRedeemed{}, err
	}
	sealed, err := sealRedeem(pub, token, session.DeviceID)
	if err != nil {
		// Without the answer the TV cannot use the session; it must not live on.
		if lerr := s.Logout(context.WithoutCancel(ctx), session.ID); lerr != nil {
			err = errors.Join(err, lerr)
		}
		return model.PairingRedeemed{}, err
	}
	s.log.InfoContext(ctx, "tv paired", "account_id", account.ID, "session_id", session.ID)
	if s.deviceLinked != nil {
		if err := s.deviceLinked(ctx, account.ID, row.DeviceKind, name, system, now); err != nil {
			s.log.WarnContext(ctx, "device linked hook failed", "account_id", account.ID, "err", err)
		}
	}
	return model.PairingRedeemed{Session: session, SealedToken: sealed, AccountEmail: account.Email}, nil
}

// parseTVKey decodes and checks the TV's X25519 public key (32 bytes, base64url).
func parseTVKey(tvKey string) ([]byte, error) {
	raw, err := b64.DecodeString(tvKey)
	if err != nil {
		return nil, err
	}
	if _, err := ecdh.X25519().NewPublicKey(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// sealRedeem seals {"token","deviceId"} for the TV's key: HPKE base mode, DHKEM(X25519,
// HKDF-SHA256), HKDF-SHA256, AES-256-GCM, info RedeemInfo, empty aad; enc followed by ciphertext.
func sealRedeem(tvPublic []byte, token, deviceID string) ([]byte, error) {
	plain, err := json.Marshal(struct {
		Token    string `json:"token"`
		DeviceID string `json:"deviceId"`
	}{token, deviceID})
	if err != nil {
		return nil, err
	}
	pk, err := pairingKEM.NewPublicKey(tvPublic)
	if err != nil {
		return nil, fmt.Errorf("tv key: %w", err)
	}
	sealed, err := hpke.Seal(pk, pairingKDF, pairingAEAD, []byte(RedeemInfo), plain)
	if err != nil {
		return nil, fmt.Errorf("seal device token: %w", err)
	}
	return sealed, nil
}

// deviceNameProblem returns the validation code of a device name (or the TV's system version) of at
// most limit characters: required, too_long, invalid_value (a control character), or "" when fine.
func deviceNameProblem(name string, limit int) string {
	switch {
	case name == "":
		return "required"
	case utf8.RuneCountInString(name) > limit:
		return "too_long"
	case strings.IndexFunc(name, unicode.IsControl) >= 0:
		return "invalid_value"
	}
	return ""
}
