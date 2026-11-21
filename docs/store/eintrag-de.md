# Store-Eintrag Deutsch

Texte für den Play-Store-Eintrag von `com.slouchcat.app` in Deutsch (Play Console: «Deutsch –
de-DE»). Die englische Fassung steht in `listing-en.md`, die Grafiken in `grafiken.md`. Grundlage:
Konzept 27 Abschnitt 11.4, Konzept 26 Abschnitt 8.2.

**Grundsatz (K27 11.4):** Der Text beschreibt die App als Abspieler für die **eigene**
Mediensammlung auf dem **eigenen** Server. Kein Wort, das nach Beschaffung von Inhalten klingt:
keine Namen von Anbietern, keine «kostenlosen Filme», kein «alles streamen», keine Hinweise auf
Downloads aus dem Internet. «Herunterladen» heisst hier nur: eine Kopie vom eigenen Server aufs
Gerät legen; der Text sagt darum «offline speichern».

**Schreibweise:** Der Eintrag geht an Leser in Deutschland, Österreich und der Schweiz. Die Texte
vermeiden darum Wörter, die in der Schweiz mit ss und in Deutschland mit ß stehen; wer sie
ändert, prüft das wieder.

**Bewusst nicht im Text:** die Wünsche über Seerr (E-13). Sie sind eine Funktion der App, aber
«Titel wünschen» liest sich im Store leicht als Beschaffung. Ob sie in den Text kommen, entscheidest
du (offene Punkte am Ende).

## Angaben

| Feld | Grenze | Inhalt |
|---|---|---|
| App-Name | 30 Zeichen | `Slouchcat` (9) |
| Kurzbeschreibung | 80 Zeichen | siehe unten (74) |
| Vollständige Beschreibung | 4000 Zeichen | siehe unten (2163) |
| «Was ist neu» | 500 Zeichen je Sprache (Angabe aus Fremdquellen, im Formular nachsehen) | siehe unten (319) |
| App oder Spiel | – | App |
| Kategorie | – | Video-Player & -Editoren |
| Tags | bis 5 aus Googles Liste | Vorschlag: Videoplayer, Medienplayer, Hörbücher, Podcasts, Streaming. Welche Tags die Liste anbietet, ist unbelegt; im Formular aus der Liste wählen |
| E-Mail-Adresse | Pflicht, öffentlich | `<E-Mail-Adresse für den Store>` |
| Website | freiwillig | `<Website des Projekts>` |
| Telefon | freiwillig | leer lassen |
| Datenschutzerklärung | Pflicht, HTTPS | `<Adresse der Datenschutzerklärung>` (Seite aus `datenschutz.html`) |

Die Grenzen für Name, Kurzbeschreibung und Beschreibung stehen auf Googles Hilfeseite «Store
listing» (<https://support.google.com/googleplay/android-developer/answer/9859152>, gelesen am
21. November 2026). Die Zahlen in Klammern hat der Bau gezählt.

## Kurzbeschreibung

```text
Filme, Serien und Hörbücher von deinem eigenen Slouchcat-Server abspielen.
```

## Vollständige Beschreibung

```text
Slouchcat ist die App zu deinem eigenen Slouchcat-Server. Der Server läuft bei dir zu Hause oder auf einem Rechner, den du betreibst, und verwaltet deine eigene Mediensammlung: Filme, Serien, Hörbücher und Podcasts. Mit der App schaust und hörst du sie auf dem Handy und dem Tablet.

Die App enthält keine Inhalte und bietet keine an. Sie braucht einen Slouchcat-Server, den du selbst eingerichtet hast oder zu dem dich jemand eingeladen hat.

STÖBERN
• Startseite mit den Reihen deines Servers: Weiterschauen, Neu, Empfehlungen des Haushalts
• Bibliotheken mit Filtern und Sortierung, Suche, Personen, Sammlungen, Merkliste
• Profile für jedes Mitglied des Haushalts, mit PIN und Altersgrenze, die der Server durchsetzt

ABSPIELEN
• Videoplayer mit Tonspuren, Untertiteln, Kapiteln und Vorschaubildern beim Spulen
• Vorspann überspringen, nächste Folge, Fortsetzen an der letzten Stelle
• Der Server wählt für dein Gerät die passende Wiedergabe; wo nötig rechnet er um
• Hörbücher und Podcasts mit Kapiteln, Tempo und Wiedergabe bei ausgeschaltetem Bildschirm
• Webradio-Sender, die dein Server anbietet

OFFLINE
• Filme, Folgen und Hörbücher von deinem Server auf dem Gerät speichern und ohne Netz abspielen
• Automatisch die nächsten Folgen speichern, auf Wunsch nur im WLAN
• Der Fortschritt gleicht sich beim nächsten Kontakt mit dem Server ab

FERNSEHER VERBINDEN
• Die Slouchcat-App auf einem Android-TV-Gerät mit einem Scan des QR-Codes einrichten, ohne Passwort am Fernseher

MELDUNGEN
• Benachrichtigungen deines Servers, zum Beispiel über neue Folgen, auch wenn die App geschlossen ist

DEINE DATEN
Die App spricht mit deinem Server und sonst mit niemandem. Konto, Sammlung und Fortschritt liegen auf deinem Server. Keine Werbung, keine Analyse, kein Konto beim Entwickler. Zwei kleine Hilfsdienste des Projekts vermitteln nur verschlüsselte Nachrichten: einer beim Einrichten eines Fernsehers, einer für Benachrichtigungen. Einzelheiten in der Datenschutzerklärung.

FREIE SOFTWARE
Server und App sind freie Software unter der GNU Affero General Public License, Version 3. Quellcode und Anleitung für den Server: https://github.com/SlouchCat/slouchcat
```

## Zusatz für die TV-Geräteart

Der Store-Eintrag ist für Handy und Fernseher derselbe (E-20); Text und Kurzbeschreibung gelten für
beide. Kommt die TV-App dazu (K26 Stufe 6), ersetzt dieser Absatz den Absatz «FERNSEHER VERBINDEN»:

```text
AUF DEM FERNSEHER
• Dieselbe App läuft auf Android-TV- und Google-TV-Geräten, bedient mit der Fernbedienung
• Einrichten per Scan mit der Handy-App oder von Hand mit Adresse und Anmeldung
• Mehrkanalton und HDR, soweit Gerät und Anlage es können
```

**Banner-Text:** Das Banner zeigt nur den App-Namen `Slouchcat` mit dem Zeichen der App, keinen
weiteren Text (TV-Qualitätsliste: Banner mit App-Namen, K26 2.6).

**TV-Beschreibung:** Eine eigene Beschreibung für den Fernseher hat der Eintrag nicht; der Absatz
oben trägt das. Ob die Play Console für die TV-Geräteart einen eigenen Text verlangt, ist
unbelegt und zeigt sich beim Anlegen der TV-Spur.

## «Was ist neu» (erste Ausgabe)

```text
Die erste Ausgabe von Slouchcat für Android: stöbern und abspielen von deinem eigenen Slouchcat-Server, Profile mit PIN, Videoplayer mit Ton- und Untertitelspuren, Hörbücher, Podcasts und Webradio, offline speichern, Benachrichtigungen und Fernseher per Scan verbinden. Braucht einen Slouchcat-Server ab Fassung 1.42.0.
```

## Noch einzutragen

| Platzhalter | Was |
|---|---|
| `<E-Mail-Adresse für den Store>` | öffentliche Kontaktadresse des Eintrags |
| `<Website des Projekts>` | freiwillig, zum Beispiel die Seite mit der Datenschutzerklärung |
| `<Adresse der Datenschutzerklärung>` | HTTPS-Adresse der veröffentlichten `datenschutz.html` |

Offen, zu entscheiden: ob die Wünsche über Seerr im Text stehen; ob «ab Fassung 1.42.0» in «Was
ist neu» stimmt, wenn die App erscheint.
