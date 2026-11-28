/* Konzept 28 «Live-TV», web mockup: the two frames (viewer app and administration), the main navigation
   with the new entry «Live-TV» (LS-20), the Live-TV tab bar, the administration side navigation as a
   tree with the new area «Live-TV» and its pills (LA-25), the search palette (Strg+K) and the pages
   the concept does not change. Owned by the core. */
'use strict';

const LOGO = '<svg class="logo" viewBox="0 0 32 32" aria-hidden="true"><path fill="currentColor" d="M7 6l5 4h8l5-4v14a6 6 0 0 1-6 6h-6a6 6 0 0 1-6-6z"/><path fill="var(--bg)" d="M13 18h6l-3 4z"/></svg>';
/* counts of the test library, the same in all three clients (Filme 1'846, Webradio 3 Stationen) */
const LIBS = [['Filme', 'film', "1'846"], ['Serien', 'tv', '505'], ['User-Empfehlungen', 'star', '3'], ['Podcast', 'mic', '100'], ['Hörbücher', 'book', '89'], ['Anime', 'film', '114'], ['Dokus', 'film', '63'], ['Radio', 'radio', '3 Stationen'], ['Sammlungen', 'layers', '337']];
const roleLabel = (p) => (p.primary ? 'Hauptprofil' : p.age ? `Profil · ab ${p.age}` : 'Profil');
const avatarHtml = (p, cls) => `<span class="av ${esc(p.accent)}${cls ? ' ' + cls : ''}">${esc(p.name[0])}</span>`;

/* ---------- viewer app ---------- */
function viewerNav(nav) {
  const p = SIM.profile(); const r = SIM.rights();
  const ent = (key, href, ic, label, cnt, hot, isNew) => `<li><a class="ent" href="${href}" ${nav === key ? 'aria-current="page"' : ''} data-fid="nav:${key}">${icon(ic, 16)}<span class="lb">${label}</span>${isNew ? '<span class="newtag">NEU</span>' : ''}${cnt != null && cnt !== '' ? `<span class="cnt ${hot ? 'hot' : ''}">${cnt}</span>` : ''}</a></li>`;
  const pl = SIM.player();
  const live = r.liveTv ? ent('live', '#/live', 'live', 'Live-TV', pl ? 'Live' : `${SIM.channels().length} Sender`, !!pl) : '';
  const libs = LIBS.map(([l, ic, c]) => (l === 'Radio' ? ent('radio', '#/radio', ic, l, c) + live : ent('lib:' + l, '#/unchanged/' + encodeURIComponent(l.toLowerCase()), ic, l, c))).join('');
  return `<div class="pcard">${avatarHtml(p)}<span class="pt"><b>${esc(p.name)}</b><span>${roleLabel(p)}</span></span><a class="swi" href="#/unchanged/profilwahl" data-fid="nav:switch">Wechseln</a></div>
  <ul>${ent('start', '#/', 'home', 'Start')}${ent('search', '#/search', 'search', 'Suche')}${ent('watchlist', '#/unchanged/merkliste', 'bookmark', 'Merkliste')}${ent('watched', '#/unchanged/gesehen', 'check', 'Gesehen')}${ent('wishes', '#/unchanged/wuensche', 'wish', 'Wünsche')}</ul>
  <div class="band">${icon('film', 16)}<span>Meine Medien</span></div>
  <ul>${libs}</ul>
  <div class="band">${icon('settings', 16)}<span>Konto</span></div>
  <ul>${ent('settings', '#/unchanged/einstellungen', 'settings', 'Einstellungen')}${ent('notif', '#/settings/notifications', 'bell', 'Benachrichtigungen')}${p.primary ? ent('profiles', '#/settings/profiles', 'users', 'Profile') : ''}${p.primary ? ent('admin', '#/admin/live', 'shield', 'Verwaltung') : ''}</ul>
  <ul class="end"><li><button type="button" class="ent quiet" data-act="hideSide" data-fid="nav:hide"><span class="lb">Seitenleiste ${S.sideHidden ? 'einblenden' : 'ausblenden'}</span></button></li></ul>`;
}
function headerHtml(admin, view) {
  const p = SIM.profile();
  return `<header class="hd ${admin ? 'admin' : ''}">
    <button type="button" class="btn ghost icon menu" ${!admin && S.sideHidden ? 'style="display:inline-flex"' : ''} data-act="${!admin && S.sideHidden ? 'hideSide' : 'drawer'}" aria-label="Menü" data-fid="hd:menu">${icon('menu')}</button>
    <a class="brand" href="${admin ? '#/admin' : '#/'}" data-fid="hd:brand">${LOGO}<span><span class="name">Slouchcat</span>${admin ? '<span class="tag">CONTROL CENTER</span>' : ''}</span></a>
    ${admin ? `<nav class="crumbs" aria-label="Brotkrumen"><a href="#/admin" data-fid="hd:crumb">Verwaltung</a><span class="sl">›</span><b>${esc(view.crumb || view.title || '')}</b></nav>` : ''}
    <div class="acc">
      ${admin ? `<button type="button" class="finder" data-act="dialog" data-name="finder" data-fid="hd:finder">${icon('search', 16)}<span>Bereich suchen</span><kbd>Strg K</kbd></button>` : `<a class="inbox" href="#/search" aria-label="Suche öffnen" data-fid="hd:search">${icon('search')}<span class="t">Suche</span></a>`}
      <a class="inbox" href="#/notifications" aria-label="Benachrichtigungen, 2 ungelesen" data-fid="hd:inbox">${icon('bell')}<span class="t">Benachrichtigungen</span><span class="badge">2</span></a>
      ${admin ? `<span class="prof"><span class="av a1">A</span><span class="pn">Admin</span></span>` : `<a class="prof" href="#/unchanged/profilwahl" aria-label="Profil wechseln, aktuell ${esc(p.name)}" data-fid="hd:prof">${avatarHtml(p)}<span class="pn">${esc(p.name)}</span></a>`}
    </div></header>`;
}

/* ---------- Live-TV area: tab bar and page frame (used by every viewer page under /live) ---------- */
/** tab bar of the Live-TV area; active: start | channels | guide | recordings | reminders */
function liveTabs(active) {
  const r = SIM.rights();
  const tabs = [['start', 'Start', '#/live'], ['channels', 'Sender', '#/live/channels'], ['guide', 'Programm', '#/live/guide'], r.recordings ? ['recordings', 'Aufnahmen', '#/live/recordings'] : null, ['reminders', 'Erinnerungen', '#/live/reminders']].filter(Boolean);
  return `<nav class="ltabs" aria-label="Bereiche von Live-TV"><ul>${tabs.map(([k, l, h]) => `<li><a href="${h}" ${k === active ? 'aria-current="page"' : ''} data-fid="lt:${k}">${l}</a></li>`).join('')}</ul></nav>`;
}
/** frame of a Live-TV page: { tab, title, count, sub, actions, html, wide } → html for view.html */
function livePage(o) {
  return `<div class="page live${o.wide ? ' wide' : ''}"><div class="lhead"><p class="eyebrow-live">${icon('live', 14)}Live-TV</p>${liveTabs(o.tab)}</div>
    <div class="head"><div class="ht"><h1>${o.title}</h1>${o.count ? `<span class="lcount">${o.count}</span>` : ''}</div>${o.sub ? `<p class="sub">${o.sub}</p>` : ''}${o.actions ? `<div class="acts">${o.actions}</div>` : ''}</div>
    ${o.html}${LU.sampleNote()}</div>`;
}

/* ---------- administration: areas with their pages (tabs) ---------- */
const AREAS = [
  { key: 'overview', label: 'Übersicht', icon: 'home', pages: [] },
  { key: 'media', label: 'Medien', icon: 'film', pages: [['libraries', 'Bibliotheken'], ['metadata', 'Metadaten'], ['maintenance', 'Pflege'], ['wishes', 'Wünsche'], ['podcasts', 'Podcasts'], ['radio', 'Radio'], ['audiobooks', 'Hörbuch-Quellen']] },
  { key: 'livetv', label: 'Live-TV', icon: 'live', isNew: true, pages: [['live', 'Übersicht', '#/admin/live', 'home'], ['sources', 'Quellen', '#/admin/live/sources', 'list'], ['channels', 'Sender', '#/admin/live/channels', 'tv'], ['epg', 'EPG', '#/admin/live/epg', 'calendar'], ['reception', 'Empfang', '#/admin/live/reception', 'wifi'], ['live-streams', 'Streams', '#/admin/live/streams', 'play'], ['diagnostics', 'Diagnose', '#/admin/live/diagnostics', 'alert'], ['storage', 'Aufnahmen und Speicher', '#/admin/live/storage', 'database']] },
  { key: 'people', label: 'Nutzer & Geräte', icon: 'users', pages: [['accounts', 'Konten', '#/admin/accounts'], ['devices', 'Geräte'], ['streams', 'Streams', '#/admin/streams']] },
  { key: 'home', label: 'Startseite', icon: 'layers', pages: [['home-rows', 'Startseite'], ['spotlight', 'Spotlight'], ['picks', 'User-Empfehlungen'], ['collections', 'Sammlungen'], ['my-media', 'Meine Medien', '#/admin/my-media'], ['announcements', 'Ankündigungen']] },
  { key: 'automation', label: 'Automatisierung', icon: 'zap', pages: [['jobs', 'Hintergrundjobs', '#/admin/jobs'], ['integrations', 'Integrationen'], ['notifications', 'Benachrichtigungen', '#/admin/notifications']] },
  { key: 'system', label: 'System', icon: 'server', pages: [['stats', 'Statistiken'], ['backup', 'Sicherung', '#/admin/backup'], ['server', 'Server', '#/admin/server']] },
];
const pageHref = (pg) => pg[2] || '#/admin/x/' + pg[0];
const areaHref = (a) => (a.key === 'overview' ? '#/admin' : pageHref(a.pages[0]));
/** area and page key of an admin route (for placeholders and pages) */
function adminAreaOf(route) {
  if (!route || route === '/admin') return { area: 'overview', page: '' };
  for (const a of AREAS) for (const pg of a.pages) { const h = pageHref(pg).slice(1); if (route === h || (route.startsWith(h + '/') && h !== '/admin/live')) return { area: a.key, page: pg[0] }; }
  if (route.startsWith('/admin/live')) return { area: 'livetv', page: 'live' };
  return { area: 'overview', page: '' };
}
/** alarm pills of the Live-TV area (computed from the sample data and the simulation) */
function livePills() {
  const bad = LIVE.diagnostics.now.length + (SIM.flags().disturbed ? 1 : 0);
  return {
    area: bad ? [`${bad} gestört`, 'warn'] : null,
    sources: LIVE.sources.some((s) => !s.lastOk) ? ['Fehler', 'bad'] : null,
    diagnostics: bad ? [`${bad} gestört`, 'warn'] : null,
    storage: ['full', 'unmounted'].includes(SIM.flags().admStorage) ? [SIM.flags().admStorage === 'full' ? 'Speicher voll' : 'Netzlaufwerk fehlt', 'bad'] : SIM.flags().admStorage === 'low' || LIVE.accounts.some((a) => a.quotaGB && a.usedGB / a.quotaGB >= 0.9) ? ['Speicher knapp', 'warn'] : null,
  };
}
const pill = (x) => (x ? `<span class="apill ${x[1]}" aria-hidden="true">${esc(x[0])}</span>` : '');
function adminNav(area, page) {
  const pills = livePills();
  const tree = AREAS.filter((a) => a.key !== 'overview').map((a) => {
    const open = ui.tree[a.key] ?? (a.key === area);
    const cur = a.key === area;
    const p = a.key === 'livetv' ? pills.area : null;
    return `<li class="br${cur ? ' cur' : ''}"><button type="button" class="ent brb" data-act="tree" data-id="${a.key}" aria-expanded="${open}" data-fid="an:${a.key}"${p ? ` aria-label="${esc(a.label)}, ${esc(p[0])}"` : ''}>${icon(a.icon, 16)}<span class="lb">${a.label}</span>${pill(p) || (a.isNew ? '<span class="newtag">NEU</span>' : '')}<span class="chev">${icon('chevR', 14)}</span></button>
      <ul class="sub"${open ? '' : ' hidden'}>${a.pages.map((pg) => { const pp = a.key === 'livetv' ? pills[pg[0]] : null; return `<li><a class="ent ind" href="${pageHref(pg)}" ${cur && pg[0] === page ? 'aria-current="page"' : ''} data-fid="an:${a.key}:${pg[0]}"${pp ? ` aria-label="${esc(pg[1])}, ${esc(pp[0])}"` : ''}><span class="lb">${pg[1]}</span>${pill(pp)}</a></li>`; }).join('')}</ul></li>`;
  }).join('');
  return `<ul><li><a class="ent" href="#/live" data-fid="an:back">${icon('arrowL', 16)}<span class="lb">Zurück zur App</span></a></li></ul>
  <div class="hdg">Verwaltung</div>
  <ul class="tree"><li><a class="ent" href="#/admin" ${area === 'overview' ? 'aria-current="page"' : ''} data-fid="an:overview">${icon('home', 16)}<span class="lb">Übersicht</span></a></li>${tree}</ul>
  <div class="who"><span class="k">A</span><span><b>Admin</b><span>Server-Admin · admin@example.ch</span></span></div>`;
}
ACT.tree = (d) => { const a = AREAS.find((x) => x.key === d.id); const cur = document.documentElement.dataset.area === d.id; ui.tree[d.id] = !(ui.tree[d.id] ?? cur); render(); if (ui.drawer) renderOverlays(); void a; };
function areaTabs(area, page) {
  const a = AREAS.find((x) => x.key === area);
  if (!a || !a.pages.length) return '';
  const pills = area === 'livetv' ? livePills() : {};
  return `<nav class="atabs" aria-label="Seiten des Bereichs ${a.label}"><ul>${a.pages.map((pg) => `<li><a href="${pageHref(pg)}" ${pg[0] === page ? 'aria-current="page"' : ''} data-fid="at:${pg[0]}">${pg[1]}${pills[pg[0]] ? `<span class="c ${pills[pg[0]][1] === 'bad' ? 'hot bad' : 'hot'}">${esc(pills[pg[0]][0])}</span>` : ''}</a></li>`).join('')}</ul></nav>`;
}
/** sub tabs inside a page (?tab=): base hash, [[key, label, count]], active key */
function subTabs(base, tabs, active) {
  return `<nav class="subtabs" aria-label="Unterseiten"><ul>${tabs.map(([k, l, c]) => `<li><a href="${base}${base.includes('?') ? '&' : '?'}tab=${k}" ${k === active ? 'aria-current="page"' : ''} data-fid="st:${k}">${l}${c != null ? ` <span class="c">${c}</span>` : ''}</a></li>`).join('')}</ul></nav>`;
}

const SHELL = {
  viewer(view) {
    return `<div class="app">${headerHtml(false, view)}<div class="shell">${S.sideHidden ? '' : `<nav class="sn" aria-label="Hauptnavigation">${viewerNav(view.nav)}</nav>`}<main class="main" id="content">${view.html}</main></div></div>`;
  },
  admin(view) {
    const a = AREAS.find((x) => x.key === view.area) || AREAS[0];
    const pg = a.pages.find((x) => x[0] === view.page);
    document.documentElement.dataset.area = a.key;
    const eyebrow = view.eyebrow || (pg ? `${a.label} / ${pg[1]}` : a.label);
    return `<div class="app adm">${headerHtml(true, view)}<div class="shell"><nav class="asn" aria-label="Verwaltung">${adminNav(view.area, view.page)}</nav><main class="amain" id="content">
      ${areaTabs(view.area, view.page)}
      ${view.back ? `<div><a class="back" href="${view.back[0]}" data-fid="a:back">${icon('chevL', 16)}${view.back[1]}</a></div>` : ''}
      <header class="ahead"><div class="hg"><p class="eye">${eyebrow}</p><h1>${view.h1 || view.title}</h1>${view.intro ? `<p class="intro">${view.intro}</p>` : ''}</div>${view.live ? `<p class="alive"><span class="dot ok"></span>${esc(view.live)}</p>` : ''}${view.actions ? `<div class="ha">${view.actions}</div>` : ''}</header>
      ${view.html}${LU.sampleNote('adm-sample')}</main></div></div>`;
  },
  bare(view) { return `<div class="app">${view.html}</div>`; },
  drawer() {
    const adm = document.documentElement.dataset.shell === 'admin';
    const { path } = parseHash();
    const a = adminAreaOf(path);
    return adm ? `<nav class="asn drawer" aria-label="Verwaltung">${adminNav(a.area, a.page)}</nav>` : `<nav class="sn drawer" aria-label="Hauptnavigation">${viewerNav(path.startsWith('/live') ? 'live' : path === '/' ? 'start' : path.slice(1))}</nav>`;
  },
};
ACT.hideSide = () => { S.sideHidden = !S.sideHidden; commit(); };

/* ---------- pages the concept does not change: reachable, explained, never a dead button ---------- */
const UNCHANGED = {
  merkliste: ['Merkliste', 'Titel, die du dir gemerkt hast.'],
  gesehen: ['Gesehen', 'Was dieses Profil gesehen hat.'],
  einstellungen: ['Einstellungen', 'Tonsprachen, Untertitel und Serienmerker des Profils.'],
  wuensche: ['Wünsche', 'Wünsche über Seerr (Konzept 25).'],
  profilwahl: ['Wer schaut?', 'Die Profilwahl. Im Mockup wechselst du das Profil oben in der Leiste unter «Zustände».'],
};
route('/unchanged/:what', (pm) => {
  const u = UNCHANGED[pm.what] || [decodeURIComponent(pm.what).replace(/^./, (c) => c.toUpperCase()), 'Bibliotheksseite mit Raster, Filter und Sortierung.'];
  /* no word about Live-TV in the app picture: the remark «unverändert durch Konzept 28» belongs to the mockup bar */
  return { type: 'viewer', nav: { merkliste: 'watchlist', gesehen: 'watched', einstellungen: 'settings', wuensche: 'wishes' }[pm.what] || 'lib:' + u[0], title: u[0], html: `<div class="page"><div class="head"><h1>${esc(u[0])}</h1><p class="sub">${u[1]}</p></div><div class="empty">Diese Seite bleibt, wie sie ist.<div class="row-gap" style="justify-content:center;margin-top:12px"><button type="button" class="btn secondary" data-act="back" data-fid="un:back">${icon('arrowL')}Zurück</button></div></div></div>` };
});
route('/admin/x/:page', (pm) => {
  const a = AREAS.find((x) => x.pages.some((p) => p[0] === pm.page)) || AREAS[1];
  const pg = a.pages.find((p) => p[0] === pm.page);
  return { type: 'admin', area: a.key, page: pm.page, title: pg ? pg[1] : pm.page, intro: 'Diese Seite ändert sich durch Konzept 28 «Live-TV» nicht.', html: `<div class="unchanged"><b>Unverändert</b><p>Alle heutigen Funktionen dieser Seite bleiben, wie sie sind. Das Mockup zeigt nur Seiten, die neu sind oder sich ändern.</p><div class="row-gap"><a class="btn secondary" href="#/admin/live" data-fid="unx:live">Zu Live-TV</a></div></div>` };
});

/* ---------- search palette (Strg+K): pages, and from three letters channels and sources ---------- */
DIALOGS.finder = () => {
  const q = (ui.q2 || '').toLowerCase();
  const pages = [['Übersicht', '#/admin'], ...AREAS.flatMap((a) => a.pages.map((pg) => [`${a.label} › ${pg[1]}`, pageHref(pg)])),
    ['Live-TV › Sender › Gruppen', '#/admin/live/channels?tab=groups'], ['Live-TV › Sender › Duplikate', '#/admin/live/channels?tab=duplicates'], ['Live-TV › EPG › Zuordnungen', '#/admin/live/epg?tab=mapping'], ['Live-TV › Aufnahmen und Speicher › Timeshift', '#/admin/live/storage?tab=timeshift']];
  let hits = pages.filter(([l]) => l.toLowerCase().includes(q));
  if (q.length >= 3) {
    hits = hits.concat(LIVE.channels.filter((c) => c.name.toLowerCase().includes(q)).map((c) => [`${c.name} › Sender bearbeiten`, `#/admin/live/channels?tab=channels&edit=${c.id}`]),
      LIVE.sources.filter((s) => s.name.toLowerCase().includes(q)).map((s) => [`${s.name} › Quelle`, `#/admin/live/sources/${s.id}`]));
  }
  return { title: 'Bereich suchen', body: `<input class="inp" data-bind="ui.q2" data-live data-fid="finder:q" data-autofocus placeholder="Seite, Sender oder Quelle" aria-label="Seite, Sender oder Quelle" value="${esc(ui.q2 || '')}"><ul class="stack" style="gap:4px">${hits.slice(0, 14).map(([l, h]) => `<li><a class="btn ghost wide" style="justify-content:flex-start" href="${h}" data-act="close" data-fid="finder:${h}">${esc(l)}</a></li>`).join('') || '<li class="muted">Nichts gefunden.</li>'}</ul>`, actions: `<button type="button" class="btn secondary" data-act="close" data-fid="finder:x">Schliessen</button>` };
};
document.addEventListener('keydown', (e) => { if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k' && document.documentElement.dataset.shell === 'admin') { e.preventDefault(); openDialog('finder'); } });
