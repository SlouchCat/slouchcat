/* Web Zuschauer B, part 2: LS-08 Meine Aufnahmen, LS-09 Aufnahme-Detail, LS-10 Aufnahme-Player, LS-11
   Geplante Aufnahmen, LS-12 Konflikte und Warnungen, LS-13 Erinnerungen, LS-14 Aufnahmen meiner Profile and
   the dialogs LD-01 to LD-11 behind LiveDlg.* (LD-09 stays the basic form of the core).
   14-oberflaeche.md 3.8 to 3.14, 5.1 to 5.11. Uses the helpers of viewer-b.js (loaded before). */
'use strict';

/* ---------- helpers for recordings ---------- */
function vbRecAccess(id) {
  const r = SIM.recording(id); if (!r || liveMode() === 'empty') return null; /* «Leer»: no recording exists */
  const me = SIM.profile();
  if (r.profile === me.id) return { r, own: true, prof: me };
  if (me.primary && SIM.childProfiles().some((p) => p.id === r.profile) && SIM.rights(r.profile).recordings) return { r, own: false, prof: SIM.profiles().find((p) => p.id === r.profile) };
  return null;
}
const vbRecPic = (r) => ({ ch: r.ch, title: r.title, img: r.img || ((r.program && SIM.program(r.program)) || {}).img || null });
const vbRecTitle = (r) => r.title + (r.sub ? ` · «${r.sub}»` : '');
const vbRecDur = (r) => Math.max(0, (r.state === 'running' ? Math.min(SIM.now(), r.end) : r.end) - r.start);
const vbRecGB = (r) => (r.state === 'running' ? (r.liveGB || r.sizeGB) : r.sizeGB) || 0;
function vbHms(min) { const s = Math.max(0, Math.round(min * 60)); const h = Math.floor(s / 3600); const mm = String(Math.floor(s / 60) % 60).padStart(h ? 2 : 1, '0'); return `${h ? h + ':' : ''}${mm}:${String(s % 60).padStart(2, '0')}`; }
const vbParseHms = (t) => { const p = String(t).split(':').map(Number); return p.length === 3 ? p[0] * 60 + p[1] + p[2] / 60 : p[0] + (p[1] || 0) / 60; };
function vbResumeAt(r) { if (r.watched || !(r.watchedPct > 0) || r.watchedPct >= 95) return null; return r.resumeAt ? vbParseHms(r.resumeAt) : vbRecDur(r) * r.watchedPct / 100; }
function vbTracks(r) { const out = { audio: ['Deutsch'], subs: [] }; String(r.tracks || '').split(' · ').forEach((part) => { if (part.startsWith('Ton: ')) out.audio = part.slice(5).split(', '); if (part.startsWith('Untertitel: ')) out.subs = part.slice(12).split(', '); }); return out; }
const vbStateWord = (r) => LU.REC_STATE[r.state] || r.state;
const vbKeepKey = (k) => (!k || k.mode === 'forever' ? 'forever' : String(k.days));
function vbKeepFrom(key, start) { if (key === 'std' || key === 'forever' || !key) return { mode: 'forever' }; const days = Number(key); return { mode: 'days', days, until: start + days * 1440 }; }
const vbWdNames = ['So', 'Mo', 'Di', 'Mi', 'Do', 'Fr', 'Sa'];
function vbWdText(w) { if (!w || !w.length) return 'jeden Tag'; const s = [...w].sort((a, b) => ((a + 6) % 7) - ((b + 6) % 7)); if (s.length === 7) return 'täglich'; if (s.join() === '1,2,3,4,5') return 'Mo–Fr'; if (s.join() === '6,0') return 'Sa–So'; return s.map((d) => vbWdNames[d]).join(', '); }
/** time bar of a recording: pre/post padding lighter, gaps hatched (LiveSeekBar, variant recording) */
function vbRecModel(r) {
  const dur = Math.max(1 / 60, vbRecDur(r)); const running = r.state === 'running';
  return { dur, running, pre: Math.min(r.padBefore || 0, dur), post: running ? 0 : Math.min(r.padAfter || 0, dur), gaps: (r.gaps || []).map(([a, b]) => [a - r.start, b - r.start]) };
}
function vbRecTrack(r, m, pos, attrs) {
  const pc = (x) => (Math.max(0, Math.min(1, x / m.dur)) * 100).toFixed(2) + '%';
  return `<div class="vbs-trk rec"${attrs || ''}><span class="vbs-rail"></span>${m.pre ? `<span class="vbs-pad" style="left:0;width:${pc(m.pre)}" title="Vorlauf"></span>` : ''}${m.post ? `<span class="vbs-pad" style="left:${pc(m.dur - m.post)};width:${pc(m.post)}" title="Nachlauf"></span>` : ''}${m.gaps.map(([a, b]) => `<span class="vbs-gap lv-gap" style="left:${pc(a)};width:${pc(b - a)}" title="Lücke ${fmt.range(r.start + a, r.start + b)}"></span>`).join('')}${pos != null ? `<span class="vbs-fill" style="width:${pc(pos)}"></span><span class="vbs-thumb" style="left:${pc(pos)}"></span>` : ''}</div>`;
}
function vbGapText(r) {
  const g = r.gaps || []; if (!g.length) return '';
  const total = Math.round(g.reduce((s, [a, b]) => s + (b - a), 0));
  const when = g.map(([a, b]) => `von ${fmt.time(a)} bis ${fmt.time(b)}`).join(' und ');
  return `Unvollständig: ${g.length === 1 ? '1 Lücke' : g.length + ' Lücken'}, zusammen ${vbMin(total)}. Der Sender war ${when} nicht erreichbar.`;
}
ACT.vbNewRec = () => LiveDlg.manualRecording({});

/* ---------- storage meter (StorageMeter): no names and no titles of other profiles ---------- */
function vbMeter() {
  const q = vbQuota(); const pc = (x) => Math.max(0, Math.min(100, x / q.totalGB * 100)).toFixed(1) + '%';
  const warn = VB.quotaFull ? notice('bad', 'Der Speicher des Kontos ist voll.', 'Neue Aufnahmen werden abgelehnt, bis wieder Platz frei ist. Du und das Hauptprofil bekommt dann eine Meldung.', `<a class="btn secondary sm" href="#/live/recordings?sort=size" data-fid="vbm:size">Nach Grösse sortieren</a>`)
    : q.pct >= 90 ? notice('warn', 'Der Speicher des Kontos ist fast voll.', 'Neue Aufnahmen können abgelehnt werden.') : '';
  return `<section class="vb-meter" aria-label="Speicher des Kontos"><p><b>Speicher des Kontos: ${fmt.gb(q.usedGB)} von ${fmt.gb(q.totalGB)} belegt</b> <span class="muted">(${q.pct} %)</span></p>
    <div class="vb-bar" role="img" aria-label="Du ${fmt.gb(q.mineGB)}, übrige Profile ${fmt.gb(q.othersGB)}, frei ${fmt.gb(q.freeGB)}"><span class="me" style="width:${pc(q.mineGB)}"></span><span class="ot" style="width:${pc(q.othersGB)}"></span></div>
    <ul class="vb-leg"><li><i class="me"></i>Du ${fmt.gb(q.mineGB)}</li><li><i class="ot"></i>Übrige Profile ${fmt.gb(q.othersGB)}</li><li><i class="fr"></i>Frei ${fmt.gb(q.freeGB)}</li></ul></section>${warn}`;
}

/* ======================================================================================
   LS-08 Meine Aufnahmen
   ====================================================================================== */
const VBR_FILTERS = [['', 'Alle'], ['running', 'Läuft'], ['done', 'Fertig'], ['incomplete', 'Unvollständig'], ['failed', 'Fehlgeschlagen']];
const VBR_SORTS = [['new', 'Neueste zuerst'], ['old', 'Älteste zuerst'], ['channel', 'Sender'], ['title', 'Titel'], ['size', 'Grösse'], ['expiry', 'Läuft bald ab']];
const vbFailed = (r) => r.state === 'failed' || r.state === 'rejected';
function vbSortRecs(list, sort) {
  const exp = (r) => (r.keep && r.keep.mode === 'days' ? (r.keep.until || r.start + r.keep.days * 1440) : Infinity);
  const ch = (r) => (SIM.channel(r.ch) || { no: 999 }).no;
  const f = { new: (a, b) => b.start - a.start, old: (a, b) => a.start - b.start, channel: (a, b) => ch(a) - ch(b) || b.start - a.start, title: (a, b) => a.title.localeCompare(b.title, 'de'), size: (a, b) => vbRecGB(b) - vbRecGB(a), expiry: (a, b) => exp(a) - exp(b) }[sort] || ((a, b) => b.start - a.start);
  return list.slice().sort(f);
}
function vbRecHref(q, patch) { const v = Object.assign({}, q, patch); const qs = ['state', 'sort', 'series'].filter((k) => v[k]).map((k) => `${k}=${encodeURIComponent(v[k])}`); return '#/live/recordings' + (qs.length ? '?' + qs.join('&') : ''); }
function vbRecCard(r, o) {
  const c = SIM.channel(r.ch) || { name: r.ch }; const dur = vbRecDur(r); const run = r.state === 'running';
  const chips = [];
  if (run) chips.push(LU.badge('recording', { text: true }));
  if (r.state === 'incomplete') chips.push(LU.badge('incomplete', { text: true }));
  if (r.origin === 'series') chips.push('<span class="chip info">Serie</span>');
  if (r.origin === 'instant' && !run) chips.push('<span class="chip info">Sofortaufnahme</span>');
  if (!run && !r.watched && !(r.watchedPct > 0)) chips.push('<span class="chip blue">Neu</span>');
  if (r.watched) chips.push('<span class="chip ok">Gesehen</span>');
  const line = run ? `Nimmt auf seit ${fmt.time(r.start)} · bis ${fmt.time(r.end)} · ${fmt.gb(vbRecGB(r))} bisher` : vbKeepText(r);
  const sel = o.sel ? `<label class="vbr-chk"><input type="checkbox" data-vbsel="${esc(r.id)}" data-vbx tabindex="-1"${ui.vbSel && ui.vbSel.has(r.id) ? ' checked' : ''} aria-label="${esc(vbRecTitle(r))} auswählen" data-fid="vbsel:${esc(r.id)}"></label>` : '';
  return `<li class="vbr-card${run ? ' run' : ''}${o.sel && ui.vbSel && ui.vbSel.has(r.id) ? ' on' : ''}" data-vbrow="${esc(r.id)}">${sel}
    <a class="vbr-main" href="#/live/recordings/${esc(r.id)}${o.child ? '' : ''}" data-vbi tabindex="${o.tab}" data-fid="vbrc:${esc(r.id)}" aria-label="${esc(vbRecTitle(r))}, ${esc(c.name)}, ${fmt.day(r.start)}, ${esc(vbStateWord(r))}">
      <span class="vbr-art">${LU.picture(vbRecPic(r), { fallback: 'logo' })}${r.watchedPct > 0 && !run ? `<span class="vbr-wp" aria-hidden="true"><i style="width:${Math.min(100, r.watchedPct)}%"></i></span>` : ''}${run ? `<span class="vbr-run">${LU.progress(r.start, r.end, { cls: 'rec', label: 'Fortschritt der Aufnahme' })}</span>` : ''}</span>
      <span class="vbr-bd"><b class="ti">${esc(r.title)}</b>${r.sub ? `<span class="sec su">«${esc(r.sub)}»</span>` : ''}<span class="m">${esc(c.name)} · ${fmt.day(r.start)} · ${fmt.dur(dur)}${r.sizeGB || run ? ' · ' + fmt.gb(vbRecGB(r)) : ''}</span><span class="bdg">${chips.join('')}</span><span class="k">${esc(line)}</span></span></a>
    <span class="vbr-acts">${r.state !== 'failed' && r.state !== 'rejected' ? `<a class="btn ghost icon sm" href="#/live/recordings/${esc(r.id)}/play" data-vbx tabindex="-1" data-fid="vbrp:${esc(r.id)}" aria-label="${esc(r.title)} abspielen">${icon('play', 18)}</a>` : ''}</span></li>`;
}
function vbStackCard(rule, eps, o) {
  const r0 = eps[0]; const gb = eps.reduce((s, x) => s + vbRecGB(x), 0); const fresh = eps.filter((x) => !x.watched && !(x.watchedPct > 0)).length;
  const sel = o.sel ? `<label class="vbr-chk"><input type="checkbox" data-vbsel="${esc(eps.map((x) => x.id).join(','))}" data-vbx tabindex="-1"${eps.every((x) => ui.vbSel && ui.vbSel.has(x.id)) ? ' checked' : ''} aria-label="Alle ${eps.length} Folgen von ${esc(rule.title)} auswählen" data-fid="vbsel:${esc(rule.id)}"></label>` : '';
  return `<li class="vbr-card stack" data-vbrow="${esc(rule.id)}">${sel}<a class="vbr-main" href="${vbRecHref(parseHash().query, { series: rule.id, state: '' })}" data-vbi tabindex="${o.tab}" data-fid="vbrs:${esc(rule.id)}" aria-label="${esc(rule.title)}, ${eps.length} Folgen, Folgenliste öffnen">
    <span class="vbr-art"><span class="vbr-stk" aria-hidden="true"></span>${LU.picture(vbRecPic(r0), { fallback: 'logo' })}<span class="vbr-n">${eps.length} Folgen</span></span>
    <span class="vbr-bd"><b class="ti">${esc(rule.title)} · ${eps.length} Folgen</b><span class="m">Serienaufnahme · ${fmt.gb(gb)}</span><span class="bdg"><span class="chip info">Serie</span>${fresh ? `<span class="chip blue">${fresh} neu</span>` : ''}</span><span class="k">${rule.keepLast ? `Behalte die letzten ${rule.keepLast} Folgen` : 'Alle Folgen behalten'}</span></span></a><span class="vbr-acts"></span></li>`;
}
route('/live/recordings', (pm, q) => {
  const mode = liveMode(); const mine = SIM.recordings(); const me = SIM.profile();
  const sort = q.sort || 'new'; const stf = q.state || ''; const series = q.series || '';
  if (!ui.vbSel) ui.vbSel = new Set();
  const selOn = !!ui.vbSelOn;
  const nUp = vbUpcoming().length;
  const acts = `<a class="btn secondary" href="#/live/scheduled" data-fid="vbr:sched">${icon('calendar', 18)}Geplant (${nUp})</a><button type="button" class="btn primary" data-act="vbNewRec" data-fid="vbr:new">${icon('plus', 18)}Neue Aufnahme</button>`;
  const kids = me.primary ? SIM.childProfiles().filter((p) => SIM.rights(p.id).recordings) : [];
  const link = kids.length ? `<a class="vb-link" href="#/live/recordings/profiles" data-fid="vbr:kids">${icon('users', 20)}<span class="grow"><b>Aufnahmen meiner Profile</b><span class="muted"> · ${esc(kids.map((k) => k.name).join(', '))}</span></span>${icon('chevR', 18)}</a>` : '';
  const counts = { '': mine.length, running: mine.filter((r) => r.state === 'running').length, done: mine.filter((r) => r.state === 'done').length, incomplete: mine.filter((r) => r.state === 'incomplete').length, failed: mine.filter(vbFailed).length };
  const tools = `<div class="vbr-tools"><div class="seg vb-seg" role="group" aria-label="Zustand">${VBR_FILTERS.map(([k, l]) => `<button type="button" aria-pressed="${stf === k}" data-act="vbGo" data-href="${vbRecHref(q, { state: k })}" data-fid="vbrf:${k || 'all'}">${l} <span class="n">${counts[k]}</span></button>`).join('')}</div>
    <label class="fld fsel"><span class="lbl">Sortierung</span><select class="sel" data-vbrsort data-fid="vbr:sort" aria-label="Sortierung">${VBR_SORTS.map(([k, l]) => `<option value="${k}"${k === sort ? ' selected' : ''}>${l}</option>`).join('')}</select></label>
    <button type="button" class="btn ${selOn ? 'primary' : 'secondary'}" data-act="vbSelMode" aria-pressed="${selOn}" data-fid="vbr:selmode">${icon('check', 18)}${selOn ? 'Auswahl beenden' : 'Auswählen'}</button></div>`;
  let body;
  if (mode === 'loading') body = skelRows(4, 120);
  else if (mode === 'error') body = liveError('Deine Aufnahmen konnten nicht geladen werden.');
  else if (!mine.length) body = emptyState('Noch keine Aufnahmen', 'Plane Aufnahmen in der Programmzeitschrift oder starte eine im Player.', `<a class="btn secondary" href="#/live/guide" data-fid="vbr:guide">Zum Programm</a>`);
  else {
    const failed = mine.filter(vbFailed);
    let list = mine.filter((r) => !vbFailed(r));
    if (stf === 'failed') list = [];
    else if (stf) list = list.filter((r) => r.state === stf);
    let head = '';
    if (series) { const rule = SIM.rule(series); list = list.filter((r) => r.rule === series); head = `<div class="vbr-sh"><a class="btn ghost sm" href="${vbRecHref(q, { series: '' })}" data-fid="vbr:allrec">${icon('chevL', 16)}Alle Aufnahmen</a><h2 class="ghead">${esc(rule ? rule.title : 'Serie')} · ${list.length} Folgen</h2>${rule ? `<span class="muted">${rule.keepLast ? `Behalte die letzten ${rule.keepLast} Folgen` : 'Alle Folgen behalten'}</span>` : ''}</div>`; }
    list = vbSortRecs(list, sort);
    const running = list.filter((r) => r.state === 'running'); const rest = list.filter((r) => r.state !== 'running');
    const items = [];
    const seen = new Set();
    rest.forEach((r) => {
      if (!series && r.rule && r.origin === 'series') { if (seen.has(r.rule)) return; const eps = rest.filter((x) => x.rule === r.rule); if (eps.length > 1) { seen.add(r.rule); items.push({ stack: SIM.rule(r.rule) || { id: r.rule, title: r.title }, eps }); return; } }
      items.push({ r });
    });
    const ids = items.map((x) => (x.stack ? x.stack.id : x.r.id)); const rids = running.map((r) => r.id);
    const card = (x) => (x.stack ? vbStackCard(x.stack, x.eps, { sel: selOn, tab: vbTab('vbr', x.stack.id, ids) }) : vbRecCard(x.r, { sel: selOn, tab: vbTab('vbr', x.r.id, ids) }));
    const runHtml = running.length ? `<section class="stack"><h2 class="ghead">Läuft</h2><ul class="vbr-grid" data-vbrove="grid" data-vbkey="vbrrun" aria-label="Laufende Aufnahmen">${running.map((r) => vbRecCard(r, { sel: selOn, tab: vbTab('vbrrun', r.id, rids) })).join('')}</ul></section>` : '';
    const gridHtml = items.length ? `<section class="stack">${running.length ? '<h2 class="ghead">Aufnahmen</h2>' : ''}<ul class="vbr-grid" data-vbrove="grid" data-vbkey="vbr" aria-label="Aufnahmen, Pfeiltasten wechseln, Enter öffnet">${items.map(card).join('')}</ul></section>` : '';
    const empty = !running.length && !items.length && stf !== 'failed' ? emptyState('Keine Aufnahmen mit diesem Zustand.', '', `<a class="btn secondary" href="${vbRecHref(q, { state: '', series: '' })}" data-fid="vbr:clear">Alle zeigen</a>`) : '';
    const fl = (!stf || stf === 'failed') && !series && failed.length ? band('vbr-failed', 'Fehlgeschlagen und nicht aufgenommen', `${failed.length === 1 ? '1 Aufnahme' : failed.length + ' Aufnahmen'}`, `<ul class="vbr-fail" data-vbrove="list" data-vbkey="vbrf">${failed.map((r) => { const c = SIM.channel(r.ch) || { name: r.ch }; return `<li data-vbrow="${esc(r.id)}"><a class="vbr-fl" href="#/live/recordings/${esc(r.id)}" data-vbi tabindex="${vbTab('vbrf', r.id, failed.map((x) => x.id))}" data-fid="vbrfl:${esc(r.id)}">${LU.badge('failed', { text: true, label: vbStateWord(r) })}<span class="grow"><b>${esc(vbRecTitle(r))}</b><span class="muted"> · ${esc(c.name)} · ${fmt.day(r.start)}, ${fmt.range(r.start, r.end)}</span><span class="sec rs">${esc(LU.REASON[r.reason] || '')}</span></span></a><button type="button" class="btn secondary sm" data-act="vbDelRec" data-id="${esc(r.id)}" data-vbx tabindex="-1" data-fid="vbrfd:${esc(r.id)}">${icon('trash', 16)}Entfernen</button></li>`; }).join('')}</ul>`, { viewer: true, open: stf === 'failed', icon: 'alert' }) : (stf === 'failed' && !failed.length ? emptyState('Keine Aufnahmen mit diesem Zustand.') : '');
    body = head + runHtml + gridHtml + empty + fl;
  }
  const selN = [...ui.vbSel].filter((id) => SIM.recording(id)).length;
  const selGB = [...ui.vbSel].map((id) => SIM.recording(id)).filter(Boolean).reduce((s, r) => s + vbRecGB(r), 0);
  const selbar = selOn ? `<div class="vb-selbar" role="region" aria-label="Auswahl"><b>${selN} ausgewählt</b><span class="muted">${fmt.gb(selGB)}</span><span class="grow"></span><button type="button" class="btn ghost sm" data-act="vbSelMode" data-fid="vbsb:x">Abbrechen</button><button type="button" class="btn danger sm" data-act="vbSelDel" data-fid="vbsb:del"${selN ? '' : ' aria-disabled="true"'}>${icon('trash', 16)}Löschen</button></div>` : '';
  return { type: 'viewer', nav: 'live', title: 'Meine Aufnahmen', html: livePage({ tab: 'recordings', title: 'Meine Aufnahmen', count: mode === 'normal' ? `${mine.length} ${mine.length === 1 ? 'Aufnahme' : 'Aufnahmen'}` : '', actions: acts, html: (mode === 'normal' || mode === 'empty' ? vbMeter() : '') + link + (mode === 'normal' && mine.length ? tools : '') + `<div class="vbr-body">${body}</div>` + selbar }) };
});
ACT.vbSelMode = () => { ui.vbSelOn = !ui.vbSelOn; ui.vbSel = new Set(); render(); };
ACT.vbSelDel = () => { const ids = [...ui.vbSel].filter((id) => SIM.recording(id)); if (!ids.length) return; LiveDlg.confirmDelete({ kind: ids.length === 1 ? 'recording' : 'recordings', ids, onDone: () => { ui.vbSel = new Set(); ui.vbSelOn = false; } }); };
ACT.vbDelRec = (d) => LiveDlg.confirmDelete({ kind: 'recording', ids: [d.id] });
document.addEventListener('change', (e) => {
  const s = e.target.closest && e.target.closest('[data-vbrsort]');
  if (s) { history.replaceState(null, '', vbRecHref(parseHash().query, { sort: s.value === 'new' ? '' : s.value })); render(); return; }
  const c = e.target.closest && e.target.closest('[data-vbsel]'); if (!c) return;
  if (!ui.vbSel) ui.vbSel = new Set();
  c.dataset.vbsel.split(',').forEach((id) => (c.checked ? ui.vbSel.add(id) : ui.vbSel.delete(id)));
  render();
});

/* ======================================================================================
   LS-14 Aufnahmen meiner Profile (registered before LS-09 so the static route wins)
   ====================================================================================== */
route('/live/recordings/profiles', () => {
  const kids = SIM.childProfiles().filter((p) => SIM.rights(p.id).recordings);
  if (!kids.length) return notFoundView();
  const mode = liveMode();
  let body;
  if (mode === 'loading') body = skelRows(2, 60);
  else if (mode === 'error') body = liveError('Die Aufnahmen der Profile konnten nicht geladen werden.');
  else {
    const all = kids.map((p) => ({ p, recs: SIM.recordings(p.id), up: vbUpcoming(p.id) }));
    if (!all.some((x) => x.recs.length)) body = emptyState(`Die Profile ${esc(kids.map((k) => k.name).join(' und '))} haben keine Aufnahmen.`, '');
    else body = all.map(({ p, recs, up }) => {
      const gb = recs.reduce((s, r) => s + vbRecGB(r), 0);
      const ids = recs.map((r) => r.id);
      const rows = recs.length ? `<ul class="vbk-list" data-vbrove="list" data-vbkey="vbk-${p.id}">${recs.map((r) => { const c = SIM.channel(r.ch) || { name: r.ch }; const ok = !vbFailed(r); return `<li class="vbk-row" data-vbrow="${esc(r.id)}"><a class="vbk-main" href="#/live/recordings/${esc(r.id)}" data-vbi tabindex="${vbTab('vbk-' + p.id, r.id, ids)}" data-fid="vbk:${esc(r.id)}"><span class="vbk-art">${LU.picture(vbRecPic(r), { fallback: 'logo' })}</span><span class="grow"><b>${esc(vbRecTitle(r))}</b><span class="muted">${esc(c.name)} · ${fmt.day(r.start)} · ${fmt.dur(vbRecDur(r))}${r.sizeGB ? ' · ' + fmt.gb(vbRecGB(r)) : ''}</span><span class="bdg">${r.state === 'done' ? '' : LU.badge(r.state === 'running' ? 'recording' : r.state === 'incomplete' ? 'incomplete' : 'failed', { text: true, label: vbStateWord(r) })}${r.origin === 'series' ? '<span class="chip info">Serie</span>' : ''}</span></span></a>${ok ? `<a class="btn secondary sm" href="#/live/recordings/${esc(r.id)}/play" data-vbx tabindex="-1" data-fid="vbkp:${esc(r.id)}">${icon('play', 16)}Abspielen</a>` : ''}<button type="button" class="btn ghost sm danger" data-act="vbKidDel" data-id="${esc(r.id)}" data-vbx tabindex="-1" data-fid="vbkd:${esc(r.id)}">${icon('trash', 16)}Löschen</button></li>`; }).join('')}</ul>` : `<p class="muted">${esc(p.name)} hat keine Aufnahmen.</p>`;
      const meta = `${recs.length === 1 ? '1 Aufnahme' : recs.length + ' Aufnahmen'} · ${fmt.gb(gb)} · ${up.length} geplant`;
      return band('vbk-' + p.id, `${avatarHtml(p, 'vbk-av')}<span>${esc(p.name)} <span class="muted vbk-age">ab ${p.age}</span></span>`, esc(meta), rows + `<p class="muted vbk-note">Geplante Termine von ${esc(p.name)} siehst du hier nur als Zahl. Ob du sie ändern darfst, ist noch offen (F-4).</p>`, { viewer: true });
    }).join('');
  }
  return { type: 'viewer', nav: 'live', title: 'Aufnahmen meiner Profile', html: livePage({ tab: 'recordings', title: 'Aufnahmen meiner Profile', sub: 'Hier siehst du die Aufnahmen der Profile mit Altersgrenze. Aufnahmen von Profilen ohne Altersgrenze bleiben privat.', actions: `<a class="btn ghost" href="#/live/recordings" data-fid="vbk:back">${icon('chevL', 18)}Meine Aufnahmen</a>`, html: `<div class="stack vbk">${body}</div>` }) };
});
ACT.vbKidDel = (d) => LiveDlg.confirmDelete({ kind: 'childRecording', ids: [d.id] });

/* ======================================================================================
   LS-10 Aufnahme-Player
   ====================================================================================== */
function vbrS(id) { if (!ui.vbr || ui.vbr.id !== id) ui.vbr = { id, pos: null, playing: false, at: Date.now(), choice: false, ended: false, vol: 70, muted: false, audio: 0, sub: 0, quality: 'orig', sw: 0, swText: '', firstFocus: false }; return ui.vbr; }
const vbrPos = (S) => (S.playing ? S.pos + (Date.now() - S.at) / 60000 : S.pos);
function vbrSet(S, pos, m) { S.pos = Math.max(0, Math.min(m.dur, pos)); S.at = Date.now(); S.ended = false; }
function vbrPic(r, paused) {
  const c = SIM.channel(r.ch) || { name: r.ch, slug: r.ch, id: r.ch }; const pic = vbRecPic(r);
  return `<div class="lv-vid${paused ? ' paused' : ''}">${pic.img && LU.imagesOn() ? `<img class="lv-vid-pic" src="${LU.img}lib/${esc(pic.img)}" alt="" onerror="this.remove()">` : ''}<div class="lv-vid-move" aria-hidden="true"></div><div class="lv-vid-logo">${LU.logo(c)}</div><span class="lv-vid-tag">${esc(LU.TX.sample)} · Aufnahme</span></div>`;
}
function vbrHtml(a, S) {
  const r = a.r; const c = SIM.channel(r.ch) || { name: r.ch, no: '' }; const phone = vbIsPhone();
  if (vbFailed(r)) return `<div class="vbp" data-vbr><div class="vbp-stage" data-vbstage tabindex="0" data-fid="vbr:stage"><div class="vbp-pic"><div class="vbp-black"></div></div><div class="vbp-top">${vbpIb('vbrLeave', 'arrowL', 'Zurück', { kbd: 'Esc', txt: 'Zurück' })}</div><div class="vbp-err" role="alert">${LU.logo(c)}<b>${esc(vbRecTitle(r))}</b><p>Diese Aufnahme lässt sich nicht abspielen. ${esc(LU.REASON[r.reason] || '')}</p><div class="row-gap"><button type="button" class="btn primary" data-act="vbrLeave" data-fid="vbr:back">${icon('arrowL', 18)}Zurück</button></div></div></div></div>`;
  const m = vbRecModel(r); const pos = Math.min(vbrPos(S), m.dur);
  const tr = vbTracks(r);
  const aud = popMenu('vbr:audio', `${icon('vbAudio', 22)}<span class="vbp-tip" aria-hidden="true">Ton <kbd>T</kbd></span>`, tr.audio.map((x, i) => ({ label: x, act: 'vbrAudio', data: { i }, icon: i === S.audio ? 'check' : 'vbBlank' })), { cls: 'vbp-ib', aria: 'Ton: ' + tr.audio[S.audio] });
  const subs = tr.subs.length ? popMenu('vbr:subs', `${icon('vbSub', 22)}<span class="vbp-tip" aria-hidden="true">Untertitel <kbd>U</kbd></span>`, [{ label: 'Aus', act: 'vbrSub', data: { i: 0 }, icon: S.sub === 0 ? 'check' : 'vbBlank' }, ...tr.subs.map((x, i) => ({ label: x, act: 'vbrSub', data: { i: i + 1 }, icon: S.sub === i + 1 ? 'check' : 'vbBlank' }))], { cls: 'vbp-ib', aria: 'Untertitel' }) : `<button type="button" class="vbp-ib" aria-disabled="true" data-fid="vbr:subs" aria-label="Keine Untertitel in dieser Aufnahme">${icon('vbSub', 22)}<span class="vbp-tip" aria-hidden="true">Keine Untertitel aufgenommen</span></button>`;
  const set = popMenu('vbr:set', `${icon('settings', 22)}<span class="vbp-tip" aria-hidden="true">Wiedergabequalität <kbd>Q</kbd></span>`, [['orig', 'Original'], ['720', '720p'], ['480', '480p']].map(([k, l]) => ({ label: 'Wiedergabe: ' + l, act: 'vbrQual', data: { q: k }, icon: S.quality === k ? 'check' : 'vbBlank' })).concat([{ label: 'Tastenkürzel (?)', act: 'vbpKeys', data: { k: 'rec' }, icon: 'help' }]), { cls: 'vbp-ib', aria: 'Einstellungen' });
  const choice = S.choice ? `<div class="vbr-choice" role="dialog" aria-label="Wo weiterschauen?"><b>${esc(r.title)}</b><div class="stack"><button type="button" class="btn primary" data-act="vbrFrom" data-f="resume" data-fid="vbrc:resume">${icon('play', 18)}Weiter ab ${vbHms(vbResumeAt(r))}</button><button type="button" class="btn secondary" data-act="vbrFrom" data-f="start" data-fid="vbrc:start">Von vorne</button>${m.pre ? `<button type="button" class="btn ghost" data-act="vbrFrom" data-f="pre" data-fid="vbrc:pre">Ab Aufnahmebeginn (mit Vorlauf)</button>` : ''}</div></div>` : '';
  const ended = S.ended ? `<div class="vbr-choice" role="status"><b>Ende der Aufnahme</b><div class="row-gap"><button type="button" class="btn primary" data-act="vbrFrom" data-f="start" data-fid="vbre:again">${icon('refresh', 18)}Nochmals</button><a class="btn secondary" href="#/live/recordings" data-fid="vbre:list">Zu meinen Aufnahmen</a></div></div>` : '';
  const gapAt = m.gaps.find(([x, y]) => pos >= x && pos < y);
  const sw = Date.now() < S.sw ? `<div class="vbp-load dim" role="status"><span class="spin lg"></span><b>${esc(S.swText)}</b></div>` : '';
  const head = `<div class="vbp-head"><p class="t">${esc(r.title)}${r.sub ? ` · «${esc(r.sub)}»` : ''}</p><p class="d">${esc(c.name)} · ${fmt.day(r.start)}, ${fmt.range(r.start, r.end)} · Aufnahme${a.own ? '' : ` von ${esc(a.prof.name)} · Fortschritt wird nicht gespeichert`}</p></div>`;
  const trk = vbRecTrack(r, m, pos, ` role="slider" tabindex="0" data-vbrtrk data-fid="vbr:seek" aria-label="Position in der Aufnahme, Pfeiltasten 10 Sekunden, 0 bis 9 springen in Prozent" aria-valuemin="0" aria-valuemax="${Math.round(m.dur * 60)}" aria-valuenow="${Math.round(pos * 60)}" aria-valuetext="${vbHms(pos)} von ${vbHms(m.dur)}"`);
  const play = vbpIb('vbrPlay', S.playing ? 'pause' : 'play', S.playing ? 'Anhalten' : 'Abspielen', { cls: phone ? 'lg' : 'mid', kbd: 'Leertaste', size: phone ? 30 : 24, fid: 'play' });
  const b10 = vbpIb('vbrSeek', 'vbBack', '10 Sekunden zurück', { data: 'data-s="-10"', fid: 'b10', n: '10', kbd: '←' });
  const f10 = vbpIb('vbrSeek', 'vbFwd', '10 Sekunden vor', { data: 'data-s="10"', fid: 'f10', n: '10', kbd: '→' });
  const chap = `<button type="button" class="vbp-ib" aria-disabled="true" data-fid="vbr:chap" aria-label="Keine Kapitel">${icon('vbChapter', 22)}<span class="vbp-tip" aria-hidden="true">Keine Kapitel</span></button>`;
  const P = S;
  const vol = `<span class="vbp-vol">${vbpIb('vbrMute', !P.muted && P.vol ? 'vbVol' : 'vbMute', P.muted ? 'Ton an' : 'Stumm schalten', { kbd: 'M', pressed: P.muted })}<input type="range" class="vbp-range" min="0" max="100" step="5" value="${P.muted ? 0 : P.vol}" data-vbrvol data-fid="vbr:vol" aria-label="Lautstärke"></span>`;
  return `<div class="vbp${phone ? ' ph' : ''}" data-vbr><div class="vbp-stage${phone ? ' ph' : ''}" data-vbstage tabindex="0" data-fid="vbr:stage" aria-label="Aufnahme ${esc(r.title)} (simuliert). Leertaste hält an, 0 bis 9 springen in Prozent, Esc verlässt den Player">
    <div class="vbp-pic" data-act="vbrStage">${vbrPic(r, !S.playing)}</div>
    <div class="vbr-gapov${gapAt ? '' : ' off'}" data-vbrgap role="status"><span class="lv-gap-text">Lücke in der Aufnahme: ${esc(LU.REASON[r.reason] || 'Der Sender war nicht erreichbar.')}</span></div>${sw}${choice}${ended}
    <div class="vbp-top">${vbpIb('vbrLeave', 'arrowL', 'Zurück', { kbd: 'Esc', txt: phone ? '' : 'Zurück' })}${head}${m.running ? `<span class="chip vbp-recchip">${LU.badge('recording', { text: true, label: 'Aufnahme läuft' })}</span>` : ''}${phone ? aud + subs + set : ''}</div>
    ${phone ? `<div class="vbp-mid">${b10}${play}${f10}</div>` : ''}
    <div class="vbp-bot"><div class="vbs rec" data-vbrseek><div class="vbs-lbls"><span class="l">Aufnahme ${fmt.range(r.start, r.end)}${m.pre || m.post ? ` · Vorlauf ${r.padBefore || 0} · Nachlauf ${r.padAfter || 0} Min. heller` : ''}</span><span class="r">${m.gaps.length ? 'Lücken schraffiert' : ''}</span></div>
      <div class="vbs-bar">${trk}${m.running ? `<button type="button" class="vbp-live off" data-act="vbrEdge" data-fid="vbr:edge">Zum aktuellen Stand</button>` : ''}</div>
      <div class="vbs-sub"><span><span data-vbrpos>${vbHms(pos)}</span> / <span data-vbrdur>${vbHms(m.dur)}</span>${m.running ? ' bisher' : ''}</span><span>Gesendet um <span data-vbrclock>${fmt.clock(r.start + pos)}</span></span></div></div>
      ${phone ? '' : `<div class="vbp-tb" role="toolbar" aria-label="Wiedergabe, Pfeiltasten wechseln den Knopf"><span class="grp">${b10}${play}${f10}<span class="sep"></span>${chap}</span><span class="grp">${vol}${aud}${subs}${set}${vbpIb('vbpFs', 'vbFull', 'Vollbild', { kbd: 'F' })}</span></div>`}</div>
  </div>${phone ? `<div class="vbp-below"><section class="vbp-card2"><b>${esc(vbRecTitle(r))}</b><p class="muted">${esc(c.name)} · ${fmt.day(r.start)} · ${fmt.dur(m.dur)}</p><div class="row-gap"><a class="btn ghost sm" href="#/live/recordings/${esc(r.id)}" data-fid="vbr:det">${icon('info', 16)}Details</a></div></section>${LU.sampleNote()}</div>` : ''}</div>`;
}
route('/live/recordings/:id/play', (pm, q) => {
  const a = vbRecAccess(pm.id); if (!a) return notFoundView();
  const r = a.r; const S = vbrS(r.id); const m = vbRecModel(r);
  if (S.pos == null && !vbFailed(r)) {
    const res = a.own ? vbResumeAt(r) : null; /* a child's recording played by the primary profile: no «Weiter ab», start at the programme (DR-21) */
    if (q.from === 'resume' && res != null) { vbrSet(S, res, m); S.playing = true; }
    else if (q.from === 'pre') { vbrSet(S, 0, m); S.playing = true; }
    else if (q.from === 'start' || res == null) { vbrSet(S, m.pre, m); S.playing = true; }
    else { vbrSet(S, res, m); S.choice = true; S.playing = false; }
  }
  return { type: 'bare', title: `${r.title} · Aufnahme`, html: vbrHtml(a, S), after: (root) => {
    if (!S.firstFocus) { S.firstFocus = true; const f = S.choice ? $('[data-fid="vbrc:resume"]', root) : $('[data-vbstage]', root); if (f) f.focus({ preventScroll: true }); }
    vbpIdleArm(root);
  } };
});
function vbrCur() { const m = parseHash().path.match(/^\/live\/recordings\/([^/]+)\/play$/); if (!m) return null; const a = vbRecAccess(decodeURIComponent(m[1])); return a ? { a, S: vbrS(a.r.id), m: vbRecModel(a.r) } : null; }
ACT.vbrPlay = () => { const x = vbrCur(); if (!x) return; const { S, m } = x; if (S.ended) { vbrSet(S, m.pre, m); S.playing = true; render(); return; } const p = vbrPos(S); vbrSet(S, p, m); S.playing = !S.playing; S.choice = false; render(); };
ACT.vbrSeek = (d) => { const x = vbrCur(); if (!x) return; const { S, m } = x; vbrSet(S, vbrPos(S) + Number(d.s) / 60, m); render(); };
ACT.vbrEdge = () => { const x = vbrCur(); if (!x) return; vbrSet(x.S, x.m.dur, x.m); x.S.playing = true; render(); };
ACT.vbrFrom = (d) => { const x = vbrCur(); if (!x) return; const { a, S, m } = x; const res = vbResumeAt(a.r); vbrSet(S, d.f === 'resume' && res != null ? res : d.f === 'pre' ? 0 : m.pre, m); S.choice = false; S.playing = true; render(); setTimeout(() => { const s = $('[data-vbstage]'); if (s) s.focus(); }, 0); };
ACT.vbrStage = () => ACT.vbrPlay();
ACT.vbrMute = () => { const x = vbrCur(); if (!x) return; x.S.muted = !x.S.muted; render(); };
ACT.vbrAudio = (d) => { const x = vbrCur(); if (!x) return; x.S.audio = Number(d.i); x.S.sw = Date.now() + 700; x.S.swText = 'Wechsle Tonspur …'; render(); setTimeout(render, 720); };
ACT.vbrSub = (d) => { const x = vbrCur(); if (!x) return; x.S.sub = Number(d.i); toast(Number(d.i) ? 'Untertitel an' : 'Untertitel aus'); render(); };
ACT.vbrQual = (d) => { const x = vbrCur(); if (!x) return; x.S.quality = d.q; x.S.sw = Date.now() + 700; x.S.swText = 'Wechsle Qualität …'; render(); setTimeout(render, 720); };
ACT.vbrLeave = () => { if (history.length > 1) history.back(); else go('#/live/recordings'); };
document.addEventListener('input', (e) => { const el = e.target.closest && e.target.closest('[data-vbrvol]'); if (!el) return; const x = vbrCur(); if (!x) return; x.S.vol = Number(el.value); x.S.muted = x.S.vol === 0; });
document.addEventListener('pointerdown', (e) => {
  const trk = e.target.closest && e.target.closest('[data-vbrtrk]'); if (!trk) return;
  const at = (ev) => { const x = vbrCur(); if (!x) return; const rc = trk.getBoundingClientRect(); const f = Math.max(0, Math.min(1, (ev.clientX - rc.left) / rc.width)); vbrSet(x.S, f * x.m.dur, x.m); x.S.choice = false; vbrTick(); };
  at(e); trk.setPointerCapture(e.pointerId);
  const mv = (ev) => at(ev); const up = () => { trk.removeEventListener('pointermove', mv); trk.removeEventListener('pointerup', up); render(); };
  trk.addEventListener('pointermove', mv); trk.addEventListener('pointerup', up);
});
function vbrTick() {
  const x = vbrCur(); const root = $('[data-vbr]'); if (!x || !root || vbFailed(x.a.r)) return;
  const { a, S, m } = x; let pos = vbrPos(S);
  if (pos >= m.dur) { if (m.running) { vbrSet(S, m.dur, m); pos = m.dur; } else if (S.playing) { vbrSet(S, m.dur, m); S.playing = false; S.ended = true; render(); return; } }
  const pc = (v) => (Math.max(0, Math.min(1, v / m.dur)) * 100).toFixed(2) + '%';
  const f = $('.vbs-fill', root), th = $('.vbs-thumb', root), trk = $('[data-vbrtrk]', root);
  if (f) f.style.width = pc(pos); if (th) th.style.left = pc(pos);
  if (trk) { trk.setAttribute('aria-valuemax', Math.round(m.dur * 60)); trk.setAttribute('aria-valuenow', Math.round(pos * 60)); trk.setAttribute('aria-valuetext', `${vbHms(pos)} von ${vbHms(m.dur)}`); }
  const p1 = $('[data-vbrpos]', root); if (p1) p1.textContent = vbHms(pos); const d1 = $('[data-vbrdur]', root); if (d1) d1.textContent = vbHms(m.dur); const c1 = $('[data-vbrclock]', root); if (c1) c1.textContent = fmt.clock(a.r.start + pos);
  const g = $('[data-vbrgap]', root); if (g) g.classList.toggle('off', !m.gaps.some(([u, v]) => pos >= u && pos < v));
}
SIM.on((t) => { if (t === 'tick' && /^\/live\/recordings\/[^/]+\/play$/.test(parseHash().path)) vbrTick(); });
/* progress per profile: saved when the player is left; a child's recording played by the primary profile saves no progress at all (DR-21) */
let vbrPrev = parseHash().path;
window.addEventListener('hashchange', () => {
  const m = vbrPrev.match(/^\/live\/recordings\/([^/]+)\/play$/);
  if (m && ui.vbr && ui.vbr.id === decodeURIComponent(m[1])) {
    const a = vbRecAccess(ui.vbr.id);
    if (a && a.own && !vbFailed(a.r)) { const mm = vbRecModel(a.r); const pct = Math.round(Math.min(vbrPos(ui.vbr), mm.dur) / mm.dur * 100); a.r.resumeAt = null; SIM.setWatched(a.r.id, pct); }
    ui.vbr = null;
  }
  vbrPrev = parseHash().path;
});
document.addEventListener('keydown', (e) => {
  const x = vbrCur(); if (!x) return;
  if (e.defaultPrevented || ui.dialog || ui.menu) return;
  const t = e.target; if (t.closest && t.closest('#tools, .lvm, select, textarea')) return;
  const K = e.key; const key = K.length === 1 ? K.toLowerCase() : K; const mod = e.ctrlKey || e.metaKey || e.altKey;
  if (t.matches && t.matches('input[type="range"]') && /^Arrow/.test(K)) return;
  if (mod) return;
  vbpIdleArm(document.getElementById('scroll'));
  const onCtl = t.closest && t.closest('button, a, input'); const inTb = t.closest && t.closest('.vbp-tb'); const slider = t.matches && t.matches('[data-vbrtrk]');
  const { S, m } = x;
  if (/^[0-9]$/.test(K)) { e.preventDefault(); vbrSet(S, m.dur * Number(K) / 10, m); S.choice = false; toast(`Sprung auf ${Number(K) * 10} %`, 'ok', 1200); render(); return; }
  switch (key) {
    case 'Escape': case 'Backspace': e.preventDefault(); ACT.vbrLeave(); break;
    case ' ': if (onCtl && !slider) return; e.preventDefault(); ACT.vbrPlay(); break;
    case 'k': e.preventDefault(); ACT.vbrPlay(); break;
    case 'Enter': if (t.matches && t.matches('[data-vbstage]')) { e.preventDefault(); ACT.vbrPlay(); } break;
    case 'ArrowLeft': case 'ArrowRight': {
      if (inTb && !slider) { e.preventDefault(); const bs = $$('.vbp-tb button:not([aria-disabled="true"]), .vbp-tb input').filter((b) => b.offsetParent !== null); const i = bs.indexOf(t.closest('button, input')); const n = bs[(i + (key === 'ArrowRight' ? 1 : -1) + bs.length) % bs.length]; if (n) n.focus(); return; }
      e.preventDefault(); ACT.vbrSeek({ s: (key === 'ArrowRight' ? 1 : -1) * (e.shiftKey ? 60 : 10) }); break;
    }
    case 'Home': if (slider) { e.preventDefault(); vbrSet(S, 0, m); render(); } break;
    case 'End': if (slider) { e.preventDefault(); vbrSet(S, m.dur, m); render(); } break;
    case 'ArrowUp': case 'ArrowDown': e.preventDefault(); S.vol = Math.max(0, Math.min(100, S.vol + (key === 'ArrowUp' ? 10 : -10))); S.muted = S.vol === 0; toast(`Lautstärke ${S.vol} %`, 'ok', 1200); render(); break;
    case 't': case 'u': case 'q': { e.preventDefault(); const b = $(`[data-fid="menu:${{ t: 'vbr:audio', u: 'vbr:subs', q: 'vbr:set' }[key]}"]`); if (b) b.click(); else toast('Keine Untertitel aufgenommen.', 'warn'); break; }
    case 'm': e.preventDefault(); ACT.vbrMute(); break;
    case 'f': e.preventDefault(); ACT.vbpFs(); break;
    case '?': e.preventDefault(); ACT.vbpKeys({ k: 'rec' }); break;
    default:
  }
});

/* ======================================================================================
   LS-09 Aufnahme-Detail
   ====================================================================================== */
route('/live/recordings/:id', (pm) => {
  const a = vbRecAccess(pm.id); if (!a) return notFoundView();
  const r = a.r; const c = SIM.channel(r.ch) || { name: r.ch, no: '' }; const mode = liveMode();
  if (mode === 'loading') return { type: 'viewer', nav: 'live', title: 'Aufnahme', html: livePage({ tab: 'recordings', title: 'Aufnahme', html: skelRows(3, 120) }) };
  const dur = vbRecDur(r); const run = r.state === 'running'; const bad = vbFailed(r);
  const res = a.own ? vbResumeAt(r) : null;
  const R = (href, act, ic, label, cls, data) => (href ? `<a class="round${cls ? ' ' + cls : ''}" href="${href}" data-fid="vbd:${esc(label)}"><span class="c">${icon(ic, 20)}</span>${esc(label)}</a>` : `<button type="button" class="round${cls ? ' ' + cls : ''}" data-act="${act}" ${data || ''} data-fid="vbd:${esc(label)}"><span class="c">${icon(ic, 20)}</span>${esc(label)}</button>`);
  const base = `#/live/recordings/${esc(r.id)}/play`;
  let rounds = '';
  if (!bad) rounds += res != null ? R(base + '?from=resume', '', 'play', `Weiter ab ${vbHms(res)}`, 'hi') + R(base + '?from=start', '', 'refresh', 'Von vorne') : R(base + '?from=start', '', 'play', 'Abspielen', 'hi');
  if (a.own && !bad) rounds += R('', 'vbKeep', 'clock', 'Aufbewahrung', '', `data-id="${esc(r.id)}"`);
  if (run && a.own) rounds += R('', 'vbRecStop', 'x', 'Aufnahme beenden', '', `data-id="${esc(r.id)}"`);
  rounds += R('', a.own ? 'vbDelRec' : 'vbKidDel', 'trash', bad ? 'Entfernen' : 'Löschen', '', `data-id="${esc(r.id)}"`);
  const m = vbRecModel(r);
  let stateHtml;
  if (run) stateHtml = `<p><b>${LU.badge('recording', { text: true })}</b> Nimmt auf seit ${fmt.time(r.start)} · bis ${fmt.time(r.end)} · ${fmt.gb(vbRecGB(r))} bisher</p>${LU.progress(r.start, r.end, { cls: 'rec', label: 'Fortschritt der Aufnahme' })}`;
  else if (bad) stateHtml = notice('bad', r.state === 'rejected' ? 'Nicht aufgenommen' : 'Fehlgeschlagen', esc(LU.REASON[r.reason] || 'Unbekannter Grund.') + (r.reason === 'quota' ? ' Du und das Hauptprofil habt eine Meldung bekommen.' : ''), `<button type="button" class="btn secondary sm" data-act="vbDelRec" data-id="${esc(r.id)}" data-fid="vbd:rm">${icon('trash', 16)}Entfernen</button>`);
  else if (r.state === 'incomplete') stateHtml = notice('warn', esc(vbGapText(r)), 'Die Lücken sind in der Zeitleiste schraffiert. Abspielen geht trotzdem.');
  else stateHtml = `<p>${icon('check', 18)} Vollständig aufgenommen.</p>`;
  if (!bad) stateHtml += `<div class="vbd-bar">${vbRecTrack(r, m, null, ' aria-hidden="true"')}<div class="vbs-sub"><span>${fmt.time(r.start)}</span>${m.gaps.length ? `<span>${m.gaps.length === 1 ? '1 Lücke' : m.gaps.length + ' Lücken'} schraffiert</span>` : ''}<span>${run ? 'jetzt' : fmt.time(r.end)}</span></div><p class="muted vbd-leg"><span class="vbd-sw pad"></span>Vorlauf und Nachlauf <span class="vbd-sw gap lv-gap"></span>Lücke</p></div>`;
  if (r.fallbackAt) stateHtml += notice('', `Um ${fmt.time(r.fallbackAt)} auf eine Ersatzquelle gewechselt.`, '');
  if (!bad) stateHtml += `<p class="sec">${esc(r.tracks || '')}</p><p class="muted">Alle empfangenen Spuren bleiben erhalten. Aufgenommen wird immer in Originalqualität.</p>`;
  const rule = r.rule ? SIM.rule(r.rule) : null;
  const origin = r.origin === 'series' ? `Aus der Serienaufnahme «${esc(rule ? rule.title : r.title)}» · <a class="lnk" href="#/live/scheduled?tab=series" data-fid="vbd:rule">Serienaufnahmen</a>` : r.origin === 'instant' ? 'Sofortaufnahme im Player' : 'Einzeln geplant';
  const facts = `<div class="facts"><div class="f">${icon('tv', 18)}<span>${c.no} ${esc(c.name)}</span></div><div class="f">${icon('calendar', 18)}<span>${fmt.dayLong(r.start)}, ${fmt.range(r.start, r.end)}</span></div><div class="f">${icon('clock', 18)}<span>${fmt.dur(dur)}${r.padBefore || r.padAfter ? `, davon Vorlauf ${r.padBefore || 0} Min. und Nachlauf ${r.padAfter || 0} Min.` : ''}</span></div>${!bad ? `<div class="f">${icon('database', 18)}<span>${fmt.gb(vbRecGB(r))}${run ? ' bisher' : ''}</span></div><div class="f">${icon('bookmark', 18)}<span>${esc(vbKeepText(r))}${a.own ? ` · <button type="button" class="lnk" data-act="vbKeep" data-id="${esc(r.id)}" data-fid="vbd:keep2">Ändern</button>` : ''}</span></div>` : ''}<div class="f">${icon('layers', 18)}<span>${origin}</span></div></div>`;
  const pic = vbRecPic(r);
  const html = `${a.own ? '' : notice('', `Aufnahme von ${esc(a.prof.name)}`, `Du siehst sie als Hauptprofil. ${esc(a.prof.name)} sieht deine Aufnahmen nicht. Spielst du sie ab, wird kein Fortschritt gespeichert, weder bei ${esc(a.prof.name)} noch bei dir.`)}
    <div class="vbd"><div class="vbd-art">${LU.picture(pic, { fallback: 'logo' })}</div><div class="vbd-info"><p class="sec">${r.sub ? `«${esc(r.sub)}» · ` : ''}${esc(c.name)} · ${fmt.day(r.start)} · ${fmt.dur(dur)}</p><div class="bdg">${LU.badge(run ? 'recording' : bad ? 'failed' : r.state === 'incomplete' ? 'incomplete' : 'scheduled', { text: true, label: vbStateWord(r) })}${!a.own ? '' : r.watched ? '<span class="chip ok">Gesehen</span>' : r.watchedPct ? `<span class="chip info">${r.watchedPct} % gesehen</span>` : ''}</div><div class="rounds" role="toolbar" aria-label="Aktionen">${rounds}</div></div></div>
    ${mode === 'error' ? liveError('Die Aufnahme konnte nicht geladen werden.') : ''}
    <section class="panel"><h2>Zustand</h2>${stateHtml}</section><section class="panel"><h2>Angaben</h2>${facts}</section>`;
  return { type: 'viewer', nav: 'live', title: r.title, html: livePage({ tab: 'recordings', title: esc(r.title), actions: `<a class="btn ghost" href="${a.own ? '#/live/recordings' : '#/live/recordings/profiles'}" data-fid="vbd:back">${icon('chevL', 18)}${a.own ? 'Meine Aufnahmen' : 'Aufnahmen meiner Profile'}</a>`, html }) };
});
ACT.vbKeep = (d) => LiveDlg.retention(d.id);
/* rounds are one toolbar: arrows move inside */
document.addEventListener('keydown', (e) => {
  const tb = e.target.closest && e.target.closest('.rounds[role="toolbar"]'); if (!tb || (e.key !== 'ArrowRight' && e.key !== 'ArrowLeft')) return;
  const bs = $$('.round', tb); const i = bs.indexOf(e.target.closest('.round')); if (i < 0) return; e.preventDefault(); bs[(i + (e.key === 'ArrowRight' ? 1 : -1) + bs.length) % bs.length].focus();
});

/* ======================================================================================
   LS-11 Geplante Aufnahmen and LS-12 Konflikte und Warnungen
   ====================================================================================== */
function vbOccState(u, now) {
  if (u.start <= now && u.end > now) return ['ok', 'Nimmt auf'];
  if (SIM.chState(u.ch) === 'gone') return ['bad', 'Warnung: Sender weg'];
  if (VB.quotaFull) return ['bad', 'Wird nicht aufgenommen: Kontingent'];
  const ws = SIM.warningsFor(u.rule.id).filter((w) => w.kind !== 'timeChanged' && w.kind !== 'duplicate');
  if (ws.length) return [ws[0].severity === 'bad' ? 'bad' : 'warn', vbRuleWarnText[ws[0].kind] || 'Warnung'];
  if (u.start - now <= 10) return ['blue', `Startet in ${Math.max(1, Math.ceil(u.start - now))} Min.`];
  return ['ok', 'Geplant'];
}
function vbRuleProgram(rule) {
  if (rule.program) return rule.program;
  const u = vbUpcoming().find((x) => x.rule.id === rule.id && x.program); if (u) return u.program;
  if (rule.next) return rule.next;
  const p = LIVE.programs.find((x) => x.series === rule.series); return p ? p.id : null;
}
function vbEditOcc(u) {
  const r = u.rule;
  if (r.kind === 'single' && r.program) return { act: 'vbPlan', data: `data-pid="${esc(r.program)}"` };
  if (r.kind === 'series') return { act: 'vbEditSeries', data: `data-rid="${esc(r.id)}" data-pid="${esc(u.program || '')}"` };
  return { act: 'vbEditManual', data: `data-rid="${esc(r.id)}"` };
}
ACT.vbEditSeries = (d) => LiveDlg.seriesRecording(d.pid || vbRuleProgram(SIM.rule(d.rid)), d.rid);
ACT.vbEditManual = (d) => LiveDlg.manualRecording({ ruleId: d.rid });
ACT.vbCancelOcc = (d) => LiveDlg.confirmDelete({ kind: 'occurrence', ids: [d.id] });
ACT.vbCancelRule = (d) => LiveDlg.confirmDelete({ kind: d.kind || 'rule', ids: [d.id] });
ACT.vbRowOpen = (d) => { ui.open[d.id] = !ui.open[d.id]; render(); };
function vbSchedTabs(tab) {
  const t = [['upcoming', 'Termine', vbUpcoming().length], ['series', 'Serien', SIM.rules().filter((r) => r.kind === 'series').length], ['recurring', 'Wiederkehrend', SIM.rules().filter((r) => r.kind === 'recurring' || r.kind === 'manual').length]];
  return `<nav class="vb-tabs" aria-label="Geplante Aufnahmen"><ul>${t.map(([k, l, n]) => `<li><a href="#/live/scheduled${k === 'upcoming' ? '' : '?tab=' + k}" ${k === tab ? 'aria-current="page"' : ''} data-fid="vbst:${k}">${l} <span class="n">${n}</span></a></li>`).join('')}</ul></nav>`;
}
function vbPadLine(r) { return r.padBefore || r.padAfter ? `Vorlauf ${r.padBefore || 0} · Nachlauf ${r.padAfter || 0} Min.` : 'ohne Vor- und Nachlauf'; }
function vbOccRow(u, ids, now) {
  const c = SIM.channel(u.ch) || { name: u.ch, no: '' }; const r = u.rule; const [sk, st] = vbOccState(u, now);
  const open = !!ui.open['occ:' + u.id]; const ed = vbEditOcc(u); const est = vbEstGB(u.ch, u.end - u.start + (r.padBefore || 0) + (r.padAfter || 0));
  const tc = SIM.warningsFor(r.id).find((w) => w.kind === 'timeChanged');
  return `<li class="vbt-row${open ? ' open' : ''}" data-vbrow="${esc(u.id)}"><div class="vbt-line">
    <button type="button" class="vbt-main" data-vbi tabindex="${vbTab('vbt', u.id, ids)}" data-act="vbRowOpen" data-id="occ:${esc(u.id)}" aria-expanded="${open}" data-fid="vbt:${esc(u.id)}" aria-label="${fmt.range(u.start, u.end)}, ${esc(c.name)}, ${esc(u.title)}, ${esc(LU.RULE_KIND[u.kind])}, ${esc(st)}. Enter klappt auf">
      <span class="tm"><b>${fmt.range(u.start, u.end)}</b><span class="muted">${esc(vbPadLine(r))}</span></span>
      <span class="ch">${LU.logo(c)}<span>${esc(c.name)}</span></span>
      <span class="ti"><b>${esc(u.title)}</b>${u.sub ? `<span class="sec">«${esc(u.sub)}»</span>` : ''}</span>
      <span class="kd"><span class="chip info">${esc(LU.RULE_KIND[u.kind])}</span></span>
      <span class="sx"><span class="dot ${sk}"></span>${esc(st)}</span></button>
    <span class="vbt-acts"><button type="button" class="btn secondary sm" data-act="${ed.act}" ${ed.data} data-vbx tabindex="-1" data-fid="vbte:${esc(u.id)}">${icon('pen', 16)}Ändern</button><button type="button" class="btn ghost sm" data-act="vbCancelOcc" data-id="${esc(u.id)}" data-vbx tabindex="-1" data-fid="vbtc:${esc(u.id)}">${icon('x', 16)}Abbrechen</button></span></div>
    ${open ? `<dl class="vbt-more"><dt>Aufbewahrung</dt><dd>${esc(vbKeepText({ keep: r.keep, start: u.start, rule: r.kind === 'series' ? r.id : null }))}</dd><dt>Voraussichtliche Grösse</dt><dd>${est != null ? 'etwa ' + fmt.gb(est) : 'Grösse unbekannt'}</dd><dt>Herkunft</dt><dd>${esc(LU.RULE_KIND[r.kind])}${r.kind === 'series' ? ` «${esc(r.title)}»` : ''}${r.created != null ? `, angelegt ${fmt.day(r.created)}` : ''}</dd>${tc ? `<dt>Anpassung an das Programm</dt><dd>${esc(tc.text)}</dd>` : ''}</dl>` : ''}</li>`;
}
route('/live/scheduled', (pm, q) => {
  const tab = ['series', 'recurring'].includes(q.tab) ? q.tab : 'upcoming'; const mode = liveMode(); const now = SIM.now();
  const up = vbUpcoming();
  const nW = vaWarnCount(); /* same count and words as LS-01 and LS-12 */
  const wn = nW ? notice('warn', vaWarnText(nW), 'Sieh nach, was einer Aufnahme im Weg steht.', `<a class="btn secondary sm" href="#/live/scheduled/warnings" data-fid="vbs:warn">${icon('alert', 16)}Konflikte und Warnungen ansehen</a>`) : '';
  let body;
  if (mode === 'loading') body = skelRows(5, 64);
  else if (mode === 'error') body = liveError('Die geplanten Aufnahmen konnten nicht geladen werden.');
  else if (tab === 'upcoming') {
    if (!up.length) body = emptyState('Keine Aufnahmen geplant.', '', `<a class="btn secondary" href="#/live/guide" data-fid="vbs:guide">Zum Programm</a>`);
    else {
      const ids = up.map((u) => u.id); const days = [...new Set(up.map((u) => Math.floor(u.start / 1440)))];
      body = `<div class="vbt-head" aria-hidden="true"><span>Zeit</span><span>Sender</span><span>Sendung</span><span>Art</span><span>Zustand</span></div><div data-vbrove="list" data-vbkey="vbt">${days.map((d) => `<h2 class="ghead vbt-day">${fmt.dayRel(d * 1440)}${fmt.dayRel(d * 1440) !== fmt.day(d * 1440) ? ` · ${fmt.day(d * 1440)}` : ''}</h2><ul class="vbt">${up.filter((u) => Math.floor(u.start / 1440) === d).map((u) => vbOccRow(u, ids, now)).join('')}</ul>`).join('')}</div>`;
    }
  } else if (tab === 'series') {
    const rs = SIM.rules().filter((r) => r.kind === 'series');
    if (!rs.length) body = emptyState('Keine Serienaufnahmen.', 'Öffne eine Sendung und wähle «Serie aufnehmen».', `<a class="btn secondary" href="#/live/guide" data-fid="vbs:guide2">Zum Programm</a>`);
    else {
      const ids = rs.map((r) => r.id);
      body = `<ul class="vbt" data-vbrove="list" data-vbkey="vbts">${rs.map((r) => {
        const occ = up.filter((u) => u.rule.id === r.id); const recs = SIM.recordings().filter((x) => x.rule === r.id); const open = !!ui.open['ser:' + r.id];
        const chans = r.channels === 'all' ? 'alle Sender' : r.channels.map((id) => (SIM.channel(id) || { name: id }).name).join(', ');
        const sum = [chans, r.onlyNew ? 'nur Erstausstrahlungen' : '',r.window ? `nur ${r.window[0]}–${r.window[1]}` : '', r.weekdays && r.weekdays.length ? vbWdText(r.weekdays) : '', r.keepLast ? `behalte ${r.keepLast}` : 'alle Folgen behalten', `${occ.length === 1 ? '1 Termin' : occ.length + ' Termine'} geplant`, `${recs.length === 1 ? '1 Aufnahme' : recs.length + ' Aufnahmen'}`].filter(Boolean).join(' · ');
        const skipped = (r.skipped || []).map(([pid, why]) => { const p = SIM.program(pid); return p ? `<li>${fmt.day(p.start)} ${fmt.time(p.start)} ${esc((SIM.channel(p.ch) || {}).name || '')}: ${esc(why)}</li>` : ''; }).join('') + (r.skip || []).map((id) => { const p = SIM.program(id.split(':').slice(1).join(':')); return p ? `<li>${fmt.day(p.start)} ${fmt.time(p.start)} ${esc((SIM.channel(p.ch) || {}).name || '')}: von dir ausgelassen</li>` : ''; }).join('');
        return `<li class="vbt-row${open ? ' open' : ''}" data-vbrow="${esc(r.id)}"><div class="vbt-line"><button type="button" class="vbt-main rule" data-vbi tabindex="${vbTab('vbts', r.id, ids)}" data-act="vbRowOpen" data-id="ser:${esc(r.id)}" aria-expanded="${open}" data-fid="vbts:${esc(r.id)}">${icon('recordSeries', 20)}<span class="ti"><b>${esc(r.title)}</b><span class="sec">${esc(sum)}</span></span>${SIM.warningsFor(r.id).length ? LU.badge('warning', { text: true }) : ''}</button>
          <span class="vbt-acts"><button type="button" class="btn secondary sm" data-act="vbEditSeries" data-rid="${esc(r.id)}" data-pid="${esc(occ[0] ? occ[0].program : '')}" data-vbx tabindex="-1" data-fid="vbtse:${esc(r.id)}">${icon('pen', 16)}Ändern</button><button type="button" class="btn ghost sm" data-act="vbCancelRule" data-kind="series" data-id="${esc(r.id)}" data-vbx tabindex="-1" data-fid="vbtsx:${esc(r.id)}">${icon('x', 16)}Beenden</button></span></div>
          ${open ? `<div class="vbt-more2"><h3>Nächste Termine</h3>${occ.length ? `<ul>${occ.slice(0, 6).map((u) => `<li>${fmt.day(u.start)} ${fmt.range(u.start, u.end)} · ${esc((SIM.channel(u.ch) || {}).name || '')}${u.sub ? ` · «${esc(u.sub)}»` : ''}</li>`).join('')}</ul>` : '<p class="muted">Gerade kein Termin im Programm.</p>'}<h3>Übersprungen</h3>${skipped ? `<ul>${skipped}</ul>` : '<p class="muted">Nichts übersprungen.</p>'}</div>` : ''}</li>`;
      }).join('')}</ul>`;
    }
  } else {
    const rec = SIM.rules().filter((r) => r.kind === 'recurring'); const man = SIM.rules().filter((r) => r.kind === 'manual');
    const ids = rec.concat(man).map((r) => r.id);
    const row = (r) => { const c = SIM.channel(r.ch) || { name: r.ch }; const nx = up.find((u) => u.rule.id === r.id); const gone = SIM.chState(r.ch) === 'gone';
      const sum = r.kind === 'recurring' ? [c.name, `${vbWdText(r.weekdays)} ${r.from}–${r.to}`, r.keepLast ? `behalte ${r.keepLast}` : 'alle behalten', nx ? `nächster Termin ${fmt.day(nx.start)}` : 'kein Termin in den nächsten Tagen'].join(' · ') : [c.name, `${fmt.day(r.start)} ${fmt.range(r.start, r.end)}`].join(' · ');
      return `<li class="vbt-row" data-vbrow="${esc(r.id)}"><div class="vbt-line"><span class="vbt-main rule" data-vbi tabindex="${vbTab('vbtr', r.id, ids)}" data-fid="vbtr:${esc(r.id)}" role="group" aria-label="${esc(r.title)}: ${esc(sum)}">${LU.logo(c)}<span class="ti"><b>${esc(r.title)}</b><span class="sec">${esc(sum)}</span></span>${gone ? `<span class="chip bad">${icon('alert', 14)}Sender weg</span>` : ''}</span>
        <span class="vbt-acts"><button type="button" class="btn secondary sm" data-act="vbEditManual" data-rid="${esc(r.id)}" data-vbx tabindex="-1" data-fid="vbtre:${esc(r.id)}">${icon('pen', 16)}${gone ? 'Anderen Sender wählen' : 'Ändern'}</button><button type="button" class="btn ghost sm" data-act="vbCancelRule" data-id="${esc(r.id)}" data-vbx tabindex="-1" data-fid="vbtrx:${esc(r.id)}">${icon('x', 16)}Abbrechen</button></span></div></li>`; };
    body = !rec.length && !man.length ? emptyState('Keine wiederkehrenden Aufnahmen.', '', `<button type="button" class="btn secondary" data-act="vbNewRec" data-fid="vbs:new2">Neue Aufnahme</button>`)
      : `<div data-vbrove="list" data-vbkey="vbtr">${rec.length ? `<h2 class="ghead">Wiederkehrend</h2><ul class="vbt">${rec.map(row).join('')}</ul>` : `<p class="muted">Keine wiederkehrenden Aufnahmen.</p>`}${man.length ? `<h2 class="ghead">Manuell</h2><ul class="vbt">${man.map(row).join('')}</ul>` : ''}</div>`;
  }
  return { type: 'viewer', nav: 'live', title: 'Geplant', html: livePage({ tab: 'recordings', title: 'Geplant', count: mode === 'normal' ? `${up.length} ${up.length === 1 ? 'Termin' : 'Termine'}` : '', actions: `<a class="btn ghost" href="#/live/recordings" data-fid="vbs:back">${icon('chevL', 18)}Meine Aufnahmen</a><button type="button" class="btn primary" data-act="vbNewRec" data-fid="vbs:new">${icon('plus', 18)}Neue Aufnahme</button>`, html: (mode === 'normal' ? wn : '') + vbSchedTabs(tab) + `<div class="vbt-body">${body}</div>` }) };
});
const VB_WKIND = { quota: 'Kontingent', quotaFull: 'Kontingent voll', serverStorage: 'Speicher des Servers', overlap: 'Überschneidung', duplicate: 'Doppelt', channelGone: 'Sender weg', programGone: 'Sendung verschwunden', timeChanged: 'Zeit geändert', dst: 'Sommer-/Winterzeit' };
function vbAllWarnings() {
  const list = SIM.warnings().slice(); /* the count is vaWarnCount() (viewer-a.js): keep both rules the same */
  if (VB.quotaFull) { const up = vbUpcoming().filter((u) => SIM.chState(u.ch) !== 'gone'); if (up.length) list.unshift({ id: 'w-quotafull', kind: 'quotaFull', severity: 'bad', rules: [...new Set(up.map((u) => u.rule.id))], text: `Im Speicher des Kontos ist kein Platz frei (${fmt.gb(vbQuota().usedGB)} von ${fmt.gb(vbQuota().totalGB)}). Ist das beim Start noch so, lehnt der Server die Aufnahme ab, und du und das Hauptprofil bekommt eine Meldung.` }); }
  return list;
}
function vbWarnActions(w, inDlg) {
  const b = (cls, act, label, data, ic) => `<button type="button" class="btn ${cls} sm" data-act="${act}" ${data || ''} data-fid="vbwa:${w.id}:${act}">${ic ? icon(ic, 16) : ''}${label}</button>`;
  const go2 = (href, label, ic) => `<a class="btn secondary sm" href="${href}" ${inDlg ? 'data-act="close"' : ''} data-fid="vbwa:${w.id}:${label}">${ic ? icon(ic, 16) : ''}${label}</a>`;
  if (w.kind === 'quota' || w.kind === 'quotaFull') return go2('#/live/recordings?sort=size', 'Aufnahmen aufräumen', 'trash') + (w.kind === 'quota' ? b('ghost', 'vbWarnKeep', ui.vbAck && ui.vbAck[w.id] ? 'Behalten (gewählt)' : 'Trotzdem behalten', `data-id="${w.id}"`) : '');
  if (w.kind === 'overlap') return popMenu('vbw:' + w.id + (inDlg ? ':d' : ''), `${icon('x', 16)}Eine abbrechen …`, w.rules.map((id) => SIM.rule(id)).filter(Boolean).map((r) => ({ label: `${r.title} nicht aufnehmen`, act: 'vbCancelRule', data: { id: r.id, kind: 'rule' }, icon: 'x' })), { cls: 'secondary sm', aria: 'Eine der Aufnahmen abbrechen' });
  if (w.kind === 'duplicate') return b('secondary', 'vbCancelRule', 'Einzelaufnahme entfernen', `data-id="${w.rules[0]}"`);
  if (w.kind === 'channelGone') return b('secondary', 'vbEditManual', 'Anderen Sender wählen', `data-rid="${w.rules[0]}"`, 'pen') + b('ghost', 'vbCancelRule', 'Abbrechen', `data-id="${w.rules[0]}"`, 'x');
  if (w.kind === 'programGone') return b('secondary', 'vbEditManual', 'Zeit trotzdem aufnehmen', `data-rid="${w.rules[0]}"`) + b('ghost', 'vbCancelRule', 'Entfernen', `data-id="${w.rules[0]}"`);
  if (w.kind === 'timeChanged' || w.kind === 'dst') return b('secondary', 'vbWarnOk', 'OK', `data-id="${w.id}"`, 'check');
  return '';
}
ACT.vbWarnKeep = (d) => { ui.vbAck = ui.vbAck || {}; ui.vbAck[d.id] = true; toast('Du behältst die Aufnahme. Die Warnung bleibt sichtbar.'); render(); };
ACT.vbWarnOk = (d) => { SIM.dismissWarning(d.id); if (ui.dialog) closeDialog(); toast('Hinweis ausgeblendet.'); };
function vbWarnCard(w) {
  const up = vbUpcoming(); const occ = w.rules.map((id) => { const r = SIM.rule(id); const u = up.find((x) => x.rule.id === id); return r ? `<li>${esc(r.title)}${u ? ` · ${fmt.when(u.start)} · ${esc((SIM.channel(u.ch) || { name: u.ch }).name)}` : ''}</li>` : ''; }).join('');
  const sev = w.severity === 'bad' ? 'bad' : w.severity === 'warn' ? 'warn' : 'info';
  return `<article class="vbw ${sev}" aria-labelledby="vbw-${w.id}"><header><span class="ic">${icon(sev === 'info' ? 'info' : 'alert', 20)}</span><h2 id="vbw-${w.id}">${esc(VB_WKIND[w.kind] || w.kind)}</h2><span class="chip ${sev === 'bad' ? 'bad' : sev === 'warn' ? 'warn' : 'info'}">${sev === 'bad' ? 'Wichtig' : sev === 'warn' ? 'Warnung' : 'Hinweis'}</span></header><p>${esc(w.text)}</p>${occ ? `<ul class="vbw-occ" aria-label="Betroffene Termine">${occ}</ul>` : ''}${ui.vbAck && ui.vbAck[w.id] ? '<p class="muted">Du hast entschieden, die Aufnahme zu behalten.</p>' : ''}<div class="row-gap">${vbWarnActions(w, false)}<button type="button" class="btn ghost sm" data-act="vbWarnDlg" data-id="${w.id}" data-fid="vbwd:${w.id}">${icon('info', 16)}Details</button></div></article>`;
}
ACT.vbWarnDlg = (d) => LiveDlg.warning(d.id);
route('/live/scheduled/warnings', () => {
  const mode = liveMode(); const list = vbAllWarnings();
  let body;
  if (mode === 'loading') body = skelRows(3, 120);
  else if (mode === 'error') body = liveError('Die Warnungen konnten nicht geladen werden.');
  else if (!list.length) body = emptyState('Alles in Ordnung.', 'Keine geplante Aufnahme hat eine Warnung.', `<a class="btn secondary" href="#/live/scheduled" data-fid="vbw:back2">Zu den geplanten Aufnahmen</a>`);
  else body = `<p class="muted">Slouchcat kennt keine Empfänger und kein Anbieterlimit. Darum gibt es keinen Kampf um einen Tuner, nur Warnungen mit Folgen.</p><div class="vbw-list">${list.map(vbWarnCard).join('')}</div>`;
  return { type: 'viewer', nav: 'live', title: 'Konflikte und Warnungen', html: livePage({ tab: 'recordings', title: 'Konflikte und Warnungen', count: list.length && mode === 'normal' ? vaWarnText(list.length) : '', actions: `<a class="btn ghost" href="#/live/scheduled" data-fid="vbw:back">${icon('chevL', 18)}Geplant</a>`, html: body }) };
});

/* ======================================================================================
   LS-13 Erinnerungen
   ====================================================================================== */
const vbLeadText = (l) => (l ? `Erinnert ${l} Min. vorher` : 'Erinnert zum Beginn');
route('/live/reminders', () => {
  const mode = liveMode(); const list = SIM.reminders();
  let body;
  if (mode === 'loading') body = skelRows(3, 64);
  else if (mode === 'error') body = liveError('Die Erinnerungen konnten nicht geladen werden.');
  else if (!list.length) body = emptyState('Keine Erinnerungen.', 'Öffne eine Sendung und wähle «Erinnern».', `<a class="btn secondary" href="#/live/guide" data-fid="vbrm:guide">Zum Programm</a>`);
  else {
    const ids = list.map((r) => r.program); const days = [...new Set(list.map((r) => Math.floor(SIM.program(r.program).start / 1440)))];
    body = `<div data-vbrove="list" data-vbkey="vbrm">${days.map((d) => `<h2 class="ghead">${fmt.dayRel(d * 1440)}${fmt.dayRel(d * 1440) !== fmt.day(d * 1440) ? ` · ${fmt.day(d * 1440)}` : ''}</h2><ul class="vbt">${list.filter((r) => Math.floor(SIM.program(r.program).start / 1440) === d).map((r) => {
      const p = SIM.program(r.program); const c = SIM.channel(p.ch);
      return `<li class="vbt-row" data-vbrow="${esc(p.id)}"><div class="vbt-line"><a class="vbt-main rem" href="#/live/programs/${esc(p.id)}" data-vbi tabindex="${vbTab('vbrm', p.id, ids)}" data-fid="vbrm:${esc(p.id)}" aria-label="${fmt.time(p.start)}, ${esc(c.name)}, ${esc(p.title)}, ${esc(vbLeadText(r.lead))}"><span class="tm"><b>${fmt.time(p.start)}</b></span><span class="ch">${LU.logo(c)}<span>${esc(c.name)}</span></span><span class="ti"><b>${esc(p.title)}</b>${p.sub ? `<span class="sec">«${esc(p.sub)}»</span>` : ''}</span><span class="sx">${LU.badge('reminder', { text: true, label: vbLeadText(r.lead) })}</span></a>
        <span class="vbt-acts"><button type="button" class="btn secondary sm" data-act="vbRemind" data-pid="${esc(p.id)}" data-vbx tabindex="-1" data-fid="vbrme:${esc(p.id)}">${icon('pen', 16)}Ändern</button><button type="button" class="btn ghost sm" data-act="vbRemOff" data-pid="${esc(p.id)}" data-vbx tabindex="-1" data-fid="vbrmx:${esc(p.id)}">${icon('x', 16)}Entfernen</button></span></div></li>`; }).join('')}</ul>`).join('')}</div>
      <p class="muted">Zur Zeit der Erinnerung erscheint oben ein Hinweis mit «Einschalten», dazu ein Eintrag in der Glocke. Vergangene Erinnerungen verschwinden nach Sendungsbeginn.</p>`;
  }
  return { type: 'viewer', nav: 'live', title: 'Erinnerungen', html: livePage({ tab: 'reminders', title: 'Erinnerungen', count: list.length ? `${list.length}` : '', html: body }) };
});

/* ======================================================================================
   Dialogs LD-01 to LD-11 (14 5): create and change in the same dialog; focus on the first field
   ====================================================================================== */
const VB_PB = [0, 1, 2, 5, 10, 15, 30], VB_PA = [0, 5, 10, 15, 30, 60], VB_KEEP = [7, 14, 30, 90], VB_LEADS = [0, 5, 15, 30];
const vbDP = (k) => `ui.dialog.props._d.${k}`;
function vbSel(k, label, opts, cur, o) { o = o || {}; return `<div class="fld"><label for="vbf-${k}">${label}</label><select class="sel" id="vbf-${k}" data-bind="${vbDP(k)}" data-rerender data-fid="vbf:${k}"${o.auto ? ' data-autofocus' : ''}${o.dis ? ' disabled' : ''}>${opts.map(([v, t]) => `<option value="${esc(v)}"${String(v) === String(cur) ? ' selected' : ''}>${esc(t)}</option>`).join('')}</select>${o.hint ? `<span class="hint">${o.hint}</span>` : ''}</div>`; }
function vbRadios(k, label, opts, cur) { return `<fieldset class="vb-fs"><legend>${label}</legend>${opts.map(([v, t, dis]) => `<label class="check"><input type="radio" name="vbr-${k}" value="${esc(v)}" data-bind="${vbDP(k)}" data-fid="vbf:${k}:${esc(v)}"${String(v) === String(cur) ? ' checked' : ''}${dis ? ' disabled' : ''}><span>${t}</span></label>`).join('')}</fieldset>`; }
function vbChk(k, label, on, o) { o = o || {}; return `<label class="check"><input type="checkbox" data-bind="${vbDP(k)}" data-fid="vbf:${k}"${on ? ' checked' : ''}${o.dis ? ' disabled' : ''}><span>${label}${o.hint ? `<small>${o.hint}</small>` : ''}</span></label>`; }
const vbPadOpts = (list, std) => [['std', `Standard (${std} Min.)`], ...list.map((n) => [String(n), `${n} Min.`])];
/* «bis du löschst» only once: when it is the standard, the standard entry stands for it (a stored 'forever' then selects the first entry) */
const vbKeepOpts = (std) => [['std', `Standard (${std})`], ...VB_KEEP.map((n) => [String(n), `${n} Tage`]), ...(LIVE.settings.dvr.keep === 'forever' ? [] : [['forever', 'bis du löschst']])];
const vbStdKeepText = () => (LIVE.settings.dvr.keep === 'forever' ? 'bis du löschst' : LIVE.settings.dvr.keep + ' Tage');
const vbPadVal = (v, std) => (v === 'std' || v == null ? std : Number(v));
function vbDlgHead(p) { const c = SIM.channel(p.ch) || { name: p.ch }; return `<div class="vb-dh">${LU.logo(c)}<div><b>${esc(p.title)}${p.sub ? ` · «${esc(p.sub)}»` : ''}</b><span class="muted">${esc(c.name)} · ${fmt.when(p.start)}–${fmt.time(p.end)}</span></div></div>`; }
const vbDlgGone = () => ({ title: 'Nicht mehr im Programm', body: `<p class="lead">Diese Sendung steht nicht mehr im Programm.</p>`, actions: `<button type="button" class="btn secondary" data-act="close" data-fid="vbdg:x" data-autofocus>Schliessen</button>` });
const vbNoRight = () => ({ title: 'Nicht möglich', body: `<p class="lead">Aufnahmen sind für dieses Profil nicht freigegeben.</p>`, actions: `<button type="button" class="btn secondary" data-act="close" data-fid="vbdn:x" data-autofocus>Schliessen</button>` });
function vbOverlaps(chId, a, b, skipRule) { return vbUpcoming().filter((u) => (!skipRule || u.rule.id !== skipRule) && u.start < b && u.end > a && !(u.ch === chId && Math.abs(u.start - a) < 30)); }
function vbQuotaNotes(est) {
  const q = vbQuota();
  if (VB.quotaFull || q.freeGB <= 0) return notice('bad', 'Im Speicher deines Kontos ist kein Platz frei', `${fmt.gb(q.usedGB)} von ${fmt.gb(q.totalGB)} belegt. Ist das beim Start noch so, lehnt der Server die Aufnahme ab. Du und das Hauptprofil bekommt eine Meldung.`, `<a class="btn secondary sm" href="#/live/recordings?sort=size" data-act="close" data-fid="vbq:clean">Aufnahmen aufräumen</a>`);
  if (est != null && est > q.freeGB) return notice('warn', esc('Für diese Aufnahme fehlt voraussichtlich Platz im Speicher des Kontos.'), `Frei ${fmt.gb(q.freeGB)}, nötig etwa ${fmt.gb(est)}.`);
  return '';
}

/* ---------- LD-01 Aufnahme planen / ändern ---------- */
registerLiveDialog('LD-01', (pr) => {
  if (!SIM.rights().recordings) return vbNoRight();
  const p = SIM.program(pr.programId); if (!p) return vbDlgGone();
  const now = SIM.now(); const dvr = LIVE.settings.dvr;
  const ex = SIM.rules().find((x) => x.kind === 'single' && x.program === p.id);
  const run = p.start <= now && p.end > now; const past = p.end <= now;
  if (SIM.recState(p.id) === 'recording') {
    const rec = SIM.runningOn(p.ch);
    return { title: 'Nimmt auf', body: vbDlgHead(p) + `<p class="lead">Diese Sendung wird gerade aufgenommen${rec ? `, seit ${fmt.time(rec.start)} bis ${fmt.time(rec.end)}` : ''}.</p>`,
      actions: `${rec ? `<button type="button" class="btn danger" data-act="vbLd01Stop" data-id="${esc(rec.id)}" data-fid="ld01:stop">Aufnahme beenden</button><a class="btn secondary" href="#/live/recordings/${esc(rec.id)}" data-act="close" data-fid="ld01:go">Zur Aufnahme</a>` : ''}<button type="button" class="btn primary" data-act="close" data-fid="ld01:x" data-autofocus>Schliessen</button>` };
  }
  if (!pr._d) pr._d = { pb: ex ? String(ex.padBefore) : 'std', pa: ex ? String(ex.padAfter) : 'std', keep: ex ? vbKeepKey(ex.keep) : 'std' };
  const d = pr._d; const pb = vbPadVal(d.pb, dvr.padBefore), pa = vbPadVal(d.pa, dvr.padAfter);
  const a = run ? now : p.start - pb, b = p.end + pa;
  const est = vbEstGB(p.ch, b - a);
  const warns = ex ? SIM.warningsFor(ex.id).filter((w) => w.kind !== 'duplicate') : [];
  const ovl = vbOverlaps(p.ch, a, b, ex && ex.id).filter((u) => u.program !== p.id);
  const fields = run ? vbSel('pa', 'Nachlauf', vbPadOpts(VB_PA, dvr.padAfter), d.pa, { auto: true }) + vbSel('keep', 'Aufbewahren', vbKeepOpts(vbStdKeepText()), d.keep)
    : vbSel('pb', 'Vorlauf', vbPadOpts(VB_PB, dvr.padBefore), d.pb, { auto: true, dis: past }) + vbSel('pa', 'Nachlauf', vbPadOpts(VB_PA, dvr.padAfter), d.pa, { dis: past }) + vbSel('keep', 'Aufbewahren', vbKeepOpts(vbStdKeepText()), d.keep, { dis: past });
  const instantBlocked = run && (VB.quotaFull || vbQuota().freeGB <= 0);
  const body = vbDlgHead(p)
    + (past ? `<p class="err" role="alert">Diese Sendung ist schon vorbei.</p>` : '')
    + (run ? notice('', `Die Sendung läuft seit ${fmt.time(p.start)}. Die Aufnahme beginnt jetzt.`, 'Eine Aufnahme ab Sendungsbeginn gibt es nicht.') : '')
    + `<div class="vb-grid3">${fields}</div>`
    + (past ? '' : `<p class="vb-sum"><b>Aufnahme von ${fmt.time(a)} bis ${fmt.time(b)}</b> · ${est != null ? 'etwa ' + fmt.gb(est) : 'Grösse unbekannt'}</p>`)
    + (past ? '' : vbQuotaNotes(est))
    + warns.map((w) => notice(w.severity === 'bad' ? 'bad' : w.severity === 'warn' ? 'warn' : '', esc(VB_WKIND[w.kind] || 'Hinweis'), esc(w.text), `<button type="button" class="btn ghost sm" data-act="vbWarnDlgFrom" data-id="${w.id}" data-fid="ld01:w:${w.id}">Mehr</button>`)).join('')
    + ovl.map((u) => notice('', esc(`Überschneidet sich mit ‹${u.title}›. Beide werden aufgenommen.`), '')).join('')
    + `<p class="muted vb-small">Ändert sich die Sendezeit, passt Slouchcat die Aufnahme an.</p>`
    + (past ? '' : `<button type="button" class="lnk vb-alt" data-act="vbToSeries" data-pid="${esc(p.id)}" data-fid="ld01:series">${icon('recordSeries', 16)}${p.series ? 'Stattdessen die Serie aufnehmen' : 'Stattdessen jede Ausstrahlung dieses Titels aufnehmen'}</button>`);
  const primary = past ? `<button type="button" class="btn primary" aria-disabled="true" data-fid="ld01:ok">Aufnahme planen</button>` : instantBlocked ? `<button type="button" class="btn primary" aria-disabled="true" data-fid="ld01:ok">Ab jetzt aufnehmen</button>` : `<button type="button" class="btn primary" data-act="vbLd01Save" data-fid="ld01:ok">${run ? 'Ab jetzt aufnehmen' : ex ? 'Speichern' : 'Aufnahme planen'}</button>`;
  return { title: ex ? 'Aufnahme ändern' : 'Aufnahme planen', body, actions: `${ex ? `<button type="button" class="btn danger vb-left" data-act="vbLd01Cancel" data-id="${esc(ex.id)}" data-fid="ld01:cancel">Aufnahme abbrechen</button>` : ''}<button type="button" class="btn secondary" data-act="close" data-fid="ld01:x">Abbrechen</button>${primary}` };
});
ACT.vbLd01Save = () => {
  const pr = ui.dialog.props; const d = pr._d; const p = SIM.program(pr.programId); const dvr = LIVE.settings.dvr; const now = SIM.now();
  const pb = vbPadVal(d.pb, dvr.padBefore), pa = vbPadVal(d.pa, dvr.padAfter); const keep = vbKeepFrom(d.keep, p.start);
  if (p.start <= now) {
    const res = SIM.startInstant(p.ch, { until: 'end', keep }); closeDialog();
    if (!res.ok) { toast('Im Speicher deines Kontos ist kein Platz frei.', 'bad'); return; }
    const r = SIM.recording(res.id); r.end = p.end + pa; r.padAfter = pa; SIM.emit('change');
    toast(`Aufnahme läuft bis ${fmt.time(r.end)}. <a href="#/live/recordings">Meine Aufnahmen</a>`); return;
  }
  const had = SIM.rules().some((x) => x.kind === 'single' && x.program === p.id);
  SIM.planRecording(p.id, { padBefore: pb, padAfter: pa, keep }); closeDialog();
  toast(`${had ? 'Gespeichert' : 'Aufnahme geplant'}: «${esc(p.title)}», ${fmt.when(p.start)}. <a href="#/live/scheduled">Geplant</a>`);
};
ACT.vbLd01Cancel = (d) => { closeDialog(); setTimeout(() => LiveDlg.confirmDelete({ kind: 'rule', ids: [d.id] }), 0); };
ACT.vbLd01Stop = (d) => { closeDialog(); setTimeout(() => LiveDlg.confirmDelete({ kind: 'running', ids: [d.id] }), 0); };
ACT.vbToSeries = (d) => { closeDialog(); setTimeout(() => LiveDlg.seriesRecording(d.pid), 0); };
ACT.vbWarnDlgFrom = (d) => { closeDialog(); setTimeout(() => LiveDlg.warning(d.id), 0); };

/* ---------- LD-02 Serienaufnahme ---------- */
function vbSeriesProgs(p, match) { const key = p.series || p.title; return LIVE.programs.filter((x) => (match === 'series' && p.series ? x.series === key : x.title === p.title)); }
registerLiveDialog('LD-02', (pr) => {
  if (!SIM.rights().recordings) return vbNoRight();
  const rule = pr.ruleId ? SIM.rule(pr.ruleId) : null;
  const pid = pr.programId || (rule ? vbRuleProgram(rule) : null);
  const p = pid ? SIM.program(pid) : null; if (!p) return vbDlgGone();
  if (!rule) { const ex = SIM.rules().find((r) => r.kind === 'series' && r.series === (p.series || p.title)); if (ex) { pr.ruleId = ex.id; return LIVE_DIALOGS['LD-02'](pr); } }
  const dvr = LIVE.settings.dvr; const now = SIM.now();
  if (!pr._d) pr._d = rule
    ? { match: rule.match || 'series', chans: rule.channels === 'all' ? 'all' : 'one', ch: rule.channels === 'all' ? p.ch : rule.channels[0], win: rule.window ? 'win' : 'any', from: rule.window ? rule.window[0] : '20:00', to: rule.window ? rule.window[1] : '22:30', wd: (rule.weekdays || []).slice(), onlyNew: !!rule.onlyNew, noDup: !!rule.noDup, keepLast: rule.keepLast ? String(rule.keepLast) : 'all', pb: String(rule.padBefore), pa: String(rule.padAfter), keep: vbKeepKey(rule.keep) }
    : { match: p.series ? 'series' : 'title', chans: 'one', ch: p.ch, win: 'any', from: '20:00', to: '22:30', wd: [], onlyNew: false, noDup: true, keepLast: 'all', pb: 'std', pa: 'std', keep: 'std' };
  const d = pr._d;
  const progs = vbSeriesProgs(p, d.match);
  const chIds = [...new Set(progs.map((x) => x.ch))]; const chNames = chIds.map((id) => (SIM.channel(id) || { name: id }).name);
  const knowsRepeat = progs.some((x) => x.repeat || x.firstRun); const knowsEp = progs.some((x) => x.sub);
  const name = p.seriesName || (rule && rule.title) || p.title;
  const fut = progs.filter((x) => x.start >= now && (d.chans === 'all' || x.ch === d.ch) && (!d.onlyNew || !knowsRepeat || !x.repeat) && (!d.wd.length || d.wd.includes(vbWd(x.start))) && (d.win !== 'win' || (() => { const m = x.start % 1440; return m >= LIVE.hm(d.from) && m <= LIVE.hm(d.to); })())).sort((a, b) => a.start - b.start);
  const seen = new Set(); const prev = fut.slice(0, 6).map((x) => { const dup = knowsEp && x.sub && seen.has(x.sub); if (x.sub) seen.add(x.sub); const done = knowsEp && x.sub && SIM.recordings().some((r) => r.sub === x.sub && r.title === x.title); return `<li${dup || done ? ' class="skip"' : ''}>${fmt.day(x.start)} ${fmt.time(x.start)} ${esc((SIM.channel(x.ch) || { name: x.ch }).name)}${x.sub ? ` · «${esc(x.sub)}»` : ''}${x.repeat ? ' · Wiederholung' : ''}${dup ? ' (gleiche Folge, wird übersprungen)' : done ? ' (schon aufgenommen, wird übersprungen)' : ''}</li>`; }).join('');
  const wdp = `<div class="vb-wd" role="group" aria-label="Wochentage (ohne Auswahl: alle)">${[1, 2, 3, 4, 5, 6, 0].map((n) => `<button type="button" class="pill vb-pill" aria-pressed="${d.wd.includes(n)}" data-act="vbWd" data-n="${n}" data-fid="vbwd:${n}">${vbWdNames[n]}</button>`).join('')}</div>`;
  const body = `<div class="vb-dh">${LU.logo(SIM.channel(p.ch))}<div><b>${esc(name)}</b><span class="muted">Ausgehend von ${esc(p.title)}${p.sub ? ` · «${esc(p.sub)}»` : ''}, ${fmt.when(p.start)}</span></div></div>
    <p class="vb-small">Slouchcat nimmt jede Sendung mit dem Titel ‹${esc(name)}› auf.</p>
    ${vbRadios('chans', 'Sender', [['one', `nur der Sender dieser Sendung (${esc((SIM.channel(d.ch) || { name: d.ch }).name)})`], ['all', `alle Sender, die du sehen kannst (gefunden: ${esc(chNames.join(', '))})`]], d.chans).replace(/(<input[^>]*checked)/, '$1 data-autofocus')}
    <div class="fld"><span class="lbl">Folgen</span><p class="vb-small">${knowsEp ? 'Folgen werden erkannt (Staffel und Folge). Schon aufgenommene Folgen werden übersprungen.' : 'Folgen nicht identifizierbar: Wiederholungen werden mit aufgenommen.'}</p></div>
    <div class="vb-grid3">${vbSel('keepLast', 'Behalten', [['all', 'alle Folgen'], ...[1, 2, 3, 5, 10, 20].map((n) => [String(n), `die letzten ${n}`])], d.keepLast)}${vbSel('pb', 'Vorlauf', vbPadOpts(VB_PB, dvr.padBefore), d.pb)}${vbSel('pa', 'Nachlauf', vbPadOpts(VB_PA, dvr.padAfter), d.pa)}${vbSel('keep', 'Aufbewahren', vbKeepOpts(vbStdKeepText()), d.keep)}</div>
    ${d.keepLast !== 'all' ? '<p class="hint">Ist eine neue Folge fertig, löscht Slouchcat die älteste (nach Sendebeginn). Fehlgeschlagene zählen nicht.</p>' : ''}
    <fieldset class="vb-fs vb-prop"><legend>Vorschlag, hängt an Frage F-23</legend><p class="muted vb-small">Diese Einschränkungen sind noch nicht entschieden.</p>
    ${vbRadios('win', 'Zeitfenster', [['any', 'jederzeit'], ['win', 'nur zu bestimmten Zeiten']], d.win)}${d.win === 'win' ? `<div class="vb-grid3"><div class="fld"><label for="vbf-from">Zwischen</label><input class="inp" type="time" step="900" id="vbf-from" value="${esc(d.from)}" data-bind="${vbDP('from')}" data-rerender data-fid="vbf:from"></div><div class="fld"><label for="vbf-to">und</label><input class="inp" type="time" step="900" id="vbf-to" value="${esc(d.to)}" data-bind="${vbDP('to')}" data-rerender data-fid="vbf:to"></div></div>` : ''}
    <div class="fld"><span class="lbl">Wochentage (freiwillig)</span>${wdp}</div>
    ${vbChk('onlyNew', 'Nur Erstausstrahlungen', d.onlyNew && knowsRepeat, { dis: !knowsRepeat, hint: knowsRepeat ? '' : 'Das Programm kennzeichnet Wiederholungen bei diesem Sender nicht.' })}</fieldset>
    <div class="vb-prev"><b>Demnächst</b>${prev ? `<ul>${prev}</ul>` : '<p class="muted">Gerade keine passende Ausstrahlung im Programm. Die Serienaufnahme wartet auf neue Termine.</p>'}</div>
    ${vbQuotaNotes(null)}`;
  return { title: rule ? 'Serienaufnahme ändern' : 'Serie aufnehmen', wide: true, body, actions: `${rule ? `<button type="button" class="btn danger vb-left" data-act="vbLd02End" data-id="${esc(rule.id)}" data-fid="ld02:end">Serienaufnahme beenden</button>` : ''}<button type="button" class="btn secondary" data-act="close" data-fid="ld02:x">Abbrechen</button><button type="button" class="btn primary" data-act="vbLd02Save" data-fid="ld02:ok">${rule ? 'Speichern' : 'Serie aufnehmen'}</button>` };
});
ACT.vbWd = (d) => { const x = ui.dialog.props._d; const n = Number(d.n); const k = x.wd.indexOf(n); if (k >= 0) x.wd.splice(k, 1); else x.wd.push(n); renderOverlays(); };
ACT.vbLd02Save = () => {
  const pr = ui.dialog.props; const d = pr._d; const rule = pr.ruleId ? SIM.rule(pr.ruleId) : null; const pid = pr.programId || vbRuleProgram(rule); const p = SIM.program(pid); const dvr = LIVE.settings.dvr;
  const progs = vbSeriesProgs(p, d.match);
  const opts = { match: d.match, channels: d.chans === 'all' ? 'all' : [d.ch], onlyNew: !!d.onlyNew && progs.some((x) => x.repeat || x.firstRun), noDup: progs.some((x) => x.sub), /* episode detection always works where it is reliable (11 2.4) */ keepLast: d.keepLast === 'all' ? null : Number(d.keepLast), padBefore: vbPadVal(d.pb, dvr.padBefore), padAfter: vbPadVal(d.pa, dvr.padAfter), keep: vbKeepFrom(d.keep, p.start), window: d.win === 'win' ? [d.from, d.to] : null, weekdays: d.wd.length ? d.wd.slice() : null };
  if (rule) SIM.updateRule(rule.id, opts); else SIM.planSeries(p.id, opts);
  closeDialog(); toast(`${rule ? 'Gespeichert' : 'Serienaufnahme angelegt'}: «${esc(p.seriesName || p.title)}». <a href="#/live/scheduled?tab=series">Serien</a>`);
};
ACT.vbLd02End = (d) => { closeDialog(); setTimeout(() => LiveDlg.confirmDelete({ kind: 'series', ids: [d.id] }), 0); };

/* ---------- LD-03 Manuelle oder wiederkehrende Aufnahme ---------- */
registerLiveDialog('LD-03', (pr) => {
  if (!SIM.rights().recordings) return vbNoRight();
  const rule = pr.ruleId ? SIM.rule(pr.ruleId) : null; const now = SIM.now(); const today = vbToday();
  if (!pr._d) {
    const q15 = Math.ceil(now / 15) * 15;
    if (rule && rule.kind === 'recurring') { const w = rule.weekdays || []; pr._d = { ch: rule.ch, day: String(today), from: rule.from, to: rule.to, rep: w.length === 7 ? 'daily' : w.join() === '1,2,3,4,5' ? 'weekdays' : 'weekly', wd: w.slice(), dur: String(rule.dur || ((LIVE.hm(rule.to) - LIVE.hm(rule.from) + 1440) % 1440) || 1440), name: rule.title, named: true, keepLast: rule.keepLast ? String(rule.keepLast) : 'all', keep: vbKeepKey(rule.keep), q: '' }; }
    else if (rule) pr._d = { ch: rule.ch, day: String(Math.floor(rule.start / 1440)), from: fmt.time(rule.start), to: fmt.time(rule.end), rep: 'once', wd: [], dur: String(Math.round(rule.end - rule.start)), name: rule.title, named: true, keepLast: 'all', keep: vbKeepKey(rule.keep), q: '' };
    else pr._d = { ch: pr.ch || '', day: String(pr.start != null ? Math.floor(pr.start / 1440) : today), from: fmt.time(pr.start != null ? pr.start : q15), to: fmt.time(pr.end != null ? pr.end : (pr.start != null ? pr.start : q15) + 60), rep: 'once', wd: [], dur: String(pr.start != null && pr.end != null ? Math.round(pr.end - pr.start) : 60), name: '', named: false, keepLast: 'all', keep: 'std', q: '' };
    pr._d.gone = SIM.chState(pr._d.ch) === 'gone' && pr._d.ch ? pr._d.ch : '';
    if (pr._d.gone) pr._d.ch = '';
  }
  const d = pr._d;
  const q = d.q.trim().toLowerCase();
  const chans = SIM.channels().filter((c) => !q || c.name.toLowerCase().includes(q) || String(c.no).startsWith(q));
  const c = d.ch ? SIM.channel(d.ch) : null;
  const day = Number(d.day); const a = day * 1440 + LIVE.hm(d.from || '00:00'); const durN = vbDurN(d.dur);
  let b = d.rep === 'once' ? day * 1440 + LIVE.hm(d.to || '00:00') : a + durN; const next = d.rep === 'once' && b <= a; if (next) b += 1440;
  const repText = d.rep === 'daily' ? 'täglich' : d.rep === 'weekdays' ? 'Mo–Fr' : vbWdText(d.wd);
  const auto = !c ? 'Manuelle Aufnahme' : d.rep === 'once' ? `${c.name}, ${fmt.day(a)}, ${fmt.time(a)}` : `${c.name}, ${repText} ${fmt.time(a)}`;
  if (!d.named) d.name = auto;
  const errs = []; if (!c) errs.push('Wähle einen Sender.'); if (d.rep === 'once' && b <= now) errs.push('Diese Zeit ist schon vorbei.'); if (d.rep === 'weekly' && !d.wd.length) errs.push('Wähle mindestens einen Wochentag.');
  const days = []; for (let x = today; x <= vbMaxDay(); x++) days.push([String(x), x === today ? 'Heute, ' + fmt.day(x * 1440) : x === today + 1 ? 'Morgen, ' + fmt.day(x * 1440) : fmt.day(x * 1440)]);
  const est = c ? vbEstGB(c.id, b - a) : null;
  const ovl = c && d.rep === 'once' ? vbOverlaps(c.id, a, b, rule && rule.id) : [];
  const body = (d.gone ? notice('bad', `${esc((SIM.channel(d.gone) || { name: d.gone }).name)} ist nicht mehr im Angebot.`, 'Wähle einen anderen Sender. Die Zeit bleibt.') : '')
    + `<div class="vb-grid2"><div class="fld"><label for="vbf-q">Sender suchen</label><input class="inp" id="vbf-q" type="search" placeholder="Nummer oder Name" value="${esc(d.q)}" data-bind="${vbDP('q')}" data-live data-fid="vbf:q" data-autofocus></div>${vbSel('ch', 'Sender', [['', chans.length ? 'Sender wählen …' : 'Kein Sender passt'], ...chans.map((x) => [x.id, `${x.no} ${x.name}${SIM.epgOf(x) === 'none' ? ' (ohne Programmdaten)' : ''}`])].concat(c && !chans.includes(c) ? [[c.id, `${c.no} ${c.name}`]] : []), d.ch)}</div>
    ${vbRadios('rep', 'Wiederholen', [['once', 'einmal'], ['daily', 'täglich'], ['weekdays', 'Mo–Fr'], ['weekly', 'an Wochentagen …']], d.rep)}
    ${d.rep === 'weekly' ? `<div class="vb-wd" role="group" aria-label="Wochentage">${[1, 2, 3, 4, 5, 6, 0].map((n) => `<button type="button" class="pill vb-pill" aria-pressed="${d.wd.includes(n)}" data-act="vbWd" data-n="${n}" data-fid="vbwd:${n}">${vbWdNames[n]}</button>`).join('')}</div>` : ''}
    <div class="vb-grid3">${d.rep === 'once' ? vbSel('day', 'Datum', days, d.day) : ''}<div class="fld"><label for="vbf-from">Beginn</label><input class="inp" type="time" id="vbf-from" step="300" value="${esc(d.from)}" data-bind="${vbDP('from')}" data-rerender data-fid="vbf:from"></div>${d.rep === 'once' ? `<div class="fld"><label for="vbf-to">Ende</label><input class="inp" type="time" id="vbf-to" step="300" value="${esc(d.to)}" data-bind="${vbDP('to')}" data-rerender data-fid="vbf:to">${next ? '<span class="hint">endet am Folgetag</span>' : ''}</div>` : `<div class="fld"><label for="vbf-dur">Dauer in Minuten</label><input class="inp" type="number" min="5" max="1440" step="5" id="vbf-dur" value="${esc(d.dur)}" data-bind="${vbDP('dur')}" data-rerender data-fid="vbf:dur"><span class="hint">bis ${fmt.time(b)}${b >= (day + 1) * 1440 ? ', am Folgetag' : ''}</span></div>`}</div>
    <p class="muted vb-small">Vor- und Nachlauf: 0 Minuten. Die Zeiten wählst du hier selbst.</p>
    ${d.rep !== 'once' ? `<p class="vb-small">Eine wiederkehrende Aufnahme läuft, bis du sie beendest. Termine entstehen 14 Tage im Voraus.</p><fieldset class="vb-fs vb-prop"><legend>Vorschlag, hängt an Frage F-23</legend><p class="muted vb-small">Ein Ende der Wiederholung (an einem Datum oder nach einer Anzahl Aufnahmen) ist noch nicht entschieden. Bis dahin gibt es das Feld nicht.</p></fieldset>` : ''}
    <div class="fld"><label for="vbf-name">Name</label><input class="inp" id="vbf-name" value="${esc(d.name)}" data-bind="${vbDP('name')}" data-vbname data-fid="vbf:name"><span class="hint">Vorgabe: «${esc(auto)}»</span></div>
    <div class="vb-grid3">${d.rep !== 'once' ? vbSel('keepLast', 'Behalten', [['all', 'alle'], ...[1, 2, 3, 5, 10, 20].map((n) => [String(n), `die letzten ${n}`])], d.keepLast) : ''}${vbSel('keep', 'Aufbewahren', vbKeepOpts(vbStdKeepText()), d.keep)}</div>
    ${c ? `<p class="vb-sum"><b>${d.rep === 'once' ? `${fmt.dayRel(a)}, ${fmt.time(a)} bis ${fmt.time(b)}${next ? ' (Folgetag)' : ''}` : `${d.rep === 'daily' ? 'täglich' : d.rep === 'weekdays' ? 'Mo–Fr' : vbWdText(d.wd)}, ${fmt.time(a)} bis ${fmt.time(b)}`}</b> · ${est != null ? 'etwa ' + fmt.gb(est) + (d.rep === 'once' ? '' : ' je Aufnahme') : 'Grösse unbekannt'}</p>` : ''}
    ${errs.length && (d.tried || d.rep === 'weekly') ? `<p class="err" role="alert">${errs.map(esc).join(' ')}</p>` : ''}
    ${ovl.map((u) => notice('', esc(`Überschneidet sich mit ‹${u.title}›. Beide werden aufgenommen.`), '')).join('')}${vbQuotaNotes(est)}
    <p class="muted vb-small">Bei der Umstellung auf Sommerzeit gibt es 02:00 bis 02:59 nicht: Eine Aufnahme in dieser Stunde beginnt um 03:00.</p>`;
  return { title: rule ? 'Aufnahme ändern' : 'Neue Aufnahme', wide: true, body, actions: `${rule ? `<button type="button" class="btn danger vb-left" data-act="vbLd01Cancel" data-id="${esc(rule.id)}" data-fid="ld03:cancel">Aufnahme abbrechen</button>` : ''}<button type="button" class="btn secondary" data-act="close" data-fid="ld03:x">Abbrechen</button><button type="button" class="btn primary" data-act="vbLd03Save" data-fid="ld03:ok">${rule ? 'Speichern' : 'Aufnahme planen'}</button>` };
});
/** duration of a recurring recording in minutes (11 stores the duration, so the clock change neither lengthens nor shortens it) */
function vbDurN(v) { return Math.max(5, Math.min(1440, Math.round(Number(v) || 60))); }
document.addEventListener('input', (e) => { if (e.target.matches && e.target.matches('[data-vbname]') && ui.dialog && ui.dialog.props._d) ui.dialog.props._d.named = true; });
ACT.vbLd03Save = () => {
  const pr = ui.dialog.props; const d = pr._d; const rule = pr.ruleId ? SIM.rule(pr.ruleId) : null; const now = SIM.now();
  const c = d.ch ? SIM.channel(d.ch) : null; const day = Number(d.day); const a = day * 1440 + LIVE.hm(d.from || '00:00'); let b = d.rep === 'once' ? day * 1440 + LIVE.hm(d.to || '00:00') : a + vbDurN(d.dur); if (b <= a) b += 1440;
  if (!c || (d.rep === 'once' && b <= now) || (d.rep === 'weekly' && !d.wd.length)) { d.tried = true; renderOverlays(); const er = $('#ov-dialog .err'); if (er) er.scrollIntoView({ block: 'nearest' }); return; }
  const keep = vbKeepFrom(d.keep, a); const title = (d.name || '').trim() || `${c.name}, ${fmt.day(a)}, ${fmt.time(a)}`;
  let o;
  if (d.rep === 'once') o = { ch: c.id, title, start: a, end: b, keep };
  else o = { ch: c.id, title, from: d.from, to: fmt.time(b), dur: vbDurN(d.dur), weekdays: d.rep === 'daily' ? [0, 1, 2, 3, 4, 5, 6] : d.rep === 'weekdays' ? [1, 2, 3, 4, 5] : d.wd.slice(), repeat: d.rep, keepLast: d.keepLast === 'all' ? null : Number(d.keepLast), padBefore: 0, padAfter: 0, keep };
  if (rule) {
    const kindNow = d.rep === 'once' ? 'manual' : 'recurring';
    if (rule.kind === kindNow) SIM.updateRule(rule.id, Object.assign({}, o, { warn: null }));
    else { SIM.removeRule(rule.id); SIM.planManual(o); }
    if (d.gone) SIM.warningsFor(rule.id).filter((w) => w.kind === 'channelGone').forEach((w) => SIM.dismissWarning(w.id));
  } else SIM.planManual(o);
  closeDialog(); toast(`${rule ? 'Gespeichert' : 'Aufnahme geplant'}: «${esc(title)}». <a href="#/live/scheduled${d.rep === 'once' ? '' : '?tab=recurring'}">Geplant</a>`);
};

/* ---------- LD-04 Sofortaufnahme ---------- */
registerLiveDialog('LD-04', (pr) => {
  if (!SIM.rights().recordings) return vbNoRight();
  const c = SIM.channel(pr.ch); if (!c) return vbDlgGone();
  const nn = SIM.nowNext(c.id).now; const run = SIM.runningOn(c.id); const p = SIM.player(); const pa = LIVE.settings.dvr.padAfter;
  if (VB.quotaFull || vbQuota().freeGB <= 0) {
    const q = vbQuota();
    return { title: 'Jetzt aufnehmen', body: `<p class="lead">${esc(c.name)}${nn ? ` · ${esc(nn.title)}` : ''}</p>${notice('bad', 'Kein Platz frei', `Im Speicher deines Kontos ist kein Platz frei (${fmt.gb(q.usedGB)} von ${fmt.gb(q.totalGB)}). Lösche Aufnahmen, um aufzunehmen.`)}`,
      actions: `<a class="btn secondary" href="#/live/recordings?sort=size" data-act="close" data-fid="ld04:list">Meine Aufnahmen</a><button type="button" class="btn primary" data-act="close" data-fid="ld04:x" data-autofocus>Schliessen</button>` };
  }
  if (run) return { title: 'Nimmt auf', body: `<p class="lead">${esc(c.name)} wird seit ${fmt.time(run.start)} aufgenommen, bis ${fmt.time(run.end)}.</p>`, actions: `<button type="button" class="btn danger" data-act="vbLd01Stop" data-id="${esc(run.id)}" data-fid="ld04:stop">Aufnahme beenden</button><button type="button" class="btn primary" data-act="close" data-fid="ld04:x" data-autofocus>Schliessen</button>` };
  if (!pr._d) pr._d = { until: nn ? 'end' : 'stop', keep: 'std' };
  const d = pr._d; const shifted = !!(p && p.ch === c.id && p.offset >= 0.05);
  if (!nn && d.until === 'end') d.until = 'stop';
  const opts = [nn ? ['end', `Bis Sendungsende (${fmt.time(nn.end)}, mit Nachlauf bis ${fmt.time(nn.end + pa)})`] : null, ['stop', `Bis ich stoppe (höchstens ${LIVE.settings.dvr.instantMaxH} Stunden)`]].filter(Boolean);
  const body = `<p class="lead"><b>${esc(c.name)}</b>${nn ? ` · ${esc(nn.title)} (bis ${fmt.time(nn.end)})` : ` · ${esc(LU.TX.epgNone)}`}</p>${vbRadios('until', 'Wie lange', opts, d.until).replace('<input', '<input data-autofocus')}${vbSel('keep', 'Aufbewahren', vbKeepOpts(vbStdKeepText()), d.keep)}
    <p>${esc(`Die Aufnahme beginnt jetzt (${fmt.time(SIM.now())}), nicht am Anfang der Sendung.`)}</p>${shifted ? notice('', 'Du schaust zeitversetzt.', 'Aufgenommen wird ab dem Live-Zeitpunkt.') : ''}`;
  return { title: 'Jetzt aufnehmen', body, actions: `<button type="button" class="btn secondary" data-act="close" data-fid="ld04:x">Abbrechen</button><button type="button" class="btn primary" data-act="vbLd04Go" data-fid="ld04:ok">${icon('record', 16)}${shifted ? 'Ab jetzt aufnehmen (live)' : 'Aufnehmen'}</button>` };
});
ACT.vbLd04Go = () => {
  const pr = ui.dialog.props; const d = pr._d; const t = SIM.now();
  const res = SIM.startInstant(pr.ch, { until: d.until === 'end' ? 'end' : 'stop', keep: vbKeepFrom(d.keep, t) });
  closeDialog();
  if (!res.ok) { toast('Im Speicher deines Kontos ist kein Platz frei.', 'bad'); return; }
  toast(`Aufnahme läuft bis ${fmt.time(res.end)}.`);
};

/* ---------- LD-05 Erinnerung ---------- */
registerLiveDialog('LD-05', (pr) => {
  const p = SIM.program(pr.programId); if (!p) return vbDlgGone();
  const c = SIM.channel(p.ch); const ex = SIM.reminderFor(p.id); const now = SIM.now();
  if (p.start <= now) return { title: 'Erinnern', body: vbDlgHead(p) + `<p class="lead">Die Sendung hat schon begonnen. Erinnern geht nur vor dem Beginn.</p>`, actions: `<button type="button" class="btn primary" data-act="close" data-fid="ld05:x" data-autofocus>Schliessen</button>` };
  if (!pr._d) pr._d = { lead: ex ? ex.lead : 5 };
  const d = pr._d; const leads = VB_LEADS.filter((l) => p.start - l > now || l === 0);
  const body = `<p class="lead">Erinnern an ‹${esc(p.title)}${p.sub ? ': ' + esc(p.sub) : ''}› · ${fmt.day(p.start)}, ${fmt.time(p.start)} · ${esc(c.name)}</p>
    <div class="fld"><span class="lbl" id="ld05-l">Zeitpunkt</span><div class="pills" role="radiogroup" aria-labelledby="ld05-l">${leads.map((l, i) => `<button type="button" class="pill" role="radio" aria-checked="${d.lead === l}" aria-pressed="${d.lead === l}" data-act="vbLead" data-l="${l}" data-fid="ld05:${l}"${(d.lead === l) || (!leads.includes(d.lead) && i === 0) ? ' data-autofocus' : ''}>${l ? `${l} Min. vorher` : 'zum Beginn'}</button>`).join('')}</div></div>
    <p class="vb-sum">Erinnerung um <b>${fmt.time(p.start - d.lead)}</b></p><p class="muted">Du bekommst die Erinnerung in Slouchcat und über die Meldewege deines Kontos.</p>`;
  return { title: ex ? 'Erinnerung ändern' : 'Erinnern', body, actions: `${ex ? `<button type="button" class="btn danger vb-left" data-act="vbLd05Del" data-fid="ld05:del">Erinnerung entfernen</button>` : ''}<button type="button" class="btn secondary" data-act="close" data-fid="ld05:x">Abbrechen</button><button type="button" class="btn primary" data-act="vbLd05Save" data-fid="ld05:ok">${ex ? 'Speichern' : 'Erinnern'}</button>` };
});
ACT.vbLead = (d) => { ui.dialog.props._d.lead = Number(d.l); renderOverlays(); };
ACT.vbLd05Save = () => { const pr = ui.dialog.props; const p = SIM.program(pr.programId); const l = pr._d.lead; SIM.setReminder(p.id, l); closeDialog(); toast(`Erinnerung um ${fmt.time(p.start - l)}: «${esc(p.title)}». <a href="#/live/reminders">Erinnerungen</a>`); };
ACT.vbLd05Del = () => { const pr = ui.dialog.props; closeDialog(); ACT.vbRemOff({ pid: pr.programId }); };
document.addEventListener('keydown', (e) => {
  const g = e.target.closest && e.target.closest('#ov-dialog [role="radiogroup"]'); if (!g || !/^Arrow/.test(e.key)) return;
  const bs = $$('[role="radio"]', g); const i = bs.indexOf(e.target); if (i < 0) return; e.preventDefault();
  const n = bs[(i + (e.key === 'ArrowRight' || e.key === 'ArrowDown' ? 1 : -1) + bs.length) % bs.length]; n.click(); setTimeout(() => { const x = $(`[data-fid="${n.dataset.fid}"]`); if (x) x.focus(); }, 0);
});

/* ---------- LD-06 Löschen und Abbrechen ---------- */
registerLiveDialog('LD-06', (pr) => {
  const kind = pr.kind || 'recording'; const ids = [].concat(pr.ids || []);
  if (!pr._d) pr._d = { mode: kind === 'occurrence' && ids[0] && ids[0].includes(':') ? 'one' : 'keep' };
  const d = pr._d;
  const x = (title, text, actions) => ({ title, body: text, actions });
  const cancel = (label) => `<button type="button" class="btn secondary" data-act="close" data-fid="ld06:x" data-autofocus>${label || 'Abbrechen'}</button>`;
  const go6 = (label, mode) => `<button type="button" class="btn danger" data-act="vbLd06Do" data-mode="${mode}" data-fid="ld06:${mode}">${label}</button>`;
  if (kind === 'recording' || kind === 'childRecording' || kind === 'running') {
    const a = vbRecAccess(ids[0]); if (!a) return x('Schon entfernt', '<p class="lead">Diese Aufnahme gibt es nicht mehr.</p>', cancel('Schliessen'));
    const r = a.r;
    const child = !a.own ? `<p>${esc(a.prof.name)} sieht die Aufnahme danach nicht mehr.</p>` : '';
    if (r.state === 'running') return x('Laufende Aufnahme', `<p class="lead">Die Aufnahme ‹${esc(vbRecTitle(r))}› läuft noch. Stoppen und löschen?</p>${child}<p class="muted">«Nur stoppen» behält, was bis jetzt aufgenommen ist (${fmt.gb(vbRecGB(r))}).</p>`, cancel() + `<button type="button" class="btn secondary" data-act="vbLd06Do" data-mode="stop" data-fid="ld06:stop">Nur stoppen, behalten</button>` + go6('Stoppen und löschen', 'stopdel'));
    return x(vbFailed(r) ? 'Eintrag entfernen' : 'Aufnahme löschen', vbFailed(r) ? `<p class="lead">‹${esc(vbRecTitle(r))}› aus der Liste entfernen?</p>` : `<p class="lead">‹${esc(vbRecTitle(r))}› (${fmt.gb(vbRecGB(r))}) löschen? Die Aufnahme wird endgültig entfernt und lässt sich nicht wiederherstellen.</p>${child}`, cancel() + go6(vbFailed(r) ? 'Entfernen' : 'Löschen', 'del'));
  }
  if (kind === 'recordings') {
    const rs = ids.map((id) => SIM.recording(id)).filter(Boolean); const gb = rs.reduce((s, r) => s + vbRecGB(r), 0); const run = rs.filter((r) => r.state === 'running').length;
    return x('Aufnahmen löschen', `<p class="lead">${rs.length} Aufnahmen (${fmt.gb(gb)}) löschen? Die Aufnahmen werden endgültig entfernt und lassen sich nicht wiederherstellen.</p>${run ? `<p>${run === 1 ? 'Eine davon läuft noch und wird gestoppt.' : run + ' davon laufen noch und werden gestoppt.'}</p>` : ''}`, cancel() + go6('Löschen', 'del'));
  }
  if (kind === 'occurrence') {
    const u = vbUpcoming().find((o) => o.id === ids[0]); if (!u) return x('Schon entfernt', '<p class="lead">Diesen Termin gibt es nicht mehr.</p>', cancel('Schliessen'));
    const multi = u.kind === 'series' || u.kind === 'recurring';
    return x('Aufnahme nicht machen', `<p class="lead">Aufnahme ‹${esc(u.title)}› ${fmt.when(u.start)} nicht machen?</p>${multi ? vbRadios('mode', 'Was soll entfallen?', [['one', 'Nur diesen Termin auslassen'], ['rule', `Ganze ${u.kind === 'series' ? 'Serienaufnahme' : 'wiederkehrende Aufnahme'} «${esc(u.rule.title)}» beenden`]], d.mode) : ''}`, cancel('Behalten') + go6('Nicht aufnehmen', multi ? d.mode : 'rule'));
  }
  if (kind === 'rule') {
    const r = SIM.rule(ids[0]); if (!r) return x('Schon entfernt', '<p class="lead">Diese Planung gibt es nicht mehr.</p>', cancel('Schliessen'));
    if (r.kind === 'series') { pr.kind = 'series'; return LIVE_DIALOGS['LD-06'](pr); }
    const n = vbUpcoming().filter((u) => u.rule.id === r.id).length; const u = vbUpcoming().find((o) => o.rule.id === r.id);
    return x(r.kind === 'recurring' ? 'Wiederkehrende Aufnahme beenden' : 'Aufnahme nicht machen', r.kind === 'recurring' ? `<p class="lead">Wiederkehrende Aufnahme ‹${esc(r.title)}› beenden? ${n === 1 ? '1 geplanter Termin entfällt' : n + ' geplante Termine entfallen'}. Fertige Aufnahmen bleiben.</p>` : `<p class="lead">Aufnahme ‹${esc(r.title)}›${u ? ' ' + fmt.when(u.start) : ''} nicht machen?</p>`, cancel('Behalten') + go6(r.kind === 'recurring' ? 'Beenden' : 'Nicht aufnehmen', 'rule'));
  }
  if (kind === 'series') {
    const r = SIM.rule(ids[0]); if (!r) return x('Schon entfernt', '<p class="lead">Diese Serienaufnahme gibt es nicht mehr.</p>', cancel('Schliessen'));
    const n = vbUpcoming().filter((u) => u.rule.id === r.id).length; const recs = SIM.recordings().filter((x2) => x2.rule === r.id); const gb = recs.reduce((s, x2) => s + vbRecGB(x2), 0);
    return x('Serienaufnahme beenden', `<p class="lead">Serienaufnahme ‹${esc(r.title)}› beenden? ${n === 1 ? '1 geplanter Termin entfällt' : n + ' geplante Termine entfallen'}.</p>${recs.length ? vbRadios('mode', `Deine ${recs.length} Aufnahmen`, [['keep', 'behalten'], ['del', `auch löschen (${fmt.gb(gb)})`]], d.mode) : ''}`, cancel('Behalten') + go6('Beenden', 'series'));
  }
  return x('Löschen', '<p class="lead">Unbekannter Fall.</p>', cancel('Schliessen'));
});
ACT.vbLd06Do = (dd) => {
  const pr = ui.dialog.props; const ids = [].concat(pr.ids || []); const kind = pr.kind; const d = pr._d || {}; const done = pr.onDone; const me = SIM.profile();
  let msg = '';
  if (kind === 'recording' || kind === 'childRecording' || kind === 'running') {
    const a = vbRecAccess(ids[0]); const r = a && a.r;
    if (r && dd.mode === 'stop') { SIM.stopRecording(r.id); msg = `Aufnahme gestoppt. ${fmt.gb(r.sizeGB)} bleiben.`; }
    else if (r) { const t = vbRecTitle(r); if (r.state === 'running') SIM.stopRecording(r.id); SIM.deleteRecordings([r.id]); msg = a.own ? `«${esc(t)}» ${vbFailed(r) ? 'entfernt' : 'gelöscht'}.` : `«${esc(t)}» gelöscht. ${esc(a.prof.name)} sieht in der Glocke: «${esc(me.name)} hat ‹${esc(t)}› gelöscht.»`; if (parseHash().path.startsWith('/live/recordings/' + r.id)) setTimeout(() => go(a.own ? '#/live/recordings' : '#/live/recordings/profiles'), 0); }
  } else if (kind === 'recordings') { ids.forEach((id) => { const r = SIM.recording(id); if (r && r.state === 'running') SIM.stopRecording(id); }); SIM.deleteRecordings(ids); msg = `${ids.length} Aufnahmen gelöscht.`; }
  else if (kind === 'occurrence') {
    const u = vbUpcoming().find((o) => o.id === ids[0]);
    if (u && dd.mode === 'one') { SIM.skipOccurrence(u.id); msg = `Termin ${fmt.when(u.start)} wird ausgelassen.`; }
    else if (u) { SIM.removeRule(u.rule.id); msg = `«${esc(u.rule.title)}» wird nicht aufgenommen.`; }
  } else if (kind === 'rule') { const r = SIM.rule(ids[0]); if (r) { SIM.removeRule(r.id); msg = `«${esc(r.title)}» wird nicht aufgenommen.`; } }
  else if (kind === 'series') { const r = SIM.rule(ids[0]); if (r) { const recs = SIM.recordings().filter((x) => x.rule === r.id); SIM.removeRule(r.id); if (d.mode === 'del' && recs.length) SIM.deleteRecordings(recs.map((x) => x.id)); msg = `Serienaufnahme «${esc(r.title)}» beendet${d.mode === 'del' && recs.length ? `, ${recs.length} Aufnahmen gelöscht` : ''}.`; } }
  closeDialog(); if (msg) toast(msg); if (typeof done === 'function') done();
  render();
};

/* ---------- LD-07 Konflikt oder Warnung ---------- */
registerLiveDialog('LD-07', (pr) => {
  const w = vbAllWarnings().find((x) => x.id === pr.warningId) || LIVE.warnings.find((x) => x.id === pr.warningId);
  if (!w) return { title: 'Erledigt', body: '<p class="lead">Diese Warnung gibt es nicht mehr.</p>', actions: `<button type="button" class="btn secondary" data-act="close" data-fid="ld07:x" data-autofocus>Schliessen</button>` };
  const up = vbUpcoming(); const occ = w.rules.map((id) => { const r = SIM.rule(id); const u = up.find((x) => x.rule.id === id); return r ? `<li>${esc(r.title)}${u ? ` · ${fmt.when(u.start)} · ${esc((SIM.channel(u.ch) || { name: u.ch }).name)}` : ''}</li>` : ''; }).join('');
  const sev = w.severity === 'bad' ? 'bad' : w.severity === 'warn' ? 'warn' : '';
  return { title: esc(VB_WKIND[w.kind] || 'Warnung'), body: notice(sev, esc(VB_WKIND[w.kind] || 'Warnung'), esc(w.text)) + (occ ? `<div><b>Betroffen</b><ul class="vbw-occ">${occ}</ul></div>` : '') + `<p class="muted">${w.kind === 'channelGone' ? 'Diese Warnung verhindert, dass die Aufnahme startet.' : 'Die Warnung verhindert das Planen nicht. Beim Start entscheidet der Server.'}</p><div class="row-gap vb-wact">${vbWarnActions(w, true).replace(/data-act="vbCancelRule"/g, 'data-act="vbDlgThen" data-do="vbCancelRule"').replace(/data-act="vbEditManual"/g, 'data-act="vbDlgThen" data-do="vbEditManual"')}</div>`,
    actions: `<button type="button" class="btn secondary" data-act="close" data-fid="ld07:x" data-autofocus>Schliessen</button>` };
});
ACT.vbDlgThen = (d, el, e) => { closeDialog(); const fn = ACT[d.do]; if (fn) setTimeout(() => fn(d, el, e), 0); };

/* ---------- LD-08 Tag und Uhrzeit wählen ---------- */
registerLiveDialog('LD-08', (pr) => {
  const today = vbToday();
  if (!pr._d) { const t = pr.time != null ? Number(pr.time) : SIM.now(); const m = Math.floor(((t % 1440) + 1440) % 1440); const day = pr.day != null ? Math.max(today, Number(pr.day)) : Math.max(today, Math.floor(t / 1440)); pr._d = { day: String(day), h: String(Math.floor(m / 60)), m: String(Math.floor((m % 60) / 15) * 15) }; }
  const d = pr._d; const days = []; for (let x = today; x <= vbMaxDay(); x++) days.push(x);
  const body = `<div class="fld"><span class="lbl" id="ld08-d">Tag</span><div class="pills" role="radiogroup" aria-labelledby="ld08-d">${days.map((x) => `<button type="button" class="pill" role="radio" aria-checked="${String(x) === d.day}" aria-pressed="${String(x) === d.day}" data-act="vbPickDay" data-day="${x}" data-fid="ld08:d${x}"${String(x) === d.day ? ' data-autofocus' : ''}>${x === today ? 'Heute' : x === today + 1 ? 'Morgen' : fmt.day(x * 1440)}</button>`).join('')}</div></div>
    <div class="vb-grid3">${vbSel('h', 'Stunde', Array.from({ length: 24 }, (_, i) => [String(i), String(i).padStart(2, '0')]), d.h)}${vbSel('m', 'Minute', [0, 15, 30, 45].map((n) => [String(n), String(n).padStart(2, '0')]), d.m)}</div>
    <p class="vb-sum">Springt zu <b>${fmt.dayRel(Number(d.day) * 1440)}, ${String(d.h).padStart(2, '0')}:${String(d.m).padStart(2, '0')}</b></p>`;
  return { title: 'Tag und Uhrzeit', body, actions: `<button type="button" class="btn secondary" data-act="close" data-fid="ld08:x">Abbrechen</button><button type="button" class="btn primary" data-act="vbLd08Go" data-fid="ld08:ok">Springen</button>` };
});
ACT.vbPickDay = (d) => { ui.dialog.props._d.day = String(d.day); renderOverlays(); };
ACT.vbLd08Go = () => { const pr = ui.dialog.props; const d = pr._d; const t = Number(d.day) * 1440 + Number(d.h) * 60 + Number(d.m); closeDialog(); if (typeof pr.onPick === 'function') pr.onPick(t); else toast(`Sprung zu ${fmt.when(t)} (Vorschau).`); };

/* ---------- LD-10 Senderwechsel bei Zeitversatz (modal form; the web player shows it non-modal over the bar) ---------- */
registerLiveDialog('LD-10', (pr) => {
  const c = SIM.channel(pr.to); const p = SIM.player(); const off = p ? Math.max(1, Math.round(p.offset)) : 12;
  return { title: c ? `Zu ${c.no} ${esc(c.name)} wechseln?` : 'Sender wechseln?', body: `<p class="lead">Du schaust ${vbMin(off)} zeitversetzt. Beim Senderwechsel geht das verloren.</p><label class="check"><input type="checkbox" data-vbnoask10 data-fid="ld10:ask"> Nicht mehr fragen (dieses Gerät)</label>`,
    actions: `<button type="button" class="btn secondary" data-act="close" data-fid="ld10:stay" data-autofocus>Bleiben</button><button type="button" class="btn primary" data-act="vbLd10Go" data-fid="ld10:go">Wechseln</button>` };
});
ACT.vbLd10Go = () => { const pr = ui.dialog.props; const cb = $('[data-vbnoask10]'); if (cb && cb.checked) VB.noAskZap = true; closeDialog(); if (typeof pr.onConfirm === 'function') pr.onConfirm(); else if (SIM.player()) vbSwitch(pr.to, { confirmed: true }); else ACT.tune({ ch: pr.to }); };

/* ---------- LD-11 Aufbewahrung ändern ---------- */
registerLiveDialog('LD-11', (pr) => {
  const a = vbRecAccess(pr.recordingId); if (!a || !a.own) return { title: 'Aufbewahrung', body: '<p class="lead">Diese Aufnahme gibt es nicht mehr.</p>', actions: `<button type="button" class="btn secondary" data-act="close" data-fid="ld11:x" data-autofocus>Schliessen</button>` };
  const r = a.r; const rule = r.rule ? SIM.rule(r.rule) : null;
  if (!pr._d) pr._d = { keep: vbKeepKey(r.keep), always: !!(r.keep && r.keep.always) };
  const d = pr._d; const until = d.keep === 'forever' ? null : r.start + Number(d.keep) * 1440;
  const body = `<p class="lead">‹${esc(vbRecTitle(r))}› · ${fmt.day(r.start)}</p>${vbRadios('keep', 'Aufbewahren', [['forever', 'bis du löschst'], ...VB_KEEP.map((n) => [String(n), `${n} Tage ab Aufnahme`])], d.keep).replace('<input', '<input data-autofocus')}
    ${until != null ? `<p class="vb-sum">${until <= SIM.now() ? 'Diese Frist ist schon abgelaufen: Slouchcat löscht die Aufnahme beim nächsten Aufräumen.' : `Wird am <b>${fmt.dayLong(until)}</b> gelöscht.`}</p>` : ''}
    ${rule && rule.keepLast ? vbChk('always', `Diese Folge immer behalten (zählt nicht zu den ${rule.keepLast})`, d.always) : ''}
    <p class="muted">Abgelaufene Aufnahmen löscht Slouchcat automatisch, weil du es so eingestellt hast.</p>`;
  return { title: 'Aufbewahrung ändern', body, actions: `<button type="button" class="btn secondary" data-act="close" data-fid="ld11:x">Abbrechen</button><button type="button" class="btn primary" data-act="vbLd11Save" data-fid="ld11:ok">Speichern</button>` };
});
ACT.vbLd11Save = () => { const pr = ui.dialog.props; const r = SIM.recording(pr.recordingId); const d = pr._d; const k = vbKeepFrom(d.keep, r.start); if (d.always) k.always = true; SIM.setKeep(r.id, k); closeDialog(); toast(`Aufbewahrung: ${esc(vbKeepText(r))}.`); };
