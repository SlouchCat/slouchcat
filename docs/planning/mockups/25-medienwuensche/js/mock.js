/* M-00: mockup controls (profile, Jellyseerr state, fast-forward, view) and the page index. */
'use strict';

const MOCK = {
  open: false,
  render() {
    const el = $('#mock');
    const p = curProfile(), ss = seerrState();
    const cond = !S.seerr.configured ? 'none' : S.seerr.condition;
    const o = (label, act, data, pressed) => `<button type="button" class="mk-o" data-act="${act}" ${Object.entries(data).map(([k, v]) => `data-${k}="${v}"`).join(' ')} aria-pressed="${!!pressed}" data-fid="mk:${act}:${Object.values(data).join('-')}">${label}</button>`;
    const open = S.wishes.filter((w) => OPEN_STATES.includes(w.state));
    const next = { queued: 'Übermitteln', pending: 'Freigeben', processing: 'Datei trifft ein', partial: 'Nächste Staffel trifft ein' };
    const panel = !MOCK.open ? '' : `<div class="mk-panel" role="region" aria-label="Mockup-Steuerung">
      <div style="display:flex;align-items:center;gap:8px"><h2 style="flex:1">Mockup-Steuerung</h2><button type="button" class="mk-o" data-act="mockToggle" data-fid="mk:close">Schliessen</button></div>
      <p>Hier spielst du durch, was sonst Jellyseerr, Radarr und die Zeit erledigen. Nichts davon gehört zur App.</p>
      <div><h3>Ansicht</h3><div class="mk-row">${o('Desktop', 'mockView', { v: 'desktop' }, S.view === 'desktop')}${o('Tablet', 'mockView', { v: 'tablet' }, S.view === 'tablet')}${o('Handy', 'mockView', { v: 'phone' }, S.view === 'phone')}</div></div>
      <div><h3>Wer schaut?</h3><div class="mk-row">${S.profiles.map((x) => o(`${esc(x.name)} · ${ageLabel(effMax(x))}`, 'mockProfile', { id: x.id }, x.id === p.id)).join('')}</div><p style="margin-top:6px">${esc(p.name)}: ${roleLabel(p)}, ${canRequest(p).ok ? 'darf anfordern als «' + esc(seerrUser(seerrUserFor(p).id).name) + '»' : 'darf nicht anfordern'}.</p></div>
      <div><h3>Jellyseerr ist …</h3><div class="mk-row">${o('Verbunden', 'mockSeerr', { c: 'ok' }, cond === 'ok')}${o('Nicht erreichbar', 'mockSeerr', { c: 'unreachable' }, cond === 'unreachable')}${o('Schlüssel ungültig', 'mockSeerr', { c: 'badkey' }, cond === 'badkey')}${o('CSRF-Schutz an', 'mockSeerr', { c: 'csrf' }, cond === 'csrf')}${o('Nicht eingerichtet', 'mockSeerr', { c: 'none' }, cond === 'none')}</div><div class="mk-row" style="margin-top:6px">${o('Abgleich jetzt ausführen', 'mockSync', {}, false)}</div></div>
      <div><h3>Offene Wünsche vorspulen</h3>${open.map((w) => `<div class="mk-w"><div class="g"><b>${esc(T[w.titleKey].title)}${w.seasons.length ? ' · S' + w.seasons.join(', S') : ''}</b><span>${STATE_LABEL[w.state]} · ${w.owner ? esc(profile(w.owner).name) : 'nicht zuordenbar'}</span></div>${o(next[w.state], 'mockAdvance', { id: w.id }, false)}${w.state === 'pending' ? o('Ablehnen', 'mockDecline', { id: w.id }, false) : ''}${w.state === 'processing' || w.state === 'partial' ? o('Scheitern', 'mockFail', { id: w.id }, false) : ''}${w.state === 'queued' ? o('24 h vergehen', 'mockTimeout', { id: w.id }, false) : ''}</div>`).join('') || '<p>Kein offener Wunsch. Fordere im Spotlight einen Titel an.</p>'}</div>
      <div><h3>Zuletzt zugestellt</h3>${S.deliveries.slice(0, 4).map((d) => `<div class="mk-d"><b>${CHANNEL_LABEL[d.channel]}</b> an ${esc(account(d.accountId).email)}<br>${esc(d.subject)}</div>`).join('') || '<p>Noch nichts.</p>'}</div>
      <div class="mk-row"><a href="#/mockup" data-act="mockToggle" data-fid="mk:pages">Alle Mockup-Seiten</a><span style="flex:1"></span>${o('Alles zurücksetzen', 'mockReset', {}, false)}</div>
    </div>`;
    const f = keepFocus();
    el.innerHTML = `${panel}<button type="button" class="mk-btn" data-act="mockToggle" aria-expanded="${MOCK.open}" data-fid="mk:toggle">${icon('settings', 18)}Mockup${ss.k !== 'ok' ? ' · Jellyseerr: ' + ss.label : ''}</button>`;
    if (f && f.fid.startsWith('mk:')) restoreFocus(f);
    const st = $('#stage');
    if (st.dataset.view !== S.view) { st.dataset.view = S.view; measure(); }
  },
};
window.MOCK = MOCK;
ACT.mockToggle = () => { MOCK.open = !MOCK.open; MOCK.render(); };
ACT.mockView = (d) => { S.view = d.v; save(); MOCK.render(); measure(); render(); };
ACT.mockProfile = (d) => { switchProfile(d.id); const pth = parseHash().path; if (pth.startsWith('/admin') && !isAdmin()) go('#/'); else render(); toast(`Aktives Profil: <b>${esc(curProfile().name)}</b>`, 'ok', 2500); };
ACT.mockSeerr = (d) => {
  if (d.c === 'none') { S.seerr.configured = false; S.seerr.enabled = false; }
  else { S.seerr.configured = true; S.seerr.enabled = true; S.seerr.condition = d.c; if (d.c !== 'ok') logSeerr('Abgleich', 'bad', d.c === 'badkey' ? 'HTTP 403: Schlüssel abgelehnt' : d.c === 'csrf' ? 'HTTP 403: CSRF-Schutz blockiert schreibende Aufrufe' : 'Zeitlimit nach 10 s: keine Antwort'); }
  commit();
};
ACT.mockSync = () => { const ok = syncNow(); toast(ok ? 'Abgleich ausgeführt.' : 'Abgleich nicht möglich: ' + seerrState().label + '.', ok ? 'ok' : 'bad'); render(); };
ACT.mockAdvance = (d) => { advanceWish(d.id); render(); };
ACT.mockDecline = (d) => { declineWish(d.id, 'in Jellyseerr'); render(); };
ACT.mockFail = (d) => { failWish(d.id); render(); };
/* a queued wish that Jellyseerr never accepted: after 24 hours it fails and the administrators hear about it */
ACT.mockTimeout = (d) => { const w = wishById(d.id); if (!w || w.state !== 'queued') return; w.state = 'failed'; w.failReason = 'timeout'; w.closed = Date.now(); ev(w, 'failed', FAIL_TEXT.timeout); logSeerr('Wunsch übermitteln', 'bad', `«${T[w.titleKey].title}»: nach 24 Stunden aufgegeben`); [w.owner, ...w.joined].filter(Boolean).forEach((pid) => notify(pid, 'wish_failed', w)); S.accounts.filter((a) => a.role === 'admin').forEach((a) => Object.entries(a.channels).forEach(([c, cfg]) => { if (cfg.enabled && cfg.events.includes('errors')) S.deliveries.unshift({ at: Date.now(), accountId: a.id, profileId: null, channel: c, event: 'errors', subject: `Wunsch nicht übermittelt: ${T[w.titleKey].title}`, state: 'sent' }); })); commit(); };
ACT.mockReset = () => { resetAll(); applyAccent(); toast('Mockup zurückgesetzt.', 'ok', 2500); go('#/mockup'); };

const MOCK_PAGES = [
  ['Zuschauer-App', [
    ['M-01', 'Startseite mit Spotlight', 'Fehlende, vorhandene und angeforderte Titel in Bühne und Posterreihe; Reihe «Deine Wünsche»', '#/'],
    ['M-02', 'Spotlight-Liste', 'Die ganze Trakt-Liste mit Filter', '#/lists/spotlight'],
    ['M-03', 'Vorschauseite Film', 'Fehlender Film: anfordern, Zustand, Trailer', '#/titles/movie/1368337'],
    ['M-04', 'Vorschauseite Serie', 'Fehlende Serie: Staffeln wählen', '#/titles/show/291507'],
    ['M-05', 'Detailseite Film', 'Vorhandener Film, unverändert', '#/items/masterplan'],
    ['M-06', 'Detailseite Serie', 'Teilweise vorhanden: fehlende Staffeln anfordern', '#/items/tulsa-king'],
    ['M-07', 'Suche', 'Bibliothek und «Noch nicht in der Bibliothek»', '#/search?q=the'],
    ['M-08', 'Meine Wünsche', 'Übersicht, Verlauf, Zurückziehen, erneut anfordern', '#/wishes'],
    ['M-09', 'Benachrichtigungen', 'Neue Meldungen mit Link zum Titel', '#/notifications'],
    ['M-10', 'Benachrichtigungskanäle', 'Neue Ereignisse je Kanal', '#/settings/notifications'],
    ['M-11', 'Profile', 'Schalter «Darf Titel anfordern» im Profildialog', '#/settings/profiles'],
    ['M-12', 'Profilwahl', '«Wer schaut?»', '#/profiles'],
  ]],
  ['Verwaltung', [
    ['M-20', 'Übersicht', 'Kennzahl und Hinweise zu Wünschen', '#/admin'],
    ['M-21', 'Integrationen', 'Alle Kacheln, neu: Jellyseerr', '#/admin/integrations'],
    ['M-22', 'Integrationen › Jellyseerr', 'Verbindung, Test, Abgleich, Protokoll', '#/admin/integrations/jellyseerr'],
    ['M-23', 'Wünsche › Anfragen', 'Alle Wünsche, Freigeben, Ablehnen, Diagnose', '#/admin/wishes'],
    ['M-24', 'Wünsche › Zuordnung', 'Konten und Profile den Jellyseerr-Benutzern zuordnen', '#/admin/wishes/mapping'],
    ['M-25', 'Wünsche › Einstellungen', 'Was erlaubt ist und woher', '#/admin/wishes/settings'],
    ['M-26', 'Konten', 'Wunschrecht je Konto, Anzeige der Zuordnung', '#/admin/accounts'],
    ['M-27', 'Spotlight', 'Band «Fehlende Titel», Einträge mit Zustand', '#/admin/spotlight'],
    ['M-28', 'Startseite', 'Neue Reihe «Deine Wünsche»', '#/admin/home-rows'],
    ['M-29', 'Benachrichtigungen', 'Kanäle mit den neuen Ereignissen', '#/admin/notifications'],
    ['M-30', 'Hintergrundjobs', 'Neue Jobarten und ihr Zustand', '#/admin/jobs'],
  ]],
];
route('/mockup', () => ({
  type: 'bare', title: 'Mockup-Übersicht',
  html: `<div class="mk-page"><div><p class="ahead"><span class="eye" style="color:var(--accent);font-size:12px;font-weight:700;letter-spacing:.08em">KONZEPT 25 · KLICKBARES MOCKUP</span></p><h1>Wünsche über Jellyseerr</h1><p class="sec" style="max-width:720px;margin-top:8px">Alles hier ist gespielt: kein Server, kein Jellyseerr, keine echte Anfrage. Was du anklickst, verändert nur den Zustand in diesem Browser. Die gelbe Schaltfläche «Mockup» unten rechts ersetzt Jellyseerr, Radarr und die Zeit.</p><div class="acts" style="margin-top:16px"><a class="btn primary lg" href="#/" data-fid="mp:start">${icon('play')}Zur Startseite</a><a class="btn secondary lg" href="#/admin" data-fid="mp:admin">${icon('shield')}Zur Verwaltung</a></div></div>
  <section class="stack"><h2>So probierst du es aus</h2><ol class="mk-steps"><li>Auf der Startseite im Spotlight zu einem Titel mit «Noch nicht in der Bibliothek» blättern und «Film anfordern» drücken.</li><li>Unten rechts «Mockup» öffnen und den Wunsch vorspulen: übermitteln, freigeben, Datei trifft ein.</li><li>Die Benachrichtigung öffnen (Glocke) und zum Titel springen. Er lässt sich jetzt abspielen.</li><li>Im Mockup das Profil wechseln (Noah braucht eine Freigabe, Mia darf nicht anfordern) und Jellyseerr ausfallen lassen.</li><li>In der Verwaltung Verbindung, Zuordnung, Einstellungen und Spotlight ausprobieren.</li></ol></section>
  ${MOCK_PAGES.map(([g, list]) => `<section class="stack"><h2>${g}</h2><div class="mk-grid">${list.map(([id, name, sub, href]) => `<a class="mk-card" href="${href}" data-fid="mp:${id}"><span class="id">${id}</span><b>${name}</b><span>${sub}</span></a>`).join('')}</div></section>`).join('')}
  <p class="muted" style="font-size:13px">Titel, Texte und Bilder: TMDB. Konten, Profile und Jellyseerr-Benutzer sind erfunden.</p></div>`,
}));

/* start */
applyAccent();
measure();
if (!location.hash) location.hash = '#/mockup';
render();
