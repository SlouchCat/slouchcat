/* TV, Teil 3: Aufnahmen und Planung (LS-08 bis LS-14), Erinnerungen (LS-13), die Dialoge LD-01 bis LD-08 und LD-11
   als TV-gerechte Auswahlblätter (Wert in der Zeile, ←/→ oder OK schaltet weiter) und die kleinen Seiten
   LS-21 (Block «Live-TV» im Profilformular), LS-23 (Glocke), LS-24 (Suche), LS-26 (Startseite, unverändert).
   14-oberflaeche.md 3.8 bis 3.14, 5.1 bis 5.11. Alle globalen Namen beginnen mit «tv». */
'use strict';

/* ---------- shared bits of the recording pages ---------- */
/** private: own recordings, plus those of profiles with an age limit for the primary profile (LS-14) */
const tvRecVisible = (r) => !!r && SIM.mode() !== 'empty' && (r.profile === SIM.profile().id || (SIM.profile().primary && SIM.childProfiles().some((p) => p.id === r.profile)));
const tvKeepText = (r) => (r.keep && r.keep.mode === 'days' ? `Wird am ${fmt.day(r.keep.until)} gelöscht` : r.rule && SIM.rule(r.rule) && SIM.rule(r.rule).keepLast ? `Behalte die letzten ${SIM.rule(r.rule).keepLast} Folgen` : 'Behalten, bis du löschst');
const tvrWatch = (pct) => `<span class="lv-prog" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow="${pct}" aria-label="Wiedergabefortschritt"><i style="width:${pct}%"></i></span>`;
function tvRecCard(r, f, def) {
  const c = SIM.channel(r.ch); const run = r.state === 'running'; const D = Math.round((run ? SIM.now() : r.end) - r.start);
  const tag = run ? LU.badge('recording', { text: true, label: 'Nimmt auf' }) : r.state === 'incomplete' ? LU.badge('incomplete', { text: true }) : r.state === 'failed' ? LU.badge('failed', { text: true }) : r.state === 'rejected' ? LU.badge('failed', { text: true, label: 'Nicht aufgenommen' }) : (!r.watchedPct ? '<span class="lv-badge"><span>Neu</span></span>' : '');
  const ok = r.state === 'done' || r.state === 'incomplete' || run;
  return `<button type="button" class="tvp-card rec" data-act="tvrOpen" data-id="${r.id}" data-f="${f}"${def ? ' data-default' : ''} data-tt="${esc(r.title)}" data-tm="${esc(`${c.name} · ${fmt.day(r.start)} · ${tvKeepText(r)}`)}" aria-label="${esc(`${r.title}, ${c.name}, ${fmt.day(r.start)}, ${LU.REC_STATE[r.state]}`)}">
    <span class="tvp-im">${LU.picture({ ch: r.ch, img: null, title: r.title }, { fallback: 'logo' })}${tag ? `<span class="tvp-tag r">${tag}</span>` : ''}</span>
    <span class="tvp-cap"><b>${esc(r.title)}</b><small>${esc(c.name)} · ${fmt.day(r.start)} · ${ok ? (run ? `seit ${fmt.time(r.start)}, bis ${fmt.time(r.end)}` : fmt.dur(D)) : esc(LU.REASON[r.reason] || '')}</small>${r.watchedPct > 0 && r.watchedPct < 95 ? tvrWatch(r.watchedPct) : ''}<small class="tvp-keep">${ok ? esc(tvKeepText(r)) : ''}</small></span></button>`;
}
ACT.tvrOpen = (d) => go('#/live/recordings/' + d.id);
ACT.tvrSeries = (d) => go('#/live/recordings?series=' + d.rule);
function tvStore() {
  const q = SIM.quota(); const w = (x) => (x / q.totalGB * 100).toFixed(1);
  return `<div class="tvp-store" role="group" aria-label="Speicher des Kontos"><p><b>Speicher des Kontos:</b> ${fmt.gb(q.usedGB)} von ${fmt.gb(q.totalGB)} belegt</p>
    <span class="tvp-meter" role="img" aria-label="${esc(`Du ${q.mineGB} GB, übrige Profile ${q.othersGB} GB, frei ${q.freeGB} GB`)}"><i class="me" style="width:${w(q.mineGB)}%"></i><i class="ot" style="width:${w(q.othersGB)}%"></i></span>
    <p class="tvp-ml"><span><i class="me"></i>Du ${fmt.gb(q.mineGB)}</span><span><i class="ot"></i>Übrige Profile ${fmt.gb(q.othersGB)}</span><span><i></i>Frei ${fmt.gb(q.freeGB)}</span></p>
    ${q.freeGB <= 0 ? tvNote('alert', 'Der Speicher des Kontos ist voll.', 'Neue Aufnahmen werden abgelehnt. Lösche Aufnahmen, um Platz zu schaffen.', 'bad') : q.pct >= 90 ? tvNote('alert', 'Der Speicher des Kontos ist fast voll.', 'Neue Aufnahmen können abgelehnt werden.', 'warn') : ''}</div>`;
}
/* «Kontingent voll»: mockup bar «Zustände» → Aufnahmen (LIVE_SIM.flags().quotaFull) */
const tvMockQuota = () => '';

/* ---------- LS-08 Meine Aufnahmen ---------- */
route('/live/recordings/profiles', () => tvProfilesPage());
route('/live/recordings', (pm, query) => {
  const mode = SIM.mode(); const st = tvStates({ n: 4, cls: 'tvp-skelcard' }); const sid = query.series;
  const all = mode === 'empty' ? [] : SIM.recordings(); const rule = sid ? SIM.rule(sid) : null;
  const shown = rule ? all.filter((r) => r.rule === sid) : all;
  const good = shown.filter((r) => ['done', 'incomplete', 'running'].includes(r.state)); const bad = shown.filter((r) => ['failed', 'rejected'].includes(r.state));
  const up = SIM.upcoming().length; const kids = SIM.childProfiles().filter((p) => SIM.recordings(p.id).length);
  let first = true; const def = () => { const v = first; first = false; return v; };
  const sec = (id, title, items, extra) => `<section class="rsec tvp-sec"><h2 class="rh">${title}${extra ? `<span class="tvp-hx">${extra}</span>` : ''}</h2><div class="tvp-rail" data-row="${id}">${items}</div></section>`;
  let body;
  if (st) body = st;
  else if (!all.length) body = `${tvNote('info', 'Noch keine Aufnahmen', 'Plane Aufnahmen in der Programmzeitschrift oder starte eine im Player.')}<div class="ltv-acts"><button type="button" class="tb primary" data-act="go" data-to="#/live/guide" data-f="rc-go" data-default>${icon('calendar', 32)}<span>Zum Programm</span></button></div>`;
  else if (!shown.length) body = tvNote('info', 'Keine Aufnahmen dieser Serie', '');
  else {
    const running = good.filter((r) => r.state === 'running'); const fresh = good.filter((r) => r.state !== 'running' && !r.watchedPct);
    const stacks = {}; if (!rule) good.filter((r) => r.origin === 'series' && r.rule).forEach((r) => (stacks[r.rule] || (stacks[r.rule] = [])).push(r));
    const stackHtml = () => Object.keys(stacks).map((id) => { const rr = SIM.rule(id); const list = stacks[id]; const last = list[0]; const c = SIM.channel(last.ch);
      return `<button type="button" class="tvp-card rec stack" data-act="tvrSeries" data-rule="${id}" data-f="st-${id}" data-tt="${esc(rr ? rr.title : last.title)}" data-tm="${list.length} Folgen · ${fmt.gb(list.reduce((s, x) => s + x.sizeGB, 0))}"${def() ? ' data-default' : ''}><span class="tvp-im">${LU.picture({ ch: last.ch, img: null, title: last.title }, { fallback: 'logo' })}<span class="tvp-tag r">${LU.badge('series', { text: true, label: 'Serie' })}</span></span><span class="tvp-cap"><b>${esc(rr ? rr.title : last.title)} · ${list.length} Folgen</b><small>${esc(c.name)}${rr && rr.keepLast ? ` · behalte ${rr.keepLast}` : ''}</small></span></button>`; }).join('');
    const failOpen = TVS.open.fail || (rule ? false : false);
    body = `${rule ? `<p class="tvp-hint">Serie «${esc(rule.title)}»: ${good.length} Folgen. Zurück führt zu allen Aufnahmen.</p>` : ''}
      ${running.length ? sec('run', 'Läuft', running.map((r) => tvRecCard(r, 'rc-' + r.id, def())).join('')) : ''}
      ${!rule && fresh.length ? sec('new', 'Neu', fresh.map((r) => tvRecCard(r, 'rn-' + r.id, def())).join('')) : ''}
      ${Object.keys(stacks).length ? sec('series', 'Serien', stackHtml()) : ''}
      ${good.length ? sec('all', rule ? 'Folgen' : 'Alle', good.map((r) => tvRecCard(r, 'ra-' + r.id, def())).join('')) : ''}
      ${bad.length ? `<section class="rsec tvp-sec"><button type="button" class="tvp-fold" data-act="tvFold" data-f="rc-fold" aria-expanded="${!!failOpen}"${def() ? ' data-default' : ''}>${icon(failOpen ? 'down' : 'right', 32)}<b>Fehlgeschlagen und nicht aufgenommen</b><span>${bad.length} Aufnahmen</span></button>${failOpen ? `<div class="tvp-rail" data-row="bad">${bad.map((r) => tvRecCard(r, 'rb-' + r.id, false)).join('')}</div>` : ''}</section>` : ''}`;
  }
  const tools = `<div class="ltv-tools">${tvBtn({ f: 'rc-sched', ic: 'clock', label: `Geplant (${up})`, act: 'go', data: { to: '#/live/scheduled' }, cls: 'ghost' })}${tvBtn({ f: 'rc-new', ic: 'plus', label: 'Neue Aufnahme', act: 'tvNewRec', cls: 'ghost' })}
    ${kids.length ? `<a class="tvp-link" href="#/live/recordings/profiles" data-f="rc-kids"><span class="mi-ic">${icon('users', 36)}</span><span><b>Aufnahmen meiner Profile</b><small>${kids.map((p) => esc(p.name)).join(', ')}</small></span></a>` : ''}</div>`;
  return { nav: '', html: livePage({ tab: 'recordings', title: rule ? esc(rule.title) : 'Meine Aufnahmen', count: all.length && !st ? `${good.length} Aufnahmen` : '', html: `${st ? '' : tools}${tvStore()}${body.replace('<div class="tvp-rail"', '<div data-top class="tvp-rail"')}${tvMockQuota()}` }), key: tvpNumKeys };
});
ACT.tvFold = () => { TVS.open.fail = !TVS.open.fail; render(); };
ACT.tvNewRec = () => LiveDlg.manualRecording({});

/* ---------- LS-09 Aufnahme-Detail ---------- */
route('/live/recordings/:id', (pm) => {
  const r = SIM.recording(pm.id);
  if (!tvRecVisible(r)) return { nav: '', html: livePage({ tab: 'recordings', title: 'Nicht gefunden', html: `${tvNote('info', 'Diese Aufnahme gibt es nicht mehr.', '')}<div class="ltv-acts"><button type="button" class="tb primary" data-act="go" data-to="#/live/recordings" data-f="rd-nf" data-default>${icon('film', 32)}<span>Zu meinen Aufnahmen</span></button></div>` }) };
  const c = SIM.channel(r.ch); const run = r.state === 'running'; const ok = ['done', 'incomplete', 'running'].includes(r.state); const D = Math.round((run ? SIM.now() : r.end) - r.start);
  const mine = r.profile === SIM.profile().id; const owner = SIM.profiles().find((p) => p.id === r.profile);
  const rule = r.rule ? SIM.rule(r.rule) : null;
  const gaps = r.gaps || []; const gm = gaps.reduce((s, g) => s + (g[1] - g[0]), 0);
  const pct = (m) => (m / Math.max(1, D) * 100).toFixed(2);
  const btns = [];
  if (ok) {
    if (mine && r.watchedPct >= 1 && r.watchedPct < 95) { btns.push(tvBtn({ f: 'rd-res', ic: 'play', label: `Weiter ab ${tvrFmt(D * r.watchedPct / 100)}`, act: 'tvrPlay', data: { id: r.id, from: 'resume' }, cls: 'primary', def: true })); btns.push(tvBtn({ f: 'rd-start', ic: 'rew', label: 'Von vorne', act: 'tvrPlay', data: { id: r.id, from: 'start' } })); }
    else btns.push(tvBtn({ f: 'rd-play', ic: 'play', label: 'Abspielen', act: 'tvrPlay', data: { id: r.id, from: 'start' }, cls: 'primary', def: true }));
    btns.push(tvBtn({ f: 'rd-keep', ic: 'clock', label: 'Aufbewahrung', act: 'tvrKeep', data: { id: r.id } }));
  }
  if (run) btns.push(tvBtn({ f: 'rd-stop', ic: 'stop', label: 'Aufnahme beenden', act: 'tvrDel', data: { id: r.id, kind: 'running' }, cls: 'ghost' }));
  btns.push(tvBtn({ f: 'rd-del', ic: 'trash', label: ok ? 'Löschen' : 'Entfernen', act: 'tvrDel', data: { id: r.id, kind: mine ? 'recording' : 'childRecording' }, cls: 'ghost', def: !ok }));
  const state = r.state === 'done' ? tvNote('check', 'Vollständig aufgenommen.', '', '') : r.state === 'incomplete' ? tvNote('alert', `Unvollständig: ${gaps.length} Lücken, zusammen ${Math.round(gm)} Minuten.`, `Der Sender war ${gaps.map(([a, b]) => `von ${fmt.time(a)} bis ${fmt.time(b)}`).join(' und ')} nicht erreichbar.`, 'warn') : run ? tvNote('info', 'Die Aufnahme läuft.', `Bis ${fmt.time(r.end)}, bisher ${fmt.gb(r.liveGB || r.sizeGB)}.`) : tvNote('alert', r.state === 'failed' ? 'Die Aufnahme ist fehlgeschlagen.' : 'Die Aufnahme wurde nicht gemacht.', esc(LU.REASON[r.reason] || ''), 'bad');
  const fall = r.fallbackAt ? `<p class="tvp-fact">${icon('info', 28)}Um ${fmt.time(r.fallbackAt)} auf eine Ersatzquelle gewechselt.</p>` : '';
  const origin = r.origin === 'series' && rule ? `Aus der Serienaufnahme «${esc(rule.title)}»` : r.origin === 'instant' ? 'Sofortaufnahme' : `Einzeln geplant am ${fmt.day(r.start - 2000)}`;
  return { nav: '', html: livePage({ tab: 'recordings', html: `<div class="tvp-split"><div class="tvp-pi"><div class="tvp-pi-top">${LU.logo(c, { cls: 'big' })}<span><b>${c.no} ${esc(c.name)}</b><small>${esc(fmt.dayLong(r.start))}${!mine && owner ? ` · Aufnahme von ${esc(owner.name)}` : ''}</small></span></div>
      <h2 class="tvp-pi-t">${esc(r.title)}${r.sub ? `<span> «${esc(r.sub)}»</span>` : ''}</h2>
      ${ok ? `<p class="tvp-pi-m">${fmt.range(r.start, run ? r.end : r.end)} · ${fmt.dur(D)}${r.padBefore || r.padAfter ? `, davon Vorlauf ${r.padBefore} Min. und Nachlauf ${r.padAfter} Min.` : ''} · ${fmt.gb(run ? (r.liveGB || r.sizeGB) : r.sizeGB)}</p><p class="tvp-keep">${esc(tvKeepText(r))}</p>` : ''}
      ${state}${gaps.length ? `<div class="tvr-tl" aria-hidden="true">${gaps.map(([a, b]) => `<i class="lv-gap" style="left:${pct(a - r.start)}%;width:${pct(b - a)}%"></i>`).join('')}</div>` : ''}${fall}
      ${ok && !mine ? `<p class="tvp-fact">${icon('info', 28)}Fortschritt wird nicht gespeichert, wenn du die Aufnahme von ${esc(owner ? owner.name : '')} abspielst.</p>` : ''}${ok ? `<p class="tvp-fact">${icon('audio', 28)}${esc(r.tracks || '')}</p>` : ''}<p class="tvp-fact">${icon('calendar', 28)}${origin}${rule && r.origin === 'series' ? ` <a class="tvp-inl" href="#/live/scheduled" data-f="rd-ser" data-act="tvGoSerien">Serien ansehen</a>` : ''}</p></div>
      <div class="tvp-col tvp-acts">${btns.join('')}</div></div>` }) };
});
ACT.tvGoSerien = () => { TVS.ctab = 'series'; go('#/live/scheduled'); };
ACT.tvrPlay = (d) => go(`#/live/recordings/${d.id}/play?from=${d.from}`);
ACT.tvrKeep = (d) => LiveDlg.retention(d.id);
ACT.tvrDel = (d) => LiveDlg.confirmDelete({ kind: d.kind, ids: [d.id], onDone: () => { if (curPath().startsWith('/live/recordings/') && d.kind !== 'running') go('#/live/recordings', { replace: true }); } });

/* ---------- LS-14 Aufnahmen meiner Profile ---------- */
function tvProfilesPage() {
  const kids = SIM.childProfiles(); const st = tvStates({ n: 3, cls: 'ltv-skel' });
  const list = SIM.mode() === 'empty' ? [] : kids;
  let body;
  if (st) body = st;
  else if (!kids.length) body = tvNote('info', 'Keine Profile mit Altersgrenze', 'Hier erscheinen Profile mit Altersgrenze und Aufnahmerecht.');
  else {
    body = `<p class="tvp-lead">Hier siehst du die Aufnahmen der Profile mit Altersgrenze. Aufnahmen von Profilen ohne Altersgrenze bleiben privat. Beim Abspielen wird kein Fortschritt gespeichert.</p>` + (list.every((p) => !SIM.recordings(p.id).length) ? tvNote('info', `Die Profile ${kids.map((p) => esc(p.name)).join(' und ')} haben keine Aufnahmen.`, '') : '') +
      list.map((p) => {
        const recs = SIM.recordings(p.id); const open = !!TVS.open['kid-' + p.id]; const gb = recs.reduce((s, r) => s + (r.sizeGB || 0), 0); const plan = SIM.upcoming(p.id).length;
        return `<section class="tvp-kid"><button type="button" class="tvp-fold" data-act="tvKid" data-p="${p.id}" data-f="kp-${p.id}" aria-expanded="${open}"${p === list[0] ? ' data-default' : ''}>${icon(open ? 'down' : 'right', 32)}<span class="avatar kp-av" style="background:var(--${AVC[p.id]})">${esc(p.name[0])}</span><b>${esc(p.name)}</b><span>${recs.length} Aufnahmen · ${fmt.gb(gb)}${plan ? ` · ${plan} geplant` : ''}</span></button>
          ${open ? `<div class="tvp-res">${recs.length ? recs.map((r) => `<div class="tvp-prw"><button type="button" class="tvp-pr" data-act="tvrOpen" data-id="${r.id}" data-f="kr-${r.id}"><span class="tvp-pt"><b>${fmt.day(r.start)}</b><small>${['done', 'incomplete'].includes(r.state) ? fmt.dur(r.end - r.start) : esc(LU.REC_STATE[r.state])}</small></span><span class="tvp-pc">${LU.logo(SIM.channel(r.ch), { cls: 'sm' })}<span>${esc(SIM.channel(r.ch).name)}</span></span><span class="tvp-px"><b>${esc(r.title)}</b><small>${fmt.gb(r.sizeGB)} · ${esc(tvKeepText(r))}</small></span><span class="tvp-pb">${r.state === 'incomplete' ? LU.badge('incomplete', {}) : ''}</span></button>
            ${['done', 'incomplete'].includes(r.state) ? `<button type="button" class="tvp-act" data-act="tvrPlay" data-id="${r.id}" data-from="start" data-f="kr-${r.id}-play" aria-label="${esc(r.title)} abspielen">${icon('play', 40)}</button>` : ''}<button type="button" class="tvp-act" data-act="tvrDel" data-id="${r.id}" data-kind="childRecording" data-f="kr-${r.id}-del" aria-label="${esc(r.title)} löschen">${icon('trash', 40)}</button></div>`).join('') : tvNote('info', `${esc(p.name)} hat keine Aufnahmen.`, '')}</div>` : ''}</section>`;
      }).join('');
  }
  return { nav: '', html: livePage({ tab: 'recordings', title: 'Aufnahmen meiner Profile', html: body }), key: tvpNumKeys };
}
ACT.tvKid = (d) => { TVS.open['kid-' + d.p] = !TVS.open['kid-' + d.p]; render(); };

/* ---------- LS-11 Geplante Aufnahmen ---------- */
const TV_WARN = { quota: 'Kontingent', overlap: 'Überschneidung', channelGone: 'Sender weg', timeChanged: 'Zeit geändert', duplicate: 'Doppelt', serverStorage: 'Speicher des Servers' };
function tvOccState(u) {
  const t = SIM.now(); const ws = SIM.warningsFor(u.rule.id);
  if (SIM.recordings().some((r) => r.state === 'running' && r.ch === u.ch && r.start < u.end && r.end > u.start)) return { t: 'Nimmt auf', c: 'rec' };
  const w = ws.find((x) => x.severity !== 'info') || ws[0];
  if (w) return { t: `Warnung: ${TV_WARN[w.kind] || 'Hinweis'}`, c: w.severity === 'bad' ? 'bad' : 'warn' };
  if (SIM.quota().freeGB <= 0) return { t: 'Wird nicht aufgenommen: Kontingent', c: 'bad' };
  if (u.start > t && u.start - t <= 60) return { t: `Startet in ${Math.ceil(u.start - t)} Min.`, c: '' };
  return { t: 'Geplant', c: '' };
}
route('/live/scheduled/warnings', () => {
  const st = tvStates({ n: 3, cls: 'ltv-skel' }); const ws = SIM.mode() === 'empty' ? [] : SIM.warnings();
  const KIND = { quota: 'Kontingent', serverStorage: 'Speicher des Servers', overlap: 'Überschneidung', duplicate: 'Doppelt', channelGone: 'Sender weg', programGone: 'Sendung verschwunden', timeChanged: 'Zeit geändert', dst: 'Sommer-/Winterzeit' };
  const body = st || (ws.length ? `<div class="tvp-wl">${ws.map((w, i) => `<button type="button" class="tvp-wc ${w.severity}" data-act="tvWarn" data-id="${w.id}" data-f="wn-${w.id}"${i === 0 ? ' data-default' : ''}><span class="tvp-wi">${icon(w.severity === 'info' ? 'info' : 'alert', 40)}</span><span><b>${esc(KIND[w.kind] || 'Hinweis')}</b><small>${esc(w.text)}</small><em>Betrifft: ${w.rules.map((id) => { const rl = SIM.rule(id); return rl ? esc(rl.title) : ''; }).filter(Boolean).join(', ')}</em></span></button>`).join('')}</div>` : tvNote('check', 'Alles in Ordnung.', 'Keine geplante Aufnahme hat eine Warnung.'));
  return { nav: '', html: livePage({ tab: 'recordings', title: 'Konflikte und Warnungen', count: ws.length && !st ? `${ws.length}` : '', html: `<div class="ltv-tools">${tvBtn({ f: 'wn-back', ic: 'clock', label: 'Zu den geplanten Aufnahmen', act: 'go', data: { to: '#/live/scheduled' }, cls: 'ghost' })}</div>${body}` }), key: tvpNumKeys };
});
ACT.tvWarn = (d) => LiveDlg.warning(d.id);

route('/live/scheduled', () => {
  const tab = TVS.ctab; const st = tvStates({ n: 5, cls: 'ltv-skel' }); const empty = SIM.mode() === 'empty';
  const warn = SIM.warningCount(); const tabs = [['upcoming', 'Termine'], ['series', 'Serien'], ['recurring', 'Wiederkehrend']];
  const head = `<div class="ltv-tools">${tabs.map(([k, l]) => `<button type="button" class="tb sm ${k === tab ? 'primary' : 'ghost'}" data-act="tvTab" data-k="${k}" data-f="sc-${k}" aria-pressed="${k === tab}"><span>${l}</span></button>`).join('')}${tvBtn({ f: 'sc-new', ic: 'plus', label: 'Neue Aufnahme', act: 'tvNewRec', cls: 'ghost sm', sz: 28 })}</div>
    ${warn && !empty ? `<a class="tnote warn tvp-alert" href="#/live/scheduled/warnings" data-f="sc-warn">${icon('alert', 36)}<div><b>${esc(LU.warnText(warn))}</b><p>Ansehen</p></div></a>` : ''}`;
  let body;
  if (st) body = st;
  else if (tab === 'upcoming') {
    const ups = empty ? [] : SIM.upcoming();
    if (!ups.length) body = `${tvNote('info', 'Keine Aufnahmen geplant.', '')}<div class="ltv-acts"><button type="button" class="tb primary" data-act="go" data-to="#/live/guide" data-f="sc-go" data-default>${icon('calendar', 32)}<span>Zum Programm</span></button></div>`;
    else { let lastD = null; body = `<div class="tvp-res">${ups.map((u, i) => { const d = Math.floor(u.start / 1440); const h = d !== lastD ? `<h2 class="tvp-dayh">${fmt.dayRel(u.start)} · ${fmt.day(u.start)}</h2>` : ''; lastD = d; const s = tvOccState(u); const c = SIM.channel(u.ch); const pb = u.rule.padBefore || 0; const pa = u.rule.padAfter || 0;
      return h + `<div class="tvp-prw"><button type="button" class="tvp-pr" data-act="tvOcc" data-id="${esc(u.id)}" data-f="oc-${esc(u.id).replace(/[^a-z0-9-]/gi, '_')}"${i === 0 ? ' data-default' : ''} aria-label="${esc(`${fmt.range(u.start, u.end)} ${c.name} ${u.title}, ${s.t}`)}"><span class="tvp-pt"><b>${fmt.range(u.start, u.end)}</b><small>Vorlauf ${pb} · Nachlauf ${pa} Min.</small></span><span class="tvp-pc">${LU.logo(c, { cls: 'sm' })}<span>${esc(c.name)}</span></span><span class="tvp-px"><b>${esc(u.title)}</b><small>${LU.RULE_KIND[u.kind]}${u.sub ? ' · ' + esc(u.sub) : ''}</small></span><span class="tvp-st ${s.c}">${esc(s.t)}</span></button></div>`; }).join('')}</div>`; }
  } else if (tab === 'series') {
    const rs = empty ? [] : SIM.rules().filter((r) => r.kind === 'series');
    body = rs.length ? `<div class="tvp-res">${rs.map((r, i) => { const n = SIM.upcoming().filter((u) => u.rule.id === r.id).length; const k = SIM.recordings().filter((x) => x.rule === r.id).length;
      return `<div class="tvp-prw"><button type="button" class="tvp-pr" data-act="tvRule" data-id="${r.id}" data-f="sr-${r.id}"${i === 0 ? ' data-default' : ''}><span class="tvp-px wide"><b>${esc(r.title)}</b><small>${r.channels === 'all' ? 'alle Sender' : r.channels.map((id) => esc(SIM.channel(id).name)).join(', ')}${r.onlyNew ? ' · nur Erstausstrahlungen' : ''}${r.keepLast ? ` · behalte ${r.keepLast}` : ''} · ${n} Termin${n === 1 ? '' : 'e'} geplant · ${k} Aufnahmen</small></span></button></div>`; }).join('')}</div>` : tvNote('info', 'Keine Serienaufnahmen.', 'Öffne eine Sendung und wähle «Serie aufnehmen».');
  } else {
    const rs = empty ? [] : SIM.rules().filter((r) => r.kind === 'recurring' || r.kind === 'manual');
    body = rs.length ? `<div class="tvp-res">${rs.map((r, i) => { const c = SIM.channel(r.ch); const nx = SIM.upcoming().find((u) => u.rule.id === r.id); const w = SIM.warningsFor(r.id)[0];
      const days = r.weekdays ? (r.weekdays.length === 7 ? 'täglich' : r.weekdays.join() === '1,2,3,4,5' ? 'Mo–Fr' : r.weekdays.map((d) => ['So', 'Mo', 'Di', 'Mi', 'Do', 'Fr', 'Sa'][d]).join(', ')) : '';
      return `<div class="tvp-prw"><button type="button" class="tvp-pr" data-act="tvRule" data-id="${r.id}" data-f="wk-${r.id}"${i === 0 ? ' data-default' : ''}><span class="tvp-pc">${LU.logo(c, { cls: 'sm' })}<span>${esc(c.name)}</span></span><span class="tvp-px wide"><b>${esc(r.title)}</b><small>${r.kind === 'recurring' ? `${days} ${r.from}–${r.to}${r.keepLast ? ` · behalte ${r.keepLast}` : ''}${nx ? ` · nächster Termin ${fmt.day(nx.start)}` : ''}` : `Manuell · ${fmt.day(r.start)} ${fmt.range(r.start, r.end)}`}</small></span>${w ? `<span class="tvp-st ${w.severity === 'bad' ? 'bad' : 'warn'}">${icon('alert', 28)} ${esc(TV_WARN[w.kind] || 'Warnung')}</span>` : ''}</button></div>`; }).join('')}</div>` : `${tvNote('info', 'Keine wiederkehrenden Aufnahmen.', '')}<div class="ltv-acts">${tvBtn({ f: 'sc-n2', ic: 'plus', label: 'Neue Aufnahme', act: 'tvNewRec', cls: 'primary', def: true })}</div>`;
  }
  return { nav: '', html: livePage({ tab: 'recordings', title: 'Geplant', html: head + body }), key: tvpNumKeys };
});
ACT.tvTab = (d) => { TVS.ctab = d.k; render(); };
function tvEditOcc(u) {
  const r = u.rule;
  if (r.kind === 'single' && r.program) LiveDlg.planRecording(r.program);
  else if (r.kind === 'series') LiveDlg.seriesRecording(u.program || r.next, r.id);
  else LiveDlg.manualRecording({ ruleId: r.id, ch: r.ch });
}
ACT.tvOcc = (d) => {
  const u = SIM.upcoming().find((x) => x.id === d.id); if (!u) return; const c = SIM.channel(u.ch); const s = tvOccState(u); const ws = SIM.warningsFor(u.rule.id);
  menu(`<h2>${esc(u.title)}</h2><p>${esc(c.name)} · ${fmt.when(u.start)}–${fmt.time(u.end)} · ${LU.RULE_KIND[u.kind]}</p><p class="tvp-st ${s.c}">${esc(s.t)}</p>${ws.map((w) => `<p class="tvp-hint">${esc(w.text)}</p>`).join('')}
    <div class="mlist">${tvMi('oc-edit', 'edit', 'Ändern', 'Zeiten, Aufbewahrung', 'tvOccEdit', { id: u.id }, true)}${tvMi('oc-del', 'close', u.kind === 'single' || u.kind === 'manual' ? 'Nicht aufnehmen' : 'Abbrechen …', u.kind === 'series' || u.kind === 'recurring' ? 'Diesen Termin auslassen oder die ganze Regel beenden' : 'Der Termin entfällt', 'tvOccDel', { id: u.id })}
    ${u.program ? tvMi('oc-prog', 'info', 'Zur Sendung', 'Details der Sendung', 'tvOccProg', { pid: u.program }) : ''}</div>`);
};
const tvMi = (f, ic, t, s, act, data, def) => `<button type="button" class="mi" data-act="${act}"${tvDataAttrs(data)} data-f="${f}"${def ? ' data-default' : ''}><span class="mi-ic">${icon(ic, 36)}</span><span class="mi-tx"><b>${t}</b><small>${s}</small></span></button>`;
ACT.tvOccEdit = (d) => { const u = SIM.upcoming().find((x) => x.id === d.id); closeMenu(); if (u) tvEditOcc(u); };
ACT.tvOccDel = (d) => { closeMenu(); LiveDlg.confirmDelete({ kind: 'occurrence', ids: [d.id] }); };
ACT.tvOccProg = (d) => { closeMenu(); tvProgramMenu(d.pid); };
ACT.tvRule = (d) => {
  const r = SIM.rule(d.id); if (!r) return; const ups = SIM.upcoming().filter((u) => u.rule.id === d.id).slice(0, 3);
  const skipped = (r.skipped || []).map(([pid, why]) => { const p = SIM.program(pid); return p ? `${fmt.day(p.start)} ${fmt.time(p.start)} ${esc(SIM.channel(p.ch).name)}: ${esc(why)}` : ''; }).filter(Boolean);
  menu(`<h2>${esc(r.title)}</h2><p>${LU.RULE_KIND[r.kind]}${r.kind === 'series' ? '' : ''}</p>${ups.length ? `<p class="tvp-hint">Nächste Termine: ${ups.map((u) => `${fmt.day(u.start)} ${fmt.time(u.start)} ${esc(SIM.channel(u.ch).name)}`).join(' · ')}</p>` : ''}${skipped.length ? `<p class="tvp-hint">Übersprungen: ${skipped.join(' · ')}</p>` : ''}
    <div class="mlist">${tvMi('ru-edit', 'edit', 'Ändern', r.kind === 'series' ? 'Serienaufnahme' : 'Zeiten und Wiederholung', 'tvRuleEdit', { id: r.id }, true)}${tvMi('ru-end', 'close', r.kind === 'series' ? 'Beenden …' : 'Aufnahme beenden …', 'Die Regel endet', 'tvRuleEnd', { id: r.id, kind: r.kind })}</div>`);
};
ACT.tvRuleEdit = (d) => { const r = SIM.rule(d.id); closeMenu(); if (r.kind === 'series') { const p = SIM.upcoming().find((u) => u.rule.id === r.id); LiveDlg.seriesRecording(p ? p.program : (LIVE.programs.find((x) => x.series === r.series) || {}).id, r.id); } else LiveDlg.manualRecording({ ruleId: r.id, ch: r.ch }); };
ACT.tvRuleEnd = (d) => { closeMenu(); LiveDlg.confirmDelete({ kind: d.kind === 'series' ? 'series' : 'rule', ids: [d.id] }); };

/* ---------- LS-13 Erinnerungen ---------- */
route('/live/reminders', () => {
  const st = tvStates({ n: 4, cls: 'ltv-skel' }); const list = SIM.mode() === 'empty' ? [] : SIM.reminders();
  let body;
  if (st) body = st;
  else if (!list.length) body = `${tvNote('info', 'Keine Erinnerungen.', 'Öffne eine Sendung und wähle «Erinnern».')}<div class="ltv-acts"><button type="button" class="tb primary" data-act="go" data-to="#/live/guide" data-f="rm-go" data-default>${icon('calendar', 32)}<span>Zum Programm</span></button></div>`;
  else { let lastD = null; body = `<div class="tvp-res">${list.map((m, i) => { const p = SIM.program(m.program); const c = SIM.channel(p.ch); const d = Math.floor(p.start / 1440); const h = d !== lastD ? `<h2 class="tvp-dayh">${fmt.dayRel(p.start)} · ${fmt.day(p.start)}</h2>` : ''; lastD = d;
    return h + `<div class="tvp-prw"><button type="button" class="tvp-pr" data-act="tvDetail" data-pid="${p.id}" data-f="rm-${m.id}"${i === 0 ? ' data-default' : ''}><span class="tvp-pt"><b>${fmt.time(p.start)}</b><small>${m.lead ? `${m.lead} Min. vorher` : 'zum Beginn'}</small></span><span class="tvp-pc">${LU.logo(c, { cls: 'sm' })}<span>${esc(c.name)}</span></span><span class="tvp-px"><b>${esc(p.title)}</b><small>${p.sub ? esc(p.sub) + ' · ' : ''}Erinnert ${m.lead ? m.lead + ' Min. vorher' : 'zum Beginn'}</small></span></button>
      <button type="button" class="tvp-act" data-act="tvpRemRow" data-pid="${p.id}" data-f="rm-${m.id}-edit" aria-label="Erinnerung an ${esc(p.title)} ändern">${icon('edit', 40)}</button><button type="button" class="tvp-act" data-act="tvRemDel" data-pid="${p.id}" data-f="rm-${m.id}-del" aria-label="Erinnerung an ${esc(p.title)} entfernen">${icon('trash', 40)}</button></div>`; }).join('')}</div>`; }
  return { nav: '', html: livePage({ tab: 'reminders', title: 'Erinnerungen', count: list.length && !st ? `${list.length}` : '', html: body }), key: tvpNumKeys };
});
ACT.tvRemDel = (d) => { SIM.removeReminder(d.pid); toast('Erinnerung entfernt'); };

/* =====================================================================================================
   Dialogs as choice sheets: a field is a row with its value, ←/→ or OK switches to the next value
   ===================================================================================================== */
let TVD = null;
const tvdCyc = (f, label, value, key, hint) => `<button type="button" class="tvp-cyc" data-act="tvdCyc" data-k="${key}" data-f="${f}" aria-label="${esc(label + ': ' + value)}"><span class="tvp-cl">${label}</span><span class="tvp-cv">${icon('left', 26)}<b>${esc(value)}</b>${icon('right', 26)}</span>${hint ? `<small>${hint}</small>` : ''}</button>`;
const tvdSw = (f, label, on, key, help) => `<button type="button" class="tsw" role="switch" aria-checked="${on}" data-act="tvdTog" data-k="${key}" data-f="${f}"><span class="tsw-tx">${label}${help ? `<small class="tvp-hint">${help}</small>` : ''}</span><span class="sw"></span></button>`;
const tvdBtns = (...b) => `<div class="acts">${b.filter(Boolean).join('')}</div>`;
function tvdKey(name) {
  if (!TVD) return false; const el = F.el;
  if (el && el.classList.contains('tvp-cyc') && el.dataset.k && TVD.o[el.dataset.k] && (name === 'left' || name === 'right')) { tvdStep(el.dataset.k, name === 'left' ? -1 : 1); return true; }
  return false;
}
function tvdStep(k, dir) { const o = TVD.o[k]; o.i = (o.i + dir + o.list.length) % o.list.length; if (TVD.on) TVD.on(k); redrawMenu(TVD.html()); }
ACT.tvdCyc = (d) => tvdStep(d.k, 1);
ACT.tvdTog = (d) => { TVD.t[d.k] = !TVD.t[d.k]; redrawMenu(TVD.html()); };
const tvdVal = (k) => TVD.o[k].list[TVD.o[k].i];
const tvdLab = (k) => tvdVal(k)[1];
const TV_PB = [0, 1, 2, 5, 10, 15, 30]; const TV_PA = [0, 5, 10, 15, 30, 60];
const tvdPad = (list, std) => ({ list: list.map((v) => [v, `${v === std ? 'Standard (' + v + ' Min.)' : v + ' Min.'}`]), i: Math.max(0, list.indexOf(std)) });
const TV_KEEP = [[{ mode: 'forever' }, 'Bis du löschst'], [{ mode: 'days', days: 7 }, '7 Tage'], [{ mode: 'days', days: 14 }, '14 Tage'], [{ mode: 'days', days: 30 }, '30 Tage'], [{ mode: 'days', days: 90 }, '90 Tage']];
const tvdKeep = (cur) => { const i = cur && cur.mode === 'days' ? TV_KEEP.findIndex((k) => k[0].days === cur.days) : 0; return { list: TV_KEEP.map((k, n) => [k[0], n === 0 ? 'Standard (bis du löschst)' : k[1]]), i: Math.max(0, i) }; };
const tvdKeepVal = () => { const k = tvdVal('keep')[0]; return k.mode === 'days' ? { mode: 'days', days: k.days, until: (Math.floor(SIM.now() / 1440) + k.days) * 1440 } : { mode: 'forever' }; };
const tvdBase = (id, d) => { TVD = d; return { dialog: true, html: d.html(), onKey: tvdKey, onClose: () => { if (TVD === d) TVD = null; } }; };

/* ---- LD-01 Aufnahme planen / ändern ---- */
registerLiveDialog('LD-01', (props) => {
  const p = SIM.program(props.programId);
  if (!p) return { dialog: true, html: `<h2>Aufnahme planen</h2><p>Diese Sendung steht nicht mehr im Programm.</p>${tvdBtns(tvBtn({ f: 'd-x', ic: 'close', label: 'Schliessen', act: 'close', def: true }))}` };
  const ex = SIM.rules().find((r) => r.kind === 'single' && r.program === p.id); const via = !ex && SIM.recState(p.id) === 'series' ? SIM.ruleForProgram(p.id) : null;
  const d = { o: { pb: tvdPad(TV_PB, ex ? ex.padBefore : LIVE.settings.dvr.padBefore), pa: tvdPad(TV_PA, ex ? ex.padAfter : LIVE.settings.dvr.padAfter), keep: tvdKeep(ex && ex.keep) }, t: {} };
  d.html = () => {
    const s = tvStat(p); const c = SIM.channel(p.ch); const pb = tvdVal('pb')[0]; const pa = tvdVal('pa')[0]; const from = s === 'now' ? SIM.now() : p.start - pb; const to = p.end + pa; const gb = Math.round((to - from) * 0.06 * 10) / 10; const q = SIM.quota();
    const past = s === 'past';
    return `<h2>${ex ? 'Aufnahme ändern' : 'Aufnahme planen'}</h2><p class="tvp-dsub"><b>${esc(p.title)}</b>${tvSub(p)}<br>${esc(c.name)} · ${esc(tvWhen(p))}</p>
      ${past ? tvNote('alert', 'Diese Sendung ist schon vorbei.', '', 'bad') : `${s === 'now' ? tvNote('info', `Die Sendung läuft seit ${fmt.time(p.start)}. Die Aufnahme beginnt jetzt.`, '') : ''}
      ${via ? tvNote('info', `Diese Sendung ist über die Serienaufnahme «${esc(via.title)}» geplant.`, '') : ''}
      <div class="tvp-fields">${s === 'now' ? '' : tvdCyc('d-pb', 'Vorlauf', tvdLab('pb'), 'pb')}${tvdCyc('d-pa', 'Nachlauf', tvdLab('pa'), 'pa')}${tvdCyc('d-keep', 'Aufbewahren', tvdLab('keep'), 'keep')}</div>
      <p class="tvp-sum">Aufnahme von ${fmt.time(from)} bis ${fmt.time(to)} · etwa ${fmt.gb(gb)}</p>
      ${q.freeGB <= 0 ? tvNote('alert', 'Im Speicher deines Kontos ist kein Platz frei.', 'Beim Start wird die Aufnahme abgelehnt.', 'bad') : q.freeGB < gb ? tvNote('alert', 'Für diese Aufnahme fehlt voraussichtlich Platz im Speicher des Kontos.', 'Ist beim Start nicht genug frei, wird sie nicht gemacht.', 'warn') : ''}
      <p class="tvp-hint">Ändert sich die Sendezeit, passt Slouchcat die Aufnahme an.</p>`}
      ${tvdBtns(tvBtn({ f: 'd-no', ic: 'close', label: 'Abbrechen', act: 'close', cls: 'ghost' }), past ? '' : tvBtn({ f: 'd-ok', ic: 'record', label: ex ? 'Speichern' : s === 'now' ? 'Ab jetzt aufnehmen' : 'Aufnahme planen', act: 'tvdPlanSave', data: { pid: p.id }, cls: 'primary' }), ex ? tvBtn({ f: 'd-del', ic: 'trash', label: 'Aufnahme abbrechen', act: 'tvdPlanDel', data: { id: ex.id }, cls: 'ghost' }) : '', !ex && (p.series || p.episode) ? tvBtn({ f: 'd-ser', ic: 'recordSeries', label: 'Stattdessen die Serie aufnehmen', act: 'tvdToSeries', data: { pid: p.id }, cls: 'ghost sm' }) : '')}`;
  };
  return tvdBase('LD-01', d);
});
ACT.tvdPlanSave = (d) => {
  const p = SIM.program(d.pid); const s = tvStat(p); const ex = SIM.rules().find((r) => r.kind === 'single' && r.program === p.id);
  if (s === 'now' && !ex) { const res = SIM.startInstant(p.ch, { until: 'end', keep: tvdKeepVal() }); closeMenu(); if (res.ok) toast(`Aufnahme läuft bis ${fmt.time(res.end)}.`); else toast('Im Speicher deines Kontos ist kein Platz frei.'); return; }
  SIM.planRecording(p.id, { padBefore: tvdVal('pb')[0], padAfter: tvdVal('pa')[0], keep: tvdKeepVal() }); closeMenu(); toast(ex ? 'Aufnahme geändert' : 'Aufnahme geplant');
};
ACT.tvdPlanDel = (d) => { closeMenu(); LiveDlg.confirmDelete({ kind: 'occurrence', ids: [d.id] }); };
ACT.tvdToSeries = (d) => { closeMenu(); LiveDlg.seriesRecording(d.pid); };

/* ---- LD-02 Serienaufnahme (14 Fassung 2, 5.2: gleicher Titel; Zeitfenster, Wochentage, Erstausstrahlungen als Vorschlag F-23) ---- */
const TV_F23 = 'Vorschlag, hängt an Frage F-23';
registerLiveDialog('LD-02', (props) => {
  const p = SIM.program(props.programId) || LIVE.programs.find((x) => x.series === (SIM.rule(props.ruleId) || {}).series); const rule = props.ruleId ? SIM.rule(props.ruleId) : null;
  if (!p) return { dialog: true, html: `<h2>Serienaufnahme</h2><p>Zu dieser Sendung gibt es keine Serie.</p>${tvdBtns(tvBtn({ f: 'd-x', ic: 'close', label: 'Schliessen', act: 'close', def: true }))}` };
  const progs = LIVE.programs.filter((x) => x.title === p.title);
  const chans = [...new Set(progs.filter((x) => x.start >= SIM.now()).map((x) => x.ch))].map((id) => SIM.channel(id).name);
  const epKnown = progs.some((x) => x.season != null || x.episode != null); const marked = progs.some((x) => x.repeat || x.firstRun);
  const KL = [null, 1, 2, 3, 5, 10, 20];
  const WDS = [['all', 'alle Tage', null], ['wk', 'Mo–Fr', [1, 2, 3, 4, 5]], ['we', 'Sa und So', [6, 0]]];
  const wdI = rule && rule.weekdays ? Math.max(0, WDS.findIndex((w) => w[2] && w[2].join() === rule.weekdays.join())) : 0;
  const d = { o: { scope: { list: [['one', `Nur ${SIM.channel(p.ch).name}`], ['all', 'Alle Sender, die du sehen kannst']], i: rule && rule.channels === 'all' ? 1 : 0 }, win: { list: [['any', 'jederzeit'], ['eve', 'nur zwischen 20:00 und 22:30']], i: rule && rule.window ? 1 : 0 }, wd: { list: WDS, i: wdI }, kl: { list: KL.map((k) => [k, k ? `die letzten ${k}` : 'alle Folgen']), i: Math.max(0, KL.indexOf(rule ? rule.keepLast : null)) }, pb: tvdPad(TV_PB, rule ? rule.padBefore : LIVE.settings.dvr.padBefore), pa: tvdPad(TV_PA, rule ? rule.padAfter : LIVE.settings.dvr.padAfter), keep: tvdKeep(rule && rule.keep) }, t: { onlyNew: rule ? !!rule.onlyNew : false } };
  d.html = () => {
    const all = tvdVal('scope')[0] === 'all'; const t = SIM.now(); const win = tvdVal('win')[0] === 'eve'; const wds = tvdVal('wd')[2];
    const cand = progs.filter((x) => x.start >= t && (all || x.ch === p.ch) && (!d.t.onlyNew || !x.repeat) && (!win || (x.start % 1440 >= 1200 && x.start % 1440 <= 1350)) && (!wds || wds.includes((6 + Math.floor(x.start / 1440)) % 7))).sort((a, b) => a.start - b.start);
    const seen = new Set(); const prev = []; const skip = [];
    cand.forEach((x) => { const key = epKnown && (x.season != null || x.episode != null) ? `${x.season}-${x.episode}` : null; if (key && seen.has(key)) { skip.push(x); return; } if (key) seen.add(key); prev.push(x); });
    return `<h2>${rule ? 'Serienaufnahme ändern' : 'Serie aufnehmen'}</h2><p class="tvp-dsub"><b>${esc(p.seriesName || p.title)}</b><br>Slouchcat nimmt jede Sendung mit gleichem Titel auf · läuft auf ${esc(chans.join(', ') || SIM.channel(p.ch).name)}</p>
      <p class="tvp-hint">${epKnown ? 'Folgen werden erkannt (Staffel und Folge). Schon aufgenommene Folgen werden übersprungen.' : 'Folgen nicht identifizierbar: Wiederholungen werden mit aufgenommen.'}</p>
      <div class="tvp-fields">${tvdCyc('d-sc', 'Sender', tvdLab('scope'), 'scope')}${tvdCyc('d-kl', 'Behalten', tvdLab('kl'), 'kl', tvdVal('kl')[0] ? 'Ist eine neue Folge fertig, löscht Slouchcat die älteste (nach Sendebeginn). Fehlgeschlagene zählen nicht.' : '')}${tvdCyc('d-pb', 'Vorlauf', tvdLab('pb'), 'pb')}${tvdCyc('d-pa', 'Nachlauf', tvdLab('pa'), 'pa')}${tvdCyc('d-keep', 'Aufbewahren', tvdLab('keep'), 'keep')}
      ${marked ? tvdSw('d-new', 'Nur Erstausstrahlungen', d.t.onlyNew, 'onlyNew', TV_F23) : `<button type="button" class="tsw" role="switch" aria-checked="false" aria-disabled="true" data-act="toast" data-msg="Das Programm kennzeichnet Wiederholungen bei diesem Sender nicht." data-f="d-new"><span class="tsw-tx">Nur Erstausstrahlungen<small class="tvp-hint">${TV_F23}. Das Programm kennzeichnet Wiederholungen bei diesem Sender nicht.</small></span><span class="sw"></span></button>`}
      ${tvdCyc('d-win', 'Zeitfenster', tvdLab('win'), 'win', TV_F23)}${tvdCyc('d-wd', 'Wochentage', tvdLab('wd'), 'wd', TV_F23)}</div>
      <p class="tvp-sum">Demnächst: ${prev.slice(0, 3).map((x) => `${fmt.day(x.start)} ${fmt.time(x.start)} ${esc(SIM.channel(x.ch).name)}`).join(' · ') || 'kein Termin'}${skip.length ? ` · (${skip.slice(0, 2).map((x) => `${fmt.time(x.start)} ${esc(SIM.channel(x.ch).name)}`).join(', ')}: gleiche Folge, wird übersprungen)` : ''}</p>
      ${tvdBtns(tvBtn({ f: 'd-no', ic: 'close', label: 'Abbrechen', act: 'close', cls: 'ghost' }), tvBtn({ f: 'd-ok', ic: 'recordSeries', label: rule ? 'Speichern' : 'Serie aufnehmen', act: 'tvdSerSave', data: { pid: p.id, rid: rule ? rule.id : '' }, cls: 'primary' }), rule ? tvBtn({ f: 'd-end', ic: 'close', label: 'Serienaufnahme beenden', act: 'tvdSerEnd', data: { id: rule.id }, cls: 'ghost' }) : '')}`;
  };
  return tvdBase('LD-02', d);
});
ACT.tvdSerSave = (d) => {
  const o = { channels: tvdVal('scope')[0] === 'all' ? 'all' : [SIM.program(d.pid).ch], onlyNew: TVD.t.onlyNew, keepLast: tvdVal('kl')[0], padBefore: tvdVal('pb')[0], padAfter: tvdVal('pa')[0], keep: tvdKeepVal(), window: tvdVal('win')[0] === 'eve' ? { from: '20:00', to: '22:30' } : null, weekdays: tvdVal('wd')[2] };
  if (d.rid) SIM.updateRule(d.rid, o); else SIM.planSeries(d.pid, o); closeMenu(); toast(d.rid ? 'Serienaufnahme geändert' : 'Serie wird aufgenommen');
};
ACT.tvdSerEnd = (d) => { closeMenu(); LiveDlg.confirmDelete({ kind: 'series', ids: [d.id] }); };

/* ---- LD-03 Manuelle oder wiederkehrende Aufnahme (14 Fassung 2, 5.3: wiederkehrend mit Beginn und Dauer, Vorlauf 0, Ende nur durch das Profil) ---- */
const tvT = (m) => `${String(Math.floor(((m % 1440) + 1440) % 1440 / 60)).padStart(2, '0')}:${String(((m % 60) + 60) % 60).padStart(2, '0')}`;
const TV_WD = [[1, 'Montag'], [2, 'Dienstag'], [3, 'Mittwoch'], [4, 'Donnerstag'], [5, 'Freitag'], [6, 'Samstag'], [0, 'Sonntag']];
const TV_DUR = [15, 30, 45, 60, 75, 90, 105, 120, 150, 180, 240];
registerLiveDialog('LD-03', (props) => {
  const rule = props.ruleId ? SIM.rule(props.ruleId) : null; const today = Math.floor(SIM.now() / 1440);
  const gone = rule && (!SIM.channel(rule.ch) || SIM.chState(rule.ch) === 'gone');
  const t0 = rule ? (rule.kind === 'manual' ? rule.start : today * 1440 + LIVE.hm(rule.from)) : props.start != null ? Math.floor(props.start / 15) * 15 : Math.ceil((SIM.now() + 30) / 15) * 15;
  const e0 = rule ? (rule.kind === 'manual' ? rule.end : today * 1440 + LIVE.hm(rule.to)) : props.end != null ? Math.ceil(props.end / 15) * 15 : t0 + 60;
  const dur0 = ((e0 - t0) % 1440 + 1440) % 1440 || 60;
  const days = []; for (let k = 0; k < 7; k++) days.push([today + k, k === 0 ? 'Heute' : k === 1 ? 'Morgen' : fmt.dayRel((today + k) * 1440)]);
  const times = []; for (let m = 0; m < 1440; m += 15) times.push([m, tvT(m)]);
  const REP = [['once', 'einmal'], ['daily', 'täglich'], ['weekdays', 'Mo–Fr'], ['weekly', 'an einem Wochentag']];
  const repI = rule && rule.kind === 'recurring' ? (rule.weekdays && rule.weekdays.length === 7 ? 1 : rule.weekdays && rule.weekdays.join() === '1,2,3,4,5' ? 2 : 3) : 0;
  const wd0 = rule && rule.weekdays && rule.weekdays.length ? rule.weekdays[0] : (6 + today) % 7;
  const durI = TV_DUR.reduce((b, v, i) => (Math.abs(v - dur0) < Math.abs(TV_DUR[b] - dur0) ? i : b), 0);
  const d = { ch: gone ? '' : (rule ? rule.ch : props.ch || ''), name: rule && !gone ? rule.title : null, fixed: rule && rule.kind === 'manual',
    o: { day: { list: days, i: Math.max(0, days.findIndex((x) => x[0] === Math.floor(t0 / 1440))) }, from: { list: times, i: (((t0 % 1440) + 1440) % 1440) / 15 }, to: { list: times, i: (((e0 % 1440) + 1440) % 1440) / 15 }, dur: { list: TV_DUR.map((v) => [v, fmt.dur(v)]), i: durI }, rep: { list: REP, i: repI }, wd: { list: TV_WD, i: Math.max(0, TV_WD.findIndex((w) => w[0] === wd0)) },
      pb: tvdPad(TV_PB, rule ? rule.padBefore || 0 : 0), pa: tvdPad(TV_PA, rule ? rule.padAfter || 0 : 0), kl: { list: [null, 1, 3, 5, 10, 20].map((k) => [k, k ? `die letzten ${k}` : 'alle']), i: rule && rule.keepLast ? Math.max(0, [null, 1, 3, 5, 10, 20].indexOf(rule.keepLast)) : 0 }, keep: tvdKeep(rule && rule.keep) }, t: {} };
  d.auto = () => { const n = d.ch ? SIM.channel(d.ch).name : 'Sender'; const rep = tvdVal('rep')[0]; return rep === 'once' ? `${n}, ${fmt.day(tvdVal('day')[0] * 1440)}, ${tvdLab('from')}` : `${n}, ${rep === 'weekly' ? tvdLab('wd') : tvdLab('rep')} ${tvdLab('from')}`; };
  d.html = () => {
    const rep = d.fixed ? 'once' : tvdVal('rep')[0]; const once = rep === 'once'; const f = tvdVal('from')[0]; const t = tvdVal('to')[0]; const next = once && t <= f; const ch = d.ch && SIM.channel(d.ch);
    const mins = once ? (next ? t + 1440 : t) - f : tvdVal('dur')[0];
    return `<h2>${rule ? 'Aufnahme ändern' : 'Neue Aufnahme'}</h2>
      ${gone ? tvNote('alert', `${esc(rule.title.split(',')[0])} ist nicht mehr im Angebot.`, 'Wähle einen anderen Sender.', 'bad') : ''}
      <div class="tvp-fields"><button type="button" class="tvp-cyc" data-act="tvdPickCh" data-f="d-ch" aria-label="Sender: ${esc(ch ? ch.name : 'wählen')}"><span class="tvp-cl">Sender</span><span class="tvp-cv"><b>${ch ? `${ch.no} ${esc(ch.name)}` : 'Sender wählen …'}</b>${icon('right', 26)}</span></button>
      ${d.fixed ? '' : tvdCyc('d-rep', 'Wiederholen', tvdLab('rep'), 'rep')}${once ? tvdCyc('d-day', 'Datum', tvdLab('day'), 'day') : rep === 'weekly' ? tvdCyc('d-wd', 'Wochentag', tvdLab('wd'), 'wd') : ''}
      ${tvdCyc('d-from', 'Beginn', tvdLab('from'), 'from')}${once ? tvdCyc('d-to', 'Ende', tvdLab('to'), 'to', next ? 'endet am Folgetag' : '') : tvdCyc('d-dur', 'Dauer', tvdLab('dur'), 'dur', 'Die Dauer bleibt auch bei der Zeitumstellung gleich.')}
      ${tvdCyc('d-pb', 'Vorlauf', tvdLab('pb'), 'pb')}${tvdCyc('d-pa', 'Nachlauf', tvdLab('pa'), 'pa')}
      <button type="button" class="tvp-cyc" data-act="tvdName" data-f="d-name" aria-label="Name: ${esc(d.name || d.auto())}"><span class="tvp-cl">Name</span><span class="tvp-cv"><b>${esc(d.name || d.auto())}</b>${icon('edit', 26)}</span></button>
      ${once ? '' : tvdCyc('d-kl', 'Behalten', tvdLab('kl'), 'kl')}${tvdCyc('d-keep', 'Aufbewahren', tvdLab('keep'), 'keep')}</div>
      <p class="tvp-sum">${once ? `Aufnahme von ${tvT(f)} bis ${tvT(t)}${next ? ' (Folgetag)' : ''}` : `${esc(rep === 'weekly' ? 'Jeden ' + tvdLab('wd') : tvdLab('rep'))} ab ${tvT(f)}, ${fmt.dur(mins)}`} · etwa ${fmt.gb(Math.round(mins * 0.06 * 10) / 10)}${once ? '' : ' je Termin'}</p>
      ${once ? '' : `<p class="tvp-hint">Die Aufnahme wiederholt sich, bis du sie beendest. Termine entstehen 14 Tage im Voraus. «Ende der Wiederholung» (am Datum, nach N Aufnahmen): ${TV_F23}.</p>`}
      ${tvdBtns(tvBtn({ f: 'd-no', ic: 'close', label: 'Abbrechen', act: 'close', cls: 'ghost' }), tvBtn({ f: 'd-ok', ic: 'record', label: rule ? 'Speichern' : 'Aufnahme planen', act: 'tvdManSave', data: { rid: rule ? rule.id : '' }, cls: 'primary' }), rule ? tvBtn({ f: 'd-del', ic: 'trash', label: 'Aufnahme beenden', act: 'tvdManDel', data: { id: rule.id }, cls: 'ghost' }) : '')}`;
  };
  return tvdBase('LD-03', d);
});
ACT.tvdPickCh = () => {
  const d = TVD; const chs = SIM.channels(); const cur = d.ch;
  menu(`<h2>Sender wählen</h2><div class="mlist">${chs.map((c, i) => `<button type="button" class="mi" role="menuitemradio" aria-checked="${c.id === cur}" data-act="tvdSetCh" data-ch="${c.id}" data-f="pc-${c.id}"${c.id === cur || (!cur && i === 0) ? ' data-default' : ''}><span class="mi-ic">${LU.logo(c, { cls: 'sm' })}</span><span class="mi-tx"><b>${c.no} ${esc(c.name)}</b></span>${c.id === cur ? icon('check', 34) : ''}</button>`).join('')}</div>`, { onClose: () => { if (TVD === d) redrawMenu(d.html()); } });
};
ACT.tvdSetCh = (d) => { TVD.ch = d.ch; closeMenu(); };
ACT.tvdName = () => { const d = TVD; keyboard({ value: d.name || d.auto(), label: 'Name der Aufnahme', onDone: (v) => { d.name = v.trim() || null; if (TVD === d) redrawMenu(d.html()); } }); };
ACT.tvdManSave = (a) => {
  const d = TVD; if (!d.ch) { toast('Wähle zuerst einen Sender'); return; }
  const f = tvdVal('from')[0]; const t = tvdVal('to')[0]; const day = tvdVal('day')[0]; const rep = d.fixed ? 'once' : tvdVal('rep')[0]; const title = d.name || d.auto();
  const pads = { padBefore: tvdVal('pb')[0], padAfter: tvdVal('pa')[0] };
  if (a.rid) SIM.removeRule(a.rid);
  if (rep === 'once') SIM.planManual({ ch: d.ch, title, start: day * 1440 + f, end: day * 1440 + (t <= f ? t + 1440 : t), keep: tvdKeepVal(), ...pads });
  else { const wds = rep === 'daily' ? [0, 1, 2, 3, 4, 5, 6] : rep === 'weekdays' ? [1, 2, 3, 4, 5] : [tvdVal('wd')[0]]; SIM.planManual({ ch: d.ch, title, from: tvT(f), to: tvT(f + tvdVal('dur')[0]), weekdays: wds, repeat: rep, keepLast: tvdVal('kl')[0], keep: tvdKeepVal(), ...pads }); }
  closeMenu(); toast(a.rid ? 'Aufnahme geändert' : 'Aufnahme geplant');
};
ACT.tvdManDel = (d) => { closeMenu(); LiveDlg.confirmDelete({ kind: 'rule', ids: [d.id] }); };

/* ---- LD-04 Sofortaufnahme im Player (14 Fassung 2, 5.4: bis Sendungsende oder bis ich stoppe; bei Zeitversatz ab jetzt live) ---- */
registerLiveDialog('LD-04', (props) => {
  const c = SIM.channel(props.ch); const nn = SIM.nowNext(c.id).now; const q = SIM.quota(); const pl = SIM.player();
  const pa = LIVE.settings.dvr.padAfter; const maxH = LIVE.settings.dvr.instantMaxH || 4;
  const opts = [nn ? ['end', `Bis Sendungsende (${fmt.time(nn.end)}, mit Nachlauf bis ${fmt.time(nn.end + pa)})`] : null, ['stop', `Bis ich stoppe (höchstens ${maxH} Stunden)`]].filter(Boolean);
  const shifted = !!(pl && pl.ch === c.id && (pl.offset > 0.05 || pl.paused));
  const d = { o: { keep: tvdKeep(null), until: { list: opts, i: 0 } }, t: {} };
  d.html = () => {
    if (q.freeGB <= 0) return `<h2>Jetzt aufnehmen</h2><p class="tvp-dsub"><b>${esc(c.name)}</b></p>${tvNote('alert', `Im Speicher deines Kontos ist kein Platz frei (${Math.round(q.usedGB)} von ${q.totalGB} GB).`, 'Lösche Aufnahmen, um aufzunehmen.', 'bad')}${tvdBtns(tvBtn({ f: 'd-mine', ic: 'film', label: 'Meine Aufnahmen', act: 'tvdToMine', cls: 'primary', def: true }), tvBtn({ f: 'd-x', ic: 'close', label: 'Schliessen', act: 'close', cls: 'ghost' }))}`;
    const head = `<h2>Jetzt aufnehmen</h2><p class="tvp-dsub"><b>${esc(c.name)}</b>${nn ? ` · ${esc(nn.title)} (bis ${fmt.time(nn.end)})` : ''}</p>`;
    const noEpg = nn ? '' : ' Ohne Programmdaten gibt es nur «Bis ich stoppe».';
    if (shifted) return `${head}${tvNote('info', 'Du schaust zeitversetzt. Aufgenommen wird ab dem Live-Zeitpunkt.', '')}
      <div class="tvp-fields">${tvdCyc('d-until', 'Dauer', tvdLab('until'), 'until')}${tvdCyc('d-keep', 'Aufbewahren', tvdLab('keep'), 'keep')}</div>${noEpg ? `<p class="tvp-hint">${noEpg.trim()}</p>` : ''}
      ${tvdBtns(tvBtn({ f: 'd-no', ic: 'close', label: 'Abbrechen', act: 'close', cls: 'ghost' }), tvBtn({ f: 'd-live', ic: 'record', label: 'Ab jetzt aufnehmen (live)', act: 'tvdInstantLive', data: { ch: c.id }, cls: 'primary', def: true }))}`;
    return `${head}
      <div class="mlist">${opts.map(([v, t], i) => `<button type="button" class="mi" data-act="tvdInstant" data-v="${v}" data-ch="${c.id}" data-f="in-${i}"${i === 0 ? ' data-default' : ''}><span class="mi-ic">${icon('record', 36)}</span><span class="mi-tx"><b>${esc(t)}</b></span></button>`).join('')}</div>
      <div class="tvp-fields">${tvdCyc('d-keep', 'Aufbewahren', tvdLab('keep'), 'keep')}</div>
      <p class="tvp-hint">Die Aufnahme beginnt jetzt (${fmt.time(SIM.now())}), nicht am Anfang der Sendung.${noEpg}</p>
      ${tvdBtns(tvBtn({ f: 'd-no', ic: 'close', label: 'Abbrechen', act: 'close', cls: 'ghost' }))}`;
  };
  return tvdBase('LD-04', d);
});
ACT.tvdInstant = (d) => { const res = SIM.startInstant(d.ch, { until: d.v, keep: tvdKeepVal() }); closeMenu(); if (res.ok) toast(`Aufnahme läuft bis ${fmt.time(res.end)}.`); else toast('Im Speicher deines Kontos ist kein Platz frei.'); };
ACT.tvdInstantLive = (d) => ACT.tvdInstant({ ch: d.ch, v: tvdVal('until')[0] });
ACT.tvdToMine = () => { closeAll(); go('#/live/recordings'); };

/* ---- LD-05 Erinnerung (Vorlauf 0, 5, 15, 30 Minuten, 03 DR-08) ---- */
registerLiveDialog('LD-05', (props) => {
  const p = SIM.program(props.programId);
  if (!p || tvStat(p) !== 'future') return { dialog: true, html: `<h2>Erinnern</h2><p>${p ? 'Diese Sendung hat schon begonnen.' : 'Diese Sendung steht nicht mehr im Programm.'} Erinnerungen gibt es nur für künftige Sendungen.</p>${tvdBtns(tvBtn({ f: 'd-x', ic: 'close', label: 'Schliessen', act: 'close', def: true }))}` };
  const ex = SIM.reminderFor(p.id); const L = [0, 5, 15, 30];
  const d = { o: { lead: { list: L.map((v) => [v, v ? `${v} Min. vorher${v === 5 ? ' (Standard)' : ''}` : 'zum Beginn']), i: ex ? Math.max(0, L.indexOf(ex.lead)) : 1 } }, t: {} };
  d.html = () => `<h2>${ex ? 'Erinnerung ändern' : 'Erinnern'}</h2><p class="tvp-dsub"><b>${esc(p.title)}</b>${tvSub(p)}<br>${esc(tvWhen(p))} · ${esc(SIM.channel(p.ch).name)}</p>
    <div class="tvp-fields">${tvdCyc('d-lead', 'Erinnern', tvdLab('lead'), 'lead', `um ${fmt.time(p.start - tvdVal('lead')[0])}`)}</div>
    <p class="tvp-hint">Du bekommst die Erinnerung in Slouchcat und als Mitteilung auf deinen Geräten.</p>
    ${tvdBtns(tvBtn({ f: 'd-no', ic: 'close', label: 'Abbrechen', act: 'close', cls: 'ghost' }), tvBtn({ f: 'd-ok', ic: 'reminder', label: ex ? 'Speichern' : 'Erinnern', act: 'tvdRemSave', data: { pid: p.id }, cls: 'primary' }), ex ? tvBtn({ f: 'd-rm', ic: 'trash', label: 'Erinnerung entfernen', act: 'tvdRemDel', data: { pid: p.id }, cls: 'ghost' }) : '')}`;
  return tvdBase('LD-05', d);
});
ACT.tvdRemSave = (d) => { SIM.setReminder(d.pid, tvdVal('lead')[0]); closeMenu(); toast('Erinnerung gespeichert'); };
ACT.tvdRemDel = (d) => { SIM.removeReminder(d.pid); closeMenu(); toast('Erinnerung entfernt'); };

/* ---- LD-06 Löschen und Abbrechen (Fokus startet auf dem harmlosen Knopf) ---- */
registerLiveDialog('LD-06', (props) => {
  const kind = props.kind || 'recording'; const ids = props.ids || []; const recs = ids.map((id) => SIM.recording(id)).filter(Boolean); const r0 = recs[0];
  const d = { o: {}, t: { alsoDel: false } };
  const done = () => { closeMenu(); if (props.onDone) props.onDone(); };
  d.ids = ids; d.kind = kind; d.done = done;
  d.html = () => {
    const cancel = (def) => tvBtn({ f: 'd-no', ic: 'back', label: kind === 'series' ? 'Behalten' : kind === 'occurrence' || kind === 'rule' ? 'Behalten' : 'Abbrechen', act: 'close', def });
    if (['recording', 'recordings', 'childRecording'].includes(kind)) {
      const gb = recs.reduce((s, r) => s + (r.sizeGB || 0), 0); const ok = recs.every((r) => ['done', 'incomplete', 'running'].includes(r.state)); const owner = r0 && SIM.profiles().find((p) => p.id === r0.profile);
      return `<h2>${recs.length > 1 ? `${recs.length} Aufnahmen` : ok ? 'Aufnahme löschen' : 'Aufnahme entfernen'}</h2><p>${recs.length > 1 ? `${recs.length} Aufnahmen (${fmt.gb(gb)}) löschen?` : `«${esc(r0 ? r0.title : '')}»${ok ? ` (${fmt.gb(gb)})` : ''} ${ok ? 'löschen?' : 'aus der Liste entfernen?'}`} ${ok ? 'Die Aufnahme wird endgültig entfernt und lässt sich nicht wiederherstellen.' : ''}</p>${kind === 'childRecording' && owner ? `<p>${esc(owner.name)} sieht die Aufnahme danach nicht mehr.</p>` : ''}${tvdBtns(cancel(true), tvBtn({ f: 'd-go', ic: 'trash', label: ok ? 'Löschen' : 'Entfernen', act: 'tvdDelRec', cls: 'primary' }))}`;
    }
    if (kind === 'running') return `<h2>Aufnahme beenden</h2><p>«${esc(r0 ? r0.title : '')}» läuft noch. Beenden, behalten oder auch löschen?</p>${tvdBtns(tvBtn({ f: 'd-no', ic: 'back', label: 'Weiter aufnehmen', act: 'close', def: true }), tvBtn({ f: 'd-stop', ic: 'stop', label: 'Nur stoppen, behalten', act: 'tvdStop', cls: 'primary' }), tvBtn({ f: 'd-sdel', ic: 'trash', label: 'Stoppen und löschen', act: 'tvdStopDel', cls: 'ghost' }))}`;
    if (kind === 'occurrence') {
      const rid = String(ids[0]).split(':')[0]; const rule = SIM.rule(rid); const u = SIM.upcoming().find((x) => x.id === ids[0]);
      if (!rule) return `<h2>Termin</h2><p>Dieser Termin ist schon weg.</p>${tvdBtns(tvBtn({ f: 'd-x', ic: 'close', label: 'Schliessen', act: 'close', def: true }))}`;
      const rep = rule.kind === 'series' || rule.kind === 'recurring';
      return `<h2>Aufnahme nicht machen?</h2><p>«${esc(u ? u.title : rule.title)}» ${u ? fmt.when(u.start) : ''} nicht aufnehmen?</p>${tvdBtns(tvBtn({ f: 'd-no', ic: 'back', label: 'Behalten', act: 'close', def: true }), tvBtn({ f: 'd-go', ic: 'close', label: rep ? 'Nur diesen Termin auslassen' : 'Nicht aufnehmen', act: rep ? 'tvdSkip' : 'tvdRuleEnd', cls: 'primary' }), rep ? tvBtn({ f: 'd-rule', ic: 'trash', label: 'Ganze Regel beenden', act: 'tvdRuleEnd', cls: 'ghost' }) : '')}`;
    }
    if (kind === 'series' || kind === 'rule') {
      const rule = SIM.rule(ids[0]); const rs = SIM.recordings().filter((x) => rule && x.rule === rule.id); const gb = rs.reduce((s, x) => s + x.sizeGB, 0); const n = SIM.upcoming().filter((x) => rule && x.rule.id === rule.id).length;
      return `<h2>${kind === 'series' ? 'Serienaufnahme beenden' : 'Aufnahme beenden'}</h2><p>«${esc(rule ? rule.title : '')}» beenden? ${n} geplante${n === 1 ? 'r Termin entfällt' : ' Termine entfallen'}.</p>${kind === 'series' && rs.length ? `<div class="tvp-fields">${tvdSw('d-alsodel', `Auch meine ${rs.length} Aufnahmen löschen (${fmt.gb(gb)})`, d.t.alsoDel, 'alsoDel', 'Aus: die Aufnahmen bleiben erhalten')}</div>` : ''}${tvdBtns(tvBtn({ f: 'd-no', ic: 'back', label: 'Behalten', act: 'close', def: true }), tvBtn({ f: 'd-go', ic: 'close', label: 'Beenden', act: 'tvdRuleEnd', cls: 'primary' }))}`;
    }
    return `<h2>Löschen</h2><p>Bitte wähle.</p>${tvdBtns(cancel(true))}`;
  };
  return tvdBase('LD-06', d);
});
ACT.tvdDelRec = () => { const d = TVD; const n = d.ids.length; SIM.deleteRecordings(d.ids); d.done(); toast(n > 1 ? `${n} Aufnahmen gelöscht` : 'Aufnahme gelöscht'); };
ACT.tvdStop = () => { const d = TVD; d.ids.forEach((id) => SIM.stopRecording(id)); d.done(); toast('Aufnahme beendet, behalten'); };
ACT.tvdStopDel = () => { const d = TVD; d.ids.forEach((id) => SIM.stopRecording(id)); SIM.deleteRecordings(d.ids); d.done(); toast('Aufnahme gestoppt und gelöscht'); };
ACT.tvdSkip = () => { const d = TVD; SIM.skipOccurrence(d.ids[0]); d.done(); toast('Dieser Termin entfällt'); };
ACT.tvdRuleEnd = () => { const d = TVD; const rid = String(d.ids[0]).split(':')[0]; const rule = SIM.rule(rid); if (d.kind === 'series' && d.t.alsoDel && rule) SIM.deleteRecordings(SIM.recordings().filter((x) => x.rule === rule.id).map((x) => x.id)); SIM.removeRule(rid); d.done(); toast('Aufnahme beendet'); };

/* ---- LD-07 Konflikt oder Warnung ---- */
registerLiveDialog('LD-07', (props) => {
  const w = LIVE.warnings.find((x) => x.id === props.warningId) || LIVE.warnings[0]; const KIND = { quota: 'Kontingent', serverStorage: 'Speicher des Servers', overlap: 'Überschneidung', duplicate: 'Doppelt', channelGone: 'Sender weg', programGone: 'Sendung verschwunden', timeChanged: 'Zeit geändert' };
  const rid = w.rules[0]; const d = { o: {}, t: {} };
  const b = { quota: [tvBtn({ f: 'd-a', ic: 'film', label: 'Aufnahmen aufräumen', act: 'tvdWarnGo', data: { to: '#/live/recordings' }, cls: 'primary', def: true }), tvBtn({ f: 'd-b', ic: 'check', label: 'Trotzdem behalten', act: 'close', cls: 'ghost' })],
    overlap: [tvBtn({ f: 'd-a', ic: 'close', label: 'Eine abbrechen …', act: 'tvdWarnPick', data: { id: w.id }, cls: 'primary', def: true }), tvBtn({ f: 'd-b', ic: 'check', label: 'Beide behalten', act: 'tvdWarnOk', data: { id: w.id }, cls: 'ghost' })],
    duplicate: [tvBtn({ f: 'd-a', ic: 'trash', label: 'Einzelaufnahme entfernen', act: 'tvdWarnRm', data: { id: rid }, cls: 'primary', def: true }), tvBtn({ f: 'd-b', ic: 'check', label: 'Behalten', act: 'close', cls: 'ghost' })],
    channelGone: [tvBtn({ f: 'd-a', ic: 'tv', label: 'Anderen Sender wählen', act: 'tvdWarnCh', data: { id: rid }, cls: 'primary', def: true }), tvBtn({ f: 'd-b', ic: 'close', label: 'Aufnahme abbrechen', act: 'tvdWarnRm', data: { id: rid }, cls: 'ghost' })],
    programGone: [tvBtn({ f: 'd-a', ic: 'record', label: 'Zeit trotzdem aufnehmen', act: 'tvdWarnCh', data: { id: rid }, cls: 'primary', def: true }), tvBtn({ f: 'd-b', ic: 'trash', label: 'Entfernen', act: 'tvdWarnRm', data: { id: rid }, cls: 'ghost' })] }[w.kind] || [tvBtn({ f: 'd-a', ic: 'check', label: 'OK', act: 'tvdWarnOk', data: { id: w.id }, cls: 'primary', def: true })];
  d.html = () => `<h2>${esc(KIND[w.kind] || 'Hinweis')}</h2><p>${esc(w.text)}</p>${w.kind === 'serverStorage' || w.kind === 'timeChanged' ? '' : ''}${tvdBtns(...b)}${w.kind !== 'quota' && w.kind !== 'timeChanged' ? '' : ''}`;
  return tvdBase('LD-07', d);
});
ACT.tvdWarnGo = (d) => { closeMenu(); go(d.to); };
ACT.tvdWarnOk = (d) => { SIM.dismissWarning(d.id); closeMenu(); toast('Hinweis ausgeblendet'); };
ACT.tvdWarnRm = (d) => { closeMenu(); LiveDlg.confirmDelete({ kind: 'rule', ids: [d.id] }); };
ACT.tvdWarnCh = (d) => { closeMenu(); LiveDlg.manualRecording({ ruleId: d.id }); };
ACT.tvdWarnPick = (d) => {
  const w = LIVE.warnings.find((x) => x.id === d.id); closeMenu();
  menu(`<h2>Welche Aufnahme abbrechen?</h2><div class="mlist">${w.rules.map((id, i) => { const rl = SIM.rule(id); return rl ? tvMi('wp-' + i, 'record', esc(rl.title), fmt.when(rl.start || 0), 'tvdWarnRm', { id }, i === 0) : ''; }).join('')}</div>`);
};

/* ---- LD-08 Tag und Uhrzeit wählen: zwei Seitenmenüs nacheinander ---- */
registerLiveDialog('LD-08', (props) => {
  const today = Math.floor(SIM.now() / 1440); const days = []; for (let d = today; d < LIVE.EPG_END / 1440; d++) days.push(d);
  tvT8 = props;
  return { dialog: false, html: `<h2>Tag wählen</h2><div class="mlist">${days.map((d, i) => `<button type="button" class="mi" role="menuitemradio" aria-checked="${d === (props.day != null ? props.day : today)}" data-act="tvdDay" data-day="${d}" data-f="td-${d}"${d === (props.day != null ? props.day : today) ? ' data-default' : ''}><span class="mi-tx"><b>${d === today ? 'Heute' : d === today + 1 ? 'Morgen' : fmt.day(d * 1440)}</b><small>${esc(fmt.dayLong(d * 1440))}</small></span>${d === (props.day != null ? props.day : today) ? icon('check', 34) : ''}</button>`).join('')}</div>` };
});
let tvT8 = {};
ACT.tvdDay = (d) => {
  const day = Number(d.day); const cur = tvT8.time != null ? tvT8.time : 1215; closeMenu();
  const times = []; for (let m = 0; m < 1440; m += 30) times.push(m);
  const sel = Math.round(cur / 30) * 30;
  menu(`<h2>Uhrzeit wählen</h2><div class="mlist">${times.map((m) => `<button type="button" class="mi" data-act="tvdTime" data-day="${day}" data-m="${m}" data-f="tt-${m}"${m === sel ? ' data-default' : ''}><span class="mi-tx"><b>${tvT(m)}</b></span>${m === sel ? icon('check', 34) : ''}</button>`).join('')}</div>`);
};
ACT.tvdTime = (d) => { const o = tvT8; closeMenu(); if (o.onPick) o.onPick({ day: Number(d.day), time: Number(d.m) }); else toast(`Gewählt: ${fmt.day(Number(d.day) * 1440)} ${tvT(Number(d.m))}`); };

/* ---- LD-11 Aufbewahrung ändern ---- */
registerLiveDialog('LD-11', (props) => {
  const r = SIM.recording(props.recordingId);
  if (!r) return { dialog: true, html: `<h2>Aufbewahren</h2><p>Diese Aufnahme gibt es nicht mehr.</p>${tvdBtns(tvBtn({ f: 'd-x', ic: 'close', label: 'Schliessen', act: 'close', def: true }))}` };
  const rule = r.rule ? SIM.rule(r.rule) : null; const d = { o: { keep: tvdKeep(r.keep) }, t: { always: !!r.keepAlways } };
  d.html = () => `<h2>Aufbewahren</h2><p class="tvp-dsub"><b>${esc(r.title)}</b> · ${esc(fmt.day(r.start))}</p>
    <div class="tvp-fields">${tvdCyc('d-keep', 'Aufbewahren', tvdVal('keep')[1] === 'Standard (bis du löschst)' ? 'bis du löschst' : tvdLab('keep') + ' ab Aufnahme', 'keep')}${rule && rule.keepLast ? tvdSw('d-always', `Diese Folge immer behalten (zählt nicht zu den ${rule.keepLast})`, d.t.always, 'always') : ''}</div>
    <p class="tvp-hint">Abgelaufene Aufnahmen löscht Slouchcat automatisch, weil du es so eingestellt hast.</p>
    ${tvdBtns(tvBtn({ f: 'd-no', ic: 'close', label: 'Abbrechen', act: 'close', cls: 'ghost' }), tvBtn({ f: 'd-ok', ic: 'check', label: 'Speichern', act: 'tvdKeepSave', data: { id: r.id }, cls: 'primary' }))}`;
  return tvdBase('LD-11', d);
});
ACT.tvdKeepSave = (d) => { const r = SIM.recording(d.id); r.keepAlways = TVD.t.always; SIM.setKeep(d.id, tvdKeepVal()); closeMenu(); toast('Aufbewahrung gespeichert'); };

/* =====================================================================================================
   Kleine Seiten: LS-21 Profilformular (Block «Live-TV»), LS-23 Glocke, LS-24 Suche, LS-26 Startseite
   ===================================================================================================== */
route('/settings/profiles', () => {
  const me_ = SIM.profile(); const sel = SIM.profiles().find((p) => p.id === (TVS.pf || me_.id)) || me_; TVS.pf = sel.id; const open = !!TVS.open.pfLive;
  const raw = SIM.rawRights('profile', sel.id); const acc = SIM.rawRights('account'); const prim = me_.primary;
  const rr = SIM.rights(); const kidsRec = prim && rr.liveTv && SIM.childProfiles().some((p) => SIM.recordings(p.id).length);
  const sum = `Live-TV: ${raw.liveTv ? 'an' : 'aus'} · Aufnahmen: ${raw.recordings ? 'an' : 'aus'} · Timeshift: ${raw.timeshift ? 'an' : 'aus'}`;
  const row = (k, label, help, needs) => { const capped = !acc[k]; const dep = needs && !raw[needs]; const dis = capped || dep || !prim; const on = raw[k] && acc[k];
    return `<button type="button" class="tsw" role="switch" aria-checked="${on}"${dis ? ' aria-disabled="true"' : ''} data-act="tvPfSw" data-k="${k}" data-f="pf-${k}"><span class="tsw-tx">${label}<small class="tvp-hint">${capped ? esc(LU.TX.capped) : dep ? 'Zuerst Live-TV einschalten.' : !prim ? 'Nur das Hauptprofil ändert Profile.' : help}</small></span><span class="sw"></span></button>`; };
  return { nav: '', html: `<div class="page"><div class="ltv-h"><h1>Profile verwalten</h1></div>
    <div class="ltv-tools tvp-pfs">${SIM.profiles().map((p) => `<button type="button" class="tb ${p.id === sel.id ? 'primary' : 'ghost'}" data-act="tvPfSel" data-id="${p.id}" data-f="pf-p-${p.id}" aria-pressed="${p.id === sel.id}"><span class="avatar" style="background:var(--${AVC[p.id]});width:44px;height:44px;font-size:20px">${esc(p.name[0])}</span><span>${esc(p.name)}${p.age ? ` · ab ${p.age}` : ''}</span></button>`).join('')}</div>
    ${prim ? `<button type="button" class="tvp-fold" data-act="tvPfOpen" data-f="pf-live" aria-expanded="${open}" data-default>${icon(open ? 'down' : 'right', 32)}<b>Live-TV</b><span>${sum}</span></button>
    ${open ? `<div class="tvp-fields">${row('liveTv', 'Live-TV', 'Fernsehen schauen und das Programm sehen.')}${row('recordings', 'Aufnahmen', 'Sendungen aufnehmen. Aufnahmen belegen Speicher des Kontos.', 'liveTv')}${row('timeshift', 'Timeshift', 'Live-TV anhalten und zurückspulen, bis zum eigenen Einschalten.', 'liveTv')}${sel.age ? tvNote('info', esc(LU.TX.noAgeCheck), '') : ''}</div>` : ''}
    ${kidsRec ? `<a class="tvp-link" href="#/live/recordings/profiles" data-f="pf-kids"><span class="mi-ic">${icon('users', 36)}</span><span><b>Aufnahmen meiner Profile</b><small>${SIM.childProfiles().filter((p) => SIM.recordings(p.id).length).map((p) => esc(p.name)).join(', ')}</small></span></a>` : ''}`
    : tvNote('info', 'Nur das Hauptprofil ändert Profile.', '')}
    <p class="ltv-sample">${esc(LU.TX.sample)}</p></div>`, key: tvKeyGuard };
});
ACT.tvPfSel = (d) => { TVS.pf = d.id; render(); };
ACT.tvPfOpen = () => { TVS.open.pfLive = !TVS.open.pfLive; render(); };
ACT.tvPfSw = (d, el) => {
  if (el.getAttribute('aria-disabled') === 'true') { toast(el.querySelector('.tvp-hint').textContent); return; }
  const id = TVS.pf; const raw = SIM.rawRights('profile', id); const p = SIM.profiles().find((x) => x.id === id); const on = !raw[d.k];
  const apply = () => { SIM.setRight('profile', d.k, on, id); if (!on && d.k === 'liveTv') { SIM.setRight('profile', 'recordings', false, id); SIM.setRight('profile', 'timeshift', false, id); } toast(`${d.k === 'liveTv' ? 'Live-TV' : d.k === 'recordings' ? 'Aufnahmen' : 'Timeshift'} ${on ? 'an' : 'aus'} für ${p.name}`); };
  if (on && d.k === 'liveTv' && p.age) { menu(`<h2>Live-TV für ${esc(p.name)} einschalten?</h2><p>${esc(p.name)} sieht dann alle Sender und Sendungen ohne Altersprüfung.</p>${tvdBtns(tvBtn({ f: 'pc-no', ic: 'back', label: 'Abbrechen', act: 'close', def: true }), tvBtn({ f: 'pc-ok', ic: 'check', label: 'Einschalten', act: 'tvPfConfirm', cls: 'primary' }))}`, { dialog: true }); tvPfPending = apply; } else apply();
};
let tvPfPending = null;
ACT.tvPfConfirm = () => { const f = tvPfPending; tvPfPending = null; closeMenu(); if (f) f(); };

route('/notifications', () => {
  const r = SIM.rights(); const items = [];
  if (r.liveTv) SIM.reminders().forEach((m) => { const p = SIM.program(m.program); items.push({ ic: 'reminder', t: `${p.title} beginnt ${m.lead ? `in ${m.lead} Min.` : 'jetzt'} auf ${SIM.channel(p.ch).name}`, s: fmt.when(p.start - m.lead), to: p.start <= SIM.now() ? '#/live/watch/' + p.ch : '#/live/programs/' + p.id }); });
  if (r.recordings) SIM.recordings().forEach((x) => { if (x.state === 'done' && x.id === 'rec-arena') items.push({ ic: 'record', t: `Aufnahme fertig: ${x.title}`, s: fmt.ago(x.end), to: '#/live/recordings/' + x.id }); if (x.state === 'incomplete') items.push({ ic: 'record', t: `Aufnahme unvollständig: ${x.title}`, s: fmt.ago(x.end), to: '#/live/recordings/' + x.id }); if (x.state === 'failed') items.push({ ic: 'alert', t: `Aufnahme fehlgeschlagen: ${x.title}`, s: fmt.ago(x.end), to: '#/live/recordings/' + x.id }); if (x.state === 'rejected') items.push({ ic: 'alert', t: `Aufnahme nicht möglich: ${x.title} (Speicher des Kontos voll)`, s: fmt.ago(x.end), to: '#/live/recordings/' + x.id }); });
  return { nav: '', html: `<div class="page"><div class="ltv-h"><h1>Benachrichtigungen</h1></div><div class="tvp-res">${items.length ? items.map((n, i) => `<div class="tvp-prw"><a class="tvp-pr" href="${n.to}" data-f="nt-${i}"${i === 0 ? ' data-default' : ''}><span class="tvp-pc">${icon(n.ic, 44)}</span><span class="tvp-px wide"><b>${esc(n.t)}</b><small>${esc(n.s)}</small></span></a></div>`).join('') : tvNote('info', 'Keine Benachrichtigungen.', '')}</div><p class="ltv-sample">${esc(LU.TX.sample)}</p></div>`, key: tvKeyGuard };
});
route('/search', (pm, query) => {
  const q = query.q || TVS.sq2 || ''; TVS.sq2 = q; const r = SIM.rights();
  return { nav: 'search', html: `<div class="page"><div class="ltv-h"><h1>Suche</h1></div><button type="button" class="tf ltv-search" data-act="tvSearchKb" data-f="se-q" data-default>${icon('search', 36)}${q ? `<span class="tf-v">${esc(q)}</span>` : '<span class="tf-ph">Titel suchen</span>'}</button>
    ${q ? `<p class="tvp-lead">Die Treffer aus den Bibliotheken bleiben, wie sie sind.</p>` : ''}
    ${q && r.liveTv ? `<a class="tvp-link" href="#/live/guide/search?q=${encodeURIComponent(q)}" data-f="se-live"><span class="mi-ic">${icon('live', 40)}</span><span><b>Im TV-Programm suchen: «${esc(q)}»</b><small>Sendungen der nächsten 7 Tage</small></span></a>` : ''}<p class="ltv-sample">${esc(LU.TX.sample)}</p></div>`, key: tvKeyGuard };
});
ACT.tvSearchKb = () => keyboard({ value: TVS.sq2 || '', label: 'Suchen', placeholder: 'zum Beispiel Tatort', onDone: (v) => { TVS.sq2 = v.trim(); go('#/search?q=' + encodeURIComponent(TVS.sq2), { replace: true }); } });
route('/', () => ({ nav: 'start', html: `<div class="page soon"><span class="soon-ic">${icon('home', 88)}</span><h1>Startseite</h1><p>Diese Seite bleibt, wie sie ist.</p>${SIM.rights().liveTv ? `<button type="button" class="tb primary" data-act="go" data-to="#/live" data-f="home-live" data-default>${icon('live', 32)}<span>Zu Live-TV</span></button>` : ''}</div>`, key: tvKeyGuard }));
