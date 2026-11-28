# Konzept 28 · Baukasten der Live-TV-Mockups

Stand: 28. November 2026 · Fassung 1 · Status: Entwurf, nicht freigegeben

Endstand nach dem Kern-Durchgang: Alle Seiten sind gebaut; die sechs Seitenagenten arbeiten nicht mehr.
Wer etwas ändert, hält sich trotzdem an diesen Vertrag. Rundgang und Abdeckung: `docs/planning/28-live-tv/19-mockups.md`.

Vertrag für alle, die Seiten ergänzen. Bauvorlage sind `docs/planning/28-live-tv/14-oberflaeche.md`
(Zuschauer, LS-01…15, LS-20…26, LD-01…11) und `15-administration.md` (LA-01…16, LA-20…28). Reines
HTML, CSS und JavaScript: klassische `<script src>`, kein Modul-Import, kein `fetch` lokaler Dateien, kein
Build, keine Fremdschrift, keine CDN. Alles muss per Doppelklick (file://) und über einen Dateiserver laufen.

## 1. Ordner

```
index.html          Übersicht: vier Einstiege, Liste aller Seiten mit Stand (aus dem Register)
shared/             gehört dem Kern
  live-data.js      window.LIVE: alle Beispieldaten (nur lesen)
  live-sim.js       window.LIVE_SIM: veränderlicher Zustand im Speicher, Uhr, Rechte, Aktionen
  live-ui.js        window.LIVE_UI: Formatierer, Logo und Bild mit Rückfall, Fortschritt, Abzeichen, Texte
  live-ui.css       Stile dazu und die Live-Tokens (--live-rec, --live-now, --live-cur, --live-past, --live-gap-*)
  live-register.js  window.LIVE_REG: Register aller Seiten und Dialoge, Agenten, Clients
  live-mock.js/.css Leiste «Zustände» und Sprungmenü (in allen drei Clients gleich), Aktionen je Client
  img/              NUR LOKAL (git-ignoriert): logos/<slug>.png, lib/<datei>.webp
web/                Web-App (Zuschauer und Verwaltung), Vorlage Mockup 25; js/admin-core.js = gemeinsame Zähler der Verwaltung
tv/                 Android TV, Vorlage Mockup 26
handy/              Handy-App (Handy und Tablet), Vorlage Mockup 27
```

Jede `index.html` lädt: `shared/live-data.js`, `live-sim.js`, `live-ui.js`, `live-register.js`,
`live-mock.js`, die Registerdateien `js/reg-*.js`, dann den Kern des Clients, die Musterseite(n), **alle
Seitendateien der Agenten** (schon eingebunden, zunächst leer) und zuletzt `js/boot.js`. Seitenagenten
ändern keine `index.html` und keine Datei des Kerns.

## 2. Dateizuschnitt (so wurde gebaut; sechs Agenten gleichzeitig, ohne Überschneidung)

| Agent | Seiten | Eigene Dateien (nur diese ändern) |
|---|---|---|
| **Web Zuschauer A** | LS-01, LS-03, LS-05, LS-06, LS-15, LS-21 bis LS-26 (LS-02 und LS-20 hat der Kern gebaut) | `web/js/viewer-a.js`, `web/js/viewer-a-2.js`, `web/css/viewer-a.css`, `web/js/reg-viewer-a.js` |
| **Web Zuschauer B** | LS-04, LS-07 bis LS-14, alle Dialoge LD-01 bis LD-11 (LD-09 hat eine Grundform im Kern) | `web/js/viewer-b.js`, `web/js/viewer-b-2.js`, `web/css/viewer-b.css`, `web/js/reg-viewer-b.js` |
| **Web Verwaltung A** | LA-01, LA-03 bis LA-08, LA-20 bis LA-24, LA-26 bis LA-28 (LA-02 und LA-25 hat der Kern gebaut) | `web/js/admin-a.js`, `web/js/admin-a-2.js`, `web/css/admin-a.css`, `web/js/reg-admin-a.js` |
| **Web Verwaltung B** | LA-09 bis LA-16 | `web/js/admin-b.js`, `web/js/admin-b-2.js`, `web/css/admin-b.css`, `web/js/reg-admin-b.js` |
| **TV** | alle Zuschauerseiten der TV-App ausser LS-02 und LS-20, die Dialoge der TV-App | `tv/js/pages.js`, `tv/js/pages-2.js`, `tv/js/pages-3.js`, `tv/css/pages.css`, `tv/js/reg-tv.js` |
| **Handy** | alle Zuschauerseiten der Handy-App ausser LS-02 und LS-20, die Dialoge (LD-09 Grundform im Kern) | `handy/js/pages.js`, `handy/js/pages-2.js`, `handy/js/pages-3.js`, `handy/css/pages.css`, `handy/js/reg-handy.js` |
| Kern | Gerüst, Musterseiten LS-02 (alle drei Clients), LS-20, LA-02, LA-25, Grundform LD-09, gemeinsame Zähler der Verwaltung (`web/js/admin-core.js`), alle Mockup-Zustände | alles andere |

Musterseiten zum Abschauen: `web/js/page-ls02.js`, `web/js/page-la02.js`, `tv/js/channels.js`,
`handy/js/channels.js`. Wer etwas im Kern vermisst, ergänzt es in der eigenen Datei (Präfix für globale
Namen: `va…`, `vb…`, `aa…`, `ab…`, `tv…`, `hp…`) und nennt es im Abschlussbericht.

**Register und Stand:** Die Einträge stehen schon in der eigenen `reg-*.js`. Ist eine Seite fertig, dort
`"offen"` auf `"gebaut"` setzen. Eine Seite ohne angemeldete Route zeigt automatisch die Platzhalterseite
«LS-xx … wird ergänzt»; sobald die Seitendatei `route(...)` für dasselbe Muster aufruft, gewinnt die echte
Seite. **Statische Routen vor Parameter-Routen anmelden** (`/live/recordings/profiles` vor
`/live/recordings/:id`), weil die erste passende Route gilt.

## 3. Seiten anmelden

### Web (`web/js/core.js`, `shell.js`)

```js
route('/live/guide', (params, query) => ({
  type: 'viewer', nav: 'live', title: 'Programm',
  html: livePage({ tab: 'guide', title: 'Programm', count: '16 Sender', actions: '…', html: '…' }),
  after: (root) => { /* nach dem Zeichnen */ },
}));
route('/admin/live/epg', (params, query) => ({
  type: 'admin', area: 'livetv', page: 'epg', title: 'EPG', intro: 'Ein Satz.', actions: '<a class="btn primary" …>',
  live: 'Aktualisiert vor 3 s', back: ['#/admin/live', 'Übersicht'], html: subTabs('#/admin/live/epg', [['sources', 'Quellen', 2], ['mapping', 'Zuordnungen', 584]], query.tab || 'sources') + '…',
}));
```

- `livePage({ tab, title, count, sub, actions, html })`: Rahmen jeder Zuschauerseite unter `/live` mit
  Reiterleiste (Start · Sender · Programm · Aufnahmen · Erinnerungen, «Aufnahmen» nur mit Recht) und Fussnote
  «Beispieldaten, simulierte Wiedergabe». Den Player (LS-07, LS-10) baut man ohne `livePage` (Vollfläche).
- Verwaltung: `area: 'livetv'` und `page` aus `sources | channels | epg | reception | live-streams |
  diagnostics | storage | live` (Übersicht); geänderte bestehende Seiten mit ihrem Bereich, z. B.
  `area: 'people', page: 'accounts'`. Unterreiter mit `subTabs(base, [[key, label, zahl]], aktiv)`.
- Navigieren: `<a href="#/…">` oder `go('#/…')`; neu zeichnen `render()` (Fokus und Bildlauf bleiben).
- Aktionen: `<button data-act="name" data-…>` ruft `ACT.name(dataset, element, event)`. Eingebaut u. a.
  `back`, `dialog` (`data-name`), `close`, `open`/`band` (Aufklapper), `tog`, `set`, `fav` (`data-ch`),
  `tune` (`data-ch`, `data-list`), `liveRetry`. Bindung: `<input data-bind="ui.draft.x" data-fid="…">`.
- Jedes bedienbare Element trägt `data-fid="…"` (eindeutig), damit der Fokus ein Neuzeichnen überlebt.
- Bauteile aus Mockup 25 (`app.css`, `viewer.css`, `admin.css`): `.btn` (`primary`, `secondary`,
  `ghost`, `danger`, `sm`, `icon`), `.chip`, `.dot`, `.fld`, `.inp`, `.sel`, `.sw` (über `sw(pfad,
  text)`), `.check`, `.seg`, `.pills`, `notice(art, titel, text, aktionen)`, `band(id, titel, meta,
  inhalt, {open, icon, viewer})`, `.panel`, `.lrow`, `.facts`, `.empty`, Verwaltung `.card`, `.stats`,
  `.tiles`, `.srow`, `.tblw`/`.tbl`, `.kv`, `.checks`, `.feed`, `.savebar`, `.subtabs`. `icon(name, grösse)`
  mit den Namen aus `ICONS` (neu: `record`, `recordSeries`, `reminder`, `calendar`, `live`, `heart`,
  `heartOn`, `grid`, `filter`, `upload`, `wifi`, `home2`, `arrowUp`, `arrowDown`).
- Neu im Kern: `popMenu(id, labelHtml, items, {aria, cls})` (Kontextmenü; `items: [{label, href | act,
  data, danger, icon}]`, Pfeiltasten und Esc gehen), `toastUndo(html, act, data)`, `skelRows(n, höhe)`,
  `liveError(text)`, `emptyState(titel, text, aktionen)`, `liveMode()`, `chFilterSheet(werte, onApply)`.
- Dialoge der Zuschauer: nur über `LiveDlg.*` öffnen (Tabelle in Abschnitt 6). «Web Zuschauer B» füllt sie
  mit `registerLiveDialog('LD-01', (props) => ({ title, body, actions, wide }))`; sonst erscheint ein
  Platzhalterdialog. Eigene Dialoge der Verwaltung: `DIALOGS.name = (props) => ({…})`, `openDialog(name,
  props)`, `closeDialog()`. Fokus beim Öffnen auf `[data-autofocus]`.
- Breiten über Container-Abfragen `@container app (min-width: 768px)` (nicht `@media`), Handy zuerst.
- Verwaltungsseiten ändern die Beispieldaten direkt (`LIVE.sources`, `LIVE.channels`, …) und rufen dann
  `render()`; Neuladen stellt alles wieder her.

### TV (`tv/js/core.js`, `live-core.js`)

```js
route('/live/guide', (params, query) => ({
  nav: '', html: livePage({ tab: 'guide', title: 'Programm', html: '…' }),
  onFocus(el) { /* Vorschau nachführen */ },
  key(name, ev) { if (name === 'ch_down') { /* eine Seite */ return true; } return false; },
  back() { return false; },          // true = erledigt
}));
```

- Fokus-Engine wie Mockup 26: jedes bedienbare Element `data-f="eindeutig"`, Startfokus `data-default`,
  Reihen `data-row`, OK ruft `data-act` oder folgt `href`. Keine `<input>` im Bild: Text über
  `keyboard({ value, label, onDone })`. Menüs `menu(html, { dialog, onKey, onClose })`, `redrawMenu`,
  `closeMenu`. `toast(text)`, `focus(el)`, `focusTo(id)`, `bump(el, dir)`.
- **Tasten** (`key(name)` der Seite bekommt sie zuerst): `up down left right ok back home play` und neu
  `ch_up ch_down guide info rec last d0…d9`. Tastatur: Bild↑/Bild↓ = Kanal ±, G = Guide, I = Info, R =
  Aufnahme, L = zuletzt gesehen, 0–9. Ohne Behandlung durch die Seite: `guide` öffnet `#/live/guide`,
  Ziffern schalten über die Nummerneingabe um, `last` schaltet zum vorigen Sender, sonst ein Hinweis.
- `liveNumber(taste, onDone(sender))`, `liveNumberCommit()`, `liveNumberActive()`: Nummerneingabe (6.7) mit
  Anzeige oben rechts und «Kein Sender mit Nummer …».
- `liveTune(ch, liste)` schaltet ein und öffnet `#/live/watch/<ch>`. Play/Pause auf einem Element mit
  `data-ch` tut dasselbe.
- Dialoge: `registerLiveDialog('LD-01', (props) => ({ html, dialog: true|false, onKey }))`, öffnen über
  `LiveDlg.*`. Bauteile aus Mockup 26 (`app.css`): `.tb`, `.tf`, `.tsw`, `.tnote`, `.badge`, `.mi`,
  `.mlist`, `.ov`, `.tc`, `.trow`, `.rsec`. Zehn-Fuss-Grössen `--fs-s` (26 px) bis `--fs-h1`.
- Öffnet ein Menü und ändert sich der Zustand, zeichnet der Kern die Seite erst nach dem Schliessen neu.

### Handy (`handy/js/core.js`, `live-core.js`)

```js
route('/live/recordings', () => ({ nav: '', html: livePage({ tab: 'recordings', title: 'Meine Aufnahmen', end: '<button class="ib" …>', html: '…' }) }));
route('/live/watch/:id', (p) => ({ full: true, rot: 'l', html: '…' }));   // Player: ohne Leisten, quer
```

- `livePage({ tab, title, end, html })`: obere Leiste mit Menü-Knopf (Medienauswahl), Chip-Reihe des
  Bereichs, Fussnote. Bauteile aus Mockup 27: `bar(titel, {back, end})`, `li(symbol, titel, unterzeile,
  ende, attribute)`, `sw(scope, pfad, titel, unterzeile)`, `.btn`, `.chips`/`.chip`, `.field`/`.inp`,
  `.sec`, `.card`, `.note`, `.badge`, `.empty`, `sheet(html, {center})`, `closeSheet()`, `toast(text)`.
- Dialoge als Blatt: `registerLiveDialog('LD-01', (props) => ({ html, center }))`, öffnen über `LiveDlg.*`;
  LD-09 hat eine Grundform (`chFilterSheet`). Zwei Breiten: `@container app (min-width: 700px)`.

## 4. Datenmodell `window.LIVE` (nur lesen)

**Zeit:** alle Zeitpunkte in **Minuten seit Sa 28.11.2026, 00:00** (Tag 0; So = 1440; So 22.11. = −6 × 1440).
`LIVE.NOW` = 1241 (20:41), `LIVE.EPG_END` = Sa 5.12., 00:00. `LIVE.at(tag, 'HH:MM')`. Formatieren nur mit
`LIVE_UI.fmt` (nie mit `Date` in Ortszeit).

| Feld | Inhalt |
|---|---|
| `account` | Familie Keller, `quotaGB` 200, `usedGB` (berechnet, 142), `rights` (Deckel) |
| `profiles[]` | `{ id: anna|luca|mia, name, primary, age (null|16|6), accent, rights }` (Belegung über `usageOf`) |
| `channels[]` | `{ id, no, name, slug, country, langs[], groups[], category, epg ('ok'|'none'|'partial'), epgKey, epgNote, reception, schedule, state ('ok'|'disturbed'), since, bitrate, video, audio[], subs[], sources[{source, name, url (maskiert), state}], favCount, active }`; `goneChannels[]` (TV24) |
| `groups[]`, `countries`, `languages`, `categories[]` | Gruppen (Herkunft Quelle/eigene), Länder, Sprachen, Kategorien |
| `programs[]` | `{ id ('p-<ch>-<start>'), ch, start, end, title, sub, genre, desc, series, seriesName, season, episode, repeat, firstRun, live, flags[], img (Datei in img/lib oder null), year, country, director }`; Sa 28.11. bis Fr 4.12.; TeleZüri ohne, phoenix nur bis Sa 20:00, KiKA bis 21:00 |
| `favorites{profil: [ch]}`, `recent{profil: [[ch, zeit]]}` | Anfangszustand (veränderlich über LIVE_SIM) |
| `recordings[]` | `{ id, profile, title, sub, ch, start, end, padBefore, padAfter, sizeGB, state (running|done|incomplete|failed|rejected), reason, gaps[[a,b]], fallbackAt, watchedPct, watched, resumeAt, keep {mode: 'forever'|'days', days, until}, origin (single|series|instant), rule, tracks }` |
| `rules[]` | Planungen: `kind` single (program, start, end), series (series, channels 'all'|[], onlyNew, noDup, keepLast, skipped[[programId, grund]]), recurring (ch, from, to, weekdays[1..5]), manual (TV24, Warnung «Sender weg») |
| `warnings[]`, `reminders[]` | Warnungen `{ id, kind, severity, rules[], text }`; Erinnerungen `{ program, lead }` |
| `sources[]`, `importRuns{}`, `importResult` | Quellen (4) mit `moreConn` (Einstellung «Vorladen für Sender dieser Quelle»; nie «Limit» oder eine Rechnung je Quelle) und `fixedTargets` (Multicast: feste Ziele im Heimnetz), letzte Läufe, Ergebnis des letzten Imports (neu, geändert, verschwunden, Duplikate 3, fehlerhaft) |
| `epgSources[]`, `epgRuns{}`, `mappings[]`, `mappingCounts`, `duplicates[]`, `totals` | EPG und Zuordnungen, Duplikate, Zahlen (584 aktiv von 10'354) |
| `inputs[]`, `consumers[]`, `diagnostics`, `resources`, `limits` | aktive Eingänge (Zustand `'deferred'` mit `deferredReason`: zurückgestellt, z. B. 3sat), Abnehmer (ohne Titel), Diagnose, Kennzahlen; `limits` = `{ inputs: 8, preloaded: 4 }` |
| `storage[]`, `accounts[]`, `settings` | drei Speicherorte, Belegung je Konto, Einstellungen (DVR, Timeshift, Empfang, Zeitpläne, EPG, Grenzen) |

Bilder: Nur wenn `LIVE_UI.imagesOn()` (Schalter «Echte Logos und Bilder», `setImages(an)`, Schlüssel
`sc28-img`) fragt ein Mockup überhaupt eine Bilddatei an; sonst Initialen und Flächen. `img` eines Programms zeigt auf ein Bild der Testbibliothek aus Mockup 27 als Stellvertreter (nur
Filme und Serien; Nachrichten, Sport, Shows haben keines). Logos über den Slug.

## 5. Zustand und Aktionen `window.LIVE_SIM`

**Mockup-Zustände: je Zustand genau ein Schalter, in der Leiste «Zustände» jedes Clients.** Seiten lesen sie
nur (`SIM.flags()`, `SIM.mode()`, `SIM.quota()`), sie bauen keine eigenen Schalter. Was kein Zustand ist, sondern
eine einmalige Vorführung (z. B. «Erinnerung jetzt zeigen»), meldet ein Client mit
`LIVE_MOCK.addAction(text, fn)` im Abschnitt «Aktionen (dieser Client)» an. Einzige Auswahl auf einer Seite:
das Testergebnis in LA-03 («Nur im Mockup: Ergebnis des Tests»), weil es kein Zustand ist, sondern die Antwort
einer gedachten Quelle auf genau diesen Test.

| Schalter | Wert in `SIM.flags()` | Wirkung |
|---|---|---|
| Uhr anhalten, +5 Min., +30 Min., +2 Std. | – | `now()`, Fortschritt, laufende Aufnahmen wachsen |
| Wer schaut? Anna, Luca, Mia | – | `setProfile` |
| Echte Logos und Bilder | `LIVE_UI.imagesOn()` | aus ab Werk: keine Bildanfrage |
| Rechte des Profils / Deckel des Kontos | – | `rights()`; ohne Live-TV «Nicht gefunden» (404) und **kein Wort** vom Bereich auf anderen Seiten (Start, Suche, Glocke, Profil, Radio, Menü, Leiste), Tasten stumm |
| Recht entzogen während der Wiedergabe | `revoked` | Recht weg; der Player zeigt «Live-TV ist für dieses Profil nicht mehr freigegeben.», alle anderen Adressen 404 |
| Zustand der Seiten: Normal, Leer, Lädt, Fehler | `SIM.mode()` | jede Seite; bei «Leer» liefern auch `recent()`, `recordings()`, `rules()`, `upcoming()`, `warnings()`, `reminders()`, `favorites()` nichts |
| Ohne EPG | `noEpg` | kein Sender hat Programmdaten |
| Sender gestört: zusätzlich SRF 1 | `disturbed` | `chState('srf1')` = disturbed |
| Viele Sender | `many` | 2'000 Testsender zusätzlich im Raster (Web: Testzeilen von LS-04, TV: `gridChannels()`) |
| Kontingent voll | `quotaFull` | `quota().full`, Sofortaufnahme abgelehnt, `rejectForQuota` |
| Player: Sender nicht erreichbar, Ersatzquelle aktiv, Grenze erreicht, Strom stockt, Langsamer Start, Nicht abspielbar, Server ausgelastet | `fault` | über `LIVE_UI.playerPhase` in allen drei Playern |
| Verwaltung (nur Web): Speicher knapp, voll, Netzlaufwerk nicht eingehängt | `admStorage` | LA-01, LA-12, LA-14 bis LA-16 |
| Verwaltung (nur Web): EPG-Import fehlgeschlagen | `epgFail` | LA-01, LA-10 |

**Player-Phasen** (gleich in allen Clients): `LIVE_UI.playerPhase(msSeitUmschalten, sender, { hot })` →
`{ k: 'loading'|'slow'|'ok'|'error', spin, err, stall, fallback }`, Schwellen in `LIVE_UI.LOAD` (im Mockup
verkürzt: Ladeanzeige ab 0.5 s, «Dauert länger» ab 2.5 s, Fehler nach 6 s; Konzept 14 Fassung 2: 0.5 / 3 / 15 s),
Fehlertexte in `LIVE_UI.PLAYER_ERR`.

**Zahlen:** Die Belegung eines Profils ist die Summe seiner Aufnahmen (`usageOf(id)`, `usage()` →
`{ usedGB, count, byProfile }`); `LIVE.account.usedGB` und der Eintrag des Kontos in `LIVE.accounts` rechnen
damit (142 GB, 31 Aufnahmen um 20:41). «Geplant · n» kommt immer aus `upcoming()`. Warnungen zählt jeder
Client nur mit `SIM.warningCount()` und schreibt sie mit `LIVE_UI.warnText(n)` («n Warnungen zu geplanten
Aufnahmen»). Die Verwaltung zählt Sender,
Zuschauer, laufende Aufnahmen und Eingänge nur über `admCounts()`, `admViewers()`, `admRecRunning()`,
`admRecSoon(min)`, `admInputs()` (`web/js/admin-core.js`; zurückgestellte Eingänge zählen getrennt), damit
LA-01, LA-12, LA-14, LA-16 und LA-21 gleich zeigen.
Zahlen immer im Format de-CH: Dezimalpunkt und Hochkomma (`4.1 GB`, `10'354`) über `fmt.dec`, `fmt.num`,
`fmt.gb`; nie `toLocaleString` und nie ein Komma als Dezimalzeichen.

- Ereignisse: `LIVE_SIM.on(fn)` mit `'change'` (Kern zeichnet neu), `'minute'`, `'tick'` (Fortschritt und
  Uhren aktualisiert `LIVE_UI` selbst).
- Uhr: `now()`, `advance(min)`, `setClock(läuft)`, `clockRunning()`.
- Profil und Rechte: `profile()`, `profiles()`, `setProfile(id)`, `rights()` → `{ liveTv, recordings,
  timeshift }` (wirksam = Konto UND Profil), `rawRights('profile'|'account')`, `setRight(ebene, schlüssel,
  an)`. Ohne Recht gibt der Kern für `/live…` (bzw. Aufnahmeseiten) «Nicht gefunden» zurück; Knöpfe
  «Aufnehmen» und Timeshift lassen die Seiten selbst weg (`rights()` prüfen).
- Mockup-Zustände: `mode()` → `'normal'|'empty'|'loading'|'error'`, `flags()` → `{ noEpg, disturbed, many, quotaFull, fault, revoked, admStorage, epgFail }`.
  **Jede Seite zeigt alle vier Modi.**
- Sender: `channels({ q, group ('fav' = Meine Sender), country, lang, category })` (sortiert, leer bei
  `empty`), `channel(id)`, `channelByNo(nr)`, `chState(c)` → `'ok'|'disturbed'|'gone'`, `epgOf(c)`.
- Programm: `program(id)`, `programsOf(ch, von, bis)`, `programAt(ch, t)`, `nowNext(ch, t?)` → `{ now,
  next }`, `gapsOf(ch, von, bis)`, `genresSoon(stunden)`, `search(q, { when: now|today|tomorrow|week,
  genre, group })`.
- Favoriten: `favorites()`, `isFav`, `toggleFav(ch)` → an?, `moveFav(ch, ±1)`, `recent()` (bis 20 Sender),
  `clearRecent()` («Liste leeren»).
- Aufnahmen: `recordings(profil?)` (laufende wachsen: `liveGB`), `recording(id)`, `deleteRecordings(ids)`,
  `stopRecording(id)`, `setKeep(id, keep)`, `setWatched(id, pct)`, `runningOn(ch)`, `childProfiles()`.
- Planung: `rules()`, `rule(id)`, `upcoming()` → konkrete Termine `{ id, rule, kind, ch, title, sub,
  start, end, program }` (Serien mit Zeitfenster `window` und Wochentagen 0 = So … 6 = Sa),
  `recState(programId)` → `null|'scheduled'|'series'|'recording'`, `skippedWhy(programId)` → Grund, warum
  eine Serienaufnahme die Sendung auslässt (`'schon geplant'`, `'Wiederholung'`, `'schon aufgenommen'`, `'ausgelassen'`),
  `ruleForProgram(id)`, `planRecording(programId, { padBefore, padAfter, keep })`, `planSeries(programId,
  opts)`, `planManual({ ch, title, start, end } | { ch, title, from, to, weekdays })`, `updateRule`,
  `removeRule`, `skipOccurrence(terminId)`, `startInstant(ch, { until: 'end'|30|60|'stop' })` → `{ ok, id,
  end }` oder `{ ok: false, reason: 'quota' }`, `rejectForQuota(termin)` (Termin startet nicht, erscheint unter
  «Nicht aufgenommen», Meldung an Profil und Hauptprofil), `notices(profil?)` (Meldungen für die Glocke).
- Warnungen und Erinnerungen: `warnings()`, `warningsFor(ruleId)`, `dismissWarning(id)`, `reminders()`,
  `reminderFor(programId)`, `setReminder(programId, vorlauf)`, `removeReminder(programId)`; `quota()` →
  `{ totalGB, usedGB, mineGB, othersGB, freeGB, pct, full }`; `usage()`, `usageOf(profil)`.
- Player und Timeshift: `tune(ch, { list })` (Timeshift beginnt neu), `zap(±1)`, `lastChannel()`,
  `pause()`, `resume()`, `seek(minutenZurück)`, `toLive()`, `stop()`, `player()` → `{ ch, joinedAt,
  offset, paused, list, prev, max, position, live }`. `reset()`.

**Anzeige `window.LIVE_UI`:** `fmt.time|clock|day|dayLong|dayRel|when|range|dur|offset|gb|num|ago|left`,
`logo(ch, {cls, label})` (helle Fläche, Logo eingepasst; nur die 16 bekannten Slugs laden eine Datei, sonst Initialen), `picture(programm, { fallback:
'logo'|'title'|'none', cls })`, `progress(start, ende, { cls: 'rec', label })` (läuft mit der Uhr),
`nowText('time'|'left', ende)`, `fmt.dec(n, max, min)`, `LOAD`, `playerPhase`, `PLAYER_ERR`, `badge('scheduled'|'recording'|'series'|'reminder'|'warning'|'incomplete'|
'failed', { text })`, `programBadges(programm, { text })`, `fakeVideo(ch, { paused })` (simuliertes Bild
mit Vermerk), `sampleNote()`, Texte `TX`, `REC_STATE`, `REASON`, `RULE_KIND`, `RECEPTION`, `SOURCE_KIND`.
CSS-Klassen: `.lv-logo`, `.lv-pic`, `.lv-prog`, `.lv-badge`, `.lv-gap` (Schraffur «Keine Programmdaten»),
`.lv-vid`, `.lv-skel`.

## 6. Dialoge (gemeinsame Aufrufe)

| Aufruf | Dialog |
|---|---|
| `LiveDlg.planRecording(programId)` | LD-01 Aufnahme planen / ändern |
| `LiveDlg.seriesRecording(programId, ruleId?)` | LD-02 Serienaufnahme |
| `LiveDlg.manualRecording({ ch, ruleId, start, end })` | LD-03 Manuelle oder wiederkehrende Aufnahme |
| `LiveDlg.instantRecording(ch)` | LD-04 Sofortaufnahme |
| `LiveDlg.reminder(programId)` | LD-05 Erinnerung |
| `LiveDlg.confirmDelete({ kind, ids, onDone })` | LD-06 Löschen und Abbrechen (`kind`: recording, recordings, running, childRecording, occurrence, rule, series) |
| `LiveDlg.warning(warningId)` | LD-07 Konflikt oder Warnung |
| `LiveDlg.pickTime({ day, time, onPick })` | LD-08 Tag und Uhrzeit |
| `LiveDlg.filterSheet({ fields, values, onApply })` | LD-09 Filterblatt (Web, Handy) |
| `LiveDlg.zapConfirm({ to, onConfirm })` | LD-10 Senderwechsel bei Zeitversatz |
| `LiveDlg.retention(recordingId)` | LD-11 Aufbewahrung ändern |

Jeder Dialog hat zudem eine Vorschauseite `#/live/dialog/LD-xx`, die ihn mit Beispieldaten öffnet.

## 7. Regeln

1. **Design wie die Vorlage:** Web exakt Mockup 25, TV exakt Mockup 26, Handy exakt Mockup 27. Farben nur
   aus den CSS-Variablen der Vorlage und `shared/live-ui.css`; keine Hex- oder rgba-Werte in Seitendateien.
2. **Marcos Regeln:** Klappbares ist zu und zeigt die Zusammenfassung in der Kopfzeile (Ausnahmen:
   Sprungziel, eben Angelegtes, Aufklapper mit Fehler); Rahmen ≥ 3:1, gedämpfter Text ≥ 8:1 (nur `--tx2`,
   `--tx3`); Logos und Sendungsbilder **ohne Rahmen**; Anlegen und Ändern im selben Formular;
   Medienauswahl links.
3. **Bilder:** Logos nur über `LIVE_UI.logo`, Sendungsbilder nur über `LIVE_UI.picture` oder die
   Bildhelfer des Clients (`pic`, `art`, `drawMini`); alle fragen nur bei `imagesOn()` an. Nie ein `<img>`
   oder `url(…)` direkt in einer Seitendatei. Ohne `shared/img/` muss jede Seite vollständig und ansehnlich
   bleiben. Echte Medienbilder nie in Git.
4. **Beispieldaten kennzeichnen:** jede Seite trägt die Fussnote (`livePage` macht das), jede Wiedergabe
   den Vermerk «Beispieldaten, simulierte Wiedergabe» (`fakeVideo`). Keine echten Stream-Adressen, keine
   Zugangsdaten; Adressen nur `example.*`, maskiert.
5. **Texte:** Deutsch in Schweizer Schreibweise (ss), «Guillemets», du-Form, Begriffe aus 14 Abschnitt 7.
6. **Grenzen des Konzepts:** kein «Von vorne», kein Archiv, kein Bild-im-Bild, kein Altersfilter, keine
   geteilten Aufnahmen, keine Aufnahmen auf Startseite, in Suche oder Bibliothek (14 Abschnitt 8). Der
   Verwalter sieht nie Titel fremder Aufnahmen.
7. **Jeder Knopf tut sichtbar etwas** (Navigation, Zustand, Dialog, Meldung). Kein toter Knopf.
8. Prüfen: `node --check <datei>.js`; im Browser über einen Dateiserver und per Doppelklick (`file://`).
9. Symbole: `recordSeries`, `recordHollow`, `stop`, `arrowUp`, `arrowDown`, `loadspin`, `expand` stehen in jedem
   `core.js`, die Player-Symbole `vb…` im Web-Kern. Seitendateien ergänzen `ICONS` nicht mehr.

## 8. Prüfliste je Seite

- [ ] Alle Knöpfe, Links, Menüs und Untermenüs, Kontextmenüs, Filter, Reiter, Such- und Eingabefelder,
      Auswahlfelder und Dialoge funktionieren und wirken sichtbar (LIVE_SIM-Aktion, Navigation, Meldung).
- [ ] Zustände: normal, **leer**, **lädt** (Skeleton in Endmass), **Fehler** (mit «Erneut versuchen»),
      **ohne EPG**, Sender **gestört**; ohne Recht Aufnahmen/Timeshift fehlen die Knöpfe; Profil Luca/Mia
      geprüft (LS-14 nur Hauptprofil).
- [ ] Keine Fehler in der Konsole; kein waagrechter Überlauf (Web: Desktop, Tablet, Handy; Handy-App:
      Handy hoch und quer, Tablet; TV: 1920 × 1080).
- [ ] Web: mit Tastatur bedienbar (Tab, Pfeile, Enter, Esc, sichtbarer Fokus, keine Fokusfalle).
      TV: nur mit Fernbedienung (Steuerkreuz, OK, Zurück; Kanal ±, Guide, Ziffern, wo die Seite sie nutzt).
- [ ] Im Sprungmenü erreichbar (Registereintrag, Stand auf «gebaut»), Adresse fest (Hash-Route).
- [ ] Ohne `shared/img/` und mit Schalter aus geprüft: Initialen und Flächen statt Bilder, 0 Konsolenfehler.
- [ ] Ohne Recht Live-TV (bzw. Aufnahmen, Timeshift): kein Wort vom Bereich auf anderen Seiten.
