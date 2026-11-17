package model

import "errors"

// Approving and declining in the administration (concept 25 stage 4, F-16; data model 4.5
// approveWish, declineWish): a pass-through to Seerr, nothing is decided in Slouchcat.
var (
	// ErrWishNotPending: the wish, or every request it carries, is not waiting for an approval in
	// Seerr (409 wish_not_pending).
	ErrWishNotPending = errors.New("wish not pending")
	// Seerr not answering or switched off is ErrSeerrUnreachable (wish_requests.go, 503
	// seerr_unreachable); nothing changed then.
	// ErrSeerrFailed: Seerr answered with an error (502 seerr_failed).
	ErrSeerrFailed = errors.New("seerr failed")
)
