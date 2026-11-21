package pair

import (
	"crypto/sha256"
	"encoding/base64"
	"html/template"
	"net/http"
	"strconv"
	"strings"
)

// The help page is where a QR code of a TV lands when somebody scans it with the camera app
// instead of the Slouchcat app (concept 27 section 7.2): https://<service>/tv#v=1&i=<id>&k=<key>.
// The part after "#" never reaches a server, and the page must not be able to read it either: no
// script at all, and a Content-Security-Policy that forbids every script, every connection and
// every embedding. The one style sheet is allowed by its hash.
//
// The texts are the service's own, in German and English after Accept-Language; the web app's
// locale files are not part of this module.

const helpCSS = `body{margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;` +
	`font-family:system-ui,sans-serif;background:#111;color:#eee}` +
	`main{max-width:32rem;padding:2rem;text-align:center;line-height:1.5}h1{font-size:1.5rem}`

var helpTemplate = template.Must(template.New("tv").Parse(`<!doctype html>
<html lang="{{.Lang}}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="referrer" content="no-referrer">
<title>{{.Title}}</title>
<style>{{.CSS}}</style>
</head>
<body>
<main>
<h1>{{.Title}}</h1>
<p>{{.Text}}</p>
</main>
</body>
</html>
`))

type helpText struct {
	Lang, Title, Text string
	CSS               template.CSS
}

var helpTexts = map[string]helpText{
	"de": {Lang: "de", Title: "Fernseher verbinden", Text: "Öffne die Slouchcat-App und wähle «Fernseher verbinden»."},
	"en": {Lang: "en", Title: "Connect TV", Text: "Open the Slouchcat app and choose “Connect TV”."},
}

// HelpPage renders GET /tv.
type HelpPage struct {
	pages map[string][]byte
	csp   string
}

// NewHelpPage renders both languages once.
func NewHelpPage() *HelpPage {
	sum := sha256.Sum256([]byte(helpCSS))
	p := &HelpPage{
		pages: map[string][]byte{},
		csp: "default-src 'none'; style-src 'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'; " +
			"base-uri 'none'; form-action 'none'; frame-ancestors 'none'",
	}
	for lang, t := range helpTexts {
		t.CSS = template.CSS(helpCSS) //nolint:gosec // a constant of this file, not input
		var b strings.Builder
		if err := helpTemplate.Execute(&b, t); err != nil {
			panic(err)
		}
		p.pages[lang] = []byte(b.String())
	}
	return p
}

// ServeHTTP answers with the page in the language the browser prefers, English by default.
func (p *HelpPage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	lang := PreferredLanguage(r.Header.Get("Accept-Language"))
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Language", lang)
	h.Set("Vary", "Accept-Language")
	h.Set("Content-Security-Policy", p.csp)
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Cross-Origin-Opener-Policy", "same-origin")
	h.Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(p.pages[lang])
}

// PreferredLanguage picks "de" or "en" from an Accept-Language header by quality; ties go to the
// one named first.
func PreferredLanguage(header string) string {
	best, bestQ := "en", -1.0
	for _, part := range strings.Split(header, ",") {
		tag, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		q := 1.0
		if v, ok := strings.CutPrefix(strings.TrimSpace(params), "q="); ok {
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				continue
			}
			q = f
		}
		base, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(tag)), "-")
		if _, ok := helpTexts[base]; ok && q > bestQ && q > 0 {
			best, bestQ = base, q
		}
	}
	return best
}
