# Aktueller Stand

Stand: 19. Oktober 2026. M0, M1a und M1b sind in `main` gemerged (`https://github.com/SlouchCat/slouchcat`,
Pull Requests #1, #2 und #3). **M2 ist in `main` gemerged**
(Pull Request #4, gemerged am 19. Oktober 2026).

## Meilenstein M2 – Konten, Profile, Zugriff

Vorab beantwortet (Entscheidungsprotokoll, 19. Oktober 2026): Frage 5 (Grundsatzentscheide 5.1–5.3) und
Frage 6 (AN-1, AN-8, AN-11, AN-14), beide wie vom Plan vorgeschlagen. Alle fünf Schritte aus
`docs/planning/03-plan-v1.md` (M2) sind umgesetzt; der Entwurf steht in ADR 0005
(`docs/adr/0005-profilwahl-sitzung.md`). Commits mit Bereich im Titel:

| Schritt | Commit | Inhalt |
|---|---|---|
| Vorab | `docs(decisions)` `87a2758` | Antworten auf Frage 5 und 6 |
| 1 Datenmodell, Profilwahl, PIN | `feat(auth,api)` `68ce178` | Migration 00007 (Konto-Obergrenze und -Bibliotheken, `profiles.pin_hash`, `devices`, `pin_attempts`, `invitations`, `sessions` neu aufgebaut mit nullable `profile_id` und `device_id`, View `visible_items` mit `min(profile, account)`), Anmeldung ohne Profil, `selectProfile` mit PIN (argon2id) und Sperrkurve AN-11 je Profil und Gerät, Sichtbarkeitsstufe `account`, Profilverwaltung `/profiles` nur mit Hauptprofil, `admin` nur Hauptkonto plus Hauptprofil, Geräte-Cookie, `SLOUCHCAT_TRUSTED_PROXIES`, `Clear-Site-Data` und Bild-URLs mit `p=` (offener M1a-Punkt Bildcache) |
| 2 Sichtbarkeit, Vertragstest | `feat(visibility,admin)` `966291f` | System-Scope im `visibility`-Reader für die Freigabe-Übersteuerung `PUT /admin/items/{id}/rating` (Trigger vererben Serie → Staffel → Folge), Fixture mit Serienbaum und Gastkonto, Vertragstest über **alle 41 Operationen** (Registries für `scoped`, `account`, `admin`, `public`), Negativtests Freigabe-Anhebung, Gast-Entzug, Bibliotheksrecht, Profilsprache |
| 3 Einladungen, Gäste, Entzug | `feat(auth,api)` `65d9ba2` | Einladungslinks einmalig, 7 Tage, widerrufbar (nur SHA-256 gespeichert), Gastkonten mit Obergrenze und Bibliotheksliste, `GET/PATCH/DELETE /admin/guests`, Entzug beendet Streams, widerruft Sitzungen und Stream-Tokens und löscht das Konto |
| 4 Profilsprache, Akzent, Web | `feat(web)` `afd0dff` | Seiten `/profiles` («Wer schaut?» mit PIN-Dialog und Sperr-Countdown), `/settings/profiles` (Name, Stufe, Sprache, Akzent-Farbfelder, Bibliotheken, PIN), `/admin/access` (Einladungen, Gäste), `/invite/$token`; Profilwechsel wendet Sprache und Akzent ohne Neuladen an, leert Query- und Service-Worker-Bildcache; nur die erkannte Sprache im Start-Bundle (430 kB, unter M1b) |
| 5 HTTPS | `feat(deploy)` `8ad3d63` | Compose-Profil `https` mit Caddy 2.10 (`deploy/Caddyfile`, Let's Encrypt bzw. interne CA für `localhost`), `compose.https-only.yaml` (8096 nicht veröffentlicht), README-Abschnitte «HTTPS mit Caddy» und «Eigener Reverse-Proxy» (nginx, Traefik, Caddy) |
| Review-Korrekturen | `fix(server,deploy)` `a7261d1`, in `afd0dff` enthalten (Web) | Server 18 Befunde (1 kritisch, 4 wichtig, 13 klein), Web 20 Befunde (5 wichtig, 15 klein), siehe unten |

### Definition of Done (Plan M2 plus 6.2) mit Belegen

| Kriterium | Ergebnis | Beleg |
|---|---|---|
| K-1 Login E-Mail/Passwort, 401 bei falschem Passwort; Profil nur `name`, `image` optional | PASS; Schema-Test: `Profile`, `ProfileSummary`, `ProfileRequest`, `ProfileUpdate` ohne E-Mail-/Passwortfeld, `ProfileRequest` verlangt nur `name` | `go test ./internal/api -run 'TestLogin|TestContractProfileHasNoCredentials'` (401 in `auth_handlers_test.go`), `web/e2e/login.spec.ts` (Fehlermeldung) |
| K-2 Profile anlegen, ändern, löschen; Stufen 0/6/12/16/18; PIN setzen/entfernen; Bibliotheksrecht entziehen → unsichtbar | PASS | `web/e2e/profiles.spec.ts` (jede Stufe wählbar, PIN setzen und entfernen, Löschen mit Bestätigung, 11. Profil abgewiesen), `go test ./internal/api -run TestContractLibraryRightRevoked` |
| K-3 Vertragstest 10.2: 0 Lecks über alle Operationen, P0–P18 positiv und negativ | PASS: 41 Operationen (`account` 11, `admin` 12, `public` 7, `scoped` 11), Profile P0, P6, P12, P16, P18, PX (ohne Bibliothek X), Gastprofile PG und PG18 (Konto auf 12 gedeckelt), Sitzung ohne Profil; unsichtbar → 404; Operation ohne Registry-Eintrag lässt den Test fehlschlagen. Fixture: Filme und eine Serie; die weiteren Fixture-Teile aus 10.2 (Hörbuch, Podcast, Sammlung, Person, Extras, Untertitel, Seerr, SSE, Offline) folgen mit ihren Meilensteinen, die Operationen existieren noch nicht | `go test ./internal/api -run 'TestContractCoversEveryOperation|TestContractChildProfiles|TestContractAccountOperations|TestContractAdminOperations|TestContractPublicOperations'` |
| Negativtest: Serie von 12 auf 16 → Folgen für P12 sofort unsichtbar | PASS: Serie, Staffel, Folgen aus `/home` und per ID 404, laufender HLS-Stream einer Folge → 404 beim nächsten Segment, offene Range-Antwort wird beendet; zurück auf 12 → wieder sichtbar; Folge mit eigener Stufe 16 bleibt für P12 unsichtbar | `go test ./internal/api -run 'TestContractRatingChange'` |
| K-4 PIN beim Wechsel verlangt; Sperrkurve AN-11 mit Uhr-Mock | PASS: 3. Fehlversuch 30 s, dann 60 s, 120 s … bis 15 min, je Profil und Gerät der Sitzung, überlebt Neustarts; Obergrenze 20 Fehlversuche/h je Profil; Zähler verfällt nach 24 h | `go test ./internal/auth -run 'TestPIN|TestSelectProfile'`, `web/e2e/profiles.spec.ts` (Dialog, falsche PIN 403, Sperre 429 mit Countdown) |
| K-6 Einladung → Gastkonto → eigenes Profil; Gast 403 auf `admin`; nur freigegebene Bibliotheken; benutzter/abgelaufener Link → 410 | PASS | `web/e2e/invite.spec.ts`, `web/e2e/access.spec.ts`, `go test ./internal/auth -run TestInvitation`, `go test ./internal/api -run 'TestContractAdminOperations|TestInvitationFlow'` |
| K-7 Entzug → 401 auf allen Sitzungen und Stream-Tokens innert 1 s | PASS: Median 1.2 ms bis Cookie und Token 401 antworten, 1.2 ms bis die offene Range-Antwort geschlossen ist (Test bricht ab > 1 s) | `go test ./internal/api -run 'TestContractGuestRevocation' -count=5 -v`, `docs/perf/M2.md` 2 |
| K-8 HTTPS-Domain mit gültigem Zertifikat (Caddy-Profil); 10.2 ohne UI direkt gegen die API | TEILWEISE: lokal mit interner CA geprüft (HTTP/2 200 über Caddy, Login setzt Cookie mit `Secure`, Range 206, HTTP → 308); echte Domain mit Let's Encrypt nur durch Marco prüfbar; 10.2 läuft als Go-Test rein gegen die API | `task deploy:caddy:validate`, Übergabenotiz des Deploy-Agenten, `deploy/README.md` |
| C-3 Schlüsselvergleich de/en = 0 fehlende Schlüssel; Profilwechsel wechselt Sprache ohne Neuladen | PASS | Vitest `locales.test.ts`, `web/e2e/profiles.spec.ts` (de → en: Überschriften, `html lang` und Akzent ohne `reload`) |
| B-3 Titel/Beschreibung in Profilsprache, Fallback en | PASS: P6 (en) «Movie 06», P12 (de) «Film 12», Sprachwechsel wirkt ab der nächsten Anfrage; Fallback auf en im Unit-Test (der Vertragstest hat keinen Fallback-Fall) | `go test ./internal/api -run TestContractProfileLanguage`, `go test ./internal/visibility -run TestPickTextsFallback` |
| AN-8 Freigaben-Abbildung DE → AT → CH → GB → US inkl. US-TV | PASS (Tabelle aus Plan 5.7 vollständig getestet) | `go test ./internal/metadata -run TestFSK` |
| `task ci` grün (6.2 Pkt. 2) | exit 0 (eigener Lauf, 19. Oktober 2026) | gen, gen:check, lint (0 issues), Go-Tests mit `-race` (alle Pakete ok), Vitest (web 124 in 22 Dateien, api-client 4, design-tokens 2), Build, Playwright 102 (Chromium 51, Firefox 51, 12 Specs), Storybook-Screenshots 70 |
| Tastaturtest je neue Seite (6.2 Pkt. 3) | PASS: `/profiles`, `/settings/profiles`, `/admin/access`, `/invite/$token` (Pfeile, Tab, Enter, Escape, Zurück, sichtbarer Fokus, keine Fokusfalle) | `web/e2e/profiles.spec.ts`, `access.spec.ts`, `invite.spec.ts` |
| Eingefrorener DB-Stand (6.2 Pkt. 4) | `testdata/db/M2.sql` (Hauptkonto mit drei Profilen, Gastkonto, Einladungen, Übersteuerungen; 92 Zeilen); M1a und M1b migrieren weiterhin, 00007 behält Sitzungen und Wiedergabesitzungen. Für M2.sql prüft `TestFrozenStatesMigrate` vorerst nur das Laden (00007 ist die jüngste Migration; aussagekräftig ab M3) | `go test ./internal/store -run 'TestFrozen|TestMigration00007'` |
| Messwerte und Gerätecheckliste (6.2 Pkt. 5) | `docs/perf/M2.md`: PIN-Prüfung 26.0 ms, Profilwahl mit PIN 26.8 ms, ohne 0.04 ms, Login 27.3 ms, Entzug → 401 1.2 ms; Gerätecheckliste Abschnitt 4: «Profilwechsel mit PIN» Desktop Chromium und Firefox bestanden, Android ausstehend (Marco), Apple ungeprüft | `go test ./internal/auth -bench .`, `docs/perf/M2.md` 4 |
| Code-Review (separate Passage, Opus) | Server 18 Befunde, Web 20 Befunde, alle behoben oder begründet akzeptiert (siehe unten) | Commits `a7261d1`, `afd0dff` |
| Verifier-Agent (6.2 Pkt. 6) | «DoD bestätigt mit Ausnahmen» (K-8 mit echter Domain und Let's Encrypt nur durch Marco prüfbar; Gerätecheckliste Android/iPhone/iPad/Safari ausstehend bzw. ungeprüft; WebKit ungeprüft); eigene Läufe: Vertragstests mit `-race` 17 PASS, Playwright `profiles/access/invite` (Chromium) 20 von 20, Vitest 124, `task gen` ohne Diff, Entzug → 401 in 0.7–1.8 ms, Benchmarks reproduziert; sechs Berichtskorrekturen übernommen (K-1-Schema-Test ergänzt, B-3-Fallback-Beleg, Gerätecheckliste in `docs/perf/M2.md`, DB-Hinweis, Fixture-Umfang, `account` in CLAUDE.md Regel 7) | Verifier-Bericht |

### Befunde der Review-Passage und ihre Behebung

- **Kritisch (Server) – PIN-Sperre per Cookie umgehbar:** der Zähler hing am Geräte-Cookie, das jede Anfrage
  neu setzen konnte. Jetzt zählt das Gerät der Sitzung (`sessions.device_id`, beim Anmelden festgelegt, nie
  geändert) plus eine Obergrenze je Profil über alle Geräte (20 Fehlversuche/h → 15 min).
- **Wichtig (Server):** Einladungs-Tokens standen im Request-Log und im Problem-`instance` (Pfadsegment wird
  jetzt redigiert, Test); Entzug lief in der Reihenfolge Streams → Löschen (jetzt Sitzungen widerrufen →
  Streams beenden → löschen → nochmals beenden; Registry kennt Konto und Profil); gesenkte Freigaben
  (Obergrenze, Bibliotheksrecht, Übersteuerung) beendeten offene Direktstreams nicht (`RecheckVisibility`);
  `private` als vertrauenswürdige Proxys ist mit veröffentlichtem 8096 unsicher (README, Limiter gedeckelt).
- **Klein (Server):** Profilzahl in der Transaktion, verschwundenes Profil → 404, Zählerverfall 24 h,
  Migration 00007 in einer Transaktion (Vorlage ergänzt), Übersteuerung löschen reiht Metadaten-Job ein,
  leere Bibliotheksauswahl → 422, Gastgrenze kappt Profile, Geräte/Zähler nach 400 Tagen aufgeräumt,
  Einladungsantwort `no-store`, HSTS im Caddyfile, Testlücken (Fremdprofile, Orakel der Fixture).
  Akzeptiert: `email already_exists` lässt Inhaber eines gültigen Links Adressen testen.
- **Wichtig (Web):** leere oder noch ladende Bibliotheksliste weitete Rechte beim Speichern auf «alle» aus
  (jetzt kein Formular ohne bekannte Liste, gespeicherte Rechte werden nie verbreitert); Service-Worker-Bildcache
  überlebte Logout und Wechsel (wird geleert); Bearbeiten des aktiven Profils leerte den Katalog-Cache nicht;
  PIN blieb im MutationCache (gcTime 0, reset); der Client rechnete `min(profile, account)` selbst (Regel 12;
  jetzt beide Werte angezeigt, nichts umgeschrieben).
- **Klein (Web):** Countdown ab dem ersten Render gesperrt, jedes 429 mit Pause, `auth`-Query überlebt den
  Wechsel, `autoComplete="one-time-code"`, Rückkehrziel `next` nochmals geprüft (eine echte offene
  Weiterleitung), MutationCache behandelt 401/`profile_required`, Sperrdauer für Screenreader, gemeinsame
  Helfer, tote Schlüssel, Kachel-Skeletons, Locale-Split (nur erkannte Sprache im Start-Bundle), Tests
  für Cache-Leerung und Bibliothekseinschränkung.

### Abweichungen und Entscheide während M2

- **Sichtbarkeitsstufe `account`** (ADR 0005): Sitzung ohne gewähltes Profil für Wähler und Profilverwaltung;
  `scoped` ohne Profil → `403 profile_required`; `admin` nur Hauptkonto **und** Hauptprofil.
- **Profilverwaltung nur mit aktivem Hauptprofil:** Alle Profile eines Haushalts teilen das Konto-Cookie; der
  Elternteil schützt sein Hauptprofil mit einer PIN (wie die Profilsperre bei Netflix). Gäste verwalten so ihre
  eigenen Profile.
- **Profilwechsel in place** (kein neues Cookie); die Wiedergabesitzungen der Sitzung werden widerrufen.
- **`GET /libraries`** (account) kam während der Umsetzung dazu: Gäste brauchen Bibliotheksnamen.
- **Freigabe-Übersteuerung** (`PUT /admin/items/{id}/rating`) ist in M2, weil der Negativtest 10.2 sie
  verlangt; sie sperrt `own_rating` in `locked_fields`. Löschen reiht `metadata.match` ein (füllt nur Filme;
  Serien bleiben bis M3 unbewertet).
- **`X-Forwarded-For` wird ohne `SLOUCHCAT_TRUSTED_PROXIES` ignoriert** (vorher geglaubt): Verhaltensänderung,
  im README beschrieben; `X-Forwarded-Proto` entscheidet weiter über `Secure`.
- **Profilbild:** `image` bleibt im Schema optional, ein Upload ist nicht gebaut (Avatar = Initiale auf Akzent).
- **Migrations-Vorlage:** DDL zwischen `BEGIN;` und `COMMIT;`, PRAGMAs ausserhalb (00007 ist die erste).
- **Zwischenstände der Server-Commits:** Vertrag und generierter Code liegen vollständig im Commit zu Schritt 1;
  das Go-Paket `api` baut erst mit dem Commit zu Schritt 3 wieder (strict-server-Interface). Historie nicht
  umgeschrieben.
- **E2E-Infrastruktur:** der Testserver läuft mit `SLOUCHCAT_TRUSTED_PROXIES=private`, jeder Test erhält per
  `X-Forwarded-For` eine eigene Adresse (10 Versuche/min je Adresse); die 30-s-Wiederfreigabe des
  PIN-Knopfs ist nur in Vitest mit Fake-Timern geprüft.
- **Spike-Seite und `SLOUCHCAT_SPIKE_DIR` bleiben** bis zum Ende von Marcos Android-Lauf (27. Oktober 2026).

### Nicht in M2 (Plan) und offene Punkte für M3

1. K-8 mit echter Domain und Let's Encrypt durch Marco prüfen (`task deploy:up:https`, README); HTTP/3 ungeprüft.
2. Spike-Entscheid endgültig machen, Spike-Seite und `SLOUCHCAT_SPIKE_DIR` entfernen (nach dem 27. Oktober).
3. Serien im Scanner und Metadaten (M3); bis dahin bleibt eine Serie mit gelöschter Übersteuerung unbewertet.
4. Profilbild-Upload (K-1 «optional ein Bild»): Schema vorhanden, Upload offen.
5. WebKit/Safari weiterhin ungeprüft (Entscheid 2); Gerätecheckliste «Profilwechsel mit PIN» am Android-Handy
   durch Marco.
6. NVIDIA-Container-Laufzeit, Qualitätsleiter, Admin-Dashboard: unverändert aus M1b.


## Meilenstein M1b – Durchstich Teil 2: HLS, Seeking, Design

Alle sechs Schritte aus `docs/planning/03-plan-v1.md` (M1b) sind umgesetzt; Entscheid 4 (AN-17) und die
GPU-Durchreichung in den Testcontainer stehen in `docs/planning/04-entscheidungen.md`. Commits mit Bereich im
Titel (ein Commit je Schritt, die Review-Korrekturen in einem eigenen):

| Schritt | Commit | Inhalt |
|---|---|---|
| 1 HLS, Seeking | `feat(transcode)` `72e5eef` | Paket `transcode`: `Detect` (NVENC, QSV, VAAPI, Software mit Test-Encode), `NewPlan` (Transcode 4-s-Raster, Remux auf Keyframe-Grenzen ≥ 4 s aus dem Index), `Manager`/`Session` mit Spulmodell (von der Platte, warten, oder ffmpeg am Zielsegment neu starten), fMP4 über den Segment-Muxer mit durchgehender Zeitachse (ADR 0003), `hvc1` für HEVC-Remux, Pipelines je Backend als Golden-Tests, `hlsbench` |
| 2 Sitzungen, Token | `feat(playback)` `9f45bea` | Migration 00005 `playback_sessions`, Stream-Token nach 5.4 (HMAC-SHA256, HKDF-Schlüssel aus `secret.key`, Ablauf und Widerruf in der Zeile), Entscheidung Direct/Remux/Transcode nach 10.3 aus Probe und Client-Fähigkeiten, HLS-Endpunkte als `http.Handler`, `DELETE /playback/sessions/{id}`, Logout widerruft, `Referrer-Policy: no-referrer`, Player mit hls.js (ADR 0004), Playwright `hls.spec.ts` und `stream-token.spec.ts`, zwei 90-s-Testclips |
| 3 Admission Control | `72e5eef`, `9f45bea`, Messung `docs/perf/M1b.md` 3 | Kosten nach Quelle und Pipeline (4/3/2/1.5/0.5/0.25), Budget je Backend, Warten bis 10 s dann 503 `transcode_busy`, FIFO, `nice 10`, `oom_score_adj 500`; Kalibrierung auf Entwicklungsrechner (NVENC, Software) und Testcontainer (QSV, Software) |
| 4 HDR | Messung `docs/perf/M1b.md` 4 | HDR10-4K → 1080p-SDR auf NVENC (CPU-Tonemapping, Ubuntu-ffmpeg ohne `tonemap_cuda`), QSV im Testcontainer (`vpp_qsv`) und Software auf 2 und 28 Kernen; Handy-Startzeit ausstehend (Marco) |
| 5 Bilder, Design | `feat(metadata)` `c685fdc`, `feat(web)` `b328782` | Bildpipeline über ffmpeg (Original behalten, Poster w200/400/800, Backdrop w780/1280/1920, WebP + JPEG, `Vary: Accept`, Akzentfarbe per `palettegen`), Migration 00006; Profil- und Titel-Akzent als CSS-Variablen, Poster-Glow, Backdrop-Tint, Fokusbewegung überall 180 ms (auch der Fokus-Scroll), Storybook 10 mit 54 Screenshot-Baselines in `task ci`, arm64-Image (Dockerfile-Fehler `TARGETARCH` behoben), CI-Job für beide Plattformen |
| 6 Spike, Rückfall | Doku | `docs/perf/M1-ios-offline.md` (Entscheid, Android-Zeilen ausstehend), Rückfallkriterium 2.5 unten |
| Review-Korrekturen | `fix(server,web)` | 23 Befunde der Review-Passage (1 kritisch, 6 wichtig, 16 klein), siehe unten |

### Definition of Done (Plan M1b plus 6.2) mit Belegen

| Kriterium | Ergebnis | Beleg |
|---|---|---|
| HEVC-Transcode startet in Firefox in < 5 s (1080p) | PASS: Firefox 284–338 ms, Chromium 373–376 ms (720p-Clip, NVENC); echter 1080p-HEVC-Film serverseitig 362 ms (NVENC) / 514 ms (Software), im eingebetteten Chromium 445 ms; ein Firefox-Wert mit 1080p-Film fehlt (Abstand zum Budget > 10×) | `web/e2e/hls.spec.ts`, `docs/perf/M1b.md` 1 |
| Spulen auf 20/50/80 % spielt in < 3 s weiter (Transcode und Remux) | PASS: langsamster Sprung 438 ms (Firefox, Software); Remux 30–254 ms; echter Film 557–960 ms | `docs/perf/M1b.md` 1 |
| Remux-HEVC spielt in Safari | **ungeprüft** (keine Hardware, Entscheid 2); `hvc1`-Tag und CODECS-String gesetzt, HEVC-Remux in Firefox 157 geprüft (Start 87 ms) | `docs/perf/M1b.md` 1 |
| HDR-Messwerte für NAS und Entwicklungsrechner | PASS: QSV 0.8 s erstes Segment (5.5× Echtzeit), NVENC 1.1 s, Software 28 Kerne 1.6 s, Software 2 Kerne 5.1 s und nicht echtzeitfähig (AN-17) | `docs/perf/M1b.md` 4 |
| Kalibrierte Kosten und Budgets | PASS: NVENC 12 (Treiberlimit 8 Sitzungen), QSV 8, Software Kerne/2, Remux 0.5 bestätigt | `docs/perf/M1b.md` 3 |
| Stream-Token nach Widerruf → 401 innert 1 s | PASS: 0–2 ms (Chromium, Firefox); laufende Range-Antworten werden beim Widerruf abgebrochen (Review I2) | `web/e2e/stream-token.spec.ts`, `go test ./internal/playback/ -run TestRevokeEndsOpenStream`, `go test ./internal/api/ -run TestStopEndsRunningRangeResponse` |
| Spike-Bericht mit Entscheid | TEILWEISE: Desktop-Ergebnisse und AN-20-Befund ausgewertet, Entscheid vorläufig; Android-Lauf durch Marco läuft (Tag 0 bis Tag 8) | `docs/perf/M1-ios-offline.md` |
| Images amd64 und arm64 starten | PASS: amd64 `healthy` 1.19–1.25 s; arm64 unter QEMU `healthz` nach 0.6–0.8 s, `healthy` 1.3–1.4 s; echte arm64-Hardware fehlt | `docs/perf/M1b.md` 6, `task deploy:healthy-time:arm64` |
| `task ci` grün (6.2 Pkt. 2) | exit 0 | gen, gen:check, lint (0 issues), Go-Tests mit `-race`, Vitest (web 68, api-client 4, design-tokens 2), Build, Playwright 62 (Chromium und Firefox), Storybook-Screenshots 55 |
| Kinderprofil-Vertragstest 10.2 | PASS für alle scoped-Operationen inkl. der vier HLS-Binärendpunkte und `stopPlayback` mit Token-Fixtures je Profil; unsichtbar → 404, fremdes/widerrufenes Token → 401 | `go test ./internal/api/ -run TestContractChildProfiles` |
| Tastaturtest je neue/geänderte Seite (6.2 Pkt. 3) | PASS: Player (`player.spec.ts`, `hls.spec.ts`), Startseite und Detail (unverändert grün) | `task e2e` |
| Eingefrorener DB-Stand (6.2 Pkt. 4) | `testdata/db/M1b.sql` (10 Titel, Keyframe-Index, Fortschritt, beendete Wiedergabesitzung); M1a.sql migriert weiterhin | `go test ./internal/store/ -run TestFrozen` |
| Messwerte und Gerätecheckliste (6.2 Pkt. 5) | `docs/perf/M1b.md` Abschnitte 1–8 | |
| Code-Review (separate Passage, Opus) | 23 Befunde, alle behoben (siehe unten) | Commit `fix(server,web)` |
| Verifier-Agent (6.2 Pkt. 6) | «DoD bestätigt mit Ausnahmen» (Safari/iOS ungeprüft, Android-Spike und Handy-Startzeit ausstehend, arm64 nur unter QEMU, `tonemap_cuda` und VAAPI ungeprüft); eigene Läufe: `task ci` exit 0, Play bis erstes Segment 255 ms, Widerruf → 401 in 0 ms; zwei Berichtskorrekturen übernommen (Testbefehl, Firefox-1080p-Hinweis) | Verifier-Bericht, CI-Log |

### Befunde der Review-Passage und ihre Behebung

- **Kritisch – Segment-Vollständigkeit:** der Segment-Muxer von ffmpeg schreibt den Listeneintrag vor den
  Segmentdaten; die Sitzung hielt ein gelistetes Segment für fertig (beobachtet als `read … EOF` in der
  Kalibrierung, im schlimmsten Fall ein verkürztes Segment mit passender Content-Length). Jetzt gilt ein
  Segment erst als fertig, wenn seine Boxen genau am Dateiende schliessen (`moof` vorhanden, letzte Box
  `mdat`); unvollständige Dateien werden bis 2 s erneut geprüft.
- **Wichtig:** Remux ohne Keyframe-Index driftete bei langen GOPs (Index wird jetzt vor dem Start berechnet,
  20 s Limit, sonst Transcode mit `keyframes_pending` und Job mit höchster Priorität); Widerruf beendet
  auch laufende Range-Antworten (Kontext-Registry je Wiedergabe, Prüfung alle 5 s, Schreibdeadline);
  Wettlauf Stop/Start liess ffmpeg verwaist weiterlaufen (`ErrClosed` in `startLocked`, Manager-Kontext);
  ffmpeg lief bis zum Filmende voraus und ruhende Sitzungen behielten ihre Dateien (Pause nach 8 Segmenten
  Vorlauf, kalte Sitzungen nach 10 min, Sweep alle 60 s im Hintergrund); die Web-App stellte die
  Play-Anfrage bei Netzwechsel erneut (einmal je Mount, Stop je Sitzungs-ID); Start begann immer bei Segment
  0 statt an der Fortsetzungsposition.
- **Klein:** gemeinsame HDR-Regel (nur Transferkurve), I/O ausserhalb des Mutex, neueste Anfrage gewinnt
  (`ErrRestarted` → 503 `segment_restarted`), Fehlerbegrenzung je Startsegment, ENOENT → 404, Re-Check der
  Zeile nach der Zulassung, Wiederaufbau der Transcode-Sitzung nach Server-Neustart, Akzentfarbe bleibt bei
  Palettenfehler erhalten, Poster-Fallback auf M1a-w500 bis die Pipeline lief, gesperrte Poster werden aus
  dem Original skaliert, High-10/4:2:2 gehen in den Transcode, `Detect` mit 10-s-Deadline, FIFO-Zulassung,
  hls.js versucht eine Wiederherstellung, `--sc-item-accent` vererbt nicht mehr.
- **Offen gelassen (dokumentiert):** die Qualitätsleiter (nächstniedrigere Qualität bei SDR, M4); Remux
  nach einem Server-Neustart kopiert nur AAC-Audio, weil die Client-Fähigkeiten nicht gespeichert sind
  (M4 speichert Geräteprofile); `hvc1` bei HEVC mit nur In-Band-Parametersätzen ungeprüft (Safari fehlt);
  ein verspäteter Keyframe-Job rechnet den Index ein zweites Mal (harmlos); der Akzent-Retry-Zähler lebt im
  Speicher.

### Rückfallkriterium 2.5 (Ende M1b)

- **Signal 1 (Aufwand):** Ein getrenntes Aufgabenprotokoll mit Stunden gibt es nicht; die Commit-Zeiten und
  die Session-Notizen dienen als Protokoll. Mit `server` getaggter Aufwand (Wanduhr der Agenten-Sessions):
  M0 rund 1 h (18.10. 19:33–20:18), M1a rund 5 h (18.10. 21:33 – 19.10. 00:46 plus Nachträge 11:05–12:52),
  M1b rund 4 h (19.10. 13:55–17:30, vier parallele Ströme plus Review und Korrekturen). Zusammen unter einem
  Arbeitstag; die Schwelle von 26 Arbeitstagen ist um mehr als eine Grössenordnung unterschritten.
  **Signal 1: nein.**
- **Signal 2 (Ursache):** Der Review-Agent hat die 23 M1b-Befunde nach der Checkliste eingestuft: 5
  Go-spezifisch (alle Nebenläufigkeit: Stop/Start-Wettlauf, Mutex während I/O, konkurrierende Leser, ENOENT
  nach Schliessen, Zulassungsfenster), 18 nicht Go-spezifisch (ffmpeg-Verhalten, Fachlogik, Browser, CSS,
  Entwurf). Kein Befund aus Typsystem, Generics, fehlender Bibliothek, sqlc oder oapi-codegen. Alle Befunde
  wurden in **einer** Korrekturrunde behoben; keine Aufgabe brauchte mehr als zwei Runden. M1a: 12 Befunde,
  keiner Go-spezifisch, eine Runde. **Signal 2: nein.**
- **Folge:** kein Prototyp in Option A; Go bleibt (Eintrag im Entscheidungsprotokoll).

### Abweichungen und Entscheide während M1b

- **Segment-Muxer statt hls-Muxer** (ADR 0003): der hls-Muxer schneidet auf einem am ersten Paket nach dem
  Seek verankerten Raster; Remux-Grenzen wären nach einem Neustart anders als in der Playlist. Der
  Segment-Muxer mit `-copyts`, `frag_discont`, `use_editlist=0` und einer Zeitachse «Quelle + 10 s» liefert
  identische `tfdt` über Neustarts (Test ±2 ms).
- **hls.js auch für Desktop-Safari** (ADR 0004): Playwright-Chromium meldet `canPlayType(mpegurl)` «maybe»;
  der Player nutzt hls.js, wo das klassische `MediaSource` existiert, natives HLS nur auf dem iPhone
  (`ManagedMediaSource`). Folge: Desktop-Safari bekommt Tonemapping statt HDR-Durchleitung.
- **Firefox 157 dekodiert hier HEVC Main 10** (System-ffmpeg) und bekäme Remux; der Transcode-Test meldet
  deshalb nur H.264. Plan 10.3 nennt für Firefox «Transcode»; die Entscheidung folgt den gemeldeten
  Fähigkeiten, nicht dem Browsernamen.
- **NVENC auf der RTX 4070 ohne `tonemap_cuda`** (Ubuntu-ffmpeg 6.1.1): Tonemapping auf der CPU, NVDEC und
  NVENC auf der GPU. Das Image hat jellyfin-ffmpeg mit `tonemap_cuda`, aber die NVIDIA-Container-Laufzeit
  fehlt auf dem Entwicklungsrechner (sudo, `scripts/install-system-tools.sh` ergänzen).
- **GeForce-Sitzungslimit:** mehr als 8 gleichzeitige NVENC-Encoder scheitern; das Budget 12 hält das ein.
- **QSV im Testcontainer:** jellyfin-ffmpeg erkennt die UHD 630 über MediaSDK, `vpp_qsv` übernimmt
  Skalierung und Tonemapping; `Detect` protokolliert den gescheiterten NVENC-Versuch auf INFO.
- **Remux mit AAC-Encode** für DTS/AC3-Ton (Plan 10.3 Zeile 4, eigentlich M4) ist drin, weil die
  Alternative ein voller Video-Transcode wäre.
- **Storybook-Screenshots laufen im Playwright-Container** (`mcr.microsoft.com/playwright`), damit die
  Schriften auf Mint und dem CI-Runner gleich sind; einmaliger Pull 986 MB.
- **Dockerfile:** `ARG TARGETARCH=amd64` gewann über `--platform`; ein arm64-Build nahm amd64-Pakete. Behoben.
- **Zwei Agenten-Streams in gemeinsamen Dateien:** die Commits wurden mit `git hash-object`/`update-index`
  so geschnitten, dass jeder Commit baut (Prüfung in temporären Worktrees).
- **Spike-Seite und `SLOUCHCAT_SPIKE_DIR` bleiben**, bis Marcos Android-Lauf (Bestand nach 8 Tagen)
  abgeschlossen ist; Entfernen ist dann ein eigener kleiner Commit.
- **`SLOUCHCAT_HW`** (`auto|nvenc|qsv|vaapi|software`), **`SLOUCHCAT_TRANSCODE_BUDGET`** (0 = kalibrierter
  Standard), Transcode-Arbeitsbereich `<cache>/transcode` (beim Start geleert).

**Noch ausstehend (nicht gated):** Android-Lauf des Spikes (Tag 0, 1, 3, 8) und HLS-Startzeit am Handy
durch Marco; Nachtrag in `docs/perf/M1-ios-offline.md` und `docs/perf/M1b.md` 4 und 8.

## Offene Punkte für M2

1. Spike-Entscheid endgültig machen, Spike-Seite und `SLOUCHCAT_SPIKE_DIR` entfernen.
2. NVIDIA-Container-Laufzeit auf dem Entwicklungsrechner (sudo, Marco), damit `tonemap_cuda` im Image
   gemessen werden kann; VAAPI/AMD bleibt «best effort».
3. Qualitätsleiter und Geräteprofile (W-7) in M4; Safari/iOS bleiben ungeprüft bis Hardware da ist.
4. Admin-Oberfläche für Budgets und laufende Streams (W-6, M8); bis dahin Log und `SLOUCHCAT_TRANSCODE_BUDGET`.
5. Bildcache im Browser nach Logout/Profilwechsel (seit M1a offen, relevant ab M2).


## Meilenstein M1a – Durchstich Teil 1: Scan, Direct Play, Startseite

Alle fünf Schritte aus `docs/planning/03-plan-v1.md` (M1a) sind umgesetzt, Commits mit Bereich im Titel:

| Schritt | Commits | Inhalt |
|---|---|---|
| 1 Offline-Spike | `feat(spike)` | `testdata/generate-spike-media.sh` (zwei 1.54-GB-Dateien: fMP4 mit globalem `sidx`, `faststart`), Endpunkt `/api/v1/spike/{variant}` mit Range (nur mit `SLOUCHCAT_SPIKE_DIR`), Test-PWA unter `/spike/` mit beiden Schreibwegen (`createSyncAccessHandle` im Worker, `createWritable`), Wiederaufnahme, Wiedergabe per Blob-URL und per Service-Worker-Range, Spulmessung, Dauerlauf, Bestandsprüfung, Protokoll als JSON; Bericht `docs/perf/M1-ios-offline.md` |
| 2 Fundament | `feat(server)` ×2 | Vertrag mit allen Operationen und `x-visibility`; Migrationen 00002–00004 (Konten, Profile, Sitzungen, Bibliotheken, `items`/`media_files`/`tracks`/`images`, `progress`, `progress_events`), View `visible_items`, Trigger für `effective_rating`, `available`, `min_child_rating`; `secrets` (AES-256-GCM, `/config/secret.key`), `auth` (argon2id, Sitzungs-Cookie, SHA-256-Token), Assistent, Login, Session-Middleware (erzwingt `x-visibility` je Route aus dem eingebetteten Vertrag), CSRF-Header |
| 3 Scanner, Metadaten | `feat(server)` | Namensparser (184 Pfadmuster, 100 %), NFO, Datei-Identität (dev/inode/size, Teil-Hash), Scan in Batches ≤ 500 mit Löschschutz, Soft-Delete nach 24 h, Watcher (inotify lokal, adaptives Stat-Polling für NFS/SMB), Probe mit Keyframe-Index, TMDB de/en über `ratelimit` (20/s, 429-Backoff), AN-8-Freigaben, Bilder unverändert unter `/cache/images`, Auslieferung nur über `/items/{id}/images/...` mit Scope |
| 4 Startseite, Detail | `feat(server)`, `feat(web)` | `visibility` als einziger Lesepfad, Startseite (Karussell «Cat's Empfehlungen» als Platzhalter, «Weiterschauen», «Neu hinzugefügt», leere Reihen entfallen), Detailseite, Design-System-Komponenten in `web/src/ui`, Pfeiltasten-Navigation mit echtem DOM-Fokus, PNG-Icons, Service Worker |
| 5 Direct Play | `feat(server)`, `feat(web)` | Range-Stream über `paths` mit Sitzungs-Cookie, `POST /items/{id}/play`, Fortschritt alle 10 s/Pause/Ende/Verlassen, Fortsetzen, Player mit Tastatur; Assistent, Login, Admin-Bibliotheken als Seiten |

### Definition of Done (Plan M1a plus 6.2 und Abnahmematrix) mit Belegen

| Kriterium | Ergebnis | Beleg |
|---|---|---|
| Frischer Stack per Compose: Assistent → 20 generierte + 5 eigene Filme in < 5 min mit Postern | **9.5 s** (zweiter Lauf mit dem finalen Image: 10.5 s) für 20 generierte + 12 eigene (NFS) Titel, alle mit Poster und Backdrop; `healthy` nach 1.6–1.7 s | `docs/perf/M1a.md` Abschnitt 2a, Container-Log ohne Warnungen |
| Umbenennung einer Datei behält den Fortschritt | PASS | `go test ./internal/library/ -run TestScanRenameKeepsItemAndProgress`: Datei und Ordner umbenannt → gleiche `media_files.id`, gleiches Item, `progress`-Zeile bleibt |
| Abgehängte Bibliothekswurzel löscht nichts | PASS | `go test ./internal/library/ -run TestLossProtection`: fehlende Wurzel → `aborted/root_missing`, leere Wurzel → `root_empty`, 3 von 10 Dateien weg → `loss_over_threshold` ohne Änderung; nach `accept-loss` übernommen; 1 von 10 direkt |
| Direct Play startet in Chrome und Firefox in < 2 s | Chromium 43 ms, Firefox 73 ms (localhost, 10-s-Testclip); Safari **ungeprüft** (keine Hardware, Entscheid 2) | Playwright `web/e2e/player.spec.ts` liest `data-start-ms` |
| W-1: H.264/AAC-MP4 → 0 ffmpeg-Prozesse, HTTP 206 | PASS | Vertragstest prüft 206 mit `Content-Range`; im Compose-Container vor und nach `curl -r 0-1048575 …/stream` (206 in 1.7 ms) zeigt `/proc` nur den Prozess `slouchcat`, kein ffmpeg; `api` und `playback` importieren `transcode` nicht |
| Startseite LCP < 2.5 s (Lighthouse Mobile) | PASS: Median 1.80 s (1.802 / 1.802 / 1.802 s); Start 13.0 s → 3.38 s → 2.57 s → 2.55 s → 1.80 s; FCP 0.75 s, CLS 0 | `docs/perf/M1a.md` Abschnitt 4 (vier Runden: gzip, parallele Aufrufe, w780-Backdrop, Link-Preload, Hero-Bild in der Shell, JS 139 kB, Start-JS erst nach dem gezeichneten Hero-Bild) |
| Spike läuft | Testseite auf Desktop-Chrome und -Firefox durchgeprüft (Download beider Varianten und Schreibwege, Wiederaufnahme, Abbruch, Spulen < 100 ms per Blob-URL); Android-Lauf durch Marco steht aus; iOS ungeprüft | `docs/perf/M1-ios-offline.md` |
| Kinderprofil-Vertragstest 10.2 | PASS für alle 8 scoped-Operationen mit P0, P6, P12, P16, P18 und PX (ohne Bibliotheksrecht), positiv und negativ, ID-Endpunkte 404, fehlender und gelöschter Titel unsichtbar | `go test ./internal/api/ -run TestContractChildProfiles`; neue scoped-Operation ohne Registry-Eintrag lässt den Test fehlschlagen |
| K-1 Login E-Mail/Passwort, falsches Passwort → 401; Profil ohne E-Mail/Passwort | PASS | `go test ./internal/api/ -run 'TestSetupFlowEndToEnd|TestLoginFailures|TestSetupConflictsAndValidation'`; Schema `Profile` hat nur `name`, optional `image` |
| B-1 Bibliothek Typ Film und Serie anlegen; `/media` read-only | PASS | Assistent und `POST /admin/libraries` (beide Typen, `admin-libraries.spec.ts`); Schreibversuch im Container: `docker exec slouchcat touch /media/Generated/x` → `Read-only file system` (ebenso `/testfilme`) |
| B-2 Namenskorpus ≥ 150 Muster, ≥ 98 % | 184 Pfade, 100 %; 30 NFO-Fälle | `go test ./internal/library/ -run TestParseMoviePathCorpus -v` druckt die Quote |
| B-3 Titel/Beschreibung in Profilsprache, Fallback en; Poster für die Referenzfilme | PASS: 32 von 32 Titeln mit de/en-Texten und Postern | Compose-Lauf; `go test ./internal/visibility/` (Sprach-Fallback) |
| B-4 ext4: neu/umbenannt/gelöscht sichtbar < 60 s; Netzfreigaben nach AN-12 mit Messwert | inotify meldet in 2 s (inkl. Entprellung); **NFS in Referenzgrösse** (60'800 Dateien auf dem NAS): neuer Titel nach 48.4 s; **SMB in Referenzgrösse** (Proxmox-Testcontainer, Samba): nach parallelem Poller (16 Worker, 3 s je Durchlauf) und Pfad-Scan `scan.path` neuer Titel nach 33–37 s, sofern der CIFS-Client `dir_cache_timeout=0` setzt (Directory Leases, siehe `deploy/README.md`) | `slouchcat watchbench`, `docs/perf/M1a.md` Abschnitte 1, 1a, 1b |
| W-5 Fortsetzung ±10 s über zwei Kontexte | Server: `progress` je Profil und Item, `resumePositionMs` im `PlaybackInfo`; Playwright-Test mit zwei Browser-Kontexten (`web/e2e/resume.spec.ts`) | `task e2e`, `go test ./internal/playback/`, Vitest `PlayerPage.test.tsx` |
| E-1 Reihenfolge Karussell, Weiterschauen, Neu hinzugefügt | PASS | Vertragstest prüft Reihenfolge und entfallende leere Reihen |
| C-1 installierbar | TEILWEISE: Manifest mit PNG-Icons (192, 512, maskable) und Service Worker werden ausgeliefert; Lighthouse 12 hat keine Installierbarkeits-Prüfung mehr, die Installation wird mit dem Android-Lauf des Spikes manuell geprüft | `web/vite.config.ts`, `curl /manifest.webmanifest`, `/sw.js` |
| C-2 Lint 0 Verstösse; Admin und Zuschauer aus `web/src/ui` | PASS | `task lint` |
| C-4 Playwright-Tastatursuite je Seite | 54 Tests (6 Seiten plus W-5-Fortsetzung, Chromium und Firefox) grün; WebKit auf dem Rechner nicht lauffähig (fehlende Systembibliothek `libavif16`) | `task e2e` |
| A-1 frischer Stack: Konto, Bibliothek, TMDB-Schlüssel geprüft (ungültig → Meldung) | PASS; GPU-Erkennung folgt in M8 | Handler-Tests (`tmdb_key_invalid`, `tmdb_unreachable`, 409, 422 mit Feldcodes) |
| `task ci` grün (6.2 Pkt. 2) | exit 0 | gen, gen:check, lint (0 issues), Go-Tests mit `-race`, Vitest (48: web 42, api-client 4, design-tokens 2), Build, Playwright (54) |
| Eingefrorener DB-Stand (6.2 Pkt. 4) | `testdata/db/M1a.sql`, Migrationstest grün | `go test ./internal/store/ -run TestFrozen` |
| Messwerte (6.2 Pkt. 5) | `docs/perf/M1a.md`, `docs/perf/M1-ios-offline.md` | |
| Lastmessung Dateiüberwachung ≥ 45'000 Dateien, 8'000 Ordner (lokale Platte) | 60'800 Dateien in 8'213 Ordnern: Stat-Durchlauf 128–193 ms, inotify 8'213 Watches in 0.4 s, Voll-Scan 2'500 Filme in 1.09 s; zusätzlich über NFS: 3.0–10.9 s je Durchlauf, Änderung nach 48 s sichtbar | `docs/perf/M1a.md` Abschnitte 1 und 1a |
| Code-Review (separate Passage, Opus) | 0 Sichtbarkeitslecks, 2 wichtige und 10 kleinere Befunde, alle behoben (siehe unten) | Commit `fix(server): harden login, scanner and progress after review` |
| Verifier-Agent (6.2 Pkt. 6) | «DoD bestätigt mit Ausnahmen» (damals LCP, Safari/iOS, Android-Spike, NFS-Lastmessung, Abspieltest); danach LCP mit Runde 4 erfüllt und NFS gemessen, Korrekturen am Bericht übernommen, W-5-Test ergänzt | `task ci` durch den Verifier erneut: exit 0 |

Nicht in M1a (Plan): Stream-Token (M1b), Bildpipeline mit Skalierung (M1b), HLS (M1b), Serien (M3),
Profile/PIN (M2), GPU-Erkennung im Assistenten (M8).

## Was läuft

```bash
task ci                      # alles bauen und prüfen (inkl. Playwright)
task server:dev              # Server auf :8096 mit .dev/config und .dev/cache
task web:dev                 # Vite auf :5173, Proxy /api → :8096
task e2e                     # Playwright gegen server/bin/slouchcat (nach task build und testdata:media)
task deploy:build            # Image slouchcat:dev
server/bin/slouchcat watchbench --root DIR --inotify   # Lastmessung der Dateiüberwachung
server/bin/slouchcat dump --config-dir DIR             # SQL-Stand für testdata/db
./testdata/generate-spike-media.sh                     # 2 × 1.5 GB für den Spike
```

Compose-Stack für den DoD-Lauf: `deploy/` mit `.env` (TMDB-Schlüssel optional, nie committen) und der
lokalen Override-Datei `.dev/compose.m1a.yaml` (Testfilme unter `/testfilme`, Spike unter `/spike`).

### Befunde der Review-Passage und ihre Behebung

- **Login konnte den Speicher erschöpfen** (argon2id 64 MiB je Versuch ohne Limit): jetzt höchstens 4
  gleichzeitige Hash-Läufe, 10 Versuche pro Minute je Client (429 `too_many_attempts`, `Retry-After`),
  überlange Eingaben werden vor dem Hashen abgewiesen, Request-Bodys sind auf 64 KiB begrenzt.
- **Ein Symlink aus der Wurzel heraus liess jeden Scan scheitern**: wird jetzt mit Warnung übersprungen.
- Sweep löscht erst, wenn alle Dateien eines Titels 24 h fehlen; Soft-Delete-Titel werden innert 30 Tagen
  per TMDB/IMDb-ID wieder verknüpft (Plan 3.2); Titel ohne Jahr und ID werden nur im selben Ordner
  zusammengeführt; verspätete Fortschrittsereignisse überschreiben die Position nicht mehr; das
  Sitzungs-Cookie gleitet mit dem Server-Ablauf; die 10.2-Fixture hat P12, P16, ein Profil ohne
  Bibliotheksrecht sowie einen fehlenden und einen gelöschten Titel.
- Offen gelassen (dokumentiert): ffprobe folgt Symlinks, die nach dem Scan ersetzt werden (nur Metadaten);
  Bild-Cache im Browser hält Bilder nach Logout/Profilwechsel (relevant ab M2); `X-Forwarded-For` wird
  ohne Proxy dem Client geglaubt (Semaphore begrenzt den Speicher trotzdem).

## Abweichungen und Entscheide während M1a

- **iOS/Safari ungeprüft** (Entscheid 2): Spike und Safari-Zeilen sind als «ungeprüft, keine Hardware»
  dokumentiert; WebKit in Playwright braucht `sudo pnpm exec playwright install-deps` (nicht ausgeführt).
- **Spike-Befund (Firefox):** fragmentiertes MP4 über Service-Worker-Range meldet eine instabile Dauer;
  per Blob-URL spielt es sauber, `faststart` über beide Wege. Geht in den M1b-Entscheid zu AN-20 ein.
- **Android braucht einen sicheren Kontext** für OPFS, Service Worker und `persist()`: über `http://IP:8096`
  nur mit Chrome-Flag `unsafely-treat-insecure-origin-as-secure` oder `adb reverse`; beschrieben im
  Spike-Protokoll. HTTPS kommt mit Caddy in M2.
- **Trigger ohne CTE:** SQLite erlaubt kein `WITH` in Triggern und ruft Trigger nicht rekursiv; die
  Vererbung ist auf drei Ebenen ausgeschrieben (Serie → Staffel → Folge → Extra), getestet.
- **Binär-Endpunkte** (`getItemImage`, `getItemStream`, `getSpikeFile`) sind in `oapi-codegen.yaml`
  ausgeschlossen und als `http.Handler` montiert; ihre `x-visibility` steht in `api.go` und wird gegen den
  Vertrag getestet.
- **`x-visibility` zur Laufzeit:** die Middleware liest den Marker je Route aus dem eingebetteten Vertrag
  (`gen.GetSpec`); Routen ohne Marker schliessen (401).
- **Karussell-Platzhalter:** ohne `top_picks` zeigt «Cat's Empfehlungen» die neuesten Titel mit Backdrop
  (max. 6); Kuratierung in M7.
- **`genres`** ist ein JSON-Objekt `{"de": [...], "en": [...]}`.
- **Ratenbegrenzer** ist ein eigener Token-Bucket (gleiches API wie `x/time/rate`).
- **Keyframe-Index** liest die ganze Datei (nur Demux) und läuft mit niedrigster Priorität nach den
  Metadaten; bei leeren Platzhalterdateien schlägt die Probe erwartungsgemäss fehl.
- **Commit `604f6f7`** enthält versehentlich das Entfernen der alten Startseite (Web baut in diesem
  Zwischenstand nicht); der Web-Commit stellt den Zustand her. Historie nicht umgeschrieben.
- **`SLOUCHCAT_WORKERS`** (Standard 2–4 nach CPU), `SLOUCHCAT_FFMPEG`/`FFPROBE`, `SLOUCHCAT_SPIKE_DIR`
  (temporär bis M1b).

**Abspieltest durch Marco** im Browser gegen den Compose-Stack (`http://192.168.1.143:8096`): bestanden,
19. Oktober 2026 («Scheint alles zu funktionieren»).

**Noch ausstehend:** Android-Lauf des Spikes (Protokoll in `docs/perf/M1-ios-offline.md`).

## Offene Punkte für M1b (Stand M1a, erledigt bzw. verschoben)

1. Lastmessung in Referenzgrösse **über NFS/SMB** (Platzhalterbaum auf der Freigabe oder Proxmox-CT,
   Entscheid 2/3); lokal erfüllt.
2. Android-Lauf des Spikes durch Marco (Download mit Sperre, Spulen, Bestand nach 1/3/8 Tagen); Auswertung
   und Go/No-Go in M1b.
3. Stream-Token (5.4), Bildpipeline (WebP, Grössen, Akzentfarbe), HLS/Remux, Design-Feinschliff, arm64.
4. `inotify_poll`-Abweichungen und Scan-Fehler erscheinen erst mit dem Dashboard (M8) in der Oberfläche;
   bis dahin Log und `/admin/libraries`.
5. Proxmox-Testcontainer als schwaches NAS (Entscheid 2, vor M1b): CT 190 `slouchcat-nas` läuft seit
   2026-10-19 auf `pve2` (Debian 13 amd64, 2 Kerne, 4 GB, Docker 29 mit Compose, Samba-Freigabe `media`,
   Platzhalterbaum mit 60'800 Dateien unter `/srv/media/placeholder-tree`), siehe `docs/dev/TESTSERVER.md`.
   Offen: `smbpasswd -a media` im Container (Marco), SMB-Lastmessung mit `watchbench` über CIFS
   (`docs/perf/M1a.md` 1b).
6. GitHub Actions: Workflow `.github/workflows/ci.yml` läuft seit dem Push dieses Branches; Ergebnis auf
   GitHub prüfen (der Lauf braucht ffmpeg, Playwright-Browser und `task testdata:media`, alles im Workflow).
