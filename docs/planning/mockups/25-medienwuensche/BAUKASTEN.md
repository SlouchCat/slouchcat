# Baukasten des Mockups (für alle, die Seiten ergänzen)

Reines HTML, CSS und JavaScript ohne Build und ohne Fremdbibliothek. `index.html` lädt klassische Skripte in fester
Reihenfolge: `data.js`, `core.js`, `store.js`, `shell.js`, `home.js`, `viewer-titles.js`, `viewer-pages.js`, `admin-a.js`,
`admin-b.js`, `mock.js`. Alle Funktionen sind global. Das Mockup teilt keinen Code mit dem Produkt.

## Regeln

1. Jeder Knopf und jeder Link tut sichtbar etwas (Zustand ändern, Dialog, Navigation, Meldung per `toast`). Kein toter Knopf.
2. Texte der Oberfläche: Deutsch, Schweizer Schreibweise (ss statt ß), Anführungszeichen «so», du-Form, die Begriffe aus dem
   Entscheid-Blatt («Wünsche», «anfordern», «Noch nicht in der Bibliothek», Zustandsnamen aus `STATE_LABEL`).
3. Alles Klappbare startet zugeklappt und zeigt seine Zusammenfassung im Kopf (`band(...)`). Ausnahmen: Sprungziel, eben
   Angelegtes, ein Aufklapper mit Fehler.
4. Anlegen und Ändern im selben Formular. Sichtbare Rahmen, gedämpfter Text nur mit `--tx3`/`--tx2` (hoher Kontrast).
5. Bedienbar mit der Tastatur: echte `<button type="button">`, `<a href>`, `<input>`, `<select>`; jedes bedienbare Element
   trägt ein eindeutiges `data-fid="…"` (damit der Fokus ein Neuzeichnen überlebt). Dialoge über `openDialog`.
6. Responsiv über Container-Abfragen: `@container app (min-width: 768px) { … }` (nicht `@media`), Handy zuerst.
7. Keine Farben ausserhalb der CSS-Variablen aus `css/app.css` (`--bg --elev --s1 --s2 --s3 --tx --tx2 --tx3 --line --frame
   --ctl --danger --ok --warn --info --accent --on-accent`). Keine externen Ressourcen, keine Netzaufrufe.
8. Eigene Stile nur in die eigene CSS-Datei, eigenes Skript nur in die eigene JS-Datei. `core.js`, `store.js`, `shell.js`,
   `home.js`, `mock.js`, `app.css`, `viewer.css`, `admin.css` nicht ändern; fehlt dort etwas, in der eigenen Datei ergänzen
   (z. B. `ACT.x = …`, zusätzliche Funktion) und im Abschlussbericht nennen.
9. Bestehende Funktionen der echten Seiten dürfen im Mockup nicht fehlen: Was heute auf der Seite ist, bleibt sichtbar und
   bedienbar (Zustand im Mockup simulieren). Neue Felder mit `class="srow new"` bzw. `<span class="newtag">NEU</span>` kennzeichnen.
10. Prüfen: `node --check js/<datei>.js`. Den Browser prüft die Hauptsession; nicht selbst einen Browser steuern.

## Seiten anmelden

```js
route('/wishes', (params, query) => ({ type: 'viewer', nav: 'wishes', title: 'Meine Wünsche', html: `…` }));
route('/admin/wishes', () => ({ type: 'admin', area: 'media', page: 'wishes', title: 'Wünsche',
  intro: 'Ein Satz.', actions: `<button …>`, html: `…`, back: ['#/admin/integrations', 'Alle Integrationen'] }));
```

- `type: 'viewer'`: Kopfzeile und Seitenleiste der App; `nav` markiert den Eintrag (`start`, `search`, `wishes`, `profiles`).
  Inhalt beginnt meist mit `<div class="page"><div class="head"><h1>…</h1><p class="sub">…</p><div class="acts">…</div></div> … </div>`.
- `type: 'admin'`: Verwaltungsrahmen mit Bereichsreitern und Seitenkopf (`area`: `media|people|home|automation|system|overview`,
  `page`: Schlüssel aus `AREAS` in `shell.js`; `title`, optional `h1`, `intro`, `actions`, `eyebrow`, `crumb`, `back`).
  `html` ist nur der Inhalt unter dem Seitenkopf. Seite in `ADMIN_BUILT` (shell.js) ist schon eingetragen:
  `wishes, accounts, home-rows, spotlight, jobs, integrations, notifications`; `#/admin` selbst ist die Übersicht.
- Routenmuster mit `:name` (`'/items/:key'`). Abfrage hinter `?` kommt als zweites Argument. Navigieren: `go('#/pfad')`.
- `view.after(root)` läuft nach dem Zeichnen (z. B. Scrollen).
- Neuzeichnen: `render()` (behält Scroll und Fokus). Nach Zustandsänderung: `commit()` (= speichern + zeichnen).

## Zustand und Bedienung

- Dauerhaft: `S` (siehe `seed()` in `store.js`): `S.session`, `S.seerr`, `S.settings.wishes`, `S.settings.spotlight`,
  `S.settings.homeRows`, `S.settings.jobs`, `S.accounts`, `S.profiles`, `S.seerrUsers`, `S.map`, `S.wishes`,
  `S.notifications`, `S.deliveries`, `S.invitations`, `S.library`, `S.marks`. Eigene Felder darfst du ergänzen (z. B.
  `S.settings.x`), aber immer mit Vorgabe lesen (`S.x ?? vorgabe`), weil alte gespeicherte Stände sie nicht haben.
- Flüchtig: `ui` (`ui.open`, `ui.draft`, `ui.filter`, `ui.q`, `ui.busy`). Entwürfe von Formularen gehören nach `ui.draft.<name>`.
- Aktionen: `<button data-act="name" data-id="…">` ruft `ACT.name(dataset, element, event)`. Eingebaut: `tog` (Pfad
  umschalten), `set` (`data-path`, `data-value`, optional `data-num`), `band`, `open` (`ui.open[id]` umschalten), `dialog`
  (`data-name`), `close`, `back`, `drawer`, `scrollRow`, `askWish` (`data-key`: Dialog «Anfordern»), `play`, `trailer`, `mark`.
- Bindung: `<input class="inp" data-bind="ui.draft.x.name" data-fid="…">` schreibt bei Eingabe in den Pfad (`S.…` wird
  gespeichert). `data-live` zeichnet bei jeder Eingabe neu, `data-rerender` bei `change`, `data-num` wandelt in Zahl,
  `data-after="aktionsname"` ruft danach `ACT[…]`. Checkbox und Radio zeichnen immer neu.
- Helfer: `esc(text)`, `icon(name, size)` (Namen siehe `ICONS` in `core.js`), `img(src, alt)` (mit Platzhalter, falls das
  Bild fehlt), `sw(pfad, beschriftung, {disabled, after})` (Schalter), `selectEl(pfad, [{v,t,disabled}], {label, after})`,
  `band(id, titel, metaHtml, inhaltHtml, {open, icon, viewer})` (Aufklapper; `viewer: true` für die Zuschauer-App),
  `isOpen(id)`, `notice(art, titel, text, aktionenHtml)` (Art: `''|'ok'|'warn'|'bad'`), `toast(html, art)`,
  `openDialog(name, props)` mit `DIALOGS.name = (props) => ({ title, body, actions, wide })`, `closeDialog()`,
  `fmtDate(ts)`, `fmtTime(ts)`, `ago(ts)`, `cls(...)`.
- Bausteine der App (`home.js`): `posterCard(t, {ctx, caption, dim, partial})` (als `<li>`), `rowHtml(key, titel, href, titel[])`,
  `mainActions(t, {source, ctx, lg, details})` (Hauptknöpfe je nach Zustand: Abspielen / Anfordern / Zustand / Benachrichtige
  mich), `metaLine(t, {age})`, `wsChip(zustand, text?)`, `missingChip()`, `availText(a)`.

## Fachfunktionen (`store.js`)

- Titel: `T[key]`, `TMDB['movie/123']`, `window.TITLES`; Felder `key, kind ('movie'|'series'), tmdbId, title, original, year,
  age, genres[], runtime, tagline, overview, poster, backdrop, cast[{name, role, img}], seasons[{n, name, year, episodes,
  poster, overview}]`. `titleHref(t)`, `mediaOf(t)`, `kindLabel(t)`, `ageLabel(n)`, `SPOT_ORDER`, `SERIES_NOTE[key]`.
- Bibliothek: `isPresent(key)`, `libSeasons(key)`, `airedSeasons(t)`, `missingSeasons(t)`, `upcoming(key)`.
- Personen: `curProfile()`, `curAccount()`, `profile(id)`, `account(id)`, `profilesOf(accountId)`, `effMax(p)`,
  `roleLabel(p)`, `isAdmin()`, `seerrUser(id)`, `seerrUserFor(p)` → `{id, via: 'profile'|'account'|'fallback'}` oder `null`,
  `suggestionFor(account)`, `mappingCounts()`, `switchProfile(id)`.
- Recht und Sicht: `canRequest(p, t?)` → `{ok, reason, text}`, `missingVisible(p, t)`, `titleVisible(p, t)`,
  `availability(p, t)` → `{kind: 'present'|'partial'|'missing'|'wished', wish?, mine?, missing?}`.
- Wünsche: `myWishes(p, alleProfileDesKontos)`, `openWish(key, staffel?)`, `openWishes(key)`, `lastClosed(key, p)`,
  `wishById(id)`, `isMine(w, p)`, `wishCounts()`; Aktionen `requestWish(key, staffeln[])`, `withdrawWish(id)`,
  `retryWish(id)`, `approveWish(id)`, `declineWish(id)`, `failWish(id)`, `arriveWish(id)`, `advanceWish(id)`, `syncNow()`.
  Felder eines Wunsches: `id, titleKey, media, seasons[], seasonState{}, state, detail, failReason, origin
  ('slouchcat'|'jellyseerr'), seerrUser, seerrRequestId, owner (Profil-ID oder null), joined[], created, closed, events[{at, k, t}]`.
  Konstanten: `STATE_LABEL`, `STATE_ICON`, `OPEN_STATES`, `FAIL_TEXT`, `EVENT_LABEL`, `CHANNEL_LABEL`.
- Jellyseerr: `seerrState()` → `{k: 'ok'|'none'|'off'|'unreachable'|'badkey'|'csrf', label, chip, dot}`, `logSeerr(op, res, text)`.
- Benachrichtigungen: `S.notifications` (`profileId, kind, titleKey, wishId, season, at, read`), `notifText(n)` →
  `{title, ctx, href}`, `unread(p)`, `notify(profilId, art, wunsch, extra)`.

## Klassen (Auswahl)

Gemeinsam (`app.css`): `.btn` + `.primary|.secondary|.ghost|.danger|.accent|.state`, Grössen `.lg|.sm`, `.icon` (rund),
`.chip` + `.ok|.warn|.bad|.info|.blue|.outline`, `.dot` + `.ok|.warn|.bad`, `.ws.<zustand>` (Zustand eines Wunsches),
`.fld` (Feld mit `label` und `.hint`), `.inp`, `.sel`, `.sw`, `.check` (Checkbox/Radio mit Text), `.seg` (Segmentwahl,
`aria-pressed`), `.pills`/`.pill` (Filter, `aria-pressed`), `.notice`, `.dlg`, `.facts`/`.f`, `.muted`, `.sec`, `.lnk`,
`.row-gap`, `.stack`, `.grow`, `.right`, `.mono`, `.empty`, `.spin`.
Zuschauer-App (`viewer.css`): `.page`/`.head`/`h1`/`.sub`/`.acts`, `.panel` (r=20), `.bandp` (über `band(..., {viewer:true})`),
`.grid` (Posterraster aus `posterCard`), `.rowhead`, `.scroller`, `.det` (Detailseite: `.bd`, `.back`, `.lay`, `.poster`,
`.info`, `.orig`, `.genres`, `.tagline`, `.overview`, `.rounds`/`.round` (+`.on`, `.hi`), `.body`, `.gutter`, `.seclabel`,
`.swall`, `.pc.ghost`, `.eprows`/`.eprow`, `.person`), `.lrow` (Listenzeile: `.thumb`, `.bd2`, `.ttl`, `.ln`, `.ra`,
`.unread`), `.steps` (Verlauf: `li.done|.now|.bad` mit `<time>`), `.meta`, `.av`.
Verwaltung (`admin.css`, unter `.adm`): `.card` (+`.ch` mit `.ik` und `h2`, `.foot`), `.cols2`, `.tiles`/`.tile` (+`.tt`,
`.tm`, `.sts`, `.arr`, `.new`), `.stats`/`.stat` (+`.v`, `.l`, `.n`; `--n` = Spaltenzahl), `.band` (über `band()`),
`.srow` (Einstellzeile: `.st` mit `<b>` und `<span>` Hilfe, `.sc` Steuerung; `.new` kennzeichnet Neues), `.tblw`/`.tbl`
(Tabelle; `td.main`, `.end`, `tr.sub`, `.tog` Aufklapp-Knopf mit `aria-expanded`, `.ta` Aktionen, `.mini` Poster+Text),
`.subtabs` (Unterreiter als Pillen, Zähler `.c`), `.kv` (Beschreibungsliste), `.checks` (Prüfliste `li.ok|.bad|.warn|.wait`),
`.feed` (Protokoll mit `<time>`), `.savebar`, `.unchanged`, Knöpfe `.btn.primary` (Akzent), `.btn.quiet`, `.btn.dq` (ruhig rot).

## Fassung 3 (`js/model2.js`, `css/wishhub.css`): Jellyseerr-Benutzer je Konto, Limit, Profil-Tags, Entdecken

Skriptreihenfolge neu: `data.js`, `data-discover.js`, `core.js`, `store.js`, `model2.js`, `shell.js`, `home.js`,
`viewer-titles.js`, `viewer-pages.js`, `admin-a.js`, `admin-b.js`, `viewer-discover.js`, `viewer-account.js`, `mock.js`.
`model2.js` gehört zum Kern (nicht ändern). Gespeicherter Stand: `S.v === 5`.

- **Titel:** `window.TITLES` enthält jetzt 82 Titel (34 + 48 aus `data-discover.js`). Neue Felder: `t.tvdbId`, `t.imdbId`
  (oder `null`), `t.ageUnknown` (Freigabe unbekannt, zählt als 18), bei den neuen Titeln `t.release` (ISO-Datum), `t.score`.
  `idLine(t)` → «TMDB 1 · TVDB 2 · IMDb tt3». `ID_MATCH[key]` / `ID_MATCH_TEXT[...]`: wie eine Serie der Trakt-Liste erkannt wurde.
- **Entdecken:** `discoverOn(p)` (Seiten gibt es nur für ein Profil, das anfordern darf), `DISC_ROWS` (`[key, titel, href]`),
  `discoverRow(key, p)`, `discoverList('movie'|'series', p, {genre, sort: 'popular'|'new'|'title', hidePresent})`,
  `discoverGenres(kind, p)`, `discoverSearch(q, p)` (auch `tmdb:123`, `tvdb:123`, `imdb:tt…`), `wishNav(aktiv)` mit
  `aktiv` = `discover|movies|series|search|mine` (Reiter + Limit-Anzeige; steht auf jeder Seite unter «Wünsche» zuoberst).
  Schalter: `S.settings.wishes.fromDiscover`.
- **Limit:** ab Werk `S.settings.wishes.limit = { movie: {n, days}, tv: {n, days} }` (`n` 0 = ohne Limit); je Konto
  `account.limit = { mode: 'default'|'custom'|'unlimited', movie?, tv? }`. `limitOf(acc)`, `quotaFor(seerrUser)` →
  `{ source: 'admin'|'account'|'seerr', unlimited, movie: {limit, days, used, remaining, restricted, nextFree, direct, entries[]}, tv: {…} }`,
  `quotaOfProfile(p)`, `limitCheck(p, t, einheiten)` → `{ok, text}`, `quotaText(x, 'movie'|'tv')`, `quotaMeter(x, kind)`,
  `quotaPill(p)`, `whenFree(ts)`, `linkedAccount(seerrUser)`. Jellyseerr zählt: Filme einzeln, Serien je Staffel,
  rollierend über `days` Tage, auch direkt in Jellyseerr gestellte Wünsche (`seerrUser.direct[]`); abgelehnte,
  zurückgezogene und Mitwünsche zählen nicht; Administratoren haben kein Limit.
- **Jellyseerr-Benutzer des Kontos:** `accSeerr(acc)` → `{ state: 'created'|'linked'|'manual'|'waiting'|'none', user, pw:
  'synced'|'next_login'|'off'|'admin'|'external', at, pwAt, info: {label, chip}, pwInfo: {label, chip} }`. Aktionen:
  `provisionAccount(accId, {password})` (verknüpft bei gleicher E-Mail, sonst legt an), `provisionAll()`,
  `linkAccount(accId, userId)`, `unlinkAccount(accId)`, `syncRights(accId)` (nach Ändern des Wunschrechts), `pwChanged(accId)`
  (Passwort in Slouchcat geändert), `pwSignIn(accId)`, `pwSetMode(accId, an)`, `acceptInvitation(einladungsId, {email, name})`
  → `{accountId, profileId}`. Schalter: `S.settings.wishes.provision`, `S.settings.wishes.pwSync`. `seerrUser`-Felder neu:
  `origin: 'slouchcat'|'seerr'`, `admin`, `direct[]`. `S.seerr.localLogin`, `S.seerr.seasonsFrom` (`'tmdb'|'tvdb'`).
- **Profil-Tag:** `S.settings.wishes.tags = { enabled, prefix }`, `tagLabel(p)` (eigener Name: `p.tagName`), `wishTags(w)` →
  `[{profile, label, state: 'set'|'fail'|'wait'}]`, `tagChips(w)`, `applyTags(w)`, `S.arr.radarr` / `S.arr.sonarr`
  (`name, url, own, condition: 'ok'|'unreachable', tagRequests`), `arrOf(w)`. Ereignisse am Wunsch: `tagged`, `tag_failed`.
- **Klassen (`wishhub.css`):** `.whead`, `.wtabs`, `.quota`, `.qm` (+ `.full`, `.none`, `.bar`).

## Nachtrag (zweite Rückmeldung vom 17. November 2026)

- **Spotlight:** Startseite und Spotlight-Liste zeigen nur Titel der Bibliothek (`spotlightFor(p)` liefert nie fehlende
  Titel). Fehlende Einträge der Trakt-Liste sieht nur die Verwaltung (M-27). Die Schalter `fromSpotlight`,
  `showMissing`, `maxMissing`, `missingOrder` gibt es nicht mehr. Die Startreihe `wishes` heisst «Erfüllte Wünsche» und
  zeigt nur Wünsche, die in den letzten 14 Tagen wahr wurden.
- **Webhooks von Radarr und Sonarr:** bleiben, wie sie heute sind (von Hand eingerichtet, Seite unverändert).
- **Download-Fortschritt (nur lesend, nur bei Wünschen):** Schalter `S.settings.wishes.progress`, Job `wishes.progress`
  (jede Minute). `dlInfo(w)` → `{ k: 'search'|'download'|'import'|'stalled'|'unknown', pct?, eta?, label, admin, arr }`
  oder `null`; `dlChip(w, admin?)` (Zeile in Listen), `dlBlock(w, admin?)` (Block mit Balken), `dlBar(x)`,
  `dlStep(id)`, `dlStall(id)`, `progressOn()`. Zustand am Wunsch: `w.dl = { step, stalled?, msg?, eps? }`. Ist Radarr
  oder Sonarr nicht erreichbar, zeigt die Zuschauer-App nichts (Wunsch bleibt «In Beschaffung»), die Verwaltung
  «Fortschritt nicht lesbar». Klassen: `.dl`, `.dlb` (+ `.search|.download|.import|.stalled|.unknown`).
