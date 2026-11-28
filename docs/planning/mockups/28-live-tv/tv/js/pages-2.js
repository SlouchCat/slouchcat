/* TV, Teil 2: LS-04 Programmzeitschrift (Raster mit Fokus von Sendung zu Sendung, auch als Überlagerung im Player),
   LS-07 Live-Player (Zapping, Senderliste, Zifferneingabe, Info, Timeshift, Aufnahme) und LS-10 Aufnahme-Player.
   14-oberflaeche.md 4.3 (Programm am TV), 4.5 bis 4.13 (Player), 3.10. Alle globalen Namen beginnen mit «tv». */
'use strict';

/* =====================================================================================================
   Programmraster
   ===================================================================================================== */
const TVG_ROWS = 7; const TVG_PXM = 8.8; const TVG_SPAN = 150;
const tvgSnap = (t) => Math.max(0, Math.min(LIVE.EPG_END - TVG_SPAN, Math.floor(t / 30) * 30));
const tvgNew = () => ({ gs: tvgSnap(SIM.now() - 37), top: 0, pos: 0, group: '', anchor: SIM.now() });
const TVG = tvgNew(); const TVGO = tvgNew();
RESETS.push(() => { Object.assign(TVG, tvgNew()); });
const tvgRows = (g) => SIM.gridChannels({ group: g.group || undefined }); // «Viele Sender» adds 2'000 test channels (core)
/** segments (programmes and data gaps) of a channel in the visible window, sorted */
function tvgSegs(g, ch) {
  const lo = g.gs; const hi = g.gs + TVG_SPAN;
  const ps = SIM.programsOf(ch, lo, hi).map((p) => ({ a: p.start, b: p.end, p, key: p.id }));
  const gs = SIM.gapsOf(ch, lo, hi).map(([a, b]) => ({ a, b, p: null, key: 'g' + Math.round(a) }));
  return ps.concat(gs).sort((x, y) => x.a - y.a);
}
const tvgAt = (segs, t) => segs.find((s) => s.a <= t && t < s.b) || segs[segs.length - 1];

function tvgInner(g, px) {
  const chs = tvgRows(g); const n = chs.length; const lo = g.gs; const hi = lo + TVG_SPAN; const t = SIM.now();
  g.top = Math.max(0, Math.min(g.top, Math.max(0, n - TVG_ROWS))); g.pos = Math.max(0, Math.min(g.pos, Math.min(TVG_ROWS, n) - 1));
  const vis = chs.slice(g.top, g.top + TVG_ROWS); const defRow = vis[g.pos];
  const ticks = []; for (let m = Math.ceil(lo / 30) * 30; m < hi; m += 30) ticks.push(`<span class="tvg-tick${m % 60 === 0 ? ' h' : ''}" style="left:${(m - lo) * TVG_PXM}px">${m % 1440 === 0 ? fmt.day(m) : fmt.time(m)}</span>`);
  const rows = vis.map((c, ri) => {
    const segs = tvgSegs(g, c.id); const bad = SIM.chState(c) !== 'ok';
    const defSeg = c === defRow ? tvgAt(segs, Math.max(lo, Math.min(g.anchor, hi - 0.01))) : null;
    const cells = segs.map((s) => {
      const a = Math.max(s.a, lo); const b = Math.min(s.b, hi); const w = (b - a) * TVG_PXM; const p = s.p;
      const cls = !p ? 'gap lv-gap' : p.end <= t ? 'past' : p.start <= t ? 'cur' : '';
      const f = `${px}${c.id}-${s.key}`; const nar = w < 120;
      const inner = p ? `<b>${esc(p.title)}</b><small>${fmt.time(p.start)}${LU.programBadges(p, {})}</small>` : (w > 260 ? `<span class="lv-gap-text">${esc(SIM.epgOf(c) === 'none' ? LU.TX.epgNoneButPlays : LU.TX.epgNone)}</span>` : '');
      const lab = p ? `${c.name}, ${p.title}, ${fmt.range(p.start, p.end)}${p.start <= t && p.end > t ? ', läuft' : ''}` : `${c.name}, ${LU.TX.epgNone}`;
      return `<button type="button" class="tvg-c ${cls}${nar ? ' nar' : ''}${p && p.start < lo ? ' cutl' : ''}${p && p.end > hi ? ' cutr' : ''}" style="left:${(a - lo) * TVG_PXM}px;width:${w - 4}px" data-f="${f}" data-k="${s.key}" data-a="${s.a}" data-ch="${c.id}"${p ? ` data-pid="${p.id}"` : ''} data-px="${px}" data-act="tvgOpen"${s === defSeg ? ' data-default' : ''} aria-label="${esc(lab)}">${inner}</button>`;
    }).join('');
    return `<div class="tvg-row"><button type="button" class="tvg-ch${bad ? ' bad' : ''}" data-f="${px}c-${c.id}" data-ch="${c.id}" data-px="${px}" data-act="tvgChOpen" aria-label="${esc(`${c.no} ${c.name}${bad ? ', gerade nicht erreichbar' : ''}, einschalten`)}"><span class="tvg-no">${c.no}</span>${LU.logo(c, { cls: 'sm' })}<span class="tvg-nm">${esc(c.name)}</span>${bad ? icon('alert', 24) : ''}</button><div class="tvg-cells">${cells}</div></div>`;
  }).join('');
  const nowX = TVG_CHW + (t - lo) * TVG_PXM;
  return `<div class="tvg-axis"><span class="tvg-corner">${esc(fmt.dayRel(lo))}</span><div class="tvg-ticks">${ticks.join('')}</div></div>
    <div class="tvg-body">${rows}${t >= lo && t < hi ? `<i class="tvg-now" style="left:${nowX}px" aria-hidden="true"></i>` : ''}</div>
    <p class="tvg-pg">${n ? `Sender ${g.top + 1}–${Math.min(n, g.top + TVG_ROWS)} von ${n}` : ''}</p>`;
}
const TVG_CHW = 250;
function tvgSkel() { return `<div class="tvg-axis"><span class="tvg-corner"></span><div class="tvg-ticks"></div></div><div class="tvg-body">${Array.from({ length: TVG_ROWS }, () => `<div class="tvg-row"><span class="tvg-ch"><span class="lv-skel tvg-sk"></span></span><div class="tvg-cells"><span class="lv-skel tvg-sk w"></span></div></div>`).join('')}</div>`; }

function tvgPreviewHtml(el) {
  const c = SIM.channel(el.dataset.ch); if (!c) return '';
  const p = el.dataset.pid ? SIM.program(el.dataset.pid) : null; const st = SIM.chState(c);
  if (!p) return `<div class="tvg-pv-h"><b>${esc(c.name)}</b><span>${esc(LU.TX.epgNone)}${SIM.epgOf(c) === 'none' ? ' · Sender läuft trotzdem' : ''}</span></div><p class="tvg-pv-d">${SIM.rights().recordings ? 'OK schaltet ein, über das Senderprogramm lässt sich eine Zeit aufnehmen.' : 'OK schaltet ein.'}</p>`;
  const s = tvStat(p);
  return `<div class="tvg-pv-h"><b>${esc(p.title)}${p.sub ? ` · «${esc(p.sub)}»` : ''}</b><span>${fmt.time(SIM.now())}</span></div>
    <p class="tvg-pv-m">${esc(c.name)} · ${fmt.range(p.start, p.end)} · ${esc(p.genre)} · ${s === 'now' ? `läuft, noch ${fmt.left(p)} Min.` : s === 'past' ? 'vorbei' : `${fmt.dayRel(p.start)}, ${fmt.dur(p.end - p.start)}`}${st !== 'ok' && s === 'now' ? ' · gerade nicht erreichbar' : ''}${LU.programBadges(p, { text: true })}</p>
    <p class="tvg-pv-d">${esc(p.desc || '')}</p>`;
}
/** focus state of a grid follows the focused element (entry from the toolbar or the rail) */
function tvgFocus(g, el) {
  const chs = tvgRows(g); const i = chs.findIndex((c) => c.id === el.dataset.ch); if (i < 0) return;
  g.pos = Math.max(0, Math.min(TVG_ROWS - 1, i - g.top));
  if (el.dataset.k != null) g.anchor = Math.max(Number(el.dataset.a), g.gs);
}
/** draw the grid again and focus the cell (ch, key) */
function tvgDraw(g, px, ch, key, noFocus) {
  const root = $('#' + px + 'grid'); if (!root) return;
  root.innerHTML = tvgInner(g, px);
  if (noFocus) return;
  const el = byF(key == null ? `${px}c-${ch}` : `${px}${ch}-${key}`, root) || $('.tvg-c', root);
  if (el) { focus(el, { instant: true, noReveal: true }); const pv = $('#' + px + 'prev'); if (pv) pv.innerHTML = tvgPreviewHtml(el); }
}
/** arrow and paging keys inside a grid; true when handled. g = state, px = id prefix, edge = what Up at the top row does */
function tvgKey(g, px, name) {
  const el = F.el; if (!el || !el.closest || !el.closest(`[data-row="${px}grid"]`)) return false;
  const chs = tvgRows(g); const n = chs.length; if (!n) return false;
  const i = chs.findIndex((c) => c.id === el.dataset.ch); if (i < 0) return false;
  const isCh = el.classList.contains('tvg-ch'); const lo = g.gs; const hi = lo + TVG_SPAN;
  const showRow = (ni) => { if (ni < g.top) g.top = ni; else if (ni >= g.top + TVG_ROWS) g.top = ni - TVG_ROWS + 1; g.pos = ni - g.top; };
  const toRow = (ni) => {
    ni = Math.max(0, Math.min(n - 1, ni)); showRow(ni); const c = chs[ni];
    if (isCh) { tvgDraw(g, px, c.id, null); return; }
    const sg = tvgAt(tvgSegs(g, c.id), Math.max(lo, Math.min(g.anchor, hi - 0.01))); tvgDraw(g, px, c.id, sg.key);
  };
  if (name === 'down') { if (i < n - 1) toRow(i + 1); else bump(el, 'down'); return true; }
  if (name === 'up') { if (i > 0) { toRow(i - 1); return true; } return px === 'go-'; }
  if (name === 'ch_down') { toRow(i + TVG_ROWS); return true; }
  if (name === 'ch_up') { toRow(i - TVG_ROWS); return true; }
  const segs = tvgSegs(g, el.dataset.ch);
  if (isCh) { if (name === 'right') { const sg = tvgAt(segs, Math.max(lo, g.anchor)); tvgDraw(g, px, el.dataset.ch, sg.key); return true; } return name === 'left' && px === 'go-'; }
  const k = segs.findIndex((s) => s.key === el.dataset.k); if (k < 0) return false; const s = segs[k];
  const land = (ng, t) => { g.gs = ng; const ss = tvgSegs(g, el.dataset.ch); const sg = tvgAt(ss, t); g.anchor = Math.max(sg.a, g.gs); tvgDraw(g, px, el.dataset.ch, sg.key); };
  if (name === 'right') {
    if (k < segs.length - 1) { const sg = segs[k + 1]; g.anchor = Math.max(sg.a, lo); tvgDraw(g, px, el.dataset.ch, sg.key); return true; }
    const T = s.p ? s.p.end : s.b; if (T >= LIVE.EPG_END && hi >= LIVE.EPG_END) { bump(el, 'right'); return true; }
    land(tvgSnap(Math.max(lo + 30, Math.ceil((T - TVG_SPAN + 60) / 30) * 30)), Math.min(T, LIVE.EPG_END - 1)); return true;
  }
  if (name === 'left') {
    if (k > 0) { const sg = segs[k - 1]; g.anchor = Math.max(sg.a, lo); tvgDraw(g, px, el.dataset.ch, sg.key); return true; }
    const start = s.p ? s.p.start : s.a;
    if (start < lo && lo > 0) { land(tvgSnap(Math.min(lo - 30, start)), lo); return true; }
    tvgDraw(g, px, el.dataset.ch, null); return true;
  }
  return false;
}
/** Jump to a channel number inside the grid */
function tvgJump(g, px, c) {
  const chs = tvgRows(g); const i = chs.findIndex((x) => x.id === c.id); if (i < 0) { toast(`${c.name} gehört nicht zur gewählten Gruppe`); return; }
  g.top = Math.max(0, Math.min(i - 3, chs.length - TVG_ROWS)); g.pos = i - g.top;
  const sg = tvgAt(tvgSegs(g, c.id), Math.max(g.gs, Math.min(g.anchor, g.gs + TVG_SPAN - 0.01))); tvgDraw(g, px, c.id, sg.key);
}
function tvgNowGs(g) { g.gs = tvgSnap(SIM.now() - 37); g.anchor = SIM.now(); }
/** keys that the page and the overlay share: Play on a running programme switches on, Info and Record */
function tvgCommonKey(g, px, name, onTune, onRec) {
  const el = F.el; const inGrid = el && el.closest && el.closest(`[data-row="${px}grid"]`);
  if (/^d\d$/.test(name)) { liveNumber(name, (c) => tvgJump(g, px, c)); return true; }
  if (name === 'ok' && liveNumberActive()) { liveNumberCommit(); return true; }
  if (!inGrid || !el.dataset.ch) return false;
  const p = el.dataset.pid ? SIM.program(el.dataset.pid) : null;
  if (name === 'play') { if (p && tvStat(p) === 'now' || (!p && !el.dataset.k)) onTune(el.dataset.ch); else if (p) toast('Diese Sendung läuft noch nicht. OK zeigt die Details'); else onTune(el.dataset.ch); return true; }
  if (name === 'info') { if (p) tvProgramMenu(p.id); else toast('Keine Programmdaten'); return true; }
  if (name === 'rec') { if (!SIM.rights().recordings) { /* without the right nothing about recordings (14, 0.1) */ } else if (p && tvStat(p) !== 'past') LiveDlg.planRecording(p.id); else toast('Aufnehmen geht bei laufenden und künftigen Sendungen'); return true; }
  return false;
}
function tvgPreviewBoxHtml(px) { return `<div class="tvg-pv" id="${px}prev" aria-live="polite"></div>`; }

/* ---------- LS-04 ---------- */
route('/live/guide', () => {
  const g = TVG; const mode = SIM.mode(); const chs = tvgRows(g);
  const groupName = g.group === 'fav' ? 'Meine Sender' : g.group ? (LIVE.groups.find((x) => x.id === g.group) || { name: g.group }).name : 'Alle Sender';
  const tools = `<div class="tvg-tools"><button type="button" class="tb ghost sm" data-act="tvgDay" data-f="gt-day">${icon('calendar', 28)}<span>${esc(fmt.dayRel(g.gs))}</span></button>
    <button type="button" class="tb ghost sm" data-act="tvgTime" data-f="gt-time">${icon('clock', 28)}<span>ab ${fmt.time(g.gs)}</span></button>
    <button type="button" class="tb ghost sm" data-act="tvgGroup" data-f="gt-group">${icon('list', 28)}<span>${esc(groupName)}</span></button>
    <button type="button" class="tb ghost sm" data-act="tvgShift" data-d="-120" data-f="gt-back" aria-label="2 Stunden zurück">${icon('left', 28)}<span>2 h</span></button>
    <button type="button" class="tb ghost sm" data-act="tvgShift" data-d="120" data-f="gt-fwd" aria-label="2 Stunden vor"><span>2 h</span>${icon('right', 28)}</button>
    <button type="button" class="tb sm" data-act="tvgNow" data-f="gt-now">${icon('live', 28)}<span>Jetzt</span></button>
    <button type="button" class="tb ghost sm" data-act="go" data-to="#/live/guide/search" data-f="gt-search">${icon('search', 28)}<span>Suche</span></button></div>`;
  let body;
  if (mode === 'loading') body = tvgSkel();
  else if (!chs.length) body = tvNote('info', g.group ? 'Keine Sender in dieser Gruppe' : 'Noch keine Sender', g.group ? 'Wähle eine andere Gruppe.' : esc(LU.TX.empty));
  else body = `${mode === 'error' ? `<div class="tvg-err">${tvNote('alert', 'Die Programmdaten konnten nicht aktualisiert werden.', 'Angezeigt wird der Stand von heute 06:00. Fernsehen geht trotzdem.', 'bad')}<button type="button" class="tb primary sm" data-act="ltvRetry" data-f="tvp-retry">${icon('refresh', 28)}<span>Erneut versuchen</span></button></div>` : ''}${SIM.flags().noEpg ? `<div class="tvg-err">${tvNote('info', 'Für die Sender gibt es gerade keine Programmdaten.', 'Fernsehen geht trotzdem.')}</div>` : ''}<div data-row="g-grid" class="tvg" id="g-grid">${tvgInner(g, 'g-')}</div>`;
  return {
    nav: '', html: livePage({ tab: 'guide', html: `<div class="tvp-guide">${chs.length && mode !== 'loading' ? tvgPreviewBoxHtml('g-') : '<div class="tvg-pv"></div>'}${tools}${body}</div>` }),
    onFocus(el) { if (el.closest('#g-grid')) { tvgFocus(g, el); const pv = $('#g-prev'); if (pv) pv.innerHTML = tvgPreviewHtml(el); } },
    key(name) {
      if (tvgKey(g, 'g-', name)) return true;
      if (tvKeyGuard(name)) return true;
      return tvgCommonKey(g, 'g-', name, (ch) => liveTune(ch, chs.map((c) => c.id)), null);
    },
  };
});
SIM.on((t) => { if (t === 'minute' && $('#g-grid') && !OV.length) { const el = F.el && F.el.closest('#g-grid') ? F.el : null; const ch = el && el.dataset.ch; const k = el && el.dataset.k; tvgDraw(TVG, 'g-', ch, k, !el); } });

ACT.tvgOpen = (d) => { if (d.px === 'go-') return tvgoPick(d); if (d.pid) tvProgramMenu(d.pid); else go('#/live/guide/channel/' + d.ch); };
ACT.tvgChOpen = (d) => { if (d.px === 'go-') return tvgoPick(d); liveTune(d.ch, tvgRows(TVG).map((c) => c.id)); };
ACT.tvgShift = (d) => { const g = TVG; const n = tvgSnap(g.gs + Number(d.d)); if (n === g.gs) { toast(Number(d.d) < 0 ? 'Weiter zurück gibt es nichts' : 'Das Programm reicht nicht weiter'); return; } g.gs = n; g.anchor = n; render(); };
ACT.tvgNow = () => { tvgNowGs(TVG); TVG.top = 0; TVG.pos = 0; render(); toast('Jetzt'); };
function tvgMenu(title, opts, cur, act) {
  menu(`<h2>${title}</h2><div class="mlist">${opts.map(([v, t, s], i) => `<button type="button" class="mi" role="menuitemradio" aria-checked="${String(cur) === String(v)}" data-act="${act}" data-v="${esc(v)}" data-f="gm-${i}"${String(cur) === String(v) ? ' data-default' : ''}><span class="mi-tx"><b>${esc(t)}</b>${s ? `<small>${esc(s)}</small>` : ''}</span>${String(cur) === String(v) ? icon('check', 34) : ''}</button>`).join('')}</div>`, { onClose: () => render() });
}
ACT.tvgDay = () => {
  const today = Math.floor(SIM.now() / 1440); const days = []; for (let d = today; d < LIVE.EPG_END / 1440; d++) days.push([d, d === today ? 'Heute' : d === today + 1 ? 'Morgen' : fmt.day(d * 1440), fmt.dayLong(d * 1440)]);
  tvgMenu('Tag', days, Math.floor(TVG.gs / 1440), 'tvgSetDay');
};
ACT.tvgSetDay = (d) => { TVG.gs = tvgSnap(Number(d.v) * 1440 + (TVG.gs % 1440)); TVG.anchor = TVG.gs; closeMenu(); };
const TVG_TIMES = [['now', 'Jetzt', ''], [360, 'Morgen', '06:00'], [720, 'Mittag', '12:00'], [1080, 'Vorabend', '18:00'], [1215, 'Hauptabend', '20:15'], [1320, 'Spätabend', '22:00'], [0, 'Nacht', '00:00'], ['other', 'Andere Uhrzeit …', 'Tag und Uhrzeit wählen']];
ACT.tvgTime = () => tvgMenu('Uhrzeit', TVG_TIMES, '', 'tvgSetTime');
ACT.tvgSetTime = (d) => {
  if (d.v === 'now') { tvgNowGs(TVG); closeMenu(); return; }
  if (d.v === 'other') { closeMenu(); LiveDlg.pickTime({ day: Math.floor(TVG.gs / 1440), time: TVG.gs % 1440, onPick: (o) => { TVG.gs = tvgSnap(o.day * 1440 + o.time); TVG.anchor = TVG.gs; render(); } }); return; }
  TVG.gs = tvgSnap(Math.floor(TVG.gs / 1440) * 1440 + Number(d.v)); TVG.anchor = TVG.gs; closeMenu();
};
ACT.tvgGroup = () => tvgMenu('Gruppe', [['', 'Alle Sender', ''], ['fav', 'Meine Sender', `${SIM.favorites().length} Sender`], ...LIVE.groups.map((x) => [x.id, x.name, ''])], TVG.group, 'tvgSetGroup');
ACT.tvgSetGroup = (d) => { TVG.group = d.v; TVG.top = 0; TVG.pos = 0; closeMenu(); };

/* ---------- guide as overlay in the player (4.9): a running programme switches the channel, a future one opens details ---------- */
function tvGuideOverlay() {
  const g = TVGO; Object.assign(g, tvgNew()); const chs = tvgRows(g); const cur = PL.id;
  const i = Math.max(0, chs.findIndex((c) => c.id === cur)); g.top = Math.max(0, Math.min(i - 3, chs.length - TVG_ROWS)); g.pos = i - g.top;
  if (!chs.length) { toast('Keine Sender'); return; }
  PL.bar = false; PL.list = false; PL.info = false; PL.infoFocus = false; PL.guide = true; render();
  menu(`<div class="tvp-gov"><h2>Programm</h2>${tvgPreviewBoxHtml('go-')}<div class="tvg" data-row="go-grid" id="go-grid">${tvgInner(g, 'go-')}</div><p class="tvp-hint">OK: laufende Sendung einschalten, künftige zeigt Details · Zurück oder Guide schliesst</p></div>`, {
    dialog: true, cls: 'tvp-guideov',
    onClose: () => { PL.guide = false; },
    onKey(name) {
      if (name === 'guide') { closeMenu(); return true; }
      if (tvgKey(g, 'go-', name)) return true;
      if (name === 'left' || name === 'right' || name === 'up' || name === 'down') return true;
      return tvgCommonKey(g, 'go-', name, (ch) => { closeMenu(); tvpRequest(ch, 'pick'); }, null);
    },
  });
  const el = $('.tvg-c[data-default]', $('#go-grid')) || $('.tvg-c', $('#go-grid')); if (el) { focus(el, { instant: true }); $('#go-prev').innerHTML = tvgPreviewHtml(el); }
}
function tvgoPick(d) {
  if (d.pid) { const p = SIM.program(d.pid); if (p && tvStat(p) === 'now') { closeMenu(); tvpRequest(d.ch, 'pick'); } else tvProgramMenu(d.pid); }
  else { closeMenu(); tvpRequest(d.ch, 'pick'); }
}
/* the overlay keeps its preview in step with the focus */
document.addEventListener('focusin', (e) => { const el = e.target.closest && e.target.closest('#go-grid [data-f]'); if (el && $('#go-prev')) { const g = TVGO; const i = tvgRows(g).findIndex((c) => c.id === el.dataset.ch); if (i >= 0) g.pos = i - g.top; $('#go-prev').innerHTML = tvgPreviewHtml(el); } });

/* =====================================================================================================
   LS-07 Live-Player
   ===================================================================================================== */
const PL = { id: null, fault: '', t0: 0, info: false, infoUntil: 0, infoFocus: false, bar: false, barUntil: 0, list: false, guide: false, force: '', pend: null, leaving: false, fb: false, tsoff: false, shown: '', step: 0, lastSeek: 0, listKind: '' };
RESETS.push(() => { Object.assign(PL, { id: null, info: false, infoFocus: false, bar: false, list: false, guide: false, force: '', pend: null, leaving: false, fb: false, tsoff: false, shown: '', listKind: '' }); });
/** RM-16: a fault switched in the mockup bar acts on the running channel; unreachable and slow start load
    again, leaving an error loads again, the others (stall, fallback, limit, busy, not playable) show at once */
const tvpFaultRestart = (was, now) => ['unreachable', 'slow'].includes(now) || ['limit', 'busy', 'busyPerm', 'notPlayable', 'unreachable'].includes(was);
const tvpIsPlayer = () => /^\/live\/watch\//.test(curPath());
const tvpPl = () => SIM.player();
const tvpInput = (id) => LIVE.inputs.some((i) => i.ch === id && i.state === 'running');
/** phase of the switching: { k: 'loading'|'slow'|'ok'|'error', spin } */
/* phases and faults come from the core (LIVE_UI.playerPhase: the same thresholds in all clients, mockup states
   in the bar «Zustände» → Player) */
const TVP_ERRMAP = { notPlayable: 'notplayable' };
function tvpPhase(c) {
  const ph = LU.playerPhase(Date.now() - PL.t0, c, { hot: tvpInput(c.id) });
  if (ph.fallback && !PL.fb) { PL.fb = true; PL.info = true; PL.infoUntil = Date.now() + 5000; setTimeout(() => toast('Auf eine Ersatzquelle umgeschaltet.'), 0); }
  if (!ph.fallback) PL.fb = false;
  if (ph.k === 'error') return { k: 'forced', spin: false, err: TVP_ERRMAP[ph.err] || ph.err };
  return ph;
}
const tvpFmtOff = (min) => fmt.offset(min);
const tvpShiftMin = () => { const p = tvpPl(); return p ? p.offset : 0; };

function tvpErrHtml(c, kind) {
  const E = {
    unreachable: [`${c.name} ist gerade nicht erreichbar. Slouchcat versucht es weiter.`, 'retry', 'other'],
    limit: ['Auf deinem Konto laufen schon 3 Wiedergaben gleichzeitig.', 'back'],
    revoked: ['Live-TV ist für dieses Profil nicht mehr freigegeben.', 'home'],
    busy: ['Der Server ist gerade ausgelastet. Versuche es gleich nochmals.', 'retry'],
    busyPerm: [LU.PLAYER_ERR.busyPerm(c), 'retry'],
    noConvert: [LU.PLAYER_ERR.noConvert(c), 'other'],
    gone: ['Diesen Sender gibt es nicht mehr.', 'other'],
    notplayable: ['Dieser Sender lässt sich auf diesem Gerät nicht abspielen.', 'other'],
  }[kind];
  const left = 20 - Math.floor(((Date.now() - PL.t0) / 1000) % 20);
  const b = { retry: tvBtn({ f: 'er-retry', ic: 'refresh', label: 'Erneut versuchen', act: 'tvpRetry', cls: 'primary', def: true }), other: tvBtn({ f: 'er-other', ic: 'list', label: 'Anderer Sender', act: 'tvpListOpen', cls: 'ghost' }), back: tvBtn({ f: 'er-back', ic: 'back', label: 'Zurück', act: 'back', cls: 'primary', def: true }), home: tvBtn({ f: 'er-home', ic: 'home', label: 'Zur Startseite', act: 'tvpHome', cls: 'primary', def: true }) };
  const bs = E.slice(1).map((k) => b[k]); if (E[1] === 'other') bs[0] = bs[0].replace(' ghost', ' primary').replace('data-f="er-other"', 'data-f="er-other" data-default');
  return `<div class="tvp-err" role="alert">${LU.logo(c, { cls: 'big', label: true })}<h2>${esc(c.name)}</h2><p>${esc(E[0])}</p>${kind === 'unreachable' ? `<p class="tvp-hint">Nächster Versuch in ${left} s</p>` : ''}<div class="ltv-acts">${bs.join('')}</div></div>`;
}
function tvpBannerHtml(c, pl) {
  const nn = SIM.nowNext(c.id); const st = SIM.chState(c); const r = SIM.rights();
  const rec = r.recordings && SIM.runningOn(c.id);
  const pb = nn.now ? LU.programBadges(nn.now, { text: true }) : '';
  const badges = (rec && !pb.includes('lv-badge recording') ? LU.badge('recording', { text: true }) : '') + pb + (PL.fb ? `<span class="lv-badge"><span class="sym">↻</span><span>Ersatzquelle</span></span>` : '');
  const blk = (f, lbl, p, nowP) => {
    const inner = p ? `<span class="tvp-bn-l">${lbl}</span><b>${fmt.range(p.start, p.end)} ${esc(p.title)}</b>${nowP ? LU.progress(p.start, p.end) : ''}` : `<span class="tvp-bn-l">${lbl}</span><b class="none">${esc(LU.TX.epgNone)}</b>`;
    return PL.infoFocus && p ? `<button type="button" class="tvp-bn-b" data-act="tvDetail" data-pid="${p.id}" data-f="${f}">${inner}</button>` : `<div class="tvp-bn-b">${inner}</div>`;
  };
  const after = nn.next && nn.now && nn.next.start === nn.now.end ? nn.next : (!nn.now ? nn.next : null);
  return `<div class="tvp-banner" role="status"><div class="tvp-bn-h"><b class="tvp-bn-no">${c.no}</b>${LU.logo(c)}<b class="tvp-bn-nm">${esc(c.name)}</b><span class="tvp-bn-bd">${badges}</span>${st !== 'ok' ? `<span class="tvp-bad">${icon('alert', 26)}${esc(LU.TX.unavailable)}</span>` : ''}</div>
    <div class="tvp-bn-g">${blk('pl-now', 'Jetzt', nn.now, true)}${blk('pl-next', 'Danach', after, false)}</div></div>`;
}
function tvpBarHtml(c, pl) {
  const r = SIM.rights(); const ts = r.timeshift; const t = SIM.now();
  const off = pl ? pl.offset : 0; const max = pl ? pl.max : 0; const live = !pl || pl.live; const paused = pl && pl.paused;
  const nn = SIM.nowNext(c.id).now; const stub = ts && nn && pl && nn.start < pl.joinedAt;
  const run = r.recordings && SIM.runningOn(c.id); const rs = r.recordings && nn ? SIM.recState(nn.id) : null;
  const b = (f, ic, label, act, o) => `<button type="button" class="tvp-b${o && o.cls ? ' ' + o.cls : ''}" data-act="${act}" data-f="${f}"${o && o.def ? ' data-default' : ''}${o && o.attr ? ' ' + o.attr : ''} aria-label="${esc(o && o.aria || label || '')}">${icon(ic, 44)}${label ? `<span>${esc(label)}</span>` : ''}</button>`;
  const pill = live ? `<span class="tvp-pill live"><i></i>${LU.TX.live}</span>` : `<button type="button" class="tvp-pill behind" data-act="tvpLive" data-f="pl-live"><i></i>${paused ? `Pausiert · ${tvpFmtOff(off)}` : tvpFmtOff(off)} · Zurück zu Live</button>`;
  const recB = !r.recordings ? '' : run ? b('pl-rec', 'record', `Nimmt auf · ${fmt.dur(Math.max(1, Math.round(t - run.start)))}`, 'tvpRec', { cls: 'recon', aria: 'Nimmt auf, Menü öffnen' }) : rs && rs !== 'recording' ? b('pl-rec', 'record', 'Geplant', 'tvpRec', { cls: 'sched' }) : b('pl-rec', 'record', 'Aufnehmen', 'tvpRec');
  const seek = ts ? `<div class="tvp-seek" data-f="pl-seek" role="slider" aria-label="Zeitversatz" aria-valuemin="0" aria-valuemax="${Math.round(max * 60)}" aria-valuenow="${Math.round((max - off) * 60)}" aria-valuetext="${off < 0.05 ? 'Live' : tvpFmtOff(off) + ' hinter Live'}">
      <span class="tvp-sl"><span>seit ${pl ? fmt.time(pl.joinedAt) : fmt.time(t)}${max >= 119.9 ? ' (höchstens 2 Std.)' : ''}</span><span>Live ${fmt.time(t)}</span></span>
      <span class="tvp-trk" id="tvp-trk">${stub ? '<i class="tvp-stub lv-gap"></i>' : ''}<i class="tvp-buf" id="tvp-buf"></i>${tvpMarks(c, pl)}<i class="tvp-pos" id="tvp-pos"><em id="tvp-bub"></em></i></span>
      <span class="tvp-sr">${max < 0.5 ? 'Noch nichts zum Zurückspulen: die Leiste beginnt bei deinem Einschalten.' : stub ? `Vor ${fmt.time(pl.joinedAt)} nicht verfügbar · ←/→ spult` : '←/→ spult'}</span></div>`
    : `<div class="tvp-seek ro"><span class="tvp-trk">${nn ? LU.progress(nn.start, nn.end, { label: 'Fortschritt der Sendung' }) : ''}</span><span class="tvp-sr">${nn ? `${esc(nn.title)} · noch ${fmt.left(nn)} Min.` : ''}</span></div>`;
  return `<div class="tvp-bar">${seek}<div class="tvp-btns">${b('pl-prev', 'prev', '', 'tvpPrev', { aria: 'Vorheriger Sender' })}
    ${ts ? b('pl-back', 'rew', '10 s', 'tvpBack10', { aria: '10 Sekunden zurück' }) + b('pl-pause', paused ? 'play' : 'pause', '', 'tvpPause', { def: true, aria: paused ? 'Weiter' : 'Anhalten' }) + b('pl-fwd', 'fwd', '10 s', 'tvpFwd10', { aria: '10 Sekunden vor' }) : ''}
    ${b('pl-next', 'next', '', 'tvpNext', { aria: 'Nächster Sender' })}<span class="tvp-sp"></span>${pill}<span class="tvp-sp"></span>
    ${recB}${b('pl-list', 'list', 'Sender', 'tvpListOpen', { def: !ts, aria: 'Senderliste' })}${b('pl-guide', 'grid', 'Programm', 'tvpGuide')}${b('pl-audio', 'audio', 'Ton', 'tvpAudio', { aria: 'Ton, Untertitel und Qualität' })}${b('pl-info', 'info', '', 'tvpInfo', { aria: 'Info' })}</div></div>`;
}
function tvpMarks(c, pl) {
  if (!pl || pl.max < 1) return '';
  const t = SIM.now(); const lo = t - pl.max;
  return SIM.programsOf(c.id, lo, t).filter((p) => p.start > lo + 0.5 && p.start < t - 0.5).map((p) => `<i class="tvp-mk" style="left:${((p.start - lo) / pl.max * 100).toFixed(2)}%" title="${esc(`ab ${fmt.time(p.start)} ${p.title}`)}"></i>`).join('');
}
function tvpListIds(pl) { return (pl && pl.list && pl.list.length ? pl.list : tvAllIds()); }
function tvpListName(pl) {
  const ids = tvpListIds(pl).join(','); if (ids === SIM.favorites().join(',') && ids) return 'Meine Sender';
  if (ids === tvAllIds().join(',')) return 'Alle Sender';
  const gr = LIVE.groups.find((g) => SIM.channels({ group: g.id }).map((c) => c.id).join(',') === ids); return gr ? gr.name : 'Eigene Auswahl';
}
function tvpListHtml(pl) {
  const ids = tvpListIds(pl); const cur = PL.id;
  return `<aside class="tvp-zap" aria-label="Senderliste"><h2>Senderliste</h2><button type="button" class="tvp-zsel" data-act="tvpListCycle" data-f="zl-sel" aria-label="Liste wechseln, jetzt ${esc(tvpListName(pl))}"><span>Liste: <b>${esc(tvpListName(pl))}</b></span>${icon('sort', 30)}</button>
    <div class="tvp-zrows" data-scroll>${ids.length ? ids.map((id) => { const c = SIM.channel(id); if (!c) return ''; const nn = SIM.nowNext(id); const n = nn.now; const bad = SIM.chState(c) !== 'ok';
    return `<button type="button" class="tvp-zr${id === cur ? ' cur' : ''}" data-act="tvpZapPick" data-ch="${id}" data-f="zr-${id}"${id === cur ? ' data-default aria-current="true"' : ''} aria-label="${esc(`${c.no} ${c.name}${n ? ', ' + n.title : ''}`)}"><span class="tvp-zno">${c.no}</span>${LU.logo(c, { cls: 'sm' })}<span class="tvp-znm"><b>${esc(c.name)}${SIM.isFav(id) ? `<span class="tvp-fv" aria-label="in Meine Sender">${icon('heartOn', 22)}</span>` : ''}</b><small>${bad ? esc(LU.TX.unavailable) : n ? esc(n.title) : esc(LU.TX.epgNone)}</small>${n && !bad ? LU.progress(n.start, n.end) : ''}</span></button>`; }).join('') : '<p class="tvp-hint">Keine Sender in dieser Liste.</p>'}</div></aside>`;
}

route('/live/watch/:id', (pm) => {
  if (!/^\/live\/watch\//.test(lastPath || '')) PL.leaving = false;
  if (SIM.mode() === 'empty') return { nav: '', html: `<div class="page soon"><span class="soon-ic">${icon('live', 88)}</span><h1>Noch keine Sender</h1><p>${esc(LU.TX.empty)}</p><button type="button" class="tb primary" data-act="back" data-f="pl-empty" data-default>${icon('back', 32)}<span>Zurück</span></button></div>` };
  const c = SIM.channel(pm.id);
  if (!c || SIM.chState(c) === 'gone') {
    const g = { id: pm.id, name: 'Dieser Sender', no: '', slug: 'x' };
    return { nav: '', full: true, html: `<div class="tvp-pl">${tvpErrHtml(c || g, 'gone')}</div>` };
  }
  const now = Date.now(); const fault = SIM.flags().fault;
  if (PL.id !== pm.id) Object.assign(PL, { id: pm.id, t0: now, info: true, infoUntil: now + 5000, infoFocus: false, bar: false, list: false, guide: false, force: '', pend: null, fb: false, fault });
  else if (PL.fault !== fault) { if (tvpFaultRestart(PL.fault, fault)) { PL.t0 = now; PL.bar = false; PL.list = false; } PL.fault = fault; PL.fb = false; }
  const pl = SIM.player(); const synced = !!pl && pl.ch === pm.id; const ph = tvpPhase(c);
  PL.shown = ph.k + (ph.spin ? '+' : '') + (PL.bar ? 'b' : '') + (PL.info ? 'i' : '');
  const err = ph.k === 'forced' ? ph.err : null;
  const paused = !!(pl && pl.paused); const r = SIM.rights();
  const run = r.recordings && SIM.runningOn(c.id);
  const showVid = !err && (ph.k === 'ok' || ph.k === 'forced');
  const loadTxt = ph.k === 'slow' ? `Dauert länger als üblich …${ph.why ? ` ${esc(ph.why)}` : ''}` : `${esc(c.name)} wird geladen`;
  const html = `<div class="tvp-pl${showVid ? '' : ' dark'}" data-phase="${ph.k}">
    <div class="tvp-vid">${showVid ? LU.fakeVideo(c, { paused }) : ''}</div><i class="tvp-scrim" aria-hidden="true"></i>
    ${paused && showVid ? `<span class="tvp-pause" aria-hidden="true">${icon('pause', 120)}</span>` : ''}
    ${!showVid && !err ? `<div class="tvp-load" role="status">${ph.spin ? `<span class="spin"></span>` : ''}<p>${ph.spin ? loadTxt : ''}</p></div>` : ''}
    ${showVid && ph.stall ? `<div class="tvp-load" role="status"><span class="spin"></span><p>Verbindung wird wiederhergestellt …</p></div>` : ''}
    ${err ? tvpErrHtml(c, err) : ''}
    ${run && showVid ? `<span class="tvp-recind">${LU.badge('recording', { text: true, label: `Nimmt auf · ${fmt.dur(Math.max(1, Math.round(SIM.now() - run.start)))}` })}</span>` : ''}
    ${PL.info && !err ? tvpBannerHtml(c, pl) : ''}
    ${PL.bar && !err && showVid ? tvpBarHtml(c, pl) : ''}
    ${PL.list ? tvpListHtml(pl) : ''}
    <p class="tvp-sample">${esc(LU.TX.sample)}</p></div>`;
  return {
    nav: '', full: true, html, key: tvpKey, back: tvpBack,
    after() {
      if (!synced && !PL.leaving) setTimeout(() => { const cur = SIM.player(); if (curPath() === '/live/watch/' + pm.id && (!cur || cur.ch !== pm.id)) SIM.tune(pm.id, { list: pl ? pl.list : undefined }); }, 0);
      tvpPaint();
      if (PL.list && !F.el) { const el = byF('zr-' + PL.id) || byF('zl-sel'); if (el) focus(el, { instant: true }); }
    },
  };
});
/** bar geometry: marker position and bubble, updated without redrawing */
function tvpPaint() {
  const pl = tvpPl(); const pos = $('#tvp-pos'); const buf = $('#tvp-buf'); if (!pos || !pl) return;
  const f = pl.max > 0.02 ? (pl.max - pl.offset) / pl.max : 1; pos.style.left = (f * 100).toFixed(2) + '%'; buf.style.width = (f * 100).toFixed(2) + '%';
  const bub = $('#tvp-bub'); if (bub) bub.textContent = fmt.time(pl.position);
}
setInterval(() => {
  if (!tvpIsPlayer() || OV.length) return;
  const c = SIM.channel(PL.id); if (!c) return; const now = Date.now(); const pl = tvpPl();
  const ph = tvpPhase(c); const sig = ph.k + (ph.spin ? '+' : '') + (PL.bar ? 'b' : '') + (PL.info ? 'i' : '');
  let redo = sig !== PL.shown;
  if (PL.info && !PL.infoFocus && now > PL.infoUntil && !PL.list) { PL.info = false; redo = true; }
  if (PL.bar && now > PL.barUntil && !(pl && pl.paused)) { PL.bar = false; redo = true; }
  if (PL.pend && now > PL.pend.until) PL.pend = null;
  if (ph.err === 'unreachable') { const w = $('.tvp-err .tvp-hint'); if (w) w.textContent = `Nächster Versuch in ${20 - Math.floor(((now - PL.t0) / 1000) % 20)} s`; }
  if (redo) render(); else tvpPaint();
}, 250);
window.addEventListener('hashchange', (e) => { const was = /#\/live\/watch\//.test(e.oldURL || ''); const is = /#\/live\/watch\//.test(e.newURL || ''); if (was && !is) { PL.leaving = true; PL.id = null; SIM.stop(); } });

/* ---- switching ---- */
function tvpNextId(dir) { const pl = tvpPl(); const ids = tvpListIds(pl); const k = ids.indexOf(PL.id); const n = ids[((k < 0 ? 0 : k) + dir + ids.length) % ids.length]; return { id: n, wrap: k >= 0 && (k + dir < 0 || k + dir >= ids.length) }; }
/** switch with the rule of 4.8: from a time shift the digit/zap keys need a second press, picks ask first (LD-10) */
function tvpRequest(id, via) {
  if (id === PL.id) { PL.list = false; PL.guide = false; render(); return; }
  const pl = tvpPl(); const shifted = pl && (pl.offset > 1 || pl.paused);
  if (shifted && via === 'zap') {
    if (PL.pend && PL.pend.id === id && Date.now() < PL.pend.until) { PL.pend = null; tvpSwitch(id); return; }
    PL.pend = { id, until: Date.now() + 3000 }; toast(`Du schaust ${fmt.dur(pl.offset)} zeitversetzt. Nochmals drücken zum Wechseln`); return;
  }
  if (shifted && via === 'pick') { LiveDlg.zapConfirm({ to: id, onConfirm: () => tvpSwitch(id) }); return; }
  tvpSwitch(id);
}
function tvpSwitch(id) { PL.list = false; PL.guide = false; PL.bar = false; go('#/live/watch/' + id, { replace: true }); }
function tvpZap(dir) { const n = tvpNextId(dir); tvpRequest(n.id, 'zap'); if (n.wrap && !PL.pend) { const c = SIM.channel(n.id); toast(dir > 0 ? `Wieder am Anfang: ${c.no} ${c.name}` : `Am Ende der Liste: ${c.no} ${c.name}`); } }
function tvpShowBar() { PL.bar = true; PL.barUntil = Date.now() + 8000; PL.info = false; PL.infoFocus = false; render(); }
function tvpShowInfo(focus) { PL.info = true; PL.infoUntil = Date.now() + 5000; PL.infoFocus = !!focus; PL.bar = false; render(); if (focus) { const el = byF('pl-next') || byF('pl-now'); if (el) focus_(el); } }
const focus_ = (el) => focus(el, { instant: true });
function tvpPlayPause() {
  const r = SIM.rights(); const pl = tvpPl();
  if (!r.timeshift) return;
  if (pl && pl.paused) SIM.resume(); else { SIM.pause(); PL.bar = true; PL.barUntil = Date.now() + 8000; }
}
function tvpSeekBy(sec) {
  const r = SIM.rights(); const pl = tvpPl();
  if (!r.timeshift) return;
  if (!pl || pl.max < 0.05) { toast('Noch nichts zum Zurückspulen: die Leiste beginnt bei deinem Einschalten.'); return; }
  SIM.seek(pl.offset - sec / 60);
}
function tvpRecAct() {
  const r = SIM.rights(); if (!r.recordings) return;
  const c = SIM.channel(PL.id); const run = SIM.runningOn(c.id); const nn = SIM.nowNext(c.id).now; const rs = nn ? SIM.recState(nn.id) : null;
  if (run) {
    menu(`<h2>Nimmt auf</h2><p>${esc(run.title)} · seit ${fmt.time(run.start)}, bis ${fmt.time(run.end)}</p><div class="mlist">
      ${nn && run.end < nn.end + 10 ? `<button type="button" class="mi" data-act="tvpRecExtend" data-id="${run.id}" data-f="rm-ext" data-default><span class="mi-ic">${icon('clock', 36)}</span><span class="mi-tx"><b>Bis Sendungsende verlängern</b><small>bis ${fmt.time(nn.end + 10)}</small></span></button>` : ''}
      <button type="button" class="mi" data-act="tvpRecStop" data-id="${run.id}" data-f="rm-stop"${nn && run.end < nn.end + 10 ? '' : ' data-default'}><span class="mi-ic">${icon('stop', 36)}</span><span class="mi-tx"><b>Aufnahme beenden</b><small>Das Bisherige bleibt erhalten</small></span></button>
      <a class="mi" href="#/live/recordings/${run.id}" data-f="rm-mine"><span class="mi-ic">${icon('film', 36)}</span><span class="mi-tx"><b>Zu meinen Aufnahmen</b><small>Beendet die Wiedergabe</small></span></a></div>`);
  } else if (rs && nn) LiveDlg.planRecording(nn.id);
  else LiveDlg.instantRecording(c.id);
}
ACT.tvpRecExtend = (d) => { const run = SIM.recording(d.id); const nn = SIM.nowNext(run.ch).now; if (run && nn) { run.end = nn.end + 10; } closeMenu(); SIM.emit('change'); toast(`Aufnahme läuft bis ${fmt.time(run.end)}.`); };
ACT.tvpRecStop = (d) => { closeMenu(); LiveDlg.confirmDelete({ kind: 'running', ids: [d.id] }); };

function tvpListKey(name) {
  if (/^d\d$/.test(name)) { liveNumber(name, (c) => tvpRequest(c.id, 'pick')); return true; }
  if (name === 'ok' && liveNumberActive()) { liveNumberCommit(); return true; }
  if (name === 'left' || name === 'right') { PL.list = false; render(); return true; }
  if (name === 'guide') { tvGuideOverlay(); return true; }
  if (name === 'ch_up' || name === 'ch_down') { const rows = $$('.tvp-zr'); const k = rows.indexOf(F.el); const t = rows[Math.max(0, Math.min(rows.length - 1, k + (name === 'ch_down' ? 7 : -7)))]; if (t) focus(t); return true; }
  if (name === 'play' || name === 'info' || name === 'rec' || name === 'last') return true;
  return false;
}
function tvpKey(name) {
  if (PL.bar) PL.barUntil = Date.now() + 8000;
  if (PL.guide) return false;
  if (PL.list) return tvpListKey(name);
  if (/^d\d$/.test(name)) { liveNumber(name, (c) => tvpRequest(c.id, 'zap')); return true; }
  if (name === 'ok' && liveNumberActive()) { liveNumberCommit(); return true; }
  const ph = SIM.channel(PL.id) ? tvpPhase(SIM.channel(PL.id)) : { k: 'ok' }; const err = ph.k === 'forced' || ph.k === 'error';
  if (name === 'ch_up') { tvpZap(1); return true; }
  if (name === 'ch_down') { tvpZap(-1); return true; }
  if (name === 'last') { const pl = tvpPl(); if (pl && pl.prev) tvpRequest(pl.prev, 'zap'); else toast('Kein zuletzt gesehener Sender'); return true; }
  if (name === 'guide') { tvGuideOverlay(); return true; }
  if (name === 'rec') { tvpRecAct(); return true; }
  if (name === 'play') { if (err) return true; tvpPlayPause(); return true; }
  if (name === 'info') { tvpShowInfo(!err); return true; }
  const el = F.el && $('#view').contains(F.el) ? F.el : null;
  if (PL.infoFocus && el && el.closest('.tvp-banner')) {
    if (name === 'left') { const e = byF('pl-now'); if (e) focus_(e); return true; }
    if (name === 'right') { const e = byF('pl-next'); if (e) focus_(e); return true; }
    if (name === 'up' || name === 'down') { PL.info = false; PL.infoFocus = false; render(); tvpZap(name === 'up' ? 1 : -1); return true; }
    return false;
  }
  if (PL.bar && el) {
    if (el.dataset.f === 'pl-seek' && (name === 'left' || name === 'right')) {
      const now = Date.now(); PL.step = now - PL.lastSeek < 700 ? Math.min(PL.step + 1, 5) : 0; PL.lastSeek = now;
      const sec = [10, 10, 30, 60, 120, 300][PL.step]; tvpSeekBy(name === 'left' ? -sec : sec); return true;
    }
    return false;
  }
  if (err) { if (name === 'left') { tvpListShow(); return true; } return false; }
  if (name === 'up') { tvpZap(1); return true; }
  if (name === 'down') { tvpZap(-1); return true; }
  if (name === 'left') { tvpListShow(); return true; }
  if (name === 'right') { tvpShowInfo(true); return true; }
  if (name === 'ok') { tvpShowBar(); return true; }
  return false;
}
function tvpListShow() { PL.list = true; PL.bar = false; PL.info = false; PL.infoFocus = false; render(); }
function tvpBack() {
  if (PL.list) { PL.list = false; render(); return true; }
  if (PL.infoFocus) { PL.info = false; PL.infoFocus = false; render(); return true; }
  if (PL.bar) { PL.bar = false; render(); return true; }
  PL.leaving = true; SIM.stop(); PL.id = null; navBack(); return true;
}
ACT.tvpPrev = () => tvpZap(-1);
ACT.tvpNext = () => tvpZap(1);
ACT.tvpPause = () => tvpPlayPause();
ACT.tvpBack10 = () => tvpSeekBy(-10);
ACT.tvpFwd10 = () => tvpSeekBy(10);
ACT.tvpLive = () => { SIM.toLive(); toast('Zurück zu Live'); };
ACT.tvpRec = () => tvpRecAct();
ACT.tvpListOpen = () => tvpListShow();
ACT.tvpGuide = () => tvGuideOverlay();
ACT.tvpInfo = () => tvpShowInfo(true);
ACT.tvpRetry = () => { PL.t0 = Date.now(); render(); };
ACT.tvpHome = () => { PL.leaving = true; SIM.stop(); PL.id = null; go('#/'); };
ACT.tvpZapPick = (d) => tvpRequest(d.ch, 'pick');
ACT.tvpListCycle = () => {
  const pl = tvpPl(); if (!pl) return; const opts = [['Alle Sender', tvAllIds()], ['Meine Sender', SIM.favorites()], ...LIVE.groups.map((g) => [g.name, SIM.channels({ group: g.id }).map((c) => c.id)])].filter((o) => o[1].length);
  const k = opts.findIndex((o) => o[0] === tvpListName(pl)); const n = opts[(k + 1) % opts.length];
  pl.list.length = 0; pl.list.push(...n[1]); render(); toast(`Liste: ${n[0]}`);
};
const TV_Q = ['Automatisch', 'Original (1080i)', '720p', '480p'];
function tvpAudioHtml() {
  /* Teletext subtitles: in no client before stage E (04 7.4, RG-05); DVB subtitles selectable */
  const c = SIM.channel(PL.id); const pf = S.prefs; const all = c.subs || []; const subs = all.filter((x) => !LU.isTeletext(x)); const ttx = all.filter(LU.isTeletext);
  const row = (f, ic, t, v, act, dis) => `<button type="button" class="mi" data-act="${act}" data-f="${f}"${dis ? ' aria-disabled="true"' : ''}><span class="mi-ic">${icon(ic, 36)}</span><span class="mi-tx"><b>${t}</b><small>${esc(v)}</small></span>${dis ? '' : icon('sort', 30)}</button>`;
  return `<h2>Ton, Untertitel, Qualität</h2><div class="mlist">${row('au-a', 'audio', 'Ton', c.audio && c.audio.includes(pf.audio) ? pf.audio : (c.audio || ['Deutsch'])[0], 'tvpAu', false)}
    ${row('au-s', 'subs', 'Untertitel', subs.length ? (subs.includes(pf.subs) ? pf.subs : 'Aus') : ttx.length ? LU.SUBS_TX.teletext : LU.SUBS_TX.none, 'tvpSub', !subs.length)}${subs.length && ttx.length ? `<p class="mi-note">${esc(ttx.join(', '))}: als Videotext noch nicht unterstützt</p>` : ''}${row('au-q', 'hd', 'Qualität', pf.quality, 'tvpQual', false)}</div>`;
}
ACT.tvpAudio = () => menu(tvpAudioHtml());
const tvpCycle = (arr, cur) => arr[(Math.max(0, arr.indexOf(cur)) + 1) % arr.length];
ACT.tvpAu = () => { const c = SIM.channel(PL.id); const a = c.audio || ['Deutsch']; S.prefs.audio = tvpCycle(a, a.includes(S.prefs.audio) ? S.prefs.audio : a[0]); save(); redrawMenu(tvpAudioHtml()); toast('Wechsle Tonspur …'); };
ACT.tvpSub = () => { const c = SIM.channel(PL.id); const sel = (c.subs || []).filter((x) => !LU.isTeletext(x)); if (!sel.length) { toast((c.subs || []).length ? LU.SUBS_TX.teletext : LU.SUBS_TX.none); return; } const a = ['Aus', ...sel]; S.prefs.subs = tvpCycle(a, a.includes(S.prefs.subs) ? S.prefs.subs : 'Aus'); save(); redrawMenu(tvpAudioHtml()); };
ACT.tvpQual = () => { S.prefs.quality = tvpCycle(TV_Q, S.prefs.quality); save(); redrawMenu(tvpAudioHtml()); };
/* fault states of the player: mockup bar «Zustände» → Player (LIVE_SIM.flags().fault) */

/* LD-10 Senderwechsel bei Zeitversatz: small hint dialog with the stay button first */
registerLiveDialog('LD-10', (props) => {
  const pl = tvpPl(); const off = pl ? pl.offset : 12.5; const c = SIM.channel(props.to) || SIM.channel('zdf');
  tvZapCb = props.onConfirm || (() => liveTune(c.id));
  return { dialog: true, html: `<h2>Zu ${esc(c.name)} wechseln?</h2><p>Du schaust ${fmt.dur(off)} zeitversetzt. Beim Senderwechsel geht das verloren.</p>
    <div class="acts">${tvBtn({ f: 'zc-stay', ic: 'back', label: 'Bleiben', act: 'close', def: true })}${tvBtn({ f: 'zc-go', ic: 'check', label: 'Wechseln', act: 'tvZapGo', cls: 'primary' })}</div>`, onKey: null };
});
let tvZapCb = null;
ACT.tvZapGo = () => { const cb = tvZapCb; tvZapCb = null; closeMenu(); if (cb) cb(); };

/* =====================================================================================================
   LS-10 Aufnahme-Player
   ===================================================================================================== */
const PLR = { id: null, pos: 0, paused: false, bar: false, barUntil: 0, step: 0, lastSeek: 0, ask: false };
RESETS.push(() => { Object.assign(PLR, { id: null, pos: 0, paused: false, bar: false, ask: false }); });
const tvrDur = (r) => Math.max(1, (r.state === 'running' ? SIM.now() : r.end) - r.start);
function tvrFmt(min) { const s = Math.max(0, Math.round(min * 60)); const h = Math.floor(s / 3600); const m = Math.floor(s / 60) % 60; const x = s % 60; return (h ? h + ':' + String(m).padStart(2, '0') : String(m)) + ':' + String(x).padStart(2, '0'); }
route('/live/recordings/:id/play', (pm, q) => {
  const r = SIM.recording(pm.id);
  if (!tvRecVisible(r) || !['done', 'incomplete', 'running'].includes(r.state)) return { nav: '', html: livePage({ tab: 'recordings', title: 'Nicht gefunden', html: `${tvNote('info', 'Diese Aufnahme lässt sich nicht abspielen.', '')}<div class="ltv-acts"><button type="button" class="tb primary" data-act="go" data-to="#/live/recordings" data-f="rp-nf" data-default>${icon('film', 32)}<span>Zu meinen Aufnahmen</span></button></div>` }) };
  /* a recording of a profile with an age limit, played by the primary profile: no progress, start at the programme (14, 3.10) */
  const mine = r.profile === SIM.profile().id;
  if (PLR.id !== pm.id) {
    const D = tvrDur(r); const resume = D * (r.watchedPct || 0) / 100;
    Object.assign(PLR, { id: pm.id, mine, bar: false, step: 0, paused: false, ask: false });
    if (q.from === 'full') PLR.pos = 0; else if (q.from === 'resume' && mine) PLR.pos = resume; else if (q.from === 'start' || !mine) PLR.pos = r.padBefore;
    else if (r.watchedPct >= 1 && r.watchedPct < 95) { PLR.pos = resume; PLR.paused = true; PLR.ask = true; } else PLR.pos = r.padBefore;
  }
  const c = SIM.channel(r.ch); const D = tvrDur(r); PLR.pos = Math.max(0, Math.min(PLR.pos, D));
  return {
    nav: '', full: true,
    html: `<div class="tvp-pl" data-rp>${LU.fakeVideo(c, { paused: PLR.paused })}<i class="tvp-scrim" aria-hidden="true"></i>${PLR.paused ? `<span class="tvp-pause" aria-hidden="true">${icon('pause', 120)}</span>` : ''}
      <div class="tvr-top"><b>${esc(r.title)}${r.sub ? ` · «${esc(r.sub)}»` : ''}</b><span>${esc(c.name)} · ${esc(fmt.when(r.start))}</span>${r.state === 'running' ? `<span class="tvr-run">${LU.badge('recording', { text: true, label: 'Aufnahme läuft' })}</span>` : ''}${!mine ? `<span class="tvr-run">Aufnahme von ${esc(SIM.profiles().find((x) => x.id === r.profile).name)} · Fortschritt wird nicht gespeichert</span>` : ''}</div>
      ${PLR.bar ? tvrBarHtml(r, D) : ''}<p class="tvp-sample">${esc(LU.TX.sample)}</p></div>`,
    after() {
      tvrPaint();
      if (PLR.ask) { PLR.ask = false; setTimeout(() => tvrAsk(r, D), 30); }
    },
    key: tvrKey, back() { tvrLeave(); return true; },
  };
});
function tvrBarHtml(r, D) {
  const pct = (m) => (m / D * 100).toFixed(2);
  const gaps = (r.gaps || []).map(([a, b]) => `<i class="tvr-gap lv-gap" style="left:${pct(a - r.start)}%;width:${pct(b - a)}%" title="${esc(`Lücke ${fmt.time(a)}–${fmt.time(b)}`)}"></i>`).join('');
  const b = (f, ic, label, act, o) => `<button type="button" class="tvp-b" data-act="${act}" data-f="${f}"${o && o.def ? ' data-default' : ''} aria-label="${esc(o && o.aria || label)}">${icon(ic, 44)}${label ? `<span>${esc(label)}</span>` : ''}</button>`;
  return `<div class="tvp-bar"><div class="tvp-seek" data-f="pr-seek" role="slider" aria-label="Position" aria-valuemin="0" aria-valuemax="${Math.round(D * 60)}" aria-valuenow="${Math.round(PLR.pos * 60)}">
    <span class="tvp-sl"><span><span id="tvr-t">${tvrFmt(PLR.pos)}</span> / ${tvrFmt(D)}${r.state === 'running' ? ' · wächst' : ''}</span>${r.state === 'incomplete' ? `<span>${LU.REC_STATE.incomplete}: ${(r.gaps || []).length} Lücken</span>` : ''}</span>
    <span class="tvp-trk tvr-trk"><i class="tvr-pad" style="left:0;width:${pct(r.padBefore)}%"></i><i class="tvr-pad" style="right:0;width:${pct(r.padAfter || 0)}%"></i>${gaps}<i class="tvp-buf" id="tvr-buf"></i><i class="tvp-pos" id="tvr-pos"></i></span>
    <span class="tvp-sr">${r.padBefore ? `Vorlauf ${r.padBefore} Min. · ` : ''}←/→ ±10 s · Ziffern springen in Prozent</span></div>
    <div class="tvp-btns">${b('pr-pause', PLR.paused ? 'play' : 'pause', '', 'tvrPause', { def: true, aria: PLR.paused ? 'Weiter' : 'Anhalten' })}${b('pr-back', 'rew', '10 s', 'tvrBack10', { aria: '10 Sekunden zurück' })}${b('pr-fwd', 'fwd', '10 s', 'tvrFwd10', { aria: '10 Sekunden vor' })}${r.state === 'running' ? b('pr-end', 'live', 'Zum aktuellen Stand', 'tvrToEnd') : ''}<span class="tvp-sp"></span>${b('pr-audio', 'audio', 'Ton und Untertitel', 'tvrAudio')}</div></div>`;
}
function tvrPaint() {
  const r = SIM.recording(PLR.id); if (!r) return; const D = tvrDur(r); const p = $('#tvr-pos'); if (!p) return;
  const f = (PLR.pos / D * 100).toFixed(2); p.style.left = f + '%'; $('#tvr-buf').style.width = f + '%'; const t = $('#tvr-t'); if (t) t.textContent = tvrFmt(PLR.pos);
}
setInterval(() => {
  if (!/^\/live\/recordings\/[^/]+\/play$/.test(curPath()) || !PLRactive()) return;
  const r = SIM.recording(PLR.id); if (!r) return; const D = tvrDur(r);
  if (!PLR.paused && !OV.length) { PLR.pos = Math.min(D, PLR.pos + 1 / 60); if (PLR.pos >= D && r.state !== 'running') { PLR.paused = true; toast('Ende der Aufnahme'); render(); return; } }
  if (PLR.bar && Date.now() > PLR.barUntil && !PLR.paused && !OV.length) { PLR.bar = false; render(); return; }
  tvrPaint();
}, 1000);
const PLRactive = () => !!PLR.id;
function tvrLeave() { const r = SIM.recording(PLR.id); if (r && PLR.mine !== false) { const pct = Math.round(PLR.pos / tvrDur(r) * 100); SIM.setWatched(r.id, pct >= 98 ? 100 : pct); } PLR.id = null; navBack(); }
function tvrAsk(r, D) {
  menu(`<h2>Weiter schauen?</h2><div class="mlist">
    <button type="button" class="mi" data-act="tvrFrom" data-from="resume" data-f="ra-r" data-default><span class="mi-ic">${icon('play', 36)}</span><span class="mi-tx"><b>Weiter ab ${tvrFmt(D * r.watchedPct / 100)}</b><small>${r.watchedPct} % gesehen</small></span></button>
    <button type="button" class="mi" data-act="tvrFrom" data-from="start" data-f="ra-s"><span class="mi-ic">${icon('rew', 36)}</span><span class="mi-tx"><b>Von vorne</b><small>Ab Sendungsbeginn</small></span></button>
    <button type="button" class="mi" data-act="tvrFrom" data-from="full" data-f="ra-f"><span class="mi-ic">${icon('film', 36)}</span><span class="mi-tx"><b>Ab Aufnahmebeginn</b><small>mit Vorlauf${r.padBefore ? ` (${r.padBefore} Min.)` : ''}</small></span></button></div>`, { dialog: true, onClose: () => { if (PLR.paused && PLR.id) { PLR.paused = false; render(); } } });
}
ACT.tvrFrom = (d) => { const r = SIM.recording(PLR.id); const D = tvrDur(r); PLR.pos = d.from === 'resume' ? D * r.watchedPct / 100 : d.from === 'start' ? r.padBefore : 0; PLR.paused = false; closeMenu(); render(); };
function tvrShow() { PLR.bar = true; PLR.barUntil = Date.now() + 6000; }
function tvrSeek(sec) { const r = SIM.recording(PLR.id); const D = tvrDur(r); PLR.pos = Math.max(0, Math.min(D, PLR.pos + sec / 60)); tvrShow(); if (!$('.tvp-bar')) render(); else tvrPaint(); }
function tvrKey(name) {
  if (PLR.bar) PLR.barUntil = Date.now() + 6000;
  const r = SIM.recording(PLR.id); if (!r) return false; const D = tvrDur(r);
  if (/^d\d$/.test(name)) { PLR.pos = D * Number(name[1]) / 10; tvrShow(); toast(`Sprung auf ${Number(name[1]) * 10} %`); render(); return true; }
  if (name === 'play') { PLR.paused = !PLR.paused; tvrShow(); render(); return true; }
  if (name === 'left' || name === 'right') {
    const onSeek = F.el && F.el.dataset.f === 'pr-seek';
    if (!PLR.bar || onSeek) { const now = Date.now(); PLR.step = now - PLR.lastSeek < 700 ? Math.min(PLR.step + 1, 5) : 0; PLR.lastSeek = now; const sec = [10, 10, 30, 60, 120, 300][PLR.step]; tvrSeek(name === 'left' ? -sec : sec); return true; }
    return false;
  }
  if (!PLR.bar) { if (name === 'up' || name === 'down' || name === 'ok') { if (name === 'ok') PLR.paused = !PLR.paused; tvrShow(); render(); return true; } }
  if (name === 'ch_up' || name === 'ch_down' || name === 'guide' || name === 'last' || name === 'info' || name === 'rec') { toast('In Aufnahmen gibt es keine Sender. Ziffern springen in Prozent.'); return true; }
  if (name === 'back') { if (PLR.bar && PLR.paused === false) { PLR.bar = false; render(); return true; } if (PLR.bar) { PLR.bar = false; render(); return true; } tvrLeave(); return true; }
  return false;
}
ACT.tvrPause = () => { PLR.paused = !PLR.paused; tvrShow(); render(); };
ACT.tvrBack10 = () => tvrSeek(-10);
ACT.tvrFwd10 = () => tvrSeek(10);
ACT.tvrToEnd = () => { const r = SIM.recording(PLR.id); PLR.pos = tvrDur(r); tvrShow(); render(); toast('Zum aktuellen Stand'); };
ACT.tvrAudio = () => { const r = SIM.recording(PLR.id); menu(`<h2>Ton und Untertitel</h2><p>${esc(r.tracks || '')}</p><div class="mlist"><button type="button" class="mi" data-act="close" data-f="ra-ok" data-default><span class="mi-ic">${icon('check', 36)}</span><span class="mi-tx"><b>Fertig</b><small>Alle empfangenen Spuren sind erhalten</small></span></button></div>`); };
