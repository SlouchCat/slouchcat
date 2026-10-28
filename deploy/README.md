# Auslieferung: Docker-Image und Compose

Slouchcat läuft als ein Container (Plan 3.7, ADR 0001 "Auslieferung"). Dieser Ordner enthält:

| Datei | Zweck |
|---|---|
| `Dockerfile` | Mehrstufiger Build: Web-App (Node 24, pnpm), Go-Binary (cross-kompiliert, `CGO_ENABLED=0`), Laufzeit-Image `debian:trixie-slim` mit jellyfin-ffmpeg |
| `Dockerfile.dockerignore` | Ausschlüsse für den Build-Kontext (Repository-Wurzel); BuildKit liest die Datei neben dem Dockerfile |
| `compose.yaml` | Standardbetrieb ohne GPU |
| `compose.nvidia.yaml`, `compose.intel.yaml`, `compose.amd.yaml` | GPU-Overrides |
| `.env.example` | Vorlage für `.env` (Medienordner, Benutzer, Render-Gruppe, Zeitzone) |
| `licenses/FFMPEG-NOTICE.md` | GPL-Hinweis und schriftliches Quellcode-Angebot für ffmpeg, landet in `/licenses` |
| `scripts/measure-healthy.sh` | Misst Zeit bis `healthy` und Leerlauf-RSS (DoD M0) |
| `Taskfile.yml` | `task deploy:build`, `deploy:build:arm64`, `deploy:build:multi`, `deploy:binfmt`, `deploy:up`, `deploy:down`, `deploy:config`, `deploy:healthy-time`, `deploy:healthy-time:arm64` |

## Image bauen

Der Build-Kontext ist die Repository-Wurzel, das Dockerfile liegt hier:

```bash
task deploy:build
# entspricht (aus deploy/):
docker buildx build --platform linux/amd64 --load -t slouchcat:dev \
  --build-arg VERSION=$(git describe --always --dirty) -f Dockerfile ..
```

Ablauf im Dockerfile:

1. **web** (`node:24-trixie-slim`, läuft auf der Build-Plattform): `corepack install` aktiviert genau die
   pnpm-Version aus `package.json#packageManager`, dann `pnpm fetch` und `pnpm install --frozen-lockfile
   --offline`, danach `pnpm --filter "@slouchcat/web..." run build` (baut die Web-App samt abhängiger
   Pakete wie den Design-Tokens) nach `web/dist`.
2. **server** (`golang:1.27-trixie`, Build-Plattform): `web/dist` wird nach
   `server/internal/webassets/dist/` kopiert und eingebettet; `CGO_ENABLED=0 GOOS=linux
   GOARCH=$TARGETARCH go build -trimpath -ldflags "-s -w -X main.version=$VERSION"`.
3. **runtime** (`debian:trixie-slim`, Zielplattform): jellyfin-ffmpeg aus dem Jellyfin-apt-Repository
   (Schlüssel und `.sources`-Datei wie im offiziellen Jellyfin-Image), `ffmpeg` und `ffprobe` als Symlinks
   in `/usr/local/bin`, `tzdata`, `ca-certificates`; `curl` und `gnupg` werden nach der Installation
   wieder entfernt. Benutzer `slouchcat` (UID/GID 1000), `/config` und `/cache` gehören ihm.

Build-Argumente:

| Argument | Standard | Bedeutung |
|---|---|---|
| `VERSION` | `dev` | Version im Binary (`main.version`) und im Label `org.opencontainers.image.version` |
| `FFMPEG_PACKAGE` | `jellyfin-ffmpeg7` | Paketname; `jellyfin-ffmpeg8` ist für trixie ebenfalls verfügbar |
| `FFMPEG_VERSION` | `7.1.4-3` | Gepinnte Version ohne Suite-Suffix; leer = neueste Version |
| `FFMPEG_SUITE` | `trixie` | Debian-Codename im Jellyfin-Repository; muss zu `BASE_IMAGE` passen |
| `BASE_IMAGE` | `debian:trixie-slim` | Laufzeit-Basis; Rückfall `debian:bookworm-slim` zusammen mit `FFMPEG_SUITE=bookworm` |
| `GO_IMAGE`, `NODE_IMAGE` | `golang:1.27-trixie`, `node:24-trixie-slim` | Build-Images |

**arm64** (ab M1b): siehe [arm64 und beide Plattformen](#arm64-und-beide-plattformen).

## arm64 und beide Plattformen

Das Image gibt es für `linux/amd64` und `linux/arm64` (P-1). Web-App und Go-Binary entstehen auf der
Build-Plattform (Go cross-kompiliert mit `GOARCH=$TARGETARCH`); nur die apt-Stufe des Laufzeit-Images
läuft für arm64 unter QEMU. Auf einem amd64-Rechner braucht das einmal pro Systemstart den
binfmt-Handler:

```bash
task deploy:binfmt            # docker run --privileged --rm tonistiigi/binfmt --install arm64
task deploy:build:arm64       # slouchcat:dev-arm64, --load
task deploy:healthy-time:arm64  # startet das arm64-Image emuliert, Zeit bis healthy
task deploy:build:multi       # amd64 und arm64 in einem Lauf, ohne --load
```

- `build:arm64` ist `build` mit `PLATFORM=linux/arm64` und dem Tag `slouchcat:dev-arm64`.
- `build:multi` baut `PLATFORMS` (Standard `linux/amd64,linux/arm64`) mit `OUTPUT=type=cacheonly`: Das
  prüft nur, dass beide Plattformen bauen, wie die CI. `OUTPUT=type=oci,dest=/tmp/slouchcat.oci.tar`
  schreibt ein OCI-Archiv; sobald es eine Registry gibt, pusht `OUTPUT=type=registry` mit passendem
  `IMAGE`.
- `healthy-time:arm64` ist `scripts/measure-healthy.sh` mit `MAX_HEALTHY_SECONDS=30` und `MAX_RSS_MB=120`: Unter QEMU zählt der
  Speicher des Emulators mit (gemessen rund 70 MB statt 24 MB nativ), und der emulierte Start dauert
  hier 1.3 s, auf einem GitHub-Runner rund 7 s; die 5-s-Grenze von P-1 gilt für echte Hardware. Echte
  Werte für arm64 liefert erst ein arm64-Rechner.
- `TARGETOS`/`TARGETARCH` stehen im Dockerfile **ohne** Standardwert: BuildKit setzt sie aus
  `--platform`, ein Standardwert hätte Vorrang (ein früheres `ARG TARGETARCH=amd64` baute für arm64 ein
  amd64-Binary und eine amd64-apt-Quelle). `ARG VERSION` steht im Laufzeit-Image erst nach der
  apt-Stufe, damit eine neue Version deren Cache nicht verwirft (arm64: 113 s emuliert gegenüber 2 s aus
  dem Cache).

Gemessen am 19. Oktober 2026 (Entwicklungsrechner, Docker 29, QEMU über `tonistiigi/binfmt`):

| Schritt | amd64 | arm64 (emuliert) |
|---|---|---|
| Build ohne apt-Cache (Web und Go aus dem Cache) | 31 s, davon apt-Stufe 22.5 s | 117 s, davon apt-Stufe 113 s |
| Build mit neuer `VERSION` | – | 2 s |
| `GET /api/v1/healthz` antwortet | – | 0.62 / 0.74 / 0.82 s |
| Docker-Status `healthy` | 1.19 / 1.20 / 1.25 s | 1.32 / 1.41 / 1.44 s |
| Leerlauf-RSS | 23.5–24.6 MB | 70–71 MB (inkl. QEMU) |

Im arm64-Container: `uname -m` = `aarch64`, ELF-Maschine des Binarys AArch64,
`ffmpeg version 7.1.4-Jellyfin`, Migrationen und Schlüsselerzeugung wie unter amd64.

**CI:** Der Job `image` in `.github/workflows/ci.yml` baut beide Plattformen mit
`docker/setup-qemu-action`, `docker/setup-buildx-action` und `docker/build-push-action` (`push: false`,
Cache `type=gha`), lädt danach jedes Image einzeln und lässt `scripts/measure-healthy.sh` laufen
(arm64 mit `MAX_RSS_MB=120`). Er läuft parallel zum Job `ci`; gepusht wird nichts, solange es keine
Registry gibt (ADR 0001).

**Hardware-Treiber:** jellyfin-ffmpeg bringt libva, die Intel-VA-Treiber (iHD, i965), die oneVPL-Laufzeit
für QSV und Mesa radeonsi für AMD-VAAPI unter `/usr/lib/jellyfin-ffmpeg/lib` selbst mit. Die Debian-Pakete
`intel-media-va-driver-non-free`, `mesa-va-drivers` und `libva2` sind deshalb nicht installiert (das
offizielle Jellyfin-Image macht es gleich). Die Intel-OpenCL-Laufzeit (nur für `tonemap_opencl` nötig)
fehlt bewusst; QSV (`vpp_qsv`) und VAAPI (`tonemap_vaapi`) brauchen sie nicht. Ob AMD `tonemap_opencl`
eine OpenCL-Laufzeit braucht, wird in M1b bei der HDR-Pipeline entschieden (Plan 3.5, best effort).

## Starten

```bash
cd deploy
cp .env.example .env          # MEDIA_DIR, PUID, PGID, RENDER_GID, TZ anpassen
mkdir -p config cache         # vor dem ersten Start, sonst legt Docker sie als root an
docker compose up -d          # oder: task deploy:up
docker compose ps             # Status "healthy" nach wenigen Sekunden
```

Die Web-Oberfläche läuft auf <http://localhost:8096>. `compose.yaml` verweist auf
`ghcr.io/slouchcat/slouchcat:${SLOUCHCAT_VERSION}` (Standard `1.2.0`); das Image entsteht bei jedem
Release (`.github/workflows/release.yml`, ADR 0001). Die Fassung ist bewusst festgenagelt: eine
Aktualisierung ist ein eigener Schritt (`SLOUCHCAT_VERSION` in `.env` ändern, dann
`docker compose pull && docker compose up -d`), die Update-Prüfung sagt nur, dass es eine gibt.
`docker compose up --build` baut stattdessen lokal aus dem Dockerfile.

### Volumes

| Pfad im Container | Host (Standard) | Inhalt |
|---|---|---|
| `/config` | `./config` | SQLite-Datenbank, Einstellungen, Sicherungen, `secret.key`. **Nur lokale Platte**, nie NFS/SMB: der Server prüft das beim Start per `statfs` und bricht sonst ab |
| `/cache` | `./cache` | Regenerierbar (Transcodes, Bilder) |
| `/media` | `${MEDIA_DIR:-/srv/media}` | Medien, nur lesend (`:ro`); darf auf SMB/NFS liegen |

Der Container läuft als `${PUID:-1000}:${PGID:-1000}`; `./config` und `./cache` müssen diesem Benutzer
gehören (`sudo chown -R 1000:1000 config cache`, falls Docker sie als root angelegt hat).

### Medien auf SMB-Freigaben (CIFS)

Der Watcher erkennt Änderungen auf Netzfreigaben per Stat-Durchlauf (Plan 3.4). Auf SMB kommt ein
Client-Cache dazu: Seit Samba 4.22 vergibt der Server **Directory Leases**, und der Linux-CIFS-Client hält
Verzeichnislistings dann bis zu `dir_cache_timeout` (Standard 30 s, verlängert sich bei Nutzung). Legt ein
Programm **direkt auf dem NAS** eine Datei an (Radarr oder Sonarr auf dem NAS selbst), erfährt der Client das
unter Umständen erst Minuten später; die Messung auf dem Testserver zeigte 218–445 s statt < 60 s
(`docs/perf/M1a.md`, Abschnitt 1b). Abhilfe auf dem Docker-Host, auf dem die Freigabe gemountet ist:

```bash
# sofort (bis zum Neustart)
echo 0 | sudo tee /sys/module/cifs/parameters/dir_cache_timeout
# dauerhaft
echo 'options cifs dir_cache_timeout=0' | sudo tee /etc/modprobe.d/cifs-slouchcat.conf
```

Nicht `drop_dir_cache` schreiben: auf Kernel 7.0 löst das einen Kernel-Fehler aus («scheduling while
atomic»). Schreibt Radarr/Sonarr über SMB von einem anderen Rechner, informiert Samba den Client selbst;
mit dem Radarr/Sonarr-Webhook (M9) entfällt die Abhängigkeit vom Polling ganz (AN-12). NFS ist nicht
betroffen (gemessen: 48 s bis zur Sichtbarkeit, `docs/perf/M1a.md` 1a).

### Umgebungsvariablen

| Variable | Standard im Image | Bedeutung |
|---|---|---|
| `SLOUCHCAT_CONFIG_DIR` | `/config` | Konfigurations- und Datenbankordner |
| `SLOUCHCAT_CACHE_DIR` | `/cache` | Cache-Ordner |
| `SLOUCHCAT_LOG_FORMAT` | `json` | `json` oder `text` |
| `SLOUCHCAT_LOG_LEVEL` | `info` | Log-Stufe |
| `SLOUCHCAT_TRUSTED_PROXIES` | leer | Welche Proxys `X-Forwarded-For` setzen dürfen: leer = niemand, `private` = RFC-1918-Netze, Loopback und `fc00::/7`, sonst eine kommagetrennte CIDR-Liste. Siehe «HTTPS mit Caddy» |
| `SLOUCHCAT_DOMAIN`, `ACME_EMAIL` | (nur Compose, `.env`) | Host-Name und ACME-Adresse für das Profil `https`; sie lesen Caddy, nicht der Server |
| `SLOUCHCAT_TMDB_API_KEY` | leer | TMDB-Lesetoken; übersteuert den im Assistenten gespeicherten Schlüssel |
| `SLOUCHCAT_TVDB_API_KEY`, `SLOUCHCAT_TVDB_PIN` | leer | TheTVDB (optional), siehe «Metadaten-Anbieter und Trailer» |
| `SLOUCHCAT_YOUTUBE_API_KEY` | leer | YouTube Data API v3 für Trailer (optional) |
| `SLOUCHCAT_TRAKT_CLIENT_ID` | leer | Trakt Client ID für die Reihe «Spotlight» (optional, M7); welche öffentliche Liste sie zeigt, steht in der Verwaltung unter «Spotlight». Ohne Schlüssel bleibt die Reihe ausgeblendet |
| `SLOUCHCAT_TRAILER_AI` | `off` | KI-Schritt der Trailer-Suche; in dieser Version nur `off` |
| `SLOUCHCAT_SMTP_HOST`, `SLOUCHCAT_SMTP_PORT`, `SLOUCHCAT_SMTP_USER`, `SLOUCHCAT_SMTP_PASSWORD`, `SLOUCHCAT_SMTP_FROM` | leer | E-Mail-Server der Benachrichtigungen (optional, M8); übersteuert den in der Verwaltung hinterlegten Server. Ohne Port gilt 587, 465 oder 25 je nach `SLOUCHCAT_SMTP_TLS` |
| `SLOUCHCAT_SMTP_TLS` | `starttls` | Verschlüsselung zum E-Mail-Server: `starttls`, `tls` (implizit) oder `none` |
| `SLOUCHCAT_TELEGRAM_BOT_TOKEN` | leer | Token des Telegram-Bots von @BotFather (optional, M8); übersteuert das in der Verwaltung hinterlegte Token |
| `SLOUCHCAT_PUBLIC_URL` | leer | Öffentliche Adresse des Servers für Links in E-Mails und Nachrichten (optional, M8); übersteuert die Einstellung in der Verwaltung |
| `SLOUCHCAT_UPDATE_MANIFEST_URL` | Release-Manifest auf GitHub | Quelle der Update-Prüfung (M8); ein anderer Wert gilt nur mit `--dev` (Tests mit Mock) |
| `SLOUCHCAT_TELEGRAM_BASE_URL` | leer | Basis-URL eines Telegram-Mocks; gilt nur mit `--dev` (Tests) |
| `TZ` | (Compose: `Europe/Zurich`) | Zeitzone |
| `NVIDIA_VISIBLE_DEVICES`, `NVIDIA_DRIVER_CAPABILITIES` | `all`, `compute,video,utility` | für das NVIDIA Container Toolkit |

### Metadaten-Anbieter und Trailer

Slouchcat holt Titeldaten, Bilder, Besetzung und Freigaben bei externen Anbietern; jede Installation
bringt ihre eigenen Schlüssel mit (Entscheid 7, ADR 0006). Schlüssel gehören nur in `deploy/.env`,
nie ins Repository; das Log maskiert sie.

| Anbieter | Pflicht? | Wofür | Schlüssel |
|---|---|---|---|
| TMDB | ja | Filme und Serien (Hauptquelle): Texte de/en, Genres, Freigaben, Besetzung, Poster, Hintergründe, Episodenbilder, Trailer-Liste | Lesetoken (v4) im Einrichtungsassistenten oder `SLOUCHCAT_TMDB_API_KEY` |
| TheTVDB | nein | Serien, die TMDB nicht kennt, und von Hand zugeordnete TVDB-IDs | `SLOUCHCAT_TVDB_API_KEY`, bei einem nutzerfinanzierten Schlüssel («user-supported») zusätzlich die Abonnenten-PIN als `SLOUCHCAT_TVDB_PIN`; die Lizenzbedingungen stehen auf thetvdb.com |
| YouTube Data API v3 | nein | Trailer für Deutsch und Englisch, wenn TMDB keinen nennt | `SLOUCHCAT_YOUTUBE_API_KEY` (kostenlos) |
| Trakt | nein | Eine öffentliche Liste nach Wahl für die Reihe «Spotlight» («Trending bei Trakt», «Beliebt bei Trakt», «Meistgesehen», «Meistgesammelt», «Am meisten erwartet», «Kinocharts» oder die Adresse einer öffentlichen Liste), im eingestellten Abstand mit der eigenen Bibliothek verschnitten; kein Login je Profil, nichts wird hochgeladen (I-7) | `SLOUCHCAT_TRAKT_CLIENT_ID` (kostenlos: unter https://trakt.tv/oauth/applications eine App anlegen, Redirect URI `urn:ietf:wg:oauth:2.0:oob`; nur die Client ID zählt) |

YouTube-Schlüssel in drei Schritten: In der Google Cloud Console (`console.cloud.google.com`) ein neues
Projekt anlegen und unter «APIs & Dienste → Bibliothek» die «YouTube Data API v3» aktivieren. Danach
unter «Anmeldedaten» mit «Anmeldedaten erstellen → API-Schlüssel» einen Schlüssel erzeugen. Den
Schlüssel in `deploy/.env` als `SLOUCHCAT_YOUTUBE_API_KEY` eintragen und den Container neu starten.

Das kostenlose Kontingent beträgt 10'000 Einheiten pro Tag; eine Suche kostet 100, die Abfrage der
Videodetails 1 Einheit, also höchstens rund 100 Suchen täglich. Das Kontingent beginnt um Mitternacht
pazifischer Zeit (9 Uhr MEZ, 10 Uhr MESZ) neu. Ist es aufgebraucht, bleibt der Trailer eines Titels
«ausstehend» und wird nach dem nächsten Bibliotheks-Scan erneut gesucht. Slouchcat speichert nur die
Video-ID und spielt Trailer über `youtube-nocookie.com` ein; es lädt nie etwas von YouTube herunter.
Von Hand eingetragene Trailer (Verwaltung eines Titels) bleiben bei jeder Aktualisierung erhalten.

`SLOUCHCAT_TRAILER_AI` ist für einen späteren KI-Schritt der Trailer-Suche reserviert; in dieser
Version gibt es nur `off` (ein anderer Wert erzeugt eine Warnung im Log und bleibt wirkungslos).

### Healthcheck

Das Image definiert den Healthcheck selbst (`slouchcat healthcheck` ruft
`http://127.0.0.1:8096/api/v1/healthz` auf; kein curl im Image), Compose übernimmt ihn:
Intervall 5 s, Timeout 2 s, Startphase 1 s mit Prüfung jede Sekunde (`--start-interval`, Docker Engine
ab 25), 3 Versuche.

## HTTPS mit Caddy (Compose-Profil `https`)

Das Profil `https` startet zusätzlich zum Server einen Caddy (`caddy:2.10`), der TLS beendet und an
`slouchcat:8096` weiterreicht. Ohne das Profil läuft Caddy nicht und es ändert sich nichts. Der Server
setzt das `Secure`-Attribut am Sitzungs-Cookie anhand von `X-Forwarded-Proto`; Caddy sendet den Header
selbst. Browser verlangen HTTPS für Dienste wie den Service Worker der PWA (ausser auf `localhost`), für
Zugriffe von aussen ist das Profil also der vorgesehene Weg.

**Voraussetzungen**

1. Ein DNS-Eintrag (A und gegebenenfalls AAAA) für die Domain, z. B. `media.example.org`, der auf die
   öffentliche Adresse des Docker-Hosts zeigt. Bei wechselnder Heimadresse ein DynDNS-Name.
2. Im Router die Ports **80/tcp**, **443/tcp** und **443/udp** (HTTP/3) auf den Docker-Host
   weiterleiten. Port 80 braucht Let's Encrypt für die Prüfung und Caddy für die Weiterleitung auf HTTPS.
3. Die Ports 80 und 443 müssen auf dem Host frei sein (kein anderer Webserver).

**Konfiguration** in `.env` (Vorlage `.env.example`):

```bash
SLOUCHCAT_DOMAIN=media.example.org   # Pflicht; ohne Wert gilt "localhost" (siehe unten)
ACME_EMAIL=admin@example.org         # optional, für Ablaufhinweise von Let's Encrypt
SLOUCHCAT_TRUSTED_PROXIES=private    # Caddy läuft im privaten Docker-Netz; nur mit compose.https-only.yaml (s. u.)
```

`SLOUCHCAT_TRUSTED_PROXIES` ist wichtig: Ohne diese Einstellung ignoriert der Server `X-Forwarded-For` und
sähe jeden Zugriff als Zugriff von der Caddy-Adresse im Docker-Netz. Das verfälscht die Zuordnung der
Anmeldeversuche (Begrenzung der Fehlversuche je Client) und das Log.

**`private` ist nur sicher, wenn Port 8096 für nicht vertrauenswürdige Clients unerreichbar ist.** `private`
vertraut jedem Absender aus einem privaten Netz (RFC 1918, Loopback, `fc00::/7`), nicht nur Caddy. Caddy
überschreibt `X-Forwarded-For` bei allem, was über ihn läuft, aber ein Gerät im Heimnetz, das direkt auf
`http://<host>:8096` zugreift, ist ebenfalls ein «privater» Absender und kann den Header selbst setzen und
sich damit eine beliebige Client-Adresse geben (und so die Begrenzung der Anmeldeversuche je Adresse
umgehen). Deshalb ist der **normale Weg für das Profil `https`**, den Override `compose.https-only.yaml`
dazuzunehmen: Dann veröffentlicht der Host Port 8096 gar nicht, und der Server ist nur über Caddy erreichbar.
Wo 8096 trotzdem im LAN offen bleibt, statt `private` die genaue Adresse oder das Docker-Netz von Caddy als
CIDR eintragen (`docker network inspect` zeigt es). Die Sperre falscher PINs hängt nicht von dieser
Einstellung ab (sie zählt je Sitzung und Profil), nur die Begrenzung der Anmeldeversuche je Adresse.

**Start**

```bash
cd deploy
mkdir -p config cache
docker compose --profile https up -d      # oder: task deploy:up:https
docker compose --profile https logs -f caddy
```

Caddy startet erst, wenn der Server `healthy` ist (`depends_on`, `service_healthy`).

**Erstes Zertifikat.** Beim ersten Aufruf holt Caddy das Zertifikat von Let's Encrypt (HTTP-01 auf Port 80,
TLS-ALPN-01 auf Port 443). Das dauert einige Sekunden; im Log erscheint `certificate obtained
successfully`. Danach ist die Seite unter `https://media.example.org` erreichbar, `http://` leitet
automatisch um. Die Erneuerung läuft selbstständig (etwa 30 Tage vor Ablauf). Zertifikate und ACME-Konto
liegen im Volume `caddy_data`: dieses Volume nicht löschen (`docker compose down -v` löscht es), sonst
fordert Caddy neue Zertifikate an und stösst an die Ratenbegrenzung von Let's Encrypt.

**Konfigurationsprüfung.** `task deploy:caddy:validate` prüft die `Caddyfile` mit dem Caddy-Image (einmal mit
leerer, einmal mit gesetzter `ACME_EMAIL`).

### Test ohne öffentliche Domain

Mit `SLOUCHCAT_DOMAIN=localhost` stellt Caddy das Zertifikat aus seiner **internen CA** aus; kein DNS, kein
Router, kein Internet nötig. Der Browser zeigt eine Warnung, solange er der Caddy-CA nicht vertraut (`curl`
mit `-k`). Das reicht, um Login, `Secure`-Cookie und Streaming über HTTPS zu prüfen. Dasselbe gilt für einen
Namen im Heimnetz (z. B. `slouchcat.home.arpa` in der lokalen DNS-Auflösung): Let's Encrypt kann ihn nicht
prüfen, also nimmt man die interne CA. Wer die Warnung auf einem Gerät nicht sehen will, installiert das
Stammzertifikat der Caddy-CA dort:

```bash
docker compose --profile https cp caddy:/data/caddy/pki/authorities/local/root.crt ./caddy-root.crt
```

### Domain nicht aus dem Internet erreichbar

Let's Encrypt prüft über Port 80/443; ist der Host von aussen nicht erreichbar (kein Portforwarding,
CGNAT), schlägt die Ausstellung fehl. Dann gibt es drei Wege:

- **Interne CA** (siehe oben): `tls internal` im Site-Block der `Caddyfile`, der Name darf beliebig sein.
- **Eigenes Zertifikat** (z. B. von einer eigenen CA oder per DNS-Prüfung andernorts erzeugt): Dateien in
  einen Ordner legen, ihn in `compose.yaml` beim Dienst `caddy` einbinden
  (`- ./certs:/certs:ro`) und in der `Caddyfile` im Site-Block ergänzen:

  ```
  tls /certs/fullchain.pem /certs/privkey.pem
  ```

- **DNS-Challenge:** braucht ein Caddy-Image mit dem Plug-in des DNS-Anbieters; das liefert Slouchcat nicht
  mit. Wer das braucht, nimmt den Weg «Eigener Reverse-Proxy» mit einem selbst gebauten Caddy.

Nach einer Änderung an der `Caddyfile`: `docker compose --profile https up -d --force-recreate caddy`.

### Zugriff im LAN auf Port 8096

`compose.yaml` veröffentlicht den Server weiterhin auf Port **8096** (HTTP, ohne TLS). Das ist für das
Heimnetz bequem, aber ungeschützt: Passwörter und Sitzungs-Cookie laufen dort im Klartext, und das Cookie
ist ohne `X-Forwarded-Proto: https` nicht `Secure`. Der Router darf 8096 nie weiterleiten. Mit dem Profil
`https` ist der Override `compose.https-only.yaml` der empfohlene Normalfall: Er veröffentlicht 8096 nicht
mehr, der Server ist nur über Caddy erreichbar, und `SLOUCHCAT_TRUSTED_PROXIES=private` ist dann sicher
(Docker Compose ab 2.24):

```bash
docker compose -f compose.yaml -f compose.https-only.yaml --profile https up -d
```

Der Healthcheck im Container und die Verbindung von Caddy laufen im Docker-Netz und brauchen die
Veröffentlichung nicht. Im LAN lässt sich `https://media.example.org` auch von innen erreichen, wenn der
Router "NAT loopback" kann oder die lokale DNS-Auflösung den Namen auf die LAN-Adresse des Hosts setzt.

## Eigener Reverse-Proxy

Wer schon einen Reverse-Proxy betreibt (nginx, Traefik, ein externer Caddy, ein Router mit Proxy), lässt
das Profil `https` weg und leitet auf `http://<Docker-Host>:8096` weiter. Der Server stellt an den Proxy
diese Anforderungen:

| Anforderung | Grund |
|---|---|
| `X-Forwarded-Proto: https` weiterreichen | Der Server setzt das Sitzungs-Cookie nur dann mit `Secure`, wenn die Anfrage über HTTPS kam; ohne den Header bekommt der Browser ein unsicheres Cookie |
| `X-Forwarded-For` setzen (am besten überschreiben, nicht anhängen) und `SLOUCHCAT_TRUSTED_PROXIES` auf die Adresse des Proxys setzen | Sonst sieht der Server nur die Proxy-Adresse als Client. Eine Adresse oder ein Netz als CIDR (`172.18.0.0/16`), im privaten Netz `private`. Nie `0.0.0.0/0` eintragen: dann kann jeder Aufrufer seine Adresse fälschen |
| Keine Pufferung von Antworten für `/api/v1/items/*/stream` und `/api/v1/playback/*` | Range-Anfragen (Spulen) und lange HLS-Antworten sollen sofort durchlaufen; Pufferung füllt die Platte des Proxys und verzögert den Start |
| Anfragekörper mindestens 1 MB erlauben | nginx begrenzt standardmässig auf 1 MB (`client_max_body_size`) |
| Lese- und Leerlauf-Timeouts mindestens 60 s | Segmentantworten warten auf den Transcoder; nginx `proxy_read_timeout` steht auf 60 s |
| HTTP/2 und HTTP/3 zum Browser sind in Ordnung | Zum Server genügt HTTP/1.1 |
| Kein WebSocket nötig | Der Server nutzt keine WebSockets |
| Keine zusätzliche Kompression nötig | Der Server komprimiert selbst, was sich lohnt; Bilder und Video sind bereits komprimiert |

Der Proxy muss die Anfragen unverändert an die Wurzel weiterreichen (kein Unterpfad wie `/slouchcat/`;
die PWA erwartet die Wurzel einer Domain). Der Healthcheck-Pfad `/api/v1/healthz` ist nicht
anmeldepflichtig und eignet sich für die Überwachung durch den Proxy.

### nginx

```nginx
server {
    listen 443 ssl;
    http2 on;
    server_name media.example.org;
    # ssl_certificate / ssl_certificate_key ...

    client_max_body_size 10m;

    location / {
        proxy_pass http://192.168.1.50:8096;
        proxy_http_version 1.1;
        proxy_set_header Host              $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-For   $remote_addr;   # überschreibt, hängt nichts an
        proxy_set_header Connection        "";
        proxy_read_timeout 300s;
        proxy_send_timeout 300s;
    }

    location ~ ^/api/v1/(items/[^/]+/stream|playback/) {
        proxy_pass http://192.168.1.50:8096;
        proxy_http_version 1.1;
        proxy_buffering off;
        proxy_request_buffering off;
        proxy_set_header Host              $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-For   $remote_addr;
        proxy_set_header Connection        "";
        proxy_read_timeout 300s;
        proxy_send_timeout 300s;
    }
}
```

`$remote_addr` statt `$proxy_add_x_forwarded_for`: ein Aufrufer kann sonst eigene Einträge vorne anhängen.
Steht vor nginx noch ein weiterer Proxy, gehört dessen Adresse in `SLOUCHCAT_TRUSTED_PROXIES`.

### Traefik

Traefik setzt `X-Forwarded-Proto` und `X-Forwarded-For` selbst und überschreibt gefälschte Werte von
Clients. Steht Traefik selbst hinter einem weiteren Proxy (z. B. Cloudflare), muss dessen Netz am
Einstiegspunkt unter `forwardedHeaders.trustedIPs` stehen, sonst sieht Slouchcat die Adresse dieses
Proxys. Als Labels am Dienst `slouchcat` (Traefik und Slouchcat im selben Docker-Netz, Port 8096 nicht
veröffentlichen, `compose.https-only.yaml`):

```yaml
services:
  slouchcat:
    labels:
      - traefik.enable=true
      - traefik.http.routers.slouchcat.rule=Host(`media.example.org`)
      - traefik.http.routers.slouchcat.entrypoints=websecure
      - traefik.http.routers.slouchcat.tls.certresolver=letsencrypt
      - traefik.http.services.slouchcat.loadbalancer.server.port=8096
      - traefik.http.services.slouchcat.loadbalancer.responseforwarding.flushinterval=-1ms
```

Als Datei (dynamische Konfiguration) gleichwertig:

```yaml
http:
  routers:
    slouchcat:
      rule: Host(`media.example.org`)
      entryPoints: [websecure]
      tls: { certResolver: letsencrypt }
      service: slouchcat
  services:
    slouchcat:
      loadBalancer:
        servers:
          - url: http://slouchcat:8096
        responseForwarding:
          flushInterval: -1ms
```

Traefik puffert Antworten nicht, solange keine `buffering`-Middleware gesetzt ist. Am Einstiegspunkt
`respondingTimeouts.writeTimeout` auf `0` lassen (Standard), sonst bricht Traefik lange Streams ab. In der
`.env` des Slouchcat-Stacks `SLOUCHCAT_TRUSTED_PROXIES` auf das Docker-Netz von Traefik setzen (`private`
genügt dort).

### Externer Caddy

Läuft Caddy auf einem anderen Rechner oder ausserhalb von Compose:

```
media.example.org {
	reverse_proxy 192.168.1.50:8096 {
		flush_interval -1
	}
}
```

Caddy setzt `X-Forwarded-For` und `X-Forwarded-Proto` selbst und hat standardmässig weder Körpergrenze
noch Lese-Timeout. Auf dem Slouchcat-Host `SLOUCHCAT_TRUSTED_PROXIES` auf die Adresse dieses Caddy setzen
(z. B. `192.168.1.10/32` oder `private`).

### Prüfen

```bash
curl -sS -D - https://media.example.org/api/v1/auth/session          # 401, Set-Cookie mit "Secure"
# Der Datenstrom prüft das Stream-Token (?st=), nicht das Cookie: Token und URL kommen aus startPlayback.
STREAM=$(curl -sS -b cookie.txt -H 'X-Requested-With: Slouchcat' -H 'Content-Type: application/json' \
  -X POST -d '{}' https://media.example.org/api/v1/items/<id>/play | jq -r .streamUrl)
curl -sS -D - -o /dev/null -H 'Range: bytes=0-99' "https://media.example.org$STREAM"   # 206 und Content-Range
```

(`cookie.txt` stammt aus einer Anmeldung mit `curl -c cookie.txt`, danach `selectProfile`, falls das Konto
mehrere Profile hat; bei HLS-Titeln ist `streamUrl` eine Playlist statt des Direktstroms.)

Das Cookie `slouchcat_session` muss das Attribut `Secure` tragen; fehlt es, kommt `X-Forwarded-Proto` nicht
an. Im Server-Log steht bei Anmeldungen die Adresse des Clients; steht dort die Adresse des Proxys, fehlt
`SLOUCHCAT_TRUSTED_PROXIES` oder `X-Forwarded-For`.

## GPU-Overrides

```bash
docker compose -f compose.yaml -f compose.nvidia.yaml up -d   # NVIDIA
docker compose -f compose.yaml -f compose.intel.yaml up -d    # Intel QSV/VAAPI
docker compose -f compose.yaml -f compose.amd.yaml up -d      # AMD VAAPI (best effort)
task deploy:up COMPOSE="docker compose -f compose.yaml -f compose.intel.yaml"
task deploy:config                                            # alle Kombinationen prüfen
```

- **NVIDIA:** braucht das NVIDIA Container Toolkit auf dem Host. Der Override reserviert eine GPU mit
  den Fähigkeiten `gpu, compute, video, utility`. Docker setzt `NVIDIA_DRIVER_CAPABILITIES` aus dieser
  Liste und überschreibt damit den Image-Standard; mit nur `[gpu, video]` (Auszug in Plan 3.7) fehlten
  `compute` für CUDA-Filter (`scale_cuda`, `tonemap_cuda`) und `utility` für `nvidia-smi`.
- **Intel und AMD:** reichen `/dev/dri` durch und fügen die Host-Gruppe `render` hinzu. `group_add`
  braucht die **numerische** GID: Docker löst Gruppennamen im Container auf, und das Image kennt keine
  Gruppe `render`, ein Name führt zum Startfehler. GID ermitteln und in `.env` eintragen:

  ```bash
  getent group render | cut -d: -f3     # z. B. 992 → RENDER_GID=992
  ```

  Ohne `RENDER_GID` bricht Compose mit einer Meldung ab, statt ohne GPU-Zugriff zu starten.

## Messung (DoD M0)

```bash
task deploy:build
task deploy:healthy-time
```

`scripts/measure-healthy.sh` startet einen Wegwerf-Container aus `slouchcat:dev` mit temporärem
`/config` und `/cache`, fragt alle 100 ms `docker inspect --format '{{.State.Health.Status}}'` ab, gibt
die Sekunden bis `healthy` aus und schlägt fehl, wenn das länger als 5 s dauert. Nach 3 s Leerlauf
folgen `VmRSS` von PID 1 und `docker stats --no-stream`; liegt das RSS über der Schranke (seit dem
23. Oktober 2026 80 MB, Begründung im Kopf des Skripts und in `docs/perf/M9.md` Abschnitt 5), schlägt
das Skript ebenfalls fehl. Grenzwerte über `MAX_HEALTHY_SECONDS`, `MAX_RSS_MB`, `IDLE_SECONDS`.

## Lizenzen im Image

`/licenses/LICENSE` (Slouchcat, AGPL-3.0), `/licenses/FFMPEG-NOTICE.md` (GPL-Hinweis, Quelle
<https://github.com/jellyfin/jellyfin-ffmpeg>, schriftliches Quellcode-Angebot), `/licenses/FFMPEG-VERSION`
(genaue Paketversion) und, falls das Paket sie mitliefert, `/licenses/FFMPEG-COPYRIGHT`.

## Verifikation trixie (M0, 18. Oktober 2026)

Offener Folgepunkt aus ADR 0001: Gibt es jellyfin-ffmpeg für `debian:trixie-slim`?

| Prüfung | Ergebnis |
|---|---|
| `debian:trixie-slim`, `node:24-trixie-slim` auf Docker Hub | vorhanden |
| `golang:1.27-trixie` auf Docker Hub | vorhanden (aktualisiert 13.10.2026; amd64 und arm64) |
| Jellyfin-Repository `https://repo.jellyfin.org/debian`, Suite `trixie` | vorhanden, Architekturen `amd64 arm64` |
| `jellyfin-ffmpeg7` für trixie amd64 | neueste Version `7.1.4-3-trixie` (`jellyfin-ffmpeg7_7.1.4-3-trixie_amd64.deb`, 46 MB), auch unter `files/ffmpeg/debian/latest-7.x/amd64/` |
| `jellyfin-ffmpeg7` für trixie arm64 | `7.1.4-3-trixie` vorhanden |
| `jellyfin-ffmpeg8` für trixie | `8.1.3-1-trixie` vorhanden (das offizielle Jellyfin-Image nutzt inzwischen 8) |
| Quellcode-Tag `v7.1.4-3` auf GitHub | vorhanden |

Ergebnis: **trixie ist für amd64 und arm64 gedeckt, kein Rückfall auf `bookworm-slim` nötig.** Das Image
pinnt `jellyfin-ffmpeg7=7.1.4-3-trixie`; der Wechsel auf `jellyfin-ffmpeg8` ist ein Build-Argument und
wird mit der HDR-Pipeline in M1b entschieden.

Image-Build auf dem Entwicklungsrechner (Docker 29.9.1, buildx 0.38): `task deploy:build` erfolgreich,
`slouchcat:dev` 128 MB komprimiert (507 MB entpackt, davon jellyfin-ffmpeg rund 210 MB). Im Image:
`ffmpeg version 7.1.4-Jellyfin`, Encoder `h264_nvenc`, `h264_qsv`, `h264_vaapi` vorhanden, VA-Treiber
`iHD`, `i965`, `radeonsi` unter `/usr/lib/jellyfin-ffmpeg/lib/dri`.

`task deploy:healthy-time`, drei Läufe: `healthy` nach 1.17 bis 1.20 s (Grenze 5 s), Leerlauf-RSS
13.5 bis 14.1 MB (Grenze 60 MB). `docker compose up` mit `compose.yaml`: Container `healthy`,
`GET /api/v1/healthz` und `/` liefern 200, `ffprobe` liest `/media` (read-only).
