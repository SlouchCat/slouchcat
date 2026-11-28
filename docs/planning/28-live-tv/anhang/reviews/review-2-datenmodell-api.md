# Konzept 28 · Review 2 Datenmodell und API

Stand: 28. November 2026 · Reviewer: oh-my-claudecode:code-reviewer

Gegenstand: `07-datenmodell.md` (Fassung 1) und `08-api.md` (Fassung 1), geprüft wie ein Entwurf, der morgen als
Migration und als OpenAPI-Vertrag gebaut würde. Massstab: `LEITPLANKEN.md`, Auftrag Abschnitte 14 und 16 sowie
LTV-008, 009, 015, 018, die Fachdokumente 03, 04, 09, 11, 12, 14, 15 und der Bestand im Arbeitsbaum
(Migrationen, Abfragen, `store`, `auth`, `api/openapi.yaml`, `docs/dev/VERTRAEGLICHKEIT.md`). Es lief kein Build,
kein Test und kein `task`. «Bestand» heisst gelesen im Arbeitsbaum mit Datei und Zeile; «Annahme» heisst nicht
im Repo belegt.

Jeder Befund nennt Schwere, Stelle, Problem, Beleg, Korrektur und wie sicher die Aussage ist. Die Widersprüche,
die 07 Abschnitt 12.3 (W-1 bis W-24) und 08 Abschnitt 9.3 (W-25 bis W-29) schon auflisten, sind nicht wiederholt.

## Zählung

| Schwere | Anzahl | Befunde |
|---|---|---|
| kritisch | 1 | RD-01 |
| hoch | 7 | RD-02 bis RD-08 |
| mittel | 12 | RD-09 bis RD-20 |
| niedrig | 10 | RD-21 bis RD-30 |
| **zusammen** | **30** | |

---

## 1. Befunde

### RD-01 · kritisch · Löschhaken stoppt Aufnahmen innerhalb der Schreibtransaktion: Verklemmung der einzigen Schreibverbindung

- **Stelle:** 07 Abschnitt 10.1 Schritte 1 und 2, Abschnitt 10.2; 07 Abschnitt 8.1.
- **Problem:** Der Haken `profileDeleting` läuft **in** der Transaktion, die das Profil löscht. 07 lässt dort
  «laufende Aufnahmen des Profils stoppen» und setzt in Schritt 2 voraus, dass «laufende … nach 1. geschlossen»
  sind, also synchron. Ein Aufnahmeschreiber, der einen Teil schliesst, schreibt aber selbst in die Datenbank
  (`live_recording_parts.bytes`, `state = 'closed'`, `live_recordings.bytes`, 07 8.2) und braucht dafür die
  Schreibverbindung, die der Haken gerade hält. Der Schreiber wartet auf die Verbindung, der Haken auf den
  Schreiber. `database/sql` wartet beim Holen einer Verbindung aus einem vollen Pool ohne Zeitgrenze;
  `busy_timeout` hilft nicht, weil es keine zweite SQLite-Verbindung gibt. Folge: Jeder Schreibvorgang des Servers
  (Anmeldung, Fortschritt, Jobs, Aufnahmen) steht, bis der Prozess neu startet. Dasselbe gilt beim Löschen eines
  Kontos. Zugleich verletzt Schritt 1 die eigene Regel aus 07 8.1 «kein Netz- oder Dateizugriff innerhalb einer
  Transaktion» (Schliessen eines Teils heisst `fdatasync`).
- **Beleg:** `server/internal/store/store.go:94` (Schreibpool mit Grösse 1), `:124` (`SetMaxOpenConns`);
  `server/internal/auth/auth.go:74-79` (beide Haken bekommen die offene `*sql.Tx`); Aufruf in der Transaktion
  `server/internal/auth/profiles.go:555-556`, `server/internal/auth/accounts.go:186-192`.
- **Korrektur:** Zweistufig festlegen. (1) **Vor** der Löschtransaktion, ausserhalb jeder Transaktion: Live-Sitzungen
  des Profils beenden, neue Starts des Profils sperren (Sperre im Speicher je Profil), Haltegründe der laufenden
  Aufnahmen entfernen, auf das Ende der Schreiber warten (mit Frist, danach Abbruch des Löschens mit Fehler).
  (2) Der Haken in der Transaktion führt nur SQL aus:
  `UPDATE live_recordings SET state = 'deleting' WHERE profile_id = ? AND state NOT IN ('waiting','recording','recording_gap')`
  und gibt einen Fehler zurück, wenn danach noch eine laufende Zeile übrig ist (ein Fehler stoppt das Löschen,
  `auth.go:77-79`). Die Regel «Haken schreiben nur SQL, warten auf nichts» in 07 8.1 aufnehmen; ein Test löscht ein
  Profil mit laufender Aufnahme unter `-race` mit Zeitgrenze.
- **Sicherheit:** mittel. Ob es klemmt, hängt davon ab, ob das Stoppen synchron gebaut wird; der Text verlangt es.

### RD-02 · hoch · Wiedererkennung Stufe 1 ohne Gegenprobe: neu nummerierte `stream_id` oder getauschte `tvg-id` verschieben Favoriten, Nummern, EPG und Planungen still auf andere Sender

- **Stelle:** 07 Abschnitt 7.1 Stufe 1 («erster eindeutiger Treffer gewinnt»), Index `live_channel_streams_xtream`;
  Abschnitt 7.2 (Import übernimmt Name, Logo, Land, Sprache ohne Sperre).
- **Problem:** Eine Xtream-Plattform, die beim Umzug alle `stream_id` neu vergibt (Annahme, kommt bei
  Panel-Wechseln vor), lässt jeden Eintrag eine **andere** bestehende Senderquelle treffen. Nichts fehlt, also greift
  die Schutzschwelle «mehr als ein Drittel verschwunden» (07 7.3 Schritt 3) nicht. Ohne Sperre übernimmt der Import
  den neuen Namen: Sender 7 «SRF 1» heisst danach «ProSieben», behält aber Nummer 1, Favoriten, EPG-Zuordnung,
  geplante Aufnahmen und Serienregeln. Am Abend nimmt Slouchcat den falschen Sender unter dem richtigen Titel auf.
  Dasselbe im Kleinen bei zwei M3U-Einträgen, die ihre `tvg-id` tauschen (Pflegefehler einer Liste; beide sind
  weiterhin eindeutig, Stufe 1 trifft).
- **Beleg:** Auftrag LTV-018 (Z. 1003-1035): «Erhalt von Favoriten … EPG-Zuordnungen … Aufnahmeplanungen», «Ein
  fehlerhafter Import darf eine funktionierende Senderliste nicht zerstören».
- **Korrektur:** Einen Treffer der Stufe 1 nur annehmen, wenn mindestens ein zweites Merkmal passt (`name_key`
  oder `url_fp`), oder wenn sich höchstens eines der beiden ändert. Widersprechen beide, gilt die Identität als
  unsicher: Stufe 2 und 3 versuchen; trifft keine, Eintrag `kind = 'changed'` mit Code `identity_conflict` im
  Bericht und **nicht** anwenden. Zweite Schutzschwelle in 7.3 Schritt 3: Ändern mehr als ein fester Anteil der
  Stufe-1-Treffer zugleich ihren Namen (Vorschlag 10 %, Annahme), wird der Lauf `held`. Testfall mit permutierten
  `stream_id` in `17-test-und-abnahme.md`.
- **Sicherheit:** hoch.

### RD-03 · hoch · Anwenden in Stapeln zu 500 ist nicht transaktional; Teilzustand und überschriebene Sperren

- **Stelle:** 07 Abschnitt 7.3 Schritte 2 und 4; Abschnitt 10.3 letzter Absatz.
- **Problem:**
  1. Schritt 2 vergleicht in einer Lesetransaktion, Schritt 4 schreibt bis zu 22 Stapel (11'000 Senderquellen,
     07 11.1). Bricht der Lauf in Stapel 9 ab (Neustart, Fehler), sind neue Sender (mit `new_enabled = 1` sofort
     sichtbar), Umbenennungen und neue Adressen aus den Stapeln 1 bis 8 wirksam. «Abgebrochener Import ändert
     nichts» stimmt nur für «verschwunden». LTV-018 verlangt «transaktionale Aktualisierung».
  2. Zwischen zwei Stapeln kann der Administrator einen Sender umbenennen und damit eine Sperre setzen; der spätere
     Stapel schreibt den Namen aus dem Vergleichsstand von Schritt 2 trotzdem. 7.2 «Der Import schreibt gesperrte
     Felder nie» gilt dann nicht.
  3. 10.3 sagt «alles in einer Transaktion» und im selben Absatz «in Stapeln»; die Löschweitergabe von
     `live_sources` auf `live_channel_streams` löscht ohnehin alle Senderquellen in **einer** Anweisung.
- **Beleg und Rechnung:** Die 500er-Regel (`server/internal/jobs/queue.go:5-7`) begrenzt Job-Stapel, sie ist keine
  Vorgabe gegen Atomarität. 11'000 Upserts in einer Transaktion mit WAL und `synchronous=NORMAL`
  (`store.go:114`) dauern auf einer SSD Zehntel-, auf einer HDD wenige Sekunden (Annahme, zu messen). Die Aufnahme
  schreibt alle 10 Sekunden (07 8.2) und puffert nach 11 ohnehin; zwei Sekunden Wartezeit verträgt sie.
- **Korrektur:** In **einer** Schreibtransaktion je Quelle anwenden; alles Rechnen vorher, kein Netz und keine Datei
  darin. Gesperrte Felder in der Schreibtransaktion neu lesen (`… WHERE NOT EXISTS (SELECT 1 FROM
  live_channel_locks …)`), nie aus dem Vergleichsstand. Dauer als Messpunkt aufnehmen; nur wenn sie eine feste
  Grenze überschreitet (Vorschlag 2 s), eine Zwischentabelle befüllen und mit einer kurzen Übernahmetransaktion
  umschalten. 10.3 auf eine Transaktion festlegen (Kosten siehe RD-04).
- **Sicherheit:** hoch.

### RD-04 · hoch · Kindspalten von Fremdschlüsseln ohne Index: Löschen einer Quelle wird zum Vollscan in Milliardenhöhe auf der einzigen Schreibverbindung

- **Stelle:** 07 Abschnitte 3.2, 3.5, 3.8, 3.9: `live_stream_events.stream_id` und `.source_id`,
  `live_source_import_items.stream_id` und `.channel_id`, `live_recording_parts.stream_id`,
  `live_epg_sources.input_source_id`, `live_sessions.session_id` und `.recording_id` (nur Teilindizes mit
  `WHERE ended_at IS NULL`), `live_reminders.account_id`, `live_recording_rules.account_id`.
- **Problem:** SQLite prüft beim Löschen einer Elternzeile jede Kindtabelle über die Kindspalte; fehlt ein passender
  Index, ist das je gelöschter Zeile ein Vollscan der Kindtabelle (SQLite-Dokumentation «Foreign Key Support»,
  Abschnitt «Required and Suggested Database Indexes»; Wissen, nicht im Repo belegt). Teilindizes mit
  Bedingung auf eine andere Spalte dienen dieser Prüfung nicht.
- **Rechnung:** Quelle mit 11'000 Senderquellen löschen (07 11.1): Die Löschweitergabe entfernt 11'000 Zeilen aus
  `live_channel_streams`. Für jede prüft SQLite `live_stream_events` (bis 50'000 Zeilen, `ON DELETE SET NULL`),
  `live_source_import_items` (bis 50'000) und `live_recording_parts` (3'000): rund 11'000 × 103'000 ≈ 1,1 Mrd.
  Zeilenbesuche. Auf einer NAS-CPU sind das Minuten, in denen jede Schreibanfrage des Servers wartet.
- **Korrektur:** Indizes `live_stream_events (stream_id)`, `(source_id)`; `live_source_import_items (stream_id)`,
  `(channel_id)`; `live_recording_parts (stream_id)`; `live_sessions (session_id)`, `(recording_id)`;
  `live_epg_sources (input_source_id)`. Einfacher für die Diagnose: `live_stream_events.stream_id` und `.source_id`
  als blosse Zahl ohne Fremdschlüssel (die Ereignisse sind laut LTV-011.X-01 kein Bewertungssystem; ein Verweis ins
  Leere ist harmlos). Ein Schematest gleicht `PRAGMA foreign_key_list` mit `PRAGMA index_list` ab, damit keine
  Kindspalte ohne führenden Index bleibt.
- **Sicherheit:** hoch.

### RD-05 · hoch · Werkseinstellung schaltet Live-TV für Profile in Konten mit Altersgrenze ein

- **Stelle:** 07 Abschnitt 3.11 (`UPDATE profiles SET can_live_tv = 1, … WHERE max_rating = 18`), Satz «Ein neues
  Profil bekommt seine Werte vom Dienst: mit Altersgrenze alles aus»; 07 3.9 und F-1 (`owner_age_limited` aus
  `min(profiles.max_rating, accounts.max_rating)`).
- **Problem:** Ein Konto kann selbst eine Altersgrenze haben; seine Profile stehen dann oft auf
  `profiles.max_rating = 18`, wirken aber begrenzt. Die Werkseinstellung setzt deren Profilschalter auf 1. Gibt der
  Administrator dem Konto später Live-TV, sehen diese Profile sofort alle Sender, ohne dass das Hauptprofil
  entschieden hat. Zugleich nutzt 07 zwei Bedeutungen von «Profil mit Altersgrenze»: bei der Werkseinstellung nur
  die Grenze des Profils, bei den Aufnahmen die wirksame Grenze aus Profil und Konto.
- **Beleg:** `server/migrations/00063_minimum_age_accounts.sql:35` (`accounts.max_rating`), `00002_accounts_sessions.sql:31`
  (Vorgabe 18 für Profile); Auftrag LTV-015 «Der Hauptnutzer eines Accounts entscheidet ausdrücklich»; 03 DR-01;
  Leitplanken 4 («Kinderprofil … jedes Profil mit Altersgrenze unter 18»).
- **Korrektur:** Eine Definition für alle Stellen: `min(profiles.max_rating, accounts.max_rating) < 18`. Werkseinstellung:
  `… WHERE max_rating = 18 AND account_id IN (SELECT id FROM accounts WHERE max_rating = 18)`. Der Dienst nutzt
  dieselbe Funktion für neue Profile und für `owner_age_limited`. Test mit einem Konto `max_rating = 12`.
- **Sicherheit:** hoch.

### RD-06 · hoch · Die Verwaltung sieht, welches Konto gerade welchen Sender aufnimmt

- **Stelle:** 08 Abschnitt 3.9 `LiveIngest.consumers` («recordings only as `{kind: recording, accountEmail}`»);
  15 Abschnitt 3.12 («Aufnahme · Konto familie.keller@example.ch»).
- **Problem:** Ein Eingang ist ein Sender zu einer Zeit. Mit dem Konto daneben liefert das öffentliche EPG den Titel
  der privaten Aufnahme («Konto X nimmt jetzt SRF 1 auf» = «Tatort»). Das ist genau, was die Leitplanke ausschliesst.
- **Beleg:** Leitplanken 4: «Der Administrator sieht nie Titel fremder Aufnahmen, nur Belegung je Konto und
  technische Zustände»; 03 DR-17 (geht 15 vor): «Aufnahmen erscheinen dort nur als Zahl je Sender, ohne Profil und
  Titel … keine Titel, Sender oder Zeiten»; 07 Abschnitt 5.1: «laufende Aufnahmen je Sender sind eine Zahl ohne
  Profil»; 15 Abschnitt 4 Zeile LA-01: Konten «nicht sichtbar». 15 widerspricht sich zwischen LA-01 und LA-12; 08
  folgt der schwächeren Stelle. Nicht in W-1 bis W-29.
- **Korrektur:** `LiveIngest.recordings` bleibt eine Zahl; `consumers` enthält nur Zuschauer. 15 LA-12 anpassen, neuen
  W-Eintrag führen. Fehlersuche an einer Aufnahme bleibt über ihre Kennung im Protokoll (15 Abschnitt 4).
- **Sicherheit:** hoch.

### RD-07 · hoch · Meldung «Kontingent voll» an das Hauptprofil fehlt im Modell; nach Muster gebaut, verriete sie Titel erwachsener Profile

- **Stelle:** 07 Abschnitt 3.11 `notifications` («der Eintrag gehört dem Profil, dem auch die Aufnahme gehört»);
  08 Abschnitt 3.10 `Notification.live` mit `title` und `channelName`.
- **Problem:** Verlangt ist eine Meldung an Profil **und** Hauptprofil. Das Modell kennt nur den Eintrag beim Besitzer.
  Wer den zweiten Eintrag nach demselben Muster baut (`live_detail` mit Titel, Sender, Zeiten), zeigt dem Hauptprofil
  Titel aus einem Profil **ohne** Altersgrenze; dasselbe gilt für externe Kanäle (11 Abschnitt 11: Profilname vorn).
- **Beleg:** Leitplanken 4 «abgelehnt und gemeldet (Profil und Hauptprofil)»; 03 DR-06; 11 Abschnitt 11, Zeile
  «Aufnahme abgelehnt, Kontingent voll | Profil **und** Hauptprofil»; Auftrag LTV-009 «Andere Profile desselben
  Accounts dürfen diese Aufnahmen nicht sehen».
- **Korrektur:** Eigene Art (Vorschlag `live_quota_rejected_profile`) für den Eintrag beim Hauptprofil, mit
  `live_detail` ohne Titel, Sender und Zeiten («Eine Aufnahme von Lea wurde abgelehnt: Kontingent voll»), Titel nur
  bei `owner_age_limited = 1`; `dedupe_key` je Aufnahme. Gleiche Regel für `deliveries`. Test: Meldung an das
  Hauptprofil für ein erwachsenes Profil enthält keinen Titel (Bytesuche).
- **Sicherheit:** hoch.

### RD-08 · hoch · «Sicherung ohne Geheimnisse» aus 12 lässt sich mit den CHECK-Regeln aus 07 nicht bauen

- **Stelle:** 07 Abschnitt 6.3 («Die Sicherung enthält `slouchcat.db` mit den verschlüsselten Werten»); CHECK-Regeln
  in 07 3.2, 3.3, 3.5: `live_sources ((kind = 'm3u_file') = (url_enc IS NULL))`, `(kind <> 'xtream' OR username_enc IS
  NOT NULL)`, `live_source_headers.value_enc NOT NULL`, `live_channel_streams (url_enc IS NOT NULL OR
  xtream_stream_id IS NOT NULL)`, `live_epg_sources ((kind = 'url') = (url_enc IS NOT NULL))`.
- **Problem:** 12 legt fest, dass die Sicherungskopie alle verschlüsselten Live-Spalten auf `NULL` setzt (SM-18,
  Stufe 1). Das verletzt jede der genannten Regeln; die Kopie scheitert, oder es braucht einen Umbau der Tabellen in
  der Kopie. Der Widerspruch 07 ↔ 12 steht in keiner W-Liste.
- **Beleg:** `12-sicherheit.md:495` und SM-18 (`:903`); Bestand: die Kopie wird mit gültigen Fremdschlüsseln und
  Regeln bereinigt (`server/internal/admin/backup.go:332-353`).
- **Korrektur:** Statt `NULL` einen Platzhalter `v0:` («Zugang fehlt») schreiben. Das Muster `GLOB 'v[0-9]*:*'`
  lässt ihn schon zu; der Dienst erkennt `v0:` als «Zugang neu eingeben» (07 6.3). Alle CHECK-Regeln bleiben gültig,
  und die Bytesuche von SM-18 findet weder Klartext noch Chiffrat. 07 6.3 an 12 angleichen; Entscheid bleibt 12 F-4.
- **Sicherheit:** hoch.

### RD-09 · mittel · Termin-Schlüssel einer Regel enthält Sender und Beginn; EPG-Verschiebung und Zusammenführen erzeugen Doppel

- **Stelle:** 07 Abschnitt 3.9 `rule_key` (nur Verweis auf 11), Index `live_recordings_rule_key`; 07 7.4.
- **Problem:** Der Schlüssel ist «Regel, Sender, Beginn laut EPG, Prüfsumme des Titels» (11, Tabelle in Abschnitt 2).
  Verschiebt `livetv.epg.reconcile` eine Sendung (`moved_auto`, `prev_core_start`), bleibt `rule_key` beim alten
  Beginn; der nächste `livetv.dvr.expand` rechnet den neuen Schlüssel und legt einen **zweiten** Termin für dieselbe
  Sendung an (doppelte Belegung im Kontingent, zwei Schreiber). Dasselbe nach dem Zusammenführen (Sender im
  Schlüssel). Auch der Schutz aus W-17 (`cancelled/skipped`) wirkt dann nicht mehr: Der ausgelassene Termin kommt
  nach einer Verschiebung zurück.
- **Korrektur:** `rule_key` in 07 selbst definieren und beim Anpassen durch reconcile und beim Zusammenführen in
  derselben Transaktion neu setzen; oder den Schlüssel aus stabilen Merkmalen bilden (`prog_ext_id` bzw.
  `episode_ns`, wenn vorhanden, sonst Regel, Titelprüfsumme und lokales Datum). Testfall «Sendung um 5 Minuten
  verschoben ergibt keinen zweiten Termin».
- **Sicherheit:** mittel bis hoch (hängt an der Umsetzung von reconcile, die 07 nicht beschreibt).

### RD-10 · mittel · Anlegen von Aufnahme, Serienregel und Sitzung ist nicht idempotent

- **Stelle:** 08 Abschnitte 3.4 (`createLiveSession`), 3.5 (`createLiveRecording`), 3.6 (`createLiveRecordingRule`);
  07 3.9 (keine Eindeutigkeit für Einzelaufnahmen).
- **Problem:** Doppelt gesendetes «Aufnahme planen» (doppelter OK-Druck am TV, Wiederholung der App nach
  Zeitüberschreitung) legt zwei Zeilen an; die Warnung `duplicate` speichert trotzdem (08 3.5 «trotz Warnung
  gespeichert»). Zwei gleiche Serienregeln planen jede Folge zweimal. Ein wiederholtes `createLiveSession` bei einem
  Konto mit einem erlaubten Strom (`accounts.max_streams`, `00063_minimum_age_accounts.sql:42`) antwortet 409
  `stream_limit_reached`, während die erste Sitzung bis zu 60 Sekunden ohne Herzschlag weiterlebt (08 4.2): Der
  Zuschauer ist eine Minute gesperrt.
- **Korrektur:** (a) Teil-Eindeutigkeit `live_recordings (profile_id, channel_id, prog_start_utc, prog_title_norm)
  WHERE kind = 'single' AND state NOT IN ('cancelled','deleting')`; bei Treffer 200 mit der bestehenden Zeile (wie
  bei Erinnerungen, 08 3.6). (b) Serienregel eindeutig je `(profile_id, title_norm, ifnull(channel_id, 0))` für
  `kind = 'series'`. (c) Eine neue Sitzung mit gleichem `deviceId` und Profil beendet die offene alte
  (`ended_by = 'client'`) unter derselben Startsperre, bevor die Grenze zählt.
- **Sicherheit:** hoch.

### RD-11 · mittel · Zusammenführen scheitert an der Eindeutigkeit der Erinnerungen und verdoppelt Planungen

- **Stelle:** 07 Abschnitt 7.4 («Erinnerungen, Planungen und Regeln wandern mit»); 3.6
  `UNIQUE (profile_id, channel_id, prog_start_utc, prog_title_norm)`.
- **Problem:** Duplikate sind gerade Sender mit gleichem Programm. Hat ein Profil dieselbe Sendung auf beiden Sendern
  als Erinnerung, verletzt das Umhängen die Eindeutigkeit, und die ganze Zusammenführung bricht ab. Planungen haben
  keine Eindeutigkeit; nach dem Umhängen nimmt das Profil dieselbe Sendung zweimal vom selben Sender auf. Nur die
  Favoriten sind mit `INSERT OR IGNORE` bedacht.
- **Korrektur:** Für Erinnerungen und Einzeltermine je Profil eins behalten, den Rest löschen (`UPDATE OR IGNORE`,
  dann `DELETE` der übrigen), Serienregeln mit gleichem `title_norm` vereinigen. `previewLiveChannelMerge` nennt
  die Zahl der wegfallenden Doppel.
- **Sicherheit:** hoch.

### RD-12 · mittel · Ablauf oder Widerruf einer Anmeldung löscht Live-Sitzungszeilen samt Umschaltverlauf

- **Stelle:** 07 Abschnitt 3.8 `live_sessions.session_id … REFERENCES sessions (id) ON DELETE CASCADE`,
  `live_session_tunes` mit Löschweitergabe.
- **Problem:** Der Bestand löscht abgelaufene und widerrufene Anmeldungen regelmässig und schont nur solche, auf die
  `playback_sessions` zeigt. Abmelden, Passwortwechsel und Ablauf eines Browser-Cookies löschen damit Zeilen, die 07
  Abschnitt 9 vierzehn Tage halten will (Rangliste «automatisch» nach 04 6.2, Verwaltung, Schauzeit). Der Wert
  `ended_by = 'logout'` überlebt kaum.
- **Beleg:** `server/queries/app/auth.sql:67-74` (`DeleteExpiredSessions`).
- **Korrektur:** `session_id INTEGER REFERENCES sessions (id) ON DELETE SET NULL` (Spalte nullbar), Index darauf
  (RD-04). Die Alternative, `DeleteExpiredSessions` um `live_sessions` zu erweitern, koppelt `auth` an eine
  Live-Tabelle und ist schlechter.
- **Sicherheit:** hoch.

### RD-13 · mittel · Die Löschhaken des Bestands sind belegt und haben für das Profil keinen Nachlauf

- **Stelle:** 07 Abschnitte 10.1 («Im Haken `profileDeleting`»), 10.2; 08 2.11.
- **Problem:** `accountDeleting` und `profileDeleting` sind je **ein** Funktionsfeld und heute von `wishes` belegt.
  `profileDeleting` gibt nur einen Fehler zurück, keine Funktion «nach dem Commit» (anders als `accountDeleting`);
  07 10.1 Schritt 4 braucht sie. Ohne Änderung an `auth` überschreibt Live den Haken von `wishes` oder umgekehrt.
- **Beleg:** `server/internal/auth/auth.go:74-79`, `:153`, `:168`.
- **Korrektur:** In 07 und in `21-aenderungen-an-dokumenten.md` festhalten: `auth` bekommt eine geordnete Liste von
  Haken (oder `main.go` verkettet `wishes` und `live` in einer Funktion), `profileDeleting` liefert wie
  `accountDeleting` eine Nachlauf-Funktion; Reihenfolge `wishes` vor `live`.
- **Sicherheit:** hoch.

### RD-14 · mittel · Kontingent: Formel, Ort und Wettlauf nicht festgelegt

- **Stelle:** 07 Abschnitt 5.1 («Belegung je Konto ist `SUM(bytes)`»), 3.9 `expected_bytes`, `files_missing`;
  8.3 (Startübergang ohne Kontingentbedingung).
- **Problem:**
  1. `SUM(bytes)` zählt weder die Prognose laufender Aufnahmen («das Grössere aus geschrieben und erwartet», 11
     8.1, 03 DR-06) noch schliesst es Aufnahmen mit fehlenden Dateien aus («zählt mit 0 Byte», 11 10.6, 07 6.3).
     Start, Vorschau, LA-16 und `Account.live.usedBytes` würden verschieden rechnen.
  2. Zwei Aufnahmen desselben Kontos starten gleichzeitig (Serien auf zwei Sendern um 20:15). Liest jede die Summe
     über den Lesepool und schaltet dann mit dem bedingten Update aus 8.3, passen beide «noch» hinein.
- **Korrektur:** Eine benannte Abfrage in `liveq` für alle Stellen, sinngemäss
  `SUM(CASE WHEN files_missing = 1 THEN 0 WHEN state IN ('waiting','recording','recording_gap') THEN
  max(bytes, ifnull(expected_bytes, 0)) ELSE bytes END)` über alle Zustände ausser `scheduled`, `suspended`,
  `cancelled`. Prüfung und Übergang in derselben Schreibtransaktion (die eine Schreibverbindung mit
  `_txlock=immediate` serialisiert das), zum Beispiel als Bedingung im `UPDATE … SET state = 'recording'`. Kein
  laufend geführter Zähler (vermeidet Abweichungen).
- **Sicherheit:** hoch.

### RD-15 · mittel · Planer-Zeitgeber im Speicher gegen Änderungen in der Datenbank

- **Stelle:** 07 Abschnitt 1.3 («Zeitgeber-Liste des Planers»), 8.3, 8.4.
- **Problem:** Die bedingten Updates schützen den Zustand, nicht Zeit und Sender. Ändern das Profil (PATCH),
  reconcile (`core_start`) oder das Zusammenführen (`channel_id`) einen Termin, feuert der Zeitgeber zur alten Zeit
  oder am alten Sender, weil `state IN ('scheduled','waiting')` weiter stimmt. Umgekehrt schreibt reconcile einen
  Termin um, der eben in `waiting` gegangen ist; «ändert nie einen Termin, dessen Vorlauf schon begonnen hat»
  (11 Abschnitt 2) ist im SQL nicht abgesichert. 8.4 legt nicht fest, ob die Zeitgeber vor oder nach Schritt 4
  geladen werden.
- **Korrektur:** Spalte `rev` in `live_recordings`, die jede Änderung eines Termins erhöht; der Zeitgeber trägt `rev`
  und startet mit `… WHERE id = ? AND state = 'scheduled' AND rev = @rev`; trifft das nicht, liest er neu und plant
  neu. reconcile und PATCH schreiben nur mit `WHERE state = 'scheduled'`. 8.4: Zeitgeber erst nach Schritt 4 laden.
- **Sicherheit:** mittel.

### RD-16 · mittel · `lt`-Fehler 404 und Segmente ausserhalb des Fensters sind für den Player nicht unterscheidbar

- **Stelle:** 08 Abschnitt 4.2 Zeile «Fehler»; 4.1 Segment-Route.
- **Problem:** 08 legt nicht fest, was ein Segmentabruf ausserhalb von Ring oder Fenster bekommt (ohne Timeshift
  liegen nur drei Segmente vor, 08 4.1). Ist es 404 wie «Token ungültig», kann der Player «neu anlegen» nicht von
  «zu spät, weiter mit dem nächsten» trennen. Die Apps reagieren heute an Token-Routen auf 401 (Bestand), jede
  Reaktion wird neu. Ein JSON-Körper auf einer HLS-Route erreicht Media3 und hls.js nicht verwertbar (Annahme).
- **Beleg:** `server/internal/api/stream_token.go:23-30`; 08 W-29.
- **Korrektur:** In 08 4.2 und in 16 als Client-Regel festschreiben: An `lt`-Routen wertet der Client einen Fehler
  nie selbst als «Sitzung weg» aus, sondern fragt den Herzschlag; nur dessen 410 heisst «neu anlegen». Für
  Segmente ausserhalb des Fensters 404 ausdrücklich nennen und die Wiederholungsgrenzen der Player für Live
  festlegen (hls.js-Einstellungen, Media3 `LoadErrorHandlingPolicy`).
- **Sicherheit:** mittel.

### RD-17 · mittel · Regel 12: Aufnahmen und Regeln liefern keine Aktionen

- **Stelle:** 08 Abschnitt 3.5 `LiveRecordingBrief`, `LiveRecording`; 3.6 `LiveRule`; Tabelle in Abschnitt 6.
- **Problem:** Welche Knöpfe eine Aufnahme zeigt, hängt an Serverregeln: Das Hauptprofil darf löschen, abspielen nur
  mit eigenem Live-TV (03 DR-03), schreibt keinen Fortschritt (DR-21), darf nicht ändern oder stoppen; «Bis
  Sendungsende verlängern» nur bei einer Sofortaufnahme mit EPG; «Ändern» nur vor dem Vorlauf; «Datei fehlt» ist
  nicht abspielbar; laufende sind nicht löschbar (409). `LiveProgramme` hat `actions`, `LiveRecording` und `LiveRule`
  nicht. Der Client müsste aus `kind`, `state`, `ownedByOtherProfile` und `SessionInfo.live` rechnen; das ist eine
  Rechteregel im Client (CLAUDE.md Regel 12).
- **Korrektur:** Pflichtfeld `LiveRecording.actions: {play, resume, delete, stop, extend, edit, retention}` und
  `LiveRule.actions: {edit, delete, clearEpisodes}`; in die Tabelle von 08 Abschnitt 6 aufnehmen.
- **Sicherheit:** hoch.

### RD-18 · mittel · Zugriff des Hauptprofils: Gegenrichtung offen, Liste und Einzelabruf uneins, Kindtermine erreichbar

- **Stelle:** 07 Abschnitt 4.3 `GetAccessibleLiveRecording`; 08 3.5 `listProfileLiveRecordings`; 14 Abschnitt 3.14.
- **Problem:**
  1. `owner_age_limited` bleibt 1, wenn das Hauptprofil die Altersgrenze später **entfernt** (das Kind wird
     volljährig). Das Hauptprofil sieht die alten Aufnahmen weiter, obwohl 14 LS-14 sagt «Profile ohne Altersgrenze
     erscheinen hier nie» und die Leitplanke Profile ohne Grenze «ganz privat» nennt. 04 F-10 fragt nur die
     Gegenrichtung (Grenze nachträglich gesetzt).
  2. LS-14 gruppiert nach heutigen Profilen mit Grenze, der Einzelabruf nach dem Schnappschuss: Es gibt Aufnahmen,
     die per Kennung erreichbar sind und in keiner Liste stehen.
  3. Die Abfrage liefert auch Termine des Kinds (`scheduled`, `suspended`, `waiting`); 08 sagt «bis zur Antwort nur
     die Zahl» (14 F-4). `getLiveRecording` mit der Kennung eines Kindtermins trifft trotzdem.
- **Korrektur:** Für 1 die Frage F-10 um die Gegenrichtung ergänzen (Vorschlag: beides verlangen,
  `owner_age_limited = 1` **und** heute wirksame Grenze unter 18). Für 2 und 3 dieselbe Bedingung als sqlc-Fragment
  für Liste und Einzelabruf, für fremde Zeilen bis zur Antwort auf 14 F-4 zusätzlich
  `r.state NOT IN ('scheduled','suspended','waiting')`.
- **Sicherheit:** hoch für 2 und 3; 1 ist eine Entscheidung für Marco.

### RD-19 · mittel · User-Agent im Klartext, die Abnahme verlangt eine Bytesuche über alle Kopfzeilenwerte

- **Stelle:** 07 Abschnitt 3.2 `live_sources.user_agent`, 3.3 `live_channel_streams.list_user_agent`, 6.1 («Im
  Klartext stehen nur `site`, …, der User-Agent»).
- **Problem:** 04 T-28 und 12 lassen im Klartext nur Schema, Host und Port sowie die **Namen** der Kopfzeilen zu.
  LTV-003.18 behandelt den User-Agent als eigene Kopfzeile; manche Anbieter schalten Zugänge über einen festen
  User-Agent frei (Annahme). SM-14 prüft per Bytesuche, dass kein Kopfzeilenwert in der Datenbank steht, und
  scheitert daran. Widerspruch nicht in W-Liste.
- **Beleg:** 04 T-28 (Zeile 96); `12-sicherheit.md:489`, SM-14 (`:899`).
- **Korrektur:** `user_agent_enc` und `list_user_agent_enc` verschlüsseln; die API meldet nur, ob ein eigener
  User-Agent gesetzt ist. Oder T-28 und SM-14 ausdrücklich um «User-Agent im Klartext» ergänzen (Entscheid).
  Empfehlung: verschlüsseln, es kostet nichts.
- **Sicherheit:** hoch.

### RD-20 · mittel · Erinnerungen: Eindeutigkeit über alle Zustände

- **Stelle:** 07 Abschnitt 3.6 `UNIQUE (profile_id, channel_id, prog_start_utc, prog_title_norm)`; 08 3.6 («gleiche
  Sendung zweimal: 200 mit der bestehenden»); 07 Abschnitt 9 (`cancelled` bleibt bis 7 Tage nach Sendungsende).
- **Problem:** Entfernt ein Profil eine Erinnerung und setzt sie neu, liefert der Server die **abgesagte** Zeile mit
  200 zurück; es wird nie erinnert. Verschiebt reconcile eine Sendung (`moved_auto`), kann das Update mit einer
  zweiten Erinnerung derselben Sendung kollidieren und den Job abbrechen.
- **Korrektur:** `deleteLiveReminder` löscht die Zeile (`cancelled` nur für vom System abgesagte), oder die
  Eindeutigkeit gilt nur `WHERE state = 'planned'`; reconcile mit `UPDATE OR IGNORE` und Löschen des Doppels.
- **Sicherheit:** hoch.

### RD-21 · niedrig · Wiederanlauf setzt das Ende offener Sitzungen auf «jetzt»

- **Stelle:** 07 Abschnitt 8.4 Schritt 1 (`ended_at = jetzt`).
- **Problem:** Nach einem Stromausfall über Nacht zählen offene Sitzungen und Umschaltzeilen Stunden zu lang. Das
  verfälscht die Rangliste «automatisch» (Tage mit mindestens 30 Sekunden, 04 6.2) und die Statistik der Verwaltung.
- **Korrektur:** `ended_at = last_seen_at` der Sitzung (höchstens eine Minute alt, 07 8.2), für offene
  `live_session_tunes` derselbe Wert.
- **Sicherheit:** hoch.

### RD-22 · niedrig · Eine CHECK-Regel lässt wegen NULL den halben Zustand zu, den 07 ausschliessen will

- **Stelle:** 07 Abschnitt 3.3 `CHECK ((strategy = 'scheduled') = (schedule_id IS NOT NULL))`; 3.4 Satz «die CHECK-Regel
  in `live_channels` verhindert einen halben Zustand».
- **Problem:** Mit `strategy IS NULL` und gesetzter `schedule_id` ergibt der Ausdruck NULL, und SQLite wertet NULL als
  bestanden. Genau der Zustand nach einem vergessenen Leeren beim Löschen eines Zeitplans geht durch.
- **Korrektur:** `CHECK ((strategy IS 'scheduled') = (schedule_id IS NOT NULL))`, als Spalten-CHECK auf `schedule_id`
  in Stufe 3 (07 11.2). Alle `(a = b)`-Regeln mit nullbaren Spalten so prüfen.
- **Sicherheit:** hoch.

### RD-23 · niedrig · `epg.db` räumt Daten gelöschter EPG-Quellen nie weg

- **Stelle:** 07 Abschnitt 3.7 (`epg_channels.source_id` ohne Fremdschlüssel), Abschnitt 9 (Aufräumen nur nach
  `stop_utc`).
- **Problem:** Nach `deleteLiveEpgSource` bleiben `epg_channels`, ihre Sendungen bis zum Ablauf und `epg_suggestions`
  liegen; `epg_channels` werden nie entfernt. Wegen `AUTOINCREMENT` greifen sie nie auf eine neue Quelle über, aber
  die Datei wächst und LA-11 kann alte Kanäle vorschlagen.
- **Korrektur:** `deleteLiveEpgSource` löscht in `epg.db` `epg_channels WHERE source_id = ?` (Löschweitergabe auf
  Sendungen und Vorschläge); die Prune-Aufgabe gleicht beim Start die `source_id` beider Dateien ab.
- **Sicherheit:** hoch.

### RD-24 · niedrig · Das Bild einer Aufnahme hängt am EPG-Zwischenspeicher

- **Stelle:** 07 Abschnitt 3.9 `image_key` (= `epg_images.url_sha256`), Abschnitt 9 Zeile `epg_images` («solange
  eine Sendung verweist»).
- **Problem:** Sendungen bleiben 24 Stunden nach Ende (03 DR-20). Danach verliert jede Aufnahme ihr Bild, nach einem
  Neuaufbau von `epg.db` sofort. LS-08 zeigt dann nur noch das Senderlogo; DR-20 sagt, die Aufnahme behalte ihre
  Angaben.
- **Korrektur:** Beim Anlegen oder Abschluss das Bild in den Ordner der Aufnahme kopieren (`<recording_id>/poster`)
  oder die Prune-Aufgabe um die Schlüssel aus `live_recordings` ergänzen.
- **Sicherheit:** hoch.

### RD-25 · niedrig · Glocke: Live-Einträge sind nicht gegen Titel-Verweise abgegrenzt; Dedupe verschluckt Wiederholungen

- **Stelle:** 07 Abschnitt 3.11 `notifications_new`.
- **Problem:** Keine Regel verbietet `item_id`, `parent_id` oder `wish_id` an einem `live_*`-Eintrag; ein solcher ginge
  mit einem Titel der Bibliothek weg. `dedupe_key = 'live:<ref>:<kind>'` mit `UNIQUE (profile_id, dedupe_key)` lässt
  je Ziel und Art nur einen Eintrag: Eine zweite Verschiebung oder ein zweiter Konflikt desselben Termins erscheint
  nie.
- **Korrektur:** `CHECK (kind NOT GLOB 'live_*' OR (item_id IS NULL AND parent_id IS NULL AND wish_id IS NULL))`.
  Für `live_schedule_moved` und `live_schedule_conflict` festlegen, ob ein Upsert gilt (`created_at` neu,
  `read_at = NULL`).
- **Sicherheit:** hoch.

### RD-26 · niedrig · Widersprüche im Text, die in 07 12.3 und 08 9.3 fehlen

| Stelle | Widerspruch | Korrektur |
|---|---|---|
| 07 4.1 gegen 07 4.2 und 08 3.10 | «Der Dienst lehnt beim Speichern nur ab, was über den Deckel ginge» gegen «Ein Profil kann einen Schalter speichern, den das Konto nicht hat» und «das Hauptprofil darf setzen, was über den Deckel geht» | den Satz in 4.1 streichen |
| 07 11.2 letzter Punkt und O-6 | «muss in Stufe 3 nicht neu gebaut werden» und im selben Absatz «wird in Stufe 3 neu angelegt»; die Sicht nennt ihre Spalten ausdrücklich, Zuschauer brauchen `strategy` nicht | Neubau streichen; nötig ist er nur, wenn `live_channels` selbst neu gebaut wird |
| 08 3.3 `getLiveGuide` | verweist auf «W-24 in 07»; W-24 ist «Ab Werk an bei Einführung» | richtigen Verweis setzen |
| 08 2.12 gegen 3.10 | `Account.live.recordings` gegen `recordingsCount` | ein Name |
| 08 3.4 | `LiveHeartbeat.state` kennt `source_switched`, `LiveSession.state` nicht | Listen angleichen oder `source_switched` nur als `notice` |
| 09 Z. 766 gegen 07 3.11 | neue Spalte `live_reminder_id` gegen `live_ref` | W-Eintrag; 09 nachziehen |

### RD-27 · niedrig · Ausliefern nach App-Fassung widerspricht der eigenen Verträglichkeitsregel und deckt nicht alles

- **Stelle:** 08 Abschnitte 1.3, 7.2; O-1, O-7.
- **Problem:** 08 filtert `livetv` und `live_*` nach der Fassung in `X-Slouchcat-Client`. `VERTRAEGLICHKEIT.md`
  (Z. 58-62) nennt neue Aufzählungswerte ausdrücklich «nicht brechend, die Apps vertragen Unbekanntes». Der Filter ist
  zusätzliche Server-Logik je App-Fassung und verbirgt Meldungen wie «Aufnahme fehlgeschlagen» vor Nutzern, die nur
  die Handy-App öffnen. `getMyChannels` (`x-clients: [app]`, neue Ereignisse `live_*`) ist nicht gefiltert, also
  uneinheitlich.
- **Korrektur:** O-1 zuerst klären. Vertragen die erzeugten Kotlin-Clients unbekannte Werte, entfällt der Filter;
  sonst ist das ein Fehler der Apps gegen Konzept 27 Abschnitt 11.5 und gehört dort behoben, nicht im Vertrag.
- **Sicherheit:** mittel (hängt an O-1).

### RD-28 · niedrig · Antwortmodelle: Feld, das nie kommt; schwere Listenzeilen; eigene Blätterung

- **Stelle:** 08 3.7 `LiveSource.username` («Never returned»); 3.8 `listAdminLiveChannels` (Zeile =
  `AdminLiveChannel`); 3.5 `listProfileLiveRecordings`.
- **Problem:** Ein Antwortfeld, das nie gefüllt werden darf, steht im erzeugten Modell und lädt zum Füllen ein.
  `listAdminLiveChannels` mit `limit` bis 500 liefert je Zeile Senderquellen, Technik, ein 7-Tage-Aggregat aus
  `live_stream_events` und Zählungen aus Favoriten und Planungen: rund 1'500 Teilabfragen und bis etwa 1 MB je Seite
  (Annahme 2 KB je Zeile). `listProfileLiveRecordings` antwortet `{profiles: [… recordings]}` mit eigener Blätterung
  statt `{items, total, offset}` (08 1.1).
- **Korrektur:** `username` aus `LiveSource` streichen (nur `LiveSourceWrite`, `writeOnly`). Zeilenmodell
  `AdminLiveChannelRow` mit den Spalten von LA-05; Details nur in `getAdminLiveChannel`. LS-14 als Profilübersicht
  plus `listLiveRecordings?profileId=` mit der üblichen Seite.
- **Sicherheit:** hoch.

### RD-29 · niedrig · Lücken aus der Stichprobe Aktion → Operation

| Seite und Aktion | Lücke | Korrektur |
|---|---|---|
| LS-12 «Anderen Sender wählen» | `LiveRecordingUpdate` hat kein `channelId` | Feld für `manual` ergänzen oder Aktion als «löschen und neu planen» beschreiben |
| LS-09 «Einzeln geplant am 20.11.» | `LiveRecordingBrief` ohne `createdAt` | Feld ergänzen |
| LS-11 Reiter «Wiederkehrend», Abschnitt «Manuell» | `listLiveRecordings` ohne Filter `kind` | Parameter `kind` |
| LA-05 «Nummern ab 101 in der aktuellen Sortierung» | `LiveBulkRequest` ohne `sort` | Feld `sort` |
| LA-06 Logo «eigene Adresse» mit Vorschau vor dem Speichern | keine Vorschau-Operation; den fremden Host darf nur der Server abrufen (08 4.5) | `POST /admin/live/logo-preview` oder Vorschau erst nach dem Speichern (14/15 anpassen) |
| LA-07 Schritt 2 «Gruppen (Vereinigung als Vorgabe)» = abwählbar | `LiveMergeRequest`: «groups are always united» | 15 oder 08 angleichen |

### RD-30 · niedrig · Schema bereitet unbeantwortete Optionen vor

- **Stelle:** 07 3.2 `live_net_targets.epg_source_id` («nur, wenn du `09` F-4 (a) wählst»); 3.11 Art
  `live_recording_deleted` («nur, wenn du die Empfehlung von `14` F-4 nimmst»).
- **Problem:** Beide stehen im SQL unabhängig von der Antwort. CLAUDE.md Regel 11 («Nichts aus einem späteren
  Meilenstein vorziehen») und der Schlusssatz von Auftrag Abschnitt 16 («keine … ungenutzte Infrastruktur») gelten
  sinngemäss für Optionen, die Marco noch nicht gewählt hat.
- **Korrektur:** Aus dem SQL nehmen und als «nur bei Antwort (a)» im Text führen. Bei `notifications` zwingt der
  einmalige Neubau zu einer Vorab-Liste; dort den Wert erst nach der Antwort auf 14 F-4 aufnehmen.
- **Sicherheit:** hoch.

---

## 2. Streichbar oder vereinfachbar

Ohne eine Anforderung zu verletzen. Die Bewertung der Menge: Die 32 Tabellen sind bis auf die Punkte unten
begründet (jede trägt eine Seite oder eine Regel aus dem Auftrag). Von den 122 Operationen lassen sich rund zehn
streichen oder zusammenlegen; das ist aufgebläht an den Rändern, nicht im Kern.

| Nr. | Was | Vorschlag | Ersparnis |
|---|---|---|---|
| S-1 | `getLiveNowNext` | `listLiveChannels?ids=…&now=true` liefert dasselbe; 08 3.3 sagt selbst, die Senderliste brauche die Operation nicht | 1 Operation |
| S-2 | `testNewLiveSource` und `testLiveSource` | eine Operation mit freiwilliger `sourceId`, wie `testLiveEpgSource` («neu oder bestehend») | 1 Operation |
| S-3 | `getLiveImportPreview`, `listLiveImportPreviewItems` und die Vorschau im Arbeitsspeicher (30 min) | Vorschau als Lauf mit `outcome = 'preview'` in `live_source_imports` und `_items` (die Tabellen gibt es schon); dieselben Lese-Operationen wie für Läufe | 2 Operationen, ein zweiter Speicher |
| S-4 | `getAdminLiveChannelLogo` | `getLiveChannelLogo` mit Ausnahme für Administratoren (inaktive Sender) | 1 Binärroute |
| S-5 | `getLiveChannelRemovalImpact` | `AdminLiveChannel.usage` hat schon `favorites`, `scheduled`; um `reminders`, `rules` ergänzen | 1 Operation |
| S-6 | `getLiveSourceRemovalImpact` | Zähler «Sender ohne andere Quelle» in `LiveSource.counts` | 1 Operation |
| S-7 | `getLiveUpdateImpact` | Feld in `getLiveOverview`, die LA-27 ohnehin abfragt | 1 Operation |
| S-8 | `live_channels.merged_into` | Nichts liest die Spalte (alte Kennung antwortet 404, 07 10.4); Ketten und Kreise sind möglich, weil nichts prüft, dass das Ziel lebt. Streichen oder als blosse Auskunft ohne Fremdschlüssel | 1 Spalte, 2 CHECK-Regeln |
| S-9 | `live_source_import_items` für die letzten 10 Läufe | Einträge nur für den letzten Lauf je Quelle behalten (LA-04 zeigt das Ergebnis eines Laufs; ältere brauchen nur Zähler) | bis 45'000 Zeilen |
| S-10 | Fremdschlüssel in `live_stream_events` | blosse Zahlen (RD-04) | Löschkosten |
| S-11 | Filter nach App-Fassung (08 7.2) | streichen, falls O-1 ergibt, dass die Apps Unbekanntes vertragen (RD-27) | Server-Logik je Fassung |
| S-12 | `live_net_targets.epg_source_id` | erst mit der Antwort auf 09 F-4 (RD-30) | 1 Spalte, 1 Index |

---

## 3. Stichprobe Aktion → Operation

43 Aktionen quer durch LS-, LD- und LA-Seiten (14 Abschnitte 3 bis 5, 15 Abschnitt 3).

| Seite | Aktion (Wortlaut der Seite) | Operation | gefunden |
|---|---|---|---|
| LS-01 | Kontextmenü «Aufnehmen» | `createLiveRecording` | ja |
| LS-01 | «Zum Senderprogramm» | `listLiveChannelProgrammes` | ja |
| LS-01 | Einstiege «Meine Aufnahmen · 1 läuft», «Geplant · 5», «Erinnerungen · 2» | `getLiveHome.counts` | ja |
| LS-01 | Hinweisband «2 geplante Aufnahmen brauchen deine Aufmerksamkeit» | `getLiveHome.counts.warnings` | ja |
| LS-01 | «Nach Genre» → LS-05 `genre=…&when=now` | `searchLiveProgrammes` | ja |
| LS-02 | Herz (Favorit) | `addLiveFavorite` / `removeLiveFavorite` | ja |
| LS-03 | Reihenfolge der Favoriten | `setLiveFavoriteOrder` | ja |
| LS-04 | Taste E (Erinnern) | `createLiveReminder` | ja |
| LS-06 | «Serie aufnehmen» | `createLiveRecordingRule` | ja |
| LS-07 | Senderwechsel | `tuneLiveSession` | ja |
| LS-07 | Sofortaufnahme, «Bis Sendungsende verlängern» | `createLiveRecording` (instant), `updateLiveRecording.extendToProgrammeEnd` | ja |
| LS-08 | Filter Läuft / Fertig / Unvollständig / Fehlgeschlagen mit Zahlen | `listLiveRecordings` `state`, `counts` | ja |
| LS-08 | Sortierung «Läuft bald ab» | `sort=expiring` | ja |
| LS-08 | Serien als Stapel | `listLiveRecordings.series`, `ruleId` | ja |
| LS-08 | Mehrfachauswahl «Löschen» | `deleteLiveRecordings` | ja |
| LS-08 | Speicherleiste «Du / Übrige Profile / Frei» | `getLiveRecordingUsage` | ja |
| LS-08 | Karte «Aufnahmen meiner Profile · Luca, Mia» | `SessionInfo.live.profileRecordings` (nur ja/nein), Namen erst aus `listProfileLiveRecordings` | teilweise |
| LS-09 | «Weiter ab 35:12» | `progress.positionMs` | ja |
| LS-09 | Herkunft «Einzeln geplant am 20.11.» | kein `createdAt` | nein (RD-29) |
| LS-09 | Aufbewahrung ändern | `updateLiveRecording` `retentionDays`, `keep` | ja |
| LS-09 | «Aufnahme beenden» | `stopLiveRecording` | ja |
| LS-10 | «Ab Aufnahmebeginn (mit Vorlauf)» | Sprung im Player innerhalb der Liste | ja |
| LS-11 | Regeltermin «Abbrechen» | `deleteLiveRecording?skip=true` | ja |
| LS-11 | übersprungene Ausstrahlungen mit Grund «Wiederholung» | `LiveRule.skipped.reason` ohne Wert dafür | nein (W-10, bekannt) |
| LS-11 | Reiter «Wiederkehrend», Abschnitt «Manuell» | kein Filter `kind` | teilweise (RD-29) |
| LS-12 | «OK» (Hinweis ausblenden) | `updateLiveRecording.acknowledge` | ja |
| LS-12 | «Anderen Sender wählen» | kein `channelId` in der Änderung | nein (RD-29) |
| LS-12 | «Zeit trotzdem aufnehmen» | `createLiveRecording` (manual) | ja |
| LS-13 | «Ändern», «Entfernen» | `updateLiveReminder`, `deleteLiveReminder` | ja (siehe RD-20) |
| LS-14 | Abspielen und Löschen fremder Aufnahmen | `createLiveSession` (recordingId), `deleteLiveRecording` | ja (siehe RD-17, RD-18) |
| LS-15 | «Zeit aufnehmen» bei Sender ohne EPG | `createLiveRecording` (manual) | ja |
| LD-05 | Vorlauf der Erinnerung | `updateLiveReminder` | ja |
| LA-02 | «Jetzt aktualisieren» mit Fortschritt «2'300 von 9'874» | `refreshLiveSource`, `LiveSource.progress` | ja |
| LA-02 | «Entfernen» mit Wahl behalten/löschen und Zahlen | `deleteLiveSource?orphans=`, `getLiveSourceRemovalImpact` | ja |
| LA-02 | die letzten fünf Läufe | `listLiveSourceImports` | ja |
| LA-05 | Filter Zustand, Quelle, Gruppe, Land, Sprache, Kategorie, EPG, Empfang | `listAdminLiveChannels` | ja |
| LA-05 | «Nummern vergeben … in der aktuellen Sortierung» | `bulkUpdateLiveChannels` ohne `sort` | teilweise (RD-29) |
| LA-06 | «Zurück zum Namen der Quelle» | `updateLiveChannel.resetName` | ja |
| LA-06 | Quelle «Testen», «Nach oben», «Als eigenen Sender abtrennen» | `testLiveChannelStream`, `setLiveChannelStreams`, `splitLiveChannelStream` | ja |
| LA-06 | Logo «eigene Adresse» mit Vorschau | keine Vorschau | nein (RD-29) |
| LA-07 | Zusammenfassung «4 Favoriten und 1 geplante Aufnahme gehen über» | `previewLiveChannelMerge` | ja |
| LA-12 | «Neu verbinden», «Zur nächsten Quelle wechseln», «Beenden» | `reconnectLiveIngest`, `switchLiveIngestSource`, `stopLiveIngest` | ja |
| LA-16 | «Jetzt aufräumen», «zuletzt heute 03:00» | `cleanupLiveStorage`, `LiveStorage.orphans.lastCleanup` | ja |

Ergebnis: 36 ja, 3 teilweise, 4 nein. Operationen ohne Seite (Scope-Creep) fand die Stichprobe keine; alle 122
Operationen tragen eine Seitenkennung in 08 Abschnitt 2, und die Verdoppelungen stehen unter S-1 bis S-7.

---

## 4. Prüffragen ohne Änderungsbedarf (mit Rechnung)

- **Schreiblast durch Herzschlag:** Der Herzschlag selbst schreibt nichts (07 8.2). Je Sitzung höchstens einmal je
  Minute `last_seen_at` und `live_watch_time`: 20 Zuschauer ergeben 40 Schreibtransaktionen je Minute (0,7 je
  Sekunde), 50 Zuschauer 100 je Minute; dazu je laufender Aufnahme 6 je Minute. Das verträgt die eine
  Schreibverbindung neben dem EPG-Import (eigene Datei) ohne Weiteres. Gefährlich ist nicht die Zahl, sondern lange
  gehaltene Transaktionen (RD-01, RD-03, RD-04).
- **Polling:** 50 Zuschauer × ein Herzschlag je 10 Sekunden = 5 Anfragen je Sekunde, nur Arbeitsspeicher. Ohne ETag
  vertretbar (08 O-2).
- **Regel 2:** Keine Abfrage aus 07 oder 08 nennt `items`, `media_files` oder `tracks`; die einzige Berührung ist der
  Neubau von `notifications` (Fremdschlüssel in einer Migration, keine Abfrage). Der Riegel für `liveq` und `epgq`
  samt `TestLiveQueriesNoTitleTables` ist schlüssig.
- **`x-visibility` und `x-clients`:** plausibel. Bilder `scoped` mit `[app, tv]` wie Radio; Verwaltung `admin` ohne
  Liste; `getProfileLiveUsage` `account` mit `[app, tv]` wie `updateProfile` (`api/openapi.yaml:886-887`). Feste
  Pfadabschnitte neben Parametern (`/live/channels/facets`, `/live/recordings/usage`) hat der Bestand vielfach
  (`/admin/items/rating`, `/wishes/discover`).
- **Fingerprint `url_fp`:** HMAC mit abgeleitetem Schlüssel über die Adresse ohne Benutzer und Passwort. Bleibt ein
  Token im Pfad, steckt er im HMAC und lässt sich nicht zurückrechnen; kein Leck. `site` enthält nur Schema, Host,
  Port.
- **Zwei Dateien:** keine Fremdschlüssel zwischen `slouchcat.db` und `epg.db`; stimmig bleibt es über Textschlüssel
  `(source_id, xmltv_id)` und nie wiederverwendete Kennungen (`AUTOINCREMENT`). Lücke nur beim Aufräumen (RD-23).
- **Ausschlüsse (Auftrag 16):** keine Tabelle oder Spalte für Catch-up, Anbieterlimit (`allow_more_conns` ist der
  Schalter aus 04 T-18, kein Limit), Altersstufe an Sender oder Sendung (Kategorie `kids` ordnet, filtert nicht),
  geteilte Aufnahme, Benutzergruppen oder Aufnahmequalität. Ausnahme: unbeantwortete Optionen (RD-30).
- **Migrationen:** Der Neubau von `notifications`, `deliveries` und `my_media` folgt 00070, 00084 und 00059 und
  kopiert jede Zeile; `deliveries` muss `device_linked` aus 00084 mitnehmen (07 nennt beide Vorlagen). `epg.db`
  über `PRAGMA user_version` mit Neuaufbau ist für wiederbeschaffbare Daten angemessen.

---

## 5. Gesamturteil

**Überarbeiten vor Freigabe** (entspricht REQUEST CHANGES).

Das Datenmodell ist im Kern tragfähig: Trennung von `items`, eigene sqlc-Pakete mit Riegel, Grabsteine statt
Löschen von Sendern, Geheimnisse verschlüsselt, Zustandsübergänge als bedingte Updates, Bestandsmuster durchgehend
belegt. Gebaut wie beschrieben, hätte es aber acht Fehler mit echter Wirkung:

1. eine mögliche Verklemmung der einzigen Schreibverbindung beim Löschen von Profil oder Konto (RD-01);
2. still vertauschte Senderidentitäten bei neu nummerierten Listen, mit Aufnahmen am falschen Sender (RD-02);
3. einen nicht transaktionalen Import mit überschriebenen Sperren (RD-03);
4. minutenlange Vollscans beim Löschen einer Quelle wegen fehlender Indizes (RD-04);
5. Live-TV für Kinder in Konten mit Altersgrenze ohne Entscheid des Hauptprofils (RD-05);
6. zwei Lecks der Privatsphäre (Konto und Sender laufender Aufnahmen in der Verwaltung, RD-06; Meldung an das
   Hauptprofil, RD-07);
7. eine Sicherung ohne Geheimnisse, die an den eigenen CHECK-Regeln scheitert (RD-08).

Keiner davon verlangt einen Umbau. Alle lassen sich mit Indizes, Bedingungen, einer Definition und einigen Sätzen
korrigieren. Die API ist weitgehend vollständig (Stichprobe: 36 von 43 Aktionen ganz, 3 teilweise, 4 nicht abgedeckt) und folgt den Konventionen des
Vertrags; sie braucht Idempotenz beim Anlegen (RD-10), Aktionsfelder für Aufnahmen nach Regel 12 (RD-17) und eine
klare Client-Regel für 404 an `lt`-Routen (RD-16). Rund zehn Operationen lassen sich streichen (Abschnitt 2).
