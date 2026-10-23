# Abnahmebericht Slouchcat 1.0

Stand: 23. Oktober 2026, Branch `claude/m10-hardening-release`.

Dieser Bericht ist das Ergebnis von Meilenstein M10. Er sagt für **jede** der 64 Abnahmezeilen aus
`docs/planning/03-plan-v1.md` Abschnitt 7 und für **jedes** der 16 Plan-Kriterien P-1 bis P-16 aus
Abschnitt 8, womit sie belegt ist — oder welche Abweichung gilt und warum. Du sollst aus diesem
Bericht allein beurteilen können, ob du 1.0 veröffentlichen willst.

## Das Ergebnis in zwei Zeilen

**64 von 64 Abnahmezeilen und 16 von 16 Plan-Kriterien sind erfüllt** — 55 Zeilen und 13 Kriterien
ohne Einschränkung, 9 Zeilen und 3 Kriterien mit einer ausdrücklich beschlossenen Abweichung. Keine
Zeile ist ohne Beleg geblieben, und keine ist als grün gezählt worden, ohne dass dabei steht, was an
ihr fehlt.

**Die Abweichungen sind der Teil, den du lesen solltest** (Tabelle unten): Slouchcat ist auf keinem
Apple-Gerät und auf keinem arm64-Rechner gelaufen, AMD-Grafikchips sind ungemessen, HTTPS ist nie
mit deiner echten Domain geprüft worden, und sieben weitere Prüfungen an echten Geräten hast du am
23. Oktober abgewählt. Nichts davon ist ein Fehler, den jemand gefunden hat — es ist, was niemand
geprüft hat.

## Wie du ihn liest

Jede Zeile trägt einen von drei Ständen:

| Stand | Bedeutung |
|---|---|
| **grün** | Erfüllt und belegt. In der Spalte «Beleg» steht, womit: ein Testname, ein Messwert, eine E2E-Spezifikation oder eine durchgeführte Prüfung. |
| **grün mit Abweichung** | Der Kern ist belegt, ein benannter Teil nicht. Die Abweichung ist ausdrücklich beschlossen; die Nummer in der Spalte «Abw.» verweist auf die Liste unten. |
| **ungeprüft** | Nicht nachgewiesen. Steht hier nur, wo ein Entscheid das ausdrücklich erlaubt — sonst wäre M10 nicht fertig. |

Ein Befehl in der Spalte «Beleg» läuft aus dem Repository-Wurzelverzeichnis (`go test` aus `server/`).
Wo ein Messwert steht, findest du ihn mit Hardware und Methode in `docs/perf/`.

## Die Abweichungen, die gelten

Sie stehen hier zusammen, damit keine davon in einer Tabellenzeile untergeht. Alle sind im
Entscheidungsprotokoll `docs/planning/04-entscheidungen.md` mit Datum eingetragen.

| Nr. | Abweichung | Beschlossen | Betrifft |
|---|---|---|---|
| **A-1** | **Kein Apple-Gerät.** iPhone, iPad und Mac standen nie zur Verfügung. iOS und macOS gelten nach dem Ersatzweg von AN-7 als ungeprüft: Safari ist nie gelaufen, der iOS-Offline-Spike (AN-13) ist nur dokumentiert, Web-Push unter iOS ist nicht geprüft. | Entscheid 2, 18.10.2026 | W-3, O-3, O-4, O-6, A-6, P-6, P-9 |
| **A-2** | **VAAPI und AMD «best effort, ungetestet».** Der Weg ist gebaut (`compose.amd.yaml`), aber nie auf echter AMD-Hardware gemessen. | Plan 3.5 und 10.3 | W-2, P-6 |
| **A-3** | **Ohne Grafikchip langsamer und kleiner.** Software-Transcoding startet in bis zu 10 s statt unter 5 und liefert bei 4K-HDR höchstens 1080p-SDR. | AN-17, Entscheid 4, 19.10.2026 | W-2, W-3, P-6 |
| **A-4** | **Netzfreigaben dürfen länger brauchen**, wo kein Webhook meldet. Die Eine-Minute-Zusage von B-4 hängt am eingerichteten Webhook. | AN-12, Entscheid 3, 18.10.2026 | B-4, P-4 |
| **A-5** | **Entzug wirkt erst beim nächsten Serverkontakt.** Ein dauerhaft offline gehaltenes Gerät ist nicht sperrbar. | AN-4, Entscheid 8, 20.10.2026 | K-7, O-6 |
| **B-1** | **O-6-Langzeittest bei Tag 3 abgeschlossen** statt bei Tag 30; der Android-Teil wurde nicht durchgeführt. Belegt bleiben: kein Ablauffeld, Entfernen nur durch Löschen oder Entzug, drei Tage Offline-Wiedergabe auf Chromium und Firefox. | Entscheid 23.10.2026 | O-6, P-9 |
| **C-1** | **Keine weiteren Gerätetests vor 1.0.** Der Auftraggeber führt die offenen Prüfungen an echten Geräten nicht durch («Wird schon funktionieren. Werde bei der nächsten Version noch Änderungen vornehmen.»). Betroffen: HTTPS mit der echten Domain (K-8), der echte Radarr-Import bis ins Ereignisprotokoll (I-8), Web-Push am Android-Handy und Telegram mit einem echten Bot (A-6), die Installation der PWA am Android-Handy (C-1), «nur WLAN» auf Android-Chrome (O-3), Flugmodus-Wiedergabe am echten Gerät (O-4), die Startzeit eines 4K-HDR-Films am Handy (W-3). | Entscheid 23.10.2026 | K-8, I-8, A-6, C-1, O-3, O-4, W-3 |
| **C-2** | **arm64 nur unter Emulation geprüft.** Es steht kein arm64-Gerät zur Verfügung. Das Image baut für `linux/arm64` und startet unter QEMU; auf echter arm64-Hardware ist es nie gelaufen. | Entscheid 23.10.2026 | P-1 |

**A-1 bis A-5** waren in der DoD von M10 von Anfang an vorgesehen. **B-1** kam am 23. Oktober dazu.
**C-1 und C-2** sind neu und gehen über die ursprüngliche Liste hinaus; die DoD in
`docs/planning/03-plan-v1.md` nennt sie seit dem 23. Oktober ausdrücklich. Ohne sie wäre M10 nicht
abgeschlossen, sondern offen.

## 1. Die 64 Abnahmezeilen

### Konten, Profile, Zugriff (K)

| ID | Stand | Abw. | Beleg |
|---|---|---|---|
| K-1 | grün | | `go test ./internal/api -run 'TestLogin\|TestContractProfileHasNoCredentials'`; `web/e2e/login.spec.ts` |
| K-2 | grün | | `web/e2e/profiles.spec.ts` (Stufen 0/6/12/16/18, PIN setzen und entfernen, Löschen mit Rückfrage); `go test ./internal/api -run TestContractLibraryRightRevoked` |
| K-3 | grün | | Vertragstest 10.2: `go test ./internal/api -run 'TestContractCoversEveryOperation\|TestContractChildProfiles\|TestContractAccountOperations\|TestContractAdminOperations\|TestContractPublicOperations'`; je Meilenstein nachgeführt, zuletzt 145 Operationen plus `receiveWebhook` |
| K-4 | grün | | `go test ./internal/auth -run 'TestPIN\|TestSelectProfile'` (Sperrkurve AN-11 mit Uhr-Mock); `web/e2e/profiles.spec.ts`, `web/e2e/offline-pin.spec.ts` |
| K-5 | grün | | `go test ./internal/playback -run 'Watch\|TestReportDedupesAndBoundsPlayedTime'`; `web/e2e/watch-limit.spec.ts`, `web/e2e/offline-watch-limit.spec.ts` |
| K-6 | grün | | `web/e2e/invite.spec.ts`, `web/e2e/access.spec.ts`; `go test ./internal/auth -run TestInvitation` |
| K-7 | grün | A-5 | `go test ./internal/api -run TestContractGuestRevocation` (Median 1.2 ms bis zum Schliessen), `docs/perf/M2.md` 2; offline `web/e2e/offline-revocation.spec.ts` |
| K-8 | grün mit Abweichung | C-1 | Die Zeile hat zwei Hälften. **Automatisiert, belegt:** der Vertragstest 10.2 läuft direkt gegen die API ohne Oberfläche (`go test ./internal/api -run TestContract`). **Manuell, teilweise:** HTTPS ist gegen die interne CA von Caddy geprüft — HTTP/2 200, `Secure`-Cookie, Range 206, Weiterleitung 308 (`docs/dev/CURRENT_STATUS.md`, M2-DoD), dazu `task deploy:caddy:validate`. **Nicht belegt:** ein Durchlauf mit der echten Domain und einem Let's-Encrypt-Zertifikat. |

### Bibliothek (B)

| ID | Stand | Abw. | Beleg |
|---|---|---|---|
| B-1 | grün | | `web/e2e/admin-libraries.spec.ts`; Schreibversuch im Container schlägt mit «Read-only file system» fehl |
| B-2 | grün | | `go test ./internal/library -run 'TestParseMoviePathCorpus\|TestParseSeriesPathCorpus'` — 184 Film- und 93 Serienpfade je 100 %, dazu 42 NFO-Fälle (Grenze 98 %) |
| B-3 | grün | | `go test ./internal/api -run TestContractProfileLanguage`; `docs/perf/M3.md` 2: 25 von 25 Referenzfilmen mit Poster |
| B-4 | grün | A-4 | `go test ./internal/library -run 'TestScanRenameKeepsItemAndProgress\|TestSeriesPathScanNewSeason'`; Messwerte `docs/perf/M1a.md` 1a (NFS 48.4 s) und 1b (SMB 33–37 s), `docs/perf/M9.md` 1 (Webhook 3.1 s) |
| B-5 | grün | | `web/e2e/versions.spec.ts`; `go test ./internal/visibility -run TestReaderVersionsPrimaryFirst` |
| B-6 | grün | | `go test ./internal/library -run TestManualMatchSurvivesScansAndRefresh` (drei Voll-Scans und ein Refresh) |
| B-7 | grün | | `web/e2e/extras.spec.ts`; `go test ./internal/library -run TestExtrasBecomeExtraItemsAndSamplesAreSkipped` |
| B-8 | grün | | `docs/perf/M3.md` 2: 25 von 25 Referenzfilmen mit je einem de- und einem en-Trailer (Grenze 90 %); `go test ./internal/metadata -run TestTrailersRefreshKeepsManualRows` |
| B-9 | grün | | `web/e2e/maintenance.spec.ts`; `go test ./internal/admin -run TestAdminMaintenance` (alle sechs Fehlerarten) |

### Hörbücher und Podcasts (H)

| ID | Stand | Abw. | Beleg |
|---|---|---|---|
| H-1 | grün | | `go test ./internal/library -run TestScanAudiobookFixtures`, `web/e2e/audiobook.spec.ts`; `docs/perf/M6.md` 1: 88 Bücher, 10'025 Kapitel, Kapitelzahl identisch mit Audiobookshelf |
| H-2 | grün | | `go test ./internal/metadata -run 'TestQueueDuePodcasts\|TestFetchPodcast'`, `web/e2e/podcast.spec.ts`; `docs/perf/M6.md` 2 |
| H-3 | grün | | `web/e2e/audiobook.spec.ts` (zwei Browserkontexte ±2 s, Geschwindigkeit 2.0); `go test ./internal/playback -run TestReportPlaybackRate` |
| H-4 | grün | | Vertragstest 10.2 mit Audio-Operationen; `go test ./internal/api -run 'TestContractAudio\|TestContractChildProfiles'` |

### Wiedergabe (W)

| ID | Stand | Abw. | Beleg |
|---|---|---|---|
| W-1 | grün | | `go test ./internal/playback -run TestDecide`; Prozesszählung im Container: 206 in 1.7 ms, kein ffmpeg |
| W-2 | grün mit Abweichung | A-2, A-3 | Testmatrix 10.3 vollständig (`docs/perf/M4.md` 1); HDR→SDR auf NVENC und QSV gemessen, Referenzbilder am **23.10.2026 von Marco freigegeben**. `tonemap_cuda` liegt mit ΔE2000 5.87 auf Hauttönen über der Grenze 5 — am 20.10.2026 ausdrücklich angenommen, weil es gegen die Ausgangsfarben die genaueste Kette ist (ΔE 3.78). VAAPI/AMD ungetestet. |
| W-3 | grün mit Abweichung | A-1, A-3, C-1 | Serverseitig gemessen: `docs/perf/M4.md` 2 (NVENC `tonemap_cuda` 0.50 s, QSV-Hybrid 1.85 s, zwei Streams im CT 3.0 s), Spulen `docs/perf/M1b.md` 1 (langsamster Sprung 438 ms); `web/e2e/hdr.spec.ts`, `web/e2e/hls.spec.ts`. **Nicht belegt:** die Stoppuhr am echten Handy. |
| W-4 | grün | | `web/e2e/tracks.spec.ts` (Wechsel ≤ 3000 ms, SRT als `text/vtt`, PGS eingebrannt) |
| W-5 | grün | | `web/e2e/resume.spec.ts` (zwei Browser-Kontexte, ±10 s) |
| W-6 | grün | | `web/e2e/admin-streams.spec.ts`; `go test ./internal/playback -run 'StreamOverview\|Streams'` |
| W-7 | grün | | `web/e2e/admin-devices.spec.ts`; `go test ./internal/playback -run Device` |
| W-8 | grün | | `web/e2e/quality.spec.ts`; `go test ./internal/playback -run 'Limit\|Quality\|Budget\|GuestStreamCap'` |
| W-9 | grün | | `docs/perf/M4.md` 3: generierte Staffel 5 von 5, echte Staffel 8 von 9 Intros ±2 s = 89 % (Grenze 80 %); `go test ./internal/segments` |
| W-10 | grün | | `web/e2e/trailer.spec.ts` (`youtube-nocookie.com`, offline ausgeblendet) |

### Entdecken (E)

| ID | Stand | Abw. | Beleg |
|---|---|---|---|
| E-1 | grün | | `go test ./internal/discover -run TestHome`; `web/e2e/home.spec.ts` |
| E-2 | grün | | `go test ./internal/visibility -run TestPicks`; `web/e2e/admin-picks.spec.ts` |
| E-3 | grün | | Vertragstest 10.2: `go test ./internal/api -run TestContractChildProfiles` |
| E-4 | grün | | `go test ./internal/visibility -run TestSearch`, `web/e2e/search.spec.ts`; `docs/perf/M7.md` 1a (50'570 Titel, p95 102.0 ms) und `docs/perf/M10.md` 3 (Referenz-NAS, p95 8.1–54.2 ms). Der Filter «in Beschaffung» ist mit dem M9-Zuschnitt entfallen. |
| E-5 | grün | | `go test ./internal/discover -run TestWatchlist`; `web/e2e/watchlist.spec.ts` |
| E-6 | grün | | `go test ./internal/metadata -run TestMatchCollection`; `web/e2e/admin-collections.spec.ts` |
| E-7 | grün | | `go test ./internal/metadata -run 'TestTraktSchedule\|TestTraktRefresh'`, `go test ./internal/visibility -run 'TestTraktRowIntersection\|TestTraktRowAtMostTwenty'`; `docs/perf/M7.md` 4. Die Matrixzeile verlangt den Mock, und der läuft; ein echter Abruf ist zusätzlich gemessen. |
| E-8 | grün | | `web/e2e/notifications.spec.ts` (30.6 s bzw. 31.1 s bis zum Zähler, Grenze 2 min); `docs/perf/M7.md` 3 |
| E-9 | grün | | `go test ./internal/api -run TestContractHomeRows`; `web/e2e/admin-home-rows.spec.ts` |

### Offline (O)

| ID | Stand | Abw. | Beleg |
|---|---|---|---|
| O-1 | grün | | `web/e2e/downloads.spec.ts`, `web/e2e/audio-download.spec.ts`; `docs/perf/M5.md` 2 und `docs/perf/M6.md` 3, alle Grössen innerhalb ±10 % |
| O-2 | grün | | `web/e2e/season-download.spec.ts` (Staffel mit 6 Folgen, eine Aktion) |
| O-3 | grün mit Abweichung | A-1, C-1 | `web/e2e/auto-download.spec.ts`, `web/e2e/podcast-auto-download.spec.ts`; `go test ./internal/offline -run 'TestAutoDownloads\|TestPodcastAutoDownloads'`. **«Nur WLAN»** ist nur gegen den Vitest-Mock von `navigator.connection` belegt, nicht auf Android-Chrome. |
| O-4 | grün mit Abweichung | A-1, C-1 | `web/e2e/offline-playback.spec.ts` (Chromium und Firefox offline, englische Tonspur, zwei WebVTT-Spuren, Spulen); `docs/perf/M5.md` 1. **Nicht belegt:** der manuelle Flugmodus-Teil am echten Gerät. |
| O-5 | grün | | `web/e2e/offline-playback.spec.ts` (258–263 ms bis zur Position auf dem zweiten Gerät, Grenze 10 s); `go test ./internal/playback -run TestReportOffline` (AN-10-Konfliktfälle) |
| O-6 | grün mit Abweichung | B-1, A-1, A-5 | `go test ./internal/offline -run TestDownloadsHaveNoExpiryColumn` (kein Ablauffeld); Entfernen nur durch Löschen oder Entzug: `go test ./internal/offline -run TestTombstonesThroughCascades`, `web/e2e/offline-revocation.spec.ts`. Langzeittest `docs/perf/M5-o6-langzeittest.md`: Tag 0, 1 und 3 auf Chromium und Firefox bestanden. **Nicht belegt: die 30-Tage-Beobachtung; Android wurde nie gestartet; iOS ist ungeprüft.** |
| O-7 | grün | | `web/e2e/offline-profiles.spec.ts` (P6 sieht den FSK-18-Download von P18 nicht, Speicheranzeige ±5 %, iOS-Hinweis) |

### Client und Vertrag (C)

| ID | Stand | Abw. | Beleg |
|---|---|---|---|
| C-1 | grün mit Abweichung | C-1 | Offline-Kaltstart im Flugmodus: Chromium 1334/1357 ms, Firefox 1446/1427 ms (Grenze 3 s), `docs/perf/M5.md` 5 und `docs/perf/M5-o6-langzeittest.md`; Manifest und Service Worker in `web/vite.config.ts`, `web/e2e/offline-cold-start.spec.ts`. **Der Lighthouse-Teil «installierbar» fehlt:** Lighthouse 12 kennt diese Prüfung nicht mehr, und die Installation am Android-Handy wurde nicht durchgeführt. |
| C-2 | grün | | `task lint` mit 0 Verstössen in jeder `task ci`-Zeile: `color-no-hex`, `declaration-strict-value`, `eslint-plugin-i18next`, `no-restricted-imports`, `depguard`, `forbidigo`, Analyzer `itemsaccess` |
| C-3 | grün | | Vitest `locales.test.ts` (0 fehlende Schlüssel de/en); `web/e2e/profiles.spec.ts` (Sprachwechsel ohne Neuladen) |
| C-4 | grün | | Tastatursuite je Seite, Helfer `web/e2e/keyboard.ts`; vollständige Seitenliste unter P-15 |
| C-5 | grün | | Codegen-Drift 0 über `task gen:check` in jeder `task ci`-Zeile. Fremd-Client-Test: `task apicheck`, **18 von 18 Zuschauerfunktionen ok** (siehe P-16). |

### Administration (A)

| ID | Stand | Abw. | Beleg |
|---|---|---|---|
| A-1 | grün | | `go test ./internal/admin -run 'TestSetupSystemInfo\|TestCheckSetupPath\|TestCheckSetupTMDBKey'`, `web/e2e/setup.spec.ts`. **GPU-Erkennung auf beiden Zielen gemessen:** NVENC/RTX 4070 am 23.10.2026 von Marco am laufenden Container bestätigt, **QSV auf dem Referenz-NAS CT 190** in `docs/perf/M10.md` 4 (`backend=qsv tonemapper=cpu budget=8`). |
| A-2 | grün | | `web/e2e/admin-home.spec.ts` (jede Admin-Seite über eine Navigation erreichbar); 155 Storybook-Screenshots mit Schwelle 0.1 % über `task web:storybook:test` |
| A-3 | grün | | `web/e2e/admin-home.spec.ts` (provozierter Fehler < 10 s); `docs/perf/M8.md` 1 (p95 225 ms während des Erst-Scans). Die Kachel «Webhooks» aus M9 füllt den Integrationsstatus, nachdem I-1 entfallen ist. |
| A-4 | grün | | `go test ./internal/admin -run 'TestRestoreOnFreshStack\|TestBackupArchiveHasOnlyDatabaseAndManifest\|TestBackupWithoutSecretsStripsEncryptedSettings\|TestBackupWithSecretsNeedsPassphrase'`; `docs/perf/M8.md` 2 (Neustart in derselben PID, 1.5–1.9 s, gleiche Tabellen-Hashes) |
| A-5 | grün | | `go test ./internal/admin -run TestUpdateCheck`; `web/e2e/admin-integrations.spec.ts` (Mock-Manifest 99.0.0 → Hinweis, Prüfung aus → kein Hinweis). Das echte Manifest entsteht seit M10 bei jedem Release (`.github/workflows/release.yml`). |
| A-6 | grün mit Abweichung | A-1, C-1 | Alle vier Kanäle gegen Mocks: `go test ./internal/notify/...`, `web/e2e/notification-channels.spec.ts`. Echtes Gerät: **E-Mail am 23.10.2026 von Marco bestätigt** (Testnachricht über sein SMTP-Konto). Vier statt fünf Ereignistypen seit M9. **Nicht belegt:** Web-Push am Android-Handy, Telegram mit einem echten Bot; Web-Push unter iOS ungeprüft. |
| A-7 | grün | | `web/e2e/announcements.spec.ts`, `web/e2e/admin-announcements.spec.ts`; `go test ./internal/notify -run TestAnnounce` |
| A-8 | grün | | `go test ./internal/admin -run TestStats` (Fixture gegen eigene Aggregation); `docs/perf/M8.md` 1 (p95 1'333 ms bei 1 Mio. Ereignissen) |
| A-9 | grün | | Log-Scan über einen kompletten E2E-Lauf: 0 Treffer bei 13 eingeschleusten Fake-Secrets über 56'706 Logzeilen, Gegenprobe schlägt an (`web/e2e/log-scan.ts`); `go test ./internal/logging` |

### Integrationen (I)

| ID | Stand | Abw. | Beleg |
|---|---|---|---|
| I-5 | grün | | `web/e2e/sidecar.spec.ts` (30.5 s Chromium, 30.8 s Firefox, Grenze 60 s) |
| I-7 | grün | | `go test ./internal/api -run TestContractNoTraktPerProfile` (kein Trakt-Konto je Profil); `go test ./internal/metadata -run TestTraktRowsGoWithoutClientID` |
| I-8 | grün mit Abweichung | C-1 | `go test ./internal/api -run TestWebhookHandlesTheRecordedRadarrAndSonarrPayloads` mit **echten aufgezeichneten Nutzlasten** aus Marcos Radarr 6.4.4.10685 und Sonarr 4.0.20.3014 (je sechs Meldungen); `go test ./internal/integrations/`; `web/e2e/admin-webhooks.spec.ts`; `docs/perf/M9.md` 1 (Titel nach 3.1 s sichtbar, 0 `scan.library`-Jobs), 3 (11. Anfrage 429). Die Verbindung von Radarr zu Slouchcat steht (Abnahme 23.10.2026). **Nicht belegt:** ein echter Import bis zum Eintrag im Ereignisprotokoll samt Pfad-Umsetzung. |

## 2. Die 16 Plan-Kriterien

| ID | Stand | Abw. | Beleg |
|---|---|---|---|
| P-1 | grün mit Abweichung | C-2 | `healthy` auf dem Entwicklungsrechner 1.130–1.264 s, auf dem **Referenz-NAS CT 190 1.200–1.221 s** (`docs/perf/M10.md` 1a), unter QEMU/arm64 1.32–1.44 s — alle unter 5 s. Beide Plattformen werden in jedem CI-Lauf gebaut und gestartet (Job `image`). **Nicht belegt:** echte arm64-Hardware. |
| P-2 | grün | | **Leerlauf-RSS 83.0 MB bei 50'570 Titeln auf dem Referenz-NAS** (`docs/perf/M10.md` 1b), Grenze 150 MB bei 5'000; Image 507 MB, Grenze 600 MB |
| P-3 | grün | | Vertragstest 10.2 in jedem Meilenstein ab M2 grün; `TestContractCoversEveryOperation` lässt jede Operation ohne Registry-Eintrag fehlschlagen |
| P-4 | grün | A-4 | `go test ./internal/library -run TestLossProtection` (fehlende Wurzel, leere Wurzel, über 20 % Verlust); Messwerte `docs/perf/M1a.md` 1/1a/1b, `docs/perf/M9.md` 1, `docs/perf/M10.md` 2 |
| P-5 | grün | | Prozesszählung im Container (kein ffmpeg bei Direct Play); `go test ./internal/playback -run TestDecide`; `depguard` hält `transcode` aus `api` und `playback` heraus |
| P-6 | grün mit Abweichung | A-1, A-2, A-3 | **(a) Referenzbilder am 23.10.2026 von Marco freigegeben**, Regression SSIM ≥ 0.95 an 10 Testframes läuft (`testdata/hdr-quality.sh`). (b) ΔE2000: QSV-Hybrid 0.00, NVENC `libplacebo` 2.91/3.71, `tonemap_cuda` 3.22 und **5.87 auf Hauttönen** — über der Grenze 5, am 20.10.2026 angenommen. Start < 5 s serverseitig gemessen, nicht am Handy. |
| P-7 | grün | | `docs/perf/M1b.md` 1: langsamster Sprung 438 ms (Firefox/Software), Remux 30–254 ms, echter Film 557–960 ms; Grenze 3 s |
| P-8 | grün | | `web/e2e/resume.spec.ts`, `web/e2e/audiobook.spec.ts`, `web/e2e/offline-playback.spec.ts` (258–263 ms, Grenze ±10 s) |
| P-9 | grün mit Abweichung | B-1, A-1 | `web/e2e/offline-playback.spec.ts`, `web/e2e/offline-cold-start.spec.ts`; Langzeitprotokoll `docs/perf/M5-o6-langzeittest.md` (Tag 0, 1, 3). **Prüfdauer 3 statt 30 Tage, Android nicht durchgeführt, iOS-Teil entfällt mangels Gerät.** |
| P-10 | grün | | Reihen: `go test ./internal/api -run TestContractHomeRows`, `web/e2e/home.spec.ts` — Karussell plus sieben Reihen, keine leere (`GET /home` liefert acht). **LCP am 23.10.2026 neu gemessen** (Lighthouse 12.8.2 Mobile, simuliert, Median aus drei Läufen): **1.802 s**, Grenze 2.5 s. Die erste Messung ergab 2.702 s; Ursache war eine Regression in `deferStartScript()`, die in M10 behoben wurde (`docs/perf/M10.md` 8). |
| P-11 | grün | | Entwicklungsrechner (`docs/perf/M7.md` 1a/1b): Suche p95 102.0 ms, Startseite p95 160.7 ms, während des Scans 229.9 ms. **Referenz-NAS CT 190** (`docs/perf/M10.md` 3): Startseite p95 105.4 ms, während des Erst-Scans 109.8 ms, Suche p95 8.1–54.2 ms — alle unter den Grenzen 300 bzw. 200 ms. |
| P-12 | grün | | `task lint`: 0 Farbwerte ausserhalb Tokens, 0 UI-Literaltexte, 0 Operationen ohne `x-visibility`, 0 `os/exec` ausserhalb `transcode` |
| P-13 | grün | | Sicherung und Wiederherstellung identisch, automatische Sicherung vor jeder Migration, Passphrase erzwungen: `go test ./internal/store -run 'TestBackupBeforePendingMigration\|TestPruneBackupsKeepsNewestFive'`, `go test ./internal/admin -run 'TestRestoreOnFreshStack\|TestBackupWithSecretsNeedsPassphrase'`. **Migrationstest aller DB-Stände M1a → 1.0:** `go test ./internal/store -run TestFrozenStatesMigrate` über elf eingefrorene Stände (`testdata/db/M1a.sql` … `M10.sql`), dazu `TestFrozenM10StateCoversM10Data`. |
| P-14 | grün | | 0 Treffer über einen kompletten E2E-Lauf, Gegenprobe schlägt an (`web/e2e/log-scan.ts`); `docs/perf/M8.md` 5 |
| P-15 | grün | | Tastatursuite auf allen vorhandenen Seiten, Zuschauer und Admin, Helfer `web/e2e/keyboard.ts`: `profiles`, `home`, `detail`, `series`, `episode`, `player`, `tracks`, `quality`, `chapters`, `audiobook`, `audio-offline`, `search`, `watchlist`, `downloads`, `offline-pin`, `offline-playback`, `offline-cold-start`, `notifications`, `announcements`; `setup`, `admin-home`, `admin-libraries`, `admin-accounts`, `access`, `invite`, `admin-devices`, `admin-streams`, `admin-integrations`, `admin-webhooks`, `admin-picks`, `admin-home-rows`, `admin-collections`, `maintenance`, `notification-channels`, `admin-announcements`, `admin-stats`, `admin-backup`. Die Seite «Wünschen» ist mit dem M9-Zuschnitt entfallen und am 23.10.2026 aus der Liste in Plan Abschnitt 8 gestrichen. |
| P-16 | grün | | `task apicheck`: der Client in `server/cmd/apicheck` kennt den Server nur über `api/openapi.yaml` (eine depguard-Regel verbietet ihm jeden Import aus `internal/`), schlägt vor jedem Aufruf Pfad und Methode im Vertrag nach und lässt einen Aufruf scheitern, den der Vertrag nicht beschreibt. Lauf vom 23.10.2026: **18 von 18 Zuschauerfunktionen ok**, 0 fehlgeschlagen, 85 Aufrufe in 3 s, Exit-Code 0. «Wünschen» entfällt mit dem M9-Zuschnitt und steht nicht mehr im Vertrag. |

## 3. Sicherheitsreview (M10 Schritt 4)

Der vollständige Bericht steht in `docs/dev/SICHERHEITSREVIEW-1.0.md`. Geprüft wurde der ganze Baum
gegen das Bedrohungsmodell aus Plan 3.8, nicht gegen ein allgemeines.

| | |
|---|---|
| Gesamtrisiko | **mittel, kein Release-Blocker** |
| Funde | kritisch 0 · hoch 0 · **mittel 2** · niedrig 8 · Hinweis 11 |
| Geheimnisse im Repository oder in der Historie | **keine** — nichts zu wechseln |
| Geprüft und haltend | Sichtbarkeit und Altersfreigabe (alle 152 Operationen tragen `x-visibility`, alle 13 mit `ItemRef` in der Antwort sind `scoped`), Autorisierung der Medienströme, Sitzungen und Konten, der Webhook-Empfänger, Dateizugriff, externe Prozesse, Geheimnisse, Image und Betrieb, der Release-Weg |

Vor dem Tag behoben: **M-1** (Warnung beim Start und klarere Empfehlung für
`SLOUCHCAT_TRUSTED_PROXIES`), **M-2** (alle fremden Actions auf Commit-SHA gepinnt, Dependabot),
**N-8** (Provenance, SBOM und Attestation am Release-Image), **N-3** (`golang 1.27.3`) und **H-7**
(der temporäre Spike-Endpunkt ist entfernt). Die übrigen Funde sind im Bericht eingeordnet und für
1.x vorgemerkt.

## 4. Gemeinsame DoD (Plan 6.2)

| Punkt | Stand |
|---|---|
| 1. Alle Matrixzeilen des Meilensteins grün | je Meilenstein in `docs/dev/CURRENT_STATUS.md` belegt; für 1.0 siehe Abschnitt 1 dieses Berichts |
| 2. `task ci` grün, Vertragstest 10.2 grün | siehe Nachtrag (Lauf auf diesem Branch) |
| 3. Playwright-Tastaturtest je neuer oder geänderter Seite | P-15 |
| 4. Neuer eingefrorener DB-Stand, Migrationstest aller früheren Stände grün | **grün**: `testdata/db/M10.sql`, `TestFrozenStatesMigrate` über elf Stände (P-13) |
| 5. Messwerte in `docs/perf/<Mx>.md`, Gerätecheckliste | `docs/perf/M10.md`; Gerätecheckliste siehe Abweichung C-1 |
| 6. Verifier-Agent bestätigt die DoD anhand von Belegen | siehe Nachtrag |

## 5. Was 1.0 nicht kann

Zusammengefasst, damit es nicht nur in Tabellenzeilen steht:

- **Auf keinem Apple-Gerät geprüft.** iPhone, iPad und Mac standen nie zur Verfügung. Safari ist nie
  gelaufen. Besonders die Offline-Wiedergabe auf dem iPhone ist offen; das Risiko stand von Anfang an
  im Plan (Abschnitt 9) und bleibt.
- **Auf keinem arm64-Rechner gelaufen.** Das Paket baut und startet unter Emulation.
- **AMD-Grafikchips nie gemessen.**
- **Sieben Prüfungen an echten Geräten nicht durchgeführt** (Abweichung C-1). Der grösste davon:
  HTTPS mit der echten Domain. Alles andere daran ist im Browser und gegen Mocks belegt, aber kein
  Mensch hat es am Handy gesehen.
- **Der Langzeittest der Downloads lief drei Tage statt dreissig.**

## 6. Nachträge

Offen bis zum Pull Request: DoD 6.2 Punkt 2 (`task ci` auf diesem Branch) und Punkt 6 (eigener
Verifikationsdurchgang gegen diese Belege). Beides wird hier eingetragen, bevor der Pull Request
zur Abnahme steht.

## 7. Was in M10 gefunden und behoben wurde

Zwei Dinge, die ohne diesen Meilenstein in 1.0 gegangen wären — beide waren vorher nicht als Mangel
bekannt:

1. **Die Startseite war langsamer geworden, ohne dass es jemand sah.** Der gemessene LCP lag bei
   2.702 s statt unter 2.5 s. Nicht weil die Seite gewachsen ist, sondern weil der Bündler seit
   einer Weile zwei Skript-Dateien erzeugt und die Verzögerungslogik nur die erste erwischte: Das
   eigentliche Startprogramm lag wieder im Weg des Bildes. Nach der Korrektur 1.802 s — derselbe
   Wert wie in M1a, obwohl die Seite inzwischen ein Karussell und sieben Reihen trägt.
2. **Ein temporärer Endpunkt aus dem allerersten Meilenstein** (`/spike/{variant}`, mit
   `Access-Control-Allow-Origin: *`) war immer noch im Bild. Er wäre mit dem öffentlichen 1.0
   ausgeliefert worden. Er ist entfernt.

Dazu die sieben Punkte, mit denen M10 angetreten ist (`docs/dev/CURRENT_STATUS.md`), und die fünf
Funde des Sicherheitsreviews aus Abschnitt 3.
