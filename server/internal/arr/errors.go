package arr

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode"
)

// Error classes. An *Error unwraps to one or two of them, so callers test with errors.Is. The
// answers that decide a class are the ones Radarr 6.4.4 and Sonarr 4.0.20 send (concept 25,
// appendix I section 5).
var (
	// ErrUnreachable: no answer (network error, time-out) or a redirect (ErrRedirect).
	ErrUnreachable = errors.New("arr unreachable")
	// ErrRedirect: the service answered 3xx. The client never follows a redirect, so the key never
	// travels to another address. Also ErrUnreachable.
	ErrRedirect = errors.New("arr answered with a redirect")
	// ErrKeyInvalid: 401, the service refused the api key. The caller reads the key anew from Seerr
	// (KeyCache.Invalidate, K-23).
	ErrKeyInvalid = errors.New("arr refused the api key")
	// ErrNotFound: 404, or a lookup by tmdbId or tvdbId that found no title (the title is not yet
	// in Radarr or Sonarr).
	ErrNotFound = errors.New("arr: not found")
	// ErrValidation: 400 or 422, the validator of the service refused the call; for a tag label
	// Radarr answers [{"propertyName": "Label", "errorMessage": "Allowed characters a-z, 0-9 and
	// -", …}] (I-52). Error.Message carries "Label: Allowed characters …".
	ErrValidation = errors.New("arr: validation failed")
	// ErrBadRequest: another 4xx answer.
	ErrBadRequest = errors.New("arr: bad request")
	// ErrServer: 5xx, or an answer that is no JSON of the expected shape, larger than the limit or
	// a queue that does not end.
	ErrServer = errors.New("arr: server error")
	// ErrInvalid: the client refused the call before sending it (bad address, missing key, unknown
	// kind, a call of the other service, an invalid tag label or id).
	ErrInvalid = errors.New("arr: invalid call")
)

// Error is a refused or failed call. Its text names the service, the method and the path without
// query, the status and the service's message; never a header, the api key or a request body.
type Error struct {
	Service Kind   // radarr or sonarr
	Op      string // method and path without query, "PUT /api/v3/movie/editor"
	Status  int    // HTTP status; 0 without an answer
	Message string // the service's message, shortened and cleaned; "" when there was none
	classes []error
	cause   error
}

func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString(string(e.Service))
	b.WriteString(" ")
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

// classify maps a non-2xx status to the error classes.
func classify(status int) []error {
	switch {
	case status >= 300 && status < 400:
		return []error{ErrRedirect, ErrUnreachable}
	case status == http.StatusUnauthorized:
		return []error{ErrKeyInvalid}
	case status == http.StatusNotFound:
		return []error{ErrNotFound}
	case status == http.StatusBadRequest || status == http.StatusUnprocessableEntity:
		return []error{ErrValidation}
	case status >= 500:
		return []error{ErrServer}
	}
	return []error{ErrBadRequest}
}

// arrMessage reads the error body: the validation array [{propertyName, errorMessage}] (I-52), an
// object {"message": …} or {"error": …}, or plain text; an HTML page gives "".
func arrMessage(data []byte) string {
	var failures []struct {
		PropertyName string `json:"propertyName"`
		ErrorMessage string `json:"errorMessage"`
	}
	if json.Unmarshal(data, &failures) == nil {
		parts := make([]string, 0, len(failures))
		for _, f := range failures {
			switch {
			case f.PropertyName != "" && f.ErrorMessage != "":
				parts = append(parts, f.PropertyName+": "+f.ErrorMessage)
			case f.ErrorMessage != "":
				parts = append(parts, f.ErrorMessage)
			}
		}
		return strings.Join(parts, "; ")
	}
	var body struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if json.Unmarshal(data, &body) == nil {
		if body.Message != "" {
			return body.Message
		}
		return body.Error
	}
	if t := strings.TrimSpace(string(data)); !strings.HasPrefix(t, "<") {
		return t
	}
	return ""
}

// maxMessage is the length Error.Message and the queue messages keep of the service's text.
const maxMessage = 200

// cleanMessage shortens msg, drops control characters and replaces every secret with
// "[redacted]": the service's message is foreign text, and an error may reach the log.
func cleanMessage(msg string, secrets ...string) string {
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
