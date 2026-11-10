# Konzept: Podcasts, Radio, Hörbücher, quadratische Cover und Lautstärke

- **Status: Fassung 3 (10. November 2026), Kritiker-Review eingearbeitet (REVISE, 0 Blocker,
  13 Major, 6 Minor; Abschnitt 18), mit klickbaren Mockups.** Marco hat am 9. November 2026 die
  Grundsatzfragen **F-1 bis F-4** beantwortet (Abschnitt 2) und nach dem ersten Blick auf die Mockups
  entschieden: der Umfang hat **drei** Werte («Nur neue Folgen», «Neue + letzte N», «Alle Folgen»),
  «Gleichzeitig laden» geht **bis 5**, die abonnierten Podcasts erscheinen als **Liste mit Covern**
  (F-5, Variante A), die Einstellung steht als **Band auf der Podcasts-Seite** (F-6, Variante B).
  Offen sind nur die Punkte in Abschnitt 17.
  **Fassung 5 (10. November 2026): Teil B Radio dazu.** Marco will Podcast und Radio in einem Konzept.
  Teil B beschreibt das Radio-Angebot (Länder, je Land einzelne Sender) und die Sender-Metadaten,
  die der Server erst nach dem Hinzufügen holt; entschieden über klickbare Mockups (B2, Anhang B),
  Kritiker-Review eingearbeitet (REVISE, 0 Blocker, 17 Major, 10 Minor; B15). Offen sind die Punkte
  in B14.
  **Fassung 6 (10. November 2026): Teil C dazu.** Hörbücher und Podcasts zeigen in der Übersicht ihrer
  Bibliothek quadratische Cover ohne Rahmen; die Startseite bleibt, wie sie ist. Offen: C6.
  **Fassung 7 (10. November 2026): Teil D dazu.** Hörbuch-Reihen im Bereich Hörbücher statt unter
  «Sammlungen», Bleistift und Editor für Hörbücher, externe Quellen («Scraper»); mit klickbaren
  Mockups (Anhang A). Offen: D11.
  **Fassung 8 (10. November 2026):** Marcos Antworten auf alle offenen Fragen der Teile A bis D
  eingetragen (17, B14, C6, D11). **Teil E dazu:** zwei Fehler der Lautstärke (Radio ohne Regler,
  Regler im Podcast-Player flackert); OE-1 und OE-2 beantwortet.
  **Die Umsetzung beginnt erst auf Marcos ausdrückliches Wort.**
- **Aufbau:** **Teil A Podcasts** (Abschnitte 1 bis 18, Anhang A), **Teil B Radio** (Abschnitte
  B1 bis B15, Anhang B), **Teil C Quadratische Cover** (Abschnitte C1 bis C6) und **Teil D Hörbücher**
  (Abschnitte D1 bis D11, Mockups in Anhang A) und **Teil E Lautstärke** (Abschnitte E1 bis E6). Die
  Teile sind unabhängig; jeder hat eigene Stufen und Releases.
- **Auslöser:** Marco will, dass Slouchcat neue Podcast-Folgen als Datei in den Ordner des Podcasts
  speichert, statt nur den Link zum Anbieter zu halten. Vorhandene Dateien sollen wie heute erkannt
  und ausgelesen werden. In der Verwaltung soll wählbar sein, ob neue Folgen gespeichert oder nur
  verlinkt werden. Dazu soll die Seite «Podcasts» der Verwaltung schöner werden; er wollte Mockups für
  die Einstellung und für die abonnierten Podcasts, klickbar.
- **Verankert gegen:** `main` **`e90109a6`** («Merge pull request #131», Patch 1.23.1),
  Vertragsfassung **1.23.0** (`api/openapi.yaml`, `info.version`), letzte Migration in `main`
  **`00043_trickplay_off.sql`**, letzte ADR **`0028`**, letzte Frage im Entscheidungsprotokoll **25**.
- **Nummern:** Dieses Papier ist **Konzept 19**, reserviert **Frage 26** (beide Teile), **ADR 0029**
  («Podcast-Folgen als Datei: schreibbare Podcast-Ordner») und **ADR 0030** («Radio-Angebot statt
  ganzer Katalog»). Vorläufig gelten für Teil A **Vertrag 1.24.0 (additiv)** und **1.25.0 (additiv)**
  sowie **Migration 00044**, für Teil B **1.26.0 (brechend für die Verwaltung)** und **1.27.0
  (additiv)** sowie **Migrationen 00045 und 00046**. Die echten Nummern werden beim Baubeginn vergeben.
- `:Zeile`-Angaben beziehen sich auf `e90109a6`; **nach Namen suchen, nicht nach Zeile.** Pfade ohne
  Präfix liegen unter `server/` (Go) oder `web/src/` (Web-App), wo es eindeutig ist.
- Massgeblich bleiben `docs/spec/01-anforderungen-v1.md`, `docs/planning/04-entscheidungen.md` und
  `docs/adr/`. Bei Widerspruch gilt: **Entscheidungsprotokoll > Plan > Spec** (`CLAUDE.md`). Dieses
  Papier hebt einen Satz der Spec teilweise auf (B-1 «read-only», Abschnitt 3); erst Marcos Antwort
  auf Frage 26 ändert ihn.

> **Dieses Papier ändert keine Zeile Code.** Teil A beschreibt, wie der Server Folgen in den
> Podcast-Ordner lädt, wie die Einstellung dazu aussieht und wie die Seite «Podcasts» neu gebaut
> wird; Abschnitt 15 teilt den Bau in zwei Stufen. Teil B beschreibt, wie der Admin das Radio-Angebot
> wählt und woher der Server Logos und weitere Angaben holt; B12 teilt den Bau in zwei Stufen. Teil C
> beschreibt quadratische Cover in der Übersicht von Hörbuch- und Podcast-Bibliotheken (eine Stufe).
> Teil D beschreibt Reihen, Bleistift und externe Quellen für Hörbücher (vier Stufen, D9). Teil E behebt
> zwei Fehler der Lautstärke (eine Stufe).

# Teil A: Podcasts

## 1. Was Marco will

| Nr. | Marcos Wunsch (9. November 2026, sinngemäss) | Abschnitt |
|---|---|---|
| W-1 | Slouchcat speichert neue Folgen als Datei im Ordner des Podcasts. | 6, 7 |
| W-2 | Bestehende Dateien werden wie jetzt erkannt, alle Daten ausgelesen. | 8 |
| W-3 | In der Verwaltung gibt es eine Option: neue Folgen speichern oder nur den Link bereitstellen. | 5, 12 |
| W-4 | Die Verwaltung der Podcasts wird schöner dargestellt. | 12 |
| W-5 | Mockups für die Einstellung und für die abonnierten Podcasts, klickbar. | 12, Anhang A |

## 2. Interview vom 9. November 2026

| Nr. | Frage | Marcos Antwort |
|---|---|---|
| F-1 | Wo wird eingestellt, ob gespeichert oder verlinkt wird? | **Je Bibliothek, je Podcast abweichend** (Empfehlung). |
| F-2 | Was wird geladen, wenn Speichern eingeschaltet ist? | **Eine Auswahl aus drei Werten:** «Nur neue Folgen», «Neue + letzte N», «Alle Folgen (plus neue)». Zuerst nannte Marco auch «Neue und fehlende Folgen»; nach dem Mockup hat er den Wert gestrichen. **Gleichzeitig laden: bis zu 5.** |
| F-3 | Audiobookshelf lädt heute noch in denselben Ordner. Wie weiter? | **Slouchcat übernimmt.** Marco schaltet die automatischen Downloads in Audiobookshelf ab; Slouchcat schreibt im selben Layout. |
| F-4 | Wie lange bleiben gespeicherte Folgen? | **Für immer.** Slouchcat löscht nie eine Datei im Podcast-Ordner. |
| F-5 | Welche Variante für die abonnierten Podcasts (A Liste, B Raster, C Tabelle)? | **A, Liste mit Covern** (Empfehlung). |
| F-6 | Welche Variante für die Einstellung (A Dialog in der Bibliothek, B Band auf der Podcasts-Seite)? | **B, Band auf der Podcasts-Seite** (empfohlen war A). Folge: eine Bestätigung mit Schätzung, bevor etwas nachgeladen wird (12.3). |

## 3. Was sich gegenüber früheren Entscheiden ändert

| Bisher | Neu mit diesem Konzept | Grund |
|---|---|---|
| Spec **B-1**: Der Admin «bindet Medien read-only ein». `deploy/compose.yaml` bindet `${MEDIA_DIR}` mit `:ro` ein. | **Ausnahme Podcast-Ordner:** ein Ordner mit `content = 'podcast'` darf schreibbar eingebunden sein; nur dorthin schreibt der Server. Film-, Serien- und Hörbuchordner bleiben read-only. | W-1, F-3 |
| Entscheid vom 21. Oktober 2026 (Sicherung): «Medienwurzeln bleiben read-only (B-1)», kein zweiter schreibbarer Mount. | Für die Sicherung gilt das weiter. Neu gibt es genau einen schreibbaren Medien-Mount, den Podcast-Ordner, als freiwillige Zusatzdatei für Compose. | W-1 |
| ADR 0007: Eine Feed-Episode hat keine Datei, sie wird über den Server vom Anbieter gestreamt (`ProxyEnclosure`). | Bleibt für «Nur Link» und als Übergang, bis die Datei da ist. Bei «Speichern» bekommt die Feed-Episode eine `media_files`-Zeile. | W-1, W-3 |
| ADR 0015: Podcast-Ordner werden nur gelesen; eine Datei entsteht ausserhalb (Audiobookshelf) und findet über `finishPodcastEpisode` zu ihrer Feed-Episode. | Bleibt für fremde Dateien. Dateien, die Slouchcat selbst schreibt, trägt der Download direkt an der Feed-Episode ein, ohne Umweg über das Verheiraten. | W-1, W-2 |
| Der Server schreibt nur nach `/config` und `/cache` (Befund in Abschnitt 4). | Er schreibt zusätzlich in Podcast-Ordner: nur neue Dateien, nie überschreiben, nie löschen. | W-1, F-4 |

Unberührt bleiben Regel 2 (Titelabfragen nur über `visibility`; Planer und Download laufen im Paket
`library`, die Podcast-Liste der Verwaltung im Paket `metadata`, beide stehen auf der Liste),
Regel 3 (`os/exec` nur in `transcode`), Regel 12 (kein Entscheid im Client) und das Nicht-Ziel
«Download von YouTube-Inhalten»: geladen wird nur die Enclosure eines abonnierten Feeds.

## 4. Befund: Stand heute

**Code (`e90109a6`):**

- `metadata.Service.FetchPodcast` und `storeEpisodes` (`internal/metadata/podcast.go`) legen je
  Feed-Eintrag ein `podcast_episode`-Item mit `enclosure_url` an. Abruf alle 55 Minuten über
  `podcast.poll` und `podcast.fetch`. Welche Episoden «neu» sind, rechnet `storeEpisodes` **beim
  Abruf** aus («veröffentlicht nach der bisher jüngsten») und gibt die Ids nur an den
  Benachrichtigungs-Haken; gespeichert wird das nirgends. Beim allerersten Abruf (`Subscribe`) gilt
  jede Episode als neu.
- Abspielen: `playback.Service.Start` nimmt zuerst die Datei (`loadInput`); nur ohne verfügbare
  Datei startet `startEnclosure` einen Durchreich-Stream (`ProxyEnclosure`). **Eine Episode mit Datei
  spielt also heute schon immer die Datei.** «Verfügbar» heisst dabei nur «nicht als fehlend
  markiert»; ein Sondenfehler ändert es nicht. Der Geräte-Download (`offline.audioSource`) hält es
  gleich.
- Podcast-Ordner: `library/podcast_folder.go` liest je Unterordner `metadata.json` und Cover, jede
  Audiodatei wird eine Datei-Episode. `finishPodcastEpisode` (`library/probe.go`) verheiratet sie
  nach der Sonde mit der Feed-Episode, wenn genau eine passt (gleiche Minute oder gleicher Titel,
  Dauer ±2 %), sonst bleiben zwei Episoden (`file_and_feed` in den Pflegewerkzeugen).
- Eine Enclosure laden kann der Server schon: `offline.fetchSource` (`internal/offline/audio.go`)
  lädt sie für Geräte-Downloads in den Cache, mit Wiederaufnahme (`.part`, `Range`, `If-Range`),
  Grenze 2 GiB, Netzwächter (`netguard.Clients.For` mit `feed_origin`). Die Funktion ist privat und
  an den Download-Ordner gebunden; sie legt Ordner mit 0750 und Dateien mit 0640 an.
- Kein Code schreibt heute in eine Bibliothekswurzel. `paths.Open` liefert ein `*os.Root`; damit ist
  Schreiben innerhalb der Wurzel möglich und ein Ausbruch aus ihr nicht.
- Der Scanner erkennt Dateien an `root_id + rel_path`, Grösse, `mtime`, Gerät und Inode und einem
  Teil-Hash (`PartialHash`). **Er, nicht der Sweep, markiert eine verschwundene Datei als fehlend**
  (`missing_since`); die Zeile bleibt. Der Sweep entfernt nur Titel, deren Dateien länger als
  24 Stunden fehlen, und lässt Feed-Episoden mit Enclosure stehen. Ein nicht eingebundener Ordner
  (leer, verschwunden, Verlust über 20 %) bricht den Scan ab; dann wird nichts als fehlend markiert.
- `media_files` hat `UNIQUE (root_id, rel_path)`; auch Zeilen fehlender Dateien halten ihren Pfad.
- `Ignored` (`library/names.go`) übergeht Namen mit führendem Punkt, in Scanner, Watcher und Poller.
  Gescannte Audio-Endungen (`audioExts`): `m4b`, `m4a`, `mp3`, `flac`, `ogg`, `opus`.
- Voll- und Pfad-Scan nehmen dieselbe Sperre je Bibliothek (`lockScan` wartet, `tryLockScan` gibt
  sofort auf).
- Die Seite «Podcasts» (`features/admin/PodcastsPage.tsx`) ist eine Tabelle mit Titel, Episoden,
  letztem Abruf und Fehler; aufgeklappt (`PodcastDetail`) Autor, Feed-Adresse, Bibliothek, «Jetzt
  abrufen». Die Liste baut `metadata.ListPodcasts` (Abfrage `PodcastList`).

**Marcos NAS** (`/mnt/nerdnas/medien/Audiobookshelf/Podcast`, gemessen am 9. November 2026, nur
lesend):

| Grösse | Wert |
|---|---|
| Podcast-Ordner | 102, davon 95 mit `metadata.json` und `feedURL` |
| Dateien | 26'538, davon 26'195 MP3, 137 M4A, 10 MP4 (Video, 13 GB, wird nicht gescannt) |
| Belegt | 1,44 TB, im Mittel 54 MB je Audiodatei |
| Frei auf dem Laufwerk | 62 TB |
| Layout | `<Podcast-Titel>/<Folgentitel>.mp3`, `metadata.json`, `cover.jpg`; Rechte 777 |
| Audiobookshelf | lädt noch: jüngste Datei in «Weltwoche Daily» vom 9. November 2026, 18:00 |

Wichtig für Abschnitt 8: Audiobookshelf benennt die Datei nach dem **Folgentitel**. Dieser Name ist
das stärkste Merkmal, um eine vorhandene Datei einer Feed-Episode zuzuordnen.

## 5. Die Einstellung

### 5.1 Zwei Ebenen

| Ebene | Werte | Ab Werk |
|---|---|---|
| Bibliothek (Art Podcasts) | **Neue Folgen:** `save` «Speichern» oder `link` «Nur Link». **Umfang** (nur bei `save`): `new`, `last_n` (mit N von 1 bis 100), `all`. **Ordner für neue Abos:** einer der Podcast-Ordner der Bibliothek. **Gleichzeitig laden:** 1 bis 5. | `link`, Umfang `new`, N 10, erster Podcast-Ordner, 2 |
| Podcast | **Neue Folgen:** «Wie Bibliothek», «Speichern», «Nur Link». **Umfang:** «Wie Bibliothek» oder einer der drei Werte (mit eigenem N). | beide «Wie Bibliothek» |

Ab Werk steht `link`: nach dem Update verhält sich Slouchcat genau wie heute, bis Marco «Speichern»
wählt (O-2). Es gilt die Einstellung des Podcasts, wenn er eine eigene hat, sonst die der Bibliothek.
Diese Regel steht **an einer Stelle, in SQL**: eine gemeinsame Abfrage über `libraries` und
`podcast_settings`, die `metadata` (für die Liste der Verwaltung) und `library` (für den Planer)
beide benutzen. Der Vertrag liefert das Ergebnis mit (`effectiveMode`, `effectiveScope`,
`effectiveLastN`); der Client rechnet nichts (Regel 12). Ein Ordner-Podcast ohne Feed hat nichts zu
laden und zeigt die Einstellung nicht.

### 5.2 Was die drei Umfänge heissen

| Umfang | Geladen wird | Beispiel «Weltwoche Daily» (2'017 im Feed, 545 Dateien) |
|---|---|---|
| **Nur neue Folgen** (`new`) | Folgen, die ein Feed-Abruf als neu meldet, **solange «Speichern» gilt**. | nur was ab jetzt erscheint |
| **Neue + letzte N** (`last_n`) | Wie `new`, dazu die N jüngsten Folgen des Feeds, soweit sie keine Datei haben. | zusätzlich höchstens 10 |
| **Alle Folgen** (`all`) | Wie `new`, dazu jede gespeicherte Folge des Feeds ohne Datei. | zusätzlich rund 1'470 Folgen, etwa 36 GB (seine Dateien haben im Mittel 25 MB) |

Lücken im Bestand eines Podcasts füllt kein eigener Umfang: dafür gibt es «Alle Folgen» oder den
Knopf «Fehlende nachladen» am Podcast (5.3).

**Was «neu» heisst, entscheidet der Feed-Abruf, nicht der Planer.** `storeEpisodes` kennt die Ids
der Episoden, die es heute als neu meldet. Ein neuer Haken `metadata.WithPodcastFetched(podcastID,
newIDs)` (verdrahtet in `cmd/slouchcat/main.go`, wie `WithNewEpisodes`) gibt sie an `library`
weiter; der Planer (6.1) lädt bei `new` genau diese Ids. Daraus folgt:

- **Ein neues Abo lädt bei «Nur neue Folgen» nichts aus dem ersten Abruf.** `Subscribe` meldet
  dem Haken keine Ids; sonst lüde das Abonnieren den ganzen Feed. Bei «Neue + letzte N» kommen die
  N jüngsten, bei «Alle Folgen» alle; der Dialog «Feed abonnieren» sagt vorher, was geladen wird.
- Eine Episode ohne Datum gilt nie als neu, sobald der Podcast eine datierte hat (heutige Regel).
- Ein Feed, der seine GUIDs erneuert oder alte Folgen nachreicht, löst keine Lawine aus (heutige
  Regel: nur was nach der bisher jüngsten erschien).
- Der Server hält je Feed höchstens die 2'000 jüngsten Episoden (`maxFeedEpisodes`). «Alle Folgen»
  meint diese; bei «Weltwoche Daily» bleiben die 17 ältesten aussen vor.

### 5.3 Was eine Änderung bewirkt

| Änderung | Wirkung |
|---|---|
| `link` → `save` | Der Planer läuft für die betroffenen Podcasts. Bei `new` lädt er nichts nach, bei `last_n` und `all` fragt das Band zuerst nach (12.3). |
| `save` → `link` | Wartende Downloads werden abgebrochen, laufende beendet der Abbruch-Haken und entfernt die Teildatei. **Gespeicherte Dateien bleiben** (F-4). |
| Umfang grösser, N grösser | Das Band fragt zuerst nach und zeigt, wie viele Folgen und wie viel Platz das ungefähr sind. Erst nach der Bestätigung gilt der neue Wert, und der Planer reiht die zusätzlichen Folgen ein. |
| Umfang kleiner, N kleiner | Wartende Downloads, die nicht mehr dazugehören, werden abgebrochen. Nichts wird gelöscht. |
| Knopf «Fehlende nachladen» am Podcast | Einmalig alle Folgen des Feeds ohne Datei, gleich welcher Umfang gilt. Auch einzeln je Folge («Laden»). |

### 5.4 Der Ordner muss schreibbar und der richtige sein

«Speichern» lässt sich nur wählen, wenn der Zielordner die **Ordnerprüfung** besteht. Der Server
macht sie beim Laden der Podcasts-Seite, beim Speichern der Einstellung (422 `root_not_writable`)
und vor jedem Download:

1. **Derselbe Dateisystemtyp wie beim Anlegen** (`paths.FSTypeAt` gleich `library_roots.fs_type`),
   und mindestens ein bekannter Podcast-Ordner der Wurzel ist da. Ist das NAS beim Start nicht
   eingebunden, zeigt der Pfad auf ein leeres lokales Verzeichnis; ohne diese Prüfung schriebe der
   Server auf die lokale Platte, auf der auch die Datenbank liegt.
2. **Schreibtest:** eine Datei `.slouchcat-schreibtest` anlegen, einen harten Link darauf setzen
   (6.2, Schritt 6 braucht ihn), beide entfernen.
3. **Rechte:** die Testdatei hat die erwartete Gruppe und Lesbarkeit (6.4).

Scheitert die Prüfung, sagt das Band warum («nur lesbar eingebunden», «Ordner ist nicht
eingebunden», «Dateisystem kann keine harten Links») und verweist auf die Installationsanleitung
(Abschnitt 13).

## 6. Vom Feed zur Datei

```
podcast.fetch (wie heute)            podcast.sync (neu)               podcast.download (neu)
Feed lesen, Episoden anlegen   ──▶   Einstellung + Umfang auswerten ──▶ je Folge: laden, prüfen,
neue Episode melden (wie heute)      Folgen einreihen                  verlinken, an der
Haken: Podcast + neue Ids                                              Feed-Episode eintragen
```

### 6.1 Der Planer `podcast.sync`

Ein Job je Podcast im Paket `library` (dort entstehen `media_files`). `metadata` importiert
`library` weiterhin nicht; es ruft nur Haken, die `main.go` verdrahtet. Der Planer läuft:

- nach einem Feed-Abruf, **wenn die wirksame Einstellung `save` ist und der Abruf neue Ids
  meldet** (der Haken reiht sonst nichts ein: 94 Podcasts mal 26 Abrufe am Tag sollen keine
  leeren Läufe erzeugen),
- nach jeder Änderung der Einstellung (Bibliothek oder Podcast) und durch die Knöpfe,
- alle 6 Stunden über den neuen Zeitplan `podcast.sync_all` für Podcasts auf `save`, die
  fehlgeschlagene Folgen haben oder deren Umfang `last_n` oder `all` ist. Ohne ihn bekäme ein Feed,
  der nur «unverändert» antwortet, nie einen neuen Versuch.

Der Planer wählt nach 5.2 die Folgen aus und reiht je Folge `podcast.download` ein
(Idempotenz-Schlüssel `podcast-dl:<episodeId>`, Nutzlast nur `{episodeId}`). Er lässt aus:

- Folgen, die gelöscht sind (`deleted_at`), und Folgen eines gelöschten Podcasts,
- **Folgen, die schon eine `media_files`-Zeile haben, fehlend oder nicht.** Eine Folge, deren
  Zeilen alle fehlen, gilt als «entfernt» (Abschnitt 8); sie wird nur auf «Laden» hin geholt,
- Folgen, deren Download läuft oder wartet, und solche, die vor weniger als 6 Stunden fehlschlugen,
- Folgen, zu denen im Ordner schon eine passende Datei liegt oder liegen könnte (Abschnitt 8),
- Folgen mit einem Format, das der Scanner nicht liest (O-3).

### 6.2 Der Download `podcast.download`

| Schritt | Was geschieht |
|---|---|
| 1. Prüfen | Episode und Podcast leben, Episode ohne `media_files`-Zeile, wirksame Einstellung verlangt sie noch (oder der Auftrag kam von einem Knopf). Ordnerprüfung (5.4), freier Platz über der Schwelle (wie Trickplay: das Kleinere von 10 GiB und 10 %). Scheitert Ordner oder Platz: `RetryLater` in 30 Minuten, ohne einen Versuch zu verbrauchen, höchstens 24 Stunden lang, dann `failed`. |
| 2. Ordner | Der Podcast hat schon einen Ordner: der bleibt, auch wenn er umbenannt wurde. Er ergibt sich aus der jüngsten vorhandenen Datei des Podcasts, nicht aus dem Titel und nicht aus einer gespeicherten Spalte. **Einen Ordner, den die Datenbank kennt, legt der Server nie neu an;** fehlt er, wartet der Auftrag (`root_missing`). Liegt er in einer nur lesbaren Wurzel, ist der Podcast «nicht speicherbar»; es gibt kein Ausweichen in einen zweiten Ordner. Hat ein Podcast zwei Ordner (`two_folders_one_feed`), gilt der mit der jüngsten Datei. Nur ein Podcast ganz ohne Datei bekommt `<Ordner für neue Abos>/<Podcast-Titel>/` mit `metadata.json` (Audiobookshelf-Schema mit `feedURL`) und `cover.jpg`. Eine vorhandene `metadata.json` wird nie überschrieben. |
| 3. Laden | In eine Teildatei **im selben Ordner**: `.slouchcat-<episodeId>.part`, daneben `.slouchcat-<episodeId>.validator`. Der führende Punkt hält Scanner, Watcher und Poller fern; derselbe Ordner erlaubt den harten Link in Schritt 6. Wiederaufnahme, Grenze 2 GiB, Netzwächter und Weiterleitungsregel wie beim Geräte-Download. Ein neuer Versuch mit vollständiger Teildatei lädt nicht noch einmal. |
| 4. Sonde | Die Teildatei wird mit derselben Sonde geprüft wie jede Mediendatei (`transcode.Runner.Probe`): sie muss eine Tonspur und eine Dauer haben, und die Endung aus Inhaltstyp oder Adresse muss zu `audioExts` gehören. Sonst Fehlschlag `not_audio`, die Teildatei wird entfernt. **Eine kaputte Datei wird so nie eingetragen**; die Folge bleibt Link. |
| 5. Name | `<Folgentitel>.<Endung>`, bereinigt (6.3). Ein Name ist frei, wenn es ihn **weder im Ordner noch in `media_files`** gibt. Sonst wird das **Veröffentlichungsdatum** vorangestellt (`2026-11-09 <Folgentitel>.mp3`), dann eine Zahl. |
| 6. Eintragen | Unter der Scan-Sperre der Bibliothek (`lockScan`, höchstens 10 Minuten warten, sonst `RetryLater` in einer Minute; die Teildatei bleibt). Unter der Sperre noch einmal prüfen: Episode und Podcast leben, Einstellung verlangt sie noch, **Episode hat inzwischen keine Datei** (Audiobookshelf oder das Verheiraten können sie in der Zwischenzeit gebracht haben). Dann: harter Link von der Teildatei auf den Zielnamen. Der Link scheitert, wenn es den Namen schon gibt, auch über NFS; ein Umbenennen würde eine fremde Datei still ersetzen. Bei «gibt es schon» der nächste Name. In derselben Sperre die `media_files`-Zeile **an der Feed-Episode** anlegen (Wurzel, Pfad, Grösse, `mtime`, Gerät, Inode, Container, Teil-Hash) samt dem, was die Sonde aus Schritt 4 gelesen hat. Zuletzt Teildatei und Prüfdatei entfernen. |
| 7. Fertig | Ab jetzt nimmt `loadInput` die Datei (die Episode war schon vorher verfügbar, über die Enclosure). Fortschritt, Favorit und Gesehen-Haken hängen am selben Item und bleiben. Ein Scan, der die Datei danach sieht, kennt sie schon. |

Scheitert eine der Prüfungen unter der Sperre, entfernt der Auftrag seine Teildatei und endet ohne
Datei. Wird eine entfernte Folge über «Laden» neu geholt und trägt die Episode noch eine fehlende
Zeile, wird **diese Zeile wiederbelebt** (neuer Pfad, Grösse, `mtime`, Inode, Hash, `missing_since`
leer) statt einer zweiten angelegt.

Die Ladelogik zieht aus `offline.fetchSource` in ein gemeinsames Paket um (Arbeitsname
`internal/fetch`: Wiederaufnahme, Grenzen, `contentRange`, Endungen), das `offline` und `library`
benutzen. Verhalten und Tests des Geräte-Downloads bleiben gleich.

**Warteschlange:** eigener Pool `podcast` mit fünf Arbeitern. Der Pool `download` (ein Arbeiter)
bleibt den Geräte-Downloads, damit ein Nachladen von tausend Folgen niemanden am Handy warten lässt.
Zwei Zähler im Paket `library` begrenzen, was gleichzeitig lädt: **je Bibliothek** so viele, wie
«Gleichzeitig laden» sagt (1 bis 5, ab Werk 2), und **je Anbieter-Host einer**. Über alle
Bibliotheken zusammen sind es nie mehr als fünf. Als Host gilt der Host der Enclosure-Adresse
**vor** Weiterleitungen; er ist vor dem Abruf bekannt. Viele Podcasts stehen hinter demselben
Zähl-Dienst (Podtrac, Chartable) und laden dann nacheinander (O-10). Ein Arbeiter, der auf einen
Zähler wartet, behält seinen Platz; der Pool gehört nur dieser Jobart.

Die Jobart ist `Windowable` (Konzept 15): auf «Hintergrundjobs» lässt sich ein Zeitfenster setzen,
pausieren und abbrechen wie bei jeder Jobart (ADR 0017). Fortschritt meldet `jobs.Progress` in
Bytes; die Notiz nennt nur den Folgentitel.

**Fehler:** fünf Versuche mit wachsender Pause (Jobsystem, zusammen rund drei Minuten), danach
`failed` mit Code (`enclosure_unavailable`, `too_large`, `not_audio`, `no_space`,
`root_not_writable`, `root_missing`). `podcast.sync_all` versucht eine fehlgeschlagene Folge alle
6 Stunden erneut, sieben Tage lang; danach bleibt sie `failed`, bis jemand «Erneut» drückt.
**Solange keine Datei eingetragen ist, bleibt die Folge als Link abspielbar.**

**Aufräumen:** der stündliche `library.sweep` entfernt `.slouchcat-*`-Dateien, die älter als sieben
Tage sind und zu keinem laufenden oder wartenden Auftrag gehören (Absturz, voller Datenträger).
Eine Teildatei eines unterbrochenen Downloads bleibt bis dahin für die Wiederaufnahme liegen.

### 6.3 Dateinamen

Aus dem Folgentitel: Steuerzeichen und `/ \ : * ? " < > |` werden zu Leerzeichen, mehrfache
Leerzeichen zu einem, Punkte und Leerzeichen am Ende fallen weg, gekürzt auf 180 Bytes an einer
Zeichengrenze. Leer oder nur Punkte: `Folge <Veröffentlichungsdatum>`. Der Name wird über `os.Root`
relativ zum Podcast-Ordner angelegt; ein Titel kann den Ordner nicht verlassen (Regel 4).

Audiobookshelf bereinigt anders (soweit bekannt: unerlaubte Zeichen fallen weg, der erste
Doppelpunkt wird zu ` - `). Zum **Vergleichen** mit vorhandenen Dateien dient darum nicht der Name,
sondern ein Schlüssel (Abschnitt 8). Vor M-1 wird Audiobookshelfs `sanitizeFilename` nachgelesen.

### 6.4 Rechte

Dateien entstehen mit 0664, Ordner mit 0775 (nicht mit den engeren Rechten des Geräte-Downloads),
damit Audiobookshelf und Freigaben des NAS neue Folgen lesen können. Besitzer ist der Benutzer des
Containers; im unprivilegierten LXC von CT 190 ist das eine andere Nummer als die von
Audiobookshelf (`100999:100991`). Die Ordnerprüfung (5.4) meldet, wenn Gruppe oder Lesbarkeit nicht
stimmen.

### 6.5 Tags schreiben (O-7)

Marco will, dass gespeicherte Folgen Titel und Datum in den Tags tragen (10. November 2026). Neu
zwischen Schritt 4 (Sonde) und Schritt 5 (Name) in 6.2:

- **4b. Tags:** `transcode.Runner.TagAudio` schreibt mit ffmpeg ohne Umkodieren (`-map 0 -c copy`,
  Regel 3: nur im Paket `transcode`, Argumentliste) die Teildatei in eine zweite eigene Teildatei
  `.slouchcat-<episodeId>.tagged`: **Titel** der Folge, **Album** = Name des Podcasts, **Interpret** =
  Autor des Feeds, **Datum** = Veröffentlichung, **Kommentar** = Adresse der Seite der Folge, wenn sie
  im Feed steht. Vorhandene Tags des Anbieters bleiben, wo Slouchcat nichts setzt.
- Nur für `mp3` (ID3v2.4) und `m4a`/`m4b` (MP4-Tags); andere Formate bleiben unverändert.
- Danach läuft die Sonde noch einmal auf der getaggten Datei; schlägt sie fehl, nimmt der Download die
  ungetaggte Teildatei und schreibt einen Hinweis ins Jobprotokoll.
- Die Datei weicht damit von der des Anbieters ab. Der Schlüssel gegen doppelte Downloads (Abschnitt 8)
  vergleicht Namen und Titel, nicht Inhalte; er ist davon nicht betroffen.
- Ein vorhandenes Cover in der Datei bleibt; Slouchcat bettet kein eigenes ein.
- Test P-19: eine geladene `mp3` und eine `m4a` tragen danach Titel, Album und Datum; eine `ogg` bleibt
  unverändert; die Sonde nach dem Taggen schlägt in einem Test fehl und die ungetaggte Datei wird
  eingetragen.

## 7. Abspielen, Benachrichtigen, Geräte-Downloads

- **Abspielen:** keine Änderung am Code der Wiedergabe. Solange die Datei fehlt, streamt die Folge
  vom Anbieter; sobald die `media_files`-Zeile da ist, nimmt `Start` die Datei. Weil die Sonde vor
  dem Eintragen läuft (6.2, Schritt 4), ist eine eingetragene Datei abspielbar. Läuft gerade ein
  Stream, wenn die Datei fertig wird, läuft er zu Ende; der nächste Start nimmt die Datei.
- **Neue Folge melden:** wie heute beim Feed-Abruf, nicht erst nach dem Download (O-5).
- **Geräte-Downloads (O-3 der Spec):** unverändert; `audioSource` nimmt die Datei, wenn sie da ist.
  Ein Geräte-Download, der vor der Datei begann, lädt die Enclosure selbst noch einmal in den Cache
  (doppelter Verkehr, sonst harmlos).
- **Sichtbarkeit:** unverändert. Die Datei ändert weder Freigabe noch Bibliothek der Folge. Der
  Auftrag kennt kein Profil; «lebt» heisst «nicht gelöscht».
- **Zuschauer-Vertrag:** kein `scoped`-Schema ändert sich.

## 8. Bestehende Dateien und der Wechsel von Audiobookshelf

**Erkennen bleibt, wie es ist (W-2).** Scanner, `metadata.json`, Cover, Tags und das Verheiraten
über `finishPodcastEpisode` ändern sich nicht. Dateien, die Marco von Hand in einen Podcast-Ordner
legt, werden weiter erkannt.

**Das Risiko doppelter Dateien.** Heute bleiben Datei- und Feed-Episode getrennt, wenn das
Verheiraten kein eindeutiges Paar findet (zum Beispiel, wenn der Anbieter Werbung einsetzt und die
Dauer um mehr als 2 % abweicht). Für den Planer sähe die Feed-Episode dann aus wie «ohne Datei»,
und bei «Alle Folgen» oder «Neue + letzte N» lüde er eine Folge, die schon im Ordner liegt.
Dagegen stehen drei Riegel:

1. **Abgleich über den Namen, mit Schlüssel.** Aus Folgentitel und aus jedem Dateinamen (ohne
   Endung) wird ein Schlüssel gebildet: Unicode-NFC, klein, nur Buchstaben und Ziffern. Verglichen
   wird gegen **jede** `media_files`-Zeile in den Ordnern des Podcasts, gleich an welchem Item sie
   hängt (auch an gelöschten Episoden eines früheren Abos). Bei genau einem Treffer an einer
   Datei-Episode ohne GUID wird **nicht geladen, sondern verheiratet** (derselbe Weg wie
   `finishPodcastEpisode`). Bei einem Treffer an anderer Stelle wird nicht geladen, die Folge ist
   «unklar».
2. **Mehrere Kandidaten.** Passen nach der heutigen Regel zwei Datei-Episoden, lädt der Planer nicht;
   die Folge ist «unklar».
3. **Kein Kandidat, aber eine Datei in der Nähe.** Liegt im Podcast eine unverheiratete
   Datei-Episode, deren Datum höchstens 24 Stunden vom Veröffentlichungsdatum der Folge abweicht,
   lädt der Planer nicht; die Folge ist «unklar».

«Unklar» zeigt die Seite mit dem Verweis auf die Pflegewerkzeuge; «Laden» an der Folge holt sie
trotzdem.

**Messauftrag M-1 vor dem Bau:** auf CT 190 je Podcast zählen: Feed-Episoden ohne Datei,
Datei-Episoden ohne GUID, Paare nach Schlüssel, Paare nach Audiobookshelfs Namensregel, Folgen ohne
jeden Kandidaten, und wie viele verschiedene Enclosure-Hosts es gibt. Das Ergebnis kommt nach
`docs/perf/19-podcasts.md` und entscheidet, ob die Riegel reichen.

**Von Hand gelöschte Dateien.** Löscht Marco eine Podcast-Datei, markiert der nächste Scan ihre
Zeile als fehlend; die Zeile bleibt an der Episode. Eine Folge, deren Zeilen alle fehlen, heisst
«entfernt». Der Planer lädt sie nicht von selbst wieder, auch nicht bei «Alle Folgen» (6.1); «Laden»
an der Folge holt sie zurück. Dafür braucht es keinen gespeicherten Zustand. Ist der ganze Ordner
weg (NAS nicht eingebunden), bricht der Scan ab und markiert nichts; es wird weder etwas «entfernt»
noch neu geladen.

**Der Wechsel (F-3), in dieser Reihenfolge:**

1. Podcast-Ordner schreibbar einbinden (Abschnitt 13), Update einspielen. Einstellung steht auf
   «Nur Link», nichts ändert sich.
2. In Audiobookshelf je Podcast den automatischen Download abschalten.
3. In Slouchcat die Bibliothek auf «Speichern» stellen, Umfang nach Wahl.

Zwischen 2 und 3 erschienene Folgen fängt «Neue + letzte N» oder der Knopf «Fehlende nachladen».
Laufen beide eine Weile parallel, schützt Schritt 6 in 6.2 vor überschriebenen und doppelt
eingetragenen Dateien, nicht aber vor zwei Dateien mit leicht anderem Namen; darum die Reihenfolge.

**Abos ohne Ordner** (von Hand abonniert): bekommen beim ersten Download einen Ordner mit
`metadata.json` und Cover (6.2, Schritt 2). Findet ein späterer Scan diesen Ordner, erkennt er über
die `feedURL` das bestehende Abo (`findOrCreatePodcast`) und legt keinen zweiten Podcast an.
`feed_origin` bleibt `admin`. Die `metadata.json` enthält die Feed-Adresse; bei einem privaten Feed
steht damit sein Schlüssel in einer Datei auf dem NAS (Abschnitt 14).

## 9. Randfälle

| Fall | Verhalten |
|---|---|
| Abo wird gelöscht, während Folgen laden | Schritt 6 prüft unter der Sperre; der Auftrag entfernt seine Teildatei. Dateien bleiben im Ordner. Kommt später eine neue Datei in den Ordner, legt der Scan den Podcast aus `metadata.json` wieder an (heutiges Verhalten). |
| Dasselbe Abo wird neu abonniert | Die alten Dateien hängen an gelöschten Episoden. Riegel 1 findet sie über `media_files`; nichts wird doppelt geladen, die Folgen sind «unklar», bis ein Scan sie zuordnet. |
| Adresse der Enclosure ändert sich nach dem Download | Nichts; die Datei bleibt. |
| Der Feed ändert eine Folge nach dem Download (neuer Ton, neuer Titel) | Texte folgen dem Feed wie heute; die Datei und ihr Name bleiben. Kein erneuter Download. |
| Datenträger voll mitten im Download | Fehlschlag `no_space`; die Teildatei bleibt für die Wiederaufnahme, der Sweep räumt sie nach sieben Tagen. |
| Wiederherstellung aus einer Sicherung, Dateien sind neuer als die Datenbank | Der Scan findet die Dateien als Datei-Episoden; das Verheiraten und Riegel 1 ordnen sie zu. Namen mit vorangestelltem Datum trifft der Schlüssel nicht; solche Folgen werden «unklar» (Riegel 3). |
| Folge wird explizit oder die Freigabe ändert sich | Kein Einfluss auf den Download; die Sichtbarkeit regelt wie heute, wer sie hört. |
| Zwei Podcast-Bibliotheken, je «Gleichzeitig laden» 3 | Zusammen höchstens fünf (Pool). |

## 10. Datenmodell (Migration 00044, vorläufig)

Nach Vorlage (`server/migrations/TEMPLATE.sql.txt`), nur vorwärts.

| Wo | Was |
|---|---|
| `libraries` | `podcast_mode` (`'save'`, `'link'`; Standard `'link'`), `podcast_scope` (`'new'`, `'last_n'`, `'all'`; Standard `'new'`), `podcast_last_n` (1 bis 100, Standard 10), `podcast_parallel` (1 bis 5, Standard 2), `podcast_root_id` (`REFERENCES library_roots(id) ON DELETE SET NULL`, Standard `NULL` = erster Podcast-Ordner). Per `ALTER TABLE … ADD COLUMN`, wie die Trickplay-Spalten in 00042. Wechselt der Inhalt einer Wurzel weg von `podcast`, wird der Verweis geleert. |
| neu `podcast_settings` | `item_id` (Podcast, Primärschlüssel, `ON DELETE CASCADE`), `mode`, `scope`, `last_n` (alle nullable: `NULL` = wie Bibliothek), `pending_folder` (nur zwischen dem Anlegen eines neuen Ordners und der ersten eingetragenen Datei). |
| neu `podcast_download_failures` | `item_id` (Episode, Primärschlüssel, `ON DELETE CASCADE`), `error` (Code, nie eine Adresse), `attempts`, `first_failed_at`, `last_failed_at`. Eine Zeile gibt es nur nach einem Fehlschlag; ein Erfolg löscht sie. |

Alles andere wird **abgeleitet, nicht gespeichert**: «gespeichert» aus `media_files`, «entfernt» aus
Zeilen, die alle fehlen, «wartet» und «lädt» aus dem Auftrag mit dem Schlüssel
`podcast-dl:<episodeId>`, der Ordner eines Podcasts aus seiner jüngsten vorhandenen Datei. So geht
nichts auseinander, wenn Marco einen Ordner umbenennt oder das Jobsystem alte Aufträge wegräumt.

Die wirksame Einstellung liefert eine gemeinsame Abfrage (5.1). Ein eingefrorener Stand kommt nach
`testdata/db/` (Regel 5).

## 11. Vertrag (1.24.0 und 1.25.0, additiv)

Jede Operation trägt `x-visibility: admin` und bekommt ihren Eintrag im Vertragstest (Regel 7).

| Stufe | Änderung |
|---|---|
| 1 | `Library.podcastDownloads` und `LibraryUpdate.podcastDownloads` (Schema `LibraryPodcastDownloads`; nur Art Podcasts, sonst 422 `not_allowed`): `mode`, `scope`, `lastN`, `rootId`, `parallel`. Muster wie `Library.trickplay`. Der Name `PodcastEpisodes` ist schon vergeben (Zuschauer-Liste). |
| 1 | `Library.podcastFolder`, nur in `getLibrary` (dort läuft die Ordnerprüfung): `rootId`, `path`, `writable`, `problem` (`read_only`, `not_mounted`, `no_hard_links`, `permissions`), `freeBytes`. |
| 1 | `AdminPodcast` wächst um `mode`, `scope` (je `inherit` oder Wert), `lastN`, `effectiveMode`, `effectiveScope`, `effectiveLastN`, `storedBytes`, `downloads` (`queued`, `running`, `failed`), `unclearEpisodeCount`, `latestEpisodeAt`, `savable` (false: Ordner nur lesbar oder Podcast ohne Feed). Die Zahl der Folgen ohne Datei bleibt `episodeCount` minus `localEpisodeCount`; `rootId` bleibt. |
| 1 | neu `updatePodcast` (`PATCH /admin/podcasts/{id}`): `mode`, `scope`, `lastN`. Bedient von `metadata` (dem das Abo gehört), das danach den Planer-Haken ruft. |
| 1 | neu `downloadPodcastEpisodes` (`POST /admin/podcasts/{id}/download`): alle Folgen ohne Datei oder eine Liste von Episoden; 202. |
| 1 | neu `estimatePodcastDownloads` (`POST /admin/libraries/{id}/podcast-downloads/estimate`): nimmt eine gedachte Einstellung (Bibliothek oder ein Podcast) und liefert Anzahl Folgen und Bytes, die sie einmalig nachladen würde (aus `enclosure_length`, sonst dem Mittel der Dateien des Podcasts), dazu den freien Platz. Ändert nichts. Für die Bestätigung im Band und im Dialog «Feed abonnieren». |
| 2 | neu `listAdminPodcastEpisodes` (`GET /admin/podcasts/{id}/episodes`, seitenweise): Schema `AdminPodcastEpisode` mit Titel, Datum, Dauer, Grösse, `state` (`file`, `link`, `queued`, `running`, `failed`, `removed`, `unclear`), Fortschritt, Fehlercode, Dateiname. Eigenes Schema, kein `ItemRef` (das wäre `scoped`). |
| 2 | neu `cancelPodcastDownload` (`DELETE /admin/podcasts/{id}/episodes/{episodeId}/download`). |
| 2 | neu `listPodcastDownloads` (`GET /admin/podcasts/downloads`): die Warteschlange für den Abschnitt «Downloads». |

Das Cover in der Liste kommt über den bestehenden `getAdminItemImage`
(`/admin/items/{id}/images/…`), nicht über den Bild-Endpunkt der Zuschauer; der würde Podcasts ab 18
ausblenden. Neue Fehlercodes: `root_not_writable`, `root_required` (Bibliothek ohne Podcast-Ordner).

## 12. Oberfläche

Mockups: private Leinwand «Konzept 19 Podcasts»
(`https://claude.ai/artifact/CL7J1ag3XYU5YftBd8avyA`), Kopie der Artboards in
`docs/planning/mockups/19-podcasts/`. Alles ist klickbar; die Leiste «Mockup» oben in jedem Bild
springt zwischen den Varianten. Gezeigt sind 14 von 94 Podcasts, Zahlen und Cover sind Platzhalter.

### 12.1 Was neu zu sehen ist

Cover, «Speichert» oder «Nur Link» je Podcast (mit Kennzeichen, wenn er von der Bibliothek
abweicht), gespeicherte und fehlende Folgen, belegter Platz, neueste Folge, Zustand (aktuell, lädt,
Feed-Fehler, nicht speicherbar).

### 12.2 Abonnierte Podcasts (F-5)

| Variante | Idee | Stärke | Schwäche |
|---|---|---|---|
| **A Liste mit Covern** (`Main`) | Eine Zeile je Podcast mit Cover, Kennzahlen oben, Filter «Speichert / Nur Link / Lädt / Fehler», Suche, Sortierung. Aufgeklappt: Einstellung, Feed, Knöpfe. | Alles Wichtige auf einen Blick, Einstellung ohne Seitenwechsel, läuft auch am Handy. | Bei 94 Podcasts lang; Mehrfachänderung nur einzeln. |
| B Raster mit Seitenfeld (`AbosB`) | Cover-Kacheln, rechts das Feld des gewählten Podcasts. | Schön, schnell zu überblicken. | Wenig Zahlen je Kachel; das Seitenfeld rutscht am Handy unter das Raster. |
| C Tabelle mit Mehrfachauswahl (`AbosC`) | Wie heute, dazu Cover, Auswahl in der Zeile, Häkchen und Sammelaktionen. | Am dichtesten. | Am wenigsten «schöner»; am Handy seitlich scrollen. |

**Gewählt: A** (Marco, 9. November 2026). B und C werden nicht gebaut. Das Ausklappfeld von A ist
das heutige `PodcastDetail`, erweitert; was Stufe 1 dort baut, bleibt in Stufe 2 stehen.

Ein Klick führt zum **einzelnen Podcast** (`Podcast`): eigene Seite mit Cover, Einstellung und der
Folgenliste. Je Folge steht «Gespeichert», «Nur Link», «Lädt», «Fehlgeschlagen», «Entfernt»,
«Unklar» mit «Laden», «Abbrechen», «Erneut»; oben «N fehlende nachladen».

### 12.3 Die Einstellung (F-6)

| Variante | Idee | Stärke | Schwäche |
|---|---|---|---|
| A Dialog in der Bibliothek (`Einstellung`) | Wie «Vorschau beim Spulen»: unter Bibliotheken, mit Erklärung, Schätzung und Knopf «Speichern». | Gleiches Muster wie Trickplay. | Ein Seitenwechsel. |
| **B Band auf der Podcasts-Seite** (`EinstellungB`) | Ein Band zuoberst, zugeklappt eine Zeile Zusammenfassung, aufgeklappt Knöpfe. Darunter der Abschnitt «Downloads». | Einstellung und Wirkung auf einer Seite. | Wenig Platz für Erklärung; «gilt sofort» wäre bei «Alle Folgen» heikel. |

**Gewählt: B** (Marco, 9. November 2026; empfohlen war A). Der Dialog in der Bibliothek wird nicht
gebaut. Damit B die Schwäche «gilt sofort» nicht hat, gilt:

- **Kleine Änderungen gelten sofort** («Gleichzeitig laden», N kleiner, Umfang kleiner, «Speichern»
  → «Nur Link»), mit der Rückmeldung «Gespeichert».
- **Alles, was einmalig nachlädt, fragt zuerst nach:** «Nur Link» → «Speichern» mit Umfang «Alle
  Folgen» oder «Neue + letzte N», Umfang grösser, N grösser. Das Band zeigt eine Zeile mit der
  Schätzung (Anzahl Folgen, Platz, freier Platz; `estimatePodcastDownloads`) und den Knöpfen «Laden»
  und «Abbrechen». Ohne Bestätigung ändert sich nichts. Dasselbe gilt für die Einstellung am
  einzelnen Podcast und für «Feed abonnieren» in einer Bibliothek, die nachlädt.
- **Je Bibliothek ein Band.** Die Seite hat heute Reiter je Podcast-Bibliothek; das Band gehört zum
  gewählten Reiter. Mit einer einzigen Podcast-Bibliothek steht es immer da, im Reiter «Alle» mit
  mehreren Bibliotheken steht je Bibliothek eine Zeile.
- **Der Ordner steht im Band** mit dem Ergebnis der Ordnerprüfung («beschreibbar, 62 TB frei» oder
  der Grund, warum nicht). Besteht sie nicht, ist «Speichern» gesperrt.
- Darunter der Abschnitt **«Downloads»** mit Warteschlange, «Anhalten» und «Abbrechen» je Folge.

### 12.4 Regeln

Die neue Seite «Podcast» bekommt einen Playwright-Tastaturtest, die umgebaute Seite «Podcasts»
behält ihren und wächst um Band und Bestätigung (Regel 8). Farben nur aus Tokens, Texte nur aus
`locales/{de,en}.json` (Regel 9). Bausteine aus `web/src/ui`: `AdminPage`, `BandPanel`,
`DataTable`, `Chip`, `RadioGroup`, `ProgressBar`, `StorageMeter`. Neu in `web/src/ui` ist ein
quadratisches Cover für Verwaltungslisten, mit Story und Storybook-Aufnahme (heute gibt es
`PosterTitle` im Hochformat unter `features/admin` und `StationLogo`).

## 13. Betrieb: der Ordner wird schreibbar

Heute bindet `deploy/compose.yaml` alle Medien mit `:ro` ein. Das bleibt so. Neu gibt es eine
**freiwillige Zusatzdatei** `deploy/compose.podcasts.yaml`, die nur den Podcast-Ordner schreibbar
über den read-only Baum legt:

```yaml
services:
  slouchcat:
    volumes:
      # Podcast folder only: writable, so new episodes can be stored (concept 19, ADR 0029).
      - ${PODCAST_DIR:?}:${PODCAST_TARGET:?}
```

Eingeschaltet wird sie in `.env`, nach dem Muster der GPU-Dateien:

```
COMPOSE_FILE=compose.yaml:compose.podcasts.yaml
PODCAST_DIR=/srv/media/Audiobookshelf/Podcast
PODCAST_TARGET=/media/Audiobookshelf/Podcast
```

Zwei Bedingungen, die `docs/INSTALLATION.md` nennt:

- `PODCAST_TARGET` muss **genau der Pfad sein, unter dem der Ordner in Slouchcat schon eingetragen
  ist**. Ein anderer Pfad wäre ein Wurzelwechsel (ADR 0023) und müsste alle Dateien neu verknüpfen.
- Der Zielordner muss im read-only Baum schon bestehen; Docker kann darin keinen Einhängepunkt
  anlegen, und der Container startet sonst nicht.

Eine feste Zeile in `compose.yaml` wäre falsch: sie gälte für alle, und der Updater ersetzt
`compose.yaml` bei jedem Update. Die Zusatzdatei und `.env` bleiben über Updates erhalten. Ohne sie
bleibt alles read-only, und «Speichern» ist in der Verwaltung gesperrt.

Für CT 190 heisst das drei Dinge, die Marco selbst macht: der NFS-Export des NAS erlaubt Schreiben
für diesen Pfad, der Mount im LXC ist nicht `ro`, und die Benutzer-Zuordnung des unprivilegierten
Containers darf im Ordner Dateien anlegen (6.4). Die Ordnerprüfung (5.4) sagt sofort, ob es stimmt.

## 14. Sicherheit

| Gefahr | Riegel |
|---|---|
| Der Server schreibt ausserhalb des Podcast-Ordners. | Schreiben nur über `os.Root` der Wurzel (`paths.Open`), nur in Wurzeln mit `content = 'podcast'`; Namen bereinigt (6.3). |
| Der Server schreibt auf die lokale Platte, weil das NAS nicht eingebunden ist. | Ordnerprüfung: Dateisystemtyp und bekannter Ordner (5.4); bekannte Ordner werden nie neu angelegt. |
| Eine Datei wird überschrieben oder gelöscht. | Harter Link statt Umbenennen (scheitert, wenn es den Namen gibt); der Server löscht in der Wurzel nur seine eigenen `.slouchcat-*`-Dateien. |
| Ein Feed lenkt den Download ins Heimnetz. | Netzwächter wie heute: `netguard.Clients.For(feedURL, feedOrigin)`; ein Feed aus einer Datei (`feed_origin = 'file'`) bekommt nie die LAN-Ausnahme. |
| Ein Feed füllt die Platte. | 2 GiB je Datei, Platzschwelle vor jedem Download, höchstens 2'000 Folgen je Feed, Schätzung und Bestätigung vor jedem Nachladen, ein neues Abo lädt bei «Nur neue Folgen» nichts. |
| Etwas anderes als Audio landet im Ordner. | Sonde vor dem Eintragen (6.2, Schritt 4); was durchfällt, wird als eigene Teildatei entfernt. |
| Zugangsdaten eines privaten Feeds im Log. | Nutzlast des Auftrags nur `{episodeId}`; Fortschrittsnotiz und Fehlercode nennen nie eine Adresse; Fehlertexte nur Schema und Host (`netguard.Clean`). Ein Test mit einem erfundenen Schlüssel in der Adresse hält es fest (Regel 10). |
| Zugangsdaten eines privaten Feeds auf dem NAS. | `metadata.json` enthält die Feed-Adresse (nötig, damit ein Scan das Abo wiedererkennt). Wer den Ordner lesen darf, sieht sie. Das ist bei Audiobookshelf heute schon so; es steht in der Installationsanleitung. |

## 15. Bauplan

| Stufe | Inhalt | Release | Aufwand |
|---|---|---|---|
| **0** | Audiobookshelfs Namensregel nachlesen, Messauftrag M-1 (Abschnitt 8) auf CT 190, Ergebnis nach `docs/perf/19-podcasts.md`. ADR 0029. | – | 1 Tag |
| **1 «Speichern»** | Paket `internal/fetch` (aus `offline` herausgelöst), Migration, gemeinsame Abfrage der wirksamen Einstellung, Haken `WithPodcastFetched`, Planer, `podcast.sync_all`, Download mit Pool `podcast` und den zwei Zählern, Ordnerprüfung, die drei Riegel, Vertrag 1.24.0. Oberfläche: Band «Neue Folgen» mit Schätzung und Bestätigung auf der heutigen Podcasts-Seite, im Ausklapp (`PodcastDetail`) die Wahl je Podcast und «Fehlende nachladen», Spalte «gespeichert». `compose.podcasts.yaml` und Installationsanleitung. | 1.24.0 | 8 bis 9 Tage |
| **2 «Neue Podcasts-Seite»** | Liste mit Covern (Variante A), Seite «Podcast» mit Folgenliste, Abschnitt «Downloads», quadratisches Cover in `web/src/ui`, Vertrag 1.25.0. | 1.25.0 | 4 bis 5 Tage |

Stufe 1 ist für sich nützlich: nach ihr speichert Slouchcat Folgen, und Audiobookshelf kann die
Downloads abgeben. Jeder Schritt ist ein eigener Commit; geprüft wird nach den Teststufen in
`CLAUDE.md`.

## 16. Abnahme und Tests

| Nr. | Prüfung | Stufe der Prüfung |
|---|---|---|
| P-1 | Bibliothek auf «Speichern», Umfang «Nur neue»: eine neue Folge im Feed liegt nach dem Abruf als Datei im Ordner des Podcasts, benannt nach dem Folgentitel; die Folge spielt die Datei. | Go-Integration (Feed- und Dateiserver mit `httptest`, Podcast-Ordner als Testwurzel), e2e |
| P-2 | Dieselbe Folge erscheint genau einmal; kein Eintrag `file_and_feed`; ein Scan während des Downloads erzeugt keine zweite Episode und keine Meldung. | Go-Integration (Scan und Download gegeneinander) |
| P-3 | Ein Podcast auf «Nur Link» in einer Bibliothek auf «Speichern»: neue Folge bleibt Link, keine Datei entsteht. | Go (gemeinsame Abfrage, Planer) |
| P-4 | «Neue + letzte 3» bestätigen: genau die drei jüngsten Folgen ohne Datei werden geladen. | Go (Planer) |
| P-5 | «Alle Folgen»: eine von Hand gelöschte Datei wird nicht wieder geladen («entfernt»), eine nie geladene Folge schon. Mit «Gleichzeitig laden» 5 und fünf verschiedenen Hosts laufen fünf Downloads, bei einem Host nur einer. | Go (Planer, Zähler) |
| P-6 | Ordner read-only, nicht eingebunden oder ohne harte Links: «Speichern» ist gesperrt, das Band nennt den Grund; nach dem Beheben besteht die Prüfung. Nichts wird auf die lokale Platte geschrieben. | Go (Ordnerprüfung mit Testverzeichnissen), Vitest (Band) |
| P-7 | Download bricht mitten im Körper ab: die Folge bleibt als Link abspielbar, im Ordner liegt keine sichtbare Teildatei, der nächste Versuch setzt fort. | Go (`httptest`-Server, der abbricht) |
| P-8 | Eine vorhandene Datei, deren Name dem Folgentitel nach Schlüssel gleicht: es wird nicht geladen, sondern zugeordnet. Zwei Kandidaten oder eine Datei innert 24 Stunden: «unklar», kein Download. | Go (Riegel, Ordner-Fixture) |
| P-9 | Abo ohne Ordner: der erste Download legt Ordner, `metadata.json` und `cover.jpg` an; ein Scan danach erzeugt keinen zweiten Podcast. | Go-Integration |
| P-10 | «Speichern» → «Nur Link»: wartende Downloads verschwinden, alle Dateien bleiben. | Go |
| P-11 | Während 200 Folgen nachladen, startet ein Geräte-Download ohne Wartezeit. | Go (zwei Pools) |
| P-12 | Tastatur: die Seiten «Podcasts» und «Podcast» sind mit Pfeiltasten, Tab, Enter und Zurück bedienbar, Band und Bestätigung eingeschlossen. | Playwright |
| P-13 | Umstellen auf «Alle Folgen» zeigt zuerst die Schätzung; nach «Abbrechen» gilt der alte Umfang und nichts wird geladen, nach «Laden» beginnt das Nachladen. | Vitest, e2e |
| P-14 | Ein neues Abo in einer Bibliothek auf «Speichern / Nur neue Folgen» lädt aus dem ersten Abruf nichts. | Go (Haken) |
| P-15 | Eine Antwort, die kein Audio ist (HTML mit Status 200): keine Datei im Ordner, die Folge bleibt Link, Zustand «Fehlgeschlagen». | Go |
| P-16 | Eine fremde Datei mit dem Zielnamen entsteht kurz vor dem Eintragen: sie bleibt unverändert, die Folge bekommt den nächsten Namen. | Go |
| P-17 | Keine Enclosure-Adresse in Jobnotiz, Fehlercode oder Log. | Go (Redaktionstest) |
| P-18 | Der Geräte-Download verhält sich nach dem Umzug nach `internal/fetch` wie vorher. | bestehende Tests von `offline` |
| P-19 | Tags (6.5): `mp3` und `m4a` tragen Titel, Album und Datum; `ogg` bleibt unverändert; scheitert die Sonde nach dem Taggen, wird die ungetaggte Datei eingetragen. | Go (`transcode` mit Testdateien) |

Eingefrorener Datenbankstand nach der Migration (`testdata/db/`), Vertragstest für jede neue
Operation, `task storybook:test` für das Cover.

## 17. Offene Punkte

| Nr. | Punkt | Empfehlung |
|---|---|---|
| O-1 | ~~Lesart von «Neue und fehlende Folgen»~~ | **erledigt** (9. November 2026): Marco hat den Wert gestrichen |
| O-2 | ~~Stand nach dem Update~~ | **beantwortet** (10. November 2026): «Nur Link», bis Marco umstellt |
| O-3 | ~~Formate, die der Scanner nicht liest~~ | **beantwortet** (10. November 2026): Link lassen, mit Hinweis «Format wird nicht gespeichert»; Video-Podcasts sind ein eigenes Thema |
| O-4 | ~~«Gleichzeitig laden» als Einstellung oder fest?~~ | **erledigt** (9. November 2026): Einstellung, 1 bis 5 |
| O-5 | ~~Neue Folge sofort melden oder erst mit Datei?~~ | **beantwortet** (10. November 2026): sofort beim Feed-Abruf |
| O-6 | ~~Bandbreite begrenzen?~~ | **beantwortet** (10. November 2026): nein |
| O-7 | ~~Tags in die Datei schreiben?~~ | **beantwortet** (10. November 2026): **ja**, gegen die Empfehlung; Umsetzung in 6.5 |
| O-8 | ~~Schlägt die Sonde einer geladenen Datei fehl: Datei löschen?~~ | **erledigt** (Review, Befund 3): die Sonde läuft vor dem Eintragen auf der eigenen Teildatei; was durchfällt, kommt nie in den Ordner |
| O-9 | ~~Mehrfachauswahl (Variante C) zusätzlich zu A?~~ | **erledigt** (9. November 2026): Marco hat A gewählt; Mehrfachauswahl erst bei Bedarf |
| O-10 | ~~Welcher Host zählt für «je Anbieter einer»?~~ | **beantwortet** (10. November 2026): der Host vor der Weiterleitung (der Zähldienst) |
| O-11 | ~~Hört Marco weiter mit Audiobookshelf?~~ | **beantwortet** (10. November 2026): nein, nur noch Slouchcat; die Rechte 0664 und 0775 bleiben trotzdem |
| O-12 | ~~Ohne harte Links «Speichern» gesperrt?~~ | **beantwortet** (10. November 2026): ja, gesperrt |

## 18. Kritiker-Review vom 9. November 2026

Geprüft wurde Fassung 2 (`4b4b0a0a`) gegen den Code. Urteil **REVISE**: 0 Blocker, 13 Major,
6 Minor. Die Richtung trägt (Datei an der Feed-Episode, Scanner unverändert, «Nur Link» ab Werk,
eigener Pool, Importrichtung). Alle Befunde sind eingearbeitet:

| Nr. | Schwere | Befund | Wo eingearbeitet |
|---|---|---|---|
| 1 | Major | Kein Code hätte den Zustand «entfernt» je geschrieben; nicht der Sweep, sondern der Scanner markiert fehlende Dateien. | «entfernt» wird abgeleitet (6.1, 8, 10); Befund in 4 richtiggestellt |
| 2 | Major | `ratelimit` begrenzt keine Gleichzeitigkeit; «Gleichzeitig laden» je Bibliothek bei einem gemeinsamen Pool war unbestimmt. | zwei Zähler in `library`, Host vor Weiterleitung (6.2), O-10 |
| 3 | Major | «Bleibt in jedem Fall als Link abspielbar» stimmt nicht mehr, sobald eine Datei eingetragen ist; ein Sondenfehler ändert das nicht. | Sonde vor dem Eintragen (6.2, Schritt 4), Satz berichtigt, O-8 erledigt |
| 4 | Minor | Kleinere Sachfehler (Trigger und `available`, Merkmale des Scanners, Ort von `PosterTitle`, Herkunft von `Windowable`, toter Verweis). | 4, 6.2, 12.4 |
| 5 | Major | «Neu» liess sich nachträglich nicht berechnen; beim ersten Abruf gälte der ganze Feed als neu. | Haken `WithPodcastFetched` mit den Ids, `Subscribe` meldet keine (5.2), `save_since` entfällt, P-14 |
| 6 | Major | Sperre und erneute Prüfung beim Eintragen waren unbestimmt; Wettlauf mit dem Verheiraten einer Audiobookshelf-Datei. | 6.2, Schritt 6 |
| 7 | Major | Umbenennen ersetzt unter Linux das Ziel still. | harter Link (6.2, Schritt 6), Ordnerprüfung (5.4), O-12, P-16 |
| 8 | Major | Die Namensprüfung übersah `UNIQUE (root_id, rel_path)`; «Laden» einer entfernten Folge wäre gescheitert. | Name frei auf Platte und in der Datenbank, fehlende Zeile wiederbeleben (6.2) |
| 9 | Major | Bei nicht eingebundenem NAS schriebe der Server auf die lokale Platte. | Ordnerprüfung (5.4), bekannte Ordner nie neu anlegen (6.2, Schritt 2) |
| 10 | Major | Die Bereinigung gleicht nicht der von Audiobookshelf; der Fall «kein Kandidat» war nicht abgedeckt. | Schlüssel, alle Zeilen des Ordners, dritter Riegel, M-1 erweitert (8) |
| 11 | Major | «Nur `library` liest die Tabellen» widersprach dem Ort, an dem `AdminPodcast` entsteht. | gemeinsame Abfrage in SQL, `updatePodcast` bei `metadata` (5.1, 11) |
| 12 | Major | Der erneute Versuch alle 6 Stunden hatte keinen Auslöser; Ordner- und Platzfehler waren widersprüchlich. | `podcast.sync_all`, `RetryLater` 30 Minuten bis 24 Stunden (6.1, 6.2) |
| 13 | Major | Die Compose-Zeile war nicht freiwillig, hätte den Start verhindern können und wäre vom Updater überschrieben worden. | Zusatzdatei `compose.podcasts.yaml`, Ziel gleich Wurzelpfad (13) |
| 14 | Major | Dateirechte und Besitzer waren unbestimmt. | 6.4, O-11 |
| 15 | Major | Die Schätzung fehlte im Vertrag. | `estimatePodcastDownloads` (11) |
| 16 | Minor | Fremdschlüssel ohne Löschregel, doppelt gespeicherter Ordner und Jobzustand. | `ON DELETE SET NULL`, nur Fehlschläge gespeichert, Ordner abgeleitet (10) |
| 17 | Minor | Vertragsdetails (`lastN`, Namenskollision `PodcastEpisodes`, Cover über den Zuschauer-Endpunkt). | 11 |
| 18 | Minor | Regel 10: Nutzlast, Notizen, Prüfdatei, `metadata.json` mit privater Adresse. | 6.1, 6.2, 14, P-17 |
| 19 | Minor | Storybook für das Cover, Wegwerfarbeit in Stufe 1, kein Testplan je Stufe. | 12.2, 12.4, 16 |

Dazu die Lücken aus dem Review als Randfälle (Abschnitt 9) und die vier Mehrdeutigkeiten
(Veröffentlichungsdatum im Namen, «neu» bei einem neuen Abo, zwei Bibliotheken und der Pool, Host
vor der Weiterleitung), alle im Text entschieden.

# Teil B: Radio-Angebot und Sender-Metadaten

Teil B gehört zu Konzept 19, weil Marco Podcast und Radio in einem Konzept will (10. November 2026).
Er ist vom Teil A unabhängig: eigene Stufen, eigener Zweig, eigene Releases (B12). **Fassung 2 des
Teils** (10. November 2026), Kritiker-Review eingearbeitet (B15).

## B1. Was Marco will

| Nr. | Marcos Wunsch (10. November 2026, sinngemäss) | Abschnitt |
|---|---|---|
| R-1 | Keine Zuordnung von Hand. Der Reiter «Einordnung» entfällt. | B5, B9 |
| R-2 | Wählen, welche Sender es gibt: je Land, und im Land einzelne Sender markieren und demarkieren, mit «Alle markieren». | B4, B9 |
| R-3 | Nicht erreichbare Sender löschen können. | B6 |
| R-4 | Sprachunabhängig: Slouchcat wird weltweit angeboten; keine Vorgaben wie «Deutschsprachig» oder «Nachbarländer». | B4 |
| R-5 | Den Anbieter eines Senders ermitteln. | B7.4 |
| R-6 | Mehr Metadaten aus weiteren Quellen, vom Server selbst geholt, **aber erst nach dem Hinzufügen, vorher nicht**. | B7 |
| R-7 | Einfacher und übersichtlicher. Die Reiter «Einstellungen» und «Eigene Sender» passen. | B9 |

## B2. Entscheide aus den Mockups (10. November 2026)

Marco hat die Radio-Verwaltung über eine klickbare Leinwand entschieden (Anhang B). Die Reihenfolge:

| Nr. | Vorlage | Marcos Entscheid |
|---|---|---|
| RF-1 | Drei Wege, das Angebot zu wählen: A Pakete, B Länder zuerst, C Regeln. | Vorgaben für Sprache oder Nachbarländer **streichen**, «stattdessen ein Sprachenfilter». Gemeint war der Filter in Paket A; mit B und RF-7 entfällt er dort. Der Sprachenfilter der App (`language`) bleibt unverändert. |
| RF-2 | Auch das Angebot wählen ist «eine Flut an Kanälen»; ganz anders denken. | Mockup «Meine Sender statt Katalog» bestellt, danach aber **B (Länder zuerst) «eigentlich gut»**. |
| RF-3 | B mit nur einigen Ländern. | **Alle Länder**, ohne dass es unübersichtlich wird: Suche, gewählte Länder, Kontinente. |
| RF-4 | Kachelblock «Die 30 grössten». | **Streichen.** |
| RF-5 | Nur ganze Länder. | **Je Land einzelne Sender markieren und demarkieren**, dazu «Alle markieren». |
| RF-6 | Reihenfolge. | **Zuerst Kontinent und Länder, danach die Sender**, die Sender **mit echtem Logo**. |
| RF-7 | Schritte «Aus diesen Ländern eingrenzen» (Genre, Sprache) und «Zusätzlich aus aller Welt». | **Beide streichen.** |
| RF-8 | Wann der Server Metadaten holt. | **Erst nach dem Hinzufügen, vorher nicht.** |
| RF-9 | Ein Suchfeld «Sender suchen» über alle Länder. | **Nein.** Einzelne Sender gehen über das Land und die Suche in dessen Senderliste. |
| RF-10 | Lage der Meldung «Übernommen». | **Ganz unten**, zusammen mit «Übernehmen» und «Verwerfen». |
| RF-11 | Abschluss. | «Sieht gut aus, nimm das so ins Konzept.» |

Nicht gewählt: Angebot A (Pakete), Angebot C (Regeln), «Meine Sender statt Katalog» (das Angebot
entstünde aus dem, was Profile über die Suche hinzufügen).

**Spannung RF-6 und RF-8:** Logos gibt es in der Senderliste der Verwaltung nur, wo der Server schon
eines hat (B7.1). Vor dem ersten Hinzufügen eines Landes stehen dort meist Initialen; das Mockup zeigt
genau das.

## B3. Was sich gegenüber früheren Entscheiden ändert

| Bisher | Neu | Grund |
|---|---|---|
| Frage 23, F-2: «ganzer Katalog von Radio Browser, täglich abgeglichen; Filter und Einordnung entstehen automatisch über Regeln «Schlagwort → Filterwert», der Admin pflegt nur Ausnahmen.» Die App zeigt den ganzen Katalog (Konzept 16, 5.2). | Der ganze Katalog wird weiter täglich abgeglichen, aber **nur noch als Auswahlliste der Verwaltung**. Die App zeigt nur das **Angebot** (B4). | R-2, RF-2 bis RF-5 |
| Konzept 16, 5.4, 5.5 und Stufe 2: Der Admin ordnet auf «Einordnung» Schlagworte zu (`radio_tag_rules`) und setzt je Sender Genre, Sendeart und Stimmung mit Schloss, auch beim Anlegen eines eigenen Senders. | **Keine Zuordnung von Hand** (B5). Die Filter der App entstehen nur noch aus der mitgelieferten Regeldatei `radio/rules/default.json`. | R-1 |
| Konzept 16, 11: Einstellung «Länder im Katalog (alle oder gewählte)» (`radio_config.countries`); der Länderfilter gilt auch für eigene Sender (`index.go`, `reachable`). | Ersetzt durch den Reiter **«Angebot»**. **Eigene Sender sind immer im Angebot**, gleich welches Land, ausser der Admin nimmt einen heraus. | R-2 |
| Konzept 16, 5.5: «Ausblenden» je Sender (`radio_stations.hidden`). | Heisst **«Aus dem Angebot nehmen»** und ist eine Ausnahme im Angebot (B4.1). | R-2 |
| Konzept 16, 6: Logos holt der Server beim ersten Anzeigen, nach dem Abgleich für Favoriten und Empfohlene. | Logos und alle weiteren Angaben holt er **nur für Sender im Angebot** (B7). | RF-8 |
| Konzept 16, 5.2: «Gestörte Sender» sind in der App ausgeblendet (Schalter, ab Werk an). | Bleibt als Schalter «Nicht erreichbare weglassen», zieht in den Reiter «Angebot». Neu: **«Nicht erreichbare entfernen»** (B6). | R-3 |
| Neue Installation und Update: die App zeigt den ganzen Katalog (oder die gewählten Länder). | Das Angebot ist nach dem Update und auf einer neuen Installation **leer**, bis der Admin Länder wählt (B4.4, Marcos Antwort auf OB-2). | R-2, OB-2 |

Unberührt bleiben ADR 0025 für Weiterleitung, Zeitversatz, Hör-Sitzungen und Hörzeit, die stabile
Kennung der Sender (Konzept 16, 5.3), das Standardland je Profil (F-12), der Sprachenfilter der App
und Regel 12: Was im Angebot ist, entscheidet der Server; der Client fragt nur ab.

## B4. Das Angebot

### B4.1 Regel

Das Angebot ist die Menge der Sender, die die App zeigt. Es besteht aus zwei Teilen:

1. **Gewählte Länder mit Modus:** «Alle» oder «Nur markierte».
2. **Ausnahmen je Sender:** «markiert» oder «demarkiert». Eine Ausnahme hängt am Sender, nicht am
   Land.

Ein Sender ist im Angebot, wenn gilt (der Reihe nach, erste Regel mit Treffer):

| Fall | Im Angebot? |
|---|---|
| Ausnahme «demarkiert» | nein |
| Ausnahme «markiert» | ja, gleich in welchem Land |
| eigener Sender | ja |
| Land gewählt mit «Alle» | ja |
| sonst (Land «Nur markierte», Land nicht gewählt, kein Land) | nein |

Folgen der Regel:

- **Neue Sender** eines Landes mit «Alle» sind nach dem nächsten Abgleich von selbst im Angebot; bei
  «Nur markierte» nicht (**OB-1**).
- **Wechselt ein Sender das Land** (der Abgleich leitet das Land bei jeder Änderung neu ab), folgt
  ihm seine Ausnahme: ein demarkierter bleibt draussen, ein markierter drin. Ohne Ausnahme gilt der
  Modus des neuen Landes.
- **Sender ohne gültiges Land** (rund 1'000) lassen sich in der Verwaltung nicht wählen (es gibt kein
  Land dafür, OB-7); wer einen will, trägt ihn als eigenen Sender ein. Ein solcher Favorit bleibt in der
  Datenbank, erscheint aber nicht.
- **Eigene Sender** zählen nicht in «n von m» eines Landes und erscheinen in der App im Länderfilter,
  wenn sie ein Land haben.
- **Ein Sender, der nach 7 Tagen ohne Katalogeintrag gelöscht wird**, nimmt seine Ausnahme mit
  (`ON DELETE CASCADE`).

**Bedienung** (B9.2), jeweils im Entwurf bis «Übernehmen»:

| Aktion | Wirkung |
|---|---|
| Land wählen | Land mit «Alle» |
| Land entfernen | Land weg, dazu alle Ausnahmen der Sender, die jetzt in diesem Land stehen |
| Alle markieren | Land auf «Alle», Ausnahmen seiner Sender weg |
| Alle demarkieren | Land auf «Nur markierte», Ausnahmen seiner Sender weg |
| Haken an einem Sender | Ausnahme dieses Senders; entspricht sie dem Modus, fällt sie weg |
| Gezeigte markieren / demarkieren (mit Suchtext) | Ausnahme für jeden Sender des Landes, der zur Suche passt; der Server löst die Suche auf (B4.3) |

Beispiel: In der Schweiz `1.fm` suchen und «Gezeigte demarkieren» nimmt alle 77 Kanäle von 1.FM aus
dem Angebot; die Schweiz bleibt auf «Alle».

### B4.2 Wo entschieden wird

Heute liest jede Anfrage der App `radio_config` aus der Datenbank (`config.go`, `view()`), und
`catalogView.reachable` (`index.go`) prüft das Land gegen diese Liste. Für Tausende Ausnahmen taugt
das nicht. Neu:

- Das Angebot liegt als **eigene, unveränderliche Momentaufnahme im Speicher**: Land → Modus,
  Sender → Ausnahme, Schalter «Nicht erreichbare weglassen», Versionsnummer. Geladen beim Start,
  nach jedem «Übernehmen» ganz ersetzt (Zeiger tauschen). Der Senderindex wird dafür **nicht** neu
  gebaut.
- `catalogView` bekommt einen Zeiger auf die Momentaufnahme; `reachable` wendet die Regel aus B4.1
  an. Das ist ein Nachschlagen in zwei Tabellen je Sender.
- Alles, was heute für ein ausgeschlossenes Land gilt, gilt für jeden Sender ausserhalb des Angebots:
  keine Liste, kein Zähler, kein Filter, keine Startseiten-Reihe; Station, Hören, Titelangaben, Bild
  und «Favorit setzen» antworten mit **404** (`listen.go`, `catalog_profile.go`), nie 403. Ein
  Favorit bleibt in der Datenbank und erscheint wieder, sobald der Sender zurück im Angebot ist.
- **Laufendes Hören:** Nach dem Tausch ruft der Server `closeUnreachable` wie heute nach
  `updateRadioConfig` (`config.go`): Sitzungen auf Sendern, die das Angebot verlassen haben, enden
  (`ended_by = 'admin'`), ihr Relay stoppt.
- Die Verwaltung sieht den ganzen Katalog; jeder Sender trägt `offered` (gespeicherter Stand).

### B4.3 Entwurf, Vorschau und Übernehmen

Änderungen sammeln sich im Reiter «Angebot» als **Liste von Schritten** (die Aktionen aus B4.1). Der
Server rechnet, der Client zeigt nur an:

- **Vorschau:** Der Client schickt die Schritte; der Server wendet sie auf den gespeicherten Stand an,
  ohne zu speichern, und liefert die Summen, den Unterschied («4'875 Sender kommen dazu, 77 fallen
  weg»), je Land «n von m» und, auf Wunsch, eine Seite der Senderliste eines Landes mit dem Stand
  im Entwurf. So zeigt die Liste auch nach «Gezeigte markieren» über 300 Treffer den richtigen Haken,
  ohne dass der Client die Regel kennt.
- **Übernehmen:** dieselben Schritte mit der Versionsnummer des Stands, auf dem der Entwurf beruht.
  Stimmt sie nicht mehr (ein zweiter Tab, «Nicht erreichbare entfernen» auf «Sender»), antwortet der
  Server mit **409**; der Client holt den neuen Stand, rechnet die Vorschau mit denselben Schritten
  neu und zeigt sie zum zweiten Übernehmen. Weil die Schritte relativ sind («Land wählen»,
  «diese Sender demarkieren»), gehen dabei weder der Entwurf noch die andere Änderung verloren.
- **Zahlen:** `m` = alle Sender des Landes im Katalog, nicht erreichbare eingeschlossen; `n` = davon
  im Angebot. Die Zusammenfassung nennt zusätzlich «davon nicht erreichbar».
- **Verlassen mit Entwurf:** Frage wie auf anderen Verwaltungsseiten (`ui/useUnsavedChanges`).

### B4.4 Erster Start nach dem Update und neue Installation

**Marcos Entscheid (OB-2, 10. November 2026): Das Angebot ist nach dem Update leer.** Auch eine neue
Installation beginnt leer. Bis der Admin im Reiter «Angebot» Länder wählt, zeigt die Radioseite keinen
Sender; Favoriten bleiben in der Datenbank und erscheinen wieder, sobald ihr Sender im Angebot ist.

Die Migration legt die Tabellen an und setzt ein Merkmal «nach dem Update noch nicht vorbereitet».
Beim ersten Start erledigt der Server im Go-Code, was SQL allein nicht kann:

1. Jeder ausgeblendete Sender (`hidden = 1`), auch ein eigener, wird **demarkiert**. Wählt der Admin
   später sein Land, bleibt er draussen wie heute.
2. Filterwerte aller Sender neu ableiten (B5), Anbieter nachtragen (B7.4), Merkmal löschen.
3. Keine Logos vorholen: im Angebot ist noch nichts.

`radio_config.countries` und `radio_stations.hidden` bleiben in den Tabellen und werden nicht mehr
gelesen (wie der Vorgabewert `'ahead'` aus 00042; kein Neubau der Tabellen). Die Werte aus
`countries` übernimmt das Angebot **nicht**.

**Hinweise:** Die Zustandsleiste der Verwaltung zeigt bei leerem Angebot «Noch keine Sender im
Angebot» mit Link zum Reiter «Angebot»; die Radioseite zeigt den leeren Zustand aus B4.5. Die
Versionshinweise des Releases sagen es deutlich.

### B4.5 Die App

- **Filter «Land»** und der Schnellfilter «Mein Land» zeigen nur Länder mit Sendern im Angebot.
- **Standardland ausserhalb des Angebots** (Marcos Entscheid OB-10): Der Server liefert neben dem
  gespeicherten `defaultCountry` ein `effectiveCountry`: das gespeicherte Land, wenn es Sender im
  Angebot hat, sonst **leer**. Leer heisst: die Radioseite öffnet **ohne Landfilter** mit allen Sendern
  des Angebots, und die Reihe «Beliebt in <Land>» heisst «Beliebt» und gilt für das ganze Angebot. Die
  Seite schreibt nur `defaultCountry` zurück; die gespeicherte Wahl bleibt und gilt wieder, sobald ihr
  Land im Angebot ist.
- **Gespeicherter Filter** (`lastFilter`) mit einem Land ausserhalb des Angebots: Der Server lässt
  dieses Land beim Lesen weg.
- **Leeres Angebot:** `RadioFacets.offerEmpty`; die Radioseite sagt «Noch keine Sender ausgewählt»
  statt «Kein Sender passt zum Filter», für den Admin mit Link zum Reiter «Angebot».
- Genre, Sendeart, Stimmung und Sprache bleiben als Filter, abgeleitet aus der Regeldatei und dem
  Katalog (B5).

## B5. Keine Zuordnung von Hand

- **Entfällt:** Reiter «Einordnung» (`TagRulesTab.tsx`), die Operationen `getRadioTagRules` und
  `setRadioTagRules`, die Felder Genre, Sendeart, Stimmung im Editor (`StationEditor.tsx`) und beim
  Anlegen eines eigenen Senders (`CreateStationDialog.tsx`), ihre Schlösser, die Abdeckung je Filter
  in der Übersicht und die Kachel «Schlagworte» der Zustandsleiste.
- **Bleibt:** die mitgelieferte Regeldatei `radio/rules/default.json` und `deriveFacets` (Schlagworte,
  sonst Wörter im Namen, sonst «Musik»). Die Regeldatei wird im Repository gepflegt wie die
  Sprachdateien.
- **Bestehende Regeln und Handwerte:** Die Migration löscht `radio_tag_rules`, die Zeilen in
  `radio_station_facets` mit `manual = 1` und die Schlösser `genre`, `kind`, `mood` (Empfehlung,
  **OB-4**). Schlösser für Name, Logo, Region und Stream bleiben.
- **Neu ableiten:** Weder der Abgleich (nur geänderte Einträge, `sync.go`) noch `applyRules` (nur
  Sender mit Schlagworten, `tagrules.go`) erreichen alle Sender. Darum leitet der erste Start nach dem
  Update (B4.4, Schritt 5) die Filterwerte **aller** Sender neu ab, auch der eigenen und der Sender
  ohne Schlagworte.

## B6. Nicht erreichbare Sender

«Löschen» kann bei einem Katalogsender nicht heissen, ihn aus dem Katalog zu löschen: der nächste
Abgleich brächte ihn zurück. Darum zwei Wege:

| Weg | Wirkung | Wo |
|---|---|---|
| Schalter **«Nicht erreichbare weglassen»** (ab Werk an, heute `hide_broken`) | Nicht erreichbare Sender im Angebot erscheinen in Listen und Zählern der App nicht. Sie kommen zurück, sobald der Abgleich oder das eigene Abspielen sie wieder als erreichbar führt. Teil des Entwurfs. | Reiter «Angebot», Zusammenfassung |
| Knopf **«Nicht erreichbare entfernen (n)»** | Demarkiert alle zurzeit nicht erreichbaren Sender im Angebot, **ausser Favoriten und eigenen Sendern**. Sie kommen nicht zurück. «Rückgängig» markiert die gelieferten Sender wieder, solange die Meldung steht. Wirkt sofort. | Reiter «Sender», über der Liste |

- «Nicht erreichbar» heisst wie heute `reachable = 0` oder `broken_at` gesetzt (`index.go`).
- Favoriten bleiben wie heute immer sichtbar und abspielbar, auch nicht erreichbare (sie gehen über
  `reachable`, nicht über `listed`; `catalog_profile.go`); die App zeigt den Hinweis «gestört»
  (**OB-6**).
- Einen **eigenen** Sender löscht «Eigene Sender» wie heute (`deleteRadioStation`).

## B7. Metadaten erst nach dem Hinzufügen

### B7.1 Was «Metadaten» hier heisst

Die **Grunddaten des Katalogs** (Name, Land, Region, Sprache, Schlagworte, Erreichbarkeit, Streams)
lädt der Abgleich für alle Sender wie heute; ohne sie gäbe es keine Auswahlliste. **Alles, was der
Server zusätzlich abrufen muss**, holt er nur für Sender im Angebot (Lesart von RF-8, **OB-3**):

| Angabe | Quelle | Stufe |
|---|---|---|
| Logo | eigenes Bild, sonst Logo-Adresse des Katalogs (`favicon`), sonst Webseite des Senders (`apple-touch-icon`, Symbole im Manifest, `og:image`); heutige Kette | B1 |
| Anbieter | Host des Streams, in B2 der Eigentümer aus Wikidata | B1, B2 |
| Eigentümer, Gründung | Wikidata (P127, P571), nur in der Verwaltung | B2 (**OB-5**) |

Wikidata liefert **keine Logos**: Die Dateien auf Wikimedia Commons haben je eigene Lizenzen, viele
verlangen eine Namensnennung, und der Gewinn ist klein (in der Schweiz 53 Logos bei 81 Treffern, fast
alle schon über Katalog oder Webseite abgedeckt; B7.5).

**Senderliste der Verwaltung:** Sie zeigt ein Logo, wenn der Server schon eines im Cache hat (auch von
früher), sonst Initialen wie `StationLogo`. Sie löst nie einen Abruf aus.

### B7.2 Wann der Server Logos holt

Heute reiht `queueLogos` (`logo.go`) Sender ein, wenn eine Liste der App sie zeigt (Haken `onListed`:
Listen, Favoriten, Startseiten-Reihe) und nach dem Abgleich die Favoriten und Empfohlenen
(`queueSyncLogos`); dazu holt `deleteOwnLogo` und das Anlegen eines eigenen Senders das Logo neu
(`own.go`). Neu:

| Auslöser | Heute | Neu |
|---|---|---|
| Listen, Favoriten, Startseite (`onListed`) | jeder gezeigte Sender | unverändert; die App zeigt ohnehin nur das Angebot |
| nach dem Abgleich (`queueSyncLogos`) | Favoriten, Empfohlene | **neu ins Angebot gekommene** Sender (neue Sender in Ländern mit «Alle»), Favoriten, Empfohlene im Angebot |
| eigener Sender angelegt, eigenes Logo gelöscht | ja | unverändert |
| **«Übernehmen»** | – | neu: alle Sender, die dazukamen und kein Logo haben |
| erster Start nach dem Update | – | nichts; das Angebot ist leer (B4.4) |
| Senderliste und Bildroute der Verwaltung | – | nie |

Reihenfolge innerhalb eines Auslösers: Favoriten, dann nach Beliebtheit (`popularity` = Stimmen plus
Klicks im Katalog, `index.go`). Ein Fehlschlag wird nach 30 Tagen wieder versucht (`logoRetryAfter`).

**Drossel.** Heute gilt eine gemeinsame Drossel von **2 Abrufen je Sekunde** für alle Logo-Aufträge
(`logo_state.go`, `ratelimit.New("radio-logo", 2, 2)`). Neu kommt **je Host höchstens ein Abruf
gleichzeitig** dazu (Host der Adresse vor einer Weiterleitung, wie Teil A 6.2):

- Ein Auftrag versucht die Sperre des Hosts; ist sie belegt, gibt er mit `jobs.RetryLater(15 s)`
  zurück, ohne einen Versuch zu verbrauchen, und blockiert keinen Worker. Scans und
  `probe.keyframes` (Priorität 9) finden so immer einen freien Worker.
- Die gemeinsame Drossel steigt auf **4 je Sekunde** (Empfehlung, **OB-11**); die Messung in B0
  prüft, ob das reicht und ob Sender sich beschweren (Antwort 429).

### B7.3 Wikidata (Stufe B2)

- **Ein Abruf für alles:** eine SPARQL-Abfrage an `query.wikidata.org` nach Radiosendern mit
  Webseite (P856), dazu Eigentümer (P127) und Gründung (P571). Gemessen am 10. November 2026 (mit
  Logo P154): 1,5 s, 4,6 MB, 19'229 Sender.
- **Abgleich lokal** über die Webseite (Host ohne `www.` und Pfad); mehrdeutige Treffer zählen nicht.
  Gespeichert wird nur, was zu einem Sender **im Angebot** passt.
- **Jobart `radio.wikidata.sync`:** `TriggerSchedule` mit `Handler`, `Run`, `Every` 7 Tage und
  `Seed`, wie `radio.catalog.sync` (`radio/jobs.go`); «Übernehmen» reiht sie zusätzlich direkt ein
  (Schlüssel `radio.wikidata.sync`), aber nur, wenn neue Sender dazukamen und der letzte Lauf über 24
  Stunden her ist (`radio_config.wikidata_synced_at`). Priorität 4, Pool `general`; Eintrag in der
  Prioritätsleiter (`jobs/queue.go`) zusammen mit `radio.logo.fetch` (0).
- **Netz:** über den Netzwächter, Antwort höchstens 20 MB, User-Agent
  `Slouchcat/<Version> (+https://github.com/SlouchCat/slouchcat)` ohne Kontaktdaten des Admins.
- **Einstellung «Wikidata verwenden»** (ab Werk an), wie die Schalter für MusicBrainz und iTunes.
- **Sprache:** Bezeichnungen in der Sprache der App, sonst Englisch.
- **Lizenz:** Die Daten von Wikidata sind CC0; das steht in ADR 0030.

### B7.4 Anbieter

Ein Sender bekommt die Spalte `provider`:

1. **Stufe B1:** die registrierbare Domain des ersten Streams in `ListRadioStreamsRanked`
   (`url_resolved`, sonst `url`): `srg-ssr.ch`, `streamtheworld.com`, `zeno.fm`, `infomaniak.ch`.
   Berechnet mit `golang.org/x/net/publicsuffix` (das Modul steht schon in `server/go.mod`). Gesetzt
   bei jeder Änderung durch den Abgleich und einmal für alle Sender beim ersten Start (B4.4).
2. **Stufe B2:** der Eigentümer aus Wikidata, wenn ein eindeutiger Treffer da ist (z. B. «Schweizer
   Radio und Fernsehen» für Radio SRF 3).

Gezeigt wird der Anbieter in der Verwaltung (Reiter «Sender», Spalte und Filter) **und in der App**
(Marcos Entscheid OB-8): in der Senderliste und in «Jetzt läuft» unter dem Namen, nach Land und Region
(«Schweiz · srg-ssr.ch»; ab B2 der Eigentümer, «Schweiz · Schweizer Radio und Fernsehen»).
Stichprobe der 8'000 meistgeklickten Katalogeinträge (9. November 2026): `zeno.fm` 494,
`streamtheworld` 436, `infomaniak` 168; von 127 Schweizer Einträgen darin laufen 41 über
`srg-ssr.ch`.

### B7.5 Gemessen (10. November 2026)

Auf dem Arbeitsrechner (nicht CT 190), 452 Schweizer Sender (Radio Browser, nach Namen
zusammengeführt), vier Abrufe gleichzeitig, einer je Host, 8 s Zeitgrenze, Bild ab 64 Pixel, **ohne**
die Drossel von 2 je Sekunde:

| Quelle | Sender mit Logo | Anteil |
|---|---|---|
| Katalog allein | 158 | 35 % |
| Webseite allein | 356 | 79 % |
| Katalog oder Webseite | 379 | 84 % |
| Wikidata, eindeutiger Treffer über die Webseite | 81 Sender, davon 53 mit Logo | 18 % / 12 % |

Aufwand: **86 s, 702 Abrufe, 52,6 MB**; je Webseite Median 0,39 s, 90 % unter 2,43 s. Die Messung
für Deutschland und die USA hat Marco abgebrochen.

**Schätzungen mit Drossel** (nicht gemessen; rund 1,6 Abrufe je Sender):

| Sender | bei 2 je Sekunde (heute) | bei 4 je Sekunde (OB-11) |
|---|---|---|
| 452 (Schweiz) | rund 6 Minuten | rund 3 Minuten |
| 5'000 | rund 65 Minuten | rund 33 Minuten |
| 55'000 (ganzer Katalog) | rund 12 Stunden | rund 6 Stunden |

**Grenze der Messung:** Das Bild einer Webseite ist das Logo der Webseite, nicht des Kanals. Alle
1.FM-Kanäle bekommen dasselbe 1.FM-Logo, die SRF-Sender teilen sich eines. Darum bleibt die
heutige Reihenfolge: Logo des Katalogs vor der Webseite. Stufe B0 misst auf CT 190 mit der echten
Kette (96 Pixel, Drossel, Sperre je Host) für die Schweiz und Deutschland (**OB-9**).

## B8. Leistung

| Teil | Last | Begründung |
|---|---|---|
| Abgleich | unverändert, 21 s auf CT 190 (`docs/perf/radio.md`) | lädt wie heute den ganzen Katalog |
| Angebot prüfen | ein Nachschlagen je Sender in zwei Tabellen im Speicher | B4.2; kein Lesen von `radio_config` je Anfrage mehr |
| Vorschau | ein Durchgang über die rund 55'000 Sender des Index je Anfrage | B0 misst die Zeit auf CT 190; Ziel unter 100 ms |
| Übernehmen | Schreiben der Schritte in einer Transaktion, Momentaufnahme tauschen | kein Neubau des Index |
| Logos | einmalig je Sender im Angebot, gedrosselt | B7.5 |
| Wikidata | ein Abruf von rund 4 MB, höchstens täglich | B7.3 |
| Speicher | heutige Logogrössen (96, 256, 512 Pixel, WebP und PNG) je Sender mit Logo | B0 misst für die Schweiz |

Die Wiedergabe merkt davon nichts: Relays laufen nicht über die Job-Queue, und Logo-Aufträge mit
Priorität 0 blockieren keinen Worker (B7.2).

**Aufräumen (neu):** Heute löscht nichts die Bilder unter `images/radio/<id>`, auch nicht nach dem
Löschen eines Senders. Neu entfernt der Abgleich nach dem Löschen eines Senders dessen Ordner. Bilder
von Sendern ausserhalb des Angebots bleiben (wenige KB je Sender); kommt der Sender zurück, ist das
Logo sofort da.

## B9. Die Verwaltung

### B9.1 Reiter und Zustandsleiste

| Heute | Neu |
|---|---|
| Übersicht | **Übersicht**: Katalog und Abgleich, Angebot in Zahlen (Länder, Sender, nicht erreichbar, mit Logo), Relays und Serverlast. Ohne Abdeckung der Einordnung. |
| – | **Angebot** (neu, B9.2) |
| Sender | **Sender** (B9.3) |
| Einordnung | entfällt |
| Eigene Sender | **Eigene Sender** ohne Genre, Sendeart, Stimmung beim Anlegen; sonst wie heute |
| Einstellungen | **Einstellungen** ohne die Zeile «Länder» und ohne den Schalter für gestörte Sender (beide in «Angebot»); neu «Wikidata verwenden» (B2) |

Zustandsleiste (`RadioAdminPage.tsx`): Die Kachel «Schlagworte» (offene Schlagworte) entfällt; die
Kachel «Geändert» (Ausnahmen inklusive ausgeblendeter) wird zu **«Im Angebot»** (Sender und Länder).

### B9.2 Reiter «Angebot» (Mockup `AngebotB`, gewählt)

Von oben nach unten:

1. **Länder:** Suchfeld «Land suchen» (Name oder Ländercode), darunter die **sechs Kontinente** mit
   Zahl der Länder und Sender. Ein Kontinent klappt seine Länder alphabetisch als schmale Zeilen auf,
   mit «Alle Länder wählen» und «Alle abwählen»; nur einer ist offen, die Liste scrollt in sich.
   Länder heissen in der Sprache der App (wie heute `countryName`).
2. **Sender der gewählten Länder:** Liste «Gewählte Länder» mit je «n von m Sendern», «Sender
   wählen» und «Land entfernen»; darunter die **Senderliste** des geöffneten Landes: Suche, «Alle
   markieren» und «Alle demarkieren» (mit Suchtext «Gezeigte …»), im Kopf der gültige Modus («Neue
   Sender kommen von selbst dazu» oder «… nicht»), je Sender Haken, Logo oder Initialen, Name,
   Schlagworte, Hinweis «nicht erreichbar». Sortiert nach Beliebtheit. Seiten zu 100, virtuell
   (Konzept 18, Teil C), geliefert von der Vorschau (B4.3).
3. **Ganz unten:** Leiste «Übernehmen» und «Verwerfen» mit dem Unterschied aus der Vorschau, nach dem
   Übernehmen die Meldung «Übernommen. Die Radioseite zeigt das neue Angebot sofort. Logos und weitere
   Angaben lädt der Server jetzt im Hintergrund.» (RF-10).

Daneben die **Zusammenfassung**: Sender im Angebot, davon nicht erreichbar, gewählte Länder,
markierte Sender, Schalter «Nicht erreichbare weglassen». Auf schmalen Bildschirmen steht sie unter
den Ländern.

**Kontinente.** Der Server liefert je Land den Kontinent aus einer eingebetteten Tabelle nach den
Unterregionen von UN M49, zusammengefasst zu sechs Gruppen: Europa; Nord- und Mittelamerika (mit
Karibik und Mittelamerika); Südamerika; Asien; Afrika; Ozeanien (mit der Antarktis `AQ`). Kosovo
(`XK`, nicht in M49) zählt zu Europa. Die Namen der Kontinente stehen in den Sprachdateien.

### B9.3 Reiter «Sender»

Liste der Sender **im Angebot** (Vorgabe) oder des ganzen Katalogs (Umschalter), Suche, Filter Land,
Anbieter, Zustand (erreichbar, nicht erreichbar, ohne Logo), sortiert nach Beliebtheit oder Name.
Spalte **Anbieter**. Über der Liste «Nicht erreichbare entfernen (n)» (B6). Je Sender:

- **Aufklappen** zeigt Streams und die Herkunft jeder Angabe (Katalog, Stream, Webseite, ab B2
  Wikidata).
- **Bearbeiten** wie heute für Name, Logo, Region, Stream (mit Schloss), «Stream prüfen»,
  «Empfehlen». Ohne Genre, Sendeart, Stimmung.
- **Aus dem Angebot nehmen** (heute «Ausblenden») und **Ins Angebot nehmen**: Ausnahme, sofort
  gespeichert (eigene Version, B4.3).

### B9.4 Regeln der Seite

- Seitenmuster S-1 bis S-17 (Konzept 12), Texte nur aus `de.json`/`en.json`, Farben nur aus Tokens,
  Bausteine aus `web/src/ui`.
- Tastatur: Pfeiltasten in den Länder- und Senderlisten, Leertaste für den Haken, Enter öffnet,
  Escape schliesst den Kontinent oder die Senderliste; Playwright-Test (Regel 8).

## B10. Datenmodell (Migration 00045, vorläufig)

| Tabelle oder Spalte | Inhalt |
|---|---|
| `radio_offer_countries` | `country` (zwei Grossbuchstaben, Primärschlüssel), `mode` (`all`/`marked`), `updated_at` |
| `radio_offer_exceptions` | `station_id` (Primärschlüssel, Fremdschlüssel auf `radio_stations`, `ON DELETE CASCADE`), `offered` (0/1), `updated_at`; gilt unabhängig vom Land (B4.1) |
| `radio_config.offer_version` | Versionsnummer des Angebots (B4.3) |
| `radio_config.offer_seeded` | 0 nach der Migration, 1 nach der Vorbereitung beim ersten Start (B4.4) |
| `radio_config.hide_broken` | bleibt, gehört jetzt zum Angebot |
| `radio_stations.provider` | Anbieter (B7.4) |
| `radio_config.countries`, `radio_stations.hidden` | bleiben, werden nicht mehr gelesen |
| `radio_tag_rules`, `radio_station_facets` mit `manual = 1`, `radio_station_locks` für `genre`/`kind`/`mood` | Zeilen gelöscht (B5) |

**Stufe B2, Migration 00046 (vorläufig):** `radio_station_wikidata` (`station_id` Primärschlüssel,
`qid`, `owner`, `founded`, `fetched_at`), `radio_config.wikidata_enabled`,
`radio_config.wikidata_synced_at`.

Nach Vorlage (`TEMPLATE.sql.txt`), nur vorwärts. Eingefrorene Stände in `testdata/db/`; die
vistest-Fixture ersetzt `RadioHidden` durch einen demarkierten Sender und `countries` durch
`radio_offer_countries`.

**Zahlen:** 50'073 Sender und 1'049 ohne Land stammen aus dem Katalog vom 9. November 2026, nach
Namen zusammengeführt. Slouchcat gruppiert nach Name, Land und Host und kommt auf CT 190 auf 55'646
Sender aus 63'562 Einträgen (`docs/perf/radio.md`). Die Verwaltung zeigt die Zahlen des Servers.

## B11. Vertrag

Im Repository heisst eine brechende Änderung eine neue Nebenversion mit dem Vermerk «brechend» (wie
1.2.0 und 1.18.0).

**Stufe B1, vorläufig 1.26.0 (brechend für die Verwaltung, additiv für die App):**

| Operation oder Schema | `x-visibility` | Änderung |
|---|---|---|
| `GET /admin/radio/offer` | `admin` | **neu:** gespeicherter Stand mit Version, Ländern (Code, Kontinent, Modus, `n`, `m`, nicht erreichbar), Summen, `hideBroken` |
| `POST /admin/radio/offer/preview` | `admin` | **neu:** Schritte rechnen ohne Speichern; Summen, Unterschied, Länder, auf Wunsch eine Seite der Senderliste eines Landes (`country`, `q`, `offset`, `limit` ≤ 100) mit Stand im Entwurf |
| `POST /admin/radio/offer/apply` | `admin` | **neu:** Schritte mit `version` speichern; 409 bei abweichender Version; Antwort neuer Stand und Zahl der eingereihten Logo-Aufträge. Schritte: `chooseCountry`, `removeCountry`, `markAll`, `demarkAll`, `mark`/`demark` (Kennungen, höchstens 5'000 je Schritt), `markMatching`/`demarkMatching` (`country`, `q`), `demarkUnreachable`, `setHideBroken`; höchstens 1'000 Schritte |
| `listAdminRadioStations` | `admin` | neu Parameter `offered`, `provider`, Zustand `noLogo`, Sortierung `popularity`; **entfallen** Parameter `genre`, Zustand `hidden`, Bedeutung von `exceptions` (hiess «geändert oder ausgeblendet») |
| `AdminRadioStation` | – | neu `offered`, `provider`; **entfallen** `hidden`, `facets` (mit `RadioStationFacet.manual`), Werte `genre`/`kind`/`mood` in `locks` |
| `updateAdminRadioStation` (`AdminRadioStationUpdate`) | `admin` | **entfallen** `genres`, `kinds`, `moods`, `hidden` und die Schlösser dazu |
| `createRadioStation` (`AdminRadioStationCreate`) | `admin` | **entfallen** `genres`, `kinds`, `moods` |
| `RadioConfig`, `RadioConfigUpdate` | `admin` | **entfallen** `countries`, `hideBroken` |
| `getRadioTagRules`, `setRadioTagRules` | – | **entfallen** |
| `RadioPreferences` | `scoped` | neu `effectiveCountry` (additiv; leer = ohne Landfilter) |
| `RadioStation` | `scoped` | neu `provider` (additiv, B7.4) |
| `RadioFacets` | `scoped` | neu `offerEmpty` (additiv) |

«Nicht erreichbare entfernen» und «Aus dem Angebot nehmen» im Reiter «Sender» sind
`POST /admin/radio/offer/apply` mit einem Schritt; «Rückgängig» ebenso (`mark` mit den gelieferten
Kennungen).

Betroffene Stellen im Client: `SettingsTab`, `StationEditor`, `StationsTab`, `CreateStationDialog`,
`OverviewTab`, Zustandsleiste in `RadioAdminPage`, `TagRulesTab` (weg), `radio/facets.ts`,
`features/admin/queries.ts`, `features/radio/RadioPage.tsx`. Vertragstest: neue Operationen in
`adminRegistry` (`contract_access_test.go`, auch der heutige Körper `{"countries":null}`),
`vistest.AddRadio` mit dem demarkierten Sender.

**Stufe B2, vorläufig 1.27.0 (additiv):** `AdminRadioStation.sources` (Herkunft je Angabe),
`AdminRadioStation.wikidata` (Eigentümer, Gründung, Kennung), `RadioConfig.wikidataEnabled`, Jobart
`radio.wikidata.sync` mit Text in `admin.jobs.types`.

## B12. Bauplan

| Stufe | Inhalt | Release | Aufwand |
|---|---|---|---|
| **B0** | Messung auf CT 190 mit der echten Logo-Kette (96 Pixel, Drossel 2 und 4 je Sekunde, Sperre je Host) für die Schweiz und Deutschland, Speicher je Logo, Zeit einer Vorschau über den ganzen Index, Zahlen je Land aus dem echten Index. Ergebnis nach `docs/perf/19-radio.md`. ADR 0030 «Radio-Angebot statt ganzer Katalog» (ergänzt ADR 0025). | – | ½ Tag |
| **B1 «Angebot»** | Migration 00045, Vorbereitung beim ersten Start (B4.4, Angebot leer), Momentaufnahme im Speicher, Regel in `reachable`, `closeUnreachable` nach dem Tausch, Vorschau und Übernehmen mit Version, neue Logo-Auslöser, Sperre je Host, Drossel, Aufräumen gelöschter Sender, Anbieter aus dem Stream-Host, Neuableitung der Filterwerte, Vertrag 1.26.0. Oberfläche: Reiter «Angebot», «Sender» neu, «Einordnung» weg, Editor und Anlegen ohne Filterfelder, Einstellungen ohne Länder, Zustandsleiste, Radioseite mit `effectiveCountry` und leerem Angebot. | 1.26.0 | 6 bis 7 Tage |
| **B2 «Mehr Angaben»** | Migration 00046, `radio.wikidata.sync`, Eigentümer als Anbieter, Gründung, Herkunft je Angabe im Aufklapp, Einstellung «Wikidata verwenden», Vertrag 1.27.0. | 1.27.0 | 2 bis 3 Tage |

Die Nummern sind vorläufig. Teil A und Teil B sind unabhängig; baut Marco Teil B zuerst, bekommt er
die nächste freie Migration und Vertragsfassung, und alle Verweise in B4.4, B10, B11 und PB-6 folgen
der echten Nummer. Jede Stufe ist ein eigener Zweig, ein PR, eine Abnahme auf CT 190 und ein Release.
**Gebaut wird erst auf Marcos ausdrückliches Wort.**

## B13. Abnahme und Tests

| Nr. | Prüfung | Stufe der Prüfung |
|---|---|---|
| PB-1 | Schweiz wählen, übernehmen: die Radioseite zeigt nur Schweizer Sender und eigene Sender; ein deutscher Sender antwortet auf Station, Hören und Bild mit 404. | Go (`vistest`), e2e |
| PB-2 | «Alle demarkieren», drei Sender markieren: genau diese drei im Angebot; ein neuer Schweizer Sender im nächsten Abgleich kommt nicht dazu. Bei «Alle» kommt er dazu. Ein demarkierter Sender, der das Land wechselt, bleibt draussen. | Go (Abgleich gegen Fixture) |
| PB-3 | `markMatching`/`demarkMatching` mit über 100 Treffern: genau die Treffer ändern sich, der Modus bleibt; die Vorschau zeigt die zweite Seite richtig. | Go |
| PB-4 | Nach «Übernehmen» werden nur die neu dazugekommenen Sender ohne Logo eingereiht, Favoriten zuerst; Vorschau, Senderliste und Bildroute der Verwaltung reihen nichts ein. | Go (Queue-Attrappe) |
| PB-5 | 77 Logo-Aufträge mit demselben Host: nie zwei gleichzeitig; ein Scan-Auftrag wird in dieser Zeit sofort angenommen. | Go |
| PB-6 | Erster Start nach der Migration: das Angebot ist leer, auch mit `countries = ["CH"]`; ein ausgeblendeter Schweizer und ein ausgeblendeter eigener Sender sind demarkiert; Favoriten stehen noch in der Datenbank. Wählt der Admin danach die Schweiz, sind der Schweizer Favorit wieder da und der ausgeblendete Sender nicht. | Go (eingefrorener Stand) |
| PB-7 | Erster Start nach der Migration: Regeln des Admins, manuelle Filterwerte und Filterschlösser sind weg; die Filterwerte eines Katalogsenders, eines Senders ohne Schlagworte und eines eigenen Senders entsprechen der Regeldatei. | Go (eingefrorener Stand) |
| PB-8 | «Nicht erreichbare entfernen» demarkiert genau die nicht erreichbaren Sender im Angebot ausser Favoriten und eigenen; «Rückgängig» stellt sie wieder her. | Go, Vitest |
| PB-9 | Standardland eines Profils nicht im Angebot: `effectiveCountry` ist leer, die Radioseite zeigt alle Sender des Angebots ohne Landfilter und die Reihe «Beliebt»; `defaultCountry` bleibt; ein gespeicherter Filter verliert das Land. Leeres Angebot: `offerEmpty`. | Go, Vitest |
| PB-10 | Tastatur im Reiter «Angebot»: Land suchen, Kontinent öffnen, Land wählen, Senderliste öffnen, Haken setzen, übernehmen, ohne Maus; sichtbarer Fokus, keine Fokusfalle. | Playwright |
| PB-11 | Zwei Entwürfe: der zweite «Übernehmen» bekommt 409, die neu gerechnete Vorschau enthält beide Änderungen. | Go, Vitest |
| PB-12 | Ein Sender verlässt das Angebot, während ein Profil ihn hört: der nächste Heartbeat bekommt 404, das Relay stoppt. | Go |
| PB-13 | Vertrag: jede neue Operation im Vertragstest; die entfallenen Operationen und Felder sind aus Vertrag, Client und Tests verschwunden. | Go (Vertragstest) |
| PB-14 | B2: Ein Sender mit eindeutigem Wikidata-Treffer bekommt Eigentümer und Gründung; ein mehrdeutiger bleibt leer; ein Sender ausserhalb des Angebots wird nicht gespeichert; bei ausgeschalteter Einstellung kein Abruf. | Go (Fixture der SPARQL-Antwort) |

## B14. Offene Punkte zum Teil B

Alle beantwortet am 10. November 2026.

| Nr. | Punkt | Marcos Antwort |
|---|---|---|
| OB-1 | Neue Sender eines Landes | bei «Alle» kommen sie von selbst dazu, nach «Alle demarkieren» nicht (Empfehlung) |
| OB-2 | Angebot nach dem Update | **leer**, bis der Admin Länder wählt (gegen die Empfehlung; B4.4) |
| OB-3 | Lesart «Metadaten erst nach dem Hinzufügen» | wie B7.1 (Empfehlung) |
| OB-4 | Alte Regeln und Handwerte | löschen (Empfehlung) |
| OB-5 | Wikidata | Eigentümer und Gründung, nur in der Verwaltung, keine Logos (Empfehlung) |
| OB-6 | Nicht erreichbare Favoriten | sichtbar mit Hinweis; «Nicht erreichbare entfernen» lässt Favoriten und eigene Sender aus (Empfehlung) |
| OB-7 | Sender ohne gültiges Land | nur über «Eigene Sender» (Empfehlung) |
| OB-8 | Wo der Anbieter erscheint | **auch in der App** (gegen die Empfehlung; B7.4) |
| OB-9 | Untergrenze Logo | 96 Pixel (Empfehlung) |
| OB-10 | Standardland ausserhalb des Angebots | **ohne Landfilter** (gegen die Empfehlung; B4.5) |
| OB-11 | Drossel für Logos | 4 je Sekunde, mit Sperre je Host (Empfehlung) |

## B15. Kritiker-Review vom 10. November 2026

Geprüft wurde Fassung 1 des Teils (`72b71f5a`) gegen den Code. Urteil **REVISE**: 0 Blocker,
17 Major, 10 Minor. Alle Befunde sind eingearbeitet:

| Nr. | Schwere | Befund | Wo eingearbeitet |
|---|---|---|---|
| 1 | Major | Der Index wird nach `updateRadioConfig` nicht neu gebaut; `view()` liest `radio_config` je Anfrage. | Momentaufnahme im Speicher (B4.2), Leistung (B8), Messung (B12) |
| 2 | Major | Laufende Sitzungen auf Sendern, die das Angebot verlassen, waren nicht geregelt. | `closeUnreachable` nach dem Tausch (B4.2), PB-12 |
| 3 | Major | Die gemeinsame Drossel von 2 Abrufen je Sekunde fehlte; Zeiten waren um das Vierfache zu tief. | B7.2, Schätzungen in B7.5, OB-11 |
| 4 | Major | «Einer je Host» ohne Mechanismus hätte Worker blockiert. | Sperre mit `RetryLater` (B7.2), PB-5 |
| 5 | Major | Wikidata mit Priorität 0 liefe nach den Logos; die dritte Logo-Stufe käme nie. | Wikidata ohne Logos, Priorität 4 (B7.1, B7.3) |
| 6 | Major | «Ereignis plus Zeitplan» ist keine gültige Jobart. | Muster von `radio.catalog.sync`, Sperre 24 Stunden (B7.3) |
| 7 | Major | Die Übernahme des Standardlands geht nicht in SQL. | Befüllung beim ersten Start (B4.4), PB-6 |
| 8 | Major | Eine neue Installation hätte ein leeres Angebot. | Standardland des Servers (B3, B4.4) |
| 9 | Major | Favoriten ohne Land gingen verloren; verborgene Favoriten kämen zurück. | Ausnahmen unabhängig vom Land, nur sichtbare Favoriten (B4.1, B4.4) |
| 10 | Major | Ausgeblendete Sender ausserhalb der gewählten Länder und eigene ausgeblendete Sender waren nicht abgedeckt. | jede Ausblendung wird Ausnahme, auch für eigene (B4.4) |
| 11 | Major | «Eigene Sender immer im Angebot» ändert heutiges Verhalten. | Zeile in B3, B4.1 |
| 12 | Major | Das Neuableiten der Filterwerte geschähe nicht von selbst. | Durchgang über alle Sender beim ersten Start (B5), PB-7 |
| 13 | Major | Der Dialog für eigene Sender setzt Filterwerte von Hand. | B5, B9.1, B11 |
| 14 | Major | Die Liste der brechenden Änderungen war unvollständig. | B11 Feld für Feld, Versionsregel, betroffene Stellen |
| 15 | Major | «Gezeigte markieren» geht nicht mit Seiten zu 100 im Client. | Schritte mit Suche, vom Server aufgelöst (B4.3, B11), PB-3 |
| 16 | Major | Der Unterschied vor «Übernehmen» hatte keine Quelle. | Vorschau (B4.3, B11), Definition von `n` und `m` |
| 17 | Major | Gleichzeitige Änderungen hätten sich still überschrieben. | Version und 409, relative Schritte (B4.3), PB-11 |
| 18 | Major | Leeres Angebot und Ersatzland brauchen Unterstützung des Servers. | `offerEmpty`, `effectiveCountry`, Filter bereinigt (B4.5, B11), PB-9 |
| 19 | Minor | Favoriten gehen über `reachable`, der Schalter blendet sie nie aus; B6 entschied OB-6 halb. | B6, OB-6 |
| 20 | Major | Ein Aufräumen von Logos gibt es heute nicht. | Aufräumen gelöschter Sender neu (B8), Satz zu 30 Tagen gestrichen |
| 21 | Minor | Bedeutung von `marked` hing vom Modus ab; Landwechsel unklar. | Ausnahmen am Sender, Regeltabelle (B4.1, B10) |
| 22 | Minor | Anbieter wäre für unveränderte Sender nie gesetzt worden; `publicsuffix` ist schon da. | Nachtrag beim ersten Start, erster Stream der Rangliste (B7.4) |
| 23 | Minor | Logo-Auslöser waren unvollständig; das übernommene Angebot würde nie vorgeholt. | Tabelle der Auslöser (B7.2), Logos aus dem Cache in der Verwaltung (B7.1), Spannung RF-6 (B2) |
| 24 | Minor | Die Zustandsleiste las Schlagwort-Regeln und Ausgeblendete. | B9.1 |
| 25 | Minor | Der Sprachenfilter aus RF-1 war nicht erklärt. | RF-1, B3, B4.5 |
| 26 | Minor | Wikidata ohne Schalter, Kontaktadresse, Sprache und Lizenz der Logos offen. | B7.3, keine Logos (B7.1) |
| 27 | Minor | Kontinente, Sortierung, Messort, Zitat aus Frage 23, «Bau auf Marcos Wort», vistest. | B9.2, B7.2, B7.4, B7.5, B3, B12, B10 |

# Teil C: Quadratische Cover in der Bibliotheksübersicht

Marco am 10. November 2026, mit Bildern der Bibliotheken «Podcast» (94 Podcasts) und «Hörbücher»
(89 Hörbücher): «Bei Podcast und Hörbücher sollen die Cover quadratisch sein. Nur in der
Bibliotheksübersicht. Auf der Startseite bleibt es so wie es ist.» Teil C ist von Teil A und Teil B
unabhängig.

## C1. Heute

- Konzept 18, Teil C, **C-1 Variante B** (Marcos Entscheid vom 9. November 2026, Frage 25): Jede
  Karte hat den Posterrahmen 2:3; ein quadratisches Cover sitzt in der Mitte, darüber und darunter
  dasselbe Bild unscharf und abgedunkelt. Das gilt für jede Reihe und jedes Raster, auch für die
  Bibliotheksraster. Grund war, dass der Titel in einer **gemischten** Reihe überall auf derselben
  Linie beginnt.
- `PosterCard` (`web/src/ui/PosterCard.tsx`) hat die Formen `poster` und `square`; `square` zeichnet
  das Cover im 2:3-Rahmen auf dem unscharfen Grund (`.ground`, `.cover`).
- Das Raster der Bibliotheksseite ist `LibraryGrid` (`web/src/features/catalog/LibraryGrid.tsx`),
  virtuell mit fester Kartenhöhe: `measure` rechnet `Breite × 1,5 + Texthöhe` für jede Karte, die
  Platzhalter haben `aspect-ratio: 2 / 3` (`LibraryGrid.module.css`). Die Karten zeichnet `ItemCell`
  (`features/discover/ItemGrid.tsx`) mit `shape={audio ? 'square' : 'poster'}`.
- `LibraryGrid` dient auch `PagedItemGrid` (lange Listen der Suche und anderer Seiten).

## C2. Neu

In der **Übersicht einer Bibliothek** (`/libraries/$libraryId`, `LibraryPage`) zeigen Hörbücher und Podcasts
ihr Cover **quadratisch ohne Rahmen**: Bild 1:1 über die ganze Kartenbreite, kein unscharfer Grund,
der Titel direkt darunter. Die Zeilen werden entsprechend niedriger (Kartenhöhe `Breite × 1 +
Texthöhe`).

**Wann ein Raster quadratisch ist:** wenn jeder Titel, den das Raster zeigen kann, ein Hörbuch oder
ein Podcast ist. Entscheidet `LibraryPage` aus den Inhalten der Bibliothek (`library.contents`) und
dem Filter «Inhalt» (`params.content`):

| Bibliothek | Raster |
|---|---|
| nur Hörbücher, nur Podcasts oder beides | quadratisch |
| gemischt mit Filmen oder Serien, Filter «Alle» | 2:3-Rahmen wie heute (C-1 gilt, der Titel bleibt bündig) |
| gemischt, Filter «Inhalt» auf Hörbücher oder Podcasts | quadratisch |

**Offline-Bibliothek** (Marcos Antwort auf OC-2): Heruntergeladene Hörbücher und Podcasts zeigen ihr
Cover ebenfalls quadratisch ohne Rahmen. Damit die Titel einer Zeile bündig bleiben (OC-1), teilt die
Seite die fertigen Downloads in zwei Abschnitte, sobald beide Arten da sind: **«Filme und Serien»**
(2:3) und **«Hörbücher und Podcasts»** (quadratisch). Gibt es nur eine Art, gibt es nur einen Abschnitt
ohne Überschrift. Der Abschnitt «Wird geladen» bleibt gemischt im 2:3-Rahmen.

**Unverändert (2:3-Rahmen mit unscharfem Grund, C-1):** die Startseite mit allen Reihen und ihren
Vorab-Karten (`preload.go`, `#poster-prepaint`), Suche, Merkliste, Sammlung, User-Empfehlungen,
Personenseite und jede Liste über `PagedItemGrid`.

## C3. Umsetzung

- `PosterCard`: neue Eigenschaft `frame` (`'poster'` ab Werk, `'own'`); mit `shape="square"` und
  `frame="own"` hat das Bild 1:1, ohne zweites `<img>` für den Grund, mit Bildmassen 400 × 400.
  Fokus- und Glanzrahmen (`.card::before`) folgen dem Bild. Stifte (`tool`, `CoverPen`) und
  Abzeichen bleiben an ihrem Platz relativ zum Bild.
- `ItemCell` reicht `frame` durch.
- `LibraryGrid`: neue Eigenschaft `cover` (`'poster'` ab Werk, `'square'`); `measure` rechnet mit dem
  Faktor 1 statt 1,5, der Platzhalter bekommt `aspect-ratio: 1 / 1`. Wechselt der Filter «Inhalt»
  zwischen gemischt und Audio, misst das Raster neu (wie bei einer anderen Spaltenbreite).
- `LibraryPage` setzt `cover="square"` nach der Tabelle in C2. `PagedItemGrid` setzt nichts und
  bleibt beim 2:3-Rahmen.
- `OfflineLibraryPage` (`DownloadCard`): `frame="own"` für Audio im Abschnitt «Hörbücher und
  Podcasts»; die fertigen Downloads werden nach Art in zwei Abschnitte geteilt.
- Kein Vertrag, keine Migration, keine Änderung am Server. Farben und Masse nur aus Tokens.

## C4. Abnahme und Tests

| Nr. | Prüfung | Stufe der Prüfung |
|---|---|---|
| PC-1 | Bibliothek nur mit Podcasts und eine nur mit Hörbüchern: Karten quadratisch, kein unscharfer Grund, Titel aller Karten einer Zeile auf derselben Linie, keine Überlappung beim Scrollen durch über 200 Titel. | Vitest (`LibraryGrid` `measure`), e2e |
| PC-2 | Gemischte Bibliothek: Filter «Alle» zeigt den 2:3-Rahmen, Filter «Hörbücher» quadratisch; der Wechsel lässt keine Lücke und keine Überlappung. | Vitest (`LibraryPage`), e2e |
| PC-3 | Startseite, Suche und Merkliste zeigen Hörbücher und Podcasts unverändert im 2:3-Rahmen. | Vitest |
| PC-4 | Tastatur in der quadratischen Übersicht: Pfeiltasten, Tab, Enter, Zurück zur selben Karte, sichtbarer Fokus. | Playwright (bestehender Test der Bibliotheksseite um eine Audio-Bibliothek ergänzt) |
| PC-5 | Storybook: `PosterCard` quadratisch ohne Rahmen, mit Stift und mit «Gesehen»-Haken; Baselines neu. | `task storybook:test` |
| PC-6 | Offline-Bibliothek mit einem Film und einem Hörbuch: zwei Abschnitte, das Hörbuch quadratisch ohne Rahmen; nur Hörbücher: ein Abschnitt ohne Überschrift. | Vitest, Playwright (bestehender Offline-Test) |

## C5. Bauplan

| Stufe | Inhalt | Release | Aufwand |
|---|---|---|---|
| **C1 «Quadratische Übersicht»** | C3 und C4 | Patch ohne Vertragsänderung (Nummer beim Bau) | ½ Tag |

Teil C kann vor oder nach Teil A und Teil B gebaut werden. **Gebaut wird erst auf Marcos
ausdrückliches Wort.**

## C6. Offene Punkte zum Teil C

Beantwortet am 10. November 2026.

| Nr. | Punkt | Marcos Antwort |
|---|---|---|
| OC-1 | Gemischte Bibliothek mit Filter «Alle» | 2:3-Rahmen wie heute, Titel bündig (Empfehlung) |
| OC-2 | Offline-Bibliothek | **auch quadratisch** (gegen die Empfehlung); umgesetzt mit zwei Abschnitten, damit OC-1 auch dort gilt (C2) |

# Teil D: Hörbücher – Reihen, Bleistift und Quellen

Teil D gehört zu Konzept 19, weil Marco ihn am 10. November 2026 dazu genommen hat. Er hängt nur an
einer Stelle an Teil C (quadratische Kacheln, D4.2) und ist sonst von Teil A und Teil B unabhängig.

## D1. Was Marco will

| Nr. | Marcos Wunsch (10. November 2026, sinngemäss) | Abschnitt |
|---|---|---|
| H-1 | «Es gibt auch bei Hörbüchern Sammlungen. Ich möchte zwar nicht, dass diese in den Bereich Sammlungen fallen. Die Sammlungen sollen einfach im Bereich Hörbücher angezeigt werden.» | D4 |
| H-2 | «Auch bei den Hörbüchern möchte ich den Bleistift, um die Metadaten anzupassen.» Dazu ein Mockup. | D5, Anhang A |
| H-3 | «Auch benötigen wir da wohl ein paar Scraper.» Ins Konzept nehmen und das Ganze planen. | D6 |

Marcos Antworten vom 10. November 2026 (D11): Hörbuch-Sammlungen sind **beides**, die Reihen aus den
Metadaten **und** von Hand angelegte Sammlungen mit Hörbüchern (OD-1); gezeigt als **Zeile «Reihen und
Sammlungen» über allen Hörbüchern** (OD-2). Alle Quellen samt Audible sind ab Werk an (OD-3), und
Slouchcat ordnet **alle Hörbücher ohne Zuordnung** automatisch zu (OD-4).

## D2. Was sich gegenüber früheren Entscheiden ändert

| Bisher | Neu | Grund |
|---|---|---|
| Konzept 07 und ADR 0015: «Der «Hörbuch Scraper» ist der heutige lokale Erkenner. Ein externer Hörbuch-Anbieter kommt im Scraper-Konzept» (`07-bibliotheken.md`, Entscheid vom 28. Oktober 2026); ADR 0007: «Metadaten ohne externen Dienst». | **Externe Quellen für Hörbücher**, je Quelle ein Schalter; die Dateien bleiben die erste Quelle (D6). | H-3 |
| Konzept 15, 7.1 bis 7.5: Bleistift und Editor für Film, Serie, Staffel und Folge. | Dazu **Hörbuch** (D5). | H-2 |
| Konzept 10: «Für Hörbücher und Podcasts bleibt `coverSource`, wie es ist.» | Für **Hörbücher** gibt es Cover-Kandidaten und Hochladen (D5.3); Podcasts bleiben, wie sie sind. | H-2 |
| Konzept 17, «Reihe», und Konzept 08 E-6: Sammlungen aller Arten stehen in der Startseiten-Reihe «Sammlungen» und als Abschnitt auf der Detailseite. | Hörbuch-Reihen sind **keine Sammlungen** (keine Zeile in `collections`); sie haben eine eigene Seite in der Bibliothek (D4). **Von Hand angelegte Sammlungen, die nur Hörbücher enthalten, stehen im Bereich Hörbücher statt in der Startseiten-Reihe «Sammlungen».** | H-1, OD-1 |

## D3. Befund (Stand `e90109a6`)

**Editor**

- Bleistift und Editor kennen nur `movie`, `series`, `season`, `episode`: `EDITABLE_KINDS`
  (`web/src/features/catalog/EditMetadata.tsx`) und `editableKinds`
  (`server/internal/admin/item_metadata.go`). Ein Hörbuch bekommt `kind:invalid_value`; ein Test hält
  das fest (`api/contract_item_metadata_test.go`). Die Detailseite zeigt für Hörbücher statt des
  Bleistifts «Verwalten» (`DetailActions.tsx`); die Verwaltungsseite des Titels erlaubt dort nur die
  Altersfreigabe.
- Das Feld-Enum `AdminMetadataField` kennt Titel, Originaltitel, Sortiertitel, Jahr, Datum, Tagline,
  Beschreibung und Genres; **Autor, Sprecher, Reihe und Band fehlen** in Editor und `AdminItem`.
- **Schlösser passen nicht zusammen:** Der Editor schreibt Textschlösser je Sprache (`title:de`,
  `overview:de`, `tagline:de`, `sort_title:de`; `model/item_metadata.go`), der Hörbuch-Scan prüft aber
  die alten Namen `title`, `overview`, `tagline` (`library/audiobook_write.go`, `writeBook`,
  `writeBookText`). Ein Feld, das der Editor sperrt, würde der nächste Scan überschreiben.
- **Bilder:** `choosableKinds` hat keine Art für Audio; Kandidaten, Wahl und Hochladen antworten mit
  422 (`metadata/image_candidates.go`). Das Cover liegt als Art `poster`. `ImageCandidateGrid` kennt nur
  hoch und quer, nicht quadratisch.

**Daten und Anbieter**

- Ein Hörbuch hat `author`, `narrator`, `series_name`, `series_index`, `duration_ms`, `cover_path`
  (Migration 00014), Untertitel als `tagline`, Beschreibung als `overview`. **Es fehlen Verlag, ISBN
  und ASIN.** Die Sprache ist nur `de` oder `en`.
- Gelesen wird `metadata.json`, dann die Tags der ersten Datei, dann der Ordnername
  `Autor/[Reihe/][NN - ]Titel` (`library/audiobook.go`, `mergeBook`). `refreshItem` liest die Dateien
  neu und reiht für Hörbücher `metadata.match` ein, das mit `unsupported_kind` endet.
- Die Schnittstelle `metadata.Provider` kennt nur Filme und Serien; Anbieter sind fest verdrahtet
  (`tmdb`, `tvdb`, `anilist`, Fanart). Für Hörbuch-Ordner ist `local` die einzige Quelle
  (`model/model.go`).

**Reihen und Sammlungen**

- `series_name` und `series_index` stehen nur auf der Detailseite («Reihe · Band n»). `ItemRef` hat
  keine Reihe; `listLibraryItems` liefert nur `ItemRef[]`, ohne Gruppen.
- Sammlungen (`collections`, Quelle `tmdb` oder `manual`) entstehen nur über den Film-Abgleich oder von
  Hand. Hörbücher kommen nur in Sammlungen, die der Admin von Hand anlegt. Für Zuschauer gibt es die
  Startseiten-Reihe «Sammlungen» und die Seite einer Sammlung, keine Übersichtsseite.

**Marcos Ordner** (`/mnt/nerdnas/medien/Audiobookshelf/Hörbücher`, gelesen am 10. November 2026)

- 107 Bücher mit `metadata.json` (die Bibliothek zeigt 89 Hörbücher); 86 mit Beschreibung, 85 mit
  Sprecher, 46 mit Reihe, **1 mit ASIN, 2 mit ISBN**, 9 mit Untertitel, 1 mit Verlag.
- **Reihen in mehreren Schreibweisen:** «Bodyguard» (6) und «Bodyguard - Band» (5); «Harry Potter» (11)
  und «Harry Potter - Gesprochen von Rufus Beck» (3); «Herr der Ringe» (3) und «Der Herr Der Ringe -
  Band» (2).
- **Autoren in mehreren Schreibweisen:** «J R R Tolkien», «J.R.R. Tolkien», «J. R. R. Tolkien»;
  «J K Rowling», «J.K. Rowling»; «Marc Uwe Kling», «Marc-Uwe Kling».
- **Doppelte Bücher:** Bodyguard 1 bis 6 liegt in zwei Ordnern; «Die Gefährten» und «Die zwei Türme»
  ebenso; «Harry Potter 1 und 3» je als Lesung und als Hörspiel mit voller Besetzung.

## D4. Reihen im Bereich Hörbücher (H-1)

### D4.1 Was im Bereich Hörbücher erscheint

Zwei Arten von Hörbuch-Sammlungen (Marcos Antwort OD-1, «beides»):

**Reihen aus den Metadaten**

- **Quelle:** die Reihe jedes Hörbuchs (`series_name`, Band `series_index`) aus Dateien, Editor oder
  Anbieter. Nichts von Hand anzulegen.
- **Zusammenfassen:** Bücher gehören zur selben Reihe, wenn sie in derselben Bibliothek liegen und der
  **Schlüssel ihrer Reihe** gleich ist: Name vereinheitlicht (Gross- und Kleinschreibung, Leerzeichen,
  Bindestriche, Artikel am Anfang) und um bekannte Anhängsel gekürzt («- Band», «-Reihe», «- Gesprochen
  von …», «[German Edition]»). So werden «Bodyguard» und «Bodyguard - Band» eine Reihe. Die Liste der
  Anhängsel liegt als eingebettete Datei im Paket `library`, gepflegt wie die Regeldatei des Radios.
- **Name der Reihe:** der Name eines Anbieters, wenn ein Band zugeordnet ist; sonst die häufigste
  Schreibweise ohne Anhängsel; von Hand änderbar (D5.4).
- **Eine Reihe braucht mindestens zwei Bände.** Ein Buch mit einer Reihe, aber ohne zweiten Band, steht
  nur als einzelnes Buch da.
- **Kein Eintrag in `collections`.**

**Von Hand angelegte Sammlungen mit Hörbüchern**

- Eine Sammlung (`collections`, Quelle `manual`), deren Mitglieder **alle Hörbücher** sind, gilt als
  Hörbuch-Sammlung. Sie erscheint im Bereich Hörbücher jeder Bibliothek, die mindestens ein Mitglied
  enthält, mit den Mitgliedern dieser Bibliothek, und öffnet die vorhandene Seite einer Sammlung
  (`/collections/$collectionId`).
- Sie erscheint **nicht** mehr in der Startseiten-Reihe «Sammlungen». Angelegt und gepflegt wird sie
  weiter unter «Verwaltung › Sammlungen».
- Eine gemischte Sammlung (Filme und Hörbücher) bleibt eine gewöhnliche Sammlung.

**Für beide gilt:** sichtbar, wenn mindestens ein Mitglied für das Profil sichtbar ist (wie Sammlungen,
Konzept 08 E-6); gezählt und gezeigt werden nur sichtbare Mitglieder.

### D4.2 Die Übersicht der Bibliothek (Mockup `Hoerbuecher`, Variante B)

**Marcos Wahl (OD-2): Variante B, Zeile «Reihen und Sammlungen» oben.** Über dem Raster steht eine
waagrechte Zeile mit einer quadratischen Kachel je Reihe und je Hörbuch-Sammlung, mit Stapel-Rand und
dem Abzeichen «6 Bände»; darunter alle Hörbücher einzeln wie heute (quadratisch nach Teil C).

- Die Kachel trägt das Cover von Band 1 (oder des kleinsten Bandes; bei einer Sammlung das Bild der
  Sammlung oder das Cover des ersten Mitglieds), den Namen und darunter Autor und Zahl der Bände. Ein
  Balken zeigt, wie viele Bände das Profil gehört hat. Antippen öffnet die Seite der Reihe (D4.3) oder
  der Sammlung.
- Die Zeile ist nach Name sortiert, unabhängig von der Sortierung des Rasters, und scrollt waagrecht wie
  eine Reihe der Startseite (Pfeiltasten, Tab).
- Filter (Genre, Jahr) gelten auch für die Zeile: eine Reihe erscheint, wenn mindestens ein Band passt.
  Die Buchstabenleiste gilt nur für das Raster.
- Die Zeile erscheint nur in Bibliotheken mit Hörbüchern und nur, wenn es mindestens eine Reihe oder
  Sammlung gibt. Die Kopfzeile sagt «89 Hörbücher · 17 Reihen».
- Variante A (eine Kachel je Reihe im Raster) ist nicht gewählt; das Mockup zeigt beide.

### D4.3 Die Seite einer Reihe (Mockup `Reihe`)

- Adresse `/libraries/$libraryId/series/$seriesId`; Zurück führt in die Übersicht an dieselbe Stelle.
- Kopf: Cover mit Stapel, Name, Autor, Sprecher (wenn bei allen gleich), Zahl der Bände, Gesamtdauer;
  Knopf **«Weiterhören: Band n»** (erster nicht fertig gehörter Band des Profils, sonst Band 1).
- **Bände** nach Bandnummer (0,5 vor 1), Bände ohne Nummer am Ende nach Titel. Zwei Bücher mit
  derselben Nummer (Lesung und Hörspiel) stehen beide da, unterschieden durch den Sprecher.
- Je Band: Nummer, Cover, Titel, Fortschritt, Dauer, Abspielen, Bleistift (Admin).
- **Nur für Admins** ein Hinweis, wenn die Reihe aus mehreren Schreibweisen zusammengefasst ist oder ein
  Band doppelt vorkommt (D3), mit «Verstanden».
- Die Detailseite eines Bandes verlinkt «Reihe · Band n» auf diese Seite und zeigt die Bände als Reihe
  (Bauteil von `CollectionShelf`, Konzept 17).

### D4.4 Server

- Neue Tabelle `audio_series` (Kennung, Bibliothek, Schlüssel, Name, Name gesperrt, Zeitstempel;
  eindeutig je Bibliothek und Schlüssel) und Spalte `items.audio_series_id`. Gepflegt von
  `finishAudiobook` im Paket `library` bei jedem Schreiben eines Buches; eine Reihe ohne Band
  verschwindet beim Sweep.
- Hörbuch-Sammlungen brauchen keine Tabelle: «nur Hörbücher» wird beim Lesen geprüft. Die Abfrage der
  Startseiten-Reihe «Sammlungen» (`ListVisibleCollections`) lässt Sammlungen weg, deren sichtbare
  Mitglieder alle Hörbücher sind.
- Die Abfragen der Übersicht und der Reihe laufen im Paket `visibility` über `visible_items` (Regel 2).
- Die Kennung ist stabil, solange der Schlüssel gleich bleibt; ändert der Admin den Namen, bleibt sie.

## D5. Bleistift und Editor (H-2)

### D5.1 Wo der Bleistift erscheint

Überall, wo er bei Filmen erscheint: auf der Karte in der Übersicht (beim Zeigen oder mit dem Fokus),
auf der Detailseite und auf der Startseite; dazu je Band auf der Seite der Reihe und als «Reihe
bearbeiten» im Kopf der Reihe. Nur für Admins (`canManage`), wie heute.

### D5.2 Der Editor (Mockup `Editor`)

Derselbe Dialog wie für Filme und Serien (`MetadataEditor`), mit quadratischem Kopfbild und vier
Reitern:

| Reiter | Inhalt |
|---|---|
| **Metadaten** | Titel, Untertitel, Sortiertitel, Autor, Sprecher, Reihe, Band, Jahr, Verlag, Sprache, Genres, Beschreibung, ISBN, ASIN. Je Feld Herkunft («Datei», Name des Anbieters, «Von Hand») und Schloss. Ein geändertes Feld bekommt das Schloss. |
| **Cover** | Kandidaten quadratisch: Bild im Ordner, eingebettetes Bild, Cover der Anbieter (D6), Hochladen. Je Kandidat Quelle und Grösse. |
| **Zuordnen** | Suche nach Titel und Autor in einer Quelle (D6); Treffer mit Cover, Autor, Sprecher, Verlag, Jahr, Kennung und **Länge**, die passende Länge markiert. Ein Treffer zeigt Feld für Feld «heute» und «neu» mit Haken; «n Felder übernehmen». Gesperrte Felder sind ausgegraut. |
| **Aktualisieren** | «Aus den Dateien neu lesen» und «Vom Anbieter neu holen» (nur mit Zuordnung), je mit «Schlösser lösen». |

**Übernommene Felder bekommen das Schloss**, wie von Hand geänderte: sonst würde der nächste Scan die
Werte aus den Dateien zurückschreiben (die Dateien sind die erste Quelle, D6.3).

### D5.3 Server und Vertrag

- `editableKinds` und `EDITABLE_KINDS` um `audiobook`; Felder `titleOnly` werden je Art geprüft.
- `AdminMetadataField` um `author`, `narrator`, `seriesName`, `seriesIndex`, `publisher`, `isbn`,
  `asin`, `language`; `AdminItem.metadata` liefert sie für Hörbücher. Additiv: der Client nutzt das Enum
  nur als `Partial<Record>`.
- **Schlösser vereinheitlichen:** `writeBook` und `writeBookText` prüfen dieselben Namen, die der Editor
  schreibt (`title:<Sprache>`, `overview:<Sprache>`, `tagline:<Sprache>`, `sort_title:<Sprache>`, dazu
  `author`, `narrator`, `series_name`, `series_index`, `publisher`, `isbn`, `asin`, `language`); die alten
  Namen gelten weiter. Test mit beiden Formen.
- **Cover:** `choosableKinds` bekommt für Audio die Art `poster` mit quadratischen Kandidaten (Ordner,
  eingebettet, Anbieter); Hochladen wie bei Filmen. `ImageCandidateGrid` bekommt die Form `square`.
- **Zuordnen:** `searchMetadata` und `setItemMatch` nehmen `audiobook` und die Hörbuch-Quellen an;
  `refreshItem` holt bei einem zugeordneten Hörbuch die Angaben des Anbieters neu, statt
  `unsupported_kind` zu melden.
- Die Verwaltungsseite des Titels (`/admin/items/$itemId`) zeigt für Hörbücher dieselben Felder.

### D5.4 Reihe bearbeiten

«Reihe bearbeiten» ändert den **Namen der Reihe** (sperrt ihn in `audio_series`) und, auf Wunsch,
schreibt ihn in alle Bände (sperrt dort `series_name`). Zwei Reihen werden eins, wenn der Admin der
einen den Namen der anderen gibt. Die Reihenfolge entsteht aus den Bandnummern; eine falsche Nummer ändert
der Admin am Band.

## D6. Quellen für Hörbücher (H-3)

### D6.1 Was es gibt (Recherche vom 10. November 2026, mit Live-Abfragen)

| Quelle | Schlüssel | Liefert | Cover | Deutsch | Lizenz und Risiko |
|---|---|---|---|---|---|
| **Audible-Katalog** (`api.audible.de/1.0/catalog/products`) | keiner | Titel, Untertitel, Autor, Sprecher, Reihe und Band, Verlag, Datum, Sprache, Beschreibung, Kategorien, Länge, ASIN | 2400 px, quadratisch | sehr gut | **Audibles Nutzungsbedingungen verbieten das automatische Auslesen**; inoffiziell, kann jederzeit brechen |
| **Audnexus** (`api.audnex.us`) | keiner | wie Audible je ASIN, dazu ISBN, Genres, Autorenbild und -text, Kapitel; **keine Buchsuche** | 2400 px | sehr gut | Daten von Audible, ein Betreiber; Grenze rund 100 je Minute; selbst betreibbar |
| **Deutsche Nationalbibliothek** (SRU) | keiner | Titel, Autor, Sprecher, Verlag, Jahr, Sprache, Genre, ISBN, «Ungekürzte Lesung» | keines | massgeblich | **CC0**, amtlich |
| **Apple Books** (iTunes Search, `country=ch`) | keiner | Titel, ein Autor, Beschreibung, Genre, Datum, Verlag | bis 3000 px | gut | Bedingungen zielen auf Werbung für den Store; rund 20 Abfragen je Minute |
| **Open Library** | keiner | Titel, Autor, ISBN, selten Hörbuch-Angaben | selten quadratisch | schwach | offen, 1 bis 3 je Sekunde |
| **Google Books** | **Schlüssel** | Titel, Verlag, ISBN, Beschreibung | nicht quadratisch | schwach | offiziell |
| audiosilo-meta | keiner | Suche und Nachschlagen per ASIN oder ISBN, 22'570 deutsche Werke | Amazon-Adressen | gut | CC0, jung; selbst betreibbar |
| MusicBrainz | keiner | Hörspiel- und Hörbuch-CDs mit Sprecher | Cover Art Archive | nur CDs | CC0, 1 je Sekunde |

Nur Audible zusammen mit Audnexus deckt alle Felder für deutsche Hörbücher ab, und genau dort liegt das
rechtliche Risiko. Die DNB ist die saubere Quelle, hat aber weder Cover noch Länge noch Beschreibung.
Cover jeder Quelle bleiben Werke der Verlage: Slouchcat speichert sie nur verkleinert im eigenen Cache
für die eigene Anzeige und gibt sie nie weiter. Das ist keine Rechtsberatung für die Schweiz.

### D6.2 Quellen ab Werk (Marcos Antwort OD-3)

| Quelle | Ab Werk | Rolle |
|---|---|---|
| Dateien | immer an, zuerst | `metadata.json`, Tags, Ordner, Cover im Ordner oder eingebettet |
| Audible (Suche) und Audnexus (Details) | **an** (Marcos Entscheid, gegen die Empfehlung «aus») | alle Felder, Reihe und Band, Länge für den Abgleich, Cover 2400 px |
| Deutsche Nationalbibliothek | an | ISBN, Sprecher, Verlag, deutsche Titel prüfen |
| Apple Books | an | Cover in hoher Auflösung, Beschreibung |
| Open Library | aus | ISBN-Suche als Rückfall |
| Google Books | aus, nur mit Schlüssel | Rückfall |

Marco hat Audible im Wissen um dessen Nutzungsbedingungen ab Werk eingeschaltet. Die Seite «Quellen»
nennt den Hinweis weiter, und jeder Admin kann die Quelle ausschalten. ADR 0031 hält den Entscheid und
das Risiko fest (inoffizielle Schnittstelle, kann brechen; dann fällt die Zuordnung auf die übrigen
Quellen zurück, ohne Fehler für die Zuschauer).

audiosilo-meta und MusicBrainz baut Teil D nicht; beide lassen sich später als weitere Quelle anhängen.
Audnexus bekommt eine einstellbare Adresse, damit Marco eine eigene Instanz nutzen kann.

### D6.3 Wie zugeordnet wird

1. **Kennung zuerst:** Steht eine ASIN oder ISBN in den Dateien oder im Editor, sucht die Quelle direkt
   danach.
2. **Sonst Titel und Autor:** Suche in der Reihenfolge der eingeschalteten Quellen; Audible zuerst in
   der Region der Installation (Schweiz → `audible.de`), für englische Bücher dazu `audible.com` und
   `audible.co.uk`.
3. **Bewertung wie bei Audiobookshelf:** Länge 70 %, Titel 20 %, Autor 10 %. **Automatisch** zugeordnet
   wird nur ein eindeutiger Treffer mit passendem Autor und einer Länge innerhalb der Toleranz (ab Werk
   3 %). Sonst bleibt das Buch bei den Angaben der Dateien und erscheint in der Liste **«Prüfen»** der
   Verwaltungsseite.
4. **Felder:** Eine automatische Zuordnung **füllt nur leere Felder**; die Dateien bleiben vorn. Eine
   Zuordnung im Editor übernimmt die angehakten Felder und sperrt sie (D5.2). Ein gesperrtes Feld ändert
   keine Quelle.
5. **Mehrere Quellen je Feld:** die obere Quelle in der Reihenfolge gewinnt; die Reihenfolge stellt der
   Admin auf der Seite «Quellen» um.
6. **Automatisch zuordnen** (Einstellung): «Aus», «Neue Hörbücher» (beim Scan) oder «Alle ohne
   Zuordnung» (einmal für die Bibliothek, danach wie «Neue Hörbücher»). Ab Werk **«Alle ohne
   Zuordnung»** (Marcos Antwort OD-4): Nach dem Update auf Stufe D3 läuft die Zuordnung einmal für alle
   Hörbücher im Hintergrund (Pool `general`, Priorität wie der Abgleich von Filmen, gedrosselt nach
   D6.5). Weil eine automatische Zuordnung nur leere Felder füllt und nur eindeutige Treffer mit
   passender Länge nimmt, ändert sie an Marcos gepflegten `metadata.json` nichts, was schon steht. Die
   Messung D0 zeigt vorher, wie viele Treffer eindeutig sind.

### D6.4 Vereinheitlichen

Zwei Schalter auf der Seite «Quellen» (ab Werk an):

- **Autorennamen:** Initialen mit Punkt und Leerzeichen («J R R Tolkien», «J.R.R. Tolkien» →
  «J. R. R. Tolkien»), Bindestrich-Vornamen nach dem häufigsten oder dem Namen des Anbieters
  («Marc Uwe Kling» → «Marc-Uwe Kling»). Gilt für Anzeige, Sortierung und Suche; die Dateien bleiben
  unverändert.
- **Reihennamen:** die Anhängsel aus D4.1; wirkt auf den Schlüssel immer, auf den angezeigten Namen nur
  mit diesem Schalter.

### D6.5 Server

- Neue Schnittstelle `metadata.BookProvider` (`Name`, `Configured`, `SearchBooks`, `Book`, `Covers`)
  neben `Provider`; eine Umsetzung je Quelle im Paket `metadata`: `audible`, `audnexus`, `dnb` (SRU mit
  MARC21), `applebooks`, `openlibrary`, `googlebooks`.
- Jede Quelle über den Netzwächter, mit eigener Drossel (Audible und DNB 1 je Sekunde, Audnexus 60 je
  Minute, Apple 20 je Minute, Open Library 1 je Sekunde), User-Agent `Slouchcat/<Version>
  (+https://github.com/SlouchCat/slouchcat)`, Antwort höchstens 5 MB, Ergebnisse 7 Tage im Cache.
- Der Schlüssel für Google Books liegt verschlüsselt in den Einstellungen wie die anderen Schlüssel und
  kommt in die Redaction-Liste (`internal/logging`, Regel 10).
- `metadata.match` bekommt einen Zweig für `audiobook`; Auftrag je Buch, Pool `general`, Priorität wie der
  Abgleich von Filmen.
- Einstellungen `metadata.audiobook.*` (Quellen an oder aus, Reihenfolge, Region, Adresse von Audnexus,
  Toleranz, automatisch zuordnen, Vereinheitlichen), für den ganzen Server.

### D6.6 Verwaltung: Seite «Quellen» (Mockup `Quellen`)

Unter «Verwaltung › Metadaten» ein Abschnitt **«Hörbücher: Quellen»**: Liste der Quellen mit Reihenfolge
(nach oben, nach unten), Schalter, was sie liefern, Region oder Schlüssel; daneben «Automatisch
zuordnen» mit Toleranz, «Vereinheitlichen» und der Stand der Bibliothek (zugeordnet, zu prüfen, ohne
Treffer) mit «Jetzt alle ohne Zuordnung zuordnen».

## D7. Datenmodell (Migration 00047, vorläufig)

| Tabelle oder Spalte | Inhalt |
|---|---|
| `items.publisher`, `items.isbn`, `items.asin` | neue Angaben für Hörbücher |
| `items.original_language` | Sprache des Hörbuchs als ISO 639-1 statt nur `de`/`en` (Spalte gibt es seit 00032) |
| `items.book_match` | Zuordnung: Quelle und Kennung, Zeitpunkt, Bewertung; leer = nicht zugeordnet |
| `audio_series` | Reihe je Bibliothek (D4.4) |
| `items.audio_series_id` | Verweis, `ON DELETE SET NULL` |
| Einstellungen `metadata.audiobook.*` | D6.5 |

Nach Vorlage, nur vorwärts, eingefrorener Stand in `testdata/db/`. Die Reihen entstehen beim ersten
Start nach dem Update aus den vorhandenen Büchern (ein Durchgang wie das Neuableiten in Teil B).

## D8. Vertrag (vorläufig 1.28.0, additiv)

| Operation oder Schema | `x-visibility` | Änderung |
|---|---|---|
| `GET /libraries/{id}/audio-groups` (`listAudioGroups`) | `scoped` | Zeile «Reihen und Sammlungen»: Reihen und Hörbuch-Sammlungen der Bibliothek mit Name, Bild, Autor, Zahl der sichtbaren Bände, gehörte Bände; Filter wie `listLibraryItems` |
| `GET /libraries/{id}/series/{seriesId}` (`getAudioSeries`) | `scoped` | Reihe mit sichtbaren Bänden, Bandnummern, «Weiterhören»; 404, wenn kein Band sichtbar |
| Startseiten-Reihe «Sammlungen» (`getHome`) | `scoped` | ohne Sammlungen, deren sichtbare Mitglieder alle Hörbücher sind (Verhalten, kein neues Feld) |
| `ItemDetail` | `scoped` | neu `seriesId` |
| `PATCH /admin/audio-series/{seriesId}` | `admin` | Name ändern, auf Wunsch in alle Bände schreiben |
| `AdminMetadataField`, `AdminItem.metadata` | `admin` | Felder aus D5.3 |
| `updateItemMetadata`, `listItemImageCandidates`, `setItemImage`, `uploadItemImage`, `searchMetadata`, `setItemMatch`, `refreshItem` | `admin` | nehmen `audiobook` an |
| `MetadataProvider` | – | neue Werte `audible`, `audnexus`, `dnb`, `applebooks`, `openlibrary`, `googlebooks` |
| `GET`/`PATCH /admin/metadata/audiobooks` | `admin` | Einstellungen der Seite «Quellen», Stand der Bibliothek |
| `POST /admin/metadata/audiobooks/match-all` | `admin` | «Jetzt alle ohne Zuordnung zuordnen» |

Neue Werte in `MetadataProvider` prüft der Bau gegen geschlossene `Record`-Typen im Client; ist einer
geschlossen, wird die Fassung brechend für die Verwaltung. Jede Operation im Vertragstest, `scoped` mit
Eintrag in `profileRegistry`.

## D9. Bauplan

| Stufe | Inhalt | Release | Aufwand |
|---|---|---|---|
| **D0** | Messung an Marcos 107 Büchern: wie viele trifft jede Quelle eindeutig, wie viele haben eine passende Länge; prüfen, ob der Audible-Katalog von CT 190 aus antwortet. Ergebnis nach `docs/perf/19-hoerbuecher.md`. ADR 0031 «Externe Quellen für Hörbücher» (ersetzt den Satz aus ADR 0007 und ADR 0015). | – | ½ Tag |
| **D1 «Bleistift»** | Editor für Hörbücher mit allen Feldern, Schlösser vereinheitlicht, Cover-Kandidaten aus Ordner und Datei, Hochladen, Bleistift überall, Migration für Verlag, ISBN, ASIN, Sprache. | vorläufig 1.28.0 | 3 bis 4 Tage |
| **D2 «Reihen»** | `audio_series`, Schlüssel und Anhängsel, Zeile «Reihen und Sammlungen» mit Hörbuch-Sammlungen, Startseiten-Reihe «Sammlungen» ohne diese, Seite der Reihe, Link von der Detailseite, «Reihe bearbeiten». | vorläufig 1.29.0 | 3 bis 4 Tage |
| **D3 «Quellen»** | `BookProvider` mit sechs Quellen, Zuordnen im Editor, automatische Zuordnung, Liste «Prüfen», Vereinheitlichen, Seite «Quellen». | vorläufig 1.30.0 | 6 bis 8 Tage |

D1 vor D3, weil D3 den Reiter «Zuordnen» des Editors braucht; D2 ist unabhängig. Die Nummern folgen
der Bau-Reihenfolge aller Teile. **Gebaut wird erst auf Marcos ausdrückliches Wort.**

## D10. Abnahme und Tests

| Nr. | Prüfung | Stufe der Prüfung |
|---|---|---|
| PD-1 | Bleistift auf einer Hörbuch-Karte öffnet den Editor; Autor, Sprecher, Reihe und Band ändern, speichern; ein Scan danach überschreibt nichts. | Go (Schlösser, beide Namensformen), e2e |
| PD-2 | Cover: Kandidaten aus Ordner und eingebettetem Bild erscheinen quadratisch; Wahl und Hochladen wirken in Übersicht, Detailseite und Startseite. | Go, Vitest, e2e |
| PD-3 | Reihen: «Bodyguard» und «Bodyguard - Band» ergeben eine Reihe mit Bänden 1 bis 6; ein Buch ohne zweiten Band steht einzeln. | Go (Schlüssel, Fixture mit Marcos Schreibweisen) |
| PD-4 | Zeile «Reihen und Sammlungen»: Reihen-Kachel mit Zahl der Bände; ein Kinderprofil, das nur zwei Bände sehen darf, sieht «2 Bände»; keine Reihe ohne sichtbaren Band; darunter alle Hörbücher einzeln. | Go (`vistest`), e2e |
| PD-5 | Seite der Reihe: Reihenfolge 0,5 vor 1, Bände ohne Nummer am Ende; «Weiterhören» nimmt den ersten nicht fertigen Band. | Vitest, e2e |
| PD-6 | Reihen und eine von Hand angelegte Sammlung nur mit Hörbüchern erscheinen in der Zeile der Hörbücher und nicht in der Startseiten-Reihe «Sammlungen»; eine gemischte Sammlung erscheint dort weiter. | Go |
| PD-7 | Zuordnen im Editor: Treffer mit passender Länge oben; Übernehmen schreibt nur angehakte, nicht gesperrte Felder und sperrt sie. | Go (Quellen mit `httptest`), Vitest |
| PD-8 | Automatisch zuordnen («Alle ohne Zuordnung» ab Werk): ein eindeutiger Treffer innerhalb 3 % wird zugeordnet und füllt nur leere Felder; zwei Treffer oder eine falsche Länge kommen in «Prüfen»; ein zweiter Lauf ändert nichts. | Go |
| PD-9 | Quelle aus: kein Abruf; ab Werk sind Dateien, Audible mit Audnexus, DNB und Apple Books an; fällt Audible aus, ordnet der Lauf mit den übrigen Quellen zu; der Schlüssel für Google Books erscheint in keinem Log. | Go (Redaktionstest) |
| PD-10 | Vereinheitlichen: «J R R Tolkien» und «J.R.R. Tolkien» sortieren und suchen als «J. R. R. Tolkien»; die Dateien bleiben unverändert. | Go |
| PD-11 | Tastatur: Bleistift per Tab erreichbar, Editor mit allen Reitern ohne Maus, Seite der Reihe und Seite «Quellen» mit Pfeiltasten, Tab, Enter, Zurück. | Playwright |
| PD-12 | Vertrag: jede neue Operation im Vertragstest; `updateItemMetadata` für Hörbücher statt `kind:invalid_value`. | Go (Vertragstest) |

## D11. Offene Punkte zum Teil D

Beantwortet am 10. November 2026.

| Nr. | Punkt | Marcos Antwort |
|---|---|---|
| OD-1 | Woher kommen Hörbuch-Sammlungen? | **beides:** Reihen aus den Metadaten und von Hand angelegte Sammlungen nur mit Hörbüchern (D4.1) |
| OD-2 | Anzeige in der Übersicht | **B, Zeile «Reihen und Sammlungen» oben** (gegen die Empfehlung A; D4.2) |
| OD-3 | Quellen ab Werk | **alle samt Audible an**, Open Library und Google Books aus (gegen die Empfehlung; D6.2) |
| OD-4 | Automatisch zuordnen ab Werk | **«Alle ohne Zuordnung»** (gegen die Empfehlung «Aus»; D6.3) |
| OD-5 | Doppelte Bücher | nur Hinweis auf der Seite der Reihe (Empfehlung) |
| OD-6 | Bleistift auf der Startseite | ja (Empfehlung) |
| OD-7 | Seite eines Autors mit Bild und Text | nicht jetzt; als Idee festhalten (Empfehlung) |

# Teil E: Lautstärke (zwei Fehler)

Marco am 10. November 2026: «Radio benötigt noch einen Lautstärke Regler und der Lautstärke Regler bei
Podcast flackert. Nimm diese Fehler auch noch ins Konzept.» Teil E ist von den anderen Teilen
unabhängig und kann als erster gebaut werden.

## E1. Befund (Stand `e90109a6`)

**Radio hat keinen Regler.**

- Das Radio spielt über ein eigenes `<audio>`-Element der App (`app/radio/RadioProvider.tsx`,
  `engine.ts`). Weder der Mini-Player (`RadioDock`, `ui/MiniPlayer.tsx`) noch «Jetzt läuft»
  (`ui/NowPlayingDialog.tsx`) noch die Engine setzen eine Lautstärke; das Element läuft immer mit 100 %.
- Konzept 16 und ADR 0025 erwähnen die Lautstärke nicht. Es ist eine Lücke, kein Entscheid.

**Der Regler im Podcast- und Hörbuch-Player flackert.** Nicht nachgestellt; drei Ursachen kommen aus
dem Code in Frage, die erste ist die wahrscheinlichste:

1. **Die Schublade verschiebt die Zeile.** `VolumeControl` (`ui/VolumeControl.tsx`) öffnet den Schieber
   und die Prozentzahl beim Zeigen: die Schublade wächst von `max-width: 0` auf `12rem`
   (`VolumeControl.module.css`). Im Player steht der Regler in der Zeile `.extras` mit
   `justify-content: center` und `flex-wrap: wrap` (`ui/AudioPlayer.module.css`). Wächst ein Element in
   einer zentrierten Zeile, rückt der Knopf unter dem Zeiger um die halbe Breite weg, oder der Regler
   bricht in die nächste Zeile um. Der Zeiger liegt dann nicht mehr über dem Regler, die Schublade
   schliesst, der Knopf rückt zurück unter den Zeiger, die Schublade öffnet wieder: ein Kreis, der als
   Flackern erscheint.
2. **Zwei Ereignisse je Schritt.** `changeVolume` (`features/player/useAudioControls.ts`) setzt
   `volume` und `muted` getrennt; jedes löst `volumechange` aus, und jeder Wert beim Ziehen zeigt
   zusätzlich die Einblendung (`showOsd`). Der Daumen kann so zwischen altem und neuem Wert springen.
3. Die Lautstärke des Audio-Players wird nicht gespeichert (`useState(1)`); jeder Start beginnt bei
   100 %. Das flackert nicht, fällt aber beim Prüfen auf.

## E2. Behebung im Player (Podcasts und Hörbücher)

1. **Zuerst nachstellen:** ein Playwright-Test fährt mit dem Zeiger auf den Lautstärkeknopf des
   Audio-Players in Desktop-Breite und prüft, dass die Schublade offen bleibt und die Nachbarn der Zeile
   sich nicht bewegen. Er muss vor der Behebung fehlschlagen; schlägt er nicht fehl, wird zuerst
   Ursache 2 geprüft.
2. **Schublade ohne Verschieben:** `VolumeControl` öffnet den Schieber **über** den Nachbarn (absolut
   neben dem Knopf, mit Hintergrund und Rand aus den Tokens) statt die Zeile breiter zu machen. Beim
   Verlassen bleibt sie 300 ms offen (`--sc-motion-duration-*`), damit ein kurzes Abrutschen sie nicht
   schliesst. Ohne Zeiger (Touch) bleibt sie wie heute offen. Der Fokus öffnet sie wie heute.
3. **Ein Ereignis je Schritt:** `muted` wird nur gesetzt, wenn es sich ändert; die Einblendung kommt
   nur bei Tasten und Mausrad, nicht beim Ziehen am Schieber (wie im Video-Player).
4. **Merken:** der Audio-Player merkt sich die Lautstärke auf dem Gerät, wie der Video-Player
   (`usePlayerPrefs`, `localStorage`), **gemeinsam mit dem Radio** (OE-1): ein Wert je Gerät für alles
   Hörbare; der Video-Player behält seinen eigenen.

Weil `VolumeControl` ein Baustein aus `web/src/ui` ist, wirkt Punkt 2 auch im Video-Player und im
Offline-Player; ihre Tests und Storybook-Baselines laufen mit.

## E3. Regler für das Radio

- **Wo:** im Mini-Player rechts neben Abspielen/Anhalten und in «Jetzt läuft» unter den Knöpfen; der
  gleiche Baustein `VolumeControl`, ohne Verstärkung über 100 % (das Radio hat keinen Web-Audio-Graphen).
- **iPhone:** nur der Stummschalter (`muteOnly`); Safari auf dem iPhone lässt eine Seite die Lautstärke
  nicht setzen (Konzept 13, 11.2).
- **Wirkung:** setzt `volume` und `muted` des Radio-Elements; gilt sofort, auch beim Wechsel des Senders
  und nach einem Wiederaufbau der Verbindung (`engine.ts` setzt die Werte nach jedem neuen Start).
- **Merken:** auf dem Gerät (`localStorage`), nicht auf dem Server, **derselbe Wert wie im Podcast- und
  Hörbuch-Player** (OE-1); ohne Speicher (privates Fenster) beginnt alles bei 100 %.
- **Tastatur:** Tab erreicht Knopf und Schieber; ←/→ 5 %, Bild auf/ab 10 %, Pos1/Ende; M schaltet stumm,
  solange der Fokus im Mini-Player oder in «Jetzt läuft» liegt.
- Texte aus `de.json`/`en.json` (die vorhandenen `player.volumeControl.*`), Farben aus Tokens.

## E4. Abnahme und Tests

| Nr. | Prüfung | Stufe der Prüfung |
|---|---|---|
| PE-1 | Podcast-Player in Desktop-Breite: Zeiger auf den Lautstärkeknopf, die Schublade öffnet einmal und bleibt offen; Lage der Nachbarknöpfe unverändert; Zeiger weg, nach 300 ms zu. | Playwright (zuerst rot, nach der Behebung grün) |
| PE-2 | Ziehen am Schieber: je Schritt ein `volumechange`, keine Einblendung; Tasten zeigen die Einblendung weiter. | Vitest |
| PE-3 | Radio: Mini-Player hat den Regler; 40 % gesetzt → das Element spielt mit 0,4; nach Senderwechsel und Neuladen der Seite weiter 40 %. | Vitest, Playwright |
| PE-4 | Radio auf dem iPhone (Emulation): nur der Stummschalter. | Vitest |
| PE-5 | Tastatur: Regler im Mini-Player und in «Jetzt läuft» mit Tab, Pfeiltasten und M bedienbar, sichtbarer Fokus, keine Fokusfalle. | Playwright |
| PE-6 | Storybook: `VolumeControl` offen über Nachbarn, `MiniPlayer` mit Regler; Baselines neu. | `task storybook:test` |

## E5. Bauplan

| Stufe | Inhalt | Release | Aufwand |
|---|---|---|---|
| **E1 «Lautstärke»** | E2 und E3 mit den Tests aus E4 | Patch ohne Vertragsänderung | ½ bis 1 Tag |

**Gebaut wird erst auf Marcos ausdrückliches Wort.**

## E6. Offene Punkte zum Teil E

Beantwortet am 10. November 2026.

| Nr. | Punkt | Marcos Antwort |
|---|---|---|
| OE-1 | Eine Lautstärke für alles Hörbare oder getrennt? | **gemeinsam** für Radio, Podcasts und Hörbücher auf dem Gerät (Empfehlung) |
| OE-2 | Schublade über den Nachbarn oder fester Platz? | **über den Nachbarn** (Empfehlung) |

## Anhang A: Mockups

Leinwand «Konzept 19 Podcasts», sechs Artboards, alle klickbar:

| Artboard | Zeigt |
|---|---|
| `Main.dc.html` | **Abos A (gewählt):** Liste mit Covern, Filter, Suche, Ausklapp mit Einstellung und «Fehlende nachladen» |
| `AbosB.dc.html` | Abos B: Raster, Seitenfeld des gewählten Podcasts (nicht gewählt) |
| `AbosC.dc.html` | Abos C: Tabelle, Auswahl in der Zeile, Sammelaktionen (nicht gewählt) |
| `Einstellung.dc.html` | Einstellung A: Dialog in der Bibliothek (nicht gewählt) |
| `EinstellungB.dc.html` | **Einstellung B (gewählt):** Band auf der Podcasts-Seite mit Bestätigung, Abschnitt «Downloads» |
| `Podcast.dc.html` | Einzelner Podcast: eigene Einstellung, Folgenliste mit Zuständen |
| `Hoerbuecher.dc.html` | **Teil D:** Übersicht der Hörbücher mit Reihen-Kacheln (A) oder Reihen-Zeile (B), Bleistift auf jeder Karte |
| `Reihe.dc.html` | Teil D: Seite einer Reihe mit Bänden, «Weiterhören», Hinweis auf zusammengefasste Schreibweisen |
| `Editor.dc.html` | Teil D: Editor für ein Hörbuch mit Reitern Metadaten, Cover, Zuordnen, Aktualisieren |
| `Quellen.dc.html` | Teil D: Verwaltung, Quellen für Hörbücher, automatisch zuordnen, vereinheitlichen |

Die Zahlen in den Mockups sind Beispiele; gemessen sind nur die Werte in Abschnitt 4. Die Zustände
«Entfernt» und «Unklar» und die Gründe der Ordnerprüfung sind in den Mockups noch nicht gezeichnet.

Kopie im Repository: `docs/planning/mockups/19-podcasts/`.

## Anhang B: Mockups Radio

Leinwand «Radio-Verwaltung», sieben Artboards, alle klickbar; die Leinwand ist privat und zeigt echte
Logos aus dem Katalog. Die Kopie im Repository (`docs/planning/mockups/19-radio/`) führt **keine
Logos** (Markenbilder gehören nicht ins öffentliche Repository) und Senderlisten nur für CH, LI, DE,
AT, FR, IT, US und GB.

| Artboard | Zeigt |
|---|---|
| `AngebotB.dc.html` | **Angebot B (gewählt):** Länder über Suche und Kontinente, je Land die Senderliste mit Haken, «Alle markieren», Übernehmen unten (B9.2) |
| `Main.dc.html` | Angebot A: Pakete nach Land, Sprache, Genre, Beliebtheit (nicht gewählt) |
| `AngebotC.dc.html` | Angebot C: Regeln «Nimm alle Sender mit» und «Lass weg» (nicht gewählt) |
| `Uebersicht.dc.html` | Übersicht mit Aufräumen, Zusammensetzung, Automatik (Vorstufe von B9.1) |
| `Sender.dc.html` | Sender mit Anbieter, «Alle nicht erreichbaren entfernen», Herkunft je Angabe (Vorstufe von B9.3) |
| `MeineSender.dc.html` | «Meine Sender statt Katalog», Radioseite der App (nicht gewählt) |
| `VerwaltungNeu.dc.html` | «Meine Sender statt Katalog», Verwaltung (nicht gewählt) |

Die Zahlen auf `AngebotB` stammen aus dem Katalog vom 9./10. November 2026, nach Namen
zusammengeführt; die Schweiz ist vollständig, sonst stehen die 30 meistgehörten Sender.
