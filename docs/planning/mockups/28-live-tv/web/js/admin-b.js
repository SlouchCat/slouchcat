/* Web Verwaltung B, Teil 1: gemeinsame Helfer, LA-09 Empfang, LA-10 EPG-Quellen, LA-11 EPG-Zuordnungen.
   Bauvorlage: 15-administration.md 3.9 bis 3.11, Vorlage: page-la02.js und admin-a.js. Globale Namen mit Präfix «ab».
   Die Seiten ändern die Beispieldaten (LIVE.*) direkt und rufen render(); Neuladen stellt alles wieder her.
   Gemeinsame Zähler aus dem Kern (web/js/admin-core.js): admViewers, admRecRunning, admCounts, admLiveText, ADM.ended. */
'use strict';

/* ---------- gemeinsame Helfer ---------- */
const abClone = (o) => JSON.parse(JSON.stringify(o));
const abMode = () => SIM.mode();
const abEmpty = () => abMode() === 'empty';
const abPl = (n, one, many) => `${fmt.num(n)} ${n === 1 ? one : many}`;
const abDe = (n, d) => Number(n).toFixed(d == null ? 1 : d);
const abGB = (x) => (x >= 1000 ? `${abDe(x / 1000, 1)} TB` : `${Number.isInteger(x) ? fmt.num(x) : abDe(x, 1)} GB`);
const abWhen = (t) => { const r = fmt.dayRel(t); return (['Heute', 'Morgen', 'Gestern'].includes(r) ? r.toLowerCase() : r) + ' ' + fmt.time(t); };
const abHM = (m) => String(Math.floor(m / 60)).padStart(2, '0') + ':' + String(m % 60).padStart(2, '0');
const abDur = (min) => { min = Math.max(0, Math.round(min)); if (min >= 1440) { const d = Math.floor(min / 1440); const h = Math.floor((min % 1440) / 60); return `${d} ${d === 1 ? 'Tag' : 'Tage'}${h ? ` ${h} Std.` : ''}`; } return fmt.dur(min); };
const abLive = () => (admLiveText());
const abView = (o) => Object.assign({ type: 'admin', area: 'livetv' }, o);
const abDot = (k) => `<span class="dot ${k}" aria-hidden="true"></span>`;
const abNorm = (s) => String(s || '').toLowerCase().replace(/[^a-z0-9äöüéèà]/g, '');
const abChan = (id) => SIM.channel(id) || { id, name: id, slug: id, no: null };
const abChLabel = (c) => `<span class="ab-chn">${LU.logo(c)}<span><b>${esc(c.name)}</b><small>${c.no != null ? 'Nr. ' + c.no : 'ohne Nummer'}</small></span></span>`;
const abAnon = (id) => { let h = 7; for (const ch of String(id)) h = (h * 31 + ch.charCodeAt(0)) % 46656; return 'R-' + h.toString(36).toUpperCase().padStart(3, '0'); };
const abHostOf = (a) => { const m = String(a).match(/^[a-z]+:\/\/(?:[^@/]*@)?(@?[^:/?#]+)/i); return m ? m[1].replace(/^@/, '') : ''; };
const abSrcName = (id) => (LIVE.sources.find((s) => s.id === id) || { name: 'Quelle entfernt' }).name;

/** Seitenzustände: lädt (Skeleton in Endmass), Fehler (mit «Erneut versuchen»), leer (eigener Text), sonst der Inhalt */
function abGate(emptyHtml, build, o) {
  const m = abMode(); o = o || {};
  if (m === 'loading') return skelRows(o.rows || 5, o.h || 72);
  if (m === 'error') return liveError(o.err || 'Die Seite konnte nicht geladen werden.');
  if (m === 'empty' && emptyHtml) return emptyHtml;
  return build();
}
const abEmptyBox = (title, text, acts) => `<div class="unchanged ab-empty"><b>${title}</b><p>${text}</p>${acts ? `<div class="row-gap">${acts}</div>` : ''}</div>`;
const abStat = (label, value, note, o) => { o = o || {}; const inner = `<span class="l">${label}</span><span class="v">${value}</span><span class="n">${note || ''}</span>`; return o.href ? `<a class="stat ${o.cls || ''}" href="${o.href}" data-fid="${o.fid}">${inner}</a>` : `<div class="stat ${o.cls || ''}">${inner}</div>`; };
const abMeter = (v, max, label, o) => { const p = max > 0 ? Math.max(0, Math.min(100, (v / max) * 100)) : 0; const k = (o && o.cls) || (p >= 95 ? 'bad' : p >= 80 ? 'warn' : ''); return `<span class="ab-meter ${k}" role="meter" aria-valuemin="0" aria-valuemax="${max}" aria-valuenow="${v}" aria-label="${esc(label)}"><i style="width:${p.toFixed(1)}%"></i></span>`; };
/** Einstellungszeile (wie SettingRow) */
const abRow = (title, desc, ctl, o) => `<div class="srow${o && o.cls ? ' ' + o.cls : ''}"><div class="st"><b>${title}</b>${desc ? `<span>${desc}</span>` : ''}</div><div class="sc${o && o.col ? ' col' : ''}">${ctl}</div></div>`;
const abErr = (id, t) => (t ? `<span class="ab-err" id="${id}-e" role="alert">${icon('alert', 14)}${esc(t)}</span>` : '');
/** Auswahlfeld; opts [[wert, text]]; o.num = Zahl */
function abSel(path, id, opts, o) {
  o = o || {}; const cur = String(o.cur != null ? o.cur : (getPath(path) ?? ''));
  return `<select class="sel" id="${id}" data-bind="${path}" data-rerender data-fid="${id}"${o.num ? ' data-num' : ''} aria-label="${esc(o.label || '')}"${o.after ? ` data-after="${o.after}"` : ''}${o.dis ? ' disabled' : ''}${o.err ? ` aria-invalid="true" aria-describedby="${id}-e"` : ''}${o.data ? ' ' + o.data : ''}>${opts.map(([v, t]) => `<option value="${esc(v)}"${String(v) === cur ? ' selected' : ''}>${esc(t)}</option>`).join('')}</select>`;
}
/** Zahl- oder Textfeld */
function abInp(path, id, o) {
  o = o || {}; const v = getPath(path);
  return `<input class="inp${o.num ? ' num' : ''}" id="${id}" type="${o.type || (o.num ? 'number' : 'text')}" data-bind="${path}" data-fid="${id}" value="${esc(v ?? '')}"${o.num ? ' data-num' : ''}${o.min != null ? ` min="${o.min}"` : ''}${o.max != null ? ` max="${o.max}"` : ''}${o.ph ? ` placeholder="${esc(o.ph)}"` : ''}${o.err ? ` aria-invalid="true" aria-describedby="${id}-e"` : ''}${o.live === false ? '' : ' data-live'}${o.auto ? ' data-autofocus' : ''}${o.dis ? ' disabled' : ''}${o.enter ? ` data-enter="${o.enter}"` : ''} autocomplete="off" spellcheck="false"${o.label ? ` aria-label="${esc(o.label)}"` : ''}>`;
}
const abFld = (id, label, inner, o) => { o = o || {}; return `<div class="fld${o.cls ? ' ' + o.cls : ''}"><label for="${id}">${label}</label>${inner}${o.hint ? `<span class="hint">${o.hint}</span>` : ''}${abErr(id, o.err)}</div>`; };
const abRadio = (path, name, v, text, fid, o) => `<label class="check"><input type="radio" name="${name}" value="${esc(v)}" data-bind="${path}" ${String(getPath(path)) === String(v) ? 'checked' : ''} data-fid="${fid}"${o && o.num ? ' data-num' : ''}${o && o.dis ? ' disabled' : ''}><span>${text}</span></label>`;
/** Speichern-Leiste; Speichern gesperrt (aria-disabled), solange Eingaben falsch sind */
const abSaveBar = (save, disc, fid, nerr) => `<div class="savebar" role="region" aria-label="Änderungen speichern"><span class="grow">Ungespeicherte Änderungen${nerr ? ` · ${abPl(nerr, 'Eingabe', 'Eingaben')} prüfen` : ''}</span><button type="button" class="btn ghost" data-act="${disc}" data-fid="${fid}:disc">Verwerfen</button><button type="button" class="btn primary" data-act="${save}" data-fid="${fid}:save"${nerr ? ' aria-disabled="true"' : ''}>Speichern</button></div>`;
/** Entwurf eines Formulars (aus dem Ist-Stand), geändert? */
function abDraft(key, base) { if (!ui.draft[key]) ui.draft[key] = abClone(base); return ui.draft[key]; }
const abDirty = (key, base) => !!ui.draft[key] && JSON.stringify(ui.draft[key]) !== JSON.stringify(base);
/** Spaltenkopf zum Sortieren: state = { sort, dir } */
const abSortHead = (st, key, label, act, extra) => `<th${extra ? ` class="${extra}"` : ''} aria-sort="${st.sort === key ? (st.dir > 0 ? 'ascending' : 'descending') : 'none'}"><button type="button" class="ab-sortb" data-act="${act}" data-k="${key}" data-fid="${act}:${key}">${label}${st.sort === key ? icon(st.dir > 0 ? 'arrowUp' : 'arrowDown', 12) : ''}</button></th>`;
const abSortToggle = (st, k) => { if (st.sort === k) st.dir = -st.dir; else { st.sort = k; st.dir = 1; } };
/** Hinweis «nur im Mockup» mit Auswahl */
const abMock = (id, label, path, opts, after) => `<label class="ab-mock" for="${id}"><span>Nur im Mockup: ${label}</span>${abSel(path, id, opts, { label, after })}</label>`;

/* ---------- Beispieldaten ergänzen (nur diese Seiten) ---------- */
const AB = { stoState: 'normal', epgState: 'normal', epgBusy: {}, just: {}, orphan: 1.2, orphanAt: LIVE.at(0, '03:00'), orphanBusy: false, base: {},
  ov: { sort: 'name', dir: 1 }, map: { f: 'all', q: '', sel: {}, sort: 'no', dir: 1, qf: null, auto: false }, diag: { n: 12, sort: 'out', dir: -1 }, aq: { sort: 'pct', dir: -1 }, inSort: { sort: 'ch', dir: 1 }, probe: {}, busyIn: {}, extra: {} };
(function abSeed() {
  const S = LIVE.settings;
  S.schedules.forEach((s) => { if (!s.rows) s.rows = [{ days: [1, 2, 3, 4, 5], from: 1080, to: 1410 }, { days: [6, 7], from: 600, to: 1440 }]; });
  LIVE.epgSources.forEach((e) => { if (e.tz == null) Object.assign(e, { tz: 'Europe/Zurich', localNet: false, compression: 'auto', hdr: [], ua: '' }); });
  if (S.limits.warnPct == null) S.limits.warnPct = 10;
  if (S.reception.schedule == null) S.reception.schedule = '';
  LIVE.storage.forEach((s) => { AB.base[s.id] = abClone(s); s.old = s.old || []; });
})();

/* ======================================================================================
   Empfang: Auswirkungen auf die aktiven Eingänge (Zahlen müssen zu LA-01 und LA-12 passen)
   ====================================================================================== */
const AB_STRAT = [
  ['permanent', 'Permanent', 'Slouchcat empfängt den Sender rund um die Uhr, auch ohne Zuschauer. Das ist kein Aufzeichnen.'],
  ['auto', 'Automatisch', 'Slouchcat hält oft genutzte Sender bereit, solange jemand Live-TV schaut und Platz ist.'],
  ['ondemand', 'Bei Bedarf', 'Der Empfang beginnt erst, wenn jemand schaut oder eine Aufnahme ihn braucht.'],
  ['schedule', 'Nach Zeitplan', 'Der Empfang läuft in festen Zeiten, Wochentag und Uhrzeit.'],
];
const abStratName = (k) => (AB_STRAT.find((x) => x[0] === k) || [k, k])[1];
const abSched = (name) => LIVE.settings.schedules.find((s) => s.name === name);
function abInWindow(name) {
  const s = abSched(name); if (!s) return false;
  const t = SIM.now(); const wd = LU.cal(t).wd || 7; const m = ((Math.floor(t) % 1440) + 1440) % 1440;
  return s.rows.some((r) => r.days.includes(wd) && m >= r.from && m < r.to);
}
const abSchedChannels = (name) => LIVE.channels.filter((c) => c.active && c.reception === 'schedule' && c.schedule === name);
/** Zahl der Sender je Art: eigene Einstellungen aus den Beispielsendern, der Rest folgt der Vorgabe */
function abStratCounts() {
  const def = LIVE.settings.reception.default; const total = abEmpty() ? 0 : admCounts().active;
  const act = LIVE.channels.filter((c) => c.active); const explicit = act.filter((c) => c.reception !== 'default');
  const by = { permanent: 0, auto: 0, ondemand: 0, schedule: 0 };
  explicit.forEach((c) => { by[c.reception] = (by[c.reception] || 0) + 1; });
  const governed = Math.max(0, total - explicit.length);
  by[def] += governed;
  return { by, explicit, governed, total };
}
/** hält die Eingangsliste im Takt mit den Einstellungen eines Senders (permanent, Zeitplan) */
function abSyncInput(c) {
  const R = LIVE.resources; let n = LIVE.inputs.find((x) => x.ch === c.id);
  const want = [];
  if (c.reception === 'permanent') want.push('permanent');
  if (c.reception === 'schedule' && abInWindow(c.schedule)) want.push('schedule');
  if (n) n.reasons = n.reasons.filter((r) => r !== 'permanent' && r !== 'schedule');
  if (want.length) {
    if (!n) {
      const src = c.sources.find((s) => s.state !== 'fail') || c.sources[0];
      n = { ch: c.id, source: src ? src.source : 'src-kabel', backup: 0, state: 'running', reasons: [], since: SIM.now(), viewers: 0, recordings: 0, bitrate: c.bitrate, transcode: '–', buffer: '12 MB', timeshift: '–', recGB: 0, errors: null };
      LIVE.inputs.push(n); R.bandIn = Math.round((R.bandIn + c.bitrate) * 10) / 10;
    }
    n.reasons.push(...want); if (want.includes('schedule')) n.schedule = c.schedule;
  } else if (n && !n.reasons.length && !n.viewers && !n.recordings) {
    LIVE.inputs.splice(LIVE.inputs.indexOf(n), 1); R.bandIn = Math.max(0, Math.round((R.bandIn - n.bitrate) * 10) / 10);
  }
}

/* ======================================================================================
   LA-09 Empfang: Strategie · Zeitpläne · Ausnahmen je Sender
   ====================================================================================== */
/* Vorladen nur global (an oder aus, Obergrenzen) und je Sender über die Strategie; je Quelle kein Schalter und keine Zahl (04 T-18, RG-03) */
const abRcBase = () => Object.assign({ preloadOn: true, checkIdle: false }, abClone(LIVE.settings.reception), { preloadMax: LIVE.limits.preloaded });
function abRcErrors(d) {
  const e = {};
  if (!Number.isInteger(d.recLeadS) || d.recLeadS < 10 || d.recLeadS > 120) e.lead = 'Gib eine ganze Zahl von 10 bis 120 ein.';
  if (d.default === 'schedule' && !d.schedule) e.sched = LIVE.settings.schedules.length ? 'Wähle einen Zeitplan.' : 'Lege zuerst einen Zeitplan an (Reiter «Zeitpläne»).';
  return e;
}
function abStrategyTab() {
  const base = abRcBase(); const d = abDraft('rc', base); const e = abRcErrors(d); const nerr = Object.keys(e).length; const dirty = abDirty('rc', base);
  const cnt = abStratCounts(); const P = 'ui.draft.rc';
  const defOpts = AB_STRAT.map(([k, t, x]) => `<label class="check ab-card${d.default === k ? ' on' : ''}"><input type="radio" name="rcdef" value="${k}" data-bind="${P}.default" ${d.default === k ? 'checked' : ''} data-fid="la09:def:${k}"><span><b>${t}</b><small>${x}</small></span></label>`).join('');
  const defWarn = d.default === 'permanent'
    ? notice('warn', `Standard «Permanent» für alle Sender: ${fmt.num(cnt.total)} dauerhafte Verbindungen`, 'Jeder permanent empfangene Sender belegt dauerhaft eine Verbindung zur Quelle, auch ohne Zuschauer. Wie viele gleichzeitige Verbindungen dein Anbieter erlaubt, musst du selbst im Blick behalten.') : '';
  const schedSel = d.default === 'schedule' ? abFld('la09-defsch', 'Zeitplan für alle Sender', abSel(`${P}.schedule`, 'la09-defsch', [['', 'Zeitplan wählen …'], ...LIVE.settings.schedules.map((s) => [s.name, `${s.name} (${s.times})`])], { err: e.sched, label: 'Zeitplan für alle Sender' }), { err: e.sched }) : '';
  const srcList = abEmpty() ? [] : LIVE.sources;
  const srcRows = srcList.length ? `<div class="tblw ab-tbl"><table class="tbl"><thead><tr><th>Quelle</th><th>Vorsorglicher Empfang</th></tr></thead><tbody>${srcList.map((s) => { const r = admRest(s.id); return `<tr><td class="main" data-l="Quelle">${esc(s.name)}</td><td data-l="Vorsorglicher Empfang">${r ? `<span class="chip warn">ruht</span> ${esc(admRestText(r))} · <a class="lnk" href="#/admin/live/diagnostics" data-fid="la09:rest:${s.id}">Diagnose</a>` : '<span class="chip ok">läuft</span>'}</td></tr>`; }).join('')}</tbody></table></div>` : abEmptyBox('Noch keine Quelle', 'Sobald eine Quelle besteht, siehst du hier, ob ihr vorsorglicher Empfang läuft oder ruht.', `<a class="btn primary" href="#/admin/live/sources/new" data-fid="la09:addsrc0">${icon('plus', 16)}Quelle hinzufügen</a>`);
  const pl = abEmpty() ? null : admPermLoad();
  const preOff = !d.preloadOn; const preNote = preOff ? '<span class="muted aa-small ab-full">Wirkt nicht, solange «Vorladen» aus ist.</span>' : '';
  return `${admLockLine()}${pl ? notice('warn', 'Dauersender belegen mehr als die Hälfte der Eingänge', admPermText(pl)) : ''}${notice('', 'Wie der Empfang arbeitet', 'Laufende Wiedergaben und Aufnahmen sind immer geschützt: Keine Einstellung beendet sie. Ein laufender Dauersender (permanent oder im Zeitfenster seines Zeitplans) bleibt ebenfalls. Reicht die Kapazität nicht, wird die neue Anfrage abgelehnt («ausgelastet»); zuerst weichen Vorgeladenes, «automatisch» gehaltene Ströme und Ströme in Karenz. Nach einem Neustart nimmt der Server dauerhaft und nach Zeitplan empfangene Sender von selbst wieder auf. Jeder dauerhaft, vorgeladen oder nach Zeitplan empfangene Sender belegt eine Verbindung zur Quelle, auch ohne Zuschauer.')}
    <section class="card"><div class="ch"><span class="ik">${icon('wifi', 18)}</span><h2>Strategie für alle Sender</h2></div>
      <div class="srow-wrap">
        ${abRow('Standard für alle Sender', 'Einzelne Sender kannst du im Reiter «Ausnahmen je Sender» abweichend einstellen.', `<div class="ab-cards" role="radiogroup" aria-label="Standard für alle Sender">${defOpts}</div>${schedSel}${defWarn}`, { col: true })}
        ${abRow('Vorladen', 'Gilt für alle Quellen und Sender. Ab Werk an, mit vorsichtigen Obergrenzen. Je Sender steuerst du den Empfang über die Strategie (Reiter «Ausnahmen je Sender»).', `<span class="ab-swl">${sw(`${P}.preloadOn`, 'Vorladen')}<span>${d.preloadOn ? 'an' : 'aus'}</span></span><span class="muted aa-small ab-full">Zugang mit nur einer Verbindung? Dann schalte das Vorladen hier aus: Es gilt für alle Quellen. Slouchcat zählt keine Verbindungen und kennt keine Anbieterlimits.</span>`)}
        ${abRow('Benachbarte Sender vorladen', 'Beim Umschalten öffnet Slouchcat die Nachbarn in der Senderliste im Voraus.', `${abSel(`${P}.neighbours`, 'la09-nb', [[0, 'aus'], [1, '1 Sender davor und danach'], [2, '2 Sender davor und danach']], { num: true, label: 'Benachbarte Sender vorladen' })}<span class="muted aa-small ab-full">Vorladen öffnet nur den Empfang. Eine Umwandlung für ein Gerät startet erst, wenn jemand schaut. Lehnt eine Quelle eine weitere Verbindung ab, ruht der vorsorgliche Empfang dieser Quelle von selbst (30 Minuten, bei Wiederholung länger); die Diagnose zeigt es.</span>${preNote}`)}
        ${abRow('Vorgeladene Sender insgesamt', 'Obergrenze für alle Zuschauer zusammen. Benachbarte und oft genutzte Sender zählen beide dazu.', `${abSel(`${P}.preloadMax`, 'la09-pm', Array.from({ length: 17 }, (_, i) => [i, i === 0 ? 'keine' : `höchstens ${i}`]), { num: true, label: 'Vorgeladene Sender insgesamt' })}<span class="muted aa-small ab-full">Vorschlag: höchstens 4.</span>${preNote}`)}
        ${abRow('Oft genutzte Sender bereithalten', `Gilt für Sender mit der Art «Automatisch» (derzeit ${abPl(cnt.by.auto, 'Sender', 'Sender')}).`, `${abSel(`${P}.keepHot`, 'la09-kh', Array.from({ length: 11 }, (_, i) => [i, i === 0 ? 'keinen' : `höchstens ${i}`]), { num: true, label: 'Oft genutzte Sender bereithalten' })}<span class="muted aa-small ab-full">Solange jemand Live-TV schaut und noch 10 Minuten danach.</span>${preNote}`)}
        ${abRow('Lieblingssender der Zuschauer berücksichtigen', 'Die Favoriten der Profile, die gerade schauen, stehen in der Liste zuerst.', sw(`${P}.favourites`, 'Lieblingssender der Zuschauer berücksichtigen'))}
        ${abRow('Nicht mehr gebrauchten Empfang beenden nach', 'Wartezeit, nachdem der letzte Zuschauer weg ist.', abSel(`${P}.stopAfterS`, 'la09-st', [[15, '15 s'], [30, '30 s'], [60, '60 s'], [120, '2 Min.'], [300, '5 Min.']], { num: true, label: 'Empfang beenden nach' }))}
        ${abRow('Empfang für Aufnahmen vorher starten', 'Zusätzlich zum Vorlauf der Aufnahme.', `<span class="ab-unit">${abInp(`${P}.recLeadS`, 'la09-lead', { num: true, min: 10, max: 120, err: e.lead, label: 'Sekunden vor dem Vorlauf' })}<span>s vor dem Vorlauf</span></span>${abErr('la09-lead', e.lead)}`)}
        ${abRow('Übrige ruhende Sender prüfen', 'Lieblingssender, Sender mit Aufnahme in 24 Stunden und Sender mit Zeitplan prüft Slouchcat immer. Alle übrigen nur, wenn das an ist.', `<span class="ab-swl">${sw(`${P}.checkIdle`, 'Übrige ruhende Sender prüfen')}<span>${d.checkIdle ? 'an' : 'aus'}</span></span><span class="muted aa-small ab-full">Werksvorgabe: aus. An: höchstens einmal je Tag und Sender in einem Nachtfenster, nie bei einer Quelle, deren vorsorglicher Empfang gerade ruht.</span>`)}
      </div>
    </section>
    <section class="card"><div class="ch"><span class="ik">${icon('link', 18)}</span><h2>Quellen</h2></div>
      <p class="muted">Nur Anzeige, keine Einstellung: je Quelle der Zustand des vorsorglichen Empfangs. Ruht er, hat Slouchcat das von selbst so gestellt.</p>
      ${srcRows}</section>
    ${dirty ? abSaveBar('abRcSave', 'abRcDisc', 'la09', nerr) : ''}`;
}
ACT.abRcDisc = () => { delete ui.draft.rc; render(); };
ACT.abRcSave = () => {
  const d = ui.draft.rc; if (!d || Object.keys(abRcErrors(d)).length) return;
  if (d.default === 'permanent' && LIVE.settings.reception.default !== 'permanent' && !ui.draft.rcOk) { openDialog('abRcConfirm', {}); return; }
  ui.draft.rcOk = false;
  const { preloadMax, ...rest } = d; LIVE.settings.reception = abClone(rest); LIVE.limits.preloaded = preloadMax;
  delete ui.draft.rc; render();
  toast('Empfang gespeichert. Laufende Wiedergaben und Aufnahmen bleiben unberührt.');
};
DIALOGS.abRcConfirm = () => {
  const c = abStratCounts(); const n = abEmpty() ? 0 : admCounts().active;
  return { title: 'Alle Sender permanent empfangen?', body: `<p class="lead">Damit empfängt Slouchcat bis zu ${fmt.num(n)} Sender rund um die Uhr, auch ohne Zuschauer. Jeder davon belegt dauerhaft eine Verbindung zu seiner Quelle.</p><p class="muted">Slouchcat kennt das Limit deines Anbieters nicht. Zu viele Verbindungen können dazu führen, dass die Quelle Sender sperrt. Du kannst die Vorgabe jederzeit zurücksetzen. ${fmt.num(c.explicit.length)} Sender haben schon eine eigene Einstellung und bleiben davon unberührt.</p>`,
    actions: `<button type="button" class="btn secondary" data-act="close" data-fid="la09c:x" data-autofocus>Abbrechen</button><button type="button" class="btn danger" data-act="abRcConfirmOk" data-fid="la09c:ok">Trotzdem speichern</button>` };
};
ACT.abRcConfirmOk = () => { ui.draft.rcOk = true; closeDialog(); ACT.abRcSave(); };

/* ---------- Zeitpläne ---------- */
const AB_WD = ['Mo', 'Di', 'Mi', 'Do', 'Fr', 'Sa', 'So'];
function abDaysText(days) {
  const d = [...days].sort((a, b) => a - b); const out = []; let i = 0;
  while (i < d.length) { let j = i; while (j + 1 < d.length && d[j + 1] === d[j] + 1) j++; out.push(j > i ? `${AB_WD[d[i] - 1]}–${AB_WD[d[j] - 1]}` : AB_WD[d[i] - 1]); i = j + 1; }
  return out.join(', ');
}
const abTimesText = (rows) => rows.map((r) => `${abDaysText(r.days)} ${abHM(r.from)}–${abHM(r.to)}`).join(', ');
const AB_T = Array.from({ length: 49 }, (_, i) => i * 30);
function abSchedTab() {
  const list = LIVE.settings.schedules;
  const rows = list.map((s) => {
    const chs = abSchedChannels(s.name); const open = isOpen('ab:sch:' + s.id); const fresh = AB.just['sch:' + s.id];
    const menu = popMenu('ab:sch:' + s.id, icon('dots', 16), [{ label: 'Ändern …', act: 'abSchedEdit', data: { id: s.id }, icon: 'pen' }, { label: 'Löschen …', act: 'abSchedDel', data: { id: s.id }, danger: true, icon: 'trash' }], { aria: `Weitere Aktionen für ${s.name}`, cls: 'quiet icon' });
    return `<tr class="${open ? 'open' : ''}"><td class="main" data-l="Name"><span class="ab-name"><button type="button" class="tog" data-act="open" data-id="ab:sch:${s.id}" aria-expanded="${open}" aria-label="${esc(s.name)}: Einzelheiten ${open ? 'zuklappen' : 'aufklappen'}" data-fid="la09:stog:${s.id}">${icon('chevR', 14)}</button><b>${esc(s.name)}</b>${fresh ? '<span class="chip ok">Neu angelegt</span>' : ''}</span></td>
      <td data-l="Zeiten">${esc(s.times)}</td><td data-l="Sender">${abPl(chs.length, 'Sender', 'Sender')}</td>
      <td class="end"><div class="ta"><button type="button" class="btn quiet" data-act="abSchedEdit" data-id="${s.id}" data-fid="la09:sed:${s.id}">${icon('pen', 16)}Ändern</button>${menu}</div></td></tr>
      ${open ? `<tr class="sub"><td colspan="4"><dl class="kv"><dt>Zeitzone</dt><dd>Europe/Zurich (Zeitzone des Servers)</dd><dt>Gilt für</dt><dd>${chs.length ? chs.map((c) => esc(c.name)).join(', ') : 'noch kein Sender. Ordne ihn im Reiter «Ausnahmen je Sender» zu.'}</dd><dt>Jetzt</dt><dd>${abInWindow(s.name) ? `${abDot('ok')} im Zeitfenster: die Sender werden empfangen` : `${abDot('')} ausserhalb des Zeitfensters`}</dd></dl></td></tr>` : ''}`;
  }).join('');
  return `<div class="ab-bar"><p class="muted">Ein Zeitplan legt fest, wann Slouchcat einen Sender auch ohne Zuschauer empfängt. Jeder Sender im Zeitfenster belegt eine Verbindung zur Quelle.</p><button type="button" class="btn primary" data-act="abSchedNew" data-fid="la09:snew">${icon('plus', 16)}Zeitplan hinzufügen</button></div>
    ${list.length ? `<div class="tblw ab-tbl"><table class="tbl"><thead><tr><th>Name</th><th>Zeiten</th><th>Sender</th><th class="end"><span class="vh">Aktionen</span></th></tr></thead><tbody>${rows}</tbody></table></div>`
      : abEmptyBox('Noch kein Zeitplan', 'Mit einem Zeitplan empfängt Slouchcat bestimmte Sender in festen Zeiten, zum Beispiel jeden Abend.', `<button type="button" class="btn primary" data-act="abSchedNew" data-fid="la09:snew2">${icon('plus', 16)}Zeitplan hinzufügen</button>`)}`;
}
function abSchedNewDraft() { return { id: null, name: '', rows: [{ days: [1, 2, 3, 4, 5], from: 1080, to: 1410 }], tried: false }; }
ACT.abSchedNew = () => { ui.draft.sch = abSchedNewDraft(); openDialog('abSched', {}); };
ACT.abSchedEdit = (dd) => { const s = LIVE.settings.schedules.find((x) => x.id === dd.id); ui.draft.sch = { id: s.id, name: s.name, rows: abClone(s.rows), tried: false }; openDialog('abSched', {}); };
function abSchedErrors(d) {
  const e = {}; const name = d.name.trim();
  if (!name) e.name = 'Gib dem Zeitplan einen Namen.';
  else if (LIVE.settings.schedules.some((s) => s.id !== d.id && s.name.toLowerCase() === name.toLowerCase())) e.name = 'Diesen Namen gibt es schon.';
  d.rows.forEach((r, i) => { if (!r.days.length) e['d' + i] = 'Wähle mindestens einen Wochentag.'; if (r.to <= r.from) e['t' + i] = '«bis» muss nach «von» liegen.'; });
  return e;
}
DIALOGS.abSched = () => {
  const d = ui.draft.sch; const e = d.tried ? abSchedErrors(d) : {}; const used = d.id ? abSchedChannels(LIVE.settings.schedules.find((s) => s.id === d.id).name).length : 0;
  const rows = d.rows.map((r, i) => `<fieldset class="aa-fs ab-trow"><legend>Zeitfenster ${i + 1}</legend>
    <div class="ab-days" role="group" aria-label="Wochentage, Zeitfenster ${i + 1}">${AB_WD.map((w, k) => `<button type="button" class="ab-day" aria-pressed="${r.days.includes(k + 1)}" data-act="abDay" data-r="${i}" data-d="${k + 1}" data-fid="sch:d:${i}:${k}">${w}</button>`).join('')}</div>${abErr('sch-d' + i, e['d' + i])}
    <div class="ab-times"><label>von ${abSel(`ui.draft.sch.rows.${i}.from`, `sch-f${i}`, AB_T.slice(0, 48).map((m) => [m, abHM(m)]), { num: true, label: `von, Zeitfenster ${i + 1}` })}</label><label>bis ${abSel(`ui.draft.sch.rows.${i}.to`, `sch-t${i}`, AB_T.slice(1).map((m) => [m, abHM(m)]), { num: true, label: `bis, Zeitfenster ${i + 1}`, err: e['t' + i] })}</label>
    ${d.rows.length > 1 ? `<button type="button" class="btn quiet dq" data-act="abRowDel" data-r="${i}" data-fid="sch:rd:${i}">${icon('trash', 16)}Entfernen</button>` : ''}</div>${abErr('sch-t' + i, e['t' + i])}</fieldset>`).join('');
  return { title: d.id ? `Zeitplan «${esc(LIVE.settings.schedules.find((s) => s.id === d.id).name)}» ändern` : 'Zeitplan hinzufügen', wide: true,
    body: `${abFld('sch-name', 'Name', abInp('ui.draft.sch.name', 'sch-name', { err: e.name, auto: true, ph: 'zum Beispiel «Abends»', enter: 'abSchedSave' }), { err: e.name })}
      <div class="stack">${rows}</div><div><button type="button" class="btn secondary" data-act="abRowAdd" data-fid="sch:radd">${icon('plus', 16)}Weiteres Zeitfenster</button></div>
      <dl class="kv"><dt>Zeitzone</dt><dd>Europe/Zurich (Zeitzone des Servers, nicht einstellbar)</dd><dt>Ergibt</dt><dd>${d.rows.every((r) => r.days.length && r.to > r.from) ? esc(abTimesText(d.rows)) : '–'}</dd>${used ? `<dt>Sender</dt><dd>${abPl(used, 'Sender nutzt', 'Sender nutzen')} diesen Zeitplan</dd>` : ''}</dl>`,
    actions: `<button type="button" class="btn secondary" data-act="close" data-fid="sch:x">Abbrechen</button><button type="button" class="btn primary" data-act="abSchedSave" data-fid="sch:ok">${d.id ? 'Speichern' : 'Zeitplan anlegen'}</button>` };
};
ACT.abDay = (dd) => { const r = ui.draft.sch.rows[Number(dd.r)]; const k = Number(dd.d); r.days = r.days.includes(k) ? r.days.filter((x) => x !== k) : [...r.days, k].sort((a, b) => a - b); render(); };
ACT.abRowAdd = () => { ui.draft.sch.rows.push({ days: [6, 7], from: 600, to: 1440 }); render(); };
ACT.abRowDel = (dd) => { ui.draft.sch.rows.splice(Number(dd.r), 1); render(); };
ACT.abSchedSave = () => {
  const d = ui.draft.sch; d.tried = true; const e = abSchedErrors(d);
  if (Object.keys(e).length) { render(); setTimeout(() => { const el = document.querySelector('#ov-dialog [aria-invalid="true"], #ov-dialog .ab-err'); if (el && el.focus) el.focus(); }, 0); return; }
  const name = d.name.trim(); const times = abTimesText(d.rows); let id = d.id;
  if (id) { const s = LIVE.settings.schedules.find((x) => x.id === id); LIVE.channels.forEach((c) => { if (c.schedule === s.name) c.schedule = name; }); Object.assign(s, { name, rows: abClone(d.rows), times }); }
  else { id = 'sch-' + (abNorm(name) || 'neu') + '-' + (LIVE.settings.schedules.length + 1); LIVE.settings.schedules.push({ id, name, rows: abClone(d.rows), times, channels: [] }); AB.just['sch:' + id] = true; ui.open['ab:sch:' + id] = true; }
  LIVE.channels.filter((c) => c.reception === 'schedule' && c.schedule === name).forEach(abSyncInput);
  delete ui.draft.sch; closeDialog(); render(); toast(`Zeitplan «${esc(name)}» ${d.id ? 'gespeichert' : 'angelegt'}.`);
};
ACT.abSchedDel = (dd) => { ui.draft.schDel = dd.id; openDialog('abSchedDel', {}); };
DIALOGS.abSchedDel = () => {
  const s = LIVE.settings.schedules.find((x) => x.id === ui.draft.schDel); const n = abSchedChannels(s.name).length;
  return { title: `Zeitplan «${esc(s.name)}» löschen?`, body: `<p class="lead">${n ? `${abPl(n, 'Sender nutzt', 'Sender nutzen')} «${esc(s.name)}». ${n === 1 ? 'Er fällt' : 'Sie fallen'} auf den Standard zurück.` : 'Kein Sender nutzt diesen Zeitplan.'}</p><p class="muted">Laufende Wiedergaben und Aufnahmen bleiben unberührt.</p>`,
    actions: `<button type="button" class="btn secondary" data-act="close" data-fid="schd:x" data-autofocus>Abbrechen</button><button type="button" class="btn danger" data-act="abSchedDelOk" data-fid="schd:ok">${icon('trash', 16)}Löschen</button>` };
};
ACT.abSchedDelOk = () => {
  const L = LIVE.settings.schedules; const k = L.findIndex((x) => x.id === ui.draft.schDel); if (k < 0) return; const [s] = L.splice(k, 1);
  const chs = abSchedChannels(s.name); chs.forEach((c) => { c.reception = 'default'; c.schedule = null; abSyncInput(c); });
  if (LIVE.settings.reception.default === 'schedule' && LIVE.settings.reception.schedule === s.name) LIVE.settings.reception.schedule = '';
  closeDialog(); render(); toast(`Zeitplan «${esc(s.name)}» gelöscht. ${chs.length ? abPl(chs.length, 'Sender fällt', 'Sender fallen') + ' auf den Standard zurück.' : ''}`);
};

/* ---------- Ausnahmen je Sender ---------- */
/** Zustand eines Senders mit eigener Empfangsart: läuft · zurückgestellt seit … · gestört · nicht empfangen */
function abOvState(c) {
  const n = LIVE.inputs.find((x) => x.ch === c.id);
  if (n && n.state === 'deferred') return `<span class="chip warn" title="${esc(admDefWhy(n))}">zurückgestellt: konnte nicht starten, ${esc(admDefCause(n).replace(' der Eingangsströme', ''))}, ${admDefTries(n)} Versuche</span>`;
  if ((n && n.state === 'reconnecting') || SIM.chState(c) !== 'ok') return '<span class="chip bad">gestört</span>';
  if (n) return '<span class="chip ok">läuft</span>';
  return '<span class="muted">nicht empfangen</span>';
}
const AB_OVF = [['', 'Alle Arten'], ['permanent', 'Permanent'], ['auto', 'Automatisch'], ['ondemand', 'Bei Bedarf'], ['schedule', 'Nach Zeitplan']];
function abOverridesTab() {
  const cnt = abStratCounts(); const q = (ui.draft.ovq || '').toLowerCase(); const f = ui.draft.ovf || '';
  const inputsFor = (k) => LIVE.inputs.filter((n) => n.reasons.includes(k)).length;
  const nDef = LIVE.inputs.filter((n) => n.state === 'deferred').length;
  const tiles = `<div class="stats" style="--n:5">${abStat('Permanent', abPl(cnt.by.permanent, 'Sender', 'Sender'), `belegt dauerhaft ${abPl(cnt.by.permanent - nDef, 'Eingang', 'Eingänge')} von ${LIVE.limits.inputs}`, { cls: cnt.by.permanent ? 'warn' : '' })}${abStat('Zurückgestellt', abPl(nDef, 'Sender', 'Sender'), nDef ? 'konnte nicht starten, belegt keinen Platz' : 'keiner', { cls: nDef ? 'warn' : '' })}${abStat('Nach Zeitplan', abPl(cnt.by.schedule, 'Sender', 'Sender'), `derzeit ${abPl(inputsFor('schedule'), 'Eingang', 'Eingänge')} im Zeitfenster`)}${abStat('Automatisch', abPl(cnt.by.auto, 'Sender', 'Sender'), `davon höchstens ${LIVE.settings.reception.keepHot} bereit`)}${abStat('Bei Bedarf', abPl(cnt.by.ondemand, 'Sender', 'Sender'), 'ohne Zuschauer kein Empfang')}</div>`;
  const pl = admPermLoad(); const plWarn = pl ? notice('warn', 'Dauersender belegen mehr als die Hälfte der Eingänge', admPermText(pl)) : '';
  let list = cnt.explicit.filter((c) => (!f || c.reception === f) && (!q || c.name.toLowerCase().includes(q) || String(c.no) === q));
  const s = AB.ov; const key = { name: (c) => c.name.toLowerCase(), strat: (c) => abStratName(c.reception), sched: (c) => c.schedule || '' }[s.sort];
  list = list.sort((a, b) => (key(a) < key(b) ? -1 : key(a) > key(b) ? 1 : 0) * s.dir);
  const addable = LIVE.channels.filter((c) => c.active && c.reception === 'default');
  const a = ui.draft.ovAdd || (ui.draft.ovAdd = { ch: '', strat: 'permanent', sched: '' });
  const aerr = ui.draft.ovAddErr;
  const addForm = `<section class="card"><div class="ch"><span class="ik">${icon('plus', 18)}</span><h2>Ausnahme hinzufügen</h2></div>
    <div class="ab-addrow">${abFld('la09-ac', 'Sender', abSel('ui.draft.ovAdd.ch', 'la09-ac', [['', 'Sender wählen …'], ...addable.map((c) => [c.id, `${c.no != null ? c.no + ' · ' : ''}${c.name}`])], { label: 'Sender', err: aerr && !a.ch }), { err: aerr && !a.ch ? 'Wähle einen Sender.' : '' })}
    ${abFld('la09-as', 'Art', abSel('ui.draft.ovAdd.strat', 'la09-as', AB_STRAT.map(([k, t]) => [k, t]), { label: 'Art' }))}
    ${a.strat === 'schedule' ? abFld('la09-asc', 'Zeitplan', abSel('ui.draft.ovAdd.sched', 'la09-asc', [['', 'Zeitplan wählen …'], ...LIVE.settings.schedules.map((x) => [x.name, x.name])], { label: 'Zeitplan', err: aerr && !a.sched }), { err: aerr && !a.sched ? 'Wähle einen Zeitplan.' : '' }) : ''}
    <button type="button" class="btn primary" data-act="abOvAdd" data-fid="la09:oadd">${icon('plus', 16)}Hinzufügen</button></div>
    ${a.strat === 'permanent' ? '<p class="muted aa-small">Ein permanent empfangener Sender belegt dauerhaft eine Verbindung zur Quelle, auch ohne Zuschauer.</p>' : ''}</section>`;
  const tbl = list.length ? `<div class="tblw ab-tbl"><table class="tbl"><thead><tr>${abSortHead(s, 'name', 'Sender', 'abOvSort')}${abSortHead(s, 'strat', 'Strategie', 'abOvSort')}${abSortHead(s, 'sched', 'Zeitplan', 'abOvSort')}<th>Zustand</th><th class="end"><span class="vh">Aktionen</span></th></tr></thead><tbody>${list.map((c) => `<tr><td class="main" data-l="Sender">${abChLabel(c)}</td>
    <td data-l="Strategie">${abSel('ui.draft.ovx', `la09-ov-${c.id}`, AB_STRAT.map(([k, t]) => [k, t]), { label: `Strategie für ${c.name}`, after: 'abOvStrat', data: `data-ch="${c.id}"`, cur: c.reception })}</td>
    <td data-l="Zeitplan">${c.reception === 'schedule' ? abSel('ui.draft.ovx', `la09-ovs-${c.id}`, LIVE.settings.schedules.map((x) => [x.name, x.name]).concat(abSched(c.schedule) ? [] : [['', 'Zeitplan wählen …']]), { label: `Zeitplan für ${c.name}`, after: 'abOvSched', data: `data-ch="${c.id}"`, cur: c.schedule || '' }) : '<span class="muted">–</span>'}</td>
    <td data-l="Zustand">${abOvState(c)}</td>
    <td class="end"><button type="button" class="btn quiet" data-act="abOvReset" data-ch="${c.id}" data-fid="la09:ores:${c.id}">${icon('undo', 16)}Auf Standard zurück</button></td></tr>`).join('')}</tbody></table></div>`
    : abEmptyBox(cnt.explicit.length ? 'Keine Treffer' : 'Keine Ausnahmen', cnt.explicit.length ? 'Passe Suche oder Filter an.' : `Alle Sender folgen dem Standard «${abStratName(LIVE.settings.reception.default)}».`);
  return `${tiles}${plWarn}${addForm}
    <div class="ab-tools"><div class="fld ab-grow"><label for="la09-q">Sender suchen</label><input class="inp" id="la09-q" type="search" data-bind="ui.draft.ovq" data-live data-fid="la09:q" value="${esc(ui.draft.ovq || '')}" placeholder="Name oder Nummer" autocomplete="off"></div>
    <div class="fld"><label for="la09-f">Art</label>${abSel('ui.draft.ovf', 'la09-f', AB_OVF, { label: 'Nach Art filtern' })}</div></div>
    ${tbl}<p class="muted aa-small">${fmt.num(list.length)} von ${fmt.num(cnt.explicit.length)} Ausnahmen · ${fmt.num(cnt.governed)} weitere Sender folgen dem Standard «${abStratName(LIVE.settings.reception.default)}».</p>`;
}
ACT.abOvSort = (dd) => { abSortToggle(AB.ov, dd.k); render(); };
ACT.abOvStrat = (dd, el) => {
  const c = SIM.channel(dd.ch); const v = el.value; c.reception = v; if (v !== 'schedule') c.schedule = null; else c.schedule = c.schedule || (LIVE.settings.schedules[0] || {}).name || null;
  abSyncInput(c); render(); toast(`${esc(c.name)}: ${abStratName(v)}${v === 'schedule' && c.schedule ? ' «' + esc(c.schedule) + '»' : ''}.`);
};
ACT.abOvSched = (dd, el) => { const c = SIM.channel(dd.ch); c.schedule = el.value || null; abSyncInput(c); render(); toast(`${esc(c.name)}: Zeitplan «${esc(c.schedule)}».`); };
ACT.abOvReset = (dd) => { const c = SIM.channel(dd.ch); const was = c.reception; c.reception = 'default'; c.schedule = null; abSyncInput(c); render(); toast(`${esc(c.name)} folgt wieder dem Standard.`); void was; };
ACT.abOvAdd = () => {
  const a = ui.draft.ovAdd; if (!a.ch || (a.strat === 'schedule' && !a.sched)) { ui.draft.ovAddErr = true; render(); return; }
  const c = SIM.channel(a.ch); c.reception = a.strat; c.schedule = a.strat === 'schedule' ? a.sched : null; abSyncInput(c);
  ui.draft.ovAdd = { ch: '', strat: 'permanent', sched: '' }; ui.draft.ovAddErr = false; render(); toast(`Ausnahme für ${esc(c.name)} angelegt: ${abStratName(c.reception)}.`);
};

route('/admin/live/reception', (pm, q) => {
  const tab = ['strategy', 'schedules', 'overrides'].includes(q.tab) ? q.tab : 'strategy';
  const nExp = LIVE.channels.filter((c) => c.active && c.reception !== 'default').length;
  const tabs = subTabs('#/admin/live/reception', [['strategy', 'Strategie'], ['schedules', 'Zeitpläne', LIVE.settings.schedules.length], ['overrides', 'Ausnahmen je Sender', nExp]], tab);
  const body = tab === 'schedules' ? abSchedTab : tab === 'overrides' ? abOverridesTab : abStrategyTab;
  return abView({ page: 'reception', title: 'Empfang', intro: 'Wann Slouchcat einen Sender empfängt: dauerhaft, nach Zeitplan, automatisch oder erst bei Bedarf.', live: abMode() === 'normal' ? abLive() : null,
    html: tabs + abGate(tab === 'strategy' ? null : abEmptyBox('Noch keine Sender', 'Ohne Sender gibt es nichts zu empfangen. Füge zuerst eine Quelle hinzu.', `<a class="btn primary" href="#/admin/live/sources/new" data-fid="la09:addsrc">${icon('plus', 16)}Quelle hinzufügen</a>`), body, { rows: 4, h: 80 }) });
});

/* ======================================================================================
   LA-10 EPG-Quellen
   ====================================================================================== */
const AB_EKIND = { 'xmltv-url': 'XMLTV-Adresse', 'xmltv-file': 'XMLTV-Datei', xtream: 'Von Xtream' };
const AB_TZ = [['Europe/Zurich', 'Europe/Zurich (Schweiz)'], ['Europe/Berlin', 'Europe/Berlin'], ['Europe/London', 'Europe/London'], ['UTC', 'UTC']];
const AB_COMP = [['auto', 'automatisch erkennen'], ['none', 'nicht gepackt'], ['gz', 'gzip (.gz)'], ['xz', 'xz (.xz)']];
const AB_EINT = [[6, 'alle 6 Stunden'], [12, 'alle 12 Stunden'], [24, 'alle 24 Stunden'], [0, 'nur von Hand']];
const AB_EST = [['normal', 'Import gelingt'], ['warn', 'Import gelingt mit Hinweis (Zeitzone fehlt)'], ['failed', 'Import fehlgeschlagen (Server antwortet 503)'], ['truncated', 'Import fehlgeschlagen (Datei abgeschnitten)']];
const abEpgComp = (e) => { if (e.kind === 'xtream') return 'vom Xtream-Zugang bestimmt'; const f = (e.kind === 'xmltv-file' ? e.file : e.address) || ''; const c = e.compression === 'auto' || !e.compression ? (/\.gz(\?|$)/.test(f) ? 'gz' : /\.xz(\?|$)/.test(f) ? 'xz' : 'none') : e.compression; return c === 'gz' ? 'gepackt (gzip)' : c === 'xz' ? 'gepackt (xz)' : 'nicht gepackt'; };
const abEpgNext = (e) => (e.interval ? abWhen(e.last ? e.last + e.interval * 60 : SIM.now()) : 'nur von Hand');
const abEpgWarn = (e) => (e.warnings || []).map((w) => w.replace('als Ortszeit gelesen', `als ${e.tz || 'Europe/Zurich'} gelesen`));
const abEpgShift = (e) => (e.shift ? `${e.shift > 0 ? '+' : '−'}${Math.abs(e.shift)} ${Math.abs(e.shift) === 1 ? 'Stunde' : 'Stunden'}` : 'keine');
const abEpgCover = (e) => Math.round(e.until / 1440);
function abEpgRun(e, res, text) { (LIVE.epgRuns[e.id] || (LIVE.epgRuns[e.id] = [])).unshift([SIM.now(), res, text]); LIVE.epgRuns[e.id] = LIVE.epgRuns[e.id].slice(0, 5); }
/** Mockup-Zustand setzen: «fehlgeschlagen» zeigt sofort den Fehlerzustand der Xtream-Programmdaten (alter Stand bleibt) */
ACT.abEpgState = () => {
  const e = LIVE.epgSources.find((x) => x.kind === 'xtream') || LIVE.epgSources[0]; if (!e) return;
  if (e.mockFail) { e.lastOk = true; e.lastError = null; e.last = e.mockFail.last; LIVE.epgRuns[e.id] = (LIVE.epgRuns[e.id] || []).filter((r) => !r.mock); delete e.mockFail; }
  const st = ui.abEpg;
  if (st === 'failed' || st === 'truncated') {
    e.mockFail = { last: e.last }; e.lastOk = false; e.lastError = st === 'failed' ? 'Server antwortet 503' : 'Datei abgeschnitten nach 18 MB: Import abgebrochen, bisherige Daten bleiben'; e.last = SIM.now() - 3;
    abEpgRun(e, 'bad', e.lastError); LIVE.epgRuns[e.id][0].mock = true;
  }
  render();
};
ACT.abEpgLoad = (dd) => {
  const e = LIVE.epgSources.find((x) => x.id === dd.id); if (!e || AB.epgBusy[e.id]) return;
  const of = e.programmes || 21400; const b = AB.epgBusy[e.id] = { n: 0, of }; render();
  const step = () => {
    if (!AB.epgBusy[e.id]) return;
    b.n = Math.min(of, b.n + Math.ceil(of / 5));
    if (b.n < of) { render(); setTimeout(step, 420); return; }
    delete AB.epgBusy[e.id]; const t = SIM.now(); const st = ui.abEpg;
    if (st === 'failed' || st === 'truncated') {
      e.lastOk = false; e.lastError = st === 'failed' ? 'Server antwortet 503' : 'Datei abgeschnitten nach 18 MB: Import abgebrochen, bisherige Daten bleiben'; e.last = t; abEpgRun(e, 'bad', e.lastError);
      render(); toast(`«${esc(e.name)}» konnte nicht geladen werden: ${esc(e.lastError)}. Der alte Stand bleibt aktiv.`, 'bad', 7000); return;
    }
    if (!e.channels) { e.channels = 180; e.programmes = 21400; e.warnings = []; }
    e.lastOk = true; e.lastError = null; e.last = t; e.until = LIVE.at(Math.max(1, LIVE.settings.epg.previewDays), '00:00'); e.programmes += 140;
    if (st === 'warn') { e.warnings = ['41 Sendungen ohne Zeitzone, als Ortszeit gelesen']; abEpgRun(e, 'warn', e.warnings[0].replace('als Ortszeit gelesen', `als ${e.tz} gelesen`)); } else abEpgRun(e, 'ok', `${fmt.num(e.programmes)} Sendungen`);
    render(); toast(`«${esc(e.name)}» aktualisiert: ${fmt.num(e.programmes)} Sendungen bis ${fmt.day(e.until)}.`, st === 'warn' ? 'warn' : 'ok', 6000);
  };
  setTimeout(step, 420);
};
ACT.abEpgTest = (dd) => openDialog('abEpgTest', { id: dd.id });
DIALOGS.abEpgTest = (p) => {
  const e = LIVE.epgSources.find((x) => x.id === p.id); const bad = !e.lastOk || ui.abEpg === 'failed' || ui.abEpg === 'truncated';
  const rows = bad ? [['bad', ui.abEpg === 'truncated' ? 'Datei zu früh abgeschnitten' : 'Server antwortet 503', ui.abEpg === 'truncated' ? 'Nach 18 MB endet die Antwort. Slouchcat übernimmt keine halbe Datei.' : `Host ${esc(abHostOf(e.address) || 'xtream.example.com')} · Dienst nicht verfügbar`], ['wait', 'Programmdaten nicht gelesen', `Der alte Stand bleibt aktiv, er reicht bis ${fmt.dayRel(e.until)} ${fmt.day(e.until)}.`]]
    : [['ok', 'Adresse erreichbar', `Antwort in ${e.localNet ? 12 : 280} ms`], ['ok', `Programmdaten gelesen: ${fmt.num(e.channels || 180)} Kanäle`, `${fmt.num(e.programmes || 21400)} Sendungen · ${abEpgComp(e)}`], [ui.abEpg === 'warn' || (e.warnings || []).length ? 'warn' : 'ok', (e.warnings || []).length || ui.abEpg === 'warn' ? 'Hinweis zu den Zeiten' : 'Zeiten eindeutig', (e.warnings || []).length ? abEpgWarn(e)[0] : `Zeitzone ${esc(e.tz)}, Zeitkorrektur ${abEpgShift(e)}`]];
  return { title: `Quelle testen: ${esc(e.name)}`, body: `<p class="lead">Slouchcat hat die Programmdaten eben einmal abgefragt, nichts übernommen. Adressen erscheinen ohne Zugangsdaten.</p><ul class="checks">${rows.map(([k, t, x]) => `<li class="${k}">${icon(k === 'ok' ? 'check' : k === 'bad' ? 'x' : k === 'warn' ? 'alert' : 'clock', 18)}<span><b>${t}</b><small>${x}</small></span></li>`).join('')}</ul>`, actions: `<button type="button" class="btn secondary" data-act="close" data-fid="epgt:x" data-autofocus>Schliessen</button>` };
};
ACT.abEpgDel = (dd) => openDialog('abEpgDel', { id: dd.id });
DIALOGS.abEpgDel = (p) => {
  const e = LIVE.epgSources.find((x) => x.id === p.id); const n = LIVE.mappings.filter((m) => m[1] === e.id).length;
  return { title: `Quelle «${esc(e.name)}» entfernen?`, body: `<p class="lead">${fmt.num(e.channels || 0)} Kanäle und ${fmt.num(e.programmes || 0)} Sendungen dieser Quelle verschwinden aus dem Programm.</p><p class="muted">${n ? `${abPl(n, 'Sender ist', 'Sender sind')} ihr zugeordnet (${abPl(LIVE.mappings.filter((m) => m[1] === e.id && m[3] === 'manual').length, 'Zuordnung von Hand', 'Zuordnungen von Hand')}). Sie stehen danach auf «fehlt». ` : ''}Das Fernsehen selbst läuft weiter. Geplante Aufnahmen bleiben, bekommen aber bei Programmänderungen keine neuen Zeiten mehr.</p>`,
    actions: `<button type="button" class="btn secondary" data-act="close" data-fid="epgd:x" data-autofocus>Abbrechen</button><button type="button" class="btn danger" data-act="abEpgDelOk" data-id="${e.id}" data-fid="epgd:ok">${icon('trash', 16)}Entfernen</button>` };
};
ACT.abEpgDelOk = (dd) => {
  const k = LIVE.epgSources.findIndex((x) => x.id === dd.id); if (k < 0) return; const [e] = LIVE.epgSources.splice(k, 1);
  LIVE.mappings.filter((m) => m[1] === e.id).forEach((m) => abSetKind(m, 'missing', null, null, 'Quelle entfernt'));
  closeDialog(); render(); toast(`Quelle «${esc(e.name)}» entfernt. Ihre Zuordnungen stehen jetzt auf «fehlt».`);
};

/* Formular: anlegen und ändern im selben Dialog */
const abEpgNew = () => ({ id: null, kind: 'xmltv-url', name: '', address: '', file: '', fileSize: '', xsrc: (LIVE.sources.find((s) => s.kind === 'xtream') || {}).id || '', interval: 12, shift: 0, tz: 'Europe/Zurich', comp: 'auto', localNet: false, ua: '', hdr: [], tried: false });
ACT.abEpgAdd = () => { ui.draft.epgf = abEpgNew(); ['ab:epgf:hdr'].forEach((k) => { ui.open[k] = false; }); openDialog('abEpgForm', {}); };
ACT.abEpgEdit = (dd) => {
  const e = LIVE.epgSources.find((x) => x.id === dd.id);
  ui.draft.epgf = Object.assign(abEpgNew(), { id: e.id, kind: e.kind, name: e.name, address: e.kind === 'xtream' ? '' : e.address, file: e.file || '', fileSize: e.fileSize || '', xsrc: e.xsrc || abEpgNew().xsrc, interval: e.interval, shift: e.shift || 0, tz: e.tz, comp: e.compression, localNet: !!e.localNet, ua: e.ua || '', hdr: abClone(e.hdr || []) });
  ui.open['ab:epgf:hdr'] = false; openDialog('abEpgForm', {});
};
const abIsPrivate = (h) => /^(10\.|192\.168\.|127\.|172\.(1[6-9]|2\d|3[01])\.|localhost|239\.)/i.test(h);
function abEpgErrors(d) {
  const e = {}; const name = d.name.trim();
  if (!name) e.name = 'Gib der Quelle einen Namen.'; else if (LIVE.epgSources.some((x) => x.id !== d.id && x.name.toLowerCase() === name.toLowerCase())) e.name = 'Diesen Namen gibt es schon.';
  if (d.kind === 'xmltv-url') { if (!/^https?:\/\/[^\s/]+/i.test(d.address.trim())) e.address = 'Gib eine Adresse mit http:// oder https:// ein.'; else if (abIsPrivate(abHostOf(d.address))) e.address = 'Adressen im Heimnetz gibt es für Programmdaten noch nicht (offene Frage 09 F-4). Nimm eine öffentliche Adresse.'; }
  if (d.kind === 'xmltv-file' && !d.file) e.file = 'Wähle eine XMLTV-Datei.';
  if (d.kind === 'xtream' && !d.xsrc) e.xsrc = 'Es gibt noch keine Xtream-Quelle. Lege zuerst unter «Quellen» eine an.';
  if (!Number.isInteger(d.shift) || d.shift < -12 || d.shift > 12) e.shift = 'Gib eine ganze Zahl von −12 bis +12 ein.';
  d.hdr.forEach((h, i) => { if (!h.k.trim() && (h.v || h.set)) e['h' + i] = 'Gib der Kopfzeile einen Namen.'; });
  return e;
}
DIALOGS.abEpgForm = () => {
  const d = ui.draft.epgf; const e = d.tried ? abEpgErrors(d) : {}; const P = 'ui.draft.epgf'; const xs = LIVE.sources.filter((s) => s.kind === 'xtream');
  const kind = (k, ic, t, x) => `<label class="aa-card"><input type="radio" name="epgk" value="${k}" data-bind="${P}.kind" ${d.kind === k ? 'checked' : ''} data-fid="epgf:k:${k}"><span class="ic">${icon(ic, 20)}</span><span><b>${t}</b><small>${x}</small></span></label>`;
  const fields = d.kind === 'xmltv-url' ? abFld('epgf-a', 'Adresse der XMLTV-Datei', abInp(`${P}.address`, 'epgf-a', { err: e.address, ph: 'https://epg.example.org/programm.xml.gz', label: 'Adresse' }), { err: e.address, hint: 'Gepackte Dateien (.gz, .xz) liest Slouchcat selbst.' })
    : d.kind === 'xmltv-file' ? abFld('epgf-f', 'XMLTV-Datei', `<div class="row-gap"><button type="button" class="btn secondary" data-act="abEpgFile" data-fid="epgf:file">${icon('upload', 16)}Datei wählen</button><span id="epgf-f">${d.file ? `<span class="mono">${esc(d.file)}</span> (${esc(d.fileSize)})` : '<span class="muted">noch keine Datei</span>'}</span></div>`, { err: e.file })
      : abFld('epgf-x', 'Xtream-Zugang', xs.length ? abSel(`${P}.xsrc`, 'epgf-x', xs.map((s) => [s.id, s.name]), { label: 'Xtream-Zugang' }) : `<span class="muted">Es gibt keine Xtream-Quelle.</span>`, { err: e.xsrc, hint: 'Slouchcat holt die Programmdaten über denselben Zugang wie die Senderliste. Zugangsdaten stehen nur dort.' });
  const hdr = `<div class="stack">${abFld('epgf-ua', 'User-Agent', abInp(`${P}.ua`, 'epgf-ua', { ph: 'leer: Standard von Slouchcat', label: 'User-Agent' }))}
    ${d.hdr.map((h, i) => `<div class="aa-hrow"><div class="fld"><label for="epgf-hk${i}">Kopfzeile ${i + 1}</label>${abInp(`${P}.hdr.${i}.k`, `epgf-hk${i}`, { err: e['h' + i], ph: 'Name' })}</div><div class="fld"><label for="epgf-hv${i}">Wert</label><input class="inp" id="epgf-hv${i}" type="password" data-bind="${P}.hdr.${i}.v" data-fid="epgf-hv${i}" value="${esc(h.v)}" placeholder="${h.set ? 'gespeichert, leer lassen zum Behalten' : 'Wert'}" autocomplete="off" data-live></div><button type="button" class="btn quiet dq" data-act="abEpgHdrDel" data-i="${i}" data-fid="epgf:hd:${i}" aria-label="Kopfzeile ${i + 1} entfernen">${icon('trash', 16)}</button>${abErr('epgf-hk' + i, e['h' + i])}</div>`).join('')}
    <div><button type="button" class="btn secondary" data-act="abEpgHdrAdd" data-fid="epgf:hadd">${icon('plus', 16)}Kopfzeile hinzufügen</button></div><p class="muted aa-small">Werte erscheinen nach dem Speichern nie wieder.</p></div>`;
  return { title: d.id ? `EPG-Quelle «${esc(LIVE.epgSources.find((x) => x.id === d.id).name)}» ändern` : 'EPG-Quelle hinzufügen', wide: true,
    body: `<div class="stack">${abFld('epgf-n', 'Name', abInp(`${P}.name`, 'epgf-n', { err: e.name, auto: true, ph: 'zum Beispiel «XMLTV Schweiz»', label: 'Name' }), { err: e.name })}
      <fieldset class="aa-cards" aria-label="Art der Quelle">${kind('xmltv-url', 'link', 'XMLTV-Adresse', 'Eine Adresse zu einer XMLTV-Datei, auch gepackt.')}${kind('xmltv-file', 'upload', 'XMLTV-Datei', 'Eine Datei, die du hochlädst.')}${kind('xtream', 'plug', 'Von Xtream', 'Die Programmdaten deines Xtream-Zugangs.')}</fieldset>
      ${fields}
      <div class="aa-two">${abFld('epgf-i', 'Aktualisieren', abSel(`${P}.interval`, 'epgf-i', AB_EINT, { num: true, label: 'Aktualisieren' }))}${abFld('epgf-c', 'Kompression', abSel(`${P}.comp`, 'epgf-c', AB_COMP, { label: 'Kompression' }))}
      ${abFld('epgf-z', 'Zeitzone für Zeiten ohne Angabe', abSel(`${P}.tz`, 'epgf-z', AB_TZ, { label: 'Zeitzone' }))}${abFld('epgf-s', 'Zeitkorrektur', `<span class="ab-unit">${abInp(`${P}.shift`, 'epgf-s', { num: true, min: -12, max: 12, err: e.shift, label: 'Zeitkorrektur in Stunden' })}<span>Stunden</span></span>`, { err: e.shift, hint: 'Nur für Quellen mit falschen Zeiten. −12 bis +12.' })}</div>
      <p class="muted aa-small">Die Adresse ist nach dem Speichern nur schreibbar; angezeigt werden Schema, Host und Port. Kopfzeilen und User-Agent gibt es für Programmdaten nicht.</p></div>`,
    actions: `<button type="button" class="btn secondary" data-act="close" data-fid="epgf:x">Abbrechen</button><button type="button" class="btn primary" data-act="abEpgSave" data-fid="epgf:ok">${d.id ? 'Speichern' : 'Quelle anlegen'}</button>` };
};
ACT.abEpgFile = () => { const d = ui.draft.epgf; d.file = 'programm-beispiel.xml.gz'; d.fileSize = '18 MB'; render(); };
ACT.abEpgHdrAdd = () => { ui.draft.epgf.hdr.push({ k: '', v: '', set: false }); ui.open['ab:epgf:hdr'] = true; render(); setTimeout(() => { const el = document.querySelector(`[data-fid="epgf-hk${ui.draft.epgf.hdr.length - 1}"]`); if (el) el.focus(); }, 0); };
ACT.abEpgHdrDel = (dd) => { ui.draft.epgf.hdr.splice(Number(dd.i), 1); render(); };
ACT.abEpgSave = () => {
  const d = ui.draft.epgf; d.tried = true; const er = abEpgErrors(d);
  if (Object.keys(er).length) { render(); setTimeout(() => { const el = document.querySelector('#ov-dialog [aria-invalid="true"], #ov-dialog .ab-err'); if (el && el.focus) el.focus(); }, 0); return; }
  const xs = LIVE.sources.find((s) => s.id === d.xsrc);
  const vals = { name: d.name.trim(), kind: d.kind, address: d.kind === 'xmltv-url' ? d.address.trim() : d.kind === 'xtream' ? `vom Xtream-Zugang «${xs ? xs.name : ''}»` : '', file: d.kind === 'xmltv-file' ? d.file : '', fileSize: d.kind === 'xmltv-file' ? d.fileSize : '', xsrc: d.kind === 'xtream' ? d.xsrc : '', interval: d.interval, shift: d.shift, tz: d.tz, compression: d.comp, localNet: d.localNet, ua: d.ua.trim(), hdr: d.hdr.filter((h) => h.k.trim()).map((h) => ({ k: h.k.trim(), v: '', set: h.set || !!h.v })) };
  let e;
  if (d.id) { e = LIVE.epgSources.find((x) => x.id === d.id); Object.assign(e, vals); if (e.interval && e.last) e.next = e.last + e.interval * 60; }
  else { e = Object.assign({ id: 'epg-' + (abNorm(vals.name) || 'neu') + '-' + (LIVE.epgSources.length + 1), last: null, lastOk: true, channels: 0, programmes: 0, until: SIM.now(), warnings: [] }, vals); LIVE.epgSources.push(e); AB.just['epg:' + e.id] = true; ui.open['ab:epg:' + e.id] = true; }
  delete ui.draft.epgf; closeDialog(); render(); toast(d.id ? `«${esc(e.name)}» gespeichert.` : `«${esc(e.name)}» angelegt. Slouchcat lädt die Programmdaten jetzt zum ersten Mal.`);
  if (!d.id) ACT.abEpgLoad({ id: e.id });
};

function abEpgRow(e) {
  const open = isOpen('ab:epg:' + e.id, !e.lastOk); const b = AB.epgBusy[e.id]; const fresh = AB.just['epg:' + e.id]; const runs = (LIVE.epgRuns[e.id] || []).slice(0, 5); const warns = abEpgWarn(e);
  const st = b ? `<span class="ab-run"><span class="spin"></span>läuft … ${fmt.num(b.n)} von ${fmt.num(b.of)} Sendungen</span>` : !e.last ? '<span class="muted">noch nie</span>'
    : `<span class="ab-st">${abDot(e.lastOk ? (warns.length ? 'warn' : 'ok') : 'bad')}${abWhen(e.last)} · ${e.lastOk ? (warns.length ? 'mit Hinweisen' : 'in Ordnung') : 'fehlgeschlagen'}</span>${!e.lastOk && e.lastError ? `<small class="ab-bad">${esc(e.lastError)}. Der alte Stand bleibt aktiv.</small>` : ''}`;
  const menu = popMenu('ab:epg:' + e.id, icon('dots', 16), [{ label: 'Bearbeiten', act: 'abEpgEdit', data: { id: e.id }, icon: 'pen' }, { label: 'Entfernen …', act: 'abEpgDel', data: { id: e.id }, danger: true, icon: 'trash' }], { aria: `Weitere Aktionen für ${e.name}`, cls: 'quiet icon' });
  const cover = abEpgCover(e); const short = cover < LIVE.settings.epg.previewDays && e.channels;
  const rr = { ok: ['ok', 'in Ordnung'], warn: ['warn', 'mit Hinweis'], bad: ['bad', 'fehlgeschlagen'] };
  return `<tr class="${open ? 'open' : ''}"><td class="main" data-l="Name"><span class="ab-name"><button type="button" class="tog" data-act="band" data-def="${e.lastOk ? 0 : 1}" data-id="ab:epg:${e.id}" aria-expanded="${open}" aria-label="${esc(e.name)}: Einzelheiten ${open ? 'zuklappen' : 'aufklappen'}" data-fid="la10:tog:${e.id}">${icon('chevR', 14)}</button><button type="button" class="lnk" data-act="abEpgEdit" data-id="${e.id}" data-fid="la10:name:${e.id}"><b>${esc(e.name)}</b></button>${fresh ? '<span class="chip ok">Neu angelegt</span>' : ''}</span></td>
    <td data-l="Art">${AB_EKIND[e.kind]}</td><td data-l="Intervall">${e.interval ? `alle ${e.interval} Std.` : 'nur von Hand'}</td><td data-l="Letzter Import">${st}</td>
    <td class="end" data-l="Kanäle">${e.channels ? fmt.num(e.channels) : '–'}</td><td class="end" data-l="Sendungen">${e.programmes ? fmt.num(e.programmes) : '–'}</td><td data-l="Reicht bis">${e.channels ? `${fmt.day(e.until)}${short ? ` <span class="chip warn">nur ${cover} Tage</span>` : ''}` : '–'}</td>
    <td class="end"><div class="ta"><button type="button" class="btn quiet" data-act="abEpgLoad" data-id="${e.id}" data-fid="la10:load:${e.id}" ${b ? 'disabled' : ''}>${icon('refresh', 16)}Jetzt laden</button><button type="button" class="btn quiet" data-act="abEpgTest" data-id="${e.id}" data-fid="la10:test:${e.id}">Testen</button>${menu}</div></td></tr>
    ${open ? `<tr class="sub"><td colspan="8"><div class="ab-det"><dl class="kv"><dt>Adresse</dt><dd>${e.kind === 'xmltv-file' ? `Datei <span class="mono">${esc(e.file)}</span> (${esc(e.fileSize)})` : `<span class="mono">${esc(e.address)}</span>`}</dd><dt>Kompression</dt><dd>${abEpgComp(e)}</dd><dt>Zeitzone</dt><dd>${esc(e.tz)} · Zeitkorrektur ${abEpgShift(e)}</dd><dt>Nächster Import</dt><dd>${abEpgNext(e)}</dd></dl>
      <h3 class="la02-h3">Letzte fünf Läufe</h3>${runs.length ? `<ul class="feed">${runs.map(([t, r, txt]) => `<li><time>${fmt.day(t)} ${fmt.time(t)}</time><span>${abDot(rr[r][0])} ${rr[r][1]} · ${esc(String(txt).replace('als Ortszeit gelesen', `als ${e.tz} gelesen`))}</span></li>`).join('')}</ul>` : '<p class="muted">Noch keine Läufe.</p>'}
      ${warns.length ? `<h3 class="la02-h3">Hinweise</h3><ul class="checks">${warns.map((w) => `<li class="warn">${icon('alert', 18)}<span><b>${esc(w)}</b></span></li>`).join('')}</ul>` : ''}</div></td></tr>` : ''}`;
}
function abEpgSources() {
  const list = LIVE.epgSources; const failed = list.filter((e) => !e.lastOk);
  const base = { previewDays: LIVE.settings.epg.previewDays }; const d = abDraft('epgset', base); const dirty = abDirty('epgset', base);
  const cover = list.filter((e) => e.channels && abEpgCover(e) < d.previewDays);
  const fail = failed.length ? notice('warn', `Import «${esc(failed[0].name)}» fehlgeschlagen (${esc(failed[0].lastError)}).`, `Der alte Stand bleibt aktiv: Das Programm reicht weiter bis ${fmt.day(failed[0].until)}; nächster Versuch ${abEpgNext(failed[0])}. Das Fernsehen selbst läuft ungestört.`, `<button type="button" class="btn secondary" data-act="abEpgLoad" data-id="${failed[0].id}" data-fid="la10:retry">${icon('refresh', 16)}Jetzt erneut laden</button><button type="button" class="btn ghost" data-act="abEpgTest" data-id="${failed[0].id}" data-fid="la10:failtest">Quelle testen</button>`) : '';
  const tbl = `<div class="tblw ab-tbl"><table class="tbl"><thead><tr><th>Name</th><th>Art</th><th>Intervall</th><th>Letzter Import</th><th class="end">Kanäle</th><th class="end">Sendungen</th><th>Reicht bis</th><th class="end"><span class="vh">Aktionen</span></th></tr></thead><tbody>${list.map(abEpgRow).join('')}</tbody></table></div>`;
  const set = `<section class="card"><div class="ch"><span class="ik">${icon('settings', 18)}</span><h2>Einstellungen</h2></div>
    ${abRow('Vorschauzeitraum', 'So weit in die Zukunft zeigt das Programm und plant Serienaufnahmen.', `${abSel('ui.draft.epgset.previewDays', 'la10-pd', Array.from({ length: 14 }, (_, i) => [i + 1, `${i + 1} ${i ? 'Tage' : 'Tag'}`]), { num: true, label: 'Vorschauzeitraum' })}${cover.length ? `<span class="ab-bad ab-full">${icon('alert', 14)}${cover.map((e) => `«${esc(e.name)}» liefert nur ${abEpgCover(e)} Tage`).join(', ')}.</span>` : ''}`)}
    <p class="muted">Ein Ausfall der Programmdaten beeinträchtigt das Fernsehen nicht. Nur Programm, Erinnerungen und Serienaufnahmen können dann veraltet sein.</p></section>`;
  return `${fail}${tbl}${set}${dirty ? abSaveBar('abEpgSetSave', 'abEpgSetDisc', 'la10', 0) : ''}`;
}
ACT.abEpgSetDisc = () => { delete ui.draft.epgset; render(); };
ACT.abEpgSetSave = () => { LIVE.settings.epg.previewDays = ui.draft.epgset.previewDays; delete ui.draft.epgset; render(); toast(`Vorschauzeitraum: ${LIVE.settings.epg.previewDays} Tage.`); };

/* ======================================================================================
   LA-11 EPG-Zuordnungen
   ====================================================================================== */
const AB_MK = { auto: ['automatisch', 'ok'], unsure: ['unsicher', 'warn'], manual: ['von Hand', 'info'], missing: ['fehlt', 'bad'], without: ['bewusst ohne', 'info'] };
const AB_MCK = { auto: 'autoSafe', unsure: 'unsure', manual: 'manual', missing: 'missing', without: 'without' };
const abEpgName = (id) => (LIVE.epgSources.find((e) => e.id === id) || { name: 'Quelle entfernt' }).name;
function abMapRows() {
  LIVE.channels.filter((c) => c.active).forEach((c) => { if (!LIVE.mappings.find((m) => m[0] === c.id)) LIVE.mappings.push([c.id, null, null, 'missing', 'kein passender Kanal gefunden']); });
  return LIVE.mappings.filter((m) => { const c = SIM.channel(m[0]); return c && c.active; });
}
/** Art einer Zuordnung ändern; passt Zahlen und den Sender an (Programm erscheint oder verschwindet) */
function abSetKind(m, kind, src, key, reason) {
  const C = LIVE.mappingCounts; const was = m[3];
  if (was !== kind) { C[AB_MCK[was]] = Math.max(0, (C[AB_MCK[was]] || 0) - 1); C[AB_MCK[kind]] = (C[AB_MCK[kind]] || 0) + 1; }
  m[1] = src; m[2] = key; m[3] = kind; m[4] = reason;
  const c = SIM.channel(m[0]);
  if (c) {
    if (kind === 'missing' || kind === 'without') { if (c.epg !== 'none') c.epgWas = c.epg; c.epg = 'none'; c.epgKey = null; }
    else { c.epgKey = key; if (c.epg === 'none') c.epg = c.epgWas && c.epgWas !== 'none' ? c.epgWas : LIVE.programs.some((p) => p.ch === c.id) ? 'ok' : 'none'; }
  }
}
function abMapFilterHtml() {
  const C = LIVE.mappingCounts; const f = AB.map.f;
  const b = (k, label, n, cls) => `<button type="button" class="stat ab-fstat ${cls || ''}" aria-pressed="${f === k}" data-act="abMapF" data-k="${k}" data-fid="la11:f:${k}"><span class="l">${label}</span><span class="v">${fmt.num(n)}</span></button>`;
  return `<div class="stats ab-fstats" style="--n:6" role="group" aria-label="Zuordnungen filtern">${b('all', 'Alle', C.all)}${b('auto', 'Automatisch sicher', C.autoSafe, 'ok')}${b('unsure', 'Unsicher', C.unsure, C.unsure ? 'warn' : '')}${b('manual', 'Von Hand', C.manual)}${b('missing', 'Fehlt', C.missing, C.missing ? 'bad' : '')}${b('without', 'Bewusst ohne', C.without)}</div>`;
}
ACT.abMapF = (dd) => { AB.map.f = dd.k; AB.map.sel = {}; render(); };
ACT.abMapSort = (dd) => { abSortToggle(AB.map, dd.k); render(); };
function abMapPreview(chId) {
  if (SIM.epgOf(chId) === 'none') return '<span class="muted">–</span>';
  const nn = SIM.nowNext(chId); const it = [nn.now, nn.next].filter(Boolean);
  return it.length ? it.map((p) => `${fmt.time(p.start)} ${esc(p.title)}`).join(' · ') : '<span class="muted">–</span>';
}
function abMapTable() {
  const all = abMapRows(); const M = AB.map; const q = (ui.draft.mpq || '').trim().toLowerCase();
  let rows = all.filter((m) => (M.f === 'all' || m[3] === M.f) && (!q || SIM.channel(m[0]).name.toLowerCase().includes(q) || String(SIM.channel(m[0]).no) === q || String(m[2] || '').toLowerCase().includes(q)));
  const key = { no: (m) => { const c = SIM.channel(m[0]); return c.no == null ? 9999 : c.no; }, kind: (m) => ['missing', 'unsure', 'manual', 'auto', 'without'].indexOf(m[3]), name: (m) => SIM.channel(m[0]).name.toLowerCase() }[M.sort];
  rows = rows.sort((a, b) => (key(a) < key(b) ? -1 : key(a) > key(b) ? 1 : 0) * M.dir);
  const sel = Object.keys(M.sel).filter((k) => M.sel[k]); const unsureVis = rows.filter((m) => m[3] === 'unsure');
  const tr = rows.map((m) => {
    const c = SIM.channel(m[0]); const k = AB_MK[m[3]]; const isSel = !!M.sel[m[0]];
    const menu = popMenu('ab:map:' + m[0], icon('dots', 16), [m[3] !== 'missing' ? { label: 'Zuordnung entfernen', act: 'abMapRemove', data: { ch: m[0] }, icon: 'x' } : null, m[3] !== 'without' ? { label: 'Ohne Programmdaten', act: 'abMapWithout', data: { ch: m[0] }, icon: 'ban' } : null].filter(Boolean), { aria: `Weitere Aktionen für ${c.name}`, cls: 'quiet icon' });
    return `<tr class="${isSel ? 'sel' : ''}"><td class="ab-cb">${m[3] === 'unsure' ? `<input type="checkbox" ${isSel ? 'checked' : ''} data-act="abMapSel" data-ch="${m[0]}" data-fid="la11:sel:${m[0]}" aria-label="${esc(c.name)} auswählen">` : ''}</td>
      <td class="main" data-l="Sender">${abChLabel(c)}</td><td data-l="Zuordnung">${m[1] ? `<span class="ab-map">${esc(abEpgName(m[1]))} › <span class="mono">${esc(m[2])}</span></span>` : '<span class="muted">keine</span>'}</td>
      <td data-l="Art"><span class="chip ${k[1]}">${k[0]}</span></td><td data-l="Grund">${esc(m[4] || '')}</td><td data-l="Vorschau">${abMapPreview(m[0])}</td>
      <td class="end"><div class="ta">${m[3] === 'unsure' ? `<button type="button" class="btn quiet" data-act="abMapOk" data-ch="${m[0]}" data-fid="la11:ok:${m[0]}">${icon('check', 16)}Bestätigen</button>` : ''}<button type="button" class="btn quiet" data-act="abMapEdit" data-ch="${m[0]}" data-fid="la11:ed:${m[0]}">Ändern …</button>${menu}</div></td></tr>`;
  }).join('');
  const allSel = unsureVis.length && unsureVis.every((m) => M.sel[m[0]]);
  const tbl = rows.length ? `<div class="tblw ab-tbl ab-mt"><table class="tbl"><thead><tr><th class="ab-cb">${unsureVis.length ? `<input type="checkbox" ${allSel ? 'checked' : ''} data-act="abMapSelAll" data-fid="la11:selall" aria-label="Alle sichtbaren Vorschläge auswählen">` : '<span class="vh">Auswahl</span>'}</th>${abSortHead(M, 'no', 'Sender', 'abMapSort')}<th>Zuordnung</th>${abSortHead(M, 'kind', 'Art', 'abMapSort')}<th>Grund</th><th>Vorschau</th><th class="end"><span class="vh">Aktionen</span></th></tr></thead><tbody>${tr}</tbody></table></div>`
    : abEmptyBox('Keine Treffer', q ? 'Zu dieser Suche gibt es keine Zuordnung. Passe Suche oder Filter an.' : 'In dieser Gruppe gibt es keine Sender.', `<button type="button" class="btn secondary" data-act="abMapF" data-k="all" data-fid="la11:allbtn">Alle anzeigen</button>`);
  const C = LIVE.mappingCounts;
  const bulk = sel.length ? `<div class="savebar aa-bulk" role="region" aria-label="Auswahl bestätigen"><b class="grow">${abPl(sel.length, 'Vorschlag', 'Vorschläge')} ausgewählt</b><button type="button" class="btn ghost" data-act="abMapSelClear" data-fid="la11:selclr">Auswahl aufheben</button><button type="button" class="btn primary" data-act="abMapOkSel" data-fid="la11:oksel">${icon('check', 16)}Ausgewählte bestätigen</button></div>` : '';
  return `${tbl}<p class="muted aa-small">Das Beispiel zeigt ${fmt.num(all.length)} der ${fmt.num(C.all)} aktiven Sender einzeln. Je Sender gilt genau eine Quelle; Zuordnungen von Hand überstehen jede Aktualisierung.</p>${bulk}`;
}
ACT.abMapSel = (dd, el) => { AB.map.sel[dd.ch] = el.checked; render(); };
ACT.abMapSelAll = (dd, el) => { abMapRows().filter((m) => m[3] === 'unsure' && (AB.map.f === 'all' || AB.map.f === 'unsure')).forEach((m) => { AB.map.sel[m[0]] = el.checked; }); render(); };
ACT.abMapSelClear = () => { AB.map.sel = {}; render(); };
const abSnap = (m) => ({ m: abClone(m), counts: abClone(LIVE.mappingCounts), epg: (SIM.channel(m[0]) || {}).epg, key: (SIM.channel(m[0]) || {}).epgKey });
AB.undo = {};
ACT.abMapUndo = (dd) => { const s = AB.undo[dd.ch]; if (!s) return; const m = LIVE.mappings.find((x) => x[0] === dd.ch); Object.assign(m, s.m); LIVE.mappingCounts = s.counts; const c = SIM.channel(dd.ch); c.epg = s.epg; c.epgKey = s.key; delete AB.undo[dd.ch]; render(); toast('Rückgängig gemacht.'); };
ACT.abMapOk = (dd) => { const m = LIVE.mappings.find((x) => x[0] === dd.ch); AB.undo[dd.ch] = abSnap(m); abSetKind(m, 'manual', m[1], m[2], 'bestätigt am ' + fmt.day(SIM.now())); delete AB.map.sel[dd.ch]; render(); toastUndo(`Zuordnung für <b>${esc(SIM.channel(dd.ch).name)}</b> bestätigt (jetzt «von Hand»)`, 'abMapUndo', { ch: dd.ch }); };
ACT.abMapRemove = (dd) => { const m = LIVE.mappings.find((x) => x[0] === dd.ch); AB.undo[dd.ch] = abSnap(m); abSetKind(m, 'missing', null, null, 'Zuordnung von Hand entfernt'); render(); toastUndo(`Zuordnung für <b>${esc(SIM.channel(dd.ch).name)}</b> entfernt`, 'abMapUndo', { ch: dd.ch }); };
ACT.abMapWithout = (dd) => { const m = LIVE.mappings.find((x) => x[0] === dd.ch); AB.undo[dd.ch] = abSnap(m); abSetKind(m, 'without', null, null, 'bewusst ohne Programmdaten'); render(); toastUndo(`<b>${esc(SIM.channel(dd.ch).name)}</b> bleibt bewusst ohne Programmdaten`, 'abMapUndo', { ch: dd.ch }); };
/* Sammelbestätigung */
ACT.abMapOkSel = () => { ui.draft.mpBulk = 'sel'; openDialog('abMapBulk', {}); };
ACT.abMapOkAll = () => { ui.draft.mpBulk = 'all'; openDialog('abMapBulk', {}); };
DIALOGS.abMapBulk = () => {
  const all = ui.draft.mpBulk === 'all'; const rows = abMapRows().filter((m) => m[3] === 'unsure' && (all || AB.map.sel[m[0]])); const C = LIVE.mappingCounts; const extra = all ? Math.max(0, C.unsure - rows.length) : 0;
  return { title: `${all ? C.unsure : rows.length} Vorschläge bestätigen?`, body: `<p class="lead">Jeder bestätigte Vorschlag wird zu einer Zuordnung «von Hand». Sie übersteht jede Aktualisierung der Quellen.</p><ul class="ab-list">${rows.map((m) => `<li><b>${esc(SIM.channel(m[0]).name)}</b> <span class="muted">› ${esc(m[2])} (${esc(m[4])})</span></li>`).join('')}${extra ? `<li class="muted">und ${abPl(extra, 'weiterer Vorschlag', 'weitere Vorschläge')}, die hier nicht einzeln gezeigt werden</li>` : ''}</ul>`,
    actions: `<button type="button" class="btn secondary" data-act="close" data-fid="mpb:x" data-autofocus>Abbrechen</button><button type="button" class="btn primary" data-act="abMapBulkOk" data-fid="mpb:ok">${icon('check', 16)}Bestätigen</button>` };
};
ACT.abMapBulkOk = () => {
  const all = ui.draft.mpBulk === 'all'; const rows = abMapRows().filter((m) => m[3] === 'unsure' && (all || AB.map.sel[m[0]])); const C = LIVE.mappingCounts; const extra = all ? Math.max(0, C.unsure - rows.length) : 0;
  rows.forEach((m) => abSetKind(m, 'manual', m[1], m[2], 'bestätigt am ' + fmt.day(SIM.now())));
  if (extra) { C.unsure -= extra; C.manual += extra; }
  AB.map.sel = {}; closeDialog(); render(); toast(`${abPl(rows.length + extra, 'Vorschlag', 'Vorschläge')} bestätigt.`);
};
ACT.abMapAuto = () => {
  const C = LIVE.mappingCounts; const n = Math.max(0, C.missing - 3);
  if (!n) { toast('0 Sender zugeordnet, ' + abPl(C.missing, 'Sender bleibt', 'Sender bleiben') + ' ohne.', 'warn'); return; }
  C.missing -= n; C.autoSafe += n; render(); toast(`${abPl(n, 'Sender', 'Sender')} zugeordnet, ${C.missing} bleiben ohne.`);
};
/* Zuordnung ändern: Suche über alle Kanäle aller EPG-Quellen */
const AB_TITLES = ['Tagesschau', 'Wetter und Verkehr', 'Spielfilm', 'Dokumentation', 'Nachrichten', 'Magazin', 'Quiz', 'Krimi', 'Sportstudio', 'Kindersendung'];
function abCatalogue() {
  const out = []; const add = (src, key) => { if (LIVE.epgSources.some((e) => e.id === src) && !out.some((o) => o.src === src && o.key === key)) out.push({ src, key }); };
  LIVE.mappings.forEach((m) => { if (m[1]) add(m[1], m[2]); });
  ['SRF1.ch', 'SRFzwei.ch', 'SRFinfo.ch', 'TeleZueri.ch', 'TeleZH.ch', 'RTSUn.ch', 'RTSDeux.ch', 'RSILa1.ch', 'RSILa2.ch', 'DasErste.de', 'ARD.de', 'ZDF.de', 'ZDFinfo.de', 'zdf.info.de', 'ZDFneo.de', 'arte.de', 'arte.fr', '3sat.de', 'phoenix.de', 'KiKA.de', 'ORF1.at', 'ORF2.at', 'ORF2W.at', 'BlueZoom.ch', 'Kanal9.ch'].forEach((k) => add('epg-xmltv', k));
  ['ORF2.at', 'ORF1.at', 'ZDFinfo.de', 'TeleZueri.ch', 'Kanal9.ch'].forEach((k) => add('epg-xtream', k));
  return out;
}
function abCatPreview(key) {
  const c = LIVE.channels.find((x) => x.epgKey === key && x.epg !== 'none');
  if (c) { const nn = SIM.nowNext(c.id); const it = [nn.now, nn.next].filter(Boolean); if (it.length) return it.map((p) => `${fmt.time(p.start)} ${esc(p.title)}`).join(' · '); }
  let h = 0; for (const ch of key) h = (h * 7 + ch.charCodeAt(0)) % 97; const t0 = Math.floor(SIM.now() / 5) * 5 - (h % 40);
  return `${fmt.time(t0)} ${AB_TITLES[h % 10]} · ${fmt.time(t0 + 45 + (h % 3) * 15)} ${AB_TITLES[(h + 3) % 10]}`;
}
ACT.abMapEdit = (dd) => { const m = LIVE.mappings.find((x) => x[0] === dd.ch); ui.draft.mpq2 = ''; ui.draft.mpsel = m[1] ? `${m[1]}|${m[2]}` : ''; openDialog('abMapEdit', { ch: dd.ch }); };
DIALOGS.abMapEdit = (p) => {
  const c = SIM.channel(p.ch); const q = abNorm(ui.draft.mpq2); const cat = abCatalogue().filter((o) => !q || abNorm(o.key).includes(q) || abNorm(abEpgName(o.src)).includes(q));
  const items = cat.slice(0, 40).map((o, i) => { const v = `${o.src}|${o.key}`; return `<label class="check ab-opt"><input type="radio" name="mpsel" value="${esc(v)}" data-bind="ui.draft.mpsel" ${ui.draft.mpsel === v ? 'checked' : ''} data-fid="mpe:o:${i}"><span><b class="mono">${esc(o.key)}</b> <small>${esc(abEpgName(o.src))}</small><small>Vorschau: ${abCatPreview(o.key)}</small></span></label>`; }).join('');
  return { title: `EPG-Kanal für ${esc(c.name)} wählen`, wide: true,
    body: `<div class="fld"><label for="mpe-q">Suche über alle Kanäle aller Quellen</label><input class="inp" id="mpe-q" type="search" data-bind="ui.draft.mpq2" data-live data-fid="mpe:q" data-autofocus value="${esc(ui.draft.mpq2 || '')}" placeholder="zum Beispiel «ZDFinfo»" autocomplete="off"></div>
      <div class="ab-optlist" role="radiogroup" aria-label="Treffer">${items || '<p class="muted">Kein Kanal gefunden.</p>'}${cat.length > 40 ? `<p class="muted aa-small">${cat.length - 40} weitere Treffer. Schränke die Suche ein.</p>` : ''}</div>
      <p class="muted aa-small">Je Sender gilt genau eine Quelle. Die Auswahl wird als «von Hand» gespeichert und übersteht jede Aktualisierung.</p>`,
    actions: `<button type="button" class="btn secondary" data-act="close" data-fid="mpe:x">Abbrechen</button><button type="button" class="btn primary" data-act="abMapEditOk" data-ch="${p.ch}" data-fid="mpe:ok"${ui.draft.mpsel ? '' : ' aria-disabled="true"'}>Zuordnung speichern</button>` };
};
ACT.abMapEditOk = (dd) => {
  if (!ui.draft.mpsel) return; const [src, key] = ui.draft.mpsel.split('|'); const m = LIVE.mappings.find((x) => x[0] === dd.ch); AB.undo[dd.ch] = abSnap(m);
  abSetKind(m, 'manual', src, key, 'von Hand am ' + fmt.day(SIM.now())); closeDialog(); render(); toastUndo(`<b>${esc(SIM.channel(dd.ch).name)}</b> ist jetzt «${esc(key)}» zugeordnet`, 'abMapUndo', { ch: dd.ch });
};
function abMapTab(q) {
  if (q.filter && q.filter !== AB.map.qf) { AB.map.qf = q.filter; if (AB_MK[q.filter] || q.filter === 'all') AB.map.f = q.filter === 'auto' ? 'auto' : q.filter; }
  const C = LIVE.mappingCounts;
  return `${C.unsure ? notice('warn', `${abPl(C.unsure, 'Vorschlag wartet', 'Vorschläge warten')} auf deine Prüfung`, 'Bei unsicheren Zuordnungen ist der Name nur ähnlich. Bestätige sie einzeln oder gesammelt, oder ändere sie.', `<button type="button" class="btn secondary" data-act="abMapF" data-k="unsure" data-fid="la11:nfs">Vorschläge ansehen</button><button type="button" class="btn secondary" data-act="abMapOkAll" data-fid="la11:nfa">${icon('check', 16)}Alle ${C.unsure} bestätigen</button>`) : ''}
    ${abMapFilterHtml()}<div class="ab-tools"><div class="fld ab-grow"><label for="la11-q">Sender oder EPG-Kanal suchen</label><input class="inp" id="la11-q" type="search" data-bind="ui.draft.mpq" data-live data-fid="la11:q" value="${esc(ui.draft.mpq || '')}" placeholder="Name, Nummer oder Kanalkennung" autocomplete="off"></div></div>${abMapTable()}`;
}

route('/admin/live/epg', (pm, q) => {
  const tab = q.tab === 'mapping' ? 'mapping' : 'sources';
  const C = LIVE.mappingCounts; const nS = LIVE.epgSources.length;
  const tabs = subTabs('#/admin/live/epg', [['sources', 'Quellen', abEmpty() ? 0 : nS], ['mapping', 'Zuordnungen', abEmpty() ? 0 : C.all]], tab);
  const m = abMode();
  if (tab === 'mapping') {
    const emptyHtml = abEmptyBox('Noch keine Sender', 'Ohne Sender gibt es nichts zuzuordnen. Füge zuerst eine Quelle hinzu.', `<a class="btn primary" href="#/admin/live/sources/new" data-fid="la11:addsrc">${icon('plus', 16)}Quelle hinzufügen</a>`);
    return abView({ page: 'epg', title: 'EPG', intro: 'Welche Programmdaten zu welchem Sender gehören. Je Sender genau ein EPG-Kanal.', live: m === 'normal' ? abLive() : null,
      actions: `<button type="button" class="btn secondary" data-act="abMapAuto" data-fid="la11:auto">${icon('zap', 16)}Automatisch zuordnen</button>`, html: tabs + abGate(emptyHtml, () => abMapTab(q), { rows: 6, h: 56 }) });
  }
  const emptyHtml = abEmptyBox('Noch keine Programmdaten', 'Ohne sie läuft Fernsehen trotzdem, aber Programm, Erinnerungen und Serienaufnahmen fehlen.', `<button type="button" class="btn primary" data-act="abEpgAdd" data-fid="la10:add2">${icon('plus', 16)}EPG-Quelle hinzufügen</button>`);
  return abView({ page: 'epg', title: 'EPG', intro: 'Programmdaten für Programm, Erinnerungen und Serienaufnahmen. Ein Ausfall stört das Fernsehen nicht.', live: m === 'normal' ? abLive() : null,
    actions: `<button type="button" class="btn primary" data-act="abEpgAdd" data-fid="la10:add">${icon('plus', 16)}EPG-Quelle hinzufügen</button>`,
    html: tabs + abGate(!nS || abEmpty() ? emptyHtml : null, abEpgSources, { rows: 3, h: 64 }) });
});
