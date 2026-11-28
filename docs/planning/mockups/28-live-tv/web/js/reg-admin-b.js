/* Web Verwaltung B.
   Register entries of this agent (see shared/live-register.js). Keep «stand» up to date: 'offen' or 'gebaut'. */
'use strict';
LIVE_REG.add('admin-b', 'web/js/admin-b.js', [
  ["LA-09", "Empfang: Strategien und Zeitpläne", "admin", "/admin/live/reception", null, "offen", ""],
  ["LA-10", "EPG-Quellen", "admin", "/admin/live/epg", "#/admin/live/epg?tab=sources", "offen", ""],
  ["LA-11", "EPG-Zuordnungen", "admin", "/admin/live/epg", "#/admin/live/epg?tab=mapping", "offen", ""],
  ["LA-12", "Aktive Streams", "admin", "/admin/live/streams", null, "offen", ""],
  ["LA-13", "Überwachung und Diagnose", "admin", "/admin/live/diagnostics", null, "offen", ""],
  ["LA-14", "Aufnahmen (DVR-Einstellungen)", "admin", "/admin/live/storage", "#/admin/live/storage?tab=dvr", "offen", ""],
  ["LA-15", "Timeshift", "admin", "/admin/live/storage", "#/admin/live/storage?tab=timeshift", "offen", ""],
  ["LA-16", "Speicher", "admin", "/admin/live/storage", "#/admin/live/storage?tab=storage", "offen", ""],
]);
