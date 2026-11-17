/* Administration, part A: M-20 overview, M-21 integrations with all detail views, M-22 Seerr,
   M-29 notifications, M-30 background jobs. Mockup only: every value is simulated in S / ui. */
'use strict';

/* ---------- shared bits ---------- */
const AA_HOST = 'http://slouchcat.lan:8096';
const aaNew = () => '<span class="newtag">NEU</span>';
function aaTok(n = 24) { const c = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789'; let s = ''; for (let i = 0; i < n; i++) s += c[Math.floor(Math.random() * c.length)]; return s; }
function aaCopy(text, msg) { try { if (navigator.clipboard) navigator.clipboard.writeText(text).catch(() => {}); } catch (e) { /* mockup: clipboard optional */ } toast(msg || 'Kopiert.', 'ok', 2600); }
function aaErr(id) { return (ui.aaErr || {})[id]; }
function aaSetErr(id, text) { ui.aaErr = ui.aaErr || {}; if (text) ui.aaErr[id] = text; else delete ui.aaErr[id]; }
function aaErrHtml(id) { const e = aaErr(id); return e ? `<p class="err" id="err-${id}">${esc(e)}</p>` : ''; }
function aaInv(id) { return aaErr(id) ? `aria-invalid="true" aria-describedby="err-${id}"` : ''; }
function aaDur(ms) {
  const m = Math.max(1, Math.round(ms / 60e3));
  if (m < 60) return `${m} min`;
  const h = Math.round(m / 60);
  if (h < 48) return `${h} h`;
  return `${Math.round(h / 24)} Tagen`;
}
function aaWhen(ts) { return `${fmtDate(ts)}, ${fmtTime(ts)}`; }
const aaSrow = (title, help, ctl, o = {}) => `<div class="${cls('srow', o.isNew && 'new')}"><div class="st"><b>${title}</b>${help ? `<span>${help}</span>` : ''}</div><div class="${cls('sc', o.col && 'col')}">${ctl}</div></div>`;
const aaInp = (path, fid, o = {}) => `<input class="${cls('inp', o.num && 'num')}" ${o.type ? `type="${o.type}"` : ''} data-bind="${path}" data-fid="${fid}" value="${esc(getPath(path) ?? '')}" ${o.ph ? `placeholder="${esc(o.ph)}"` : ''} aria-label="${esc(o.label || '')}" ${o.dis ? 'disabled' : ''} ${o.err ? aaInv(o.err) : ''} ${o.type === 'password' ? 'autocomplete="new-password"' : ''} ${o.num ? 'inputmode="numeric" data-num' : ''} ${o.live ? 'data-live' : ''}>`;

/* generic confirmation dialog: the confirm button calls ACT[act] with data-id */
DIALOGS.aaConfirm = (p) => ({
  title: p.title,
  body: `<p class="lead">${p.body}</p>${p.extra || ''}`,
  actions: `<button type="button" class="btn secondary" data-act="close" data-fid="aac:keep" data-autofocus>${p.keep || 'Abbrechen'}</button><button type="button" class="btn ${p.danger ? 'danger' : 'primary'}" data-act="${p.act}" data-id="${esc(p.id || '')}" data-fid="aac:ok">${p.confirm}</button>`,
});
function aaConfirm(o) { openDialog('aaConfirm', o); }

/* ---------- integrations: simulated settings, read with defaults ---------- */
function aaInt() {
  if (!S.integrations) {
    const now = Date.now();
    S.integrations = {
      keys: {
        tmdb: { src: 'stored', tail: { a: '9f3c' }, rejected: false },
        tvdb: { src: 'stored', tail: { a: 'k2p8', b: '' }, rejected: false },
        fanart: { src: 'stored', tail: { a: 'm4w1', b: '' }, rejected: false },
        youtube: { src: 'stored', tail: { a: 'q7xd' }, rejected: false },
        trakt: { src: 'env', tail: { a: 'e5c0' }, rejected: false },
        googlebooks: { src: 'none', tail: { a: '' }, rejected: false },
      },
      webhooks: [
        { id: 'h1', name: 'Radarr', kind: 'radarr', tail: 'zBQo', last: now - 2 * H, map: [{ from: '/mnt/medien/Filme', to: '/media/filme' }],
          log: [{ at: now - 2 * H, ev: 'Download', rep: '/mnt/medien/Filme/Godzilla Minus One (2023)', res: 'scan_queued' }, { at: now - 26 * H, ev: 'Rename', rep: '/mnt/medien/Filme/Dune (2021)', res: 'scan_queued' }, { at: now - 9 * D, ev: 'Test', rep: '', res: 'acknowledged' }] },
        { id: 'h2', name: 'Sonarr', kind: 'sonarr', tail: 'Lm4t', last: now - 9 * D, map: [{ from: '/mnt/medien/Serien', to: '/media/serien' }],
          log: [{ at: now - 9 * D, ev: 'Download', rep: '/mnt/medien/Serien/Mobland/Season 02', res: 'scan_queued' }, { at: now - 9 * D - H, ev: 'Download', rep: '/tmp/sonarr/incoming', res: 'outside_roots' }] },
      ],
      email: { host: 'smtp.example.ch', port: 587, user: 'media@example.ch', pwTail: 'x9k2', from: 'Slouchcat <media@example.ch>', tls: 'starttls' },
      telegram: { bot: 'slouchcat_bot', tail: 'Ab3Z' },
      push: { has: true, pub: 'BJx3kQ9vT2mN7pL0sR4wY8zC1dF6gH5jK3lM9nB2vC7xZ4qW1eR8tY6uI0oP3aS5dF7gH9jK2lZ' },
      server: { update: true, publicUrl: 'https://media.example.ch', gpu: 'auto' },
    };
    save();
  }
  return S.integrations;
}
const AA_PROV = [
  { k: 'tmdb', n: 'TMDB', p: 'Texte, Poster und Trailer für Filme und Serien. Pflicht für Video-Bibliotheken.', h: 'Kostenlos im TMDB-Konto unter «API»: das Lesezugriffs-Token (v4).', f: [['a', 'Zugriffstoken (v4)']] },
  { k: 'tvdb', n: 'TVDB', p: 'Zweite Quelle für Serien und Anime: Staffeln und Folgentitel.', h: 'Schlüssel und PIN stehen im TVDB-Konto. Ohne Abo bleibt die PIN leer.', f: [['a', 'Schlüssel'], ['b', 'Abo-PIN (nur bei Abo-Schlüssel)']] },
  { k: 'fanart', n: 'Fanart.tv', p: 'Logos, Hintergründe und Disc-Bilder; eingeschaltet wird die Quelle unter «Metadaten».', h: 'Mit persönlichem Schlüssel erscheinen neue Bilder nach zwei Tagen, sonst nach sieben.', f: [['a', 'Projekt-Schlüssel'], ['b', 'Persönlicher Schlüssel (freiwillig)']] },
  { k: 'youtube', n: 'YouTube', p: 'Trailer suchen, wenn TMDB keinen kennt.', h: 'Ein Schlüssel der «YouTube Data API v3» aus der Google Cloud Console.', f: [['a', 'Schlüssel']] },
  { k: 'trakt', n: 'Trakt', p: 'Listen für das Spotlight und die Reihen der Startseite.', h: 'Die Client-ID einer eigenen Trakt-App. Den Stand der Listen zeigt die Seite «Spotlight».', f: [['a', 'Client-ID']] },
  { k: 'googlebooks', n: 'Google Books', p: 'Buchdaten für Hörbücher; ohne Schlüssel fragt Slouchcat Google Books nie.', h: 'Ein Schlüssel der «Books API» aus der Google Cloud Console. Welcher Ordner die Quelle nutzt, steht unter «Hörbücher: Quellen».', f: [['a', 'Schlüssel']] },
];
const AA_SRC = { env: 'In deploy/.env gesetzt', stored: 'Gespeichert', none: 'Nicht gesetzt' };
const AA_HOOK_RES = { scan_queued: 'Ordner wird gelesen', acknowledged: 'Test quittiert', ignored: 'Ohne Wirkung', outside_roots: 'Ausserhalb der Bibliotheken', invalid_payload: 'Ohne Pfad' };
const AA_GPU = [{ v: 'auto', t: 'Automatisch (empfohlen)' }, { v: 'software', t: 'Nur Prozessor' }, { v: 'qsv', t: 'Intel Quick Sync (QSV)' }, { v: 'vaapi', t: 'VA-API' }, { v: 'nvenc', t: 'NVIDIA NVENC – Gerät fehlt im Container', disabled: true }];

function aaKeySet(k) { const s = aaInt().keys[k]; return s.src !== 'none'; }
function aaKeyDirty(k) { const d = (ui.draft.aaKeys || {})[k]; return !!(d && (d.a || d.b)); }
function aaEmailDraft() { const e = aaInt().email; if (!ui.draft.aaEmail) ui.draft.aaEmail = { host: e.host, port: e.port, user: e.user, pw: '', from: e.from, tls: e.tls }; return ui.draft.aaEmail; }
function aaEmailDirty() { const d = ui.draft.aaEmail, e = aaInt().email; return !!d && (d.host !== e.host || String(d.port) !== String(e.port) || d.user !== e.user || d.pw || d.from !== e.from || d.tls !== e.tls); }
function aaHookSilent() { return aaInt().webhooks.filter((h) => h.last && Date.now() - h.last > 7 * D); }
/* nine dashboard entries: tmdb, tvdb, youtube, trakt, email, telegram, push, update check, jellyseerr */
function aaIntCounts() {
  const I = aaInt();
  const list = [aaKeySet('tmdb'), aaKeySet('tvdb'), aaKeySet('youtube'), aaKeySet('trakt'), !!I.email.host, !!I.telegram.tail, I.push.has, I.server.update, seerrState().k === 'ok'];
  return { set: list.filter(Boolean).length, total: list.length };
}
function aaStateLine(name, ok, detail, kind) {
  const k = kind || (ok ? 'ok' : '');
  return `<li><span class="dot ${k}"></span><span>${esc(name)}: ${ok ? 'Eingerichtet' : kind === 'bad' ? 'Fehler' : kind === 'off' ? 'Ausgeschaltet' : 'Nicht eingerichtet'}${detail ? ` · <span class="muted">${detail}</span>` : ''}</span></li>`;
}

/* ---------- M-21 Integrationen ---------- */
function aaTile(o) {
  return `<article class="${cls('tile', o.isNew && 'new')}"><span class="ik">${icon(o.icon, 18)}</span><div class="tt"><h2><a href="#/admin/integrations/${o.k}" data-fid="aa:tile:${o.k}">${o.title}</a>${o.isNew ? ' ' + aaNew() : ''}</h2><p>${o.desc}</p>${o.meta ? `<div class="tm">${o.meta}</div>` : ''}<ul class="sts">${o.sts}</ul></div><span class="arr">${icon('chevR', 18)}</span></article>`;
}
function aaIntegrationsPage() {
  const I = aaInt(), c = aaIntCounts(), st = seerrState(), mc = aaMapCounts(), wc = wishCounts();
  const keysSet = AA_PROV.filter((p) => aaKeySet(p.k)).length;
  const keysDirty = AA_PROV.some((p) => aaKeyDirty(p.k));
  const ev24 = I.webhooks.reduce((n, h) => n + h.log.filter((l) => Date.now() - l.at < D).length, 0);
  const silent = aaHookSilent();
  const unsaved = '<span class="chip warn">ungespeichert</span>';
  const seerrDetail = st.k === 'ok' ? esc(S.seerr.version) : st.k === 'unreachable' ? `zuletzt erreicht ${ago(S.seerr.lastSync)}` : '';
  const hints = aaSeerrHints(), firstHint = hints ? aaSeerrHintList()[0] : '';
  const tiles = [
    aaTile({ k: 'keys', icon: 'key', title: 'Schlüssel der Anbieter', desc: 'TMDB, TVDB, Fanart.tv, YouTube, Trakt und Google Books.', meta: `<span>${keysSet} von ${AA_PROV.length} Anbietern eingerichtet</span>${keysDirty ? unsaved : ''}`,
      sts: ['tmdb', 'tvdb', 'youtube', 'trakt'].map((k) => { const s = I.keys[k]; return aaStateLine(AA_PROV.find((p) => p.k === k).n, s.src !== 'none' && !s.rejected, s.src === 'env' ? 'In deploy/.env gesetzt' : s.rejected ? 'abgewiesen' : '', s.rejected ? 'bad' : ''); }).join('') }),
    aaTile({ k: 'webhooks', icon: 'link', title: 'Webhooks von Radarr und Sonarr', desc: 'Radarr und Sonarr melden neue und geänderte Dateien.', meta: `<span>${I.webhooks.length} ${I.webhooks.length === 1 ? 'Integration' : 'Integrationen'}</span>`,
      sts: I.webhooks.length ? `<li><span class="dot ${ev24 ? 'ok' : ''}"></span><span>${ev24} ${ev24 === 1 ? 'Meldung' : 'Meldungen'} im Protokoll der letzten 24 Stunden, von ${I.webhooks.length} Integrationen.</span></li>${silent.length ? `<li><span class="dot warn"></span><span>Seit sieben Tagen ohne Meldung: ${silent.map((h) => esc(h.name)).join(', ')}</span></li>` : ''}` : '<li><span class="dot"></span><span>Kein Webhook eingerichtet.</span></li>' }),
    aaTile({ k: 'jellyseerr', isNew: true, icon: 'wish', title: 'Seerr', desc: 'Wünsche der Zuschauer an Seerr weitergeben und ihren Zustand abgleichen. Konten erhalten dort einen Benutzer mit demselben Login.',
      meta: S.seerr.configured ? `<span>Seerr-Benutzer: ${mc.mapped} von ${mc.total} Konten</span><span>·</span><span>${wc.open} offene Wünsche</span>${aaSeerrDirty() ? unsaved : ''}` : (aaSeerrDirty() ? unsaved : ''),
      sts: `<li><span class="dot ${st.dot && st.k === 'ok' && hints ? 'warn' : st.dot}"></span><span>Seerr: ${st.label}${firstHint ? ` · ${esc(firstHint)}` : ''}${seerrDetail ? ` · <span class="muted">${seerrDetail}</span>` : ''}</span></li>` }),
    aaTile({ k: 'email', icon: 'mail', title: 'E-Mail-Server', desc: 'Server für Benachrichtigungen, Einladungen und Ankündigungen per E-Mail.', meta: `<span>${I.email.host ? 'Gespeichert' : 'Nicht gesetzt'}</span>${aaEmailDirty() ? unsaved : ''}`, sts: aaStateLine('E-Mail', !!I.email.host, I.email.host ? esc(I.email.host + ':' + I.email.port) : '') }),
    aaTile({ k: 'telegram', icon: 'send', title: 'Telegram-Bot', desc: 'Bot für Benachrichtigungen per Telegram.', meta: `<span>${I.telegram.tail ? 'Gespeichert' : 'Nicht gesetzt'}</span>${(ui.draft.aaTg || {}).token ? unsaved : ''}`, sts: aaStateLine('Telegram', !!I.telegram.tail, I.telegram.bot ? '@' + esc(I.telegram.bot) : '') }),
    aaTile({ k: 'push', icon: 'bell', title: 'Web-Push', desc: 'Benachrichtigungen im Browser und in der installierten App.', meta: `<span>${I.push.has ? 'Schlüsselpaar vorhanden.' : 'Noch kein Schlüsselpaar.'}</span>`, sts: aaStateLine('Web-Push', I.push.has) }),
    aaTile({ k: 'server', icon: 'server', title: 'Server', desc: 'Update-Prüfung, öffentliche Adresse und Grafikchip.', meta: `<span>${I.server.publicUrl ? esc(I.server.publicUrl) : 'Keine öffentliche Adresse'}</span>`, sts: aaStateLine('Update-Prüfung', I.server.update, '', I.server.update ? '' : 'off') }),
  ];
  return { type: 'admin', area: 'automation', page: 'integrations', title: 'Integrationen', intro: 'Schlüssel der Anbieter, Versand von Benachrichtigungen und die Update-Prüfung. Geheime Werte speichert Slouchcat verschlüsselt und zeigt sie nie wieder an.',
    html: `<p class="aa-count"><b>${c.set} von ${c.total}</b> eingerichtet</p><section aria-label="Dienste" class="tiles">${tiles.join('')}</section>` };
}
route('/admin/integrations', aaIntegrationsPage);

/* ---------- detail views ---------- */
const AA_ITEMS = { keys: 'Schlüssel der Anbieter', webhooks: 'Webhooks von Radarr und Sonarr', jellyseerr: 'Seerr', email: 'E-Mail-Server', telegram: 'Telegram-Bot', push: 'Web-Push', server: 'Server' };
route('/admin/integrations/:item', (pm) => {
  aaInt();
  const it = pm.item;
  const base = { type: 'admin', area: 'automation', page: 'integrations', back: ['#/admin/integrations', 'Alle Integrationen'] };
  if (!AA_ITEMS[it]) return { ...base, title: 'Nicht gefunden', html: notice('warn', 'Diese Integration gibt es nicht.', '', '<a class="btn secondary" href="#/admin/integrations" data-fid="aa:nf">Alle Integrationen</a>') };
  const v = { keys: aaKeysView, webhooks: aaHooksView, jellyseerr: aaSeerrView, email: aaEmailView, telegram: aaTgView, push: aaPushView, server: aaServerView }[it]();
  return { ...base, title: AA_ITEMS[it], crumb: 'Integrationen › ' + AA_ITEMS[it], ...v };
});

/* keys */
function aaKeysView() {
  const I = aaInt();
  const set = AA_PROV.filter((p) => aaKeySet(p.k)).length;
  const cards = AA_PROV.map((p) => {
    const s = I.keys[p.k], env = s.src === 'env';
    const d = (ui.draft.aaKeys = ui.draft.aaKeys || {})[p.k] = (ui.draft.aaKeys[p.k] || { a: '', b: '' });
    const chk = (ui.aaCheck || {})[p.k];
    const meta = `${aaKeyDirty(p.k) ? '<span class="chip warn">ungespeichert</span>' : ''}${s.rejected ? '<span class="chip bad">abgewiesen</span>' : ''}<span class="chip ${s.src === 'none' ? 'outline' : 'info'}">${s.src === 'stored' && s.tail.a ? 'Gespeichert, endet auf ' + esc(s.tail.a) : AA_SRC[s.src]}</span>`;
    const fields = p.f.map(([f, label]) => {
      const tail = s.tail[f];
      const ph = env ? 'In deploy/.env gesetzt' : tail ? `Gesetzt, endet auf …${tail}; leer lassen behält ihn` : '';
      return `<div class="fld"><label for="aa-key-${p.k}-${f}">${label}</label><input id="aa-key-${p.k}-${f}" class="inp" type="password" autocomplete="new-password" data-bind="ui.draft.aaKeys.${p.k}.${f}" data-fid="aa:key:${p.k}:${f}" value="${esc(d[f] || '')}" placeholder="${esc(ph)}" ${env ? 'disabled' : ''}></div>`;
    }).join('');
    const body = `<p class="sec">${p.p}</p>
      ${env ? notice('', 'Der Wert kommt aus deploy/.env und lässt sich nur dort ändern.', '«Prüfen» fragt den Anbieter trotzdem.') : ''}
      <div class="aa-fields">${fields}</div>
      <p class="muted">${p.h} <button type="button" class="lnk" data-act="aaExtLink" data-name="${p.n}" data-fid="aa:key:${p.k}:link">Schlüssel bei ${p.n} holen</button></p>
      ${chk ? notice(chk.kind, chk.text, '') : ''}
      <div class="row-gap"><button type="button" class="btn primary" data-act="aaKeySave" data-id="${p.k}" data-fid="aa:key:${p.k}:save" ${env ? 'disabled' : ''}>Speichern</button><button type="button" class="btn secondary" data-act="aaKeyCheck" data-id="${p.k}" data-fid="aa:key:${p.k}:check">Prüfen</button>${s.src === 'stored' ? `<button type="button" class="btn dq" data-act="aaKeyClear" data-id="${p.k}" data-fid="aa:key:${p.k}:clear">Löschen …</button>` : ''}</div>
      <p class="muted aa-small">Leer lassen, um den gespeicherten Wert zu behalten. Ein neuer Wert ersetzt ihn.</p>`;
    return band('aa:key:' + p.k, p.n, meta, body, { icon: 'key' });
  }).join('');
  return { intro: 'Slouchcat holt damit Texte, Bilder und Trailer. Ohne Schlüssel bleibt der Anbieter aus.', html: `<p class="aa-count"><b>${set} von ${AA_PROV.length}</b> Anbietern eingerichtet</p><div class="stack">${cards}</div><p class="aa-mockhint">Mockup: Ein Schlüssel, der «falsch» enthält, wird beim Prüfen und Speichern abgewiesen.</p>` };
}
ACT.aaExtLink = (d) => toast(`Im Produkt öffnet sich hier die Seite von ${esc(d.name)}. Das Mockup ruft keine fremden Seiten auf.`, 'ok', 3500);
ACT.aaKeySave = (d) => {
  const s = aaInt().keys[d.id], dr = ui.draft.aaKeys[d.id], p = AA_PROV.find((x) => x.k === d.id);
  if (!dr.a && !dr.b) { toast(`Nichts geändert: ${p.n} behält den gespeicherten Wert.`, 'warn'); return; }
  if (dr.a) s.tail.a = dr.a.slice(-4);
  if (dr.b) s.tail.b = dr.b.slice(-4);
  s.src = 'stored';
  s.rejected = /falsch/i.test(dr.a + dr.b);
  ui.draft.aaKeys[d.id] = { a: '', b: '' };
  (ui.aaCheck = ui.aaCheck || {})[d.id] = s.rejected ? { kind: 'bad', text: `${p.n} gespeichert, aber der Anbieter weist den Schlüssel ab.` } : null;
  toast(s.rejected ? `${p.n} gespeichert, aber der Anbieter weist den Schlüssel ab.` : `${p.n} gespeichert.`, s.rejected ? 'warn' : 'ok');
  commit();
};
ACT.aaKeyCheck = (d) => {
  const s = aaInt().keys[d.id], dr = ui.draft.aaKeys[d.id], p = AA_PROV.find((x) => x.k === d.id);
  let r;
  if (s.src === 'none' && !dr.a) r = { kind: 'warn', text: `${p.n}: Es ist kein Schlüssel gesetzt.` };
  else if (/falsch/i.test(dr.a + dr.b) || (!dr.a && s.rejected)) r = { kind: 'bad', text: `${p.n}: Der Anbieter lehnt den Schlüssel ab (401 Unauthorized).` };
  else r = { kind: 'ok', text: `${p.n}: Der Anbieter hat geantwortet, der Schlüssel gilt.` };
  (ui.aaCheck = ui.aaCheck || {})[d.id] = r;
  render();
};
ACT.aaKeyClear = (d) => { const p = AA_PROV.find((x) => x.k === d.id); aaConfirm({ title: `${p.n} löschen?`, body: 'Der gespeicherte Wert wird entfernt. Was ihn braucht, funktioniert danach nicht mehr.', confirm: 'Löschen', danger: true, act: 'aaKeyClearGo', id: d.id }); };
ACT.aaKeyClearGo = (d) => { const s = aaInt().keys[d.id]; s.src = 'none'; s.tail = { a: '', b: '' }; s.rejected = false; if (ui.aaCheck) ui.aaCheck[d.id] = null; closeDialog(); toast(`${AA_PROV.find((x) => x.k === d.id).n} gelöscht.`, 'ok'); commit(); };

/* webhooks of Radarr and Sonarr */
function aaResolve(h, path) { const m = h.map.find((r) => r.from && path.startsWith(r.from)); return m ? m.to + path.slice(m.from.length) : path; }
function aaHooksView() {
  const I = aaInt();
  const f = (ui.draft.aaHook = ui.draft.aaHook || { name: '', kind: 'radarr' });
  const cr = ui.aaHookCreated;
  const created = cr ? `<section class="card aa-once" aria-label="Adresse für ${esc(cr.name)}"><div class="ch"><span class="ik">${icon('link', 18)}</span><h2>Adresse für «${esc(cr.name)}»</h2></div>${notice('warn', 'Jetzt kopieren — danach ist die Adresse nicht mehr sichtbar.', '')}<div class="fld"><label for="aa-hook-url">Vollständige Adresse</label><input id="aa-hook-url" class="inp mono" readonly value="${esc(cr.url)}" data-fid="aa:hook:url"></div><div class="row-gap"><button type="button" class="btn primary" data-act="aaHookCopy" data-fid="aa:hook:copy">${icon('copy', 16)}Adresse kopieren</button><button type="button" class="btn secondary" data-act="aaHookHide" data-fid="aa:hook:hide">Ausblenden</button></div></section>` : '';
  const form = `<section class="card"><div class="ch"><span class="ik">${icon('plus', 18)}</span><h2>Neue Integration</h2></div>
    <div class="aa-hookform"><div class="fld"><label for="aa-hook-name">Name</label><input id="aa-hook-name" class="inp" data-bind="ui.draft.aaHook.name" data-fid="aa:hook:name" value="${esc(f.name)}" ${aaInv('hookName')} data-enter="aaHookCreate"><span class="hint">Frei wählbar, nur zur Unterscheidung in dieser Liste.</span>${aaErrHtml('hookName')}</div>
    <fieldset class="fld aa-fs"><legend class="lbl">Dienst</legend><div class="row-gap">${['radarr', 'sonarr'].map((k) => `<label class="check"><input type="radio" name="aa-hook-kind" value="${k}" data-bind="ui.draft.aaHook.kind" data-fid="aa:hook:kind:${k}" ${f.kind === k ? 'checked' : ''}>${k === 'radarr' ? 'Radarr' : 'Sonarr'}</label>`).join('')}</div></fieldset></div>
    <div class="row-gap"><button type="button" class="btn primary" data-act="aaHookCreate" data-fid="aa:hook:create">Integration anlegen</button></div></section>`;
  const rows = I.webhooks.map((h) => {
    const open = isOpen('aa:hook:' + h.id, cr && cr.id === h.id);
    const map = (ui.draft.aaMap = ui.draft.aaMap || {})[h.id] = ui.draft.aaMap[h.id] || h.map.map((r) => ({ ...r }));
    const mapDirty = JSON.stringify(map) !== JSON.stringify(h.map);
    const sub = !open ? '' : `<tr class="sub"><td colspan="5"><div class="stack">
      <h3 class="aa-h3">Pfad-Umsetzung</h3><p class="sec">Setzt den Pfad um, den ${h.kind === 'radarr' ? 'Radarr' : 'Sonarr'} meldet. Nötig, sobald derselbe Speicher auf beiden Seiten anders hängt — etwa «/mnt/medien/Filme» im Dienst und «/media/filme» hier.</p>
      <div class="aa-map">${map.map((r, i) => `<div class="aa-maprow"><div class="fld"><label for="aa-map-${h.id}-${i}-f">Pfad im Dienst</label><input id="aa-map-${h.id}-${i}-f" class="inp mono" data-bind="ui.draft.aaMap.${h.id}.${i}.from" data-fid="aa:map:${h.id}:${i}:f" value="${esc(r.from)}"></div><div class="fld"><label for="aa-map-${h.id}-${i}-t">Pfad auf diesem Server</label><input id="aa-map-${h.id}-${i}-t" class="inp mono" data-bind="ui.draft.aaMap.${h.id}.${i}.to" data-fid="aa:map:${h.id}:${i}:t" value="${esc(r.to)}"></div><button type="button" class="btn dq" data-act="aaMapDel" data-id="${h.id}" data-i="${i}" aria-label="Zeile für ${esc(r.from || 'leer')} entfernen" data-fid="aa:map:${h.id}:${i}:del">Entfernen</button></div>`).join('') || '<p class="muted">Keine Umsetzung: Slouchcat nimmt den gemeldeten Pfad, wie er ist.</p>'}</div>
      ${aaErrHtml('map' + h.id)}
      <div class="row-gap"><button type="button" class="btn secondary" data-act="aaMapAdd" data-id="${h.id}" data-fid="aa:map:${h.id}:add">${icon('plus', 16)}Zeile hinzufügen</button><button type="button" class="btn primary" data-act="aaMapSave" data-id="${h.id}" data-fid="aa:map:${h.id}:save" aria-label="Pfad-Umsetzung von ${esc(h.name)} speichern">Speichern</button>${mapDirty ? '<span class="chip warn">ungespeichert</span>' : ''}</div>
      <h3 class="aa-h3">Meldungen von ${esc(h.name)}</h3>
      ${h.log.length ? `<div class="tblw"><table class="tbl"><thead><tr><th>Zeit</th><th>Ereignis</th><th>Gemeldeter Pfad</th><th>Umgesetzter Pfad</th><th>Ergebnis</th></tr></thead><tbody>${h.log.map((l) => `<tr><td class="nowrap">${aaWhen(l.at)}</td><td>${esc(l.ev)}</td><td class="mono">${esc(l.rep || '–')}</td><td class="mono">${esc(l.rep ? aaResolve(h, l.rep) : '–')}</td><td><span class="chip ${l.res === 'scan_queued' || l.res === 'acknowledged' ? 'ok' : 'warn'}">${AA_HOOK_RES[l.res]}</span></td></tr>`).join('')}</tbody></table></div>` : '<p class="muted">Noch keine Meldungen.</p>'}
      <div class="row-gap"><button type="button" class="btn secondary sm" data-act="aaHookTest" data-id="${h.id}" data-fid="aa:hook:${h.id}:test">Testmeldung simulieren</button><span class="aa-mockhint">Mockup: spielt eine Meldung von ${h.kind === 'radarr' ? 'Radarr' : 'Sonarr'} ein.</span></div>
    </div></td></tr>`;
    return `<tr><td class="main"><button type="button" class="tog" data-act="open" data-id="aa:hook:${h.id}" aria-expanded="${open}" data-fid="aa:hook:${h.id}:tog"><span class="chev">${icon('chevR', 16)}</span>${esc(h.name)}</button>${mapDirty ? ' <span class="chip warn">ungespeichert</span>' : ''}</td><td><span class="chip info">${h.kind === 'radarr' ? 'Radarr' : 'Sonarr'}</span></td><td class="mono">…${esc(h.tail)}</td><td>${h.last ? aaWhen(h.last) + (Date.now() - h.last > 7 * D ? ' <span class="chip warn">still</span>' : '') : 'Hat noch nie etwas gemeldet.'}</td><td class="end"><span class="ta"><button type="button" class="btn dq sm" data-act="aaHookRenew" data-id="${h.id}" data-fid="aa:hook:${h.id}:renew" aria-label="Token von ${esc(h.name)} erneuern">Token erneuern</button><button type="button" class="btn dq sm" data-act="aaHookDel" data-id="${h.id}" data-fid="aa:hook:${h.id}:del" aria-label="${esc(h.name)} löschen">Löschen</button></span></td></tr>${sub}`;
  }).join('');
  const table = I.webhooks.length ? `<div class="tblw"><table class="tbl"><caption class="vh">Webhook-Integrationen</caption><thead><tr><th>Name</th><th>Dienst</th><th>Adresse</th><th>Letzte Meldung</th><th class="end">Aktionen</th></tr></thead><tbody>${rows}</tbody></table></div>` : '<div class="unchanged"><b>Noch keine Integration eingerichtet.</b></div>';
  return { intro: 'Radarr und Sonarr melden geänderte Dateien hierher, damit neue Titel sofort erscheinen. Slouchcat fragt die Dienste nie von sich aus ab.', html: `${created}${form}<p class="aa-count"><b>${I.webhooks.length}</b> ${I.webhooks.length === 1 ? 'Integration' : 'Integrationen'}</p>${table}` };
}
function aaHookUrl(tok) { return `${AA_HOST}/api/v1/webhooks/${tok}`; }
ACT.aaHookCreate = () => {
  const f = ui.draft.aaHook;
  if (!f.name.trim()) { aaSetErr('hookName', 'Gib der Integration einen Namen.'); render(); return; }
  aaSetErr('hookName');
  const tok = aaTok();
  const h = { id: 'h' + ++S.seq, name: f.name.trim(), kind: f.kind, tail: tok.slice(-4), last: null, map: [], log: [] };
  aaInt().webhooks.push(h);
  ui.aaHookCreated = { id: h.id, name: h.name, url: aaHookUrl(tok) };
  ui.draft.aaHook = { name: '', kind: 'radarr' };
  toast(`Integration «${esc(h.name)}» angelegt. Kopiere jetzt die Adresse.`, 'ok');
  commit();
};
ACT.aaHookCopy = () => aaCopy(ui.aaHookCreated ? ui.aaHookCreated.url : '', 'Adresse kopiert.');
ACT.aaHookHide = () => { ui.aaHookCreated = null; render(); };
ACT.aaHookRenew = (d) => { const h = aaInt().webhooks.find((x) => x.id === d.id); aaConfirm({ title: `Token von «${esc(h.name)}» erneuern?`, body: 'Die bisherige Adresse hört sofort auf zu arbeiten. Die neue wird einmal angezeigt und muss in Radarr oder Sonarr neu eingetragen werden.', confirm: 'Erneuern', danger: true, act: 'aaHookRenewGo', id: d.id }); };
ACT.aaHookRenewGo = (d) => { const h = aaInt().webhooks.find((x) => x.id === d.id); const tok = aaTok(); h.tail = tok.slice(-4); ui.aaHookCreated = { id: h.id, name: h.name, url: aaHookUrl(tok) }; closeDialog(); toast(`Neues Token für «${esc(h.name)}». Die alte Adresse gilt nicht mehr.`, 'ok'); commit(); };
ACT.aaHookDel = (d) => { const h = aaInt().webhooks.find((x) => x.id === d.id); aaConfirm({ title: `«${esc(h.name)}» löschen?`, body: 'Die Adresse hört sofort auf zu arbeiten, und das Meldungsprotokoll verschwindet mit.', confirm: 'Löschen', danger: true, act: 'aaHookDelGo', id: d.id }); };
ACT.aaHookDelGo = (d) => { const I = aaInt(); const h = I.webhooks.find((x) => x.id === d.id); I.webhooks = I.webhooks.filter((x) => x.id !== d.id); if (ui.aaHookCreated && ui.aaHookCreated.id === d.id) ui.aaHookCreated = null; closeDialog(); toast(`«${esc(h.name)}» gelöscht.`, 'ok'); commit(); };
ACT.aaMapAdd = (d) => { ui.draft.aaMap[d.id].push({ from: '', to: '' }); render(); };
ACT.aaMapDel = (d) => { ui.draft.aaMap[d.id].splice(Number(d.i), 1); render(); };
ACT.aaMapSave = (d) => {
  const rows = ui.draft.aaMap[d.id];
  if (rows.some((r) => !r.from.trim() || !r.to.trim())) { aaSetErr('map' + d.id, 'Jede Zeile braucht beide Pfade.'); render(); return; }
  aaSetErr('map' + d.id);
  aaInt().webhooks.find((x) => x.id === d.id).map = rows.map((r) => ({ from: r.from.trim(), to: r.to.trim() }));
  ui.draft.aaMap[d.id] = null;
  toast('Pfad-Umsetzung gespeichert.', 'ok');
  commit();
};
ACT.aaHookTest = (d) => { const h = aaInt().webhooks.find((x) => x.id === d.id); h.last = Date.now(); h.log.unshift({ at: Date.now(), ev: 'Test', rep: '', res: 'acknowledged' }); toast(`${h.kind === 'radarr' ? 'Radarr' : 'Sonarr'} hat eine Testmeldung geschickt: «Test quittiert».`, 'ok'); commit(); };

/* e-mail server */
function aaEmailView() {
  const I = aaInt(), d = aaEmailDraft(), e = I.email;
  const tls = [['starttls', 'STARTTLS'], ['tls', 'TLS'], ['none', 'Keine']];
  const busy = ui.busy.aaMail;
  const body = `${aaSrow('Server und Port', 'Adresse des E-Mail-Servers und sein Port.', `<div class="aa-hostport"><input class="inp" data-bind="ui.draft.aaEmail.host" data-fid="aa:mail:host" value="${esc(d.host)}" aria-label="Server" ${aaInv('mailHost')}><input class="inp num" data-bind="ui.draft.aaEmail.port" data-num data-fid="aa:mail:port" value="${esc(d.port)}" aria-label="Port" inputmode="numeric" ${aaInv('mailPort')}></div>${aaErrHtml('mailHost')}${aaErrHtml('mailPort')}`, { col: true })}
    ${aaSrow('Benutzer', 'Anmeldename am E-Mail-Server, falls nötig.', aaInp('ui.draft.aaEmail.user', 'aa:mail:user', { label: 'Benutzer' }))}
    ${aaSrow('Passwort', `Wird verschlüsselt gespeichert und nie wieder angezeigt. Leer lassen, um das gespeicherte Passwort zu behalten.`, aaInp('ui.draft.aaEmail.pw', 'aa:mail:pw', { type: 'password', label: 'Passwort', ph: e.pwTail ? `Gesetzt, endet auf …${e.pwTail}` : '' }))}
    ${aaSrow('Absender', 'Zum Beispiel Slouchcat &lt;media@example.ch&gt;.', aaInp('ui.draft.aaEmail.from', 'aa:mail:from', { label: 'Absender' }))}
    ${aaSrow('Verschlüsselung', 'STARTTLS für Port 587, TLS für Port 465.', `<div class="seg" role="group" aria-label="Verschlüsselung">${tls.map(([v, t]) => `<button type="button" data-act="set" data-path="ui.draft.aaEmail.tls" data-value="${v}" aria-pressed="${d.tls === v}" data-fid="aa:mail:tls:${v}">${t}</button>`).join('')}</div>`)}`;
  const acts = `<div class="row-gap"><button type="button" class="btn primary" data-act="aaMailSave" data-fid="aa:mail:save">Speichern</button><button type="button" class="btn secondary" data-act="aaMailTest" data-fid="aa:mail:test" ${busy ? 'disabled' : ''}>${busy ? '<span class="spin"></span>' : icon('send', 16)}Testmail senden</button><button type="button" class="btn quiet" data-act="aaMailDiscard" data-fid="aa:mail:discard" ${aaEmailDirty() ? '' : 'disabled'}>Verwerfen</button>${e.host ? '<button type="button" class="btn dq" data-act="aaMailClear" data-fid="aa:mail:clear">Löschen …</button>' : ''}${aaEmailDirty() ? '<span class="chip warn">ungespeichert</span>' : ''}</div>
    <p class="muted aa-small">Die Testmail geht an ${esc(curAccount().email)}. <a class="lnk" href="#/admin/notifications" data-fid="aa:mail:chan">Zu den Benachrichtigungskanälen</a></p>
    <p class="aa-mockhint">Mockup: Ein Server, der «falsch» enthält, lässt die Testmail scheitern.</p>`;
  return { intro: 'Für Benachrichtigungen, Einladungen und Ankündigungen per E-Mail.', html: `<section class="card"><div class="ch"><span class="ik">${icon('mail', 18)}</span><h2>E-Mail-Server</h2><span class="chip ${e.host ? 'ok' : 'outline'}">${e.host ? 'Eingerichtet' : 'Nicht eingerichtet'}</span></div><div>${body}</div>${acts}</section>` };
}
ACT.aaMailSave = () => {
  const d = ui.draft.aaEmail, e = aaInt().email;
  const port = Number(d.port);
  aaSetErr('mailHost', d.host.trim() ? '' : 'Gib den Server an.');
  aaSetErr('mailPort', Number.isInteger(port) && port >= 1 && port <= 65535 ? '' : 'Der Port ist eine Zahl von 1 bis 65535.');
  if (aaErr('mailHost') || aaErr('mailPort')) { render(); return; }
  Object.assign(e, { host: d.host.trim(), port, user: d.user, from: d.from, tls: d.tls });
  if (d.pw) e.pwTail = d.pw.slice(-4);
  ui.draft.aaEmail = null;
  toast('E-Mail-Server gespeichert.', 'ok');
  commit();
};
ACT.aaMailDiscard = () => { ui.draft.aaEmail = null; aaSetErr('mailHost'); aaSetErr('mailPort'); toast('Änderungen verworfen.', 'ok', 2500); render(); };
ACT.aaMailTest = () => {
  const d = ui.draft.aaEmail;
  ui.busy.aaMail = true; render();
  setTimeout(() => {
    ui.busy.aaMail = false;
    const to = esc(curAccount().email);
    if (!d.host || /falsch/i.test(d.host)) toast(`Die Testmail kam nicht an: ${d.host ? 'dial tcp: lookup ' + esc(d.host) + ': no such host' : 'kein Server eingetragen'}`, 'bad', 6500);
    else { toast(`Testmail an ${to} gesendet.`, 'mail'); S.deliveries.unshift({ at: Date.now(), accountId: curAccount().id, profileId: null, channel: 'email', event: 'test', subject: 'Testmail von Slouchcat', state: 'sent' }); save(); }
    render();
  }, 1000);
};
ACT.aaMailClear = () => aaConfirm({ title: 'E-Mail-Server löschen?', body: 'Der gespeicherte Wert wird entfernt. Was ihn braucht, funktioniert danach nicht mehr.', confirm: 'Löschen', danger: true, act: 'aaMailClearGo' });
ACT.aaMailClearGo = () => { Object.assign(aaInt().email, { host: '', port: 587, user: '', pwTail: '', from: '', tls: 'starttls' }); ui.draft.aaEmail = null; closeDialog(); toast('E-Mail-Server gelöscht.', 'ok'); commit(); };

/* telegram */
function aaTgView() {
  const t = aaInt().telegram;
  const d = (ui.draft.aaTg = ui.draft.aaTg || { token: '' });
  return { intro: 'Bot für Benachrichtigungen per Telegram.', html: `<section class="card"><div class="ch"><span class="ik">${icon('send', 18)}</span><h2>Telegram-Bot</h2><span class="chip ${t.tail ? 'ok' : 'outline'}">${t.tail ? 'Eingerichtet' : 'Nicht eingerichtet'}</span></div>
    ${aaSrow('Bot', 'Slouchcat findet ihn über das Token.', `<span>${t.bot ? 'Bot: @' + esc(t.bot) : 'Kein Bot eingerichtet.'}</span>`)}
    ${aaSrow('Bot-Token', `Für Benachrichtigungen per Telegram. Das Token kommt von @BotFather, Slouchcat prüft es bei Telegram.${t.tail ? ' Ein neues Token ersetzt das gespeicherte.' : ''}`, `<input class="inp" type="password" autocomplete="new-password" data-bind="ui.draft.aaTg.token" data-fid="aa:tg:token" value="${esc(d.token)}" placeholder="${t.tail ? 'Token gesetzt' : ''}" aria-label="Bot-Token" ${aaInv('tg')}>${aaErrHtml('tg')}`, { col: true })}
    <div class="row-gap"><button type="button" class="btn primary" data-act="aaTgSave" data-fid="aa:tg:save">Speichern</button>${t.tail ? '<button type="button" class="btn dq" data-act="aaTgClear" data-fid="aa:tg:clear">Löschen …</button>' : ''}${d.token ? '<span class="chip warn">ungespeichert</span>' : ''}</div>
    <p class="aa-mockhint">Mockup: Ein Token, das «falsch» enthält, kennt Telegram nicht.</p></section>` };
}
ACT.aaTgSave = () => {
  const d = ui.draft.aaTg, t = aaInt().telegram;
  if (!d.token) { aaSetErr('tg', 'Gib das Token von @BotFather ein.'); render(); return; }
  if (/falsch/i.test(d.token)) { aaSetErr('tg', 'Telegram kennt dieses Token nicht (401 Unauthorized).'); render(); return; }
  aaSetErr('tg'); t.tail = d.token.slice(-4); t.bot = t.bot || 'slouchcat_bot'; d.token = '';
  toast('Telegram-Bot gespeichert. Bot: @' + esc(t.bot), 'ok'); commit();
};
ACT.aaTgClear = () => aaConfirm({ title: 'Telegram-Bot löschen?', body: 'Der gespeicherte Wert wird entfernt. Was ihn braucht, funktioniert danach nicht mehr.', confirm: 'Löschen', danger: true, act: 'aaTgClearGo' });
ACT.aaTgClearGo = () => { Object.assign(aaInt().telegram, { bot: '', tail: '' }); closeDialog(); toast('Telegram-Bot gelöscht.', 'ok'); commit(); };

/* web push */
function aaPushView() {
  const p = aaInt().push;
  return { intro: 'Benachrichtigungen im Browser und in der installierten App.', html: `<section class="card"><div class="ch"><span class="ik">${icon('bell', 18)}</span><h2>Web-Push</h2><span class="chip ${p.has ? 'ok' : 'outline'}">${p.has ? 'Eingerichtet' : 'Nicht eingerichtet'}</span></div>
    <p>${p.has ? 'Schlüsselpaar vorhanden.' : 'Noch kein Schlüsselpaar. Es entsteht, sobald ein Gerät Web-Push aktiviert.'}</p>
    ${p.has ? `<div class="fld"><span class="lbl">Öffentlicher Schlüssel</span><code class="mono aa-pre">${esc(p.pub)}</code></div><div class="row-gap"><button type="button" class="btn dq" data-act="aaPushRotate" data-fid="aa:push:rotate">${icon('refresh', 16)}Schlüssel neu erzeugen</button></div>` : '<div class="row-gap"><button type="button" class="btn secondary" data-act="aaPushCreate" data-fid="aa:push:create">Gerät aktiviert Web-Push (Mockup)</button></div>'}</section>` };
}
ACT.aaPushRotate = () => aaConfirm({ title: 'Neue Web-Push-Schlüssel erzeugen?', body: 'Alle Geräte verlieren ihr Abo und müssen Web-Push in den Einstellungen neu aktivieren.', confirm: 'Neu erzeugen', danger: true, act: 'aaPushRotateGo' });
ACT.aaPushRotateGo = () => { aaInt().push.pub = 'B' + aaTok(86); closeDialog(); toast('Neue Schlüssel erzeugt. Die Geräte müssen Web-Push neu aktivieren.', 'ok'); commit(); };
ACT.aaPushCreate = () => { Object.assign(aaInt().push, { has: true, pub: 'B' + aaTok(86) }); toast('Schlüsselpaar erzeugt.', 'ok'); commit(); };

/* server */
function aaServerView() {
  const s = aaInt().server;
  if (ui.draft.aaUrl == null) ui.draft.aaUrl = s.publicUrl;
  return { intro: 'Update-Prüfung, öffentliche Adresse und Grafikchip.', html: `<section class="card"><div class="ch"><span class="ik">${icon('refresh', 18)}</span><h2>Update-Prüfung</h2></div>
      <p class="sec">Slouchcat liest einmal täglich das öffentliche Release-Manifest und zeigt einen Hinweis, wenn es eine neue Version gibt. Die Prüfung selbst installiert nichts.</p>
      ${aaSrow('Täglich nach neuen Versionen suchen', 'Wird sofort gespeichert. Dieselbe Einstellung wie Ein/Aus des Jobs «Update-Prüfung».', sw('S.integrations.server.update', 'Täglich nach neuen Versionen suchen', { after: 'aaUpdToggled' }))}</section>
    <section class="card"><div class="ch"><span class="ik">${icon('link', 18)}</span><h2>Öffentliche Adresse</h2></div>
      <p class="sec">Unter dieser Adresse führen die Links in E-Mails und Nachrichten zum Server.</p>
      ${aaSrow('Adresse', 'Zum Beispiel https://media.example.ch', `${aaInp('ui.draft.aaUrl', 'aa:srv:url', { label: 'Adresse', err: 'srvUrl' })}${aaErrHtml('srvUrl')}`, { col: true })}
      <div class="row-gap"><button type="button" class="btn primary" data-act="aaUrlSave" data-fid="aa:srv:save">Speichern</button>${s.publicUrl ? '<button type="button" class="btn dq" data-act="aaUrlClear" data-fid="aa:srv:clear">Löschen …</button>' : ''}</div></section>
    <section class="card"><div class="ch"><span class="ik">${icon('monitor', 18)}</span><h2>Grafikchip für das Umwandeln</h2></div>
      ${aaSrow('Grafikchip für das Umwandeln', 'Gilt nach einem Neustart des Servers (System › Server › Neu starten).', selectEl('S.integrations.server.gpu', AA_GPU, { label: 'Grafikchip für das Umwandeln', after: 'aaGpuSaved' }))}
      <p class="muted">Grafikchip erkannt: Slouchcat wandelt Videos auf dem Grafikchip um.</p></section>` };
}
ACT.aaUpdToggled = () => toast(aaInt().server.update ? 'Update-Prüfung eingeschaltet.' : 'Update-Prüfung ausgeschaltet.', 'ok', 2600);
ACT.aaGpuSaved = () => toast(`Gespeichert: ${esc(AA_GPU.find((g) => g.v === aaInt().server.gpu).t)}. Gilt nach einem Neustart.`, 'ok');
ACT.aaUrlSave = () => {
  const v = (ui.draft.aaUrl || '').trim();
  if (!/^https?:\/\/[^\s/]+/.test(v)) { aaSetErr('srvUrl', 'Gib eine Adresse mit http:// oder https:// ein.'); render(); return; }
  aaSetErr('srvUrl'); aaInt().server.publicUrl = v; ui.draft.aaUrl = null; toast('Öffentliche Adresse gespeichert.', 'ok'); commit();
};
ACT.aaUrlClear = () => aaConfirm({ title: 'Öffentliche Adresse löschen?', body: 'Der gespeicherte Wert wird entfernt. Ohne öffentliche Adresse fehlen Links in E-Mails.', confirm: 'Löschen', danger: true, act: 'aaUrlClearGo' });
ACT.aaUrlClearGo = () => { aaInt().server.publicUrl = ''; ui.draft.aaUrl = null; closeDialog(); toast('Öffentliche Adresse gelöscht.', 'ok'); commit(); };

/* ---------- M-22 Integrationen › Seerr ---------- */
const AA_SJ_TPL = '{\n  "notification_type": "{{notification_type}}",\n  "subject": "{{subject}}",\n  "media": {\n    "media_type": "{{media_type}}",\n    "tmdbId": "{{media_tmdbid}}",\n    "tvdbId": "{{media_tvdbid}}",\n    "status": "{{media_status}}"\n  },\n  "request": { "request_id": "{{request_id}}" }\n}';
function aaMapCounts() { return mappingCounts(); }
function aaSeerrDraft() { if (!ui.draft.seerr) ui.draft.seerr = { url: S.seerr.configured ? S.seerr.url : '', key: '' }; return ui.draft.seerr; }
function aaSeerrDirty() { const d = ui.draft.seerr; return !!d && ((S.seerr.configured ? d.url !== S.seerr.url : !!d.url) || !!d.key); }
function aaSyncEvery() { return ((S.settings.jobs || {})['wishes.sync'] || {}).every || '5 min'; }
function aaLastBad(op) { return S.seerr.log.find((l) => l.op === op && l.res === 'bad'); }
/* the simulated test: mockup condition, overridden by «falsch» in address or key */
function aaSeerrRun() {
  const d = aaSeerrDraft();
  let c = S.seerr.condition || 'ok';
  if (/falsch/i.test(d.url)) c = 'unreachable';
  else if (/falsch/i.test(d.key)) c = 'badkey';
  else if (!S.seerr.configured && !d.key) c = 'nokey';
  return aaSeerrRows(c);
}
/* address of Radarr or Sonarr that Slouchcat uses for the profile tag: its own, else the one Seerr has */
function aaArrAddr(a) { return (a.own || '').trim() || a.url; }
/* «vor 40 s» below a minute, else the usual «vor 3 min» */
function aaAgoS(ts) { const s = Math.round((Date.now() - ts) / 1000); return s < 60 ? `vor ${Math.max(1, s)} s` : ago(ts); }
/* tested versions: Jellyseerr 2.7 and Seerr 3.x; an older one is a hint, not an error */
function aaVersionOld(v) {
  const m = /^(Jellyseerr|Seerr)\s+(\d+)\.(\d+)/.exec(v || '');
  if (!m) return false;
  const maj = Number(m[2]), min = Number(m[3]);
  return m[1] === 'Jellyseerr' ? maj < 2 || (maj === 2 && min < 7) : maj < 3;
}
/* the check list for one condition of Seerr; a warn is a hint and never fails the test */
function aaSeerrRows(c) {
  const ver = S.seerr.version || 'Seerr 3.5.2';
  const R = [
    ['Erreichbar', 'Antwort in 84 ms.'],
    ['Schlüssel gültig', 'Seerr nimmt den Schlüssel an.'],
    ['Version', ver],
    ['Handeln im Namen von Benutzern (X-API-User)', 'Wünsche gelten mit Rechten, Limit und Auto-Freigabe des zugeordneten Benutzers.'],
    ['CSRF-Schutz in Seerr aus', 'Schreibende Aufrufe kommen an.'],
    ['Radarr eingerichtet (1 Server)', 'Film-Wünsche gehen an Radarr.'],
    ['Sonarr eingerichtet (1 Server)', 'Serien-Wünsche gehen an Sonarr.'],
  ].map(([l, t]) => ({ k: 'ok', l, t }));
  if (aaVersionOld(ver)) R[2] = { k: 'warn', l: 'Version', t: 'Version nicht geprüft: getestet sind Jellyseerr 2.7 und Seerr 3.x', short: 'Version nicht geprüft' };
  R.push({ k: 'ok', isNew: true, l: 'Benutzer anlegen und ändern', t: 'Der Schlüssel des Administrators darf Konten in Seerr eröffnen und Rechte, Limit und Passwort setzen.' });
  R.push(S.seerr.localLogin
    ? { k: 'ok', isNew: true, l: 'Lokale Anmeldung in Seerr', t: 'Eröffnete Benutzer melden sich in Seerr mit E-Mail und Passwort aus Slouchcat an.' }
    : { k: 'warn', isNew: true, l: 'Lokale Anmeldung in Seerr', t: 'Eröffnete Benutzer können sich in Seerr nicht anmelden. In Seerr unter Einstellungen › Benutzer «Lokale Anmeldung» einschalten.', short: 'Lokale Anmeldung in Seerr aus' });
  R.push(S.seerr.seasonsFrom === 'tvdb'
    ? { k: 'ok', isNew: true, l: 'Staffeln aus TVDB', t: 'Seerr, Sonarr und die Slouchcat-Bibliothek zählen Staffeln gleich.' }
    : { k: 'warn', isNew: true, l: 'Staffeln aus TVDB', t: 'Seerr liest Staffeln aus TMDB. Empfohlen ist TVDB, wie in Sonarr und in der Slouchcat-Bibliothek, sonst können Staffelnummern abweichen.', short: 'Staffeln aus TMDB' });
  if (aaArrOn()) ['radarr', 'sonarr'].forEach((k) => {
    const a = S.arr[k], addr = aaArrAddr(a);
    R.push(a.condition === 'ok'
      ? { k: 'ok', isNew: true, l: `${a.name} für Profil-Tags und Fortschritt`, t: `${a.name} erreichbar unter ${addr} · Warteschlange lesbar · Tag Requests: ${a.tagRequests ? 'an' : 'aus'}` }
      : { k: 'warn', isNew: true, l: `${a.name} für Profil-Tags und Fortschritt`, t: `${a.name} nicht erreichbar: Profil-Tags werden nachgeholt, sobald es geht; der Download-Fortschritt fehlt so lange. Eigene Adresse eintragen?`, short: `${a.name} für Profil-Tags und Fortschritt nicht erreichbar` });
  });
  const rest = (from) => { for (let i = from; i < R.length; i++) R[i] = { ...R[i], k: 'wait', t: 'nicht geprüft' }; };
  if (c === 'unreachable') { R[0] = { ...R[0], k: 'bad', t: 'Keine Antwort nach 10 Sekunden. Läuft Seerr, stimmt die Adresse?' }; rest(1); }
  else if (c === 'badkey') { R[1] = { ...R[1], k: 'bad', t: 'Seerr antwortet mit 403.' }; rest(2); }
  else if (c === 'nokey') { R[1] = { ...R[1], k: 'bad', t: 'Kein Schlüssel eingetragen.' }; rest(2); }
  else if (c === 'csrf') {
    R[4] = { ...R[4], k: 'bad', t: 'Schreibende Aufrufe scheitern, solange der Schutz an ist. In Seerr unter Einstellungen › Netzwerk ausschalten.' };
    R[7] = { ...R[7], k: 'bad', t: 'Scheitert wie alle schreibenden Aufrufe, solange der CSRF-Schutz an ist. Konten werden danach beim nächsten Abgleich eröffnet.' };
  }
  const bad = R.filter((r) => r.k === 'bad').length, warn = R.filter((r) => r.k === 'warn').length;
  return { at: Date.now(), rows: R, bad, warn };
}
/* accounts with the wish right but without a Seerr user (not opened, waiting, or only a proposed link);
   none when the collective user covers them (F-4) */
function aaNoUserAccounts() { return S.settings.wishes.unmapped === 'fallback' && S.settings.wishes.fallbackUser ? [] : S.accounts.filter((a) => a.canRequest && !accSeerr(a).user); }
/* hints of the saved connection, as the tile on «Integrationen» shows them (the first one stands in the tile) */
function aaSeerrHintList() {
  if (seerrState().k !== 'ok') return [];
  const out = aaSeerrRows('ok').rows.filter((r) => r.k === 'warn').map((r) => r.short || r.l);
  const n = aaNoUserAccounts().length;
  if (n) out.push(`${n} ${n === 1 ? 'Konto' : 'Konten'} ohne Seerr-Benutzer`);
  return out;
}
function aaSeerrHints() { return aaSeerrHintList().length; }
const aaHintWord = (n) => `${n} ${n === 1 ? 'Hinweis' : 'Hinweise'}`;
function aaChecksHtml(res) {
  const ic = { ok: 'check', bad: 'x', warn: 'alert', wait: 'clock' };
  return `<ul class="checks" aria-label="Ergebnis des Verbindungstests">${res.rows.map((r) => `<li class="${r.k}">${icon(ic[r.k], 18)}<div><b>${esc(r.l)}${r.isNew ? ' ' + aaNew() : ''}</b><small>${esc(r.t)}</small></div></li>`).join('')}</ul>`;
}
/* Radarr and Sonarr are called for profile tags and for the download progress of wishes (read only) */
function aaArrOn() { return S.settings.wishes.tags.enabled || progressOn(); }
/* «Radarr erreichbar · Sonarr nicht erreichbar» */
function aaArrSummary() {
  if (!aaArrOn()) return 'aus · Profil-Tags und Fortschritt ausgeschaltet';
  return ['radarr', 'sonarr'].map((k) => `${S.arr[k].name} ${S.arr[k].condition === 'ok' ? 'erreichbar' : 'nicht erreichbar'}`).join(' · ');
}
function aaArrDown() { return aaArrOn() ? ['radarr', 'sonarr'].map((k) => S.arr[k]).filter((a) => a.condition !== 'ok') : []; }
/* tags that failed, or that have waited for more than 24 hours since the title was handed over */
/* a tag that could not be set; a tag that is still waiting for the title does not count */
function aaTagLate(w) { return wishTags(w).filter((x) => x.state === 'fail'); }
/* counters of the band: tags set in the last 30 days, still open, failed */
function aaTagCounts() {
  const since = Date.now() - 30 * D;
  const all = S.wishes.flatMap((w) => wishTags(w).map((x) => ({ ...x, w })));
  return {
    set: S.wishes.reduce((n, w) => n + w.events.filter((e) => e.k === 'tagged' && e.at > since).length, 0),
    open: all.filter((x) => x.state === 'wait' && x.w.seerrRequestId && ['processing', 'partial'].includes(x.w.state)).length,
    fail: all.filter((x) => x.state === 'fail').length,
  };
}
/* wishes whose download progress Radarr or Sonarr report right now */
function aaDlWishes() { return S.wishes.filter((w) => { const x = dlInfo(w); return x && x.k !== 'unknown'; }); }
function aaLimitText(l) {
  const part = (x, w) => (x && x.n ? `${x.n} ${w}` : `${w} ohne Limit`);
  if (!l.movie.n && !l.tv.n) return 'ohne Limit';
  const same = l.movie.days === l.tv.days || !l.movie.n || !l.tv.n;
  const days = l.movie.n ? l.movie.days : l.tv.days;
  return same ? `${part(l.movie, 'Filme')} / ${part(l.tv, 'Staffeln')} je ${days} Tage` : `${l.movie.n} Filme je ${l.movie.days} Tage / ${l.tv.n} Staffeln je ${l.tv.days} Tage`;
}
const AA_OPENED = ['created', 'linked', 'manual'];
/* band «Benutzer und Limit» */
function aaUsersBand() {
  const w = S.settings.wishes, lim = w.limit;
  const st = S.accounts.map((a) => accSeerr(a));
  const opened = st.filter((x) => AA_OPENED.includes(x.state)).length;
  const counts = Object.keys(SEERR_ACC).map((k) => [k, st.filter((x) => x.state === k).length]).filter(([, n]) => n);
  const log = S.seerr.log.filter((l) => l.op === 'Benutzer eröffnen' || l.op === 'Passwort abgleichen').slice(0, 3);
  const onoff = (v) => `<span class="chip ${v ? 'ok' : 'outline'}">${v ? 'an' : 'aus'}</span>`;
  const meta = `<span>${opened} von ${S.accounts.length} Konten eröffnet</span><span class="aa-sep">·</span><span>Limit ${esc(aaLimitText(lim))}</span>${st.some((x) => x.state === 'waiting') ? '<span class="chip warn">wartet</span>' : ''}`;
  const body = `<p class="sec">Jedes Konto hat in Seerr einen eigenen Benutzer mit demselben Login. Seine Profile fordern als dieser Benutzer an und tragen einen Tag. Rechte, Limit und Freigabe kommen von diesem Benutzer.</p>
    <dl class="kv aa-kv">
      <dt>Konten in Seerr eröffnen</dt><dd>${onoff(w.provision)}</dd>
      <dt>Passwort abgleichen</dt><dd>${onoff(w.pwSync)}</dd>
      <dt>Passwort</dt><dd>Slouchcat gibt das Passwort an Seerr weiter, wenn es gesetzt oder geändert wird und bei der Anmeldung in Slouchcat. Slouchcat speichert es nie im Klartext. Ein Passwort, das jemand direkt in Seerr ändert, bemerkt Slouchcat nicht.</dd>
      <dt>Lokale Anmeldung in Seerr</dt><dd>${onoff(S.seerr.localLogin)} <span class="muted">wird in Seerr eingestellt (Einstellungen › Benutzer)</span></dd>
      <dt>Limit ab Werk</dt><dd>${esc(aaLimitText(lim))}, rollierend. Slouchcat schreibt es in den Seerr-Benutzer, Seerr zählt.<br><span class="muted">Administratoren in Seerr haben kein Limit.</span></dd>
    </dl>
    <div class="row-gap"><a class="lnk" href="#/admin/wishes/settings" data-fid="aa:sj:usr:set">Schalter und Limit unter Wünsche › Einstellungen ändern</a></div>
    <h3 class="aa-h3">Konten in Seerr</h3>
    <ul class="aa-counts">${counts.map(([k, n]) => `<li><span class="chip ${SEERR_ACC[k].chip}">${n}</span><span>${esc(SEERR_ACC[k].label)}</span></li>`).join('')}</ul>
    <div class="row-gap"><a class="btn secondary sm" href="#/admin/wishes/mapping" data-fid="aa:sj:usr:map">${icon('users', 16)}Seerr-Benutzer verwalten</a></div>
    <h3 class="aa-h3">Zuletzt</h3>
    ${log.length ? `<ul class="feed aa-feed">${log.map((l) => `<li><time datetime="${new Date(l.at).toISOString()}">${fmtTime(l.at)}<br>${ago(l.at)}</time><span class="dot ${l.res === 'ok' ? 'ok' : 'bad'}" aria-label="${l.res === 'ok' ? 'gelungen' : 'Fehler'}"></span><div class="grow"><b>${esc(l.op)}</b> <span class="sec">${esc(l.text)}</span></div></li>`).join('')}</ul>` : '<p class="muted">Noch kein Konto eröffnet und kein Passwort abgeglichen.</p>'}`;
  return band('aa:sj:users', `Benutzer und Limit ${aaNew()}`, meta, body, { icon: 'users' });
}
/* draft of the own addresses; saved with «Speichern» of the band */
function aaArrDraft() { if (!ui.draft.aaArr) ui.draft.aaArr = { radarr: S.arr.radarr.own || '', sonarr: S.arr.sonarr.own || '' }; return ui.draft.aaArr; }
function aaArrDirty() { const d = ui.draft.aaArr; return !!d && ['radarr', 'sonarr'].some((k) => (d[k] || '').trim() !== (S.arr[k].own || '').trim()); }
/* the fact «Download-Fortschritt» of the bands «Abgleich» and «Radarr und Sonarr» */
function aaProgFact(o = {}) {
  if (!progressOn()) return null;
  const down = aaArrDown();
  if (down.length) return { k: 'bad', t: `Fortschritt nicht lesbar: ${down.map((a) => a.name).join(' und ')} nicht erreichbar` };
  const l = aaLast('wishes.progress'), n = aaDlWishes().length;
  return { k: 'ok', t: o.sync ? `Fortschritt aus Radarr und Sonarr: jede Minute, solange Wünsche in Beschaffung sind${l ? ` · zuletzt ${aaAgoS(l.at)}` : ''}` : `an · ${n} ${n === 1 ? 'Wunsch' : 'Wünsche'} mit Fortschritt${l ? ` · zuletzt gelesen ${aaAgoS(l.at)}` : ''}` };
}
/* band «Radarr und Sonarr (Profil-Tags und Fortschritt)» */
function aaArrBand() {
  const tags = S.settings.wishes.tags.enabled, on = aaArrOn(), down = aaArrDown(), d = aaArrDraft(), tc = aaTagCounts(), dl = aaDlWishes().length;
  const dirty = aaArrDirty() ? '<span class="chip warn">ungespeichert</span>' : '';
  const meta = `${dirty}<span>${esc(aaArrSummary())}${on && tags ? ` · ${tc.open} ${tc.open === 1 ? 'Tag' : 'Tags'} offen` : ''}${on && progressOn() ? ` · ${dl} ${dl === 1 ? 'Download' : 'Downloads'}` : ''}</span>${down.length ? '<span class="chip bad">Fehler</span>' : ''}`;
  const row = (k) => {
    const a = S.arr[k], busy = ui.busy['aaArr' + k], t = (ui.aaArrTest || {})[k];
    return `<section class="aa-arr" aria-label="${a.name}"><div class="aa-arrh"><b>${a.name}</b>${a.condition === 'ok' ? '<span class="chip ok">Erreichbar</span>' : '<span class="chip warn">Nicht erreichbar · Eigene Adresse eintragen</span>'}</div>
      <dl class="kv aa-kv"><dt>Adresse aus Seerr</dt><dd class="mono">${esc(a.url)}</dd>
        <dt>Tag Requests in Seerr</dt><dd>${a.tagRequests ? 'an' : 'aus · Empfohlen: dann trägt jeder Titel auch den Tag des Kontos'}</dd></dl>
      <div class="fld"><label for="aa-arr-${k}-own">Eigene Adresse</label><input id="aa-arr-${k}-own" class="inp" type="url" data-bind="ui.draft.aaArr.${k}" data-rerender data-fid="aa:arr:${k}:own" value="${esc(d[k] || '')}" placeholder="Leer: ${esc(a.url)}" ${aaInv('arr' + k)}><span class="hint">Nur nötig, wenn Slouchcat die Adresse aus Seerr nicht erreicht, etwa bei Docker-internen Namen. Leer lassen, um die Adresse aus Seerr zu nehmen. Den Schlüssel liest Slouchcat immer aus Seerr.</span>${aaErrHtml('arr' + k)}</div>
      <div class="row-gap"><button type="button" class="btn secondary sm" data-act="aaArrTest" data-id="${k}" data-fid="aa:arr:${k}:test" ${busy ? 'disabled aria-disabled="true"' : ''} aria-label="${a.name} testen">${busy ? '<span class="spin"></span>' : icon('zap', 14)}Testen</button>${busy ? '<span role="status">Wird geprüft …</span>' : t ? `<span class="sec" role="status">Geprüft ${fmtTime(t.at)}: ${esc(t.text)}</span>` : ''}</div></section>`;
  };
  const cnt = (n, href, fid, label) => `<dt>${label}</dt><dd><a class="lnk" href="${href}" data-fid="${fid}">${n}</a></dd>`;
  const pf = aaProgFact();
  const body = `<p class="sec">Slouchcat legt in Radarr und Sonarr je Profil einen Tag an und hängt ihn an den Titel, sobald Seerr ihn dorthin übergeben hat. Dazu liest es die Warteschlange und den Dateistand der gewünschten Titel, um bei Wünschen den Download-Fortschritt zu zeigen. Sonst ändert Slouchcat in Radarr und Sonarr nichts. Neue Titel meldet weiter der Webhook.</p>
    ${on ? '' : notice('', 'Profil-Tags und Download-Fortschritt sind ausgeschaltet.', 'Slouchcat ruft Radarr und Sonarr nicht auf.')}
    <div class="aa-arrs">${row('radarr')}${row('sonarr')}</div>
    <div class="row-gap"><button type="button" class="btn primary" data-act="aaArrSave" data-fid="aa:arr:save">Speichern</button>${dirty}</div>
    <dl class="kv aa-kv">
      ${tags ? cnt(tc.set, '#/admin/wishes', 'aa:arr:c:set', 'Tags gesetzt (30 Tage)') + cnt(tc.open, '#/admin/wishes?f=notag', 'aa:arr:c:open', 'offen') + cnt(tc.fail, '#/admin/wishes?f=notag', 'aa:arr:c:fail', 'fehlgeschlagen') : '<dt>Profil-Tag</dt><dd>aus</dd>'}
      <dt>Download-Fortschritt ${aaNew()}</dt><dd>${pf ? `<span class="${pf.k === 'bad' ? 'err' : ''}">${esc(pf.t)}</span>` : 'aus · <a class="lnk" href="#/admin/wishes/settings" data-fid="aa:arr:progset">Einstellungen</a>'}</dd>
    </dl>
    <ul class="aa-links-in">
      <li><a class="lnk" href="#/admin/wishes/settings" data-fid="aa:arr:set">Präfix ${esc(S.settings.wishes.tags.prefix)} und Download-Fortschritt · Einstellungen</a></li>
      <li><a class="lnk" href="#/admin/wishes/mapping" data-fid="aa:arr:map">Namen je Profil · Seerr-Benutzer</a></li>
      <li><a class="lnk" href="#/admin/integrations/webhooks" data-fid="aa:arr:hooks">Webhooks von Radarr und Sonarr · unverändert, von Hand eingerichtet</a></li>
    </ul>
    <p class="aa-mockhint">Mockup: Den Zustand von Radarr und Sonarr stellst du unten rechts unter «Mockup» um. Eine eigene Adresse, die «falsch» enthält, ist nicht erreichbar.</p>`;
  return band('aa:sj:arr', `Radarr und Sonarr (Profil-Tags und Fortschritt) ${aaNew()}`, meta, body, { icon: 'pin', open: down.length > 0 });
}
function aaSeerrView() {
  const st = seerrState(), c = S.seerr, d = aaSeerrDraft(), wc = wishCounts(), mc = aaMapCounts();
  const busy = ui.busy.seerrTest, test = ui.aaSeerrTest;
  const status = `<div class="aa-status"><span class="chip ${st.chip}">${st.label}</span>${c.configured ? `<span>${esc(c.version)}</span><span class="aa-sep">·</span><span>Zuletzt abgeglichen ${ago(c.lastSync)}</span>` : '<span>Trage Adresse und Schlüssel ein.</span>'}</div>`;
  const dirty = aaSeerrDirty() ? '<span class="chip warn">ungespeichert</span>' : '';
  const guide = !c.configured ? `<ol class="aa-steps"><li>In Seerr unter Einstellungen › Allgemein den API-Schlüssel kopieren.</li><li>Adresse und Schlüssel hier eintragen und «Verbindung testen».</li><li>«Speichern». Danach unter <a class="lnk" href="#/admin/wishes/mapping" data-fid="aa:sj:guide:map">Wünsche › Seerr-Benutzer</a> die Konten in Seerr eröffnen.</li></ol>` : '';
  const conn = `${guide}
    ${aaSrow('Seerr verwenden', 'Aus: Slouchcat ruft Seerr nicht auf. Wünsche bleiben gespeichert.', sw('S.seerr.enabled', 'Seerr verwenden', { after: 'aaSeerrToggled', disabled: !c.configured }) + (c.configured ? '' : '<span class="muted">Erst nach dem Speichern der Verbindung.</span>'), { isNew: true })}
    ${aaSrow('Adresse', 'Zum Beispiel http://192.168.1.50:5055. Slouchcat spricht nur diese Adresse an. Eine Adresse im Heimnetz ist erlaubt; Weiterleitungen folgt Slouchcat nicht.', `<input class="inp" type="url" data-bind="ui.draft.seerr.url" data-fid="aa:sj:url" value="${esc(d.url)}" placeholder="http://192.168.1.50:5055" aria-label="Adresse" ${aaInv('seerrUrl')}>${aaErrHtml('seerrUrl')}`, { col: true, isNew: true })}
    ${aaSrow('API-Schlüssel', 'Aus Seerr: Einstellungen › Allgemein. Leer lassen, um den gespeicherten zu behalten.', `<input class="inp" type="password" autocomplete="new-password" data-bind="ui.draft.seerr.key" data-fid="aa:sj:key" value="${esc(d.key)}" placeholder="${c.configured && c.keyTail ? 'Gesetzt, endet auf …' + esc(c.keyTail) : ''}" aria-label="API-Schlüssel" ${aaInv('seerrKey')}>${aaErrHtml('seerrKey')}`, { col: true, isNew: true })}
    <div class="row-gap"><button type="button" class="btn secondary" data-act="aaSeerrTest" data-fid="aa:sj:test" ${busy ? 'disabled aria-disabled="true"' : ''}>${busy ? '<span class="spin"></span>' : icon('zap', 16)}Verbindung testen</button><button type="button" class="btn primary" data-act="aaSeerrSave" data-fid="aa:sj:save">Speichern</button><button type="button" class="btn quiet" data-act="aaSeerrDiscard" data-fid="aa:sj:discard" ${aaSeerrDirty() ? '' : 'disabled'}>Verwerfen</button>${dirty}</div>
    ${busy ? '<div class="row-gap" role="status"><span class="spin"></span><span>Verbindung wird geprüft …</span></div>' : test ? `<div class="aa-test">${notice(test.bad ? 'bad' : 'ok', test.bad ? `Der Test ist gescheitert: ${esc(test.rows.filter((r) => r.k === 'bad').map((r) => r.l).join(', '))}.` : test.warn ? `Verbunden · ${aaHintWord(test.warn)}` : 'Alle Prüfungen bestanden.', `Geprüft ${fmtTime(test.at)}.${test.warn && !test.bad ? ' Ein Hinweis hält nichts auf; er sagt, was in Seerr besser eingestellt wäre.' : ''}`)}${aaChecksHtml(test)}</div>` : ''}
    <p class="aa-mockhint">Mockup: Den Zustand von Seerr stellst du unten rechts unter «Mockup» um. Enthält die Adresse oder der Schlüssel «falsch», scheitert der Test.</p>
    ${c.configured ? `<div class="aa-danger"><button type="button" class="btn dq" data-act="aaSeerrDelete" data-fid="aa:sj:delete">${icon('trash', 16)}Verbindung löschen …</button></div>` : ''}`;
  const lastBad = aaLastBad('Abgleich');
  const every = aaSyncEvery(), next = c.lastSync + AA_EVERY_MS[every] - Date.now();
  const syncMeta = c.configured ? `<span>Zuletzt ${ago(c.lastSync)} · alle ${every}</span>${st.k !== 'ok' && st.k !== 'off' ? '<span class="chip bad">Fehler</span>' : ''}` : '<span>Nicht eingerichtet</span>';
  const pf = aaProgFact({ sync: true });
  const sync = `<dl class="kv">
      <dt>Letzter erfolgreicher Abgleich</dt><dd>${c.configured ? aaWhen(c.lastSync) + ` (${ago(c.lastSync)})` : '–'}${st.k !== 'ok' && lastBad && lastBad.at > c.lastSync ? `<br><span class="err">Letzter Versuch ${fmtTime(lastBad.at)}: ${esc(lastBad.text)}</span>` : ''}</dd>
      <dt>Nächster Lauf</dt><dd>${st.k === 'ok' ? `in ${aaDur(Math.max(60e3, next))}` : st.k === 'off' || st.k === 'none' ? 'aus' : `in ${aaDur(Math.max(60e3, next))}, solange Seerr ${st.k === 'unreachable' ? 'nicht erreichbar ist, ohne Wirkung' : 'den Schlüssel ablehnt, ohne Wirkung'}`}</dd>
      <dt>Takt</dt><dd>alle ${every} · <a class="lnk" href="#/admin/jobs?group=system&amp;job=wishes.sync" data-fid="aa:sj:sched">Zeitplan ändern</a></dd>
      <dt>Offene Wünsche</dt><dd><a class="lnk" href="#/admin/wishes" data-act="aaGoWishes" data-f="open" data-fid="aa:sj:c:open">${wc.open}</a></dd>
      <dt>Warten auf Übermittlung</dt><dd><a class="lnk" href="#/admin/wishes" data-act="aaGoWishes" data-f="queued" data-fid="aa:sj:c:queued">${wc.queued}</a></dd>
      <dt>Nicht zuordenbar</dt><dd><a class="lnk" href="#/admin/wishes" data-act="aaGoWishes" data-f="unassigned" data-fid="aa:sj:c:un">${wc.unassigned}</a></dd>
      <dt>Download-Fortschritt ${aaNew()}</dt><dd>${pf ? `<span class="${pf.k === 'bad' ? 'err' : ''}">${esc(pf.t)}</span>` : 'aus'}. Der Abgleich mit Seerr bleibt davon unberührt. ${c.configured ? '<button type="button" class="lnk" data-act="aaToArrBand" data-fid="aa:sj:prog">Zum Band Radarr und Sonarr</button>' : ''}</dd>
    </dl>
    <div class="row-gap"><button type="button" class="btn secondary" data-act="aaSyncNow" data-fid="aa:sj:sync" ${c.configured ? '' : 'disabled'}>${icon('refresh', 16)}Jetzt abgleichen</button><a class="lnk" href="#/admin/jobs?group=system" data-fid="aa:sj:jobs">Zu den Jobs</a></div>`;
  const shown = ui.aaSeerrHook;
  const tpl = AA_SJ_TPL;
  const hook = `<p class="sec">Seerr meldet Änderungen sofort, statt dass Slouchcat bis zu ${every} wartet; ohne Sofortmeldung gleicht Slouchcat alle ${every} ab, der Abgleich bleibt die Grundlage. Seerr kennt nur ein Webhook-Ziel: Nutzt du es schon für etwas anderes, ersetzt diese Adresse es.</p>
    ${shown ? `<div class="card aa-once">${notice('warn', 'Jetzt kopieren, danach ist die Adresse nicht mehr sichtbar.', '')}<div class="fld"><label for="aa-sj-hookurl">Adresse für Seerr</label><input id="aa-sj-hookurl" class="inp mono" readonly value="${esc(shown)}" data-fid="aa:sj:hookurl"></div><div class="row-gap"><button type="button" class="btn primary" data-act="aaSjHookCopy" data-fid="aa:sj:hookcopy">${icon('copy', 16)}Adresse kopieren</button><button type="button" class="btn secondary" data-act="aaSjHookHide" data-fid="aa:sj:hookhide">Ausblenden</button></div>
      <div class="fld"><span class="lbl">JSON-Vorlage für Seerr (Einstellungen › Benachrichtigungen › Webhook)</span><pre class="mono aa-pre">${esc(tpl)}</pre></div><div class="row-gap"><button type="button" class="btn secondary" data-act="aaSjTplCopy" data-fid="aa:sj:tplcopy">${icon('copy', 16)}Vorlage kopieren</button></div></div>` : ''}
    ${c.webhook ? `<dl class="kv"><dt>Adresse</dt><dd class="mono">…${esc(c.webhookTail || '')}</dd><dt>Letzte Meldung</dt><dd>${c.webhookLast ? aaWhen(c.webhookLast) : 'Hat noch nie etwas gemeldet.'}</dd></dl>
      <div class="row-gap"><button type="button" class="btn dq" data-act="aaSjHookRenew" data-fid="aa:sj:hookrenew">Token erneuern</button><button type="button" class="btn dq" data-act="aaSjHookDel" data-fid="aa:sj:hookdel">Entfernen</button><button type="button" class="btn quiet sm" data-act="aaSjHookPing" data-fid="aa:sj:hookping">Meldung simulieren (Mockup)</button></div>`
      : `<div class="row-gap"><button type="button" class="btn secondary" data-act="aaSjHookMake" data-fid="aa:sj:hookmake" ${c.configured ? '' : 'disabled'}>${icon('link', 16)}Adresse erzeugen</button></div>`}`;
  const log = S.seerr.log.slice(0, 20);
  const errs = log.filter((l) => l.res === 'bad').length;
  const logBody = log.length ? `<ul class="feed aa-feed">${log.map((l) => `<li><time datetime="${new Date(l.at).toISOString()}">${fmtTime(l.at)}<br>${ago(l.at)}</time><span class="dot ${l.res === 'ok' ? 'ok' : 'bad'}" aria-label="${l.res === 'ok' ? 'gelungen' : 'Fehler'}"></span><div class="grow"><b>${esc(l.op)}</b> <span class="sec">${esc(l.text)}</span></div></li>`).join('')}</ul><p class="muted aa-small">Nie mit Kopfzeilen oder Schlüssel. Höchstens 20 Einträge.</p>` : '<p class="muted">Noch keine Einträge.</p>';
  const openConn = !['ok', 'off'].includes(st.k);
  const html = `${status}
    ${band('aa:sj:conn', 'Verbindung', `${dirty}<span>${c.configured ? esc(c.url) : 'Keine Adresse'}</span><span class="chip ${st.chip}">${st.label}</span>`, conn, { open: openConn, icon: 'plug' })}
    ${band('aa:sj:sync', 'Abgleich', syncMeta + `<span>${wc.open} offen</span>`, sync, { icon: 'refresh' })}
    ${c.configured ? aaUsersBand() + aaArrBand() : ''}
    ${band('aa:sj:hook', 'Sofortmeldung', `<span class="chip info">Stufe 4</span><span>${c.webhook ? 'Eingerichtet' : 'Aus'}</span>`, hook, { icon: 'zap' })}
    ${band('aa:sj:log', 'Protokoll', log.length ? `<span>${log.length} Einträge · ${errs} ${errs === 1 ? 'Fehler' : 'Fehler'}</span><span>Zuletzt: ${esc(log[0].op)} ${ago(log[0].at)}</span>` : '<span>leer</span>', logBody, { icon: 'list' })}
    <section class="card aa-links" aria-label="Verweise"><ul>
      <li><a class="lnk" href="#/admin/wishes/mapping" data-fid="aa:sj:l:map">Seerr-Benutzer: ${mc.mapped} von ${mc.total} Konten eröffnet oder verknüpft</a></li>
      <li><a class="lnk" href="#/admin/wishes/settings" data-fid="aa:sj:l:set">Was angefordert werden darf</a></li>
      <li><a class="lnk" href="#/admin/wishes" data-fid="aa:sj:l:all">Alle Wünsche</a></li>
      ${aaInt().server.publicUrl ? '' : '<li class="muted">Ohne öffentliche Adresse fehlen Links in E-Mails. <a class="lnk" href="#/admin/integrations/server" data-fid="aa:sj:l:srv">Integrationen › Server</a></li>'}
    </ul></section>`;
  return { h1: 'Seerr', intro: 'Slouchcat reicht Wünsche an Seerr weiter und liest ihren Zustand. Freigabe und Beschaffung bleiben in Seerr.', html };
}
ACT.aaSeerrToggled = () => { logSeerr('Einstellungen', 'ok', S.seerr.enabled ? 'Seerr eingeschaltet' : 'Seerr ausgeschaltet: keine Aufrufe mehr'); save(); toast(S.seerr.enabled ? 'Seerr eingeschaltet.' : 'Seerr ausgeschaltet. Wünsche bleiben gespeichert.', 'ok'); };
/* «Testen» for Radarr or Sonarr: checks the entered address (tag list and queue); simulated: an address with «falsch»
   is unreachable, a new address answers, the saved one answers as the mockup condition says */
ACT.aaArrTest = (d) => {
  const a = S.arr[d.id], own = (aaArrDraft()[d.id] || '').trim();
  if (own && !/^https?:\/\/[^\s/]+/.test(own)) { aaSetErr('arr' + d.id, 'Die Adresse beginnt mit http:// oder https://.'); render(); return; }
  aaSetErr('arr' + d.id);
  ui.busy['aaArr' + d.id] = true; render();
  setTimeout(() => {
    ui.busy['aaArr' + d.id] = false;
    const addr = own || a.url, saved = own === (a.own || '').trim();
    const ok = !/falsch/i.test(addr) && (!saved || a.condition === 'ok' || !!a.aaFalsch);
    const text = ok ? `erreichbar unter ${addr} · Schlüssel gültig · Warteschlange lesbar` : `nicht erreichbar unter ${addr}`;
    (ui.aaArrTest = ui.aaArrTest || {})[d.id] = { at: Date.now(), ok, text };
    logSeerr('Radarr und Sonarr', ok ? 'ok' : 'bad', `${a.name}: ${text}${saved ? '' : ' (noch nicht gespeichert)'}`);
    toast(ok ? `${esc(a.name)} antwortet unter ${esc(addr)}, die Warteschlange ist lesbar.${saved ? '' : ' Speichere die Adresse, damit Slouchcat sie nutzt.'}` : `${esc(a.name)} antwortet nicht unter ${esc(addr)}. Trage eine Adresse ein, die Slouchcat erreicht.`, ok ? 'ok' : 'bad');
    save(); render();
  }, 900);
};
/* «Speichern» of the band: takes the own addresses; simulated: «falsch» is unreachable, a corrected one answers */
ACT.aaArrSave = () => {
  const d = aaArrDraft();
  let bad = false;
  ['radarr', 'sonarr'].forEach((k) => { const v = (d[k] || '').trim(); const e = v && !/^https?:\/\/[^\s/]+/.test(v) ? 'Die Adresse beginnt mit http:// oder https://.' : ''; aaSetErr('arr' + k, e); if (e) bad = true; });
  if (bad) { render(); return; }
  const changed = [];
  ['radarr', 'sonarr'].forEach((k) => {
    const a = S.arr[k], v = (d[k] || '').trim();
    if (v === (a.own || '').trim()) return;
    a.own = v; changed.push(a.name);
    if (/falsch/i.test(v)) { a.condition = 'unreachable'; a.aaFalsch = true; }
    else if (a.aaFalsch || (v && a.condition !== 'ok')) { a.condition = 'ok'; delete a.aaFalsch; }
    logSeerr('Radarr und Sonarr', a.condition === 'ok' ? 'ok' : 'bad', `${a.name}: ${v ? 'eigene Adresse ' + v : 'Adresse aus Seerr'} gespeichert, ${a.condition === 'ok' ? 'erreichbar' : 'nicht erreichbar'}`);
  });
  ui.draft.aaArr = null;
  if (!changed.length) { toast('Nichts geändert.', 'ok', 2500); render(); return; }
  S.wishes.forEach(applyTags);
  toast(`Gespeichert: ${esc(changed.join(' und '))}.`, 'ok');
  commit();
};
/* link «Zum Band Radarr und Sonarr» in the band «Abgleich» */
ACT.aaToArrBand = () => {
  ui.open['aa:sj:arr'] = true; render();
  const el = document.querySelector('[data-fid="band:aa:sj:arr"]');
  if (el) { if (el.scrollIntoView) el.scrollIntoView({ block: 'start' }); el.focus(); }
};
ACT.aaSeerrTest = () => {
  const d = aaSeerrDraft();
  if (!d.url.trim()) { aaSetErr('seerrUrl', 'Gib die Adresse von Seerr ein.'); ui.aaSeerrTest = null; render(); return; }
  aaSetErr('seerrUrl'); aaSetErr('seerrKey');
  ui.busy.seerrTest = true; ui.aaSeerrTest = null; render();
  setTimeout(() => {
    ui.busy.seerrTest = false;
    const r = aaSeerrRun();
    ui.aaSeerrTest = r;
    S.seerr.lastTest = r.at;
    logSeerr('Verbindung testen', r.bad ? 'bad' : 'ok', r.bad ? r.rows.find((x) => x.k === 'bad').l + ': ' + r.rows.find((x) => x.k === 'bad').t : r.warn ? `Seerr 3.5.2, verbunden, ${aaHintWord(r.warn)}: ${r.rows.filter((x) => x.k === 'warn').map((x) => x.l).join(', ')}` : 'Seerr 3.5.2, alle Prüfungen bestanden');
    save(); render();
  }, 1200);
};
ACT.aaSeerrSave = () => {
  const d = aaSeerrDraft(), c = S.seerr;
  const url = d.url.trim();
  aaSetErr('seerrUrl', !url ? 'Gib die Adresse von Seerr ein.' : !/^https?:\/\/[^\s/]+/.test(url) ? 'Die Adresse beginnt mit http:// oder https://.' : /\/api\/v1/.test(url) ? 'Ohne /api/v1: nur die Adresse von Seerr.' : '');
  aaSetErr('seerrKey', !c.configured && !d.key ? 'Gib den API-Schlüssel ein.' : '');
  if (aaErr('seerrUrl') || aaErr('seerrKey')) { render(); return; }
  const first = !c.configured;
  c.url = url;
  if (d.key) c.keyTail = d.key.slice(-4);
  c.configured = true;
  if (first) c.enabled = true;
  if (/falsch/i.test(url)) { c.condition = 'unreachable'; c.aaFalsch = true; }
  else if (/falsch/i.test(d.key)) { c.condition = 'badkey'; c.aaFalsch = true; }
  else if ((first || c.aaFalsch) && c.condition !== 'csrf') { c.condition = 'ok'; delete c.aaFalsch; }
  if (first) c.lastSync = Date.now();
  logSeerr('Einstellungen', 'ok', 'Verbindung gespeichert; Benutzerliste wird geladen');
  ui.draft.seerr = { url: c.url, key: '' };
  toast(first ? 'Verbindung gespeichert. Eröffne jetzt die Konten in Seerr: <a href="#/admin/wishes/mapping">Seerr-Benutzer</a>' : 'Verbindung zu Seerr gespeichert.', 'ok', 6500);
  commit();
};
ACT.aaSeerrDiscard = () => { ui.draft.seerr = null; aaSetErr('seerrUrl'); aaSetErr('seerrKey'); toast('Änderungen verworfen.', 'ok', 2500); render(); };
ACT.aaSeerrDelete = () => aaConfirm({ title: 'Verbindung zu Seerr löschen?', body: `Adresse und Schlüssel werden entfernt. Die ${wishCounts().open} offenen Wünsche bleiben in Slouchcat stehen, werden aber nicht mehr übermittelt und abgeglichen.`, confirm: 'Verbindung löschen', danger: true, act: 'aaSeerrDeleteGo' });
ACT.aaSeerrDeleteGo = () => {
  const c = S.seerr;
  Object.assign(c, { configured: false, enabled: false, url: '', keyTail: '', webhook: false, webhookTail: '' });
  logSeerr('Einstellungen', 'ok', 'Verbindung gelöscht');
  ui.draft.seerr = null; ui.aaSeerrTest = null; ui.aaSeerrHook = null;
  closeDialog(); toast('Verbindung zu Seerr gelöscht. Wünsche bleiben gespeichert.', 'ok'); commit();
};
ACT.aaSyncNow = () => { const ok = syncNow(); aaJobMark('wishes.sync', ok); toast(ok ? 'Abgleich ausgeführt.' : `Abgleich nicht möglich: ${esc(seerrState().label)}.`, ok ? 'ok' : 'bad'); commit(); };
function aaSjNewHook() { const tok = aaTok(); S.seerr.webhook = true; S.seerr.webhookTail = tok.slice(-4); S.seerr.webhookLast = null; ui.aaSeerrHook = `${AA_HOST}/api/v1/webhooks/${tok}`; logSeerr('Sofortmeldung', 'ok', 'Adresse für Seerr erzeugt'); }
ACT.aaSjHookMake = () => { aaSjNewHook(); toast('Adresse erzeugt. Trage sie in Seerr ein.', 'ok'); commit(); };
ACT.aaSjHookCopy = () => aaCopy(ui.aaSeerrHook || '', 'Adresse kopiert.');
ACT.aaSjTplCopy = () => aaCopy(AA_SJ_TPL, 'Vorlage kopiert.');
ACT.aaSjHookHide = () => { ui.aaSeerrHook = null; render(); };
ACT.aaSjHookRenew = () => aaConfirm({ title: 'Token der Sofortmeldung erneuern?', body: 'Die bisherige Adresse hört sofort auf zu arbeiten. Die neue wird einmal angezeigt und muss in Seerr neu eingetragen werden.', confirm: 'Erneuern', danger: true, act: 'aaSjHookRenewGo' });
ACT.aaSjHookRenewGo = () => { aaSjNewHook(); closeDialog(); toast('Neues Token erzeugt. Die alte Adresse gilt nicht mehr.', 'ok'); commit(); };
ACT.aaSjHookDel = () => aaConfirm({ title: 'Sofortmeldung entfernen?', body: 'Die Adresse hört sofort auf zu arbeiten. Slouchcat gleicht weiter alle ' + aaSyncEvery() + ' ab.', confirm: 'Entfernen', danger: true, act: 'aaSjHookDelGo' });
ACT.aaSjHookDelGo = () => { Object.assign(S.seerr, { webhook: false, webhookTail: '', webhookLast: null }); ui.aaSeerrHook = null; logSeerr('Sofortmeldung', 'ok', 'Adresse entfernt'); closeDialog(); toast('Sofortmeldung entfernt.', 'ok'); commit(); };
ACT.aaSjHookPing = () => { S.seerr.webhookLast = Date.now(); const ok = syncNow(); aaJobMark('wishes.sync', ok); logSeerr('Sofortmeldung', ok ? 'ok' : 'bad', ok ? 'MEDIA_APPROVED: Abgleich angestossen' : 'Meldung erhalten, Abgleich nicht möglich'); toast('Seerr hat gemeldet; der Abgleich lief sofort.', 'ok'); commit(); };

/* ---------- M-30 Hintergrundjobs ---------- */
const AA_EVERY = ['1 min', '5 min', '15 min', '30 min', '1 h', '2 h', '6 h', '12 h', '24 h', '48 h', '7 d'];
const AA_EVERY_MS = { '1 min': 60e3, '5 min': 5 * 60e3, '15 min': 15 * 60e3, '30 min': 30 * 60e3, '1 h': H, '2 h': 2 * H, '6 h': 6 * H, '12 h': 12 * H, '24 h': D, '48 h': 2 * D, '7 d': 7 * D };
const AA_EVERY_LABEL = { '1 min': 'jede Minute', '5 min': 'alle 5 Minuten', '15 min': 'alle 15 Minuten', '30 min': 'alle 30 Minuten', '1 h': 'stündlich', '2 h': 'alle 2 Stunden', '6 h': 'alle 6 Stunden', '12 h': 'alle 12 Stunden', '24 h': 'täglich', '48 h': 'alle 2 Tage', '7 d': 'wöchentlich' };
const AA_GROUPS = [['library', 'Bibliothek'], ['metadata', 'Metadaten'], ['delivery', 'Zustellung'], ['system', 'System']];
const AA_POOLS = [['general', 'Allgemein', 4], ['download', 'Downloads', 1], ['notify', 'Benachrichtigungen', 2], ['index', 'Keyframe-Index', 1], ['trickplay', 'Vorschau beim Spulen', 1], ['podcast', 'Podcast-Folgen', 5]];
/* [type, name, group, every ('' = event), pool, off by default, new, description] */
const AA_JOBS = [
  ['scan.library', 'Bibliotheksscan', 'library', '6 h', 'general', 0, 0, 'Sucht in allen Bibliotheken neue, geänderte und fehlende Dateien.'],
  ['library.sweep', 'Aufräumen fehlender Titel', 'library', '1 h', 'general', 0, 0, 'Entfernt Titel, deren Dateien seit 24 Stunden fehlen (weich, 30 Tage zurückholbar).'],
  ['scan.path', 'Ordnerscan', 'library', '', 'general', 0, 0, 'Liest einen Ordner nach Webhook oder Dateiwächter.'],
  ['probe.file', 'Datei prüfen', 'library', '', 'general', 0, 0, 'Liest Spuren, Längen und Formate einer Datei.'],
  ['probe.keyframes', 'Keyframe-Index (ganze Datei lesen)', 'library', '', 'index', 0, 0, 'Baut den Index für schnelles Springen.'],
  ['keyframes.backfill', 'Keyframe-Index nachholen', 'library', '7 d', 'index', 1, 0, 'Holt fehlende Indizes für ältere Dateien nach.'],
  ['segments.detect', 'Intro und Abspann erkennen', 'library', '', 'general', 0, 0, 'Findet Intro und Abspann in Serienfolgen.'],
  ['chapters.images', 'Kapitelbilder erzeugen', 'library', '', 'general', 0, 0, 'Erzeugt die Bilder der Kapitelmarken.'],
  ['chapters.images_backfill', 'Kapitelbilder nachholen', 'library', '7 d', 'general', 1, 0, 'Holt fehlende Kapitelbilder nach.'],
  ['trickplay.generate', 'Vorschau beim Spulen erzeugen', 'library', '', 'trickplay', 0, 0, 'Erzeugt die Vorschaubilder für die Zeitleiste.'],
  ['trickplay.series', 'Vorschau für die übrigen Folgen', 'library', '', 'trickplay', 0, 0, 'Plant die Vorschau für alle Folgen einer Serie.'],
  ['trickplay.backfill', 'Vorschau beim Spulen nachholen', 'library', '7 d', 'trickplay', 1, 0, 'Holt fehlende Vorschauen nach.'],
  ['library.relink', 'Gelöschten Titel wiederverknüpfen', 'library', '', 'general', 0, 0, 'Verknüpft einen zurückgekehrten Titel mit seinem alten Stand.'],
  ['library.delete', 'Bibliothek löschen', 'library', '', 'general', 0, 0, 'Entfernt eine Bibliothek mit allen Einträgen.'],
  ['library.author_display', 'Autorennamen vereinheitlichen', 'library', '', 'general', 0, 0, 'Gleicht die Schreibweise der Autoren an.'],
  ['podcast.poll', 'Podcast-Feeds prüfen', 'library', '5 min', 'general', 0, 0, 'Fragt fällige Feeds nach neuen Folgen.'],
  ['podcast.sync_all', 'Podcast-Folgen nachholen', 'library', '6 h', 'general', 0, 0, 'Plant fehlende Folgen aller Podcasts.'],
  ['podcast.sync', 'Podcast-Folgen planen', 'library', '', 'general', 0, 0, 'Plant die Folgen eines Podcasts.'],
  ['podcast.download', 'Podcast-Folge speichern', 'library', '', 'podcast', 0, 0, 'Lädt eine Podcast-Folge herunter.'],
  ['metadata.match', 'Titel zuordnen', 'metadata', '', 'general', 0, 0, 'Ordnet einen neuen Titel bei TMDB oder TVDB zu.'],
  ['metadata.fill', 'Metadaten auffüllen', 'metadata', '', 'general', 0, 0, 'Holt Texte, Besetzung und Staffeln.'],
  ['metadata.apply', 'Metadaten anwenden', 'metadata', '', 'general', 0, 0, 'Wendet geänderte Einstellungen auf alle Titel an.'],
  ['metadata.vote_backfill', 'Wertungen nachholen', 'metadata', '', 'general', 0, 0, 'Holt fehlende Wertungen nach.'],
  ['metadata.ratings_reread', 'Freigaben neu lesen', 'metadata', '', 'general', 0, 0, 'Liest die Altersfreigaben neu.'],
  ['metadata.original_language_backfill', 'Originalsprachen nachholen', 'metadata', '7 d', 'general', 1, 0, 'Holt die Originalsprache älterer Titel nach.'],
  ['image.fetch', 'Bilder holen', 'metadata', '', 'general', 0, 0, 'Lädt Poster, Hintergründe und Logos in den Bildspeicher.'],
  ['trailer.find', 'Trailer suchen', 'metadata', '', 'general', 0, 0, 'Sucht Trailer bei TMDB, sonst bei YouTube.'],
  ['people.images', 'Personen- und Sammlungsbilder holen', 'metadata', '', 'general', 0, 0, 'Lädt Bilder von Personen und Sammlungen.'],
  ['people.details', 'Angaben zu Personen und Sammlungen holen', 'metadata', '', 'general', 0, 0, 'Holt Lebensläufe und Beschreibungen.'],
  ['people.images_backfill', 'Personen- und Sammlungsbilder nachholen', 'metadata', '7 d', 'general', 1, 0, 'Holt fehlende Personenbilder nach.'],
  ['trakt.refresh', 'Spotlight-Liste abrufen', 'metadata', '24 h', 'general', 0, 0, 'Holt die Trakt-Liste, aus der das Spotlight seine Titel nimmt.'],
  ['podcast.fetch', 'Podcast-Feed abrufen', 'metadata', '', 'general', 0, 0, 'Liest einen Podcast-Feed.'],
  ['radio.catalog_sync', 'Radiokatalog abgleichen', 'metadata', '24 h', 'general', 0, 0, 'Gleicht die Senderliste mit Radio Browser ab.'],
  ['radio.wikidata_sync', 'Senderangaben aus Wikidata', 'metadata', '7 d', 'general', 0, 0, 'Holt Angaben zu den Sendern aus Wikidata.'],
  ['radio.logo.fetch', 'Senderlogo holen', 'metadata', '', 'general', 0, 0, 'Lädt das Logo eines Senders.'],
  ['titles.refresh', 'Fehlende Titel auffrischen', 'metadata', '24 h', 'general', 0, 1, 'Frischt die Angaben zu fehlenden Titeln (fehlende Einträge der Trakt-Liste, offene Wünsche) bei TMDB auf, nach «Spotlight-Liste abrufen».'],
  ['series.catalog', 'Staffelliste der Serien', 'metadata', '7 d', 'general', 0, 1, 'Liest die Staffelliste laut TMDB für Serien in der Bibliothek, damit fehlende Staffeln erkennbar werden. Auch nach dem Zuordnen einer Serie.'],
  ['notify.deliver', 'Mitteilung zustellen', 'delivery', '', 'notify', 0, 0, 'Verschickt eine Benachrichtigung per E-Mail, Telegram oder Web-Push.'],
  ['download.prepare', 'Download vorbereiten', 'delivery', '', 'download', 0, 0, 'Bereitet einen Offline-Download vor.'],
  ['wishes.submit', 'Wunsch übermitteln', 'delivery', '', 'general', 0, 1, 'Reicht einen Wunsch an Seerr weiter; versucht es erneut, solange Seerr nicht erreichbar ist (bis 24 Stunden).'],
  ['update.check', 'Update-Prüfung', 'system', '24 h', 'general', 0, 0, 'Schaut nach, ob es eine neuere Slouchcat-Version gibt.'],
  ['jobs.prune', 'Verlauf aufräumen', 'system', '24 h', 'general', 0, 0, 'Entfernt Läufe, die älter als 30 Tage sind.'],
  ['wishes.sync', 'Wünsche abgleichen', 'system', '5 min', 'general', 0, 1, 'Liest den Zustand der Wünsche aus Seerr und übermittelt wartende.'],
  ['wishes.reconcile', 'Verfügbare Wünsche prüfen', 'system', '1 h', 'general', 0, 1, 'Prüft, ob gewünschte Titel inzwischen in Slouchcat abspielbar sind, und meldet sie.'],
  ['seerr.users', 'Seerr-Benutzer laden', 'system', '24 h', 'general', 0, 1, 'Lädt die Benutzer aus Seerr mit Rechten und Limit.'],
  ['wishes.withdraw', 'Wunsch in Seerr zurückziehen', 'delivery', '', 'general', 0, 1, 'Löscht nach dem Zurückziehen den Request in Seerr, solange er nicht freigegeben ist. War er schon freigegeben, öffnet Slouchcat den Wunsch wieder.'],
  ['wishes.import', 'Anfragen aus Seerr übernehmen (Stufe 4)', 'system', '', 'general', 0, 1, 'Liest die Anfragen eines Seerr-Benutzers, sobald er einem Konto oder Profil zugeordnet wird, und führt sie als Wünsche mit dem Vermerk «In Seerr angelegt».'],
  ['seerr.provision', 'Konten in Seerr eröffnen', 'system', '', 'general', 0, 1, 'Konten in Seerr eröffnen und Rechte, Limit, Passwort nachziehen. Ein Konto mit derselben E-Mail wird verknüpft statt neu angelegt.'],
  ['wishes.tag', 'Profil-Tag setzen', 'system', '', 'general', 0, 1, 'Hängt den Tag des Profils an den Titel, sobald Seerr ihn an Radarr oder Sonarr übergeben hat, und holt Tags nach, die nicht gesetzt werden konnten. Ändert sonst nichts in Radarr und Sonarr.'],
  ['wishes.progress', 'Download-Fortschritt lesen', 'system', '1 min', 'general', 0, 1, 'Liest für Wünsche in Beschaffung die Warteschlange von Radarr und Sonarr. Läuft nur, solange es solche Wünsche gibt; ändert dort nichts.'],
  ['wishes.discover', 'Wünsche-Seiten vorladen', 'metadata', '1 h', 'general', 0, 1, 'Holt über Seerr die Listen der Seiten «Entdecken», «Filme» und «Serien» (Trend, beliebt, demnächst) neu. Ruht, wenn die Seiten aus sind.'],
].map(([k, name, group, every, pool, off, isNew, desc]) => ({ k, name, group, every, pool, off: !!off, isNew: !!isNew, desc }));
const aaJob = (k) => AA_JOBS.find((j) => j.k === k);
function aaHash(s) { let h = 7; for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) % 100003; return h; }

/* runtime state of the jobs (persisted in S so a reload keeps pauses and runs) */
function aaJobsState() {
  if (!S.aaJobs) {
    const now = Date.now();
    const last = {};
    AA_JOBS.forEach((j) => {
      if (aaSeerrJob(j.k)) return;
      if (j.every && j.off) return;
      const span = j.every ? AA_EVERY_MS[j.every] : 3 * H;
      last[j.k] = { at: now - (aaHash(j.k) % Math.max(1, Math.floor(span / 60e3))) * 60e3 - 60e3, res: aaHash(j.k) % 5 === 0 ? 'skipped' : 'ok', err: '' };
    });
    last['seerr.users'] = { at: now - 7 * H, res: 'ok', err: '' };
    last['wishes.reconcile'] = { at: now - 22 * 60e3, res: 'skipped', err: '' };
    last['image.fetch'] = { at: now - 4 * 60e3, res: 'failed', err: 'TMDB antwortet mit 429 (zu viele Anfragen)' };
    S.aaJobs = {
      paused: {},
      last,
      queued: { 'image.fetch': 37, 'metadata.fill': 4, 'probe.file': 2 },
      running: [
        { id: 'r1', k: 'image.fetch', subj: 'Dune: Part Two · Poster und Logo', done: 34, total: 120, since: now - 2 * 60e3 },
        { id: 'r2', k: 'trickplay.generate', subj: 'MobLand S02E06', done: 61, total: 100, since: now - 6 * 60e3 },
      ],
      failed: [
        { id: 'f1', k: 'image.fetch', subj: 'Clayface · Hintergrund', err: 'TMDB antwortet mit 429 (zu viele Anfragen)', at: now - 4 * 60e3, att: '5 von 5' },
        { id: 'f2', k: 'notify.deliver', subj: 'Telegram an grosseltern@example.ch', err: 'Telegram: Bot wurde vom Empfänger blockiert (403)', at: now - 30 * H, att: '5 von 5' },
      ],
      seq: 10,
    };
    save();
  }
  return S.aaJobs;
}
function aaSched(k) { const j = aaJob(k), s = (S.settings.jobs || {})[k] || {}; return { every: s.every || j.every, on: s.on ?? !j.off, win: !!s.win, from: s.from || '02:00', to: s.to || '05:00', once: s.once || '' }; }
function aaSeerrJob(k) { return k.startsWith('wishes.') || k === 'seerr.users' || k === 'seerr.provision'; }
/* last run of the three jobs of Fassung 3 when the job state has none of its own yet */
function aaLastNew(k, own) {
  const newer = (x) => (own && (!x || own.at >= x.at) ? own : x);
  if (k === 'seerr.provision') { const l = S.seerr.log.find((x) => x.op === 'Benutzer eröffnen'); return newer(l ? { at: l.at, res: l.res === 'ok' ? 'ok' : 'failed', err: l.res === 'ok' ? '' : l.text } : null); }
  if (k === 'wishes.tag') {
    if (!S.settings.wishes.tags.enabled) return own || null;
    const fail = aaArrDown()[0];
    return newer({ at: Date.now() - 4 * 60e3, res: fail ? 'failed' : 'ok', err: fail ? `${fail.name} ist nicht erreichbar` : '' });
  }
  if (k === 'wishes.progress') {
    if (!progressOn()) return own || null;
    if (!aaProgWishes().length) return newer({ at: Date.now() - 40 * 60e3, res: 'skipped', err: '' });
    const fail = aaArrDown()[0];
    return newer({ at: Date.now() - 40e3, res: fail ? 'failed' : 'ok', err: fail ? `${fail.name} ist nicht erreichbar` : '' });
  }
  if (k === 'wishes.discover') return S.settings.wishes.fromDiscover === false ? own || null : newer({ at: Date.now() - 17 * 60e3, res: 'ok', err: '' });
  return own || null;
}
/* wishes being acquired, whose progress «Download-Fortschritt lesen» reads */
function aaProgWishes() { return S.wishes.filter((w) => dlInfo(w)); }
/* last run: real state for most jobs, derived from Seerr for the new ones */
function aaLast(k) {
  const js = aaJobsState(), st = seerrState(), own = js.last[k];
  if (aaSeerrJob(k) && !S.seerr.configured) return own ? { ...own, res: 'skipped' } : null;
  if (k === 'wishes.sync') {
    if (st.k !== 'ok' && st.k !== 'off') { const b = aaLastBad('Abgleich'); return { at: b ? b.at : Date.now() - 60e3, res: 'failed', err: st.label }; }
    return own && own.at > S.seerr.lastSync ? own : { at: S.seerr.lastSync, res: 'ok', err: '' };
  }
  if (k === 'wishes.submit') { const l = S.seerr.log.find((x) => x.op === 'Wunsch übermitteln'); return l ? { at: l.at, res: l.res === 'ok' ? 'ok' : 'failed', err: l.res === 'ok' ? '' : l.text } : own || null; }
  if (['seerr.provision', 'wishes.tag', 'wishes.progress', 'wishes.discover'].includes(k)) return aaLastNew(k, own);
  return own || null;
}
function aaQueued(k) { if (k === 'wishes.submit') return S.wishes.filter((w) => w.state === 'queued').length; return aaJobsState().queued[k] || 0; }
function aaWishFails() { return S.wishes.filter((w) => w.state === 'failed' || (w.state === 'queued' && w.detail)); }
function aaFailCount(k) { const js = aaJobsState(); return js.failed.filter((f) => f.k === k).length + (k === 'wishes.submit' ? aaWishFails().length : 0); }
function aaJobMark(k, ok, err) { aaJobsState().last[k] = { at: Date.now(), res: ok === 'skipped' ? 'skipped' : ok ? 'ok' : 'failed', err: ok ? '' : err || seerrState().label }; save(); }
function aaNext(j) {
  const js = aaJobsState();
  if (js.paused[j.k]) return 'pausiert';
  if (j.k === 'seerr.provision') {
    if (!S.settings.wishes.provision) return 'aus · unter Wünsche › Einstellungen ausgeschaltet';
    if (!S.seerr.configured || !S.seerr.enabled) return 'wartet bis Seerr eingerichtet ist';
    const wait = S.accounts.filter((a) => accSeerr(a).state === 'waiting').length;
    return `${esc(((S.settings.jobs || {})[j.k] || {}).every || 'bei Bedarf')}${wait ? ` · ${wait} ${wait === 1 ? 'Konto wartet' : 'Konten warten'} auf den nächsten Abgleich` : ''}`;
  }
  if (j.k === 'wishes.tag') {
    if (!S.settings.wishes.tags.enabled) return 'aus · Profil-Tags ausgeschaltet';
    if (!S.seerr.configured || !S.seerr.enabled) return 'wartet bis Seerr eingerichtet ist';
    return `mit dem Abgleich · ${aaNext(aaJob('wishes.sync'))}`;
  }
  if (j.k === 'wishes.discover' && S.settings.wishes.fromDiscover === false) return 'ruht · Seiten Entdecken, Filme, Serien und Suche ausgeschaltet';
  if (j.k === 'wishes.progress' && !progressOn()) return 'aus · Download-Fortschritt ausgeschaltet';
  if (!j.every) return 'automatisch';
  const s = aaSched(j.k);
  if (!s.on) return s.once ? `aus · Termin: ${esc(s.once.replace('T', ', '))}` : 'aus';
  if (aaSeerrJob(j.k) && (!S.seerr.configured || !S.seerr.enabled)) return 'wartet bis Seerr eingerichtet ist';
  const l = aaLast(j.k);
  const base = j.k === 'wishes.sync' ? S.seerr.lastSync : l ? l.at : Date.now();
  const due = base + AA_EVERY_MS[s.every] - Date.now();
  return `in ${aaDur(Math.max(60e3, due))} · alle ${s.every}${s.win ? ` · ${s.from}–${s.to}` : ''}${s.once ? ` · Termin: ${esc(s.once.replace('T', ', '))}` : ''}`;
}
function aaLastHtml(k) {
  const l = aaLast(k);
  if (!l) return 'noch nie';
  const r = l.res === 'ok' ? '<span class="chip ok">OK</span>' : l.res === 'skipped' ? '<span class="chip info">nichts zu tun</span>' : l.res === 'cancelled' ? '<span class="chip info">abgebrochen</span>' : `<span class="chip bad">Fehler</span> <span class="err">${esc(l.err)}</span>`;
  return `${ago(l.at)} ${r}`;
}
function aaJobStateChip(j) {
  const js = aaJobsState();
  if (js.paused[j.k]) return '<span class="chip warn">pausiert</span>';
  if (js.running.some((r) => r.k === j.k)) return '<span class="chip blue">läuft</span>';
  if (aaQueued(j.k)) return '<span class="chip info">wartet</span>';
  return '<span class="chip outline">ruht</span>';
}
function aaRunsDialogOpen(k) { ui.filter.aaRunTab = 'running'; openDialog('aaRuns', { k }); }

function aaJobRow(j) {
  const js = aaJobsState(), paused = !!js.paused[j.k], q = aaQueued(j.k);
  const trig = j.every ? `Zeitplan ${AA_EVERY_LABEL[aaSched(j.k).every] || ''}${j.k === 'wishes.sync' ? ', «Jetzt abgleichen», Sofortmeldung von Seerr' : j.k === 'trakt.refresh' ? ', «Jetzt abrufen»' : j.k === 'seerr.users' ? ', «Benutzerliste neu laden», Speichern der Verbindung' : j.k === 'wishes.progress' ? ', nur solange Wünsche in Beschaffung sind und «Download-Fortschritt zeigen» an ist' : j.k === 'wishes.discover' ? ', ruht, wenn die Seiten Entdecken, Filme, Serien und Suche aus sind' : ''}` : j.k === 'wishes.tag' ? 'nach jedem Abgleich, Wunsch «In Beschaffung», neuer Mitwunsch' : j.k === 'wishes.submit' ? 'Anfordern in der App, «Erneut übermitteln», fällige Wünsche aus dem Abgleich' : j.k === 'seerr.provision' ? 'Einladung angenommen, Wunschrecht, Limit oder Passwort geändert, Anmeldung, «Alle eröffnen» unter Seerr-Benutzer, Abgleich für wartende Konten' : 'automatisch, durch Ereignisse im Server';
  const acts = [
    j.every || j.k === 'seerr.provision' || j.k === 'wishes.tag' ? `<button type="button" class="btn secondary sm" data-act="aaJobRun" data-id="${j.k}" data-fid="aa:job:${j.k}:run" aria-label="${esc(j.name)} jetzt ausführen">${icon('play', 14)}Jetzt ausführen</button>` : '',
    `<button type="button" class="btn ghost sm" data-act="aaJobPause" data-id="${j.k}" data-fid="aa:job:${j.k}:pause" aria-label="${esc(j.name)} ${paused ? 'fortsetzen' : 'pausieren'}">${icon(paused ? 'play' : 'pause', 14)}${paused ? 'Fortsetzen' : 'Pausieren'}</button>`,
    j.every ? `<button type="button" class="btn ghost sm" data-act="aaJobSched" data-id="${j.k}" data-fid="aa:job:${j.k}:sched">${icon('clock', 14)}Zeitplan</button>` : '',
    `<button type="button" class="btn ghost sm" data-act="aaJobRuns" data-id="${j.k}" data-fid="aa:job:${j.k}:runs">${icon('list', 14)}Läufe</button>`,
    q && j.k !== 'wishes.submit' ? `<button type="button" class="btn dq sm" data-act="aaJobCancelQ" data-id="${j.k}" data-fid="aa:job:${j.k}:cq">Wartende abbrechen (${q})</button>` : '',
  ].join('');
  const hl = ui.aaJobHl === j.k;
  return `<article class="${cls('aa-job', paused && 'paused', j.isNew && 'isnew', hl && 'hl')}" id="aa-job-${j.k.replace(/\./g, '-')}">
    <div class="aa-jn"><b>${esc(j.name)}</b>${j.isNew ? ' ' + aaNew() : ''}<span class="sec">${esc(j.desc)}</span><span class="muted aa-small">Ausgelöst durch: ${esc(trig)} · <span class="mono">${j.k}</span></span></div>
    <div class="aa-jf"><span class="lbl">Zustand</span><span>${aaJobStateChip(j)}${q ? ` <span class="sec">Wartend ${q}</span>` : ''}</span></div>
    <div class="aa-jf"><span class="lbl">Letzter Lauf</span><span>${aaLastHtml(j.k)}</span></div>
    <div class="aa-jf"><span class="lbl">Nächster Lauf</span><span>${aaNext(j)}</span></div>
    <div class="aa-ja">${acts}</div></article>`;
}
function aaFailuresHtml() {
  const js = aaJobsState();
  const wf = aaWishFails();
  const rows = [
    ...wf.map((w) => `<tr><td class="main">Wunsch übermitteln ${aaNew()}</td><td><div class="mini"><span class="po">${img(T[w.titleKey].poster, T[w.titleKey].title)}</span><span><b>${esc(T[w.titleKey].title)}</b><span>${wsChip(w.state)}</span></span></div></td><td>${esc(w.state === 'failed' ? FAIL_TEXT[w.failReason] || 'Fehlgeschlagen' : w.detail)}</td><td class="nowrap">${ago(w.closed || w.created)}</td><td class="end"><span class="ta"><button type="button" class="btn secondary sm" data-act="aaWishRetry" data-id="${w.id}" data-fid="aa:fail:w:${w.id}">Erneut</button><a class="btn ghost sm" href="#/admin/wishes" data-fid="aa:fail:wl:${w.id}">Zum Wunsch</a></span></td></tr>`),
    ...js.failed.map((f) => `<tr><td class="main">${esc(aaJob(f.k).name)}</td><td>${esc(f.subj)}</td><td>${esc(f.err)} <span class="muted">· ${esc(f.att)} Versuche</span></td><td class="nowrap">${ago(f.at)}</td><td class="end"><span class="ta"><button type="button" class="btn secondary sm" data-act="aaFailRetry" data-id="${f.id}" data-fid="aa:fail:${f.id}:retry">Erneut</button><button type="button" class="btn dq sm" data-act="aaFailDel" data-id="${f.id}" data-fid="aa:fail:${f.id}:del">Löschen</button></span></td></tr>`),
  ];
  return `<section class="stack" aria-label="Fehlschläge (alle Arten)"><div class="row-gap"><h2 class="aa-h2 grow">Fehlschläge (alle Arten)</h2>${rows.length ? `<button type="button" class="btn secondary sm" data-act="aaFailRetryAll" data-fid="aa:fail:retryall">Alle erneut versuchen</button><button type="button" class="btn dq sm" data-act="aaFailDelAll" data-fid="aa:fail:delall" ${js.failed.length ? '' : 'disabled'}>Alle löschen …</button>` : ''}</div>
    ${rows.length ? `<div class="tblw"><table class="tbl"><thead><tr><th>Job</th><th>Gegenstand</th><th>Fehler</th><th>Seit</th><th class="end">Aktionen</th></tr></thead><tbody>${rows.join('')}</tbody></table></div>` : '<div class="unchanged"><b>Keine Fehlschläge.</b></div>'}
    ${wf.length ? '<p class="muted aa-small">Wünsche, die auf die Übermittlung warten oder fehlgeschlagen sind, stehen hier mit dem Grund aus Seerr. «Erneut» übermittelt sofort.</p>' : ''}</section>`;
}
function aaJobsPage(pm, query) {
  const js = aaJobsState();
  const qk = (query.group || '') + '|' + (query.job || '');
  if ((query.group || query.job) && ui.aaJobsQ !== qk) { ui.aaJobsQ = qk; if (query.group) ui.filter.aaJobGroup = query.group; ui.aaJobHl = query.job || null; }
  const grp = ui.filter.aaJobGroup || 'library', flt = ui.filter.aaJobFlt || 'all';
  const sched = AA_JOBS.filter((j) => j.every && aaSched(j.k).on && !js.paused[j.k]).length;
  const queued = AA_JOBS.reduce((n, j) => n + aaQueued(j.k), 0);
  const pausedJobs = AA_JOBS.filter((j) => js.paused[j.k]);
  const failN = js.failed.length + aaWishFails().length;
  const stats = `<section class="stats" style="--n:4" aria-label="Zusammenfassung">
    <div class="stat"><span class="v">${js.running.length}</span><span class="l">Laufend</span></div>
    <div class="stat"><span class="v">${queued}</span><span class="l">Wartend</span><span class="n">${sched} geplant</span></div>
    <div class="stat ${pausedJobs.length ? 'warn' : ''}"><span class="v">${pausedJobs.length}</span><span class="l">Pausiert</span></div>
    <button type="button" class="stat ${failN ? 'bad' : ''} aa-statbtn" data-act="aaJobGroup" data-g="failures" data-fid="aa:jobs:statfail"><span class="v">${failN}</span><span class="l">Fehlgeschlagen</span><span class="n">Fehlschläge ansehen</span></button></section>`;
  const pools = AA_POOLS.map(([k, n, w]) => { const run = js.running.filter((r) => aaJob(r.k).pool === k).length; const q = AA_JOBS.filter((j) => j.pool === k).reduce((s, j) => s + aaQueued(j.k), 0); return `<li><b>${n}</b><span>${w} Arbeiter · ${run ? `${run} belegt` : 'frei'}${q ? ` · Stau ca. ${aaDur(q * 9e3)}` : ''}</span></li>`; }).join('');
  const poolMeta = AA_POOLS.slice(0, 3).map(([k, n, w]) => `${n} ${w}`).join(' · ') + ' …';
  const banner = pausedJobs.length ? notice('warn', `${pausedJobs.length} ${pausedJobs.length === 1 ? 'Job' : 'Jobs'} pausiert`, `Pausierte Jobs starten keine Läufe, bis du sie fortsetzt: ${pausedJobs.map((j) => esc(j.name)).join(', ')}.${pausedJobs.some((j) => j.k === 'wishes.submit') ? ' Wünsche bleiben «Wird übermittelt»; nach 24 Stunden gelten sie als fehlgeschlagen.' : ''}`, pausedJobs.map((j) => `<button type="button" class="btn secondary sm" data-act="aaJobPause" data-id="${j.k}" data-fid="aa:banner:${j.k}">Fortsetzen: ${esc(j.name)}</button>`).join('')) : '';
  const running = `<section class="card" aria-labelledby="aa-running-h"><div class="ch"><span class="ik">${icon('zap', 18)}</span><h2 id="aa-running-h">Läuft gerade</h2><span class="muted">${js.running.length} ${js.running.length === 1 ? 'Lauf' : 'Läufe'}</span></div>
    ${js.running.length ? `<div class="tblw"><table class="tbl"><thead><tr><th>Job</th><th>Gegenstand</th><th>Fortschritt</th><th>Seit</th><th>Restzeit</th><th class="end">Aktionen</th></tr></thead><tbody>${js.running.map((r) => { const pct = Math.round((r.done / r.total) * 100); return `<tr><td class="main">${esc(aaJob(r.k).name)}</td><td>${esc(r.subj)}</td><td><span class="aa-bar" role="progressbar" aria-valuenow="${pct}" aria-valuemin="0" aria-valuemax="100" aria-label="${esc(aaJob(r.k).name)}: ${pct} %"><i style="width:${pct}%"></i></span> <span class="aa-small">${r.done} von ${r.total}</span></td><td class="nowrap">seit ${aaDur(Date.now() - r.since)}</td><td class="nowrap">${r.manual ? 'wird berechnet' : `noch ca. ${aaDur(((r.total - r.done) / Math.max(1, r.done)) * (Date.now() - r.since))}`}</td><td class="end"><span class="ta"><button type="button" class="btn ghost sm" data-act="aaJobPause" data-id="${r.k}" data-fid="aa:run:${r.id}:pause" aria-label="${esc(aaJob(r.k).name)} pausieren (hält diesen und alle laufenden Läufe des Jobs an)">Lauf pausieren</button><button type="button" class="btn dq sm" data-act="aaRunCancel" data-id="${r.id}" data-fid="aa:run:${r.id}:cancel">Abbrechen</button></span></td></tr>`; }).join('')}</tbody></table></div>` : '<p class="muted">Gerade läuft nichts.</p>'}</section>`;
  const tabs = `<div class="subtabs" role="group" aria-label="Gruppen">${AA_GROUPS.map(([g, l]) => { const n = AA_JOBS.filter((j) => j.group === g).length; const nn = AA_JOBS.some((j) => j.group === g && j.isNew); return `<button type="button" data-act="aaJobGroup" data-g="${g}" aria-pressed="${grp === g}" data-fid="aa:jobs:tab:${g}">${l}<span class="c">${n}</span>${nn ? aaNew() : ''}</button>`; }).join('')}<button type="button" data-act="aaJobGroup" data-g="failures" aria-pressed="${grp === 'failures'}" data-fid="aa:jobs:tab:failures">Fehlschläge<span class="c ${failN ? 'hot' : ''}">${failN}</span></button></div>`;
  let body;
  if (grp === 'failures') body = aaFailuresHtml();
  else {
    const list = AA_JOBS.filter((j) => j.group === grp).filter((j) => flt === 'all' || (flt === 'running' && js.running.some((r) => r.k === j.k)) || (flt === 'failed' && ((aaLast(j.k) || {}).res === 'failed' || aaFailCount(j.k))) || (flt === 'paused' && js.paused[j.k]));
    const fl = [['all', 'Alle'], ['running', 'Läuft'], ['failed', 'Fehler'], ['paused', 'Pausiert']];
    body = `<div class="pills" role="group" aria-label="Filter">${fl.map(([v, l]) => `<button type="button" class="pill" data-act="set" data-path="ui.filter.aaJobFlt" data-value="${v}" aria-pressed="${flt === v}" data-fid="aa:jobs:flt:${v}">${l}</button>`).join('')}</div>
      <div class="aa-jobs">${list.map(aaJobRow).join('') || '<div class="unchanged"><b>Kein Job in diesem Filter.</b></div>'}</div>`;
  }
  return { type: 'admin', area: 'automation', page: 'jobs', title: 'Hintergrundjobs', intro: 'Was der Server im Hintergrund tut: welche Jobs laufen, seit wann, wie lange noch, und wann der nächste Lauf kommt. Jeden Job kannst du jetzt ausführen, pausieren und seine Läufe abbrechen.',
    html: `${stats}${banner}${band('aa:jobs:pools', 'Arbeiter', `<span>${poolMeta}</span>`, `<ul class="aa-pools">${pools}</ul><p class="muted aa-small">Die Zahl der Arbeiter stellst du nur mit SLOUCHCAT_WORKERS in deploy/.env ein.</p>`, { icon: 'users' })}${running}${tabs}${body}`,
    after: () => { if (ui.aaJobHl) { const el = document.getElementById('aa-job-' + ui.aaJobHl.replace(/\./g, '-')); if (el && el.scrollIntoView) el.scrollIntoView({ block: 'center' }); ui.aaJobHl = null; } } };
}
route('/admin/jobs', aaJobsPage);

ACT.aaJobGroup = (d) => { ui.filter.aaJobGroup = d.g; render(); };
ACT.aaJobRun = (d) => {
  const j = aaJob(d.id), js = aaJobsState();
  if (js.paused[j.k]) { toast(`«${esc(j.name)}» ist pausiert. Setze den Job zuerst fort.`, 'bad'); return; }
  if (j.k === 'wishes.sync') { const ok = syncNow(); aaJobMark(j.k, ok); toast(ok ? '«Wünsche abgleichen» ist gelaufen.' : `«Wünsche abgleichen»: ${esc(seerrState().label)}.`, ok ? 'ok' : 'bad'); commit(); return; }
  if (j.k === 'seerr.users') {
    const ok = seerrState().k === 'ok';
    aaJobMark(j.k, ok, seerrState().label);
    logSeerr('Benutzerliste laden', ok ? 'ok' : 'bad', ok ? `${S.seerrUsers.length} Benutzer geladen` : seerrState().label);
    toast(ok ? `${S.seerrUsers.length} Seerr-Benutzer geladen.` : `«Seerr-Benutzer laden»: ${esc(seerrState().label)}.`, ok ? 'ok' : 'bad'); commit(); return;
  }
  if (j.k === 'seerr.provision') {
    if (!S.settings.wishes.provision) { aaJobMark(j.k, 'skipped'); toast('«Konten in Seerr eröffnen» ist unter Wünsche › Einstellungen ausgeschaltet. Nichts eröffnet.', 'warn'); commit(); return; }
    if (seerrState().k !== 'ok') { const n = provisionAll(); aaJobMark(j.k, false, seerrState().label); toast(`«Konten in Seerr eröffnen»: ${esc(seerrState().label)}. ${n ? `${n} ${n === 1 ? 'Konto wartet' : 'Konten warten'} auf den nächsten Abgleich.` : ''}`, 'bad'); commit(); return; }
    const before = S.accounts.filter((a) => !accSeerr(a).user).map((a) => a.email);
    const n = provisionAll();
    aaJobMark(j.k, n ? true : 'skipped');
    toast(n ? `${n} ${n === 1 ? 'Konto' : 'Konten'} in Seerr eröffnet oder verknüpft: ${esc(before.join(', '))}.` : '«Konten in Seerr eröffnen»: Alle Konten haben einen Seerr-Benutzer, nichts zu tun.', 'ok');
    commit(); return;
  }
  if (j.k === 'wishes.tag') {
    if (!S.settings.wishes.tags.enabled) { aaJobMark(j.k, 'skipped'); toast('Profil-Tags sind ausgeschaltet. Nichts zu tun.', 'warn'); commit(); return; }
    const count = (s) => S.wishes.reduce((n, w) => n + wishTags(w).filter((x) => x.state === s).length, 0);
    const set0 = count('set');
    S.wishes.forEach(applyTags);
    const added = count('set') - set0, failed = count('fail'), down = aaArrDown();
    aaJobMark(j.k, failed ? false : added ? true : 'skipped', down.map((a) => a.name + ' ist nicht erreichbar').join(', '));
    toast(failed ? `${failed} ${failed === 1 ? 'Tag' : 'Tags'} nicht gesetzt: ${esc(down.map((a) => a.name).join(', '))} nicht erreichbar. Die Wünsche laufen weiter.` : added ? `${added} ${added === 1 ? 'Tag' : 'Tags'} gesetzt.` : 'Alle Tags sind gesetzt, nichts zu tun.', failed ? 'bad' : 'ok');
    commit(); return;
  }
  if (j.k === 'wishes.progress') {
    if (!progressOn()) { aaJobMark(j.k, 'skipped'); toast('Der Download-Fortschritt ist ausgeschaltet. Nichts zu tun.', 'warn'); commit(); return; }
    const list = aaProgWishes(), down = aaArrDown();
    const read = list.filter((w) => dlInfo(w).k !== 'unknown').length, missed = list.length - read;
    aaJobMark(j.k, missed ? false : read ? true : 'skipped', down.map((a) => a.name + ' ist nicht erreichbar').join(', '));
    toast(!list.length ? 'Kein Wunsch in Beschaffung, nichts zu lesen.' : missed ? `${read} von ${list.length} ${list.length === 1 ? 'Wunsch' : 'Wünschen'} gelesen. ${esc(down.map((a) => a.name).join(' und '))} nicht erreichbar; der Fortschritt fehlt so lange.` : `Fortschritt von ${read} ${read === 1 ? 'Wunsch' : 'Wünschen'} aus Radarr und Sonarr gelesen.`, missed ? 'bad' : 'ok');
    commit(); return;
  }
  if (j.k === 'wishes.discover') {
    if (S.settings.wishes.fromDiscover === false) { aaJobMark(j.k, 'skipped'); toast('«Wünsche-Seiten vorladen» ruht: Die Seiten Entdecken, Filme, Serien und Suche sind ausgeschaltet.', 'warn'); commit(); return; }
    const ok = seerrState().k === 'ok';
    const n = Object.values(window.DISCOVER_ROWS || {}).reduce((s, l) => s + l.length, 0);
    aaJobMark(j.k, ok, seerrState().label);
    toast(ok ? `Listen aufgefrischt: ${DISC_ROWS.length} Reihen mit ${n} Titeln.` : `«Wünsche-Seiten vorladen»: ${esc(seerrState().label)}. Die Seiten zeigen den letzten Stand.`, ok ? 'ok' : 'bad');
    commit(); return;
  }
  if (j.k === 'wishes.reconcile') { aaJobMark(j.k, 'skipped'); toast('«Verfügbare Wünsche prüfen»: kein neuer Titel abspielbar, nichts zu tun.', 'ok'); commit(); return; }
  const r = { id: 'r' + ++js.seq, k: j.k, subj: 'Von Hand gestartet', done: 0, total: 1, since: Date.now(), manual: true };
  js.running.push(r);
  toast(`«${esc(j.name)}» läuft.`, 'ok', 2500);
  commit();
  setTimeout(() => { const s = aaJobsState(); if (!s.running.some((x) => x.id === r.id)) return; s.running = s.running.filter((x) => x.id !== r.id); s.last[j.k] = { at: Date.now(), res: 'ok', err: '' }; save(); toast(`«${esc(j.name)}» ist fertig.`, 'ok', 2500); render(); }, 2500);
};
ACT.aaJobPause = (d) => {
  const j = aaJob(d.id), js = aaJobsState();
  js.paused[j.k] = !js.paused[j.k];
  if (!js.paused[j.k]) delete js.paused[j.k];
  else js.running = js.running.filter((r) => r.k !== j.k || (js.queued[j.k] = (js.queued[j.k] || 0) + 1, false));
  toast(js.paused[j.k] ? `«${esc(j.name)}» pausiert.${j.k === 'wishes.submit' ? ' Wünsche bleiben «Wird übermittelt»; nach 24 Stunden gelten sie als fehlgeschlagen.' : ''}` : `«${esc(j.name)}» läuft weiter.`, js.paused[j.k] ? 'warn' : 'ok');
  commit();
};
ACT.aaJobCancelQ = (d) => { const j = aaJob(d.id), n = aaQueued(j.k); aaConfirm({ title: `Wartende Läufe von ${esc(j.name)} abbrechen?`, body: `${n} ${n === 1 ? 'wartender Lauf wird' : 'wartende Läufe werden'} abgebrochen und nicht nachgeholt.${j.k === 'notify.deliver' ? ' Eine abgebrochene Zustellung kommt nicht an.' : ''}`, confirm: 'Abbrechen', keep: 'Behalten', danger: true, act: 'aaJobCancelQGo', id: j.k }); };
ACT.aaJobCancelQGo = (d) => { aaJobsState().queued[d.id] = 0; if (ui.aaRuns) delete ui.aaRuns[d.id]; closeDialog(); toast('Wartende Läufe abgebrochen.', 'ok'); commit(); };
ACT.aaRunCancel = (d) => { const r = aaJobsState().running.find((x) => x.id === d.id); if (!r) return; aaConfirm({ title: `Lauf von ${esc(aaJob(r.k).name)} abbrechen?`, body: `Der Lauf endet sofort und wird nicht nachgeholt.${r.k.startsWith('scan.') ? ' Die Bibliothek endet als «abgebrochen»; der nächste Scan holt nach.' : ''}`, confirm: 'Abbrechen', keep: 'Behalten', danger: true, act: 'aaRunCancelGo', id: d.id }); };
ACT.aaRunCancelGo = (d) => { const js = aaJobsState(); const r = js.running.find((x) => x.id === d.id); js.running = js.running.filter((x) => x.id !== d.id); if (r) js.last[r.k] = { at: Date.now(), res: 'cancelled', err: '' }; closeDialog(); toast('Lauf abgebrochen.', 'ok'); commit(); };
ACT.aaWishRetry = (d) => {
  const before = wishById(d.id);
  const was = before && before.state;
  const w = retryWish(d.id);
  if (was === 'queued' && w) toast(w.state === 'queued' ? `«${esc(T[w.titleKey].title)}» wartet weiter: ${esc(w.detail)}.` : `«${esc(T[w.titleKey].title)}» übermittelt: ${STATE_LABEL[w.state]}.`, w.state === 'queued' ? 'warn' : 'ok');
  commit();
};
ACT.aaFailRetry = (d) => { const js = aaJobsState(); const f = js.failed.find((x) => x.id === d.id); js.failed = js.failed.filter((x) => x.id !== d.id); if (f) js.queued[f.k] = (js.queued[f.k] || 0) + 1; toast('Lauf wieder eingereiht.', 'ok', 2500); commit(); };
ACT.aaFailDel = (d) => aaConfirm({ title: 'Fehlgeschlagenen Lauf löschen?', body: 'Slouchcat versucht diesen Lauf nicht mehr. Unter «Läufe» steht er danach als «abgebrochen», bis die Aufräumung ihn später entfernt.', confirm: 'Löschen', danger: true, act: 'aaFailDelGo', id: d.id });
ACT.aaFailDelGo = (d) => { const js = aaJobsState(); js.failed = js.failed.filter((x) => x.id !== d.id); closeDialog(); toast('Lauf gelöscht.', 'ok'); commit(); };
ACT.aaFailRetryAll = () => { const n = aaJobsState().failed.length + aaWishFails().length; aaConfirm({ title: `Alle ${n} fehlgeschlagenen Läufe erneut versuchen?`, body: 'Die Läufe kommen mit zurückgesetzten Versuchen wieder in die Warteschlange. Wünsche übermittelt Slouchcat sofort erneut.', confirm: 'Alle erneut versuchen', act: 'aaFailRetryAllGo' }); };
ACT.aaFailRetryAllGo = () => { const js = aaJobsState(); js.failed.forEach((f) => (js.queued[f.k] = (js.queued[f.k] || 0) + 1)); js.failed = []; aaWishFails().map((w) => w.id).forEach((id) => retryWish(id)); closeDialog(); toast('Alle Fehlschläge wieder eingereiht.', 'ok'); commit(); };
ACT.aaFailDelAll = () => { const n = aaJobsState().failed.length; aaConfirm({ title: `Alle ${n} fehlgeschlagenen Läufe löschen?`, body: 'Slouchcat versucht diese Läufe nicht mehr. Wünsche bleiben stehen; sie verwaltest du unter «Wünsche».', confirm: 'Alle löschen', danger: true, act: 'aaFailDelAllGo' }); };
ACT.aaFailDelAllGo = () => { aaJobsState().failed = []; closeDialog(); toast('Fehlgeschlagene Läufe gelöscht.', 'ok'); commit(); };

/* dialog «Zeitplan von X» */
ACT.aaJobSched = (d) => { const s = aaSched(d.id); ui.draft.aaSched = { k: d.id, ...s }; openDialog('aaSched', { k: d.id }); };
DIALOGS.aaSched = (p) => {
  const j = aaJob(p.k), s = ui.draft.aaSched;
  const hint = j.k === 'trakt.refresh' ? 'Takt und Ein/Aus sind dieselbe Einstellung wie auf der Seite Spotlight.' : j.k === 'update.check' ? 'Ein/Aus ist dieselbe Einstellung wie die Update-Prüfung unter Integrationen › Server.' : j.k === 'wishes.sync' ? 'Hier ist der einzige Ort des Abgleichtakts. Die Seite Seerr zeigt ihn nur an.' : '';
  return {
    title: `Zeitplan von ${esc(j.name)}`,
    body: `<p class="lead">Takt, Zeitfenster und Ein/Aus. Der nächste Lauf richtet sich danach neu aus; «Jetzt ausführen» ignoriert das Fenster.</p>
      <div class="row-gap">${sw('ui.draft.aaSched.on', 'Zeitplan eingeschaltet')}<span>Zeitplan eingeschaltet</span></div>
      <div class="fld"><label for="aa-sched-every">Takt</label><select id="aa-sched-every" class="sel" data-bind="ui.draft.aaSched.every" data-rerender data-fid="aa:sched:every">${AA_EVERY.map((e) => `<option value="${e}" ${s.every === e ? 'selected' : ''}>${AA_EVERY_LABEL[e]}</option>`).join('')}</select></div>
      <label class="check"><input type="checkbox" data-bind="ui.draft.aaSched.win" data-fid="aa:sched:win" ${s.win ? 'checked' : ''}>Nur in einem Zeitfenster</label>
      ${s.win ? `<div class="aa-hostport"><div class="fld"><label for="aa-sched-from">Von</label><input id="aa-sched-from" class="inp" type="time" data-bind="ui.draft.aaSched.from" data-fid="aa:sched:from" value="${esc(s.from)}"></div><div class="fld"><label for="aa-sched-to">Bis</label><input id="aa-sched-to" class="inp" type="time" data-bind="ui.draft.aaSched.to" data-fid="aa:sched:to" value="${esc(s.to)}"></div></div><p class="muted aa-small">Zeitzone des Servers: Europe/Zurich. Ein Fenster darf über Mitternacht gehen.</p>` : ''}
      <div class="fld"><label for="aa-sched-once">Einmalig am</label><div class="row-gap"><input id="aa-sched-once" class="inp grow" type="datetime-local" data-bind="ui.draft.aaSched.once" data-rerender data-fid="aa:sched:once" value="${esc(s.once)}">${s.once ? '<button type="button" class="btn ghost sm" data-act="set" data-path="ui.draft.aaSched.once" data-value="" data-fid="aa:sched:clearonce">Termin entfernen</button>' : ''}</div><span class="hint">Zeitzone des Servers: Europe/Zurich. Der Termin läuft einmal zusätzlich zum Takt, auch ausserhalb des Zeitfensters.</span></div>
      ${hint ? notice('', hint, '') : ''}`,
    actions: `<button type="button" class="btn secondary" data-act="close" data-fid="aa:sched:discard">Verwerfen</button><button type="button" class="btn primary" data-act="aaSchedSave" data-fid="aa:sched:save">Speichern</button>`,
  };
};
ACT.aaSchedSave = () => {
  const s = ui.draft.aaSched, j = aaJob(s.k);
  S.settings.jobs = S.settings.jobs || {};
  S.settings.jobs[s.k] = { every: s.every, on: !!s.on, win: !!s.win, from: s.from, to: s.to, once: s.once || '' };
  if (s.k === 'update.check') aaInt().server.update = !!s.on;
  closeDialog();
  toast(`Zeitplan von «${esc(j.name)}» gespeichert: ${s.on ? AA_EVERY_LABEL[s.every] : 'aus'}.`, 'ok');
  commit();
};

/* dialog «Läufe von X» */
ACT.aaJobRuns = (d) => aaRunsDialogOpen(d.id);
function aaRunsOf(k) {
  ui.aaRuns = ui.aaRuns || {};
  const js = aaJobsState(), j = aaJob(k), now = Date.now();
  const titles = Object.keys(T);
  if (!ui.aaRuns[k]) {
    const h = aaHash(k), mk = (i, extra) => ({ id: `${k}:${i}`, subj: k.startsWith('scan.') || k === 'library.sweep' ? ['Filme', 'Serien', 'Anime', 'Dokus'][i % 4] : T[titles[(h + i * 7) % titles.length]].title, prio: j.every ? 5 : 3, start: now - (i + 1) * (j.every ? AA_EVERY_MS[j.every] : 40 * 60e3), dur: `${(h % 50) + i + 2} s`, att: '1 von 5', err: '', ...extra });
    ui.aaRuns[k] = { queued: Array.from({ length: Math.min(5, js.queued[k] || 0) }, (_, i) => mk(i + 20, { start: null, att: '0 von 5' })), done: [0, 1, 2, 3].map((i) => mk(i)), cancelled: h % 3 === 0 ? [mk(9, { dur: '–' })] : [] };
  }
  const R = ui.aaRuns[k];
  const out = {
    running: js.running.filter((r) => r.k === k).map((r) => ({ id: r.id, subj: r.subj, prio: 7, start: r.since, dur: aaDur(now - r.since), att: '1 von 5', err: '', live: true })),
    queued: R.queued,
    planned: j.every && aaSched(k).on && !js.paused[k] ? [{ id: k + ':plan', subj: 'Nächster Lauf nach Zeitplan', prio: 5, start: null, plan: aaNext(j), att: '–', err: '' }] : [],
    failed: js.failed.filter((f) => f.k === k).map((f) => ({ id: f.id, subj: f.subj, prio: 3, start: f.at, dur: '–', att: f.att, err: f.err, gen: true })),
    done: R.done,
    cancelled: R.cancelled,
  };
  if (k === 'wishes.submit') {
    out.queued = S.wishes.filter((w) => w.state === 'queued').map((w) => ({ id: w.id, subj: T[w.titleKey].title, prio: 7, start: null, att: '–', err: w.detail || '', wish: true }));
    out.failed = S.wishes.filter((w) => w.state === 'failed').map((w) => ({ id: w.id, subj: T[w.titleKey].title, prio: 7, start: w.closed || w.created, dur: '–', att: '1 von 1', err: FAIL_TEXT[w.failReason] || '', wish: true }));
    out.done = S.wishes.filter((w) => w.seerrRequestId && w.origin === 'slouchcat' && w.state !== 'failed').map((w) => ({ id: w.id, subj: T[w.titleKey].title, prio: 7, start: w.created + 2e3, dur: '1 s', att: '1 von 5', err: '' }));
  }
  return out;
}
const AA_RUN_TABS = [['running', 'Laufend'], ['queued', 'Wartend'], ['planned', 'Geplant'], ['failed', 'Fehlgeschlagen'], ['done', 'Erledigt'], ['cancelled', 'Abgebrochen']];
DIALOGS.aaRuns = (p) => {
  const j = aaJob(p.k), runs = aaRunsOf(p.k), tab = ui.filter.aaRunTab || 'running', list = runs[tab];
  const act = (r) => {
    if (tab === 'failed') return r.wish ? `<button type="button" class="btn secondary sm" data-act="aaRunsWishRetry" data-id="${r.id}" data-k="${p.k}" data-fid="aa:runs:${r.id}:retry">Erneut</button>` : `<button type="button" class="btn secondary sm" data-act="aaRunsRetry" data-id="${r.id}" data-k="${p.k}" data-fid="aa:runs:${r.id}:retry" aria-label="Lauf ${esc(r.id)} erneut einreihen">Erneut</button><button type="button" class="btn dq sm" data-act="aaRunsDel" data-id="${r.id}" data-k="${p.k}" data-fid="aa:runs:${r.id}:del">Löschen</button>`;
    if (tab === 'queued' && r.wish) return `<button type="button" class="btn secondary sm" data-act="aaRunsWishRetry" data-id="${r.id}" data-k="${p.k}" data-fid="aa:runs:${r.id}:retry">Jetzt übermitteln</button>`;
    if (tab === 'queued' || tab === 'running') return `<button type="button" class="btn dq sm" data-act="aaRunsCancel" data-id="${r.id}" data-k="${p.k}" data-fid="aa:runs:${r.id}:cancel" aria-label="Lauf ${esc(r.id)} abbrechen">Abbrechen</button>`;
    return '';
  };
  const rows = list.map((r) => `<tr><td class="main">${esc(r.subj)}</td><td>${r.prio}</td><td class="nowrap">${r.plan ? r.plan : r.start ? `${fmtTime(r.start)} · ${ago(r.start)}` : 'wartet'}</td><td>${esc(r.dur || '–')}</td><td>${esc(r.att)}</td><td>${r.err ? `<span class="err">${esc(r.err)}</span>` : '–'}</td><td class="end"><span class="ta">${act(r)}</span></td></tr>`).join('');
  const bulk = tab === 'failed' && list.length ? `<button type="button" class="btn secondary sm" data-act="aaRunsRetryAll" data-k="${p.k}" data-fid="aa:runs:retryall" aria-label="Alle fehlgeschlagenen Läufe von ${esc(j.name)} erneut versuchen">Alle erneut versuchen</button>${p.k === 'wishes.submit' ? '' : `<button type="button" class="btn dq sm" data-act="aaRunsDelAll" data-k="${p.k}" data-fid="aa:runs:delall" aria-label="Alle fehlgeschlagenen Läufe von ${esc(j.name)} löschen">Alle löschen</button>`}` : '';
  const canAll = (runs.queued.length || runs.running.length) && p.k !== 'wishes.submit';
  return {
    title: `Läufe von ${esc(j.name)}`, wide: true,
    body: `<div class="subtabs" role="group" aria-label="Läufe nach Zustand">${AA_RUN_TABS.map(([k, l]) => `<button type="button" data-act="set" data-path="ui.filter.aaRunTab" data-value="${k}" aria-pressed="${tab === k}" data-fid="aa:runs:tab:${k}">${l}<span class="c ${k === 'failed' && runs[k].length ? 'hot' : ''}">${runs[k].length}</span></button>`).join('')}</div>
      ${list.length ? `<div class="tblw"><table class="tbl"><thead><tr><th>Gegenstand</th><th>Prio</th><th>Start</th><th>Dauer</th><th>Versuche</th><th>Fehler</th><th class="end">Aktionen</th></tr></thead><tbody>${rows}</tbody></table></div>` : '<p class="muted">Keine Läufe in diesem Zustand.</p>'}
      ${bulk ? `<div class="row-gap">${bulk}</div>` : ''}`,
    actions: `${canAll ? `<button type="button" class="btn danger" data-act="aaRunsCancelAll" data-k="${p.k}" data-fid="aa:runs:cancelall">Alle Läufe abbrechen</button>` : ''}<button type="button" class="btn secondary" data-act="close" data-fid="aa:runs:close">Schliessen</button>`,
  };
};
ACT.aaRunsCancel = (d) => {
  const js = aaJobsState();
  aaRunsOf(d.k);
  if (js.running.some((r) => r.id === d.id)) { js.running = js.running.filter((r) => r.id !== d.id); js.last[d.k] = { at: Date.now(), res: 'cancelled', err: '' }; }
  else { const raw = ui.aaRuns[d.k]; const r = raw.queued.find((x) => x.id === d.id); raw.queued = raw.queued.filter((x) => x.id !== d.id); if (r) raw.cancelled.unshift({ ...r, dur: '–' }); js.queued[d.k] = Math.max(0, (js.queued[d.k] || 0) - 1); }
  toast('Lauf abgebrochen.', 'ok', 2500); commit();
};
ACT.aaRunsRetry = (d) => { ACT.aaFailRetry({ id: d.id }); };
ACT.aaRunsDel = (d) => { const js = aaJobsState(); const f = js.failed.find((x) => x.id === d.id); js.failed = js.failed.filter((x) => x.id !== d.id); if (f) { aaRunsOf(d.k); ui.aaRuns[d.k].cancelled.unshift({ id: f.id, subj: f.subj, prio: 3, start: f.at, dur: '–', att: f.att, err: '' }); } toast('Lauf gelöscht.', 'ok', 2500); commit(); };
ACT.aaRunsWishRetry = (d) => { ACT.aaWishRetry({ id: d.id }); };
ACT.aaRunsRetryAll = (d) => {
  if (d.k === 'wishes.submit') S.wishes.filter((w) => w.state === 'failed').map((w) => w.id).forEach((id) => retryWish(id));
  else { const js = aaJobsState(); const mine = js.failed.filter((f) => f.k === d.k); js.failed = js.failed.filter((f) => f.k !== d.k); js.queued[d.k] = (js.queued[d.k] || 0) + mine.length; }
  toast('Alle wieder eingereiht.', 'ok', 2500); commit();
};
ACT.aaRunsDelAll = (d) => { const js = aaJobsState(); js.failed = js.failed.filter((f) => f.k !== d.k); toast('Fehlgeschlagene Läufe gelöscht.', 'ok', 2500); commit(); };
ACT.aaRunsCancelAll = (d) => { const js = aaJobsState(); js.running = js.running.filter((r) => r.k !== d.k); js.queued[d.k] = 0; if (ui.aaRuns) delete ui.aaRuns[d.k]; js.last[d.k] = { at: Date.now(), res: 'cancelled', err: '' }; toast(`Alle Läufe von «${esc(aaJob(d.k).name)}» abgebrochen.`, 'ok'); commit(); };

/* ---------- M-29 Benachrichtigungen ---------- */
route('/admin/notifications', () => {
  const I = aaInt();
  const chans = [
    ['E-Mail', !!I.email.host, I.email.host ? esc(I.email.host) : '', 'email'],
    ['Telegram', !!I.telegram.tail, I.telegram.bot ? '@' + esc(I.telegram.bot) : '', 'telegram'],
    ['Web-Push', I.push.has, I.push.has ? 'Schlüsselpaar vorhanden.' : '', 'push'],
  ];
  const server = `<section class="card"><div class="ch"><span class="ik">${icon('server', 18)}</span><h2>Kanäle des Servers</h2></div>
    <ul class="aa-chanlist">${chans.map(([n, ok, det, k]) => `<li><span class="dot ${ok ? 'ok' : ''}"></span><b>${n}</b><span class="sec">${ok ? 'Eingerichtet' : 'Nicht eingerichtet'}${det ? ' · ' + det : ''}</span><a class="lnk right" href="#/admin/integrations/${k}" data-fid="aa:nt:ch:${k}">Ansehen</a></li>`).join('')}</ul>
    <div class="foot"><a class="lnk" href="#/admin/integrations" data-fid="aa:nt:int">Unter Integrationen einrichten</a></div></section>`;
  const neu = `<section class="card aa-newcard"><div class="ch"><span class="ik">${icon('wish', 18)}</span><h2>Was neu ist ${aaNew()}</h2></div>
    <p>Jeder Kanal kennt drei neue Ereignisse: «${EVENT_LABEL.wish_available}», «${EVENT_LABEL.wish_status}» und, nur für Administratoren, «${EVENT_LABEL.wish_review}»; ab Werk ist «${EVENT_LABEL.wish_available}» an. Bei Profilen mit PIN nennen E-Mail, Telegram und Web-Push den Titel nicht (<a class="lnk" href="#/admin/wishes/settings" data-fid="aa:nt:pin">Wünsche › Einstellungen</a>).</p></section>`;
  const own = typeof channelSettingsHtml === 'function' ? channelSettingsHtml({ admin: true }) : notice('', 'Kanäle deines Kontos', 'Die Einstellungen je Kanal stehen auf der Kanalseite des Kontos.', '<a class="btn secondary" href="#/settings/notifications" data-fid="aa:nt:own">Zu den Benachrichtigungskanälen</a>');
  const dl = S.deliveries.slice(0, 12);
  const prof = (id) => { const p = id ? profile(id) : null; return p ? ` · ${esc(p.name)}` : ''; };
  const table = `<section class="card"><div class="ch"><span class="ik">${icon('send', 18)}</span><h2>Zuletzt zugestellt ${aaNew()}</h2><span class="muted">${S.deliveries.length} Zustellungen</span></div>
    <p class="sec">Was Slouchcat über E-Mail, Telegram und Web-Push verschickt hat. Kommt eine Meldung nicht an, steht hier, ob sie den Server verlassen hat; Fehlschläge von «Mitteilung zustellen» zeigt <a class="lnk" href="#/admin/jobs?group=delivery&amp;job=notify.deliver" data-fid="aa:nt:jobs">Hintergrundjobs</a>.</p>
    ${dl.length ? `<div class="tblw"><table class="tbl"><thead><tr><th>Zeit</th><th>Kanal</th><th>Empfänger</th><th>Betreff</th><th>Zustand</th></tr></thead><tbody>${dl.map((d) => `<tr><td class="nowrap">${fmtTime(d.at)} · ${ago(d.at)}</td><td>${esc(CHANNEL_LABEL[d.channel] || d.channel)}</td><td>${esc(account(d.accountId).email)}${prof(d.profileId)}</td><td class="main">${esc(d.subject)}</td><td><span class="chip ${d.state === 'sent' ? 'ok' : 'bad'}">${d.state === 'sent' ? 'Zugestellt' : 'Fehlgeschlagen'}</span></td></tr>`).join('')}</tbody></table></div>` : '<p class="muted">Noch nichts zugestellt.</p>'}</section>`;
  return { type: 'admin', area: 'automation', page: 'notifications', title: 'Benachrichtigungen', intro: 'Wohin Slouchcat deinem Konto Meldungen schickt: E-Mail, Telegram und Web-Push, je mit eigenen Ereignissen.', html: `<div class="cols2">${server}${neu}</div>${own}${table}` };
});

/* ---------- M-20 Übersicht ---------- */
function aaHints() {
  const wc = wishCounts(), st = seerrState(), I = aaInt(), js = aaJobsState();
  const out = [];
  const failedJobs = js.failed.length;
  if (failedJobs) out.push({ k: 'bad', t: `${failedJobs} ${failedJobs === 1 ? 'Hintergrundjob' : 'Hintergrundjobs'} fehlgeschlagen`, b: js.failed.map((f) => aaJob(f.k).name).filter((v, i, a) => a.indexOf(v) === i).join(', '), href: '#/admin/jobs?group=failures', l: 'Zu den Hintergrundjobs' });
  const paused = Object.keys(js.paused).length;
  if (paused) out.push({ k: 'warn', t: `${paused} ${paused === 1 ? 'Jobart' : 'Jobarten'} pausiert`, b: Object.keys(js.paused).map((k) => aaJob(k).name).join(', '), href: '#/admin/jobs', l: 'Zu den Hintergrundjobs' });
  AA_PROV.filter((p) => I.keys[p.k].rejected).forEach((p) => out.push({ k: 'bad', t: `${p.n}-Schlüssel abgewiesen`, b: 'Der Anbieter weist den Schlüssel ab. Bis ein neuer Schlüssel gilt, holt Slouchcat bei ihm keine Metadaten.', href: '#/admin/integrations/keys', l: 'Zu den Schlüsseln' }));
  const silent = aaHookSilent();
  if (silent.length) out.push({ k: 'warn', t: `Seit sieben Tagen ohne Meldung: ${silent.map((h) => h.name).join(', ')}`, b: 'Prüfe in Radarr oder Sonarr, ob der Webhook noch eingetragen ist.', href: '#/admin/integrations/webhooks', l: 'Webhooks verwalten' });
  if (S.seerr.configured) {
    if (st.k === 'unreachable') out.push({ k: 'bad', isNew: true, t: 'Seerr nicht erreichbar', b: `Wünsche warten als «Wird übermittelt». Letzter erfolgreicher Abgleich ${ago(S.seerr.lastSync)}.`, href: '#/admin/integrations/jellyseerr', l: 'Zu Seerr' });
    if (st.k === 'badkey') out.push({ k: 'bad', isNew: true, t: 'Seerr lehnt den Schlüssel ab', b: 'Seerr antwortet mit 403. Trage einen gültigen API-Schlüssel ein.', href: '#/admin/integrations/jellyseerr', l: 'Zu Seerr' });
    if (st.k === 'csrf') out.push({ k: 'warn', isNew: true, t: 'Seerr blockiert schreibende Aufrufe', b: 'Der CSRF-Schutz in Seerr ist an. In Seerr unter Einstellungen › Netzwerk ausschalten.', href: '#/admin/integrations/jellyseerr', l: 'Zu Seerr' });
    const unmapped = aaNoUserAccounts();
    if (unmapped.length) out.push({ k: 'warn', isNew: true, t: `${unmapped.length} ${unmapped.length === 1 ? 'Konto' : 'Konten'} ohne Seerr-Benutzer`, b: `${unmapped.map((a) => a.email).join(', ')}. Ihre Profile sehen keine fehlenden Titel.${st.k !== 'ok' ? ' Die Konten warten und werden beim nächsten Abgleich eröffnet.' : ' «Alle eröffnen» unter Seerr-Benutzer holt das nach.'}`, href: '#/admin/wishes/mapping', l: 'Zu den Seerr-Benutzern' });
    /* linked at once because the invitee typed an e-mail that Seerr already knew: nobody checked it, so it shows for seven days */
    recentSelfLinks().forEach((a) => {
      const x = accSeerr(a);
      out.push({ k: 'info', isNew: true, t: `Neu verknüpft: ${a.email} mit dem bestehenden Seerr-Benutzer ${x.user.name}`, b: `Beim Annehmen der Einladung gab es in Seerr schon den Benutzer «${x.user.name}» mit dieser E-Mail-Adresse. Slouchcat hat ihn verknüpft${x.pw === 'synced' ? ' und das Passwort übernommen' : ''}. Stimmt das nicht, löse die Verknüpfung.`, href: '#/admin/wishes/mapping', l: 'Zu den Seerr-Benutzern' });
    });
    const gone = Object.values(S.map.accounts).concat(Object.values(S.map.profiles)).map(seerrUser).filter((u) => u && u.gone);
    if (gone.length) out.push({ k: 'warn', isNew: true, t: 'Benutzer in Seerr gelöscht', b: gone.map((u) => u.name).filter((v, i, a) => a.indexOf(v) === i).join(', '), href: '#/admin/wishes/mapping', l: 'Zu den Seerr-Benutzern' });
    S.accounts.forEach((a) => {
      const u = accSeerr(a).user;
      if (!u || u.gone) return;
      const q = quotaFor(u);
      const full = [['movie', 'Filme'], ['tv', 'Staffeln']].filter(([k]) => q[k].restricted);
      if (full.length) out.push({ k: 'warn', isNew: true, t: `Limit erreicht: ${a.email}`, b: full.map(([k, w]) => `${w}: ${q[k].used} von ${q[k].limit} in ${q[k].days} Tagen, der nächste Platz wird ${whenFree(q[k].nextFree)} frei`).join('. ') + '.', href: `#/admin/accounts?open=${a.id}`, l: 'Zum Konto' });
    });
    /* tags that failed, or that have waited for more than 24 hours since the title was handed over */
    const untagged = S.wishes.filter((w) => aaTagLate(w).length);
    if (untagged.length) out.push({ k: 'warn', isNew: true, t: `${untagged.length} ${untagged.length === 1 ? 'Wunsch' : 'Wünsche'} ohne Profil-Tag`, b: `${untagged.map((w) => `«${T[w.titleKey].title}» (${arrOf(w).name}): ${aaTagLate(w).some((x) => x.state === 'fail') ? `${arrOf(w).name} nicht erreichbar` : `Titel noch nicht in ${arrOf(w).name}`}`).join(', ')}. Der Wunsch läuft weiter; der nächste Abgleich versucht es erneut.`, href: '#/admin/wishes?f=notag', l: 'Zu den Anfragen' });
    /* one collective hint; the message of Radarr or Sonarr stays in the expanded row of the request */
    const stalled = S.wishes.filter((w) => { const x = dlInfo(w); return x && x.k === 'stalled'; });
    if (stalled.length) out.push({ k: 'warn', isNew: true, t: `${stalled.length} ${stalled.length === 1 ? 'Download' : 'Downloads'} verzögert`, b: stalled.map((w) => `«${T[w.titleKey].title}» (${arrOf(w).name}): Download verzögert`).join(', ') + '.', href: '#/admin/wishes?f=stalled', l: 'Zu den Anfragen' });
  }
  if (wc.pending) out.push({ k: 'warn', isNew: true, t: `${wc.pending} ${wc.pending === 1 ? 'Wunsch wartet' : 'Wünsche warten'} auf Freigabe`, b: S.wishes.filter((w) => w.state === 'pending').map((w) => `«${T[w.titleKey].title}»`).join(', '), href: '#/admin/wishes?state=pending', l: 'Zu den Anfragen' });
  if (wc.problems) out.push({ k: 'bad', isNew: true, t: `${wc.problems} ${wc.problems === 1 ? 'Wunsch' : 'Wünsche'} fehlgeschlagen`, b: S.wishes.filter((w) => w.state === 'failed').map((w) => `«${T[w.titleKey].title}»: ${FAIL_TEXT[w.failReason] || ''}`).join(' '), href: '#/admin/wishes?state=failed', l: 'Zu den Anfragen' });
  if (wc.queued) out.push({ k: 'warn', isNew: true, t: `${wc.queued} ${wc.queued === 1 ? 'Wunsch wartet' : 'Wünsche warten'} auf Übermittlung`, b: S.wishes.filter((w) => w.state === 'queued').map((w) => `«${T[w.titleKey].title}»`).join(', ') + '. Slouchcat versucht es weiter, bis 24 Stunden.', href: '#/admin/wishes?state=queued', l: 'Zu den Anfragen' });
  return out;
}
route('/admin', () => {
  const wc = wishCounts(), js = aaJobsState(), I = aaInt(), c = aaIntCounts();
  const queued = AA_JOBS.reduce((n, j) => n + aaQueued(j.k), 0);
  const pausedN = Object.keys(js.paused).length;
  const showWishes = S.seerr.configured;
  const stats = `<section class="stats" style="--n:${showWishes ? 5 : 4}" aria-label="Kennzahlen">
    <a class="stat" href="${adminHref('streams')}" data-fid="aa:ov:st:streams"><span class="v">1</span><span class="l">Aktive Streams</span><span class="n">Gerade verbunden</span></a>
    <a class="stat" href="#/admin/jobs" data-fid="aa:ov:st:jobs"><span class="v">${js.running.length}</span><span class="l">Laufende Jobs</span><span class="n">${queued} wartend${pausedN ? ` · ${pausedN} pausiert` : ''}</span></a>
    <a class="stat warn" href="${adminHref('maintenance')}" data-fid="aa:ov:st:maint"><span class="v">268</span><span class="l">Titel zur Pflege</span><span class="n">Offene Fälle in der Pflege</span></a>
    <a class="stat" href="${adminHref('server')}" data-fid="aa:ov:st:storage"><span class="v">34 % · 61 %</span><span class="l">Speicherplatz</span><span class="n">Belegt: Konfiguration · Cache</span></a>
    ${showWishes ? `<a class="stat aa-new ${wc.pending ? 'warn' : ''}" href="#/admin/wishes" data-fid="aa:ov:st:wishes"><span class="v">${seerrState().k === 'ok' || wc.open ? wc.open : '–'}</span><span class="l">Offene Wünsche ${aaNew()}</span><span class="n">${wc.pending} ${wc.pending === 1 ? 'wartet' : 'warten'} auf Freigabe</span></a>` : ''}
  </section>`;
  const hints = aaHints();
  const att = `<section class="card" aria-labelledby="aa-att-h"><div class="ch"><span class="ik">${icon('alert', 18)}</span><h2 id="aa-att-h">Was benötigt Aufmerksamkeit?</h2><span class="muted">${hints.length ? `${hints.length} ${hints.length === 1 ? 'Hinweis' : 'Hinweise'}` : ''}</span></div>
    ${hints.length ? `<ul class="aa-hints">${hints.map((h, i) => `<li class="${h.k}"><span class="ic">${icon(h.k === 'bad' ? 'alert' : 'info', 18)}</span><div class="grow"><b>${esc(h.t)}${h.isNew ? ' ' + aaNew() : ''}</b>${h.b ? `<span>${esc(h.b)}</span>` : ''}</div><a class="btn secondary sm" href="${h.href.includes('?state=') ? h.href.split('?state=')[0] : h.href}" ${h.href.includes('?state=') ? `data-act="aaGoWishes" data-f="${h.href.split('?state=')[1]}"` : ''} data-fid="aa:ov:hint:${i}" aria-label="${esc(h.l)}: ${esc(h.t)}">Ansehen</a></li>`).join('')}</ul>` : `<p class="aa-allok">${icon('check', 18)}Alles in Ordnung.</p>`}</section>`;
  const runRows = js.running.map((r) => { const pct = Math.round((r.done / r.total) * 100); return `<li><div class="grow"><b>${esc(aaJob(r.k).name)}</b><span class="sec aa-small">${esc(r.subj)}</span></div><span class="aa-bar" role="progressbar" aria-valuenow="${pct}" aria-valuemin="0" aria-valuemax="100" aria-label="Fortschritt ${esc(aaJob(r.k).name)}"><i style="width:${pct}%"></i></span></li>`; }).join('');
  const active = `<section class="card" aria-labelledby="aa-act-h"><div class="ch"><span class="ik">${icon('zap', 18)}</span><h2 id="aa-act-h">Gerade aktiv</h2></div>
    <h3 class="aa-h3">Aufgaben</h3>${runRows ? `<ul class="aa-active" aria-label="Laufende Aufgaben">${runRows}</ul>` : '<p class="muted">Gerade läuft keine Aufgabe.</p>'}
    <h3 class="aa-h3">Jetzt läuft</h3><ul class="aa-active" aria-label="Jetzt läuft"><li><div class="grow"><b>Godzilla Minus One</b><span class="sec aa-small">Oma &amp; Opa · Wohnzimmer-TV · Direktwiedergabe</span></div><span class="aa-bar" role="progressbar" aria-valuenow="42" aria-valuemin="0" aria-valuemax="100" aria-label="Fortschritt Godzilla Minus One"><i style="width:42%"></i></span></li></ul>
    <div class="foot"><a class="lnk" href="#/admin/jobs" data-fid="aa:ov:jobs">Hintergrundjobs</a>${js.failed.length ? '<a class="lnk" href="#/admin/jobs?group=failures" data-fid="aa:ov:failed">Fehlschläge ansehen</a>' : ''}<a class="lnk" href="${adminHref('streams')}" data-fid="aa:ov:streams">Alle Streams</a></div></section>`;
  const quick = `<section class="stack" aria-labelledby="aa-quick-h"><h2 id="aa-quick-h" class="aa-h2">Direkt zu deinen Aufgaben</h2><ul class="aa-quick">
    ${showWishes ? `<li><a class="aa-lc new" href="#/admin/wishes" data-fid="aa:ov:q:wishes"><span class="ik">${icon('wish', 18)}</span><span class="grow"><b>Wünsche ${aaNew()}</b><span>${wc.open} offen · ${wc.pending} ${wc.pending === 1 ? 'wartet' : 'warten'} auf Freigabe · Seerr-Benutzer · Einstellungen</span></span>${icon('chevR', 16)}</a></li>` : ''}
    ${AREAS.filter((a) => a.pages.length).map((a) => `<li><a class="aa-lc" href="${areaHref(a)}" data-fid="aa:ov:q:${a.key}"><span class="ik">${icon(a.icon, 18)}</span><span class="grow"><b>${a.label}</b><span>${a.pages.map((p) => p[1]).join(', ')}</span></span>${icon('chevR', 16)}</a></li>`).join('')}
  </ul></section>`;
  const silent = aaHookSilent();
  const ev24 = I.webhooks.reduce((n, h) => n + h.log.filter((l) => Date.now() - l.at < D).length, 0);
  const st = seerrState();
  const server = `<section class="card"><div class="ch"><span class="ik">${icon('server', 18)}</span><h2>Zustand des Servers</h2></div><dl class="kv">
      <dt>Version</dt><dd>1.40.1</dd><dt>Läuft seit</dt><dd>3 Tagen, 4 Stunden</dd><dt>Updates</dt><dd>${I.server.update ? 'Aktuell' : 'Prüfung ausgeschaltet'}</dd>
      <dt>Integrationen</dt><dd>${c.set} von ${c.total} eingerichtet</dd>
      <dt>Seerr ${aaNew()}</dt><dd><span class="dot ${st.dot}"></span> ${st.label}</dd>
      <dt>Webhooks</dt><dd>${I.webhooks.length ? `${ev24} ${ev24 === 1 ? 'Meldung' : 'Meldungen'} im Protokoll der letzten 24 Stunden, von ${I.webhooks.length} Integrationen.${silent.length ? ` Seit sieben Tagen ohne Meldung: ${silent.map((h) => esc(h.name)).join(', ')}` : ''}` : 'Kein Webhook eingerichtet.'}</dd>
      <dt>Fehler im Protokoll</dt><dd>Keine in den letzten 24 Stunden</dd></dl>
    <div class="foot"><a class="lnk" href="${adminHref('server')}" data-fid="aa:ov:log">Server-Protokoll ansehen</a><a class="lnk" href="#/admin/integrations" data-fid="aa:ov:int">Integrationen einrichten</a><a class="lnk" href="#/admin/integrations/webhooks" data-fid="aa:ov:hooks">Webhooks verwalten</a></div></section>`;
  const now = Date.now();
  const stat = [{ at: now - 40 * 60e3, t: 'Scan der Bibliothek «Filme» abgeschlossen: 2 neue Titel' }, { at: now - 5 * H, t: 'Konto grosseltern@example.ch hat sich angemeldet' }, { at: now - 30 * H, t: 'Sicherung erstellt (412 MB)' }, { at: now - 2 * D - H, t: 'Update-Prüfung: Version 1.40.1 ist aktuell' }];
  const wev = S.wishes.flatMap((w) => w.events.map((e) => ({ at: e.at, t: `«${T[w.titleKey].title}»: ${e.t}`, wish: true })));
  const feed = [...stat, ...wev].sort((a, b) => b.at - a.at).slice(0, 8);
  const activity = `<section class="card"><div class="ch"><span class="ik">${icon('list', 18)}</span><h2>Letzte Aktivität</h2></div>
    <ul class="feed">${feed.map((f) => `<li><time datetime="${new Date(f.at).toISOString()}">${ago(f.at)}</time><span class="grow">${f.wish ? '<span class="chip info">Wunsch</span> ' : ''}${esc(f.t)}</span></li>`).join('')}</ul>
    <div class="foot"><a class="lnk" href="${adminHref('server')}" data-fid="aa:ov:details2">Serverdetails</a></div></section>`;
  const summary = `<section class="stack" aria-labelledby="aa-sum-h"><div class="row-gap"><h2 id="aa-sum-h" class="aa-h2 grow">Server &amp; letzte Aktivitäten</h2><a class="lnk" href="${adminHref('server')}" data-fid="aa:ov:details">Serverdetails</a></div><div class="cols2">${server}${activity}</div></section>`;
  return { type: 'admin', area: 'overview', eyebrow: 'Übersicht', title: 'Übersicht', h1: 'Alles im Blick', crumb: 'Übersicht', intro: 'Wichtiges zuerst. Alles Weitere erreichst du über die Bereiche.',
    actions: `<button type="button" class="btn secondary" data-act="aaRefresh" data-fid="aa:ov:refresh">${icon('refresh', 16)}Aktualisieren</button>`,
    html: `${stats}<div class="cols2">${att}${active}</div>${quick}${summary}` };
});
/* links to Wünsche › Anfragen preset its filter (admin-b keeps it in ui.filter.adminWishes) */
ACT.aaGoWishes = (d) => { ui.filter.adminWishes = d.f; };
ACT.aaRefresh = () => { render(); toast('Übersicht aktualisiert.', 'ok', 2200); };
