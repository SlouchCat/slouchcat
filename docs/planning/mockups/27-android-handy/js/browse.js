/* Library, detail pages, person, search, lists, collections and the video player.
   Routes: /library/:key, /item/:i, /person/:id, /search, /list/:key, /watchlist, /watched,
   /collections, /collection/:id, /play/:i. Download dialog: ACT.dlAsk lives in media.js. */
'use strict';

/* ---------- data the mockup adds ----------
   The server delivers genres only for the titles with details (D(i).g). For the filters to work on
   every title, the other titles get their usual genres here. «Bewertung» is a stable stand-in value. */
const GEN = (() => {
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
  for (const [g, ids] of Object.entries(src)) for (const i of ids) out[i] = g.split(',').map(s => s.trim());
  return out;
})();
const genres = i => { const d = D(i); return d && d.g ? d.g.slice(0, 3) : (GEN[i] || []); };
const rating = i => 6 + ((i * 53 + 17) % 31) / 10;
const ROLE = { director: 'Regie', writer: 'Drehbuch', creator: 'Idee', producer: 'Produktion' };
const roleText = r => ROLE[r] || r;
const nf = n => Number(n).toLocaleString('de-CH');
const dec = n => String(Math.round(n * 10) / 10).replace('.', ',');
const hasB = i => T(i).kind === 'm' || T(i).kind === 's';
const isSq = i => T(i).kind === 'a' || T(i).kind === 'p';
const norm = s => String(s || '').normalize('NFD').replace(/\p{Diacritic}/gu, '').toLowerCase();
const tc = sec => { const s = Math.max(0, Math.round(sec)); const h = Math.floor(s / 3600); const m = Math.floor((s % 3600) / 60); const x = String(s % 60).padStart(2, '0'); return h ? `${h}:${String(m).padStart(2, '0')}:${x}` : `${m}:${x}`; };
const emptyBox = (ic, title, text, to, label) => `<div class="empty bx-empty">${icon(ic, 44)}<b>${esc(title)}</b><p>${esc(text)}</p>${to ? `<a class="btn primary" href="${to}" data-fid="emp">${esc(label)}</a>` : ''}</div>`;
const topBack = end => `<header class="bar over bx-top"><button type="button" class="ib back" data-act="back" aria-label="Zurück" data-fid="tb-back">${icon('back')}</button><span class="bx-sp"></span>${end || ''}</header>`;
const others = () => S.profiles.filter((p, k) => k !== S.profile).map(p => p.name);

/* ---------- progress and episodes ---------- */
/** S.progress: { [i]: { t, pct, at, s?, k? } } for films and series; S.epSeen: keys of seen episodes. */
function prog() {
  if (!S.progress) {
    S.progress = { 37: { s: 1, k: 2, t: Math.round(33 * 60 * 0.4), pct: 40, at: 1 } };
    S.epSeen = S.epSeen ?? ['01M7GJ8R88X7KCVT0PM1312ZAT', '01M7GJ8R83HZHTCNRNKDAM5BE8'];
  }
  return S.progress;
}
const epSeen = () => S.epSeen ?? (S.epSeen = []);
const eps = i => (D(i) && D(i).e) || [];
const seasonLen = (i, s) => { const se = ((D(i) || {}).s || []).find(x => x[0] === s); return se ? se[1] : (s === 1 ? eps(i).length : 0); };
/** Episode k (0-based) of season s. Season 1 comes from D(i).e; further rows reuse the stills. */
function epInfo(i, s, k) {
  const list = eps(i); if (!list.length || k < 0) return null;
  if (s === 1 && list[k]) { const e = list[k]; return { s, k, id: e[0], key: e[0], num: e[1], title: e[2], min: e[3], ov: e[4] }; }
  if (k >= seasonLen(i, s)) return null;
  const avg = Math.round(list.reduce((a, e) => a + e[3], 0) / list.length);
  return { s, k, id: list[k % list.length][0], key: `${i}:${s}:${k}`, num: k + 1, title: '', min: avg, ov: '' };
}
const epNext = (i, ep) => epInfo(i, ep.s, ep.k + 1) || epInfo(i, ep.s + 1, 0);
const epPrev = (i, ep) => (ep.k > 0 ? epInfo(i, ep.s, ep.k - 1) : ep.s > 1 ? epInfo(i, ep.s - 1, seasonLen(i, ep.s - 1) - 1) : null);
const epUrl = (i, ep) => `#/play/${i}?e=${ep.k}${ep.s > 1 ? `&s=${ep.s}` : ''}`;
const epLabel = ep => `S${ep.s} · F${ep.num}`;
const isEpSeen = (i, ep) => S.seen.includes(i) || epSeen().includes(ep.key);

/* «Weiterschauen» on the start page reads DATA.R.cont; stored progress goes in front of it. */
const CONT0 = DATA.R.cont.slice();
function syncCont() {
  const P = prog(); const ids = Object.keys(P).map(Number).sort((a, b) => (P[b].at || 0) - (P[a].at || 0));
  DATA.R.cont.splice(0, DATA.R.cont.length, ...ids.map(i => [i, Math.max(1, Math.round(P[i].pct))]), ...CONT0.filter(([i]) => !(i in P)));
}
syncCont();

/* ---------- shared rows ---------- */
function lrow(i, opts) {
  const o = opts || {}; const t = T(i); const audio = isSq(i);
  const sub = [audio ? t.author : t.year || '', KIND[t.kind], t.min && t.kind !== 'p' ? dur(t.min) : ''].filter(Boolean).join(' · ');
  const g = genres(i).slice(0, 2).join(' · ');
  const tag = o.act ? ` data-act="${o.act}" data-to="${href(i)}"` : '';
  return `<a class="lr" href="${href(i)}" data-fid="lr${o.fid || ''}${i}"${tag}>
    <span class="lr-im${audio ? ' sq' : ''}${t.kind === 'e' ? ' wide' : ''}">${poster(i)}</span>
    <span class="lr-tx"><b>${o.q ? hl(t.title, o.q) : esc(t.title)}</b><small>${o.q ? hl(sub, o.q) : esc(sub)}</small>${g && !o.q ? `<small>${esc(g)}</small>` : ''}</span>
    <span class="lr-end">${o.end || ''}${S.seen.includes(i) ? `<span class="bx-seen" title="Gesehen">${icon('check', 16)}</span>` : ''}${icon('right', 18)}</span></a>`;
}
function moreLike(i) {
  const g = genres(i); const lib = LIBS.find(L => L.ids().includes(i));
  const pool = (lib ? lib.ids() : DATA.T.map((x, j) => j).filter(j => T(j).kind === T(i).kind)).filter(j => j !== i);
  const score = j => genres(j).filter(x => g.includes(x)).length;
  return pool.map((j, n) => [j, score(j), n]).sort((a, b) => b[1] - a[1] || a[2] - b[2]).slice(0, 12).map(x => x[0]);
}

/* ============================================================
   Library
   ============================================================ */
const SORTS = [['added', 'Zuletzt hinzugefügt'], ['title', 'Titel A–Z'], ['year', 'Erscheinungsjahr'], ['rating', 'Bewertung']];
const libState = key => { ui.lib = ui.lib || {}; return ui.lib[key] || (ui.lib[key] = { f: 'all', g: '', y: '', sort: 'added', view: 'grid' }); };
const decade = y => (!y ? '' : y < 1990 ? 'Vor 1990' : `${Math.floor(y / 10) * 10}er`);
function libItems(L, st) {
  let ids = L.ids();
  if (st.f === 'unseen') ids = ids.filter(i => !S.seen.includes(i));
  if (st.g) ids = ids.filter(i => genres(i).includes(st.g));
  if (st.y) ids = ids.filter(i => decade(T(i).year) === st.y);
  const by = {
    title: (a, b) => T(a).title.replace(/^\W+/, '').localeCompare(T(b).title.replace(/^\W+/, ''), 'de'),
    year: (a, b) => T(b).year - T(a).year, rating: (a, b) => rating(b) - rating(a),
  }[st.sort];
  return by ? ids.slice().sort(by) : ids;
}
const countBy = (ids, fn) => { const m = new Map(); ids.forEach(i => [].concat(fn(i)).filter(Boolean).forEach(v => m.set(v, (m.get(v) || 0) + 1))); return m; };

route('/library/:key', p => {
  const L = LIBS.find(l => l.key === p.key);
  if (!L) return { nav: 'start', html: bar('Bibliothek') + emptyBox('film', 'Bibliothek nicht gefunden', 'Diese Bibliothek gibt es auf dem Server nicht.', '#/', 'Zur Startseite') };
  const st = libState(L.key); const ids = libItems(L, st); const filtered = st.f !== 'all' || st.g || st.y;
  const sq = L.key === 'books' || L.key === 'podcasts'; const years = L.key !== 'podcasts';
  const sortName = (SORTS.find(s => s[0] === st.sort) || SORTS[0])[1];
  const viewBtn = `<button type="button" class="ib" data-act="libView" data-k="${L.key}" data-fid="lib-view" aria-label="${st.view === 'grid' ? 'Als Liste zeigen' : 'Als Raster zeigen'}">${icon(st.view === 'grid' ? 'list' : 'grid')}</button>`;
  const chips = `<div class="chips bx-chips" role="toolbar" aria-label="Filter">
    <button type="button" class="chip" aria-pressed="${!filtered}" data-act="libAll" data-k="${L.key}" data-fid="lf-all">Alle</button>
    <button type="button" class="chip" aria-pressed="${st.f === 'unseen'}" data-act="libUnseen" data-k="${L.key}" data-fid="lf-un">${st.f === 'unseen' ? icon('check', 16) : ''}Ungesehen</button>
    <button type="button" class="chip bx-drop" aria-pressed="${!!st.g}" aria-haspopup="dialog" data-act="libSheet" data-m="g" data-k="${L.key}" data-fid="lf-g">${esc(st.g || 'Genre')}${icon('down', 16)}</button>
    ${years ? `<button type="button" class="chip bx-drop" aria-pressed="${!!st.y}" aria-haspopup="dialog" data-act="libSheet" data-m="y" data-k="${L.key}" data-fid="lf-y">${esc(st.y || 'Jahr')}${icon('down', 16)}</button>` : ''}</div>`;
  const tools = `<div class="bx-tools"><span class="bx-count" role="status">${filtered ? `${ids.length} ${ids.length === 1 ? 'Treffer' : 'Treffer'}` : `${nf(L.count)} Titel`}</span>
    <button type="button" class="bx-sort" data-act="libSheet" data-m="sort" data-k="${L.key}" data-fid="lf-sort" aria-haspopup="dialog">${icon('sort', 18)}<span>${esc(sortName)}</span></button></div>`;
  let body;
  if (!ids.length) {
    body = `<div class="empty bx-empty">${icon('filter', 44)}<b>Keine Titel</b><p>Kein Titel passt zu diesen Filtern.</p><button type="button" class="btn secondary" data-act="libAll" data-k="${L.key}" data-fid="lf-reset">Filter zurücksetzen</button></div>`;
  } else if (st.view === 'list') {
    body = `<div class="bx-list">${ids.map(i => lrow(i, { end: st.sort === 'rating' ? `<span class="bx-rate">${icon('star', 14)}${dec(rating(i))}</span>` : '' })).join('')}</div>`;
  } else {
    body = `<div class="grid bx-grid${sq ? ' sq' : ''}">${ids.map(i => card(i, { sq, tag: S.seen.includes(i) ? `${icon('check', 12)} Gesehen` : null })).join('')}</div>`;
  }
  return { nav: 'start', html: bar(L.name, { end: `<a class="ib" href="#/search" aria-label="Suchen" data-fid="lib-s">${icon('search')}</a>${viewBtn}` }) + chips + tools + body };
});
ACT.libAll = d => { Object.assign(libState(d.k), { f: 'all', g: '', y: '' }); render(); };
ACT.libUnseen = d => { const st = libState(d.k); st.f = st.f === 'unseen' ? 'all' : 'unseen'; render(); };
ACT.libView = d => { const st = libState(d.k); st.view = st.view === 'grid' ? 'list' : 'grid'; render(); };
ACT.libSet = d => { libState(d.k)[d.m] = d.v; closeSheet(true); render(); };
ACT.libSheet = d => {
  const L = LIBS.find(l => l.key === d.k); const st = libState(d.k); const all = L.ids();
  const opt = (v, label, sub) => `<button type="button" class="li bx-opt" role="radio" aria-checked="${st[d.m] === v}" data-act="libSet" data-k="${d.k}" data-m="${d.m}" data-v="${esc(v)}"><span class="tx"><b>${esc(label)}</b>${sub ? `<small>${esc(sub)}</small>` : ''}</span><span class="end">${st[d.m] === v ? icon('check', 22) : ''}</span></button>`;
  let h;
  if (d.m === 'sort') {
    h = `<h2>Sortieren nach</h2><div role="radiogroup" aria-label="Sortieren">${SORTS.filter(s => s[0] !== 'year' || d.k !== 'podcasts').map(([v, l]) => opt(v, l, v === 'added' ? 'Neueste zuerst' : v === 'year' ? 'Neueste zuerst' : v === 'rating' ? 'Beste zuerst' : '')).join('')}</div>`;
  } else if (d.m === 'g') {
    const m = countBy(all, genres); const list = [...m.entries()].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0], 'de'));
    h = `<h2>Genre</h2><div role="radiogroup" aria-label="Genre">${opt('', 'Alle Genres', `${all.length} Titel`)}${list.map(([g, n]) => opt(g, g, `${n} ${n === 1 ? 'Titel' : 'Titel'}`)).join('')}</div>`;
  } else {
    const m = countBy(all, i => decade(T(i).year)); const yv = y => parseInt(y, 10) || 0; const list = [...m.entries()].sort((a, b) => yv(b[0]) - yv(a[0]));
    h = `<h2>Erscheinungsjahr</h2><div role="radiogroup" aria-label="Jahr">${opt('', 'Alle Jahre', `${all.length} Titel`)}${list.map(([y, n]) => opt(y, y, `${n} Titel`)).join('')}</div>`;
  }
  sheet(`${h}<div class="acts"><button type="button" class="btn ghost" data-act="close">Schliessen</button></div>`);
};
PAGES.push(['#/library/films', 'Bibliothek Filme'], ['#/library/series', 'Bibliothek Serien'], ['#/library/books', 'Bibliothek Hörbücher'], ['#/library/podcasts', 'Bibliothek Podcasts']);

/* ============================================================
   Detail page
   ============================================================ */
const CODEC = { hevc: 'HEVC (H.265)', h264: 'H.264 (AVC)', av1: 'AV1' };
const ACODEC = { eac3: 'Dolby Digital Plus', ac3: 'Dolby Digital', dts: 'DTS', aac: 'AAC', truehd: 'Dolby TrueHD' };
const resLabel = h => (h > 1100 ? `4K · 3840 × ${h}` : h > 720 ? `1080p · 1920 × ${h}` : h > 480 ? `720p · 1280 × ${h}` : `SD · ${h} Zeilen`);
const resShort = h => (h > 1100 ? '4K' : h > 720 ? '1080p' : h > 480 ? '720p' : 'SD');

function epRow(i, ep, pr) {
  const seen = isEpSeen(i, ep); const cur = pr && pr.s === ep.s && pr.k === ep.k ? pr : null;
  const title = ep.title ? `${ep.num}. ${esc(ep.title)}` : `Folge ${ep.num}`;
  const meta = [`${ep.title ? '' : 'ca. '}${ep.min} Min.`, cur ? `noch ${Math.max(1, Math.round(ep.min * (100 - cur.pct) / 100))} Min.` : '', seen ? 'gesehen' : ''].filter(Boolean).join(' · ');
  return `<div class="ep${ep.title ? '' : ' plain'}">
    <a class="ep-link" href="${epUrl(i, ep)}" data-fid="ep${ep.s}-${ep.k}" aria-label="${esc(epLabel(ep))}${ep.title ? ` · ${esc(ep.title)}` : ''} abspielen">
      <span class="ep-im">${pic(`e-${ep.id}.webp`, '')}<span class="ep-play">${icon('play', 16)}</span>${seen ? `<span class="ep-seen">${icon('check', 14)}</span>` : ''}${cur ? `<span class="prog"><i style="width:${Math.max(4, cur.pct)}%"></i></span>` : ''}</span>
      <span class="ep-tx"><b>${title}</b><small>${esc(meta)}</small></span>
      ${ep.ov ? `<span class="ep-ov">${esc(ep.ov)}</span>` : ''}</a>
    <button type="button" class="ib ep-dl" data-act="dlAsk" data-i="${i}"${ep.title ? ` data-e="${ep.k}"` : ` data-s="${ep.s}" data-n="${ep.num}"`} data-fid="dl${ep.s}-${ep.k}" aria-label="${esc(epLabel(ep))} herunterladen">${icon('download', 20)}</button></div>`;
}
function episodes(i, d, pr) {
  const seasons = d.s || [[1, eps(i).length, 0]];
  const s = Number(ui.tab['s' + i] || (pr && pr.s) || 1); const n = seasonLen(i, s);
  const rows = Array.from({ length: n }, (x, k) => epInfo(i, s, k)).filter(Boolean).map(ep => epRow(i, ep, pr)).join('');
  return `<section class="dt-sec" aria-label="Folgen"><div class="dt-h"><h2>Folgen</h2><span>${n} Folgen</span></div>
    <div class="chips dt-seasons" role="tablist" aria-label="Staffeln">${seasons.map(([num, c]) => `<button type="button" class="chip" role="tab" aria-selected="${num === s}" aria-pressed="${num === s}" data-act="season" data-i="${i}" data-n="${num}" data-fid="se${num}">Staffel ${num}</button>`).join('')}</div>
    <div class="eps">${rows}</div></section>`;
}
ACT.season = d => { ui.tab['s' + d.i] = Number(d.n); render(); };
ACT.recommend = () => { const o = others(); toast(o.length ? `Empfohlen für ${o.join(' und ')}` : 'Empfohlen'); };
ACT.trailer = () => toast('Öffnet YouTube');

function techSection(i, f) {
  const open = !!ui.open['tech' + i];
  const rows = [['Video', CODEC[f[0]] || f[0].toUpperCase()], ['Auflösung', resLabel(f[1])], ['Ton', `${ACODEC[f[2]] || f[2].toUpperCase()} · 5.1`],
    ['Grösse', `${dec(f[3])} GB`], ['Container', f[4] === 'mkv' ? 'MKV (Matroska)' : f[4].toUpperCase()], ['Fassungen', f[5] === 1 ? '1 Fassung' : `${f[5]} Fassungen`]];
  return `<section class="dt-sec dt-fold-sec"><button type="button" class="dt-fold" aria-expanded="${open}" aria-controls="tech${i}" data-act="toggle" data-scope="ui" data-path="open.tech${i}" data-fid="tech">
      <span class="lead">${icon('info', 20)}</span><span class="tx"><b>Datei und Technik</b><small>${esc([(CODEC[f[0]] || f[0]).split(' ')[0], resShort(f[1]), `${dec(f[3])} GB`].join(' · '))}</small></span>${icon(open ? 'up' : 'down', 20)}</button>
    ${open ? `<dl class="dt-tech" id="tech${i}">${rows.map(([k, v]) => `<div><dt>${k}</dt><dd>${esc(v)}</dd></div>`).join('')}</dl>` : ''}</section>`;
}

route('/item/:i', p => {
  const i = Number(p.i);
  if (!DATA.T[i]) return { nav: 'start', html: bar('Titel') + emptyBox('film', 'Titel nicht gefunden', 'Diesen Titel gibt es nicht oder du darfst ihn nicht sehen.', '#/', 'Zur Startseite') };
  const t = T(i); const d = D(i) || {}; const k = t.kind; const ser = k === 's'; const audio = isSq(i); const P = prog()[i];
  const inW = S.watch.includes(i); const seen = S.seen.includes(i);
  const g = genres(i);
  const nSeasons = d.s ? d.s.length : 0;
  const meta = [t.year ? `<span>${t.year}</span>` : '', t.fsk ? fsk(t.fsk) : '',
    ser ? (nSeasons ? `<span>${nSeasons} ${nSeasons === 1 ? 'Staffel' : 'Staffeln'}</span>` : '<span>Serie</span>') : t.min ? `<span>${dur(t.min)}</span>` : `<span>${KIND[k]}</span>`,
    g.length ? `<span>${esc(g.join(', '))}</span>` : ''].filter(Boolean).join('<i aria-hidden="true">·</i>');
  const name = d.l ? `<h1 class="dt-title logo"><img src="img/l-${i}.webp" alt="${esc(t.title)}"></h1>` : `<h1 class="dt-title">${esc(t.title)}</h1>`;
  const hero = hasB(i) ? backdrop(i) : k === 'e' ? poster(i) : pic(`p-${i}.webp`, '', 'blur');

  /* primary action */
  let cta;
  if (audio) {
    cta = `<a class="btn primary dt-play" href="#/listen/${i}" data-fid="play">${icon('headphones', 22)} Anhören</a>`;
  } else {
    let label = 'Abspielen'; let to = `#/play/${i}`; let rest = '';
    if (ser) {
      const ep = P ? epInfo(i, P.s, P.k) : d.n ? epInfo(i, d.n[0], d.n[1] - 1) : epInfo(i, 1, 0);
      if (ep) { label = `${epLabel(ep)} ${P && P.pct > 0 ? 'fortsetzen' : 'abspielen'}`; to = epUrl(i, ep); }
      if (P && P.pct > 0 && ep) rest = `noch ${Math.max(1, Math.round(ep.min * (100 - P.pct) / 100))} Min.${ep.title ? ` · ${ep.title}` : ''}`;
    } else if (P) { label = 'Weiterschauen'; rest = `noch ${Math.max(1, Math.round((t.min || 30) * (100 - P.pct) / 100))} Min.`; }
    cta = `<a class="btn primary dt-play" href="${to}" data-fid="play">${icon('play', 22)} ${esc(label)}</a>
      ${rest ? `<div class="dt-prog"><span class="bx-pbar"><i style="width:${Math.max(3, P.pct)}%"></i></span><span>${esc(rest)}</span><a href="${to}${to.includes('?') ? '&' : '?'}from=0" data-fid="from0">Von vorn</a></div>` : ''}`;
  }
  const acts = [
    ['watch', inW ? 'bookmarkOn' : 'bookmark', inW ? 'Gemerkt' : 'Merken', inW],
    ['seen', 'check', 'Gesehen', seen],
    ['dlAsk', 'download', 'Herunterladen', false],
    ['recommend', 'heart', 'Empfehlen', false],
    audio ? null : ['trailer', 'film', 'Trailer', false],
  ].filter(Boolean).map(([a, ic, label, on]) => `<button type="button" class="dt-act${on ? ' on' : ''}" data-act="${a}" data-i="${i}" data-fid="a-${a}"${a === 'watch' || a === 'seen' ? ` aria-pressed="${on}"` : ''}><span class="ico">${icon(ic, 22)}</span><span class="lb">${label}</span></button>`).join('');

  /* text and lists */
  const ovOpen = !!ui.open['ov' + i]; const long = d.o && d.o.length > 170;
  const ov = d.o ? `<section class="dt-sec dt-ov-sec"><p class="dt-ov${long && !ovOpen ? ' clamp' : ''}" id="ov${i}">${esc(d.o)}</p>
      ${long ? `<button type="button" class="dt-more" aria-expanded="${ovOpen}" aria-controls="ov${i}" data-act="toggle" data-scope="ui" data-path="open.ov${i}" data-fid="ovm">${ovOpen ? 'Weniger anzeigen' : 'Mehr anzeigen'}</button>` : ''}</section>`
    : `<section class="dt-sec"><p class="dt-ov dim">${audio ? `${esc(t.author)}${t.min ? ` · ${dur(t.min)}` : ''}` : 'Zu diesem Titel liefert der Server keine Inhaltsangabe.'}</p></section>`;
  const cast = d.c ? `<section class="dt-sec" aria-label="Besetzung und Team"><div class="dt-h"><h2>Besetzung und Team</h2></div>
      <div class="cast">${d.c.map(([n, r, id], x) => `<a class="cp" href="#/person/${id}" data-fid="cp${x}"><span class="ci">${pic(`c-${id}.webp`, n)}</span><b>${esc(n)}</b><small>${esc(roleText(r))}</small></a>`).join('')}</div></section>` : '';
  const epsHtml = ser && eps(i).length ? episodes(i, d, P) : '';
  const tech = d.f ? techSection(i, d.f) : '';
  const more = moreLike(i);

  const html = `<div class="dt${audio ? ' sq' : ''}${k === 'e' ? ' still' : ''}">
    ${topBack(`<a class="ib" href="#/search" aria-label="Suchen" data-fid="dt-s">${icon('search')}</a>`)}
    <div class="dt-bg">${hero}<span class="dt-shade"></span></div>
    <div class="dt-cols">
      <div class="dt-l">
        <div class="dt-poster${audio ? ' sq' : ''}${k === 'e' ? ' wide' : ''}">${poster(i)}</div>
        <div class="dt-cta">${cta}</div>
        <div class="dt-acts">${acts}</div>
      </div>
      <div class="dt-r">
        <div class="dt-head">${name}<div class="dt-meta">${meta}</div>${d.t ? `<p class="dt-tag">${esc(d.t)}</p>` : ''}</div>
        <div class="dt-main">${ov}${cast}${epsHtml}${tech}</div>
      </div>
    </div>
    ${more.length ? hrow('Mehr davon', more) : ''}</div>`;
  return { nav: 'start', html };
});
PAGES.push(['#/item/0', 'Film: Spider-Man (Details)'], ['#/item/2', 'Serie: Die Ringe der Macht'], ['#/item/37', 'Serie: Normal People (angefangen)'], ['#/item/75', 'Film ohne Einzelheiten']);

/* ============================================================
   Person
   ============================================================ */
function people() {
  const m = new Map();
  for (const [k, d] of Object.entries(DATA.D)) for (const [n, r, id] of d.c || []) { const p = m.get(id) || { id, name: n, roles: [] }; p.roles.push([Number(k), r]); m.set(id, p); }
  return [...m.values()];
}
route('/person/:id', p => {
  const id = Number(p.id); const per = people().find(x => x.id === id);
  if (!per) return { nav: 'start', html: bar('Person') + emptyBox('user', 'Person nicht gefunden', 'Zu dieser Person kennt die Bibliothek keine Titel.', '#/', 'Zur Startseite') };
  const ids = [...new Set(per.roles.map(r => r[0]))];
  const jobs = [...new Set(per.roles.map(([, r]) => (ROLE[r] ? ROLE[r] : 'Schauspiel')))].join(' · ');
  const sub = i => per.roles.filter(r => r[0] === i).map(([, r]) => (ROLE[r] ? ROLE[r] : `als ${r}`)).join(', ');
  const html = `<div class="pp">${topBack()}
    <div class="pp-head"><div class="pp-im">${pic(`c-${id}.webp`, per.name)}</div>
      <div class="pp-tx"><h1>${esc(per.name)}</h1><p>${esc(jobs)}</p><span class="badge">${ids.length} ${ids.length === 1 ? 'Titel' : 'Titel'} in deiner Bibliothek</span></div></div>
    <div class="rowh"><h2>Bekannt aus</h2></div>
    <div class="grid bx-grid">${ids.map(i => card(i, { sub: sub(i) })).join('')}</div></div>`;
  return { nav: 'start', html };
});
PAGES.push(['#/person/1881', 'Person: Tom Holland'], ['#/person/15247', 'Person: J. R. R. Tolkien']);

/* ============================================================
   Search
   ============================================================ */
const recentQ = () => S.recentQ ?? (S.recentQ = ['Resident Evil', 'Tolkien', 'Cube']);
function addRecent(q) { const v = String(q || '').trim(); if (v.length < 2) return; const r = recentQ().filter(x => norm(x) !== norm(v)); r.unshift(v); S.recentQ = r.slice(0, 6); save(); }
function hl(text, q) {
  const s = String(text || ''); const k = s.toLowerCase().indexOf(String(q || '').trim().toLowerCase());
  if (!q || !q.trim() || k < 0) return esc(s);
  const n = q.trim().length; return `${esc(s.slice(0, k))}<mark>${esc(s.slice(k, k + n))}</mark>${esc(s.slice(k + n))}`;
}
const GROUPS = [['m', 'Filme'], ['s', 'Serien'], ['e', 'Folgen'], ['a', 'Hörbücher'], ['p', 'Podcasts']];
function searchBody() {
  const q = String(ui.q || '').trim();
  if (!q) {
    const r = recentQ();
    const libTiles = LIBS.map(L => {
      const first = L.ids()[0]; const sq = isSq(first);
      return `<a class="tile${sq ? ' sq' : ''}" href="#/library/${L.key}" data-fid="tl-${L.key}"><span class="tile-im">${sq ? poster(first) : backdrop(first)}</span>
        <span class="tile-tx">${icon(L.icon, 20)}<b>${esc(L.name)}</b><small>${nf(L.count)} Titel</small></span></a>`;
    }).join('');
    const films = LIBS[0].ids().concat([0, 1, 3, 4, 30]);
    const gTiles = ['Action', 'Science Fiction', 'Drama', 'Komödie', 'Horror', 'Thriller'].map(g => {
      const pick = films.find(i => genres(i).includes(g) && hasB(i));
      return `<button type="button" class="tile g" data-act="browseGenre" data-k="films" data-g="${g}" data-fid="tg-${g}"><span class="tile-im">${pick != null ? backdrop(pick) : ''}</span><span class="tile-tx"><b>${esc(g)}</b><small>Filme</small></span></button>`;
    }).join('');
    return `${r.length ? `<section><div class="rowh"><h2>Zuletzt gesucht</h2><button type="button" class="bx-link" data-act="qForget" data-fid="qf">Verlauf löschen</button></div>
        <div class="chips">${r.map((x, n) => `<button type="button" class="chip" data-act="qRecent" data-q="${esc(x)}" data-fid="qr${n}">${icon('clock', 16)} ${esc(x)}</button>`).join('')}</div></section>` : ''}
      <section><div class="rowh"><h2>Stöbern</h2></div><div class="tiles">${libTiles}</div></section>
      <section><div class="rowh"><h2>Genres</h2></div><div class="tiles">${gTiles}</div></section>`;
  }
  const nq = norm(q);
  const hits = DATA.T.map((x, i) => i).filter(i => norm(T(i).title).includes(nq) || norm(T(i).author).includes(nq));
  const per = people().filter(x => norm(x.name).includes(nq));
  const total = hits.length + per.length;
  const wish = `<section class="sr-wish"><div class="rowh"><h2>Noch nicht in der Bibliothek?</h2></div>
    ${li('wish', `«${q}» wünschen`, 'Suche bei Seerr und schicke einen Wunsch', null, `href="#/wishes" data-fid="sr-wish"`)}</section>`;
  if (!total) return `<div class="empty bx-empty">${icon('search', 44)}<b>Nichts gefunden</b><p>Für «${esc(q)}» gibt es keinen Titel und keine Person. Prüfe die Schreibweise oder versuche einen kürzeren Begriff.</p></div>${wish}`;
  const groups = GROUPS.map(([k, label]) => {
    const ids = hits.filter(i => T(i).kind === k); if (!ids.length) return '';
    return `<section class="sr-g"><div class="rowh"><h2>${label}</h2><span class="bx-n">${ids.length}</span></div><div class="bx-list">${ids.map(i => lrow(i, { q, act: 'qKeep', fid: 's' })).join('')}</div></section>`;
  }).join('');
  const pp = per.length ? `<section class="sr-g"><div class="rowh"><h2>Personen</h2><span class="bx-n">${per.length}</span></div><div class="bx-list">${per.map(x => `<a class="lr" href="#/person/${x.id}" data-act="qKeep" data-to="#/person/${x.id}" data-fid="sp${x.id}">
      <span class="lr-im round">${pic(`c-${x.id}.webp`, x.name)}</span><span class="lr-tx"><b>${hl(x.name, q)}</b><small>${esc([...new Set(x.roles.map(([, r]) => ROLE[r] || 'Schauspiel'))].join(' · '))} · ${x.roles.length} ${x.roles.length === 1 ? 'Titel' : 'Titel'}</small></span><span class="lr-end">${icon('right', 18)}</span></a>`).join('')}</div></section>` : '';
  return `<p class="sr-count" role="status">${total} ${total === 1 ? 'Treffer' : 'Treffer'}</p>${groups}${pp}${wish}`;
}
route('/search', () => ({
  nav: 'search',
  html: `<header class="sr-head"><form class="sr-box" role="search" onsubmit="return false">${icon('search', 22)}
      <input id="q" class="sr-inp" type="search" value="${esc(ui.q || '')}" data-bind="ui.q" data-fid="q" placeholder="Titel, Person, Autorin …" autocomplete="off" enterkeyhint="search" aria-label="Bibliothek durchsuchen">
      <button type="button" class="ib sr-clear" data-act="qClear" data-fid="qc" aria-label="Suche leeren"${ui.q ? '' : ' hidden'}>${icon('close', 20)}</button></form></header>
    <div id="sres">${searchBody()}</div>`,
  after: root => {
    const inp = $('#q', root); if (!inp) return;
    inp.addEventListener('input', () => { ui.q = inp.value; $('.sr-clear', root).hidden = !inp.value; $('#sres', root).innerHTML = searchBody(); });
    inp.addEventListener('keydown', e => { if (e.key === 'Enter') { addRecent(inp.value); inp.blur(); } });
  },
}));
ACT.qClear = () => { ui.q = ''; render(); const q = $('#q'); if (q) q.focus(); };
ACT.qRecent = d => { ui.q = d.q; addRecent(d.q); render(); };
ACT.qForget = () => { S.recentQ = []; toast('Suchverlauf gelöscht'); commit(); };
ACT.qKeep = d => { addRecent(ui.q); go(d.to); };
ACT.browseGenre = d => { Object.assign(libState(d.k), { f: 'all', g: d.g, y: '' }); go(`#/library/${d.k}`); };
PAGES.push(['#/search', 'Suche']);

/* ============================================================
   Lists, watchlist, watched
   ============================================================ */
const LISTS = { rel: 'Neu erschienen', rec: 'Zuletzt hinzugefügt', picks: 'Empfohlen von anderen Profilen' };
route('/list/:key', p => {
  const title = LISTS[p.key]; const ids = title ? row(p.key) : [];
  if (!title || !ids.length) return { nav: 'start', html: bar(title || 'Liste') + emptyBox('list', 'Diese Liste ist leer', 'Hier ist gerade nichts zu sehen.', '#/', 'Zur Startseite') };
  const o = others(); const opts = p.key === 'picks' ? (i => ({ tag: `${icon('heart', 12)} ${esc(o[i % o.length] || 'Tipp')}` })) : null;
  return {
    nav: 'start',
    html: `${bar(title)}<p class="bx-intro">${p.key === 'picks' ? 'Titel, die andere Profile dieses Kontos dir empfehlen.' : `${ids.length} Titel`}</p>
      <div class="grid bx-grid">${ids.map(i => card(i, typeof opts === 'function' ? opts(i) : null)).join('')}</div>`,
  };
});
function editList(key, ids, act, label, emptyIc, emptyT, emptyX) {
  const edit = !!ui.open[key + 'Edit'] && ids.length;
  const end = ids.length ? `<button type="button" class="btn ghost sm bx-edit" data-act="toggle" data-scope="ui" data-path="open.${key}Edit" data-fid="${key}-edit" aria-pressed="${!!edit}">${edit ? 'Fertig' : 'Bearbeiten'}</button>` : '';
  if (!ids.length) return bar(label, { end }) + emptyBox(emptyIc, emptyT, emptyX, '#/', 'Zur Startseite');
  return `${bar(label, { end })}<p class="bx-intro">${ids.length} Titel${edit ? ' · tippe auf ✕, um einen Titel zu entfernen' : ''}</p>
    <div class="grid bx-grid${edit ? ' editing' : ''}">${ids.map(i => `<div class="bx-wl">${card(i)}${edit ? `<button type="button" class="bx-x" data-act="${act}" data-i="${i}" data-fid="x${i}" aria-label="${esc(T(i).title)} entfernen">${icon('close', 18)}</button>` : ''}</div>`).join('')}</div>`;
}
route('/watchlist', () => ({ nav: 'start', html: editList('wl', S.watch, 'watch', 'Merkliste', 'bookmark', 'Deine Merkliste ist leer', 'Tippe bei einem Titel auf «Merken», dann findest du ihn hier.') }));
route('/watched', () => ({ nav: 'start', html: editList('ws', S.seen.slice().reverse(), 'seen', 'Gesehen', 'eye', 'Noch nichts gesehen', 'Was du fertig schaust oder als gesehen markierst, steht hier.') }));
PAGES.push(['#/list/rel', 'Liste: Neu erschienen'], ['#/list/picks', 'Liste: Empfehlungen'], ['#/watchlist', 'Merkliste'], ['#/watched', 'Gesehen']);

/* ============================================================
   Collections
   ============================================================ */
const colMembers = k => { const L = row('lib1'); const n = Math.max(1, DATA.col[k][2]); return Array.from({ length: n }, (x, j) => L[(k * 3 + j) % L.length]); };
route('/collections', () => ({
  nav: 'start',
  html: `${bar('Sammlungen')}<p class="bx-intro">${DATA.col.length} Sammlungen</p>
    <div class="grid bx-grid">${DATA.col.map(([id, name, n]) => `<a class="pc" href="#/collection/${id}" data-fid="col${id}"><div class="im">${pic(`k-${id}.webp`, name)}</div><div class="t">${esc(name)}</div><div class="m">${n} ${n === 1 ? 'Titel' : 'Titel'}</div></a>`).join('')}</div>`,
}));
route('/collection/:id', p => {
  const k = DATA.col.findIndex(c => c[0] === p.id);
  if (k < 0) return { nav: 'start', html: bar('Sammlung') + emptyBox('film', 'Sammlung nicht gefunden', 'Diese Sammlung gibt es nicht mehr.', '#/collections', 'Alle Sammlungen') };
  const [id, name] = DATA.col[k]; const ids = colMembers(k); const allIn = ids.every(i => S.watch.includes(i));
  return {
    nav: 'start',
    html: `<div class="colp">${topBack()}<div class="colp-bg">${pic(`k-${id}.webp`, '', 'blur')}<span class="dt-shade"></span></div>
      <div class="colp-head"><div class="colp-im">${pic(`k-${id}.webp`, name)}</div>
        <div class="colp-tx"><span class="bx-over">Sammlung</span><h1>${esc(name)}</h1><p>${ids.length} ${ids.length === 1 ? 'Titel' : 'Titel'}</p>
          <div class="colp-acts"><a class="btn primary" href="#/play/${ids[0]}" data-fid="col-play">${icon('play', 20)} Abspielen</a>
          <button type="button" class="btn secondary" data-act="colWatch" data-k="${k}" data-fid="col-w" aria-pressed="${allIn}">${icon(allIn ? 'bookmarkOn' : 'bookmark', 20)} ${allIn ? 'Gemerkt' : 'Alle merken'}</button></div></div></div>
      <div class="rowh"><h2>In dieser Sammlung</h2></div><div class="grid bx-grid">${ids.map(i => card(i)).join('')}</div></div>`,
  };
});
ACT.colWatch = d => {
  const ids = colMembers(Number(d.k)); const allIn = ids.every(i => S.watch.includes(i));
  if (allIn) { S.watch = S.watch.filter(i => !ids.includes(i)); toast('Von der Merkliste entfernt'); } else { ids.forEach(i => { if (!S.watch.includes(i)) S.watch.unshift(i); }); toast(`${ids.length} Titel gemerkt`); }
  commit();
};
PAGES.push(['#/collections', 'Sammlungen'], ['#/collection/01M5N7PTTWWDJ78BHEJ70EAQFS', 'Sammlung: 28 Days Later']);

/* ============================================================
   Video player
   ============================================================ */
const PL = { active: false, key: null, i: null, ep: null, t: 0, dur: 60, playing: true, ctl: true, hideAt: 0, speed: 1, audio: 0, subs: 'Aus', quality: 'Automatisch', drag: null, hover: null, bub: 0, cancelNext: false, timer: null, img: '' };
const plAudio = i => { const f = (D(i) || {}).f; const c = f ? ACODEC[f[2]] || f[2].toUpperCase() : 'Dolby Digital'; return [`Deutsch · ${c} 5.1`, 'Englisch · DTS-HD 5.1', 'Deutsch · Audiodeskription · Stereo']; };
const SUBS = [['Aus', ''], ['Deutsch', 'Alle Dialoge'], ['Deutsch (erzwungen)', 'Nur fremdsprachige Stellen und Schilder'], ['Englisch', 'Alle Dialoge']];
const QUAL = [['Automatisch', 'Passt sich der Verbindung an'], ['Original', ''], ['1080p', 'ca. 3,2 GB pro Stunde'], ['720p', 'ca. 1,6 GB pro Stunde'], ['480p', 'ca. 0,7 GB pro Stunde']];
const SPEEDS = [0.5, 0.75, 1, 1.25, 1.5, 2];
const speedText = v => (v === 1 ? 'Normal' : `${String(v).replace('.', ',')}×`);

function plInit(i, s, k, q) {
  const t = T(i); const ep = t.kind === 's' ? (epInfo(i, s, k) || epInfo(i, 1, 0)) : null;
  const dur = Math.max(60, (ep ? ep.min : (t.min || 30)) * 60);
  let start = 0; const pr = prog()[i];
  if (q.at === 'end') start = dur - 27;
  else if (q.from !== '0') {
    if (pr && (!ep || (pr.s === ep.s && pr.k === ep.k))) start = pr.t;
    else if (!ep) { const c = CONT0.find(x => x[0] === i); if (c) start = dur * c[1] / 100; }
  }
  const img = ep ? `e-${ep.id}.webp` : hasB(i) ? `b-${i}.webp` : `p-${i}.webp`;
  Object.assign(PL, { active: true, key: `${i}|${ep ? `${ep.s}|${ep.k}` : ''}`, i, ep, dur, t: Math.max(0, Math.min(start, dur - 5)), playing: true, ctl: true, hideAt: Date.now() + 4000, drag: null, hover: null, bub: 0, cancelNext: false, img });
}
function plStore() {
  if (PL.i == null) return;
  const P = prog(); const i = PL.i; const pct = PL.t / PL.dur * 100;
  if (PL.ep) {
    if (pct >= 92) {
      if (!epSeen().includes(PL.ep.key)) epSeen().push(PL.ep.key);
      const nx = epNext(i, PL.ep);
      if (nx) P[i] = { s: nx.s, k: nx.k, t: 0, pct: 0, at: Date.now() }; else { delete P[i]; if (!S.seen.includes(i)) S.seen.push(i); }
    } else if (PL.t >= 10) P[i] = { s: PL.ep.s, k: PL.ep.k, t: Math.round(PL.t), pct, at: Date.now() };
  } else if (pct >= 95) { delete P[i]; if (!S.seen.includes(i)) S.seen.push(i); } else if (PL.t >= 10) P[i] = { t: Math.round(PL.t), pct, at: Date.now() };
  syncCont(); save();
}
function plLeave() { plStore(); clearInterval(PL.timer); PL.active = false; PL.key = null; render(); }
window.addEventListener('hashchange', () => { if (PL.active && !location.hash.startsWith('#/play/')) plLeave(); });
function plGoEp(ep) { if (!ep) return; PL.playing = false; plStore(); go(epUrl(PL.i, ep)); }
const plWake = () => { PL.hideAt = Date.now() + 4000; };

function plNextCard() {
  if (!PL.ep || PL.cancelNext) return null;
  const nx = epNext(PL.i, PL.ep); const rem = PL.dur - PL.t; if (!nx || rem > 25) return null;
  return { nx, n: Math.max(0, Math.ceil(rem - 15)) };
}
function plPaint() {
  const v = $('.vp'); if (!v) return;
  v.classList.toggle('ctl-off', !PL.ctl);
  const pos = PL.drag != null ? PL.drag : PL.t / PL.dur; const cur = pos * PL.dur;
  $('#vpEl').textContent = tc(cur); $('#vpRem').textContent = `-${tc(PL.dur - cur)}`;
  $('#vpFill').style.width = `${pos * 100}%`; $('#vpKnob').style.left = `${pos * 100}%`;
  const tr = $('#vpTrack'); tr.setAttribute('aria-valuenow', String(Math.round(cur))); tr.setAttribute('aria-valuetext', `${tc(cur)} von ${tc(PL.dur)}`);
  const pp = $('#vpPP'); const st = PL.playing ? 'pause' : 'play';
  if (pp.dataset.st !== st) { pp.dataset.st = st; pp.innerHTML = icon(st, 40); pp.setAttribute('aria-label', PL.playing ? 'Pause' : 'Abspielen'); }
  /* seek preview bubble */
  const bub = $('#vpBub'); const showBub = PL.drag != null || PL.hover != null || Date.now() < PL.bub;
  const bpos = PL.drag != null ? PL.drag : PL.hover != null ? PL.hover : pos; const bcur = bpos * PL.dur;
  bub.hidden = !showBub;
  if (showBub) {
    const w = tr.offsetWidth; const bw = bub.offsetWidth || 176;
    bub.style.left = `${Math.min(w - bw / 2, Math.max(bw / 2, bpos * w))}px`;
    $('#vpBubT').textContent = tc(bcur); $('img', bub).style.objectPosition = `${Math.round(bpos * 100)}% 50%`;
  }
  /* skip intro in the first minute of an episode */
  $('#vpIntro').hidden = !(PL.ep && cur < 60);
  /* next episode card */
  const nc = plNextCard(); const box = $('#vpNext');
  if (!nc) { box.hidden = true; box.dataset.n = ''; } else if (box.dataset.n !== String(nc.n)) {
    box.hidden = false; box.dataset.n = String(nc.n);
    box.innerHTML = `<span class="vp-next-im">${pic(`e-${nc.nx.id}.webp`, '')}<span class="vp-ring" style="--p:${(10 - Math.min(10, nc.n)) * 10}"></span></span>
      <div class="vp-next-tx"><small>Nächste Folge in ${nc.n}</small><b>${esc(epLabel(nc.nx))}${nc.nx.title ? ` · ${esc(nc.nx.title)}` : ''}</b>
      <div class="vp-next-acts"><button type="button" class="btn primary sm" data-act="plNextNow" data-fid="vn-go">${icon('play', 18)} Jetzt abspielen</button><button type="button" class="btn ghost sm" data-act="plNextCancel" data-fid="vn-no">Abbrechen</button></div></div>`;
  }
}
function plTick() {
  if (!PL.active || !$('.vp')) { clearInterval(PL.timer); return; }
  if (PL.playing && PL.drag == null) {
    PL.t = Math.min(PL.dur, PL.t + PL.speed);
    const nc = plNextCard();
    if (nc && PL.dur - PL.t <= 15) { plGoEp(nc.nx); return; }
    if (PL.t >= PL.dur) { PL.playing = false; PL.ctl = true; }
  }
  if (PL.playing && PL.ctl && Date.now() > PL.hideAt && !$('#overlays .scrim') && PL.drag == null && PL.hover == null && !(document.activeElement && $('.vp').contains(document.activeElement) && document.activeElement.matches(':focus-visible'))) PL.ctl = false;
  plPaint();
}
function plSeekTo(sec, bubble) { PL.t = Math.max(0, Math.min(PL.dur - 1, sec)); if (bubble) PL.bub = Date.now() + 900; plWake(); plPaint(); }

route('/play/:i', (p, q) => {
  const i = Number(p.i);
  if (!DATA.T[i]) return { nav: 'start', html: bar('Wiedergabe') + emptyBox('film', 'Titel nicht gefunden', 'Diesen Titel gibt es nicht.', '#/', 'Zur Startseite') };
  const t = T(i); const pr = prog()[i];
  let s = Number(q.s || 1); let k = Number(q.e || 0);
  if (q.e == null && t.kind === 's') { if (pr) { s = pr.s; k = pr.k; } else if (D(i) && D(i).n) { s = D(i).n[0]; k = D(i).n[1] - 1; } }
  const epx = t.kind === 's' && eps(i).length ? (epInfo(i, s, k) || epInfo(i, 1, 0)) : null;
  const key = `${i}|${epx ? `${epx.s}|${epx.k}` : ''}`;
  if (PL.active && PL.key !== key) plStore();
  if (!PL.active || PL.key !== key) plInit(i, s, k, q);
  const ep = PL.ep; const prev = ep && epPrev(i, ep); const next = ep && epNext(i, ep);
  const line1 = ep ? `${epLabel(ep)}${ep.title ? ` · ${ep.title}` : ''}` : t.title;
  const line2 = ep ? t.title : [t.year || '', KIND[t.kind], t.min ? dur(t.min) : ''].filter(Boolean).join(' · ');
  const tool = (m, ic, label, on) => `<button type="button" class="vp-tool${on ? ' on' : ''}" data-act="plMenu" data-m="${m}" data-fid="vt-${m}" aria-haspopup="dialog">${icon(ic, 22)}<span>${label}</span></button>`;
  const html = `<div class="vp" data-fid="vp">
    <div class="vp-pic${hasB(i) || ep || t.kind === 'e' ? '' : ' contain'}"><img src="img/${PL.img}" alt=""></div>
    <div class="vp-ui" data-act="plTap">
      <div class="vp-top" data-act="plNoop">
        <button type="button" class="ib" data-act="plBack" data-fid="vp-back" aria-label="Player schliessen">${icon('back')}</button>
        <div class="vp-ttl"><b>${esc(line1)}</b><small>${esc(line2)}</small></div>
        <button type="button" class="ib" data-act="plMenu" data-m="info" data-fid="vp-info" aria-label="Wiedergabe-Info">${icon('info')}</button>
      </div>
      <div class="vp-mid">
        <button type="button" class="vp-jump" data-act="plSeek" data-d="-10" data-fid="vp-rew" aria-label="10 Sekunden zurück">${icon('rew', 34)}<span>10</span></button>
        <button type="button" class="vp-pp" id="vpPP" data-act="plPlay" data-fid="vp-pp" aria-label="Pause">${icon('pause', 40)}</button>
        <button type="button" class="vp-jump" data-act="plSeek" data-d="10" data-fid="vp-fwd" aria-label="10 Sekunden vor">${icon('fwd', 34)}<span>10</span></button>
      </div>
      <div class="vp-bot" data-act="plNoop">
        <div class="vp-seek"><span id="vpEl" class="vp-time">0:00</span>
          <div class="vp-track" id="vpTrack" role="slider" tabindex="0" data-fid="vp-track" aria-label="Position" aria-valuemin="0" aria-valuemax="${Math.round(PL.dur)}" aria-valuenow="0">
            <span class="vp-rail"></span><span class="vp-buf" style="width:${Math.min(100, (PL.t / PL.dur) * 100 + 18)}%"></span><span class="vp-fill" id="vpFill"></span><span class="vp-knob" id="vpKnob"></span>
            <div class="vp-bub" id="vpBub" hidden><img src="img/${PL.img}" alt=""><b id="vpBubT">0:00</b></div></div>
          <span id="vpRem" class="vp-time">-0:00</span></div>
        <div class="vp-tools">
          ${tool('audio', 'audio', 'Ton', PL.audio !== 0)}${tool('subs', 'subs', 'Untertitel', PL.subs !== 'Aus')}${tool('quality', 'hd', 'Qualität', PL.quality !== 'Automatisch')}${tool('speed', 'speed', PL.speed === 1 ? 'Geschwindigkeit' : speedText(PL.speed), PL.speed !== 1)}
          <span class="vp-sp"></span>
          ${ep ? `${tool('eps', 'list', 'Folgen', false)}
            <button type="button" class="ib vp-ep" data-act="plEp" data-d="-1" data-fid="vp-prev" aria-label="Vorige Folge"${prev ? '' : ' disabled'}>${icon('prev', 22)}</button>
            <button type="button" class="ib vp-ep" data-act="plEp" data-d="1" data-fid="vp-next" aria-label="Nächste Folge"${next ? '' : ' disabled'}>${icon('next', 22)}</button>` : ''}
        </div>
      </div>
    </div>
    <button type="button" class="btn secondary vp-intro" id="vpIntro" data-act="plIntro" data-fid="vp-intro" hidden>Vorspann überspringen ${icon('next', 18)}</button>
    <div class="vp-next" id="vpNext" role="status" hidden></div>
  </div>`;
  return {
    full: true, rot: 'l', html,
    after: root => {
      const tr = $('#vpTrack', root);
      const at = e => { const r = tr.getBoundingClientRect(); return Math.min(1, Math.max(0, (e.clientX - r.left) / r.width)); };
      tr.addEventListener('pointerdown', e => { e.preventDefault(); tr.setPointerCapture(e.pointerId); PL.drag = at(e); plWake(); plPaint(); });
      tr.addEventListener('pointermove', e => { if (PL.drag != null) { PL.drag = at(e); plPaint(); } else if (e.pointerType === 'mouse') { PL.hover = at(e); plWake(); plPaint(); } });
      tr.addEventListener('pointerleave', () => { PL.hover = null; plPaint(); });
      tr.addEventListener('pointerup', e => { if (PL.drag == null) return; const v = PL.drag; PL.drag = null; plSeekTo(v * PL.dur, true); e.stopPropagation(); });
      tr.addEventListener('pointercancel', () => { PL.drag = null; plPaint(); });
      tr.addEventListener('keydown', e => {
        const step = { ArrowLeft: -10, ArrowRight: 10, PageDown: -60, PageUp: 60 }[e.key];
        if (step) { e.preventDefault(); e.stopPropagation(); plSeekTo(PL.t + step, true); } else if (e.key === 'Home') { e.preventDefault(); plSeekTo(0, true); } else if (e.key === 'End') { e.preventDefault(); plSeekTo(PL.dur - 30, true); }
      });
      clearInterval(PL.timer); PL.timer = setInterval(plTick, 1000); plPaint();
    },
  };
});
document.addEventListener('keydown', e => {
  if (!PL.active || !$('.vp') || $('#overlays .scrim')) return;
  const tag = e.target.tagName;
  if ((e.key === ' ' || e.key === 'k') && tag !== 'BUTTON' && tag !== 'INPUT') { e.preventDefault(); ACT.plPlay(); }
  else if ((e.key === 'ArrowLeft' || e.key === 'ArrowRight') && e.target.id !== 'vpTrack') { e.preventDefault(); plSeekTo(PL.t + (e.key === 'ArrowLeft' ? -10 : 10), true); }
});
ACT.plTap = () => { PL.ctl = !PL.ctl; plWake(); plPaint(); };
ACT.plNoop = () => plWake();
ACT.plBack = () => history.back();
ACT.plPlay = () => { if (PL.t >= PL.dur - 1) PL.t = 0; PL.playing = !PL.playing; PL.ctl = true; plWake(); plPaint(); };
ACT.plSeek = d => plSeekTo(PL.t + Number(d.d), true);
ACT.plIntro = () => { plSeekTo(90, false); toast('Vorspann übersprungen'); };
ACT.plEp = d => plGoEp(Number(d.d) < 0 ? epPrev(PL.i, PL.ep) : epNext(PL.i, PL.ep));
ACT.plGo = d => { closeSheet(true); plGoEp(epInfo(PL.i, Number(d.s), Number(d.k))); };
ACT.plNextNow = () => { const nc = plNextCard(); if (nc) plGoEp(nc.nx); };
ACT.plNextCancel = () => { PL.cancelNext = true; plPaint(); };
ACT.plSet = d => {
  const v = d.m === 'speed' ? Number(d.v) : d.m === 'audio' ? Number(d.v) : d.v; PL[d.m] = v; closeSheet(true);
  const label = { audio: 'Ton', subs: 'Untertitel', quality: 'Qualität', speed: 'Geschwindigkeit' }[d.m];
  toast(`${label}: ${d.m === 'audio' ? plAudio(PL.i)[v] : d.m === 'speed' ? speedText(v) : v}`); plWake(); render();
};
ACT.plMenu = d => {
  const i = PL.i; const f = (D(i) || {}).f; plWake();
  const opt = (m, v, label, sub, on) => `<button type="button" class="li bx-opt" role="radio" aria-checked="${on}" data-act="plSet" data-m="${m}" data-v="${esc(v)}"><span class="tx"><b>${esc(label)}</b>${sub ? `<small>${esc(sub)}</small>` : ''}</span><span class="end">${on ? icon('check', 22) : ''}</span></button>`;
  const close = '<div class="acts"><button type="button" class="btn ghost" data-act="close">Schliessen</button></div>';
  let h = '';
  if (d.m === 'audio') h = `<h2>Ton</h2><div role="radiogroup" aria-label="Tonspur">${plAudio(i).map((a, n) => opt('audio', n, a, n === 0 ? 'Standard' : '', PL.audio === n)).join('')}</div>`;
  else if (d.m === 'subs') h = `<h2>Untertitel</h2><div role="radiogroup" aria-label="Untertitel">${SUBS.map(([v, sub]) => opt('subs', v, v, sub, PL.subs === v)).join('')}</div>`;
  else if (d.m === 'quality') {
    const orig = f ? `${resShort(f[1])} · ${(CODEC[f[0]] || f[0]).split(' ')[0]} · ca. ${dec(f[3] / ((T(i).min || 60) / 60))} GB pro Stunde` : 'Datei wie auf dem Server';
    h = `<h2>Qualität</h2><div role="radiogroup" aria-label="Qualität">${QUAL.map(([v, sub]) => opt('quality', v, v, v === 'Original' ? orig : sub, PL.quality === v)).join('')}</div>
      <div class="note bx-note">${icon('wifi', 20)}<span>Über mobile Daten zählt jede Stunde: «Original» braucht am meisten. ${S.prefs && S.prefs.wifiOnly ? 'Downloads laufen bei dir nur im WLAN.' : ''}</span></div>`;
  } else if (d.m === 'speed') h = `<h2>Geschwindigkeit</h2><div role="radiogroup" aria-label="Geschwindigkeit">${SPEEDS.map(v => opt('speed', v, speedText(v), '', PL.speed === v)).join('')}</div>`;
  else if (d.m === 'eps' && PL.ep) {
    const s = PL.ep.s; const list = Array.from({ length: seasonLen(i, s) }, (x, k) => epInfo(i, s, k)).filter(Boolean);
    h = `<h2>Staffel ${s}</h2><div class="vp-eps">${list.map(ep => { const on = ep.k === PL.ep.k; return `<button type="button" class="vp-epi${on ? ' on' : ''}" data-act="${on ? 'close' : 'plGo'}" data-s="${ep.s}" data-k="${ep.k}" aria-current="${on}">
        <span class="ep-im">${pic(`e-${ep.id}.webp`, '')}${isEpSeen(i, ep) ? `<span class="ep-seen">${icon('check', 14)}</span>` : ''}</span>
        <span class="ep-tx"><b>${ep.title ? `${ep.num}. ${esc(ep.title)}` : `Folge ${ep.num}`}</b><small>${on ? 'Läuft gerade' : `${ep.min} Min.`}</small></span></button>`; }).join('')}</div>`;
  } else if (d.m === 'info') {
    const direct = PL.quality === 'Automatisch' || PL.quality === 'Original';
    const rows = [['Abspielart', direct ? 'Direktwiedergabe' : `Transcode auf ${PL.quality} (H.264)`],
      ['Video', f ? `${CODEC[f[0]] || f[0]} · ${resLabel(f[1])}` : 'H.264 · 1080p'],
      ['Ton', plAudio(i)[PL.audio]], ['Untertitel', PL.subs],
      ['Bitrate', f ? `${dec(f[3] * 8000 / ((T(i).min || 60) * 60))} Mbit/s` : 'ca. 6 Mbit/s'],
      ['Container', f ? f[4].toUpperCase() : 'MKV'], ['Gerät', 'Pixel 9 · Android 16']];
    h = `<h2>Wiedergabe-Info</h2><dl class="dt-tech bx-info">${rows.map(([a, b]) => `<div><dt>${a}</dt><dd>${esc(b)}</dd></div>`).join('')}</dl>`;
  }
  if (h) sheet(h + close);
};
PAGES.push(['#/play/0', 'Player: Film'], ['#/play/2?e=0', 'Player: Serie S1 · F1 (Vorspann)'], ['#/play/2?e=2&at=end', 'Player: Countdown nächste Folge']);
