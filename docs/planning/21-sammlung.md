# Konzept 21: Sammlung (Arbeitstitel)

- **Status: Fassung 1, freigegeben (14. November 2026).** Elf Punkte P-1 bis P-11, alle offenen
  Fragen O-1 bis O-21 beantwortet (O-6 KinoCheck: Prüfung folgt später). Marco hat am
  14. November 2026 die Umsetzung in sechs Stufen freigegeben (Abschnitt 14), **ohne Kritiker-Review**.
- **Verankert gegen:** `main` **`c96c7f72`** («Merge pull request #153», Konzept 20 gebaut,
  Release 1.31.0), Vertragsfassung **1.31.0** (`api/openapi.yaml:4`), letzte Migration
  **`00056_library_deleting.sql`**, letzte ADR **`0032`**, letzte Frage **27**. Die nächste freie
  ADR ist 0033, die nächste freie Frage 28.
- `:Zeile`-Angaben beziehen sich auf `c96c7f72`; **nach Namen suchen, nicht nach Zeile.** Pfade
  ohne Präfix liegen unter `server/` (Go) oder `web/src/` (Web-App), wo es eindeutig ist.
- Massgeblich bleiben `docs/spec/01-anforderungen-v1.md`, `docs/planning/04-entscheidungen.md`
  und `docs/adr/` (hier vor allem ADR 0006 Trailer-Kette und ADR 0018 Sprachliste). Bei
  Widerspruch gilt: **Entscheidungsprotokoll > Plan > Spec** (`CLAUDE.md`).

> **Dieses Papier ändert keine Zeile Code.**

## 1. Was Marco will

| Nr. | Marcos Wunsch (sinngemäss) | Datum | Abschnitt |
|---|---|---|---|
| P-1 | Einige deutsche Trailer haben schlechte Qualität. Die Trailerwahl soll die Auflösung berücksichtigen. | 12.11.2026 | 2 |
| P-2 | Fehler aus der Analyse der 5 235 Fehlschläge auf CT 190 korrigieren: ungültiger Schlüssel. | 12.11.2026 | 3 |
| P-3 | Dito: YouTube-Tageslimit für Suchen wird nicht erkannt. | 12.11.2026 | 4 |
| P-4 | Dito: Trailer in weiteren Sprachen (Slouchcat soll Open Source und mehrsprachig sein). | 12.11.2026 | 5 |
| P-5 | Eine beschädigte Folge lässt «Intro und Abspann erkennen» für die ganze Staffel scheitern. O-9/O-10 beantwortet, Bauauftrag steht (6.4), Bau zurückgestellt. | 14.11.2026 | 6 |
| P-6 | Serienansicht: Staffeln und Folgen nutzen nicht die volle Breite. Entschieden: Kachelwand und kompakte Liste. | 14.11.2026 | 8 |
| P-7 | Besetzungsbilder überall grösser. Entschieden: Hochformat 2:3. | 14.11.2026 | 9 |
| P-8 | Personenseite: grösseres Bild und Angaben zur Person. Entschieden: Farbband, Porträt 2:3, Angaben aus TMDB. | 14.11.2026 | 10 |
| P-9 | Radio (und Sammlungen) in «Meine Medien». | 14.11.2026 | 11 |
| P-10 | Vorschlag für die Sammlungen. Entschieden: Raster mit Filter (standardmässig alle), Sammlung als nummerierte Liste. | 14.11.2026 | 12 |
| P-11 | Reihenfolge von «Meine Medien» in der Verwaltung ändern, Einträge ausblenden. Entschieden: eigene Seite. | 14.11.2026 | 13 |

Weitere Punkte folgen.

## 2. P-1 Trailerwahl nach Auflösung

### 2.1 Heute

- Die Trailer-Kette (ADR 0006, `internal/metadata/trailers.go`) fragt zuerst TMDB
  (`/movie/{id}/videos` bzw. `/tv/{id}/videos`), je Sprache der Bibliothek eine Abfrage
  (ADR 0018, `trailerLangsOf`). Nur für Sprachen ohne Treffer sucht sie danach auf YouTube
  (`internal/metadata/youtube.go`).
- TMDB liefert je Video das Feld **`size`** (Auflösung: `360`, `480`, `720`, `1080`, `2160`). Das
  Struct `Video` in `internal/metadata/tmdb.go` liest es **nicht**; gelesen werden nur
  `iso_639_1`, `name`, `key`, `site`, `type`, `official` und `published_at`.
- `pickTMDBTrailers` behält Einträge vom Typ `Trailer` auf YouTube in einer gewünschten Sprache
  und wählt je Sprache einen: **offiziell vor inoffiziell, dann der neueste**. Ein offizieller
  Trailer in `480` schlägt damit immer einen inoffiziellen in `1080`.
- Der YouTube-Weg (`pickYouTubeVideo`) prüft Dauer (60 bis 300 Sekunden), Titel und offiziellen
  Kanal, aber nicht die Auflösung. `videos.list` könnte `contentDetails.definition` (`hd`/`sd`)
  mitliefern; der Teil `contentDetails` wird für die Dauer bereits abgefragt, die Abfrage kostet
  1 Einheit.
- Die Tabelle `trailers` (Migration `00008`) hat keine Spalte für die Auflösung.
- Bestand auf CT 190 (12.11.2026): 4 582 Trailer von TMDB (en 2 512, de 2 070), 144 von YouTube
  (de 115, en 29).

### 2.2 Vorschlag

1. `Video` liest `size` mit. `pickTMDBTrailers` wählt je Sprache den ersten Treffer dieser
   Rangfolge (nur Typ `Trailer` auf YouTube in der gewünschten Sprache, wie heute):

   | Rang | Quelle | Auflösung | Offiziell |
   |---|---|---|---|
   | 1 | TMDB | 2160 (4K) | ja |
   | 2 | TMDB | 1080 | ja |
   | 3 | TMDB | 720 | ja |
   | 4 | TMDB | 2160 (4K) | nein |
   | 5 | TMDB | 1080 | nein |
   | 6 | TMDB | 720 | nein |
   | 7 | TMDB | 480 | ja |
   | 8 | TMDB | 360 oder unbekannt | ja |
   | 9 | TMDB | 480 | nein |
   | 10 | TMDB | 360 oder unbekannt | nein |
   | 11 | YouTube-Suche (nur wenn TMDB in dieser Sprache nichts hat) | `hd` | wie heute: offizieller Kanal zuerst |
   | 12 | YouTube-Suche | `sd` | wie heute |

   **Marco hat diese Rangfolge am 12. November 2026 bestätigt.** Bei Gleichstand gewinnt der
   neuere (`published_at`). Ab 720 bleibt «offiziell» vorne; unter 720
   verliert ein offizieller Trailer gegen jeden inoffiziellen ab 720.
2. YouTube-Weg: `definition = hd` geht vor `sd` (Ränge 11 und 12); `videos.list` fragt
   `contentDetails` schon ab, es kostet nichts zusätzlich.
3. **Bestand:** Bereits gefundene Trailer ändern sich erst bei einem neuen `trailer.find`. Für den
   Bestand braucht es einen einmaligen Lauf über alle Titel mit `trailer_state = found` und
   automatischen Trailern (manuelle bleiben unberührt, wie heute).

`size` pflegt die TMDB-Community von Hand; der Wert stimmt nicht immer. Er taugt als Reihenfolge,
nicht als Garantie.

### 2.3 Offene Punkte

- **O-1 Schwelle:** ~~720 als Grenze für «gut genug»?~~ **Beantwortet 12.11.2026:** ja, 720,
  mit der Rangfolge aus Abschnitt 2.2.
- **O-2 Auflösung speichern und zeigen:** Eine Spalte `trailers.size` (Migration) erlaubt, die
  Auflösung in der Trailer-Verwaltung anzuzeigen und später gezielt schlechte Trailer zu finden.
  Empfehlung: ja, mit Anzeige im Bleistift-Dialog der Trailer.
  **Beantwortet 14.11.2026:** ja, `trailers.size` speichern und im Bleistift-Dialog der Trailer anzeigen.
- **O-3 Bestandslauf:** automatisch einmal nach dem Update oder als Knopf in der Verwaltung?
  Empfehlung: automatisch einmal (wie `metadata.image_sizes_trimmed` in 1.28.1); der Lauf braucht
  je Titel und Sprache eine TMDB-Abfrage, aber keine YouTube-Suche, solange TMDB einen Trailer hat.
  **Beantwortet 14.11.2026:** einmal automatisch nach dem Update.

## 3. P-2 Ungültiger Schlüssel: kein Wiederholen, kein Neu-Einreihen

### 3.1 Heute (Befund vom 12. November 2026 auf CT 190)

- Von 5 235 Fehlschlägen waren **4 899** `metadata.fill` mit
  `metadata.fill: tvdb <id>: tmdb key invalid: tvdb /login: status 401`. Der TVDB-Schlüssel war
  seit etwa 4. November ungültig (`InvalidAPIKey: apikey invalid`); Marco hat ihn am 12. November
  ersetzt, seither laufen die Fills wieder.
- Betroffen waren nur **536 Titel** (Serien2 503, Anime 29, Serien 2). Die Menge entsteht so:
  - `tvdbStatusError` (`internal/metadata/tvdb.go`) macht aus 401 `model.ErrTMDBKeyInvalid`. Der
    Fill gibt den Fehler zurück, die Warteschlange behandelt ihn wie einen vorübergehenden und
    versucht es **5 Mal**.
  - `QueuePendingFills` (`internal/metadata/fill.go`) reiht nach **jedem** Scan jeden Titel neu
    ein, dessen `filled_providers` nicht zur wirksamen Anbieterliste passt. Weil der Fill nie
    gelingt, kommen alle 534 Titel bei jedem geplanten Scan (etwa alle 2 Stunden) wieder: rund
    2 100 bis 2 600 Fehlschläge pro Tag.
- Die Meldung führt in die Irre: Der Sentinel heisst `ErrTMDBKeyInvalid` und trägt den Text
  `tmdb key invalid`, gilt aber für alle Anbieter. Im Fehler steht so «tmdb», obwohl TVDB den
  Schlüssel abweist.
- Gespeichert wird ein neuer Schlüssel ohne Prüfung (`PUT /api/v1/admin/settings`); ob er gilt,
  zeigt sich erst beim nächsten Job. Seit 1.31.0 gibt es je Anbieter einen Knopf «Prüfen»
  (Konzept 20 P-12, `checkProviderKey`, `CheckProviderKey` in `internal/metadata/key_check.go`);
  er läuft nur auf Klick.
- Der Trailer-Weg macht es schon richtig: `findTrailers` setzt bei `ErrTMDBKeyInvalid`
  `trailer_state = failed` und endet ohne Wiederholung.

### 3.2 Vorschlag

1. **Kein Wiederholen:** Ein Fill, der an einem abgewiesenen Schlüssel scheitert, endet sofort
   (ein Versuch, kein Retry), wie der Trailer-Weg.
2. **Kein Neu-Einreihen:** Solange ein Anbieter seinen Schlüssel abweist, zählt er für
   `QueuePendingFills` wie «nicht eingerichtet». Der Server merkt sich die Abweisung je Anbieter
   und Schlüssel (im Speicher genügt, nach Neustart prüft der erste Job erneut). Ein neuer Schlüssel
   hebt die Sperre auf und reiht die liegengebliebenen Titel einmal ein.
3. **Sichtbar machen:** Die Verwaltung zeigt bei «Schlüssel der Anbieter» und in der Übersicht
   «TVDB-Schlüssel abgewiesen». Die Setup-Prüfung in `internal/admin/setup_checks.go`
   (`CheckSetupTMDBKey`) prüft heute nur TMDB; TVDB kommt dazu. Ein Eintrag statt Tausender
   Fehlschläge.
4. **Beim Speichern prüfen:** Ein neuer TMDB- oder TVDB-Schlüssel wird beim Speichern einmal
   geprüft, mit derselben Prüfung wie der Knopf «Prüfen» (`CheckProviderKey`). Ein abgewiesener
   Schlüssel wird trotzdem gespeichert, die Antwort sagt aber «abgewiesen».
5. **Meldung korrigieren:** Der Fehlertext nennt den Anbieter, der abweist (z. B.
   `tvdb key invalid`). Ob der Sentinel umbenannt wird (`ErrProviderKeyInvalid`), entscheidet der
   Bau; der Vertragscode `tmdb_key_invalid` bleibt, weil Clients ihn kennen.

### 3.3 Offene Punkte

- **O-4 Alte Fehlschläge:** Die 4 899 Einträge bleiben bis zur Aufräumfrist (30 Tage) stehen oder
  gehen mit «alle fehlgeschlagenen löschen» aus Konzept 20 (P-6). Empfehlung: nichts Eigenes bauen.
  **Beantwortet 14.11.2026:** beim Update einmal löschen: Ein einmaliger Lauf entfernt die Fehlschläge mit ungültigem Schlüssel (abweichend von der Empfehlung).

## 4. P-3 YouTube-Tageslimit für Suchen erkennen

### 4.1 Heute

- Google begrenzt die YouTube-Suche zusätzlich zum Tageskontingent von 10 000 Einheiten auf
  **100 Suchen pro Tag und Projekt** (Metrik `Search Queries per day`). Am 12. November auf CT 190
  gemessen:
  `429 RESOURCE_EXHAUSTED … reason "rateLimitExceeded" … quota_limit_value "100"`. Der Zähler
  beginnt um 08:00 UTC neu (Mitternacht Pazifikzeit).
- `getJSON` in `internal/metadata/youtube.go` erkennt nur die Gründe `quotaExceeded` und
  `dailyLimitExceeded` als «Kontingent leer». Eine 429 mit `rateLimitExceeded` ist seit 1.31.0
  `errYouTubeUnavailable`; das wertet aber nur die Schlüsselprüfung aus (Konzept 20 P-12), die
  Trailersuche behandelt ihn wie jeden anderen Fehler.
- Vorher wiederholt der Limiter (`internal/ratelimit`) jede 429 mit wachsender Pause (bis 60 s je
  Pause). Gemessen: **rund 8 Minuten je Versuch**, in denen der Worker blockiert ist. Am
  12. November hingen alle 4 Worker gleichzeitig daran, andere Jobs (`metadata.fill`) warteten.
- Nach 5 Versuchen ist der Job ein Fehlschlag (54 bis 12. November), statt `pending` zu bleiben,
  wie ADR 0006 es für ein leeres Kontingent vorsieht.

### 4.2 Vorschlag

1. Eine 429 von YouTube, deren Begründung ein Tageslimit nennt (`quota_unit` mit `/d/` oder
   Meldung «per day»), gilt als `errQuotaExhausted`: `quota.Exhaust()`, Titel bleibt `pending`,
   kein Fehlschlag.
2. YouTube-Anfragen laufen ohne Limiter-Wiederholung bei 429 (eine Antwort genügt zum
   Entscheiden); das blockiert keinen Worker mehr.
3. Das eigene Kontingent (`youtubeDailyUnits` = 10 000, Suche = 100 Einheiten) rechnet zusätzlich
   mit **100 Suchen pro Tag**, damit Slouchcat das Limit gar nicht erst erreicht.
4. Der Zähler setzt um 08:00 UTC zurück, wie bei Google.

### 4.3 Offene Punkte

- **O-5 Weniger suchen:** Heute braucht ein Titel bis zu eine Suche pro fehlender Sprache. Soll
  YouTube nur suchen, wenn TMDB **gar keinen** Trailer hat? Empfehlung: nein, die
  Sprachwünsche der Bibliothek bleiben massgeblich; P-4 Punkt 3 verringert die Suchen ohnehin.
  **Beantwortet 14.11.2026:** nein, wie heute: YouTube sucht je fehlender Sprache der Bibliothek.
- **O-6 Andere Quelle:** KinoCheck liefert deutsche Trailer über die TMDB-ID. Limits und
  Bedingungen sind nicht geprüft. Empfehlung: nicht in diesem Konzept, als Idee in `docs/ideas`.
  **Beantwortet 14.11.2026:** ja, KinoCheck kommt in Konzept 21. Vor dem Vorschlag Limits, Bedingungen und Abdeckung prüfen (eigener Abschnitt folgt).

## 5. P-4 Trailer in weiteren Sprachen

### 5.1 Heute

- Die Sprachen kommen aus der Textsprachen-Liste der Bibliothek (ADR 0018, `trailerLangsOf`),
  gekürzt auf den Hauptteil (`pt-BR` → `pt`). Für jede Sprache geht eine TMDB-Abfrage raus
  (`language=<tmdbLang>`). Das ist schon mehrsprachig.
- `Videos` in `internal/metadata/tmdb.go` setzt `include_video_language` aber fest auf
  `de,en,null`; der Kommentar dazu spricht nur von de und en. Für eine französische Bibliothek
  kommen französische Trailer trotzdem (über `language`), dazu unnötig die deutschen und englischen.
- Das Land (`iso_3166_1`) wird ignoriert: `es-ES` und `es-MX`, `pt-BR` und `pt-PT`, `fr-FR` und
  `fr-CA` landen in derselben Sprache, gewählt wird ohne Rücksicht auf das Land.
- TMDB-Trailer **ohne Sprache** (`null`, oft Teaser ohne Sprache oder mit Musik) werden abgefragt,
  aber nie verwendet (`pickTMDBTrailers` kennt nur gewünschte Sprachen).

### 5.2 Vorschlag

1. `include_video_language` = die Sprachen der Bibliothek plus `null`, nicht fest `de,en,null`.
   Kommentare und ADR-Text entsprechend.
2. Hat die Sprachliste ein Land (`pt-BR`), geht ein Trailer aus diesem Land vor einem anderen
   derselben Sprache; danach gilt die Rangfolge aus Abschnitt 2.2.
3. Ein Trailer ohne Sprache dient als Ersatz für eine Sprache, für die weder TMDB noch YouTube
   etwas hat (oder statt der YouTube-Suche, siehe O-7).

### 5.3 Offene Punkte

- **O-7 Ersatz ohne Sprache vor oder nach YouTube:** Vor der YouTube-Suche spart er Suchen,
  liefert aber womöglich einen Teaser statt eines Trailers in der richtigen Sprache. Empfehlung:
  nach YouTube, als letzter Rang.
  **Beantwortet 14.11.2026:** nach YouTube, als letzter Rang.
- **O-8 Ländervorrang:** Lohnt sich Punkt 2 jetzt, oder erst, wenn jemand mit `pt-BR` oder
  `es-MX` danach fragt? Empfehlung: jetzt, er ist klein und betrifft nur die Sortierung.
  **Beantwortet 14.11.2026:** jetzt einbauen.

## 6. P-5 Eine beschädigte Folge blockiert die Segmente der ganzen Staffel

### 6.1 Heute (Befund vom 14. November 2026 auf CT 190, Release 1.31.0)

- Der Job «Intro und Abspann erkennen» (`segments.detect`) für Dragon Ball Z · Staffel 2
  (Staffel `01M73QMZJGZ298BJMRNJ0KPBN9`) ist 5 von 5 Mal gescheitert:
  `insert outro of 01M73QMZK49DXATATX0ESS7325: constraint failed: CHECK constraint failed: end_ms > start_ms (275)`.
- Auslöser ist die Folge S02E11 «Unexpected Problem» (`Dragon Ball Z - S02E11 - Unexpected Problem
  Bluray-720p.mkv`, Dauer 1 455 296 ms, keine Kapitel). Die Datei ist beschädigt: Der Sprung
  `-ss 1095.296` landet bei 244 s (das erste Audiopaket nach 1094 s hat `pts` 243.776), ffmpeg
  meldet `ac3 … Invalid data found when processing input`.
- Der Fingerprint des Endes (`-ss 1095.296 -t 360`) hat dadurch **9 395 Werte** (rund 1 163 s Ton)
  statt rund **2 886** wie bei den anderen 34 Folgen. Er beginnt an der falschen Stelle (244 s statt
  1 095 s) und ist zu lang.
- `tail()` und `window.span()` in `internal/segments/segments.go` nehmen an, dass der Fingerprint
  genau `[from, from + length]` abdeckt (`Step` = 1365/11025 s je Wert). Bei zu vielen Werten liegt
  `start = from + first*Step` hinter dem Dateiende; `End` wird auf die Dateidauer gekappt
  (`min(end, w.duration)`), also `End < Start`.
- `handleSegments` in `internal/library/segments.go` löscht die erkannten Segmente der Staffel und
  schreibt die neuen in **einer** Transaktion (Schleife `for id, r := range res`). Ein ungültiges
  Segment rollt alles zurück: Keine der 35 Folgen bekommt neue Segmente (die alten bleiben), und
  jeder Versuch scheitert gleich.

### 6.2 Vorschlag

1. **Fingerprint gegen die erwartete Länge prüfen.** Hat der Fingerprint von Anfang oder Ende mehr
   als `length / Step` Werte (plus kleiner Toleranz, z. B. 2 % und `edgeWindow`), gilt dieser Teil
   der Folge als «kein Fingerprint». Kürzen hilft nicht: Der Fingerprint beginnt an der falschen
   Stelle der Datei. Das Log meldet eine Warnung mit `media_file_id`, erwarteter und erhaltener
   Zahl der Werte. Die übrigen Folgen der Staffel werden wie heute verglichen.
2. **Jedes Segment vor dem Speichern prüfen.** Vor `InsertSegment` muss `0 <= Start < End <= Dauer`
   gelten. Ein ungültiges Segment wird verworfen und geloggt (Folge, Art, Start, Ende); die anderen
   Segmente der Staffel werden gespeichert, der Job endet erfolgreich. `window.span()` meldet einen
   solchen Bereich gar nicht erst als Treffer.
3. **Tests:**
   - `segments_test.go`: Eine Folge mit überlangem Fingerprint des Endes ergibt kein Segment mit
     `End <= Start`; die anderen Folgen bekommen ihre Segmente.
   - Job-Test in `internal/library`: Eine Staffel mit einer kaputten Folge; die übrigen Folgen
     bekommen Segmente, der Job ist `done`.

Unabhängig davon kann Marco die Datei neu besorgen oder neu muxen.

### 6.3 Offene Punkte

- **O-9 Kürzen oder verwerfen:** ~~Ein zu langer Fingerprint könnte auf die erwartete Länge gekürzt
  werden.~~ **Beantwortet 14.11.2026:** verwerfen, nicht kürzen. Ein überlanger Fingerprint (Anfang
  oder Ende, jeder Teil für sich) gilt als «kein Fingerprint» für diese Folge.
- **O-10 Sichtbar machen:** ~~Soll die Verwaltung eine Folge mit verworfenem Fingerprint anzeigen?~~
  **Beantwortet 14.11.2026:** vorerst keine Anzeige in der Verwaltung, nur eine WARN im Log mit
  `media_file_id`, `item_id`, erwarteter und tatsächlicher Zahl der Werte. Eine Anzeige folgt
  später, falls nötig.

### 6.4 Bauauftrag (Marco, 14. November 2026; Umsetzung erst auf sein Wort)

Marco hat den Bau am 14. November 2026 vorbereitet und dann zurückgestellt: «Ich möchte einfach
dass du das mit ins Konzept aufnimmst. Keine Umsetzung.» Wenn er ihn freigibt, gilt:

1. **Zweig** `claude/k21-p5` ab `main` in einem eigenen Worktree, unabhängig vom Konzeptpapier.
2. **Fingerprint-Prüfung:** mehr Werte als `length / Step` plus kleine Toleranz → dieser Teil der
   Folge (Anfang bzw. Ende einzeln) gilt als ohne Fingerprint; WARN wie in O-10.
3. **Spans vor `InsertSegment` prüfen:** `0 <= Start < End <= Duration`. Ungültige Spans werden
   verworfen und geloggt. Die Transaktion der Staffel darf daran nicht mehr scheitern.
4. **Tests** (ohne Fix rot, mit Fix grün):
   - `internal/segments`: Folge mit überlangem Ende → kein Span mit `End <= Start`.
   - `internal/library`: Staffel mit einer kaputten Folge → die übrigen Folgen bekommen Segmente,
     der Job endet nicht mit Fehler.
5. **Stufe 1:** `task check:fast` und die betroffenen Tests
   (`go test ./internal/segments ./internal/library -run …`), Belege mit Befehl, Ausgabe und SHA.
6. **Commit** nach Hausregel, Bereich `fix(segments)`. PR gegen `main` mit Belegen, davor einmal
   `task ci`.
7. **Nicht mergen, nicht taggen.** CT 190 erst auf Marcos Wort. Abnahme dort: Job «Intro und
   Abspann erkennen» für Dragon Ball Z · S02 (Staffel `01M73QMZJGZ298BJMRNJ0KPBN9`) erneut
   starten. Erwartet: Staffel `done`, 34 Folgen mit Segmenten, WARN für S02E11
   (`01M73QMZK49DXATATX0ESS7325`).
8. Nach dem Bau P-5 hier als «gebaut» markieren.

## 7. Mockups vom 14. November 2026 (P-6 bis P-11)

Die folgenden Punkte hat Marco am 14. November 2026 an klickbaren Mockups entschieden. Die Mockups
liegen nur lokal (Scratchpad der Session, ausgeliefert über `python3 -m http.server`), weil sie echte
Film-, Serien- und Personenbilder von CT 190 enthalten: **nie ins Repo und nie veröffentlichen.**
Sie sind Wegwerf-Seiten mit Token-Werten, nicht mit den Bauteilen aus `web/src/ui`.

| Mockup | Inhalt |
|---|---|
| `index.html` | Varianten Serienansicht: Staffeln, Folgen, Besetzung, Personenseite (je A/B/C) |
| `auswahl.html` | Marcos Wahl Serienansicht und neue Personenseite (`#person`) |
| `sammlungen.html` | Radio und Sammlungen in «Meine Medien», Übersicht und Seite einer Sammlung |
| `verwaltung.html` | Verwaltungsseite «Meine Medien» |

## 8. P-6 Serienansicht: Staffeln und Folgen über die ganze Breite

### 8.1 Heute

- Die Staffeln sind eine `PosterCard`-Reihe (`SeasonShelf` in `features/catalog/SeasonShelf.tsx`) mit
  Pfeilen und Bleistift; auf 2000 px enden die Karten bei rund 1070 px.
- Die Folgen sind `EpisodeListItem`-Zeilen (Standbild, Code, Titel, Text, Dauer, «Details»,
  Gesehen-Kreis) in `.season` mit `max-width: 64rem` (`ItemPage.module.css`).

### 8.2 Entschieden (Marco, 14.11.2026)

1. **Staffeln als Kachelwand (Variante A).** Raster über die ganze Breite
   (`repeat(auto-fill, minmax(var(--sc-size-poster-md), 1fr))`, Handy ab 6.5rem). Zuerst zwei Reihen,
   darunter «Alle N Staffeln anzeigen» / «Weniger anzeigen». Die gewählte Staffel bleibt immer
   sichtbar, auch wenn sie hinter den zwei Reihen läge. Fortschrittsbalken und Haken wie heute; der
   Bleistift erscheint nur bei Zeiger oder Fokus.
2. **Folgen als kompakte Liste (Variante C).** Eine Zeile je Folge über die ganze Breite: Nummer,
   Titel (mit «Als Nächstes»/«Weiterschauen»), einzeiliger Text (ab 700 px), Dauer, Gesehen-Haken.
   Ein Klick klappt die Zeile auf: Standbild, längerer Text, «Abspielen»/«Weiterschauen»,
   «Details», «Herunterladen». Die Folge von `nextUp` ist beim Öffnen aufgeklappt. Der Knopf
   «Details» in jeder Zeile fällt weg.
3. Kopf der Serie und Reihenfolge der Abschnitte bleiben wie heute.

### 8.3 Offene Punkte

- **O-11 Tastatur und TV:** Pfeil ↓ aus der Kachelwand landet in der Folgenliste, Enter klappt eine
  Zeile auf, ein zweites Enter spielt ab? Vorschlag so, Playwright-Tastaturtest nach Regel 8.
  **Beantwortet 14.11.2026:** ja, so: ↓ aus der Kachelwand in die Folgenliste, Enter klappt auf, zweites Enter spielt ab; Playwright-Tastaturtest nach Regel 8.

## 9. P-7 Besetzung im Hochformat 2:3, überall

### 9.1 Heute

`CastCard` zeigt ein rundes `Avatar` von 4.5rem (72 px) mit Name und Rolle darunter, auf Film-,
Serien- und Folgenseiten und in der Suche.

### 9.2 Entschieden (Marco, 14.11.2026, Variante B)

- Porträt im **Hochformat 2:3**, Breite 9.5rem (Handy 7rem), Ecken `--sc-radius-lg`, Ausschnitt
  oben (`object-position: center 20%`). Name und Rolle darunter, linksbündig, Rolle höchstens zwei
  Zeilen.
- Gilt für **jede** Besetzung: Film, Serie, Folge, Suche. `Avatar` bleibt für Profile.
- Ohne Bild: Fläche im Format 2:3 mit Initialen wie heute.

## 10. P-8 Personenseite mit Farbband, grossem Porträt und Angaben

### 10.1 Heute

- `PersonPage` zeigt ein rundes `Avatar` `lg` (8rem) neben dem Namen, «Zurück» und das Raster
  «Filme und Serien».
- Slouchcat speichert zu einer Person nur `name` und `profile_path` (Tabelle `people`, Migration
  00008) und lädt das Porträt nur in TMDB `w185` (`personImageSize`, `internal/metadata/people_images.go`).
  Ab rund 185 px Anzeigebreite wird es unscharf.
- TMDB `/person/{id}` liefert `biography`, `birthday`, `deathday`, `place_of_birth`,
  `known_for_department`; die Biografie fehlt oft auf Deutsch (Beispiel Hank Azaria: nur Englisch).

### 10.2 Entschieden (Marco, 14.11.2026: Variante C mit dem Bild aus B, ergänzt um Angaben)

- **Farbband** aus dem unscharfen, vergrösserten Porträt hinter dem Kopf (wie Variante C).
- **Porträt im Hochformat 2:3**, 15rem breit (Handy 8.5rem), Ecken `--sc-radius-xl`, Schatten.
- Daneben **Name** und **Angaben**: Bekannt für, Geboren (mit Alter), Geburtsort, gegebenenfalls
  Gestorben, «In deiner Bibliothek: n von m Titeln».
- **Biografie** drei Zeilen mit «Mehr lesen»; fehlt sie in der Profilsprache, kommt die englische mit
  dem Hinweis «Biografie von TMDB, nur auf Englisch vorhanden».
- Darunter «Zurück» und das Raster «Filme und Serien · n» über die ganze Breite, je Titel
  «Film · Jahr · als Rolle».

### 10.3 Was dafür nötig ist

- Migration: neue Spalten in `people` (Biografie je Sprache oder mit Sprachkennung, Geburtstag,
  Todestag, Geburtsort, Abteilung, Zahl der Rollen bei TMDB).
- Job: der Abruf von `people.images` holt auch die Angaben (oder ein eigener Job), dazu das Porträt in
  `w300` statt `w185` (O-12).
- Vertrag: neue Felder in `Person` (Minor-Version), `x-visibility` unverändert.

### 10.4 Offene Punkte

- **O-12 Porträtgrösse:** ~~`h632` (421 × 632) oder `w300`?~~ **Beantwortet 14.11.2026:** `w300`
  (300 × 450) **statt** `w185`, eine Grösse für Besetzung und Personenseite. Gemessen am Beispiel
  Hank Azaria: je Person 37,8 KB (JPG 24,9 KB + WebP 12,9 KB) statt 15,7 KB; hochgerechnet auf die
  20 819 Personen auf CT 190 rund 1,3 GB statt 542 MB. Die vorhandenen `w185`-Bilder werden durch
  `w300` ersetzt (Bestandslauf von `people.images`).
- **O-13 Biografie-Sprache:** ~~Englisch als Ersatz zeigen oder weglassen?~~ **Beantwortet 14.11.2026:**
  immer auf Englisch zurückfallen, wenn die Hauptsprache fehlt (mit dem Hinweis aus 10.2).
- **O-14 «n von m Titeln»:** Die Gesamtzahl kommt aus TMDB (`combined_credits`); speichern oder
  weglassen?
  **Beantwortet 14.11.2026:** ja, die Gesamtzahl aus TMDB (`combined_credits`) speichern und «n von m Titeln» zeigen.

## 11. P-9 Radio und Sammlungen in «Meine Medien»

### 11.1 Heute

- In der Seitenleiste (`AppLayout.tsx`) steht Radio in der oberen Gruppe (Start, Suche, Merkliste,
  Radio, Gesehen); «Meine Medien» enthält die Bibliotheken und «User-Empfehlungen».
- Die Startseiten-Reihe «Meine Medien» zeigt nur Bibliothekskacheln (`LibraryTile`) in einer Reihe,
  die nicht die ganze Breite füllt.

### 11.2 Entschieden (Marco, 14.11.2026)

- **Radio** wandert in die Gruppe «Meine Medien» (Zusatz «3 Sender»); in der oberen Gruppe entfällt er.
- **Sammlungen** kommt als eigener Eintrag in «Meine Medien» (Zahl aller Sammlungen).
- Startseite: In «Meine Medien» gibt es eine **Radio-Kachel** mit den Logos der Favoriten. Die Kacheln
  füllen die Breite (`repeat(auto-fit, minmax(13rem, 1fr))`), auf dem Handy wischbar.
- Reihenfolge und Sichtbarkeit regelt P-11.

### 11.3 Offene Punkte

- **O-15 Radio ohne Favoriten:** Kachel mit «Sender entdecken» oder Kachel ausblenden?
  **Beantwortet 14.11.2026:** Kachel «Sender entdecken» ohne Logos; ein Klick führt zum Radio-Angebot.
- **O-16 Reihe «Sammlungen» auf der Startseite:** ~~Welche Sammlungen zeigt die Reihe?~~
  **Beantwortet 14.11.2026:** alle Sammlungen mit mindestens 2 Titeln; Sammlungen mit nur einem
  Titel fehlen in der Reihe (in der Übersicht stehen sie weiterhin).

## 12. P-10 Sammlungen: Übersicht und Seite einer Sammlung

### 12.1 Heute (CT 190, 14.11.2026)

- `/collections` zeigt 333 Sammlungen alphabetisch als Posterraster; 113 haben nur 1 Titel, 93 haben 2.
  Es gibt keine Suche, keinen Filter und keine Sortierung.
- Die Seite einer Sammlung zeigt Kopf (Hintergrund, Poster, «8 Titel») und ein Posterraster ohne
  Reihenfolge, Jahre oder Fortschritt.
- `Collection` im Vertrag hat nur `id`, `name`, `image`, `items` (keine Beschreibung); die Liste
  liefert `count` und `cover`.

### 12.2 Entschieden (Marco, 14.11.2026)

1. **Übersicht Variante A, standardmässig alle:** Raster über die ganze Breite mit Suche,
   Grössenfilter («Alle» vorgewählt, «Ab 2 Titeln», «Ab 3», «Ab 5»), Sortierung (Name A–Z, Meiste
   Titel, Angefangen zuerst), ohne Zufallsknopf (O-17). Auf dem Poster ein Stapel-Zeichen mit der Anzahl,
   bei angefangenen Sammlungen ein Fortschrittsbalken; darunter «n Titel · Jahr–Jahr».
2. **Seite einer Sammlung Variante A, nummerierte Liste:** Kopf mit Hintergrund des ersten Titels,
   Poster der Sammlung, Name, «8 Filme · 2001–2011 · zusammen 19 h 38 min», Fortschritt «x von n
   gesehen», grosser Knopf «Abspielen/Weiterschauen · Teil N», runde Knöpfe «Alle merken» und
   «Gesehen». Darunter Teil 1…N in Erscheinungsreihenfolge mit Poster, Titel, Jahr, Dauer
   und Gesehen-Haken; der nächste Teil trägt «Als Nächstes» bzw. «Weiterschauen».

### 12.3 Was dafür nötig ist

- Vertrag: Liste und Seite brauchen Jahre, Gesamtdauer und Fortschritt je Profil (scoped); Filter,
  Sortierung und Suche serverseitig oder clientseitig (333 Einträge passen in eine Antwort).

### 12.4 Offene Punkte

- **O-17 Zufallsknopf:** ~~im Mockup enthalten~~ **Beantwortet 14.11.2026:** braucht es nicht, weder in
  der Übersicht noch auf der Seite einer Sammlung.
- **O-18 Beschreibung:** TMDB liefert eine Beschreibung der Sammlung; laden und im Kopf zeigen?
  **Beantwortet 14.11.2026:** ja, Beschreibung der Sammlung laden und im Kopf zeigen; Sprache wie O-13 (Hauptsprache, sonst Englisch).
- **O-19 «Gesehen» für die ganze Sammlung:** alle Teile auf einmal als gesehen markieren?
  **Beantwortet 14.11.2026:** ja, «Gesehen» markiert alle Teile, mit Rückfrage wie bei einer ganzen Serie; nochmals drücken nimmt es zurück.

## 13. P-11 Verwaltung: Reihenfolge und Sichtbarkeit von «Meine Medien»

### 13.1 Wunsch

Marco will die Reihenfolge von «Meine Medien» in der Verwaltung ändern und einzelne Einträge aus-
und einblenden können.

### 13.2 Entschieden (Marco, 14.11.2026, Variante A)

- **Eigene Seite «Meine Medien»** unter Kuratierung, direkt nach «Startseite».
- Gebaut wie `HomeRowsPage` (`/admin/home-rows`): `AdminPage`, `BandPanel` mit den Reitern
  «Reihenfolge» und «Vorschau», `DataTable` mit `RankSelect` («Platz N», «Die anderen Einträge
  rücken nach.»), Spalten Eintrag (mit Art: Bibliothek, Radio, Sammlungen, Empfehlungen), Inhalt,
  Zustand (`StatusDot`), je ein Kästchen «Ausblenden» für Seitenleiste und Startseite (O-21),
  `SaveBar` mit Verwerfen und Speichern.
- Die Vorschau zeigt Seitenleiste und Startseiten-Kacheln mit dem Entwurf.
- **Eine** Reihenfolge gilt für Seitenleiste und Startseiten-Kacheln, für alle Profile; ausgeblendet
  wird je Ort getrennt (O-21). Ausblenden versteckt nur den Eintrag; die Titel bleiben in Suche und Reihen. Wer was sehen darf, regeln
  weiterhin die Konten.

### 13.3 Was dafür nötig ist

- Speicher für Reihenfolge und Ausblenden (Tabelle oder Einstellung), Admin-Endpunkte
  (`x-visibility: admin`) und Auslieferung an die App (in `/libraries/visible` oder `/home`).

### 13.4 Offene Punkte

- **O-20 Neue Bibliothek:** wird am Ende eingereiht und ist sichtbar?
  **Beantwortet 14.11.2026:** am Ende eingereiht und sichtbar (Seitenleiste und Startseite).
- **O-21 Getrennt ausblenden:** ~~ein Kästchen für beides oder je eines?~~ **Beantwortet 14.11.2026:**
  getrennt. Die Tabelle hat statt «Ausblenden» die Spalten «Seitenleiste» und «Startseite» mit je
  einem Kästchen «Ausblenden»; der Zustand zeigt «sichtbar», «nur Seitenleiste», «nur Startseite»
  oder «ausgeblendet». Die Vorschau zeigt beide Orte getrennt.

## 14. Stufen der Umsetzung (freigegeben 14. November 2026)

Je Stufe ein Zweig ab `main` in eigenem Worktree, ein PR mit Belegen (Stufe 1 der Teststufen je
Schritt, `task ci` vor dem PR), ein Release. Gemergt und getaggt wird nur auf Marcos Wort; CT 190 hebt
Marco per Updater.

| Stufe | Punkte | Inhalt | Berührt |
|---|---|---|---|
| 1 | P-5, P-2, P-3 | Segmente robust gegen kaputte Folge (6.4), abgewiesener Schlüssel ohne Wiederholen und Neu-Einreihen, alte Fehlschläge einmal löschen (O-4), YouTube-Tageslimit erkennen | Server, Verwaltung (Schlüssel) |
| 2 | P-1, P-4 | Trailerwahl nach Auflösung mit `trailers.size` und Bestandslauf (O-2, O-3), weitere Sprachen, Ländervorrang (O-8), Ersatz ohne Sprache als letzter Rang (O-7) | Server, Migration, Vertrag, Bleistift-Dialog |
| 3 | P-6, P-7 | Staffeln als Kachelwand, Folgen als kompakte Liste (O-11), Besetzung im Hochformat 2:3 | nur Web |
| 4 | P-8 | Personenseite: Angaben und Biografie aus TMDB (O-13, O-14), Porträt `w300` statt `w185` (O-12) | Server, Migration, Vertrag, Web |
| 5 | P-9, P-11 | Radio und Sammlungen in «Meine Medien», Radio-Kachel (O-15), Verwaltungsseite «Meine Medien» mit getrenntem Ausblenden (O-20, O-21) | Server, Migration, Vertrag, Web |
| 6 | P-10 | Sammlungen: Übersicht mit Filter (alle), Startseiten-Reihe ab 2 Titeln (O-16), Seite als nummerierte Liste, Beschreibung (O-18), «Gesehen» für alle (O-19) | Server, Migration, Vertrag, Web |

- Stufe 3 läuft parallel zu Stufe 1 in einer eigenen Session (nur Web, keine Überschneidung).
- Stufen 2, 4, 5 und 6 ändern Vertrag und Migrationen; sie laufen nacheinander, damit Vertragsfassung
  und Migrationsnummern eindeutig bleiben.
- KinoCheck (O-6) ist in keiner Stufe; die Prüfung folgt später als eigener Abschnitt.
