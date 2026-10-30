# ADR 0002: Job-Queue – eigene Queue statt goqite

- Status: angenommen (18. Oktober 2026); **teilweise ersetzt durch ADR 0017** (30. Oktober 2026):
  Zeitpläne (`job_schedules` wird in Migration 00027 neu gebaut, der Scheduler mit Intervallen, Zeitfenster
  und Ein/Aus kommt mit Konzept 09, nicht mit `rating.verify`) und Abbruch (Abbruch-Haken je Jobart und
  Pause je Jobart; der Heartbeat ist nur noch der Rückfall). Queue-Semantik, Lease, Idempotenz,
  Backoff, Priorität und `RaiseQueuedPriority` bleiben wörtlich.
- Quelle: `docs/planning/03-plan-v1.md`, Abschnitt 3.6 (Jobs) und M0 Schritt 3 (goqite-Timebox)
- Betroffene Spec-IDs: alle Hintergrundarbeiten (Scan, Proben, Metadaten, Bilder, Downloads)

## Kontext

Plan 3.6 legt den Umfang der Job-Queue fest: Lease mit Ablauf (5 min, Heartbeat alle 60 s),
Idempotenz-Schlüssel (unique), Debounce (gleiche `scan.path`-Jobs innert 2 s zusammengefasst), Retry mit
exponentiellem Backoff (max. 5 Versuche), Priorität 0–9, Abbruch und cron-artige Zeitpläne. Die Queue
liegt in derselben SQLite-Datei wie alle anderen Daten; ein Zusatzdienst kommt nicht in Frage (ADR 0001).

Vor einer Eigenlösung war `goqite` zu prüfen (Timebox 1 Tag). Fehlen Priorität, Idempotenz oder
Zeitpläne, bleibt die Eigenlösung.

## Prüfung von goqite

Geprüft wurde `maragu.dev/goqite` v0.4.0 (aktuellstes Release, 20. November 2025; der frühere
Modulpfad `github.com/maragudk/goqite` zeigt auf dasselbe Projekt) anhand von README und Quellcode
(`goqite.go`, `schema_sqlite.sql`, `jobs/runner.go`). Aufwand: rund eine Stunde, deutlich innerhalb der
Timebox.

| Anforderung aus Plan 3.6 | goqite v0.4.0 | Beleg |
|---|---|---|
| Priorität | **ja** | `Message.Priority`, Empfang mit `order by priority desc, created` |
| Lease mit Verlängerung | ja | Feld `timeout`, `Queue.Extend`; der Runner verlängert selbständig |
| Verzögerter Start | ja | `Message.Delay` |
| Idempotenz-Schlüssel | **nein** | Tabelle ohne Schlüsselspalte und ohne Unique-Index; `Send` fügt immer ein |
| Debounce | nein | folgt aus dem fehlenden Schlüssel |
| Zeitpläne (cron-artig) | **nein** | weder Tabelle noch API |
| Retry mit exponentiellem Backoff | nein | erneute Zustellung nach festem Timeout; nach `MaxReceive` (Standard 3) bleibt die Nachricht unsichtbar in der Tabelle liegen, ohne Zustand „fehlgeschlagen“ und ohne letzte Fehlermeldung |
| Abbruch | teilweise | nur `Delete`; ein laufender Handler erfährt davon nichts |
| Testbare Uhr | nein | `time.Now()` fest eingebaut; Lease-Ablauf ist nur mit echtem Warten testbar |
| Robustheit des Runners | eingeschränkt | unbekannter Job-Name löst `panic` aus; Fehler werden auf Info-Stufe geloggt |
| Abhängigkeiten | keine | positiv: reines `database/sql` |

Ergebnis: goqite deckt Priorität und Lease ab, aber zwei der drei im Plan genannten Pflichtmerkmale
fehlen (Idempotenz-Schlüssel und Zeitpläne), dazu Backoff, ein Endzustand für fehlgeschlagene Jobs und
der Abbruch laufender Jobs. Diese Merkmale nachzurüsten hiesse, eigene Spalten, Indizes und Abfragen
neben das goqite-Schema zu legen; der Wrapper würde grösser als die Eigenlösung.

## Entscheid

Slouchcat nutzt eine **eigene Queue** im Paket `server/internal/jobs`. goqite wird nicht als
Abhängigkeit aufgenommen.

Umsetzung in M0 (Grundgerüst):

- Tabelle `jobs` (Migration `00001_jobs.sql`): `type`, `priority` 0–9, `idempotency_key`, `payload`
  (JSON), `state` (`queued`, `running`, `done`, `failed`, `cancelled`), `run_after`, `lease_owner`,
  `lease_until`, `attempts`, `max_attempts` (Standard 5), `last_error`, `created_at`, `updated_at`.
  Zeiten als Unix-Millisekunden.
- Der Idempotenz-Schlüssel ist eindeutig unter den **aktiven** Jobs (`queued`, `running`), umgesetzt als
  partieller Unique-Index. Ein zweites Einreihen mit demselben Schlüssel liefert die ID des aktiven Jobs.
  Ist der Job beendet, ist der Schlüssel wieder frei; sonst könnte derselbe Pfad nie ein zweites Mal
  gescannt werden.
- Debounce: Einreihen mit Schlüssel und `Delay = DebounceWindow` (2 s). Weitere Einreihungen im Fenster
  fallen auf denselben Job.
- `Claim` wählt den Job mit höchster Priorität, bei Gleichstand den ältesten, und setzt die Lease auf
  jetzt + 5 min. Abgelaufene Leases werden neu vergeben; hat ein Job dabei alle Versuche verbraucht,
  endet er als `failed`.
- `Fail` plant den nächsten Versuch nach 10 s · 2^(Versuch − 1) (10 s, 20 s, 40 s, 80 s), nach
  `max_attempts` endet der Job als `failed` mit letzter Fehlermeldung.
- `Cancel` bricht wartende und laufende Jobs ab; der Worker bemerkt den Abbruch beim nächsten Heartbeat
  und bricht den Kontext des Handlers ab. **Ergänzt durch ADR 0017:** Abbruch und Pause laufen
  über den Abbruch-Haken der Jobart und die Laufliste im Speicher; der Heartbeat ist der Rückfall.
- Der Worker läuft als Goroutine, verlängert die Lease alle 60 s, fängt Panics ab und gibt beim
  Herunterfahren einen unterbrochenen Job ohne verbrauchten Versuch an die Queue zurück.
- Tabelle `job_schedules` (`type`, `cron_expr`, `next_run`) ist angelegt; der Scheduler, der daraus Jobs
  erzeugt, folgt mit dem ersten periodischen Job (spätestens `rating.verify`, Plan 3.2). **Abgelöst durch ADR 0017:**
  die Tabelle wird neu gebaut (`every_ms`, `start_delay_ms`, `enabled`, Zeitfenster, `next_run_at`), der
  Scheduler liegt im Paket `jobs`.
- Alle Abfragen stehen in `server/queries/system/jobs.sql` (sqlc, Paket `store/systemq`).
- **Ergänzung vom 26. Oktober 2026 (M11):** `RaiseQueuedPriority` hebt die Priorität eines noch nicht
  beanspruchten Jobs an und senkt sie nie. Grund: Das zweite Einreihen unter einem aktiven Schlüssel
  liefert den wartenden Job unverändert zurück, auch mit höherer Priorität — eine manuelle Handlung
  (Spotlight «Jetzt abrufen», Schlüsselwechsel) blieb damit hinter einem Scan-Nachlauf gleicher
  Priorität stehen, auf CT 190 hinter 1683 `segments.detect` für Tage. Manuelle Handlungen reihen mit
  Priorität 7 (wie der manuelle Match) ein und heben den wartenden Job an; die nächtlichen Läufe bleiben
  bei ihrer Priorität. Entscheid in `docs/planning/04-entscheidungen.md`.

## Konsequenzen

- Rund 400 Zeilen eigener Code (Queue und Worker samt Kommentaren) mit Tests: Deduplizierung,
  Priorität, Lease-Ablauf mit injizierbarer Uhr, Backoff, Abbruch, Worker-Verhalten. Kein Fremdcode im
  kritischen Pfad der Hintergrundarbeit.
- Handler müssen idempotent sein, weil eine abgelaufene Lease zu doppelter Ausführung führen kann
  (Paketdokumentation von `jobs`).
- Wer während eines laufenden Jobs denselben Schlüssel einreiht, erhält den laufenden Job. Braucht ein
  Aufrufer danach einen weiteren Lauf (z. B. Dateiänderung während eines Scans), reiht er nach Ende des
  Jobs erneut ein; der Watcher in M1a berücksichtigt das.
- Der Scheduler für `job_schedules` war bis Konzept 09 offen; ADR 0017 (30. Oktober 2026) baut ihn
  mit Intervallen statt cron-Ausdrücken.
