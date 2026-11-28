/* Konzept 28 «Live-TV»: the mockup controls every client puts into its tool bar (outside the app):
   «Zustände» (clock, profile, rights, page states, programme data, disturbed channel, reset) and the jump
   menu built from the register. Deliberately not in the product's look. Depends on live-sim.js and
   live-register.js. Usage: LIVE_MOCK.mount(buttonHostElement); LIVE_MOCK.jump(selectElement, clients, go). */
'use strict';
(function (g) {
  const SIM = g.LIVE_SIM; const UI = g.LIVE_UI; const REG = g.LIVE_REG;
  const esc = UI.esc;
  let host = null; let open = false;

  function panel() {
    const r = SIM.rights(); const pr = SIM.rawRights('profile'); const ac = SIM.rawRights('account'); const p = SIM.profile(); const f = SIM.flags(); const m = SIM.mode();
    const o = (label, act, val, pressed, title) => `<button type="button" class="lvm-o" data-lvm="${act}" data-v="${esc(val)}" aria-pressed="${!!pressed}"${title ? ` title="${esc(title)}"` : ''}>${label}</button>`;
    const sw = (label, act, val, on) => o(`${on ? '☑' : '☐'} ${label}`, act, val, on);
    return `<div class="lvm-panel" role="region" aria-label="Zustände des Mockups">
      <div class="lvm-top"><b>Zustände des Mockups</b><button type="button" class="lvm-o" data-lvm="close">Schliessen</button></div>
      <p>Alles ist gespielt: Beispieldaten, simulierte Wiedergabe, kein Server.</p>
      <section><h3>Uhr</h3><p class="lvm-clock">${esc(UI.fmt.dayLong(SIM.now()))}, <span data-lv-now="time">${UI.fmt.time(SIM.now())}</span> ${SIM.clockRunning() ? '(läuft)' : '(angehalten)'}</p>
        <div class="lvm-row">${o(SIM.clockRunning() ? 'Anhalten' : 'Weiterlaufen', 'clock', '', false)}${o('+5 Min.', 'adv', 5)}${o('+30 Min.', 'adv', 30)}${o('+2 Std.', 'adv', 120)}</div></section>
      <section><h3>Wer schaut?</h3><div class="lvm-row">${SIM.profiles().map((x) => o(`${esc(x.name)}${x.primary ? ' · Hauptprofil' : ''}${x.age ? ' · ab ' + x.age : ''}`, 'profile', x.id, x.id === p.id)).join('')}</div></section>
      <section><h3>Rechte von ${esc(p.name)} (Profil)</h3><div class="lvm-row">${sw('Live-TV', 'rp', 'liveTv', pr.liveTv)}${sw('Aufnahmen', 'rp', 'recordings', pr.recordings)}${sw('Timeshift', 'rp', 'timeshift', pr.timeshift)}</div>
        <h3>Deckel des Kontos «Familie Keller»</h3><div class="lvm-row">${sw('Live-TV', 'ra', 'liveTv', ac.liveTv)}${sw('Aufnahmen', 'ra', 'recordings', ac.recordings)}${sw('Timeshift', 'ra', 'timeshift', ac.timeshift)}</div>
        <p>Wirksam: Live-TV ${r.liveTv ? 'an' : 'aus'} · Aufnahmen ${r.recordings ? 'an' : 'aus'} · Timeshift ${r.timeshift ? 'an' : 'aus'}${r.liveTv ? '' : ' (der Bereich fehlt ganz, Adressen zeigen «Nicht gefunden»)'}</p></section>
      <section><h3>Zustand der Seiten</h3><div class="lvm-row">${o('Normal', 'mode', 'normal', m === 'normal')}${o('Leer', 'mode', 'empty', m === 'empty', 'Kein Sender im Angebot')}${o('Lädt', 'mode', 'loading', m === 'loading')}${o('Fehler', 'mode', 'error', m === 'error')}</div></section>
      <section><h3>Programmdaten und Empfang</h3><div class="lvm-row">${sw('Ohne EPG (kein Sender hat Programmdaten)', 'flag', 'noEpg', f.noEpg)}${sw('Sender gestört: zusätzlich SRF 1', 'flag', 'disturbed', f.disturbed)}</div>
        <p>ZDFinfo ist in den Beispieldaten immer gestört, TeleZüri hat keine Programmdaten, phoenix nur bis 20:00, KiKA endet um 21:00.</p></section>
      <div class="lvm-row lvm-end"><a href="${esc(g.LIVE_MOCK_HOME || '../index.html')}">Alle Einstiege</a><span style="flex:1"></span>${o('Alles zurücksetzen', 'reset', '')}</div>
    </div>`;
  }
  function draw() {
    if (!host) return;
    const p = SIM.profile(); const m = SIM.mode(); const r = SIM.rights();
    const note = [p.name, m !== 'normal' ? { empty: 'leer', loading: 'lädt', error: 'Fehler' }[m] : '', !r.liveTv ? 'ohne Live-TV' : '', SIM.flags().noEpg ? 'ohne EPG' : ''].filter(Boolean).join(' · ');
    const focused = document.activeElement && host.contains(document.activeElement) ? document.activeElement.dataset.lvm + ':' + document.activeElement.dataset.v : null;
    host.innerHTML = `<button type="button" class="lvm-btn" data-lvm="toggle" data-v="" aria-expanded="${open}">Zustände · ${esc(note)}</button>${open ? panel() : ''}`;
    if (focused) { const el = host.querySelector(`[data-lvm="${focused.split(':')[0]}"][data-v="${focused.split(':').slice(1).join(':')}"]`); if (el) el.focus(); }
  }
  function act(a, v) {
    if (a === 'toggle') { open = !open; draw(); return; }
    if (a === 'close') { open = false; draw(); return; }
    if (a === 'clock') SIM.setClock(!SIM.clockRunning());
    else if (a === 'adv') SIM.advance(Number(v));
    else if (a === 'profile') SIM.setProfile(v);
    else if (a === 'rp') SIM.setRight('profile', v, !SIM.rawRights('profile')[v]);
    else if (a === 'ra') SIM.setRight('account', v, !SIM.rawRights('account')[v]);
    else if (a === 'mode') SIM.setMode(v);
    else if (a === 'flag') SIM.setFlag(v, !SIM.flags()[v]);
    else if (a === 'reset') SIM.reset();
    draw();
  }
  function mount(el) {
    host = el; host.classList.add('lvm');
    host.addEventListener('click', (e) => { const b = e.target.closest('[data-lvm]'); if (b) { e.preventDefault(); e.stopPropagation(); act(b.dataset.lvm, b.dataset.v); } });
    host.addEventListener('keydown', (e) => { e.stopPropagation(); if (e.key === 'Escape' && open) { open = false; draw(); host.querySelector('.lvm-btn').focus(); } });
    SIM.on((t) => { if (t === 'change') draw(); });
    draw();
  }
  /** fill a <select> with the register entries of the given clients; go(href) navigates */
  function jump(sel, clients, go) {
    const groups = clients.map((c) => [c, REG.forClient(c).filter((e) => e.href).sort(REG.order)]);
    sel.innerHTML = '<option value="">Seite wählen …</option>' + groups.map(([c, list]) => `<optgroup label="${esc(REG.CLIENTS[c])}">${list.map((e) => `<option value="${esc(e.href)}">${esc(e.id)} · ${esc(e.title)}${e.stand === 'gebaut' ? '' : ' (offen)'}</option>`).join('')}</optgroup>`).join('');
    sel.addEventListener('change', () => { const v = sel.value; if (!v) return; sel.blur(); go(v); });
    sel.addEventListener('keydown', (e) => e.stopPropagation());
  }
  const syncJump = (sel) => { if (!sel) return; const h = location.hash; sel.value = [...sel.options].some((o) => o.value === h) ? h : ''; };
  g.LIVE_MOCK = { mount, jump, syncJump, redraw: draw };
})(typeof window !== 'undefined' ? window : globalThis);
