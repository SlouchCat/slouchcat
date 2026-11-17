/* Fassung 3 of the concept: the Seerr user of an account (created on invitation, same login), the limit per
   account (Seerr counts), the profile tag in Radarr and Sonarr, and the titles of the pages «Entdecken»,
   «Filme», «Serien», «Suche». Simulated like everything else in this mockup. */
'use strict';

/* ---------- profile tag in Radarr and Sonarr ---------- */
function tagSlug(s) { return String(s).toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/&/g, ' und ').replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, ''); }
/* label of a profile: prefix + name; the account name is added only if two profiles would get the same label */
function tagLabel(p) {
  if (p.tagName) return p.tagName;
  const base = tagSlug(p.name);
  /* tags are never renamed: the first profile keeps the plain label, a later one with the same name gets the account name */
  const twice = S.profiles.slice(0, S.profiles.indexOf(p)).some((x) => !x.tagName && tagSlug(x.name) === base);
  return S.settings.wishes.tags.prefix + base + (twice ? '-' + tagSlug(account(p.accountId).email.split('@')[0]) : '');
}
function wishPeople(w) { return [w.owner, ...w.joined].filter(Boolean); }
function arrOf(w) { return w.media === 'movie' ? S.arr.radarr : S.arr.sonarr; }
/* Slouchcat labels the title once Seerr has passed it on; a joined profile gets its tag as well. Nothing else
   is ever changed in Radarr or Sonarr. A tag that cannot be set does not touch the wish; the next sync tries again. */
function applyTags(w) {
  const cfg = S.settings.wishes.tags;
  /* a request that was placed directly in Seerr has no profile of its own: no profile tag, even if it was assigned later */
  if (!cfg || !cfg.enabled || w.origin === 'jellyseerr' || !w.seerrRequestId || !['processing', 'partial', 'available'].includes(w.state)) return;
  w.tagged = w.tagged || {};
  const arr = arrOf(w);
  wishPeople(w).forEach((pid) => {
    const p = profile(pid);
    if (!p || w.tagged[pid] === 'set') return;
    if (arr.condition !== 'ok') {
      if (w.tagged[pid] !== 'fail') ev(w, 'tag_failed', `Tag «${tagLabel(p)}» nicht gesetzt: ${arr.name} ist nicht erreichbar`);
      w.tagged[pid] = 'fail';
      return;
    }
    w.tagged[pid] = 'set';
    ev(w, 'tagged', `Tag «${tagLabel(p)}» in ${arr.name} gesetzt`);
  });
}
/* what the administration shows per wish: [{ profile, label, state: 'set'|'fail'|'wait' }] */
function wishTags(w) {
  if (!S.settings.wishes.tags.enabled || w.origin === 'jellyseerr') return [];
  return wishPeople(w).map((pid) => profile(pid)).filter(Boolean).map((p) => ({ profile: p, label: tagLabel(p), state: (w.tagged || {})[p.id] || 'wait' }));
}
const TAG_STATE = { set: 'gesetzt', fail: 'fehlgeschlagen', wait: 'noch nicht gesetzt' };
function tagChips(w) {
  return wishTags(w).map((x) => `<span class="chip ${x.state === 'set' ? 'outline' : x.state === 'fail' ? 'bad' : 'info'}" title="${x.state === 'set' ? 'Tag gesetzt' : x.state === 'fail' ? arrOf(w).name + ' war nicht erreichbar, der nächste Abgleich versucht es wieder' : 'Tag folgt, sobald der Titel in ' + arrOf(w).name + ' steht'}">${icon('pin', 12)}${esc(x.label)}${x.state === 'set' ? '' : ' · ' + TAG_STATE[x.state]}</span>`).join('');
}

/* ---------- limit per account: Slouchcat sets it, Seerr counts ---------- */
function wishUnits(w) { return w.media === 'movie' ? 1 : (w.seasons.length || airedSeasons(T[w.titleKey]).length || 1); }
function linkedAccount(u) { const e = Object.entries(S.map.accounts).find(([, id]) => id === u.id); return e ? account(e[0]) : null; }
/* effective limit of an account: { movie: { n, days }, tv: { n, days } } or null for «unbegrenzt» */
function limitOf(acc) {
  const l = acc.limit || { mode: 'default' };
  if (l.mode === 'unlimited') return null;
  if (l.mode === 'custom') return { movie: l.movie, tv: l.tv };
  return S.settings.wishes.limit;
}
/* which limit Seerr applies to this user: administrators have none, an account's user has the limit Slouchcat
   wrote, any other user keeps what is set in Seerr itself */
function quotaCfg(u) {
  if (u.admin) return { cfg: null, source: 'admin' };
  const acc = linkedAccount(u);
  if (acc) return { cfg: limitOf(acc), source: 'account', account: acc };
  return { cfg: u.quota || null, source: 'seerr' };
}
function quotaEntries(u, kind, except) {
  const media = kind === 'movie' ? 'movie' : 'show';
  const own = S.wishes.filter((x) => x !== except && x.seerrUser === u.id && x.media === media && !['declined', 'withdrawn'].includes(x.state) && (x.seerrRequestId || x.state === 'queued'))
    .map((x) => ({ at: x.created, n: wishUnits(x), wish: x }));
  const direct = (u.direct || []).filter((d) => d.media === media).map((d) => ({ at: d.at, n: d.n, direct: true, title: d.title }));
  return [...own, ...direct];
}
function kindQuota(u, kind, c, except) {
  if (!c || !c.n) return { limit: 0, days: c ? c.days : 0, used: 0, remaining: Infinity, restricted: false, nextFree: null, entries: [] };
  const since = Date.now() - c.days * D;
  const entries = quotaEntries(u, kind, except).filter((e) => e.at > since).sort((a, b) => a.at - b.at);
  const used = entries.reduce((s, e) => s + e.n, 0);
  const remaining = Math.max(0, c.n - used);
  return { limit: c.n, days: c.days, used, remaining, restricted: remaining <= 0, nextFree: entries.length ? entries[0].at + c.days * D : null, entries, direct: entries.filter((e) => e.direct).reduce((s, e) => s + e.n, 0) };
}
/* like GET /user/{id}/quota, plus wishes that still wait to be sent */
function quotaFor(u, except) {
  const { cfg, source, account: acc } = quotaCfg(u);
  const movie = kindQuota(u, 'movie', cfg && cfg.movie, except), tv = kindQuota(u, 'tv', cfg && cfg.tv, except);
  return { source, account: acc || null, unlimited: !movie.limit && !tv.limit, movie, tv };
}
function quotaOfProfile(p) { const su = seerrUserFor(p); const u = su && seerrUser(su.id); return u && !u.gone ? quotaFor(u) : null; }
function whenFree(ts) { return ts ? `am ${new Date(ts).toLocaleDateString('de-CH', { weekday: 'long', day: 'numeric', month: 'long' })} um ${fmtTime(ts)}` : 'bald'; }
const unitWord = (kind, n) => (kind === 'movie' ? (n === 1 ? 'Film' : 'Filme') : n === 1 ? 'Staffel' : 'Staffeln');
/* may this profile order `units` more of this title now? A joined wish costs nothing and is never checked here. */
function limitCheck(p, t, units = 1) {
  const q = quotaOfProfile(p);
  const k = t.kind === 'movie' ? 'movie' : 'tv';
  if (!q || !q[k].limit) return { ok: true, q, k, none: true };
  const x = q[k];
  if (x.remaining >= units) return { ok: true, q, k };
  const text = x.remaining === 0
    ? `Das Limit ist erreicht: ${x.limit} ${unitWord(k, x.limit)} in ${x.days} Tagen. Der nächste Platz wird ${whenFree(x.nextFree)} frei.`
    : `Noch ${x.remaining} von ${x.limit} Staffeln frei. Wähle höchstens ${x.remaining === 1 ? 'eine Staffel' : x.remaining + ' Staffeln'}.`;
  return { ok: false, q, k, text };
}
/* «7 von 10 Filmen frei» for one kind */
function quotaText(x, kind) { return x.limit ? `${x.remaining} von ${x.limit} ${kind === 'movie' ? 'Filmen' : 'Staffeln'} frei` : `${kind === 'movie' ? 'Filme' : 'Staffeln'} ohne Limit`; }
function quotaMeter(x, kind) {
  if (!x.limit) return `<span class="qm none"><b>${kind === 'movie' ? 'Filme' : 'Staffeln'}</b><span>ohne Limit</span></span>`;
  const pct = Math.min(100, Math.round((x.used / x.limit) * 100));
  return `<span class="qm ${x.restricted ? 'full' : ''}"><b>${kind === 'movie' ? 'Filme' : 'Staffeln'}</b><span class="bar" role="img" aria-label="${x.used} von ${x.limit} verbraucht"><i style="width:${pct}%"></i></span><span>${x.used} / ${x.limit} in ${x.days} Tagen</span></span>`;
}
/* the account's limit as the viewer app shows it on the pages under «Wünsche» */
function quotaPill(p) {
  const q = quotaOfProfile(p);
  if (!q) return '';
  if (q.unlimited) return `<div class="quota" aria-label="Limit"><span class="qm none"><b>Limit</b><span>${q.source === 'admin' ? 'keines (Administrator)' : 'keines'}</span></span></div>`;
  return `<div class="quota" aria-label="Limit dieses Kontos">${quotaMeter(q.movie, 'movie')}${quotaMeter(q.tv, 'tv')}</div>`;
}

/* ---------- the Seerr user of an account ---------- */
const SEERR_ACC = {
  created: { label: 'Von Slouchcat eröffnet', chip: 'ok' },
  linked: { label: 'Verknüpft über gleiche E-Mail', chip: 'ok' },
  manual: { label: 'Von Hand zugeordnet', chip: 'ok' },
  waiting: { label: 'Wird eröffnet, sobald Seerr erreichbar ist', chip: 'warn' },
  none: { label: 'Nicht eröffnet', chip: 'outline' },
};
const PW_STATE = {
  synced: { label: 'Gleich wie in Slouchcat', chip: 'ok' },
  next_login: { label: 'Wird bei der nächsten Anmeldung in Slouchcat übernommen', chip: 'info' },
  off: { label: 'Wird nicht abgeglichen', chip: 'outline' },
  admin: { label: 'Bleibt in Seerr (Administrator)', chip: 'outline' },
  external: { label: 'Anmeldung über Jellyfin oder Plex', chip: 'outline' },
};
/* state of an account in Seerr: { state, user, pw, info, pwInfo } */
function accSeerr(acc) {
  const id = S.map.accounts[acc.id];
  const u = id ? seerrUser(id) : null;
  let st = acc.seerr || { state: 'none' };
  if (u && ['none', 'waiting'].includes(st.state)) st = { state: 'manual', pw: 'off' };
  if (!u && !['none', 'waiting'].includes(st.state)) st = { state: 'none' };
  return { ...st, user: u, info: SEERR_ACC[st.state], pwInfo: u ? PW_STATE[st.pw || 'off'] : null };
}
/* a Seerr user belongs to one account at most: free means no account uses it and no profile sends as it */
function seerrFree(u) { return !u.gone && !Object.values(S.map.accounts).includes(u.id) && !Object.values(S.map.profiles).includes(u.id); }
/* may this account take the user? Its own profiles may already send as it (exception). */
function seerrFreeFor(u, accId) { return !u.gone && !Object.entries(S.map.accounts).some(([a, id]) => id === u.id && a !== accId) && !Object.entries(S.map.profiles).some(([pid, id]) => id === u.id && (profile(pid) || {}).accountId !== accId); }
/* Seerr rights follow the wish right of the account (administrators keep theirs) */
function syncRights(accId) {
  const acc = account(accId), u = seerrUser(S.map.accounts[accId]);
  if (!acc || !u || u.admin || u.origin !== 'slouchcat') return;
  u.canMovie = u.canShow = !!acc.canRequest;
}
/* open the account in Seerr: link an existing user with the same e-mail, else create one.
   o.password: the password is at hand right now (invitation accepted, password changed, sign-in).
   o.by === 'self': the account typed the e-mail itself (invitation accepted). The owner decided on 17 Nov 2026
   that an existing user with the same e-mail is linked at once in this case too; the administrators see a hint. */
function provisionAccount(accId, o = {}) {
  const acc = account(accId);
  if (!acc || S.map.accounts[accId]) return accSeerr(acc);
  if (seerrState().k !== 'ok') {
    acc.seerr = { state: 'waiting', at: Date.now(), by: o.by || 'admin' };
    logSeerr('Benutzer eröffnen', 'bad', `${acc.email}: ${seerrState().label}. Neuer Versuch beim nächsten Abgleich.`);
    return accSeerr(acc);
  }
  const sync = S.settings.wishes.pwSync;
  const hit = S.seerrUsers.find((u) => seerrFree(u) && u.email.toLowerCase() === acc.email.toLowerCase());
  if (hit) {
    S.map.accounts[accId] = hit.id;
    acc.seerr = { state: 'linked', at: Date.now(), by: o.by || 'admin', pw: hit.admin ? 'admin' : hit.type !== 'Lokal' ? 'external' : !sync ? 'off' : o.password ? 'synced' : 'next_login', pwAt: o.password && sync && !hit.admin && hit.type === 'Lokal' ? Date.now() : null };
    logSeerr('Benutzer eröffnen', 'ok', `${acc.email}: in Seerr gab es «${hit.name}» schon, verknüpft`);
    return accSeerr(acc);
  }
  const id = Math.max(...S.seerrUsers.map((u) => u.id)) + 1;
  const u = { id, name: acc.email.split('@')[0], email: acc.email, type: 'Lokal', origin: 'slouchcat', canMovie: !!acc.canRequest, canShow: !!acc.canRequest, autoApprove: false, quota: null, gone: false, direct: [] };
  S.seerrUsers.push(u);
  S.map.accounts[accId] = id;
  acc.seerr = { state: 'created', at: Date.now(), pw: !sync ? 'off' : o.password ? 'synced' : 'next_login', pwAt: o.password && sync ? Date.now() : null };
  const lim = limitOf(acc);
  logSeerr('Benutzer eröffnen', 'ok', `${acc.email} als «${u.name}» angelegt, ${acc.seerr.pw === 'synced' ? 'Passwort übernommen' : acc.seerr.pw === 'next_login' ? 'Passwort folgt bei der nächsten Anmeldung' : 'ohne Passwortabgleich'}, ${lim ? `Limit ${lim.movie.n || '∞'} Filme / ${lim.tv.n || '∞'} Staffeln` : 'ohne Limit'}`);
  return accSeerr(acc);
}
function provisionAll() { const list = S.accounts.filter((a) => !S.map.accounts[a.id]); list.forEach((a) => provisionAccount(a.id)); save(); return list.length; }
/* accounts that were linked to an existing Seerr user because the invitee typed the same e-mail (last 7 days) */
function recentSelfLinks() { return S.accounts.filter((a) => { const st = accSeerr(a); return st.state === 'linked' && st.by === 'self' && Date.now() - (st.at || 0) < 7 * D; }); }
/* the administrator picks an existing Seerr user by hand */
function linkAccount(accId, userId) {
  const acc = account(accId), u = seerrUser(userId);
  if (!acc || !u) return;
  S.map.accounts[accId] = u.id;
  acc.seerr = { state: 'manual', at: Date.now(), pw: u.admin ? 'admin' : u.type !== 'Lokal' ? 'external' : S.settings.wishes.pwSync ? 'next_login' : 'off' };
  logSeerr('Benutzer zuordnen', 'ok', `${acc.email} → «${u.name}»`);
  save();
}
/* the link goes, the user stays in Seerr (deleting it there would delete its requests) */
function unlinkAccount(accId) {
  const acc = account(accId);
  if (!acc) return;
  delete S.map.accounts[accId];
  acc.seerr = { state: 'none' };
  save();
}
/* sign-in or password change in Slouchcat: the only moments at which Slouchcat holds the password */
function pwSignIn(accId) {
  const acc = account(accId), st = accSeerr(acc);
  if (!st.user || st.pw !== 'next_login') return false;
  if (seerrState().k !== 'ok') return false;
  acc.seerr.pw = 'synced'; acc.seerr.pwAt = Date.now();
  logSeerr('Passwort abgleichen', 'ok', `«${st.user.name}»: Passwort bei der Anmeldung übernommen`);
  save();
  return true;
}
function pwChanged(accId) {
  const acc = account(accId), st = accSeerr(acc);
  if (!st.user || !['synced', 'next_login'].includes(st.pw)) return st.pw || 'none';
  if (seerrState().k !== 'ok') { acc.seerr.pw = 'next_login'; logSeerr('Passwort abgleichen', 'bad', `«${st.user.name}»: ${seerrState().label}. Folgt bei der nächsten Anmeldung.`); }
  else { acc.seerr.pw = 'synced'; acc.seerr.pwAt = Date.now(); logSeerr('Passwort abgleichen', 'ok', `«${st.user.name}»: neues Passwort übernommen`); }
  save();
  return acc.seerr.pw;
}
function pwSetMode(accId, on) {
  const acc = account(accId), st = accSeerr(acc);
  if (!st.user || ['admin', 'external'].includes(st.pw)) return;
  acc.seerr.pw = on ? 'next_login' : 'off';
  save();
}
/* «Einladung annehmen»: the account with its main profile, and at the same moment its user in Seerr */
function acceptInvitation(invId, d) {
  const inv = S.invitations.find((i) => i.id === invId);
  if (!inv || inv.state !== 'open') return null;
  const now = Date.now(), aid = 'a' + ++S.seq, pid = 'p' + ++S.seq;
  const ev2 = ['announcements', 'wish_available'];
  S.accounts.push({ id: aid, email: d.email, role: 'user', owner: false, maxRating: inv.maxRating, allLibraries: false, canRequest: !!inv.canRequest, lastSeen: now, limit: inv.limit || { mode: 'default' }, seerr: { state: 'none' },
    channels: { email: { enabled: true, events: [...ev2], address: '' }, push: { enabled: false, events: [...ev2], devices: 0 }, telegram: { enabled: false, events: [...ev2], linked: false } } });
  S.profiles.push({ id: pid, accountId: aid, name: d.name, primary: true, maxRating: inv.maxRating, pin: false, accent: '#f0b84a', on: '#1f1503', canRequest: !!inv.canRequest });
  inv.state = 'used'; inv.usedBy = aid; inv.usedAt = now;
  if (S.settings.wishes.provision && S.seerr.configured && S.seerr.enabled) provisionAccount(aid, { password: true, by: 'self' });
  save();
  return { accountId: aid, profileId: pid };
}

/* ---------- pages «Entdecken», «Filme», «Serien», «Suche» ---------- */
const DISC_ROWS = [
  ['trending', 'Im Trend', ''],
  ['popularMovies', 'Beliebte Filme', '#/discover/movies'],
  ['upcomingMovies', 'Demnächst im Kino', '#/discover/movies?sort=new'],
  ['popularShows', 'Beliebte Serien', '#/discover/series'],
  ['upcomingShows', 'Demnächst als Serie', '#/discover/series?sort=new'],
];
/* the pages exist for a profile that may request; everybody else keeps «Meine Wünsche» if there are any */
function discoverOn(p) { return S.settings.wishes.fromDiscover !== false && canRequest(p).ok; }
function discoverRow(key, p) { return ((window.DISCOVER_ROWS || {})[key] || []).map((k) => T[k]).filter((t) => t && titleVisible(p, t)); }
const DISC_ORDER = (() => { const r = window.DISCOVER_ROWS || {}; const seen = new Set(); return [...(r.popularMovies || []), ...(r.popularShows || []), ...(r.trending || []), ...(r.upcomingMovies || []), ...(r.upcomingShows || []), ...window.TITLES.map((t) => t.key)].filter((k) => !seen.has(k) && seen.add(k)); })();
/* kind: 'movie' | 'series'; o: { genre, sort: 'popular'|'new'|'title', hidePresent } */
function discoverList(kind, p, o = {}) {
  let list = DISC_ORDER.map((k) => T[k]).filter((t) => t && t.kind === kind && titleVisible(p, t));
  if (o.genre) list = list.filter((t) => (t.genres || []).includes(o.genre));
  if (o.hidePresent) list = list.filter((t) => !isPresent(t.key));
  if (o.sort === 'new') list = [...list].sort((a, b) => String(b.release || b.year || '').localeCompare(String(a.release || a.year || '')));
  if (o.sort === 'title') list = [...list].sort((a, b) => a.title.localeCompare(b.title, 'de'));
  return list;
}
function discoverGenres(kind, p) { return [...new Set(discoverList(kind, p).flatMap((t) => t.genres || []))].sort((a, b) => a.localeCompare(b, 'de')); }
/* search over everything TMDB knows (here: all demo titles); «tmdb:123», «tvdb:123» and «imdb:tt…» find by id */
function discoverSearch(q, p) {
  const s = String(q || '').trim().toLowerCase();
  if (!s) return [];
  const all = DISC_ORDER.map((k) => T[k]).filter((t) => t && titleVisible(p, t));
  const id = s.match(/^(tmdb|tvdb|imdb):\s*(\S+)$/);
  if (id) return all.filter((t) => (id[1] === 'tmdb' ? String(t.tmdbId) === id[2] : id[1] === 'tvdb' ? String(t.tvdbId || '') === id[2] : String(t.imdbId || '').toLowerCase() === id[2]));
  return all.filter((t) => t.title.toLowerCase().includes(s) || (t.original || '').toLowerCase().includes(s));
}
/* tabs of the area «Wünsche»; `active`: 'discover' | 'movies' | 'series' | 'search' | 'mine' */
function wishNav(active) {
  const p = curProfile();
  const open = myWishes(p).filter((w) => OPEN_STATES.includes(w.state)).length;
  const tabs = discoverOn(p) ? [['discover', '#/discover', 'Entdecken', 'star'], ['movies', '#/discover/movies', 'Filme', 'film'], ['series', '#/discover/series', 'Serien', 'tv'], ['search', '#/discover/search', 'Suche', 'search']] : [];
  tabs.push(['mine', '#/wishes', 'Meine Wünsche', 'wish', open]);
  return `<div class="whead"><nav class="wtabs" aria-label="Bereiche unter Wünsche"><ul>${tabs.map(([k, h, l, ic, c]) => `<li><a href="${h}" ${k === active ? 'aria-current="page"' : ''} data-fid="wt:${k}">${icon(ic, 16)}<span>${l}</span>${c ? `<span class="c">${c}</span>` : ''}</a></li>`).join('')}</ul></nav>${quotaPill(p)}</div>`;
}
/* how Slouchcat recognised a series of the Trakt list: Trakt sends TMDB, TVDB and IMDb ids */
const ID_MATCH = { 'mobland': 'tvdb', 'tulsa-king': 'tvdb', 'rings-of-power': 'tvdb', 'dexter-resurrection': 'tmdb', 'the-body': 'resolved', 'black-doves': 'both', 'black-clover': 'both' };
const ID_MATCH_TEXT = { tvdb: 'In der Bibliothek über die TVDB-ID erkannt', tmdb: 'In der Bibliothek über die TMDB-ID erkannt', both: 'TMDB- und TVDB-ID von Trakt', resolved: 'Trakt nennt nur die TVDB-ID, TMDB-ID nachgeschlagen' };
/* «TMDB 1 · TVDB 2 · IMDb tt3»; an id that is not known shows as «–» (films have no TVDB id) */
function idLine(t) { return ['TMDB ' + (t.tmdbId || '–'), ...(t.kind === 'series' ? ['TVDB ' + (t.tvdbId || '–')] : []), 'IMDb ' + (t.imdbId || '–')].join(' · '); }

/* ---------- download progress from Radarr and Sonarr (read only), shown at wishes and nowhere else ---------- */
const DL_STEPS = [{ k: 'search' }, { k: 'download', pct: 12, eta: 38 }, { k: 'download', pct: 45, eta: 12 }, { k: 'download', pct: 92, eta: 2 }, { k: 'import' }];
function progressOn() { return S.settings.wishes.progress !== false; }
/* what Radarr or Sonarr report for a wish that is being acquired:
   { k: 'search'|'download'|'import'|'stalled'|'unknown', pct?, eta?, label (viewer), admin (administration), arr } or null */
function dlInfo(w) {
  if (!progressOn() || !['processing', 'partial'].includes(w.state) || !w.seerrRequestId) return null;
  const arr = arrOf(w);
  if (arr.condition !== 'ok') return { k: 'unknown', arr, label: '', admin: `Fortschritt nicht lesbar: ${arr.name} ist nicht erreichbar` };
  const d = w.dl || { step: 0 };
  const s = DL_STEPS[Math.min(d.step || 0, DL_STEPS.length - 1)];
  if (d.stalled) return { k: 'stalled', arr, pct: s.pct, label: 'Download verzögert', admin: d.msg || `${arr.name} meldet einen Fehler beim Download.` };
  if (s.k === 'search') return { k: 'search', arr, label: 'Wird gesucht', admin: `${arr.name} sucht eine Veröffentlichung. Noch kein Download in der Warteschlange.` };
  if (s.k === 'import') return { k: 'import', arr, pct: 100, label: 'Wird eingelesen', admin: `${arr.name} hat die Datei importiert. Slouchcat liest sie ein.` };
  const eps = d.eps ? ` · ${d.eps[0]} von ${d.eps[1]} Folgen da` : '';
  return { k: 'download', arr, pct: s.pct, eta: s.eta, label: `Wird heruntergeladen · ${s.pct} %${eps} · noch ${s.eta} min`, admin: `In der Warteschlange von ${arr.name}: ${s.pct} %, noch etwa ${s.eta} Minuten.` };
}
const DL_ICON = { search: 'search', download: 'dl', import: 'refresh', stalled: 'alert', unknown: 'help' };
function dlBar(x) { return x.pct != null ? `<span class="bar" role="img" aria-label="${x.pct} Prozent"><i style="width:${x.pct}%"></i></span>` : ''; }
/* compact line for lists; in the viewer app an unreadable progress shows nothing (the wish stays «In Beschaffung») */
function dlChip(w, admin) {
  const x = dlInfo(w);
  if (!x || (x.k === 'unknown' && !admin)) return '';
  return `<span class="dl ${x.k}">${icon(DL_ICON[x.k], 14)}${dlBar(x)}<span>${esc(x.k === 'unknown' ? 'Fortschritt nicht lesbar' : x.label)}</span></span>`;
}
/* block for the wish card of a title and for the details in the administration */
function dlBlock(w, admin) {
  const x = dlInfo(w);
  if (!x || (x.k === 'unknown' && !admin)) return '';
  return `<div class="dlb ${x.k}"><div class="dlh">${icon(DL_ICON[x.k], 16)}<b>${esc(x.k === 'unknown' ? 'Fortschritt nicht lesbar' : x.label)}</b></div>${dlBar(x)}<p>${admin ? esc(x.admin) : `Stand aus ${esc(x.arr.name)}, vor weniger als einer Minute gelesen.`}</p></div>`;
}
function dlStep(id) { const w = wishById(id); if (!w) return; w.dl = w.dl || { step: 0 }; w.dl.stalled = false; if (w.dl.step < DL_STEPS.length - 1) w.dl.step++; save(); }
function dlStall(id) { const w = wishById(id); if (!w) return; w.dl = w.dl || { step: 1 }; if (!w.dl.step) w.dl.step = 1; w.dl.stalled = !w.dl.stalled; if (w.dl.stalled && !w.dl.msg) w.dl.msg = `${arrOf(w).name} meldet: Der Download-Client antwortet nicht.`; save(); }
