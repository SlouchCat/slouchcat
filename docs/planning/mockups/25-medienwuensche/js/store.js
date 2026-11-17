/* Simulated state of the mockup: accounts, profiles, Jellyseerr users, mapping, wishes, notifications.
   Nothing here talks to a server. The state lives in localStorage so a reload keeps what you tried. */
'use strict';

const STORE_KEY = 'slouchcat-mockup-25-v4';
const H = 3600e3, D = 24 * H;
const T = Object.fromEntries(window.TITLES.map((t) => [t.key, t]));
T['godzilla-minus-one'].age = 12;
const TMDB = Object.fromEntries(window.TITLES.map((t) => [(t.kind === 'movie' ? 'movie' : 'show') + '/' + t.tmdbId, t]));
/* Trakt order of the Spotlight list (present and missing titles mixed, as Trakt delivers them) */
const SPOT_ORDER = ['heart-of-the-beast', 'odyssey', 'resident-evil', 'spider-man-bnd', 'primetime', 'end-of-oak-street', 'verity', 'dog-stars', 'street-fighter', 'project-hail-mary', 'obsession', 'godzilla-minus-one', 'coyote-vs-acme', 'masterplan', 'tenzing', 'toy-story-5', 'backrooms', 'super-mario-galaxy', 'mobland', 'black-doves', 'dexter-resurrection', 'clayface', 'tulsa-king', 'moana', 'dune-2', 'the-body', 'supergirl', 'other-mommy', 'michael', 'black-clover', 'rings-of-power', 'dune', 'interstellar', 'matrix'];
/* seasons that TMDB lists but that have not aired yet: shown as «Angekündigt», not requestable */
const UPCOMING = { 'tulsa-king': [4], 'rings-of-power': [3], 'dexter-resurrection': [2] };
const SERIES_NOTE = { 'mobland': 'Läuft noch · nächste Folge am 22. November 2026', 'tulsa-king': 'Staffel 4 ist angekündigt', 'rings-of-power': 'Staffel 3 ist angekündigt', 'dexter-resurrection': 'Staffel 2 ist angekündigt', 'black-doves': 'Läuft noch · Staffel 2 seit Oktober 2026', 'the-body': 'Läuft noch · nächste Folge am 19. November 2026', 'black-clover': 'Läuft noch' };

const STATE_LABEL = { queued: 'Wird übermittelt', pending: 'Freigabe ausstehend', processing: 'In Beschaffung', partial: 'Teilweise verfügbar', available: 'Verfügbar', declined: 'Abgelehnt', failed: 'Fehlgeschlagen', withdrawn: 'Zurückgezogen' };
const STATE_ICON = { queued: 'send', pending: 'hourglass', processing: 'truck', partial: 'truck', available: 'check', declined: 'ban', failed: 'alert', withdrawn: 'undo' };
const OPEN_STATES = ['queued', 'pending', 'processing', 'partial'];
const FAIL_TEXT = {
  quota: 'Das Kontingent des Jellyseerr-Benutzers ist erschöpft.',
  no_permission: 'Der Jellyseerr-Benutzer darf diese Art von Titel nicht anfordern.',
  user_gone: 'Den zugeordneten Jellyseerr-Benutzer gibt es nicht mehr.',
  arr: 'Jellyseerr konnte den Titel nicht an Radarr oder Sonarr übergeben.',
  timeout: 'Jellyseerr war 24 Stunden lang nicht erreichbar.',
};
const EVENT_LABEL = { new_content: 'Neue Inhalte', errors: 'Fehler', invitations: 'Einladungen', announcements: 'Ankündigungen', wish_available: 'Wünsche: verfügbar', wish_status: 'Wünsche: freigegeben, abgelehnt, fehlgeschlagen', wish_review: 'Wünsche: Freigabe nötig' };
const CHANNEL_LABEL = { email: 'E-Mail', push: 'Web-Push', telegram: 'Telegram' };

function seed() {
  const now = Date.now();
  const ch = (email, push, tg) => ({ email, push, telegram: tg });
  return {
    v: 4,
    session: { accountId: 'a1', profileId: 'p1' },
    view: 'desktop',
    seerr: {
      enabled: true, configured: true, url: 'http://192.168.1.50:5055', keyTail: 'a1b2', condition: 'ok', version: 'Seerr 3.5.2',
      lastSync: now - 3 * 60e3, lastTest: now - 2 * D, webhook: false,
      log: [
        { at: now - 3 * 60e3, op: 'Abgleich', res: 'ok', text: '6 Anfragen geprüft, 0 geändert' },
        { at: now - 3 * H, op: 'Wunsch übermitteln', res: 'ok', text: '«Toy Story 5» als kinder · Freigabe ausstehend' },
        { at: now - 26 * H, op: 'Abgleich', res: 'ok', text: '«Godzilla Minus One»: in Jellyseerr verfügbar' },
        { at: now - 2 * D, op: 'Verbindung testen', res: 'ok', text: 'Seerr 3.5.2, alle Prüfungen bestanden' },
      ],
    },
    settings: {
      wishes: { enabled: true, movies: true, shows: true, fromSpotlight: true, fromSearch: true, fromDetail: true, join: 'join', unmapped: 'deny', fallbackUser: '', newAccounts: false, withdraw: true, pinPrivacy: true },
      spotlight: { enabled: true, listKey: 'trending', listUrl: '', media: 'both', itemCount: 20, minHits: 5, intervalHours: 24, rotationSeconds: 5, showMissing: true, maxMissing: 5, missingOrder: 'trakt', overrides: {} },
      homeRows: [{ key: 'spotlight', hidden: false }, { key: 'continue', hidden: false }, { key: 'wishes', hidden: false }, { key: 'recent', hidden: false }, { key: 'released', hidden: false }, { key: 'picks', hidden: false }, { key: 'watchlist', hidden: false }, { key: 'collections', hidden: true }, { key: 'genres', hidden: true }, { key: 'libraries', hidden: true }],
      jobs: { 'wishes.sync': { every: '5 min', on: true }, 'wishes.reconcile': { every: '1 h', on: true }, 'seerr.users': { every: '24 h', on: true }, 'titles.refresh': { every: '24 h', on: true }, 'series.catalog': { every: '7 d', on: true } },
    },
    accounts: [
      { id: 'a1', email: 'marco@example.ch', role: 'admin', owner: true, maxRating: 18, allLibraries: true, canRequest: true, lastSeen: now - 5 * 60e3,
        channels: ch({ enabled: true, events: ['new_content', 'errors', 'invitations', 'announcements', 'wish_available', 'wish_review'], address: '' }, { enabled: true, events: ['new_content', 'wish_available', 'wish_review'], devices: 2 }, { enabled: false, events: ['announcements', 'wish_available'], linked: false }) },
      { id: 'a2', email: 'grosseltern@example.ch', role: 'user', owner: false, maxRating: 16, allLibraries: true, canRequest: true, lastSeen: now - 2 * D,
        channels: ch({ enabled: true, events: ['announcements', 'wish_available'], address: '' }, { enabled: false, events: ['announcements', 'wish_available'], devices: 0 }, { enabled: false, events: ['announcements', 'wish_available'], linked: false }) },
      { id: 'a3', email: 'gast@example.ch', role: 'user', owner: false, maxRating: 16, allLibraries: false, canRequest: false, lastSeen: now - 9 * D,
        channels: ch({ enabled: true, events: ['announcements', 'wish_available'], address: '' }, { enabled: false, events: ['announcements', 'wish_available'], devices: 0 }, { enabled: false, events: ['announcements', 'wish_available'], linked: false }) },
    ],
    profiles: [
      { id: 'p1', accountId: 'a1', name: 'Marco', primary: true, maxRating: 18, pin: false, accent: '#e8794a', on: '#1c0d06', canRequest: true },
      { id: 'p2', accountId: 'a1', name: 'Sandra', primary: false, maxRating: 18, pin: true, accent: '#ec77a3', on: '#1f0812', canRequest: true },
      { id: 'p3', accountId: 'a1', name: 'Noah', primary: false, maxRating: 12, pin: false, accent: '#62aef0', on: '#06121f', canRequest: true },
      { id: 'p4', accountId: 'a1', name: 'Mia', primary: false, maxRating: 6, pin: false, accent: '#a3d465', on: '#111a06', canRequest: false },
      { id: 'p5', accountId: 'a2', name: 'Oma & Opa', primary: true, maxRating: 16, pin: false, accent: '#45c2b1', on: '#04191a', canRequest: true },
      { id: 'p6', accountId: 'a3', name: 'Gast', primary: true, maxRating: 16, pin: false, accent: '#a993f5', on: '#120c24', canRequest: false },
    ],
    seerrUsers: [
      { id: 1, name: 'marco', email: 'marco@example.ch', type: 'Administrator', canMovie: true, canShow: true, autoApprove: true, quota: null, gone: false },
      { id: 2, name: 'sandra', email: 'sandra@example.ch', type: 'Lokal', canMovie: true, canShow: true, autoApprove: true, quota: null, gone: false },
      { id: 3, name: 'kinder', email: 'kinder@example.ch', type: 'Lokal', canMovie: true, canShow: true, autoApprove: false, quota: { n: 5, days: 7 }, gone: false },
      { id: 4, name: 'grosseltern', email: 'grosseltern@example.ch', type: 'Lokal', canMovie: true, canShow: false, autoApprove: false, quota: null, gone: false },
      { id: 5, name: 'wohnzimmer-tv', email: 'tv@example.ch', type: 'Jellyfin', canMovie: true, canShow: true, autoApprove: true, quota: null, gone: false },
    ],
    map: { accounts: { a1: 1 }, profiles: { p2: 2, p3: 3 } },
    library: { 'godzilla-minus-one': { present: true, seasons: [] } },
    marks: { watchlist: { 'dune-2': true, 'mobland': true }, watched: { 'dune': true, 'interstellar': true } },
    seq: 108,
    wishes: [
      { id: 'w101', titleKey: 'resident-evil', media: 'movie', seasons: [], state: 'processing', origin: 'slouchcat', seerrUser: 1, seerrRequestId: 412, owner: 'p1', joined: [], created: now - 2 * D,
        events: [{ at: now - 2 * D, k: 'created', t: 'Angefordert von Marco' }, { at: now - 2 * D + 2e3, k: 'submitted', t: 'An Jellyseerr übermittelt als marco' }, { at: now - 2 * D + 3e3, k: 'approved', t: 'Automatisch freigegeben' }] },
      { id: 'w102', titleKey: 'toy-story-5', media: 'movie', seasons: [], state: 'pending', origin: 'slouchcat', seerrUser: 3, seerrRequestId: 418, owner: 'p3', joined: [], created: now - 3 * H,
        events: [{ at: now - 3 * H, k: 'created', t: 'Angefordert von Noah' }, { at: now - 3 * H + 2e3, k: 'submitted', t: 'An Jellyseerr übermittelt als kinder' }] },
      { id: 'w103', titleKey: 'godzilla-minus-one', media: 'movie', seasons: [], state: 'available', origin: 'slouchcat', seerrUser: 1, seerrRequestId: 398, owner: 'p1', joined: ['p3'], created: now - 4 * D, closed: now - 26 * H,
        events: [{ at: now - 4 * D, k: 'created', t: 'Angefordert von Marco' }, { at: now - 4 * D + 2e3, k: 'submitted', t: 'An Jellyseerr übermittelt als marco' }, { at: now - 4 * D + 3e3, k: 'approved', t: 'Automatisch freigegeben' }, { at: now - 3 * D, k: 'joined', t: 'Noah wünscht mit' }, { at: now - 26 * H, k: 'available', t: 'In Slouchcat verfügbar' }] },
      { id: 'w104', titleKey: 'street-fighter', media: 'movie', seasons: [], state: 'declined', origin: 'slouchcat', seerrUser: 3, seerrRequestId: 405, owner: 'p3', joined: [], created: now - 3 * D, closed: now - 2 * D,
        events: [{ at: now - 3 * D, k: 'created', t: 'Angefordert von Noah' }, { at: now - 3 * D + 2e3, k: 'submitted', t: 'An Jellyseerr übermittelt als kinder' }, { at: now - 2 * D, k: 'declined', t: 'In Jellyseerr abgelehnt' }] },
      { id: 'w105', titleKey: 'black-doves', media: 'show', seasons: [1], seasonState: { 1: 'processing' }, state: 'processing', origin: 'slouchcat', seerrUser: 2, seerrRequestId: 415, owner: 'p2', joined: [], created: now - 20 * H,
        events: [{ at: now - 20 * H, k: 'created', t: 'Angefordert von Sandra: Staffel 1' }, { at: now - 20 * H + 2e3, k: 'submitted', t: 'An Jellyseerr übermittelt als sandra' }, { at: now - 20 * H + 3e3, k: 'approved', t: 'Automatisch freigegeben' }] },
      { id: 'w106', titleKey: 'coyote-vs-acme', media: 'movie', seasons: [], state: 'failed', failReason: 'arr', origin: 'slouchcat', seerrUser: 2, seerrRequestId: 409, owner: 'p2', joined: [], created: now - 30 * H, closed: now - 29 * H,
        events: [{ at: now - 30 * H, k: 'created', t: 'Angefordert von Sandra' }, { at: now - 30 * H + 2e3, k: 'submitted', t: 'An Jellyseerr übermittelt als sandra' }, { at: now - 29 * H, k: 'failed', t: FAIL_TEXT.arr }] },
      { id: 'w107', titleKey: 'clayface', media: 'movie', seasons: [], state: 'processing', origin: 'jellyseerr', seerrUser: 5, seerrRequestId: 401, owner: null, joined: [], created: now - 5 * D,
        events: [{ at: now - 5 * D, k: 'imported', t: 'In Jellyseerr angelegt von wohnzimmer-tv' }] },
      { id: 'w108', titleKey: 'verity', media: 'movie', seasons: [], state: 'processing', origin: 'jellyseerr', seerrUser: 2, seerrRequestId: 403, owner: 'p2', joined: [], created: now - 6 * D,
        events: [{ at: now - 6 * D, k: 'imported', t: 'In Jellyseerr angelegt von sandra, Sandra zugeordnet' }] },
    ],
    notifications: [
      { id: 'n1', profileId: 'p1', kind: 'wish_available', titleKey: 'godzilla-minus-one', wishId: 'w103', at: now - 26 * H, read: false },
      { id: 'n2', profileId: 'p3', kind: 'wish_available', titleKey: 'godzilla-minus-one', wishId: 'w103', at: now - 26 * H, read: false },
      { id: 'n3', profileId: 'p3', kind: 'wish_declined', titleKey: 'street-fighter', wishId: 'w104', at: now - 2 * D, read: true },
      { id: 'n4', profileId: 'p1', kind: 'new_episode', titleKey: 'mobland', at: now - 3 * D, read: true, text: 'S02E06 · Die Abrechnung' },
      { id: 'n5', profileId: 'p2', kind: 'wish_failed', titleKey: 'coyote-vs-acme', wishId: 'w106', at: now - 29 * H, read: false },
    ],
    deliveries: [
      { at: now - 26 * H, accountId: 'a1', profileId: 'p1', channel: 'email', event: 'wish_available', subject: '[Marco] Dein Wunsch ist da: Godzilla Minus One', state: 'sent' },
      { at: now - 26 * H, accountId: 'a1', profileId: 'p1', channel: 'push', event: 'wish_available', subject: '[Marco] Dein Wunsch ist da: Godzilla Minus One', state: 'sent' },
      { at: now - 3 * H, accountId: 'a1', profileId: 'p3', channel: 'email', event: 'wish_review', subject: 'Wunsch wartet auf Freigabe: Toy Story 5 (Noah)', state: 'sent' },
    ],
    invitations: [{ id: 'i1', note: 'Für Tante Ruth', state: 'open', maxRating: 16, canRequest: false, until: now + 5 * D }],
  };
}

let S;
function load() {
  try { const raw = localStorage.getItem(STORE_KEY); if (raw) { const o = JSON.parse(raw); if (o && o.v === 4) return o; } } catch (e) { /* private mode or blocked storage */ }
  return null;
}
function save() { try { localStorage.setItem(STORE_KEY, JSON.stringify(S)); } catch (e) { /* ignore */ } }
S = load() || seed();
function resetAll() { S = seed(); save(); ui.open = {}; ui.draft = {}; ui.hero = { i: 0, paused: false }; }
function commit() { save(); render(); }

/* ---------- selectors ---------- */
const profile = (id) => S.profiles.find((p) => p.id === id);
const account = (id) => S.accounts.find((a) => a.id === id);
const curProfile = () => profile(S.session.profileId);
const curAccount = () => account(S.session.accountId);
const isAdmin = () => curAccount().role === 'admin' && curProfile().primary;
const seerrUser = (id) => S.seerrUsers.find((u) => u.id === Number(id));
const profilesOf = (accId) => S.profiles.filter((p) => p.accountId === accId);
const effMax = (p) => Math.min(p.maxRating, account(p.accountId).maxRating);
const roleLabel = (p) => (account(p.accountId).role === 'admin' && p.primary ? 'Server-Admin' : p.primary ? 'Hauptprofil' : 'Weiteres Profil');
const ageLabel = (n) => (n >= 18 ? 'ab 18' : 'ab ' + n);
const mediaOf = (t) => (t.kind === 'movie' ? 'movie' : 'show');
const kindLabel = (t) => (t.kind === 'movie' ? 'Film' : 'Serie');

function isPresent(key) { const t = T[key]; return !!(t.inLibrary || (S.library[key] && S.library[key].present)); }
function libSeasons(key) { const t = T[key]; return [...new Set([...(t.inLibrary ? t.libSeasons || [] : []), ...((S.library[key] && S.library[key].seasons) || [])])].sort((a, b) => a - b); }
function upcoming(key) { return UPCOMING[key] || []; }
function airedSeasons(t) { return (t.seasons || []).filter((s) => !upcoming(t.key).includes(s.n)); }
function missingSeasons(t) { const have = libSeasons(t.key); return airedSeasons(t).filter((s) => !have.includes(s.n)).map((s) => s.n); }
function titleHref(t) { return isPresent(t.key) ? '#/items/' + t.key : `#/titles/${mediaOf(t)}/${t.tmdbId}`; }

/* which Jellyseerr user a profile sends as: own mapping, else the account's, else the fallback user */
function seerrUserFor(p) {
  const own = S.map.profiles[p.id];
  if (own) return { id: own, via: 'profile' };
  const acc = S.map.accounts[p.accountId];
  if (acc) return { id: acc, via: 'account' };
  if (S.settings.wishes.unmapped === 'fallback' && S.settings.wishes.fallbackUser) return { id: Number(S.settings.wishes.fallbackUser), via: 'fallback' };
  return null;
}
/* the server's answer «may this profile request (this title)?» with a reason the app can show */
function canRequest(p, t) {
  const w = S.settings.wishes;
  if (!w.enabled) return { ok: false, reason: 'off', text: 'Wünsche sind ausgeschaltet.' };
  if (!S.seerr.enabled || !S.seerr.configured) return { ok: false, reason: 'seerr_off', text: 'Jellyseerr ist nicht eingerichtet.' };
  if (!account(p.accountId).canRequest) return { ok: false, reason: 'account', text: 'Dieses Konto darf keine Titel anfordern.' };
  if (!p.canRequest) return { ok: false, reason: 'profile', text: 'Dieses Profil darf keine Titel anfordern. Das Hauptprofil kann es unter «Profile» erlauben.' };
  const su = seerrUserFor(p);
  if (!su) return { ok: false, reason: 'unmapped', text: 'Diesem Konto ist noch kein Jellyseerr-Benutzer zugeordnet. Das richtet der Administrator ein.' };
  const u = seerrUser(su.id);
  if (!u || u.gone) return { ok: false, reason: 'seerr_user_gone', text: 'Den zugeordneten Jellyseerr-Benutzer gibt es nicht mehr. Das behebt der Administrator.' };
  if (t) {
    if ((t.kind === 'movie' && !u.canMovie) || (t.kind === 'series' && !u.canShow)) return { ok: false, reason: 'seerr_kind', text: 'Der zugeordnete Jellyseerr-Benutzer darf diese Art von Titel nicht anfordern.' };
    if (t.kind === 'movie' && !w.movies) return { ok: false, reason: 'kind', text: 'Filme können zurzeit nicht angefordert werden.' };
    if (t.kind === 'series' && !w.shows) return { ok: false, reason: 'kind', text: 'Serien können zurzeit nicht angefordert werden.' };
    if (t.age > effMax(p)) return { ok: false, reason: 'age', text: 'Dieser Titel liegt über der Altersstufe des Profils.' };
  }
  return { ok: true };
}
/* missing titles exist for a profile only if it may request and the age allows it (else: 404) */
function missingVisible(p, t) { const w = S.settings.wishes; return (w.fromSpotlight || w.fromSearch || w.fromDetail) && canRequest(p).ok && t.age <= effMax(p) && canRequest(p, t).ok; }
function titleVisible(p, t) { return isPresent(t.key) ? t.age <= effMax(p) : missingVisible(p, t); }

function openWish(key, season) {
  return S.wishes.find((w) => w.titleKey === key && OPEN_STATES.includes(w.state) && (season == null || !w.seasons.length || w.seasons.includes(season)));
}
function openWishes(key) { return S.wishes.filter((w) => w.titleKey === key && OPEN_STATES.includes(w.state)); }
function isMine(w, p) { return w.owner === p.id || w.joined.includes(p.id); }
function lastClosed(key, p) { return S.wishes.filter((w) => w.titleKey === key && !OPEN_STATES.includes(w.state) && isMine(w, p)).sort((a, b) => (b.closed || 0) - (a.closed || 0))[0]; }
/* what the app shows for one title and the active profile */
function availability(p, t) {
  if (isPresent(t.key) && t.kind === 'movie') return { kind: 'present' };
  const ws = openWishes(t.key);
  const mine = ws.find((w) => isMine(w, p));
  if (isPresent(t.key)) {
    const miss = missingSeasons(t);
    return { kind: miss.length ? 'partial' : 'present', missing: miss, wish: mine || ws[0], mine: !!mine };
  }
  if (mine) return { kind: 'wished', wish: mine, mine: true };
  if (ws[0]) return { kind: 'wished', wish: ws[0], mine: false };
  return { kind: 'missing' };
}
function myWishes(p, allProfiles) {
  const ids = allProfiles ? profilesOf(p.accountId).map((x) => x.id) : [p.id];
  return S.wishes.filter((w) => ids.includes(w.owner) || w.joined.some((j) => ids.includes(j))).sort((a, b) => b.created - a.created);
}
function unread(p) { return S.notifications.filter((n) => n.profileId === p.id && !n.read).length; }
function wishCounts() {
  const c = { open: 0, queued: 0, pending: 0, processing: 0, available30: 0, problems: 0, unassigned: 0 };
  S.wishes.forEach((w) => {
    if (OPEN_STATES.includes(w.state)) c.open++;
    if (w.state === 'queued') c.queued++;
    if (w.state === 'pending') c.pending++;
    if (w.state === 'processing' || w.state === 'partial') c.processing++;
    if (w.state === 'available') c.available30++;
    if (w.state === 'failed') c.problems++;
    if (!w.owner) c.unassigned++;
  });
  return c;
}
function mappingCounts() {
  const total = S.accounts.length;
  const mapped = S.accounts.filter((a) => S.map.accounts[a.id]).length;
  return { total, mapped, missing: total - mapped };
}
function suggestionFor(acc) {
  if (S.map.accounts[acc.id]) return null;
  const u = S.seerrUsers.find((x) => !x.gone && x.email.toLowerCase() === acc.email.toLowerCase());
  if (!u || Object.entries(S.map.accounts).some(([a, id]) => id === u.id && a !== acc.id)) return null;
  return u;
}
function seerrState() {
  const c = S.seerr;
  if (!c.configured) return { k: 'none', label: 'Nicht eingerichtet', chip: 'outline', dot: '' };
  if (!c.enabled) return { k: 'off', label: 'Ausgeschaltet', chip: 'outline', dot: '' };
  if (c.condition === 'unreachable') return { k: 'unreachable', label: 'Nicht erreichbar', chip: 'bad', dot: 'bad' };
  if (c.condition === 'badkey') return { k: 'badkey', label: 'Schlüssel ungültig', chip: 'bad', dot: 'bad' };
  if (c.condition === 'csrf') return { k: 'csrf', label: 'CSRF-Schutz blockiert', chip: 'warn', dot: 'warn' };
  return { k: 'ok', label: 'Verbunden', chip: 'ok', dot: 'ok' };
}

/* Spotlight for one profile: Trakt order, first slide always present, at most N missing titles */
function spotlightFor(p) {
  const cfg = S.settings.spotlight, w = S.settings.wishes;
  if (!cfg.enabled) return [];
  const ov = cfg.overrides || {};
  let list = SPOT_ORDER.map((k) => T[k]).filter((t) => !(ov[t.key] && ov[t.key].hidden));
  if (cfg.media === 'movie') list = list.filter((t) => t.kind === 'movie');
  if (cfg.media === 'show') list = list.filter((t) => t.kind === 'series');
  const showMissing = cfg.showMissing && w.enabled && w.fromSpotlight && S.seerr.enabled && S.seerr.configured && canRequest(p).ok;
  const present = list.filter((t) => isPresent(t.key) && t.age <= effMax(p));
  if (present.length < cfg.minHits) return [];
  let missing = showMissing ? list.filter((t) => !isPresent(t.key) && missingVisible(p, t)).slice(0, cfg.maxMissing) : [];
  let out;
  if (cfg.missingOrder === 'after') out = [...present, ...missing];
  else out = list.filter((t) => present.includes(t) || missing.includes(t));
  const pinned = Object.entries(ov).filter(([k, o]) => o.pinned && out.some((t) => t.key === k)).sort((a, b) => a[1].pinned - b[1].pinned);
  pinned.forEach(([k, o]) => { const i = out.findIndex((t) => t.key === k); const [t] = out.splice(i, 1); out.splice(Math.min(o.pinned - 1, out.length), 0, t); });
  const firstPresent = out.findIndex((t) => isPresent(t.key));
  if (firstPresent > 0) { const [t] = out.splice(firstPresent, 1); out.unshift(t); }
  return out.slice(0, cfg.itemCount);
}

/* ---------- notifications and deliveries ---------- */
const N_TEXT = {
  wish_available: (t, n) => ({ title: `Dein Wunsch ist da: ${t.title}`, ctx: n.season ? `Staffel ${n.season} ist jetzt verfügbar` : t.kind === 'movie' ? 'Der Film ist jetzt verfügbar' : 'Die Serie ist jetzt verfügbar', href: '#/items/' + t.key }),
  wish_season_available: (t, n) => ({ title: `${t.title}: Staffel ${n.season} ist da`, ctx: 'Gewünschte Staffel verfügbar', href: '#/items/' + t.key }),
  wish_approved: (t) => ({ title: `Freigegeben: ${t.title}`, ctx: 'Dein Wunsch wird jetzt beschafft', href: '#/wishes' }),
  wish_declined: (t) => ({ title: `Abgelehnt: ${t.title}`, ctx: 'Dein Wunsch wurde abgelehnt', href: '#/wishes' }),
  wish_failed: (t) => ({ title: `Fehlgeschlagen: ${t.title}`, ctx: 'Die Beschaffung hat nicht geklappt', href: '#/wishes' }),
  new_episode: (t, n) => ({ title: n.text || 'Neue Folge', ctx: `Neue Folge von ${t.title}`, href: '#/items/' + t.key }),
};
function notifText(n) { const t = T[n.titleKey]; return (N_TEXT[n.kind] || N_TEXT.new_episode)(t, n); }
const KIND_EVENT = { wish_available: 'wish_available', wish_season_available: 'wish_available', wish_approved: 'wish_status', wish_declined: 'wish_status', wish_failed: 'wish_status' };

function notify(pid, kind, w, extra = {}) {
  const p = profile(pid);
  if (!p) return;
  const t = T[w.titleKey];
  const key = `${w.id}:${kind}:${extra.season || ''}`;
  if (S.notifications.some((n) => n.profileId === pid && n.key === key)) return; /* one message per wish, kind and profile */
  if ((kind === 'wish_available' || kind === 'wish_season_available') && t.age > effMax(p)) return; /* not visible for this profile: no message */
  const n = { id: 'n' + ++S.seq, key, profileId: pid, kind, titleKey: w.titleKey, wishId: w.id, at: Date.now(), read: false, ...extra };
  S.notifications.unshift(n);
  const acc = account(p.accountId);
  const ev = KIND_EVENT[kind];
  const txt = notifText(n);
  const hideTitle = p.pin && S.settings.wishes.pinPrivacy;
  const neutral = { wish_available: 'Ein Wunsch ist jetzt verfügbar.', wish_season_available: 'Ein Wunsch ist jetzt verfügbar.', wish_approved: 'Ein Wunsch wurde freigegeben.', wish_declined: 'Ein Wunsch wurde abgelehnt.', wish_failed: 'Ein Wunsch ist fehlgeschlagen.' }[kind];
  const subject = `[${p.name}] ${hideTitle ? neutral : txt.title}`;
  const sent = ['App'];
  Object.entries(acc.channels).forEach(([c, cfg]) => {
    if (cfg.enabled && cfg.events.includes(ev)) { S.deliveries.unshift({ at: Date.now(), accountId: acc.id, profileId: pid, channel: c, event: ev, subject, state: 'sent' }); sent.push(CHANNEL_LABEL[c]); }
  });
  toast(`Benachrichtigung an <b>${esc(p.name)}</b>: «${esc(txt.title)}» · ${sent.join(', ')}`, 'mail', 6500);
}
function notifyAdmins(w) {
  const t = T[w.titleKey], p = profile(w.owner);
  S.accounts.filter((a) => a.role === 'admin').forEach((a) => {
    Object.entries(a.channels).forEach(([c, cfg]) => {
      if (cfg.enabled && cfg.events.includes('wish_review')) S.deliveries.unshift({ at: Date.now(), accountId: a.id, profileId: null, channel: c, event: 'wish_review', subject: `Wunsch wartet auf Freigabe: ${t.title} (${p ? p.name : 'unbekannt'})`, state: 'sent' });
    });
  });
}
function logSeerr(op, res, text) { S.seerr.log.unshift({ at: Date.now(), op, res, text }); S.seerr.log = S.seerr.log.slice(0, 20); }
function ev(w, k, t) { w.events.push({ at: Date.now(), k, t }); }

/* ---------- actions: what the server and Jellyseerr would do ---------- */
function submitWish(w) {
  const c = S.seerr, t = T[w.titleKey];
  if (!c.enabled || !c.configured || c.condition !== 'ok') {
    w.state = 'queued';
    w.detail = c.condition === 'badkey' ? 'Jellyseerr lehnt den Schlüssel ab' : c.condition === 'csrf' ? 'Jellyseerr blockiert schreibende Aufrufe (CSRF-Schutz)' : 'Jellyseerr ist nicht erreichbar';
    logSeerr('Wunsch übermitteln', 'bad', `«${t.title}»: ${w.detail}. Neuer Versuch folgt.`);
    return;
  }
  const u = seerrUser(w.seerrUser);
  const fail = (r) => { w.state = 'failed'; w.failReason = r; w.closed = Date.now(); ev(w, 'failed', FAIL_TEXT[r]); logSeerr('Wunsch übermitteln', 'bad', `«${t.title}»: ${FAIL_TEXT[r]}`); if (w.owner) notify(w.owner, 'wish_failed', w); };
  if (!u || u.gone) return fail('user_gone');
  if ((w.media === 'movie' && !u.canMovie) || (w.media === 'show' && !u.canShow)) return fail('no_permission');
  if (u.quota && S.wishes.filter((x) => x !== w && x.seerrUser === u.id && x.created > Date.now() - u.quota.days * D && x.seerrRequestId).length >= u.quota.n) return fail('quota');
  w.seerrRequestId = 400 + ++S.seq;
  w.detail = '';
  ev(w, 'submitted', `An Jellyseerr übermittelt als ${u.name}`);
  if (u.autoApprove) { w.state = 'processing'; ev(w, 'approved', 'Automatisch freigegeben'); logSeerr('Wunsch übermitteln', 'ok', `«${t.title}» als ${u.name} · freigegeben`); }
  else { w.state = 'pending'; logSeerr('Wunsch übermitteln', 'ok', `«${t.title}» als ${u.name} · Freigabe ausstehend`); notifyAdmins(w); }
}
/* createWish: one open wish per film / season; a second profile joins instead of ordering twice */
function requestWish(key, seasons = []) {
  const p = curProfile(), t = T[key];
  const cr = canRequest(p, t);
  if (!cr.ok) { toast(esc(cr.text), 'bad'); return null; }
  if (t.kind === 'series' && seasons.length) {
    const others = openWishes(key).filter((w) => !isMine(w, p) && (!w.seasons.length || w.seasons.some((s) => seasons.includes(s))));
    if (others.length && S.settings.wishes.join === 'join') {
      others.forEach((w) => { w.joined.push(p.id); ev(w, 'joined', `${p.name} wünscht mit`); });
      seasons = seasons.filter((s) => !others.some((w) => !w.seasons.length || w.seasons.includes(s)));
      save();
      if (!seasons.length) { toast(`Du wirst benachrichtigt, sobald «${esc(t.title)}» verfügbar ist.`, 'ok'); return others[0]; }
    }
  }
  const existing = openWishes(key).find((w) => !seasons.length || !w.seasons.length || seasons.some((s) => w.seasons.includes(s)));
  if (existing) {
    if (isMine(existing, p)) { toast('Diesen Titel hast du schon angefordert.', 'warn'); return existing; }
    if (S.settings.wishes.join !== 'join') { toast('Dieser Titel ist schon angefordert.', 'warn'); return existing; }
    existing.joined.push(p.id);
    ev(existing, 'joined', `${p.name} wünscht mit`);
    save();
    toast(`Du wirst benachrichtigt, sobald «${esc(t.title)}» verfügbar ist.`, 'ok');
    return existing;
  }
  const su = seerrUserFor(p);
  const w = { id: 'w' + ++S.seq, titleKey: key, media: mediaOf(t), seasons: [...seasons].sort((a, b) => a - b), seasonState: Object.fromEntries(seasons.map((s) => [s, 'processing'])), state: 'queued', origin: 'slouchcat', seerrUser: su.id, seerrRequestId: null, owner: p.id, joined: [], created: Date.now(), events: [] };
  ev(w, 'created', `Angefordert von ${p.name}` + (seasons.length ? `: Staffel ${seasons.join(', ')}` : ''));
  S.wishes.unshift(w);
  submitWish(w);
  save();
  const msg = { queued: `«${esc(t.title)}» ist vorgemerkt. Slouchcat übermittelt den Wunsch, sobald Jellyseerr erreichbar ist.`, pending: `«${esc(t.title)}» ist angefordert und wartet auf die Freigabe.`, processing: `«${esc(t.title)}» ist angefordert und wird beschafft.`, failed: `«${esc(t.title)}» konnte nicht angefordert werden: ${esc(FAIL_TEXT[w.failReason] || '')}` }[w.state];
  toast(msg + ' <a href="#/wishes">Meine Wünsche</a>', w.state === 'failed' ? 'bad' : w.state === 'queued' ? 'warn' : 'ok', 7000);
  return w;
}
function wishById(id) { return S.wishes.find((w) => w.id === id); }
function approveWish(id, by = 'in der Verwaltung') {
  const w = wishById(id); if (!w || w.state !== 'pending') return;
  w.state = 'processing'; ev(w, 'approved', `Freigegeben ${by}`); logSeerr('Freigeben', 'ok', `«${T[w.titleKey].title}»`);
  if (w.owner) notify(w.owner, 'wish_approved', w); save();
}
function declineWish(id, by = 'in der Verwaltung') {
  const w = wishById(id); if (!w || w.state !== 'pending') return;
  w.state = 'declined'; w.closed = Date.now(); ev(w, 'declined', `Abgelehnt ${by}`); logSeerr('Ablehnen', 'ok', `«${T[w.titleKey].title}»`);
  [w.owner, ...w.joined].filter(Boolean).forEach((pid) => notify(pid, 'wish_declined', w)); save();
}
function failWish(id) {
  const w = wishById(id); if (!w || !['processing', 'partial'].includes(w.state)) return;
  w.state = 'failed'; w.failReason = 'arr'; w.closed = Date.now(); ev(w, 'failed', FAIL_TEXT.arr);
  [w.owner, ...w.joined].filter(Boolean).forEach((pid) => notify(pid, 'wish_failed', w)); save();
}
/* the file arrives: Slouchcat scans, matches, and only then calls the wish fulfilled */
function arriveWish(id) {
  const w = wishById(id); if (!w || !['processing', 'partial'].includes(w.state)) return;
  const t = T[w.titleKey];
  const lib = (S.library[w.titleKey] = S.library[w.titleKey] || { present: false, seasons: [] });
  lib.present = true;
  const people = [w.owner, ...w.joined].filter(Boolean);
  if (w.media === 'show' && w.seasons.length) {
    const next = w.seasons.find((s) => w.seasonState[s] !== 'available');
    w.seasonState[next] = 'available';
    if (!lib.seasons.includes(next)) lib.seasons.push(next);
    const left = w.seasons.filter((s) => w.seasonState[s] !== 'available');
    ev(w, 'available', `Staffel ${next} ist in Slouchcat verfügbar`);
    if (left.length) { w.state = 'partial'; people.forEach((pid) => notify(pid, 'wish_season_available', w, { season: next })); }
    else { w.state = 'available'; w.closed = Date.now(); people.forEach((pid) => notify(pid, w.seasons.length > 1 ? 'wish_available' : 'wish_season_available', w, { season: w.seasons.length > 1 ? undefined : next })); }
  } else {
    if (w.media === 'show') lib.seasons = airedSeasons(t).map((s) => s.n);
    w.state = 'available'; w.closed = Date.now(); ev(w, 'available', 'In Slouchcat verfügbar');
    people.forEach((pid) => notify(pid, 'wish_available', w));
  }
  logSeerr('Abgleich', 'ok', `«${t.title}»: verfügbar`);
  save();
}
function withdrawWish(id) {
  const w = wishById(id), p = curProfile(); if (!w) return;
  if (w.joined.includes(p.id)) { w.joined = w.joined.filter((x) => x !== p.id); ev(w, 'left', `${p.name} wünscht nicht mehr mit`); }
  else if (w.owner === p.id && ['queued', 'pending'].includes(w.state)) {
    if (w.joined.length) { w.owner = w.joined.shift(); ev(w, 'handover', `${p.name} hat zurückgezogen, ${profile(w.owner).name} übernimmt`); }
    else { w.state = 'withdrawn'; w.closed = Date.now(); ev(w, 'withdrawn', `Zurückgezogen von ${p.name}`); }
  }
  save();
}
function retryWish(id) {
  const w = wishById(id); if (!w) return;
  if (w.state === 'queued') { submitWish(w); save(); return w; }
  const p = profile(w.owner) || curProfile();
  const prev = S.session.profileId; S.session.profileId = p.id; S.session.accountId = p.accountId;
  const nw = requestWish(w.titleKey, w.seasons);
  const back = profile(prev); S.session.profileId = prev; S.session.accountId = back.accountId; save();
  return nw;
}
function syncNow() {
  if (seerrState().k !== 'ok') { logSeerr('Abgleich', 'bad', seerrState().label); save(); return false; }
  const q = S.wishes.filter((w) => w.state === 'queued');
  q.forEach(submitWish);
  S.seerr.lastSync = Date.now();
  logSeerr('Abgleich', 'ok', `${S.wishes.filter((w) => OPEN_STATES.includes(w.state)).length} Anfragen geprüft` + (q.length ? `, ${q.length} übermittelt` : ', 0 geändert'));
  save();
  return true;
}
/* one step forward in the life of a wish (mockup control «Vorspulen») */
function advanceWish(id) {
  const w = wishById(id); if (!w) return;
  if (w.state === 'queued') { if (seerrState().k !== 'ok') { toast('Jellyseerr ist im Mockup noch nicht erreichbar. Stelle den Zustand auf «Verbunden».', 'warn'); return; } submitWish(w); save(); }
  else if (w.state === 'pending') approveWish(id, 'in Jellyseerr');
  else if (w.state === 'processing' || w.state === 'partial') arriveWish(id);
}
function switchProfile(pid) { const p = profile(pid); S.session = { accountId: p.accountId, profileId: p.id }; ui.hero = { i: 0, paused: false }; save(); applyAccent(); }
function applyAccent() { const p = curProfile(); const r = document.documentElement.style; r.setProperty('--accent', p.accent); r.setProperty('--on-accent', p.on); }
