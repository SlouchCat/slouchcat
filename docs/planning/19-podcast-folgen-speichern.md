# Konzept: Podcast-Folgen speichern und neue Podcasts-Seite

- **Status: Fassung 1 (9. November 2026), mit klickbaren Mockups.** Marco hat am 9. November 2026 die
  Grundsatzfragen **F-1 bis F-4** beantwortet (Abschnitt 2). Offen sind die Wahl der Mockup-Varianten
  (**F-5, F-6**) und die Punkte **O-1 bis O-9** (Abschnitt 16). Ein Kritiker-Review steht noch aus.
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
> wird. Abschnitt 14 teilt den Bau in zwei Stufen, die je als Release erscheinen.

## 1. Was Marco will

| Nr. | Marcos Wunsch (9. November 2026, sinngemäss) | Abschnitt |
|---|---|---|
| W-1 | Slouchcat speichert neue Folgen als Datei im Ordner des Podcasts. | 6, 7 |
| W-2 | Bestehende Dateien werden wie jetzt erkannt, alle Daten ausgelesen. | 8 |
| W-3 | In der Verwaltung gibt es eine Option: neue Folgen speichern oder nur den Link bereitstellen. | 5, 11 |
| W-4 | Die Verwaltung der Podcasts wird schöner dargestellt. | 11 |
| W-5 | Mockups für die Einstellung und für die abonnierten Podcasts, klickbar. | 11, Anhang A |

## 2. Interview vom 9. November 2026

| Nr. | Frage | Marcos Antwort |
|---|---|---|
| F-1 | Wo wird eingestellt, ob gespeichert oder verlinkt wird? | **Je Bibliothek, je Podcast abweichend** (Empfehlung). |
| F-2 | Was wird geladen, wenn Speichern eingeschaltet ist? | **Eine Auswahl:** «Alle Folgen», «Neue und fehlende Folgen», «Nur neue Folgen», «Neue + letzte N». |
| F-3 | Audiobookshelf lädt heute noch in denselben Ordner. Wie weiter? | **Slouchcat übernimmt.** Marco schaltet die automatischen Downloads in Audiobookshelf ab; Slouchcat schreibt im selben Layout. |
| F-4 | Wie lange bleiben gespeicherte Folgen? | **Für immer.** Slouchcat löscht nie eine Datei im Podcast-Ordner. |
| F-5 | Welche Variante für die abonnierten Podcasts (A Liste, B Raster, C Tabelle)? | **offen**, Empfehlung A (11.2) |
| F-6 | Welche Variante für die Einstellung (A Dialog in der Bibliothek, B Band auf der Podcasts-Seite)? | **offen**, Empfehlung A mit dem Abschnitt «Downloads» aus B (11.3) |

## 3. Was sich gegenüber früheren Entscheiden ändert

| Bisher | Neu mit diesem Konzept | Grund |
|---|---|---|
| Spec **B-1**: Der Admin «bindet Medien read-only ein». `deploy/compose.yaml` bindet `${MEDIA_DIR}` mit `:ro` ein. | **Ausnahme Podcast-Ordner:** ein Ordner mit `content = 'podcast'` darf schreibbar eingebunden sein; nur dorthin schreibt der Server. Film-, Serien- und Hörbuchordner bleiben read-only. | W-1, F-3 |
| Entscheid vom 21. Oktober 2026 (Sicherung): «Medienwurzeln bleiben read-only (B-1)», kein zweiter schreibbarer Mount. | Für die Sicherung gilt das weiter. Neu gibt es genau einen schreibbaren Medien-Mount, den Podcast-Ordner. | W-1 |
| ADR 0007: Eine Feed-Episode hat keine Datei, sie wird über den Server vom Anbieter gestreamt (`ProxyEnclosure`). | Bleibt für «Nur Link» und als Übergang, bis die Datei da ist. Bei «Speichern» bekommt die Feed-Episode eine `media_files`-Zeile. | W-1, W-3 |
| ADR 0015: Podcast-Ordner werden nur gelesen; eine Datei entsteht ausserhalb (Audiobookshelf) und findet über `finishPodcastEpisode` zu ihrer Feed-Episode. | Bleibt für fremde Dateien. Dateien, die Slouchcat selbst schreibt, trägt der Download direkt an der Feed-Episode ein, ohne Umweg über das Verheiraten. | W-1, W-2 |
| Der Server schreibt nur nach `/config` und `/cache` (Befund in Abschnitt 4). | Er schreibt zusätzlich in Podcast-Ordner: nur neue Dateien, nie überschreiben, nie löschen. | W-1, F-4 |

Unberührt bleiben Regel 2 (Titelabfragen nur über `visibility`; der Download läuft im Paket
`library`, das auf der Liste steht), Regel 3 (`os/exec` nur in `transcode`), Regel 12 (kein
Entscheid im Client) und das Nicht-Ziel «Download von YouTube-Inhalten»: geladen wird nur die
Enclosure eines abonnierten Feeds.

## 4. Befund: Stand heute

**Code (`e90109a6`):**

- `metadata.Service.FetchPodcast` und `storeEpisodes` (`internal/metadata/podcast.go`) legen je
  Feed-Eintrag ein `podcast_episode`-Item mit `enclosure_url` an. Abruf alle 55 Minuten über
  `podcast.poll` und `podcast.fetch`.
- Abspielen: `playback.Service.Start` nimmt zuerst die Datei (`loadInput`); nur ohne verfügbare
  Datei startet `startEnclosure` einen Durchreich-Stream (`ProxyEnclosure`). **Eine Episode mit Datei
  spielt also heute schon immer die Datei.** Der Geräte-Download (`offline.audioSource`) hält es
  gleich.
- Podcast-Ordner: `library/podcast_folder.go` liest je Unterordner `metadata.json` und Cover, jede
  Audiodatei wird eine Datei-Episode. `finishPodcastEpisode` (`library/probe.go`) verheiratet sie
  nach der Sonde mit der Feed-Episode, wenn genau eine passt (gleiche Minute oder gleicher Titel,
  Dauer ±2 %), sonst bleiben zwei Episoden (`file_and_feed` in den Pflegewerkzeugen).
- Eine Enclosure laden kann der Server schon: `offline.fetchSource` (`internal/offline/audio.go`)
  lädt sie für Geräte-Downloads in den Cache, mit Wiederaufnahme (`.part`, `Range`, `If-Range`),
  Grenze 2 GiB, Netzwächter (`netguard.Clients.For` mit `feed_origin`). Die Funktion ist privat und
  an den Download-Ordner gebunden.
- Kein Code schreibt heute in eine Bibliothekswurzel. `paths.Open` liefert ein `*os.Root`; damit ist
  Schreiben innerhalb der Wurzel möglich und ein Ausbruch aus ihr nicht.
- Der Scanner erkennt Dateien an `root_id + rel_path`, Grösse und `mtime`. `Ignored`
  (`library/names.go`) übergeht Namen mit führendem Punkt; eine Regel für `.part` gibt es nicht.
  Gescannte Audio-Endungen (`audioExts`): `m4b`, `m4a`, `mp3`, `flac`, `ogg`, `opus`.
- Die Seite «Podcasts» (`features/admin/PodcastsPage.tsx`) ist eine Tabelle mit Titel, Episoden,
  letztem Abruf und Fehler; aufgeklappt Autor, Feed-Adresse, Bibliothek, «Jetzt abrufen».

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
| Bibliothek (Art Podcasts) | **Neue Folgen:** `save` «Speichern» oder `link` «Nur Link». **Umfang** (nur bei `save`): `new`, `gaps`, `last_n` (mit N von 1 bis 100), `all`. **Ordner für neue Abos:** einer der Podcast-Ordner der Bibliothek. **Gleichzeitig laden:** 1 bis 3. | `link`, Umfang `new`, N 10, erster Podcast-Ordner, 2 |
| Podcast | **Neue Folgen:** «Wie Bibliothek», «Speichern», «Nur Link». **Umfang:** «Wie Bibliothek» oder einer der vier Werte. | beide «Wie Bibliothek» |

Ab Werk steht `link`: nach dem Update verhält sich Slouchcat genau wie heute, bis Marco «Speichern»
wählt (O-2). Es gilt die Einstellung des Podcasts, wenn er eine eigene hat, sonst die der Bibliothek.
Der Server rechnet das aus und liefert das Ergebnis mit (`effectiveMode`, `effectiveScope`); der
Client rechnet nichts (Regel 12).

### 5.2 Was die vier Umfänge heissen

| Umfang | Geladen wird | Beispiel «Weltwoche Daily» (2'017 im Feed, 545 Dateien) |
|---|---|---|
| **Nur neue Folgen** (`new`) | Folgen, die ein Feed-Abruf **nach dem Einschalten** neu anlegt. | nur was ab jetzt erscheint |
| **Neue und fehlende Folgen** (`gaps`) | Wie `new`, dazu **Lücken**: Folgen ohne Datei, die jünger sind als die älteste gespeicherte Datei des Podcasts. | zusätzlich die Lücken seit der ältesten Datei |
| **Neue + letzte N** (`last_n`) | Wie `new`, dazu die N jüngsten Folgen des Feeds, soweit sie keine Datei haben. | zusätzlich höchstens 10 |
| **Alle Folgen** (`all`) | Jede Folge des Feeds ohne Datei (der Server hält höchstens die 2'000 jüngsten je Feed). | zusätzlich rund 1'470 Folgen, etwa 36 GB (seine Dateien haben im Mittel 25 MB) |

Die Lesart von «Neue und fehlende» ist meine Auslegung von Marcos Antwort und steht als **O-1** zur
Bestätigung. Ohne sie wäre «fehlend» dasselbe wie «alle».

«Nach dem Einschalten» hält der Server je Podcast als Zeitpunkt fest (`save_since`): gesetzt, sobald
«Speichern» für diesen Podcast wirksam wird, gelöscht bei «Nur Link». Eine Folge gilt als neu, wenn
ihr Item nach `save_since` entstand und sie die heutige Regel für «neue Episode» erfüllt
(veröffentlicht nach der bisher jüngsten; `storeEpisodes`). Ein Feed, der seine GUIDs erneuert oder
alte Folgen nachreicht, löst so keine Lawine aus.

### 5.3 Was eine Änderung bewirkt

| Änderung | Wirkung |
|---|---|
| `link` → `save` | `save_since` = jetzt; der Planer (6.1) läuft für die betroffenen Podcasts und reiht je nach Umfang Folgen ein. |
| `save` → `link` | Wartende Downloads werden abgebrochen, laufende beendet der Abbruch-Haken und entfernt die Teildatei. **Gespeicherte Dateien bleiben** (F-4). |
| Umfang grösser (z. B. `new` → `all`) | Der Planer reiht die zusätzlichen Folgen ein. Vor dem Speichern zeigt der Dialog, wie viele Folgen und wie viel Platz das ungefähr sind. |
| Umfang kleiner | Wartende Downloads, die nicht mehr dazugehören, werden abgebrochen. Nichts wird gelöscht. |
| Knopf «Fehlende nachladen» am Podcast | Einmalig alle Folgen des Feeds ohne Datei, gleich welcher Umfang gilt. Auch einzeln je Folge («Laden»). |

### 5.4 Der Ordner muss schreibbar sein

«Speichern» lässt sich nur wählen, wenn der Zielordner schreibbar ist. Der Server prüft es mit einer
Testdatei (`.slouchcat-schreibtest`, sofort wieder entfernt) beim Öffnen des Dialogs, beim Speichern
(422 `root_not_writable`) und vor jedem Download. Ist der Ordner nur lesbar eingebunden, sagt der
Dialog das und verweist auf die Installationsanleitung (Abschnitt 12).

## 6. Vom Feed zur Datei

```
podcast.fetch (wie heute)          podcast.sync (neu)               podcast.download (neu)
Feed lesen, Episoden anlegen ──▶   Einstellung + Umfang auswerten ──▶ je Folge: laden, umbenennen,
neue Episode melden (wie heute)    Folgen einreihen                  an der Feed-Episode eintragen,
                                                                      Sonde einreihen
```

### 6.1 Der Planer `podcast.sync`

Ein Job je Podcast, eingereiht nach jedem Feed-Abruf mit Inhalt, nach jeder Änderung der Einstellung
und durch die Knöpfe. Er lebt im Paket `library` (dort entstehen `media_files`) und wird von
`metadata` nur mit Namen und `podcastId` eingereiht; `metadata` importiert `library` weiterhin nicht.

Der Planer bestimmt die wirksame Einstellung, wählt nach 5.2 die Folgen ohne Datei aus und reiht je
Folge `podcast.download` ein (Idempotenz-Schlüssel `podcast-dl:<episodeId>`). Er lässt aus:

- Folgen, deren Download läuft, wartet oder vor weniger als 6 Stunden fehlschlug,
- Folgen im Zustand `removed` (5.5 in Abschnitt 8: die Datei wurde von Hand gelöscht),
- Folgen, zu denen im Ordner schon eine passende Datei liegt (Abschnitt 8),
- Folgen mit einem Format, das der Scanner nicht liest (O-3).

### 6.2 Der Download `podcast.download`

| Schritt | Was geschieht |
|---|---|
| 1. Prüfen | Episode sichtbar vorhanden, ohne Datei, Einstellung verlangt sie noch. Ordner schreibbar, freier Platz über der Schwelle (wie Trickplay: das Kleinere von 10 GiB und 10 %). Sonst `RetryLater` mit Grund, ohne einen Versuch zu verbrauchen. |
| 2. Ordner | Der Podcast hat schon einen Ordner (aus seinen vorhandenen Dateien): der bleibt. Sonst legt der Server `<Ordner für neue Abos>/<Podcast-Titel>/` an und schreibt `metadata.json` (Audiobookshelf-Schema mit `feedURL`) und `cover.jpg` hinein. Eine vorhandene `metadata.json` wird nie überschrieben. |
| 3. Laden | In eine Teildatei **im selben Ordner**: `.slouchcat-<episodeId>.part`. Der führende Punkt hält Scanner und Watcher fern (`Ignored`); derselbe Ordner macht das Umbenennen atomar, auch über NFS. Wiederaufnahme, Grenze 2 GiB, Netzwächter und Weiterleitungsregel wie beim Geräte-Download. |
| 4. Plausibilität | Grösse über null; Antwort ist kein HTML (viele Anbieter liefern bei Fehlern eine Seite mit Status 200); Endung aus Inhaltstyp oder Adresse gehört zu `audioExts`. Sonst Fehlschlag, Teildatei weg. |
| 5. Name | `<Folgentitel>.<Endung>`, bereinigt (6.3). Existiert der Name schon, wird das Datum vorangestellt (`2026-11-09 <Folgentitel>.mp3`), dann eine Zahl. Angelegt wird mit «nur wenn es die Datei nicht gibt»; **überschrieben wird nie.** |
| 6. Eintragen | Unter der Scan-Sperre der Bibliothek (`tryLockScan`): umbenennen und in einer Transaktion die `media_files`-Zeile **an der Feed-Episode** anlegen (Wurzel, Pfad, Grösse, `mtime`, Gerät und Inode). Ein Scan, der die Datei danach sieht, kennt sie schon. Dann `probe.file` einreihen. |
| 7. Fertig | Die Trigger setzen `available`; ab jetzt spielt die Folge die Datei. Fortschritt, Favorit und Gesehen-Haken hängen am selben Item und bleiben. |

Die Ladelogik zieht aus `offline.fetchSource` in ein gemeinsames Paket um (Arbeitsname
`internal/fetch`: Wiederaufnahme, Grenzen, `contentRange`, Endungen), das `offline` und `library`
benutzen. Verhalten und Tests des Geräte-Downloads bleiben gleich.

**Warteschlange:** eigener Pool `podcast` mit drei Arbeitern; die Einstellung «Gleichzeitig laden»
(1 bis 3) begrenzt, wie viele davon gleichzeitig laden. Der Pool `download` (ein Arbeiter) bleibt
den Geräte-Downloads, damit ein Nachladen von tausend Folgen niemanden am Handy warten lässt. Je
Anbieter-Host läuft höchstens ein Download gleichzeitig (`ratelimit`). Die Jobart ist `Windowable`:
auf «Hintergrundjobs» lässt sich ein Zeitfenster setzen, pausieren und abbrechen wie bei jeder
Jobart (ADR 0017). Der Abbruch-Haken entfernt die Teildatei. Fortschritt meldet `jobs.Progress` in
Bytes.

**Fehler:** fünf Versuche mit wachsender Pause (Jobsystem), danach Zustand `failed` mit Code
(`enclosure_unavailable`, `too_large`, `not_audio`, `no_space`, `root_not_writable`). Der Planer
versucht eine fehlgeschlagene Folge höchstens alle 6 Stunden und sieben Tage lang erneut; danach
bleibt sie `failed`, bis jemand «Erneut» drückt. **Die Folge bleibt in jedem Fall als Link
abspielbar.**

### 6.3 Dateinamen

Aus dem Folgentitel: Steuerzeichen und `/ \ : * ? " < > |` werden zu Leerzeichen, mehrfache
Leerzeichen zu einem, Punkte und Leerzeichen am Ende fallen weg, gekürzt auf 180 Bytes an einer
Zeichengrenze. Leer oder nur Punkte: `Folge <Datum>`. Der Name wird über `os.Root` relativ zum
Podcast-Ordner angelegt; ein Titel kann den Ordner nicht verlassen (Regel 4).

Bestehende Ordner werden nie umbenannt und nicht aus dem Titel errechnet: der Ordner eines Podcasts
steht in der Datenbank, sobald er bekannt ist (aus dem Pfad seiner Dateien oder beim Anlegen).

## 7. Abspielen, Benachrichtigen, Geräte-Downloads

- **Abspielen:** keine Änderung am Code der Wiedergabe. Solange die Datei fehlt, streamt die Folge
  vom Anbieter; sobald die `media_files`-Zeile da ist, nimmt `Start` die Datei. Läuft gerade ein
  Stream, wenn die Datei fertig wird, läuft er zu Ende; der nächste Start nimmt die Datei.
- **Neue Folge melden:** wie heute beim Feed-Abruf, nicht erst nach dem Download (O-5). Die Folge
  ist sofort hörbar.
- **Geräte-Downloads (O-3 der Spec):** unverändert; `audioSource` nimmt die Datei, wenn sie da ist,
  sonst die Enclosure.
- **Sichtbarkeit:** unverändert. Die Datei ändert weder Freigabe noch Bibliothek der Folge.
- **Zuschauer-Vertrag:** kein `scoped`-Schema ändert sich.

## 8. Bestehende Dateien und der Wechsel von Audiobookshelf

**Erkennen bleibt, wie es ist (W-2).** Scanner, `metadata.json`, Cover, Tags und das Verheiraten
über `finishPodcastEpisode` ändern sich nicht. Dateien, die Marco von Hand in einen Podcast-Ordner
legt, werden weiter erkannt.

**Das Risiko doppelter Dateien.** Heute bleiben Datei- und Feed-Episode getrennt, wenn das
Verheiraten kein eindeutiges Paar findet. Für den Planer sähe die Feed-Episode dann aus wie «ohne
Datei», und bei «Alle Folgen» oder «Neue und fehlende» lüde er eine Folge, die schon im Ordner
liegt. Dagegen stehen zwei Riegel:

1. **Abgleich über den Dateinamen.** Vor dem Einreihen prüft der Planer, ob im Ordner eine
   Datei-Episode ohne GUID liegt, deren Dateiname (ohne Endung, bereinigt nach 6.3) dem bereinigten
   Folgentitel gleicht. Audiobookshelf benennt genau so (Abschnitt 4). Bei genau einem Treffer wird
   **nicht geladen, sondern verheiratet** (derselbe Weg wie `finishPodcastEpisode`: Datei an die
   Feed-Episode, Datei-Episode weg).
2. **Vorsicht bei Zweifel.** Gibt es zu einer Feed-Episode eine Datei-Episode, die nach der
   heutigen Regel passen könnte, aber nicht eindeutig ist (zwei Kandidaten), lädt der Planer nicht
   und zeigt die Folge auf der Seite als «unklar: Datei vorhanden?» mit dem Verweis auf die
   Pflegewerkzeuge.

Wie viele solcher ungepaarten Paare es heute gibt, ist nicht gemessen. **Messauftrag M-1 vor dem
Bau:** auf CT 190 je Podcast zählen: Feed-Episoden ohne Datei, Datei-Episoden ohne GUID, und wie
viele davon der Dateinamen-Abgleich paart. Das Ergebnis kommt nach `docs/perf/19-podcasts.md` und
entscheidet, ob Riegel 1 reicht.

**Von Hand gelöschte Dateien.** Verschwindet eine Podcast-Datei (der Sweep markiert sie als
fehlend), bekommt die Folge den Zustand `removed`. Der Planer lädt sie nicht von selbst wieder, auch
nicht bei «Alle Folgen»; «Laden» an der Folge holt sie zurück. So bleibt Aufräumen von Hand
wirksam (F-4: Slouchcat löscht nichts, aber es macht Marcos Löschen auch nicht rückgängig).

**Der Wechsel (F-3), in dieser Reihenfolge:**

1. Podcast-Ordner schreibbar einbinden (Abschnitt 12), Update einspielen. Einstellung steht auf
   «Nur Link», nichts ändert sich.
2. In Audiobookshelf je Podcast den automatischen Download abschalten.
3. In Slouchcat die Bibliothek auf «Speichern» stellen, Umfang nach Wahl.

Zwischen 2 und 3 erschienene Folgen fängt «Neue und fehlende» oder der Knopf «Fehlende nachladen».
Laufen beide eine Weile parallel, verhindert Schritt 5 in 6.2 (nie überschreiben) kaputte Dateien,
nicht aber zwei Dateien mit leicht anderem Namen; darum die Reihenfolge.

**Abos ohne Ordner** (von Hand abonniert, sieben der 102 Ordner haben umgekehrt keinen Feed):
bekommen beim ersten Download einen Ordner mit `metadata.json` und Cover (6.2, Schritt 2). Findet
ein späterer Scan diesen Ordner, erkennt er über die `feedURL` das bestehende Abo
(`findOrCreatePodcast`) und legt keinen zweiten Podcast an. `feed_origin` bleibt `admin`.

## 9. Datenmodell (Migration 00044, vorläufig)

Nach Vorlage (`server/migrations/TEMPLATE.sql.txt`), nur vorwärts.

| Wo | Was |
|---|---|
| `libraries` | `podcast_mode` (`'save'`, `'link'`; Standard `'link'`), `podcast_scope` (`'new'`, `'gaps'`, `'last_n'`, `'all'`; Standard `'new'`), `podcast_last_n` (1 bis 100, Standard 10), `podcast_root_id` (nullable, Fremdschlüssel auf `library_roots`), `podcast_parallel` (1 bis 3, Standard 2). Per `ALTER TABLE … ADD COLUMN … CHECK`, wie die Trickplay-Spalten in 00042. |
| neu `podcast_settings` | `item_id` (Podcast, Primärschlüssel), `mode` (nullable: `NULL` = wie Bibliothek), `scope` (nullable), `last_n` (nullable), `save_since` (nullable), `root_id` und `folder` (der Ordner des Podcasts, sobald bekannt). |
| neu `podcast_downloads` | `item_id` (Episode, Primärschlüssel), `state` (`queued`, `running`, `failed`, `removed`), `error` (Code), `attempts`, `bytes_done`, `bytes_total`, `updated_at`. Eine Zeile gibt es nur, solange etwas zu sagen ist; «gespeichert» ergibt sich aus `media_files`, «nur Link» aus deren Fehlen. |

Eigene Tabellen statt Spalten an `items`: die Einstellung betrifft 102 von vielen tausend Zeilen,
und Zustandswechsel eines Downloads sollen die Trigger an `items` nicht auslösen. Beide Tabellen
liest und schreibt nur das Paket `library` (Regel 2: Abfragen mit `items` und `media_files` sind
dort erlaubt). Ein eingefrorener Stand kommt nach `testdata/db/` (Regel 5).

## 10. Vertrag (1.24.0 und 1.25.0, additiv)

Jede Operation trägt `x-visibility: admin` und bekommt ihren Eintrag im Vertragstest (Regel 7).

| Stufe | Änderung |
|---|---|
| 1 | `Library.podcastEpisodes` und `LibraryUpdate.podcastEpisodes` (nur Art Podcasts, sonst 422 `not_allowed`): `mode`, `scope`, `lastN`, `rootId`, `parallel`. Muster wie `Library.trickplay`. |
| 1 | `LibraryRoot.writable` (bool) und `LibraryRoot.freeBytes`, nur in `getLibrary` (dort läuft der Schreibtest). |
| 1 | `AdminPodcast` wächst um `mode`, `scope` (je `inherit` oder Wert), `effectiveMode`, `effectiveScope`, `storedBytes`, `missingEpisodeCount`, `downloads` (`queued`, `running`, `failed`), `latestEpisodeAt`, `folder`. |
| 1 | neu `updatePodcast` (`PATCH /admin/podcasts/{id}`): `mode`, `scope`, `lastN`. |
| 1 | neu `downloadPodcastEpisodes` (`POST /admin/podcasts/{id}/download`): alle fehlenden oder eine Liste von Episoden; 202. 409 `mode_link`, wenn der Podcast auf «Nur Link» steht und keine Liste mitkommt. |
| 2 | neu `listAdminPodcastEpisodes` (`GET /admin/podcasts/{id}/episodes`, seitenweise): Schema `AdminPodcastEpisode` mit Titel, Datum, Dauer, Grösse, `state` (`file`, `link`, `queued`, `running`, `failed`, `removed`, `unclear`), Fortschritt, Fehlercode, Dateiname. Eigenes Schema, kein `ItemRef` (das wäre `scoped`). |
| 2 | neu `cancelPodcastDownload` (`DELETE /admin/podcasts/{id}/episodes/{episodeId}/download`). |
| 2 | neu `listPodcastDownloads` (`GET /admin/podcasts/downloads`): die Warteschlange für den Abschnitt «Downloads». |
| 2 | `AdminPodcast.coverUrl` für das Cover in der Liste (Bild-Endpunkt wie für Titelbilder, als `http.Handler`, Regel 6). |
| 2 | nur mit Variante C: `updatePodcasts` (Mehrfachauswahl, eine Liste von Ids mit denselben Feldern). |

Neue Fehlercodes: `root_not_writable`, `root_required` (Bibliothek ohne Podcast-Ordner),
`mode_link`.

## 11. Oberfläche

Mockups: private Leinwand «Konzept 19 Podcasts»
(`https://claude.ai/artifact/CL7J1ag3XYU5YftBd8avyA`), Kopie der Artboards in
`docs/planning/mockups/19-podcasts/`. Alles ist klickbar; die Leiste «Mockup» oben in jedem Bild
springt zwischen den Varianten. Gezeigt sind 14 von 94 Podcasts, Zahlen und Cover sind Platzhalter.

### 11.1 Was jede Variante zeigt

Allen gemeinsam, neu gegenüber heute: Cover, «Speichert» oder «Nur Link» je Podcast (mit Kennzeichen,
wenn er von der Bibliothek abweicht), gespeicherte und fehlende Folgen, belegter Platz, neueste
Folge, Zustand (aktuell, lädt, Feed-Fehler).

### 11.2 Abonnierte Podcasts (F-5)

| Variante | Idee | Stärke | Schwäche |
|---|---|---|---|
| **A Liste mit Covern** (`Main`) | Eine Zeile je Podcast mit Cover, Kennzahlen oben, Filter «Speichert / Nur Link / Lädt / Fehler», Suche, Sortierung. Aufgeklappt: Einstellung, Feed, Knöpfe. | Alles Wichtige auf einen Blick, Einstellung ohne Seitenwechsel, läuft auch am Handy. | Bei 94 Podcasts lang; Mehrfachänderung nur einzeln. |
| **B Raster mit Seitenfeld** (`AbosB`) | Cover-Kacheln, rechts das Feld des gewählten Podcasts mit Einstellung und letzten Folgen. | Schön, schnell zu überblicken, wenn man die Cover kennt. | Wenig Zahlen je Kachel; das Seitenfeld rutscht am Handy unter das Raster. |
| **C Tabelle mit Mehrfachauswahl** (`AbosC`) | Wie heute, dazu Cover, «Neue Folgen» als Auswahl in der Zeile, sortierbare Spalten, Häkchen und Sammelaktionen. | Am dichtesten; 20 Podcasts in einem Zug umstellen. | Am wenigsten «schöner»; am Handy seitlich scrollen. |

**Empfehlung: A.** Sie erfüllt W-4 am besten und bleibt am Handy bedienbar. Die Mehrfachauswahl aus C
lässt sich später nachrüsten, wenn sie fehlt; mit der Vorgabe je Bibliothek braucht es sie selten.

In jeder Variante führt ein Klick zum **einzelnen Podcast** (`Podcast`): eigene Seite mit Cover,
Einstellung und der Folgenliste. Je Folge steht «Gespeichert», «Nur Link», «Lädt», «Fehlgeschlagen»
mit «Laden», «Abbrechen», «Erneut»; oben «N fehlende nachladen».

### 11.3 Die Einstellung (F-6)

| Variante | Idee | Stärke | Schwäche |
|---|---|---|---|
| **A Dialog in der Bibliothek** (`Einstellung`) | Wie «Vorschau beim Spulen»: unter Bibliotheken, mit Erklärung je Wahl, Schreibtest des Ordners, «So läuft es» und Schätzung vor dem Speichern. | Gleiches Muster wie Trickplay; Platz für Erklärung und Warnung; der Ordner gehört zur Bibliothek. | Ein Seitenwechsel von der Podcasts-Seite (ein Knopf «Einstellungen» führt hin). |
| **B Band auf der Podcasts-Seite** (`EinstellungB`) | Ein Band zuoberst, zugeklappt eine Zeile Zusammenfassung, aufgeklappt Knöpfe; jede Änderung gilt sofort. Darunter der Abschnitt «Downloads» mit Warteschlange, Anhalten, Abbrechen. | Einstellung und Wirkung auf einer Seite. | Wenig Platz für Erklärung; «gilt sofort» ist bei «Alle Folgen» heikel (Dutzende GB mit einem Klick). |

**Empfehlung: A, dazu der Abschnitt «Downloads» aus B auf der Podcasts-Seite.** Ein Umfang wie
«Alle Folgen» soll eine Schätzung zeigen und einen Knopf «Speichern» brauchen. Die Warteschlange
gehört dorthin, wo man die Podcasts ansieht.

### 11.4 Regeln

Die neue Seite «Podcast» bekommt einen Playwright-Tastaturtest, die umgebaute Seite «Podcasts»
behält ihren (Regel 8). Farben nur aus Tokens, Texte nur aus `locales/{de,en}.json` (Regel 9).
Bausteine aus `web/src/ui`: `AdminPage`, `BandPanel`, `DataTable`, `Chip`, `RadioGroup`,
`ProgressBar`, `StorageMeter`, `FormDialog`. Neu ist nur ein quadratisches Cover für
Verwaltungslisten (heute gibt es `PosterTitle` im Hochformat und `StationLogo`).

## 12. Betrieb: der Ordner wird schreibbar

Heute bindet `deploy/compose.yaml` alle Medien mit `:ro` ein. Neu kommt eine zweite, genauere Zeile
dazu, die **nur den Podcast-Ordner** schreibbar über den read-only Baum legt:

```yaml
    volumes:
      - ${MEDIA_DIR:-/srv/media}:/media:ro
      # Podcast folder only: writable, so new episodes can be stored (concept 19, ADR 0029).
      - ${PODCAST_DIR:-/srv/media/podcasts}:/media/podcasts
```

Die Zeile ist freiwillig: ohne sie bleibt alles read-only, und «Speichern» ist in der Verwaltung
nicht wählbar. `docs/INSTALLATION.md` bekommt einen Abschnitt dazu.

Für CT 190 heisst das drei Dinge, die Marco selbst macht: der NFS-Export des NAS erlaubt Schreiben
für diesen Pfad, der Mount im LXC ist nicht `ro`, und die Benutzer-Zuordnung des unprivilegierten
Containers darf im Ordner Dateien anlegen (die Ordner haben heute 777, neue Dateien von
Audiobookshelf gehören `100999:100991`). Der Schreibtest der Verwaltung (5.4) sagt sofort, ob es
stimmt.

## 13. Sicherheit

| Gefahr | Riegel |
|---|---|
| Der Server schreibt ausserhalb des Podcast-Ordners. | Schreiben nur über `os.Root` der Wurzel (`paths.Open`), nur in Wurzeln mit `content = 'podcast'`; Namen bereinigt (6.3). |
| Eine Datei wird überschrieben oder gelöscht. | Anlegen nur exklusiv; der Server löscht in der Wurzel nur seine eigenen `.slouchcat-*`-Dateien. |
| Ein Feed lenkt den Download ins Heimnetz. | Netzwächter wie heute: `netguard.Clients.For(feedURL, feedOrigin)`; ein Feed aus einer Datei (`feed_origin = 'file'`) bekommt nie die LAN-Ausnahme. |
| Ein Feed füllt die Platte. | 2 GiB je Datei, Platzschwelle vor jedem Download, höchstens 2'000 Folgen je Feed, Schätzung vor «Alle Folgen». |
| Etwas anderes als Audio landet im Ordner. | Endung nur aus `audioExts`, kein HTML; die Sonde prüft danach. Schlägt die Sonde fehl, bleibt die Datei liegen, die Folge zeigt den Fehler (kein automatisches Löschen, F-4). |
| Zugangsdaten eines privaten Feeds im Log. | Wie heute nur Schema und Host (`netguard.Clean`), Regel 10. |

## 14. Bauplan

| Stufe | Inhalt | Release | Aufwand |
|---|---|---|---|
| **0** | Messauftrag M-1 (Abschnitt 8) auf CT 190, Ergebnis nach `docs/perf/19-podcasts.md`. Frage 26 und ADR 0029. | – | 0,5 Tage |
| **1 «Speichern»** | Paket `internal/fetch` (aus `offline` herausgelöst), Migration, Planer und Download mit Pool `podcast`, Schreibtest, Dateinamen-Abgleich, Vertrag 1.24.0, Dialog «Neue Folgen» in der Bibliothek (Variante A), auf der heutigen Podcasts-Seite: Wahl je Podcast im Ausklapp, Spalte «gespeichert», Knopf «Fehlende nachladen». `compose.yaml` und Installationsanleitung. | 1.24.0 | 6 bis 7 Tage |
| **2 «Neue Podcasts-Seite»** | Gewählte Listenvariante, Seite «Podcast» mit Folgenliste, Abschnitt «Downloads», Cover, Vertrag 1.25.0. | 1.25.0 | 4 bis 5 Tage |

Stufe 1 ist für sich nützlich: nach ihr speichert Slouchcat Folgen, und Audiobookshelf kann die
Downloads abgeben. Jeder Schritt ist ein eigener Commit; geprüft wird nach den Teststufen in
`CLAUDE.md`.

## 15. Abnahme

| Nr. | Prüfung |
|---|---|
| P-1 | Bibliothek auf «Speichern», Umfang «Nur neue»: eine neue Folge im Feed liegt nach dem Abruf als Datei im Ordner des Podcasts, benannt nach dem Folgentitel; die Folge spielt die Datei (Streams-Seite: Quelle «Datei»). |
| P-2 | Dieselbe Folge erscheint genau einmal; kein Eintrag `file_and_feed`. |
| P-3 | Ein Podcast auf «Nur Link» in einer Bibliothek auf «Speichern»: neue Folge bleibt Link, keine Datei entsteht. |
| P-4 | «Neue + letzte 3» einschalten: genau die drei jüngsten Folgen ohne Datei werden geladen. |
| P-5 | «Neue und fehlende»: eine von Hand entfernte Lücke wird nicht gefüllt (Zustand «entfernt»), eine nie geladene Lücke schon. |
| P-6 | Ordner read-only eingebunden: «Speichern» ist nicht wählbar, der Dialog sagt warum; nach dem Umbinden besteht der Schreibtest. |
| P-7 | Download bricht ab (Netz weg): die Folge bleibt als Link abspielbar, im Ordner liegt keine sichtbare Teildatei, der nächste Versuch setzt fort. |
| P-8 | Eine vorhandene Datei mit dem Namen des Folgentitels: es wird nicht geladen, sondern zugeordnet. |
| P-9 | Abo ohne Ordner: der erste Download legt Ordner, `metadata.json` und `cover.jpg` an; ein Scan danach erzeugt keinen zweiten Podcast. |
| P-10 | «Speichern» → «Nur Link»: wartende Downloads verschwinden, alle Dateien bleiben. |
| P-11 | Während 200 Folgen nachladen, startet ein Geräte-Download am Handy ohne Wartezeit. |
| P-12 | Tastatur: die Seiten «Podcasts» und «Podcast» sind mit Pfeiltasten, Tab, Enter und Zurück bedienbar (Playwright). |

## 16. Offene Punkte

| Nr. | Punkt | Empfehlung |
|---|---|---|
| O-1 | Heisst «Neue und fehlende Folgen» «Lücken seit der ältesten gespeicherten Datei» (5.2)? | ja; sonst wäre es dasselbe wie «Alle Folgen» |
| O-2 | Stand nach dem Update: «Nur Link», bis Marco umstellt? | ja; kein Download ohne sein Zutun |
| O-3 | Formate, die der Scanner nicht liest (`aac`, `wav`, Video-Podcasts als `mp4`; auf dem NAS liegen 10 MP4): Link lassen? | ja, mit Hinweis «Format wird nicht gespeichert»; Video-Podcasts sind ein eigenes Thema |
| O-4 | «Gleichzeitig laden» als Einstellung (1 bis 3) oder fest 2? | Einstellung, sie kostet wenig |
| O-5 | Neue Folge sofort beim Feed-Abruf melden oder erst, wenn die Datei da ist? | sofort, die Folge ist als Stream hörbar |
| O-6 | Bandbreite begrenzen? | nein; Zeitfenster und «Gleichzeitig laden» genügen |
| O-7 | Tags in die Datei schreiben (Titel, Datum)? | nein; die Datei bleibt, wie der Anbieter sie liefert, Slouchcat kennt die Daten aus dem Feed |
| O-8 | Schlägt die Sonde einer geladenen Datei fehl: Datei löschen? | nein (F-4); Fehler zeigen, Marco entscheidet |
| O-9 | Mehrfachauswahl (Variante C) zusätzlich zu A? | erst bei Bedarf |

## Anhang A: Mockups

Leinwand «Konzept 19 Podcasts», sechs Artboards, alle klickbar:

| Artboard | Zeigt |
|---|---|
| `Main.dc.html` | Abos A: Liste mit Covern, Filter, Suche, Ausklapp mit Einstellung und «Fehlende nachladen» |
| `AbosB.dc.html` | Abos B: Raster, Seitenfeld des gewählten Podcasts |
| `AbosC.dc.html` | Abos C: Tabelle, Auswahl «Neue Folgen» in der Zeile, Sammelaktionen, Sortierung |
| `Einstellung.dc.html` | Einstellung A: Dialog in der Bibliothek mit Schreibtest und Schätzung |
| `EinstellungB.dc.html` | Einstellung B: Band auf der Podcasts-Seite, Abschnitt «Downloads» |
| `Podcast.dc.html` | Einzelner Podcast: eigene Einstellung, Folgenliste mit Zuständen |

Die Zahlen in den Mockups sind Beispiele; gemessen sind nur die Werte in Abschnitt 4.
