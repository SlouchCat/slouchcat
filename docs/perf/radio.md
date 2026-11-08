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

### 3.2 Nachmessung mit 1.18.2 (8. November 2026, Server-Log)

Marco hat die Verwaltung «Sender» und die Seite Radio der App durchgeklickt; dabei liefen 155
`radio.logo.fetch` im Hintergrund. 0 ERROR.

| Aufruf | Anzahl | Dauer 1.18.2 | Dauer 1.18.1 |
|---|---|---|---|
| Verwaltung «Sender», Ausnahmen, 50 Einträge | 6 | 31 bis 47 ms | 15 s |
| Verwaltung «Sender», Land CH, Seiten 1 bis 5 | 7 | 29 bis 43 ms | 14,7 s |
| Verwaltung «Sender», CH mit Ausnahmen | 1 | 4 ms | 0,5 bis 0,7 s |
| App: Senderliste mit Filtern, Suche und Favoriten | 32 | 2,6 bis 52 ms | 6 ms bis 20,7 s |
| App: Filter mit Zählern | 10 | 3,9 bis 12 ms | 8 bis 17 ms |
| Start eines neuen Relays (`POST …/listen`) | 3 | 0,9 bis 3,4 s (AAC kopiert und MP3 umgerechnet) | 1,3 s |

Die Ziele «Liste p95 unter 200 ms» und «Suche p95 unter 150 ms» hält schon der langsamste Einzelwert;
ein p95 mit `measure-home.sh` folgt mit der übrigen Abnahme.

### 3.3 Abnahme in Chromium (8. November 2026, 1.18.2)

Gemessen im Browser-Pane der Claude-App (Chromium) am Entwicklungsrechner über das LAN, mit Marcos
Sitzung; Server-Werte aus dem Log und auf CT 190.

**Liste, Suche, Filter:** je 5 Aufwärmrunden und 60 Anfragen, Zeit im Browser (inklusive LAN), 0 Fehler.

| Anfrage | p50 | p95 | max | Ziel |
|---|---|---|---|---|
| `/radio/stations?country=CH&sort=popular` | 13,4 ms | 16,6 ms | 19,0 ms | p95 unter 200 ms ✔ |
| `/radio/stations?sort=popular&limit=60` (ganzer Katalog) | 49,9 ms | 55,3 ms | 65,8 ms | p95 unter 200 ms ✔ |
| `/radio/stations?q=radio` (Suche) | 31,5 ms | 34,3 ms | 37,7 ms | p95 unter 150 ms ✔, FTS5 nicht nötig |
| `/radio/facets?country=CH` | 12,8 ms | 15,6 ms | 21,8 ms | ✔ |
| `/radio/facets` | 14,2 ms | 17,2 ms | 18,4 ms | ✔ |
| `/admin/radio/stations?exceptions=true` (50 Einträge) | 35,7 ms | 40,6 ms | 48,0 ms | ✔ |

**Erster Ton** (Klick auf «abspielen» bis Ereignis `playing`, hls.js in Chromium):

| Fall | Werte | Ziel |
|---|---|---|
| Neues Relay, 6 Sender (MP3 umgerechnet und AAC kopiert) | 0,16 / 0,63 / 0,69 / 1,04 / 1,47 / 1,48 s | höchstens 6 s ✔ |
| Laufendes Relay, 5 Sender | 38 / 39 / 40 / 46 / 156 ms | höchstens 2 s ✔ |

Der Server braucht für den Start eines neuen Relays (`POST …/listen`) 0,12 bis 3,4 s, je nach Sender;
bei einem laufenden 1 bis 2 ms.

**CPU und Speicher je Relay** (`top` über 15 s, sieben Relays gleichzeitig):

| Relay | CPU eines Kerns | RSS ffmpeg | Ziel |
|---|---|---|---|
| AAC kopiert (`-c:a copy`, `+delay_moov`) | 0,1 % | 16 MB | unter 2 % ✔ |
| MP3 umgerechnet nach AAC 128 bis 192 kbit/s | 1,8 bis 3,0 % | 18 MB | unter 2 % knapp verfehlt (6 Relays, im Mittel 2,3 %) |

Der Container belegte mit sieben Relays 218 MiB. Der Puffer in der `tmpfs` wächst bei 128 kbit/s um rund
1 MB je Minute (7,1 MB nach 7 Minuten) und passt damit zur Reservierung von 29,7 MB für 30 Minuten; die
`tmpfs` war mit sechs Puffern zu 4 % belegt.

**Wiedergabeliste je Abruf:** mit `_HLS_skip=YES` und gzip 614 bis 661 Byte übertragen (20 Abrufe,
Kopfzeilen inbegriffen) ✔; die volle Liste mit 27 Stücken hat 3014 Byte, die Delta-Liste 1274 Byte vor gzip.

**`+delay_moov` mit dem ffmpeg im Container** (7.1.4-Jellyfin): das Init-Stück eines kopierten
AAC-Senders enthält `esds`; ffprobe liest aus Init und erstem Stück `aac`, Profil LC, 44,1 kHz, Stereo,
AudioSpecificConfig 2 Byte ✔.

### 3.4 Offen

Erster Ton in Firefox, iPhone (nativ) mit gesperrtem Bildschirm; beides von Hand.
