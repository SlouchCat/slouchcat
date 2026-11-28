/* Web Zuschauer A.
   Register entries of this agent (see shared/live-register.js). Keep «stand» up to date: 'offen' or 'gebaut'. */
'use strict';
LIVE_REG.add('web-a', 'web/js/viewer-a.js', [
  ["LS-01", "Live-TV-Start", "web", "/live", null, "gebaut", ""],
  ["LS-03", "Meine Sender", "web", "/live/channels/favorites", null, "gebaut", ""],
  ["LS-05", "Programmsuche", "web", "/live/guide/search", "#/live/guide/search?q=tatort", "gebaut", ""],
  ["LS-06", "Sendungsdetails", "web", "/live/programs/:id", "#/live/programs/p-zdf-1215", "gebaut", ""],
  ["LS-15", "Senderprogramm", "web", "/live/guide/channel/:id", "#/live/guide/channel/srf1", "gebaut", ""],
  ["LS-21", "Profilformular: Block «Live-TV»", "web", "/settings/profiles", null, "gebaut", ""],
  ["LS-22", "Benachrichtigungen einstellen", "web", "/settings/notifications", null, "gebaut", ""],
  ["LS-23", "Benachrichtigungen (Glocke)", "web", "/notifications", null, "gebaut", ""],
  ["LS-24", "Suche: Verweis ins TV-Programm", "web", "/search", "#/search?q=tatort", "gebaut", ""],
  ["LS-25", "Ton-Vorrang und Mini-Player", "web", "/radio", null, "gebaut", ""],
  ["LS-26", "Startseite (unverändert)", "web", "/", null, "gebaut", ""],
]);
