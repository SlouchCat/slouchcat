/* Konzept 28 «Live-TV», phone and tablet mockup: the bridge to LIVE_SIM and what every Live-TV page of the
   app shares: rights gate (404), chip row of the area, page frame, dialog interface (LD-xx as sheets), the
   basic filter sheet LD-09, placeholders from the register. Owned by the core. */
'use strict';
const SIM = window.LIVE_SIM;
const LU = window.LIVE_UI;
LU.img = '../shared/img/';
const fmt = LU.fmt;

/** without the right the area is missing: «Nicht gefunden» (404, never 403) */
function liveGate(path) {
  const r = SIM.rights();
  const nf = () => ({ html: `${bar('Nicht gefunden', { back: false })}<div class="empty">${icon('alert', 40)}<b>Nicht gefunden</b><p>Diese Seite gibt es nicht.</p><p style="margin-top:16px"><a class="btn ghost" href="#/" data-fid="nf-home">Zur Startseite</a></p></div>` });
  /* right taken away while watching (mockup state «Recht entzogen …»): the player shows the message (14, 4.12) */
  if (SIM.flags().revoked && path.startsWith('/live/watch/')) return null;
  if (path.startsWith('/live') && !r.liveTv) return nf();
  if (/^\/live\/(recordings|scheduled)/.test(path) && !r.recordings) return nf();
  if (path === '/live/recordings/profiles' && !SIM.profile().primary) return nf();
  return null;
}
/** switch a channel on and open the player (LS-07); list = zapping list (channel ids) */
function liveTune(ch, list) { SIM.tune(ch, { list }); go('#/live/watch/' + ch); }

/** chip row of the Live-TV area under the top bar, scrolls sideways; active: start|channels|guide|recordings|reminders */
function liveChips(active) {
  const r = SIM.rights();
  const tabs = [['start', 'Start', '#/live'], ['channels', 'Sender', '#/live/channels'], ['guide', 'Programm', '#/live/guide'], r.recordings ? ['recordings', 'Aufnahmen', '#/live/recordings'] : null, ['reminders', 'Erinnerungen', '#/live/reminders']].filter(Boolean);
  return `<nav class="chips lh-tabs" aria-label="Bereiche von Live-TV">${tabs.map(([k, l, h]) => `<a class="chip${k === active ? ' on' : ''}" href="${h}"${k === active ? ' aria-current="page"' : ''} data-fid="lt-${k}">${l}</a>`).join('')}</nav>`;
}
/** frame of a Live-TV page: { tab, title, end (html for the bar end), html } */
function livePage(o) {
  return `${bar(o.title, { back: false, end: o.end || '' })}${liveChips(o.tab)}${o.html}<p class="lh-sample">${esc(LU.TX.sample)}</p>`;
}

/* ---------- dialogs (LD-xx) as sheets ---------- */
const LIVE_DIALOGS = {};
/** registerLiveDialog('LD-01', (props) => ({ html, center })) — html is the inside of sheet() */
function registerLiveDialog(id, fn) { LIVE_DIALOGS[id] = fn; }
function liveDialog(id, props) {
  const fn = LIVE_DIALOGS[id];
  if (fn) { const d = fn(props || {}); sheet(d.html, { center: d.center }); return; }
  const e = LIVE_REG.get(id, 'handy') || { title: id, agent: 'handy', file: '' };
  sheet(`<h2>${esc(id)} ${esc(e.title)}</h2><p class="pad hint">Dieser Dialog wird ergänzt (${esc(LIVE_REG.AGENTS[e.agent].name)}, ${esc(e.file)}).</p><div class="acts"><button type="button" class="btn primary" data-act="close" data-fid="dph-x">Schliessen</button></div>`);
}
const LiveDlg = {
  planRecording: (programId) => liveDialog('LD-01', { programId }),
  seriesRecording: (programId, ruleId) => liveDialog('LD-02', { programId, ruleId }),
  manualRecording: (o) => liveDialog('LD-03', o || {}),
  instantRecording: (ch) => liveDialog('LD-04', { ch }),
  reminder: (programId) => liveDialog('LD-05', { programId }),
  confirmDelete: (o) => liveDialog('LD-06', o || {}),
  warning: (warningId) => liveDialog('LD-07', { warningId }),
  pickTime: (o) => liveDialog('LD-08', o || {}),
  /** LD-09: o = { fields: [{ key, label, options: [{v, t}] }], values: {}, onApply(values) } */
  filterSheet: (o) => liveDialog('LD-09', o || {}),
  zapConfirm: (o) => liveDialog('LD-10', o || {}),
  retention: (recordingId) => liveDialog('LD-11', { recordingId }),
};
/* LD-09 basic form (14-oberflaeche.md 5.9): choices per field, «Anwenden», «Zurücksetzen»; back closes without change */
let LD09 = null;
registerLiveDialog('LD-09', (p) => {
  if (!LD09 || LD09.p !== p) LD09 = { p, v: Object.assign({}, p.values || {}) };
  return { html: ld09Html() };
});
function ld09Html() {
  const { p, v } = LD09;
  return `<h2>Filter</h2>${(p.fields || []).map((f) => `<div class="sec">${esc(f.label)}</div><div class="chips lh-wrap">${f.options.map((o) => `<button type="button" class="chip${String(v[f.key] || '') === String(o.v) ? ' on' : ''}" aria-pressed="${String(v[f.key] || '') === String(o.v)}" data-act="ld09Pick" data-k="${esc(f.key)}" data-v="${esc(o.v)}" data-fid="ld09-${esc(f.key)}-${esc(o.v) || 'all'}">${esc(o.t)}</button>`).join('')}</div>`).join('')}
    <div class="acts"><button type="button" class="btn ghost" data-act="ld09Reset" data-fid="ld09-reset">Zurücksetzen</button><button type="button" class="btn primary" data-act="ld09Apply" data-fid="ld09-ok">Anwenden</button></div>`;
}
const ld09Redraw = () => { const sh = $('#overlays .sheet'); if (sh) { const f = document.activeElement && document.activeElement.dataset.fid; sh.innerHTML = ld09Html(); const el = f && sh.querySelector(`[data-fid="${f}"]`); if (el) el.focus(); } };
ACT.ld09Pick = (d) => { LD09.v[d.k] = d.v; ld09Redraw(); };
ACT.ld09Reset = () => { Object.keys(LD09.v).forEach((k) => { LD09.v[k] = ''; }); ld09Redraw(); };
ACT.ld09Apply = () => { const { p, v } = LD09; LD09 = null; closeSheet(); if (p.onApply) p.onApply(v); };
const DIALOG_DEMO = {
  'LD-01': () => LiveDlg.planRecording('p-zdf-1215'), 'LD-02': () => LiveDlg.seriesRecording('p-daserste-2655'), 'LD-03': () => LiveDlg.manualRecording({ ch: 'telezueri' }),
  'LD-04': () => LiveDlg.instantRecording('zdf'), 'LD-05': () => LiveDlg.reminder('p-srf1-2645'), 'LD-06': () => LiveDlg.confirmDelete({ kind: 'recording', ids: ['rec-arena'] }),
  'LD-07': () => LiveDlg.warning('w-quota'), 'LD-08': () => LiveDlg.pickTime({ day: 0, time: 1215 }), 'LD-09': () => LiveDlg.filterSheet(chFilterSheet({}, () => toast('Filter angewendet (Vorschau)'))),
  'LD-10': () => LiveDlg.zapConfirm({ to: 'zdf' }), 'LD-11': () => LiveDlg.retention('rec-arena'),
};
ACT.demoDialog = (d) => DIALOG_DEMO[d.id] && DIALOG_DEMO[d.id]();
/** the filter fields of the channel list (LS-02), also used by other pages */
function chFilterSheet(values, onApply) {
  return {
    fields: [
      { key: 'group', label: 'Gruppe', options: [{ v: '', t: 'Alle Sender' }, { v: 'fav', t: 'Meine Sender' }, ...LIVE.groups.map((g) => ({ v: g.id, t: g.name }))] },
      { key: 'country', label: 'Land', options: [{ v: '', t: 'Alle' }, ...Object.entries(LIVE.countries).map(([v, t]) => ({ v, t }))] },
      { key: 'lang', label: 'Sprache', options: [{ v: '', t: 'Alle' }, ...Object.entries(LIVE.languages).map(([v, t]) => ({ v, t }))] },
      { key: 'category', label: 'Kategorie', options: [{ v: '', t: 'Alle' }, ...[...new Set(LIVE.channels.filter(SIM.viewerChannel).map((c) => c.category))].map((v) => ({ v, t: v }))] },
    ],
    values: Object.assign({}, values), onApply,
  };
}

/* ---------- placeholders for every register entry without a page ---------- */
function installPlaceholders() {
  const have = new Set(ROUTES.map((r) => r.pattern));
  const byPattern = {};
  LIVE_REG.forClient('handy').filter((e) => e.route && !have.has(e.route) && !e.id.startsWith('LD-')).forEach((e) => (byPattern[e.route] || (byPattern[e.route] = [])).push(e));
  Object.keys(byPattern).sort((a, b) => (a.includes(':') - b.includes(':')) || b.length - a.length).forEach((pt) => {
    const e = byPattern[pt][0];
    route(pt, () => ({ nav: pt === '/' ? 'start' : pt === '/search' ? 'search' : '', html: `${bar(e.title, { back: pt !== '/' })}<div class="empty lh-ph">${icon('clock', 40)}<b>${esc(e.id)} ${esc(e.title)} wird ergänzt</b><p>Diese Seite baut «${esc(LIVE_REG.AGENTS[e.agent].name)}» in ${esc(e.file)}.</p><p class="hint">${esc(pt)}</p></div>` }));
  });
  if (!have.has('/live/dialog/:id')) route('/live/dialog/:id', (pm) => {
    const e = LIVE_REG.get(pm.id, 'handy');
    return { after: () => setTimeout(() => { if (!$('#overlays .scrim') && DIALOG_DEMO[pm.id]) DIALOG_DEMO[pm.id](); }, 30), html: livePage({ tab: '', title: `${pm.id} ${e ? e.title : ''}`, html: `<p class="pad hint">Vorschau des Dialogs mit Beispieldaten.</p><p class="pad" style="margin-top:12px"><button type="button" class="btn primary" data-act="demoDialog" data-id="${esc(pm.id)}" data-fid="dh-open">Dialog öffnen</button></p>` }) };
  });
}

/* a change of the simulation redraws the page; with an open sheet the page redraws after closing */
let liveDirty = false;
SIM.on((t) => { if (t !== 'change') return; if ($('#overlays .scrim')) liveDirty = true; else render(); });
setInterval(() => { if (liveDirty && !$('#overlays .scrim')) { liveDirty = false; render(); } }, 400);
