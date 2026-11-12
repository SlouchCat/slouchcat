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
| Medien | Deine Filme, Serien, Hörbücher und Podcasts in Ordnern. Slouchcat liest sie **nur**; es verschiebt, benennt und löscht nichts. Einzige Ausnahme, wenn du es willst: neue Podcast-Folgen als Datei speichern (siehe «Podcast-Folgen speichern») |
| Zugangsschlüssel | Ein kostenloser Lesetoken von [themoviedb.org](https://www.themoviedb.org) (TMDB). Er ist nicht zwingend, aber ohne ihn bleiben deine Titel ohne Beschreibung, Poster und Altersfreigabe — und ohne bekannte Freigabe sind sie für alle Profile unter 18 gesperrt |

Slouchcat läuft auf Linux. Für Apple-Geräte als **Zuschauer** gilt: iPhone, iPad und Mac sind in dieser
Fassung nicht geprüft worden, weil im Projekt keine solchen Geräte vorhanden waren (siehe
«Was nicht geprüft ist» weiter unten).

## 2. Ordner anlegen und das Programm holen

```bash
mkdir -p ~/slouchcat && cd ~/slouchcat
curl -fsSLO https://raw.githubusercontent.com/SlouchCat/slouchcat/v1.31.0/deploy/compose.yaml
curl -fsSL -o .env https://raw.githubusercontent.com/SlouchCat/slouchcat/v1.31.0/deploy/.env.example
mkdir -p config cache
```

Du hast jetzt vier Dinge nebeneinander:

- `compose.yaml` — die Beschreibung des Containers. Diese Datei änderst du nicht.
- `.env` — deine Einstellungen. Diese Datei änderst du gleich.
- `config/` — Datenbank, Einstellungen und Sicherungen. **Dieser Ordner muss auf einer lokalen Platte
  liegen**, nie auf einem Netzlaufwerk; der Server prüft das beim Start und bricht sonst ab.
- `cache/` — Vorschaubilder und umgewandelte Videoabschnitte. Darf jederzeit gelöscht werden.
- Kein Ordner, aber Arbeitsspeicher: Das Webradio puffert in einem RAM-Laufwerk (tmpfs) von 564 MB,
  das `compose.yaml` unter `/run/slouchcat/radio` anlegt. Ohne es spielt Radio nur live.

## 3. `.env` ausfüllen

Öffne `.env` in einem Texteditor. Diese Werte musst du anschauen:

```bash
# Die Fassung, die laufen soll.
SLOUCHCAT_VERSION=1.31.0

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

Im Formular «Bibliothek hinzufügen» steht neben jedem Ordnerfeld der Knopf «Durchsuchen …». Er zeigt die
Unterordner von `/media`, du klickst dich zum richtigen Ordner und musst den Pfad im Container nicht
auswendig wissen. Aus demselben Grund darf eine neue Bibliothek nur unter `/media` liegen. Ein
Podcast-Ordner im Layout von Audiobookshelf (ein Unterordner je Podcast mit `metadata.json`) wird
gelesen wie ein Hörbuch-Ordner; steht in der `metadata.json` eine Feed-Adresse, abonniert Slouchcat
den Feed gleich mit. Brauchst du eine zweite
Einhängung, ergänzt du sie nicht in `compose.yaml`, sondern in einer eigenen Override-Datei, zum
Beispiel `compose.override.yaml` im selben Ordner:

```yaml
services:
  slouchcat:
    volumes:
      - /mnt/usb/medien:/media2:ro
```

Ohne `COMPOSE_FILE` und ohne `-f` liest Docker Compose `compose.override.yaml` von selbst; sonst hängst
du sie hinten an `COMPOSE_FILE` an (zum Beispiel `COMPOSE_FILE=compose.yaml:compose.override.yaml`)
bzw. an deine `-f`-Liste. Der Grund: Ein Update ersetzt die offiziellen Compose-Dateien, die du nicht
geändert hast; eine selbst geänderte bleibt liegen und bekommt dann keine Neuerungen. Die zweite
Einhängung trägst du komma-getrennt in `SLOUCHCAT_BROWSE_ROOTS` ein (zum Beispiel
`SLOUCHCAT_BROWSE_ROOTS=/media,/media2`). Bibliotheken, die es schon gibt, bleiben in jedem Fall
bestehen.

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
curl -fsSLO https://raw.githubusercontent.com/SlouchCat/slouchcat/v1.31.0/deploy/compose.nvidia.yaml
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

### Podcast-Folgen speichern (freiwillig)

Ab Werk spielt Slouchcat Podcast-Folgen aus dem Feed als Link: es holt sie beim Abspielen vom
Anbieter und speichert nichts. Soll es neue Folgen als Datei in den Ordner ihres Podcasts legen
(Verwaltung → Podcasts → «Neue Folgen» → «Speichern»), muss **genau dieser eine Ordner** schreibbar
sein. Alle anderen Medien bleiben nur lesbar. Dafür gibt es eine Zusatzdatei, die du wie die Dateien
für den Grafikchip dazunimmst:

```bash
curl -fsSLO https://raw.githubusercontent.com/SlouchCat/slouchcat/v1.31.0/deploy/compose.podcasts.yaml
```

In `.env` trägst du drei Zeilen ein (mit Grafikchip kommt `compose.podcasts.yaml` hinten an deine
bisherige `COMPOSE_FILE`-Zeile, zum Beispiel `compose.yaml:compose.nvidia.yaml:compose.podcasts.yaml`):

```bash
COMPOSE_FILE=compose.yaml:compose.podcasts.yaml
# Der Podcast-Ordner auf deinem Rechner …
PODCAST_DIR=/srv/media/Audiobookshelf/Podcast
# … und der Pfad, unter dem er in Slouchcat schon eingetragen ist.
PODCAST_TARGET=/media/Audiobookshelf/Podcast
```

Danach `docker compose up -d`. Zwei Bedingungen musst du einhalten:

- **`PODCAST_TARGET` ist genau der Pfad, unter dem der Ordner in Slouchcat eingetragen ist**
  (Verwaltung → Bibliotheken). Ein anderer Pfad wäre für Slouchcat ein neuer Ordner, und alle Dateien
  müssten neu verknüpft werden.
- **Der Ordner liegt schon unter `MEDIA_DIR`.** Docker kann im nur lesbaren Baum keinen neuen
  Einhängepunkt anlegen; fehlt der Ordner dort, startet der Container nicht.

Die Zusatzdatei und `.env` bleiben bei einem Update erhalten. Ohne sie bleibt alles nur lesbar, und
«Speichern» ist in der Verwaltung gesperrt.

**Rechte.** Slouchcat legt neue Dateien mit `0664` und neue Ordner mit `0775` an, Besitzer ist der
Benutzer aus `PUID`/`PGID`. Damit können Audiobookshelf und die Freigaben deines NAS die Folgen lesen.
Der Benutzer braucht Schreibrecht im Podcast-Ordner; liegt der Ordner auf einem NAS, muss auch die
Freigabe (NFS-Export oder SMB) Schreiben für diesen Pfad erlauben und die Einhängung auf dem Rechner
darf nicht `ro` sein. In einem unprivilegierten LXC-Container gilt die verschobene Benutzernummer.

**Die Ordnerprüfung.** Bevor Slouchcat schreibt, prüft es den Ordner: derselbe Dateisystemtyp wie beim
Eintragen und mindestens ein bekannter Unterordner vorhanden (sonst ist das NAS wohl nicht
eingebunden, und Slouchcat würde auf die lokale Platte schreiben), eine Testdatei mit hartem Link
und die Rechte. Das Band «Neue Folgen» zeigt das Ergebnis: «beschreibbar» mit dem freien Platz oder
den Grund («nur lesbar eingebunden», «Ordner ist nicht eingebunden», «Dateisystem kann keine harten
Links», «neue Dateien bekommen nicht die nötigen Rechte»). Ohne harte Links bleibt «Speichern»
gesperrt.

**Private Feeds.** Für einen Podcast, der noch keinen Ordner hat, legt Slouchcat beim ersten Download
einen an, mit einer `metadata.json` im Format von Audiobookshelf. Darin steht die Feed-Adresse, damit
ein späterer Scan das Abo wiedererkennt. Bei einem privaten Feed steckt in dieser Adresse oft dein
Zugangsschlüssel: Wer den Ordner lesen darf, sieht ihn. Audiobookshelf macht es genauso.

**Wechsel von Audiobookshelf.** Lädt bisher Audiobookshelf die Folgen, wechselst du in dieser
Reihenfolge, damit keine Folge doppelt ankommt:

1. Podcast-Ordner wie oben schreibbar einbinden und das Update einspielen. Die Einstellung steht auf
   «Nur Link»; es ändert sich nichts.
2. In Audiobookshelf je Podcast den automatischen Download abschalten.
3. In Slouchcat die Bibliothek auf «Speichern» stellen, mit dem Umfang deiner Wahl.

Folgen, die zwischen Schritt 2 und 3 erscheinen, holt «Neue + letzte N» oder der Knopf «Fehlende
nachladen» am Podcast. Laufen beide eine Weile parallel, überschreibt Slouchcat keine Datei und trägt
keine zweimal ein, kann aber eine Folge ein zweites Mal unter leicht anderem Namen speichern; darum
die Reihenfolge.

### Dateien aus Slouchcat löschen (freiwillig)

Ab Werk sind alle Medien nur lesbar eingebunden (`:ro` in `compose.yaml`), und das bleibt so. «Löschen» auf
der Detailseite eines Titels bietet dann nur «Nur aus Slouchcat entfernen»: Der Titel verschwindet aus allen
Listen, die Dateien bleiben liegen, und unter «Wartung › Entfernte Titel» holst du ihn mit Fortschritt und
Haken zurück. Die zweite Wahl, «Auch die Dateien löschen», ist grau mit dem Grund «nur lesbar eingebunden».

Willst du Dateien aus Slouchcat heraus löschen, bindest du **genau den Ordner, um den es geht,** selbst und
bewusst schreibbar ein. Nimm dafür zuerst einen **Testordner** und erst später, wenn überhaupt, einen echten.
Der Ordner muss schon unter `MEDIA_DIR` liegen, und der Pfad im Container muss genau der sein, unter dem die
Bibliothek ihn kennt (Verwaltung → Bibliotheken). Du ergänzt ihn in `compose.override.yaml` (siehe oben),
ohne `:ro`:

```yaml
services:
  slouchcat:
    volumes:
      # Nur dieser Ordner ist schreibbar; alles andere unter /media bleibt nur lesbar.
      - /srv/media/Testfilme:/media/Testfilme
```

Danach `docker compose up -d`. Im Dialog «Ordner von …» steht beim Ordner jetzt «schreibbar». Was Slouchcat
dann tut und was nie:

- **Es prüft, ohne zu schreiben.** Je Datei fragt es, ob es im Ordner dieser Datei löschen darf (Rechte des
  Benutzers aus `PUID`/`PGID`, nur lesbare Einhängung). Ist auch nur eine Datei nicht löschbar, bleibt
  «Auch die Dateien löschen» grau, und der Dialog nennt den Grund.
- **Es löscht genau das:** die Mediendateien des Titels (bei Serie, Staffel und Podcast die aller Folgen,
  dazu die Extras), die Bild- und `.nfo`-Dateien, die es selbst gelesen hat, Begleitdateien mit dem ganzen
  Dateinamen davor (`Film (2024).de.srt`), danach leere Ordner. Was es nicht kennt, bleibt liegen; der
  Ordner bleibt dann stehen.
- **Nie** eine Datei, die zu einem anderen Titel gehört, nie rekursiv, nie automatisch, nie ohne deine
  Bestätigung (bei mehr als einer Datei tippst du den Titel ab). Es gibt keinen Papierkorb: gelöscht ist
  gelöscht.
- **Danach ist der Titel endgültig weg**, samt Fortschritt, Haken und Merkliste. Lässt sich eine Datei
  nicht löschen, bleibt der Titel mit ihr, und das Ergebnis nennt sie.
- **Radarr und Sonarr erfahren davon nichts.** Überwachen sie den Titel noch, laden sie ihn womöglich neu;
  er erscheint dann als neuer Titel.

**Rechte.** Der Benutzer aus `PUID`/`PGID` braucht Schreibrecht in den Ordnern der Dateien, nicht nur im
obersten Ordner; Radarr und Sonarr legen ihre Ordner oft mit eigenem Besitzer an. Liegt der Ordner auf einem
NAS, muss auch die Freigabe (NFS-Export oder SMB) Schreiben erlauben, und die Einhängung auf dem Rechner darf
nicht `ro` sein.

**Zurück zu nur lesbar.** Zeile aus `compose.override.yaml` entfernen und `docker compose up -d`. Eine
Bibliothek zu löschen (Verwaltung → Bibliotheken → «Bibliothek löschen …») entfernt übrigens nie eine Datei,
auch nicht in einem schreibbaren Ordner.

## 5. Einrichten

Beim ersten Aufruf führt dich ein Assistent durch vier Schritte:

1. **Konto** — deine E-Mail-Adresse und ein Passwort. Dieses Konto ist der Hauptbenutzer und darf alles.
2. **Bibliothek** — die Art (Video, Hörbücher, Podcasts) und ihre Ordner. Jeder Ordner trägt seinen
   **Inhalt** (Filme oder Serien; eine Video-Bibliothek darf beide mischen) und seine
   **Metadatenquellen** (TMDB, bei Serien auch TVDB, dazu AniList für Anime; die erste Quelle
   erkennt den Titel, die weiteren füllen auf). Mit «Weiteren Ordner hinzufügen» legst du mehrere
   Ordner in einem Zug an. Der Pfad ist der Pfad **im Container**, also `/media/Filme`, nicht
   `/mnt/nas/medien/Filme`.
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
curl -fsSLO https://raw.githubusercontent.com/SlouchCat/slouchcat/v1.31.0/deploy/Caddyfile
curl -fsSLO https://raw.githubusercontent.com/SlouchCat/slouchcat/v1.31.0/deploy/compose.https-only.yaml
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
Die Prüfung lässt sich in der Verwaltung abschalten; dann fragt der Server nirgends mehr nach. Antwortet die
Download-Adresse der Releases nicht (Netzfehler, Fehler 5xx), liest die Prüfung dieselbe Datei über die
GitHub-API; erst wenn auch das scheitert, meldet die Karte einen Fehler. Das zuletzt bekannte Update
bleibt dabei stehen.

### Update per Knopf: der Updater-Begleiter (freiwillig)

Wer das Update lieber in der Verwaltung anstösst, nimmt den **Updater-Begleiter** dazu. Dann steht auf
der Karte «Server» der Knopf «Update auf … installieren», sobald eine neuere Fassung da ist. Ohne
Begleiter zeigt der Knopf die zwei Befehle von oben. Ist der Begleiter eingerichtet, antwortet aber
nicht, sagt die Karte das und zeigt dieselben Befehle.

**Warum ein eigener Container.** Ein Update holt ein neues Image und startet den Container neu. Das kann
nur, wer Docker auf dem Rechner steuert, und wer Docker steuert (`/var/run/docker.sock`), hat
Root-Rechte auf dem ganzen Rechner. Darum bekommt nicht Slouchcat selbst diesen Zugang, sondern ein
kleines eigenes Programm im Container `slouchcat-updater`:

- Es hat keinen offenen Port, keine Benutzer, keine Medien und sieht weder `config/` (Datenbank,
  Schlüssel, Sicherungen) noch `cache/`: `compose.updater.yaml` legt bei ihm leere, schreibgeschützte
  Ordner darüber. Liegt einer der beiden Ordner ausserhalb des Slouchcat-Ordners, hängt der Begleiter
  ihn gar nicht erst ein (und verdeckt ihn darum auch nicht); Docker legt dann beim Start des
  Begleiters einen leeren Ordner `config/` oder `cache/` (Eigentümer root) im Slouchcat-Ordner an.
  Slouchcat erreicht es nur im internen Docker-Netz, mit einem geheimen Token aus `updater/token`.
- Es nimmt genau einen Auftrag an: «installiere die neueste Fassung». Welche das ist, liest es selbst
  aus dem Release-Manifest; es installiert nur eine neuere Fassung und prüft vor dem Neustart den
  Fingerabdruck (Digest) des heruntergeladenen Images.
- Es ändert in `.env` nur den Wert der Zeile `SLOUCHCAT_VERSION=` (Anführungszeichen und ein
  Kommentar dahinter bleiben) und sichert die Datei vorher als `.env.bak-vor-<Fassung>` (nur für dich
  lesbar). Es löscht nie eine Sicherung und ersetzt keine: Gibt es den Namen schon (etwa von
  Hand angelegt), nimmt es `.env.bak-vor-<Fassung>-2`, `-3`, …; räume ältere von Hand auf.
- Es startet nur den Container `slouchcat` neu, mit genau den Compose-Dateien, mit denen er gestartet
  wurde; Caddy und sich selbst lässt es stehen.

Wer das nicht will, lässt den Begleiter weg; alles andere bleibt gleich.

**Einrichten.**

```bash
cd ~/slouchcat
curl -fsSLO https://raw.githubusercontent.com/SlouchCat/slouchcat/v1.31.0/deploy/compose.updater.yaml
mkdir -p updater
echo "SLOUCHCAT_DEPLOY_DIR=$(pwd)" >> .env
```

`SLOUCHCAT_DEPLOY_DIR` ist der **absolute** Pfad des Ordners mit `compose.yaml` und `.env`. Der
Begleiter hängt ihn unter genau diesem Pfad ein, damit `./config`, `./cache` und alle anderen Pfade
der Compose-Dateien bei ihm dasselbe bedeuten wie auf dem Rechner. Alle Compose-Dateien und `.env`
müssen in diesem Ordner liegen, sonst lehnt der Begleiter ab. Der Begleiter liest Variablen nur aus
den Env-Dateien, nie aus deiner Shell: Jede Variable, die eine Compose-Datei verwendet (zum Beispiel
`RENDER_GID` aus `compose.intel.yaml` oder `SLOUCHCAT_VERSION`), muss in `.env` stehen; nur mit
`export` gesetzt, fehlt sie beim Update.

Dann `compose.updater.yaml` **zu allen Dateien dazunehmen, die du bisher verwendest**, als letzte. Mit
`COMPOSE_FILE` in `.env` hängst du sie hinten an, zum Beispiel:

```bash
COMPOSE_FILE=compose.yaml:compose.nvidia.yaml:compose.updater.yaml
```

Wer die Dateien mit `-f` aufzählt, schreibt sie alle auf, zum Beispiel
`docker compose -f compose.yaml -f compose.https-only.yaml -f compose.intel.yaml -f compose.updater.yaml --profile https up -d`.
Danach `docker compose up -d`; `docker compose ps` zeigt nun auch `slouchcat-updater`. Fehlt eine Datei
in der Liste, fehlt sie auch nach dem Update (zum Beispiel der Grafikchip oder die Sperre von Port
8096 hinter Caddy).

**Was der Knopf tut.** Nach der Bestätigung arbeitet der Begleiter diese Schritte ab; die Karte zeigt,
wo er steht:

1. Manifest lesen und prüfen (nur eine neuere Fassung, mit Digest). Antwortet die Download-Adresse
   nicht, liest er es über die GitHub-API.
   Danach ersetzt er die offiziellen Compose-Dateien, die du nicht geändert hast, durch die des
   Releases; die alte Datei bleibt als `<Datei>.bak-vor-<Fassung>` liegen. Eine selbst geänderte Datei
   bleibt liegen und bekommt dann keine Neuerungen. **Eigene Anpassungen gehören darum in `.env` oder
   in eine eigene Override-Datei** (siehe Abschnitt 3), nicht in `compose.yaml` und die anderen
   offiziellen Dateien.
2. `.env` sichern und `SLOUCHCAT_VERSION` setzen.
3. Das neue Image holen (`docker compose pull slouchcat`, bei einem Fehler bis zu dreimal) und seinen Digest mit dem Manifest
   vergleichen. Weicht er ab, setzt er `.env` zurück und startet nichts neu.
4. Den Container `slouchcat` mit der neuen Fassung starten. Laufende Wiedergaben brechen dabei ab.
5. Bis zu **zehn Minuten** warten, bis der neue Server gesund ist und seine Fassung meldet. So viel
   Zeit hat auch eine lange Datenbank-Änderung. Stürzt der neue Container ab (dreimal neu gestartet
   oder endgültig beendet), wartet er nicht länger.
6. Alte Images aufräumen: Es bleiben die laufende und die vorherige Fassung (Ziel einer Rücknahme).

Während das Update läuft, lehnt der Server «Neu starten», eine Sicherung und eine Wiederherstellung ab
(und die Karte blendet «Neu starten» aus); das Update selbst startet nicht, solange eine Sicherung,
eine Wiederherstellung oder ein Neustart ansteht. Bleibt eine Installation länger als 70 Minuten
stehen und der Begleiter antwortet nicht, zeigt die Karte sie als fehlgeschlagen. So lange dauert
selbst der längste Lauf nicht (Download bis 30 Minuten, zweimal bis 10 Minuten Warten, Rücknahme).

**Wenn die neue Fassung nicht startet.**

- **Ohne Datenbank-Änderung** nimmt der Begleiter das Update selbst zurück: alte Zeile in `.env`, alte
  Fassung starten. Die Karte meldet «zurückgenommen».
- **Mit Datenbank-Änderung** nimmt er nichts zurück, denn die alte Fassung ist für die geänderte
  Datenbank nicht gebaut. Die Karte meldet das und nennt die Sicherung, die der Server vor der Änderung
  angelegt hat (`config/backups/pre-migration-…`). Zurück geht es dann nur von Hand: Server anhalten
  (`docker compose stop slouchcat`), `config/slouchcat.db` durch diese Sicherung ersetzen und
  `config/slouchcat.db-wal` und `config/slouchcat.db-shm` löschen, falls vorhanden, die alte Fassung
  in `.env` eintragen (sie steht in `.env.bak-vor-<Fassung>`, bei belegtem Namen mit `-2`, `-3`, …) und `docker compose up -d`.

**Wenn der Begleiter mitten im Update anhält** (Neustart des Rechners, `docker compose down`), macht er
beim nächsten Start Ordnung: Vor dem Neustart des Servers setzt er `.env` aus `.env.bak-vor-<Fassung>`
zurück und meldet einen Fehler. Danach wartet er erneut bis zu zehn Minuten auf die neue Fassung
(die Karte zeigt so lange die Gesundheitsprüfung) und meldet «fertig», wenn sie gesund läuft, sonst
einen Fehler, ohne etwas zurückzunehmen; dann entscheidest du (Update von Hand wie oben oder die
Sicherung).

**Den Begleiter selbst aktualisieren.** Ab 1.18.0 folgt er dem Server von selbst: Nach einem Update
(und alle sechs Stunden) hebt er seine Fassung `SLOUCHCAT_UPDATER_VERSION` in `.env` auf die des Servers
und erstellt sich neu. Ein Begleiter **vor 1.18.0** kann das noch nicht (die Karte zeigt «Updater
veraltet» mit dem Befehl zum Kopieren); heb ihn einmal von Hand, mit der Fassung des Servers für
`X`. Der Befehl setzt die Zeile in `.env` oder hängt sie an, wenn sie fehlt:

```bash
cd <deploy-ordner> && (grep -q '^SLOUCHCAT_UPDATER_VERSION=' .env && sed -i 's/^SLOUCHCAT_UPDATER_VERSION=.*/SLOUCHCAT_UPDATER_VERSION=X/' .env || echo 'SLOUCHCAT_UPDATER_VERSION=X' >> .env) && docker compose up -d updater
```

Wer die Compose-Dateien mit `-f` aufzählt statt mit `COMPOSE_FILE`, hängt dieselbe `-f`-Liste an
(`docker compose -f compose.yaml -f compose.updater.yaml up -d updater`).

**Ohne Begleiter** bleibt es bei den zwei Befehlen von oben, nachdem du `SLOUCHCAT_VERSION` in `.env`
gesetzt hast: `docker compose pull` und `docker compose up -d`. Den Begleiter wieder entfernen:
`compose.updater.yaml` aus `COMPOSE_FILE` (oder der `-f`-Liste) streichen, dann
`docker compose up -d --remove-orphans`.

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
