# Idee 05: Vorschau beim Spulen (Trickplay)

- **Status: Ausgearbeitet** (4. November 2026). Nicht beauftragt.
- **Herkunft:** Konzept 13 «Abspielseite», Vorschlag V-21 (`docs/planning/13-abspielseite.md`,
  Abschnitt 6.3). Marco hat auf F-7 am 1. November 2026 «**später**» geantwortet (Frage 19).
  V-21 entfällt darum in Konzept 13 und steht hier, damit er später ohne neue Grundsatzdiskussion
  geplant werden kann.

## 1. Problem und Ziel

Wer auf der Zeitleiste spult, sieht nur die Zeit und, ab Konzept 13, den Namen des Kapitels und
das Kapitelbild (Stufe 3). Er sieht nicht, welche Szene an der Stelle unter dem Zeiger kommt.
Emby, Jellyfin und Streamingdienste zeigen dort ein kleines Vorschaubild.

**Ziel:** Beim Zeigen und Ziehen auf der Zeitleiste ein Vorschaubild der Stelle, im Abstand von
höchstens 10 s.

## 2. Zielgruppe und Nutzen

Alle Profile, die Filme und Serien schauen. Der Nutzen ist gross bei langen Filmen ohne gute
Kapitel und beim Suchen einer bestimmten Szene.

## 3. Abgrenzung zur Spec v1

- Berührt kein Nicht-Ziel.
- Berührte Anforderungen: W-1 (Wiedergabe im Browser), C-4 (Fernbedienung).
- Ergänzt die Kapitelbilder aus Konzept 13 Stufe 3 (V-19). Ersetzt sie nicht.

## 4. Nutzerfluss

1. Das Profil zeigt oder zieht auf der Zeitleiste (Maus, Finger, ←/→ bei Fokus auf der Zeitleiste).
2. Über dem Zeiger erscheint ein Bild der Stelle, darunter Zeit und Kapitelname.
3. Gibt es für die Datei noch keine Vorschau, erscheint wie heute nur das Kapitelbild bzw. die
   Zeit.

## 5. Datenmodell und API-Skizze

- **Ein Kachelbild je Datei:** ein Bild alle 10 s, 320 px breit, in Kacheln zu einem Raster
  gefügt (z. B. 10 × 10 je Blatt), WebP. Geschätzt 3 bis 6 MB je Film.
- **Ablage** im Bild-Cache wie Poster und Kapitelbilder:
  `cache/trickplay/<mediaFileId>-<probedAt>/<blatt>.webp`. Ein neuer Probe ergibt einen neuen
  Ordner, der alte wird gelöscht.
- **Jobart** `trickplay.generate` je Mediendatei, nach dem Muster von `chapters.images` aus
  Konzept 13.
- **Ausliefern:** `GET /items/{id}/trickplay/{blatt}?mediaFileId=` als `http.Handler` (Regel 6),
  `x-visibility: scoped`, aufgelöst über `vis.MediaFile`. Dazu ein Feld in `PlaybackInfo` mit
  Rasterangaben (Abstand, Kachelgrösse, Anzahl).

## 6. Berührungspunkte mit den harten Regeln

- **Regel 3:** ffmpeg nur über `transcode` (Methode am `Runner`, Muster `GrabFrame`).
- **Regel 2:** Bilder eines unsichtbaren Titels und einer fremden Datei antworten 404.
- **Regel 6:** Bildendpunkt als `http.Handler` mit Beschreibung im Vertrag.
- **Regel 9:** Texte und Farben der Vorschau aus Locales und Tokens.

## 7. Offene Fragen

- **Wann erzeugen?** Das ist der Grund für «später». Ein Durchlauf liest die **ganze Datei**,
  wie `probe.keyframes`, das die Bibliothek auf CT 190 in rund 11 Tagen einmal durchläuft. Der
  Vorschlag von F-7 war «nur auf Abruf» für Titel, die abgespielt werden, nach dem Ende der
  Wiedergabe, oder gekoppelt an den Keyframe-Durchlauf (ein Lesen für beides).
- **HDR und Dolby Vision:** Tonemapping auf der CPU bei 320 px ist billig, aber für jeden
  zehnten Sekundenpunkt nötig.
- **Offline:** Lädt ein Download die Vorschau mit (3 bis 6 MB), oder nicht?
- **Speicherbudget** des Bild-Caches bei vollständigem Durchlauf.

## 8. Grobe Aufwandsschätzung und Abhängigkeiten

- Rund 3 Tage, nach Konzept 13 Stufe 3. Wiederverwendet werden `Runner.GrabFrame`, die Ablage und
  das Aufräumen der Kapitelbilder sowie das Vorschaufeld der `SeekBar`.
- Dazu eine Messung der NAS-Last auf CT 190 vor dem Bau.
