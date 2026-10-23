package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

// csrfHeader is the header every unsafe request must carry (contract: "Sessions").
const (
	csrfHeader = "X-Requested-With"
	csrfValue  = "Slouchcat"
)

// maxBody bounds what the client reads from one response; the largest body it reads whole is a
// media segment of a few megabytes.
const maxBody = 32 << 20

// client talks to a running server with nothing but the contract: it resolves every call against
// the operation list of api/openapi.yaml and refuses a path the contract does not describe. The
// session cookie and the device cookie travel in the cookie jar, like in a browser.
type client struct {
	base     *url.URL
	contract *contract
	http     *http.Client

	// used collects the operation ids of the running check, in call order, for the report.
	used  []string
	seen  map[string]bool
	calls int
}

// newClient returns a client for the base URL of a running server (scheme, host, no path).
func newClient(base string, c *contract, timeout time.Duration) (*client, error) {
	u, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("parse base URL %q: %w", base, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("base URL %q needs a scheme and a host", base)
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("cookie jar: %w", err)
	}
	return &client{
		base:     u,
		contract: c,
		http:     &http.Client{Jar: jar, Timeout: timeout},
		used:     nil,
		seen:     map[string]bool{},
		calls:    0,
	}, nil
}

// begin starts recording the operations of the next check.
func (c *client) begin() {
	c.used = nil
	c.seen = map[string]bool{}
}

// response is one answer, read whole.
type response struct {
	status int
	header http.Header
	body   []byte
}

// problem is the RFC 9457 body the contract uses for every error.
type problem struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Code   string `json:"code"`
	Status int    `json:"status"`
}

// send performs one request. path is the concrete path below the contract's server URL (for
// example /items/01J.../play); the contract must describe it with this method, otherwise the call
// fails without touching the network.
func (c *client) send(ctx context.Context, method, path string, query url.Values, body any) (*response, error) {
	op, err := c.contract.lookup(method, path)
	if err != nil {
		return nil, err
	}
	if !c.seen[op.id] {
		c.seen[op.id] = true
		c.used = append(c.used, op.id)
	}
	c.calls++

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode body of %s: %w", op.id, err)
		}
		reader = bytes.NewReader(encoded)
	}
	target := *c.base
	target.Path = c.contract.basePath + path
	target.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, method, target.String(), reader)
	if err != nil {
		return nil, fmt.Errorf("build request %s: %w", op.id, err)
	}
	req.Header.Set("Accept", "application/json, application/problem+json, */*")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions {
		req.Header.Set(csrfHeader, csrfValue)
	}
	return c.do(req, op)
}

// sendRaw performs a request on a URL the server itself handed out (a stream URL with its token,
// a playlist entry): the path is checked against the contract just like any other call.
func (c *client) sendRaw(ctx context.Context, method, raw string, header http.Header) (*response, error) {
	ref, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse URL %q: %w", raw, err)
	}
	target := c.base.ResolveReference(ref)
	path := strings.TrimPrefix(target.Path, c.contract.basePath)
	op, err := c.contract.lookup(method, path)
	if err != nil {
		return nil, err
	}
	if !c.seen[op.id] {
		c.seen[op.id] = true
		c.used = append(c.used, op.id)
	}
	c.calls++

	req, err := http.NewRequestWithContext(ctx, method, target.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request %s: %w", op.id, err)
	}
	for name, values := range header {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
	return c.do(req, op)
}

// do runs the request and reads the whole answer.
func (c *client) do(req *http.Request, op operation) (*response, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op.id, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fmt.Errorf("read answer of %s: %w", op.id, err)
	}
	return &response{status: resp.StatusCode, header: resp.Header, body: data}, nil
}

// call is send with the usual expectations: one of want as the status and, when out is not nil,
// a JSON body decoded into it.
func (c *client) call(ctx context.Context, method, path string, query url.Values, body, out any, want ...int) error {
	resp, err := c.send(ctx, method, path, query, body)
	if err != nil {
		return err
	}
	if err := resp.expect(method, path, want...); err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(resp.body, out); err != nil {
		return fmt.Errorf("%s %s: decode answer: %w", method, path, err)
	}
	return nil
}

// get is call for a GET without a body.
func (c *client) get(ctx context.Context, path string, query url.Values, out any) error {
	return c.call(ctx, http.MethodGet, path, query, nil, out, http.StatusOK)
}

// expect checks the status against the wanted ones and turns anything else into an error that
// names the problem code of the contract.
func (r *response) expect(method, path string, want ...int) error {
	for _, status := range want {
		if r.status == status {
			return nil
		}
	}
	var p problem
	if err := json.Unmarshal(r.body, &p); err == nil && p.Code != "" {
		return fmt.Errorf("%s %s answered %d %s: %s", method, path, r.status, p.Code, p.Detail)
	}
	return fmt.Errorf("%s %s answered %d: %s", method, path, r.status, snippet(r.body))
}

// snippet shortens a body for an error message.
func snippet(body []byte) string {
	const limit = 200
	text := strings.TrimSpace(string(body))
	if len(text) > limit {
		return text[:limit] + "…"
	}
	return text
}
