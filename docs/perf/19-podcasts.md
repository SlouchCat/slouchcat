# Messwerte: Konzept 19 Teil A «Podcasts», Stufe A0

Konzept `docs/planning/19-podcasts-und-radio.md` Teil A (Abschnitte 6.3 und 8, Bauplan Stufe 0), ADR 0029.
Gemessen am 11. November 2026 auf CT 190 (Image `quellen-1cd8f01`) und auf Marcos NAS. Die Messung ist reine
Dokumentation: Sie öffnet die Datenbank von CT 190 nur lesend (`?mode=ro`), listet den Podcast-Ordner und
liest von 120 Dateien den ID3-Kopf. Sie schreibt nichts auf das NAS, auf CT 190 und in Marcos Instanz.

| Frage von M-1 (Abschnitt 8) | Antwort |
|---|---|
| Feed-Episoden ohne Datei | **23'710** von 23'710: CT 190 hat **keinen Podcast-Ordner eingetragen** (Abschnitt 1); jede Feed-Episode ist heute Link |
| Datei-Episoden ohne GUID | **0** in der Datenbank (kein Ordner); auf dem NAS **26'377** Audiodateien in 102 Ordnern |
| Paare nach Schlüssel (NFC, klein, nur Buchstaben und Ziffern) | **21'309** eindeutig (89,9 %), **446** mit mehreren Dateien (1,9 %) |
| Paare nach Audiobookshelfs Namensregel | **21'809** (92,0 %); 94 davon trifft der Schlüssel nicht (gekürzte Namen, Abschnitt 3) |
| Folgen ohne jeden Kandidaten | **1'731** (7,3 %), davon 1'470 in «Weltwoche Daily»; zusammen **rund 69 GB** (aus `enclosure_length`) |
| Folgen ohne Schlüssel, aber mit einer Datei innert 24 Stunden | **224** nach `mtime`; die `mtime` taugt nicht als Datum (Abschnitt 4), im Server gilt das Datum der Tags |
| Verschiedene Enclosure-Hosts | **21**; ein Host trägt die Hälfte aller Folgen (`audio.podigee-cdn.net`, 11'826) |
| Hosts je Podcast | 94 von 95 Podcasts nur einen, höchstens zwei |

**Ergebnis:** Die drei Riegel reichen, mit zwei Anpassungen, die ADR 0029 festhält: der Schlüssel wird
zusätzlich aus dem gekürzten Namen nach Audiobookshelfs Regel gebildet (Abschnitt 3), und Slouchcat benennt
neue Dateien nach Audiobookshelfs Regel statt nach der Regel von Abschnitt 6.3 (Abschnitt 5).

## 1. Stand auf CT 190

| Grösse | Wert |
|---|---|
| Bibliothek «Podcast» (Art `podcasts`) | 94 lebende Podcasts, 93 mit `feed_origin = 'file'`, 1 von Hand abonniert |
| Feed-Episoden (lebend, mit GUID) | 23'604 in der Datenbank; 23'710 in der Zählung je Ordner, weil ein Feed in zwei Ordnern liegt (`two_folders_one_feed`) |
| `library_roots` mit `content = 'podcast'` | **keine**. Die Wurzel des Podcast-Ordners ist aus der Bibliothek entfernt; mit ihr gingen alle `media_files` der Podcast-Episoden |
| `media_files` an Podcast-Episoden | 0 |
| Podcast-Ordner auf dem NAS | `/srv/media/nas/Audiobookshelf/Podcast` (im Container `/media/nas/Audiobookshelf/Podcast`), über NFS eingebunden, im CT read-only (`mp1 … ro=1`) |

Folge für die Abnahme von Stufe 1: Marco trägt den Ordner wieder als Podcast-Ordner der Bibliothek ein, bevor er
«Speichern» wählt. Der erste Scan liest 26'377 Dateien und verheiratet sie mit ihren Feed-Episoden
(`finishPodcastEpisode`); erst danach kennt der Planer, was schon im Ordner liegt. `PODCAST_TARGET` muss genau der
eingetragene Pfad sein (Abschnitt 13 des Papiers).

## 2. Methode

- Skript `docs/perf/19-podcasts/m1.py` (Python 3, nur Standardbibliothek), auf dem CT über die Standardeingabe
  ausgeführt, ohne eine Datei anzulegen:
  `ssh root@192.168.1.43 'python3 - /opt/slouchcat/deploy/config/slouchcat.db /srv/media/nas/Audiobookshelf/Podcast' < docs/perf/19-podcasts/m1.py`.
  Laufzeit 8 min 21 s (Liste und `stat` von 26'377 Dateien über NFS). Die Ausgabe nennt nur Zahlen und Hosts,
  keine Feed- oder Enclosure-Adresse.
- Je Ordner: `feedURL` aus `metadata.json`, dazu der lebende Podcast mit dieser `feed_url`; je Feed-Episode der
  Schlüssel des Titels gegen die Schlüssel aller Audiodateien des Ordners (Dateiname ohne Endung, ohne den
  Anhang ` (<uuid>)` von Audiobookshelf). Dazu der Name nach Audiobookshelfs Regel (Abschnitt 3) gegen die
  Dateinamen.
- **Riegel 3** (Datei innert 24 Stunden) mit der `mtime` als Ersatz für das Datum der Tags, weil CT 190 keine
  Datei-Episoden hat. Gegenprobe: 120 zufällige MP3 (drei je Ordner) auf ihren ID3-Kopf gelesen.
- Gelesen am 11. November 2026 aus dem Hauptzweig von Audiobookshelf: `server/utils/fileUtils.js`
  (`sanitizeFilename`), `server/objects/PodcastEpisodeDownload.js` (`getSanitizedFilename`, `fileExtension`) und
  `server/managers/PodcastManager.js` (Namenskollision).

## 3. Audiobookshelfs Namensregel

| Schritt | Regel |
|---|---|
| Name | `<Titel der Folge, getrimmt>.<Endung>`; kein Datum, keine Nummer |
| Endung | aus dem Pfad der Enclosure-Adresse (ohne Query); ist sie keine bekannte Audio-Endung, `mp3`. Der Inhaltstyp zählt nicht |
| Bereinigen | Unicode-NFC; der **erste** Doppelpunkt wird zu ` - `; `/ ? < > \ : * \| "`, Steuerzeichen (`\x00`–`\x1f`, `\x80`–`\x9f`) und Zeilenumbrüche fallen **weg** (nicht Leerzeichen); ein Name nur aus Punkten wird leer; reservierte Windows-Namen (`con`, `nul`, `com1` …) werden leer; Punkte und Leerzeichen am Ende fallen weg; mehrfache Leerzeichen werden eines |
| Länge | Name samt Endung höchstens 255 Bytes **in UTF-16** (etwa 123 Zeichen bei `.mp3`); der Stamm wird Zeichen für Zeichen gekürzt und getrimmt |
| Kollision | Liegt der Zielname schon im Ordner, hängt Audiobookshelf ` (<uuid>)` an den Stamm (vor dem Kürzen; ein langer Titel verliert den Anhang wieder) |

Die Regel des Papiers (6.3: unerlaubte Zeichen zu Leerzeichen, 180 Bytes UTF-8) ergäbe für **9'527 von 23'710
Titeln (40 %)** einen anderen Namen als Audiobookshelf. **94 Folgen** trifft der Schlüssel nur, wenn er aus dem
gekürzten Namen gebildet wird: der Titel ist länger als Audiobookshelfs Grenze, die Datei heisst nach dem
gekürzten Titel.

## 4. Riegel 3 und die `mtime`

| Grösse | Wert |
|---|---|
| Paare nach Schlüssel mit Datum | 21'309 |
| davon `mtime` innert 24 Stunden nach der Veröffentlichung | 4'942 (23 %) |
| Median `mtime` minus Veröffentlichung | 10'973 Stunden (457 Tage); p10 1 Stunde, p90 45'658 Stunden |
| Gegenprobe ID3: Dateien mit ID3-Kopf | 120 von 120 |
| davon mit vollem Datum (`TDRL`, `TDRC` oder `TYER`) | **116 von 120** |

Die Dateien wurden zum grossen Teil kopiert oder nachgeladen; die `mtime` sagt nichts über die Folge. Die 224
Folgen der Zählung sind darum nur ein Hinweis. Der Server nimmt für Riegel 3 das Datum, das
`finishPodcastEpisode` aus den Tags liest (`date`, `releasedate`, `tdrl`, `tdat`); fast jede Datei trägt eines.

## 5. Wo die Riegel greifen

| Podcast (Beispiele) | Feed | Dateien | eindeutig | mehrere | innert 24 h | ohne Kandidat |
|---|---:|---:|---:|---:|---:|---:|
| Weltwoche Daily | 2'022 | 550 | 532 | 2 | 18 | 1'470 |
| Podcast mit erneuerten Titeln | 615 | 615 | 538 | 0 | 9 | 68 |
| Podcast mit gleichen Titeln je Folge | 719 | 720 | 579 | 139 | 1 | 0 |
| Alle 95 Podcasts mit Ordner | 23'710 | 25'098 | 21'309 | 446 | 224 | 1'731 |

- **74 von 95 Podcasts** haben keine Folge ohne Kandidaten; bei ihnen lüde «Alle Folgen» nichts doppelt.
- **Erneuerte Titel:** Ein Podcast hat 68 Folgen ohne Kandidaten, obwohl Feed und Ordner gleich viele Folgen
  zählen. Der Anbieter hat die Titel nach dem Download geändert (die Dateien tragen die alte Nummer «#242 …»).
  Riegel 3 mit dem Datum der Tags fängt sie; ohne Tag-Datum würde «Alle Folgen» sie ein zweites Mal laden.
- **Gleiche Titel:** 253 Folgen tragen in ihrem Podcast denselben Titel wie eine andere. Riegel 2 («mehrere
  Kandidaten») lässt sie aus; sie sind «unklar».
- **7 Ordner ohne `feedURL`** (1'279 Dateien) und **3'323 Dateien** ohne Feed-Episode (ältere Folgen, die der
  Feed nicht mehr führt, oder mehr als 2'000 Folgen) bleiben Datei-Episoden wie heute.
- Endungen: 26'240 `mp3`, 137 `m4a`. Andere Endungen gibt es im Ordner nicht (die 10 MP4 der Messung vom
  9. November liegen nicht als Audiodatei vor und werden nicht gezählt).

## 6. Hosts und «je Host einer»

| Host (vor Weiterleitung) | Folgen |
|---|---:|
| `audio.podigee-cdn.net` | 11'826 |
| `traffic.megaphone.fm` | 3'284 |
| `download-media.srf.ch` | 2'234 |
| `traffic.omny.fm` | 953 |
| `cdn.julephosting.de` | 615 |
| übrige 16 Hosts | 4'798 |

Ein Nachladen «Alle Folgen» über alle Podcasts liefe also weitgehend nacheinander, weil die Hälfte hinter
einem Host liegt. Für die 1'731 Folgen ohne Kandidaten (69 GB) heisst das bei 20 MB/s rund eine Stunde; das
ist gewollt (O-10). Für den Alltag (ein bis zwei neue Folgen je Podcast und Tag) spielt die Grenze keine Rolle.

## 7. Nicht gemessen

- Wie viele der 446 Mehrfach-Kandidaten das Verheiraten (`finishPodcastEpisode`, gleiche Minute oder gleicher
  Titel, Dauer ±2 %) selbst auflösen würde: dazu braucht es die Sonde jeder Datei, also den Scan nach dem
  Wiedereintragen des Ordners.
- Bandbreite der Anbieter, Zeit je Download.
- Video-Podcasts (O-3: bleiben Link).
