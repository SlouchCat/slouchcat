/* TV, Teil 1: gemeinsame Helfer der Live-TV-Seiten der TV-App und die Seiten LS-01 Start, LS-03 Meine Sender,
   LS-05 Programmsuche, LS-06 Sendungsdetails (Seite und Seitenmenü), LS-15 Senderprogramm.
   Vorlage: LS-02 in channels.js. Alle globalen Namen beginnen mit «tv». 14-oberflaeche.md 3.1, 3.3, 3.5, 3.6, 3.15. */
'use strict';

/* icons recordSeries, stop: in core.js */

const TV_USED0 = LIVE.account.usedGB;
const TVS = { sq: { sig: null }, search: { q: '', when: 'week', genre: '', group: '' }, own: { order: false, held: null, orig: null }, open: {}, ctab: 'upcoming' };
RESETS.push(() => { TVS.sq = { sig: null }; TVS.search = { q: '', when: 'week', genre: '', group: '' }; TVS.own = { order: false, held: null, orig: null }; TVS.open = {}; TVS.ctab = 'upcoming'; });

/* ---------- small helpers ---------- */
const tvAllIds = () => SIM.channels().map((c) => c.id);
const tvStat = (p) => { const t = SIM.now(); return p.end <= t ? 'past' : p.start <= t ? 'now' : 'future'; };
const tvSub = (p) => (p.sub ? ` «${esc(p.sub)}»` : '');
const tvDataAttrs = (o) => Object.entries(o || {}).map(([k, v]) => ` data-${k}="${esc(v)}"`).join('');
function tvBtn(o) {
  return `<button type="button" class="tb ${o.cls || ''}" data-act="${o.act}"${tvDataAttrs(o.data)} data-f="${o.f}"${o.def ? ' data-default' : ''}${o.pressed != null ? ` aria-pressed="${o.pressed}"` : ''}>${o.ic ? icon(o.ic, o.sz || 32) : ''}<span>${o.label}</span></button>`;
}
const tvListFor = (kind) => (kind === 'fav' ? SIM.favorites() : kind === 'recent' ? SIM.recent().map((x) => x.ch.id) : tvAllIds());
const tvSkel = (n, cls) => `<div class="tvp-skels">${Array.from({ length: n }, () => `<span class="lv-skel ${cls || 'tvp-skel'}"></span>`).join('')}</div>`;
/** loading and error of a page (empty is page specific); null in the normal case */
function tvStates(o) {
  o = o || {}; const m = SIM.mode();
  if (m === 'loading') return tvSkel(o.n || 6, o.cls);
  if (m === 'error') return `<div class="tnote bad">${icon('alert', 36)}<div><b>${esc(o.err || 'Live-TV konnte nicht geladen werden.')}</b><p>Der Server hat nicht geantwortet.</p></div></div><div class="ltv-acts"><button type="button" class="tb primary" data-act="ltvRetry" data-f="tvp-retry" data-default>${icon('refresh', 32)}<span>Erneut versuchen</span></button></div>`;
  return null;
}
const tvNote = (ic, b, p, cls) => `<div class="tnote ${cls || ''}">${icon(ic, 36)}<div><b>${b}</b>${p ? `<p>${p}</p>` : ''}</div></div>`;
const tvNoRight = () => !SIM.rights().recordings;

/* ---------- programme row (search, channel programme, reminders, schedule) ---------- */
function tvpRowActs(p) {
  const r = SIM.rights(); const s = tvStat(p); const a = [];
  if (s === 'now') a.push({ k: 'tune', ic: 'play', act: 'tvpTuneProg', label: `${p.title} einschalten` });
  if (r.recordings && s !== 'past') { const rs = SIM.recState(p.id); a.push({ k: 'rec', ic: 'record', act: 'tvpRecRow', label: rs ? `Aufnahme von ${p.title} ändern` : `${p.title} aufnehmen`, on: !!rs }); }
  if (s === 'future') a.push({ k: 'rem', ic: 'reminder', act: 'tvpRemRow', label: SIM.reminderFor(p.id) ? `Erinnerung an ${p.title} ändern` : `An ${p.title} erinnern`, on: !!SIM.reminderFor(p.id) });
  return a;
}
function tvpProgRow(p, o) {
  o = o || {}; const c = SIM.channel(p.ch); const s = tvStat(p);
  const acts = tvpRowActs(p).map((a) => `<button type="button" class="tvp-act${a.on ? ' on' : ''}" data-act="${a.act}" data-pid="${p.id}" data-ch="${p.ch}" data-f="${o.f}-${a.k}" aria-label="${esc(a.label)}">${icon(a.ic, 40)}</button>`).join('');
  return `<div class="tvp-prw"><button type="button" class="tvp-pr ${s}" data-act="tvDetail" data-pid="${p.id}" data-f="${o.f}"${o.def ? ' data-default' : ''} aria-label="${esc(`${fmt.range(p.start, p.end)} ${c.name} ${p.title}`)}">
    <span class="tvp-pt"><b>${fmt.range(p.start, p.end)}</b>${s === 'now' ? LU.progress(p.start, p.end) : s === 'past' ? '<small>vorbei</small>' : `<small>${fmt.dur(p.end - p.start)}</small>`}</span>
    ${o.noCh ? '' : `<span class="tvp-pc">${LU.logo(c, { cls: 'sm' })}<span>${esc(c.name)}</span></span>`}
    <span class="tvp-px"><b>${esc(p.title)}</b>${p.sub ? `<small>${esc(p.sub)}</small>` : `<small>${esc(p.genre)}</small>`}</span>
    <span class="tvp-pb">${LU.programBadges(p, {})}</span></button>${acts}</div>`;
}
ACT.tvpTuneProg = (d) => liveTune(d.ch, tvAllIds());
ACT.tvpRecRow = (d) => LiveDlg.planRecording(d.pid);
ACT.tvpRemRow = (d) => LiveDlg.reminder(d.pid);
ACT.tvDetail = (d) => tvProgramMenu(d.pid);

/* ---------- LS-06 programme details: side menu and page share the content ---------- */
function tvProgInfo(p) {
  const c = SIM.channel(p.ch); const s = tvStat(p); const st = SIM.chState(c);
  const facts = [['Folge', p.season && p.episode ? `Staffel ${p.season}, Folge ${p.episode}` : p.episode ? `Folge ${p.episode}` : ''], ['Jahr', p.year], ['Land', p.country], ['Regie', p.director]].filter(([, v]) => v);
  const chips = [p.genre, ...(p.flags || []), p.repeat ? 'Wiederholung' : ''].filter(Boolean);
  const rec = s === 'past' ? SIM.recordings().find((r) => r.program === p.id) : null;
  return `<div class="tvp-pi-top">${LU.logo(c, { cls: 'big' })}<span><b>${c.no} ${esc(c.name)}</b><small>${esc(tvWhen(p))} · ${fmt.dur(p.end - p.start)}</small></span></div>
    <h2 class="tvp-pi-t">${esc(p.title)}${p.sub ? `<span> «${esc(p.sub)}»</span>` : ''}</h2>
    <p class="tvp-chips">${chips.map((x) => `<span class="badge">${esc(x)}</span>`).join('')}${LU.programBadges(p, { text: true })}</p>
    ${s === 'now' ? `<p class="tvp-pi-st">Läuft · ${LU.nowText('left', p.end)}</p>${LU.progress(p.start, p.end)}` : s === 'past' ? `<p class="tvp-pi-st">${esc(LU.TX.past)}</p>` : ''}
    ${st !== 'ok' && s === 'now' ? tvNote('alert', `${esc(c.name)} ist gerade nicht erreichbar.`, 'Einschalten geht trotzdem, Slouchcat versucht es weiter.', 'warn') : ''}
    ${p.desc ? `<p class="tvp-pi-d">${esc(p.desc)}</p>` : ''}
    ${facts.length ? `<dl class="tvp-facts">${facts.map(([k, v]) => `<div><dt>${k}</dt><dd>${esc(v)}</dd></div>`).join('')}</dl>` : ''}${rec ? '' : ''}`;
}
const tvWhen = (p) => `${fmt.dayRel(p.start)}, ${fmt.range(p.start, p.end)}`;
/** buttons of the details; px = prefix of the data-f ids (page and menu never exist together, but keep them apart) */
function tvProgActions(p, px, inMenu) {
  const r = SIM.rights(); const s = tvStat(p); const c = SIM.channel(p.ch); const out = []; const rs = r.recordings ? SIM.recState(p.id) : null;
  const d = { pid: p.id, ch: p.ch, menu: inMenu ? 1 : '' };
  let first = true; const def = () => { const v = first; first = false; return v; };
  if (s === 'now') out.push(tvBtn({ f: px + 'tune', ic: 'play', label: 'Einschalten', act: 'tvdTune', data: d, cls: 'primary', def: def() }));
  if (r.recordings && s !== 'past') {
    if (rs === 'recording') out.push(tvBtn({ f: px + 'rec', ic: 'record', label: 'Nimmt auf', act: 'tvdRunning', data: d, pressed: true, def: def() }));
    else out.push(tvBtn({ f: px + 'rec', ic: 'record', label: rs ? 'Aufnahme geplant' : s === 'now' ? 'Ab jetzt aufnehmen' : 'Aufnehmen', act: 'tvdRec', data: d, pressed: !!rs, def: def() }));
    if (p.series || p.episode) out.push(tvBtn({ f: px + 'ser', ic: 'recordSeries', label: rs === 'series' ? 'Serienaufnahme ändern' : 'Serie aufnehmen', act: 'tvdSeries', data: d, pressed: rs === 'series' }));
  }
  if (s === 'future') { const rm = SIM.reminderFor(p.id); out.push(tvBtn({ f: px + 'rem', ic: 'reminder', label: rm ? `Erinnerung um ${fmt.time(p.start - rm.lead)}` : 'Erinnern', act: 'tvdRemind', data: d, pressed: !!rm, def: def() })); }
  if (s === 'past') { const rec = SIM.recordings().find((x) => x.program === p.id); if (rec) out.push(tvBtn({ f: px + 'mine', ic: 'film', label: 'Deine Aufnahme ansehen', act: 'tvdGo', data: { to: '#/live/recordings/' + rec.id }, def: def() })); }
  out.push(tvBtn({ f: px + 'prog', ic: 'calendar', label: 'Senderprogramm', act: 'tvdGo', data: { to: '#/live/guide/channel/' + c.id }, cls: 'ghost', def: def() }));
  return out.join('');
}
function tvProgramMenu(pid) {
  const p = SIM.program(pid); if (!p) { toast('Diese Sendung steht nicht mehr im Programm.'); return; }
  menu(`<div class="tvp-menu">${tvProgInfo(p)}<div class="tvp-col">${tvProgActions(p, 'pm-', true)}</div></div>`, { cls: 'tvp-detail' });
}
/** close the details menu before the next dialog opens (focus goes back to the cell, the dialog returns there) */
const tvLeaveMenu = (d) => { if (d.menu && OV.length) closeMenu(); };
ACT.tvdTune = (d) => { if (d.menu) closeAll(); liveTune(d.ch, tvAllIds()); };
ACT.tvdRec = (d) => { tvLeaveMenu(d); LiveDlg.planRecording(d.pid); };
ACT.tvdSeries = (d) => { tvLeaveMenu(d); const rs = SIM.ruleForProgram(d.pid); LiveDlg.seriesRecording(d.pid, rs && rs.kind === 'series' ? rs.id : undefined); };
ACT.tvdRemind = (d) => { tvLeaveMenu(d); LiveDlg.reminder(d.pid); };
ACT.tvdGo = (d) => { if (d.menu) closeAll(); go(d.to); };
ACT.tvdRunning = (d) => {
  tvLeaveMenu(d); const run = SIM.runningOn(d.ch);
  menu(`<h2>Nimmt auf</h2><div class="mlist">${run ? `<a class="mi" href="#/live/recordings/${run.id}" data-f="rn-mine" data-default><span class="mi-ic">${icon('film', 36)}</span><span class="mi-tx"><b>Zur Aufnahme</b><small>${esc(run.title)}</small></span></a>
    <button type="button" class="mi" data-act="tvdStopRec" data-id="${run.id}" data-f="rn-stop"><span class="mi-ic">${icon('stop', 36)}</span><span class="mi-tx"><b>Aufnahme beenden</b><small>Das Bisherige bleibt erhalten</small></span></button>` : ''}</div>`);
};
ACT.tvdStopRec = (d) => { closeMenu(); LiveDlg.confirmDelete({ kind: 'running', ids: [d.id] }); };

route('/live/programs/:id', (pm) => {
  const p = SIM.program(pm.id);
  if (!p || SIM.epgOf(p.ch) === 'none') return { nav: '', html: livePage({ tab: 'guide', title: 'Nicht gefunden', html: `${tvNote('info', 'Diese Sendung steht nicht mehr im Programm.', '')}<div class="ltv-acts"><button type="button" class="tb primary" data-act="go" data-to="#/live/guide" data-f="tvp-nf" data-default>${icon('calendar', 32)}<span>Zum Programm</span></button></div>` }) };
  const more = L_more(p);
  return { nav: '', html: livePage({ tab: 'guide', html: `<div class="tvp-split"><div class="tvp-pi">${tvProgInfo(p)}</div><div class="tvp-col tvp-acts">${tvProgActions(p, 'pa-', false)}</div></div>
    ${more.length ? `<section class="tvp-more"><h2 class="tvp-rh">Weitere Termine</h2>${more.map((q, i) => tvpProgRow(q, { f: 'mo-' + i })).join('')}</section>` : ''}` }) };
});
function L_more(p) { const t = SIM.now(); return LIVE.programs.filter((q) => q.id !== p.id && q.start > Math.max(t, p.start) && SIM.epgOf(q.ch) !== 'none' && ((p.series && q.series === p.series) || q.title === p.title)).sort((a, b) => a.start - b.start).slice(0, 4); }

/* ---------- LS-01 Live-TV-Start ---------- */
function tvpCard(p, o) {
  const c = SIM.channel(p.ch); const s = tvStat(p); const bad = SIM.chState(c) !== 'ok';
  const rec = SIM.rights().recordings && SIM.runningOn(p.ch);
  return `<button type="button" class="tvp-card" data-act="${o.act}" data-pid="${p.id}" data-ch="${p.ch}" data-list="${o.list || 'all'}" data-f="${o.f}"${o.def ? ' data-default' : ''} data-tt="${esc(p.title)}" data-tm="${esc(`${c.name} · ${fmt.range(p.start, p.end)} · ${s === 'now' ? 'noch ' + fmt.left(p) + ' Min.' : 'ab ' + fmt.time(p.start)}`)}" aria-label="${esc(`${c.no} ${c.name}, ${p.title}, ${fmt.range(p.start, p.end)}`)}">
    <span class="tvp-im">${LU.picture(p, { fallback: 'logo' })}<span class="tvp-tag"><b>${c.no}</b> ${esc(c.name)}</span>${rec && s === 'now' ? `<span class="tvp-tag r">${LU.badge('recording', { text: true })}</span>` : ''}</span>
    <span class="tvp-cap"><b>${esc(p.title)}</b><small>${bad && s === 'now' ? `<span class="tvp-bad">${icon('alert', 22)}${esc(LU.TX.unavailable)}</span>` : s === 'now' ? `${fmt.range(p.start, p.end)} · noch ${fmt.left(p)} Min.` : `ab ${fmt.time(p.start)} · ${fmt.dur(p.end - p.start)}`}</small>${s === 'now' ? LU.progress(p.start, p.end) : `<span class="tvp-bd">${LU.programBadges(p, {})}</span>`}</span></button>`;
}
function tvpChCard(c, o) {
  const nn = SIM.nowNext(c.id); const n = nn.now; const bad = SIM.chState(c) !== 'ok';
  return `<button type="button" class="tvp-card ch" data-act="tvpTuneCh" data-ch="${c.id}" data-list="${o.list}" data-f="${o.f}"${o.def ? ' data-default' : ''} data-tt="${c.no} ${esc(c.name)}" data-tm="${esc(bad ? LU.TX.unavailable : n ? `${n.title} · ${fmt.range(n.start, n.end)}` : LU.TX.epgNone)}" aria-label="${esc(`${c.no} ${c.name}`)}">
    <span class="tvp-im plate">${LU.logo(c, { cls: 'big' })}<span class="tvp-tag"><b>${c.no}</b></span>${SIM.isFav(c.id) ? `<span class="tvp-tag r">${icon('heartOn', 24)}</span>` : ''}</span>
    <span class="tvp-cap"><b>${esc(c.name)}</b><small>${bad ? `<span class="tvp-bad">${icon('alert', 22)}${esc(LU.TX.unavailable)}</span>` : n ? esc(n.title) : esc(LU.TX.epgNone)}</small>${n && !bad ? LU.progress(n.start, n.end) : ''}</span></button>`;
}
const tvpSec = (id, title, html, extra) => `<section class="rsec tvp-sec"><h2 class="rh">${title}${extra ? `<span class="tvp-hx">${extra}</span>` : ''}</h2><div class="tvp-rail" data-row="${id}">${html}</div></section>`;
ACT.tvpTuneCh = (d) => liveTune(d.ch, tvListFor(d.list));
ACT.tvpNow = (d) => liveTune(d.ch, tvListFor(d.list));
ACT.tvpSoon = (d) => tvProgramMenu(d.pid);
ACT.tvpGenre = (d) => go(`#/live/guide/search?genre=${encodeURIComponent(d.genre)}&when=now`);

route('/live', () => {
  const r = SIM.rights(); const mode = SIM.mode();
  const st = tvStates({ n: 4, cls: 'tvp-skelcard' });
  let body;
  if (st) body = st;
  else {
    const chs = SIM.channels();
    if (!chs.length) body = tvNote('info', 'Noch keine Sender', esc(LU.TX.empty));
    else {
      const noEpg = SIM.flags().noEpg; const t = SIM.now();
      const fav = SIM.favorites().map((id) => SIM.channel(id)).filter(Boolean);
      const order = [...fav, ...chs.filter((c) => !SIM.isFav(c.id))];
      const nn = order.map((c) => ({ c, nn: SIM.nowNext(c.id) }));
      const nowL = nn.filter((x) => x.nn.now).map((x) => x.nn.now);
      const soonL = nn.filter((x) => x.nn.next && x.nn.next.start > t && x.nn.next.start - t <= 60).map((x) => x.nn.next);
      const recent = SIM.recent().slice(0, 12);
      const genres = noEpg ? [] : SIM.genresSoon(3).sort((a, b) => b.n - a.n);
      let first = true; const def = () => { const v = first; first = false; return v; };
      const warn = r.recordings ? SIM.warnings().length : 0;
      const run = r.recordings ? SIM.recordings().filter((x) => x.state === 'running').length : 0;
      const up = r.recordings ? SIM.upcoming().length : 0;
      const rem = SIM.reminders().length;
      const tile = (f, ic, h, t1, t2, hr, dd) => `<a class="tvp-link" href="${hr}" data-f="${f}"${dd ? ' data-default' : ''}><span class="mi-ic">${icon(ic, 40)}</span><span><b>${t1}</b><small>${t2}</small></span></a>`;
      body = `${warn ? `<a class="tnote warn tvp-alert" href="#/live/scheduled/warnings" data-f="st-warn">${icon('alert', 36)}<div><b>${warn === 1 ? 'Eine geplante Aufnahme braucht' : `${warn} geplante Aufnahmen brauchen`} deine Aufmerksamkeit</b><p>Öffnen und ansehen</p></div></a>` : ''}
        ${noEpg ? tvNote('info', 'Für die Sender gibt es gerade keine Programmdaten.', 'Fernsehen geht trotzdem.') : ''}
        ${nowL.length ? tvpSec('now', 'Jetzt im Fernsehen', nowL.map((p, i) => tvpCard(p, { f: 'now-' + p.ch, act: 'tvpNow', list: fav.some((c) => c.id === p.ch) ? 'fav' : 'all', def: def() })).join('')) : ''}
        ${soonL.length ? tvpSec('soon', 'Gleich im Fernsehen', soonL.map((p) => tvpCard(p, { f: 'soon-' + p.ch, act: 'tvpSoon', def: def() })).join('')) : ''}
        ${recent.length ? tvpSec('recent', 'Zuletzt gesehen', recent.map((x) => tvpChCard(x.ch, { f: 'rc-' + x.ch.id, list: 'recent', def: def() })).join('')) : ''}
        ${fav.length ? tvpSec('fav', 'Meine Sender', fav.map((c) => tvpChCard(c, { f: 'fv-' + c.id, list: 'fav', def: def() })).join('') + `<a class="tvp-card more" href="#/live/channels/favorites" data-f="fv-all"><span class="tvp-im"><b>Alle</b>${icon('right', 44)}</span></a>`) : ''}
        ${genres.length ? `<section class="rsec tvp-sec"><h2 class="rh">Nach Genre</h2><div class="tvp-rail slim" data-row="genre">${genres.map((g) => `<button type="button" class="tb ghost" data-act="tvpGenre" data-genre="${esc(g.genre)}" data-f="gn-${esc(g.genre)}"${def() ? ' data-default' : ''}><span>${esc(g.genre)}</span></button>`).join('')}</div></section>` : ''}
        <section class="rsec tvp-sec"><h2 class="rh">Einstiege</h2><div class="tvp-rail slim" data-row="links">
          ${tile('ln-guide', 'calendar', '', 'Programmzeitschrift', 'Was läuft, was kommt', '#/live/guide', def())}
          ${r.recordings ? tile('ln-rec', 'record', '', 'Meine Aufnahmen', run ? `${run} läuft` : 'Deine Aufnahmen', '#/live/recordings') : ''}
          ${r.recordings ? tile('ln-sched', 'clock', '', 'Geplant', `${up} Termine`, '#/live/scheduled') : ''}
          ${tile('ln-rem', 'reminder', '', 'Erinnerungen', `${rem}`, '#/live/reminders')}</div></section>`;
    }
  }
  return { nav: '', html: livePage({ tab: 'start', html: `<div class="tvp-start">${body.replace('<div class="tvp-rail"', '<div data-top class="tvp-rail"')}</div>` }), key: tvpNumKeys };
});
/** digits jump to the channel with that number; OK commits an entry in progress */
function tvpNumKeys(name, ev) {
  if (/^d\d$/.test(name)) { liveNumber(name, (c) => liveTune(c.id, tvAllIds())); return true; }
  if (name === 'ok' && liveNumberActive()) { liveNumberCommit(); return true; }
  return false;
}

/* ---------- LS-03 Meine Sender (arrange: OK picks a row up, up/down moves, OK puts it down, Back cancels) ---------- */
route('/live/channels/favorites', () => {
  const o = TVS.own; const favs = SIM.favorites().map((id) => SIM.channel(id)).filter(Boolean);
  const st = tvStates({ n: 6, cls: 'ltv-skel' }); let body;
  if (st) body = st;
  else if (!favs.length || SIM.mode() === 'empty') body = `${tvNote('info', 'Noch keine Lieblingssender', 'Drücke in der Senderliste auf das Herz.')}<div class="ltv-acts"><button type="button" class="tb primary" data-act="go" data-to="#/live/channels" data-f="fv-go" data-default>${icon('list', 32)}<span>Zu den Sendern</span></button></div>`;
  else {
    body = `<div class="ltv-tools"><button type="button" class="tb${o.order ? ' primary' : ' ghost'}" data-act="tvOrder" data-f="fv-order"${o.order ? ' aria-pressed="true"' : ''}>${icon(o.order ? 'check' : 'sort', 32)}<span>${o.order ? 'Fertig' : 'Ordnen'}</span></button>${o.order ? `<span class="tvp-hint">${o.held ? 'Mit ↑ und ↓ verschieben, OK legt ab, Zurück bricht ab.' : 'OK nimmt eine Zeile auf. Mit → entfernst du einen Sender.'}</span>` : ''}</div>
      <div class="ltv-split"><div class="ltv-list" aria-label="Meine Sender">${favs.map((c, k) => {
    const held = o.held === c.id; const rec = SIM.rights().recordings && SIM.runningOn(c.id);
    const main = o.order
      ? `<button type="button" class="ltv-ch${held ? ' held' : ''}" data-act="tvGrab" data-ch="${c.id}" data-f="ch-${c.id}"${k === 0 ? ' data-default' : ''} aria-label="${esc(`${c.name}, Platz ${k + 1} von ${favs.length}`)}"${held ? ' aria-pressed="true"' : ''}>`
      : `<a class="ltv-ch" href="#/live/watch/${c.id}" data-act="tvpTuneCh" data-list="fav" data-ch="${c.id}" data-f="ch-${c.id}"${k === 0 ? ' data-default' : ''} aria-label="${c.no} ${esc(c.name)}">`;
    return `<div class="ltv-row${SIM.chState(c) !== 'ok' ? ' bad' : ''}" data-k="${k}">${main}<span class="ltv-no">${o.order ? k + 1 : c.no}</span>${LU.logo(c)}<span class="ltv-name">${esc(c.name)}${rec ? `<span class="ltv-rec">${LU.badge('recording', { text: true })}</span>` : ''}</span>${ltv02NowLine(c)}${main.startsWith('<button') ? '</button>' : '</a>'}
        <button type="button" class="ltv-heart on" data-act="tvUnfav" data-ch="${c.id}" data-f="fav-${c.id}" aria-label="${esc(c.name)} aus Meine Sender entfernen">${icon('heartOn', 40)}</button></div>`;
  }).join('')}</div><aside class="ltv-prev" id="ltv-prev" aria-live="polite">${ltv02Preview(favs[0].id)}</aside></div>`;
  }
  return {
    nav: '', html: livePage({ tab: 'channels', title: 'Meine Sender', count: favs.length ? `${favs.length} Sender` : '', html: body }),
    onFocus(el) { const ch = el.dataset.ch; if (ch) { const p = $('#ltv-prev'); if (p && p.dataset.ch !== ch) { p.dataset.ch = ch; p.innerHTML = ltv02Preview(ch); } } },
    key(name) {
      if (tvpNumKeys(name)) return true;
      if (!o.held) return false;
      const k = SIM.favorites().indexOf(o.held);
      if (name === 'up' || name === 'down') { if (SIM.moveFav(o.held, name === 'up' ? -1 : 1)) toast(`${SIM.channel(o.held).name}, Platz ${SIM.favorites().indexOf(o.held) + 1} von ${SIM.favorites().length}`); else bump(F.el, name); return true; }
      if (name === 'back') { tvRestoreOrder(o.orig); o.held = null; render(); toast('Reihenfolge nicht geändert'); return true; }
      if (name === 'left' || name === 'right') return true;
      return k < 0;
    },
    back() { if (o.order) { o.order = false; o.held = null; render(); return true; } return false; },
  };
});
function tvRestoreOrder(orig) { if (!orig) return; for (let i = 0; i < orig.length; i++) { let k = SIM.favorites().indexOf(orig[i]); while (k > i) { SIM.moveFav(orig[i], -1); k--; } } }
ACT.tvOrder = () => { TVS.own.order = !TVS.own.order; TVS.own.held = null; render(); toast(TVS.own.order ? 'Ordnen: OK nimmt eine Zeile auf' : 'Reihenfolge gespeichert'); };
ACT.tvGrab = (d) => { const o = TVS.own; if (o.held === d.ch) { o.held = null; render(); toast('Abgelegt'); } else { o.held = d.ch; o.orig = SIM.favorites(); render(); toast(`${SIM.channel(d.ch).name} aufgenommen: ↑ und ↓ verschieben`); } };
ACT.tvUnfav = (d) => { SIM.toggleFav(d.ch); toast(`${SIM.channel(d.ch).name} ist nicht mehr in Meine Sender`); };

/* ---------- LS-05 Programmsuche ---------- */
const TV_WHEN = [['now', 'Jetzt'], ['today', 'Heute'], ['tomorrow', 'Morgen'], ['week', 'Nächste 7 Tage']];
route('/live/guide/search', (pm, query) => {
  const s = TVS.search; const sig = JSON.stringify(query);
  if (sig !== TVS.sq.sig) { TVS.sq.sig = sig; if (sig !== '{}') { s.q = query.q || ''; s.when = query.when || (query.q || query.genre ? 'week' : s.when); s.genre = query.genre || ''; s.group = query.group || ''; } }
  const st = tvStates({ n: 6, cls: 'ltv-skel' });
  const groupName = s.group === 'fav' ? 'Meine Sender' : s.group ? (LIVE.groups.find((g) => g.id === s.group) || { name: s.group }).name : 'Alle Sender';
  const tools = `<div class="ltv-tools">
    <button type="button" class="tf ltv-search sx" data-act="tvsKeyboard" data-f="sr-q" data-default>${icon('search', 36)}${s.q ? `<span class="tf-v">${esc(s.q)}</span>` : '<span class="tf-ph">Sendungen suchen</span>'}</button>
    <button type="button" class="tb ghost" data-act="tvsWhen" data-f="sr-when">${icon('clock', 32)}<span>${esc(TV_WHEN.find((w) => w[0] === s.when)[1])}</span></button>
    <button type="button" class="tb ghost" data-act="tvsGenre" data-f="sr-genre">${icon('filter', 32)}<span>${esc(s.genre || 'Alle Genres')}</span></button>
    <button type="button" class="tb ghost" data-act="tvsGroup" data-f="sr-group">${icon('list', 32)}<span>${esc(groupName)}</span></button></div>`;
  let body;
  if (st) body = st;
  else if (!s.q && !s.genre) body = tvNote('search', 'Gib einen Titel ein oder wähle ein Genre.', '');
  else {
    const res = SIM.search(s.q, { when: s.when, genre: s.genre, group: s.group || undefined });
    const noData = SIM.channels({ group: s.group || undefined }).filter((c) => SIM.epgOf(c) !== 'ok');
    const note = noData.length ? tvNote('info', SIM.flags().noEpg ? 'Für die Sender gibt es gerade keine Programmdaten.' : `Für ${noData.length} Sender gibt es keine oder nur teilweise Programmdaten: ${noData.map((c) => esc(c.name) + (SIM.epgOf(c) === 'partial' ? ' (teilweise)' : '')).join(', ')}.`, '') : '';
    if (!res.length) body = `${tvNote('info', 'Keine Sendung gefunden.', `Programmdaten reichen bis ${fmt.day(LIVE.EPG_END - 1)}`)}${note}`;
    else {
      const shown = res.slice(0, 40); let lastDay = null; let idx = 0;
      body = `${note}<div class="tvp-res" aria-label="Treffer">${shown.map((p) => {
        const d = Math.floor(p.start / 1440); const h = d !== lastDay ? `<h2 class="tvp-dayh">${fmt.dayRel(p.start)}${fmt.dayRel(p.start).startsWith('Heute') || fmt.dayRel(p.start) === 'Morgen' ? ` · ${fmt.day(p.start)}` : ''}</h2>` : ''; lastDay = d;
        return h + tvpProgRow(p, { f: 'rs-' + p.id, def: idx++ === 0 && false });
      }).join('')}</div>${res.length > shown.length ? `<p class="tvp-hint">${res.length - shown.length} weitere Treffer: Suche enger fassen.</p>` : ''}`;
    }
  }
  return { nav: '', html: livePage({ tab: 'guide', title: 'Programm durchsuchen', count: '', html: tools + body }), key: tvpNumKeys };
});
const tvsGo = () => render();
ACT.tvsKeyboard = () => keyboard({ value: TVS.search.q, label: 'Sendungen suchen (Titel oder Beschreibung)', placeholder: 'zum Beispiel Tatort', onDone: (v) => { TVS.search.q = v.trim(); if (TVS.search.q && TVS.search.when === 'now') TVS.search.when = 'week'; render(); } });
function tvsPick(title, opts, cur, act) {
  menu(`<h2>${title}</h2><div class="mlist">${opts.map(([v, t, n], i) => `<button type="button" class="mi" role="menuitemradio" aria-checked="${cur === v}" data-act="${act}" data-v="${esc(v)}" data-f="so-${i}"${cur === v ? ' data-default' : ''}><span class="mi-tx"><b>${esc(t)}</b>${n ? `<small>${esc(n)}</small>` : ''}</span>${cur === v ? icon('check', 34) : ''}</button>`).join('')}</div>`, { onClose: () => render() });
}
ACT.tvsWhen = () => tvsPick('Wann', TV_WHEN.map(([v, t]) => [v, t, '']), TVS.search.when, 'tvsSetWhen');
ACT.tvsSetWhen = (d) => { TVS.search.when = d.v; closeMenu(); };
ACT.tvsGenre = () => { const gs = [...new Set(LIVE.programs.map((p) => p.genre))].filter((g) => g !== 'Sonstiges').sort((a, b) => a.localeCompare(b, 'de')); tvsPick('Genre', [['', 'Alle Genres', ''], ...gs.map((g) => [g, g, ''])], TVS.search.genre, 'tvsSetGenre'); };
ACT.tvsSetGenre = (d) => { TVS.search.genre = d.v; closeMenu(); };
ACT.tvsGroup = () => tvsPick('Gruppe', [['', 'Alle Sender', ''], ['fav', 'Meine Sender', `${SIM.favorites().length} Sender`], ...LIVE.groups.map((g) => [g.id, g.name, ''])], TVS.search.group, 'tvsSetGroup');
ACT.tvsSetGroup = (d) => { TVS.search.group = d.v; closeMenu(); };

/* ---------- LS-15 Senderprogramm ---------- */
route('/live/guide/channel/:id', (pm, query) => {
  const c = SIM.channel(pm.id);
  if (!c || SIM.chState(c) === 'gone') return { nav: '', html: livePage({ tab: 'guide', title: 'Nicht gefunden', html: `${tvNote('info', 'Diesen Sender gibt es nicht mehr.', '')}<div class="ltv-acts"><button type="button" class="tb primary" data-act="go" data-to="#/live/channels" data-f="cp-nf" data-default>${icon('list', 32)}<span>Zu den Sendern</span></button></div>` }) };
  const t = SIM.now(); const today = Math.floor(t / 1440);
  const day = query.day != null ? Number(query.day) : (TVS.cpDay != null && TVS.cpCh === c.id ? TVS.cpDay : today); TVS.cpDay = day; TVS.cpCh = c.id;
  const days = []; for (let d = today; d < LIVE.EPG_END / 1440; d++) days.push(d);
  const head = `<div class="ltv-tools"><span class="tvp-chhead">${LU.logo(c, { cls: 'big' })}<b>${c.no} ${esc(c.name)}</b></span>
    ${tvBtn({ f: 'cp-tune', ic: 'play', label: 'Einschalten', act: 'tvpTuneCh', data: { ch: c.id, list: 'all' }, cls: 'primary' })}
    <button type="button" class="tb ltv-fav${SIM.isFav(c.id) ? ' on' : ''}" data-act="ltvFav" data-ch="${c.id}" data-f="cp-fav" aria-pressed="${SIM.isFav(c.id)}" aria-label="${SIM.isFav(c.id) ? 'Aus Meine Sender entfernen' : 'Zu Meine Sender hinzufügen'}">${icon(SIM.isFav(c.id) ? 'heartOn' : 'heart', 32)}<span>Meine Sender</span></button></div>
    <div class="tvp-days" data-row="days">${days.map((d) => `<button type="button" class="tb sm ${d === day ? 'primary' : 'ghost'}" data-act="tvcDay" data-day="${d}" data-ch="${c.id}" data-f="cp-d${d}" aria-pressed="${d === day}"><span>${d === today ? 'Heute' : d === today + 1 ? 'Morgen' : fmt.day(d * 1440)}</span></button>`).join('')}</div>`;
  const st = tvStates({ n: 6, cls: 'ltv-skel' }); let body;
  if (st) body = st;
  else if (SIM.chState(c) === 'disturbed') body = tvNote('alert', `${esc(c.name)} ist gerade nicht erreichbar.`, 'Slouchcat versucht es weiter. Das Programm ist trotzdem da.', 'warn');
  else body = '';
  if (!st) {
    const from = day * 1440; const list = SIM.programsOf(c.id, from, from + 1440);
    if (!list.length) {
      const canRec = SIM.rights().recordings;
      body += `${tvNote('info', SIM.epgOf(c) === 'none' ? 'Für diesen Sender gibt es keine Programmdaten.' : 'Keine Programmdaten für diesen Tag.', SIM.epgOf(c) === 'none' ? esc(LU.TX.epgNoneButPlays) : '')}<div class="ltv-acts">${canRec ? tvBtn({ f: 'cp-time', ic: 'record', label: 'Zeit aufnehmen', act: 'tvcManual', data: { ch: c.id }, def: true }) : ''}${tvBtn({ f: 'cp-on', ic: 'play', label: 'Einschalten', act: 'tvpTuneCh', data: { ch: c.id, list: 'all' }, cls: canRec ? 'ghost' : 'primary', def: !canRec })}</div>`;
    } else {
      const rows = []; let cursor = from; let defSet = false;
      const gapRow = (a, b) => `<p class="tvp-gaprow lv-gap"><span class="lv-gap-text">${fmt.range(a, b)} · Keine Programmdaten</span></p>`;
      list.forEach((p) => {
        if (p.start > cursor) rows.push(gapRow(cursor, p.start));
        const s = tvStat(p); const isDef = s === 'now' || (!defSet && s === 'future' && !list.some((q) => tvStat(q) === 'now'));
        if (isDef) defSet = true;
        rows.push(tvpProgRow(p, { f: 'cp-' + p.id, noCh: true, def: isDef })); cursor = Math.max(cursor, p.end);
      });
      if (cursor < from + 1440) rows.push(gapRow(cursor, from + 1440));
      body += `<div class="tvp-res cp">${rows.join('')}</div>`;
    }
  }
  return { nav: '', html: livePage({ tab: 'guide', html: head + body }), key: tvpNumKeys };
});
ACT.tvcDay = (d) => { TVS.cpDay = Number(d.day); render(); };
ACT.tvcManual = (d) => LiveDlg.manualRecording({ ch: d.ch });
