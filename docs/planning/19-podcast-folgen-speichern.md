# Konzept: Podcast-Folgen speichern und neue Podcasts-Seite

- **Status: Fassung 3 (10. November 2026), Kritiker-Review eingearbeitet (REVISE, 0 Blocker,
  13 Major, 6 Minor; Abschnitt 18), mit klickbaren Mockups.** Marco hat am 9. November 2026 die
  Grundsatzfragen **F-1 bis F-4** beantwortet (Abschnitt 2) und nach dem ersten Blick auf die Mockups
  entschieden: der Umfang hat **drei** Werte («Nur neue Folgen», «Neue + letzte N», «Alle Folgen»),
  «Gleichzeitig laden» geht **bis 5**, die abonnierten Podcasts erscheinen als **Liste mit Covern**
  (F-5, Variante A), die Einstellung steht als **Band auf der Podcasts-Seite** (F-6, Variante B).
  Offen sind nur die Punkte in Abschnitt 17.
  **Die Umsetzung beginnt erst auf Marcos ausdrückliches Wort.**
- **Auslöser:** Marco will, dass Slouchcat neue Podcast-Folgen als Datei in den Ordner des Podcasts
  speichert, statt nur den Link zum Anbieter zu halten. Vorhandene Dateien sollen wie heute erkannt
  und ausgelesen werden. In der Verwaltung soll wählbar sein, ob neue Folgen gespeichert oder nur
  verlinkt werden. Dazu soll die Seite «Podcasts» der Verwaltung schöner werden; er wollte Mockups für
  die Einstellung und für die abonnierten Podcasts, klickbar.
- **Verankert gegen:** `main` **`e90109a6`** («Merge pull request #131», Patch 1.23.1),
  Vertragsfassung **1.23.0** (`api/openapi.yaml`, `info.version`), letzte Migration in `main`
  **`00043_trickplay_off.sql`**, letzte ADR **`0028`**, letzte Frage im Entscheidungsprotokoll **25**.
- **Nummern:** Dieses Papier ist **Konzept 19**, reserviert **Frage 26** und **ADR 0029**
  («Podcast-Folgen als Datei: schreibbare Podcast-Ordner»). Vorläufig gelten **Vertrag 1.24.0
  (additiv)** für Stufe 1 und **1.25.0 (additiv)** für Stufe 2 sowie **Migration 00044**. Die echten
  Nummern werden beim Baubeginn vergeben.
- `:Zeile`-Angaben beziehen sich auf `e90109a6`; **nach Namen suchen, nicht nach Zeile.** Pfade ohne
  Präfix liegen unter `server/` (Go) oder `web/src/` (Web-App), wo es eindeutig ist.
- Massgeblich bleiben `docs/spec/01-anforderungen-v1.md`, `docs/planning/04-entscheidungen.md` und
  `docs/adr/`. Bei Widerspruch gilt: **Entscheidungsprotokoll > Plan > Spec** (`CLAUDE.md`). Dieses
  Papier hebt einen Satz der Spec teilweise auf (B-1 «read-only», Abschnitt 3); erst Marcos Antwort
  auf Frage 26 ändert ihn.

> **Dieses Papier ändert keine Zeile Code.** Es beschreibt, wie der Server Folgen in den
> Podcast-Ordner lädt, wie die Einstellung dazu aussieht und wie die Seite «Podcasts» neu gebaut
> wird. Abschnitt 15 teilt den Bau in zwei Stufen, die je als Release erscheinen.

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

Eingefrorener Datenbankstand nach der Migration (`testdata/db/`), Vertragstest für jede neue
Operation, `task storybook:test` für das Cover.

## 17. Offene Punkte

| Nr. | Punkt | Empfehlung |
|---|---|---|
| O-1 | ~~Lesart von «Neue und fehlende Folgen»~~ | **erledigt** (9. November 2026): Marco hat den Wert gestrichen |
| O-2 | Stand nach dem Update: «Nur Link», bis Marco umstellt? | ja; kein Download ohne sein Zutun |
| O-3 | Formate, die der Scanner nicht liest (`aac`, `wav`, Video-Podcasts als `mp4`; auf dem NAS liegen 10 MP4): Link lassen? | ja, mit Hinweis «Format wird nicht gespeichert»; Video-Podcasts sind ein eigenes Thema |
| O-4 | ~~«Gleichzeitig laden» als Einstellung oder fest?~~ | **erledigt** (9. November 2026): Einstellung, 1 bis 5 |
| O-5 | Neue Folge sofort beim Feed-Abruf melden oder erst, wenn die Datei da ist? | sofort, die Folge ist als Stream hörbar |
| O-6 | Bandbreite begrenzen? | nein; Zeitfenster und «Gleichzeitig laden» genügen |
| O-7 | Tags in die Datei schreiben (Titel, Datum)? | nein; die Datei bleibt, wie der Anbieter sie liefert |
| O-8 | ~~Schlägt die Sonde einer geladenen Datei fehl: Datei löschen?~~ | **erledigt** (Review, Befund 3): die Sonde läuft vor dem Eintragen auf der eigenen Teildatei; was durchfällt, kommt nie in den Ordner |
| O-9 | ~~Mehrfachauswahl (Variante C) zusätzlich zu A?~~ | **erledigt** (9. November 2026): Marco hat A gewählt; Mehrfachauswahl erst bei Bedarf |
| O-10 | «Je Anbieter einer»: Podcasts hinter demselben Zähl-Dienst laden nacheinander. Reicht das, oder soll der Host nach der Weiterleitung zählen? | vor der Weiterleitung (einfach, schonend); M-1 zählt die Hosts, danach entscheiden |
| O-11 | Hört Marco nach dem Wechsel weiter mit Audiobookshelf? Dann müssen dessen Benutzer die neuen Dateien lesen können (6.4). | Rechte 0664 und 0775 genügen dafür; bei der Abnahme prüfen |
| O-12 | Dateisysteme ohne harte Links (manche SMB-Freigaben): «Speichern» gesperrt. Genügt das? | ja; Marcos Ordner liegt auf NFS |

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

Die Zahlen in den Mockups sind Beispiele; gemessen sind nur die Werte in Abschnitt 4. Die Zustände
«Entfernt» und «Unklar» und die Gründe der Ordnerprüfung sind in den Mockups noch nicht gezeichnet.
