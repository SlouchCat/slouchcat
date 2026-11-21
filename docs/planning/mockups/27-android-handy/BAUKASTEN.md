# Baukasten des Mockups «Handy-App» (Konzept 27)

Klickbares Mockup der nativen Android-App in einem Geräterahmen (Handy 412 × 915, Tablet 1280 × 800,
Hoch- und Querformat). Reines HTML, CSS und JavaScript ohne Build und ohne Fremdbibliothek. Alle
Funktionen sind global. Das Mockup teilt keinen Code mit dem Produkt.

`index.html` lädt in dieser Reihenfolge: `data.js`, `core.js`, `home.js`, `setup.js`, `browse.js`,
`media.js`, `boot.js`. Die Bilder liegen in `img/` und stehen **nicht** im Repo (`.gitignore`): Es sind
echte Cover aus der Testbibliothek.

## Regeln

1. **Jeder Knopf und jeder Link tut sichtbar etwas:** Navigation, Zustand ändern, Blatt von unten
   (`sheet`), Meldung (`toast`). Kein toter Knopf.
2. **Echte Bilder, keine Platzhalter:** Bilder nur über die Helfer `poster(i)`, `backdrop(i)`, `pic(datei)`
   aus `img/`. Keine Fremdadressen, keine Netzaufrufe, keine erfundenen Grafiken als Cover.
3. Texte der Oberfläche: Deutsch, Schweizer Schreibweise (ss statt ß), Anführungszeichen «so», du-Form.
   Titel und Inhaltsangaben bleiben, wie der Server sie liefert.
4. Wirkt wie eine Android-App (Material 3, dunkel): Leiste oben (`bar`), Blätter von unten statt Dialogen
   in der Mitte, Tippflächen mindestens 48 px, Listen mit `li(...)`, Schalter mit `sw(...)`.
5. Alles Klappbare startet zugeklappt. Hoher Kontrast: Text nur mit `--tx`, `--tx2`, `--tx3`, Rahmen mit
   `--frame` oder `--ctl`.
6. Zwei Breiten je Seite über Container-Abfragen: `@container app (min-width: 700px) { … }` (nicht
   `@media`). Schmal zuerst. Im Tablet-Querformat ist der Inhalt rund 1180 px breit: Raster mit mehr
   Spalten, Detailseiten zweispaltig, Formulare höchstens 560 px breit und mittig.
7. Farben nur aus den CSS-Variablen in `css/app.css`. Keine Hex-Werte in den eigenen Dateien.
8. Eigene Stile nur in die eigene CSS-Datei, eigenes Skript nur in die eigene JS-Datei. `core.js`,
   `home.js`, `data.js`, `boot.js`, `index.html` und `app.css` nicht ändern; fehlt dort etwas, in der
   eigenen Datei ergänzen und im Abschlussbericht nennen.
9. Bedienbar mit der Tastatur: echte `<button type="button">`, `<a href>`, `<input>`; jedes bedienbare
   Element trägt ein eindeutiges `data-fid="…"`, damit der Fokus ein Neuzeichnen überlebt.
10. Prüfen: `node --check js/<datei>.js`. Keinen Browser steuern; das macht die Hauptsession.

## Daten (`data.js`)

`DATA.T` sind 110 echte Titel; ein Titel wird überall über seinen **Index** `i` angesprochen.

- `T(i)` → `{ i, kind, title, year, min, fsk, author }`; `kind`: `m` Film, `s` Serie, `e` Folge,
  `a` Hörbuch, `p` Podcast. `KIND[kind]` ist die deutsche Bezeichnung, `dur(min)` die Dauer als Text.
- `D(i)` → Einzelheiten oder `null`. Es gibt sie für die Filme **0, 1, 3, 4** und die Serien **2, 37, 38**:
  `o` Inhaltsangabe, `g` Genres, `t` Leitsatz, `c` Besetzung `[[Name, Rolle, PersonenId], …]`,
  `l` (1 = Titel-Logo vorhanden), `f` Datei `[Videocodec, Höhe, Toncodec, GB, Container, Anzahl Fassungen]`,
  bei Serien `s` Staffeln `[[Nummer, Folgen, gesehen], …]`, `e` Folgen der ersten Staffel
  `[[FolgenId, Nummer, Titel, Minuten, Inhaltsangabe], …]`, `n` nächste Folge `[Staffel, Folge, Titel]`.
  Für alle anderen Titel aus `T(i)` eine schlichte Detailseite zeigen (ohne Besetzung).
- `row(key)` → Indizes einer Reihe: `spot`, `cont`, `rel`, `rec`, `picks`, `watch`, `wish`, `lib1` (Filme),
  `lib6` (Serien), `lib4` (Podcasts), `films`, `books`, `pods`, `docs`. `DATA.R.cont` hat `[i, Prozent]`.
- `LIBS` (in `home.js`): Bibliotheken mit `key`, `name`, `icon`, `count`, `ids()`.
- `DATA.radio` → `[[id, Name, Land, Stil], …]` (drei Sender), `DATA.col` → Sammlungen `[[id, Name, Anzahl], …]`.

Bilddateien in `img/`:

| Datei | Inhalt |
|---|---|
| `p-<i>.webp` | Poster 2:3 (Hörbücher und Podcasts quadratisch; bei Folgen ein Standbild 16:9) |
| `b-<i>.webp` | Hintergrundbild 16:9 (nicht für Hörbücher, Podcasts, Folgen) |
| `l-<i>.webp` | Titel-Logo, nur wo `D(i).l` gesetzt ist |
| `e-<FolgenId>.webp` | Standbild einer Folge 16:9 |
| `c-<PersonenId>.webp` | Porträt 2:3; fehlt bei einzelnen Personen |
| `r-<SenderId>.webp` | Senderlogo (39211, 38967) |
| `k-<SammlungsId>.webp` | Poster einer Sammlung |

`pic(datei, alt)` zeigt bei einem fehlenden Bild ein Feld mit dem Namen. Für «Titel ausserhalb der
Bibliothek» (Wünsche, Entdecken) dienen Titel aus `row('films')` und `row('docs')` als Stellvertreter:
Die Cover sind echt, nur «nicht in der Bibliothek» ist gespielt.

## Seiten anmelden

```js
route('/library/:key', (params, query) => ({ nav: 'start', html: `…` }));
PAGES.push(['#/library/films', 'Bibliothek Filme']);   // Eintrag im Sprungmenü der Mockup-Leiste
```

Eine Route liefert `{ html, nav, full, darkStatus, rot, after }`:

- `nav`: markiert das Ziel der Navigation (`start`, `search`, `downloads`, `wishes`, `more`).
- `full: true`: ohne Navigation und ohne Statuszeile (Einrichten, Player). `darkStatus` gibt es dafür nicht
  mehr zu beachten.
- `rot: 'l'`: erzwingt Querformat, solange die Seite offen ist (Videoplayer am Handy).
- `after(root)`: läuft nach dem Zeichnen.

Navigieren: `<a href="#/pfad">` oder `go('#/pfad')`. Zurück: `data-act="back"`. Neu zeichnen: `render()`;
nach einer Zustandsänderung `commit()` (speichern und zeichnen).

## Zustand

- Dauerhaft `S` (siehe `seed()` in `core.js`): `S.setup`, `S.server`, `S.account`, `S.profile`,
  `S.profiles` (`{name, color, main, pin, fsk}`), `S.watch`, `S.seen`, `S.downloads`, `S.wishes`, `S.tvs`,
  `S.devices`, `S.prefs`, `S.notes`, `S.playing`. Eigene Felder immer mit Vorgabe lesen
  (`S.x ?? vorgabe`), weil ältere gespeicherte Stände sie nicht haben.
- Flüchtig `ui` (`ui.open`, `ui.draft`, `ui.q`, `ui.tab`).
- `me()` ist das aktive Profil.

## Bedienung

- `<button data-act="name" data-i="…">` ruft `ACT.name(dataset, element, event)`. Eingebaut: `back`,
  `go` (`data-to`), `toggle` (`data-scope="S|ui"`, `data-path`), `set` (`data-path="S.prefs.x"`,
  `data-value`), `close`, `toast` (`data-msg`), `watch` und `seen` (`data-i`).
- `<input data-bind="ui.draft.name">` schreibt bei jeder Eingabe in den Pfad; `data-live` zeichnet neu.
- `sheet(html, {center})` öffnet ein Blatt von unten (auf breiten Anzeigen mittig); `closeSheet()`
  schliesst es. Aufbau: `<h2>Titel</h2> … <div class="acts"><button class="btn ghost" data-act="close">…`.
- `toast('Text')` zeigt eine kurze Meldung.
- Tonwiedergabe: `S.playing = { title, sub, img, href, paused }` zeigt den kleinen Player über der
  Navigation (`drawMini()` zeichnet ihn).

## Medienauswahl links

Marco, 19. November 2026: «Ich möchte jeweils auf der Linken Seite die Medienauswahl. Kann auch mit
hereinwischen kommen». Auf die Rückfrage hat er «Menü links, Leiste bleibt» gewählt.

- **Handy:** ein Menü, das von links hereinwischt (`openDrawer()`, `closeDrawer()`, `ACT.drawer`), mit
  den Bibliotheken, Webradio und «Meine Medien» (Merkliste, Gesehen, Sammlungen). Es öffnet sich über
  den Menü-Knopf oben links und durch Wischen vom linken Rand (`#edge`). `bar(titel, {back: false})`
  zeigt den Menü-Knopf von selbst; die Startseite setzt ihn mit `menuBtn()`. Die untere Leiste bleibt.
- **Tablet:** dieselbe Auswahl steht fest in der linken Spalte unter den fünf Zielen (`mediaNav()` in
  `#rail`). Markiert ist immer nur ein Eintrag.
- Auf einem echten Android-Handy mit Gestensteuerung gehört das Wischen vom linken Rand dem System
  (Zurück). In der App öffnet dann der Menü-Knopf das Menü; das ist beim Bau zu berücksichtigen.

## Bauteile (`app.css`)

`bar(titel, {back, end, over})`, `.ib` (runder Knopf mit Symbol), `.btn` (`primary`, `accent`, `secondary`,
`ghost`, `danger`, `block`, `sm`), `.chips` und `.chip`, `.field` mit `.inp`, `.hint`, `li(symbol, titel,
unterzeile, ende, attribute)`, `sw(scope, pfad, titel, unterzeile)`, `.sec` (Abschnittstitel), `.card`,
`.note` (auch `.warn`), `.badge`, `.empty`, `card(i, {wide, sq, prog, sub, tag, href})`, `hrow(titel, ids,
opts, alleLink)`, `.grid`, `avatar(profil, grösse)`, `icon(name, grösse)` (Namen siehe `ICONS` in `core.js`),
`fsk(n)`.

## Wem welche Seite gehört

| Datei | Routen |
|---|---|
| `home.js` | `/` |
| `setup.js` | `/setup`, `/setup/login`, `/profiles`, `/more`, `/devices`, `/tv`, `/settings/profiles`, `/settings/playback`, `/settings/notifications`, `/account` |
| `browse.js` | `/library/:key`, `/item/:i`, `/person/:id`, `/search`, `/list/:key`, `/watchlist`, `/watched`, `/collections`, `/collection/:id`, `/play/:i` |
| `media.js` | `/downloads`, `/book/:i`, `/podcast/:i`, `/listen/:i`, `/radio`, `/wishes`, `/wish/:i`, `/notifications`; dazu `ACT.dlAsk` (Download-Dialog, `data-i`) |

Seiten verweisen frei aufeinander; die Adressen dieser Tabelle gelten.
