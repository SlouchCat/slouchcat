/* Web Zuschauer A (part 2): LS-06 Sendungsdetails, LS-15 Senderprogramm and the changed existing pages
   LS-21 Profilformular (Block «Live-TV»), LS-22 Benachrichtigungen einstellen, LS-23 Benachrichtigungen (Glocke),
   LS-24 Suche, LS-25 Ton-Vorrang und Mini-Player (Radio), LS-26 Startseite (unverändert).
   Uses helpers of viewer-a.js (prefix «va»). 14-oberflaeche.md 1.2, 1.3, 3.6, 3.15. */
'use strict';

/* ======================================================================================
   LS-06 Sendungsdetails
   ====================================================================================== */
function vaRound(o) {
  const cls = `round${o.on ? ' on' : ''}${o.hi ? ' hi' : ''}`;
  const inner = `<span class="c">${icon(o.icon, 20)}</span>${esc(o.label)}`;
  if (o.href) return `<a class="${cls}" href="${o.href}" data-vt="1" tabindex="-1" data-fid="${o.fid}">${inner}</a>`;
  return `<button type="button" class="${cls}" data-act="${o.act}" ${vaAttrs(o.data)} ${o.pressed != null ? `aria-pressed="${o.pressed}"` : ''} data-vt="1" tabindex="-1" data-fid="${o.fid}">${inner}</button>`;
}
function vaRounds(p) {
  const t = SIM.now(); const r = SIM.rights(); const c = SIM.channel(p.ch); const past = p.end <= t; const running = p.start <= t && p.end > t;
  const list = [];
  if (running && SIM.chState(c) !== 'gone') list.push(vaRound({ icon: 'play', label: 'Einschalten', hi: true, act: 'tune', data: { ch: c.id }, fid: 'ls06:tune' }));
  if (r.recordings && !past) {
    const rs = SIM.recState(p.id);
    if (rs === 'recording') {
      const rec = SIM.runningOn(p.ch);
      const items = [{ label: 'Aufnahme beenden', act: 'vaStopRec', data: { id: rec ? rec.id : '' }, icon: 'x', danger: true }, { label: 'Zur Aufnahme', href: `#/live/recordings/${rec ? rec.id : ''}`, icon: 'record' }];
      list.push(popMenu('va:rec:' + p.id, `<span class="c">${icon('record', 20)}</span>${esc(LU.TX.recording)}`, items, { cls: 'round on va-roundbtn', aria: `Nimmt auf: Aufnahme von «${p.title}», Menü` }).replace('data-act="menu"', 'data-act="menu" data-vt="1" tabindex="-1"'));
    } else list.push(vaRound({ icon: rs ? 'record' : 'record', label: rs ? LU.TX.scheduled : LU.TX.record, on: !!rs, act: 'vaRec', data: { id: p.id }, pressed: !!rs, fid: 'ls06:rec' }));
    if (vaIsSeries(p)) { const sr = vaSeriesRule(p); list.push(vaRound({ icon: 'recordSeries', label: sr ? LU.TX.series : 'Serie aufnehmen', on: !!sr, act: 'vaSeries', data: { id: p.id }, pressed: !!sr, fid: 'ls06:series' })); }
  }
  if (!past && p.start > t) { const rm = SIM.reminderFor(p.id); list.push(vaRound({ icon: 'reminder', label: rm ? `Erinnerung um ${fmt.time(p.start - rm.lead)}` : 'Erinnern', on: !!rm, act: 'vaRem', data: { id: p.id }, pressed: !!rm, fid: 'ls06:rem' })); }
  list.push(vaRound({ icon: 'calendar', label: 'Senderprogramm', href: `#/live/guide/channel/${c.id}`, fid: 'ls06:chan' }));
  return `<div class="rounds va-rounds" role="toolbar" aria-label="${esc('Aktionen für ' + p.title)}" data-vtool>${list.join('').replace('tabindex="-1"', 'tabindex="0"')}</div>`;
}
ACT.vaSeries = (d) => { const p = SIM.program(d.id); if (!p) return; const sr = vaSeriesRule(p); LiveDlg.seriesRecording(p.id, sr ? sr.id : undefined); };
ACT.vaStopRec = (d) => { if (d.id) LiveDlg.confirmDelete({ kind: 'running', ids: [d.id] }); };

route('/live/programs/:id', (pm) => {
  const p = SIM.program(pm.id);
  if (!p || !SIM.channel(p.ch) || SIM.epgOf(p.ch) === 'none' || SIM.chState(p.ch) === 'gone') {
    return vaNotFound('Nicht gefunden', 'Diese Sendung steht nicht mehr im Programm.', `<a class="btn secondary" href="${p && SIM.channel(p.ch) ? '#/live/guide/channel/' + p.ch : '#/live/guide'}" data-fid="ls06:nf">${icon('calendar', 18)}Zum Senderprogramm</a>`);
  }
  const c = SIM.channel(p.ch); const t = SIM.now(); const r = SIM.rights(); const past = p.end <= t; const running = p.start <= t && p.end > t;
  const back = `<button type="button" class="btn ghost" data-act="back" data-fid="ls06:back">${icon('arrowL', 18)}Zurück</button>`;
  let body = vaMode(`<div class="va-detskel"><span class="lv-skel" style="aspect-ratio:16/9"></span><div class="stack"><span class="lv-skel" style="height:22px;width:60%"></span><span class="lv-skel" style="height:44px;width:40%"></span><span class="lv-skel" style="height:90px"></span></div></div>`, 'Die Sendung konnte nicht geladen werden.');
  if (!body) {
    const rec = r.recordings && past ? SIM.recordings().find((x) => x.ch === p.ch && x.start < p.end && x.end > p.start && x.state !== 'rejected' && x.state !== 'failed') : null;
    const status = running ? `<div class="va-status">${LU.progress(p.start, p.end, { label: `Fortschritt von ${p.title}` })}<span><b>Läuft</b> · ${LU.nowText('left', p.end)}</span></div>`
      : past ? `<p class="va-status past">${esc(LU.TX.past)}${rec ? ` <a class="lnk" href="#/live/recordings/${rec.id}" data-fid="ls06:myrec">Deine Aufnahme ansehen</a>` : ''}</p>`
        : `<p class="va-status">Beginnt ${fmt.when(p.start)} · in ${fmt.dur(Math.max(1, p.start - t))}</p>`;
    const chips = [`<span class="chip info">${esc(p.genre)}</span>`].concat((p.flags || []).map((f) => `<span class="chip info">${esc(f)}</span>`), p.repeat ? ['<span class="chip info">Wiederholung</span>'] : []).join('');
    const long = p.desc && p.desc.length > 200; const openD = isOpen('va-desc:' + p.id);
    const desc = p.desc ? `<div class="va-descw"><p class="va-desc${long && !openD ? ' clamp' : ''}">${esc(p.desc)}</p>${long ? `<button type="button" class="lnk" data-act="open" data-id="va-desc:${p.id}" aria-expanded="${openD}" data-fid="ls06:more">${openD ? 'Weniger' : 'Mehr'}</button>` : ''}</div>` : '';
    const facts = [p.season && p.episode ? ['Folge', `Staffel ${p.season}, Folge ${p.episode}`] : null, p.year ? ['Jahr', p.year] : null, p.country ? ['Land', p.country] : null, p.director ? ['Regie', p.director] : null, p.cast && p.cast.length ? ['Mitwirkende', p.cast.join(', ')] : null].filter(Boolean);
    const more = LIVE.programs.filter((x) => x.id !== p.id && x.start !== p.start && x.end > t && x.start < t + 7 * 1440 && ((p.series && x.series === p.series) || x.title === p.title) && SIM.epgOf(x.ch) !== 'none').sort((a, b) => a.start - b.start).slice(0, 8);
    const bad = SIM.chState(c) === 'disturbed' && running ? notice('warn', `${esc(c.name)}: ${esc(LU.TX.unavailable)}`, 'Einschalten ist trotzdem möglich. Der Player zeigt dann den Zustand.') : '';
    body = `${bad}<div class="va-det"><div class="va-detpic">${LU.picture(p)}</div><div class="va-detinfo">
      <p class="va-meta"><a class="va-metach" href="#/live/guide/channel/${c.id}" data-fid="ls06:ch">${LU.logo(c)}<b>${esc(c.name)}</b></a><span>${fmt.dayRel(p.start)}, ${fmt.range(p.start, p.end)}</span><span>${fmt.dur(p.end - p.start)}</span></p>
      <div class="va-chips">${chips}</div>${status}${vaRounds(p)}${desc}</div></div>
      ${facts.length ? `<section class="va-sec" aria-labelledby="ls06-facts"><h2 class="st" id="ls06-facts">Angaben</h2><dl class="va-facts">${facts.map(([k, v]) => `<div><dt>${esc(k)}</dt><dd>${esc(String(v))}</dd></div>`).join('')}</dl></section>` : ''}
      ${more.length ? `<section class="va-sec" aria-labelledby="ls06-more"><h2 class="st" id="ls06-more">Weitere Termine</h2>${vaList('ls06', more.map((x) => vaProgRow(x, { day: true })),'Weitere Termine, Pfeiltasten wechseln')}</section>` : ''}`;
  }
  return { type: 'viewer', nav: 'live', title: p.title, html: livePage({ tab: 'guide', title: esc(p.title), sub: p.sub ? `«${esc(p.sub)}»` : '', actions: back, html: body + vaDock() }) };
});

/* toolbar of the round buttons: ←/→ */
document.addEventListener('focusin', (e) => {
  const b = e.target.closest && e.target.closest('[data-vtool] [data-vt]'); if (!b) return;
  $$('[data-vt]', b.closest('[data-vtool]')).forEach((x) => x.setAttribute('tabindex', x === b ? '0' : '-1'));
});
document.addEventListener('keydown', (e) => {
  if (ui.menu || ui.dialog) return;
  const box = e.target.closest && e.target.closest('[data-vtool]'); if (!box || e.target.closest('.lmenu')) return;
  const items = $$('[data-vt]', box); const k = items.indexOf(e.target); if (k < 0) return;
  const to = (n) => items[(n + items.length) % items.length].focus();
  if (e.key === 'ArrowRight') { e.preventDefault(); to(k + 1); } else if (e.key === 'ArrowLeft') { e.preventDefault(); to(k - 1); }
  else if (e.key === 'Home') { e.preventDefault(); to(0); } else if (e.key === 'End') { e.preventDefault(); to(items.length - 1); }
});

/* ======================================================================================
   LS-15 Senderprogramm
   ====================================================================================== */
route('/live/guide/channel/:id', (pm, query) => {
  const c = SIM.channel(pm.id);
  /* mockup state «Leer»: no channel in the offer, so no channel programme either */
  if (liveMode() === 'empty') return { type: 'viewer', nav: 'live', title: 'Senderprogramm', html: livePage({ tab: 'guide', title: 'Senderprogramm', html: emptyState('Noch keine Sender', LU.TX.empty, `<a class="btn secondary" href="#/live/channels" data-fid="ls15:empty">${icon('tv', 18)}Zu den Sendern</a>`) }) };
  if (!c || SIM.chState(c) === 'gone' || c.active === false) return vaNotFound('Nicht gefunden', 'Diesen Sender gibt es nicht.', `<a class="btn secondary" href="#/live/channels" data-fid="ls15:nf">${icon('tv', 18)}Zu den Sendern</a>`);
  const t = SIM.now(); const today = Math.floor(t / 1440);
  const di = Math.max(0, Math.min(6, Number(query.day) || 0)); const from = (today + di) * 1440; const to = from + 1440;
  const mode = liveMode(); const r = SIM.rights(); const dis = SIM.chState(c) === 'disturbed'; const fav = SIM.isFav(c.id);
  const noEpg = SIM.epgOf(c) === 'none';
  const days = `<nav class="va-days" aria-label="Tag wählen"><ul>${Array.from({ length: 7 }, (_, i) => `<li><a class="va-chip" href="#/live/guide/channel/${c.id}${i ? '?day=' + i : ''}" aria-current="${i === di}" data-fid="ls15:d:${i}">${i === 0 ? 'Heute' : i === 1 ? 'Morgen' : fmt.day((today + i) * 1440)}</a></li>`).join('')}</ul></nav>`;
  const title = `${LU.logo(c, { cls: 'va-biglogo', label: true })}<span>${esc(c.name)}</span><span class="va-chno">Nr. ${c.no}</span>`;
  const acts = `<button type="button" class="btn primary" data-act="tune" data-ch="${c.id}" data-fid="ls15:tune">${icon('play', 18)}Einschalten</button>
    <button type="button" class="btn secondary heart${fav ? ' on' : ''}" data-act="fav" data-ch="${c.id}" aria-pressed="${fav}" data-fid="ls15:fav">${icon(fav ? 'heartOn' : 'heart', 18)}${fav ? 'In Meine Sender' : 'Zu Meine Sender'}</button>
    <a class="btn ghost" href="#/live/guide" data-fid="ls15:guide">${icon('grid', 18)}Programmzeitschrift</a>`;
  const recBtn = (o) => (r.recordings ? `<button type="button" class="btn secondary sm va-gaprec" data-act="vaManual" data-ch="${c.id}" ${o ? `data-start="${o[0]}" data-end="${o[1]}"` : ''} data-fid="ls15:manual${o ? ':' + o[0] : ''}">${icon('record', 14)}Zeit aufnehmen</button>` : '');
  let body = vaMode(skelRows(8, 60), 'Das Senderprogramm konnte nicht geladen werden.');
  if (!body) {
    const list = SIM.programsOf(c.id, from, to);
    const warn = dis ? notice('warn', `${esc(c.name)}: ${esc(LU.TX.unavailable)}`, 'Einschalten ist trotzdem möglich. Der Player zeigt dann den Zustand.') : '';
    if (noEpg || !list.length) {
      body = warn + (noEpg
        ? emptyState('Für diesen Sender gibt es keine Programmdaten.', 'Fernsehen geht trotzdem. Eine Aufnahme nach Uhrzeit ist möglich.', `<button type="button" class="btn secondary" data-act="tune" data-ch="${c.id}" data-fid="ls15:tune2">${icon('play', 18)}Einschalten</button>${recBtn(null).replace('btn secondary sm', 'btn secondary')}`)
        : emptyState('Keine Programmdaten für diesen Tag', 'Für diesen Tag liegen keine Angaben vor.', recBtn([from, to]).replace('btn secondary sm', 'btn secondary')));
    } else {
      const lis = []; let cur = from;
      const gapRow = (a, b) => `<li class="va-gaprow lv-gap" data-k="gap${a}"><span class="va-time">${vaT(a, to)}–${vaT(b, to)}</span><span class="lv-gap-text">${esc(LU.TX.epgNone)}</span>${b > t ? recBtn([a, b]) : ''}</li>`;
      list.forEach((p) => { if (p.start > cur) lis.push(gapRow(cur, p.start)); lis.push(vaProgRow(p, { noCh: true, cur: true })); cur = Math.max(cur, p.end); });
      if (cur < to) lis.push(gapRow(cur, to));
      body = warn + vaList('ls15:' + c.id + ':' + di, lis, `Programm von ${c.name}, Pfeiltasten wechseln, nach rechts zu den Aktionen`, 'va-day');
    }
  }
  return { type: 'viewer', nav: 'live', title: c.name, html: livePage({ tab: 'guide', title, count: `${fmt.dayLong(from)}`, actions: acts, html: days + `<div class="va-pgbody">${body}</div>${vaDock()}` }),
    after: (root) => { const key = location.hash; if (VA.scrolledKey === key) return; VA.scrolledKey = key; const cur = $('[aria-current="time"]', root); if (cur) cur.scrollIntoView({ block: 'center' }); } };
});
ACT.vaManual = (d) => LiveDlg.manualRecording({ ch: d.ch, start: d.start ? Number(d.start) : undefined, end: d.end ? Number(d.end) : undefined });

/* ======================================================================================
   LS-21 Profilformular, Block «Live-TV» (nur Hauptprofil ändert Profile)
   ====================================================================================== */
VA.pf.req = VA.pf.req || { anna: true, luca: true, mia: false };
const VA_RIGHTS = [
  ['liveTv', 'Live-TV', 'Fernsehen schauen und das Programm sehen.'],
  ['recordings', 'Aufnahmen', 'Sendungen aufnehmen. Aufnahmen belegen Speicher des Kontos.'],
  ['timeshift', 'Timeshift', 'Live-TV anhalten und zurückspulen, bis zum eigenen Einschalten.'],
];
const VA_AGES = [['', 'Keine Altersgrenze'], ['6', 'ab 6 Jahren'], ['12', 'ab 12 Jahren'], ['16', 'ab 16 Jahren'], ['18', 'ab 18 Jahren']];
function vaPfDraft(pid) {
  const pr = LIVE.profiles.find((x) => x.id === pid); const raw = SIM.rawRights('profile', pid);
  const base = { name: pr.name, age: pr.age == null ? '' : String(pr.age), req: !!VA.pf.req[pid], liveTv: !!raw.liveTv, recordings: !!raw.recordings, timeshift: !!raw.timeshift };
  const d = VA.pf.d[pid];
  if (!d || ['liveTv', 'recordings', 'timeshift'].some((k) => d.base[k] !== base[k])) VA.pf.d[pid] = { base, v: Object.assign({}, base) };
  return VA.pf.d[pid];
}
const vaPfDirty = (d) => Object.keys(d.base).some((k) => d.base[k] !== d.v[k]);
/** effective state of one right in the form: draft AND account cap AND live TV on */
function vaPfEff(d, pid, k) {
  const capped = !SIM.rawRights('account')[k]; const dep = k !== 'liveTv' && !d.v.liveTv;
  return { capped, dep, disabled: capped || dep, on: !!d.v[k] && !capped && (k === 'liveTv' || !!d.v.liveTv) };
}
function vaPfSwitch(d, pid, k, label, help) {
  const e = vaPfEff(d, pid, k); const note = e.capped ? LU.TX.capped : e.dep ? 'Zuerst Live-TV einschalten.' : '';
  return `<div class="va-sw${e.disabled ? ' off' : ''}"><div class="va-swt"><b id="pf-${k}-l">${label}</b><span id="pf-${k}-h">${help}</span>${note ? `<span class="va-swn" id="pf-${k}-n">${esc(note)}</span>` : ''}</div>
    <button type="button" class="sw" role="switch" aria-checked="${e.on}" ${e.disabled ? 'aria-disabled="true"' : ''} aria-labelledby="pf-${k}-l" aria-describedby="pf-${k}-h${note ? ` pf-${k}-n` : ''}" data-act="vaPfTog" data-key="${k}" data-pid="${pid}" data-fid="pf:${pid}:${k}"></button></div>`;
}
route('/settings/profiles', () => {
  const me = SIM.profile(); const frame = (h) => ({ type: 'viewer', nav: 'profiles', title: 'Profile', html: `<div class="page"><div class="head"><h1>Profile</h1><p class="sub">Profile deines Kontos «Familie Keller». Nur das Hauptprofil ändert Profile.</p></div>${h}${LU.sampleNote()}</div>` });
  if (!me.primary) return frame(notice('', 'Nur das Hauptprofil ändert Profile.', `Du bist als ${esc(me.name)} angemeldet. Wechsle zum Hauptprofil, um Profile zu ändern.`) + `<ul class="stack">${LIVE.profiles.map((x) => `<li class="lrow"><span class="av ${esc(x.accent)}">${esc(x.name[0])}</span><div class="bd2"><span class="ttl">${esc(x.name)}</span><span class="ln">${esc(roleLabel(x))}</span></div></li>`).join('')}</ul>`);
  let sel = VA.pf.sel; if (!LIVE.profiles.some((x) => x.id === sel)) sel = VA.pf.sel = 'anna';
  const pr = LIVE.profiles.find((x) => x.id === sel); const d = vaPfDraft(sel); const dirty = vaPfDirty(d);
  const eff = Object.fromEntries(VA_RIGHTS.map(([k]) => [k, vaPfEff(d, sel, k)]));
  const summary = VA_RIGHTS.map(([k, l]) => `${l}: ${eff[k].on ? 'an' : 'aus'}`).join(' · ');
  const tabs = `<div class="va-pftabs" role="group" aria-label="Profil wählen">${LIVE.profiles.map((x) => `<button type="button" class="va-pft" aria-pressed="${x.id === sel}" data-act="vaPfSel" data-id="${x.id}" data-fid="pf:sel:${x.id}"><span class="av ${esc(x.accent)}">${esc(x.name[0])}</span><span><b>${esc(x.name)}</b><small>${esc(roleLabel(x))}</small></span></button>`).join('')}</div>`;
  const age = d.v.age;
  const live = `${VA_RIGHTS.map(([k, l, h]) => vaPfSwitch(d, sel, k, l, h)).join('')}${age ? notice('', 'Live-TV zeigt alles, was der Sender zeigt.', 'Slouchcat prüft keine Altersfreigaben von Sendern oder Sendungen.') : ''}`;
  const form = `<div class="panel va-pf"><h2>${esc(d.v.name || pr.name)}</h2>
    <div class="va-pfgrid"><div class="fld"><label for="pf-name">Name</label><input class="inp" id="pf-name" value="${esc(d.v.name)}" data-bind="ui.va.pf.d.${sel}.v.name" data-live data-fid="pf:name" ${d.v.name ? '' : 'aria-invalid="true"'}>${d.v.name ? '' : '<span class="err">Der Name darf nicht leer sein.</span>'}</div>
    <div class="fld"><label for="pf-age">Altersgrenze</label><select class="sel" id="pf-age" data-bind="ui.va.pf.d.${sel}.v.age" data-rerender data-fid="pf:age" aria-describedby="pf-age-h">${VA_AGES.map(([v, t]) => `<option value="${v}"${d.v.age === v ? ' selected' : ''}>${t}</option>`).join('')}</select><span class="hint" id="pf-age-h">Die Altersgrenze gilt für die Bibliothek. Sie ändert nichts an Live-TV.</span></div></div>
    <div class="va-sw"><div class="va-swt"><b id="pf-req-l">Darf Titel anfordern</b><span id="pf-req-h">Wünsche über Seerr stellen.</span></div><button type="button" class="sw" role="switch" aria-checked="${d.v.req}" aria-labelledby="pf-req-l" aria-describedby="pf-req-h" data-act="vaPfTog" data-key="req" data-pid="${sel}" data-fid="pf:${sel}:req"></button></div>
    ${band('va-pf-live:' + sel, 'Live-TV', `<span>${esc(summary)}</span>`, `<div class="va-pflive">${live}</div>`, { viewer: true, icon: 'live' })}
    ${dirty ? `<div class="savebar" role="group" aria-label="Änderungen"><span class="grow">Ungespeicherte Änderungen für ${esc(d.v.name || pr.name)}</span><button type="button" class="btn secondary" data-act="vaPfReset" data-pid="${sel}" data-fid="pf:reset">Verwerfen</button><button type="button" class="btn primary" data-act="vaPfSave" data-pid="${sel}" data-fid="pf:save">${icon('check', 18)}Speichern</button></div>` : ''}</div>`;
  /* LS-14 is reached from here; without own «Aufnahmen» only in variant E-28-42 (a) (mockup state a12, A-12, 14 3.14) */
  const kids = SIM.rights().recordings || SIM.flags().a12 ? SIM.childProfiles().filter((k) => SIM.rights(k.id).recordings && SIM.recordings(k.id).length) : [];
  const kidsLink = kids.length ? `<a class="vb-link" href="#/live/recordings/profiles" data-fid="pf:kids">${icon('users', 20)}<span class="grow"><b>Aufnahmen meiner Profile</b><span class="muted"> · ${esc(kids.map((k) => k.name).join(', '))}</span></span>${icon('chevR', 18)}</a>` : '';
  return frame(tabs + form + kidsLink);
});
ACT.vaPfSel = (d) => { VA.pf.sel = d.id; render(); };
ACT.vaPfTog = (d, el) => {
  if (el.getAttribute('aria-disabled') === 'true') return;
  const dr = vaPfDraft(d.pid); const k = d.key; const turnOn = !dr.v[k];
  if (k === 'liveTv' && turnOn && dr.v.age) { openDialog('vaConfirmLive', { pid: d.pid }); return; }
  dr.v[k] = turnOn; render();
};
DIALOGS.vaConfirmLive = (p) => {
  const dr = vaPfDraft(p.pid); const name = esc(dr.v.name || LIVE.profiles.find((x) => x.id === p.pid).name);
  return { title: `Live-TV für ${name} einschalten?`, body: `<p class="lead">${name} sieht dann alle Sender und Sendungen ohne Altersprüfung.</p>`,
    actions: `<button type="button" class="btn secondary" data-act="close" data-autofocus data-fid="vacl:no">Abbrechen</button><button type="button" class="btn primary" data-act="vaPfConfirm" data-pid="${p.pid}" data-fid="vacl:yes">Einschalten</button>` };
};
ACT.vaPfConfirm = (d) => { vaPfDraft(d.pid).v.liveTv = true; closeDialog(); render(); };
ACT.vaPfReset = (d) => { delete VA.pf.d[d.pid]; render(); };
ACT.vaPfSave = (d) => {
  const dr = vaPfDraft(d.pid); if (!dr.v.name.trim()) { toast('Der Name darf nicht leer sein.', 'bad'); const i = document.querySelector('[data-fid="pf:name"]'); if (i) i.focus(); return; }
  const pr = LIVE.profiles.find((x) => x.id === d.pid); pr.name = dr.v.name.trim(); pr.age = dr.v.age ? Number(dr.v.age) : null; VA.pf.req[d.pid] = dr.v.req;
  const v = dr.v; delete VA.pf.d[d.pid];
  ['liveTv', 'recordings', 'timeshift'].forEach((k) => SIM.setRight('profile', k, v[k], d.pid));
  SIM.emit('change'); toast(`Profil <b>${esc(pr.name)}</b> gespeichert.`);
};

/* ======================================================================================
   LS-22 Benachrichtigungen einstellen (neue Ereignisse)
   ====================================================================================== */
const VA_CH = [['bell', 'Glocke'], ['mail', 'E-Mail'], ['push', 'Push']];
const VA_EVENTS = [
  { g: 'wish', id: 'wishAvail', t: 'Wunsch verfügbar', d: 'Ein gewünschter Titel ist in der Bibliothek.', def: [1, 1, 1] },
  { g: 'wish', id: 'wishDecl', t: 'Wunsch abgelehnt', d: 'Ein Administrator hat einen Wunsch abgelehnt.', def: [1, 1, 0] },
  { g: 'lib', id: 'newEp', t: 'Neue Folge', d: 'Eine Serie deiner Merkliste hat eine neue Folge.', def: [1, 0, 1] },
  { g: 'lib', id: 'newMovie', t: 'Neu in der Bibliothek', d: 'Neue Titel in deinen Bibliotheken.', def: [1, 0, 0] },
  { g: 'live', id: 'remind', t: 'Sendungserinnerung', d: 'Kurz vor einer Sendung, an die du dich erinnern lässt.', def: [1, 0, 1], need: 'liveTv' },
  { g: 'live', id: 'recDone', t: 'Aufnahme fertig', d: 'Eine Aufnahme ist vollständig gespeichert.', def: [1, 0, 0], need: 'recordings' },
  { g: 'live', id: 'recFail', t: 'Aufnahme fehlgeschlagen oder unvollständig', d: 'Der Sender war nicht erreichbar oder es fehlen Teile.', def: [1, 1, 1], need: 'recordings' },
  { g: 'live', id: 'recNo', t: 'Aufnahme nicht möglich', d: 'Zum Beispiel, weil der Speicher des Kontos voll ist.', def: [1, 1, 1], need: 'recordings' },
];
const VA_GROUPS = [['wish', 'Wünsche', 'wish'], ['lib', 'Bibliothek', 'film'], ['live', 'Live-TV und Aufnahmen', 'live']];
VA.nt = VA.nt || Object.fromEntries(VA_EVENTS.map((e) => [e.id, e.def.map(Boolean)]));
route('/settings/notifications', (pm, query) => {
  const r = SIM.rights();
  const groups = VA_GROUPS.map(([g, title0, ic]) => {
    const evs = VA_EVENTS.filter((e) => e.g === g && (!e.need || r[e.need]));
    if (!evs.length) return '';
    const title = g === 'live' && !r.recordings ? 'Live-TV' : title0;
    const on = evs.reduce((n, e) => n + VA.nt[e.id].filter(Boolean).length, 0); const all = evs.length * 3;
    const rows = evs.map((e) => `<li class="va-ev"><div class="va-evt"><b>${esc(e.t)}</b><span>${esc(e.d)}</span></div><div class="va-evc">${VA_CH.map(([ck, cl], i) => `<span class="va-nc"><span class="va-ncl" aria-hidden="true">${cl}</span>${sw(`ui.va.nt.${e.id}.${i}`, `${e.t}: ${cl}`)}</span>`).join('')}</div></li>`).join('');
    const titles = g === 'live' ? vaTitlesRow() : '';
    return band('va-nt:' + g, title, `<span>${on} von ${all} an</span>`, `<div class="va-evh" aria-hidden="true"><span></span><span>${VA_CH.map(([, l]) => `<span>${l}</span>`).join('')}</span></div><ul class="va-evl">${rows}</ul>${titles}`, { viewer: true, icon: ic, open: query.open === g });
  }).join('');
  const html = `<div class="page"><div class="head"><h1>Benachrichtigungen</h1><p class="sub">Wähle, worüber und wie du benachrichtigt wirst. Die Glocke findest du oben rechts. Push kommt auf Geräte mit der App.</p><div class="acts"><a class="btn secondary" href="#/notifications" data-fid="ls22:inbox">${icon('bell', 18)}Zur Glocke</a><button type="button" class="btn ghost" data-act="vaNtTest" data-fid="ls22:test">${icon('send', 18)}Testmeldung senden</button></div></div>
    ${groups}${LU.sampleNote()}</div>`;
  return { type: 'viewer', nav: 'notif', title: 'Benachrichtigungen einstellen', html };
});
/* switch of the profile «Titel und Sender in Meldungen nach aussen nennen» (12 SM-56): default off, never for profiles with a PIN */
VA.titles = VA.titles || {};
function vaTitlesRow() {
  const p = SIM.profile(); const pin = !!p.pin;
  return `<div class="va-sw"><div class="va-swt"><b id="nt-titles-l">Titel und Sender in Meldungen nach aussen nennen</b><span id="nt-titles-h">Ohne diesen Schalter nennen E-Mail, Push und Telegram weder Titel noch Sender. Die Glocke zeigt beides.</span><span class="va-swn" id="nt-titles-n">Für Profile mit PIN geht das nie, damit eine Meldung nichts aus einem geschützten Profil verrät.</span></div>${sw(`ui.va.titles.${p.id}`, 'Titel und Sender in Meldungen nach aussen nennen', { after: 'vaTitlesTog', disabled: pin })}</div>`;
}
ACT.vaTitlesTog = () => toast(VA.titles[SIM.profile().id] ? 'Meldungen nach aussen nennen jetzt Titel und Sender.' : 'Meldungen nach aussen nennen keinen Titel und keinen Sender mehr.');
ACT.vaNtTest = () => toast('Testmeldung gesendet. Sie erscheint in der Glocke.', 'mail');

/* ======================================================================================
   LS-23 Benachrichtigungen (Glocke)
   ====================================================================================== */
function vaInboxItems() {
  const r = SIM.rights(); const t = SIM.now(); const out = [];
  out.push({ id: 'n-wish', t: t - 190, ic: 'wish', title: 'Dein Wunsch «Dune: Teil zwei» ist verfügbar', line: 'Du findest ihn jetzt in der Bibliothek Filme.', href: '#/unchanged/wuensche' });
  out.push({ id: 'n-ep', t: t - 1500, ic: 'film', title: 'Neue Folge: Der Bestatter', line: 'Staffel 6, Folge 2 ist in der Bibliothek Serien.', href: '#/unchanged/serien' });
  if (r.liveTv) {
    const seed = SIM.programAt('srf1', LIVE.at(0, '20:06'));
    if (seed && seed.start - 5 < t) out.push({ id: 'n-rem-seed', t: seed.start - 5, ic: 'reminder', live: 'remind', unread: true, title: `${seed.title} beginnt in 5 Min. auf ${SIM.channel(seed.ch).name}`, line: `${fmt.range(seed.start, seed.end)} · Sendungserinnerung`, href: `#/live/programs/${seed.id}`, tune: seed.ch });
    SIM.reminders().forEach((m) => { const p = SIM.program(m.program); if (p && p.start - m.lead <= t && p.start > t - 30) out.push({ id: 'n-' + m.id, t: p.start - m.lead, ic: 'reminder', live: 'remind', unread: true, title: m.lead ? `${p.title} beginnt in ${m.lead} Min. auf ${SIM.channel(p.ch).name}` : `${p.title} beginnt jetzt auf ${SIM.channel(p.ch).name}`, line: `${fmt.range(p.start, p.end)} · Sendungserinnerung`, href: `#/live/programs/${p.id}`, tune: p.ch }); });
  }
  if (r.recordings) {
    SIM.recordings().filter((x) => x.state !== 'running' && x.end <= t && x.end > t - 12 * 1440).forEach((x) => {
      const base = { href: `#/live/recordings/${x.id}`, ic: 'record', live: 'rec' };
      if (x.state === 'done') out.push(Object.assign(base, { id: 'n-' + x.id, t: x.end, title: `Aufnahme fertig: ${x.title}${x.sub ? ' «' + x.sub + '»' : ''}`, line: `${SIM.channel(x.ch).name} · ${fmt.gb(x.sizeGB)} · ${fmt.when(x.start)}` }));
      else if (x.state === 'incomplete') out.push(Object.assign(base, { id: 'n-' + x.id, t: x.end, unread: true, ic: 'alert', title: `Aufnahme unvollständig: ${x.title}`, line: `${SIM.channel(x.ch).name} · ${x.gaps.length} Lücken, zusammen ${Math.round(x.gaps.reduce((s, g) => s + g[1] - g[0], 0))} Minuten` }));
      else if (x.state === 'failed') out.push(Object.assign(base, { id: 'n-' + x.id, t: x.end, ic: 'alert', title: `Aufnahme fehlgeschlagen: ${x.title}`, line: LU.REASON[x.reason] || '' }));
      else if (x.state === 'rejected') out.push(Object.assign(base, { id: 'n-' + x.id, t: x.start, ic: 'alert', title: `Aufnahme nicht möglich: ${x.title}`, line: LU.REASON[x.reason] || '' }));
    });
  }
  return out.sort((a, b) => b.t - a.t);
}
route('/notifications', () => {
  let body = vaMode(skelRows(5, 76), 'Die Benachrichtigungen konnten nicht geladen werden.');
  let acts = '';
  if (!body) {
    const items = vaInboxItems(); const unread = items.filter((i) => i.unread && !VA.read[i.id]).length;
    acts = `${unread ? `<button type="button" class="btn secondary" data-act="vaReadAll" data-fid="ls23:all">${icon('check', 18)}Alle als gelesen markieren</button>` : ''}<a class="btn ghost" href="#/settings/notifications" data-fid="ls23:set">${icon('settings', 18)}Einstellungen</a>`;
    body = items.length ? `<ul class="va-inbox" aria-label="Benachrichtigungen">${items.map((i) => {
      const un = i.unread && !VA.read[i.id];
      return `<li class="va-in${un ? ' unread' : ''}"><a class="va-inmain" href="${i.href}" data-act="vaRead" data-id="${i.id}" data-fid="in:${i.id}"><span class="va-inic${i.ic === 'alert' ? ' warn' : ''}">${icon(i.ic, 20)}</span><span class="va-int"><b>${esc(i.title)}${un ? '<span class="vh"> (ungelesen)</span>' : ''}</b><span>${esc(i.line)}</span></span><time class="va-intime">${fmt.ago(i.t)}</time></a>${i.tune ? `<span class="va-inacts"><button type="button" class="btn secondary sm" data-act="vaNtTune" data-id="${i.id}" data-ch="${i.tune}" data-fid="in:tune:${i.id}">${icon('play', 14)}Einschalten</button></span>` : ''}</li>`;
    }).join('')}</ul>` : emptyState('Keine Benachrichtigungen', 'Sobald etwas passiert, erscheint es hier.');
  }
  return { type: 'viewer', nav: '', title: 'Benachrichtigungen', html: `<div class="page"><div class="head"><h1>Benachrichtigungen</h1>${acts ? `<div class="acts">${acts}</div>` : ''}</div>${body}${LU.sampleNote()}</div>` };
});
ACT.vaRead = (d) => { VA.read[d.id] = true; };
ACT.vaReadAll = () => { vaInboxItems().forEach((i) => { VA.read[i.id] = true; }); render(); };
ACT.vaNtTune = (d, el, e) => { VA.read[d.id] = true; ACT.tune({ ch: d.ch }, el, e); };

/* ======================================================================================
   LS-24 Suche: Verweis ins TV-Programm (nie Aufnahmen)
   ====================================================================================== */
const VA_LIB = [['Tatort: Der Hammer', 'Film', 2019], ['Tatort Münster: Die Wahrheit', 'Serie', 2022], ['Tatort: Borowski und der stille Gast', 'Film', 2014], ['Der Marsianer', 'Film', 2015], ['Der Bestatter', 'Serie', 2013], ['Die Alpen von oben', 'Doku', 2021], ['Eishockey: Die Geschichte der NL', 'Doku', 2020], ['Sturm der Liebe', 'Serie', 2005]];
route('/search', (pm, query) => {
  const q = (query.q || '').trim(); const r = SIM.rights();
  const res = q ? VA_LIB.filter((x) => x[0].toLowerCase().includes(q.toLowerCase())) : [];
  const grid = res.length ? `<section aria-labelledby="ls24-res"><h2 class="st" id="ls24-res">In deiner Bibliothek · ${res.length}</h2><ul class="grid">${res.map(([t, k, y]) => `<li class="pc"><a class="pl" href="#/unchanged/${encodeURIComponent(t.toLowerCase())}" data-fid="ls24:r:${esc(t)}"><span class="frame">${img(null, t)}</span><span class="cap"><b>${esc(t)}</b><span>${k} · ${y}</span></span></a></li>`).join('')}</ul></section>`
    : q ? emptyState('Nichts in deiner Bibliothek gefunden', `Zu «${esc(q)}» gibt es keinen Titel in deinen Bibliotheken.`) : emptyState('Suche in deiner Bibliothek', 'Gib einen Titel, einen Namen oder ein Stichwort ein.');
  const tv = r.liveTv && q ? `<a class="va-lc va-tvlink" href="#/live/guide/search?q=${encodeURIComponent(q)}" data-fid="ls24:tv"><span class="va-lc-ic">${icon('live', 22)}</span><span class="va-lc-t"><b>Im TV-Programm suchen: ‹${esc(q)}›</b><span>Sendungen der nächsten 7 Tage auf allen Sendern</span></span><span class="va-lc-go">${icon('chevR', 18)}</span></a>` : '';
  const html = `<div class="page"><div class="head"><h1>Suche</h1><div class="va-tools" role="search"><label class="fld fsearch va-sbig"><span class="vh">Suchen</span><span class="sic">${icon('search', 18)}</span><input class="inp" type="search" placeholder="Titel, Namen, Stichwörter" value="${esc(q)}" data-vaq="search" data-fid="ls24:q" aria-label="Suchen"></label></div></div>${grid}${tv}${LU.sampleNote()}</div>`;
  return { type: 'viewer', nav: 'search', title: 'Suche', html };
});

/* ======================================================================================
   LS-25 Ton-Vorrang und Mini-Player (Radio, Hörbuch, Live-TV)
   ====================================================================================== */
const VA_AUDIO = [
  { id: 'srf3', k: 'radio', name: 'SRF 3', line: 'Pop und Rock · Live-Radio' },
  { id: 'virus', k: 'radio', name: 'SRF Virus', line: 'Neue Musik · Live-Radio' },
  { id: 'swisspop', k: 'radio', name: 'Radio Swiss Pop', line: 'Hits · Live-Radio' },
  { id: 'schwarm', k: 'book', name: 'Der Schwarm', line: 'Hörbuch · Kapitel 3, noch 1 Std. 12 Min.' },
];
route('/radio', () => {
  const cur = VA.radio; const r = SIM.rights();
  const li = (a) => `<li class="va-aud${cur && cur.id === a.id ? ' on' : ''}"><span class="va-audic">${icon(a.k === 'book' ? 'book' : 'radio', 20)}</span><span class="va-int"><b>${esc(a.name)}</b><span>${esc(a.line)}</span></span><button type="button" class="btn ${cur && cur.id === a.id ? 'secondary' : 'primary'} sm" data-act="${cur && cur.id === a.id ? 'vaRadioStop' : 'vaRadioPlay'}" data-id="${a.id}" aria-pressed="${!!(cur && cur.id === a.id)}" data-fid="ls25:p:${a.id}">${icon(cur && cur.id === a.id ? 'pause' : 'play', 14)}${cur && cur.id === a.id ? 'Beenden' : a.k === 'book' ? 'Fortsetzen' : 'Hören'}</button></li>`;
  const tv = r.liveTv ? `<section class="va-sec" aria-labelledby="ls25-tv"><h2 class="st" id="ls25-tv">Live-TV</h2><div class="va-prio"><span>${icon('live', 20)}</span><p>Live-TV ist eine dritte Tonquelle. Schaltest du einen Sender ein, endet Radio und Hörbuch. Verlässt du den Live-Player, bleibt kein Mini-Player zurück: Es gibt kein Bild-im-Bild.</p><button type="button" class="btn secondary" data-act="tune" data-ch="srf1" data-fid="ls25:tune">${icon('play', 18)}SRF 1 einschalten</button></div></section>` : '';
  const html = `<div class="page"><div class="head"><h1>Radio</h1><p class="sub">Live-Radio und Hörbücher. Es spielt immer nur eine Tonquelle.</p></div>
    <section class="va-sec" aria-labelledby="ls25-a"><h2 class="st" id="ls25-a">Hören</h2><ul class="va-audl">${VA_AUDIO.map(li).join('')}</ul></section>${tv}${LU.sampleNote()}</div>${vaDock()}`;
  return { type: 'viewer', nav: 'radio', title: 'Radio', html };
});
ACT.vaRadioPlay = (d) => { const a = VA_AUDIO.find((x) => x.id === d.id); if (!a) return; VA.radio = { id: a.id, name: a.name, line: a.line }; toast(`<b>${esc(a.name)}</b> läuft.`); render(); };
ACT.vaRadioStop = () => { const s = VA.radio; VA.radio = null; if (s) toast(`${esc(s.name)} beendet.`); render(); };
/* priority: switching on Live-TV ends radio and audiobook (the player of LS-07 sits on top of ACT.tune) */
(function () {
  const inner = ACT.tune;
  ACT.tune = (d, el, e) => { if (VA.radio) { const s = VA.radio; VA.radio = null; toast(`${esc(s.name)} wurde beendet: Live-TV hat Vorrang.`, 'warn'); } inner(d, el, e); };
})();

/* ======================================================================================
   LS-26 Startseite (unverändert)
   ====================================================================================== */
const VA_HOME = [['Weiterschauen', [['Der Marsianer', 'Film'], ['Der Bestatter', 'S06 E02'], ['Tatort: Spur im Nebel', 'Film'], ['Die Alpen von oben', 'Doku'], ['Arena', 'Sendung']]], ['Neu in deiner Bibliothek', [['Dune: Teil zwei', 'Film'], ['Wilde Schweiz', 'Doku'], ['Kassensturz', 'Serie'], ['Checker Tobi', 'Kinder'], ['Terra X', 'Doku'], ['Der Schwarm', 'Hörbuch']]]];
route('/', () => {
  const p = SIM.profile();
  const rows = VA_HOME.map(([title, list], k) => `<section><div class="rowhead"><h2>${title}</h2></div><div class="scroller"><ul>${list.map(([t, s]) => `<li class="pc"><a class="pl" href="#/unchanged/${encodeURIComponent(t.toLowerCase())}" data-fid="ls26:${k}:${esc(t)}"><span class="frame">${img(null, t)}</span><span class="cap"><b>${esc(t)}</b><span>${esc(s)}</span></span></a></li>`).join('')}</ul></div></section>`).join('');
  /* unchanged by concept 28 (DR-23): nothing from Live-TV and never a recording; the explanation is in 14 1.2, not in the picture */
  return { type: 'viewer', nav: 'start', title: 'Start', html: `<div class="home"><div class="hero va-hero"><h1>Hallo ${esc(p.name)}</h1><p class="sub">Das ist die Startseite der Bibliothek, wie sie heute ist.</p></div>${rows}<div class="va-homefoot">${LU.sampleNote()}</div></div>` };
});
