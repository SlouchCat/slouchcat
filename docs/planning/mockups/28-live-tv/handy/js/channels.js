/* LS-02 Senderübersicht in the phone and tablet app (sample page of the core): menu button (media drawer),
   chip row of the area, search, «Filter (n)» as a sheet (LD-09), two-line rows with heart and menu ⋯ (long
   press opens the same sheet), tablet with «Danach». Every state: empty, filtered empty, loading, error,
   without programme data, disturbed. 14-oberflaeche.md 3.2 (Handy-App: Muster RadioScreen, ListRow). */
'use strict';

const LH02 = { f: { q: '', group: '', country: '', lang: '', category: '' } };
const LH02_KEYS = ['group', 'country', 'lang', 'category'];

function lh02Row(c) {
  const nn = SIM.nowNext(c.id); const st = SIM.chState(c); const fav = SIM.isFav(c.id);
  const rec = SIM.rights().recordings && SIM.runningOn(c.id);
  const now = st === 'disturbed' ? `<span class="lh-now warn">${icon('alert', 16)}${esc(LU.TX.unavailable)}</span>`
    : nn.now ? `<span class="lh-now"><b>${esc(nn.now.title)}</b><small>${fmt.range(nn.now.start, nn.now.end)} · ${LU.nowText('left', nn.now.end)}</small>${LU.progress(nn.now.start, nn.now.end)}</span>`
      : `<span class="lh-now none">${esc(SIM.epgOf(c) === 'none' ? LU.TX.epgNoneButPlays : LU.TX.epgNone)}</span>`;
  const next = nn.next ? `<span class="lh-next"><small>Danach</small><b>${esc(nn.next.title)}</b><small>ab ${fmt.time(nn.next.start)}</small></span>` : '<span class="lh-next"></span>';
  return `<li class="lh-row${st !== 'ok' ? ' bad' : ''}">
    <a class="lh-main" href="#/live/watch/${c.id}" data-act="lhTune" data-ch="${c.id}" data-lp="${c.id}" data-fid="row-${c.id}" aria-label="${c.no} ${esc(c.name)} einschalten">
      <span class="lh-top"><span class="lh-no">${c.no}</span>${LU.logo(c)}<span class="lh-name">${esc(c.name)}</span>${rec ? LU.badge('recording', { text: true }) : nn.now ? LU.programBadges(nn.now) : ''}</span>${now}${next}</a>
    <button type="button" class="ib lh-heart${fav ? ' on' : ''}" data-act="lhFav" data-ch="${c.id}" aria-pressed="${fav}" aria-label="${esc(c.name)} ${fav ? 'aus Meine Sender entfernen' : 'zu Meine Sender hinzufügen'}" data-fid="fav-${c.id}">${icon(fav ? 'heartOn' : 'heart', 22)}</button>
    <button type="button" class="ib lh-more" data-act="lhMenu" data-ch="${c.id}" aria-label="Weitere Aktionen für ${esc(c.name)}" data-fid="more-${c.id}">${icon('dots', 22)}</button></li>`;
}

route('/live/channels', () => {
  const f = LH02.f; const total = SIM.channels().length; const list = SIM.channels(f);
  const n = LH02_KEYS.filter((k) => f[k]).length;
  const top = `<div class="lh-tools"><label class="lh-search"><span class="vh">Sender suchen</span>${icon('search', 20)}<input class="inp" type="search" placeholder="Sender suchen" value="${esc(f.q)}" data-lh02q data-fid="lh-q" aria-label="Sender suchen (Name oder Nummer)"></label>
    <button type="button" class="btn ghost sm lh-filter${n ? ' on' : ''}" data-act="lhFilter" data-fid="lh-filter">${icon('filter', 18)}Filter${n ? ` (${n})` : ''}</button></div>
    ${n ? `<div class="chips lh-active">${LH02_KEYS.filter((k) => f[k]).map((k) => `<button type="button" class="chip on" data-act="lhUnset" data-k="${k}" data-fid="af-${k}" aria-label="Filter entfernen">${esc(lhLabel(k, f[k]))} ${icon('close', 14)}</button>`).join('')}<button type="button" class="chip" data-act="lhClear" data-fid="af-all">Alle entfernen</button></div>` : ''}`;
  const mode = SIM.mode();
  let body;
  if (mode === 'loading') body = `<div class="pad lh-skel">${Array.from({ length: 7 }, () => '<span class="lv-skel" style="height:72px"></span>').join('')}</div>`;
  else if (mode === 'error') body = `<div class="note warn">${icon('alert', 20)}<div><b>Die Sender konnten nicht geladen werden.</b><br>Der Server hat nicht geantwortet.</div></div><p class="pad"><button type="button" class="btn secondary" data-act="lhRetry" data-fid="lh-retry">${icon('refresh', 18)}Erneut versuchen</button></p>`;
  else if (!total) body = `<div class="empty">${icon('live', 40)}<b>Noch keine Sender</b><p>${esc(LU.TX.empty)}</p></div>`;
  else if (!list.length) body = `<div class="empty">${icon('filter', 40)}<b>Kein Sender passt zu den Filtern</b><p>Ändere die Suche oder entferne Filter.</p><p style="margin-top:16px"><button type="button" class="btn ghost" data-act="lhClear" data-fid="lh-clear">Filter entfernen</button></p></div>`;
  else body = `${SIM.flags().noEpg ? `<div class="note">${icon('info', 20)}<div>Für die Sender gibt es gerade keine Programmdaten. Fernsehen geht trotzdem.</div></div>` : ''}
    ${f.group ? `<div class="sec">${f.group === 'fav' ? 'Meine Sender' : 'Gruppe ' + esc(lhLabel('group', f.group))} · ${list.length} Sender</div>` : ''}<ul class="lh-list">${list.map(lh02Row).join('')}</ul>`;
  const count = !total ? '' : list.length === total ? `${total} Sender` : `${list.length} von ${total}`;
  return { nav: '', html: livePage({ tab: 'channels', title: 'Sender', end: count ? `<span class="lh-count">${count}</span>` : '', html: top + body }) };
});
function lhLabel(k, v) { const fd = chFilterSheet({}).fields.find((x) => x.key === k); const o = fd && fd.options.find((x) => x.v === v); return o ? o.t : v; }

document.addEventListener('input', (e) => { const el = e.target.closest('[data-lh02q]'); if (!el) return; LH02.f.q = el.value; render(); const n = $('[data-lh02q]'); if (n) { n.focus(); n.setSelectionRange(n.value.length, n.value.length); } });
ACT.lhTune = (d) => liveTune(d.ch, SIM.channels(LH02.f).map((c) => c.id));
ACT.lhFav = (d) => { const on = SIM.toggleFav(d.ch); toast(`${SIM.channel(d.ch).name} ${on ? 'ist jetzt in Meine Sender' : 'ist nicht mehr in Meine Sender'}`); };
ACT.lhRetry = () => { SIM.setMode('normal'); toast('Erneut geladen'); };
ACT.lhClear = () => { LH02.f = { q: '', group: '', country: '', lang: '', category: '' }; render(); };
ACT.lhUnset = (d) => { LH02.f[d.k] = ''; render(); };
ACT.lhFilter = () => LiveDlg.filterSheet(chFilterSheet(LH02.f, (v) => { Object.assign(LH02.f, v); render(); }));
ACT.lhMenu = (d) => {
  const c = SIM.channel(d.ch); const nn = SIM.nowNext(c.id); const fav = SIM.isFav(c.id);
  sheet(`<h2>${c.no} ${esc(c.name)}</h2>${nn.now ? `<p class="pad hint">Jetzt: ${esc(nn.now.title)} · ${fmt.range(nn.now.start, nn.now.end)}</p>` : ''}
    ${li('play', 'Einschalten', '', '', `data-act="lhTuneClose" data-ch="${c.id}" data-fid="m-tune"`)}
    ${li('calendar', 'Senderprogramm', 'Tagesprogramm als Liste', null, `href="#/live/guide/channel/${c.id}" data-fid="m-prog"`)}
    ${nn.now ? li('info', 'Jetzige Sendung: Details', nn.now.title, null, `href="#/live/programs/${nn.now.id}" data-fid="m-det"`) : ''}
    ${li(fav ? 'close' : 'heart', fav ? 'Aus Meine Sender' : 'Zu Meine Sender', '', '', `data-act="lhFavClose" data-ch="${c.id}" data-fid="m-fav"`)}
    <div class="acts"><button type="button" class="btn ghost" data-act="close" data-fid="m-x">Schliessen</button></div>`);
};
ACT.lhTuneClose = (d) => { closeSheet(true); ACT.lhTune(d); };
ACT.lhFavClose = (d) => { closeSheet(true); ACT.lhFav(d); };
/* long press on a row opens the same sheet (touch); a short tap switches on */
(function longPress() {
  let t = null; let fired = false;
  document.addEventListener('pointerdown', (e) => { const el = e.target.closest('[data-lp]'); if (!el) return; fired = false; t = setTimeout(() => { fired = true; ACT.lhMenu({ ch: el.dataset.lp }); }, 550); });
  ['pointerup', 'pointercancel', 'pointerleave'].forEach((n) => document.addEventListener(n, () => clearTimeout(t)));
  document.addEventListener('click', (e) => { if (fired && e.target.closest('[data-lp]')) { e.stopImmediatePropagation(); e.preventDefault(); fired = false; } }, true);
})();
