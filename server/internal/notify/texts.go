package notify

import (
	"fmt"
	"html"
	"strings"
)

// Texts of the messages the server writes itself, in the two languages of the profiles (German in
// Swiss spelling and English). The language is the profile's for an event of a profile and the
// primary profile's for an event of an account. Announcements carry the main user's own words.
var texts = map[string]map[string]string{
	"de": {
		"new_episode":          "Neue Folge: %s",
		"new_podcast_episode":  "Neue Episode: %s",
		"invitation_subject":   "Einladung angenommen",
		"invitation_body":      "%s hat deine Einladung angenommen und ein Konto erstellt.",
		"job_failed_subject":   "Fehler: Job %s",
		"job_failed_body":      "Der Job %s ist nach %d Versuchen fehlgeschlagen.\n\n%s",
		"scan_aborted_subject": "Scan abgebrochen: %s",
		"scan_aborted_body":    "Der Scan der Bibliothek %s wurde abgebrochen, nichts wurde geändert.\n\n%s",
		"root_missing":         "Ein Ordner der Bibliothek fehlt (Laufwerk oder Freigabe nicht verbunden).",
		"root_empty":           "Ein Ordner der Bibliothek ist leer, obwohl Dateien darin bekannt sind (nicht eingehängt?).",
		"loss_over_threshold":  "Mehr als 20 % der bekannten Dateien wären verschwunden. Unter Verwaltung › Bibliotheken lässt sich der Verlust bestätigen.",
		"acquisition_subject":  "Beschaffung: %s",
		"acquisition_body":     "Neuer Stand: %s",
		"test_subject":         "Slouchcat: Testnachricht",
		"test_body":            "Diese Nachricht bestätigt, dass der Kanal %s funktioniert.",
		"telegram_linked":      "Slouchcat: Dieser Chat ist jetzt mit dem Konto %s verknüpft.",
		"channel_email":        "E-Mail",
		"channel_push":         "Push",
		"channel_telegram":     "Telegram",
	},
	"en": {
		"new_episode":          "New episode: %s",
		"new_podcast_episode":  "New episode: %s",
		"invitation_subject":   "Invitation accepted",
		"invitation_body":      "%s accepted your invitation and created an account.",
		"job_failed_subject":   "Error: job %s",
		"job_failed_body":      "The job %s failed after %d attempts.\n\n%s",
		"scan_aborted_subject": "Scan aborted: %s",
		"scan_aborted_body":    "The scan of the library %s was aborted; nothing was changed.\n\n%s",
		"root_missing":         "A folder of the library is missing (disk or share not attached).",
		"root_empty":           "A folder of the library is empty although files in it are known (not mounted?).",
		"loss_over_threshold":  "More than 20 % of the known files would have disappeared. Administration › Libraries can accept the loss.",
		"acquisition_subject":  "Acquisition: %s",
		"acquisition_body":     "New state: %s",
		"test_subject":         "Slouchcat: test message",
		"test_body":            "This message confirms that the channel %s works.",
		"telegram_linked":      "Slouchcat: this chat is now linked to the account %s.",
		"channel_email":        "e-mail",
		"channel_push":         "push",
		"channel_telegram":     "Telegram",
	},
}

// text formats the text key in lang (German for an unknown language); a key without text returns
// the key, so an unknown reason code still says something.
func text(lang, key string, args ...any) string {
	t, ok := texts[lang]
	if !ok {
		t = texts["de"]
	}
	format, ok := t[key]
	if !ok {
		return key
	}
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}

// message is what a channel sends: subject (title of a push message, first line of a Telegram
// message), plain text body and an optional link (a path of the web app or an absolute URL).
type message struct {
	Subject string
	Body    string
	Link    string
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
	h.WriteString(`<!doctype html><html><body><p>`)
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
