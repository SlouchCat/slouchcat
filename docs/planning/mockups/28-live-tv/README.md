# Konzept 28 · Klickbare Mockups «Live-TV»

Stand: 28. November 2026 · Fassung 1 · Status: Entwurf, nicht freigegeben

Doppelklick auf `index.html` genügt (ohne Server, über `file://`). Wer lieber einen Dateiserver nimmt:
`python3 -m http.server 8728 --directory docs/planning/mockups/28-live-tv` im Repo starten und
`http://localhost:8728/` öffnen. Kein Build, keine Abhängigkeit, kein Netz; alles sind Beispieldaten mit
simulierter Wiedergabe. Rundgang, Abdeckung des Auftrags und Abweichungen: `docs/planning/28-live-tv/19-mockups.md`.

## Einstiege

| Einstieg | Datei | Was |
|---|---|---|
| Übersicht | `index.html` | vier Einstiege und die Liste aller 121 Seiten und Dialoge (alle gebaut) |
| Web-App | `web/index.html#/live/channels` | Zuschauer; Breitenumschalter Desktop, Tablet, Handy |
| Verwaltung | `web/index.html#/admin/live` | neuer Bereich «Live-TV» der Verwaltung |
| Fernseher | `tv/index.html#/live/channels` | Android TV im gezeichneten Fernseher; Fernbedienung mit Kanal +/−, Guide, Info, Aufnahme, Ziffern |
| Handy und Tablet | `handy/index.html#/live/channels` | Handy-App im Geräterahmen, Handy und Tablet, hoch und quer |

## Leiste oben

- **Zustände:** Schalter «Echte Logos und Bilder», Uhr (anhalten, vorspulen), Profil Anna (Hauptprofil), Luca (ab 16), Mia (ab 6), Rechte des
  Profils und Deckel des Kontos, «Recht entzogen während der Wiedergabe», Seiten leer (auf jeder Seite:
  keine Sender, Aufnahmen, Planungen, Erinnerungen, Favoriten, nichts zuletzt gesehen), lädt, Fehler, ohne EPG,
  Sender gestört, viele Sender (2'000 im Raster), Kontingent voll, Fehler im Player (nicht erreichbar,
  Ersatzquelle, Grenze erreicht, Strom stockt, langsamer Start, nicht abspielbar, Server ausgelastet), in der
  Web-App zusätzlich Speicher knapp oder voll, Netzlaufwerk nicht eingehängt, EPG-Import fehlgeschlagen.
  Je Zustand gibt es genau einen Schalter.
- **Seite:** Sprungmenü mit allen Seiten des Clients (Kennungen LS-, LD-, LA-).

## Bilder

Senderlogos und Sendungsbilder liegen nur lokal in `shared/img/` (git-ignoriert, Markenrechte), nie in Git.
Das Mockup lädt sie nur, wenn der Schalter «Echte Logos und Bilder» an ist (auf `index.html` oder unter
«Zustände»; gemerkt im Browser unter `sc28-img`). Ab Werk ist er aus: Initialen auf heller Fläche und ruhige
Flächen statt Bildern, keine Anfrage nach Bilddateien, keine Fehler in der Konsole; bedienbar bleibt alles.
Wer Seiten ändert, liest `BAUKASTEN.md`.
