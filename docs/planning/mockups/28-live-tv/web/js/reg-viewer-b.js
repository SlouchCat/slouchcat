/* Web Zuschauer B.
   Register entries of this agent (see shared/live-register.js). Keep «stand» up to date: 'offen' or 'gebaut'. */
'use strict';
LIVE_REG.add('web-b', 'web/js/viewer-b.js', [
  ["LS-04", "Programmzeitschrift", "web", "/live/guide", null, "offen", ""],
  ["LS-07", "Live-Player", "web", "/live/watch/:id", "#/live/watch/srfzwei", "offen", ""],
  ["LS-08", "Meine Aufnahmen", "web", "/live/recordings", null, "offen", ""],
  ["LS-09", "Aufnahme-Detail", "web", "/live/recordings/:id", "#/live/recordings/rec-terra", "offen", ""],
  ["LS-10", "Aufnahme-Player", "web", "/live/recordings/:id/play", "#/live/recordings/rec-arena/play", "offen", ""],
  ["LS-11", "Geplante Aufnahmen", "web", "/live/scheduled", null, "offen", ""],
  ["LS-12", "Konflikte und Warnungen", "web", "/live/scheduled/warnings", null, "offen", ""],
  ["LS-13", "Erinnerungen", "web", "/live/reminders", null, "offen", ""],
  ["LS-14", "Aufnahmen meiner Profile", "web", "/live/recordings/profiles", null, "offen", ""],
  ["LD-01", "Aufnahme planen", "web", "/live/dialog/LD-01", null, "offen", ""],
  ["LD-02", "Serienaufnahme", "web", "/live/dialog/LD-02", null, "offen", ""],
  ["LD-03", "Manuelle oder wiederkehrende Aufnahme", "web", "/live/dialog/LD-03", null, "offen", ""],
  ["LD-04", "Sofortaufnahme", "web", "/live/dialog/LD-04", null, "offen", ""],
  ["LD-05", "Erinnerung", "web", "/live/dialog/LD-05", null, "offen", ""],
  ["LD-06", "Löschen und Abbrechen", "web", "/live/dialog/LD-06", null, "offen", ""],
  ["LD-07", "Konflikt oder Warnung", "web", "/live/dialog/LD-07", null, "offen", ""],
  ["LD-08", "Tag und Uhrzeit wählen", "web", "/live/dialog/LD-08", null, "offen", ""],
  ["LD-09", "Filterblatt", "web", "/live/dialog/LD-09", null, "gebaut", "Grundform im Kern (web/js/dialogs.js), darf ersetzt werden"],
  ["LD-10", "Senderwechsel bei Zeitversatz", "web", "/live/dialog/LD-10", null, "offen", ""],
  ["LD-11", "Aufbewahrung ändern", "web", "/live/dialog/LD-11", null, "offen", ""],
]);
