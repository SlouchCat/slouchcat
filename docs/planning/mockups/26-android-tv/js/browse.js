/* Browsing pages and the video player of the TV mockup.
   Routes: /library/:key, /item/:i, /person/:id, /search, /list/:key, /watchlist, /watched,
   /collections, /collection/:id, /play/:i. Global names of this file carry the prefix br / BR. */
'use strict';

/* ---------- data the mockup adds ----------
   The server delivers genres only for the titles with details (D(i).g). For the filters to work on
   every title, the other titles get their usual genres here (same list as the phone mockup). */
const BR_GEN = (() => {
  const src = {
    'Abenteuer,Drama': [5], 'Komödie,Science Fiction': [6], 'Drama,Thriller': [70, 76], 'Thriller,Science Fiction,Horror': [71],
    'Komödie,Liebesfilm': [72, 55, 20], 'Liebesfilm,Drama': [73], 'Krimi,Drama': [74, 82, 14, 17, 18, 50, 42], 'Drama,Historie': [75],
    'Abenteuer,Historie,Drama': [77], 'Drama': [78, 85, 16, 19, 25, 41], 'Drama,Science Fiction': [79], 'Action,Krimi': [80],
    'Action,Science Fiction': [81, 26, 31], 'Komödie,Action': [83, 84], 'Horror,Science Fiction': [86, 28, 51, 52],
    'Horror,Action': [21, 23], 'Thriller,Drama': [24, 27], 'Horror': [53, 54], 'Horror,Thriller': [107], 'Thriller': [36, 93],
    'Science Fiction,Abenteuer': [30], 'Dokumentarfilm,Abenteuer': [104], 'Dokumentarfilm': [105], 'Dokumentarfilm,Historie': [106],
    'Reality': [108, 109, 35], 'Reality,Dokumentarfilm': [15], 'Animation,Action & Adventure': [12, 22], 'Drama,Western': [13, 49],
    'Sci-Fi & Fantasy,Action & Adventure': [29], 'Drama,Mystery': [39], 'Komödie': [40, 45],
    'Komödie,Drama': [43, 47], 'Krimi,Mystery': [44], 'Komödie,Sci-Fi & Fantasy': [46], 'Sci-Fi & Fantasy,Drama': [48],
    'Science Fiction': [32, 33, 34], 'Sachbuch': [87, 88, 94, 96], 'Fantasy,Lyrik': [89], 'Roman': [90, 92], 'Krimi': [91],
    'Ratgeber': [95], 'Gesellschaft': [56, 61, 62, 100], 'Nachrichten': [57, 102], 'True Crime': [58],
    'Politik': [59, 69, 98], 'Gesundheit': [60, 63], 'Comedy': [64, 103], 'Wissen': [65, 66, 67, 68, 97, 99], 'Nachrichten,Comedy': [101],
  };
  const out = {};
  for (const [g, ids] of Object.entries(src)) for (const i of ids) out[i] = g.split(',');
  return out;
})();
const brGenres = i => { const d = D(i); return d && d.g ? d.g.slice(0, 3) : (BR_GEN[i] || []); };
const BR_ROLE = { director: 'Regie', writer: 'Drehbuch', creator: 'Idee', producer: 'Produktion' };
const brNf = n => Number(n).toLocaleString('de-CH');
const brIsSq = i => { const k = T(i).kind; return k === 'a' || k === 'p'; };
const brNorm = s => String(s || '').normalize('NFD').replace(/\p{Diacritic}/gu, '').toLowerCase();
const brTc = sec => { const s = Math.max(0, Math.round(sec)); const h = Math.floor(s / 3600); const m = Math.floor((s % 3600) / 60); const x = String(s % 60).padStart(2, '0'); return h ? `${h}:${String(m).padStart(2, '0')}:${x}` : `${m}:${x}`; };
const brClock = d => `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
const brOthers = () => S.profiles.filter((p, k) => k !== S.profile);
const BR_VC = { hevc: 'HEVC', h264: 'H.264', av1: 'AV1', vp9: 'VP9' };
const BR_AC = { eac3: 'Dolby Digital Plus', ac3: 'Dolby Digital', dts: 'DTS', truehd: 'Dolby TrueHD', aac: 'AAC' };
/* series carry no file in the test data; the player info then shows this typical file */
const BR_F0 = ['hevc', 1080, 'eac3', 0, 'mkv', 1];
const brFile = i => (D(i) && D(i).f) || BR_F0;
const brRes = h => `${h > 1100 ? 3840 : 1920} × ${h} · ${h > 1100 ? '4K' : h >= 700 ? 'Full HD' : 'SD'}`;
/** volatile state of this file */
const BR = { sum: '', at: 0 };
RESETS.push(() => { BR.sum = ''; delete ui.brLib; delete ui.brSeason; delete ui.brEdit; delete ui.brSym; ui.q = ''; BRP.key = null; brSyncCont(); });

/** empty state; to is a hash, or null for «Zurück»; btn replaces the button */
function brEmpty(ic, title, text, to, label, btn0) {
  const btn = btn0 || (to ? `<a class="tb primary" href="${to}" data-f="emp" data-default>${icon('right', 32)}<span>${esc(label)}</span></a>`
    : `<button type="button" class="tb primary" data-act="back" data-f="emp" data-default>${icon('back', 32)}<span>Zurück</span></button>`);
  return `<div class="br-empty"><span class="br-eic">${icon(ic, 80)}</span><h2>${esc(title)}</h2><p>${esc(text)}</p>${btn}</div>`;
}
const brEmptyPage = (ic, title, text, to, label) => `<div class="page br-fixed">${brEmpty(ic, title, text, to, label)}</div>`;
/** side-panel option with a check mark on the current choice (default focus) */
function brOpt(f, act, attrs, label, sub, on) {
  return `<button type="button" class="mi" role="menuitemradio" aria-checked="${!!on}" data-f="${f}" data-act="${act}" ${attrs}${on ? ' data-default' : ''}>
    <span class="mi-tx"><b>${esc(label)}</b>${sub ? `<small>${esc(sub)}</small>` : ''}</span>${on ? `<span class="br-mcheck">${icon('check', 36)}</span>` : ''}</button>`;
}

/* ---------- progress and episodes ----------
   S.brProg: { [i]: { t, pct, at, s?, k? } } for films and series; S.brEpSeen: keys of seen episodes. */
function brProg() {
  if (!S.brProg) {
    S.brProg = { 37: { s: 1, k: 2, t: Math.round(33 * 60 * 0.4), pct: 40, at: 1 } };
    S.brEpSeen = S.brEpSeen ?? ['01M7GJ8R88X7KCVT0PM1312ZAT', '01M7GJ8R83HZHTCNRNKDAM5BE8'];
  }
  return S.brProg;
}
const brEpSeen = () => S.brEpSeen ?? (S.brEpSeen = []);
const brEps = i => (D(i) && D(i).e) || [];
const brSeasonLen = (i, s) => { const se = ((D(i) || {}).s || []).find(x => x[0] === s); return se ? se[1] : (s === 1 ? brEps(i).length : 0); };
/** Episode k (0-based) of season s. Season 1 comes from D(i).e; later seasons reuse its stills (test data has no more). */
function brEp(i, s, k) {
  const list = brEps(i); if (!list.length || k < 0) return null;
  if (s === 1 && list[k]) { const e = list[k]; return { s, k, id: e[0], key: e[0], num: e[1], title: e[2], min: e[3], ov: e[4] }; }
  if (k >= brSeasonLen(i, s)) return null;
  const avg = Math.round(list.reduce((a, e) => a + e[3], 0) / list.length);
  return { s, k, id: list[k % list.length][0], key: `${i}:${s}:${k}`, num: k + 1, title: '', min: avg, ov: '' };
}
const brEpNext = (i, ep) => brEp(i, ep.s, ep.k + 1) || brEp(i, ep.s + 1, 0);
const brEpPrev = (i, ep) => (ep.k > 0 ? brEp(i, ep.s, ep.k - 1) : ep.s > 1 ? brEp(i, ep.s - 1, brSeasonLen(i, ep.s - 1) - 1) : null);
const brEpUrl = (i, ep) => `#/play/${i}?e=${ep.k}${ep.s > 1 ? `&s=${ep.s}` : ''}`;
const brEpLabel = ep => `S${ep.s} · F${ep.num}`;
const brEpName = ep => ep.title || `Folge ${ep.num}`;
const brIsEpSeen = (i, ep) => S.seen.includes(i) || brEpSeen().includes(ep.key);
/** what «Abspielen» does on a title: url, label, episode, stored progress */
function brResume(i) {
  const t = T(i); const p = brProg()[i];
  if (t.kind === 's' && brEps(i).length) {
    const n = (D(i) || {}).n;
    const ep = (p && brEp(i, p.s || 1, p.k || 0)) || (n && brEp(i, n[0], n[1] - 1)) || brEp(i, 1, 0);
    return { url: brEpUrl(i, ep), label: `${brEpLabel(ep)} ${p ? 'fortsetzen' : 'abspielen'}`, ep, p };
  }
  return { url: `#/play/${i}`, label: p ? 'Fortsetzen' : 'Abspielen', ep: null, p };
}

/* «Weiterschauen» on the start page reads DATA.R.cont; stored progress goes in front of it. */
const BR_CONT0 = DATA.R.cont.slice();
function brSyncCont() {
  const P = brProg(); const ids = Object.keys(P).map(Number).sort((a, b) => (P[b].at || 0) - (P[a].at || 0));
  DATA.R.cont.splice(0, DATA.R.cont.length, ...ids.map(i => [i, Math.max(1, Math.round(P[i].pct))]), ...BR_CONT0.filter(([i]) => !(i in P) && !S.seen.includes(i)));
}
brSyncCont();

function brMoreLike(i) {
  const g = brGenres(i); const lib = LIBS.find(L => L.ids().includes(i));
  const pool = (lib ? lib.ids() : DATA.T.map((x, j) => j).filter(j => T(j).kind === T(i).kind)).filter(j => j !== i);
  const score = j => brGenres(j).filter(x => g.includes(x)).length;
  return pool.map((j, n) => [j, score(j), n]).sort((a, b) => b[1] - a[1] || a[2] - b[2]).slice(0, 12).map(x => x[0]);
}
function brPeople() {
  const m = new Map();
  for (const [k, d] of Object.entries(DATA.D)) for (const [n, r, id] of d.c || []) { const p = m.get(id) || { id, name: n, roles: [] }; p.roles.push([Number(k), r]); m.set(id, p); }
  return [...m.values()];
}

/* ---------- grid pages: fixed header, grid that scrolls by rows ---------- */
function brGrid(ids, key, sq, opts) {
  const op = i => Object.assign({ row: key }, typeof opts === 'function' ? opts(i) : (opts || {}));
  return `<div class="br-gw" data-scroll><div class="br-grid${sq ? ' sq' : ''}">${ids.map(i => card(i, op(i))).join('')}</div></div>`;
}
/** onFocus of grid pages: the focused card's row moves to the top of the grid; controls show the summary */
function brGridFocus(el) {
  const gw = el.closest('.br-gw');
  if (!gw) { const ri = $('.br-rinfo'); if (ri && el.dataset.tt == null && BR.sum) ri.innerHTML = BR.sum; return; }
  gw.scrollTo({ top: Math.max(0, el.offsetTop - 22), behavior: Date.now() - BR.at < 200 ? 'auto' : 'smooth' });
}
const brGridAfter = () => { BR.at = Date.now(); };

/* ============================================================
   Library
   ============================================================ */
const BR_SORTS = [['added', 'Zuletzt hinzugefügt'], ['title', 'Titel A–Z'], ['new', 'Neueste zuerst'], ['old', 'Älteste zuerst']];
const brLibSt = key => { ui.brLib = ui.brLib || {}; return ui.brLib[key] || (ui.brLib[key] = { f: 'all', g: '', y: '', sort: 'added' }); };
const brDecade = y => (!y ? '' : y < 1990 ? 'Vor 1990' : `${Math.floor(y / 10) * 10}er`);
const brAudioLib = L => L.key === 'books' || L.key === 'podcasts';
function brLibItems(L, st) {
  let ids = L.ids();
  if (st.f === 'unseen') ids = ids.filter(i => !S.seen.includes(i));
  if (st.g) ids = ids.filter(i => brGenres(i).includes(st.g));
  if (st.y) ids = ids.filter(i => brDecade(T(i).year) === st.y);
  const title = i => T(i).title.replace(/^\W+/, '');
  const by = { title: (a, b) => title(a).localeCompare(title(b), 'de'), new: (a, b) => T(b).year - T(a).year, old: (a, b) => T(a).year - T(b).year }[st.sort];
  return by ? ids.slice().sort(by) : ids;
}
const brCount = (ids, fn) => { const m = new Map(); ids.forEach(i => [].concat(fn(i)).filter(Boolean).forEach(v => m.set(v, (m.get(v) || 0) + 1))); return m; };
const brHasYears = L => L.ids().some(i => T(i).year);

route('/library/:key', p => {
  const L = LIBS.find(l => l.key === p.key);
  if (!L) return { nav: 'libs', html: brEmptyPage('grid', 'Diese Bibliothek gibt es nicht', 'Wähle eine Bibliothek in der Leiste links.', '#/', 'Zur Startseite') };
  const st = brLibSt(L.key); const ids = brLibItems(L, st); const audio = brAudioLib(L);
  const active = st.f !== 'all' || st.g || st.y;
  const sortL = (BR_SORTS.find(s => s[0] === st.sort) || BR_SORTS[0])[1];
  BR.sum = `<b>${active ? `${ids.length} ${ids.length === 1 ? 'Treffer' : 'Treffer'}` : 'Alle Titel'}</b><span>${esc(sortL)}</span>`;
  const cb = (m, ic, label, on) => `<button type="button" class="br-cb${on ? ' on' : ''}" data-f="lf-${m}" data-act="brLibMenu" data-k="${L.key}" data-m="${m}" aria-haspopup="dialog">${icon(ic, 32)}<span>${esc(label)}</span>${icon('down', 28)}</button>`;
  const ctrls = cb('f', 'eye', st.f === 'unseen' ? (audio ? 'Ungehört' : 'Ungesehen') : 'Alle', st.f !== 'all')
    + cb('g', 'filter', st.g || 'Genre', !!st.g)
    + (brHasYears(L) ? cb('y', 'clock', st.y || 'Jahr', !!st.y) : '')
    + cb('s', 'sort', sortL, st.sort !== 'added')
    + (active ? `<button type="button" class="br-cb ghost" data-f="lf-x" data-act="brLibReset" data-k="${L.key}">${icon('close', 30)}<span>Zurücksetzen</span></button>` : '');
  const empty = `<div class="br-gw">${brEmpty('filter', 'Keine Titel mit diesen Filtern', 'Ändere einen Filter oder setze alle zurück.', null, '',
    `<button type="button" class="tb primary" data-act="brLibReset" data-k="${L.key}" data-f="emp">${icon('refresh', 32)}<span>Filter zurücksetzen</span></button>`)}</div>`;
  return {
    nav: 'libs', onFocus: brGridFocus, after: brGridAfter,
    html: `<div class="page br-fixed" data-info-sec>
      <header class="br-head"><span class="br-hic">${icon(L.icon, 44)}</span><h1>${esc(L.name)}</h1><span class="br-count">${brNf(L.count)} Titel</span></header>
      <div class="br-ctrls">${ctrls}</div>
      <div class="rinfo br-rinfo" aria-live="polite">${BR.sum}</div>
      ${ids.length ? brGrid(ids, 'g', audio, i => (S.seen.includes(i) ? { tag: `${icon('check', 24)} ${audio ? 'Gehört' : 'Gesehen'}` } : {})) : empty}
    </div>`,
  };
});
ACT.brLibMenu = d => {
  const L = LIBS.find(l => l.key === d.k); const st = brLibSt(L.key); const all = L.ids(); const audio = brAudioLib(L);
  const o = (v, label, sub, cur) => brOpt(`lo-${slug(v || 'alle')}`, 'brLibSet', `data-k="${L.key}" data-m="${d.m}" data-v="${esc(v)}"`, label, sub, cur);
  const n = x => `${x} Titel`;
  let h = '';
  if (d.m === 'f') {
    const unseen = all.filter(i => !S.seen.includes(i)).length;
    h = `<h2>Anzeigen</h2><div class="mlist">${o('all', 'Alle', n(all.length), st.f === 'all')}${o('unseen', audio ? 'Ungehört' : 'Ungesehen', n(unseen), st.f === 'unseen')}</div>`;
  } else if (d.m === 'g') {
    const list = [...brCount(all, brGenres).entries()].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0], 'de'));
    h = `<h2>Genre</h2><div class="mlist">${o('', 'Alle Genres', n(all.length), !st.g)}${list.map(([g, c]) => o(g, g, n(c), st.g === g)).join('')}</div>`;
  } else if (d.m === 'y') {
    const order = y => (y === 'Vor 1990' ? 0 : parseInt(y, 10));
    const list = [...brCount(all, i => brDecade(T(i).year)).entries()].sort((a, b) => order(b[0]) - order(a[0]));
    h = `<h2>Jahr</h2><div class="mlist">${o('', 'Alle Jahre', n(all.length), !st.y)}${list.map(([y, c]) => o(y, y, n(c), st.y === y)).join('')}</div>`;
  } else {
    const sorts = brHasYears(L) ? BR_SORTS : BR_SORTS.slice(0, 2);
    h = `<h2>Sortierung</h2><div class="mlist">${sorts.map(([v, label]) => brOpt(`lo-${v}`, 'brLibSet', `data-k="${L.key}" data-m="sort" data-v="${v}"`, label, '', st.sort === v)).join('')}</div>`;
  }
  menu(h);
};
ACT.brLibSet = d => { const st = brLibSt(d.k); st[d.m === 's' ? 'sort' : d.m] = d.v; closeAll(); render(); };
ACT.brLibReset = d => { Object.assign(brLibSt(d.k), { f: 'all', g: '', y: '' }); render(); focusTo('lf-f'); };
PAGES.push(['#/library/films', 'Bibliothek Filme'], ['#/library/series', 'Bibliothek Serien'], ['#/library/books', 'Bibliothek Hörbücher (quadratisch)']);

/* ============================================================
   Detail page of a film, a series or an episode
   ============================================================ */
function brMeta(i) {
  const t = T(i); const d = D(i) || {}; const parts = [];
  if (t.year) parts.push(`<span>${t.year}</span>`);
  if (t.fsk) parts.push(fsk(t.fsk));
  if (t.kind === 's') parts.push(`<span>${d.s ? `${d.s.length} ${d.s.length === 1 ? 'Staffel' : 'Staffeln'}` : 'Serie'}</span>`);
  else if (t.min) parts.push(`<span>${t.kind === 'e' ? 'Folge · ' : ''}${dur(t.min)}</span>`);
  const g = brGenres(i).join(' · '); if (g) parts.push(`<span>${esc(g)}</span>`);
  return parts.join('<i class="sep" aria-hidden="true"></i>');
}
function brProgLine(i, r) {
  const p = r.p; if (!p) return '';
  const min = r.ep ? r.ep.min : T(i).min; const left = Math.max(1, Math.round(min * (100 - p.pct) / 100));
  const what = r.ep ? `${brEpLabel(r.ep)} · ${esc(brEpName(r.ep))} · ` : '';
  return `<div class="br-progline"><span class="br-pbar"><i style="width:${Math.max(3, p.pct)}%"></i></span><span>${what}noch ${left} Min.</span></div>`;
}
const brIb = (f, ic, label, attrs) => `<button type="button" class="br-ib" data-f="${f}" aria-label="${esc(label)}" ${attrs}>${icon(ic, 36)}<span class="br-ibl">${esc(label)}</span></button>`;
function brEpCards(i, s) {
  const P = brProg()[i]; const out = [];
  for (let k = 0; k < brSeasonLen(i, s); k++) {
    const ep = brEp(i, s, k); if (!ep) break;
    const seen = brIsEpSeen(i, ep); const cur = P && (P.s || 1) === s && P.k === k; const url = brEpUrl(i, ep);
    const sub = cur ? ` · noch ${Math.max(1, Math.round(ep.min * (100 - P.pct) / 100))} Min.` : seen ? ' · gesehen' : '';
    out.push(`<a class="br-ep" href="${url}" data-f="ep-${s}-${k}" data-play="${url}" data-es="${s}" data-ek="${k}">
      <span class="br-epim">${pic(`e-${ep.id}.webp`, brEpName(ep))}${seen ? `<span class="br-seen" aria-label="gesehen">${icon('check', 28)}</span>` : ''}${cur ? `<span class="prog"><i style="width:${Math.max(4, P.pct)}%"></i></span>` : ''}</span>
      <b>${ep.num}. ${esc(brEpName(ep))}</b><small>${ep.min} Min.${sub}</small></a>`);
  }
  return out.join('');
}
function brEpOv(i, s, k) {
  const ep = brEp(i, s, k); if (!ep) return '';
  return `<b>${brEpLabel(ep)} · ${esc(brEpName(ep))}</b><p>${esc(ep.ov || 'Für diese Folge liefert der Server noch keine Inhaltsangabe.')}</p>`;
}
const brSeasonOf = (i, r) => { ui.brSeason = ui.brSeason || {}; return ui.brSeason[i] || (r.ep ? r.ep.s : 1); };
function brEpSection(i, r) {
  const d = D(i); const seasons = d.s || [[1, brEps(i).length, 0]]; const s = brSeasonOf(i, r);
  const k0 = r.ep && r.ep.s === s ? r.ep.k : 0;
  return `<section class="rsec br-eps" data-sec="eps"><h2 class="rh">Folgen</h2>
    <div class="br-chips" data-row="seasons" data-enter>${seasons.map(([n, c]) => `<button type="button" class="br-chip${n === s ? ' on' : ''}" data-f="se-${n}" data-season="${n}" data-it="${i}" data-act="brSeason"${n === s ? ' aria-current="true" data-default' : ''}><b>Staffel ${n}</b><small>${c} Folgen</small></button>`).join('')}</div>
    <div class="trow br-eprow" data-row="eps" id="br-eprow" data-s="${s}">${brEpCards(i, s)}</div>
    <div class="br-epov" id="br-epov" aria-live="polite">${brEpOv(i, s, k0)}</div></section>`;
}
function brCast(i) {
  const d = D(i); if (!d || !d.c || !d.c.length) return '';
  const m = new Map();
  d.c.forEach(([n, r, id]) => { const x = m.get(id) || { id, n, r: [] }; x.r.push(BR_ROLE[r] || r); m.set(id, x); });
  return `<section class="rsec" data-sec="cast"><h2 class="rh">Besetzung und Team</h2><div class="trow br-castrow" data-row="cast">${[...m.values()].map(x => `<a class="br-cp" href="#/person/${x.id}" data-f="cp-${x.id}">
    <span class="br-cpim">${pic(`c-${x.id}.webp`, x.n)}</span><b>${esc(x.n)}</b><small>${esc(x.r.join(', '))}</small></a>`).join('')}</div></section>`;
}
route('/item/:i', p => {
  const i = Number(p.i);
  if (!DATA.T[i]) return { nav: 'libs', html: brEmptyPage('film', 'Diesen Titel gibt es nicht', 'Vielleicht wurde er aus der Bibliothek entfernt.', '#/', 'Zur Startseite') };
  const t = T(i);
  if (t.kind === 'a' || t.kind === 'p') return { nav: 'libs', html: '', after: () => go(href(i), { replace: true }) };
  const d = D(i) || {}; const r = brResume(i); const watched = S.seen.includes(i); const onWatch = S.watch.includes(i);
  const name = d.l ? `<img class="sp-logo br-logo" src="${IMG}l-${i}.webp" alt="${esc(t.title)}" onerror="this.replaceWith(Object.assign(document.createElement('h1'),{className:'sp-title br-title',textContent:this.alt}))">`
    : `<h1 class="sp-title br-title">${esc(t.title)}</h1>`;
  const hero = `<section class="br-hero" data-top>
      <div class="sp-kind">${KIND[t.kind]}${watched ? ` <span class="badge ok br-seenb">${icon('check', 26)} Gesehen</span>` : ''}</div>
      <div class="sp-name">${name}</div>
      <div class="sp-meta">${brMeta(i)}</div>
      ${d.t ? `<p class="br-tag">${esc(d.t)}</p>` : ''}
      ${d.o ? `<p class="br-ov">${esc(d.o)}</p>` : ''}
      ${brProgLine(i, r)}
      <div class="br-acts" data-row="acts">
        <a class="tb primary br-play" href="${r.url}" data-f="it-play" data-play="${r.url}" data-default>${icon('play', 34)}<span>${esc(r.label)}</span></a>
        ${brIb('it-watch', onWatch ? 'bookmarkOn' : 'bookmark', onWatch ? 'Gemerkt' : 'Merken', `data-act="watch" data-i="${i}" aria-pressed="${onWatch}"`)}
        ${brIb('it-seen', 'check', watched ? 'Gesehen' : 'Als gesehen markieren', `data-act="brSeen" data-it="${i}" aria-pressed="${watched}"`)}
        ${brIb('it-rec', 'heart', 'Empfehlen', `data-act="brRec" data-it="${i}"`)}
        ${t.kind === 'e' ? '' : brIb('it-trailer', 'film', 'Trailer', 'data-act="toast" data-msg="Trailer startet …"')}
        ${brIb('it-more', 'info', 'Mehr Infos', `data-act="brMore" data-it="${i}"`)}
      </div></section>`;
  const more = brMoreLike(i);
  const rows = (t.kind === 's' && brEps(i).length ? brEpSection(i, r) : '') + brCast(i) + (more.length ? hrow('Mehr davon', more, null, null, 'more') : '');
  return {
    nav: 'libs', cls: 'br-item', html: hero + rows,
    onFocus: el => {
      art(artOf(i), el.closest('.br-hero') ? 'hero' : 'dim');
      if (el.dataset.ek != null) { const o = $('#br-epov'); if (o) o.innerHTML = brEpOv(i, Number(el.dataset.es), Number(el.dataset.ek)); }
      if (el.dataset.season) brSeasonShow(i, Number(el.dataset.season));
    },
    key: name2 => {
      if (name2 !== 'play') return false;
      const el = F.el;
      if (el && el.dataset.play) { go(el.dataset.play); return true; }
      if (el && el.dataset.i != null && el.closest('.rsec')) return false;
      go(r.url); return true;
    },
    after: () => art(artOf(i), 'hero'),
  };
});
/** focus on a season chip shows that season's episodes */
function brSeasonShow(i, s) {
  ui.brSeason = ui.brSeason || {};
  const row2 = $('#br-eprow'); if (!row2 || (ui.brSeason[i] || 0) === s && row2.dataset.s === String(s)) return;
  ui.brSeason[i] = s; row2.dataset.s = String(s);
  row2.innerHTML = brEpCards(i, s); row2.scrollLeft = 0;
  $$('.br-chip').forEach(c => { const on = Number(c.dataset.season) === s; c.classList.toggle('on', on); c.toggleAttribute('data-default', on); if (on) c.setAttribute('aria-current', 'true'); else c.removeAttribute('aria-current'); });
  const o = $('#br-epov'); if (o) o.innerHTML = brEpOv(i, s, 0);
}
ACT.brSeason = d => { brSeasonShow(Number(d.it), Number(d.season)); focusTo(`ep-${d.season}-0`); };
ACT.brSeen = d => {
  const i = Number(d.it); const k = S.seen.indexOf(i);
  if (k < 0) { S.seen.push(i); delete brProg()[i]; } else S.seen.splice(k, 1);
  brSyncCont(); toast(k < 0 ? 'Als gesehen markiert' : 'Markierung «Gesehen» entfernt'); commit();
};
ACT.brRec = d => {
  const i = Number(d.it); const o = brOthers();
  menu(`<h2>Empfehlen</h2><p class="br-mp">«${esc(T(i).title)}» erscheint beim gewählten Profil unter «Empfohlen von anderen Profilen».</p>
    <div class="mlist">${o.map((pr, n) => `<button type="button" class="mi" data-f="rec-${n}" data-act="brRecTo" data-n="${esc(pr.name)}"${n === 0 ? ' data-default' : ''}><span class="mi-ic br-mav">${avatar(pr, 64)}</span><span class="mi-tx"><b>${esc(pr.name)}</b><small>Profil</small></span></button>`).join('')}</div>`);
};
ACT.brRecTo = d => { closeMenu(); toast(`An ${d.n} empfohlen`); };
ACT.brMore = d => {
  const i = Number(d.it); const t = T(i); const dd = D(i) || {}; const f = dd.f;
  const row2 = (k, v) => `<div><dt>${k}</dt><dd>${v}</dd></div>`;
  const facts = [
    t.year ? row2('Jahr', t.year) : '', t.fsk ? row2('Freigabe', `ab ${t.fsk}`) : '',
    t.kind === 's' ? row2('Staffeln', dd.s ? dd.s.map(s => `Staffel ${s[0]}: ${s[1]} Folgen`).join('<br>') : 'Serie') : t.min ? row2('Dauer', dur(t.min)) : '',
    brGenres(i).length ? row2('Genres', esc(brGenres(i).join(', '))) : '',
  ].join('');
  const tech = f ? `<h3 class="br-mh3">Datei und Technik</h3><dl class="br-dl">
      ${row2('Video', `${BR_VC[f[0]] || f[0].toUpperCase()} · ${brRes(f[1])}`)}${row2('Ton', esc(BR_AC[f[2]] || f[2].toUpperCase()))}
      ${row2('Container', f[4].toUpperCase())}${row2('Grösse', `${String(f[3]).replace('.', ',')} GB`)}${row2('Fassungen', f[5] === 1 ? 'Eine Fassung' : `${f[5]} Fassungen`)}</dl>` : '';
  menu(`<div class="br-mhead"><h2>${esc(t.title)}</h2><button type="button" class="tb sm ghost" data-act="close" data-f="mi-close" data-default>${icon('close', 30)}<span>Schliessen</span></button></div>
    ${dd.t ? `<p class="br-mtag">${esc(dd.t)}</p>` : ''}<p class="br-mov">${esc(dd.o || 'Für diesen Titel liefert der Server keine Inhaltsangabe.')}</p>
    <dl class="br-dl">${facts}</dl>${tech}<p class="br-mhint">Mit Hoch und Runter blätterst du.</p>`, {
    cls: 'br-wide',
    onKey: name => { if (name !== 'up' && name !== 'down') return false; const pn = $('.ov-panel', $('.br-wide')); if (pn) pn.scrollBy({ top: name === 'down' ? 240 : -240, behavior: 'smooth' }); return true; },
  });
};
PAGES.push(['#/item/0', 'Film: Spider-Man (Logo, Technik)'], ['#/item/2', 'Serie: Die Ringe der Macht (3 Staffeln)'], ['#/item/37', 'Serie mit Fortschritt: Normal People'], ['#/item/16', 'Film ohne Einzelheiten: Heartland']);

/* ============================================================
   Person
   ============================================================ */
route('/person/:id', p => {
  const id = Number(p.id); const per = brPeople().find(x => x.id === id);
  if (!per) return { nav: 'start', html: brEmptyPage('user', 'Person nicht gefunden', 'Zu dieser Person kennt die Bibliothek keine Titel.', '#/', 'Zur Startseite') };
  const ids = [...new Set(per.roles.map(r => r[0]))];
  const jobs = [...new Set(per.roles.map(([, r]) => BR_ROLE[r] || 'Schauspiel'))].join(' · ');
  const sub = i => per.roles.filter(r => r[0] === i).map(([, r]) => (BR_ROLE[r] ? BR_ROLE[r] : `als ${r}`)).join(', ');
  return {
    nav: 'start', after: () => art(artOf(ids[0]), 'dim'),
    html: `<div class="page br-person">
      <div class="br-pim">${pic(`c-${id}.webp`, per.name)}</div>
      <div class="br-ptx" data-info-sec><p class="sp-kind">Person</p><h1 class="br-h1">${esc(per.name)}</h1><p class="br-jobs">${esc(jobs)}</p>
        <span class="badge">${ids.length} ${ids.length === 1 ? 'Titel' : 'Titel'} in deiner Bibliothek</span>
        <h2 class="br-h2">Bekannt aus</h2>
        <div class="br-prow" data-row="known">${ids.map((i, n) => card(i, { row: 'kn', sub: sub(i) }).replace('<a class="tc', n === 0 ? '<a data-default class="tc' : '<a class="tc')).join('')}</div>
        <div class="rinfo br-rinfo" aria-live="polite"></div></div></div>`,
  };
});
PAGES.push(['#/person/1881', 'Person: Tom Holland'], ['#/person/15247', 'Person: J. R. R. Tolkien']);

/* ============================================================
   Search: keyboard on the left, results on the right
   ============================================================ */
const brRecentQ = () => S.brRecentQ ?? (S.brRecentQ = ['Resident Evil', 'Tolkien', 'Cube']);
function brAddRecent(q) { const v = String(q || '').trim(); if (v.length < 2) return; S.brRecentQ = [v, ...brRecentQ().filter(x => brNorm(x) !== brNorm(v))].slice(0, 6); save(); }
const BR_GROUPS = [['m', 'Filme'], ['s', 'Serien'], ['a', 'Hörbücher'], ['p', 'Podcasts']];
const BR_SPH = 'Titel, Person, Autorin …';
function brWishLink(q) {
  return `<a class="br-wish" href="#/wishes" data-f="sr-wish"><span class="br-wic">${icon('wish', 40)}</span><span class="br-wtx"><b>Noch nicht in der Bibliothek?</b><small>${q ? `«${esc(q)}» bei den Wünschen suchen` : 'Zu den Wünschen'}</small></span>${icon('right', 36)}</a>`;
}
function brSrBody() {
  const q = String(ui.q || '').trim();
  if (!q) {
    const r = brRecentQ();
    const tiles = LIBS.map(L => {
      const first = L.ids()[0]; const audio = brIsSq(first);
      return `<a class="br-tile" href="#/library/${L.key}" data-f="tl-${L.key}"><span class="br-tim">${audio ? poster(first) : backdrop(first)}</span>
        <span class="br-tlx">${icon(L.icon, 34)}<span><b>${esc(L.name)}</b><small>${brNf(L.count)} Titel</small></span></span></a>`;
    }).join('');
    return `${r.length ? `<section class="rsec"><h2 class="rh">Zuletzt gesucht</h2><div class="br-rchips" data-row="recent">${r.map((x, n) => `<button type="button" class="br-cb" data-act="brSrRecent" data-q="${esc(x)}" data-f="qr-${n}">${icon('clock', 30)}<span>${esc(x)}</span></button>`).join('')}
        <button type="button" class="br-cb ghost" data-act="brSrForget" data-f="qr-x">${icon('trash', 30)}<span>Verlauf löschen</span></button></div></section>` : ''}
      <section class="rsec"><h2 class="rh">Bibliotheken</h2><div class="br-tiles">${tiles}</div></section>`;
  }
  const nq = brNorm(q);
  const hits = DATA.T.map((x, i) => i).filter(i => T(i).kind !== 'e' && (brNorm(T(i).title).includes(nq) || brNorm(T(i).author).includes(nq)));
  const per = brPeople().filter(x => brNorm(x.name).includes(nq));
  const total = hits.length + per.length;
  if (!total) return `<div class="br-snone"><span class="br-eic">${icon('search', 72)}</span><h2>Nichts gefunden</h2><p>Für «${esc(q)}» gibt es keinen Titel und keine Person. Prüfe die Schreibweise oder versuche einen kürzeren Begriff.</p></div>${brWishLink(q)}`;
  const groups = BR_GROUPS.map(([k, label]) => {
    const ids = hits.filter(i => T(i).kind === k); if (!ids.length) return '';
    return hrow(`${label} · ${ids.length}`, ids, null, null, `sr-${k}`);
  }).join('');
  const pp = per.length ? `<section class="rsec" data-sec="sr-pp"><h2 class="rh">Personen · ${per.length}</h2><div class="trow" data-row="sr-pp">${per.map(x => `<a class="br-spr" href="#/person/${x.id}" data-f="sp-${x.id}" data-tt="${esc(x.name)}" data-tm="${esc([...new Set(x.roles.map(([, r]) => BR_ROLE[r] || 'Schauspiel'))].join(' · '))}">
      <span class="br-sprim">${pic(`c-${x.id}.webp`, x.name)}</span><b>${esc(x.name)}</b></a>`).join('')}</div><div class="rinfo" aria-live="polite"></div></section>` : '';
  return `<p class="br-scount" role="status">${total} Treffer für «${esc(q)}»</p>${groups}${pp}${brWishLink(q)}`;
}
function brKbKeys() {
  const rows = ui.brSym ? KB_ROWS.sym : KB_ROWS.abc;
  const key = (f, code, label, cls, extra) => `<button type="button" class="kb-k${cls ? ' ' + cls : ''}" data-f="${f}" data-act="brKey" data-k="${esc(code)}"${extra || ''}>${label}</button>`;
  return rows.map((r, ri) => [...r].map((ch, ci) => key(`sk-${ri}-${ci}`, 'c:' + ch, esc(ch), '', ri === 1 && ci === 0 ? ' data-default' : '')).join('')
    + (ri === 0 ? key('sk-0-10', 'del', icon('backspace', 36), 'fn', ' aria-label="Zeichen löschen"') : '')).join('')
    + key('sk-4-0', 'sym', ui.brSym ? 'abc' : '#+=', 'fn w2', ` aria-pressed="${!!ui.brSym}"`) + key('sk-4-1', 'space', 'Leerzeichen', 'fn w6') + key('sk-4-2', 'clear', `${icon('close', 30)} Leeren`, 'fn w3');
}
const brSfv = () => (ui.q ? esc(ui.q) : `<span class="kb-ph">${BR_SPH}</span>`);
function brSrUpdate() {
  const v = $('#br-sfv'); if (v) v.innerHTML = brSfv();
  const r = $('#br-sres'); if (!r) return;
  const had = F.el && r.contains(F.el);
  r.innerHTML = brSrBody(); r.scrollTop = 0;
  if (had || (F.el && !document.contains(F.el))) { const first = defaultIn(r); if (first && had) focus(first); else focusTo('sk-1-0'); }
}
route('/search', () => ({
  nav: 'search',
  html: `<div class="page br-search">
    <div class="br-sl">
      <h1 class="br-sh1">Suche</h1>
      <div class="br-sline"><div class="br-sf">${icon('search', 40)}<span class="br-sfv" id="br-sfv">${brSfv()}</span><i class="kb-caret" aria-hidden="true"></i></div>
        <button type="button" class="br-mic" data-f="sr-mic" data-act="brMic" aria-label="Spracheingabe">${icon('mic', 44)}</button></div>
      <div class="br-kb" id="br-kbg">${brKbKeys()}</div>
      <p class="br-shint">Tippen geht auch mit der Tastatur des Computers.</p>
    </div>
    <div class="br-sres" id="br-sres" data-scroll>${brSrBody()}</div></div>`,
  key: (name, ev) => {
    if (ev && ev.key && ev.key.length === 1 && !ev.ctrlKey && !ev.metaKey) { ui.q = (ui.q || '') + ev.key; brSrUpdate(); return true; }
    if (ev && ev.key === 'Backspace') { if (ui.q) { ui.q = ui.q.slice(0, -1); brSrUpdate(); return true; } return false; }
    if (name === 'ok' && F.el && F.el.closest('#br-sres') && ui.q) brAddRecent(ui.q);
    return false;
  },
}));
ACT.brKey = d => {
  const k = d.k;
  if (k.startsWith('c:')) ui.q = (ui.q || '') + k.slice(2);
  else if (k === 'space') ui.q = (ui.q || '') + ' ';
  else if (k === 'del') ui.q = (ui.q || '').slice(0, -1);
  else if (k === 'clear') ui.q = '';
  else if (k === 'sym') { ui.brSym = !ui.brSym; $('#br-kbg').innerHTML = brKbKeys(); focusTo('sk-4-0'); return; }
  brSrUpdate();
};
ACT.brSrRecent = d => { ui.q = d.q; brAddRecent(d.q); brSrUpdate(); const r = $('#br-sres'); const first = r && defaultIn(r); if (first) focus(first); };
ACT.brSrForget = () => { S.brRecentQ = []; save(); toast('Suchverlauf gelöscht'); brSrUpdate(); const r = $('#br-sres'); const first = r && defaultIn(r); if (first) focus(first); };
ACT.brMic = (d, el) => {
  el.classList.add('on'); toast('Ich höre zu … (im Mockup sagst du «Tolkien»)');
  setTimeout(() => { el.classList.remove('on'); if (curPath() !== '/search') return; ui.q = 'Tolkien'; brAddRecent(ui.q); brSrUpdate(); }, 1600);
};
PAGES.push(['#/search', 'Suche']);

/* ============================================================
   Lists, watchlist, watched
   ============================================================ */
const BR_LISTS = { rel: ['Neu erschienen', 'star'], rec: ['Zuletzt hinzugefügt', 'clock'], picks: ['Empfohlen von anderen Profilen', 'heart'] };
function brListHead(ic, title, n, extra) {
  return `<header class="br-head"><span class="br-hic">${icon(ic, 44)}</span><h1>${esc(title)}</h1><span class="br-count">${n} ${n === 1 ? 'Titel' : 'Titel'}</span></header>${extra || ''}`;
}
route('/list/:key', p => {
  const L = BR_LISTS[p.key]; const ids = L ? row(p.key) : [];
  if (!L || !ids.length) return { nav: 'start', html: brEmptyPage('list', 'Diese Liste ist leer', 'Hier ist gerade nichts zu sehen.', '#/', 'Zur Startseite') };
  const o = brOthers();
  const opts = p.key === 'picks' ? (i => ({ tag: `${icon('heart', 24)} ${esc((o[i % o.length] || { name: 'Tipp' }).name)}` })) : null;
  BR.sum = '';
  return {
    nav: 'start', onFocus: brGridFocus, after: brGridAfter,
    html: `<div class="page br-fixed" data-info-sec>${brListHead(L[1], L[0], ids.length)}<div class="rinfo br-rinfo" aria-live="polite"></div>${brGrid(ids, 'l', false, opts)}</div>`,
  };
});
/** watchlist and watched: grid with a remove mode («Bearbeiten») */
function brEditList(list, ids, title, ic, emptyT, emptyX) {
  ui.brEdit = ui.brEdit || {};
  if (!ids.length) { ui.brEdit[list] = false; return { nav: 'start', html: brEmptyPage(ic, emptyT, emptyX, '#/', 'Zur Startseite') }; }
  const edit = !!ui.brEdit[list];
  const what = list === 'watch' ? 'Entfernen' : 'Markierung entfernen';
  const mode = `<div class="br-ctrls">${edit
    ? `<button type="button" class="br-cb on" data-f="ed-tog" data-act="brEdit" data-list="${list}">${icon('check', 32)}<span>Fertig</span></button><p class="br-edhint">${icon('trash', 30)} OK auf einem Titel: ${what}</p>`
    : `<button type="button" class="br-cb" data-f="ed-tog" data-act="brEdit" data-list="${list}">${icon('edit', 32)}<span>Bearbeiten</span></button>`}</div>`;
  BR.sum = edit ? `<b>Bearbeiten</b><span>Wähle die Titel, die du ${list === 'watch' ? 'von der Merkliste entfernen' : 'nicht mehr als gesehen führen'} willst</span>` : '';
  const cards = ids.map(i => {
    const c = card(i, { row: list });
    return edit ? c.replace('<a class="tc', `<a data-act="brRemove" data-list="${list}" class="tc br-edit`).replace('<span class="im">', `<span class="im"><span class="br-rm">${icon('trash', 52)}<b>${what}</b></span>`) : c;
  }).join('');
  return {
    nav: 'start', onFocus: brGridFocus, after: brGridAfter,
    html: `<div class="page br-fixed${edit ? ' br-editing' : ''}" data-info-sec>${brListHead(ic, title, ids.length, mode)}<div class="rinfo br-rinfo" aria-live="polite">${BR.sum}</div>
      <div class="br-gw" data-scroll><div class="br-grid">${cards}</div></div></div>`,
  };
}
route('/watchlist', () => brEditList('watch', S.watch, 'Merkliste', 'bookmark', 'Deine Merkliste ist leer', 'Wähle bei einem Titel «Merken», dann findest du ihn hier.'));
route('/watched', () => brEditList('seen', S.seen.slice().reverse(), 'Gesehen', 'eye', 'Noch nichts gesehen', 'Was du fertig schaust oder als gesehen markierst, steht hier.'));
ACT.brEdit = d => { ui.brEdit = ui.brEdit || {}; ui.brEdit[d.list] = !ui.brEdit[d.list]; render(); focusTo(ui.brEdit[d.list] ? `c-${d.list}-${(d.list === 'watch' ? S.watch : S.seen.slice().reverse())[0]}` : 'ed-tog'); };
ACT.brRemove = (d, el) => {
  const i = Number(el.dataset.i); const cards = $$('.br-grid .tc'); const k = cards.indexOf(el); const nb = cards[k + 1] || cards[k - 1];
  if (d.list === 'watch') { S.watch = S.watch.filter(x => x !== i); toast('Von der Merkliste entfernt'); } else { S.seen = S.seen.filter(x => x !== i); brSyncCont(); toast('Markierung «Gesehen» entfernt'); }
  commit(); if (nb) focusTo(nb.dataset.f);
};
PAGES.push(['#/list/rel', 'Liste: Neu erschienen'], ['#/list/picks', 'Liste: Empfohlen von anderen Profilen'], ['#/watchlist', 'Merkliste (mit Bearbeiten)'], ['#/watched', 'Gesehen']);

/* ============================================================
   Collections
   ============================================================ */
/* members the test data knows; the rest stand in from the film library (marked in the mockup) */
const BR_COLKNOWN = { '01M5N7PQ19J07M2DJS1TBNWBQ8': [72], '01M5N7PTTWWDJ78BHEJ70EAQFS': [86] };
function brColMembers(k) {
  const [id, , n] = DATA.col[k]; const known = BR_COLKNOWN[id] || []; const pool = row('lib1').filter(i => !known.includes(i));
  const ids = known.slice(); let j = k * 3; while (ids.length < Math.max(1, n)) { ids.push(pool[j % pool.length]); j++; }
  return { ids, stand: ids.length - known.length };
}
route('/collections', () => {
  BR.sum = `<b>${DATA.col.length} Sammlungen</b><span>Filmreihen deiner Bibliothek</span>`;
  return {
    nav: 'libs', onFocus: brGridFocus, after: brGridAfter,
    html: `<div class="page br-fixed" data-info-sec>${brListHead('grid', 'Sammlungen', DATA.col.length).replace(/(\d+) Titel</, '$1 Sammlungen<')}<div class="rinfo br-rinfo" aria-live="polite">${BR.sum}</div>
      <div class="br-gw" data-scroll><div class="br-grid">${DATA.col.map(([id, name, n], k) => `<a class="tc" href="#/collection/${id}" data-f="col-${k}" data-tt="${esc(name)}" data-tm="${n} Titel"${k === 0 ? ' data-default' : ''}><span class="im">${pic(`k-${id}.webp`, name)}</span></a>`).join('')}</div></div></div>`,
  };
});
route('/collection/:id', p => {
  const k = DATA.col.findIndex(c => c[0] === p.id);
  if (k < 0) return { nav: 'libs', html: brEmptyPage('grid', 'Sammlung nicht gefunden', 'Diese Sammlung gibt es nicht mehr.', '#/collections', 'Zu den Sammlungen') };
  const [id, name] = DATA.col[k]; const { ids, stand } = brColMembers(k); const allIn = ids.every(i => S.watch.includes(i));
  const years = ids.map(i => T(i).year).filter(Boolean); const span = years.length ? (Math.min(...years) === Math.max(...years) ? `${years[0]}` : `${Math.min(...years)}–${Math.max(...years)}`) : '';
  return {
    nav: 'libs', after: () => art(artOf(ids[0]), 'dim'),
    html: `<div class="page br-colp">
      <div class="br-pim">${pic(`k-${id}.webp`, name)}</div>
      <div class="br-ptx" data-info-sec><p class="sp-kind">Sammlung</p><h1 class="br-h1 br-colh">${esc(name)}</h1><p class="br-jobs">${ids.length} Titel${span ? ` · ${span}` : ''}</p>
        <div class="br-acts">
          <a class="tb primary" href="#/play/${ids[0]}" data-f="col-play" data-default>${icon('play', 34)}<span>Abspielen</span></a>
          <button type="button" class="tb" data-f="col-w" data-act="brColWatch" data-k="${k}" aria-pressed="${allIn}">${icon(allIn ? 'bookmarkOn' : 'bookmark', 34)}<span>${allIn ? 'Alle gemerkt' : 'Alle merken'}</span></button></div>
        <h2 class="br-h2">In dieser Sammlung</h2>
        <div class="br-prow" data-row="colm">${ids.map(i => card(i, { row: 'cm' })).join('')}</div>
        <div class="rinfo br-rinfo" aria-live="polite"></div>
        ${stand ? `<p class="mockhint"><span class="mk">Nur im Mockup</span> ${stand} ${stand === 1 ? 'Titel steht' : 'Titel stehen'} nicht in den Testdaten und ${stand === 1 ? 'ist' : 'sind'} durch andere Filme ersetzt.</p>` : ''}</div></div>`,
  };
});
ACT.brColWatch = d => {
  const { ids } = brColMembers(Number(d.k)); const allIn = ids.every(i => S.watch.includes(i));
  if (allIn) { S.watch = S.watch.filter(i => !ids.includes(i)); toast('Von der Merkliste entfernt'); } else { ids.forEach(i => { if (!S.watch.includes(i)) S.watch.unshift(i); }); toast(`${ids.length} Titel gemerkt`); }
  commit();
};
PAGES.push(['#/collections', 'Sammlungen'], ['#/collection/01M5N7PTTWWDJ78BHEJ70EAQFS', 'Sammlung: 28 Days/Weeks Later']);

/* ============================================================
   Video player
   ============================================================ */
const BRP = { key: null, i: -1, ep: null, t: 0, dur: 60, playing: true, ctl: true, shown: 0, last: 0, saved: 0, timer: null, hideT: null,
  audio: 0, subs: 0, qual: 0, info: false, streak: 0, lastSeek: 0, acc: 0, bubT: 0, skipped: false, cancel: false, nextOn: false, nextLeft: 10,
  ended: false, done: false, rowF: 'pl-ton', isEp: false };
function brAudioTracks(i) {
  const f = brFile(i); const main = BR_AC[f[2]] || f[2].toUpperCase(); const sur = S.prefs.surround !== false;
  const pass = sur ? 'Original · wird an die Tonanlage durchgereicht' : 'Wird zu Stereo gemischt (Einstellung «Immer Stereo»)';
  return [
    { b: `Deutsch · ${main} 5.1 · Original`, s: pass, codec: main, ch: '5.1', n: 6, multi: true },
    { b: 'Englisch · DTS-HD MA 7.1 · Original', s: pass, codec: 'DTS-HD Master Audio', ch: '7.1', n: 8, multi: true },
    { b: 'Deutsch · Stereo', s: 'AAC · für TV-Lautsprecher und Kopfhörer', codec: 'AAC', ch: '2.0', n: 2, multi: false },
  ];
}
const BR_SUBS = [['Aus', ''], ['Deutsch', 'Alle Dialoge · Text'], ['Deutsch erzwungen', 'Nur fremdsprachige Stellen und Schilder'], ['Englisch', 'Alle Dialoge · Text'], ['Englisch · PGS', 'Bilduntertitel, vom Player selbst gezeichnet']];
const BR_QUAL = [['Automatisch', 'Originaldatei, solange die Verbindung reicht'], ['Original', 'Immer die Originaldatei'], ['1080p', 'Wird auf dem Server umgewandelt'], ['720p', 'Wird auf dem Server umgewandelt']];
const brPlImg = () => (BRP.ep ? `e-${BRP.ep.id}.webp` : T(BRP.i).kind === 'e' ? `p-${BRP.i}.webp` : `b-${BRP.i}.webp`);

function brPlInit(i, q) {
  const t = T(i); const P = brProg()[i]; let ep = null;
  if (t.kind === 's' && brEps(i).length) {
    if (q.e != null) ep = brEp(i, Number(q.s || 1), Number(q.e));
    if (!ep) ep = brResume(i).ep;
  }
  const dur = Math.max(120, (ep ? ep.min : t.min || 30) * 60);
  let start = 0;
  if (q.at === 'end') start = dur - 24;
  else if (P && (!ep || ((P.s || 1) === ep.s && P.k === ep.k))) start = Math.min(P.t, dur - 30);
  if (BRP.i !== i) {
    BRP.audio = S.prefs.audio === 'Englisch' ? 1 : 0;
    BRP.subs = Math.max(0, BR_SUBS.findIndex(s => s[0] === S.prefs.subs));
    BRP.qual = Math.max(0, BR_QUAL.findIndex(s => s[0] === S.prefs.quality));
    BRP.info = false;
  }
  Object.assign(BRP, { key: curHash(), i, ep, t: start, dur, playing: true, ctl: true, shown: Date.now(), last: Date.now(), saved: Date.now(),
    streak: 0, acc: 0, bubT: 0, skipped: start >= 60, cancel: false, nextOn: false, nextLeft: 10, ended: false, done: false, isEp: !!ep || t.kind === 'e' });
}
function brPlHtml() {
  const i = BRP.i; const t = T(i); const ep = BRP.ep; const img = brPlImg();
  const title = ep ? `${brEpLabel(ep)} · ${brEpName(ep)}` : t.title;
  const over = ep ? t.title : t.kind === 'e' ? 'Folge' : [t.year, 'Film'].filter(Boolean).join(' · ');
  const prev = ep && brEpPrev(i, ep); const next = ep && brEpNext(i, ep);
  const pb = (f, ic, label, act, extra) => `<button type="button" class="br-pb" data-f="${f}" data-act="${act}"${extra || ''}>${icon(ic, 32)}<span>${label}</span></button>`;
  const nav = (f, ic, label, to) => pb(f, ic, label, 'brPlGoEp', to ? ` data-to="${to}"` : ' aria-disabled="true"');
  return `<div class="br-pl${BRP.ctl ? ' ctl' : ''}${BRP.playing ? '' : ' paused'}" id="br-pl">
    <div class="br-pic">${pic(img, t.title)}</div>
    <div class="br-flash" id="br-flash" aria-hidden="true"></div>
    <button type="button" class="tb br-skip" data-f="pl-skip" data-act="brPlSkip" data-default hidden>${icon('fwd', 34)}<span>Vorspann überspringen</span></button>
    ${next ? `<div class="br-next" id="br-next" hidden><span class="br-nxim">${pic(`e-${next.id}.webp`, brEpName(next))}</span>
      <div class="br-nxtx"><small id="br-nxin">Nächste Folge</small><b>${brEpLabel(next)} · ${esc(brEpName(next))}</b>
        <div class="br-nxacts"><button type="button" class="tb primary sm" data-f="pl-nx-play" data-act="brPlNow" data-default>${icon('play', 30)}<span>Jetzt abspielen</span></button>
          <button type="button" class="tb ghost sm" data-f="pl-nx-x" data-act="brPlCancel"><span>Abbrechen</span></button></div></div></div>` : ''}
    <div class="br-ctl" id="br-ctl"${BRP.ctl ? '' : ' hidden'}>
      <div class="br-ptop"><div class="br-ptitle"><small>${esc(over)}</small><h1>${esc(title)}</h1></div><div class="br-pclock"><b id="br-now"></b><small id="br-ends"></small></div></div>
      <div class="br-pbot">
        <div class="br-state" id="br-state" aria-live="polite"></div>
        <div class="br-seek" data-f="pl-bar" data-default role="slider" aria-label="Zeitleiste" aria-valuemin="0" aria-valuemax="${Math.round(BRP.dur)}">
          <div class="br-track"><i class="br-fill" id="br-fill"></i><b class="br-knob" id="br-knob"></b></div>
          <div class="br-bub" id="br-bub" hidden><span class="br-bubim">${pic(img, '')}</span><b id="br-bubt"></b><small id="br-buba"></small></div>
        </div>
        <div class="br-times"><span id="br-el"></span><span id="br-rem"></span></div>
        <div class="br-pbtns" data-row="plbtn">
          ${pb('pl-ton', 'audio', 'Ton', 'brPlMenu', ' data-m="ton"')}${pb('pl-sub', 'subs', 'Untertitel', 'brPlMenu', ' data-m="sub"')}${pb('pl-q', 'hd', 'Qualität', 'brPlMenu', ' data-m="q"')}
          ${ep ? pb('pl-eps', 'list', 'Folgen', 'brPlMenu', ' data-m="eps"') + nav('pl-prev', 'prev', 'Vorige', prev && brEpUrl(i, prev)) + nav('pl-next', 'next', 'Nächste', next && brEpUrl(i, next)) : ''}
          ${pb('pl-info', 'info', 'Info', 'brPlInfo', ` aria-pressed="${BRP.info}"`)}
        </div>
      </div>
    </div>
    <div class="br-info" id="br-info"${BRP.info ? '' : ' hidden'}>${brPlInfoHtml()}</div>
  </div>`;
}
function brPlInfoHtml() {
  const i = BRP.i; const f = brFile(i); const a = brAudioTracks(i)[BRP.audio]; const tr = BRP.qual >= 2; const sur = S.prefs.surround !== false;
  const row2 = (k, v) => `<div><dt>${k}</dt><dd>${v}</dd></div>`;
  const keep = a.codec === 'Dolby Digital' || a.codec === 'Dolby Digital Plus';
  const audio = tr ? (keep ? `${a.codec} · ${a.ch} (kopiert)` : a.multi ? `Dolby Digital · 5.1 (neu kodiert)` : 'AAC · 2.0') : `${a.codec} · ${a.ch} (${a.n} Kanäle)`;
  const min = BRP.ep ? BRP.ep.min : T(i).min; const rate = f[3] && min ? `${String((f[3] * 8000 / (min * 60)).toFixed(1)).replace('.', ',')} Mbit/s` : '';
  const passOk = a.multi && sur;
  return `<h2>Wiedergabe</h2><dl class="br-dl">
    ${row2('Abspielart', tr ? `Umwandlung auf ${BR_QUAL[BRP.qual][0]}` : 'Direktwiedergabe')}
    ${row2('Video', tr ? `H.264 · ${BR_QUAL[BRP.qual][0]} (umgewandelt)` : `${BR_VC[f[0]] || f[0].toUpperCase()} · ${brRes(f[1])}`)}
    ${row2('HDR', 'Nein (SDR)')}${row2('Ton', esc(audio))}${row2('Container', tr ? 'HLS' : f[4].toUpperCase())}${rate && !tr ? row2('Bitrate', rate) : ''}
    ${row2('Untertitel', esc(BR_SUBS[BRP.subs][0]))}</dl>
    <p class="br-pass${passOk ? ' ok' : ''}">${icon(passOk ? 'check' : 'audio', 32)}<span>${passOk ? 'Ton wird an die Tonanlage durchgereicht' : a.multi ? 'Ton wird zu Stereo gemischt' : 'Stereo-Ton für TV-Lautsprecher'}</span></p>`;
}

function brPlDraw() {
  const root = $('#br-pl'); if (!root) return;
  const now = Date.now(); const pct = Math.min(100, BRP.t / BRP.dur * 100); const left = BRP.dur - BRP.t;
  $('#br-fill').style.width = `${pct}%`; $('#br-knob').style.left = `${pct}%`;
  $('#br-el').textContent = brTc(BRP.t); $('#br-rem').textContent = `−${brTc(left)}`;
  $('#br-now').textContent = brClock(new Date()); $('#br-ends').textContent = `Endet um ${brClock(new Date(now + left * 1000))}`;
  const bar = byF('pl-bar'); if (bar) bar.setAttribute('aria-valuetext', `${brTc(BRP.t)} von ${brTc(BRP.dur)}`);
  root.classList.toggle('paused', !BRP.playing);
  $('#br-state').innerHTML = BRP.ended ? `${icon('check', 36)}<span>Zu Ende geschaut</span>` : BRP.playing ? '' : `${icon('pause', 36)}<span>Pausiert</span>`;
  /* preview bubble while seeking */
  const bub = $('#br-bub'); const showBub = now - BRP.bubT < 1500;
  bub.hidden = !showBub;
  if (showBub) {
    bub.style.left = `${Math.max(9, Math.min(91, pct))}%`; $('#br-bubt').textContent = brTc(BRP.t);
    const a = Math.abs(BRP.acc); $('#br-buba').textContent = `${BRP.acc < 0 ? '−' : '+'}${a < 60 ? `${a} s` : brTc(a)}`;
  }
  /* «Vorspann überspringen» in the first minute of an episode */
  const sk = byF('pl-skip'); const showSk = BRP.isEp && BRP.t < 60 && !BRP.skipped && !BRP.ended;
  if (sk && sk.hidden === showSk) {
    sk.hidden = !showSk;
    if (!showSk && F.el === sk) focus(byF('pl-bar'), { noReveal: true });
  }
  /* «Nächste Folge in 10» near the end */
  const nx = $('#br-next');
  if (nx) {
    const on = left <= 20 && !BRP.cancel && !BRP.ended;
    if (on !== BRP.nextOn) {
      BRP.nextOn = on; nx.hidden = !on;
      if (on) { BRP.nextLeft = 10; focus(byF('pl-nx-play'), { noReveal: true }); } else if (F.el && nx.contains(F.el)) focus(byF('pl-bar'), { noReveal: true });
    }
    if (on) $('#br-nxin').textContent = S.prefs.autoplay === false ? 'Nächste Folge' : `Nächste Folge in ${Math.max(0, Math.ceil(BRP.nextLeft))}`;
  }
}
function brPlShow(f) {
  BRP.shown = Date.now();
  if (!BRP.ctl) { BRP.ctl = true; const c = $('#br-ctl'); if (c) { c.hidden = false; void c.offsetWidth; } const r = $('#br-pl'); if (r) r.classList.add('ctl'); }
  if (f) focusTo(f, { noReveal: true });
}
function brPlHide() {
  BRP.ctl = false; const r = $('#br-pl'); if (r) r.classList.remove('ctl');
  clearTimeout(BRP.hideT); BRP.hideT = setTimeout(() => { if (!BRP.ctl) { const c = $('#br-ctl'); if (c) c.hidden = true; } }, 320);
  const nx = byF('pl-nx-play'); const sk = byF('pl-skip');
  if (nx && !$('#br-next').hidden) focus(nx, { noReveal: true });
  else if (sk && !sk.hidden) focus(sk, { noReveal: true });
  else focus(byF('pl-bar'), { noReveal: true });
}
function brPlFlash(ic) { const fl = $('#br-flash'); if (!fl) return; fl.innerHTML = icon(ic, 96); fl.classList.remove('go'); void fl.offsetWidth; fl.classList.add('go'); }
function brPlToggle() {
  if (BRP.ended) { BRP.t = 0; BRP.ended = false; BRP.done = false; BRP.cancel = false; BRP.skipped = false; }
  BRP.playing = !BRP.playing; brPlFlash(BRP.playing ? 'play' : 'pause'); brPlShow(); brPlDraw();
}
function brPlSeek(dir) {
  const now = Date.now(); const again = now - BRP.lastSeek < 700;
  BRP.streak = again ? BRP.streak + 1 : 0; BRP.lastSeek = now;
  const step = [10, 10, 20, 30, 30, 60, 60, 120][Math.min(BRP.streak, 7)];
  const before = BRP.t; BRP.t = Math.max(0, Math.min(BRP.dur - 1, BRP.t + dir * step));
  BRP.acc = (again && Math.sign(BRP.acc) === dir ? BRP.acc : 0) + Math.round(BRP.t - before);
  if (BRP.ended && BRP.t < BRP.dur - 1) BRP.ended = false;
  BRP.bubT = now; brPlDraw();
}
/** store progress; at 95 % the title or episode counts as seen */
function brPlStore(persist) {
  const i = BRP.i; if (i < 0 || BRP.done) { if (persist) save(); return; }
  const P = brProg(); const pct = BRP.t / BRP.dur * 100;
  if (pct >= 95) {
    BRP.done = true; const ep = BRP.ep;
    if (ep) {
      if (!brEpSeen().includes(ep.key)) brEpSeen().push(ep.key);
      const nx = brEpNext(i, ep);
      if (nx) P[i] = { t: 0, pct: 1, s: nx.s, k: nx.k, at: Date.now() };
      else { delete P[i]; if (!S.seen.includes(i)) S.seen.push(i); }
    } else { delete P[i]; if (!S.seen.includes(i)) S.seen.push(i); }
  } else if (BRP.t >= 5) P[i] = { t: Math.round(BRP.t), pct, s: BRP.ep ? BRP.ep.s : undefined, k: BRP.ep ? BRP.ep.k : undefined, at: Date.now() };
  brSyncCont();
  if (persist) { save(); BRP.saved = Date.now(); }
}
function brPlGo(url) { clearInterval(BRP.timer); BRP.timer = null; brPlStore(true); closeAll(); go(url, { replace: true }); }
function brPlTick() {
  if (!curPath().startsWith('/play/') || !$('#br-pl')) { clearInterval(BRP.timer); BRP.timer = null; brPlStore(true); BRP.key = null; return; }
  const now = Date.now(); const dt = Math.min(1, (now - BRP.last) / 1000); BRP.last = now;
  if (BRP.playing && !OFF) {
    BRP.t = Math.min(BRP.dur, BRP.t + dt);
    if (BRP.nextOn && S.prefs.autoplay !== false) { BRP.nextLeft -= dt; if (BRP.nextLeft <= 0) { ACT.brPlNow(); return; } }
    if (BRP.t >= BRP.dur) {
      const nx = BRP.ep && brEpNext(BRP.i, BRP.ep);
      if (nx && !BRP.cancel) { ACT.brPlNow(); return; }
      BRP.playing = false; BRP.ended = true; brPlStore(true); brPlShow('pl-bar'); toast('Fertig geschaut');
    }
  }
  if (BRP.ctl && BRP.playing && !OV.length && now - Math.max(lastInput, BRP.shown) > 4000) brPlHide();
  brPlStore(now - BRP.saved > 5000);
  brPlDraw();
}
function brPlKey(name) {
  if (!name) return false;
  if (OFF) return false;
  const f = F.el && F.el.dataset.f; const onSkip = f === 'pl-skip'; const onNext = !!f && f.startsWith('pl-nx');
  if (name === 'play') { brPlToggle(); return true; }
  if (name === 'back' || name === 'home') return false;
  if (!BRP.ctl) {
    if (name === 'left' || name === 'right') { if (onNext) return false; brPlSeek(name === 'left' ? -1 : 1); brPlShow(onSkip ? null : 'pl-bar'); return true; }
    if (name === 'ok') { if (onSkip || onNext) return false; brPlToggle(); return true; }
    if (name === 'down') { brPlShow(onSkip || onNext ? null : BRP.rowF); return true; }
    if (name === 'up') { brPlShow(onSkip || onNext ? null : 'pl-bar'); return true; }
    return true;
  }
  BRP.shown = Date.now();
  if (onSkip && (name === 'left' || name === 'right')) { brPlSeek(name === 'left' ? -1 : 1); return true; }
  if (f === 'pl-bar') {
    if (name === 'left' || name === 'right') { brPlSeek(name === 'left' ? -1 : 1); return true; }
    if (name === 'ok') { brPlToggle(); return true; }
    if (name === 'down') { focusTo(usable(byF(BRP.rowF)) ? BRP.rowF : 'pl-ton', { noReveal: true }); return true; }
  }
  return false;
}
route('/play/:i', (p, q) => {
  const i = Number(p.i);
  if (!DATA.T[i]) return { full: true, html: brEmptyPage('film', 'Diesen Titel gibt es nicht', 'Er lässt sich nicht abspielen.', '#/', 'Zur Startseite') };
  const k = T(i).kind;
  if (k === 'a' || k === 'p') return { full: true, html: '', after: () => go(href(i), { replace: true }) };
  if (BRP.key !== curHash() || lastPath !== curPath()) brPlInit(i, q);
  return {
    full: true, html: brPlHtml(), key: brPlKey,
    back: () => {
      if (BRP.info) { ACT.brPlInfo(); return true; }
      if (BRP.ctl) { brPlHide(); return true; }
      brPlStore(true); return false;
    },
    onFocus: el => { if (el.closest('.br-pbtns')) BRP.rowF = el.dataset.f; },
    after: () => { clearInterval(BRP.timer); BRP.last = Date.now(); BRP.timer = setInterval(brPlTick, 250); brPlDraw(); },
  };
});
/* panels over the player: Wiedergabe/Pause still toggles playback */
const BR_PLM = { onKey: name => { if (name !== 'play') return false; brPlToggle(); return true; } };
ACT.brPlMenu = d => {
  const i = BRP.i; BRP.shown = Date.now();
  const menu = (h, o) => window.menu(h, Object.assign({}, BR_PLM, o || {}));
  if (d.m === 'ton') {
    const sur = S.prefs.surround !== false;
    menu(`<h2>Ton</h2><div class="mlist">${brAudioTracks(i).map((a, n) => brOpt(`pa-${n}`, 'brPlSet', `data-m="audio" data-v="${n}"`, a.b, a.s, BRP.audio === n)).join('')}</div>
      <div class="tnote">${icon(sur ? 'audio' : 'alert', 36)}<p>${sur ? 'Tonanlage über HDMI erkannt: Dolby Digital Plus und DTS-HD bis 7.1. Mehrkanalton geht unverändert an die Tonanlage.' : 'In den Einstellungen steht der Ton auf «Immer Stereo».'}</p></div>
      <div class="acts"><a class="tb sm ghost" href="#/settings/playback" data-f="pa-set">${icon('settings', 30)}<span>Ton-Einstellungen</span></a></div>`);
  } else if (d.m === 'sub') {
    menu(`<h2>Untertitel</h2><div class="mlist">${BR_SUBS.map(([b, s], n) => brOpt(`ps-${n}`, 'brPlSet', `data-m="subs" data-v="${n}"`, b, s, BRP.subs === n)).join('')}</div>`);
  } else if (d.m === 'q') {
    menu(`<h2>Qualität</h2><div class="mlist">${BR_QUAL.map(([b, s], n) => brOpt(`pq-${n}`, 'brPlSet', `data-m="qual" data-v="${n}"`, b, s, BRP.qual === n)).join('')}</div>`);
  } else menu(brEpsMenu(BRP.ep.s), { cls: 'br-wide' });
};
function brEpsMenu(s) {
  const i = BRP.i; const d = D(i) || {}; const cur = BRP.ep; const seasons = d.s || [[1, brEps(i).length, 0]]; const list = [];
  for (let k = 0; k < brSeasonLen(i, s); k++) { const ep = brEp(i, s, k); if (ep) list.push(ep); }
  const chips = seasons.length > 1 ? `<div class="br-mchips">${seasons.map(([n]) => `<button type="button" class="br-chip${n === s ? ' on' : ''}" data-f="pes-${n}" data-act="brPlEpSeason" data-s="${n}"${n === s ? ' aria-current="true"' : ''}><b>Staffel ${n}</b></button>`).join('')}</div>` : '';
  return `<h2>Folgen · ${esc(T(i).title)}</h2>${chips}<div class="mlist">${list.map(ep => {
    const on = ep.s === cur.s && ep.k === cur.k; const seen = brIsEpSeen(i, ep);
    return `<button type="button" class="mi br-mep" data-f="pe-${ep.s}-${ep.k}" data-act="brPlGoEp" data-to="${brEpUrl(i, ep)}"${on || (cur.s !== s && ep.k === 0) ? ' data-default' : ''}${on ? ' aria-current="true"' : ''}>
      <span class="br-mepim">${pic(`e-${ep.id}.webp`, brEpName(ep))}</span><span class="mi-tx"><b>${ep.num}. ${esc(brEpName(ep))}</b><small>${ep.min} Min.${on ? ' · läuft gerade' : seen ? ' · gesehen' : ''}</small></span>${on ? `<span class="br-mcheck">${icon('play', 32)}</span>` : ''}</button>`;
  }).join('')}</div>`;
}
ACT.brPlEpSeason = d => { redrawMenu(brEpsMenu(Number(d.s))); };
ACT.brPlSet = d => {
  const n = Number(d.v); BRP[d.m] = n; closeMenu(); BRP.shown = Date.now();
  const label = d.m === 'audio' ? `Ton: ${brAudioTracks(BRP.i)[n].b}` : d.m === 'subs' ? `Untertitel: ${BR_SUBS[n][0]}` : `Qualität: ${BR_QUAL[n][0]}`;
  toast(label); const inf = $('#br-info'); if (inf) inf.innerHTML = brPlInfoHtml();
};
ACT.brPlGoEp = d => { if (d.to) brPlGo(d.to); };
ACT.brPlInfo = () => {
  BRP.info = !BRP.info; const inf = $('#br-info'); if (!inf) return;
  inf.innerHTML = brPlInfoHtml(); inf.hidden = !BRP.info;
  const b = byF('pl-info'); if (b) b.setAttribute('aria-pressed', String(BRP.info));
};
ACT.brPlSkip = () => { BRP.t = 80; BRP.skipped = true; brPlDraw(); };
ACT.brPlNow = () => { const nx = BRP.ep && brEpNext(BRP.i, BRP.ep); if (!nx) return; BRP.t = Math.max(BRP.t, BRP.dur * 0.96); brPlStore(true); brPlGo(brEpUrl(BRP.i, nx)); };
ACT.brPlCancel = () => { BRP.cancel = true; brPlDraw(); brPlShow('pl-bar'); };
PAGES.push(['#/play/0', 'Player: Film'], ['#/play/2?e=0', 'Player: Folge mit «Vorspann überspringen»'], ['#/play/2?e=2&at=end', 'Player: Ende mit «Nächste Folge»']);
