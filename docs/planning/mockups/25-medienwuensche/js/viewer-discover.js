/* M-31 Entdecken, M-32 Filme, M-33 Serien, M-34 Suche: the area «Wünsche» like the start page of Seerr.
   Global helpers of this file carry the prefix «dc». State lives in ui.disc (never persisted). */
'use strict';


const DC_PAGE = 24;
const DC_SORTS = [['popular', 'Beliebtheit'], ['new', 'Neueste zuerst'], ['title', 'Titel A–Z']];
const DC_KIND = {
  movie: { path: '/discover/movies', tab: 'movies', h1: 'Filme', one: 'Film', many: 'Filme', sub: 'Alle Filme, die TMDB kennt, ob schon in der Bibliothek oder nicht. Was fehlt, kannst du anfordern.' },
  series: { path: '/discover/series', tab: 'series', h1: 'Serien', one: 'Serie', many: 'Serien', sub: 'Alle Serien, die TMDB kennt, ob schon in der Bibliothek oder nicht. Was fehlt, kannst du anfordern.' },
};
const dcSortLabel = (v) => (DC_SORTS.find((x) => x[0] === v) || DC_SORTS[0])[1];
const dcGridDefault = () => ({ sort: 'popular', genre: '', hide: false, n: DC_PAGE });

/* ui.disc with defaults (created on first use; ui is not persisted) */
function dcUi() {
  const d = (ui.disc = ui.disc || {});
  if (d.q == null) d.q = '';
  if (d.home == null) d.home = '';
  d.seen = d.seen || {};
  d.preset = d.preset || {};
  ['movie', 'series'].forEach((k) => { d[k] = { ...dcGridDefault(), ...(d[k] || {}) }; });
  return d;
}
const dcOffline = () => seerrState().k !== 'ok';
const dcStale = () => notice('warn', 'Jellyseerr ist nicht erreichbar', 'Jellyseerr ist nicht erreichbar. Du siehst den Stand vom letzten Abruf. Wünsche werden vorgemerkt.');

/* the pages exist only for a profile that may request; with own wishes it gets a pointer to «Meine Wünsche» */
function dcGate(p) {
  if (discoverOn(p)) return null;
  if (!myWishes(p).length) return vtNotFound();
  const cr = canRequest(p);
  const why = S.settings.wishes.fromDiscover === false ? 'Die Seiten zum Entdecken sind ausgeschaltet.' : esc(cr.text || 'Dieses Profil kann keine Titel anfordern.');
  return { type: 'viewer', nav: 'wishes', title: 'Wünsche', html: `<div class="page dc-scope">${wishNav('')}
    <div class="head"><h1>Wünsche</h1><p class="sub">Deine bisherigen Wünsche findest du weiterhin unter «Meine Wünsche».</p></div>
    ${notice('', 'Entdecken gibt es für dieses Profil nicht', why, `<a class="btn secondary" href="#/wishes" data-fid="dc:gate:mine">${icon('wish')}Zu «Meine Wünsche»</a>`)}</div>` };
}

/* ======================================================================================
   M-31 Entdecken
   ====================================================================================== */
function dcGenreRow(kind, p) {
  const list = discoverGenres(kind, p).map((g) => [g, discoverList(kind, p, { genre: g }).length]).filter((x) => x[1]);
  if (!list.length) return '';
  const key = 'g-' + kind, title = kind === 'movie' ? 'Film-Genres' : 'Serien-Genres', base = '#' + DC_KIND[kind].path;
  return `<section class="row dc-genres" aria-label="${title}"><div class="rowhead"><h2>${title}</h2><div class="paging"><button type="button" class="btn ghost icon" data-act="scrollRow" data-row="${key}" data-dir="prev" aria-label="Zurückblättern" data-fid="rp:${key}">${icon('chevL')}</button><button type="button" class="btn ghost icon" data-act="scrollRow" data-row="${key}" data-dir="next" aria-label="Weiterblättern" data-fid="rn:${key}">${icon('chevR')}</button></div></div>
    <div class="scroller" data-keep="${key}"><ul>${list.map(([g, n]) => `<li><a class="dc-tile ${kind === 'movie' ? 'm' : 's'}" href="${base}?genre=${encodeURIComponent(g)}" data-fid="dc:g:${kind}:${esc(g)}"><b>${esc(g)}</b><span>${n} ${n === 1 ? DC_KIND[kind].one : DC_KIND[kind].many}</span></a></li>`).join('')}</ul></div></section>`;
}
route('/discover', () => {
  const p = curProfile(), gate = dcGate(p);
  if (gate) return gate;
  const d = dcUi();
  const rows = (keys) => DISC_ROWS.filter(([k]) => keys.includes(k)).map(([k, t, h]) => rowHtml(k, t, h, discoverRow(k, p), { ctx: 'd-' + k, libMark: true })).join('');
  /* order like the start page of Seerr: genres between the popular and the upcoming row of their kind */
  const body = rows(['trending', 'popularMovies']) + dcGenreRow('movie', p) + rows(['upcomingMovies', 'popularShows']) + dcGenreRow('series', p) + rows(['upcomingShows']);
  return { type: 'viewer', nav: 'wishes', title: 'Entdecken', html: `<div class="page dc-scope dc-home">${wishNav('discover')}
    <div class="head"><h1>Entdecken</h1><p class="sub">Was gerade beliebt ist und was bald erscheint. Was noch nicht in der Bibliothek ist, kannst du anfordern.</p>
      <div class="dc-q" role="search"><span class="dc-qi">${icon('search')}<input class="inp" type="search" id="dc-home-q" data-bind="ui.disc.home" data-enter="dcHomeGo" data-fid="dc:home:q" value="${esc(d.home)}" placeholder="Film oder Serie suchen" aria-label="Film oder Serie suchen" autocomplete="off"></span><button type="button" class="btn primary" data-act="dcHomeGo" data-fid="dc:home:go">${icon('search')}Suchen</button></div>
    </div>
    ${dcOffline() ? dcStale() : ''}
    <div class="dc-rows">${body || `<div class="empty">Im Moment gibt es hier nichts zu entdecken.</div>`}</div>
  </div>` };
});
ACT.dcHomeGo = () => { const q = (dcUi().home || '').trim(); go('#/discover/search?q=' + encodeURIComponent(q)); };

/* ======================================================================================
   M-32 Filme, M-33 Serien
   ====================================================================================== */
function dcHash(kind, pr) {
  const qs = [pr.genre ? 'genre=' + encodeURIComponent(pr.genre) : '', pr.sort ? 'sort=' + pr.sort : ''].filter(Boolean).join('&');
  return '#' + DC_KIND[kind].path + (qs ? '?' + qs : '');
}
function dcGridPage(kind, query) {
  const p = curProfile(), gate = dcGate(p);
  if (gate) return gate;
  const K = DC_KIND[kind], d = dcUi(), st = d[kind];
  /* a query preset applies once when the hash query changes; afterwards the user's choice wins */
  const qGenre = query.genre || '', qSort = DC_SORTS.some((x) => x[0] === query.sort) ? query.sort : '';
  const seen = qGenre + '|' + qSort;
  if (d.seen[kind] !== seen) {
    d.seen[kind] = seen;
    if (qGenre) st.genre = qGenre;
    if (qSort) st.sort = qSort;
    if (qGenre || qSort) st.n = DC_PAGE;
    d.preset[kind] = { genre: qGenre, sort: qSort };
  }
  const pr = d.preset[kind] || {};
  const genres = discoverGenres(kind, p);
  if (st.genre && !genres.includes(st.genre)) genres.unshift(st.genre);
  const list = discoverList(kind, p, { genre: st.genre, sort: st.sort, hidePresent: st.hide });
  const inLib = list.filter((t) => isPresent(t.key)).length;
  const shown = list.slice(0, st.n);
  const P = `ui.disc.${kind}`;

  const meta = `<span>${esc([dcSortLabel(st.sort), st.genre || 'Alle Genres', st.hide ? 'Vorhandene ausgeblendet' : 'Vorhandene sichtbar'].join(' · '))}</span>`;
  const body = `<div class="dc-filters">
      <div class="fld"><label for="dc-sort-${kind}">Sortierung</label>${selectEl(P + '.sort', DC_SORTS.map(([v, t]) => ({ v, t })), { label: 'Sortierung', after: 'dcResetN' }).replace('<select ', `<select id="dc-sort-${kind}" `)}</div>
      <div class="fld"><label for="dc-genre-${kind}">Genre</label>${selectEl(P + '.genre', [{ v: '', t: 'Alle Genres' }, ...genres.map((g) => ({ v: g, t: g }))], { label: 'Genre', after: 'dcResetN' }).replace('<select ', `<select id="dc-genre-${kind}" `)}</div>
      <div class="dc-sw">${sw(P + '.hide', 'Vorhandene ausblenden', { after: 'dcResetN' })}<span>Vorhandene ausblenden<small>Zeigt nur, was noch nicht in der Bibliothek ist.</small></span></div>
    </div>
    <div class="acts"><button type="button" class="btn ghost" data-act="dcReset" data-kind="${kind}" data-fid="dc:${kind}:reset">${icon('refresh')}Filter zurücksetzen</button></div>`;

  const pills = [];
  if (pr.genre && st.genre === pr.genre) pills.push(`<button type="button" class="pill dc-pre" aria-pressed="true" data-act="dcDropPreset" data-kind="${kind}" data-p="genre" aria-label="Vorgabe Genre ${esc(pr.genre)} entfernen" data-fid="dc:${kind}:pre:genre">Genre: ${esc(pr.genre)}${icon('x', 14)}</button>`);
  if (pr.sort && st.sort === pr.sort && pr.sort !== 'popular') pills.push(`<button type="button" class="pill dc-pre" aria-pressed="true" data-act="dcDropPreset" data-kind="${kind}" data-p="sort" aria-label="Vorgabe Sortierung ${dcSortLabel(pr.sort)} entfernen" data-fid="dc:${kind}:pre:sort">Sortierung: ${dcSortLabel(pr.sort)}${icon('x', 14)}</button>`);

  const count = `${list.length} ${list.length === 1 ? K.one : K.many}${st.hide ? ', vorhandene ausgeblendet' : inLib ? `, davon ${inLib} in der Bibliothek` : ''}`;
  const grid = list.length
    ? `<ul class="grid" aria-label="${K.h1}">${shown.map((t) => posterCard(t, { ctx: 'g' + kind, libMark: true })).join('')}</ul>
       ${list.length > shown.length ? `<div class="dc-more"><span>${shown.length} von ${list.length} angezeigt</span><button type="button" class="btn secondary" data-act="dcMore" data-kind="${kind}" data-fid="dc:${kind}:more">${icon('plus')}Mehr laden</button></div>` : ''}`
    : `<div class="empty dc-empty"><b>Keine ${K.many} für diese Auswahl</b><p>Versuche ein anderes Genre oder blende vorhandene Titel wieder ein.</p><div class="acts"><button type="button" class="btn secondary" data-act="dcReset" data-kind="${kind}" data-fid="dc:${kind}:reset2">${icon('refresh')}Filter zurücksetzen</button></div></div>`;

  return { type: 'viewer', nav: 'wishes', title: K.h1, html: `<div class="page dc-scope dc-gridpage">${wishNav(K.tab)}
    <div class="head"><h1>${K.h1}</h1><p class="sub">${K.sub}</p></div>
    ${dcOffline() ? dcStale() : ''}
    ${band('dc-f-' + kind, 'Filter und Sortierung', meta, body, { viewer: true, icon: 'settings' })}
    <div class="dc-res">
      ${pills.length ? `<div class="pills dc-pres" role="group" aria-label="Vorgaben aus dem Link">${pills.join('')}</div>` : ''}
      <p class="dc-count" role="status">${count}</p>
      ${grid}
    </div>
  </div>` };
}
route('/discover/movies', (pm, query) => dcGridPage('movie', query));
route('/discover/series', (pm, query) => dcGridPage('series', query));
const dcKindOf = (d) => (d.kind || String(d.bind || d.path || '').split('.')[2]);
ACT.dcResetN = (d) => { const k = dcKindOf(d); if (DC_KIND[k]) dcUi()[k].n = DC_PAGE; };
ACT.dcMore = (d) => { const st = dcUi()[d.kind]; st.n += DC_PAGE; render(); };
ACT.dcReset = (d) => {
  const u = dcUi();
  u[d.kind] = dcGridDefault();
  u.preset[d.kind] = {};
  toast('Filter zurückgesetzt.', 'ok', 2500);
  go('#' + DC_KIND[d.kind].path);
};
ACT.dcDropPreset = (d) => {
  const u = dcUi(), st = u[d.kind], pr = { ...(u.preset[d.kind] || {}) };
  if (d.p === 'genre') st.genre = '';
  if (d.p === 'sort') st.sort = 'popular';
  delete pr[d.p];
  st.n = DC_PAGE;
  u.preset[d.kind] = pr;
  u.seen[d.kind] = (pr.genre || '') + '|' + (pr.sort || '');
  go(dcHash(d.kind, pr));
};

/* ======================================================================================
   M-34 Suche
   ====================================================================================== */
function dcChip(p, t) {
  const a = availability(p, t);
  if (a.kind === 'present') return `<span class="chip ok">${icon('check', 14)}In der Bibliothek</span>`;
  if (a.kind === 'partial') return `<span class="ws missing">${libSeasons(t.key).length} von ${airedSeasons(t).length} Staffeln vorhanden</span>`;
  if (a.kind === 'wished') return a.mine ? wsChip(a.wish.state) : wsChip('queued', 'Bereits angefordert');
  return missingChip();
}
function dcHit(p, t) {
  const href = titleHref(t);
  const meta = [t.year, t.kind === 'movie' ? 'Film' : 'Serie', t.ageUnknown ? 'Freigabe unbekannt, zählt als ab 18' : ageLabel(t.age)].filter(Boolean).map(esc).join(' · ');
  return `<li class="lrow dc-hit">
    <a class="thumb" href="${href}" tabindex="-1" aria-hidden="true" data-fid="dc:s:th:${t.key}">${img(t.poster, t.title)}</a>
    <div class="bd2">
      <a class="ttl dc-tl" href="${href}" data-fid="dc:s:tt:${t.key}">${esc(t.title)}</a>
      <span class="ln">${meta}</span>
      ${t.original && t.original !== t.title ? `<span class="ln">Originaltitel: ${esc(t.original)}</span>` : ''}
      <div class="dc-tags">${dcChip(p, t)}</div>
    </div>
    <div class="ra dc-acts">${mainActions(t, { lg: false, details: false, ctx: 's-' + t.key })}</div>
  </li>`;
}
function dcExamples() {
  const tk = T['tulsa-king'];
  const im = DISC_ORDER.map((k) => T[k]).find((t) => t && t.kind === 'movie' && t.imdbId);
  return [['tmdb:1368337'], tk && tk.tvdbId ? ['tvdb:' + tk.tvdbId] : null, im ? ['imdb:' + im.imdbId] : null].filter(Boolean)
    .map(([q], i) => `<button type="button" class="lnk dc-ex mono" data-act="dcFill" data-q="${esc(q)}" data-fid="dc:ex:${i}">${esc(q)}</button>`).join('<span aria-hidden="true"> · </span>');
}
route('/discover/search', (pm, query) => {
  const p = curProfile(), gate = dcGate(p);
  if (gate) return gate;
  const d = dcUi();
  const fresh = typeof lastPath === 'undefined' || lastPath !== '/discover/search';
  if (query.q != null && d.seen.q !== query.q) { d.seen.q = query.q; d.q = query.q; }
  const q = (d.q || '').trim();
  const off = dcOffline();
  let res = '';
  if (off) {
    res = notice('warn', 'Jellyseerr ist nicht erreichbar', 'Die Suche fragt Jellyseerr direkt und geht erst wieder, wenn Jellyseerr antwortet. Die Reihen unter «Entdecken» zeigen den Stand vom letzten Abruf.', `<a class="btn secondary" href="#/discover" data-fid="dc:s:off">${icon('star')}Zu «Entdecken»</a>`);
  } else if (!q) {
    const sug = (discoverRow('trending', p).length ? discoverRow('trending', p) : DISC_ORDER.map((k) => T[k]).filter((t) => t && titleVisible(p, t))).slice(0, 6);
    res = `<p class="dc-status" role="status">Gib einen Titel ein. Gesucht wird in allem, was TMDB kennt, auch nach Originaltitel.</p>
      ${sug.length ? `<section class="dc-sec" aria-labelledby="dc-s-sug"><h2 id="dc-s-sug">Gerade beliebt</h2><ul class="grid">${sug.map((t) => posterCard(t, { ctx: 'sg', libMark: true })).join('')}</ul></section>` : ''}`;
  } else if (q.length < 2) {
    res = `<p class="dc-status" role="status">Gib mindestens zwei Zeichen ein.</p>`;
  } else {
    const hits = discoverSearch(q, p);
    const mv = hits.filter((t) => t.kind === 'movie'), sr = hits.filter((t) => t.kind === 'series');
    if (!hits.length) {
      res = `<p class="dc-status" role="status">0 Treffer</p><div class="empty dc-empty"><b>Nichts gefunden für «${esc(q)}»</b><p>Gesucht wird nach Film- und Serientiteln, nicht nach Personen. Versuche einen anderen Begriff oder eine Kennung.</p></div>`;
    } else {
      const sec = (id, label, list) => (list.length ? `<section class="dc-sec" aria-labelledby="dc-s-${id}"><h2 id="dc-s-${id}">${label} (${list.length})</h2><ul class="dc-list">${list.map((t) => dcHit(p, t)).join('')}</ul></section>` : '');
      res = `<p class="dc-status" role="status">${hits.length} Treffer für «${esc(q)}»</p>${sec('m', 'Filme', mv)}${sec('s', 'Serien', sr)}`;
    }
  }
  return {
    type: 'viewer', nav: 'wishes', title: 'Suche',
    html: `<div class="page dc-scope dc-search">${wishNav('search')}
      <div class="head"><h1>Suche</h1><p class="sub">Suche in allem, was TMDB kennt, auch in dem, was noch fehlt.</p>
        <div class="dc-qi dc-big">${icon('search')}<input class="inp" type="search" data-bind="ui.disc.q" data-live data-autofocus data-fid="dc:s:q" value="${esc(d.q)}" placeholder="Film oder Serie" aria-label="Film oder Serie suchen" aria-describedby="dc-s-hint" autocomplete="off" ${off ? 'disabled' : ''}></div>
        <p class="dc-hint" id="dc-s-hint">Auch nach Kennung: ${dcExamples()}</p>
      </div>
      ${res}
    </div>`,
    after(root) {
      const u = dcUi();
      if (!fresh && !u.focus) return;
      u.focus = false;
      const el = root.querySelector('[data-autofocus]');
      if (el && !el.disabled) { el.focus({ preventScroll: true }); const n = el.value.length; try { el.setSelectionRange(n, n); } catch (e) { /* not a text field */ } }
    },
  };
});
ACT.dcFill = (d) => { const u = dcUi(); u.q = d.q; u.focus = true; render(); };
