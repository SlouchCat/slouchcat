/* Konzept 28 «Live-TV», TV mockup core: helpers, state, router, focus engine, overlays, on-screen keyboard,
   TV frame, remote with the new keys (channel up/down, guide, info, record, digits 0–9). Copied from the
   mockup of Konzept 26 and adapted: no setup, no library data (Live-TV data comes from ../shared/), images
   from ../shared/img/lib/ (only local). Plain scripts, all functions global. Owned by the core. */
'use strict';

/* ---------- helpers ---------- */
const $ = (sel, root) => (root || document).querySelector(sel);
const $$ = (sel, root) => Array.from((root || document).querySelectorAll(sel));
const esc = s => String(s ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
const IMG = '../shared/img/lib/';
const W = 1920; const H = 1080;

const ICONS = {
  home: '<path d="M3 10.5 12 3l9 7.5V20a1 1 0 0 1-1 1h-5v-6h-6v6H4a1 1 0 0 1-1-1z"/>',
  search: '<circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/>',
  wish: '<path d="M12 3l2.6 5.4 5.9.8-4.3 4.1 1 5.9L12 16.4 6.8 19.2l1-5.9L3.5 9.2l5.9-.8z"/>',
  play: '<path d="M7 4.5v15l12-7.5z" fill="currentColor"/>',
  pause: '<path d="M7 5v14M17 5v14" stroke-width="3"/>',
  playpause: '<path d="M3 5v14l9-7z" fill="currentColor"/><path d="M16 5v14M21 5v14"/>',
  back: '<path d="M19 12H5m0 0 6-6m-6 6 6 6"/>',
  close: '<path d="M6 6l12 12M18 6 6 18"/>',
  check: '<path d="m5 12.5 4.5 4.5L19 7.5"/>',
  plus: '<path d="M12 5v14M5 12h14"/>',
  bookmark: '<path d="M6 4h12v17l-6-4.5L6 21z"/>',
  bookmarkOn: '<path d="M6 4h12v17l-6-4.5L6 21z" fill="currentColor"/>',
  right: '<path d="m9 5 7 7-7 7"/>',
  left: '<path d="m15 5-7 7 7 7"/>',
  down: '<path d="m5 9 7 7 7-7"/>',
  up: '<path d="m5 15 7-7 7 7"/>',
  settings: '<circle cx="12" cy="12" r="3"/><path d="M12 2v3m0 14v3M2 12h3m14 0h3M4.9 4.9l2.1 2.1m10 10 2.1 2.1M19.1 4.9 17 7M7 17l-2.1 2.1"/>',
  user: '<circle cx="12" cy="8" r="4"/><path d="M4 21c1.2-4 4.3-6 8-6s6.8 2 8 6"/>',
  users: '<circle cx="9" cy="8" r="3.5"/><path d="M2.5 20c.9-3.3 3.4-5 6.5-5s5.6 1.7 6.5 5M16 4.8a3.5 3.5 0 0 1 0 6.4M18 15.3c1.8.7 3 2.3 3.5 4.7"/>',
  tv: '<rect x="2.5" y="4" width="19" height="13" rx="2"/><path d="M8 21h8"/>',
  qr: '<path d="M4 4h6v6H4zM14 4h6v6h-6zM4 14h6v6H4zM14 14h2v2h-2zM18 14h2M14 18h2m2 0h2v2"/>',
  radio: '<rect x="3" y="9" width="18" height="11" rx="2"/><path d="m7 9 10-5"/><circle cx="8.5" cy="14.5" r="2.5"/><path d="M15 13h3m-3 3h3"/>',
  headphones: '<path d="M4 15v-3a8 8 0 0 1 16 0v3"/><rect x="3" y="14" width="4" height="6" rx="1.5"/><rect x="17" y="14" width="4" height="6" rx="1.5"/>',
  mic: '<rect x="9" y="3" width="6" height="11" rx="3"/><path d="M5 11a7 7 0 0 0 14 0M12 18v3"/>',
  bell: '<path d="M6 16V11a6 6 0 0 1 12 0v5l2 2H4zM10 20a2 2 0 0 0 4 0"/>',
  filter: '<path d="M4 6h16M7 12h10M10 18h4"/>',
  sort: '<path d="M7 5v14m0 0-3-3m3 3 3-3M17 19V5m0 0-3 3m3-3 3 3"/>',
  star: '<path d="M12 3l2.6 5.4 5.9.8-4.3 4.1 1 5.9L12 16.4 6.8 19.2l1-5.9L3.5 9.2l5.9-.8z" fill="currentColor"/>',
  info: '<circle cx="12" cy="12" r="9"/><path d="M12 11v6m0-9.5v.5"/>',
  lock: '<rect x="5" y="10" width="14" height="10" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3"/>',
  unlock: '<rect x="5" y="10" width="14" height="10" rx="2"/><path d="M8 10V7a4 4 0 0 1 7.5-1.9"/>',
  trash: '<path d="M4 7h16M9 7V4h6v3m-8 0 1 13h8l1-13"/>',
  wifi: '<path d="M2.5 9a14 14 0 0 1 19 0M6 12.5a9 9 0 0 1 12 0M9.5 16a4 4 0 0 1 5 0"/><circle cx="12" cy="19.5" r="1" fill="currentColor"/>',
  subs: '<rect x="3" y="5" width="18" height="14" rx="2"/><path d="M7 14h4m3 0h3M7 10.5h2m3 0h5"/>',
  audio: '<path d="M4 9v6h4l5 4V5L8 9zM16.5 8.5a5 5 0 0 1 0 7M19 6a9 9 0 0 1 0 12"/>',
  fwd: '<path d="M12 5a7 7 0 1 1-6.6 4.7M12 5l-3-3m3 3-3 3"/>',
  rew: '<path d="M12 5a7 7 0 1 0 6.6 4.7M12 5l3-3m-3 3 3 3"/>',
  next: '<path d="M6 5v14l10-7z" fill="currentColor"/><path d="M19 5v14"/>',
  prev: '<path d="M18 5v14L8 12z" fill="currentColor"/><path d="M5 5v14"/>',
  list: '<path d="M8 6h12M8 12h12M8 18h12M4 6h.01M4 12h.01M4 18h.01"/>',
  grid: '<rect x="4" y="4" width="7" height="7" rx="1"/><rect x="13" y="4" width="7" height="7" rx="1"/><rect x="4" y="13" width="7" height="7" rx="1"/><rect x="13" y="13" width="7" height="7" rx="1"/>',
  film: '<rect x="3" y="4" width="18" height="16" rx="2"/><path d="M8 4v16M16 4v16M3 9h5m8 0h5M3 15h5m8 0h5"/>',
  book: '<path d="M5 4h11a3 3 0 0 1 3 3v13H8a3 3 0 0 1-3-3zM5 17a3 3 0 0 1 3-3h11"/>',
  eye: '<path d="M2 12s3.6-7 10-7 10 7 10 7-3.6 7-10 7S2 12 2 12z"/><circle cx="12" cy="12" r="3"/>',
  eyeOff: '<path d="M3 3l18 18M10.6 5.1A10.6 10.6 0 0 1 12 5c6.4 0 10 7 10 7a17 17 0 0 1-3.2 4M6.6 6.6C3.8 8.3 2 12 2 12s3.6 7 10 7c1.9 0 3.5-.6 4.9-1.4M9.9 9.9a3 3 0 0 0 4.2 4.2"/>',
  clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3.5 2"/>',
  alert: '<path d="M12 3 2 20h20zM12 10v4m0 3v.5"/>',
  logout: '<path d="M10 4H5v16h5M14 8l4 4-4 4m4-4H9"/>',
  phone: '<rect x="7" y="2.5" width="10" height="19" rx="2.5"/><path d="M11 18.5h2"/>',
  server: '<rect x="3" y="4" width="18" height="7" rx="2"/><rect x="3" y="13" width="18" height="7" rx="2"/><path d="M7 7.5h.01M7 16.5h.01"/>',
  link: '<path d="M10 14a4 4 0 0 0 6 0l3-3a4.2 4.2 0 0 0-6-6l-1 1M14 10a4 4 0 0 0-6 0l-3 3a4.2 4.2 0 0 0 6 6l1-1"/>',
  refresh: '<path d="M20 5v5h-5M4 19v-5h5M19 10a7.5 7.5 0 0 0-13-2.5M5 14a7.5 7.5 0 0 0 13 2.5"/>',
  heart: '<path d="M12 20s-7.5-4.6-7.5-10A4.3 4.3 0 0 1 12 7.4 4.3 4.3 0 0 1 19.5 10c0 5.4-7.5 10-7.5 10z"/>',
  dots: '<circle cx="12" cy="5" r="1.6" fill="currentColor"/><circle cx="12" cy="12" r="1.6" fill="currentColor"/><circle cx="12" cy="19" r="1.6" fill="currentColor"/>',
  hd: '<rect x="3" y="6" width="18" height="12" rx="2"/><path d="M7 9.5v5m0-2.5h3m0-2.5v5M14 9.5v5h1.5a2.5 2.5 0 0 0 0-5z"/>',
  speed: '<path d="M5 17a8 8 0 1 1 14 0M12 13l4-4"/>',
  globe: '<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3c3 3.3 3 14.7 0 18M12 3c-3 3.3-3 14.7 0 18"/>',
  shield: '<path d="M12 3 5 6v6c0 4.5 3 7.7 7 9 4-1.3 7-4.5 7-9V6z"/>',
  keyboard: '<rect x="2.5" y="6" width="19" height="12" rx="2"/><path d="M6 10h.01M9.5 10h.01M13 10h.01M16.5 10h.01M6 14h.01M18 14h.01M9 14h6"/>',
  edit: '<path d="M4 20h4L19 9l-4-4L4 16z"/><path d="m13.5 6.5 4 4"/>',
  backspace: '<path d="M21 5H9l-6 7 6 7h12z"/><path d="m12 9 6 6m0-6-6 6"/>',
  shift: '<path d="M12 4 4 12h4v7h8v-7h4z"/>',
  power: '<path d="M12 3v9M6.3 6.3a8 8 0 1 0 11.4 0"/>',
  recordHollow: '<circle cx="12" cy="12" r="6.5"/>',
  stop: '<rect x="6" y="6" width="12" height="12" rx="2" fill="currentColor" stroke="none"/>',
  expand: '<path d="M4 9V4h5M20 9V4h-5M4 15v5h5M20 15v5h-5"/>',
  loadspin: '<path d="M12 3a9 9 0 1 0 9 9"/>',
  recordSeries: '<circle cx="8.5" cy="12" r="5" fill="currentColor" stroke="none"/><circle cx="16.5" cy="12" r="4.5"/>',
  arrowUp: '<path d="M12 19V5m0 0-6 6m6-6 6 6"/>',
  arrowDown: '<path d="M12 5v14m0 0-6-6m6 6 6-6"/>',
  live: '<path d="M12 13.5v7M9 20.5h6"/><circle cx="12" cy="11" r="1.8"/><path d="M8.2 7.5a5.2 5.2 0 0 0 0 7M15.8 7.5a5.2 5.2 0 0 1 0 7M5.5 5a9 9 0 0 0 0 12M18.5 5a9 9 0 0 1 0 12"/>',
  record: '<circle cx="12" cy="12" r="6.5" fill="currentColor" stroke="none"/>',
  reminder: '<path d="M6 16V11a6 6 0 0 1 12 0v5l2 2H4zM10 20a2 2 0 0 0 4 0"/><path d="M12 8.5V11l1.8 1.2"/>',
  calendar: '<rect x="4" y="5.5" width="16" height="14" rx="2"/><path d="M4 10h16M8.5 3.5v4M15.5 3.5v4"/>',
  heartOn: '<path d="M12 20s-7.5-4.6-7.5-10A4.3 4.3 0 0 1 12 7.4 4.3 4.3 0 0 1 19.5 10c0 5.4-7.5 10-7.5 10z" fill="currentColor"/>',
  filter: '<path d="M4 6h16M7 12h10M10 18h4"/>',
  tv: '<rect x="2.5" y="4" width="19" height="13" rx="2"/><path d="M8 21h8"/>',
  radio: '<rect x="3" y="9" width="18" height="11" rx="2"/><path d="m7 9 10-5"/><circle cx="8.5" cy="14.5" r="2.5"/><path d="M15 13h3m-3 3h3"/>',
  headphones: '<path d="M4 15v-3a8 8 0 0 1 16 0v3"/><rect x="3" y="14" width="4" height="6" rx="1.5"/><rect x="17" y="14" width="4" height="6" rx="1.5"/>',
  mic: '<rect x="9" y="3" width="6" height="11" rx="3"/><path d="M5 11a7 7 0 0 0 14 0M12 18v3"/>',
};
const icon = (name, size) => `<svg width="${size || 32}" height="${size || 32}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${ICONS[name] || ICONS.info}</svg>`;

/* ---------- pictures ---------- */
const dur = min => (min >= 60 ? `${Math.floor(min / 60)} Std. ${min % 60} Min.` : `${min} Min.`);
/** picture from ../shared/img/lib/ (only local); a missing file becomes a calm field with the alt text */
const pic = (file, alt, cls) => `<img src="${IMG}${file}" alt="${esc(alt || '')}" loading="lazy"${cls ? ` class="${cls}"` : ''} onerror="this.outerHTML='&lt;span class=&quot;ph&quot;&gt;${esc(esc(alt || ''))}&lt;/span&gt;'">`;
const hostOf = url => String(url || '').replace(/^https?:\/\//, '').replace(/\/.*$/, '');
const slug = s => String(s).toLowerCase().replace(/ä/g, 'ae').replace(/ö/g, 'oe').replace(/ü/g, 'ue').replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
/* media of the rail: the libraries stay unchanged (Konzept 26); Live-TV is new, before the web radio */
const LIBS = [
  { key: 'films', name: 'Filme', icon: 'film', count: 1846 }, { key: 'series', name: 'Serien', icon: 'tv', count: 505 },
  { key: 'docs', name: 'Dokus', icon: 'globe', count: 63 }, { key: 'books', name: 'Hörbücher', icon: 'headphones', count: 89 },
  { key: 'podcasts', name: 'Podcasts', icon: 'mic', count: 100 },
];
const avatar = (p, size) => `<span class="avatar" style="background:var(--${p.color || 'av1'});width:${size || 56}px;height:${size || 56}px;font-size:${Math.round((size || 56) * 0.44)}px">${esc(p.name[0])}</span>`;
const wordmark = () => '<span class="wordmark">Slouch<i>cat</i></span>';

/* ---------- state ----------
   Live-TV state lives in LIVE_SIM (../shared/live-sim.js). S keeps only TV preferences of the mockup. */
const KEY = 'sc28-tv-mock-v1';
function seed() { return { prefs: { quality: 'Automatisch', audio: 'Deutsch', subs: 'Aus' } }; }
let S;
try { S = JSON.parse(localStorage.getItem(KEY)) || seed(); } catch (e) { S = seed(); }
const save = () => { try { localStorage.setItem(KEY, JSON.stringify(S)); } catch (e) { /* private mode */ } };
const ui = { open: {}, draft: {}, q: '', tab: {} };
const AVC = { anna: 'av1', luca: 'av2', mia: 'av3' };
/** the active profile in the shape of the TV mockup: { id, name, color, main, age } */
const me = () => { const p = LIVE_SIM.profile(); return { id: p.id, name: p.name, color: AVC[p.id] || 'av1', main: p.primary, age: p.age }; };
const RESETS = []; /* page files push functions that clear their own volatile state on «Neu starten» */

/* ---------- router and history ---------- */
const ROUTES = [];
function route(pattern, fn) {
  const names = []; const re = new RegExp('^' + pattern.replace(/:[a-z]+/g, m => { names.push(m.slice(1)); return '([^/]+)'; }) + '$');
  ROUTES.push({ re, names, fn, pattern });
}
const curHash = () => location.hash || '#/';
const curPath = () => (location.hash.slice(1) || '/').split('?')[0];
function current() {
  const [path, qs] = (location.hash.slice(1) || '/').split('?');
  const query = Object.fromEntries(new URLSearchParams(qs || ''));
  for (const r of ROUTES) { const m = path.match(r.re); if (m) { const p = {}; r.names.forEach((n, k) => { p[n] = decodeURIComponent(m[k + 1]); }); return { r, p, query, path }; } }
  return { r: null, p: {}, query, path };
}
const NAVSTK = [curHash()]; let replacing = false; let navDir = 'push';
/** go('#/pfad') navigates; go(hash, { replace: true }) replaces the current history entry. */
function go(hash, o) {
  if (curHash() === hash) { render(); return; }
  if (o && o.replace) { replacing = true; location.replace(hash); } else location.hash = hash;
}
function navBack() {
  if (NAVSTK.length > 1) history.back();
  else if (curPath() !== '/') go('#/', { replace: true });
}
window.addEventListener('hashchange', () => {
  const h = curHash();
  if (NAVSTK.length > 1 && h === NAVSTK[NAVSTK.length - 2]) { NAVSTK.pop(); navDir = 'back'; }
  else if (replacing) { NAVSTK[NAVSTK.length - 1] = h; replacing = false; }
  else NAVSTK.push(h);
  render();
});

const NAV = [
  ['start', 'Start', 'home', '#/'], ['search', 'Suche', 'search', '#/search'], ['libs', 'Bibliotheken', 'grid', null],
  ['wishes', 'Wünsche', 'wish', '#/wishes'], ['settings', 'Einstellungen', 'settings', '#/settings'],
];
/* The rail carries the media selection itself (Marco, 19 November 2026): every library and the radio
   are entries of the left rail, no extra panel in between. */
function drawRail(nav) {
  const p = me(); const path = (location.hash.slice(1) || '/').split('?')[0];
  const live = LIVE_SIM.rights().liveTv ? [['live', 'Live-TV', 'live', '#/live']] : [];
  const media = [...LIBS.map(l => ['lib-' + l.key, l.name, l.icon, '#/library/' + l.key]), ...live, ['radio', 'Webradio', 'radio', '#/radio']];
  const mediaCur = media.find(m => path === m[3].slice(1) || (m[0] === 'live' && path.startsWith('/live'))) || null;
  const main = ['start', 'search', 'wishes', 'settings'];
  const fallback = !mediaCur && !main.includes(nav) && nav !== 'profiles';
  const item = ([k, label, ic, h], cur, def, cls) => `<a class="r-it${cls || ''}" href="${h}" data-f="r-${k}"${cur ? ' aria-current="page"' : ''}${def ? ' data-default' : ''}><span class="r-ic">${icon(ic, cls ? 32 : 38)}</span><span class="r-lb">${label}</span></a>`;
  const plain = keys => NAV.filter(n => keys.includes(n[0])).map(n => { const cur = !mediaCur && nav === n[0]; return item(n, cur, cur || (fallback && n[0] === 'start')); }).join('');
  const med = media.map(m => item(m, mediaCur === m, mediaCur === m, ' r-med')).join('');
  $('#rail').innerHTML = `<div class="r-brand"><span class="r-logo">S</span><span class="r-lb">${wordmark()}</span></div><div class="r-items">${plain(['start', 'search'])}<div class="r-sec"><span class="r-lb">Medien</span></div>${med}<div class="r-sec"></div>${plain(['wishes', 'settings'])}</div><a class="r-it r-prof" href="#/profiles" data-f="r-prof"${nav === 'profiles' ? ' aria-current="page"' : ''}>${avatar(p, 54)}<span class="r-lb"><b>${esc(p.name)}</b><small>Profil wechseln</small></span></a>`;
}

/* page memory: MEM[path] = { f, top, left: {row: scrollLeft}, rows: {row: data-f} } */
const MEM = {};
let VIEW = {}; let lastPath = null; let lastContent = null;
function remember(path) {
  const m = MEM[path] || (MEM[path] = { rows: {} });
  const v = $('#view');
  if (F.el && v.contains(F.el)) m.f = F.el.dataset.f;
  m.top = v.scrollTop; m.left = {};
  $$('[data-row]', v).forEach(r => { m.left[r.dataset.row] = r.scrollLeft; });
}
function missing(path) {
  return {
    nav: '', html: `<div class="page soon"><span class="soon-ic">${icon('clock', 88)}</span><h1>Unverändert</h1>
      <p>Diese Seite ändert sich durch Konzept 28 «Live-TV» nicht (Mockup Konzept 26).</p><p class="soon-path">${esc(path)}</p>
      <button type="button" class="tb primary" data-act="back" data-f="soon-back" data-default>${icon('back', 32)}<span>Zurück</span></button></div>`,
  };
}
function render() {
  const c = current(); const path = c.path;
  closeAll();
  const same = lastPath === path;
  if (lastPath) remember(lastPath);
  if (!same && navDir !== 'back') MEM[path] = { rows: {} };
  const m = MEM[path] || (MEM[path] = { rows: {} });
  const v = liveGate(path) || (c.r ? c.r.fn(c.p, c.query) : missing(path)) || {};
  VIEW = v;
  $('#app').classList.toggle('full', !!v.full);
  $('#app').classList.remove('rail-open');
  const view = $('#view');
  view.className = v.cls || '';
  view.innerHTML = v.html || '';
  drawRail(v.nav);
  if (!same) art(null);
  lastPath = path; F.el = null;
  const restoring = same || navDir === 'back';
  navDir = 'push';
  if (restoring) {
    view.scrollTop = m.top || 0;
    $$('[data-row]', view).forEach(r => { const l = m.left && m.left[r.dataset.row]; if (l) r.scrollLeft = l; });
  } else view.scrollTop = 0;
  if (v.after) v.after(view);
  const want = restoring && m.f ? byF(m.f, view) : null;
  if (want && usable(want)) focus(want, { noReveal: true });
  else focus(defaultIn(view) || defaultIn($('#rail')), { instant: true });
  LIVE_MOCK.syncJump($('#jump'));
  LIVE_UI.refresh($('#screen'));
}
const commit = () => { save(); render(); };

/* ---------- focus engine ---------- */
const F = { el: null };
let SCALE = 1; let lastInput = Date.now();
const byF = (f, root) => (f == null ? null : (root || document).querySelector(`[data-f="${CSS.escape(f)}"]`));
const usable = el => !!el && !el.disabled && el.getAttribute('aria-disabled') !== 'true' && el.getClientRects().length > 0;
const scopeEl = () => (OV.length ? OV[OV.length - 1].panel : $('#app'));
const focusables = scope => $$('[data-f]', scope).filter(usable);
function defaultIn(scope) {
  if (!scope) return null;
  const d = $$('[data-default]', scope).find(usable);
  return d || focusables(scope)[0] || null;
}
/** Focus by data-f (inside the open menu, else the page). */
function focusTo(f, o) { const el = byF(f, scopeEl()); if (el) focus(el, o); return !!el; }
function focus(el, o) {
  o = o || {}; if (!el) return;
  if (F.el && F.el !== el) F.el.classList.remove('is-f');
  F.el = el; el.classList.add('is-f');
  if (!el.matches('a[href],button,input,select,textarea,[tabindex]')) el.tabIndex = -1;
  if (document.activeElement !== el) el.focus({ preventScroll: true });
  if (!o.noReveal) reveal(el, o.instant);
  const inRail = !!el.closest('#rail');
  $('#app').classList.toggle('rail-open', inRail && !OV.length);
  const view = $('#view');
  if (!inRail && view.contains(el)) {
    lastContent = el;
    const m = MEM[lastPath]; if (m) { m.f = el.dataset.f; const r = el.closest('[data-row]'); if (r && el.dataset.f) m.rows[r.dataset.row] = el.dataset.f; }
  }
  if (el.dataset.tt != null) {
    const sec = el.closest('.rsec, [data-info-sec]'); const ri = sec && $('.rinfo', sec);
    if (ri) ri.innerHTML = `<b>${esc(el.dataset.tt)}</b>${el.dataset.tm ? `<span>${esc(el.dataset.tm)}</span>` : ''}`;
  }
  if (VIEW.onFocus && view.contains(el)) VIEW.onFocus(el);
}
/** Scroll the focused element into view: inside its row (card aligned to the row start) and in the page. */
function reveal(el, instant) {
  const behavior = instant ? 'auto' : 'smooth';
  const r = el.closest('[data-row]');
  if (r && r.scrollWidth > r.clientWidth + 2) {
    const pad = parseFloat(getComputedStyle(r).paddingLeft) || 0;
    const left = Math.max(0, Math.min(el.offsetLeft - pad, r.scrollWidth - r.clientWidth));
    r.scrollTo({ left, behavior });
  }
  const sc = el.closest('[data-scroll]'); if (!sc) return;
  const sr = sc.getBoundingClientRect(); const er = el.getBoundingClientRect(); const k = SCALE || 1;
  const top = (er.top - sr.top) / k + sc.scrollTop; const h = er.height / k; const vh = sc.clientHeight;
  let target = null;
  if (el.closest('[data-top]')) target = 0;
  else {
    const sec = el.closest('.rsec');
    if (sec && sc.contains(sec)) target = (sec.getBoundingClientRect().top - sr.top) / k + sc.scrollTop - 54;
    else { const mg = 110; if (top < sc.scrollTop + mg) target = top - mg; else if (top + h > sc.scrollTop + vh - mg) target = top + h - vh + mg; }
  }
  if (target != null) sc.scrollTo({ top: Math.max(0, target), behavior });
}
/** Geometric search: nearest focusable in the direction, strongly preferring the same row or column. */
function pick(from, dir, list) {
  const a = from.getBoundingClientRect(); const ax = a.left + a.width / 2; const ay = a.top + a.height / 2;
  let best = null; let bs = Infinity;
  for (const el of list) {
    if (el === from || el.contains(from) || from.contains(el)) continue;
    const b = el.getBoundingClientRect(); const bx = b.left + b.width / 2; const by = b.top + b.height / 2;
    let main; let cross; let ov;
    if (dir === 'left' || dir === 'right') {
      if (dir === 'right' ? bx <= ax + 1 : bx >= ax - 1) continue;
      main = dir === 'right' ? b.left - a.right : a.left - b.right;
      ov = Math.min(a.bottom, b.bottom) - Math.max(a.top, b.top); cross = Math.abs(by - ay);
    } else {
      if (dir === 'down' ? by <= ay + 1 : by >= ay - 1) continue;
      main = dir === 'down' ? b.top - a.bottom : a.top - b.bottom;
      ov = Math.min(a.right, b.right) - Math.max(a.left, b.left); cross = Math.abs(bx - ax);
    }
    main = Math.max(0, main);
    const s = ov > 0 ? main + cross * 0.3 : 100000 + main + cross * 2;
    if (s < bs) { bs = s; best = el; }
  }
  return best;
}
function move(dir) {
  const scope = scopeEl(); const from = F.el && scope.contains(F.el) && usable(F.el) ? F.el : null;
  if (!from) { focus(defaultIn(scope === $('#app') ? $('#view') : scope) || defaultIn(scope)); return; }
  let t = pick(from, dir, focusables(scope));
  if (!t) { bump(from, dir); return; }
  const tr = t.closest('[data-row]'); const fr = from.closest('[data-row]');
  if (tr && tr !== fr && scope.contains(tr)) {
    if (tr.hasAttribute('data-enter')) t = $$('[data-default]', tr).find(usable) || t;
    else { const m = MEM[lastPath]; const e = m && m.rows[tr.dataset.row] && byF(m.rows[tr.dataset.row], tr); if (usable(e)) t = e; }
  }
  if (fr && fr.id === 'rail' && !t.closest('#rail') && lastContent && document.contains(lastContent) && usable(lastContent)) t = lastContent;
  focus(t);
}
function bump(el, dir) { el.classList.remove('bump-l', 'bump-r', 'bump-u', 'bump-d'); void el.offsetWidth; el.classList.add('bump-' + dir[0]); setTimeout(() => el.classList.remove('bump-' + dir[0]), 260); }
function activate(el) {
  if (!el || el.getAttribute('aria-disabled') === 'true' || el.disabled) return;
  const a = el.dataset.act;
  if (a && ACT[a]) { ACT[a](el.dataset, el); return; }
  const h = el.getAttribute('href'); if (h && h !== '#') go(h);
}

/* ---------- keys: remote and physical keyboard ---------- */
/* remote keys; new for Live-TV (14-oberflaeche.md 4.13): ch_up, ch_down (Bild↑/Bild↓), guide (G), info (I),
   rec (R), last (L, zuletzt gesehener Sender), d0 … d9 (Ziffern) */
const KEYMAP = { ArrowUp: 'up', ArrowDown: 'down', ArrowLeft: 'left', ArrowRight: 'right', Enter: 'ok', Backspace: 'back', Escape: 'back', ' ': 'play', MediaPlayPause: 'play', BrowserHome: 'home', PageUp: 'ch_up', PageDown: 'ch_down', g: 'guide', G: 'guide', i: 'info', I: 'info', r: 'rec', R: 'rec', l: 'last', L: 'last', 0: 'd0', 1: 'd1', 2: 'd2', 3: 'd3', 4: 'd4', 5: 'd5', 6: 'd6', 7: 'd7', 8: 'd8', 9: 'd9' };
let exitArm = 0; let OFF = false;
/** One remote key: up, down, left, right, ok, back, home, play. ev is the keyboard event or null. */
function press(name, ev) {
  lastInput = Date.now();
  if (name) flashRemote(name);
  if (OFF) { if (name === 'ok' || name === 'home') reopen(); return !!name; }
  const top = OV[OV.length - 1];
  if (top) { if (top.o.onKey && top.o.onKey(name, ev)) return true; }
  else if (VIEW.key && VIEW.key(name, ev)) return true;
  if (!name) return false;
  if (name === 'up' || name === 'down' || name === 'left' || name === 'right') move(name);
  else if (name === 'ok') activate(F.el);
  else if (name === 'back') { if (top) closeMenu(); else doBack(); }
  else if (name === 'play') playKey();
  else if (name === 'guide') { if (top) closeAll(); go('#/live/guide'); }
  else if (/^(ch_up|ch_down|info|rec|last|d\d)$/.test(name)) liveKeyDefault(name);
  else if (name === 'home') { if (top) closeAll(); if (curPath() === '/') { focus(defaultIn($('#view'))); } else go('#/'); }
  return true;
}
function doBack() {
  if (VIEW.back && VIEW.back()) return;
  const p = curPath();
  if (p === '/' || VIEW.root) {
    const top = defaultIn($('#view'));
    if (p === '/' && top && F.el !== top) { focus(top); return; }
    if (Date.now() - exitArm < 3000) { exitArm = 0; exitApp(); return; }
    exitArm = Date.now(); toast('Zum Verlassen nochmals Zurück drücken'); return;
  }
  navBack();
}
/** Play/Pause on an element with data-ch switches the channel on; elsewhere nothing to play */
function playKey() {
  const el = F.el; const ch = el && el.dataset.ch;
  if (ch) { liveTune(ch); return; }
  toast('Hier gibt es nichts abzuspielen');
}
function exitApp() { OFF = true; closeAll(); const o = $('#off'); o.hidden = false; o.innerHTML = `<div class="off-in">${wordmark()}<p>Slouchcat ist beendet.</p><p class="off-s">Mit OK oder Start öffnest du die App wieder.</p></div>`; }
function reopen() { OFF = false; $('#off').hidden = true; $('#off').innerHTML = ''; NAVSTK.length = 0; NAVSTK.push(curHash()); render(); }
document.addEventListener('keydown', e => {
  if (e.target.tagName === 'SELECT' || e.target.tagName === 'INPUT') return;
  if (e.target.closest && e.target.closest('#tools')) return;
  if (e.ctrlKey || e.metaKey || e.altKey) return;
  const name = KEYMAP[e.key] || null;
  if (press(name, e)) e.preventDefault();
});
document.addEventListener('focusin', e => {
  const el = e.target.closest && e.target.closest('#screen [data-f]');
  if (el && el !== F.el && scopeEl().contains(el)) focus(el, { instant: true });
});
function flashRemote(name) { const b = $(`#remote [data-rc="${name}"]`); if (!b) return; b.classList.add('down'); setTimeout(() => b.classList.remove('down'), 140); }

/* mouse convenience inside the screen: a click focuses and activates; never required */
document.addEventListener('click', e => {
  const tool = e.target.closest('[data-tool]');
  if (tool) { TOOL[tool.dataset.tool](tool.dataset.v, tool); tool.blur(); if (F.el) F.el.focus({ preventScroll: true }); return; }
  const rc = e.target.closest('[data-rc]'); if (rc) { press(rc.dataset.rc, null); return; }
  if (!e.target.closest('#screen')) return;
  const a = e.target.closest('a[href]'); if (a) e.preventDefault();
  if (OFF) { reopen(); return; }
  if (OV.length && !OV[OV.length - 1].panel.contains(e.target)) { if (e.target.closest('.ov-scrim')) closeMenu(); return; }
  const el = e.target.closest('[data-f], [data-act]') || a; if (!el) return;
  lastInput = Date.now();
  if (el.dataset.f != null && usable(el)) focus(el);
  activate(el);
});

/* ---------- actions ---------- */
const ACT = {};
ACT.back = () => press('back', null);
ACT.go = d => go(d.to);
ACT.close = () => closeMenu();
ACT.toast = d => toast(d.msg);
ACT.toggle = d => { const o = d.scope === 'S' ? S : ui; const p = d.path.split('.'); let t = o; for (let k = 0; k < p.length - 1; k++) t = t[p[k]] = t[p[k]] || {}; t[p[p.length - 1]] = !t[p[p.length - 1]]; commit(); };
ACT.set = d => { const p = d.path.split('.'); let t = p[0] === 'S' ? S : ui; for (let k = 1; k < p.length - 1; k++) t = t[p[k]] = t[p[k]] || {}; t[p[p.length - 1]] = d.num != null ? Number(d.value) : d.value; closeAll(); commit(); };
ACT.watch = d => { const i = Number(d.i); const k = S.watch.indexOf(i); if (k < 0) { S.watch.unshift(i); toast('Zur Merkliste hinzugefügt'); } else { S.watch.splice(k, 1); toast('Von der Merkliste entfernt'); } commit(); };
ACT.seen = d => { const i = Number(d.i); const k = S.seen.indexOf(i); if (k < 0) S.seen.push(i); else S.seen.splice(k, 1); toast(k < 0 ? 'Als gesehen markiert' : 'Markierung entfernt'); commit(); };
ACT.libs = () => menu(`<h2>Bibliotheken</h2><div class="mlist">${LIBS.map((l, n) => `<a class="mi" href="#/library/${l.key}" data-f="lib-${l.key}"${n === 0 ? ' data-default' : ''}><span class="mi-ic">${icon(l.icon, 40)}</span><span class="mi-tx"><b>${esc(l.name)}</b><small>${LIVE_UI.fmt.num(l.count)} Titel</small></span>${icon('right', 32)}</a>`).join('')}</div>`);

/* ---------- menus and dialogs (focus is trapped in the top one) ---------- */
const OV = [];
/** menu(html, { dialog, cls, onKey(name, ev), onClose }) opens a side panel (default) or a centred dialog. */
function menu(html, o) {
  o = o || {};
  const box = document.createElement('div');
  box.className = `ov ${o.dialog ? 'dlg' : 'side'}${o.cls ? ' ' + o.cls : ''}`;
  box.innerHTML = `<div class="ov-scrim"></div><div class="ov-panel" role="dialog" aria-modal="true" data-scroll>${html}</div>`;
  $('#overlays').appendChild(box);
  const entry = { box, panel: $('.ov-panel', box), prev: F.el, prevF: F.el && F.el.dataset.f, o };
  OV.push(entry);
  $('#app').classList.remove('rail-open');
  focus(defaultIn(entry.panel), { instant: true });
  return entry.panel;
}
/** Replace the content of the top menu; focus stays on the element with the same data-f. */
function redrawMenu(html) {
  const top = OV[OV.length - 1]; if (!top) return;
  const f = F.el && top.panel.contains(F.el) ? F.el.dataset.f : null;
  top.panel.innerHTML = html;
  const el = f && byF(f, top.panel);
  focus(usable(el) ? el : defaultIn(top.panel), { instant: true });
}
function closeMenu() {
  const t = OV.pop(); if (!t) return;
  t.box.remove();
  if (t.o.onClose) t.o.onClose();
  const scope = scopeEl();
  const back = t.prev && document.contains(t.prev) ? t.prev : byF(t.prevF, scope);
  focus(usable(back) && scope.contains(back) ? back : defaultIn(scope === $('#app') ? $('#view') : scope), { instant: true });
}
function closeAll() { while (OV.length) { const t = OV.pop(); t.box.remove(); } }

/* ---------- toast ---------- */
let toastT;
function toast(msg) { const n = $('#toast'); n.textContent = msg; n.classList.remove('on'); void n.offsetWidth; n.classList.add('on'); clearTimeout(toastT); toastT = setTimeout(() => n.classList.remove('on'), 2600); }

/* ---------- art layer: backdrop behind the page, cross-fading ---------- */
let artCur = null;
/** art('b-3.webp', 'hero' | 'dim') shows a backdrop behind the page; art(null) fades it out. */
function art(file, mode) {
  const box = $('#art'); box.dataset.mode = mode || 'hero';
  if (file === artCur) return; artCur = file;
  const olds = $$('img', box); olds.forEach(o => o.classList.remove('on')); setTimeout(() => olds.forEach(o => o.remove()), 700);
  if (!file) return;
  const im = new Image(); im.alt = ''; im.onload = () => im.classList.add('on'); im.onerror = () => im.remove(); im.src = IMG + file; box.appendChild(im);
}

/* ---------- on-screen keyboard ---------- */
const KB_ROWS = {
  abc: ['1234567890', 'qwertzuiop-', 'asdfghjkl._', 'yxcvbnm@:/&'],
  sym: ['1234567890', '!?#$%*+=()~', '[]{}<>\'";,|', '\\^`€§°äöüéè'],
};
const KB_QUICK = { url: ['https://', '.ch'], email: ['.ch', '.com'], text: ['-', '.'], password: ['-', '.'] };
let KB = null;
/** keyboard({ value, label, onDone(value), onCancel, type: 'text' | 'url' | 'email' | 'password', placeholder }) */
function keyboard(o) {
  KB = { v: String(o.value || ''), label: o.label || '', type: o.type || 'text', ph: o.placeholder || '', shift: false, sym: false, show: false, typed: false, o };
  menu(kbHtml(), {
    dialog: true, cls: 'kbd',
    onClose: () => { const k = KB; KB = null; if (k && !k.done && k.o.onCancel) k.o.onCancel(); },
    onKey: (name, e) => {
      if (!KB) return false;
      if (name === 'up' || name === 'down' || name === 'left' || name === 'right') KB.typed = false;
      if (!e) return false;
      if (e.key === 'Backspace') { kbPress('del'); KB.typed = true; return true; }
      if (e.key === 'Enter' && KB.typed) { kbPress('done'); return true; }
      if (e.key.length === 1) { kbAdd(e.key); KB.typed = true; return true; }
      return false;
    },
  });
}
function kbShown() { const k = KB; if (!k.v) return `<span class="kb-ph">${esc(k.ph)}</span>`; return esc(k.type === 'password' && !k.show ? '•'.repeat(k.v.length) : k.v); }
function kbHtml() {
  const k = KB; const rows = k.sym ? KB_ROWS.sym : KB_ROWS.abc; const q = KB_QUICK[k.type] || KB_QUICK.text;
  const key = (f, code, label, cls, extra) => `<button type="button" class="kb-k${cls ? ' ' + cls : ''}" data-f="${f}" data-act="kb" data-k="${esc(code)}"${extra || ''}>${label}</button>`;
  const keys = rows.map((r, ri) => [...r].map((ch, ci) => { const c = k.shift ? ch.toUpperCase() : ch; return key(`k-${ri}-${ci}`, 'c:' + c, esc(c), '', ri === 1 && ci === 0 ? ' data-default' : ''); }).join('')
    + (ri === 0 ? key('k-0-10', 'del', icon('backspace', 40), 'fn', ' aria-label="Löschen"') : '')).join('');
  const last = key('k-4-0', 'shift', icon('shift', 36), 'fn', ` aria-label="Grossbuchstaben" aria-pressed="${k.shift}"`)
    + key('k-4-1', 'sym', k.sym ? 'abc' : '#+=', 'fn', ` aria-pressed="${k.sym}"`)
    + key('k-4-2', 'q:' + q[0], esc(q[0]), 'fn w2') + key('k-4-3', 'space', 'Leerzeichen', 'fn w3')
    + key('k-4-4', 'q:' + q[1], esc(q[1]), 'fn w2') + key('k-4-5', 'done', `${icon('check', 34)} Fertig`, 'done w2');
  return `<div class="kb-top"><div class="kb-lab">${esc(k.label)}</div><div class="kb-line"><div class="kb-field"><span class="kb-val">${kbShown()}</span><i class="kb-caret"></i></div>
      ${k.type === 'password' ? `<button type="button" class="kb-k fn kb-show" data-f="k-show" data-act="kb" data-k="show">${icon(k.show ? 'eyeOff' : 'eye', 34)} ${k.show ? 'verbergen' : 'anzeigen'}</button>` : ''}</div></div>
    <div class="kb-grid">${keys}${last}</div>
    <p class="kb-hint">Tippen geht auch mit der Tastatur des Computers. Zurück schliesst ohne Änderung.</p>`;
}
function kbAdd(s) { if (!KB) return; KB.v += s; if (KB.shift && s.length === 1) { KB.shift = false; redrawMenu(kbHtml()); return; } kbField(); }
function kbField() { const v = $('.kb-val', scopeEl()); if (v) v.innerHTML = kbShown(); }
function kbPress(code) {
  const k = KB; if (!k) return;
  if (code.startsWith('c:')) { kbAdd(code.slice(2)); return; }
  if (code.startsWith('q:')) { kbAdd(code.slice(2)); return; }
  if (code === 'space') { kbAdd(' '); return; }
  if (code === 'del') { k.v = k.v.slice(0, -1); kbField(); return; }
  if (code === 'shift') { k.shift = !k.shift; redrawMenu(kbHtml()); return; }
  if (code === 'sym') { k.sym = !k.sym; redrawMenu(kbHtml()); return; }
  if (code === 'show') { k.show = !k.show; redrawMenu(kbHtml()); return; }
  if (code === 'done') { k.done = true; const val = k.v; closeMenu(); if (k.o.onDone) k.o.onDone(val); }
}
ACT.kb = d => kbPress(d.k);

/* ---------- TV frame, remote, mockup tool bar ---------- */
const REM = { on: true };
try { const r = JSON.parse(localStorage.getItem(KEY + '-remote')); if (r) Object.assign(REM, r); } catch (e) { /* ignore */ }
function fit() {
  const st = $('#stage'); const sw = st.clientWidth; const sh = st.clientHeight;
  const bez = 28; const stand = 64; const keys = 40; const pad = 40; const remW = 232; const remH = 200;
  const side = Math.min((sw - pad * 2 - (REM.on ? remW + 40 : 0) - bez) / W, (sh - pad - stand - keys - bez) / H);
  const below = REM.on ? Math.min((sw - pad * 2 - bez) / W, (sh - pad - stand - keys - bez - remH - 16) / H) : side;
  const stack = REM.on && below > side;
  st.classList.toggle('stack', stack);
  document.body.classList.toggle('remote-off', !REM.on);
  SCALE = Math.max(0.12, stack ? below : side);
  const fitEl = $('#fit'); fitEl.style.width = `${W * SCALE}px`; fitEl.style.height = `${H * SCALE}px`;
  $('#screen').style.transform = `scale(${SCALE})`;
  $$('[data-tool="remote"]').forEach(b => b.setAttribute('aria-pressed', String(REM.on)));
}
const TOOL = {
  reset: () => {
    try { localStorage.removeItem(KEY); } catch (e) { /* ignore */ }
    S = seed(); Object.keys(MEM).forEach(k => delete MEM[k]); RESETS.forEach(fn => fn()); LIVE_SIM.reset();
    OFF = false; $('#off').hidden = true; lastPath = null;
    history.replaceState(null, '', '#/live/channels'); NAVSTK.length = 0; NAVSTK.push('#/live/channels');
    render(); toast('Mockup zurückgesetzt');
  },
  remote: () => { REM.on = !REM.on; try { localStorage.setItem(KEY + '-remote', JSON.stringify(REM)); } catch (e) { /* ignore */ } fit(); },
};
window.addEventListener('resize', fit);
$$('#remote [data-rc]').forEach(b => b.addEventListener('mousedown', e => e.preventDefault()));

/* the jump menu comes from the register (js/reg-*.js); PAGES stays for extra entries of page files */
const PAGES = [];
