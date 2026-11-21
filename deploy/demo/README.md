# Vorführserver für Googles Prüfung

Dieser Ordner setzt einen Slouchcat-Server auf, an dem sich Googles Prüfer mit der Handy-App und
später mit der TV-App anmelden können (Konzept 27, Frage F-10; Konzept 26, 8.2). **Der Server ist
nur für die Prüfung da.** Er hält vier offene Filme der Blender Foundation, ein Hauptkonto und ein
Prüfkonto, sonst nichts: keine eigenen Medien, keine echten Konten, keine Anbindung an Seerr,
Radarr oder Sonarr.

| Datei | Zweck |
|---|---|
| `compose.yaml` | vier Dienste: `media` (lädt die Filme), `slouchcat` (Server), `seed` (Ersteinrichtung und Prüfkonto), `caddy` (HTTPS, nur mit dem Profil `https`) |
| `.env.example` | Vorlage für `.env` |
| `films.tsv` | Filme mit Adresse und SHA-256 |
| `prepare-media.sh` | lädt und prüft die Filme, läuft im Dienst `media` |
| `seed.sh` | Ersteinrichtung über die API, läuft im Dienst `seed` |
| `Caddyfile` | TLS-Abschluss für das Profil `https`, wie `deploy/Caddyfile` |

Im Repo liegt keine Mediendatei. Die Filme kommen beim ersten Start von `download.blender.org`.

## Filme

Alle vier sind Open Movies der Blender Foundation. Die Lizenzen stehen auf den Seiten der
Projekte (geprüft am 21. November 2026). Die Lizenz verlangt die Namensnennung. Die Filme tragen
ihren Abspann selbst, `prepare-media.sh` schreibt die Nennung zusätzlich nach
`media/CREDITS.txt`.

| Titel | Lizenz | Nennung | Lizenz belegt auf | Datei (Quelle) | Grösse |
|---|---|---|---|---|---|
| Elephants Dream (2006) | CC BY 2.5 | (c) copyright 2006, Blender Foundation / Netherlands Media Art Institute / www.elephantsdream.org | <https://orange.blender.org/> | <https://download.blender.org/ED/elephantsdream-720-h264-st-aac.mov> | 152 MB |
| Big Buck Bunny (2008) | CC BY 3.0 | (c) copyright 2008, Blender Foundation / www.bigbuckbunny.org | <https://peach.blender.org/about/> | <https://download.blender.org/peach/bigbuckbunny_movies/big_buck_bunny_720p_h264.mov.zip> | 417 MB |
| Sintel (2010) | CC BY 3.0 | (c) copyright Blender Foundation \| durian.blender.org | <https://durian.blender.org/about/> | <https://download.blender.org/durian/movies/Sintel.2010.720p.mkv.zip> | 681 MB |
| Tears of Steel (2012) | CC BY 3.0 | (CC) Blender Foundation \| mango.blender.org | <https://mango.blender.org/about/> | <https://download.blender.org/demo/movies/ToS/tears_of_steel_720p.mov>, dazu die Untertitel `TOS-en.srt` und `TOS-de.srt` aus `…/ToS/subtitles/` | 372 MB |

Zusammen rund 1,6 GB. Die Prüfsummen hat der Bau am 21. November 2026 aus den Downloads
genommen; ändert Blender eine Datei, lehnt `prepare-media.sh` sie ab und endet mit Fehler. Dann
die neue Prüfsumme in `films.tsv` eintragen, nachdem die Lizenz auf der Projektseite noch stimmt.

Die vier Filme bringen zwei Behälter (MOV mit H.264, MKV) und Untertitel aus Begleitdateien (Tears
of Steel, Deutsch und Englisch). Ob ein Film direkt, als Remux oder umgerechnet läuft, entscheidet
der Server je Gerät; geprüft ist das mit diesen Dateien noch nicht (offen bis zur ersten Probe mit
der App). Die Lizenz der Musik von Tears of Steel als eigene Dateien ist CC BY-ND 3.0 (`copyright.txt` im
Download-Ordner); das betrifft den Film nicht, der hier nur als Ganzes läuft.

**Nicht dabei:** Serien, Hörbücher, Podcasts und Webradio. Die Seiten zeigen dann ihren leeren
Zustand. Wer das ändern will: Abschnitt «Ohne Download» (synthetische Serien) oder von Hand über die
Verwaltung, mit Inhalten unter freier Lizenz.

## Einrichten auf einem kleinen Mietserver

Ein kleiner Mietserver mit 2 CPU-Kernen, 2 GB Arbeitsspeicher und 10 GB Platte genügt (Schätzung,
nicht gemessen: die Filme sind 1,6 GB, der Server rechnet mit `SLOUCHCAT_HW=software` um, für
eine Wiedergabe gleichzeitig). Was du brauchst: einen Namen im DNS, der auf den Mietserver zeigt,
und offene Ports 80 und 443.

1. **Docker installieren.** Docker Engine mit dem Compose-Plugin nach der Anleitung des
   Herstellers. Geklappt hat es, wenn `docker compose version` eine Fassung zeigt.
2. **Ordner kopieren.** Diesen Ordner `deploy/demo/` auf den Mietserver kopieren, zum Beispiel
   nach `/srv/slouchcat-demo`. Mehr aus dem Repo braucht es nicht.
3. **Umgebungsdatei anlegen.** `cp .env.example .env`, dann in `.env` eintragen:
   - `DEMO_OWNER_EMAIL`, `DEMO_OWNER_PASSWORD`: das Hauptkonto, nur für dich.
   - `DEMO_REVIEW_EMAIL`, `DEMO_REVIEW_PASSWORD`: das Prüfkonto, das in die Play Console kommt.
     Eine Adresse, die es gibt, ist nicht nötig; der Server verschickt keine E-Mail.
   - die fünf Zeilen unter «HTTPS on a rented server» einkommentieren und `DEMO_DOMAIN`,
     `ACME_EMAIL` und `SLOUCHCAT_PUBLIC_URL` setzen.
   - `SLOUCHCAT_VERSION`: die Fassung des Servers, die zur App passt, die du einreichst.

   Passwörter: mindestens 10 Zeichen, ohne Leerzeichen, Anführungszeichen und Backslash. Ein
   zufälliges: `openssl rand -base64 24 | tr -d '/+='`. Danach `chmod 600 .env`.
4. **Ordner anlegen und starten.** `mkdir -p config cache media`, dann `docker compose up -d`.
   Beim ersten Mal lädt der Dienst `media` die Filme (einige Minuten), dann startet der Server,
   dann richtet `seed` ihn ein.
5. **Prüfen.** `docker compose logs seed` endet mit
   `seed: done, the review account sees 4 titles on the start page`. Von einem anderen Rechner:
   `curl -s https://<Name des Vorführservers>/api/v1/healthz` antwortet mit `"status":"ok"`.
   Dann in der App die Adresse `https://<Name des Vorführservers>` eintragen und mit dem
   Prüfkonto anmelden: Die Startseite zeigt vier Filme.
6. **In die Play Console eintragen:** Adresse, E-Mail und Passwort des Prüfkontos unter «App-Zugriff»
   (Text in `docs/store/datensicherheit.md`, Abschnitt «Zugang für Googles Prüfer»).

Ohne `SLOUCHCAT_TMDB_API_KEY` zeigen die Filme nur ihren Namen und kein Plakat. Für die Prüfung
reicht das; für Bildschirmfotos im Store einen TMDB-Schlüssel eintragen und den Server einmal mit
leerem `config/` neu aufsetzen (Abschnitt «Neu aufsetzen»), damit der Scan die Bilder holt.

## Was das Einrichten tut

`seed.sh` arbeitet nur über die API des Servers und prüft vor jedem Schritt, ob er schon getan
ist. Ein zweites `docker compose up -d` ändert darum nichts.

1. Ersteinrichtung (`runSetup`) mit dem Hauptkonto, seinem Profil «Owner» und der Bibliothek
   «Films» auf `/media/Movies`. Gibt es `media/TV`, kommt die Bibliothek «Series» dazu.
2. Warten, bis der Scan fertig ist.
3. Das Hauptkonto stellt eine Einladung aus (alle Bibliotheken, Altersgrenze 18), und das Prüfkonto
   nimmt sie an. Es bekommt ein eigenes Hauptprofil (`DEMO_REVIEW_PROFILE`, ab Werk «Demo») ohne
   PIN und keine Verwaltungsrechte. Die App gibt ohnehin keine Verwaltung frei (E-14).
4. Anmelden mit dem Prüfkonto und Zählen der Titel auf der Startseite.

Das Prüfkonto läuft nicht ab. Ein Passwort, das du in der Play Console angibst, darfst du danach
nicht ändern, ohne es dort nachzutragen.

## Lokal ausprobieren

```bash
cd deploy/demo
cp .env.example .env          # die vier DEMO_*-Werte eintragen, DEMO_BIND=127.0.0.1, DEMO_PORT=8797
mkdir -p config cache media
docker compose up -d
docker compose logs -f seed   # bis «seed: done …»
```

Der Server ist dann unter `http://127.0.0.1:8797` erreichbar, der Emulator erreicht ihn unter
`http://10.0.2.2:8797`. Wegräumen: `docker compose --profile https down -v`, dann
`rm -rf config cache media`.

Belegt am 21. November 2026 auf dem Entwicklungsrechner mit `SLOUCHCAT_VERSION=1.41.3`: Download
und Prüfung der sechs Dateien, Server gesund nach 45 s, `seed` in 3 s, das Prüfkonto sieht vier
Filme, Tears of Steel hat zwei Untertitel aus Begleitdateien (`de`, `en`), ein Aufruf der
Verwaltung mit dem Prüfkonto bekommt 403. Ein zweiter Lauf von `seed` meldet «review account
exists».

## Ohne Download

Statt der Filme lassen sich synthetische Testmedien nehmen, zum Beispiel wenn
`download.blender.org` nicht erreichbar ist. Sie sind Testbilder mit Ton, für die Prüfung brauchbar,
für Bildschirmfotos nicht.

```bash
OUT_DIR="$PWD/deploy/demo/media" testdata/generate-media.sh   # aus der Wurzel des Repos; braucht ffmpeg und python3
```

Dann in `.env` `DEMO_SOURCE=local`. `generate-media.sh` legt `Movies/` und `TV/` an; `seed`
legt dann auch die Bibliothek «Series» an.

## Neu aufsetzen

`docker compose down`, dann `rm -rf config cache` und `docker compose up -d`. Die Filme in
`media/` bleiben und werden nicht neu geladen.

## Betrieb

- Der Server bekommt Updates wie jeder andere: `SLOUCHCAT_VERSION` in `.env` setzen, dann
  `docker compose pull && docker compose up -d`. Vor dem Einreichen einer App-Version prüfen, dass
  die Fassung passt (`docs/dev/VERTRAEGLICHKEIT.md`).
- Nach der Prüfung darf der Server laufen bleiben: Google prüft jedes Update der App erneut, und
  das Prüfkonto muss dann wieder gehen. Wer ihn abschaltet, trägt vor dem nächsten Update einen
  neuen ein.
- Was der Server protokolliert, bleibt auf dem Mietserver. Die zwei Dienste aus `relay/` braucht
  der Vorführserver nicht: Die Prüfer koppeln keinen Fernseher per Scan, sondern richten ihn von
  Hand ein (K26 8.2), und Push ist für die Prüfung nicht nötig.

## Noch einzutragen

| Platzhalter | Wo | Was |
|---|---|---|
| `<E-Mail-Adresse des Hauptkontos>`, `<Passwort des Hauptkontos>` | `.env` | Anmeldung des Hauptkontos |
| `<E-Mail-Adresse des Prüfkontos>`, `<Passwort des Prüfkontos>` | `.env`, Play Console | Anmeldung für Googles Prüfer |
| `<Name des Vorführservers>` | `.env` (`DEMO_DOMAIN`, `SLOUCHCAT_PUBLIC_URL`), DNS, Play Console | öffentlicher Name des Servers |
| `<E-Mail-Adresse für Let's Encrypt>` | `.env` (`ACME_EMAIL`) | Kontakt für das Zertifikat, freiwillig |
