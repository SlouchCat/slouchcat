# Messwerte: Wiederverknüpfung beim Ordnerwechsel (Idee 04, ADR 0023)

Idee `docs/ideas/04-wurzelwechsel-benutzerdaten.md`, Frage 21, ADR 0023. Gemessen am 3. November 2026
auf dem Entwicklungsrechner (i7-14700K, 28 Kerne, Go 1.27.2) mit Wegwerf-Benchmarks über die
Testdatenbank-Hilfen des Pakets `library`. Die Benchmarks sind nicht eingecheckt.

## Scanner-Index mit vielen gelöschten Titeln

Seit ADR 0023 verknüpft der Scanner über die Anbieter-ID **ohne Frist**. `loadItemIndex` lädt darum
alle gelöschten Titel einer Bibliothek, nicht mehr nur die der letzten 30 Tage. Gemessen wurde mit
der Grösse der Bibliothek «Serien» auf CT 190: 500 lebende Titel und **31 641 gelöschte Zeilen**
(498 Serien, 2 010 Staffeln, 29 106 Folgen, 27 Extras). Ein Aufruf je Messung, Mittel aus 3 Läufen
zu je 30 Wiederholungen, Streuung unter 3 %.

| Variante | ms/op | MB/op | allocs/op |
|---|---:|---:|---:|
| vorher (`258abce`), alle gelöschten Zeilen im 30-Tage-Fenster | 224 | 247 | 806 553 |
| vorher (`258abce`), alle ausserhalb des Fensters | 13 | 2,3 | 13 420 |
| nachher (`ListDeletedItemsByLibrary`, ohne Frist, schmale Abfrage) | 90 | 59,6 | 364 723 |

- In den ersten 30 Tagen nach einem Ordnerwechsel ist der neue Index **2,5-mal schneller** und
  braucht **4-mal weniger Speicher** als vorher. Grund ist die schmale Abfrage: 11 Spalten statt
  `SELECT *`, und nur die Arten, die der Index braucht.
- Nach Ablauf der 30 Tage lud der alte Code die gelöschten Zeilen gar nicht mehr. Der neue braucht
  dann **rund 77 ms mehr** je Scan der Bibliothek. Das ist der Preis der Wiederverknüpfung ohne Frist.
  Von den 90 ms entfallen 65 ms auf die Abfrage (mit Sortierung `deleted_at DESC, id DESC` in einem
  temporären B-Baum), 25 ms auf den Aufbau der Maps.
- Ein Scan einer unveränderten kleinen Bibliothek (20 Filme, keine gelöschten Titel): vorher
  0,74–0,78 ms, nachher 0,76–0,79 ms, kein messbarer Unterschied.

Abfragepläne auf derselben Datenbank:

- `ListDeletedItemsByLibrary`: `SEARCH items USING INDEX items_library_kind (library_id=? AND
  kind=? AND deleted_at>?)`, dazu `USE TEMP B-TREE FOR ORDER BY`.
- `FindDeletedTwin` (Zwilling nach dem Match): `SEARCH l USING INDEX sqlite_autoindex_items_1
  (id=?)`, `MULTI-INDEX OR` über `items_tmdb`, `items_tvdb`, `items_imdb`, dazu ein temporärer
  B-Baum für die Sortierung. **58 µs** je Treffer.

## Pflegeaktion «Verwaiste Benutzerdaten»

200 gelöschte Serien mit je 3 Staffeln zu 10 Folgen (7 012 Titel), 6 000 Fortschritte, 30 000
Wiedergabe-Ereignisse. Mittel aus 20 Runden.

| Schritt | Zeit |
|---|---:|
| Benutzerdaten-Zähler vorher: eine Abfrage je Paar, 200 Paare | 1,04–1,12 s |
| Benutzerdaten-Zähler nachher: eine Abfrage für alle Paare | 34,5–34,7 ms |
| `ListRelinkPairs` mit 200 Paaren | 1,8 ms |

Die Zähler je Paar liefen über einen Baumausdruck mit `OR` und lasen dabei jedes Mal alle Titel.
Die zusammengefasste Abfrage ersetzt sie im Review-Fix `5c74811`.

## CT 190

Bibliothek «Serien» (2 lebende Serien, 291 Folgen, rund 31 600 gelöschte Zeilen), Scans nach Zeitplan,
Dauer aus `jobs.finished_at - started_at`:

| Stand | Läufe | Dauer |
|---|---:|---|
| 1.8.0 (`verwaltungsseiten-bf456e1` und `1.8.0`), 2. und 3. November 2026 | 8 | 213–608 ms, Median etwa 345 ms |
| `wurzelwechsel-34822aa` | – | folgt mit dem nächsten Scan nach Zeitplan |

Die Pflegeaktion auf CT 190 (3. November 2026, Marco): die Liste in 1,4–1,6 s; das Umhängen der Paare
in 1,47 s (Two and a Half Men, 274 Knoten) und etwa gleich lang für Silo (34 Knoten).
