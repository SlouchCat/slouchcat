/* Konzept 28 «Live-TV»: register of every page and dialog of the mockups (14-oberflaeche.md,
   15-administration.md). The entries come from one file per agent (js/reg-NAME.js in each client folder); each agent keeps
   only the «stand» of its own entries up to date. The clients build the jump menu and the placeholder
   pages from it; index.html shows the whole list.

   Entry: { id: 'LS-02', title, client: 'web'|'admin'|'tv'|'handy', route: '/live/channels' (pattern with
   :name, null = no page of its own), href: '#/live/channels' (example address), file, agent, stand:
   'gebaut'|'offen', note }. */
'use strict';
(function (g) {
  const list = [];
  const AGENTS = {
    kern: { name: 'Kern (Gerüst und Musterseiten)', files: ['shared/*', 'web/index.html', 'web/js/core.js', 'web/js/shell.js', 'web/js/dialogs.js', 'web/js/mock.js', 'web/js/boot.js', 'web/js/page-ls02.js', 'web/js/page-la02.js', 'web/css/app.css', 'web/css/viewer.css', 'web/css/admin.css', 'web/css/core.css', 'tv/index.html', 'tv/js/core.js', 'tv/js/live-core.js', 'tv/js/channels.js', 'tv/js/boot.js', 'tv/css/app.css', 'tv/css/live-core.css', 'handy/index.html', 'handy/js/core.js', 'handy/js/live-core.js', 'handy/js/channels.js', 'handy/js/boot.js', 'handy/css/app.css', 'handy/css/live-core.css', 'index.html', 'README.md', 'BAUKASTEN.md'] },
    'web-a': { name: 'Web Zuschauer A', files: ['web/js/viewer-a.js', 'web/js/viewer-a-2.js', 'web/css/viewer-a.css', 'web/js/reg-viewer-a.js'] },
    'web-b': { name: 'Web Zuschauer B', files: ['web/js/viewer-b.js', 'web/js/viewer-b-2.js', 'web/css/viewer-b.css', 'web/js/reg-viewer-b.js'] },
    'admin-a': { name: 'Web Verwaltung A', files: ['web/js/admin-a.js', 'web/js/admin-a-2.js', 'web/css/admin-a.css', 'web/js/reg-admin-a.js'] },
    'admin-b': { name: 'Web Verwaltung B', files: ['web/js/admin-b.js', 'web/js/admin-b-2.js', 'web/css/admin-b.css', 'web/js/reg-admin-b.js'] },
    tv: { name: 'TV', files: ['tv/js/pages.js', 'tv/js/pages-2.js', 'tv/js/pages-3.js', 'tv/css/pages.css', 'tv/js/reg-tv.js'] },
    handy: { name: 'Handy', files: ['handy/js/pages.js', 'handy/js/pages-2.js', 'handy/js/pages-3.js', 'handy/css/pages.css', 'handy/js/reg-handy.js'] },
  };
  const CLIENTS = { web: 'Web-App (Zuschauer)', admin: 'Web-App (Verwaltung)', tv: 'Android TV', handy: 'Handy-App (Handy und Tablet)' };
  /** add entries: add(agent, file, [[id, title, client, route, href, stand, note], …]) */
  function add(agent, file, rows) {
    rows.forEach(([id, title, client, route, href, stand, note]) => list.push({ id, title, client, route, href: href || (route ? '#' + route : null), file, agent, stand: stand || 'offen', note: note || '' }));
  }
  const forClient = (c) => list.filter((e) => (Array.isArray(c) ? c.includes(e.client) : e.client === c));
  const get = (id, client) => list.find((e) => e.id === id && (!client || e.client === client)) || null;
  /** order for the jump menu: LS, LD, LA, each by number */
  const order = (a, b) => { const k = (e) => ({ LS: 0, LD: 1, LA: 2 }[e.id.slice(0, 2)] * 1000 + Number(e.id.slice(3))); return k(a) - k(b); };
  g.LIVE_REG = { list, add, forClient, get, order, AGENTS, CLIENTS };
})(typeof window !== 'undefined' ? window : globalThis);
