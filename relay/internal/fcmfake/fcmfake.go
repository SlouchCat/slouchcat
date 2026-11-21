// Package fcmfake stands in for Google's push service in tests and local runs: the OAuth token
// endpoint and FCM HTTP v1 messages:send. It remembers every message it accepted and hands them
// out on a read address. It never ships (no release image) and is no part of either service.
//
// Behaviour by the device token of a message:
//
//	unregistered-...  404 with errorCode UNREGISTERED ("token unknown", the forwarder answers 410)
//	invalid-...       400 INVALID_ARGUMENT (the forwarder answers 503)
//	fail-...          500 INTERNAL
//	busy-...          429 RESOURCE_EXHAUSTED with Retry-After: 7
//	anything else     200, the message is stored
//
// Routes: POST /token, POST /v1/projects/{project}/messages:send, GET /messages[?token=...],
// DELETE /messages, GET /healthz.
package fcmfake

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Received is one accepted message.
type Received struct {
	Project  string            `json:"project"`
	Token    string            `json:"token"`
	Data     map[string]string `json:"data"`
	Priority string            `json:"priority"`
	TTL      string            `json:"ttl"`
	At       time.Time         `json:"at"`
}

// Fake is the fake push service.
type Fake struct {
	Project string
	Key     *rsa.PrivateKey // of the service account; the token endpoint checks assertions with it
	Email   string

	mu       sync.Mutex
	tokens   map[string]bool
	messages []Received
	tokenReq int
}

// New makes a fake with a fresh service account key.
func New(project string) (*Fake, error) {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	return &Fake{Project: project, Key: k, Email: "relay@" + project + ".example.test", tokens: map[string]bool{}}, nil
}

// ServiceAccount returns the JSON of a service account file whose token_uri is tokenURL.
func (f *Fake) ServiceAccount(tokenURL string) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(f.Key)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(map[string]string{
		"type": "service_account", "project_id": f.Project, "client_email": f.Email,
		"private_key": string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		"token_uri":   tokenURL,
	}, "", "  ")
}

// Messages returns the messages accepted so far, for token or all.
func (f *Fake) Messages(token string) []Received {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Received
	for _, m := range f.messages {
		if token == "" || m.Token == token {
			out = append(out, m)
		}
	}
	return out
}

// TokenRequests counts the requests to the token endpoint.
func (f *Fake) TokenRequests() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tokenReq
}

// Handler serves the routes.
func (f *Fake) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("POST /token", f.token)
	mux.HandleFunc("POST /v1/projects/{rest...}", f.send)
	mux.HandleFunc("GET /messages", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		msgs := f.Messages(r.URL.Query().Get("token"))
		if msgs == nil {
			msgs = []Received{}
		}
		_ = json.NewEncoder(w).Encode(msgs)
	})
	mux.HandleFunc("DELETE /messages", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		f.messages = nil
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func (f *Fake) token(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.tokenReq++
	f.mu.Unlock()
	if err := r.ParseForm(); err != nil || r.PostForm.Get("grant_type") != "urn:ietf:params:oauth:grant-type:jwt-bearer" {
		googleError(w, http.StatusBadRequest, "invalid_grant", "")
		return
	}
	if err := f.checkAssertion(r.PostForm.Get("assertion")); err != nil {
		googleError(w, http.StatusBadRequest, "invalid_grant", "")
		return
	}
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	tok := "fake-" + base64.RawURLEncoding.EncodeToString(b)
	f.mu.Lock()
	f.tokens[tok] = true
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"access_token": tok, "expires_in": 3599, "token_type": "Bearer"})
}

func (f *Fake) checkAssertion(a string) error {
	parts := strings.Split(a, ".")
	if len(parts) != 3 {
		return fmt.Errorf("not a JWT")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(&f.Key.PublicKey, crypto.SHA256, digest[:], sig); err != nil {
		return err
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return err
	}
	var c struct {
		Iss   string `json:"iss"`
		Scope string `json:"scope"`
		Exp   int64  `json:"exp"`
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return err
	}
	if c.Iss != f.Email || c.Scope != "https://www.googleapis.com/auth/firebase.messaging" || time.Unix(c.Exp, 0).Before(time.Now()) {
		return fmt.Errorf("wrong claims")
	}
	return nil
}

func (f *Fake) send(w http.ResponseWriter, r *http.Request) {
	project, ok := strings.CutSuffix(r.PathValue("rest"), "/messages:send")
	if !ok {
		googleError(w, http.StatusNotFound, "NOT_FOUND", "")
		return
	}
	bearer, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	f.mu.Lock()
	known := f.tokens[bearer]
	f.mu.Unlock()
	if !known {
		googleError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "")
		return
	}
	if project != f.Project {
		googleError(w, http.StatusForbidden, "PERMISSION_DENIED", "")
		return
	}
	var body struct {
		Message struct {
			Token   string            `json:"token"`
			Data    map[string]string `json:"data"`
			Android struct {
				Priority string `json:"priority"`
				TTL      string `json:"ttl"`
			} `json:"android"`
		} `json:"message"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&body); err != nil || body.Message.Token == "" {
		googleError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "INVALID_ARGUMENT")
		return
	}
	size := 0
	for k, v := range body.Message.Data {
		size += len(k) + len(v)
	}
	tok := body.Message.Token
	switch {
	case size > 4096:
		googleError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "INVALID_ARGUMENT")
	case strings.HasPrefix(tok, "unregistered-"):
		googleError(w, http.StatusNotFound, "NOT_FOUND", "UNREGISTERED")
	case strings.HasPrefix(tok, "invalid-"):
		googleError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "INVALID_ARGUMENT")
	case strings.HasPrefix(tok, "fail-"):
		googleError(w, http.StatusInternalServerError, "INTERNAL", "INTERNAL")
	case strings.HasPrefix(tok, "busy-"):
		w.Header().Set("Retry-After", "7")
		googleError(w, http.StatusTooManyRequests, "RESOURCE_EXHAUSTED", "QUOTA_EXCEEDED")
	default:
		f.mu.Lock()
		f.messages = append(f.messages, Received{
			Project: project, Token: tok, Data: body.Message.Data,
			Priority: body.Message.Android.Priority, TTL: body.Message.Android.TTL, At: time.Now().UTC(),
		})
		n := len(f.messages)
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"name": fmt.Sprintf("projects/%s/messages/%d", project, n)})
	}
}

func googleError(w http.ResponseWriter, status int, s, errorCode string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	e := map[string]any{"code": status, "status": s, "message": strings.ToLower(s)}
	if errorCode != "" {
		e["details"] = []map[string]string{{"@type": "type.googleapis.com/google.firebase.fcm.v1.FcmError", "errorCode": errorCode}}
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"error": e})
}
