/* Web Verwaltung B, Teil 2: LA-12 Aktive Streams, LA-13 Überwachung und Diagnose, LA-14 Aufnahmen (DVR),
   LA-15 Timeshift, LA-16 Speicher. Bauvorlage: 15-administration.md 3.12 bis 3.16 und 4 (Privatsphäre).
   Globale Namen mit Präfix «ab» (Helfer in admin-b.js). Zahlen kommen aus dem simulierten Zustand und sind
   dieselben wie in LA-01: admViewers(), admRecRunning(), LIVE.inputs, LIVE.resources, LIVE.storage, LIVE.accounts. */
'use strict';

/* ---------- Beispieldaten ergänzen: Timeshift je Puffer ---------- */
(function abSeed2() {
  LIVE.inputs.forEach((n) => { const m = String(n.timeshift || '').match(/([\d.]+) GB \((\d+)/); n.tsPer = m ? parseFloat(m[1]) / Number(m[2]) : 0; });
})();

/* ---------- Protokoll und Kennzahlen der Woche ---------- */
const AB_LOGK = { outage: ['Ausfall', 'bad'], switch: ['Quellwechsel', 'info'], error: ['Fehler', 'bad'], warn: ['Warnung', 'warn'], reconnect: ['Wiederverbindung', 'info'], info: ['Info', 'info'], import: ['Import', 'info'] };
function abLog(ch, kind, text, t) { LIVE.diagnostics.log.unshift([t == null ? SIM.now() + 0.02 : t, ch, kind, text]); }
/* Laufzeitregel statt Einstellung je Quelle (04 T-17, RG-03): Beispielereignisse vom Sa 28.11. vor 20:41 */
ADM_REST.forEach((r) => LIVE.diagnostics.log.push([r.at + 0.04, null, 'warn', r.pattern === 'A'
  ? `Vorsorglicher Empfang von «${admSrcName(r.src)}» ruht bis ${fmt.time(r.until)}: Quelle lehnt eine weitere Verbindung ab · Muster A · ${r.n}. Mal`
  : `Vorsorglicher Empfang von «${admSrcName(r.src)}» ruht bis morgen ${fmt.time(r.until)}: Quelle trennt die ältere Verbindung: Ein-Strom-Betrieb · Muster B · ${r.n}. Mal`]));
function abPc(ch) { let r = LIVE.diagnostics.perChannel.find((x) => x[0] === ch); if (!r) { r = [ch, 0, '0 Min.', 0, 0]; LIVE.diagnostics.perChannel.push(r); } return r; }
const abParseDur = (s) => { const h = (String(s).match(/(\d+) Std/) || [0, 0])[1]; const m = (String(s).match(/(\d+) Min/) || [0, 0])[1]; return Number(h) * 60 + Number(m); };
const abWeek = () => { const P = LIVE.diagnostics.perChannel; return { outages: P.reduce((a, r) => a + r[1], 0), reconnects: P.reduce((a, r) => a + r[3], 0), switches: P.reduce((a, r) => a + r[4], 0) }; };

/* ======================================================================================
   LA-12 Aktive Streams
   ====================================================================================== */
const AB_REASON = { viewer: 'Zuschauer', recording: 'Aufnahme', permanent: 'Permanent', preload: 'Vorgeladen', schedule: 'Zeitplan', auto: 'Automatisch' };
ui.abIn = { view: 'tiles', f: 'all' };
/** laufende Aufnahmen je Eingang, ohne Titel und Profil: Konto und Kennung */
function abRecsOn(ch) {
  const mine = LIVE.profiles.flatMap((p) => SIM.recordings(p.id).filter((r) => r.state === 'running' && r.ch === ch)).map((r) => ({ acct: LIVE.account.email, id: r.id }));
  const others = LIVE.consumers.filter((c) => c[1] === 'recording' && c[0] === ch && c[2] !== LIVE.account.email).map((c, i) => ({ acct: c[2], id: ch + i }));
  return mine.concat(others);
}
/** Eingänge, die eine laufende Aufnahme der Beispielkonten brauchen, aber noch keinen Eintrag haben, kommen dazu */
function abSyncRecInputs() {
  LIVE.profiles.flatMap((p) => SIM.recordings(p.id).filter((r) => r.state === 'running')).forEach((r) => {
    let n = LIVE.inputs.find((x) => x.ch === r.ch);
    if (!n) { const c = SIM.channel(r.ch); const s = (c.sources.find((q) => q.state !== 'fail') || c.sources[0]); n = { ch: r.ch, source: s ? s.source : 'src-kabel', backup: 0, state: 'running', reasons: ['recording'], since: r.start, viewers: 0, recordings: 1, bitrate: c.bitrate, transcode: '–', buffer: '12 MB', timeshift: '–', tsPer: 0, recGB: 0, errors: null }; LIVE.inputs.push(n); LIVE.resources.bandIn = Math.round((LIVE.resources.bandIn + c.bitrate) * 10) / 10; }
    else if (!n.reasons.includes('recording')) n.reasons.push('recording');
  });
}
const abRank = (n) => (n.backup ? n.backup : 1);
function abInfo(n) {
  const c = SIM.channel(n.ch); const vs = admViewers().filter((v) => v.ch === n.ch); const recs = abRecsOn(n.ch);
  const reconnecting = n.state === 'reconnecting'; const dist = SIM.chState(c) !== 'ok'; const busy = !!AB.busyIn[n.ch];
  const deferred = n.state === 'deferred';
  const status = busy ? ['Verbindet', 'info'] : deferred ? [`Zurückgestellt seit ${fmt.time(n.since)}: konnte nicht starten (${admDefCause(n)}, ${admDefTries(n)} Versuche)`, 'warn'] : reconnecting || dist ? ['Gestört', 'bad'] : ['Läuft', 'ok'];
  const direct = vs.filter((v) => !v.tr).length; const trans = vs.length - direct;
  const tsGB = Math.round(n.tsPer * vs.length * 10) / 10;
  let err = n.errors || null; if (reconnecting && err) err = `${err} · nächster Versuch ${fmt.time(SIM.now() + 2)}`; if (deferred) err = null;
  if (dist && !reconnecting && !err) err = 'Bild gestört (Zustand im Mockup)';
  if (recs.length && !reconnecting && ['full', 'unmounted'].includes(ui.abSto)) err = (err ? err + ' · ' : '') + 'Aufnahme: Speicherort nicht verfügbar';
  const srcIdx = c.sources.findIndex((s) => s.source === n.source);
  return { n, c, vs, recs, status, deferred, bad: reconnecting || dist, direct, trans, tsGB, err, runMin: SIM.now() - n.since, srcName: abSrcName(n.source), rank: abRank(n), canSwitch: c.sources.length > 1, srcIdx,
    conv: vs.length ? `${direct} direkt · ${trans} umgewandelt${trans ? ' (Hardware QSV)' : ''}` : '–', br: reconnecting ? 0 : n.bitrate, reasons: n.reasons };
}
const abInReasons = (i) => i.reasons.map((r) => `<span class="chip info">${AB_REASON[r] || r}${r === 'schedule' && i.n.schedule ? ` «${esc(i.n.schedule)}»` : ''}</span>`).join(' ');
function abInFacts(i) {
  const n = i.n; const c = i.c;
  const src = `${esc(i.srcName)} (${i.rank === 1 ? 'Hauptquelle' : 'Ersatzquelle ' + i.rank})`;
  const stor = `Startpuffer ${esc(n.buffer)} · Timeshift ${i.vs.length ? `${abDe(i.tsGB, 1)} GB (${abPl(i.vs.length, 'Puffer', 'Puffer')})` : '–'} · Aufnahmen ${i.recs.length && n.recGB ? abDe(n.recGB, 1) + ' GB' : '–'}`;
  return `<dl class="kv ab-facts"><dt>Aktive Quelle</dt><dd>${src}</dd><dt>Empfangsstatus</dt><dd><span class="chip ${i.status[1]}">${esc(i.status[0])}</span></dd><dt>Grund</dt><dd>${abInReasons(i)}</dd>
    <dt>Laufzeit</dt><dd>${abDur(i.runMin)}</dd><dt>Zuschauer</dt><dd>${i.vs.length}</dd><dt>Aufnahmen</dt><dd>${admRecCell(i.recs.length)}</dd><dt>Bitrate</dt><dd>${abDe(i.br, 1)} Mbit/s</dd>
    <dt>Bild</dt><dd>${esc(c.video)}</dd><dt>Ton</dt><dd>${esc(c.audio.join(', '))}</dd><dt>Untertitel</dt><dd>${c.subs.length ? esc(c.subs.join(', ')) : 'keine'}</dd><dt>Umwandlung</dt><dd>${i.conv}</dd><dt>Speicher</dt><dd>${stor}</dd><dt>Fehler</dt><dd>${i.err ? `<span class="ab-bad">${icon('alert', 14)}${esc(i.err)}</span>` : '–'}</dd></dl>`;
}
function abInTakers(i) {
  const v = i.vs.map((x) => `<li>${icon('user', 16)}<span><b>${esc(x.acct)}</b> · ${esc(x.prof)} · ${esc(x.dev)}<small>${esc(x.mode)} · ${/zeitversetzt/.test(x.off) ? esc(x.off) : 'Live'}</small></span></li>`).join('');
  const r = i.recs.length ? `<li>${icon('record', 16)}<span><b>${admRecWord(i.recs.length)}</b><small>Zahl erst ab 2; Konto, Profil, Kennung und Titel einer Aufnahme siehst du nicht (Privatsphäre).</small></span></li>` : '';
  return `<h3 class="la02-h3">Abnehmer</h3>${v || r ? `<ul class="ab-takers">${v}${r}</ul>` : (i.deferred ? `<p class="muted">Zurückgestellt: konnte nicht starten (${esc(admDefCause(i.n))}, ${admDefTries(i.n)} Versuche). ${esc(admDefWhy(i.n))} Der Sender belegt keinen Platz. Slouchcat startet ihn, sobald die Zulassung 60 Sekunden lang gelänge; laufende Sender weichen dafür nicht.</p>` : '<p class="muted">Kein Zuschauer und keine Aufnahme. Der Sender wird aus einem anderen Grund empfangen (siehe «Grund»).</p>')}`;
}
function abInActions(i) {
  const ch = i.n.ch; const busy = !!AB.busyIn[ch];
  return `<div class="row-gap ab-acts"><button type="button" class="btn secondary" data-act="abInRe" data-ch="${ch}" data-fid="la12:re:${ch}" ${busy ? 'disabled' : ''}>${icon('refresh', 16)}Neu verbinden</button>
    <button type="button" class="btn secondary" data-act="abInNext" data-ch="${ch}" data-fid="la12:nx:${ch}" ${i.canSwitch && !busy ? '' : 'disabled'} ${i.canSwitch ? '' : 'title="Dieser Sender hat keine Ersatzquelle"'}>${icon('arrowDown', 16)}Zur nächsten Quelle wechseln</button>
    <button type="button" class="btn quiet dq" data-act="abInEnd" data-ch="${ch}" data-fid="la12:end:${ch}">${icon('x', 16)}Beenden</button><a class="btn ghost" href="#/admin/live/diagnostics?channel=${ch}" data-fid="la12:dg:${ch}">Diagnose</a></div>${i.canSwitch ? '' : '<p class="muted aa-small">Dieser Sender hat nur eine Quelle, ein Wechsel ist nicht möglich.</p>'}`;
}
const abInSummary = (i) => `<span class="chip ${i.status[1]}">${esc(i.status[0])}</span>${abInReasons(i)}<span>${abPl(i.vs.length, 'Zuschauer', 'Zuschauer')} · ${i.recs.length ? admRecWord(i.recs.length) : '0 Aufnahmen'} · ${abDe(i.br, 1)} Mbit/s · ${abDur(i.runMin)}</span>`;
function abInTile(i) {
  const c = i.c;
  return `<div class="ab-tile${i.bad ? ' bad' : ''}">${band('ab:in:' + c.id, `<span class="ab-chn">${LU.logo(c)}<span><b>${c.no != null ? c.no + ' ' : ''}${esc(c.name)}</b><small>${esc(i.srcName)}</small></span></span>`, abInSummary(i), `${abInFacts(i)}${abInTakers(i)}${abInActions(i)}`)}</div>`;
}
function abInRow(i) {
  const c = i.c; const open = isOpen('ab:in:' + c.id);
  return `<tr class="${open ? 'open' : ''}"><td class="main" data-l="Sender"><span class="ab-name"><button type="button" class="tog" data-act="open" data-id="ab:in:${c.id}" aria-expanded="${open}" aria-label="${esc(c.name)}: Einzelheiten ${open ? 'zuklappen' : 'aufklappen'}" data-fid="la12:tog:${c.id}">${icon('chevR', 14)}</button>${abChLabel(c)}</span></td>
    <td data-l="Aktive Quelle">${esc(i.srcName)}${i.rank > 1 ? ` <span class="chip warn">Ersatzquelle ${i.rank}</span>` : ''}</td><td data-l="Status"><span class="chip ${i.status[1]}">${esc(i.status[0])}</span></td><td data-l="Grund">${abInReasons(i)}</td><td data-l="Laufzeit">${abDur(i.runMin)}</td>
    <td class="end" data-l="Zuschauer">${i.vs.length}</td><td class="end" data-l="Aufnahmen">${admRecCell(i.recs.length)}</td><td class="end" data-l="Bitrate">${abDe(i.br, 1)} Mbit/s</td><td data-l="Fehler">${i.err ? `<span class="ab-bad">${esc(i.err)}</span>` : '–'}</td></tr>
    ${open ? `<tr class="sub"><td colspan="9"><div class="ab-det">${abInFacts(i)}${abInTakers(i)}${abInActions(i)}</div></td></tr>` : ''}`;
}
ACT.abInView = (dd) => { ui.abIn.view = dd.v; render(); };
ACT.abInF = (dd) => { ui.abIn.f = dd.k; render(); };
ACT.abInSort = (dd) => { abSortToggle(AB.inSort, dd.k); render(); };
const AB_INF = [['all', 'Alle'], ['viewer', 'Zuschauer'], ['recording', 'Aufnahme'], ['permanent', 'Permanent'], ['preload', 'Vorgeladen'], ['schedule', 'Zeitplan'], ['deferred', 'Zurückgestellt'], ['bad', 'Gestört']];
function abStreams() {
  const all = admInputs().map(abInfo); /* the same inputs as LA-01 (admin-core.js) */ const vs = admViewers(); const R = LIVE.resources; const I = aaInputStats(); const RS = aaRecStats();
  const trans = vs.filter((v) => v.tr).length; const bandOut = Math.round(vs.reduce((a, v) => a + v.br, 0) * 10) / 10;
  const strip = `<div class="stats ab-strip" style="--n:6">${abStat('Eingänge', `${I.n} von ${I.max}`, `empfängt Slouchcat gerade${I.def ? ` · ${I.def} zurückgestellt` : ''}`, { href: '#/admin/live', fid: 'la12:s-in' })}${abStat('Zuschauer', vs.length, 'alle Eingänge zusammen')}${abStat('Aufnahmen', RS.run, `laufen gerade auf ${abPl(RS.chs, 'Sender', 'Sendern')}`)}${abStat('Umwandlungen', trans, 'für einzelne Geräte')}${abStat('Bandbreite ein', `${abDe(R.bandIn, 1)}`, 'Mbit/s von den Quellen')}${abStat('Bandbreite aus', `${abDe(bandOut, 1)}`, 'Mbit/s zu den Geräten')}</div>`;
  const match = (i, k) => (k === 'all' ? true : k === 'bad' ? i.bad : k === 'deferred' ? i.deferred : !i.deferred && i.reasons.includes(k));
  const cnt = (k) => all.filter((i) => match(i, k)).length;
  const f = ui.abIn.f; let list = all.filter((i) => match(i, f));
  const S = AB.inSort; const key = { ch: (i) => (i.c.no == null ? 9999 : i.c.no), src: (i) => i.srcName, st: (i) => i.status[0], run: (i) => i.runMin, v: (i) => i.vs.length, r: (i) => i.recs.length, br: (i) => i.br }[S.sort] || ((i) => i.c.no);
  list = list.sort((a, b) => (key(a) < key(b) ? -1 : key(a) > key(b) ? 1 : 0) * S.dir);
  const filt = `<div class="ab-ftog" role="group" aria-label="Nach Grund filtern">${AB_INF.map(([k, t]) => `<button type="button" class="btn ${f === k ? 'secondary' : 'ghost'} sm" aria-pressed="${f === k}" data-act="abInF" data-k="${k}" data-fid="la12:f:${k}">${t} <span class="c">${cnt(k)}</span></button>`).join('')}</div>`;
  const body = list.length ? (ui.abIn.view === 'table'
    ? `<div class="tblw ab-tbl"><table class="tbl"><thead><tr>${abSortHead(S, 'ch', 'Sender', 'abInSort')}${abSortHead(S, 'src', 'Aktive Quelle', 'abInSort')}${abSortHead(S, 'st', 'Status', 'abInSort')}<th>Grund</th>${abSortHead(S, 'run', 'Laufzeit', 'abInSort')}${abSortHead(S, 'v', 'Zuschauer', 'abInSort', 'end')}${abSortHead(S, 'r', 'Aufnahmen', 'abInSort', 'end')}${abSortHead(S, 'br', 'Bitrate', 'abInSort', 'end')}<th>Fehler</th></tr></thead><tbody>${list.map(abInRow).join('')}</tbody></table></div>`
    : `<div class="ab-grid" role="group" aria-label="Aktive Eingänge. Mit den Pfeiltasten wechselst du zwischen den Kacheln.">${list.map(abInTile).join('')}</div>`)
    : abEmptyBox(all.length ? 'Keine Treffer' : 'Gerade empfängt Slouchcat keinen Sender.', all.length ? 'Zu diesem Grund läuft gerade kein Eingang.' : 'Sobald jemand schaut, eine Aufnahme läuft oder ein Sender permanent empfangen wird, erscheint er hier.');
  const why = band('ab:in:why', 'Warum läuft ein Eingang?', 'Haltegründe: Zuschauer · Aufnahme · Permanent · Zeitplan · Vorgeladen · Automatisch', `<ul class="ab-list"><li><b>Zuschauer</b>: jemand schaut den Sender. Geschützt, wenn die Technik knapp wird.</li><li><b>Aufnahme</b>: eine Aufnahme braucht den Sender, auch im Vor- und Nachlauf. Geschützt.</li><li><b>Permanent</b>: der Sender ist auf «Permanent» gestellt und läuft rund um die Uhr. Ein laufender permanenter Sender bleibt auch bei Mangel; reicht die Kapazität nicht, wird die neue Anfrage abgelehnt («ausgelastet»). «Zurückgestellt» heisst: konnte nicht starten.</li><li><b>Zeitplan</b>: der Sender läuft im Zeitfenster seines Zeitplans.</li><li><b>Vorgeladen</b>: Nachbar eines Senders, den jemand schaut. Fällt bei Mangel zuerst weg.</li><li><b>Automatisch</b>: einer der oft genutzten Sender, solange jemand Live-TV schaut.</li></ul><p class="muted">Ein Sender kann mehrere Gründe haben. Er endet, wenn der letzte Grund weg ist. Einzelne Wiedergaben je Gerät siehst du unter <a class="lnk" href="#/admin/streams" data-fid="la12:la21">Nutzer &amp; Geräte › Streams</a>.</p>`, { icon: 'info' });
  const priv = band('ab:in:priv', 'Privatsphäre in dieser Ansicht', 'Aufnahmen nur als Zahl, ohne Konto, Profil und Titel', '<p>Bei Zuschauern siehst du Konto, Profil und Gerät, wie überall in der Verwaltung. Bei Aufnahmen siehst du nur die Zahl je Sender. Ein Eingang ist ein Sender zu einer Zeit; mit dem Konto daneben verriete das öffentliche Programm den Titel. Du kannst fremde Aufnahmen weder abspielen noch löschen.</p>', { icon: 'lock' });
  const pl = admPermLoad();
  return `${admLockLine()}${pl ? notice('warn', 'Dauersender belegen mehr als die Hälfte der Eingänge', admPermText(pl)) : ''}${strip}<div class="ab-tools">${filt}<div class="seg" role="group" aria-label="Ansicht"><button type="button" aria-pressed="${ui.abIn.view === 'tiles'}" data-act="abInView" data-v="tiles" data-fid="la12:v:t">${icon('grid', 14)} Kacheln</button><button type="button" aria-pressed="${ui.abIn.view === 'table'}" data-act="abInView" data-v="table" data-fid="la12:v:r">${icon('list', 14)} Tabelle</button></div></div>${body}${why}${priv}`;
}
/* Pfeiltasten im Kachelraster */
document.addEventListener('keydown', (e) => {
  if (!['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(e.key)) return;
  const a = document.activeElement; const grid = a && a.closest && a.closest('.ab-grid'); if (!grid || !a.matches('.bh .tg')) return;
  const btns = $$('.bh .tg', grid); const k = btns.indexOf(a); if (k < 0) return; e.preventDefault();
  if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') { btns[(k + (e.key === 'ArrowRight' ? 1 : -1) + btns.length) % btns.length].focus(); return; }
  const r = a.getBoundingClientRect(); const dir = e.key === 'ArrowDown' ? 1 : -1; let best = null; let bd = 1e9;
  btns.forEach((b) => { if (b === a) return; const q = b.getBoundingClientRect(); const dy = (q.top - r.top) * dir; if (dy <= 8) return; const d = dy * 4 + Math.abs(q.left - r.left); if (d < bd) { bd = d; best = b; } });
  if (best) best.focus();
});
/* Aktionen */
ACT.abInRe = (dd) => {
  const n = LIVE.inputs.find((x) => x.ch === dd.ch); const c = SIM.channel(dd.ch); AB.busyIn[dd.ch] = true; render();
  setTimeout(() => {
    delete AB.busyIn[dd.ch];
    if (n.state === 'reconnecting') { n.attempt = [Math.min(n.attempt[1], n.attempt[0] + 1), n.attempt[1]]; abLog(dd.ch, 'reconnect', `Wiederverbindung ${n.attempt[0]} von ${n.attempt[1]} (von Hand)`); abPc(dd.ch)[3]++; render(); toast(`${esc(c.name)}: Keine Quelle antwortet. Nächster Versuch ${fmt.time(SIM.now() + 2)}.`, 'warn', 6000); return; }
    n.errors = null; n.since = SIM.now(); abLog(dd.ch, 'reconnect', 'Von Hand neu verbunden'); abPc(dd.ch)[3]++; render(); toast(`${esc(c.name)} ist neu verbunden.`);
  }, 1400);
};
ACT.abInNext = (dd) => {
  const n = LIVE.inputs.find((x) => x.ch === dd.ch); const c = SIM.channel(dd.ch); if (c.sources.length < 2) return;
  const idx = c.sources.findIndex((s) => s.source === n.source); const nx = (idx + 1) % c.sources.length; const to = c.sources[nx];
  n.source = to.source; n.backup = nx === 0 ? 0 : nx + 1; if (n.state === 'reconnecting' && to.state !== 'fail') { n.state = 'running'; n.attempt = null; }
  abLog(dd.ch, 'switch', `Wechsel auf ${nx === 0 ? 'Hauptquelle' : 'Ersatzquelle ' + (nx + 1)} «${abSrcName(to.source)}» (von Hand)`); abPc(dd.ch)[4]++;
  render(); toast(`${esc(c.name)} läuft jetzt über «${esc(abSrcName(to.source))}» (${nx === 0 ? 'Hauptquelle' : 'Ersatzquelle ' + (nx + 1)}).`);
};
ACT.abInEnd = (dd) => openDialog('abInEnd', { ch: dd.ch });
DIALOGS.abInEnd = (p) => {
  const n = LIVE.inputs.find((x) => x.ch === p.ch); const i = abInfo(n); const keep = n.reasons.some((r) => ['permanent', 'schedule'].includes(r));
  const parts = [i.vs.length ? `${abPl(i.vs.length, 'Zuschauer wird', 'Zuschauer werden')} unterbrochen` : '', i.recs.length ? `${i.recs.length === 1 ? 'Laufende Aufnahmen bekommen' : abPl(i.recs.length, 'Aufnahme bekommt', 'Aufnahmen bekommen')} eine Lücke oder enden` : ''].filter(Boolean);
  return { title: `${esc(i.c.name)} beenden?`, body: `<p class="lead">${parts.length ? parts.join(', ') + '.' : 'Niemand schaut zu und keine Aufnahme läuft.'} ${keep ? 'Permanente Sender und Sender nach Zeitplan startet Slouchcat danach wieder.' : i.recs.length ? 'Laufende Aufnahmen öffnen den Empfang danach wieder.' : ''}</p><p class="muted">Der Empfang wird sofort getrennt. Bei Aufnahmen siehst du nur die Zahl, nicht wessen Aufnahme es ist. Wer einen Sender ganz aus dem Angebot nehmen will, deaktiviert ihn unter «Sender».</p>`,
    actions: `<button type="button" class="btn secondary" data-act="close" data-fid="ine:x" data-autofocus>Abbrechen</button><button type="button" class="btn danger" data-act="abInEndOk" data-ch="${p.ch}" data-fid="ine:ok">${icon('x', 16)}Beenden</button>` };
};
ACT.abInEndOk = (dd) => {
  const n = LIVE.inputs.find((x) => x.ch === dd.ch); if (!n) return; const i = abInfo(n); const c = i.c;
  i.vs.forEach((v) => { ADM.ended[v.i] = true; });
  const pl = SIM.player(); if (pl && pl.ch === dd.ch) SIM.stop();
  const ts = LIVE.storage.find((s) => s.id === 'timeshift'); if (ts) ts.usedGB = Math.max(0, Math.round((ts.usedGB - n.tsPer * i.vs.length) * 10) / 10);
  abLog(dd.ch, 'info', 'Eingang beendet (Verwaltung)');
  const stays = n.reasons.some((r) => ['permanent', 'schedule'].includes(r)) || i.recs.length > 0;
  if (stays) { n.reasons = n.reasons.filter((r) => r !== 'viewer' && r !== 'preload'); n.since = SIM.now(); n.errors = null; abLog(dd.ch, 'reconnect', 'Empfang neu gestartet (Haltegrund bleibt)'); }
  else { LIVE.inputs.splice(LIVE.inputs.indexOf(n), 1); LIVE.resources.bandIn = Math.max(0, Math.round((LIVE.resources.bandIn - n.bitrate) * 10) / 10); }
  closeDialog(); render(); toast(`${esc(c.name)} beendet.${stays ? ' Slouchcat startet den Empfang neu, weil ein Grund bleibt.' : ''}`);
};

route('/admin/live/streams', () => {
  const m = abMode();
  return abView({ page: 'live-streams', title: 'Aktive Streams', intro: 'Jeder Sender, den Slouchcat gerade empfängt, mit Quelle, Zuschauern, Aufnahmen und Technik.', live: m === 'normal' ? abLive() : null,
    html: abGate(abEmptyBox('Gerade empfängt Slouchcat keinen Sender.', 'Sobald jemand schaut, eine Aufnahme läuft oder ein Sender permanent empfangen wird, erscheint er hier.'), abStreams, { rows: 4, h: 110 }) });
});

/* ======================================================================================
   LA-13 Überwachung und Diagnose
   ====================================================================================== */
const AB_DGE = [['6h', 'alle 6 Stunden'], ['24h', 'täglich'], ['off', 'aus']];
function abDiagNow() {
  const L = LIVE.diagnostics.now.map((x) => Object.assign({}, x, { text: x.text.replace(/nächster Versuch \d\d:\d\d/, `nächster Versuch ${fmt.time(SIM.now() + 2)}`) })); /* relativ zur Mockup-Uhr */
  if (SIM.flags().disturbed) L.push({ ch: 'srf1', severity: 'bad', text: `seit ${fmt.time(SIM.now())} · Bild gestört (Zustand im Mockup)`, mock: true });
  return L.sort((a, b) => ['bad', 'warn'].indexOf(a.severity) - ['bad', 'warn'].indexOf(b.severity));
}
/** LA-13 «Jetzt»: ruhender vorsorglicher Empfang je Quelle und zurückgestellte Dauersender mit Ursache und Versuchen */
function abDiagRest() {
  const rest = ADM_REST.map((x) => admRest(x.src)).filter(Boolean); const def = admInputs().filter((n) => n.state === 'deferred');
  if (!rest.length && !def.length) return '';
  return `<h3 class="la02-h3">Quellen und zurückgestellte Sender</h3><ul class="ab-list">${rest.map((r) => `<li><b>${esc(admSrcName(r.src))}</b> · ${esc(admRestDiag(r))}</li>`).join('')}${def.map((n) => `<li><b>${esc(abChan(n.ch).name)}</b> · ${esc(admDefText(n))} · seit ${fmt.time(n.since)}</li>`).join('')}</ul>
    <p class="muted aa-small">Fristen für ruhenden vorsorglichen Empfang: 30 Minuten, beim 2. Mal 6 Stunden, danach 24 Stunden. Das ist Laufzeitverhalten, keine Einstellung je Quelle.</p>`;
}
function abHeat() {
  const H = LIVE.diagnostics.heat; const hrs = Array.from({ length: 24 }, (_, h) => h);
  return `<div class="tblw ab-heatw" tabindex="0" role="region" aria-label="Ausfälle je Stunde, letzte sieben Tage"><table class="ab-heat"><thead><tr><th scope="col"><span class="vh">Tag</span></th>${hrs.map((h) => `<th scope="col" class="${h % 3 ? 'ab-hx' : ''}">${h % 3 ? `<span class="vh">${h}</span>` : h}</th>`).join('')}</tr></thead>
    <tbody>${H.map((row, i) => `<tr><th scope="row">${fmt.day(LIVE.at(i - 6, '00:00'))}</th>${row.map((v, h) => `<td class="l${Math.min(3, v)}" title="${fmt.day(LIVE.at(i - 6, '00:00'))}, ${h}:00 bis ${h + 1}:00: ${v ? abPl(v, 'Ausfall', 'Ausfälle') : 'keine Ausfälle'}" aria-label="${v ? abPl(v, 'Ausfall', 'Ausfälle') : 'keine Ausfälle'}, ${h} Uhr">${v || ''}</td>`).join('')}</tr>`).join('')}</tbody></table></div>
    <p class="muted aa-small ab-legend"><span class="ab-sw l0"></span>keiner <span class="ab-sw l1"></span>1 <span class="ab-sw l2"></span>2 <span class="ab-sw l3"></span>3 oder mehr Ausfälle in dieser Stunde. Spalten: Stunde des Tages.</p>`;
}
function abLogRows() {
  const D = ui.draft; const t = SIM.now(); const day0 = Math.floor(t / 1440) * 1440; const q = (D.lgQ || '').toLowerCase();
  let rows = LIVE.diagnostics.log.slice().sort((a, b) => b[0] - a[0]).filter((r) => (!D.lgCh || r[1] === D.lgCh) && (!D.lgKind || r[2] === D.lgKind) && (D.lgRange === '1h' ? r[0] >= t - 60 : D.lgRange === 'today' || !D.lgRange ? r[0] >= day0 : true) && (!q || r[3].toLowerCase().includes(q) || (r[1] && abChan(r[1]).name.toLowerCase().includes(q))));
  if (D.lgOnly) rows = rows.filter((r) => r[1] === D.lgOnly);
  return rows;
}
function abLogTable(rows, chFixed) {
  const shown = rows.slice(0, AB.diag.n);
  return rows.length ? `<div class="tblw ab-tbl"><table class="tbl"><thead><tr><th>Zeit</th>${chFixed ? '' : '<th>Sender</th>'}<th>Art</th><th>Meldung</th></tr></thead><tbody>${shown.map(([tm, ch, k, text]) => { const kk = AB_LOGK[k] || ['Info', 'info']; return `<tr><td data-l="Zeit" class="ab-t">${LU.dayOf(tm) === LU.dayOf(SIM.now()) ? '' : fmt.day(tm) + ' '}${fmt.clock(tm)}</td>${chFixed ? '' : `<td class="main" data-l="Sender">${ch ? esc(abChan(ch).name) : '<span class="muted">Server</span>'}</td>`}<td data-l="Art"><span class="chip ${kk[1]}">${kk[0]}</span></td><td data-l="Meldung">${esc(text)}</td></tr>`; }).join('')}</tbody></table></div>
    <div class="row-gap"><span class="muted aa-small">${shown.length} von ${rows.length} Einträgen. Texte ohne Zugangsdaten, Adressen nur als Host.</span>${rows.length > shown.length ? `<button type="button" class="btn secondary sm" data-act="abLogMore" data-fid="la13:more">Weitere anzeigen</button>` : ''}</div>`
    : abEmptyBox('Keine Einträge', 'Zu diesen Filtern gibt es keine Protokollzeilen. Wähle einen längeren Zeitraum.', `<button type="button" class="btn secondary" data-act="abLogReset" data-fid="la13:lgreset">Filter zurücksetzen</button>`);
}
ACT.abLogMore = () => { AB.diag.n += 12; render(); };
ACT.abLogReset = () => { ['lgCh', 'lgKind', 'lgQ', 'lgOnly'].forEach((k) => { ui.draft[k] = ''; }); ui.draft.lgRange = '7d'; AB.diag.n = 12; render(); };
const abDgBase = () => abClone(LIVE.diagnostics.settings);
const abDgSum = (s) => `Prüfen ${s.checkEvery === 'off' ? 'aus' : s.checkEvery === '24h' ? 'täglich' : 'alle 6 Std.'} · Meldung nach ${s.notifyAfter} Min.`;
function abDiagSettings() {
  const base = abDgBase(); const d = abDraft('dg', base); const dirty = abDirty('dg', base); const P = 'ui.draft.dg';
  return `${abRow('Nicht empfangene Sender prüfen', 'Slouchcat prüft Sender, die gerade niemand braucht, ob sie noch erreichbar sind.', `${abSel(`${P}.checkEvery`, 'la13-ce', AB_DGE, { label: 'Prüfen' })}<span class="ab-unit"><span>höchstens</span>${abSel(`${P}.parallel`, 'la13-par', [1, 2, 3, 4].map((x) => [x, String(x)]), { num: true, label: 'Prüfungen gleichzeitig' })}<span>gleichzeitig</span></span>`)}
    ${abRow('Startfristen', 'Nur Anzeige: feste Regeln des Servers.', '<span>Bis zum ersten Byte 15 s je Versuch; bis «läuft» gespeicherte Bildgruppe plus 5 s (15 bis 45 s); bis zum ersten Segment einer Sitzung zusätzlich, zusammen höchstens 90 s.</span>')}
    ${abRow('Prüfung ruhender Sender', 'Nur Anzeige: feste Regeln des Servers.', '<span>Nicht bei Quellen, deren vorsorglicher Empfang gerade ruht. Immer: Lieblingssender, Sender mit Aufnahme in 24 Stunden, Sender mit Zeitplan vor dem Fenster. Übrige nur, wenn unter «Empfang» eingeschaltet. Eine Prüfung zugleich, 5 Sekunden Pause.</span>')}
    ${abRow('Wiederverbindungen bis «gestört»', 'Mit wachsendem Abstand zwischen den Versuchen.', abSel(`${P}.retries`, 'la13-rt', [3, 5, 8, 10].map((x) => [x, `${x} Versuche`]), { num: true, label: 'Wiederverbindungen' }))}
    ${abRow('Administrator benachrichtigen, wenn ein Sender länger ausfällt als', `Wohin die Meldung geht, stellst du unter <a class="lnk" href="#/admin/notifications" data-fid="la13:ntf">Benachrichtigungen</a> ein.`, abSel(`${P}.notifyAfter`, 'la13-na', [5, 15, 30, 60].map((x) => [x, `${x} Minuten`]), { num: true, label: 'Benachrichtigen nach' }))}
    ${abRow('Nur für Sender mit Zuschauern, Aufnahmen oder permanentem Empfang melden', 'Sonst meldet Slouchcat jeden ausgefallenen Sender, auch wenn ihn niemand braucht.', sw(`${P}.onlyUsed`, 'Nur für genutzte Sender melden'))}
    <p class="muted">Slouchcat meldet je Sender höchstens einmal pro Stunde.</p>${dirty ? abSaveBar('abDgSave', 'abDgDisc', 'la13', 0) : ''}`;
}
ACT.abDgDisc = () => { delete ui.draft.dg; render(); };
ACT.abDgSave = () => { LIVE.diagnostics.settings = abClone(ui.draft.dg); delete ui.draft.dg; render(); toast('Einstellungen der Überwachung gespeichert.'); };
const AB_HELP = [
  ['Zeitüberschreitung', 'Die Quelle antwortet nicht in 10 Sekunden.', 'Prüfe, ob der Anbieter erreichbar ist (Quelle testen unter «Quellen»). Läuft der Sender nur zeitweise nicht, gleicht Slouchcat das mit Wiederverbindungen aus.'],
  ['Anmeldung abgelehnt (403)', 'Der Anbieter lehnt die Zugangsdaten oder die Verbindung ab.', 'Prüfe Benutzer und Passwort der Quelle. Erlaubt dein Anbieter nur eine gleichzeitige Verbindung, kann er eine weitere Verbindung (etwa fürs Vorladen) ablehnen. Slouchcat lässt dann den vorsorglichen Empfang dieser Quelle von selbst für eine Frist ruhen (siehe Protokoll). Hat dein Zugang nur eine Verbindung, schalte das Vorladen unter «Empfang» aus.'],
  ['Server antwortet 503', 'Der Dienst des Anbieters ist gerade überlastet oder in Wartung.', 'Meist vorübergehend. Slouchcat bleibt bei der bisherigen Liste und versucht es später wieder.'],
  ['Bild eingefroren', 'Daten kommen an, aber das Bild bewegt sich nicht.', 'Wechsle zur Ersatzquelle oder verbinde neu («Aktive Streams»). Tritt es immer wieder auf, melde es dem Anbieter.'],
  ['Kein Ton', 'Das Bild läuft, aber der Ton fehlt oder ist stumm.', 'Prüfe in «Jetzt prüfen», welche Tonspuren die Quelle liefert. Manchmal hilft die Ersatzquelle.'],
  ['Ziel im Heimnetz nicht freigegeben', 'Eine Adresse im Heimnetz ist für die Quelle nicht freigegeben.', 'Bestätige das Ziel in der Quelle als festes Ziel im Heimnetz (Quelle ändern › Ziele im Heimnetz), aber nur für Geräte, die du kennst.'],
];
function abDiagMain() {
  const now = abDiagNow(); const wk = abWeek(); const P = LIVE.diagnostics.perChannel; const S = AB.diag; const D = ui.draft;
  const nowTbl = now.length ? `<div class="tblw ab-tbl"><table class="tbl"><thead><tr><th>Sender</th><th>Zustand</th><th>Meldung</th><th class="end"><span class="vh">Aktionen</span></th></tr></thead><tbody>${now.map((x) => { const c = abChan(x.ch); const inp = LIVE.inputs.find((n) => n.ch === x.ch); return `<tr><td class="main" data-l="Sender">${abChLabel(c)}</td><td data-l="Zustand"><span class="chip ${x.severity === 'bad' ? 'bad' : 'warn'}">${x.severity === 'bad' ? 'Nicht erreichbar' : 'Fehlerhafter Stream'}</span>${inp && inp.state === 'deferred' ? '<span class="chip info">zurückgestellt</span>' : ''}</td><td data-l="Meldung">${esc(x.text)}</td>
      <td class="end"><div class="ta"><a class="btn secondary" href="#/admin/live/diagnostics?channel=${x.ch}" data-fid="la13:chk:${x.ch}">${icon('search', 16)}Prüfen</a><a class="btn quiet" href="#/admin/live/channels?tab=channels&edit=${x.ch}&open=sources" data-fid="la13:ed:${x.ch}">Sender bearbeiten</a></div></td></tr>`; }).join('')}</tbody></table></div>` : `<div class="notice ok"><span class="ic">${icon('check')}</span><div><b>Alles in Ordnung</b><p>Kein Sender ist gerade gestört.</p></div></div>`;
  const key = { ch: (r) => abChan(r[0]).name.toLowerCase(), out: (r) => r[1], dur: (r) => abParseDur(r[2]), rc: (r) => r[3], sw: (r) => r[4] }[S.sort] || ((r) => r[1]);
  const pc = P.slice().sort((a, b) => (key(a) < key(b) ? -1 : key(a) > key(b) ? 1 : 0) * S.dir);
  const week = `${abHeat()}<h3 class="la02-h3">Je Sender</h3><div class="tblw ab-tbl"><table class="tbl"><thead><tr>${abSortHead(S, 'ch', 'Sender', 'abDgSort')}${abSortHead(S, 'out', 'Ausfälle', 'abDgSort', 'end')}${abSortHead(S, 'dur', 'Dauer gesamt', 'abDgSort', 'end')}${abSortHead(S, 'rc', 'Wiederverbindungen', 'abDgSort', 'end')}${abSortHead(S, 'sw', 'Quellwechsel', 'abDgSort', 'end')}</tr></thead><tbody>${pc.map((r) => `<tr><td class="main" data-l="Sender"><a class="lnk" href="#/admin/live/diagnostics?channel=${r[0]}" data-fid="la13:pc:${r[0]}">${esc(abChan(r[0]).name)}</a></td><td class="end" data-l="Ausfälle">${r[1]}</td><td class="end" data-l="Dauer gesamt">${esc(r[2])}</td><td class="end" data-l="Wiederverbindungen">${r[3]}</td><td class="end" data-l="Quellwechsel">${r[4]}</td></tr>`).join('')}</tbody></table></div><p class="muted aa-small">Kein Qualitätsverlauf über sieben Tage hinaus: Slouchcat bewertet Sender nicht.</p>`;
  const chOpts = [['', 'Alle Sender'], ...[...new Set(LIVE.diagnostics.log.map((r) => r[1]).filter(Boolean))].map((id) => [id, abChan(id).name])];
  const logHtml = `<div class="ab-tools"><div class="fld"><label for="la13-lc">Sender</label>${abSel('ui.draft.lgCh', 'la13-lc', chOpts, { label: 'Sender' })}</div><div class="fld"><label for="la13-lk">Art</label>${abSel('ui.draft.lgKind', 'la13-lk', [['', 'Alle Arten'], ...Object.entries(AB_LOGK).map(([k, v]) => [k, v[0]])], { label: 'Art' })}</div><div class="fld"><label for="la13-lr">Zeitraum</label>${abSel('ui.draft.lgRange', 'la13-lr', [['today', 'Heute'], ['1h', 'Letzte Stunde'], ['7d', '7 Tage']], { label: 'Zeitraum', cur: D.lgRange || 'today' })}</div><div class="fld ab-grow"><label for="la13-lq">Suche</label><input class="inp" id="la13-lq" type="search" data-bind="ui.draft.lgQ" data-live data-fid="la13:lq" value="${esc(D.lgQ || '')}" placeholder="Text oder Sender" autocomplete="off"></div></div>${abLogTable(abLogRows())}`;
  const sets = abDgBase(); const dirty = abDirty('dg', sets);
  const help = `<ul class="ab-help">${AB_HELP.map(([t, w, h]) => `<li><b>${t}</b><span>${w}</span><span class="muted">${h}</span></li>`).join('')}</ul>`;
  return `${admNoContain()}<div class="stats" style="--n:3">${abStat('Jetzt gestört', now.length, now.length ? `${now.filter((x) => x.severity === 'bad').length} nicht erreichbar · ${now.filter((x) => x.severity !== 'bad').length} fehlerhaft` : 'alles in Ordnung', { cls: now.length ? (now.some((x) => x.severity === 'bad') ? 'bad' : 'warn') : 'ok' })}${abStat('Letzte 7 Tage', abPl(wk.outages, 'Ausfall', 'Ausfälle'), `${wk.reconnects} Wiederverbindungen · ${wk.switches} Quellwechsel`)}${abStat('Meldung', `nach ${sets.notifyAfter} Min.`, sets.onlyUsed ? 'nur für genutzte Sender' : 'für jeden aktiven Sender')}</div>
    <section class="card"><div class="ch"><span class="ik">${icon('alert', 18)}</span><h2>Jetzt</h2><button type="button" class="btn secondary sm" data-act="abCheckAll" data-fid="la13:all">${icon('refresh', 16)}Nicht empfangene Sender jetzt prüfen</button></div>${nowTbl}${abDiagRest()}</section>
    ${band('ab:dg:week', 'Letzte 7 Tage', `${abPl(wk.outages, 'Ausfall', 'Ausfälle')} · ${wk.reconnects} Wiederverbindungen · ${wk.switches} Quellwechsel`, week, { icon: 'chart' })}
    <section class="card"><div class="ch"><span class="ik">${icon('list', 18)}</span><h2>Protokoll</h2></div>${logHtml}</section>
    ${band('ab:dg:set', 'Einstellungen der Überwachung', `${abDgSum(sets)}${dirty ? ' · <span class="chip warn">nicht gespeichert</span>' : ''}`, abDiagSettings(), { icon: 'settings' })}
    ${band('ab:dg:help', 'Hilfen zur Fehlersuche', 'Was Meldungen bedeuten und was du tun kannst', help, { icon: 'help' })}`;
}
ACT.abDgSort = (dd) => { abSortToggle(AB.diag, dd.k); render(); };
ACT.abCheckAll = () => {
  ui.busy.checkAll = true; render(); toast('Prüfung gestartet: höchstens ' + LIVE.diagnostics.settings.parallel + ' Sender gleichzeitig.', 'ok', 3000);
  setTimeout(() => { delete ui.busy.checkAll; abLog(null, 'info', `Prüfung der nicht empfangenen Sender abgeschlossen: ${abDiagNow().length} auffällig`); render(); toast(`Prüfung abgeschlossen: ${abPl(abDiagNow().length, 'Sender auffällig', 'Sender auffällig')}.`, 'ok'); }, 1800);
};
/* Diagnose eines Senders */
function abProbe(c, s, i) {
  const host = abHostOf(s.url); const dn = LIVE.diagnostics.now.find((x) => x.ch === c.id) || (SIM.flags().disturbed && c.id === 'srf1' ? { text: 'Bild gestört' } : null);
  if (s.state === 'fail') return { reach: false, msg: i === 0 ? `Zeitüberschreitung nach 10 s · Host ${host}` : `Anmeldung abgelehnt (403) · Host ${host}`, host };
  if (SIM.flags().disturbed && c.id === 'srf1' && i === 0) return { reach: true, ms: 310, ttf: null, frozen: true, msg: `Verbindung steht, aber das Bild bewegt sich nicht · Host ${host}`, host };
  const frozen = dn && /eingefroren/.test(dn.text); const mute = dn && /kein Ton/.test(dn.text);
  return { reach: true, ms: 140 + i * 90 + (c.id.length * 7) % 60, ttf: 620 + i * 140, frozen, mute, msg: frozen ? `Bild eingefroren seit der letzten Prüfung · Host ${host}` : mute ? `Bild läuft, aber kein Ton erkannt · Host ${host}` : `Stream liefert Bild und Ton · Host ${host}`, host };
}
const abProbeText = (c, res) => `Sender: ${c.name} (Nr. ${c.no})\nZustand: ${SIM.chState(c) === 'ok' ? 'in Ordnung' : 'gestört'}\n` + res.map(({ s, i, p }) => `Quelle ${i + 1} «${abSrcName(s.source)}» (${i === 0 ? 'Hauptquelle' : 'Ersatzquelle ' + (i + 1)}): ${p.reach ? `erreichbar, ${p.ms} ms` : 'nicht erreichbar'} · ${p.msg}${p.reach ? `\n  Bild ${c.video} · Ton ${c.audio.join(', ')} · Untertitel ${c.subs.length ? c.subs.join(', ') : 'keine'} · ${abDe(c.bitrate, 1)} Mbit/s` : ''}`).join('\n') + `\nStand: ${fmt.day(SIM.now())} ${fmt.time(SIM.now())} (Mockup, Beispieldaten; Adressen ohne Zugangsdaten)`;
ACT.abProbe = (dd) => {
  const c = SIM.channel(dd.ch); const st = AB.probe[dd.ch] = { running: true, step: 0, res: [] }; render();
  const next = () => {
    if (AB.probe[dd.ch] !== st) return;
    if (st.step >= c.sources.length) { st.running = false; render(); return; }
    const s = c.sources[st.step]; st.res.push({ s, i: st.step, p: abProbe(c, s, st.step) }); st.step++; render(); setTimeout(next, 750);
  };
  setTimeout(next, 600);
};
ACT.abCopy = (dd) => {
  const c = SIM.channel(dd.ch); const res = c.sources.map((s, i) => ({ s, i, p: abProbe(c, s, i) })); const text = abProbeText(c, res);
  const done = () => toast('Technische Angaben kopiert (ohne Zugangsdaten, Adressen nur als Host).');
  try { navigator.clipboard.writeText(text).then(done, () => { ui.draft.copyText = text; openDialog('abCopyText', {}); }); } catch (e) { ui.draft.copyText = text; openDialog('abCopyText', {}); }
};
DIALOGS.abCopyText = () => ({ title: 'Technische Angaben', body: `<p class="lead">Dein Browser erlaubt das Kopieren hier nicht. Markiere den Text und kopiere ihn von Hand.</p><textarea class="inp aa-ta" rows="9" readonly data-fid="cpy:t" data-autofocus aria-label="Technische Angaben">${esc(ui.draft.copyText || '')}</textarea>`, actions: `<button type="button" class="btn secondary" data-act="close" data-fid="cpy:x">Schliessen</button>` });
function abChannelDiag(id) {
  const c = SIM.channel(id); const st = SIM.chState(c); const pr = AB.probe[id]; const inp = LIVE.inputs.find((n) => n.ch === id);
  const state = inp && inp.state === 'reconnecting' ? ['Wiederverbindet', 'bad'] : st !== 'ok' ? ['Gestört', 'bad'] : (LIVE.diagnostics.now.find((x) => x.ch === id) ? ['Fehlerhaft', 'warn'] : ['In Ordnung', 'ok']);
  const probe = !pr ? '<p class="muted">Noch nicht geprüft. «Jetzt prüfen» verbindet einmal mit jeder Quelle, nacheinander, und liest einige Sekunden.</p>'
    : `<div class="ab-probes">${pr.res.map(({ s, i, p }) => `<div class="ab-probe ${p.reach ? (p.frozen || p.mute ? 'warn' : 'ok') : 'bad'}"><div class="ab-ph"><b>Quelle ${i + 1}: ${esc(abSrcName(s.source))}</b><span class="chip ${p.reach ? (p.frozen || p.mute ? 'warn' : 'ok') : 'bad'}">${p.reach ? (p.frozen ? 'Bild eingefroren' : p.mute ? 'Kein Ton' : 'in Ordnung') : 'nicht erreichbar'}</span></div>
        <dl class="kv"><dt>Erreichbar</dt><dd>${p.reach ? 'ja' : 'nein'}</dd>${p.reach ? `<dt>Antwortzeit</dt><dd>${p.ms} ms</dd><dt>Erstes Bild nach</dt><dd>${p.ttf ? p.ttf + ' ms' : 'kein Bild'}</dd><dt>Bild</dt><dd>${esc(c.video)}</dd><dt>Tonspuren</dt><dd>${esc(c.audio.join(', '))}${p.mute ? ' · <span class="ab-bad">kein Ton erkannt</span>' : ''}</dd><dt>Untertitel</dt><dd>${c.subs.length ? esc(c.subs.join(', ')) : 'keine'}</dd><dt>Bitrate</dt><dd>${abDe(c.bitrate, 1)} Mbit/s</dd><dt>Erkennung</dt><dd>${p.frozen ? '<span class="ab-bad">eingefroren</span>' : p.mute ? '<span class="ab-bad">kein Ton</span>' : 'unauffällig'}</dd>` : ''}<dt>Meldung</dt><dd>${esc(p.msg)}</dd></dl></div>`).join('')}${pr.running ? `<div class="aa-testing"><span class="spin"></span>${pr.step < c.sources.length ? `Quelle ${pr.step + 1} wird geprüft …` : 'Fertig …'}</div>` : ''}</div>`;
  const D = ui.draft; D.lgOnly = id;
  return `<section class="card"><div class="ch"><span class="ik">${icon('tv', 18)}</span><h2>${abChLabel(c)}</h2><span class="chip ${state[1]}">${state[0]}</span></div>
    <p>${c.sources.length === 1 ? 'Dieser Sender hat eine Quelle.' : `Dieser Sender hat ${c.sources.length} Quellen. Die erste ist die Hauptquelle.`} Aktuell: ${inp ? `empfangen über «${esc(abSrcName(inp.source))}»` : 'nicht empfangen, niemand braucht ihn'}.</p>
    <div class="row-gap"><button type="button" class="btn primary" data-act="abProbe" data-ch="${id}" data-fid="la13c:probe" ${pr && pr.running ? 'disabled' : ''}>${icon('search', 16)}Jetzt prüfen</button><button type="button" class="btn secondary" data-act="abCopy" data-ch="${id}" data-fid="la13c:copy">${icon('copy', 16)}Technische Angaben kopieren</button><a class="btn ghost" href="#/admin/live/channels?tab=channels&edit=${id}&open=sources" data-fid="la13c:edit">Sender bearbeiten</a></div>${probe}</section>
    <section class="card"><div class="ch"><span class="ik">${icon('list', 18)}</span><h2>Verlauf dieses Senders</h2></div>${abLogTable(LIVE.diagnostics.log.filter((r) => r[1] === id).sort((a, b) => b[0] - a[0]), true)}</section>`;
}
route('/admin/live/diagnostics', (pm, q) => {
  const m = abMode(); const id = q.channel; const c = id && SIM.channel(id);
  if (c) return abView({ page: 'diagnostics', title: `Diagnose: ${c.name}`, h1: `Diagnose: ${esc(c.name)}`, intro: 'Eine einmalige Prüfung jeder Quelle dieses Senders, ohne Zuschauer zu stören.', back: ['#/admin/live/diagnostics', 'Überwachung und Diagnose'], live: m === 'normal' ? abLive() : null, html: abGate(null, () => abChannelDiag(id), { rows: 3, h: 120 }) });
  ui.draft.lgOnly = '';
  return abView({ page: 'diagnostics', title: 'Überwachung und Diagnose', intro: 'Nicht erreichbare Sender, fehlerhafte Streams, Wiederverbindungen und das technische Protokoll.', live: m === 'normal' ? abLive() : null,
    html: abGate(abEmptyBox('Keine Sender, keine Meldungen', 'Sobald Sender eingerichtet sind, erscheinen hier Ausfälle und das Protokoll.', `<a class="btn primary" href="#/admin/live/sources/new" data-fid="la13:addsrc">${icon('plus', 16)}Quelle hinzufügen</a>`), abDiagMain, { rows: 5, h: 90 }) });
});

/* ======================================================================================
   Aufnahmen und Speicher: gemeinsame Teile (Zustand der Speicherorte, Belegung je Konto)
   ====================================================================================== */
/* ui.abSto and ui.abEpg point to the mockup states of the core (admin-core.js, bar «Zustände» → Verwaltung) */
const AB_STO_MOCK = [['normal', 'alles in Ordnung'], ['low', 'Aufnahmen fast voll'], ['full', 'Aufnahmen voll'], ['unmounted', 'Netzlaufwerk der Aufnahmen nicht eingehängt'], ['readonly', 'Timeshift-Ordner nicht beschreibbar'], ['tsfull', 'Timeshift-Grenze erreicht']];
const AB_STS = { ok: ['Bereit', 'ok'], low: ['Fast voll', 'warn'], full: ['Voll', 'bad'], unmounted: ['Netzlaufwerk nicht eingehängt', 'bad'], readonly: ['Nicht beschreibbar', 'bad'] };
const abSto = (id) => LIVE.storage.find((s) => s.id === id);
/** setzt LIVE.storage aus dem gemerkten Stand und dem Zustand des Mockups (LA-01 liest dieselben Werte) */
function abStoApply() {
  LIVE.storage.forEach((s) => { const b = AB.base[s.id]; s.usedGB = b.usedGB; s.state = b.state || 'ok'; });
  const rec = abSto('recordings'); const ts = abSto('timeshift'); const st = ui.abSto;
  if (st === 'low') rec.usedGB = rec.totalGB - 21;
  if (st === 'full') rec.usedGB = rec.totalGB - 14;
  if (st === 'unmounted') rec.state = 'unmounted';
  if (st === 'readonly') ts.state = 'readonly';
  if (st === 'tsfull') ts.usedGB = ts.limitGB;
}
ACT.abStoState = () => { abStoApply(); render(); };
const abReserve = (id, tot) => { const L = LIVE.settings.limits; return id === 'recordings' ? L.reserveRec : id === 'timeshift' ? L.reserveTs : Math.ceil((tot || 2) * L.reserveBufPct / 100 * 10) / 10; };
const abFree = (s) => (s.id === 'timeshift' ? s.freeGB : s.totalGB - s.usedGB);
/** Zustand eines Speicherorts: gemerkter Zustand, sonst aus Platz und Reserve */
function abStoStatus(s) {
  if (s.state === 'unmounted' || s.state === 'readonly') return s.state;
  const res = abReserve(s.id, s.totalGB); const warn = res * (1 + LIVE.settings.limits.warnPct / 100);
  if (s.id === 'timeshift' && s.usedGB >= s.limitGB) return 'full';
  const f = abFree(s); return f <= res ? 'full' : f < warn ? 'low' : 'ok';
}
const abStoExplain = (s, st) => ({
  unmounted: 'Das Netzlaufwerk ist nicht eingehängt. Slouchcat schreibt nicht auf die lokale Platte darunter: Laufende Aufnahmen bekommen eine Lücke, neue Aufnahmen werden nicht angenommen und die Profile bekommen eine Meldung.',
  readonly: s.id === 'timeshift' ? 'Der Ordner ist nicht beschreibbar. Zuschauer sehen ohne Zeitversatz weiter, aber nicht zurückspulen. Slouchcat schreibt nirgends sonst hin.' : 'Der Ordner ist nicht beschreibbar. Slouchcat schreibt nirgends sonst hin.',
  full: s.id === 'recordings' ? 'Unter der Mindestreserve nimmt Slouchcat nicht mehr auf. Es löscht nichts von sich aus. Die betroffenen Profile und das Hauptprofil bekommen eine Meldung.' : s.id === 'timeshift' ? 'Die Grenze ist erreicht. Slouchcat kürzt zuerst den ältesten Teil des grössten Puffers; die betroffenen Zuschauer sehen eine kürzere Rückspulgrenze.' : 'Der Startpuffer ist voll. Neue Sender starten dann ohne schnellen Start.',
  low: 'Der freie Platz nähert sich der Mindestreserve. Gib Platz frei oder erhöhe die Reserve nicht weiter.',
}[st] || '');
function abStoBanner(only) {
  const out = LIVE.storage.filter((s) => (!only || only.includes(s.id))).map((s) => [s, abStoStatus(s)]).filter(([, st]) => st !== 'ok');
  return out.map(([s, st]) => notice(st === 'low' ? 'warn' : 'bad', `${esc(s.name)}: ${AB_STS[st][0]}`, abStoExplain(s, st), `<a class="btn secondary" href="#/admin/live/storage?tab=storage" data-fid="sto:ban:${s.id}">Zu «Speicher»</a>`)).join('');
}
const abMockSto = () => ''; /* storage states: bar «Zustände» → Verwaltung */
/* Belegung je Konto */
const abAcc = () => LIVE.accounts.filter((a) => a.quotaGB > 0 || a.rights.recordings);
function abAccInfo(a) {
  const pct = a.quotaGB ? a.usedGB / a.quotaGB * 100 : 0; const over = a.quotaGB && a.usedGB + a.plannedGB > a.quotaGB;
  const st = !a.quotaGB ? ['Aufnahmen nicht freigeschaltet', 'info'] : pct >= 100 ? ['voll', 'bad'] : pct >= 90 ? ['≥ 90 %', 'warn'] : over ? ['Prognose über Kontingent', 'warn'] : ['in Ordnung', 'ok'];
  return { a, pct, st, over };
}
function abAccTable(compact) {
  const S = AB.aq; const list = abAcc().map(abAccInfo); const key = { acc: (x) => x.a.email, q: (x) => x.a.quotaGB, used: (x) => x.a.usedGB, pct: (x) => x.pct, n: (x) => x.a.count, plan: (x) => x.a.plannedGB }[S.sort] || ((x) => x.pct);
  list.sort((a, b) => (key(a) < key(b) ? -1 : key(a) > key(b) ? 1 : 0) * S.dir);
  const rec = abSto('recordings'); const sumQ = list.reduce((n, x) => n + x.a.quotaGB, 0); const sumU = list.reduce((n, x) => n + x.a.usedGB, 0);
  const rows = list.map((x) => { const a = x.a; return `<tr><td class="main" data-l="Konto">${esc(a.email)}</td><td class="end" data-l="Kontingent">${a.quotaGB ? abGB(a.quotaGB) : '–'}</td><td class="end" data-l="Belegt">${a.quotaGB ? abGB(a.usedGB) : '–'}</td>
    <td data-l="Anteil">${a.quotaGB ? `<span class="ab-pct">${abMeter(a.usedGB, a.quotaGB, `${a.email}: Belegung des Kontingents`)}<b>${Math.round(x.pct)} %</b></span>` : '–'}</td><td class="end" data-l="Gespeicherte Aufnahmen">${a.quotaGB ? fmt.num(a.count) : '–'}</td><td class="end" data-l="Geplant in 7 Tagen">${a.quotaGB ? `ca. ${abGB(a.plannedGB)}` : '–'}</td>
    <td data-l="Zustand"><span class="chip ${x.st[1]}">${x.st[0]}</span></td><td class="end"><button type="button" class="btn quiet" data-act="abQuota" data-email="${esc(a.email)}" data-fid="la16:q:${esc(a.email)}">Kontingent ändern …</button></td></tr>`; }).join('');
  return `<div class="tblw ab-tbl"><table class="tbl"><thead><tr>${abSortHead(S, 'acc', 'Konto', 'abAqSort')}${abSortHead(S, 'q', 'Kontingent', 'abAqSort', 'end')}${abSortHead(S, 'used', 'Belegt', 'abAqSort', 'end')}${abSortHead(S, 'pct', 'Anteil', 'abAqSort')}${abSortHead(S, 'n', 'Gespeicherte Aufnahmen', 'abAqSort', 'end')}${abSortHead(S, 'plan', 'Geplant in 7 Tagen', 'abAqSort', 'end')}<th>Zustand</th><th class="end"><span class="vh">Aktionen</span></th></tr></thead><tbody>${rows}</tbody></table></div>
    <p class="muted aa-small">Zugeteilt ${abGB(sumQ)} von ${abGB(rec.totalGB)} · belegt ${abGB(sumU)}. Titel, Profile und Sender der Aufnahmen siehst du nicht (Privatsphäre).${sumQ > rec.totalGB ? ' <span class="ab-bad">Die Kontingente übersteigen den Speicher.</span>' : ''}</p>`;
}
ACT.abAqSort = (dd) => { abSortToggle(AB.aq, dd.k); render(); };
ACT.abQuota = (dd) => { const a = LIVE.accounts.find((x) => x.email === dd.email); ui.draft.quota = { email: a.email, gb: a.quotaGB, tried: false }; openDialog('abQuota', {}); };
function abQuotaErr(d) { return !Number.isInteger(d.gb) || d.gb < 1 || d.gb > 20000 ? 'Gib eine ganze Zahl von 1 bis 20\'000 ein (in GB).' : ''; }
DIALOGS.abQuota = () => {
  const d = ui.draft.quota; const a = LIVE.accounts.find((x) => x.email === d.email); const e = d.tried ? abQuotaErr(d) : ''; const gb = Number.isInteger(d.gb) ? d.gb : a.quotaGB;
  const sum = abAcc().reduce((n, x) => n + (x.email === a.email ? gb : x.quotaGB), 0); const cap = abSto('recordings').totalGB;
  return { title: `Kontingent von ${esc(a.email)}`, body: `<p class="lead">Aufnahmen belegen im Moment ${abGB(a.usedGB)} (${abPl(a.count, 'gespeicherte Aufnahme', 'gespeicherte Aufnahmen')}). Mehr siehst du als Administrator nicht.</p>
    ${abFld('quota-gb', 'Kontingent für Aufnahmen', `<span class="ab-unit">${abInp('ui.draft.quota.gb', 'quota-gb', { num: true, min: 1, err: e, auto: true, enter: 'abQuotaSave', label: 'Kontingent in GB' })}<span>GB</span></span>`, { err: e, hint: 'Die Profile des Kontos teilen sich dieses Kontingent.' })}
    ${gb < a.usedGB ? notice('warn', 'Unter der heutigen Belegung', 'Slouchcat löscht nichts. Es nimmt aber keine neuen Aufnahmen an, bis die Belegung unter dem Kontingent liegt. Die Profile bekommen eine Meldung.') : ''}
    ${sum > cap ? notice('', 'Die Kontingente übersteigen den Speicher', `Zugeteilt wären ${abGB(sum)} bei ${abGB(cap)} Speicher. Das ist erlaubt, solange nicht alle Konten ihr Kontingent füllen.`) : ''}`,
    actions: `<button type="button" class="btn secondary" data-act="close" data-fid="quota:x">Abbrechen</button><button type="button" class="btn primary" data-act="abQuotaSave" data-fid="quota:ok">Speichern</button>` };
};
ACT.abQuotaSave = () => {
  const d = ui.draft.quota; d.tried = true; if (abQuotaErr(d)) { render(); return; }
  const a = LIVE.accounts.find((x) => x.email === d.email); a.quotaGB = d.gb; if (a.email === LIVE.account.email) LIVE.account.quotaGB = d.gb; delete ui.draft.quota; closeDialog(); render(); toast(`Kontingent von ${esc(a.email)}: ${abGB(a.quotaGB)}.`);
};

/* ======================================================================================
   LA-14 Aufnahmen (DVR-Einstellungen)
   ====================================================================================== */
const abDvrBase = () => { const d = LIVE.settings.dvr; return Object.assign(abClone(d), { mpMode: d.maxParallel == null ? 'none' : 'limit', mpN: d.maxParallel == null ? 4 : d.maxParallel }); };
function abDvrErr(d) { const e = {}; if (d.mpMode === 'limit' && (!Number.isInteger(d.mpN) || d.mpN < 1 || d.mpN > 32)) e.mp = 'Gib eine ganze Zahl von 1 bis 32 ein.'; return e; }
function abRecProblems() {
  const rows = LIVE.profiles.flatMap((p) => SIM.recordings(p.id)).filter((r) => ['failed', 'incomplete', 'rejected'].includes(r.state)).sort((a, b) => b.start - a.start);
  return rows.length ? `<div class="tblw ab-tbl"><table class="tbl"><thead><tr><th>Kennung</th><th>Sender</th><th>Zeit</th><th>Zustand</th><th>Grund</th></tr></thead><tbody>${rows.map((r) => `<tr><td class="main mono" data-l="Kennung">${abAnon(r.id)}</td><td data-l="Sender">${esc(abChan(r.ch).name)}</td><td data-l="Zeit">${fmt.day(r.start)} ${fmt.time(r.start)}</td><td data-l="Zustand"><span class="chip ${r.state === 'incomplete' ? 'warn' : 'bad'}">${LU.REC_STATE[r.state]}</span></td><td data-l="Grund">${esc(LU.REASON[r.reason] || '–')}</td></tr>`).join('')}</tbody></table></div><p class="muted aa-small">Nur Kennung, Sender, Zeit und Grund. Titel und Konto einer Aufnahme siehst du nicht; sie gehören dem Profil.</p>` : '<p class="muted">Keine Aufnahme mit Problem.</p>';
}
function abDvrTab() {
  const base = abDvrBase(); const d = abDraft('dvr', base); const e = abDvrErr(d); const nerr = Object.keys(e).length; const dirty = abDirty('dvr', base); const P = 'ui.draft.dvr'; const R = LIVE.resources;
  const RS = aaRecStats(); const run = RS.run; const chs = RS.chs; /* dieselbe Zählung wie LA-01 und LA-12 */
  const rows = [
    abRow('Standard-Vorlauf', 'Wie früh vor dem Beginn die Aufnahme startet.', abSel(`${P}.padBefore`, 'la14-pb', [0, 1, 2, 5, 10, 15, 30].map((x) => [x, x ? `${x} ${x === 1 ? 'Minute' : 'Minuten'}` : 'keiner']), { num: true, label: 'Standard-Vorlauf' })),
    abRow('Standard-Nachlauf', 'Wie lange nach dem Ende die Aufnahme weiterläuft.', abSel(`${P}.padAfter`, 'la14-pa', [0, 5, 10, 15, 30, 60].map((x) => [x, x ? `${x} Minuten` : 'keiner']), { num: true, label: 'Standard-Nachlauf' })),
    abRow('Standard-Aufbewahrung', 'Gilt für neue Aufnahmen. Jedes Profil kann sie je Aufnahme ändern.', abSel(`${P}.keep`, 'la14-kp', [[7, '7 Tage'], [14, '14 Tage'], [30, '30 Tage'], [90, '90 Tage'], ['forever', 'bis das Profil löscht']], { label: 'Standard-Aufbewahrung', cur: d.keep })),
    abRow('Längste Sofortaufnahme «bis ich stoppe»', 'Danach endet die Aufnahme von selbst.', abSel(`${P}.instantMaxH`, 'la14-im', Array.from({ length: 12 }, (_, i) => [i + 1, `${i + 1} ${i ? 'Stunden' : 'Stunde'}`]), { num: true, label: 'Längste Sofortaufnahme' })),
    abRow('Geplante Aufnahmen an Programmänderungen anpassen', 'Laufende Aufnahmen werden verlängert, nie gekürzt.', sw(`${P}.followEpg`, 'Geplante Aufnahmen an Programmänderungen anpassen')),
    abRow('Termine im Voraus planen', 'Nur Anzeige. Der Vorschauzeitraum steht bei den EPG-Quellen.', `<span>so weit wie die Programmdaten: ${LIVE.settings.epg.previewDays} Tage</span><a class="lnk" href="#/admin/live/epg?tab=sources" data-fid="la14:epg">Ändern unter EPG</a>`),
    abRow('Höchstens gleichzeitige Aufnahmen auf dem Server', 'Schützt die Hardware, nicht den Anbieter. Slouchcat kennt kein Limit des Anbieters.', `<div class="stack">${abRadio(`${P}.mpMode`, 'dvr-mp', 'none', 'keine Grenze', 'la14:mp:none')}${abRadio(`${P}.mpMode`, 'dvr-mp', 'limit', 'Grenze', 'la14:mp:limit')}${d.mpMode === 'limit' ? `<span class="ab-unit ab-ind">${abInp(`${P}.mpN`, 'la14-mpn', { num: true, min: 1, max: 32, err: e.mp, label: 'Höchstens gleichzeitige Aufnahmen' })}<span>gleichzeitig</span></span>${abErr('la14-mpn', e.mp)}` : ''}</div>`, { col: true }),
  ].join('');
  const q = `${abAccTable(true)}<p class="muted aa-small"><a class="lnk" href="#/admin/live/storage?tab=storage" data-fid="la14:to16">Standard-Kontingent, Reserve und Speicherorte unter «Speicher»</a></p>`;
  return `${abMockSto()}${abStoBanner(['recordings'])}${admNoContain()}${notice('', 'So arbeiten Aufnahmen', 'Aufnahmen speichern das Signal in Originalqualität, ohne Neucodierung. Alle empfangenen Ton- und Untertitelspuren bleiben erhalten. Gelöscht wird automatisch nur, was ein Profil selbst eingestellt hat: Aufbewahrungsfrist und «behalte die letzten N Folgen». Ist das Kontingent eines Kontos voll, nimmt Slouchcat nicht auf und meldet es dem Profil und dem Hauptprofil. Aufnahmen laufen auch, wenn niemand angemeldet ist.')}
    <div class="stats" style="--n:4">${abStat('Laufen jetzt', run, `auf ${abPl(chs, 'Sender', 'Sendern')}`, { href: '#/admin/live/streams', fid: 'la14:s-run' })}${abStat('Geplant', R.recPlanned24h, 'in den nächsten 24 Stunden')}${abStat('Gleichzeitig höchstens', R.recMaxParallel, d.mpMode === 'limit' ? `Grenze: ${d.mpN}` : 'keine Grenze eingestellt')}${abStat('Konten mit Aufnahmen', abAcc().filter((a) => a.rights.recordings).length, `${abAcc().filter((a) => abAccInfo(a).pct >= 90).length} über 90 % belegt`)}</div>
    <section class="card"><div class="ch"><span class="ik">${icon('record', 18)}</span><h2>Vorgaben für Aufnahmen</h2></div>${rows}</section>
    ${band('ab:dvr:q', 'Kontingente je Konto', `${abPl(abAcc().filter((a) => a.rights.recordings).length, 'Konto', 'Konten')} · Standard ${LIVE.settings.limits.defaultQuotaGB} GB`, q, { icon: 'database' })}
    ${band('ab:dvr:p', 'Aufnahmen mit Problem (nur technisch)', `${abPl(LIVE.profiles.flatMap((p) => SIM.recordings(p.id)).filter((r) => ['failed', 'incomplete', 'rejected'].includes(r.state)).length, 'Fall', 'Fälle')}`, abRecProblems(), { icon: 'alert' })}
    ${dirty ? abSaveBar('abDvrSave', 'abDvrDisc', 'la14', nerr) : ''}`;
}
ACT.abDvrDisc = () => { delete ui.draft.dvr; render(); };
ACT.abDvrSave = () => {
  const d = ui.draft.dvr; if (Object.keys(abDvrErr(d)).length) return;
  const { mpMode, mpN, ...rest } = d; LIVE.settings.dvr = Object.assign(rest, { keep: rest.keep === 'forever' ? 'forever' : Number(rest.keep), maxParallel: mpMode === 'none' ? null : mpN });
  delete ui.draft.dvr; render(); toast('Einstellungen für Aufnahmen gespeichert. Bereits geplante Aufnahmen behalten ihre Werte.');
};

/* ======================================================================================
   LA-15 Timeshift
   ====================================================================================== */
const abTsBase = () => ({ maxMin: LIVE.settings.timeshift.maxMin, totalGB: LIVE.settings.timeshift.totalGB });
function abTsErr(d) { const e = {}; const ts = abSto('timeshift'); if (!Number.isInteger(d.totalGB) || d.totalGB < 1) e.gb = 'Gib eine ganze Zahl in GB ein (mindestens 1).'; else if (d.totalGB > ts.freeGB + ts.usedGB) e.gb = `Mehr als der freie Platz: höchstens ${abGB(ts.freeGB + ts.usedGB)}.`; return e; }
function abTimeshift() {
  const base = abTsBase(); const d = abDraft('ts', base); const e = abTsErr(d); const nerr = Object.keys(e).length; const dirty = abDirty('ts', base); const P = 'ui.draft.ts'; const ts = abSto('timeshift');
  const bufGB = d.maxMin * 0.06; const fit = Math.floor((Number.isInteger(d.totalGB) ? d.totalGB : 0) / Math.max(0.01, bufGB)); const n = admViewers().length;
  const tsOn = LIVE.accounts.filter((a) => a.rights.liveTv && a.rights.timeshift).length; const accN = LIVE.accounts.filter((a) => a.rights.liveTv).length;
  const used = ts.usedGB; const lim = LIVE.settings.timeshift.totalGB;
  return `${abMockSto()}${abStoBanner(['timeshift'])}${admNoContain()}${notice('', 'Jeder Zuschauer hat seinen eigenen Puffer', 'Der Puffer beginnt beim Einschalten. Beim Senderwechsel und am Ende der Wiedergabe wird er gelöscht. Es gibt keinen gemeinsamen Puffer und kein Archiv. Der kurze Startpuffer für schnelles Umschalten ist davon getrennt und nicht einstellbar.')}
    <div class="stats" style="--n:4">${abStat('Puffer jetzt', abPl(n, 'Puffer', 'Puffer'), 'einer je Zuschauer')}${abStat('Belegt', abGB(used), `von ${abGB(lim)} Grenze`, { cls: used >= lim ? 'bad' : used >= lim * 0.8 ? 'warn' : '' })}${abStat('Ein voller Puffer', `ca. ${abDe(bufGB, 1)} GB`, `bei 8 Mbit/s und ${abDur(d.maxMin)}`)}${abStat('Platz für', abPl(fit, 'volle Puffer', 'volle Puffer'), 'innerhalb der Grenze', { cls: fit < 4 ? 'warn' : '' })}</div>
    <section class="card"><div class="ch"><span class="ik">${icon('clock', 18)}</span><h2>Einstellungen</h2></div>
      ${abRow('Höchstdauer je Zuschauer', `So weit kann ein Zuschauer höchstens zurückspulen. Vor das Einschalten geht es nie. Werksvorgabe ${abDur(LIVE.settings.timeshift.defaultMin || 60)}, auf diesem Server eingestellt ${abDur(LIVE.settings.timeshift.maxMin)}`, abSel(`${P}.maxMin`, 'la15-mm', [[15, '15 Minuten'], [30, '30 Minuten'], [60, '1 Stunde (Vorgabe)'], [120, '2 Stunden'], [240, '4 Stunden']], { num: true, label: 'Höchstdauer je Zuschauer' }))}
      ${abRow('Timeshift-Speicher insgesamt höchstens', `Alle Puffer zusammen. Auf dem Speicherort sind ${abGB(ts.freeGB)} frei.`, `<span class="ab-unit">${abInp(`${P}.totalGB`, 'la15-gb', { num: true, min: 1, err: e.gb, label: 'Timeshift-Speicher in GB' })}<span>GB</span></span>${abErr('la15-gb', e.gb)}`)}
      ${abRow('Wenn die Grenze erreicht ist', 'Nur Anzeige.', '<span>Slouchcat kürzt zuerst den ältesten Teil des grössten Puffers. Die betroffenen Zuschauer sehen die kürzere Rückspulgrenze.</span>')}
      ${abRow('Wer Timeshift nutzen darf', 'Das stellst du bei den Konten und Profilen ein.', `<span>${tsOn} von ${accN} Konten mit Live-TV</span><a class="lnk" href="#/admin/accounts" data-fid="la15:acc">Zu den Konten</a>`)}</section>
    <p class="ab-calc">${icon('info', 16)}<span>Bei 8 Mbit/s braucht ein voller Puffer von ${abDur(d.maxMin)} etwa <b>${abDe(bufGB, 1)} GB</b>. Mit ${abGB(Number.isInteger(d.totalGB) ? d.totalGB : 0)} Grenze haben ${fit} Zuschauer gleichzeitig ihren vollen Puffer.</span></p>
    ${dirty ? abSaveBar('abTsSave', 'abTsDisc', 'la15', nerr) : ''}`;
}
ACT.abTsDisc = () => { delete ui.draft.ts; render(); };
ACT.abTsSave = () => {
  const d = ui.draft.ts; if (Object.keys(abTsErr(d)).length) return;
  LIVE.settings.timeshift = { maxMin: d.maxMin, defaultMin: LIVE.settings.timeshift.defaultMin || 60, totalGB: d.totalGB }; LIVE.timeshiftMaxMin = d.maxMin; const ts = abSto('timeshift'); ts.limitGB = d.totalGB; AB.base.timeshift.limitGB = d.totalGB;
  delete ui.draft.ts; abStoApply(); render(); toast(`Timeshift: höchstens ${abDur(d.maxMin)} je Zuschauer, ${abGB(d.totalGB)} insgesamt. Laufende Puffer behalten ihre Länge.`);
};

/* ======================================================================================
   LA-16 Speicher
   ====================================================================================== */
const AB_LOCICON = { buffer: 'zap', timeshift: 'clock', recordings: 'record' };
function abStoCard(s) {
  const st = abStoStatus(s); const k = AB_STS[st]; const res = abReserve(s.id, s.totalGB); const bad = st !== 'ok';
  const cap = s.id === 'timeshift' ? s.limitGB : s.totalGB; const f = abFree(s);
  const nums = s.id === 'timeshift' ? `${abGB(s.usedGB)} belegt von ${abGB(s.limitGB)} Grenze · ${abGB(s.freeGB)} frei auf dem Datenträger` : st === 'unmounted' ? `Grösse unbekannt, ${abGB(s.usedGB)} zuletzt belegt` : `${abGB(s.usedGB)} von ${abGB(s.totalGB)} belegt · ${abGB(f)} frei`;
  return `<section class="card ab-loc${bad ? ' bad' : ''}"><div class="ch"><span class="ik">${icon(AB_LOCICON[s.id], 18)}</span><h2>${esc(s.name)}</h2><span class="chip ${k[1]}">${k[0]}</span></div>
    <dl class="kv"><dt>Pfad</dt><dd><span class="mono">${esc(s.path)}</span></dd><dt>Art</dt><dd>${esc(s.kind)}</dd><dt>Belegung</dt><dd>${nums}</dd><dt>Mindestens frei</dt><dd>${abGB(res)}</dd></dl>
    ${abMeter(s.usedGB, cap, `${s.name}: Belegung`)}
    ${s.id === 'buffer' && !/RAM|tmpfs/.test(s.kind) ? `<p class="ab-bad">${icon('alert', 14)}Der Startpuffer liegt auf einem Datenträger. Eine RAM-Disk ist besser: Stockt der Ordner, hat der Sender keinen schnellen Start.</p>` : ''}
    ${bad ? `<p class="${st === 'low' ? 'ab-warn' : 'ab-bad'}">${icon('alert', 14)}${abStoExplain(s, st)}</p>` : ''}
    ${(s.old || []).length ? `<p class="muted aa-small">Alter Ort, nur lesend eingebunden, bis er leer ist: ${s.old.map((o) => `<span class="mono">${esc(o.path)}</span> (${abGB(o.usedGB)})`).join(', ')}</p>` : ''}
    <div class="foot"><button type="button" class="btn secondary" data-act="abStoCheck" data-id="${s.id}" data-fid="la16:chk:${s.id}">${icon('check', 16)}Prüfen</button><button type="button" class="btn secondary" data-act="abMove" data-id="${s.id}" data-fid="la16:mv:${s.id}">${icon('folder', 16)}Ändern …</button>${s.id === 'timeshift' ? `<a class="btn ghost" href="#/admin/live/storage?tab=timeshift" data-fid="la16:ts">Grenze ändern</a>` : ''}</div></section>`;
}
ACT.abStoCheck = (dd) => { AB.chk = { id: dd.id, done: false }; openDialog('abStoCheck', {}); setTimeout(() => { if (AB.chk && AB.chk.id === dd.id) { AB.chk.done = true; if (ui.dialog && ui.dialog.name === 'abStoCheck') render(); } }, 900); };
DIALOGS.abStoCheck = () => {
  const s = abSto(AB.chk.id); const st = abStoStatus(s); const res = abReserve(s.id, s.totalGB); const net = /Netzlaufwerk/.test(s.kind);
  const rows = AB.chk.done ? [
    [st === 'readonly' ? 'bad' : 'ok', 'Ordner vorhanden und beschreibbar', st === 'readonly' ? 'Die Probedatei konnte nicht angelegt werden: Nur-Lese-Zugriff.' : 'Probedatei angelegt und wieder gelöscht.'],
    [st === 'unmounted' ? 'bad' : st === 'full' ? 'bad' : st === 'low' ? 'warn' : 'ok', 'Freier Platz', st === 'unmounted' ? 'nicht lesbar, weil das Laufwerk fehlt' : `${abGB(abFree(s))} frei, Mindestreserve ${abGB(res)}`],
    ['ok', `Art erkannt: ${s.kind}`, 'Die Markerdatei dieses Ortes stimmt mit der gespeicherten Kennung überein.'],
    ...(net ? [[st === 'unmounted' ? 'bad' : 'ok', st === 'unmounted' ? 'Netzlaufwerk nicht eingehängt' : 'Netzlaufwerk eingehängt', st === 'unmounted' ? 'Slouchcat schreibt nicht auf die lokale Platte darunter.' : 'Markierungsdatei gefunden, Gerät verschieden vom Elternordner.']] : []),
    ...(s.id === 'buffer' ? [[/RAM|tmpfs/.test(s.kind) ? 'ok' : 'warn', 'RAM-Disk', /RAM|tmpfs/.test(s.kind) ? 'Der Startpuffer liegt im Arbeitsspeicher.' : 'Eine RAM-Disk wird empfohlen.']] : []),
  ] : [['wait', 'Slouchcat prüft …', 'Probedatei, freier Platz, Art, Einbindung']];
  return { title: `Speicherort prüfen: ${esc(s.name)}`, body: `<p class="lead"><span class="mono">${esc(s.path)}</span></p><ul class="checks" aria-live="polite">${rows.map(([k, t, x]) => `<li class="${k}">${icon(k === 'ok' ? 'check' : k === 'bad' ? 'x' : k === 'warn' ? 'alert' : 'clock', 18)}<span><b>${t}</b><small>${x}</small></span></li>`).join('')}</ul>`, actions: `<button type="button" class="btn secondary" data-act="close" data-fid="stc:x" data-autofocus>Schliessen</button>` };
};
/* Grenzen */
const abLimBase = () => { const L = LIVE.settings.limits; return { recMode: L.recTotal == null ? 'none' : 'limit', recN: L.recTotal == null ? 2000 : L.recTotal, reserveRec: L.reserveRec, reserveTs: L.reserveTs, reserveBufPct: L.reserveBufPct, warnPct: L.warnPct, defaultQuotaGB: L.defaultQuotaGB }; };
function abLimErr(d) {
  const e = {}; const ok = (v, a, b) => Number.isInteger(v) && v >= a && v <= b; const rec = abSto('recordings');
  if (d.recMode === 'limit') { if (!ok(d.recN, 1, 100000)) e.recN = 'Gib eine ganze Zahl in GB ein.'; else if (d.recN > rec.totalGB) e.recN = `Mehr als der Speicher: ${abGB(rec.totalGB)}.`; }
  if (!ok(d.reserveRec, 0, 2000)) e.reserveRec = 'Gib eine ganze Zahl von 0 bis 2000 ein.';
  if (!ok(d.reserveTs, 0, 2000)) e.reserveTs = 'Gib eine ganze Zahl von 0 bis 2000 ein.';
  if (!ok(d.reserveBufPct, 0, 50)) e.reserveBufPct = 'Gib eine ganze Zahl von 0 bis 50 ein.';
  if (!ok(d.defaultQuotaGB, 1, 20000)) e.defaultQuotaGB = 'Gib eine ganze Zahl in GB ein (mindestens 1).';
  return e;
}
function abLimits() {
  const base = abLimBase(); const d = abDraft('lim', base); const e = abLimErr(d); const nerr = Object.keys(e).length; const dirty = abDirty('lim', base); const P = 'ui.draft.lim';
  const w = (r) => abGB(Math.round(r * (1 + d.warnPct / 100) * 10) / 10);
  return `${abRow('Aufnahmen insgesamt höchstens', 'Alle Konten zusammen. Ohne Grenze gilt nur der freie Platz.', `<div class="stack">${abRadio(`${P}.recMode`, 'lim-rec', 'none', 'ohne Grenze', 'la16:rm:none')}${abRadio(`${P}.recMode`, 'lim-rec', 'limit', 'Grenze', 'la16:rm:limit')}${d.recMode === 'limit' ? `<span class="ab-unit ab-ind">${abInp(`${P}.recN`, 'la16-rn', { num: true, min: 1, err: e.recN, label: 'Aufnahmen insgesamt höchstens in GB' })}<span>GB</span></span>${abErr('la16-rn', e.recN)}` : ''}</div>`, { col: true })}
    ${abRow('Mindestens frei lassen', 'Je Speicherort. Darunter legt Slouchcat nichts Neues an.', `<div class="ab-res"><label>Aufnahmen<span class="ab-unit">${abInp(`${P}.reserveRec`, 'la16-rr', { num: true, min: 0, err: e.reserveRec, label: 'Mindestreserve Aufnahmen in GB' })}<span>GB</span></span>${abErr('la16-rr', e.reserveRec)}</label><label>Timeshift<span class="ab-unit">${abInp(`${P}.reserveTs`, 'la16-rt', { num: true, min: 0, err: e.reserveTs, label: 'Mindestreserve Timeshift in GB' })}<span>GB</span></span>${abErr('la16-rt', e.reserveTs)}</label><label>Startpuffer<span class="ab-unit">${abInp(`${P}.reserveBufPct`, 'la16-rb', { num: true, min: 0, err: e.reserveBufPct, label: 'Mindestreserve Startpuffer in Prozent' })}<span>%</span></span>${abErr('la16-rb', e.reserveBufPct)}</label></div>`)}
    ${abRow('Warnen, wenn frei weniger als', `Mindestreserve plus ein Zuschlag. Heute: Aufnahmen ${w(d.reserveRec)}, Timeshift ${w(d.reserveTs)}.`, abSel(`${P}.warnPct`, 'la16-wp', [0, 10, 20, 50].map((x) => [x, x ? `Mindestreserve + ${x} %` : 'Mindestreserve']), { num: true, label: 'Warnen' }))}
    ${abRow('Standard-Kontingent für neue Konten', `Etwa ${fmt.dec(admHours(100, 8), 1)} Stunden HD bei 8 Mbit/s je 100 GB (3.6 GB je Stunde). Im Konto änderbar.`, `<span class="ab-unit">${abInp(`${P}.defaultQuotaGB`, 'la16-dq', { num: true, min: 1, err: e.defaultQuotaGB, label: 'Standard-Kontingent in GB' })}<span>GB</span></span>${abErr('la16-dq', e.defaultQuotaGB)}`)}
    ${dirty ? abSaveBar('abLimSave', 'abLimDisc', 'la16', nerr) : ''}`;
}
ACT.abLimDisc = () => { delete ui.draft.lim; render(); };
ACT.abLimSave = () => { const d = ui.draft.lim; if (Object.keys(abLimErr(d)).length) return; Object.assign(LIVE.settings.limits, { recTotal: d.recMode === 'none' ? null : d.recN, reserveRec: d.reserveRec, reserveTs: d.reserveTs, reserveBufPct: d.reserveBufPct, warnPct: d.warnPct, defaultQuotaGB: d.defaultQuotaGB }); delete ui.draft.lim; render(); toast('Grenzen und Reserven gespeichert.'); };
ACT.abClean = () => {
  if (AB.orphanBusy) return; AB.orphanBusy = true; render();
  setTimeout(() => { const g = AB.orphan; AB.orphan = 0; AB.orphanAt = SIM.now(); AB.orphanBusy = false; abLog(null, 'info', `Aufgeräumt: ${abDe(g, 1)} GB verwaiste Dateien entfernt`); render(); toast(`${abDe(g, 1)} GB verwaiste Dateien entfernt. Laufende Aufnahmen blieben unberührt.`); }, 1200);
};
function abStorage() {
  const cards = LIVE.storage.map(abStoCard).join('');
  const clean = `<p>Nach einem Absturz können Pufferdateien liegen bleiben. Slouchcat räumt sie beim Start und jede Nacht auf. Laufende Aufnahmen sind immer geschützt.</p><div class="row-gap"><button type="button" class="btn secondary" data-act="abClean" data-fid="la16:clean" ${AB.orphanBusy || !AB.orphan ? 'disabled' : ''}>${AB.orphanBusy ? '<span class="spin"></span>räumt auf …' : `${icon('trash', 16)}Jetzt aufräumen`}</button><span class="muted">${AB.orphan ? `Verwaiste Dateien: ${abDe(AB.orphan, 1)} GB` : 'Keine verwaisten Dateien.'}</span></div>`;
  return `${abMockSto()}${abStoBanner()}<div class="ab-locs">${cards}</div>
    <section class="card"><div class="ch"><span class="ik">${icon('settings', 18)}</span><h2>Grenzen</h2></div>${abLimits()}</section>
    <section class="card"><div class="ch"><span class="ik">${icon('users', 18)}</span><h2>Belegung je Konto</h2></div>${abEmpty() ? abEmptyBox('Noch keine Aufnahmen', 'Sobald ein Konto mit dem Recht «Aufnahmen» aufnimmt, steht hier seine Belegung. Rechte und Kontingent stellst du bei den Konten ein.', `<a class="btn secondary" href="#/admin/accounts" data-fid="la16:acc0">Zu den Konten</a>`) : abAccTable(false)}</section>
    ${band('ab:sto:clean', 'Aufräumen', `Verwaiste Dateien: ${abDe(AB.orphan, 1)} GB · zuletzt ${abWhen(AB.orphanAt)}`, clean, { icon: 'trash' })}`;
}
/* Speicherort wechseln, Schritt für Schritt */
const AB_PATHS = [
  { path: '/mnt/aufnahmen-neu', kind: 'Netzlaufwerk (SMB)', totalGB: 8000, freeGB: 3400, note: 'Beispiel: Erfolg' },
  { path: '/mnt/ssd2/live', kind: 'SSD (ext4)', totalGB: 1800, freeGB: 1600, note: 'Beispiel: Erfolg' },
  { path: '/dev/shm/slouchcat', kind: 'RAM-Disk (tmpfs)', totalGB: 2, freeGB: 1.8, note: 'Beispiel: RAM-Disk' },
  { path: '/mnt/usb-extern', kind: 'USB-Datenträger (exFAT)', totalGB: 2000, freeGB: 1500, ro: true, note: 'Beispiel: nicht beschreibbar' },
  { path: '/mnt/nas-offline', kind: 'Netzlaufwerk (SMB)', totalGB: 6000, freeGB: 5000, unmounted: true, note: 'Beispiel: nicht eingehängt' },
  { path: '/mnt/klein', kind: 'SSD (ext4)', totalGB: 40, freeGB: 12, note: 'Beispiel: zu wenig Platz' },
  { path: '/media', kind: 'Medienordner (schreibgeschützt eingebunden)', totalGB: 12000, freeGB: 3000, media: true, ro: true, note: 'Beispiel: Medienordner' },
];
const abMvEntry = (id) => { const s = abSto(id); const cur = { path: s.path, kind: s.kind, totalGB: s.totalGB || 4000, freeGB: abFree(s), current: true, note: 'aktueller Ort' }; return [cur, ...AB_PATHS.filter((p) => p.path !== s.path)]; };
function abMvChecks(id, e) {
  const res = abReserve(id, id === 'buffer' ? 2 : e.totalGB); const other = LIVE.storage.find((s) => s.id !== id && s.path === e.path); const out = [];
  out.push(e.ro ? ['bad', 'Ordner vorhanden und beschreibbar', e.media ? 'Der Ordner ist schreibgeschützt eingebunden.' : 'Die Probedatei konnte nicht angelegt werden: Nur-Lese-Zugriff.'] : ['ok', 'Ordner vorhanden und beschreibbar', 'Probedatei angelegt und wieder gelöscht.']);
  out.push(e.unmounted ? ['bad', 'Freier Platz', 'nicht lesbar, weil das Laufwerk fehlt'] : e.freeGB < res ? ['bad', 'Freier Platz', `Nur ${abGB(e.freeGB)} frei, die Mindestreserve ist ${abGB(res)}.`] : ['ok', 'Freier Platz', `${abGB(e.freeGB)} frei, Mindestreserve ${abGB(res)}.`]);
  out.push(['ok', `Art erkannt: ${e.kind}`, 'Slouchcat legt eine Markerdatei mit zufälliger Kennung an.']);
  if (/Netzlaufwerk/.test(e.kind)) out.push(e.unmounted ? ['bad', 'Netzlaufwerk nicht eingehängt', 'Slouchcat schreibt nicht auf die lokale Platte darunter.'] : ['ok', 'Netzlaufwerk eingehängt', 'Markierungsdatei gefunden, Gerät verschieden vom Elternordner.']);
  out.push(e.media ? ['bad', 'Nicht der Medienordner', 'Das ist der Medienordner. Er ist schreibgeschützt eingebunden und kein Speicherort.'] : other ? ['warn', 'Anderer Speicherort am selben Ort', `Dieser Ordner dient schon als «${other.name}». Das ist erlaubt, teilt aber den Platz.`] : ['ok', 'Nicht der Medienordner, kein anderer Speicherort', 'Der Ordner ist frei.']);
  if (id === 'buffer') out.push(/RAM/.test(e.kind) ? ['ok', 'RAM-Disk', 'Der Startpuffer liegt im Arbeitsspeicher.'] : ['warn', 'RAM-Disk empfohlen', 'Auf einem Datenträger kann der Ordner stocken. Dann hat der Sender keinen schnellen Start.']);
  return out;
}
ACT.abMove = (dd) => { ui.draft.mv = { id: dd.id, step: 1, path: '', shown: 0 }; openDialog('abMove', {}); };
ACT.abMvPick = () => { render(); };
ACT.abMvNext = () => {
  const m = ui.draft.mv; if (m.step === 1 && !m.path) return;
  if (m.step === 2 && !abMvDone(m)) return;
  m.step++; if (m.step === 2) { m.shown = 0; const total = abMvChecks(m.id, abMvEntry(m.id).find((x) => x.path === m.path)).length; const tick = () => { const mv = ui.draft.mv; if (mv !== m || m.step !== 2) return; m.shown++; if (ui.dialog && ui.dialog.name === 'abMove') render(); if (m.shown <= total) setTimeout(tick, 420); }; setTimeout(tick, 400); }
  render();
};
ACT.abMvBack = () => { const m = ui.draft.mv; if (m.step > 1) m.step--; render(); };
const abMvDone = (m) => { const e = abMvEntry(m.id).find((x) => x.path === m.path); const c = abMvChecks(m.id, e); return m.shown >= c.length; };
DIALOGS.abMove = () => {
  const m = ui.draft.mv; const s = abSto(m.id); const list = abMvEntry(m.id); const e = list.find((x) => x.path === m.path);
  const steps = ['Neuer Pfad', 'Prüfungen', 'Bestehende Inhalte', 'Bestätigen'];
  const prog = `<ol class="ab-steps" aria-label="Schritte">${steps.map((t, i) => `<li ${i + 1 === m.step ? 'aria-current="step"' : ''} class="${i + 1 < m.step ? 'done' : ''}"><span class="n">${i + 1 < m.step ? icon('check', 12) : i + 1}</span><span class="t">${t}</span></li>`).join('')}</ol>`;
  let body = ''; let acts = '';
  const cancel = `<button type="button" class="btn secondary" data-act="close" data-fid="mv:x">Abbrechen</button>`;
  if (m.step === 1) {
    body = `<p class="lead">Wähle einen Ordner, den der Container beschreibbar eingebunden hat. Frei tippen kannst du nicht.</p><div class="ab-optlist" role="radiogroup" aria-label="Neuer Pfad">${list.map((x, i) => `<label class="check ab-opt"><input type="radio" name="mvp" value="${esc(x.path)}" data-bind="ui.draft.mv.path" ${m.path === x.path ? 'checked' : ''} ${x.current ? 'disabled' : ''} data-fid="mv:p:${i}"${i === 1 ? ' data-autofocus' : ''}><span><b class="mono">${esc(x.path)}</b> <small>${esc(x.kind)} · ${abGB(x.freeGB)} frei · ${esc(x.note)}</small></span></label>`).join('')}</div><p class="muted aa-small">Im Mockup hängt das Ergebnis der Prüfungen vom gewählten Beispielordner ab.</p>`;
    acts = `${cancel}<button type="button" class="btn primary" data-act="abMvNext" data-fid="mv:next"${m.path ? '' : ' aria-disabled="true"'}>Weiter</button>`;
  } else if (m.step === 2) {
    const checks = abMvChecks(m.id, e); const shown = checks.slice(0, m.shown); const done = m.shown >= checks.length; const bad = checks.filter((c) => c[0] === 'bad');
    body = `<p class="lead"><span class="mono">${esc(m.path)}</span> wird geprüft.</p><ul class="checks" aria-live="polite">${shown.map(([k, t, x]) => `<li class="${k}">${icon(k === 'ok' ? 'check' : k === 'bad' ? 'x' : 'alert', 18)}<span><b>${t}</b><small>${x}</small></span></li>`).join('')}${done ? '' : `<li class="wait"><span class="spin"></span><span><b>Prüft …</b></span></li>`}</ul>
      ${done && bad.length ? band('ab:mv:why', 'Warum geht der Wechsel nicht?', `${abPl(bad.length, 'Prüfung', 'Prüfungen')} fehlgeschlagen`, `<p>${bad.map((b) => esc(b[2])).join(' ')}</p><p class="muted">Wähle einen anderen Ordner oder behebe das Problem auf dem Server und prüfe noch einmal.</p>`, { open: true, icon: 'alert' }) : ''}${done && !bad.length ? notice('ok', 'Alle Prüfungen bestanden', checks.some((c) => c[0] === 'warn') ? 'Es gibt Hinweise, aber keine Fehler.' : '') : ''}`;
    acts = `<button type="button" class="btn secondary" data-act="abMvBack" data-fid="mv:back">Zurück</button><button type="button" class="btn primary" data-act="abMvNext" data-fid="mv:next"${done && !bad.length ? '' : ' aria-disabled="true"'}>Weiter</button>`;
  } else if (m.step === 3) {
    body = `<p class="lead">${m.id === 'recordings' ? 'Bestehende Aufnahmen bleiben am alten Ort und sind weiter abspielbar. Der alte Ort bleibt nur lesend eingebunden, bis er leer ist. Slouchcat verschiebt nichts von selbst.' : 'Laufende Puffer bleiben am alten Ort, bis sie enden. Neue Puffer entstehen am neuen Ort.'}</p><dl class="kv"><dt>Alter Ort</dt><dd><span class="mono">${esc(s.path)}</span></dd><dt>Neuer Ort</dt><dd><span class="mono">${esc(m.path)}</span></dd></dl>`;
    acts = `<button type="button" class="btn secondary" data-act="abMvBack" data-fid="mv:back">Zurück</button><button type="button" class="btn primary" data-act="abMvNext" data-fid="mv:next">Weiter</button>`;
  } else {
    const run = m.id === 'recordings' ? admRecRunning() : 0;
    body = `<p class="lead">${m.id === 'recordings' ? 'Neue Aufnahmen ab jetzt in' : `${esc(s.name)} ab jetzt in`} <span class="mono">${esc(m.path)}</span>.</p>${run ? notice('warn', `${abPl(run, 'Aufnahme läuft', 'Aufnahmen laufen')} gerade`, 'Der Wechsel wirkt erst für die nächste Aufnahme. Laufende Aufnahmen bleiben am alten Ort.') : ''}`;
    acts = `<button type="button" class="btn secondary" data-act="abMvBack" data-fid="mv:back">Zurück</button><button type="button" class="btn primary" data-act="abMvOk" data-fid="mv:ok">Speicherort wechseln</button>`;
  }
  return { title: `Speicherort wechseln: ${esc(s.name)}`, wide: true, body: `${prog}${body}`, actions: acts };
};
ACT.abMvOk = () => {
  const m = ui.draft.mv; const s = abSto(m.id); const e = abMvEntry(m.id).find((x) => x.path === m.path); const old = { path: s.path, usedGB: s.usedGB };
  if (m.id === 'recordings') { s.old = (s.old || []).concat([old]); Object.assign(s, { path: e.path, kind: e.kind, totalGB: e.totalGB, usedGB: 0, state: 'ok' }); }
  else if (m.id === 'timeshift') Object.assign(s, { path: e.path, kind: e.kind, freeGB: e.freeGB, state: 'ok' });
  else Object.assign(s, { path: e.path, kind: e.kind, state: 'ok' });
  AB.base[m.id] = abClone(s); ui.abSto = 'normal'; abStoApply(); delete ui.draft.mv; closeDialog(); render(); toast(`${esc(s.name)}: neuer Ort ${esc(s.path)}.`);
};

route('/admin/live/storage', (pm, q) => {
  const tab = ['dvr', 'timeshift', 'storage'].includes(q.tab) ? q.tab : 'dvr'; const mo = abMode();
  abStoApply();
  const tabs = subTabs('#/admin/live/storage', [['dvr', 'Aufnahmen'], ['timeshift', 'Timeshift'], ['storage', 'Speicher']], tab);
  const build = tab === 'timeshift' ? abTimeshift : tab === 'storage' ? abStorage : abDvrTab;
  const empty = abEmptyBox('Noch keine Quelle', 'Aufnahmen und Timeshift brauchen Sender. Füge zuerst eine Quelle hinzu. Die Speicherorte kannst du schon jetzt prüfen.', `<a class="btn primary" href="#/admin/live/sources/new" data-fid="la14:addsrc">${icon('plus', 16)}Quelle hinzufügen</a><a class="btn secondary" href="#/admin/live/storage?tab=storage" data-fid="la14:tosto">Zu «Speicher»</a>`);
  return abView({ page: 'storage', title: 'Aufnahmen und Speicher', intro: 'Vorgaben für Aufnahmen, Timeshift und die drei Speicherorte. Titel von Aufnahmen siehst du nie.', live: mo === 'normal' ? abLive() : null,
    html: tabs + abGate(tab === 'storage' ? null : empty, build, { rows: 4, h: 80 }) });
});
