/* Web Verwaltung A.
   Register entries of this agent (see shared/live-register.js). Keep «stand» up to date: 'offen' or 'gebaut'. */
'use strict';
LIVE_REG.add('admin-a', 'web/js/admin-a.js', [
  ["LA-01", "Live-TV-Übersicht", "admin", "/admin/live", null, "offen", ""],
  ["LA-03", "Quelle anlegen und ändern", "admin", "/admin/live/sources/:id", "#/admin/live/sources/new", "offen", ""],
  ["LA-04", "Import-Vorschau und -Ergebnis", "admin", "/admin/live/sources/:id/import/:run", "#/admin/live/sources/src-kabel/import/run-k5", "offen", ""],
  ["LA-05", "Sender", "admin", "/admin/live/channels", "#/admin/live/channels?tab=channels", "offen", ""],
  ["LA-06", "Sender bearbeiten", "admin", "/admin/live/channels", "#/admin/live/channels?tab=channels&edit=srf1", "offen", ""],
  ["LA-07", "Duplikate und Zusammenführen", "admin", "/admin/live/channels", "#/admin/live/channels?tab=duplicates", "offen", ""],
  ["LA-08", "Sendergruppen", "admin", "/admin/live/channels", "#/admin/live/channels?tab=groups", "offen", ""],
  ["LA-20", "Verwaltungsübersicht", "admin", "/admin", null, "offen", ""],
  ["LA-21", "Aktive Wiedergaben", "admin", "/admin/streams", null, "offen", ""],
  ["LA-22", "Konten und Einladungen", "admin", "/admin/accounts", null, "offen", ""],
  ["LA-23", "Hintergrundjobs", "admin", "/admin/jobs", null, "offen", ""],
  ["LA-24", "Benachrichtigungen der Verwaltung", "admin", "/admin/notifications", null, "offen", ""],
  ["LA-26", "Meine Medien", "admin", "/admin/my-media", null, "offen", ""],
  ["LA-27", "Server › Update", "admin", "/admin/server", null, "offen", ""],
  ["LA-28", "Sicherung", "admin", "/admin/backup", null, "offen", ""],
]);
