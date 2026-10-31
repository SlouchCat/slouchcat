# Konzept: Hintergrundjobs – eigene Verwaltungsseite mit Start, Pause und Abbruch

- **Status: Fassung 4 (30. Oktober 2026), freigegeben mit dem Merge von PR #54 (`05df9c5`); in
  Umsetzung.** Schritt 0 (Entscheid 15, ADR 0017, Neuverankerung; Abschnitt 13.3), Schritt 1
  (Schema, Registry, Scheduler; 13.4), Schritt 2 (Fortschritt, Pause, Abbruch-Haken, Verlauf;
  13.5), Schritt 3 (Vertrag 1.5.0, Server-Handler; 13.6) und Schritt 4 (Seite; 13.7) liegen auf
  einem Zweig ab `main` `05df9c5` (PR #55). Der
  OMC-Kritiker hat Fassung 1 geprüft (0 Blocker,
  8 Major, 16 Minor; Abschnitt 13); Fassung 2 arbeitet alle ein. Marco hat am 30. Oktober 2026
  alle Fragen beantwortet (Abschnitt 11, 13.2); zwei Antworten weichen von der Empfehlung ab und
  sind eingearbeitet: **Zeitpläne einstellbar** (F-4: Takt, Zeitfenster, Ein/Aus) und **Verlauf
  30 Tage** (F-8). Ein klickbarer Prototyp der
  Seite liegt unter `docs/planning/mockups/09-hintergrundjobs-prototyp.html` (Abschnitt 7.1).
- **Datum: 29. Oktober 2026; Fassung 2, 3 und 4 vom 30. Oktober 2026** (Nachträge 13.1: Pausieren
  auch in «Läuft gerade»; 13.2: Marcos Antworten, F-4 und F-8 eingearbeitet; 13.3: Neuverankerung
  in Schritt 0; 13.4: Befunde aus Schritt 1).
- **Wogegen dieses Konzept verankert ist:** `main`, Stand **`05df9c5`** («Merge pull request #54»,
  Freigabe dieses Papiers; davor `65563bb`, Storybook-Baselines zu 08), Vertragsfassung **1.4.0**
  (`api/openapi.yaml:4`), letzte Migration **`00026_library_views.sql`**, letzte ADR
  **`0016-startseite-seitenleiste.md`**. Konzept **08 Startseite ist seit dem 30. Oktober 2026 in
  `main`** (PR #47, Merge `fa642f1`; **Frage 14, ADR 0016, Migration 00026, Vertrag 1.4.0**), dazu
  die Nachträge PR #48 bis #53. Dieses Papier **setzt 08 voraus** und nimmt **Frage 15, ADR 0017,
  Migration 00027 und Vertrag 1.5.0** (Entscheid 15 und ADR 0017 stehen seit Schritt 0). 08 bringt
  die Jobart `metadata.vote_backfill` mit, die hier mitgezählt wird (1.1, 4.3), und hat dieselben
  Navigations- und Sprachdateien angefasst (12.5). `:Zeile`-Angaben beziehen sich auf `05df9c5`
  (in Schritt 0 geprüft, 13.3); nach Namen suchen, nicht nach Zeile.
- Massgeblich bleiben `docs/spec/01-anforderungen-v1.md`, `docs/planning/04-entscheidungen.md` und
  `docs/adr/`. Bei Widerspruch gilt: **Entscheidungsprotokoll > Plan > Spec** (`CLAUDE.md`).
- **Jeder Schritt ist ein eigener Commit, und vor dem Abschluss jedes Schritts ist `task ci` grün**
  (`CLAUDE.md`, «Arbeitsweise»). DoD-Belege sind Befehl **und** Ausgabe.

> **Dieses Papier selbst ändert keine Zeile Code.** Die Arbeit hat mit Schritt 0 aus Abschnitt 10
> begonnen, einem reinen Doku-Commit; der Stand je Schritt steht in `docs/dev/CURRENT_STATUS.md`.

> **Abgrenzung.** Hier geht es um das **Sehen und Steuern** der Hintergrundarbeit: welche Jobs es
> gibt, was gerade läuft, wie lange schon, wie lange noch, wann der letzte Lauf fertig wurde, wann
> der nächste kommt, und die Knöpfe Start, Pause, Abbruch. **Nicht** hierher gehören: neue
> **fachliche** Jobarten (der nächtliche `rating.verify` aus Plan 3.2 bleibt draussen, 3),
> Kalender-Zeitpläne («jeden Montag um 3», cron; einstellbar sind Takt, Zeitfenster und Ein/Aus,
> F-4), eine einstellbare Arbeiterzahl in der Oberfläche (F-9), Erinnerungen an eine stehende
> Pause (Idee), Jobs je Zuschauer, Diagramme oder Auswertungen über den Verlauf. Die zwei
> Jobarten, die dieses Papier selbst einführt (`jobs.prune`, `podcast.poll`), sind **Haushalt der
> Queue**: Aufräumen und ein sichtbar gemachter Takt, kein neues Verhalten. Ein Zuschauer sieht
> von alledem nichts.

---

## 1. Ausgangslage und Problem

### 1.1 Was heute da ist

**Die Queue.** Slouchcat hat seit M0 eine eigene Job-Queue in SQLite (ADR 0002, Paket
`server/internal/jobs`): Tabelle `jobs` mit `type`, `priority` 0–9, `idempotency_key`, `payload`,
`state` (`queued`, `running`, `done`, `failed`, `cancelled`), `run_after`, Lease (`lease_owner`,
`lease_until`, 5 min, Heartbeat alle 60 s), `attempts`, `max_attempts`, `last_error`,
`created_at`, `updated_at`. Ein Job wird mit Lease beansprucht, der Handler muss idempotent sein,
bei Fehlern gibt es Backoff (10 s · 2^(Versuch − 1)), `Cancel` bricht wartende und laufende Jobs ab
(der Handler merkt es beim nächsten Heartbeat). **Was fehlt:** ein Startzeitpunkt je Lauf (nur
`updated_at`, das der Claim mitschreibt), ein Fortschritt je Lauf, eine Endzeit, ein Zustand
«pausiert», und jede Art von Verlauf: `done`-Zeilen bleiben liegen, es gibt **keinen** Aufräumjob
(ausserhalb der Tests löscht nichts aus `jobs`), die Tabelle wächst seit dem ersten Start.

**Die Jobarten.** Fünfzehn Typen sind registriert (`RegisterJobs` in fünf Paketen), eine
sechzehnte kommt mit Konzept 08:

| Jobart | Paket | Priorität | Wer löst aus | Dauer je Lauf |
|---|---|---|---|---|
| `scan.library` | `library` | 5 | Zeitplan alle 6 h, Start des Servers, «Scannen» auf der Bibliotheksseite, Webhook, Dateiwächter bei Wurzeländerung (`service.go:847`) | Minuten bis **Stunden** (erster Scan am NAS) |
| `scan.path` | `library` | 6 | Dateiwächter (Debounce 2 s), Webhook | Sekunden |
| `probe.file` | `library` | 3 | Scan je neue Datei, Pflege («Datei prüfen», `metadata/manage.go:193`) | Sekunden |
| `probe.keyframes` | `library` | 1, bei Wiedergabe 9 | Scan, vor der Wiedergabe | Sekunden bis Minuten; auf CT 190 standen 29 753 davon an |
| `library.sweep` | `library` | 2 | Zeitplan stündlich, Start des Servers | Sekunden (eine Transaktion, `sweep.go:29`) |
| `segments.detect` | `library` | 2 | Scan von Serienordnern (Intro/Abspann) | Minuten; auf CT 190 standen 1683 davon an |
| `metadata.match` | `metadata` | 4, Folgen 3, von Hand 7 | Scan, Zuordnung von Hand, Ordner ändern | Sekunden |
| `metadata.fill` | `metadata` | 2 | nach `metadata.match`, Listenänderung am Ordner | Sekunden |
| `image.fetch` | `metadata` | 2, Stills 1 | nach Match, Nachholen beim Start | Sekunden |
| `trailer.find` | `metadata` | 1 | nach Match | Sekunden |
| `podcast.fetch` | `metadata` | 4 | Poller alle 5 min (je Abo fällig nach 55 min), «Aktualisieren» je Podcast, neues Abo | Sekunden |
| `trakt.refresh` | `metadata` | 2, von Hand 7 | Poller alle 15 min (fällig nach `interval_hours`), «Jetzt abrufen» | Sekunden bis Minuten |
| `metadata.vote_backfill` | `metadata` | 2 | **Konzept 08**: `QueuePendingMetadata` nach jedem vollständigen Scan und einmal beim Start des Servers (`main.go`), solange Titel ohne Wertung warten; der Job reiht seinen Nachfolger mit Cursor selbst ein (`vote_backfill.go`). Ein **Ereignis**-Job, kein Hand-Job (13.3) | Minuten je Kette (Batches à 200) |
| `update.check` | `admin` | **8** (seit 30. Oktober 2026, PR #52; vorher 1) | Poller alle 15 min (fällig nach 24 h), «Jetzt prüfen» | Sekunden |
| `download.prepare` | `offline` | 5 | Download eines Profils; **Abgleich alle 10 min** (`offline.Run` → `Reconcile`, `cleanup.go:24`) reiht jeden unfertigen Download ohne aktiven Job neu ein | Minuten bis Stunden; ein Lauf, der auf Budget oder Quelle wartet, endet in Millisekunden `done` und reiht seinen Nachfolger ein (`job.go:103`) |
| `notify.deliver` | `notify` | 3 | jedes Ereignis mit Kanal (E-Mail, Web-Push, Telegram) | Sekunden |

Drei Arbeiter-Pools laufen (`server/cmd/slouchcat/main.go:317` bis `:336`): `cfg.Workers`
allgemeine Arbeiter (Vorgabe `max(2, min(4, NumCPU))`, `config.go:262`; per
`SLOUCHCAT_WORKERS` einstellbar, auf CT 190 sind es vier), ein Arbeiter `download-1` nur für
`download.prepare` und ein Arbeiter `notify-1` nur für `notify.deliver`. Die Zahl «Laufend» im
Dashboard zählt alle drei Pools zusammen. Ein Arbeiter nimmt immer den Job mit der höchsten
Priorität seines Pools, bei Gleichstand den ältesten (`ClaimJob`, `ORDER BY priority DESC`).

**Die Zeitpläne liegen nicht in der Queue.** Die Tabelle `job_schedules` (`00001_jobs.sql:32`,
`type`, `cron_expr`, `next_run`) ist seit M0 angelegt und **leer**; ADR 0002 vertagte den
Scheduler auf «den ersten periodischen Job». Stattdessen entstanden fünf eigene Schleifen mit
`time.NewTicker`:

| Schleife | Wo | Takt | Was sie tut |
|---|---|---|---|
| Scan aller Bibliotheken | `library.Service.Start`, `service.go:761` | 6 h, dazu sofort beim Start | reiht `scan.library` je Bibliothek ein |
| Sweep | ebenda | 1 h, sofort beim Start | reiht `library.sweep` ein |
| Podcast-Poller | `metadata.RunPodcasts`, `podcast.go:349` | prüft alle 5 min | reiht `podcast.fetch` für Abos ein, deren letzter Abruf 55 min alt ist (`QueueDuePodcasts`, `:365`) |
| Trakt-Poller | `metadata.RunTrakt`, `trakt.go:122` | prüft alle 15 min | reiht `trakt.refresh` ein, wenn der letzte Versuch `spotlight_config.interval_hours` alt ist; **ohne Client-Schlüssel leert er die Einträge** (`QueueDueTrakt`, `:142` bis `:149`). `spotlight_config.enabled` prüft er **nicht**: das Band wird bei `enabled = 0` nur ausgeblendet (`visibility/discover.go:277`), die Liste wird weiter geholt (13.3) |
| Update-Poller | `admin.RunUpdateCheck`, `update.go:89` | prüft alle 15 min, erstmals 60 s nach dem Start | reiht `update.check` ein, wenn die letzte Prüfung 24 h alt ist und die Prüfung eingeschaltet |

Der Kommentar in `service.go:30` sagt, warum: «two fixed intervals do not need a cron scheduler».
Inzwischen sind es fünf Schleifen in drei Paketen, jede mit eigener «fällig»-Logik, und **keine
weiss, wann der nächste Lauf ist** — die drei Poller rechnen es bei jedem Tick neu aus. Zwei
weitere Schleifen bleiben, wie sie sind: der Download-Abgleich `offline.Run` (alle 10 min; er
reiht `download.prepare` ein, ist aber kein Zeitplan, sondern eine Reparatur) und der
Telegram-Link-Poller (`telegram_link.go:108`, reiht nichts ein).

**Das Dashboard** (`getDashboard`, A-3; `web/src/features/admin/AdminHomePage.tsx:531`) zeigt die
Kachel aus Marcos Bild: drei Zahlen (`JobCounts.queued`, `running`, `failed`), darunter die
letzten zehn fehlgeschlagenen Jobs mit «Erneut» (`retryJob`) und «Abbrechen» (`cancelJob`),
alle 5 s abgefragt (`DASHBOARD_POLL_MS`), nur für Administratoren im Hauptprofil
(`AdminHomePage.tsx:336`). Mehr weiss die Oberfläche nicht: **nicht welche** vier Jobs laufen,
**nicht seit wann**, **nicht wie lange noch**, **nicht** was die 236 Wartenden sind. Einzig der
Bibliotheksscan hat einen Fortschritt (`libraries.scan_files_seen` / `scan_files_total`,
`LibraryScan.filesSeen` / `filesTotal`, gemeldet erst in der Schreibphase, `write.go:249`) und
einen eigenen Abbruch (`cancelLibraryScan`, `api/openapi.yaml:2540`), beides auf der
Bibliotheksseite.

**Abbruch und Fachlogik hängen zusammen.** `cancelJob` ist der rohe Queue-Abbruch; er kennt
die Fachzustände nicht. Ein laufender Scan, dessen Job so abgebrochen wird, sieht einen
gewöhnlichen `context.Canceled` und schreibt `last_scan_state = queued` (`scanner.go:330`),
obwohl kein Job mehr wartet; nur `CancelScan` (`service.go:379`) schreibt `aborted/cancelled`.
Ein so abgebrochener `download.prepare` setzt den Download auf `queued` (`job.go:513`), und der
Abgleich reiht ihn nach spätestens 10 min wieder ein. Das ist heute schon so, fällt aber erst
auf, wenn der Abbruch einen eigenen Knopf bekommt (6.3).

### 1.2 Was Marco will

Vom 29. Oktober 2026, am Dashboard-Bild (236 wartend, 4 laufend, 0 fehlgeschlagen):

1. **Ein eigenes Register** in der Verwaltung, nur für die Hintergrundjobs.
2. Dort sehen: **welche Jobs laufen, wie lange sie schon laufen, wie lange sie voraussichtlich
   noch laufen, wann der nächste Lauf ist, wann der letzte Lauf fertig wurde** «usw.».
3. **Jeden Job starten, pausieren und abbrechen** können.

### 1.3 Warum das heute nicht geht, in drei Sätzen

Die Queue kennt keinen Startzeitpunkt, keinen Fortschritt und keine Endzeit je Lauf, also lässt
sich weder «seit wann» noch «wie lange noch» beantworten. Die Zeitpläne stehen in fünf Schleifen
ausserhalb der Queue, also gibt es keinen Ort, der «nächster Lauf» kennt oder einen Lauf auf
Knopfdruck auslöst. Und «pausiert» gibt es als Zustand nirgends, weder je Lauf noch je Jobart.

---

## 2. Zielbild

### 2.1 Zwei Ebenen: Job und Lauf

Das Papier unterscheidet durchgehend zwei Dinge, und die Oberfläche benennt sie so (F-7):

- **Job** — eine **Jobart** wie `scan.library` oder `notify.deliver`: das, was man starten,
  pausieren und wovon man die Läufe abbrechen kann; achtzehn Stück (die fünfzehn von heute,
  `metadata.vote_backfill` aus 08, dazu `jobs.prune` und `podcast.poll`, 4.3 und 4.4). Ein Job
  hat einen Zustand (**läuft**, **wartet**, **ruht**, **pausiert**, **wird angehalten**), einen
  Auslöser (Zeitplan, Ereignis oder Hand), gegebenenfalls einen Zeitplan, einen letzten und einen
  nächsten Lauf.
- **Lauf** — eine Zeile in `jobs`: ein Scan der Bibliothek «Filme», eine Zustellung an Marcos
  Telegram, ein Match von Titel 4711. Ein Lauf hat Gegenstand, Priorität, Start, Dauer,
  Fortschritt, Restzeit, Ergebnis.

Die Knöpfe **Jetzt ausführen** und **Pausieren** gehören zum **Job**, der Knopf **Abbrechen**
zum **Lauf** und als Sammelknopf («Wartende abbrechen» oder «Alle Läufe abbrechen», F-2) zum
Job. Begründung in 6.3: ein einzelner `metadata.match` lebt zwei Sekunden, ihn zu pausieren ist
sinnlos; was Marco anhalten will, ist «die 236 Wartenden sollen jetzt nicht weiterlaufen», und
das ist eine Aussage über die Jobart. **Pausieren** steht darum an zwei Stellen, wirkt aber
immer gleich: auf der Karte des Jobs und in jeder Zeile unter «Läuft gerade» (Marcos Wunsch vom
30. Oktober 2026). Wer den laufenden Scan sieht und ihn anhalten will, muss nicht erst die Karte
suchen; der Knopf in der Zeile pausiert den Job, dessen Lauf das ist — der Lauf hält sofort an
(F-1), und bis zum Fortsetzen startet kein weiterer Lauf dieser Jobart (F-10).

### 2.2 Die Seite «Hintergrundjobs»

Neue Verwaltungsseite unter `/admin/jobs`, in der Navigation in der Gruppe **System** vor
«Integrationen», auf der Übersicht als Karte wie jede andere Seite (`adminSections`), mit
derselben Schranke wie das Dashboard (Administrator im Hauptprofil). Von oben nach unten:

1. **Kopf.** Titel «Hintergrundjobs», eine Zeile Zahlen als Chips: *Laufend 4 · Wartend 236 ·
   Geplant 12 · Pausiert 0 · Fehlgeschlagen 0*. «Geplant» sind Läufe, deren `run_after` in der
   Zukunft liegt (Backoff, Debounce, wartender Download) — sie «laufen nicht», und niemand soll
   sich fragen, warum. Darunter je Pool die Arbeiter und der Stau: «Allgemein: 4 Arbeiter, Stau
   ca. 3 h 10 min · Downloads: 1 Arbeiter, frei · Benachrichtigungen: 1 Arbeiter, frei» (6.2).
   Ist ein Job pausiert, steht hier ein Hinweis in Warnfarbe («2 Jobs pausiert — neue Dateien
   werden nicht erfasst»), denn eine vergessene Pause ist der gefährlichste Zustand dieses
   Konzepts (9.4).
2. **Läuft gerade.** Eine Liste der laufenden Läufe über alle Jobs: Jobname, **Gegenstand**
   (vom Server aus dem Payload abgeleitet, 5.3: «Filme», «The Expanse · S03E07», «Telegram an
   Marco»), die Fortschrittsnotiz, wo der Lauf eine meldet («Ordner Filme/2024 · 1 240 von 3 980»),
   **seit** (Dauer, laufend hochgezählt), ein Fortschrittsbalken, wo der Lauf einen meldet,
   **Restzeit** («noch ca. 12 min», «wird berechnet», «länger als üblich»), und je Zeile die
   Knöpfe **Pausieren** und **Abbrechen**. **Pausieren** in der Zeile ist derselbe Knopf wie auf
   der Karte (2.1, F-10): der Lauf hält sofort an und geht ohne verbrauchten Versuch zurück unter
   *Wartend*, die Zeile zeigt «wird angehalten», dann verschwindet sie, und der Job steht als
   «pausiert» im Kopf und auf seiner Karte; dort und im Hinweis des Kopfs sitzt **Fortsetzen**.
   Die Rückmeldung sagt, was geschah («Bibliotheksscan pausiert; der laufende Lauf hält an und
   geht in die Warteschlange zurück»), damit niemand meint, nur diese eine Zeile sei betroffen.
   **Abbrechen** beendet nur diesen Lauf (endgültig, 9.3). Leer: «Gerade läuft nichts.»
3. **Jobs.** Eine Karte je Jobart, in vier Gruppen (*Bibliothek*, *Metadaten*, *Zustellung*,
   *System*). Je Karte: Name auf Deutsch und der Code in Schreibmaschinenschrift
   («Bibliotheksscan · `scan.library`»), ein Zustandschip, dann vier Zeilen:
   - **Wartend** — Zahl und Schätzung («236 · dran in ca. 2 h 40 min»), kumuliert nach Priorität
     über den Pool (6.2); daneben «geplant 3», falls vorhanden;
   - **Letzter Lauf** — «vor 12 min · 3 s · OK» oder «vor 2 h · 48 min · Fehler: …»;
   - **Nächster Lauf** — «in 4 h 12 min · alle 6 h» bei Zeitplan, mit Zeitfenster «in 9 h 48 min ·
     täglich · 03:00–05:00», «aus» bei ausgeschaltetem Zeitplan, «bei Bedarf» bei Ereignis,
     «pausiert»;
   - **Ausgelöst durch** — eine Zeile Text aus der Sprachdatei («Zeitplan alle 6 h und Start des
     Servers», «vom Scan, je neue Datei», «jedes Ereignis mit Kanal»).

   Und die Knöpfe: **Jetzt ausführen** (nur bei Zeitplan- und Hand-Jobs, F-3), **Pausieren** /
   **Fortsetzen**, **Wartende abbrechen (236)** bzw. **Alle Läufe abbrechen** mit Bestätigung
   (F-2), bei Zeitplan-Jobs **Zeitplan …** (F-4): ein Dialog «Zeitplan von X» mit **Ein/Aus**,
   dem **Takt** als Auswahl (5 min, 15 min, 30 min, 1 h, 2 h, 6 h, 12 h, täglich, wöchentlich; die
   Grenzen kommen vom Server, 5.3) und einem **Zeitfenster** («nur zwischen 03:00 und 05:00»,
   über Mitternacht erlaubt, Zeitzone des Servers); «Speichern» schreibt den Zeitplan, die Karte
   zeigt danach den neuen nächsten Lauf. Bei `trakt.refresh` steht im Dialog, dass Takt und
   Ein/Aus dieselbe Einstellung wie auf der Seite Spotlight sind (6.4). Ein Klick auf «Läufe …»
   öffnet die **Läufe** des Jobs: Reiter *Laufend · Wartend ·
   Geplant · Fehlgeschlagen · Erledigt*, je Zeile Gegenstand, Priorität, Start, Dauer, Versuche,
   Fehler, **Erneut** bei fehlgeschlagenen, **Abbrechen** bei wartenden und laufenden. Der Reiter
   *Wartend* ist in **Claim-Reihenfolge** sortiert (Priorität, dann Alter), damit die erste
   Zeile die ist, die als nächste dran ist; die Gesamtzahl steht am Reiter.
4. **Fehlgeschlagen** bleibt auf dem Dashboard, wie heute (die letzten zehn mit Erneut und
   Abbrechen); die Kachel bekommt einen Link «Alle Jobs» hierher und, falls > 0, einen vierten
   Wert «Pausiert» in Warnfarbe.

Auf dem Handy stapeln sich Kopf, «Läuft gerade» und die Karten in einer Spalte; die Knöpfe einer
Karte stehen nebeneinander und brechen um. Alles wird wie das Dashboard alle 5 s abgefragt;
laufende Dauern zählt der Client zwischen zwei Abfragen selbst hoch (nur Anzeige, keine Regel,
Regel 12).

### 2.3 Was der Server neu kann

1. **Ein Scheduler** im Paket `jobs` löst die fünf Ticker-Schleifen ab und bedient
   `job_schedules`; er kennt zu jedem Zeitplan-Job den nächsten Lauf und führt ihn auf Knopfdruck
   sofort aus (4.3, 6.1). **Takt, Zeitfenster und Ein/Aus** je Zeitplan-Job sind auf der Seite
   einstellbar und liegen in der Tabelle, nicht mehr nur im Code (F-4).
2. **Fortschritt je Lauf**: ein Handler meldet `done`, `total` und eine Notiz; der Arbeiter schreibt
   sie gedrosselt in die Zeile (4.1, 6.2).
3. **Start- und Endzeit je Lauf** und ein **Verlauf** mit Aufräumjob, aus dem die Median-Dauer je
   Jobart kommt — die Grundlage jeder Schätzung (4.1, 4.4, 6.2).
4. **Pause je Jobart**, dauerhaft (überlebt Neustart, F-6): keine neuen Claims, laufende Läufe
   halten **sofort** an und gehen ohne verbrauchten Versuch in die Queue zurück (4.2, 6.3).
5. **Abbruch mit Fachlogik**: jede Jobart kann einen Abbruch-Haken registrieren, damit Scan und
   Download ihren Fachzustand richtig setzen (6.1, 6.3).
6. **Sieben neue Operationen** im Vertrag, alle `x-visibility: admin`, nicht brechend (5).

### 2.4 Was sich für den Zuschauer ändert

Nichts. Keine Operation mit `ItemRef`, keine Sichtbarkeitsregel, keine neue Seite ausserhalb
`/admin`. Eine Pause von `notify.deliver` verzögert Benachrichtigungen, eine Pause von
`download.prepare` lässt Downloads in «wird vorbereitet» stehen — das sind Folgen, die der
Administrator sieht und verantwortet (9.3).

---

## 3. Konflikt mit Spec, Plan und ADRs

| Stelle | Heute | Mit diesem Konzept |
|---|---|---|
| Spec A-3 («Dashboard zeigt … Hintergrundjobs») | Zahlen und fehlgeschlagene Jobs | **Erfüllt und erweitert**: Dashboard bleibt, eigene Seite kommt dazu. Keine Spec-Änderung |
| Plan 3.6 («Zeitpläne (cron-artig)») | Tabelle leer, fünf Ticker | **Scheduler kommt**, aber mit **Intervallen statt cron-Ausdrücken** (4.3): kein Job braucht «jeden Montag um 3»; Trakt rechnet schon in Stunden (`interval_hours`). ADR 0017 hält den Abstand zum Plan fest |
| ADR 0002, «Scheduler … gehört zum Meilenstein des ersten periodischen Jobs», «spätestens `rating.verify`» | nie eingelöst | **Eingelöst** durch ADR 0017 als Teilnachfolger (Scheduler, Pause, Fortschritt, Verlauf, Abbruch-Haken); Queue-Semantik (Lease, Idempotenz, Backoff, Priorität) bleibt wörtlich. `rating.verify` bleibt, was es seit M10 ist: ein Pflege-Befund `rating_mismatch` (`admin/maintenance.go:155`), **kein** Job — Regel 11; ihn zum nächtlichen Job zu machen wäre neues Verhalten und ist eine Idee, nicht dieses Papier |
| ADR 0002, Zustände `queued … cancelled` | fünf Zustände | **Unverändert.** «Pausiert» ist ein Merkmal der Jobart (`job_types.paused_at`), kein sechster Zustand; ein angehaltener Lauf ist wieder `queued` (6.3) |
| `library/service.go:30` «two fixed intervals do not need a cron scheduler» | Begründung für die Ticker | Hinfällig: die Intervalle wandern in `job_schedules`, der Kommentar fällt |
| Entscheid vom 26. Oktober 2026 (M11, `RaiseQueuedPriority`, manuelle Handlungen mit Priorität 7) | gilt für «Jetzt abrufen» und Match | **Gilt auch für «Jetzt ausführen»**: ein Hand-Lauf reiht mit 7 ein und **muss** einen wartenden Zeitplan-Lauf gleichen Schlüssels anheben; `enqueueScan` (`service.go:488`, feste `prioScan`) bekommt dafür einen Prioritätsparameter (6.1) |
| CLAUDE.md Regel 11 (nichts vorziehen) | — | Kein Nicht-Ziel berührt; keine neue fachliche Jobart (Abgrenzung) |
| CLAUDE.md Regel 12 (Entscheid im Server) | — | Schätzungen (Restzeit, Wartezeit, Stau) und der Gegenstand eines Laufs kommen vom **Server**; der Client zählt nur Sekunden hoch |

---

## 4. Datenmodell

### 4.1 `jobs` lernt Start, Ende und Fortschritt

Sechs Spalten, alle NULL-fähig oder mit Vorgabe, keine bestehende Zeile ändert sich:

| Spalte | Typ | Gesetzt von | Wozu |
|---|---|---|---|
| `started_at` | INTEGER (ms) | `ClaimJob` (jeder Versuch neu) | «läuft seit», Dauer |
| `finished_at` | INTEGER (ms) | `CompleteJob`, `FailJob` (Endzustand), `CancelJob`, **`FailExhaustedLeases`** | Dauer, «letzter Lauf fertig vor», Median |
| `progress_done` | INTEGER | Handler über `jobs.Progress` | Balken, Restzeit |
| `progress_total` | INTEGER | ebenso | Balken, Restzeit |
| `progress_note` | TEXT (≤ 200 Zeichen) | ebenso | «woran gerade» («Ordner Filme/2024 · 1 240 von 3 980») |
| `skipped` | INTEGER 0/1, Vorgabe 0 | Handler über `jobs.Skip(ctx)` | ein Lauf, der **nichts zu tun hatte** (Download wartet auf Budget, Scan ohne Ordner, Update-Prüfung ausgeschaltet), zählt nicht zum Median (6.2) |

`ReleaseJob` (Herunterfahren, Pause) löscht `started_at` und den Fortschritt nicht — der nächste
Versuch überschreibt sie beim Claim; der Verlauf zeigt so den letzten Stand. `RetryFailedJob`
setzt `started_at`, `finished_at`, Fortschritt und `skipped` zurück, wie es `attempts` schon
zurücksetzt. Die Dauer eines Laufs ist `finished_at − started_at` des **letzten** Versuchs, nicht
die Summe aller Versuche: Backoff-Wartezeit ist keine Arbeit. Ein neuer Index
`jobs_history (type, state, finished_at DESC)` trägt die Median-Abfrage und das Aufräumen;
`jobs_claim` bleibt.

### 4.2 Neue Tabelle `job_types`

```sql
CREATE TABLE job_types (
    type       TEXT    PRIMARY KEY,
    paused_at  INTEGER,            -- NULL = nicht pausiert
    updated_at INTEGER NOT NULL
);
```

Eine Zeile je registrierter Jobart, angelegt beim Start durch die Registry (6.1); eine Zeile für
einen Typ, den kein Arbeiter mehr kennt, bleibt stehen und wird in der Oberfläche als «unbekannt»
gezeigt (das ist der Fall nach einem Downgrade). Die Pause lebt hier und nur hier. Alles andere
über eine Jobart (Gruppe, Auslöser, Pool, ob «Jetzt ausführen» geht) ist Code und steht in der
Registry, nicht in der Datenbank — es ändert sich nur mit einer neuen Version.

### 4.3 `job_schedules` wird neu gebaut

Die Tabelle ist leer, darum **DROP und CREATE** statt Umbau (Regel 5 bleibt gewahrt: vorwärts,
nach Vorlage, mit `foreign_key_check`):

```sql
CREATE TABLE job_schedules (
    type           TEXT    PRIMARY KEY REFERENCES job_types(type),
    every_ms       INTEGER NOT NULL CHECK (every_ms BETWEEN 300000 AND 604800000), -- 5 min .. 7 d
    start_delay_ms INTEGER,          -- NULL = beim Start nicht feuern; 0 = sofort; 60000 = nach 60 s
    enabled        INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    window_start   INTEGER,          -- Minuten seit Mitternacht, Zeitzone des Servers (TZ); NULL = kein Fenster
    window_end     INTEGER,          -- exklusiv; kleiner als window_start = über Mitternacht
    next_run_at    INTEGER NOT NULL,
    last_fired_at  INTEGER,
    updated_at     INTEGER NOT NULL,
    CHECK ((window_start IS NULL) = (window_end IS NULL)),
    CHECK (window_start IS NULL OR (window_start BETWEEN 0 AND 1439 AND window_end BETWEEN 0 AND 1439
           AND window_start <> window_end))
);
```

`cron_expr` fällt weg (3). Die Zeilen schreibt **nicht** die Migration, sondern die Registry beim
Start (`EnsureSchedule`), und zwar **nur, wenn sie fehlen**: Intervall und `start_delay_ms` aus
dem Code (`fullScanInterval`, `sweepInterval`, …) sind seit F-4 **Vorgaben für die erste Zeile**,
danach ist die Tabelle die Quelle, und `updateJobSchedule` (5.2) schreibt Takt, Zeitfenster und
Ein/Aus. Zwei Jobarten haben ihre Einstellung schon an anderer Stelle: `trakt.refresh`
(`spotlight_config.interval_hours`, Seite Spotlight) und `update.check`
(`update_check_enabled`, Seite Update). Dort gilt **ein Wert, zwei Türen**: `updateJobSchedule`
schreibt für diese zwei in die bestehende Einstellung (mit deren Prüfung, Trakt 6 bis 48 h), und
die bestehende Seite schreibt wie heute, beide über `UpdateSchedule` in die Zeile (6.1); das
Zeitfenster liegt für alle nur in `job_schedules`, und das Ein/Aus von `trakt.refresh` ebenfalls
(`spotlight_config.enabled` blendet nur das Band aus, 1.1, 13.3). Beim Start gilt: ist `start_delay_ms`
gesetzt, feuert der Zeitplan nach dieser Verzögerung (das ersetzt «sofort beim Start» und «60 s
nach dem Start»); sonst bleibt `next_run_at` erhalten, wenn die Zeile schon da ist, damit ein
Neustart den Takt nicht verschiebt. Legt `EnsureSchedule` eine Zeile **neu** an (erster Start
nach der Migration), leitet es `next_run_at` aus der bisherigen Historie ab, damit niemand einen
Tag wartet oder doppelt läuft: Update-Prüfung aus `update_check.checked_at + 24 h`, Trakt aus
`spotlight_trakt.last_attempt + interval_hours`, beides mit Untergrenze «jetzt».

Die sieben Zeilen nach Schritt 1 und 2:

| `type` | `every_ms` | `start_delay_ms` | `enabled` | Herkunft, Verhalten |
|---|---|---|---|---|
| `scan.library` | 6 h | 0 | 1 | `fullScanInterval`; `Run` reiht einen Scan je Bibliothek ein |
| `library.sweep` | 1 h | 0 | 1 | `sweepInterval` |
| `podcast.poll` | 5 min | 0 | 1 | **neu, sichtbar gemachter Takt**: `Run` ist `QueueDuePodcasts` (reiht `podcast.fetch` für Abos ein, deren Abruf 55 min alt ist). `podcast.fetch` selbst ist eine **Ereignis**-Jobart; «Jetzt ausführen» an `podcast.poll` reiht **alle** Abos ein, nicht nur die fälligen (sonst täte der Knopf meist nichts) |
| `trakt.refresh` | `interval_hours` | NULL | 1 (**nicht** an `spotlight_config.enabled` gebunden, 13.3) | der Zeitplan **ist** die Fälligkeit; feuert beim Start nur, wenn `next_run_at` schon vorbei ist. Der Zweig «kein Client-Schlüssel → Einträge leeren» (`trakt.go:147`) bleibt **im `Run`**, nicht im Handler |
| `update.check` | 24 h | 60 000 | `update_check.enabled` | `UpdateInterval`; 60 s nach dem Start wie heute, aber nur, wenn fällig (`next_run_at` aus `checked_at`) |
| `jobs.prune` | 24 h | NULL | 1 | neu, 4.4; kommt mit Schritt 2 |
| `metadata.vote_backfill` | — | — | — | **kein** Zeitplan und **kein** `Run`: eine **Ereignis**-Jobart (`Trigger = event`), denn 08 stösst ihn nach jedem vollständigen Scan und beim Start selbst an (`QueuePendingMetadata` → `QueueVoteBackfill`, 1.1); ein Hand-Knopf wäre doppelt, und F-3 gilt (13.3) |

Podcast-Fälligkeit je Abo (55 min) bleibt im `Run` von `podcast.poll`, weil sie je Abo gilt,
nicht je Jobart.

### 4.4 Verlauf und Aufräumen: `jobs.prune`

Heute bleibt jede `done`-Zeile für immer. Für die Median-Dauer braucht der Server einen Verlauf,
aber einen **begrenzten**: `jobs.prune` läuft täglich und löscht `done`- und `cancelled`-Läufe,
die älter als **30 Tage** sind, behält aber je Jobart die **neuesten 50** (F-8). `failed` bleibt
wie heute liegen, bis der Administrator «Erneut» oder «Abbrechen» drückt — ein Fehler, den
niemand gesehen hat, darf nicht von selbst verschwinden. `jobs.prune` ist selbst ein Job der
Gruppe *System* und erscheint auf der Seite; sein erster Lauf nach der Migration räumt den Bestand
auf (auf CT 190 nach Wochen Betrieb mit 29 753 Keyframe-Läufen und 1683 Segment-Läufen:
Zehntausende Zeilen; Batches à 500, Regel aus der Paketdoku).

### 4.5 Migrationsplan 00027 (Schritte, kein fertiges SQL)

Nach `server/migrations/TEMPLATE.sql.txt`: `-- +goose NO TRANSACTION`, `PRAGMA
foreign_keys=OFF`, dann **`BEGIN;`**, die Schritte, **`COMMIT;`**, dann `PRAGMA
foreign_key_check` und `PRAGMA foreign_keys=ON`:

1. `ALTER TABLE jobs ADD COLUMN` × 6 (4.1); `CREATE INDEX jobs_history`.
2. `CREATE TABLE job_types` (4.2), leer; die Registry füllt sie beim Start, **vor**
   `EnsureSchedule`, so dass der Fremdschlüssel von `job_schedules` immer eine Zeile findet.
3. Wächter wie in 00023 (`00023_library_roots_providers.sql:120`): eine temporäre Tabelle mit
   `CHECK (n = 0)`, in die `SELECT count(*) FROM job_schedules` eingefügt wird — ist die Tabelle
   wider Erwarten nicht leer, bricht die Migration vor dem Löschen ab. Dann `DROP TABLE
   job_schedules; CREATE TABLE job_schedules` (4.3).

Danach: `testdata/db/` bekommt einen eingefrorenen Stand nach 00027 (8.3). Der Server sichert
vorher wie immer mit `VACUUM INTO` (Regel 5).

---

## 5. Auswirkung auf den Vertrag (1.5.0, nicht brechend)

### 5.1 Zwei Ressourcen statt einer

Heute gibt es `/admin/jobs/{id}` (Lauf, DELETE) und `/admin/jobs/{id}/retry`; beide bleiben
unverändert. Die Jobart bekommt eine eigene Ressource `/admin/job-types/{type}`, damit `{id}`
(Zahl) und `{type}` (Text) sich nicht denselben Pfad teilen — OpenAPI erlaubt zwei Vorlagen
`/admin/jobs/{id}` und `/admin/jobs/{type}` nicht nebeneinander.

### 5.2 Operationen

| Operation | Pfad | Antwort | Wozu |
|---|---|---|---|
| `listJobTypes` | `GET /admin/job-types` | `JobOverview` | die Seite: Pools mit Arbeitern und Stau, Zahlen, eine `JobTypeInfo` je Jobart samt laufenden Läufen und letztem Lauf — **eine** Abfrage alle 5 s wie `getDashboard` |
| `listJobRuns` | `GET /admin/jobs?type=&state=&limit=` | `JobRunPage` (`total`, `runs[]`) | die Läufe einer Jobart nach Zustand, höchstens 200; `queued` und `planned` in Claim-Reihenfolge, alle anderen neueste zuerst; `state=planned` ist `queued` mit `run_after > now` |
| `runJobType` | `POST /admin/job-types/{type}/run` | 202 `JobRun[]` | «Jetzt ausführen»: ruft die `Run`-Funktion der Registry mit Priorität 7 und gibt die eingereihten Läufe zurück (ein Scan je Bibliothek). 409 `job_type_not_runnable` bei Ereignis-Jobs, 409 `job_type_paused` bei pausierten; **Fehler der Dienste gehen durch**: 422 `provider_not_configured` (Trakt ohne Schlüssel), 422 `update_check_disabled`, 429 `refresh_too_soon` (Trakt, einmal pro Minute) — dieselben Codes wie an den heutigen Knöpfen |
| `pauseJobType` | `POST /admin/job-types/{type}/pause` | 200 `JobTypeInfo` | setzt `paused_at`, hält laufende Läufe an; idempotent |
| `resumeJobType` | `POST /admin/job-types/{type}/resume` | 200 `JobTypeInfo` | löscht `paused_at`; idempotent |
| `updateJobSchedule` | `PATCH /admin/job-types/{type}/schedule` | 200 `JobTypeInfo` | «Zeitplan …» (F-4): Körper `JobScheduleUpdate { everySeconds?, window?: { startMinute, endMinute } \| null, enabled? }`, nur gesetzte Felder ändern sich; prüft die Grenzen aus `schedule.limits` (sonst 422 `validation_error`), bei `trakt.refresh` und `update.check` schreibt er in deren bestehende Einstellung (4.3); 409 `job_type_not_scheduled` bei Ereignis- und Hand-Jobs; richtet `next_run_at` neu aus (6.1) |
| `cancelJobRuns` | `DELETE /admin/job-types/{type}/runs?scope=queued\|all` | 200 `{ cancelled: n }` | «Wartende abbrechen» (`queued`, Vorgabe) oder «Alle Läufe abbrechen» (`all`: wartende und laufende, F-2); läuft über den Abbruch-Haken der Jobart (6.3) |
| `retryJob`, `cancelJob` | wie heute | wie heute | Lauf-Ebene; `cancelJob` läuft neu ebenfalls über den Abbruch-Haken |

Alle neu: `x-visibility: admin`, Tag `admin`, 401/403 wie die Nachbarn, 404 `not_found` bei
unbekannter Jobart; jede Operation bekommt ihren Eintrag im Vertragstest
(`TestContractCoversEveryOperation`, Regel 7). Keine Antwort enthält einen `ItemRef`: ein Lauf
nennt einen Titel nur als Text im Gegenstand und als Id, wie `JobSummary.lastError` es heute hält.

### 5.3 Schemata

- `JobRun` **erweitert `JobSummary`** (gleiche Pflichtfelder, dazu `priority`, optional
  `runAfter`, `subject`, `startedAt`, `finishedAt`, `durationSeconds`, `skipped`,
  `progress { done, total, note }`, `estimatedSecondsLeft`, `estimate` = `measured | median |
  none`). `JobSummary` selbst bleibt, damit `getDashboard` unverändert gilt. **`subject`** leitet
  das Paket `admin` aus dem Payload ab, je Jobart eine kleine Funktion (Bibliotheksname aus
  `libraryId`, Titel mit Folgenmarke aus `itemId`, Kanal und Konto aus `deliveryId`, Podcastname
  aus dem Abo), redigiert durch `logging.RedactString` wie `lastError`; wo keine Ableitung
  gelingt, fehlt das Feld und der Client zeigt die Id.
- `JobTypeInfo`: `type`, `group` (`library | metadata | delivery | system`), `trigger`
  (`schedule | event | manual`), `pool` (`general | download | notify`), `paused`, `pausedAt?`,
  `stopping` (Läufe halten gerade an), `runnable` (bool), `schedule? { everySeconds, nextRunAt,
  lastFiredAt, enabled, window?: { startMinute, endMinute }, timeZone, limits: { minEverySeconds,
  maxEverySeconds } }` (`timeZone` ist die des Servers, z. B. `Europe/Zurich`, damit der Dialog
  sagt, in welcher Zeit das Fenster gilt; `limits` sind je Jobart, Trakt 6 bis 48 h, sonst 5 min
  bis 7 d — der Client baut seine Auswahl daraus, Regel 12), `counts { queued, planned, running, failed }`, `running: JobRun[]`
  (≤ 10), `lastRun?: JobRun`, `medianDurationSeconds?`, `queuedEstimateSeconds?`, `known`
  (false für eine Zeile in `job_types` ohne registrierte Jobart).
- `JobOverview`: `pools: [{ pool, workers, backlogSeconds? }]`, `paused` (Zahl), `types:
  JobTypeInfo[]`.
- `JobRunPage`: `total`, `runs: JobRun[]`.
- `JobCounts` (Dashboard) bekommt optional `paused` (Zahl) — additiv.
- Problemcodes neu: `job_type_not_runnable`, `job_type_paused`, `job_type_not_scheduled`; die bestehenden
  `provider_not_configured`, `update_check_disabled`, `refresh_too_soon` werden an `runJobType`
  wiederverwendet.

Texte der Jobarten (Name, Auslöser-Zeile) kommen **nicht** aus dem Vertrag, sondern aus
`web/src/locales/{de,en}.json` (Regel 9, Schlüssel in 7); eine Jobart ohne Schlüssel zeigt ihren
Code. Das Schema erzwingt kein Enum über `type`, damit eine neue Jobart keinen Vertragsbruch
auslöst.

### 5.4 Ist das eine brechende Anhebung?

Nein. Nur neue Operationen, neue optionale Felder, ein neues Schema als Obermenge. **1.5.0**. Der
Eintrag für die Release-Notes (C-5): «Neue Verwaltungsseite Hintergrundjobs; Vertrag 1.5.0 fügt
`/admin/job-types` hinzu, nichts entfällt.»

---

## 6. Server

### 6.1 Registry und Scheduler im Paket `jobs`

Heute registriert jeder Dienst seine Handler mit `w.Handle(typ, fn)`. Neu registriert er
**die Jobart** mit ihren Eigenschaften, und der Handler ist eines davon:

```go
// jobs.Type describes one job type for the worker, the scheduler and the admin page.
type Type struct {
	Name    string        // "scan.library"
	Group   Group         // library, metadata, delivery, system
	Pool    Pool          // general, download, notify
	Trigger Trigger       // schedule, event, manual
	Handler Handler       // runs one job (as today)
	// Run enqueues what one run of this type means (every library, every due feed) with prio
	// and returns the job ids. It must raise a waiting job that holds the same idempotency key
	// to prio (RaiseQueuedPriority, decision of 26 October 2026). nil for event types.
	Run func(ctx context.Context, q *Queue, prio int) ([]int64, error)
	// Cancel ends the domain state of job j before the queue cancels it (a scan ends aborted,
	// a download ends failed so that Reconcile does not queue it again). nil: queue only.
	Cancel func(ctx context.Context, j Job) error
	Every      time.Duration // > 0 for schedule types
	StartDelay *time.Duration // nil: do not fire at start; 0: at once; 60 s: update.check
}
```

`Registry.Register(t Type)` legt die Zeile in `job_types` an, bei `Every > 0` den Zeitplan
(4.3), und übergibt den Handler an den Arbeiter des Pools. `RegisterJobs(w)` in den fünf Paketen
wird zu `RegisterTypes(r *jobs.Registry)`; der Umbau ist mechanisch, die Handler bleiben.
`enqueueScan` (`service.go:488`) und seine Geschwister bekommen die Priorität als Parameter und
heben über `RaiseQueuedPriority` an — ein Test je Jobart mit `Run` prüft genau das.

Der **Scheduler** ist eine Goroutine in `jobs` (eine je Server, wie die Arbeiter in `main.go`),
Tick alle 60 s: `SELECT … FROM job_schedules WHERE enabled = 1 AND next_run_at <= ?`, je Treffer
`Run(ctx, q, prio)` mit der normalen Priorität der Jobart, dann `next_run_at = now + every_ms`,
`last_fired_at = now`. Drei Regeln, je mit Test:

- **Pausierte Jobart:** der Scheduler ruft `Run` **nicht**, schreibt aber `next_run_at` fort
  (die Pause verschiebt den Takt nicht, sie lässt Läufe aus); «Nächster Lauf» zeigt «pausiert».
- **`UpdateSchedule(type, every, window, enabled)`** setzt `next_run_at = last_fired_at + every`,
  ohne `last_fired_at` auf `now + every`, in beiden Fällen mit Untergrenze «jetzt», danach
  Ausrichtung am Fenster (nächste Regel). Wer den Spotlight-Takt von 24 h auf 6 h senkt, bekommt
  den nächsten Lauf also früher, nicht erst nach dem alten. Der Scheduler liest die Zeile bei
  jedem Tick, darum wirkt eine Änderung spätestens nach 60 s.
- **Zeitfenster** (F-4): nach jedem Feuern und nach jedem `UpdateSchedule` wird `next_run_at`
  **am Fenster ausgerichtet**: liegt es ausserhalb (`window_start`, `window_end` als Minuten des
  Tages in `time.Local`, also der `TZ` des Containers; `end < start` heisst über Mitternacht),
  rückt es auf den nächsten Fensterbeginn. Ein Takt kürzer als das Fenster feuert innerhalb des
  Fensters im Takt (1 h in 02:00–05:00: dreimal), ein längerer einmal je Fenster. «Jetzt
  ausführen» ignoriert das Fenster, die Pause bleibt davon unberührt. Sommerzeitwechsel
  verschieben das Fenster um eine Stunde; das ist hingenommen.
- **Zeile ohne registrierte Jobart** (`Run == nil`, nach einem Downgrade): bleibt stehen, wird
  übersprungen, erscheint als «unbekannt» mit `known = false`; `jobs.prune` löscht sie nicht.

Verpasste Läufe (Server war aus) holen **nicht** nach: ein Takt, nicht ein Kalender — genau das
Verhalten der heutigen Poller. Die fünf Ticker-Schleifen (1.1) fallen; `library.Start` behält nur
die Dateiwächter. Tests mit injizierter Uhr wie `queue_test.go` heute: «feuert nach
`StartDelay`», «feuert nach `every`», «feuert nicht bei `enabled = 0`», «ändert `next_run_at`
nicht bei `runJobType`» (F-5), «Fenster über Mitternacht», «Fensteränderung verschiebt
`next_run_at`», «Takt kürzer als Fenster feuert mehrfach», plus die Regeln oben.

### 6.2 Fortschritt und Schätzung

**Melden.** Der Arbeiter legt einen Reporter in den Kontext des Handlers; ein Handler ruft
`jobs.Progress(ctx, done, total, note)` so oft er will (`total = 0` heisst «Umfang unbekannt»,
der Balken läuft dann unbestimmt, die Notiz zählt trotzdem). Der Reporter schreibt höchstens
**alle 2 s** und zusätzlich mit jedem Heartbeat in die Zeile (`UpdateJobProgress`, nur unter
eigener Lease), die Notiz durch `logging.RedactString`. Vier Handler melden in Schritt 2:

| Handler | Was er meldet | Woher |
|---|---|---|
| `scan.library` | **Durchsuchphase**: «Ordner durchsucht: 312», `total = 0` (die Phase, die am NAS Stunden dauert, stünde sonst auf «wird berechnet»); **Schreibphase**: `seen`/`total` wie heute (`write.go:249`), Notiz Bibliothek und laufender Ordner relativ zur Wurzel | hat er schon |
| `segments.detect` | je Folge einer Serie, `done`/`total` über die Folgen des Laufs | Handler zählt |
| `download.prepare` | **Segmente** (`SegmentsDone`/`SegmentsTotal`, `job.go:478`), nicht Bytes (die sind nur geschätzt, `EstimatedBytes`) | hat er schon |
| `probe.keyframes` | keine Zahl, aber die Notiz (Datei relativ zur Wurzel) | Payload |

`library.sweep` meldet nichts: er ist eine Transaktion mit vier Anweisungen (`sweep.go:29`),
dauert Sekunden. Alle anderen bleiben ohne Fortschritt — ihre Läufe dauern Sekunden, da wäre ein
Balken Theater. **`jobs.Skip(ctx)`** setzt `skipped = 1` für einen Lauf, der nichts zu tun hatte;
drei Handler rufen es: `download.prepare` beim Warten auf Budget, Quelle oder Vorrang
(`job.go:103`), `scan.library` ohne Ordner, `update.check` bei ausgeschalteter Prüfung.

**Restzeit je Lauf** (`estimatedSecondsLeft`, vom Server gerechnet):

| Fall | Rechnung | `estimate` |
|---|---|---|
| Fortschritt gemeldet mit `total > 0`, `done ≥ 5 %` von `total` **und** Dauer ≥ 30 s | `elapsed · (total − done) / done` | `measured` |
| kein messbarer Fortschritt, Median der Jobart bekannt (≥ 5 nicht übersprungene erledigte Läufe im Verlauf) | `max(0, median − elapsed)`; ist `elapsed > 2 · median`: `null` mit `estimate = median` (Client: «länger als üblich») | `median` |
| sonst | `null` | `none` (Client: «wird berechnet») |

Der Median kommt aus `finished_at − started_at` der neuesten 50 `done`-Läufe der Jobart mit
`skipped = 0` (4.1, 4.4); Median statt Mittelwert, weil ein erster NAS-Scan von sechs Stunden
sonst jeden Zwei-Minuten-Scan auf Jahre verschätzt, und ohne die übersprungenen, weil sonst der
Download-Median bei Millisekunden läge.

**Wartezeit je Jobart** (`queuedEstimateSeconds`, «dran in ca. …»): der Pool ist über alle
Jobarten geteilt und läuft nach Priorität, darum **kumuliert nach Priorität**: Summe von
`queued · median` über alle Jobarten desselben Pools mit **gleicher oder höherer** Priorität
(die niedrigste Priorität der wartenden Läufe dieser Jobart zählt), geteilt durch die
Arbeiterzahl des Pools. Für `probe.keyframes` (Priorität 1 hinter 1683 Segmenten mit 2) ist das
die ehrliche Zahl; die naive Rechnung `queued · median / workers` wäre für ihn systematisch zu
klein. **Stau je Pool** (`backlogSeconds`, im Kopf): dieselbe Summe über alle Jobarten des Pools.
Ohne Median zählt eine Jobart mit 0 und der Client schreibt «ohne Schätzung» dazu. Beides ist
eine Schätzung und wird so benannt («ca.»); der Client rundet auf Minuten, ab 90 min auf
Stunden, ab einem Tag «über 1 Tag».

**Was die Schätzung nicht kann:** sie sieht den Stau eines Netzlaufwerks nicht (CT 190, 45 MB/s,
`slouchcat-ct190-stand`). Darum «länger als üblich» statt einer falschen Zahl, und darum zählt
der Client die laufende Dauer immer mit — sie ist das einzige, was sicher stimmt.

### 6.3 Pause, Start, Abbruch

**Pause** (F-1). `pauseJobType` setzt `job_types.paused_at`. Wirkung in zwei Schritten:

1. **Keine neuen Claims.** `ClaimJob` bekommt `AND type NOT IN (SELECT type FROM job_types WHERE
   paused_at IS NOT NULL)`; der Index `jobs_claim` trägt das weiter, die Unterabfrage liest
   höchstens achtzehn Zeilen.
2. **Laufende Läufe halten sofort an.** Server und Arbeiter laufen im selben Prozess; jeder
   Arbeiter trägt seinen laufenden Lauf in eine Liste im Speicher der Registry ein (Vorbild
   `library.Service.runs`). `pauseJobType` bricht den Kontext jedes laufenden Laufs der Jobart
   mit `context.WithCancelCause(…, jobs.ErrPaused)` ab — in unter einer Sekunde, nicht erst beim
   Heartbeat. Als Rückfall (ein Lauf, der nach einem Neustart unter alter Lease weiterläuft)
   prüft der Heartbeat den Pausenstand zusätzlich. Bis der Handler zurückgekehrt ist, meldet die
   Jobart `stopping = true` («wird angehalten»).

Der Knopf **Pausieren** in der Zeile unter «Läuft gerade» ruft dieselbe Operation
`pauseJobType` mit der Jobart des Laufs; es gibt **keinen** Endpunkt «diesen einen Lauf
pausieren» und keinen Zustand `held` je Lauf (F-10). Zwei Läufe derselben Jobart (zwei
`segments.detect`) halten darum zusammen an — die Oberfläche sagt das in der Rückmeldung.

**Die Lücke liegt im Arbeiter, nicht in den Handlern.** `worker.run` entscheidet heute mit
`case err != nil && ctx.Err() != nil` (`worker.go:121`) über `Release`, und `ctx` ist der
Eltern-Kontext — nur beim Herunterfahren. Ein Abbruch von `jobCtx` liefe darum in `Fail`:
Versuch verbraucht, Backoff, nach fünf Pausen `failed` und eine Meldung über
`notify.JobFailed`; gibt der Handler nach dem Abbruch `nil` zurück, würde er gar als `done`
abgeschlossen. Darum prüft `run` **vor** dem `switch`: ist `context.Cause(jobCtx) ==
ErrPaused`, folgt **immer** `Release`, gleichgültig, was der Handler zurückgab. Test: «fünfmal
pausieren und fortsetzen → `attempts` unverändert, kein `JobFailed`, Lauf wieder `queued`».
Die Handler selbst brauchen keine Prüfung der Ursache: der Scan schreibt bei einem
Kontextabbruch schon heute `queued` (`scanner.go:330`), der Download ebenfalls (`job.go:513`);
beim **Fortsetzen** nimmt der nächste Claim den Lauf wieder auf, der Scan beginnt den Ordnerlauf
von vorn und überspringt Erledigtes (idempotent, `library/doc.go:20`), der Download setzt an
seinen gesicherten Segmenten fort. Pause und Fortsetzen werden protokolliert (`slog`, Info, mit
Jobart; keine Nutzdaten).

**Start** («Jetzt ausführen»). Nur Jobarten mit `Run ≠ nil` (Zeitplan- und Hand-Jobs, F-3);
der Server ruft `Run` mit **Priorität 7** (Entscheid vom 26. Oktober 2026) und gibt die Läufe
zurück. Ein Hand-Lauf verschiebt `next_run_at` **nicht** (F-5): der Takt bleibt, wie er war.
Ist die Jobart pausiert, antwortet der Server 409 `job_type_paused`; der Client bietet den Knopf
dann gar nicht an (Regel 12: der 409 ist der Riegel, die Oberfläche spart nur den Klick). Die
heutigen Knöpfe «Jetzt abrufen» (Spotlight), «Jetzt prüfen» (Update) und «Aktualisieren» je
Podcast **bleiben samt ihren Endpunkten und Rückmeldungen** (`refreshSpotlight` mit 429 nach
einer Minute, `checkForUpdate` synchron mit `UpdateState`, `refreshPodcast` für einen Podcast);
`runJobType` ruft **dieselben Dienstmethoden** und übernimmt deren Fehlercodes (5.2). Zwei Wege,
ein Verhalten.

**Abbruch.** Je Lauf wie heute (`cancelJob`); neu je Jobart `cancelJobRuns` mit `scope`
`queued` oder `all` (F-2). Beide laufen über den **Abbruch-Haken** der Jobart (`Type.Cancel`,
6.1), damit die Fachlogik stimmt:

| Jobart | Haken | Warum |
|---|---|---|
| `scan.library`, `scan.path` | `library.CancelScan` (`service.go:379`) | schreibt `aborted/cancelled` an die Bibliothek; ohne ihn bliebe `queued` stehen, bis der nächste Zeitplan-Lauf kommt (1.1) |
| `download.prepare` | neu `offline.CancelPreparation`: Download auf `failed` mit Code `cancelled_by_admin` | ohne ihn reiht `Reconcile` den Download nach 10 min wieder ein; mit dem Code lässt `Reconcile` ihn liegen, der Zuschauer sieht «fehlgeschlagen» und kann neu anstossen |
| alle anderen | keiner | der Queue-Abbruch reicht |

Beide Knöpfe mit Bestätigung in der Oberfläche; bei `notify.deliver` mit Warntext, dass die
Mitteilung dann nicht zugestellt wird (9.3); bei `scope = all` nennt die Bestätigung die Zahl
der laufenden Läufe gesondert.

### 6.4 Was mit den heutigen Pfaden geschieht

| Heute | Neu |
|---|---|
| `cancelLibraryScan` auf der Bibliotheksseite | bleibt; ist der Abbruch-Haken |
| «Scannen» je Bibliothek | bleibt (ein Lauf); «Jetzt ausführen» bei `scan.library` reiht **alle** Bibliotheken ein |
| «Jetzt abrufen» (Spotlight), «Jetzt prüfen» (Update), «Aktualisieren» (Podcasts) | bleiben unverändert; `runJobType` ruft dieselben Dienstmethoden (6.3) |
| `getDashboard.jobs` | bleibt, plus `paused` |
| Spotlight-Takt `intervalHours` (Seite Spotlight), `update_check_enabled` (Seite Update) | bleiben; **ein Wert, zwei Türen**: die Jobkarte schreibt in dieselbe Einstellung (4.3). `spotlight_config.enabled` bleibt die Sichtbarkeit des Bands und wird nicht zum Zeitplan-Schalter (13.3) |
| `notify.JobFailed` (Meldung nach letztem Versuch) | bleibt; eine Pause löst sie nie aus (6.3) |
| `offline.Run` (Abgleich alle 10 min) | bleibt; lässt Downloads mit `cancelled_by_admin` liegen |

---

## 7. Oberfläche

Grundsatz wie in jedem Konzept dieses Projekts: der Client prüft keine Regel (Regel 12). Er
blendet Knöpfe aus, die der Server mit 409 beantworten würde (`runnable`, `paused`), mehr nicht.

| Seite / Datei | Was geschieht |
|---|---|
| **Neue Seite** `web/src/features/admin/JobsPage.tsx`, Route `/admin/jobs` (`router.tsx`, `paths.adminJobs`) | Kopf mit Chips und Pools, «Läuft gerade» als Liste (`ui` `Panel`, die **bestehende** `ProgressBar` aus `web/src/ui/ProgressBar.tsx` mit `role="progressbar"` und `aria-valuetext` — Text daneben nur für Sehende, kein doppeltes Ansagen), Karten je Jobart in vier Gruppen (`StatTile` für die Zahlen, `StatusDot` für den Zustand, Chips aus `ui`), Dialog «Läufe von X» mit `Tabs` (wie `MaintenancePage`) und Tabelle wie die Fehlerliste des Dashboards (`AdminTablePage.module.css`). Schranke wie das Dashboard (`AdminHomePage.tsx:336`) |
| **Navigation** `sections.ts` | neuer Eintrag `{ key: 'jobs', href: paths.adminJobs, icon: 'activity', group: 'system' }` **vor** `integrations`; Icon `activity` neu in `web/src/ui/Icon.tsx`, falls keines passt |
| **Dashboard** `AdminHomePage.tsx:507` | Kachel bekommt den Link «Alle Jobs» und, bei `paused > 0`, eine vierte `StatTile` «Pausiert» in `warning` |
| **Abfragen** `features/admin/queries.ts` | `jobTypesQuery` (`refetchInterval: JOBS_POLL_MS = 5000`, aus, solange ein Bestätigungsdialog offen ist, wie `AdminHomePage.tsx:343`), `jobRunsQuery(type, state)`; Mutationen `runJobType`, `pauseJobType`, `resumeJobType`, `cancelJobRuns`, dazu die bestehenden `retryJob`, `cancelJob` |
| **Laufende Dauer** | `useElapsed(startedAt)` zählt sekündlich hoch; mit `prefers-reduced-motion` keine Animation des unbestimmten Balkens |
| **Dialog «Zeitplan von X»** | `JobScheduleDialog.tsx` neben `JobsPage.tsx`: `Switch` Ein/Aus, `Select` Takt (Optionen aus `schedule.limits`, Werte in Sekunden, Beschriftung aus der Sprachdatei), `Switch` Zeitfenster mit zwei `<input type="time">` (Minuten des Tages, Zeitzone aus `schedule.timeZone` als Hinweis), Hinweiszeile bei `trakt.refresh`; Mutation `updateJobSchedule`; 422 zeigt die Feldfehler, wie das Spotlight-Formular (`SpotlightPage.tsx`) |
| **Zeile «Läuft gerade»** | Knöpfe **Pausieren** (ruft `pauseJobType(run.type)`, beschriftet «Lauf … pausieren» mit Jobname, deaktiviert, solange `stopping`) und **Abbrechen** (`cancelJob(run.id)`); Komponententest `JobsPage.test.tsx` (Vitest, wie `AdminHomePage.test.tsx`) mit zwei laufenden Läufen derselben Jobart: ein Klick auf Pausieren in der ersten Zeile ruft die Mutation **einmal** mit der Jobart, beide Zeilen zeigen «wird angehalten» |
| **Bibliotheken, Spotlight, Integrationen, Podcasts** | **unverändert** |

**Regel 8 — Tastaturtest.** Neue Route, also `web/e2e/admin-jobs.spec.ts`: Seite über die
Verwaltungsnavigation erreichen (Tab, Pfeiltasten, Enter), «Läuft gerade» und die erste Karte per
Tab, «Jetzt ausführen» bei `library.sweep` per Enter (Rückmeldung «Lauf eingereiht», dann
`expect.poll` auf den Endzustand: die Zeile erscheint im Reiter *Erledigt*), «Pausieren» per Enter
und Zustandschip «Pausiert», «Fortsetzen», «Wartende abbrechen» öffnet die Bestätigung (Tab in
die Bestätigung, Escape schliesst, Fokus zurück auf dem Knopf), Dialog «Läufe von X» (Reiter per
Pfeiltaste, Escape, Zurück, keine Fokusfalle), Dialog «Zeitplan von X» bei `library.sweep` (Tab
durch Ein/Aus, Takt, Zeitfenster, Takt per Pfeiltaste auf 2 h, Enter speichert, die Karte zeigt
«alle 2 h»; `afterEach` stellt 1 h ohne Fenster wieder her), sichtbarer Fokus überall. Der Test nutzt
`library.sweep` und `jobs.prune`, weil beide am e2e-Server in Sekunden enden und nichts Fremdes
anstossen; den Knopf «Pausieren» in der Zeile «Läuft gerade» erreicht der e2e-Test darum nicht
(kein Lauf lebt dort lange genug), ihn deckt der Komponententest aus der Tabelle oben ab; weil
eine Pause den Neustart überlebt (F-6) und der e2e-Server geteilt ist, setzt
`afterEach` jede pausierte Jobart über `resumeJobType` fort. `admin-home.spec.ts:25`
(`NAV_PAGES`) bekommt die Zeile `['Hintergrundjobs', '/admin/jobs']`; `admin-home.spec.ts`
prüft zusätzlich den Link «Alle Jobs».

**Regel 9 — Texte.** Neue Schlüssel in `de.json` und `en.json`: `admin.sections.jobs.{title,
description}`, `admin.jobs.{title,intro,pools,running,runningEmpty,queued,planned,queuedEstimate,
backlog,noEstimate,lastRun,nextRun,trigger,onDemand,paused,stopping,pausedBanner,run,pause,resume,
cancelQueued,cancelAll,cancelTitle,cancelBody,cancelAllBody,runs,columns.*,
estimate.{computing,longer,about},state.{idle,queued,running,paused,stopping,unknown},
groups.{library,metadata,delivery,system}}`, dazu `admin.jobs.types.<type>.{name,trigger}` für
achtzehn Typen — **mit `_` statt `.` im Typnamen** (`scan_library`), weil i18next den Punkt als
`keySeparator` liest; der Rückfall auf den Code läuft über `defaultValue`. Die Problemcodes
`job_type_not_runnable` und `job_type_paused` liegen wie die heutigen unter `errors.*`
(`de.json:156`, `job_not_failed`); `admin.home.jobs.{all,paused}` für die Kachel.
`pnpm -C web test locales` prüft die Symmetrie. Farben nur aus Tokens: der Balken nutzt
`color.state.track` für den leeren Teil und die Akzentfarbe für den vollen, der Pausen-Hinweis
`color.state.warning`.

### 7.1 Klickbarer Prototyp

`docs/planning/mockups/09-hintergrundjobs-prototyp.html` ist eine einzelne HTML-Datei ohne
Abhängigkeiten (Doppelklick genügt), gebaut wie die Prototypen zu 07 und 08: die Seite
«Hintergrundjobs» mit Marcos Zahlen vom 29. Oktober 2026 (236 wartend, 4 laufend, 0
fehlgeschlagen), Pools mit Stau, «Läuft gerade» mit vier Läufen (ein Scan mit Fortschritt und
Restzeit, zwei Segmenterkennungen mit Median-Schätzung, eine Zustellung ohne), achtzehn
Jobkarten in vier Gruppen, der Dialog «Läufe von X» mit Reitern und die Knöpfe mit Wirkung
(Pausieren auf der Karte **und** in der Zeile «Läuft gerade» hält die Läufe des Jobs an und zeigt
den Hinweis im Kopf, Jetzt ausführen reiht Läufe ein, Wartende abbrechen und Alle Läufe
abbrechen fragen nach, Zeitplan … öffnet den Dialog mit Ein/Aus, Takt und Zeitfenster und
schreibt den nächsten Lauf auf die Karte). Ein Knopf «Handy-Ansicht» schaltet auf 390 px. Farben und Abstände sind
die Tokens aus `packages/design-tokens/tokens/`. Der Prototyp ist **kein** Code für `web/src`
und wird nicht gebaut oder getestet. Stand: vierte Fassung vom 30. Oktober 2026 (Pausieren in
der Laufzeile, Zeitplan-Dialog). Dieselbe Fassung ist als privates Artefakt unter
`https://claude.ai/artifact/E9nZyNurBZL7gGBEXYgPbK` erreichbar.

---

## 8. Migration und Rückwärtsverträglichkeit

### 8.1 Was mit dem Bestand geschieht

- Alte `jobs`-Zeilen haben `started_at = NULL`: sie zeigen keine Dauer und zählen nicht zum
  Median; der erste `jobs.prune` räumt sie nach 30 Tagen weg (die neuesten 50 je Jobart bleiben,
  ohne Dauer).
- Die Zeitpläne beginnen beim ersten Start nach der Migration aus der bisherigen Historie
  (4.3): `update.check` und `trakt.refresh` aus `checked_at` bzw. `last_attempt`, die drei
  anderen sofort bzw. nach `every_ms`. Die Poller-Historie bleibt für die Anzeige «zuletzt
  geholt» ihrer Seiten, treibt aber keinen Lauf mehr.
- Nichts ist pausiert (`paused_at` NULL überall).

### 8.2 Wo alte Clients scheitern

Nirgends. Ein Client auf 1.4.0 sieht `getDashboard` unverändert (`paused` ist optional) und
kennt `/admin/job-types` nicht. Ein Server auf 1.4.0 mit einem 1.5.0-Client: die neue Seite
bekommt 404 und zeigt den Fehler wie jede Seite; das Dashboard geht.

### 8.3 Eingefrorene Datenbankstände

`testdata/db/` bekommt den Stand nach 00027 mit je einer Zeile in `job_types` und
`job_schedules`, einem `done`-Lauf mit `started_at`/`finished_at`, einem übersprungenen und einem
`running`-Lauf mit Fortschritt; `frozen_*_test.go` prüft wie bei M11.

---

## 9. Sicherheit

### 9.1 Nur Administratoren, nichts Scoped

Alle Operationen sind `x-visibility: admin`; der Schema-Test erzwingt es; die Seite hat die
Schranke des Dashboards. Kein `ItemRef`, keine Bilder, keine Pfade des Zuschauers. Gegenstand
und Fortschrittsnotiz nennen Bibliothek und Ordner relativ zur Wurzel — Pfade, die der
Administrator auf der Bibliotheksseite ohnehin sieht; **nie** den absoluten Pfad, nie einen
Feed-Token, nie eine Adresse. `lastError`, `subject` und `progress_note` gehen durch
`logging.RedactString` (Regel 10): die Notiz beim Schreiben im Reporter, der Gegenstand bei der
Ableitung.

### 9.2 Keine neue Aussenverbindung, kein `os/exec`

Der Scheduler reiht ein, mehr nicht; die Handler bleiben in ihren Paketen (Regel 3 unberührt).

### 9.3 Abbrechen ist endgültig, Pausieren hat Folgen

- `notify.deliver` abbrechen = die Mitteilung kommt nicht an. Bestätigung mit Warntext.
- `download.prepare` abbrechen = der Download des Profils endet `failed` mit
  `cancelled_by_admin`; der Zuschauer sieht «fehlgeschlagen» und kann neu anstossen; der Abgleich
  reiht ihn **nicht** wieder ein (6.3).
- `scan.library` abbrechen = die Bibliothek endet `aborted/cancelled` wie beim heutigen Knopf;
  der nächste Zeitplan-Lauf konvergiert.
- `scan.library` pausieren = neue Dateien bleiben unerkannt, Webhooks von Radarr und Sonarr
  reihen weiter ein (die Läufe warten). Der Kopf der Seite und das Dashboard sagen das in
  Warnfarbe, solange die Pause steht.
- Pause von `notify.deliver` = Zustellungen stauen; beim Fortsetzen kommen sie **alle** nach.
  Der Hinweistext nennt das.

### 9.4 Die vergessene Pause

Das grösste Risiko dieses Konzepts ist eine Pause, die nach Tagen noch steht und niemand mehr
sieht. Darum: `paused` im Dashboard als eigene Kachel in Warnfarbe, der Banner oben auf der
Seite, die Pause im Log, **und** kein automatisches Fortsetzen — ein Server, der von selbst
wieder losscannt, wäre schlimmer als einer, der steht, denn die Pause hat einen Grund. Eine
Erinnerung als Mitteilung nach 24 h wäre eine neue Mitteilungsart und ein weiterer Zeitgeber;
sie ist als Idee notiert, nicht Teil dieses Papiers (Regel 11).

### 9.5 Zeitplan-Eingaben prüft der Server

Takt und Zeitfenster kommen aus einem Formular, also gelten die Grenzen im Server, nicht im
Client (Regel 12): `everySeconds` innerhalb `limits`, Minuten des Tages 0 bis 1439, Anfang
ungleich Ende, beides oder nichts; alles andere 422 mit Feldname. Ein Takt unter fünf Minuten
ist auch für den Administrator tabu (`CHECK` in 4.3), sonst reiht ein Tippfehler alle 10 s einen
Bibliotheksscan ein. Die Zeitzone ist die des Servers (`TZ` im Container, auf CT 190
`Europe/Zurich`); der Dialog nennt sie, damit ein Fenster «03:00–05:00» nicht in UTC gelesen
wird.

---

## 10. Schnitt in Schritte

Jeder Schritt ein Commit, `task ci` grün, DoD-Belege mit Befehl und Ausgabe. Zweig ab `main`
`05df9c5`, nachdem dieses Papier **und Konzept 08** gemergt sind (der Zweig trägt den Namen seines
Worktrees, `claude/vigorous-raman-651439`; Schritte 0 und 1 in einem PR).

### Schritt 0 — Entscheide und Papiere

Frage 15 (F-1 bis F-10, O-1, O-2) ins Entscheidungsprotokoll; ADR 0017 «Scheduler, Fortschritt, Pause und
Abbruch-Haken in der Job-Queue» als Teilnachfolger von ADR 0002 (Abschnitte «Zeitpläne» und
«Abbruch»); Hinweis in ADR 0002; dieses Papier auf den dann aktuellen `main` verankern (Zeilen,
Nummern, `metadata.vote_backfill` nach dem Stand von 08). `git diff --stat docs/spec/` leer.

### Schritt 1 — Schema, Registry, Scheduler

Migration 00027 (4.5), `jobs.Type`, `jobs.Registry`, Scheduler mit Uhr-Mock-Tests, die fünf
Ticker-Schleifen raus, `RegisterJobs` → `RegisterTypes` in fünf Paketen, `job_types` und
`job_schedules` beim Start gefüllt (nur fehlende Zeilen), `UpdateSchedule` mit Fenster-Ausrichtung
aus Spotlight- und Update-Einstellungen,
`podcast.poll` als Zeitplan-Jobart mit `QueueDuePodcasts` als `Run`, Prioritätsparameter in
`enqueueScan` und Geschwistern. Kein Vertragsbruch, keine Oberfläche. DoD: `task ci`;
`go test ./internal/jobs/ -run Scheduler`; auf dem Entwicklungsrechner nach `task dev` zeigt
`sqlite3 …/slouchcat.db 'SELECT type, next_run_at FROM job_schedules'` **fünf** Zeilen
(`jobs.prune` kommt mit seinem Handler in Schritt 2; ein Zeitplan ohne Handler bliebe ewig
wartend).

### Schritt 2 — Fortschritt, Pause, Abbruch-Haken, Verlauf

`jobs.Progress`, `jobs.Skip`, Reporter im Arbeiter, vier meldende Handler; `paused_at` in
`ClaimJob`, Laufliste im Speicher, sofortiger Abbruch mit `ErrPaused`, `Release` vor dem
`switch` in `worker.run`; `Type.Cancel` mit `CancelScan` und `offline.CancelPreparation`
(`cancelled_by_admin`, `Reconcile` lässt ihn liegen); `started_at`/`finished_at` in
Claim/Complete/Fail/Cancel/FailExhaustedLeases, Rücksetzen in `RetryFailedJob`; `jobs.prune` mit
Batches und Zeitplanzeile; Median-Abfrage. Tests: «pausiert wird nicht beansprucht», «laufender
Scan hält sofort an und ist wieder queued, Versuche unverändert, kein JobFailed, auch wenn der
Handler nil zurückgibt», «Fortschritt gedrosselt», «prune behält 50», «abgebrochener Download
wird nicht wieder eingereiht», «Run hebt einen wartenden Lauf an».

### Schritt 3 — Vertrag 1.5.0 und Server-Handler

`api/openapi.yaml` (5), `task gen`, die sieben Operationen im Paket `admin` (Queue-Methoden
`ListTypes`, `ListRuns`, `RunType`, `Pause`, `Resume`, `CancelRuns`, `UpdateSchedule` mit
Durchschreiben in `spotlight_config` und `update_check`), Gegenstand je Jobart,
Schätzungen (6.2), Vertragstest, Fixtures (`web/src/test/fixtures.ts`, `web/e2e/fixtures.ts`).
Web-Minimum: der api-client kompiliert, keine neue Seite.

### Schritt 4 — Oberfläche

`JobsPage.tsx`, `JobScheduleDialog.tsx`, Navigation, Dashboard-Link und -Kachel, Sprachdateien,
`admin-jobs.spec.ts` mit `afterEach`, `NAV_PAGES`. DoD: `task ci`; `pnpm -C web test locales`; Playwright-Lauf der zwei
Specs mit Ausgabe.

### Schritt 5 — Abnahme, Messung, Doku

Image bauen, CT 190 umstellen (10.1), Marco nimmt am Handy ab: Scan starten, pausieren,
fortsetzen, Wartende abbrechen, Restzeit eines NAS-Scans beobachten, Zeitplan des Scans auf
«täglich, 03:00–05:00» stellen und am Morgen den Lauf im Verlauf sehen. Messung nach
`docs/perf/hintergrundjobs.md`: Kosten von `listJobTypes` bei 50 000 Zeilen in `jobs` (vor dem
ersten `prune`) und danach, Ziel < 50 ms p95 auf CT 190; Vergleich Schätzung gegen Wirklichkeit
an einem NAS-Scan (12.3). Ein Tag Beobachtung der Zeitpläne vor dem Merge (12.1).
`docs/dev/CURRENT_STATUS.md`, Release-Notes-Zeile (5.4).

### 10.1 Der Weg jedes Schritts

Wie `07-bibliotheken.md` 10.8, mit denselben zwei Umgebungen (Entwicklung hier, Test auf CT 190,
`192.168.1.43`): Zweig, Commit je Schritt, `task ci`, Push, Abnahme auf CT 190 vor dem Merge
(Image `ghcr.io/slouchcat/slouchcat:hintergrundjobs`, `.env.bak-vor-hintergrundjobs`,
`/healthz`), PR gegen `main`, `bind_pr`, Merge erst mit grüner CI **und** Marcos Wort. Die
Datenbank dreht sich nicht zurück: 00027 baut `job_schedules` neu; der Server sichert vorher
nach `/config/backups/`.

---

## 11. Fragen an Marco

Format wie `docs/planning/04-entscheidungen.md`. Alle Antworten gehen als **Frage 15** ins
Entscheidungsprotokoll, sobald das Papier freigegeben ist.

| Nr. | Frist | Thema | Empfehlung | Antwort | Datum |
|-----|-------|-------|------------|---------|-------|
| **F-1** | vor Schritt 2 | **Was heisst Pause für einen laufenden Lauf?** (a) Nur keine neuen Starts, Laufendes läuft zu Ende — ein NAS-Scan also noch Stunden; (b) Laufendes hält sofort an und geht ohne verbrauchten Versuch in die Queue zurück, beim Fortsetzen wird es wieder aufgenommen | **(b)**: sonst ist «Pause» bei genau dem Job eine Lüge, bei dem sie zählt (6.3) | **(b)**, hält sofort an | 30. Oktober 2026 |
| **F-2** | vor Schritt 3 | **Abbrechen je Job.** (a) Nur die Wartenden («Wartende abbrechen (236)»), laufende Läufe einzeln; (b) zusätzlich «Alle Läufe abbrechen» (wartende **und** laufende) mit Bestätigung, die die laufenden gesondert nennt; (c) auch Pause je einzelnem Lauf | **(b)**. Start und Pause bleiben je Jobart; (c) nicht: ein Lauf von zwei Sekunden hat keine Pause (2.1, F-10) | **(b)**, Wartende und Alle Läufe abbrechen | 30. Oktober 2026 |
| **F-3** | vor Schritt 3 | **«Jetzt ausführen» auch für Ereignis-Jobs** (`metadata.match`, `image.fetch`, `probe.file`, …)? Dafür müsste jeder eine Bedeutung von «alle» bekommen («alle Titel ohne Bilder») | **Nein.** Das Nachholen gehört zur Seite Pflege (Befunde) und zum Scan; die Karte zeigt «bei Bedarf» und den Auslöser | **Nein** | 30. Oktober 2026 |
| **F-4** | vor Schritt 1 | **Zeitpläne einstellbar** (alle 6 h → alle 12 h, nachts)? | **Nur anzeigen.** Spotlight-Takt bleibt an seiner Seite einstellbar; alles Weitere als Idee «Zeitpläne» notieren, mit `every_ms` ist der Boden gelegt | **Einstellbar: Takt, Zeitfenster, Ein/Aus** (Abweichung von der Empfehlung; eingearbeitet in Abgrenzung, 2.2, 2.3, 4.3, 5.2, 5.3, 6.1, 6.4, 7, 9.5, 10, 12.7) | 30. Oktober 2026 |
| **F-5** | vor Schritt 1 | **Verschiebt «Jetzt ausführen» den nächsten geplanten Lauf?** | **Nein.** Takt bleibt; wer um 14:00 von Hand scannt, bekommt den 6-h-Lauf trotzdem um 18:00 — vorhersagbar, idempotent, billig | **Nein**, Takt bleibt | 30. Oktober 2026 |
| **F-6** | vor Schritt 2 | **Überlebt eine Pause den Neustart?** | **Ja** (Spalte, nicht Speicher), mit Warnkachel im Dashboard und Banner (9.4) | **Ja** | 30. Oktober 2026 |
| **F-7** | vor Schritt 4 | **Wortwahl.** «Hintergrundjobs» als Seitentitel (wie die Kachel heute); «Job» für die Jobart, «Lauf» für die Zeile; Knöpfe «Jetzt ausführen», «Pausieren», «Fortsetzen», «Abbrechen», «Wartende abbrechen», «Alle Läufe abbrechen» | wie links | **So lassen** | 30. Oktober 2026 |
| **F-8** | vor Schritt 2 | **Verlauf:** erledigte Läufe 7 Tage, mindestens 50 je Jobart; fehlgeschlagene bleiben bis «Erneut» oder «Abbrechen» | wie links | **30 Tage, mindestens 50** (Abweichung; 4.4, 8.1) | 30. Oktober 2026 |
| **F-9** | vor Schritt 4 | **Arbeiterzahl in der Oberfläche einstellbar?** Heute nur `SLOUCHCAT_WORKERS` in `deploy/.env` (auf CT 190 so gedrosselt); die Seite zeigt die Zahl je Pool | **Nein, nur anzeigen**, mit Hinweis «`SLOUCHCAT_WORKERS` in deploy/.env». Ein Regler im Browser müsste Arbeiter zur Laufzeit starten und stoppen — eigenes Thema, Idee | **Nur anzeigen** | 30. Oktober 2026 |
| **F-10** | vor Schritt 3 | **Pausieren in der Zeile «Läuft gerade»** (Marcos Wunsch vom 30. Oktober 2026). (a) Der Knopf pausiert den **Job** des Laufs: dieser Lauf hält an, weitere laufende Läufe derselben Jobart ebenfalls, kein neuer Start bis «Fortsetzen» — gleicher Knopf wie auf der Karte, kein neuer Endpunkt; (b) der Knopf hält nur **diesen Lauf** an und legt ihn beiseite (neuer Zustand `held` je Lauf, eigener Endpunkt `pauseJobRun`, eigener Reiter *Angehalten* im Dialog), andere Läufe und neue Starts der Jobart laufen weiter | **(a)**. Die zwei Jobarten, bei denen Pause zählt (`scan.library`, `download.prepare`), laufen je Gegenstand ohnehin nur einmal; (b) brächte einen sechsten Zustand (ADR 0002), eine dritte Stelle für «Fortsetzen» und die Frage, was mit einem beiseitegelegten Lauf nach Tagen geschieht | **(a)**, pausiert den Job | 30. Oktober 2026 |

Offene Punkte, die keine Entscheidung brauchen, aber eine Antwort verdienen:

- **O-1** Sollen `done`-Läufe im Dialog «Läufe von X» überhaupt erscheinen (Reiter *Erledigt*)?
  Vorschlag: ja, die neuesten 50 — man sieht, was der Server die letzte Stunde getan hat.
  **Antwort (30. Oktober 2026): ja, die neuesten 50.**
- **O-2** Zeigt die Karte `scan.library` je Bibliothek eine Zeile («Filme: läuft, 31 %», «Serien:
  wartet»)? Vorschlag: nein — die Bibliotheksseite hat das; unter «Läuft gerade» steht die
  Bibliothek als Gegenstand. **Antwort (30. Oktober 2026): nein.**
- **O-3** Reihenfolge: dieses Papier geht **nach 08** (Kopf). Erledigt durch den Lauf der
  Dinge: 08 ist seit dem 30. Oktober 2026 in `main` (PR #47), der Zweig für 09 kann von `main`
  geschnitten werden (12.5).

---

## 12. Risiken

### 12.1 Fünf Schleifen werden eine

Der Scheduler ersetzt Code, der seit M1a, M6, M8 und M11 läuft. Das Risiko ist ein verlorener
Takt (Trakt läuft nie mehr, Updates werden nie geprüft) oder ein verlorener Sonderfall (Trakt
ohne Client-Schlüssel leert die Einträge, 4.3). Gegenmittel: die Zeilen in `job_schedules`
sind **sichtbar** — auf der neuen Seite steht «Nächster Lauf» für jede, und ein fehlender
Eintrag fällt beim ersten Blick auf. Dazu die Uhr-Mock-Tests je Zeitplan, der Test für den
Schlüssel-Sonderfall und auf CT 190 ein Tag Beobachtung vor dem Merge (Schritt 5).

### 12.2 Pause und Abbruch gegen den heutigen Arbeiter

Der Arbeiter unterscheidet heute nur «Herunterfahren» von «Fehler»; ein dritter Grund
(Pause) muss **vor** dem `switch` in `worker.run` erkannt werden, sonst verbraucht jede Pause
einen Versuch und meldet nach fünf Pausen einen Fehlschlag (6.3). Und der rohe Queue-Abbruch
lässt Scan und Download in einem Fachzustand zurück, den der Abgleich wieder einreiht oder die
Bibliotheksseite als «wartet» zeigt (1.1). Beides sind gezielte Tests in Schritt 2 («fünfmal
pausieren», «abgebrochener Download bleibt liegen»); ohne sie wäre der Knopf schlimmer als
keiner.

### 12.3 Die Schätzung ist falsch, und Marco glaubt ihr

Ein Balken wirkt genauer, als er ist. Darum «ca.», «wird berechnet», «länger als üblich» als
drei ausdrückliche Zustände und nie eine Sekundenangabe unter einer Minute, und darum der
Median nur über Läufe, die gearbeitet haben (`skipped = 0`). Nach Schritt 5 vergleicht die
Messung Schätzung und Wirklichkeit an einem NAS-Scan; liegt der Median-Fall um mehr als Faktor 2
daneben, verschwindet er, und nur der gemessene Fall bleibt.

### 12.4 `jobs` ist gross, bevor `prune` das erste Mal lief

`listJobTypes` zählt und sucht den letzten Lauf je Jobart; mit Zehntausenden `done`-Zeilen und
ohne `jobs_history` wäre das ein Table-Scan alle 5 s. Der Index kommt in derselben Migration;
die Messung in Schritt 5 prüft den Fall «vor dem ersten prune».

### 12.5 Konzept 08 fasst dieselben Dateien an und bringt eine Jobart mit

`sections.ts`, `router.tsx`, `de.json`/`en.json`, `admin-home.spec.ts` (`NAV_PAGES`) und die
Registry-Aufrufe in `metadata` werden von 08 **und** 09 angefasst, und 08 bringt
`metadata.vote_backfill`. Darum geht 09 nach 08 (Kopf, O-3). 08 ist seit dem 30. Oktober 2026
gemergt (PR #47); der Zweig für 09 ist von `main` `05df9c5` geschnitten, und Schritt 0 hat das
Papier neu verankert (13.3) — die Zeilenangaben in `AdminHomePage.tsx` sind geprüft, `sections.ts`
und `router.tsx` nennt das Papier ohne Zeile.

### 12.6 Was dieses Papier nicht zählen konnte

Wie viele `done`-Zeilen auf CT 190 liegen (nach 29 753 `probe.keyframes` und 1683
`segments.detect` sicher Zehntausende) — Schritt 5 misst. Ob `segments.detect` die Folgen eines
Laufs vorab zählen kann (sonst meldet er nur die Notiz). Wie 08 `metadata.vote_backfill`
anstösst, ist seit Schritt 0 geklärt (nach jedem vollständigen Scan und beim Start, 1.1, 4.3):
Gruppe *Metadaten*, Auslöser «vom Scan, solange Titel ohne Wertung warten».

### 12.7 Ein Zeitfenster kann einen Job aushungern

Takt 6 h und Fenster 03:00–04:00 heisst: ein Lauf am Tag, nicht vier. Das ist gewollt, aber
leicht vergessen. Darum steht der nächste Lauf mit Takt **und** Fenster auf der Karte («in 9 h
48 min · alle 6 h · 03:00–04:00»), und ein Fenster, in das der Takt nie fällt, gibt es nicht:
die Ausrichtung rückt auf den Fensterbeginn (6.1). Der zweite Fallstrick ist die Zeitzone: ein
Container ohne `TZ` rechnet in UTC, und «03:00» ist dann 04:00 oder 05:00 Schweizer Zeit; der
Dialog zeigt die Zone des Servers (5.3, 9.5), und `deploy/compose.yaml` setzt `TZ` schon heute.

---

## 13. Review-Protokoll (30. Oktober 2026)

Fassung 1 (Commit `269cf7b`) ging an den OMC-Kritiker (`oh-my-claudecode:critic`), geprüft
gegen `main` `4049724` und die Worktrees von 08. Ergebnis: **0 Blocker, 8 Major, 16 Minor**,
Urteil «Überarbeitung vor der Freigabe». Alle 24 Befunde sind in Fassung 2 eingearbeitet:

| Nr. | Grad | Befund (kurz) | Einarbeitung |
|---|---|---|---|
| 1 | Major | Pause über `jobCtx` landet in `worker.run` im `Fail`-Pfad (Versuch verbraucht, Backoff, nach fünf Pausen `failed` und `JobFailed`; `nil` vom Handler → `done`) | 6.3: `context.Cause == ErrPaused` **vor** dem `switch`, immer `Release`; Test «fünfmal pausieren»; 12.2 |
| 2 | Major | Roher Queue-Abbruch umgeht die Fachlogik: `Reconcile` reiht Downloads nach 10 min neu ein, Scan bleibt `queued`; 1.1 und 9.3 falsch | `Type.Cancel`-Haken (6.1, 6.3): `CancelScan`, `offline.CancelPreparation` mit `cancelled_by_admin`; 1.1, 6.4, 9.3 korrigiert |
| 3 | Major | «Jetzt …»-Knöpfe sind nicht `runJobType` (synchron, 422, 429, ein Podcast) | Knöpfe und Endpunkte bleiben; `runJobType` ruft dieselben Dienstmethoden und übernimmt die Codes (5.2, 6.3, 6.4) |
| 4 | Major | 08 ist in Umsetzung (`claude/startseite`, `dca8efc`, Frage 14, ADR 0016, 00026, 1.4.0) und bringt `metadata.vote_backfill`; «berühren sich nicht» falsch | Kopf, 1.1, 2.1, 4.3, 12.5, O-3: 09 setzt 08 voraus, 15 / 0017 / 00027 / 1.5.0 fest, Jobart aufgenommen; altes F-9 gestrichen |
| 5 | Major | Drei Zeitpläne passen nicht: `podcast.fetch` als Zeitplan widerspricht sich, `update.check` 60 s nicht abbildbar, Trakt feuert bei jedem Start und verliert den Schlüssel-Sonderfall | 4.3: `podcast.poll` als sichtbarer Takt, `start_delay_ms`, `next_run_at` aus der Historie, Trakt ohne Startfeuer, Sonderfall im `Run` |
| 6 | Major | Median durch Millisekunden-Läufe (Download-Warten, Scan ohne Ordner) verfälscht; Wartezeit ignoriert Priorität und geteilten Pool; Gesamtstau fehlt | `skipped` (4.1, 6.2), Wartezeit kumuliert nach Priorität, `backlogSeconds` je Pool im Kopf |
| 7 | Major | `JobRun` ohne `priority`, `runAfter`, Gegenstand; Notizen nur von drei Handlern; *Wartend* falsch sortiert, ohne Gesamtzahl | 5.3 `subject` (Server, redigiert), `priority`, `runAfter`; `JobRunPage.total`; Claim-Reihenfolge; Zustand «geplant» (2.2) |
| 8 | Major | Migrationsplan ohne `BEGIN`/`COMMIT`, `RAISE` ausserhalb eines Triggers, überflüssiges `INSERT … SELECT` | 4.5 nach Vorlage, Wächter als CHECK-Tabelle wie 00023, `INSERT` gestrichen |
| 9 | Minor | Scan und Download schreiben bei Kontextabbruch schon heute `queued`; `jobs.go:215` gibt es nicht | 6.3 korrigiert, Handler ohne Ursachenprüfung; Zeilen berichtigt |
| 10 | Minor | «CT 190 zwei Arbeiter» und «Rechner mit vier» falsch; «Laufend» zählt alle Pools | 1.1 korrigiert (`SLOUCHCAT_WORKERS`, vier auf CT 190) |
| 11 | Minor | Sweep hat keine Batches; Download kennt Segmente, nicht Bytes; lange Läufe (`segments.detect`, `probe.keyframes`) melden nichts; Scan meldet erst in der Schreibphase | 6.2 Tabelle der meldenden Handler, Durchsuchphase mit `total = 0`, Segmente |
| 12 | Minor | «sieben» Operationen, Tabelle nennt sechs | 2.3, 5.2, Schritt 3: sechs |
| 13 | Minor | `ProgressBar` gibt es schon, mit `role="progressbar"` | 7: bestehende Komponente, kein `aria-hidden` |
| 14 | Minor | Schritt 1 verlangt sechs Zeilen, `jobs.prune` kommt erst in Schritt 2 | Schritt 1: fünf Zeilen; Begründung |
| 15 | Minor | Abgrenzung «keine neuen Jobarten» gegen `jobs.prune`; O-2 braucht Zeitgeber und Mitteilungsart | Abgrenzung präzisiert (Haushalt der Queue); Erinnerung als Idee (9.4) |
| 16 | Minor | `finished_at` fehlt in `FailExhaustedLeases`; `RetryFailedJob` muss zurücksetzen | 4.1 |
| 17 | Minor | Scheduler-Regeln offen: pausierte Jobart, `UpdateSchedule`, unregistrierter Typ | 6.1 drei Regeln mit Tests |
| 18 | Minor | 60 s Verzug der Pause; sofortiger Abbruch über Laufliste im Speicher möglich | 6.3 sofortiger Abbruch, Heartbeat als Rückfall, Zustand «wird angehalten» (`stopping`) |
| 19 | Minor | Punkte in Schlüsseln (i18next `keySeparator`), `defaultValue`, Pfad der Problemcodes | 7: `_` statt `.`, `defaultValue`, `errors.*` |
| 20 | Minor | `progress_note` nicht redigiert | 6.2, 9.1: `RedactString` im Reporter und bei `subject` |
| 21 | Minor | Prioritäten als Konstantennamen; Auslöser fehlen; `grep DELETE FROM jobs` trifft Tests | 1.1 Zahlen, Auslöser ergänzt, Satz korrigiert |
| 22 | Minor | `enqueueScan` reiht mit fester `prioScan`; `Run` muss anheben | 3, 6.1: Vertrag von `Run`, Prioritätsparameter, Test je Jobart |
| 23 | Minor | «Abbrechen je Job» meint wohl auch laufende Läufe | F-2 mit Variante «Alle Läufe abbrechen», `cancelJobRuns?scope=all` |
| 24 | Minor | e2e: Pause überlebt am geteilten Server; «oder» nicht deterministisch | 7: `afterEach` fortsetzen, `expect.poll` auf *Erledigt* |

### 13.1 Nachtrag vom 30. Oktober 2026

Marco wünscht nach der Durchsicht des Prototyps, auch in der Liste «Läuft gerade» pausieren zu
können. Eingearbeitet als Knopf **Pausieren** je Zeile, der den Job des Laufs pausiert (2.1,
2.2, 6.3, 7, 7.1); kein neuer Endpunkt, kein neuer Zustand. Die Alternative «nur diesen Lauf
anhalten» steht als F-10 zur Entscheidung. Prototyp und Artefakt tragen den Knopf.

### 13.2 Marcos Antworten vom 30. Oktober 2026

Marco hat F-1 bis F-10, O-1 und O-2 beantwortet (Tabelle in 11). Acht Antworten folgen der
Empfehlung. Zwei weichen ab und sind in dieser Fassung 3 eingearbeitet:

- **F-4 Zeitpläne einstellbar** — Takt, Zeitfenster und Ein/Aus je Zeitplan-Job auf der Seite:
  Spalten `window_start`/`window_end` und Grenzen in `job_schedules` (4.3), siebte Operation
  `updateJobSchedule` mit `JobScheduleUpdate` und `schedule.limits`/`timeZone` (5.2, 5.3),
  Fenster-Ausrichtung im Scheduler (6.1), «ein Wert, zwei Türen» für Spotlight und Update (4.3,
  6.4), Dialog «Zeitplan von X» mit Tastaturtest (2.2, 7), Prüfung im Server (9.5), Schritte 1, 3,
  4, 5 (10), Risiko 12.7. Die Abgrenzung schliesst nur noch Kalender-Zeitpläne (cron) aus.
- **F-8 Verlauf 30 Tage** statt 7, mindestens 50 je Jobart (4.4, 8.1). Auf CT 190 heisst das
  mehr Zeilen in `jobs`, bevor `prune` greift; die Messung in Schritt 5 (12.4) deckt den Fall.

Drei offene Fragen des Kritikers ohne Zählung: Schranke «Administrator im Hauptprofil» — ja,
wie das Dashboard (2.2, 7, 9.1); Arbeiterzahl einstellbar — als F-9 an Marco; `rating.verify`
als Job — bleibt draussen (3, Regel 11).

### 13.3 Neuverankerung in Schritt 0 (30. Oktober 2026)

Geprüft gegen `main` `05df9c5` (Konzept 08 samt Nachträgen PR #48 bis #53, dieses Papier aus
PR #54). Was sich seit `4049724` geändert hat, und wie das Papier damit umgeht — **wo Papier und
Code sich widersprachen, gilt der Code**:

| Stelle | Papier (Fassung 3) | Code in `05df9c5` | Folge |
|---|---|---|---|
| 1.1, Priorität von `update.check` | 1 | **8** seit PR #52 (30. Oktober 2026, `CURRENT_STATUS.md`): die tägliche Prüfung wartete hinter dem Rückstau eines Imports | Tabelle berichtigt; der Scheduler ruft `Run` mit 8, der Hand-Lauf mit 7 |
| 1.1, 4.3, `metadata.vote_backfill` | Hand-Jobart mit `Run`, «Details folgen dem Stand von 08» | 08 stösst ihn **selbst** an: `QueuePendingMetadata` nach jedem vollständigen Scan und beim Start (`main.go`), nur wenn kein Job des Typs aktiv ist und Titel ohne Wertung warten; Kette mit Cursor, Priorität 2 | **Ereignis**-Jobart ohne `Run` (F-3); kein Hand-Knopf |
| 1.1, 4.3, 6.4, Ein/Aus von `trakt.refresh` | `enabled` des Zeitplans = `spotlight_config.enabled` | Der Trakt-Poller prüft `enabled` **nicht**; `enabled = 0` blendet nur das Band aus (`visibility/discover.go:277`), die Liste wird weiter geholt, damit der Cache warm bleibt und das Einschalten sofort etwas zeigt | Das Ein/Aus des Zeitplans ist eigenständig (Vorgabe 1) und liegt nur in `job_schedules`; «ein Wert, zwei Türen» gilt für den **Takt** (`interval_hours`). Für `update.check` gilt es für Takt **und** Ein/Aus wie beschrieben, denn dort prüft der Code `update_check_enabled` |
| Zeilenangaben | Stand `4049724` | geprüft | Berichtigt: `service.go:847` (Wächter), `:761` (`Start`), `:488` (`enqueueScan`); `config.go:262`; `00001_jobs.sql:32`; `podcast.go:349`/`:365`; `trakt.go:142`–`:149`; `update.go:89`; `AdminHomePage.tsx:531`; `api/openapi.yaml:2540`; `00023_library_roots_providers.sql:120`; `de.json:156`. Unverändert: `manage.go:193`, `sweep.go:29`, `cleanup.go:24`, `job.go:103`/`:478`/`:513`, `write.go:249`, `scanner.go:330`, `service.go:379`, `maintenance.go:155`, `worker.go:121`, `AdminHomePage.tsx:336`/`:343`, `admin-home.spec.ts:25`, `telegram_link.go:108`, `service.go:30` |
| 10, Zweigname | `claude/hintergrundjobs` | Der Zweig trägt den Namen seines Worktrees (`claude/vigorous-raman-651439`) | Kopf und Abschnitt 10 nennen ihn |

Keine Änderung an `docs/spec/`; Entscheid 15 steht in `04-entscheidungen.md`, ADR 0017 in
`docs/adr/`, ADR 0002 trägt den Hinweis.

### 13.4 Befunde aus Schritt 1 (30. Oktober 2026)

Schritt 1 ist nach Abschnitten 4.1 bis 4.5 und 6.1 gebaut (Migration `00027_jobs_scheduler.sql`,
`jobs.Type`, `jobs.Registry`, `jobs.Scheduler`, `RegisterTypes` in fünf Paketen, fünf Ticker-Schleifen
weg, `podcast.poll`). Wo der Code vom Papier abweicht, steht es hier; das Papier wird nicht
umgeschrieben:

| Stelle | Papier | Gebaut | Grund |
|---|---|---|---|
| 6.1, Struct `Type` | Felder bis `StartDelay` | Dazu **`Priority`** (Priorität des planmässigen Laufs; der Scheduler ruft `Run(ctx, q, t.Priority)`) und **`Seed func(ctx) (ScheduleSeed, error)`** (Takt, Schalter und letzter Lauf für die **erste** Zeile: Trakt aus `spotlight_config.interval_hours` und der Einstellung `trakt_last_attempt`, Update aus `update_check_enabled` und `update_state.checkedAt`) | Der Scheduler kennt sonst die Priorität nicht, und die Historie liegt je Jobart an einer anderen Stelle; die Namen `spotlight_trakt.last_attempt` und `update_check.checked_at` aus 4.3 gibt es so nicht |
| 4.3, `start_delay_ms` von `update.check` | «60 s nach dem Start wie heute, aber nur, wenn fällig» | **60 s nach jedem Start**, auch wenn nicht fällig: `StartDelay` heisst für alle Jobarten «beim Start nach dieser Verzögerung feuern» (`scan.library`, `library.sweep`, `podcast.poll` mit 0) | Ein bedingtes Startfeuer bräuchte ein zweites Feld; eine Manifestabfrage je Neustart kostet nichts und ist nach einem Update erwünscht. Die Ableitung aus `checkedAt` greift damit nur, wenn die Zeile erst entsteht und `StartDelay` nicht wirkt — also praktisch nur für Trakt |
| 6.1, `UpdateSchedule(type, every, window, enabled)` | vier Argumente | **`UpdateSchedule(ctx, type, ScheduleUpdate{Every, Enabled, Window, ClearWindow})`**: nur gesetzte Felder ändern sich (wie `updateJobSchedule` in 5.2 es braucht); Grenzen 5 min bis 7 d und Fenster 0 bis 1439, Anfang ≠ Ende, als `model.ValidationError` (`everySeconds`, `window`); unbekannte Jobart `model.ErrNotFound` | Spotlight-Seite schreibt nur den Takt, Update-Seite nur den Schalter |
| 6.1, Fehler in `Run` | nicht geregelt | Schlägt `Run` fehl, bleibt `next_run_at` stehen und der nächste Tick (60 s) versucht es erneut; `last_fired_at` wird nicht gesetzt | Ein vorübergehender DB-Fehler darf keinen Tageslauf kosten; Idempotenz-Schlüssel machen den Neuversuch harmlos |
| 6.1, Fenster | «nach jedem Feuern und nach jedem `UpdateSchedule`» | Zusätzlich beim **Startfeuer** (`StartDelay`): ein Scan mit Fenster 03:00–05:00 läuft nach einem Neustart um 12:00 nicht sofort, sondern um 03:00 | Jedes planmässige Feuern achtet das Fenster; nur «Jetzt ausführen» (Schritt 3) ignoriert es |
| 6.1, Takt des Schedulers | Tick alle 60 s | `Run` tickt **sofort** und dann alle 60 s; `Start` (Zeilen in `job_types`, `EnsureSchedule`, Startregel) läuft davor in `main.go`, bevor die Arbeiter starten | Damit `start_delay_ms = 0` wirklich «sofort» heisst |
| 4.3, `podcast.poll` von Hand | «Jetzt ausführen reiht **alle** Abos ein» | `Run` reiht die **fälligen** ein, mit der übergebenen Priorität; `QueueDuePodcasts` bleibt als Hülle für Tests | `runJobType` kommt erst in Schritt 3; dort entscheidet sich, ob `Run` den Auslöser erfährt (Hand = alle) oder `runJobType` für `podcast.poll` `RefreshLibrary` je Podcast-Bibliothek ruft. **Offen für Schritt 3** |
| 6.1, Test «ändert `next_run_at` nicht bei `runJobType`» (F-5) | in Schritt 1 | folgt mit `runJobType` in Schritt 3 | Es gibt noch keinen `runJobType`; der Scheduler-Test deckt ab, dass nur `Tick` `last_fired_at` und `next_run_at` schreibt |
| 4.5, eingefrorener Stand `testdata/db/` | «danach» | kommt mit Schritt 2 | 8.3 verlangt Zeilen mit `skipped`, Fortschritt und `started_at`, die erst Schritt 2 schreibt; `TestFrozenStatesMigrate` führt 00027 heute über alle 15 bestehenden Stände (Wächter greift nie, die Tabelle war immer leer) |
| 1.1, `scanbench` | nicht genannt | `cmd/slouchcat/scanbench.go` registriert über `RegisterTypes` und `Registry.Attach` | mechanische Folge |
| 3, `enqueueScan` | Prioritätsparameter | `enqueueScan(ctx, id, delay, prio)` hebt über `RaiseQueuedPriority`; ebenso `enqueuePodcast(ctx, id, prio)`, `enqueueTrakt` (gab es schon) und `runUpdateCheck`; `QueueScan` (Knopf «Scannen») und die Wächter reihen weiter mit 5 | wie 6.1; «Scannen» je Bibliothek bleibt ein Lauf mit 5 (6.4) |

Nicht geändert, weil der Code es nicht verlangt: die fünf Zustände, `ClaimJob` (Pause kommt in
Schritt 2), der Vertrag (1.4.0 bleibt), die Oberfläche.

### 13.5 Befunde aus Schritt 2 (30. Oktober 2026)

Schritt 2 ist nach Abschnitten 4.1, 4.4, 6.2 und 6.3 gebaut: `jobs.Progress`, `jobs.Skip`, Reporter
im Arbeiter, vier meldende Handler, Pause in `ClaimJob` und über die Laufliste, `Release` vor dem
`switch`, `Type.Cancel` mit `CancelScan` und `offline.CancelPreparation`, Start- und Endzeit je
Lauf, `jobs.prune`, Median, eingefrorener Stand `testdata/db/hintergrundjobs.sql`. Abweichungen:

| Stelle | Papier | Gebaut | Grund |
|---|---|---|---|
| 6.3, Ort von Pause, Fortsetzen und Abbruch | «Queue-Methoden» (Schritt 3), Laufliste «im Speicher der Registry» | **`Registry.Pause/Resume/CancelJob/CancelRuns/Running`**; `NewRegistry(q, log)` kennt die Queue, `Attach` hängt die Laufliste an den Arbeiter. `Queue` behält `Cancel` (roh), `PausedTypes`, `TypePaused`, `ListActive`, `MedianDuration`, `Prune` | Die Haken (`Type.Cancel`) liegen in der Registry; eine zweite Stelle mit Typwissen in der Queue wäre doppelt |
| 6.3, Abbruch eines laufenden Laufs | «der Handler merkt es beim nächsten Heartbeat» | Sofort über die Laufliste mit Ursache **`jobs.ErrCancelled`**; der Arbeiter schreibt dann nichts mehr (die Zeile ist schon `cancelled`), kein `Fail`, kein Versuch | Dieselbe Mechanik wie die Pause; der Heartbeat bleibt der Rückfall |
| 6.3, Download-Handler bei Abbruch | «keine Ursachenprüfung nötig» | Der Haken setzt `failed/cancelled_by_admin` **vor** dem Abbruch; der Handler setzt bei Kontextabbruch nur noch `queued`, wenn der Download noch `preparing` oder `paused` ist (`requeueInterrupted` statt `setState(queued)`) | Sonst hätte der unterbrochene Handler den Fachzustand des Hakens überschrieben und `Reconcile` den Download nach 10 min wieder eingereiht |
| 5.3, Fehlercode | `cancelled_by_admin` | `model.DownloadErrCancelled = "cancelled_by_admin"`; `DownloadState.error` ist im Vertrag freier Text (Beschreibung zählt Codes auf), also **kein Vertragsbruch**; die Beschreibung wird in Schritt 3 ergänzt | — |
| 4.1, `ClaimJob` | `started_at` je Versuch | Dazu löscht der Claim Fortschritt und `skipped` des vorigen Versuchs; `RetryFailedJob` setzt Start, Ende, Fortschritt und `skipped` zurück; `FailJob` setzt `finished_at` nur im Endzustand `failed` | Der Verlauf zeigt den letzten Stand des letzten Versuchs |
| 6.2, Notiz des Scans | «Ordner Filme/2024 · 1 240 von 3 980» | Notiz ohne Sprachtext: `<Bibliothek> · <Wurzelname>/<Ordner relativ zur Wurzel>`, die Zahlen stehen in `done`/`total` (Durchsuchphase: gezählte Ordner, `total = 0`) | Regel 9: kein Text im Server; der Client formatiert |
| 6.2, Notiz von `probe.keyframes` | Datei relativ zur Wurzel | `media_files.rel_path` | wie beschrieben |
| 4.4, `jobs.prune` | täglich, Priorität offen | Priorität **2**, `StartDelay` nil, registriert von `NewRegistry` selbst (Gruppe System); ein Lauf ohne gelöschte Zeile meldet `Skip` | Haushalt der Queue gehört zur Queue; Alter nach `finished_at`, für Zeilen vor 00027 nach `updated_at` |
| 6.2, Median | neueste 50 `done` mit `skipped = 0`, ab 5 Läufen | `Queue.MedianDuration(ctx, typ)` liefert Median, Anzahl und `ok` (ab `MedianMinRuns = 5`); die Schätzungen selbst (Restzeit, Wartezeit, Stau) kommen mit `listJobTypes` in Schritt 3 | — |
| 6.2, Fortschritt von `library.sweep` | keiner | keiner | wie beschrieben |
| 8.3, eingefrorener Stand | nach Schritt 2 | `testdata/db/hintergrundjobs.sql` aus `startseite.sql` plus 80 s Serverlauf mit Schritt 2 (18 Jobarten, 6 Zeitpläne, 196 erledigte Läufe mit Start/Ende, `jobs.prune` übersprungen, ein laufender `segments.detect` mit Fortschritt per SQL) | — |

Offen für Schritt 3: `JobCounts.paused` im Dashboard, `stopping` je Jobart (`Registry.Running(typ) > 0` bei
gesetzter Pause), die Schätzungen aus 6.2 und die Durchreichung von `ErrPaused`/`ErrCancelled` als 409.

### 13.6 Befunde aus Schritt 3 (30. Oktober 2026)

Schritt 3 ist nach Abschnitt 5 und 6.2 bis 6.4 gebaut: Vertrag **1.5.0** mit den sieben Operationen
unter `/admin/job-types/{type}` und `GET /admin/jobs`, `JobCounts.paused`, die Schemata `JobRun`,
`JobProgress`, `JobWindow`, `JobSchedule`, `JobScheduleUpdate`, `JobTypeInfo`, `JobPool`,
`JobOverview`, `JobRunPage`, `JobCancelResult`, `JobTypeName`; `task gen` für Go und TypeScript;
Paket `admin` (`jobtypes.go`: `ListJobTypes`, `ListJobRuns`, `RunJobType`, `PauseJobType`,
`ResumeJobType`, `UpdateJobSchedule`, `CancelJobRuns`, Gegenstand je Jobart, Schätzungen),
Handler in `api/jobtype_handlers.go`, drei neue Problemcodes, Vertragstest. Abweichungen:

| Stelle | Papier | Gebaut | Grund |
|---|---|---|---|
| 5.2, `JobScheduleUpdate.window` | `window?: {…} \| null` | `window?` plus **`clearWindow?: boolean`** | Die generierten Clients (oapi-codegen, openapi-typescript) unterscheiden `null` nicht von «fehlt»; ein Schalter ist eindeutig |
| 5.2, `listJobRuns` | `?type=&state=&limit=` | `type` und `state` **Pflicht**, `limit` 1 bis 200 (Vorgabe 50); `state` ist ein Enum mit `planned` | Ohne Jobart gäbe es keine Seite, die die Liste braucht |
| 5.3, `JobRun` «erweitert `JobSummary`» | `allOf` | Eigenes Schema mit denselben Feldern plus den neuen (`priority`, `estimate` und `skipped` Pflicht) | `allOf` erzeugt in oapi-codegen eingebettete Strukturen mit Fallstricken; die Felder sind dieselben |
| 5.2, `runJobType` für `trakt.refresh` und `podcast.poll` | «ruft dieselben Dienstmethoden» | Trakt über `RefreshSpotlight` (Client-Schlüssel, eine Minute Abstand, Priorität 7), zurückgegeben wird der aktive Lauf; `podcast.poll` über neues `metadata.QueueAllPodcasts(ctx, prio)` (**alle** Abos, 4.3); `update.check` prüft den Schalter und ruft sonst `Run` mit 7 (kein synchroner `CheckForUpdate`: die Operation antwortet mit Läufen) | wie 6.3, ohne die 13.4 offene Frage |
| 5.3, Grenzen von `trakt.refresh` | 6 bis 48 h | 6 bis 48 **ganze** Stunden (`interval_hours` ist ganzzahlig); `everySeconds` sonst 422 `everySeconds` | Spalte der Spotlight-Einstellung |
| 5.3, `JobTypeInfo` einer unbekannten Zeile | `known = false` | dazu `group = system`, `trigger = event`, `pool = general` als Platzhalter (die Felder sind Pflicht) | Schema bleibt einfach; der Client zeigt «unbekannt» |
| 6.2, Wartezeit | kumuliert nach Priorität über den Pool | wie beschrieben; Jobarten **ohne Median zählen 0** und eine Jobart ohne jeden Median im Pool bekommt **kein** `queuedEstimateSeconds` (der Client schreibt «ohne Schätzung») | wie 6.2 |
| 6.2, Restzeit «länger als üblich» | `estimate = median` ohne Zahl ab `elapsed > 2 · median` | so gebaut; der gemessene Fall verlangt `done ≥ total / 20` **und** `done > 0` | Division |
| 5.3, Gegenstand | je Jobart eine Funktion | ein Ableiter über die Payload-Schlüssel `libraryId`, `itemId`, `podcastId`, `seasonId`, `mediaFileId`, `deliveryId`, `downloadId`: Bibliotheksname; Titel, Staffel «Serie · S01», Folge «Serie · S01E02»; Kanal einer Zustellung (kein Kontoname, Regel 10); Titel eines Downloads; durch `RedactString` | ein Ort statt sieben |
| 10, Schritt 3, Fixtures | `web/src/test/fixtures.ts`, `web/e2e/fixtures.ts` | unverändert: die neuen Felder sind optional, die Seite kommt in Schritt 4 mit ihren Fixtures | kein Bedarf vor der Seite |
| 5.3, `DownloadState.error` | — | Beschreibung nennt jetzt `cancelled_by_admin` nicht; folgt mit der Seite (Schritt 4), wenn der Client den Code zeigt | kein Vertragsbruch (freier Text) |

Vertragstest: `TestContractCoversEveryOperation` kennt die sieben Operationen (Fixture mit
Registry und Scheduler); `JobCounts.paused` kommt aus `job_types`.

### 13.7 Befunde aus Schritt 4 (30. Oktober 2026)

Schritt 4 ist nach Abschnitt 7 gebaut: `web/src/features/admin/JobsPage.tsx` (Kopf mit fünf
Zahlen, Pools, Pausen-Hinweis mit «Fortsetzen», «Läuft gerade», Karten in vier Gruppen, Dialog
«Läufe von X» mit sechs Reitern), `JobScheduleDialog.tsx`, `jobsFormat.ts` (Dauern, Namen,
Zustand), Route `/admin/jobs` und `paths.adminJobs`, Eintrag in `sections.ts` vor
`integrations`, Dashboard-Kachel mit «Alle Jobs» und «Pausiert», Sprachdateien (`admin.jobs.*`,
`admin.sections.jobs`, `admin.home.jobs.{all,paused}`, drei Problemcodes, achtzehn
`admin.jobs.types.<typ>`), `JobsPage.test.tsx`, `web/e2e/admin-jobs.spec.ts`, `NAV_PAGES`.
Abweichungen:

| Stelle | Papier | Gebaut | Grund |
|---|---|---|---|
| 7, Navigation | Icon `activity`, «neu in `Icon.tsx`, falls keines passt» | Icon **`refresh`** (vorhanden) | Ein neues Icon änderte die Storybook-Baselines von `Icon`, die in dieser Sandbox nicht geschossen werden können (kein Docker); die Navigation zeigt ohnehin keine Symbole |
| 2.2, Kopf | Zahlen «als Chips» | fünf `StatTile` (Laufend, Wartend, Geplant, Pausiert, Fehlgeschlagen), wie das Dashboard | dieselbe Komponente wie die Kachel; Chips tragen auf den Karten den Zustand |
| 7, Tastaturtest | «Wartende abbrechen» öffnet die Bestätigung, Escape, Fokus zurück | im **Komponententest** (`JobsPage.test.tsx`: Escape behält, Bestätigen ruft `cancelJobRuns` mit `scope=queued`); der e2e-Test lässt es aus | Auf dem e2e-Server gibt es keinen wartenden Lauf, solange die Jobart pausiert ist (kein API-Weg reiht dann ein), und ohne Pause ist der Lauf in Sekunden weg |
| 7, Zeile «Läuft gerade» | Knopf «Pausieren» beschriftet «Lauf … pausieren» | sichtbar «Lauf pausieren», `aria-label` «<Name> pausieren (hält diesen und alle laufenden Läufe des Jobs an)»; der Komponententest prüft einen Aufruf mit der Jobart und «wird angehalten» an beiden Zeilen | wie 2.2 |
| 7, Pausen-Hinweis | Hinweis in Warnfarbe | `Notice` in `warning` mit einem «Fortsetzen»-Knopf je pausierter Jobart in der Hinweiszeile | der Weg zur Karte entfällt |
| 7, Zeitplan-Dialog | `Select` aus `ui` | `SelectField` (natives `<select>`, Pfeiltasten wählen) mit den Stufen 5 min bis wöchentlich innerhalb `limits`; ein gespeicherter Takt ausserhalb der Stufen bleibt als Eintrag erhalten; Fenster über zwei `TextField type="time"` | vorhandene Bausteine |
| 7, Texte | Gegenstand und Notiz «vom Server» | Notizen sind Rohtext des Servers (`Filme · Movies/2024`), der Client formatiert nur Zahlen und Dauern (`admin.jobs.duration.*`: Sekunden unter einer Minute, Minuten bis 90, dann Stunden, «über 1 Tag») | Regel 9, 12 |
| 5.3 | `DownloadState.error` | Beschreibung nennt `cancelled_by_admin` | kein Vertragsbruch |
| 7, Storybook | — | keine neue `ui`-Komponente, keine neuen Baselines | — |

Nicht gebaut (Abgrenzung): kein Pausieren je Lauf, keine Arbeiterzahl, keine Diagramme.


### 13.8 Befunde aus Schritt 5 (30. Oktober 2026)

Schritt 5 ist, soweit er ohne CT 190 geht, nach Abschnitt 10 gemacht: Messung in
`docs/perf/hintergrundjobs.md` (mit `docs/perf/seed-jobs.py`), Release-Notes-Entwurf 1.5.0 in
`docs/dev/CURRENT_STATUS.md`, Versionsstellen auf 1.5.0. Abweichungen und Befunde:

| Stelle | Papier | Gemacht | Grund |
|---|---|---|---|
| 10, Schritt 5, Ort der Messung | «Ziel < 50 ms p95 auf CT 190» | Auf dem Entwicklungsrechner mit 55'423 Zeilen: `listJobTypes` **23.6 ms p95** vor und nach dem ersten `prune` (p50 14.9 → 9.0 ms); mit dem Faktor 2 bis 3 aus `docs/perf/M3.md` 1c sind auf CT 190 47 bis 71 ms zu erwarten | CT 190 läuft noch auf 1.4.0; Marco stellt um, danach dieselben Aufrufe dort (Abschnitt 3 der Messung) |
| 12.4, «vor dem ersten `prune`» | auf CT 190 messbar | Nur mit eingespielten Zeilen (`seed-jobs.py`): `jobs.prune` feuert beim ersten Start, auf CT 190 ist der Zustand nur Sekunden zu sehen | Startregel des Schedulers (13.4) |
| 12.3, Vergleich Schätzung gegen Wirklichkeit | «liegt der Median-Fall um mehr als Faktor 2 daneben, verschwindet er» | Der **gemessene** Fall lag beim NAS-Scan um Faktor 60 daneben (2'036 s statt 34 s), weil die Rate mit der Zeit seit `started_at` rechnet und der Scan zwei Phasen ohne Fortschritt hat (Hash der neuen Dateien 150 von 186 s); der **Median-Fall** kippt auf 0 s, weil Rescans ohne Änderung als Arbeit zählen. Befunde B-1 bis B-4 in `docs/perf/hintergrundjobs.md` 2 mit Hebeln (Timer im Reporter, Fortschritt in `planRoot`, Rate ab der ersten Meldung mit `total > 0`, `Skip` bei leerem Schreibplan) | **Gebaut am 31. Oktober 2026 auf Marcos Wort** (Nachmessung in 2a der Messung: Faktor 60 → unter Faktor 2): Timer im Reporter, Fortschritt in `planRoot`, `Registry.ProgressSince` aus der Laufliste als Basis der Rate (kein Schema, dazu `measuredMinSeries` 10 s), `Skip` bei leerem Schreibplan; `jobs.Recorder`/`jobs.WithRecorder` als Testhilfe der Handler |
| 5.4, Release-Notes-Zeile | ein Satz | Der Satz steht als erste Zeile; dazu der volle Entwurf (Seite, Scheduler, Vertrag 1.5.0 additiv mit allen Operationen und Schemata, Migration 00027, Verlauf 30 Tage / 50) in `docs/dev/CURRENT_STATUS.md` | Marco taggt mit der Beschriftung (HANDOVER, Regel «jede Anhebung von `info.version` wird beim Merge getaggt») |
| 10.1, Versionsstellen | nicht genannt | `deploy/.env.example`, `deploy/compose.yaml`, `deploy/README.md`, `docs/INSTALLATION.md`, `docs/dev/HANDOVER.md` auf **1.5.0**, im PR wie bei 1.4.0 (PR #49) | HANDOVER verlangt es vor dem Tag; bis zum Tag zeigen die Adressen auf einen Stand, den es noch nicht gibt |
| 6.3, Pause | «hält die laufenden an» | Bestätigt: vier laufende `probe.keyframes` sofort zurück in die Warteschlange | — |
| 6.2, «gemessen» | «ab 5 % und 30 s» | dazu **mindestens 10 s der laufenden Reihe** (`measuredMinSeries`); eine Phase unter 10 s zeigt «wird berechnet» | Eine Rate aus zwei Sekunden Reihe wäre Rauschen (B-3) |
| 6.2, Median | «Läufe, die gearbeitet haben» | ein Scan ohne Änderung gilt als **ohne Arbeit** (`Skip`), obwohl er durchsucht und gehasht hat | Sonst kippt der Median auf 0 s (B-4); die Alternative des Papiers (Median-Fall streichen) träfe alle Jobarten |

Offen bleibt, was nur Marco kann: Image auf CT 190, Abnahme am Handy (Scan starten, pausieren,
fortsetzen, Wartende abbrechen, Restzeit, Zeitplan «täglich 03:00–05:00», Lauf im Verlauf am
Morgen), ein Tag Beobachtung der Zeitpläne; dann der Merge.
