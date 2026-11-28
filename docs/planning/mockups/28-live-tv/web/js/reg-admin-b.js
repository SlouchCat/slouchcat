/* Web Verwaltung B.
   Register entries of this agent (see shared/live-register.js). Keep «stand» up to date: 'offen' or 'gebaut'. */
'use strict';
LIVE_REG.add('admin-b', 'web/js/admin-b.js', [
  ["LA-09", "Empfang: Strategien und Zeitpläne", "admin", "/admin/live/reception", "#/admin/live/reception?tab=strategy", "gebaut", ""],
  ["LA-10", "EPG-Quellen", "admin", "/admin/live/epg", "#/admin/live/epg?tab=sources", "gebaut", ""],
  ["LA-11", "EPG-Zuordnungen", "admin", "/admin/live/epg", "#/admin/live/epg?tab=mapping", "gebaut", ""],
  ["LA-12", "Aktive Streams", "admin", "/admin/live/streams", "#/admin/live/streams", "gebaut", ""],
  ["LA-13", "Überwachung und Diagnose", "admin", "/admin/live/diagnostics", "#/admin/live/diagnostics", "gebaut", ""],
  ["LA-14", "Aufnahmen (DVR-Einstellungen)", "admin", "/admin/live/storage", "#/admin/live/storage?tab=dvr", "gebaut", ""],
  ["LA-15", "Timeshift", "admin", "/admin/live/storage", "#/admin/live/storage?tab=timeshift", "gebaut", ""],
  ["LA-16", "Speicher", "admin", "/admin/live/storage", "#/admin/live/storage?tab=storage", "gebaut", ""],
]);
