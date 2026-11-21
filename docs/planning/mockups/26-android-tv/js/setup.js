/* Setup (with the phone or by hand) and the profile choice with PIN. Routes: /setup, /setup/phone, /setup/hand, /profiles. */
'use strict';

/* volatile state of this file */
const SU = { seed: 0, until: 0, phase: 'wait', timer: null, linkedAt: 0, auto: 0 };
const HD = { step: 1, url: '', check: null, httpOk: false, email: '', pw: '', show: false, busy: false, err: '' };
const PIN = { k: -1, v: '', busy: false, locks: {}, timer: null };
const SU_SERVER = 'https://medien.example.org'; const SU_ACCOUNT = 'marco@example.org'; const SU_VPN = 'https://100.84.12.7:8096';
const SU_VERSION = '1.41.3';
RESETS.push(() => {
  Object.assign(SU, { seed: 0, until: 0, phase: 'wait', linkedAt: 0, auto: 0 });
  Object.assign(HD, { step: 1, url: '', check: null, httpOk: false, email: '', pw: '', show: false, busy: false, err: '' });
  PIN.locks = {};
});

/* ---------- drawn QR code: three finder patterns, timing lines, seeded pattern ---------- */
function qrSvg(seed) {
  const n = 29; let s = (seed >>> 0) || 1;
  const rnd = () => { s = (Math.imul(s, 1664525) + 1013904223) >>> 0; return s / 4294967296; };
  const inFinder = (x, y) => (x < 8 && y < 8) || (x > n - 9 && y < 8) || (x < 8 && y > n - 9);
  const inAlign = (x, y) => x >= n - 9 && x <= n - 5 && y >= n - 9 && y <= n - 5;
  let d = '';
  for (let y = 0; y < n; y++) {
    for (let x = 0; x < n; x++) {
      if (inFinder(x, y) || inAlign(x, y)) continue;
      const dark = (y === 6 || x === 6) ? (x + y) % 2 === 0 : rnd() < 0.48;
      if (dark) d += `M${x} ${y}h1v1h-1z`;
    }
  }
  const finder = (x, y) => `M${x} ${y}h7v7h-7zM${x + 1} ${y + 1}v5h5v-5zM${x + 2} ${y + 2}h3v3h-3z`;
  const a = n - 9;
  d += finder(0, 0) + finder(n - 7, 0) + finder(0, n - 7) + `M${a} ${a}h5v5h-5zM${a + 1} ${a + 1}v3h3v-3zM${a + 2} ${a + 2}h1v1h-1z`;
  return `<svg class="qr" viewBox="-4 -4 ${n + 8} ${n + 8}" role="img" aria-label="QR-Code zum Verbinden" shape-rendering="crispEdges"><rect x="-4" y="-4" width="${n + 8}" height="${n + 8}" class="qr-bg"/><path class="qr-fg" fill-rule="evenodd" d="${d}"/></svg>`;
}
function newCode() { SU.seed = Math.floor(Math.random() * 1e9) + 1; SU.until = Date.now() + 5 * 60 * 1000; SU.phase = 'wait'; }
const mmss = ms => { const t = Math.max(0, Math.ceil(ms / 1000)); return `${Math.floor(t / 60)}:${String(t % 60).padStart(2, '0')}`; };

/* ---------- /setup: two ways ---------- */
route('/setup', () => ({
  full: true, root: NAVSTK.length <= 1,
  html: `<div class="page su su-choose">
    <div class="su-brand">${wordmark()}</div>
    <h1 class="su-h1">Willkommen bei Slouchcat</h1>
    <p class="su-lead">Verbinde diesen Fernseher mit deinem Slouchcat-Server.</p>
    <div class="su-choices">
      <a class="su-choice" href="#/setup/phone" data-f="su-phone" data-default>
        <span class="su-ci">${icon('phone', 72)}</span><span class="badge acc">Empfohlen</span>
        <b>Mit dem Handy einrichten</b><span>Code scannen und am Handy bestätigen. Am Fernseher tippst du nichts.</span></a>
      <a class="su-choice" href="#/setup/hand" data-f="su-hand">
        <span class="su-ci">${icon('keyboard', 72)}</span><span class="badge">Ohne Handy</span>
        <b>Von Hand einrichten</b><span>Serveradresse, E-Mail und Passwort mit der Fernbedienung eingeben.</span></a>
    </div></div>`,
}));

/* ---------- /setup/phone ---------- */
function phoneWait() {
  const left = SU.until - Date.now(); const off = SU.phase === 'expired';
  return `<div class="page su su-phone">
    <div class="su-cols">
      <div class="su-left">
        <h1 class="su-h1">Mit dem Handy einrichten</h1>
        <ol class="su-steps">
          <li><b>1</b><span>Öffne die Slouchcat-App am Handy und wähle «Fernseher verbinden».</span></li>
          <li><b>2</b><span>Scanne diesen Code mit der App.</span></li>
          <li><b>3</b><span>Bestätige am Handy. Der Fernseher richtet sich dann selbst ein.</span></li>
        </ol>
        ${off ? `<div class="su-wait done">${icon('clock', 34)}Der Code ist abgelaufen.</div>` : `<div class="su-wait" role="status"><span class="su-pulse" aria-hidden="true"></span>Warte auf dein Handy …</div>`}
        <div class="su-acts">
          <button type="button" class="tb${off ? ' primary' : ''}" data-f="su-new" data-act="suNew" data-default>${icon('refresh', 32)}<span>Neuen Code anzeigen</span></button>
          <a class="tb ghost" href="#/setup/hand" data-f="su-tohand">${icon('keyboard', 32)}<span>Von Hand einrichten</span></a>
        </div>
      </div>
      <div class="su-right">
        <div class="su-qrbox${off ? ' off' : ''}">${qrSvg(SU.seed)}${off ? `<span class="su-qrx">${icon('clock', 64)}Abgelaufen</span>` : ''}</div>
        <p class="su-time" id="su-time">${off ? 'Zeige einen neuen Code an.' : `Code gültig noch ${mmss(left)}`}</p>
      </div>
    </div>
    <div class="mockvar"><span class="mk">Varianten (nur im Mockup)</span>
      <button type="button" class="tb sm" data-act="suVar" data-v="linked" data-f="v-linked">Handy hat verbunden</button>
      <button type="button" class="tb sm" data-act="suVar" data-v="nosvc" data-f="v-nosvc">Dienst nicht erreichbar</button>
      <button type="button" class="tb sm" data-act="suVar" data-v="noaddr" data-f="v-noaddr">Adresse nicht erreichbar</button>
      <button type="button" class="tb sm" data-act="suVar" data-v="expired" data-f="v-exp">Code läuft ab</button>
    </div></div>`;
}
function phoneLinked() {
  return `<div class="page su su-done">
    <span class="su-check">${icon('check', 104)}</span>
    <h1 class="su-h1">Verbunden</h1>
    <p class="su-lead">Dieser Fernseher ist jetzt mit diesem Server und diesem Konto verbunden.</p>
    <dl class="su-dl">
      <div><dt>Server</dt><dd>${esc(hostOf(S.server))} <span class="badge ok">${icon('lock', 26)} verschlüsselt</span></dd></div>
      <div><dt>Konto</dt><dd>${esc(S.account)}</dd></div>
      <div><dt>Name des Fernsehers</dt><dd>${esc(S.tvName)}</dd></div>
    </dl>
    <p class="su-hint">Kennst du Server oder Konto nicht? Dann trenne die Verbindung und richte den Fernseher neu ein.</p>
    <div class="su-acts c">
      <button type="button" class="tb primary" data-f="su-go" data-act="suToProfiles" data-default>${icon('users', 32)}<span>Weiter zu den Profilen <span id="su-auto"></span></span></button>
      <button type="button" class="tb ghost" data-f="su-undo" data-act="suUndo">${icon('close', 32)}<span>Nicht mein Konto, trennen</span></button>
    </div></div>`;
}
function phoneProblem(kind) {
  const nosvc = kind === 'nosvc';
  return `<div class="page su su-done">
    <span class="su-check bad">${icon('alert', 96)}</span>
    <h1 class="su-h1">${nosvc ? 'Dienst nicht erreichbar' : 'Adresse nicht erreichbar'}</h1>
    ${nosvc ? '<p class="su-lead">Der Vermittlungsdienst für das Einrichten mit dem Handy antwortet gerade nicht. Von Hand klappt es auch ohne ihn.</p>'
    : `<p class="su-lead">Dein Handy hat diese Serveradresse geschickt:</p><p class="su-addr">${esc(SU_VPN)}</p>
       <p class="su-lead">Der Fernseher erreicht sie nicht. Vielleicht nutzt dein Handy ein VPN oder ein anderes Netz.</p>`}
    <div class="su-acts c">
      <a class="tb primary" href="#/setup/hand" data-f="su-tohand" data-default>${icon('keyboard', 32)}<span>Von Hand einrichten</span></a>
      <button type="button" class="tb ghost" data-f="su-retry" data-act="suNew">${icon('refresh', 32)}<span>${nosvc ? 'Nochmals versuchen' : 'Neuen Code anzeigen'}</span></button>
    </div></div>`;
}
route('/setup/phone', () => {
  if (!SU.until) newCode();
  if (SU.phase === 'wait' && Date.now() >= SU.until) SU.phase = 'expired';
  const html = SU.phase === 'linked' ? phoneLinked() : SU.phase === 'nosvc' || SU.phase === 'noaddr' ? phoneProblem(SU.phase) : phoneWait();
  return {
    full: true, html,
    back: () => {
      if (SU.phase === 'linked') { ACT.suToProfiles(); return true; }
      if (SU.phase === 'nosvc' || SU.phase === 'noaddr') { newCode(); render(); return true; }
      return false;
    },
    after: () => { clearInterval(SU.timer); SU.timer = setInterval(suTick, 500); suTick(); },
  };
});
function suTick() {
  if (curPath() !== '/setup/phone') { clearInterval(SU.timer); return; }
  if (SU.phase === 'wait') {
    const left = SU.until - Date.now();
    if (left <= 0) { SU.phase = 'expired'; render(); focusTo('su-new'); return; }
    const t = $('#su-time'); if (t) t.textContent = `Code gültig noch ${mmss(left)}`;
  } else if (SU.phase === 'linked') {
    const a = $('#su-auto'); if (!a) return;
    if (lastInput > SU.linkedAt) { a.textContent = ''; return; }
    const left = 10000 - (Date.now() - SU.linkedAt);
    if (left <= 0) { ACT.suToProfiles(); return; }
    a.textContent = `(${Math.ceil(left / 1000)} s)`;
  }
}
ACT.suNew = () => { newCode(); render(); focusTo('su-new'); };
ACT.suVar = d => {
  if (d.v === 'linked') {
    Object.assign(S, { setup: true, server: SU_SERVER, account: SU_ACCOUNT, secure: true }); save();
    SU.phase = 'linked'; SU.linkedAt = Date.now() + 50; render();
  } else if (d.v === 'expired') { SU.until = Date.now() + 4000; SU.phase = 'wait'; render(); toast('Der Code läuft in 4 Sekunden ab'); } else { SU.phase = d.v; render(); }
};
ACT.suToProfiles = () => { SU.phase = 'wait'; SU.until = 0; go('#/profiles', { replace: true }); };
ACT.suUndo = () => { S.setup = false; S.server = ''; save(); newCode(); render(); toast('Verbindung getrennt'); };

/* ---------- /setup/hand: three steps with the on-screen keyboard ---------- */
const HD_STEPS = [['Server', 'Adresse des Servers'], ['E-Mail', 'deines Kontos'], ['Passwort', 'deines Kontos']];
function tfield(f, label, value, ph, act, extra) {
  return `<div class="tf-wrap"><span class="tf-lab">${esc(label)}</span>
    <button type="button" class="tf" data-f="${f}" data-act="${act}"${extra || ''}>${value ? `<span class="tf-v">${esc(value)}</span>` : `<span class="tf-ph">${esc(ph)}</span>`}<span class="tf-ic">${icon('keyboard', 34)}</span></button></div>`;
}
function hdCheck() {
  const c = HD.check;
  if (!c) return '';
  if (c === 'busy') return `<div class="su-busy" role="status"><span class="spin" aria-hidden="true"></span>Prüfe die Adresse …</div>`;
  if (c === 'err') return `<div class="tnote bad" role="alert">${icon('alert', 36)}<p>Unter dieser Adresse antwortet kein Slouchcat-Server. Prüfe Schreibweise und Port.</p></div>`;
  return `<div class="su-result">${icon('server', 48)}<div class="su-rtx"><b>${esc(c.name)}</b><span>Slouchcat ${SU_VERSION} · ${esc(c.host)}</span></div>
      ${c.secure ? `<span class="badge ok">${icon('lock', 26)} verschlüsselt</span>` : `<span class="badge warn">${icon('unlock', 26)} nicht verschlüsselt</span>`}</div>
    ${c.secure ? '' : `<div class="tnote warn">${icon('alert', 36)}<p>Diese Verbindung ist nicht verschlüsselt. In fremden Netzen kann jemand dein Passwort mitlesen und deinen Zugang übernehmen.</p></div>
      <button type="button" class="tsw" role="switch" aria-checked="${HD.httpOk}" data-f="su-risk" data-act="hdRisk"><span class="tsw-tx">Ich verstehe das und will trotzdem verbinden</span><span class="sw" aria-hidden="true"></span></button>`}`;
}
function hdBody() {
  const okUrl = HD.check && typeof HD.check === 'object' && (HD.check.secure || HD.httpOk);
  if (HD.step === 1) {
    return `<h1 class="su-h1">Serveradresse</h1><p class="su-lead">Die Adresse, unter der du Slouchcat im Browser öffnest.</p>
      ${tfield('su-url', 'Serveradresse', HD.url, 'z. B. https://medien.example.org', 'hdEdit', HD.url ? '' : ' data-default')}
      ${hdCheck()}
      <div class="su-acts"><button type="button" class="tb primary" data-f="su-next" data-act="hdNext"${okUrl ? ' data-default' : ' aria-disabled="true"'}>${icon('right', 32)}<span>Weiter</span></button></div>
      <p class="mockhint"><span class="mk">Nur im Mockup</span> Jede Adresse antwortet, ausser sie enthält «fehler». Mit http:// erscheint die Warnung.</p>`;
  }
  if (HD.step === 2) {
    const ok = /.+@.+\..+/.test(HD.email);
    return `<h1 class="su-h1">E-Mail</h1><p class="su-lead">Die E-Mail, mit der du dich bei Slouchcat anmeldest.</p>
      ${tfield('su-mail', 'E-Mail', HD.email, 'name@example.org', 'hdEdit', HD.email ? '' : ' data-default')}
      <div class="su-acts"><button type="button" class="tb primary" data-f="su-next" data-act="hdNext"${ok ? ' data-default' : ' aria-disabled="true"'}>${icon('right', 32)}<span>Weiter</span></button></div>`;
  }
  const masked = HD.show ? HD.pw : '•'.repeat(HD.pw.length);
  return `<h1 class="su-h1">Passwort</h1><p class="su-lead">Das Passwort deines Kontos bei ${esc(hostOf(HD.url))}.</p>
    <div class="tf-row">${tfield('su-pw', 'Passwort', masked, 'Passwort eingeben', 'hdEdit', HD.pw ? '' : ' data-default')}
      <button type="button" class="tb ghost tf-side" data-f="su-show" data-act="hdShow" aria-pressed="${HD.show}">${icon(HD.show ? 'eyeOff' : 'eye', 32)}<span>${HD.show ? 'verbergen' : 'anzeigen'}</span></button></div>
    ${HD.err ? `<div class="tnote bad" role="alert">${icon('alert', 36)}<p>${esc(HD.err)}</p></div>` : ''}
    ${HD.busy ? `<div class="su-busy" role="status"><span class="spin" aria-hidden="true"></span>Melde an …</div>` : ''}
    <div class="su-acts"><button type="button" class="tb primary" data-f="su-login" data-act="hdLogin"${HD.pw && !HD.busy ? ' data-default' : ' aria-disabled="true"'}>${icon('check', 32)}<span>Anmelden</span></button></div>
    <p class="mockhint"><span class="mk">Nur im Mockup</span> Jedes Passwort geht, ausser «falsch».</p>`;
}
route('/setup/hand', () => {
  const vals = [hostOf(HD.url), HD.email, HD.pw ? '•'.repeat(HD.pw.length) : ''];
  const prog = HD_STEPS.map(([t, s], k) => {
    const n = k + 1; const st = n < HD.step ? 'done' : n === HD.step ? 'cur' : '';
    return `<li class="${st}"><span class="su-n">${n < HD.step ? icon('check', 32) : n}</span><span class="su-pt"><b>${t}</b><small>${esc(n < HD.step && vals[k] ? vals[k] : s)}</small></span></li>`;
  }).join('');
  return {
    full: true,
    html: `<div class="page su su-hand"><div class="su-brand">${wordmark()}<span class="su-brsub">Von Hand einrichten</span></div>
      <div class="su-hcols"><ol class="su-prog" aria-label="Schritte">${prog}</ol><div class="su-form">${hdBody()}</div></div></div>`,
    back: () => { if (HD.busy) return true; if (HD.step > 1) { HD.step -= 1; HD.err = ''; render(); return true; } return false; },
  };
});
ACT.hdEdit = d => {
  const f = { 'su-url': ['url', 'Serveradresse', 'url', ''], 'su-mail': ['email', 'E-Mail', 'email', ''], 'su-pw': ['pw', 'Passwort', 'password', ''] }[d.f];
  if (!f) return;
  const [field, label, type, start] = f;
  keyboard({
    value: HD[field] || start, label, type, placeholder: field === 'url' ? 'medien.example.org' : '',
    onDone: v => {
      HD[field] = v.trim(); HD.err = '';
      if (field === 'url') { hdVerify(); return; }
      render();
      if (field === 'email' && /.+@.+\..+/.test(HD.email)) focusTo('su-next');
      if (field === 'pw' && HD.pw) focusTo('su-login');
    },
  });
};
function hdVerify() {
  let u = HD.url; if (!u || u === 'https://' || u === 'http://') { HD.url = ''; HD.check = null; render(); return; }
  u = u.replace(/^(https?:\/\/)+(https?:\/\/)/i, '$2');
  if (!/^https?:\/\//i.test(u)) u = 'https://' + u;
  HD.url = u.replace(/\/+$/, ''); HD.check = 'busy'; HD.httpOk = false; render();
  setTimeout(() => {
    if (curPath() !== '/setup/hand') return;
    const host = hostOf(HD.url);
    HD.check = /fehler/i.test(HD.url) ? 'err' : { secure: /^https:/i.test(HD.url), host, name: `Slouchcat bei ${host.split(/[.:]/)[0] || 'dir'}` };
    render();
    focusTo(HD.check === 'err' ? 'su-url' : HD.check.secure ? 'su-next' : 'su-risk');
  }, 900);
}
ACT.hdRisk = () => { HD.httpOk = !HD.httpOk; render(); if (HD.httpOk) focusTo('su-next'); };
ACT.hdNext = () => { HD.step += 1; HD.err = ''; render(); focusTo(HD.step === 2 ? (HD.email ? 'su-next' : 'su-mail') : (HD.pw ? 'su-login' : 'su-pw')); };
ACT.hdShow = () => { HD.show = !HD.show; render(); };
ACT.hdLogin = () => {
  HD.busy = true; HD.err = ''; render();
  setTimeout(() => {
    HD.busy = false;
    if (curPath() !== '/setup/hand') return;
    if (HD.pw === 'falsch') { HD.err = 'E-Mail oder Passwort stimmt nicht.'; render(); focusTo('su-pw'); return; }
    Object.assign(S, { setup: true, server: HD.url, account: HD.email, secure: /^https:/i.test(HD.url) }); save();
    const host = hostOf(HD.url);
    Object.assign(HD, { step: 1, url: '', check: null, httpOk: false, email: '', pw: '', show: false });
    go('#/profiles', { replace: true }); toast(`Verbunden mit ${host}`);
  }, 1000);
};

/* ---------- /profiles: who is watching, with PIN pad ---------- */
route('/profiles', () => ({
  full: true, nav: 'profiles', root: NAVSTK.length <= 1 || NAVSTK[NAVSTK.length - 2].startsWith('#/setup'),
  html: `<div class="page su su-prof">
    <h1 class="su-h1 c">Wer schaut?</h1>
    <div class="pf-row" data-row="profiles">${S.profiles.map((p, k) => `<button type="button" class="pf" data-f="pf-${k}" data-act="pfPick" data-k="${k}"${k === S.profile ? ' data-default' : ''} aria-label="${esc(p.name)}${p.pin ? ', mit PIN geschützt' : ''}">
      <span class="pf-av">${avatar(p, 200)}${p.pin ? `<span class="pf-lock">${icon('lock', 32)}</span>` : ''}</span><b>${esc(p.name)}</b></button>`).join('')}</div>
    <div class="pf-acts"><a class="tb ghost" href="#/settings/profiles" data-f="pf-manage">${icon('edit', 32)}<span>Profile verwalten</span></a></div>
    <p class="pf-conn">${icon('server', 30)}<span>Verbunden mit <b>${esc(hostOf(S.server) || 'unbekannt')}</b> · ${esc(S.account)}</span></p></div>`,
}));
function pfChoose(k) { S.profile = k; save(); go('#/'); toast(`Hallo ${me().name}`); }
ACT.pfPick = d => {
  const k = Number(d.k); const p = S.profiles[k];
  if (!p.pin) { pfChoose(k); return; }
  PIN.k = k; PIN.v = ''; PIN.busy = false;
  const key = (v, label, cls, extra) => `<button type="button" class="pin-k${cls ? ' ' + cls : ''}" data-f="pin-${v}" data-act="pinKey" data-d="${v}"${extra || ''}>${label}</button>`;
  menu(`<div class="pin" id="pin">${avatar(p, 120)}<h2>PIN für ${esc(p.name)}</h2>
      <div class="pin-dots" aria-hidden="true"><i></i><i></i><i></i><i></i></div>
      <p class="pin-msg" role="status" aria-live="polite">Gib die vierstellige PIN ein.</p>
      <div class="pin-pad">${[1, 2, 3, 4, 5, 6, 7, 8, 9].map(n => key(n, n, '', n === 1 ? ' data-default' : '')).join('')}
        ${key('del', icon('backspace', 40), 'fn', ' aria-label="Löschen"')}${key(0, 0)}${key('x', 'Abbrechen', 'fn')}</div></div>`, {
    dialog: true,
    onKey: (name, e) => {
      if (e && /^[0-9]$/.test(e.key)) { pinPress(e.key); return true; }
      if (e && e.key === 'Backspace') { pinPress('del'); return true; }
      return false;
    },
    onClose: () => { clearInterval(PIN.timer); PIN.k = -1; },
  });
  drawPin();
};
ACT.pinKey = d => pinPress(String(d.d));
const pinLock = k => PIN.locks[k] || (PIN.locks[k] = { tries: 0, until: 0 });
function drawPin(msg) {
  const box = $('#pin'); if (!box || PIN.k < 0) return;
  const L = pinLock(PIN.k); const left = Math.ceil((L.until - Date.now()) / 1000); const locked = left > 0;
  $$('.pin-dots i', box).forEach((d, n) => d.classList.toggle('on', n < PIN.v.length));
  box.classList.toggle('locked', locked);
  const m = $('.pin-msg', box);
  if (locked) m.innerHTML = `${icon('lock', 30)} Zu viele falsche Versuche. Warte noch ${left} s.`;
  else if (msg) m.innerHTML = msg;
  clearInterval(PIN.timer);
  if (locked) {
    PIN.timer = setInterval(() => {
      if (!$('#pin')) { clearInterval(PIN.timer); return; }
      if (Date.now() >= L.until) { L.until = 0; L.tries = 0; clearInterval(PIN.timer); drawPin('Du kannst es wieder versuchen.'); return; }
      $('.pin-msg').innerHTML = `${icon('lock', 30)} Zu viele falsche Versuche. Warte noch ${Math.ceil((L.until - Date.now()) / 1000)} s.`;
    }, 250);
  }
}
function pinPress(d) {
  if (PIN.k < 0 || PIN.busy) return;
  if (d === 'x') { closeMenu(); return; }
  const L = pinLock(PIN.k); if (L.until > Date.now()) { drawPin(); return; }
  if (d === 'del') { PIN.v = PIN.v.slice(0, -1); drawPin(); return; }
  if (PIN.v.length >= 4) return;
  PIN.v += d; drawPin();
  if (PIN.v.length < 4) return;
  PIN.busy = true;
  setTimeout(() => {
    PIN.busy = false; const box = $('#pin'); if (!box) return;
    if (PIN.v !== '0000') { const k = PIN.k; L.tries = 0; closeMenu(); pfChoose(k); return; }
    L.tries += 1; PIN.v = '';
    box.classList.remove('shake'); void box.offsetWidth; box.classList.add('shake');
    if (L.tries >= 3) { L.until = Date.now() + 30000; drawPin(); return; }
    const rest = 3 - L.tries;
    drawPin(`${icon('alert', 30)} PIN falsch. Noch ${rest} ${rest === 1 ? 'Versuch' : 'Versuche'}.`);
  }, 350);
}

PAGES.push(['#/setup', 'Einrichten: Auswahl'], ['#/setup/phone', 'Einrichten mit dem Handy'], ['#/setup/hand', 'Einrichten von Hand'], ['#/profiles', 'Wer schaut? (Profile)']);
