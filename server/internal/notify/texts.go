package notify

import (
	"embed"
	"html"
	"io/fs"
	"strings"

	"github.com/slouchcat/slouchcat/server/internal/i18n"
)

// Texts of the messages the server writes itself (concept 22 section 4.5): embedded i18next JSON
// files, one per language (German in Swiss spelling, English), with {{name}} placeholders. The
// language is the profile's for an event of a profile and the primary profile's for an event of an
// account; a text the language lacks comes from German for de-DE, then from English (i18n.Chain).
// Announcements carry the main user's own words and are not translated.
//
//go:embed locales/*.json
var localeFiles embed.FS

var texts = mustLoadTexts()

func mustLoadTexts() *i18n.Bundle {
	sub, err := fs.Sub(localeFiles, "locales")
	if err != nil {
		panic(err)
	}
	b, err := i18n.Load(sub)
	if err != nil {
		panic(err) // the files are embedded; TestServerTexts reads them in every test run
	}
	return b
}

// vars are the values of the {{name}} placeholders of a text.
type vars = map[string]any

// text returns the text key in lang with the placeholders filled in from v; a key without text
// returns the key, so an unknown reason code still says something.
func text(lang, key string, v ...vars) string {
	var values vars
	if len(v) > 0 {
		values = v[0]
	}
	if t, ok := texts.Text(lang, key, values); ok {
		return t
	}
	return key
}

// message is what a channel sends: subject (title of a push message, first line of a Telegram
// message), plain text body, an optional link (a path of the web app or an absolute URL) and the
// language it is written in (the lang attribute of the HTML mail; none when unknown).
type message struct {
	Subject string
	Body    string
	Link    string
	Lang    string
}

// absolute returns the link as an absolute URL on the public address, "" without one.
func absolute(link, publicURL string) string {
	switch {
	case link == "":
		return ""
	case strings.HasPrefix(link, "https://") || strings.HasPrefix(link, "http://"):
		return link
	case publicURL == "":
		return ""
	}
	return publicURL + link
}

// emailBodies renders the text and HTML part of an e-mail: the body with the link below it.
func emailBodies(m message, publicURL string) (string, string) {
	link := absolute(m.Link, publicURL)
	plain := m.Body
	var h strings.Builder
	h.WriteString(`<!doctype html><html`)
	if m.Lang != "" {
		h.WriteString(` lang="` + html.EscapeString(m.Lang) + `"`)
	}
	h.WriteString(`><body><p>`)
	h.WriteString(strings.ReplaceAll(html.EscapeString(m.Body), "\n", "<br>"))
	h.WriteString(`</p>`)
	if link != "" {
		plain += "\n\n" + link
		h.WriteString(`<p><a href="` + html.EscapeString(link) + `">` + html.EscapeString(link) + `</a></p>`)
	}
	h.WriteString(`</body></html>`)
	return plain, h.String()
}

// telegramText renders a Telegram message: subject, body and link on their own lines.
func telegramText(m message, publicURL string) string {
	out := m.Subject
	if m.Body != "" {
		out += "\n\n" + m.Body
	}
	if link := absolute(m.Link, publicURL); link != "" {
		out += "\n\n" + link
	}
	return out
}

// truncate cuts s to n characters.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
