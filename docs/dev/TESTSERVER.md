# Testserver: Proxmox-Container als schwaches NAS

Entscheidungsprotokoll Frage 2: Statt eines Intel-N100-Mini-PCs dient ein LXC-Container (CT) in Marcos
Proxmox im lokalen Netz als Muster-NAS. Claude legt ihn an und dimensioniert ihn absichtlich knapp.
Zugang, GPU-Durchreichung und Limits sind vor M1b festzulegen; dieses Dokument legt sie fest.

## Zweck

1. **SMB-Lastmessung der Dateiüberwachung** (M1a offener Punkt, Plan 3.4, Entscheid 3): der
   Platzhalterbaum (60'800 Dateien, 8'213 Ordner) liegt auf einer Samba-Freigabe des Containers; der
   Entwicklungsrechner mountet sie per CIFS und misst mit `slouchcat watchbench`. NFS ist bereits auf dem
   echten NAS gemessen (48 s bis zur Sichtbarkeit, `docs/perf/M1a.md` 1a).
2. **Leistung auf schwacher Hardware** (ab M1b, Plan 2.2 Driver 1, P-1, P-2, 3.5 Kalibrierung): der
   Slouchcat-Stack läuft per Docker Compose **im Container** und liest die Medien lokal oder vom NAS.
   Startzeit, Leerlauf-RSS, Scan-Dauer und Transcoding ohne GPU werden hier gemessen.

## Dimensionierung (Startwerte, im Admin überschreibbar sind nur Slouchcat-Werte)

| Eigenschaft | Wert | Begründung |
|---|---|---|
| Typ | unprivilegierter LXC-CT, Debian 13 (trixie) Standard-Template | wie die Image-Basis; wenig Aufwand |
| vCPU | 2 Kerne, `cpuunits` 512 | schwächer als ein N100 (4 Kerne) |
| RAM | 4 GB, Swap 1 GB | Hälfte des N100-Vorschlags (8 GB); SQLite + Server + ffmpeg-Software-Transcode müssen damit auskommen |
| Root-Disk | 16 GB | System, Docker-Image (≈ 0.5 GB), Cache |
| Daten-Disk | Mountpoint `/srv/media`, 64 GB | Platzhalterbaum (leer) und einige echte Testdateien |
| Features | `nesting=1,keyctl=1` | Docker im unprivilegierten CT |
| GPU | keine (Startwert) | Software-Transcoding ist die dokumentierte Ausnahme AN-17. Der Host `pve2` hat eine Intel UHD 630 (`/dev/dri/renderD128`); sie wird erst für die QSV/VAAPI-Messung in M1b durchgereicht |
| Netz | `vmbr0`, DHCP, Hostname `slouchcat-nas` | gleiches LAN wie Entwicklungsrechner und NAS |
| Dienste | Samba (Freigabe `media`), Docker + Compose-Plugin, Python 3 (Baum-Generator) | NFS-Server läuft in unprivilegierten CTs nicht zuverlässig; NFS bleibt Sache des echten NAS |
| Zugang | SSH mit Schlüssel als `root` (CT), aus Claude Desktop als gespeicherte SSH-Verbindung | kein Passwort im Repo oder Chat |

Samba-Freigabe `media`: Gäste lesen, Benutzer `media` schreibt (Passwort setzt Marco einmalig im Container mit
`smbpasswd -a media`). Der Platzhalterbaum wird im Container erzeugt (`generate-library-tree.py` wird
mitgeliefert), nicht über das Netz geschrieben.

## Ablauf

### Einmalig durch Marco (sudo-Rechte auf dem Proxmox-Host)

1. In Claude Desktop eine SSH-Verbindung zum Proxmox-Host anlegen (Benutzer mit Recht auf `pct` und
   `pveam`, in der Regel `root`). Adresse und Schlüssel bleiben in Claude Desktop.
2. Danach legt Claude den Container mit `scripts/proxmox/create-testserver.sh` an (Schritt unten) und
   richtet ihn mit `scripts/proxmox/setup-nas.sh` ein. Alternativ führt Marco beide Skripte selbst aus.
3. Nach der Einrichtung: `smbpasswd -a media` im Container (Passwort nur dort), und die IP des Containers
   als zweite SSH-Verbindung in Claude Desktop hinterlegen (für M1b-Messungen im Container).

### Container anlegen (auf dem Proxmox-Host, als root)

```bash
# Vorgabe: VMID 190, Storage local-lvm, Bridge vmbr0; alle Werte sind Variablen des Skripts.
# Auf pve2 heisst der Storage local-zfs.
bash create-testserver.sh --vmid 190 --storage local-zfs --bridge vmbr0 --ssh-key ~/.ssh/id_ed25519.pub
```

Das Skript erwartet `testdata/generate-library-tree.py` zwei Ebenen über sich; auf dem Host also die
Repo-Struktur unter `/root/slouchcat/` nachbilden (`scripts/proxmox/`, `testdata/`). Das Template wird nach
Host-Architektur gewählt (`dpkg --print-architecture`); eine reine Sortierung hätte `arm64` vor `amd64`
genommen, der CT wäre dann nicht gestartet.

Das Skript lädt das Debian-13-Template (`pveam`), erzeugt den CT mit den Werten der Tabelle, startet ihn,
kopiert `setup-nas.sh` und `generate-library-tree.py` hinein und führt die Einrichtung aus. Am Ende druckt
es die IP des Containers.

### SMB-Lastmessung (auf dem Entwicklungsrechner)

```bash
sudo mkdir -p /mnt/testnas
sudo mount -t cifs //<CT-IP>/media /mnt/testnas -o guest,ro,vers=3.1.1,iocharset=utf8
server/bin/slouchcat watchbench --root /mnt/testnas/placeholder-tree --passes 5
```

Der Baum liegt nach `setup-nas.sh` unter `/srv/media/placeholder-tree` (60'800 Dateien). Ergebnis nach
`docs/perf/M1a.md`, Abschnitt 1b. Grenzwert: Stat-Durchlauf < 15 s je 10'000 Dateien, neuer Titel < 60 s.

### Slouchcat im Container (ab M1b)

```bash
# im Container
cd /opt/slouchcat && git clone https://github.com/SlouchCat/slouchcat .   # oder Image von ghcr.io
cp deploy/.env.example deploy/.env && mkdir -p deploy/config deploy/cache
MEDIA_DIR=/srv/media docker compose -f deploy/compose.yaml up -d --build
```

Messungen nach `docs/perf/M1b.md`: `healthy`-Zeit, Leerlauf-RSS, Scan des Platzhalterbaums, Transcode-Start
ohne GPU (AN-17).

## Stand

CT 190 `slouchcat-nas` wurde am 2026-10-19 auf `pve2` angelegt (Storage `local-zfs`, DHCP-Adresse im LAN).
Geprüft: SSH als `root` mit Marcos Schlüssel, Debian 13 amd64, 2 Kerne, 4 GB, Docker 29.9 mit Compose v5
(`hello-world` läuft), `smbd` aktiv und Freigabe `media` vom Entwicklungsrechner anonym sichtbar,
Platzhalterbaum mit 60'800 Dateien in 2.7 s erzeugt.

## Befund SMB-Verzeichniscache (19. Oktober 2026)

Samba 4.22 im Container vergibt Directory Leases; der CIFS-Client auf dem Entwicklungsrechner (Kernel 7.0)
hält das Listing von `Movies/` danach bis zu `dir_cache_timeout` (30 s, bei laufender Nutzung länger).
Eine im Container angelegte Datei war über das Mount erst nach 218–445 s sichtbar, obwohl Poller (3 s)
und Pfad-Scan (2 s) längst schnell genug sind. Für Messungen und für den Betrieb gilt die Abhilfe aus
`deploy/README.md` («Medien auf SMB-Freigaben»): `dir_cache_timeout=0` auf dem Client.

## Noch offen

- `smbpasswd -a media` im Container (Marco, Passwort nur dort).
- SMB-Lastmessung mit `watchbench` über CIFS (Abschnitt oben), Ergebnis nach `docs/perf/M1a.md` 1b.
- GPU: `/dev/dri/renderD128` des Hosts erst für die QSV/VAAPI-Messung in M1b per
  `lxc.cgroup2.devices.allow` + `lxc.mount.entry` durchreichen.
