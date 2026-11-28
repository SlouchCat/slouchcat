/* Web Zuschauer A.
   Register entries of this agent (see shared/live-register.js). Keep «stand» up to date: 'offen' or 'gebaut'. */
'use strict';
LIVE_REG.add('web-a', 'web/js/viewer-a.js', [
  ["LS-01", "Live-TV-Start", "web", "/live", null, "offen", ""],
  ["LS-03", "Meine Sender", "web", "/live/channels/favorites", null, "offen", ""],
  ["LS-05", "Programmsuche", "web", "/live/guide/search", "#/live/guide/search?q=tatort", "offen", ""],
  ["LS-06", "Sendungsdetails", "web", "/live/programs/:id", "#/live/programs/p-zdf-1215", "offen", ""],
  ["LS-15", "Senderprogramm", "web", "/live/guide/channel/:id", "#/live/guide/channel/srf1", "offen", ""],
  ["LS-21", "Profilformular: Block «Live-TV»", "web", "/settings/profiles", null, "offen", ""],
  ["LS-22", "Benachrichtigungen einstellen", "web", "/settings/notifications", null, "offen", ""],
  ["LS-23", "Benachrichtigungen (Glocke)", "web", "/notifications", null, "offen", ""],
  ["LS-24", "Suche: Verweis ins TV-Programm", "web", "/search", "#/search?q=tatort", "offen", ""],
  ["LS-25", "Ton-Vorrang und Mini-Player", "web", "/radio", null, "offen", ""],
  ["LS-26", "Startseite (unverändert)", "web", "/", null, "offen", ""],
]);
