/* Web mockup tool bar (outside the app): width switch Desktop/Tablet/Handy, viewer or administration,
   «Zustände» (shared/live-mock.js) and the jump menu from the register. Owned by the core. */
'use strict';
const MOCK = {
  open: false,
  render() {
    const st = $('#stage');
    if (st.dataset.view !== S.view) { st.dataset.view = S.view; measure(); }
    $$('#tools [data-view]').forEach((b) => b.setAttribute('aria-pressed', String(b.dataset.view === S.view)));
    const adm = parseHash().path.startsWith('/admin');
    $$('#tools [data-side]').forEach((b) => b.setAttribute('aria-current', String((b.dataset.side === 'admin') === adm)));
    LIVE_MOCK.syncJump($('#jump'));
  },
};
window.MOCK = MOCK;
document.addEventListener('click', (e) => {
  const b = e.target.closest('#tools [data-view]'); if (!b) return;
  S.view = b.dataset.view; save(); MOCK.render(); render();
});
