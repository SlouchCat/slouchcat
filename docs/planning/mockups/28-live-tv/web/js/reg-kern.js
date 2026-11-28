/* Web, Kern (Musterseiten).
   Register entries of this agent (see shared/live-register.js). Keep «stand» up to date: 'offen' or 'gebaut'. */
'use strict';
LIVE_REG.add('kern', 'web/js/page-ls02.js, web/js/page-la02.js, web/js/shell.js', [
  ["LS-02", "Senderübersicht", "web", "/live/channels", null, "gebaut", ""],
  ["LS-20", "Hauptnavigation mit «Live-TV»", "web", null, "#/live/channels", "gebaut", "Seitenleiste und Menü-Schublade, Eintrag «Live-TV» in «Meine Medien»"],
  ["LA-02", "Quellen", "admin", "/admin/live/sources", null, "gebaut", ""],
  ["LA-25", "Seitenleiste und Suchpalette", "admin", null, "#/admin/live/sources", "gebaut", "Bereich «Live-TV» im Baum, Pillen, Suchpalette Strg+K"],
]);
