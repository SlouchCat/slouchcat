# Entwicklungs-Setup

Entwicklungsrechner: Linux Mint 22.3 (Ubuntu noble), NVIDIA RTX 4070. Alles ausser Docker und ffmpeg
wird ohne Root unter `~/.local` installiert; die gepinnten Versionen stehen in `.tool-versions`.

## Ohne sudo (von Claude eingerichtet)

| Werkzeug | Ort | Einrichtung |
|---|---|---|
| Go | `~/.local/share/go`, Link `~/.local/bin/go` | offizielles Tarball von go.dev, SHA-256 geprüft |
| Node.js 24 LTS | `~/.local/share/node`, Link `/usr/local/bin/node` | vorhanden |
| pnpm | `npm install -g pnpm`, Link `~/.local/bin/pnpm` | |
| task | `go install github.com/go-task/task/v3/cmd/task@latest`, Link `~/.local/bin/task` | |
| sqlc, goose, oapi-codegen, golangci-lint | `go tool` aus `server/go.mod` | keine Installation nötig |

Go-Upgrade: neues Tarball nach `~/.local/share/go` entpacken, `go.mod` und `.tool-versions` anpassen.

## Mit sudo (macht Marco im Terminal-Panel)

```bash
sudo bash scripts/install-system-tools.sh
```

Installiert Docker Engine mit Compose- und Buildx-Plugin aus Dockers apt-Repository sowie ffmpeg/ffprobe
aus Ubuntu und nimmt den Benutzer in die Gruppe `docker` auf. Bis zur nächsten Anmeldung:
`sg docker -c 'docker ps'`.

Optional für GPU-Tests im Container (ab M1b): NVIDIA Container Toolkit.

## Täglich

```bash
task ci          # alles bauen und prüfen (wie CI)
task gen         # Codegen: OpenAPI → Go und TypeScript, sqlc, Tokens
task dev         # Server und Vite-Dev-Server
task --list      # alle Ziele
```
