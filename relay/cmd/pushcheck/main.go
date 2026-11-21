// Command pushcheck plays a Slouchcat server (and the app's keys) against a running push forwarder
// (concept 27 sections 7.3 and 13, stage 2): it registers a token and a VAPID key, sends a Web Push
// message signed with VAPID and encrypted for the "app" (RFC 8291), and checks the refusals: a
// wrong VAPID key, a wrong aud, an expired token, a body over the limit. With -fcmfake it also
// reads what reached the stand-in for Google, decrypts it with the app's keys, and checks 410 for
// a token Google no longer knows. It is a tool of the move rehearsal (7.5, step 3).
//
//	pushcheck -url https://push.example.test [-fcmfake http://127.0.0.1:8190] [-ca ca.pem] [-connect host:port]
//
// For a move: -register-url names the instance that seals the address (the old one), -url the one
// that receives the push (the new one, under its probe name). -expect-unavailable checks that an
// instance without the sealing key answers 503 and nothing reaches Google (D-10).
package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/relay/internal/checkclient"
	"github.com/slouchcat/slouchcat/relay/internal/vapid"
	"github.com/slouchcat/slouchcat/relay/internal/webpush"
)

// Options of a run.
type Options struct {
	URL               string // push forwarder that receives the push
	RegisterURL       string // push forwarder that seals the address (default URL)
	FCMFake           string // read address of the fake; empty: Google is real, arrival is not checked
	Token             string // device token to register (default check-<random>)
	MaxBody           int    // the forwarder's RELAY_PUSH_MAX_BODY, for the size check
	ExpectUnavailable bool
}

func main() {
	var o Options
	flag.StringVar(&o.URL, "url", "http://127.0.0.1:8181", "push forwarder that receives the push")
	flag.StringVar(&o.RegisterURL, "register-url", "", "push forwarder that seals the address (default -url)")
	flag.StringVar(&o.FCMFake, "fcmfake", "", "read address of fcmfake, to check arrival and decryption")
	flag.StringVar(&o.Token, "token", os.Getenv("PUSHCHECK_TOKEN"), "device token to register (default $PUSHCHECK_TOKEN, else check-<random>); prefer the variable, it stays out of the shell history")
	flag.IntVar(&o.MaxBody, "max-body", 2700, "the forwarder's RELAY_PUSH_MAX_BODY")
	flag.BoolVar(&o.ExpectUnavailable, "expect-unavailable", false, "expect 503 from -url (an instance without the sealing key)")
	ca := flag.String("ca", "", "PEM file of an extra CA to trust (a local test CA)")
	connect := flag.String("connect", "", "dial this host:port for -url instead of resolving its name")
	flag.Parse()
	pushClient, err := checkclient.New(*ca, *connect, 20*time.Second)
	if err != nil {
		fmt.Fprintln(os.Stderr, "pushcheck:", err)
		os.Exit(2)
	}
	regClient := pushClient
	if o.RegisterURL != "" && o.RegisterURL != o.URL {
		if regClient, err = checkclient.New(*ca, "", 20*time.Second); err != nil {
			fmt.Fprintln(os.Stderr, "pushcheck:", err)
			os.Exit(2)
		}
	}
	n, err := Run(regClient, pushClient, http.DefaultClient, o, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pushcheck: FAILED after %d checks: %v\n", n, err)
		os.Exit(1)
	}
	fmt.Printf("pushcheck: %d checks passed against %s\n", n, o.URL)
}

type checker struct {
	reg, push, fake *http.Client
	o               Options
	out             io.Writer
	n               int
	vapidKey        *ecdsa.PrivateKey
	app             *webpush.Subscriber
}

func (k *checker) ok(what string, status int) {
	k.n++
	fmt.Fprintf(k.out, "ok   %-48s %d\n", what, status)
}

// Run performs the checks and returns how many passed.
func Run(reg, push, fake *http.Client, o Options, out io.Writer) (int, error) {
	o.URL = strings.TrimRight(o.URL, "/")
	if o.RegisterURL == "" {
		o.RegisterURL = o.URL
	}
	o.RegisterURL = strings.TrimRight(o.RegisterURL, "/")
	k := &checker{reg: reg, push: push, fake: fake, o: o, out: out}
	err := k.run()
	return k.n, err
}

func (k *checker) run() error {
	var err error
	if k.vapidKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader); err != nil {
		return err
	}
	if k.app, err = webpush.NewSubscriber(); err != nil {
		return err
	}
	token := k.o.Token
	if token == "" {
		token = "check-" + randomText(12)
	}
	endpoint, err := k.register(token)
	if err != nil {
		return err
	}
	if k.o.ExpectUnavailable {
		return k.unavailable(endpoint, token)
	}
	if status, err := k.health(); err != nil {
		return err
	} else {
		k.ok("health check", status)
	}

	text := `{"title":"pushcheck","body":"` + randomText(8) + `","url":"/"}`
	body, err := webpush.Encrypt(k.app.Key.PublicKey(), k.app.Auth, []byte(text), 1024)
	if err != nil {
		return err
	}
	aud := origin(k.o.URL)
	if err := k.send("push signed by the sealed VAPID key", endpoint, k.vapidKey, aud, time.Hour, body, http.StatusCreated); err != nil {
		return err
	}
	if k.o.FCMFake != "" {
		if err := k.arrived(token, text); err != nil {
			return err
		}
	}
	other, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err := k.send("push signed by another VAPID key", endpoint, other, aud, time.Hour, body, http.StatusForbidden); err != nil {
		return err
	}
	// A valid signature for another name means RELAY_PUSH_ORIGINS misses a name: the forwarder's
	// own fault, 503 (D-10).
	if err := k.send("push with a foreign aud (503)", endpoint, k.vapidKey, "https://elsewhere.example.test", time.Hour, body, http.StatusServiceUnavailable); err != nil {
		return err
	}
	if err := k.send("push with an expired VAPID token", endpoint, k.vapidKey, aud, -time.Minute, body, http.StatusForbidden); err != nil {
		return err
	}
	if err := k.send("push over the size limit", endpoint, k.vapidKey, aud, time.Hour, make([]byte, k.o.MaxBody+1), http.StatusRequestEntityTooLarge); err != nil {
		return err
	}
	if k.o.FCMFake != "" {
		gone, err := k.register("unregistered-" + randomText(8))
		if err != nil {
			return err
		}
		if err := k.send("push to a token Google no longer knows", gone, k.vapidKey, aud, time.Hour, body, http.StatusGone); err != nil {
			return err
		}
		failing, err := k.register("fail-" + randomText(8))
		if err != nil {
			return err
		}
		if err := k.send("push while Google fails (503, not 410)", failing, k.vapidKey, aud, time.Hour, body, http.StatusServiceUnavailable); err != nil {
			return err
		}
	}
	return nil
}

func (k *checker) unavailable(endpoint, token string) error {
	text := `{"title":"pushcheck","body":"unavailable"}`
	body, err := webpush.Encrypt(k.app.Key.PublicKey(), k.app.Auth, []byte(text), 1024)
	if err != nil {
		return err
	}
	if err := k.send("push to an instance without the key (503)", endpoint, k.vapidKey, origin(k.o.URL), time.Hour, body, http.StatusServiceUnavailable); err != nil {
		return err
	}
	if k.o.FCMFake != "" {
		msgs, err := k.messages(token)
		if err != nil {
			return err
		}
		if len(msgs) != 0 {
			return fmt.Errorf("%d message(s) reached Google from an instance without the key", len(msgs))
		}
		k.ok("nothing reached Google", http.StatusOK)
	}
	return nil
}

func (k *checker) health() (int, error) {
	resp, err := k.push.Get(k.o.URL + "/v1/healthz") //nolint:noctx // the client has a timeout
	if err != nil {
		return 0, err
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("health check: status %d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}

func (k *checker) register(token string) (string, error) {
	pub, err := vapid.PublicKeyString(&k.vapidKey.PublicKey)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(map[string]string{"fcmToken": token, "vapidKey": pub})
	resp, err := k.reg.Post(k.o.RegisterURL+"/v1/register", "application/json", bytes.NewReader(b)) //nolint:noctx // the client has a timeout
	if err != nil {
		return "", fmt.Errorf("register: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("register: status %d (%s)", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var r struct {
		Endpoint string `json:"endpoint"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return "", fmt.Errorf("register: %w", err)
	}
	u, err := url.Parse(r.Endpoint)
	if err != nil || !strings.HasPrefix(u.Path, "/v1/push/") {
		return "", fmt.Errorf("register: endpoint %q is not a push address", r.Endpoint)
	}
	k.ok("register a device token", resp.StatusCode)
	// The push goes to -url: during a move that is the new instance under its probe name, and the
	// path (version and blob) is all an instance needs.
	return k.o.URL + u.Path, nil
}

func (k *checker) send(what, endpoint string, key *ecdsa.PrivateKey, aud string, valid time.Duration, body []byte, want int) error {
	auth, err := vapid.Authorization(key, vapid.Claims{Aud: aud, Exp: time.Now().Add(valid).Unix(), Sub: "mailto:check@example.test"})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body)) //nolint:noctx // the client has a timeout
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("TTL", "3600")
	req.Header.Set("Urgency", "normal")
	resp, err := k.push.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	_ = resp.Body.Close()
	if resp.StatusCode != want {
		return fmt.Errorf("%s: status %d, want %d (%s)", what, resp.StatusCode, want, strings.TrimSpace(string(raw)))
	}
	k.ok(what, resp.StatusCode)
	return nil
}

type received struct {
	Token string            `json:"token"`
	Data  map[string]string `json:"data"`
}

func (k *checker) messages(token string) ([]received, error) {
	resp, err := k.fake.Get(strings.TrimRight(k.o.FCMFake, "/") + "/messages?token=" + url.QueryEscape(token)) //nolint:noctx // test tool
	if err != nil {
		return nil, fmt.Errorf("fcmfake: %w", err)
	}
	defer resp.Body.Close()
	var msgs []received
	if err := json.NewDecoder(resp.Body).Decode(&msgs); err != nil {
		return nil, fmt.Errorf("fcmfake: %w", err)
	}
	return msgs, nil
}

func (k *checker) arrived(token, text string) error {
	msgs, err := k.messages(token)
	if err != nil {
		return err
	}
	if len(msgs) != 1 {
		return fmt.Errorf("fcmfake holds %d messages for the token, want 1", len(msgs))
	}
	raw, err := base64.StdEncoding.DecodeString(msgs[0].Data["wp"])
	if err != nil {
		return fmt.Errorf("data.wp is not base64: %w", err)
	}
	plain, err := k.app.Decrypt(raw)
	if err != nil {
		return fmt.Errorf("the app cannot decrypt the message: %w", err)
	}
	if string(plain) != text {
		return fmt.Errorf("the app decrypted %q, want %q", plain, text)
	}
	k.ok("message reached Google and the app decrypts it", http.StatusOK)
	return nil
}

func origin(base string) string {
	u, err := url.Parse(base)
	if err != nil {
		return base
	}
	return u.Scheme + "://" + strings.ToLower(u.Host)
}

func randomText(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
