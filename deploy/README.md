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
  Speicher des Emulators mit (gemessen rund 70 MB statt 24 MB nativ). Die Zeit bis `healthy` gilt
  weiterhin mit 5 s Grenze. Echte Werte für arm64 liefert erst ein arm64-Rechner.
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
`ghcr.io/slouchcat/slouchcat:0.1`; der Besitzer `slouchcat` ist ein Platzhalter, bis das öffentliche
GitHub-Repository existiert (ADR 0001, offene Folgepunkte). Ohne veröffentlichtes Image baut
`docker compose up --build` lokal aus dem Dockerfile.

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
| `TZ` | (Compose: `Europe/Zurich`) | Zeitzone |
| `NVIDIA_VISIBLE_DEVICES`, `NVIDIA_DRIVER_CAPABILITIES` | `all`, `compute,video,utility` | für das NVIDIA Container Toolkit |

### Healthcheck

Das Image definiert den Healthcheck selbst (`slouchcat healthcheck` ruft
`http://127.0.0.1:8096/api/v1/healthz` auf; kein curl im Image), Compose übernimmt ihn:
Intervall 5 s, Timeout 2 s, Startphase 1 s mit Prüfung jede Sekunde (`--start-interval`, Docker Engine
ab 25), 3 Versuche.

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
folgen `VmRSS` von PID 1 und `docker stats --no-stream`; liegt das RSS über 60 MB (DoD M0), schlägt
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
