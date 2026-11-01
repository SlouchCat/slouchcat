# Konzept: Abspielseite (Videoplayer)

- **Status: Fassung 1 (1. November 2026), Vorschläge zur Wahl.** Auslöser: Marco am
  1. November 2026 mit zwei Bildschirmfotos (Slouchcat, «Two and a Half Men»; Emby, «Silo»):
  «Ich möchte die Lautstärke einstellen können, ich möchte Kapitel, bessere
  Transcoding-Einstellungen, Untertitel-Auswahl, Sprach-Auswahl etc. Mach mal ein paar
  Vorschläge.» Dieses Papier legt **zwei Varianten** für die Bedienleiste (Abschnitt 5),
  **21 Einzelvorschläge in drei Stufen** (Abschnitt 6) und einen klickbaren Prototyp mit echten
  Bildern aus der eigenen Bibliothek vor (Abschnitt 9). **Die Umsetzung beginnt erst auf Marcos
  ausdrückliches Wort.**
- **Wogegen dieses Konzept verankert ist:** `main` **`5e0e63d`** («Merge pull request #61»),
  Vertragsfassung **1.6.0**. Konzept 11 reserviert Frage 17, ADR 0019, Migration 00031 und
  Vertrag 1.7.0; Konzept 12 (Zweig `claude/konzept-verwaltungsseiten`, nicht gemergt) nimmt
  Frage 18 und ADR 0020. Dieses Konzept nimmt darum **Frage 19** und, falls nötig, **ADR 0021**;
  Migration und Vertragsfassung sind die nächsten freien nach 11 und 12 (voraussichtlich additiv
  in **1.8.0**).
- Massgeblich bleiben `docs/spec/01-anforderungen-v1.md`, `docs/planning/04-entscheidungen.md`
  und `docs/adr/`. Bei Widerspruch gilt: **Entscheidungsprotokoll > Plan > Spec** (`CLAUDE.md`).

> **Dieses Papier ändert keine Zeile Code.** Stufe 1 betrifft nur die Web-App, Stufe 2 und 3
> brauchen additive Felder im Vertrag, eine Migration und neue Jobarten (Abschnitt 7).

## 1. Ausgangslage

### 1.1 Was die Abspielseite heute kann

Der Videoplayer (`web/src/features/player/VideoPlayer.tsx`, 1046 Zeilen) zeigt oben «Zurück» und
den Titel, unten die Zeitleiste und eine Leiste mit: Abspielen/Pause, Zeit, Vollbild, Stream-Info,
«Spuren» (Ton und Untertitel in einem Menü), «Qualität», «Kapitel» und, bei mehreren Dateien,
«Version». Dazu kommen «Intro überspringen», «Abspann überspringen» und der Countdown zur nächsten
Folge (W-9). Die Tastatur kann mehr als die Maus: ↑/↓ Lautstärke, ←/→ ±10 s, T, Q, C, S, N, I, F.

### 1.2 Was fehlt oder stört

Am Bildschirmfoto vom 1. November 2026 und im Code sind diese Mängel zu sehen:

| Nr. | Mangel | Ursache im Code |
|---|---|---|
| P-1 | **Keine Lautstärke mit der Maus oder am Handy.** Kein Regler, kein Stumm-Knopf. | Lautstärke gibt es nur über ↑/↓ mit kurzer Einblendung (`changeVolume`, `VideoPlayer.tsx:322`). Der Audioplayer hat Regler und Stumm schon (`ui/AudioPlayer.tsx:48-64`). |
| P-2 | **Kein Zusammenhang im Titel.** Das Foto zeigt nur «Stur, zwanghaft und unflexibel», nicht Serie, Staffel und Folge. | `<h1>{info.item.title}</h1>` (`VideoPlayer.tsx:909`); `episodeCode` wird nur für die nächste Folge benutzt. |
| P-3 | **Keine Knöpfe zum Springen.** ±10 s, vorheriges/nächstes Kapitel und nächste Folge gehen nur über Tasten. | Die Leiste hat nur Abspielen/Pause (`VideoPlayer.tsx:922`). |
| P-4 | **Ton und Untertitel in einem langen Menü.** «Dune: Part Two» hat 19 Tonspuren und 43 Untertitel; Deutsch steht als 12. Tonspur zwischen Slowakisch und Englisch. | Ein `MenuButton` mit zwei Gruppen in Dateireihenfolge (`trackGroups`, `VideoPlayer.tsx:737`). |
| P-5 | **Keine Sprachwahl je Profil.** Es läuft die Tonspur mit dem Standard-Kennzeichen der Datei, sonst die erste; bei «Dune: Part Two» Englisch, ohne Kennzeichen wäre es Italienisch. Untertitel sind aus, ausser die Datei markiert einen als Standard. | `audioStream` (`transcode/plan.go:371`), `decideAudio` (`playback/decide.go:415`); das Profil kennt keine Sprachen. |
| P-6 | **Kapitel nur als Textliste.** «Blade Runner 2049» hat 16 Kapitel ohne Namen (`00:12:36.172`), «Dune: Part Two» nur drei («Chapter 1», «Credits», «Chapter 2»). Die Zeitleiste zeigt keine Marken. | `SeekBar.tsx` kennt keine Kapitel; das Menü zeigt `chapterLabel` (Name · Zeit). |
| P-7 | **Spulen ohne Vorschau.** Wer die Zeitleiste zieht, sieht nicht, wohin. | Keine Vorschaubilder auf dem Server. |
| P-8 | **Qualität ohne Erklärung.** Fünf feste Stufen (Automatisch, 1080p 8 Mbit/s, 720p 4, 480p 2, 360p 1). Keine Stufe «Original», keine 4K-Stufe, keine Angabe, warum umgewandelt wird. | `Quality`-Enum (`openapi.yaml:8045`), Leiter in `transcode/plan.go:124`. Der Grund steht nur in der Stream-Info (Taste I). |
| P-9 | **Kein Einfluss auf den Ton.** Kein Nachtmodus (leise Explosionen, laute Dialoge), kein Stereo-Zwang für Kopfhörer. | Ton wird nur umgewandelt, wenn der Browser den Codec nicht kann (z. B. DTS bei «Blade Runner 2049»). |
| P-10 | **Untertitel nicht einstellbar.** Grösse, Hintergrund, Farbe und Zeitversatz sind fest. | Der Browser zeichnet `<track>` mit seinen eigenen Regeln. |
| P-11 | **Keine Geschwindigkeit, kein Bild-in-Bild, keine Restzeit.** | Gibt es nur im Audioplayer (`useAudioControls.ts`). |
| P-12 | **Kleine Schrift, viel Leere.** Auf dem Foto sind Zeit und Knöpfe bei 2000 px Breite winzig, die Leiste wirkt leer. | Schriftgrad `--sc-font-size-xs` für die Zeit, nur Text-Knöpfe rechts. |

### 1.3 Was Emby besser macht (zweites Foto)

- **Logo statt Titel** oben links («SILO»), darunter unten **«S1:E4 – Wahrheit»** und der Serienname.
- **Lautstärke-Regler** oben rechts, daneben Cast, Bild-in-Bild, Vollbild.
- **Transport** unten links: vorherige Folge, −10, Pause, +30, nächste Folge.
- **Symbole rechts** über der Zeitleiste: Untertitel, Ton, Geschwindigkeit, Einstellungen.
- **Restzeit** («−42:24») und Endzeit.
- **Reiter unten**: Info, Kapitel, Besetzung, Als Nächstes (Läuft gerade und TV Guide gehören zu
  Live-TV, das ist ein Non-Goal).

Was Emby **nicht** gut macht und Slouchcat nicht übernimmt: winzige Schrift (11 px auf 2000 px),
Symbole ohne Beschriftung, Bedienelemente oben **und** unten verstreut, Reiter, die das Bild nicht
verlassen und darum über dem Film liegen.

## 2. Was gilt (Spec, Plan, Entscheide)

- **W-4** Tonspur und Untertitel lassen sich während der Wiedergabe wechseln; Text oder eingebrannt
  je nach Gerät. **W-8** Qualität manuell wählbar, Grenzen je Profil. **W-9** Intro, Abspann,
  nächste Folge, Kapitelmarken anwählbar. Alles heute erfüllt; dieses Konzept verbessert die
  Bedienung, es führt keine neue Funktion gegen die Spec ein.
- **Regel 12 (Entscheid im Server):** Welche Spur, welche Qualität und ob umgewandelt wird,
  entscheidet der Server. Der Client meldet Wünsche (Spur, Qualität, neu: Sprachvorlieben,
  Ton-Optionen) und zeigt, was der Server liefert.
- **Regel 9:** Farben nur aus Tokens, Texte nur aus `de.json`/`en.json`, Bauteile aus `web/src/ui`.
  **Regel 8:** Die Seite hat einen Playwright-Tastaturtest; jede neue Leiste und jedes neue Panel
  braucht ihn auch.
- **Non-Goals:** Live-TV (kein «Läuft gerade», kein «TV Guide»), TV-Apps. **Chromecast** braucht
  eine Empfänger-App auf dem Fernseher; das liegt nahe am Non-Goal «TV-Apps» und bleibt draussen
  (Abschnitt 6.4).

## 3. Ziel

Ein Player, der sich **wie ein Streamingdienst** bedienen lässt: mit der Maus, am Handy und mit
der Fernbedienung gleich gut. Konkret:

1. Lautstärke, Ton, Untertitel und Qualität sind **mit einem Klick** erreichbar und **erklären
   sich selbst** (Sprache in Worten, Format, Grund einer Umwandlung).
2. Der Player **weiss, was das Profil will**: deutsche Tonspur, wenn vorhanden; Untertitel nur
   erzwungen, sonst aus. Wer einmal wählt, muss es nicht bei jeder Folge wieder tun.
3. **Kapitel und Spulen sind sichtbar**: Marken auf der Zeitleiste, Bilder in der Kapitelliste,
   Vorschau beim Ziehen.
4. Der Bildschirm bleibt **ruhig**: nichts liegt über dem Bild, solange der Film läuft; alles
   blendet nach drei Sekunden aus (wie heute, `CONTROLS_HIDE_MS`).

## 4. Gestaltungsregeln (gelten für jede Variante)

- **Ein Ort pro Aufgabe.** Transport links unten, Einstellungen rechts unten, Titel links oben,
  Zurück ganz links oben. Nichts doppelt.
- **Jedes Symbol hat einen Namen.** Auf dem Desktop zeigt die Leiste Symbol plus kurzes Wort,
  wo Platz ist («Deutsch», «1080p»); sonst Symbol mit Tooltip und `aria-label`.
- **Grösser als heute.** Zeit und Knopfbeschriftung mindestens `--sc-font-size-sm`, Knöpfe
  44 × 44 px (Touch-Ziel). Die Leiste wächst mit der Breite, bis 1600 px.
- **Panels statt langer Menüs.** Ton, Untertitel, Qualität und Kapitel öffnen ein Panel rechts
  (Desktop) bzw. ein Blatt von unten (Handy). Das Video läuft weiter und bleibt sichtbar.
- **Akzentfarbe des Profils** für Fortschritt, gewählte Einträge und Lautstärke (wie heute
  die Zeitleiste im Foto, Ember-Orange).
- **Fernbedienung zuerst:** Jedes Element ist mit Pfeiltasten erreichbar, Zurück schliesst das
  offene Panel, dann den Player (C-4, wie heute).

## 5. Die zwei Varianten der Bedienleiste

### Variante A «Wie ein Streamingdienst» (Empfehlung)

Nah an Emby, aber aufgeräumt.

- **Oben links:** Zurück, darunter das **Logo** des Titels (`TitleOrLogo` gibt es schon), bei
  Folgen darunter «S1:E4 · Wahrheit». Oben rechts nichts ausser Bild-in-Bild und Vollbild.
- **Unten, über der Zeitleiste:** links Serienname/Titel klein, rechts **Restzeit und Endzeit**
  («noch 1:47:20 · endet um 23:41»).
- **Zeitleiste** mit Kapitelmarken, Intro/Abspann als hellere Abschnitte, Vorschaubild beim
  Ziehen (Stufe 3) bzw. Kapitelname (Stufe 1).
- **Unten links:** ⏮ (Kapitel/Folge zurück), −10, ⏯ (gross), +30, ⏭; daneben **Lautstärke**
  (Symbol, Regler klappt beim Zeigen auf).
- **Unten rechts:** **Untertitel** (CC mit Sprache), **Ton** (mit Sprache), **Einstellungen**
  (Zahnrad: Qualität, Geschwindigkeit, Untertitel-Stil, Ton-Optionen, Version), Stream-Info.
- **Reiterzeile** unter der Leiste (nur sichtbar, wenn die Leiste sichtbar ist): «Kapitel»,
  «Als Nächstes» (Folgen der Staffel), «Besetzung», «Info». Ein Reiter schiebt eine Bildreihe
  von unten herein, das Video rückt dafür **nicht** zur Seite.

### Variante B «Kino, ruhig»

Weniger auf dem Bild.

- Oben nur Zurück und Titel (Text, kein Logo).
- Unten **eine Zeile**: ⏯, −10, +10, Lautstärke, Zeitleiste über die ganze Breite, Restzeit,
  und rechts **ein einziger Knopf «Ton & Untertitel»** plus Zahnrad. Kapitel, Besetzung und
  «Als Nächstes» gibt es nicht als Reiter, sondern im Zahnrad bzw. in der Pause-Ansicht (V-10).
- Vorteil: sehr ruhig, nahe am heutigen Aufbau, kleiner Umbau. Nachteil: Kapitel und nächste Folgen
  sind zwei Klicks entfernt; die Leiste sieht auf grossen Bildschirmen leer aus (P-12 bleibt halb).

### 5.1 Vergleich

| | A Streamingdienst | B Kino ruhig |
|---|---|---|
| Klicks bis Ton/Untertitel | 1 | 1 (ein Panel für beides) |
| Klicks bis Kapitel | 1 (Reiter) | 2 (Zahnrad → Kapitel) |
| Lautstärke | Regler links neben Transport | Regler in der Zeile |
| Nächste Folgen sichtbar | Reiter «Als Nächstes» | nur am Ende (Countdown) und in der Pause |
| Umbau | grösser (neue Leiste, Reiter) | kleiner (Leiste ändern) |
| Fernbedienung | Pfeil runter öffnet die Reiter | wie heute |

**Empfehlung: A.** Sie löst P-1 bis P-6 sichtbar und passt zu dem, was Marco an Emby gefällt.

## 6. Die Vorschläge

### 6.1 Stufe 1 «Bedienung» – nur Web-App, kein Vertrag

| Nr. | Vorschlag | löst |
|---|---|---|
| V-1 | **Kopf mit Zusammenhang:** Logo oder Titel, bei Folgen «Serie · S1:E4 · Folgentitel» (`episodeCode`, Serie aus `ItemRef`). | P-2 |
| V-2 | **Transportknöpfe:** −10 s, +30 s (wie Emby; Pfeiltasten bleiben ±10), vorheriges/nächstes Kapitel, nächste Folge (nur bei Folgen). | P-3 |
| V-3 | **Lautstärke:** Stumm-Knopf (Taste M), Regler (beim Zeigen ausklappend, am Handy im Blatt), Mausrad über dem Video, **Wert je Gerät gemerkt** (`localStorage`, wie die Seitenleiste). Optional **Verstärkung bis 150 %** über Web Audio (`GainNode`), siehe F-5. **Am iPhone** erlaubt Safari keinen Regler (`volume` ist dort nur lesbar); dort gelten die Tasten des Telefons, der Player zeigt nur Stumm. | P-1 |
| V-4 | **Restzeit und Endzeit:** Klick auf die Zeit wechselt zwischen «0:54 / 21:53» und «−20:59 · endet um 23:41». | P-11 |
| V-5 | **Ton und Untertitel getrennt**, je ein Panel. **Sortiert:** zuerst die Sprachen des Profils (V-14; bis dahin: Sprache der App, dann Englisch), dann «Weitere Sprachen (17)» zugeklappt. **Beschriftet:** «Deutsch · DTS 5.1 · Standard», «Englisch · SDH», «Deutsch · erzwungen». **Kennzeichnet**, was eine Umwandlung auslöst («Bild-Untertitel, wird eingebrannt»). Die Leiste zeigt die gewählte Sprache als Wort. | P-4 |
| V-6 | **Untertitel-Stil:** Grösse (S, M, L, XL), Hintergrund (keiner, Schatten, halbtransparent, Box), Farbe (Weiss, Gelb), Position (unten, etwas höher). **Zeitversatz** ±0,1 s für Text-Untertitel (der Client verschiebt die WebVTT-Cues). Gilt je Gerät. Gezeichnet wird über `::cue` bzw. eine eigene Ebene, nicht über den Browser-Standard. | P-10 |
| V-7 | **Geschwindigkeit** 0,5× bis 2× (Bauteil aus dem Audioplayer). | P-11 |
| V-8 | **Kapitel sichtbar:** Marken auf der Zeitleiste, Kapitelname beim Zeigen; Reiter «Kapitel» als Bildreihe (bis Stufe 3 mit Nummer und Zeit statt Bild); Kapitel ohne Namen heissen «Kapitel 3», nicht `00:23:29.283`. | P-6 |
| V-9 | **Bild-in-Bild** (Picture-in-Picture-API, Chromium und Safari; Firefox hat einen eigenen Knopf). | P-11 |
| V-10 | **Pause-Ansicht:** Nach 5 s Pause dunkelt das Bild ab, links erscheinen Logo, Jahr, Laufzeit, Kurzbeschreibung und Kapitel. Jede Taste oder Bewegung blendet sie aus. | – |
| V-11 | **Reiter «Als Nächstes» und «Besetzung»:** Folgen der Staffel mit Bild und Fortschritt (vorhandene Endpunkte der Detailseite), Besetzung mit `CastCard`. Klick auf eine Folge spielt sie. | – |
| V-12 | **Tastatur und Hilfe:** M Stumm, J/L ±10, ⇧←/⇧→ Kapitel, `<`/`>` Geschwindigkeit, 0–9 springt auf 0–90 %, `?` zeigt alle Tasten. Bestehende Tasten bleiben. | – |
| V-13 | **Handy:** Doppeltipp links/rechts ±10 s, grosse Knöpfe, Panels als Blatt von unten; **Bildschirmsperre** (Schloss-Knopf: Bild läuft, Tippen tut nichts, Entsperren mit langem Druck), gedacht für Kinder (F-9). | P-12 |

### 6.2 Stufe 2 «Sprache, Qualität, Ton» – Server, additiv

| Nr. | Vorschlag | löst |
|---|---|---|
| V-14 | **Sprachvorlieben je Profil:** Tonsprachen in Reihenfolge (z. B. Deutsch, Englisch), Untertitel-Regel (**Aus**, **Nur erzwungen**, **Immer**, **Wenn Ton nicht in meiner Sprache**), Untertitelsprachen in Reihenfolge, Schalter «Originalton bevorzugen». Der **Server** wählt beim Start danach (Regel 12) und merkt sich eine Wahl im Player **je Serie** (wer bei «Silo» Englisch wählt, hört die nächste Folge auf Englisch). Einstellbar unter Profil → Wiedergabe und im Ton-Panel («Als Standard für dieses Profil»). | P-5 |
| V-15 | **Qualität mit Klartext:** «Original · 1080p HEVC · 4,6 Mbit/s · Direkt» als oberste Stufe, wenn das Gerät die Datei kann; **2160p 25 Mbit/s**, wenn die Quelle 4K ist (F-8); jede Stufe mit Bitrate und geschätzten GB pro Stunde. Oben im Panel **eine Zeile, warum** umgewandelt wird («Ton wird umgewandelt: DTS kann dieser Browser nicht»; Gründe gibt es schon, `streamInfo.reasons`). | P-8 |
| V-16 | **Ton-Optionen:** **Nachtmodus** (Dynamik zusammendrücken: leise lauter, laut leiser) im Client über Web Audio (`DynamicsCompressorNode`), funktioniert ohne Umwandlung; **Stereo erzwingen** (Kopfhörer) als Wunsch an den Server, der dann den Ton als AAC-Stereo liefert (nur Ton umwandeln, Bild bleibt direkt, sofern möglich). | P-9 |
| V-17 | **Qualität merken je Gerät:** Wer am Handy 720p wählt, bekommt am Handy wieder 720p (Wunsch des Geräts beim Start, Server entscheidet weiter nach Grenzen des Profils, W-8). | P-8 |
| V-18 | **Langsame Verbindung:** Puffert der Player zweimal in einer Minute, schlägt er eine Stufe tiefer vor («Verbindung langsam – auf 720p wechseln?»), wechselt aber nicht von selbst. | P-8 |

### 6.3 Stufe 3 «Bilder» – Server, neue Jobarten

| Nr. | Vorschlag | löst |
|---|---|---|
| V-19 | **Kapitelbilder:** neue Jobart `chapters.images`, ein Bild je Kapitel (ffmpeg springt an die Stelle, liest also nur wenige MB je Bild, kein ganzer Durchlauf), Ablage im Bild-Cache wie Poster. | P-6 |
| V-20 | **Ersatzkapitel:** Hat ein Film keine oder nur 1–3 Kapitel (Dune: Part Two), setzt der Server Marken alle 10 Minuten, im Panel als «0:10, 0:20, …» ohne Namen, deutlich als automatisch markiert (F-6). | P-6 |
| V-21 | **Vorschau beim Spulen (Trickplay):** ein Kachelbild je Datei, ein Bild alle 10 s, 320 px breit. **Teuer:** liest die ganze Datei (wie `probe.keyframes`, das die Bibliothek in rund 11 Tagen einmal durchläuft, CT-190-Stand), braucht rund 3–6 MB je Film. Vorschlag: nur **auf Abruf** für Titel, die abgespielt werden, oder gekoppelt an den Keyframe-Durchlauf (F-7). | P-7 |

### 6.4 Bewusst nicht vorgeschlagen

- **Adaptives HLS mit mehreren Stufen gleichzeitig** (wie Netflix): jede Stufe ist ein eigener
  Encode, bei Hardware-Transcoding auf CT 190 (QSV) zu teuer für mehrere Streams. V-18 genügt.
- **Chromecast:** braucht eine Empfänger-App (Non-Goal-Nähe «TV-Apps»), gehört in die
  Ideensammlung (`docs/ideas`). **AirPlay** (nur Safari, `webkitShowPlaybackTargetPicker`) wäre
  billig, ebenfalls als Idee.
- **Dialog anheben** (Mittelkanal lauter): geht nur mit 5.1-Ton im Server, also mit
  Ton-Umwandlung; als Idee notieren, falls der Nachtmodus nicht reicht.
- **«Läuft gerade», «TV Guide»:** Live-TV, Non-Goal.

## 7. Was sich im Vertrag und auf dem Server ändert

**Stufe 1:** nichts. Alles kommt aus `PlaybackInfo` (Tracks, Chapters, Segments, nextEpisode) und
den Endpunkten der Detailseite (Staffel, Besetzung). Lautstärke, Untertitel-Stil und Versatz
liegen im Browser je Gerät.

**Stufe 2 (additiv):**

- `ProfilePlaybackPrefs` am Profil: `audioLanguages[]`, `subtitleMode`
  (`off|forced|always|foreign`), `subtitleLanguages[]`, `preferOriginalAudio`; Endpunkte
  `GET/PUT /profiles/{id}/playback-prefs` (`x-visibility: account`).
- Tabelle `profile_series_tracks` (Profil, Serie, Tonsprache, Untertitelsprache) für «je Serie
  merken»; gelöscht mit dem Profil.
- `PlaybackRequest.audioOptions` (`stereo: bool`) und `PlaybackRequest.preferredQuality`.
- `QualityOption` erhält `bitrate`, `codec`, `direct` und die Stufe `original`; `Quality` erhält
  `2160p` (Enum-Erweiterung, Clients mit altem Stand zeigen sie als unbekannte Stufe).
- Server: `audioStream`/`decideAudio` wählen nach Vorlieben, dann Standard-Kennzeichen, dann
  erste Spur; Untertitel nach Regel. Test mit den echten Spurlisten von «Dune: Part Two» und
  «Blade Runner 2049» (Probe-JSON als Testdaten).

**Stufe 3:** Jobarten `chapters.images` und (falls gewählt) `trickplay.generate` im Scheduler
(Konzept 09), Bilder im Cache, `Chapter.imageUrl`, `PlaybackInfo.trickplay` (URL, Kachelraster,
Abstand). Falls Trickplay gewählt wird: **ADR 0021** zu Speicher und NAS-Last.

## 8. Grober Ablauf nach der Wahl

```text
Phase 1  Marco wählt Variante und beantwortet F-1 bis F-10
Phase 2  Fassung 2 «Bauplan»: Schritte, Tests, Frage 19, ggf. ADR 0021; OMC-Kritiker
Phase 3  Stufe 1 (Web-App)            ~ 6–8 Tage, eigener Zweig und PR
Phase 4  Stufe 2 (Sprache, Qualität)  ~ 5–7 Tage, Migration, Vertrag additiv
Phase 5  Stufe 3 (Bilder)             ~ 3 Tage Kapitelbilder, + 4 Tage Trickplay falls gewählt
```

Jede Stufe ist für sich nützlich und einzeln abnehmbar. Stufe 1 löst die meisten Klagen schon.

## 9. Klickbarer Prototyp

`docs/planning/mockups/13-abspielseite-prototyp.html` und privates Artefakt
`https://claude.ai/artifact/Qoz39AtPrRSRmR3RTx6Y7f` (Version 1, mit Bildern).
Die Bilder stammen aus Marcos eigener Bibliothek auf CT 190: 16 echte Einzelbilder aus
«Blade Runner 2049» (je eins pro Kapitel, mit ffmpeg aus der Datei gezogen), Logo und Folgenbilder
von «Silo» Staffel 1 aus dem Bild-Cache. **Die Bilder liegen nur im Artefakt, nicht im
Repository** (das Repository ist öffentlich, die Bilder sind urheberrechtlich geschützt); die
Datei im Repository zeigt ohne Bilder graue Flächen. Spurlisten sind die echten der Dateien.

Schalter im Prototyp:

- **Variante:** A Streamingdienst, B Kino ruhig, «Heute» (zum Vergleich).
- **Titel:** Blade Runner 2049 (Film, 16 Kapitel, DTS, Bild-Untertitel), Dune: Part Two
  (19 Tonspuren, 43 Untertitel), Silo S1:E4 (Folge mit «Als Nächstes»).
- **Ansicht:** Desktop, Handy.
- Klickbar: Abspielen/Pause, Lautstärke und Stumm, ±10/+30, Zeitleiste mit Kapitelmarken und
  Vorschau, Panels Ton, Untertitel (mit Stil und Versatz), Einstellungen (Qualität, Geschwindigkeit,
  Ton-Optionen), Reiter Kapitel, Als Nächstes, Besetzung, Info, Pause-Ansicht, Countdown nächste
  Folge, Bildschirmsperre (Handy).

## 10. Fragen an Marco

| Nr. | Frage | Empfehlung |
|---|---|---|
| F-1 | Welche Variante: **A** Streamingdienst oder **B** Kino ruhig? | A |
| F-2 | Lautstärke merken **je Gerät** oder **je Profil**? | je Gerät (Handy und TV-Browser sind verschieden laut) |
| F-3 | Untertitel-Regel als Standard für neue Profile? | «Nur erzwungen» |
| F-4 | Tonsprache: **Deutsch zuerst** oder **Originalton bevorzugen** als Standard? | Deutsch zuerst, Originalton als Schalter |
| F-5 | Nachtmodus und Verstärkung bis 150 % einbauen? | Nachtmodus ja; Verstärkung ja, mit Warnung ab 100 % |
| F-6 | Ersatzkapitel alle 10 Minuten bei Filmen ohne echte Kapitel? | Ja, als «automatisch» gekennzeichnet |
| F-7 | Vorschau beim Spulen (Trickplay): **jetzt**, **nur auf Abruf** oder **später**? | Nur auf Abruf (abgespielte Titel), wegen NAS-Last |
| F-8 | Zusätzliche Qualitätsstufen 2160p (25 Mbit/s) und «Original»? | Beide ja |
| F-9 | Bildschirmsperre am Handy (für Kinder)? | Ja, Stufe 1 |
| F-10 | Reihenfolge der Stufen 1 → 2 → 3, oder Sprache (Stufe 2) zuerst? | 1 → 2 → 3 |

## 11. Review-Protokoll

Noch keins; Fassung 1 geht nach Marcos Wahl an den OMC-Kritiker.
