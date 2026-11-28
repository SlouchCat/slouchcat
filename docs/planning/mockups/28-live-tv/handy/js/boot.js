/* Starts the phone and tablet mockup after every page file has registered its routes. Owned by the core. */
'use strict';
(function boot() {
  installPlaceholders();
  window.LIVE_MOCK_HOME = '../index.html';
  LIVE_MOCK.mount($('#lvm'));
  LIVE_MOCK.jump($('#jump'), ['handy'], (h) => go(h));
  $('#status .sico').innerHTML = icon('wifi', 15) + '<svg width="22" height="12" viewBox="0 0 22 12" aria-hidden="true"><rect x="0.5" y="0.5" width="18" height="11" rx="3" fill="none" stroke="currentColor"/><rect x="2" y="2" width="12" height="8" rx="1.5" fill="currentColor"/><rect x="19.5" y="4" width="2" height="4" rx="1" fill="currentColor"/></svg>';
  if (!location.hash || location.hash === '#/') history.replaceState(null, '', '#/live/channels');
  fit(); render();
})();
