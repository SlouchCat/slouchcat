# Konzept: Webradio in Slouchcat

- **Status: Fassung 3 (6. November 2026), Kritiker-Review eingearbeitet (REVISE, 2 Blocker,
  12 Major, 11 Minor; Abschnitt 17), mit Mockups.** Marco hat am 6. November 2026 die
  Grundsatzfragen **F-1 bis F-8** (Abschnitt 2) und die Fragen **F-9 bis F-18** (Abschnitt 13)
  beantwortet: **App A, Verwaltung A**. Abweichend von der Empfehlung: **kein Sperrbildschirm**
  (F-11), **Standardland je Profil** (F-12), **keine Sender im Heimnetz** (F-15), **Radio zählt zur
  täglichen Hörzeit** (F-16), **Favoriten verschwinden mit dem Katalogeintrag** (F-18), **Radio
  zählt nicht zu den Stream-Grenzen W-8** (F-19). Keine Frage ist mehr offen.
  **Die Umsetzung beginnt erst auf Marcos ausdrückliches Wort.**
- **Auslöser:** Marco will in Slouchcat Webradio hören: vorgeladene Sender wie bei TuneIn, schnell
  filterbar (zum Beispiel nur Schweizer Sender), Favoriten, immer ein Senderbild, und Titel, Künstler
  und Cover, wenn der Sender sie mitschickt. Der Server soll den Sender abholen, puffern und
  weitergeben, damit Sender, die nur in der Schweiz zu hören sind, über Slouchcat weltweit laufen.
  Er wollte je drei Mockups für die App und die Verwaltung (Anhang A).
- **Verankert gegen:** `main` **`d2361e54`** («Merge pull request #102», Status zu Release 1.14.0),
  Vertragsfassung **1.14.0** (`api/openapi.yaml:4`), letzte Migration **`00034_job_windows.sql`**,
  letzte ADR in `main` **`0023`** (0024 ist für Konzept 15 reserviert), letzte Frage im
  Entscheidungsprotokoll **22**.
- **Nummern:** Dieses Papier ist **Konzept 16**, reserviert **Frage 23** und **ADR 0025**
  («Webradio: Katalog, Weiterleitung mit Zeitversatz»). Vertrag und Migration werden beim Baubeginn
  vergeben, nach den noch offenen Stufen 3 und 4 von Konzept 15 (dort vorläufig 1.15.0, 1.16.0 und
  00035). Vorläufig gelten **Vertrag 1.17.0 (additiv) und 1.18.0 (brechend für `getHome` und
  `setHomeRows`)** und **Migration 00036 und 00037**.
- `:Zeile`-Angaben beziehen sich auf `d2361e54`; **nach Namen suchen, nicht nach Zeile.** Pfade ohne
  Präfix liegen unter `server/` (Go) oder `web/src/` (Web-App), wo es eindeutig ist.
- Massgeblich bleiben `docs/spec/01-anforderungen-v1.md`, `docs/planning/04-entscheidungen.md` und
  `docs/adr/`. Bei Widerspruch gilt: **Entscheidungsprotokoll > Plan > Spec** (`CLAUDE.md`). Dieses
  Papier hebt ein Nicht-Ziel teilweise auf (Abschnitt 3); erst Marcos Antwort auf Frage 23 ändert es.

> **Dieses Papier ändert keine Zeile Code.** Es beschreibt einen neuen Bereich «Radio» mit eigenem
> Senderkatalog, eigener Weiterleitung und einem Player, der beim Stöbern weiterläuft. Abschnitt 14
> teilt den Bau in zwei Stufen, die je als Release erscheinen.

## 1. Was Marco will

| Nr. | Marcos Wunsch (6. November 2026, sinngemäss) | Abschnitt |
|---|---|---|
| W-1 | Webradio-Sender in Slouchcat abspielen. | 9, 10 |
| W-2 | Der Server holt den Sender ab, puffert ihn und gibt ihn weiter. So hört man Sender, die nur in der Schweiz freigegeben sind, über Slouchcat überall. | 7 |
| W-3 | Vorgeladene Sender wie bei TuneIn. | 5 |
| W-4 | Sortieren und Filtern nach Genre, Land, Musikart und mehr; Schweizer Sender schnell finden. | 5.4, 10.1 |
| W-5 | Favoriten. | 10.2 |
| W-6 | Zu jedem Sender immer ein Bild. | 6 |
| W-7 | Liefert der Stream Titel, Künstler, Cover und Ähnliches, wird es ansprechend gezeigt. | 8 |
| W-8 | Wie sieht das in der Verwaltung und in der App aus: je drei Mockups. | 10.4, 11, Anhang A |

## 2. Interview vom 6. November 2026

| Nr. | Frage | Marcos Antwort |
|---|---|---|
| F-1 | Wie weit reicht die Ausnahme vom Nicht-Ziel «Musik»? | **Nur Live-Webradio.** Keine Musikbibliothek, keine Podcasts aus dem Radio, keine Aufnahmen. |
| F-2 | Woher kommen die Sender? | **Alle Sender aus Radio Browser laden und täglich aktualisieren.** Der Admin kann kuratieren, aber Filter und Einordnung entstehen automatisch: «Der Admin soll nicht 50k Sender kategorisieren müssen.» |
| F-3 | Wann läuft ein Sender über den Server? | **Immer über den Server.** |
| F-4 | Pause und Zurückspulen? | **Ja, bis 30 Minuten** (Empfehlung). |
| F-5 | Wer darf Radio hören? | **Alle Profile, immer.** Kein Schalter je Profil. |
| F-6 | Woher kommen die Cover? | **Kette: MusicBrainz mit Cover Art Archive, dann iTunes-Suche** (Empfehlung). |
| F-7 | Was heisst «Musikart»? | **Beides:** Sendeart (Musik, News, Talk, Sport, Kultur) **und** Stimmung (Chill, Party, Fokus, Schlafen, Workout) als eigene Filter neben dem Genre. |
| F-8 | Wo erscheint Radio ausserhalb der Radioseite? | **Mini-Player überall** und **Startseite: Reihe «Meine Sender»**. Nicht gewählt: Steuerung auf dem Sperrbildschirm und «Zuletzt gehört» (zu Sperrbildschirm siehe F-11). |

## 3. Was sich gegenüber früheren Entscheiden ändert

| Bisher | Neu mit diesem Konzept | Grund |
|---|---|---|
| Spec «Non-Goals» (`01-anforderungen-v1.md:56`): «Musik, Fotos, Live-TV und DVR». | **Ausnahme Webradio:** Live-Radio ist erlaubt. Musikbibliothek, Fotos, Live-TV und DVR bleiben Nicht-Ziele. Der Zeitversatz bis 30 Minuten ist kein DVR: nichts wird dauerhaft gespeichert, der Puffer lebt nur im Arbeitsspeicher, solange jemand hört. | F-1, F-4 |
| Frage 7 (19. Oktober 2026): «Radio ist kein Bestandteil von Spec v1 … für Spec v2 vorgemerkt.» | Radio kommt als eigener Bereich in Version 1.x. | Marcos Auftrag vom 6. November 2026 |
| Medientypen der Spec: Filme, Serien, Hörbücher, Podcasts. | Neuer Bereich **Radio**, aber **kein Medientyp in `items`**: Sender sind keine Titel, haben keine Altersfreigabe und keine Bibliothek. | F-5, Abschnitt 12.1 |
| Startseite: feste Reihen-Schlüssel (`model.HomeRowKeys`; Konzept 15 erweitert auf höchstens 15 Reihen). | Neuer Reihen-Schlüssel **`radio`** («Meine Sender»), zählt zu den 15 Reihen. | F-8 |
| Tägliches Limit je Profil (K-5) misst Wiedergabezeit von Titeln. | Radio-Hörzeit zählt mit (7.10). | F-16 |
| Grenzen je Profil und Gast (W-8): Streams und Bitrate. | Gelten für Radio **nicht**. | F-19 |
| Die App hat keinen Player, der beim Navigieren weiterläuft: `AudioPlayerPage` lebt in der Route `/play/$itemId` ausserhalb von `AppLayout` (`app/router.tsx`, `playRoute`). | Neuer **Radio-Player auf App-Ebene** mit Mini-Player in `AppLayout`. Hörbücher und Podcasts bleiben, wie sie sind. | F-8, Abschnitt 10.3 |

Regel 2 (Titelabfragen nur über `visibility`) und Regel 12 (Entscheid im Server) bleiben unberührt:
Radio fragt weder `items` noch `media_files` noch `tracks` ab, und es gibt keine Sichtbarkeitsregel,
die der Client nachbauen könnte (F-5).

## 4. Überblick

```
Radio Browser ──täglich──▶ radio.catalog.sync ──▶ radio_stations (≈ 50'000)
                                                     │  Einordnung: Regeln + Admin-Ausnahmen
                                                     ▼
App: Radio-Seite ── Filter, Suche, Favoriten ──▶ GET /radio/stations
     Klick «Abspielen» ──▶ POST /radio/stations/{id}/listen ──▶ HLS-Adresse mit Token
                                                     │
Server: Relay je Sender (eine Verbindung zum Sender, egal wie viele hören)
        Sender ──ICY/HLS──▶ Go (netguard) trennt Ton und Titelangaben
               Ton ──pipe──▶ ffmpeg packt HLS (4-s-Stücke, ffmpeg ohne Netz)
        Puffer 30 min im RAM (tmpfs) ──▶ alle Hörer, jeder an seiner Stelle (live oder zurück)
        Titelangaben ──▶ Zeitleiste nach Ton-Position ──▶ Cover-Suche (MusicBrainz → iTunes, Cache)
```

## 5. Senderkatalog und automatische Einordnung

### 5.1 Quelle: Radio Browser

[Radio Browser](https://www.radio-browser.info) ist eine offene, von der Gemeinschaft gepflegte
Senderdatenbank mit rund 50'000 Einträgen. Die Daten sind nach Angabe des Betreibers gemeinfrei,
die Server-Software steht unter GPL; der Lizenztext wird vor dem Bau auf der Webseite des Betreibers
nachgelesen und im ADR zitiert. Je Sender liefert die API unter anderem: `stationuuid`, `name`,
`url` und `url_resolved`, `homepage`, `favicon`, `tags` (freie Schlagworte, kommagetrennt),
`countrycode` (ISO 3166-1), `iso_3166_2` (Region, z. B. `CH-ZH`, oft leer), `state` (freier Text),
`language` und `languagecodes`, `codec`, `bitrate`, `hls`, `lastcheckok`, `votes`, `clickcount`,
`clicktrend` und `lastchangetime`.

- Abruf über die Spiegel-Liste (`all.api.radio-browser.info`), mit eigenem User-Agent
  `Slouchcat/<Version>` wie bei den Podcast-Feeds (`metadata.WithFeedUserAgent`) und über
  `ratelimit` (1 Anfrage je Sekunde reicht).
- **Keine Rückmeldung an Radio Browser:** Slouchcat meldet weder Klicks noch Stimmen
  (Empfehlung, F-14). Der Server verrät dem Katalog damit nicht, was im Haushalt gehört wird.

### 5.2 Täglicher Abgleich

Neue Jobart **`radio.catalog.sync`** in der Gruppe der Metadaten-Jobs, registriert wie
`trakt.refresh` (`metadata/metadata.go`, `jobs.Type` mit `TriggerSchedule`, `Every`, `Seed`):

- **Zeitplan:** alle 24 Stunden mit Zeitfenster ab 04:30 (der Scheduler kennt Intervalle und
  Zeitfenster, keine Uhrzeiten; Konzept 09 und 15), einstellbar wie jede Jobart. **Erster Lauf:**
  eigener Seed, der den Abgleich einmal 10 Minuten nach dem Serverstart einreiht, solange der
  Katalog leer ist (`StartDelay`). Kein Lesezugriff der App löst einen Abgleich aus; bis zum
  ersten Lauf zeigt die Radioseite «Senderkatalog wird geladen» und die Verwaltung den Fortschritt.
- **Ablauf:** ganze Liste holen (rund 30 bis 40 MB JSON, gestreamt gelesen), je `stationuuid`
  vergleichen, neue Einträge anlegen, geänderte nachführen. Verschwundene Einträge werden als
  «nicht mehr im Katalog» markiert und ist in der App 7 Tage lang mit dem Hinweis «Nicht mehr im
  Katalog, verschwindet am …» zu sehen. Danach wird er gelöscht, **samt Favoriten und Schlössern**
  (F-18). Eigene Sender (5.6) sind nicht betroffen. Danach läuft die Einordnung (5.4) für alle
  geänderten Sender.
- **Fortschritt und Abbruch** über die vorhandenen Haken der Job-Queue (ADR 0017).
- **Was nie überschrieben wird:** Felder, die der Admin von Hand gesetzt hat (Schloss, 5.5), und
  eigene Sender (5.6).
- **Gestörte Sender:** `lastcheckok = 0` heisst bei Radio Browser «letzte Prüfung fehlgeschlagen».
  Solche Sender sind in der App standardmässig ausgeblendet (Einstellung, Vorgabe an). Fällt ein
  Sender beim eigenen Abspielen dreimal hintereinander aus, merkt der Server sich das ebenfalls.

### 5.3 Ein Sender, mehrere Streams

Radio Browser führt viele Sender mehrfach: je Bitrate, je Format, je Spiegel. Slouchcat fasst
Einträge mit gleichem normalisiertem Namen, gleichem Land und gleicher Webseite (Host) zu **einem
Sender mit mehreren Streams** zusammen. Gespielt wird der beste erreichbare Stream: zuerst
`lastcheckok`, dann AAC vor MP3 bei gleicher Bitrate, dann höchste Bitrate bis 320 kbit/s. Der Admin
kann den Stream je Sender festlegen. Das senkt die Zahl der sichtbaren Sender spürbar; der genaue
Wert wird beim Bau gemessen (Erwartung: von rund 51'000 Einträgen bleiben 35'000 bis 40'000 Sender).

**Stabile Kennung.** Favoriten und Schlösser hängen an der Slouchcat-Kennung des Senders, nicht an
der Gruppe. Beim ersten Zusammenführen bekommt der Sender eine eigene Kennung; seine Katalogeinträge
stehen in `radio_station_aliases` (`stationuuid` → Sender). Beim Abgleich gilt:

- Ein Eintrag, der schon einem Sender gehört, bleibt bei ihm, auch wenn sich Name oder Webseite
  ändern (der Katalog benennt um, die Gruppe zerfällt nicht).
- Ein neuer Eintrag wird über Name, Land und Host einem bestehenden Sender zugeordnet oder bildet
  einen neuen.
- Verschmelzen zwei Sender, bleibt die ältere Kennung; Favoriten und Schlösser des anderen ziehen
  um, die alte Kennung leitet weiter.
- Trennen kann nur der Admin («Stream abtrennen»), nie der Abgleich.

### 5.4 Automatische Einordnung

Marcos Vorgabe (F-2): Filter entstehen automatisch, der Admin pflegt nur Ausnahmen. Dafür leitet der
Server aus den Katalogfeldern **sieben Filter** ab:

| Filter | Quelle | Regel |
|---|---|---|
| **Land** | `countrycode` | direkt; Anzeige mit Landesname aus der Sprachdatei |
| **Region** | `iso_3166_2`, sonst `state` | Schweiz: Kantone über eine feste Liste von Schreibweisen («Zurich», «Zürich», «ZH», «Canton of Zurich» → Zürich); andere Länder: `iso_3166_2` oder gar keine Region |
| **Sprache** | `languagecodes`, sonst `language` | auf ISO 639-1 vereinheitlicht |
| **Genre** | `tags` | Regel-Tabelle Schlagwort → Genre (rund 25 Genres: Pop, Rock, Jazz, Klassik, Elektronisch, Hip-Hop, Schlager, Volksmusik, Alternative, Oldies, Chillout, Weltmusik, Country, Metal, Mixed …); ein Sender kann mehrere Genres haben |
| **Sendeart** | `tags`, Name | Musik, News, Talk, Sport, Kultur; Schlagworte wie `news`, `information`, `talk`, `sport`, `culture`, `public radio`; ohne Treffer **Musik** |
| **Stimmung** | `tags` | Chill, Party, Fokus, Schlafen, Workout; nur bei Treffer (`chill`, `lounge`, `ambient`, `lofi`, `study`, `sleep`, `party`, `dance`, `workout` …), sonst keine |
| **Qualität** | `bitrate`, `codec`, `hls` | hoch (ab 256 kbit/s oder verlustfrei), gut (128 bis 255), einfach (unter 128); Codec als Zusatz |

Dazu kommen **«zeigt Titel»** (der Sender hat beim Hören auf Slouchcat schon Titelangaben geschickt,
5.2 und 8) und die **Suche** über Namen, Region und Schlagworte.

**Regeln statt Einzelpflege.** Die Zuordnung Schlagwort → Filterwert steht in einer Regel-Tabelle.
Slouchcat bringt eine Vorgabe mit (eingebettete Datei `radio/rules/default.json`, rund 300
Schlagworte, gepflegt im Repository wie die Sprachdateien). Der Admin sieht auf der Seite
«Einordnung» die **nicht zugeordneten Schlagworte, nach Zahl der Sender sortiert**, und ordnet sie mit
einer Auswahl zu (Beispiel: `80s` mit 2'105 Sendern → Genre Oldies). Eine Regel wirkt sofort auf alle
Sender mit diesem Schlagwort. So deckt eine Handvoll Entscheide Tausende Sender ab. Die Seite zeigt je
Filter die **Abdeckung** («Genre zugeordnet: 91 % der Sender»).

**Vorauswahl für die App.** Jedes Profil hat sein eigenes **Standardland** (F-12). Vorgabe für
ein neues Profil ist das Land seiner Sprache und Region (für Marco die Schweiz); ändern lässt es
sich auf der Radioseite im Menü des ersten Schnellfilters («Mein Land: Schweiz» → anderes Land).
Die Radioseite öffnet mit diesem Land als erstem Schnellfilter, und jedes Profil merkt sich seinen
zuletzt gewählten Filter auf dem Server.

### 5.5 Kuratieren: nur Ausnahmen

Der Admin kann je Sender:

- **ausblenden** (erscheint nirgends in der App, bleibt im Katalog und kommt beim Abgleich nicht
  zurück),
- **Name, Bild, Genre, Sendeart, Stimmung, Region** setzen; jedes von Hand gesetzte Feld trägt ein
  **Schloss** wie im Metadaten-Editor aus Konzept 15 und wird vom Abgleich nicht überschrieben,
- den **Stream** festlegen und **«Stream prüfen»** auslösen (Codec, Bitrate, Titelangaben ja/nein,
  aktueller Titel, Antwortzeit),
- einen Sender **empfehlen**: Empfohlene stehen in ihrem Land oben (Sortierung «Beliebt»).

Gezeigt werden in der Verwaltung zuerst die **Ausnahmen** (von Hand geändert, ausgeblendet, eigene
Sender, gestört); die volle Tabelle der 40'000 Sender ist durchsuchbar, aber nicht der Einstieg.

### 5.6 Eigene Sender

Der Admin kann Sender eintragen, die im Katalog fehlen: Name, Stream-Adresse (direkt oder als
`.pls`/`.m3u`, die der Server auflöst), Webseite, Land, Region, Genre, Bild. Eigene Sender sind
wie Katalogsender filterbar. Adressen im lokalen Netz sind auch für eigene Sender **nicht**
erlaubt (F-15, 7.8).

## 6. Senderbild: immer ein Bild

Kette, erste Stufe mit Treffer gewinnt:

1. **Eigenes Bild** des Admins (Upload).
2. **Logo aus dem Katalog** (`favicon`): vom Server geladen, geprüft (echtes Bild, mindestens
   96 × 96 Pixel, höchstens 10 MiB wie `maxCoverBytes`), auf ein Quadrat gesetzt.
3. **Bild von der Webseite** des Senders: `apple-touch-icon`, Symbole aus dem Web-App-Manifest,
   `og:image`; gleiche Prüfung.
4. **Erzeugtes Bild:** Initialen des Senders (bis drei Zeichen) auf einer Farbe, die fest aus dem
   Namen abgeleitet ist (acht Farben aus den Design-Tokens). Dieses Bild entsteht im Client aus
   Tokens und braucht keinen Abruf. **Damit hat jeder Sender immer ein Bild.**

Logos lädt der Server nie im Voraus für alle 40'000 Sender. Sie werden beim ersten Anzeigen über die
Job-Queue geholt (Jobart **`radio.logo.fetch`**, niedrige Priorität, gedrosselt), bis dahin steht das
erzeugte Bild. Nach dem Abgleich holt der Job die Logos der Favoriten aller Profile und der
empfohlenen Sender vor. Abgelegt werden sie wie Posterbilder unter
`<CacheDir>/images/radio/<station>/logo-w<Breite>.webp` (Breiten 96, 256, 512) über die vorhandene
Skalierung `transcode.Runner.ScaleImage`; ausgeliefert über eine eigene Bildroute (12.3). Die App lädt
nie ein Bild direkt vom Sender: das schützt die Privatsphäre und vermeidet gemischte Inhalte
(`http`-Bilder in einer `https`-App).

## 7. Weiterleitung und Zeitversatz (ADR 0025)

### 7.1 Ablauf

1. Die App ruft `POST /radio/stations/{id}/listen` und bekommt eine **Hör-Sitzung** mit eigener
   HLS-Adresse und eigenem Token (7.5). Das vorhandene `hlsAuth`/`streamAuth` taugt dafür nicht: es
   löst auf eine `model.PlaybackGrant` mit `ItemID` und `MediaFileID` auf, und `playback_sessions`
   verlangt einen Titel (`item_id NOT NULL`).
2. Läuft für diesen Sender noch kein **Relay**, startet der Server eines: **eine** Verbindung zum
   Sender, gleichgültig wie viele Profile zuhören.
3. Ein **eigener Go-Leser** holt den Stream mit `Icy-MetaData: 1` über eine Verbindung, deren
   Dialer `netguard.RefuseLocal` prüft. `net/http` reicht nicht: Shoutcast v1 antwortet mit der
   Statuszeile `ICY 200 OK`, die `http.ReadResponse` ablehnt. Der Leser schreibt diese Zeile um,
   liest `icy-metaint` selbst, folgt Weiterleitungen selbst (höchstens 5, jede erneut geprüft),
   trennt die **Titelangaben** (`StreamTitle`, `StreamUrl`) vom Ton und gibt nur den reinen Ton an
   ffmpeg weiter.
4. **HLS-Sender** (z. B. BBC) holt ebenfalls Go: Wiedergabeliste und Stücke über den geschützten
   Client, ID3-Titel aus den Stücken; an ffmpeg geht nur der Ton. **ffmpeg sieht nie eine
   Netzadresse:** Eingabe nur `pipe:0` mit `-protocol_whitelist pipe`, Ausgabe nur in den
   Pufferordner. Damit kann ffmpeg weder ins Heimnetz greifen noch eine Adresse mit Schlüssel in
   seine Fehlermeldungen schreiben (Regel 10; `transcode` hängt stderr an Fehler an).
5. **ffmpeg** (nur im Paket `transcode`, Regel 3, `exec.CommandContext` mit Argumentliste) packt den
   Ton in **HLS mit 4-Sekunden-Stücken** (fMP4) und einer gleitenden Wiedergabeliste über die
   Pufferlänge (30 Minuten = 450 Stücke). AAC wird nur umverpackt. MP3, Ogg Vorbis, Opus und FLAC
   werden in AAC umgerechnet, und zwar mit **der Bitrate der Quelle, mindestens 96 und höchstens
   192 kbit/s**: ein 64-kbit-MP3 wird nicht künstlich dreimal so gross. CPU-Bedarf je Sender wird in
   Stufe 1 gemessen (Erwartung unter 2 % eines Kerns).
6. **Wiedergabeliste klein halten:** 450 Einträge wären bei jedem Neuladen rund 60 KB. Der Server
   liefert darum **Delta-Listen** (`EXT-X-SERVER-CONTROL:CAN-SKIP-UNTIL`, Anfrage mit `_HLS_skip`;
   hls.js und Safari können das) und komprimiert `.m3u8` mit gzip. `EXT-X-PROGRAM-DATE-TIME` steht
   am Anfang und nach jeder Unterbrechung. Damit kostet die Liste im Live-Betrieb nur wenige
   hundert Byte je Abruf; die Bandbreitenrechnung (7.3) schliesst sie ein.
7. Die App spielt den HLS-Strom mit **hls.js** (ADR 0004; auch Safari auf dem Mac) und auf dem
   **iPhone nativ**. Live heisst: nahe am Ende der Liste. Pause oder Zurückziehen heisst: weiter
   hinten in derselben Liste. **«Live»** springt ans Ende. Die Abspielzeit liest die App je Engine
   aus `hls.playingDate` beziehungsweise `video.getStartDate()` plus `currentTime`.

### 7.2 Wer zählt als Hörer

Ein pausiertes Gerät holt weder Liste noch Stücke. Daran lässt sich Zuhören also nicht messen.
Darum gilt:

- Ein **Hörer** ist eine **offene Hör-Sitzung**. Die App meldet sich alle 30 Sekunden (auch
  während der Pause) mit `POST /radio/listen/{session}/heartbeat`, beim Beenden mit `DELETE`.
- Eine Sitzung ohne Meldung bleibt so lange offen wie der Puffer lang ist (30 Minuten), denn auf
  dem iPhone schlafen Zeitgeber im Hintergrund. Danach schliesst der Server sie.
- **Nachlauf:** Erst wenn **keine** Sitzung eines Senders mehr offen ist, läuft das Relay noch
  2 Minuten weiter (einstellbar) und gibt dann Verbindung, ffmpeg und Puffer frei.
- **Pausierte Hörer zählen zur Höchstzahl** der Sender (7.3), weil ihr Puffer weiterläuft.
- **Länger pausiert als der Puffer:** Die App springt beim Fortsetzen an den ältesten noch
  vorhandenen Stand und zeigt «Die Pause war länger als 30 Minuten. Weiter ab dem ältesten Stand»
  mit Knopf «Live».

### 7.3 Puffer und Grenzen

| Grösse | Vorgabe | Einstellbar | Begründung |
|---|---|---|---|
| Zeitversatz | 30 Minuten | 10, 30, 60 Minuten | F-4 |
| Ablage der Stücke | `tmpfs` im Container (`/run/slouchcat/radio`), also Arbeitsspeicher | nein | F-4: nichts auf die Platte |
| Speicherbedarf je Sender | Bitrate × Zeitversatz: 128 kbit/s → 29 MB, 192 → 43 MB, 320 → 72 MB (30 Minuten) | folgt | – |
| **Budget** | 512 MB für alle Puffer, **in Byte durchgesetzt** | ja | Grösse der `tmpfs`; vor dem Bau gegen den RAM von CT 190 prüfen (F-13) |
| Gleichzeitige Sender | **folgt aus dem Budget**: ein neues Relay startet nur, wenn Bitrate × Zeitversatz noch ins Budget passt; zusätzlich eine Obergrenze (Vorgabe 10) | ja | 10 × 43 MB passt, 10 × 72 MB nicht |
| Nachlauf ohne Sitzung | 2 Minuten | ja | kurzer Wechsel ohne Neustart |

Passt ein weiteres Relay nicht mehr, antwortet der Server mit dem Fehlercode `radio_capacity` und die
App zeigt «Gerade laufen schon zu viele Sender über den Server» mit Vorschlag, einen laufenden Sender
zu hören (Text aus `de.json`, Regel 9). Bei 60 Minuten Zeitversatz halbiert sich die Zahl der
Sender, wenn das Budget nicht steigt; die Einstellung sagt das an.

**Bandbreite:** Jeder Hörer kostet Upload in der Höhe der ausgelieferten Bitrate (meist 96 bis
192 kbit/s) plus wenige kbit/s für Delta-Listen. Zehn Hörer unterwegs sind höchstens rund 2 Mbit/s.

### 7.4 Unterbrüche und Wiederaufbau

Live-Sender brechen ab. Ablauf:

1. Bricht die Verbindung ab, baut Go sie mit Wartezeiten von 1, 2, 5, 10 und 20 Sekunden neu auf.
   Solange Codec und Bitrate gleich bleiben, läuft **dasselbe** ffmpeg weiter; die Hörer hören eine
   kurze Lücke.
2. Ändern sich Codec oder Bitrate (oder wechselt der Server auf einen anderen Stream des Senders,
   5.3), startet ein neues ffmpeg. Die Wiedergabeliste zählt weiter, markiert den Wechsel mit
   `EXT-X-DISCONTINUITY`, bringt ein neues Init-Stück und einen neuen
   `EXT-X-PROGRAM-DATE-TIME`-Anker.
3. Nach fünf erfolglosen Versuchen endet das Relay mit «Sender nicht erreichbar». Drei solche
   Abbrüche in Folge markieren den Sender als gestört (5.2).

### 7.5 Hör-Sitzung und Token

- Neue Tabelle **`radio_listen_sessions`**: Sitzung, Konto-Sitzung, Profil, Sender, letzte Meldung,
  beendet, entzogen. Eigene Middleware `radioAuth` für Liste und Stücke.
- Das Token ist **an den Sender gebunden**: mit dem Token für Sender A gibt es keine Stücke von B.
- **Gleitender Ablauf:** jede Meldung verlängert die Sitzung; stundenlanges Hören geht ohne neues
  Token.
- **Entzug** bei Abmelden, Profilwechsel, Entzug der Konto-Sitzung und Stopp durch die Verwaltung:
  die Auslieferung an diesen Hörer endet (wie `abortWrites` beim Podcast-Proxy).
- **Kein öffentlicher Link:** ohne gültige Hör-Sitzung gibt es weder Liste noch Stück.

### 7.6 Titelangaben im Zeitversatz

Wer 12 Minuten zurückliegt, soll den Titel sehen, der **an seiner Stelle** läuft. Darum führt der
Server je Relay eine **Zeitleiste der Titelangaben** über die Pufferlänge. Gestempelt wird nicht mit
der Uhr, sondern mit der **Ton-Position**: Go zählt die Audio-Rahmen, die an ffmpeg gehen (Dauer aus
den MP3- und AAC-Rahmenköpfen), und rechnet sie ab dem `EXT-X-PROGRAM-DATE-TIME`-Anker in Zeit um.
So stimmt der Titel auch nach Stunden, nach dem Anlauf-Schwall des Senders (Icecast liefert beim
Verbinden oft 2 bis 3 Sekunden auf einmal) und nach Unterbrüchen. Abnahmetest mit dem Test-Sender:
Titelwechsel an bekannter Stelle, Abweichung höchstens ein Stück (4 s).

Die App fragt `GET /radio/stations/{id}/now?since=…` alle 10 Sekunden, solange sie sichtbar ist,
und wählt den Eintrag zu ihrer Abspielzeit. Dieselbe Zeitleiste füllt «Zuvor gespielt».

### 7.7 Startzeit

hls.js beginnt live standardmässig drei Stücke vor dem Ende, Apple empfiehlt mindestens drei
Stückdauern Abstand. Ein neues Relay hat anfangs nur ein Stück. Darum:

- Der Anlauf-Schwall des Senders füllt die ersten Stücke vor; die Antwort auf `listen` wartet, bis
  **zwei Stücke** bereitliegen.
- hls.js startet mit `liveSyncDurationCount: 2`; das iPhone bekommt `EXT-X-START:TIME-OFFSET=-8`.
- **Ziele:** erster Ton höchstens **6 s** bei einem neuen Relay, höchstens **2 s** bei einem
  laufenden; gemessen je Engine (hls.js in Chromium und Firefox, iPhone nativ) in
  `docs/perf/radio.md`. Werden die Ziele verfehlt, sind 2-s-Stücke für die ersten 30 Sekunden der
  Rückfall (Wechsel mit `EXT-X-DISCONTINUITY`).

### 7.8 Sicherheit

- **Jeder Abruf über `netguard`:** Stream-Adressen, Weiterleitungen, Adressen aus `.pls`/`.m3u`,
  HLS-Listen und -Stücke, `StreamUrl`, `favicon`, Webseite, Manifest-Symbole, `og:image`, Cover Art
  Archive (leitet zu archive.org weiter) und iTunes. Ziele im lokalen Netz werden nach der
  DNS-Auflösung abgewiesen (`RefuseLocal`), ohne Ausnahme, auch nicht für eigene Sender (F-15).
- **ffmpeg sieht nie eine Netzadresse** (7.1 Punkt 4).
- **Zulassungsliste der Inhalte:** `audio/mpeg`, `audio/aac`, `audio/aacp`, `audio/mp4`,
  `audio/ogg`, `application/ogg`, `audio/flac`, `audio/opus`; Listen `audio/x-scpls`,
  `audio/x-mpegurl`, `audio/mpegurl`, `application/vnd.apple.mpegurl`, `application/x-mpegurl`.
  Alles andere bricht mit «kein Audio» ab; ein fehlender oder falscher Typ wird am Inhalt erkannt
  (Rahmenköpfe), nicht geraten. Ogg-Sender schicken oft kein `icy-metaint`; ihre Titel stehen in
  den Vorbis- und Opus-Kommentaren.
- **Bilder:** Favicons sind oft ICO oder SVG. Der Typ wird vor `ScaleImage` geprüft; SVG wird nicht
  verarbeitet (nächste Stufe der Kette, 6).
- **Logs:** Adressen nur gekürzt auf Schema und Host (`netguard.Clean`, Regel 10).

### 7.9 Standort und Rechte

Weil der Sender den Slouchcat-Server sieht und nicht das Handy, gilt der Standort des Servers:
Marcos Server steht in der Schweiz, also hören auch seine Profile im Ausland so, als wären sie zu
Hause. Manche Anbieter verbieten in ihren Nutzungsbedingungen, eine Sperre so zu umgehen, oder sperren
nur einzelne Sendungen (Sportrechte). Slouchcat gibt den Ton nur an angemeldete Profile der eigenen
Installation weiter, nie öffentlich; ob das für einen Sender zulässig ist, verantwortet der
Betreiber der Installation. Die Hilfe der Verwaltungsseite sagt das in zwei Sätzen.

### 7.10 Tägliche Hörzeit (K-5, F-16)

Radio zählt zum täglichen Limit des Profils (`profiles.watch_limit_minutes`, K-5), wie Filme,
Hörbücher und Podcasts:

- Die Meldung der App (7.2) sagt, ob gerade gespielt wird oder Pause ist. Der Server rechnet nur
  gespielte Zeit an, je Viertelstunde in der neuen Tabelle `radio_listen_time` (Profil,
  Viertelstunde, gespielte Millisekunden).
- `playback.Service.WatchTime` und `WatchReport` zählen diese Zeit zu der aus den
  Wiedergabesitzungen dazu; der Bericht des Hauptprofils zeigt sie mit.
- Ist das Limit erreicht, beendet `checkWatchLimit` (über `endProfile`) auch die Hör-Sitzungen des
  Profils; die App zeigt denselben Hinweis wie bei Filmen (`watch_limit_reached`). Ein neues
  `listen` wird mit diesem Fehler abgewiesen.
- Die Verwaltungsseite «Statistik» («Wann geschaut wird», Top 20) bleibt bei Titeln; Radio zählt
  dort nicht mit.

## 8. Titelangaben und Cover

**Was ein Stream liefert.** Icecast- und Shoutcast-Sender schicken im Kopf `icy-name`, `icy-genre`,
`icy-url` und `icy-br` und im Strom alle paar Kilobyte `StreamTitle='Künstler - Titel';`, manchmal
`StreamUrl` mit einem Bild. Der Browser kann diese Blöcke meist nicht lesen (CORS gibt `Icy-MetaInt`
nicht frei); darum liest der Server sie (7.1).

**Aufbereitung:**

1. Rohtext säubern: Leerzeichen; Zeichensatz zuerst als UTF-8 lesen, nur wenn das ungültig ist als
   Windows-1252 (bei Ogg sind die Kommentare immer UTF-8); Zusätze wie «(Radio Edit)» für die
   Suche entfernen, für die Anzeige behalten.
2. Aufteilen in Künstler und Titel am ersten ` - ` oder ` – `.
3. **Kein Titel**, wenn der Text dem Sendernamen, einer Sendung oder Werbung gleicht (Liste typischer
   Muster und Vergleich mit `icy-name`): dann zeigt die App Sender und «Live».
4. **Cover-Suche (F-6):** zuerst **MusicBrainz** (Aufnahme suchen, Veröffentlichung wählen, Bild aus
   dem **Cover Art Archive**, 500 px), sonst **iTunes-Suche** (ohne Schlüssel). Beide über
   `ratelimit`: MusicBrainz verlangt höchstens eine Anfrage je Sekunde und einen User-Agent mit
   Kontaktangabe (`Slouchcat/<Version> (+https://github.com/SlouchCat/slouchcat)`), iTunes erlaubt
   rund 20 Anfragen je Minute. Gesucht wird nur bei einem Titelwechsel eines laufenden Relays, nie
   für den ganzen Katalog. Treffer werden 30 Tage, Fehlschläge 24 Stunden im Cache gehalten (Tabelle
   `radio_tracks`, Schlüssel «künstler|titel» normalisiert), Bilder wie Logos abgelegt.
5. Liefert der Sender selbst ein Bild (`StreamUrl` mit Bild-Adresse), gewinnt es vor der Suche.
6. Ohne Cover zeigt die App das Senderbild.

**Darstellung:** Mini-Player mit Cover, Künstler und Titel; «Jetzt läuft» mit grossem Cover, dem
Cover als unscharfem Hintergrund, Künstler, Titel, Sender und «Zuvor gespielt» (7.6). Ein Wechsel des
Titels blendet weich über, ohne Bewegung bei `prefers-reduced-motion`.

## 9. Datenmenge und Leistung

| Grösse | Wert | Ziel |
|---|---|---|
| Sender im Katalog | rund 51'000 Einträge, nach Zusammenführen rund 35'000 bis 40'000 | – |
| Datenbank | rund 40 MB für Sender, Streams, Schlagworte | gemessen in Stufe 1 |
| Liste mit Filtern und Zählern | eine Seite mit 60 Sendern und den Zählern aller Filter | p95 unter 200 ms auf CT 190 |
| Suche | Namen, Region, Schlagworte | p95 unter 150 ms auf CT 190 |
| Abgleich | ganze Liste | unter 2 Minuten auf CT 190, ohne die Wiedergabe zu stören (Zeitfenster der Job-Queue) |

Ob die Suche mit SQLite-FTS5 oder einem normalisierten Namensfeld läuft, entscheidet die Messung in
Stufe 1.

## 10. Die App

### 10.1 Seite «Radio»

Neuer Eintrag **«Radio»** in der Seitenleiste (`app/AppLayout.tsx`, Liste `nav`), Route `/radio`. Die
Seite hat:

- **Suche** über Sendernamen, Region und Schlagworte.
- **Schnellfilter:** Standardland des Profils (für Marco «Schweiz», 5.4), «Meine Sender», «Beliebt».
- **Filter:** Land, Region (erscheint, sobald ein Land gewählt ist), Sprache, Genre, Sendeart,
  Stimmung, Qualität, Schalter «nur mit Titelanzeige». Mehrfachwahl je Filter, Zähler je Wert, aktive
  Filter als entfernbare Chips, «Alle Filter zurücksetzen».
- **Sortierung:** Beliebt (Empfohlene zuerst, dann Stimmen und Klicks aus dem Katalog), Name A–Z,
  Qualität, Neu im Katalog.
- Leerer Zustand mit Vorschlag, einen Filter zu lösen.
- Filter und Sortierung stehen in der Adresse (`?country=CH&genre=pop`), damit Zurück und Teilen
  funktionieren; der zuletzt gewählte Filter wird je Profil gespeichert (5.4).

### 10.2 Favoriten und Startseite

- **Favoriten je Profil** (Herz am Sender, im Mini-Player und in «Jetzt läuft»), Reihenfolge per
  Platzwahl wie in der Verwaltung (S-16).
- **Startseite:** neue Reihe **«Meine Sender»** (Schlüssel `radio`) mit den Favoriten als quadratische
  Kacheln und dem letzten bekannten Titel. Die Reihe zählt zu den höchstens 15 Reihen (Konzept 15,
  F-12) und ist in «Startseite» der Verwaltung wie jede andere Reihe verschieb- und ausblendbar. Ohne
  Favoriten erscheint sie nicht. Ab Werk eingeblendet oder ausgeblendet, und an welchem Platz, ist
  Frage F-17 (Konzept 15 blendet neue Reihen ab Werk aus).
- **Vertrag:** ein neuer Wert in `HomeRowKey` ist **brechend** für `getHome` und `setHomeRows`
  (Konzept 15, 8.3: `homeRowTitleKey` in `catalog/labels.ts` ist ein geschlossenes `Record`). Die
  Reihe braucht zudem ein neues Feld `HomeRow.stations` (Senderkacheln statt `ItemRef`). Steht die
  Reihe an erster Stelle, malt die Vorabmalerei (`startPagePreload`, P-10) die Senderkacheln wie eine
  Posterreihe vor.

### 10.3 Mini-Player und «Jetzt läuft»

Heute gibt es keinen Player, der beim Navigieren weiterläuft (Abschnitt 3). Neu:

- Ein **Radio-Player auf App-Ebene** hält Sender, Zustand und Abspielstelle, mit eigenem
  `<audio>`-Element und hls.js. Er sitzt als React-Kontext in **`activeRoute`**, also **über**
  `chromeRoute` (`app/router.tsx`). So stirbt er nicht zufällig beim Wechsel nach `/play/$itemId`
  oder `/profiles`, sondern hält nur nach den Regeln der Tabelle unten an.
- Der **Mini-Player** steht unten in `AppLayout` über dem Seiteninhalt (auf dem Handy über der
  unteren Leiste): Senderbild oder Cover, Künstler und Titel (oder «Live»), Abspielen/Pause, Herz,
  Zustand «LIVE» oder «−12:30».
- Ein Klick öffnet **«Jetzt läuft»** als Vollansicht: grosses Cover, Künstler, Titel, Sender,
  Zeitversatz-Regler von −30 Minuten bis Live mit Knopf «Live», Pause, «Zuvor gespielt», Herz und der
  Hinweis «Über Slouchcat, Standort des Servers».
- **Vorrang**, damit nie zwei Töne gleichzeitig laufen:

| Ereignis | Radio |
|---|---|
| Film, Folge, Hörbuch oder Podcast startet (`/play/$itemId`, auch offline) | stoppt, Hör-Sitzung endet |
| Trailer startet (`ui/TrailerDialog.tsx`, Vorschau in `admin/item/TrailersTab.tsx`) | pausiert, läuft nach dem Schliessen weiter (im Zeitversatz) |
| Profilwechsel, Abmelden | stoppt, Sitzung wird entzogen (7.5) |
| Navigation innerhalb der App, auch in die Verwaltung | läuft weiter |
| App oder Tab im Hintergrund | läuft weiter (Handy: so lange das System es zulässt) |
| Tägliche Hörzeit des Profils erreicht (K-5, 7.10) | stoppt mit Hinweis, wie bei Filmen |
| Zweiter Tab startet Radio | erster Tab pausiert (Absprache über `BroadcastChannel`) |

  Hörbücher und Podcasts laufen heute vollflächig in `/play/$itemId` ausserhalb von `AppLayout`;
  dort gibt es keinen Mini-Player. Der umgekehrte Fall «Radio startet, während ein Hörbuch läuft»
  kommt darum nicht vor.
- **Tastatur:** Mini-Player und «Jetzt läuft» sind mit Tab erreichbar, Leertaste pausiert, Esc
  schliesst «Jetzt läuft» (Regel 8: Playwright-Tastaturtests für `/radio`, «Jetzt läuft» und
  `/admin/radio`).
- **Kein Bedienfeld auf dem Sperrbildschirm** (F-11, Media Session nicht gebaut). Das Handy zeigt
  dort, was der Browser von sich aus zeigt. Ob das iPhone den Ton bei gesperrtem Bildschirm hält,
  wird bei der Abnahme geprüft (Risiko in 16).

### 10.4 Drei Varianten (Mockup, Anhang A.1)

| Variante | Aufbau | Stärke | Schwäche |
|---|---|---|---|
| **A «Katalog wie TuneIn»** | Suche oben, eine Zeile Filterknöpfe mit Auswahlfeldern, Schnellfilter; darunter Reihen «Meine Sender», «Beliebt in der Schweiz», Kacheln «Nach Genre» und «Nach Stimmung», dann das Raster der Treffer | vertraut, gut am Handy, Stöbern | viele Filter gleichzeitig sind weniger übersichtlich |
| **B «Filterleiste links»** | Filterspalte mit Kästchen und Zählern, rechts eine dichte Liste mit aktuellem Titel je Sender, Sortierung und Trefferzahl oben; am Handy als Blatt «Filter (3)» | schnellstes gezieltes Filtern, viele Sender auf einen Blick | wirkt eher wie ein Werkzeug als wie ein Radio |
| **C «Jetzt läuft zuerst»** | grosse Bühne mit dem laufenden Sender, darunter «Meine Sender» als grosse Kacheln mit letztem Titel, dann «Entdecken» mit Länder-, Stimmungs- und Genre-Kacheln, die in ein gefiltertes Raster führen | am schönsten für Favoriten-Hörer | Filtern braucht einen Schritt mehr |

**Gewählt: A «Katalog wie TuneIn»** (Marco, 6. November 2026, F-9). Gebaut wird A so, wie das Mockup
sie zeigt: Zähler je Filterwert in den Auswahlfeldern, Trefferzahl über dem Raster, am Handy die
Filter als Blatt von unten. Die Reihen zum Stöbern («Meine Sender», «Beliebt in der Schweiz», «Nach
Genre», «Nach Stimmung») stehen nur ohne aktiven Filter; mit Filter zeigt die Seite nur das Raster.

## 11. Die Verwaltung

Neue Seite **«Radio»** unter `/admin/radio` in der Gruppe `library` der Verwaltungsleiste
(`features/admin/sections.ts`), nach dem Seitenmuster S-1 bis S-17 aus Konzept 12 (Seitenkopf mit
Hilfe, Zustandsleiste zuerst, dichte Tabellen, höchstens zwei Zeilenaktionen, Einstellungen als
Zeilen, Auswahl in der Adresse, kompakte Dichte).

Inhalt in allen Varianten:

- **Katalog:** Quelle, letzter Abgleich («heute 04:30, 51'240 Einträge, 38'912 erreichbar, 214 neu,
  88 entfernt»), nächster Lauf, «Jetzt abgleichen», Link zur Jobart.
- **Einordnung:** Regeln Schlagwort → Filterwert, nicht zugeordnete Schlagworte nach Senderzahl,
  Abdeckung je Filter (5.4).
- **Sender:** Ausnahmen zuerst, volle Tabelle durchsuchbar; Bearbeiten mit Schlössern, Bildwahl
  (6), Streams und «Stream prüfen», Empfehlen, Ausblenden (5.5); eigene Sender (5.6).
- **Live:** laufende Relays als Kacheln wie auf «Streams» (Sender, Hörer mit Profilnamen, Bitrate,
  Laufzeit, Pufferstand, Zustand der Verbindung) und die Leiste **«Serverlast»** (Sender 3 von 10,
  Upload, Puffer 112 von 512 MB).
- **Einstellungen:** Zeitversatz, Höchstzahl Sender, Nachlauf ohne Hörer, Cover-Suche
  (MusicBrainz und iTunes je ein Schalter), gestörte Sender ausblenden, Länder im Katalog (alle oder
  gewählte).

Zusätzlich: Radio-Hörer erscheinen auf der Verwaltungsseite **«Streams»** als eigene Art «Radio»
(ohne Transcoding-Anteil). Das braucht eine Vertragsänderung: `listStreams` verlangt heute `itemId`,
`mediaFileId` und `mode`, und `stopStream` beendet eine `playback_session`. Radio bekommt darum eine
eigene Liste `radioListeners` in der Antwort und einen eigenen Stopp
(`DELETE /admin/radio/listeners/{session}`). Die Jobarten `radio.catalog.sync` und `radio.logo.fetch` stehen mit
Beschreibung auf «Hintergrundjobs» (Konzept 15, P-2).

### 11.1 Drei Varianten (Mockup, Anhang A.2)

| Variante | Aufbau | Stärke | Schwäche |
|---|---|---|---|
| **A «Reiter»** | Zustandsleiste, Reiter «Übersicht» (Katalog, Live, Serverlast), «Sender», «Einordnung», «Eigene Sender», «Einstellungen»; Bearbeiten im Seitenblatt | folgt dem Muster der übrigen Seiten (Hintergrundjobs C) | Einordnung ist ein Reiter unter vielen |
| **B «Liste und Detail»** | links durchsuchbare Senderliste mit Filtern, rechts der gewählte Sender mit Bild, Einordnung, Streams, Sichtbarkeit; Katalog und Einstellungen über zwei Knöpfe im Kopf | schnell für Einzelpflege | verleitet zur Pflege Sender für Sender, die Marco nicht will |
| **C «Zustand und Regeln»** | Kacheln oben (Katalog, Erreichbar, Live, Serverlast), dann «Automatische Einordnung» als Herzstück mit Abdeckung und Arbeitsliste, darunter nur die Ausnahmen, Einstellungen als Zeilen | setzt Marcos Vorgabe «nicht 50k Sender kategorisieren» am direktesten um | lange Seite; volle Sendertabelle nur über Suche |

**Gewählt: A «Reiter»** (Marco, 6. November 2026, F-10; Empfehlung war C). Reiter mit `?tab=`
(S-14): **Übersicht** (Katalog, Abdeckung, Live-Weiterleitungen, Serverlast), **Sender**, **Einordnung**,
**Eigene Sender**, **Einstellungen**; Bearbeiten eines Senders im Seitenblatt. Damit Marcos Vorgabe
«nicht 50k Sender kategorisieren» auch in A trägt:

- Der Reiter **Sender** öffnet mit dem Filter **«Nur Ausnahmen»** (von Hand geändert, ausgeblendet,
  eigene, gestört); die Suche geht immer über alle Sender (5.5).
- Die **Übersicht** zeigt die Abdeckung je Filter und die Zahl der nicht zugeordneten Schlagworte
  mit Link in den Reiter **Einordnung**.
- Die Zustandsleiste nennt «n Schlagworte ohne Zuordnung», solange welche mit mehr als 50 Sendern
  offen sind.

## 12. Server, Vertrag und Datenbank

### 12.1 Pakete

- Neues Paket **`server/internal/radio`**: Katalog, Abgleich, Einordnung, Relays, Zeitleiste,
  Cover-Kette, Favoriten. Es fragt keine Titeltabellen ab (Regel 2 bleibt unberührt).
- **`transcode`** bekommt den ffmpeg-Aufruf für das HLS-Packen (Regel 3).
- **`netguard`** für alle Abrufe (7.8), **`ratelimit`** für Radio Browser, MusicBrainz und iTunes.
- Jobarten in `radio`, registriert wie `trakt.refresh`; Texte in `admin.jobs.types.radio_*`.

### 12.2 Datenbank (nach Vorlage, nur vorwärts)

**Migration 00036 (Stufe 1):**

| Tabelle | Inhalt |
|---|---|
| `radio_stations` | Sender: eigene Kennung, Quelle (`catalog`/`own`), Name, Land, Region, Sprachen, Webseite, Beliebtheit, Zustand, ausgeblendet, empfohlen, «zeigt Titel», «nicht mehr im Katalog seit», Zeitstempel |
| `radio_station_aliases` | Katalogeinträge je Sender (`stationuuid` → Sender), für die stabile Kennung (5.3) |
| `radio_streams` | Streams je Sender: Adresse, Codec, Bitrate, HLS, erreichbar, bevorzugt |
| `radio_station_tags` | Schlagworte je Sender (roh, für die Regeln) |
| `radio_station_facets` | abgeleitete Filterwerte je Sender (Genre, Sendeart, Stimmung), mit Merkmal «von Hand» |
| `radio_station_locks` | gesperrte Felder je Sender (Schloss, 5.5) |
| `radio_favorites` | `profile_id`, `station_id`, `position`; `ON DELETE CASCADE` auf Profil **und** Sender (F-18) |
| `radio_profile_state` | `profile_id`, Standardland (F-12), zuletzt gewählter Filter und Sortierung (JSON), `ON DELETE CASCADE` |
| `radio_listen_sessions` | Hör-Sitzungen (7.5) |
| `radio_listen_time` | gespielte Zeit je Profil und Viertelstunde für das Tageslimit (7.10) |
| `radio_config` | eine Zeile (`id = 1`) mit den Einstellungen, wie `spotlight_config` |

**Migration 00037 (Stufe 2):** `radio_tag_rules` (Regeln des Admins, überschreiben die
Vorgabe-Datei), `radio_tracks` (Cache der Titelsuche) und der neue Schlüssel `radio` im `CHECK` von
`home_rows`. Ein neuer `CHECK`-Wert heisst in SQLite: Tabelle nach Vorlage neu aufbauen
(`server/migrations/TEMPLATE.sql.txt`).

### 12.3 Vertrag

**Stufe 1, vorläufig 1.17.0 (additiv):**

| Operation | `x-visibility` | Zweck |
|---|---|---|
| `GET /radio/stations` | `scoped` | Liste mit Filtern, Sortierung, Suche, Seiten |
| `GET /radio/facets` | `scoped` | Zähler je Filterwert zum aktuellen Filter |
| `GET /radio/stations/{id}` | `scoped` | ein Sender mit Streams und Zustand |
| `GET`/`PUT /radio/favorites` | `scoped` | Favoriten des Profils lesen und ordnen |
| `PUT`/`DELETE /radio/favorites/{id}` | `scoped` | Favorit setzen oder lösen |
| `POST /radio/stations/{id}/listen` | `scoped` | Relay starten oder beitreten, Hör-Sitzung mit HLS-Adresse |
| `POST /radio/listen/{session}/heartbeat`, `DELETE /radio/listen/{session}` | `scoped` | Sitzung offen halten, beenden (7.2) |
| `GET /radio/stations/{id}/now` | `scoped` | Zeitleiste der Titelangaben |
| `GET`/`PUT /radio/preferences` | `scoped` | Standardland, letzter Filter und Sortierung des Profils (5.4) |
| `GET /radio/live/{session}/…` (HLS) | `scoped`, `http.Handler` (Regel 6), Middleware `radioAuth` | Wiedergabeliste und Stücke |
| `GET /radio/images/{id}/{kind}/{size}` | `scoped`, `http.Handler` | Senderlogo und Cover |
| `GET`/`PATCH /admin/radio/config` | `admin` | Einstellungen |
| `GET /admin/radio/catalog`, `POST /admin/radio/catalog/sync` | `admin` | Zustand, Abgleich starten |
| `GET`/`PATCH`/`POST`/`DELETE /admin/radio/stations…` | `admin` | Ausnahmen, Bearbeiten, eigene Sender |
| `POST /admin/radio/stations/{id}/probe` | `admin` | Stream prüfen |
| `PUT /admin/radio/stations/{id}/logo` | `admin` | eigenes Bild (Upload) |
| `GET /admin/radio/relays`, `DELETE /admin/radio/listeners/{session}` | `admin` | laufende Relays, Serverlast, Hörer stoppen |

**Stufe 2, vorläufig 1.18.0 (brechend für `getHome` und `setHomeRows`):** `HomeRowKey` `radio`,
`HomeRow.stations`, `GET`/`PUT /admin/radio/tag-rules`, `radioListeners` in der Antwort von
`listStreams`.

- `scoped` heisst hier: Sitzung mit gewähltem Profil (für Favoriten nötig). Antworten enthalten kein
  `ItemRef`.
- Ein **ausgeblendeter** Sender antwortet auf `GET /radio/stations/{id}`, `listen`, `now` und die
  Bildroute mit **404**, wie nicht sichtbare Titel.
- Jede Operation bekommt ihren Eintrag im Vertragstest (`TestContractCoversEveryOperation`, Regel 7),
  jede `scoped`-Operation einen Eintrag in `profileRegistry` (`contract_profiles_test.go`); der
  Radiodienst kommt in die vistest-Fixture. Die binären Routen kommen in `binaryRoutes`
  (`api/api.go`). `compressText` hängt nur an den Strict-Routen; die HLS-Route komprimiert `.m3u8`
  selbst (7.1 Punkt 6).
- Fehlercodes mit Texten in `de.json`/`en.json` (Regel 9): `radio_capacity`,
  `radio_unreachable`, `radio_not_audio`, `radio_catalog_loading`.

### 12.4 Container

`deploy/compose.yaml` bekommt eine `tmpfs` für die Puffer (`/run/slouchcat/radio`, Grösse = Budget
plus 10 % Reserve). Fehlt sie (ältere Compose-Datei), spielt Radio **nur live ohne Zeitversatz** und
die Radio-Seite der Verwaltung warnt; der Server legt den Puffer nie auf die Platte (F-4). Die
Release-Notes der Stufe 1 nennen die Änderung an der Compose-Datei.

### 12.5 Tests ohne echte Sender

- **Test-Sender** im Testbuild: ein kleiner Server, der Icecast und Shoutcast v1 nachahmt (Status
  `ICY 200 OK`, `icy-metaint`, `StreamTitle` an bekannten Stellen, Abbruch auf Befehl), dazu eine
  HLS-Quelle. Die Töne erzeugt ffmpeg mit `lavfi` (MP3, AAC, Ogg); im Repository liegen nur kurze
  erzeugte Dateien, keine Musik.
- Der Test-Sender läuft auf localhost, den `netguard` abweist. Darum gibt es **nur im Testbuild**
  (Build-Tag, nie über eine Umgebungsvariable in der Produktion) einen Dialer, der localhost
  erlaubt, und eine umstellbare Katalog-Adresse für den eingefrorenen Auszug unter
  `testdata/radio/`.
- e2e prüfen Zustand und Titelanzeige, nicht den Klang: die Firefox-Runner der CI haben keine
  Audioausgabe (PR #18). Abspiel-Tests laufen wie dort im Playwright-Container.
- Abnahmetest der Zeitleiste (7.6) und des Wiederaufbaus (7.4) in Go gegen den Test-Sender.

## 13. Fragen an Marco und seine Antworten (6. November 2026)

| Nr. | Frage | Antwort |
|---|---|---|
| F-9 | Welche Variante für die App (A, B, C oder eine Mischung)? | **Beantwortet: A** (6. November 2026) |
| F-10 | Welche Variante für die Verwaltung? | **Beantwortet: A** (6. November 2026; Empfehlung war C) |
| F-11 | Sperrbildschirm doch? Sender, Titel, Cover und Pause auf dem Handy-Sperrbildschirm und in der Mitteilungszentrale (Media Session). Ohne das zeigt das Handy beim gesperrten Bildschirm nur «Slouchcat», und das iPhone hält Ton im Hintergrund weniger zuverlässig. | **Nein** (Empfehlung war Ja). Kein Bedienfeld auf dem Sperrbildschirm (10.3) |
| F-12 | Standardland: eines für die ganze Installation (Admin) oder je Profil? | **Je Profil** (Empfehlung war Installation). Standardland in `radio_profile_state`, wählbar auf der Radioseite (5.4) |
| F-13 | Grenzen: 512 MB Puffer (in Byte durchgesetzt), höchstens 10 Sender, 2 Minuten Nachlauf. Der Puffer belegt Arbeitsspeicher; vor dem Bau wird der RAM von CT 190 nachgesehen. | **Ja, wie vorgeschlagen**, mit Prüfung gegen den RAM von CT 190 vor dem Bau |
| F-14 | Klicks an Radio Browser melden (verbessert deren Beliebtheitswerte, verrät aber, was gehört wird)? | **Nein**, wie vorgeschlagen |
| F-15 | Sollen eigene Sender im lokalen Netz möglich sein (z. B. ein eigener Icecast im Heimnetz)? | **Nein** (Empfehlung war Ja mit Haken). Keine Adressen im lokalen Netz, auch nicht für eigene Sender (5.6, 7.8) |
| F-16 | Gäste und Grenzen: Zählt Radio zum Schauzeit-Limit (K-5) und zu den gleichzeitigen Streams und der Bandbreite je Profil und Gast (W-8)? Bekommen Gastkonten das Relay mit Schweizer Standort? | **«Gehört zur täglichen Hörzeit»** (eigene Antwort): Radio zählt zum Tageslimit des Profils (K-5, 7.10). Gäste hören wie alle Profile (F-5). Ob Radio auch zu den Stream-Grenzen W-8 zählt, ist F-19 |
| F-17 | Reihe «Meine Sender» auf der Startseite: ab Werk eingeblendet oder ausgeblendet, an welchem Platz? | **Ausgeblendet, am Ende**, wie vorgeschlagen |
| F-18 | Verschwindet ein Favorit aus dem Katalog, bleibt er stehen, solange sein Stream antwortet (5.2). Einverstanden? | **Nein, entfernen** (Empfehlung war Ja). Mit dem Katalogeintrag verschwindet nach 7 Tagen auch der Favorit (5.2) |
| F-19 | Zählt Radio auch zu den Grenzen W-8 je Profil und Gast (höchstens n gleichzeitige Streams, höchstens n kbit/s)? | **Nein** (Empfehlung war Ja). Eine Hör-Sitzung belegt keinen der erlaubten Streams, und die Bitraten-Grenze gilt für Radio nicht; geschützt wird der Server durch Budget und Höchstzahl der Sender (7.3) |

## 14. Bauplan in Stufen

| Stufe | Inhalt | Release |
|---|---|---|
| **1 «Hören»** | Katalog und täglicher Abgleich, Zusammenführen mit stabiler Kennung, Einordnung mit Vorgabe-Regeln, Radioseite mit Filtern, Suche und Sortierung, Favoriten, Relay mit Hör-Sitzungen **live** (Puffer läuft schon, Zeitversatz-Regler noch nicht), Hörzeit im Tageslimit (7.10), Unterbrüche, Titelangaben aus dem Stream, Senderbild-Kette, Mini-Player und «Jetzt läuft», Verwaltungsseite mit Katalog, Live, Ausnahmen und Einstellungen, Test-Sender, ADR 0025, Messungen 9 und 7.7 | vorläufig 1.17.0 (additiv), Migration 00036 |
| **2 «Zurück und Cover»** | Zeitversatz bis 30 Minuten mit Regler, Zeitleiste nach Ton-Position, Cover-Kette MusicBrainz → iTunes, «Zuvor gespielt», Startseiten-Reihe «Meine Sender», Regeln des Admins mit Arbeitsliste und Abdeckung, eigene Sender, Radio auf «Streams» | vorläufig 1.18.0 (brechend für `getHome`/`setHomeRows`), Migration 00037 |

Je Stufe ein Zweig, ein PR, Abnahme auf CT 190 und ein Release. Prüfung in den drei Teststufen
(`docs/dev/SETUP.md`), Tests nach 12.5.

## 15. Was nicht dazugehört

- Musikbibliothek, eigene Musikdateien, Wiedergabelisten aus Titeln (Nicht-Ziel bleibt).
- Aufnahmen, Zeitversatz über die Pufferlänge hinaus, Speichern auf der Platte (kein DVR).
- Podcasts aus dem Radio (Podcasts gibt es schon als Bibliothek).
- Offline-Radio, Weckfunktion, Schlaf-Timer (heute nur beim Hörbuch; später möglich).
- Algorithmische Senderempfehlungen (Nicht-Ziel); «Beliebt» nutzt nur Katalogzahlen und die
  Empfehlungen des Admins.
- Altersgrenzen für Radio (F-5).
- «Zuletzt gehört» (F-8 nicht gewählt).
- Bedienfeld auf dem Sperrbildschirm, Media Session (F-11).
- Sender im lokalen Netz, auch als eigene Sender (F-15).
- Steuerung von Sendern, Abstimmen bei Radio Browser, Rückmeldungen an den Katalog (F-14).

## 16. Risiken

| Risiko | Folge | Gegenmittel |
|---|---|---|
| Katalogqualität: falsche Länder, tote Streams, wirre Schlagworte | falsche Treffer | Zusammenführen, gestörte ausblenden, Regeln, Ausnahmen mit Schloss |
| Radio Browser fällt aus | kein Abgleich | letzter Stand bleibt nutzbar; Abgleich versucht es wie `update.check` nach 15 min, 1 h, 6 h |
| Sender sperrt auch den Server oder verlangt Anmeldung | Sender läuft nicht | Meldung «Sender nicht erreichbar» mit Grund; gestört nach drei Abbrüchen (7.4) |
| Speicher: viele Sender, hohe Bitraten | Server knapp | Budget in Byte (7.3) |
| Titelangaben fehlen bei vielen Sendern | wenig Cover | Senderbild als Rückfall; Filter «nur mit Titelanzeige» |
| Umrechnen nach AAC kostet etwas Klang | leicht schlechter bei MP3-Sendern | Bitrate der Quelle, höchstens 192 kbit/s; Messung in Stufe 1 |
| iPhone hält Ton im Hintergrund nicht | Radio stoppt beim Sperren | Prüfung am iPhone bei der Abnahme; Media Session wäre das Gegenmittel, ist nach F-11 aber nicht gebaut und bräuchte Marcos Wort |
| Favorit verschwindet überraschend | Profil sucht seinen Sender | 7 Tage Hinweis «Nicht mehr im Katalog, verschwindet am …» (5.2) |
| Startzeit über dem Ziel | träger Start | Vorfüllen, kürzere Stücke am Anfang (7.7) |

## 17. Kritiker-Review von Fassung 1 (6. November 2026)

Ergebnis **REVISE**: 2 Blocker, 12 Major, 11 Minor. Alle eingearbeitet:

| Befund | Kern | Wo |
|---|---|---|
| B-1 | HLS-Quellen über ffmpeg umgehen `netguard`, Schlüssel in stderr | 7.1 Punkt 4, 7.8 |
| B-2 | Pause über 2 Minuten verlor den Puffer (gegen F-4) | 7.2 |
| M-1 | `hlsAuth` braucht einen Titel; eigene Hör-Sitzung | 7.1, 7.5, 12.2 |
| M-2 | Wiedergabeliste mit 900 Einträgen kostet mehr als der Ton | 7.1 Punkt 5 und 6, 7.3 |
| M-3 | Titelangaben und Zeitstempel auf verschiedenen Uhren | 7.6 |
| M-4 | Abbruch und Wiederaufbau fehlten | 7.4 |
| M-5 | Speicherrechnung nur für 192 kbit/s | 7.3 |
| M-6 | Shoutcast v1 (`ICY 200 OK`) mit `net/http` nicht lesbar | 7.1 Punkt 3 |
| M-7 | Startzeit nicht abgesichert | 7.7 |
| M-8 | Reihe `radio` ist brechend und braucht eine eigene Migration | 10.2, 12.2, 12.3, 14, F-17 |
| M-9 | Senderkennung und Favoriten nicht stabil, Kaskade widersprüchlich | 5.2, 5.3, 12.2, F-18 |
| M-10 | Test-Sender scheitert an `netguard` | 12.5 |
| M-11 | Ort des Players und Vorrang der Töne unklar | 10.3 |
| M-12 | Mockups fehlten | Anhang A |
| Minor 1–11 | Safari auf dem Mac mit hls.js, Bitrate der Quelle, Zulassungsliste, alle Abrufe über `netguard`, Zeichensatz, iTunes-Grenze und MusicBrainz-Kontakt, Zeitplan als Intervall mit Fenster und Seed, kein Plattenpuffer ohne `tmpfs`, 404 und `profileRegistry`, Streams-Vertrag, Tastaturtest Verwaltung und Fehlercodes | 7.1, 7.8, 8, 5.2, 12.3, 12.4, 11, 10.3 |
| Fehlende Fragen | Gäste und Grenzen, Reihe ab Werk, RAM, Favoriten aus dem Katalog | F-13, F-16, F-17, F-18 |

## Anhang A. Mockups

- **A.1 App:** `docs/planning/mockups/16-webradio-app-prototyp.html` (Varianten A, B, C; Desktop und
  Handy; Schalter «Ansicht: Radio / Startseite»; zwei Beispielprofile mit eigenen «Meine Sender»).
  Privates Artefakt: https://claude.ai/artifact/42tpSaef2Mi8hqWYFjSUfQ
- **A.2 Verwaltung:** `docs/planning/mockups/16-webradio-verwaltung-prototyp.html` (Varianten A, B, C;
  Adresszeile mit `?tab=` und `?item=`). Privates Artefakt:
  https://claude.ai/artifact/5xMtdGgsRTeHQdYuNpakGV

Wo Mockup und Konzept abweichen, gilt das Konzept: Die Verwaltung zeigt im Mockup noch die
Einstellung «Standardland» (nach F-12 je Profil auf der Radioseite) und die Bildwahl kennt keinen
Haken für Adressen im lokalen Netz (F-15).

Was die Mockups vereinfachen: Titel wechseln alle 20 Sekunden, Upload und Seitenwechsel der Tabelle
sind nur angedeutet, die übrigen Einträge der Seitenleisten führen nirgends hin, die Abdeckung nach
einer Zuordnung ist geschätzt, Stream-Adressen sind erfunden.

Die Mockups zeigen echte Sendernamen, aber **keine echten Logos und keine echten Cover**: alle Bilder
sind erzeugte Initialen- und Farbkacheln. Titel sind Beispieltexte; Ton spielt das Mockup nicht.
