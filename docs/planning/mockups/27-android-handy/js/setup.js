/* Setup, sign-in, profile choice, «Mehr», devices, TV pairing, settings and account.
   Owner of this file: the routes listed for setup.js in BAUKASTEN.md. Everything sits in one closure
   so helper names cannot collide with the other page files; actions and CSS classes start with «su». */
'use strict';
(() => {
  const U = (ui.su = ui.su || {});
  /* «Neu starten» in the mockup bar reseeds S; forget the flows of this file as well */
  document.addEventListener('click', e => { if (e.target.closest('[data-tool="reset"]')) Object.keys(U).forEach(k => delete U[k]); }, true);
  const VERSION = '1.42.0';
  const DEFAULT_SERVER = 'https://medien.example.org';
  const DEVICE = 'Pixel 9';
  const TV_NAME = 'Wohnzimmer · Chromecast mit Google TV';

  /* ---------- small helpers ---------- */
  const secure = a => !/^http:\/\//i.test(a || '');
  const host = a => String(a || '').replace(/^https?:\/\//i, '').replace(/\/.*$/, '');
  const norm = a => { let s = String(a || '').trim().replace(/\/+$/, ''); if (s && !/^https?:\/\//i.test(s)) s = 'https://' + s; return s; };
  const later = (fn, ms) => setTimeout(fn, ms);
  const focusFid = fid => { const el = document.querySelector(`[data-fid="${fid}"]`); if (el) el.focus(); };
  const spin = '<span class="su-spin" aria-hidden="true"></span>';
  const eyeOff = '<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M2 12s3.6-7 10-7c2 0 3.7.7 5.1 1.6M22 12s-3.6 7-10 7c-2 0-3.7-.7-5.1-1.6"/><path d="M9.9 9.9a3 3 0 0 0 4.2 4.2M3 3l18 18"/></svg>';
  const err = (id, msg) => (msg ? `<span class="su-err" id="${id}">${icon('alert', 16)}<span>${esc(msg)}</span></span>` : '');
  const plural = (n, one, many) => `${n} ${n === 1 ? one : many}`;
  const group = (rows, cls) => `<div class="su-group${cls ? ' ' + cls : ''}">${rows.join('')}</div>`;
  const sec = t => `<h2 class="sec">${esc(t)}</h2>`;

  /* Defaults for fields that older saved states do not have. */
  function prefs() {
    const p = (S.prefs = S.prefs || {});
    const d = { quality: 'Hoch (1080p)', qMobile: 'Mittel (720p)', audio: 'Deutsch', subs: 'Aus', autoplay: true, skipIntro: true, wifiOnly: true, push: true, mail: true, tg: false };
    for (const k in d) if (p[k] === undefined) p[k] = d[k];
    p.ev = Object.assign({ ep: true, wish: true, dev: true, ann: true }, p.ev || {});
    S.tvs = S.tvs || []; S.devices = S.devices || []; S.downloads = S.downloads || [];
    return p;
  }

  /* Re-open a sheet in place (keeps focus and scroll, no second slide-in). */
  function reSheet(html) {
    const a = document.activeElement; const fid = a && a.dataset ? a.dataset.fid : null;
    const old = $('#overlays .sheet'); const top = old ? old.scrollTop : 0;
    sheet(html);
    if (!old) return;
    const s = $('#overlays .sheet'); const sc = $('#overlays .scrim');
    s.style.animation = 'none'; sc.style.animation = 'none'; s.scrollTop = top;
    if (fid) { const el = $(`#overlays [data-fid="${fid}"]`); if (el) el.focus({ preventScroll: true }); }
  }

  /* Real covers of the test library as a quiet backdrop of the entry screens. */
  function art() {
    const ids = [...new Set([...row('spot'), ...row('rel'), ...row('rec'), ...row('lib1'), ...row('lib6')])].filter(i => T(i).kind === 'm' || T(i).kind === 's').slice(0, 21);
    const rows = [0, 1, 2].map(r => `<div class="su-art-row">${ids.slice(r * 7, r * 7 + 7).concat(ids.slice(r * 7, r * 7 + 7)).map(i => `<span class="su-art-p">${pic(`p-${i}.webp`, '')}</span>`).join('')}</div>`).join('');
    return `<div class="su-art" aria-hidden="true"><div class="su-art-in">${rows}</div></div>`;
  }
  const brand = sub => `<div class="su-brand"><span class="wordmark">Slouch<i>cat</i></span>${sub ? `<p>${esc(sub)}</p>` : ''}</div>`;
  const entry = inner => `<div class="su-entry">${art()}<div class="su-pane"><div class="su-panein">${inner}</div></div></div>`;
  const bindSubmit = (root, fn) => { const f = root.querySelector('form[data-form]'); if (f) f.addEventListener('submit', e => { e.preventDefault(); fn(); }); };

  /* ======================================================================
     /setup  server address
     ====================================================================== */
  function resultCard(f) {
    const lock = f.secure
      ? `<p class="su-lock ok">${icon('lock', 18)}<span>Verbindung verschlüsselt</span></p>`
      : `<p class="su-lock warn">${icon('alert', 18)}<span>Verbindung nicht verschlüsselt</span></p>`;
    return `<div class="su-result" role="status" tabindex="-1" data-fid="su-result">
        <div class="su-rh"><span class="su-sico">${icon('server', 22)}</span>
          <span class="su-rtx"><b>${esc(host(f.addr))}</b><small>Slouchcat ${VERSION}</small></span>
          <span class="badge ok">${icon('check', 14)} Gefunden</span></div>
        <dl class="su-dl"><div><dt>Adresse</dt><dd>${esc(f.addr)}</dd></div><div><dt>Version</dt><dd>Slouchcat ${VERSION}</dd></div></dl>
        ${lock}
      </div>
      <button type="button" class="btn primary block" data-act="suNext" data-fid="su-next">Weiter ${icon('right', 18)}</button>
      <button type="button" class="btn ghost block su-mt8" data-act="suChange" data-fid="su-change">Andere Adresse</button>`;
  }

  route('/setup', () => {
    if (U.addr === undefined) U.addr = S.server || '';
    const f = U.found;
    const html = entry(`
      ${brand('Deine Filme, Serien, Hörbücher und Podcasts vom eigenen Server.')}
      <form class="su-form" data-form="connect" novalidate aria-labelledby="su-h-connect">
        <h1 class="su-h" id="su-h-connect">Mit deinem Server verbinden</h1>
        <label class="field"><span>Adresse des Servers</span>
          <input class="inp${U.addrErr ? ' su-bad' : ''}" data-bind="ui.su.addr" data-fid="su-addr" value="${esc(U.addr)}" placeholder="${DEFAULT_SERVER}"
            inputmode="url" autocomplete="url" autocapitalize="off" spellcheck="false"${f ? ' readonly' : ''} aria-invalid="${!!U.addrErr}" aria-describedby="su-addr-msg">
          ${U.addrErr ? err('su-addr-msg', U.addrErr) : `<span class="hint su-hint" id="su-addr-msg">Dieselbe Adresse, unter der du Slouchcat im Browser öffnest.</span>`}
        </label>
        ${f ? resultCard(f) : `<button type="submit" class="btn primary block" data-fid="su-connect"${U.busy ? ' disabled' : ''}>${U.busy ? `${spin} Verbinde …` : 'Verbinden'}</button>`}
      </form>`);
    return { full: true, html, after: root => bindSubmit(root, ACT.suConnect) };
  });
  PAGES.push(['#/setup', 'Einrichten: Serveradresse']);

  function probe(a) {
    U.busy = true; render();
    later(() => { U.busy = false; U.found = { addr: a, secure: secure(a) }; render(); focusFid('su-result'); }, 750);
  }
  ACT.suConnect = () => {
    if (U.busy || U.found) return;
    const a = norm(U.addr);
    if (!a) { U.addrErr = 'Gib die Adresse deines Servers ein.'; render(); focusFid('su-addr'); return; }
    if (!/^https?:\/\/[^\s/:]+(\.[^\s/:]+)*(:\d+)?(\/\S*)?$/i.test(a)) {
      U.addrErr = 'Das sieht nicht nach einer Adresse aus. Beispiel: https://medien.example.org'; render(); focusFid('su-addr'); return;
    }
    U.addrErr = ''; U.addr = a;
    if (secure(a)) { probe(a); return; }
    sheet(`<div class="su-shead warn">${icon('alert', 28)}</div>
      <h2>Unverschlüsselte Verbindung</h2>
      <div class="su-sb"><p class="su-p">Diese Verbindung ist nicht verschlüsselt. In fremden Netzen kann jemand dein Passwort mitlesen und deinen Zugang übernehmen.</p>
        <p class="su-addrline">${icon('globe', 16)}<span>${esc(a)}</span></p></div>
      <div class="acts"><button type="button" class="btn primary" data-act="close" autofocus>Abbrechen</button>
        <button type="button" class="btn ghost" data-act="suHttpOk">Trotzdem verbinden</button></div>`);
  };
  ACT.suHttpOk = () => { closeSheet(); probe(U.addr); };
  ACT.suChange = () => { U.found = null; render(); focusFid('su-addr'); const el = $('[data-fid="su-addr"]'); if (el) el.select(); };
  ACT.suNext = () => go('#/setup/login');

  /* ======================================================================
     /setup/login
     ====================================================================== */
  route('/setup/login', () => {
    const addr = (U.found && U.found.addr) || S.server || DEFAULT_SERVER;
    if (U.mail === undefined) U.mail = S.account || '';
    const html = entry(`
      ${brand('')}
      <a class="su-srv" href="#/setup" data-fid="su-srv" aria-label="Server ${esc(host(addr))}, Adresse ändern">
        <span class="su-sico">${icon('server', 20)}</span>
        <span class="su-rtx"><b>${esc(host(addr))}</b><small>${secure(addr) ? `${icon('lock', 13)} Verschlüsselt` : `${icon('alert', 13)} Nicht verschlüsselt`} · Slouchcat ${VERSION}</small></span>
        <span class="su-srv-ch">Ändern</span></a>
      <form class="su-form" data-form="login" novalidate aria-labelledby="su-h-login">
        <h1 class="su-h" id="su-h-login">Anmelden</h1>
        <label class="field"><span>E-Mail</span>
          <input class="inp${U.mailErr ? ' su-bad' : ''}" type="email" data-bind="ui.su.mail" data-fid="su-mail" value="${esc(U.mail)}" placeholder="name@example.org"
            autocomplete="username" autocapitalize="off" spellcheck="false" aria-invalid="${!!U.mailErr}" aria-describedby="su-mail-msg"${U.busy ? ' disabled' : ''}>
          ${err('su-mail-msg', U.mailErr)}</label>
        <label class="field"><span>Passwort</span>
          <span class="su-pwwrap"><input class="inp${U.pwErr ? ' su-bad' : ''}" type="${U.showPw ? 'text' : 'password'}" data-bind="ui.su.pw" data-fid="su-pw" value="${esc(U.pw || '')}"
            autocomplete="current-password" aria-invalid="${!!U.pwErr}" aria-describedby="su-pw-msg"${U.busy ? ' disabled' : ''}>
          <button type="button" class="ib su-eye" data-act="suPw" data-fid="su-eye" aria-pressed="${!!U.showPw}" aria-label="${U.showPw ? 'Passwort verbergen' : 'Passwort anzeigen'}">${U.showPw ? eyeOff : icon('eye')}</button></span>
          ${err('su-pw-msg', U.pwErr)}</label>
        <button type="submit" class="btn primary block" data-fid="su-login"${U.busy ? ' disabled' : ''}>${U.busy ? `${spin} Melde an …` : 'Anmelden'}</button>
        <p class="hint su-note-sm">${icon('phone', 16)}<span>Dieses Handy erscheint danach als «${DEVICE}» unter «Meine Geräte» und bleibt angemeldet, bis du es abmeldest.</span></p>
      </form>`);
    return { full: true, html, after: root => bindSubmit(root, ACT.suLogin) };
  });
  PAGES.push(['#/setup/login', 'Einrichten: Anmelden']);

  ACT.suPw = () => { U.showPw = !U.showPw; render(); };
  ACT.suLogin = () => {
    if (U.busy) return;
    const m = String(U.mail || '').trim(); const p = U.pw || '';
    U.mailErr = !m ? 'Gib deine E-Mail-Adresse ein.' : !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(m) ? 'Diese E-Mail-Adresse ist unvollständig.' : '';
    U.pwErr = p ? '' : 'Gib dein Passwort ein.';
    if (U.mailErr || U.pwErr) { render(); focusFid(U.mailErr ? 'su-mail' : 'su-pw'); return; }
    U.busy = true; render();
    later(() => {
      prefs();
      S.setup = true; S.server = (U.found && U.found.addr) || S.server || DEFAULT_SERVER; S.account = m;
      if (!S.devices.some(d => d.here)) S.devices.unshift({ name: DEVICE, kind: 'phone', since: 'heute', here: true });
      U.busy = false; U.pw = ''; U.showPw = false; U.found = null;
      save(); go('#/profiles');
    }, 900);
  };

  /* ======================================================================
     /profiles  «Wer schaut?» with PIN pad
     ====================================================================== */
  route('/profiles', () => {
    const cur = S.profile;
    const tiles = S.profiles.map((p, k) => `<li><button type="button" class="su-tile" data-act="suPick" data-k="${k}" data-fid="su-p${k}"
        aria-label="${esc(p.name)}${p.pin ? ', mit PIN geschützt' : ''}${k === cur ? ', zuletzt aktiv' : ''}">
        <span class="su-av${k === cur ? ' cur' : ''}">${avatar(p, 92)}${p.pin ? `<span class="su-lockb">${icon('lock', 15)}</span>` : ''}</span>
        <b>${esc(p.name)}</b><small>${p.main ? 'Hauptprofil' : p.fsk ? `Freigabe ab ${p.fsk}` : 'Freigabe 0'}</small></button></li>`).join('');
    const html = `<div class="su-who">
      <div class="su-who-top"><span class="wordmark">Slouch<i>cat</i></span></div>
      <h1>Wer schaut?</h1>
      <ul class="su-tiles">${tiles}</ul>
      <a class="btn ghost" href="#/settings/profiles" data-fid="su-manage">${icon('settings', 20)} Profile verwalten</a>
      <p class="su-who-acc">${icon('user', 16)}<span>${esc(S.account)} · ${esc(host(S.server))}</span></p>
    </div>`;
    return { full: true, html };
  });
  PAGES.push(['#/profiles', 'Profilwahl']);

  const lockOf = k => ((U.lock = U.lock || {})[k] = U.lock[k] || { tries: 0, until: 0 });
  function choose(k) { S.profile = k; save(); toast(`Hallo ${S.profiles[k].name}`); go('#/'); }
  ACT.suPick = d => {
    const k = Number(d.k); const p = S.profiles[k]; if (!p) return;
    if (!p.pin) { choose(k); return; }
    U.pin = { k, v: '' };
    const keys = [['1', ''], ['2', 'ABC'], ['3', 'DEF'], ['4', 'GHI'], ['5', 'JKL'], ['6', 'MNO'], ['7', 'PQRS'], ['8', 'TUV'], ['9', 'WXYZ']]
      .map(([n, l]) => `<button type="button" class="su-key" data-act="suKey" data-d="${n}" data-fid="su-k${n}">${n}<small>${l || '&nbsp;'}</small></button>`).join('');
    sheet(`<div id="su-pin" class="su-pin">
        <div class="su-pinhead">${avatar(p, 56)}<h2>PIN für ${esc(p.name)}</h2></div>
        <div class="su-dots" aria-hidden="true"><i></i><i></i><i></i><i></i></div>
        <p class="su-pinmsg" role="status" aria-live="polite">Gib die vierstellige PIN ein.</p>
        <div class="su-pad">${keys}<span></span>
          <button type="button" class="su-key" data-act="suKey" data-d="0" data-fid="su-k0">0<small>&nbsp;</small></button>
          <button type="button" class="su-key su-del" data-act="suKey" data-d="del" data-fid="su-kdel" aria-label="Letzte Ziffer löschen">${icon('back', 22)}</button></div>
        <div class="acts"><button type="button" class="btn ghost" data-act="close">Abbrechen</button></div>
      </div>`);
    drawPin();
  };
  let pinTimer = null;
  function drawPin(msg) {
    const box = $('#su-pin'); if (!box || !U.pin) return;
    const L = lockOf(U.pin.k); const left = Math.ceil((L.until - Date.now()) / 1000); const locked = left > 0;
    $$('.su-dots i', box).forEach((d, n) => d.classList.toggle('on', n < U.pin.v.length));
    $$('.su-key', box).forEach(b => { b.disabled = locked; });
    const m = $('.su-pinmsg', box);
    if (locked) { m.textContent = `Zu viele falsche Versuche. Warte noch ${left} s.`; m.className = 'su-pinmsg bad'; }
    else if (msg) { m.textContent = msg.t; m.className = 'su-pinmsg' + (msg.bad ? ' bad' : ''); }
    clearInterval(pinTimer);
    if (locked) {
      pinTimer = setInterval(() => {
        if (!$('#su-pin')) { clearInterval(pinTimer); return; }
        if (Date.now() >= L.until) { L.tries = 0; L.until = 0; clearInterval(pinTimer); drawPin({ t: 'Du kannst es wieder versuchen.' }); const k1 = $('#su-pin .su-key'); if (k1) k1.focus(); return; }
        drawPin();
      }, 1000);
    }
  }
  ACT.suKey = d => {
    if (!U.pin || !$('#su-pin')) return;
    const L = lockOf(U.pin.k); if (L.until > Date.now() || U.pin.checking) return;
    if (d.d === 'del') { U.pin.v = U.pin.v.slice(0, -1); drawPin(); return; }
    if (U.pin.v.length >= 4) return;
    U.pin.v += d.d; drawPin();
    if (U.pin.v.length < 4) return;
    U.pin.checking = true;
    later(() => {
      U.pin.checking = false;
      if (!$('#su-pin')) return;
      if (U.pin.v !== '0000') {
        $('#su-pin .su-dots').classList.add('ok');
        later(() => { const k = U.pin.k; closeSheet(true); U.pin = null; lockOf(k).tries = 0; choose(k); }, 260);
        return;
      }
      L.tries += 1; U.pin.v = '';
      const dots = $('#su-pin .su-dots'); dots.classList.remove('shake'); void dots.offsetWidth; dots.classList.add('shake');
      if (L.tries >= 3) { L.until = Date.now() + 30000; drawPin(); return; }
      drawPin({ t: `PIN falsch. Noch ${plural(3 - L.tries, 'Versuch', 'Versuche')}.`, bad: true });
    }, 180);
  };
  document.addEventListener('keydown', e => {
    if (!$('#su-pin') || e.altKey || e.ctrlKey || e.metaKey) return;
    if (/^[0-9]$/.test(e.key)) { e.preventDefault(); ACT.suKey({ d: e.key }); }
    else if (e.key === 'Backspace') { e.preventDefault(); ACT.suKey({ d: 'del' }); }
  });

  /* ======================================================================
     /more
     ====================================================================== */
  route('/more', () => {
    prefs();
    const p = me(); const sec0 = secure(S.server);
    const others = S.profiles.length - 1;
    const tvs = S.tvs.length;
    const html = `${bar('Mehr', { back: false })}
      <div class="su-page su-wide">
        <div class="su-me">
          ${avatar(p, 56)}
          <span class="su-me-tx"><b>${esc(p.name)}</b><small>${p.main ? 'Hauptprofil' : `Freigabe ab ${p.fsk}`} · ${esc(S.account)}</small></span>
          <a class="btn secondary sm" href="#/profiles" data-fid="su-switch">${icon('users', 18)} Profil wechseln</a>
        </div>
        ${sec0 ? '' : `<div class="note warn su-http" role="note">${icon('alert', 20)}<span><b>Unverschlüsselte Verbindung.</b> Passwort und Zugang gehen offen über das Netz. In fremden Netzen kann jemand mitlesen. Richte für den Server HTTPS ein.</span></div>`}
        <div class="su-cols">
          <section>${sec('Bibliothek')}${group([
            li('bookmark', 'Merkliste', S.watch.length ? plural(S.watch.length, 'Titel', 'Titel') : 'Noch leer', null, 'href="#/watchlist" data-fid="su-m-watch"'),
            li('eye', 'Gesehen', S.seen.length ? plural(S.seen.length, 'Titel', 'Titel') : 'Was du fertig geschaut hast', null, 'href="#/watched" data-fid="su-m-seen"'),
            li('grid', 'Sammlungen', plural(DATA.col.length, 'Sammlung', 'Sammlungen'), null, 'href="#/collections" data-fid="su-m-col"'),
            li('headphones', 'Hörbücher', null, null, 'href="#/library/books" data-fid="su-m-books"'),
            li('mic', 'Podcasts', null, null, 'href="#/library/podcasts" data-fid="su-m-pods"'),
            li('radio', 'Webradio', 'Live-Sender', null, 'href="#/radio" data-fid="su-m-radio"'),
          ])}</section>
          <section>${sec('Geräte')}${group([
            li('tv', 'Fernseher verbinden', tvs ? `${plural(tvs, 'Fernseher', 'Fernseher')} verbunden` : 'QR-Code am Fernseher scannen', null, 'href="#/tv" data-fid="su-m-tv"'),
            li('phone', 'Meine Geräte', plural(S.devices.length, 'Gerät angemeldet', 'Geräte angemeldet'), null, 'href="#/devices" data-fid="su-m-dev"'),
          ])}
          ${sec('Einstellungen')}${group([
            li('users', 'Profile verwalten', `${p.main ? 'Namen, Freigaben, PINs' : 'Nur das Hauptprofil'} · ${plural(others + 1, 'Profil', 'Profile')}`, null, 'href="#/settings/profiles" data-fid="su-m-prof"'),
            li('play', 'Wiedergabe', `${S.prefs.qMobile} mobil · ${S.prefs.quality} im WLAN`, null, 'href="#/settings/playback" data-fid="su-m-play"'),
            li('bell', 'Benachrichtigungen', S.prefs.push ? 'Push an' : 'Push aus', null, 'href="#/settings/notifications" data-fid="su-m-note"'),
            li('user', 'Konto', S.account, null, 'href="#/account" data-fid="su-m-acc"'),
          ])}</section>
          <section>${sec('Server')}${group([
            li('server', host(S.server) || 'Kein Server', `Slouchcat ${VERSION} · ${S.server}`, null, 'data-act="suServer" data-fid="su-m-srv"'),
            `<div class="li su-static" role="note"><span class="lead${sec0 ? ' su-ok' : ' su-warnc'}">${icon(sec0 ? 'lock' : 'alert', 22)}</span><span class="tx"><b>${sec0 ? 'Verbindung verschlüsselt' : 'Verbindung nicht verschlüsselt'}</b><small>${sec0 ? 'HTTPS' : 'HTTP · Passwort und Zugang offen lesbar'}</small></span></div>`,
          ])}
          <div class="su-out"><button type="button" class="btn danger block" data-act="suLogoutAsk" data-fid="su-logout">${icon('logout', 20)} Abmelden</button>
            <p class="hint">Slouchcat für Android · Mockup</p></div></section>
        </div>
      </div>`;
    return { nav: 'more', html };
  });
  PAGES.push(['#/more', 'Mehr']);

  ACT.suServer = () => {
    const s = secure(S.server);
    sheet(`<h2>Server</h2><div class="su-sb">
      <dl class="su-dl"><div><dt>Adresse</dt><dd>${esc(S.server)}</dd></div><div><dt>Version</dt><dd>Slouchcat ${VERSION}</dd></div>
        <div><dt>Verbindung</dt><dd>${s ? 'verschlüsselt (HTTPS)' : 'nicht verschlüsselt (HTTP)'}</dd></div><div><dt>Konto</dt><dd>${esc(S.account)}</dd></div></dl>
      <p class="hint su-mt12">Die App verbindet sich mit genau einem Server. Für einen anderen Server meldest du dich ab und gibst die neue Adresse ein.</p></div>
      <div class="acts"><button type="button" class="btn ghost" data-act="close">Schliessen</button><button type="button" class="btn secondary" data-act="suLogoutAsk">Server wechseln</button></div>`);
  };
  ACT.suLogoutAsk = () => {
    const n = (S.downloads || []).length;
    sheet(`<div class="su-shead">${icon('logout', 28)}</div><h2>Abmelden?</h2>
      <div class="su-sb"><p class="su-p">Die App meldet sich auf diesem Gerät ab. ${n ? `Deine ${plural(n, 'Download', 'Downloads')} auf diesem Gerät ${n === 1 ? 'wird' : 'werden'} gelöscht.` : 'Auf diesem Gerät liegen keine Downloads.'}</p>
      <p class="hint su-mt8">Zum Wiederkommen brauchst du die Adresse des Servers, deine E-Mail und dein Passwort.</p></div>
      <div class="acts"><button type="button" class="btn ghost" data-act="close" autofocus>Abbrechen</button><button type="button" class="btn danger" data-act="suLogout">Abmelden</button></div>`);
  };
  ACT.suLogout = () => {
    const keep = S.server;
    S.setup = false; S.downloads = []; S.playing = null; S.profile = 0;
    S.devices = (S.devices || []).filter(d => !d.here);
    Object.keys(U).forEach(k => delete U[k]); U.addr = keep;
    save(); closeSheet(true); toast('Abgemeldet'); go('#/setup');
  };

  /* ======================================================================
     /devices  «Meine Geräte»
     ====================================================================== */
  const KIND_ICON = { phone: 'phone', tablet: 'tablet', tv: 'tv', browser: 'globe' };
  const KIND_NAME = { phone: 'Handy', tablet: 'Tablet', tv: 'Fernseher', browser: 'Browser' };
  const devName = d => String(d.name || '').replace(/\s*\(dieses Gerät\)\s*$/, '');

  route('/devices', () => {
    prefs();
    const main = me().main;
    const list = S.devices.map((d, k) => `<li class="su-dev">
        <span class="lead">${icon(KIND_ICON[d.kind] || 'globe', 24)}</span>
        <span class="tx"><b>${esc(devName(d))}</b>
          <small>${d.here ? '<span class="badge acc">dieses Gerät</span> ' : ''}${esc(KIND_NAME[d.kind] || 'Gerät')} · angemeldet seit ${esc(d.since || 'unbekannt')}</small>
          <small>Letzter Kontakt: ${esc(d.here ? 'jetzt' : d.last || 'vor 2 Tagen')}</small></span>
        ${!d.here && main ? `<button type="button" class="btn ghost sm" data-act="suDevAsk" data-k="${k}" data-fid="su-dev${k}" aria-label="${esc(devName(d))} abmelden">Abmelden</button>` : ''}
      </li>`).join('');
    const html = `${bar('Meine Geräte')}
      <div class="su-page">
        <p class="su-lede">${icon('info', 18)}<span>Handys und Fernseher bleiben angemeldet, bis jemand sie hier abmeldet.</span></p>
        ${main ? '' : `<div class="note" role="note">${icon('lock', 20)}<span>Geräte abmelden kann nur das Hauptprofil. Wechsle ins Hauptprofil, um ein Gerät abzumelden.</span></div>`}
        <ul class="su-group su-devs">${list || '<li class="empty">Keine Geräte</li>'}</ul>
        <div class="su-actrow"><a class="btn secondary" href="#/tv" data-fid="su-dev-tv">${icon('tv', 20)} Fernseher verbinden</a></div>
      </div>`;
    return { nav: 'more', html };
  });
  PAGES.push(['#/devices', 'Meine Geräte']);

  ACT.suDevAsk = d => {
    const k = Number(d.k); const dev = S.devices[k]; if (!dev) return;
    sheet(`<div class="su-shead">${icon(KIND_ICON[dev.kind] || 'globe', 28)}</div>
      <h2>«${esc(devName(dev))}» abmelden?</h2>
      <div class="su-sb"><p class="su-p">Das Gerät wird abgemeldet und seine Downloads werden dort gelöscht.${dev.kind === 'tv' ? ' Den Fernseher verbindest du danach neu.' : ' Es kann sich mit E-Mail und Passwort wieder anmelden.'}</p></div>
      <div class="acts"><button type="button" class="btn ghost" data-act="close" autofocus>Abbrechen</button><button type="button" class="btn danger" data-act="suDevOut" data-k="${k}">Abmelden</button></div>`);
  };
  ACT.suDevOut = d => {
    const k = Number(d.k); const dev = S.devices[k]; if (!dev || dev.here) return;
    S.devices.splice(k, 1);
    if (dev.kind === 'tv') S.tvs = S.tvs.filter(t => `${t.name} · ${t.model}` !== dev.name && t.name !== dev.name);
    closeSheet(true); toast(`${devName(dev)} abgemeldet`); commit();
  };

  /* ======================================================================
     /tv  pair a TV by QR code
     ====================================================================== */
  /* A plausible QR pattern: three finder squares, timing lines, one alignment square, seeded noise. */
  const QR = (() => {
    const n = 25; let s = 2711; const rnd = () => { s = (s * 1103515245 + 12345) % 2147483648; return s / 2147483648; };
    const m = Array.from({ length: n }, () => Array.from({ length: n }, () => rnd() > 0.53));
    const sq = (ox, oy, size, on) => { for (let y = -1; y <= size; y++) for (let x = -1; x <= size; x++) { const X = ox + x; const Y = oy + y; if (X < 0 || Y < 0 || X >= n || Y >= n) continue; m[Y][X] = on(Math.max(Math.abs(x - (size - 1) / 2), Math.abs(y - (size - 1) / 2))); } };
    for (let k = 8; k < n - 8; k++) { m[6][k] = k % 2 === 0; m[k][6] = k % 2 === 0; }
    [[0, 0], [n - 7, 0], [0, n - 7]].forEach(([x, y]) => sq(x, y, 7, d => d <= 1 || d === 3));
    sq(n - 9, n - 9, 5, d => d !== 1 && d <= 2);
    let path = '';
    for (let y = 0; y < n; y++) for (let x = 0; x < n; x++) if (m[y][x]) path += `M${x} ${y}h1v1h-1z`;
    return `<svg class="su-qr" viewBox="-2 -2 29 29" aria-hidden="true"><rect x="-2" y="-2" width="29" height="29" rx="1.5"/><path d="${path}"/></svg>`;
  })();

  const TV_LINES = ['Gutschein vom Server geholt', 'Verschlüsselt an den Fernseher übergeben', 'Fernseher meldet sich an …'];
  const VARIANTS = [['ok', 'Klappt'], ['pkg', 'Fremdes Paket liegt schon'], ['time', 'Zeit abgelaufen']];
  const tvState = () => (U.tv = U.tv || { step: 'intro', v: 'ok', n: 0 });
  const variants = () => {
    const t = tvState();
    return `<div class="su-var" role="group" aria-labelledby="su-var-l"><span id="su-var-l">Varianten (nur im Mockup)</span>
      <div class="su-var-chips">${VARIANTS.map(([v, l]) => `<button type="button" class="chip" data-act="suTvVar" data-v="${v}" data-fid="su-var-${v}" aria-pressed="${t.v === v}">${t.v === v ? icon('check', 16) : ''}${esc(l)}</button>`).join('')}</div></div>`;
  };

  function tvIntro() {
    const main = me().main;
    const steps = [
      ['tv', 'Öffne am Fernseher die Slouchcat-App. Sie zeigt einen QR-Code.'],
      ['qr', 'Scanne den Code mit diesem Handy.'],
      ['check', 'Bestätige hier. Der Fernseher meldet sich mit deinem Konto an.'],
    ];
    return `${bar('Fernseher verbinden')}
      <div class="su-page su-form-w">
        <div class="su-hero">${icon('tv', 40)}</div>
        <h2 class="su-title">Fernseher mit dem Handy verbinden</h2>
        <p class="su-sub">Kein Tippen mit der Fernbedienung: Das Handy gibt dem Fernseher den Zugang.</p>
        <ol class="su-steps">${steps.map(([ic, t], k) => `<li><span class="su-n">${k + 1}</span><span class="su-st-ic">${icon(ic, 20)}</span><span>${esc(t)}</span></li>`).join('')}</ol>
        ${main
          ? `<button type="button" class="btn accent block" data-act="suTv" data-step="scan" data-fid="su-tv-scan">${icon('qr', 20)} Scanner öffnen</button>
             <p class="hint su-center su-mt8">Die App fragt beim ersten Mal nach der Kamera.</p>`
          : `<div class="note" role="note">${icon('lock', 20)}<span>Einen Fernseher verbindet nur das Hauptprofil. So hängt niemand aus einem Kinderprofil ein Gerät an dein Konto.</span></div>
             <a class="btn secondary block" href="#/profiles" data-fid="su-tv-switch">${icon('users', 20)} Profil wechseln</a>`}
        ${S.tvs.length ? `${sec('Verbunden')}${group(S.tvs.map((t, k) => li('tv', `${t.name} · ${t.model}`, `seit ${t.since}`, null, `href="#/devices" data-fid="su-tvl${k}"`)))}` : ''}
        ${main ? variants() : ''}
      </div>`;
  }
  function tvScan() {
    return `<div class="su-cam">
      <div class="su-scene" aria-hidden="true"><div class="su-room"></div>
        <div class="su-tvset"><div class="su-tvscr"><span class="su-tvbrand">Slouch<i>cat</i></span>${QR}<span class="su-tvcap">Mit dem Handy scannen</span></div></div></div>
      <div class="su-vf" aria-hidden="true"><i></i><i></i><i></i><i></i><span class="su-line"></span></div>
      <header class="su-camtop"><button type="button" class="ib" data-act="suTv" data-step="intro" data-fid="su-cam-x" aria-label="Scanner schliessen">${icon('close')}</button><h1>QR-Code scannen</h1></header>
      <div class="su-cambot">
        <p>Richte die Kamera auf den Code am Fernseher.</p>
        <button type="button" class="btn primary" data-act="suTvScan" data-fid="su-cam-ok">${icon('qr', 20)} Code erkannt</button>
        ${variants()}
      </div></div>`;
  }
  function tvConfirm() {
    const p = me();
    return `${bar('Fernseher verbinden', { end: '' })}
      <div class="su-page su-form-w">
        <div class="su-hero warnish">${icon('shield', 40)}</div>
        <h2 class="su-title">Diesen Fernseher mit deinem Konto verbinden?</h2>
        <p class="su-sub">Tu das nur, wenn der Code auf deinem eigenen Fernseher steht.</p>
        <dl class="su-dl su-card">
          <div><dt>Konto</dt><dd>${esc(S.account)}</dd></div>
          <div><dt>Server</dt><dd>${esc(S.server)}</dd></div>
          <div><dt>Bestätigt von</dt><dd>${esc(p.name)} (${p.main ? 'Hauptprofil' : 'Profil'})</dd></div>
        </dl>
        <p class="hint su-mt12">Der Fernseher bekommt Zugang zu den Profilen dieses Kontos, aber nicht zur Verwaltung. Du bekommst eine Meldung, sobald er verbunden ist.</p>
        <div class="su-actrow two"><button type="button" class="btn ghost" data-act="suTv" data-step="intro" data-fid="su-tvc-no">Abbrechen</button>
          <button type="button" class="btn accent" data-act="suTvGo" data-fid="su-tvc-yes">Verbinden</button></div>
      </div>`;
  }
  function tvRun() {
    const t = tvState(); const failAt = t.step === 'fail-pkg' ? 1 : t.step === 'fail-time' ? 2 : -1;
    const rows = TV_LINES.map((l, k) => {
      const st = k === failAt ? 'fail' : k < t.n ? 'done' : k === t.n ? 'busy' : 'wait';
      const ic = st === 'done' ? icon('check', 18) : st === 'fail' ? icon('close', 18) : st === 'busy' ? spin : '';
      return `<li class="su-r ${st}"><span class="su-ri">${ic}</span><span>${esc(l)}</span></li>`;
    }).join('');
    let tail = '';
    if (t.step === 'fail-pkg') {
      tail = `<div class="note warn" role="alert">${icon('alert', 20)}<span>Am Treffpunkt liegt schon ein Paket. Der Fernseher ist womöglich mit einem fremden Server verbunden. Prüfe, welchen Server und welches Konto der Fernseher anzeigt.</span></div>
        <p class="hint su-center">Dein Gutschein verfällt ungenutzt.</p>
        <div class="su-actrow"><button type="button" class="btn primary" data-act="suTv" data-step="intro" data-fid="su-tvf-ok">Verstanden</button></div>`;
    } else if (t.step === 'fail-time') {
      tail = `<div class="note warn" role="alert">${icon('clock', 20)}<span>Das hat nicht geklappt. Zeige am Fernseher einen neuen Code an.</span></div>
        <div class="su-actrow two"><button type="button" class="btn ghost" data-act="suTv" data-step="intro" data-fid="su-tvf-no">Abbrechen</button>
          <button type="button" class="btn primary" data-act="suTv" data-step="scan" data-fid="su-tvf-again">${icon('qr', 20)} Nochmals scannen</button></div>`;
    }
    return `${bar('Fernseher verbinden', { back: t.step !== 'run' })}
      <div class="su-page su-form-w">
        <div class="su-hero${failAt >= 0 ? ' bad' : ''}">${icon(failAt >= 0 ? 'alert' : 'link', 40)}</div>
        <h2 class="su-title">${failAt >= 0 ? 'Nicht verbunden' : 'Verbinde …'}</h2>
        <ul class="su-run" aria-live="polite">${rows}</ul>
        ${tail}
      </div>`;
  }
  function tvDone() {
    return `${bar('Fernseher verbinden')}
      <div class="su-page su-form-w su-done">
        <div class="su-hero ok pop">${icon('check', 44)}</div>
        <h2 class="su-title">Verbunden</h2>
        <p class="su-tvname">${icon('tv', 20)}<span>${esc(TV_NAME)}</span></p>
        <p class="su-sub">Der Fernseher zeigt jetzt die Profilwahl. Du findest ihn unter «Meine Geräte».</p>
        <div class="su-actrow two"><a class="btn ghost" href="#/devices" data-fid="su-tvd-dev">Meine Geräte</a>
          <a class="btn primary" href="#/more" data-fid="su-tvd-ok">Fertig</a></div>
      </div>`;
  }

  route('/tv', () => {
    prefs();
    const t = tvState();
    if (lastPath !== '/tv' && ['done', 'fail-pkg', 'fail-time', 'confirm', 'scan'].includes(t.step)) t.step = 'intro';
    if (!me().main && t.step !== 'intro') t.step = 'intro';
    if (t.step === 'scan') return { full: true, html: tvScan() };
    const html = t.step === 'confirm' ? tvConfirm() : t.step === 'done' ? tvDone() : t.step === 'intro' ? tvIntro() : tvRun();
    return { nav: 'more', html };
  });
  PAGES.push(['#/tv', 'Fernseher verbinden']);

  let tvTimer = null;
  const tvTick = (fn, ms) => { tvTimer = setTimeout(() => { fn(); if (location.hash.split('?')[0] === '#/tv') render(); }, ms); };
  ACT.suTv = d => { clearTimeout(tvTimer); tvState().step = d.step; render(); focusFid(d.step === 'scan' ? 'su-cam-ok' : 'su-tv-scan'); };
  ACT.suTvVar = d => { tvState().v = d.v; render(); };
  ACT.suTvScan = () => { tvState().step = 'confirm'; toast('Code erkannt'); render(); focusFid('su-tvc-yes'); };
  ACT.suTvGo = () => {
    const t = tvState(); clearTimeout(tvTimer);
    t.step = 'run'; t.n = 0; render();
    tvTick(() => {
      t.n = 1;
      if (t.v === 'pkg') { tvTick(() => { t.step = 'fail-pkg'; }, 1200); return; }
      tvTick(() => {
        t.n = 2;
        if (t.v === 'time') { tvTick(() => { t.step = 'fail-time'; }, 2400); return; }
        tvTick(() => {
          t.n = 3; t.step = 'done';
          const name = 'Wohnzimmer'; const model = 'Chromecast mit Google TV';
          if (!S.tvs.some(x => x.name === name && x.model === model)) S.tvs.push({ name, model, since: 'heute' });
          const dev = S.devices.find(x => x.name === TV_NAME);
          if (dev) dev.last = 'gerade eben'; else S.devices.push({ name: TV_NAME, kind: 'tv', since: 'heute', last: 'gerade eben' });
          save();
        }, 1300);
      }, 1000);
    }, 900);
  };

  /* ======================================================================
     /settings/profiles
     ====================================================================== */
  const COLORS = [['var(--accent)', 'Orange'], ['var(--info)', 'Blau'], ['var(--ok)', 'Grün'], ['var(--warn)', 'Gelb'], ['var(--danger)', 'Rot'], ['var(--tx3)', 'Grau']];
  /* the seed stores the colours as hex values; map them onto the swatches */
  const SEED_COLOR = { '#e8794a': 'var(--accent)', '#62aef0': 'var(--info)', '#5fcf8f': 'var(--ok)' };
  const AGES = [0, 6, 12, 16, 18];
  const needPinHint = () => { const m = S.profiles.find(p => p.main); return !!m && !m.pin && S.profiles.some(p => (p.fsk ?? 18) < 18); };
  const profSub = p => [p.main ? 'Hauptprofil' : null, `Freigabe ${p.fsk ?? 18}`, p.pin ? 'PIN' : 'ohne PIN', p.opin ? 'Offline-PIN' : null].filter(Boolean).join(' · ');

  route('/settings/profiles', () => {
    const main = me().main;
    const rows = S.profiles.map((p, k) => main
      ? li(avatar(p, 40), p.name, profSub(p), `${p.pin ? icon('lock', 18) : ''}${icon('right', 18)}`, `data-act="suProf" data-k="${k}" data-fid="su-pr${k}" aria-label="${esc(p.name)} bearbeiten"`)
      : `<div class="li su-static"><span class="lead su-avlead">${avatar(p, 40)}</span><span class="tx"><b>${esc(p.name)}</b><small>${esc(profSub(p))}</small></span></div>`);
    const html = `${bar('Profile verwalten')}
      <div class="su-page">
        ${main && needPinHint() ? `<div class="note warn su-rec" role="note">${icon('shield', 20)}<span class="su-rec-tx">Empfehlung: Gib dem Hauptprofil eine PIN. Sonst kann jeder ins Hauptprofil wechseln und Altersfreigaben ändern.
          <button type="button" class="btn sm secondary" data-act="suPinRec" data-fid="su-rec">${icon('lock', 16)} PIN setzen</button></span></div>` : ''}
        ${main ? '' : `<div class="note" role="note">${icon('lock', 20)}<span>Profile verwaltet nur das Hauptprofil. Wechsle ins Hauptprofil, um Namen, Altersfreigaben und PINs zu ändern.</span></div>`}
        ${group(rows)}
        ${main
          ? `<div class="su-actrow"><button type="button" class="btn secondary" data-act="suProf" data-k="-1" data-fid="su-pr-add">${icon('plus', 20)} Profil hinzufügen</button></div>`
          : `<div class="su-actrow"><a class="btn secondary" href="#/profiles" data-fid="su-pr-sw">${icon('users', 20)} Profil wechseln</a></div>`}
      </div>`;
    return { nav: 'more', html };
  });
  PAGES.push(['#/settings/profiles', 'Profile verwalten']);

  function editHtml() {
    const e = U.edit; const isNew = e.k < 0; const er = e.err || {};
    const pinRow = (kind, label, len, sub) => {
      const on = e[kind];
      if (e.mode === kind) {
        return `<div class="su-pinset" role="group" aria-label="${label} festlegen">
          <div class="su-pinset-h">${icon('key', 20)}<b>${on ? `${label} ändern` : `${label} festlegen`}</b></div>
          <div class="su-two">
            <label class="field"><span>Neue ${label} (${len} Ziffern)</span><input class="inp su-digits" type="password" inputmode="numeric" maxlength="${len}" autocomplete="new-password" data-bind="ui.su.edit.p1" data-fid="su-e-${kind}1" value="${esc(e.p1 || '')}"></label>
            <label class="field"><span>Wiederholen</span><input class="inp su-digits" type="password" inputmode="numeric" maxlength="${len}" autocomplete="new-password" data-bind="ui.su.edit.p2" data-fid="su-e-${kind}2" value="${esc(e.p2 || '')}"></label>
          </div>
          ${err('su-e-pinerr', er.pin)}
          <div class="su-btns"><button type="button" class="btn ghost sm" data-act="suPinMode" data-m="" data-fid="su-e-pinx">Abbrechen</button>
            <button type="button" class="btn primary sm" data-act="suPinOk" data-fid="su-e-pinok">Übernehmen</button></div></div>`;
      }
      return `<div class="su-pinrow"><span class="lead">${icon(on ? 'lock' : 'key', 20)}</span>
        <span class="tx"><b>${label}</b><small>${on ? (e[kind + 'New'] ? 'Neu gesetzt · wird beim Speichern übernommen' : 'Aktiv') : esc(sub)}</small></span>
        <span class="su-btns">${on
          ? `<button type="button" class="btn ghost sm" data-act="suPinMode" data-m="${kind}" data-fid="su-e-${kind}ch">Ändern</button><button type="button" class="btn ghost sm su-dangert" data-act="suPinDrop" data-m="${kind}" data-fid="su-e-${kind}rm">Entfernen</button>`
          : `<button type="button" class="btn secondary sm" data-act="suPinMode" data-m="${kind}" data-fid="su-e-${kind}set">Setzen</button>`}</span></div>`;
    };
    const del = e.del
      ? `<div class="note warn su-mt12" role="alert">${icon('trash', 20)}<span>«${esc(e.name || S.profiles[e.k].name)}» wird gelöscht, mit Merkliste, Gesehen und Fortschritt. Das lässt sich nicht rückgängig machen.</span></div>
         <div class="acts"><button type="button" class="btn ghost" data-act="suDelAsk" data-v="0" data-fid="su-e-delno">Behalten</button><button type="button" class="btn danger" data-act="suDel" data-fid="su-e-delyes">Löschen</button></div>`
      : `<div class="acts"><button type="button" class="btn ghost" data-act="close" data-fid="su-e-cancel">Abbrechen</button><button type="button" class="btn accent" data-act="suSave" data-fid="su-e-save">${isNew ? 'Hinzufügen' : 'Speichern'}</button></div>`;
    return `<div class="su-edit"><div class="su-edhead"><span id="su-prev">${avatar({ name: (e.name || '').trim() || '?', color: e.color }, 56)}</span><h2>${isNew ? 'Profil hinzufügen' : 'Profil bearbeiten'}</h2></div>
      <div class="su-sb">
        <label class="field"><span>Name</span><input class="inp${er.name ? ' su-bad' : ''}" id="su-e-name" data-bind="ui.su.edit.name" data-fid="su-e-name" value="${esc(e.name)}" maxlength="20" autocomplete="off" aria-invalid="${!!er.name}" aria-describedby="su-e-nameerr">${err('su-e-nameerr', er.name)}</label>
        <div class="su-fs"><span class="su-lab" id="su-lab-col">Farbe</span>
          <div class="su-swatches" role="radiogroup" aria-labelledby="su-lab-col">${COLORS.map(([c, n]) => `<button type="button" class="su-swatch" role="radio" aria-checked="${e.color === c}" aria-label="${n}" data-act="suEdit" data-f="color" data-v="${c}" data-fid="su-e-c${n}"><span style="background:${c}">${e.color === c ? icon('check', 18) : ''}</span></button>`).join('')}</div></div>
        <div class="su-fs"><span class="su-lab" id="su-lab-age">Altersfreigabe</span>
          <div class="su-ages" role="radiogroup" aria-labelledby="su-lab-age">${AGES.map(a => `<button type="button" class="chip" role="radio" aria-checked="${e.fsk === a}" aria-pressed="${e.fsk === a}" data-act="suEdit" data-f="fsk" data-v="${a}" data-fid="su-e-a${a}">${e.fsk === a ? icon('check', 16) : ''}${a}</button>`).join('')}</div>
          <span class="hint">${e.fsk === 18 ? 'Dieses Profil sieht alle Titel.' : `Dieses Profil sieht nur Titel mit Freigabe bis ${e.fsk}.`}</span></div>
        <div class="su-fs"><span class="su-lab">Schutz</span>
          ${pinRow('pin', 'PIN', 4, 'Jeder kann ins Profil wechseln')}
          ${pinRow('opin', 'Offline-PIN', 6, 'Eigene, längere PIN für die Downloads ohne Netz')}</div>
        ${!isNew && !e.main && !e.del ? `<button type="button" class="btn danger block su-mt12" data-act="suDelAsk" data-v="1" data-fid="su-e-del">${icon('trash', 20)} Profil löschen</button>` : ''}
        ${e.main && !isNew ? '<p class="hint su-mt8">Das Hauptprofil lässt sich nicht löschen.</p>' : ''}
      </div>${del}</div>`;
  }
  const drawEdit = () => reSheet(editHtml());
  function openEdit(k, mode) {
    if (!me().main) { toast('Nur das Hauptprofil verwaltet Profile'); return; }
    const p = k >= 0 ? S.profiles[k] : { name: '', color: COLORS[(S.profiles.length) % COLORS.length][0], main: false, pin: false, opin: false, fsk: 12 };
    U.edit = { k, name: p.name, color: SEED_COLOR[String(p.color).toLowerCase()] || p.color, fsk: p.fsk ?? 18, pin: !!p.pin, opin: !!p.opin, main: !!p.main, mode: mode || '', p1: '', p2: '', err: {} };
    sheet(editHtml());
    focusFid(mode ? `su-e-${mode}1` : 'su-e-name');
  }
  ACT.suProf = d => openEdit(Number(d.k));
  ACT.suPinRec = () => openEdit(S.profiles.findIndex(p => p.main), 'pin');
  ACT.suEdit = d => { const e = U.edit; if (!e) return; e[d.f] = d.f === 'fsk' ? Number(d.v) : d.v; drawEdit(); };
  ACT.suPinMode = d => { const e = U.edit; e.mode = d.m; e.p1 = ''; e.p2 = ''; e.err = {}; drawEdit(); if (d.m) focusFid(`su-e-${d.m}1`); };
  ACT.suPinDrop = d => { const e = U.edit; e[d.m] = false; e[d.m + 'New'] = false; drawEdit(); toast(`${d.m === 'pin' ? 'PIN' : 'Offline-PIN'} wird beim Speichern entfernt`); };
  function applyPin() {
    const e = U.edit; const len = e.mode === 'opin' ? 6 : 4; const a = e.p1 || ''; const b = e.p2 || '';
    const msg = !new RegExp(`^\\d{${len}}$`).test(a) ? `Die ${e.mode === 'opin' ? 'Offline-PIN' : 'PIN'} hat genau ${len} Ziffern.`
      : a !== b ? 'Die beiden Eingaben stimmen nicht überein.'
      : e.mode === 'opin' && /^(\d)\1+$/.test(a) ? 'Wähle keine Ziffernfolge aus lauter gleichen Ziffern.' : '';
    if (msg) { e.err = { pin: msg }; return false; }
    e[e.mode] = true; e[e.mode + 'New'] = true; e.mode = ''; e.p1 = ''; e.p2 = ''; e.err = {};
    return true;
  }
  ACT.suPinOk = () => { const ok = applyPin(); drawEdit(); focusFid(ok ? 'su-e-save' : `su-e-${U.edit.mode}1`); };
  ACT.suDelAsk = d => { U.edit.del = d.v === '1'; drawEdit(); focusFid(U.edit.del ? 'su-e-delno' : 'su-e-del'); };
  ACT.suDel = () => {
    const e = U.edit; const k = e.k; const p = S.profiles[k]; if (!p || p.main) return;
    S.profiles.splice(k, 1);
    if (S.profile === k) S.profile = Math.max(0, S.profiles.findIndex(x => x.main)); else if (S.profile > k) S.profile -= 1;
    U.edit = null; closeSheet(true); toast(`Profil «${p.name}» gelöscht`); commit();
  };
  ACT.suSave = () => {
    const e = U.edit; if (!e) return;
    const name = String(e.name || '').trim();
    e.err = {};
    if (!name) e.err.name = 'Gib dem Profil einen Namen.';
    else if (S.profiles.some((p, k) => k !== e.k && p.name.toLowerCase() === name.toLowerCase())) e.err.name = 'Es gibt schon ein Profil mit diesem Namen.';
    if (e.mode) { const nameErr = e.err.name; if (!applyPin()) { if (nameErr) e.err.name = nameErr; drawEdit(); focusFid(`su-e-${e.mode}1`); return; } if (nameErr) e.err.name = nameErr; }
    if (e.err.name) { drawEdit(); focusFid('su-e-name'); return; }
    const data = { name, color: e.color, fsk: e.fsk, pin: e.pin, opin: e.opin };
    if (e.k < 0) S.profiles.push(Object.assign({ main: false }, data)); else Object.assign(S.profiles[e.k], data);
    const isNew = e.k < 0; U.edit = null;
    closeSheet(true); toast(isNew ? `Profil «${name}» hinzugefügt` : 'Gespeichert'); commit();
  };
  /* live initial in the avatar preview while typing the name */
  document.addEventListener('input', ev => {
    if (ev.target.id !== 'su-e-name') return;
    const a = $('#su-prev .avatar'); if (a) a.textContent = (ev.target.value.trim()[0] || '?');
  });

  /* ======================================================================
     /settings/playback
     ====================================================================== */
  const CHOICES = {
    qMobile: ['Qualität über Mobilfunk', ['Automatisch', 'Hoch (1080p)', 'Mittel (720p)', 'Niedrig (480p)', 'Datensparend (360p)'], 'Weniger Qualität braucht weniger Daten deines Abos.'],
    quality: ['Qualität im WLAN', ['Automatisch', 'Hoch (1080p)', 'Mittel (720p)', 'Niedrig (480p)'], 'Gilt für Streams. Die Qualität eines Downloads wählst du beim Herunterladen.'],
    audio: ['Bevorzugte Tonsprache', ['Deutsch', 'Englisch', 'Französisch', 'Italienisch', 'Originalsprache'], 'Fehlt die Sprache, nimmt der Player die erste Tonspur.'],
    subs: ['Untertitel', ['Aus', 'Nur erzwungene', 'Deutsch', 'Englisch'], '«Nur erzwungene» zeigt nur Stellen in fremder Sprache.'],
  };
  ACT.suChoice = d => {
    const [title, opts, hint] = CHOICES[d.c]; const cur = S.prefs[d.c];
    sheet(`<h2>${esc(title)}</h2><div class="su-opts" role="radiogroup" aria-label="${esc(title)}">${opts.map(o => `<button type="button" class="li su-opt" role="radio" aria-checked="${o === cur}" data-act="set" data-path="S.prefs.${d.c}" data-value="${esc(o)}"${o === cur ? ' autofocus' : ''}>
        <span class="tx"><b>${esc(o)}</b></span><span class="su-radio" aria-hidden="true"></span></button>`).join('')}</div>
      ${hint ? `<p class="hint su-sb su-mt8">${esc(hint)}</p>` : ''}
      <div class="acts"><button type="button" class="btn ghost" data-act="close">Abbrechen</button></div>`);
  };
  const pick = (c, ic, label) => li(ic, label, null, `<span class="su-val">${esc(S.prefs[c])}</span>${icon('right', 18)}`, `data-act="suChoice" data-c="${c}" data-fid="su-ch-${c}"`);

  route('/settings/playback', () => {
    prefs();
    const html = `${bar('Wiedergabe')}
      <div class="su-page">
        ${sec('Qualität')}${group([pick('qMobile', 'speed', 'Über Mobilfunk'), pick('quality', 'wifi', 'Im WLAN')])}
        ${sec('Sprache')}${group([pick('audio', 'audio', 'Bevorzugte Tonsprache'), pick('subs', 'subs', 'Untertitel')])}
        ${sec('Ablauf')}${group([
          sw('S', 'prefs.autoplay', 'Nächste Folge automatisch', 'Startet nach dem Abspann die nächste Folge'),
          sw('S', 'prefs.skipIntro', 'Vorspann überspringen anbieten', 'Zeigt einen Knopf, wenn der Server den Vorspann kennt'),
        ])}
        ${sec('Downloads')}${group([sw('S', 'prefs.wifiOnly', 'Downloads nur im WLAN', S.prefs.wifiOnly ? 'Über Mobilfunk wartet der Download' : 'Lädt auch über Mobilfunk')])}
        <p class="hint su-foot">Diese Einstellungen gelten für dieses Gerät.</p>
      </div>`;
    return { nav: 'more', html };
  });
  PAGES.push(['#/settings/playback', 'Einstellungen: Wiedergabe']);

  /* ======================================================================
     /settings/notifications
     ====================================================================== */
  route('/settings/notifications', () => {
    const p = prefs(); const main = me().main;
    const ch = (ic, name, on, sub, act) => main
      ? li(ic, name, sub, `<span class="badge${on ? ' ok' : ''}">${on ? 'Aktiv' : 'Aus'}</span>${icon('right', 18)}`, `data-act="${act}" data-fid="su-n-${act}"`)
      : `<div class="li su-static"><span class="lead">${icon(ic, 22)}</span><span class="tx"><b>${esc(name)}</b><small>${esc(sub)}</small></span><span class="end"><span class="badge${on ? ' ok' : ''}">${on ? 'Aktiv' : 'Aus'}</span></span></div>`;
    const html = `${bar('Benachrichtigungen')}
      <div class="su-page">
        ${sec('Auf diesem Gerät')}${group([sw('S', 'prefs.push', 'Push-Meldungen', p.push ? `Meldungen erscheinen auf diesem ${DEVICE}` : 'Dieses Gerät bekommt keine Meldungen')])}
        ${sec('Ereignisse')}${group([
          sw('S', 'prefs.ev.ep', 'Neue Folge', 'Bei Serien, die du schaust'),
          sw('S', 'prefs.ev.wish', 'Wunsch verfügbar', 'Ein gewünschter Titel ist in der Bibliothek'),
          sw('S', 'prefs.ev.dev', 'Neues Gerät verbunden', 'Empfohlen: an. So merkst du fremde Geräte'),
          sw('S', 'prefs.ev.ann', 'Ankündigungen', 'Mitteilungen der Verwaltung deines Servers'),
        ], p.push ? '' : 'su-dim')}
        ${p.push ? '' : '<p class="hint su-foot">Push ist aus. Die Ereignisse gelten weiter für E-Mail und Telegram.</p>'}
        ${sec('Kanäle des Kontos')}
        ${main ? '' : `<div class="note" role="note">${icon('lock', 20)}<span>Kanäle schaltet nur das Hauptprofil ein und aus.</span></div>`}
        ${group([
          ch('globe', 'E-Mail', p.mail, p.mail ? `An ${S.account}` : 'Ausgeschaltet', 'suMail'),
          ch('share', 'Telegram', p.tg, p.tg ? 'Verbunden' : 'Nicht verbunden', 'suTg'),
        ])}
        <div class="su-actrow"><button type="button" class="btn secondary" data-act="suTest" data-fid="su-n-test">${icon('bell', 20)} Testmeldung senden</button></div>
      </div>`;
    return { nav: 'more', html };
  });
  PAGES.push(['#/settings/notifications', 'Einstellungen: Benachrichtigungen']);

  ACT.suMail = () => {
    const on = prefs().mail;
    sheet(`<h2>Meldungen per E-Mail</h2><div class="su-sb"><p class="su-p">${on ? `Slouchcat schickt Meldungen an ${esc(S.account)}.` : 'E-Mail ist ausgeschaltet.'}</p>
      <p class="hint su-mt8">Gilt für das ganze Konto, auf allen Geräten.</p></div>
      <div class="acts"><button type="button" class="btn ghost" data-act="close">Abbrechen</button>
        <button type="button" class="btn ${on ? 'danger' : 'accent'}" data-act="suMailSet">${on ? 'Ausschalten' : 'Einschalten'}</button></div>`);
  };
  ACT.suMailSet = () => { S.prefs.mail = !S.prefs.mail; closeSheet(true); toast(S.prefs.mail ? 'E-Mail eingeschaltet' : 'E-Mail ausgeschaltet'); commit(); };
  ACT.suTg = () => {
    if (S.prefs.tg) {
      sheet(`<h2>Telegram</h2><div class="su-sb"><p class="su-p">Telegram ist mit deinem Konto verbunden. Meldungen kommen als Nachricht des Bots deines Servers.</p></div>
        <div class="acts"><button type="button" class="btn ghost" data-act="close">Schliessen</button><button type="button" class="btn danger" data-act="suTgSet">Trennen</button></div>`);
      return;
    }
    sheet(`<h2>Telegram verbinden</h2><div class="su-sb">
      <ol class="su-steps sm"><li><span class="su-n">1</span><span>Öffne in Telegram den Bot deines Servers.</span></li><li><span class="su-n">2</span><span>Sende ihm diesen Code:</span></li></ol>
      <p class="su-code" aria-label="Code 4 8 2 9 1 3">482 913</p><p class="hint su-center">Der Code gilt 10 Minuten.</p></div>
      <div class="acts"><button type="button" class="btn ghost" data-act="close">Abbrechen</button><button type="button" class="btn accent" data-act="suTgSet">Code gesendet</button></div>`);
  };
  ACT.suTgSet = () => { S.prefs.tg = !S.prefs.tg; closeSheet(true); toast(S.prefs.tg ? 'Telegram verbunden' : 'Telegram getrennt'); commit(); };
  ACT.suTest = () => {
    const p = prefs(); const where = [p.push ? 'dieses Gerät' : null, p.mail ? 'E-Mail' : null, p.tg ? 'Telegram' : null].filter(Boolean);
    toast(where.length ? `Testmeldung gesendet an ${where.join(', ')}` : 'Kein Kanal ist an. Schalte Push, E-Mail oder Telegram ein.');
  };

  /* ======================================================================
     /account
     ====================================================================== */
  route('/account', () => {
    prefs();
    const others = S.devices.filter(d => !d.here).length;
    const html = `${bar('Konto')}
      <div class="su-page">
        ${sec('Anmeldung')}${group([
          `<div class="li su-static"><span class="lead">${icon('user', 22)}</span><span class="tx"><b>${esc(S.account)}</b><small>E-Mail-Adresse und Anmeldename</small></span></div>`,
          li('key', 'Passwort ändern', others ? `Meldet ${plural(others, 'anderes Gerät', 'andere Geräte')} ab` : 'Aktuelles und neues Passwort', null, 'data-act="suPwdAsk" data-fid="su-a-pwd"'),
        ])}
        ${sec('Server')}${group([`<div class="li su-static"><span class="lead">${icon('server', 22)}</span><span class="tx"><b>${esc(host(S.server))}</b><small>Slouchcat ${VERSION}</small></span></div>`])}
        <p class="hint su-foot">Die Verwaltung des Servers gibt es nur im Browser.</p>
      </div>`;
    return { nav: 'more', html };
  });
  PAGES.push(['#/account', 'Konto']);

  function pwdHtml() {
    const f = U.pwd; const er = f.err || {}; const tvs = S.devices.filter(d => d.kind === 'tv').length; const others = S.devices.filter(d => !d.here).length;
    const inp = (k, label, ac, fid) => `<label class="field"><span>${label}</span><input class="inp${er[k] ? ' su-bad' : ''}" type="password" data-bind="ui.su.pwd.${k}" data-fid="${fid}" value="${esc(f[k] || '')}" autocomplete="${ac}" aria-invalid="${!!er[k]}" aria-describedby="${fid}-e"${f.busy ? ' disabled' : ''}>${err(fid + '-e', er[k])}</label>`;
    return `<h2>Passwort ändern</h2><div class="su-sb">
      <div class="note warn su-flush" role="note">${icon('alert', 20)}<span>Wenn du das Passwort änderst, wird jedes andere Gerät abgemeldet, auch die Fernseher${others ? ` (${plural(others, 'Gerät', 'Geräte')}${tvs ? `, davon ${plural(tvs, 'Fernseher', 'Fernseher')}` : ''})` : ''}. Fernseher verbindest du danach neu.</span></div>
      ${inp('cur', 'Aktuelles Passwort', 'current-password', 'su-pw-cur')}
      ${inp('n1', 'Neues Passwort', 'new-password', 'su-pw-n1')}
      ${inp('n2', 'Neues Passwort wiederholen', 'new-password', 'su-pw-n2')}
      ${er.n1 ? '' : '<p class="hint">Mindestens 8 Zeichen.</p>'}</div>
      <div class="acts"><button type="button" class="btn ghost" data-act="close"${f.busy ? ' disabled' : ''}>Abbrechen</button>
        <button type="button" class="btn accent" data-act="suPwdSave" data-fid="su-pw-save"${f.busy ? ' disabled' : ''}>${f.busy ? `${spin} Ändere …` : 'Passwort ändern'}</button></div>`;
  }
  ACT.suPwdAsk = () => { U.pwd = { cur: '', n1: '', n2: '', err: {} }; sheet(pwdHtml()); focusFid('su-pw-cur'); };
  ACT.suPwdSave = () => {
    const f = U.pwd; if (!f || f.busy) return;
    const er = {};
    if (!f.cur) er.cur = 'Gib dein aktuelles Passwort ein.';
    if ((f.n1 || '').length < 8) er.n1 = 'Das neue Passwort braucht mindestens 8 Zeichen.';
    else if (f.n1 === f.cur) er.n1 = 'Das neue Passwort ist gleich wie das alte.';
    if (!er.n1 && f.n2 !== f.n1) er.n2 = 'Die beiden Eingaben stimmen nicht überein.';
    f.err = er;
    const first = ['cur', 'n1', 'n2'].find(k => er[k]);
    if (first) { reSheet(pwdHtml()); focusFid('su-pw-' + first); return; }
    f.busy = true; reSheet(pwdHtml());
    later(() => {
      const gone = S.devices.filter(d => !d.here).length;
      S.devices = S.devices.filter(d => d.here); S.tvs = []; U.pwd = null;
      closeSheet(true); toast(gone ? `Passwort geändert. ${plural(gone, 'anderes Gerät', 'andere Geräte')} abgemeldet.` : 'Passwort geändert'); commit();
    }, 900);
  };
})();
