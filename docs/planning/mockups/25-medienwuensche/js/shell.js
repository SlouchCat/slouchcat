/* Frames of the two shells: viewer app and administration. Pages return a view object, the shell wraps it. */
'use strict';

const LOGO = '<svg class="logo" viewBox="0 0 32 32" aria-hidden="true"><path fill="currentColor" d="M7 6l5 4h8l5-4v14a6 6 0 0 1-6 6h-6a6 6 0 0 1-6-6z"/><path fill="#0b0c10" d="M13 18h6l-3 4z"/></svg>';
const LIBS = [['Filme', 'film', '1840'], ['Serien', 'tv', '505'], ['User-Empfehlungen', 'star', '3'], ['Podcast', 'mic', '100'], ['Hörbücher', 'book', '89'], ['Anime', 'film', '114'], ['Dokus', 'film', '63'], ['Radio', 'radio', '2 Sender'], ['Sammlungen', 'layers', '337']];

function viewerNav(nav) {
  const p = curProfile();
  const mine = myWishes(p).filter((w) => OPEN_STATES.includes(w.state)).length;
  const showWishes = canRequest(p).ok || myWishes(p).length > 0;
  const ent = (key, href, ic, label, cnt, hot) => `<li><a class="ent" href="${href}" ${nav === key ? 'aria-current="page"' : ''} data-fid="nav:${key}">${icon(ic, 16)}<span class="lb">${label}</span>${cnt != null && cnt !== '' ? `<span class="cnt ${hot ? 'hot' : ''}">${cnt}</span>` : ''}</a></li>`;
  return `<div class="pcard"><span class="av">${esc(p.name[0])}</span><span class="pt"><b>${esc(p.name)}</b><span>${roleLabel(p)}</span></span><a class="swi" href="#/profiles" data-fid="nav:switch">Wechseln</a></div>
  <ul>${ent('start', '#/', 'home', 'Start')}${ent('search', '#/search', 'search', 'Suche')}${ent('watchlist', '#/unchanged/merkliste', 'bookmark', 'Merkliste')}${ent('watched', '#/unchanged/gesehen', 'check', 'Gesehen')}${showWishes ? ent('wishes', discoverOn(p) ? '#/discover' : '#/wishes', 'wish', 'Wünsche', mine || '', true) : ''}</ul>
  <div class="band">${icon('film', 16)}<span>Meine Medien</span></div>
  <ul>${LIBS.map(([l, ic, c]) => ent('lib:' + l, '#/unchanged/' + encodeURIComponent(l.toLowerCase()), ic, l, c)).join('')}</ul>
  <div class="band">${icon('settings', 16)}<span>Konto</span></div>
  <ul>${ent('settings', '#/unchanged/einstellungen', 'settings', 'Einstellungen')}${p.primary ? ent('profiles', '#/settings/profiles', 'users', 'Profile') : ''}${isAdmin() ? ent('admin', '#/admin', 'shield', 'Verwaltung') : ''}</ul>
  <ul class="end"><li><a class="ent" href="#/profiles" data-fid="nav:logout">${icon('logout', 16)}<span class="lb">Abmelden</span></a></li><li><button type="button" class="ent quiet" data-act="hideSide" data-fid="nav:hide"><span class="lb">Seitenleiste ${S.sideHidden ? 'einblenden' : 'ausblenden'}</span></button></li></ul>`;
}
function headerHtml(admin, view) {
  const p = curProfile();
  const n = unread(p);
  return `<header class="hd ${admin ? 'admin' : ''}">
    <button type="button" class="btn ghost icon menu" ${!admin && S.sideHidden ? 'style="display:inline-flex"' : ''} data-act="${!admin && S.sideHidden ? 'hideSide' : 'drawer'}" aria-label="Menü" data-fid="hd:menu">${icon('menu')}</button>
    <a class="brand" href="${admin ? '#/admin' : '#/'}" data-fid="hd:brand">${LOGO}<span><span class="name">Slouchcat</span>${admin ? '<span class="tag">CONTROL CENTER</span>' : ''}</span></a>
    ${admin ? `<nav class="crumbs" aria-label="Brotkrumen"><a href="#/admin" data-fid="hd:crumb">Verwaltung</a><span class="sl">›</span><b>${esc(view.crumb || view.title || '')}</b></nav>` : ''}
    <div class="acc">
      ${admin ? `<button type="button" class="finder" data-act="dialog" data-name="finder" data-fid="hd:finder">${icon('search', 16)}<span>Bereich suchen</span><kbd>Strg K</kbd></button>` : `<a class="inbox" href="#/search" aria-label="Suche öffnen" data-fid="hd:search">${icon('search')}<span class="t">Suche</span></a>`}
      <a class="inbox" href="#/notifications" aria-label="Benachrichtigungen${n ? ', ' + n + ' ungelesen' : ''}" data-fid="hd:inbox">${icon('bell')}<span class="t">Benachrichtigungen</span>${n ? `<span class="badge">${n}</span>` : ''}</a>
      <a class="prof" href="#/profiles" aria-label="Profil wechseln, aktuell ${esc(p.name)}" data-fid="hd:prof"><span class="av">${esc(p.name[0])}</span><span class="pn">${esc(p.name)}</span></a>
    </div></header>`;
}

/* administration: six areas; each area's pages are the area tabs */
const AREAS = [
  { key: 'overview', label: 'Übersicht', icon: 'home', pages: [] },
  { key: 'media', label: 'Medien', icon: 'film', pages: [['libraries', 'Bibliotheken'], ['metadata', 'Metadaten'], ['maintenance', 'Pflege'], ['wishes', 'Wünsche'], ['podcasts', 'Podcasts'], ['radio', 'Radio'], ['audiobooks', 'Hörbuch-Quellen']] },
  { key: 'people', label: 'Nutzer & Geräte', icon: 'users', pages: [['accounts', 'Konten'], ['devices', 'Geräte'], ['streams', 'Streams']] },
  { key: 'home', label: 'Startseite', icon: 'layers', pages: [['home-rows', 'Startseite'], ['spotlight', 'Spotlight'], ['picks', 'User-Empfehlungen'], ['collections', 'Sammlungen'], ['my-media', 'Meine Medien'], ['announcements', 'Ankündigungen']] },
  { key: 'automation', label: 'Automatisierung', icon: 'zap', pages: [['jobs', 'Hintergrundjobs'], ['integrations', 'Integrationen'], ['notifications', 'Benachrichtigungen']] },
  { key: 'system', label: 'System', icon: 'server', pages: [['stats', 'Statistiken'], ['backup', 'Sicherung'], ['server', 'Server']] },
];
const ADMIN_BUILT = ['wishes', 'accounts', 'home-rows', 'spotlight', 'jobs', 'integrations', 'notifications'];
const adminHref = (page) => (ADMIN_BUILT.includes(page) ? '#/admin/' + page : '#/admin/x/' + page);
function areaHref(a) { return a.key === 'overview' ? '#/admin' : adminHref(a.pages[0][0]); }
function adminNav(area) {
  const p = curProfile();
  const wc = wishCounts();
  return `<ul><li><a class="ent" href="#/" data-fid="an:back">${icon('arrowL', 16)}<span class="lb">Zurück zur App</span></a></li></ul>
  <div class="hdg">Verwaltung</div>
  <ul>${AREAS.map((a) => `<li><a class="ent" href="${areaHref(a)}" ${a.key === area ? 'aria-current="page"' : ''} data-fid="an:${a.key}">${icon(a.icon, 16)}<span class="lb">${a.label}</span>${a.key === 'media' ? `<span class="cnt" title="Pflegefälle${wc.pending ? ' und Wünsche, die auf Freigabe warten' : ''}">${268 + wc.pending}</span>` : ''}</a></li>`).join('')}</ul>
  <div class="who"><span class="k">${esc(p.name[0])}</span><span><b>${esc(p.name)}</b><span>${roleLabel(p)}</span></span></div>`;
}
function areaTabs(area, page) {
  const a = AREAS.find((x) => x.key === area);
  if (!a || !a.pages.length) return '';
  const wc = wishCounts(), mc = mappingCounts();
  const extra = { wishes: wc.pending ? `<span class="c hot">${wc.pending}</span>` : '<span class="new">NEU</span>', integrations: (() => { const c = typeof aaIntCounts === 'function' ? aaIntCounts() : { set: 8, total: 9 }; return `<span class="c">${c.set} / ${c.total}</span>`; })(), maintenance: '<span class="c">268</span>', accounts: `<span class="c">${S.accounts.length}</span>` };
  return `<nav class="atabs" aria-label="Seiten des Bereichs ${a.label}"><ul>${a.pages.map(([k, l]) => `<li><a href="${adminHref(k)}" ${k === page ? 'aria-current="page"' : ''} data-fid="at:${k}">${l}${extra[k] || ''}</a></li>`).join('')}</ul></nav>`;
}

const SHELL = {
  viewer(view) {
    return `<div class="app">${headerHtml(false, view)}<div class="shell">${S.sideHidden ? '' : `<nav class="sn" aria-label="Hauptnavigation">${viewerNav(view.nav)}</nav>`}<main class="main" id="content">${view.html}</main></div></div>`;
  },
  admin(view) {
    if (!isAdmin()) {
      return SHELL.viewer({ nav: '', html: `<div class="page"><div class="head"><h1>Nur für Administratoren</h1><p class="sub">Diesen Bereich verwaltet das Hauptprofil eines Administratorkontos. Aktiv ist ${esc(curProfile().name)}.</p><div class="acts"><a class="btn secondary" href="#/profiles">Profil wechseln</a><a class="btn ghost" href="#/">Zur Startseite</a></div></div></div>` });
    }
    const a = AREAS.find((x) => x.key === view.area) || AREAS[0];
    const pg = a.pages.find((x) => x[0] === view.page);
    const eyebrow = view.eyebrow || (pg ? `${a.label} / ${pg[1]}` : a.label);
    return `<div class="app adm">${headerHtml(true, view)}<div class="shell"><nav class="asn" aria-label="Verwaltung">${adminNav(view.area)}</nav><main class="amain" id="content">
      ${areaTabs(view.area, view.page)}
      ${view.back ? `<div><a class="back" href="${view.back[0]}" data-fid="a:back">${icon('chevL', 16)}${view.back[1]}</a></div>` : ''}
      <header class="ahead"><div class="hg"><p class="eye">${eyebrow}</p><h1>${view.h1 || view.title}</h1>${view.intro ? `<p class="intro">${view.intro}</p>` : ''}</div>${view.actions ? `<div class="ha">${view.actions}</div>` : ''}</header>
      ${view.html}</main></div></div>`;
  },
  bare(view) { return `<div class="app">${view.html}</div>`; },
  drawer() {
    const adm = document.documentElement.dataset.shell === 'admin' && isAdmin();
    const { path } = parseHash();
    return adm ? `<nav class="asn drawer" aria-label="Verwaltung">${adminNav('')}</nav>` : `<nav class="sn drawer" aria-label="Hauptnavigation">${viewerNav(path === '/' ? 'start' : path.startsWith('/discover') ? 'wishes' : path.slice(1))}</nav>`;
  },
};
ACT.hideSide = () => { S.sideHidden = !S.sideHidden; commit(); };

/* pages the concept does not change: reachable, explained, never a dead button */
const UNCHANGED = {
  merkliste: ['Merkliste', 'Titel, die du dir gemerkt hast. Fehlende Titel lassen sich nicht merken: dafür gibt es «Wünsche».'],
  gesehen: ['Gesehen', 'Was dieses Profil gesehen hat.'],
  einstellungen: ['Einstellungen', 'Tonsprachen, Untertitel und Serienmerker des Profils, dazu der Verweis auf die Benachrichtigungskanäle.'],
};
route('/unchanged/:what', (pm) => {
  const u = UNCHANGED[pm.what] || [pm.what.charAt(0).toUpperCase() + pm.what.slice(1), 'Bibliotheksseite mit Raster, Filter und Sortierung.'];
  return { type: 'viewer', nav: pm.what === 'merkliste' ? 'watchlist' : pm.what === 'gesehen' ? 'watched' : pm.what === 'einstellungen' ? 'settings' : 'lib:' + u[0], title: u[0], html: `<div class="page"><div class="head"><h1>${esc(u[0])}</h1><p class="sub">${u[1]}</p></div>${notice('', 'Unverändert', 'Diese Seite ändert sich durch das Konzept «Wünsche» nicht. Das Mockup zeigt sie deshalb nicht im Einzelnen.', `<button type="button" class="btn secondary" data-act="back" data-fid="un:back">${icon('arrowL')}Zurück</button><a class="btn ghost" href="#/mockup">Alle Mockup-Seiten</a>`)}</div>` };
});
const ADMIN_UNCHANGED = { libraries: 'Bibliotheken', metadata: 'Metadaten', maintenance: 'Pflege', podcasts: 'Podcasts', radio: 'Radio', audiobooks: 'Hörbuch-Quellen', devices: 'Geräte', streams: 'Streams', picks: 'User-Empfehlungen', collections: 'Sammlungen', 'my-media': 'Meine Medien', announcements: 'Ankündigungen', stats: 'Statistiken', backup: 'Sicherung', server: 'Server' };
route('/admin/x/:page', (pm) => {
  const a = AREAS.find((x) => x.pages.some((p) => p[0] === pm.page)) || AREAS[1];
  const name = ADMIN_UNCHANGED[pm.page] || pm.page;
  return { type: 'admin', area: a.key, page: pm.page, title: name, intro: 'Diese Seite ändert sich durch das Konzept «Wünsche» nicht.', html: `<div class="unchanged"><b>Unverändert</b><p>Alle heutigen Funktionen dieser Seite bleiben, wie sie sind. Das Mockup zeigt nur Seiten, die neu sind oder sich ändern.</p><div class="row-gap"><a class="btn secondary" href="#/mockup">Alle Mockup-Seiten</a></div></div>` };
});
DIALOGS.finder = () => {
  const q = (ui.q2 || '').toLowerCase();
  const all = [['Übersicht', '#/admin'], ...AREAS.flatMap((a) => a.pages.map(([k, l]) => [`${a.label} › ${l}`, adminHref(k)])), ['Automatisierung › Integrationen › Jellyseerr', '#/admin/integrations/jellyseerr'], ['Medien › Wünsche › Jellyseerr-Benutzer', '#/admin/wishes/mapping'], ['Medien › Wünsche › Einstellungen', '#/admin/wishes/settings']];
  const hits = all.filter(([l]) => l.toLowerCase().includes(q));
  return { title: 'Bereich suchen', body: `<input class="inp" data-bind="ui.q2" data-live data-fid="finder:q" data-autofocus placeholder="Seite der Verwaltung" aria-label="Seite der Verwaltung" value="${esc(ui.q2 || '')}"><ul class="stack" style="gap:4px">${hits.map(([l, h]) => `<li><a class="btn ghost wide" style="justify-content:flex-start" href="${h}" data-act="close" data-fid="finder:${h}">${esc(l)}</a></li>`).join('') || '<li class="muted">Nichts gefunden.</li>'}</ul>`, actions: `<button type="button" class="btn secondary" data-act="close" data-fid="finder:x">Schliessen</button>` };
};
document.addEventListener('keydown', (e) => { if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k' && document.documentElement.dataset.shell === 'admin') { e.preventDefault(); openDialog('finder'); } });
