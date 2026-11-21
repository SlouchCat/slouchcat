package api

import (
	"context"
	"encoding/base64"
	"net/http"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// Pairing of a TV and the proof of the server (concept 27 sections 6 and 10, ADR 0040) through
// Options.Pairing; nil answers 501. The answers that carry a secret are not cached.

func (e serviceProblem) VisitProveServerResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitCreatePairingVoucherResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitRedeemPairingVoucherResponse(w http.ResponseWriter) error {
	return e.visit(w)
}

// ProveServer signs the app's nonce with the server's key. A nonce that is no base64url reaches
// the service empty, so the attempt still counts.
func (s *server) ProveServer(ctx context.Context, req gen.ProveServerRequestObject) (gen.ProveServerResponseObject, error) {
	if s.opts.Pairing == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	nonce, err := base64.RawURLEncoding.DecodeString(deref(req.Body).Nonce)
	if err != nil {
		nonce = nil
	}
	sig, err := s.opts.Pairing.ProveServer(model.WithClientIP(ctx, s.clientAddr(ctx)), nonce)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.ProveServer200JSONResponse{
		Signature: base64.RawURLEncoding.EncodeToString(sig), ServerKey: s.opts.Pairing.ServerKey(),
	}, nil
}

// CreatePairingVoucher issues the voucher of the session's account (app sessions only).
func (s *server) CreatePairingVoucher(ctx context.Context, req gen.CreatePairingVoucherRequestObject) (gen.CreatePairingVoucherResponseObject, error) {
	session, ok := sessionFrom(ctx)
	if !ok {
		return nil, errNoSession
	}
	if s.opts.Pairing == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	v, err := s.opts.Pairing.CreatePairingVoucher(ctx, session, deref(req.Body).TvKey)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return voucherIssued{gen.PairingVoucher{Voucher: v.Voucher, ExpiresAt: v.ExpiresAt}}, nil
}

// voucherIssued answers createPairingVoucher with Cache-Control: no-store (the body is a secret).
type voucherIssued struct{ body gen.PairingVoucher }

func (v voucherIssued) VisitCreatePairingVoucherResponse(w http.ResponseWriter) error {
	w.Header().Set("Cache-Control", "no-store")
	return gen.CreatePairingVoucher201JSONResponse(v.body).VisitCreatePairingVoucherResponse(w)
}

// RedeemPairingVoucher starts the TV's device session and answers its token sealed for the TV.
// The middleware drops every cookie of this route (appRoutes).
func (s *server) RedeemPairingVoucher(ctx context.Context, req gen.RedeemPairingVoucherRequestObject) (gen.RedeemPairingVoucherResponseObject, error) {
	if s.opts.Pairing == nil {
		return s.problem(ctx, model.ErrNotImplemented), nil
	}
	body := deref(req.Body)
	out, err := s.opts.Pairing.RedeemPairingVoucher(model.WithClientIP(ctx, s.clientAddr(ctx)), deref(body.Voucher), body.TvKey,
		body.DeviceName, body.SystemVersion)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return voucherRedeemed{gen.PairingRedeemed{
		SealedToken: base64.RawURLEncoding.EncodeToString(out.SealedToken), Signature: base64.RawURLEncoding.EncodeToString(out.Signature),
	}}, nil
}

// voucherRedeemed answers redeemPairingVoucher with Cache-Control: no-store.
type voucherRedeemed struct{ body gen.PairingRedeemed }

func (v voucherRedeemed) VisitRedeemPairingVoucherResponse(w http.ResponseWriter) error {
	w.Header().Set("Cache-Control", "no-store")
	return gen.RedeemPairingVoucher200JSONResponse(v.body).VisitRedeemPairingVoucherResponse(w)
}
