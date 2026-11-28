/* LS-02 Senderübersicht in the TV app (sample page of the core): big channel rows (one row = one focus),
   heart with →, preview of the focused channel, search with the TV keyboard, filters as a side menu from
   the right, channel up/down = one page (7 rows), digits jump to a channel number, Play/Pause or OK switches
   on. Every state: empty, filtered empty, loading, error, without programme data, disturbed.
   14-oberflaeche.md 3.2 (TV) and 4.13. */
'use strict';

const LTV02 = { f: { q: '', group: '', country: '', lang: '' } };
RESETS.push(() => { LTV02.f = { q: '', group: '', country: '', lang: '' }; });
const LTV02_FIELDS = [
  ['group', 'Gruppe', 'Alle Sender', () => [['fav', 'Meine Sender'], ...LIVE.groups.map((g) => [g.id, g.name])]],
  ['country', 'Land', 'Alle Länder', () => Object.entries(LIVE.countries)],
  ['lang', 'Sprache', 'Alle Sprachen', () => Object.entries(LIVE.languages)],
];
const ltv02Val = (k) => { const fd = LTV02_FIELDS.find((x) => x[0] === k); const v = LTV02.f[k]; return v ? (fd[3]().find((o) => o[0] === v) || [v, v])[1] : fd[2]; };

function ltv02NowLine(c) {
  const nn = SIM.nowNext(c.id);
  if (SIM.chState(c) === 'disturbed') return `<span class="ltv-now warn">${icon('alert', 26)}${esc(LU.TX.unavailable)}</span>`;
  if (!nn.now) return `<span class="ltv-now none">${esc(SIM.epgOf(c) === 'none' ? LU.TX.epgNoneButPlays : LU.TX.epgNone)}</span>`;
  return `<span class="ltv-now"><b>${esc(nn.now.title)}</b><small>${fmt.range(nn.now.start, nn.now.end)}</small>${LU.progress(nn.now.start, nn.now.end)}</span>`;
}
function ltv02Row(c, ids, k) {
  const fav = SIM.isFav(c.id);
  const rec = SIM.rights().recordings && SIM.runningOn(c.id);
  return `<div class="ltv-row${SIM.chState(c) !== 'ok' ? ' bad' : ''}" data-k="${k}">
    <a class="ltv-ch" href="#/live/watch/${c.id}" data-act="ltvTune" data-ch="${c.id}" data-f="ch-${c.id}"${k === 0 ? ' data-default' : ''} aria-label="${c.no} ${esc(c.name)}">
      <span class="ltv-no">${c.no}</span>${LU.logo(c)}<span class="ltv-name">${esc(c.name)}${rec ? `<span class="ltv-rec">${LU.badge('recording', { text: true })}</span>` : ''}</span>${ltv02NowLine(c)}</a>
    <button type="button" class="ltv-heart${fav ? ' on' : ''}" data-act="ltvFav" data-ch="${c.id}" data-f="fav-${c.id}" aria-pressed="${fav}" aria-label="${esc(c.name)} ${fav ? 'aus Meine Sender entfernen' : 'zu Meine Sender hinzufügen'}">${icon(fav ? 'heartOn' : 'heart', 40)}</button></div>`;
}
function ltv02Preview(id) {
  const c = SIM.channel(id); if (!c) return '';
  const nn = SIM.nowNext(c.id); const st = SIM.chState(c);
  const n = nn.now;
  return `<div class="ltv-pv-top">${LU.logo(c, { cls: 'big' })}<span><b>${c.no} ${esc(c.name)}</b><small>${esc(LIVE.countries[c.country])} · ${c.langs.map((l) => LIVE.languages[l]).join(', ')}${SIM.isFav(c.id) ? ' · in Meine Sender' : ''}</small></span></div>
    ${st === 'disturbed' ? `<div class="tnote warn">${icon('alert', 36)}<div><b>${esc(c.name)} ist gerade nicht erreichbar.</b><p>Slouchcat versucht es weiter. Einschalten geht trotzdem.</p></div></div>` : ''}
    ${n ? `<p class="ltv-pv-lbl">Jetzt</p><h2 class="ltv-pv-t">${esc(n.title)}${n.sub ? ` <span>«${esc(n.sub)}»</span>` : ''}</h2><p class="ltv-pv-m">${fmt.range(n.start, n.end)} · ${esc(n.genre)} · ${LU.nowText('left', n.end)}</p>${LU.progress(n.start, n.end)}${n.desc ? `<p class="ltv-pv-d">${esc(n.desc)}</p>` : ''}<p class="ltv-pv-b">${LU.programBadges(n, { text: true })}</p>`
      : `<p class="ltv-pv-lbl">Jetzt</p><p class="ltv-pv-t none">${esc(SIM.epgOf(c) === 'none' ? 'Für diesen Sender gibt es keine Programmdaten.' : LU.TX.epgNone)}</p>`}
    ${nn.next ? `<p class="ltv-pv-lbl">Danach</p><p class="ltv-pv-n"><b>${esc(nn.next.title)}</b> ab ${fmt.time(nn.next.start)}</p>` : ''}
    <p class="ltv-pv-keys">OK einschalten · → Herz · Kanal +/− eine Seite · Ziffern: Sender mit Nummer · Guide: Programm</p>`;
}

route('/live/channels', () => {
  const f = LTV02.f; const total = SIM.channels().length; const list = SIM.channels(f); const ids = list.map((c) => c.id);
  const nActive = LTV02_FIELDS.filter(([k]) => f[k]).length;
  const tools = `<div class="ltv-tools">
    <button type="button" class="tf ltv-search" data-act="ltvSearch" data-f="ltv-q">${icon('search', 36)}${f.q ? `<span class="tf-v">${esc(f.q)}</span>` : '<span class="tf-ph">Sender suchen</span>'}</button>
    <button type="button" class="tb ghost" data-act="ltvFilter" data-f="ltv-filter">${icon('filter', 34)}<span>Filter${nActive ? ` (${nActive})` : ''}: ${esc(ltv02Val('group'))}</span></button>
    ${nActive || f.q ? `<button type="button" class="tb ghost sm" data-act="ltvClear" data-f="ltv-clear">${icon('close', 28)}<span>Filter entfernen</span></button>` : ''}</div>`;
  const mode = SIM.mode();
  let body;
  if (mode === 'loading') body = `<div class="ltv-list">${Array.from({ length: 6 }, () => '<span class="lv-skel ltv-skel"></span>').join('')}</div>`;
  else if (mode === 'error') body = `<div class="tnote bad">${icon('alert', 36)}<div><b>Die Sender konnten nicht geladen werden.</b><p>Der Server hat nicht geantwortet.</p></div></div><div class="ltv-acts"><button type="button" class="tb primary" data-act="ltvRetry" data-f="ltv-retry" data-default>${icon('refresh', 32)}<span>Erneut versuchen</span></button></div>`;
  else if (!total) body = `<div class="tnote">${icon('info', 36)}<div><b>Noch keine Sender</b><p>${esc(LU.TX.empty)}</p></div></div>`;
  else if (!list.length) body = `<div class="tnote">${icon('info', 36)}<div><b>Kein Sender passt zu den Filtern</b><p>Ändere die Suche oder entferne Filter.</p></div></div><div class="ltv-acts"><button type="button" class="tb primary" data-act="ltvClear" data-f="ltv-clear2" data-default>${icon('close', 32)}<span>Filter entfernen</span></button></div>`;
  else body = `${SIM.flags().noEpg ? `<div class="tnote">${icon('info', 36)}<div><b>Für die Sender gibt es gerade keine Programmdaten.</b><p>Fernsehen geht trotzdem.</p></div></div>` : ''}
    <div class="ltv-split"><div class="ltv-list" aria-label="Sender">${list.map((c, k) => ltv02Row(c, ids, k)).join('')}</div><aside class="ltv-prev" id="ltv-prev" aria-live="polite">${ltv02Preview(ids[0])}</aside></div>`;
  const count = !total ? '' : list.length === total ? `${total} Sender` : `${list.length} von ${total} Sendern`;
  return {
    nav: '', html: livePage({ tab: 'channels', title: 'Sender', count, html: tools + body }),
    onFocus(el) { const ch = el.dataset.ch || (el.closest('.ltv-row') && $('.ltv-ch', el.closest('.ltv-row')).dataset.ch); if (ch) { const p = $('#ltv-prev'); if (p && p.dataset.ch !== ch) { p.dataset.ch = ch; p.innerHTML = ltv02Preview(ch); } } },
    key(name) {
      const rows = $$('.ltv-row', $('#view'));
      const row = F.el && F.el.closest('.ltv-row'); const k = row ? rows.indexOf(row) : -1;
      const toRow = (n) => { n = Math.max(0, Math.min(rows.length - 1, n)); const el = $('.ltv-ch', rows[n]); if (el) focus(el); };
      if (/^d\d$/.test(name)) { liveNumber(name, (c) => { const el = byF('ch-' + c.id); if (el) focus(el); else toast(`${c.name} passt nicht zu den Filtern`); }); return true; }
      if (name === 'ok' && liveNumberActive()) { liveNumberCommit(); return true; }
      if (!rows.length) return false;
      if (name === 'ch_down') { toRow(k < 0 ? 0 : k + 7); return true; }
      if (name === 'ch_up') { toRow(k < 0 ? 0 : k - 7); return true; }
      if (k < 0) return false;
      if (name === 'down') { if (k < rows.length - 1) toRow(k + 1); else bump(F.el, 'down'); return true; }
      if (name === 'up' && k > 0) { toRow(k - 1); return true; }
      if (name === 'left' && F.el.classList.contains('ltv-heart')) { focus($('.ltv-ch', row)); return true; }
      if (name === 'info') { const ch = $('.ltv-ch', row).dataset.ch; const n = SIM.nowNext(ch).now; if (n) go('#/live/programs/' + n.id); else toast('Keine Programmdaten'); return true; }
      return false;
    },
  };
});

ACT.ltvTune = (d) => liveTune(d.ch, SIM.channels(LTV02.f).map((c) => c.id));
ACT.ltvFav = (d) => { const on = SIM.toggleFav(d.ch); toast(`${SIM.channel(d.ch).name} ${on ? 'ist jetzt in Meine Sender' : 'ist nicht mehr in Meine Sender'}`); };
ACT.ltvRetry = () => { SIM.setMode('normal'); toast('Erneut geladen'); };
ACT.ltvClear = () => { LTV02.f = { q: '', group: '', country: '', lang: '' }; render(); toast('Filter entfernt'); };
ACT.ltvSearch = () => keyboard({ value: LTV02.f.q, label: 'Sender suchen (Name oder Nummer)', placeholder: 'zum Beispiel SRF oder 11', onDone: (v) => { LTV02.f.q = v.trim(); render(); } });
function ltv02FilterHtml() {
  return `<h2>Filter</h2><div class="mlist">${LTV02_FIELDS.map(([k, l], n) => `<button type="button" class="mi" data-act="ltvFilterField" data-k="${k}" data-f="flt-${k}"${n === 0 ? ' data-default' : ''}><span class="mi-ic">${icon(k === 'group' ? 'list' : k === 'country' ? 'globe' : 'audio', 36)}</span><span class="mi-tx"><b>${l}</b><small>${esc(ltv02Val(k))}</small></span>${icon('right', 32)}</button>`).join('')}</div>
    <div class="acts"><button type="button" class="tb ghost" data-act="ltvFilterReset" data-f="flt-reset">${icon('refresh', 30)}<span>Zurücksetzen</span></button><button type="button" class="tb primary" data-act="close" data-f="flt-done">${icon('check', 30)}<span>Fertig</span></button></div>`;
}
ACT.ltvFilter = () => menu(ltv02FilterHtml(), { onClose: () => render() });
ACT.ltvFilterReset = () => { LTV02.f.group = ''; LTV02.f.country = ''; LTV02.f.lang = ''; redrawMenu(ltv02FilterHtml()); };
ACT.ltvFilterField = (d) => {
  const fd = LTV02_FIELDS.find((x) => x[0] === d.k);
  const opts = [['', fd[2]], ...fd[3]()];
  menu(`<h2>${fd[1]}</h2><div class="mlist">${opts.map(([v, t]) => `<button type="button" class="mi" role="menuitemradio" aria-checked="${(LTV02.f[d.k] || '') === v}" data-act="ltvFilterPick" data-k="${d.k}" data-v="${esc(v)}" data-f="opt-${d.k}-${esc(v) || 'all'}"${(LTV02.f[d.k] || '') === v ? ' data-default' : ''}><span class="mi-tx"><b>${esc(t)}</b><small>${esc(t === fd[2] ? 'kein Filter' : SIM.channels(Object.assign({}, LTV02.f, { [d.k]: v })).length + ' Sender')}</small></span>${(LTV02.f[d.k] || '') === v ? icon('check', 34) : ''}</button>`).join('')}</div>`);
};
ACT.ltvFilterPick = (d) => { LTV02.f[d.k] = d.v; closeMenu(); redrawMenu(ltv02FilterHtml()); };
