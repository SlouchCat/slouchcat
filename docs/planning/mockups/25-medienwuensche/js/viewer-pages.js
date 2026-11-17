/* M-07 Suche, M-08 Meine Wünsche, M-09 Benachrichtigungen, M-10 Benachrichtigungskanäle, M-11 Profile, M-12 Profilwahl.
   Global helpers of this file carry the prefix «vp»; channelSettingsHtml(opts) is shared with the administration. */
'use strict';

/* ---------- small shared bits of this file ---------- */
const vpNew = () => '<span class="newtag">NEU</span>';
const vpPill = (fid, label, pressed, attrs) => `<button type="button" class="pill" aria-pressed="${!!pressed}" data-fid="${esc(fid)}" ${attrs}>${label}</button>`;
const vpOnlyPrimary = (title, body) => notice('warn', title || 'Nur im Hauptprofil', body, `<a class="btn secondary" href="#/profiles" data-fid="vp:switch">${icon('users')}Profil wechseln</a>`);
const vpPlural = (n, one, many) => `${n} ${n === 1 ? one : many}`;
ACT.vpMulti = (d) => { const arr = getPath(d.path) || []; setPath(d.path, arr.includes(d.value) ? arr.filter((x) => x !== d.value) : [...arr, d.value]); render(); };

/* ======================================================================================
   M-07 Suche
   ====================================================================================== */
const VP_DOWNLOADS = ['dune-2', 'mobland', 'interstellar']; /* titles saved on «this device» (offline), simulated */
const VP_COLLECTIONS = [{ name: 'Dune Collection', keys: ['dune', 'dune-2'] }, { name: 'Matrix Collection', keys: ['matrix'] }];
const VP_TYPES = [['movie', 'Filme'], ['series', 'Serien'], ['audiobook', 'Hörbücher'], ['podcast', 'Podcasts']];
const VP_AGES = [0, 6, 12, 14, 16, 18];
const vpSearchDefault = () => ({ genre: '', from: '', to: '', rating: '', types: [], avail: [] });

function vpSearchActive(f) {
  const parts = [];
  if (f.genre) parts.push(esc(f.genre));
  if (f.from !== '' && f.from != null) parts.push(`ab ${esc(f.from)}`);
  if (f.to !== '' && f.to != null) parts.push(`bis ${esc(f.to)}`);
  if (f.rating !== '' && f.rating != null) parts.push(`höchstens ${ageLabel(Number(f.rating))}`);
  f.types.forEach((k) => parts.push((VP_TYPES.find((x) => x[0] === k) || [k, k])[1]));
  const AV = { local: 'Auf dem Server', download: 'Auf diesem Gerät', missing: 'Noch nicht da' };
  f.avail.forEach((k) => parts.push(AV[k] || k));
  return parts;
}

route('/search', (pm, query) => {
  const p = curProfile(), cr = canRequest(p), w = S.settings.wishes;
  if (query.q != null && ui.vpQSeen !== query.q) { ui.q = query.q; ui.vpQSeen = query.q; }
  const f = (ui.filter.search = { ...vpSearchDefault(), ...(ui.filter.search || {}) });
  const q = (ui.q || '').trim().toLowerCase();
  const short = q.length === 1, hasQ = q.length >= 2;
  const mayMissing = w.fromSearch && cr.ok;
  const av = f.avail.filter((k) => k !== 'missing' || mayMissing);

  const match = (t) => !hasQ || [t.title, t.original, ...(t.cast || []).map((c) => c.name)].some((s) => (s || '').toLowerCase().includes(q));
  const pass = (t) => (!f.genre || t.genres.includes(f.genre))
    && (f.from === '' || f.from == null || Number(t.year) >= Number(f.from))
    && (f.to === '' || f.to == null || Number(t.year) <= Number(f.to))
    && (f.rating === '' || f.rating == null || t.age <= Number(f.rating))
    && (!f.types.length || f.types.includes(t.kind === 'movie' ? 'movie' : 'series'));
  const libOk = (t) => !av.length || av.includes('local') || (av.includes('download') && VP_DOWNLOADS.includes(t.key));
  const missOk = !av.length || av.includes('missing');
  const byTitle = (a, b) => a.title.localeCompare(b.title, 'de');

  const lib = short ? [] : window.TITLES.filter((t) => isPresent(t.key) && t.age <= effMax(p) && match(t) && pass(t) && libOk(t)).sort(byTitle);
  const showMiss = !short && mayMissing && missOk && (hasQ || av.includes('missing'));
  const miss = showMiss ? window.TITLES.filter((t) => !isPresent(t.key) && missingVisible(p, t) && match(t) && pass(t)).sort(byTitle) : [];
  const unmappedHint = false; /* rule F-9: no hint for viewers; the administrator sees a missing mapping in the administration */
  const people = [];
  if (hasQ) [...lib, ...miss].forEach((t) => (t.cast || []).forEach((c) => { if (c.name.toLowerCase().includes(q) && !people.some((x) => x.name === c.name)) people.push({ ...c, from: t.title }); }));
  const colls = hasQ ? VP_COLLECTIONS.filter((c) => c.name.toLowerCase().includes(q)).map((c) => ({ ...c, items: c.keys.map((k) => T[k]).filter((t) => isPresent(t.key) && t.age <= effMax(p)) })).filter((c) => c.items.length) : [];
  const total = lib.length + miss.length;

  /* filters */
  const genres = [...new Set(window.TITLES.filter((t) => (isPresent(t.key) ? t.age <= effMax(p) : mayMissing && missingVisible(p, t))).flatMap((t) => t.genres))].sort((a, b) => a.localeCompare(b, 'de'));
  const active = vpSearchActive({ ...f, avail: av });
  const grp = (id, label, body) => `<div class="vp-fg"><span class="vp-fl" id="vp-fl-${id}">${label}</span><div class="pills" role="group" aria-labelledby="vp-fl-${id}">${body}</div></div>`;
  const filterBody = `<div class="vp-filters">
    ${grp('genre', 'Genre', vpPill('sf:g:', 'Alle Genres', !f.genre, 'data-act="set" data-path="ui.filter.search.genre" data-value=""') + genres.map((g) => vpPill('sf:g:' + g, esc(g), f.genre === g, `data-act="set" data-path="ui.filter.search.genre" data-value="${esc(g)}"`)).join(''))}
    <div class="vp-fg"><span class="vp-fl">Jahr</span><div class="vp-years">
      <div class="fld"><label for="vp-yf">Jahr von</label><input class="inp" id="vp-yf" type="number" inputmode="numeric" min="1900" max="2030" placeholder="z. B. 1990" data-bind="ui.filter.search.from" data-num data-live data-fid="sf:from" value="${esc(f.from)}"></div>
      <div class="fld"><label for="vp-yt">Jahr bis</label><input class="inp" id="vp-yt" type="number" inputmode="numeric" min="1900" max="2030" placeholder="z. B. 2026" data-bind="ui.filter.search.to" data-num data-live data-fid="sf:to" value="${esc(f.to)}"></div>
    </div></div>
    ${grp('rating', 'Altersfreigabe höchstens', vpPill('sf:r:', 'Alle', f.rating === '' || f.rating == null, 'data-act="set" data-path="ui.filter.search.rating" data-value=""') + VP_AGES.filter((a) => a <= effMax(p)).map((a) => vpPill('sf:r:' + a, ageLabel(a), String(f.rating) === String(a), `data-act="set" data-path="ui.filter.search.rating" data-value="${a}" data-num`)).join(''))}
    ${grp('type', 'Medientyp', VP_TYPES.map(([k, l]) => vpPill('sf:t:' + k, l, f.types.includes(k), `data-act="vpMulti" data-path="ui.filter.search.types" data-value="${k}"`)).join(''))}
    ${grp('avail', 'Verfügbarkeit', vpPill('sf:a:local', 'Auf dem Server', av.includes('local'), 'data-act="vpMulti" data-path="ui.filter.search.avail" data-value="local"') + vpPill('sf:a:download', 'Auf diesem Gerät', av.includes('download'), 'data-act="vpMulti" data-path="ui.filter.search.avail" data-value="download"') + (mayMissing ? vpPill('sf:a:missing', `Noch nicht da ${vpNew()}`, av.includes('missing'), 'data-act="vpMulti" data-path="ui.filter.search.avail" data-value="missing"') : ''))}
    ${active.length ? `<div class="acts"><button type="button" class="btn ghost" data-act="vpSearchReset" data-fid="sf:reset">${icon('refresh')}Filter zurücksetzen</button></div>` : ''}
  </div>`;
  const filterMeta = active.length ? `<span class="vp-active">${active.map((a) => `<span class="chip info">${a}</span>`).join('')}</span>` : '<span>Keine Filter aktiv</span>';

  /* status and results */
  let status;
  if (short) status = 'Gib mindestens zwei Zeichen ein.';
  else if (!hasQ && !active.length) status = `Suche nach Titeln, Personen und Sammlungen oder wähle einen Filter. ${vpPlural(total, 'Titel', 'Titel')} in der Bibliothek.`;
  else status = `${total} Titel gefunden`;
  let res = '';
  if (!short) {
    if (lib.length) res += `<section class="vp-sec" aria-labelledby="vp-s-titles"><h2 id="vp-s-titles">Titel</h2><ul class="grid">${lib.map((t) => posterCard(t, { ctx: 's' })).join('')}</ul></section>`;
    if (miss.length) res += `<section class="vp-sec vp-miss" aria-labelledby="vp-s-miss"><div class="vp-sech"><h2 id="vp-s-miss">Noch nicht in der Bibliothek ${vpNew()}</h2><p class="sub">Aus TMDB. Diese Titel kannst du anfordern.</p>${hasQ && discoverOn(p) ? `<p><a class="lnk" href="#/discover/search?q=${encodeURIComponent((ui.q || '').trim())}" data-fid="s:disc">${icon('wish', 16)}Alle Treffer unter Wünsche suchen</a></p>` : ''}</div><ul class="grid">${miss.map((t) => posterCard(t, { ctx: 'sm' })).join('')}</ul></section>`;
    if (unmappedHint) res += `<section class="vp-sec">${notice('', 'Noch nicht in der Bibliothek', esc(cr.text))}</section>`;
    if (people.length) res += `<section class="vp-sec" aria-labelledby="vp-s-people"><h2 id="vp-s-people">Personen</h2><ul class="vp-people">${people.slice(0, 12).map((c, i) => `<li class="person"><a href="#/unchanged/person" data-fid="sp:${i}" class="vp-pl"><span class="pt">${c.img ? img(c.img, c.name) : esc(c.name[0])}</span><b>${esc(c.name)}</b><span>${esc(c.role || '')}${c.role ? ' · ' : ''}${esc(c.from)}</span></a></li>`).join('')}</ul></section>`;
    if (colls.length) res += `<section class="vp-sec" aria-labelledby="vp-s-coll"><h2 id="vp-s-coll">Sammlungen</h2><ul class="grid">${colls.map((c, i) => `<li class="pc"><a class="pl" href="#/unchanged/sammlungen" data-fid="sc:${i}"><span class="frame">${img(c.items[0].poster, c.name)}</span><span class="cap"><b>${esc(c.name)}</b><span>${vpPlural(c.items.length, 'Titel', 'Titel')}</span></span></a></li>`).join('')}</ul></section>`;
    if (!total && !people.length && !colls.length && !unmappedHint) res += notice('', 'Nichts gefunden', 'Versuche einen anderen Begriff oder weniger Filter.');
  }

  return { type: 'viewer', nav: 'search', title: 'Suche', html: `<div class="page vp-scope vp-search">
    <div class="head"><h1>Suche</h1>
      <div class="vp-q">${icon('search')}<input class="inp" type="search" data-bind="ui.q" data-live data-fid="s:q" value="${esc(ui.q || '')}" placeholder="Titel, Personen, Sammlungen" aria-label="Suchbegriff" autocomplete="off"></div>
    </div>
    ${band('vp-sf', 'Filter', filterMeta, filterBody, { viewer: true, icon: 'settings' })}
    <p class="vp-status" role="status">${status}</p>
    ${res}
  </div>` };
});
ACT.vpSearchReset = () => { ui.filter.search = vpSearchDefault(); render(); };

/* ======================================================================================
   M-08 Meine Wünsche
   ====================================================================================== */
const VP_CLOSED = ['declined', 'failed', 'withdrawn'];
const VP_WFILTER = [['all', 'Alle', () => true], ['open', 'Offen', (w) => OPEN_STATES.includes(w.state)], ['available', 'Verfügbar', (w) => w.state === 'available'], ['closed', 'Abgeschlossen', (w) => VP_CLOSED.includes(w.state)]];
const VP_NEXT = {
  queued: ['An Seerr übermitteln', 'Freigabe in Seerr', 'Beschaffung durch Radarr oder Sonarr', 'In Slouchcat verfügbar'],
  pending: ['Freigabe in Seerr', 'Beschaffung durch Radarr oder Sonarr', 'In Slouchcat verfügbar'],
  processing: ['Beschaffung durch Radarr oder Sonarr', 'In Slouchcat verfügbar'],
  partial: ['Weitere Staffeln treffen ein', 'Alle gewünschten Staffeln verfügbar'],
};
function vpSteps(w) {
  const done = w.events.map((e) => `<li class="${['failed', 'declined'].includes(e.k) ? 'bad' : 'done'}">${esc(e.t)}<time>${fmtDate(e.at)}, ${fmtTime(e.at)}</time></li>`);
  const todo = (VP_NEXT[w.state] || []).map((t, i) => `<li class="${i === 0 ? 'now' : 'vp-todo'}">${esc(t)}<time>${i === 0 ? 'Läuft gerade' : 'Ausstehend'}</time></li>`);
  return `<ol class="steps" aria-label="Verlauf">${done.join('')}${todo.join('')}</ol>`;
}
function vpWishRow(w, p, all) {
  const t = T[w.titleKey], href = titleHref(t);
  const kind = t.kind === 'movie' ? `Film · ${esc(t.year)}` : `Serie · ${w.seasons.length ? 'Staffel ' + w.seasons.join(', ') : 'alle Staffeln'}`;
  const who = all && w.owner ? ` · ${esc(profile(w.owner).name)}` : '';
  const tags = [wsChip(w.state)];
  { const dl = dlChip(w); if (dl) tags.push(dl); }
  if (w.joined.includes(p.id)) tags.push(`<span class="chip info">${icon('bell', 14)}Du wünschst mit</span>`);
  if (w.origin === 'jellyseerr') tags.push(`<span class="chip blue">${icon('external', 14)}In Seerr angelegt</span>`);
  let extra = '';
  if (w.state === 'queued' && w.detail) extra = `${esc(w.detail)}. Slouchcat versucht es weiter.`;
  else if (w.state === 'failed') extra = esc(FAIL_TEXT[w.failReason] || 'Die Beschaffung hat nicht geklappt.');
  else if (w.state === 'pending') extra = 'Wartet auf die Freigabe in Seerr.';
  const seasons = w.media === 'show' && w.seasons.length ? `<div class="vp-seasons">${w.seasons.map((n) => `<span class="vp-ss">Staffel ${n}: ${wsChip(w.seasonState?.[n] || w.state)}</span>`).join('')}</div>` : '';
  const acts = [];
  if (w.state === 'available' || w.state === 'partial') {
    acts.push(`<button type="button" class="btn primary sm" data-act="play" data-key="${t.key}" data-fid="w:play:${w.id}">${icon('play', 16)}Abspielen</button>`);
    acts.push(`<a class="btn secondary sm" href="${href}" data-fid="w:go:${w.id}">${icon('info', 16)}Zum Titel</a>`);
  }
  if (['queued', 'pending'].includes(w.state) && w.owner === p.id && S.settings.wishes.withdraw) acts.push(`<button type="button" class="btn secondary sm" data-act="dialog" data-name="vpWithdraw" data-id="${w.id}" data-fid="w:wd:${w.id}">${icon('undo', 16)}Zurückziehen</button>`);
  if (OPEN_STATES.includes(w.state) && w.joined.includes(p.id)) acts.push(`<button type="button" class="btn secondary sm" data-act="vpLeave" data-id="${w.id}" data-fid="w:leave:${w.id}">${icon('bell', 16)}Nicht mehr benachrichtigen</button>`);
  if (VP_CLOSED.includes(w.state)) acts.push(`<button type="button" class="btn secondary sm" data-act="askWish" data-key="${t.key}" data-fid="w:again:${w.id}">${icon('refresh', 16)}Erneut anfordern</button>`);
  const hid = 'wh:' + w.id, open = isOpen(hid);
  acts.push(`<button type="button" class="btn ghost sm" data-act="open" data-id="${hid}" aria-expanded="${open}" aria-controls="vp-st-${w.id}" data-fid="w:hist:${w.id}"><span class="vp-chev">${icon('chevR', 16)}</span>Verlauf</button>`);
  return `<li class="lrow vp-wish" id="vp-w-${w.id}">
    <a class="thumb" href="${href}" tabindex="-1" aria-hidden="true" data-fid="w:th:${w.id}">${img(t.poster, t.title)}</a>
    <div class="bd2">
      <a class="ttl vp-tl" href="${href}" data-fid="w:tt:${w.id}">${esc(t.title)}</a>
      <span class="ln">${kind}</span>
      <span class="ln">Angefordert ${ago(w.created)}${who}</span>
      <div class="vp-tags">${tags.join('')}</div>
      ${extra ? `<p class="ln vp-extra">${extra}</p>` : ''}
      ${seasons}
      <div class="ra">${acts.join('')}</div>
      <div id="vp-st-${w.id}">${open ? vpSteps(w) : ''}</div>
    </div></li>`;
}
route('/wishes', (pm, query) => {
  const p = curProfile(), cr = canRequest(p);
  if (query.focus && ui.vpFocused !== query.focus) { ui.vpFocused = query.focus; ui.open['wh:' + query.focus] = true; ui.filter.wishes = 'all'; ui.vpScrollTo = query.focus; if (!myWishes(p).some((w) => w.id === query.focus) && p.primary) ui.filter.allProfiles = true; }
  const all = p.primary && !!ui.filter.allProfiles;
  const list = myWishes(p, all);
  const fk = ui.filter.wishes || 'all';
  const fdef = VP_WFILTER.find((x) => x[0] === fk) || VP_WFILTER[0];
  const shown = list.filter(fdef[2]);
  const openN = list.filter((w) => OPEN_STATES.includes(w.state)).length;
  let body;
  if (!list.length) {
    body = !cr.ok ? notice('', 'Keine Wünsche', esc(cr.text))
      : `<div class="empty vp-empty"><b>Noch keine Wünsche</b><p>Fehlt dir ein Film oder eine Serie? Unter «Entdecken», «Filme», «Serien» und in der Suche findest du Titel, die noch nicht in der Bibliothek sind, und kannst sie anfordern.</p><div class="acts"><a class="btn secondary" href="#/discover" data-fid="w:e:disc">${icon('star')}Zu «Entdecken»</a><a class="btn secondary" href="#/search" data-fid="w:e:search">${icon('search')}Zur Suche</a></div></div>`;
  } else {
    body = `<div class="vp-wbar"><div class="pills" role="group" aria-label="Wünsche filtern">${VP_WFILTER.map(([k, l, fn]) => vpPill('w:f:' + k, `${l}<span class="n">${list.filter(fn).length}</span>`, fk === k, `data-act="set" data-path="ui.filter.wishes" data-value="${k}"`)).join('')}</div>
      ${p.primary ? `<div class="vp-swrow">${sw('ui.filter.allProfiles', 'Wünsche aller Profile meines Kontos')}<span>Wünsche aller Profile meines Kontos</span></div>` : ''}</div>
      ${!cr.ok ? notice('', 'Neue Wünsche sind nicht möglich', esc(cr.text)) : ''}
      ${shown.length ? `<ul class="vp-list">${shown.map((w) => vpWishRow(w, p, all)).join('')}</ul>` : `<div class="empty vp-empty"><b>Keine Wünsche in dieser Auswahl</b><div class="acts"><button type="button" class="btn secondary" data-act="set" data-path="ui.filter.wishes" data-value="all" data-fid="w:e:all">Alle zeigen</button></div></div>`}`;
  }
  return {
    type: 'viewer', nav: 'wishes', title: 'Meine Wünsche',
    html: `<div class="page vp-scope">${wishNav('mine')}<div class="head"><h1>Meine Wünsche</h1><p class="sub">${openN} offen</p><div class="acts"><button type="button" class="btn ghost" data-act="back" data-fid="w:back">${icon('arrowL')}Zurück</button></div></div>${body}</div>`,
    after(root) { if (ui.vpScrollTo) { const el = root.querySelector('#vp-w-' + CSS.escape(ui.vpScrollTo)); ui.vpScrollTo = null; if (el) el.scrollIntoView({ block: 'center' }); } },
  };
});
DIALOGS.vpWithdraw = ({ id }) => {
  const w = wishById(id), t = T[w.titleKey];
  return {
    title: 'Wunsch zurückziehen?',
    body: `<p class="lead">«${esc(t.title)}» wird nicht weiter angefordert. Slouchcat löscht die Anfrage in Seerr, solange sie nicht freigegeben ist.</p>${w.joined.length ? notice('', 'Ein anderes Profil wünscht mit', 'Der Wunsch bleibt für dieses Profil bestehen; du wirst nicht mehr benachrichtigt.') : ''}`,
    actions: `<button type="button" class="btn ghost" data-act="close" data-fid="wdd:x">Abbrechen</button><button type="button" class="btn primary" data-act="vpWithdrawOk" data-id="${id}" data-fid="wdd:ok" data-autofocus>${icon('undo')}Zurückziehen</button>`,
  };
};
ACT.vpWithdrawOk = (d) => {
  const w = wishById(d.id), t = T[w.titleKey];
  withdrawWish(d.id);
  closeDialog();
  toast(w.state === 'withdrawn' ? `«${esc(t.title)}» ist zurückgezogen.` : `Du wünschst «${esc(t.title)}» nicht mehr. Ein anderes Profil übernimmt den Wunsch.`, 'ok');
  commit();
};
ACT.vpLeave = (d) => { const t = T[wishById(d.id).titleKey]; withdrawWish(d.id); toast(`Du wirst zu «${esc(t.title)}» nicht mehr benachrichtigt.`, 'ok'); commit(); };

/* ======================================================================================
   M-09 Benachrichtigungen
   ====================================================================================== */
const VP_NKIND = {
  wish_available: ['check', 'ok', 'Wunsch verfügbar'], wish_season_available: ['check', 'ok', 'Staffel verfügbar'],
  wish_approved: ['truck', 'info', 'Wunsch freigegeben'], wish_declined: ['ban', 'bad', 'Wunsch abgelehnt'],
  wish_failed: ['alert', 'bad', 'Wunsch fehlgeschlagen'], new_episode: ['tv', '', 'Neue Folge'],
};
function vpNoteHref(n) { const h = notifText(n).href; return h === '#/wishes' && n.wishId ? '#/wishes?focus=' + n.wishId : h; }
route('/notifications', () => {
  const p = curProfile();
  const list = S.notifications.filter((n) => n.profileId === p.id).sort((a, b) => b.at - a.at);
  const un = list.filter((n) => !n.read).length;
  const rows = list.map((n) => {
    const t = T[n.titleKey], tx = notifText(n), k = VP_NKIND[n.kind] || VP_NKIND.new_episode;
    return `<li class="${cls('lrow vp-note', !n.read && 'unread')}">
      <a class="vp-nl" href="${vpNoteHref(n)}" data-act="vpRead" data-id="${n.id}" data-fid="n:o:${n.id}" aria-label="${n.read ? '' : 'Ungelesen: '}${esc(tx.title)}, ${esc(tx.ctx)}, ${ago(n.at)}">
        <span class="thumb">${img(t.poster, t.title)}</span>
        <span class="bd2"><span class="vp-nk ${k[1]}">${icon(k[0], 14)}${k[2]}</span><span class="ttl">${esc(tx.title)}</span><span class="ln">${esc(tx.ctx)}</span><span class="ln">${ago(n.at)}</span></span>
      </a>
      <button type="button" class="btn ghost sm vp-nr" data-act="vpToggleRead" data-id="${n.id}" data-fid="n:r:${n.id}">${icon(n.read ? 'bell' : 'check', 16)}${n.read ? 'Als ungelesen markieren' : 'Als gelesen markieren'}</button>
    </li>`;
  }).join('');
  return {
    type: 'viewer', nav: '', title: 'Benachrichtigungen',
    html: `<div class="page vp-scope"><div class="head"><h1>Benachrichtigungen</h1><p class="sub">${un} ungelesen</p><div class="acts">
      <button type="button" class="btn ghost" data-act="back" data-fid="n:back">${icon('arrowL')}Zurück</button>
      <a class="btn ghost" href="#/settings/notifications" data-fid="n:ch">${icon('settings')}Kanäle einstellen</a>
      ${un ? `<button type="button" class="btn ghost" data-act="vpReadAll" data-fid="n:all">${icon('check')}Alle als gelesen markieren</button>` : ''}
    </div></div>
    ${list.length ? `<ul class="vp-list">${rows}</ul>` : notice('', 'Keine Benachrichtigungen', 'Setze Serien oder Podcasts auf deine Merkliste oder fordere Titel an, dann erfährst du hier davon.')}</div>`,
  };
});
ACT.vpRead = (d) => { const n = S.notifications.find((x) => x.id === d.id); if (n && !n.read) { n.read = true; save(); } };
ACT.vpToggleRead = (d) => { const n = S.notifications.find((x) => x.id === d.id); if (!n) return; n.read = !n.read; toast(n.read ? 'Als gelesen markiert.' : 'Als ungelesen markiert.', 'ok', 2500); commit(); };
ACT.vpReadAll = () => { const p = curProfile(); S.notifications.forEach((n) => { if (n.profileId === p.id) n.read = true; }); toast('Alle Benachrichtigungen sind gelesen.', 'ok', 2500); commit(); };

/* ======================================================================================
   M-10 Benachrichtigungskanäle (shared with the administration via channelSettingsHtml)
   ====================================================================================== */
const VP_EVENTS = [['new_content', false, false], ['errors', true, false], ['invitations', false, false], ['announcements', false, false], ['wish_available', false, true], ['wish_status', false, true], ['wish_review', true, true]];
const VP_DEV_SEED = [['Firefox auf Linux', 12], ['Safari auf iPhone (installierte App)', 40], ['Chrome auf Android', 3]];
function vpDevices(ch) {
  if (Array.isArray(ch.list)) return ch.list;
  return Array.from({ length: ch.devices || 0 }, (_, i) => ({ id: 'd' + i, name: VP_DEV_SEED[i % 3][0], at: Date.now() - VP_DEV_SEED[i % 3][1] * D, self: false }));
}
function vpChState(c, ch) {
  if (c === 'telegram' && !ch.linked) return ['Nicht eingerichtet', 'outline'];
  return ch.enabled ? ['Aktiv', 'ok'] : ['Aus', 'outline'];
}
function channelSettingsHtml(opts = {}) {
  const acc = curAccount(), p = curProfile(), ai = S.accounts.indexOf(acc);
  const edit = p.primary, adminAcc = acc.role === 'admin', dis = edit ? '' : 'disabled';
  const base = `S.accounts.${ai}.channels`;
  const panel = (c, ic) => {
    const ch = acc.channels[c], st = vpChState(c, ch), devs = vpDevices(ch);
    const meta = c === 'email' ? esc(ch.address || acc.email) : c === 'push' ? vpPlural(devs.length, 'Gerät', 'Geräte') : ch.linked ? 'Verknüpft' : 'Nicht verknüpft';
    const evs = VP_EVENTS.filter(([, adminOnly]) => !adminOnly || adminAcc).map(([k, , isNew]) => `<label class="check"><input type="checkbox" data-act="vpEv" data-c="${c}" data-e="${k}" data-fid="ch:${c}:${k}" ${ch.events.includes(k) ? 'checked' : ''} ${dis}><span>${EVENT_LABEL[k]}${isNew ? ' ' + vpNew() : ''}</span></label>`).join('');
    let own = '';
    if (c === 'email') {
      own = `<div class="fld"><label for="vp-mail">E-Mail-Adresse</label><input class="inp" id="vp-mail" type="email" data-bind="${base}.email.address" data-fid="ch:email:addr" value="${esc(ch.address || '')}" placeholder="${esc(acc.email)}" ${dis}><span class="hint">Standard ist die Adresse deines Kontos.</span></div>`;
    } else if (c === 'telegram') {
      const tg = ui.vpTg;
      if (ch.linked) {
        own = `<div class="vp-kv"><b>Verknüpfung</b><span>Verknüpft mit Chat ${esc(ch.chat || '48213977')}</span></div><div class="acts"><button type="button" class="btn secondary" data-act="dialog" data-name="vpTgUnlink" data-fid="ch:tg:unlink" ${dis}>${icon('x')}Verknüpfung lösen</button></div>`;
      } else if (tg) {
        own = `<div class="vp-kv"><b>Verknüpfung</b><span class="chip warn">Code offen</span></div>
          <div class="vp-code"><span class="mono">${esc(tg.code)}</span><p>Sende dem Bot @slouchcat_bot die Nachricht <span class="mono">/start ${esc(tg.code)}</span></p><p class="muted">Der Code gilt bis ${fmtTime(tg.until)}. Wartet auf die Nachricht, Abfrage jede Sekunde.</p></div>
          <div class="acts"><button type="button" class="btn primary" data-act="vpTgCheck" data-fid="ch:tg:check" ${dis}>${icon('check')}Verknüpfung prüfen</button><button type="button" class="btn secondary" data-act="vpTgOpen" data-fid="ch:tg:open">${icon('external')}In Telegram öffnen</button><button type="button" class="btn secondary" data-act="vpTgLink" data-fid="ch:tg:renew" ${dis}>${icon('refresh')}Code erneuern</button><button type="button" class="btn ghost" data-act="dialog" data-name="vpTgDiscard" data-fid="ch:tg:discard" ${dis}>Code verwerfen</button></div>`;
      } else {
        own = `<div class="vp-kv"><b>Verknüpfung</b><span>Nicht verknüpft. Verbindet das Konto mit deinem Chat beim Bot.</span></div><div class="acts"><button type="button" class="btn secondary" data-act="vpTgLink" data-fid="ch:tg:link" ${dis}>${icon('link')}Verknüpfen</button></div>`;
      }
    } else {
      const self = devs.some((x) => x.self);
      own = `<div class="fld"><span class="lbl">Angemeldete Geräte</span>${devs.length ? `<ul class="vp-devs">${devs.map((x) => `<li>${icon(x.name.includes('iPhone') || x.name.includes('Android') ? 'phone' : 'monitor', 18)}<span class="grow"><b>${esc(x.name)}</b>${x.self ? ' <span class="chip info">Dieses Gerät</span>' : ''}<small>Angemeldet am ${fmtDate(x.at)}</small></span><button type="button" class="btn ghost sm" data-act="vpPushDel" data-id="${x.id}" aria-label="${esc(x.name)} entfernen" data-fid="ch:push:del:${x.id}" ${dis}>${icon('trash', 16)}Entfernen</button></li>`).join('')}</ul>` : '<p class="muted">Noch kein Gerät angemeldet.</p>'}</div>
        ${self ? '<p class="sec">Dieses Gerät erhält jetzt Push-Nachrichten.</p>' : `<div class="acts"><button type="button" class="btn secondary" data-act="vpPushAdd" data-fid="ch:push:add" ${dis}>${icon('bell')}Auf diesem Gerät aktivieren</button></div>`}
        <p class="hint muted">Auf iPhone und iPad gibt es Web-Push nur in der installierten App: im Teilen-Menü «Zum Home-Bildschirm» wählen und Slouchcat von dort öffnen.</p>`;
    }
    const body = `<div class="vp-chrow"><div><b>Aktiv</b><span>Gilt für das ganze Konto mit allen Profilen.</span></div>${sw(`${base}.${c}.enabled`, 'Aktiv: ' + CHANNEL_LABEL[c], { disabled: !edit })}</div>
      <fieldset class="vp-evs"><legend>Ereignisse</legend><span class="hint">${adminAcc ? 'Wofür eine Nachricht kommt. «Fehler» und «Wünsche: Freigabe nötig» gibt es nur für Administratoren.' : 'Wofür eine Nachricht kommt.'}</span><div class="vp-checks">${evs}</div></fieldset>
      ${own}
      <div class="acts"><button type="button" class="btn primary" data-act="vpChSave" data-fid="ch:${c}:save" ${dis}>Speichern</button><button type="button" class="btn secondary" data-act="vpTest" data-c="${c}" data-fid="ch:${c}:test" ${dis}>${icon('send')}Testnachricht</button></div>`;
    return band('vp-ch-' + c, CHANNEL_LABEL[c], `<span>${meta}</span><span class="chip ${st[1]}">${st[0]}</span>`, body, { viewer: !opts.admin, icon: ic });
  };
  const rule = notice('', 'Wünsche und Profile', 'Meldungen zu Wünschen eines Profils gehen an die Kanäle des Kontos, mit dem Profilnamen vorne. Bei Profilen mit PIN nennt die Nachricht den Titel nicht.');
  const intro = 'Neue Folgen der Serien und Podcasts auf deiner Merkliste, Meldungen zu deinen Wünschen, Einladungen und Ankündigungen kommen hier zusätzlich zum Eingang der App an. Die Einstellungen gelten für das ganze Konto mit allen Profilen; eine Nachricht zu einem Profil beginnt mit seinem Namen.';
  const inner = `${edit ? '' : vpOnlyPrimary('Nur im Hauptprofil', 'Die Kanäle gelten für das ganze Konto; ändern lassen sie sich im Hauptprofil. Wechsle dorthin, um Kanäle ein- oder auszuschalten.')}
    <div class="vp-chs">${panel('email', 'mail')}${panel('telegram', 'send')}${panel('push', 'bell')}</div>
    ${rule}
    ${isAdmin() ? `<p><a class="lnk" href="#/admin/integrations" data-fid="ch:srv">Kanäle des Servers einrichten → Integrationen</a></p>` : ''}`;
  if (opts.admin) return `<div class="vp-scope vp-ch stack">${inner}</div>`;
  return `<div class="page vp-scope vp-ch"><div class="head"><h1>Benachrichtigungskanäle</h1><p class="sub">${intro}</p><div class="acts"><button type="button" class="btn ghost" data-act="back" data-fid="ch:back">${icon('arrowL')}Zurück</button></div></div>${inner}</div>`;
}
route('/settings/notifications', () => ({ type: 'viewer', nav: 'settings', title: 'Benachrichtigungskanäle', html: channelSettingsHtml({ admin: false }) }));
ACT.vpEv = (d, el) => { const ch = curAccount().channels[d.c]; ch.events = el.checked ? [...new Set([...ch.events, d.e])] : ch.events.filter((x) => x !== d.e); commit(); };
ACT.vpChSave = () => { save(); toast('Gespeichert.', 'ok', 2500); };
ACT.vpTest = (d) => {
  const acc = curAccount(), ch = acc.channels[d.c];
  if (!ch.enabled) { toast(`${CHANNEL_LABEL[d.c]} ist aus. Schalte den Kanal zuerst ein.`, 'warn'); return; }
  if (d.c === 'telegram' && !ch.linked) { toast('Telegram ist noch nicht verknüpft.', 'warn'); return; }
  if (d.c === 'push' && !vpDevices(ch).length) { toast('Für Web-Push ist noch kein Gerät angemeldet.', 'warn'); return; }
  const target = d.c === 'email' ? esc(ch.address || acc.email) : d.c === 'push' ? vpPlural(vpDevices(ch).length, 'Gerät', 'Geräte') : 'den Telegram-Chat';
  S.deliveries.unshift({ at: Date.now(), accountId: acc.id, profileId: null, channel: d.c, event: 'test', subject: 'Testnachricht von Slouchcat', state: 'sent' });
  toast(`Testnachricht an ${target} gesendet.`, 'mail');
  commit();
};
let vpCodeN = 0;
ACT.vpTgLink = () => { ui.vpTg = { code: vpCodeN++ ? Math.random().toString(36).slice(2, 8).toUpperCase() : 'ABC123', until: Date.now() + 15 * 60e3 }; render(); };
ACT.vpTgOpen = () => toast('Im Mockup öffnet sich Telegram nicht. In der App führt der Knopf zum Chat mit @slouchcat_bot.', 'ok');
ACT.vpTgCheck = () => { const ch = curAccount().channels.telegram; ch.linked = true; ch.chat = '48213977'; ui.vpTg = null; toast('Telegram ist verknüpft.', 'ok'); commit(); };
DIALOGS.vpTgDiscard = () => ({ title: 'Code verwerfen?', body: '<p class="lead">Der offene Code gilt sofort nicht mehr. Neu verknüpfen geht jederzeit mit einem neuen Code.</p>', actions: `<button type="button" class="btn ghost" data-act="close" data-fid="tgd:x">Abbrechen</button><button type="button" class="btn primary" data-act="vpTgDiscardOk" data-fid="tgd:ok" data-autofocus>Code verwerfen</button>` });
ACT.vpTgDiscardOk = () => { ui.vpTg = null; closeDialog(); render(); };
DIALOGS.vpTgUnlink = () => ({ title: 'Verknüpfung lösen?', body: '<p class="lead">Telegram-Nachrichten gehen nicht mehr an den verknüpften Chat. Neu verknüpfen geht jederzeit mit einem neuen Code.</p>', actions: `<button type="button" class="btn ghost" data-act="close" data-fid="tgu:x">Abbrechen</button><button type="button" class="btn primary" data-act="vpTgUnlinkOk" data-fid="tgu:ok" data-autofocus>Verknüpfung lösen</button>` });
ACT.vpTgUnlinkOk = () => { const ch = curAccount().channels.telegram; ch.linked = false; ch.chat = ''; closeDialog(); toast('Die Verknüpfung ist gelöst.', 'ok'); commit(); };
ACT.vpPushDel = (d) => { const ch = curAccount().channels.push; ch.list = vpDevices(ch).filter((x) => x.id !== d.id); ch.devices = ch.list.length; toast('Das Gerät ist entfernt.', 'ok'); commit(); };
ACT.vpPushAdd = () => { const ch = curAccount().channels.push; ch.list = [...vpDevices(ch), { id: 'd' + ++S.seq, name: 'Chrome auf Windows', at: Date.now(), self: true }]; ch.devices = ch.list.length; toast('Dieses Gerät erhält jetzt Push-Nachrichten.', 'ok'); commit(); };

/* ======================================================================================
   M-11 Profile
   ====================================================================================== */
const VP_COLORS = seed().profiles.map((x) => [x.accent, x.on]); /* the palette of the app's profile colours */
const VP_LANG = [['de-CH', 'Deutsch (Schweiz)'], ['de-DE', 'Deutsch (Deutschland)'], ['en', 'English']];
const VP_META = [['', 'Wie Oberfläche'], ['de', 'Deutsch'], ['en', 'English']];
const VP_SUBRULE = [['forced', 'Nur erzwungene'], ['foreign', 'Bei fremder Tonsprache'], ['always', 'Immer'], ['never', 'Nie']];
const VP_QUAL = [['', 'Ohne Limit'], ['2160p', '4K'], ['1080p', '1080p'], ['720p', '720p'], ['480p', '480p']];
const VP_BW = [['', 'Ohne Limit'], ['40', '40 Mbit/s'], ['20', '20 Mbit/s'], ['8', '8 Mbit/s'], ['4', '4 Mbit/s']];
const VP_STREAMS = [['', 'Ohne Limit'], ['1', '1'], ['2', '2'], ['3', '3'], ['4', '4']];
const vpT = (list, v) => (list.find((x) => String(x[0]) === String(v ?? '')) || list[0])[1];
const vpLibNames = (x) => (x.libs && x.libs.length ? x.libs.map((i) => LIBS[i][0]) : null);
function vpProfData(x) {
  return { lang: x.lang ?? 'de-CH', metaLang: x.metaLang ?? '', libs: x.libs ?? null, audio: x.audio ?? 'Deutsch, Englisch', subRule: x.subRule ?? 'forced', subLangs: x.subLangs ?? 'Deutsch', quality: x.quality ?? '', bandwidth: x.bandwidth ?? '', streams: x.streams ?? '', watchLimit: x.watchLimit ?? (x.maxRating <= 6 ? 90 : '') };
}
function vpReport(x) {
  const seedN = [...x.id].reduce((a, c) => a + c.charCodeAt(0), 0);
  const days = Array.from({ length: 30 }, (_, i) => ((seedN * (i + 7) * 37) % 150) * (i % 7 === 5 ? 0.3 : 1) | 0);
  const max = Math.max(...days, 1), total = days.reduce((a, b) => a + b, 0), lim = vpProfData(x).watchLimit;
  return `<div class="vp-rep"><b>Schauzeit von ${esc(x.name)}</b><p class="muted">Minuten pro Tag in den letzten 30 Tagen.</p>
    <div class="vp-bars" role="img" aria-label="Schauzeit pro Tag, heute ${days[29]} min">${days.map((m) => `<span style="height:${Math.max(2, (m / max) * 100)}%"></span>`).join('')}</div>
    <p class="vp-repl"><span>Heute: ${days[29]} min</span><span>Total: ${total} min</span><span>${lim ? `Limit: ${lim} min pro Tag` : 'Limit: keines'}</span></p></div>`;
}
function vpProfCard(x) {
  const acc = account(x.accountId), dt = vpProfData(x), su = seerrUserFor(x);
  const chips = [];
  if (x.primary) chips.push('<span class="chip outline">Hauptprofil</span>');
  chips.push(`<span class="chip info">${ageLabel(x.maxRating)}${x.maxRating > acc.maxRating ? ` · Konto höchstens ${ageLabel(acc.maxRating)}` : ''}</span>`);
  chips.push(`<span class="chip info">${vpT(VP_LANG, dt.lang)}</span>`);
  chips.push(`<span class="chip info">Sprache der Beschreibungen: ${vpT(VP_META, dt.metaLang)}</span>`);
  if (x.pin) chips.push(`<span class="chip info">${icon('lock', 12)}PIN</span>`);
  if (dt.quality) chips.push(`<span class="chip info">Höchstens ${vpT(VP_QUAL, dt.quality)}</span>`);
  if (dt.bandwidth) chips.push(`<span class="chip info">${vpT(VP_BW, dt.bandwidth)}</span>`);
  if (dt.streams) chips.push(`<span class="chip info">${dt.streams} ${dt.streams === '1' ? 'Stream' : 'Streams'}</span>`);
  if (dt.watchLimit) chips.push(`<span class="chip info">${dt.watchLimit} min pro Tag</span>`);
  chips.push(x.canRequest && acc.canRequest ? `<span class="chip ok">${icon('wish', 12)}Darf anfordern</span>` : `<span class="chip outline">Darf nicht anfordern</span>`);
  const send = su ? `Sendet als ${esc(seerrUser(su.id).name)} (${su.via === 'profile' ? 'eigener Seerr-Benutzer' : su.via === 'account' ? 'Benutzer des Kontos' : 'Sammelbenutzer'})` : 'Kein Seerr-Benutzer';
  const mid = 'vp-menu:' + x.id, rid = 'vp-rep:' + x.id, menu = isOpen(mid);
  return `<li class="vp-card">
    <div class="vp-ph"><span class="av lg" style="background:${esc(x.accent)};color:${esc(x.on)}">${esc(x.name[0])}</span>
      <div class="grow"><h2>${esc(x.name)}</h2><div class="vp-chips">${chips.join('')}</div></div></div>
    <p class="vp-line">${icon('folder', 16)}Bibliotheken: ${vpLibNames(dt) ? esc(vpLibNames(dt).join(', ')) : 'Alle'}</p>
    <p class="vp-line vp-newl">${icon('user', 16)}${send} ${vpNew()}</p>
    <div class="acts">
      <button type="button" class="btn secondary sm" data-act="vpProfEdit" data-id="${x.id}" data-fid="pf:edit:${x.id}">${icon('pen', 16)}Bearbeiten</button>
      <button type="button" class="btn ghost sm" data-act="open" data-id="${rid}" aria-expanded="${isOpen(rid)}" data-fid="pf:rep:${x.id}">${icon('chart', 16)}${isOpen(rid) ? 'Schauzeit ausblenden' : 'Schauzeit anzeigen'}</button>
      <span class="vp-mw"><button type="button" class="btn ghost icon sm" data-act="open" data-id="${mid}" aria-expanded="${menu}" aria-haspopup="true" aria-label="Weitere Aktionen für ${esc(x.name)}" data-fid="pf:menu:${x.id}">${icon('dots', 16)}</button>
      ${menu ? `<span class="vp-menu" role="menu"><button type="button" role="menuitem" data-act="vpPinSet" data-id="${x.id}" data-fid="pf:pin:${x.id}">${icon('lock', 16)}${x.pin ? 'PIN ändern' : 'PIN setzen'}</button>${x.pin ? `<button type="button" role="menuitem" data-act="vpMenuDlg" data-name="vpPinDel" data-id="${x.id}" data-fid="pf:pindel:${x.id}">${icon('x', 16)}PIN entfernen</button>` : ''}${x.primary ? '' : `<button type="button" role="menuitem" class="vp-dq" data-act="vpMenuDlg" data-name="vpProfDel" data-id="${x.id}" data-fid="pf:del:${x.id}">${icon('trash', 16)}Löschen</button>`}</span>` : ''}</span>
    </div>
    ${isOpen(rid) ? vpReport(x) : ''}
  </li>`;
}
route('/settings/profiles', () => {
  const p = curProfile();
  const intro = 'Jede Person im Haushalt bekommt ein eigenes Profil mit Altersfreigabe, Sprache, Farbe und Bibliotheken.';
  if (!p.primary) return { type: 'viewer', nav: 'profiles', title: 'Profile', html: `<div class="page vp-scope"><div class="head"><h1>Profile</h1><p class="sub">${intro}</p></div>${vpOnlyPrimary('Nur im Hauptprofil', 'Profile verwaltest du im Hauptprofil dieses Kontos. Wechsle dorthin, um Profile anzulegen oder zu ändern.')}</div>` };
  return {
    type: 'viewer', nav: 'profiles', title: 'Profile',
    html: `<div class="page vp-scope"><div class="head"><h1>Profile</h1><p class="sub">${intro}</p><div class="acts"><button type="button" class="btn primary" data-act="vpProfNew" data-fid="pf:new">${icon('plus')}Profil hinzufügen</button></div></div>
    <ul class="vp-cards" aria-label="Profile dieses Kontos">${profilesOf(p.accountId).map(vpProfCard).join('')}</ul></div>`,
  };
});
ACT.vpMenuDlg = (d) => { ui.open['vp-menu:' + d.id] = false; openDialog(d.name, { id: d.id }); ui.dialog.opener = 'pf:menu:' + d.id; render(); };
function vpDraftFrom(x) {
  const dt = vpProfData(x);
  return { id: x.id, name: x.name, maxRating: String(x.maxRating), accent: x.accent, on: x.on, canRequest: !!x.canRequest, pin: '', ...dt, libsAll: !dt.libs, libsPick: Object.fromEntries((dt.libs || []).map((i) => [i, true])), streams: String(dt.streams), watchLimit: dt.watchLimit, err: '' };
}
ACT.vpProfNew = () => {
  const acc = curAccount(), c = VP_COLORS[profilesOf(acc.id).length % VP_COLORS.length];
  ui.open['vp-pf-play'] = ui.open['vp-pf-lim'] = false;
  ui.draft.prof = { ...vpDraftFrom({ id: null, name: '', maxRating: Math.min(12, acc.maxRating), accent: c[0], on: c[1], canRequest: false }), watchLimit: '' };
  openDialog('vpProf');
};
ACT.vpProfEdit = (d) => { ui.open['vp-pf-play'] = ui.open['vp-pf-lim'] = false; ui.draft.prof = vpDraftFrom(profile(d.id)); openDialog('vpProf'); };
ACT.vpColor = (d) => { const c = VP_COLORS[Number(d.i)]; ui.draft.prof.accent = c[0]; ui.draft.prof.on = c[1]; renderOverlays(); };
DIALOGS.vpProf = () => {
  const dr = ui.draft.prof, acc = curAccount(), isNew = !dr.id, P = 'ui.draft.prof';
  const opt = (list) => list.map(([v, t]) => ({ v, t }));
  const playSum = `${esc(dr.audio || 'Keine Tonsprache')} · Untertitel: ${vpT(VP_SUBRULE, dr.subRule)}`;
  const limSum = [dr.quality ? vpT(VP_QUAL, dr.quality) : '', dr.bandwidth ? vpT(VP_BW, dr.bandwidth) : '', dr.streams ? dr.streams + ' Streams' : '', dr.watchLimit ? dr.watchLimit + ' min pro Tag' : ''].filter(Boolean).join(' · ') || 'Ohne Limit';
  const play = `<div class="fld"><label for="vp-pf-audio">Tonsprachen</label><input class="inp" id="vp-pf-audio" data-bind="${P}.audio" data-fid="pfd:audio" value="${esc(dr.audio)}"><span class="hint">In Rangfolge, durch Komma getrennt.</span></div>
    <div class="fld"><label>Untertitel</label>${selectEl(P + '.subRule', opt(VP_SUBRULE), { label: 'Untertitel' })}</div>
    <div class="fld"><label for="vp-pf-subl">Untertitelsprachen</label><input class="inp" id="vp-pf-subl" data-bind="${P}.subLangs" data-fid="pfd:subl" value="${esc(dr.subLangs)}"></div>`;
  const lim = `<div class="fld"><label>Höchste Qualität</label>${selectEl(P + '.quality', opt(VP_QUAL), { label: 'Höchste Qualität' })}</div>
    <div class="fld"><label>Bandbreite</label>${selectEl(P + '.bandwidth', opt(VP_BW), { label: 'Bandbreite' })}</div>
    <div class="fld"><label>Gleichzeitige Streams</label>${selectEl(P + '.streams', opt(VP_STREAMS), { label: 'Gleichzeitige Streams' })}</div>
    <div class="fld"><label for="vp-pf-wl">Schauzeit pro Tag (Minuten)</label><input class="inp" id="vp-pf-wl" type="number" min="0" inputmode="numeric" data-bind="${P}.watchLimit" data-num data-fid="pfd:wl" value="${esc(dr.watchLimit)}" placeholder="Ohne Limit"></div>`;
  const body = `<div class="vp-form">
    ${dr.err ? `<p class="err" role="alert">${esc(dr.err)}</p>` : ''}
    <div class="fld"><label for="vp-pf-name">Name</label><input class="inp" id="vp-pf-name" maxlength="60" data-bind="${P}.name" data-fid="pfd:name" value="${esc(dr.name)}" ${dr.err && !dr.name.trim() ? 'aria-invalid="true"' : ''} data-autofocus></div>
    <div class="fld"><label>Altersfreigabe</label>${selectEl(P + '.maxRating', VP_AGES.map((a) => ({ v: String(a), t: ageLabel(a), disabled: a > acc.maxRating })), { label: 'Altersfreigabe' })}<span class="hint">Titel mit höherer Freigabe bleiben für dieses Profil unsichtbar. Das Konto erlaubt höchstens ${ageLabel(acc.maxRating)}.</span></div>
    <div class="fld"><label>Sprache</label>${selectEl(P + '.lang', opt(VP_LANG), { label: 'Sprache' })}</div>
    <div class="fld"><label>Sprache der Beschreibungen</label>${selectEl(P + '.metaLang', opt(VP_META), { label: 'Sprache der Beschreibungen' })}<span class="hint">Titel, Beschreibungen und Genres; die Oberfläche folgt der Sprache oben.</span></div>
    <div class="fld"><span class="lbl" id="vp-pf-col">Farbe</span><div class="vp-swatches" role="group" aria-labelledby="vp-pf-col">${VP_COLORS.map(([a], i) => `<button type="button" class="vp-swc" style="background:${esc(a)}" aria-pressed="${dr.accent === a}" aria-label="Farbe ${i + 1}" data-act="vpColor" data-i="${i}" data-fid="pfd:col:${i}">${dr.accent === a ? icon('check', 16) : ''}</button>`).join('')}</div></div>
    <div class="fld"><span class="lbl" id="vp-pf-lib">Bibliotheken</span><div class="seg" role="group" aria-labelledby="vp-pf-lib"><button type="button" aria-pressed="${dr.libsAll}" data-act="set" data-path="${P}.libsAll" data-value="true" data-fid="pfd:liball">Alle</button><button type="button" aria-pressed="${!dr.libsAll}" data-act="set" data-path="${P}.libsAll" data-value="false" data-fid="pfd:libpick">Auswahl</button></div>
      ${dr.libsAll ? '' : `<div class="vp-checks">${LIBS.map(([l], i) => `<label class="check"><input type="checkbox" data-bind="${P}.libsPick.${i}" data-fid="pfd:lib:${i}" ${dr.libsPick[i] ? 'checked' : ''}><span>${esc(l)}</span></label>`).join('')}</div>`}</div>
    ${band('vp-pf-play', 'Wiedergabe', `<span>${playSum}</span>`, play, { viewer: true })}
    ${band('vp-pf-lim', 'Wiedergabe-Limits', `<span>${esc(limSum)}</span>`, lim, { viewer: true })}
    <fieldset class="vp-wishset"><legend>Wünsche ${vpNew()}</legend>
      <div class="vp-chrow"><div><b>Darf Titel anfordern</b><span>Fordert fehlende Filme und Serien an. Was freigegeben wird, entscheidet Seerr.</span></div>${sw(P + '.canRequest', 'Darf Titel anfordern', { disabled: !acc.canRequest })}</div>
      ${acc.canRequest ? '' : '<p class="sec vp-lock">' + icon('lock', 16) + 'Das Konto darf nicht anfordern. Das stellt der Administrator ein.</p>'}
    </fieldset>
    ${isNew ? `<div class="fld"><label for="vp-pf-pin">PIN (optional)</label><input class="inp" id="vp-pf-pin" type="password" inputmode="numeric" autocomplete="new-password" maxlength="8" data-bind="${P}.pin" data-fid="pfd:pin" value="${esc(dr.pin)}"><span class="hint">4 bis 8 Ziffern. Ohne die PIN lässt sich das Profil nicht öffnen.</span></div>` : ''}
  </div>`;
  return { title: isNew ? 'Neues Profil' : 'Profil bearbeiten', wide: true, body, actions: `<button type="button" class="btn ghost" data-act="close" data-fid="pfd:x">Abbrechen</button><button type="button" class="btn primary" data-act="vpProfSave" data-fid="pfd:ok">${isNew ? 'Profil anlegen' : 'Speichern'}</button>` };
};
ACT.vpProfSave = () => {
  const dr = ui.draft.prof, acc = curAccount();
  if (!dr.name.trim()) { dr.err = 'Gib einen Namen ein.'; renderOverlays(); return; }
  if (dr.pin && !/^\d{4,8}$/.test(dr.pin)) { dr.err = 'Die PIN braucht 4 bis 8 Ziffern.'; renderOverlays(); return; }
  const libs = dr.libsAll ? null : Object.keys(dr.libsPick).filter((k) => dr.libsPick[k]).map(Number);
  const fields = { name: dr.name.trim().slice(0, 60), maxRating: Math.min(Number(dr.maxRating), acc.maxRating), accent: dr.accent, on: dr.on, canRequest: acc.canRequest ? !!dr.canRequest : false, lang: dr.lang, metaLang: dr.metaLang, libs: libs && libs.length ? libs : null, audio: dr.audio, subRule: dr.subRule, subLangs: dr.subLangs, quality: dr.quality, bandwidth: dr.bandwidth, streams: dr.streams, watchLimit: dr.watchLimit };
  let x;
  if (dr.id) { x = profile(dr.id); Object.assign(x, fields); }
  else { x = { id: 'p' + ++S.seq, accountId: acc.id, primary: false, pin: !!dr.pin, ...fields }; S.profiles.push(x); }
  if (x.id === curProfile().id) applyAccent();
  closeDialog();
  toast(dr.id ? `«${esc(x.name)}» ist gespeichert.` : `Profil «${esc(x.name)}» ist angelegt.`, 'ok');
  commit();
};
ACT.vpPinSet = (d) => { ui.open['vp-menu:' + d.id] = false; ui.draft.pin = { id: d.id, v: '', err: '' }; openDialog('vpPin', { id: d.id }); ui.dialog.opener = 'pf:menu:' + d.id; render(); };
DIALOGS.vpPin = ({ id }) => {
  const x = profile(id), dr = ui.draft.pin;
  return { title: `PIN für ${esc(x.name)}`, body: `${dr.err ? `<p class="err" role="alert">${esc(dr.err)}</p>` : ''}<div class="fld"><label for="vp-pin-new">Neue PIN</label><input class="inp" id="vp-pin-new" type="password" inputmode="numeric" autocomplete="new-password" maxlength="8" data-bind="ui.draft.pin.v" data-enter="vpPinSave" data-fid="pin:v" value="${esc(dr.v)}" data-autofocus><span class="hint">4 bis 8 Ziffern. Ohne die PIN lässt sich das Profil nicht öffnen.</span></div>`, actions: `<button type="button" class="btn ghost" data-act="close" data-fid="pin:x">Abbrechen</button><button type="button" class="btn primary" data-act="vpPinSave" data-fid="pin:ok">PIN speichern</button>` };
};
ACT.vpPinSave = () => { const dr = ui.draft.pin; if (!/^\d{4,8}$/.test(dr.v || '')) { dr.err = 'Die PIN braucht 4 bis 8 Ziffern.'; renderOverlays(); return; } const x = profile(dr.id); x.pin = true; closeDialog(); toast(`Die PIN für «${esc(x.name)}» ist gespeichert.`, 'ok'); commit(); };
DIALOGS.vpPinDel = ({ id }) => { const x = profile(id); return { title: `PIN von ${esc(x.name)} entfernen?`, body: '<p class="lead">Danach kann jeder dieses Profil ohne PIN öffnen.</p>', actions: `<button type="button" class="btn ghost" data-act="close" data-fid="pind:x">Abbrechen</button><button type="button" class="btn primary" data-act="vpPinDelOk" data-id="${id}" data-fid="pind:ok" data-autofocus>PIN entfernen</button>` }; };
ACT.vpPinDelOk = (d) => { const x = profile(d.id); x.pin = false; closeDialog(); toast(`Die PIN von «${esc(x.name)}» ist entfernt.`, 'ok'); commit(); };
DIALOGS.vpProfDel = ({ id }) => { const x = profile(id); return { title: `Profil «${esc(x.name)}» löschen?`, body: '<p class="lead">Fortschritt und Einstellungen dieses Profils gehen verloren. Das lässt sich nicht rückgängig machen.</p>', actions: `<button type="button" class="btn ghost" data-act="close" data-fid="pdel:x">Abbrechen</button><button type="button" class="btn primary" data-act="vpProfDelOk" data-id="${id}" data-fid="pdel:ok" data-autofocus>${icon('trash')}Profil löschen</button>` }; };
ACT.vpProfDelOk = (d) => {
  const x = profile(d.id);
  closeDialog();
  if (!x || x.primary || x.id === curProfile().id) { toast('Das aktive Profil und das Hauptprofil lassen sich nicht löschen.', 'warn'); return; }
  /* its wishes go to the main profile of the account, so nobody loses a wish and somebody still hears about it */
  const main = profilesOf(x.accountId).find((y) => y.primary);
  let moved = 0;
  S.wishes.forEach((w) => {
    if (w.owner === x.id) { w.owner = main.id; w.joined = w.joined.filter((j) => j !== main.id); ev(w, 'handover', `Von gelöschtem Profil ${x.name} übernommen: ${main.name}`); moved++; }
    else if (w.joined.includes(x.id)) w.joined = w.joined.filter((j) => j !== x.id);
  });
  delete S.map.profiles[x.id];
  S.profiles = S.profiles.filter((y) => y.id !== x.id);
  S.notifications = S.notifications.filter((n) => n.profileId !== x.id);
  toast(`Profil «${esc(x.name)}» ist gelöscht.${moved ? ` ${moved} ${moved === 1 ? 'Wunsch gehört' : 'Wünsche gehören'} jetzt ${esc(main.name)}.` : ''}`, 'ok');
  commit();
};

/* ======================================================================================
   M-12 Profilwahl «Wer schaut?»
   ====================================================================================== */
function vpTile(x, small) {
  const active = x.id === curProfile().id;
  const marks = [x.primary && active ? 'Hauptprofil · aktiv' : active ? 'aktiv' : x.primary ? 'Hauptprofil' : '', x.pin ? 'mit PIN geschützt' : ''].filter(Boolean);
  return `<li><button type="button" class="${cls('vp-tile', small && 'sm', active && 'on')}" data-act="vpPick" data-id="${x.id}" aria-current="${active}" data-fid="pk:${x.id}">
    <span class="av lg" style="background:${esc(x.accent)};color:${esc(x.on)}">${esc(x.name[0])}${x.pin ? `<span class="vp-lk">${icon('lock', 14)}</span>` : ''}</span>
    <b>${esc(x.name)}</b>${marks.map((m) => `<span class="vp-mk">${m}</span>`).join('')}</button></li>`;
}
route('/profiles', () => {
  const acc = curAccount();
  const others = S.accounts.filter((a) => a.id !== acc.id);
  return {
    type: 'bare', title: 'Wer schaut?',
    html: `<div class="vp-scope vp-pick">
      <header class="vp-bh"><a class="vp-brand" href="#/" data-fid="pk:brand">${LOGO}<span>Slouchcat</span></a></header>
      <main class="vp-pm"><h1>Wer schaut?</h1><p class="sub">Wähle dein Profil.</p>
        <ul class="vp-tiles" aria-label="Profile">${profilesOf(acc.id).map((x) => vpTile(x, false)).join('')}</ul>
        <div class="acts"><a class="btn secondary" href="#/settings/profiles" data-fid="pk:manage">${icon('settings')}Profile verwalten</a></div>
        <section class="vp-mockhelp" aria-labelledby="vp-oa"><h2 id="vp-oa">Andere Konten (nur im Mockup)</h2><p>Im echten Slouchcat meldet sich jedes Konto selbst an. Hier wechselst du direkt, um andere Rechte auszuprobieren.</p>
          ${others.map((a) => `<div class="vp-oacc"><b>${esc(a.email)}</b><span>${a.canRequest ? 'Konto darf anfordern' : 'Konto darf nicht anfordern'}</span><ul class="vp-tiles">${profilesOf(a.id).map((x) => vpTile(x, true)).join('')}</ul></div>`).join('')}
        </section>
      </main></div>`,
  };
});
ACT.vpPick = (d) => {
  const x = profile(d.id);
  if (x.pin && x.id !== curProfile().id) { ui.draft.pinOpen = { id: x.id, v: '', err: '' }; openDialog('vpPinOpen', { id: x.id }); return; }
  vpEnter(x);
};
function vpEnter(x) { switchProfile(x.id); toast(`Aktives Profil: <b>${esc(x.name)}</b>`, 'ok', 2500); go('#/'); }
DIALOGS.vpPinOpen = ({ id }) => {
  const x = profile(id), dr = ui.draft.pinOpen;
  return { title: `PIN für ${esc(x.name)}`, body: `<p class="lead">Dieses Profil ist mit einer PIN geschützt.</p>${dr.err ? `<p class="err" role="alert">${esc(dr.err)}</p>` : ''}<div class="fld"><label for="vp-pin-open">PIN</label><input class="inp" id="vp-pin-open" type="password" inputmode="numeric" autocomplete="off" maxlength="8" data-bind="ui.draft.pinOpen.v" data-enter="vpPinOpenOk" data-fid="pino:v" value="${esc(dr.v)}" ${dr.err ? 'aria-invalid="true"' : ''} data-autofocus><span class="hint">4 bis 8 Ziffern.</span></div><p class="vp-mocknote">Nur im Mockup: Jede vierstellige PIN öffnet das Profil.</p>`, actions: `<button type="button" class="btn ghost" data-act="close" data-fid="pino:x">Abbrechen</button><button type="button" class="btn primary" data-act="vpPinOpenOk" data-fid="pino:ok">Öffnen</button>` };
};
ACT.vpPinOpenOk = () => { const dr = ui.draft.pinOpen; if (!/^\d{4}$/.test(dr.v || '')) { dr.err = 'Die PIN stimmt nicht. Versuche es noch einmal.'; dr.v = ''; renderOverlays(); return; } closeDialog(); vpEnter(profile(dr.id)); };
