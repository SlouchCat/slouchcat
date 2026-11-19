/* Starts the mockup after every page file has registered its routes. */
'use strict';
(function boot() {
  const j = $('#jump');
  j.innerHTML = '<option value="">Seite wählen …</option>' + PAGES.map(([h, l]) => `<option value="${h}">${esc(l)}</option>`).join('');
  j.addEventListener('change', () => {
    if (!j.value) return;
    if (!S.setup && !j.value.startsWith('#/setup')) { S.setup = true; S.server = 'https://medien.example.org'; save(); }
    go(j.value);
  });
  const tick = () => { const d = new Date(); $('#clock').textContent = String(d.getHours()).padStart(2, '0') + ':' + String(d.getMinutes()).padStart(2, '0'); };
  tick(); setInterval(tick, 30000);
  $('#status .sico').innerHTML = icon('wifi', 15) + '<svg width="22" height="12" viewBox="0 0 22 12" aria-hidden="true"><rect x="0.5" y="0.5" width="18" height="11" rx="3" fill="none" stroke="currentColor"/><rect x="2" y="2" width="12" height="8" rx="1.5" fill="currentColor"/><rect x="19.5" y="4" width="2" height="4" rx="1" fill="currentColor"/></svg>';
  fit(); render();
})();
