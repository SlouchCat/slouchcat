/* Web Zuschauer B, part 1: helpers of this agent, extra mockup states (added to the panel «Zustände»),
   roving focus for lists and card grids, LS-04 Programmzeitschrift (grid with virtual rows, phone list
   «Jetzt und danach»), LS-07 Live-Player (timeshift bar, zapping, channel list, number entry, programme
   overlay, menus, loading and error states). 14-oberflaeche.md 4.1, 4.2, 4.4 to 4.12.
   Part 2 (viewer-b-2.js): LS-08 to LS-14, LS-10 and the dialogs LD-01 to LD-11. Global names start with «vb». */
'use strict';

/* icons vbVol, vbMute, vbFull, vbBack, vbFwd, vbSub, vbAudio, vbChapter, vbBlank: in core.js */

/* ---------- mockup states: all in the core (LIVE_SIM.flags(), bar «Zustände»); VB only reads them ---------- */
/** web player: no selectable subtitles until stage E (14 4.10); the button is greyed out (aria-disabled, focusable) */
const VB_SUB_LATER = LU.SUBS_TX.browser;
const vbSubBtn = (fid, c) => { const t = c && !(c.subs || []).length ? LU.SUBS_TX.none : c && c.subs.every(LU.isTeletext) ? LU.SUBS_TX.teletext : `${LU.SUBS_TX.later}. ${LU.SUBS_TX.browser}`; return `<button type="button" class="vbp-ib" aria-disabled="true" data-t="${esc(t)}" data-fid="${fid}" aria-label="Untertitel: ${esc(t)}">${icon('vbSub', 22)}<span class="vbp-tip" aria-hidden="true">${esc(t)}</span></button>`; };
const VB = {
  get quotaFull() { return SIM.quota().full; },
  get perr() { const f = SIM.flags().fault; return ['limit', 'notPlayable', 'busy', 'busyPerm', 'stall'].includes(f) ? f : ''; },
  get fallback() { return SIM.flags().fault === 'fallback'; },
  get slow() { return SIM.flags().fault === 'slow'; },
  get many() { return SIM.flags().many; },
  noAskZap: false, fbShown: false,
};
/* actions of this client in the bar «Zustände» (not states) */
LIVE_MOCK.addAction('Erinnerung jetzt zeigen', () => vbRemindNow());
LIVE_MOCK.addAction('Start einer geplanten Aufnahme (bei vollem Kontingent: abgelehnt)', () => vbSimReject());
LIVE_MOCK.addAction('Beim Senderwechsel wieder fragen', () => { VB.noAskZap = false; toast('Beim Senderwechsel mit Zeitversatz wird wieder gefragt.'); });

/* ---------- small helpers ---------- */
const vbWidth = () => { const d = document.getElementById('device'); const w = d ? d.clientWidth : 1280; if (typeof S !== 'undefined' && S.view === 'phone') return Math.min(w, 390); if (typeof S !== 'undefined' && S.view === 'tablet') return Math.min(w, 834); return w; };
const vbIsPhone = () => vbWidth() < 768;
const vbToday = () => Math.floor(SIM.now() / 1440);
const vbMaxDay = () => Math.floor((LIVE.EPG_END - 1) / 1440);
const vbHm = (t) => fmt.time(t);
const vbMin = (n) => `${n} ${n === 1 ? 'Minute' : 'Minuten'}`;
/** account storage as the viewer sees it; «Kontingent voll» of the mockup fills it */
const vbQuota = () => SIM.quota(); // «Kontingent voll» of the mockup is in the core
/** size estimate from the measured bitrate of the channel (Mbit/s); null = unknown */
function vbEstGB(chId, min) { const c = SIM.channel(chId); if (!c || !c.bitrate) return null; return Math.round(c.bitrate * 60 * min / 8 / 1000 * 10) / 10; }
const vbRuleWarnText = { quota: 'Warnung: Kontingent', serverStorage: 'Warnung: Speicher des Servers', overlap: 'Warnung: Überschneidung', duplicate: 'Hinweis: doppelt', channelGone: 'Warnung: Sender weg', programGone: 'Warnung: Sendung verschwunden', timeChanged: 'Zeit angepasst', dst: 'Hinweis: Zeitumstellung' };
const vbWd = (t) => (6 + Math.floor(t / 1440)) % 7; // 0 = Sunday, day 0 is a Saturday
/** concrete upcoming recordings; the core applies time window and weekdays of series rules */
const vbUpcoming = (pid) => SIM.upcoming(pid);
/** one context per drawing pass: rights, upcoming, running recordings, reminders, rules with warnings */
function vbCtx() {
  const r = SIM.rights();
  const warnRules = new Set(); SIM.warnings().forEach((w) => w.rules.forEach((id) => warnRules.add(id)));
  return { rec: r.recordings, up: r.recordings ? vbUpcoming() : [], run: r.recordings ? SIM.recordings().filter((x) => x.state === 'running') : [], rem: new Set(SIM.reminders().map((x) => x.program)), warnRules };
}
function vbRecState(p, cx) {
  if (!cx.rec || !p) return null;
  if (cx.run.some((r) => r.ch === p.ch && r.start < p.end && r.end > p.start)) return 'recording';
  const u = cx.up.find((x) => x.program === p.id || (x.ch === p.ch && x.start < p.end && x.end > p.start + 1));
  return u ? { st: u.kind === 'series' ? 'series' : 'scheduled', u } : null;
}
/** badges of a programme: [{ kind, word }] */
function vbBadges(p, cx) {
  const out = []; const rs = vbRecState(p, cx);
  if (rs === 'recording') out.push(['recording', LU.TX.recording]);
  else if (rs) { out.push([rs.st, rs.st === 'series' ? LU.TX.series : LU.TX.scheduled]); if (VB.quotaFull || cx.warnRules.has(rs.u.rule.id)) out.push(['warning', 'Warnung']); }
  if (cx.rem.has(p.id)) out.push(['reminder', LU.TX.reminder]);
  return out;
}
const vbBadgeHtml = (list, text) => list.map(([k, w]) => LU.badge(k, text ? { text: true, label: w } : null)).join('');
/** keep text of a recording */
function vbKeepText(r) {
  if (r.keep && r.keep.always) return 'Immer behalten';
  if (r.keep && r.keep.mode === 'days') { const until = r.keep.until || r.start + r.keep.days * 1440; return `Wird am ${fmt.day(until).replace(/^\S+ /, '')} gelöscht`; }
  const rule = r.rule ? SIM.rule(r.rule) : null;
  if (rule && rule.keepLast) return `Behalte die letzten ${rule.keepLast} Folgen`;
  return 'Behalten, bis du löschst';
}
/** actions for one programme (context menu, tablet sheet, phone menu, preview) */
function vbProgActions(p, o) {
  o = o || {}; const now = SIM.now(); const r = SIM.rights(); const out = [];
  const run = p.start <= now && p.end > now; const fut = p.start > now;
  if (run) out.push({ label: 'Einschalten', act: 'vbTuneP', data: { ch: p.ch, list: o.list || '' }, icon: 'play' });
  out.push({ label: 'Details', href: '#/live/programs/' + p.id, icon: 'info' });
  if (r.recordings && p.end > now) {
    const rs = SIM.recState(p.id);
    if (rs === 'recording') { const rec = SIM.runningOn(p.ch); out.push(rec ? { label: 'Nimmt auf: zur Aufnahme', href: '#/live/recordings/' + rec.id, icon: 'record' } : { label: 'Nimmt auf', act: 'vbPlan', data: { pid: p.id }, icon: 'record' }); }
    else if (rs === 'series') out.push({ label: 'Serie geplant: ändern', act: 'vbSeries', data: { pid: p.id }, icon: 'recordSeries' });
    else out.push({ label: rs === 'scheduled' ? 'Aufnahme ändern' : 'Aufnehmen', act: 'vbPlan', data: { pid: p.id }, icon: 'record' });
    if (p.series && rs !== 'series') out.push({ label: 'Serie aufnehmen', act: 'vbSeries', data: { pid: p.id }, icon: 'recordSeries' });
  }
  if (fut) {
    const rem = SIM.reminderFor(p.id);
    out.push({ label: rem ? 'Erinnerung ändern' : 'Erinnern', act: 'vbRemind', data: { pid: p.id }, icon: 'reminder' });
    if (rem) out.push({ label: 'Erinnerung entfernen', act: 'vbRemOff', data: { pid: p.id }, icon: 'x', danger: true });
  }
  if (o.channelProgram !== false) out.push({ label: 'Senderprogramm', href: '#/live/guide/channel/' + p.ch, icon: 'calendar' });
  return out;
}
ACT.vbTuneP = (d) => ACT.tune({ ch: d.ch, list: d.list || '', listName: 'filter' });
ACT.vbPlan = (d) => LiveDlg.planRecording(d.pid);
ACT.vbSeries = (d) => { const u = vbUpcoming().find((x) => x.program === d.pid && x.kind === 'series'); LiveDlg.seriesRecording(d.pid, u ? u.rule.id : undefined); };
ACT.vbRemind = (d) => LiveDlg.reminder(d.pid);
ACT.vbRemOff = (d) => { const r = SIM.reminderFor(d.pid); const lead = r ? r.lead : 5; SIM.removeReminder(d.pid); toastUndo('Erinnerung entfernt', 'vbRemUndo', { pid: d.pid, lead }); };
ACT.vbRemUndo = (d) => { SIM.setReminder(d.pid, Number(d.lead)); toast('Erinnerung wieder gesetzt.'); };
ACT.vbGo = (d) => go(d.href);

/* ---------- reminder band at the moment itself (14 3.13): stays until clicked ---------- */
function vbDrawBand() {
  let el = document.getElementById('vb-band');
  if (!el) { const ov = document.getElementById('overlays'); if (!ov) return; ov.insertAdjacentHTML('beforeend', '<div id="vb-band" role="status" aria-live="polite"></div>'); el = document.getElementById('vb-band'); }
  const b = ui.vbBand; const p = b && SIM.program(b.program);
  if (!p) { el.innerHTML = ''; return; }
  const c = SIM.channel(p.ch); const mins = Math.max(0, Math.ceil(p.start - SIM.now()));
  el.innerHTML = `<div class="vb-band">${icon('reminder', 20)}<span class="grow"><b>${esc(p.title)}</b> beginnt ${mins ? `in ${mins} Min.` : 'jetzt'} auf ${esc(c.name)}</span><button type="button" class="btn primary sm" data-act="vbBandTune" data-ch="${esc(p.ch)}" data-fid="vbband:on">Einschalten</button><button type="button" class="btn secondary sm" data-act="vbBandClose" data-fid="vbband:x">Schliessen</button></div>`;
}
function vbRemindNow() { const r = SIM.reminders()[0]; if (!r) { toast('Es ist keine Erinnerung gesetzt.', 'warn'); return; } ui.vbBand = { program: r.program }; vbDrawBand(); }
ACT.vbBandTune = (d) => { ui.vbBand = null; vbDrawBand(); const p = SIM.player(); if (p && p.ch !== d.ch && parseHash().path.startsWith('/live/watch/')) vbSwitch(d.ch, { via: 'click' }); else ACT.tune({ ch: d.ch }); };
ACT.vbBandClose = () => { ui.vbBand = null; vbDrawBand(); };
const vbBandSeen = new Set();
SIM.on((t) => {
  if (t !== 'minute' && t !== 'change') return;
  const now = SIM.now();
  SIM.reminders().forEach((r) => { const p = SIM.program(r.program); const k = r.program + ':' + r.lead; if (p && p.start - r.lead <= now && p.start > now - 1 && !vbBandSeen.has(k)) { vbBandSeen.add(k); ui.vbBand = { program: r.program }; vbDrawBand(); } });
  if (ui.vbBand) vbDrawBand();
});

/** «Kontingent voll» at the start of a planned recording: the server rejects it and both profiles get a message */
function vbSimReject() {
  const o = vbUpcoming().find((u) => SIM.chState(u.ch) !== 'gone'); if (!o) { toast('Keine geplante Aufnahme vorhanden.', 'warn'); return; }
  if (!VB.quotaFull) { toast(`«${esc(o.title)}» startet normal: Der Speicher des Kontos hat Platz. Schalte unter «Zustände» → Aufnahmen «Kontingent voll» ein.`, 'warn', 7000); return; }
  SIM.rejectForQuota(o);
  const main = SIM.profiles().find((x) => x.primary);
  toast(`«${esc(o.title)}» wurde nicht aufgenommen: ${esc(LU.REASON.quota)} ${SIM.profile().primary ? 'Du hast' : `Du und ${esc(main.name)} habt`} eine Meldung in der Glocke. <a href="#/live/recordings">Meine Aufnahmen</a>`, 'warn', 8000);
}

/* ---------- roving focus: container [data-vbrove="list|grid"] (key data-vbkey), rows [data-vbrow], main [data-vbi], extras [data-vbx] ---------- */
ui.vbRove = ui.vbRove || {};
/** tabindex of the main element of one row: one tab stop per container */
function vbTab(key, id, ids) { const a = ids.includes(ui.vbRove[key]) ? ui.vbRove[key] : ids[0]; return id === a ? '0' : '-1'; }
document.addEventListener('keydown', (e) => {
  const box = e.target.closest && e.target.closest('[data-vbrove]'); if (!box || ui.menu || ui.dialog) return;
  if (e.target.closest('.lmenu') || e.target.matches('input, select, textarea')) return;
  const row = e.target.closest('[data-vbrow]'); if (!row || !box.contains(row)) return;
  const rows = $$('[data-vbrow]', box).filter((r) => r.closest('[data-vbrove]') === box && r.offsetParent !== null);
  const k = rows.indexOf(row); if (k < 0) return;
  const parts = $$('[data-vbi], [data-vbx]', row).filter((x) => x.closest('[data-vbrow]') === row && x.offsetParent !== null && !x.disabled);
  const pi = parts.indexOf(e.target);
  const grid = box.dataset.vbrove === 'grid';
  const to = (n) => { n = Math.max(0, Math.min(rows.length - 1, n)); const m = $$('[data-vbi]', rows[n]).find((x) => x.closest('[data-vbrow]') === rows[n]); if (!m) return; $$('[data-vbi]', box).forEach((x) => x.setAttribute('tabindex', '-1')); m.setAttribute('tabindex', '0'); ui.vbRove[box.dataset.vbkey] = rows[n].dataset.vbrow; m.focus(); m.scrollIntoView({ block: 'nearest' }); };
  const cols = grid ? Math.max(1, rows.filter((x) => x.offsetTop === rows[0].offsetTop).length) : 1;
  const K = e.key;
  if (K === 'ArrowDown') { e.preventDefault(); to(k + cols); }
  else if (K === 'ArrowUp') { e.preventDefault(); to(k - cols); }
  else if (K === 'Home') { e.preventDefault(); to(0); }
  else if (K === 'End') { e.preventDefault(); to(rows.length - 1); }
  else if (K === 'PageDown') { e.preventDefault(); to(k + 6 * cols); }
  else if (K === 'PageUp') { e.preventDefault(); to(k - 6 * cols); }
  else if (K === 'ArrowRight') { e.preventDefault(); if (grid && pi <= 0) to(k + 1); else if (parts[pi + 1]) parts[pi + 1].focus(); }
  else if (K === 'ArrowLeft') { e.preventDefault(); if (grid && pi <= 0) to(k - 1); else if (pi > 0) parts[pi - 1].focus(); }
});
document.addEventListener('focusin', (e) => {
  const m = e.target.closest && e.target.closest('[data-vbi]'); if (!m) return;
  const box = m.closest('[data-vbrove]'); const row = m.closest('[data-vbrow]'); if (!box || !row) return;
  $$('[data-vbi]', box).forEach((x) => x.setAttribute('tabindex', x === m ? '0' : '-1'));
  ui.vbRove[box.dataset.vbkey] = row.dataset.vbrow;
});

/* ======================================================================================
   LS-04 Programmzeitschrift
   ====================================================================================== */
const VBG_PX = 6, VBG_ROW = 64, VBG_AX = 40;
const VBG_TIMES = [['06:00', 'Morgen 06:00'], ['12:00', 'Mittag 12:00'], ['18:00', 'Vorabend 18:00'], ['20:15', 'Hauptabend 20:15'], ['22:00', 'Spätabend 22:00'], ['00:00', 'Nacht 00:00']];
function vbgState(inst) { const k = inst === 'ov' ? 'vbgo' : 'vbg'; return ui[k] || (ui[k] = { key: '', top: 0, left: null, f: null, jumpT: null }); }
/** rows of the grid; «Viele Sender» adds test rows (copies of the real channels) to show the virtual scrolling */
function vbgRows(group) {
  const list = SIM.channels(group ? { group } : {});
  const rows = list.map((c) => ({ id: c.id, c, src: c.id }));
  if (VB.many && list.length) for (let i = 0; rows.length < 2000; i++) { const s = list[i % list.length]; rows.push({ id: 'syn' + i, c: { id: 'syn' + i, no: 100 + i, name: `Testsender ${100 + i}`, slug: 'none' }, src: s.id, synth: true }); }
  return rows;
}
/** programmes and gaps of one row for one day, sorted */
function vbgItems(row, d0, d1) {
  const pre = row.synth ? row.id + '|' : '';
  const all = SIM.programsOf(row.src, d0, d1).map((p) => ({ kind: 'p', p, key: pre + p.id, s: Math.max(p.start, d0), e: Math.min(p.end, d1) }))
    .concat(SIM.gapsOf(row.src, d0, d1).map(([a, b]) => ({ kind: 'g', key: `g:${row.id}:${a}`, s: a, e: b })));
  return all.filter((x) => x.e > x.s).sort((a, b) => a.s - b.s);
}
function vbgHtml(inst, o) {
  const st = vbgState(inst);
  const rows = vbgRows(o.group);
  const d0 = o.day * 1440, d1 = d0 + 1440;
  const key = `${o.day}|${o.group}|${VB.many}|${o.colW}|${rows.length}`;
  if (st.key !== key) { st.key = key; st.top = 0; st.left = null; st.f = null; }
  st._ = { rows, d0, d1, colW: o.colW, inst, day: o.day, group: o.group };
  if (st.want && rows.length) { const w = st.want; st.want = null; let ri = w.ch ? rows.findIndex((r) => r.id === w.ch) : Math.min(rows.length - 1, w.row || 0); if (ri < 0) ri = 0; const it = vbgAt(st._, ri, w.t); if (it) st.f = { row: ri, key: it.key, t: w.t }; st.top = Math.max(0, ri * VBG_ROW - 2 * VBG_ROW); }
  const W = o.colW + 1440 * VBG_PX, H = VBG_AX + rows.length * VBG_ROW;
  const now = SIM.now();
  let ticks = '';
  for (let m = 0; m < 1440; m += 15) { const x = o.colW + m * VBG_PX; ticks += m % 30 === 0 ? `<span class="vbg-tick major" style="left:${x}px"><b>${fmt.time(d0 + m)}</b></span>` : `<span class="vbg-tick" style="left:${x}px"></span>`; }
  const today = now >= d0 && now < d1; const nx = today ? o.colW + (now - d0) * VBG_PX : 0;
  return `<div class="vbg-scroll" data-vbg="${inst}" style="--vbg-col:${o.colW}px;height:min(${o.maxH}, ${H + 2}px)" role="grid" aria-label="Programm ${esc(fmt.dayLong(d0))}. Pfeiltasten wechseln die Sendung, Enter öffnet Details, Leertaste schaltet ein" aria-rowcount="${rows.length + 1}">
    <div class="vbg-canvas" style="width:${W}px;height:${H}px">
      <div class="vbg-axis" role="row" aria-rowindex="1" style="width:${W}px"><span class="vbg-corner" role="columnheader">${o.colW > 120 ? 'Sender' : 'Nr.'}</span><span aria-hidden="true">${ticks}${today ? `<span class="vbg-nowlbl" data-vbgnowlbl style="left:${nx}px">${fmt.time(now)}</span>` : ''}</span></div>
      ${today ? `<span class="vbg-now" data-vbgnow style="left:${nx}px" aria-hidden="true"></span>` : ''}
      <div class="vbg-rows" role="rowgroup"></div>
    </div></div>`;
}
function vbgCell(st, g, i, it, now, whole, cx, fKey) {
  const row = g.rows[i]; const left = g.colW + (it.s - g.d0) * VBG_PX; const w = Math.max(22, (it.e - it.s) * VBG_PX - 2);
  const tab = fKey === it.key ? 0 : -1;
  const pos = `data-key="${esc(it.key)}" data-row="${i}" tabindex="${tab}" style="left:${left}px;width:${w}px"`;
  if (it.kind === 'g') {
    const txt = whole ? LU.TX.epgNoneButPlays : LU.TX.epgNone;
    return `<div class="vbg-cell gap lv-gap${w < 90 ? ' tiny' : ''}" role="gridcell" ${pos} aria-label="${esc(row.c.name)}, ${esc(txt)}, ${fmt.time(it.s)} bis ${fmt.time(it.e)}" title="${esc(txt)}"><span class="t"><span class="lv-gap-text">${esc(txt)}</span></span></div>`;
  }
  const p = it.p; const past = p.end <= now; const cur = p.start <= now && p.end > now;
  const bl = row.synth ? [] : vbBadges(p, cx);
  const words = (cur ? ', läuft' : past ? ', vorbei' : '') + bl.map(([, w2]) => ', ' + w2).join('');
  return `<div class="vbg-cell${cur ? ' cur' : past ? ' past' : ''}${w < 70 ? ' tiny' : ''}" role="gridcell" ${pos} aria-label="${esc(row.c.name)}, ${esc(p.title)}, ${fmt.time(p.start)} bis ${fmt.time(p.end)}${esc(words)}" title="${esc(p.title)}${p.sub ? ' · «' + esc(p.sub) + '»' : ''} · ${fmt.range(p.start, p.end)}"><span class="t">${esc(p.title)}</span><span class="m"><span class="tm">${fmt.time(p.start)}</span>${vbBadgeHtml(bl)}</span>${cur ? LU.progress(p.start, p.end, { cls: 'vbg-prog', label: 'Fortschritt' }) : ''}</div>`;
}
function vbgRowHtml(st, g, i, tA, tB, cx, now, mode) {
  const row = g.rows[i]; const c = row.c;
  const bad = !row.synth && SIM.chState(c) === 'disturbed';
  const fKey = st.f ? st.f.key : null;
  const head = `<div class="vbg-ch" role="rowheader"><button type="button" class="vbg-chb" data-key="ch:${i}" data-row="${i}" tabindex="${fKey === 'ch:' + i ? 0 : -1}" aria-label="${c.no} ${esc(c.name)} einschalten${bad ? ', gestört' : ''}"><span class="no">${c.no}</span>${LU.logo(c)}<span class="nm">${esc(c.name)}${bad ? `<span class="vbg-bad"><span class="dot warn"></span>${esc(LU.TX.disturbed)}</span>` : ''}</span></button></div>`;
  let cells = '';
  if (mode === 'loading') { for (let t = Math.floor(tA / 60) * 60; t < tB; t += 60) { if (t < g.d0 || t >= g.d1) continue; cells += `<span class="vbg-cell skel" style="left:${g.colW + (t - g.d0) * VBG_PX}px;width:${60 * VBG_PX - 2}px" aria-hidden="true"><span class="lv-skel"></span></span>`; } }
  else {
    const items = vbgItems(row, g.d0, g.d1); const whole = !items.some((x) => x.kind === 'p');
    items.forEach((it) => { if ((it.e <= tA || it.s >= tB) && it.key !== fKey) return; cells += vbgCell(st, g, i, it, now, whole, cx, fKey); });
  }
  return `<div class="vbg-row${bad ? ' bad' : ''}" role="row" aria-rowindex="${i + 2}" style="top:${VBG_AX + i * VBG_ROW}px">${head}${cells}</div>`;
}
const vbgAt = (g, r, t) => { const its = vbgItems(g.rows[r], g.d0, g.d1); return its.find((x) => x.s <= t && x.e > t) || its[its.length - 1] || null; };
function vbgDraw(sc, force, noRefocus) {
  const st = vbgState(sc.dataset.vbg); const g = st._; if (!g) return;
  const rowsEl = $('.vbg-rows', sc); if (!rowsEl) return;
  if (!g.rows.length) { rowsEl.innerHTML = ''; return; }
  const top = sc.scrollTop, h = sc.clientHeight || 400, left = sc.scrollLeft, w = sc.clientWidth || 800;
  const r0 = Math.max(0, Math.floor((top - VBG_AX) / VBG_ROW) - 3), r1 = Math.min(g.rows.length - 1, Math.ceil((top + h) / VBG_ROW) + 3);
  const tA = g.d0 + left / VBG_PX - 150, tB = g.d0 + (left + w) / VBG_PX + 150;
  const bucket = `${r0}:${r1}:${Math.floor(tA / 60)}`;
  if (!force && bucket === st.bucket) return;
  st.bucket = bucket;
  if (!st.f && liveMode() !== 'loading') {
    const fr = Math.min(g.rows.length - 1, Math.max(0, Math.round(top / VBG_ROW)));
    const t = st.jumpT != null && st.jumpT >= g.d0 && st.jumpT < g.d1 ? st.jumpT : Math.max(g.d0, Math.min(g.d1 - 1, SIM.now()));
    const it = vbgAt(g, fr, t); if (it) st.f = { row: fr, key: it.key, t };
  }
  const act = document.activeElement; const had = act && rowsEl.contains(act) ? act.dataset.key : null;
  const cx = vbCtx(); const now = SIM.now(); const mode = liveMode();
  const idx = []; for (let i = r0; i <= r1; i++) idx.push(i);
  if (st.f && st.f.row < g.rows.length && !idx.includes(st.f.row)) { idx.push(st.f.row); idx.sort((a, b) => a - b); }
  rowsEl.innerHTML = idx.map((i) => vbgRowHtml(st, g, i, tA, tB, cx, now, mode)).join('');
  LU.refresh(rowsEl);
  if (had && !noRefocus) { const el = rowsEl.querySelector(`[data-key="${CSS.escape(had)}"]`); if (el) el.focus({ preventScroll: true }); }
  if (st.ctx) vbgCtxDraw(sc);
}
function vbgMount(root) {
  $$('.vbg-scroll', root).forEach((sc) => {
    const st = vbgState(sc.dataset.vbg); const g = st._; if (!g) return;
    if (st.left == null) { const t = st.jumpT != null && st.jumpT >= g.d0 && st.jumpT < g.d1 ? st.jumpT : (SIM.now() >= g.d0 && SIM.now() < g.d1 ? SIM.now() : g.d0 + 20 * 60 + 15); st.left = Math.max(0, (t - g.d0) * VBG_PX - (sc.clientWidth - g.colW) / 4); }
    sc.scrollLeft = st.left; sc.scrollTop = st.top;
    st.bucket = null;
    vbgDraw(sc, true);
    sc.addEventListener('scroll', () => { st.left = sc.scrollLeft; st.top = sc.scrollTop; if (!st.raf) st.raf = requestAnimationFrame(() => { st.raf = 0; vbgDraw(sc); }); }, { passive: true });
    if (st.refocus || st.focusNow) { st.refocus = false; st.focusNow = false; const el = $('.vbg-rows [tabindex="0"]', sc); if (el) el.focus({ preventScroll: true }); }
    vbgPreview();
  });
}
/** move the roving focus; keeps the focus time t for vertical moves (14 4.1) */
function vbgFocus(sc, row, it, t) {
  const st = vbgState(sc.dataset.vbg); const g = st._;
  st.f = { row, key: it ? it.key : 'ch:' + row, t: t != null ? t : (st.f ? st.f.t : g.d0) };
  const y = VBG_AX + row * VBG_ROW;
  if (y < sc.scrollTop + VBG_AX) sc.scrollTop = Math.max(0, y - VBG_AX); else if (y + VBG_ROW > sc.scrollTop + sc.clientHeight) sc.scrollTop = y + VBG_ROW - sc.clientHeight;
  if (it) { const x0 = g.colW + (it.s - g.d0) * VBG_PX, x1 = g.colW + (it.e - g.d0) * VBG_PX; const vs = sc.scrollLeft + g.colW, ve = sc.scrollLeft + sc.clientWidth; if (x1 < vs + 24 || x0 > ve - 48) sc.scrollLeft = Math.max(0, x0 - g.colW - 24); }
  st.left = sc.scrollLeft; st.top = sc.scrollTop;
  vbgDraw(sc, true, true);
  const el = $(`.vbg-rows [data-key="${CSS.escape(st.f.key)}"]`, sc); if (el) el.focus({ preventScroll: true });
  vbgPreview();
}
function vbgListIds(g) { return g.rows.filter((r) => !r.synth).map((r) => r.id).join(','); }
function vbgTune(g, row) {
  if (row.synth) { toast('Testzeile für das virtuelle Scrollen. Einschalten geht nur bei den echten Sendern.', 'warn'); return; }
  if (g.inst === 'ov') { vbgState('ov').detail = null; vbpS().guide = false; vbSwitch(row.id, { via: 'click' }); return; }
  ACT.tune({ ch: row.id, list: vbgListIds(g), listName: g.group === 'fav' ? 'fav' : g.group || 'all' });
}
function vbgOpen(sc, el, viaSpace) {
  const st = vbgState(sc.dataset.vbg); const g = st._; const row = g.rows[Number(el.dataset.row)]; if (!row) return;
  if (el.classList.contains('vbg-chb')) { vbgTune(g, row); return; }
  const it = vbgItems(row, g.d0, g.d1).find((x) => x.key === el.dataset.key); if (!it) return;
  const now = SIM.now();
  if (row.synth) { toast('Testzeile für das virtuelle Scrollen. Details gibt es nur bei den echten Sendern.', 'warn'); return; }
  if (it.kind === 'g') {
    if (it.s <= now && it.e > now) vbgTune(g, row);
    else if (SIM.rights().recordings && it.e > now) LiveDlg.manualRecording({ ch: row.id, start: Math.max(it.s, Math.ceil(now / 15) * 15), end: Math.max(it.s, Math.ceil(now / 15) * 15) + 60 });
    else toast(`${esc(row.c.name)}: ${esc(LU.TX.epgNone)}.`, 'warn');
    return;
  }
  const running = it.p.start <= now && it.p.end > now;
  if (viaSpace && running) { vbgTune(g, row); return; }
  if (g.inst === 'ov') { if (running) vbgTune(g, row); else { st.detail = it.p.id; render(); } return; }
  if (vbWidth() < 1100) openDialog('vbProg', { pid: it.p.id, list: vbgListIds(g) });
  else go('#/live/programs/' + it.p.id);
}
/** jump to a time (day change when needed) */
function vbgJump(sc, t, row) {
  const st = vbgState(sc.dataset.vbg); const g = st._;
  const day = Math.floor(t / 1440);
  if (day < vbToday() || day > vbMaxDay()) { toast(day < vbToday() ? 'Vergangene Tage gibt es nicht. Slouchcat bietet keine verpassten Sendungen an.' : 'Die Programmdaten reichen bis Fr 4.12.', 'warn'); return; }
  if (day !== g.day) {
    st.jumpT = t; st.focusNow = true; st.want = { row, t };
    if (g.inst === 'ov') { st.day = day; render(); }
    else { const q = parseHash().query; vbgSetQuery(Object.assign({}, q, { day: String(day), time: '' })); }
    return;
  }
  const it = vbgAt(g, row, t); sc.scrollLeft = Math.max(0, (t - g.d0) * VBG_PX - (sc.clientWidth - g.colW) / 4); vbgFocus(sc, row, it, t);
}
function vbgSetQuery(q) {
  const qs = ['day', 'time', 'group'].filter((k) => q[k]).map((k) => `${k}=${encodeURIComponent(q[k])}`).join('&');
  history.replaceState(null, '', '#/live/guide' + (qs ? '?' + qs : '')); render();
}
document.addEventListener('keydown', (e) => {
  const el = e.target.closest && e.target.closest('.vbg-cell, .vbg-chb'); if (!el || ui.dialog || ui.menu) return;
  const sc = el.closest('.vbg-scroll'); if (!sc) return;
  const st = vbgState(sc.dataset.vbg); const g = st._; if (!g) return;
  if (st.ctx && e.key !== 'Escape') return;
  const row = Number(el.dataset.row); const items = vbgItems(g.rows[row], g.d0, g.d1);
  const isCh = el.classList.contains('vbg-chb');
  const k = isCh ? -1 : items.findIndex((x) => x.key === el.dataset.key);
  const t = st.f && st.f.t != null ? st.f.t : (k >= 0 ? items[k].s : g.d0);
  const K = e.key; const last = g.rows.length - 1;
  const perPage = Math.max(1, Math.floor((sc.clientHeight - VBG_AX) / VBG_ROW) - 1);
  const vis = () => g.d0 + (sc.scrollLeft) / VBG_PX + 30;
  if (K === 'ContextMenu' || (K === 'F10' && e.shiftKey)) { e.preventDefault(); if (!isCh) vbgCtxOpen(sc, el); return; }
  if ((e.ctrlKey || e.metaKey) && (K === 'ArrowLeft' || K === 'ArrowRight')) { e.preventDefault(); vbgJump(sc, t + (K === 'ArrowRight' ? 120 : -120), row); return; }
  if (e.altKey && (K === 'PageUp' || K === 'PageDown')) { e.preventDefault(); vbgJump(sc, t + (K === 'PageDown' ? 1440 : -1440), row); return; }
  switch (K) {
    case 'ArrowRight': e.preventDefault(); if (isCh) { const it = vbgAt(g, row, Math.max(g.d0, Math.min(g.d1 - 1, Math.max(t, vis())))); vbgFocus(sc, row, it, it.s); } else if (items[k + 1]) vbgFocus(sc, row, items[k + 1], items[k + 1].s); else vbgJump(sc, g.d1, row); break;
    case 'ArrowLeft': e.preventDefault(); if (k > 0) vbgFocus(sc, row, items[k - 1], items[k - 1].s); else if (k === 0) vbgFocus(sc, row, null, t); break;
    case 'ArrowDown': case 'ArrowUp': { e.preventDefault(); const r = Math.max(0, Math.min(last, row + (K === 'ArrowDown' ? 1 : -1))); if (isCh) vbgFocus(sc, r, null, t); else vbgFocus(sc, r, vbgAt(g, r, t), t); break; }
    case 'PageDown': case 'PageUp': { e.preventDefault(); const r = Math.max(0, Math.min(last, row + (K === 'PageDown' ? perPage : -perPage))); if (isCh) vbgFocus(sc, r, null, t); else vbgFocus(sc, r, vbgAt(g, r, t), t); break; }
    case 'Home': e.preventDefault(); if (g.inst === 'ov' || g.day === vbToday()) vbgJump(sc, SIM.now(), row); else vbgJump(sc, SIM.now(), 0); break;
    case 'End': e.preventDefault(); if (items.length) vbgFocus(sc, row, items[items.length - 1], items[items.length - 1].s); break;
    case 'Enter': e.preventDefault(); vbgOpen(sc, el, false); break;
    case ' ': e.preventDefault(); vbgOpen(sc, el, true); break;
    case 'r': case 'R': { if (isCh || e.ctrlKey || e.metaKey) return; const it = items[k]; if (!it || it.kind !== 'p' || g.rows[row].synth) return; e.preventDefault(); if (!SIM.rights().recordings) return; if (it.p.end <= SIM.now()) toast(LU.TX.past, 'warn'); else LiveDlg.planRecording(it.p.id); break; }
    case 'e': case 'E': { if (isCh || e.ctrlKey || e.metaKey) return; const it = items[k]; if (!it || it.kind !== 'p' || g.rows[row].synth) return; e.preventDefault(); if (it.p.start <= SIM.now()) toast('Erinnern geht nur bei Sendungen, die noch nicht begonnen haben.', 'warn'); else if (SIM.reminderFor(it.p.id)) ACT.vbRemOff({ pid: it.p.id }); else LiveDlg.reminder(it.p.id); break; }
    case 'Escape': if (st.ctx) { e.preventDefault(); st.ctx = null; vbgCtxDraw(sc); el.focus(); } else if (g.inst === 'page') { e.preventDefault(); const b = $('[data-fid="vbg:now"]'); if (b) b.focus(); } break;
    default:
  }
});
document.addEventListener('focusin', (e) => {
  const el = e.target.closest && e.target.closest('.vbg-cell, .vbg-chb'); if (!el) return;
  const sc = el.closest('.vbg-scroll'); const st = vbgState(sc.dataset.vbg); const g = st._; if (!g) return;
  $$('.vbg-rows [tabindex="0"]', sc).forEach((x) => x !== el && x.setAttribute('tabindex', '-1')); el.setAttribute('tabindex', '0');
  const row = Number(el.dataset.row);
  if (!st.f || st.f.key !== el.dataset.key) { const it = vbgItems(g.rows[row], g.d0, g.d1).find((x) => x.key === el.dataset.key); st.f = { row, key: el.dataset.key, t: it ? Math.max(it.s, Math.min(it.e - 1, st.f && st.f.t != null ? st.f.t : it.s)) : (st.f ? st.f.t : g.d0) }; }
  vbgPreview();
});
document.addEventListener('click', (e) => {
  const el = e.target.closest && e.target.closest('.vbg-cell:not(.skel), .vbg-chb');
  $$('.vbg-scroll').forEach((sc) => { const st = vbgState(sc.dataset.vbg); if (st.ctx && !e.target.closest('.vbg-ctx')) { st.ctx = null; vbgCtxDraw(sc); } });
  if (!el) return;
  const sc = el.closest('.vbg-scroll'); if (sc) vbgOpen(sc, el, false);
});
document.addEventListener('contextmenu', (e) => {
  const el = e.target.closest && e.target.closest('.vbg-cell:not(.skel)'); if (!el) return;
  e.preventDefault(); el.focus({ preventScroll: true }); vbgCtxOpen(el.closest('.vbg-scroll'), el);
});
/* context menu of a cell: Einschalten · Details · Aufnehmen · Serie aufnehmen · Erinnern */
function vbgCtxOpen(sc, el) {
  const st = vbgState(sc.dataset.vbg); const g = st._; const row = g.rows[Number(el.dataset.row)];
  const it = vbgItems(row, g.d0, g.d1).find((x) => x.key === el.dataset.key); if (!it) return;
  if (it.kind !== 'p' || row.synth) { toast(it.kind === 'p' ? 'Testzeile ohne Aktionen.' : `${esc(LU.TX.epgNone)}.`, 'warn'); return; }
  st.ctx = { key: it.key, pid: it.p.id, row: Number(el.dataset.row), x: el.offsetLeft, y: VBG_AX + Number(el.dataset.row) * VBG_ROW + VBG_ROW - 4 };
  vbgCtxDraw(sc);
  const f = $('.vbg-ctx [role="menuitem"]', sc); if (f) f.focus();
}
function vbgCtxDraw(sc) {
  const st = vbgState(sc.dataset.vbg); const old = $('.vbg-ctx', sc); if (old) old.remove();
  if (!st.ctx) return;
  const p = SIM.program(st.ctx.pid); if (!p) return;
  const items = vbProgActions(p, { list: vbgListIds(st._) });
  const attrs = (d) => Object.entries(d || {}).map(([k, v]) => `data-${k}="${esc(v)}"`).join(' ');
  const x = Math.max(st._.colW + 4, Math.min(st.ctx.x, sc.scrollLeft + sc.clientWidth - 260));
  $('.vbg-canvas', sc).insertAdjacentHTML('beforeend', `<span class="lmenu vbg-ctx" role="menu" aria-label="Aktionen für ${esc(p.title)}" style="left:${x}px;top:${st.ctx.y}px">${items.map((it) => it.href
    ? `<a role="menuitem" href="${esc(it.href)}" data-act="vbgCtxClose">${icon(it.icon, 16)}${esc(it.label)}</a>`
    : `<button type="button" role="menuitem" class="${it.danger ? 'dg' : ''}" data-act="vbgCtxDo" data-do="${esc(it.act)}" ${attrs(it.data)}>${icon(it.icon, 16)}${esc(it.label)}</button>`).join('')}</span>`);
  const menu = $('.vbg-ctx', sc);
  const rowTop = st.ctx.y - VBG_ROW + 4; if (st.ctx.y + menu.offsetHeight > sc.scrollTop + sc.clientHeight && rowTop - menu.offsetHeight > sc.scrollTop) menu.style.top = (rowTop - menu.offsetHeight) + 'px';
  menu.addEventListener('keydown', (e) => { if (e.key === 'Escape' || e.key === 'Tab') { e.preventDefault(); e.stopPropagation(); const k = st.ctx.key; st.ctx = null; menu.remove(); const c = $(`.vbg-rows [data-key="${CSS.escape(k)}"]`, sc); if (c) c.focus(); } });
}
ACT.vbgCtxClose = () => { $$('.vbg-scroll').forEach((sc) => { vbgState(sc.dataset.vbg).ctx = null; }); };
ACT.vbgCtxDo = (d, el, e) => { ACT.vbgCtxClose(); const fn = ACT[d.do]; if (fn) fn(d, el, e); render(); };
/* focus preview (computer only, closed by default, the head carries the summary) */
function vbgPreview() {
  const box = document.querySelector('[data-vbprev]'); if (!box) return;
  const st = vbgState('page'); const g = st._; if (!g || !st.f) return;
  const row = g.rows[st.f.row]; const it = row && vbgItems(row, g.d0, g.d1).find((x) => x.key === st.f.key);
  const mt = box.querySelector('.mt'); const bb = box.querySelector('.bb');
  if (!it || it.kind !== 'p') { if (mt) mt.textContent = row ? `${row.c.name} · ${it ? LU.TX.epgNone : ''}` : ''; if (bb && isOpen('vbg-prev')) bb.innerHTML = `<p class="muted">${row ? esc(row.c.name) + ': ' : ''}${esc(LU.TX.epgNone)}.</p>`; return; }
  const p = it.p;
  if (mt) mt.textContent = `${p.title} · ${row.c.name} · ${fmt.range(p.start, p.end)}`;
  if (bb && isOpen('vbg-prev')) {
    const acts = row.synth ? [] : vbProgActions(p, { list: vbgListIds(g), channelProgram: false }).filter((a) => a.act && a.icon !== 'x');
    bb.innerHTML = `<div class="vbg-pv"><div class="vbg-pv-t"><b>${esc(p.title)}</b>${p.sub ? ` <span class="sec">«${esc(p.sub)}»</span>` : ''}<span class="muted"> · ${esc(row.c.name)} · ${fmt.when(p.start)}–${fmt.time(p.end)} · ${esc(p.genre || '')}</span></div><p class="vbg-pv-d">${esc(p.desc || 'Keine Beschreibung in den Programmdaten.')}</p><div class="row-gap">${acts.map((a) => `<button type="button" class="btn secondary sm" data-act="${a.act}" ${Object.entries(a.data || {}).map(([k, v]) => `data-${k}="${esc(v)}"`).join(' ')} data-fid="vbpv:${a.act}">${icon(a.icon, 16)}${esc(a.label)}</button>`).join('')}<a class="btn ghost sm" href="#/live/programs/${esc(p.id)}" data-fid="vbpv:det">${icon('info', 16)}Details</a></div></div>`;
  }
}
/* tablet: tapping a cell opens the details as a sheet with the buttons */
DIALOGS.vbProg = (pr) => {
  const p = SIM.program(pr.pid); if (!p) return { title: 'Sendung', body: `<p class="lead">Diese Sendung steht nicht mehr im Programm.</p>`, actions: '<button type="button" class="btn secondary" data-act="close" data-autofocus>Schliessen</button>' };
  const c = SIM.channel(p.ch); const now = SIM.now(); const run = p.start <= now && p.end > now;
  const acts = vbProgActions(p, { list: pr.list });
  const btn = (a, k) => a.href ? `<a class="btn ${k ? 'secondary' : 'primary'}" href="${esc(a.href)}" data-act="close" data-fid="vbps:${k}">${icon(a.icon, 18)}${esc(a.label)}</a>` : `<button type="button" class="btn ${k ? 'secondary' : 'primary'}${a.danger ? ' danger' : ''}" data-act="vbSheetDo" data-do="${esc(a.act)}" ${Object.entries(a.data || {}).map(([x, v]) => `data-${x}="${esc(v)}"`).join(' ')} data-fid="vbps:${k}"${k === 0 ? ' data-autofocus' : ''}>${icon(a.icon, 18)}${esc(a.label)}</button>`;
  return { title: esc(p.title), body: `<div class="vb-sheet-h">${LU.logo(c)}<div><p class="sec">${p.sub ? '«' + esc(p.sub) + '» · ' : ''}${esc(c.name)} · ${fmt.when(p.start)}–${fmt.time(p.end)}</p><p class="muted">${esc(p.genre || '')}${run ? ' · ' + LU.nowText('left', p.end) : p.end <= now ? ' · ' + esc(LU.TX.past) : ''}</p></div></div>${run ? LU.progress(p.start, p.end) : ''}${p.desc ? `<p class="lead">${esc(p.desc)}</p>` : ''}<div class="vb-sheet-acts">${acts.map(btn).join('')}</div>`,
    actions: `<button type="button" class="btn ghost" data-act="close" data-fid="vbps:x">Schliessen</button>` };
};
ACT.vbSheetDo = (d, el, e) => { closeDialog(); const fn = ACT[d.do]; if (fn) setTimeout(() => fn(d, el, e), 0); };

/* ---------- LS-04 page ---------- */
function vbgDays(day, act) {
  const out = []; for (let d = vbToday(); d <= vbMaxDay(); d++) out.push(`<button type="button" class="pill vb-pill" aria-pressed="${d === day}" data-act="${act}" data-day="${d}" data-fid="vbgd:${d}">${d === vbToday() ? 'Heute' : d === vbToday() + 1 ? 'Morgen' : fmt.day(d * 1440)}</button>`);
  return `<div class="vbg-days" role="group" aria-label="Tag">${out.join('')}</div>`;
}
function vbgGroupSel(group, fid) {
  const opts = [['', 'Alle Sender'], ['fav', 'Meine Sender'], ...LIVE.groups.map((x) => [x.id, x.name])];
  return `<label class="fld fsel"><span class="lbl">Gruppe</span><select class="sel" data-vbgsel="group" data-fid="${fid}" aria-label="Gruppe">${opts.map(([v, t]) => `<option value="${esc(v)}"${v === group ? ' selected' : ''}>${esc(t)}</option>`).join('')}</select></label>`;
}
route('/live/guide', (pm, q) => {
  const today = vbToday();
  const day = q.day ? Math.max(today, Math.min(vbMaxDay(), Number(q.day) || today)) : today;
  const group = q.group || '';
  const phone = vbIsPhone(); const mode = liveMode();
  const total = SIM.channels().length; const list = SIM.channels(group ? { group } : {});
  const tq = q.time && /^\d\d:\d\d$/.test(q.time) ? q.time : '';
  if (phone) return vbnPage(day, tq, group, list, total, mode);
  const st = vbgState('page');
  const act = document.activeElement; if (act && act.closest && act.closest('.vbg-rows')) st.refocus = true;
  if (tq && st.lastTq !== `${day}|${tq}`) { st.lastTq = `${day}|${tq}`; st.jumpT = day * 1440 + LIVE.hm(tq); st.left = null; st.f = null; }
  else if (!tq) st.lastTq = '';
  const colW = vbWidth() < 1100 ? 96 : 200;
  const preset = VBG_TIMES.some(([v]) => v === tq);
  const timeSel = `<label class="fld fsel"><span class="lbl">Uhrzeit</span><select class="sel" data-vbgsel="time" data-fid="vbg:time" aria-label="Uhrzeit-Sprung"><option value=""${!tq ? ' selected' : ''}>${day === today ? 'Jetzt' : 'Uhrzeit wählen'}</option>${VBG_TIMES.map(([v, t]) => `<option value="${v}"${v === tq ? ' selected' : ''}>${t}</option>`).join('')}${tq && !preset ? `<option value="${tq}" selected>${tq}</option>` : ''}<option value="other">Andere Uhrzeit …</option></select></label>`;
  const tools = `<div class="vbg-tools">${vbgDays(day, 'vbgDay')}<div class="vbg-row2"><button type="button" class="btn secondary" data-act="vbgNow" data-fid="vbg:now">${icon('clock', 18)}Jetzt</button>${timeSel}${vbgGroupSel(group, 'vbg:group')}<a class="btn ghost" href="#/live/guide/search" data-fid="vbg:search">${icon('search', 18)}Im Programm suchen</a></div></div>`;
  let body = '';
  if (mode === 'empty' || !total) body = emptyState('Keine Sender', LU.TX.empty);
  else if (!list.length) body = emptyState('Keine Sender in dieser Gruppe', group === 'fav' ? 'Du hast noch keine Lieblingssender. Markiere Sender mit dem Herz in der Senderübersicht.' : 'Wähle eine andere Gruppe.', `<button type="button" class="btn secondary" data-act="vbgGroup" data-v="" data-fid="vbg:all">Alle Sender</button>`);
  else {
    const notes = (mode === 'error' ? notice('bad', 'Die Programmdaten konnten nicht aktualisiert werden.', 'Angezeigt wird der Stand von heute 06:00. Fernsehen geht trotzdem.', `<button type="button" class="btn secondary sm" data-act="liveRetry" data-fid="err:retry">${icon('refresh', 16)}Erneut versuchen</button>`) : '')
      + (SIM.flags().noEpg ? notice('', 'Für die Sender gibt es gerade keine Programmdaten.', 'Fernsehen geht trotzdem. Das Raster zeigt «Keine Programmdaten».') : '');
    const prev = colW > 120 ? band('vbg-prev', 'Vorschau', '', '<p class="muted">Wähle eine Sendung im Raster.</p>', { viewer: true, icon: 'eye' }).replace('<section class="bandp"', '<section class="bandp vbg-prevband" data-vbprev') : '';
    body = notes + prev + vbgHtml('page', { day, group, colW, maxH: 'max(360px, calc(var(--dh) - 330px))' })
      + `<p class="vbg-legend" aria-label="Zeichenerklärung">${SIM.rights().recordings ? `${LU.badge('scheduled', { text: true })}${LU.badge('recording', { text: true })}${LU.badge('series', { text: true })}${LU.badge('warning', { text: true })}` : ''}${LU.badge('reminder', { text: true })}<span class="vbg-leg-gap"><span class="lv-gap"></span>${esc(LU.TX.epgNone)}</span><span class="muted">Tasten: Pfeile, Bild↑/↓, Pos1 = Jetzt, Strg+←/→ = 2 Std., Alt+Bild↑/↓ = Tag, Enter = Details, Leertaste = Einschalten${SIM.rights().recordings ? ', R = Aufnehmen' : ''}, E = Erinnern, Shift+F10 = Menü</span></p>`;
  }
  const count = total ? (list.length === total ? `${total} Sender` : `${list.length} von ${total} Sendern`) + (VB.many && list.length ? ` · Raster mit 2'000 Testzeilen` : '') : '';
  return { type: 'viewer', nav: 'live', title: 'Programm', html: livePage({ tab: 'guide', title: 'Programm', count, sub: fmt.dayLong(day * 1440), html: tools + `<div class="vbg-body">${body}</div>` }), after: (root) => vbgMount(root) };
});
ACT.vbgDay = (d) => { const q = parseHash().query; const st = vbgState('page'); st.jumpT = Number(d.day) === vbToday() ? SIM.now() : Number(d.day) * 1440 + 20 * 60 + 15; vbgSetQuery(Object.assign({}, q, { day: String(d.day), time: '' })); };
ACT.vbgNow = () => { const st = vbgState('page'); st.jumpT = SIM.now(); st.left = null; st.f = null; st.focusNow = true; const q = parseHash().query; vbgSetQuery(Object.assign({}, q, { day: String(vbToday()), time: '' })); };
ACT.vbgGroup = (d) => { const q = parseHash().query; vbgSetQuery(Object.assign({}, q, { group: d.v })); };
document.addEventListener('change', (e) => {
  const el = e.target.closest && e.target.closest('[data-vbgsel]'); if (!el) return;
  const q = parseHash().query; const k = el.dataset.vbgsel;
  if (k === 'group') { vbgSetQuery(Object.assign({}, q, { group: el.value })); return; }
  if (el.value === 'other') {
    const day = q.day ? Number(q.day) : vbToday(); el.value = q.time || '';
    LiveDlg.pickTime({ day, time: q.time ? LIVE.hm(q.time) : (day === vbToday() ? Math.floor(SIM.now() % 1440) : 20 * 60 + 15), onPick: (t) => { const st = vbgState('page'); st.jumpT = t; st.left = null; st.f = null; st.focusNow = true; vbgSetQuery(Object.assign({}, q, { day: String(Math.floor(t / 1440)), time: fmt.time(t) })); } });
    return;
  }
  const st = vbgState('page');
  if (!el.value) { st.jumpT = SIM.now(); st.left = null; st.f = null; vbgSetQuery(Object.assign({}, q, { time: '', day: q.day || '' })); return; }
  vbgSetQuery(Object.assign({}, q, { time: el.value }));
});
/* the now line wanders every mockup minute; cells get their «läuft/vorbei» look again */
SIM.on((t) => {
  if (t !== 'minute') return;
  $$('.vbg-scroll').forEach((sc) => {
    const st = vbgState(sc.dataset.vbg); const g = st._; if (!g) return; const now = SIM.now();
    const x = g.colW + (now - g.d0) * VBG_PX; const l = $('[data-vbgnow]', sc); const lb = $('[data-vbgnowlbl]', sc);
    if (l) l.style.left = x + 'px'; if (lb) { lb.style.left = x + 'px'; lb.textContent = fmt.time(now); }
    vbgDraw(sc, true);
  });
});
let vbLastW = 0;
window.addEventListener('resize', () => { const w = vbWidth(); const cls = (x) => (x < 768 ? 0 : x < 1100 ? 1 : 2); if (cls(w) !== cls(vbLastW)) { vbLastW = w; render(); } else vbLastW = w; });

/* ---------- LS-04 at phone width: «Jetzt und danach» (14 4.2) ---------- */
function vbnPage(day, tq, group, list, total, mode) {
  const today = vbToday();
  const at = tq ? day * 1440 + LIVE.hm(tq) : (day === today ? SIM.now() : day * 1440 + 20 * 60 + 15);
  const isNow = !tq && day === today;
  const sel = `<label class="fld fsel"><span class="lbl">Zeit</span><select class="sel" data-vbgsel="time" data-fid="vbn:time" aria-label="Zeit">${day === today ? `<option value=""${!tq ? ' selected' : ''}>Jetzt</option>` : ''}<option value="20:15"${tq === '20:15' || (!tq && day !== today) ? ' selected' : ''}>20:15</option><option value="22:00"${tq === '22:00' ? ' selected' : ''}>22:00</option>${tq && tq !== '20:15' && tq !== '22:00' ? `<option value="${tq}" selected>${tq}</option>` : ''}<option value="other">Andere Uhrzeit …</option></select></label>`;
  const tools = `<div class="vbg-tools">${vbgDays(day, 'vbgDay')}<div class="vbg-row2">${sel}${vbgGroupSel(group, 'vbn:group')}<a class="btn ghost sm" href="#/live/guide/search" data-fid="vbn:search">${icon('search', 16)}Suchen</a></div></div>`;
  let body;
  if (mode === 'loading') body = skelRows(7, 92);
  else if (mode === 'empty' || !total) body = emptyState('Keine Sender', LU.TX.empty);
  else if (!list.length) body = emptyState('Keine Sender in dieser Gruppe', group === 'fav' ? 'Du hast noch keine Lieblingssender.' : 'Wähle eine andere Gruppe.');
  else {
    const cx = vbCtx(); const ids = list.map((c) => c.id); const now = SIM.now();
    body = (mode === 'error' ? liveError('Die Programmdaten konnten nicht aktualisiert werden.') : '') + (SIM.flags().noEpg ? notice('', 'Für die Sender gibt es gerade keine Programmdaten.', 'Fernsehen geht trotzdem.') : '')
      + `<h2 class="ghead">${isNow ? 'Jetzt' : `Um ${fmt.time(at)}`}${day !== today ? ` · ${fmt.dayRel(day * 1440)}` : ''}</h2><ul class="vbn" data-vbrove="list" data-vbkey="vbn" aria-label="Sender mit Sendung und danach">${list.map((c) => {
        const nn = SIM.nowNext(c.id, at); const p = nn.now; const bad = SIM.chState(c) === 'disturbed';
        const bl = p ? vbBadgeHtml(vbBadges(p, cx)) : '';
        const nx = nn.next ? `<span class="vbn-next">danach ${fmt.time(nn.next.start)} ${esc(nn.next.title)} ${vbBadgeHtml(vbBadges(nn.next, cx))}</span>` : p ? `<span class="vbn-next">danach ${esc(LU.TX.epgNone)}</span>` : '';
        const prog = p ? `<a class="vbn-p" href="#/live/programs/${esc(p.id)}" data-vbx tabindex="-1" data-fid="vbn:p:${c.id}" aria-label="${esc(p.title)}, ${fmt.time(p.start)} bis ${fmt.time(p.end)}, Details"><span class="vbn-t">${esc(p.title)} ${bl}</span><span class="vbn-m">${fmt.range(p.start, p.end)}${isNow ? ' · ' + LU.nowText('left', p.end) : ''}</span>${p.start <= now && p.end > now ? LU.progress(p.start, p.end, { label: 'Fortschritt von ' + p.title }) : ''}${nx}</a>`
          : `<span class="vbn-p none"><span class="vbn-t">${esc(SIM.epgOf(c) === 'none' ? LU.TX.epgNoneButPlays : LU.TX.epgNone)}</span>${nx}</span>`;
        const menu = popMenu('vbn:' + c.id, icon('dots', 18), p ? vbProgActions(p, { list: ids.join(',') }) : [{ label: 'Einschalten', act: 'vbTuneP', data: { ch: c.id, list: ids.join(',') }, icon: 'play' }, { label: 'Senderprogramm', href: '#/live/guide/channel/' + c.id, icon: 'calendar' }], { aria: `Aktionen für ${c.name}` }).replace('data-act="menu"', 'data-act="menu" data-vbx tabindex="-1"');
        return `<li class="vbn-row${bad ? ' bad' : ''}" data-vbrow="${c.id}"><button type="button" class="vbn-ch" data-vbi tabindex="${vbTab('vbn', c.id, ids)}" data-act="tune" data-ch="${c.id}" data-list="${ids.join(',')}" data-list-name="${group === 'fav' ? 'fav' : group || 'all'}" data-fid="vbn:ch:${c.id}" aria-label="${c.no} ${esc(c.name)} einschalten${bad ? ', gestört' : ''}"><span class="no">${c.no}</span>${LU.logo(c)}<span class="nm">${esc(c.name)}${bad ? `<span class="vbg-bad"><span class="dot warn"></span>${esc(LU.TX.disturbed)}</span>` : ''}</span></button>${prog}<span class="vbn-menu">${menu}</span></li>`;
      }).join('')}</ul><p class="muted vbn-help">Tippe auf Logo oder Name zum Einschalten, auf die Sendung für Details. Das Menü «…» bietet Aufnehmen, Erinnern und das Senderprogramm.</p>`;
  }
  const count = total ? (list.length === total ? `${total} Sender` : `${list.length} von ${total} Sendern`) : '';
  return { type: 'viewer', nav: 'live', title: 'Programm', html: livePage({ tab: 'guide', title: 'Programm', count, html: tools + body }) };
}

/* ======================================================================================
   LS-07 Live-Player
   ====================================================================================== */
function vbpS() { return ui.vbp || (ui.vbp = { load: null, infoUntil: 0, info: false, zl: false, zlSel: null, guide: false, num: null, miss: null, zc: null, vol: 70, muted: false, audio: {}, sub: {}, quality: 'auto', listIds: null, listName: '', retry: 0, readyAt: 0, sw: 0, swText: '', firstFocus: '' }); }
const VB_QUAL = [['auto', 'Automatisch'], ['orig', 'Original (1080i)'], ['720', '720p'], ['480', '480p']];
function vbListIds(name) {
  const p = SIM.player();
  if (name === 'fav') return SIM.favorites().filter((id) => SIM.channel(id) && SIM.channel(id).active);
  if (name === 'filter') return (p && p.list) || SIM.channels().map((c) => c.id);
  if (name && name !== 'all' && LIVE.groups.some((x) => x.id === name)) return SIM.channels({ group: name }).map((c) => c.id);
  return SIM.channels().map((c) => c.id);
}
function vbListLabel(name) { if (name === 'fav') return 'Meine Sender'; if (name === 'filter') return 'Gefilterte Sender'; const g = LIVE.groups.find((x) => x.id === name); return g ? 'Gruppe ' + g.name : 'Alle Sender'; }
function vbpList() { const P = vbpS(); const p = SIM.player(); const l = P.listIds || (p && p.list) || []; return l.length ? l : SIM.channels().map((c) => c.id); }
function vbpQs() { const P = vbpS(); return P.listName ? '?list=' + encodeURIComponent(P.listName) : ''; }
/** switch channel inside the player; asks first when behind live (LD-10, non-modal in the web player) */
function vbSwitch(to, o) {
  o = o || {}; const P = vbpS(); const p = SIM.player();
  const c = SIM.channel(to); if (!c) return;
  if (p && to === p.ch && !o.force) { P.zl = false; render(); return; }
  if (p && !o.confirmed && !VB.noAskZap && SIM.rights().timeshift && (p.offset > 1 || p.paused)) {
    const zc = P.zc;
    if (!(zc && zc.to === to && o.via && o.via !== 'click' && zc.via === o.via && Date.now() - zc.t < 3000)) { P.zc = { to, via: o.via || 'click', t: Date.now() }; render(); if (o.via === 'click' || !o.via) setTimeout(() => { const b = $('[data-fid="vbzc:go"]'); if (b) b.focus(); }, 0); return; }
  }
  P.zc = null; P.zl = false; P.guide = false;
  const list = vbpList();
  SIM.tune(to, { list });
  history.replaceState(null, '', '#/live/watch/' + to + vbpQs());
  render();
}
function vbZap(dir, via) {
  const p = SIM.player(); if (!p) return;
  const l = vbpList(); const k = l.indexOf(p.ch); const n = (k + dir + l.length) % l.length; const to = l[n];
  if (dir > 0 && n === 0 && l.length > 1) { const c = SIM.channel(to); toast(`Wieder am Anfang: ${c.no} ${esc(c.name)}`); }
  if (dir < 0 && k === 0 && l.length > 1) { const c = SIM.channel(to); toast(`Wieder am Ende: ${c.no} ${esc(c.name)}`); }
  vbSwitch(to, { via: via || 'zap' + dir });
}
/** loading phases after a switch (14 4.11): switching (0 s) · connecting (0.5 s) · slow (5 s) · ready */
function vbpLoadState(p) {
  /* «Langsamer Start» and «Sender nicht erreichbar» of the bar «Zustände» restart the loading phases at once on the running channel (RM-16);
     the other faults come straight from LIVE_UI.playerPhase on the next drawing */
  const f = SIM.flags().fault; const P = vbpS(); const key = `${p.ch}@${p.joinedAt}#${P.retry}~${f === 'slow' || f === 'unreachable' ? f : ''}${SIM.flags().noContain ? '!nc' : ''}`;
  if (!P.load || P.load.key !== key) {
    const recv = LIVE.inputs.some((i) => i.ch === p.ch && i.state === 'running');
    const L = LU.LOAD; const dur = VB.slow ? L.slowReadyMs : recv ? L.hotReadyMs : L.coldReadyMs; /* thresholds of the core, shortened in the mockup */
    P.load = { key, t0: Date.now(), dur, recv }; P.readyAt = Date.now() + dur; P.infoUntil = Date.now() + dur + 5000;
    [L.spinnerMs, 1200, L.slowAfterMs, dur, L.giveUpMs, dur + 5050].forEach((ms) => setTimeout(() => { if (P.load && P.load.key === key && parseHash().path.startsWith('/live/watch/')) render(); }, ms + 10));
  }
  const ph = LU.playerPhase(Date.now() - P.load.t0, SIM.channel(p.ch), { hot: P.load.recv });
  P.load.err = ph.err || null; P.load.why = ph.why || '';
  return ph.k === 'ok' || ph.k === 'error' ? 'ready' : ph.k === 'slow' ? 'slow' : ph.spin ? 'connecting' : 'switching';
}
function vbpError(c) {
  const s = SIM.chState(c);
  if (s === 'gone') return 'gone';
  const P = vbpS(); return P.load && P.load.err !== 'revoked' ? P.load.err : null; /* from LIVE_UI.playerPhase (fault states and disturbed channels) */
}
function vbpErrHtml(kind, c) {
  const T = {
    unreachable: [`${c.name} ist gerade nicht erreichbar. Slouchcat versucht es weiter.`, 'Nächster Versuch in <span data-vbcount>20</span> s', ['retry', 'other']],
    gone: ['Diesen Sender gibt es nicht mehr.', '', ['other']],
    limit: ['Auf deinem Konto laufen schon 3 Wiedergaben gleichzeitig.', 'Beende eine andere Wiedergabe und versuche es dann nochmals.', ['back']],
    notPlayable: ['Dieser Sender lässt sich auf diesem Gerät nicht abspielen.', '', ['other']],
    busy: ['Der Server ist gerade ausgelastet. Versuche es gleich nochmals.', '', ['retry']],
    busyPerm: [LU.PLAYER_ERR.busyPerm(c), '', ['retry']],
    noConvert: [LU.PLAYER_ERR.noConvert(c), '', ['other']],
  }[kind];
  const B = { retry: ['vbpRetry', 'Erneut versuchen', 'refresh'], other: ['vbpOther', 'Anderer Sender', 'list'], back: ['vbpLeave', 'Zurück', 'arrowL'] };
  return `<div class="vbp-err" role="alert">${LU.logo(c)}<b>${c.no} ${esc(c.name)}</b><p>${esc(T[0])}</p>${T[1] ? `<p class="m">${T[1]}</p>` : ''}<div class="row-gap">${T[2].map((k, i) => `<button type="button" class="btn ${i ? 'secondary' : 'primary'}" data-act="${B[k][0]}" data-fid="vbperr:${k}">${icon(B[k][2], 18)}${B[k][1]}</button>`).join('')}</div></div>`;
}
/* ---------- LiveSeekBar (6.5) ---------- */
function vbpSeekModel(p) {
  const now = SIM.now(); const ws = now - p.max; const pos = now - p.offset; const span = now - ws;
  const prog = SIM.nowNext(p.ch, pos).now; const stub = prog && prog.start < ws - 0.01 ? prog.start : null;
  const frac = span < 1 / 60 ? 1 : Math.max(0, Math.min(1, (pos - ws) / span));
  const marks = span < 1 ? [] : SIM.programsOf(p.ch, ws, now).filter((x) => x.start > ws && x.start < now).map((x) => ({ f: (x.start - ws) / span, p: x }));
  const capped = p.max >= LIVE.timeshiftMaxMin - 0.01;
  return { now, ws, pos, span, frac, stub, prog, marks, capped, next: SIM.nowNext(p.ch).next, joined: p.joinedAt };
}
function vbpSeekSub(m) { return m.capped ? `ab ${fmt.time(m.ws)} (höchstens ${LIVE.timeshiftMaxMin / 60} Std.)` : `seit ${fmt.time(m.joined)}`; }
function vbpSeekHtml(p) {
  const m = vbpSeekModel(p);
  return `<div class="vbs" data-vbseek>
    <div class="vbs-lbls"><span class="l">${m.prog ? `${esc(m.prog.title)} ${fmt.time(m.prog.start)}` : esc(LU.TX.epgNone)}</span><span class="r">${m.next ? `${esc(m.next.title)} ${fmt.time(m.next.start)} (später)` : ''}</span></div>
    <div class="vbs-bar">${m.stub != null ? `<span class="vbs-stub lv-gap" title="Vor ${fmt.time(m.ws)} nicht verfügbar"></span>` : ''}
      <div class="vbs-trk" role="slider" tabindex="0" data-vbtrk data-fid="vbp:seek" aria-label="Zeitversatz, Pfeiltasten spulen 10 Sekunden" aria-valuemin="0" aria-valuemax="${Math.round(m.span * 60)}" aria-valuenow="${Math.round((m.pos - m.ws) * 60)}" aria-valuetext="${fmt.time(m.pos)}${p.offset >= 0.05 ? ', ' + fmt.offset(p.offset) + ' zeitversetzt' : ', live'}">
        <span class="vbs-rail"></span><span class="vbs-fill" style="width:${(m.frac * 100).toFixed(2)}%"></span>${m.marks.map((x) => `<span class="vbs-mark" style="left:${(x.f * 100).toFixed(2)}%" title="ab ${fmt.time(x.p.start)} ${esc(x.p.title)}"></span>`).join('')}<span class="vbs-thumb" style="left:${(m.frac * 100).toFixed(2)}%"></span></div>
      <span class="vbs-edge">Live <span data-vbedge>${fmt.time(m.now)}</span></span></div>
    <div class="vbs-sub"><span data-vbsub>${vbpSeekSub(m)}</span>${m.stub != null ? `<span class="vbs-na">Vor <span data-vbna>${fmt.time(m.ws)}</span> nicht verfügbar</span>` : ''}</div></div>`;
}
function vbpSeekPatch(el, p) {
  const m = vbpSeekModel(p); const trk = $('[data-vbtrk]', el); if (!trk) return;
  if ((m.stub != null) !== !!$('.vbs-stub', el) || m.marks.length !== $$('.vbs-mark', el).length) { const f = document.activeElement === trk; el.outerHTML = vbpSeekHtml(p); if (f) { const t2 = $('[data-vbtrk]'); if (t2) t2.focus({ preventScroll: true }); } return; }
  $('.vbs-fill', el).style.width = (m.frac * 100).toFixed(2) + '%'; $('.vbs-thumb', el).style.left = (m.frac * 100).toFixed(2) + '%';
  $$('.vbs-mark', el).forEach((mk, i) => { mk.style.left = (m.marks[i].f * 100).toFixed(2) + '%'; });
  trk.setAttribute('aria-valuemax', Math.round(m.span * 60)); trk.setAttribute('aria-valuenow', Math.round((m.pos - m.ws) * 60)); trk.setAttribute('aria-valuetext', `${fmt.time(m.pos)}${p.offset >= 0.05 ? ', ' + fmt.offset(p.offset) + ' zeitversetzt' : ', live'}`);
  const e1 = $('[data-vbedge]', el); if (e1) e1.textContent = fmt.time(m.now); const s1 = $('[data-vbsub]', el); if (s1) s1.textContent = vbpSeekSub(m); const n1 = $('[data-vbna]', el); if (n1) n1.textContent = fmt.time(m.ws);
}
/** program progress instead of the seek bar (no timeshift right, 14 4.7) */
function vbpProgHtml(p) {
  const nn = SIM.nowNext(p.ch).now;
  return `<div class="vbs ro" data-vbseekro><div class="vbs-lbls"><span class="l">${nn ? `${esc(nn.title)} · ${fmt.range(nn.start, nn.end)}` : esc(LU.TX.epgNone)}</span><span class="r"></span></div>${nn ? LU.progress(nn.start, nn.end, { cls: 'vbs-prog', label: 'Fortschritt der Sendung' }) : ''}</div>`;
}
function vbpInd(p) {
  if (!p) return '';
  if (p.paused) return `<button type="button" class="vbp-live off" data-vbind="p" data-act="vbpLive" data-fid="vbp:live" aria-label="Pausiert, ${fmt.offset(p.offset)}, zurück zu Live">Pausiert · <span data-vboff>${fmt.offset(p.offset)}</span></button>`;
  if (p.offset >= 0.05) return `<button type="button" class="vbp-live off" data-vbind="s" data-act="vbpLive" data-fid="vbp:live"><span data-vboff>${fmt.offset(p.offset)}</span> · ${esc(LU.TX.backToLive)}</button>`;
  return `<span class="vbp-live on" data-vbind="l" role="img" aria-label="Live"><span class="d" aria-hidden="true"></span>${esc(LU.TX.live)}</span>`;
}
/* ---------- buttons ---------- */
const vbpIb = (act, ic, label, o) => { o = o || {}; return `<button type="button" class="vbp-ib${o.cls ? ' ' + o.cls : ''}" data-act="${act}" ${o.data || ''} data-fid="vbp:${o.fid || act}" aria-label="${esc(o.aria || label)}"${o.dis ? ' aria-disabled="true"' : ''}${o.pressed != null ? ` aria-pressed="${o.pressed}"` : ''}${o.exp != null ? ` aria-expanded="${o.exp}"` : ''}>${icon(ic, o.size || 22)}${o.txt ? `<span class="txt">${o.txt}</span>` : ''}${o.n ? `<span class="n" aria-hidden="true">${o.n}</span>` : ''}<span class="vbp-tip" aria-hidden="true">${esc(label)}${o.kbd ? ` <kbd>${o.kbd}</kbd>` : ''}</span></button>`; };
function vbpZapBtn(dir, cur) {
  const l = vbpList(); const k = l.indexOf(cur); const c = SIM.channel(l[(k + dir + l.length) % l.length]); const nn = c ? SIM.nowNext(c.id).now : null;
  const lbl = dir > 0 ? 'Nächster Sender' : 'Vorheriger Sender';
  return `<span class="vbp-zw"><button type="button" class="vbp-ib zap" data-act="vbpZap" data-dir="${dir}" data-fid="vbp:zap${dir}" aria-label="${lbl}${c ? ': ' + c.no + ' ' + esc(c.name) : ''}">${dir < 0 ? icon('chevL', 20) : ''}<span class="txt">Sender</span>${dir > 0 ? icon('chevR', 20) : ''}</button>${c ? `<span class="vbp-card" aria-hidden="true"><span class="e">${lbl} <kbd>${dir > 0 ? 'Bild↑' : 'Bild↓'}</kbd></span>${LU.logo(c)}<span class="n">${c.no} ${esc(c.name)}</span><span class="m">${nn ? esc(nn.title) : esc(LU.TX.epgNone)}</span></span>` : ''}</span>`;
}
function vbpRecBtn(c, p, nn) {
  if (!SIM.rights().recordings || SIM.chState(c) === 'gone') return '';
  const run = SIM.runningOn(c.id);
  if (run) {
    const items = [];
    if (nn && run.end < nn.end + LIVE.settings.dvr.padAfter - 0.5) items.push({ label: 'Bis Sendungsende verlängern', act: 'vbRecExtend', data: { id: run.id }, icon: 'clock' });
    items.push({ label: 'Aufnahme beenden', act: 'vbRecStop', data: { id: run.id }, icon: 'x', danger: true });
    items.push({ label: 'Zu meinen Aufnahmen', href: '#/live/recordings', icon: 'list' });
    return popMenu('vbp:rec', `${icon('record', 16)}<span class="txt">Nimmt auf · <span data-vbrecel data-s="${run.start}">${vbElapsed(run.start)}</span></span>`, items, { cls: 'vbp-ib rec on', aria: 'Nimmt auf, Menü der Aufnahme' });
  }
  const rs = nn ? SIM.recState(nn.id) : null;
  if (rs === 'scheduled' || rs === 'series') return `<button type="button" class="vbp-ib rec planned" data-act="${rs === 'series' ? 'vbSeries' : 'vbPlan'}" data-pid="${esc(nn.id)}" data-fid="vbp:rec" aria-label="Aufnahme geplant, ändern">${icon('record', 16)}<span class="txt">Geplant</span><span class="vbp-tip" aria-hidden="true">Aufnahme ändern</span></button>`;
  return `<button type="button" class="vbp-ib rec" data-act="vbpRec" data-fid="vbp:rec" aria-label="Aufnehmen">${icon('record', 16)}<span class="txt">${esc(LU.TX.record)}</span><span class="vbp-tip" aria-hidden="true">Sofort aufnehmen <kbd>R</kbd></span></button>`;
}
function vbElapsed(start) { const m = Math.max(0, Math.floor(SIM.now() - start)); return `${Math.floor(m / 60)}:${String(m % 60).padStart(2, '0')}`; }
function vbpMenus(c) {
  const P = vbpS(); const ai = P.audio[c.id] || 0; const si = P.sub[c.id] || 0;
  const aud = popMenu('vbp:audio', `${icon('vbAudio', 22)}<span class="vbp-tip" aria-hidden="true">Ton <kbd>T</kbd></span>`, (c.audio || []).map((a, i) => ({ label: a, act: 'vbpAudio', data: { i }, icon: i === ai ? 'check' : 'vbBlank' })), { cls: 'vbp-ib', aria: 'Ton: ' + ((c.audio || [])[ai] || '') });
  /* Browser: no selectable subtitles until stage E (14 4.10, RG-05): greyed-out button with the reason */
  const subs = vbSubBtn('vbp:subs', c);
  const set = popMenu('vbp:set', `${icon('settings', 22)}<span class="vbp-tip" aria-hidden="true">Einstellungen <kbd>Q</kbd></span>`, [...VB_QUAL.map(([k, l]) => ({ label: 'Qualität: ' + l, act: 'vbpQual', data: { q: k }, icon: P.quality === k ? 'check' : 'vbBlank' })), { label: 'Info Jetzt/Danach (I)', act: 'vbpInfo', icon: 'info' }, { label: 'Tastenkürzel (?)', act: 'vbpKeys', data: { k: 'live' }, icon: 'help' }], { cls: 'vbp-ib', aria: 'Einstellungen: Qualität und Info' });
  return { aud, subs, set };
}
function vbpVol() { const P = vbpS(); const on = !P.muted && P.vol > 0; return `<span class="vbp-vol">${vbpIb('vbpMute', on ? 'vbVol' : 'vbMute', on ? 'Stumm schalten' : 'Ton an', { kbd: 'M', pressed: P.muted })}<input type="range" class="vbp-range" min="0" max="100" step="5" value="${P.muted ? 0 : P.vol}" data-vbvol data-fid="vbp:vol" aria-label="Lautstärke" aria-valuetext="${P.muted ? 'stumm' : P.vol + ' %'}"></span>`; }
function vbpBanner(c, fb) {
  const nn = SIM.nowNext(c.id); const cx = vbCtx();
  const b = nn.now ? vbBadgeHtml(vbBadges(nn.now, cx), true) : '';
  return `<div class="vbp-info" role="status" aria-live="polite"><div class="h">${LU.logo(c)}<span class="no">${c.no}</span><b>${esc(c.name)}</b>${fb ? '<span class="chip info">Ersatzquelle</span>' : ''}${b}</div>
    ${nn.now ? `<p class="r"><span class="k">${esc(LU.TX.now)}</span><span class="tm">${fmt.range(nn.now.start, nn.now.end)}</span><span class="tt">${esc(nn.now.title)}${nn.now.sub ? ` <span class="sec">«${esc(nn.now.sub)}»</span>` : ''}</span></p>${LU.progress(nn.now.start, nn.now.end, { label: 'Fortschritt' })}` : `<p class="r"><span class="tt">${esc(SIM.epgOf(c) === 'none' ? LU.TX.epgNoneButPlays : LU.TX.epgNone)}</span></p>`}
    ${nn.next ? `<p class="r"><span class="k">${esc(LU.TX.next)}</span><span class="tm">${fmt.time(nn.next.start)}</span><span class="tt">${esc(nn.next.title)}</span></p>` : nn.now ? `<p class="r"><span class="k">${esc(LU.TX.next)}</span><span class="tt muted">${esc(LU.TX.epgNone)}</span></p>` : ''}</div>`;
}
function vbpZapList(c) {
  const P = vbpS(); const ids = vbpList(); const rows = ids.map((id) => SIM.channel(id)).filter(Boolean);
  const sel = P.zlSel && ids.includes(P.zlSel) ? P.zlSel : c.id;
  const opts = [['all', 'Alle Sender'], ['fav', 'Meine Sender'], ...LIVE.groups.map((x) => [x.id, 'Gruppe ' + x.name])];
  const cur = P.listName && P.listName !== 'filter' ? P.listName : (P.listName === 'filter' ? 'filter' : 'all');
  return `<div class="vbz" data-vbz role="dialog" aria-label="Senderliste">
    <div class="vbz-h"><b>Senderliste</b><label class="vbz-sel"><span>Liste</span><select class="sel" data-vbzlist data-fid="vbz:list" aria-label="Liste für den Senderwechsel">${cur === 'filter' ? '<option value="filter" selected>Gefilterte Sender</option>' : ''}${opts.map(([v, t]) => `<option value="${v}"${v === cur ? ' selected' : ''}>${esc(t)}</option>`).join('')}</select></label></div>
    ${rows.length ? `<ul class="vbz-l" role="listbox" aria-label="Sender. Pfeiltasten wechseln, Enter schaltet um, Buchstaben springen">${rows.map((x) => { const nn = SIM.nowNext(x.id).now; const bad = SIM.chState(x) !== 'ok'; return `<li role="option" class="${x.id === c.id ? 'cur' : ''}${bad ? ' bad' : ''}" aria-selected="${x.id === c.id}" tabindex="${x.id === sel ? 0 : -1}" data-vbzch="${x.id}" data-fid="vbz:${x.id}" data-name="${esc(x.name.toLowerCase())}"><span class="no">${x.no}</span>${LU.logo(x)}<span class="nm"><b>${esc(x.name)}${x.id === c.id ? ' <span class="vh">, läuft gerade</span>' : ''}</b><span class="m">${bad ? `<span class="dot warn"></span>${esc(LU.TX.unavailable)}` : nn ? esc(nn.title) : esc(LU.TX.epgNone)}</span>${nn && !bad ? LU.progress(nn.start, nn.end, { label: 'Fortschritt' }) : ''}</span>${x.id === c.id ? `<span class="chip info">Läuft</span>` : ''}</li>`; }).join('')}</ul>` : `<p class="muted vbz-empty">In dieser Liste ist kein Sender.</p>`}</div>`;
}
function vbpGuide() {
  const st = vbgState('ov'); const day = st.day != null ? st.day : vbToday();
  const det = st.detail ? SIM.program(st.detail) : null;
  const colW = vbWidth() < 1100 ? 96 : 200;
  const dp = det ? (() => { const c = SIM.channel(det.ch); const acts = vbProgActions(det, { channelProgram: false }).filter((a) => a.act); return `<aside class="vbgo-det" role="dialog" aria-label="Details: ${esc(det.title)}"><b>${esc(det.title)}</b>${det.sub ? `<span class="sec">«${esc(det.sub)}»</span>` : ''}<span class="muted">${esc(c.name)} · ${fmt.when(det.start)}–${fmt.time(det.end)}</span><p class="sec">${esc(det.desc || 'Keine Beschreibung in den Programmdaten.')}</p><div class="stack">${acts.map((a, i) => `<button type="button" class="btn secondary sm" data-act="${a.act}" ${Object.entries(a.data || {}).map(([k, v]) => `data-${k}="${esc(v)}"`).join(' ')} data-fid="vbgod:${i}"${i === 0 ? ' data-vbgofirst' : ''}>${icon(a.icon, 16)}${esc(a.label)}</button>`).join('')}<button type="button" class="btn ghost sm" data-act="vbgoDetClose" data-fid="vbgod:x">Zurück zum Raster</button></div></aside>`; })() : '';
  return `<div class="vbgo" data-vbgo role="dialog" aria-label="Programm über dem Bild"><div class="vbgo-h"><b>Programm</b>${vbgDays(day, 'vbgoDay')}<button type="button" class="btn secondary sm" data-act="vbpGuide" data-fid="vbgo:x">Schliessen <kbd>Esc</kbd></button></div>
    <div class="vbgo-b">${vbgHtml('ov', { day, group: '', colW, maxH: '100%' })}${dp}</div></div>`;
}
function vbpZcHtml(p) {
  const P = vbpS(); const c = SIM.channel(P.zc.to);
  return `<div class="vbp-zc" role="alertdialog" aria-labelledby="vbzc-t" aria-describedby="vbzc-d"><p id="vbzc-t"><b>Zu ${c.no} ${esc(c.name)} wechseln?</b></p><p id="vbzc-d">Du schaust ${vbMin(Math.max(1, Math.round(p.offset)))} zeitversetzt${p.paused ? ' (pausiert)' : ''}. Beim Senderwechsel geht das verloren.</p>${P.zc.via !== 'click' ? `<p class="m">Nochmals drücken zum Wechseln</p>` : ''}<label class="check"><input type="checkbox" data-vbnoask data-fid="vbzc:ask"> Nicht mehr fragen (dieses Gerät)</label><div class="row-gap"><button type="button" class="btn primary sm" data-act="vbpZcGo" data-fid="vbzc:go">Wechseln</button><button type="button" class="btn secondary sm" data-act="vbpZcStay" data-fid="vbzc:stay">Bleiben</button></div></div>`;
}
function vbpNumHtml() {
  const P = vbpS();
  if (P.miss) return `<div class="vbp-num miss" role="status">Kein Sender mit Nummer ${esc(P.miss.n)}</div>`;
  if (!P.num) return '';
  const c = SIM.channelByNo(Number(P.num.d));
  return `<div class="vbp-num" role="status" aria-live="polite"><span class="d">${esc(P.num.d)}<span class="cur" aria-hidden="true">_</span></span>${c ? `<span class="pv">${c.no} · ${esc(c.name)}</span>` : '<span class="pv muted">Nummer eingeben …</span>'}</div>`;
}
/* ---------- the page ---------- */
function vbpHtml(c, p) {
  const P = vbpS(); const phone = vbIsPhone(); const r = SIM.rights();
  const gone = SIM.chState(c) === 'gone';
  const phase = gone || !p ? 'ready' : vbpLoadState(p);
  const err = gone ? 'gone' : phase === 'ready' ? vbpError(c) : null;
  const ready = phase === 'ready' && !err;
  const stall = ready && VB.perr === 'stall';
  const fb = ready && VB.fallback;
  if (!VB.fallback) VB.fbShown = false;
  if (fb && !VB.fbShown) { VB.fbShown = true; setTimeout(() => toast(esc(LU.TX.fallback), 'ok'), 0); }
  const nn = SIM.nowNext(c.id);
  const sw = Date.now() < P.sw;
  const pic = ready ? LU.fakeVideo(c, { paused: (p && p.paused) || stall }) : `<div class="vbp-black"></div>`;
  const load = !ready && !err && phase !== 'switching' ? `<div class="vbp-load" role="status"><span class="spin lg"></span><b>${esc(c.name)} wird geladen</b>${phase === 'slow' ? `<span>${esc('Dauert länger als üblich …')}</span>${P.load && P.load.why ? `<span>${esc(P.load.why)}</span>` : ''}` : ''}</div>` : '';
  const ov = stall ? `<div class="vbp-load dim" role="status"><span class="spin lg"></span><b>${esc('Verbindung wird wiederhergestellt …')}</b></div>` : sw ? `<div class="vbp-load dim" role="status"><span class="spin lg"></span><b>${esc(P.swText)}</b></div>` : '';
  const showInfo = !err && (P.info || Date.now() < P.infoUntil);
  const ts = r.timeshift && !gone && p;
  const title = `<div class="vbp-head"><p class="t">${c.no} ${esc(c.name)}</p><p class="d">${nn.now ? esc(nn.now.title) + (nn.now.sub ? ` · «${esc(nn.now.sub)}»` : '') : esc(SIM.epgOf(c) === 'none' ? LU.TX.epgNoneButPlays : LU.TX.epgNone)}</p></div>`;
  const recChip = r.recordings && SIM.runningOn(c.id) ? `<span class="chip vbp-recchip">${LU.badge('recording', { text: true })}</span>` : '';
  const m = vbpMenus(c);
  const seek = !p || gone ? '' : ts ? vbpSeekHtml(p) : vbpProgHtml(p);
  const play = ts ? vbpIb('vbpPlay', p.paused ? 'play' : 'pause', p.paused ? 'Weiter' : 'Anhalten', { cls: phone ? 'lg' : 'mid', kbd: 'Leertaste', size: phone ? 30 : 24 }) : '';
  const back = ts ? vbpIb('vbpSeek', 'vbBack', '10 Sekunden zurück', { data: 'data-s="-10"', fid: 'b10', n: '10', kbd: '←' }) : '';
  const fwd = ts ? vbpIb('vbpSeek', 'vbFwd', '10 Sekunden vor', { data: 'data-s="10"', fid: 'f10', n: '10', kbd: '→' }) : '';
  const clock = p ? `<span class="vbp-clock" data-vbclock aria-label="Uhrzeit der Position">${fmt.clock(p.position)}</span>` : '';
  const stage = `<div class="vbp-stage${phone ? ' ph' : ''}" data-vbstage tabindex="0" data-fid="vbp:stage" aria-label="Bild von ${esc(c.name)} (simuliert). Leertaste hält an, Bild↑/↓ wechselt den Sender, 0 bis 9 Sendernummer, C Senderliste, G Programm, Esc verlässt den Player">
      <div class="vbp-pic" data-act="vbpStage">${pic}</div>${load}${ov}${err ? vbpErrHtml(err, c) : ''}
      <div class="vbp-top">${vbpIb('vbpLeave', 'arrowL', 'Zurück', { kbd: 'Esc', txt: phone ? '' : 'Zurück' })}${title}${recChip}${phone ? `${m.aud}${m.subs}${m.set}` : ''}</div>
      ${vbpNumHtml()}${showInfo && !phone ? vbpBanner(c, fb) : ''}${P.zl && !phone ? vbpZapList(c) : ''}${P.guide && !phone ? vbpGuide() : ''}${P.zc && p ? vbpZcHtml(p) : ''}
      ${phone ? `<div class="vbp-mid">${vbpZapBtn(-1, c.id)}${back}${play}${fwd}${vbpZapBtn(1, c.id)}</div>` : ''}
      <div class="vbp-bot">${seek}${phone ? `<div class="vbp-tb ph"><span class="grp">${vbpInd(p)}${clock}</span><span class="grp">${vbpIb('vbpFs', 'vbFull', 'Vollbild', { kbd: 'F' })}</span></div>` : `<div class="vbp-tb" role="toolbar" aria-label="Wiedergabe, Pfeiltasten wechseln den Knopf">
        <span class="grp">${vbpZapBtn(-1, c.id)}${back}${play}${fwd}${vbpZapBtn(1, c.id)}<span class="sep"></span>${vbpInd(p)}${clock}</span>
        <span class="grp">${vbpVol()}${vbpRecBtn(c, p, nn.now)}${vbpIb('vbpList', 'list', 'Senderliste', { kbd: 'C', exp: P.zl, fid: 'zl' })}${vbpIb('vbpGuide', 'grid', 'Programm', { kbd: 'G', exp: P.guide, fid: 'guide' })}${m.aud}${m.subs}${m.set}${vbpIb('vbpFs', 'vbFull', 'Vollbild', { kbd: 'F' })}</span></div>`}</div>
    </div>`;
  if (!phone) return `<div class="vbp" data-vbp>${stage}</div>`;
  const cx = vbCtx(); const ids = vbpList();
  const below = `<div class="vbp-below">
    <section class="vbp-card2" aria-label="Jetzt und danach">${vbpBanner(c, fb).replace('class="vbp-info"', 'class="vbp-info flat"')}<div class="row-gap">${r.recordings && !gone ? vbpRecBtn(c, p, nn.now).replace(/vbp-ib/g, 'btn secondary sm') : ''}${nn.now ? `<a class="btn ghost sm" href="#/live/programs/${esc(nn.now.id)}" data-fid="vbp:det">${icon('info', 16)}Details</a>` : ''}<a class="btn ghost sm" href="#/live/guide" data-fid="vbp:guideph">${icon('grid', 16)}Programm</a></div></section>
    <section class="vbp-chs"><div class="vbz-h"><b>Sender</b><label class="vbz-sel"><span>Liste</span><select class="sel" data-vbzlist data-fid="vbz:list" aria-label="Liste">${[['all', 'Alle'], ['fav', 'Meine Sender'], ...LIVE.groups.map((x) => [x.id, x.name])].map(([v, t]) => `<option value="${v}"${v === (P.listName || 'all') ? ' selected' : ''}>${esc(t)}</option>`).join('')}${P.listName === 'filter' ? '<option value="filter" selected>Gefilterte Sender</option>' : ''}</select></label></div>
    <ul class="vbp-chl" data-vbrove="list" data-vbkey="vbpch" aria-label="Sender der Liste">${ids.map((id) => SIM.channel(id)).filter(Boolean).map((x) => { const n2 = SIM.nowNext(x.id).now; const bad = SIM.chState(x) !== 'ok'; return `<li data-vbrow="${x.id}"><button type="button" class="vbp-chr${x.id === c.id ? ' cur' : ''}" data-vbi tabindex="${vbTab('vbpch', x.id, ids)}" data-act="vbpPick" data-ch="${x.id}" data-fid="vbpch:${x.id}" aria-current="${x.id === c.id}"><span class="no">${x.no}</span>${LU.logo(x)}<span class="nm"><b>${esc(x.name)}</b><span class="m">${bad ? esc(LU.TX.unavailable) : n2 ? esc(n2.title) : esc(LU.TX.epgNone)} ${n2 ? vbBadgeHtml(vbBadges(n2, cx)) : ''}</span>${n2 && !bad ? LU.progress(n2.start, n2.end, { label: 'Fortschritt' }) : ''}</span>${x.id === c.id ? '<span class="chip info">Läuft</span>' : ''}</button></li>`; }).join('')}</ul></section>
    ${LU.sampleNote()}</div>`;
  return `<div class="vbp ph" data-vbp>${stage}${below}</div>`;
}
route('/live/watch/:id', (pm, q) => {
  const P = vbpS(); const c = SIM.channel(pm.id);
  if (!c) return notFoundView();
  const ae = document.activeElement; if (ae && ae.closest && ae.closest('.vbgo .vbg-rows')) vbgState('ov').refocus = true;
  let p = SIM.player();
  if (q.list != null && q.list !== P.listName) { P.listName = q.list; P.listIds = null; }
  if (SIM.chState(c) !== 'gone' && (!p || p.ch !== c.id)) { SIM.tune(c.id, { list: P.listIds || vbListIds(P.listName) }); p = SIM.player(); }
  if (SIM.chState(c) === 'gone') p = null;
  return { type: 'bare', title: `${c.name} · Live`, html: vbpHtml(c, p), after: vbpAfter };
});
/** recording right or live-TV right withdrawn while watching: the player says so (14 4.12) */
route('/player/revoked', (pm, q) => {
  const c = q.ch ? SIM.channel(q.ch) : null;
  const txt = q.what === 'rec' ? 'Aufnahmen sind für dieses Profil nicht mehr freigegeben.' : 'Live-TV ist für dieses Profil nicht mehr freigegeben.';
  return { type: 'bare', title: 'Nicht mehr freigegeben', html: `<div class="vbp" data-vbp><div class="vbp-stage" tabindex="-1"><div class="vbp-pic"><div class="vbp-black"></div></div><div class="vbp-err" role="alert">${c ? LU.logo(c) + `<b>${c.no} ${esc(c.name)}</b>` : ''}<p>${esc(txt)}</p><div class="row-gap"><a class="btn primary" href="#/" data-fid="vbrev:home" data-autofocus>${icon('home', 18)}Zur Startseite</a></div></div></div></div>`,
    after: (root) => { const b = $('[data-fid="vbrev:home"]', root); if (b) b.focus(); } };
});
let vbRevPath = '';
SIM.on((t) => {
  if (t !== 'change') return;
  const { path } = parseHash();
  if (path.startsWith('/live/watch/') && !SIM.rights().liveTv) { vbRevPath = path; history.replaceState(null, '', '#/player/revoked?ch=' + encodeURIComponent(path.split('/')[3])); SIM.stop(); render(); }
  else if (/^\/live\/recordings\/[^/]+\/play$/.test(path) && !SIM.rights().recordings) { history.replaceState(null, '', '#/player/revoked?what=rec'); render(); }
});
function vbpAfter(root) {
  const P = vbpS(); const key = location.hash.split('?')[0];
  if (P.firstFocus !== key && !P.zl && !P.guide) { P.firstFocus = key; const st = $('[data-vbstage]', root); if (st && !ui.dialog && (!document.activeElement || document.activeElement === document.body || document.activeElement.tagName === 'H1' || !root.contains(document.activeElement))) st.focus({ preventScroll: true }); }
  if (P.zl && P.zlFocus) { P.zlFocus = false; const el = $('.vbz-l [tabindex="0"]', root); if (el) { el.focus({ preventScroll: true }); const l = el.parentElement; l.scrollTop = Math.max(0, el.offsetTop - l.clientHeight / 2); } }
  if (P.guide) {
    vbgMount(root);
    const st = vbgState('ov');
    if (st.detail) { const b = $('[data-vbgofirst]', root) || $('[data-fid="vbgod:x"]', root); if (b && !root.querySelector('.vbgo-det').contains(document.activeElement)) b.focus(); }
  }
  vbpIdleArm(root);
  if (P.zc && P.zc.via === 'click' && !P.zc.focused) { P.zc.focused = true; const b = $('[data-fid="vbzc:go"]', root); if (b) b.focus(); }
}
/* auto-hide of the controls after 4 s without mouse or key (computer); focus inside keeps them */
let vbIdleT = 0;
function vbpIdleArm(root) {
  const stg = root && $('[data-vbstage]', root); if (!stg || stg.classList.contains('ph')) return;
  clearTimeout(vbIdleT); stg.classList.remove('idle');
  const P = vbpS(); const p = SIM.player();
  vbIdleT = setTimeout(() => { const s = $('[data-vbstage]'); if (!s || P.zl || P.guide || P.zc || ui.menu || (p && p.paused)) return; if (s.querySelector('.vbp-tb, .vbp-top').contains(document.activeElement)) return; s.classList.add('idle'); }, 4000);
}
document.addEventListener('mousemove', (e) => { if (e.target.closest && e.target.closest('[data-vbstage]')) vbpIdleArm(document.getElementById('scroll')); }, { passive: true });

/* ---------- actions ---------- */
ACT.vbpLeave = () => { const P = vbpS(); P.zl = false; P.guide = false; P.zc = null; P.num = null; if (history.length > 1) history.back(); else go('#/live'); };
ACT.vbpPlay = () => { const p = SIM.player(); if (!p) return; if (!SIM.rights().timeshift) { toast(esc('Anhalten und Zurückspulen sind für dieses Profil nicht freigegeben.'), 'warn'); return; } if (p.paused) SIM.resume(); else SIM.pause(); };
ACT.vbpSeek = (d) => { const p = SIM.player(); if (!p || !SIM.rights().timeshift) return; const s = Number(d.s); if (s > 0 && p.offset < 0.05 && !p.paused) { toast('Du bist schon live.'); return; } if (s < 0 && p.max - p.offset < 0.05) { toast(`Vor ${fmt.time(SIM.now() - p.max)} gibt es nichts: dein Einschalten ist der Anfang.`, 'warn'); } SIM.seek(p.offset - s / 60); };
ACT.vbpLive = () => { SIM.toLive(); setTimeout(() => { const s = $('[data-vbstage]'); if (s && (!document.activeElement || document.activeElement === document.body)) s.focus(); }, 0); };
ACT.vbpZap = (d) => vbZap(Number(d.dir), 'click');
ACT.vbpList = () => { const P = vbpS(); P.zl = !P.zl; P.guide = false; P.zlSel = null; P.zlFocus = P.zl; render(); if (!P.zl) setTimeout(() => restoreFocus({ fid: 'vbp:zl' }), 0); };
ACT.vbpOther = () => { const P = vbpS(); if (vbIsPhone()) { const el = $('.vbp-chl [tabindex="0"]'); if (el) { el.focus(); el.scrollIntoView({ block: 'center' }); } return; } P.zl = true; P.zlFocus = true; P.zlSel = null; render(); };
ACT.vbpGuide = () => { const P = vbpS(); P.guide = !P.guide; P.zl = false; const st = vbgState('ov'); st.detail = null; st.ctx = null; if (P.guide) { const p = SIM.player(); st.day = vbToday(); st.key = ''; st.jumpT = SIM.now(); st.want = { ch: p ? p.ch : null, t: SIM.now() }; st.focusNow = true; render(); } else { render(); setTimeout(() => restoreFocus({ fid: 'vbp:guide' }), 0); } };
ACT.vbgoDay = (d) => { const st = vbgState('ov'); st.day = Number(d.day); st.jumpT = st.day === vbToday() ? SIM.now() : st.day * 1440 + 20 * 60 + 15; st.detail = null; render(); };
ACT.vbgoDetClose = () => { const st = vbgState('ov'); st.detail = null; st.refocus = true; render(); };
ACT.vbpInfo = () => { const P = vbpS(); P.info = !P.info; if (!P.info) P.infoUntil = 0; render(); };
ACT.vbpMute = () => { const P = vbpS(); P.muted = !P.muted; render(); };
ACT.vbpAudio = (d) => { const P = vbpS(); const p = SIM.player(); if (!p) return; P.audio[p.ch] = Number(d.i); vbpSwitchTrack('Wechsle Tonspur …'); };
/* ACT.vbpSub stays for the keyboard help; the menu offers no selectable entry */
ACT.vbpSub = (d) => { const P = vbpS(); const p = SIM.player(); if (!p) return; P.sub[p.ch] = Number(d.i); const c = SIM.channel(p.ch); toast(Number(d.i) ? `Untertitel: ${esc(c.subs[Number(d.i) - 1])}` : 'Untertitel aus'); };
ACT.vbpQual = (d) => { const P = vbpS(); P.quality = d.q; vbpSwitchTrack('Wechsle Qualität …'); };
function vbpSwitchTrack(text) { const P = vbpS(); P.sw = Date.now() + 800; P.swText = text; render(); setTimeout(render, 820); }
ACT.vbpFs = () => { const d = document.getElementById('device'); try { if (document.fullscreenElement) document.exitFullscreen(); else if (d && d.requestFullscreen) d.requestFullscreen(); else toast('Vollbild geht in diesem Browser nicht.', 'warn'); } catch (e) { toast('Vollbild geht hier nicht.', 'warn'); } };
ACT.vbpKeys = (d) => openDialog('vbKeys', { k: d.k || 'live' });
ACT.vbpRetry = () => { const P = vbpS(); P.retry++; render(); };
ACT.vbpRec = () => { const p = SIM.player(); if (p) LiveDlg.instantRecording(p.ch); };
ACT.vbpStage = () => { const P = vbpS(); if (P.zl || P.guide) { P.zl = false; P.guide = false; render(); return; } if (vbIsPhone()) { ACT.vbpInfo(); return; } if (SIM.rights().timeshift) ACT.vbpPlay(); else ACT.vbpInfo(); };
ACT.vbpPick = (d) => vbSwitch(d.ch, { via: 'click' });
ACT.vbpZcGo = () => { const P = vbpS(); if (!P.zc) return; if ($('[data-vbnoask]') && $('[data-vbnoask]').checked) VB.noAskZap = true; vbSwitch(P.zc.to, { confirmed: true }); };
ACT.vbpZcStay = () => { const P = vbpS(); P.zc = null; render(); setTimeout(() => { const s = $('[data-vbstage]'); if (s) s.focus(); }, 0); };
ACT.vbRecExtend = (d) => { const r = SIM.recording(d.id); if (!r) return; const nn = SIM.nowNext(r.ch).now; if (nn) { r.end = nn.end + LIVE.settings.dvr.padAfter; SIM.emit('change'); toast(`Aufnahme läuft bis ${fmt.time(r.end)}.`); } };
ACT.vbRecStop = (d) => LiveDlg.confirmDelete({ kind: 'running', ids: [d.id] });
document.addEventListener('dblclick', (e) => { if (e.target.closest && e.target.closest('.vbp-pic') && !vbIsPhone()) ACT.vbpFs(); });
document.addEventListener('change', (e) => {
  const el = e.target.closest && e.target.closest('[data-vbzlist]'); if (!el) return;
  const P = vbpS(); P.listName = el.value; P.listIds = vbListIds(el.value); P.zlSel = null;
  history.replaceState(null, '', location.hash.split('?')[0] + vbpQs()); render();
  toast(`Senderwechsel folgt jetzt der Liste «${esc(vbListLabel(el.value))}».`);
});
document.addEventListener('input', (e) => {
  const el = e.target.closest && e.target.closest('[data-vbvol]'); if (!el) return;
  const P = vbpS(); P.vol = Number(el.value); P.muted = P.vol === 0; el.setAttribute('aria-valuetext', P.vol + ' %');
});
/* seek by pointer on the bar */
document.addEventListener('pointerdown', (e) => {
  const trk = e.target.closest && e.target.closest('[data-vbtrk]'); if (!trk) return;
  const at = (ev) => { const r = trk.getBoundingClientRect(); const f = Math.max(0, Math.min(1, (ev.clientX - r.left) / r.width)); const p = SIM.player(); if (!p) return; const m = vbpSeekModel(p); SIM.seek((1 - f) * m.span); };
  at(e); trk.setPointerCapture(e.pointerId);
  const mv = (ev) => at(ev); const up = () => { trk.removeEventListener('pointermove', mv); trk.removeEventListener('pointerup', up); };
  trk.addEventListener('pointermove', mv); trk.addEventListener('pointerup', up);
});
/* number entry (6.7): up to 4 digits, switch after 2 s or Enter, Esc cancels */
function vbpDigit(d) { const P = vbpS(); P.miss = null; P.num = { d: ((P.num ? P.num.d : '') + d).slice(0, 4) }; clearTimeout(P.numT); P.numT = setTimeout(vbpNumCommit, 2000); render(); }
function vbpNumCommit() {
  const P = vbpS(); clearTimeout(P.numT); if (!P.num) return; const n = P.num.d; P.num = null;
  const c = SIM.channelByNo(Number(n));
  if (!c) { P.miss = { n }; clearTimeout(P.missT); P.missT = setTimeout(() => { P.miss = null; render(); }, 2000); render(); return; }
  vbSwitch(c.id, { via: 'num' });
}
/* ---------- keys of the live player (14 4.8, Tastenkarte) ---------- */
document.addEventListener('keydown', (e) => {
  const { path } = parseHash(); if (!path.startsWith('/live/watch/')) return;
  if (e.defaultPrevented || ui.dialog || ui.menu) return;
  const t = e.target; if (t.closest && t.closest('#tools, .lvm, input[type="checkbox"], select, textarea')) return;
  const P = vbpS(); const K = e.key; const mod = e.ctrlKey || e.metaKey || e.altKey;
  vbpIdleArm(document.getElementById('scroll'));
  const inZ = t.closest && t.closest('[data-vbz]'); const inG = t.closest && t.closest('[data-vbgo]');
  if (t.matches && t.matches('input[type="range"]') && (K === 'ArrowUp' || K === 'ArrowDown' || K === 'ArrowLeft' || K === 'ArrowRight')) return;
  if (/^[0-9]$/.test(K) && !mod && !inG) { e.preventDefault(); vbpDigit(K); return; }
  if (P.num && K === 'Enter') { e.preventDefault(); vbpNumCommit(); return; }
  if (K === 'Escape' || (K === 'Backspace' && !inZ)) {
    e.preventDefault();
    if (P.num) { P.num = null; clearTimeout(P.numT); render(); return; }
    if (P.zc) { ACT.vbpZcStay(); return; }
    if (P.guide) { const st = vbgState('ov'); if (st.ctx) return; if (st.detail) { ACT.vbgoDetClose(); return; } ACT.vbpGuide(); return; }
    if (P.zl) { ACT.vbpList(); return; }
    ACT.vbpLeave(); return;
  }
  if (inG) { if (K.toLowerCase() === 'g' && !mod && !(t.closest('.vbg-cell'))) { e.preventDefault(); ACT.vbpGuide(); } return; }
  if (inZ) {
    const opts = $$('[data-vbzch]', inZ); const cur = t.closest('[data-vbzch]'); const k = opts.indexOf(cur);
    const mv = (n) => { n = Math.max(0, Math.min(opts.length - 1, n)); opts.forEach((o) => o.setAttribute('tabindex', '-1')); opts[n].setAttribute('tabindex', '0'); P.zlSel = opts[n].dataset.vbzch; opts[n].focus(); opts[n].scrollIntoView({ block: 'nearest' }); };
    if (K === 'ArrowDown') { e.preventDefault(); mv(k + 1); } else if (K === 'ArrowUp') { e.preventDefault(); mv(k - 1); }
    else if (K === 'Home') { e.preventDefault(); mv(0); } else if (K === 'End') { e.preventDefault(); mv(opts.length - 1); }
    else if (K === 'PageDown') { e.preventDefault(); mv(k + 8); } else if (K === 'PageUp') { e.preventDefault(); mv(k - 8); }
    else if ((K === 'Enter' || K === ' ') && cur) { e.preventDefault(); vbSwitch(cur.dataset.vbzch, { via: 'click' }); }
    else if (K.toLowerCase() === 'c' && !mod && !cur) { e.preventDefault(); ACT.vbpList(); }
    else if (/^\p{L}$/u.test(K) && !mod && cur) { e.preventDefault(); const l = K.toLowerCase(); const n = opts.findIndex((o, i) => i > k && o.dataset.name.startsWith(l)); const n2 = n >= 0 ? n : opts.findIndex((o) => o.dataset.name.startsWith(l)); if (n2 >= 0) mv(n2); }
    return;
  }
  const onCtl = t.closest && t.closest('button, a, input');
  const inTb = t.closest && t.closest('.vbp-tb');
  const slider = t.matches && t.matches('[data-vbtrk]');
  const key = K.length === 1 ? K.toLowerCase() : K;
  if (mod && key !== 'ArrowLeft' && key !== 'ArrowRight') return;
  switch (key) {
    case ' ': if (onCtl && !slider) return; e.preventDefault(); ACT.vbpPlay(); break;
    case 'k': e.preventDefault(); ACT.vbpPlay(); break;
    case 'Enter': if (t.matches && t.matches('[data-vbstage]')) { e.preventDefault(); ACT.vbpPlay(); } break;
    case 'ArrowLeft': case 'ArrowRight': {
      if (inTb && !slider) { e.preventDefault(); const bs = $$('.vbp-tb button:not([aria-disabled="true"]), .vbp-tb input', document).filter((x) => x.offsetParent !== null); const i = bs.indexOf(t.closest('button, input')); const n = bs[(i + (key === 'ArrowRight' ? 1 : -1) + bs.length) % bs.length]; if (n) n.focus(); return; }
      e.preventDefault(); const p = SIM.player(); if (!p) return; if (!SIM.rights().timeshift) { toast(esc('Anhalten und Zurückspulen sind für dieses Profil nicht freigegeben.'), 'warn'); return; }
      ACT.vbpSeek({ s: (key === 'ArrowRight' ? 1 : -1) * (e.shiftKey ? 60 : 10) }); break;
    }
    case 'Home': if (slider) { e.preventDefault(); const p = SIM.player(); if (p) SIM.seek(p.max); } break;
    case 'End': case 'l': e.preventDefault(); ACT.vbpLive(); break;
    case 'ArrowUp': case 'ArrowDown': { e.preventDefault(); P.vol = Math.max(0, Math.min(100, P.vol + (key === 'ArrowUp' ? 10 : -10))); P.muted = P.vol === 0; toast(`Lautstärke ${P.vol} %`, 'ok', 1200); render(); break; }
    case 'PageUp': case 'n': e.preventDefault(); vbZap(1, 'zap1'); break;
    case 'PageDown': case 'p': e.preventDefault(); vbZap(-1, 'zap-1'); break;
    case 'c': e.preventDefault(); if (!vbIsPhone()) ACT.vbpList(); else ACT.vbpOther(); break;
    case 'g': e.preventDefault(); if (vbIsPhone()) go('#/live/guide'); else ACT.vbpGuide(); break;
    case 'r': e.preventDefault(); if (SIM.rights().recordings) { const b = $('[data-fid="vbp:rec"], [data-fid="menu:vbp:rec"]'); if (b) b.click(); } break;
    case 'i': e.preventDefault(); ACT.vbpInfo(); break;
    case 't': case 'u': case 'q': { e.preventDefault(); const id = { t: 'vbp:audio', u: 'vbp:subs', q: 'vbp:set' }[key]; const b = $(`[data-fid="menu:${id}"]`); const sb = key === 'u' ? $('[data-fid="vbp:subs"]') : null; if (b) b.click(); else if (sb) toast(esc(sb.dataset.t), 'warn'); break; }
    case 'm': e.preventDefault(); ACT.vbpMute(); break;
    case 'f': e.preventDefault(); ACT.vbpFs(); break;
    case '?': e.preventDefault(); ACT.vbpKeys({ k: 'live' }); break;
    default:
  }
});
DIALOGS.vbKeys = (pr) => {
  const live = [['Leertaste, K', 'Anhalten und weiter (nur mit Timeshift)'], ['← / →', '10 Sekunden zurück oder vor'], ['Shift + ← / →', '60 Sekunden'], ['↑ / ↓', 'Lautstärke'], ['Bild↑ / Bild↓, N / P', 'Nächster oder vorheriger Sender'], ['0 bis 9', 'Sendernummer'], ['C', 'Senderliste'], ['G', 'Programm über dem Bild'], ['L, Ende', 'Zurück zu Live'], ['R', 'Aufnehmen'], ['I', 'Info Jetzt/Danach'], ['T, U, Q, M, F', 'Ton, Untertitel, Qualität, Stumm, Vollbild'], ['Esc, Rücktaste', 'Menü zu, sonst Player verlassen']];
  const rec = [['Leertaste, K', 'Anhalten und weiter'], ['← / →', '10 Sekunden zurück oder vor'], ['Shift + ← / →', '60 Sekunden'], ['0 bis 9', 'Sprung auf 0 % bis 90 %'], ['↑ / ↓', 'Lautstärke'], ['T, U, Q, M, F', 'Ton, Untertitel, Qualität, Stumm, Vollbild'], ['Esc', 'Player verlassen']];
  const list = pr.k === 'rec' ? rec : live;
  return { title: pr.k === 'rec' ? 'Tasten im Aufnahme-Player' : 'Tasten im Live-Player', body: `<p class="lead">${pr.k === 'rec' ? 'Im Aufnahme-Player springen 0 bis 9 in Prozent, weil die Dauer bekannt ist.' : 'Im Live-Player bedeuten 0 bis 9 Sendernummern. Im Aufnahme-Player springen sie in Prozent.'}</p><dl class="vb-keys">${list.map(([k, v]) => `<dt><kbd>${esc(k)}</kbd></dt><dd>${esc(v)}</dd>`).join('')}</dl>`, actions: `<button type="button" class="btn secondary" data-act="close" data-fid="vbkeys:x" data-autofocus>Schliessen</button>` };
};
/* the player follows the clock every second without a full redraw */
SIM.on((t) => {
  if (t !== 'tick') return;
  const { path } = parseHash();
  if (path.startsWith('/live/watch/')) vbpTick();
});
function vbpTick() {
  const p = SIM.player(); const root = $('[data-vbp]'); if (!p || !root) return;
  if (p.paused && p.max >= LIVE.timeshiftMaxMin - 0.02 && p.offset >= p.max - 0.02) { SIM.resume(); toast(`Du warst zu lange pausiert. Weiter ab ${fmt.time(SIM.now() - p.max)}.`, 'warn'); return; }
  const seek = $('[data-vbseek]', root); if (seek) vbpSeekPatch(seek, p);
  const clk = $('[data-vbclock]', root); if (clk) clk.textContent = fmt.clock(p.position);
  const ind = $('[data-vbind]', root); const kind = p.paused ? 'p' : p.offset >= 0.05 ? 's' : 'l';
  if (ind && ind.dataset.vbind !== kind) { const f = document.activeElement === ind; ind.outerHTML = vbpInd(p); if (f) { const s = $('[data-vbstage]'); if (s) s.focus({ preventScroll: true }); } } else if (ind) { const o = $('[data-vboff]', ind); if (o) o.textContent = fmt.offset(p.offset); }
  $$('[data-vbrecel]', root).forEach((el) => { el.textContent = vbElapsed(Number(el.dataset.s)); });
  const P = vbpS(); $$('[data-vbcount]', root).forEach((el) => { el.textContent = String(20 - (Math.floor((Date.now() - P.readyAt) / 1000) % 20)); });
}
/* leaving the player ends playback (14 0.6): no mini player, no picture-in-picture */
let vbPrevPath = parseHash().path;
window.addEventListener('hashchange', () => {
  const { path } = parseHash();
  if (vbPrevPath.startsWith('/live/watch/') && !path.startsWith('/live/watch/') && SIM.player()) SIM.stop();
  if (!path.startsWith('/live/watch/') && ui.vbp) { ui.vbp.zl = false; ui.vbp.guide = false; ui.vbp.zc = null; ui.vbp.num = null; ui.vbp.firstFocus = ''; }
  vbPrevPath = path;
});
