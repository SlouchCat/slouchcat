package email

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
	"io"
	"math/big"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	testUser     = "mailer"
	testPassword = "s3cret-smtp-Passw0rd"
)

// fakeSMTP is a minimal SMTP server with a scripted dialogue: EHLO, STARTTLS, AUTH PLAIN and LOGIN,
// MAIL, RCPT, DATA, QUIT. It records the envelope and the message of every delivery.
type fakeSMTP struct {
	t        *testing.T
	ln       net.Listener
	cert     tls.Certificate
	implicit bool     // TLS from the first byte (port 465)
	starttls bool     // offers STARTTLS
	mechs    []string // AUTH mechanisms; empty = no AUTH extension

	mu      sync.Mutex
	mails   []fakeMail
	authed  []string // mechanism of every successful authentication
	tlsUsed []bool   // whether the connection was encrypted when MAIL FROM came
}

type fakeMail struct {
	from, to string
	data     string
}

func newFakeSMTP(t *testing.T, implicit, starttls bool, mechs ...string) (*fakeSMTP, *x509.CertPool) {
	t.Helper()
	cert, pool := selfSigned(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if implicit {
		ln = tls.NewListener(ln, &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
	}
	s := &fakeSMTP{t: t, ln: ln, cert: cert, implicit: implicit, starttls: starttls, mechs: mechs}
	t.Cleanup(func() { _ = ln.Close() })
	go s.serve()
	return s, pool
}

func (s *fakeSMTP) port() int { return s.ln.Addr().(*net.TCPAddr).Port }

func (s *fakeSMTP) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.session(conn)
	}
}

func (s *fakeSMTP) session(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	encrypted := s.implicit
	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)
	say := func(lines ...string) {
		for _, l := range lines {
			_, _ = w.WriteString(l + "\r\n")
		}
		_ = w.Flush()
	}
	read := func() (string, bool) {
		line, err := r.ReadString('\n')
		if err != nil {
			return "", false
		}
		return strings.TrimRight(line, "\r\n"), true
	}
	say("220 fake ESMTP")
	var m fakeMail
	for {
		line, ok := read()
		if !ok {
			return
		}
		verb := strings.ToUpper(strings.Fields(line + " x")[0])
		switch verb {
		case "EHLO", "HELO":
			lines := []string{"250-fake greets you"}
			if s.starttls && !encrypted {
				lines = append(lines, "250-STARTTLS")
			}
			if len(s.mechs) > 0 {
				lines = append(lines, "250-AUTH "+strings.Join(s.mechs, " "))
			}
			say(append(lines, "250 8BITMIME")...)
		case "STARTTLS":
			say("220 go ahead")
			tc := tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{s.cert}, MinVersion: tls.VersionTLS12})
			if err := tc.Handshake(); err != nil {
				return
			}
			conn, encrypted = tc, true
			r, w = bufio.NewReader(conn), bufio.NewWriter(conn)
		case "AUTH":
			f := strings.Fields(line)
			var user, pass string
			switch strings.ToUpper(f[1]) {
			case "PLAIN":
				b, _ := base64.StdEncoding.DecodeString(f[2])
				parts := strings.Split(string(b), "\x00")
				if len(parts) == 3 {
					user, pass = parts[1], parts[2]
				}
			case "LOGIN":
				say("334 " + base64.StdEncoding.EncodeToString([]byte("Username:")))
				l, _ := read()
				u, _ := base64.StdEncoding.DecodeString(l)
				say("334 " + base64.StdEncoding.EncodeToString([]byte("Password:")))
				l, _ = read()
				p, _ := base64.StdEncoding.DecodeString(l)
				user, pass = string(u), string(p)
			}
			if user != testUser || pass != testPassword {
				say("535 5.7.8 Authentication credentials invalid")
				continue
			}
			s.mu.Lock()
			s.authed = append(s.authed, strings.ToUpper(f[1]))
			s.mu.Unlock()
			say("235 2.7.0 Authentication successful")
		case "MAIL":
			m = fakeMail{from: between(line, "<", ">")}
			s.mu.Lock()
			s.tlsUsed = append(s.tlsUsed, encrypted)
			s.mu.Unlock()
			say("250 OK")
		case "RCPT":
			m.to = between(line, "<", ">")
			say("250 OK")
		case "DATA":
			say("354 end with .")
			var data strings.Builder
			for {
				l, ok := read()
				if !ok {
					return
				}
				if l == "." {
					break
				}
				data.WriteString(strings.TrimPrefix(l, ".") + "\r\n")
			}
			m.data = data.String()
			s.mu.Lock()
			s.mails = append(s.mails, m)
			s.mu.Unlock()
			say("250 queued")
		case "QUIT":
			say("221 bye")
			return
		default:
			say("502 unknown")
		}
	}
}

func (s *fakeSMTP) received() ([]fakeMail, []string, []bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]fakeMail(nil), s.mails...), append([]string(nil), s.authed...), append([]bool(nil), s.tlsUsed...)
}

func between(s, open, end string) string {
	_, rest, _ := strings.Cut(s, open)
	v, _, _ := strings.Cut(rest, end)
	return v
}

// selfSigned returns a certificate for 127.0.0.1 and a pool that trusts it.
func selfSigned(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "127.0.0.1"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, DNSNames: []string{"localhost"},
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, pool
}

// parts reads the text and HTML parts of a received message.
func parts(t *testing.T, data string) (*mail.Message, map[string]string) {
	t.Helper()
	msg, err := mail.ReadMessage(strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	media, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	if media != "multipart/alternative" {
		t.Fatalf("content type %s", media)
	}
	mr := multipart.NewReader(msg.Body, params["boundary"])
	for {
		p, err := mr.NextPart() // decodes quoted-printable
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(p)
		if err != nil {
			t.Fatal(err)
		}
		kind, _, _ := mime.ParseMediaType(p.Header.Get("Content-Type"))
		out[kind] = string(b)
	}
	return msg, out
}

func TestSendModes(t *testing.T) {
	cases := []struct {
		name               string
		implicit, starttls bool
		mode               string
		mechs              []string
		wantAuth           string
	}{
		{"starttls with PLAIN", false, true, TLSStartTLS, []string{"PLAIN", "LOGIN"}, "PLAIN"},
		{"implicit TLS with LOGIN", true, false, TLSImplicit, []string{"LOGIN"}, "LOGIN"},
		{"none to localhost with PLAIN", false, false, TLSNone, []string{"PLAIN"}, "PLAIN"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, pool := newFakeSMTP(t, tc.implicit, tc.starttls, tc.mechs...)
			s := Sender{Host: "127.0.0.1", Port: srv.port(), User: testUser, Password: testPassword,
				From: "Slouchcat <slouchcat@example.ch>", TLS: tc.mode, RootCAs: pool,
				Now: func() time.Time { return time.Date(2026, 10, 21, 9, 30, 0, 0, time.UTC) }}
			err := s.Send(context.Background(), "gast@example.ch", "[Kind] Neue Folge: Lümmel & Co", "Text mit Ümlaut\nund Link", "<p>HTML</p>")
			if err != nil {
				t.Fatal(err)
			}
			mails, authed, tlsUsed := srv.received()
			if len(mails) != 1 {
				t.Fatalf("mails = %d", len(mails))
			}
			m := mails[0]
			if m.from != "slouchcat@example.ch" || m.to != "gast@example.ch" {
				t.Errorf("envelope = %s -> %s", m.from, m.to)
			}
			if len(authed) != 1 || authed[0] != tc.wantAuth {
				t.Errorf("authentication = %v, want %s", authed, tc.wantAuth)
			}
			if want := tc.mode != TLSNone; tlsUsed[0] != want {
				t.Errorf("encrypted = %v, want %v", tlsUsed[0], want)
			}
			msg, body := parts(t, m.data)
			subject, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
			if err != nil || subject != "[Kind] Neue Folge: Lümmel & Co" {
				t.Errorf("subject = %q (%v)", subject, err)
			}
			if got := msg.Header.Get("Date"); got != "Wed, 21 Oct 2026 09:30:00 +0000" {
				t.Errorf("date = %q", got)
			}
			if id := msg.Header.Get("Message-Id"); !strings.HasSuffix(id, "@example.ch>") || !strings.HasPrefix(id, "<") {
				t.Errorf("message id = %q", id)
			}
			if msg.Header.Get("From") != `"Slouchcat" <slouchcat@example.ch>` || msg.Header.Get("To") != "<gast@example.ch>" {
				t.Errorf("from %q to %q", msg.Header.Get("From"), msg.Header.Get("To"))
			}
			if body["text/plain"] != "Text mit Ümlaut\r\nund Link" || body["text/html"] != "<p>HTML</p>" {
				t.Errorf("parts = %q", body)
			}
		})
	}
}

// Wrong credentials, a server without STARTTLS and credentials over plain text to another host
// fail, and no error carries the password.
func TestSendErrorsKeepThePasswordOut(t *testing.T) {
	plain, _ := newFakeSMTP(t, false, false, "PLAIN")
	tlsSrv, pool := newFakeSMTP(t, false, true, "PLAIN")
	cases := []struct {
		name string
		s    Sender
		want string
	}{
		{"wrong password", Sender{Host: "127.0.0.1", Port: tlsSrv.port(), User: testUser, Password: "wrong-" + testPassword,
			From: "a@example.ch", TLS: TLSStartTLS, RootCAs: pool}, "535"},
		{"no STARTTLS offered", Sender{Host: "127.0.0.1", Port: plain.port(), User: testUser, Password: testPassword,
			From: "a@example.ch", TLS: TLSStartTLS}, "STARTTLS"},
		{"untrusted certificate", Sender{Host: "127.0.0.1", Port: tlsSrv.port(), User: testUser, Password: testPassword,
			From: "a@example.ch", TLS: TLSStartTLS}, "certificate"},
		{"bad sender", Sender{Host: "127.0.0.1", Port: plain.port(), From: "not an address", TLS: TLSNone}, "sender"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err := tc.s.Send(ctx, "b@example.ch", "Betreff", "Text", "")
			if err == nil {
				t.Fatal("sent")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
			if strings.Contains(err.Error(), testPassword) || strings.Contains(err.Error(), base64.StdEncoding.EncodeToString([]byte(testPassword))) {
				t.Errorf("error carries the password: %v", err)
			}
		})
	}
	if mails, _, _ := tlsSrv.received(); len(mails) != 0 {
		t.Errorf("mails after failures: %d", len(mails))
	}
}

// LOGIN over plain text to a host other than localhost is refused before the password is sent.
func TestLoginAuthNeedsTLS(t *testing.T) {
	a := loginAuth{user: testUser, password: testPassword, host: "mail.example.ch"}
	if _, _, err := a.Start(&smtp.ServerInfo{Name: "mail.example.ch"}); err == nil {
		t.Error("LOGIN started over plain text to a remote host")
	}
	if _, _, err := a.Start(&smtp.ServerInfo{Name: "mail.example.ch", TLS: true}); err != nil {
		t.Errorf("LOGIN over TLS: %v", err)
	}
}

// Text only: one text/plain part; a subject with a line break stays on one header line.
func TestSendTextOnly(t *testing.T) {
	srv, _ := newFakeSMTP(t, false, false)
	s := Sender{Host: "127.0.0.1", Port: srv.port(), From: "slouchcat@example.ch", TLS: TLSNone}
	if err := s.Send(context.Background(), "b@example.ch", "Zeile 1\r\nBcc: x@example.ch", "nur Text", ""); err != nil {
		t.Fatal(err)
	}
	mails, _, _ := srv.received()
	msg, err := mail.ReadMessage(strings.NewReader(mails[0].data))
	if err != nil {
		t.Fatal(err)
	}
	if msg.Header.Get("Bcc") != "" || msg.Header.Get("Subject") != "Zeile 1 Bcc: x@example.ch" {
		t.Errorf("headers = %v", msg.Header)
	}
	if ct := msg.Header.Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("content type %q", ct)
	}
	b, _ := io.ReadAll(msg.Body)
	if strings.TrimRight(string(b), "\r\n") != "nur Text" { // the DATA terminator ends the last line
		t.Errorf("body %q", b)
	}
}

// A cancelled context ends a delivery that hangs in the dialogue.
func TestSendRespectsContext(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			defer func() { _ = conn.Close() }()
			time.Sleep(3 * time.Second) // never greets
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	s := Sender{Host: "127.0.0.1", Port: ln.Addr().(*net.TCPAddr).Port, From: "a@example.ch", TLS: TLSNone}
	start := time.Now()
	err = s.Send(ctx, "b@example.ch", "x", "y", "")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want deadline exceeded", err)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("took %v", d)
	}
}
