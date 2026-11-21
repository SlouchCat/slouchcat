// Command push is the push forwarder of Slouchcat ("Push-Weiterleitung", concept 27 section 7.3):
// Slouchcat servers send Web Push to it, and it hands the ciphertext to Google's push service for
// the Android app.
//
// It holds two secrets, both as files outside the image (D-6): the Firebase service account and
// the sealing key file. Without them it still starts and answers 503 (D-10). Settings come from
// the environment; see docs/relay/BETRIEB.md.
//
//	push                         serve
//	push -healthcheck            container health check
//	push -add-key <version> -keyfile <file>
//	                             add a new sealing key version to the file and make it current
package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/relay/internal/clientip"
	"github.com/slouchcat/slouchcat/relay/internal/fcm"
	"github.com/slouchcat/slouchcat/relay/internal/limit"
	"github.com/slouchcat/slouchcat/relay/internal/logging"
	"github.com/slouchcat/slouchcat/relay/internal/push"
	"github.com/slouchcat/slouchcat/relay/internal/seal"
	"github.com/slouchcat/slouchcat/relay/internal/serve"
)

var version = "dev"

func main() {
	healthcheck := flag.Bool("healthcheck", false, "ask the running service for /v1/healthz and exit 0 when it is up")
	addKey := flag.String("add-key", "", "add a sealing key `version` to -keyfile and make it current")
	keyfile := flag.String("keyfile", "", "sealing key file for -add-key (default RELAY_PUSH_KEYFILE)")
	flag.Parse()
	listen := serve.Env("RELAY_LISTEN", ":8080")
	switch {
	case *healthcheck:
		os.Exit(serve.Healthcheck(listen))
	case *addKey != "":
		path := *keyfile
		if path == "" {
			path = serve.Env("RELAY_PUSH_KEYFILE", "")
		}
		if path == "" {
			fmt.Fprintln(os.Stderr, "push: -add-key needs -keyfile")
			os.Exit(2)
		}
		if err := seal.AddVersion(path, *addKey); err != nil {
			fmt.Fprintln(os.Stderr, "push:", err)
			os.Exit(1)
		}
		fmt.Printf("push: version %s added to %s and made current\n", *addKey, path)
		return
	}
	if err := run(listen); err != nil {
		fmt.Fprintln(os.Stderr, "push:", err)
		os.Exit(1)
	}
}

func run(listen string) error {
	log, err := logging.New(os.Stderr, serve.Env("RELAY_LOG_FORMAT", "json"), serve.LogLevel())
	if err != nil {
		return err
	}
	callers, err := clientip.ParseTrusted(serve.Env("RELAY_TRUSTED_PROXIES", ""))
	if err != nil {
		return err
	}
	origins, err := parseOrigins(serve.Env("RELAY_PUSH_ORIGINS", "https://push.slouchcat.com"))
	if err != nil {
		return err
	}
	maxBody, err := serve.EnvInt("RELAY_PUSH_MAX_BODY", 3000)
	if err != nil {
		return err
	}
	callerRate, err := serve.EnvFloat("RELAY_RATE", 20)
	if err != nil {
		return err
	}
	callerBurst, err := serve.EnvInt("RELAY_BURST", 200)
	if err != nil {
		return err
	}
	addrRate, err := serve.EnvFloat("RELAY_PUSH_ADDRESS_RATE", 0.5)
	if err != nil {
		return err
	}
	addrBurst, err := serve.EnvInt("RELAY_PUSH_ADDRESS_BURST", 20)
	if err != nil {
		return err
	}
	regRate, err := serve.EnvFloat("RELAY_PUSH_REGISTER_RATE", 0.2)
	if err != nil {
		return err
	}
	regBurst, err := serve.EnvInt("RELAY_PUSH_REGISTER_BURST", 10)
	if err != nil {
		return err
	}

	h := &push.Handler{
		Origins: origins, MaxBody: maxBody, Callers: callers, Log: log,
		RegisterLimit: limit.New(regRate, regBurst, 100000),
		CallerLimit:   limit.New(callerRate, callerBurst, 100000),
		AddressLimit:  limit.New(addrRate, addrBurst, 200000),
	}
	// Missing secrets do not stop the service: it answers 503 and the health check reports it
	// (D-10). A move without the key file must not turn into deleted subscriptions.
	if path := serve.Env("RELAY_PUSH_KEYFILE", "/run/secrets/sealing-keys.json"); path != "" {
		keys, err := seal.Load(path)
		if err != nil {
			log.Error("sealing keys not loaded; every push answers 503", "err", err)
		} else {
			h.Keys = keys
		}
	}
	if path := serve.Env("RELAY_FCM_CREDENTIALS", "/run/secrets/firebase.json"); path != "" {
		client, err := fcm.Load(path, serve.Env("RELAY_FCM_BASE_URL", ""), serve.Env("RELAY_FCM_TOKEN_URL", ""), &http.Client{Timeout: 15 * time.Second})
		if err != nil {
			log.Error("Firebase credentials not loaded; every push answers 503", "err", err)
		} else {
			h.FCM = client
		}
	}
	go func() {
		for range time.Tick(time.Minute) {
			h.RegisterLimit.Sweep()
			h.CallerLimit.Sweep()
			h.AddressLimit.Sweep()
		}
	}()
	log.Info("push forwarder starting", "version", version, "origins", strings.Join(origins, " "),
		"sealing_versions", h.Keys.Versions(), "firebase", h.FCM != nil, "trusted_proxies", callers.Count(), "max_body", maxBody)
	return serve.Run(log, listen, h.Routes())
}

// parseOrigins reads the own origins: the fixed name first, then any probe name (7.5 step 3).
func parseOrigins(list string) ([]string, error) {
	var out []string
	for _, f := range strings.FieldsFunc(list, func(c rune) bool { return c == ',' || c == ' ' }) {
		u, err := url.Parse(f)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || (u.Path != "" && u.Path != "/") {
			return nil, fmt.Errorf("RELAY_PUSH_ORIGINS: %q is not an origin like https://push.example.test", f)
		}
		out = append(out, u.Scheme+"://"+strings.ToLower(u.Host))
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("RELAY_PUSH_ORIGINS is empty")
	}
	return out, nil
}
