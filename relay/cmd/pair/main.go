// Command pair is the meeting point service of Slouchcat ("Vermittlung", concept 27 section 7):
// a TV and a phone find each other through it when the TV is set up by scanning its QR code.
//
// It holds no secret and no state on disk (D-1, D-6). Settings come from the environment; see
// docs/relay/BETRIEB.md. `pair -healthcheck` is the container health check.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/slouchcat/slouchcat/relay/internal/clientip"
	"github.com/slouchcat/slouchcat/relay/internal/limit"
	"github.com/slouchcat/slouchcat/relay/internal/logging"
	"github.com/slouchcat/slouchcat/relay/internal/pair"
	"github.com/slouchcat/slouchcat/relay/internal/serve"
)

var version = "dev"

func main() {
	healthcheck := flag.Bool("healthcheck", false, "ask the running service for /v1/healthz and exit 0 when it is up")
	flag.Parse()
	listen := serve.Env("RELAY_LISTEN", ":8080")
	if *healthcheck {
		os.Exit(serve.Healthcheck(listen))
	}
	if err := run(listen); err != nil {
		fmt.Fprintln(os.Stderr, "pair:", err)
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
	ttl, err := pairTTL()
	if err != nil {
		return err
	}
	maxOpen, err := serve.EnvInt("RELAY_PAIR_MAX_OPEN", 10000)
	if err != nil {
		return err
	}
	perCaller, err := serve.EnvInt("RELAY_PAIR_MAX_OPEN_PER_CALLER", 10)
	if err != nil {
		return err
	}
	perNet, err := serve.EnvInt("RELAY_PAIR_MAX_OPEN_PER_NET", 50)
	if err != nil {
		return err
	}
	rate, err := serve.EnvFloat("RELAY_RATE", 2)
	if err != nil {
		return err
	}
	burst, err := serve.EnvInt("RELAY_BURST", 30)
	if err != nil {
		return err
	}
	netRate, err := serve.EnvFloat("RELAY_NET_RATE", 20)
	if err != nil {
		return err
	}
	netBurst, err := serve.EnvInt("RELAY_NET_BURST", 300)
	if err != nil {
		return err
	}
	store := pair.NewStore(ttl, pair.Limits{MaxOpen: maxOpen, MaxPerCaller: perCaller, MaxPerNet: perNet})
	lim := limit.New(rate, burst, 100000)
	netLim := limit.New(netRate, netBurst, 100000)
	untrusted := logging.NewThrottle(log, time.Hour)
	callers.Untrusted = func() {
		untrusted.Warn("untrusted_forward", "X-Forwarded-For from a peer that is not a trusted proxy; check RELAY_TRUSTED_PROXIES")
	}
	go func() {
		for range time.Tick(10 * time.Second) {
			store.Expire()
			lim.Sweep()
			netLim.Sweep()
		}
	}()
	h := &pair.Handler{Store: store, Callers: callers, Limit: lim, NetLimit: netLim, Log: log,
		Warn: logging.NewThrottle(log, time.Minute), HelpPage: pair.NewHelpPage()}
	log.Info("pair service starting", "version", version, "trusted_proxies", callers.Count(), "ttl", ttl.String())
	return serve.Run(log, listen, h.Routes())
}

// pairTTL reads RELAY_PAIR_TTL: five minutes by default, shorter only for tests, never longer
// (K-1, D-1).
func pairTTL() (time.Duration, error) {
	ttl, err := serve.EnvDuration("RELAY_PAIR_TTL", 5*time.Minute)
	if err != nil {
		return 0, err
	}
	if ttl > 5*time.Minute {
		return 0, fmt.Errorf("RELAY_PAIR_TTL %s: at most 5m (K-1, D-1)", ttl)
	}
	return ttl, nil
}
