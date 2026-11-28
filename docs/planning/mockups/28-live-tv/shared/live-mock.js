/* Konzept 28 «Live-TV»: the mockup controls every client puts into its tool bar (outside the app):
   «Zustände» (clock, profile, rights, page states, programme data, disturbed channel, reset) and the jump
   menu built from the register. Deliberately not in the product's look. Depends on live-sim.js and
   live-register.js. Usage: LIVE_MOCK.mount(buttonHostElement); LIVE_MOCK.jump(selectElement, clients, go). */
'use strict';
(function (g) {
  const SIM = g.LIVE_SIM; const UI = g.LIVE_UI; const REG = g.LIVE_REG;
  const esc = UI.esc;
  let host = null; let open = false; let opts = {};
  /* client actions (not states): LIVE_MOCK.addAction(label, fn) */
  const actions = [];
  const FAULTS = [['', 'Normal'], ['unreachable', 'Sender nicht erreichbar'], ['fallback', 'Ersatzquelle aktiv'], ['limit', 'Grenze erreicht'], ['stall', 'Strom stockt'], ['slow', 'Langsamer Start'], ['notPlayable', 'Nicht abspielbar'], ['busy', 'Server ausgelastet'], ['busyPerm', 'Server ausgelastet: Dauerempfang belegt die Kapazität']];
  const ADM = [['', 'Normal'], ['low', 'Speicher knapp'], ['full', 'Speicher voll'], ['unmounted', 'Netzlaufwerk nicht eingehängt']];

  function panel() {
    const r = SIM.rights(); const pr = SIM.rawRights('profile'); const ac = SIM.rawRights('account'); const p = SIM.profile(); const f = SIM.flags(); const m = SIM.mode();
    const o = (label, act, val, pressed, title) => `<button type="button" class="lvm-o" data-lvm="${act}" data-v="${esc(val)}" aria-pressed="${!!pressed}"${title ? ` title="${esc(title)}"` : ''}>${label}</button>`;
    const sw = (label, act, val, on) => o(`${on ? '☑' : '☐'} ${label}`, act, val, on);
    return `<div class="lvm-panel" role="region" aria-label="Zustände des Mockups">
      <div class="lvm-top"><b>Zustände des Mockups</b><button type="button" class="lvm-o" data-lvm="close">Schliessen</button></div>
      <p>Alles ist gespielt: Beispieldaten, simulierte Wiedergabe, kein Server.</p>
      <section><h3>Echte Logos und Bilder</h3><div class="lvm-row">${[['auto', 'Automatisch'], ['on', 'An'], ['off', 'Aus']].map(([v, t]) => o(t, 'img', v, UI.imageMode() === v)).join('')}</div><p>Jetzt ${UI.imagesOn() ? 'an' : 'aus'}${UI.imageMode() === 'auto' ? (UI.probeResult() === 'ok' ? ' (Bildsatz gefunden)' : ' (kein Bildsatz in shared/img/)') : ''}. Die Bilder liegen nur lokal (Link shared/img/), nie im Repo. Aus: Initialen und ruhige Flächen.</p></section>
      <section><h3>Uhr</h3><p class="lvm-clock">${esc(UI.fmt.dayLong(SIM.now()))}, <span data-lv-now="time">${UI.fmt.time(SIM.now())}</span> ${SIM.clockRunning() ? '(läuft)' : '(angehalten)'}</p>
        <div class="lvm-row">${o(SIM.clockRunning() ? 'Anhalten' : 'Weiterlaufen', 'clock', '', false)}${o('+5 Min.', 'adv', 5)}${o('+30 Min.', 'adv', 30)}${o('+2 Std.', 'adv', 120)}</div></section>
      <section><h3>Wer schaut?</h3><div class="lvm-row">${SIM.profiles().map((x) => o(`${esc(x.name)}${x.primary ? ' · Hauptprofil' : ''}${x.age ? ' · ab ' + x.age : ''}`, 'profile', x.id, x.id === p.id)).join('')}</div></section>
      <section><h3>Rechte von ${esc(p.name)} (Profil)</h3><div class="lvm-row">${sw('Live-TV', 'rp', 'liveTv', pr.liveTv)}${sw('Aufnahmen', 'rp', 'recordings', pr.recordings)}${sw('Timeshift', 'rp', 'timeshift', pr.timeshift)}</div>
        <h3>Deckel des Kontos «Familie Keller»</h3><div class="lvm-row">${sw('Live-TV', 'ra', 'liveTv', ac.liveTv)}${sw('Aufnahmen', 'ra', 'recordings', ac.recordings)}${sw('Timeshift', 'ra', 'timeshift', ac.timeshift)}</div>
        <div class="lvm-row" style="margin-top:6px">${sw('Recht entzogen während der Wiedergabe', 'flag', 'revoked', f.revoked)}${sw('Variante E-28-42 (a): Hauptprofil ohne Live-TV erreicht «Aufnahmen meiner Profile» (Vorgabe: nicht)', 'flag', 'a12', f.a12)}</div>
        <p>Wirksam: Live-TV ${r.liveTv ? 'an' : 'aus'} · Aufnahmen ${r.recordings ? 'an' : 'aus'} · Timeshift ${r.timeshift ? 'an' : 'aus'}${r.liveTv ? '' : f.revoked ? ' (der Player zeigt die Meldung, alle anderen Adressen «Nicht gefunden»)' : ' (der Bereich fehlt ganz, Adressen zeigen «Nicht gefunden»)'}</p></section>
      <section><h3>Zustand der Seiten</h3><div class="lvm-row">${o('Normal', 'mode', 'normal', m === 'normal')}${o('Leer', 'mode', 'empty', m === 'empty', 'Kein Sender im Angebot')}${o('Lädt', 'mode', 'loading', m === 'loading')}${o('Fehler', 'mode', 'error', m === 'error')}</div></section>
      <section><h3>Programmdaten und Empfang</h3><div class="lvm-row">${sw('Ohne EPG (kein Sender hat Programmdaten)', 'flag', 'noEpg', f.noEpg)}${sw('Sender gestört: zusätzlich SRF 1', 'flag', 'disturbed', f.disturbed)}${sw('Viele Sender (2\'000 Testsender im Programmraster)', 'flag', 'many', f.many)}</div>
        <p>ZDFinfo ist in den Beispieldaten immer gestört, TeleZüri hat keine Programmdaten, phoenix nur bis 20:00, KiKA endet um 21:00.</p></section>
      <section><h3>Aufnahmen</h3><div class="lvm-row">${sw('Kontingent voll (neue Aufnahmen werden abgelehnt und gemeldet)', 'flag', 'quotaFull', f.quotaFull)}</div></section>
      <section><h3>Player</h3><div class="lvm-row">${FAULTS.map(([v, t]) => o(t, 'fault', v, f.fault === v)).join('')}</div><div class="lvm-row" style="margin-top:6px">${sw('Eindämmung nicht verfügbar: Umwandlung für Live-TV aus (ZDF braucht eine Umwandlung)', 'flag', 'noContain', f.noContain)}</div><p>${esc(UI.LOAD.note)}</p></section>
      ${opts.admin ? `<section><h3>Verwaltung</h3><div class="lvm-row">${ADM.map(([v, t]) => o(t, 'adm', v, f.admStorage === v)).join('')}</div><div class="lvm-row" style="margin-top:6px">${sw('EPG-Import fehlgeschlagen', 'flag', 'epgFail', f.epgFail)}</div></section>` : ''}
      ${actions.length ? `<section><h3>Aktionen (dieser Client)</h3><div class="lvm-row">${actions.map((a, k) => o(esc(a.label), 'action', k, false)).join('')}</div></section>` : ''}
      <div class="lvm-row lvm-end"><a href="${esc(g.LIVE_MOCK_HOME || '../index.html')}">Alle Einstiege</a><span style="flex:1"></span>${o('Alles zurücksetzen', 'reset', '')}</div>
    </div>`;
  }
  function draw() {
    if (!host) return;
    const p = SIM.profile(); const m = SIM.mode(); const r = SIM.rights();
    const f = SIM.flags();
    const note = [p.name, m !== 'normal' ? { empty: 'leer', loading: 'lädt', error: 'Fehler' }[m] : '', f.revoked ? 'Recht entzogen' : !r.liveTv ? 'ohne Live-TV' : '', f.noEpg ? 'ohne EPG' : '', f.noContain ? 'ohne Eindämmung' : '', f.quotaFull ? 'Kontingent voll' : '', f.fault ? (FAULTS.find((x) => x[0] === f.fault) || [0, f.fault])[1] : '', f.many ? 'viele Sender' : '', opts.admin && f.admStorage ? (ADM.find((x) => x[0] === f.admStorage) || [0, ''])[1] : '', opts.admin && f.epgFail ? 'EPG-Fehler' : ''].filter(Boolean).join(' · ');
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
    else if (a === 'img') UI.setImageMode(v);
    else if (a === 'fault') SIM.setFlag('fault', v);
    else if (a === 'adm') SIM.setFlag('admStorage', v);
    else if (a === 'action') { open = false; draw(); actions[Number(v)].fn(); return; }
    else if (a === 'reset') SIM.reset();
    draw();
  }
  /** mount(host, { admin: true }) — admin adds the switches of the administration */
  function mount(el, o) {
    host = el; host.classList.add('lvm'); opts = o || {};
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
  /** a client action in the panel (for example «Erinnerung jetzt zeigen»): not a state, runs once */
  const addAction = (label, fn) => { actions.push({ label, fn }); draw(); };
  g.LIVE_MOCK = { mount, jump, syncJump, redraw: draw, addAction };
})(typeof window !== 'undefined' ? window : globalThis);
