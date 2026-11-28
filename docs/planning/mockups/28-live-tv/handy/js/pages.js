/* Konzept 28 «Live-TV», phone and tablet app: helpers of the page files (prefix hp) and the pages
   LS-01 start, LS-03 my channels, LS-04 programme guide (phone: «Jetzt und danach» list, tablet: grid),
   LS-05 programme search, LS-06 programme details, LS-15 channel programme.
   14-oberflaeche.md 3.1, 3.3, 3.5, 3.6, 3.15, 4.1, 4.2. Plain script, own globals start with hp / HP. */
'use strict';

/* icons recordHollow, recordSeries, arrowUp, arrowDown, expand, loadspin: in core.js */

/* ---------- shared page state ---------- */
const HP = {
  g: { day: 0, time: 'now', group: '', sl: null, open: false },
  s: { q: '', when: 'week', genre: '', group: '', limit: 40, key: '' },
  cd: { day: 0, key: '' },
  fav: { order: false },
  rec: { filter: 'all', sort: 'new', select: false, sel: [], failOpen: false, series: '' },
  sch: { tab: 'upcoming', open: {} },
  prof: { open: {} },
  p: null, /* live player (pages-2.js) */
  noAsk: false, radio: null, dragged: 0, fullQuota: false, origUsed: LIVE.account.usedGB,
};
const HP_PX = 5; /* guide grid: px per minute */

const hpGenres = () => [...new Set(LIVE.programs.map((p) => p.genre))].filter((g) => g && g !== 'Sonstiges').sort((a, b) => a.localeCompare(b, 'de'));
const hpMin = (c) => (c.bitrate || 8) * 60 / 8000; /* GB per minute from the measured bit rate */
const hpGBtext = (min, c) => `etwa ${fmt.gb(min * hpMin(c))}`;
const hpWhen = (p) => `${fmt.dayRel(p.start)}, ${fmt.range(p.start, p.end)}`;
const hpIsNow = (p) => p.start <= SIM.now() && p.end > SIM.now();
const hpDays = () => Array.from({ length: Math.round((LIVE.EPG_END - 0) / 1440) }, (_, i) => i);
const hpSerial = (p) => !!(p && (p.series || p.episode));
const hpProfName = (id) => (LIVE.profiles.find((x) => x.id === id) || {}).name || id;
const hpKeepText = (k, r) => (!k || k.mode === 'forever' ? 'Behalten, bis du löschst' : `Wird am ${fmt.day(k.until != null ? k.until : (r ? r.start : SIM.now()) + k.days * 1440)} gelöscht`);

/** loading and error state of a page, '' in normal and empty mode */
function hpState(rows, h) {
  const m = SIM.mode();
  if (m === 'loading') return `<div class="pad hp-skel" aria-busy="true" aria-label="Lädt">${Array.from({ length: rows || 6 }, () => `<span class="lv-skel" style="height:${h || 72}px"></span>`).join('')}</div>`;
  if (m === 'error') return `<div class="note warn" role="alert">${icon('alert', 20)}<div><b>${esc(LU.TX.loadingErr)}</b><br>Der Server hat nicht geantwortet.</div></div><p class="pad"><button type="button" class="btn secondary" data-act="hpRetry" data-fid="hp-retry">${icon('refresh', 18)}Erneut versuchen</button></p>`;
  return '';
}
ACT.hpRetry = () => { SIM.setMode('normal'); toast('Erneut geladen'); };
const hpEmptyBox = (ic, title, text, acts) => `<div class="empty">${icon(ic, 40)}<b>${esc(title)}</b><p>${esc(text)}</p>${acts ? `<p style="margin-top:16px">${acts}</p>` : ''}</div>`;
const hpNotFound = (text, linkHref, linkText) => ({ nav: '', html: `${bar('Nicht gefunden', { back: true })}${hpEmptyBox('alert', 'Nicht gefunden', text, linkHref ? `<a class="btn ghost" href="${linkHref}" data-fid="nf-link">${esc(linkText)}</a>` : '')}` });
const hpBackBar = (title, end) => bar(title, { back: true, end: end || '' });

/* ---------- actions shared by the pages ---------- */
ACT.hpTune = (d) => liveTune(d.ch, d.list ? d.list.split(',') : undefined);
ACT.hpTuneClose = (d) => { closeSheet(true); liveTune(d.ch); };
ACT.hpFav = (d) => { const on = SIM.toggleFav(d.ch); toast(`${SIM.channel(d.ch).name} ${on ? 'ist jetzt in Meine Sender' : 'ist nicht mehr in Meine Sender'}`); };
ACT.hpRec = (d) => {
  const p = SIM.program(d.p); if (!p) return;
  if (SIM.recState(d.p) === 'recording') return hpRunningMenu(p.ch);
  LiveDlg.planRecording(d.p);
};
ACT.hpRecClose = (d) => ACT.hpRec(d);
ACT.hpSeries = (d) => LiveDlg.seriesRecording(d.p);
ACT.hpRem = (d) => LiveDlg.reminder(d.p);
ACT.hpRemClose = (d) => LiveDlg.reminder(d.p);
ACT.hpSheet = (d) => hpProgSheet(d.p);
/** menu of a running recording on a channel (player button, details) */
function hpRunningMenu(chId) {
  const r = SIM.runningOn(chId); if (!r) return;
  const nn = SIM.nowNext(chId).now; const canExtend = nn && r.end < nn.end + LIVE.settings.dvr.padAfter - 0.5;
  sheet(`<h2>Nimmt auf · ${esc(r.title)}</h2><p class="pad hint">Seit ${fmt.time(r.start)} · bis ${fmt.time(r.end)} · ${fmt.gb(r.liveGB || r.sizeGB)} bisher</p>
    ${canExtend ? li('clock', 'Bis Sendungsende verlängern', `bis ${fmt.time(nn.end + LIVE.settings.dvr.padAfter)}`, '', `data-act="hpExtend" data-id="${r.id}" data-fid="rm-ext"`) : ''}
    ${li('close', 'Aufnahme beenden', '', null, `data-act="hpStopRec" data-id="${r.id}" data-fid="rm-stop"`)}
    ${li('film', 'Zu meinen Aufnahmen', '', null, `href="#/live/recordings/${r.id}" data-fid="rm-go"`)}
    <div class="acts"><button type="button" class="btn ghost" data-act="close" data-fid="rm-x">Schliessen</button></div>`);
}
ACT.hpExtend = (d) => { const r = SIM.recording(d.id); const nn = SIM.nowNext(r.ch).now; r.end = nn.end + LIVE.settings.dvr.padAfter; closeSheet(true); SIM.emit('change'); toast(`Aufnahme läuft bis ${fmt.time(r.end)}.`); };
ACT.hpStopRec = (d) => LiveDlg.confirmDelete({ kind: 'running', ids: [d.id] });

/** bottom sheet for one programme (long press, grid cell, menu button) */
function hpProgSheet(pid) {
  const p = SIM.program(pid); if (!p) return; const c = SIM.channel(p.ch); const r = SIM.rights(); const run = hpIsNow(p); const past = p.end <= SIM.now();
  const rs = r.recordings && !past ? SIM.recState(pid) : null; const rem = SIM.reminderFor(pid);
  sheet(`<h2>${esc(p.title)}</h2><p class="pad hint">${esc(c.name)} · ${esc(hpWhen(p))}</p>
    ${run ? li('play', 'Einschalten', c.name, '', `data-act="hpTuneClose" data-ch="${c.id}" data-fid="ps-tune"`) : ''}
    ${li('info', 'Details', p.sub || '', null, `href="#/live/programs/${p.id}" data-fid="ps-det"`)}
    ${r.recordings && !past ? li(rs === 'recording' ? 'record' : 'recordHollow', rs === 'recording' ? 'Nimmt auf' : rs ? 'Aufnahme ändern' : 'Aufnehmen', rs ? LU.TX[rs === 'series' ? 'series' : rs === 'recording' ? 'recording' : 'scheduled'] : '', '', `data-act="hpRecClose" data-p="${p.id}" data-fid="ps-rec"`) : ''}
    ${r.recordings && !past && hpSerial(p) ? li('recordSeries', 'Serie aufnehmen', '', '', `data-act="hpSeries" data-p="${p.id}" data-fid="ps-ser"`) : ''}
    ${!past && !run ? li('reminder', rem ? 'Erinnerung ändern' : 'Erinnern', rem ? `${rem.lead ? rem.lead + ' Min. vorher' : 'Zum Beginn'}` : '', '', `data-act="hpRemClose" data-p="${p.id}" data-fid="ps-rem"`) : ''}
    ${li('calendar', 'Senderprogramm', 'Tagesprogramm als Liste', null, `href="#/live/guide/channel/${c.id}" data-fid="ps-prog"`)}
    <div class="acts"><button type="button" class="btn ghost" data-act="close" data-fid="ps-x">Schliessen</button></div>`);
}
/* long press on a guide or search row opens the programme sheet (a short tap follows the link) */
(function hpLongPress() {
  let t = null; let fired = false;
  document.addEventListener('pointerdown', (e) => { const el = e.target.closest('[data-hplp]'); if (!el) return; fired = false; t = setTimeout(() => { fired = true; hpProgSheet(el.dataset.hplp); }, 550); });
  ['pointerup', 'pointercancel', 'pointerleave'].forEach((n) => document.addEventListener(n, () => clearTimeout(t)));
  document.addEventListener('click', (e) => { if (fired && e.target.closest('[data-hplp]')) { e.stopImmediatePropagation(); e.preventDefault(); fired = false; } }, true);
})();

/* ---------- small building blocks ---------- */
/** icon button for recording; state shown by symbol AND accessible name */
function hpRecIb(p) {
  if (!SIM.rights().recordings || p.end <= SIM.now()) return '';
  const s = SIM.recState(p.id);
  const lab = { recording: 'Nimmt auf', series: 'Serie geplant', scheduled: 'Aufnahme geplant' }[s] || 'Aufnehmen';
  return `<button type="button" class="ib hp-rec${s ? ' on ' + s : ''}" data-act="hpRec" data-p="${p.id}" aria-label="${esc(lab)}: ${esc(p.title)}" data-fid="rec-${p.id}">${icon(s === 'series' ? 'recordSeries' : s === 'recording' ? 'record' : 'recordHollow', 22)}</button>`;
}
function hpRemIb(p) {
  if (p.start <= SIM.now()) return '';
  const on = !!SIM.reminderFor(p.id);
  return `<button type="button" class="ib hp-remb${on ? ' on' : ''}" data-act="hpRem" data-p="${p.id}" aria-pressed="${on}" aria-label="${on ? 'Erinnerung ändern' : 'Erinnern'}: ${esc(p.title)}" data-fid="rem-${p.id}">${icon('reminder', 22)}</button>`;
}
/** skipped by a series recording: «schon geplant» … */
function hpSkip(p) {
  if (!SIM.rights().recordings) return '';
  return SIM.recState(p.id) ? '' : SIM.skippedWhy(p.id) || ''; /* from the core */
}
/** programme row: time, (channel), title, sub, progress, badges, actions */
function hpPR(p, o) {
  o = o || {}; const t = SIM.now(); const c = SIM.channel(p.ch); const run = p.start <= t && p.end > t; const past = p.end <= t; const skip = hpSkip(p);
  const bad = LU.programBadges(p, { text: true });
  const acts = run ? `<button type="button" class="ib" data-act="hpTune" data-ch="${p.ch}" aria-label="${esc(c.name)} einschalten" data-fid="pt-${o.k || ''}${p.id}">${icon('play', 22)}</button>${hpRecIb(p)}` : `${hpRecIb(p)}${hpRemIb(p)}`;
  return `<li class="hp-pr${run ? ' run' : ''}${past ? ' past' : ''}">
    <a class="hp-pr-main" href="#/live/programs/${p.id}" data-hplp="${p.id}" data-fid="pr-${o.k || ''}${p.id}" aria-label="${esc(c.name)}, ${esc(p.title)}${p.sub ? ', ' + esc(p.sub) : ''}, ${fmt.time(p.start)} bis ${fmt.time(p.end)}${run ? ', läuft' : ''}">
      <span class="hp-pr-time"><b>${fmt.time(p.start)}</b><small>${fmt.time(p.end)}</small></span>
      <span class="hp-pr-body">${o.noCh ? '' : `<span class="hp-pr-ch">${LU.logo(c)}<small>${c.no} ${esc(c.name)}</small></span>`}<b class="hp-pr-t">${esc(p.title)}</b>${p.sub ? `<small>${esc(p.sub)}</small>` : ''}${run ? LU.progress(p.start, p.end) : ''}
        <span class="hp-pr-meta">${p.genre ? `<small>${esc(p.genre)}</small>` : ''}${p.repeat ? '<small>Wiederholung</small>' : ''}${bad}${skip ? `<small>Übersprungen: ${esc(skip)}</small>` : ''}</span></span></a>
    <span class="hp-pr-act">${acts}</span></li>`;
}
/** small channel card (start page rows) */
function hpChCard(c, list, k) {
  const nn = SIM.nowNext(c.id); const st = SIM.chState(c); const rec = SIM.rights().recordings && SIM.runningOn(c.id);
  const mid = st === 'disturbed' ? `<span class="hp-warn">${icon('alert', 14)}${esc(LU.TX.unavailable)}</span>` : nn.now ? `<b>${esc(nn.now.title)}</b><small>${fmt.range(nn.now.start, nn.now.end)}</small>${LU.progress(nn.now.start, nn.now.end)}` : `<small>${esc(SIM.epgOf(c) === 'none' ? LU.TX.epgNoneButPlays : LU.TX.epgNone)}</small>`;
  return `<a class="hp-cc" href="#/live/watch/${c.id}" data-act="hpTune" data-ch="${c.id}" data-list="${list.join(',')}" data-fid="${k}-${c.id}" aria-label="${c.no} ${esc(c.name)}${nn.now ? ', ' + esc(nn.now.title) : ''} einschalten"><span class="hp-cc-h"><span class="lh-no">${c.no}</span>${LU.logo(c)}<span class="hp-cc-n">${esc(c.name)}</span>${rec ? LU.badge('recording') : ''}</span>${mid}</a>`;
}
/** wide card: programme now or soon with picture */
function hpProgCard(p, list, mode, k) {
  const c = SIM.channel(p.ch); const st = SIM.chState(c); const now = mode === 'now';
  const tag = LU.programBadges(p);
  return `<a class="pc wide hp-pcard" href="${now ? '#/live/watch/' + c.id : '#/live/programs/' + p.id}" ${now ? `data-act="hpTune" data-ch="${c.id}" data-list="${list.join(',')}"` : ''} data-fid="${k}-${p.id}" aria-label="${c.no} ${esc(c.name)}, ${esc(p.title)}, ${fmt.range(p.start, p.end)}${now ? ' einschalten' : ', Details'}">
    <span class="im">${LU.picture(p, { fallback: 'logo' })}${tag ? `<span class="tag">${tag}</span>` : ''}${now ? `<span class="hp-ovprog">${LU.progress(p.start, p.end)}</span>` : ''}</span>
    <span class="t">${esc(p.title)}</span><span class="m">${c.no} ${esc(c.name)} · ${now ? fmt.range(p.start, p.end) : 'ab ' + fmt.time(p.start)}${st === 'disturbed' ? ' · ' + esc(LU.TX.unavailable) : ''}</span></a>`;
}

/* ============================================================ LS-01 Live-TV-Start */
route('/live', () => {
  const r = SIM.rights(); const noEpg = SIM.flags().noEpg; const t = SIM.now();
  let body = hpState(4, 150);
  if (!body) {
    const chs = SIM.channels();
    if (!chs.length) body = hpEmptyBox('live', 'Noch keine Sender', LU.TX.empty);
    else {
      const favs = SIM.favorites().map(SIM.channel).filter(Boolean); const order = [...favs, ...chs.filter((c) => !favs.some((f) => f.id === c.id))];
      const ids = order.map((c) => c.id);
      const nowList = order.map((c) => SIM.nowNext(c.id).now).filter(Boolean);
      const soon = order.map((c) => SIM.programsOf(c.id, t, t + 60).find((p) => p.start > t)).filter(Boolean);
      const recent = SIM.recent().slice(0, 20).map((x) => x.ch).filter((c) => SIM.chState(c) !== 'gone');
      const genres = SIM.genresSoon(3).sort((a, b) => b.n - a.n);
      const bad = r.recordings ? SIM.warningCount() : 0;
      const run = r.recordings ? SIM.recordings().filter((x) => x.state === 'running').length : 0; const up = r.recordings ? SIM.upcoming().length : 0; const rem = SIM.reminders().length;
      const rowH = (title, link) => `<div class="rowh"><h2>${esc(title)}</h2>${link || ''}</div>`;
      body = `${bad ? `<a class="note warn hp-link" href="#/live/scheduled/warnings" data-fid="ls01-warn">${icon('alert', 20)}<div><b>${esc(LU.warnText(bad))}</b><br>Warnungen ansehen</div></a>` : ''}
        ${noEpg ? `<div class="note">${icon('info', 20)}<div>Für die Sender gibt es gerade keine Programmdaten. Fernsehen geht trotzdem.</div></div>` : ''}
        ${!noEpg && nowList.length ? `${rowH('Jetzt im Fernsehen')}<div class="hrow" role="list" aria-label="Jetzt im Fernsehen">${nowList.map((p) => hpProgCard(p, ids, 'now', 'n')).join('')}</div>` : ''}
        ${!noEpg && soon.length ? `${rowH('Gleich im Fernsehen')}<div class="hrow" role="list" aria-label="Gleich im Fernsehen">${soon.map((p) => hpProgCard(p, ids, 'soon', 's')).join('')}</div>` : ''}
        ${recent.length ? `${rowH('Zuletzt gesehen', `<button type="button" class="ib" data-act="hpRecentMenu" aria-label="Menü der Reihe «Zuletzt gesehen»" data-fid="ls01-recent-menu">${icon('dots', 20)}</button>`)}<div class="hrow" role="list" aria-label="Zuletzt gesehen">${recent.map((c) => hpChCard(c, recent.map((x) => x.id), 'z')).join('')}</div>` : ''}
        ${favs.length ? `${rowH('Meine Sender', `<a href="#/live/channels/favorites" data-fid="ls01-favall" aria-label="Alle Meine Sender">Alle ${icon('right', 16)}</a>`)}<div class="hrow" role="list" aria-label="Meine Sender">${favs.map((c) => hpChCard(c, favs.map((x) => x.id), 'f')).join('')}</div>` : ''}
        ${!noEpg && genres.length ? `${rowH('Nach Genre')}<div class="chips" role="list" aria-label="Nach Genre">${genres.map((g) => `<a class="chip" role="listitem" href="#/live/guide/search?genre=${encodeURIComponent(g.genre)}&when=now" data-fid="ls01-g-${esc(g.genre)}">${esc(g.genre)}</a>`).join('')}</div>` : ''}
        <div class="hp-tiles">
          <a class="hp-tile" href="#/live/guide" data-fid="ls01-t-guide">${icon('calendar', 24)}<b>Programmzeitschrift</b><small>Jetzt und danach</small></a>
          ${r.recordings ? `<a class="hp-tile" href="#/live/recordings" data-fid="ls01-t-rec">${icon('record', 24)}<b>Meine Aufnahmen</b><small>${run ? run + (run === 1 ? ' läuft' : ' laufen') : 'Alle ansehen'}</small></a>
          <a class="hp-tile" href="#/live/scheduled" data-fid="ls01-t-sch">${icon('clock', 24)}<b>Geplant</b><small>${up} ${up === 1 ? 'Termin' : 'Termine'}</small></a>` : ''}
          <a class="hp-tile" href="#/live/reminders" data-fid="ls01-t-rem">${icon('reminder', 24)}<b>Erinnerungen</b><small>${rem}</small></a></div>`;
    }
  }
  return { nav: '', html: livePage({ tab: 'start', title: 'Live-TV', html: body }) };
});

ACT.hpRecentMenu = () => sheet(`<h2>Zuletzt gesehen</h2>${li('trash', 'Liste leeren', 'Die Sender bleiben, nur die Reihe wird leer.', null, 'data-act="hpRecentClear" data-fid="rc-clear"')}<div class="acts"><button type="button" class="btn ghost" data-act="close" data-fid="rc-x">Schliessen</button></div>`);
ACT.hpRecentClear = () => { closeSheet(true); SIM.clearRecent(); toast('Die Liste «Zuletzt gesehen» ist leer'); };

/* ============================================================ LS-03 Meine Sender */
route('/live/channels/favorites', () => {
  const ids = SIM.favorites(); const list = ids.map(SIM.channel).filter(Boolean); const ord = HP.fav.order;
  let body = hpState(5, 72);
  if (!body) {
    if (!list.length) body = hpEmptyBox('heart', 'Noch keine Lieblingssender', 'Tippe in der Senderliste aufs Herz.', `<a class="btn ghost" href="#/live/channels" data-fid="fav-go">Zu den Sendern</a>`);
    else body = `<div class="hp-tools"><span class="hp-cnt">${list.length} ${list.length === 1 ? 'Sender' : 'Sender'}</span><span class="hp-sp"></span><a class="btn ghost sm" href="#/live/channels" data-fid="fav-all">Alle Sender</a><button type="button" class="btn ${ord ? 'primary' : 'secondary'} sm" data-act="hpOrder" aria-pressed="${ord}" data-fid="fav-order">${ord ? 'Fertig' : 'Ordnen'}</button></div>
      ${SIM.flags().noEpg ? `<div class="note">${icon('info', 20)}<div>Für die Sender gibt es gerade keine Programmdaten. Fernsehen geht trotzdem.</div></div>` : ''}
      <ul class="lh-list" aria-label="Meine Sender">${ord ? list.map((c, i) => `<li class="lh-row hp-ordrow"><span class="lh-main hp-ordmain"><span class="lh-top"><span class="lh-no">${i + 1}</span>${LU.logo(c)}<span class="lh-name">${esc(c.name)}</span></span></span>
          <button type="button" class="ib" data-act="hpMove" data-ch="${c.id}" data-dir="-1" ${i === 0 ? 'disabled' : ''} aria-label="${esc(c.name)} nach oben, Platz ${i + 1} von ${list.length}" data-fid="mv-u-${c.id}">${icon('arrowUp', 22)}</button>
          <button type="button" class="ib" data-act="hpMove" data-ch="${c.id}" data-dir="1" ${i === list.length - 1 ? 'disabled' : ''} aria-label="${esc(c.name)} nach unten, Platz ${i + 1} von ${list.length}" data-fid="mv-d-${c.id}">${icon('arrowDown', 22)}</button>
          <button type="button" class="ib" data-act="hpFav" data-ch="${c.id}" aria-label="${esc(c.name)} entfernen" data-fid="mv-x-${c.id}">${icon('close', 22)}</button></li>`).join('') : list.map(lh02Row).join('')}</ul>`;
  }
  return { nav: '', html: livePage({ tab: 'channels', title: 'Meine Sender', end: list.length ? `<span class="lh-count">${list.length}</span>` : '', html: body }) };
});
ACT.hpOrder = () => { HP.fav.order = !HP.fav.order; render(); };
ACT.hpMove = (d) => { SIM.moveFav(d.ch, Number(d.dir)); const c = SIM.channel(d.ch); const k = SIM.favorites().indexOf(d.ch); toast(`${c.name}, Platz ${k + 1} von ${SIM.favorites().length}`); };

/* ============================================================ LS-04 Programmzeitschrift */
const hpGuideT = () => { const g = HP.g; return g.time === 'now' ? (g.day === 0 ? SIM.now() : g.day * 1440 + 20 * 60 + 15) : g.day * 1440 + g.time; };
const hpGuideTimeLabel = () => { const g = HP.g; return g.time === 'now' && g.day === 0 ? 'Jetzt' : `Um ${fmt.time(hpGuideT())}`; };
const hpGroupName = (g) => (!g ? 'Alle Sender' : g === 'fav' ? 'Meine Sender' : ((LIVE.groups.find((x) => x.id === g) || {}).name || g));
function hpGuideTools() {
  const g = HP.g;
  return `<div class="chips hp-days" role="group" aria-label="Tag">${hpDays().map((d) => `<button type="button" class="chip${g.day === d ? ' on' : ''}" aria-pressed="${g.day === d}" data-act="hpGDay" data-d="${d}" data-fid="gd-${d}">${esc(fmt.dayRel(d * 1440))}</button>`).join('')}</div>
    <div class="hp-tools"><button type="button" class="btn ghost sm" data-act="hpGTime" data-fid="g-time">${icon('clock', 18)}${esc(hpGuideTimeLabel())} ${icon('down', 14)}</button>
      <button type="button" class="btn ghost sm" data-act="hpGGroup" data-fid="g-group">${icon('filter', 18)}${esc(hpGroupName(g.group))} ${icon('down', 14)}</button><span class="hp-sp"></span>
      <a class="btn ghost sm" href="#/live/guide/search" data-fid="g-search">${icon('search', 18)}Im Programm suchen</a></div>`;
}
ACT.hpGDay = (d) => { HP.g.day = Number(d.d); if (HP.g.day !== 0 && HP.g.time === 'now') HP.g.time = 20 * 60 + 15; HP.g.sl = null; render(); };
ACT.hpGTime = () => {
  const opts = [['now', 'Jetzt'], [20 * 60 + 15, '20:15 · Hauptabend'], [22 * 60, '22:00 · Spätabend']];
  sheet(`<h2>Uhrzeit</h2>${opts.map(([v, t]) => li('clock', t, '', v === HP.g.time ? icon('check', 20) : '', `data-act="hpGSet" data-v="${v}" data-fid="gt-${v}"`)).join('')}${li('calendar', 'Andere Uhrzeit …', '', null, 'data-act="hpGOther" data-fid="gt-other"')}<div class="acts"><button type="button" class="btn ghost" data-act="close" data-fid="gt-x">Schliessen</button></div>`);
};
ACT.hpGSet = (d) => { closeSheet(true); HP.g.time = d.v === 'now' ? 'now' : Number(d.v); if (d.v === 'now') HP.g.day = 0; HP.g.sl = null; render(); };
ACT.hpGOther = () => LiveDlg.pickTime({ day: HP.g.day, time: hpGuideT() % 1440, onPick: (v) => { HP.g.day = v.day; HP.g.time = v.time; HP.g.sl = null; render(); } });
ACT.hpGGroup = () => LiveDlg.filterSheet({ fields: [{ key: 'group', label: 'Gruppe', options: [{ v: '', t: 'Alle Sender' }, { v: 'fav', t: 'Meine Sender' }, ...LIVE.groups.map((x) => ({ v: x.id, t: x.name }))] }], values: { group: HP.g.group }, onApply: (v) => { HP.g.group = v.group || ''; render(); } });
ACT.hpGNow = () => { HP.g.day = 0; HP.g.time = 'now'; HP.g.sl = null; render(); };

function hpGuideRow(c, t, ids) {
  const nn = SIM.nowNext(c.id, t); const st = SIM.chState(c); const isNow = HP.g.time === 'now' && HP.g.day === 0; const rec = SIM.rights().recordings && SIM.runningOn(c.id);
  const head = `<div class="hp-gr-head"><a class="hp-gr-ch" href="#/live/watch/${c.id}" data-act="hpTune" data-ch="${c.id}" data-list="${ids.join(',')}" data-fid="gr-t-${c.id}" aria-label="${c.no} ${esc(c.name)} einschalten"><span class="lh-no">${c.no}</span>${LU.logo(c)}<span class="lh-name">${esc(c.name)}</span></a>${rec ? LU.badge('recording', { text: true }) : ''}${st === 'disturbed' ? `<span class="hp-warn">${icon('alert', 14)}${esc(LU.TX.unavailable)}</span>` : ''}</div>`;
  let now;
  if (SIM.epgOf(c) === 'none' && !nn.now) now = `<div class="hp-gr-none">${esc(LU.TX.epgNoneButPlays)}</div>`;
  else if (!nn.now) now = `<div class="hp-gr-none">${esc(LU.TX.epgNone)}</div>`;
  else {
    const p = nn.now; const bad = LU.programBadges(p);
    now = `<div class="hp-gr-line"><a class="hp-gr-now" href="#/live/programs/${p.id}" data-hplp="${p.id}" data-fid="gr-n-${c.id}" aria-label="${esc(p.title)}, ${fmt.range(p.start, p.end)}, Details"><b>${esc(p.title)}</b><small>${isNow ? '' : 'Um ' + fmt.time(t) + ' · '}${fmt.range(p.start, p.end)}${isNow ? ' · ' + LU.nowText('left', p.end) : ''}</small>${isNow ? LU.progress(p.start, p.end) : ''}</a>${bad ? `<span class="hp-gr-b">${bad}</span>` : ''}<button type="button" class="ib" data-act="hpSheet" data-p="${p.id}" aria-label="Aktionen für ${esc(p.title)}" data-fid="gr-m-${c.id}">${icon('dots', 22)}</button></div>`;
  }
  const nx = nn.next ? `<a class="hp-gr-next" href="#/live/programs/${nn.next.id}" data-hplp="${nn.next.id}" data-fid="gr-x-${c.id}"><small>danach</small> <small>${fmt.time(nn.next.start)}</small> <b>${esc(nn.next.title)}</b>${LU.programBadges(nn.next)}</a>` : '';
  return `<li class="hp-gr${st !== 'ok' ? ' bad' : ''}">${head}${now}${nx}</li>`;
}
/* tablet: grid channels x time */
function hpGuideGrid(chs, day) {
  const from = day * 1440; const to = from + 1440; const W = 1440 * HP_PX; const t = SIM.now(); const rec = SIM.rights().recordings;
  const axis = Array.from({ length: 48 }, (_, i) => `<span class="hp-ax" style="left:${i * 30 * HP_PX}px">${i % 2 ? '' : fmt.time(i * 30)}</span><i class="hp-tick${i % 2 ? '' : ' hr'}" style="left:${i * 30 * HP_PX}px"></i>`).join('');
  const now = t >= from && t < to ? `<i class="hp-now" data-from="${from}" style="left:${96 + (t - from) * HP_PX}px" aria-hidden="true"></i>` : '';
  const rows = chs.map((c) => {
    const list = SIM.programsOf(c.id, from, to); const gaps = SIM.gapsOf(c.id, from, to); const none = SIM.epgOf(c) === 'none';
    const cells = list.map((p) => {
      const a = Math.max(p.start, from); const b = Math.min(p.end, to); const run = hpIsNow(p); const past = p.end <= t;
      const rs = rec ? SIM.recState(p.id) : null; const rem = SIM.reminderFor(p.id);
      const w = Math.max(24, (b - a) * HP_PX - 2);
      const label = `${c.name}, ${p.title}, ${fmt.time(p.start)} bis ${fmt.time(p.end)}${run ? ', läuft' : ''}${rs ? ', ' + LU.TX[{ recording: 'recording', series: 'series', scheduled: 'scheduled' }[rs]] : ''}${rem ? ', Erinnerung' : ''}`;
      return `<button type="button" class="hp-gc${run ? ' cur' : ''}${past ? ' past' : ''}" style="left:${(a - from) * HP_PX}px;width:${w}px" data-act="hpSheet" data-p="${p.id}" aria-label="${esc(label)}" data-fid="gc-${p.id}"><b>${esc(p.title)}</b><small>${fmt.time(p.start)}</small><span class="hp-gc-b">${LU.programBadges(p)}</span>${run ? `<span class="hp-ovprog">${LU.progress(p.start, p.end)}</span>` : ''}</button>`;
    }).join('');
    const gcells = gaps.map(([a, b]) => `<span class="hp-gc gap lv-gap" style="left:${(a - from) * HP_PX}px;width:${Math.max(24, (b - a) * HP_PX - 2)}px"><span class="lv-gap-text">${esc(none ? LU.TX.epgNoneButPlays : LU.TX.epgNone)}</span></span>`).join('');
    return `<div class="hp-grow"><a class="hp-gch" href="#/live/watch/${c.id}" data-act="hpTune" data-ch="${c.id}" data-list="${chs.map((x) => x.id).join(',')}" data-fid="gh-${c.id}" aria-label="${c.no} ${esc(c.name)} einschalten"><span class="lh-no">${c.no}</span>${LU.logo(c)}</a><div class="hp-gcells" style="width:${W}px">${cells}${gcells}</div></div>`;
  }).join('');
  return `<div class="hp-gscroll" data-hpgs tabindex="0" role="region" aria-label="Programmraster, waagrecht wischbar"><div class="hp-gbody" style="width:${96 + W}px"><div class="hp-grow hp-gaxisrow"><span class="hp-gch hp-gcorner"><button type="button" class="btn ghost sm" data-act="hpGNow" data-fid="g-jetzt">Jetzt</button></span><div class="hp-gaxis" style="width:${W}px">${axis}</div></div>${rows}${now}</div></div>`;
}
route('/live/guide', () => {
  let body = hpState(6, 96);
  if (!body) {
    const chs = SIM.channels({ group: HP.g.group }); const t = hpGuideT(); const ids = chs.map((c) => c.id);
    const none = SIM.channels().filter((c) => SIM.epgOf(c) === 'none').length;
    if (!SIM.channels().length) body = hpEmptyBox('live', 'Keine Sender', LU.TX.empty);
    else if (!chs.length) body = hpEmptyBox('filter', 'Keine Sender in dieser Gruppe', 'Wähle eine andere Gruppe.', `<button type="button" class="btn ghost" data-act="hpGGroup" data-fid="g-grp2">Gruppe wählen</button>`);
    else body = `${SIM.flags().noEpg ? `<div class="note">${icon('info', 20)}<div>Für die Sender gibt es gerade keine Programmdaten. Fernsehen geht trotzdem.</div></div>` : ''}
      <ul class="hp-glist hp-phone-only" aria-label="Jetzt und danach, ${esc(hpGuideTimeLabel())}">${chs.map((c) => hpGuideRow(c, t, ids)).join('')}</ul>
      <div class="hp-tablet-only">${hpGuideGrid(chs, HP.g.day)}</div>`;
  }
  return { nav: '', html: livePage({ tab: 'guide', title: 'Programm', html: `${hpGuideTools()}${body}` }), after: () => hpGridScroll() };
});
function hpGridScroll() {
  const s = $('[data-hpgs]'); if (!s || !s.offsetWidth) return;
  const t = hpGuideT(); const from = HP.g.day * 1440;
  s.scrollLeft = HP.g.sl != null ? HP.g.sl : Math.max(0, (t - from - 40) * HP_PX);
}
document.addEventListener('scroll', (e) => { if (e.target && e.target.dataset && e.target.dataset.hpgs != null) HP.g.sl = e.target.scrollLeft; }, true);
SIM.on((tp) => { if (tp !== 'minute') return; const t = SIM.now(); $$('.hp-now').forEach((el) => { el.style.left = (96 + (t - Number(el.dataset.from)) * HP_PX) + 'px'; }); });
/* mouse swipe on the grid (a finger scrolls by itself) */
(function hpGridDrag() {
  let st = null;
  document.addEventListener('pointerdown', (e) => { const s = e.target.closest('[data-hpgs]'); if (!s || e.pointerType !== 'mouse') return; st = { s, x: e.clientX, y: e.clientY, sl: s.scrollLeft, moved: false }; });
  document.addEventListener('pointermove', (e) => { if (!st) return; const sc = $('#device').getBoundingClientRect().width / $('#device').offsetWidth || 1; const dx = (e.clientX - st.x) / sc; if (Math.abs(dx) > 5) st.moved = true; if (st.moved) st.s.scrollLeft = st.sl - dx; });
  ['pointerup', 'pointercancel'].forEach((n) => document.addEventListener(n, () => { if (st && st.moved) HP.dragged = Date.now(); st = null; }));
  document.addEventListener('click', (e) => { if (Date.now() - HP.dragged < 300 && e.target.closest('[data-hpgs]')) { e.stopImmediatePropagation(); e.preventDefault(); } }, true);
})();

/* ============================================================ LS-05 Programmsuche */
document.addEventListener('input', (e) => {
  const el = e.target.closest('[data-hpq]'); if (!el) return;
  HP.s.q = el.value; HP.s.limit = 40; render(); const n = $('[data-hpq]'); if (n) { n.focus(); n.setSelectionRange(n.value.length, n.value.length); }
});
route('/live/guide/search', (p, q) => {
  if (HP.s.key !== location.hash) HP.s = { q: q.q || '', when: q.when || (q.genre ? 'now' : 'week'), genre: q.genre || '', group: q.group || '', limit: 40, key: location.hash };
  const s = HP.s; const whens = [['now', 'Jetzt'], ['today', 'Heute'], ['tomorrow', 'Morgen'], ['week', 'Nächste 7 Tage']];
  const nf = (s.genre ? 1 : 0) + (s.group ? 1 : 0);
  const tools = `<div class="hp-tools"><label class="lh-search"><span class="vh">Sendungen suchen</span>${icon('search', 20)}<input class="inp" type="search" placeholder="Sendungen suchen" value="${esc(s.q)}" data-hpq data-fid="s-q" aria-label="Sendungen suchen (Titel, Episode, Beschreibung)"></label>
    <button type="button" class="btn ghost sm lh-filter${nf ? ' on' : ''}" data-act="hpSFilter" data-fid="s-filter">${icon('filter', 18)}Filter${nf ? ` (${nf})` : ''}</button></div>
    <div class="chips" role="group" aria-label="Wann">${whens.map(([v, t]) => `<button type="button" class="chip${s.when === v ? ' on' : ''}" aria-pressed="${s.when === v}" data-act="hpSWhen" data-v="${v}" data-fid="sw-${v}">${t}</button>`).join('')}</div>
    ${nf ? `<div class="chips">${s.genre ? `<button type="button" class="chip on" data-act="hpSUnset" data-k="genre" data-fid="su-genre" aria-label="Filter Genre ${esc(s.genre)} entfernen">${esc(s.genre)} ${icon('close', 14)}</button>` : ''}${s.group ? `<button type="button" class="chip on" data-act="hpSUnset" data-k="group" data-fid="su-group" aria-label="Filter Gruppe entfernen">${esc(hpGroupName(s.group))} ${icon('close', 14)}</button>` : ''}</div>` : ''}`;
  let body = hpState(6, 84);
  if (!body) {
    const part = SIM.channels().filter((c) => c.epg === 'partial'); const none = SIM.channels().filter((c) => c.epg === 'none');
    const note = SIM.flags().noEpg ? `<div class="note">${icon('info', 20)}<div>Für alle Sender fehlen gerade die Programmdaten.</div></div>`
      : (none.length + part.length ? `<div class="note">${icon('info', 20)}<div>Für ${none.length + part.length} Sender gibt es keine Programmdaten: ${[...none.map((c) => c.name), ...part.map((c) => c.name + ' (teilweise)')].join(', ')}.</div></div>` : '');
    if (!SIM.channels().length) body = hpEmptyBox('live', 'Keine Sender', LU.TX.empty);
    else if (!s.q.trim() && !s.genre) body = `${note}${hpEmptyBox('search', 'Programm durchsuchen', 'Gib einen Titel ein oder wähle ein Genre.')}<div class="chips lh-wrap">${hpGenres().map((g) => `<button type="button" class="chip" data-act="hpSGenre" data-g="${esc(g)}" data-fid="sg-${esc(g)}">${esc(g)}</button>`).join('')}</div>`;
    else {
      const res = SIM.search(s.q, { when: s.when, genre: s.genre, group: s.group });
      if (!res.length) body = `${note}${hpEmptyBox('search', 'Nichts gefunden', 'Keine Sendung gefunden. Programmdaten reichen bis Fr 4.12.')}`;
      else {
        const shown = res.slice(0, s.limit); let day = null; let html = '';
        shown.forEach((p) => { const d = LU.dayOf(p.start); if (d !== day) { if (day !== null) html += '</ul>'; day = d; html += `<div class="sec">${esc(fmt.dayRel(p.start) === fmt.day(p.start) ? fmt.dayLong(p.start) : fmt.dayRel(p.start))}</div><ul class="hp-plist">`; } html += hpPR(p, { k: 's' }); });
        html += '</ul>';
        body = `${note}<p class="pad hint" role="status">${res.length} ${res.length === 1 ? 'Treffer' : 'Treffer'}</p>${html}${res.length > shown.length ? `<p class="pad" style="margin-top:12px"><button type="button" class="btn ghost block" data-act="hpSMore" data-fid="s-more">Mehr anzeigen (${res.length - shown.length})</button></p>` : ''}`;
      }
    }
  }
  return { nav: '', html: `${hpBackBar('Programm suchen')}${tools}${body}<p class="lh-sample">${esc(LU.TX.sample)}</p>` };
});
ACT.hpSWhen = (d) => { HP.s.when = d.v; HP.s.limit = 40; render(); };
ACT.hpSUnset = (d) => { HP.s[d.k] = ''; HP.s.limit = 40; render(); };
ACT.hpSGenre = (d) => { HP.s.genre = d.g; HP.s.when = 'now'; render(); };
ACT.hpSMore = () => { HP.s.limit += 40; render(); };
ACT.hpSFilter = () => LiveDlg.filterSheet({ fields: [{ key: 'genre', label: 'Genre', options: [{ v: '', t: 'Alle' }, ...hpGenres().map((g) => ({ v: g, t: g }))] }, { key: 'group', label: 'Gruppe', options: [{ v: '', t: 'Alle Sender' }, { v: 'fav', t: 'Meine Sender' }, ...LIVE.groups.map((x) => ({ v: x.id, t: x.name }))] }], values: { genre: HP.s.genre, group: HP.s.group }, onApply: (v) => { Object.assign(HP.s, { genre: v.genre || '', group: v.group || '', limit: 40 }); render(); } });

/* ============================================================ LS-06 Sendungsdetails */
route('/live/programs/:id', (pm) => {
  const p = SIM.program(pm.id); const m = SIM.mode();
  if (m === 'loading') return { nav: '', html: `${hpBackBar('Sendung')}<div class="pad hp-skel"><span class="lv-skel" style="height:200px"></span><span class="lv-skel" style="height:34px"></span><span class="lv-skel" style="height:60px"></span><span class="lv-skel" style="height:120px"></span></div>` };
  if (m === 'error') return { nav: '', html: `${hpBackBar('Sendung')}${hpState()}` };
  if (m === 'empty') return { nav: '', html: `${hpBackBar('Sendung')}${hpEmptyBox('live', 'Noch keine Sender', LU.TX.empty)}` };
  if (!p || SIM.epgOf(p.ch) === 'none') return hpNotFound('Diese Sendung steht nicht mehr im Programm.', p ? `#/live/guide/channel/${p.ch}` : '#/live/guide', 'Zum Senderprogramm');
  const c = SIM.channel(p.ch); const r = SIM.rights(); const t = SIM.now(); const run = hpIsNow(p); const past = p.end <= t;
  const rs = r.recordings && !past ? SIM.recState(p.id) : null; const rem = SIM.reminderFor(p.id);
  const mine = r.recordings ? SIM.recordings().find((x) => x.ch === p.ch && (x.program === p.id || (x.start < p.end && x.end > p.start && x.state !== 'failed' && x.state !== 'rejected'))) : null;
  const chips = [p.genre, ...(p.flags || []), p.repeat ? 'Wiederholung' : '', p.firstRun ? 'Erstausstrahlung' : '', p.live ? 'Live' : ''].filter(Boolean);
  const ra = (ic, label, act, extra, on, fid) => `<button type="button" class="hp-ra${on ? ' on' : ''}" data-act="${act}" ${extra} data-fid="${fid}"><span class="hp-ra-i">${icon(ic, 24)}</span><span>${esc(label)}</span></button>`;
  const same = LIVE.programs.filter((x) => x.id !== p.id && x.start >= t && x.start < t + 7 * 1440 && (p.series ? x.series === p.series : x.title === p.title) && SIM.epgOf(x.ch) !== 'none').sort((a, b) => a.start - b.start).slice(0, 8);
  const facts = [['Folge', p.season && p.episode ? `Staffel ${p.season}, Folge ${p.episode}` : p.episode ? `Folge ${p.episode}` : ''], ['Jahr', p.year], ['Land', p.country], ['Regie', p.director], ['Mitwirkende', (p.cast || []).join(', ')]].filter((x) => x[1]);
  const html = `${hpBackBar('Sendung')}
    <div class="hp-head"><div class="hp-head-pic">${LU.picture(p, { fallback: 'logo' })}</div>
      <div class="pad"><h2 class="hp-h2">${esc(p.title)}</h2>${p.sub ? `<p class="hp-sub">${esc(p.sub)}</p>` : ''}<p class="hp-meta">${esc(c.name)} · ${esc(hpWhen(p))} · ${fmt.dur(p.end - p.start)}</p>
        <div class="hp-chips">${chips.map((x) => `<span class="badge">${esc(x)}</span>`).join('')}${LU.programBadges(p, { text: true })}</div></div></div>
    <div class="pad hp-state">${run ? `${LU.progress(p.start, p.end)}<p><b>Läuft</b> · ${LU.nowText('left', p.end)}</p>` : past ? `<p>${esc(LU.TX.past)}</p>${mine ? `<p><a class="hp-lnk" href="#/live/recordings/${mine.id}" data-fid="d-myrec">Deine Aufnahme ansehen</a></p>` : ''}` : `<p>${fmt.dayRel(p.start)}, ab ${fmt.time(p.start)}</p>`}</div>
    <div class="hp-ras" role="toolbar" aria-label="Aktionen">
      ${run ? ra('play', 'Einschalten', 'hpTune', `data-ch="${c.id}"`, false, 'd-tune') : ''}
      ${r.recordings && !past ? ra(rs === 'recording' ? 'record' : 'recordHollow', rs === 'recording' ? 'Nimmt auf' : rs ? (rs === 'series' ? 'Serie geplant' : 'Aufnahme geplant') : 'Aufnehmen', 'hpRec', `data-p="${p.id}"`, !!rs, 'd-rec') : ''}
      ${r.recordings && !past && hpSerial(p) ? ra('recordSeries', 'Serie aufnehmen', 'hpSeries', `data-p="${p.id}"`, rs === 'series', 'd-ser') : ''}
      ${!past && !run ? ra('reminder', rem ? `Erinnerung um ${fmt.time(p.start - rem.lead)}` : 'Erinnern', 'hpRem', `data-p="${p.id}"`, !!rem, 'd-rem') : ''}
      <a class="hp-ra" href="#/live/guide/channel/${c.id}" data-fid="d-prog"><span class="hp-ra-i">${icon('calendar', 24)}</span><span>Senderprogramm</span></a></div>
    ${p.desc ? `<div class="pad"><p class="hp-desc">${esc(p.desc)}</p></div>` : ''}
    ${facts.length ? `<div class="sec">Angaben</div><dl class="hp-facts">${facts.map(([k, v]) => `<div><dt>${esc(k)}</dt><dd>${esc(v)}</dd></div>`).join('')}</dl>` : ''}
    ${same.length ? `<div class="sec">Weitere Termine</div><ul class="hp-plist">${same.map((x) => hpPR(x, { k: 'w' })).join('')}</ul>` : ''}
    <p class="lh-sample">${esc(LU.TX.sample)}</p>`;
  return { nav: '', html };
});

/* ============================================================ LS-15 Senderprogramm */
route('/live/guide/channel/:id', (pm) => {
  const c = SIM.channel(pm.id); const m = SIM.mode();
  if (m === 'empty') return { nav: '', html: `${hpBackBar('Senderprogramm')}${hpEmptyBox('live', 'Noch keine Sender', LU.TX.empty)}` };
  if (!c || SIM.chState(c) === 'gone') return hpNotFound('Diesen Sender gibt es nicht mehr.', '#/live/channels', 'Zu den Sendern');
  const key = pm.id; if (HP.cd.key !== key) HP.cd = { day: 0, key };
  const day = HP.cd.day; const from = day * 1440; const to = from + 1440; const r = SIM.rights(); const t = SIM.now(); const st = SIM.chState(c);
  const head = `<header class="hp-chh">${LU.logo(c, { label: true })}<div class="tx"><b>${c.no} ${esc(c.name)}</b><small>${st === 'disturbed' ? esc(LU.TX.unavailable) : esc(c.category)}</small></div>
    <button type="button" class="ib hp-heart${SIM.isFav(c.id) ? ' on' : ''}" data-act="hpFav" data-ch="${c.id}" aria-pressed="${SIM.isFav(c.id)}" aria-label="${esc(c.name)} ${SIM.isFav(c.id) ? 'aus Meine Sender entfernen' : 'zu Meine Sender hinzufügen'}" data-fid="ch-fav">${icon(SIM.isFav(c.id) ? 'heartOn' : 'heart', 22)}</button></header>
    <div class="hp-tools"><button type="button" class="btn primary sm" data-act="hpTune" data-ch="${c.id}" data-fid="ch-tune">${icon('play', 18)}Einschalten</button></div>
    <div class="chips" role="group" aria-label="Tag">${hpDays().map((d) => `<button type="button" class="chip${day === d ? ' on' : ''}" aria-pressed="${day === d}" data-act="hpCDay" data-d="${d}" data-fid="cd-${d}">${esc(fmt.dayRel(d * 1440))}</button>`).join('')}</div>`;
  let body = hpState(8, 64);
  if (!body) {
    const list = SIM.programsOf(c.id, from, to); const gaps = SIM.gapsOf(c.id, from, to);
    if (SIM.epgOf(c) === 'none' || (!list.length && day === 0 && gaps.length === 1)) {
      body = `<div class="empty">${icon('calendar', 40)}<b>Keine Programmdaten</b><p>${SIM.epgOf(c) === 'none' ? 'Für diesen Sender gibt es keine Programmdaten.' : 'Keine Programmdaten für diesen Tag.'}</p></div><div class="pad hp-btnrow"><button type="button" class="btn secondary" data-act="hpTune" data-ch="${c.id}" data-fid="ch-tune2">${icon('play', 18)}Einschalten</button>${r.recordings ? `<button type="button" class="btn secondary" data-act="hpManual" data-ch="${c.id}" data-fid="ch-manual">${icon('recordHollow', 18)}Zeit aufnehmen</button>` : ''}</div>`;
    } else {
      const items = [...list.map((p) => ({ a: Math.max(p.start, from), p })), ...gaps.map((g) => ({ a: g[0], g }))].sort((x, y) => x.a - y.a);
      body = `<ul class="hp-plist hp-cprog">${items.map((it) => it.p ? hpPR(it.p, { noCh: true, k: 'c' }).replace('class="hp-pr', 'data-now="' + (hpIsNow(it.p) ? 1 : 0) + '" class="hp-pr') : `<li class="hp-pr gapr"><span class="hp-pr-main lv-gap"><span class="hp-pr-time"><b>${fmt.time(it.g[0])}</b><small>${it.g[1] >= to ? '24:00' : fmt.time(it.g[1])}</small></span><span class="hp-pr-body"><b class="hp-pr-t">${esc(LU.TX.epgNone)}</b></span></span>${r.recordings && it.g[1] > t ? `<span class="hp-pr-act"><button type="button" class="btn ghost sm" data-act="hpManual" data-ch="${c.id}" data-a="${Math.max(it.g[0], t)}" data-b="${it.g[1]}" data-fid="ch-gap-${it.g[0]}">Zeit aufnehmen</button></span>` : ''}</li>`).join('')}</ul>`;
    }
  }
  return { nav: '', html: `${hpBackBar('Senderprogramm')}${head}${body}<p class="lh-sample">${esc(LU.TX.sample)}</p>`, after: () => { if (day === 0) { const el = $('[data-now="1"]'); if (el && HP.cd.fresh !== true) { HP.cd.fresh = true; el.scrollIntoView({ block: 'center' }); } } } };
});
ACT.hpCDay = (d) => { HP.cd.day = Number(d.d); HP.cd.fresh = false; render(); };
ACT.hpManual = (d) => LiveDlg.manualRecording({ ch: d.ch, start: d.a != null ? Number(d.a) : undefined, end: d.b != null ? Number(d.b) : undefined });
