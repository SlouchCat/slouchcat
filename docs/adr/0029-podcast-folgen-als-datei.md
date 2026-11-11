# ADR 0029: Podcast-Folgen als Datei: schreibbare Podcast-Ordner

- Status: angenommen (11. November 2026); **hebt Spec B-1 («read-only») für Podcast-Ordner teilweise auf**
  (Frage 26, F-3), **schreibt ADR 0007 fort** (eine Feed-Episode kann eine Datei bekommen, die der Server selbst
  lädt) und **ergänzt ADR 0015** (Podcast-Ordner werden nicht mehr nur gelesen) sowie **ADR 0017** (neue Jobarten
  mit Zeitfenster)
- Quelle: Konzept `docs/planning/19-podcasts-und-radio.md` Teil A (Abschnitte 5 bis 10, 13, 14, 17, 18);
  **Frage 26** in `docs/planning/04-entscheidungen.md` (F-1 bis F-6, O-2 bis O-12); Messung M-1
  `docs/perf/19-podcasts.md`; Audiobookshelf `sanitizeFilename` (gelesen am 11. November 2026)
- Vertrag 1.27.0 (Stufe 1, additiv) und 1.28.0 (Stufe 2, additiv), Migration 00047 (vorläufig; die Nummern
  vergibt die Reihenfolge der Abnahmen, siehe Protokoll)
- Betroffene Spec-IDs: B-1, H-2, O-3, A-3, A-9
- Harte Regeln: 2 (Planer und Download in `library`, Liste und Einstellung in `metadata`), 3 (`os/exec` nur in
  `transcode`: Sonde und Tags), 4 (Schreiben nur über `paths` und `os.Root`), 5 (Migration nach Vorlage),
  7 (`x-visibility: admin`), 10 (keine Enclosure-Adresse im Log), 12 (Entscheid im Server)

## Kontext

Slouchcat hält Podcast-Folgen bis 1.26.0 nur als Link: eine Feed-Episode hat `enclosure_url`, abgespielt wird
über den Server vom Anbieter (`ProxyEnclosure`, ADR 0007). Dateien in einem Podcast-Ordner entstehen ausserhalb,
heute durch Audiobookshelf, und finden über `finishPodcastEpisode` zu ihrer Feed-Episode (ADR 0015). Kein Code
schreibt in eine Bibliothekswurzel, und `deploy/compose.yaml` bindet alle Medien mit `:ro` ein.

Marco will, dass Slouchcat neue Folgen selbst als Datei in den Ordner des Podcasts speichert und Audiobookshelf
die Downloads abgibt (F-3), dass gespeicherte Folgen für immer bleiben (F-4) und dass er je Bibliothek, je
Podcast abweichend, zwischen «Speichern» und «Nur Link» wählt (F-1, F-2). Zu entscheiden war:

1. Wie wird genau ein Ordner schreibbar, ohne B-1 für Filme, Serien und Hörbücher zu lockern?
2. Wie kommt die Datei an die Feed-Episode, ohne ein zweites Item, ohne Wettlauf mit Scan und Verheiraten,
   ohne eine fremde Datei zu ersetzen?
3. Wie verhindert der Server, dass er eine Folge lädt, die schon im Ordner liegt?
4. Wie heisst die neue Datei?
5. Wie verhindert der Server, dass er auf die lokale Platte schreibt, wenn das NAS fehlt?

Verworfen: (a) eine feste schreibbare Zeile in `compose.yaml` (gälte für alle, der Updater ersetzt die Datei,
ein fehlender Pfad verhindert den Start); (b) `rename` der Teildatei auf den Zielnamen (ersetzt unter Linux
eine Datei gleichen Namens still); (c) ein gespeicherter Zustand je Folge («gespeichert», «entfernt», Ordner
des Podcasts), der mit dem Ordner und dem Jobsystem auseinanderläuft; (d) der Download im Paket `metadata`
(dort entstehen keine `media_files`, und `metadata` importiert `library` nicht); (e) die Regel 6.3 des
Papiers für Dateinamen (M-1: 40 % der Namen wichen von Audiobookshelf ab).

## Entscheid

1. **Ein schreibbarer Medien-Mount, freiwillig.** `deploy/compose.podcasts.yaml` legt `${PODCAST_DIR}` ohne
   `:ro` über den read-only Baum, eingeschaltet über `COMPOSE_FILE` in `.env`. `PODCAST_TARGET` ist genau der
   Pfad der eingetragenen Podcast-Wurzel (sonst wäre es ein Wurzelwechsel nach ADR 0023). Ohne die Datei
   bleibt alles read-only, und «Speichern» ist gesperrt. Der Server schreibt nur in Wurzeln mit
   `content = 'podcast'`, nur über das `os.Root` aus `paths.Open`, nur neue Dateien; er löscht und
   überschreibt in der Wurzel nichts ausser seinen eigenen `.slouchcat-*`-Dateien.
2. **Ordnerprüfung** vor «Speichern», beim Laden der Seite und vor jedem Download: Dateisystemtyp gleich
   `library_roots.fs_type` und mindestens ein bekannter Podcast-Ordner da (sonst `not_mounted`), Schreibtest
   mit hartem Link (`read_only`, `no_hard_links`), Gruppe und Lesbarkeit (`permissions`). Ohne harte Links ist
   «Speichern» gesperrt (O-12). Einen Ordner, den die Datenbank kennt, legt der Server nie neu an.
3. **Planer und Download im Paket `library`.** `metadata` meldet nach jedem Feed-Abruf über den Haken
   `WithPodcastFetched(podcastID, newIDs)` die neuen Folgen; `Subscribe` meldet keine. Der Planer
   `podcast.sync` (je Podcast, dazu `podcast.sync_all` alle 6 Stunden) reiht je Folge `podcast.download` ein
   (Schlüssel `podcast-dl:<episodeId>`, Nutzlast nur `episodeId`). Eigener Pool `podcast` mit fünf Arbeitern;
   zwei Zähler begrenzen je Bibliothek («Gleichzeitig laden», 1 bis 5) und je Host der Enclosure-Adresse vor
   Weiterleitungen (einer, O-10). Die Jobarten sind `Windowable` (ADR 0017).
4. **Eintragen per hartem Link unter der Scan-Sperre.** Geladen wird in `.slouchcat-<episodeId>.part` im
   Ordner des Podcasts, geprüft mit der Sonde (`not_audio` fällt durch und wird entfernt), getaggt (O-7: Titel,
   Album, Interpret, Datum, Kommentar; nur `mp3` und `m4a`/`m4b`, ohne Umkodieren, Rückfall auf die ungetaggte
   Datei), dann unter `lockScan` nochmals geprüft und per `link(2)` auf den Zielnamen gesetzt; «gibt es schon»
   führt zum nächsten Namen. In derselben Sperre entsteht die `media_files`-Zeile an der Feed-Episode (eine
   fehlende Zeile derselben Episode wird wiederbelebt). Danach spielt die Folge die Datei; Fortschritt und
   Favoriten bleiben am Item.
5. **Dateiname nach Audiobookshelfs Regel** (M-1, Abschnitt 3; ersetzt 6.3 des Papiers): Unicode-NFC, der
   erste `:` wird ` - `, `/ ? < > \ : * | "`, Steuerzeichen und Zeilenumbrüche fallen weg, reservierte
   Windows-Namen und Namen nur aus Punkten werden leer, Punkte und Leerzeichen am Ende fallen weg, mehrfache
   Leerzeichen werden eines; Name samt Endung höchstens 255 Bytes in UTF-16 **und** höchstens 250 Bytes in
   UTF-8 (die zweite Grenze schützt Dateisysteme mit 255 Bytes je Namen bei Schriften mit drei Bytes je Zeichen).
   Leer: `Folge <JJJJ-MM-TT>`. **Endung** aus dem Pfad der Enclosure-Adresse, wenn sie zu den gescannten
   Audio-Endungen gehört, sonst aus dem Inhaltstyp der Antwort, sonst aus dem Format der Sonde; passt nichts,
   wird die Folge nicht gespeichert (O-3). **Kollision** (der Name liegt im Ordner oder in `media_files`): das
   Veröffentlichungsdatum vorangestellt (`2026-11-09 <Titel>.mp3`), dann ` (2)`, ` (3)` … am Stamm.
6. **Drei Riegel gegen doppelte Dateien** (Abschnitt 8, mit M-1 angepasst): der **Schlüssel** (NFC, klein, nur
   Buchstaben und Ziffern) wird aus dem Titel **und** aus dem Stamm des Namens nach Entscheid 5 gebildet und
   gegen den Stamm jeder `media_files`-Zeile in den Ordnern des Podcasts verglichen, ein Anhang ` (<uuid>)` von
   Audiobookshelf zählt nicht. Ein Treffer an einer Datei-Episode ohne GUID wird verheiratet, ein Treffer an
   anderer Stelle, mehrere Kandidaten oder eine unverheiratete Datei-Episode mit einem Datum innert 24 Stunden
   machen die Folge «unklar»; der Planer lädt sie nicht, «Laden» holt sie trotzdem.
7. **Abgeleitet statt gespeichert.** «Gespeichert» ist eine `media_files`-Zeile, «entfernt» sind Zeilen, die
   alle fehlen (der Planer lädt sie nicht von selbst), «wartet» und «lädt» ist der Auftrag mit dem Schlüssel,
   der Ordner eines Podcasts ist der seiner jüngsten vorhandenen Datei. Gespeichert werden nur die Einstellung
   (`libraries.podcast_*`, `podcast_settings`) und Fehlschläge (`podcast_download_failures`, Code ohne
   Adresse). Die wirksame Einstellung rechnet eine gemeinsame SQL-Abfrage für `metadata` und `library`.
8. **Ab Werk «Nur Link»** (O-2): nach dem Update verhält sich Slouchcat wie vorher, bis der Admin umstellt.
   Was einmalig nachlädt, fragt zuerst mit einer Schätzung (`estimatePodcastDownloads`) nach.

## Folgen

- Ein Ordner der Sammlung ist schreibbar. Der Schaden eines Fehlers ist begrenzt: neue Dateien in einer
  Podcast-Wurzel, nie ein Überschreiben, nie ein Löschen fremder Dateien. Die Sicherung schliesst Medien
  weiter aus.
- `metadata.json` eines neu angelegten Ordners enthält die Feed-Adresse; bei einem privaten Feed steht damit
  sein Schlüssel auf dem NAS, wie heute bei Audiobookshelf (Installationsanleitung).
- Die Ladelogik des Geräte-Downloads zieht in das Paket `internal/fetch`; `offline` und `library` teilen sie,
  das Verhalten des Geräte-Downloads bleibt.
- Ein Nachladen vieler Folgen läuft weitgehend nacheinander, weil viele Podcasts hinter einem Host liegen
  (M-1: die Hälfte hinter einem); gewollt (O-10).
- Neue Dateien heissen wie die von Audiobookshelf; die beiden Werkzeuge können eine Weile nebeneinander
  laufen, die Reihenfolge in Abschnitt 8 des Papiers bleibt trotzdem empfohlen.
