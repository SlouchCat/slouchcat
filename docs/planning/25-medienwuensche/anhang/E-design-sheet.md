# Design-Blatt Slouchcat (gemessen an der laufenden App 1.40.x auf CT 190, 16.11.2026)

Quelle: berechnete Stile der echten App (Desktop 1440 px, Handy 390 px) und `packages/design-tokens/tokens/color.json`.
Zweck: Die Mockups sollen wie die echte App aussehen. Klassennamen unten sind die lokalen Namen der CSS-Module.

## Farben (aktuelle Tokens, dunkel; es gibt nur das dunkle Thema)

| Token | Wert |
|---|---|
| bg-base | #0b0c10 |
| bg-elevated (Panels, Seitenleiste, Karten der Verwaltung) | #171a22 |
| surface-1 (Karten, Felder, Posterrahmen) | #1b1e27 |
| surface-2 (Tabellenkopf, Zähler) | #20242f |
| surface-3 (sekundäre Knöpfe, aktive Einträge) | #2b3040 |
| text-primary | #f3f3f5 |
| text-secondary | #d9dce4 |
| text-muted | #bcc1cd |
| border-subtle (Trenner) | rgba(255,255,255,.22) |
| border-frame (Rahmen von Flächen) | rgba(255,255,255,.36) |
| border-control (Rahmen von Knöpfen/Feldern) | rgba(255,255,255,.45) |
| danger / success / warning | #f2777a / #5fcf8f / #e8b65a |
| accent ember (Standardakzent, Profil «marco») | #e8794a, Text darauf #1c0d06 |
| scrim-strong / overlay | rgba(11,12,16,.8) / rgba(11,12,16,.85) |

Schrift: system-ui-Stapel. Grössen 12 / 14 / 16 / 20 / 24 / 32 / 44 / 60 px. Radien 4 / 8 / 12 / 20 / 9999.
Abstände 4-8-12-16-20-24-32-40-48-64. Fokusring 3 px #f3f3f5, Versatz 2 px.

## Rahmen der Zuschauer-App

- Kopfzeile fest, 64 px hoch, Grund rgba(11,12,16,.96). Links Logo (32 px, Akzent) + «Slouchcat» 20/700 (Handy: nur Logo,
  davor Menü-Knopf 44 px). Rechts: «Suche» und «Benachrichtigungen» als Pillen (Icon 20 + Text 14/500, Handy nur Icon 36 px),
  Profil-Pille (Avatar 32 px rund in Akzentfarbe mit Initiale + Name 14/400).
- Seitenleiste ab 768 px: 240 px breit, Grund #171a22, Innenabstand 12 px. Oben Profilkarte (r=12, Grund #1b1e27:
  Avatar, Name 14/600, Rolle 12/400 «Server-Admin», Pille «Wechseln»). Liste: Start, Suche, Merkliste, Gesehen (Einträge
  44 px hoch, r=8, Icon 16 + Text 14/500 #d9dce4; aktiv: Grund #2b3040, Text 14/600 #f3f3f5, links 3 px Akzentstrich).
  Gruppe «MEINE MEDIEN» (Band 34 px, r=8, Grund Akzent 16 %, Text 12/700 Akzent, Grossbuchstaben): Filme 1840, Serien 505,
  User-Empfehlungen 3, Podcast 100, Hörbücher 89, Anime 114, Dokus 63, Radio «2 Sender», Sammlungen 337 (Zähler-Pille
  12/500 #bcc1cd auf #1b1e27). Gruppe «KONTO»: Einstellungen (/settings/playback), Profile (/settings/profiles),
  [Verwaltung (/admin) nur für Administratoren]. Unten: Abmelden, «Seitenleiste ausblenden».
- Handy (< 768 px): keine Seitenleiste, Menü-Knopf öffnet sie als Schublade. Seitenrand 16 px statt 64 px.
- Inhalt: Seitenrand 64 px (Desktop). Inhaltsseiten (`main.page`): padding 104px 64px 64px, gap 32; h1 60/700
  (Handy 44), darunter gedämpfte Zeile 16/400 #bcc1cd, Aktionszeile mit Geist-Knöpfen («Zurück» mit Pfeil).

## Spotlight (Startseite, `section.hero`)

- Überschrift «Spotlight ›» (20/600, Link auf /lists/spotlight).
- Bühne `stage`: r=20, Desktop 1057x540, Handy 358x506. Bild füllt die Bühne (cover), darüber Verlauf von unten
  (rgba(11,12,16,.8) → .5 bei 50 % → 0). Inhalt unten links: padding 32px 68px (Handy 16), Titel 60/700, lh 66
  (Handy 32/700) oder Logo-Bild; Metazeile 14/500 #d9dce4 («2026 · 1 h 41 min»); Aktionen gap 12:
  «Abspielen» primär lg (48 hoch, Pille, Grund #f3f3f5, Text #0b0c10 16/600, Icon 20),
  «Details» und «Trailer» sekundär lg (Grund #2b3040, Rahmen 1px rgba(255,255,255,.45), Text #f3f3f5).
  Ecke oben rechts: Positions-Pille «3 von 20» (12/400, Grund rgba(11,12,16,.8), padding 4 12) + Pause-Knopf rund 44.
  Pfeile links/rechts rund 44 (sekundär), vertikal mittig (auf dem Handy keine Pfeile).
- Darunter Posterreihe «Spotlight-Titel» (alle 20 Titel, waagrecht scrollbar, padding 16px 64px 24px, gap 16):
  Poster 200x300 (Handy 109x163), r=12; der aktive Titel normal, die anderen `dimmed`; darunter Titel 14/500 und Jahr 12/400
  #bcc1cd. Auf dem Poster: oben links Bleistift (nur Admin), oben rechts «Gesehen»-Haken (32 rund), unten rechts
  «Merken» (32 rund, Lesezeichen; gemerkt = in Akzentfarbe gefüllt).

## Reihen (`section.row`)

Kopf 44 hoch: Titel 20/600 mit «›» (Link auf Liste), rechts Blätterknöpfe (Geist, rund 44). Scroller padding 16px 64px 24px,
gap 16; Posterkarte 200x300 + Meta (Titel 14/500, Zeile 12/400). Reihen der Startseite z. B. «User-Empfehlungen»,
«Weiterschauen», «Neu erschienen», Trakt-Reihen, Sammlungen.

## Detailseite (`/items/<id>`, Film und Serie)

- Backdrop oben 720 hoch (Handy 416) mit Verlauf nach unten in bg-base und Farbton-Schleier; Zurück-Knopf rund 44 oben links.
- Layout padding-top 316 (Handy 300), Spalten gap 48: Poster 288x432 r=12 (auf dem Handy kein Poster), rechts Info (gap 20):
  Logo-Bild (max 400x120) oder Titel, «Originaltitel: …» 16/400, Metazeile 14/500 («2025 · 47 min»), Genres 14/400 #bcc1cd,
  Tagline 20/400, Hauptknopf primär lg (57 hoch bei zweizeilig: «Weiterschauen» + Detailzeile 12/500 + Fortschrittsstrich),
  runde Aktionen (`roundActions`, gap 8): je Kreis 44 (Grund #2b3040, Rahmen .45) + Beschriftung 12/500 darunter:
  Trailer, Von vorne, Merken, Gesehen, Empfehlen, Löschen (Löschen nur mit Recht). Dann Beschreibung 16/400 #d9dce4 (max ~590 breit).
- Serie: Abschnitt «STAFFELN» (12/600 #bcc1cd, Grossbuchstaben): Staffelwand aus Posterkarten 199x298 («Staffel 1»,
  «2025 · 10 Folgen»), gewählte Staffel `active`, andere `dimmed`. Darunter Staffelkopf (20/600, Zeile, Text) und Folgenliste
  (`episodeRows`, Trenner .22): Zeile 48 hoch: Nummer 20/600 #bcc1cd, Titel, Kurztext 14/400, Dauer 12/400, rechts Gesehen-Haken 44;
  offene Zeile: Grund #1b1e27, r=8, Bild 352x198 r=8, Code «S01E01», Text, Aktionen.
- Danach Reihen «Besetzung» (Personenkarten 152x228 r=12), «Ähnliche» usw.

## Suche (`/search`)

h1 «Suche»; Suchfeld Pille 768x52 (Grund #1b1e27, Rahmen .45, Platzhalter «Titel, Personen, Sammlungen»); Filter: Genre-Chips
(Pillen 44 hoch, 14/500; gewählt = Grund #f3f3f5, Text #0b0c10; sonst Grund #1b1e27, Rahmen .45), Jahr von/bis,
«Altersfreigabe höchstens», Medientyp (Filme, Serien, …). Zeile «8 Titel gefunden» 16/400 #bcc1cd. Ergebnisse in Abschnitten
(«Titel», «Personen», «Sammlungen»; h2 20/600), Raster aus Posterkarten 199 breit, gap 24 16.

## Benachrichtigungen (`/notifications`) und Kanäle (`/settings/notifications`)

- Liste: h1, «0 ungelesen», Aktionen «Zurück», «Kanäle einstellen» (Geist-Knöpfe). Leerzustand als Hinweis-Kasten (r=12,
  Grund #1b1e27, padding 16, Icon + Titel 16/600 + Text 16/400). Einträge: `InboxEntry` (Karte mit Titel, Text, Zeit, Link).
- Einstellungen (`/settings/*`, kein Verwaltungs-Look): h1 60/700 + Hilfe-Knopf; Panels r=20, Grund #171a22, padding 24, gap 20,
  h2 24/600; Formularfelder; primärer Knopf Pille 44 «Speichern»; sekundäre Knöpfe Pille.
  Kanäle: je Kanal ein klappbares Band-Panel (r=20, Rahmen .36, Band-Grund Akzent 12 %, Titel 24/600 mit Chevron + Icon;
  rechts Zusammenfassung 12/400 und Chip): E-Mail (Adresse, Chip «Aus»/«Aktiv»), Telegram («Nicht eingerichtet»),
  Web-Push («0 Geräte», «Aktiv»). Chips: r=4, 24 hoch, 12/600; success = Grund #5fcf8f Text #0b0c10; outline = Rahmen #d9dce4.

## Verwaltung (`/admin`, kompakte Dichte, eigener Look)

- Kopfzeile wie App, aber: Menü-Knopf, Logo mit Unterzeile «Control Center» (10.5/700), Brotkrumen «Verwaltung › Seite» (12/500),
  rechts Suchknopf «Bereich suchen  Strg K» (32 hoch, r=8, Grund #1b1e27, Rahmen .45), Benachrichtigungen, Profil.
- Seitenleiste 240 px, Grund #171a22, rechts Trenner .22: «Zurück zur App», Überschrift «VERWALTUNG» (10.5/700 #bcc1cd),
  Einträge 44 hoch r=8, 13/500 #d9dce4 (aktiv: Grund Akzent 12 %, 13/600 #f3f3f5): Übersicht, Medien (Zähler-Pille warning),
  Nutzer & Geräte, Startseite, Automatisierung, System. Unten Profil (Initiale in Akzent-Kachel r=8, Name, Rolle).
- Inhalt: padding 96px 32px 64px, gap 24. Zuerst Bereichsreiter (`AreaTabs`): Leiste r=12, Grund #1b1e27, Rahmen .36, padding 4,
  Reiter 32 hoch r=8, 12/600 (aktiv Grund #2b3040), optional Zähler («7 / 8»). Bereiche und Reiter:
  Medien: Bibliotheken, Metadaten, Pflege, …; Nutzer & Geräte: Konten, Geräte; Startseite: Startseite (home-rows), Spotlight, …;
  Automatisierung: Hintergrundjobs, Integrationen, Benachrichtigungen; System: Statistik, Server, Sicherung, …
- Seitenkopf (`framed`): Eyebrow 11/700 Akzent Grossbuchstaben («AUTOMATISIERUNG / INTEGRATIONEN»), h1 30/700, Intro 13/400
  #bcc1cd (zweizeilig gekürzt, «Mehr»), rechts Aktionen (Knöpfe 32 hoch, r=8, 14/600; primär = Akzentgrund #e8794a, Text #1c0d06).
- Karten (`panel`): r=12, Grund #171a22, Rahmen 1px .36, padding 16, gap 12; Kartenkopf: Icon-Kachel 32 r=8 Grund #2b3040 + h2 16/600.
- Kennzahl-Kacheln (`StatTile`): r=12, Rahmen .36, Wert 32/700 (kompakt 20/700), Label 14/400 (kompakt 12).
- Integrationen: Zeile «7 von 8 eingerichtet», Raster aus Kacheln (3 Spalten, gap 12; Kachel r=12, padding 16: Icon-Kachel, Titel-Link
  16/600, Text 14/500 #d9dce4, Meta/Chip, Statusliste mit Punkt 8 px (success grün, sonst grau), Pfeil rechts).
  Kacheln heute: Schlüssel der Anbieter, Webhooks von Radarr und Sonarr, E-Mail-Server, Telegram-Bot, Web-Push, … (8 Stück).
  Detail (`?item=webhooks`): Link «‹ Alle Integrationen», Band-Panel (Band 48 hoch, Titel 16/600, rechts Meta 12/400),
  Körper padding 16 gap 12: Statuszeile, Erklärtext 14/400, Einstellzeilen (`SettingRow`: links Label 14/500 + Hilfe 12/400 #bcc1cd,
  rechts Steuerung), Felder 32 hoch r=8 (Grund #1b1e27, Rahmen .45), Segmentwahl (Pille, Grund #1b1e27, aktives Segment #2b3040),
  Tabelle (`DataTable`: Rahmen .36 r=12, Kopf Grund #20242f 12/600, Zeilen 44 hoch 14/400, erste Spalte Aufklapp-Knopf mit Chevron,
  Chips r=4, rechts Aktionen; gefährliche Aktionen als ruhiger roter Text-Knopf «Token erneuern», «Löschen»).
- Konten: Reiter «Konten | Geräte», Kopf mit «Konto einladen», Kennzahl-Streifen (Konten, Administratoren), Unterreiter als Pillen
  («Konten 1», «Einladungen 1 offen»), Tabelle (E-Mail aufklappbar, Rolle + Chip «Besitzer», Altersstufe «FSK 18», Bibliotheken, …).
- Chips der Verwaltung: success = Text #5fcf8f auf 16 % Tönung mit Rahmen 70 %; warning/danger gleich; info = Grund #20242f Rahmen .36;
  outline = Rahmen #d9dce4.
- Regeln von Marco (16.11.2026): Alles Klappbare startet zugeklappt und zeigt seine Zusammenfassung im Kopf; Rahmen sichtbar
  (≥ 3:1), gedämpfter Text ≥ 8:1; Anlegen und Ändern im selben Formular; in der Zuschauer-App kein Rahmen um Bilder.

## Knöpfe (alle Bereiche)

`button`: Pille (Verwaltung r=8), Höhe 44 (lg 48, sm 32, Verwaltung 32), padding 0 20 (lg 0 24), 16/600 (Verwaltung 14/600), gap 8.
primary: Grund #f3f3f5, Text #0b0c10 (Verwaltung: Akzent). secondary: Grund #2b3040, Rahmen 1px rgba(255,255,255,.45).
ghost: ohne Grund, Text #d9dce4. iconOnly: rund 44 (sm 32). overlay: Grund rgba(11,12,16,.8). dangerQuiet: Text #f2777a ohne Grund.
