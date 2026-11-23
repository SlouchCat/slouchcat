# Grafiken für den Store-Eintrag

Was die Play Console an Bildern verlangt, mit Massen, und was darauf zu sehen sein soll. Die
Masse stehen auf Googles Hilfeseite «Add preview assets»
(<https://support.google.com/googleplay/android-developer/answer/9866151>, gelesen am
21. November 2026), ausser wo «Quelle» etwas anderes nennt.

**Woher die Bilder kommen:** Die Bildschirmfotos entstehen aus den Emulatoren (`sc_phone`,
`sc_tablet`, `sc_tv`) gegen den Vorführserver aus `deploy/demo/` mit den offenen Filmen der Blender
Foundation (Lizenzen in `deploy/demo/README.md`). Nie Bilder aus der eigenen Sammlung, nie Plakate
geschützter Filme; Filmbilder kommen nie ins öffentliche Repo. Plakate und Beschreibungen der vier Filme
kommen mit einem TMDB-Schlüssel am Vorführserver; ob TMDB-Bilder in Store-Fotos erlaubt sind, ist
unbelegt. Sicherer: ohne TMDB-Schlüssel und mit den Vorschaubildern, die der Server aus den Filmen
selbst zieht, oder die Plakate nicht im Bild. Profilnamen und Konten auf den Fotos: nur die des
Vorführservers («Demo»). «Big Buck Bunny» nicht in den Vordergrund stellen: Ein Trickfilm mit
Tieren lässt den Eintrag wirken, als richte er sich an Kinder (`datensicherheit.md`, Abschnitt 4);
besser «Tears of Steel» und «Sintel».

Die Bilder liegen nicht im Repo. Ablage: `<Ordner für die Store-Grafiken>`.

**App-Symbol, Feature-Grafik und TV-Banner** entstehen aus den SVG-Quellen in `docs/store/grafiken/`
(`app-icon.svg`, `feature-graphic.svg`, `tv-banner.svg`; Zeichen und Name aus dem Banner der TV-App,
Farben aus den Tokens):

```bash
scripts/store-graphics.sh <Ordner für die Store-Grafiken>
```

Das Skript schreibt `app-icon.png` (512 × 512, RGBA), `feature-graphic.png` (1024 × 500, RGB) und
`tv-banner.png` (1280 × 720, RGB), prüft Mass und Farbtyp jeder Datei und verweigert einen Zielordner
im Repo. Es braucht den Workspace (`pnpm install`) und das Chromium von Playwright
(`pnpm --dir web exec playwright install chromium`), wie die e2e-Tests. Die Feature-Grafik zeigt nur
Zeichen und Name, kein Handy (Entscheid beim Bau: keine Bildschirmfotos im Repo, kein Geräte-Foto).

## Pflicht und Mass

| Grafik | Pflicht | Mass | Format | Quelle |
|---|---|---|---|---|
| App-Symbol | ja | 512 × 512 px | PNG 32 Bit mit Alphakanal, höchstens 1024 KB | Hilfeseite |
| Feature-Grafik | ja | 1024 × 500 px | JPEG oder PNG 24 Bit ohne Alphakanal | Hilfeseite |
| Bildschirmfotos Handy | ja, 2 bis 8 | Seiten 320 bis 3840 px, lange Seite höchstens 2,3-mal die kurze; für Empfehlungen im Store mindestens 4 Fotos mit mindestens 1080 px (Hochformat 1080 × 1920, Querformat 1920 × 1080) | JPEG oder PNG 24 Bit ohne Alphakanal, höchstens 8 MB je Bild | Hilfeseite |
| Bildschirmfotos 7-Zoll-Tablet | für die Anzeige auf Tablets: mindestens 4 | Seiten 1080 bis 7680 px, 16:9 oder 9:16 | wie Handy | Hilfeseite |
| Bildschirmfotos 10-Zoll-Tablet | wie 7 Zoll: mindestens 4 | Seiten 1080 bis 7680 px, 16:9 oder 9:16 | wie Handy | Hilfeseite |
| TV-Banner im Store | ja für die TV-Geräteart | 1280 × 720 px | JPEG oder PNG 24 Bit ohne Alphakanal | Hilfeseite |
| TV-Banner in der App | ja (TV-Qualitätsliste) | 320 × 180 dp, als `android:banner` im Manifest der TV-App, mit App-Namen | Bild in den Ressourcen der App | K26 2.6, TV-Qualitätsliste |
| Bildschirmfotos TV | ja, mindestens 1 | Mass nennt die Hilfeseite nicht; Vorschlag 1920 × 1080 px (16:9) | wie Handy | Hilfeseite (Anzahl), Mass unbelegt |
| Video (YouTube) | nein | – | – | weggelassen |

Ob Tablet-Fotos Pflicht sind oder die App ohne sie nur weniger sichtbar ist, sagt die Hilfeseite
nicht eindeutig. Vorschlag: liefern, weil die App Tablets und Querformat können muss (K27 3.2).

## Was darauf zu sehen sein soll

**App-Symbol:** das Zeichen der App aus `web/public/icon.svg` (dieselbe Form wie das PWA-Symbol),
auf 512 × 512 gerendert, auf dem dunklen Hintergrund der Tokens. Kein Text, keine Rahmen oder
Schatten (Google legt Maske und Schatten selbst darüber).

**Feature-Grafik:** nur Zeichen und Name «Slouchcat», mittig auf dem dunklen Hintergrund der Tokens
(`docs/store/grafiken/feature-graphic.svg`). Kein Handy, kein Bildschirmfoto, kein Slogan.

**Bildschirmfotos Handy** (Hochformat 1080 × 1920 oder 1080 × 2400 aus `sc_phone`, je Sprache ein
Satz; die Oberfläche in der Sprache des Eintrags):

1. Startseite mit Spotlight und Reihen (vier Filme)
2. Detailseite «Tears of Steel» mit Tonspur und Untertiteln
3. Videoplayer im Querformat beim Spulen mit Vorschaubild (Querformat-Foto 1920 × 1080)
4. Seite «Downloads» mit einem gespeicherten Film und dem belegten Platz
5. Profilwahl mit zwei Profilen, eines mit PIN-Schloss
6. Tonplayer eines Hörbuchs (setzt ein frei lizenziertes Hörbuch am Vorführserver voraus, siehe
   `deploy/demo/README.md`, «Nicht dabei»; sonst weglassen)
7. «Fernseher verbinden» mit der Rückfrage vor dem Koppeln

**Bildschirmfotos Tablet** (aus `sc_tablet`, Querformat 1920 × 1080 für 7 Zoll und 2560 × 1440 für
10 Zoll, beides 16:9): Startseite mit Navigation an der Seite, Bibliothek als Raster,
Detailseite, Videoplayer. Je vier.

**TV-Banner (Store, 1280 × 720) und App-Banner (320 × 180 dp):** Zeichen und Name «Slouchcat»
mittig auf dunklem Grund, sonst nichts. Der Name muss auf dem Banner lesbar sein (TV-Qualitätsliste).

**Bildschirmfotos TV** (aus `sc_tv`, 1920 × 1080): Startseite mit Fokus auf einem Film,
Detailseite, Videoplayer mit Tonmenü, Einrichten-Seite mit QR-Code. Der QR-Code auf dem Foto muss
ein abgelaufener Treffpunkt sein, keiner, der noch gilt.

## Bildschirmfotos aufnehmen

```bash
scripts/store-screenshots.sh <Ordner für die Store-Grafiken>
```

Das Skript lädt die vier Filme einmal über `deploy/demo/prepare-media.sh` nach
`~/.cache/slouchcat-demo` (rund 1,6 GB, mit Prüfsumme; `MEDIA_CACHE` ändert den Ort), startet je
Sprache einen Server dieses Repos mit frischen Daten auf dem Loopback (`PORT`, ab Werk 8897 für
Deutsch und 8899 für Englisch) und richtet ihn über die API ein: Konto `demo@example.com`,
Hauptprofil «Demo» mit PIN, ein zweites Profil («Gast», «Guest»), die Bibliothek der Filme mit
Vorschau beim Spulen «Im Voraus» und eine Hörbuch-Bibliothek. Dann fährt es die Emulatoren
nacheinander (`sc_phone`, `sc_tablet`, `sc_tv`; ein Emulator zugleich über die Sperre aus
`scripts/android-helpers.sh`), je Sprache mit gelöschten App-Daten und der Testklasse
`StoreShotsTest` der App, und legt die Bilder nach `<Ordner>/{phone,tablet7,tablet10,tv}-{de,en}/`.
Zum Schluss wandelt es jedes Bild in PNG mit 24 Bit ohne Alphakanal und prüft Mass, Format und
Seitenverhältnis mit `scripts/store-screenshots-check.py`. Einen Ordner im Repo verweigert es.
`DEVICES` (ab Werk `phone tablet tv`) und `LANGS` (`de en`) schränken einen Lauf ein.

- **Ohne TMDB:** Plakat und Hintergrund jedes Films sind Standbilder aus dem Film selbst, die das
  Skript mit ffmpeg zieht. Ohne TMDB-Schlüssel ordnet der Server einen Film keiner Quelle zu und
  übernimmt dann weder NFO-Texte noch Bilder neben dem Film; das Skript setzt darum Inhaltsangabe
  (eigene Worte, je Sprache) und Genres über die Verwaltung (`updateItemMetadata`) und lädt Plakat
  und Hintergrund hoch (`uploadItemImage`), wie es eine Verwaltung ohne Schlüssel von Hand täte.
- **Vorschau beim Spulen:** «Im Voraus» erzeugt Vorschaubilder nur für Dateien, die nach dem ersten
  Scan dazukommen. Das Skript startet darum einmal `trickplay.backfill` (`runJobType`).
- **Server:** mit `SLOUCHCAT_HW=software` wie der Vorführserver auf einem kleinen Mietserver (auf
  dem Entwicklungsrechner scheiterte die GPU-Erkennung beim Vorbereiten eines Downloads).
- **Hörbuch:** Es gibt am Vorführserver kein frei lizenziertes Hörbuch. Das Motiv zeigt das
  erfundene Testhörbuch «The Quiet Harbour» von «Mira Lindqvist» aus
  `scripts/android-test-video.sh` (Sinustöne) mit einem Umschlag, den das Skript zeichnet.
- **Tablets:** Die AVD `sc_tablet` hat 2560 × 1600 (16:10). Der Store will 16:9; das Skript stellt
  die Anzeige des Emulators mit `wm size` auf 1920 × 1080 (Dichte 280, 7 Zoll) und 2560 × 1440
  (Dichte 320, 10 Zoll). Die App zeichnet sich in dieser Grösse; es wird nichts abgeschnitten.
- **Handy-Player:** Für das Querformat-Foto stellt das Skript die Anzeige auf 1080 × 1920, das
  Foto hat dann 1920 × 1080. Die anderen Handy-Fotos haben das Mass der AVD (1080 × 2424).
- **Statusleiste:** im Demo-Modus der Systemoberfläche (12:00, volles WLAN und voller Akku, keine
  Mitteilungen).
- **QR-Code:** Der Treffpunkt liegt bei der lokalen Vermittlung `relay/bin/pair` des Laufs; sie
  endet mit dem Lauf, der Code auf dem Foto führt danach nirgends hin.
- Die Testklassen `StoreShotsTest` laufen nur mit dem Argument `slouchcatStore`; in den übrigen
  Emulator-Läufen enden sie an ihrer Annahme (`assumeTrue`).

### Was vorliegt

Stand des Laufs vom 23. November 2026, je Sprache (`de` in der Schweizer Schreibweise der App,
`de-CH`, und `en`) ein Satz. Dateien unter `<Ordner für die Store-Grafiken>/<Satz>-<Sprache>/`,
PNG mit 24 Bit ohne Alphakanal.

| Satz | Motiv | Datei | Mass |
|---|---|---|---|
| `phone` | Startseite mit Reihen | `01-home.png` | 1080 × 2424 |
| `phone` | Detailseite «Tears of Steel», «Datei und Technik» offen | `02-detail.png` | 1080 × 2424 |
| `phone` | Videoplayer im Querformat beim Spulen mit Vorschaubild | `03-player.png` | 1920 × 1080 |
| `phone` | «Downloads» mit «Sintel» (Qualität «Niedrig») und belegtem Platz | `04-downloads.png` | 1080 × 2424 |
| `phone` | Profilwahl: «Demo» mit PIN-Schloss und ein zweites Profil | `05-profiles.png` | 1080 × 2424 |
| `phone` | Tonplayer mit dem erfundenen Testhörbuch | `06-audiobook.png` | 1080 × 2424 |
| `phone` | «Fernseher verbinden», die Rückfrage vor dem Koppeln | `07-tv-link.png` | 1080 × 2424 |
| `tablet7`, `tablet10` | Startseite mit Navigation an der Seite | `01-home.png` | 1920 × 1080, 2560 × 1440 |
| `tablet7`, `tablet10` | Bibliothek «Filme» als Raster | `02-library.png` | 1920 × 1080, 2560 × 1440 |
| `tablet7`, `tablet10` | Detailseite «Tears of Steel» | `03-detail.png` | 1920 × 1080, 2560 × 1440 |
| `tablet7`, `tablet10` | Videoplayer beim Spulen | `04-player.png` | 1920 × 1080, 2560 × 1440 |
| `tv` | Startseite, Fokus auf «Tears of Steel» | `01-home.png` | 1920 × 1080 |
| `tv` | Detailseite «Tears of Steel» | `02-detail.png` | 1920 × 1080 |
| `tv` | Videoplayer mit offenem Tonmenü | `03-player-audio.png` | 1920 × 1080 |
| `tv` | Einrichten mit QR-Code (Treffpunkt abgelaufen) | `04-setup-qr.png` | 1920 × 1080 |

Was abweicht oder fehlt:

- **Untertitel auf der Detailseite (Handy):** Die Detailseite der Handy-App nennt unter «Datei und
  Technik» Video, Auflösung, Ton, Grösse und Behälter, aber keine Untertitel. Das Foto zeigt darum
  die Tonspur; die Untertitel (Deutsch, Englisch) wählt man im Player.
- **Hörbuch:** erfundener Testinhalt (siehe oben), kein echtes Hörbuch. Seine Teile heissen auch
  im englischen Satz «Teil 1» und «Teil 2» (so benennt sie `scripts/android-test-video.sh`). Es
  steht als zuletzt hinzugefügter Titel in der Reihe «Neu hinzugefügt» vor den Filmen.
- **Startseite:** Der Vorführserver hat keine Spotlight-Liste (sie kommt von einem Anbieter
  ausserhalb, den der Lauf nicht anbindet); die Startseite zeigt «Weiterschauen» (ausser auf dem ersten Handy-Foto), «Neu hinzugefügt» und
  «Genres».
- **HTTP-Hinweis:** Der Vorführserver des Laufs spricht HTTP auf `10.0.2.2`. Darum zeigt die
  Rückfrage «Fernseher verbinden» den Hinweis, dass die Verbindung nicht verschlüsselt ist, und die
  Adresse `http://10.0.2.2:8897`; die Profilwahl zeigt `10.0.2.2` in der Fusszeile, die TV-Seite
  mit dem QR-Code «Eigene Vermittlung: 10.0.2.2». Mit einem Vorführserver hinter HTTPS und der
  öffentlichen Vermittlung fielen Hinweis und Zeile weg.
- **Uhr der TV-App:** Die TV-Oberfläche zeigt die Uhrzeit des Emulators (kein Demo-Modus auf dem
  Fernseher).
- **Tablets:** gerendert in 16:9 durch `wm size`, die AVD selbst ist 16:10 (siehe oben); der
  7-Zoll-Satz ist dieselbe Oberfläche mit kleinerer Dichte, kein eigenes Gerät.

## Noch einzutragen

| Platzhalter | Was |
|---|---|
| `<Ordner für die Store-Grafiken>` | wo die fertigen Bilder ausserhalb des Repos liegen |
