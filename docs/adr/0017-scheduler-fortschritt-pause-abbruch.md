# ADR 0017: Scheduler, Fortschritt, Pause und Abbruch-Haken in der Job-Queue

- Status: angenommen (30. Oktober 2026); **ersetzt teilweise ADR 0002** (Abschnitte «Zeitpläne» und
  «Abbruch»; Abschnitt «Was von ADR 0002 stehen bleibt»)
- Quelle: Konzept `docs/planning/09-hintergrundjobs.md` (Fassung 3, 30. Oktober 2026, Abschnitte 2
  bis 6 und 10); **Frage 15** in `docs/planning/04-entscheidungen.md` mit Marcos Antworten F-1 bis
  F-10, O-1 und O-2; ADR 0002 (Job-Queue), Entscheid vom 26. Oktober 2026 (`RaiseQueuedPriority`,
  manuelle Handlungen mit Priorität 7)
- Betroffene Spec-IDs: A-3 (Dashboard mit Hintergrundjobs), alle Hintergrundarbeiten (Scan, Proben,
  Metadaten, Bilder, Downloads, Zustellungen), Plan 3.6 (Jobs, Zeitpläne)
- Harte Regeln: 1 (generierter Code nur durch Codegen), 5 (Migrationen nur vorwärts, nach Vorlage),
  6 und 7 (JSON-Endpunkte über den strict server, `x-visibility` je Operation), 8 (Tastaturtest je
  neue Seite), 9 (keine harten Farben und Texte), 10 (keine Secrets in Logs; Notiz und Gegenstand
  redigiert), 11 (keine Nicht-Ziele), 12 (Entscheid im Server; Schätzungen kommen vom Server)

## Kontext

ADR 0002 hat in M0 die eigene Queue in SQLite festgelegt und zwei Dinge vertagt: den Scheduler für
`job_schedules` («folgt mit dem ersten periodischen Job») und einen Abbruch, der mehr kann als den
Kontext des Handlers beim nächsten Heartbeat zu beenden. Seither sind in drei Paketen fünf eigene
Schleifen mit `time.NewTicker` entstanden (Vollscan alle 6 h und Sweep stündlich in
`library.Service.Start`, Podcast-Poller alle 5 min, Trakt-Poller alle 15 min, Update-Poller alle
15 min), jede mit eigener «fällig»-Logik, und keine kennt den nächsten Lauf. Die Tabelle
`job_schedules` (`type`, `cron_expr`, `next_run`) ist seit M0 leer. Die Queue kennt je Lauf weder
Startzeit noch Endzeit noch Fortschritt, nur `updated_at`; `done`-Zeilen bleiben für immer liegen
(ausserhalb der Tests löscht nichts aus `jobs`); einen Zustand «pausiert» gibt es nirgends; und der
rohe Queue-Abbruch lässt Scan und Download in einem Fachzustand zurück, den die Bibliotheksseite als
«wartet» zeigt oder der Abgleich nach 10 min wieder einreiht.

Marco will (29. Oktober 2026, am Dashboard-Bild mit 236 wartenden und 4 laufenden Jobs) ein eigenes
Register in der Verwaltung: welche Jobs laufen, seit wann, wie lange noch, wann der letzte Lauf
fertig wurde, wann der nächste kommt, und je Job die Knöpfe Start, Pause und Abbruch. Am
30. Oktober 2026 hat er die zehn Fragen des Konzepts beantwortet (Frage 15); zwei Antworten weichen
von der Empfehlung ab: **Zeitpläne sind einstellbar** (Takt, Zeitfenster, Ein/Aus; F-4) und der
**Verlauf hält 30 Tage** (F-8).

Fünf Dinge waren zu entscheiden, die weder Spec noch Plan noch ADR 0002 festlegen:

1. **Intervalle oder cron-Ausdrücke** für die Zeitpläne, die Plan 3.6 «cron-artig» nennt.
2. **Was Pause bedeutet**: ein sechster Zustand je Lauf, oder ein Merkmal der Jobart.
3. **Woher die Restzeit kommt**, wenn nur der Bibliotheksscan heute einen Fortschritt meldet.
4. **Wie ein Abbruch die Fachlogik erreicht**, ohne dass die Queue Scan und Download kennt.
5. **Wie viel Verlauf** die Queue behält, und wer ihn aufräumt.

Verworfen: (a) **cron-Ausdrücke** – kein Job braucht «jeden Montag um 3»; Trakt rechnet schon in
Stunden (`interval_hours`), und ein Kalender-Zeitplan brächte Zeitzonen- und Nachhol-Fragen mit, die
kein heutiger Takt hat. (b) **Ein Zustand `paused` je Lauf** (und ein Endpunkt «diesen Lauf
anhalten», F-10 Variante b) – ein sechster Zustand in ADR 0002, eine dritte Stelle für «Fortsetzen»
und die offene Frage, was mit einem beiseitegelegten Lauf nach Tagen geschieht; ein `metadata.match`
von zwei Sekunden hat keine Pause. (c) **Pause nur für neue Starts, Laufendes läuft zu Ende** (F-1
Variante a) – genau beim Job, bei dem die Pause zählt (ein NAS-Scan von Stunden), wäre sie eine Lüge.
(d) **Mittelwert statt Median** für die Schätzung – ein erster NAS-Scan von sechs Stunden
verschätzte jeden Zwei-Minuten-Scan auf Jahre. (e) **Fünf Schleifen behalten und nur anzeigen** –
dann wüsste weiterhin niemand, wann der nächste Lauf ist, und «Jetzt ausführen» hätte keinen Ort.
(f) **`goqite` oder ein anderer Fremd-Scheduler** – ADR 0002 gilt: kein Fremdcode im kritischen Pfad
der Hintergrundarbeit, und die Prüfung von damals hat sich nicht geändert. (g) **Eine einstellbare
Arbeiterzahl in der Oberfläche** (F-9) – Arbeiter zur Laufzeit starten und stoppen ist ein eigenes
Thema; `SLOUCHCAT_WORKERS` bleibt die einzige Stellschraube, die Seite zeigt die Zahl je Pool.

## Entscheid

| Gegenstand | Entscheid |
|---|---|
| **Scheduler im Paket `jobs`** | Eine Goroutine je Server (wie die Arbeiter in `main.go`), Tick alle 60 s, liest `job_schedules WHERE enabled = 1 AND next_run_at <= now`, ruft je Treffer die `Run`-Funktion der Jobart mit deren normaler Priorität, schreibt `next_run_at = now + every_ms` (am Zeitfenster ausgerichtet) und `last_fired_at = now`. **Intervalle statt cron** (Abstand zu Plan 3.6, Abschnitt 3 des Konzepts): `every_ms` 5 min bis 7 d, dazu ein **Zeitfenster** je Zeitplan (`window_start`, `window_end` in Minuten des Tages, `time.Local` des Servers, über Mitternacht erlaubt) und `enabled`. Verpasste Läufe (Server war aus) holen **nicht** nach: ein Takt, kein Kalender. Die fünf Ticker-Schleifen fallen; `library.Start` behält nur die Dateiwächter. Zwei Schleifen bleiben, weil sie keine Zeitpläne sind: der Download-Abgleich `offline.Run` (Reparatur) und der Telegram-Link-Poller (reiht nichts ein) |
| **Zeitpläne einstellbar, eine Zeile je Zeitplan-Jobart** (F-4) | Takt, Zeitfenster und Ein/Aus liegen in `job_schedules`, nicht mehr im Code. Die Konstanten im Code (`fullScanInterval`, `sweepInterval`, …) sind nur noch **Vorgaben für die erste Zeile**; `EnsureSchedule` legt beim Start **nur fehlende** Zeilen an und leitet `next_run_at` aus der bisherigen Historie ab (`update_check.checked_at + 24 h`, `trakt_last_attempt + interval_hours`, Untergrenze «jetzt»). `start_delay_ms` ersetzt «sofort beim Start» (0) und «60 s nach dem Start» (60 000); NULL heisst «beim Start nicht feuern» (Trakt). `UpdateSchedule` richtet `next_run_at` neu aus (`last_fired_at + every`, Untergrenze «jetzt», dann am Fenster), damit ein kürzerer Takt den nächsten Lauf vorzieht. **Ein Wert, zwei Türen** für den Takt von `trakt.refresh` (`spotlight_config.interval_hours`, Seite Spotlight) und das Ein/Aus von `update.check` (`update_check_enabled`, Seite Update): beide Seiten schreiben über `UpdateSchedule` in die Zeile, die Jobseite schreibt in die bestehende Einstellung zurück. Das Ein/Aus von `trakt.refresh` ist **nicht** an `spotlight_config.enabled` gebunden: der heutige Poller holt die Liste auch bei ausgeschaltetem Band (der Cache bleibt warm, das Einschalten zeigt sofort etwas); das bleibt so (Konzept 13.3) |
| **Jobart als Begriff: `jobs.Type` und `jobs.Registry`** | Jeder Dienst registriert nicht mehr nur Handler (`RegisterJobs(w)`), sondern die **Jobart** (`RegisterTypes(r)`): Name, Gruppe (`library`, `metadata`, `delivery`, `system`), Pool (`general`, `download`, `notify`), Auslöser (`schedule`, `event`, `manual`), Priorität des planmässigen Laufs, Handler, `Run` (was ein Lauf dieser Jobart heisst: ein Scan je Bibliothek, jeder fällige Feed; muss einen wartenden Lauf gleichen Schlüssels über `RaiseQueuedPriority` anheben), `Cancel` (Fachabbruch), `Every`, `StartDelay`. Die Registry legt je Jobart die Zeile in `job_types` an und bei `Every > 0` den Zeitplan. Alles über eine Jobart ausser der Pause liegt im Code, nicht in der Datenbank. Zwei neue Jobarten sind Haushalt der Queue, kein neues Verhalten: `podcast.poll` (der sichtbar gemachte 5-Minuten-Takt, `Run` = `QueueDuePodcasts`) und `jobs.prune` (Aufräumen) |
| **Start («Jetzt ausführen»)** (F-3, F-5) | Nur Jobarten mit `Run ≠ nil` (Zeitplan- und Hand-Jobs). Der Server ruft `Run` mit **Priorität 7** (Entscheid vom 26. Oktober 2026) und gibt die Läufe zurück; ein Hand-Lauf verschiebt `next_run_at` **nicht** und ignoriert das Zeitfenster. Pausiert: 409 `job_type_paused`. Die heutigen Knöpfe «Jetzt abrufen», «Jetzt prüfen», «Aktualisieren» bleiben samt Endpunkten; `runJobType` ruft dieselben Dienstmethoden und übernimmt deren Fehlercodes |
| **Pause ist ein Merkmal der Jobart** (F-1, F-6, F-10) | Neue Tabelle `job_types (type, paused_at, updated_at)`; `paused_at` ist die Pause, dauerhaft über Neustarts. Die fünf Zustände von ADR 0002 bleiben: ein angehaltener Lauf ist wieder `queued`. Wirkung: `ClaimJob` nimmt keine Läufe pausierter Jobarten; laufende Läufe werden **sofort** über `context.WithCancelCause(…, jobs.ErrPaused)` abgebrochen (Laufliste im Speicher der Registry, Heartbeat als Rückfall) und gehen **ohne verbrauchten Versuch** zurück (`Release` **vor** dem `switch` in `worker.run`, gleichgültig, was der Handler zurückgab; kein `JobFailed`). Der Knopf «Pausieren» in der Zeile «Läuft gerade» pausiert den Job des Laufs: kein Endpunkt je Lauf, kein Zustand `held`. Kein automatisches Fortsetzen; Dashboard-Kachel «Pausiert» in Warnfarbe und Banner auf der Seite gegen die vergessene Pause |
| **Fortschritt, Start und Ende je Lauf** | Sechs Spalten in `jobs`: `started_at` (jeder Claim neu), `finished_at` (`Complete`, `Fail` im Endzustand, `Cancel`, `FailExhaustedLeases`), `progress_done`, `progress_total`, `progress_note` (≤ 200 Zeichen, durch `logging.RedactString`), `skipped` (ein Lauf, der nichts zu tun hatte). Handler melden über `jobs.Progress(ctx, done, total, note)`; der Arbeiter schreibt höchstens alle 2 s und mit jedem Heartbeat unter eigener Lease. Vier Handler melden (Scan in beiden Phasen, Segmente, Download in Segmenten, Keyframes als Notiz); drei rufen `jobs.Skip`. Die Dauer eines Laufs ist `finished_at − started_at` des **letzten** Versuchs |
| **Schätzung vom Server** (Regel 12) | Restzeit je Lauf: gemessen (`elapsed · (total − done) / done` ab 5 % und 30 s), sonst **Median** der neuesten 50 nicht übersprungenen `done`-Läufe der Jobart (ab 5 Läufen; `elapsed > 2 · median` ergibt «länger als üblich»), sonst keine. Wartezeit je Jobart **kumuliert nach Priorität** über den Pool, geteilt durch die Arbeiterzahl; Stau je Pool als dieselbe Summe. Alles ist «ca.»; der Client zählt nur Sekunden hoch |
| **Verlauf 30 Tage, Aufräumjob `jobs.prune`** (F-8, O-1) | Täglich; löscht `done` und `cancelled` älter als 30 Tage, behält je Jobart die neuesten 50, in Batches à 500; `failed` bleibt, bis der Administrator «Erneut» oder «Abbrechen» drückt. Der Dialog «Läufe von X» zeigt die neuesten 50 erledigten. Neuer Index `jobs_history (type, state, finished_at DESC)` trägt Median und Aufräumen |
| **Abbruch-Haken je Jobart** (F-2) | `Type.Cancel` beendet den Fachzustand, bevor die Queue den Lauf abbricht: `scan.library`/`scan.path` über `library.CancelScan` (Bibliothek endet `aborted/cancelled`), `download.prepare` über `offline.CancelPreparation` (Download `failed` mit `cancelled_by_admin`, der Abgleich lässt ihn liegen); alle anderen nur Queue. `cancelJob` je Lauf läuft neu über den Haken; neu `cancelJobRuns` je Jobart mit `scope=queued` (Vorgabe) oder `all` (wartende und laufende, mit Bestätigung, die die laufenden gesondert nennt). Abbrechen ist endgültig; eine Zustellung, die abgebrochen wird, kommt nicht an |
| **Oberfläche und Vertrag** (F-7) | Neue Verwaltungsseite `/admin/jobs` («Hintergrundjobs»; «Job» = Jobart, «Lauf» = Zeile) mit der Schranke des Dashboards; Vertrag **1.5.0, additiv**: `listJobTypes`, `listJobRuns`, `runJobType`, `pauseJobType`, `resumeJobType`, `updateJobSchedule`, `cancelJobRuns` unter `/admin/job-types/{type}` (getrennt von `/admin/jobs/{id}`), alle `x-visibility: admin`, kein `ItemRef`; `JobCounts.paused` optional. Texte der Jobarten aus den Sprachdateien mit `_` statt `.` im Schlüssel |

## Was von ADR 0002 stehen bleibt

Die Queue selbst: Tabelle `jobs` mit den fünf Zuständen, Lease (5 min, Heartbeat 60 s), Idempotenz-
Schlüssel unter den aktiven Jobs, Debounce, Backoff 10 s · 2^(Versuch − 1), Priorität 0–9 mit der
Leiter in der Paketdokumentation, `RaiseQueuedPriority` (26. Oktober 2026), der Arbeiter als
Goroutine mit Panic-Fang und `Release` beim Herunterfahren, alle Abfragen in
`server/queries/system/jobs.sql`. **Abgelöst** sind allein der Absatz «Tabelle `job_schedules`
(`type`, `cron_expr`, `next_run`) ist angelegt; der Scheduler … folgt mit dem ersten periodischen
Job» (die Tabelle wird in Migration 00027 neu gebaut, der Scheduler kommt jetzt, ohne
`rating.verify`, das ein Pflege-Befund bleibt) und der Satz zu `Cancel` («der Worker bemerkt den
Abbruch beim nächsten Heartbeat»): Abbruch und Pause laufen neu über den Abbruch-Haken und die
Laufliste im Speicher, der Heartbeat ist nur noch der Rückfall.

## Restrisiken

- **Fünf Schleifen werden eine.** Ein verlorener Takt (Trakt läuft nie mehr, Updates werden nie
  geprüft) oder ein verlorener Sonderfall (Trakt ohne Client-Schlüssel leert die Einträge; bleibt im
  `Run`). Gegenmittel: die Zeilen in `job_schedules` sind auf der Seite sichtbar, Uhr-Mock-Tests je
  Zeitplan, ein Tag Beobachtung auf CT 190 vor dem Merge.
- **Pause gegen den heutigen Arbeiter.** Wird die Ursache `ErrPaused` nicht vor dem `switch` in
  `worker.run` erkannt, verbraucht jede Pause einen Versuch und meldet nach fünf Pausen einen
  Fehlschlag. Test «fünfmal pausieren und fortsetzen → `attempts` unverändert».
- **Die Schätzung ist falsch, und man glaubt ihr.** Drei ausdrückliche Zustände («ca.», «wird
  berechnet», «länger als üblich»), nie Sekunden unter einer Minute; die Messung in Schritt 5
  vergleicht Schätzung und Wirklichkeit an einem NAS-Scan, und der Median-Fall fällt, wenn er um
  mehr als Faktor 2 danebenliegt.
- **Ein Zeitfenster kann einen Job aushungern** (Takt 6 h, Fenster 03:00–04:00: ein Lauf am Tag).
  Gewollt, aber leicht vergessen; die Karte zeigt Takt **und** Fenster. Die Zeitzone ist die des
  Servers (`TZ` im Container); der Dialog nennt sie.
- **`jobs` ist gross, bevor `prune` das erste Mal lief** (CT 190: Zehntausende Zeilen). Der Index
  `jobs_history` kommt in derselben Migration; Schritt 5 misst `listJobTypes` davor und danach
  (Ziel < 50 ms p95).
- **Die vergessene Pause** ist das grösste Risiko: Kachel, Banner, Log; kein automatisches
  Fortsetzen, weil die Pause einen Grund hat. Eine Erinnerung nach 24 h ist als Idee notiert.

## Folgen

- **Migration 00027** (nur vorwärts, nach Vorlage): sechs Spalten und der Index `jobs_history` an
  `jobs`; neue Tabelle `job_types`; `job_schedules` fällt und wird neu gebaut (Wächter wie in 00023:
  die Migration bricht ab, wäre die Tabelle wider Erwarten nicht leer). Die Zeilen schreibt die
  Registry beim Start, nicht die Migration.
- **Paket `jobs`** wächst um `Type`, `Registry`, Scheduler, `Progress`, `Skip`, `ErrPaused`, die
  Pausenprüfung im Claim und den Aufräumjob; `RegisterJobs(w)` in `library`, `metadata`, `admin`,
  `offline`, `notify` wird zu `RegisterTypes(r)`, die Handler bleiben. `enqueueScan` und seine
  Geschwister bekommen die Priorität als Parameter.
- **Vertrag 1.5.0, additiv**; Release-Notes-Zeile: «Neue Verwaltungsseite Hintergrundjobs; Vertrag
  1.5.0 fügt `/admin/job-types` hinzu, nichts entfällt.»
- **Eine neue Seite in der Wirkung** (Regel 8): `web/e2e/admin-jobs.spec.ts`, `NAV_PAGES` in
  `admin-home.spec.ts`.
- **Kein Geheimnis im Protokoll** (Regel 10): Pause und Fortsetzen werden mit Jobart protokolliert,
  nie mit Nutzdaten; `progress_note` und `subject` gehen durch `logging.RedactString`.
- Spec A-3 bleibt unverändert und wird erweitert erfüllt; Plan 3.6 «cron-artig» ist durch Intervalle
  mit Zeitfenster abgelöst (Abschnitt 3 des Konzepts). Umsetzung in sechs Schritten nach Konzept
  Abschnitt 10; Vertrag erst in Schritt 3, Oberfläche in Schritt 4.
