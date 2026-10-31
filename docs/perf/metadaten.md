# Messwerte: Metadaten-Einstellungen (Konzept 10, ADR 0018)

Konzept `docs/planning/10-metadaten-einstellungen.md`, Abschnitt 8 («Leistung»). Latten aus dem
Papier: `listLibraryItems` p95 nach Schritt 1 **unverändert ±10 %** (Zeile «Scoped-Abfragen
langsamer?»: `item_texts` bekommt `rank`, die Kette liest je Titel alle Zeilen); Scanner nach
Schritt 2 (NAS-Scan «Filme» heute 186 s); `apply` und Cache-Grösse in Schritt 7 auf CT 190.

## 1. Schritt 1: Bibliotheksseite mit der Sprachkette (`listLibraryItems`)

Entwicklungsrechner (Linux Mint 22.3, i7-14700K, NVMe, wie `docs/perf/startseite.md`),
31. Oktober 2026. Zwei Binaries aus demselben Quellstand-Paar, je eigener Server auf
`127.0.0.1:8097` (`main` `d3d73bc`, Vertrag 1.5.0) und `127.0.0.1:8098` (Zweig
`claude/festive-swanson-7af516`, Schritt 1, Migration 00028), je leerer Konfigurations- und
Cache-Ordner auf der lokalen Platte, ohne TMDB-Schlüssel, `SLOUCHCAT_WORKERS=4`. Platzhalterbaum
aus `task testdata:tree` auf der lokalen Platte (2'500 Filme mit Extras, 400 Serien mit 2'167
Staffeln und 40'334 Folgen), Bibliotheken «Filme» (`Movies/`, `movie`, `["tmdb"]`) und «Serien»
(`TV/`, `series`, `["tmdb","tvdb"]`) über die echte API; gemessen nach Ende beider Scans mit
`docs/perf/measure-library.sh` (60 Anfragen nach fünf Aufwärmläufen, Hauptprofil). Ohne
TMDB-Schlüssel bleiben alle Titel unbewertet und kein Match schreibt Texte; darum werden die
Textzeilen vor der Messung per SQL eingesät (siehe unten), damit die Joins und der Rang-Index
etwas zu tun haben. Der Lauf selbst: `run.sh` und `seed.py` in den Sitzungsnotizen.

Fünf Läufe, je frischer Server und frischer Scan (45 s für 50'570 Titel), danach die Texte je
Titel per SQL eingesät (`item_texts`: `de` und `en` wie heute, auf dem Zweig zusätzlich ein Lauf
mit `es` als dritter Sprache, Rang 0/1/2), weil ohne TMDB-Schlüssel kein Match Texte schreibt.
Werte in ms, p50 / p95.

| Aufruf | main (Lauf 1) | main (Lauf 2) | Schritt 1, de+en (Lauf 1) | Schritt 1, de+en (Lauf 2) | Schritt 1, de+en+es |
|---|---|---|---|---|---|
| `GET /libraries/visible` | 85.6 / 158.7 | 90.1 / 158.2 | 80.3 / 139.4 ¹ | 90.3 / 136.7 | 92.5 / 153.8 |
| `GET /libraries/1/items` (Filme, Titel) | 37.3 / 75.1 | 27.6 / 78.0 | 47.7 / 91.2 ¹ | 40.5 / 79.5 | 20.0 / 55.8 |
| `… ?sort=added` | 21.0 / 33.1 | 18.7 / 27.8 | 22.9 / 59.8 ¹ | 24.6 / 72.4 | 19.6 / 25.8 |
| `… ?sort=vote` | 17.4 / 26.2 | 20.1 / 31.1 | 18.5 / 26.3 ¹ | 17.5 / 28.6 | 20.4 / 34.0 |
| `… ?sort=title&letter=S` | 19.6 / 31.5 | 19.9 / 30.7 | 21.9 / 34.2 ¹ | 18.7 / 28.1 | 19.3 / 27.6 |
| `GET /libraries/2/items` (Serien, Titel) | 56.1 / 65.2 | 58.4 / 69.2 | 56.4 / 66.0 ¹ | 56.0 / 66.9 | 58.6 / 65.4 |
| `… ?sort=played` | 185.2 / 190.4 | 186.7 / 191.8 | 186.3 / 193.7 ¹ | 184.7 / 194.3 | 185.2 / 192.0 |
| `… ?sort=title&letter=S&status=started` (schwerster Fall) | 189.8 / 196.1 | 192.4 / 198.8 | 190.3 / 197.7 ¹ | 189.6 / 196.7 | 189.5 / 198.8 |
| `GET /picks` | 2.4 / 2.9 | 2.7 / 3.5 | 2.7 / 2.9 | 2.1 / 2.8 | 2.6 / 3.0 |

¹ Lauf 1 des Zweigs trug noch die dritte Sprache (`es`, 136'203 Zeilen) **und** den zweiten
`item_texts`-Join auch bei gleicher Metadaten- und Oberflächensprache; danach wurde der zweite
Join für diesen Fall gestrichen (`library.go`: `tu` nur, wenn `metaLang != language`) und die
Messung mit zwei und mit drei Sprachen wiederholt (Spalten 4 und 5).

**Lesart.** Die beiden schweren Fälle der Serienbibliothek (Folgen-Aggregation, rund 190 ms p95)
liegen in allen fünf Läufen innerhalb von 2 % beieinander: die Kette kostet dort nichts
Messbares, die Zeit geht wie in `docs/perf/startseite.md` in `MATERIALIZE ep`. Die Filmbibliothek
nach Titel schwankt zwischen den Läufen **desselben** Binaries stärker (main 27.6 bis 37.3 ms p50,
Zweig 20.0 bis 47.7 ms p50; p95 55.8 bis 91.2 ms) als zwischen den Binaries; mit drei Sprachen
(136'203 statt 90'802 Zeilen) ist der Zweig im letzten Lauf sogar der schnellste. Die Latte
«±10 %» ist mit n = 60 auf einem Entwicklungsrechner darum nur für die schweren Fälle belastbar,
und dort hält sie; für die leichten Fälle lässt sich kein Unterschied über dem Rauschen zeigen.
Was die neue Abfrage mehr tut: ein `LEFT JOIN` auf `item_texts` nach der Metadatensprache (wie
bisher nach der Profilsprache), ein zweiter nur bei abweichender Oberflächensprache (vorher fest
`'en'`), und eine korrelierte Unterabfrage nach dem kleinsten Rang, die SQLite nur auswertet, wenn
beide Joins keinen Schlüssel liefern (`COALESCE` bricht am ersten Wert ab). Der Index
`item_texts_rank (item_id, rank)` trägt sie. CT 190 (2 vCPU) steht aus; nach `docs/perf/M3.md` 1c
ist dort der Faktor 2 bis 3 zu erwarten, die schweren Fälle also 400 bis 600 ms wie schon bei
Konzept 08 vermerkt, unabhängig von diesem Schritt.

## 2. Schritt 2: NAS-Scan «Filme» mit Bilddateien (`item_local_images`)

Entwicklungsrechner wie oben, 31. Oktober 2026, 13:43 bis 13:55 Uhr. Ordner `/mnt/nerdnas/medien/Filme`
auf Marcos NAS über autofs (1'789 Filmordner, 1'792 Dateien), je Lauf ein frischer Server
(`SLOUCHCAT_WORKERS=4`, leerer Konfigurations- und Cache-Ordner auf der lokalen Platte, ohne
TMDB-Schlüssel), Bibliothek «Filme» (`movie`, `["tmdb"]`) über `POST /setup`; gemessen vom Setup
bis `scan.state = done` über `GET /admin/libraries` (Abfrage alle 2 s) und aus der Zeile
`library scan done` im Serverlog (`duration`). Binaries: `main` `d3d73bc` (Vertrag 1.5.0) und der
Zweig nach Schritt 2 (`v1.5.0-9-ge8290b4-dirty`). Skript `nas-scan.sh` in den Sitzungsnotizen.

| Lauf (Reihenfolge) | Binary | Scan (Serverlog) | bis `done` (API) | `item_local_images` |
|---|---|---|---|---|
| 1, kalter Verzeichnis-Cache | Schritt 2 | 218.8 s | 220.4 s | 536 Zeilen, 211 Titel |
| 2 | main | 6.3 s | 8.2 s | (Tabelle gibt es nicht) |
| 3 | Schritt 2 | 3.6 s | 6.2 s | 536 Zeilen, 211 Titel |
| 4 | main | 3.2 s | 4.2 s | (Tabelle gibt es nicht) |

**Lesart.** Der erste Lauf trägt das Aufwärmen des NAS-Verzeichnis-Caches (autofs mountet, der
Kernel liest 1'789 Ordner zum ersten Mal); das ist die Grössenordnung der «186 s heute» aus dem
Papier (Abschnitt 8) und hat mit dem Binary nichts zu tun. Mit warmem Cache kostet der Scan auf
dem Zweig **0.4 s mehr** als auf `main` (3.6 s statt 3.2 s): das ist das Lesen der Header von
536 Bilddateien über das NAS (rund 0.7 ms je Datei) und das Schreiben der 536 Zeilen in einer
Transaktion. Ein kalter Lauf mit `main` zum Vergleich war ohne `sudo` (Verwerfen des
Seiten-Caches) nicht möglich; die Latte des Papiers («vernachlässigbar») hält mit dem warmen
Paar, der kalte Wert ist eine obere Schranke für beide Binaries.

Was die Tabelle nach dem Scan enthält (Lauf 3): `poster` 210 (Breite 300 bis 2'000, 4 davon unter
500), `backdrop` 207 (1'280 bis 3'840), `logo` 119 (316 bis 7'735, 7 unter 500), in 211 der 1'789
Filmordner; die Dateien heissen dort `folder.jpg`, `backdrop.jpg` und `logo.png` (darum nimmt der
Scanner neben `poster`/`fanart`/`clearlogo` auch diese Namen, Papier 4.6). Der Scan liest nur den
Header (JPEG/PNG über `image.DecodeConfig`, WebP aus dem RIFF-Kopf), nie das ganze Bild; das Bild
selbst holt erst `image.fetch`, wenn die lokale Datei die Wahl gewinnt.
