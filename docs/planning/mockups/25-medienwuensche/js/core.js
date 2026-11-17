/* Mockup core: helpers, icons, router, dialogs, toasts, event delegation. No framework, no build. */
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

/* ---------- transient UI state (never persisted) ---------- */
const ui = { open: {}, dialog: null, drawer: false, toasts: [], draft: {}, busy: {}, hero: { i: 0, paused: false }, q: '', filter: {} };

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
  ROUTES.push({ re, keys, fn });
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
  let view = null;
  for (const r of ROUTES) {
    const m = path.match(r.re);
    if (m) { const params = {}; r.keys.forEach((k, i) => (params[k] = decodeURIComponent(m[i + 1]))); view = r.fn(params, query); break; }
  }
  if (!view) view = { type: 'viewer', nav: '', title: 'Nicht gefunden', html: `<div class="page"><div class="head"><h1>Nicht gefunden</h1><p class="sub">Diese Seite gibt es im Mockup nicht.</p><div class="acts"><a class="btn secondary" href="#/">Zur Startseite</a></div></div></div>` };
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
  document.title = (view.title ? view.title + ' · ' : '') + 'Slouchcat (Mockup)';
  lastPath = path;
  if (view.after) view.after(sc);
  if (window.MOCK && MOCK.render) MOCK.render();
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
  const id = ++toastId;
  ui.toasts.push({ id, html, kind });
  if (ui.toasts.length > 3) ui.toasts.shift();
  renderToasts();
  setTimeout(() => { ui.toasts = ui.toasts.filter((x) => x.id !== id); renderToasts(); }, ms);
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
  if (!a) return;
  if (a.disabled || a.getAttribute('aria-disabled') === 'true') { e.preventDefault(); return; }
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
