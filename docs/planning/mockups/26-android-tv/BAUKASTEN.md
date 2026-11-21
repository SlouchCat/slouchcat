# Baukasten des Mockups «TV-App» (Konzept 26)

Klickbares Mockup der nativen Android-TV-App. Ein gezeichneter Fernseher zeigt eine Bildfläche von
1920 × 1080 Punkten, die per CSS-Transform ins Fenster passt. Bedient wird nur mit dem Fokus: Pfeiltasten,
OK, Zurück, Start, Wiedergabe/Pause, über die Tastatur oder die Fernbedienung am Bildschirmrand. Reines
HTML, CSS und JavaScript ohne Build und ohne Fremdbibliothek. Alle Funktionen sind global. Das Mockup
teilt keinen Code mit dem Produkt und keinen mit dem Handy-Mockup, nur dessen Daten und Bilder.

Ausgeliefert wird `docs/planning/mockups/` als Wurzel. `index.html` lädt in dieser Reihenfolge:
`../27-android-handy/js/data.js`, `core.js`, `home.js`, `setup.js`, `browse.js`, `media.js`, `boot.js`, dazu
`app.css`, `browse.css`, `media.css`. Jede Datei trägt `?v=1`; wer eine Datei ändert, zählt ihre Nummer in
`index.html` hoch.

## Regeln

1. **Nur mit dem Fokus bedienbar.** Jedes bedienbare Element trägt `data-f="eindeutige-id"` (eindeutig auf
   der Seite). Ein Mausklick fokussiert und löst aus, aber nichts darf die Maus brauchen. Keine `<input>`
   in der Bildfläche: Texteingabe nur über `keyboard(...)`.
2. **Jeder Knopf tut sichtbar etwas:** Navigation, Zustand ändern, Seitenleiste oder Dialog (`menu`),
   Meldung (`toast`). Kein toter Knopf. Verweise auf Seiten, die es noch nicht gibt, sind erlaubt: Eine
   unbekannte Route zeigt «Diese Seite folgt» mit Zurück.
3. **Echte Bilder, keine Platzhalter:** Bilder nur über `poster(i)`, `backdrop(i)`, `pic(datei)` und
   `art(...)`. Sie kommen aus `../27-android-handy/img/` (Konstante `IMG`). Keine Fremdadressen, keine
   erfundenen Grafiken als Cover.
4. Texte der Oberfläche: Deutsch, Schweizer Schreibweise (ss statt ß), Anführungszeichen «so», du-Form.
   Titel und Inhaltsangaben bleiben, wie der Server sie liefert.
5. **Zehn-Fuss-Gestaltung:** Text mindestens 26 px (`--fs-s`), Fliesstext 28 px (`--fs`), Titel grösser.
   Sicherer Rand rund 5 % (`--safe-x` 96 px, `--safe-y` 54 px). Seiteninhalt beginnt bei `--cx` (rechts der
   Leiste; auf Vollbildseiten gleich `--safe-x`). Wenige Elemente je Bildschirm, grosszügige Abstände.
6. Hoher Kontrast: Text nur mit `--tx`, `--tx2`, `--tx3` (auf hellem Fokusgrund `--bg`). Statusfarben
   (`--ok`, `--warn`, `--danger`, `--accent`) nur für Rahmen, Symbole, Balken. Alles Klappbare startet zu.
7. Farben nur aus den Variablen in `css/app.css` (`:root`). Keine Hex- oder rgba-Werte in eigenen Dateien
   und Skripten; fehlt eine Farbe, im Bericht nennen.
8. Eigene Stile nur in die eigene CSS-Datei, eigenes Skript nur in die eigene JS-Datei. `core.js`,
   `home.js`, `setup.js`, `boot.js`, `index.html` und `app.css` nicht ändern; fehlt dort etwas, in der eigenen
   Datei ergänzen und im Abschlussbericht nennen. Globale Namen der eigenen Datei mit einem Präfix versehen
   (zum Beispiel `br…`, `md…`), weil alle Skripte einen gemeinsamen globalen Raum teilen.
9. Nicht in der TV-App: Verwaltung, Downloads und Offline, Einladungen, Meldekanäle, Kontodaten.
10. Prüfen: `node --check js/<datei>.js`. Keinen Browser steuern; das macht die Hauptsession.

## Daten und Bilder

Die Daten lädt `index.html` aus dem Handy-Mockup (`../27-android-handy/js/data.js`, Objekt `DATA`).
`DATA.T` sind 110 echte Titel; ein Titel wird überall über seinen **Index** `i` angesprochen.

- `T(i)` → `{ i, kind, title, year, min, fsk, author }`; `kind`: `m` Film, `s` Serie, `e` Folge,
  `a` Hörbuch, `p` Podcast. `KIND[kind]` ist die deutsche Bezeichnung, `dur(min)` die Dauer als Text,
  `metaOf(T(i))` die Zeile «Jahr · Art · Dauer · ab 12».
- `D(i)` → Einzelheiten oder `null`. Es gibt sie für die Filme **0, 1, 3, 4** und die Serien **2, 37, 38**:
  `o` Inhaltsangabe, `g` Genres, `t` Leitsatz, `c` Besetzung `[[Name, Rolle, PersonenId], …]`,
  `l` (1 = Titel-Logo), `f` Datei `[Videocodec, Höhe, Toncodec, GB, Container, Fassungen]`, bei Serien
  `s` Staffeln `[[Nummer, Folgen, gesehen], …]`, `e` Folgen der ersten Staffel
  `[[FolgenId, Nummer, Titel, Minuten, Inhaltsangabe], …]`, `n` nächste Folge `[Staffel, Folge, Titel]`.
  Für alle anderen Titel eine schlichte Detailseite aus `T(i)` zeigen.
- `row(key)` → Indizes einer Reihe: `spot`, `cont`, `rel`, `rec`, `picks`, `watch`, `wish`, `lib1` (Filme),
  `lib6` (Serien), `lib4` (Podcasts), `films`, `books`, `pods`, `docs`. `DATA.R.cont` hat `[i, Prozent]`.
- `LIBS` (in `core.js`): Bibliotheken mit `key` (`films`, `series`, `docs`, `books`, `podcasts`), `name`,
  `icon`, `count`, `ids()`.
- `DATA.radio` → `[[id, Name, Land, Stil], …]`, `DATA.col` → Sammlungen `[[id, Name, Anzahl], …]`.

Bilddateien in `../27-android-handy/img/` (nicht im Repo, echte Cover der Testbibliothek):

| Datei | Inhalt |
|---|---|
| `p-<i>.webp` | Poster 2:3 (Hörbücher und Podcasts quadratisch; bei Folgen ein Standbild 16:9) |
| `b-<i>.webp` | Hintergrundbild 16:9 (nicht für Hörbücher, Podcasts, Folgen) |
| `l-<i>.webp` | Titel-Logo, wo `D(i).l` gesetzt ist, dazu für alle Titel aus `row('spot')` |
| `e-<FolgenId>.webp` | Standbild einer Folge 16:9 |
| `c-<PersonenId>.webp` | Porträt 2:3; fehlt bei einzelnen Personen |
| `r-<SenderId>.webp` | Senderlogo (39211, 38967) |
| `k-<SammlungsId>.webp` | Poster einer Sammlung |

`pic(datei, alt)` zeigt bei einem fehlenden Bild ein Feld mit dem Namen. `artOf(i)` liefert die Datei für
die Hintergrundebene (`b-<i>` bei Film und Serie, `p-<i>` bei einer Folge, sonst `null`). Für Titel
ausserhalb der Bibliothek (Wünsche) dienen Titel aus `row('films')` und `row('docs')` als Stellvertreter.

## Seiten anmelden

```js
route('/library/:key', (params, query) => ({ nav: 'libs', html: `<div class="page">…</div>` }));
PAGES.push(['#/library/films', 'Bibliothek Filme']);   // Eintrag im Sprungmenü der Mockup-Leiste
```

Eine Route liefert ein Objekt mit diesen Feldern (alle ausser `html` freiwillig):

| Feld | Bedeutung |
|---|---|
| `html` | Inhalt der Seite. Seiten mit Leiste nutzen `<div class="page">`, Reihen stehen ohne `.page` |
| `nav` | markiert den Eintrag der Leiste: `start`, `search`, `libs`, `wishes`, `settings`, `profiles` |
| `full` | `true`: ohne Leiste und ohne Uhr (Einrichten, Profilwahl, Player); `--cx` wird `--safe-x` |
| `cls` | Klasse für `#view` (zum Beispiel `home`) |
| `after(view)` | läuft nach dem Zeichnen, vor dem ersten Fokus |
| `onFocus(el)` | läuft bei jedem Fokuswechsel innerhalb der Seite |
| `back()` | Zurück-Taste zuerst an die Seite; `true` heisst erledigt (zum Beispiel Schritt zurück, Player-Bedienung ausblenden) |
| `key(name, ev)` | jede Taste zuerst an die Seite (`up`, `down`, `left`, `right`, `ok`, `back`, `play`, `home`, bei Tastatur auch `null` mit `ev`); `true` heisst erledigt. Für den Player: Links/Rechts springen 10 s |
| `root` | `true`: Zurück verlässt die App statt zurückzugehen (Startseite immer) |

Navigieren: `<a href="#/pfad" data-f="…">` oder `go('#/pfad')`, ersetzend `go('#/pfad', { replace: true })`.
Zurück: `data-act="back"` (wirkt wie die Taste). Neu zeichnen: `render()`; nach einer Zustandsänderung
`commit()` (speichern und zeichnen). Beim Neuzeichnen derselben Seite bleiben Fokus und Bildlauf.

## Fokus-Engine (`core.js`)

| Attribut | Wirkung |
|---|---|
| `data-f="id"` | macht das Element fokussierbar; die Id muss auf der Seite eindeutig sein, damit der Fokus Neuzeichnen und Zurück überlebt |
| `data-default` | Startfokus der Seite, eines Menüs oder einer Reihe mit `data-enter` (das erste sichtbare gewinnt) |
| `data-row="name"` | Gruppe, meist eine waagrechte Reihe (`.trow`). Merkt sich die zuletzt fokussierte Karte und kehrt beim Betreten von oben oder unten dorthin zurück. Läuft sie waagrecht über, rückt die fokussierte Karte an den Reihenanfang |
| `data-enter` | auf einer `data-row`: beim Betreten immer `[data-default]` (die Leiste nutzt das) |
| `data-top` | Fokus darin rollt die Seite ganz nach oben (Spotlight) |
| `data-scroll` | Rollbereich, in dem der Fokus sichtbar gehalten wird (`#view`, jedes Menü). Eine `section.rsec` rückt mit ihrem Titel an den oberen Rand |
| `data-act="name"` | OK ruft `ACT.name(dataset, element)`; sonst folgt OK dem `href` |
| `data-tt`, `data-tm` | Titel und Metazeile; bei Fokus erscheinen sie in der `.rinfo` derselben `.rsec` (setzt `card()` selbst) |
| `aria-disabled="true"` | sichtbar, aber nicht fokussierbar und ohne Wirkung |

- Pfeiltasten suchen geometrisch das nächste Element in der Richtung; was in derselben Zeile oder Spalte
  liegt, hat klaren Vorrang. Am Rand wippt das Element kurz. Von der Leiste nach rechts geht es zurück zum
  zuletzt fokussierten Inhalt.
- Die Leiste links zeigt Symbole und klappt mit Beschriftung auf, sobald sie den Fokus hat.
- Zurück: schliesst zuerst das oberste Menü, dann fragt es `view.back()`, dann geht es in der Geschichte
  zurück. Auf der Startseite springt der Fokus zuerst nach oben, dann folgt «Zum Verlassen nochmals Zurück
  drücken», ein zweites Zurück innert 3 s beendet die App (OK öffnet sie wieder).
- Start (Taste Home) geht zur Startseite. Wiedergabe/Pause auf einer Karte mit `data-i` startet bei Film,
  Serie und Folge `#/play/<i>`, sonst öffnet es die Seite des Titels.
- Kommt man mit Zurück auf eine Seite, stehen Fokus, Bildlauf und Reihenpositionen wie zuvor. Vorwärts
  geöffnet beginnt eine Seite frisch.
- Helfer: `focus(el)`, `focusTo('data-f-id')` (im offenen Menü, sonst auf der Seite), `F.el` (fokussiertes
  Element), `press(name)` (eine Taste auslösen), `move(dir)`, `activate(el)`.
- Die Tastatur: Pfeiltasten, Enter = OK, Backspace oder Esc = Zurück, Leertaste = Wiedergabe/Pause.

## Zustand

- Dauerhaft `S` unter dem Schlüssel `sc26-tv-mock-v1` (siehe `seed()` in `core.js`): `S.setup`, `S.server`,
  `S.account`, `S.secure`, `S.tvName`, `S.profile`, `S.profiles` (`{name, color, main, pin, fsk}`; `color`
  ist ein Variablenname `av1` … `av6`), `S.watch`, `S.seen`, `S.wishes`, `S.prefs` (`quality`, `audio`,
  `subs`, `autoplay`, `surround`), `S.notes`, `S.playing`. Eigene Felder immer mit Vorgabe lesen
  (`S.x ?? vorgabe`), weil ältere gespeicherte Stände sie nicht haben.
- Flüchtig `ui` (`ui.open`, `ui.draft`, `ui.q`, `ui.tab`), frei für eigene Felder.
- `me()` ist das aktive Profil. Flüchtigen Zustand der eigenen Datei bei «Neu starten» leeren:
  `RESETS.push(() => { … })`.

## Bauteile

- **Aktionen** (`ACT`): eingebaut `back`, `go` (`data-to`), `close`, `toast` (`data-msg`), `toggle`
  (`data-scope="S|ui"`, `data-path`), `set` (`data-path="S.prefs.x"`, `data-value`, `data-num`), `watch` und
  `seen` (`data-i`), `libs` (Menü der Bibliotheken).
- **Menüs und Dialoge:** `menu(html, { dialog, cls, onKey(name, ev), onClose })` öffnet eine Seitenleiste
  von rechts (Vorgabe) oder mit `dialog: true` einen Dialog in der Mitte; der Fokus bleibt darin gefangen.
  `redrawMenu(html)` ersetzt den Inhalt des obersten Menüs, `closeMenu()` schliesst es und gibt den Fokus
  zurück. Aufbau: `<h2>Titel</h2><div class="mlist"><button class="mi" data-f="…"><span class="mi-ic">…</span>
  <span class="mi-tx"><b>…</b><small>…</small></span></button></div>`, unten `<div class="acts">`.
  Menüs dürfen gestapelt werden.
- **Bildschirmtastatur:** `keyboard({ value, label, type, placeholder, onDone(wert), onCancel })`; `type` ist
  `text`, `url`, `email` oder `password` (maskiert, mit «anzeigen»). Buchstaben (QWERTZ), Ziffern,
  `- . _ @ : / &`, Umschalten, Sonderzeichen (`#+=`, auch ä ö ü é è), zwei Schnelltasten je Art
  (`https://`, `.ch`, `.com`), Leerzeichen, Löschen, Fertig. Tippen auf der Computertastatur fügt direkt ein;
  Enter nach Tippen heisst Fertig, Zurück schliesst ohne Änderung.
- **Meldung:** `toast('Text')`.
- **Hintergrundebene:** `art(datei, 'hero' | 'dim')` blendet ein Bild hinter der Seite über, `art(null)`
  blendet aus. Beim Seitenwechsel leert sie sich selbst.
- **Karten und Reihen:** `card(i, { wide, sq, prog, sub, tag, href, row })`, `hrow(titel, ids, opts, alleLink,
  schlüssel)` (Reihe mit Kachel «Alle anzeigen» und Infozeile), Raster `.tgrid`.
- **Knöpfe:** `.tb` (`primary`, `ghost`, `sm`), Feld `.tf` mit `.tf-lab` (öffnet die Tastatur), Schalter
  `.tsw` mit `role="switch" aria-checked` und `<span class="sw">`, Hinweis `.tnote` (`warn`, `bad`), `.badge`
  (`acc`, `ok`, `warn`), `.mockhint` und `.mockvar` (gelb gestrichelt, nur im Mockup), `.spin`.
- **Weiteres:** `icon(name, grösse)` (Namen siehe `ICONS`), `fsk(n)`, `avatar(profil, grösse)`, `wordmark()`,
  `hostOf(adresse)`, `slug(text)`, `esc(text)`, `$`, `$$`.
- **Mockup-Leiste:** `PAGES` füllt das Sprungmenü; «Neu starten» setzt alles zurück; «Fernbedienung»
  blendet die Fernbedienung ein und aus.

## Wem welche Seite gehört

| Datei | Routen |
|---|---|
| `home.js` | `/` |
| `setup.js` | `/setup`, `/setup/phone`, `/setup/hand`, `/profiles` |
| `browse.js` | `/library/:key`, `/item/:i`, `/person/:id`, `/search`, `/list/:key`, `/watchlist`, `/watched`, `/collections`, `/collection/:id`, `/play/:i` |
| `media.js` | `/book/:i`, `/podcast/:i`, `/listen/:i`, `/radio`, `/wishes`, `/wish/:i`, `/notifications`, `/settings`, `/settings/profiles`, `/settings/playback` |

Seiten verweisen frei aufeinander; die Adressen dieser Tabelle gelten. `/list/:key` nimmt die Schlüssel
`rel`, `rec`, `picks` der Startseite.
