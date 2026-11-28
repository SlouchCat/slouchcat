/* Web Verwaltung A, Teil 1: gemeinsame Helfer, LA-01 Live-TV-Übersicht, LA-03 Quelle anlegen und ändern,
   LA-04 Import-Vorschau und -Ergebnis. Vorlage: page-la02.js. Bauvorlage: 15-administration.md 3.1, 3.3, 3.4.
   Globale Namen mit Präfix «aa». Die Seiten ändern die Beispieldaten (LIVE.*) direkt und rufen render(). */
'use strict';

const AA = {
  sel: {}, selAll: false, srcFor: null, focusStep: false, dupHandled: 0,
  chVisited: false, test: {},
};

/* Beispieldaten der Verwaltung (neue und doppelte Sender, Zuordnungen): in shared/live-data.js */

/* ---------- gemeinsame Helfer ---------- */
const aaMode = () => SIM.mode();
const aaEmpty = () => aaMode() === 'empty';
const aaSources = () => (aaEmpty() ? [] : LIVE.sources);
const aaClone = (o) => JSON.parse(JSON.stringify(o));
const aaWhen = (t) => { const r = fmt.dayRel(t); return (['Heute', 'Morgen', 'Gestern'].includes(r) ? r.toLowerCase() : r) + ' ' + fmt.time(t); };
const aaPl = (n, one, many) => `${fmt.num(n)} ${n === 1 ? one : many}`;
/** Seitenzustände: lädt (Skeleton in Endmass), Fehler (mit «Erneut versuchen»), leer (eigener Text), sonst der Inhalt */
function aaGate(emptyHtml, build, o) {
  const m = aaMode();
  if (m === 'loading') return skelRows((o && o.rows) || 5, (o && o.h) || 72);
  if (m === 'error') return liveError((o && o.err) || 'Die Seite konnte nicht geladen werden.');
  if (m === 'empty' && emptyHtml) return emptyHtml;
  return build();
}
const aaView = (o) => Object.assign({ type: 'admin' }, o);
const aaEmptyBox = (title, text, acts) => `<div class="unchanged aa-empty"><b>${title}</b><p>${text}</p>${acts ? `<div class="row-gap">${acts}</div>` : ''}</div>`;
const aaMeter = (v, max, label) => { const p = Math.max(0, Math.min(100, (v / max) * 100)); return `<span class="aa-meter${p >= 90 ? ' bad' : p >= 75 ? ' warn' : ''}" role="meter" aria-valuemin="0" aria-valuemax="${max}" aria-valuenow="${v}" aria-label="${esc(label)}"><i style="width:${p.toFixed(1)}%"></i></span>`; };
const aaStat = (label, value, note, o) => { o = o || {}; const inner = `<span class="l">${label}</span><span class="v">${value}</span><span class="n">${note || ''}</span>`; return o.href ? `<a class="stat ${o.cls || ''}" href="${o.href}" data-fid="${o.fid}">${inner}</a>` : `<div class="stat ${o.cls || ''}">${inner}</div>`; };
const aaDot = (k) => `<span class="dot ${k}" aria-hidden="true"></span>`;
const aaVerantwortung = (id) => band(id, 'Verantwortung für Quellen', 'Nutzungsrechte · keine Verbindungszählung', '<p>Du bist dafür verantwortlich, dass du die eingetragenen Quellen nutzen und im Haushalt verteilen darfst. Slouchcat prüft keine Nutzungsrechte.</p><p>Slouchcat zählt keine Verbindungen je Anbieter und kennt keine Abos. Ob dein Anbieter mehrere Verbindungen zugleich zulässt, verantwortest du.</p>', { icon: 'info' });
const aaGroupName = (id) => (LIVE.groups.find((g) => g.id === id) || { name: id }).name;
const aaSrcName = (id) => (LIVE.sources.find((s) => s.id === id) || { name: 'Quelle entfernt' }).name;
/* Formularbausteine: Bezeichnung, Feld, Fehler */
const aaF = (id, label, inner, o) => { o = o || {}; return `<div class="fld${o.cls ? ' ' + o.cls : ''}"><label for="${id}">${label}</label>${inner}${o.hint ? `<span class="hint">${o.hint}</span>` : ''}${o.err ? `<span class="aa-err" id="${id}-e" role="alert">${icon('alert', 14)}${esc(o.err)}</span>` : ''}</div>`; };
const aaI = (path, id, o) => { o = o || {}; return `<input class="inp${o.num ? ' num' : ''}" id="${id}" type="${o.type || 'text'}" data-bind="${path}" data-fid="${id}" value="${esc(getPath(path) ?? '')}"${o.num ? ' data-num inputmode="numeric"' : ''}${o.ph ? ` placeholder="${esc(o.ph)}"` : ''}${o.err ? ` aria-invalid="true" aria-describedby="${id}-e"` : ''}${o.live === false ? '' : ' data-live'}${o.auto ? ' data-autofocus' : ''}${o.enter ? ` data-enter="${o.enter}"` : ''}${o.dis ? ' disabled' : ''}${o.rerender ? ' data-rerender' : ''} autocomplete="off" spellcheck="false">`; };
const aaT = (path, id, o) => { o = o || {}; return `<textarea class="inp aa-ta" id="${id}" rows="${o.rows || 3}" data-bind="${path}" data-fid="${id}"${o.err ? ` aria-invalid="true" aria-describedby="${id}-e"` : ''}${o.ph ? ` placeholder="${esc(o.ph)}"` : ''} spellcheck="false" data-live>${esc(getPath(path) ?? '')}</textarea>`; };
const aaRadio = (path, name, v, text, fid, o) => `<label class="check"><input type="radio" name="${name}" value="${esc(v)}" data-bind="${path}" ${String(getPath(path)) === String(v) ? 'checked' : ''} data-fid="${fid}"${o && o.after ? ` data-after="${o.after}"` : ''}${o && o.dis ? ' disabled' : ''}><span>${text}</span></label>`;
const aaMask = (u) => String(u).replace(/\/\/[^/@]*@/, '//••••@').replace(/([?&][^=&]+=)[^&]*/g, '$1••••');

/* ---------- Zahlen aus dem simulierten Zustand: im Kern (web/js/admin-core.js: admCounts, admViewers, admRecRunning, admRecSoon) ---------- */
function aaState() {
  const c = admCounts();
  if (!aaSources().length) return ['Keine Quelle', 'bad', 'Füge eine Quelle hinzu'];
  if (SIM.flags().noEpg || LIVE.epgSources.every((e) => SIM.now() - e.last > 48 * 60)) return ['Programmdaten veraltet', 'warn', 'Programmdaten laden prüfen'];
  if (c.unavailable > 0) return ['Eingeschränkt', 'warn', `${aaPl(c.unavailable, 'Sender', 'Sender')} nicht verfügbar`];
  return ['Bereit', 'ok', 'alle Sender verfügbar'];
}
/** «Braucht Aufmerksamkeit»: Fehler zuerst, dann Warnungen, dann Hinweise */
function aaAttention() {
  if (aaEmpty()) return [];
  const L = [];
  LIVE.inputs.filter((n) => n.state === 'reconnecting').forEach((n) => L.push({ k: 'bad', t: n.since, text: `${SIM.channel(n.ch).name} ist seit ${fmt.time(n.since)} nicht erreichbar.`, href: `#/admin/live/diagnostics?channel=${n.ch}`, fid: 'att:' + n.ch }));
  if (SIM.flags().disturbed) L.push({ k: 'bad', t: SIM.now(), text: `SRF 1 ist seit ${fmt.time(SIM.now())} gestört.`, href: '#/admin/live/diagnostics?channel=srf1', fid: 'att:srf1' });
  /* Speicherorte (Mockup-Zustand «Verwaltung»: Speicher knapp, voll, Netzlaufwerk nicht eingehängt), Zustand wie in LA-16 */
  if (typeof abStoStatus === 'function') LIVE.storage.forEach((s) => {
    const st = abStoStatus(s); if (st === 'ok') return; const free = s.id === 'timeshift' ? s.limitGB - s.usedGB : s.totalGB - s.usedGB;
    const text = st === 'unmounted' ? `Das Netzlaufwerk für ${esc(s.name)} (${esc(s.path)}) ist nicht eingehängt. Neue Aufnahmen werden nicht angenommen, laufende bekommen eine Lücke.`
      : st === 'readonly' ? `Der Speicherort ${esc(s.name)} (${esc(s.path)}) ist nicht beschreibbar.`
        : st === 'full' ? `Speicher für ${esc(s.name)} ist voll (${fmt.gb(s.usedGB)} von ${fmt.gb(s.id === 'timeshift' ? s.limitGB : s.totalGB)}). ${s.id === 'recordings' ? 'Slouchcat nimmt nicht mehr auf und löscht nichts von sich aus.' : 'Slouchcat kürzt die ältesten Puffer.'}`
          : `Speicher für ${esc(s.name)} wird knapp: noch ${fmt.gb(free)} frei.`;
    L.push({ k: st === 'low' ? 'warn' : 'bad', t: SIM.now() - 1, text, href: '#/admin/live/storage?tab=storage', fid: 'att:sto:' + s.id });
  });
  /* EPG-Import fehlgeschlagen (Mockup-Zustand «EPG-Import fehlgeschlagen» oder echter Fehler einer EPG-Quelle) */
  LIVE.epgSources.filter((e) => e.lastOk === false).forEach((e) => L.push({ k: 'warn', t: e.last, text: `Programmdaten «${esc(e.name)}» konnten nicht geladen werden (${esc(e.lastError || 'unbekannter Fehler')}). Die bisherigen Programmdaten bleiben.`, href: '#/admin/live/epg?tab=sources', fid: 'att:epgsrc:' + e.id }));
  LIVE.sources.filter((s) => !s.lastOk).forEach((s) => L.push({ k: 'warn', t: s.last, text: `Import «${esc(s.name)}» fehlgeschlagen (${esc(s.lastError)}). Die bisherige Liste bleibt aktiv.`, href: `#/admin/live/sources/${s.id}/import/${(LIVE.importRuns[s.id] || [[]])[0][0]}`, fid: 'att:src:' + s.id }));
  /* zurückgestellte Eingänge (15 Fassung 2, 3.1 und 3.9): nur hier, keine Meldung */
  LIVE.inputs.filter((n) => n.state === 'deferred').forEach((n) => { const c = SIM.channel(n.ch); L.push({ k: 'warn', t: n.since, text: `${esc(c.name)} (${aaRecName(c.reception)}) ist seit ${fmt.time(n.since)} zurückgestellt: Der Platz wurde für einen Zuschauer gebraucht. Slouchcat startet ihn wieder, sobald 60 Sekunden lang Platz frei ist.`, href: '#/admin/live/streams', fid: 'att:def:' + n.ch }); });
  LIVE.accounts.filter((a) => a.quotaGB && a.usedGB / a.quotaGB >= 0.9).forEach((a) => L.push({ k: 'warn', t: SIM.now() - 60, text: `Konto ${esc(a.email)}: ${fmt.num(a.usedGB)} von ${fmt.num(a.quotaGB)} GB für Aufnahmen belegt.`, href: '#/admin/live/storage?tab=storage', fid: 'att:acc:' + a.email }));
  if (LIVE.mappingCounts.missing) L.push({ k: 'warn', t: SIM.now() - 120, text: `${LIVE.mappingCounts.missing} aktive Sender ohne Programmdaten.`, href: '#/admin/live/epg?tab=mapping&filter=missing', fid: 'att:epg' });
  LIVE.goneChannels.forEach((c) => { const n = LIVE.profiles.reduce((a, p) => a + SIM.rules(p.id).filter((r) => r.ch === c.id).length, 0); L.push({ k: 'info', t: c.since, text: `${esc(c.name)} ist aus «${esc(aaSrcName(c.sources[0].source))}» verschwunden.${n ? ` ${aaPl(n, 'geplante Aufnahme', 'geplante Aufnahmen')} betroffen.` : ''}`, href: '#/admin/live/channels?tab=channels&state=gone', fid: 'att:gone:' + c.id }); });
  return L.sort((a, b) => ['bad', 'warn', 'info'].indexOf(a.k) - ['bad', 'warn', 'info'].indexOf(b.k) || b.t - a.t);
}
const aaRecName = (k) => ({ permanent: 'dauerhaft', schedule: 'nach Zeitplan' })[k] || 'Standard';
/** laufende Aufnahmen je Sender, nur als Zahl (LA-01, LA-12, LA-14 rechnen alle damit): eigene aus der Simulation, fremde aus den Beispieldaten */
function aaRecsOn(ch) {
  return LIVE.profiles.reduce((n, p) => n + SIM.recordings(p.id).filter((r) => r.state === 'running' && r.ch === ch).length, 0)
    + LIVE.consumers.filter((c) => c[1] === 'recording' && c[0] === ch && c[2] !== LIVE.account.email).length;
}
/** laufende Aufnahmen insgesamt und auf wie vielen Sendern; run ist dieselbe Zahl wie admRecRunning() */
function aaRecStats() {
  if (aaEmpty()) return { run: 0, chs: 0 };
  const chs = new Set(LIVE.profiles.flatMap((p) => SIM.recordings(p.id).filter((r) => r.state === 'running').map((r) => r.ch)).concat(LIVE.consumers.filter((c) => c[1] === 'recording' && c[2] !== LIVE.account.email).map((c) => c[0])));
  return { run: admRecRunning(), chs: chs.size };
}
/** Eingänge wie in LA-01 und LA-12: «6 von 8»; zurückgestellte belegen keinen Platz */
function aaInputStats() {
  const inp = admInputs(); const live = inp.filter((n) => n.state !== 'deferred'); const has = (k) => live.filter((n) => n.reasons.includes(k)).length;
  return { inp, live, n: live.length, max: LIVE.limits.inputs, perm: has('permanent'), pre: has('preload'), sch: has('schedule'), def: inp.length - live.length };
}
const aaAttRow = (a) => `<li class="aa-att ${a.k}">${icon(a.k === 'info' ? 'info' : 'alert', 18)}<span><a class="lnk" href="${a.href}" data-fid="${a.fid}">${a.text}</a><small>${fmt.ago(a.t)}</small></span></li>`;

/* ======================================================================================
   LA-01 Live-TV-Übersicht
   ====================================================================================== */
const AA_REASON = { viewer: 'Zuschauer', recording: 'Aufnahme', permanent: 'Permanent', preload: 'Vorgeladen', schedule: 'Zeitplan' };
function aaSetupBand() {
  const src = aaSources().length > 0; const epg = LIVE.epgSources.length > 0 && !aaEmpty(); const acc = LIVE.accounts.some((a) => a.rights.liveTv) && !aaEmpty();
  const steps = [['Quelle hinzufügen', '#/admin/live/sources/new', src], ['Sender prüfen', '#/admin/live/channels?tab=channels', src && AA.chVisited], ['Programmdaten hinzufügen', '#/admin/live/epg?tab=sources', epg], ['Konten freischalten', '#/admin/accounts', acc]];
  return `<section class="card aa-setup"><div class="ch"><span class="ik">${icon('live', 18)}</span><h2>Live-TV einrichten</h2></div>
    <p class="muted">Live-TV ist freiwillig. Die Schritte kannst du in dieser Reihenfolge erledigen; erledigte sind abgehakt.</p>
    <ol class="aa-steps">${steps.map(([t, h, done], i) => `<li class="${done ? 'done' : ''}"><span class="n">${done ? icon('check', 14) : i + 1}</span><a class="lnk" href="${h}" data-fid="la01:step${i}">${t}</a>${done ? '<span class="vh"> (erledigt)</span>' : ''}</li>`).join('')}</ol></section>`;
}
function aaOverview() {
  const empty = !aaSources().length; const c = admCounts(); const st = aaState(); const I = aaInputStats(); const inp = I.inp; const R = LIVE.resources; const vs = admViewers();
  const trans = vs.filter((v) => v.tr).length; const rs = aaRecStats(); const run = rs.run; const bandOut = Math.round(vs.reduce((a, v) => a + v.br, 0) * 10) / 10;
  const dash = '–';
  const stats = `<div class="stats" style="--n:3">${aaStat('Zustand', empty ? 'Keine Quelle' : st[0], empty ? 'Füge eine Quelle hinzu' : st[2], { cls: empty ? 'bad' : st[1], href: '#/admin/live/diagnostics', fid: 'la01:s-state' })}
    ${aaStat('Sender', empty ? dash : fmt.num(c.active) + ' aktiv', empty ? '' : `${fmt.num(c.available)} verfügbar · ${fmt.num(c.unavailable)} nicht verfügbar`, { href: '#/admin/live/channels?tab=channels', fid: 'la01:s-ch', cls: c.unavailable ? '' : '' })}
    ${aaStat('Eingänge', empty ? dash : `${I.n} von ${I.max}`, empty ? '' : `davon ${I.perm} dauerhaft · ${I.pre} vorgeladen · ${I.sch} nach Zeitplan · ${I.def} zurückgestellt`, { href: '#/admin/live/streams', fid: 'la01:s-in' })}
    ${aaStat('Zuschauer', empty ? dash : vs.length, empty ? '' : `Umwandlungen: ${trans}`, { href: '#/admin/live/streams', fid: 'la01:s-vi' })}
    ${aaStat('Aufnahmen', empty ? dash : run + ' laufen', empty ? '' : `${R.recPlanned24h} geplant (nächste 24 h)`, { href: '#/admin/live/storage?tab=dvr', fid: 'la01:s-rec' })}
    ${aaStat('Bandbreite', empty ? dash : `ein ${fmt.dec(R.bandIn, 1, 1)} Mbit/s`, empty ? '' : `aus ${fmt.dec(bandOut, 1, 1)} Mbit/s`, { href: '#/admin/live/streams', fid: 'la01:s-bw' })}</div>`;
  const att = aaAttention();
  const cardAtt = `<section class="card"><div class="ch"><span class="ik">${icon('alert', 18)}</span><h2>Braucht Aufmerksamkeit</h2></div>${att.length ? `<ul class="aa-atts">${att.map(aaAttRow).join('')}</ul>` : `<p class="muted">${empty ? 'Noch nichts zu prüfen.' : 'Alles in Ordnung. Nichts braucht Aufmerksamkeit.'}</p>`}</section>`;
  const top = I.live.map((n) => Object.assign({}, n, { v: admViewersOf(n.ch), recordings: aaRecsOn(n.ch) })).sort((a, b) => b.v + b.recordings - (a.v + a.recordings)).slice(0, 5);
  const cardIn = `<section class="card"><div class="ch"><span class="ik">${icon('play', 18)}</span><h2>Aktive Eingänge</h2><a class="btn quiet sm" href="#/admin/live/streams" data-fid="la01:all-in">Alle ansehen</a></div>
    ${top.length ? `<div class="tblw aa-tbl-in"><table class="tbl"><thead><tr><th>Sender</th><th>Grund</th><th class="end">Zuschauer</th><th class="end">Aufnahmen</th><th class="end">Bitrate</th></tr></thead><tbody>${top.map((n) => { const c0 = SIM.channel(n.ch); return `<tr><td class="main"><span class="aa-chn">${LU.logo(c0)}<a class="lnk" href="#/admin/live/streams" data-fid="la01:in:${n.ch}">${esc(c0.name)}</a></span></td><td data-l="Grund">${n.state === 'reconnecting' ? '<span class="chip bad">Gestört</span>' : n.reasons.map((r) => `<span class="chip info">${AA_REASON[r]}${r === 'schedule' ? ` «${esc(n.schedule)}»` : ''}</span>`).join(' ')}</td><td class="end" data-l="Zuschauer">${n.v}</td><td class="end" data-l="Aufnahmen">${n.recordings}</td><td class="end" data-l="Bitrate">${n.bitrate ? fmt.dec(n.bitrate, 1, 1) + ' Mbit/s' : '–'}</td></tr>`; }).join('')}</tbody></table></div>` : '<p class="muted">Gerade ist kein Sender eingeschaltet.</p>'}</section>`;
  const chs = rs.chs;
  const cardRec = `<section class="card"><div class="ch"><span class="ik">${icon('record', 18)}</span><h2>Aufnahmen</h2></div>
    <p>${empty ? 'Noch keine Aufnahmen.' : `<b>${aaPl(run, 'läuft', 'laufen')}</b> auf ${aaPl(chs, 'Sender', 'Sendern')} · <b>${R.recPlanned24h}</b> geplant in 24 Std. · höchstens ${R.recMaxParallel} gleichzeitig`}</p>
    <p class="muted">Hier erscheinen nur Zahlen. Titel, Konten und Profile der Aufnahmen siehst du als Administrator nie (Privatsphäre).</p></section>`;
  const sto = LIVE.storage; const cap = (s) => (s.id === 'timeshift' ? s.limitGB : s.totalGB);
  const meter = (l, v, m, fmtv) => `<div class="aa-res"><span>${l}</span><b>${fmtv}</b>${aaMeter(v, m, l)}</div>`;
  const cardRes = `<section class="card"><div class="ch"><span class="ik">${icon('server', 18)}</span><h2>Ressourcen</h2></div>
    ${empty ? '<p class="muted">Ohne Quelle läuft nichts.</p>' : `${meter('CPU', R.cpu, 100, R.cpu + ' %')}${meter('GPU', R.gpu, 100, R.gpu + ' %')}${meter('RAM von Slouchcat', R.ramGB, 8, fmt.dec(R.ramGB, 1, 1) + ' GB')}
    <h3 class="la02-h3">Speicher</h3>${sto.map((s) => meter(esc(s.name), s.usedGB, cap(s), `${fmt.gb(s.usedGB)} von ${fmt.gb(cap(s))}`)).join('')}
    <p class="muted aa-small">Wo der Server einen Wert nicht lesen kann, steht «–» und «Nicht verfügbar auf diesem System».</p>`}</section>`;
  const eps = LIVE.epgSources; const last = Math.max(...eps.map((e) => e.last)); const until = Math.max(...eps.map((e) => e.until)); const warns = eps.reduce((n, e) => n + e.warnings.length, 0);
  const cardEpg = `<section class="card"><div class="ch"><span class="ik">${icon('calendar', 18)}</span><h2>Programmdaten</h2><a class="btn quiet sm" href="#/admin/live/epg?tab=sources" data-fid="la01:epg">Öffnen</a></div>
    <p>${aaEmpty() ? 'Noch keine Programmdaten.' : SIM.flags().noEpg ? '<b>Keine Programmdaten</b> (Mockup-Zustand «Ohne EPG»). Das Fernsehen läuft trotzdem.' : `Stand ${aaWhen(last)} · reicht bis ${fmt.day(until)} · ${aaPl(eps.length, 'Quelle', 'Quellen')} · ${aaPl(warns, 'Warnung', 'Warnungen')}`}</p></section>`;
  return `${empty ? aaSetupBand() : ''}${stats}<div class="cols2">${cardAtt}<div class="stack">${cardRec}${cardEpg}</div></div>${cardIn}${cardRes}${aaVerantwortung('la01:resp')}`;
}
route('/admin/live', () => {
  const m = aaMode();
  return aaView({ area: 'livetv', page: 'live', title: 'Live-TV', intro: 'Zustand von Quellen, Sendern, Empfang und Aufnahmen auf einen Blick.', live: m === 'normal' ? admLiveText() : null,
    actions: `<a class="btn secondary" href="#/admin/live/channels?tab=channels" data-fid="la01:ch">Sender verwalten</a><a class="btn primary" href="#/admin/live/sources/new" data-fid="la01:add">${icon('plus', 16)}Quelle hinzufügen</a>`,
    html: aaGate(null, aaOverview, { rows: 4, h: 96 }) });
});

/* ======================================================================================
   LA-03 Quelle anlegen und ändern (ein Formular, neu in Schritten, ändern als Aufklapper)
   ====================================================================================== */
const AA_KINDS = [
  ['m3u-url', 'Senderliste als Adresse (M3U/M3U8-URL)', 'Der Anbieter gibt dir eine Adresse zu einer Senderliste.', 'link'],
  ['m3u-file', 'Senderliste als Datei (M3U/M3U8)', 'Du hast eine Datei mit Sendern.', 'upload'],
  ['xtream', 'Xtream-Codes-Zugang', 'Der Anbieter gibt dir Server, Benutzername und Passwort. Übernommen wird nur Live-TV, keine Filme und Serien.', 'key'],
  ['single', 'Einzelner Stream', 'Eine Adresse für einen Sender: HTTP(S), HLS, MPEG-DASH ohne Kopierschutz, MPEG-TS, RTSP, RTMP, UDP, RTP, Multicast.', 'live'],
];
const AA_STEPS = ['Art', 'Adresse und Zugang', 'Testen', 'Vorschau', 'Einstellungen', 'Übernehmen'];
const AA_OUT = [['auto', 'Automatisch aus den Eingaben'], ['ok', 'Erfolg'], ['auth', 'Zugangsdaten falsch'], ['unreach', 'Nicht erreichbar'], ['format', 'Keine Senderliste'], ['hls', 'Einzelner Stream statt Liste'], ['drm', 'Kopierschutz (DRM)'], ['local', 'Ziel im Heimnetz nicht freigegeben']];
const AA_V3 = 'Mit dieser Freigabe darf Slouchcat für diese Quelle genau diese Ziele in deinem Heimnetz aufrufen. Gib nur Geräte frei, die du kennst. Andere Quellen und andere Teile von Slouchcat bleiben vom Heimnetz getrennt.';
const AA_PRELOAD_HINT = 'Vorladen öffnet zusätzliche Verbindungen zu dieser Quelle. Ob dein Anbieter das zulässt, verantwortest du.';
const AA_PORTS = { https: 443, http: 80, rtsp: 554, rtmp: 1935 };
/** eine Adresse nur als Schema · Host · Port (15 Grundsatz 5): nie Pfad, Abfrage oder Zugangsdaten, auch nicht maskiert */
function aaSHP(u) {
  const m = String(u || '').trim().match(/^([a-z][a-z0-9+.-]*):\/\/(?:[^@/]*@)?@?(\[[^\]]+\]|[^:/?#]+)(?::(\d+))?/i);
  if (!m) return '–';
  const sch = m[1].toLowerCase(); const port = m[3] || AA_PORTS[sch] || '–';
  return `${sch} · ${m[2]} · ${port}`;
}
const aaAddrLines = (a) => String(a || '').split('\n').map((x) => x.trim()).filter(Boolean);
function aaHost(a) { const m = String(a).match(/^[a-z]+:\/\/(?:[^@/]*@)?(@?[^:/?#]+)/i); return m ? m[1].replace(/^@/, '') : ''; }
const aaPortOf = (a) => { const m = String(a).match(/^[a-z]+:\/\/(?:[^@/]*@)?@?[^:/?#]+:(\d+)/i); return m ? m[1] : ''; };
const aaIsPrivate = (h) => /^(10\.|192\.168\.|127\.|172\.(1[6-9]|2\d|3[01])\.|localhost|2(2[4-9]|3\d)\.)/i.test(h);
/** feste Ziele aus einer Quelle der Beispieldaten (Heimnetz Multicast: die zwei Gruppen) */
const aaTargetsOf = (s) => (s.targets ? aaClone(s.targets) : s.kind === 'single' && s.localNet ? (s.streams || []).map((u) => ({ addr: aaHost(u), port: aaPortOf(u) })) : []);
function aaSrcNew() {
  return { id: null, kind: '', name: '', address: '', addrSet: false, addrShow: [], addrNew: true, file: '', fileSize: '', user: '', userSet: false, userNew: true, pw: '', pwSet: false, pwNew: true,
    hdr: [], uaMode: 'std', ua: '', uaSet: false, uaNew: true, origins: [], originAdd: '', targets: [], targetsOk: false, interval: 24, hour: '06:00', offerNew: true, takeGroups: true, takeNumbers: true, moreConn: false,
    epgAdd: true, test: null, outcome: 'auto', step: 1, single: { mode: 'new', name: '', no: '', group: 'ch', backup: 'srf1' }, tried: false, err1: false, nextWhy: false };
}
function aaSrcFrom(s) {
  const lines = s.kind === 'single' ? s.streams : s.kind === 'm3u-file' ? [] : [s.address];
  return Object.assign(aaSrcNew(), { id: s.id, kind: s.kind, name: s.name, address: '', addrSet: s.kind !== 'm3u-file', addrShow: lines.map(aaSHP), addrNew: false, file: s.file || '', fileSize: s.fileSize || '',
    userSet: !!s.user, userNew: !s.user, pwSet: s.kind === 'xtream', pwNew: s.kind !== 'xtream', hdr: (s.hdr || []).map((h) => ({ k: h.k, v: '', set: !!h.set, edit: false })), uaMode: s.uaMode || 'std', uaSet: s.uaMode === 'own', uaNew: s.uaMode !== 'own',
    origins: aaClone(s.origins || []), targets: aaTargetsOf(s), targetsOk: aaTargetsOf(s).length > 0, interval: s.interval, hour: s.hour || (s.last ? fmt.time(s.last) : '06:00'), offerNew: s.offerNew !== false,
    takeGroups: s.takeGroups !== false, takeNumbers: s.takeNumbers !== false, moreConn: !!s.moreConn, step: 0 });
}
const aaDraftKey = (d) => JSON.stringify([d.name, d.address, d.addrNew, d.file, d.user, d.userSet, d.pw, d.pwSet, d.hdr, d.uaMode, d.ua, d.uaSet, d.origins, d.targets, d.targetsOk, d.interval, d.hour, d.offerNew, d.takeGroups, d.takeNumbers, d.moreConn]);
/** Hosts der Quelle: eingetippte (typed) oder gespeicherte */
function aaHostsOf(d) {
  if (d.addrNew) return aaAddrLines(d.address).map((a) => ({ host: aaHost(a), port: aaPortOf(a), typed: true }));
  const s = LIVE.sources.find((x) => x.id === d.id); if (!s) return [];
  return (s.kind === 'single' ? s.streams : s.kind === 'm3u-file' ? [] : [s.address]).map((a) => ({ host: aaHost(a), port: aaPortOf(a), typed: false }));
}
const aaInNet = (t, host) => { const a = String(t.addr).trim(); if (a === host) return true; const m = a.match(/^(\d+\.\d+\.\d+)\.\d+\/(\d+)$/); return !!(m && Number(m[2]) <= 32 && Number(m[2]) >= 24 && host.startsWith(m[1] + '.')); };
const aaCovered = (d, host) => d.targetsOk && d.targets.some((t) => aaInNet(t, host));
const aaNeedTargets = (d) => d.kind !== 'm3u-file' && (d.targets.length > 0 || aaHostsOf(d).some((h) => aaIsPrivate(h.host)) || (d.test && d.test.o === 'local'));
function aaTargetErr(t) {
  const a = String(t.addr || '').trim(); const p = String(t.port || '').trim();
  if (!a) return 'Trage eine Adresse, ein Netz oder einen Hostnamen ein.';
  if (/^(127\.|0\.|169\.254\.|localhost$|::1$|fe80:)/i.test(a)) return 'Dieses Ziel ist nie freigebbar (Loopback, Link-Local oder eigene Adresse).';
  const cidr = a.match(/\/(\d+)$/); if (cidr && Number(cidr[1]) < 24) return 'Höchstens ein Netz /24.';
  if (!/^(\d{1,3}(\.\d{1,3}){3}(\/\d+)?|[a-z0-9-]+(\.[a-z0-9-]+)*)$/i.test(a)) return 'Das ist keine Adresse, kein Netz und kein Hostname.';
  const r = p.match(/^(\d+)(?:-(\d+))?$/); if (!r) return 'Trage einen Port oder einen Bereich ein, z. B. 5000 oder 5000-5010.';
  const lo = Number(r[1]); const hi = Number(r[2] || r[1]); if (lo < 1 || hi > 65535 || hi < lo) return 'Ports liegen zwischen 1 und 65535.'; if (hi - lo + 1 > 64) return 'Höchstens 64 Ports in einem Bereich.';
  return '';
}
function aaSrcErrors(d) {
  const e = {}; const a = d.address.trim();
  if (!d.name.trim()) e.name = 'Gib der Quelle einen Namen.';
  if (d.addrNew) {
    if (d.kind === 'm3u-url' && !/^https?:\/\/\S+$/i.test(a)) e.address = 'Die Adresse muss mit http:// oder https:// beginnen.';
    if (d.kind === 'xtream' && !/^https?:\/\/[^\s/:]+(:\d+)?\/?$/i.test(a)) e.address = 'Der Server hat die Form http://host:port.';
    if (d.kind === 'single') { const ls = aaAddrLines(a); if (!ls.length) e.address = 'Trage mindestens eine Adresse ein.'; else if (ls.some((l) => !/^(https?|rtsp|rtmp|udp|rtp):\/\/\S+$/i.test(l))) e.address = 'Erlaubt sind http, https, rtsp, rtmp, udp und rtp.'; }
  }
  if (d.kind === 'xtream') { if (!d.user.trim() && !d.userSet) e.user = 'Der Benutzername fehlt.'; if (!d.pw && !d.pwSet) e.pw = 'Das Passwort fehlt.'; }
  if (d.kind === 'm3u-file') { if (!d.file) e.file = 'Wähle eine Datei (.m3u oder .m3u8).'; else if (!/\.m3u8?$/i.test(d.file)) e.file = 'Nur Dateien mit der Endung .m3u oder .m3u8.'; else if (d.fileBig) e.file = 'Die Datei ist grösser als 200 MiB.'; }
  d.hdr.forEach((h, i) => { if (!h.k.trim() && (h.v || h.set)) e['h' + i] = 'Der Name der Kopfzeile fehlt.'; else if (h.k.trim() && !h.set && !h.v) e['h' + i] = 'Der Wert der Kopfzeile fehlt.'; });
  if (d.uaMode === 'own' && !d.ua.trim() && !d.uaSet) e.ua = 'Trage den User-Agent ein.';
  d.targets.forEach((t, i) => { const x = aaTargetErr(t); if (x) e['t' + i] = x; });
  if (d.targets.length && !d.targetsOk) e.tok = 'Bestätige die Ziele. Ohne Bestätigung speichert Slouchcat sie nicht.';
  if (d.targets.length > 16) e.tok = 'Höchstens 16 Ziele je Quelle.';
  return e;
}
const aaErrBand = (e) => ({ addr: !!(e.name || e.address || e.user || e.pw || e.file), hdr: Object.keys(e).some((k) => /^h\d|^ua$/.test(k)), net: Object.keys(e).some((k) => /^t\d|^tok$/.test(k)) });
/** simuliertes Testergebnis (15, 3.3 Schritt 3): ohne Wahl aus den Eingaben, sonst der gewählte Fall im Mockup */
function aaOutcome(d) {
  let o = d.outcome; const ex = d.id ? LIVE.sources.find((s) => s.id === d.id) : null;
  const a = d.addrNew ? aaAddrLines(d.address)[0] || '' : ex ? (ex.kind === 'single' ? ex.streams[0] : ex.address || '') : '';
  const hosts = aaHostsOf(d); const host = (hosts[0] || {}).host || ''; const priv = hosts.find((h) => aaIsPrivate(h.host) && !aaCovered(d, h.host));
  if (o === 'auto') {
    if (d.kind !== 'm3u-file' && priv) o = 'local';
    else if (/drm/i.test(host + a)) o = 'drm';
    else if (/offline|unreach|timeout|down\./i.test(host)) o = 'unreach';
    else if ((d.kind === 'xtream' && /^(falsch|wrong)$/i.test(d.user + '')) || /^(falsch|wrong)$/i.test(d.pw)) o = 'auth';
    else if (/\.(txt|html)$/i.test(d.file || a)) o = 'format';
    else if ((d.kind === 'm3u-url' || d.kind === 'm3u-file') && /manifest|\/hls\//i.test(a + d.file)) o = 'hls';
    else if (ex && ex.kind === 'xtream' && !ex.lastOk) o = 'busy';
    else o = 'ok';
  }
  const hostTxt = esc(host || 'server.example.net'); const wiz = !d.id;
  const back = wiz ? `<button type="button" class="btn secondary sm" data-act="aaToStep2" data-fid="la03:tostep2">${icon('chevL', 16)}Zurück zu Schritt 2</button>` : '';
  const T = { ok: false, o, act: back };
  if (o === 'auth') return Object.assign(T, { title: 'Anmeldung abgelehnt (Benutzername oder Passwort falsch).', text: 'Prüfe die Zugangsdaten bei deinem Anbieter. Das Passwort steht in keiner Meldung.' });
  if (o === 'unreach') return Object.assign(T, { title: 'Server nicht erreichbar (Zeitüberschreitung nach 10 s).', text: `Host ${hostTxt}. Prüfe die Adresse und ob der Anbieter erreichbar ist.` });
  if (o === 'busy') return Object.assign(T, { title: 'Server antwortet 503 (Dienst nicht verfügbar).', text: `Host ${hostTxt}. Die bisherige Liste bleibt aktiv.` });
  if (o === 'format') return Object.assign(T, { title: 'Antwort ist keine Senderliste (Zeile 1: erwartet «#EXTM3U»).', text: 'Prüfe, ob die Adresse zu einer M3U- oder M3U8-Senderliste führt.' });
  if (o === 'drm') return Object.assign(T, { title: 'Kopiergeschützter Stream (DRM). Slouchcat spielt solche Streams nicht ab.', text: 'Kopiergeschützte Streams spielt Slouchcat nicht ab und umgeht keinen Schutz.' });
  if (o === 'local') {
    const toNet = `<button type="button" class="btn secondary sm" data-act="aaToTargets" data-fid="la03:totargets">${icon('home2', 16)}${wiz ? 'Zurück zu Schritt 2: Ziel bestätigen' : 'Ziele im Heimnetz öffnen'}</button>`;
    /* selbst eingetippte Adresse: der Test nennt sie und verbindet nicht; Einträge einer Liste: nur ein Zustand, gezählt, ohne Adressen */
    if (priv && priv.typed) return Object.assign(T, { title: `Die Adresse liegt im Heimnetz (${esc(priv.host)}). Bestätige sie als festes Ziel dieser Quelle.`, text: 'Slouchcat hat dabei keine Verbindung aufgebaut.', act: toNet });
    return Object.assign(T, { title: 'Ziel im Heimnetz nicht freigegeben.', text: d.kind === 'single' ? 'Slouchcat hat das Ziel nicht aufgerufen.' : '3 Einträge der Liste zeigen ins Heimnetz (1 Netz, 1 Port). Slouchcat hat sie nicht aufgerufen.', act: toNet });
  }
  if (o === 'hls') return Object.assign(T, { title: 'Das ist kein Senderverzeichnis, sondern ein einzelner Stream (HLS-Manifest).', text: 'Als Einzelstrom anlegen?', act: `<button type="button" class="btn secondary sm" data-act="aaToSingle" data-fid="la03:tosingle">Als Einzelstrom anlegen</button>${back}` });
  if (d.kind === 'xtream') return { ok: true, o, kind: 'xtream', title: 'Zugang in Ordnung.', facts: [['Live-Sender', `${fmt.num(ex ? ex.total : 9874)} in ${ex ? ex.groups : 212} Kategorien`], ['Übernommen', 'nur Live-TV, keine Filme und Serien'], ['Hinweis', 'Angaben des Anbieters zu Verbindungszahl oder Ablaufdatum zeigt Slouchcat nicht.']], total: ex ? ex.total : 9874, groups: ex ? ex.groups : 212 };
  if (d.kind === 'single') return { ok: true, o, kind: 'single', title: 'Stream erkannt.', facts: [['Bild', 'H.264 1920 × 1080, 25 Bilder/s, interlaced'], ['Ton', 'AAC Stereo, AC-3 5.1'], ['Untertitel', 'Videotext'], ['Bitrate', 'etwa 8 Mbit/s'], ['Erstes Bild', 'nach 1.4 s']], total: Math.max(1, hosts.length), groups: 0 };
  const n = ex ? ex.total : 412; const g = ex ? ex.groups : 18;
  return { ok: true, o, kind: 'list', title: 'M3U-Senderliste erkannt.', facts: [['Einträge', fmt.num(n)], ['Gruppen', String(g)], ['Mit Logo', fmt.num(Math.round(n * 0.964))], ['Mit EPG-Kennung', fmt.num(Math.round(n * 0.922))], ['Programmdaten-Adresse', '<span class="mono">https · epg.example.org · 443</span> in der Liste gefunden']], epgFound: true, total: n, groups: g };
}
function aaTestStart() {
  const d = ui.draft.src; if (!d || (d.test && d.test.run)) return;
  const e = aaSrcErrors(d); if (Object.keys(e).length) { d.tried = true; const b = aaErrBand(e); if (b.addr) ui.open['la03:addr'] = true; if (b.hdr) ui.open['la03:hdr'] = true; if (b.net) ui.open['la03:net'] = true; render(); return; }
  d.test = { run: true }; d.nextWhy = false; render();
  const tok = d.test;
  setTimeout(() => { if (ui.draft.src !== d || d.test !== tok) return; d.test = aaOutcome(d); render(); }, 900);
}
ACT.aaTest = () => aaTestStart();
/* Auswahl «Nur im Mockup»: wirkt sofort, ohne «Noch einmal testen» */
ACT.aaOutcomeSet = () => { const d = ui.draft.src; if (!d) return; d.nextWhy = false; if (d.step === 3 || (d.test && !d.test.run)) d.test = aaOutcome(d); };
ACT.aaToStep2 = () => { const d = ui.draft.src; if (!d.id) { d.step = 2; d.nextWhy = false; AA.focusStep = true; } render(); };
ACT.aaToTargets = () => {
  const d = ui.draft.src; const p = aaHostsOf(d).find((h) => aaIsPrivate(h.host) && !aaCovered(d, h.host));
  if (p && !d.targets.some((t) => aaInNet(t, p.host))) d.targets.push({ addr: p.host, port: p.port || '' });
  if (!d.targets.length) d.targets.push({ addr: '', port: '' });
  d.targetsOk = false; ui.open['la03:net'] = true; d.nextWhy = false;
  if (!d.id) { d.step = 2; d.test = null; d.outcome = d.outcome === 'local' ? 'auto' : d.outcome; }
  render(); setTimeout(() => { const el = document.querySelector('[data-fid="la03:ta0"]'); if (el) el.focus(); }, 0);
};
ACT.aaToSingle = () => { const d = ui.draft.src; d.kind = 'single'; d.test = null; d.outcome = 'auto'; toast('Die Quelle ist jetzt ein Einzelstrom. Teste noch einmal.', 'ok'); render(); };
ACT.aaKind = () => { const d = ui.draft.src; d.interval = d.kind === 'xtream' ? 12 : 24; d.test = null; d.err1 = false; };
ACT.aaHdrAdd = () => { ui.draft.src.hdr.push({ k: '', v: '', set: false, edit: true }); render(); setTimeout(() => { const el = document.querySelector(`[data-fid="la03:hk${ui.draft.src.hdr.length - 1}"]`); if (el) el.focus(); }, 0); };
ACT.aaHdrDel = (dd) => { ui.draft.src.hdr.splice(Number(dd.i), 1); render(); };
ACT.aaHdrEdit = (dd) => { const h = ui.draft.src.hdr[Number(dd.i)]; h.edit = !h.edit; h.v = ''; render(); setTimeout(() => { const el = document.querySelector(`[data-fid="la03:hv${dd.i}"]`); if (el) el.focus(); }, 0); };
ACT.aaUaEdit = () => { const d = ui.draft.src; d.uaNew = !d.uaNew; d.ua = ''; render(); };
ACT.aaAddrNew = () => { const d = ui.draft.src; d.addrNew = !d.addrNew; d.address = ''; d.test = null; render(); setTimeout(() => { const el = document.getElementById('la03-addr'); if (el) el.focus(); }, 0); };
ACT.aaCred = (dd) => {
  const d = ui.draft.src; const k = dd.k;
  if (dd.op === 'edit') { d[k + 'New'] = !d[k + 'New']; d[k] = ''; }
  else { d[k + 'Set'] = false; d[k + 'New'] = true; d[k] = ''; toast(`${k === 'pw' ? 'Passwort' : 'Benutzername'} wird beim Speichern entfernt.`, 'warn'); }
  render();
};
ACT.aaOrigAdd = () => { const d = ui.draft.src; const h = String(d.originAdd || '').trim().toLowerCase(); if (!/^[a-z0-9-]+(\.[a-z0-9-]+)+$/.test(h)) { toast('Trage einen Hostnamen ein, z. B. cdn.example.net.', 'warn'); return; } if (!d.origins.includes(h)) d.origins.push(h); d.originAdd = ''; render(); };
ACT.aaOrigDel = (dd) => { ui.draft.src.origins.splice(Number(dd.i), 1); render(); };
ACT.aaTgAdd = () => { const d = ui.draft.src; d.targets.push({ addr: '', port: '' }); d.targetsOk = false; render(); setTimeout(() => { const el = document.querySelector(`[data-fid="la03:ta${d.targets.length - 1}"]`); if (el) el.focus(); }, 0); };
ACT.aaTgDel = (dd) => { const d = ui.draft.src; d.targets.splice(Number(dd.i), 1); d.targetsOk = false; render(); };
ACT.aaSample = () => { const d = ui.draft.src; d.file = 'senderliste-beispiel.m3u'; d.fileSize = '0.9 MB'; d.fileBig = false; d.test = null; render(); };
document.addEventListener('change', (e) => {
  const el = e.target; if (!el.matches || !el.matches('input[type="file"][data-aa]') || !ui.draft.src) return;
  const f = el.files && el.files[0]; if (!f) return; const d = ui.draft.src;
  d.file = f.name; d.fileSize = fmt.dec(f.size / 1048576, 1) + ' MB'; d.fileBig = f.size > 200 * 1048576; d.test = null; render();
});
function aaFileField(d, e, repl) {
  return aaF('la03-file', repl ? 'Datei ersetzen' : 'Datei', `<div class="row-gap"><label class="btn secondary aa-file">${icon('upload', 16)}Datei wählen<input type="file" class="vh" accept=".m3u,.m3u8" data-aa data-fid="la03:file" id="la03-file"></label><button type="button" class="btn quiet" data-act="aaSample" data-fid="la03:sample">Beispieldatei verwenden</button><span>${d.file ? `<span class="mono">${esc(d.file)}</span> · ${esc(d.fileSize || '')}` : '<span class="muted">Keine Datei gewählt</span>'}</span></div>`, { hint: '.m3u oder .m3u8, höchstens 200 MiB.', err: e.file });
}
/** gespeicherter, nur schreibbarer Wert: «gesetzt» mit Ändern und Entfernen */
const aaSetRow = (label, k) => `<div class="fld"><span class="lbl">${label}</span><div class="row-gap"><span class="chip ok">gesetzt</span><button type="button" class="btn quiet sm" data-act="aaCred" data-op="edit" data-k="${k}" data-fid="la03:${k}-edit">Ändern</button><button type="button" class="btn quiet dq sm" data-act="aaCred" data-op="del" data-k="${k}" data-fid="la03:${k}-del">Entfernen</button></div><span class="hint">Nur schreibbar: Slouchcat zeigt den Wert nie wieder an.</span></div>`;
function aaAddrFields(d, e, wizard) {
  const en = wizard ? 'aaNext' : null; let h = aaF('la03-name', 'Name', aaI('ui.draft.src.name', 'la03-name', { err: e.name, auto: wizard, enter: en }), { err: e.name, hint: 'Zum Beispiel «Kabelanbieter Schweiz».' });
  const label = d.kind === 'xtream' ? 'Server' : d.kind === 'single' ? 'Adresse des Streams' : 'Adresse der Senderliste';
  if (d.kind !== 'm3u-file' && d.addrSet && !d.addrNew) {
    h += aaF('la03-addrs', label, `<div class="stack aa-shp">${d.addrShow.map((x) => `<span><span class="mono">${esc(x)}</span> <span class="muted">· Adresse gespeichert</span></span>`).join('')}<div><button type="button" class="btn secondary sm" data-act="aaAddrNew" data-fid="la03:addrnew" id="la03-addrs">${icon('pen', 16)}${d.kind === 'single' ? 'Neue Adressen eingeben' : 'Neue Adresse eingeben'}</button></div></div>`, { hint: 'Angezeigt werden nur Schema, Host und Port. Die gespeicherte Adresse kommt nie zurück in den Browser.' });
  } else {
    const cancel = d.addrSet ? `<button type="button" class="btn quiet sm" data-act="aaAddrNew" data-fid="la03:addrkeep">Gespeicherte Adresse behalten</button>` : '';
    if (d.kind === 'm3u-url') h += aaF('la03-addr', label, aaI('ui.draft.src.address', 'la03-addr', { err: e.address, ph: 'https://iptv.example.net/playlist.m3u8', enter: en }) + cancel, { err: e.address, hint: 'Nach dem Speichern zeigt Slouchcat nur noch Schema, Host und Port.' });
    if (d.kind === 'xtream') h += aaF('la03-addr', label, aaI('ui.draft.src.address', 'la03-addr', { err: e.address, ph: 'http://xtream.example.com:8080', enter: en }) + cancel, { err: e.address });
    if (d.kind === 'single') h += aaF('la03-addr', label, aaT('ui.draft.src.address', 'la03-addr', { err: e.address, ph: 'udp://@239.10.0.1:5000', rows: 2 }) + cancel, { err: e.address, hint: 'Eine Adresse pro Zeile. «udp://@239.…» ist Multicast im Heimnetz und braucht ein festes Ziel (unten).' });
  }
  if (d.kind === 'm3u-file') h += aaFileField(d, e, !wizard);
  if (d.kind === 'xtream') {
    h += d.userSet && !d.userNew ? aaSetRow('Benutzername', 'user') : aaF('la03-user', 'Benutzername', aaI('ui.draft.src.user', 'la03-user', { err: e.user, enter: en }), { err: e.user, hint: 'Nur schreibbar: nach dem Speichern steht hier nur «gesetzt».' });
    h += d.pwSet && !d.pwNew ? aaSetRow('Passwort', 'pw') : aaF('la03-pw', 'Passwort', `<input class="inp" id="la03-pw" type="password" data-bind="ui.draft.src.pw" data-live data-fid="la03-pw" value="${esc(d.pw)}" autocomplete="new-password"${e.pw ? ' aria-invalid="true" aria-describedby="la03-pw-e"' : ''}${en ? ` data-enter="${en}"` : ''}>`, { err: e.pw, hint: 'Nur schreibbar: Slouchcat zeigt das Passwort nie wieder an.' });
    h += `<p class="muted aa-small">Von Xtream-Zugängen übernimmt Slouchcat nur Live-TV. Filme und Serien des Anbieters werden nicht übernommen.</p>`;
  }
  return h;
}
const aaHdrSum = (d) => { const n = d.hdr.filter((h) => h.k.trim()).length; return `${n ? `${n} ${n === 1 ? 'Kopfzeile' : 'Kopfzeilen'} gesetzt (${d.hdr.filter((h) => h.k.trim()).map((h) => esc(h.k)).join(', ')})` : 'keine Kopfzeilen'} · ${d.uaMode === 'own' ? 'eigener User-Agent' : 'Standard-User-Agent'}`; };
function aaHdrFields(d, e) {
  const rows = d.hdr.map((r, i) => {
    const err = e['h' + i] ? `<span class="aa-err" role="alert">${icon('alert', 14)}${esc(e['h' + i])}</span>` : '';
    if (r.set && !r.edit) return `<div class="aa-hrow aa-hset"><span><b class="mono">${esc(r.k)}</b></span><span class="chip ok">gesetzt</span><span class="row-gap"><button type="button" class="btn quiet sm" data-act="aaHdrEdit" data-i="${i}" data-fid="la03:he${i}">Überschreiben</button><button type="button" class="btn quiet icon" data-act="aaHdrDel" data-i="${i}" aria-label="Kopfzeile ${esc(r.k)} entfernen" data-fid="la03:hd${i}">${icon('trash', 16)}</button></span>${err}</div>`;
    return `<div class="aa-hrow"><div class="fld"><label for="la03-hk${i}" class="vh">Name der Kopfzeile ${i + 1}</label><input class="inp" id="la03-hk${i}" data-bind="ui.draft.src.hdr.${i}.k" data-live data-fid="la03:hk${i}" value="${esc(r.k)}" placeholder="Referer" spellcheck="false" autocomplete="off"${r.set ? ' readonly' : ''}${e['h' + i] ? ' aria-invalid="true"' : ''}></div>
    <div class="fld"><label for="la03-hv${i}" class="vh">${r.set ? 'Neuer Wert' : 'Wert'} der Kopfzeile ${i + 1}</label><input class="inp" id="la03-hv${i}" type="password" data-bind="ui.draft.src.hdr.${i}.v" data-live data-fid="la03:hv${i}" value="${esc(r.v)}" placeholder="${r.set ? 'neuer Wert' : 'Wert'}" autocomplete="new-password" spellcheck="false"></div>
    ${r.set ? `<button type="button" class="btn quiet sm" data-act="aaHdrEdit" data-i="${i}" data-fid="la03:hx${i}">Abbrechen</button>` : `<button type="button" class="btn quiet icon" data-act="aaHdrDel" data-i="${i}" aria-label="Kopfzeile ${i + 1} entfernen" data-fid="la03:hd${i}">${icon('trash', 16)}</button>`}${err}</div>`;
  }).join('');
  const ua = d.uaMode !== 'own' ? '' : d.uaSet && !d.uaNew ? `<div class="row-gap"><span class="chip ok">gesetzt</span><button type="button" class="btn quiet sm" data-act="aaUaEdit" data-fid="la03:uaedit">Überschreiben</button></div>`
    : `<input class="inp" id="la03-ua" type="password" data-bind="ui.draft.src.ua" data-live data-fid="la03-ua" value="${esc(d.ua)}" placeholder="eigener User-Agent" autocomplete="new-password" spellcheck="false" aria-label="Eigener User-Agent"${e.ua ? ' aria-invalid="true"' : ''}>${e.ua ? `<span class="aa-err" role="alert">${icon('alert', 14)}${esc(e.ua)}</span>` : ''}`;
  return `<p class="muted aa-small">Gespeicherte Werte von Kopfzeilen zeigt Slouchcat nie wieder an, auch nicht verdeckt. Du siehst nur den Namen und «gesetzt»; überschreiben und entfernen geht.</p><div class="stack">${rows || '<p class="muted">Keine eigenen Kopfzeilen.</p>'}</div>
    <div><button type="button" class="btn secondary sm" data-act="aaHdrAdd" data-fid="la03:hadd">${icon('plus', 16)}Kopfzeile hinzufügen</button></div>
    <div class="fld"><span class="lbl">User-Agent</span>${aaRadio('ui.draft.src.uaMode', 'la03ua', 'std', 'Standard von Slouchcat', 'la03:ua-std')}${aaRadio('ui.draft.src.uaMode', 'la03ua', 'own', 'Eigener (nur schreibbar wie eine Kopfzeile)', 'la03:ua-own')}${ua}</div>`;
}
/** Ursprünge für Zugangsdaten und Kopfzeilen: ab Werk nur der Host der Adresse */
function aaOriginFields(d) {
  const base = (aaHostsOf(d)[0] || {}).host;
  return `<p class="muted aa-small">An diese Hosts dürfen Zugangsdaten und Kopfzeilen gehen. Ab Werk nur der Host der eingetragenen Adresse. Füge einen Host hinzu, wenn die Ströme deines Anbieters auf einem anderen Host liegen.</p>
    <ul class="aa-origs">${base ? `<li><span class="mono">${esc(base)}</span><span class="muted">Host der Adresse</span></li>` : '<li class="muted">Host der Adresse (nach dem Eintragen)</li>'}${d.origins.map((h, i) => `<li><span class="mono">${esc(h)}</span><button type="button" class="btn quiet icon sm" data-act="aaOrigDel" data-i="${i}" aria-label="${esc(h)} entfernen" data-fid="la03:od${i}">${icon('trash', 16)}</button></li>`).join('')}</ul>
    <div class="row-gap"><label class="vh" for="la03-orig">Host hinzufügen</label><input class="inp aa-origin" id="la03-orig" data-bind="ui.draft.src.originAdd" data-fid="la03-orig" value="${esc(d.originAdd || '')}" placeholder="cdn.example.net" autocomplete="off" spellcheck="false" data-enter="aaOrigAdd"><button type="button" class="btn secondary sm" data-act="aaOrigAdd" data-fid="la03:oadd">${icon('plus', 16)}Host hinzufügen</button></div>`;
}
const aaOrigSum = (d) => { const n = (aaHostsOf(d)[0] ? 1 : 0) + d.origins.length; return `${n} ${n === 1 ? 'Host' : 'Hosts'}`; };
/** Schritt 2a: feste Ziele im Heimnetz (kein Ja-Nein-Schalter für das ganze Heimnetz) */
function aaTargetFields(d, e) {
  const p = aaHostsOf(d).find((h) => aaIsPrivate(h.host) && !d.targets.some((t) => aaInNet(t, h.host)));
  const rows = d.targets.map((t, i) => `<div class="aa-trow"><div class="fld"><label for="la03-ta${i}">Adresse oder Netz</label><input class="inp" id="la03-ta${i}" data-bind="ui.draft.src.targets.${i}.addr" data-live data-fid="la03:ta${i}" value="${esc(t.addr)}" placeholder="192.168.1.20" autocomplete="off" spellcheck="false"${e['t' + i] ? ' aria-invalid="true"' : ''}></div>
    <div class="fld"><label for="la03-tp${i}">Port</label><input class="inp" id="la03-tp${i}" data-bind="ui.draft.src.targets.${i}.port" data-live data-fid="la03:tp${i}" value="${esc(t.port)}" placeholder="5000" inputmode="numeric" autocomplete="off"${e['t' + i] ? ' aria-invalid="true"' : ''}></div>
    <button type="button" class="btn quiet icon" data-act="aaTgDel" data-i="${i}" aria-label="Ziel ${i + 1} entfernen" data-fid="la03:td${i}">${icon('trash', 16)}</button>${e['t' + i] ? `<span class="aa-err" role="alert">${icon('alert', 14)}${esc(e['t' + i])}</span>` : ''}</div>`).join('');
  return `<p class="muted aa-small">Es gibt keinen Schalter für das ganze Heimnetz. Trage die Ziele dieser Quelle einzeln ein: eine Adresse, ein kleines Netz (höchstens /24) oder einen Hostnamen, dazu einen Port oder einen kleinen Bereich (höchstens 64 Ports). Höchstens 16 Ziele.</p>
    ${p ? `<p class="aa-small">Die Adresse zeigt ins Heimnetz (<span class="mono">${esc(p.host)}${p.port ? ' · ' + esc(p.port) : ''}</span>). <button type="button" class="lnk" data-act="aaToTargets" data-fid="la03:tgfrom">Als Ziel übernehmen</button></p>` : ''}
    <div class="stack">${rows || '<p class="muted">Keine Ziele im Heimnetz.</p>'}</div>
    <div><button type="button" class="btn secondary sm" data-act="aaTgAdd" data-fid="la03:tadd" ${d.targets.length >= 16 ? 'disabled' : ''}>${icon('plus', 16)}Ziel hinzufügen</button></div>
    ${d.targets.length ? `<label class="check aa-confirm"><input type="checkbox" ${d.targetsOk ? 'checked' : ''} data-bind="ui.draft.src.targetsOk" data-fid="la03:tok"${e.tok ? ' aria-invalid="true"' : ''}><span>${AA_V3}</span></label>${e.tok ? `<span class="aa-err" role="alert">${icon('alert', 14)}${esc(e.tok)}</span>` : ''}` : ''}
    <p class="muted aa-small">Nie freigebbar: Loopback, 0.0.0.0/8, Link-Local und die eigenen Adressen des Containers. Jede Freigabe steht im Protokoll der Verwaltung.</p>`;
}
const aaTgSum = (d) => (d.targets.length ? `${d.targets.length} ${d.targetsOk ? 'bestätigt' : 'nicht bestätigt'}` : 'keine');
const AA_INTERVALS = [{ v: 6, t: 'alle 6 Stunden' }, { v: 12, t: 'alle 12 Stunden' }, { v: 24, t: 'alle 24 Stunden' }, { v: 168, t: 'wöchentlich' }, { v: 0, t: 'nur von Hand' }];
/** neue, inaktive Sender dieser Quelle (Filter «Neu» in LA-05) */
const aaNewOf = (id) => LIVE.channels.filter((c) => c.isNew && !c.active && c.sources.some((s) => s.source === id));
function aaUpdFields(d) {
  const nw = d.id && !d.offerNew ? aaNewOf(d.id) : [];
  return `<div class="srow"><div class="st"><b>Automatisch aktualisieren</b><span>Vorschlag: alle 24 Stunden, bei Xtream alle 12 Stunden.</span></div><div class="sc">${selectEl('ui.draft.src.interval', AA_INTERVALS, { label: 'Aktualisierung' })}${d.interval ? `<label class="vh" for="la03-hour">Uhrzeit</label><span>um</span><input class="inp" type="time" id="la03-hour" data-bind="ui.draft.src.hour" data-live value="${esc(d.hour)}" data-fid="la03-hour" style="max-width:120px">` : ''}</div></div>
    <div class="srow"><div class="st"><b>Neue Sender sofort anbieten</b><span>Werksvorgabe: an. An: neue Sender sind gleich für Zuschauer aktiv. Aus: sie kommen inaktiv in die Senderliste (Filter «Neu»).${nw.length ? ` In dieser Quelle auf aus gestellt; deshalb sind ${nw.map((c) => esc(c.name)).join(', ')} noch inaktiv (Filter <a class="lnk" href="#/admin/live/channels?tab=channels&state=new" data-fid="la03:newf">«Neu»</a>).` : ''}</span></div><div class="sc">${sw('ui.draft.src.offerNew', 'Neue Sender sofort anbieten')}<span>${d.offerNew ? 'an' : 'aus'}</span></div></div>`;
}
function aaTakeFields(d) {
  return `<div class="srow"><div class="st"><b>Gruppen der Quelle übernehmen</b></div><div class="sc">${sw('ui.draft.src.takeGroups', 'Gruppen der Quelle übernehmen')}<span>${d.takeGroups ? 'an' : 'aus'}</span></div></div>
    <div class="srow"><div class="st"><b>Kanalnummern der Quelle übernehmen</b><span>Nur für neue Sender.</span></div><div class="sc">${sw('ui.draft.src.takeNumbers', 'Kanalnummern übernehmen')}<span>${d.takeNumbers ? 'an' : 'aus'}</span></div></div>
    <div class="srow"><div class="st"><b>Verschwundene Sender</b><span>Sofort für Zuschauer ausgeblendet. Favoriten, Nummer, Gruppen, Zuordnung und Planungen bleiben gespeichert und wirken wieder, wenn der Sender zurückkommt. Slouchcat löscht nie von selbst.</span></div><div class="sc"><span class="muted">keine Wahl</span></div></div>
    <div class="srow"><div class="st"><b>Schutz vor unvollständigen Listen</b><span>Fehlt mehr als ein Drittel der bisherigen Sender, hält Slouchcat die Liste zurück und markiert nichts als verschwunden.</span></div><div class="sc"><span class="muted">immer an</span></div></div>`;
}
/** Vorladen je Quelle: dieselbe Einstellung wie LA-09 «Vorladen je Quelle» (Datenfeld moreConn) */
const aaPreFields = () => `<div class="srow"><div class="st"><b>Vorladen für Sender dieser Quelle</b><span>${AA_PRELOAD_HINT} Werksvorgabe: aus. Wie viele Sender insgesamt vorgeladen werden, stellst du unter <a class="lnk" href="#/admin/live/reception?tab=strategy" data-fid="la03:rcp">Empfang</a> ein.</span></div><div class="sc">${sw('ui.draft.src.moreConn', 'Vorladen für Sender dieser Quelle')}<span>${ui.draft.src.moreConn ? 'an' : 'aus'}</span></div></div>`;
function aaTestBlock(d) {
  const t = d.test; let h = `<div class="aa-test"><div class="row-gap"><button type="button" class="btn secondary" data-act="aaTest" data-fid="la03:test" ${t && t.run ? 'disabled' : ''}>${icon('play', 16)}${t && !t.run ? 'Noch einmal testen' : 'Testen'}</button>
    <label class="aa-mock" for="la03-out"><span>Nur im Mockup, nicht Teil der App: Ergebnis des Tests</span>${selectEl('ui.draft.src.outcome', AA_OUT.map(([v, x]) => ({ v, t: x })), { label: 'Nur im Mockup, nicht Teil der App: Ergebnis des Tests', after: 'aaOutcomeSet' }).replace('<select ', '<select id="la03-out" ')}</label></div>`;
  if (t && t.run) h += `<div class="aa-testing" role="status"><span class="spin"></span>Slouchcat ruft die Adresse ab …</div>`;
  else if (t) {
    h += t.ok ? notice('ok', t.title, '', t.epgFound ? `<label class="check"><input type="checkbox" ${d.epgAdd ? 'checked' : ''} data-bind="ui.draft.src.epgAdd" data-fid="la03:epgadd">Programmdaten-Quelle aus der Liste auch als EPG-Quelle hinzufügen</label>` : '') + `<dl class="kv aa-facts">${t.facts.map(([k, v]) => `<dt>${k}</dt><dd>${v}</dd>`).join('')}</dl>`
      : notice('bad', t.title, t.text, t.act || '');
  }
  return h + '</div>';
}
/* Vorschau im Assistenten: dieselbe Darstellung wie LA-04 mit den Beispielzahlen der Quelle; das Ergebnis nach dem Übernehmen nennt dieselben Zahlen */
function aaPreviewData(d) {
  const r = aaClone(LIVE.importResult); const n = d.test && d.test.total ? d.test.total : 412;
  r.counts = { new: Math.max(0, n - r.dup.length - r.bad.length), changed: 0, gone: 0, dup: r.dup.length, bad: r.bad.length, same: 0 }; r.changed = []; r.gone = []; return r;
}
function aaSingleStep(d) {
  const s = d.single; const chs = LIVE.channels.filter((c) => c.active);
  return `<div class="stack"><p class="lead">Slouchcat hat den Stream erkannt. Lege einen neuen Sender an oder nutze ihn als Ersatzquelle.</p>
    ${aaRadio('ui.draft.src.single.mode', 'la03sm', 'new', 'Neuen Sender anlegen', 'la03:sm-new')}${s.mode === 'new' ? `<div class="aa-sub">${aaF('la03-sn', 'Name des Senders', aaI('ui.draft.src.single.name', 'la03-sn', { err: d.tried && !s.name.trim() ? 'x' : '' }), { err: d.tried && !s.name.trim() ? 'Gib dem Sender einen Namen.' : '' })}${aaF('la03-sno', 'Nummer', aaI('ui.draft.src.single.no', 'la03-sno', { num: true }), { hint: 'Leer: ohne Nummer, die Sender folgen alphabetisch.' })}
      ${aaF('la03-sg', 'Gruppe', selectEl('ui.draft.src.single.group', LIVE.groups.map((g) => ({ v: g.id, t: g.name })), { label: 'Gruppe' }).replace('<select ', '<select id="la03-sg" '))}</div>` : ''}
    ${aaRadio('ui.draft.src.single.mode', 'la03sm', 'backup', 'Als Ersatzquelle zu einem bestehenden Sender hinzufügen', 'la03:sm-backup')}${s.mode === 'backup' ? `<div class="aa-sub">${aaF('la03-sb', 'Sender wählen', selectEl('ui.draft.src.single.backup', chs.map((c) => ({ v: c.id, t: `${c.no ?? '–'} ${c.name}` })), { label: 'Sender wählen' }).replace('<select ', '<select id="la03-sb" '), { hint: 'Die neue Quelle kommt an die letzte Stelle der Rangfolge.' })}</div>` : ''}</div>`;
}
function aaSrcDone(d) {
  const t = d.test || {}; const sch = AA_INTERVALS.find((x) => x.v === d.interval).t; const kind = AA_KINDS.find((k) => k[0] === d.kind)[1]; const pv = aaPreviewData(d).counts;
  return `<dl class="kv"><dt>Name</dt><dd>${esc(d.name)}</dd><dt>Art</dt><dd>${kind}</dd><dt>Adresse</dt><dd>${d.kind === 'm3u-file' ? `Datei <span class="mono">${esc(d.file)}</span>` : aaAddrLines(d.address).map((a) => `<span class="mono">${esc(aaSHP(a))}</span>`).join('<br>')}</dd>
    <dt>Sender</dt><dd>${d.kind === 'single' ? (d.single.mode === 'new' ? `neuer Sender «${esc(d.single.name || '–')}»` : `Ersatzquelle von ${esc((SIM.channel(d.single.backup) || {}).name || '')}`) : `${fmt.num(pv.new)} neu, ${d.offerNew ? 'gleich aktiv' : 'zunächst inaktiv (Filter «Neu»)'} · ${pv.dup} mögliche Duplikate · ${pv.bad} fehlerhafte Einträge`}</dd>
    <dt>Aktualisierung</dt><dd>${d.kind === 'single' ? 'entfällt' : sch + (d.interval ? ` um ${esc(d.hour)}` : '')}</dd><dt>Ziele im Heimnetz</dt><dd>${aaTgSum(d)}</dd><dt>Vorladen</dt><dd>${d.moreConn ? 'an' : 'aus'}</dd></dl>
    <p class="muted">Mit «Quelle speichern und Sender übernehmen» legt Slouchcat die Quelle an und liest die Liste ein. Das Ergebnis siehst du danach als Import-Ergebnis.</p>`;
}
function aaWizard(d, e) {
  const names = AA_STEPS.map((x, i) => (i === 3 && d.kind === 'single' ? 'Sender' : x));
  const bar = `<ol class="aa-prog" aria-label="Schritte">${names.map((x, i) => `<li ${d.step === i + 1 ? 'aria-current="step"' : ''} class="${d.step > i + 1 ? 'done' : ''}"><span class="n">${d.step > i + 1 ? icon('check', 14) : i + 1}</span><span class="t">${x}</span></li>`).join('')}</ol>`;
  let body = '';
  if (d.step === 1) body = `${aaVerantwortung('la03:resp')}<fieldset class="aa-cards"><legend class="vh">Art der Quelle</legend>${AA_KINDS.map(([k, t, x, ic]) => `<label class="aa-card"><input type="radio" name="la03k" value="${k}" data-bind="ui.draft.src.kind" data-after="aaKind" ${d.kind === k ? 'checked' : ''} data-fid="la03:k:${k}"><span class="ic">${icon(ic, 22)}</span><span><b>${t}</b><small>${x}</small></span></label>`).join('')}</fieldset>${d.err1 ? `<span class="aa-err" role="alert">${icon('alert', 14)}Wähle eine Art.</span>` : ''}`;
  if (d.step === 2) {
    body = `<div class="stack aa-form">${aaAddrFields(d, e, true)}</div>${band('la03:hdr', 'Kopfzeilen und User-Agent', aaHdrSum(d), aaHdrFields(d, e), { open: aaErrBand(e).hdr && d.tried })}${d.kind === 'm3u-file' ? '' : band('la03:orig', 'Ursprünge für Zugangsdaten und Kopfzeilen', aaOrigSum(d), aaOriginFields(d))}`;
    if (aaNeedTargets(d)) body += `<section class="aa-2a" aria-labelledby="la03-2a"><h3 id="la03-2a" class="la02-h3">2a · Feste Ziele im Heimnetz</h3>${aaTargetFields(d, e)}</section>`;
  }
  if (d.step === 3) body = `<p class="lead">Slouchcat ruft die Adresse einmal ab und zeigt, was dort liegt. Es wird nichts übernommen.</p>${aaTestBlock(d)}`;
  if (d.step === 4) body = d.kind === 'single' ? aaSingleStep(d) : `<p class="lead">So würde der erste Import aussehen. Du übernimmst noch nichts.</p>${aaImportView(aaPreviewData(d), { preview: true, key: 'wizard' })}`;
  if (d.step === 5) body = `<div class="card"><div class="srow-wrap">${d.kind === 'single' ? '<p class="muted">Einzelströme werden nicht importiert und nicht aktualisiert.</p>' : aaUpdFields(d) + aaTakeFields(d)}${aaPreFields()}</div></div>`;
  if (d.step === 6) body = aaSrcDone(d);
  const last = d.step === 6; const t = d.test; const blocked = d.step === 3 && !(t && t.ok);
  const why = !blocked ? '' : t && t.run ? 'Der Test läuft noch. Warte auf das Ergebnis.' : t ? `Weiter geht es erst nach einem erfolgreichen Test. Der Test meldet: ${t.title} Korrigiere die Angaben in Schritt 2 und teste noch einmal.` : 'Weiter geht es erst nach einem erfolgreichen Test.';
  const whyHtml = blocked && d.nextWhy ? `<div class="notice bad aa-why" role="alert" id="la03-why" tabindex="-1"><span class="ic">${icon('alert')}</span><div><b>Weiter ist gesperrt.</b><p>${why}</p>${t && !t.run ? `<div class="row-gap"><button type="button" class="btn secondary sm" data-act="aaToStep2" data-fid="la03:why2">${icon('chevL', 16)}Zurück zu Schritt 2</button></div>` : ''}</div></div>` : '';
  const nav = `<div class="aa-wnav"><button type="button" class="btn ghost" data-act="aaBack" data-fid="la03:back">${d.step === 1 ? 'Abbrechen' : 'Zurück'}</button>${last ? `<button type="button" class="btn primary" data-act="aaFinish" data-fid="la03:finish">${icon('check', 16)}Quelle speichern und Sender übernehmen</button>` : `<span class="row-gap">${blocked ? '<span class="muted aa-small" id="la03-next-hint">Erst nach erfolgreichem Test</span>' : ''}<button type="button" class="btn primary${blocked ? ' aa-blocked' : ''}" data-act="aaNext" data-fid="la03:next"${blocked ? ' aria-describedby="la03-next-hint"' : ''}>Weiter</button></span>`}</div>`;
  return `${bar}<section class="card aa-step" aria-labelledby="la03-h"><h2 id="la03-h" tabindex="-1" class="aa-sh">Schritt ${d.step} von 6 · ${names[d.step - 1]}</h2>${body}</section>${whyHtml}${nav}`;
}
ACT.aaBack = () => { const d = ui.draft.src; if (d.step === 1) { go('#/admin/live/sources'); return; } d.step--; d.nextWhy = false; AA.focusStep = true; render(); };
ACT.aaNext = () => {
  const d = ui.draft.src; if (!d || d.step === 0) return;
  if (d.step === 1) { if (!d.kind) { d.err1 = true; render(); return; } d.step = 2; }
  else if (d.step === 2) { const e = aaSrcErrors(d); if (Object.keys(e).length) { d.tried = true; const b = aaErrBand(e); if (b.hdr) ui.open['la03:hdr'] = true; render(); return; } d.step = 3; d.test = null; d.nextWhy = false; AA.focusStep = true; render(); aaTestStart(); return; }
  else if (d.step === 3) { if (!(d.test && d.test.ok)) { d.nextWhy = true; render(); setTimeout(() => { const el = document.getElementById('la03-why'); if (el) el.focus(); }, 0); return; } d.step = 4; }
  else if (d.step === 4) { if (d.kind === 'single' && d.single.mode === 'new' && !d.single.name.trim()) { d.tried = true; render(); return; } d.step = 5; }
  else if (d.step === 5) d.step = 6;
  AA.focusStep = true; render();
};
AA.firstRun = {};
ACT.aaFinish = () => {
  const d = ui.draft.src; const t = d.test || {}; const now = SIM.now(); const lines = aaAddrLines(d.address);
  const id = 'src-' + (d.name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'neu') + '-' + LIVE.sources.length;
  const pv = aaPreviewData(d); const nNew = d.kind === 'single' ? 1 : pv.counts.new;
  const tg = d.targetsOk ? aaClone(d.targets) : [];
  const s = { id, name: d.name.trim(), kind: d.kind, address: d.kind === 'single' ? '' : aaMask(d.address.trim()), host: aaHost(d.address), user: d.kind === 'xtream' ? 'gesetzt' : '', file: d.file, fileSize: d.fileSize, uploaded: now, streams: d.kind === 'single' ? lines.map(aaMask) : undefined,
    active: d.kind === 'single' ? 1 : d.offerNew ? nNew : 0, total: d.kind === 'single' ? 1 : nNew, groups: t.groups || 0, gone: 0, interval: d.kind === 'single' ? 0 : d.interval, last: d.kind === 'single' ? null : now, lastOk: true, next: d.interval && d.kind !== 'single' ? now + d.interval * 60 : null, state: 'ok',
    targets: tg, localNet: tg.length > 0, offerNew: d.offerNew, moreConn: d.moreConn, origins: aaClone(d.origins),
    hdr: d.hdr.filter((h) => h.k.trim()).map((h) => ({ k: h.k.trim(), v: '', set: true })), uaMode: d.uaMode, hour: d.hour, takeGroups: d.takeGroups, takeNumbers: d.takeNumbers };
  LIVE.sources.push(s); ADM.addedImported += s.total; let run = null;
  if (d.kind === 'single') {
    if (d.single.mode === 'new') { const no = d.single.no === '' ? null : Number(d.single.no); LIVE.channels.push({ id: 'n-' + id, no, name: d.single.name.trim(), slug: 'n-' + id, country: 'CH', langs: ['de'], groups: [d.single.group], category: 'Sonstiges', epg: 'none', epgKey: null, reception: 'default', state: 'ok', since: null, bitrate: 8, video: 'H.264 · 1920 × 1080 · 25 Bilder/s', audio: ['Deutsch (AAC Stereo)'], subs: [], sources: [{ source: id, name: d.single.name.trim(), url: aaMask(lines[0]), state: 'ok' }], favCount: 0, active: true, nameInSource: d.single.name.trim() }); }
    else { const c = SIM.channel(d.single.backup); if (c) c.sources.push({ source: id, name: s.name, url: aaMask(lines[0]), state: 'ok' }); }
  } else {
    run = `run-${id}-1`; LIVE.importRuns[id] = [[run, now, 'ok', nNew, 0, 0, 'Erster Import']];
    AA.firstRun[run] = Object.assign(pv, { source: id, run, at: now, res: 'ok', note: 'Erster Import' });
    if (d.epgAdd && t.epgFound && !LIVE.epgSources.some((x) => x.address === 'https://epg.example.org/ch-de.xml.gz')) LIVE.epgSources.push({ id: 'epg-' + id, name: `Programmdaten aus «${s.name}»`, kind: 'xmltv-url', address: 'https://epg.example.org/ch-de.xml.gz', interval: 24, last: now, lastOk: true, channels: 214, programmes: 48312, until: LIVE.EPG_END, warnings: [], shift: 0 });
  }
  ui.open['la02:' + id] = true; AA.srcFor = null;
  toast(`Quelle «${esc(s.name)}» gespeichert${d.kind === 'single' ? '.' : `. ${fmt.num(nNew)} neue Sender gelesen.`}`, 'ok');
  go(run ? `#/admin/live/sources/${id}/import/${run}` : '#/admin/live/sources');
};
/* Ändern: dieselben Felder als Aufklapper, SaveBar */
function aaEditView(d, s) {
  const e = d.tried ? aaSrcErrors(d) : {}; const eb = aaErrBand(e); const dirty = aaDraftKey(d) !== AA.srcSnap;
  const kind = AA_KINDS.find((k) => k[0] === d.kind)[1]; const sch = AA_INTERVALS.find((x) => x.v === d.interval).t;
  const shp = d.addrNew ? (aaAddrLines(d.address).map(aaSHP)[0] || 'neue Adresse') : d.addrShow[0] || '';
  const sumAddr = d.kind === 'xtream' ? `Xtream, <span class="mono">${esc(shp)}</span>, Benutzer ${d.userSet || d.user ? 'gesetzt' : 'nicht gesetzt'}, Passwort ${d.pwSet || d.pw ? 'gesetzt' : 'nicht gesetzt'}` : d.kind === 'm3u-file' ? `Datei ${esc(d.file)}` : d.kind === 'single' ? `${d.addrNew ? aaAddrLines(d.address).length : d.addrShow.length} Einzelströme` : `<span class="mono">${esc(shp)}</span>`;
  const bands = band('la03:addr', 'Adresse und Zugang', sumAddr, `<div class="stack aa-form">${aaAddrFields(d, e, false)}</div>${aaTestBlock(d)}${d.test && d.test.ok ? `<div><a class="btn quiet sm" href="#/admin/live/sources/${s.id}/import/preview" data-fid="la03:prev">Import-Vorschau ansehen</a></div>` : ''}`, { icon: 'link', open: eb.addr })
    + band('la03:hdr', 'Kopfzeilen und User-Agent', aaHdrSum(d), aaHdrFields(d, e), { icon: 'key', open: eb.hdr })
    + (d.kind === 'm3u-file' ? '' : band('la03:orig', 'Ursprünge', aaOrigSum(d), aaOriginFields(d), { icon: 'link' }))
    + (d.kind === 'single' ? '' : band('la03:upd', 'Aktualisierung', d.interval ? `${sch} um ${esc(d.hour)}` : 'nur von Hand', aaUpdFields(d), { icon: 'refresh' }) + band('la03:take', 'Übernahme', `neue Sender ${d.offerNew ? 'sofort' : 'inaktiv'}, Gruppen ${d.takeGroups ? 'an' : 'aus'}, Nummern ${d.takeNumbers ? 'an' : 'aus'}`, aaTakeFields(d), { icon: 'list' }))
    + band('la03:pre', 'Vorladen', d.moreConn ? 'an' : 'aus', aaPreFields(), { icon: 'zap' })
    + (d.kind === 'm3u-file' ? '' : band('la03:net', 'Ziele im Heimnetz', aaTgSum(d), aaTargetFields(d, e), { icon: 'home2', open: eb.net || undefined }));
  return `<p class="muted">${kind}. Eigene Namen, Logos, Gruppen, Nummern, Favoriten, Zuordnungen und geplante Aufnahmen bleiben bei jeder Aktualisierung erhalten.</p>${aaVerantwortung('la03:resp')}${bands}
    <div class="aa-danger"><button type="button" class="btn quiet dq" data-act="la02Remove" data-id="${s.id}" data-fid="la03:rm">${icon('trash', 16)}Quelle entfernen</button></div>
    ${dirty ? `<div class="savebar" role="region" aria-label="Änderungen speichern"><span class="grow">Ungespeicherte Änderungen</span><button type="button" class="btn ghost" data-act="aaDiscard" data-fid="la03:discard">Verwerfen</button><button type="button" class="btn primary" data-act="aaSave" data-fid="la03:save">Speichern</button></div>` : ''}`;
}
ACT.aaDiscard = () => { const s = LIVE.sources.find((x) => x.id === ui.draft.src.id); ui.draft.src = aaSrcFrom(s); AA.srcSnap = aaDraftKey(ui.draft.src); toast('Änderungen verworfen.', 'ok'); render(); };
ACT.aaSave = () => {
  const d = ui.draft.src; const e = aaSrcErrors(d); d.tried = true;
  if (Object.keys(e).length) { const b = aaErrBand(e); if (b.addr) ui.open['la03:addr'] = true; if (b.hdr) ui.open['la03:hdr'] = true; if (b.net) ui.open['la03:net'] = true; toast('Bitte korrigiere die markierten Angaben.', 'bad'); render(); return; }
  const s = LIVE.sources.find((x) => x.id === d.id); const tg = d.targetsOk ? aaClone(d.targets) : [];
  Object.assign(s, { name: d.name.trim(), targets: tg, localNet: tg.length > 0, offerNew: d.offerNew, moreConn: d.moreConn, origins: aaClone(d.origins), interval: d.kind === 'single' ? 0 : d.interval, hour: d.hour, takeGroups: d.takeGroups, takeNumbers: d.takeNumbers, uaMode: d.uaMode,
    hdr: d.hdr.filter((h) => h.k.trim()).map((h) => ({ k: h.k.trim(), v: '', set: true })) });
  if (d.kind === 'xtream') s.user = d.user ? 'gesetzt' : d.userSet ? s.user : '';
  if (d.addrNew && d.address.trim()) { if (d.kind === 'single') s.streams = aaAddrLines(d.address).map(aaMask); else { s.address = aaMask(d.address.trim()); s.host = aaHost(d.address); } }
  if (d.kind === 'm3u-file') { s.file = d.file; s.fileSize = d.fileSize; }
  s.next = s.interval ? SIM.now() + s.interval * 60 : null;
  ui.draft.src = aaSrcFrom(s); AA.srcSnap = aaDraftKey(ui.draft.src);
  toast(`Quelle «${esc(s.name)}» gespeichert.`, 'ok'); render();
};
route('/admin/live/sources/new', () => aaSrcRoute('new'));
route('/admin/live/sources/:id', (pm) => aaSrcRoute(pm.id));
function aaSrcRoute(id) {
  const fresh = parseHash().path !== lastPath; const m = aaMode(); const s = id === 'new' ? null : LIVE.sources.find((x) => x.id === id);
  if (fresh || AA.srcFor !== id) { AA.srcFor = id; ui.draft.src = s ? aaSrcFrom(s) : aaSrcNew(); AA.srcSnap = aaDraftKey(ui.draft.src); AA.focusStep = id === 'new'; }
  const d = ui.draft.src; let html;
  if (m === 'loading') html = skelRows(4, 80);
  else if (m === 'error') html = liveError('Die Quelle konnte nicht geladen werden.');
  else if (id !== 'new' && !s) html = notice('warn', 'Diese Quelle gibt es nicht (mehr).', 'Sie wurde entfernt oder die Adresse stimmt nicht.', '<a class="btn secondary" href="#/admin/live/sources" data-fid="la03:nf">Zu den Quellen</a>');
  else html = id === 'new' ? aaWizard(d, d.tried ? aaSrcErrors(d) : {}) : aaEditView(d, s);
  return aaView({ area: 'livetv', page: 'sources', title: id === 'new' ? 'Quelle hinzufügen' : `Quelle ändern${s ? ': ' + s.name : ''}`, intro: id === 'new' ? 'In sechs Schritten: Art wählen, Zugang eintragen, testen, Vorschau prüfen, einstellen, übernehmen.' : 'Dasselbe Formular wie beim Anlegen. Jeder Abschnitt zeigt seine Zusammenfassung in der Kopfzeile.', back: ['#/admin/live/sources', 'Quellen'], html, after: () => { if (AA.focusStep) { AA.focusStep = false; const h = document.getElementById('la03-h'); if (h) h.focus({ preventScroll: true }); } } });
}

/* ======================================================================================
   LA-04 Import-Vorschau und -Ergebnis
   ====================================================================================== */
const AA_POOL = ['Kanal Nord', 'Tele Alpin', 'Doku Plus', 'Kino Klassik', 'Wetter Live', 'Arena 2', 'Regio TV Ost', 'Musik 24'];
function aaRunResult(s, run) {
  if (LIVE.importResult.run === run[0]) return aaClone(LIVE.importResult);
  if (AA.firstRun[run[0]]) return aaClone(AA.firstRun[run[0]]); /* erster Import einer neuen Quelle: dieselben Zahlen wie die Vorschau in LA-03 */
  const [, , res, n, c, g, note] = run; const names = LIVE.channels.filter((x) => x.active && x.sources.some((q) => q.source === s.id));
  return { source: s.id, run: run[0], at: run[1], res, note, counts: { new: n, changed: c, gone: g, dup: 0, bad: 0, same: Math.max(0, s.total - n - c - g) },
    new: AA_POOL.slice(0, Math.min(n, 5)).map((x, i) => [x, 'Schweiz', null, i % 2 === 0, '']), changed: names.slice(0, Math.min(c, 5)).map((x) => [x.name, 'Adresse geändert', 'wirkt sofort']), gone: AA_POOL.slice(5, 5 + Math.min(g, 3)).map((x) => [x, 'seit ' + fmt.day(run[1]), 0, 0]), dup: [], bad: [] };
}
const AA_GROUPS = [['new', 'Neu'], ['changed', 'Geändert'], ['gone', 'Verschwunden'], ['dup', 'Mögliche Duplikate'], ['bad', 'Fehlerhafte Einträge'], ['same', 'Unverändert']];
function aaImportView(res, o) {
  const st = ui.draft.la04 || (ui.draft.la04 = { grp: 'new', take: {}, gone: {}, nodup: {} }); const c = res.counts; const g = st.grp;
  const nodup = st.nodup; const dupRows = res.dup.filter((r) => !nodup[r[0]]);
  const cnt = (k) => (k === 'dup' ? dupRows.length : c[k]);
  const bar = `<div class="stats aa-grp" style="--n:6" role="group" aria-label="Gruppen des Imports">${AA_GROUPS.map(([k, l]) => `<button type="button" class="stat ${cnt(k) && (k === 'bad' || k === 'dup') ? 'warn' : ''}" aria-pressed="${g === k}" data-act="aaGrp" data-g="${k}" data-fid="la04:g:${k}"><span class="l">${l}</span><span class="v">${fmt.num(cnt(k))}</span></button>`).join('')}</div>`;
  const tb = (heads, rows, empty) => rows.length ? `<div class="tblw aa-tbl"><table class="tbl"><thead><tr>${heads.map((h) => `<th${h === 'Aktion' ? ' class="end"' : ''}>${h === 'Aktion' ? '<span class="vh">Aktion</span>' : h}</th>`).join('')}</tr></thead><tbody>${rows.join('')}</tbody></table></div>` : `<p class="muted">${empty}</p>`;
  let tab = '';
  if (g === 'new') {
    const rows = res.new.map(([n, grp, no, logo, epg], i) => `<tr><td class="main" data-l="Name in der Liste">${esc(n)}</td><td data-l="Gruppe">${esc(grp)}</td><td data-l="Nummer">${no ?? '–'}</td><td data-l="Logo">${logo ? 'ja' : 'nein'}</td><td data-l="EPG-Kennung">${epg ? `<span class="mono">${esc(epg)}</span>` : '–'}</td>${o.preview ? `<td class="end" data-l="Übernehmen"><label class="check"><input type="checkbox" ${st.take[n] === false ? '' : 'checked'} data-act="aaTake" data-n="${esc(n)}" data-fid="la04:take:${i}"><span class="vh">${esc(n)} übernehmen</span></label></td>` : '<td></td>'}</tr>`);
    tab = tb(['Name in der Liste', 'Gruppe', 'Nummer', 'Logo', 'EPG-Kennung', o.preview ? 'Aktion' : ''], rows, 'Keine neuen Einträge.') + (res.new.length < c.new ? `<p class="muted aa-small">Gezeigt sind ${res.new.length} von ${fmt.num(c.new)} Einträgen (Beispielausschnitt).</p>` : '');
  } else if (g === 'changed') tab = tb(['Sender', 'Was sich geändert hat', 'Wirkung'], res.changed.map(([n, w, x]) => `<tr><td class="main" data-l="Sender">${esc(n)}</td><td data-l="Änderung">${esc(w)}</td><td data-l="Wirkung">${esc(x)}</td></tr>`), 'Keine Änderungen.');
  else if (g === 'gone') tab = tb(['Sender', 'Seit', 'Betroffen', 'Aktion'], res.gone.map(([n, since, fav, rec], i) => { const k = st.gone[n]; return `<tr><td class="main" data-l="Sender">${esc(n)}</td><td data-l="Seit">${esc(since)}</td><td data-l="Betroffen">${fav} Favoriten · ${rec} geplante Aufnahmen</td><td class="end" data-l="Aktion">${k ? `<span class="chip ${k === 'off' ? 'warn' : 'ok'}">${k === 'off' ? 'abgeschaltet' : 'behalten'}</span>` : `<div class="ta"><button type="button" class="btn quiet" data-act="aaGone" data-n="${esc(n)}" data-v="off" data-fid="la04:off:${i}">Abschalten</button><button type="button" class="btn quiet" data-act="aaGone" data-n="${esc(n)}" data-v="keep" data-fid="la04:keep:${i}">Behalten</button></div>`}</td></tr>`; }), 'Keine verschwundenen Sender.');
  else if (g === 'dup') tab = tb(['Eintrag', 'Vermuteter Sender', 'Grund', 'Aktion'], dupRows.map(([n, other, why], i) => `<tr><td class="main" data-l="Eintrag">${esc(n)}</td><td data-l="Vermuteter Sender">${esc(other)}</td><td data-l="Grund">${esc(why)}</td><td class="end" data-l="Aktion"><div class="ta"><button type="button" class="btn quiet" data-act="aaDupFromImport" data-n="${esc(n)}" data-fid="la04:merge:${i}">Zusammenführen</button><button type="button" class="btn quiet" data-act="aaNoDup" data-n="${esc(n)}" data-fid="la04:nodup:${i}">Kein Duplikat</button></div></td></tr>`), 'Keine möglichen Duplikate.');
  else if (g === 'bad') tab = tb(['Zeile in der Datei', 'Grund'], res.bad.map(([z, w]) => `<tr><td class="main" data-l="Zeile">${fmt.num(z)}</td><td data-l="Grund">${esc(w)}</td></tr>`), 'Keine fehlerhaften Einträge.');
  else tab = `<p class="muted">${fmt.num(c.same)} Sender sind unverändert und werden nicht einzeln aufgelistet.</p>`;
  return `${bar}<div class="aa-grpbody" aria-live="polite">${tab}</div>`;
}
ACT.aaGrp = (d) => { ui.draft.la04.grp = d.g; render(); };
ACT.aaTake = (d) => { const t = ui.draft.la04.take; t[d.n] = t[d.n] === false; render(); };
ACT.aaGone = (d) => { ui.draft.la04.gone[d.n] = d.v; toast(d.v === 'off' ? `«${esc(d.n)}» ist abgeschaltet.` : `«${esc(d.n)}» bleibt im Angebot.`, 'ok'); render(); };
ACT.aaNoDup = (d) => { ui.draft.la04.nodup[d.n] = true; toast('Slouchcat merkt sich: kein Duplikat.', 'ok'); render(); };
ACT.aaDupFromImport = (d) => { const dup = LIVE.duplicates.find((x) => x.names[1].startsWith(d.n.replace(/ HD.*/, '')) || x.names[1].startsWith(d.n)); if (dup) ACT.aaMergeOpen({ ids: dup.channels.join(',') }); else toast('Dieses Duplikat steht in der Senderliste unter «Duplikate».', 'warn'); };
function aaRetry(s) {
  if (AA.test['rt' + s.id]) return; AA.test['rt' + s.id] = true; toast('Neuer Versuch gestartet …', 'ok', 1500); render();
  setTimeout(() => { delete AA.test['rt' + s.id]; const t = SIM.now(); s.lastOk = true; s.lastError = null; s.state = 'ok'; s.last = t; s.next = s.interval ? t + s.interval * 60 : null; const id = `run-${s.id}-${Math.round(t)}`; LIVE.importRuns[s.id] = [[id, t, 'ok', 0, 3, 0, '']].concat(LIVE.importRuns[s.id] || []).slice(0, 5); toast(`Import «${esc(s.name)}» erfolgreich: 3 geändert.`, 'ok'); go(`#/admin/live/sources/${s.id}/import/${id}`); }, 1200);
}
ACT.aaRetry = (d) => aaRetry(LIVE.sources.find((x) => x.id === d.id));
ACT.aaForce = (d) => { const s = LIVE.sources.find((x) => x.id === d.id); const r = (LIVE.importRuns[s.id] || []).find((x) => x[0] === d.run); if (r) { r[2] = 'ok'; r[6] = 'Trotzdem übernommen'; } toast('Die neue Liste ist übernommen.', 'ok'); render(); };
ACT.aaKeep = (d) => { ui.draft.la04.kept = true; toast('Die bisherige Liste bleibt, wie sie ist.', 'ok'); render(); };
ACT.aaStartImport = (d) => { ACT.la02Refresh({ id: d.id }); go('#/admin/live/sources'); };
route('/admin/live/sources/:id/import/:run', (pm) => {
  const fresh = parseHash().path !== lastPath; if (fresh) ui.draft.la04 = { grp: 'new', take: {}, gone: {}, nodup: {} };
  const m = aaMode(); const s = LIVE.sources.find((x) => x.id === pm.id); const back = ['#/admin/live/sources', 'Quellen'];
  const mk = (title, html, extra) => aaView(Object.assign({ area: 'livetv', page: 'sources', title, back, html }, extra));
  if (m === 'loading') return mk('Import', skelRows(5, 64));
  if (m === 'error') return mk('Import', liveError('Der Import konnte nicht geladen werden.'));
  if (!s || m === 'empty') return mk('Import', aaEmptyBox('Kein Import vorhanden', 'Diese Quelle gibt es nicht oder sie hat noch keinen Lauf.', '<a class="btn secondary" href="#/admin/live/sources" data-fid="la04:nf">Zu den Quellen</a>'));
  const preview = pm.run === 'preview'; const run = preview ? null : (LIVE.importRuns[s.id] || []).find((x) => x[0] === pm.run);
  if (!preview && !run) return mk('Import', notice('warn', 'Diesen Lauf gibt es nicht (mehr).', 'Es werden nur die letzten fünf Läufe aufbewahrt.', `<a class="btn secondary" href="#/admin/live/sources" data-fid="la04:nf2">Zu den Quellen</a>`));
  const res = preview ? Object.assign(aaClone(LIVE.importResult), { source: s.id }) : aaRunResult(s, run); const st = ui.draft.la04; const busy = AA.test['rt' + s.id];
  let banner = '';
  if (preview) banner = notice('', 'Vorschau, noch nichts übernommen.', 'So würde der nächste Import von «' + esc(s.name) + '» aussehen.', `<button type="button" class="btn primary sm" data-act="aaStartImport" data-id="${s.id}" data-fid="la04:start">Import jetzt starten</button>`);
  else if (run[2] === 'failed') banner = notice('bad', `Die Liste konnte nicht gelesen werden (${esc(run[6] || 'unbekannter Fehler')}).`, `<b>Die bisherigen ${fmt.num(s.total)} Sender bleiben unverändert.</b> Nächster Versuch ${s.next ? aaWhen(s.next) : 'von Hand'}.`, `<button type="button" class="btn secondary sm" data-act="aaRetry" data-id="${s.id}" data-fid="la04:retry" ${busy ? 'disabled' : ''}>${busy ? '<span class="spin"></span>' : icon('refresh', 16)}Jetzt erneut versuchen</button>`);
  else if (run[2] === 'incomplete') banner = st.kept ? notice('warn', 'Die bisherige Liste bleibt.', 'Du hast entschieden, die unvollständige Liste nicht zu übernehmen.') : notice('warn', 'Die neue Liste hat 61 % weniger Sender als bisher.', 'Slouchcat hat keine Sender als verschwunden markiert.', `<button type="button" class="btn secondary sm" data-act="aaForce" data-id="${s.id}" data-run="${run[0]}" data-fid="la04:force">Liste trotzdem so übernehmen</button><button type="button" class="btn ghost sm" data-act="aaKeep" data-fid="la04:keepold">So lassen</button>`);
  const failed = !preview && run[2] === 'failed';
  const head = `<p class="muted">Erhalten bleiben immer: eigene Namen, eigene Logos, Gruppen, Kanalnummern, Favoriten der Zuschauer, EPG-Zuordnungen und geplante Aufnahmen.</p>`;
  const title = preview ? `Vorschau «${s.name}»` : `Import «${s.name}» · ${aaWhen(run[1])}`;
  return mk(title, banner + head + (failed ? '' : aaImportView(res, { preview, key: pm.run })), { intro: preview ? 'Vorschau vor dem Übernehmen.' : 'Was dieser Lauf geändert hat. Die letzten fünf Läufe stehen in der Quellenliste.', actions: `<a class="btn secondary" href="#/admin/live/sources/${s.id}" data-fid="la04:edit">Quelle ändern</a>` });
});
