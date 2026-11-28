/* Handy, Kern (Musterseite).
   Register entries of this agent (see shared/live-register.js). Keep «stand» up to date: 'offen' or 'gebaut'. */
'use strict';
LIVE_REG.add('kern', 'handy/js/channels.js, handy/js/live-core.js', [
  ["LS-02", "Senderübersicht", "handy", "/live/channels", null, "gebaut", ""],
  ["LS-20", "Hauptnavigation mit «Live-TV»", "handy", null, "#/live/channels", "gebaut", "Menü zum Hereinwischen und Tablet-Spalte: «Live-TV» nach Webradio; untere Leiste bleibt"],
  ["LD-09", "Filterblatt", "handy", "/live/dialog/LD-09", null, "gebaut", "Grundform im Kern (handy/js/live-core.js), darf verfeinert werden"],
]);
