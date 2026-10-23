# Sicherheitsreview M10, Schritt 4 — Slouchcat 1.0

**Datum:** 23. Oktober 2026 · **Umfang:** ganzer Baum, nicht nur der Diff · **Stand:** Branch
`claude/m10-hardening-release`
**Grundlage:** `CLAUDE.md`, `docs/planning/03-plan-v1.md` Abschnitt 3.3 (Sichtbarkeit und SystemScope) und 3.8 (Sicherheitsgrundsätze)

**Gesamtrisiko: MITTEL — kein Release-Blocker.**
Kein kritischer und kein hoher Befund. Der Kern — Sichtbarkeit, Altersfreigabe, Medien-Autorisierung,
Sitzungen, Pfade, externe Prozesse, Geheimnisse — hält. Die zwei mittleren Befunde sind eine
Fehlkonfigurations-Falle im Betrieb und eine Supply-Chain-Härtung am Release-Weg, beides vor dem
ersten öffentlichen Tag zu erledigen und beides in Minuten bis Stunden.

**Zahlen:** kritisch 0 · hoch 0 · **mittel 2** · niedrig 8 · Hinweis 11

**Was seither behoben ist** (im selben Meilenstein, Belege im Abnahmebericht
`docs/dev/ABNAHME-1.0.md`):

| Fund | Behoben wie |
|---|---|
| **M-1** | Der Server warnt beim Start, wenn `SLOUCHCAT_TRUSTED_PROXIES` ein ganzes privates Netz umfasst (`config.TrustedProxyWarning`, mit Test). `.env.example`, `docs/INSTALLATION.md` und der Kopfkommentar von `deploy/compose.yaml` empfehlen jetzt das Docker-Netz von Caddy als CIDR und nennen `compose.https-only.yaml` als Teil des HTTPS-Wegs. |
| **M-2** | Jede fremde Action in beiden Workflows ist auf einen vollen Commit-SHA gepinnt, der Tag steht als Kommentar daneben; `.github/dependabot.yml` hält die Pins nach. |
| **N-8** | Das Release-Image trägt Provenance (`provenance: mode=max`, `sbom: true`) und eine Attestation über `actions/attest-build-provenance`; prüfbar mit `gh attestation verify oci://ghcr.io/slouchcat/slouchcat:<version> --repo SlouchCat/slouchcat`. |
| **N-3** | `.tool-versions` steht auf `golang 1.27.3`. Das ausgelieferte Image baut ohnehin mit `golang:1.27-trixie`, die CI liest die Fassung aus `server/go.mod`. |
| **H-7** | Der temporäre Spike-Endpunkt `/spike/{variant}` ist aus dem Vertrag, dem Server und der Konfiguration entfernt, zusammen mit `SLOUCHCAT_SPIKE_DIR` und `testdata/generate-spike-media.sh`. |

Alles Übrige bleibt offen und steht unten mit Einordnung; die Reihenfolge am Dateiende gilt
weiterhin, ab Punkt 6.

---

## Vorbemerkung zum Bedrohungsmodell

Geprüft wurde gegen das Modell aus Plan 3.8 und der Aufgabenstellung, nicht gegen ein allgemeines:
Admin und Besitzer sind dieselbe Person, Gäste sind eingeladene Bekannte, Kinderprofile sind echte
Benutzer mit eigener Stufe. Ein Admin, der seinen eigenen Server angreift, ist kein Befund. Ein Gast
oder ein Kinderprofil, das mehr sieht oder mehr kann als es darf, ist einer. Neu hinzu kommt mit 1.0
ein Angreifer, den es vorher nicht gab: **jeder, der nach der Veröffentlichung einen Pull Request
aufmachen kann.** Darum ist Abschnitt 9 (Release-Weg) mit derselben Sorgfalt geprüft worden wie der
Server selbst.

Mehrfach habe ich einen naheliegenden Verdacht geprüft und **entkräftet**. Diese Stellen stehen
bewusst im Abschnitt «Geprüft und in Ordnung» mit Beleg, nicht als abgeschwächter Befund.

---

# Befunde

## Mittel

### M-1 — `SLOUCHCAT_TRUSTED_PROXIES=private` zusammen mit veröffentlichtem Port 8096 macht den Ratenbegrenzer des Logins wirkungslos

**Wo**
- `deploy/compose.yaml:3` (Kopfkommentar) und `deploy/compose.yaml:20-21` (`ports: "8096:8096"`)
- `server/internal/api/session.go:198-211` (`clientIP`)
- `server/internal/config/config.go:331` (`ParseTrustedProxies("private")`)
- `server/internal/auth/limits.go:55-86` (`attemptLimiter.allow`), `limits.go:91` (`AllowAttempt`)
- `docs/INSTALLATION.md:148`, `deploy/README.md:238`

**Was genau schiefgeht.** `ParseTrustedProxies("private")` erzeugt `10/8, 172.16/12, 192.168/16,
127/8, ::1/128, fc00::/7`. `clientIP` glaubt `X-Forwarded-For`, sobald `RemoteAddr` in dieser Liste
liegt. Im Heimnetz sind aber die Clients selbst RFC-1918-Adressen — das Preset vermischt «Proxy» und
«Client», jeder LAN-Client gilt als vertrauenswürdiger Proxy. Der Ratenbegrenzer des Logins ist
ausschliesslich auf diese Adresse geschlüsselt (`limits.go:57-78`: unbekannte Adresse → frischer
Bucket mit vollem `attemptBurst`), und `Login` hat **keine kontobezogene Sperre**: der einzige Riegel
vor `GetAccountByEmail` ist `AllowAttempt(ctx)` (`server/internal/auth/auth.go:245`).

**Angriffsweg.** Der Admin richtet HTTPS ein, lässt aber `compose.https-only.yaml` weg, sodass Port
8096 weiter auf `0.0.0.0` steht. Ein Gerät im WLAN — das Handy eines eingeladenen Gastes, eine
kompromittierte IoT-Kamera — schickt dann Login-Versuche direkt an `http://<host>:8096/api/v1/auth/login`
mit je einem frischen `X-Forwarded-For`-Wert; die Bremse von 10 Versuchen pro Minute feuert kein
einziges Mal. Übrig bleibt nur die globale argon2id-Drossel von vier parallelen Läufen
(`limits.go:15`), also rund 40–80 Rateversuche pro Sekunde gegen das Passwort des Hauptkontos.

**Ehrliche Gewichtung.** Der Standard ist richtig herum: `deploy/.env.example:75` lässt die Variable
leer, `ParseTrustedProxies("")` liefert `nil`, und `clientIP` sieht `X-Forwarded-For` dann gar nicht
erst an (`session.go:198-200`). `docs/INSTALLATION.md:148-166` macht es vollständig richtig und warnt
in genau diesen Worten: «solange 8096 offen bliebe, könnte ein Gerät im Heimnetz sich damit eine
fremde Adresse zuschreiben.» `deploy/README.md:241-250` warnt ebenfalls. Die Lücke ist der
Kopfkommentar von `deploy/compose.yaml:3`, der den HTTPS-Weg ohne den Override nennt. Und das
Mindestpasswort von 10 Zeichen (`server/internal/auth/auth.go:39`) heisst: ein gutes Passwort fällt
bei 40–80 Versuchen pro Sekunde nicht. Ein schwaches zehnstelliges Passwort — `Sommer2024!` — fällt
dagegen sehr wohl, und drei bis sieben Millionen Versuche pro Tag hinterlassen zudem keine brauchbare
Spur mehr. Darum mittel und nicht niedrig.

**Nicht betroffen, und das ist wichtig:** Die PIN-Sperre hängt an `profile_id` und `device_id`
(`server/internal/store/appq/access.sql.go:103`), nicht an der IP — eine gefälschte Adresse hilft
gegen ein Kinderprofil nichts. Ich habe alle Verbraucher von `model.ClientIP` verfolgt
(`system_handlers.go:27,48`, `auth_handlers.go:168,178`, `settings_handlers.go:62`,
`profile_handlers.go:64`, `channel_handlers.go:89`, `account_handlers.go:54`,
`invitation_handlers.go:40,71`): sie münden **alle** in `AllowAttempt`. Es gibt keine
«lokales Netz»-Vertrauensentscheidung, keine IP-Bindung der Sitzung, keinen IP-Check für Admin-Zugriff.

**Was dagegen zu tun wäre**
1. `deploy/compose.yaml:3` korrigieren — der HTTPS-Weg muss im eigenen Kommentar
   `-f compose.https-only.yaml` nennen, nicht nur im README. Minuten.
2. Beim Start warnen, wenn `TrustedProxies` die privaten Netze enthält **und** der Listener nicht auf
   Loopback steht, neben dem vorhandenen `slog.Any("trusted_proxies", …)` in `config.go:268`. Eine
   Fehlkonfiguration, die man nicht sieht, bleibt bestehen.
3. Sauberste Lösung: in `.env.example` und beiden READMEs statt `private` das konkrete Docker-Netz als
   CIDR empfehlen (`172.18.0.0/16`). `private` löst das Problem nie ganz, weil der Server-Container
   immer selbst in einem privaten Netz hängt.

---

### M-2 — Fremd-Actions im privilegierten Release-Job nur auf bewegliche Tags gepinnt

**Wo** `.github/workflows/release.yml:26, 39, 43, 45, 54` (`actions/checkout@v5`,
`docker/setup-qemu-action@v3`, `docker/setup-buildx-action@v3`, `docker/login-action@v3`,
`docker/build-push-action@v6`), Rechte in `release.yml:17-19`, ghcr.io-Login in `release.yml:45-49`.

**Was genau schiefgeht.** Der Release-Job läuft mit `contents: write` und `packages: write` und
meldet sich mit `secrets.GITHUB_TOKEN` bei ghcr.io an. Fünf fremde Actions laufen in diesem Kontext,
alle auf einem Tag — und ein Git-Tag ist verschiebbar.

**Angriffsweg.** Wer die Kontrolle über eines dieser Action-Repositories erlangt (gestohlener
Maintainer-Token, kompromittiertes Konto — in den letzten Jahren mehrfach real vorgekommen), setzt
`v3` oder `v6` auf einen bösartigen Commit. Beim nächsten `git tag v1.0.1 && git push --tags` läuft
dieser Code mit Schreibrechten auf Packages und Releases, kann das Image vor dem Push manipulieren
oder ein eigenes `:latest` nachschieben. Jeder Haushalt, der danach `docker compose pull` ausführt,
bekommt es.

**Was dagegen zu tun wäre.** Auf volle Commit-SHAs pinnen, den Tag als Kommentar daneben:

```yaml
# VORHER
- uses: docker/build-push-action@v6
# NACHHER
- uses: docker/build-push-action@4f58ea79222b3b9dc2c8bbdd6debcef730109a75 # v6.9.0
```

Dazu `.github/dependabot.yml` mit `package-ecosystem: github-actions` anlegen (die Datei gibt es
bisher nicht), damit die SHAs gepflegt bleiben. Für `ci.yml` ist es weniger dringend (nur
`contents: read`, keine Secrets), aber konsistent sinnvoll.

---

## Niedrig

### N-1 — `queries/app/` ist die dritte Abfrage-Kategorie, und für sie gibt es keinen Wächter

**Wo**
- `server/tools/itemsaccess/itemsaccess.go:28` — `DefaultAllowed = "visibility,admin,library,metadata,jobs,notify,offline,store"`
- `server/tools/itemsaccess/itemsaccess.go:80-92` — `isAllowed` lässt jeden Pfad durch, der `/internal/store/` enthält
- `server/.golangci.yml:29-36` — die depguard-Regel `systemq` verbietet nur `store/systemq`, nichts über `store/appq`
- `server/internal/store/scoped_test.go:35` — der Test liest ausschliesslich `queries/scoped/`

**Was genau schiefgeht.** Plan 3.3 und CLAUDE.md-Regel 2 kennen zwei Kategorien: `queries/scoped/`
(Test erzwingt `visible_items`) und `queries/system/` (depguard begrenzt die Importeure von
`systemq`). `queries/app/` → `store/appq` ist eine dritte, die in keiner der beiden Regeln vorkommt.
Keine depguard-Regel beschränkt, wer `appq` importieren darf; `scoped_test.go` schaut nicht hin; und
der Analyzer `itemsaccess` nimmt den gesamten `store`-Baum aus, weil die generierten Dateien sonst
sich selbst melden würden.

**Angriffsweg.** Heute **nicht ausnutzbar**: ich habe `queries/app/` vollständig gegen
`(FROM|JOIN|INTO|UPDATE)\s+(items|media_files|tracks)` geprüft — null Treffer, und `internal/api`
importiert `appq` gar nicht. Der Angriffsweg ist eine Regression: schreibt ein späterer Beitrag
`SELECT title, effective_rating FROM items WHERE id = ?` nach `queries/app/`, erzeugt sqlc daraus
`store/appq`-Code, der an **jedem** Lint vorbeikommt — Analyzer exempt, depguard stumm, Scoped-Test
blind — und aus jedem Paket, auch `internal/api`, aufrufbar ist. Damit wäre K-3 an einer Stelle
umgangen, an der drei Wächter stehen sollten.

**Was dagegen zu tun wäre.** Den Test in `server/internal/store/scoped_test.go` um eine zweite
Schleife über `queries/app/` erweitern, die dort jede Abfrage mit `items`, `media_files` oder `tracks`
ablehnt — spiegelbildlich zum bestehenden `visible_items`-Test. Alternativ `isAllowed` so verengen,
dass nur `store/systemq` und `store/scopedq` ausgenommen sind, `store/appq` aber geprüft wird. Die
erste Variante ist näher am vorhandenen Muster.

---

### N-2 — `syscall.Exec` umgeht die Lint-Durchsetzung von CLAUDE.md-Regel 3

**Wo** `server/cmd/slouchcat/main.go:207`:
`err = syscall.Exec(exe, os.Args, os.Environ()) //nolint:gosec // G204: the server's own binary …`

**Was genau schiefgeht.** `forbidigo` verbietet nur `^exec\.(Command|CommandContext|LookPath)$` aus
`pkg: ^os/exec$` (`server/.golangci.yml:50-54`), `depguard` nur den Import `os/exec`
(`.golangci.yml:21-28`). Weder `syscall.Exec`/`syscall.ForkExec` noch `os.StartProcess` noch ein von
Hand gebautes `exec.Cmd{Path: …}.Start()` fallen darunter.

**Angriffsweg.** Heute keiner: `exe`, `os.Args` und `os.Environ()` stammen vom Prozess selbst, kein
Benutzerwert fliesst ein, und der Aufruf ist der Neustart nach einer Wiederherstellung. Die Lücke ist
die Regel, nicht der Aufruf: ein späterer Beitrag kann auf diesem Weg still einen externen Prozess
ausserhalb von `transcode` starten, und genau das soll Regel 3 verhindern.

**Was dagegen zu tun wäre.** Die forbidigo-Liste ergänzen, mit einer Ausnahme für
`cmd/slouchcat/main.go` analog zur bestehenden für `internal/transcode/`:

```yaml
  - pattern: ^syscall\.(Exec|ForkExec)$
    pkg: ^syscall$
    msg: re-exec only in cmd/slouchcat (restart after a restore)
  - pattern: ^os\.StartProcess$
    pkg: ^os$
    msg: external processes run only through internal/transcode (CLAUDE.md rule 3)
```

---

### N-3 — Go-Toolchain auf 1.27.2 gepinnt, `govulncheck` meldet GO-2026-6809 als aufgerufen

**Wo** `.tool-versions:1` (`golang 1.27.2`), `server/go.mod:3` (`go 1.27`)

**Was genau schiefgeht.** `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` meldet
GO-2026-6809 («Distrust compromised intermediate certificates in `crypto/x509`»), gefunden in
`crypto/x509@go1.27.2`, behoben in `go1.27.3`, mit der Trace
`internal/playback/streams.go:145 → http.readWriteCloserBody.Read → x509.Certificate.Verify`.

**Angriffsweg.** Ein Angreifer mit Kontrolle über eine kompromittierte CA-Zwischenstelle und einer
Netzwerkposition zwischen Server und TMDB/TVDB/Trakt/Podcast-Host kann die ausgehende HTTPS-Verbindung
aufbrechen und gefälschte Metadaten oder einen untergeschobenen Enclosure-Datenstrom liefern. Das
liegt am Rand des Bedrohungsmodells — es braucht eine Position, die ein Gast im Heimnetz nicht hat.

**Warum trotzdem nur niedrig.** Das ausgelieferte Image baut mit `golang:1.27-trixie`
(`deploy/Dockerfile:10`), also dem jeweils aktuellen Patch-Stand; der Pin betrifft in erster Linie die
Entwickler- und CI-Toolchain.

**Was dagegen zu tun wäre.** `.tool-versions` auf `golang 1.27.3` heben und vor dem 1.0-Tag einmal
`go version -m` gegen das publizierte Image laufen lassen, den Beleg nach `docs/dev/CURRENT_STATUS.md`.

---

### N-4 — Keine Ratenbegrenzung vor der Token-Prüfung am Webhook

**Wo** `server/internal/integrations/receive.go:41-47` (`Authenticate` in Zeile 41, `s.limits.allow`
erst in 45); die globale Kette in `server/internal/api/api.go:109` führt kein Anfragebudget.

**Was genau schiefgeht.** Die Begrenzung von 10 Anfragen/s ist **pro Integration** und darum erst nach
einer erfolgreichen Token-Auflösung erreichbar. Ein unauthentifizierter Aufrufer wird nie begrenzt.

**Angriffsweg.** Jemand postet so schnell die Leitung hergibt an `/api/v1/webhooks/<43 Zeichen>`.
Jede Anfrage kostet einen Body-Read (auf 64 KiB gedeckelt), ein SHA-256, ein indiziertes SELECT auf
dem geteilten SQLite-Lesepool und eine INFO-Zeile im Anfrageprotokoll. Es gibt keine Sperre und kein
Budget pro Adresse, also ist die einzige Decke die Bandbreite; anhaltend konkurriert das mit dem
Streaming um Leseverbindungen und flutet das Protokoll.

**Ehrliche Gewichtung.** Die Vorarbeit ist wirklich klein, das Token hat 256 Bit (Raten ausgeschlossen),
und pro unbekanntem Token wird **nichts** alloziert — der Limiter-Eintrag entsteht erst nach
erfolgreicher `Authenticate` (`receive.go:113-138`) und verschwindet beim Löschen
(`integrations.go:221`). Härtung, keine offene Tür.

**Was dagegen zu tun wäre.** Ein Budget pro Adresse vor die öffentliche Route hängen —
`ratelimit.Limiter` auf `clientIP(r, trusted)` (`session.go:189` ist bereits da) — oder im
Installationsleitfaden festhalten, dass der vorgelagerte Reverse-Proxy das trägt.

---

### N-5 — Die vorgemerkte Pfad-Scan-Menge wächst pro gemeldetem Verzeichnis unbegrenzt

**Wo** `server/internal/library/service.go:490-507` (`triggerPathScan` schreibt jedes gemeldete
Verzeichnis nach `s.pathPending[libraryID][rootID]`, der Flush wird übersprungen solange
`s.pathRetrying[libraryID]` gesetzt ist), gespeist aus
`server/internal/integrations/pathmap.go:97-101`.

**Was genau schiefgeht.** `paths.Resolve` in `pathmap.go:97` ist eine rein lexikalische Prüfung. Sie
stellt sicher, dass der abgebildete Rest die Wurzel nicht verlassen kann, prüft aber nie, ob das
Verzeichnis existiert. Jeder in-der-Wurzel-aussehende Pfad wird darum angenommen und vorgemerkt.

**Angriffsweg.** Wer das Webhook-Token hat — ein kompromittiertes oder falsch konfiguriertes Radarr,
oder ein Token, das aus der Arr-Konfiguration entwichen ist — schickt `Download`-Ereignisse mit immer
neuen, nicht existierenden Unterverzeichnissen unterhalb einer Bibliothekswurzel, mit den erlaubten
10 Anfragen/s. Solange ein `scan.path`-Job in der Wiederholung hängt, ist der Flush blockiert und die
Menge wächst im Prozessspeicher; beim Flush zählt der Scan dann jedes dieser Verzeichnisse auf.

**Was dagegen zu tun wäre.** Die vorgemerkte Menge pro Bibliothek deckeln (über N Verzeichnisse hinaus
verwerfen und stattdessen einen vollen Scan einreihen) und/oder in `Resolve` das abgebildete
Verzeichnis über den `*os.Root` der Wurzel statten, bevor es eingereiht wird.

---

### N-6 — Kein globales `X-Content-Type-Options`, kein CSP, kein `frame-ancestors`

**Wo** `server/internal/api/stream_token.go:49-54` setzt als einzige globale Middleware
`Referrer-Policy: no-referrer`; `deploy/Caddyfile:15` setzt nur HSTS. `nosniff` steht nur an
einzelnen Binär-Endpunkten (`playback_handlers.go:66`, `download_binary.go:102`,
`playback_m4.go:166`), nicht an der App-Shell und nicht am Bild-Endpunkt (`item_handlers.go:311`).

**Was genau schiefgeht.** Es fehlt die zweite Schicht, falls je eine XSS-Lücke in der React-App
auftaucht.

**Angriffsweg — und warum er heute nicht trägt.** Die naheliegenden Wege sind geschlossen: der
Bild-Endpunkt kann keinen fremden Content-Type ausliefern, weil `storeOriginal` nur
`image/jpeg|png|webp` durchlässt und zusätzlich den Bild-Header dekodiert
(`server/internal/metadata/images.go:400-406`, Podcast-Cover ebenso `:921-924`) — ein Feed-Publisher
kann also kein HTML unter die eigene Origin legen. Podcast-Enclosures laufen bereits mit
`Content-Security-Policy: sandbox; default-src 'none'` plus `nosniff`
(`server/internal/playback/enclosure.go:214-215`). Clickjacking ist durch `SameSite=Lax` am
Session-Cookie weitgehend entschärft (`session.go:164`): im Cross-Site-Iframe wird das Cookie nicht
mitgeschickt, die eingebettete App lädt unauthentifiziert. Darum niedrig und ausdrücklich als
Defence-in-Depth eingestuft.

**Was dagegen zu tun wäre.** `securityHeaders` erweitern — eine Stelle, die ohnehin jede Antwort sieht:

```go
w.Header().Set("X-Content-Type-Options", "nosniff")
w.Header().Set("Content-Security-Policy",
    "default-src 'self'; img-src 'self' data: blob:; media-src 'self' blob:; "+
        "connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
```

Vorher gegen die gebaute Web-App prüfen (Vite-Inline-Styles, Service-Worker, hls.js-Blob-URLs). Ein
CSP, das die App bricht, wird wieder entfernt und nützt dann niemandem.

---

### N-7 — Der Jellyfin-GPG-Schlüssel wird zur Build-Zeit ungeprüft aus dem Netz geholt

**Wo** `deploy/Dockerfile:89-90`:
`curl -fsSL https://repo.jellyfin.org/jellyfin_team.gpg.key | gpg --dearmor -o /etc/apt/keyrings/jellyfin.gpg`

**Was genau schiefgeht.** Das ist kein `curl | sh` — der Schlüssel wird nicht ausgeführt. Er wird aber
zur **Vertrauenswurzel** für das anschliessende `apt-get install jellyfin-ffmpeg7`
(`Dockerfile:94-96`), ohne dass sein Fingerabdruck irgendwo geprüft würde.

**Angriffsweg.** Wer zur Build-Zeit `repo.jellyfin.org` oder die TLS-Kette unterwandert, liefert einen
eigenen Schlüssel **und** ein eigenes Paket; ffmpeg läuft danach im selben Container wie der
Media-Server. Die gepinnte Paketversion (`FFMPEG_VERSION=7.1.4-3`, `Dockerfile:75`) hilft nicht, weil
der Angreifer die Versionsnummer frei wählt.

**Was dagegen zu tun wäre.** Den Schlüssel als Datei ins Repository legen
(`deploy/jellyfin_team.gpg`) und per `COPY` einbinden, oder den Fingerabdruck nach dem Import hart
gegen einen erwarteten Wert prüfen und den Build sonst abbrechen.

---

### N-8 — Release-Image ohne Provenance und Signatur

**Wo** `.github/workflows/release.yml:53-65` — kein `id-token: write`, kein
`actions/attest-build-provenance`, keine cosign-Signatur.

**Was genau schiefgeht.** Wer `ghcr.io/slouchcat/slouchcat:1.0.0` zieht, kann nicht überprüfen, dass
das Image aus diesem Workflow und diesem Commit stammt.

**Angriffsweg.** Gegen den PR-Autor schützt eine Signatur nicht direkt; sie schützt gegen eine
Kompromittierung der Registry oder des Workflows und macht einen Fall wie M-2 nachträglich
erkennbar. Für ein öffentliches 1.0 ist das die übliche Erwartung.

**Was dagegen zu tun wäre.** `id-token: write` nur für den Release-Job, am Build-Schritt
`provenance: mode=max` und `sbom: true`, danach `actions/attest-build-provenance` mit
`push-to-registry: true`. Passt in denselben Commit wie M-2. Dass `deploy/compose.yaml:12` auf
`${SLOUCHCAT_VERSION:-1.0.0}` und nicht auf `latest` pinnt, ist richtig und soll so bleiben.

---

## Hinweise

### H-1 — Lexikalisches `Resolve` plus späteres Öffnen: Prüflücke auf den ffmpeg-Pfaden

`server/internal/paths/paths.go:32-47` (`Resolve` ist rein lexikalisch und löst nie Symlinks auf; in
`paths/doc.go:9-14` offen gesagt), `server/internal/library/scanner.go:607-613` (der Scanner nimmt
eine symbolisch verlinkte Mediendatei bewusst an, solange `r.Stat(p)` zur Scan-Zeit gelingt — also
darf `media_files.rel_path` legitim einen Symlink benennen), und die Stellen, an denen der lexikalische
Pfad danach an ffmpeg geht: `server/internal/playback/subtitles.go:176-183` und `:192-199` zusammen
mit `playback/sessions.go:469-483`, ebenso `library/probe.go:54-58`, `library/segments.go:177-181`,
`offline/job.go:175-179`, `offline/audio.go:250-254`.

Wer in eine Bibliothekswurzel schreiben kann — der Besitzer, oder ein Download-Client (Arr,
Torrent-Container), der dasselbe Medien-Volume teilt — legt einen Symlink, der während des Scans in
die Wurzel zeigt und danach nach aussen umgehängt wird. Die nächste Probe, Transkodierung oder
Untertitel-Konversion lässt ffmpeg das neue Ziel lesen, weil ffmpeg den Pfad selbst auflöst, ohne
Wurzelbindung. **Kein Befund im Bedrohungsmodell:** ein Gast oder Kinderprofil hat keinen
Dateisystemzugriff auf eine Bibliothekswurzel. Festgehalten, weil das Image öffentlich wird und Leute
geteilte Volumes einhängen werden. Abhilfe: symbolisch verlinkte Mediendateien in `videoFile`
ablehnen, oder über `paths.Open` öffnen und ffmpeg den Dateideskriptor (`/dev/fd/N`) statt des Pfades
geben. Mindestens aber den Rand in `paths/doc.go` benennen: «sicher gegen `..`, nicht gegen einen nach
der Prüfung untergeschobenen Symlink».

### H-2 — Download-Token ohne Ablauf und ohne Sitzungsbindung

`server/internal/offline/manifest.go:169` — `SignToken(key, downloadID, itemID, profileID)` trägt
weder Zeitstempel noch Sitzungs-ID. `AuthorizeDownload` (`manifest.go:197-222`) prüft bei **jeder**
Anfrage neu, dass das Profil zum Konto des Downloads gehört und den Titel sehen darf — aber nicht, ob
überhaupt noch eine Sitzung besteht. Der Stream-Token macht das anders: er löst über
`playback_sessions` auf, und die Abfrage verbindet auf `sessions` und achtet den Entzug
(`server/queries/app/playback.sql`, `GetPlaybackSession`). Für K-7 ist das gedeckt — beim Entzug eines
Gastkontos verschwinden die Download-Zeilen über `ON DELETE CASCADE`
(`server/migrations/00013_offline.sql:32`), und `GetDownload` antwortet dann 401. Eine blosse
Abmeldung hebt den Download-Token dagegen nicht auf: eine `dt=`-URL, die in einem Browser-Verlauf oder
auf einem Bildschirmfoto landet, funktioniert weiter, bis der Download gelöscht wird. Im
Bedrohungsmodell vertretbar; wenn es stören soll, gehört die Sitzungs-ID in den MAC.

### H-3 — Die Redaction trifft nur Schlüssel-**Suffixe**

`server/internal/logging/logging.go:181-189` — `isSensitiveKey` nutzt `strings.HasSuffix(k, s)`. Ein
künftiger Schlüssel wie `secret_key`, `token_value` oder `vapid_private_key_pem` passt auf keinen
Eintrag und ginge im Klartext durch. Alle heute im Baum verwendeten slog-Schlüssel wurden aufgezählt:
es gibt **keinen** solchen Fall. `strings.Contains` statt `HasSuffix`, oder ein Test, der die
tatsächlich verwendeten Schlüsselnamen gegen die Liste prüft.

### H-4 — `RedactString` maskiert keine URL-Userinfo

`server/internal/logging/logging.go:125-142` deckt Query-Parameter und die Token-Pfade
(`/invitations/`, `/invite/`, `/webhooks/`, `/bot<id>:<secret>/`) ab, aber nicht
`https://user:pass@host/…`. Heute folgenlos, weil jede Stelle, an der eine URL vom Benutzer kommt,
`u.User != nil` zurückweist: `internal/metadata/podcast.go:204`, `internal/metadata/images.go:886`,
`internal/notify/push/push.go:75`, `internal/config/config.go:316`, `internal/admin/settings.go:434`.
Die Redaction hat unter diesen Validatoren allerdings kein Netz — ein Satz in der Paketdoku wäre
ehrlicher als Stillschweigen.

### H-5 — Der Modus der Schlüsseldatei wird beim Lesen nur gewarnt, nicht korrigiert

`server/internal/secrets/secrets.go:58-61` — bei `perm & 0o077 != 0` wird geloggt und weitergemacht.
Beim Anlegen ist 0600 hart erzwungen (`secrets.go:97`). Im Bedrohungsmodell vertretbar; wer die
Warnung überliest, lässt den Masterschlüssel aber dauerhaft offen. Alternativ `os.Chmod(path, 0o600)`.

### H-6 — Die Sicherung ohne Schlüssel ist unverschlüsselt und enthält Hashes und Feed-Zugangsdaten

`server/internal/admin/backup.go:327-341` entfernt Ciphertext-Einstellungen, Push-Abos,
Telegram-Verknüpfungen, Sitzungen und PIN-Versuche mit `PRAGMA secure_delete = ON`. Nicht entfernt
werden die argon2id-Passwort- und PIN-Hashes (`internal/auth/password.go:41`), die
SHA-256-Einladungstoken (`migrations/00007_accounts_profiles_access.sql:13`) und die Zugangsdaten
privater Podcast-Feeds in Feed- und Enclosure-URLs. Letzteres ist bewusst so und steht im Manifest
(`backup.go:86-93`, `IncludesFeedCredentials` immer `true`); die Hashes stehen dort nicht. Alles ist
korrekt gehasht, kein Klartext, die Route ist admin-only — **kein Befund gegen das Bedrohungsmodell.**
Ein Satz im Admin-Dialog wäre trotzdem ehrlich, weil der Admin die Datei herunterlädt und irgendwo
ablegt.

### H-7 — Der Spike-Endpunkt mit `Access-Control-Allow-Origin: *` ist noch im 1.0-Image

`server/internal/api/spike_handler.go:40`, Route in `api.go:131`, Sichtbarkeit `public` in `api.go:51`.
Entschärft: ohne `SLOUCHCAT_SPIKE_DIR` antwortet der Handler 404 (`spike_handler.go:22`), der
Dateiname kommt aus einer festen Tabelle (`spike_handler.go:12-15`, keine Traversal-Fläche), und
`ACAO: *` ohne `Access-Control-Allow-Credentials` gibt keinen Zugriff auf die Sitzung. Der Kommentar
nennt es selbst «Temporär bis zum M1b-Entscheid» — M1b ist durch. Nach der Arbeitsweise in CLAUDE.md
(«Platzhalter gelten als Blocker») gehört das vor dem Veröffentlichen entfernt oder als bewusster
Entscheid in `docs/planning/04-entscheidungen.md` festgehalten.

### H-8 — Base-Images auf beweglichen Tags statt auf Digests

`deploy/Dockerfile:9-11` — `node:24-trixie-slim`, `golang:1.27-trixie`, `debian:trixie-slim`. Für ein
öffentlich verteiltes 1.0 heisst das: zwei Builds desselben Commits ergeben unterschiedliche Images.
Das Risiko ist gering (offizielle Docker-Library-Images), die Reproduzierbarkeit ist das Argument.
`ARG BASE_IMAGE=debian:trixie-slim@sha256:…`, Dependabot pflegt es mit.

### H-9 — `NVIDIA_VISIBLE_DEVICES=all` steckt im Image statt im GPU-Override

`deploy/Dockerfile:124-125`. Auf einem Host, auf dem die NVIDIA Container Toolkit als Default-Runtime
konfiguriert ist, bekommt der Container alle GPUs eingehängt, auch wenn der Nutzer nur `compose.yaml`
ohne jeden GPU-Override startet. Das widerspricht dem, was die Compose-Datei deklariert.
`deploy/compose.nvidia.yaml` setzt die Capabilities ohnehin explizit — die ENV im Image ist entbehrlich.

### H-10 — Workflow-Kleinigkeiten ohne Angriffsweg in diesem Modell

`.github/workflows/release.yml:75, 78, 92` interpolieren `${{ steps.version.outputs.version }}` aus
`GITHUB_REF_NAME` unquotiert in `run:`-Blöcke; ein Tag wie `v1.0.0$(id)` passt auf das Trigger-Glob
`v*.*.*`. **Kein Befund**, weil ein Tag Push-Rechte braucht und ein Fork-PR keinen Tag im Upstream
erzeugt — aber die billige Gewohnheit ist, den Wert über `env:` zu reichen und als `"$VERSION"` zu
benutzen; die Release-Notes machen es in `release.yml:72-76` bereits richtig. Dazu: der
`permissions`-Block steht auf Top-Level (`release.yml:17-19`) statt am Job; heute gibt es genau einen
Job, ein zweiter erbte `contents:write` und `packages:write` still.

### H-11 — Offene Punkte der Werkzeugkette

- **`pnpm audit`:** ein hohes Advisory, `braces` ≤ 3.0.3 (GHSA-vfj7-8cjw-p6xm), ausschliesslich unter
  `web > stylelint > fast-glob|globby > micromatch > braces`, also einer devDependency; nicht im
  ausgelieferten Bundle und nicht im Container. «Patched versions: None» — nicht behebbar. Als bekannte,
  nicht erreichbare Meldung dokumentieren oder per `pnpm.auditConfig.ignoreGhsas` stummschalten, damit
  das 1.0-CI nicht daran hängen bleibt. **Kein Release-Blocker.**
- **`govulncheck` ist nicht gepinnt:** `server/Taskfile.yml:78` ruft
  `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` — ein unversionierter Netzwerk-Abruf bei
  jedem Lauf. Für reproduzierbares CI gehört es in den `tool`-Block von `server/go.mod`, konsistent
  mit der Toolchain-Regel in CLAUDE.md.
- **HSTS nur über den mitgelieferten Caddy:** `deploy/Caddyfile:15` setzt
  `Strict-Transport-Security`. Wer dem dokumentierten Weg «Eigener Reverse-Proxy»
  (`deploy/README.md:329-345`) folgt, bekommt keines; die Anforderungstabelle dort nennt
  `X-Forwarded-Proto`, `X-Forwarded-For`, Timeouts und Body-Grösse, aber HSTS nicht.
- **`ffmpeg.log` läuft nicht durch die Redaction:** `server/internal/transcode/session.go:301-305`
  schreibt ffmpeg-stderr roh in `<session>/ffmpeg.log` (Modus 0600, mit der Session gelöscht). Inhalt
  sind absolute Medienpfade, keine Geheimnisse; der stderr-Tail erreicht weder einen Client
  (`api/problem.go:73-76` sendet bei 500 ein leeres `detail`) noch ein Log ohne Redaction. Nur der
  Vollständigkeit halber vermerkt.

---

# Geprüft und in Ordnung

Das ist der Abnahmebeleg: je Punkt die Stelle, an der ich es festmache.

### 1. Sichtbarkeit und Altersfreigabe (K-3) — **hält**

Die Kette ist vollständig und schliesst an jedem Ende. **Vertrag:** alle **152** Operationen in
`api/openapi.yaml` tragen `x-visibility`, keine fehlt; von den Operationen, deren Antwort
transitiv ein `ItemRef` enthält, sind **alle 13** `scoped` — keine einzige Ausnahme (maschinell über
den aufgelösten Schema-Graphen geprüft). **Vertragstest:**
`server/internal/api/contract_access_test.go:314` `TestContractCoversEveryOperation` erzwingt für
jede Operation einen Eintrag in der Registry ihrer Sichtbarkeitsstufe und meldet jede Stufe, die
keine Registry kennt; `TestContractAccountOperations` prüft zusätzlich, dass eine `account`-Operation
nie eine Titel-ID und nie einen Hash trägt (`secretsIn`). **Middleware:**
`server/internal/api/session.go:105-125` fällt für jede Route ohne brauchbaren Marker geschlossen auf
401 (`validVisibility`, `!known || vis == "system"`), `scoped` ohne aktives Profil ist 403
`profile_required`. **Abfragen:** jede Datei in `queries/scoped/` enthält `visible_items` (selbst
nachgeprüft, null Ausnahmen), erzwungen von `internal/store/scoped_test.go:35`; `queries/app/` enthält
**keine** Abfrage auf `items`, `media_files` oder `tracks` (selbst nachgeprüft). **Importe:** die
Importeure von `store/systemq` sind exakt `visibility, admin, library, metadata, jobs, notify, offline`
— genau die depguard-Liste aus `.golangci.yml:29-36`, kein Paket darüber hinaus; `store/scopedq`
importiert ausschliesslich `visibility`. **Query-Builder:** der eine dynamische Pfad
(`internal/visibility/search.go`) baut ausschliesslich aus konstanten Fragmenten, jeder Benutzerwert
ist ein gebundener Parameter, `in(n)` zählt nur Platzhalter, und der Join auf `visible_items` steht in
jedem Zweig fest im Rumpf (`search.go:176`, `:184-185`); `MatchExpression` (`:56-67`) verwirft alles,
was kein Buchstabe oder Ziffer ist, und verdoppelt `"` — keine FTS5-Operator-Injektion. Auch die
Nebentreffer sind gefiltert: Personen (`searchPeople`, `:224-228`) und Sammlungen (`:209-212`)
verbinden beide auf `visible_items`. **Die unauffälligen Wege, die ich eigens gesucht habe, sind
ebenfalls dicht:** Benachrichtigungen prüfen die Sichtbarkeit beim Erzeugen **und** beim Lesen
(`internal/notify/notify.go:188-189`, `:234`), und sogar der Zähler für Gelesen/Ungelesen verbindet auf
`visible_items` (`queries/system/notify.sql`, `NotifyCount`) — die Ungelesen-Zahl verrät also nicht
einmal die Existenz eines verborgenen Titels; die Fortschrittsmeldung geht durch
`s.vis.Summary(...)`, bevor irgendetwas geschrieben wird (`internal/playback/playback.go:295`); der
Vorlade-Header und das vorgemalte Hero-Bild der App-Shell stammen aus
`FirstPick(ctx, session.Profile)` und damit aus der Sichtbarkeitsschicht
(`internal/api/preload.go:96-100`), html-escaped. **Nicht sichtbar heisst 404, nicht 403** — so
umgesetzt in `OpenGrant` (`playback/sessions.go:774-779`), `HLS` (`:622-626`) und
`AuthorizeDownload` (`offline/manifest.go:218-220`). Einzige Einschränkung: die Wächter-Kette hat
die Lücke aus **N-1** (`queries/app/`), heute ohne Angriffsweg.

### 2. Autorisierung der Medienströme — **hält**

Ein Stream-Token ist `<ULID>.<HMAC-SHA256(key, id + "\n" + itemID)>` (`playback/token.go:10-33`), der
Schlüssel wird aus der Schlüsseldatei abgeleitet (`secrets.Keyring.Derive("stream-token")`) und lebt
nur im Speicher. **Ein Token für ein Item kann kein anderes öffnen:** `Authorize`
(`sessions.go:559-574`) liest die Zeile über den Primärschlüssel und rechnet den MAC gegen
`row.ItemID` aus der Datenbank nach — die Item-ID ist für den Angreifer nicht formbar; zusätzlich
vergleicht `getItemStream` die Pfad-ID gegen `g.ItemID` (`api/item_handlers.go:326-329`). **Entzug
wirkt sofort:** die Abfrage `GetPlaybackSession` verbindet auf `sessions` und verlangt
`ps.revoked_at IS NULL AND ps.expires_at > @now AND s.revoked_at IS NULL AND s.expires_at > @now` —
Sitzung und Wiedergabesitzung müssen beide leben, und sie wird bei **jeder** Anfrage neu gelesen; ein
laufender Range-Stream wird über `abortOnRevoke` (`item_handlers.go:357`) binnen einer Sekunde
abgeschnitten. **Sichtbarkeit wird nicht nur beim Start geprüft, sondern bei jedem Abruf:**
`HLS` (`sessions.go:622`), `Open` über `s.vis.MediaFile(...)` (`playback/playback.go:172`),
`streams.go:113` im laufenden Lesen, `offline/job.go:119`, `offline/manifest.go:219`. Der
Download-Token (`offline/manifest.go:169`) bindet zusätzlich die Profil-ID in den MAC und prüft bei
jeder Anfrage, dass das Profil zum Konto des Downloads gehört **und** den Titel sieht
(`:212-220`); ein unbekannter Download ist 401 und nicht 404, damit sich IDs nicht aufzählen lassen.
**Kein Token im falschen Log:** `Referrer-Policy: no-referrer` auf jeder Antwort
(`api/stream_token.go:49-54`), und `st`, `dt`, `token`, `api_key` und ihre Verwandten werden in jedem
geloggten String maskiert (`logging/logging.go:56`, `:125-142`). Offener Rand: **H-2**.

### 3. Sitzungen und Konten — **hält**

Passwörter und PINs mit argon2id, 64 MiB / t=3 / p=4, Salz 16 Byte, Vergleich über
`subtle.ConstantTimeCompare` (`auth/password.go:24`, `:71`); aus einer gespeicherten Zeile gelesene
Parameter sind nach oben gedeckelt, damit eine beschädigte Zeile keinen Gigabyte-Login auslöst
(`:27-30`). Ein Login gegen ein nicht existierendes Konto verifiziert gegen einen einmal berechneten
Dummy-Hash, verbraucht also dieselbe Zeit (`auth/auth.go:250-254`, `dummy()` `:307`). Sitzungstoken
sind 256-Bit-Zufallswerte, gespeichert als SHA-256 (`StartSession` `:321-328`, `hashToken` `:496`).
Das Cookie ist `HttpOnly`, `SameSite=Lax`, `Path=/`, `Secure` sobald die Anfrage über TLS kam
(`api/session.go:228-240`; Klartext-HTTP im LAN ist ein ausdrücklich unterstützter Fall), mit
gleitender Erneuerung. **CSRF** ist sauber gelöst: der eigene Header `X-Requested-With: Slouchcat` für
jede zustandsändernde Methode (`session.go:67-74`) zusätzlich zu `SameSite=Lax`, geprüft **nach** dem
Routing, sodass unbekannte Routen weiter 404/405 antworten; die einzige Ausnahme ist der
Webhook-Empfänger, und sie ist auf das chi-**Routenmuster** geschlüsselt, nicht auf den rohen Pfad
(`session.go:88-90`), also nicht vortäuschbar. **PIN-Sperre (AN-11)** ist die sorgfältigste Stelle des
Pakets: gezählt wird pro Profil **und** Gerät in `pin_attempts`, also über Neustarts hinweg, ab dem
dritten Fehlversuch 30 s mit Verdopplung bis 15 min, Rücksetzung nach 24 h ohne Fehlversuch, und
während einer Sperre wird die PIN gar nicht erst geprüft; das Gerät ist das der **Sitzung**, nicht das
Cookie der aktuellen Anfrage, und darüber liegt eine Decke pro Profil über alle Geräte (20
Fehlversuche je Stunde → 15 min für alle), gebucht auf ein reserviertes Pseudo-Gerät, das ein Client
nicht beanspruchen kann (`auth/pin.go:27-46`, `:128-205`); gleichzeitige Versuche sind über
`s.pinMu` serialisiert. Einladungstoken sind 256 Bit, als SHA-256 gespeichert
(`auth/invitations.go:72`) und im Pfad redigiert. **Der Entzug eines Gastes (K-7)** hat eine bewusst
gewählte Reihenfolge, die ich nachvollzogen habe (`auth/guests.go:142-165`): zuerst alle Sitzungen
des Kontos in einer eigenen festgeschriebenen Anweisung, dann `end` (offene Streams und HLS-Ausgaben),
dann die Löschung in einer Transaktion, dann `end` ein zweites Mal für einen Stream, den eine Anfrage
zwischendurch noch an den Prüfungen vorbei geöffnet hat. Die Downloads verschwinden über
`ON DELETE CASCADE` mit (`migrations/00013_offline.sql:32`).

### 4. Der Webhook-Empfänger aus M9 — **hält**

**Token-Prüfung:** 32 Byte aus `crypto/rand`, base64url, 43 Zeichen; gespeichert wird nur der
SHA-256-Hex und die letzten vier Zeichen (`integrations.go:260-273`,
`migrations/00017_integrations.sql:25-26` mit `CHECK (length(token_hash) = 64)` und `UNIQUE`); der
Klartext steht genau einmal in der Antwort auf das Anlegen. Die Prüfung ist eine Längenprüfung plus
ein parametrisiertes Nachschlagen auf dem Hash (`integrations.go:313-325`) — **nicht**
`subtle.ConstantTimeCompare`, und das ist hier richtig so: verglichen wird ein SHA-256 des
Geheimnisses, ein Angreifer kann die verglichenen Bytes nicht steuern, und ein Zeitunterschied
verrät nichts über das Urbild. **Ratenbegrenzung:** ein Limiter pro Integrations-ID, nicht-blockierend
(`ratelimit.go:114-130`), die Karte entsteht erst **nach** erfolgreicher Authentifizierung und wird
beim Löschen abgeräumt (`receive.go:113-138`, `integrations.go:221`) — ein unbekanntes Token alloziert
nichts, der naheliegende Speicher-Erschöpfungsangriff läuft ins Leere. Offener Rand vor der
Authentifizierung: **N-4**. **Pfad-Umsetzung — der gemeldete Pfad kann die Wurzeln nicht verlassen:**
`pathmap.go:25-41` `Map` führt zuerst `path.Clean(reported)` aus, sodass jedes `..` verschwindet,
bevor überhaupt verglichen wird; `under` (`:47-61`) verlangt beide Seiten absolut und vergleicht ganze
Komponenten, sodass `/movies` nicht auf `/moviesclub` passt und ein relativer Bericht auf nichts; der
Rest läuft durch `paths.Resolve` (`:97`). Danach erreicht der Nutzlast-Pfad das Dateisystem **gar
nicht als Pfad**: er wird zu `Target.Dir`, einem Pfad relativ zur Wurzel, den `library.RequestPathScan`
über `paths.Open`/`*os.Root` öffnet (`library/pathscan.go:228-234`). Der Empfänger schreibt nichts und
öffnet nichts. **Kein Token im Log:** `webhookPath = (?i)(/webhooks/)[A-Za-z0-9_-]{16,}`
(`logging.go:114-117`), angewandt im Anfrageprotokoll (`api/middleware.go:59`), im `instance` des
Problem-Dokuments (`api/problem.go:52`) und in den Fehler-/Panik-Logs (`:75`, `:90`); der Test
`logging_test.go:98-124` belegt `/webhooks/[redacted]` bei weiterhin lesbarem Dateipfad. **Kein
SSRF:** `Receive` macht keine einzige ausgehende Anfrage. Begrenzung: der Ereignis-Log ist über einen
Trigger auf 20 Zeilen je Integration gedeckelt (`00017_integrations.sql:46-57`). Offener Rand:
**N-5**.

### 5. Dateizugriff — **hält**

`paths.Resolve` (`paths/paths.go:36-46`) verwirft in dieser Reihenfolge und **vor** jedem Reinigen:
leer, jedes NUL-Byte, absolut, führender `/`, jedes `..`-Segment (getrennt an `/` **und** am
OS-Trenner), und prüft am Ende `filepath.IsLocal`. `"a/../b"` wird also abgelehnt statt still
bereinigt — die strengere Wahl. `paths.Open` liefert einen `*os.Root`, dessen `Open`/`Stat`/`Create`
`..`, absolute Pfade und aus der Wurzel führende Symlinks von sich aus ablehnen (`root.go:13-22`);
`Walk` läuft über `r.FS()`, steigt also nie in ein symbolisch verlinktes Verzeichnis ab (`:27-33`).
`NewRoot` verlangt ein absolutes Verzeichnis (`paths.go:21-26`) — nebenbei der Grund, warum kein
aufgelöster Pfad je mit `-` beginnen kann. Ich habe **alle 117** Dateisystem-Aufrufstellen in
`server/internal/` und `server/cmd/` durchgesehen (`os.Open/ReadFile/Stat/Create/MkdirAll/Remove/
OpenFile/ReadDir/Rename/WriteFile`, `http.ServeFile/ServeContent/Dir`, `os.DirFS`, `filepath.Walk`).
Keine einzige macht aus Benutzerdaten einen Pfad am `paths`-Dienst vorbei. Die heiklen namentlich:
Bilder — `kind` und `size` gegen feste Erlaubnislisten (`api/item_handlers.go:280-288`), Datei über
`paths.NewRoot(cacheDir)` → `paths.Open` → `dir.Open(loc.RelPath)` aus einer gescopten Abfrage
(`discover/discover.go:208-235`); HLS-Segmente — `strconv.Atoi` mit `n < 0` abgelehnt, Sitzungs-ID
muss der des Grants gleichen (`api/playback_handlers.go:45-48`, `:97-108`); Downloads — Track per
`ParseInt` nach `.vtt`-Prüfung, Download-ID gegen den Grant, Lesen ausschliesslich über
`*os.Root`-Methoden (`api/download_binary.go:83-94`, `offline/manifest.go:114-139`); Untertitel —
die Wiedergabe-ID muss auf `^[0-9A-HJKMNP-TV-Z]{26}$` passen und der Track exakt der von der Sitzung
gewählte sein (`playback/subtitles.go:47`, `:54-57`); Podcast-Enclosures — URL bei jeder Anfrage neu
geparst, Schema und Host neu geprüft, Sichtbarkeit zuerst, Umleitungen gedeckelt und schema-begrenzt,
Content-Type auf `audio/*`, `video/mp4` oder `application/octet-stream` verengt, dazu `nosniff` und
`Content-Security-Policy: sandbox; default-src 'none'` (`playback/enclosure.go:49-54`, `:167-215`);
YouTube-IDs über `youtubeid.Valid` (`metadata/manage.go:193-199`); die Web-App über `fs.FS` auf einer
`embed.FS` (`webassets/webassets.go:44-67`); Sicherungen über einen Namensabgleich gegen die gelistete
Verzeichnis (`admin/backup.go:503-514`). Auch der Wiederherstellungspfad ist gegen Zip-Slip dicht:
`unpackArchive` (`backup.go:626-651`) nimmt den Namen aus dem Archiv **nie** entgegen, sondern gleicht
ihn gegen drei Konstanten ab und schreibt mit `filepath.Join(dir, <Konstante>)`;
`h.Typeflag != tar.TypeReg` wirft Symlinks, Hardlinks und Verzeichnisse hinaus, `O_EXCL` beim
Schreiben, Grössen gedeckelt, der scrypt-Arbeitsfaktor eines hochgeladenen Archivs auf 2^20 begrenzt.
Offener Rand: **H-1**.

### 6. Externe Prozesse — **hält**

`grep -rn '"os/exec"' server/` liefert ausserhalb von `internal/transcode` **null** Treffer; depguard
(`.golangci.yml:21-28`) und forbidigo (`:50-54`, Ausnahme nur `:76-79`) setzen es durch. Ich habe
**jede** ffmpeg-/ffprobe-Argumentliste im Paket einzeln durchgesehen (`transcode.go:29`,
`backend.go:234`, `probe.go:344`, `images.go:196`, `audio.go:42`, `subtitles.go:163,221`,
`pipeline.go:51-197`). **Argv-Injektion greift nicht:** jeder Pfad in einer Argumentliste stammt aus
`paths.Resolve` und ist damit `filepath.Join(<absolute Wurzel>, rel)` — er beginnt immer mit `/`, nie
mit `-`. **Protokoll-Prefixe greifen aus demselben Grund nicht:** ffmpeg erkennt ein Protokoll nur an
`schema:` **vor** dem ersten `/`; ein Pfad, der mit `/` beginnt, kann nie `http:`, `concat:`,
`subfile:`, `crypto:` oder `data:` sein, und `-i` verbraucht sein Folgeargument unabhängig vom Inhalt.
**Die concat-Liste** (`-f concat -safe 0`, `pipeline.go:96`) wird in `audio.go:169-186` geschrieben und
lehnt jeden Teil ab, der nicht absolut ist oder `\n`/`\r` enthält (`:176-178`) — kein zweiter
`file`-Eintrag, keine Direktive einschleusbar — und quotet Apostrophe nach ffmpegs eigener Regel
`'` → `'\''` (`:180`). **Filter-Syntax-Injektion** über einen Dateinamen ist die einzige Stelle, die
wirklich knifflig ist (`subtitles.go:85-91`, `subtitles=filename=…`), und sie ist richtig gelöst:
`filterValue` (`:96-98`) escaped doppelt und in der richtigen Reihenfolge — erst die Option-Ebene
(`\ ' : =`), dann die Graph-Ebene (`\ ' [ ] , ;`) einschliesslich der in Schritt 1 eingefügten
Backslashes; ein Dateiname mit `,` kann die Verkettung in `pipeline.go:121` nicht aufbrechen, einer
mit `:` keine zweite Filteroption anhängen. Der Bitmap-Pfad (`overlayGraph`, `:258-270`) setzt nur
Ganzzahlen in den Graphen. **Kein `-f` und kein Codec aus Benutzereingabe:** Muxer und Encoder kommen
aus Konstanten oder festen Karten (`images.go:39-48` mit `default:` als Fehler, `backend.go:51-53`,
`pipeline.go:482-484`); `Target.Quality` geht über `rungByName` (`plan.go:156-179`), ein unbekannter
Name ist ein Fehler; alle Raten, Höhen, Indizes und Zeiten sind Ganzzahlen über `strconv.Itoa` bzw.
`seconds6`. **Prozesse sind begrenzt und hinterlassen keine Waisen:** Budget je Backend und Admission
Control mit höchstens 10 s Warten in Ankunftsreihenfolge (`manager.go:106-115`, `:176-261`), jeder
ffmpeg hängt an `m.ctx`, SIGTERM beim Cancel mit `WaitDelay` für SIGKILL (`session.go:566-576`),
Leerlauf-Stopp nach 60 s. **Ein Gast kann das nicht missbrauchen:** zusätzlich zum globalen Budget
begrenzt `playback/limits.go:142-160` die gleichzeitigen Streams pro Profil **und** pro Konto, für
Gäste gesetzt in `auth/guests.go:72,105,130`; Untertitel-Konversionen laufen über ein Semaphor
(`playback/playback.go:72`). Offener Rand: **N-2** (die Lint-Regel, nicht der Code).

### 7. Geheimnisse — **hält, und im Repository liegt keines**

**Schlüsseldatei:** atomar angelegt (`CreateTemp` im Zielverzeichnis, explizit `Chmod(0o600)`,
`Write`, `Sync`, `Close`, `Rename`, Aufräumen bei jedem Fehler — `secrets.go:83-113`), Länge bei jedem
Start geprüft (`:63-65`): fehlt sie, wird sie erzeugt; ist sie zu kurz oder abgeschnitten, startet der
Server mit klarer Meldung **nicht** — kein stilles Weiterarbeiten mit einem Teilschlüssel. Der
Schlüssel erscheint in keiner Log- und keiner Fehlermeldung. **Krypto:** AES-256-GCM, Nonce je
Verschlüsselung frisch aus `crypto/rand` und korrekt als Prefix über `Seal(nonce, nonce, …)`
(`:117-121`) — eine Wiederverwendung ist strukturell ausgeschlossen; Version als Associated Data in
beide Richtungen; `Decrypt` prüft die Mindestlänge vor dem Slicing und gibt nur zwei generische
Sentinels zurück (`:32-35`, `:132`); Zweckschlüssel über HKDF-SHA256 mit Label, nur im Speicher.
**Redaction:** die Rekursion in `slog.Group` ist da (`logging.go:152-158`), `WithGroup`/`WithAttrs`
sind gewrappt (`:97-108`), `LogValuer` wird über `a.Value.Resolve()` aufgelöst (`:148`) — und genau
das trägt den wichtigsten Fall: `Config.LogValue` (`config/config.go:253-280`) reduziert TVDB-Key,
YouTube-Key, SMTP-Passwort und Telegram-Token auf `*_configured`-Booleans, bevor
`cmd/slouchcat/main.go:167` die Konfiguration loggt. Der Vergleich ist case-insensitiv und ignoriert
`-`/`_` (`:182`). Ich habe **alle** slog-Schlüsselnamen im Server-Baum aufgezählt und gegen die Liste
gehalten: kein Schlüssel, der einen Secret-Wert trägt, fehlt. Es gibt keine Access-Log-Middleware, die
eine vollständige URL loggt; `api/problem.go:75,90` loggen nur `RedactString(r.URL.Path)`. Der
ErrorRing sitzt **hinter** der Redaction (`errors.go:41-55`), sieht also nur bereinigte Datensätze.
**Sicherung:** ohne Schlüssel kommt kein Schlüsselmaterial ins Archiv (`backup.go:242-250`,
`:420-424`) und die entschlüsselbaren Werte werden mit `PRAGMA secure_delete = ON` entfernt, sodass
kein Ciphertext in einer freien Seite überlebt (`:327-341`); mit Schlüssel ist die Passphrase
zwingend und mindestens 12 Runen (`:214-221`), verschlüsselt mit age/scrypt (`:397-407`). Ein Archiv
mit Schlüssel **ohne** Passphrase ist strukturell unmöglich: `passphrase` wird nur im
`IncludeSecrets`-Zweig gesetzt, und genau dieser Zweig steht hinter der Prüfung. **`deploy/.env.example`
enthält nur Platzhalter:** jede Secret-Variable steht leer da, mit Anleitung und dem Hinweis «Never
commit this file». **Secret-Scan über Arbeitsbaum und Historie:** `git log --all -p -S'-----BEGIN'`
ist **leer** — kein privater Schlüssel wurde je committet; `git log --all --diff-filter=A --name-only`
gefiltert auf `.env*`, `secret.key`, `*.pem`, `*.key`, `id_rsa`, `credentials`, `*.p12`, `*.pfx`
liefert als einzigen Treffer `deploy/.env.example`. Ein echtes `deploy/.env` (Modus 0600) existiert
lokal, ist aber nicht getrackt (`git ls-files --error-unmatch` schlägt fehl, `.gitignore` deckt es).
Der einzige Entropie-Treffer in getrackten Dateien ist der gzip-base64-kodierte OpenAPI-Block in
`internal/api/gen/api.gen.go` — generierter Code. **Für ein öffentliches Repository ist der Baum
sauber; es ist nichts zu rotieren.**

### 8. Das Image und der Betrieb — **hält, mit der Falle M-1**

`USER 1000:1000` (`Dockerfile:136`), der Benutzer ohne Home und mit `/usr/sbin/nologin`
(`:113-115`), `/config` und `/cache` gehören 1000:1000 mit Modus `0750` (`:116`) — nicht
world-readable. Mehrstufig und sauber: Stage 3 übernimmt ausschliesslich `/out/slouchcat` (`:118`),
kein Quellcode, keine Go-Toolchain, keine `node_modules`, keine Build-Secrets im finalen Layer; die
Build-Werkzeuge werden mit `apt-get purge --auto-remove curl gnupg` wieder entfernt (`:97-99`); keine
`ADD` von einer URL, kein `curl | sh`. Der Healthcheck ruft das eigene Binary in Exec-Form gegen
Loopback (`:140-141`), ohne Shell und ohne curl im Image. Der Build-Kontext ist gefiltert
(`deploy/Dockerfile.dockerignore` schliesst `.git`, `.github`, `**/.env`, `docs`, `testdata`,
`config`, `cache` aus). **Compose:** Medien sind read-only eingehängt, wie die Spec verlangt
(`compose.yaml:25`, `:/media:ro`); **kein `privileged`, kein `cap_add` in irgendeiner Compose-Datei**;
GPU über den minimal nötigen Weg (`/dev/dri` plus `group_add` bei Intel/AMD, explizit aufgezählte
Capabilities bei NVIDIA); keine Standard-Zugangsdaten irgendwo; `restart: unless-stopped` statt
`always`. **Client-Adresse — der Standard ist fail-closed, und das ist der wichtigste Punkt:**
`ParseTrustedProxies("")` liefert `nil` (`config.go:336-339`), und `clientIP` gibt bei
`len(trusted) == 0` sofort den Host von `RemoteAddr` zurück, ohne `X-Forwarded-For` auch nur
anzusehen (`session.go:198-200`) — ohne Konfiguration ist der Header schlicht wirkungslos. Der
Algorithmus selbst («der letzte Eintrag, der kein vertrauenswürdiger Proxy ist») ist die korrekte
Variante, und eine kaputte Kette wird nicht geglaubt, sondern fällt auf `RemoteAddr` zurück
(`:204-206`). Die Bucket-Tabelle ist gegen Wachstum durch gefälschte Adressen gedeckelt
(`maxBuckets = 10_000`, LRU-Verdrängung, `limits.go:19-21`, `:63-73`). **Was daran hängt, ist eng
begrenzt:** alle Verbraucher der Client-Adresse münden in `AllowAttempt`; die PIN-Sperre ist profil-
und gerätebasiert, es gibt keine «lokales Netz»-Vertrauensentscheidung und keine IP-Bindung der
Sitzung. Der Weg über Caddy ist in Ordnung: Caddy übernimmt ein eingehendes `X-Forwarded-For` nur,
wenn der direkte Peer in Caddys eigenem `trusted_proxies` steht, was im `Caddyfile` nicht gesetzt und
per Default leer ist — Caddy **überschreibt** den Header also. **CORS ist nicht permissiv:** im ganzen
Server gibt es genau ein `Access-Control-Allow-Origin`, am opt-in Spike-Endpunkt und ohne Credentials
(H-7); es gibt keine CORS-Middleware und kein `ACAO` auf den API-Routen — die befürchtete Kombination
«permissives CORS plus Cookie-Auth» liegt **nicht** vor. **Ausgehend ist gegen SSRF gehärtet:**
`netguard` prüft auf der **aufgelösten** Adresse über `Dialer.Control`
(`netguard/netguard.go:53-62`), greift damit auch bei Umleitungen und DNS-Rebinding, lehnt Loopback,
RFC 1918, ULA, Link-Local und Unspecified ab (`:31-34`), nimmt keinen Proxy aus der Umgebung (`:66`)
und reduziert Fehler-URLs auf Schema und Host, damit Feed-Zugangsdaten nicht ins Log laufen
(`:116-122`). Offene Ränder: **M-1**, **N-6**, **N-7**, **H-8**, **H-9**.

### 9. Der neue Release-Weg — **hält; ein Fremd-PR erreicht nichts Privilegiertes**

Das war die Frage mit dem grössten neuen Risiko, und die Antwort ist klar. Im **ganzen**
`.github/`-Verzeichnis gibt es **keinen** `pull_request_target`, **keinen** `workflow_run` und
**keinen** `issue_comment`-Trigger — kein einziger Treffer. `ci.yml:10-14` triggert auf `push` (nur
`main`), `pull_request` und `workflow_dispatch`. Fremder PR-Code läuft damit ausschliesslich im
`pull_request`-Kontext, in dem GitHub das `GITHUB_TOKEN` zwingend auf read-only setzt und keine
Repository-Secrets exponiert. `ci.yml:22-23` setzt zusätzlich ein explizites
`permissions: contents: read` auf Top-Level und verwendet **kein einziges Secret**; dass der Job
untrusted Code ausführt (`pnpm install --frozen-lockfile` mit Lifecycle-Scripts in `:50`, `task ci` in
`:138`, `sudo apt-get` in `:84-86`) ist bei einem CI, das den PR bauen soll, unvermeidbar und hier
folgenlos, weil nichts da ist, was gestohlen werden könnte. **Keine self-hosted Runner:** alle drei
Jobs laufen auf `ubuntu-24.04` (`ci.yml:27`, `:158`, `release.yml:23`), also auf ephemeren
GitHub-Runnern — das `sudo` im CI ist damit unbedenklich; auf einem self-hosted Runner wäre es ein
kritischer Befund gewesen. **Die Caches sind gegen Vergiftung aus einem Fork geschützt** — ich habe
das eigens geprüft, weil es die richtige Frage war: `ci.yml` schreibt vier Caches (`:58`, `:109`,
`:127`, BuildKit `cache-to: type=gha,mode=max` in `:178`), `release.yml:64` liest mit
`cache-from: type=gha`; GitHubs Cache-Scoping isoliert Schreibzugriffe eines `pull_request`-Laufs auf
`refs/pull/<n>/merge`, ein Lauf auf einem Tag-Ref liest nur den eigenen Scope und den des
Default-Branch. Der verbleibende Weg — ein Lauf auf `main` vergiftet den Cache des späteren
Release-Builds — erfordert Merge-Rechte und liegt ausserhalb des Bedrohungsmodells. **Veröffentlicht
wird nur auf einen Tag:** `on: push: tags: ["v*.*.*"]` (`release.yml:13-15`); nichts geht raus, bevor
jemand mit Push-Rechten taggt. Offene Ränder: **M-2**, **N-8**, **H-10**.

---

## Reihenfolge der Behebung

| | Was | Wann |
|---|---|---|
| 1 | **M-1** — `deploy/compose.yaml:3` korrigieren (Minuten), Start-Warnung ergänzen (eine halbe Stunde) | vor dem öffentlichen Release |
| 2 | **M-2** — Actions auf SHAs pinnen, `.github/dependabot.yml` für `github-actions` anlegen | vor dem ersten Tag |
| 3 | **N-8** — Provenance/Attestation; passt in denselben Commit wie M-2 | vor dem ersten Tag |
| 4 | **H-7** — Spike-Endpunkt entfernen oder als Entscheid festhalten (CLAUDE.md: Platzhalter sind Blocker) | vor dem Veröffentlichen |
| 5 | **N-3** — `.tool-versions` auf `golang 1.27.3`, Beleg gegen das publizierte Image | vor dem Tag |
| 6 | **N-1, N-2** — die zwei Wächter-Lücken schliessen; beide sind je ein kleiner Zusatz an einer vorhandenen Stelle | kurz nach 1.0 |
| 7 | **N-4 … N-7, H-1 … H-11** | geplant |

**Nichts zu rotieren.** Es liegt kein echtes Geheimnis im Arbeitsbaum und keines in der Historie.
