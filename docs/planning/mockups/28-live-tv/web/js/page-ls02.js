/* LS-02 Senderübersicht (web, sample page of the core): groups, filters in the address, search, favourites,
   now and next with progress, list and tiles, context menu, every state (empty, filtered empty, loading,
   error, without programme data, disturbed channel). 14-oberflaeche.md 3.2. */
'use strict';

const LS02_FIELDS = ['group', 'country', 'lang', 'category'];
function ls02Filters(q) { return { q: q.q || '', group: q.group || '', country: q.country || '', lang: q.lang || '', category: q.category || '', view: q.view === 'tiles' ? 'tiles' : 'list' }; }
function ls02Href(f, patch) {
  const v = Object.assign({}, f, patch || {});
  const qs = ['q', ...LS02_FIELDS].filter((k) => v[k]).map((k) => `${k}=${encodeURIComponent(v[k])}`);
  if (v.view === 'tiles') qs.push('view=tiles');
  return '#/live/channels' + (qs.length ? '?' + qs.join('&') : '');
}
const ls02Label = { group: (v) => (v === 'fav' ? 'Meine Sender' : (LIVE.groups.find((g) => g.id === v) || {}).name), country: (v) => LIVE.countries[v], lang: (v) => LIVE.languages[v], category: (v) => v };
const ls02FieldName = { group: 'Gruppe', country: 'Land', lang: 'Sprache', category: 'Kategorie' };

/** one channel: now/next block (also used by the tiles) */
function ls02Now(c, nn) {
  const epg = SIM.epgOf(c);
  if (SIM.chState(c) === 'disturbed') return `<span class="cn-now"><span class="cn-t warnline"><span class="dot warn"></span>${esc(LU.TX.unavailable)}</span>${nn.now ? `<span class="cn-m">${esc(nn.now.title)} · ${fmt.range(nn.now.start, nn.now.end)}</span>` : ''}</span>`;
  if (!nn.now) return `<span class="cn-now"><span class="cn-t none">${esc(epg === 'none' ? LU.TX.epgNoneButPlays : LU.TX.epgNone)}</span></span>`;
  return `<span class="cn-now"><span class="cn-t">${esc(nn.now.title)}${nn.now.sub ? ` <span class="cn-sub">«${esc(nn.now.sub)}»</span>` : ''}</span><span class="cn-m">${fmt.range(nn.now.start, nn.now.end)} · ${LU.nowText('left', nn.now.end)}</span>${LU.progress(nn.now.start, nn.now.end, { label: `Fortschritt von ${nn.now.title}` })}</span>`;
}
function ls02Next(nn) {
  if (nn.next) return `<span class="cn-next"><span class="cn-lbl vh">Danach</span><span class="cn-t2">${esc(nn.next.title)}</span><span class="cn-m">ab ${fmt.time(nn.next.start)}</span></span>`;
  if (nn.now) return `<span class="cn-next"><span class="cn-lbl vh">Danach</span><span class="cn-m">${esc(LU.TX.epgNone)}</span></span>`;
  return '<span class="cn-next"></span>';
}
function ls02Badges(c, nn) {
  let h = '';
  if (SIM.chState(c) === 'disturbed') h += `<span class="chip warn">${icon('alert', 14)}${esc(LU.TX.disturbed)}</span>`;
  if (SIM.rights().recordings && SIM.runningOn(c.id)) h += LU.badge('recording', { text: true });
  else if (nn.now) h += LU.programBadges(nn.now, { text: false });
  return h;
}
function ls02Menu(c, nn, where) {
  const fav = SIM.isFav(c.id);
  const items = [{ label: 'Senderprogramm', href: `#/live/guide/channel/${c.id}`, icon: 'calendar' }];
  if (nn.now) items.push({ label: 'Jetzige Sendung: Details', href: `#/live/programs/${nn.now.id}`, icon: 'info' });
  items.push({ label: 'Einschalten', act: 'tune', data: { ch: c.id }, icon: 'play' });
  items.push({ label: fav ? 'Aus Meine Sender' : 'Zu Meine Sender', act: 'fav', data: { ch: c.id }, icon: fav ? 'x' : 'heart' });
  return popMenu(`ls02:${where}:${c.id}`, icon('dots', 18), items, { aria: `Weitere Aktionen für ${c.name}` });
}
function ls02Heart(c) {
  const on = SIM.isFav(c.id);
  return `<button type="button" class="btn ghost icon sm heart${on ? ' on' : ''}" data-act="fav" data-ch="${c.id}" aria-pressed="${on}" aria-label="${esc(c.name)} ${on ? 'aus Meine Sender entfernen' : 'zu Meine Sender hinzufügen'}" data-fid="fav:${c.id}" tabindex="-1">${icon(on ? 'heartOn' : 'heart', 18)}</button>`;
}
function ls02Row(c, list, active) {
  const nn = SIM.nowNext(c.id);
  const name = `${c.no} ${c.name}`;
  return `<li class="chrow${SIM.chState(c) !== 'ok' ? ' bad' : ''}" data-ch="${c.id}">
    <button type="button" class="chmain" data-act="tune" data-ch="${c.id}" data-list="${list.join(',')}" data-list-name="filter" data-fid="row:${c.id}" tabindex="${active ? 0 : -1}" aria-label="${esc(name)} einschalten${nn.now ? ', jetzt ' + esc(nn.now.title) : ''}">
      <span class="cn-no">${c.no}</span>${LU.logo(c)}<span class="cn-name">${esc(c.name)}${SIM.isFav(c.id) ? `<span class="vh">, in Meine Sender</span>` : ''}</span>${ls02Now(c, nn)}${ls02Next(nn)}</button>
    <span class="cn-badges">${ls02Badges(c, nn)}</span>${ls02Heart(c)}<span class="cn-menu">${ls02Menu(c, nn, 'row').replace('data-act="menu"', 'data-act="menu" tabindex="-1"')}</span></li>`;
}
function ls02Tile(c, list, active) {
  const nn = SIM.nowNext(c.id);
  return `<li class="chtile${SIM.chState(c) !== 'ok' ? ' bad' : ''}" data-ch="${c.id}">
    <button type="button" class="chmain" data-act="tune" data-ch="${c.id}" data-list="${list.join(',')}" data-list-name="filter" data-fid="tile:${c.id}" tabindex="${active ? 0 : -1}" aria-label="${c.no} ${esc(c.name)} einschalten${nn.now ? ', jetzt ' + esc(nn.now.title) : ''}">
      <span class="ct-art">${LU.logo(c)}<span class="ct-no">${c.no}</span></span>
      <span class="ct-bd"><b>${esc(c.name)}</b>${ls02Now(c, nn)}</span></button>
    <span class="ct-acts"><span class="cn-badges">${ls02Badges(c, nn)}</span>${ls02Heart(c).replace('tabindex="-1"', '')}${ls02Menu(c, nn, 'tile')}</span></li>`;
}

route('/live/channels', (pm, query) => {
  const f = ls02Filters(query);
  const total = SIM.channels().length;
  const list = SIM.channels(f);
  const ids = list.map((c) => c.id);
  const mode = liveMode();
  const nActive = LS02_FIELDS.filter((k) => f[k]).length;
  if (!ui.ls02row || !ids.includes(ui.ls02row)) ui.ls02row = ids[0];
  const sel = (k, all, opts) => `<label class="fld fsel"><span class="lbl">${ls02FieldName[k]}</span><select class="sel" data-act-change="ls02Filter" data-key="${k}" data-fid="ls02:${k}" aria-label="${ls02FieldName[k]}"><option value="">${all}</option>${opts.map(([v, t]) => `<option value="${esc(v)}"${f[k] === v ? ' selected' : ''}>${esc(t)}</option>`).join('')}</select></label>`;
  const groupOpts = [['fav', 'Meine Sender'], ...LIVE.groups.map((g) => [g.id, g.name])];
  const cats = [...new Set(LIVE.channels.map((c) => c.category))].map((v) => [v, v]);
  const tools = `<div class="ls02-tools" role="search">
      <label class="fld fsearch"><span class="vh">Sender suchen</span><span class="sic">${icon('search', 18)}</span><input class="inp" type="search" placeholder="Sender suchen" value="${esc(f.q)}" data-ls02q data-fid="ls02:q" aria-label="Sender suchen (Name oder Nummer)"></label>
      <div class="ls02-facets">${sel('group', 'Alle Sender', groupOpts)}${sel('country', 'Alle Länder', Object.entries(LIVE.countries))}${sel('lang', 'Alle Sprachen', Object.entries(LIVE.languages))}${sel('category', 'Alle Kategorien', cats)}</div>
      <button type="button" class="btn secondary ls02-fbtn" data-act="ls02Sheet" data-fid="ls02:sheet">${icon('filter', 18)}Filter${nActive ? ` (${nActive})` : ''}</button>
      <div class="seg" role="group" aria-label="Ansicht"><button type="button" aria-pressed="${f.view === 'list'}" data-act="ls02View" data-v="list" data-fid="ls02:vl">${icon('list', 16)}<span class="vt">Liste</span></button><button type="button" aria-pressed="${f.view === 'tiles'}" data-act="ls02View" data-v="tiles" data-fid="ls02:vt">${icon('grid', 16)}<span class="vt">Kacheln</span></button></div>
    </div>`;
  const active = nActive || f.q ? `<div class="afilters" aria-label="Aktive Filter">${f.q ? `<a class="chip outline" href="${ls02Href(f, { q: '' })}" data-fid="af:q">«${esc(f.q)}» ${icon('x', 12)}<span class="vh">Suche entfernen</span></a>` : ''}${LS02_FIELDS.filter((k) => f[k]).map((k) => `<a class="chip outline" href="${ls02Href(f, { [k]: '' })}" data-fid="af:${k}">${ls02FieldName[k]}: ${esc(ls02Label[k](f[k]) || f[k])} ${icon('x', 12)}<span class="vh">Filter entfernen</span></a>`).join('')}<a class="lnk" href="${ls02Href({ view: f.view })}" data-fid="af:all">Alle Filter entfernen</a></div>` : '';
  let body;
  if (mode === 'loading') body = skelRows(8, 72);
  else if (mode === 'error') body = liveError('Die Sender konnten nicht geladen werden.');
  else if (!total) body = emptyState('Keine Sender', LU.TX.empty);
  else if (!list.length) body = emptyState('Kein Sender passt zu den Filtern', f.group === 'fav' && !SIM.favorites().length ? 'Du hast noch keine Lieblingssender. Tippe in der Liste aufs Herz.' : 'Ändere die Suche oder entferne Filter.', `<a class="btn secondary" href="${ls02Href({ view: f.view })}" data-fid="ls02:clear">Filter entfernen</a>`);
  else {
    const g = f.group && f.group !== 'fav' ? LIVE.groups.find((x) => x.id === f.group) : null;
    const gh = f.group ? `<h2 class="ghead">${f.group === 'fav' ? 'Meine Sender' : 'Gruppe ' + esc(g ? g.name : f.group)} · ${list.length} Sender</h2>` : '';
    const noEpg = SIM.flags().noEpg ? notice('', 'Für die Sender gibt es gerade keine Programmdaten.', 'Fernsehen geht trotzdem.') : '';
    body = noEpg + gh + (f.view === 'tiles'
      ? `<ul class="chtiles" data-roving="tile" aria-label="Sender">${list.map((c) => ls02Tile(c, ids, c.id === ui.ls02row)).join('')}</ul>`
      : `<div class="chhead" aria-hidden="true"><span>Nr.</span><span></span><span>Sender</span><span>Jetzt</span><span>Danach</span></div><ul class="chlist" data-roving="row" aria-label="Sender, Pfeiltasten wechseln, Enter schaltet ein">${list.map((c) => ls02Row(c, ids, c.id === ui.ls02row)).join('')}</ul>`);
  }
  const count = !total ? '' : list.length === total ? `${total} Sender` : `${list.length} von ${total} Sendern`;
  return { type: 'viewer', nav: 'live', title: 'Sender', html: livePage({ tab: 'channels', title: 'Sender', count, html: tools + active + `<div class="ls02-body">${body}</div>` }) };
});

/* search: updates the address without a history entry, keeps focus */
document.addEventListener('input', (e) => {
  const el = e.target.closest('[data-ls02q]'); if (!el) return;
  const f = ls02Filters(parseHash().query); history.replaceState(null, '', ls02Href(f, { q: el.value })); render();
});
document.addEventListener('change', (e) => {
  const el = e.target.closest('[data-act-change="ls02Filter"]'); if (!el) return;
  const f = ls02Filters(parseHash().query); go(ls02Href(f, { [el.dataset.key]: el.value }));
});
ACT.ls02View = (d) => { const f = ls02Filters(parseHash().query); history.replaceState(null, '', ls02Href(f, { view: d.v })); render(); };
ACT.ls02Sheet = () => { const f = ls02Filters(parseHash().query); LiveDlg.filterSheet(chFilterSheet(f, (v) => go(ls02Href(f, v)))); };

/* roving focus: the list is one Tab stop; ↑/↓ rows (tiles: 2D), → reaches heart and menu, ← back */
document.addEventListener('keydown', (e) => {
  const box = e.target.closest && e.target.closest('[data-roving]'); if (!box || ui.menu || ui.dialog) return;
  if (e.target.closest('.lmenu')) return;
  const item = e.target.closest('li'); if (!item) return;
  const items = $$(':scope > li', box); const k = items.indexOf(item);
  const parts = $$('.chmain, .heart, .cn-menu .btn, .ct-acts .lmenu-w .btn', item);
  const pi = parts.indexOf(e.target);
  const tiles = box.dataset.roving === 'tile';
  const toItem = (n) => { n = Math.max(0, Math.min(items.length - 1, n)); const m = $('.chmain', items[n]); $$('.chmain', box).forEach((x) => x.setAttribute('tabindex', '-1')); m.setAttribute('tabindex', '0'); ui.ls02row = items[n].dataset.ch; m.focus(); m.scrollIntoView({ block: 'nearest' }); };
  const cols = tiles ? Math.max(1, items.filter((x) => x.offsetTop === items[0].offsetTop).length) : 1;
  if (e.key === 'ArrowDown') { e.preventDefault(); toItem(k + cols); }
  else if (e.key === 'ArrowUp') { e.preventDefault(); toItem(k - cols); }
  else if (e.key === 'Home') { e.preventDefault(); toItem(0); }
  else if (e.key === 'End') { e.preventDefault(); toItem(items.length - 1); }
  else if (e.key === 'PageDown') { e.preventDefault(); toItem(k + 8 * cols); }
  else if (e.key === 'PageUp') { e.preventDefault(); toItem(k - 8 * cols); }
  else if (e.key === 'ArrowRight') { e.preventDefault(); if (tiles && pi <= 0) toItem(k + 1); else if (parts[pi + 1]) parts[pi + 1].focus(); }
  else if (e.key === 'ArrowLeft') { e.preventDefault(); if (tiles && pi <= 0) toItem(k - 1); else if (pi > 0) parts[pi - 1].focus(); }
});
