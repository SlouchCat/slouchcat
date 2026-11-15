# Konzept: Player-Bedienung – nächste Folge, Menüs ohne X, Besetzung mit Bildern

- **Status: Fassung 3 (15. November 2026): alle Fragen beantwortet** (Abschnitt 9), mit Nachtrag
  P-4 «Countdown-Karte» vom selben Tag (Abschnitte 2.4 und 5a). Marco hat F-1
  am Mockup beantwortet (Variante B) und F-2 bis F-5 über AskUserQuestion; **F-3 und F-5 gegen die
  Empfehlung:** Es gibt einen eigenen Knopf «Vorherige Folge», und bei Podcasts geht er zur älteren,
  der Folgenknopf zur neueren Folge. Fassung 2 hatte das Kritiker-Review eingearbeitet (REVISE,
  0 Blocker, 3 Major, 8 Minor; Abschnitt 11). **Am 15. November 2026 von Marco
  freigegeben (Frage 30) und am selben Tag ganz gebaut: Release 1.38.0** (Entscheide beim Bau im
  Nachtrag zu Frage 30).
- **Auslöser:** Marco am 15. November 2026 mit zwei Bildern aus dem Player (Knopfreihe, Panel
  «Einstellungen») und einem dritten aus dem Reiter «Besetzung»:
  «Ich möchte noch ein Icon, welches bei den Serien auf die nächste Folge springt. […] Es soll
  überall einheitlich sein. Auch bin ich mit der Einstellungsleiste nicht einverstanden. Ich will
  nicht aufs X klicken müssen um es zu deaktivieren.» Und: «In der Besetzung kommen keine Bilder.
  Die sollen auch da angezeigt werden. Nicht rund.»
- **Verankert gegen:** `main` **`19757472`** («Merge pull request #167»), Vertragsfassung
  **1.37.0** (`api/openapi.yaml:4`), letzte Migration **`00064_item_certifications.sql`**, letzte
  ADR **`0034`**, letzte Frage im Entscheidungsprotokoll **29**.
- **Nummern:** Dieses Papier ist **Konzept 23** und reserviert **Frage 30**. Eine ADR braucht es
  nicht (keine Architekturfrage). Alle drei Stufen ändern den Vertrag additiv (Stufe 1:
  `PlaybackInfo.previousEpisode`, Abschnitt 3.3), keine braucht eine Migration; die
  Versionsnummern vergibt der Bau.
- Pfade ohne Präfix liegen unter `web/src/` oder `server/`. **Nach Namen suchen, nicht nach
  Zeile**; Zeilen beziehen sich auf den Anker.
- Massgeblich bleiben `docs/spec/01-anforderungen-v1.md`, `docs/planning/04-entscheidungen.md`
  und `docs/adr/`. Bei Widerspruch gilt: **Entscheidungsprotokoll > Plan > Spec** (`CLAUDE.md`).
  Dieses Papier ändert zwei Punkte aus Konzept 13 (Abschnitt 8).

> **Dieses Papier ändert keine Zeile Code.** Es beschreibt vier Wünsche, sagt je Wunsch, was der
> Code heute tut, und schlägt eine Lösung vor. Stufe 1 ist vor allem Web-App, dazu liefert der
> Server die vorherige Folge; die Folgenknöpfe im Offline-Player (Stufe 2) und für Podcasts
> (Stufe 3) brauchen weitere Felder vom Server.

## 1. Was Marco will

| Nr. | Wunsch (15. November 2026, sinngemäss) | Abschnitt |
|---|---|---|
| P-1 | Ein eigener Knopf, der bei Serien zur nächsten Folge springt. Der Kapitelknopf bleibt. Überall einheitlich. | 3 |
| P-2 | Das Panel «Einstellungen» soll sich nicht nur über das X schliessen lassen. | 4 |
| P-3 | Im Reiter «Besetzung» Bilder zeigen, nicht rund. | 5 |
| P-4 | Die Karte «Nächste Folge» am Ende verdeckt den Fortschrittsbalken; sie soll etwas höher stehen. | 5a |

## 2. Heute

### 2.1 Knöpfe für Kapitel und Folge (P-1)

- Die Knopfreihe (`ui/PlayerChrome.tsx`, `transport`) hat links **«zurück»** (`previous`),
  −10 s, Play/Pause, +30 s und **«vor»** (`next`), beide mit den Symbolen `previous` und `next`
  aus `ui/Icon.tsx` (Dreieck mit Strich). Am Handy fehlen beide (`step()`: `config && !phone`).
- Die Bedeutung wechselt mit der Lage (`features/player/VideoPlayer.tsx`, `backStep` und
  `forwardStep`, Konzept 13 V-2):
  - **«vor»** springt zum nächsten Kapitel; nach dem letzten Kapitel oder ohne Kapitel startet
    er die **nächste Folge** (Beschriftung wechselt von «Nächstes Kapitel» zu «Nächste Folge»).
  - **«zurück»** springt zum vorherigen Kapitel; in den ersten 3 s des ersten Kapitels
    (`RESTART_CHAPTER_MS`, `features/player/audio.ts`) oder ohne Kapitel startet er die
    **vorherige Folge**, nur innerhalb der Staffel (Konzept 13, Baunotiz Schritt 5).
  - Ein Film ohne Kapitel hat keinen der beiden Knöpfe.
- Taste **N** startet die nächste Folge (`features/player/keymap.ts`, `nextEpisode`), auch
  während des Countdowns am Ende. ⇧←/⇧→ springen Kapitel.
- Die nächste Folge liefert der Server in `PlaybackInfo.nextEpisode`
  (`visibility.Reader.NextEpisode`) als ganzes `ItemRef` (Bilder, Laufzeit, Staffel- und
  Folgennummer): höhere Nummer derselben Staffel, sonst erste Folge einer späteren Staffel;
  Specials folgen nur auf Specials. **Nur für `kind = episode`** (`episodeChain` verlangt eine
  Staffel als Eltern); für Podcast-Folgen ist das Feld leer, und Podcast-Folgen ohne eigene Datei
  laufen über `startEnclosure` (`playback/enclosure.go`), das gar kein `NextEpisode` setzt.
- Der **Offline-Player** (`features/offline/OfflineVideo.tsx`) hat nur die Kapitelknöpfe, keinen
  Folgenwechsel. In IndexedDB liegen nur `downloads`, `profiles`, `events` und `meta`
  (`features/offline/db.ts`); ein Download kennt seinen Nachfolger nicht.
- Der **Audioplayer** (`ui/AudioPlayer.tsx`) hat Kapitel zurück, −15 s, Play, +30 s, Kapitel vor
  mit denselben Symbolen; Podcast-Folgen ohne Kapitel zeigen keine Kapitelknöpfe
  (`e2e/podcast.spec.ts`). Einen Folgenknopf gibt es dort nicht.

**Befund:** Ein Knopf mit zwei Bedeutungen. Wer Kapitel springt, landet nach dem letzten
Kapitel ungewollt in der nächsten Folge; wer die nächste Folge will, muss erst durch alle Kapitel.

### 2.2 Panels (P-2)

- Ton, Untertitel, Zahnrad und Hilfe öffnen je ein `ui/PlayerPanel.tsx`, eines zur selben Zeit
  (`features/player/panels/usePlayerPanels.ts`).
- Das Panel steht rechts **über die ganze Höhe** des Players (`PlayerPanel.module.css`: `top`
  und `bottom` = `--sc-space-4`, Breite bis 24 rem), auch wenn der Inhalt nur fünf Zeilen hat (Bild
  von Marco: rund 80 % leere Fläche).
- Schliessen: das **X** im Kopf, **Escape** oder **Backspace**, ein **zweiter Klick auf den Knopf**
  in der Leiste (`togglePanel`, beim Zahnrad `panel === 'settings' ? closePanel()`). Ein **Klick
  daneben** (aufs Bild, auf die Leiste) **schliesst nicht**. Ein Mausklick aufs Bild schaltet
  Play/Pause, auch im schmalen Fenster; ein Fingertipp zeigt oder verbirgt nur die Bedienung
  (`features/player/usePhoneTap.ts`).
- Unter 768 px und am Handy quer ist das Panel ein Blatt von unten (bis 85 % hoch), ebenfalls
  mit X.
- Die Menüs des Audioplayers (Geschwindigkeit, Kapitel, Schlummerfunktion) sind `MenuButton`
  mit react-aria-`Popover` (`ui/Menu.tsx`) und schliessen schon heute beim Klick daneben.

### 2.3 Besetzung im Player (P-3)

- Der Reiter «Besetzung» (`features/player/tabs/playerTabs.tsx`) zeichnet jede Person mit
  `PlayerCard shape="person"`. Diese Form zeigt **nur den Anfangsbuchstaben in einem Kreis**
  (`ui/PlayerCard.tsx`; `PlayerCard.module.css`: `.card[data-shape='person'] .frame` mit
  `border-radius: var(--sc-radius-full)`). Das Bild der Person (`CastMember.image`) wird gar
  nicht übergeben.
- Die Detailseite zeigt dieselben Personen seit Konzept 21 P-7 als **Hochformat 2:3** mit
  abgerundeten Ecken (`ui/CastCard.tsx`, Breite `size.portrait.sm` 7 rem, ab 768 px
  `size.portrait.md` 9,5 rem); runde Bilder bleiben den Profilen vorbehalten.

**Befund:** Dass der Player `PlayerCard` statt `CastCard` nutzt, ist ein Bauentscheid aus
Konzept 13 (Schritt 5: `CastCard` ist ein Link der räumlichen Navigation, die auf der Player-Route
nicht geladen ist). **Fehler** ist nur, dass `PlayerCard` das Bild nicht zeigt. Alle Reiter teilen
dieselbe Spaltenbreite (`PlayerChrome.module.css`, `.cards`: `minmax(10rem, 14rem)`).

### 2.4 Countdown-Karte am Ende (P-4)

- Die Karte «Nächste Folge» (`VideoPlayer.tsx`, `styles.next`) und der Knopf «Intro/Abspann
  überspringen» (`styles.skip`) stehen mit **festem Abstand** über dem unteren Rand:
  `bottom: calc(var(--sc-space-24) + var(--sc-space-8))` = **128 px**
  (`features/player/Player.module.css`).
- Die untere Bedienung (`PlayerChrome`, `.bottom`) ist seit Konzept 13 höher: Verlauf oben
  (`--sc-space-16`), Zeile mit Titel und Zeit, Zeitleiste, Knopfreihe, Reiterleiste, Abstand unten
  (`--sc-space-6`), zusammen rund 220 px, mit offenem Reiter deutlich mehr. Die Karte liegt darum
  **über Zeitleiste und Zeit** (Bild von Marco vom 15. November 2026: «43:54 / 43:54» scheint
  durch die Karte, der Balken ist unter ihr abgeschnitten).
- Während des Countdowns bleibt die Bedienung sichtbar (`hidden` verlangt `!countdownActive`).

## 3. P-1: Ein Knopf je Bedeutung

### 3.1 Die Knöpfe

| Knopf | Symbol (Variante B, F-1) | tut | Taste |
|---|---|---|---|
| Kapitel zurück | **neu:** Bogen nach links über zwei Marken | an den Anfang des Kapitels; in den ersten 3 s zum vorherigen Kapitel | ⇧← |
| Kapitel vor | **neu:** Bogen nach rechts über zwei Marken | zum nächsten Kapitel | ⇧→ |
| Vorherige Folge | **heutiges** Strich mit Dreieck (`previous`) | startet die vorherige Folge | P |
| Nächste Folge | **heutiges** Dreieck mit Strich (`next`) | startet die nächste Folge | N |

- **Kapitelknöpfe wechseln nie mehr den Titel.** Am ersten Kapitel bleibt «zurück» beim Anfang des
  Kapitels, am letzten ist «vor» ausgegraut (Hinweis «Letztes Kapitel»).
- **Ohne Kapitel** bleiben beide Knöpfe ausgegraut an ihrem Platz (Hinweis «Keine Kapitel»),
  damit nichts verrutscht (F-2). Das gilt auch für einen **Film ohne Kapitel**, der heute gar keine
  Kapitelknöpfe hat, und für Podcast-Folgen ohne Kapitel.
- **«Ausgegraut» heisst `aria-disabled`, nicht `disabled`:** Der Knopf bleibt fokussierbar und in
  der Pfeilreihe der `Toolbar`, tut beim Drücken nichts und trägt seinen Hinweis als Beschreibung
  (`aria-describedby`), so dass Tastatur, Fernbedienung und Bildschirmleser ihn hören. Heute
  überspringt die `Toolbar` gesperrte Knöpfe (`PlayerChrome.tsx`, `enabled`), und ein gesperrter
  react-aria-Knopf zeigt weder Hinweis noch Fokus. Wird der fokussierte Knopf ausgegraut («Kapitel
  vor» ins letzte Kapitel), **bleibt der Fokus auf ihm**; heute fiele er auf `body`.
- Ausnahme: Während eines Wechsels (`transportDisabled`) bleiben die Knöpfe wie heute `disabled`.
- **«Vorherige Folge» bekommt einen eigenen Knopf** (F-3), gespiegelt zum Folgenknopf: ganz links
  in der Reihe, durch einen Trennstrich von den Kapitelknöpfen abgesetzt. Die Reihe heisst damit:
  vorherige Folge | Kapitel zurück · zurückspulen · Play · vorspulen · Kapitel vor | nächste Folge.
  Taste **P** (frei in `keymap.ts`; YouTube nutzt ⇧P, Slouchcat bleibt ohne Umschalttaste wie
  bei N).
- Die Symbole folgen dem verbreiteten Brauch: Dreieck mit Strich heisst bei Netflix und YouTube
  «nächster Titel». Die Kapitel bekommen ein eigenes Zeichen, das an die Marken der Zeitleiste
  erinnert. Beide neuen Symbole kommen nach `ui/Icon.tsx` (`chapterPrevious`, `chapterNext`,
  Strichstärke und Raster wie die übrigen). `step()` in `PlayerChrome` setzt heute `icon={key}`;
  die Schritte bekommen ihr Symbol künftig ausdrücklich, der Folgenknopf einen eigenen
  `RowKey` `nextEpisode`.

### 3.2 Der Folgenknopf

- **Platz:** rechts neben «Kapitel vor», durch einen feinen Trennstrich abgesetzt, vor der
  Lautstärke. Die Reihe heisst damit überall: Kapitel zurück · zurückspulen · Play · vorspulen ·
  Kapitel vor | nächste Folge.
- **Da**, wenn der Titel eine Folge ist (Serienfolge, in Stufe 2 auch Podcast-Folge). Ein Film und
  ein Hörbuch haben keinen (F-4).
- **Ausgegraut**, wenn `nextEpisode` fehlt, mit dem Hinweis «Keine weitere Folge» (nicht
  «Letzte Folge der Serie»: nach dem letzten Special folgt nichts, auch wenn die Serie weitergeht).
- **Über die Staffelgrenze:** nach S01E06 kommt S02E01; die Karte nennt dann «Staffel 2 · S02E01».
- **Vorschaukarte:** Beim Zeigen und beim Fokus (Tastatur, Fernbedienung) erscheint über dem
  Knopf eine Karte: Bild der Folge (`backdropSmall`, sonst `backdrop`), «Nächste Folge», Code und
  Titel, Laufzeit, Taste «N». Alles steht schon im `ItemRef` von `PlaybackInfo.nextEpisode`,
  auch über die Staffelgrenze; keine weitere Abfrage, kein neuer Endpunkt.
- **Name für Bildschirmleser:** «Nächste Folge: S01E05 · Die Brücke».
- **Tastatur:** Der Knopf ist ein Glied der Leiste (Pfeiltasten, kein eigener Tab-Halt). N
  bleibt; die Hilfe (`ShortcutHelp`) nennt N als «Nächste Folge» und ⇧←/⇧→ als «Kapitel».
- **Während des Countdowns** am Ende bleibt alles wie heute; der Knopf tut dasselbe wie
  «Jetzt abspielen».

### 3.2a Der Knopf «Vorherige Folge»

- Gleiche Regeln wie der Folgenknopf, gespiegelt: **da** bei Folgen, **ausgegraut** ohne vorherige
  Folge («Keine frühere Folge»), **Vorschaukarte** mit «Vorherige Folge», Code, Titel, Laufzeit,
  Name für Bildschirmleser «Vorherige Folge: S01E03 · Das Echo».
- **Über die Staffelgrenze:** vor S02E01 kommt die letzte Folge von Staffel 1; Specials nur vor
  Specials. Heute springt «Folge zurück» nur innerhalb der Staffel (Konzept 13, Baunotiz
  Schritt 5) und nimmt die Folge aus der Folgenliste der Staffel im Client. Neu liefert der
  **Server** die vorherige Folge (Regel 12, gleiche Regel wie `NextEpisode`, gespiegelt; siehe 3.3).

### 3.3 Überall gleich

| Player | Kapitelknöpfe | Folgenknopf |
|---|---|---|
| Video, Computer und Tablet | in der Leiste | in der Leiste, mit Vorschaukarte |
| Video, Handy | keine (wie heute; Kapitel über die Zeitleiste) | **in der Mitte**, links und rechts neben −10 · Play · +10, kleiner als diese; keine Karte. Am Handy gibt es keine Reiter, darum braucht es hier beide Knöpfe. |
| Offline-Player | in der Leiste | **Stufe 2:** beide in der Leiste, wenn die Folge auf dem Gerät liegt; sonst ausgegraut, Hinweis «Nicht auf diesem Gerät». In Stufe 1 keine. |
| Audioplayer, Hörbuch | in der Reihe (neues Symbol) | keine (F-4) |
| Audioplayer, Podcast-Folge | in der Reihe; ohne Kapitel ausgegraut (heute ausgeblendet) | **Stufe 3:** in der Reihe; «Vorherige Folge» = nächstältere, «Nächste Folge» = nächstneuere Folge desselben Podcasts (F-5); Karte mit Titel, Datum, Länge |

**Vorherige Folge im Server (Stufe 1):** Neue Abfrage `GetPreviousVisibleEpisode` in
`server/queries/scoped/items.sql` (mit `visible_items`, Regel 2), gespiegelt zu
`GetNextVisibleEpisode`: niedrigere Nummer derselben Staffel (vor dem ersten Teil einer
Mehrfachfolge), sonst die letzte Folge der nächstfrüheren Staffel; Specials nur vor Specials.
`visibility.Reader.PreviousEpisode` und `episodeChain` füllen `previousEpisode`; der Vertrag
bekommt `PlaybackInfo.previousEpisode` (`ItemRef`, optional, `x-visibility: scoped` wie
`nextEpisode`). `ItemDetail` bekommt das Feld nicht (die Detailseite braucht es nicht).

**Offline (Stufe 2):** Der Client darf die Folgenregel nicht selbst nachbauen (Regel 12; Specials,
Staffelgrenze). Darum bekommt `DownloadState` die Felder `nextEpisodeId` und `previousEpisodeId`,
die
`server/internal/offline/sync.go` beim Abgleich füllt (es ruft `vis.NextEpisode` dort schon auf,
`vis.PreviousEpisode` kommt aus Stufe 1). Der Client speichert sie mit dem Download in `StoredDownload` und schaut nur nach, ob der Titel
fertig auf dem Gerät liegt. `OfflineVideo` bekommt dazu einen Weg zum Wechsel auf einen anderen
Download (heute fehlt er).

## 4. P-2: Menüs ohne X

### 4.1 Form

- **Computer und Tablet:** Das Panel öffnet als **Menü direkt über seinem Knopf**, rechts bündig
  mit dem Knopf, **so hoch wie sein Inhalt** (höchstens bis 12 px unter den oberen Rand des
  Players, darüber scrollt es). Breite 20 rem. Gilt für Einstellungen, Ton, Untertitel und Hilfe,
  auch wenn eine Taste es öffnet (T, U, Q, C, ?): Es steht dann über seinem Knopf in der Leiste.
- **Handy und schmale Fenster:** Blatt von unten wie heute, mit einem **Griff** oben statt X. Der
  Griff ist ein beschrifteter Knopf («Schliessen»).
- **Bildschirmleser:** Das X verschwindet nur sichtbar. Ein **visuell verborgener Knopf
  «Schliessen»** bleibt als letztes Element im Panel, damit auch ein Bildschirmleser auf einem
  Touch-Tablet (das ab 768 px die Desktop-Form bekommt) das Panel schliessen kann.

### 4.2 Schliessen

| Handlung | heute | neu |
|---|---|---|
| X im Kopf | schliesst | **entfällt** (Desktop); am Handy ersetzt durch den Griff |
| Zweiter Klick auf den Knopf | schliesst | schliesst |
| Klick aufs Bild | Play/Pause, Panel bleibt | **schliesst nur das Panel**; das Video läuft weiter |
| Klick auf einen anderen Knopf der Leiste | Knopf wirkt, Panel bleibt | **schliesst das Panel und der Knopf wirkt** (ein anderes Panel öffnet sich sofort) |
| Klick auf die Zeitleiste | springt, Panel bleibt | springt, Panel schliesst |
| Escape, Backspace | schliesst (Backspace: eine Ebene zurück) | unverändert |
| Am Handy: Tippen aufs Bild über dem Blatt, Griff antippen oder nach unten ziehen | – | schliesst |

- Der Klick daneben wird im **Capture** auf `pointerdown` erkannt, **anders als react-aria**: Das
  `Popover` von react-aria schluckt den Klick, der Knopf darunter wirkte nicht. Hier wirkt er.
- Ein Klick oder Tipp aufs Bild, der nur schliesst, löst **kein Play/Pause und keinen Doppelklick
  in den Vollbildmodus** aus. Die Sperre gehört in `usePhoneTap`, weil der Offline-Player denselben
  Weg nutzt.
- **Fokus:** Schliesst ein Klick auf einen Knopf oder die Zeitleiste das Panel, bekommt dieses Ziel
  den Fokus. Schliesst ein Klick aufs Bild (das keinen Fokus nehmen kann), geht der Fokus **zum
  Knopf, der das Panel geöffnet hat**; der Player verhindert dafür den Standard des `mousedown`
  aufs Bild, sonst fiele der Fokus nach dem Rückgeben in `PlayerPanel` (Mikrotask) auf `body`.
  Schliesst Escape, geht er wie heute zum Knopf zurück.
- Das Panel bleibt ein **nicht-modaler Dialog** ohne Fokusfalle; Tab führt wie heute hinaus. Ein
  Fokuswechsel per Tab schliesst es nicht (sonst verliert eine Fernbedienung das Menü).
- Eine Auswahl in Ton oder Untertitel schliesst das Panel wie heute; im Zahnrad führt sie zur
  Hauptseite zurück.

### 4.3 Bau

- `PlayerPanel` bekommt die Lage «über dem Knopf» über einen **ausdrücklichen Anker je
  `PanelName`** (Ref auf den Knopf der Leiste, den `usePlayerPanels` hält). `aria-controls` taugt
  nicht: Der Hilfeknopf hat keines, die übrigen setzen es nur, solange das Panel offen ist. Der
  Hilfeknopf bekommt `aria-controls` wie die anderen.
- Dazu das Schliessen bei `pointerdown` ausserhalb von Panel und Öffner. Das X verschwindet
  sichtbar aus dem Kopf; der Zurück-Knopf der Unterseiten bleibt.
- Alle Farben und Abstände aus Tokens; neue Breite `size.panel.menu` 20 rem in
  `packages/design-tokens/tokens/`.

## 5. P-3: Besetzung mit Bildern

- `PlayerCard shape="person"` zeigt das **Bild der Person im Hochformat 2:3** mit
  `--sc-radius-lg`, **links ausgerichtet**, Name und Rolle (höchstens zwei Zeilen) darunter, wie
  `CastCard`. Ohne Bild oder wenn es nicht lädt: der Anfangsbuchstabe auf `surface-3`, **eckig**.
- `playerTabs.tsx` übergibt `imageUrl={imageUrl(member.image)}`.
- Die Reihe der Besetzung bekommt schmalere Spalten: `size.portrait.sm` (7 rem), damit ein Bild
  höchstens rund 10,5 rem hoch ist. Dafür bekommt `PlayerChromeTab.cards` eine Variante
  (`variant: 'portrait'`), die `CardRow` als Attribut setzt und die CSS in eine eigene
  `grid-auto-columns` übersetzt. Die Reihen «Kapitel» und «Als Nächstes» behalten ihre 16:9-
  Karten (10–14 rem).
- Klick auf eine Person öffnet wie heute ihre Seite.

## 5a. P-4: Countdown-Karte über der Bedienung

- Die Karte «Nächste Folge» und «Intro/Abspann überspringen» stehen **immer 12 px über der Zeile
  mit Titel und Zeit**, der obersten sichtbaren Zeile der unteren Bedienung, statt 128 px über dem
  Rand. Zeitleiste, Zeit und Knopfreihe bleiben frei.
- **Messen statt schätzen:** `PlayerChrome` misst die Oberkante dieser Zeile mit einem
  `ResizeObserver` und setzt den Abstand zum unteren Rand des Players als CSS-Variable
  `--player-chrome-top` auf das Element des Players. `.next` und `.skip` rechnen
  `bottom: calc(var(--player-chrome-top, 128px) + var(--sc-space-3))`. Der Ersatzwert gilt nur,
  bis die erste Messung da ist. Ändert sich die Höhe (Reiter, Handy, Fenstergrösse), rückt die
  Karte mit.
- **Offener Reiter:** Startet der Countdown, **schliesst ein offener Reiter**. Sonst reicht der
  Platz über der Bedienung in kleinen Fenstern nicht für die Karte. Öffnet man während des
  Countdowns einen Reiter, rückt die Karte mit nach oben.
- **Handy:** Die gleiche Regel; die Zeile mit Zeit, Bild-in-Bild und Vollbild ist dort die oberste
  Zeile der unteren Bedienung.
- **Offline-Player:** Hat keinen Countdown (kein Folgenwechsel in Stufe 1); `.skip` gilt dort
  gleich.

## 6. Was gleich bleibt

- −10 s und +30 s (Audio −15 s, +30 s), Pfeiltasten ±10 s, Zeitleiste, Countdown am Ende,
  Reiter, Fokusmodell der Leiste (Konzept 13, 11.2).
- Die Menüs des Audioplayers (schliessen schon heute beim Klick daneben).
- Die Regel für die nächste Folge im Server (`NextEpisode`) für Serienfolgen.

## 7. Stufen

| Stufe | Inhalt | Server, Vertrag, Migration |
|---|---|---|
| **1** | P-1 im Video-Player (beide Folgenknöpfe, neue Kapitelsymbole, `aria-disabled`), neue Kapitelsymbole auch im Offline- und Audioplayer; P-2; P-3; P-4 | `GetPreviousVisibleEpisode`, `PlaybackInfo.previousEpisode` (3.3); Minor-Version. Keine Migration. |
| **2** | Folgenknöpfe im Offline-Player (3.3) | `DownloadState.nextEpisodeId` und `previousEpisodeId`, gefüllt in `offline/sync.go`; Minor-Version. Keine Migration (IndexedDB-Schema des Clients bekommt zwei Felder). |
| **3** | Folgenknöpfe für Podcast-Folgen (F-5) | eigene Regel, siehe unten; Minor-Version. Keine Migration. |

**Stufe 3 im Einzelnen** (`episodeChain` passt nicht, er verlangt eine Staffel):

- Neue Regel im Paket `visibility` für `podcast_episode` (F-5): **nächste Folge** = die
  nächstneuere sichtbare Folge desselben Podcasts nach `published_at`, **vorherige Folge** = die
  nächstältere. Ob das Profil sie schon gehört hat, spielt keine Rolle (Marcos Antwort nennt nur
  die Richtung). Ohne `published_at` gilt die Reihenfolge des Feeds (Einfügezeitpunkt), bei
  Gleichstand die Kennung. `NextPodcastEpisodes` (`podcast.go`) beantwortet eine andere Frage
  (neue Folgen der Merkliste) und taugt nicht.
- Aufruf auch im Enclosure-Weg (`playback/enclosure.go`), denn die meisten Podcast-Folgen haben
  keine eigene Datei.
- `ItemDetail.nextEpisode`, `PlaybackInfo.nextEpisode` und `PlaybackInfo.previousEpisode`:
  Beschreibung «same series» im Vertrag um «or the same podcast» erweitern. Die Detailseite (`features/catalog/ItemPage.tsx`, heute nur `kind === 'episode'`)
  bleibt, wie sie ist, oder zeigt die nächste Folge auch beim Podcast; das entscheidet der Bau mit
  Marco.

Hörbuch-Reihen bekommen keinen Folgenknopf (F-4).

## 8. Änderungen an früheren Entscheiden

| Früher | Neu |
|---|---|
| Konzept 13 V-2 und 11.2: «vor» nach dem letzten Kapitel = nächste Folge; «zurück» am Anfang = vorherige Folge | Kapitelknöpfe nur Kapitel; je ein eigener Knopf für die nächste und die vorherige Folge |
| Konzept 13, Baunotiz Schritt 5: «Folge zurück» nur innerhalb der Staffel, aus der Folgenliste im Client | über die Staffelgrenze, vom Server (`PlaybackInfo.previousEpisode`) |
| Konzept 13: Titel ohne Kapitel ohne Kapitelknöpfe; Podcast-Folge ohne Kapitel ohne Kapitelknöpfe | ausgegraut an ihrem Platz (F-2) |
| Konzept 13 11.2: Panels rechts über die ganze Höhe, mit X | Menü über dem Knopf in Inhaltshöhe, ohne X, schliesst beim Klick daneben |

Beides kommt beim Bau als Nachtrag unter **Frage 30** ins Entscheidungsprotokoll.

## 9. Fragen

| Nr. | Frage | Empfehlung | Antwort |
|---|---|---|---|
| F-1 | Symbole: **B** (Dreieck mit Strich = Folge, neues Bogen-Symbol für Kapitel) oder **A** (Dreieck mit Strich = Kapitel, neues Stapel-Symbol für die Folge)? | B | **B** (Marco, 15. November 2026) |
| F-2 | Kapitelknöpfe ohne Kapitel: ausgegraut an ihrem Platz oder ausgeblendet? | ausgegraut (fester Platz, nichts verrutscht) | **ausgegraut** (wie empfohlen) |
| F-3 | Eigener Knopf «Vorherige Folge»? | nein, über den Reiter «Als Nächstes» | **eigener Knopf** (gegen die Empfehlung; 3.1, 3.2a) |
| F-4 | Hörbuch in einer Reihe: derselbe Knopf springt zum nächsten Band? | nein (ein Band ist ein eigenes Buch; die Reihe steht auf der Detailseite) | **nein** (wie empfohlen) |
| F-5 | Podcast: Was ist die «nächste Folge»? | die nächstneuere Folge desselben Podcasts, die noch nicht fertig gehört ist | **«Zurück: ältere Folge, vorwärts: neuere Folge»** (Marco); ohne Filter auf Gehörtes (7) |

Antworten zu F-2 bis F-5 von Marco am 15. November 2026 über AskUserQuestion.

## 10. Prüfung beim Bau

- **Einheiten:** `PlayerChrome.test.tsx` (Knöpfe je Lage, `aria-disabled` mit Beschreibung,
  Folgenknopf mit Karte und Name), `PlayerPanel.test.tsx` (Schliessen bei `pointerdown` ausserhalb,
  nicht beim Öffner, Fokus zum Öffner nach Klick aufs Bild, verborgener Schliessen-Knopf),
  `PlayerCard.test.tsx` (Bild 2:3, Buchstabe ohne Bild, `onError`), `usePhoneTap`-Test (Tipp, der
  nur schliesst, spielt nicht ab), `keymap.test.ts` unverändert.
- **Server:** Test für `GetPreviousVisibleEpisode` neben dem von `GetNextVisibleEpisode`
  (Staffelgrenze, Mehrfachfolge, Specials, unsichtbare Folge übersprungen), Vertragstest
  (`TestContractCoversEveryOperation` unverändert, neues Feld im Schema-Test).
- **P-4:** e2e in `e2e/player.spec.ts`: am Ende einer Folge liegt die Unterkante der Karte über
  der Oberkante der Zeitzeile (Boxen vergleichen, nicht Pixel), auch mit offenem Reiter (der
  schliesst) und in der Handy-Ansicht; dasselbe für «Intro überspringen».
- **Bestehende Tests, die sich ändern:** `PlayerKeys.test.tsx` (die Knöpfe heissen jetzt
  «Nächste Folge: …» und «Vorherige Folge: …»; «Vorherige Folge» springt über die Staffelgrenze), `e2e/podcast.spec.ts` (Kapitelknöpfe ohne
  Kapitel jetzt ausgegraut statt fehlend), `e2e/chapters.spec.ts` und
  `e2e/offline-chapters.spec.ts` («Kapitel vor» am letzten Kapitel startet **keine** Folge mehr,
  der Fokus bleibt auf dem Knopf).
- **Playwright-Tastaturtest** (Regel 8): `e2e/player.spec.ts` um den Folgenknopf in der Leiste
  (Pfeiltasten, Enter, sichtbarer Fokus mit Karte, ausgegraut mit Beschreibung) und um das Menü
  (öffnen per Klick und per Taste, Escape, Klick daneben, kein Fokusverlust).
- **Storybook-Baselines** für `PlayerChrome`, `PlayerPanel`, `PlayerCard`, `AudioPlayer`, `Icon`
  (neue Symbole) werden neu aufgenommen (`task storybook:test`).
- **Texte:** neue Schlüssel (`player.chrome.noChapters`, `player.chrome.lastChapter`,
  `player.chrome.noNextEpisode`, `player.chrome.noPreviousEpisode`, `player.chrome.nextEpisodeNamed`,
  `player.chrome.previousEpisodeNamed`, `player.chrome.nextEpisodeCard`,
  `player.chrome.previousEpisodeCard`, `player.panel.close`, Tastenhilfe für P; Stufe 2
  `player.chrome.notOnDevice`) in `de.json` und `en.json`, die übrigen Sprachen über den
  Übersetzungsweg von Konzept 22 (`locales.test.ts` prüft die Schlüssel streng).
  `player.chrome.previousEpisode` bleibt.

## Anhang A. Mockup

Klickbares Mockup **«Player-Bedienung K23»**: <https://claude.ai/artifact/Mseg7DrKgwrYu9jp7SpgAB>
(privat, nur für Marco). Kopie im Repository unter `docs/planning/mockups/23-player/prototyp.html`.
Farbflächen stehen für echte Bilder; alle Titel und Namen sind erfunden.

Oben umschaltbar: was läuft (Serienfolge, ohne Kapitel, letzte der Staffel, letzte der Serie, Film,
Podcast-Folge, Hörbuch), Gerät (Computer/Tablet, Handy), Symbole (B, A). Zu sehen: Folgenknopf mit
Vorschaukarte und Wechsel über die Staffelgrenze in beide Richtungen, Knopf «Vorherige Folge», Kapitelknöpfe mit neuem Symbol, Menüs über dem
Knopf, die sich beim Klick daneben schliessen (Video läuft weiter), Blatt am Handy, Countdown-Karte
über der Zeitzeile (Umschalter «Folge endet»), Reiter
«Besetzung» mit Bildern im Hochformat. Wo der Text dieses Papiers genauer ist als das Mockup, gilt
der Text.

## 11. Kritiker-Review (Fassung 1)

Review vom 15. November 2026 gegen den Code am Anker: **REVISE**, 0 Blocker, 3 Major, 8 Minor.
Die «Heute»-Angaben stimmten bis auf den Bildklick am schmalen Fenster.

| Nr. | Stufe | Befund | Eingearbeitet in |
|---|---|---|---|
| 1 | Major | Offline kennt den Nachfolger nicht; nachbauen bräche Regel 12 | 2.1, 3.3, 7: eigene Stufe 2 mit `DownloadState.nextEpisodeId` |
| 2 | Major | Gesperrte Knöpfe sind für Tastatur und Bildschirmleser stumm, Fokus fällt auf `body` | 3.1: `aria-disabled` mit Beschreibung, Fokus bleibt |
| 3 | Major | Podcast-Folgen: `episodeChain` und Enclosure-Weg liefern keine nächste Folge | 2.1, 7: Stufe 3 mit eigener Regel |
| 4 | Minor | Vergleich mit react-aria verkehrt | 4.2 |
| 5 | Minor | Anker über `aria-controls` trägt nicht | 4.3: Anker je `PanelName`, Hilfe mit `aria-controls` |
| 6 | Minor | Fokus nach Klick aufs Bild, Sperre in `usePhoneTap` | 4.2 |
| 7 | Minor | `nextEpisode` ist schon ein volles `ItemRef`; «Letzte Folge der Serie» falsch bei Specials | 3.2 |
| 8 | Minor | `PlayerCard` statt `CastCard` ist ein Bauentscheid; Spalten für alle Reiter gleich | 2.3, 5 |
| 9 | Minor | betroffene Tests und Schlüssel fehlten | 10 |
| 10 | Minor | Touch-Tablet mit Bildschirmleser ohne Schliessen | 4.1: verborgener Knopf, Griff beschriftet |
| 11 | Minor | Film ohne Kapitel unklar | 3.1, 8 |

