# Konzept: Hintergrundjobs – eigene Verwaltungsseite mit Start, Pause und Abbruch

- **Status: Entwurf, Fassung 1 (29. Oktober 2026), zur Freigabe durch Marco.** Nichts davon ist
  umgesetzt. Die Fragen an Marco stehen in Abschnitt 11; die Umsetzung beginnt erst nach seiner
  Freigabe mit Schritt 0 aus Abschnitt 10. Ein klickbarer Prototyp der Seite liegt unter
  `docs/planning/mockups/09-hintergrundjobs-prototyp.html` (Abschnitt 7.1).
- **Datum: 29. Oktober 2026**
- **Wogegen dieses Konzept verankert ist:** `main`, Stand **`4049724`** («Merge pull request #45»,
  Bibliotheken), Vertragsfassung **1.3.0** (`api/openapi.yaml:4`), letzte Migration
  **`00025_items_filled_providers.sql`**, letzte ADR **`0015-bibliotheken-ordner-scraper.md`**.
  Konzept **08 Startseite** (`claude/konzept-startseite`, PR #46, freigegeben, nicht gestartet)
  hat **Frage 14, ADR 0016, Migration 00026 und Vertrag 1.4.0** reserviert. Dieses Papier nimmt
  darum **Frage 15, ADR 0017, Migration 00027 und Vertrag 1.5.0**. Die beiden Konzepte berühren
  sich nicht (08 baut Startseite und Seitenleiste, 09 baut Queue und eine Verwaltungsseite);
  wird 09 vor 08 umgesetzt, rücken die Nummern auf 14 / 0016 / 00026 / 1.4.0 vor, und 08 rückt nach.
  `:Zeile`-Angaben beziehen sich auf `4049724`; nach Namen suchen, nicht nach Zeile.
- Massgeblich bleiben `docs/spec/01-anforderungen-v1.md`, `docs/planning/04-entscheidungen.md` und
  `docs/adr/`. Bei Widerspruch gilt: **Entscheidungsprotokoll > Plan > Spec** (`CLAUDE.md`).
- **Jeder Schritt ist ein eigener Commit, und vor dem Abschluss jedes Schritts ist `task ci` grün**
  (`CLAUDE.md`, «Arbeitsweise»). DoD-Belege sind Befehl **und** Ausgabe.

> **Dieses Papier selbst ändert keine Zeile Code.** Die Arbeit beginnt mit Schritt 0 aus
> Abschnitt 10, einem reinen Doku-Commit.

> **Abgrenzung.** Hier geht es um das **Sehen und Steuern** der Hintergrundarbeit: welche Jobs es
> gibt, was gerade läuft, wie lange schon, wie lange noch, wann der letzte Lauf fertig wurde, wann
> der nächste kommt, und die Knöpfe Start, Pause, Abbruch. **Nicht** hierher gehören: neue
> Jobarten, einstellbare Zeitpläne (Abschnitt 11, F-4), Jobs je Zuschauer, eine Historie über
> Wochen oder Diagramme. Ein Zuschauer sieht von alledem nichts.

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
(`grep -rn "DELETE FROM jobs" server/` ist leer), die Tabelle wächst seit dem ersten Start.

**Die Jobarten.** Fünfzehn Typen sind registriert (`RegisterJobs` in fünf Paketen):

| Jobart | Paket | Priorität | Wer löst aus | Dauer je Lauf |
|---|---|---|---|---|
| `scan.library` | `library` | 5 (`prioScan`) | Zeitplan alle 6 h, Start des Servers, «Scannen» auf der Bibliotheksseite, Webhook | Minuten bis **Stunden** (erster Scan am NAS) |
| `scan.path` | `library` | `prioPathScan` | Dateiwächter (Debounce 2 s), Webhook | Sekunden |
| `probe.file` | `library` | 3 | Scan, je neue Datei | Sekunden |
| `probe.keyframes` | `library` | `prioKeyframesPlayback` / Scan | Wiedergabe, Scan | Sekunden bis Minuten |
| `library.sweep` | `library` | `prioSweep` | Zeitplan stündlich, Start des Servers | Sekunden bis Minuten |
| `segments.detect` | `library` | Scan | Scan von Serienordnern (Intro/Abspann) | Minuten; auf CT 190 standen 1683 davon an |
| `metadata.match` | `metadata` | 4 / 3 (Folgen) / 7 (manuell) | Scan, Zuordnung von Hand, Ordner ändern | Sekunden |
| `metadata.fill` | `metadata` | `fillPriority` | nach `metadata.match`, Listenänderung am Ordner | Sekunden |
| `image.fetch` | `metadata` | 2 / 1 (Stills) | nach Match, Nachholen beim Start | Sekunden |
| `trailer.find` | `metadata` | 1 | nach Match | Sekunden |
| `podcast.fetch` | `metadata` | 4 | Poller alle 5 min (fällig nach 55 min), «Aktualisieren», Abo | Sekunden |
| `trakt.refresh` | `metadata` | 2 / 7 (manuell) | Poller alle 15 min (fällig nach `interval_hours`), «Jetzt abrufen» | Sekunden bis Minuten |
| `update.check` | `admin` | 1 | Poller alle 15 min (fällig nach 24 h), «Jetzt prüfen» | Sekunden |
| `download.prepare` | `offline` | `jobPriority` 5 | Download eines Profils, Abgleich alle 10 min | Minuten bis Stunden |
| `notify.deliver` | `notify` | 3 | jedes Ereignis mit Kanal (E-Mail, Web-Push, Telegram) | Sekunden |

Drei Arbeiter-Pools laufen (`server/cmd/slouchcat/main.go:317` bis `:336`): `cfg.Workers`
allgemeine Arbeiter (Vorgabe `max(2, min(4, NumCPU))`, auf CT 190 also 2, daneben die vier
«Laufend» aus Marcos Dashboard-Bild stammen von einem Rechner mit vier), ein Arbeiter `download-1`
nur für `download.prepare` und ein Arbeiter `notify-1` nur für `notify.deliver`. Ein Arbeiter
nimmt immer den Job mit der höchsten Priorität, bei Gleichstand den ältesten.

**Die Zeitpläne liegen nicht in der Queue.** Die Tabelle `job_schedules` (`00001_jobs.sql:33`,
`type`, `cron_expr`, `next_run`) ist seit M0 angelegt und **leer**; ADR 0002 vertagte den
Scheduler auf «den ersten periodischen Job». Stattdessen entstanden fünf eigene Schleifen mit
`time.NewTicker`:

| Schleife | Wo | Takt | Was sie tut |
|---|---|---|---|
| Scan aller Bibliotheken | `library.Service.Start`, `service.go:758` | 6 h, dazu sofort beim Start | reiht `scan.library` je Bibliothek ein |
| Sweep | ebenda | 1 h, sofort beim Start | reiht `library.sweep` ein |
| Podcast-Poller | `metadata.RunPodcasts`, `podcast.go:348` | prüft alle 5 min | reiht `podcast.fetch` für Abos ein, deren letzter Abruf 55 min alt ist |
| Trakt-Poller | `metadata.RunTrakt`, `trakt.go:122` | prüft alle 15 min | reiht `trakt.refresh` ein, wenn der letzte Versuch `spotlight_config.interval_hours` alt ist |
| Update-Poller | `admin.RunUpdateCheck`, `update.go:85` | prüft alle 15 min, 60 s nach dem Start | reiht `update.check` ein, wenn die letzte Prüfung 24 h alt ist und die Prüfung eingeschaltet |

Der Kommentar in `service.go:30` sagt, warum: «two fixed intervals do not need a cron scheduler».
Inzwischen sind es fünf Schleifen in drei Paketen, jede mit eigener «fällig»-Logik, und **keine
weiss, wann der nächste Lauf ist** — die drei Poller rechnen es bei jedem Tick neu aus. Dazu gibt
es zwei Schleifen, die keine Jobs einreihen und hier nicht gemeint sind: der Download-Abgleich
`offline.Run` (alle 10 min, `cleanup.go:24`) und der Telegram-Link-Poller
(`telegram_link.go:108`); sie bleiben, wie sie sind.

**Das Dashboard** (`getDashboard`, A-3; `web/src/features/admin/AdminHomePage.tsx:507`) zeigt die
Kachel aus Marcos Bild: drei Zahlen (`JobCounts.queued`, `running`, `failed`), darunter die
letzten zehn fehlgeschlagenen Jobs mit «Erneut» (`retryJob`) und «Abbrechen» (`cancelJob`),
alle 5 s abgefragt (`DASHBOARD_POLL_MS`). Mehr weiss die Oberfläche nicht: **nicht welche** vier
Jobs laufen, **nicht seit wann**, **nicht wie lange noch**, **nicht** was die 236 Wartenden sind.
Einzig der Bibliotheksscan hat einen Fortschritt (`libraries.scan_files_seen` / `scan_files_total`,
`LibraryScan.filesSeen` / `filesTotal`) und einen eigenen Abbruch (`cancelLibraryScan`,
`api/openapi.yaml:2328`), beides auf der Bibliotheksseite.

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
  pausieren und wovon man die Wartenden abbrechen kann; sechzehn Stück (die fünfzehn von heute
  plus `jobs.prune`, Abschnitt 4.4). Ein Job hat einen Zustand (**läuft**, **wartet**, **ruht**,
  **pausiert**), einen Auslöser (Zeitplan, Ereignis oder Hand), gegebenenfalls einen Zeitplan,
  einen letzten und einen nächsten Lauf.
- **Lauf** — eine Zeile in `jobs`: ein Scan der Bibliothek «Filme», eine Zustellung an Marcos
  Telegram, ein Match von Titel 4711. Ein Lauf hat Start, Dauer, Fortschritt, Restzeit, Ergebnis.

Die Knöpfe **Start** und **Pause** gehören zum **Job**, der Knopf **Abbrechen** zum **Lauf** (und
als Sammelknopf «Wartende abbrechen» zum Job). Begründung in Abschnitt 6.3 und Frage F-2: ein
einzelner `metadata.match` lebt zwei Sekunden, ihn zu pausieren ist sinnlos; was Marco anhalten
will, ist «die 236 Wartenden sollen jetzt nicht weiterlaufen», und das ist eine Aussage über die
Jobart.

### 2.2 Die Seite «Hintergrundjobs»

Neue Verwaltungsseite unter `/admin/jobs`, in der Navigation in der Gruppe **System** vor
«Integrationen», auf der Übersicht als Karte wie jede andere Seite (`adminSections`). Von oben
nach unten:

1. **Kopf.** Titel «Hintergrundjobs», eine Zeile Zahlen als Chips: *Laufend 4 · Wartend 236 ·
   Pausiert 0 · Fehlgeschlagen 0*, dazu «Arbeiter: 4 allgemein, 1 Downloads, 1 Benachrichtigungen»
   — damit klar ist, warum nie mehr als vier allgemeine Läufe gleichzeitig laufen. Ist ein Job
   pausiert, steht hier ein Hinweis in Warnfarbe («2 Jobs pausiert — neue Dateien werden nicht
   erfasst»), denn eine vergessene Pause ist der gefährlichste Zustand dieses Konzepts (9.4).
2. **Läuft gerade.** Eine Liste der laufenden Läufe über alle Jobs: Jobname, worum es geht
   (Fortschrittsnotiz: «Filme · /media/nas/Filme/…», «Telegram an Marco», «Titel 4711»), **seit**
   (Dauer, laufend hochgezählt), ein Fortschrittsbalken, wo der Lauf einen meldet, **Restzeit**
   («noch ca. 12 min», «wird berechnet», «länger als üblich»), und je Zeile **Abbrechen**. Leer:
   «Gerade läuft nichts.»
3. **Jobs.** Eine Karte je Jobart, in vier Gruppen (*Bibliothek*, *Metadaten*, *Zustellung*,
   *System*). Je Karte: Name auf Deutsch und der Code in Schreibmaschinenschrift
   («Bibliotheksscan · `scan.library`»), ein Zustandschip, dann vier Zeilen:
   - **Wartend** — Zahl und Schätzung («236 · ca. 40 min»), aus Median-Dauer und Pool-Grösse (6.2);
   - **Letzter Lauf** — «vor 12 min · 3 s · OK» oder «vor 2 h · 48 min · Fehler: …»;
   - **Nächster Lauf** — «in 4 h 12 min» bei Zeitplan, «bei Bedarf» bei Ereignis, «pausiert»;
   - **Ausgelöst durch** — eine Zeile Text aus der Sprachdatei («Zeitplan alle 6 h und Start des
     Servers», «vom Scan, je neue Datei», «jedes Ereignis mit Kanal»).

   Und die Knöpfe: **Jetzt ausführen** (nur bei Zeitplan- und Hand-Jobs, F-3), **Pausieren** /
   **Fortsetzen**, **Wartende abbrechen (236)** mit Bestätigung. Ein Klick auf die Karte öffnet
   die **Läufe** des Jobs: Reiter *Laufend · Wartend · Fehlgeschlagen · Erledigt*, je Zeile
   Start, Dauer, Versuche, Fehler, **Erneut** bei fehlgeschlagenen, **Abbrechen** bei wartenden
   und laufenden.
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
   sofort aus (4.3, 6.1).
2. **Fortschritt je Lauf**: ein Handler meldet `done`, `total` und eine Notiz; der Arbeiter schreibt
   sie gedrosselt in die Zeile (4.1, 6.2).
3. **Start- und Endzeit je Lauf** und ein **Verlauf** mit Aufräumjob, aus dem die Median-Dauer je
   Jobart kommt — die Grundlage jeder Schätzung (4.1, 4.4, 6.2).
4. **Pause je Jobart**, dauerhaft (überlebt Neustart, F-6): keine neuen Claims, laufende Läufe
   halten beim nächsten Heartbeat an und gehen ohne verbrauchten Versuch in die Queue zurück
   (4.2, 6.3).
5. **Sieben neue Operationen** im Vertrag, alle `x-visibility: admin`, nicht brechend (5).

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
| ADR 0002, «Scheduler … gehört zum Meilenstein des ersten periodischen Jobs» | nie eingelöst | **Eingelöst** durch ADR 0017 als Teilnachfolger (Scheduler, Pause, Fortschritt, Verlauf); Queue-Semantik (Lease, Idempotenz, Backoff, Priorität) bleibt wörtlich |
| ADR 0002, Zustände `queued … cancelled` | fünf Zustände | **Unverändert.** «Pausiert» ist ein Merkmal der Jobart (`job_types.paused_at`), kein sechster Zustand; ein angehaltener Lauf ist wieder `queued` (6.3) |
| `library/service.go:30` «two fixed intervals do not need a cron scheduler» | Begründung für die Ticker | Hinfällig: die Intervalle wandern in `job_schedules`, der Kommentar fällt |
| Entscheid vom 26. Oktober 2026 (M11, `RaiseQueuedPriority`, manuelle Handlungen mit Priorität 7) | gilt für «Jetzt abrufen» und Match | **Gilt auch für «Jetzt ausführen»**: ein Hand-Lauf reiht mit 7 ein und hebt einen wartenden Zeitplan-Lauf gleichen Schlüssels an |
| CLAUDE.md Regel 11 (nichts vorziehen) | — | Kein Nicht-Ziel berührt. Der nächtliche `rating.verify` aus Plan 3.2 ist bis heute **kein Job** (die Prüfung lebt als Pflege-Befund `rating_mismatch`, `admin/maintenance.go:155`); dieses Papier macht ihn **nicht** zum Job |
| CLAUDE.md Regel 12 (Entscheid im Server) | — | Schätzungen (Restzeit, Wartezeit) rechnet der **Server** (`estimatedSecondsLeft`); der Client zählt nur Sekunden hoch |

---

## 4. Datenmodell

### 4.1 `jobs` lernt Start, Ende und Fortschritt

Fünf Spalten, alle NULL-fähig, keine bestehende Zeile ändert sich:

| Spalte | Typ | Gesetzt von | Wozu |
|---|---|---|---|
| `started_at` | INTEGER (ms) | `ClaimJob` (jeder Versuch neu) | «läuft seit», Dauer |
| `finished_at` | INTEGER (ms) | `CompleteJob`, `FailJob` (Endzustand), `CancelJob` | Dauer, «letzter Lauf fertig vor», Median |
| `progress_done` | INTEGER | Handler über `jobs.Progress` | Balken, Restzeit |
| `progress_total` | INTEGER | ebenso | Balken, Restzeit |
| `progress_note` | TEXT (≤ 200 Zeichen) | ebenso | «woran gerade» («Filme · 1 240 von 3 980») |

`ReleaseJob` (Herunterfahren, Pause) löscht `started_at` und den Fortschritt nicht — der nächste
Versuch überschreibt sie beim Claim; der Verlauf zeigt so den letzten Stand. Die Dauer eines Laufs
ist `finished_at − started_at` des **letzten** Versuchs, nicht die Summe aller Versuche: Backoff-
Wartezeit ist keine Arbeit. Ein neuer Index `jobs_history (type, state, finished_at DESC)` trägt
die Median-Abfrage und das Aufräumen; `jobs_claim` bleibt.

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
    type          TEXT    PRIMARY KEY REFERENCES job_types(type),
    every_ms      INTEGER NOT NULL CHECK (every_ms >= 60000),
    run_at_start  INTEGER NOT NULL DEFAULT 0 CHECK (run_at_start IN (0, 1)),
    enabled       INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    next_run_at   INTEGER NOT NULL,
    last_fired_at INTEGER,
    updated_at    INTEGER NOT NULL
);
```

`cron_expr` fällt weg (3). Die Zeilen schreibt **nicht** die Migration, sondern die Registry beim
Start (`EnsureSchedule`): Intervall und `run_at_start` sind Code-Konstanten (`fullScanInterval`,
`sweepInterval`, …) oder Einstellungen (`spotlight_config.interval_hours`,
`update_check.enabled`); ändert der Administrator eine Einstellung, schreibt der betroffene
Dienst die Zeile um (`UpdateSchedule`). `next_run_at` bleibt beim Start erhalten, wenn die Zeile
schon da ist, damit ein Neustart den Takt nicht verschiebt — ausser bei `run_at_start = 1`, wo
der Scheduler sofort feuert, wie `library.Start` es heute tut.

Die sechs Zeilen nach Schritt 1:

| `type` | `every_ms` | `run_at_start` | `enabled` | Herkunft |
|---|---|---|---|---|
| `scan.library` | 6 h | 1 | 1 | `fullScanInterval` |
| `library.sweep` | 1 h | 1 | 1 | `sweepInterval` |
| `podcast.fetch` | 5 min | 1 | 1 | `feedPollInterval`; der Lauf reiht die **fälligen** Abos ein (55 min, `FeedInterval`), der Takt ist die Prüfung |
| `trakt.refresh` | `interval_hours` | 1 | `spotlight_config.enabled` | der Lauf prüft nichts mehr: der Zeitplan **ist** die Fälligkeit |
| `update.check` | 24 h | 0 (60 s nach Start, wie heute) | `update_check.enabled` | `UpdateInterval` |
| `jobs.prune` | 24 h | 0 | 1 | neu, 4.4 |

Die drei Poller mit «fällig, wenn der letzte älter als X» werden damit zu echten Zeitplänen. Das
ändert eine Kleinigkeit: heute läuft Trakt nach einem Neustart sofort, wenn der letzte Lauf alt
genug war; neu feuert er beim Start immer (`run_at_start`), und sein Handler ist idempotent (die
Rate-Limits von Trakt sind durch den Limiter gedeckt, `trakt.go:51`). Podcast-Fälligkeit je Abo
(55 min) bleibt im Handler, weil sie je Abo gilt, nicht je Jobart.

### 4.4 Verlauf und Aufräumen: `jobs.prune`

Heute bleibt jede `done`-Zeile für immer. Für die Median-Dauer braucht der Server einen Verlauf,
aber einen **begrenzten**: `jobs.prune` läuft täglich und löscht `done`- und `cancelled`-Läufe,
die älter als **7 Tage** sind, behält aber je Jobart die **neuesten 50** (F-8). `failed` bleibt
wie heute liegen, bis der Administrator «Erneut» oder «Abbrechen» drückt — ein Fehler, den
niemand gesehen hat, darf nicht von selbst verschwinden. `jobs.prune` ist selbst ein Job der
Gruppe *System* und erscheint auf der Seite; sein erster Lauf nach der Migration räumt den Bestand
auf (auf CT 190 nach Wochen Betrieb: Zehntausende Zeilen; Batches à 500, Regel aus der
Paketdoku).

### 4.5 Migrationsplan 00027 (Schritte, kein fertiges SQL)

Nach `server/migrations/TEMPLATE.sql.txt`: `NO TRANSACTION`, `PRAGMA foreign_keys=OFF`, dann:

1. `ALTER TABLE jobs ADD COLUMN` × 5 (4.1); `CREATE INDEX jobs_history`.
2. `CREATE TABLE job_types` (4.2); `INSERT` einer Zeile je Typ, der in `jobs` vorkommt
   (`INSERT OR IGNORE … SELECT DISTINCT type FROM jobs`), damit `job_schedules` seinen
   Fremdschlüssel findet, bevor die Registry läuft.
3. `DROP TABLE job_schedules; CREATE TABLE job_schedules` (4.3). Kein Datenverlust: die Tabelle
   war immer leer (Wächter: `SELECT RAISE(ABORT, …)` über eine `CHECK`-Tabelle, falls doch eine
   Zeile da ist, wie 00023 es vormacht).
4. `PRAGMA foreign_key_check`, `PRAGMA foreign_keys=ON`.

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
| `listJobTypes` | `GET /admin/job-types` | `JobOverview` | die Seite: Arbeiter, Zahlen, eine `JobTypeInfo` je Jobart samt laufenden Läufen und letztem Lauf — **eine** Abfrage alle 5 s wie `getDashboard` |
| `listJobRuns` | `GET /admin/jobs?type=&state=&limit=` | `JobRun[]` | die Läufe einer Jobart nach Zustand, neueste zuerst, höchstens 200 |
| `runJobType` | `POST /admin/job-types/{type}/run` | 202 `JobRun[]` | «Jetzt ausführen»: ruft die `Run`-Funktion der Registry mit Priorität 7; gibt die eingereihten Läufe zurück (ein Scan je Bibliothek). 409 `job_type_not_runnable` bei Ereignis-Jobs, 409 `job_type_paused` bei pausierten |
| `pauseJobType` | `POST /admin/job-types/{type}/pause` | 200 `JobTypeInfo` | setzt `paused_at`; idempotent |
| `resumeJobType` | `POST /admin/job-types/{type}/resume` | 200 `JobTypeInfo` | löscht `paused_at`; idempotent |
| `cancelQueuedRuns` | `DELETE /admin/job-types/{type}/queued` | 200 `{ cancelled: n }` | «Wartende abbrechen»: alle `queued` dieser Jobart; laufende bleiben |
| `retryJob`, `cancelJob` | wie heute | wie heute | Lauf-Ebene |

Alle neu: `x-visibility: admin`, Tag `admin`, 401/403 wie die Nachbarn, 404 `not_found` bei
unbekannter Jobart; jede Operation bekommt ihren Eintrag im Vertragstest
(`TestContractCoversEveryOperation`, Regel 7). Keine Antwort enthält einen `ItemRef`: ein Lauf
nennt einen Titel nur als Notiz-Text und Id, wie `JobSummary.lastError` es heute hält.

### 5.3 Schemata

- `JobRun` **erweitert `JobSummary`** (gleiche Pflichtfelder, dazu optional `startedAt`,
  `finishedAt`, `durationSeconds`, `progress { done, total, note }`, `estimatedSecondsLeft`,
  `estimate` = `measured | median | none`). `JobSummary` selbst bleibt, damit `getDashboard`
  unverändert gilt.
- `JobTypeInfo`: `type`, `group` (`library | metadata | delivery | system`), `trigger`
  (`schedule | event | manual`), `pool` (`general | download | notify`), `paused`, `pausedAt?`,
  `runnable` (bool), `schedule? { everySeconds, nextRunAt, lastFiredAt, enabled }`,
  `counts { queued, running, failed }`, `running: JobRun[]` (≤ 10), `lastRun?: JobRun`,
  `medianDurationSeconds?`, `queuedEstimateSeconds?`.
- `JobOverview`: `workers { general, download, notify }`, `paused` (Zahl), `types: JobTypeInfo[]`.
- `JobCounts` (Dashboard) bekommt optional `paused` (Zahl) — additiv.
- Problemcodes neu: `job_type_not_runnable`, `job_type_paused`.

Texte der Jobarten (Name, Auslöser-Zeile) kommen **nicht** aus dem Vertrag, sondern aus
`web/src/locales/{de,en}.json` unter `admin.jobs.types.<type>` (Regel 9); eine Jobart ohne
Schlüssel zeigt ihren Code. Das Schema erzwingt kein Enum über `type`, damit eine neue Jobart
keinen Vertragsbruch auslöst.

### 5.4 Ist das eine brechende Anhebung?

Nein. Nur neue Operationen, neue optionale Felder, ein neues Schema als Obermenge. **1.5.0**
(bzw. 1.4.0, siehe Kopf). Der Eintrag für die Release-Notes (C-5): «Neue Verwaltungsseite
Hintergrundjobs; Vertrag 1.5.0 fügt `/admin/job-types` hinzu, nichts entfällt.»

---

## 6. Server

### 6.1 Registry und Scheduler im Paket `jobs`

Heute registriert jeder Dienst seine Handler mit `w.Handle(typ, fn)`. Neu registriert er
**die Jobart** mit ihren Eigenschaften, und der Handler ist eines davon:

```go
// jobs.Type describes one job type for the worker, the scheduler and the admin page.
type Type struct {
	Name    string          // "scan.library"
	Group   Group           // library, metadata, delivery, system
	Pool    Pool            // general, download, notify
	Trigger Trigger         // schedule, event, manual
	Handler Handler         // runs one job (as today)
	Run     func(ctx context.Context, q *Queue, prio int) ([]int64, error) // enqueues what a run means; nil for event types
	Every   time.Duration   // > 0 for schedule types
	AtStart bool            // fire at server start
}
```

`Registry.Register(t Type)` legt die Zeile in `job_types` an, bei `Every > 0` den Zeitplan
(4.3), und übergibt den Handler an den Arbeiter des Pools. `RegisterJobs(w)` in den fünf Paketen
wird zu `RegisterTypes(r *jobs.Registry)`; der Umbau ist mechanisch, die Handler bleiben.

Der **Scheduler** ist eine Goroutine in `jobs` (eine je Server, wie die Arbeiter in `main.go`),
Tick alle 60 s: `SELECT … FROM job_schedules WHERE enabled = 1 AND next_run_at <= ?`, je Treffer
`Run(ctx, q, prio)` mit der normalen Priorität der Jobart, dann `next_run_at = now + every_ms`,
`last_fired_at = now`. Verpasste Läufe (Server war aus) holen **nicht** nach: ein Takt, nicht ein
Kalender — genau das Verhalten der heutigen Poller. Die fünf Ticker-Schleifen (1.1) fallen;
`library.Start` behält nur die Dateiwächter. Tests mit injizierter Uhr wie `queue_test.go`
heute: «feuert beim Start», «feuert nach `every`», «feuert nicht, wenn `enabled = 0`»,
«ändert `next_run_at` nicht bei `runJobType`» (F-5).

### 6.2 Fortschritt und Schätzung

**Melden.** Der Arbeiter legt einen Reporter in den Kontext des Handlers; ein Handler ruft
`jobs.Progress(ctx, done, total, note)` so oft er will. Der Reporter schreibt höchstens **alle
2 s** und zusätzlich mit jedem Heartbeat in die Zeile (`UpdateJobProgress`, nur unter eigener
Lease). Drei Handler melden in Schritt 2: `scan.library` je Batch (`seen`/`total` hat er schon,
`write.go:249`; die Notiz ist Bibliothek und laufender Ordner, relativ zur Wurzel),
`library.sweep` je Batch (geprüfte von gesamt), `download.prepare` in Bytes (vorbereitet von
gesamt; der Dienst kennt beides). Alle anderen bleiben ohne Fortschritt — ihre Läufe dauern
Sekunden, da wäre ein Balken Theater.

**Restzeit je Lauf** (`estimatedSecondsLeft`, vom Server gerechnet):

| Fall | Rechnung | `estimate` |
|---|---|---|
| Fortschritt gemeldet, `done ≥ 5 %` von `total` **und** Dauer ≥ 30 s | `elapsed · (total − done) / done` | `measured` |
| kein Fortschritt, Median der Jobart bekannt (≥ 5 erledigte Läufe im Verlauf) | `max(0, median − elapsed)`; ist `elapsed > 2 · median`: `null` mit `estimate = median` (Client: «länger als üblich») | `median` |
| sonst | `null` | `none` (Client: «wird berechnet») |

Der Median kommt aus `finished_at − started_at` der neuesten 50 `done`-Läufe der Jobart (4.4);
Median statt Mittelwert, weil ein erster NAS-Scan von sechs Stunden sonst jeden Zwei-Minuten-Scan
auf Jahre verschätzt. **Wartezeit je Jobart** (`queuedEstimateSeconds`): `queued · median /
poolSize`, wobei `poolSize` die Arbeiterzahl des Pools ist; ohne Median `null`. Beides ist eine
Schätzung und wird so benannt («ca.»); der Client rundet auf Minuten, ab 90 min auf Stunden, ab
einem Tag «über 1 Tag».

**Was die Schätzung nicht kann:** sie sieht den Stau eines Netzlaufwerks nicht (CT 190, 45 MB/s,
`slouchcat-ct190-stand`). Darum «länger als üblich» statt einer falschen Zahl, und darum zählt
der Client die laufende Dauer immer mit — sie ist das einzige, was sicher stimmt.

### 6.3 Pause, Start, Abbruch

**Pause** (F-1). `pauseJobType` setzt `job_types.paused_at`. Wirkung in zwei Schritten:

1. **Keine neuen Claims.** `ClaimJob` bekommt `AND type NOT IN (SELECT type FROM job_types WHERE
   paused_at IS NOT NULL)`; der Index `jobs_claim` trägt das weiter, die Unterabfrage liest
   höchstens sechzehn Zeilen.
2. **Laufende Läufe halten beim nächsten Heartbeat an.** Der Heartbeat (alle 60 s) liest den
   Pausenstand mit; ist die Jobart pausiert, bricht der Arbeiter den Kontext des Handlers mit
   `context.WithCancelCause(…, jobs.ErrPaused)` ab und gibt den Lauf mit `Release` zurück — in
   die Queue, **ohne** verbrauchten Versuch, wie beim Herunterfahren (`worker.go:124`). Was in
   unter einer Minute endet, endet also noch; ein Scan hält spätestens nach 60 s an.

Handler, die eine Kontextabbruch heute als «abgebrochen» persistieren, prüfen die Ursache: der
Scan schreibt bei `context.Cause(ctx) == jobs.ErrPaused` **nicht** `aborted/cancelled` in
`libraries.last_scan_state`, sondern lässt `queued` stehen (`scanner.go:43` bis `:54` und
`jobs.go:215`); der Download lässt den Stand `preparing`. Beim **Fortsetzen** nimmt der nächste
Claim den Lauf wieder auf; der Scan beginnt den Ordnerlauf von vorn, überspringt aber alles
Erledigte (er ist idempotent, Paketdoku `library/doc.go:20`), der Download setzt an seinen
gesicherten Segmenten fort. Pause und Fortsetzen werden protokolliert (`slog`, Info, mit
Jobart; keine Nutzdaten).

**Start** («Jetzt ausführen»). Nur Jobarten mit `Run ≠ nil` (Zeitplan- und Hand-Jobs, F-3);
der Server ruft `Run` mit **Priorität 7** (Entscheid vom 26. Oktober 2026) und gibt die Läufe
zurück. Ein Hand-Lauf verschiebt `next_run_at` **nicht** (F-5): der Takt bleibt, wie er war.
Ist die Jobart pausiert, antwortet der Server 409 `job_type_paused`; der Client bietet den Knopf
dann gar nicht an (Regel 12: der 409 ist der Riegel, die Oberfläche spart nur den Klick).

**Abbruch.** Je Lauf wie heute (`cancelJob`); neu je Jobart `cancelQueuedRuns`, das nur
`queued` trifft — ein laufender Lauf wird nie «mit abgebrochen», dafür ist sein eigener Knopf da.
Beide mit Bestätigung in der Oberfläche; bei `notify.deliver` mit Warntext, dass die Mitteilung
dann nicht zugestellt wird (9.3). Ein abgebrochener Scan bleibt, was er heute ist: `aborted` mit
`cancelled`, der nächste Zeitplan-Lauf konvergiert.

### 6.4 Was mit den heutigen Pfaden geschieht

| Heute | Neu |
|---|---|
| `cancelLibraryScan` auf der Bibliotheksseite | bleibt; ruft intern dasselbe `Cancel` |
| «Scannen» je Bibliothek | bleibt (ein Lauf); «Jetzt ausführen» bei `scan.library` reiht **alle** Bibliotheken ein |
| «Jetzt abrufen» (Spotlight), «Jetzt prüfen» (Update), «Aktualisieren» (Podcasts) | bleiben; sind dasselbe wie `runJobType` mit Priorität 7 und werden in Schritt 3 darauf umgestellt, damit es **einen** Weg gibt |
| `getDashboard.jobs` | bleibt, plus `paused` |
| `notify.JobFailed` (Meldung nach letztem Versuch) | bleibt |

---

## 7. Oberfläche

Grundsatz wie in jedem Konzept dieses Projekts: der Client prüft keine Regel (Regel 12). Er
blendet Knöpfe aus, die der Server mit 409 beantworten würde (`runnable`, `paused`), mehr nicht.

| Seite / Datei | Was geschieht |
|---|---|
| **Neue Seite** `web/src/features/admin/JobsPage.tsx`, Route `/admin/jobs` (`router.tsx`, `paths.adminJobs`) | Kopf mit Chips und Arbeitern, «Läuft gerade» als Liste (`ui` `Panel`, `ProgressBar` neu in `web/src/ui`, falls es keinen gibt — Storybook-Baseline wie `ProviderList.stories.tsx`), Karten je Jobart in vier Gruppen (`StatTile` für die Zahlen, `StatusDot` für den Zustand, Chips aus `ui`), Dialog «Läufe von X» mit `Tabs` (wie `MaintenancePage`) und Tabelle wie die Fehlerliste des Dashboards (`AdminTablePage.module.css`) |
| **Navigation** `sections.ts` | neuer Eintrag `{ key: 'jobs', href: paths.adminJobs, icon: 'activity', group: 'system' }` **vor** `integrations`; Icon `activity` neu in `web/src/ui/Icon.tsx`, falls keines passt |
| **Dashboard** `AdminHomePage.tsx:507` | Kachel bekommt den Link «Alle Jobs» und, bei `paused > 0`, eine vierte `StatTile` «Pausiert» in `warning` |
| **Abfragen** `features/admin/queries.ts` | `jobTypesQuery` (`refetchInterval: JOBS_POLL_MS = 5000`, aus, solange ein Bestätigungsdialog offen ist, wie `AdminHomePage.tsx:343`), `jobRunsQuery(type, state)`; Mutationen `runJobType`, `pauseJobType`, `resumeJobType`, `cancelQueuedRuns`, dazu die bestehenden `retryJob`, `cancelJob` |
| **Laufende Dauer** | `useElapsed(startedAt)` zählt sekündlich hoch, mit `prefers-reduced-motion` ohne Animation des Balkens; der Text im Balken ist die Wahrheit, der Balken ist `aria-hidden` |
| **Bibliotheken, Spotlight, Integrationen** | unverändert in der Bedienung; Schritt 3 stellt ihre «Jetzt …»-Knöpfe auf `runJobType` um, Texte bleiben |

**Regel 8 — Tastaturtest.** Neue Route, also `web/e2e/admin-jobs.spec.ts`: Seite über die
Verwaltungsnavigation erreichen (Tab, Pfeiltasten, Enter), «Läuft gerade» und die erste Karte per
Tab, «Jetzt ausführen» bei `library.sweep` per Enter (sichtbare Rückmeldung «Lauf eingereiht», die
Zeile erscheint unter «Läuft gerade» oder «Erledigt»), «Pausieren» per Enter und Zustandschip
«Pausiert», «Fortsetzen», «Wartende abbrechen» öffnet die Bestätigung (Tab in die Bestätigung,
Escape schliesst, Fokus zurück auf dem Knopf), Dialog «Läufe von X» (Reiter per Pfeiltaste,
Escape, Zurück, keine Fokusfalle), sichtbarer Fokus überall. Der Test nutzt `library.sweep` und
`jobs.prune`, weil beide am e2e-Server in Sekunden enden und nichts Fremdes anstossen.
`admin-home.spec.ts:25` (`NAV_PAGES`) bekommt die Zeile `['Hintergrundjobs', '/admin/jobs']`;
`admin-home.spec.ts` prüft zusätzlich den Link «Alle Jobs».

**Regel 9 — Texte.** Neue Schlüssel in `de.json` und `en.json`: `admin.sections.jobs.{title,
description}`, `admin.jobs.{title,intro,workers,running,runningEmpty,queued,queuedEstimate,
lastRun,nextRun,trigger,onDemand,paused,pausedBanner,run,pause,resume,cancelQueued,
cancelQueuedTitle,cancelQueuedBody,runs,columns.*,estimate.{computing,longer,about},
state.{idle,queued,running,paused},groups.{library,metadata,delivery,system}}`,
`admin.jobs.types.<type>.{name,trigger}` für sechzehn Typen, die Problemcodes
`job_type_not_runnable`, `job_type_paused`, und `admin.home.jobs.{all,paused}`.
`pnpm -C web test locales` prüft die Symmetrie. Farben nur aus Tokens: der Balken nutzt
`color.state.track` für den leeren Teil und die Akzentfarbe für den vollen, der Pausen-Hinweis
`color.state.warning`.

### 7.1 Klickbarer Prototyp

`docs/planning/mockups/09-hintergrundjobs-prototyp.html` ist eine einzelne HTML-Datei ohne
Abhängigkeiten (Doppelklick genügt), gebaut wie die Prototypen zu 07 und 08: die Seite
«Hintergrundjobs» mit Marcos Zahlen vom 29. Oktober 2026 (236 wartend, 4 laufend, 0
fehlgeschlagen), «Läuft gerade» mit vier Läufen (ein Scan mit Fortschritt und Restzeit, zwei
Segmenterkennungen mit Median-Schätzung, eine Zustellung ohne), sechzehn Jobkarten in vier
Gruppen, der Dialog «Läufe von X» mit Reitern und die drei Knöpfe mit Wirkung (Pausieren hält die
Karte an und zeigt den Hinweis im Kopf, Jetzt ausführen reiht einen Lauf ein, Wartende abbrechen
fragt nach und leert die Zahl). Ein Knopf «Handy-Ansicht» schaltet auf 390 px. Farben und
Abstände sind die Tokens aus `packages/design-tokens/tokens/`. Der Prototyp ist **kein** Code für
`web/src` und wird nicht gebaut oder getestet.

---

## 8. Migration und Rückwärtsverträglichkeit

### 8.1 Was mit dem Bestand geschieht

- Alte `jobs`-Zeilen haben `started_at = NULL`: sie zeigen keine Dauer und zählen nicht zum
  Median; der erste `jobs.prune` räumt sie nach 7 Tagen weg (die neuesten 50 je Jobart bleiben,
  ohne Dauer).
- Die Zeitpläne beginnen beim ersten Start nach der Migration bei «jetzt» (`run_at_start`) bzw.
  «jetzt + Intervall»; die Poller-Historie (`spotlight_trakt.last_attempt`, `update_check.checked_at`)
  bleibt für die Anzeige «zuletzt geholt» ihrer Seiten, treibt aber keinen Lauf mehr.
- Nichts ist pausiert (`paused_at` NULL überall).

### 8.2 Wo alte Clients scheitern

Nirgends. Ein Client auf 1.3.0 sieht `getDashboard` unverändert (`paused` ist optional) und
kennt `/admin/job-types` nicht. Ein Server auf 1.3.0 mit einem 1.5.0-Client: die neue Seite
bekommt 404 und zeigt den Fehler wie jede Seite; das Dashboard geht.

### 8.3 Eingefrorene Datenbankstände

`testdata/db/` bekommt den Stand nach 00027 mit je einer Zeile in `job_types` und
`job_schedules`, einem `done`-Lauf mit `started_at`/`finished_at` und einem `running`-Lauf mit
Fortschritt; `frozen_*_test.go` prüft wie bei M11.

---

## 9. Sicherheit

### 9.1 Nur Administratoren, nichts Scoped

Alle Operationen sind `x-visibility: admin`; der Schema-Test erzwingt es. Kein `ItemRef`, keine
Bilder, keine Pfade des Zuschauers. Die Fortschrittsnotiz nennt Bibliothek und Ordner relativ
zur Wurzel — Pfade, die der Administrator auf der Bibliotheksseite ohnehin sieht; **nie** den
absoluten Pfad, nie einen Feed-Token, nie eine Adresse. `lastError` bleibt durch den
Redaction-Handler gefiltert (Regel 10).

### 9.2 Keine neue Aussenverbindung, kein `os/exec`

Der Scheduler reiht ein, mehr nicht; die Handler bleiben in ihren Paketen (Regel 3 unberührt).

### 9.3 Abbrechen ist endgültig, Pausieren hat Folgen

- `notify.deliver` abbrechen = die Mitteilung kommt nicht an. Bestätigung mit Warntext.
- `download.prepare` abbrechen = der Download des Profils endet mit Fehler, der Zuschauer sieht
  «fehlgeschlagen» wie bei jedem Fehlschlag; kein neuer Zustand.
- `scan.library` pausieren = neue Dateien bleiben unerkannt, Webhooks von Radarr und Sonarr
  reihen weiter ein (die Läufe warten). Der Kopf der Seite und das Dashboard sagen das in
  Warnfarbe, solange die Pause steht.
- Pause von `notify.deliver` = Zustellungen stauen; beim Fortsetzen kommen sie **alle** nach.
  Der Hinweistext nennt das.

### 9.4 Die vergessene Pause

Das grösste Risiko dieses Konzepts ist eine Pause, die nach Tagen noch steht und niemand mehr
sieht. Darum: `paused` im Dashboard als eigene Kachel in Warnfarbe, der Banner oben auf der
Seite, die Pause im Log, **und** kein automatisches Fortsetzen — ein Server, der von selbst
wieder losscannt, wäre schlimmer als einer, der steht, denn die Pause hat einen Grund (O-2 fragt,
ob eine Erinnerung nach 24 h als Mitteilung sinnvoll ist).

---

## 10. Schnitt in Schritte

Jeder Schritt ein Commit, `task ci` grün, DoD-Belege mit Befehl und Ausgabe. Zweig
`claude/hintergrundjobs` ab `main`, nachdem dieses Papier gemergt ist.

### Schritt 0 — Entscheide und Papiere

Frage 15 (F-1 bis F-9) ins Entscheidungsprotokoll; ADR 0017 «Scheduler, Fortschritt und Pause in
der Job-Queue» als Teilnachfolger von ADR 0002 (Abschnitte «Zeitpläne» und «Abbruch»); Hinweis
in ADR 0002; dieses Papier auf den dann aktuellen `main` verankern. `git diff --stat docs/spec/`
leer.

### Schritt 1 — Schema, Registry, Scheduler

Migration 00027 (4.5), `jobs.Type`, `jobs.Registry`, Scheduler mit Uhr-Mock-Tests, die fünf
Ticker-Schleifen raus, `RegisterJobs` → `RegisterTypes` in fünf Paketen, `job_types` und
`job_schedules` beim Start gefüllt, `UpdateSchedule` aus Spotlight- und Update-Einstellungen.
Kein Vertragsbruch, keine Oberfläche. DoD: `task ci`; `go test ./internal/jobs/ -run Scheduler`;
auf dem Entwicklungsrechner nach `task dev` zeigt `sqlite3 …/slouchcat.db 'SELECT type,
next_run_at FROM job_schedules'` sechs Zeilen.

### Schritt 2 — Fortschritt, Pause, Verlauf

`jobs.Progress`, Reporter im Arbeiter, drei meldende Handler; `paused_at` in `ClaimJob`,
Heartbeat-Pause mit `ErrPaused`, Scan und Download prüfen die Ursache; `started_at`/`finished_at`
in Claim/Complete/Fail/Cancel; `jobs.prune` mit Batches; Median-Abfrage. Tests: «pausiert wird
nicht beansprucht», «laufender Scan hält beim Heartbeat an und ist wieder queued, Versuche
unverändert, `last_scan_state` bleibt queued», «Fortschritt gedrosselt», «prune behält 50».

### Schritt 3 — Vertrag 1.5.0 und Server-Handler

`api/openapi.yaml` (5), `task gen`, die sieben Operationen im Paket `admin` (Queue-Methoden
`ListTypes`, `ListRuns`, `RunType`, `Pause`, `Resume`, `CancelQueued`), Schätzungen (6.2),
Vertragstest, Fixtures (`web/src/test/fixtures.ts`, `web/e2e/fixtures.ts`), die drei «Jetzt …»-
Knöpfe auf `runJobType`. Web-Minimum: der api-client kompiliert, keine neue Seite.

### Schritt 4 — Oberfläche

`JobsPage.tsx`, Navigation, Dashboard-Link und -Kachel, `ProgressBar` mit Storybook-Baseline,
Sprachdateien, `admin-jobs.spec.ts`, `NAV_PAGES`. DoD: `task ci`; `pnpm -C web test locales`;
Playwright-Lauf der zwei Specs mit Ausgabe.

### Schritt 5 — Abnahme, Messung, Doku

Image bauen, CT 190 umstellen (10.1), Marco nimmt am Handy ab: Scan starten, pausieren,
fortsetzen, Wartende abbrechen, Restzeit eines NAS-Scans beobachten. Messung nach
`docs/perf/hintergrundjobs.md`: Kosten von `listJobTypes` bei 50 000 Zeilen in `jobs` (vor dem
ersten `prune`) und danach; Ziel < 50 ms p95 auf CT 190. `docs/dev/CURRENT_STATUS.md`,
Release-Notes-Zeile (5.4).

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
| **F-1** | vor Schritt 2 | **Was heisst Pause für einen laufenden Lauf?** (a) Nur keine neuen Starts, Laufendes läuft zu Ende — ein NAS-Scan also noch Stunden; (b) Laufendes hält beim nächsten Heartbeat an (≤ 60 s) und geht ohne verbrauchten Versuch in die Queue zurück, beim Fortsetzen wird es wieder aufgenommen | **(b)**: sonst ist «Pause» bei genau dem Job eine Lüge, bei dem sie zählt (6.3) | | |
| **F-2** | vor Schritt 3 | **Ebene der Knöpfe.** Start und Pause je Jobart, Abbrechen je Lauf plus «Wartende abbrechen» je Jobart — oder auch Pause je Lauf? | **Je Jobart.** Ein Lauf von zwei Sekunden hat keine Pause; was Marco anhalten will, ist «die 236 Wartenden» (2.1) | | |
| **F-3** | vor Schritt 3 | **«Jetzt ausführen» auch für Ereignis-Jobs** (`metadata.match`, `image.fetch`, `probe.file`, …)? Dafür müsste jeder eine Bedeutung von «alle» bekommen («alle Titel ohne Bilder») | **Nein.** Das Nachholen gehört zur Seite Pflege (Befunde) und zum Scan; die Karte zeigt «bei Bedarf» und den Auslöser | | |
| **F-4** | vor Schritt 1 | **Zeitpläne einstellbar** (alle 6 h → alle 12 h, nachts)? | **Nur anzeigen.** Spotlight-Takt bleibt an seiner Seite einstellbar; alles Weitere als Idee 03 «Zeitpläne» notieren, mit `every_ms` ist der Boden gelegt | | |
| **F-5** | vor Schritt 1 | **Verschiebt «Jetzt ausführen» den nächsten geplanten Lauf?** | **Nein.** Takt bleibt; wer um 14:00 von Hand scannt, bekommt den 6-h-Lauf trotzdem um 18:00 — vorhersagbar, idempotent, billig | | |
| **F-6** | vor Schritt 2 | **Überlebt eine Pause den Neustart?** | **Ja** (Spalte, nicht Speicher), mit Warnkachel im Dashboard und Banner (9.4) | | |
| **F-7** | vor Schritt 4 | **Wortwahl.** «Hintergrundjobs» als Seitentitel (wie die Kachel heute); «Job» für die Jobart, «Lauf» für die Zeile; Knöpfe «Jetzt ausführen», «Pausieren», «Fortsetzen», «Abbrechen», «Wartende abbrechen» | wie links | | |
| **F-8** | vor Schritt 2 | **Verlauf:** erledigte Läufe 7 Tage, mindestens 50 je Jobart; fehlgeschlagene bleiben bis «Erneut» oder «Abbrechen» | wie links | | |
| **F-9** | vor Schritt 0 | **Reihenfolge zu Konzept 08** (Startseite, freigegeben, nicht gestartet). Beide sind unabhängig; wer zuerst geht, nimmt Frage 14 / ADR 0016 / 00026 / 1.4.0 | **Nach Marcos Wahl.** Dieses Papier ist kleiner (fünf Schritte, kein Vertragsbruch) und löst das Rätsel «236 wartend» sofort; 08 ist das grössere Stück Oberfläche | | |

Offene Punkte, die keine Entscheidung brauchen, aber eine Antwort verdienen:

- **O-1** Sollen `done`-Läufe im Dialog «Läufe von X» überhaupt erscheinen (Reiter *Erledigt*)?
  Vorschlag: ja, die neuesten 50 — man sieht, was der Server die letzte Stunde getan hat.
- **O-2** Erinnerung nach 24 h Pause als Mitteilung an das Hauptkonto (Kanal «System», wie
  `notify.JobFailed`)? Vorschlag: ja, einmalig, in Schritt 4 billig.
- **O-3** Zeigt die Karte `scan.library` je Bibliothek eine Zeile («Filme: läuft, 31 %», «Serien:
  wartet»)? Vorschlag: nein — die Bibliotheksseite hat das; unter «Läuft gerade» steht die
  Bibliothek in der Notiz.

---

## 12. Risiken

### 12.1 Fünf Schleifen werden eine

Der Scheduler ersetzt Code, der seit M1a, M6, M8 und M11 läuft. Das Risiko ist ein verlorener
Takt (Trakt läuft nie mehr, Updates werden nie geprüft). Gegenmittel: die Zeilen in
`job_schedules` sind **sichtbar** — auf der neuen Seite steht «Nächster Lauf» für jede, und ein
fehlender Eintrag fällt beim ersten Blick auf. Dazu die Uhr-Mock-Tests je Zeitplan und auf CT 190
ein Tag Beobachtung vor dem Merge (Schritt 5).

### 12.2 Pause bricht einen Handler, der die Ursache nicht prüft

Jeder Handler, der einen Kontextabbruch als Fehlschlag persistiert, muss `context.Cause` lesen.
Betroffen sind nach Durchsicht nur Scan (`aborted`) und Download (`failed`); alle anderen geben
den Fehler zurück und der Arbeiter macht `Release`. Ein Test je Handler in Schritt 2 sichert es.

### 12.3 Die Schätzung ist falsch, und Marco glaubt ihr

Ein Balken wirkt genauer, als er ist. Darum «ca.», «wird berechnet», «länger als üblich» als
drei ausdrückliche Zustände und nie eine Sekundenangabe unter einer Minute. Nach Schritt 5
vergleicht die Messung Schätzung und Wirklichkeit an einem NAS-Scan; liegt der Median-Fall um
mehr als Faktor 2 daneben, verschwindet er, und nur der gemessene Fall bleibt.

### 12.4 `jobs` ist gross, bevor `prune` das erste Mal lief

`listJobTypes` zählt und sucht den letzten Lauf je Jobart; mit Zehntausenden `done`-Zeilen und
ohne `jobs_history` wäre das ein Table-Scan alle 5 s. Der Index kommt in derselben Migration;
die Messung in Schritt 5 prüft den Fall «vor dem ersten prune».

### 12.5 Zwei Konzepte berühren dieselben drei Dateien

`sections.ts`, `router.tsx`, `de.json`/`en.json` und `admin-home.spec.ts` (`NAV_PAGES`) werden
von 08 **und** 09 angefasst. Das sind Zeilen-Konflikte beim Merge, keine Logik-Konflikte; wer
zweiter ist, rebased (F-9).

### 12.6 Was dieses Papier nicht zählen konnte

Wie viele `done`-Zeilen auf CT 190 liegen (Schätzung: Zehntausende nach 1683 `segments.detect`
und wochenlangen Stunden-Sweeps) — Schritt 5 misst. Und ob `download.prepare` seinen Fortschritt
in Bytes ohne Umbau melden kann; falls nicht, meldet er Segmente.

---

## 13. Review-Protokoll

Fassung 1 geht an den OMC-Kritiker (`oh-my-claudecode:critic`); seine Befunde und ihre
Einarbeitung werden hier protokolliert, bevor Marco das Papier freigibt.
