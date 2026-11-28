/* Konzept 28 «Live-TV», phone and tablet mockup core: helpers, state, router, media drawer (with the new
   entry «Live-TV» after the web radio), device frame. Copied from the mockup of Konzept 27 and adapted:
   no setup, no library data (Live-TV data comes from ../shared/), images from ../shared/img/lib/ (only
   local). Plain scripts, all functions global. Owned by the core. */
'use strict';

/* ---------- helpers ---------- */
const $ = (sel, root) => (root || document).querySelector(sel);
const $$ = (sel, root) => Array.from((root || document).querySelectorAll(sel));
const esc = s => String(s ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));

const ICONS = {
  home: '<path d="M3 10.5 12 3l9 7.5V20a1 1 0 0 1-1 1h-5v-6h-6v6H4a1 1 0 0 1-1-1z"/>',
  search: '<circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/>',
  download: '<path d="M12 3v12m0 0-4.5-4.5M12 15l4.5-4.5M4 20h16"/>',
  wish: '<path d="M12 3l2.6 5.4 5.9.8-4.3 4.1 1 5.9L12 16.4 6.8 19.2l1-5.9L3.5 9.2l5.9-.8z"/>',
  more: '<path d="M4 6h16M4 12h16M4 18h16"/>',
  play: '<path d="M7 4.5v15l12-7.5z" fill="currentColor"/>',
  pause: '<path d="M7 5v14M17 5v14" stroke-width="3"/>',
  back: '<path d="M19 12H5m0 0 6-6m-6 6 6 6"/>',
  close: '<path d="M6 6l12 12M18 6 6 18"/>',
  check: '<path d="m5 12.5 4.5 4.5L19 7.5"/>',
  plus: '<path d="M12 5v14M5 12h14"/>',
  bookmark: '<path d="M6 4h12v17l-6-4.5L6 21z"/>',
  bookmarkOn: '<path d="M6 4h12v17l-6-4.5L6 21z" fill="currentColor"/>',
  right: '<path d="m9 5 7 7-7 7"/>',
  down: '<path d="m5 9 7 7 7-7"/>',
  up: '<path d="m5 15 7-7 7 7"/>',
  settings: '<circle cx="12" cy="12" r="3"/><path d="M12 2v3m0 14v3M2 12h3m14 0h3M4.9 4.9l2.1 2.1m10 10 2.1 2.1M19.1 4.9 17 7M7 17l-2.1 2.1"/>',
  user: '<circle cx="12" cy="8" r="4"/><path d="M4 21c1.2-4 4.3-6 8-6s6.800 2 8 6"/>',
  users: '<circle cx="9" cy="8" r="3.5"/><path d="M2.5 20c.9-3.300 3.400-5 6.500-5s5.600 1.700 6.500 5M16 4.800a3.500 3.500 0 0 1 0 6.400M18 15.300c1.800.700 3 2.300 3.500 4.700"/>',
  tv: '<rect x="2.500" y="4" width="19" height="13" rx="2"/><path d="M8 21h8"/>',
  qr: '<path d="M4 4h6v6H4zM14 4h6v6h-6zM4 14h6v6H4zM14 14h2v2h-2zM18 14h2M14 18h2m2 0h2v2"/>',
  radio: '<rect x="3" y="9" width="18" height="11" rx="2"/><path d="m7 9 10-5"/><circle cx="8.500" cy="14.500" r="2.500"/><path d="M15 13h3m-3 3h3"/>',
  headphones: '<path d="M4 15v-3a8 8 0 0 1 16 0v3"/><rect x="3" y="14" width="4" height="6" rx="1.500"/><rect x="17" y="14" width="4" height="6" rx="1.500"/>',
  mic: '<rect x="9" y="3" width="6" height="11" rx="3"/><path d="M5 11a7 7 0 0 0 14 0M12 18v3"/>',
  bell: '<path d="M6 16V11a6 6 0 0 1 12 0v5l2 2H4zM10 20a2 2 0 0 0 4 0"/>',
  filter: '<path d="M4 6h16M7 12h10M10 18h4"/>',
  sort: '<path d="M7 5v14m0 0-3-3m3 3 3-3M17 19V5m0 0-3 3m3-3 3 3"/>',
  star: '<path d="M12 3l2.600 5.400 5.900.800-4.300 4.100 1 5.900L12 16.400 6.800 19.200l1-5.900L3.500 9.200l5.900-.800z" fill="currentColor"/>',
  info: '<circle cx="12" cy="12" r="9"/><path d="M12 11v6m0-9.500v.500"/>',
  lock: '<rect x="5" y="10" width="14" height="10" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3"/>',
  trash: '<path d="M4 7h16M9 7V4h6v3m-8 0 1 13h8l1-13"/>',
  wifi: '<path d="M2.500 9a14 14 0 0 1 19 0M6 12.500a9 9 0 0 1 12 0M9.500 16a4 4 0 0 1 5 0"/><circle cx="12" cy="19.500" r="1" fill="currentColor"/>',
  subs: '<rect x="3" y="5" width="18" height="14" rx="2"/><path d="M7 14h4m3 0h3M7 10.500h2m3 0h5"/>',
  audio: '<path d="M4 9v6h4l5 4V5L8 9zM16.500 8.500a5 5 0 0 1 0 7M19 6a9 9 0 0 1 0 12"/>',
  fwd: '<path d="M12 5a7 7 0 1 1-6.600 4.700M12 5l-3-3m3 3-3 3"/>',
  rew: '<path d="M12 5a7 7 0 1 0 6.600 4.700M12 5l3-3m-3 3 3 3"/>',
  next: '<path d="M6 5v14l10-7z" fill="currentColor"/><path d="M19 5v14"/>',
  prev: '<path d="M18 5v14L8 12z" fill="currentColor"/><path d="M5 5v14"/>',
  list: '<path d="M8 6h12M8 12h12M8 18h12M4 6h.010M4 12h.010M4 18h.010"/>',
  grid: '<rect x="4" y="4" width="7" height="7" rx="1"/><rect x="13" y="4" width="7" height="7" rx="1"/><rect x="4" y="13" width="7" height="7" rx="1"/><rect x="13" y="13" width="7" height="7" rx="1"/>',
  film: '<rect x="3" y="4" width="18" height="16" rx="2"/><path d="M8 4v16M16 4v16M3 9h5m8 0h5M3 15h5m8 0h5"/>',
  book: '<path d="M5 4h11a3 3 0 0 1 3 3v13H8a3 3 0 0 1-3-3zM5 17a3 3 0 0 1 3-3h11"/>',
  eye: '<path d="M2 12s3.600-7 10-7 10 7 10 7-3.600 7-10 7S2 12 2 12z"/><circle cx="12" cy="12" r="3"/>',
  clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3.500 2"/>',
  alert: '<path d="M12 3 2 20h20zM12 10v4m0 3v.500"/>',
  logout: '<path d="M10 4H5v16h5M14 8l4 4-4 4m4-4H9"/>',
  phone: '<rect x="7" y="2.500" width="10" height="19" rx="2.500"/><path d="M11 18.500h2"/>',
  tablet: '<rect x="3" y="5" width="18" height="14" rx="2.500"/>',
  server: '<rect x="3" y="4" width="18" height="7" rx="2"/><rect x="3" y="13" width="18" height="7" rx="2"/><path d="M7 7.500h.010M7 16.500h.010"/>',
  link: '<path d="M10 14a4 4 0 0 0 6 0l3-3a4.200 4.200 0 0 0-6-6l-1 1M14 10a4 4 0 0 0-6 0l-3 3a4.200 4.200 0 0 0 6 6l1-1"/>',
  refresh: '<path d="M20 5v5h-5M4 19v-5h5M19 10a7.500 7.500 0 0 0-13-2.500M5 14a7.500 7.500 0 0 0 13 2.500"/>',
  heart: '<path d="M12 20s-7.500-4.600-7.500-10A4.300 4.300 0 0 1 12 7.400 4.300 4.300 0 0 1 19.500 10c0 5.400-7.500 10-7.500 10z"/>',
  share: '<path d="M12 15V4m0 0L8 8m4-4 4 4M5 13v6h14v-6"/>',
  dots: '<circle cx="12" cy="5" r="1.600" fill="currentColor"/><circle cx="12" cy="12" r="1.600" fill="currentColor"/><circle cx="12" cy="19" r="1.600" fill="currentColor"/>',
  hd: '<rect x="3" y="6" width="18" height="12" rx="2"/><path d="M7 9.500v5m0-2.500h3m0-2.500v5M14 9.500v5h1.500a2.500 2.500 0 0 0 0-5z"/>',
  speed: '<path d="M5 17a8 8 0 1 1 14 0M12 13l4-4"/>',
  moon: '<path d="M20 14.500A8.500 8.500 0 0 1 9.500 4 8.500 8.500 0 1 0 20 14.500z"/>',
  key: '<circle cx="8" cy="15" r="4"/><path d="m11 12 9-9m-4 4 3 3"/>',
  globe: '<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3c3 3.300 3 14.700 0 18M12 3c-3 3.300-3 14.700 0 18"/>',
  shield: '<path d="M12 3 5 6v6c0 4.500 3 7.700 7 9 4-1.300 7-4.500 7-9V6z"/>',
  live: '<path d="M12 13.5v7M9 20.5h6"/><circle cx="12" cy="11" r="1.8"/><path d="M8.2 7.5a5.2 5.2 0 0 0 0 7M15.8 7.5a5.2 5.2 0 0 1 0 7M5.5 5a9 9 0 0 0 0 12M18.5 5a9 9 0 0 1 0 12"/>',
  record: '<circle cx="12" cy="12" r="6.5" fill="currentColor" stroke="none"/>',
  reminder: '<path d="M6 16V11a6 6 0 0 1 12 0v5l2 2H4zM10 20a2 2 0 0 0 4 0"/><path d="M12 8.5V11l1.8 1.2"/>',
  calendar: '<rect x="4" y="5.5" width="16" height="14" rx="2"/><path d="M4 10h16M8.5 3.5v4M15.5 3.5v4"/>',
  heartOn: '<path d="M12 20s-7.5-4.6-7.5-10A4.3 4.3 0 0 1 12 7.4 4.3 4.3 0 0 1 19.5 10c0 5.4-7.5 10-7.5 10z" fill="currentColor"/>',
  film: '<rect x="3" y="4" width="18" height="16" rx="2"/><path d="M8 4v16M16 4v16M3 9h5m8 0h5M3 15h5m8 0h5"/>',
  storage: '<ellipse cx="12" cy="6" rx="8" ry="3"/><path d="M4 6v12c0 1.700 3.600 3 8 3s8-1.300 8-3V6M4 12c0 1.700 3.600 3 8 3s8-1.300 8-3"/>',
};
const icon = (name, size) => `<svg width="${size || 24}" height="${size || 24}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${ICONS[name] || ICONS.info}</svg>`;

/* ---------- pictures ---------- */
const dur = min => (min >= 60 ? `${Math.floor(min / 60)} Std. ${min % 60} Min.` : `${min} Min.`);
/** picture from ../shared/img/lib/ (only local); a missing file becomes a calm field with the alt text */
const pic = (file, alt, cls) => `<img src="../shared/img/lib/${file}" alt="${esc(alt || '')}" loading="lazy"${cls ? ` class="${cls}"` : ''} onerror="this.outerHTML='&lt;div class=&quot;ph&quot;&gt;${esc(esc(alt || ''))}&lt;/div&gt;'">`;
/* media of the drawer: the libraries stay unchanged (Konzept 27); Live-TV is new, after the web radio */
const LIBS = [
  { key: 'films', name: 'Filme', icon: 'film', count: 1846 }, { key: 'series', name: 'Serien', icon: 'tv', count: 505 },
  { key: 'docs', name: 'Dokus', icon: 'globe', count: 63 }, { key: 'books', name: 'Hörbücher', icon: 'headphones', count: 89 },
  { key: 'podcasts', name: 'Podcasts', icon: 'mic', count: 100 },
];

/* ---------- state ----------
   Live-TV state lives in LIVE_SIM (../shared/live-sim.js). S keeps only mockup preferences. */
const KEY = 'sc28-handy-mock-v1';
function seed() { return { prefs: { wifiOnly: true, quality: 'Hoch (1080p)' }, playing: null }; }
let S;
try { S = JSON.parse(localStorage.getItem(KEY)) || seed(); } catch (e) { S = seed(); }
const save = () => { try { localStorage.setItem(KEY, JSON.stringify(S)); } catch (e) { /* private mode */ } };
const ui = { open: {}, draft: {}, q: '', tab: {} };
/** the active profile in the shape of the phone mockup: { id, name, color, main, age } */
const me = () => { const p = LIVE_SIM.profile(); return { id: p.id, name: p.name, color: `var(--av-${p.accent})`, main: p.primary, age: p.age }; };

/* ---------- router ---------- */
const ROUTES = [];
function route(pattern, fn) {
  const names = []; const re = new RegExp('^' + pattern.replace(/:[a-z]+/g, m => { names.push(m.slice(1)); return '([^/]+)'; }) + '$');
  ROUTES.push({ re, names, fn, pattern });
}
function go(hash) { if (location.hash === hash) render(); else location.hash = hash; }
function current() {
  const [path, qs] = (location.hash.slice(1) || '/').split('?');
  const query = Object.fromEntries(new URLSearchParams(qs || ''));
  for (const r of ROUTES) { const m = path.match(r.re); if (m) { const p = {}; r.names.forEach((n, k) => { p[n] = decodeURIComponent(m[k + 1]); }); return { r, p, query, path }; } }
  return null;
}
const NAV = [
  ['start', 'Start', 'home', '#/'], ['search', 'Suche', 'search', '#/search'], ['downloads', 'Downloads', 'download', '#/downloads'],
  ['wishes', 'Wünsche', 'wish', '#/wishes'], ['more', 'Mehr', 'dots', '#/more'],
];

/* ---------- media selection: drawer on the left (phone), permanent panel (tablet) ---------- */
function mediaNav() {
  const path = (location.hash.slice(1) || '/').split('?')[0];
  const it = (h, ic, label, end) => { const on = path === h.slice(1); return `<a class="dn${on ? ' on' : ''}" href="${h}"${on ? ' aria-current="page"' : ''} data-fid="dn${h}"><span class="dn-i">${icon(ic, 22)}</span><span class="dn-t">${esc(label)}</span>${end !== '' && end != null ? `<span class="dn-e">${end}</span>` : ''}</a>`; };
  const liveOn = path.startsWith('/live');
  const live = LIVE_SIM.rights().liveTv ? `<a class="dn${liveOn ? ' on' : ''}" href="#/live"${liveOn ? ' aria-current="page"' : ''} data-fid="dn#/live"><span class="dn-i">${icon('live', 22)}</span><span class="dn-t">Live-TV</span><span class="dn-e">${LIVE_SIM.player() ? 'Live' : LIVE_SIM.channels().length + ' Sender'}</span></a>` : '';
  return `<div class="dn-sec">Medien</div>${LIBS.map(l => it('#/library/' + l.key, l.icon, l.name, l.count.toLocaleString('de-CH'))).join('')}${it('#/radio', 'radio', 'Webradio', 3)}${live}
    <div class="dn-sec">Meine Medien</div>${it('#/watchlist', 'bookmark', 'Merkliste', 12)}${it('#/watched', 'eye', 'Gesehen', '')}${it('#/collections', 'grid', 'Sammlungen', 337)}`;
}
const railShown = () => getComputedStyle($('#rail')).display !== 'none';
function openDrawer() {
  if (railShown() || document.body.classList.contains('full')) return;
  const p = me(); const d = $('#drawer');
  d.innerHTML = `<div class="dr-scrim" data-act="drawerClose"></div><aside class="dr-panel" role="dialog" aria-modal="true" aria-label="Medienauswahl">
    <a class="dr-head" href="#/profiles">${avatar(p, 44)}<span class="tx"><b>${esc(p.name)}</b><small>Profil wechseln</small></span>${icon('right', 18)}</a>
    <nav class="dr-nav" aria-label="Medienauswahl">${mediaNav()}</nav></aside>`;
  void d.offsetWidth; /* start from the closed position so the panel slides in */
  d.classList.add('open'); const f = $('.dr-nav a', d); if (f) f.focus({ preventScroll: true });
}
function closeDrawer() { const d = $('#drawer'); if (d) d.classList.remove('open'); }
const drawerOpen = () => $('#drawer').classList.contains('open');
(function drawerSetup() {
  const dev = $('#device');
  const d = document.createElement('div'); d.id = 'drawer'; dev.appendChild(d);
  const edge = document.createElement('div'); edge.id = 'edge'; edge.setAttribute('aria-hidden', 'true'); dev.appendChild(edge);
  const scale = () => dev.getBoundingClientRect().width / dev.offsetWidth;
  let st = null;
  edge.addEventListener('pointerdown', e => { if (railShown()) return; st = { x: e.clientX, sc: scale() }; edge.setPointerCapture(e.pointerId); });
  edge.addEventListener('pointermove', e => { if (st && (e.clientX - st.x) / st.sc > 32) { st = null; openDrawer(); } });
  d.addEventListener('pointerdown', e => { if (drawerOpen()) st = { x: e.clientX, sc: scale(), close: true }; });
  d.addEventListener('pointermove', e => { if (st && st.close && (e.clientX - st.x) / st.sc < -40) { st = null; closeDrawer(); } });
  ['pointerup', 'pointercancel'].forEach(n => { edge.addEventListener(n, () => { st = null; }); d.addEventListener(n, () => { st = null; }); });
})();
let lastPath = null;
function render() {
  const c = current();
  const path = (location.hash.slice(1) || '/').split('?')[0];
  const v = liveGate(path) || (c ? c.r.fn(c.p, c.query) : unchangedPage(path)) || {};
  document.body.classList.toggle('full', !!v.full);
  document.body.classList.toggle('dark-status', !!v.darkStatus);
  if ((v.rot || null) !== forcedRot) { forcedRot = v.rot || null; fit(); }
  const scroll = $('#scroll'); const keep = lastPath === path ? scroll.scrollTop : 0;
  const fid = document.activeElement && document.activeElement.dataset ? document.activeElement.dataset.fid : null;
  $('#view').innerHTML = v.html || '';
  const navHtml = NAV.map(([k, label, ic, h]) => `<a href="${h}"${v.nav === k ? ' aria-current="page"' : ''}><span class="pill">${icon(ic, 22)}</span>${label}</a>`).join('');
  $('#tabs').innerHTML = navHtml;
  /* in the wide column only one entry is marked: a media entry wins over «Start» */
  const mediaOn = /^\/(library\/|radio$|watchlist$|watched$|collections$|live)/.test(path);
  const railNav = (mediaOn ? navHtml.replace(/ aria-current="page"/g, '') : navHtml).replace(/<a /g, '<a class="rn" ');
  $('#rail').innerHTML = `<div class="brand"><span>S</span><b>Slouch<i>cat</i></b></div>${railNav}<div class="rail-media">${mediaNav()}</div>`;
  scroll.scrollTop = keep; lastPath = path;
  if (fid) { const el = document.querySelector(`[data-fid="${fid}"]`); if (el) el.focus({ preventScroll: true }); }
  drawMini();
  if (v.after) v.after($('#view'));
  LIVE_MOCK.syncJump($('#jump'));
  LIVE_UI.refresh($('#device'));
}
const commit = () => { save(); render(); };
window.addEventListener('hashchange', () => { closeSheet(true); closeDrawer(); render(); });

/* ---------- actions ---------- */
const ACT = {};
document.addEventListener('click', e => {
  const tool = e.target.closest('[data-tool]'); if (tool) { TOOL[tool.dataset.tool](tool.dataset.v); return; }
  const el = e.target.closest('[data-act]'); if (!el) return;
  const fn = ACT[el.dataset.act]; if (!fn) return;
  if (el.tagName === 'A') e.preventDefault();
  fn(el.dataset, el, e);
});
document.addEventListener('input', e => {
  const el = e.target.closest('[data-bind]'); if (!el) return;
  const path = el.dataset.bind.split('.'); let o = path[0] === 'S' ? S : ui;
  for (let k = 1; k < path.length - 1; k++) o = o[path[k]] = o[path[k]] || {};
  o[path[path.length - 1]] = el.type === 'checkbox' ? el.checked : el.value;
  if (path[0] === 'S') save();
  if (el.dataset.live != null) render();
});
ACT.back = () => history.back();
ACT.go = d => go(d.to);
ACT.toggle = d => { const o = d.scope === 'S' ? S : ui; const p = d.path.split('.'); let t = o; for (let k = 0; k < p.length - 1; k++) t = t[p[k]] = t[p[k]] || {}; t[p[p.length - 1]] = !t[p[p.length - 1]]; commit(); };
ACT.set = d => { const p = d.path.split('.'); let t = p[0] === 'S' ? S : ui; for (let k = 1; k < p.length - 1; k++) t = t[p[k]] = t[p[k]] || {}; t[p[p.length - 1]] = d.num != null ? Number(d.value) : d.value; closeSheet(true); commit(); };
ACT.close = () => closeSheet();
ACT.toast = d => toast(d.msg);
ACT.watch = d => { const i = Number(d.i); const k = S.watch.indexOf(i); if (k < 0) { S.watch.unshift(i); toast('Zur Merkliste hinzugefügt'); } else { S.watch.splice(k, 1); toast('Von der Merkliste entfernt'); } commit(); };
ACT.seen = d => { const i = Number(d.i); const k = S.seen.indexOf(i); if (k < 0) S.seen.push(i); else S.seen.splice(k, 1); toast(k < 0 ? 'Als gesehen markiert' : 'Markierung entfernt'); commit(); };

/* ---------- sheets and snack ---------- */
function sheet(html, opts) { $('#overlays').innerHTML = `<div class="scrim${opts && opts.center ? ' center' : ''}" data-act="scrim"><div class="sheet" role="dialog" aria-modal="true">${html}</div></div>`; const f = $('#overlays [autofocus], #overlays button, #overlays input'); if (f) f.focus(); }
function closeSheet(silent) { const o = $('#overlays'); const sn = $('.snack', o); o.innerHTML = ''; if (sn && silent) o.appendChild(sn); }
ACT.scrim = (d, el, e) => { if (e.target === el) closeSheet(); };
let snackT;
function toast(msg) { const o = $('#overlays'); const old = $('.snack', o); if (old) old.remove(); const n = document.createElement('div'); n.className = 'snack'; n.setAttribute('role', 'status'); n.textContent = msg; o.appendChild(n); clearTimeout(snackT); snackT = setTimeout(() => n.remove(), 2600); }
document.addEventListener('keydown', e => { if (e.key === 'Escape') { if (drawerOpen()) closeDrawer(); else if ($('#overlays .scrim')) closeSheet(); else history.back(); } });
ACT.drawer = () => openDrawer();
ACT.drawerClose = () => closeDrawer();

/* ---------- shared page parts ---------- */
const menuBtn = () => `<button type="button" class="ib menu" data-act="drawer" aria-label="Medienauswahl öffnen" data-fid="menu">${icon('more')}</button>`;
const bar = (title, opts) => { const o = opts || {}; return `<header class="bar${o.over ? ' over' : ''}">${o.back === false ? menuBtn() : `<button type="button" class="ib back" data-act="back" aria-label="Zurück">${icon('back')}</button>`}<h1>${esc(title)}</h1>${o.end || ''}</header>`; };
const li = (ic, title, sub, end, attrs) => `<${attrs && attrs.includes('href=') ? 'a' : 'button type="button"'} class="li" ${attrs || ''}><span class="lead">${typeof ic === 'string' && ic.startsWith('<') ? ic : icon(ic, 22)}</span><span class="tx"><b>${esc(title)}</b>${sub ? `<small>${esc(sub)}</small>` : ''}</span><span class="end">${end ?? icon('right', 18)}</span></${attrs && attrs.includes('href=') ? 'a' : 'button'}>`;
const sw = (scope, path, label, sub) => { const p = path.split('.'); let t = scope === 'S' ? S : ui; for (const k of p) t = t ? t[k] : undefined; return `<button type="button" class="li" role="switch" aria-checked="${!!t}" data-act="toggle" data-scope="${scope}" data-path="${path}" data-fid="sw-${path}"><span class="tx"><b>${esc(label)}</b>${sub ? `<small>${esc(sub)}</small>` : ''}</span><span class="sw" aria-checked="${!!t}"></span></button>`; };
const avatar = (p, size) => `<span class="avatar" style="background:${p.color};${size ? `width:${size}px;height:${size}px;font-size:${Math.round(size * 0.42)}px` : ''}">${esc(p.name[0])}</span>`;
function drawMini() { const m = $('#mini'); if (!m) return; if (!S.playing || document.body.classList.contains('full')) { m.innerHTML = ''; return; } const p = S.playing; m.innerHTML = `<div class="minip"><a href="${p.href}" style="display:contents"><img src="../shared/img/lib/${p.img}" alt=""><span class="tx"><b>${esc(p.title)}</b><small>${esc(p.sub)}</small></span></a><button type="button" class="ib" data-act="miniToggle" aria-label="${p.paused ? 'Weiter' : 'Pause'}">${icon(p.paused ? 'play' : 'pause')}</button><button type="button" class="ib" data-act="miniStop" aria-label="Beenden">${icon('close')}</button></div>`; }
ACT.miniToggle = () => { S.playing.paused = !S.playing.paused; save(); drawMini(); };
ACT.miniStop = () => { S.playing = null; save(); drawMini(); };

/* ---------- device frame ---------- */
const DEV = { phone: [412, 915], tablet: [800, 1280] };
const frame = { view: 'phone', rot: 'p' };
let forcedRot = null; /* a page may force an orientation with { rot: 'l' } (video player on a phone) */
try { Object.assign(frame, JSON.parse(localStorage.getItem(KEY + '-frame')) || {}); } catch (e) { /* ignore */ }
function fit() {
  const real = window.innerWidth <= 600; const dev = $('#device'); const box = $('#fit');
  document.body.dataset.view = frame.view;
  if (real) { dev.style.cssText = 'width:100vw;height:100dvh;transform:none'; box.style.cssText = 'width:100vw;height:100dvh'; return; }
  let [w, h] = DEV[frame.view]; if ((forcedRot || frame.rot) === 'l') [w, h] = [h, w];
  document.body.dataset.rot = forcedRot || frame.rot;
  const st = $('#stage'); const s = Math.min((st.clientWidth - 56) / w, (st.clientHeight - 56) / h, 1.15);
  dev.style.cssText = `width:${w}px;height:${h}px;transform:scale(${s})`;
  box.style.cssText = `width:${w * s}px;height:${h * s}px`;
  $$('[data-tool="view"]').forEach(b => b.setAttribute('aria-pressed', String(b.dataset.v === frame.view)));
  $$('[data-tool="rot"]').forEach(b => b.setAttribute('aria-pressed', String(b.dataset.v === frame.rot)));
}
const TOOL = {
  view: v => { frame.view = v; frame.rot = v === 'tablet' ? 'l' : 'p'; saveFrame(); },
  rot: v => { frame.rot = v; saveFrame(); },
  reset: () => { try { localStorage.setItem(KEY, JSON.stringify(seed())); } catch (e) { /* ignore */ } S = seed(); LIVE_SIM.reset(); location.hash = '#/live/channels'; render(); toast('Mockup zurückgesetzt'); },
};
function saveFrame() { try { localStorage.setItem(KEY + '-frame', JSON.stringify(frame)); } catch (e) { /* ignore */ } fit(); }
window.addEventListener('resize', fit);

/* the jump menu comes from the register (js/reg-*.js); PAGES stays for extra entries of page files */
const PAGES = [];
/** pages the concept does not change (libraries, settings …): reachable, explained */
function unchangedPage(path) {
  return { nav: path === '/' ? 'start' : '', html: `${bar('Unverändert', { back: false })}<div class="empty">${icon('info', 40)}<b>Diese Seite ändert sich nicht</b><p>Konzept 28 «Live-TV» lässt sie, wie sie im Mockup von Konzept 27 ist.</p><p class="hint">${esc(path)}</p><p style="margin-top:16px"><a class="btn ghost" href="#/live" data-fid="un-live">Zu Live-TV</a></p></div>` };
}
