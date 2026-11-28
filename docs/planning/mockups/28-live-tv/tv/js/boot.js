/* Starts the TV mockup after every page file has registered its routes. Owned by the core. */
'use strict';
(function boot() {
  installPlaceholders();
  window.LIVE_MOCK_HOME = '../index.html';
  LIVE_MOCK.mount($('#lvm'));
  LIVE_MOCK.jump($('#jump'), ['tv'], (h) => { if (OFF) { OFF = false; $('#off').hidden = true; } closeAll(); go(h); });
  if (!location.hash || location.hash === '#/') history.replaceState(null, '', '#/live/channels');
  NAVSTK.length = 0; NAVSTK.push(curHash());
  fit(); render();
})();
