package seerr

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode"
)

// Error classes. An *Error unwraps to one or two of them (a specific class and its group, for
// example ErrQuota and ErrForbidden), so callers test with errors.Is. The texts that decide a class
// are the ones Seerr v3.5.2 and Jellyseerr v2.7.3 send (concept 25, appendix D 3.4, 7.2, 8 and
// appendix H 3).
var (
	// ErrUnreachable: no answer (network error, time-out) or a redirect (ErrRedirect).
	ErrUnreachable = errors.New("seerr unreachable")
	// ErrRedirect: Seerr answered 3xx. The client never follows a redirect, so the key and a
	// password in the body never travel to another address. Also ErrUnreachable.
	ErrRedirect = errors.New("seerr answered with a redirect")
	// ErrKeyInvalid: a call as the administrator was refused (401, or 403 "You do not have
	// permission to access this endpoint"). User 1 has ADMIN and passes every check, so only a
	// wrong API key explains it.
	ErrKeyInvalid = errors.New("seerr refused the api key")
	// ErrAccessDenied: a call on behalf of a user was refused by Seerr's authentication (same
	// answers as ErrKeyInvalid). Either the key is wrong, the user id is unknown (deleted user) or
	// the user lacks the right; a call as the administrator (Me) tells them apart. Also ErrForbidden.
	ErrAccessDenied = errors.New("seerr refused the call for this user")
	// ErrForbidden: any other 403 answer; Error.Message carries Seerr's text.
	ErrForbidden = errors.New("seerr forbids the call")
	// ErrNoPermission: "You do not have permission to …" (make movie requests, series requests,
	// modify the request user, bypass user quota limits, delete this request). Also ErrForbidden.
	ErrNoPermission = errors.New("seerr: no permission")
	// ErrQuota: "Movie Quota exceeded." or "Series Quota exceeded.". Also ErrForbidden.
	ErrQuota = errors.New("seerr: quota exceeded")
	// ErrBlocklisted: "This media is blocklisted." (3.x) or "… blacklisted." (2.x). Also ErrForbidden.
	ErrBlocklisted = errors.New("seerr: media is blocklisted")
	// ErrCSRF: Seerr's CSRF protection is on; it refuses every write with the api key (appendix D 2.4).
	ErrCSRF = errors.New("seerr: csrf protection blocks writes")
	// ErrNotFound: 404.
	ErrNotFound = errors.New("seerr: not found")
	// ErrConflict: any 409 answer, for example "Only pending requests can be approved or declined.".
	ErrConflict = errors.New("seerr: conflict")
	// ErrDuplicate: 409 "Request for this media already exists." or "Auto-request for this media and
	// user already exists.". Also ErrConflict.
	ErrDuplicate = errors.New("seerr: request already exists")
	// ErrUserExists: 409 "User already exists with submitted email." of POST /user. Also ErrConflict.
	ErrUserExists = errors.New("seerr: user already exists")
	// ErrNoSeasons: POST /request answered 202 "No seasons available to request": every season is
	// already requested or available. Nothing was created.
	ErrNoSeasons = errors.New("seerr: no seasons available to request")
	// ErrBadRequest: 400 (the request validator of Seerr) or another 4xx without a class.
	ErrBadRequest = errors.New("seerr: bad request")
	// ErrServer: 5xx (for example a TMDB failure inside Seerr) or an answer that is no JSON of the
	// expected shape or larger than the limit.
	ErrServer = errors.New("seerr: server error")
	// ErrInvalid: the client refused the call before sending it (bad address, missing key, a
	// request that cannot be valid).
	ErrInvalid = errors.New("seerr: invalid call")
)

// Error is a refused or failed call. Its text names the method and the path without query, the
// status and Seerr's message; never a header, the api key or a request body.
type Error struct {
	Op      string // method and path without query, "POST /user/5/settings/password"
	Status  int    // HTTP status; 0 without an answer
	Message string // Seerr's message, shortened and cleaned; "" when there was none
	classes []error
	cause   error
}

func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString("seerr ")
	b.WriteString(e.Op)
	if e.Status != 0 {
		fmt.Fprintf(&b, ": status %d", e.Status)
	}
	if e.Message != "" {
		b.WriteString(": ")
		b.WriteString(e.Message)
	}
	if len(e.classes) > 0 {
		b.WriteString(" (")
		b.WriteString(e.classes[0].Error())
		b.WriteString(")")
	}
	if e.cause != nil {
		b.WriteString(": ")
		b.WriteString(e.cause.Error())
	}
	return b.String()
}

// Unwrap returns the classes and the cause (a transport or context error).
func (e *Error) Unwrap() []error {
	out := append([]error(nil), e.classes...)
	if e.cause != nil {
		out = append(out, e.cause)
	}
	return out
}

// Texts of Seerr that decide a class (appendix D 2.1, 3.4, 7.2, 8).
const (
	textAccessEndpoint = "you do not have permission to access this endpoint"
	textNoPermission   = "you do not have permission to"
	textQuota          = "quota exceeded"
	textBlocklisted    = "this media is blocklisted"
	textBlacklisted    = "this media is blacklisted"
	textCSRF           = "csrf"
	textUserExists     = "user already exists"
	textDuplicate      = "already exists"
)

// classify maps a non-2xx status and Seerr's message to the error classes. asUser is true for a
// call with X-API-User.
func classify(status int, message string, asUser bool) []error {
	m := strings.ToLower(message)
	refused := func() []error {
		if asUser {
			return []error{ErrAccessDenied, ErrForbidden}
		}
		return []error{ErrKeyInvalid}
	}
	switch {
	case status >= 300 && status < 400:
		return []error{ErrRedirect, ErrUnreachable}
	case status == http.StatusUnauthorized:
		if strings.HasPrefix(m, textNoPermission) && !strings.HasPrefix(m, textAccessEndpoint) {
			return []error{ErrNoPermission, ErrForbidden} // DELETE /request answers 401 (appendix D 8)
		}
		return refused()
	case status == http.StatusForbidden:
		switch {
		case strings.Contains(m, textCSRF):
			return []error{ErrCSRF}
		case strings.HasPrefix(m, textAccessEndpoint):
			return refused()
		case strings.HasSuffix(strings.TrimSuffix(m, "."), textQuota):
			return []error{ErrQuota, ErrForbidden}
		case strings.HasPrefix(m, textBlocklisted), strings.HasPrefix(m, textBlacklisted):
			return []error{ErrBlocklisted, ErrForbidden}
		case strings.HasPrefix(m, textNoPermission):
			return []error{ErrNoPermission, ErrForbidden}
		}
		return []error{ErrForbidden}
	case status == http.StatusNotFound:
		return []error{ErrNotFound}
	case status == http.StatusConflict:
		switch {
		case strings.HasPrefix(m, textUserExists):
			return []error{ErrUserExists, ErrConflict}
		case strings.Contains(m, textDuplicate):
			return []error{ErrDuplicate, ErrConflict}
		}
		return []error{ErrConflict}
	case status >= 500:
		return []error{ErrServer}
	}
	return []error{ErrBadRequest}
}

// maxMessage is the length Error.Message keeps of Seerr's message.
const maxMessage = 200

// cleanMessage shortens msg, drops control characters and replaces every secret (the api key, a
// password of the request body) with "[redacted]": Seerr's message is foreign text, and an error
// may reach the log.
func cleanMessage(msg string, secrets []string) string {
	for _, s := range secrets {
		if s != "" {
			msg = strings.ReplaceAll(msg, s, redacted)
		}
	}
	msg = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, strings.TrimSpace(msg))
	if r := []rune(msg); len(r) > maxMessage {
		msg = string(r[:maxMessage]) + "…"
	}
	return msg
}
