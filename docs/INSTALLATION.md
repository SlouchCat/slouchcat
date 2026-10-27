# Slouchcat installieren

Diese Anleitung richtet sich an dich, wenn du Slouchcat bei dir zu Hause betreiben willst. Du brauchst
dafür keine Programmierkenntnisse, aber einen Rechner, der durchläuft, und etwas Geduld beim ersten
Einlesen der Sammlung.

Slouchcat ist ein einziger Container. Alles steckt darin: Server, Weboberfläche, Datenbank und ffmpeg.
Es gibt keine zweite Datenbank, keinen Zusatzdienst und keine Plugins.

Technische Einzelheiten, Reverse-Proxys und Messwerte stehen in `deploy/README.md`. Hier steht der
kurze Weg.

## 1. Was du brauchst

| | |
|---|---|
| Rechner | Ein Linux-Rechner oder NAS, das durchläuft. Zwei Kerne und 4 GB Arbeitsspeicher reichen; mit Grafikchip läuft die Umwandlung von Videos deutlich schneller |
| Docker | Docker Engine 25 oder neuer mit dem Compose-Plugin (`docker compose version`) |
| Speicher | Rund 1 GB für das Programm, dazu Platz für Vorschaubilder und zwischengespeicherte Umwandlungen (plane 10 GB) |
| Medien | Deine Filme, Serien, Hörbücher und Podcasts in Ordnern. Slouchcat liest sie **nur**; es verschiebt, benennt und löscht nichts |
| Zugangsschlüssel | Ein kostenloser Lesetoken von [themoviedb.org](https://www.themoviedb.org) (TMDB). Er ist nicht zwingend, aber ohne ihn bleiben deine Titel ohne Beschreibung, Poster und Altersfreigabe — und ohne bekannte Freigabe sind sie für alle Profile unter 18 gesperrt |

Slouchcat läuft auf Linux. Für Apple-Geräte als **Zuschauer** gilt: iPhone, iPad und Mac sind in dieser
Fassung nicht geprüft worden, weil im Projekt keine solchen Geräte vorhanden waren (siehe
«Was nicht geprüft ist» weiter unten).

## 2. Ordner anlegen und das Programm holen

```bash
mkdir -p ~/slouchcat && cd ~/slouchcat
curl -fsSLO https://raw.githubusercontent.com/SlouchCat/slouchcat/v1.1.0/deploy/compose.yaml
curl -fsSL -o .env https://raw.githubusercontent.com/SlouchCat/slouchcat/v1.1.0/deploy/.env.example
mkdir -p config cache
```

Du hast jetzt vier Dinge nebeneinander:

- `compose.yaml` — die Beschreibung des Containers. Diese Datei änderst du nicht.
- `.env` — deine Einstellungen. Diese Datei änderst du gleich.
- `config/` — Datenbank, Einstellungen und Sicherungen. **Dieser Ordner muss auf einer lokalen Platte
  liegen**, nie auf einem Netzlaufwerk; der Server prüft das beim Start und bricht sonst ab.
- `cache/` — Vorschaubilder und umgewandelte Videoabschnitte. Darf jederzeit gelöscht werden.

## 3. `.env` ausfüllen

Öffne `.env` in einem Texteditor. Diese Werte musst du anschauen:

```bash
# Die Fassung, die laufen soll.
SLOUCHCAT_VERSION=1.1.0

# Der Ordner auf deinem Rechner, in dem die Medien liegen.
MEDIA_DIR=/srv/media

# Benutzer und Gruppe, unter denen der Server läuft. `id -u` und `id -g` zeigen deine Werte.
PUID=1000
PGID=1000

# Zeitzone.
TZ=Europe/Zurich
```

`MEDIA_DIR` ist der Ordner, der im Container als `/media` erscheint. Liegen deine Filme unter
`/mnt/nas/medien/Filme` und deine Serien unter `/mnt/nas/medien/Serien`, trägst du
`MEDIA_DIR=/mnt/nas/medien` ein; die Bibliotheken heissen später `/media/Filme` und `/media/Serien`.

Die Ordner `config` und `cache` müssen dem Benutzer aus `PUID`/`PGID` gehören:

```bash
sudo chown -R "$(id -u):$(id -g)" config cache
```

Im Formular «Bibliothek hinzufügen» steht neben dem Ordnerfeld der Knopf «Durchsuchen …». Er zeigt die
Unterordner von `/media`, du klickst dich zum richtigen Ordner und musst den Pfad im Container nicht
auswendig wissen. Aus demselben Grund darf eine neue Bibliothek nur unter `/media` liegen. Hast du in
`compose.yaml` eine zweite Einhängung ergänzt, trägst du sie komma-getrennt in
`SLOUCHCAT_BROWSE_ROOTS` ein (zum Beispiel `SLOUCHCAT_BROWSE_ROOTS=/media,/media2`). Bibliotheken, die
es schon gibt, bleiben in jedem Fall bestehen.

Den TMDB-Lesetoken kannst du hier eintragen (`SLOUCHCAT_TMDB_API_KEY=…`) oder beim ersten Start im
Assistenten angeben. Beides ist gleichwertig; in `.env` bleibt er auch nach einer Wiederherstellung
erhalten. **Schlüssel gehören nur in diese Datei**, nie in eine Nachricht und nie in ein
Repository; das Protokoll des Servers maskiert sie.

## 4. Starten

```bash
docker compose up -d
docker compose ps
```

Nach wenigen Sekunden steht bei `slouchcat` der Zustand `healthy`. Dann ist die Oberfläche unter
<http://localhost:8096> erreichbar — von einem anderen Gerät im Heimnetz unter
`http://<Adresse-des-Rechners>:8096`.

Geht etwas schief, zeigt `docker compose logs slouchcat` den Grund.

### Mit Grafikchip

Ohne Grafikchip wandelt Slouchcat Videos auf dem Hauptprozessor um. Das funktioniert, braucht aber
Zeit: Der Start eines 4K-HDR-Films dauert dann bis zu zehn Sekunden statt unter fünf, und die Ausgabe
ist höchstens 1080p ohne HDR. Mit Grafikchip nimmst du eine zweite Datei dazu:

```bash
curl -fsSLO https://raw.githubusercontent.com/SlouchCat/slouchcat/v1.1.0/deploy/compose.nvidia.yaml
echo 'COMPOSE_FILE=compose.yaml:compose.nvidia.yaml' >> .env
docker compose up -d
```

- **NVIDIA:** `compose.nvidia.yaml`. Auf dem Rechner muss das NVIDIA Container Toolkit installiert sein.
- **Intel:** `compose.intel.yaml`. Dazu die Nummer der Gruppe `render` in `.env` eintragen:
  `getent group render | cut -d: -f3` ergibt zum Beispiel `992` → `RENDER_GID=992`.
- **AMD:** `compose.amd.yaml`. Dieser Weg ist gebaut, aber im Projekt nie auf echter Hardware geprüft
  worden; er gilt als «nach bestem Wissen, ungetestet».

Ob der Chip erkannt wurde, siehst du in der Verwaltung unter «Übersicht» und im Protokoll
(`transcode backend detected`).

## 5. Einrichten

Beim ersten Aufruf führt dich ein Assistent durch vier Schritte:

1. **Konto** — deine E-Mail-Adresse und ein Passwort. Dieses Konto ist der Hauptbenutzer und darf alles.
2. **Bibliothek** — ein Ordner und sein Typ (Filme, Serien, Hörbücher, Podcasts). Der Pfad ist der
   Pfad **im Container**, also `/media/Filme`, nicht `/mnt/nas/medien/Filme`.
3. **Metadaten von TMDB** — der Lesetoken. Der Assistent prüft ihn sofort. Du darfst ihn weglassen,
   solltest es aber nicht: ohne bekannte Altersfreigabe ist jeder Titel für Profile unter 18 gesperrt.
4. **Grafikchip und Dateisystem** — nur eine Anzeige dessen, was der Server vorgefunden hat.

Danach beginnt der erste Durchlauf. Bei einer grossen Sammlung dauert er Stunden; die Übersicht zeigt
den Fortschritt, und du kannst ihn jederzeit unterbrechen. Titel erscheinen, sobald sie eingelesen
sind — du musst nicht auf das Ende warten.

Profile, Altersfreigaben, Gäste und Einladungen legst du danach in der Verwaltung an.

## 6. Von aussen erreichbar machen (HTTPS)

Im Heimnetz reicht `http://…:8096`. Von unterwegs brauchst du HTTPS: Browser verlangen es für die
Installation als App und für die Offline-Wiedergabe.

Slouchcat bringt dafür Caddy mit. Du brauchst einen Namen (eine Domain oder einen DynDNS-Namen), der
auf deinen Anschluss zeigt, und im Router die Weiterleitung der Ports 80/tcp, 443/tcp und 443/udp auf
den Rechner.

```bash
curl -fsSLO https://raw.githubusercontent.com/SlouchCat/slouchcat/v1.1.0/deploy/Caddyfile
curl -fsSLO https://raw.githubusercontent.com/SlouchCat/slouchcat/v1.1.0/deploy/compose.https-only.yaml
```

In `.env`:

```bash
SLOUCHCAT_DOMAIN=media.beispiel.ch
ACME_EMAIL=du@beispiel.ch
SLOUCHCAT_TRUSTED_PROXIES=172.18.0.0/16
SLOUCHCAT_PUBLIC_URL=https://media.beispiel.ch
COMPOSE_FILE=compose.yaml:compose.https-only.yaml
```

`SLOUCHCAT_TRUSTED_PROXIES` sagt dem Server, wessen Angabe «von welchem Gerät kommt diese Anfrage»
er glauben darf. Trag das Docker-Netz von Caddy ein — `docker network inspect slouchcat_default`
zeigt es, meistens `172.18.0.0/16`. Der Wert `private` tut es auch, vertraut aber jedem Gerät in
deinem Heimnetz; der Server warnt dann beim Start.

Dann:

```bash
docker compose --profile https up -d
docker compose --profile https logs -f caddy
```

Das Zertifikat holt Caddy selbst; im Protokoll steht `certificate obtained successfully`. Danach ist
die Seite unter `https://media.beispiel.ch` erreichbar.

`compose.https-only.yaml` nimmt dem Rechner den offenen Port 8096 weg, sodass alles über Caddy läuft.
Das gehört dazu: `SLOUCHCAT_TRUSTED_PROXIES=private` vertraut jedem Absender aus dem Heimnetz, und
solange 8096 offen bliebe, könnte ein Gerät im Heimnetz sich damit eine fremde Adresse zuschreiben.

Das Volume `caddy_data` nicht löschen — darin liegen die Zertifikate. `docker compose down -v` löscht es.

## 7. Sichern und wiederherstellen

Die Verwaltung hat unter «Sicherung» einen Knopf. Das Archiv enthält die Datenbank und die
Einstellungen, **keine Mediendatei**. Zwei Fassungen:

- **Mit Schlüssel** (eine Passphrase, die du dir merkst): enthält auch die verschlüsselten
  Einstellungen, Push-Abos, Telegram-Verknüpfungen und Sitzungen.
- **Ohne Schlüssel**: lässt diese Dinge weg; nach einer Wiederherstellung trägst du die Zugangsdaten
  neu ein. Private Podcast-Adressen bleiben im Archiv, weil ein Abo ohne seine Adresse wertlos wäre;
  der Dialog sagt es.

Die Archive liegen unter `config/backups`. Kopiere sie regelmässig irgendwo anders hin — ein Archiv
auf derselben Platte hilft nicht, wenn die Platte ausfällt.

Vor jeder Datenbank-Änderung durch eine neue Fassung sichert der Server die Datenbank von selbst und
behält die letzten fünf Stände.

## 8. Aktualisieren

Slouchcat prüft einmal täglich, ob es eine neuere Fassung gibt, und sagt es in der Verwaltung. Es
installiert nichts von selbst. Aktualisieren heisst:

```bash
cd ~/slouchcat
# SLOUCHCAT_VERSION in .env auf die neue Fassung setzen
docker compose pull
docker compose up -d
```

Beim ersten Start der neuen Fassung sichert der Server die Datenbank und wendet die Änderungen an.
Die Prüfung lässt sich in der Verwaltung abschalten; dann fragt der Server nirgends mehr nach.

## 9. Was nicht geprüft ist

Zu einer ehrlichen Anleitung gehört, was in dieser Fassung **nicht** nachgewiesen wurde:

- **iPhone, iPad und Mac.** Im Projekt stand kein Apple-Gerät zur Verfügung. Die Weboberfläche ist für
  Safari gebaut, aber nie darauf gelaufen. Besonders die Offline-Wiedergabe auf dem iPhone ist offen.
- **AMD-Grafikchips (VAAPI).** Gebaut, nie auf echter Hardware gemessen.
- **Dauerhaftigkeit der Downloads über 30 Tage.** Geprüft sind drei Tage am Rechner; länger wurde nicht
  beobachtet, und auf Android wurde es nicht durchgeführt.

Alles Übrige ist mit Tests oder Messwerten belegt; der vollständige Nachweis steht im Abnahmebericht
(`docs/dev/ABNAHME-1.0.md`).

## 10. Wenn etwas klemmt

| Beobachtung | Woran es meistens liegt |
|---|---|
| Container wird nicht `healthy` | `docker compose logs slouchcat`. Häufig: `config/` gehört `root` statt dir, oder `config/` liegt auf einem Netzlaufwerk |
| «Kein Grafikchip erkannt» | Die GPU-Datei fehlt im `COMPOSE_FILE`, oder bei Intel/AMD fehlt `RENDER_GID` |
| Neue Dateien erscheinen nicht | Liegen die Medien auf einer SMB-Freigabe? Dann hält der Rechner das Verzeichnis zwischengespeichert; `deploy/README.md`, Abschnitt «Medien auf SMB-Freigaben», zeigt die Abhilfe. Mit dem Webhook von Radarr oder Sonarr (Verwaltung → Integrationen) entfällt das Warten ganz |
| Titel ohne Poster und Beschreibung | TMDB-Schlüssel fehlt oder ist ungültig; Verwaltung → Integrationen zeigt es |
| Wiedergabe startet nicht | Verwaltung → Streams zeigt, was läuft; Verwaltung → Übersicht zeigt die letzten Fehler |
| Passwort vergessen | Es gibt keinen Zurücksetz-Weg von aussen. Die Sicherung zurückspielen oder die Datenbank in `config/` neu anlegen lassen (dann ist die Sammlung neu einzulesen) |

Fehler meldest du als Issue unter <https://github.com/SlouchCat/slouchcat/issues>. Hilfreich sind die
Fassung (Verwaltung → Übersicht), die letzten Zeilen aus `docker compose logs slouchcat` und was du
getan hast.

## Lizenz

Slouchcat steht unter der GNU Affero General Public License, Version 3. Das Image enthält ffmpeg aus
dem jellyfin-ffmpeg-Build unter der GPL; der Lizenzordner `/licenses` im Image und die Datei
`FFMPEG-NOTICE.md` jedes Releases nennen die Quelle und das schriftliche Quellcode-Angebot.
