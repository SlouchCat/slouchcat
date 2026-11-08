# Messwerte: Konzept 16 «Webradio», Stufe 1

Konzept `docs/planning/16-webradio.md` (Abschnitte 7.3, 7.7 und 9), ADR 0025. Ziele und wo sie
gemessen werden. Werte vom Entwicklungsrechner stehen in Abschnitt 1, die Messungen auf CT 190
folgen mit der Abnahme nach dem PR (wie bei Konzept 14).

| Grösse | Ziel | Wo |
|---|---|---|
| Erster Ton, neues Relay | höchstens 6 s je Engine (hls.js in Chromium und Firefox, iPhone nativ) | CT 190, Abnahme |
| Erster Ton, laufendes Relay | höchstens 2 s je Engine | CT 190, Abnahme |
| Liste mit Filtern und Zählern | p95 unter 200 ms | CT 190 |
| Suche | p95 unter 150 ms (entscheidet über FTS5) | CT 190 |
| Abgleich des ganzen Katalogs | unter 2 Minuten, ohne Ruckeln einer laufenden Wiedergabe | CT 190 |
| Datenbank | rund 40 MB für Sender, Streams, Schlagworte | CT 190 nach dem ersten Abgleich |
| Sender nach dem Zusammenführen | erwartet 35'000 bis 40'000 von rund 51'000 Einträgen | CT 190 |
| CPU je Relay | unter 2 % eines Kerns, Kopie und Umrechnung getrennt | CT 190 |
| Wiedergabeliste je Abruf | wenige hundert Byte mit Delta-Liste und gzip | CT 190 |
| Arbeitsspeicher je Relay | Bitrate × Zeitversatz (128 kbit/s → 29 MB, 192 → 43 MB, 320 → 72 MB bei 30 min) | CT 190 |
| Ton bei gesperrtem iPhone | läuft weiter (Risiko, Konzept 16 Abschnitt 16) | iPhone, Abnahme |

## 1. Entwicklungsrechner (7. November 2026)

Linux Mint 22.3, i7-14700K, NVMe (wie `docs/perf/M3.md`), Go aus `server/go.mod`, ohne `-race`.

- **Abgleich, 50'000 erzeugte Einträge** (Test der Spur B in `server/internal/radio`): 3,6 s,
  höchster Heap 65 MiB. Die Liste wird gestreamt gelesen; Einträge mit unverändertem
  `lastchangetime` werden nicht neu geschrieben.
- **Eingefrorener Auszug** `testdata/radio/catalog.json`: 68 Einträge werden zu 58 Sendern.
- **Liste mit Zählern über 40'000 Sender:** im Mittel 7,1 ms (Index im Arbeitsspeicher). Ein p95
  folgt auf CT 190.
- **Init-Stück bei AAC-Kopie:** `esds` mit DecoderSpecificInfo (`+delay_moov`), geprüft mit ffmpeg
  6.1.1 (`TestRadioPackagerInitHasAudioSpecificConfig`). Ob die ffmpeg-Fassung im Container gleich
  schreibt, zeigt derselbe Test auf CT 190 bei der Abnahme.

## 2. Messweg auf CT 190

Liste und Suche mit dem vorhandenen Skript `docs/perf/measure-home.sh` (60 Anfragen nach fünf
Aufwärmrunden, auf dem Server selbst):

```
SLOUCHCAT_URL=http://localhost:8096 SLOUCHCAT_EMAIL=… SLOUCHCAT_PASSWORD=… SLOUCHCAT_PROFILE_ID=1 \
  bash docs/perf/measure-home.sh '/api/v1/radio/stations?country=CH&sort=popular'
… bash docs/perf/measure-home.sh '/api/v1/radio/facets?country=CH'
… bash docs/perf/measure-home.sh '/api/v1/radio/stations?q=radio'
```

- **Abgleich:** Dauer und Zähler aus «Hintergrundjobs» (Jobart `radio.catalog.sync`) und aus der
  Übersicht der Radio-Verwaltung; Datenbankgrösse vor und nach dem ersten Lauf
  (`ls -l /config/slouchcat.db*`). Während des Laufs einen Film abspielen und auf Ruckler achten.
- **Erster Ton:** in den Entwicklerwerkzeugen die Zeit vom Klick «Abspielen» bis zum Ereignis
  `playing` des `<audio>`-Elements, je Engine fünfmal mit einem neuen Sender und fünfmal mit einem
  laufenden. iPhone von Hand mit Stoppuhr.
- **CPU und Speicher je Relay:** `docker stats` und `top -p <ffmpeg-pid>` bei einem AAC-Sender
  (Kopie) und einem MP3-Sender (Umrechnung); Pufferstand aus der Leiste «Serverlast».
- **Wiedergabeliste:** Grösse der Antworten auf `/api/v1/radio/live/<listenId>/index.m3u8` mit und ohne `_HLS_skip=YES` in den
  Entwicklerwerkzeugen (übertragene Byte).

## 3. Ergebnisse auf CT 190

### 3.1 Erste Werte aus dem Server-Log (8. November 2026, Release 1.18.1)

CT 190 (4 GB RAM, 3,8 GB frei), ffmpeg 7.1.4-Jellyfin im Container, `tmpfs` `/run/slouchcat/radio` 564 MB.

| Grösse | Wert | Ziel |
|---|---|---|
| Abgleich des ganzen Katalogs | 21 s (`radio.catalog.sync`) | unter 2 Minuten ✔ |
| Sender nach dem Zusammenführen | 55 646 aus 63 562 Einträgen | erwartet 35 000 bis 40 000 (mehr, weil der Katalog seit der Schätzung wuchs) |
| Filter mit Zählern (`/radio/facets`) | 8 bis 17 ms | ✔ |
| Senderliste ohne neue Logos | 6 bis 59 ms | ✔ |
| Start eines neuen Relays (Server, `POST …/listen`) | 1,3 s | Teil der 6 s bis zum ersten Ton |
| Verwaltung «Sender», 50 Einträge | **15 s** | Befund, behoben in 1.18.2 |
| Senderliste mit neuen Logos | **bis 20 s** | Befund, behoben in 1.18.2 |

Ursache der beiden Befunde: das Nachladen eines Senders las den ganzen Katalog (rund 300 ms je Sender).
Mit 1.18.2 liest es nur die Zeilen des Senders; auf dem Entwicklungsrechner sinkt eine Verwaltungsseite
über 40 000 Sender von 4,8 s auf 13 ms.

### 3.2 Offen

Erster Ton je Engine, iPhone mit gesperrtem Bildschirm, CPU und RAM je Relay, p95 von Liste und Suche mit
`measure-home.sh`, Wiedergabeliste je Abruf, `+delay_moov` mit dem ffmpeg im Container, Nachmessung der
Verwaltung mit 1.18.2.
