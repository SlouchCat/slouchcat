// Command fcmfake is the stand-in for Google's push service in local runs and the move rehearsal
// (concept 27 stage 2). It never ships: no release image builds it.
//
//	fcmfake -listen 127.0.0.1:8190 -credentials firebase.json [-token-url http://fcmfake:8190/token]
//
// It writes a service account file for a fresh key to -credentials; the push forwarder reads that
// file as its Firebase credentials and is pointed at the fake with RELAY_FCM_BASE_URL. The token
// endpoint written into the file is -token-url (default http://<listen>/token). Received messages
// are on GET /messages[?token=...]. Device tokens starting with unregistered-, invalid-, fail- or
// busy- play Google's error answers (see package fcmfake).
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/slouchcat/slouchcat/relay/internal/fcmfake"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:8190", "address to listen on")
	creds := flag.String("credentials", "", "write the service account file here")
	tokenURL := flag.String("token-url", "", "token endpoint written into the file (default http://<listen>/token)")
	project := flag.String("project", "slouchcat-fake", "Firebase project id")
	flag.Parse()
	f, err := fcmfake.New(*project)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fcmfake:", err)
		os.Exit(1)
	}
	if *creds != "" {
		u := *tokenURL
		if u == "" {
			u = "http://" + *listen + "/token"
		}
		sa, err := f.ServiceAccount(u)
		if err == nil {
			err = os.WriteFile(*creds, sa, 0o644) //nolint:gosec // a throwaway key of the fake, read by the container user
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "fcmfake:", err)
			os.Exit(1)
		}
	}
	fmt.Printf("fcmfake: project %s on http://%s\n", *project, *listen)
	srv := &http.Server{Addr: *listen, Handler: f.Handler(), ReadHeaderTimeout: 5 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, "fcmfake:", err)
		os.Exit(1)
	}
}
