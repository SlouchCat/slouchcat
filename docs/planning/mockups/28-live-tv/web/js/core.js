/* Konzept 28 «Live-TV», web mockup core (viewer app and administration): helpers, icons, router,
   dialogs, toasts, event delegation, the bridge to LIVE_SIM. Taken from the mockup of Konzept 25 and
   adapted. No framework, no build. Owned by the core; page files do not change it (BAUKASTEN.md). */
'use strict';

const $ = (s, r = document) => r.querySelector(s);
const $$ = (s, r = document) => Array.from(r.querySelectorAll(s));
const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
const cls = (...a) => a.filter(Boolean).join(' ');

const ICONS = {
  play: '<path d="M7 5v14l12-7z" fill="currentColor" stroke="none"/>',
  pause: '<path d="M8 5v14M16 5v14"/>',
  info: '<circle cx="12" cy="12" r="9"/><path d="M12 11v6M12 7.5v.5"/>',
  plus: '<path d="M12 5v14M5 12h14"/>',
  check: '<path d="m5 12.5 4.5 4.5L19 7.5"/>',
  x: '<path d="M6 6l12 12M18 6 6 18"/>',
  bookmark: '<path d="M7 4h10v16l-5-4-5 4z"/>',
  search: '<circle cx="11" cy="11" r="6.5"/><path d="m16 16 4.5 4.5"/>',
  bell: '<path d="M6 16V11a6 6 0 1 1 12 0v5l1.5 2h-15zM10 20.5h4"/>',
  home: '<path d="M4 11.5 12 5l8 6.5V20h-5v-5h-6v5H4z"/>',
  film: '<rect x="4" y="5" width="16" height="14" rx="2"/><path d="M8 5v14M16 5v14M4 9.5h4M4 14.5h4M16 9.5h4M16 14.5h4"/>',
  tv: '<rect x="3.5" y="7" width="17" height="12" rx="2"/><path d="m9 3.5 3 3.5 3-3.5"/>',
  star: '<path d="m12 4 2.5 5.2 5.7.8-4.1 4 1 5.7L12 17l-5.1 2.7 1-5.7-4.1-4 5.7-.8z"/>',
  mic: '<rect x="9" y="3.5" width="6" height="11" rx="3"/><path d="M6 11.5a6 6 0 0 0 12 0M12 17.5V21"/>',
  book: '<path d="M5 4.5h11a3 3 0 0 1 3 3V20H8a3 3 0 0 1-3-3zM5 17a3 3 0 0 1 3-3h11"/>',
  radio: '<rect x="4" y="9" width="16" height="11" rx="2"/><path d="m7 9 9-5M8.5 14.5h.01"/><circle cx="15" cy="14.5" r="2.5"/>',
  layers: '<path d="m12 4 8.5 4.5L12 13 3.5 8.5zM3.5 12.5 12 17l8.5-4.5M3.5 16.5 12 21l8.5-4.5"/>',
  settings: '<path d="M4 7h10M18 7h2M4 17h2M10 17h10"/><circle cx="16" cy="7" r="2"/><circle cx="8" cy="17" r="2"/>',
  users: '<circle cx="9" cy="8.5" r="3.5"/><path d="M2.5 20a6.5 6.5 0 0 1 13 0M16 5.5a3.5 3.5 0 0 1 0 6.5M18 14.5a6 6 0 0 1 3.5 5.5"/>',
  user: '<circle cx="12" cy="8.5" r="3.5"/><path d="M5 20a7 7 0 0 1 14 0"/>',
  shield: '<path d="M12 3.5 5 6v6c0 4.2 3 7.2 7 8.5 4-1.300 7-4.300 7-8.500V6z"/>',
  logout: '<path d="M10 4.5H6a1.500 1.500 0 0 0-1.500 1.500v12A1.500 1.500 0 0 0 6 19.500h4M15 8l4 4-4 4M19 12H9.500"/>',
  menu: '<path d="M4 7h16M4 12h16M4 17h16"/>',
  chevR: '<path d="m9 5 7 7-7 7"/>',
  chevL: '<path d="m15 5-7 7 7 7"/>',
  chevD: '<path d="m5 9 7 7 7-7"/>',
  arrowL: '<path d="M20 12H5M11 6l-6 6 6 6"/>',
  clock: '<circle cx="12" cy="12" r="8.5"/><path d="M12 7.500V12l3 2"/>',
  hourglass: '<path d="M7 4h10M7 20h10M8 4c0 4 4 5.500 4 8s-4 4-4 8M16 4c0 4-4 5.500-4 8s4 4 4 8"/>',
  truck: '<path d="M3 6.500h10.500v9.500H3zM13.500 10h4L20.500 13v3h-7"/><circle cx="7" cy="17.500" r="1.800"/><circle cx="17" cy="17.500" r="1.800"/>',
  alert: '<path d="M12 4.500 21 19.500H3z"/><path d="M12 10v4.500M12 17v.300"/>',
  ban: '<circle cx="12" cy="12" r="8.500"/><path d="m6 6 12 12"/>',
  undo: '<path d="M8 8H4V4M4.500 8A8.500 8.500 0 1 1 3.800 14"/>',
  refresh: '<path d="M20 5v5h-5M4 19v-5h5M19.300 10A8 8 0 0 0 5.500 7.500M4.700 14a8 8 0 0 0 13.800 2.500"/>',
  external: '<path d="M13 5h6v6M19 5l-8.500 8.500M10 6.500H6A1.500 1.500 0 0 0 4.500 8v10A1.500 1.500 0 0 0 6 19.500h10a1.500 1.500 0 0 0 1.500-1.500v-4"/>',
  pen: '<path d="m5 19 1-4L16.500 4.500l3 3L9 18zM14.500 6.500l3 3"/>',
  trash: '<path d="M5 7h14M10 4.500h4M7 7l1 12.500h8L17 7M10 10.500v6M14 10.500v6"/>',
  link: '<path d="M10 14a4 4 0 0 0 5.700 0l3-3a4 4 0 0 0-5.700-5.700l-1 1M14 10a4 4 0 0 0-5.700 0l-3 3a4 4 0 0 0 5.700 5.700l1-1"/>',
  send: '<path d="M20.500 3.500 10 14M20.500 3.500 14 20.500 10 14 3.500 10z"/>',
  wish: '<path d="M12 3.500l1.800 4.700 4.700 1.800-4.700 1.800L12 16.500l-1.800-4.700L5.500 10l4.700-1.800zM18.500 15l.9 2.100 2.100.900-2.100.900-.900 2.100-.900-2.100-2.100-.900 2.100-.900z"/>',
  list: '<path d="M8 7h12M8 12h12M8 17h12M4 7h.01M4 12h.01M4 17h.01"/>',
  eye: '<path d="M2.500 12S6 5.500 12 5.500 21.500 12 21.500 12 18 18.500 12 18.500 2.500 12 2.500 12z"/><circle cx="12" cy="12" r="3"/>',
  lock: '<rect x="5" y="10.500" width="14" height="9.500" rx="2"/><path d="M8 10.500V8a4 4 0 0 1 8 0v2.500"/>',
  server: '<rect x="4" y="4.500" width="16" height="6" rx="1.500"/><rect x="4" y="13.500" width="16" height="6" rx="1.500"/><path d="M7.500 7.500h.01M7.500 16.500h.01"/>',
  zap: '<path d="M13 3 5 13.500h6L10 21l8-10.500h-6z"/>',
  plug: '<path d="M9 3.500V8M15 3.500V8M6.500 8h11v3.500a5.500 5.500 0 0 1-11 0zM12 17v3.500"/>',
  mail: '<rect x="3.500" y="5.500" width="17" height="13" rx="2"/><path d="m4 7 8 6 8-6"/>',
  phone: '<rect x="7" y="3" width="10" height="18" rx="2"/><path d="M11 17.500h2"/>',
  tablet: '<rect x="5" y="3" width="14" height="18" rx="2"/><path d="M11 17.500h2"/>',
  monitor: '<rect x="3" y="4.500" width="18" height="12" rx="2"/><path d="M8.500 20h7M12 16.500V20"/>',
  key: '<circle cx="8" cy="15" r="4"/><path d="m11 12 8.500-8.500M16 6.500l2.500 2.500M14 8.500l2 2"/>',
  chart: '<path d="M4 20V4M4 20h16M8 16v-4M12 16V8M16 16v-6"/>',
  database: '<ellipse cx="12" cy="6" rx="7.500" ry="2.500"/><path d="M4.500 6v6c0 1.400 3.400 2.500 7.500 2.500s7.500-1.100 7.500-2.500V6M4.500 12v6c0 1.400 3.400 2.500 7.500 2.500s7.500-1.100 7.500-2.500v-6"/>',
  megaphone: '<path d="M4 10v4l3 .500L8.500 19h2.500l-1-4 9.500 3V6L7 9.500z"/>',
  help: '<circle cx="12" cy="12" r="9"/><path d="M9.500 9.500a2.500 2.500 0 1 1 3.500 2.300c-.700.400-1 1-1 1.700M12 16.500v.300"/>',
  pin: '<path d="M9 4h6l-1 5 3 3H7l3-3zM12 12v8"/>',
  note: '<path d="M6 4.500h9l3 3V19.500H6zM9 10h6M9 13.500h6M9 17h3"/>',
  dots: '<circle cx="6" cy="12" r="1.300" fill="currentColor"/><circle cx="12" cy="12" r="1.300" fill="currentColor"/><circle cx="18" cy="12" r="1.300" fill="currentColor"/>',
  folder: '<path d="M3.500 7A1.500 1.500 0 0 1 5 5.500h4.500l2 2.500H19a1.500 1.500 0 0 1 1.500 1.500V17A1.500 1.500 0 0 1 19 18.500H5A1.500 1.500 0 0 1 3.500 17z"/>',
  dl: '<path d="M12 4v11M7.500 11l4.500 4.500 4.500-4.500M5 19.500h14"/>',
  record: '<circle cx="12" cy="12" r="6.5" fill="currentColor" stroke="none"/>',
  recordSeries: '<circle cx="8.5" cy="12" r="5" fill="currentColor" stroke="none"/><circle cx="16.5" cy="12" r="5"/>',
  reminder: '<path d="M6 16V11a6 6 0 1 1 12 0v5l1.5 2h-15zM10 20.5h4"/><path d="M12 8.5V11l1.8 1.2"/>',
  calendar: '<rect x="4" y="5.5" width="16" height="14" rx="2"/><path d="M4 10h16M8.5 3.5v4M15.5 3.5v4"/>',
  live: '<path d="M12 13.5v7M9 20.5h6"/><circle cx="12" cy="11" r="1.8"/><path d="M8.2 7.5a5.200 5.200 0 0 0 0 7M15.8 7.5a5.200 5.200 0 0 1 0 7M5.500 5a9 9 0 0 0 0 12M18.500 5a9 9 0 0 1 0 12"/>',
  heart: '<path d="M12 20s-7.500-4.600-7.500-10A4.300 4.300 0 0 1 12 7.400 4.300 4.300 0 0 1 19.500 10c0 5.400-7.500 10-7.500 10z"/>',
  heartOn: '<path d="M12 20s-7.500-4.600-7.500-10A4.300 4.300 0 0 1 12 7.400 4.300 4.300 0 0 1 19.500 10c0 5.400-7.500 10-7.500 10z" fill="currentColor"/>',
  grid: '<rect x="4" y="4" width="7" height="7" rx="1"/><rect x="13" y="4" width="7" height="7" rx="1"/><rect x="4" y="13" width="7" height="7" rx="1"/><rect x="13" y="13" width="7" height="7" rx="1"/>',
  filter: '<path d="M4 6h16M7 12h10M10 18h4"/>',
  upload: '<path d="M12 20V9M7.500 13 12 8.500l4.500 4.500M5 4.500h14"/>',
  wifi: '<path d="M2.500 9a14 14 0 0 1 19 0M6 12.500a9 9 0 0 1 12 0M9.500 16a4 4 0 0 1 5 0"/><circle cx="12" cy="19.500" r="1" fill="currentColor"/>',
  home2: '<path d="M3.500 11 12 4l8.500 7M6 9.500V20h12V9.500"/><path d="M10 20v-5h4v5"/>',
  arrowUp: '<path d="M12 19V5M6 11l6-6 6 6"/>',
  arrowDown: '<path d="M12 5v14M6 13l6 6 6-6"/>',
  recordHollow: '<circle cx="12" cy="12" r="6.5"/>',
  stop: '<rect x="6" y="6" width="12" height="12" rx="2" fill="currentColor" stroke="none"/>',
  expand: '<path d="M4 9V4h5M20 9V4h-5M4 15v5h5M20 15v5h-5"/>',
  loadspin: '<path d="M12 3a9 9 0 1 0 9 9"/>',
  /* player controls of the web app (formerly added by viewer-b.js) */
  vbVol: '<path d="M4 9.5h3.5L12 5.5v13l-4.5-4H4z"/><path d="M15.5 9a4 4 0 0 1 0 6M18 6.5a7.5 7.5 0 0 1 0 11"/>',
  vbMute: '<path d="M4 9.5h3.5L12 5.5v13l-4.5-4H4z"/><path d="m16 9.5 5 5M21 9.5l-5 5"/>',
  vbFull: '<path d="M4 9V4h5M20 9V4h-5M4 15v5h5M20 15v5h-5"/>',
  vbBack: '<path d="M4 4v4.5h4.5M4.7 8.5A8 8 0 1 1 4 13"/>',
  vbFwd: '<path d="M20 4v4.5h-4.5M19.3 8.5A8 8 0 1 0 20 13"/>',
  vbSub: '<rect x="3.5" y="5.5" width="17" height="13" rx="2"/><path d="M7 12.5h4M13 12.5h4M7 15.5h7"/>',
  vbAudio: '<path d="M4 13v-2M8 16V8M12 19V5M16 15V9M20 13v-2"/>',
  vbChapter: '<path d="M5 5v14M9 8h10M9 12h10M9 16h6"/>',
  vbBlank: '',
  copy: '<rect x="8" y="8" width="11.500" height="11.500" rx="2"/><path d="M5 15.500V6A1.500 1.500 0 0 1 6.500 4.500H15"/>',
};
function icon(name, size = 20) {
  const p = ICONS[name] || ICONS.info;
  return `<svg width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.900" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${p}</svg>`;
}

/* image with graceful fallback: the public repository carries no artwork */
function img(src, alt, extra = '') {
  if (!src) return `<span class="ph">${esc(alt)}</span>`;
  return `<img src="${esc(src)}" alt="${esc(alt)}" loading="lazy" ${extra} onerror="imgFail(this)">`;
}
function imgFail(el) { const s = document.createElement('span'); s.className = 'ph'; s.textContent = el.alt; el.replaceWith(s); }

/* ---------- mockup preferences (persisted: view width, hidden side nav) ---------- */
const STORE_KEY = 'sc28-web-mock-v1';
let S;
try { S = JSON.parse(localStorage.getItem(STORE_KEY)) || null; } catch (e) { S = null; }
if (!S) S = { view: 'desktop', sideHidden: false };
function save() { try { localStorage.setItem(STORE_KEY, JSON.stringify(S)); } catch (e) { /* private mode */ } }
function commit() { save(); render(); }

/* ---------- transient UI state (never persisted) ---------- */
const ui = { open: {}, dialog: null, drawer: false, toasts: [], draft: {}, busy: {}, q: '', filter: {}, menu: null, tree: {} };

function getPath(path) {
  const parts = path.split('.');
  let o = parts[0] === 'S' ? S : parts[0] === 'ui' ? ui : null;
  for (let i = 1; i < parts.length && o != null; i++) o = o[parts[i]];
  return o;
}
function setPath(path, value) {
  const parts = path.split('.');
  let o = parts[0] === 'S' ? S : ui;
  for (let i = 1; i < parts.length - 1; i++) { if (o[parts[i]] == null) o[parts[i]] = {}; o = o[parts[i]]; }
  o[parts[parts.length - 1]] = value;
  if (parts[0] === 'S') save();
}

/* ---------- small building blocks ---------- */
function sw(path, label, opts = {}) {
  const on = !!getPath(path);
  return `<button type="button" class="sw" role="switch" aria-checked="${on}" aria-label="${esc(label)}" data-act="tog" data-path="${path}" data-fid="sw:${path}" ${opts.after ? `data-after="${opts.after}"` : ''} ${opts.disabled ? 'disabled' : ''}></button>`;
}
function selectEl(path, options, opts = {}) {
  const cur = String(getPath(path) ?? '');
  return `<select class="sel" data-bind="${path}" data-rerender data-fid="sel:${path}" aria-label="${esc(opts.label || '')}" ${opts.after ? `data-after="${opts.after}"` : ''} ${opts.disabled ? 'disabled' : ''}>${options
    .map((o) => `<option value="${esc(o.v)}" ${String(o.v) === cur ? 'selected' : ''} ${o.disabled ? 'disabled' : ''}>${esc(o.t)}</option>`)
    .join('')}</select>`;
}
function isOpen(id, def = false) { return ui.open[id] ?? def; }
/* collapsible band; closed by default (Marco's rule), the head carries the summary */
function band(id, title, meta, body, opts = {}) {
  const open = isOpen(id, !!opts.open);
  const k = opts.viewer ? 'bandp' : 'band';
  return `<section class="${k}" data-open="${open}"><div class="bh"><h2><button type="button" class="tg" data-act="band" data-id="${id}" data-def="${opts.open ? 1 : 0}" aria-expanded="${open}" data-fid="band:${id}"><span class="chev">${icon('chevR', 16)}</span>${opts.icon ? icon(opts.icon, 16) : ''}<span>${title}</span></button></h2><div class="mt">${meta || ''}</div></div><div class="bb">${open ? body : ''}</div></section>`;
}
function notice(kind, title, text, acts = '') {
  const ic = kind === 'bad' ? 'alert' : kind === 'warn' ? 'alert' : kind === 'ok' ? 'check' : 'info';
  return `<div class="notice ${kind}"><span class="ic">${icon(ic)}</span><div><b>${title}</b>${text ? `<p>${text}</p>` : ''}${acts ? `<div class="acts">${acts}</div>` : ''}</div></div>`;
}
function fmtDate(ts) { return new Date(ts).toLocaleDateString('de-CH', { day: 'numeric', month: 'long', year: 'numeric' }); }
function fmtTime(ts) { return new Date(ts).toLocaleTimeString('de-CH', { hour: '2-digit', minute: '2-digit' }); }
function ago(ts) {
  const s = Math.max(0, (Date.now() - ts) / 1000);
  if (s < 60) return 'gerade eben';
  if (s < 3600) return `vor ${Math.round(s / 60)} min`;
  if (s < 86400) return `vor ${Math.round(s / 3600)} h`;
  const d = Math.round(s / 86400);
  return d === 1 ? 'gestern' : `vor ${d} Tagen`;
}

/* ---------- router ---------- */
const ROUTES = [];
function route(pattern, fn) {
  const keys = [];
  const re = new RegExp('^' + pattern.replace(/:[a-zA-Z]+/g, (m) => { keys.push(m.slice(1)); return '([^/]+)'; }) + '$');
  ROUTES.push({ re, keys, fn, pattern });
}
function parseHash() {
  const h = location.hash.replace(/^#/, '') || '/';
  const [path, qs] = h.split('?');
  const query = {};
  (qs || '').split('&').filter(Boolean).forEach((p) => { const [k, v] = p.split('='); query[decodeURIComponent(k)] = decodeURIComponent(v || ''); });
  return { path: path || '/', query };
}
function go(hash) { if (location.hash === hash) render(); else location.hash = hash; }

let lastPath = null;
let renderQueued = false;
function render() {
  if (renderQueued) return;
  renderQueued = true;
  queueMicrotask(() => { renderQueued = false; renderNow(); });
}
function keepFocus() {
  const a = document.activeElement;
  if (!a || !a.dataset || !a.dataset.fid) return null;
  let sel = null;
  try { if (a.selectionStart != null) sel = [a.selectionStart, a.selectionEnd]; } catch (e) { /* not a text field */ }
  return { fid: a.dataset.fid, sel };
}
function restoreFocus(f) {
  if (!f) return;
  const el = document.querySelector(`[data-fid="${CSS.escape(f.fid)}"]`);
  if (!el) return;
  el.focus({ preventScroll: true });
  if (f.sel) { try { el.setSelectionRange(f.sel[0], f.sel[1]); } catch (e) { /* ignore */ } }
}
function renderNow() {
  const { path, query } = parseHash();
  if (path !== lastPath) { ui.menu = null; if (lastPath !== null && ui.dialog) ui.dialog = null; } /* a dialog belongs to its page */
  let view = liveGate(path);
  if (!view) for (const r of ROUTES) {
    const m = path.match(r.re);
    if (m) { const params = {}; r.keys.forEach((k, i) => (params[k] = decodeURIComponent(m[i + 1]))); view = r.fn(params, query); break; }
  }
  if (!view) view = notFoundView();
  const sc = $('#scroll');
  const same = path === lastPath;
  const top = sc.scrollTop;
  const hs = {};
  if (same) $$('[data-keep]', sc).forEach((el) => (hs[el.dataset.keep] = el.scrollLeft));
  const f = keepFocus();
  document.documentElement.dataset.shell = view.type;
  sc.innerHTML = SHELL[view.type](view);
  if (same) { sc.scrollTop = top; $$('[data-keep]', sc).forEach((el) => { if (hs[el.dataset.keep] != null) { el.style.scrollBehavior = 'auto'; el.scrollLeft = hs[el.dataset.keep]; el.style.scrollBehavior = ''; } }); }
  else { sc.scrollTop = 0; ui.drawer = false; }
  renderOverlays();
  if (same) restoreFocus(f);
  else if (!ui.dialog) { const h = $('h1', sc); if (h) { h.setAttribute('tabindex', '-1'); h.focus({ preventScroll: true }); } }
  document.title = (view.title ? view.title + ' · ' : '') + 'Slouchcat · Live-TV (Mockup)';
  lastPath = path;
  if (view.after) view.after(sc);
  if (window.MOCK && MOCK.render) MOCK.render();
  LIVE_UI.refresh(sc);
}

/* ---------- overlays: dialog, drawer, toasts ---------- */
const DIALOGS = {};
function openDialog(name, props = {}) {
  const a = document.activeElement;
  ui.dialog = { name, props, opener: a && a.dataset ? a.dataset.fid : null, fresh: true };
  renderOverlays();
}
function closeDialog() {
  const d = ui.dialog;
  ui.dialog = null;
  renderOverlays();
  if (d && d.opener) restoreFocus({ fid: d.opener });
}
function renderOverlays() {
  const ov = $('#overlays');
  let dl = $('#ov-dialog'), dr = $('#ov-drawer');
  if (!dl) { ov.insertAdjacentHTML('beforeend', '<div id="ov-drawer"></div><div id="ov-dialog"></div><div class="toasts" id="ov-toasts" aria-live="polite"></div>'); dl = $('#ov-dialog'); dr = $('#ov-drawer'); }
  const f = keepFocus();
  if (ui.dialog) {
    const d = DIALOGS[ui.dialog.name];
    const inner = d ? d(ui.dialog.props) : { title: 'Dialog', body: '' };
    dl.innerHTML = `<div class="scrim" data-act="scrim"><div class="dlg ${inner.wide ? 'wide' : ''}" role="dialog" aria-modal="true" aria-labelledby="dlg-t"><h2 id="dlg-t" tabindex="-1">${inner.title}</h2>${inner.body}${inner.actions ? `<div class="dlg-acts">${inner.actions}</div>` : ''}</div></div>`;
    if (ui.dialog.fresh) { ui.dialog.fresh = false; const first = $('[data-autofocus]', dl) || $('#dlg-t', dl); first.focus({ preventScroll: true }); }
    else restoreFocus(f);
  } else dl.innerHTML = '';
  dr.innerHTML = ui.drawer ? `<div class="scrim" data-act="scrim" style="justify-content:flex-start;padding:0">${SHELL.drawer()}</div>` : '';
  renderToasts();
}
function renderToasts() {
  const t = $('#ov-toasts');
  if (!t) return;
  t.innerHTML = ui.toasts.map((x) => `<div class="toast ${x.kind}" role="status">${icon(x.kind === 'bad' ? 'alert' : x.kind === 'warn' ? 'alert' : x.kind === 'mail' ? 'send' : 'check', 18)}<span>${x.html}</span></div>`).join('');
}
let toastId = 0;
function toast(html, kind = 'ok', ms = 5200) {
  /* the same message twice (for example repeated rewinding before the start) is shown once */
  const same = ui.toasts.find((x) => x.html === html);
  if (same) { clearTimeout(same.t); same.t = setTimeout(() => { ui.toasts = ui.toasts.filter((x) => x !== same); renderToasts(); }, ms); return; }
  const id = ++toastId;
  const entry = { id, html, kind }; ui.toasts.push(entry);
  if (ui.toasts.length > 3) ui.toasts.shift();
  renderToasts();
  entry.t = setTimeout(() => { ui.toasts = ui.toasts.filter((x) => x.id !== id); renderToasts(); }, ms);
}

/* ---------- actions (event delegation) ---------- */
const ACT = {
  tog(d) { setPath(d.path, !getPath(d.path)); if (d.after && ACT[d.after]) ACT[d.after](d); render(); },
  band(d) { ui.open[d.id] = !isOpen(d.id, d.def === '1'); render(); },
  open(d) { ui.open[d.id] = !isOpen(d.id); render(); },
  set(d) { let v = d.value; if (v === 'true') v = true; else if (v === 'false') v = false; else if (d.num != null) v = Number(v); setPath(d.path, v); if (d.after && ACT[d.after]) ACT[d.after](d); render(); },
  dialog(d) { openDialog(d.name, { ...d }); },
  close() { closeDialog(); },
  scrim(d, el, e) { if (e.target === el) { if (ui.drawer) { ui.drawer = false; renderOverlays(); } else closeDialog(); } },
  drawer() { ui.drawer = !ui.drawer; renderOverlays(); const first = $('#ov-drawer a, #ov-drawer button'); if (first) first.focus(); },
  back() { if (history.length > 1) history.back(); else go('#/'); },
  nop() {},
  scrollRow(d) { const el = document.querySelector(`[data-keep="${d.row}"]`); if (el) el.scrollBy({ left: (d.dir === 'prev' ? -1 : 1) * el.clientWidth * 0.8, behavior: 'smooth' }); },
};
document.addEventListener('click', (e) => {
  const a = e.target.closest('[data-act]');
  if (!a) { if (ui.menu && !e.target.closest('.lmenu')) { ui.menu = null; render(); } return; }
  if (a.disabled || a.getAttribute('aria-disabled') === 'true') { e.preventDefault(); return; }
  if (ui.menu && a.dataset.act !== 'menu' && !a.closest('.lmenu')) ui.menu = null;
  if (a.dataset.act === 'scrim' && e.target !== a) return;
  const fn = ACT[a.dataset.act];
  if (a.tagName === 'A' && !a.getAttribute('href')) e.preventDefault();
  if (fn) fn(a.dataset, a, e);
  else console.warn('Mockup: keine Aktion', a.dataset.act);
});
function valueOf(el) {
  if (el.type === 'checkbox') return el.checked;
  if (el.dataset.num != null) return el.value === '' ? '' : Number(el.value);
  if (el.value === 'true') return true;
  if (el.value === 'false') return false;
  return el.value;
}
document.addEventListener('input', (e) => {
  const el = e.target.closest('[data-bind]');
  if (!el || el.tagName === 'SELECT' || el.type === 'checkbox' || el.type === 'radio') return;
  setPath(el.dataset.bind, valueOf(el));
  if (el.dataset.live != null) render();
});
document.addEventListener('change', (e) => {
  const el = e.target.closest('[data-bind]');
  if (!el) return;
  if (el.type === 'radio' && !el.checked) return;
  setPath(el.dataset.bind, el.type === 'radio' ? (el.dataset.num != null ? Number(el.value) : valueOf(el)) : valueOf(el));
  if (el.dataset.after && ACT[el.dataset.after]) ACT[el.dataset.after](el.dataset, el);
  if (el.dataset.rerender != null || el.type === 'checkbox' || el.type === 'radio') render();
});
document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape') {
    if (ui.menu) { e.preventDefault(); const m = ui.menu; ui.menu = null; render(); restoreFocus({ fid: m.opener }); return; }
    if (ui.dialog) { e.preventDefault(); closeDialog(); }
    else if (ui.drawer) { ui.drawer = false; renderOverlays(); }
    else if (window.MOCK && MOCK.open) { MOCK.open = false; MOCK.render(); }
    return;
  }
  if (e.key === 'Tab' && (ui.dialog || ui.drawer)) {
    const root = ui.dialog ? $('#ov-dialog .dlg') : $('#ov-drawer nav');
    if (!root) return;
    const els = $$('a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea, [tabindex="0"]', root).filter((x) => x.offsetParent !== null);
    if (!els.length) return;
    const first = els[0], last = els[els.length - 1];
    if (!root.contains(document.activeElement)) { e.preventDefault(); first.focus(); }
    else if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
    else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
  }
  if (e.key === 'Enter' && e.target.matches && e.target.matches('input.inp') && e.target.dataset.enter) { const fn = ACT[e.target.dataset.enter]; if (fn) { e.preventDefault(); fn(e.target.dataset, e.target, e); } }
});
window.addEventListener('hashchange', render);

/* device height for the sticky side navigation; the frame can be a phone inside a desktop window */
function measure() { const d = $('#device'); if (d) document.documentElement.style.setProperty('--dh', d.clientHeight + 'px'); }
window.addEventListener('resize', measure);

/* ======================================================================================
   Live-TV bridge: rights gate, placeholders from the register, dialog interface, menus, states
   ====================================================================================== */
const SIM = window.LIVE_SIM;
const LU = window.LIVE_UI;
LU.img = '../shared/img/';
const fmt = LU.fmt;

/** Without the right the area is missing: every address answers «Nicht gefunden» (404, never 403). */
function liveGate(path) {
  const r = SIM.rights();
  /* right taken away while watching (mockup state «Recht entzogen …»): the player says so (14, 4.12; page of viewer-b.js) */
  if (SIM.flags().revoked && path.startsWith('/live/watch/')) { const rv = ROUTES.find((x) => x.pattern === '/player/revoked'); if (rv) return rv.fn({}, { ch: path.split('/')[3] }); }
  /* variant E-28-42 (a), mockup state a12 (default off = Vorgabe (b): 404 as before). A-12 (14 3.14): the primary profile keeps LS-14 and the details of its profiles' recordings, reached only
     through «Profile verwalten», even without its own right «Live-TV» or «Aufnahmen»; playing needs Live-TV */
  const kid = path.match(/^\/live\/recordings\/([^/]+)(\/play)?$/); const kr = kid && SIM.recording(kid[1]);
  if (SIM.flags().a12 && SIM.profile().primary && (path === '/live/recordings/profiles' || (kr && SIM.childProfiles().some((p) => p.id === kr.profile) && (!kid[2] || r.liveTv)))) return null;
  if (path.startsWith('/live') && !r.liveTv) return notFoundView();
  if (/^\/live\/(recordings|scheduled)/.test(path) && !r.recordings) return notFoundView();
  if (path === '/live/recordings/profiles' && !SIM.profile().primary) return notFoundView();
  return null;
}
function notFoundView() {
  return { type: 'viewer', nav: '', title: 'Nicht gefunden', html: `<div class="page"><div class="head"><h1>Nicht gefunden</h1><p class="sub">Diese Seite gibt es nicht.</p><div class="acts"><a class="btn secondary" href="#/" data-fid="nf:home">Zur Startseite</a></div></div></div>` };
}

/* ---------- placeholders for every register entry without a page yet ---------- */
function placeholderView(entries, client) {
  const e = entries[0];
  const ids = entries.map((x) => `<b>${esc(x.id)}</b> ${esc(x.title)}`).join(', ');
  const html = `<div class="ph-page"><span class="ph-id">${entries.map((x) => esc(x.id)).join(' · ')}</span><h1>${esc(e.title)} wird ergänzt</h1>
    <p>${ids}. Diese Seite baut «${esc(LIVE_REG.AGENTS[e.agent].name)}» in <code>${esc(e.file)}</code>.${e.note ? ' ' + esc(e.note) : ''}</p>
    <div class="row-gap"><button type="button" class="btn secondary" data-act="back" data-fid="ph:back">${icon('arrowL')}Zurück</button><a class="btn ghost" href="../index.html" data-fid="ph:all">Alle Seiten</a></div>${LU.sampleNote()}</div>`;
  if (client === 'admin') { const a = adminAreaOf(e.route); return { type: 'admin', area: a.area, page: a.page, title: e.title, intro: 'Platzhalter im Mockup.', html }; }
  return { type: 'viewer', nav: e.route && e.route.startsWith('/live') ? 'live' : '', title: e.title, html };
}
/** at boot: one placeholder route per register pattern that no page file has registered */
function installPlaceholders() {
  const have = new Set(ROUTES.map((r) => r.pattern));
  const byPattern = {};
  LIVE_REG.forClient(['web', 'admin']).filter((e) => e.route && !have.has(e.route)).forEach((e) => (byPattern[e.route] || (byPattern[e.route] = [])).push(e));
  Object.keys(byPattern).sort((a, b) => (a.includes(':') - b.includes(':')) || b.length - a.length).forEach((pt) => {
    const list = byPattern[pt].filter((e) => !e.id.startsWith('LD-'));
    if (list.length) route(pt, () => placeholderView(list, list[0].client));
  });
  /* dialogs: a host page per dialog that opens it */
  route('/live/dialog/:id', (pm) => dialogHostView(pm.id));
}

/* ---------- dialog interface (LD-01 … LD-11) ----------
   Page files open dialogs only through LiveDlg.*; «Web Zuschauer B» fills them with registerLiveDialog. */
const LIVE_DIALOGS = {};
/** registerLiveDialog('LD-01', (props) => ({ title, body, actions, wide })) */
function registerLiveDialog(id, fn) { LIVE_DIALOGS[id] = fn; DIALOGS['live:' + id] = (props) => fn(props || {}); }
function liveDialog(id, props) {
  if (id === 'LD-09') ui.draft.ld09 = null;
  if (!LIVE_DIALOGS[id]) DIALOGS['live:' + id] = () => dialogPlaceholder(id, props || {});
  openDialog('live:' + id, props || {});
}
function dialogPlaceholder(id, props) {
  const e = LIVE_REG.get(id, 'web') || { title: id, file: '', agent: 'web-b' };
  return { title: `${esc(id)} ${esc(e.title)}`, body: `<p class="lead">Dieser Dialog wird ergänzt (${esc(LIVE_REG.AGENTS[e.agent].name)}, <code>${esc(e.file)}</code>).</p><pre class="mono ph-props">${esc(JSON.stringify(props, null, 1))}</pre>`, actions: `<button type="button" class="btn secondary" data-act="close" data-fid="dph:x" data-autofocus>Schliessen</button>` };
}
/** Named entry points (signatures are the contract; props are passed on unchanged). */
const LiveDlg = {
  /** LD-01: plan or edit a recording of one programme */
  planRecording: (programId) => liveDialog('LD-01', { programId }),
  /** LD-02: series recording, from a programme or to edit a rule */
  seriesRecording: (programId, ruleId) => liveDialog('LD-02', { programId, ruleId }),
  /** LD-03: manual or recurring recording; o = { ch, ruleId, start, end } (all optional) */
  manualRecording: (o) => liveDialog('LD-03', o || {}),
  /** LD-04: instant recording in the player */
  instantRecording: (ch) => liveDialog('LD-04', { ch }),
  /** LD-05: reminder for a programme */
  reminder: (programId) => liveDialog('LD-05', { programId }),
  /** LD-06: delete or cancel; o = { kind: 'recording'|'recordings'|'running'|'childRecording'|'occurrence'|'rule'|'series', ids: [], onDone } */
  confirmDelete: (o) => liveDialog('LD-06', o || {}),
  /** LD-07: one warning (id from LIVE.warnings) */
  warning: (warningId) => liveDialog('LD-07', { warningId }),
  /** LD-08: day and time; o = { day, time, onPick(minute) } */
  pickTime: (o) => liveDialog('LD-08', o || {}),
  /** LD-09: filter sheet (phone width); o = { fields: [{ key, label, options: [{v, t}] }], values: {}, onApply(values) } */
  filterSheet: (o) => liveDialog('LD-09', o || {}),
  /** LD-10: switch channel while behind live; o = { to, onConfirm } */
  zapConfirm: (o) => liveDialog('LD-10', o || {}),
  /** LD-11: retention of a recording */
  retention: (recordingId) => liveDialog('LD-11', { recordingId }),
};
const DIALOG_DEMO = {
  'LD-01': () => LiveDlg.planRecording('p-zdf-1215'), 'LD-02': () => LiveDlg.seriesRecording('p-daserste-2655'), 'LD-03': () => LiveDlg.manualRecording({ ch: 'telezueri' }),
  'LD-04': () => LiveDlg.instantRecording('zdf'), 'LD-05': () => LiveDlg.reminder('p-srf1-2645'), 'LD-06': () => LiveDlg.confirmDelete({ kind: 'recording', ids: ['rec-arena'] }),
  'LD-07': () => LiveDlg.warning('w-quota'), 'LD-08': () => LiveDlg.pickTime({ day: 0, time: 1215 }), 'LD-09': () => LiveDlg.filterSheet(chFilterSheet({})),
  'LD-10': () => LiveDlg.zapConfirm({ to: 'zdf' }), 'LD-11': () => LiveDlg.retention('rec-arena'),
};
function dialogHostView(id) {
  const e = LIVE_REG.get(id, 'web');
  if (!e) return notFoundView();
  return { type: 'viewer', nav: 'live', title: `${id} ${e.title}`, after: () => { if (!ui.dialog && ui.lastDemo !== location.hash) { ui.lastDemo = location.hash; setTimeout(() => DIALOG_DEMO[id] && DIALOG_DEMO[id](), 0); } },
    html: `<div class="page live"><div class="head"><p class="eyebrow-live">${esc(id)} · Dialog</p><h1>${esc(e.title)}</h1><p class="sub">Vorschau des Dialogs mit Beispieldaten. Im Ablauf öffnet er sich aus den Seiten (14-oberflaeche.md Abschnitt 5).</p>
    <div class="acts"><button type="button" class="btn primary" data-act="demoDialog" data-id="${esc(id)}" data-fid="dh:open">Dialog öffnen</button><a class="btn ghost" href="#/live/channels" data-fid="dh:ch">Zur Senderübersicht</a></div></div>${LU.sampleNote()}</div>` };
}
ACT.demoDialog = (d) => DIALOG_DEMO[d.id] && DIALOG_DEMO[d.id]();

/* ---------- context menu (popover) ----------
   popMenu(id, triggerHtmlLabel, items) renders a button and, when open, a list under it.
   items: [{ label, act, data: {…}, href, danger, icon }] — act is an ACT name; each item closes the menu. */
function popMenu(id, label, items, o) {
  o = o || {};
  const open = ui.menu && ui.menu.id === id;
  const attrs = (d) => Object.entries(d || {}).map(([k, v]) => `data-${k}="${esc(v)}"`).join(' ');
  return `<span class="lmenu-w"><button type="button" class="btn ${o.cls || 'ghost icon sm'}" data-act="menu" data-id="${esc(id)}" aria-haspopup="menu" aria-expanded="${open}" aria-label="${esc(o.aria || 'Weitere Aktionen')}" data-fid="menu:${esc(id)}">${label}</button>${open ? `<span class="lmenu" role="menu">${items.map((it, k) => it.href
    ? `<a role="menuitem" href="${esc(it.href)}" class="${it.danger ? 'dg' : ''}" data-fid="mi:${esc(id)}:${k}" data-act="menuClose">${it.icon ? icon(it.icon, 16) : ''}${esc(it.label)}</a>`
    : `<button type="button" role="menuitem" class="${it.danger ? 'dg' : ''}" data-act="menuItem" data-do="${esc(it.act)}" ${attrs(it.data)} data-fid="mi:${esc(id)}:${k}">${it.icon ? icon(it.icon, 16) : ''}${esc(it.label)}</button>`).join('')}</span>` : ''}</span>`;
}
ACT.menu = (d, el) => {
  if (ui.menu && ui.menu.id === d.id) { ui.menu = null; render(); return; }
  ui.menu = { id: d.id, opener: el.dataset.fid }; render();
  setTimeout(() => { const f = document.querySelector('.lmenu [role="menuitem"]'); if (f) f.focus(); }, 0);
};
ACT.menuClose = () => { ui.menu = null; };
ACT.menuItem = (d, el, e) => { const op = ui.menu && ui.menu.opener; ui.menu = null; const fn = ACT[d.do]; if (fn) fn(d, el, e); else console.warn('Mockup: keine Aktion', d.do); render(); if (op && !ui.dialog) setTimeout(() => restoreFocus({ fid: op }), 0); };
document.addEventListener('keydown', (e) => {
  const m = e.target.closest && e.target.closest('.lmenu');
  if (!m || !['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(e.key)) return;
  const items = $$('[role="menuitem"]', m); const k = items.indexOf(e.target); e.preventDefault();
  const n = e.key === 'Home' ? 0 : e.key === 'End' ? items.length - 1 : (k + (e.key === 'ArrowDown' ? 1 : -1) + items.length) % items.length;
  items[n].focus();
});

/* ---------- states of the Live-TV pages (mockup switch «Zustände») ---------- */
const liveMode = () => SIM.mode();
/** skeleton rows in final size while «lädt» */
const skelRows = (n, h) => `<div class="stack" aria-busy="true" aria-label="Wird geladen">${Array.from({ length: n || 6 }, () => `<span class="lv-skel" style="height:${h || 64}px"></span>`).join('')}</div>`;
/** error notice with «Erneut versuchen» (sets the mockup back to normal) */
const liveError = (text) => notice('bad', text || LU.TX.loadingErr, 'Der Server hat nicht geantwortet. Deine Einstellungen bleiben erhalten.', `<button type="button" class="btn secondary sm" data-act="liveRetry" data-fid="err:retry">${icon('refresh', 16)}Erneut versuchen</button>`);
ACT.liveRetry = () => { SIM.setMode('normal'); toast('Erneut geladen.'); };
/** empty state with optional action html */
const emptyState = (title, text, acts) => `<div class="empty lv-empty"><b>${title}</b>${text ? `<p>${text}</p>` : ''}${acts ? `<div class="row-gap" style="justify-content:center;margin-top:12px">${acts}</div>` : ''}</div>`;
/** toast with an undo button; undo = ACT name with data */
function toastUndo(html, act, data) { toast(`${html} · <button type="button" class="lnk" data-act="${esc(act)}" ${Object.entries(data || {}).map(([k, v]) => `data-${k}="${esc(v)}"`).join(' ')}>Rückgängig</button>`, 'ok', 6500); }

/* ---------- favourites (used by many pages) ---------- */
ACT.fav = (d) => { const on = SIM.toggleFav(d.ch); const c = SIM.channel(d.ch); if (on) toastUndo(`<b>${esc(c.name)}</b> ist jetzt in Meine Sender`, 'fav', { ch: d.ch }); else toast(`<b>${esc(c.name)}</b> ist nicht mehr in Meine Sender.`); };
/** tune in and open the player (LS-07); list = zapping list of channel ids */
ACT.tune = (d) => { SIM.tune(d.ch, { list: d.list ? d.list.split(',') : undefined }); go('#/live/watch/' + d.ch + (d.list ? '?list=' + encodeURIComponent(d.listName || 'filter') : '')); };

/* ---------- filter sheet of LS-02 (basis of LD-09) ---------- */
function chFilterSheet(values, onApply) {
  return {
    fields: [
      { key: 'group', label: 'Gruppe', options: [{ v: '', t: 'Alle Sender' }, { v: 'fav', t: 'Meine Sender' }, ...LIVE.groups.map((g) => ({ v: g.id, t: g.name }))] },
      { key: 'country', label: 'Land', options: [{ v: '', t: 'Alle Länder' }, ...Object.entries(LIVE.countries).map(([v, t]) => ({ v, t }))] },
      { key: 'lang', label: 'Sprache', options: [{ v: '', t: 'Alle Sprachen' }, ...Object.entries(LIVE.languages).map(([v, t]) => ({ v, t }))] },
      { key: 'category', label: 'Kategorie', options: [{ v: '', t: 'Alle Kategorien' }, ...[...new Set(LIVE.channels.filter(SIM.viewerChannel).map((c) => c.category))].map((v) => ({ v, t: v }))] },
    ],
    values: Object.assign({}, values), onApply: onApply || (() => toast('Filter angewendet (Vorschau).')),
  };
}
/* LD-09 basic form; «Web Zuschauer B» may replace it with registerLiveDialog('LD-09', …) */
registerLiveDialog('LD-09', (p) => {
  if (!ui.draft.ld09) ui.draft.ld09 = Object.assign({}, p.values || {});
  const v = ui.draft.ld09;
  return { title: 'Filter', body: `<div class="stack">${(p.fields || []).map((f) => `<div class="fld"><label for="ld09-${f.key}">${esc(f.label)}</label><select class="sel" id="ld09-${f.key}" data-bind="ui.draft.ld09.${f.key}" data-rerender data-fid="ld09:${f.key}"${f === p.fields[0] ? ' data-autofocus' : ''}>${f.options.map((o) => `<option value="${esc(o.v)}"${String(v[f.key] || '') === String(o.v) ? ' selected' : ''}>${esc(o.t)}</option>`).join('')}</select></div>`).join('')}</div>`,
    actions: `<button type="button" class="btn ghost" data-act="ld09Reset" data-fid="ld09:reset">Zurücksetzen</button><button type="button" class="btn secondary" data-act="close" data-fid="ld09:x">Abbrechen</button><button type="button" class="btn primary" data-act="ld09Apply" data-fid="ld09:ok">Anwenden</button>` };
});
ACT.ld09Reset = () => { const p = ui.dialog.props; (p.fields || []).forEach((f) => { ui.draft.ld09[f.key] = ''; }); renderOverlays(); };
ACT.ld09Apply = () => { const p = ui.dialog.props; const v = Object.assign({}, ui.draft.ld09); ui.draft.ld09 = null; closeDialog(); if (p.onApply) p.onApply(v); };

/* every change of the simulation redraws the page (focus and scroll stay) */
SIM.on((t) => { if (t === 'change') { render(); if (ui.dialog) renderOverlays(); } });
