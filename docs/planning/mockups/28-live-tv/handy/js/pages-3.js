/* Konzept 28 «Live-TV», phone and tablet app: LS-11 scheduled, LS-12 conflicts and warnings, LS-13 reminders,
   LS-14 recordings of my profiles, LS-21 profile form block «Live-TV», LS-22 notification settings,
   LS-23 notifications, LS-24 search link, LS-25 sound priority (web radio), LS-26 start (unchanged), the
   reminder band, and the dialogs LD-01 to LD-08, LD-10, LD-11 as bottom sheets.
   14-oberflaeche.md 3.11 to 3.14, 1.2, 1.3, 5.1 to 5.11. Own globals start with hp / HP. */
'use strict';

/* ============================================================ dialog frame */
let HPD = null; /* { st, build, redraw } of the open dialog */
function hpRedraw() {
  const sh = $('#overlays .sheet'); if (!sh || !HPD) return;
  const f = document.activeElement && document.activeElement.dataset && document.activeElement.dataset.fid; const sc = sh.scrollTop;
  sh.innerHTML = HPD.build(); sh.scrollTop = sc; const e = f && sh.querySelector(`[data-fid="${f}"]`); if (e) e.focus();
}
function hpDlg(st, build) { HPD = { st, build, redraw: hpRedraw }; return { html: build() }; }
const hpCh = (g, v, text, on, fid, extra) => `<button type="button" class="chip${on ? ' on' : ''}" aria-pressed="${on}" data-act="hpDPick" data-g="${g}" data-v="${esc(v)}" ${extra || ''} data-fid="${fid || 'dc-' + g + '-' + v}">${esc(text)}</button>`;
ACT.hpDPick = (d) => { HPD.st[d.g] = d.v; if (HPD.pick) HPD.pick(d.g, d.v); hpRedraw(); };
ACT.hpDToggle = (d) => { HPD.st[d.g] = !HPD.st[d.g]; hpRedraw(); };
ACT.hpDDay = (d) => { const a = HPD.st.days; const v = Number(d.v); const k = a.indexOf(v); if (k < 0) a.push(v); else a.splice(k, 1); hpRedraw(); };
document.addEventListener('input', (e) => { const el = e.target.closest('[data-hpd]'); if (!el || !HPD) return; HPD.st[el.dataset.hpd] = el.value; if (el.dataset.hpd === 'name') HPD.st.nameTouched = true; });
document.addEventListener('change', (e) => { const el = e.target.closest('[data-hpd]'); if (!el || !HPD) return; HPD.st[el.dataset.hpd] = el.value; if (el.dataset.hpd === 'name') HPD.st.nameTouched = true; hpRedraw(); });
const hpMinOf = (s) => { const [h, m] = String(s || '0:0').split(':').map(Number); return h * 60 + m; };
const hpDef = () => LIVE.settings.dvr;
const PB = ['std', '0', '1', '2', '5', '10', '15', '30']; const PA = ['std', '0', '5', '10', '15', '30', '60']; const KP = ['std', '7', '14', '30', '90', 'forever'];
const hpPad = (g, v) => (v === 'std' ? `Standard (${g === 'pb' ? hpDef().padBefore : hpDef().padAfter} Min.)` : `${v} Min.`);
const hpKeepLbl = (v) => (v === 'std' ? 'Standard (bis du löschst)' : v === 'forever' ? 'Bis du löschst' : `${v} Tage`);
const hpPadVal = (g, v) => (v === 'std' ? (g === 'pb' ? hpDef().padBefore : hpDef().padAfter) : Number(v));
const hpKeepVal = (v, start) => (v === 'std' || v === 'forever' ? { mode: 'forever' } : { mode: 'days', days: Number(v), until: (start || SIM.now()) + Number(v) * 1440 });
const hpKeepKey = (k) => (!k || k.mode === 'forever' ? 'forever' : String(k.days));
const hpPadKey = (v, def) => (v === def ? 'std' : String(v));
function hpPadsHtml(st, withPads) {
  return `${withPads ? `<div class="sec">Vorlauf</div><div class="chips lh-wrap">${PB.map((v) => hpCh('pb', v, hpPad('pb', v), st.pb === v)).join('')}</div><div class="sec">Nachlauf</div><div class="chips lh-wrap">${PA.map((v) => hpCh('pa', v, hpPad('pa', v), st.pa === v)).join('')}</div>` : ''}
    <div class="sec">Aufbewahren</div><div class="chips lh-wrap">${KP.map((v) => hpCh('keep', v, hpKeepLbl(v), st.keep === v)).join('')}</div>`;
}
const hpNote = (kind, html) => `<div class="note${kind ? ' ' + kind : ''}" role="${kind === 'warn' || kind === 'bad' ? 'alert' : 'status'}">${icon(kind ? 'alert' : 'info', 20)}<div>${html}</div></div>`;
const hpActs = (...b) => `<div class="acts">${b.join('')}</div>`;
const hpBtn = (cls, act, text, fid, extra) => `<button type="button" class="btn ${cls}" data-act="${act}" ${extra || ''} data-fid="${fid}">${text}</button>`;

/* ---------- LD-01 Aufnahme planen / ändern ---------- */
registerLiveDialog('LD-01', (props) => {
  const p = SIM.program(props.programId); const rule = p && SIM.rules().find((r) => r.kind === 'single' && r.program === p.id); const sr = p && !rule ? SIM.ruleForProgram(p.id) : null;
  const st = { pid: props.programId, pb: rule ? hpPadKey(rule.padBefore, hpDef().padBefore) : 'std', pa: rule ? hpPadKey(rule.padAfter, hpDef().padAfter) : 'std', keep: rule ? hpKeepKey(rule.keep) : 'std', rule, sr };
  return hpDlg(st, () => {
    if (!p) return `<h2>Aufnahme planen</h2>${hpNote('bad', 'Diese Sendung steht nicht mehr im Programm.')}${hpActs(hpBtn('primary', 'close', 'Schliessen', 'ld1-x'))}`;
    const c = SIM.channel(p.ch); const t = SIM.now(); const run = p.start <= t && p.end > t; const past = p.end <= t;
    const head = `<h2>${st.rule ? 'Aufnahme ändern' : 'Aufnahme planen'}</h2><p class="pad hp-dsub"><b>${esc(p.title)}</b>${p.sub ? ` · «${esc(p.sub)}»` : ''}<br><small>${esc(c.name)} · ${esc(hpWhen(p))}</small></p>`;
    if (st.sr) return `${head}${hpNote('', `Diese Sendung wird über die Serienaufnahme «${esc(st.sr.title)}» aufgenommen.`)}${hpActs(hpBtn('ghost', 'hpLd1Skip', 'Nur diesen Termin auslassen', 'ld1-skip'), hpBtn('primary', 'hpLd1Ser', 'Serienaufnahme ändern', 'ld1-ser'))}<p class="pad"><button type="button" class="btn ghost block" data-act="close" data-fid="ld1-x">Schliessen</button></p>`;
    const pb = hpPadVal('pb', st.pb); const pa = hpPadVal('pa', st.pa); const s0 = run ? t : p.start - pb; const e0 = p.end + pa; const gb = (e0 - s0) * hpMin(c); const q = SIM.quota();
    const ov = SIM.upcoming().filter((u) => u.rule !== st.rule && u.start < e0 && u.end > s0 && u.program !== p.id);
    const warns = (st.rule ? SIM.warningsFor(st.rule.id) : []).map((w) => hpNote(w.severity === 'bad' ? 'bad' : w.severity === 'warn' ? 'warn' : '', esc(w.text)));
    if (gb > q.freeGB) warns.push(hpNote('warn', 'Für diese Aufnahme fehlt voraussichtlich Platz im Speicher des Kontos.'));
    ov.slice(0, 2).forEach((u) => warns.push(hpNote('', `Überschneidet sich mit «${esc(u.title)}». Beide werden aufgenommen.`)));
    return `${head}${past ? hpNote('bad', 'Diese Sendung ist schon vorbei.') : ''}${run ? hpNote('', `Die Sendung läuft seit ${fmt.time(p.start)}. Die Aufnahme beginnt jetzt.`) : ''}
      ${run ? '' : `<div class="sec">Vorlauf</div><div class="chips lh-wrap">${PB.map((v) => hpCh('pb', v, hpPad('pb', v), st.pb === v)).join('')}</div>`}<div class="sec">Nachlauf</div><div class="chips lh-wrap">${PA.map((v) => hpCh('pa', v, hpPad('pa', v), st.pa === v)).join('')}</div>
      <div class="sec">Aufbewahren</div><div class="chips lh-wrap">${KP.map((v) => hpCh('keep', v, hpKeepLbl(v), st.keep === v)).join('')}</div>
      <p class="pad hp-dline"><b>Aufnahme von ${fmt.time(s0)} bis ${fmt.time(e0)}</b> · ${hpGBtext(e0 - s0, c)}</p>${warns.join('')}
      ${hpSerial(p) && !st.rule ? `<p class="pad"><button type="button" class="hp-lnk" data-act="hpLd1Ser" data-fid="ld1-ser2">Stattdessen die Serie aufnehmen</button></p>` : ''}
      <p class="pad hint">Ändert sich die Sendezeit, passt Slouchcat die Aufnahme an.</p>
      ${st.rule ? `<p class="pad"><button type="button" class="hp-lnk danger" data-act="hpLd1Cancel" data-fid="ld1-cancel">Aufnahme abbrechen</button></p>` : ''}
      ${hpActs(hpBtn('ghost', 'close', 'Abbrechen', 'ld1-x'), hpBtn('primary', 'hpLd1Save', past ? 'Aufnahme planen' : run ? 'Ab jetzt aufnehmen' : st.rule ? 'Speichern' : 'Aufnahme planen', 'ld1-ok', past ? 'disabled' : ''))}`;
  });
});
ACT.hpLd1Save = () => {
  const st = HPD.st; const p = SIM.program(st.pid); const t = SIM.now(); const keep = hpKeepVal(st.keep, p.start);
  if (p.start <= t && p.end > t) { const r = SIM.startInstant(p.ch, { until: 'end', keep }); closeSheet(); if (!r.ok) return LiveDlg.instantRecording(p.ch); return toast(`Aufnahme läuft bis ${fmt.time(r.end)}.`); }
  SIM.planRecording(st.pid, { padBefore: hpPadVal('pb', st.pb), padAfter: hpPadVal('pa', st.pa), keep }); closeSheet(); toast(st.rule ? 'Aufnahme geändert' : `Aufnahme geplant: ${p.title}, ${fmt.time(p.start)}`);
};
ACT.hpLd1Ser = () => { const st = HPD.st; LiveDlg.seriesRecording(st.pid, st.sr ? st.sr.id : undefined); };
ACT.hpLd1Skip = () => { const st = HPD.st; const u = SIM.upcoming().find((x) => x.program === st.pid); closeSheet(); if (u) { SIM.skipOccurrence(u.id); toast('Dieser Termin wird ausgelassen'); } };
ACT.hpLd1Cancel = () => { const r = HPD.st.rule; LiveDlg.confirmDelete({ kind: 'occurrence', ids: [r.id] }); };

/* ---------- LD-02 Serienaufnahme ---------- */
registerLiveDialog('LD-02', (props) => {
  const rule = props.ruleId ? SIM.rule(props.ruleId) : null; const p = SIM.program(props.programId) || (rule && SIM.program(rule.next)) || LIVE.programs.find((x) => rule && x.series === rule.series);
  const series = rule ? rule.series : (p.series || p.title); const progs = LIVE.programs.filter((x) => (p && p.series ? x.series === series : x.title === series));
  const chIds = [...new Set(progs.map((x) => x.ch))]; const tab = progs.some((x) => x.repeat || x.firstRun); const dupOk = progs.some((x) => x.sub);
  const st = { rule, pid: p && p.id, series, scope: rule ? (rule.channels === 'all' ? 'all' : 'this') : 'this', win: rule && rule.window ? 'between' : 'any', from: (rule && rule.window && rule.window.from) || '20:00', to: (rule && rule.window && rule.window.to) || '22:30', days: rule && rule.weekdays ? rule.weekdays.slice() : [],
    onlyNew: rule ? !!rule.onlyNew : false, noDup: rule ? !!rule.noDup : true, keepLast: rule && rule.keepLast ? String(rule.keepLast) : 'all',
    pb: rule ? hpPadKey(rule.padBefore, hpDef().padBefore) : 'std', pa: rule ? hpPadKey(rule.padAfter, hpDef().padAfter) : 'std', keep: rule ? hpKeepKey(rule.keep) : 'std', ch: p ? p.ch : (rule && rule.channels !== 'all' ? rule.channels[0] : 'daserste'), chIds, tab, dupOk, byId: !!(p && p.series) };
  return hpDlg(st, () => {
    const names = chIds.map((id) => SIM.channel(id).name); const WD = [[1, 'Mo'], [2, 'Di'], [3, 'Mi'], [4, 'Do'], [5, 'Fr'], [6, 'Sa'], [0, 'So']];
    const t = SIM.now(); const sel = st.scope === 'all' ? chIds : [st.ch];
    let list = progs.filter((x) => x.start >= t && sel.includes(x.ch) && (!st.onlyNew || !x.repeat)).sort((a, b) => a.start - b.start);
    if (st.win === 'between') list = list.filter((x) => { const m = x.start % 1440; return m >= hpMinOf(st.from) && m <= hpMinOf(st.to); });
    if (st.days.length) list = list.filter((x) => st.days.includes((6 + Math.floor(x.start / 1440)) % 7));
    const seen = new Set(); const prev = []; const skipped = [];
    list.forEach((x) => { const k = x.sub || x.id; if (st.noDup && x.sub && seen.has(k)) skipped.push(x); else { seen.add(k); if (prev.length < 2) prev.push(x); } });
    const c0 = SIM.channel(st.ch);
    return `<h2>${st.rule ? 'Serienaufnahme ändern' : 'Serie aufnehmen'}</h2><p class="pad hp-dsub"><b>${esc(p ? (p.seriesName || p.title) : series)}</b><br><small>${st.byId ? 'Erkennung nach Serienkennung des Programms' : 'Erkennung nach Titel'}</small></p>
      ${st.byId ? '' : hpNote('', 'Slouchcat erkennt die Serie am Titel. Ähnliche Titel können mitkommen.')}
      <div class="sec">Sender</div><div class="chips lh-wrap">${hpCh('scope', 'this', `Nur ${c0.name}`, st.scope === 'this')}${hpCh('scope', 'all', 'Alle Sender, auf denen die Serie läuft', st.scope === 'all')}</div><p class="pad hint">Gefunden auf: ${esc(names.join(', '))}</p>
      <div class="sec">Zeitfenster</div><div class="chips lh-wrap">${hpCh('win', 'any', 'Jederzeit', st.win === 'any')}${hpCh('win', 'between', 'Nur zwischen …', st.win === 'between')}</div>
      ${st.win === 'between' ? `<div class="hp-two"><label class="field"><span>Von</span><input class="inp" type="time" step="900" value="${esc(st.from)}" data-hpd="from" data-fid="ld2-from"></label><label class="field"><span>Bis</span><input class="inp" type="time" step="900" value="${esc(st.to)}" data-hpd="to" data-fid="ld2-to"></label></div>` : ''}
      <div class="chips lh-wrap" role="group" aria-label="Wochentage (leer = alle)">${WD.map(([v, t2]) => `<button type="button" class="chip${st.days.includes(v) ? ' on' : ''}" aria-pressed="${st.days.includes(v)}" data-act="hpDDay" data-v="${v}" data-fid="ld2-d${v}">${t2}</button>`).join('')}</div>
      <div class="sec">Optionen</div>
      <button type="button" class="li" role="switch" aria-checked="${st.onlyNew}" ${st.tab ? '' : 'aria-disabled="true"'} data-act="${st.tab ? 'hpDToggle' : 'hpDNo'}" data-g="onlyNew" data-msg="Das Programm kennzeichnet Wiederholungen bei diesem Sender nicht." data-fid="ld2-new"><span class="tx"><b>Nur Erstausstrahlungen</b>${st.tab ? '' : '<small>Das Programm kennzeichnet Wiederholungen bei diesem Sender nicht.</small>'}</span><span class="sw" aria-checked="${st.onlyNew}"></span></button>
      <button type="button" class="li" role="switch" aria-checked="${st.noDup}" ${st.dupOk ? '' : 'aria-disabled="true"'} data-act="${st.dupOk ? 'hpDToggle' : 'hpDNo'}" data-g="noDup" data-msg="Folgen sind hier nicht eindeutig erkennbar." data-fid="ld2-dup"><span class="tx"><b>Folgen nicht doppelt aufnehmen</b>${st.dupOk ? '' : '<small>Folgen sind hier nicht eindeutig erkennbar. Slouchcat nimmt jede passende Ausstrahlung auf.</small>'}</span><span class="sw" aria-checked="${st.noDup}"></span></button>
      <div class="sec">Behalten</div><div class="chips lh-wrap">${['all', '1', '2', '3', '5', '10', '20'].map((v) => hpCh('keepLast', v, v === 'all' ? 'Alle Folgen' : `Die letzten ${v}`, st.keepLast === v)).join('')}</div>${st.keepLast !== 'all' ? '<p class="pad hint">Ist eine neue Folge fertig, löscht Slouchcat die älteste. Angesehene zuerst.</p>' : ''}
      ${hpPadsHtml(st, true)}
      <p class="pad hp-dline"><b>Demnächst</b><br>${prev.length ? prev.map((x) => `${fmt.day(x.start)} ${fmt.time(x.start)} ${esc(SIM.channel(x.ch).name)}`).join(' · ') : 'Kein Termin im Programm'}${skipped.length ? `<br><small>${skipped.slice(0, 2).map((x) => `${fmt.time(x.start)} ${esc(SIM.channel(x.ch).name)}: gleiche Folge, wird übersprungen`).join(' · ')}</small>` : ''}</p>
      ${st.rule ? `<p class="pad"><button type="button" class="hp-lnk danger" data-act="hpLd2End" data-fid="ld2-end">Serienaufnahme beenden</button></p>` : ''}
      ${hpActs(hpBtn('ghost', 'close', 'Abbrechen', 'ld2-x'), hpBtn('primary', 'hpLd2Save', st.rule ? 'Speichern' : 'Serie aufnehmen', 'ld2-ok'))}`;
  });
});
ACT.hpDNo = (d) => toast(d.msg);
ACT.hpLd2Save = () => {
  const st = HPD.st; const o = { channels: st.scope === 'all' ? 'all' : [st.ch], onlyNew: st.onlyNew, noDup: st.noDup, keepLast: st.keepLast === 'all' ? null : Number(st.keepLast), padBefore: hpPadVal('pb', st.pb), padAfter: hpPadVal('pa', st.pa), keep: hpKeepVal(st.keep), window: st.win === 'between' ? { from: st.from, to: st.to } : null, weekdays: st.days.length ? st.days.slice() : null };
  SIM.planSeries(st.pid, o); closeSheet(); toast(st.rule ? 'Serienaufnahme gespeichert' : `Serie geplant: ${st.series.charAt(0).toUpperCase() + st.series.slice(1)}`);
};
ACT.hpLd2End = () => LiveDlg.confirmDelete({ kind: 'series', ids: [HPD.st.rule.id] });

/* ---------- LD-03 Manuelle oder wiederkehrende Aufnahme ---------- */
registerLiveDialog('LD-03', (props) => {
  const rule = props.ruleId ? SIM.rule(props.ruleId) : null; const t = SIM.now(); const next = Math.ceil((t + 1) / 60) * 60;
  const s0 = rule ? (rule.kind === 'recurring' ? Math.floor(t / 1440) * 1440 + hpMinOf(rule.from) : rule.start) : (props.start != null ? props.start : next); const e0 = rule ? (rule.kind === 'recurring' ? Math.floor(t / 1440) * 1440 + hpMinOf(rule.to) : rule.end) : (props.end != null ? props.end : s0 + 60);
  const gone = rule && SIM.chState(rule.ch) === 'gone';
  const st = { rule, ch: gone ? '' : (rule ? rule.ch : props.ch || 'telezueri'), day: String(Math.max(0, Math.floor(s0 / 1440))), from: fmt.time(s0), to: fmt.time(e0), rep: rule ? (rule.kind === 'recurring' ? (rule.repeat === 'weekdays' ? 'weekdays' : rule.weekdays && rule.weekdays.length === 7 ? 'daily' : 'weekly') : 'once') : 'once', days: rule && rule.weekdays ? rule.weekdays.slice() : [6],
    ends: 'never', endDay: '6', count: '10', name: rule ? rule.title : '', nameTouched: !!rule, keepLast: rule && rule.keepLast ? String(rule.keepLast) : 'all', keep: rule ? hpKeepKey(rule.keep) : 'std', gone, fixedKind: rule ? rule.kind : null };
  return hpDlg(st, () => {
    const chs = SIM.channels(); const WD = [[1, 'Mo'], [2, 'Di'], [3, 'Mi'], [4, 'Do'], [5, 'Fr'], [6, 'Sa'], [0, 'So']]; const c = st.ch ? SIM.channel(st.ch) : null; const day = Number(st.day);
    const next2 = hpMinOf(st.to) <= hpMinOf(st.from); const rec = st.rep !== 'once';
    const auto = rec ? `${c ? c.name : 'Sender'}, ${st.from}` : `${c ? c.name : 'Sender'}, ${fmt.day(day * 1440)}, ${st.from}`; if (!st.nameTouched) st.name = auto;
    return `<h2>${st.rule ? 'Aufnahme ändern' : 'Neue Aufnahme'}</h2>
      ${st.gone ? hpNote('bad', 'TV24 ist nicht mehr im Angebot. 1 Aufnahme kann nicht starten. Wähle einen anderen Sender.') : ''}
      <label class="field pad"><span>Sender</span><select class="inp" data-hpd="ch" data-fid="ld3-ch" data-autofocus><option value="" ${st.ch ? '' : 'selected'}>Sender wählen</option>${chs.map((x) => `<option value="${x.id}" ${st.ch === x.id ? 'selected' : ''}>${x.no} ${esc(x.name)}</option>`).join('')}</select></label>
      <label class="field pad"><span>Datum</span><select class="inp" data-hpd="day" data-fid="ld3-day">${hpDays().map((d) => `<option value="${d}" ${day === d ? 'selected' : ''}>${esc(d < 2 ? fmt.dayRel(d * 1440) + ', ' + fmt.day(d * 1440) : fmt.dayLong(d * 1440))}</option>`).join('')}</select></label>
      <div class="hp-two"><label class="field"><span>Von</span><input class="inp" type="time" step="900" value="${esc(st.from)}" data-hpd="from" data-fid="ld3-from"></label><label class="field"><span>Bis</span><input class="inp" type="time" step="900" value="${esc(st.to)}" data-hpd="to" data-fid="ld3-to"></label></div>${next2 ? '<p class="pad hint">Endet am Folgetag.</p>' : ''}
      ${st.fixedKind === 'manual' || st.fixedKind === 'single' ? '' : `<div class="sec">Wiederholen</div><div class="chips lh-wrap">${[['once', 'Einmal'], ['daily', 'Täglich'], ['weekdays', 'Mo–Fr'], ['weekly', 'Wöchentlich']].map(([v, t2]) => hpCh('rep', v, t2, st.rep === v)).join('')}</div>`}
      ${st.rep === 'weekly' ? `<div class="chips lh-wrap" role="group" aria-label="Wochentage">${WD.map(([v, t2]) => `<button type="button" class="chip${st.days.includes(v) ? ' on' : ''}" aria-pressed="${st.days.includes(v)}" data-act="hpDDay" data-v="${v}" data-fid="ld3-d${v}">${t2}</button>`).join('')}</div>` : ''}
      ${rec ? `<div class="sec">Wiederholung endet</div><div class="chips lh-wrap">${[['never', 'Nie'], ['date', 'Am Datum'], ['count', 'Nach N Aufnahmen']].map(([v, t2]) => hpCh('ends', v, t2, st.ends === v)).join('')}</div>
        ${st.ends === 'date' ? `<label class="field pad"><span>Letzter Tag</span><select class="inp" data-hpd="endDay" data-fid="ld3-endday">${hpDays().map((d) => `<option value="${d}" ${String(d) === st.endDay ? 'selected' : ''}>${esc(fmt.dayLong(d * 1440))}</option>`).join('')}</select></label>` : ''}${st.ends === 'count' ? `<label class="field pad"><span>Anzahl</span><input class="inp" type="number" min="1" max="99" value="${esc(st.count)}" data-hpd="count" data-fid="ld3-count"></label>` : ''}` : ''}
      <label class="field pad"><span>Name</span><input class="inp" type="text" value="${esc(st.name)}" data-hpd="name" data-fid="ld3-name"></label>
      ${rec ? `<div class="sec">Behalten</div><div class="chips lh-wrap">${['all', '1', '2', '3', '5', '10', '20'].map((v) => hpCh('keepLast', v, v === 'all' ? 'Alle' : `Die letzten ${v}`, st.keepLast === v)).join('')}</div>` : ''}
      <div class="sec">Aufbewahren</div><div class="chips lh-wrap">${KP.map((v) => hpCh('keep', v, hpKeepLbl(v), st.keep === v)).join('')}</div>
      ${c && hpGBtext(60, c) ? `<p class="pad hp-dline">${fmt.time(day * 1440 + hpMinOf(st.from))} bis ${fmt.time(hpMinOf(st.to))} · ${hpGBtext(((hpMinOf(st.to) - hpMinOf(st.from) + 1440) % 1440) || 1440, c)}</p>` : ''}
      ${st.rule ? `<p class="pad"><button type="button" class="hp-lnk danger" data-act="hpLd3Del" data-fid="ld3-del">Aufnahme abbrechen</button></p>` : ''}
      ${hpActs(hpBtn('ghost', 'close', 'Abbrechen', 'ld3-x'), hpBtn('primary', 'hpLd3Save', st.rule ? 'Speichern' : 'Aufnahme planen', 'ld3-ok', st.ch ? '' : 'disabled aria-disabled="true"'))}`;
  });
});
ACT.hpLd3Save = () => {
  const st = HPD.st; if (!st.ch) return; const c = SIM.channel(st.ch); const day = Number(st.day); const f = hpMinOf(st.from); let e = hpMinOf(st.to); if (e <= f) e += 1440;
  const keep = hpKeepVal(st.keep); const title = st.name || `${c.name}, ${st.from}`; const rec = st.rep !== 'once'; const kl = st.keepLast === 'all' ? null : Number(st.keepLast);
  if (st.rule) {
    const r = st.rule; const patch = { ch: st.ch, title, keep };
    if (r.kind === 'recurring') Object.assign(patch, { from: st.from, to: st.to, weekdays: st.rep === 'daily' ? [0, 1, 2, 3, 4, 5, 6] : st.rep === 'weekdays' ? [1, 2, 3, 4, 5] : st.days.slice(), repeat: st.rep, keepLast: kl }); else Object.assign(patch, { start: day * 1440 + f, end: day * 1440 + e });
    if (r.warn) { patch.warn = null; SIM.dismissWarning('w-gone'); }
    SIM.updateRule(r.id, patch); closeSheet(); return toast('Aufnahme geändert');
  }
  if (rec) SIM.planManual({ ch: st.ch, title, from: st.from, to: st.to, weekdays: st.rep === 'daily' ? [0, 1, 2, 3, 4, 5, 6] : st.rep === 'weekdays' ? [1, 2, 3, 4, 5] : (st.days.length ? st.days.slice() : [6]), repeat: st.rep, keepLast: kl, keep, endsAfter: st.ends === 'count' ? Number(st.count) : null, padBefore: 0, padAfter: 0 });
  else SIM.planManual({ ch: st.ch, title, start: day * 1440 + f, end: day * 1440 + e, keep });
  closeSheet(); toast(`Aufnahme geplant: ${title}`);
};
ACT.hpLd3Del = () => LiveDlg.confirmDelete({ kind: 'rule', ids: [HPD.st.rule.id] });

/* ---------- LD-04 Sofortaufnahme ---------- */
registerLiveDialog('LD-04', (props) => {
  const c = SIM.channel(props.ch); const nn = SIM.nowNext(c.id).now; const st = { ch: c.id, until: nn ? 'end' : '60', keep: 'std', err: false };
  return hpDlg(st, () => {
    const pl = SIM.player(); const q = SIM.quota(); const full = q.freeGB <= 0 || st.err; const shifted = pl && pl.offset > 0.05;
    if (full) return `<h2>Jetzt aufnehmen</h2>${hpNote('bad', `Im Speicher deines Kontos ist kein Platz frei (${fmt.gb(q.usedGB)} von ${fmt.gb(q.totalGB)}). Lösche Aufnahmen, um aufzunehmen.`)}${hpActs(hpBtn('ghost', 'close', 'Schliessen', 'ld4-x'), `<a class="btn primary" href="#/live/recordings" data-fid="ld4-go">Meine Aufnahmen</a>`)}${hpFullSwitch()}`;
    const pa = hpDef().padAfter;
    return `<h2>Jetzt aufnehmen</h2><p class="pad hp-dsub"><b>${esc(c.name)}</b>${nn ? ` · ${esc(nn.title)} (bis ${fmt.time(nn.end)})` : ''}</p>
      <div class="hp-radios" role="radiogroup" aria-label="Dauer">${[nn ? ['end', `Bis Sendungsende (${fmt.time(nn.end)}, mit Nachlauf bis ${fmt.time(nn.end + pa)})`] : null, ['30', '30 Minuten'], ['60', '1 Stunde'], ['stop', `Bis ich stoppe (höchstens ${hpDef().instantMaxH} Stunden)`]].filter(Boolean).map(([v, t]) => `<button type="button" class="li" role="radio" aria-checked="${st.until === v}" data-act="hpDPick" data-g="until" data-v="${v}" data-fid="ld4-${v}"><span class="hp-radio${st.until === v ? ' on' : ''}" aria-hidden="true"></span><span class="tx"><b>${esc(t)}</b></span></button>`).join('')}</div>
      ${hpPadsHtml(st, false)}
      <p class="pad hint">${shifted ? 'Du schaust zeitversetzt. Aufgenommen wird ab dem Live-Zeitpunkt.' : `Die Aufnahme beginnt jetzt (${fmt.time(SIM.now())}), nicht am Anfang der Sendung.`}</p>
      ${hpActs(hpBtn('ghost', 'close', 'Abbrechen', 'ld4-x'), hpBtn('primary', 'hpLd4Go', 'Aufnehmen', 'ld4-ok'))}${hpFullSwitch()}`;
  });
});
ACT.hpLd4Go = () => {
  const st = HPD.st; const r = SIM.startInstant(st.ch, { until: st.until === 'end' ? 'end' : st.until === 'stop' ? 'stop' : Number(st.until), keep: hpKeepVal(st.keep) });
  if (!r.ok) { st.err = true; return hpRedraw(); }
  closeSheet(); toast(`Aufnahme läuft bis ${fmt.time(r.end)}.`);
};

/* ---------- LD-05 Erinnerung ---------- */
registerLiveDialog('LD-05', (props) => {
  const p = SIM.program(props.programId); const ex = p && SIM.reminderFor(p.id); const st = { pid: props.programId, lead: String(ex ? ex.lead : 5), ex };
  return hpDlg(st, () => {
    if (!p) return `<h2>Erinnerung</h2>${hpNote('bad', 'Diese Sendung steht nicht mehr im Programm.')}${hpActs(hpBtn('primary', 'close', 'Schliessen', 'ld5-x'))}`;
    const c = SIM.channel(p.ch); const past = p.start <= SIM.now();
    return `<h2>${st.ex ? 'Erinnerung ändern' : 'Erinnern'}</h2><p class="pad hp-dsub">An <b>«${esc(p.title)}${p.sub ? ': ' + esc(p.sub) : ''}»</b><br><small>${esc(hpWhen(p))} · ${esc(c.name)}</small></p>${past ? hpNote('bad', 'Die Sendung hat schon begonnen.') : ''}
      <div class="sec">Zeitpunkt</div><div class="chips lh-wrap" role="group" aria-label="Zeitpunkt">${[['0', 'Zum Beginn'], ['2', '2 Min. vorher'], ['5', '5 Min. vorher'], ['10', '10 Min. vorher'], ['15', '15 Min. vorher']].map(([v, t]) => hpCh('lead', v, t, st.lead === v)).join('')}</div>
      <p class="pad hint">Du bekommst die Erinnerung in Slouchcat und als Mitteilung auf deinen Geräten.</p>
      ${st.ex ? `<p class="pad"><button type="button" class="hp-lnk danger" data-act="hpLd5Del" data-fid="ld5-del">Erinnerung entfernen</button></p>` : ''}
      ${hpActs(hpBtn('ghost', 'close', 'Abbrechen', 'ld5-x'), hpBtn('primary', 'hpLd5Save', st.ex ? 'Speichern' : 'Erinnern', 'ld5-ok', past ? 'disabled' : ''))}`;
  });
});
ACT.hpLd5Save = () => { const st = HPD.st; SIM.setReminder(st.pid, Number(st.lead)); closeSheet(); toast(`Erinnerung ${Number(st.lead) ? st.lead + ' Min. vorher' : 'zum Beginn'} gesetzt`); };
ACT.hpLd5Del = () => { SIM.removeReminder(HPD.st.pid); closeSheet(); toast('Erinnerung entfernt'); };

/* ---------- LD-06 Löschen und Abbrechen ---------- */
registerLiveDialog('LD-06', (props) => {
  const st = { kind: props.kind, ids: props.ids || [], onDone: props.onDone, opt: 'one', keepSeries: 'keep' };
  return hpDlg(st, () => {
    const ids = st.ids; const done = () => { closeSheet(); if (st.onDone) st.onDone(); };
    if (st.kind === 'recording' || st.kind === 'childRecording') {
      const r = SIM.recording(ids[0]); if (!r) return `<h2>Löschen</h2>${hpActs(hpBtn('primary', 'close', 'Schliessen', 'ld6-x'))}`;
      return `<h2>Aufnahme löschen</h2><p class="pad">«${esc(r.title)}» (${fmt.gb(r.sizeGB)}) löschen? Die Aufnahme wird endgültig entfernt und lässt sich nicht wiederherstellen.${st.kind === 'childRecording' ? `<br>${esc(hpProfName(r.profile))} sieht die Aufnahme danach nicht mehr.` : ''}</p>${hpActs(hpBtn('ghost', 'close', 'Abbrechen', 'ld6-x', 'data-autofocus'), hpBtn('danger', 'hpLd6Del', 'Löschen', 'ld6-ok'))}`;
    }
    if (st.kind === 'recordings') {
      const rs = ids.map((i) => SIM.recording(i)).filter(Boolean);
      return `<h2>Aufnahmen löschen</h2><p class="pad">${rs.length === 1 ? '1 Aufnahme' : rs.length + ' Aufnahmen'} (${fmt.gb(rs.reduce((a, r) => a + r.sizeGB, 0))}) löschen? ${rs.length === 1 ? 'Die Aufnahme wird' : 'Die Aufnahmen werden'} endgültig entfernt und ${rs.length === 1 ? 'lässt' : 'lassen'} sich nicht wiederherstellen.</p>${hpActs(hpBtn('ghost', 'close', 'Abbrechen', 'ld6-x', 'data-autofocus'), hpBtn('danger', 'hpLd6Del', 'Löschen', 'ld6-ok'))}`;
    }
    if (st.kind === 'running') {
      const r = SIM.recording(ids[0]);
      return `<h2>Aufnahme beenden</h2><p class="pad">Die Aufnahme «${esc(r.title)}» läuft noch. Stoppen und löschen?</p>${hpActs(hpBtn('ghost', 'close', 'Abbrechen', 'ld6-x', 'data-autofocus'), hpBtn('secondary', 'hpLd6Stop', 'Nur stoppen', 'ld6-stop'), hpBtn('danger', 'hpLd6Del', 'Stoppen und löschen', 'ld6-ok'))}`;
    }
    if (st.kind === 'occurrence') {
      const [rid] = String(ids[0]).split(':'); const rule = SIM.rule(rid); const u = SIM.upcoming().find((x) => x.id === ids[0]); const multi = rule && (rule.kind === 'series' || rule.kind === 'recurring');
      return `<h2>Aufnahme abbrechen</h2><p class="pad">Aufnahme «${esc(u ? u.title : rule ? rule.title : '')}» ${u ? esc(fmt.dayRel(u.start).toLowerCase() + ' ' + fmt.time(u.start)) : ''} nicht machen?</p>${multi ? `<div class="hp-radios" role="radiogroup">${[['one', 'Nur diesen Termin auslassen'], ['all', 'Ganze Regel beenden']].map(([v, t]) => `<button type="button" class="li" role="radio" aria-checked="${st.opt === v}" data-act="hpDPick" data-g="opt" data-v="${v}" data-fid="ld6-${v}"><span class="hp-radio${st.opt === v ? ' on' : ''}" aria-hidden="true"></span><span class="tx"><b>${t}</b></span></button>`).join('')}</div>` : ''}${hpActs(hpBtn('ghost', 'close', 'Behalten', 'ld6-x', 'data-autofocus'), hpBtn('danger', 'hpLd6Occ', 'Nicht aufnehmen', 'ld6-ok'))}`;
    }
    if (st.kind === 'rule') { const rule = SIM.rule(ids[0]); return `<h2>Aufnahme abbrechen</h2><p class="pad">«${esc(rule ? rule.title : '')}» nicht mehr aufnehmen? Alle geplanten Termine entfallen.</p>${hpActs(hpBtn('ghost', 'close', 'Behalten', 'ld6-x', 'data-autofocus'), hpBtn('danger', 'hpLd6Rule', 'Nicht aufnehmen', 'ld6-ok'))}`; }
    if (st.kind === 'series') {
      const rule = SIM.rule(ids[0]); const mine = SIM.recordings().filter((x) => x.rule === ids[0]); const n = SIM.upcoming().filter((u) => u.rule.id === ids[0]).length;
      return `<h2>Serienaufnahme beenden</h2><p class="pad">Serienaufnahme «${esc(rule.title)}» beenden? ${n === 1 ? '1 geplanter Termin entfällt' : n + ' geplante Termine entfallen'}. Deine ${mine.length} Aufnahmen:</p><div class="hp-radios" role="radiogroup">${[['keep', 'Behalten'], ['del', `Auch löschen (${fmt.gb(mine.reduce((a, x) => a + x.sizeGB, 0))})`]].map(([v, t]) => `<button type="button" class="li" role="radio" aria-checked="${st.keepSeries === v}" data-act="hpDPick" data-g="keepSeries" data-v="${v}" data-fid="ld6-${v}"><span class="hp-radio${st.keepSeries === v ? ' on' : ''}" aria-hidden="true"></span><span class="tx"><b>${t}</b></span></button>`).join('')}</div>${hpActs(hpBtn('ghost', 'close', 'Behalten', 'ld6-x', 'data-autofocus'), hpBtn('danger', 'hpLd6Series', 'Beenden', 'ld6-ok'))}`;
    }
    return `<h2>Löschen</h2>${hpActs(hpBtn('primary', 'close', 'Schliessen', 'ld6-x'))}`;
  });
});
const hpLd6Done = (msg) => { const st = HPD.st; closeSheet(); toast(msg); if (st.onDone) st.onDone(); };
ACT.hpLd6Del = () => { const st = HPD.st; const n = st.ids.length; const r = SIM.recording(st.ids[0]); const t = r && r.title; SIM.deleteRecordings(st.ids); hpLd6Done(n > 1 ? `${n} Aufnahmen gelöscht` : `«${t}» gelöscht`); };
ACT.hpLd6Stop = () => { SIM.stopRecording(HPD.st.ids[0]); hpLd6Done('Aufnahme gestoppt und behalten'); };
ACT.hpLd6Occ = () => { const st = HPD.st; const [rid] = String(st.ids[0]).split(':'); const rule = SIM.rule(rid); if (rule && (rule.kind === 'series' || rule.kind === 'recurring') && st.opt === 'one') { SIM.skipOccurrence(st.ids[0]); return hpLd6Done('Dieser Termin wird ausgelassen'); } SIM.removeRule(rid); hpLd6Done('Aufnahme abgebrochen'); };
ACT.hpLd6Rule = () => { SIM.removeRule(HPD.st.ids[0]); hpLd6Done('Aufnahme abgebrochen'); };
ACT.hpLd6Series = () => { const st = HPD.st; if (st.keepSeries === 'del') SIM.deleteRecordings(SIM.recordings().filter((x) => x.rule === st.ids[0]).map((x) => x.id)); SIM.removeRule(st.ids[0]); hpLd6Done('Serienaufnahme beendet'); };

/* ---------- LD-07 Konflikt oder Warnung ---------- */
const HP_WKIND = { quota: 'Speicher des Kontos', serverStorage: 'Speicher des Servers', overlap: 'Überschneidung', duplicate: 'Doppelt', channelGone: 'Sender weg', programGone: 'Sendung verschwunden', timeChanged: 'Zeit geändert', dst: 'Sommer-/Winterzeit' };
function hpWarnActions(w) {
  const r0 = w.rules[0]; const rl = SIM.rule(r0); const b = [];
  if (w.kind === 'quota') { b.push(hpBtn('secondary', 'hpWClean', 'Aufnahmen aufräumen', 'w-clean-' + w.id)); b.push(hpBtn('ghost', 'hpWKeep', 'Trotzdem behalten', 'w-keep-' + w.id)); }
  else if (w.kind === 'overlap') w.rules.forEach((id, i) => b.push(hpBtn(i ? 'ghost' : 'secondary', 'hpWCancel', `${i ? 'Oder ' : 'Eine abbrechen: '}«${esc((SIM.rule(id) || {}).title || '')}»`, `w-c${i}-${w.id}`, `data-r="${id}"`)));
  else if (w.kind === 'duplicate') b.push(hpBtn('secondary', 'hpWCancel', 'Einzelaufnahme entfernen', 'w-dup-' + w.id, `data-r="${r0}"`));
  else if (w.kind === 'channelGone') { b.push(hpBtn('secondary', 'hpWChannel', 'Anderen Sender wählen', 'w-ch-' + w.id, `data-r="${r0}"`)); b.push(hpBtn('danger', 'hpWCancel', 'Abbrechen', 'w-cx-' + w.id, `data-r="${r0}"`)); }
  else if (w.kind === 'programGone') { b.push(hpBtn('secondary', 'hpWChannel', 'Zeit trotzdem aufnehmen', 'w-pg-' + w.id, `data-r="${r0}"`)); b.push(hpBtn('ghost', 'hpWCancel', 'Entfernen', 'w-px-' + w.id, `data-r="${r0}"`)); }
  else b.push(hpBtn('secondary', 'hpWOk', 'OK', 'w-ok-' + w.id, `data-w="${w.id}"`));
  return b.join('');
}
registerLiveDialog('LD-07', (props) => {
  const st = { id: props.warningId };
  return hpDlg(st, () => {
    const w = SIM.warnings().find((x) => x.id === st.id) || (LIVE.warnings.find((x) => x.id === st.id));
    if (!w) return `<h2>Warnung</h2>${hpNote('', 'Diese Warnung gibt es nicht mehr.')}${hpActs(hpBtn('primary', 'close', 'Schliessen', 'ld7-x'))}`;
    return `<h2>${esc(HP_WKIND[w.kind] || 'Warnung')}</h2>${hpNote(w.severity === 'bad' ? 'bad' : w.severity === 'warn' ? 'warn' : '', esc(w.text))}<div class="hp-wl">${w.rules.map((id) => { const r = SIM.rule(id); return r ? `<p class="pad hint">Betrifft: ${esc(r.title)}</p>` : ''; }).join('')}</div><p class="pad hint">Die Warnung verhindert das Planen nicht. Beim Start entscheidet der Server.</p><div class="acts hp-wacts">${hpWarnActions(w)}</div><p class="pad"><button type="button" class="btn ghost block" data-act="close" data-fid="ld7-x">Schliessen</button></p>`;
  });
});
ACT.hpWClean = () => { HP.rec.sort = 'size'; closeSheet(true); go('#/live/recordings'); };
ACT.hpWKeep = () => { closeSheet(); toast('Die Warnung bleibt sichtbar'); };
ACT.hpWOk = (d) => { SIM.dismissWarning(d.w); closeSheet(); toast('Warnung ausgeblendet'); };
ACT.hpWCancel = (d) => { const r = SIM.rule(d.r); if (!r) return; const u = SIM.upcoming().find((x) => x.rule.id === r.id); LiveDlg.confirmDelete({ kind: r.kind === 'single' || r.kind === 'manual' ? 'occurrence' : 'rule', ids: [r.kind === 'single' || r.kind === 'manual' ? r.id : r.id] }); };
ACT.hpWChannel = (d) => LiveDlg.manualRecording({ ruleId: d.r });

/* ---------- LD-08 Tag und Uhrzeit ---------- */
registerLiveDialog('LD-08', (props) => {
  const st = { day: String(props.day || 0), h: String(Math.floor((props.time || 0) / 60)), m: String(Math.round(((props.time || 0) % 60) / 15) * 15 % 60), onPick: props.onPick };
  return hpDlg(st, () => `<h2>Tag und Uhrzeit</h2><div class="sec">Tag</div><div class="chips lh-wrap">${hpDays().map((d) => hpCh('day', String(d), fmt.dayRel(d * 1440), st.day === String(d), 'ld8-d' + d)).join('')}</div>
    <div class="hp-two pad"><label class="field"><span>Stunde</span><select class="inp" data-hpd="h" data-fid="ld8-h">${Array.from({ length: 24 }, (_, i) => `<option value="${i}" ${String(i) === st.h ? 'selected' : ''}>${String(i).padStart(2, '0')}</option>`).join('')}</select></label>
    <label class="field"><span>Minute</span><select class="inp" data-hpd="m" data-fid="ld8-m">${[0, 15, 30, 45].map((i) => `<option value="${i}" ${String(i) === st.m ? 'selected' : ''}>${String(i).padStart(2, '0')}</option>`).join('')}</select></label></div>
    ${hpActs(hpBtn('ghost', 'close', 'Abbrechen', 'ld8-x'), hpBtn('primary', 'hpLd8Go', 'Springen', 'ld8-ok'))}`);
});
ACT.hpLd8Go = () => { const st = HPD.st; closeSheet(); if (st.onPick) st.onPick({ day: Number(st.day), time: Number(st.h) * 60 + Number(st.m) }); };

/* ---------- LD-10 Senderwechsel bei Zeitversatz ---------- */
registerLiveDialog('LD-10', (props) => {
  const st = { to: props.to, onConfirm: props.onConfirm, ask: false };
  return hpDlg(st, () => {
    const pl = SIM.player(); const off = pl ? pl.offset : 12; const to = SIM.channel(st.to);
    return `<h2>Senderwechsel</h2><p class="pad">Du schaust ${pl && pl.paused && off < 1 ? 'zeitversetzt' : fmt.dur(Math.max(1, off)) + ' zeitversetzt'}. Beim Senderwechsel geht das verloren.<br><small>Wechsel zu ${to ? `${to.no} ${esc(to.name)}` : ''}</small></p>
      <button type="button" class="li" role="checkbox" aria-checked="${st.ask}" data-act="hpDToggle" data-g="ask" data-fid="ld10-ask"><span class="hp-chk${st.ask ? ' on' : ''}" aria-hidden="true">${st.ask ? icon('check', 16) : ''}</span><span class="tx"><b>Nicht mehr fragen</b><small>Gilt für dieses Gerät</small></span></button>
      ${hpActs(hpBtn('ghost', 'hpLd10No', 'Bleiben', 'ld10-x', 'data-autofocus'), hpBtn('primary', 'hpLd10Go', 'Wechseln', 'ld10-ok'))}`;
  });
});
ACT.hpLd10Go = () => { const st = HPD.st; if (st.ask) HP.noAsk = true; closeSheet(); if (st.onConfirm) st.onConfirm(); };
ACT.hpLd10No = () => { if (HPD.st.ask) HP.noAsk = true; closeSheet(); };

/* ---------- LD-11 Aufbewahrung ändern ---------- */
registerLiveDialog('LD-11', (props) => {
  const r = SIM.recording(props.recordingId); const rule = r && r.rule ? SIM.rule(r.rule) : null;
  const st = { id: props.recordingId, keep: r ? hpKeepKey(r.keep) : 'forever', pin: !!(r && r.pin) };
  return hpDlg(st, () => {
    if (!r) return `<h2>Aufbewahren</h2>${hpNote('bad', 'Diese Aufnahme gibt es nicht mehr.')}${hpActs(hpBtn('primary', 'close', 'Schliessen', 'ld11-x'))}`;
    return `<h2>Aufbewahren</h2><p class="pad hp-dsub"><b>${esc(r.title)}</b>${r.sub ? ` · «${esc(r.sub)}»` : ''}</p><div class="hp-radios" role="radiogroup" aria-label="Aufbewahren">${[['forever', 'Bis du löschst'], ['7', '7 Tage ab Aufnahme'], ['14', '14 Tage ab Aufnahme'], ['30', '30 Tage ab Aufnahme'], ['90', '90 Tage ab Aufnahme']].map(([v, t]) => `<button type="button" class="li" role="radio" aria-checked="${st.keep === v}" data-act="hpDPick" data-g="keep" data-v="${v}" data-fid="ld11-${v}"><span class="hp-radio${st.keep === v ? ' on' : ''}" aria-hidden="true"></span><span class="tx"><b>${t}</b></span></button>`).join('')}</div>
      ${rule && rule.keepLast ? `<button type="button" class="li" role="checkbox" aria-checked="${st.pin}" data-act="hpDToggle" data-g="pin" data-fid="ld11-pin"><span class="hp-chk${st.pin ? ' on' : ''}" aria-hidden="true">${st.pin ? icon('check', 16) : ''}</span><span class="tx"><b>Diese Folge immer behalten</b><small>Zählt nicht zu den ${rule.keepLast}</small></span></button>` : ''}
      <p class="pad hint">Abgelaufene Aufnahmen löscht Slouchcat automatisch, weil du es so eingestellt hast.</p>${hpActs(hpBtn('ghost', 'close', 'Abbrechen', 'ld11-x'), hpBtn('primary', 'hpLd11Save', 'Speichern', 'ld11-ok'))}`;
  });
});
ACT.hpLd11Save = () => { const st = HPD.st; const r = SIM.recording(st.id); r.pin = st.pin; SIM.setKeep(st.id, hpKeepVal(st.keep, r.start)); closeSheet(); toast('Aufbewahrung gespeichert'); };

/* ============================================================ LS-11 Geplante Aufnahmen */
const hpWarnState = (rule) => {
  const ws = SIM.warningsFor(rule.id); const has = (k) => ws.find((w) => w.kind === k);
  if (has('channelGone')) return ['Warnung: Sender weg', has('channelGone')]; if (has('overlap')) return ['Warnung: Überschneidung', has('overlap')];
  if (has('quota')) return [SIM.quota().freeGB <= 0 ? 'Wird nicht aufgenommen: Kontingent' : 'Warnung: Platz im Speicher', has('quota')]; if (has('timeChanged')) return ['Zeit angepasst', has('timeChanged')];
  return null;
};
const hpWdText = (days) => { const n = { 1: 'Mo', 2: 'Di', 3: 'Mi', 4: 'Do', 5: 'Fr', 6: 'Sa', 0: 'So' }; const d = days.slice().sort((a, b) => ((a + 6) % 7) - ((b + 6) % 7)); if (d.length === 7) return 'täglich'; if (d.join() === '1,2,3,4,5') return 'Mo–Fr'; return d.map((x) => n[x]).join(', '); };
route('/live/scheduled/warnings', () => hpWarningsPage());
route('/live/scheduled', (p, q) => {
  if (HP.sch.key !== location.hash) { HP.sch.key = location.hash; HP.sch.tab = ['upcoming', 'series', 'recurring'].includes(q.tab) ? q.tab : 'upcoming'; }
  const S = HP.sch; const empty = SIM.mode() === 'empty'; const up = empty ? [] : SIM.upcoming(); const rules = empty ? [] : SIM.rules(); const t = SIM.now();
  const wn = up.filter((u) => SIM.warningsFor(u.rule.id).length).length;
  let body = hpState(5, 96);
  const tabs = `<div class="chips" role="tablist" aria-label="Ansicht">${[['upcoming', 'Termine'], ['series', 'Serien'], ['recurring', 'Wiederkehrend']].map(([k, l]) => `<button type="button" role="tab" class="chip${S.tab === k ? ' on' : ''}" aria-selected="${S.tab === k}" data-act="hpSchTab" data-k="${k}" data-fid="st-${k}">${l}</button>`).join('')}</div>`;
  const row = (id, sum, bodyHtml, acts) => { const o = !!S.open[id]; return `<li class="hp-xr"><button type="button" class="hp-xr-h" aria-expanded="${o}" data-act="hpSchOpen" data-id="${esc(id)}" data-fid="sx-${esc(id)}">${sum}<span class="hp-xr-c">${icon(o ? 'up' : 'down', 20)}</span></button>${o ? `<div class="hp-xr-b">${bodyHtml}</div>` : ''}<div class="hp-xr-a">${acts}</div></li>`; };
  if (!body) {
    if (S.tab === 'upcoming') {
      if (!up.length) body = hpEmptyBox('clock', 'Keine Aufnahmen geplant', '', `<a class="btn ghost" href="#/live/guide" data-fid="sch-toguide">Zum Programm</a>`);
      else {
        let day = null; let html = '';
        up.forEach((u) => {
          const d = LU.dayOf(u.start); if (d !== day) { if (day !== null) html += '</ul>'; day = d; html += `<div class="sec">${esc(fmt.dayRel(u.start) === fmt.day(u.start) ? fmt.dayLong(u.start) : fmt.dayRel(u.start) + ', ' + fmt.day(u.start))}</div><ul class="hp-plist">`; }
          const c = SIM.channel(u.ch); const ws = hpWarnState(u.rule); const run = u.start - u.rule.padBefore <= t; const soon = u.start - t > 0 && u.start - t < 60;
          const state = run ? 'Nimmt auf' : ws ? ws[0] : soon ? `Startet in ${Math.ceil(u.start - t)} Min.` : 'Geplant';
          const sum = `<span class="hp-xr-t"><b>${fmt.time(u.start)}–${fmt.time(u.end)}</b><small>${u.rule.padBefore || u.rule.padAfter ? `Vorlauf ${u.rule.padBefore} · Nachlauf ${u.rule.padAfter} Min.` : 'Ohne Vor- und Nachlauf'}</small></span><span class="hp-xr-m">${LU.logo(c)}<span><b>${esc(u.title)}</b><small>${esc(c ? c.name : u.ch)}${u.sub ? ' · ' + esc(u.sub) : ''}</small></span></span><span class="hp-xr-s"><span class="badge">${esc(LU.RULE_KIND[u.kind])}</span><small class="${ws ? 'hp-warn' : ''}">${ws ? icon('alert', 14) : ''}${esc(state)}</small></span>`;
          const bd = `<p>Aufbewahrung: ${esc(u.rule.keep && u.rule.keep.mode === 'days' ? u.rule.keep.days + ' Tage' : 'bis du löschst')}</p><p>Voraussichtliche Grösse: ${hpGBtext(u.end - u.start + (u.rule.padBefore || 0) + (u.rule.padAfter || 0), c || { bitrate: 8 })}</p><p>Herkunft: ${u.kind === 'series' ? `Serienaufnahme «${esc(u.rule.title)}»` : u.kind === 'recurring' ? `Wiederkehrend «${esc(u.rule.title)}»` : u.kind === 'manual' ? 'Manuell geplant' : 'Einzeln geplant'}</p>${SIM.warningsFor(u.rule.id).some((w) => w.kind === 'timeChanged') ? '<p>Zeit angepasst: neu 20:05–21:35 statt 20:05–21:40</p>' : ''}${ws ? `<p><button type="button" class="hp-lnk" data-act="hpWarnOpen" data-w="${ws[1].id}" data-fid="sw-${esc(u.id)}">${icon('alert', 14)} ${esc(ws[0])}: Einzelheiten</button></p>` : ''}`;
          const chg = u.kind === 'single' && u.program ? `data-act="hpSchEditSingle" data-p="${u.program}"` : u.kind === 'series' ? `data-act="hpSchEditSeries" data-p="${u.program}" data-r="${u.rule.id}"` : `data-act="hpSchEditManual" data-r="${u.rule.id}"`;
          html += row(u.id, sum, bd, `<button type="button" class="btn ghost sm" ${chg} aria-label="${esc(u.title)} ändern" data-fid="sc-${esc(u.id)}">Ändern</button><button type="button" class="btn danger sm" data-act="hpSchCancel" data-id="${esc(u.id)}" aria-label="${esc(u.title)} abbrechen" data-fid="sx2-${esc(u.id)}">Abbrechen</button>`);
        });
        body = html + '</ul>';
      }
    } else if (S.tab === 'series') {
      const sr = rules.filter((r) => r.kind === 'series');
      body = !sr.length ? hpEmptyBox('record', 'Keine Serienaufnahmen', 'Öffne eine Sendung und wähle ‹Serie aufnehmen›.') : `<ul class="hp-plist">${sr.map((r) => {
        const n = up.filter((u) => u.rule.id === r.id).length; const nr = SIM.recordings().filter((x) => x.rule === r.id).length;
        const chn = r.channels === 'all' ? 'alle Sender' : r.channels.map((id) => (SIM.channel(id) || {}).name).join(', ');
        const sum = `<span class="hp-xr-m"><span class="hp-xr-ic">${icon('recordSeries', 22)}</span><span><b>${esc(r.title)}</b><small>${esc(chn)}${r.onlyNew ? ' · nur Erstausstrahlungen' : ''}${r.noDup ? ' · Folgen nicht doppelt' : ''}${r.keepLast ? ` · behalte ${r.keepLast}` : ''}</small><small>${n} ${n === 1 ? 'Termin' : 'Termine'} geplant · ${nr} Aufnahmen</small></span></span>`;
        const nx = up.filter((u) => u.rule.id === r.id).slice(0, 3);
        const bd = `${nx.length ? `<p><b>Nächste Termine</b></p>${nx.map((u) => `<p>${esc(fmt.dayRel(u.start))} ${fmt.time(u.start)} · ${esc(SIM.channel(u.ch).name)}${u.sub ? ' · «' + esc(u.sub) + '»' : ''}</p>`).join('')}` : '<p>Kein Termin im Programm.</p>'}${(r.skipped || []).length ? `<p><b>Übersprungen</b></p>${r.skipped.map(([pid, why]) => { const p2 = SIM.program(pid); return p2 ? `<p>${esc(fmt.day(p2.start))} ${fmt.time(p2.start)} ${esc(SIM.channel(p2.ch).name)}: ${esc(why)}</p>` : ''; }).join('')}` : ''}`;
        return row('r-' + r.id, sum, bd, `<button type="button" class="btn ghost sm" data-act="hpSchEditSeries" data-r="${r.id}" aria-label="${esc(r.title)} ändern" data-fid="sc-${r.id}">Ändern</button><button type="button" class="btn danger sm" data-act="hpSchEnd" data-r="${r.id}" aria-label="${esc(r.title)} beenden" data-fid="sx2-${r.id}">Beenden</button>`);
      }).join('')}</ul>`;
    } else {
      const rc = rules.filter((r) => r.kind === 'recurring'); const mn = rules.filter((r) => r.kind === 'manual');
      const mk = (r) => {
        const c = SIM.channel(r.ch); const nx = up.find((u) => u.rule.id === r.id); const ws = hpWarnState(r);
        const sum = r.kind === 'recurring' ? `<span class="hp-xr-m">${LU.logo(c)}<span><b>${esc(r.title)}</b><small>${esc(c.name)} · ${hpWdText(r.weekdays)} ${r.from}–${r.to}${r.keepLast ? ` · behalte ${r.keepLast}` : ''}</small><small>${nx ? `Nächster Termin ${fmt.dayRel(nx.start)} ${fmt.day(nx.start)}` : 'Kein Termin mehr'}</small></span></span>`
          : `<span class="hp-xr-m">${LU.logo(c)}<span><b>${esc(r.title)}</b><small>${esc(c.name)} · ${fmt.day(r.start)} ${fmt.range(r.start, r.end)}</small>${ws ? `<small class="hp-warn">${icon('alert', 14)}${esc(ws[0])}</small>` : ''}</span></span>`;
        const bd = `<p>Aufbewahrung: ${esc(r.keep && r.keep.mode === 'days' ? r.keep.days + ' Tage' : 'bis du löschst')}</p>${ws ? `<p><button type="button" class="hp-lnk" data-act="hpWarnOpen" data-w="${ws[1].id}" data-fid="sw-${r.id}">${icon('alert', 14)} ${esc(ws[0])}: Einzelheiten</button></p>` : ''}`;
        return row('m-' + r.id, sum, bd, `<button type="button" class="btn ghost sm" data-act="hpSchEditManual" data-r="${r.id}" aria-label="${esc(r.title)} ändern" data-fid="sc-${r.id}">Ändern</button><button type="button" class="btn danger sm" data-act="hpSchRule" data-r="${r.id}" aria-label="${esc(r.title)} abbrechen" data-fid="sx2-${r.id}">${r.kind === 'recurring' ? 'Beenden' : 'Abbrechen'}</button>`);
      };
      body = !rc.length && !mn.length ? hpEmptyBox('clock', 'Keine wiederkehrenden Aufnahmen', '', `<button type="button" class="btn ghost" data-act="hpNewRec" data-fid="sch-new2">Neue Aufnahme</button>`)
        : `${rc.length ? `<ul class="hp-plist">${rc.map(mk).join('')}</ul>` : ''}${mn.length ? `<div class="sec">Manuell</div><ul class="hp-plist">${mn.map(mk).join('')}</ul>` : ''}`;
    }
  }
  const top = `<div class="hp-tools"><button type="button" class="btn secondary sm" data-act="hpNewRec" data-fid="sch-new">${icon('plus', 18)}Neue Aufnahme</button><span class="hp-sp"></span><a class="btn ghost sm" href="#/live/recordings" data-fid="sch-rec">Meine Aufnahmen</a></div>${wn ? `<a class="note warn hp-link" href="#/live/scheduled/warnings" data-fid="sch-warn">${icon('alert', 20)}<div><b>${wn} ${wn === 1 ? 'Termin hat' : 'Termine mit'} Warnungen</b><br>Warnungen ansehen</div></a>` : ''}`;
  return { nav: '', html: livePage({ tab: 'recordings', title: 'Geplant', html: `${top}${tabs}${body}` }) };
});
ACT.hpSchTab = (d) => { HP.sch.tab = d.k; render(); };
ACT.hpSchOpen = (d) => { HP.sch.open[d.id] = !HP.sch.open[d.id]; render(); };
ACT.hpSchEditSingle = (d) => LiveDlg.planRecording(d.p);
ACT.hpSchEditSeries = (d) => LiveDlg.seriesRecording(d.p, d.r);
ACT.hpSchEditManual = (d) => LiveDlg.manualRecording({ ruleId: d.r });
ACT.hpSchCancel = (d) => LiveDlg.confirmDelete({ kind: 'occurrence', ids: [d.id] });
ACT.hpSchEnd = (d) => LiveDlg.confirmDelete({ kind: 'series', ids: [d.r] });
ACT.hpSchRule = (d) => LiveDlg.confirmDelete({ kind: 'rule', ids: [d.r] });
ACT.hpWarnOpen = (d) => LiveDlg.warning(d.w);

/* ============================================================ LS-12 Konflikte und Warnungen */
function hpWarningsPage() {
  let body = hpState(3, 110); const ws = SIM.mode() === 'empty' ? [] : SIM.warnings();
  if (!body) body = !ws.length ? hpEmptyBox('check', 'Alles in Ordnung', 'Keine geplante Aufnahme hat eine Warnung.') : `<div class="hp-wcards">${ws.map((w) => `<div class="card hp-wc ${w.severity}"><div class="hp-wc-h">${LU.badge(w.severity === 'info' ? 'scheduled' : 'warning', { text: true, label: HP_WKIND[w.kind] || 'Warnung' })}</div><p>${esc(w.text)}</p><p class="hint">Betrifft: ${w.rules.map((id) => esc((SIM.rule(id) || {}).title || '')).join(', ')}</p><div class="hp-btnrow">${hpWarnActions(w)}</div></div>`).join('')}</div>`;
  return { nav: '', html: `${hpBackBar('Konflikte und Warnungen')}<p class="pad hint" style="margin-bottom:8px">Slouchcat hat keine Tuner und kein Anbieterlimit. Hier stehen Warnungen mit Folgen für geplante Aufnahmen.</p>${body}<p class="lh-sample">${esc(LU.TX.sample)}</p>` };
}

/* ============================================================ LS-13 Erinnerungen */
route('/live/reminders', () => {
  let body = hpState(4, 84); const list = SIM.mode() === 'empty' ? [] : SIM.reminders();
  if (!body) {
    if (!list.length) body = hpEmptyBox('reminder', 'Keine Erinnerungen', 'Öffne eine Sendung und wähle ‹Erinnern›.', `<a class="btn ghost" href="#/live/guide" data-fid="rem-toguide">Zum Programm</a>`);
    else { let day = null; let html = ''; list.forEach((r) => { const p = SIM.program(r.program); const c = SIM.channel(p.ch); const d = LU.dayOf(p.start); if (d !== day) { if (day !== null) html += '</ul>'; day = d; html += `<div class="sec">${esc(fmt.dayRel(p.start) === fmt.day(p.start) ? fmt.dayLong(p.start) : fmt.dayRel(p.start) + ', ' + fmt.day(p.start))}</div><ul class="hp-plist">`; }
      html += `<li class="hp-pr"><a class="hp-pr-main" href="#/live/programs/${p.id}" data-fid="rm-${r.id}"><span class="hp-pr-time"><b>${fmt.time(p.start)}</b><small>${fmt.time(p.end)}</small></span><span class="hp-pr-body"><span class="hp-pr-ch">${LU.logo(c)}<small>${c.no} ${esc(c.name)}</small></span><b class="hp-pr-t">${esc(p.title)}</b>${p.sub ? `<small>${esc(p.sub)}</small>` : ''}<span class="hp-pr-meta"><small>${r.lead ? `Erinnert ${r.lead} Min. vorher` : 'Erinnert zum Beginn'}</small></span></span></a><span class="hp-pr-act"><button type="button" class="ib" data-act="hpRem" data-p="${p.id}" aria-label="Erinnerung für ${esc(p.title)} ändern" data-fid="re-${r.id}">${icon('settings', 22)}</button><button type="button" class="ib" data-act="hpRemDel" data-p="${p.id}" aria-label="Erinnerung für ${esc(p.title)} entfernen" data-fid="rd-${r.id}">${icon('trash', 22)}</button></span></li>`; }); body = html + '</ul>'; }
  }
  return { nav: '', html: livePage({ tab: 'reminders', title: 'Erinnerungen', html: body }) };
});
ACT.hpRemDel = (d) => { const p = SIM.program(d.p); SIM.removeReminder(d.p); toast(`Erinnerung für «${p.title}» entfernt`); };
/* reminder band: appears when a reminder is due, stays until you click */
const hpShown = new Set();
function hpRemCheck(list) {
  const t = SIM.now(); if (!SIM.rights().liveTv) return;
  (list || SIM.reminders()).forEach((r) => {
    const p = SIM.program(r.program); if (!p || hpShown.has(r.id) || p.start - r.lead > t || p.end <= t) return; hpShown.add(r.id);
    const c = SIM.channel(p.ch); const band = document.createElement('div'); band.className = 'hp-rband'; band.setAttribute('role', 'status');
    band.innerHTML = `${icon('reminder', 22)}<span><b>${esc(p.title)}</b> ${p.start > t ? `beginnt in ${Math.ceil(p.start - t)} Min.` : 'hat begonnen'} auf ${esc(c.name)}</span><button type="button" class="btn primary sm" data-act="hpBandGo" data-ch="${c.id}" data-fid="band-go">Einschalten</button><button type="button" class="btn ghost sm" data-act="hpBandX" data-fid="band-x">Schliessen</button>`;
    $('#device').appendChild(band);
  });
}
ACT.hpBandGo = (d, el) => { el.closest('.hp-rband').remove(); const pl = SIM.player(); if (pl && pl.ch !== d.ch) hpSwitch(d.ch); else if (!pl) liveTune(d.ch); };
ACT.hpBandX = (d, el) => el.closest('.hp-rband').remove();
SIM.on((t) => { if (t === 'minute') hpRemCheck(); });
/* a jump of the mockup clock («+5 Min.», «+2 Std.») may pass a reminder: look at the list from before the jump */
document.addEventListener('click', (e) => { const a = e.target.closest && e.target.closest('[data-lvm="adv"]'); if (!a) return; const pre = SIM.reminders().slice(); setTimeout(() => hpRemCheck(pre), 0); }, true);
window.addEventListener('hashchange', () => { /* the band stays across pages */ });

/* ============================================================ LS-14 Aufnahmen meiner Profile */
function hpProfilesPage() {
  const kids = SIM.childProfiles(); let body = hpState(3, 80); const empty = SIM.mode() === 'empty';
  if (!body) {
    const data = kids.map((k) => ({ k, recs: empty ? [] : SIM.recordings(k.id), up: SIM.upcoming(k.id) }));
    const none = data.every((x) => !x.recs.length);
    body = !kids.length || none ? hpEmptyBox('record', 'Keine Aufnahmen', `Die Profile ${kids.map((k) => k.name).join(' und ')} haben keine Aufnahmen.`) : data.map(({ k, recs, up }) => {
      const o = !!HP.prof.open['k-' + k.id]; const gb = recs.reduce((a, r) => a + hpRecSize(r), 0);
      return `<section class="hp-band"><button type="button" class="hp-band-h" aria-expanded="${o}" data-act="hpProfOpen" data-id="k-${k.id}" data-fid="kp-${k.id}"><span class="avatar" style="background:var(--av-${k.accent})">${esc(k.name[0])}</span><span class="hp-band-t"><b>${esc(k.name)}</b><small>${recs.length} ${recs.length === 1 ? 'Aufnahme' : 'Aufnahmen'} · ${fmt.gb(gb)} · ${up.length} geplant</small></span>${icon(o ? 'up' : 'down', 20)}</button>
        ${o ? `<div class="hp-band-b">${recs.length ? `<ul class="hp-plist">${recs.map((r) => hpRecRow(r, { del: true })).join('')}</ul>` : '<p class="pad hint">Keine Aufnahmen.</p>'}${up.length ? `<div class="sec">Geplant</div>${up.map((u) => `<p class="pad hp-dsub"><b>${esc(u.title)}</b><br><small>${esc(SIM.channel(u.ch).name)} · ${esc(fmt.dayRel(u.start))} ${fmt.range(u.start, u.end)}</small></p>`).join('')}` : ''}</div>` : ''}</section>`;
    }).join('');
  }
  return { nav: '', html: `${hpBackBar('Aufnahmen meiner Profile')}<p class="pad hint" style="margin-bottom:10px">Hier siehst du die Aufnahmen der Profile mit Altersgrenze. Aufnahmen von Profilen ohne Altersgrenze bleiben privat.</p>${body}<p class="lh-sample">${esc(LU.TX.sample)}</p>` };
}
ACT.hpProfOpen = (d) => { HP.prof.open[d.id] = !HP.prof.open[d.id]; render(); };
ACT.hpKidDel = (d) => LiveDlg.confirmDelete({ kind: 'childRecording', ids: [d.id] });

/* ============================================================ LS-21 Profile verwalten: Block «Live-TV» */
const HP_RIGHTS = [['liveTv', 'Live-TV', 'Fernsehen schauen und das Programm sehen.'], ['recordings', 'Aufnahmen', 'Sendungen aufnehmen. Aufnahmen belegen Speicher des Kontos.'], ['timeshift', 'Timeshift', 'Live-TV anhalten und zurückspulen, bis zum eigenen Einschalten.']];
route('/settings/profiles', () => {
  const me0 = SIM.profile(); const acc = SIM.rawRights('account');
  const list = LIVE.profiles.map((p) => {
    const raw = SIM.rawRights('profile', p.id); const o = !!HP.prof.open['p-' + p.id]; const onoff = (v) => (v ? 'an' : 'aus');
    const sum = `Live-TV: ${onoff(raw.liveTv)} · Aufnahmen: ${onoff(raw.recordings)} · Timeshift: ${onoff(raw.timeshift)}`;
    const sws = HP_RIGHTS.map(([k, t, help]) => {
      const capped = !acc[k]; const dep = k !== 'liveTv' && !raw.liveTv; const dis = capped || dep || !me0.primary;
      const sub = capped ? LU.TX.capped : dep ? 'Zuerst Live-TV einschalten.' : help;
      return `<button type="button" class="li" role="switch" aria-checked="${!!raw[k] && !capped}" ${dis ? 'aria-disabled="true"' : ''} data-act="hpPSw" data-p="${p.id}" data-k="${k}" data-fid="ps-${p.id}-${k}"><span class="tx"><b>${t}</b><small class="hp-wrapsm">${esc(sub)}</small></span><span class="sw" aria-checked="${!!raw[k] && !capped}"></span></button>`;
    }).join('');
    return `<section class="hp-band"><div class="hp-prow"><span class="avatar" style="background:var(--av-${p.accent})">${esc(p.name[0])}</span><span class="hp-band-t"><b>${esc(p.name)}</b><small>${p.primary ? 'Hauptprofil' : p.age ? 'Altersgrenze ab ' + p.age : 'Ohne Altersgrenze'}</small></span></div>
      <button type="button" class="hp-band-h" aria-expanded="${o}" data-act="hpProfOpen" data-id="p-${p.id}" data-fid="pb-${p.id}"><span class="hp-band-t"><b>Live-TV</b><small>${esc(sum)}</small></span>${icon(o ? 'up' : 'down', 20)}</button>
      ${o ? `<div class="hp-band-b">${sws}${p.age != null ? `<div class="note" style="margin-top:8px">${icon('info', 20)}<div>${esc(LU.TX.noAgeCheck)}</div></div>` : ''}</div>` : ''}</section>`;
  }).join('');
  return { nav: '', html: `${bar('Profile verwalten', { back: true })}${me0.primary ? '' : `<div class="note">${icon('info', 20)}<div>Nur das Hauptprofil ändert Profile.</div></div>`}<p class="pad hint" style="margin-bottom:10px">Konto «${esc(LIVE.account.name)}»</p>${list}<p class="lh-sample">${esc(LU.TX.sample)}</p>` };
});
ACT.hpPSw = (d) => {
  const me0 = SIM.profile(); if (!me0.primary) return toast('Nur das Hauptprofil kann Profile ändern.'); const acc = SIM.rawRights('account'); const raw = SIM.rawRights('profile', d.p); const p = LIVE.profiles.find((x) => x.id === d.p);
  if (!acc[d.k]) return toast(LU.TX.capped); if (d.k !== 'liveTv' && !raw.liveTv) return toast('Zuerst Live-TV einschalten.');
  const on = !raw[d.k];
  if (d.k === 'liveTv' && on && p.age != null) { sheet(`<h2>Live-TV für ${esc(p.name)} einschalten?</h2><p class="pad">${esc(p.name)} sieht dann alle Sender und Sendungen ohne Altersprüfung.</p><div class="acts"><button type="button" class="btn ghost" data-act="close" data-fid="pc-x" autofocus>Abbrechen</button><button type="button" class="btn primary" data-act="hpPConfirm" data-p="${p.id}" data-fid="pc-ok">Einschalten</button></div>`, { center: true }); return; }
  SIM.setRight('profile', d.k, on, d.p); toast(`${HP_RIGHTS.find((x) => x[0] === d.k)[1]} für ${p.name}: ${on ? 'an' : 'aus'}`);
};
ACT.hpPConfirm = (d) => { closeSheet(); SIM.setRight('profile', 'liveTv', true, d.p); toast(`Live-TV für ${LIVE.profiles.find((x) => x.id === d.p).name}: an`); };

/* ============================================================ LS-22 Benachrichtigungen einstellen */
ui.hpn = ui.hpn || { rem: true, remApp: true, done: true, fail: true, rej: true };
route('/settings/notifications', () => {
  const r = SIM.rights();
  return { nav: '', html: `${bar('Benachrichtigungen', { back: true })}
    <p class="pad hint" style="margin-bottom:6px">Wann Slouchcat dir etwas meldet. Die Meldungen erscheinen in der Glocke und, wenn du es erlaubst, als Mitteilung auf deinen Geräten.</p>
    ${r.liveTv ? `<div class="sec">Live-TV</div>${sw('ui', 'hpn.rem', 'Sendungserinnerung', 'Vor dem Beginn einer Sendung, an die du dich erinnern lässt.')}
    ${r.recordings ? `${sw('ui', 'hpn.done', 'Aufnahme fertig', 'Wenn eine Aufnahme vollständig ist.')}${sw('ui', 'hpn.fail', 'Aufnahme fehlgeschlagen oder unvollständig', 'Wenn der Sender nicht erreichbar war.')}${sw('ui', 'hpn.rej', 'Aufnahme nicht möglich', 'Wenn der Speicher des Kontos beim Start voll war. Das Hauptprofil bekommt die Meldung ebenfalls.')}` : ''}` : ''}
    <div class="sec">Weitere</div>${sw('ui', 'hpn.other1', 'Neu in der Bibliothek', 'Neue Filme und Folgen.')}${sw('ui', 'hpn.other2', 'Wünsche', 'Wenn ein gewünschter Titel verfügbar ist.')}
    <p class="lh-sample">${esc(LU.TX.sample)}</p>` };
});

/* ============================================================ LS-23 Benachrichtigungen (Glocke) */
route('/notifications', () => {
  const r = SIM.rights(); const items = [];
  if (r.liveTv) SIM.reminders().slice(0, 2).forEach((x) => { const p = SIM.program(x.program); items.push({ ic: 'reminder', t: `${p.title} beginnt in ${x.lead || 0} Min. auf ${SIM.channel(p.ch).name}`, s: `Erinnerung · ${fmt.dayRel(p.start)} ${fmt.time(p.start)}`, h: `#/live/programs/${p.id}`, at: p.start - x.lead });
    });
  if (r.recordings) {
    SIM.recordings().forEach((x) => {
      if (x.state === 'done' && x.start > -3 * 1440) items.push({ ic: 'record', t: `Aufnahme fertig: ${x.title}`, s: `${SIM.channel(x.ch).name} · ${fmt.ago(x.end)}`, h: `#/live/recordings/${x.id}`, at: x.end });
      if (x.state === 'incomplete') items.push({ ic: 'alert', t: `Aufnahme unvollständig: ${x.title}`, s: `${x.gaps.length} Lücken · ${fmt.ago(x.end)}`, h: `#/live/recordings/${x.id}`, at: x.end });
      if (x.state === 'failed') items.push({ ic: 'alert', t: `Aufnahme fehlgeschlagen: ${x.title}`, s: `${LU.REASON[x.reason]} · ${fmt.ago(x.end)}`, h: '#/live/recordings', at: x.end });
      if (x.state === 'rejected') items.push({ ic: 'alert', t: `Aufnahme nicht möglich: ${x.title}`, s: `${LU.REASON[x.reason]} · ${fmt.ago(x.start)}`, h: '#/live/recordings', at: x.start });
    });
    if (SIM.profile().primary) SIM.childProfiles().forEach((k) => SIM.recordings(k.id).filter((x) => x.state === 'done').slice(0, 1).forEach((x) => items.push({ ic: 'record', t: `${k.name}: Aufnahme fertig, ${x.title}`, s: `${SIM.channel(x.ch).name} · ${fmt.ago(x.end)}`, h: '#/live/recordings/profiles', at: x.end })));
  }
  items.sort((a, b) => b.at - a.at);
  const body = SIM.mode() === 'loading' ? hpState(5, 64) : SIM.mode() === 'error' ? hpState() : !items.length ? hpEmptyBox('bell', 'Keine Meldungen', 'Neue Meldungen erscheinen hier.') : `<ul>${items.map((i, k) => `<li><a class="li" href="${i.h}" data-fid="nt-${k}"><span class="lead">${icon(i.ic, 22)}</span><span class="tx"><b>${esc(i.t)}</b><small>${esc(i.s)}</small></span><span class="end">${icon('right', 18)}</span></a></li>`).join('')}</ul>`;
  return { nav: '', html: `${bar('Benachrichtigungen', { back: true })}${body}<p class="lh-sample">${esc(LU.TX.sample)}</p>` };
});

/* ============================================================ LS-24 Suche: Verweis ins TV-Programm */
HP.q = '';
document.addEventListener('input', (e) => { const el = e.target.closest('[data-hpmq]'); if (!el) return; HP.q = el.value; render(); const n = $('[data-hpmq]'); if (n) { n.focus(); n.setSelectionRange(n.value.length, n.value.length); } });
route('/search', (p, q) => {
  if (q.q != null && HP.qKey !== location.hash) { HP.q = q.q; HP.qKey = location.hash; }
  const r = SIM.rights(); const t = HP.q.trim();
  return { nav: 'search', html: `${bar('Suche', { back: false })}<div class="hp-tools"><label class="lh-search"><span class="vh">Suchen</span>${icon('search', 20)}<input class="inp" type="search" placeholder="Filme, Serien, Podcasts …" value="${esc(HP.q)}" data-hpmq data-fid="mq" aria-label="Suchen"></label></div>
    ${t ? `<div class="sec">Bibliothek</div><div class="note">${icon('info', 20)}<div>Treffer aus den Bibliotheken zeigt dieses Mockup nicht. Die Suche bleibt unverändert, nur das Live-TV kommt dazu.</div></div>${r.liveTv ? `<div class="sec">Fernsehen</div><a class="li" href="#/live/guide/search?q=${encodeURIComponent(t)}" data-fid="mq-tv"><span class="lead">${icon('live', 22)}</span><span class="tx"><b>Im TV-Programm suchen: ‹${esc(t)}›</b><small>Zeigt Sendungen, keine Aufnahmen</small></span><span class="end">${icon('right', 18)}</span></a>` : ''}` : hpEmptyBox('search', 'Suchen', 'Gib einen Begriff ein.')}<p class="lh-sample">${esc(LU.TX.sample)}</p>` };
});

/* ============================================================ LS-25 Ton-Vorrang (Webradio) */
route('/radio', () => {
  const st = [['Radio SRF 1', 'Schweiz · Pop'], ['Radio SRF 3', 'Schweiz · Pop und Rock'], ['Radio Swiss Classic', 'Schweiz · Klassik']];
  return { nav: '', html: `${bar('Webradio', { back: false })}<div class="note">${icon('info', 20)}<div>Live-TV ist eine dritte Tonquelle: Einschalten von Live-TV beendet das Radio. Verlassen des Live-Players hinterlässt keinen Mini-Player.</div></div>
    ${HP.radio ? `<div class="card hp-run"><div class="hp-run-h">${icon('radio', 20)}<b>${esc(HP.radio)}</b></div><p class="hint">Läuft</p><div class="hp-btnrow"><button type="button" class="btn ghost sm" data-act="hpRadioStop" data-fid="rad-stop">${icon('pause', 18)}Beenden</button></div></div>` : ''}
    <ul>${st.map((s, i) => `<li><button type="button" class="li" data-act="hpRadioPlay" data-n="${esc(s[0])}" data-fid="rad-${i}"><span class="lead">${icon('radio', 22)}</span><span class="tx"><b>${esc(s[0])}</b><small>${esc(s[1])}</small></span><span class="end">${icon('play', 18)}</span></button></li>`).join('')}</ul>
    ${SIM.rights().liveTv ? `<p class="pad" style="margin-top:14px"><a class="btn secondary" href="#/live/channels" data-fid="rad-tv">${icon('live', 18)}Zu Live-TV</a></p>` : ''}<p class="lh-sample">${esc(LU.TX.sample)}</p>` };
});
ACT.hpRadioPlay = (d) => { HP.radio = d.n; toast(`${d.n} läuft`); render(); };
ACT.hpRadioStop = () => { HP.radio = null; render(); };
SIM.on((t) => { if (t === 'change' && HP.radio && SIM.player()) { HP.radio = null; toast('Das Radio wurde beendet, weil Live-TV läuft.'); } });

/* ============================================================ LS-26 Startseite (unverändert) */
route('/', () => unchangedPage('/'));
