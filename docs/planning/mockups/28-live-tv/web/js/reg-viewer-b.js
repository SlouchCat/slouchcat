/* Web Zuschauer B.
   Register entries of this agent (see shared/live-register.js). Keep «stand» up to date: 'offen' or 'gebaut'. */
'use strict';
LIVE_REG.add('web-b', 'web/js/viewer-b.js', [
  ["LS-04", "Programmzeitschrift", "web", "/live/guide", null, "gebaut", ""],
  ["LS-07", "Live-Player", "web", "/live/watch/:id", "#/live/watch/srfzwei", "gebaut", ""],
  ["LS-08", "Meine Aufnahmen", "web", "/live/recordings", null, "gebaut", ""],
  ["LS-09", "Aufnahme-Detail", "web", "/live/recordings/:id", "#/live/recordings/rec-terra", "gebaut", ""],
  ["LS-10", "Aufnahme-Player", "web", "/live/recordings/:id/play", "#/live/recordings/rec-arena/play", "gebaut", ""],
  ["LS-11", "Geplante Aufnahmen", "web", "/live/scheduled", null, "gebaut", ""],
  ["LS-12", "Konflikte und Warnungen", "web", "/live/scheduled/warnings", null, "gebaut", ""],
  ["LS-13", "Erinnerungen", "web", "/live/reminders", null, "gebaut", ""],
  ["LS-14", "Aufnahmen meiner Profile", "web", "/live/recordings/profiles", null, "gebaut", ""],
  ["LD-01", "Aufnahme planen", "web", "/live/dialog/LD-01", null, "gebaut", ""],
  ["LD-02", "Serienaufnahme", "web", "/live/dialog/LD-02", null, "gebaut", ""],
  ["LD-03", "Manuelle oder wiederkehrende Aufnahme", "web", "/live/dialog/LD-03", null, "gebaut", ""],
  ["LD-04", "Sofortaufnahme", "web", "/live/dialog/LD-04", null, "gebaut", ""],
  ["LD-05", "Erinnerung", "web", "/live/dialog/LD-05", null, "gebaut", ""],
  ["LD-06", "Löschen und Abbrechen", "web", "/live/dialog/LD-06", null, "gebaut", ""],
  ["LD-07", "Konflikt oder Warnung", "web", "/live/dialog/LD-07", null, "gebaut", ""],
  ["LD-08", "Tag und Uhrzeit wählen", "web", "/live/dialog/LD-08", null, "gebaut", ""],
  ["LD-09", "Filterblatt", "web", "/live/dialog/LD-09", null, "gebaut", "Grundform im Kern (web/js/dialogs.js), darf ersetzt werden"],
  ["LD-10", "Senderwechsel bei Zeitversatz", "web", "/live/dialog/LD-10", null, "gebaut", ""],
  ["LD-11", "Aufbewahrung ändern", "web", "/live/dialog/LD-11", null, "gebaut", ""],
]);
