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

**Feature-Grafik:** Zeichen und Name «Slouchcat» links, rechts ein angeschnittenes Handy mit der
Startseite des Vorführservers. Dunkler Hintergrund aus den Tokens. Kein Slogan, der nach Inhalten
klingt; wenn Text, dann «Deine Sammlung. Dein Server.» / «Your collection. Your server.»

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

## Noch einzutragen

| Platzhalter | Was |
|---|---|
| `<Ordner für die Store-Grafiken>` | wo die fertigen Bilder ausserhalb des Repos liegen |
