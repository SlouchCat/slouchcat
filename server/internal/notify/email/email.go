// Package email sends the e-mails of the notification channels (M8, A-6) through an SMTP server
// with net/smtp: implicit TLS (tls), STARTTLS (starttls) or a plain connection (none), PLAIN or
// LOGIN authentication when a user is set, and a MIME message with a text and an optional HTML
// part (multipart/alternative). The package has no database and logs nothing; the password never
// appears in a returned error. net/smtp refuses to send credentials over an unencrypted connection
// to any host but localhost, so TLS none with a user works only against a local relay.
package email

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Encryption modes of the connection (SLOUCHCAT_SMTP_TLS).
const (
	TLSStartTLS = "starttls"
	TLSImplicit = "tls"
	TLSNone     = "none"
)

// Timeout bounds one delivery (connect, dialogue, data) unless ctx ends earlier.
const Timeout = 30 * time.Second

// Sender is one SMTP server with its account. From is the sender address, optionally with a name
// ("Slouchcat <slouchcat@example.ch>").
type Sender struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
	TLS      string         // TLSStartTLS (default), TLSImplicit or TLSNone
	RootCAs  *x509.CertPool // nil = the system roots; tests trust their own certificate
	Now      func() time.Time
}

// Send delivers one message to the address to. html may be empty (text only).
func (s Sender) Send(ctx context.Context, to, subject, text, html string) error {
	from, err := mail.ParseAddress(s.From)
	if err != nil {
		return errors.New("invalid sender address")
	}
	rcpt, err := mail.ParseAddress(to)
	if err != nil {
		return errors.New("invalid recipient address")
	}
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	msg, err := buildMessage(from, rcpt, subject, text, html, now())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	err = s.deliver(ctx, from.Address, rcpt.Address, msg)
	if err != nil && ctx.Err() != nil {
		// A closed connection after a timeout or a cancellation says less than the context.
		return fmt.Errorf("smtp %s: %w", s.addr(), ctx.Err())
	}
	return err
}

func (s Sender) addr() string {
	return net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
}

func (s Sender) tlsConfig() *tls.Config {
	return &tls.Config{ServerName: s.Host, RootCAs: s.RootCAs, MinVersion: tls.VersionTLS12}
}

// deliver runs the SMTP dialogue. Errors name the step and carry the server's answer, never the
// credentials.
func (s Sender) deliver(ctx context.Context, from, to string, msg []byte) error {
	mode := s.TLS
	if mode == "" {
		mode = TLSStartTLS
	}
	if mode != TLSStartTLS && mode != TLSImplicit && mode != TLSNone {
		return fmt.Errorf("unknown encryption mode %q", mode)
	}
	dialer := &net.Dialer{Timeout: Timeout}
	var conn net.Conn
	var err error
	if mode == TLSImplicit {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: s.tlsConfig()}).DialContext(ctx, "tcp", s.addr())
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", s.addr())
	}
	if err != nil {
		return fmt.Errorf("smtp connect %s: %w", s.addr(), err)
	}
	// The dialogue of net/smtp knows no context: a deadline bounds it and a cancellation closes the
	// connection.
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	c, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp greeting: %w", err)
	}
	defer func() { _ = c.Close() }()
	if err := c.Hello(helloName(from)); err != nil {
		return fmt.Errorf("smtp EHLO: %w", err)
	}
	if mode == TLSStartTLS {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return errors.New("smtp: the server offers no STARTTLS")
		}
		if err := c.StartTLS(s.tlsConfig()); err != nil {
			return fmt.Errorf("smtp STARTTLS: %w", err)
		}
	}
	if s.User != "" {
		auth, err := s.auth(c)
		if err != nil {
			return err
		}
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("smtp authentication: %w", err)
		}
	}
	if err := c.Mail(from); err != nil {
		return fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("smtp RCPT TO: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp message: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp message: %w", err)
	}
	if err := c.Quit(); err != nil {
		return fmt.Errorf("smtp QUIT: %w", err)
	}
	return nil
}

// auth picks PLAIN, else LOGIN, from the mechanisms the server offers.
func (s Sender) auth(c *smtp.Client) (smtp.Auth, error) {
	ok, params := c.Extension("AUTH")
	if !ok {
		return nil, errors.New("smtp: the server offers no authentication")
	}
	mechs := strings.Fields(strings.ToUpper(params))
	switch {
	case slices.Contains(mechs, "PLAIN"):
		return smtp.PlainAuth("", s.User, s.Password, s.Host), nil
	case slices.Contains(mechs, "LOGIN"):
		return loginAuth{user: s.User, password: s.Password, host: s.Host}, nil
	default:
		return nil, fmt.Errorf("smtp: the server offers neither PLAIN nor LOGIN (%s)", params)
	}
}

// loginAuth is the LOGIN mechanism (not in net/smtp) with the same rule as smtp.PlainAuth:
// credentials only over TLS or to localhost.
type loginAuth struct {
	user, password, host string
}

func (a loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if !server.TLS && !isLocalhost(server.Name) {
		return "", nil, errors.New("unencrypted connection")
	}
	if server.Name != a.host {
		return "", nil, errors.New("wrong host name")
	}
	return "LOGIN", nil, nil
}

func (a loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	switch prompt := strings.ToLower(strings.TrimSpace(string(fromServer))); {
	case strings.HasPrefix(prompt, "user"):
		return []byte(a.user), nil
	case strings.HasPrefix(prompt, "pass"):
		return []byte(a.password), nil
	default:
		return nil, errors.New("unexpected LOGIN challenge")
	}
}

func isLocalhost(name string) bool {
	return name == "localhost" || name == "127.0.0.1" || name == "::1"
}

// helloName is the domain of the sender address, the name the server sees in EHLO.
func helloName(from string) string {
	if _, domain, ok := strings.Cut(from, "@"); ok && domain != "" {
		return domain
	}
	return "localhost"
}

// buildMessage writes the RFC 5322 message: headers, then a text/plain part or a
// multipart/alternative body with text and HTML, both UTF-8 and quoted-printable.
func buildMessage(from, to *mail.Address, subject, text, html string, now time.Time) ([]byte, error) {
	var id [12]byte
	_, _ = rand.Read(id[:]) // crypto/rand.Read never fails
	var b bytes.Buffer
	header := func(k, v string) { b.WriteString(k + ": " + v + "\r\n") }
	header("From", from.String())
	header("To", to.String())
	header("Subject", mime.QEncoding.Encode("utf-8", oneLine(subject)))
	header("Date", now.Format(time.RFC1123Z))
	header("Message-ID", "<"+hex.EncodeToString(id[:])+"@"+helloName(from.Address)+">")
	header("MIME-Version", "1.0")
	if html == "" {
		header("Content-Type", "text/plain; charset=utf-8")
		header("Content-Transfer-Encoding", "quoted-printable")
		b.WriteString("\r\n")
		if err := writeQP(&b, text); err != nil {
			return nil, err
		}
		return b.Bytes(), nil
	}
	mw := multipart.NewWriter(&b)
	header("Content-Type", mime.FormatMediaType("multipart/alternative", map[string]string{"boundary": mw.Boundary()}))
	b.WriteString("\r\n")
	for _, part := range []struct{ kind, body string }{{"text/plain", text}, {"text/html", html}} {
		w, err := mw.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {part.kind + "; charset=utf-8"},
			"Content-Transfer-Encoding": {"quoted-printable"},
		})
		if err != nil {
			return nil, err
		}
		if err := writeQP(w, part.body); err != nil {
			return nil, err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func writeQP(w io.Writer, s string) error {
	qp := quotedprintable.NewWriter(w)
	if _, err := qp.Write([]byte(s)); err != nil {
		return err
	}
	return qp.Close()
}

// oneLine keeps a header value on one line (no header injection through a subject).
func oneLine(s string) string {
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool { return r == '\r' || r == '\n' }), " ")
}
