/* Konzept 28 «Live-TV», TV mockup: the bridge to LIVE_SIM and what every Live-TV page of the TV app shares:
   rights gate (404), tab row, page frame, channel number entry (ChannelNumberEntry, 6.7), default handling
   of the new remote keys, dialog interface (LD-xx), placeholders from the register. Owned by the core. */
'use strict';
const SIM = window.LIVE_SIM;
const LU = window.LIVE_UI;
LU.img = '../shared/img/';
const fmt = LU.fmt;

/** without the right the area is missing: «Nicht gefunden» (404, never 403) */
function liveGate(path) {
  const r = SIM.rights();
  const nf = () => ({ nav: '', html: `<div class="page soon"><span class="soon-ic">${icon('alert', 88)}</span><h1>Nicht gefunden</h1><p>Diese Seite gibt es nicht.</p><button type="button" class="tb primary" data-act="go" data-to="#/" data-f="nf-home" data-default>${icon('home', 32)}<span>Zur Startseite</span></button></div>` });
  /* right taken away while watching (mockup state «Recht entzogen …»): the player shows the message (14, 4.12) */
  if (SIM.flags().revoked && path.startsWith('/live/watch/')) return null;
  /* variant E-28-42 (a), mockup state a12 (default off = Vorgabe (b): 404 as before). A-12 (14 3.14): the primary profile keeps LS-14 and the details of its profiles' recordings, reached only
     through «Profile verwalten», even without its own right «Live-TV» or «Aufnahmen»; playing needs Live-TV */
  const kid = path.match(/^\/live\/recordings\/([^/]+)(\/play)?$/); const kr = kid && SIM.recording(kid[1]);
  if (SIM.flags().a12 && SIM.profile().primary && (path === '/live/recordings/profiles' || (kr && SIM.childProfiles().some((p) => p.id === kr.profile) && (!kid[2] || r.liveTv)))) return null;
  if (path.startsWith('/live') && !r.liveTv) return nf();
  if (/^\/live\/(recordings|scheduled)/.test(path) && !r.recordings) return nf();
  if (path === '/live/recordings/profiles' && !SIM.profile().primary) return nf();
  return null;
}

/** switch a channel on and open the player (LS-07); list = zapping list (channel ids) */
function liveTune(ch, list) { SIM.tune(ch, { list }); go('#/live/watch/' + ch); }

/* ---------- tab row of the Live-TV area: ↑ from the first content row leads in, ↓ back ---------- */
/** active: start | channels | guide | recordings | reminders */
function liveTabs(active) {
  const r = SIM.rights();
  const tabs = [['start', 'Start', '#/live'], ['channels', 'Sender', '#/live/channels'], ['guide', 'Programm', '#/live/guide'], r.recordings ? ['recordings', 'Aufnahmen', '#/live/recordings'] : null, ['reminders', 'Erinnerungen', '#/live/reminders']].filter(Boolean);
  return `<nav class="ltv-tabs" data-row="ltabs" aria-label="Bereiche von Live-TV">${tabs.map(([k, l, h]) => `<a class="ltv-tab" href="${h}" data-f="lt-${k}"${k === active ? ' aria-current="page"' : ''}>${l}</a>`).join('')}</nav>`;
}
/** frame of a Live-TV page: { tab, title, count, html } */
function livePage(o) {
  return `<div class="page ltv-page"><div class="ltv-head"><span class="ltv-eye">${icon('live', 30)}Live-TV</span>${liveTabs(o.tab)}</div>
    ${o.title ? `<div class="ltv-h"><h1>${o.title}</h1>${o.count ? `<span class="ltv-count">${o.count}</span>` : ''}</div>` : ''}${o.html}
    <p class="ltv-sample">${esc(LU.TX.sample)}</p></div>`;
}

/* ---------- channel number entry (6.7): digits, preview, «Kein Sender mit Nummer …» ---------- */
const NUM = { v: '', t: null, done: null };
/** liveNumber('d4', onDone(channel)) collects digits; after 1.6 s (or OK via liveNumberCommit) it calls onDone */
function liveNumber(key, onDone) {
  NUM.v = (NUM.v + key.slice(1)).slice(-3); NUM.done = onDone || NUM.done;
  clearTimeout(NUM.t); NUM.t = setTimeout(liveNumberCommit, 1600);
  drawNumber();
}
function liveNumberCommit() {
  clearTimeout(NUM.t); if (!NUM.v) return false;
  const c = SIM.channelByNo(NUM.v); const v = NUM.v; const fn = NUM.done;
  NUM.v = ''; NUM.done = null;
  if (!c) { drawNumber(`Kein Sender mit Nummer ${v}`); setTimeout(() => drawNumber(), 1800); return true; }
  drawNumber(); if (fn) fn(c);
  return true;
}
const liveNumberActive = () => !!NUM.v;
function drawNumber(err) {
  let box = $('#ltv-num');
  if (!box) { box = document.createElement('div'); box.id = 'ltv-num'; box.setAttribute('role', 'status'); box.setAttribute('aria-live', 'polite'); $('#screen').appendChild(box); }
  if (!NUM.v && !err) { box.className = ''; box.innerHTML = ''; return; }
  const c = NUM.v ? SIM.channelByNo(NUM.v) : null;
  box.className = 'on' + (err ? ' err' : '');
  box.innerHTML = err ? `<b>–</b><span>${esc(err)}</span>` : `<b>${esc(NUM.v)}</b>${c ? `${LU.logo(c)}<span>${esc(c.name)}</span>` : '<span>…</span>'}`;
}

/** what the new keys do where a page does not handle them (14-oberflaeche.md 4.13) */
function liveKeyDefault(name) {
  const r = SIM.rights(); if (!r.liveTv || (name === 'rec' && !r.recordings)) return; /* without the right the keys do nothing (14, 0 Grundsatz 1) */
  if (name === 'last') { const p = SIM.player(); if (p && p.prev) { SIM.lastChannel(); go('#/live/watch/' + SIM.player().ch); } else toast('Kein zuletzt gesehener Sender'); return; }
  if (/^d\d$/.test(name)) { liveNumber(name, (c) => liveTune(c.id)); return; }
  if (name === 'rec') { toast(SIM.rights().recordings ? 'Aufnehmen geht im Player und im Programm' : 'Aufnahmen sind für dieses Profil nicht freigegeben'); return; }
  if (name === 'info') { toast('Info zeigt im Player das Info-Banner, im Programm die Details'); return; }
  toast('Kanal +/− wirkt im Player, im Programm und in der Senderliste');
}

/* ---------- dialogs (LD-xx): side menu or centred dialog through menu() ---------- */
const LIVE_DIALOGS = {};
/** registerLiveDialog('LD-01', (props) => ({ html, dialog: true|false, onKey })) */
function registerLiveDialog(id, fn) { LIVE_DIALOGS[id] = fn; }
function liveDialog(id, props) {
  const fn = LIVE_DIALOGS[id];
  if (fn) { const d = fn(props || {}); return menu(d.html, { dialog: d.dialog !== false, cls: d.cls, onKey: d.onKey, onClose: d.onClose }); }
  const e = LIVE_REG.get(id, 'tv') || { title: id, agent: 'tv', file: '' };
  return menu(`<h2>${esc(id)} ${esc(e.title)}</h2><p>Dieser Dialog wird ergänzt (${esc(LIVE_REG.AGENTS[e.agent].name)}, ${esc(e.file)}).</p><div class="acts"><button type="button" class="tb primary" data-act="close" data-f="dph-x" data-default>${icon('close', 32)}<span>Schliessen</span></button></div>`, { dialog: true });
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
  zapConfirm: (o) => liveDialog('LD-10', o || {}),
  retention: (recordingId) => liveDialog('LD-11', { recordingId }),
};
const DIALOG_DEMO = {
  'LD-01': () => LiveDlg.planRecording('p-zdf-1215'), 'LD-02': () => LiveDlg.seriesRecording('p-daserste-2655'), 'LD-03': () => LiveDlg.manualRecording({ ch: 'telezueri' }),
  'LD-04': () => LiveDlg.instantRecording('zdf'), 'LD-05': () => LiveDlg.reminder('p-srf1-2645'), 'LD-06': () => LiveDlg.confirmDelete({ kind: 'recording', ids: ['rec-arena'] }),
  'LD-07': () => LiveDlg.warning('w-quota'), 'LD-08': () => LiveDlg.pickTime({ day: 0, time: 1215 }), 'LD-10': () => LiveDlg.zapConfirm({ to: 'zdf' }), 'LD-11': () => LiveDlg.retention('rec-arena'),
};
ACT.demoDialog = (d) => DIALOG_DEMO[d.id] && DIALOG_DEMO[d.id]();

/* ---------- placeholders for every register entry without a page ---------- */
function installPlaceholders() {
  const have = new Set(ROUTES.map((r) => r.pattern));
  const byPattern = {};
  LIVE_REG.forClient('tv').filter((e) => e.route && !have.has(e.route) && !e.id.startsWith('LD-')).forEach((e) => (byPattern[e.route] || (byPattern[e.route] = [])).push(e));
  Object.keys(byPattern).sort((a, b) => (a.includes(':') - b.includes(':')) || b.length - a.length).forEach((pt) => {
    const list = byPattern[pt];
    route(pt, () => ({ nav: '', html: `<div class="page soon"><span class="soon-ic">${icon('clock', 88)}</span><h1>${esc(list[0].id)} ${esc(list[0].title)} wird ergänzt</h1>
      <p>Diese Seite baut «${esc(LIVE_REG.AGENTS[list[0].agent].name)}» in ${esc(list[0].file)}.</p><p class="soon-path">${esc(pt)}</p>
      <button type="button" class="tb primary" data-act="back" data-f="ph-back" data-default>${icon('back', 32)}<span>Zurück</span></button></div>` }));
  });
  if (!have.has('/live/dialog/:id')) route('/live/dialog/:id', (pm) => {
    const e = LIVE_REG.get(pm.id, 'tv');
    return { nav: '', after: () => setTimeout(() => { if (!OV.length && DIALOG_DEMO[pm.id]) DIALOG_DEMO[pm.id](); }, 60), html: livePage({ tab: '', title: `${esc(pm.id)} ${esc(e ? e.title : '')}`, html: `<p class="ltv-lead">Vorschau des Dialogs mit Beispieldaten.</p><button type="button" class="tb primary" data-act="demoDialog" data-id="${esc(pm.id)}" data-f="dh-open" data-default>Dialog öffnen</button>` }) };
  });
}

/* a change of the simulation redraws the page; with an open menu the page redraws after closing */
let liveDirty = false;
SIM.on((t) => { if (t !== 'change') return; if (OV.length) liveDirty = true; else render(); });
setInterval(() => { if (liveDirty && !OV.length) { liveDirty = false; render(); } }, 400);

/* 14 4.5: the app in the background (or screen off) pauses; after 30 s the session ends and on return the app
   tunes the same channel in again (offset gone, toast «Neu eingeschaltet»). Shorter breaks keep the session. */
let lvHiddenAt = 0;
function lvBackReturn(awayMs) {
  const p = SIM.player(); if (!p) { if (awayMs >= 30000) toast('Im Mockup: zuerst einen Sender einschalten.'); return; }
  if (awayMs >= 30000) { SIM.tune(p.ch, { list: p.list }); SIM.emit('retuned'); }
}
document.addEventListener('visibilitychange', () => {
  if (document.hidden) { lvHiddenAt = Date.now(); return; }
  if (lvHiddenAt) { lvBackReturn(Date.now() - lvHiddenAt); lvHiddenAt = 0; }
});
LIVE_MOCK.addAction('App 30 s im Hintergrund (Rückkehr zum Player)', () => lvBackReturn(30000));
