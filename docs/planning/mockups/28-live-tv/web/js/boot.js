/* Starts the web mockup after every page file has registered its routes and dialogs. Owned by the core. */
'use strict';
installPlaceholders();
LIVE_MOCK.mount($('#lvm'), { admin: true });
LIVE_MOCK.jump($('#jump'), ['web', 'admin'], (h) => go(h));
measure();
if (!location.hash) history.replaceState(null, '', '#/live/channels');
render();
