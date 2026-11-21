// Package checkclient builds the HTTP client of the check programs (paircheck, pushcheck, watch):
// optionally trusting an extra CA (a local test CA for the move rehearsal) and optionally dialling
// a fixed address instead of the one the name resolves to, so a probe name can be checked before
// any DNS entry exists (concept 27 section 7.5, step 3).
package checkclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

// New returns a client. caFile adds a PEM CA to the system roots; connect ("127.0.0.1:8443")
// replaces the address of every connection while the URL, the SNI and the Host stay the name.
func New(caFile, connect string, timeout time.Duration) (*http.Client, error) {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if caFile != "" {
		pem, err := os.ReadFile(caFile) //nolint:gosec // a path the operator passes on the command line
		if err != nil {
			return nil, err
		}
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("%s: no certificate", caFile)
		}
		tr.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	}
	if connect != "" {
		d := &net.Dialer{Timeout: 5 * time.Second}
		tr.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
			return d.DialContext(ctx, network, connect)
		}
	}
	return &http.Client{Transport: tr, Timeout: timeout}, nil
}
