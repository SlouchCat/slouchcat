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

## Audioausgabe für die Playwright-Tests

Firefox spielt einen reinen Audiotitel über seine Audioausgabe ab. Fehlt sie (Rechner ohne Soundkarte,
Container, CI-Runner), scheitert die Ausgabe, das Medienelement meldet `MediaError 3`
(«OnMediaSinkAudioError»), hls.js hängt es ab und neu an — und die Hörbuch-Specs kommen nie vom Fleck.
Auf Marcos Rechner ist nichts zu tun. Auf einem Rechner ohne Soundkarte braucht es einmal eine
Dummy-Ausgabe, wie sie auch die CI einrichtet:

```bash
sudo apt-get install -y pulseaudio pulseaudio-utils
pulseaudio --start --exit-idle-time=-1
pactl load-module module-null-sink sink_name=slouchcat_null
```

Chromium und WebKit bringen ihre eigene Ausgabe mit und brauchen das nicht.

## Täglich

```bash
task ci          # alles bauen und prüfen (wie CI)
task gen         # Codegen: OpenAPI → Go und TypeScript, sqlc, Tokens
task dev         # Server und Vite-Dev-Server
task --list      # alle Ziele
```
