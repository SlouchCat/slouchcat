/* Web Zuschauer A (part 1): shared helpers (prefix «va»), LS-01 Live-TV-Start, LS-03 Meine Sender,
   LS-05 Programmsuche. 14-oberflaeche.md 3.1, 3.3, 3.5. Part 2 (LS-06, LS-15, LS-21 … LS-26) is in viewer-a-2.js.
   Only the shared API of BAUKASTEN.md is used (route, livePage, SIM, LU, LiveDlg, popMenu …). */
'use strict';

/* transient state of this agent (never persisted) */
ui.va = ui.va || { rowAct: {}, listAct: {}, ord: false, ordMsg: '', more: 50, radio: null, read: {}, pf: { sel: 'anna', d: {} }, nt: null, scrolledKey: null };
const VA = ui.va;
window.addEventListener('hashchange', () => { VA.scrolledKey = null; VA.more = 50; VA.ord = false; });

/* ---------- small helpers ---------- */
const vaNowTx = (p) => `${fmt.range(p.start, p.end)} · ${LU.nowText('left', p.end)}`;
const vaAttrs = (d) => Object.entries(d || {}).map(([k, v]) => `data-${k}="${esc(v)}"`).join(' ');
/** time text; the end of a day reads 24:00 */
const vaT = (t, dayEnd) => (dayEnd != null && t === dayEnd ? '24:00' : fmt.time(t));
const vaIsSeries = (p) => !!(p.series || p.episode);
const vaSeriesRule = (p) => SIM.rules().find((r) => r.kind === 'series' && r.series === (p.series || p.title)) || null;
/** channels in display order: «Meine Sender» first (their order), then by number */
function vaOrdered() {
  const all = SIM.channels(); const fav = SIM.favorites().map((id) => all.find((c) => c.id === id)).filter(Boolean);
  return fav.concat(all.filter((c) => !fav.includes(c)));
}
/** the Live-TV area when radio plays: a thin dock (LS-25). Live-TV has priority and ends it (see viewer-a-2.js). */
function vaDock() {
  const s = VA.radio; if (!s) return '';
  return `<div class="va-dock" role="region" aria-label="Radio, laufende Wiedergabe"><span class="va-dock-ic">${icon('radio', 18)}</span><span class="va-dock-t"><b>${esc(s.name)}</b><span>${esc(s.line)}</span></span><button type="button" class="btn ghost icon sm" data-act="vaRadioStop" data-fid="dock:stop" aria-label="Radio beenden">${icon('x', 16)}</button></div>`;
}
/** state of the page for the four mockup modes; null = show the page */
function vaMode(skel, errText) {
  const m = liveMode();
  if (m === 'loading') return `<div class="va-skel" aria-busy="true" aria-label="Wird geladen">${skel}</div>`;
  if (m === 'error') return liveError(errText);
  return null;
}
const vaSkelRow = (n, w, h) => `<div class="va-skelrow"><span class="lv-skel" style="width:180px;height:26px"></span><div class="va-skelcards">${Array.from({ length: n }, () => `<span class="lv-skel" style="width:${w}px;height:${h}px"></span>`).join('')}</div></div>`;
const vaNotFound = (title, text, acts) => ({ type: 'viewer', nav: 'live', title: 'Nicht gefunden', html: `<div class="page live"><div class="head"><h1>${title}</h1><p class="sub">${text}</p>${acts ? `<div class="acts">${acts}</div>` : ''}</div>${LU.sampleNote()}</div>` });

/* ---------- ProgramCard / ChannelCard of LS-01 ---------- */
/** where: id of the row (menu ids, tab stop). kind: 'now' | 'soon' */
function vaProgCard(p, kind, where, ids, listName) {
  const c = SIM.channel(p.ch); const bad = SIM.chState(c) !== 'ok'; const badges = LU.programBadges(p, { text: false });
  const art = `<span class="va-art">${LU.picture(p)}${p.img ? `<span class="va-pl">${LU.logo(c)}</span>` : ''}<span class="va-no">${c.no}</span></span>`;
  const sub = p.sub ? `<span class="va-s">«${esc(p.sub)}»</span>` : '';
  const body = kind === 'now'
    ? `<span class="va-body"><b class="va-t">${esc(p.title)}</b>${sub}<span class="va-m">${esc(c.name)} · ${vaNowTx(p)}</span>${LU.progress(p.start, p.end, { label: `Fortschritt von ${p.title}` })}</span>`
    : `<span class="va-body"><b class="va-t">${esc(p.title)}</b>${sub}<span class="va-m">${esc(c.name)} · ab ${fmt.time(p.start)} · in ${Math.max(1, Math.round(p.start - SIM.now()))} Min.</span></span>`;
  const tail = `${bad ? `<span class="va-flag"><span class="chip warn">${icon('alert', 14)}${esc(LU.TX.disturbed)}</span></span>` : ''}${badges ? `<span class="va-bdg">${badges}</span>` : ''}`;
  const main = kind === 'now'
    ? `<button type="button" class="va-main" data-act="tune" data-ch="${c.id}" data-list="${ids.join(',')}" data-list-name="${listName}" data-fid="vc:${where}:${p.id}" tabindex="-1" aria-label="${esc(c.no + ' ' + c.name + ' einschalten, jetzt ' + p.title)}">${art}${body}${tail}</button>`
    : `<a class="va-main" href="#/live/programs/${p.id}" data-fid="vc:${where}:${p.id}" tabindex="-1" aria-label="${esc(p.title + ', ' + c.name + ', ab ' + fmt.time(p.start) + ', Details')}">${art}${body}${tail}</a>`;
  const rights = SIM.rights(); const items = [{ label: 'Details', href: `#/live/programs/${p.id}`, icon: 'info' }];
  if (kind === 'now') items.push({ label: 'Einschalten', act: 'tune', data: { ch: c.id }, icon: 'play' });
  if (rights.recordings) items.push({ label: SIM.recState(p.id) ? 'Aufnahme ändern' : 'Aufnehmen', act: 'vaRec', data: { id: p.id }, icon: 'record' });
  items.push({ label: 'Zum Senderprogramm', href: `#/live/guide/channel/${c.id}`, icon: 'calendar' });
  return `<li class="va-card${bad ? ' bad' : ''}" data-k="${p.id}">${main}<span class="va-menu">${popMenu(`va:${where}:${p.id}`, icon('dots', 18), items, { aria: `Weitere Aktionen für ${p.title}` }).replace('data-act="menu"', 'data-act="menu" tabindex="-1"')}</span></li>`;
}
function vaChanCard(c, where, ids, listName) {
  const nn = SIM.nowNext(c.id); const bad = SIM.chState(c) !== 'ok'; const fav = SIM.isFav(c.id);
  const line = bad ? `<span class="va-m warnline"><span class="dot warn"></span>${esc(LU.TX.unavailable)}</span>`
    : nn.now ? `<span class="va-m">${esc(nn.now.title)}${nn.now.sub ? ` «${esc(nn.now.sub)}»` : ''}</span>${LU.progress(nn.now.start, nn.now.end, { label: `Fortschritt von ${nn.now.title}` })}`
      : `<span class="va-m">${esc(SIM.epgOf(c) === 'none' ? LU.TX.epgNoneButPlays : LU.TX.epgNone)}</span>`;
  const main = `<button type="button" class="va-main" data-act="tune" data-ch="${c.id}" data-list="${ids.join(',')}" data-list-name="${listName}" data-fid="vc:${where}:${c.id}" tabindex="-1" aria-label="${esc(c.no + ' ' + c.name + ' einschalten' + (nn.now && !bad ? ', jetzt ' + nn.now.title : ''))}"><span class="va-art chan">${LU.logo(c, { cls: 'wide' })}<span class="va-no">${c.no}</span></span><span class="va-body"><b class="va-t">${esc(c.name)}${fav ? '<span class="vh">, in Meine Sender</span>' : ''}</b>${line}</span></button>`;
  const items = [{ label: 'Senderprogramm', href: `#/live/guide/channel/${c.id}`, icon: 'calendar' }];
  if (nn.now) items.push({ label: 'Jetzige Sendung: Details', href: `#/live/programs/${nn.now.id}`, icon: 'info' });
  items.push({ label: 'Einschalten', act: 'tune', data: { ch: c.id }, icon: 'play' });
  items.push({ label: fav ? 'Aus Meine Sender' : 'Zu Meine Sender', act: 'fav', data: { ch: c.id }, icon: fav ? 'x' : 'heart' });
  return `<li class="va-card chan${bad ? ' bad' : ''}" data-k="${c.id}">${main}<span class="va-menu">${popMenu(`va:${where}:${c.id}`, icon('dots', 18), items, { aria: `Weitere Aktionen für ${c.name}` }).replace('data-act="menu"', 'data-act="menu" tabindex="-1"')}</span></li>`;
}
/** one horizontal row (Row, variant cards/tiles): one Tab stop, arrow keys inside */
function vaRow(id, title, lis, o) {
  o = o || {};
  const act = VA.rowAct[id]; const keys = lis.map((h) => (h.match(/data-k="([^"]+)"/) || [])[1]);
  const at = keys.includes(act) ? act : keys[0];
  const html = lis.map((h, i) => (keys[i] === at ? h.replace(/(class="va-main"[^>]*?) tabindex="-1"/, '$1 tabindex="0"') : h)).join('');
  return `<section class="va-row" aria-labelledby="vh-${id}"><div class="va-rowhead"><h2 id="vh-${id}">${title}</h2>${o.meta ? `<span class="va-rmeta">${o.meta}</span>` : ''}${o.all ? `<a class="lnk va-all" href="${o.all[0]}" data-fid="va:all:${id}">${o.all[1]}</a>` : ''}<span class="va-paging"><button type="button" class="btn ghost icon sm" data-act="scrollRow" data-row="va-${id}" data-dir="prev" aria-label="${esc(title)}: zurückblättern" tabindex="-1" data-fid="va:pg:${id}:p">${icon('chevL', 18)}</button><button type="button" class="btn ghost icon sm" data-act="scrollRow" data-row="va-${id}" data-dir="next" aria-label="${esc(title)}: weiterblättern" tabindex="-1" data-fid="va:pg:${id}:n">${icon('chevR', 18)}</button></span></div><div class="va-scroll" data-keep="va-${id}"><ul class="va-cards" data-vrow="${id}" aria-label="${esc(title)}, Pfeiltasten wechseln">${html}</ul></div></section>`;
}
/** link card (LinkCard) */
const vaLink = (href, ic, title, meta, fid) => `<a class="va-lc" href="${href}" data-fid="${fid}"><span class="va-lc-ic">${icon(ic, 22)}</span><span class="va-lc-t"><b>${title}</b>${meta ? `<span>${meta}</span>` : ''}</span><span class="va-lc-go">${icon('chevR', 18)}</span></a>`;

/* ---------- ProgramRow (LS-05, LS-06, LS-15) ---------- */
function vaRecBtn(p) {
  if (!SIM.rights().recordings || p.end <= SIM.now()) return '';
  const rs = SIM.recState(p.id);
  const label = rs === 'recording' ? `«${p.title}» nimmt auf, Details` : rs ? `Aufnahme von «${p.title}» ändern` : `«${p.title}» aufnehmen`;
  const ic = rs === 'series' ? 'recordSeries' : 'record';
  return `<button type="button" class="btn ghost icon sm va-act va-rec${rs ? ' on' : ''}${rs === 'recording' ? ' run' : ''}" data-act="vaRec" data-id="${p.id}" aria-pressed="${!!rs}" aria-label="${esc(label)}" title="${esc(rs === 'recording' ? LU.TX.recording : rs ? LU.TX.scheduled : LU.TX.record)}" data-fid="rec:${p.id}" tabindex="-1">${icon(ic, 18)}</button>`;
}
function vaRemBtn(p) {
  if (p.start <= SIM.now()) return '';
  const on = !!SIM.reminderFor(p.id);
  return `<button type="button" class="btn ghost icon sm va-act va-rem${on ? ' on' : ''}" data-act="vaRem" data-id="${p.id}" aria-pressed="${on}" aria-label="${esc(on ? `Erinnerung an «${p.title}» ändern` : `An «${p.title}» erinnern`)}" title="${on ? 'Erinnerung gesetzt' : 'Erinnern'}" data-fid="rem:${p.id}" tabindex="-1">${icon('reminder', 18)}</button>`;
}
/** o: { noCh (hide channel), cur (mark as running line) } */
function vaProgRow(p, o) {
  o = o || {}; const t = SIM.now(); const c = SIM.channel(p.ch); const running = p.start <= t && p.end > t; const past = p.end <= t;
  let badges = LU.programBadges(p, { text: false });
  /* skipped by a series rule: say why (Aufgenommen / Schon geplant), from the rule's list */
  if (SIM.rights().recordings && !SIM.recState(p.id) && !past) { const sk = SIM.rules().map((x) => (x.skipped || []).find((s) => s[0] === p.id)).find(Boolean); if (sk) badges += LU.badge('scheduled', { text: true, label: sk[1] === 'schon aufgenommen' ? 'Schon aufgenommen' : sk[1] === 'Wiederholung' ? 'Wiederholung übersprungen' : 'Schon geplant' }); }
  const play = running && SIM.chState(c) !== 'gone' ? `<button type="button" class="btn secondary sm va-act va-play" data-act="tune" data-ch="${c.id}" data-fid="play:${p.id}" tabindex="-1" aria-label="${esc(c.name)} einschalten">${icon('play', 14)}<span>Einschalten</span></button>` : '';
  return `<li class="va-prow${running ? ' now' : ''}${past ? ' past' : ''}" data-k="${p.id}"${running && o.cur ? ' aria-current="time"' : ''}>
    <a class="va-pmain" href="#/live/programs/${p.id}" data-fid="pm:${p.id}" tabindex="-1" aria-label="${esc(`${fmt.range(p.start, p.end)}${o.noCh ? '' : ', ' + c.name}, ${p.title}${p.sub ? ', ' + p.sub : ''}${running ? ', läuft' : past ? ', vorbei' : ''}`)}">
      <span class="va-time">${fmt.range(p.start, p.end)}${past ? '<span class="va-past">vorbei</span>' : ''}</span>
      ${o.noCh ? '' : `<span class="va-ch">${LU.logo(c)}<span class="va-chn">${esc(c.name)}</span></span>`}
      <span class="va-pt"><b>${esc(p.title)}</b>${p.sub ? `<span class="va-ep">«${esc(p.sub)}»</span>` : ''}<span class="va-gn">${esc(p.genre)}${p.repeat ? ' · Wiederholung' : ''}</span>${running ? `<span class="va-pg">${LU.progress(p.start, p.end, { label: `Fortschritt von ${p.title}` })}<span class="va-m">${LU.nowText('left', p.end)}</span></span>` : ''}</span></a>
    <span class="va-bdg">${badges}</span><span class="va-acts">${play}${vaRecBtn(p)}${vaRemBtn(p)}</span></li>`;
}
/** list wrapper: one Tab stop (the remembered row), ↑/↓ rows, → actions */
function vaList(id, lis, label, cls) {
  const keys = lis.map((h) => (h.match(/data-k="([^"]+)"/) || [])[1]);
  const act = VA.listAct[id]; const at = act && keys.includes(act) ? act : keys.find(Boolean);
  const html = lis.map((h, i) => (keys[i] === at && /class="va-pmain"/.test(h) ? h.replace(/(class="va-pmain"[^>]*?) tabindex="-1"/, '$1 tabindex="0"') : h)).join('');
  return `<ul class="va-plist${cls ? ' ' + cls : ''}" data-vlist="${id}" aria-label="${esc(label)}">${html}</ul>`;
}

/* ---------- actions of the program rows ---------- */
ACT.vaRec = (d) => {
  const p = SIM.program(d.id); if (!p) return;
  if (SIM.recState(p.id) === 'recording') { go('#/live/programs/' + p.id); return; }
  LiveDlg.planRecording(p.id);
};
ACT.vaRem = (d) => { if (SIM.program(d.id)) LiveDlg.reminder(d.id); };

/* ======================================================================================
   LS-01 Live-TV-Start
   ====================================================================================== */
const VA_GENRE_ORDER = ['Nachrichten', 'Sport', 'Spielfilm', 'Serie', 'Krimi', 'Dokumentation', 'Kinder', 'Musik', 'Show', 'Kultur', 'Magazin', 'Unterhaltung', 'Wissen', 'Talk', 'Politik', 'Religion'];
route('/live', () => {
  const total = SIM.channels().length; const r = SIM.rights(); const t = SIM.now();
  const count = total ? `${total} Sender` : '';
  let body = vaMode(vaSkelRow(5, 260, 220) + vaSkelRow(5, 260, 220) + vaSkelRow(6, 200, 150), LU.TX.loadingErr);
  if (!body) {
    if (!total) body = emptyState('Noch keine Sender', LU.TX.empty);
    else {
      const noEpg = SIM.flags().noEpg; const ordered = vaOrdered(); let h = '';
      const attention = r.recordings ? [...new Set(SIM.warnings().filter((w) => w.severity !== 'info').flatMap((w) => w.rules))].length : 0;
      if (attention) h += notice('warn', `${attention} geplante ${attention === 1 ? 'Aufnahme braucht' : 'Aufnahmen brauchen'} deine Aufmerksamkeit`, '', `<a class="btn secondary sm" href="#/live/scheduled/warnings" data-fid="ls01:warn">${icon('alert', 16)}Konflikte und Warnungen ansehen</a>`);
      if (noEpg) h += notice('', 'Für die Sender gibt es gerade keine Programmdaten.', 'Fernsehen geht trotzdem.');
      const nowP = ordered.map((c) => SIM.nowNext(c.id).now).filter(Boolean);
      const soonP = ordered.map((c) => SIM.nowNext(c.id).next).filter((p) => p && p.start - t <= 60);
      if (nowP.length) { const ids = nowP.map((p) => p.ch); h += vaRow('now', 'Jetzt im Fernsehen', nowP.map((p) => vaProgCard(p, 'now', 'now', ids, 'now')), { meta: `${nowP.length} Sender` }); }
      if (soonP.length) h += vaRow('soon', 'Gleich im Fernsehen', soonP.map((p) => vaProgCard(p, 'soon', 'soon', [], 'soon')), { meta: 'in den nächsten 60 Minuten' });
      const recent = SIM.recent().filter((x) => x.ch.active !== false && SIM.chState(x.ch) !== 'gone').slice(0, 12);
      if (recent.length) { const ids = recent.map((x) => x.ch.id); h += vaRow('recent', 'Zuletzt gesehen', recent.map((x) => vaChanCard(x.ch, 'recent', ids, 'recent')), { meta: `zuletzt ${fmt.ago(recent[0].at)}` }); }
      const favs = SIM.favorites().map(SIM.channel).filter(Boolean);
      h += favs.length ? vaRow('favs', 'Meine Sender', favs.map((c) => vaChanCard(c, 'favs', favs.map((x) => x.id), 'favorites')), { all: ['#/live/channels/favorites', 'Alle'], meta: `${favs.length} Sender` })
        : `<section class="va-row"><div class="va-rowhead"><h2>Meine Sender</h2></div>${emptyState('Noch keine Lieblingssender', 'Tippe in der Senderliste aufs Herz.', '<a class="btn secondary" href="#/live/channels" data-fid="ls01:tochan">Zu den Sendern</a>')}</section>`;
      if (!noEpg) {
        const gs = SIM.genresSoon(3).sort((a, b) => (VA_GENRE_ORDER.indexOf(a.genre) + 99) % 99 - (VA_GENRE_ORDER.indexOf(b.genre) + 99) % 99 || a.genre.localeCompare(b.genre, 'de')).slice(0, 12);
        if (gs.length) h += `<section class="va-row" aria-labelledby="vh-genre"><div class="va-rowhead"><h2 id="vh-genre">Nach Genre</h2><span class="va-rmeta">in den nächsten 3 Stunden</span></div><div class="pills va-genres" role="list">${gs.map((g) => `<a class="pill" role="listitem" href="#/live/guide/search?genre=${encodeURIComponent(g.genre)}&when=now" data-fid="ls01:g:${esc(g.genre)}">${esc(g.genre)}</a>`).join('')}</div></section>`;
      }
      const running = r.recordings ? SIM.recordings().filter((x) => x.state === 'running').length : 0;
      const links = [vaLink('#/live/guide', 'calendar', 'Programmzeitschrift', 'Alle Sender, Tag und Uhrzeit', 'ls01:l:guide')];
      if (r.recordings) links.push(vaLink('#/live/recordings', 'record', 'Meine Aufnahmen', running ? `${running} läuft` : 'Keine läuft', 'ls01:l:rec'), vaLink('#/live/scheduled', 'recordSeries', 'Geplant', `${SIM.upcoming().length} Termine`, 'ls01:l:sched'));
      links.push(vaLink('#/live/reminders', 'reminder', 'Erinnerungen', `${SIM.reminders().length}`, 'ls01:l:rem'));
      h += `<nav class="va-links" aria-label="Einstiege">${links.join('')}</nav>`;
      body = h;
    }
  }
  return { type: 'viewer', nav: 'live', title: 'Live-TV', html: livePage({ tab: 'start', title: 'Live-TV', count, html: `<div class="va-start">${body}</div>${vaDock()}` }) };
});

/* ======================================================================================
   LS-03 Meine Sender (Ordnen mit Knöpfen und Alt+↑/↓)
   ====================================================================================== */
function vaSegNav(active) {
  return `<nav class="va-segnav" aria-label="Senderlisten"><a href="#/live/channels" ${active === 'all' ? 'aria-current="page"' : ''} data-fid="seg:all">Alle Sender</a><a href="#/live/channels/favorites" ${active === 'fav' ? 'aria-current="page"' : ''} data-fid="seg:fav">Meine Sender</a></nav>`;
}
function vaOrdRow(c, i, n) {
  const first = i === 0; const last = i === n - 1;
  return `<li class="va-ord" data-k="${c.id}"><span class="va-ordmain" tabindex="0" data-ord="${c.id}" data-fid="ordrow:${c.id}" aria-label="${esc(`${c.name}, Platz ${i + 1} von ${n}. Alt plus Pfeil hoch oder runter verschiebt.`)}"><span class="va-ordn">${i + 1}</span>${LU.logo(c)}<span class="va-ordname"><b>${esc(c.name)}</b><span>Nr. ${c.no}</span></span></span>
    <span class="va-ordacts"><button type="button" class="btn ghost sm" data-act="vaMoveFav" data-ch="${c.id}" data-dir="-1" ${first ? 'aria-disabled="true"' : ''} data-fid="ord:up:${c.id}" aria-label="${esc(c.name)} nach oben">${icon('arrowUp', 16)}<span class="vt">Nach oben</span></button><button type="button" class="btn ghost sm" data-act="vaMoveFav" data-ch="${c.id}" data-dir="1" ${last ? 'aria-disabled="true"' : ''} data-fid="ord:down:${c.id}" aria-label="${esc(c.name)} nach unten">${icon('arrowDown', 16)}<span class="vt">Nach unten</span></button><button type="button" class="btn ghost sm danger" data-act="vaRemoveFav" data-ch="${c.id}" data-fid="ord:rm:${c.id}" aria-label="${esc(c.name)} aus Meine Sender entfernen">${icon('x', 16)}<span class="vt">Entfernen</span></button></span></li>`;
}
route('/live/channels/favorites', () => {
  const favs = SIM.favorites().map(SIM.channel).filter(Boolean); const total = SIM.channels().length; const ord = VA.ord && favs.length > 0;
  const act = favs.length ? `<button type="button" class="btn ${ord ? 'primary' : 'secondary'}" data-act="vaOrd" aria-pressed="${ord}" data-fid="ls03:ord">${icon(ord ? 'check' : 'list', 18)}${ord ? 'Fertig' : 'Ordnen'}</button>` : '';
  let body = vaMode(skelRows(5, 72), 'Meine Sender konnten nicht geladen werden.');
  if (!body) {
    if (!total) body = emptyState('Keine Sender', LU.TX.empty);
    else if (!favs.length) body = emptyState('Noch keine Lieblingssender', 'Tippe in der Senderliste aufs Herz.', '<a class="btn secondary" href="#/live/channels" data-fid="ls03:tochan">Zu den Sendern</a>');
    else {
      const ids = favs.map((c) => c.id); if (!ids.includes(ui.ls02row)) ui.ls02row = ids[0];
      const noEpg = SIM.flags().noEpg ? notice('', 'Für die Sender gibt es gerade keine Programmdaten.', 'Fernsehen geht trotzdem.') : '';
      body = noEpg + (ord
        ? `<p class="va-hint">Mit den Knöpfen oder Alt+↑ und Alt+↓ verschiebst du einen Sender. Jeder Schritt gilt sofort. Das ist auch die Reihenfolge auf der Startseite von Live-TV und beim Zappen.</p><ul class="va-ordlist" aria-label="Meine Sender ordnen">${favs.map((c, i) => vaOrdRow(c, i, favs.length)).join('')}</ul><p class="vh" role="status" aria-live="polite">${esc(VA.ordMsg)}</p>`
        : `<div class="chhead" aria-hidden="true"><span>Nr.</span><span></span><span>Sender</span><span>Jetzt</span><span>Danach</span></div><ul class="chlist" data-roving="row" aria-label="Meine Sender, Pfeiltasten wechseln, Enter schaltet ein">${favs.map((c) => ls02Row(c, ids, c.id === ui.ls02row)).join('')}</ul>`);
    }
  }
  return { type: 'viewer', nav: 'live', title: 'Meine Sender', html: livePage({ tab: 'channels', title: 'Meine Sender', count: favs.length && !/loading|error/.test(liveMode()) ? `${favs.length} Sender` : '', actions: act, html: vaSegNav('fav') + `<div class="ls02-body">${body}</div>${vaDock()}` }) };
});
ACT.vaOrd = () => { VA.ord = !VA.ord; VA.ordMsg = ''; render(); };
ACT.vaMoveFav = (d, el) => {
  if (el && el.getAttribute('aria-disabled') === 'true') return;
  const c = SIM.channel(d.ch); if (!SIM.moveFav(d.ch, Number(d.dir))) return;
  VA.ordMsg = `${c.name}, Platz ${SIM.favorites().indexOf(d.ch) + 1} von ${SIM.favorites().length}`;
};
ACT.vaRemoveFav = (d) => {
  const c = SIM.channel(d.ch); const k = SIM.favorites().indexOf(d.ch); SIM.toggleFav(d.ch);
  VA.ordMsg = `${c.name} ist nicht mehr in Meine Sender`;
  toastUndo(`<b>${esc(c.name)}</b> ist nicht mehr in Meine Sender`, 'fav', { ch: d.ch });
  const next = SIM.favorites()[Math.min(k, SIM.favorites().length - 1)];
  setTimeout(() => { const e = next && document.querySelector(`[data-fid="ordrow:${next}"]`); if (e) e.focus(); else { const b = document.querySelector('[data-fid="ls03:ord"], [data-fid="ls03:tochan"]'); if (b) b.focus(); } }, 0);
};

/* ======================================================================================
   LS-05 Programmsuche
   ====================================================================================== */
const VA_WHEN = [['now', 'Jetzt'], ['today', 'Heute'], ['tomorrow', 'Morgen'], ['week', 'Nächste 7 Tage']];
const vaSearchF = (q) => ({ q: q.q || '', when: VA_WHEN.some((w) => w[0] === q.when) ? q.when : 'week', genre: q.genre || '', group: q.group || '' });
function vaSearchHref(f, patch) {
  const v = Object.assign({}, f, patch || {}); const qs = [];
  if (v.q) qs.push('q=' + encodeURIComponent(v.q)); if (v.when && v.when !== 'week') qs.push('when=' + v.when);
  if (v.genre) qs.push('genre=' + encodeURIComponent(v.genre)); if (v.group) qs.push('group=' + encodeURIComponent(v.group));
  return '#/live/guide/search' + (qs.length ? '?' + qs.join('&') : '');
}
route('/live/guide/search', (pm, query) => {
  const f = vaSearchF(query); const total = SIM.channels().length;
  const genres = [...new Set(LIVE.programs.map((p) => p.genre))].filter((g) => g !== 'Sonstiges').sort((a, b) => a.localeCompare(b, 'de'));
  const groupOpts = [['fav', 'Meine Sender'], ...LIVE.groups.map((g) => [g.id, g.name])];
  const sel = (k, lbl, all, opts) => `<label class="fld fsel"><span class="lbl">${lbl}</span><select class="sel" data-act-change="vaSearchSel" data-key="${k}" data-fid="ls05:${k}" aria-label="${lbl}"><option value="">${all}</option>${opts.map(([v, t]) => `<option value="${esc(v)}"${f[k] === v ? ' selected' : ''}>${esc(t)}</option>`).join('')}</select></label>`;
  const tools = `<div class="va-tools" role="search"><label class="fld fsearch"><span class="vh">Sendungen suchen</span><span class="sic">${icon('search', 18)}</span><input class="inp" type="search" placeholder="Sendungen suchen" value="${esc(f.q)}" data-vaq="guide" data-fid="ls05:q" aria-label="Sendungen suchen (Titel, Episodentitel, Beschreibung)"></label>
    <div class="pills va-when" role="group" aria-label="Wann">${VA_WHEN.map(([k, l]) => `<a class="pill" href="${vaSearchHref(f, { when: k })}" aria-current="${f.when === k}" data-fid="ls05:w:${k}">${l}</a>`).join('')}</div>
    <div class="va-facets">${sel('genre', 'Genre', 'Alle Genres', genres.map((g) => [g, g]))}${sel('group', 'Gruppe', 'Alle Sender', groupOpts)}</div>
    <button type="button" class="btn secondary va-fbtn" data-act="vaSheet" data-fid="ls05:sheet">${icon('filter', 18)}Filter${(f.genre ? 1 : 0) + (f.group ? 1 : 0) ? ` (${(f.genre ? 1 : 0) + (f.group ? 1 : 0)})` : ''}</button></div>`;
  const active = (f.q || f.genre || f.group) ? `<div class="afilters" aria-label="Aktive Filter">${f.genre ? `<a class="chip outline" href="${vaSearchHref(f, { genre: '' })}" data-fid="ls05:af:g">Genre: ${esc(f.genre)} ${icon('x', 12)}<span class="vh">Filter entfernen</span></a>` : ''}${f.group ? `<a class="chip outline" href="${vaSearchHref(f, { group: '' })}" data-fid="ls05:af:gr">Gruppe: ${esc(f.group === 'fav' ? 'Meine Sender' : (LIVE.groups.find((g) => g.id === f.group) || {}).name || f.group)} ${icon('x', 12)}<span class="vh">Filter entfernen</span></a>` : ''}<a class="lnk" href="#/live/guide/search" data-fid="ls05:af:all">Alle Filter entfernen</a></div>` : '';
  let body = vaMode(skelRows(7, 64), 'Die Suche konnte nicht ausgeführt werden.');
  if (!body) {
    const partial = SIM.channels().filter((c) => SIM.epgOf(c) !== 'ok');
    const allNone = total > 0 && partial.length === total;
    const names = partial.map((c) => c.name + (SIM.epgOf(c) === 'partial' ? ' (teilweise)' : ''));
    const note = !total ? '' : allNone ? notice('', 'Für alle Sender gibt es gerade keine Programmdaten.', 'Die Suche findet deshalb nichts. Fernsehen geht trotzdem.')
      : partial.length ? notice('', `Für ${partial.length} Sender gibt es keine Programmdaten: ${esc(names.join(', '))}.`, '') : '';
    if (!total) body = emptyState('Keine Sender', LU.TX.empty);
    else if (!f.q && !f.genre) body = note + emptyState('Gib einen Titel ein oder wähle ein Genre.', '', `<div class="pills">${['Nachrichten', 'Sport', 'Spielfilm', 'Krimi', 'Dokumentation', 'Kinder'].map((g) => `<a class="pill" href="${vaSearchHref(f, { genre: g })}" data-fid="ls05:s:${g}">${g}</a>`).join('')}</div>`);
    else {
      const res = SIM.search(f.q, { when: f.when, genre: f.genre, group: f.group });
      if (!res.length) body = note + emptyState('Nichts gefunden', `Keine Sendung gefunden. Programmdaten reichen bis ${fmt.day(LIVE.EPG_END - 1)}`, `<a class="btn secondary" href="#/live/guide/search" data-fid="ls05:clear">Filter entfernen</a>`);
      else {
        const shown = res.slice(0, VA.more); let lastDay = null; const lis = [];
        shown.forEach((p) => {
          const d = Math.floor(Math.max(p.start, SIM.now()) / 1440);
          if (d !== lastDay) { lastDay = d; lis.push(`<li class="va-dayh" role="presentation"><h2>${fmt.dayRel(Math.max(p.start, SIM.now()))}</h2></li>`); }
          lis.push(vaProgRow(p));
        });
        body = note + `<p class="va-count" role="status">${res.length} ${res.length === 1 ? 'Sendung' : 'Sendungen'}${res.length > shown.length ? `, ${shown.length} angezeigt` : ''}</p>` + vaList('ls05', lis, 'Treffer, Pfeiltasten wechseln, nach rechts zu den Aktionen') + (res.length > shown.length ? `<div class="row-gap" style="justify-content:center"><button type="button" class="btn secondary" data-act="vaMore" data-fid="ls05:more">Mehr anzeigen</button></div>` : '');
      }
    }
  }
  return { type: 'viewer', nav: 'live', title: 'Programmsuche', html: livePage({ tab: 'guide', title: 'Programmsuche', sub: '<a class="lnk" href="#/live/guide" data-fid="ls05:toguide">Zur Programmzeitschrift</a>', html: tools + active + `<div class="va-pgbody">${body}</div>${vaDock()}` }) };
});
ACT.vaMore = () => { VA.more += 50; render(); };
ACT.vaSheet = () => {
  const f = vaSearchF(parseHash().query);
  const genres = [...new Set(LIVE.programs.map((p) => p.genre))].filter((g) => g !== 'Sonstiges').sort((a, b) => a.localeCompare(b, 'de'));
  LiveDlg.filterSheet({ fields: [{ key: 'genre', label: 'Genre', options: [{ v: '', t: 'Alle Genres' }, ...genres.map((g) => ({ v: g, t: g }))] }, { key: 'group', label: 'Gruppe', options: [{ v: '', t: 'Alle Sender' }, { v: 'fav', t: 'Meine Sender' }, ...LIVE.groups.map((g) => ({ v: g.id, t: g.name }))] }], values: { genre: f.genre, group: f.group }, onApply: (v) => { VA.more = 50; go(vaSearchHref(f, { genre: v.genre || '', group: v.group || '' })); } });
};

/* ---------- global listeners of this part ---------- */
document.addEventListener('input', (e) => {
  const el = e.target.closest('[data-vaq]'); if (!el) return;
  VA.more = 50;
  if (el.dataset.vaq === 'guide') { const f = vaSearchF(parseHash().query); history.replaceState(null, '', vaSearchHref(f, { q: el.value })); render(); }
  else if (el.dataset.vaq === 'search') { history.replaceState(null, '', '#/search' + (el.value ? '?q=' + encodeURIComponent(el.value) : '')); render(); }
});
document.addEventListener('change', (e) => {
  const el = e.target.closest('[data-act-change="vaSearchSel"]'); if (!el) return;
  const f = vaSearchF(parseHash().query); VA.more = 50; go(vaSearchHref(f, { [el.dataset.key]: el.value }));
});

/* rows (cards): ←/→ cards, Home/End, Shift+F10 or menu key opens the card menu */
document.addEventListener('focusin', (e) => {
  const m = e.target.closest && e.target.closest('.va-main'); if (!m) return;
  const ul = m.closest('[data-vrow]'); if (!ul) return;
  VA.rowAct[ul.dataset.vrow] = m.closest('li').dataset.k;
  $$('.va-main', ul).forEach((x) => x.setAttribute('tabindex', x === m ? '0' : '-1'));
});
document.addEventListener('keydown', (e) => {
  if (ui.menu || ui.dialog) return;
  const ul = e.target.closest && e.target.closest('[data-vrow]'); if (!ul || e.target.closest('.lmenu')) return;
  const li = e.target.closest('li'); const items = $$(':scope > li', ul); const k = items.indexOf(li);
  const to = (n) => { n = Math.max(0, Math.min(items.length - 1, n)); const m = $('.va-main', items[n]); m.focus(); m.scrollIntoView({ block: 'nearest', inline: 'nearest' }); };
  if (e.key === 'ArrowRight') { e.preventDefault(); to(k + 1); } else if (e.key === 'ArrowLeft') { e.preventDefault(); to(k - 1); }
  else if (e.key === 'Home') { e.preventDefault(); to(0); } else if (e.key === 'End') { e.preventDefault(); to(items.length - 1); }
  else if (e.key === 'ContextMenu' || (e.key === 'F10' && e.shiftKey)) { const b = $('[data-act="menu"]', li); if (b) { e.preventDefault(); ACT.menu({ id: b.dataset.id }, b); } }
});
/* lists (program rows): ↑/↓ rows, → actions, ← back, Home/End/PageUp/PageDown */
document.addEventListener('focusin', (e) => {
  const li = e.target.closest && e.target.closest('.va-prow'); if (!li) return;
  const ul = li.closest('[data-vlist]'); if (!ul) return;
  VA.listAct[ul.dataset.vlist] = li.dataset.k;
  $$('.va-pmain', ul).forEach((x) => x.setAttribute('tabindex', x.closest('li') === li ? '0' : '-1'));
});
document.addEventListener('keydown', (e) => {
  if (ui.menu || ui.dialog) return;
  const ul = e.target.closest && e.target.closest('[data-vlist]'); if (!ul || e.target.closest('.lmenu')) return;
  const li = e.target.closest('li.va-prow'); if (!li) return;
  const items = $$(':scope > li.va-prow', ul); const k = items.indexOf(li);
  const parts = $$('.va-pmain, .va-act', li); const pi = parts.indexOf(e.target);
  const to = (n) => { n = Math.max(0, Math.min(items.length - 1, n)); const m = $('.va-pmain', items[n]); m.focus(); m.scrollIntoView({ block: 'nearest' }); };
  if (e.key === 'ArrowDown') { e.preventDefault(); to(k + 1); } else if (e.key === 'ArrowUp') { e.preventDefault(); to(k - 1); }
  else if (e.key === 'Home') { e.preventDefault(); to(0); } else if (e.key === 'End') { e.preventDefault(); to(items.length - 1); }
  else if (e.key === 'PageDown') { e.preventDefault(); to(k + 8); } else if (e.key === 'PageUp') { e.preventDefault(); to(k - 8); }
  else if (e.key === 'ArrowRight') { e.preventDefault(); if (parts[pi + 1]) parts[pi + 1].focus(); } else if (e.key === 'ArrowLeft') { e.preventDefault(); if (pi > 0) parts[pi - 1].focus(); }
});
/* ordering mode of LS-03: Alt+↑/↓ moves the focused row and keeps focus on it; ↑/↓ alone move between rows */
document.addEventListener('keydown', (e) => {
  const row = e.target.closest && e.target.closest('[data-ord]'); if (!row || ui.dialog) return;
  const id = row.dataset.ord;
  if (e.altKey && (e.key === 'ArrowUp' || e.key === 'ArrowDown')) {
    e.preventDefault(); ACT.vaMoveFav({ ch: id, dir: e.key === 'ArrowUp' ? -1 : 1 });
    setTimeout(() => { const r = document.querySelector(`[data-fid="ordrow:${id}"]`); if (r) r.focus(); }, 0);
  } else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
    const rows = $$('[data-ord]'); const n = rows.indexOf(row) + (e.key === 'ArrowDown' ? 1 : -1);
    if (rows[n]) { e.preventDefault(); rows[n].focus(); }
  }
});
