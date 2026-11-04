# Konzept: Abspielseite (Videoplayer)

- **Status: Fassung 3 (1. November 2026), Bauplan für Variante A (Abschnitt 11), nach dem
  OMC-Kritiker (Abschnitt 12).** Fassung 1 legte
  zwei Varianten und 21 Vorschläge zur Wahl vor; ihr Text bleibt stehen, damit die Wahl
  nachvollziehbar ist. **Wo Abschnitt 11 etwas anderes sagt als die Abschnitte 5 bis 9, gilt
  Abschnitt 11.** Auslöser: Marco am 1. November 2026 mit zwei Bildschirmfotos (Slouchcat, «Two and
  a Half Men»; Emby, «Silo»): «Ich möchte die Lautstärke einstellen können, ich möchte Kapitel,
  bessere Transcoding-Einstellungen, Untertitel-Auswahl, Sprach-Auswahl etc.» **Marco hat
  Variante A gewählt und F-2 bis F-15 beantwortet** (Abschnitt 10). Klickbarer Prototyp mit Bildern
  aus der eigenen Bibliothek: Abschnitt 9. **Die Umsetzung beginnt erst auf Marcos ausdrückliches
  Wort.**
- **Wogegen dieses Konzept verankert ist:** `main` **`5e0e63d`** («Merge pull request #61»),
  Vertragsfassung **1.6.0**. Konzept 11 reserviert Frage 17, ADR 0019, Migration 00031 und
  Vertrag 1.7.0; Konzept 12 (Zweig `claude/konzept-verwaltungsseiten`, nicht gemergt) nimmt
  Frage 18 und ADR 0020. Dieses Konzept nimmt darum **Frage 19** und **ADR 0021** (2160p);
  Migration und Vertragsfassung sind die nächsten freien nach 11 und 12 (voraussichtlich additiv
  in **1.8.0**). **Nachtrag 2. November 2026:** Konzept 12 nimmt 1.8.0 als eigenes Release
  (Frage 18); dieses Konzept rückt auf **1.9.0 oder später** und behält Frage 19 und ADR 0021. **Nachtrag 4. November 2026:** Bau neu ab `main` `c023448`, Verträge 1.10.0 bis 1.12.0 je Stufe, Stopp durch die Verwaltung in Stufe 1 (Abschnitt 11.15).
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
| P-5 | **Keine Sprachwahl je Profil.** Es läuft die Tonspur mit dem Standard-Kennzeichen der Datei, sonst die erste; bei «Dune: Part Two» Englisch, ohne Kennzeichen wäre es Italienisch. Untertitel sind immer aus, auch wenn die Datei eine erzwungene Spur als Standard markiert (`decide.go:315-326`). | `audioStream` (`transcode/plan.go:371`), `decideAudio` (`playback/decide.go:415`); das Profil kennt keine Sprachen. |
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
| V-12 | **Tastatur und Hilfe:** M Stumm, ⇧←/⇧→ Kapitel, `[`/`]` Geschwindigkeit (wie im Audioplayer), 0–9 springt auf 0–90 %, `?` zeigt alle Tasten. Bestehende Tasten bleiben. | – |
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

### 10.1 Marcos Wahl (1. November 2026)

**F-1: Variante A «Wie ein Streamingdienst».** Variante B entfällt; Abschnitt 5 bleibt stehen,
damit die Wahl nachvollziehbar ist.

### 10.2 Marcos Antworten (1. November 2026)

| Nr. | Antwort | Folge |
|---|---|---|
| F-2 | Je Gerät | wie empfohlen (V-3, `localStorage`) |
| F-3 | Nur erzwungen | wie empfohlen; Standard für neue und bestehende Profile |
| F-4 | **«Der Ton, den das Profil als Standard festgelegt hat, sonst der Originalton.»** | **Abweichung:** kein «Deutsch zuerst» und kein Rückfall auf den Standard der Datei. Reihenfolge: Sprachen des Profils, dann Originalsprache des Titels, erst wenn beides fehlt die Standardspur der Datei. Braucht die **Originalsprache je Titel** (Abschnitt 11.3). |
| F-5 | Nachtmodus und Verstärkung bis 150 % | **Stereo erzwingen entfällt** (V-16 nur noch Client, kein Wunsch an den Server) |
| F-6 | Ja, markiert | wie empfohlen (V-20) |
| F-7 | **Später** | **V-21 Trickplay entfällt** in diesem Konzept; die Zeitleiste zeigt beim Zeigen Zeit, Kapitelname und, ab Stufe 3, das **Kapitelbild** des Kapitels unter dem Zeiger. Trickplay geht in die Ideensammlung. |
| F-8 | Original und 2160p | wie empfohlen (V-15) |
| F-9 | **Nein** | **Bildschirmsperre entfällt** (V-13 ohne Sperre) |
| F-10 | Erst Bedienleiste (1 → 2 → 3) | wie empfohlen |

### 10.3 Marcos Entscheide nach dem Review (1. November 2026)

Der OMC-Kritiker hat Fassung 2 geprüft (Abschnitt 12). Fünf Punkte brauchten Marcos Entscheid:

| Nr. | Frage | Antwort |
|---|---|---|
| F-11 | Erzwungene Untertitel, die es nur als **Bild** gibt (müssen eingebrannt werden, Film wird dann umgewandelt) | **Nur Text automatisch.** Bild-Spuren nur, wenn ohnehin umgewandelt wird; sonst Hinweis im Panel |
| F-12 | Vorbelegung der Sprachen für bestehende Profile, neue Profile und Gäste | **App-Sprache des Profils** (`profiles.language`), Ton und Untertitel |
| F-13 | Wer legt die Sprachen eines Profils fest? | **Jedes Profil selbst**; das Hauptprofil zusätzlich für alle |
| F-14 | 2160p, obwohl CT 190 HDR nur auf der CPU umrechnet (2160p dort fast nie wählbar) | **Trotzdem einbauen** |
| F-15 | Neue Leiste auch im Offline-Player? | **Ja, gleiche Leiste** |

## 11. Umsetzung von Variante A (Fassung 3)

Grundlage: Abschnitt 10 (F-1 bis F-15), Code-Stand `main` `5e0e63d`; die Zeilenangaben beziehen
sich darauf.

### 11.1 Was gebaut wird und was nicht

| Bleibt | Entfällt |
|---|---|
| V-1 bis V-12, V-13 ohne Sperre | Variante B (F-1) |
| V-14 Sprachen je Profil nach F-4, F-11, F-12, F-13 | Bildschirmsperre (F-9) |
| V-15 Qualität mit «Original» und «2160p» (F-8, F-14) | «Stereo erzwingen» (F-5) |
| V-16 Nachtmodus, Verstärkung bis 150 % | V-21 Trickplay (F-7, geht in `docs/ideas`) |
| V-17 Qualität je Gerät merken, V-18 Hinweis bei langsamer Verbindung | |
| V-19 Kapitelbilder, V-20 Ersatzkapitel | |
| Versionswahl (B-5) im Zahnrad | |
| Offline-Player mit derselben Leiste (F-15) | |

Ohne Trickplay zeigt die Zeitleiste beim Zeigen Zeit, Kapitelname und ab Stufe 3 das
**Kapitelbild** des Kapitels unter dem Zeiger.

### 11.2 Zielbild der Abspielseite

**Desktop und Tablet:**

- **Kopf** links: Zurück, `TitleOrLogo`, darunter bei Folgen «S1:E4 · Wahrheit», bei Filmen
  Jahr und Genres. Rechts: Bild-in-Bild (nur wenn `document.pictureInPictureEnabled`), Vollbild.
  **Daten:** `PlaybackInfo.item` ist ein `ItemRef` ohne Logo und Genres. Der Player lädt dafür
  `itemQuery(item.id)` (`catalog/queries.ts:46-51`), bei Folgen zusätzlich die Serie über
  `ItemDetail.series` (zweiter Abruf, aus dem Cache der Detailseite, wenn man von dort kommt).
  Bis die Daten da sind, steht der Titel als Text da.
- **Zeile über der Zeitleiste:** links Titel bzw. Serie und Kapitelname, rechts die Zeit; ein Klick
  wechselt zu «−1:38:16 · endet um 23:41» (Wahl je Gerät gemerkt).
- **Zeitleiste** (`SeekBar`): Kapitelmarken, Intro und Abspann heller, beim Zeigen ein Feld mit
  Zeit, Kapitelname und (Stufe 3) Kapitelbild.
- **Links unten:** Kapitel bzw. Folge zurück, −10 s, Abspielen/Pause, +30 s, Kapitel bzw. Folge vor,
  Lautstärke (Stumm-Knopf, Regler klappt bei Zeiger und Fokus auf).
- **Rechts unten:** «Untertitel» mit Sprache, «Ton» mit Sprache, Zahnrad mit der Qualität, «?».
- **Zahnrad:** Qualität, **Version** (nur bei zwei oder mehr Dateien, wie heute), Geschwindigkeit,
  Untertitel-Stil, Ton-Optionen, Stream-Info.
- **Reiter** unter der Leiste: Kapitel, Als Nächstes (nur Folgen), Besetzung, Info.
- **Panels** rechts; die Leiste bleibt sichtbar, solange ein Panel offen ist.
- **Pause-Ansicht** nach 5 s Pause; **Nächste Folge** als Karte mit Folgenbild (W-9 wie heute).

**Handy** (Breite unter 768 px oder `(pointer: coarse) and (max-height: 500px)`): −10/Pause/+10
gross in der Mitte, Ton, Untertitel, Zahnrad oben rechts, Panels als Blatt von unten, Kapitel im
Zahnrad, Doppeltipp links/rechts ±10 s, kein Reiterband. iPhone: kein Lautstärkeregler (Safari
lässt `volume` nicht setzen), nur Stumm.

**Fokusmodell** (Regel 8, Fernbedienung C-4). Der Player hat drei Zonen:

| Fokus auf | ←/→ | ↑/↓ | Enter/Leertaste |
|---|---|---|---|
| nichts bzw. Video (Leiste ausgeblendet) | ±10 s | Lautstärke ±5 % | Abspielen/Pause |
| Zeitleiste | ±10 s (⇧: Kapitel) | ↑ Lautstärke, ↓ in die Knopfreihe | Abspielen/Pause |
| Knopfreihe | von Knopf zu Knopf (roving tabindex) | ↑ Zeitleiste, ↓ Reiter | Knopf auslösen |
| Reiter und Bildreihe | zwischen Reitern bzw. Karten | ↑ zurück in die Knopfreihe | Reiter öffnen, Karte wählen |
| Panel | – | zwischen Zeilen | Zeile wählen |

Heute springen ←/→ immer, auch mit Fokus auf einem Knopf (`VideoPlayer.tsx:476-490`); das ändert
sich bewusst. Die Buchstabentasten gelten in jeder Zone ausser in Panels mit Texteingabe:

| Taste | Wirkung |
|---|---|
| Leertaste, K | Abspielen/Pause |
| **M** | Stumm |
| T | **Ton-Panel** (heute Menü «Spuren») |
| **U** | **Untertitel-Panel** |
| C | **Reiter Kapitel** (Handy: Kapitel im Zahnrad) |
| Q | Qualität |
| **[ ]** | Geschwindigkeit ∓0,25 (wie Audioplayer) |
| **0–9** | springt auf 0–90 % |
| F, I, S, N | wie heute |
| **?** | Tastenhilfe |
| Escape, Backspace | schliesst Panel, Reiter, Hilfe; dann zurück zur Detailseite |

### 11.3 Spur- und Untertitelwahl (F-3, F-4, F-11)

**Regel 12:** Der Server wählt, der Client sortiert nur die Anzeige.

**Vertrag, Bedeutung der Felder (Blocker B-2 behoben).** Heute heissen fehlendes **und**
`null` bei `subtitleTrack` «keine Untertitel» (`openapi.yaml:8017-8021`), der Handler macht aus
beidem `0` (`item_handlers.go:247`, `model.go:1375`). Neu:

- `audioTrack` fehlt → der Server wählt nach der Regel unten.
- `subtitleTrack` fehlt **und** `subtitleOff` nicht `true` → der Server wählt.
- neues Feld **`subtitleOff: true`** → keine Untertitel. `subtitleTrack: null` wird wie «fehlt»
  behandelt (Änderung der Bedeutung, im Changelog).
- **Jeder Wechsel im Player** (Ton, Untertitel, Qualität, Version) schickt Ton **und** Untertitel
  ausdrücklich mit (`audioTrack`, `subtitleTrack` oder `subtitleOff`); nur der erste Start lässt
  den Server wählen. Sonst würde ein Tonwechsel die Untertitel neu wählen.

**Gemeinsame Funktion.** Die Regel steht einmal, in einem neuen Blattpaket
**`internal/trackpick`** (keine Abhängigkeiten ausser `model`). Es nutzen sie `playback`
(`Decide`, `decide.go:189` `defaultAudio`, `transcode/plan.go:371` `audioStream`) **und**
`offline` (`offline/profiles.go:65` `defaultAudio`, `offline/downloads.go:96` `defaultIndex`),
damit Downloads und Auto-Downloads dieselbe Sprache bekommen.

**Tonspur**, wenn die Anfrage keine nennt:

1. **Serie gemerkt:** bei Folgen die zuletzt von Hand gewählte Tonsprache dieses Profils für
   diese Serie (`profile_series_tracks`), falls die Datei sie hat.
2. **Profil:** die erste Sprache aus `audio_languages`, die die Datei hat.
3. **Originalton:** `original_language` des Films bzw. der Serie.
4. **Datei:** Standard-Kennzeichen, sonst erste Spur (wie heute).

Mehrere Spuren derselben Sprache: zuerst eine, die das Gerät dekodiert (`caps.Audio`), dann
Standard-Kennzeichen, dann mehr Kanäle, dann Reihenfolge; Spuren mit «Commentary» oder
«Kommentar» im Titel zuletzt. Spuren mit `und` oder ohne Sprache passen auf keine Sprache und
kommen nur in Schritt 4 zum Zug.

**Kosten:** Eine gewählte Spur, die nicht die Standardspur der Datei ist, macht Direct Play
unmöglich (`ReasonAudioTrack`, `decide.go:302-306, 352, 390`): die Datei wird remuxt, ohne
Keyframe-Index sogar umgewandelt (`sessions.go:510-513`). Beispiel: Eine MP4 mit Englisch als
Standard und ein deutsches Profil heisst jedes Mal Remux. Das ist gewollt (F-4) und geht in die
Messung (11.4). `directFallback` (`sessions.go:354`) fällt heute still auf die Standardspur
zurück; neu meldet es die tatsächlich gespielte Spur mit `audioReason: file`.

**Untertitel**, wenn die Anfrage keine nennt:

1. **Serie gemerkt:** bei Folgen die zuletzt gewählte Untertitelsprache bzw. «aus».
2. **Regel des Profils** (`subtitle_mode`):

| Modus | Wahl |
|---|---|
| `off` | keiner |
| `forced` (Standard, F-3) | erzwungene **Text**-Spur in der Sprache des gewählten Tons |
| `foreign` | wie `forced`; ist der Ton **nicht** in einer Profilsprache: volle Spur in der ersten Untertitelsprache |
| `always` | volle Spur in der ersten Untertitelsprache, sonst wie `forced` |

**Bild-Spuren (F-11):** Eine erzwungene Spur, die es nur als Bild gibt (PGS, DVD), wählt der
Server **nur**, wenn die Wiedergabe ohnehin umgewandelt wird. Sonst wählt er keine und meldet
`subtitleReason: forced_bitmap_available`; das Untertitel-Panel zeigt «Erzwungene Untertitel
verfügbar (werden eingebrannt)». Bei vollen Spuren gehen Text-Spuren vor Bild-Spuren derselben
Sprache. SDH-Spuren («SDH», «für Hörgeschädigte», bei externen Dateien `.sdh`/`.hi`, das der
Scan schon erkennt, `library/probe.go`, aber heute nicht speichert) nur bei `always`, wenn keine
andere da ist; `tracks` bekommt dafür `is_hearing_impaired`.

**Begründung** im Vertrag: `TrackSelection.audioReason` (`request`, `series`, `profile`,
`original`, `file`), `TrackSelection.subtitleReason` (`request`, `series`, `mode`, `off`,
`forced_bitmap_available`). Bei Hörbüchern und Podcasts (derselbe `Start`) ist beides `file` bzw.
`off`; der Audioplayer zeigt nichts davon.

**Je Serie merken:** Wählt das Profil während einer Folge von Hand Ton oder Untertitel und ist
«Für diese Serie merken» an (Standard an), schickt der Client `rememberForSeries: true`; der
Server schreibt die **Sprachen** nach `profile_series_tracks` (Spur ohne Sprache: Sprache leer,
dann zählt der Eintrag nicht). Zurücksetzen auf der Seite «Meine Wiedergabe» (11.6).

**Datenzugriff (Regel 2):** `playback` stellt keine SQL-Abfrage auf `items`. Neu ist eine Methode
in `visibility`, die zu einem sichtbaren Titel die **Serien-ID** (bei Folgen über Staffel zur
Serie; `ItemSummary.ParentID` ist die Staffel, `model.go:486-488`) und die **Originalsprache**
liefert. `profile_series_tracks` berührt `items` nicht und liegt in einer eigenen Abfragedatei
neben den Abfragen für `playback_sessions`; die Serien-ID kommt immer aus `visibility`.

**Sprachcodes:** Profile speichern ISO 639-1 (`de`, `en`), wie `profiles.language`. Eine Tabelle
in `trackpick/languages.go` bildet ab: ISO 639-2/B und /T (`ger`, `deu`), Namen aus externen
Untertiteln (`german`), Regionen (`pt-br` → `pt`), **`gsw` (Schweizerdeutsch) → `de`**, TMDB
`cn` → `zh`, `xx` → keine. Ein Paritätstest prüft sie gegen die Tabelle des Web-Clients
(`labels.ts:20-30`).

### 11.4 Profile und Vorbelegung (F-12, F-13)

- **Jedes Profil selbst (F-13):** neue Operationen **`GET` und `PUT /me/playback-prefs`** für das
  aktive Profil der Sitzung (`x-visibility: scoped`), dazu **`DELETE /me/playback-prefs/series`**
  (Serienmerker zurücksetzen). Das Hauptprofil setzt dieselben Felder für alle Profile über
  `ProfileUpdate` (heute `requirePrimary`, `auth/profiles.go:179`). Ein Wert, zwei Türen.
- **Seite «Meine Wiedergabe»** unter den Einstellungen für jedes Profil (Tonsprachen mit
  Reihenfolge, Untertitel-Regel, Untertitelsprachen, Serienmerker zurücksetzen); dieselben Felder
  im `ProfileForm.tsx` für das Hauptprofil. Das Ton-Panel bietet «Als Standard für dieses
  Profil» für **jedes** Profil an.
- **Vorbelegung (F-12):** Die Migration belegt `audio_languages` und `subtitle_languages` aller
  bestehenden Profile mit `["<language>"]` aus `profiles.language`. Neue Profile ebenso, an
  **allen drei** Stellen, die Profile anlegen: `auth/profiles.go:106/156` (Hauptprofil legt an),
  `auth/invitations.go:325` (Gäste), `auth/auth.go:227` (Einrichtung). Je ein Test.
  `metadata_language` (Migration 00028) bleibt aussen vor: sie steuert Titel und Texte, nicht den
  Ton.

### 11.5 Qualität (F-8, F-14)

**`original`:** Das **Bild wird nicht neu kodiert** (Direct Play oder Remux; der Ton darf
umgewandelt werden). Unterschied zu `auto`: `auto` darf umwandeln, `original` nie. Genau
definiert:

- (a) Remux ohne fertigen Keyframe-Index wäre ein Transcode (`sessions.go:512`): `original` ist
  dann **nicht verfügbar** («Index wird erstellt»), es wird nicht still umgewandelt.
- (b) Eine eingebrannte Untertitelspur ist gewählt: `original` nicht verfügbar («Untertitel
  werden eingebrannt»).
- (c) V-17 schickt ein gemerktes `original`, das nicht geht: der Server spielt `auto`, ohne
  Fehler; `QualityOption.current` zeigt `auto`.
- (d) `recordedDecision` und `rungIndex` behandeln `original` als eigene Stufe, nicht als `auto`.
- Läuft `auto` gerade ohne Umwandlung, heisst die Zeile «Automatisch · zurzeit Original».

**`2160p` (F-14).** Neu über die heutige Grenze hinaus: AN-17 verlangt 1080p nur für
**Software**-Transcoding, der Code deckelt aber jeden Transcode. Verfügbar ist 2160p, wenn die
Quelle mindestens 2160 Zeilen hat, ein **Hardware-Backend** läuft, das Profil `max_height` 2160
erlaubt, das Gerät mindestens 1440 Zeilen meldet und die Quelle **SDR** ist oder das Backend auf der
GPU tonemappt. Auf CT 190 (QSV, Tonemapping auf der CPU) also nur bei 4K-SDR-Quellen, **das ist
selten** (Marco weiss das, F-14). `auto` wählt nie selbst 2160p. Änderungen in `transcode`:

- `MaxTranscodeHeight` (`decide.go:46`) wird je Backend: Software 1080, Hardware 2160.
- `heightLimit` (`transcode/plan.go:166-169`) klemmt nicht mehr fest auf 1080.
- Die Kommentarregel «Tonemap immer mit MaxHeight ≤ 1080» (`plan.go:50`) gilt weiter: 2160p mit
  Tonemapping nur auf GPU-Backends.
- `-level:v 4.1` (`pipeline.go:363, 394, 421, 431`) und der HLS-Codec-String `avc1.640029`
  (`plan.go:427`) werden aus der Ausgabehöhe abgeleitet (2160p: Level 5.1, `avc1.640033`).
- `ladder`, `QualityFor`, `Rungs` bekommen 2160p (25 Mbit/s); die doppelte Leiter
  (`playback/quality.go:20-25`) entfällt, `playback` liest `transcode.Rungs`.
- **Budget:** Die Kosten folgen Quelle und Pipeline (`Cost`, `plan.go:303-330`; Plan 3.5; 4K-SDR
  heute 3). Ein 2160p-Transcode kostet **5** von 8 Einheiten auf QSV (Dekodieren und Kodieren in
  4K); der Wert wird in der Messung kalibriert und in **ADR 0021** begründet.
- **Datenbank (Blocker B-1):** `playback_sessions.quality` hat einen `CHECK` nur bis 1080p
  (`00011_playback_complete.sql:32`). Die Migration baut die Tabelle nach Vorlage neu auf (neue
  Tabelle, kopieren, umbenennen, `foreign_key_check`), mit `original` und `2160p` im `CHECK`.
- **Messung** auf CT 190 vor dem Merge: Start, Spulen und gleichzeitige Streams für 2160p (SDR)
  und für den Remux durch Profilsprache (11.3) in `docs/perf/13-abspielseite.md`.

**`QualityOption`** hat `height` und `bitrate` schon (`openapi.yaml:8176-8192`); neu nur `codec`
und `direct`. Der Client rechnet «ca. 3,6 GB pro Stunde».

**V-17 (je Gerät merken):** Gemerkt wird nur eine Wahl aus dem Qualitäts-Panel, nie ein Wechsel
nach dem Hinweis V-18. Eine gemerkte Stufe unter 2160p geht als neues Feld
**`PlaybackRequest.qualityCap`** an den Server und wirkt wie eine Gerätegrenze (`rungAllowed`,
`quality.go:82-95`): Dateien innerhalb der Grenze laufen weiter direkt. Heute erzwingt eine
gewählte Stufe immer einen Transcode (`decide.go:349`). Das Panel zeigt «gemerkt: 720p» mit
«Zurücksetzen». `original` und `auto` gehen wie heute als `quality`.

**V-18:** Der Client zählt Pufferpausen (`waiting` über 1 s); zwei in 60 s zeigen «Verbindung
langsam – auf 720p wechseln?» mit Knopf, ohne selbst zu wechseln.

### 11.6 Kapitel (V-8, V-19, V-20)

- **Eine Funktion** `transcode.Chapters(probe, kind, durationMs)` liefert die Kapitel einer Datei;
  `playback` und der Job nutzen sie (ein Aufruf aus `library` wäre ein Importzyklus,
  `sessions.go:18`). Nur für **Filme und Folgen**; Hörbücher und Podcasts behalten ihre Kapitel
  wie heute (`sessions.go:243-245`, `audio.go:103-111`).
- **Titel bereinigen:** Titel, die nur eine Zeit (`00:12:36.172`) oder «Chapter 12» bzw.
  «Kapitel 12» sind, gehen ohne Titel hinaus (`Chapter.title` ist schon optional, die Go-Struktur
  lässt leere Titel weg). Der Client nummeriert **nach Position** in der gelieferten Liste.
- **Ersatzkapitel (F-6):** weniger als 4 Kapitel und mindestens 30 Minuten → Marken alle 10
  Minuten, `Chapter.auto: true`; ein letzter Abschnitt unter 3 Minuten geht im vorigen auf; ein
  echtes Kapitel «Credits»/«Abspann» bleibt als Marke.
- **Kapitelbilder (Stufe 3):** Methode **`Runner.GrabFrame(ctx, path, atMs, width)`** in
  `transcode` (Muster `Runner.ScaleImage`, `images.go:100, 197-202`; Regel 3). Zeitpunkt
  Kapitelbeginn plus ein Zehntel der Kapitellänge, höchstens 30 s. HDR- und Dolby-Vision-Quellen
  werden auf der CPU tonemappt (480 px, billig). WebP, 480 px.
- **Jobs (M-10):** Event-Jobart **`chapters.images`** je Mediendatei (ohne `Run`, Muster
  `segments.detect`) und manuelle Jobart **`chapters.images_backfill`** mit `Run`, die alle
  Dateien ohne Bilder einreiht; sie startet nur von Hand auf der Seite Hintergrundjobs.
  Automatisch eingereiht werden (1) Dateien, die **nach** dem ersten vollständigen Scan ihrer
  Bibliothek dazukommen, und (2) eine abgespielte Datei ohne Bilder, **nach dem Ende** der
  Wiedergabe (nicht während, sonst konkurriert der Job mit dem eigenen Transcode). Priorität 5,
  unter Hand-Läufen (7) und Keyframe-Bedarf (9).
- **Ablage:** `cache/chapters/<mediaFileId>-<probedAt>/<idx>-w480.webp`. Ein neuer Probe mit
  anderen Kapiteln ergibt einen neuen Ordner, der alte wird gelöscht; ebenso mit der Mediendatei.
  Grösse rund 25 KB je Bild.
- **Ausliefern:** `GET /items/{id}/chapters/{idx}/image?mediaFileId=` als `http.Handler`
  (Regel 6), `x-visibility: scoped`, Eintrag in `binaryRoutes` (`api/api.go:53`) und im
  Vertragstest. Die Datei wird über **`vis.MediaFile(ctx, scope, itemID, fileID)`**
  (`visibility.go:343`) aufgelöst, nicht nur der Titel; `mediaFileId` muss eine ULID sein, `idx`
  eine Zahl innerhalb der Kapitelliste; sonst 404. Vertragstest auch für «Datei eines anderen,
  unsichtbaren Titels» (404).

### 11.7 Originalsprache (F-4)

- `items.original_language`: `NULL` = noch nicht abgefragt, `''` = abgefragt, unbekannt,
  sonst ISO 639-1.
- Teil von **`metadata.fill`** (`fill.go:19-35`) mit Herkunft in `item_field_sources`,
  sperrbar über `locked_fields` (B-6) und in der Titelbearbeitung der Verwaltung änderbar.
  Quellen in dieser Reihenfolge: TMDB `original_language` (dieselben Antworten, die `tmdb.go:113,
  217` lesen), TVDB `originalLanguage` (`tvdb.go:331`), AniList über `countryOfOrigin`
  (JP → `ja`, KR → `ko`, CN → `zh`).
- **Bestand:** manuelle Jobart **`metadata.original_language_backfill`** (Muster
  `vote_backfill`, `fill.go:518`) mit `Run`, gedrosselt wie die übrigen TMDB-Abrufe; nimmt nur
  Titel mit `NULL`. Einmal automatisch beim ersten Start nach der Migration eingereiht (wie
  `QueuePendingMetadata`, `match.go:1288`), danach nur von Hand. Bis dahin gilt Schritt 4 der
  Tonregel.

### 11.8 Vertrag (additiv, eine Bedeutungsänderung)

Fassung beim Release festlegen (Regel Vertrag = Tag), nach 11 und 12 voraussichtlich **1.8.0**.
Nachtrag 2. November 2026: 1.8.0 gehört Konzept 12 (Frage 18); hier gilt **1.9.0 oder später**.

| Stelle | Neu |
|---|---|
| `Profile`, `ProfileRequest`, `ProfileUpdate` | `audioLanguages`, `subtitleMode` (`off`, `forced`, `foreign`, `always`), `subtitleLanguages` |
| neu | `getMyPlaybackPrefs`, `putMyPlaybackPrefs` (`/me/playback-prefs`), `resetMySeriesTracks` (`DELETE /me/playback-prefs/series`), alle `scoped` |
| `PlaybackRequest` | `subtitleOff`, `rememberForSeries`, `qualityCap`; **Bedeutung:** fehlende Spur = Server wählt, `subtitleTrack: null` = wie fehlend (Changelog) |
| `TrackSelection` | `audioReason`, `subtitleReason` |
| `Quality` | `original`, `2160p` (Enum-Erweiterung, Web-Client im selben PR, Changelog) |
| `QualityOption` | `codec`, `direct` |
| `Chapter` | `auto`, `imageUrl` (Stufe 3) |
| neu (Stufe 3) | `getChapterImage`, `http.Handler`, `scoped` |
| `ItemDetail` (Verwaltung) | `originalLanguage` (lesen und sperren wie andere Felder) |

Nach jeder Vertragsänderung `task gen` (strict server, `packages/api-client`).

### 11.9 Migration

Eine Migration nach Vorlage (Regel 5), Nummer = nächste freie beim Start von Stufe 2 (nach 00031):

- `profiles`: `audio_languages TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(audio_languages))`,
  `subtitle_mode TEXT NOT NULL DEFAULT 'forced' CHECK (subtitle_mode IN ('off','forced','foreign','always'))`,
  `subtitle_languages` wie `audio_languages`; Bestand aus `profiles.language` (F-12).
- `profile_series_tracks (profile_id, series_id, audio_language, subtitle_language, subtitle_off,
  updated_at)`, Primärschlüssel (profile_id, series_id), `ON DELETE CASCADE` an Profil und Titel.
- `items.original_language TEXT` (Bedeutung 11.7).
- `tracks.is_hearing_impaired INTEGER NOT NULL DEFAULT 0`.
- `playback_sessions` neu aufgebaut mit erweitertem `CHECK` (11.5).
- Eingefrorener DB-Stand in `testdata/db/`; Migrationstest fügt `original` und `2160p` ein.

### 11.10 Oberfläche im Detail (Regel 9)

**Bauteile in `web/src/ui`** (je Story und Storybook-Baseline):

| Bauteil | Zweck |
|---|---|
| `VolumeControl` | Stumm und Regler 0–100 %, mit Verstärkung 0–150 %; der Audioplayer (`ui/AudioPlayer.tsx:434-454`) nutzt es danach auch |
| `PlayerPanel` | Seitenpanel bzw. Blatt; Fokus beim Öffnen auf die gewählte Zeile, Escape/Backspace schliesst, keine Fokusfalle |
| `OptionRow` | Zeile mit Haken, zwei Textzeilen, Kennzeichen; auf `SelectableList`, falls dessen Rollen passen |
| `ChapterCard` | Kapitelbild bzw. Nummer, Titel, Zeit, Fortschritt, «automatisch» |
| `PlayerChrome` | Kopf, Leiste, Reiter mit Fokusmodell; nutzen Online- und Offline-Player |
| `SeekBar` (von `features/player` nach `ui`) | Kapitelmarken, Abschnitte, Vorschaufeld |
| bestehend | `Tabs`, `ChoiceChips`, `EpisodeCard`, `CastCard`, `TitleOrLogo`, `Switch`, `AriaButton` |

**`features/player`:** `VideoPlayer.tsx` (1046 Zeilen) wird aufgeteilt in `panels/` (Audio,
Subtitle, Settings, Quality, Version, Speed, SubtitleStyle, Sound, StreamInfo), `tabs/`
(Chapters, Next, Cast, Info), `PauseView.tsx`, `ShortcutHelp.tsx`, Hooks `useVolume.ts`,
`useSubtitleStyle.ts`, `usePlayerPrefs.ts`. Sitzungslogik (Wechsel, Widerruf, Countdown,
Fortschritt) bleibt in `VideoPlayer`.

**Offline-Player (F-15):** `OfflinePlayer.tsx` (872 Zeilen) importiert heute `SeekBar` und
`Player.module.css` (Zeilen 4, 14; `OfflinePlayerPage.tsx:22`). Er nutzt danach `PlayerChrome`
mit Fähigkeiten-Schaltern: keine Qualität, keine Version, kein Spurwechsel über den Server (Ton
und Untertitel nur, soweit im Download enthalten), Kapitel und Reiter aus den gespeicherten
Metadaten, Lautstärke, Untertitel-Stil, Nachtmodus wie online.

- **Lautstärke und Ton-Optionen:** `element.volume` bis 100 %; Verstärkung und Nachtmodus über
  Web Audio (`createMediaElementSource` → `DynamicsCompressorNode` → `GainNode`), erst beim
  ersten Einschalten nach einer Nutzeraktion, dann bis zum Verlassen des Players. **Vor Schritt 4
  ein Spike** in Safari (macOS, iOS) mit nativem HLS: WebKit lieferte bei HLS-Quellen über
  `createMediaElementSource` früher Stille. Geht es nicht, sind Nachtmodus und 150 % in Safari
  ausgeblendet.
- **Untertitel:** Text-Untertitel zeichnet der Player in einer eigenen Ebene (`<track>` im Modus
  `hidden`, Cues aus `cuechange`); Stil, Position, Anheben über die Leiste und **Versatz** (Zeit
  beim Abfragen verschoben, ±0,1 s je Gerät) wirken so in allen Browsern gleich. **Bild-in-Bild:**
  das PiP-Fenster zeigt nur native Cues; bei `enterpictureinpicture` schaltet der Player auf
  `showing` mit `::cue`-Stil und verschobenen Cue-Zeiten, bei `leavepictureinpicture` zurück.
- **Sortierung** in Ton- und Untertitel-Panel: Profilsprachen zuerst, «Weitere Sprachen (n)»
  alphabetisch, zugeklappt. In Stufe 1 dient `profiles.language` als Profilsprache.
- **«Als Nächstes», «Besetzung»:** aus `itemQuery(seasonId)` bzw. `itemQuery(id)`
  (`ItemPage.tsx:402-425, 626-631`), keine neuen Endpunkte.
- **Texte** nur in `de.json`/`en.json`; Sprachnamen über `languageName` (`labels.ts`).

### 11.11 Tests

- **Server:** `trackpick` als Tabellentest mit den Probe-JSON von «Dune: Part Two», «Blade Runner
  2049» und «Silo» S1:E4 (vor dem Einchecken `format.filename` und Tags entfernt, das Repository
  ist öffentlich); jede Stufe der Regeln, jeder Modus, Bild-Spuren nach F-11, `und`-Spuren,
  Gleichstand nach Codec. Anfrage-Test «fehlt» gegen `subtitleOff`. Ersatzkapitel,
  Titelbereinigung, nur Film und Folge. `original` (a) bis (d), `2160p` (Software → nicht
  verfügbar, HDR auf QSV → nicht verfügbar, Level und Codec-String). Migration (Vorbelegung,
  `playback_sessions`). Vorbelegung an allen drei Anlagestellen. `fill` mit Originalsprache und
  Sperre. `GrabFrame` mit dem Testmedien-Generator. Vertragstest für die neuen Operationen, auch
  «fremde Datei» bei `getChapterImage`. Offline-Downloads mit Profilsprache.
- **Web (Vitest):** Sortierung, Endzeit, Untertitel-Ebene mit Versatz und PiP-Umschaltung,
  `useVolume`, Fokusmodell, Tastenkarte.
- **Vor Schritt 2:** die Tests für Wechsel, Widerruf und Countdown in `PlayerM4.test.tsx`
  (29 Stellen mit Menü-Rollen) auf Rollen und Hooks umschreiben, die nicht an Menüs hängen;
  sie sind das Sicherheitsnetz des Umbaus.
- **Playwright (Regel 8), angepasst:** `player.spec.ts` (Leiste, Panels, Reiter, Hilfe, Fokusmodell,
  keine Fokusfalle), `tracks.spec.ts` und `sidecar.spec.ts` (Panels statt «Spuren»,
  `sidecar.spec.ts:54-59`), `quality.spec.ts`, `chapters.spec.ts`, `versions.spec.ts:78-97`,
  `audiobook.spec.ts` und `podcast.spec.ts` (`VolumeControl`), `offline-playback.spec.ts`;
  Vitest `PlayerVersions.test.tsx`. **Neu:** Seite «Meine Wiedergabe».
- **Storybook-Baselines** für die neuen Bauteile (Weg über CT 190 wie bei PR #53).

### 11.12 Schnitt in Schritte

Jede Stufe ein eigener Zweig und Pull Request, jeder Schritt ein Commit, `task ci` grün,
`CURRENT_STATUS.md` und Changelog nachgeführt.

| Schritt | Inhalt | Tage |
|---|---|---|
| 0 | Frage 19 (F-1 bis F-15), ADR 0021 (2160p, Budget 5, Grenze je Backend), Trickplay in `docs/ideas` | 0,5 |
| **Stufe 1** | **Bedienung (nur Web-App)** | **12,5** |
| 1 | `PlayerM4.test.tsx` von Menüs lösen | 1 |
| 2 | Bauteile `VolumeControl`, `PlayerPanel`, `OptionRow`, `ChapterCard`, `SeekBar`; Stories, Baselines; Audioplayer auf `VolumeControl` | 2 |
| 3 | `VideoPlayer` aufteilen; `PlayerChrome` mit Kopf, Leiste, Fokusmodell, Zeit/Endzeit, Transport, PiP, Tastenkarte, Hilfe | 3 |
| 4 | Safari-Spike Web Audio; Panels Ton, Untertitel, Zahnrad mit Version; Untertitel-Ebene, Versatz, PiP; Nachtmodus, 150 % | 2,5 |
| 5 | Reiter, Pause-Ansicht, Nächste-Folge-Karte, Handy-Layout, Doppeltipp, V-17 (nur clientseitig vorbereitet), V-18 | 1,5 |
| 6 | Offline-Player auf `PlayerChrome` | 2 |
| **Stufe 2** | **Sprache und Qualität (Server)** | **12** |
| 7 | Migration (11.9), sqlc, Modell, Vertrag, `task gen` | 1,5 |
| 8 | Originalsprache in `fill` mit Quellen, Sperre, Verwaltung; Backfill-Jobart | 2 |
| 9 | `trackpick` (Ton, Untertitel, F-11, `subtitleOff`), `visibility`-Methode, Serienmerker, Begründungen; Offline-Downloads | 2,5 |
| 10 | `/me/playback-prefs`, Vorbelegung an drei Stellen, Seite «Meine Wiedergabe», Felder im `ProfileForm` | 1,5 |
| 11 | `original` (a) bis (d), `qualityCap`, Leiter zusammenführen, Qualitätsliste im Panel | 1,5 |
| 12 | 2160p in `transcode` (11.5), Budget; Messung CT 190 | 3 |
| **Stufe 3** | **Kapitel** | **3,5** |
| 13 | `transcode.Chapters`, Titelbereinigung, Ersatzkapitel | 0,5 |
| 14 | `Runner.GrabFrame`, Jobarten, Ablage, Aufräumen, `getChapterImage` | 2,5 |
| 15 | Web: Kapitelbilder in Reiter und Zeitleiste | 0,5 |
| | **Summe** | **≈ 28,5** |

### 11.13 Offene Punkte

- **Safari und Web Audio** (Spike in Schritt 4): offen, ändert höchstens, ob Nachtmodus und 150 %
  in Safari erscheinen.
- Die Streams-Ansicht der Verwaltung zeigt `audioReason` nicht; bei Bedarf später.
- **Stopp durch die Verwaltung erkennen** (gefunden am 2. November 2026 beim Härten der Tests von
  Konzept 12, Firefox-e2e `admin-streams`; besteht seit 1.7.0 und stammt nicht aus Konzept 12).
  - **Befund:** Beendet ein Administrator einen Stream («Beenden» unter `/admin/streams`), erfährt der
    Videoplayer das erst mit seiner nächsten Medienanfrage (401). hls.js lädt einen kurzen Titel ganz;
    ist alles gepuffert, gibt es keine Anfragen mehr, und der Player spielt bis zum Ende weiter.
    Gemessen: 88,4 s von 90 s gepuffert beim Stopp, 40 s lang kein 401. Bei teilweise gefülltem Puffer
    fällt der Stopp nach etwa 1 s auf. Dieselbe Lücke hat `AudioPlayerPage` (Hörbücher, Podcasts).
  - **Anforderung:** Der Player merkt einen Stopp durch die Verwaltung innerhalb von 5 s, auch bei
    vollständig gepuffertem Titel, für Video und Audio. Ohne zusätzliche Last für den Transcoder.
  - **Optionen:**
    1. Statusendpunkt, den der Player während des Abspielens abfragt (kleine Anfrage, kein
       Transcoder-Zugriff).
    2. SSE oder WebSocket: der Server meldet den Stopp sofort, kostet aber eine offene Verbindung je
       Player.
    3. Antwort auf die Fortschrittsmeldung (`POST /items/{id}/progress`, kommt ohnehin alle 10 s)
       trägt ein Stopp-Kennzeichen. Das braucht keine neue Anfrage. Die Erkennung dauert bis zu 10 s
       und erfüllt die Anforderung von 5 s nur, wenn der Takt dafür verkürzt wird oder die Option mit
       einer der anderen verbunden wird.
  - **Entscheid Marco 2. November 2026:** in Konzept 13 lösen. Eine schnelle Zwischenlösung (der Player
    fragt seinen Range-Stream alle 2 s ab, 404 gilt als gültig, 401 als Stopp) wurde gebaut und wieder
    verworfen. Gewählt ist eine saubere Lösung für Video und Audio zugleich; die Option wählt das
    Konzept in der Fassung zur Abnahme.
- **Erster Fokus in Player- und Offline-Dialogen** (gefunden am 3. November 2026 beim Bau von
  Konzept 12, nicht dort behoben).
  - **Befund:** Die Verwaltung setzt den ersten Fokus eines Dialogs über `focusOnMount`
    (`web/src/ui/focusOnMount.ts`). react-arias `autoFocus` wartet nach einem Tastendruck auf
    Enter, bis keine CSS-Transition mehr läuft. Ein Modal bricht die Transition des Fokusrings ab,
    und react-aria fokussiert dann erst nach 500 ms den Dialog selbst statt der Schaltfläche.
  - **Wo noch `autoFocus` steht:** `VideoPlayer.tsx`, `AudioPlayer.tsx`, `OfflinePlayer.tsx`,
    `OfflinePlayerPage.tsx`, `DownloadDialog.tsx`, `OfflinePinDialog.tsx`, `OfflineLibraryPage.tsx`.
  - **Vorschlag:** Beim Umbau der Abspielseite auf `focusOnMount` umstellen und je einen
    Tastaturtest ergänzen, der Enter als Auslöser prüft.

### 11.14 Risiken

| Risiko | Gegenmittel |
|---|---|
| Umbau von `VideoPlayer.tsx` bricht Wechsel, Widerruf, Countdown | Schritt 1 löst die Tests von den Menüs, Sitzungslogik bleibt, wo sie ist |
| Profilsprache erzwingt Remux statt Direct Play (11.3) | gewollt (F-4); gemessen in Schritt 12; Keyframe-Index fehlt → Transcode, sichtbar im Stream-Info |
| Web Audio in Safari stumm | Spike, sonst ausgeblendet |
| 2160p überlastet die iGPU | nur Hardware, nur SDR oder GPU-Tonemapping, Kosten 5, Messung vor Merge |
| `playback_sessions` neu aufbauen (400 Tage Zeilen, Fremdschlüssel) | Vorlage, `foreign_key_check`, Test mit eingefrorenem Stand, Sicherung vor der Migration (Regel 5) |
| TMDB-Backfill dauert | gedrosselt; bis dahin Standardspur der Datei |
| Schwarze Kapitelbilder | Zeitpunkt nach 11.6; sonst zweiter Versuch 10 s später |
| Bedeutungsänderung bei `subtitleTrack: null` trifft Fremd-Clients | Changelog; heute nutzt nur der eigene Web-Client die API für Wiedergabe |

### 11.15 Nachtrag zum Bau (4. November 2026)

Entscheide aus Marcos Interview vom 3. November 2026 und vom Neustart am 4. November 2026
(Frage 19). Wo dieser Nachtrag etwas anderes sagt als 11.1 bis 11.14, gilt der Nachtrag.

- **Neustart ab `main`.** Der Bau beginnt neu ab `main` `c023448`. Der erste Bau auf
  `claude/abspielseite-stufe1` wird nicht weiterverwendet. Je Zweig koordiniert genau eine
  Session, die als einzige `docs/dev/CURRENT_STATUS.md` schreibt und `task ci` startet.
- **Je Stufe ein PR und ein Release:** Stufe 1 → 1.10.0, Stufe 2 → 1.11.0, Stufe 3 → 1.12.0.
  Jede Stufe wird auf CT 190 abgenommen. Merge, Tag und Release nur auf Marcos Wort.
- **Teststufen statt `task ci` je Schritt** (Entscheid vom 4. November 2026,
  `docs/dev/SETUP.md` «Teststufen»). Das ersetzt «`task ci` grün» je Schritt in 11.12:
  - Je Schritt `task check:fast` und die betroffenen Tests.
  - `task ci` einmal vor dem PR.
  - Die GitHub-CI prüft alles.
- **Stopp durch die Verwaltung (11.13), Option 1, neuer Schritt 5b in Stufe 1:**
  - Der Player fragt beim Abspielen alle 4 s eine kleine Statusabfrage ab.
  - Der Server hält gestoppte Sitzungen nur im Speicher (Haken an der Stelle, durch die jeder Stopp
    läuft), ohne DB-Zugriff und ohne Transcoder.
  - Das gilt für Video (`VideoPlayer`) und Audio (`AudioPlayerPage`).
  - Anforderung: Ein Stopp wird in höchstens 5 s erkannt, auch bei ganz gepuffertem Titel.
  - Weil das eine neue Operation ist, hat Stufe 1 den **additiven Vertrag 1.10.0**. Das weicht ab von
    6.1 «kein Vertrag» und von 11.8 («1.9.0 oder später»: Stufe 2 bekommt 1.11.0, Stufe 3 1.12.0).
- **Kein Safari-Spike.** Es gibt kein Apple-Gerät. Der Spike vor Schritt 4 (11.10, 11.13) entfällt.
  Nachtmodus und Verstärkung bis 150 % sind in Safari vorsorglich ausgeblendet. Die Lautstärke bis
  100 % bleibt.
- **Schritt C «CI beschleunigen»** vor Stufe 1: PR #81 (Doku-PRs überspringen, Firefox reduziert,
  Server-Tests in Gruppen), PR #82 (Teststufen, Plätze für schwere Läufe), PR #83 (checkptr aus in
  `modernc.org` beim `-race`-Lauf).
- **Reihenfolge in Stufe 1:**
  - Welle A: Schritte 1, 2 und 5b parallel.
  - Welle B: Schritt 3.
  - Welle C: Schritt 4.
  - Welle D: Schritte 5 und 6.
  - Danach Changelog, `task ci` und PR.
  - Höchstens drei Executor gleichzeitig, je in einem eigenen Worktree und mit eigenem e2e-Port.
- **Englisch als zweite Wahl** (Marcos Entscheid vom 4. November 2026, Review von Schritt 4, m-8):
  Ton- und Untertitel-Panel sortieren zuerst die Profilsprache, dann Englisch, dann «Weitere
  Sprachen (n)». Das ergänzt 11.10 «Profilsprachen zuerst».
- **Bekannte Grenze: Bild-in-Bild in Firefox ohne Untertitel** (Review von Schritt 4, m-3). Das
  eigene Bild-in-Bild von Firefox meldet kein `enterpictureinpicture`. Der `<track>` bleibt darum
  `hidden`, und das Firefox-Fenster zeigt keine Untertitel. In Chromium schaltet der Player wie in
  11.10 beschrieben auf `showing`. Ein verlässliches Signal für Firefox gibt es nicht.
- **Karten in den Reitern** (Schritt 5): «Als Nächstes» und «Besetzung» nutzen das neue Bauteil
  `ui/PlayerCard` (Knopf mit Roving-Tabindex) statt `EpisodeCard` und `CastCard` wie in 11.10.
  Diese sind Links und brauchen die räumliche Navigation, die auf der Player-Route nicht geladen
  ist.
- **«Folge zurück»** (Schritt 5) springt nur innerhalb der Staffel, nicht über die Staffelgrenze.
- PR #79 (Scanner, wiederverwendete Inode) gehört nicht zu Konzept 13 und kommt in ein späteres
  Release.

## 12. Review-Protokoll

### Fassung 2 → 3 (OMC-Kritiker, 1. November 2026)

Urteil: **überarbeiten**, 2 Blocker, 19 Major, 7 Minor; alle Zeilenangaben stimmten. Alle Befunde
sind eingearbeitet, fünf über Marcos Entscheide F-11 bis F-15 (Abschnitt 10.3).

| Befund | Kern | Eingearbeitet in |
|---|---|---|
| B-1 | `playback_sessions.quality` `CHECK` lässt `2160p` nicht zu | 11.5, 11.9 (Tabelle neu aufbauen, Test) |
| B-2 | Server kann «fehlt» und «aus» bei Untertiteln nicht unterscheiden | 11.3 (`subtitleOff`, Wechsel schicken beides) |
| M-1 | «Nur erzwungen» brennt Bild-Spuren ein | F-11, 11.3 |
| M-2 | gewählte Spur ≠ Standard kostet Remux | 11.3 «Kosten», `directFallback` meldet |
| M-3 | Gleichstand nach Kanälen wählt nicht dekodierbare Spuren | 11.3 (Codec zuerst) |
| M-4 | 2160p braucht mehr als `decide.go:46` | 11.5 (Level, Codec-String, Leiter, Budget) |
| M-5 | `original` unscharf | 11.5 (a) bis (d) |
| M-6 | Offline fehlt | F-15, 11.3 `trackpick`, 11.10, Schritt 6 |
| M-7 | Versionswahl fehlte | 11.2 Zahnrad, Schritt 4 |
| M-8 | brechende Tests unvollständig | 11.11, Schritt 1 |
| M-9 | Ersatzkapitel am falschen Ort | 11.6 `transcode.Chapters`, nur Film und Folge |
| M-10 | Jobart unmöglich wie beschrieben | 11.6 (Event- und Backfill-Jobart, Zeitpunkte) |
| M-11 | Sichtbarkeitsloch beim Bild-Endpunkt | 11.6 `vis.MediaFile`, Test «fremde Datei» |
| M-12 | nur Hauptprofil darf Profile ändern | F-13, 11.4 `/me/playback-prefs` |
| M-13 | Vorbelegung offen, drei Anlagestellen | F-12, 11.4 |
| M-14 | Datenzugriff nicht entworfen | 11.3 «Datenzugriff» |
| M-15 | Originalsprache nicht im Metadatenmodell | 11.7 |
| M-16 | Pfeiltasten widersprüchlich | 11.2 Fokusmodell |
| M-17 | Untertitel fehlen im Bild-in-Bild | 11.10 |
| M-18 | gemerkte Stufe erzwingt Transcode | 11.5 `qualityCap` |
| M-19 | Schätzung zu tief | 11.12 (≈ 28,5 Tage) |
| m-1 | `height`, `bitrate` gibt es schon | 11.5, 11.8 |
| m-2 | Sprachcodes (`gsw`, `pt-br`, `cn`, `xx`), `CHECK` | 11.3, 11.9 |
| m-3 | `GrabFrame` als `Runner`-Methode, HDR, veraltete Bilder | 11.6 |
| m-4 | Logo und Genres nicht in `PlaybackInfo` | 11.2 «Daten» |
| m-5 | Serienschritt bei Untertiteln, «Kommentar», SDH bei externen Dateien | 11.3, 11.9 |
| m-6 | Probe-JSON enthält Dateinamen | 11.11 |
| m-7 | `task gen`, Doku-Schritte, Text P-5 | 11.8, 11.12, Text P-5 in 1.2 korrigiert |
