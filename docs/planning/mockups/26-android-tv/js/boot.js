/* Starts the mockup after every page file has registered its routes. */
'use strict';
(function boot() {
  const j = $('#jump');
  j.innerHTML = '<option value="">Seite wählen …</option>' + PAGES.map(([h, l]) => `<option value="${h}">${esc(l)}</option>`).join('');
  j.addEventListener('change', () => {
    const v = j.value; if (!v) return;
    if (!S.setup && !v.startsWith('#/setup')) { Object.assign(S, { setup: true, server: 'https://medien.example.org', secure: true }); save(); }
    j.blur(); if (OFF) { OFF = false; $('#off').hidden = true; }
    go(v);
  });
  const tick = () => { const d = new Date(); $('#tvclock').textContent = String(d.getHours()).padStart(2, '0') + ':' + String(d.getMinutes()).padStart(2, '0'); };
  tick(); setInterval(tick, 20000);
  /* a TV app that is already set up opens on the profile choice */
  if (S.setup && (!location.hash || location.hash === '#/')) history.replaceState(null, '', '#/profiles');
  NAVSTK.length = 0; NAVSTK.push(curHash());
  fit(); render();
})();
