# Analyse C: Integrationen, Webhooks, ausgehende Aufrufe, Secrets, Jobs, Scan, Verwaltungsrahmen, API-Regeln, Non-Goals

Stand: Haupt-Checkout, Zweig `main`, Kopf `ba6efe31` (16. November 2026). Nur gelesen, nichts geändert,
keine Tests oder Builds. Pfade relativ zu `/mnt/Nerdcloud/NerdcloudDaten/06_KI/01_Slouchcat`.
Kennzeichnung: **[gelesen]** = am Code/Dokument belegt, **[Vermutung]** = Schluss ohne vollständigen Beleg.

---

## 1. Webhook-Empfang Radarr/Sonarr (ADR 0010)

**[gelesen]**

- **Richtung:** nur eingehend. «Slouchcat never calls Radarr or Sonarr and registers nothing there
  (controlling the Arr services is a non-goal)» – `server/internal/integrations/integrations.go:5-6`;
  Plan 5.6 «Der Webhook ist die einzige Verbindung zu den Arr-Diensten und rein eingehend» –
  `docs/planning/03-plan-v1.md:314-316`.
- **Route:** `POST {BasePath}/webhooks/{token}`, `x-visibility: public`, CSRF-Ausnahme als Liste
  `csrfExemptRoutes` – `server/internal/api/webhook_handlers.go:45-54`; ADR `docs/adr/0010-webhook-empfang.md:24-25`.
  ADR 0010 Folgen: «Die CSRF-Ausnahme ist die erste ihrer Art. Wächst die Liste, gehört sie in eine eigene
  Betrachtung» – `docs/adr/0010-webhook-empfang.md:64-65`.
- **Token:** 256 Bit (`tokenBytes = 32`, 43 Zeichen base64url) – `integrations.go:40-45`; gespeichert nur
  SHA-256 (`HashToken`) und die letzten vier Zeichen (`token_tail`) – `integrations.go:262-275`;
  Klartext und volle Adresse nur in der Antwort von Anlegen/Erneuern (`reveal`) – `integrations.go:248-256`;
  unbekanntes Token → 401 ohne Protokolleintrag – ADR 0010:25; Längenprüfung vor DB-Abfrage –
  `integrations.go:315-327`. Tabellen `webhook_integrations` (kind CHECK `radarr`/`sonarr`, `path_map` JSON,
  `last_seen_at`) und `webhook_events` (Trigger hält 20 Zeilen) – `server/migrations/00017_*.sql:21-57`.
- **Ablauf `Receive`** – `server/internal/integrations/receive.go:41-93`: Authenticate → Ratenbegrenzung
  10/s, Burst 10 je Integration (`RequestsPerSecond`, `ratelimit.Allow`, 429 + `Retry-After: 1`) –
  `receive.go:17-19, 114-136`, ADR 0010:30 → `TouchWebhookIntegration` (letzter Kontakt) → Ereignis:
  - `Test` → `acknowledged` (protokolliert, kein Scan) – `receive.go:57-59`
  - wirkende Ereignisse `download` (Import/Upgrade), `rename`, `moviefiledelete`, `episodefiledelete`,
    `moviedelete`, `seriesdelete` (Vergleich ohne Gross/Klein) – `receive.go:24-32`
  - alle anderen (`Grab`, `Health`, …) → `ignored`, 200 – `receive.go:60-62`
- **Pfad:** Radarr `movie.folderPath`, Sonarr `series.path`, strikt nach Typ der Integration; fehlt er →
  422 + `invalid_payload` – `receive.go:65-75`. Pfad-Umsetzung (längster Präfix, absolute Pfade) und
  Auflösung gegen Bibliothekswurzeln (`paths`) – `integrations/pathmap.go`, ADR 0010:42-52; ausserhalb →
  422 `outside_roots` – `receive.go:77-81`.
- **Wirkung:** genau ein `scan.path` über `library.RequestPathScan` (gleicher Auftrag wie der Watcher,
  Debounce 2 s, Schlüssel `scan:<id>:path`, Priorität 6) – `receive.go:88-92`, ADR 0010:27,
  `server/internal/library/doc.go:90-97`. Protokolleintrag erst nach erfolgreichem Einreihen – `receive.go:85-91`.
- **Protokoll:** je Integration 20 Zeilen: Zeit, Ereignis (≤ 80 Zeichen), gemeldeter Pfad, umgesetzter Pfad,
  Ergebnis (`scan_queued|acknowledged|ignored|outside_roots|invalid_payload`) – `00017:33-57`.
  Token im Log redigiert (`webhookPath`) – `server/internal/logging/logging.go:119ff`, ADR 0010:26.
- **Dashboard:** Anzahl Meldungen 24 h, Warnung «still» nach `SilentAfter = 7 * 24h` –
  `server/internal/admin/integrations.go:30-32, 84-107`.
- **TMDB-/TVDB-IDs im Webhook?** **Nein, werden weder gelesen noch gespeichert.** `model.WebhookPayload`
  hat nur `Event`, `MoviePath`, `SeriesPath` – `server/internal/model/integrations.go:45-51`; der Handler
  kopiert nur `eventType`, `movie.folderPath`, `series.path` – `server/internal/api/webhook_handlers.go:22-33`;
  im Vertrag `additionalProperties: true`, «Only `eventType` and the directory are read» –
  `api/openapi.yaml:19211-19241`. `webhook_events` hat keine ID-Spalte – `00017:33-44`.
  **[Vermutung]** Radarr/Sonarr senden `movie.tmdbId`/`series.tvdbId` mit (Fremdwissen, im Repo nicht
  geprüft; aufgezeichnete Nutzlasten liegen laut `04-entscheidungen.md:54` unter `testdata/webhooks/` – nicht gelesen).
- **Paketgrenze:** `integrations` greift nie auf `items`/`media_files`/`tracks` zu (Regel 2, depguard) –
  `integrations.go:17-18`, ADR 0010:28.

## 2. Verwaltungsseite Integrationen – vollständiges Inventar

**[gelesen]** Seite `/admin/integrations` (`IntegrationsPage`, `web/src/features/admin/IntegrationsPage.tsx`),
Bereich «Automatisierung». Texte aus `web/src/locales/de.json` (`admin.integrations.*`, ab Zeile ~3698).

### 2.1 Rahmen
- Titel «Integrationen», Einleitung «Schlüssel der Anbieter, Versand von Benachrichtigungen und die
  Update-Prüfung. Geheime Werte speichert Slouchcat verschlüsselt und zeigt sie nie wieder an.»
- Zugang nur Administrator mit Hauptprofil; sonst Hinweis `admin.access.notAllowed` – `IntegrationsPage.tsx:65, 180-184`.
- Ladezustand Skeleton, Fehler mit «Erneut versuchen» – `:186-197`.
- Ohne `?item=`: Zeile «{{count}} von {{total}} eingerichtet» (aus `getDashboard`) und **sechs Kacheln**
  (Liste «Dienste») – `:208-237`; mit `?item=<dienst>` die Ansicht des Diensts, Verweis «Alle Integrationen» – `:200-205`.
  Dienste und Reihenfolge `SERVICES = ['keys','webhooks','email','telegram','push','server']` –
  `web/src/features/admin/integrations/services.tsx:12`; Deep-Link `?item=` – `services.tsx:16`.
- Jede Kachel: Symbol, Titel als Link, Beschreibung (`admin.integrations.services.*`), Meta («n von m
  Anbietern eingerichtet», «n Integration(en)», Quelle, «Schlüsselpaar vorhanden.», Chip «ungespeichert»),
  Zustand aus dem Dashboard (Punkt + «Name: Eingerichtet/Nicht eingerichtet/Fehler/Ausgeschaltet» und Detail;
  für Webhooks «Kein Webhook eingerichtet.» bzw. «n Meldungen im Protokoll der letzten 24 Stunden, von m
  Integrationen.» und «Seit sieben Tagen ohne Meldung: …») – `services.tsx:59-151`.
  Zuordnung Dashboard-Schlüssel → Dienst: keys = tmdb, tvdb, youtube, trakt; email; telegram; push;
  server = updateCheck (acht Schlüssel) – `services.tsx:28-35`.
- Alle Ansichten bleiben gemountet (Entwürfe, einmal gezeigte Adresse überleben Wechsel); Verlassen mit
  ungespeicherten Eingaben fragt (`useUnsavedChanges`, ConfirmDialog) – `IntegrationsPage.tsx:49-60, 105, 297`.
  Fokusführung beim Wechsel – `:136-154`.

### 2.2 Ansicht «Schlüssel der Anbieter» (`?item=keys`)
- Einleitung «Slouchcat holt damit Texte, Bilder und Trailer. Ohne Schlüssel bleibt der Anbieter aus.»
- **Sechs Anbieterkarten** (`PROVIDERS`, `ProviderCard.tsx:37-52`), je Karte zugeklappt: Name, Zweck,
  Chips «ungespeichert», «abgewiesen», Quelle («In deploy/.env gesetzt» / «Gespeichert» / «Nicht gesetzt»),
  Knopf «Einrichten/Bearbeiten/Ansehen» bzw. «Zuklappen». Offen: Felder, Hilfe mit Link «Schlüssel bei
  {{name}} holen», «Speichern» (ein Knopf für alle Felder), «Prüfen» (Server fragt Anbieter: ok /
  not_configured / invalid / unreachable / failed), «Löschen …» mit Bestätigung («{{name}} löschen?» –
  «Der gespeicherte Wert wird entfernt. Was ihn braucht, funktioniert danach nicht mehr.»). Platzhalter
  «Gesetzt[, endet auf ….]; leer lassen behält ihn»; Wert aus `.env` sperrt Felder («Der Wert kommt aus
  deploy/.env … «Prüfen» fragt den Anbieter trotzdem.»); Meldung «gespeichert, aber der Anbieter weist den
  Schlüssel ab» – `ProviderCard.tsx:85-277`, `integrations/shared.tsx:136-160`.
  | Anbieter | Felder | Zweck (de.json) |
  |---|---|---|
  | TMDB | Zugriffstoken (v4) | Texte, Poster, Trailer; Pflicht für Video |
  | TVDB | Schlüssel; Abo-PIN (nur bei Abo-Schlüssel) | zweite Quelle Serien/Anime |
  | Fanart.tv | Projekt-Schlüssel; Persönlicher Schlüssel (freiwillig) | Logos, Hintergründe, Disc |
  | YouTube | Schlüssel | Trailer-Suche |
  | Trakt | Client-ID | Listen für Spotlight und Reihen |
  | Google Books | Schlüssel | Buchdaten für Hörbücher |

### 2.3 Ansicht «Webhooks von Radarr und Sonarr» (`?item=webhooks`) – `WebhooksSection.tsx`
- Einleitung «… Slouchcat fragt die Dienste nie von sich aus ab.» (`webhooks.intro`) – `:499`.
- Formular «Neue Integration»: Feld Name (Hilfe «Frei wählbar …», Pflicht), Radiogruppe «Dienst»
  Radarr/Sonarr, Knopf «Integration anlegen» – `:159-184`.
- Nach Anlegen/Erneuern Panel «Adresse für «{{name}}»»: Warnung «Jetzt kopieren — danach ist die Adresse
  nicht mehr sichtbar.», Feld «Vollständige Adresse», «Adresse kopieren» (Meldung kopiert / Kopieren ging
  nicht), «Ausblenden» – `:96-119`. Antwort nicht im Mutation-Cache (`useTransientMutation`) – `:397-404`.
- Leer: «Noch keine Integration eingerichtet.»; Tabelle «Webhook-Integrationen» mit Spalten Name (Chips
  «ungespeichert», «anderswo geändert»), Dienst (Chip), Adresse («…{{tail}}»), Letzte Meldung (Datum oder
  «Hat noch nie etwas gemeldet.»), Aktionen – `:515-590`.
- Aktionen je Zeile: «Token erneuern» mit Bestätigung («Die bisherige Adresse hört sofort auf zu arbeiten …»,
  «Erneuern»), «Löschen» mit Bestätigung («… das Meldungsprotokoll verschwindet mit.») – `:423-455`.
- Aufklappbare Zeile: **Pfad-Umsetzung** (Einleitung, Zeilen «Pfad im Dienst» / «Pfad auf diesem Server»,
  «Entfernen», «Zeile hinzufügen», «Speichern»; Konflikthinweis «Ein anderer Zugang hat die
  Pfad-Umsetzung … geändert» mit «Andere Fassung übernehmen») – `:238-318`; **Meldungsprotokoll**
  «Meldungen von {{name}}» (Zeit, Ereignis, Gemeldeter Pfad, Umgesetzter Pfad, Ergebnis: «Ordner wird
  gelesen», «Test quittiert», «Ohne Wirkung», «Ausserhalb der Bibliotheken», «Ohne Pfad»; leer «Noch keine
  Meldungen.») – `:335-369`.

### 2.4 Ansicht «E-Mail-Server» (`?item=email`) – `integrations/EmailForm.tsx`
Felder: Server und Port (zwei Felder, Port validiert), Benutzer (Hilfe), Passwort (verschlüsselt, «Leer
lassen, um das gespeicherte Passwort zu behalten.»), Absender (Hilfe «Zum Beispiel Slouchcat
<media@example.ch>»), Verschlüsselung STARTTLS/TLS/Keine (Hilfe). Link «Zu den Benachrichtigungskanälen».
Aktionen: «Testmail senden» (an eigene Adresse, Erfolg/Fehler mit Detail), «Verwerfen», «Löschen …»,
«Speichern». `.env`-Hinweis sperrt – `EmailForm.tsx:61-296`.

### 2.5 Ansicht «Telegram-Bot» – `integrations/TelegramForm.tsx`
Zeile «Bot» («Bot: @name» / «Kein Bot eingerichtet.»), Feld «Bot-Token» (Platzhalter «Token gesetzt»,
Hilfe @BotFather, Prüfung bei Telegram), Speichern/Löschen über `Actions` – `TelegramForm.tsx:24-101`.

### 2.6 Ansicht «Web-Push» – `integrations/PushBlock.tsx`
Zustand «Schlüsselpaar vorhanden.» / «Noch kein Schlüsselpaar …», «Öffentlicher Schlüssel» (Anzeige),
«Schlüssel neu erzeugen» mit Bestätigung («Alle Geräte verlieren ihr Abo …») – `PushBlock.tsx:20-49`.

### 2.7 Ansicht «Server» – `integrations/ServerBlock.tsx:155-171`
Schalter «Täglich nach neuen Versionen suchen» (Update-Prüfung, sofort gespeichert), «Öffentliche Adresse»
(Feld, URL-Prüfung, Speichern/Löschen), «Grafikchip für das Umwandeln» (`HardwareChoice`, Hinweis
Neustart). ADR 0035:40: «Grafikchip-Wahl, öffentliche Adresse und Update-Prüfung bleiben auf «Integrationen».»

### 2.8 Wo externe Schlüssel sonst eingegeben werden
- Ersteinrichtung `web/src/features/setup/ServicesStep.tsx:10, 136-174` – dieselbe `ProviderCard`
  (empfohlen/optional), dazu E-Mail und Telegram.
- Hörbuch-Quellen `web/src/features/admin/AudiobookSourcesPage.tsx:344-428` – zeigt Zustand des
  Google-Books-Schlüssels, verlinkt auf `serviceHref('keys')`.
- Umgebung (`.env`) gewinnt immer: `SLOUCHCAT_TMDB_API_KEY`, `…_TVDB_API_KEY` (+PIN), `…_YOUTUBE_API_KEY`,
  `…_FANART_API_KEY`, `…_GOOGLEBOOKS_API_KEY`, `…_TRAKT_CLIENT_ID`, SMTP_*, `TELEGRAM_BOT_TOKEN`,
  `PUBLIC_URL` – `server/internal/admin/admin.go:36-58`, `admin/settings.go:44-45`, `config/config.go` (env-Liste).
- Spotlight-Seite: Takt von `trakt.refresh` (6–48 h, `admin/spotlight.go:28-29`) – zweite Tür zum Zeitplan (ADR 0017:65).

## 3. Ausgehende HTTP-Aufrufe

**[gelesen]**

- **`netguard`** (`server/internal/netguard/netguard.go`): Zweck ausdrücklich Abruf von URLs, die ein
  **Feed** nennt (Herausgeber wählt sie, nicht der Admin) – `:1-6`. `Local` sperrt Loopback, privat
  (10/8, 172.16/12, 192.168/16, fc00::/7), Link-local, Multicast, unspezifiziert, CGNAT 100.64/10 (auch
  Tailscale), 198.18/15, 0/8, NAT64/6to4/site-local – `:29-54`. `RefuseLocal` als Dialer-`Control` prüft
  **nach** DNS-Auflösung jede Verbindung inkl. Weiterleitungen – `:70-82`. **Ja: `http://192.168.1.x:5055`
  würde über einen geschützten Client mit `ErrRefused` abgewiesen.**
- **Erlaubter LAN-Weg heute:** `netguard.Clients` hat zwei Clients `public` (geschützt) und `local`
  (ungeschützt) – `:84-111`; `For(ctx, feedURL, origin)` gibt `local`, wenn der Host der **vom Admin
  abonnierten** Feed-URL selbst lokal ist; aus einer Datei gelesene Feeds (`FeedOriginFile`) nie –
  `:113-128`; ADR `docs/adr/0007-audio-modell.md:45`. Weitere ungeschützte, vom Betreiber gesetzte Ziele:
  - SMTP-Server: eigener `net.Dialer` ohne Guard, Timeout 30 s – `server/internal/notify/email/email.go:55-56, 124-130`.
  - Updater-Begleiter: `SLOUCHCAT_UPDATER_URL` (internes Docker-Netz), Client mit Timeout und
    **ohne Weiterleitungen** («A redirect would carry the token elsewhere») – `admin/update_install.go:107-113`,
    `config/config.go:103-106, 417-420`.
  - TMDB/TVDB/Trakt/YouTube/Update-Manifest: feste öffentliche Basis-URL, einfacher `http.Client{Timeout}`
    ohne Guard (TMDB 15 s – `metadata/tmdb.go:27, 62`; Trakt – `trakt/trakt.go:242`; Update 20 s – `admin/update.go:103-106`).
- **Gegenbeispiel:** eigene Radiosender lehnen LAN-Adressen ab, auch wenn der Admin sie einträgt
  (`local_address`, F-15) – `server/internal/radio/own.go:31-37, 57-81`; ADR 0025:217. Web-Push immer
  geschützt (Endpunkt wählt der Browser) – `notify/push/push.go:164`.
- **`fetch`**: nur Lader für Podcast-Enclosures/Offline (Resume, ≤ 2 GiB, Header-Timeout 30 s, ≤ 5
  Weiterleitungen, nur http/https), Fehler `ErrUnavailable`/`ErrTooLarge` – `server/internal/fetch/fetch.go:1-57`.
  Kein allgemeiner HTTP-Client.
- **`ratelimit`**: Token-Bucket je Anbieter; `Wait` blockiert, `Allow` nicht; `Do` wiederholt nach 429
  höchstens 3-mal mit `Retry-After`, sonst 2 s verdoppelnd bis 60 s – `server/internal/ratelimit/ratelimit.go:1-30`.
  `Quota` für Tageskontingente (YouTube) im Speicher – `ratelimit/quota.go:9-15`. Plan-Raten: TMDB ≤ 20/s,
  TVDB ≤ 5/s, Trakt ≤ 1/s – `docs/planning/03-plan-v1.md:115`.
- **Fehlerklassen/Muster:** Sentinel-Fehler wie `model.ErrTMDBUnreachable` (Netz, Timeout, 429 nach
  Wiederholung, 5xx) gegen `ErrTMDBKeyInvalid` (401) – `metadata/tmdb.go:80-84, 479-495`; Fehlertexte
  nennen nur Schema+Host (`netguard.Clean`/`Site`) – `netguard.go:130-150`. Wiederholung erledigt sonst die
  Job-Queue (Abschnitt 5), nicht der Client.

## 4. Secrets

**[gelesen]**

- **Paket `secrets`:** AES-256-GCM, Schlüssel 32 Byte in `<config>/secret.key` (0600, beim ersten Start
  atomar erzeugt, Warnung bei zu offenen Rechten), nie in der DB – `server/internal/secrets/secrets.go:1-67, 81-113`.
  Format `v1:` + base64url(nonce‖ciphertext), Version als AAD – `:115-141`. `Derive(label)` = HKDF-SHA256
  für Zweckschlüssel (stream-token, radio-token), nur im Speicher – `:143-153`.
- **Speicherung:** in der Tabelle `settings` verschlüsselt über `settingsWriter.setSecret` –
  `server/internal/admin/settings.go:630-640`; Anbieterschlüssel als Liste `providerKeys {field, setting, env}`
  – `settings.go:113-130`; weitere geheime Settings: `smtp_password`, `telegram_bot_token`,
  `vapid_private_key` – `settings.go:28-43`. Umgebung gewinnt und ist hier nicht änderbar (`EnvWinsError`) –
  `settings.go:22-27, 519-522`.
- **UI-Behandlung:** write-only; Antwort nennt nur Quelle (`env|stored|none`) und bei ≥ 12 Zeichen die
  letzten vier (`minHintKeyLength`, TVDB-PIN zeigt keinen Hinweis) – `settings.go:56, 192-212`; Plan 3.8
  Pkt. 3: «In OpenAPI `writeOnly`, nie in Antworten» – `03-plan-v1.md:234`. Webhook-Token: einmal anzeigen
  (Abschnitt 1). Sicherung ohne Schlüssel als Standard; mit Schlüssel nur passphrasegeschützt (age) – `03-plan-v1.md:234`.
- **Redaction (Regel 10):** `sensitiveKeys` (Suffixvergleich nach Kleinschreibung ohne `-`/`_`):
  password, pin, token, apikey, authorization, secret, pinhash, tokenhash, passwordhash, youtubeapikey,
  tvdbapikey, tvdbpin, verifier, clientid, smtppassword, bottoken, passphrase, vapidprivatekey, p256dh,
  authsecret, endpoint, listurl, deviceid, clientkey – `server/internal/logging/logging.go:43-56`.
  Query-Parameter `api_key|apikey|key|client_key|st|dt|rt|auth|token|access_token|secret|sig|signature` –
  `logging.go:58-61`; Pfade `/invitations/<token>`, `/webhooks/<token>` – `logging.go:115-120`.
  «apikey» deckt `X-Api-Key`/`apiKey` ab (Kommentar `:44`). Neue Schlüsselnamen gehören in Liste **und** Test (CLAUDE.md Regel 10).

## 5. Hintergrundjobs

**[gelesen]**

- **Queue (ADR 0002, gilt weiter):** Tabelle `jobs`, Zustände `queued|running|done|failed|cancelled`,
  Lease 5 min, Heartbeat 60 s, Idempotenz-Schlüssel eindeutig unter aktiven Jobs (zweites Einreihen liefert
  den aktiven Job), Debounce 2 s, Backoff 10 s·2^(n−1), Standard 5 Versuche, Priorität 0–9,
  `RaiseQueuedPriority` – `docs/adr/0002-job-queue.md:54-85`, `server/internal/jobs/queue.go:76-80`.
  `RetryLater` wartet ohne Versuch zu verbrauchen – `queue.go:39-40`. Prioritätsleiter (9 Keyframes für
  Wiedergabe … 8 `update.check` … 7 manuell … 6 `scan.path` … 3 `notify.deliver` … 0 Radiologo) und Regel
  «ein Tagesjob von Sekunden gehört über die Massenstufen» – `queue.go:11-29`. Handler müssen idempotent
  sein (doppelte Ausführung nach abgelaufener Lease) – ADR 0002:92-93.
- **Scheduler/Registry (ADR 0017):** eine Goroutine, Tick 60 s, Intervalle statt cron (`MinEvery` 5 min,
  `MaxEvery` 7 d – `jobs/scheduler.go:53-54`), Zeitfenster (über Mitternacht, Serverzeitzone), Ein/Aus,
  einmaliger Termin `once_at` (ADR 0022), verpasste Läufe werden nicht nachgeholt – ADR 0017:64-65, 130-141.
  `jobs.Type`: Name, Gruppe (`library|metadata|delivery|system`), Pool, Auslöser (`schedule|event|manual`),
  Priorität, Handler, `Run`, `Cancel` (Fachabbruch), `Every`, `StartDelay`, `Seed`,
  `OverridesPauseForPlayback`, `Windowable` – `server/internal/jobs/registry.go:15-110`.
  Pools: general, download, notify, index (1), trickplay (1), podcast (5) – `registry.go:33-52`;
  Arbeiterzahl nur `SLOUCHCAT_WORKERS` (ADR 0017:56-58).
- **Start/Pause/Abbruch:** «Jetzt ausführen» nur mit `Run`, Priorität 7, ignoriert Fenster; pausiert → 409
  `job_type_paused` – ADR 0017:67. Pause je Jobart (`job_types.paused_at`), laufende Läufe sofort zurück
  ohne Versuchsverbrauch – ADR 0017:68, `jobs/worker.go:91-151`. Abbruch je Lauf oder je Jobart
  (`scope=queued|all`), endgültig – ADR 0017:72. Fortschritt `jobs.Progress(ctx,done,total,note)`, `jobs.Skip`
  – `jobs/progress.go:42-49`; Schätzung vom Server (Messung/Median) – ADR 0017:69-70. Verlauf 30 Tage,
  `jobs.prune` täglich (`PruneInterval` 24 h) – `jobs/history.go:21-25`, ADR 0017:71.
- **Jobarten (35 Registrierungen) – [gelesen]:**
  | Zeitplan-Jobart | Vorgabe-Takt | Beleg |
  |---|---|---|
  | `scan.library` | 6 h, beim Start | `library/service.go:36`, `library/jobs.go:287-288` |
  | `library.sweep` | 1 h, beim Start | `library/service.go:37` |
  | `podcast.poll` | 5 min, beim Start (je Feed `FeedInterval` 55 min) | `metadata/podcast.go:48-52` |
  | `podcast.sync_all` | 6 h | `library/podcast_sync.go:51, 122` |
  | `trakt.refresh` | 24 h (Seed aus Spotlight 6–48 h) | `metadata/trakt.go:39` |
  | `update.check` | 24 h, 60 s nach Start | `admin/update.go:43, 114-120` |
  | `radio.catalog_sync` | 24 h (ab 04:30 lt. Text) | `radio/jobs.go:24` |
  | `radio.wikidata_sync` | 7 d (Fenster) | `radio/wikidata.go:55, 108-118` |
  | `metadata.original_language_backfill`, `people.images_backfill`, `chapters.images_backfill`, `trickplay.backfill`, `keyframes.backfill` | 7 d, ab Werk aus (`SeedOff`) | `metadata/original_language.go:50`, `people_images.go:59`, `library/chapter_images.go:55`, `trickplay_job.go:72`, `keyframes_backfill.go:31` |
  | `jobs.prune` | 24 h | `jobs/history.go:25, 102` |

  Ereignis-Jobarten: `scan.path`, `probe.file`, `probe.keyframes`, `segments.detect`, `library.relink`,
  `chapters.images`, Bibliothek löschen, `library.author_display`, `podcast.download`, `podcast.sync`,
  `trickplay.generate`, `trickplay.series`, `metadata.match`, `metadata.fill`, `metadata.vote_backfill`,
  `image.fetch`, Freigaben neu lesen, `metadata.apply`, `trailer.find`, `podcast.fetch`, `people.images`,
  `people.details`, `notify.deliver` (Pool notify, Prio 3, 5 Versuche – `notify/dispatch.go:18-23`),
  `download.prepare`, `radio.logo.fetch` – `library/jobs.go:284-320`, `metadata/metadata.go:502-540`,
  `notify/deliver.go:41`, `offline/job.go:51`.
- **Neue Jobart ergänzen (Dateien) [gelesen, Ablauf abgeleitet]:**
  1. Konstante + Handler (+ `Run` bei Zeitplan, + `Cancel` falls Fachzustand) im Dienstpaket; Registrierung in
     dessen `RegisterTypes(r *jobs.Registry)` (`r.Register(jobs.Type{…})`).
  2. Verdrahtung in `server/cmd/slouchcat/main.go:418-443` (`NewRegistry`, `…RegisterTypes(registry)`, Scheduler).
  3. Texte `admin.jobs.types.<name mit _ statt .>.{name,description,trigger[,progress]}` in `de.json`/`en.json`
     (+ `de-DE` bei ss-Wörtern) – `web/src/features/admin/jobsFormat.ts:13-20`; Vertrag hat bewusst **kein Enum**
     über `type` – `docs/planning/09-hintergrundjobs.md:453-456`. Zeile in `job_schedules` legt die Registry beim
     Start an (keine Migration nötig) – ADR 0017:112-115.
  4. Priorität in die Leiter `jobs/queue.go:11-29` einordnen; Tests mit Uhr-Mock.
- **Verwaltungsseite Jobs `/admin/jobs`** (`JobsPage.tsx`, Texte `admin.jobs.*`) [gelesen aus de.json]:
  Zusammenfassung (Laufend, Wartend, Pausiert, Fehlgeschlagen, «n geplant»); «Arbeiter» je Pool (Arbeiterzahl,
  «Stau ca.», frei, ohne Schätzung); Banner «n Jobs pausiert» (+ Wiedergabe-Ausnahme); «Läuft gerade»
  (Job, Gegenstand, Fortschritt, Seit, Restzeit; «Lauf pausieren», «Abbrechen»); Reiter nach Gruppe
  (Bibliothek, Metadaten, Zustellung, System, «Fehlschläge») mit Filter Alle/Läuft/Fehler/Pausiert; je Job
  Karte/Zeile: Zustand, Wartend + «dran in ca.», Letzter Lauf (OK/Fehler/abgebrochen/nichts zu tun),
  Nächster Lauf (in …, alle …, Fenster, Termin, aus, wartet bis), «Ausgelöst durch», «üblich …»;
  Aktionen «Jetzt ausführen», «Pausieren/Fortsetzen», «Wartende abbrechen (n)», «Alle Läufe abbrechen»;
  Dialog «Läufe von X» mit Reitern Laufend/Wartend/Geplant/Fehlgeschlagen/Erledigt/Abgebrochen (Spalten
  Gegenstand, Prio, Start, Dauer, Versuche, Fehler, Aktionen: Erneut, Abbrechen, Löschen; «Alle erneut
  versuchen», «Alle löschen»); Dialog «Zeitplan von X» (Ein/Aus, Takt 5 min…wöchentlich, Zeitfenster
  Von/Bis mit Zeitzone, Einmalig am / Termin entfernen, Hinweise Trakt/Update-Doppeltür, Speichern/Verwerfen);
  Dialog «Zeitfenster von X» für fensterbare Ereignisjobs; Abschnitt «Fehlschläge (alle Arten)» mit «Alle
  erneut versuchen», «Alle löschen …»; Bestätigungen mit Warnungen (Zustellung kommt nicht an, Scan endet
  abgebrochen, Downloads fehlgeschlagen); «Aktualisierung pausiert».

## 6. Vom Dateieingang bis «abspielbar»

**[gelesen]**

1. **Auslöser:** inotify (lokal) bzw. Poller (NFS/SMB, Intervall max(45 s, 3× Dauer)) bzw. Webhook →
   `scan.path` (Debounce 2 s, Prio 6); Vollscan alle 6 h – `library/doc.go:88-97, 143-148`;
   `03-plan-v1.md:160`.
2. **Scan (Schreibphase):** neue Datei → Film über TMDB-ID, sonst IMDb-ID, sonst (sort_title, year), sonst
   neuer Film + `metadata.match` (Prio 4) in **derselben Transaktion**; Serie über TVDB-ID …; neue Folge
   `metadata.match` Prio 3 nur, wenn die Serie schon zugeordnet ist; jede neue/geänderte Datei → `probe.file`
   – `library/doc.go:21-42`. IDs können schon aus Ordner-/Dateinamen und NFO kommen (`tmdbid`, `tvdbid`,
   `imdbid`) – `library/nfo.go:168-180`, `library/names_series.go:16-18, 237-245`.
3. **Sichtbar:** `items.available` = Blatt hat eine Mediendatei ohne `missing_since` (Trigger) –
   `migrations/00003_libraries_items.sql:6-8, 263`; `visible_items` filtert `deleted_at IS NULL`,
   `available = 1`, `COALESCE(effective_rating, 18)` gegen Profil/Konto, Bibliothekszugriff, nicht löschende
   Bibliothek – `migrations/00061_profiles_open_language.sql:84-104`. **Kein Filter auf `metadata_state`
   oder Probe**: der Titel ist nach dem Scan-Commit sichtbar (mit Namen aus der Datei), für Profile unter 18
   aber erst, wenn eine Freigabe gesetzt ist.
4. **Probe** `probe.file` (Prio 3) – `library/doc.go:68-75`. **[Vermutung]** Abspielen braucht die Probe für
   die Direct-Play/Remux-Entscheidung; nicht geprüft.
5. **Match** `metadata.match`: schreibt Anbieter-IDs, danach Hook `WithRelink` («after it wrote the provider
   ids of a film or series») – `metadata/metadata.go:344-349`, `metadata/match.go:312-330`,
   `library/relink.go:182-187`; danach Staffeln, `metadata.fill`, `trailer.find`; Bilder über `image.fetch`
   – `match.go:296-309`, `library/doc.go:55-59, 114-117`.
6. **Hook-Punkte heute:**
   - `library.WithNewEpisodes` – nur **Folgen-Items**, die ein Scan-Batch angelegt hat, nach Commit, für Voll-
     und Pfad-Scan; Timeout 30 s, losgelöst vom Scan-Kontext – `library/service.go:227-232`,
     `library/write.go:280-295, 314`; gleiches für Podcast-Folgen `metadata.WithNewEpisodes` –
     `metadata/metadata.go:301-305`. Ziel `notify.NewEpisodes` (nur Merkliste, Sichtbarkeit je Profil) –
     `notify/notify.go:138-180`, `notify/dispatch.go:35-59`; Verdrahtung `cmd/slouchcat/main.go:341, 361`.
   - **Kein Hook für neue Filme oder neue Serien** und keiner «Datei zu Titel hinzugekommen»; der Hook feuert
     **vor** dem Match (IDs evtl. noch leer).
   - `WithRelink` nach dem Match (IDs gesetzt) – heute nur für den Zwilling-Abgleich.
   - Aktivitätsprotokoll `scan_started/done/failed/aborted` (Pfad-Scans schreiben nichts) – ADR 0019:49-58.
   - Plan 3.4 Pkt. 5 nennt «Ereignis `item.available` an `notify`» – `03-plan-v1.md:164`; **im Code nicht
     vorhanden** (grep ohne Treffer).
   - **Verzögerung:** Webhook/inotify ~2 s + Scan; SMB-Poller ≥ 45 s; ohne Watcher bis 6 h; Match/Bilder
     hängen an der Queue (bei Massenimport Stunden möglich, ADR 0002:83).
7. **«Fehlend»:** `media_files.missing_since` (Index partiell seit 00052) – `00003:117`,
   `migrations/00052_media_files_missing_partial.sql:3-15`; Titel, deren Dateien 24 h fehlen, löscht
   `library.sweep` weich (stündlich) – `library/doc.go:139-141`; Rückkehr einer Datei hebt auf, Wiederkehr
   weich gelöschter Titel innert 30 Tagen über gleiche Anbieter-ID – `library/doc.go:25-28, 41-42`.
   Verlustschutz (root_missing, root_empty, > 20 %) – `library/doc.go:121-136`.

## 7. Verwaltungsrahmen (ADR 0035)

**[gelesen]** Quelle `web/src/features/admin/sections.ts`; Bereichsnamen `admin.nav.areas.*`.

| Bereich (Seitenleiste) | Seiten = Bereichsreiter (Route) |
|---|---|
| Übersicht | `/admin` |
| Medien (`media`) | Bibliotheken `/admin/libraries`; Metadaten `/admin/metadata`; Pflege `/admin/maintenance` (+ `/admin/items/*`); Podcasts `/admin/podcasts`; Radio `/admin/radio`; Hörbuch-Quellen `/admin/metadata/audiobooks` |
| Nutzer & Geräte (`people`) | Konten `/admin/accounts`; Geräte `/admin/devices`; Streams `/admin/streams` |
| Startseite (`home`) | Startseite `/admin/home-rows`; Spotlight `/admin/spotlight`; User-Empfehlungen `/admin/picks`; Sammlungen `/admin/collections`; Meine Medien `/admin/my-media`; Ankündigungen `/admin/announcements` |
| Automatisierung (`automation`) | Hintergrundjobs `/admin/jobs`; Integrationen `/admin/integrations`; Benachrichtigungen `/admin/notifications` |
| System (`system`) | Statistiken `/admin/stats`; Sicherung `/admin/backup`; Server `/admin/server` |

Belege `sections.ts:15-25, 59-91`, Pfade `web/src/app/paths.ts:15-95`. Aktuelle Seite = längster passender
Pfad – `sections.ts:112-128`. Bereichsreiter mit Zahlen, Seitenleiste zeigt nur Dringendes (Pflegefälle,
laufende Streams, fehlgeschlagene Jobs, stille Webhooks, «Update») – ADR 0035:34. Suchpalette `Strg+K`/`⌘K`,
reine Navigation – ADR 0035:36. Übersicht: Hinweisband, vier Kennzahlen, «Was benötigt Aufmerksamkeit?»
(inkl. Integrationsfehler, stille Webhooks), «Gerade aktiv», Schnellzugriffe «Direkt zu deinen Aufgaben»,
Kurzstatus Server mit Aktivität, Abfrage alle 5 s – ADR 0035:39. Server-Seite – ADR 0035:40.
Benachrichtigungs-Kanäle: Konto-Seite, in der Verwaltung unter `/admin/notifications`, für alle unter
`/settings/notifications` – `sections.ts:4-9`. Bedingung des Umbaus: «Keine bestehende Funktion, Option oder
Einstellmöglichkeit darf verloren gehen» – ADR 0035:21-22; Inventar `docs/design/admin-redesign/functional-inventory.csv` (876 Zeilen) – ADR 0035:47-51.

## 8. API-Konventionen und Migrationen

**[gelesen]**

- **Vertragsversion:** `info.version: 1.40.0` – `api/openapi.yaml:4`. Regel «Vertrag = Tag»: «jede Anhebung von
  `info.version` … wird beim Merge getaggt» – `docs/dev/HANDOVER.md:120-127`; additiv = Minor (Beispiel ADR 0017:120).
- **x-visibility** je Operation: `public|account|scoped|system|admin|owner` – `openapi.yaml:9-16`; Regel 7
  CLAUDE.md; Antworten mit `ItemRef` sind `scoped`; Vertragstest `TestContractCoversEveryOperation` –
  `server/internal/api/contract_access_test.go:494`.
- **Fehler:** RFC 9457 `application/problem+json` mit `code` (`^[a-z][a-z0-9_]*$`), `errors[]` für
  `validation_failed`, `detail` ohne Secrets – `openapi.yaml:39-41, 10651-10691`. Nicht sichtbar → 404.
- **CSRF:** Nicht-GET braucht `X-Requested-With: Slouchcat` – `openapi.yaml:33-35`.
- **Paginierung:** `offset` (≥ 0, Std. 0) / `limit` (1–100, Std. 50) als Query – z. B. `openapi.yaml:972-986`.
- **strict server vs. `http.Handler`:** JSON über generierten strict server; Binäres (Bilder, Streams) als
  `http.Handler`, im Vertrag beschrieben – `openapi.yaml:43-44`, CLAUDE.md Regel 6.
- **Migrationen:** letzte `00065_root_metadata.sql` → nächste frei **00066**; Vorlage
  `server/migrations/TEMPLATE.sql.txt:1-29` (nur vorwärts, `NO TRANSACTION`, `foreign_keys=OFF`, BEGIN/COMMIT,
  `foreign_key_check`, `foreign_keys=ON`; Backup per `VACUUM INTO`, 5 behalten); eingefrorene DB-Stände in
  `testdata/db/` (CLAUDE.md Regel 5).
- **Regel 2 technisch:** depguard erlaubt `store/systemq` nur in `visibility, admin, library, metadata, jobs,
  notify, offline` – `server/.golangci.yml:37-43`; Analyzer `tools/itemsaccess/itemsaccess.go:30-42` für SQL-Literale.
- Nächste freie Nummern [gelesen aus Listen]: ADR 0037, Konzept `docs/planning/25-…`, Entscheidungsprotokoll Frage 33.

## 9. Non-Goals und frühere Entscheide (Wortlaut)

**[gelesen]**

- **CLAUDE.md Regel 11** – `CLAUDE.md:57-59`: «Keine Nicht-Ziele bauen (… eigene Wunschverwaltung, Steuerung
  von Arr/Seerr, …).»
- **Spec Non-Goals** – `docs/spec/01-anforderungen-v1.md`:
  - `:65` «Freigabe-Anfragen von Kinderprofilen an den Hauptbenutzer.»
  - `:66` «Eigene Wunschverwaltung und ein Bereich „Wünschen“ über die Seerr-API in Slouchcat; Wünsche
    entstehen direkt in Seerr (Runde 20).»
  - `:67` «Beschaffungsstatus aus Radarr/Sonarr am Titel und in der Suche („gewünscht“, „in Beschaffung“,
    „verfügbar“) sowie die Benachrichtigung bei Verfügbarkeit (Runde 20, Kandidaten für die Ideensammlung).»
  - `:68` «Download-Fortschritt aus SABnzbd am Titel (Runde 20).»
  - `:69` «Steuerung von Radarr, Sonarr, SABnzbd aus Slouchcat heraus (Qualitätsprofile, Warteschlangen).»
  - `:35` Tabelle: «Statusanzeige, Wünschen über Seerr, SABnzbd und die Steuerung der Arr-Dienste zurückgestellt»
  - `:46` Abgrenzung: «Wünschen von Inhalten erfolgt ausserhalb von Slouchcat direkt in Seerr
    (Overseerr/Jellyseerr); Slouchcat baut weder eine eigene Wunschverwaltung noch einen Seerr-Bereich (Runde 20).»
  - `:159` «I-1 bis I-4 und I-6 (Statusanzeige, Benachrichtigung bei Verfügbarkeit, Bereich „Wünschen“ über
    Seerr, SABnzbd-Fortschritt) sind in Runde 20 entfallen … Die Nummern bleiben unbesetzt»
  - `:176, :182, :185` (Annahmen/Offene Punkte: Seerr-Einbettung entfallen), `:347-361` Nachtrag Runde 20
    («Nein. Von den Integrationen bleibt nur der Webhook-Empfang …»), Interview `:300-310` (Runde 12: «Nur
    Statusanzeige und Benachrichtigung; kein Wünschen durch Profile, keine Steuerung.»; Runde 13: «Seerr-Bereich zum Wünschen»).
  - **Veraltete Reste im Spec-Text:** E-4 nennt noch «in Beschaffung» – `:120`; A-6 nennt noch
    «Beschaffungsstatus» – `:149` (beide durch Frage 10 gestrichen, Text nicht nachgezogen).
- **Plan** – `docs/planning/03-plan-v1.md`: `:30` Non-Goal-Zusammenfassung; `:286-294` 5.1 «entfallen …
  Slouchcat bindet Seerr gar nicht an, weder über die API noch als Einbettung … AN-5 (Wunschrecht) und AN-16
  (Kontozuordnung)»; `:312-316` 5.6 «registriert keine Webhooks selbst (Steuerung ist Non-Goal) und ruft
  Radarr/Sonarr auch nicht ab — seit dem Wegfall von I-1 gibt es kein Polling mehr»; `:502-514` M9 (Kanal
  «Beschaffung» entfernen); `:588` A-6 «ab M9 ohne den Typ "Beschaffung"»; `:729, :734` offene Fragen 5/10;
  `:740-752` Nachtrag 22.10.: «Gestrichen: I-1 (Arr-Clients mit API-Schlüssel und Polling), I-2
  (Beschaffungsstatus am Titel), I-3 (Benachrichtigung bei Verfügbarkeit), I-4 (Bereich "Wünschen" über Seerr),
  I-6 …, der Suchfilter "in Beschaffung" aus E-4, AN-5, AN-16 und Entscheid 5.1 … Statusanzeige und Wünschen
  sind Kandidaten für die Ideensammlung.»
- **Entscheidungsprotokoll** – `docs/planning/04-entscheidungen.md`: `:14` Frage 5 (a) ursprünglich bestätigt
  «eigener Bereich «Wünschen» über die Seerr-API, keine Einbettung»; `:19` **Frage 10** (Vorschlag damals:
  «Profile des Hauptkontos ab FSK 12 dürfen wünschen, Gäste nicht; jedem Konto wird ein bestehender
  Seerr-Benutzer zugeordnet»; Antwort «M9 radikal verkleinert auf den Webhook-Empfang. Gestrichen: … alles
  ersatzlos und als Non-Goal in der Spec, Statusanzeige und Wünschen als Kandidaten für die Ideensammlung» …
  «(f) Der in M8 angelegte Benachrichtigungskanal «Beschaffung» wird vollständig entfernt»); `:37`, `:55`.
- **ADR 0010:** `:11-13` Zuschnitt; `:58-59` Migration 00018 entfernt Kanal `acquisition`; `:62-63`.
- **ADR 0009 Nachtrag** – `docs/adr/0009-administration-benachrichtigungen.md:60-67`: `acquisition` entfallen,
  «ohne Erzeuger wäre der Kanal eine Schaltfläche ohne Wirkung».
- **Spotlight:** S-4 «Titel ohne Bibliothekstreffer auf der Startseite zeigen? Nein, Non-Goal «eigene
  Wunschverwaltung». In der Verwaltung stehen sie trotzdem» – `docs/planning/05-plan-spotlight.md:84-85, 291, 1622`;
  `docs/adr/0011-spotlight.md:99`.
- **ADR 0008:82** «Offen: Filter «in Beschaffung» (M9)» (veraltet).
- **Vision** `docs/planning/01-vision.md:47-49` (ursprünglich Wunsch-Bereich über Seerr), `:72-73` Non-Goals, `:78`.
- **CURRENT_STATUS** `docs/dev/CURRENT_STATUS.md:5139-5140, 5177, 5210, 5377, 5425, 5509` (Historie).
- **Ideensammlung:** Regeln `docs/ideas/README.md:9-12` («Konzepte dürfen Non-Goals der Spec berühren … der
  Non-Goal fällt erst mit einem Eintrag im Entscheidungsprotokoll»); Index `:31-37` hat **keinen** Eintrag zu
  Wünschen/Statusanzeige (trotz «Kandidaten für die Ideensammlung»).
- grep-Auswertung: übrige Treffer von «wunsch/wünsch» sind Marcos Änderungswünsche (Konzepte 08–24) ohne Bezug.

**Bei Freigabe zu ändern [Vorschlag]:**
| Dokument | Änderung |
|---|---|
| `docs/planning/04-entscheidungen.md` | neue Frage 33 + Zeile im Verlauf: hebt Non-Goals Spec :66/:67 (teilweise) und Regel 11 auf, mit Abgrenzung (keine Steuerung Arr, kein SABnzbd) |
| `docs/spec/01-anforderungen-v1.md` | Non-Goals :66–:67 umformulieren («Ausnahme seit …», Muster wie :56-57 Webradio); :46 Abgrenzung; :35 Tabelle; neue I-Kriterien (neue Nummern ab I-9, I-1..I-4 bleiben unbesetzt laut :159); A-6/E-4 Reste bereinigen; ggf. :65 Kinderprofile klären |
| `CLAUDE.md` Regel 11 | «eigene Wunschverwaltung, Steuerung von Arr/Seerr» präzisieren (Wünsche an Jellyseerr erlaubt, Steuerung nicht) |
| `docs/planning/03-plan-v1.md` | 3.1 Modul-Tabelle (neues Modul oder `integrations` erweitert), 3.8 (ausgehendes Secret Jellyseerr, LAN-Ausnahme), 5.1 Nachtrag, 5.6 («einzige Verbindung … rein eingehend» relativieren), Changelog |
| `docs/adr/0010-webhook-empfang.md` | Nachtrag (Paket nicht mehr rein eingehend; falls Jellyseerr-Webhook: zweite CSRF-Ausnahme) |
| `docs/adr/0009-…` | Nachtrag, falls ein neuer Ereignistyp kommt |
| `docs/adr/0011`, `05-plan-spotlight.md` S-4 | prüfen, ob «Wünschen» aus Spotlight erlaubt wird |
| `docs/ideas/README.md` | Idee 06 eintragen (Status) |
| neue ADR 0037, Konzept `docs/planning/25-…` | Entscheide |
| `server/internal/integrations/integrations.go:5-6` Paketdoku, `de.json` `webhooks.intro` («fragt die Dienste nie von sich aus ab») | Texte stimmen danach weiter (Jellyseerr ≠ Arr), nur prüfen |
| `docs/dev/CURRENT_STATUS.md`, Release-Notes | üblich |

## 10. PWA/Offline

**[gelesen]**

- Service Worker (Workbox `generateSW`): Precache nur App-Shell («never media or API answers») –
  `web/vite.config.ts:123-131`; Laufzeit-Cache nur Bilder `CacheFirst` (`slouchcat-images`, 300 Einträge,
  30 Tage) – `:141-156`; **alles andere unter `/api` `NetworkOnly`** – `:157-161`; Navigation zu `/api/` nie
  mit Shell beantwortet – `:140`. Startseite/Spotlight-JSON kommen also nie aus dem SW-Cache, nur deren Bilder.
- React Query im Speicher: `refetchOnWindowFocus: true`, Wiederholung nur bei 5xx/Netz einmal –
  `web/src/app/queryClient.ts:21-31`; keine Persistenz (kein Persister gefunden).
- Server setzt `Cache-Control: no-store` gezielt (Probleme, Auth, Browse, Playback) – `server/internal/api/browse_handlers.go:21-32`,
  `auth_handlers.go:82`, `playback_handlers.go:93`. **[nicht geprüft]** ob jede JSON-Antwort `no-store` trägt.
- Regel 12 / Plan Prinzip 1: «Jede Sichtbarkeits-, Alters- und Rechteentscheidung fällt serverseitig …
  Offline speichert der Client nur Entscheide des Servers … und schlägt nach» – `03-plan-v1.md:38`.
- Folge: Wunschstatus kommt heute automatisch frisch (NetworkOnly); offline gibt es ihn nicht. Der Client
  darf weder «verfügbar» ableiten noch Wunschrecht/Alter prüfen.

---

## Folgen für das Wunsch-Konzept

**Einschränkungen**
1. **Non-Goal aufheben zuerst.** Spec :66/:67/:69, CLAUDE.md Regel 11, Plan 5.1/5.6, Frage 10. Ohne Eintrag im
   Entscheidungsprotokoll darf kein Agent bauen (`docs/ideas/README.md:9-12`). «Steuerung von Arr» (:69) kann
   bleiben, wenn Slouchcat nur Jellyseerr anspricht.
2. **Kinderprofile:** Non-Goal «Freigabe-Anfragen von Kinderprofilen an den Hauptbenutzer» (:65) kollidiert mit
   Wünschen aus Kinderprofilen oder einer Freigabe durch den Hauptbenutzer. Früherer Vorschlag (Frage 10):
   «Profile des Hauptkontos ab FSK 12, Gäste nicht». Wunschrecht muss der Server entscheiden (Regel 12).
3. **LAN-Aufruf:** `netguard`-geschützte Clients sperren `192.168.x.x`. Kein Grund, `netguard` zu benutzen
   (Admin gibt die Adresse ein, wie SMTP/Updater); Vorbilder für eine ausdrückliche LAN-Ausnahme: Podcast-Feed
   des Admins (`netguard.go:113-128`), Updater ohne Weiterleitungen (`update_install.go:110-113`). Gegenbeispiel
   Radio (`own.go`) zeigt: Entscheid gehört in die ADR. Empfehlung: einfacher Client, `CheckRedirect` →
   `http.ErrUseLastResponse` (Schlüssel darf nicht weitergeleitet werden), Timeout ~15 s, Fehler nur Schema+Host
   (`netguard.Clean`), `ratelimit.Limiter` je Instanz. Nur http/https.
4. **Secret:** API-Schlüssel über `settingsWriter.setSecret` (AES-GCM), Muster `providerKey` mit Env-Override
   (z. B. `SLOUCHCAT_JELLYSEERR_API_KEY`), Hinweis letzte vier Zeichen, `writeOnly`. Header `X-Api-Key` ist
   durch `apikey` redigiert; neuen Settings-Namen trotzdem in Redaction-Test. Basis-URL kann Zugangsdaten
   tragen → nur `Site()` loggen.
5. **Regel 2:** Ein neues Paket (`requests`/`seerr`) darf nicht auf `items` zugreifen. Der Abgleich «Titel jetzt
   in der Bibliothek» braucht SQL auf `items` (tmdb_id/tvdb_id, available) → in `library`, `metadata`, `notify`
   oder `visibility` legen, oder Allowlist in `.golangci.yml:37-43` + `itemsaccess` erweitern (ADR nötig).
   «Abspielbar für den Wünschenden» = über `visible_items` des Profils prüfen (Freigabe NULL zählt als 18 →
   Kinderprofil sieht den Titel erst nach dem Match mit Freigabe).
6. **Kein Hook für neue Filme.** `WithNewEpisodes` deckt nur Folgen und feuert vor dem Match (ohne IDs). Für
   «verfügbar» braucht es (a) einen neuen Hook nach dem Match (Anknüpfung `relinkQueued`/`WithRelink`,
   `metadata/match.go:312-330`, dort sind IDs gesetzt) **oder** (b) den Abgleich im Zeitplan-Job (Polling von
   `items` je offener Wunsch-ID). Plan 3.4 Pkt. 5 `item.available` existiert nicht – könnte hier entstehen.
   Achtung: Titel ohne passenden Match (falsche/keine ID) wird nie «erfüllt» → Rückfall über Jellyseerr-Status
   «available» + Zeitgrenze.
7. **Webhook liefert keine IDs.** Für eine schnellere Erkennung könnte `WebhookPayload` `movie.tmdbId`/
   `series.tvdbId` mitlesen (Vertrag additiv, `additionalProperties` schon offen), aber `integrations` darf
   die nicht gegen `items` halten; nur als Hinweis an `library`/`metadata` weitergeben. **[Vermutung]**
   Nutzlasten in `testdata/webhooks/` vorher prüfen.
8. **Benachrichtigung:** Neuer Ereignistyp heisst: `eventNames` (`notify/channels.go:23`), `EventType`-Enum im
   Vertrag (`openapi.yaml:18835-18842`), CHECK in `deliveries.event` (`migrations/00018_*.sql:26`) → Migration
   (Tabellenumbau) und Kanalwahl je Konto (A-6). Alternative ohne neuen Typ: `new_content` wiederverwenden
   (Beschreibung ändern). Sichtbarkeitsprüfung vor Versand wie 5.5. ADR 0009 Nachtrag: ein Kanal braucht einen
   echten Erzeuger.
9. **Jobs:** Zeitplan-Jobart für den Statusabgleich (z. B. Gruppe `metadata` oder `system`, Pool `general`,
   Vorgabe 15–60 min, `MinEvery` 5 min, abschaltbar, Fenster möglich), mit `Run` → «Jetzt ausführen» gratis;
   Ereignis-Jobart für das Weiterreichen eines Wunschs (Idempotenz-Schlüssel je Wunsch, 5 Versuche mit Backoff
   → Admin sieht Fehlschläge unter Jobs). Texte `admin.jobs.types.*`, Priorität in die Leiter. Jobs dürfen
   doppelt laufen → POST an Jellyseerr idempotent machen (vorher Status lesen).
10. **Vertrag:** additive Minor (1.41.0) + Tag; Zuschaueroperationen mit `ItemRef` → `scoped`; Suche nach
    fehlenden Titeln (TMDB-Treffer ohne Bibliothek) ist **kein** `ItemRef` → eigenes Schema; Admin-Operationen
    `admin`; Problemcodes wie `seerr_unreachable`, `seerr_key_invalid` nach Muster TMDB. Migration 00066 nach Vorlage.
11. **PWA:** API ist `NetworkOnly` → Status nie aus dem SW-Cache. Nicht in Offline-Daten aufnehmen. Antworten
    mit `Cache-Control: no-store` markieren (wie Browse). Kein Client-Entscheid über Wunschrecht.
12. **Verwaltung:** Nichts darf verloren gehen (ADR 0035:21-22). Neuer Dienst = siebte Kachel in `SERVICES`
    (`services.tsx:12`) mit `STATUS_KEYS` und Dashboard-Schlüssel (`IntegrationStatus.key`, heute acht), Ansicht
    `?item=seerr`; Hinweis in «Was benötigt Aufmerksamkeit?» bei Fehler. Bestehende sechs Ansichten (2.2–2.7)
    unverändert übernehmen. Wunschliste für den Admin wohl als neue Seite (Bereich «Medien» oder
    «Automatisierung») → Regel 8 Tastaturtest, `NAV_PAGES` in `admin-home.spec.ts`.

**Anknüpfungspunkte**
- `secrets`/`settingsWriter.setSecret`, `providerKeys`, `keyState` (Hinweis), `CheckKey`-Muster («Prüfen»-Knopf).
- `ratelimit.Limiter` + `Do` (429), `netguard.Clean/Site`.
- `jobs.Registry`/Scheduler (Zeitplan, Pause, Fenster, «Jetzt ausführen», Fehlschläge-UI fertig).
- `WithRelink`-Stelle in `metadata/match.go` für einen Hook «Titel zugeordnet», `visible_items` für «abspielbar».
- `notify.Emit` mit Profilpräfix und Kanälen.
- Spotlight-Fehltreffer (Trakt-Titel ohne Bibliothek, nur in der Verwaltung sichtbar, S-4) als möglicher Einstieg.
- Aktivitätsprotokoll (`kind` ohne Enum, ADR 0019:49-58) für «Wunsch gesendet/erfüllt».

## Unsicher / nicht geprüft

- Inhalt der aufgezeichneten Radarr/Sonarr-Nutzlasten (`testdata/webhooks/`) und ob sie `tmdbId`/`tvdbId` tragen.
- Ob Abspielen eine erfolgreiche Probe voraussetzt (Abschnitt 6 Pkt. 4).
- Ob alle JSON-Antworten global `Cache-Control: no-store` setzen (Middleware nicht gelesen).
- `JobsPage.tsx` nur über `de.json`-Schlüssel und Stichproben inventarisiert, nicht Zeile für Zeile; ebenso
  `ServerPage.tsx` nur über ADR 0035:40. Für ein Mockup-Inventar `docs/design/admin-redesign/functional-inventory.csv` heranziehen.
- `docs/planning/09-hintergrundjobs.md` nur gezielt (Abschnitt 5.3/5.4) gelesen.
- Genaue Liste der Ereignis-Jobarten aus Code-grep (35 Registrierungen); einzelne Namen (Bibliothek löschen,
  Freigaben neu lesen) ohne Konstantenwert belegt.
- Jellyseerr-API selbst (Endpunkte, Statuswerte, Auth-Header) wurde nicht geprüft – kein Fremdwissen im Bericht verwendet ausser als Vermutung markiert.
