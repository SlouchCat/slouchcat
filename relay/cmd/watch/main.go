// Command watch is the reachability check of the relay services (concept 27 section 7.5): it asks
// /v1/healthz of each service and sends a message when one goes down and when it is back. Run it
// on a machine outside the network of the services.
//
//	watch -target pair=https://pair.slouchcat.com -target push=https://push.slouchcat.com \
//	      -notify https://notify.example.test/relay [-interval 1m] [-failures 2] [-once]
//
// The message is a POST to -notify with a text/plain body and a Title header. -once checks a
// single time and exits 1 when a service is down (for a cron job or a timer of the system).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/slouchcat/slouchcat/relay/internal/checkclient"
	"github.com/slouchcat/slouchcat/relay/internal/logging"
	"github.com/slouchcat/slouchcat/relay/internal/watch"
)

type targets []watch.Target

func (t *targets) String() string { return fmt.Sprint(*t) }

func (t *targets) Set(v string) error {
	name, u, ok := strings.Cut(v, "=")
	if !ok || name == "" || !strings.HasPrefix(u, "http") {
		return fmt.Errorf("want name=https://host, got %q", v)
	}
	*t = append(*t, watch.Target{Name: name, URL: strings.TrimRight(u, "/") + "/v1/healthz"})
	return nil
}

func main() {
	var ts targets
	flag.Var(&ts, "target", "service to watch, name=base URL (repeatable)")
	notify := flag.String("notify", os.Getenv("RELAY_WATCH_NOTIFY"), "URL that receives the messages (POST, text/plain)")
	interval := flag.Duration("interval", time.Minute, "time between checks")
	failures := flag.Int("failures", 2, "consecutive failed checks before a service counts as down")
	once := flag.Bool("once", false, "check once and exit 1 if a service is down")
	ca := flag.String("ca", "", "PEM file of an extra CA to trust")
	flag.Parse()
	if len(ts) == 0 {
		fmt.Fprintln(os.Stderr, "watch: at least one -target")
		os.Exit(2)
	}
	client, err := checkclient.New(*ca, "", 15*time.Second)
	if err != nil {
		fmt.Fprintln(os.Stderr, "watch:", err)
		os.Exit(2)
	}
	log, _ := logging.New(os.Stderr, "text", nil)
	w := &watch.Watcher{Targets: ts, Client: client, Notify: *notify, Failures: *failures, Log: log}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *once {
		w.Failures = 1
		for _, e := range w.CheckOnce(ctx) {
			if e.Down {
				os.Exit(1)
			}
		}
		return
	}
	w.Run(ctx, *interval)
}
