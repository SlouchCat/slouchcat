# Konzept 28 · Klickbare Mockups «Live-TV»

Stand: 28. November 2026 · Fassung 1 · Status: Entwurf, nicht freigegeben

Öffne `index.html` per Doppelklick oder über einen einfachen Dateiserver (zum Beispiel
`python3 -m http.server` im Ordner `docs/planning/mockups/`). Alles läuft ohne Vorbereitung, ohne Build
und ohne Netz; Inhalte sind Beispieldaten mit simulierter Wiedergabe.

## Einstiege

| Einstieg | Datei | Was |
|---|---|---|
| Übersicht | `index.html` | vier Einstiege und die Liste aller Seiten und Dialoge mit Stand |
| Web-App | `web/index.html#/live/channels` | Zuschauer; Breitenumschalter Desktop, Tablet, Handy |
| Verwaltung | `web/index.html#/admin/live/sources` | neuer Bereich «Live-TV» der Verwaltung |
| Fernseher | `tv/index.html#/live/channels` | Android TV im gezeichneten Fernseher mit Fernbedienung |
| Handy und Tablet | `handy/index.html#/live/channels` | Handy-App im Geräterahmen, hoch und quer |

Jedes Mockup hat oben eine Leiste mit **Zustände** (Uhr, Profil Anna/Luca/Mia, Rechte, leer, lädt,
Fehler, ohne EPG, Sender gestört) und einem **Sprungmenü** mit allen Seiten (Kennungen LS-, LD-, LA-).

Senderlogos und Sendungsbilder liegen nur lokal in `shared/img/` (nicht im Repo). Fehlen sie, zeigt das
Mockup Initialen auf heller Fläche und ruhige Flächen statt Bildern. Wer Seiten ergänzt, liest
`BAUKASTEN.md`.
