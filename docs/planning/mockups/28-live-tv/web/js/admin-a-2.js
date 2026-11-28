/* Web Verwaltung A, Teil 2: LA-05 Sender, LA-06 Sender bearbeiten, LA-07 Duplikate und Zusammenführen, LA-08 Sendergruppen
   (15-administration.md 3.5 bis 3.8). LA-20 bis LA-28 (geänderte bestehende Seiten, 3.17) stehen am Ende dieser Datei.
   Hilfen und Zahlen kommen aus admin-a.js (Präfix «aa»). */
'use strict';

/* ---------- gemeinsame Hilfen der Senderseiten ---------- */
const AA2 = { dupDone: {}, opened: null, mapDelta: 0 };
const aaChAll = () => (aaEmpty() ? [] : LIVE.channels.concat(LIVE.goneChannels));
const aaMapOf = (c) => LIVE.mappings.find((m) => m[0] === c.id) || null;
function aaMapKind(c) { const m = aaMapOf(c); if (!m) return c.epg === 'none' ? 'missing' : 'ok'; return m[3] === 'unsure' ? 'unsure' : m[3] === 'missing' ? 'missing' : m[3] === 'without' ? 'without' : 'ok'; }
const AA_MAPL = { ok: 'zugeordnet', unsure: 'unsicher', missing: 'fehlt', without: 'bewusst ohne' };
const AA_RECL = Object.assign({}, LU.RECEPTION, { default: 'Standard (bei Bedarf)' });
const aaNum = (c) => (c.no == null ? '–' : c.no);
const aaDupList = () => (aaEmpty() ? [] : LIVE.duplicates.filter((d) => !AA2.dupDone[d.id] && d.channels.every((id) => SIM.channel(id))));
const aaDupCount = () => aaDupList().length; /* Reiter «Duplikate n» = Zahl der Zeilen */
/* Was läuft gerade auf einem Sender (nur Zahlen) und «Sofort beenden» (15 Grundsatz 6, 3.5, 3.6; `04` T-29) */
if (!LU.REASON.channelOff) LU.REASON.channelOff = 'Der Sender wurde deaktiviert.'; /* fehlt im Kern (shared/live-ui.js) */
const aaRunningOn = (id) => ({ v: admViewersOf(id), r: aaRecsOn(id) });
const aaRunText = (x) => [x.v ? aaPl(x.v, 'Zuschauer', 'Zuschauer') : '', x.r ? aaPl(x.r, 'Aufnahme', 'Aufnahmen') : ''].filter(Boolean).join(' und ');
function aaStopNow(id) {
  const c = SIM.channel(id); const x = aaRunningOn(id);
  LIVE.consumers.forEach((k) => { if (k[0] === id && (k[1] === 'viewer' || k[1] === 'recording')) k[1] = 'ended'; });
  LIVE.profiles.forEach((p) => SIM.recordings(p.id).filter((r) => r.state === 'running' && r.ch === id).forEach((r) => { SIM.stopRecording(r.id); const rr = SIM.recording(r.id); if (rr) { rr.state = 'incomplete'; rr.reason = 'channelOff'; } }));
  const pl = SIM.player(); if (pl && pl.ch === id) SIM.stop();
  const k = LIVE.inputs.findIndex((n) => n.ch === id); if (k >= 0) { const [n] = LIVE.inputs.splice(k, 1); LIVE.resources.bandIn = Math.max(0, Math.round((LIVE.resources.bandIn - (n.bitrate || 0)) * 10) / 10); }
  if (c) c.active = false;
  return x;
}
DIALOGS.aaOffAsk = (p) => {
  const c = SIM.channel(p.id); const x = aaRunningOn(p.id);
  return { title: `${esc(c.name)} ausblenden?`, body: `<p class="lead">${aaRunText(x)} ${x.v + x.r === 1 ? 'läuft' : 'laufen'} weiter bis zu ${x.v + x.r === 1 ? 'seinem' : 'ihrem'} Ende.</p><p class="muted">Deaktivieren macht den Sender sofort für alles Neue unsichtbar und beendet nichts Laufendes.</p>`,
    actions: `<button type="button" class="btn secondary" data-act="close" data-fid="off:x" data-autofocus>Abbrechen</button><button type="button" class="btn primary" data-act="aaOffOk" data-id="${p.id}" data-fid="off:ok">Ausblenden</button><button type="button" class="btn danger" data-act="aaStopAsk" data-id="${p.id}" data-fid="off:stop">Ausblenden und sofort beenden …</button>` };
};
ACT.aaOffOk = (d) => { const c = SIM.channel(d.id); c.active = false; closeDialog(); toastUndo(`«${esc(c.name)}» ist inaktiv. Was läuft, läuft weiter bis zum Ende.`, 'aaChActive', { id: c.id }); render(); };
ACT.aaStopAsk = (d) => openDialog('aaStopC', { id: d.id, back: ui.dialog && ui.dialog.name === 'aaChEd' });
DIALOGS.aaStopC = (p) => {
  const c = SIM.channel(p.id); const x = aaRunningOn(p.id);
  const parts = [x.v ? `${aaPl(x.v, 'Zuschauer sieht', 'Zuschauer sehen')} «Sender nicht verfügbar»` : '', x.r ? `${aaPl(x.r, 'Aufnahme endet', 'Aufnahmen enden')} als «unvollständig, Sender deaktiviert»` : ''].filter(Boolean);
  return { title: `Alles Laufende auf ${esc(c.name)} sofort beenden?`, body: `<p class="lead">${parts.length ? parts.join(', ') + '.' : 'Gerade läuft nichts auf diesem Sender.'}</p><p class="muted">Der Sender wird dabei deaktiviert.</p>`,
    actions: `<button type="button" class="btn secondary" data-act="${p.back ? 'aaStopBack' : 'close'}" data-fid="stop:x" data-autofocus>Abbrechen</button><button type="button" class="btn danger" data-act="aaStopOk" data-id="${p.id}" data-fid="stop:ok">Sofort beenden</button>` };
};
ACT.aaStopBack = () => openDialog('aaChEd', {});
ACT.aaStopOk = (d) => { const c = SIM.channel(d.id); const x = aaStopNow(d.id); closeDialog(); if (/[?&]edit=/.test(location.hash)) go('#/admin/live/channels?tab=channels'); toast(`«${esc(c.name)}» ist deaktiviert. ${aaRunText(x) || 'Nichts'} sofort beendet.`, 'ok'); render(); };
function aaChState(c) {
  const s = SIM.chState(c);
  if (s === 'gone') return `${aaDot('bad')}Verschwunden seit ${fmt.day(c.since)}`;
  if (s === 'disturbed') return `${aaDot('warn')}Gestört seit ${fmt.time(c.since || SIM.now())}`;
  return `${aaDot('ok')}Verfügbar`;
}
const aaPlannedOn = (id) => LIVE.profiles.reduce((a, p) => a + SIM.rules(p.id).filter((r) => r.ch === id || (Array.isArray(r.channels) && r.channels.includes(id))).length, 0);
function aaMapSet(c, sel) { // Zuordnung speichern und die Zähler nachführen
  const kinds = { auto: 'autoSafe', unsure: 'unsure', manual: 'manual', missing: 'missing', without: 'without' }; const old = aaMapOf(c); const oldK = old ? old[3] : (c.epg === 'none' ? 'missing' : 'auto');
  const [src, key] = sel && sel !== 'without' ? sel.split('|') : [null, null]; const nk = sel === 'without' ? 'without' : sel ? 'manual' : 'missing';
  if (old) { old[1] = src; old[2] = key; old[3] = nk; old[4] = nk === 'manual' ? 'von Hand, heute' : nk === 'without' ? 'bewusst ohne' : 'nicht zugeordnet'; } else LIVE.mappings.push([c.id, src, key, nk, 'von Hand, heute']);
  if (kinds[oldK] && oldK !== nk) { LIVE.mappingCounts[kinds[oldK]] = Math.max(0, LIVE.mappingCounts[kinds[oldK]] - 1); LIVE.mappingCounts[kinds[nk]]++; }
  c.epg = sel && sel !== 'without' ? 'ok' : 'none'; c.epgKey = key;
}
function aaEpgOptions() {
  const set = new Map(); LIVE.mappings.forEach((m) => { if (m[1]) set.set(m[1] + '|' + m[2], `${(LIVE.epgSources.find((e) => e.id === m[1]) || { name: m[1] }).name} › ${m[2]}`); });
  set.set('epg-xmltv|zdf.info.de', 'XMLTV Schweiz und Deutschland › zdf.info.de'); set.set('epg-xtream|SRF1.ch', 'Programmdaten Xtream › SRF1.ch');
  return [{ v: '', t: 'Noch nicht zugeordnet' }, { v: 'without', t: 'Ohne Programmdaten (bewusst)' }].concat([...set].map(([v, t]) => ({ v, t })));
}
const aaSw = (on, act, data, label, fid, dis) => `<button type="button" class="sw" role="switch" aria-checked="${on}" aria-label="${esc(label)}" data-act="${act}" ${Object.entries(data).map(([k, v]) => `data-${k}="${esc(v)}"`).join(' ')} data-fid="${fid}"${dis ? ' disabled' : ''}></button>`;
const aaCb = (on, act, data, label, fid) => `<input type="checkbox" ${on ? 'checked' : ''} data-act="${act}" ${Object.entries(data).map(([k, v]) => `data-${k}="${esc(v)}"`).join(' ')} data-fid="${fid}" aria-label="${esc(label)}">`;

/* ======================================================================================
   LA-05 Sender (Tabelle), Reiter Gruppen (LA-08) und Duplikate (LA-07)
   ====================================================================================== */
const aaCF = () => ui.draft.cf || (ui.draft.cf = { q: '', state: '', source: '', group: '', country: '', lang: '', category: '', epg: '', rec: '', sort: 'no', dir: 1 });
const AA_FACETS = [
  ['state', 'Zustand', () => [['', 'Alle'], ['active', 'Aktiv'], ['inactive', 'Inaktiv'], ['new', 'Neu'], ['disturbed', 'Gestört'], ['gone', 'Verschwunden'], ['nosource', 'Ohne Quelle']]],
  ['source', 'Quelle', () => [['', 'Alle Quellen']].concat(LIVE.sources.map((s) => [s.id, s.name]))],
  ['group', 'Gruppe', () => [['', 'Alle Gruppen']].concat(LIVE.groups.map((g) => [g.id, g.name]))],
  ['country', 'Land', () => [['', 'Alle Länder']].concat(Object.entries(LIVE.countries))],
  ['lang', 'Sprache', () => [['', 'Alle Sprachen']].concat(Object.entries(LIVE.languages))],
  ['category', 'Kategorie', () => [['', 'Alle Kategorien']].concat(LIVE.categories.map((c) => [c, c]))],
  ['epg', 'EPG', () => [['', 'Alle'], ['ok', 'zugeordnet'], ['unsure', 'unsicher'], ['missing', 'fehlt']]],
  ['rec', 'Empfang', () => [['', 'Alle'], ['default', 'Standard'], ['permanent', 'Permanent'], ['auto', 'Automatisch'], ['ondemand', 'Bei Bedarf'], ['schedule', 'Nach Zeitplan']]],
];
function aaChFiltered() {
  const f = aaCF(); const q = f.q.trim().toLowerCase();
  let rows = aaChAll().filter((c) => {
    const st = SIM.chState(c);
    if (f.state === 'active' && !(c.active && st !== 'gone')) return false;
    if (f.state === 'inactive' && c.active) return false;
    if (f.state === 'new' && !c.isNew) return false;
    if (f.state === 'disturbed' && st !== 'disturbed') return false;
    if (f.state === 'gone' && st !== 'gone') return false;
    if (f.state === 'nosource' && c.sources.length) return false;
    if (f.source && !c.sources.some((s) => s.source === f.source)) return false;
    if (f.group && !c.groups.includes(f.group)) return false;
    if (f.country && c.country !== f.country) return false;
    if (f.lang && !c.langs.includes(f.lang)) return false;
    if (f.category && c.category !== f.category) return false;
    if (f.epg && aaMapKind(c) !== f.epg) return false;
    if (f.rec && c.reception !== f.rec) return false;
    if (q && !(c.name.toLowerCase().includes(q) || String(c.no) === q || (c.nameInSource || '').toLowerCase().includes(q) || c.sources.some((s) => s.name.toLowerCase().includes(q)))) return false;
    return true;
  });
  const rank = (c) => ({ ok: 0, disturbed: 1, gone: 2 }[SIM.chState(c)]);
  rows.sort((a, b) => f.dir * (f.sort === 'name' ? a.name.localeCompare(b.name, 'de') : f.sort === 'state' ? rank(a) - rank(b) || a.name.localeCompare(b.name, 'de') : (a.no == null) - (b.no == null) || (a.no || 0) - (b.no || 0) || a.name.localeCompare(b.name, 'de')) || 0);
  return rows;
}
const aaSelCount = () => (AA.selAll ? admCounts().imported : Object.keys(AA.sel).length);
const aaHasFilter = () => { const f = aaCF(); return !!(f.q || AA_FACETS.some(([k]) => f[k])); };
function aaChTable(rows) {
  const f = aaCF(); const all = rows.length > 0 && rows.every((c) => AA.sel[c.id] || AA.selAll);
  const th = (k, l) => `<th aria-sort="${f.sort === k ? (f.dir > 0 ? 'ascending' : 'descending') : 'none'}"><button type="button" class="aa-sortb" data-act="aaSort" data-k="${k}" data-fid="la05:sort:${k}">${l}${f.sort === k ? icon(f.dir > 0 ? 'arrowUp' : 'arrowDown', 12) : ''}</button></th>`;
  const body = rows.map((c) => {
    const src = c.sources[0]; const act = c.sources.find((s) => s.state !== 'fail'); const sel = AA.sel[c.id] || AA.selAll; const mk = aaMapKind(c);
    const srcNote = src && src.name.replace(/ HD( CH)?$/, '').replace(/\s/g, '').toLowerCase() !== c.name.replace(/\s/g, '').toLowerCase() ? `<small>Quelle: ${esc(src.name)}</small>` : '';
    const chips = `${c.isNew ? '<span class="chip blue">Neu</span> ' : ''}${!c.active && c.state !== 'gone' ? '<span class="chip info">Inaktiv</span> ' : ''}${!c.sources.length ? '<span class="chip warn">Ohne Quelle</span>' : ''}`;
    return `<tr class="aa-row${sel ? ' sel' : ''}" data-act="aaChOpen" data-id="${c.id}"><td class="aa-cb" data-act="nop">${aaCb(sel, 'aaSel', { id: c.id }, `${c.name} auswählen`, 'la05:cb:' + c.id)}</td>
      <td data-l="Nr." class="aa-no">${aaNum(c)}</td>
      <td class="main" data-l="Sender"><span class="aa-chn">${LU.logo(c)}<span><button type="button" class="aa-link" data-act="aaChOpen" data-id="${c.id}" data-fid="la05:open:${c.id}">${esc(c.name)}</button> ${chips}<small>${c.groups.map((g) => esc(aaGroupName(g))).join(', ') || 'keine Gruppe'}</small>${srcNote}</span></span></td>
      <td data-l="Land · Sprache">${esc(c.country)} · ${c.langs.map((l) => LIVE.languages[l]).join(', ')}</td><td data-l="Kategorie">${esc(c.category)}</td>
      <td data-l="Quellen">${c.sources.length ? `${c.sources.length}${act ? ` (aktiv: ${esc(aaSrcName(act.source))})` : ' (alle gestört)'}` : '–'}</td>
      <td data-l="EPG"><span class="chip ${mk === 'ok' ? 'ok' : mk === 'unsure' ? 'warn' : mk === 'without' ? 'info' : 'bad'}">${AA_MAPL[mk]}</span></td>
      <td data-l="Empfang">${esc(AA_RECL[c.reception] || '')}${c.reception === 'schedule' && c.schedule ? ` «${esc(c.schedule)}»` : ''}</td>
      <td data-l="Zustand" class="aa-st">${aaChState(c)}</td>
      <td class="aa-cb" data-act="nop" data-l="Aktiv">${aaSw(c.active && SIM.chState(c) !== 'gone', 'aaChActive', { id: c.id }, `${c.name} aktiv`, 'la05:act:' + c.id)}</td></tr>`;
  }).join('');
  return `<div class="tblw aa-ch"><table class="tbl"><thead><tr><th class="aa-cb"><input type="checkbox" ${all ? 'checked' : ''} data-act="aaSelAll" data-fid="la05:all" aria-label="Alle Sender dieser Liste auswählen"></th>${th('no', 'Nr.')}${th('name', 'Sender')}<th>Land · Sprache</th><th>Kategorie</th><th>Quellen</th><th>EPG</th><th>Empfang</th>${th('state', 'Zustand')}<th>Aktiv</th></tr></thead><tbody>${body}</tbody></table></div>`;
}
function aaBulkBar(rows) {
  const n = aaSelCount(); if (!n) return '';
  const b = (op, l, o) => `<button type="button" class="btn secondary sm" data-act="aaBulk" data-op="${op}" data-fid="la05:bk:${op}"${o && o.dis ? ' disabled' : ''}>${l}</button>`;
  const full = rows.length && rows.every((c) => AA.sel[c.id]) && !AA.selAll && !aaHasFilter() && rows.length < admCounts().imported;
  return `<div class="savebar aa-bulk" role="region" aria-label="Massenbearbeitung"><b class="grow">${fmt.num(n)} ausgewählt${full ? ` · <button type="button" class="lnk" data-act="aaSelEvery" data-fid="la05:every">Alle ${fmt.num(admCounts().imported)} Treffer auswählen</button>` : ''}${AA.selAll ? ' · <button type="button" class="lnk" data-act="aaSelClear" data-fid="la05:onlypage">Nur diese Liste</button>' : ''}</b>
    ${b('on', 'Aktivieren')}${b('off', 'Deaktivieren')}${b('gadd', 'Zu Gruppe hinzufügen …')}${b('grem', 'Aus Gruppe entfernen …')}${b('country', 'Land …')}${b('lang', 'Sprache …')}${b('category', 'Kategorie …')}${b('rec', 'Empfang …')}${b('numbers', 'Nummern vergeben …')}${b('merge', 'Zusammenführen', { dis: n < 2 })}
    <button type="button" class="btn ghost sm" data-act="aaSelClear" data-fid="la05:clear">Auswahl aufheben</button></div>`;
}
function aaChannelsTab(q) {
  const f = aaCF(); const c = admCounts();
  return aaGate(aaEmptyBox('Noch keine Sender', 'Füge zuerst eine Quelle hinzu. Aus ihr entstehen die Sender.', `<a class="btn primary" href="#/admin/live/sources/new" data-fid="la05:addsrc">${icon('plus', 16)}Quelle hinzufügen</a>`), () => {
    const rows = aaChFiltered(); const nf = AA_FACETS.filter(([k]) => f[k]).length;
    const facets = AA_FACETS.map(([k, l, opts]) => `<label class="fld fsel"><span class="lbl">${l}</span>${selectEl('ui.draft.cf.' + k, opts().map(([v, t]) => ({ v, t })), { label: l })}</label>`).join('');
    const chips = AA_FACETS.filter(([k]) => f[k]).map(([k, l, opts]) => `<button type="button" class="chip info aa-fchip" data-act="aaFClear" data-k="${k}" data-fid="la05:fc:${k}" aria-label="Filter ${l} entfernen">${l}: ${esc((opts().find((o) => o[0] === f[k]) || [0, f[k]])[1])}${icon('x', 12)}</button>`).join('');
    return `<div class="aa-tools"><div class="fsearch"><span class="sic">${icon('search', 18)}</span><label class="vh" for="la05-q">Sender suchen</label><input class="inp" id="la05-q" type="search" data-bind="ui.draft.cf.q" data-live data-fid="la05:q" value="${esc(f.q)}" placeholder="Name, Nummer oder Name in der Quelle" autocomplete="off"></div>
      <button type="button" class="btn secondary aa-fbtn" data-act="open" data-id="aa:facets" aria-expanded="${isOpen('aa:facets')}" data-fid="la05:fbtn">${icon('filter', 16)}Filter${nf ? ` (${nf})` : ''}</button></div>
      <div class="aa-facets${isOpen('aa:facets') ? ' on' : ''}">${facets}</div>
      ${chips || f.q ? `<div class="afilters" aria-label="Aktive Filter">${chips}${chips ? '<button type="button" class="btn quiet sm" data-act="aaFClearAll" data-fid="la05:fall">Alle Filter entfernen</button>' : ''}</div>` : ''}
      ${f.state === 'new' ? LIVE.sources.filter((s) => !s.offerNew && aaNewOf(s.id).length).map((s) => `<p class="muted aa-small">«Neue Sender sofort anbieten» ist ab Werk an. In der Quelle <a class="lnk" href="#/admin/live/sources/${s.id}" data-fid="la05:offer:${s.id}">«${esc(s.name)}»</a> ist es auf aus gestellt; deshalb ${aaNewOf(s.id).length === 1 ? 'ist' : 'sind'} ${aaNewOf(s.id).map((c) => esc(c.name)).join(', ')} inaktiv.</p>`).join('') : ''}
      ${rows.length ? aaChTable(rows) : aaEmptyBox('Kein Sender passt', 'Ändere oder entferne die Filter.', '<button type="button" class="btn secondary" data-act="aaFClearAll" data-fid="la05:fall2">Filter zurücksetzen</button>')}
      <p class="muted aa-small">${rows.length} von ${fmt.num(c.active)} aktiven Sendern gezeigt (Beispielausschnitt der Senderliste). Die Reihenfolge richtet sich nach der Kanalnummer; ohne Nummer folgen die Sender alphabetisch.</p>${aaBulkBar(rows)}`;
  }, { rows: 8, h: 48 });
}
ACT.aaSort = (d) => { const f = aaCF(); if (f.sort === d.k) f.dir = -f.dir; else { f.sort = d.k; f.dir = 1; } render(); };
ACT.aaFClear = (d) => { aaCF()[d.k] = ''; render(); };
ACT.aaFClearAll = () => { Object.assign(aaCF(), { q: '', state: '', source: '', group: '', country: '', lang: '', category: '', epg: '', rec: '' }); render(); };
ACT.aaSel = (d) => { if (AA.selAll) { AA.selAll = false; aaChFiltered().forEach((c) => { AA.sel[c.id] = true; }); } if (AA.sel[d.id]) delete AA.sel[d.id]; else AA.sel[d.id] = true; render(); };
ACT.aaSelAll = () => { const rows = aaChFiltered(); if (rows.every((c) => AA.sel[c.id])) { AA.sel = {}; AA.selAll = false; } else rows.forEach((c) => { AA.sel[c.id] = true; }); render(); };
ACT.aaSelEvery = () => { AA.selAll = true; render(); };
ACT.aaSelClear = () => { AA.sel = {}; AA.selAll = false; render(); };
ACT.aaChActive = (d) => {
  const c = SIM.channel(d.id); if (!c) return;
  if (!c.sources.length || c.state === 'gone') { toast(`«${esc(c.name)}» hat keine Quelle und kann nicht aktiv sein.`, 'warn'); return; }
  if (c.active) { const x = aaRunningOn(c.id); if (x.v + x.r) { openDialog('aaOffAsk', { id: c.id }); return; } }
  c.active = !c.active; if (c.active) c.isNew = false; toastUndo(`«${esc(c.name)}» ist ${c.active ? 'aktiv' : 'inaktiv'}.`, 'aaChActive', { id: c.id }); render();
};
route('/admin/live/channels', (pm, q) => {
  const fresh = parseHash().path !== lastPath; const tab = ['channels', 'groups', 'duplicates'].includes(q.tab) ? q.tab : 'channels';
  if (fresh) { ui.draft.cf = null; AA.sel = {}; AA.selAll = false; const f = aaCF(); ['state', 'source', 'group', 'country', 'lang', 'category', 'epg', 'rec', 'q'].forEach((k) => { if (q[k]) f[k] = q[k]; }); if (q.filter === 'missing') f.epg = 'missing'; }
  if (aaSources().length) AA.chVisited = true;
  const c = admCounts(); let html; const tabs = subTabs('#/admin/live/channels', [['channels', 'Sender', null], ['groups', 'Gruppen', aaEmpty() ? 0 : LIVE.groups.length], ['duplicates', `Duplikate`, aaDupCount()]], tab);
  if (tab === 'groups') html = aaGroupsTab(); else if (tab === 'duplicates') html = aaDupTab(); else html = aaChannelsTab(q);
  return aaView({ area: 'livetv', page: 'channels', title: tab === 'groups' ? 'Sendergruppen' : tab === 'duplicates' ? 'Duplikate' : `Sender · ${fmt.num(c.active)} aktiv von ${fmt.num(c.imported)}`, h1: tab === 'channels' ? `Sender <span class="lcount">${fmt.num(c.active)} aktiv von ${fmt.num(c.imported)}</span>` : undefined,
    intro: tab === 'groups' ? 'Gruppen ordnen Sender für die Zuschauer. Sie sind keine Rechte: Wer Live-TV darf, sieht alle aktiven Sender.' : tab === 'duplicates' ? 'Slouchcat erkennt Sender, die vermutlich dasselbe zeigen. Du entscheidest, was zusammengehört.' : 'Alle Sender aller Quellen. Ein Klick auf eine Zeile öffnet den Sender zum Bearbeiten.',
    actions: tab === 'groups' ? `<button type="button" class="btn primary" data-act="aaGroupOpen" data-id="" data-fid="la08:add">${icon('plus', 16)}Gruppe hinzufügen</button>` : `<a class="btn secondary" href="#/admin/live/sources/new" data-fid="la05:addsingle">${icon('plus', 16)}Sender hinzufügen</a><button type="button" class="btn secondary" data-act="aaBulk" data-op="numbers" data-fid="la05:nums">Nummern vergeben …</button>`,
    html: (aaMode() === 'normal' || aaMode() === 'empty' ? '' : '') + tabs + html,
    after: () => { if (q.edit && AA2.opened !== location.hash && !ui.dialog && aaMode() === 'normal') { AA2.opened = location.hash; ACT.aaChOpen({ id: q.edit, open: q.open }); } } });
});

/* ---------- Massenbearbeitung ---------- */
const AA_BULK = { on: 'Aktivieren', off: 'Deaktivieren', gadd: 'Zu Gruppe hinzufügen', grem: 'Aus Gruppe entfernen', country: 'Land setzen', lang: 'Sprache setzen', category: 'Kategorie setzen', rec: 'Empfang setzen', numbers: 'Nummern vergeben' };
const aaTargets = () => { const rows = aaChFiltered(); return AA.selAll ? rows : rows.filter((c) => AA.sel[c.id]); };
ACT.aaBulk = (d) => {
  if (d.op === 'merge') { const ids = aaTargets().map((c) => c.id); if (ids.length < 2) { toast('Wähle mindestens zwei Sender zum Zusammenführen.', 'warn'); return; } ACT.aaMergeOpen({ ids: ids.join(',') }); return; }
  const t = aaTargets(); ui.draft.bk = { op: d.op, val: d.op === 'country' ? 'CH' : d.op === 'lang' ? 'de' : d.op === 'category' ? LIVE.categories[0] : d.op === 'rec' ? 'permanent' : LIVE.groups[0].id, sched: LIVE.settings.schedules[0].name, start: 101, step: 1, mode: 'over', n: aaSelCount() || t.length, whole: !aaSelCount() };
  openDialog('aaBulk', {});
};
function aaNumberPlan(b) {
  const ts = aaTargets(); const ids = new Set(ts.map((c) => c.id)); const occ = new Map(); aaChAll().forEach((c) => { if (c.no != null && (b.mode === 'fill' ? true : !ids.has(c.id))) occ.set(c.no, c); });
  const errs = []; const plan = []; let n = Number(b.start); const st = Number(b.step);
  if (!(n >= 1) || !(st >= 1)) { errs.push('Start und Schritt müssen mindestens 1 sein.'); return { plan, errs }; }
  ts.forEach((c) => { if (b.mode === 'fill' && c.no != null) return; while (b.mode === 'fill' && occ.has(n) && false) n += st; const o = occ.get(n); if (o && o.id !== c.id) errs.push(`Nummer ${n} hat schon ${o.name}.`); plan.push([c, n]); n += st; });
  return { plan, errs };
}
DIALOGS.aaBulk = () => {
  const b = ui.draft.bk; const t = aaTargets(); const opt = (arr) => arr.map(([v, x]) => ({ v, t: x }));
  let field = '';
  if (b.op === 'gadd' || b.op === 'grem') field = aaF('bk-v', 'Gruppe', selectEl('ui.draft.bk.val', LIVE.groups.map((g) => ({ v: g.id, t: g.name })), { label: 'Gruppe' }).replace('<select ', '<select id="bk-v" data-autofocus '));
  if (b.op === 'country') field = aaF('bk-v', 'Land', selectEl('ui.draft.bk.val', opt(Object.entries(LIVE.countries)), { label: 'Land' }).replace('<select ', '<select id="bk-v" data-autofocus '));
  if (b.op === 'lang') field = aaF('bk-v', 'Sprache', selectEl('ui.draft.bk.val', opt(Object.entries(LIVE.languages)), { label: 'Sprache' }).replace('<select ', '<select id="bk-v" data-autofocus '));
  if (b.op === 'category') field = aaF('bk-v', 'Kategorie', selectEl('ui.draft.bk.val', LIVE.categories.map((x) => ({ v: x, t: x })), { label: 'Kategorie' }).replace('<select ', '<select id="bk-v" data-autofocus '));
  if (b.op === 'rec') field = aaF('bk-v', 'Empfang', selectEl('ui.draft.bk.val', opt(Object.entries(AA_RECL)), { label: 'Empfang' }).replace('<select ', '<select id="bk-v" data-autofocus ')) + (b.val === 'schedule' ? aaF('bk-s', 'Zeitplan', selectEl('ui.draft.bk.sched', LIVE.settings.schedules.map((s) => ({ v: s.name, t: s.name })), { label: 'Zeitplan' })) : '') + (['permanent', 'schedule'].includes(b.val) ? '<p class="muted aa-small">Jeder permanent oder nach Zeitplan empfangene Sender belegt dauerhaft eine Verbindung zur Quelle, auch ohne Zuschauer.</p>' : '');
  let err = ''; let ex = '';
  if (b.op === 'numbers') {
    const { plan, errs } = aaNumberPlan(b);
    field = `<div class="aa-two">${aaF('bk-st', 'Nummern ab', aaI('ui.draft.bk.start', 'bk-st', { num: true, auto: true, live: true }))}${aaF('bk-sp', 'Schritt', aaI('ui.draft.bk.step', 'bk-sp', { num: true, live: true }))}</div><div class="fld"><span class="lbl">Bestehende Nummern</span>${aaRadio('ui.draft.bk.mode', 'bkm', 'over', 'überschreiben', 'bk:over')}${aaRadio('ui.draft.bk.mode', 'bkm', 'fill', 'nur leere füllen', 'bk:fill')}</div>`;
    ex = plan.length ? `<p class="muted aa-small">In der aktuellen Sortierung: ${plan.slice(0, 3).map(([c, n]) => `${esc(c.name)} → ${n}`).join(', ')}${plan.length > 3 ? ' …' : ''}</p>` : '';
    err = errs.length ? `<div class="notice bad" role="alert"><span class="ic">${icon('alert')}</span><div><b>Doppelte Nummern verhindert das Formular.</b><ul class="aa-list">${errs.slice(0, 4).map((x) => `<li>${esc(x)}</li>`).join('')}</ul></div></div>` : '';
  }
  if (b.op === 'off') { const run = t.reduce((a, c) => { const x = aaRunningOn(c.id); return { v: a.v + x.v, r: a.r + x.r }; }, { v: 0, r: 0 }); ex = `<p class="muted">Deaktivieren macht die Sender sofort für alles Neue unsichtbar und beendet nichts Laufendes.${run.v + run.r ? ` ${aaRunText(run)} ${run.v + run.r === 1 ? 'läuft' : 'laufen'} weiter bis zu ihrem Ende. Sofort beenden kannst du im einzelnen Sender («Sofort beenden …»).` : ''}</p>`; }
  const n = aaSelCount() || t.length; const bad = b.op === 'numbers' && aaNumberPlan(b).errs.length;
  return { title: AA_BULK[b.op], body: `<div class="stack">${field}${ex}${err}<p class="lead">Gilt für ${aaPl(n, 'Sender', 'Sender')}${aaSelCount() ? '' : ' (alle in der Liste)'}.</p></div>`, actions: `<button type="button" class="btn secondary" data-act="close" data-fid="bk:x">Abbrechen</button><button type="button" class="btn primary" data-act="aaBulkOk" data-fid="bk:ok" ${bad ? 'aria-disabled="true"' : ''}>${b.op === 'numbers' ? 'Nummern vergeben' : 'Anwenden'}</button>` };
};
ACT.aaBulkOk = () => {
  const b = ui.draft.bk; const t = aaTargets(); const n = aaSelCount() || t.length; let msg = '';
  if (b.op === 'numbers') { const { plan, errs } = aaNumberPlan(b); if (errs.length) return; plan.forEach(([c, no]) => { c.no = no; }); msg = `${plan.length} Nummern vergeben (ab ${b.start}).`; }
  else if (b.op === 'on' || b.op === 'off') { let k = 0; t.forEach((c) => { if (c.sources.length && c.state !== 'gone') { c.active = b.op === 'on'; if (b.op === 'on') c.isNew = false; k++; } }); msg = `${aaPl(n, 'Sender', 'Sender')} ${b.op === 'on' ? 'aktiviert' : 'deaktiviert'}${k < t.length ? ` (${t.length - k} ohne Quelle übersprungen)` : ''}.`; }
  else if (b.op === 'gadd') { t.forEach((c) => { if (!c.groups.includes(b.val)) c.groups.push(b.val); }); msg = `${aaPl(n, 'Sender', 'Sender')} zu «${esc(aaGroupName(b.val))}» hinzugefügt.`; }
  else if (b.op === 'grem') { t.forEach((c) => { c.groups = c.groups.filter((g) => g !== b.val); }); msg = `${aaPl(n, 'Sender', 'Sender')} aus «${esc(aaGroupName(b.val))}» entfernt.`; }
  else if (b.op === 'country') { t.forEach((c) => { c.country = b.val; }); msg = `Land bei ${aaPl(n, 'Sender', 'Sendern')} gesetzt.`; }
  else if (b.op === 'lang') { t.forEach((c) => { c.langs = [b.val]; }); msg = `Sprache bei ${aaPl(n, 'Sender', 'Sendern')} gesetzt.`; }
  else if (b.op === 'category') { t.forEach((c) => { c.category = b.val; }); msg = `Kategorie bei ${aaPl(n, 'Sender', 'Sendern')} gesetzt.`; }
  else if (b.op === 'rec') { t.forEach((c) => { c.reception = b.val; if (b.val === 'schedule') c.schedule = b.sched; }); msg = `Empfang bei ${aaPl(n, 'Sender', 'Sendern')} gesetzt.`; }
  closeDialog(); toast(msg, 'ok'); render();
};

/* ======================================================================================
   LA-06 Sender bearbeiten (Dialog)
   ====================================================================================== */
ACT.aaChOpen = (d) => {
  const c = SIM.channel(d.id); if (!c) return;
  ['general', 'logo', 'groups', 'epg', 'sources', 'reception', 'tech', 'avail', 'use'].forEach((k) => { ui.open['lae:' + k] = false; });
  if (d.open) ui.open['lae:' + d.open] = true;
  const m = aaMapOf(c); const ce = Object.assign(aaClone(c), { errors: {}, logoMode: 'src', logoUrl: '', logoFile: '', schedule: c.schedule || LIVE.settings.schedules[0].name, epgSel: m && m[1] ? m[1] + '|' + m[2] : m && m[3] === 'without' ? 'without' : '', addOpen: false, addVal: '', noAudio: !!c.noAudio, noVideo: !!c.noVideo });
  ui.draft.ce = ce; openDialog('aaChEd', {});
};
const aaCeSum = (ce) => {
  const rec = AA_RECL[ce.reception] || ''; const first = ce.sources.find((s) => s.state !== 'fail');
  const n = (LIVE.diagnostics.perChannel.find((x) => x[0] === ce.id) || [0, 0])[1];
  return {
    general: `${esc(ce.name)} · Nr. ${aaNum(ce)} · ${esc(ce.country)} · ${ce.langs.map((l) => LIVE.languages[l]).join(', ')} · ${esc(ce.category)}`,
    logo: ce.logoMode === 'url' ? 'eigene Adresse' : ce.logoMode === 'file' ? 'eigene Datei' : 'aus der Quelle',
    groups: ce.groups.map((g) => esc(aaGroupName(g))).join(', ') || 'keine Gruppe',
    epg: ce.epgSel === 'without' ? 'bewusst ohne Programmdaten' : ce.epgSel ? esc((aaEpgOptions().find((o) => o.v === ce.epgSel) || { t: ce.epgSel }).t) : 'nicht zugeordnet',
    sources: `${aaPl(ce.sources.length, 'Quelle', 'Quellen')}${first ? `, aktiv: ${esc(aaSrcName(first.source))}` : ''}`,
    reception: `${rec}${ce.reception === 'schedule' ? ` «${esc(ce.schedule)}»` : ''}`,
    tech: `${esc(ce.video.split('·').slice(0, 2).join('·').trim())} · ${fmt.dec(ce.bitrate, 1, 1)} Mbit/s`,
    avail: SIM.chState(ce) === 'ok' ? `Verfügbar, ${aaPl(n, 'Ausfall', 'Ausfälle')} in 7 Tagen` : `${SIM.chState(ce) === 'gone' ? 'Verschwunden' : 'Gestört'}, ${aaPl(n, 'Ausfall', 'Ausfälle')} in 7 Tagen`,
    use: `${aaPl(ce.favCount, 'Favorit', 'Favoriten')} · ${aaPl(aaPlannedOn(ce.id), 'geplante Aufnahme', 'geplante Aufnahmen')}`,
  };
};
function aaCeSources(ce) {
  const rows = ce.sources.map((s, i) => `<li class="aa-rank"><span class="aa-rk">${i + 1}</span><span class="aa-rt"><b>${esc(s.name)}</b><small>${esc(aaSrcName(s.source))} · ${i === 0 ? 'Hauptquelle' : 'Ersatzquelle ' + (i + 1)}</small><small class="mono">${esc(aaSHP(s.url))}</small><small>${aaDot(s.state === 'fail' ? 'bad' : 'ok')} ${s.state === 'fail' ? 'nicht erreichbar' : 'in Ordnung'} · zuletzt geprüft ${fmt.ago(SIM.now() - (3 + i * 4))}</small></span>
    <span class="aa-rb"><button type="button" class="btn quiet sm" data-act="aaCeSrc" data-op="test" data-i="${i}" data-fid="lae:t${i}">Testen</button><button type="button" class="btn quiet icon sm" data-act="aaCeSrc" data-op="up" data-i="${i}" ${i === 0 ? 'disabled' : ''} aria-label="${esc(s.name)} nach oben" data-fid="lae:u${i}">${icon('arrowUp', 16)}</button><button type="button" class="btn quiet icon sm" data-act="aaCeSrc" data-op="down" data-i="${i}" ${i === ce.sources.length - 1 ? 'disabled' : ''} aria-label="${esc(s.name)} nach unten" data-fid="lae:d${i}">${icon('arrowDown', 16)}</button>${ce.sources.length > 1 ? `<button type="button" class="btn quiet sm" data-act="aaCeSrc" data-op="split" data-i="${i}" data-fid="lae:s${i}">Als eigenen Sender abtrennen</button>` : ''}</span></li>`).join('');
  const singles = LIVE.sources.filter((s) => s.kind === 'single').flatMap((s) => s.streams.map((u) => ({ v: s.id + '|' + u, t: `${s.name}: ${aaSHP(u)}` }))).filter((o) => !ce.sources.some((x) => x.source === o.v.split('|')[0] && x.url === o.v.split('|')[1]));
  return `<p class="muted">Fällt die erste Quelle aus, wechselt Slouchcat zur nächsten.</p><ol class="aa-ranks">${rows || '<li class="muted">Dieser Sender hat keine Quelle.</li>'}</ol>
    ${ce.addOpen ? `<div class="aa-add">${singles.length ? aaF('lae-add', 'Einzelstrom als Ersatzquelle', selectEl('ui.draft.ce.addVal', [{ v: '', t: 'Bitte wählen' }].concat(singles), { label: 'Einzelstrom' }).replace('<select ', '<select id="lae-add" ')) : '<p class="muted">Es gibt keine freien Einzelströme.</p>'}
      <div class="row-gap"><button type="button" class="btn primary sm" data-act="aaCeAdd" data-fid="lae:addok">Hinzufügen</button><button type="button" class="btn ghost sm" data-act="aaCeAddMerge" data-fid="lae:addmerge">Anderen Sender zusammenführen …</button><button type="button" class="btn ghost sm" data-act="aaCeAddToggle" data-fid="lae:addx">Abbrechen</button></div></div>` : `<div><button type="button" class="btn secondary sm" data-act="aaCeAddToggle" data-fid="lae:add">${icon('plus', 16)}Quelle hinzufügen</button></div>`}`;
}
DIALOGS.aaChEd = () => {
  const ce = ui.draft.ce; const c = SIM.channel(ce.id); const S = aaCeSum(ce); const e = ce.errors; const st = SIM.chState(c);
  const B = (k, t, ic, inner) => band('lae:' + k, t, S[k], inner, { icon: ic, open: (k === 'general' && (e.name || e.no)) || undefined });
  const langs = Object.entries(LIVE.languages).map(([k, v]) => `<label class="check"><input type="checkbox" ${ce.langs.includes(k) ? 'checked' : ''} data-act="aaCeLang" data-v="${k}" data-fid="lae:lang:${k}">${v}</label>`).join('');
  const gen = `<div class="stack">${aaF('lae-name', 'Name', aaI('ui.draft.ce.name', 'lae-name', { err: e.name, auto: true }), { err: e.name, hint: c.nameInSource && c.nameInSource !== ce.name ? `<button type="button" class="lnk" data-act="aaCeSrcName" data-fid="lae:srcname">Zurück zum Namen der Quelle («${esc(c.nameInSource)}»)</button>` : '' })}
    <div class="aa-two">${aaF('lae-no', 'Nummer', aaI('ui.draft.ce.no', 'lae-no', { num: true, err: e.no }), { err: e.no, hint: 'Leer: ohne Nummer.' })}${aaF('lae-co', 'Land', selectEl('ui.draft.ce.country', Object.entries(LIVE.countries).map(([v, t]) => ({ v, t })), { label: 'Land' }).replace('<select ', '<select id="lae-co" '))}</div>
    <div class="fld"><span class="lbl">Sprache</span><div class="row-gap">${langs}</div></div>${aaF('lae-cat', 'Kategorie', selectEl('ui.draft.ce.category', LIVE.categories.map((x) => ({ v: x, t: x })), { label: 'Kategorie' }).replace('<select ', '<select id="lae-cat" '))}</div>`;
  const logo = `<div class="stack">${aaRadio('ui.draft.ce.logoMode', 'laelm', 'src', 'aus der Quelle', 'lae:lm-src')}${aaRadio('ui.draft.ce.logoMode', 'laelm', 'url', 'eigene Adresse', 'lae:lm-url')}${ce.logoMode === 'url' ? aaI('ui.draft.ce.logoUrl', 'lae-lurl', { ph: 'https://logos.example.net/sender.png' }) : ''}${aaRadio('ui.draft.ce.logoMode', 'laelm', 'file', 'Datei hochladen', 'lae:lm-file')}${ce.logoMode === 'file' ? `<div class="row-gap"><button type="button" class="btn secondary sm" data-act="aaCeLogoFile" data-fid="lae:lfile">${icon('upload', 16)}Datei wählen</button><span class="mono">${esc(ce.logoFile || 'keine Datei')}</span></div>` : ''}
    <div class="aa-prev"><div class="aa-pd">${LU.logo(c)}<small>dunkler Grund</small></div><div class="aa-pl">${LU.logo(c)}<small>heller Grund</small></div></div>
    <div><button type="button" class="btn quiet sm" data-act="aaCeLogoReset" data-fid="lae:lreset">Zurücksetzen</button></div></div>`;
  const grp = `<div class="aa-cbs">${LIVE.groups.map((g) => `<label class="check"><input type="checkbox" ${ce.groups.includes(g.id) ? 'checked' : ''} data-act="aaCeGroup" data-v="${g.id}" data-fid="lae:g:${g.id}"><span>${esc(g.name)}<small>${g.origin === 'own' ? 'eigene' : 'aus Quelle'}</small></span></label>`).join('')}</div>`;
  const next = ce.epgSel && ce.epgSel !== 'without' && !SIM.flags().noEpg ? SIM.programsOf(ce.id, SIM.now(), SIM.now() + 8 * 60).slice(0, 3) : [];
  const epg = `<div class="stack">${aaF('lae-epg', 'Zuordnung', selectEl('ui.draft.ce.epgSel', aaEpgOptions(), { label: 'Programmdaten-Zuordnung' }).replace('<select ', '<select id="lae-epg" '), { hint: ce.epgSel === 'without' ? 'Dieser Sender hat bewusst keine Programmdaten.' : 'Eine Zuordnung von Hand übersteht jede Aktualisierung.' })}
    ${ce.epgSel && ce.epgSel !== 'without' ? `<div class="fld"><span class="lbl">Die nächsten drei Sendungen</span>${next.length ? `<ul class="aa-next">${next.map((p) => `<li><b>${fmt.time(p.start)}</b> ${esc(p.title)}</li>`).join('')}</ul>` : '<p class="muted">Keine Programmdaten in dieser Zeit.</p>'}</div>` : ''}</div>`;
  const rec = `<div class="stack">${Object.entries(AA_RECL).map(([k, t]) => aaRadio('ui.draft.ce.reception', 'laerc', k, k === 'default' ? 'Standard des Servers (bei Bedarf)' : t, 'lae:rc:' + k)).join('')}${ce.reception === 'schedule' ? aaF('lae-sch', 'Zeitplan', selectEl('ui.draft.ce.schedule', LIVE.settings.schedules.map((s) => ({ v: s.name, t: `${s.name} (${s.times})` })), { label: 'Zeitplan' }).replace('<select ', '<select id="lae-sch" ')) : ''}
    <p class="muted aa-small">Ob dieser Sender vorgeladen wird, regelt die Einstellung «Vorladen für Sender dieser Quelle» der Quelle (${ce.sources.length ? ce.sources.map((s) => { const q = LIVE.sources.find((x) => x.id === s.source); return q ? `${esc(q.name)}: ${q.moreConn ? 'an' : 'aus'}` : ''; }).filter(Boolean).filter((v, i, a) => a.indexOf(v) === i).join(', ') : 'keine Quelle'}). Einen Schalter je Sender gibt es nicht. <a class="lnk" href="#/admin/live/reception?tab=strategy" data-act="close" data-fid="lae:rcp">Zum Empfang</a></p>
    ${['permanent', 'schedule'].includes(ce.reception) ? notice('', 'Dauerhafter Empfang', 'Jeder dauerhaft, vorgeladen oder nach Zeitplan empfangene Sender belegt eine Verbindung zur Quelle, auch ohne Zuschauer.') : ''}</div>`;
  const tech = `<dl class="kv"><dt>Bild</dt><dd>${esc(ce.video)}</dd><dt>Ton</dt><dd>${ce.audio.map(esc).join('<br>')}</dd><dt>Untertitel</dt><dd>${ce.subs.length ? ce.subs.map(esc).join(', ') : 'keine'}</dd><dt>Bitrate</dt><dd>${fmt.dec(ce.bitrate, 1, 1)} Mbit/s</dd><dt>Gemessen</dt><dd>${aaWhen(SIM.now() - 6)}</dd></dl>
    <div class="srow"><div class="st"><b>Sender hat keinen Ton</b><span>Die Überwachung meldet fehlenden Ton dann nicht als Fehler.</span></div><div class="sc">${sw('ui.draft.ce.noAudio', 'Sender hat keinen Ton')}</div></div>
    <div class="srow"><div class="st"><b>Sender hat kein Bild</b><span>Reiner Tonsender.</span></div><div class="sc">${sw('ui.draft.ce.noVideo', 'Sender hat kein Bild')}</div></div>`;
  const log = LIVE.diagnostics.log.filter((l) => l[1] === ce.id).sort((a, b) => b[0] - a[0]);
  const avail = `${log.length ? `<ul class="feed">${log.slice(0, 4).map(([t, , k, txt]) => `<li><time>${fmt.day(t)} ${fmt.time(t)}</time><span>${esc(txt)}</span></li>`).join('')}</ul>` : '<p class="muted">Keine Ausfälle in den letzten sieben Tagen.</p>'}<div><a class="btn secondary sm" href="#/admin/live/diagnostics?channel=${ce.id}" data-act="close" data-fid="lae:diag">Diagnose öffnen</a></div>`;
  const use = `<p>${aaPl(ce.favCount, 'Favorit', 'Favoriten')} · ${aaPl(aaPlannedOn(ce.id), 'geplante Aufnahme', 'geplante Aufnahmen')}</p><p class="muted aa-small">Nur Zahlen. Welche Profile den Sender merken oder aufnehmen, siehst du nicht.</p>`;
  const del = !ce.sources.length || st === 'gone';
  return { title: `<span class="aa-dh">${LU.logo(c)}<span>${aaNum(ce) !== '–' ? `<small>Nr. ${aaNum(ce)}</small>` : ''}${esc(ce.name)}</span></span>`, wide: true,
    body: `<div class="aa-chead"><div class="row-gap"><span class="aa-st2">${aaChState(c)}</span>${ce.isNew ? '<span class="chip blue">Neu</span>' : ''}</div><div class="row-gap"><span>Aktiv</span>${sw('ui.draft.ce.active', 'Sender aktiv')}</div></div>
      ${Object.keys(e).length ? notice('bad', 'Bitte korrigiere die markierten Angaben.', '') : ''}
      <div class="stack">${B('general', 'Allgemein', 'tv', gen)}${B('logo', 'Logo', 'eye', logo)}${B('groups', 'Gruppen', 'layers', grp)}${B('epg', 'Programmdaten', 'calendar', epg)}${B('sources', 'Quellen', 'list', aaCeSources(ce))}${B('reception', 'Empfang', 'wifi', rec)}${B('tech', 'Technik', 'settings', tech)}${B('avail', 'Verfügbarkeit', 'alert', avail)}${B('use', 'Nutzung', 'heart', use)}</div>`,
    actions: `${del ? `<button type="button" class="btn quiet dq aa-left" data-act="aaChDel" data-fid="lae:del">${icon('trash', 16)}Sender löschen</button>` : ''}${aaRunningOn(ce.id).v + aaRunningOn(ce.id).r ? `<button type="button" class="btn quiet dq aa-left" data-act="aaStopAsk" data-id="${ce.id}" data-fid="lae:stop">${icon('x', 16)}Sofort beenden …</button>` : ''}<button type="button" class="btn secondary" data-act="aaChClose" data-fid="lae:x">Abbrechen</button><button type="button" class="btn primary" data-act="aaChSave" data-fid="lae:save">Speichern</button>` };
};
ACT.aaChClose = () => { closeDialog(); if (/[?&]edit=/.test(location.hash)) go('#/admin/live/channels?tab=channels'); };
ACT.aaCeLang = (d) => { const ce = ui.draft.ce; ce.langs = ce.langs.includes(d.v) ? ce.langs.filter((x) => x !== d.v) : ce.langs.concat(d.v); if (!ce.langs.length) ce.langs = [d.v]; renderOverlays(); };
ACT.aaCeGroup = (d) => { const ce = ui.draft.ce; ce.groups = ce.groups.includes(d.v) ? ce.groups.filter((x) => x !== d.v) : ce.groups.concat(d.v); renderOverlays(); };
ACT.aaCeSrcName = () => { const ce = ui.draft.ce; ce.name = ce.nameInSource; renderOverlays(); };
ACT.aaCeLogoFile = () => { ui.draft.ce.logoFile = 'logo-' + ui.draft.ce.id + '.png'; renderOverlays(); };
ACT.aaCeLogoReset = () => { ui.draft.ce.logoMode = 'src'; ui.draft.ce.logoUrl = ''; ui.draft.ce.logoFile = ''; toast('Das Logo kommt wieder aus der Quelle.', 'ok'); renderOverlays(); };
ACT.aaCeAddToggle = () => { ui.draft.ce.addOpen = !ui.draft.ce.addOpen; ui.draft.ce.addVal = ''; renderOverlays(); };
ACT.aaCeAdd = () => { const ce = ui.draft.ce; if (!ce.addVal) { toast('Wähle einen Einzelstrom.', 'warn'); return; } const [sid, url] = ce.addVal.split('|'); const s = LIVE.sources.find((x) => x.id === sid); ce.sources.push({ source: sid, name: s.name, url, state: 'ok' }); ce.addOpen = false; ce.addVal = ''; toast(`«${esc(s.name)}» ist Ersatzquelle ${ce.sources.length}.`, 'ok'); renderOverlays(); };
ACT.aaCeAddMerge = () => { const ce = ui.draft.ce; const other = LIVE.duplicates.find((d) => d.channels.includes(ce.id)); closeDialog(); if (other) ACT.aaMergeOpen({ ids: other.channels.join(',') }); else go('#/admin/live/channels?tab=duplicates'); };
ACT.aaCeSrc = (d) => {
  const ce = ui.draft.ce; const i = Number(d.i); const s = ce.sources[i];
  if (d.op === 'up' && i > 0) { [ce.sources[i - 1], ce.sources[i]] = [s, ce.sources[i - 1]]; toast(`«${esc(s.name)}» ist jetzt Quelle ${i}.`, 'ok'); }
  else if (d.op === 'down' && i < ce.sources.length - 1) { [ce.sources[i + 1], ce.sources[i]] = [s, ce.sources[i + 1]]; toast(`«${esc(s.name)}» ist jetzt Quelle ${i + 2}.`, 'ok'); }
  else if (d.op === 'test') { if (s.state === 'fail') toast(`«${esc(s.name)}»: nicht erreichbar (Zeitüberschreitung nach 10 s).`, 'bad'); else toast(`«${esc(s.name)}»: in Ordnung, erstes Bild nach 1.4 s.`, 'ok'); }
  else if (d.op === 'split') {
    const c = SIM.channel(ce.id); const id = 'split-' + (LIVE.channels.length + 1); const nc = Object.assign(aaClone(c), { id, no: null, name: s.name, nameInSource: s.name, sources: [s], active: false, favCount: 0, isNew: false, groups: [] });
    ce.sources.splice(i, 1); c.sources = c.sources.filter((x) => !(x.source === s.source && x.url === s.url)); LIVE.channels.push(nc); toast(`«${esc(s.name)}» ist jetzt ein eigener Sender (inaktiv, ohne Nummer).`, 'ok');
  }
  renderOverlays();
};
ACT.aaChSave = () => {
  const ce = ui.draft.ce; const c = SIM.channel(ce.id); const e = {};
  if (!String(ce.name).trim()) e.name = 'Gib dem Sender einen Namen.';
  if (ce.no !== '' && ce.no != null) { const n = Number(ce.no); if (!(n >= 1 && n <= 9999)) e.no = 'Die Nummer liegt zwischen 1 und 9999.'; else { const o = aaChAll().find((x) => x.id !== c.id && x.no === n); if (o) e.no = `Nummer ${n} hat schon ${o.name}.`; } }
  ce.errors = e; if (Object.keys(e).length) { ui.open['lae:general'] = true; renderOverlays(); return; }
  const offRun = c.active && !ce.active && aaRunningOn(c.id).v + aaRunningOn(c.id).r > 0;
  Object.assign(c, { name: ce.name.trim(), no: ce.no === '' || ce.no == null ? null : Number(ce.no), country: ce.country, langs: ce.langs, category: ce.category, groups: ce.groups, reception: ce.reception, schedule: ce.reception === 'schedule' ? ce.schedule : undefined, preload: ce.preload, sources: ce.sources, noAudio: ce.noAudio, noVideo: ce.noVideo, active: ce.active && !!ce.sources.length && c.state !== 'gone' });
  if (c.active) c.isNew = false; aaMapSet(c, ce.epgSel); closeDialog(); if (/[?&]edit=/.test(location.hash)) go('#/admin/live/channels?tab=channels'); toast(`«${esc(c.name)}» gespeichert.${offRun ? ' Der Sender ist inaktiv; was läuft, läuft weiter bis zum Ende.' : ''}`, 'ok'); render();
};
ACT.aaChDel = () => { openDialog('aaChDelC', { id: ui.draft.ce.id }); };
DIALOGS.aaChDelC = (p) => { const c = SIM.channel(p.id); const rec = aaPlannedOn(c.id); return { title: `Sender «${esc(c.name)}» löschen?`, body: `<p class="lead">${aaPl(c.favCount, 'Favorit', 'Favoriten')} und ${aaPl(rec, 'geplante Aufnahme', 'geplante Aufnahmen')} sind betroffen. Geplante Aufnahmen dieses Senders werden abgesagt; die Profile bekommen eine Warnung.</p>`, actions: `<button type="button" class="btn secondary" data-act="aaChDelBack" data-fid="lae:dx" data-autofocus>Abbrechen</button><button type="button" class="btn danger" data-act="aaChDelOk" data-id="${c.id}" data-fid="lae:dok">${icon('trash', 16)}Löschen</button>` }; };
ACT.aaChDelBack = () => { openDialog('aaChEd', {}); };
ACT.aaChDelOk = (d) => { const c = SIM.channel(d.id); [LIVE.channels, LIVE.goneChannels].forEach((a) => { const k = a.indexOf(c); if (k >= 0) a.splice(k, 1); }); closeDialog(); if (/[?&]edit=/.test(location.hash)) go('#/admin/live/channels?tab=channels'); toast(`Sender «${esc(c.name)}» gelöscht.`, 'ok'); render(); };

/* ======================================================================================
   LA-07 Duplikate und Zusammenführen
   ====================================================================================== */
function aaDupTab() {
  return aaGate(aaEmptyBox('Keine möglichen Duplikate', 'Slouchcat hat keine Sender gefunden, die vermutlich dasselbe zeigen.'), () => {
    const list = aaDupList(); if (!list.length) return aaEmptyBox('Keine möglichen Duplikate', 'Alle Vorschläge sind erledigt. Neue Vorschläge erscheinen nach dem nächsten Import.');
    return `<div class="tblw aa-tbl"><table class="tbl"><thead><tr><th>Sender</th><th>Grund</th><th>Vorschlag</th><th class="end"><span class="vh">Aktionen</span></th></tr></thead><tbody>${list.map((d) => `<tr><td class="main" data-l="Sender"><div class="aa-pair">${d.channels.map((id, i) => `<span class="aa-chn">${LU.logo(id)}<span>${esc(d.names[i])}<small>${esc(aaSrcName(SIM.channel(id).sources[0].source))}</small></span></span>`).join('')}</div></td><td data-l="Grund">${esc(d.reason)}</td><td data-l="Vorschlag">«${esc(SIM.channel(d.keep).name)}» bleibt</td>
      <td class="end"><div class="ta"><button type="button" class="btn secondary sm" data-act="aaMergeOpen" data-ids="${d.channels.join(',')}" data-fid="la07:m:${d.id}">Zusammenführen …</button><button type="button" class="btn quiet sm" data-act="aaNoDupe" data-id="${d.id}" data-fid="la07:n:${d.id}">Kein Duplikat</button></div></td></tr>`).join('')}</tbody></table></div>
      <p class="muted aa-small">Weitere Sender lassen sich von Hand zusammenführen: in der Senderliste zwei oder mehr Sender wählen und «Zusammenführen» drücken. Rückgängig: im Sender unter «Quellen» → «Als eigenen Sender abtrennen».</p>`;
  }, { rows: 3, h: 72 });
}
ACT.aaNoDupe = (d) => { AA2.dupDone[d.id] = true; toast('Slouchcat merkt sich das Paar: kein Duplikat.', 'ok'); render(); };
const AA_ATT = [['name', 'Name'], ['logo', 'Logo'], ['no', 'Nummer'], ['groups', 'Gruppen'], ['epg', 'Programmdaten-Zuordnung'], ['meta', 'Land, Sprache, Kategorie']];
ACT.aaMergeOpen = (d) => {
  const ids = String(d.ids).split(',').filter(Boolean); const chs = ids.map((id) => SIM.channel(id)).filter(Boolean); if (chs.length < 2) { toast('Dazu fehlt ein Sender.', 'warn'); return; }
  const keep = chs.slice().sort((a, b) => (b.no != null) - (a.no != null) || b.favCount - a.favCount)[0].id;
  ui.draft.mg = { ids, keep, take: { name: keep, logo: keep, no: keep, groups: 'union', epg: keep, meta: keep }, order: [] };
  aaMgOrder(); openDialog('aaMerge', {});
};
function aaMgOrder() { const g = ui.draft.mg; const chs = [g.keep].concat(g.ids.filter((i) => i !== g.keep)); g.order = chs.flatMap((id) => SIM.channel(id).sources.map((s, i) => id + ':' + i)); }
ACT.aaMgKeep = () => { const g = ui.draft.mg; Object.keys(g.take).forEach((k) => { if (g.take[k] !== 'union') g.take[k] = g.keep; }); aaMgOrder(); renderOverlays(); };
const aaMgSrc = (key) => { const [id, i] = key.split(':'); return { c: SIM.channel(id), s: SIM.channel(id).sources[Number(i)] }; };
DIALOGS.aaMerge = () => {
  const g = ui.draft.mg; const chs = g.ids.map((id) => SIM.channel(id)); const keep = SIM.channel(g.keep); const others = chs.filter((c) => c.id !== g.keep);
  const val = (att) => { const c = SIM.channel(g.take[att] === 'union' ? g.keep : g.take[att]); return att === 'name' ? c.name : att === 'logo' ? `Logo von «${c.name}»` : att === 'no' ? `Nr. ${aaNum(c)}` : att === 'groups' ? (g.take.groups === 'union' ? [...new Set(chs.flatMap((x) => x.groups))] : c.groups).map(aaGroupName).join(', ') || 'keine' : att === 'epg' ? (c.epgKey || 'keine') : `${c.country} · ${c.langs.join(', ')} · ${c.category}`; };
  const rows = AA_ATT.map(([k, l]) => `<div class="aa-mrow"><label for="mg-${k}">${l}</label>${selectEl('ui.draft.mg.take.' + k, (k === 'groups' ? [{ v: 'union', t: 'Vereinigung aller Gruppen' }] : []).concat(chs.map((c) => ({ v: c.id, t: `von «${c.name}»` }))), { label: l }).replace('<select ', `<select id="mg-${k}" `)}<small>${esc(val(k))}</small></div>`).join('');
  const order = g.order.map((key, i) => { const { c, s } = aaMgSrc(key); return `<li class="aa-rank"><span class="aa-rk">${i + 1}</span><span class="aa-rt"><b>${esc(s.name)}</b><small>${esc(aaSrcName(s.source))} · von «${esc(c.name)}»</small></span><span class="aa-rb"><button type="button" class="btn quiet icon sm" data-act="aaMgMove" data-i="${i}" data-dir="-1" ${i === 0 ? 'disabled' : ''} aria-label="${esc(s.name)} nach oben" data-fid="mg:u${i}">${icon('arrowUp', 16)}</button><button type="button" class="btn quiet icon sm" data-act="aaMgMove" data-i="${i}" data-dir="1" ${i === g.order.length - 1 ? 'disabled' : ''} aria-label="${esc(s.name)} nach unten" data-fid="mg:d${i}">${icon('arrowDown', 16)}</button></span></li>`; }).join('');
  const favs = others.reduce((a, c) => a + c.favCount, 0); const recs = others.reduce((a, c) => a + aaPlannedOn(c.id), 0);
  const sum = `<b>${esc(keep.name)}</b> bleibt. ${g.order.map((k, i) => ({ k, i })).filter((x) => aaMgSrc(x.k).c.id !== g.keep).map((x) => `«${esc(aaMgSrc(x.k).s.name)}» wird zur ${x.i === 0 ? 'Hauptquelle' : 'Ersatzquelle ' + (x.i + 1)}`).join(', ')}${others.length ? '. ' : ''}${aaPl(favs, 'Favorit', 'Favoriten')} und ${aaPl(recs, 'geplante Aufnahme', 'geplante Aufnahmen')} gehen auf «${esc(keep.name)}» über.`;
  return { title: 'Sender zusammenführen', wide: true, body: `<div class="stack"><fieldset class="aa-fs"><legend>1 · Welcher Sender bleibt?</legend>${chs.map((c) => `<label class="check"><input type="radio" name="mgk" value="${c.id}" data-bind="ui.draft.mg.keep" data-after="aaMgKeep" ${g.keep === c.id ? 'checked' : ''} data-fid="mg:k:${c.id}"><span>${esc(c.name)}<small>${c.no != null ? 'Nr. ' + c.no : 'ohne Nummer'} · ${aaPl(c.favCount, 'Favorit', 'Favoriten')} · ${esc(aaSrcName(c.sources[0] ? c.sources[0].source : ''))}</small></span></label>`).join('')}</fieldset>
    <fieldset class="aa-fs"><legend>2 · Was übernimmt der bleibende Sender?</legend>${rows}</fieldset>
    <fieldset class="aa-fs"><legend>3 · Quellen in dieser Reihenfolge</legend><ol class="aa-ranks">${order}</ol></fieldset>
    <div class="aa-sum"><b>4 · Zusammenfassung</b><p>${sum}</p></div></div>`, actions: `<button type="button" class="btn secondary" data-act="close" data-fid="mg:x">Abbrechen</button><button type="button" class="btn primary" data-act="aaMgOk" data-fid="mg:ok">Zusammenführen</button>` };
};
ACT.aaMgMove = (d) => { const g = ui.draft.mg; const i = Number(d.i); const j = i + Number(d.dir); if (j < 0 || j >= g.order.length) return; [g.order[i], g.order[j]] = [g.order[j], g.order[i]]; renderOverlays(); };
ACT.aaMgOk = () => {
  const g = ui.draft.mg; const chs = g.ids.map((id) => SIM.channel(id)); const keep = SIM.channel(g.keep); const T = (k) => SIM.channel(g.take[k]); const others = chs.filter((c) => c.id !== g.keep);
  const sources = g.order.map((k) => aaMgSrc(k).s); const favs = others.reduce((a, c) => a + c.favCount, 0); const nm = T('name'); const no = T('no'); const meta = T('meta'); const ep = T('epg'); const lg = T('logo');
  const upd = { name: nm.name, nameInSource: keep.nameInSource, no: no.no, slug: lg.slug, country: meta.country, langs: meta.langs.slice(), category: meta.category, epgKey: ep.epgKey, epg: ep.epg, groups: g.take.groups === 'union' ? [...new Set(chs.flatMap((x) => x.groups))] : T('groups').groups.slice() };
  others.forEach((o) => { LIVE.profiles.forEach((p) => SIM.rules(p.id).forEach((r) => { if (r.ch === o.id) SIM.updateRule(r.id, { ch: keep.id }); })); const k = LIVE.channels.indexOf(o); if (k >= 0) LIVE.channels.splice(k, 1); });
  Object.assign(keep, upd, { sources, favCount: keep.favCount + favs }); keep.active = keep.active || others.some((o) => o.active);
  AA.dupHandled++; closeDialog(); toast(`«${esc(keep.name)}» hat jetzt ${aaPl(sources.length, 'Quelle', 'Quellen')} in dieser Rangfolge. <a href="#/admin/live/channels?tab=channels&edit=${keep.id}&open=sources">Ansehen</a>`, 'ok', 8000); AA.sel = {}; AA.selAll = false; render();
};

/* ======================================================================================
   LA-08 Sendergruppen
   ====================================================================================== */
const aaGroupCount = (id) => LIVE.channels.filter((c) => c.groups.includes(id) && c.active).length;
function aaGroupsTab() {
  return aaGate(aaEmptyBox('Keine Gruppen', 'Lege eine Gruppe an, um Sender für die Zuschauer zu ordnen.', `<button type="button" class="btn primary" data-act="aaGroupOpen" data-id="" data-fid="la08:add2">${icon('plus', 16)}Gruppe hinzufügen</button>`), () => {
    const gs = LIVE.groups; if (!gs.length) return aaEmptyBox('Keine Gruppen', 'Lege eine Gruppe an.');
    return `<div class="tblw aa-tbl aa-gr"><table class="tbl"><thead><tr><th>Name</th><th>Herkunft</th><th class="end">Sender</th><th>Für Zuschauer sichtbar</th><th>Reihenfolge</th><th class="end"><span class="vh">Aktionen</span></th></tr></thead><tbody>${gs.map((g, i) => `<tr><td class="main" data-l="Name"><button type="button" class="aa-link" data-act="aaGroupOpen" data-id="${g.id}" data-fid="la08:n:${g.id}">${esc(g.name)}</button></td>
      <td data-l="Herkunft">${g.origin === 'own' ? 'eigene' : `aus Quelle «${esc(aaSrcName(g.source))}»`}</td><td class="end" data-l="Sender">${aaGroupCount(g.id)}</td><td data-l="Sichtbar">${aaSw(g.visible !== false, 'aaGrVis', { id: g.id }, `${g.name} für Zuschauer sichtbar`, 'la08:v:' + g.id)}</td>
      <td data-l="Reihenfolge"><button type="button" class="btn quiet icon sm" data-act="aaGrMove" data-id="${g.id}" data-dir="-1" ${i === 0 ? 'disabled' : ''} aria-label="${esc(g.name)} nach oben" data-fid="la08:u:${g.id}">${icon('arrowUp', 16)}</button><button type="button" class="btn quiet icon sm" data-act="aaGrMove" data-id="${g.id}" data-dir="1" ${i === gs.length - 1 ? 'disabled' : ''} aria-label="${esc(g.name)} nach unten" data-fid="la08:d:${g.id}">${icon('arrowDown', 16)}</button></td>
      <td class="end"><div class="ta"><button type="button" class="btn quiet sm" data-act="aaGroupOpen" data-id="${g.id}" data-fid="la08:e:${g.id}">Bearbeiten</button>${g.origin === 'own' ? `<button type="button" class="btn quiet dq sm" data-act="aaGrDel" data-id="${g.id}" data-fid="la08:x:${g.id}">Löschen</button>` : ''}</div></td></tr>`).join('')}</tbody></table></div>
      <p class="muted aa-small">Gruppen aus Quellen lassen sich umbenennen (der eigene Name bleibt bei Aktualisierungen) und ausblenden, aber nicht löschen, solange die Quelle sie liefert.</p>`;
  }, { rows: 5, h: 48 });
}
ACT.aaGrVis = (d) => { const g = LIVE.groups.find((x) => x.id === d.id); g.visible = g.visible === false; toast(`«${esc(g.name)}» ist für Zuschauer ${g.visible ? 'sichtbar' : 'ausgeblendet'}.`, 'ok'); render(); };
ACT.aaGrMove = (d) => { const i = LIVE.groups.findIndex((x) => x.id === d.id); const j = i + Number(d.dir); if (j < 0 || j >= LIVE.groups.length) return; const a = LIVE.groups; [a[i], a[j]] = [a[j], a[i]]; render(); setTimeout(() => { const el = document.querySelector(`[data-fid="la08:${d.dir < 0 ? 'u' : 'd'}:${d.id}"]`); if (el && !el.disabled) el.focus(); }, 0); };
ACT.aaGroupOpen = (d) => {
  const g = d.id ? LIVE.groups.find((x) => x.id === d.id) : null;
  ui.draft.gr = { id: g ? g.id : null, name: g ? g.name : '', visible: g ? g.visible !== false : true, origin: g ? g.origin : 'own', ids: Object.fromEntries(LIVE.channels.filter((c) => g && c.groups.includes(g.id)).map((c) => [c.id, true])), q: '', err: '' };
  openDialog('aaGroup', {});
};
DIALOGS.aaGroup = () => {
  const d = ui.draft.gr; const src = d.origin !== 'own'; const q = d.q.trim().toLowerCase(); const list = LIVE.channels.filter((c) => c.active && (!q || c.name.toLowerCase().includes(q)));
  const chosen = LIVE.channels.filter((c) => d.ids[c.id]);
  return { title: d.id ? `Gruppe «${esc(LIVE.groups.find((x) => x.id === d.id).name)}» bearbeiten` : 'Gruppe hinzufügen', wide: true, body: `<div class="stack">${aaF('gr-n', 'Name', aaI('ui.draft.gr.name', 'gr-n', { auto: true, err: d.err }), { err: d.err, hint: src ? 'Der eigene Name bleibt bei Aktualisierungen erhalten.' : '' })}
    <div class="srow"><div class="st"><b>Für Zuschauer sichtbar</b></div><div class="sc">${sw('ui.draft.gr.visible', 'Für Zuschauer sichtbar')}</div></div>
    ${src ? '<p class="muted">Die Sender dieser Gruppe liefert die Quelle. Du kannst sie umbenennen und ausblenden.</p>' : `<div class="fld"><label for="gr-q">Sender suchen</label><input class="inp" id="gr-q" type="search" data-bind="ui.draft.gr.q" data-live data-fid="gr:q" value="${esc(d.q)}" autocomplete="off"></div>
      <div class="aa-cbs aa-scroll" role="group" aria-label="Sender der Gruppe">${list.map((c) => `<label class="check"><input type="checkbox" ${d.ids[c.id] ? 'checked' : ''} data-act="aaGrPick" data-id="${c.id}" data-fid="gr:c:${c.id}"><span>${c.no != null ? c.no + ' · ' : ''}${esc(c.name)}</span></label>`).join('') || '<p class="muted">Kein Sender passt.</p>'}</div>
      <div class="fld"><span class="lbl">Gewählt: ${chosen.length}</span><div class="pills">${chosen.map((c) => `<span class="chip info">${esc(c.name)}</span>`).join('') || '<span class="muted">noch keine</span>'}</div></div>`}</div>`,
    actions: `<button type="button" class="btn secondary" data-act="close" data-fid="gr:x">Abbrechen</button><button type="button" class="btn primary" data-act="aaGrSave" data-fid="gr:ok">Speichern</button>` };
};
ACT.aaGrPick = (d) => { const g = ui.draft.gr; if (g.ids[d.id]) delete g.ids[d.id]; else g.ids[d.id] = true; renderOverlays(); };
ACT.aaGrSave = () => {
  const d = ui.draft.gr; if (!d.name.trim()) { d.err = 'Gib der Gruppe einen Namen.'; renderOverlays(); return; }
  if (LIVE.groups.some((g) => g.id !== d.id && g.name.toLowerCase() === d.name.trim().toLowerCase())) { d.err = 'Eine Gruppe mit diesem Namen gibt es schon.'; renderOverlays(); return; }
  let g = LIVE.groups.find((x) => x.id === d.id); if (!g) { g = { id: 'g-' + (LIVE.groups.length + 1) + '-' + Date.now() % 1000, name: '', origin: 'own' }; LIVE.groups.push(g); }
  g.name = d.name.trim(); g.visible = d.visible;
  if (g.origin === 'own') LIVE.channels.forEach((c) => { const has = c.groups.includes(g.id); if (d.ids[c.id] && !has) c.groups.push(g.id); else if (!d.ids[c.id] && has) c.groups = c.groups.filter((x) => x !== g.id); });
  closeDialog(); toast(`Gruppe «${esc(g.name)}» gespeichert.`, 'ok'); render();
};
ACT.aaGrDel = (d) => openDialog('aaGrDelC', { id: d.id });
DIALOGS.aaGrDelC = (p) => { const g = LIVE.groups.find((x) => x.id === p.id); return { title: `Gruppe «${esc(g.name)}» löschen?`, body: `<p class="lead">${aaPl(aaGroupCount(g.id), 'Sender ist', 'Sender sind')} in dieser Gruppe. Die Sender bleiben erhalten, nur die Gruppe verschwindet.</p>`, actions: `<button type="button" class="btn secondary" data-act="close" data-fid="grd:x" data-autofocus>Abbrechen</button><button type="button" class="btn danger" data-act="aaGrDelOk" data-id="${g.id}" data-fid="grd:ok">${icon('trash', 16)}Löschen</button>` }; };
ACT.aaGrDelOk = (d) => { const k = LIVE.groups.findIndex((x) => x.id === d.id); const [g] = LIVE.groups.splice(k, 1); LIVE.channels.forEach((c) => { c.groups = c.groups.filter((x) => x !== g.id); }); closeDialog(); toast(`Gruppe «${esc(g.name)}» gelöscht.`, 'ok'); render(); };

/* ======================================================================================
   LA-20 bis LA-28: geänderte bestehende Seiten (15-administration.md 3.17)
   Nur das Neue ist ausgearbeitet; alles andere zeigt die Seite in ihrer bisherigen Art knapp als Beispiel.
   ====================================================================================== */
const aaAv = (name) => `<span class="av ${({ Anna: 'a1', Luca: 'a2', Mia: 'a3' })[name] || 'a1'}" aria-hidden="true">${esc(name[0])}</span>`;
const aaMbit = (x) => `${fmt.dec(x, 1, 1)} Mbit/s`;
const AA_SAMPLES = [
  { k: 'v1', type: 'Video', title: 'Oppenheimer', acct: 'familie.keller@example.ch', prof: 'Luca', dev: 'Tablet Kinderzimmer', mode: 'direkt', br: 5.6 },
  { k: 'h1', type: 'Hörbuch', title: 'Der Prozess', acct: 'grosseltern@example.ch', prof: 'Grosseltern', dev: 'Küchenradio', mode: 'direkt', br: 0.2 },
  { k: 'r1', type: 'Radio', title: 'Radio SRF 3', acct: 'familie.keller@example.ch', prof: 'Mia', dev: 'Pixel 8', mode: 'direkt', br: 0.1 },
];
const aaSamples = () => (aaEmpty() ? [] : AA_SAMPLES.filter((s) => !ADM.endedS[s.k]));
const aaPrivPlay = () => (aaEmpty() || ADM.endedS.pr ? null : { acct: 'familie.keller@example.ch', prof: 'Luca', dev: 'Tablet Kinderzimmer', mode: 'direkt', br: 4.8 });

/* ---------- LA-20 Verwaltungsübersicht ---------- */
function aaHome() {
  const empty = aaEmpty(); const st = aaState(); const vs = admViewers(); const run = admRecRunning(); const c = admCounts(); const sm = aaSamples();
  const tiles = [['Medien', 'film', 'Bibliotheken, Metadaten, Pflege, Wünsche, Podcasts, Radio', '#/admin/x/libraries', false],
    ['Live-TV', 'live', empty ? 'Noch nicht eingerichtet · Quelle hinzufügen' : `${st[0]} · ${aaPl(vs.length, 'Zuschauer', 'Zuschauer')} · ${aaPl(run, 'Aufnahme läuft', 'Aufnahmen laufen')} · ${aaPl(c.unavailable, 'Sender gestört', 'Sender gestört')}`, '#/admin/live', true],
    ['Nutzer & Geräte', 'users', `${LIVE.accounts.length} Konten, Geräte und Streams`, '#/admin/accounts', false], ['Automatisierung', 'zap', 'Hintergrundjobs, Integrationen, Benachrichtigungen', '#/admin/jobs', false], ['System', 'server', 'Statistiken, Sicherung, Server', '#/admin/server', false]];
  const rows = vs.map((v) => `<li class="aa-sr">${aaAv(v.prof)}<span><b>${esc(v.prof)} · ${esc(SIM.channel(v.ch).name)}</b><small>${esc(v.dev)} · ${/zeitversetzt/.test(v.off) ? esc(v.off) : 'Live'}</small></span></li>`).concat(sm.filter((s) => s.type === 'Video').map((s) => `<li class="aa-sr">${aaAv(s.prof)}<span><b>${esc(s.prof)} · ${esc(s.title)}</b><small>${esc(s.dev)} · Direkt</small></span></li>`));
  const att = aaAttRowsFor3();
  return `<div class="tiles">${tiles.map(([t, ic, x, h, nw]) => `<div class="tile${nw ? ' new' : ''}"><span class="ik">${icon(ic, 18)}</span><div class="tt"><h2><a href="${h}" data-fid="la20:t:${t}">${t}</a>${nw ? ' <span class="newtag">NEU</span>' : ''}</h2><p>${x}</p></div><span class="arr">${icon('chevR', 16)}</span></div>`).join('')}</div>
    <div class="cols2"><section class="card"><div class="ch"><span class="ik">${icon('play', 18)}</span><h2>Aktive Wiedergaben</h2><a class="btn quiet sm" href="#/admin/streams" data-fid="la20:allstreams">Alle ansehen</a></div>${rows.length ? `<ul class="aa-srows">${rows.join('')}</ul>` : '<p class="muted">Gerade schaut niemand.</p>'}</section>
    <section class="card"><div class="ch"><span class="ik">${icon('alert', 18)}</span><h2>Braucht Aufmerksamkeit</h2></div>${att.html}${att.more ? `<a class="lnk" href="#/admin/live" data-fid="la20:more">Alle in Live-TV (${att.n})</a>` : ''}</section></div>`;
}
function aaAttRowsFor3() { const a = aaAttention(); const top = a.filter((x) => x.k !== 'info').slice(0, 3); return { html: top.length ? `<ul class="aa-atts">${top.map(aaAttRow).join('')}</ul>` : '<p class="muted">Alles in Ordnung.</p>', more: a.length > top.length, n: a.length }; }
route('/admin', () => aaView({ area: 'overview', page: '', title: 'Übersicht', intro: 'Der Zustand deines Servers auf einen Blick.', live: aaMode() === 'normal' ? admLiveText() : null, html: aaGate(null, aaHome, { rows: 4, h: 96 }) }));

/* ---------- LA-21 Aktive Wiedergaben ---------- */
const AA_TYPES = ['Alle', 'Video', 'Hörbuch', 'Podcast', 'Radio', 'Live-TV'];
function aaPlays() {
  const live = admViewers().map((v) => ({ type: 'Live-TV', k: 'l' + v.i, i: v.i, ch: v.ch, prof: v.prof, acct: v.acct, dev: v.dev, mode: v.tr ? `umgewandelt ${v.res}p (Hardware QSV)` : 'direkt', br: v.br, off: v.off, since: v.since }));
  const pp = aaPrivPlay(); if (pp) live.push(Object.assign({ type: 'Live-TV', k: 'pr', priv: true, since: SIM.now() - 52 }, pp));
  return live.concat(aaSamples().map((s) => Object.assign({ since: SIM.now() - 25 }, s)));
}
function aaPlayTile(p) {
  const kv = (a) => `<dl class="kv">${a.map(([k, v]) => `<dt>${k}</dt><dd>${v}</dd>`).join('')}</dl>`;
  let head; let rows;
  if (p.priv) { head = `<span class="ik">${icon('lock', 18)}</span><div><b>Private Aufnahme</b><small>Titel und Sender sind für dich nicht sichtbar.</small></div>`; rows = [['Konto', esc(p.acct)], ['Profil', esc(p.prof)], ['Gerät', esc(p.dev)], ['Modus', p.mode], ['Bitrate', aaMbit(p.br)], ['Seit', fmt.time(p.since)]]; }
  else if (p.ch) { const c = SIM.channel(p.ch); const now = SIM.flags().noEpg ? null : SIM.nowNext(p.ch).now; head = `${LU.logo(c)}<div><b>${esc(c.name)}</b><small>${now ? esc(now.title) : 'Keine Programmdaten'}</small></div>`; rows = [['Konto', esc(p.acct)], ['Profil', esc(p.prof)], ['Gerät', esc(p.dev)], ['Modus', esc(p.mode)], ['Bitrate', aaMbit(p.br)], ['Wiedergabe', /zeitversetzt/.test(p.off) ? esc(p.off.replace('−', '−').replace(' zeitversetzt', ' zeitversetzt')) : 'Live'], ['Seit', fmt.time(p.since)]]; }
  else { head = `<span class="ik">${icon({ Video: 'film', 'Hörbuch': 'book', Radio: 'radio', Podcast: 'mic' }[p.type] || 'play', 18)}</span><div><b>${esc(p.title)}</b><small>${p.type} (unverändert)</small></div>`; rows = [['Konto', esc(p.acct)], ['Profil', esc(p.prof)], ['Gerät', esc(p.dev)], ['Modus', p.mode], ['Bitrate', aaMbit(p.br)]]; }
  return `<article class="card aa-pt"><header class="aa-ph">${head}</header>${kv(rows)}<div class="foot"><button type="button" class="btn secondary sm" data-act="aaEnd" data-k="${p.k}" ${p.i != null ? `data-i="${p.i}"` : ''} data-fid="la21:end:${p.k}">Beenden</button></div></article>`;
}
function aaStreamsView() {
  const f = ui.draft.stf || 'Alle'; const all = aaPlays(); const list = f === 'Alle' ? all : all.filter((p) => p.type === f); const bw = Math.round(all.reduce((a, p) => a + p.br, 0) * 10) / 10; const tr = all.filter((p) => /umgewandelt/.test(p.mode)).length;
  const bar = `<div class="subtabs" role="group" aria-label="Art der Wiedergabe">${AA_TYPES.map((t) => `<button type="button" aria-pressed="${f === t}" data-act="aaStF" data-t="${t}" data-fid="la21:f:${t}">${t}<span class="c">${t === 'Alle' ? all.length : all.filter((p) => p.type === t).length}</span></button>`).join('')}</div>`;
  const strip = `<div class="stats" style="--n:4">${aaStat('Wiedergaben', all.length, `davon Live-TV: ${all.filter((p) => p.type === 'Live-TV').length}`, {})}${aaStat('Bandbreite', aaMbit(bw), 'enthält Live-TV', {})}${aaStat('Umwandlungen', tr, 'laufen gerade', {})}${aaStat('Live-TV-Wiedergaben', all.filter((p) => p.type === 'Live-TV').length, `${aaPl(all.filter((p) => p.type === 'Live-TV' && !p.priv).length, 'Zuschauer live', 'Zuschauer live')} · ${aaPl(all.filter((p) => p.priv).length, 'Aufnahme wird abgespielt', 'Aufnahmen werden abgespielt')}`, {})}</div>`;
  return `${strip}${bar}${list.length ? `<div class="tiles">${list.map(aaPlayTile).join('')}</div>` : aaEmptyBox(f === 'Alle' ? 'Gerade schaut niemand' : `Gerade läuft kein ${f}`, 'Sobald jemand etwas startet, erscheint es hier.')}<p class="muted aa-small">Für Live-TV zeigt die Verwaltung wie bei Videos Konto, Profil, Gerät und laufende Sendung. Bei der Wiedergabe einer Aufnahme steht nur «Private Aufnahme»: ohne Titel, ohne Sender.</p>`;
}
ACT.aaStF = (d) => { ui.draft.stf = d.t; render(); };
ACT.aaEnd = (d) => { if (d.i != null && d.i !== '') ADM.ended[Number(d.i)] = true; else ADM.endedS[d.k] = true; toast('Wiedergabe beendet. Das Gerät zeigt eine Meldung.', 'ok'); render(); };
route('/admin/streams', () => aaView({ area: 'people', page: 'streams', title: 'Aktive Wiedergaben', intro: 'Was gerade auf deinem Server läuft: Video, Hörbuch, Podcast, Radio und neu Live-TV.', live: aaMode() === 'normal' ? admLiveText() : null, html: aaGate(aaEmptyBox('Gerade schaut niemand', 'Sobald jemand etwas startet, erscheint es hier.'), aaStreamsView, { rows: 3, h: 150 }) }));

/* ---------- LA-22 Konten und Einladungen ---------- */
const aaAccR = (a) => (a.email === LIVE.account.email ? SIM.rawRights('account') : a.rights);
const aaDefQ = () => LIVE.settings.limits.defaultQuotaGB;
AA.invites = [{ email: 'tante.erika@example.ch', until: LIVE.at(5, '23:59'), live: false }];
function aaLiveBlock(root, o) {
  const d = getPath(root); const dis = !d.liveTv; const q = d.quota === '' || d.quota == null ? aaDefQ() : Number(d.quota); let ns = '';
  if (o.used != null && d.recordings && d.liveTv && q < o.used) ns += notice('warn', `Belegt sind ${fmt.num(o.used)} GB.`, 'Neue Aufnahmen werden abgelehnt, bis das Konto Platz freigibt. Es wird nichts gelöscht.');
  if (o.wasLive && !d.liveTv) ns += notice('', 'Live-TV ist für dieses Konto aus.', 'Laufende Wiedergaben enden. Geplante Aufnahmen ruhen. Aufnahmen bleiben erhalten.');
  return `<div class="aa-lblock"><div class="srow"><div class="st"><b>Live-TV</b><span>Das Konto darf Live-TV nutzen. Das Hauptprofil entscheidet für seine Profile.</span></div><div class="sc">${sw(root + '.liveTv', 'Live-TV')}<span>${d.liveTv ? 'an' : 'aus'}</span></div></div>
    <div class="srow"><div class="st"><b>Aufnahmen</b><span>Kontingent in GB; leer gilt der Standard von ${aaDefQ()} GB.${dis ? ' Nur mit Live-TV wählbar.' : ''}</span></div><div class="sc">${sw(root + '.recordings', 'Aufnahmen', { disabled: dis })}<label class="vh" for="${o.id}-q">Kontingent in GB</label>${aaI(root + '.quota', o.id + '-q', { num: true, dis: dis || !d.recordings, ph: String(aaDefQ()), rerender: true })}<span>GB</span>${o.used != null ? `<span class="muted">belegt ${fmt.num(o.used)} GB</span>` : ''}</div></div>
    <div class="srow"><div class="st"><b>Timeshift</b><span>Anhalten und Zurückspulen bis zum eigenen Einschalten.${dis ? ' Nur mit Live-TV wählbar.' : ''}</span></div><div class="sc">${sw(root + '.timeshift', 'Timeshift', { disabled: dis })}</div></div>${ns}
    <p class="muted aa-small">Live-TV kennt keine Altersprüfung. Profile mit Altersgrenze sehen alle Sender, wenn das Hauptprofil sie freigibt.</p></div>`;
}
const aaLiveSum = (d) => (d.liveTv ? `Live-TV · an · Aufnahmen ${d.recordings ? `an (${d.quota === '' || d.quota == null ? aaDefQ() : d.quota} GB)` : 'aus'} · Timeshift ${d.timeshift ? 'an' : 'aus'}` : 'Live-TV · aus');
function aaAccList(q) {
  const f = ui.draft.accf || 'Alle'; const tab = q.tab === 'invites' ? 'invites' : 'accounts';
  const tabs = subTabs('#/admin/accounts', [['accounts', 'Konten', LIVE.accounts.length], ['invites', 'Einladungen', AA.invites.length]], tab);
  if (tab === 'invites') return tabs + (AA.invites.length ? `<div class="tblw aa-tbl"><table class="tbl"><thead><tr><th>E-Mail</th><th>Gültig bis</th><th>Live-TV</th></tr></thead><tbody>${AA.invites.map((i) => `<tr><td class="main" data-l="E-Mail">${esc(i.email)}</td><td data-l="Gültig bis">${fmt.day(i.until)}</td><td data-l="Live-TV"><span class="chip ${i.live ? 'ok' : 'info'}">${i.live ? 'an' : 'aus'}</span></td></tr>`).join('')}</tbody></table></div>` : aaEmptyBox('Keine offenen Einladungen', 'Lade jemanden ein, damit er ein Konto bekommt.'));
  const rows = LIVE.accounts.filter((a) => f === 'Alle' || aaAccR(a).liveTv);
  return `${tabs}<div class="subtabs" role="group" aria-label="Filter">${['Alle', 'Mit Live-TV'].map((t) => `<button type="button" aria-pressed="${f === t}" data-act="aaAccF" data-t="${t}" data-fid="la22:f:${t}">${t}</button>`).join('')}</div>
    <div class="tblw aa-tbl"><table class="tbl"><thead><tr><th>Konto</th><th>Rolle</th><th>Live-TV</th><th>Aufnahmen</th></tr></thead><tbody>${rows.map((a) => { const r = aaAccR(a); return `<tr class="aa-rowlink"><td class="main" data-l="Konto"><a class="lnk" href="#/admin/accounts/${encodeURIComponent(a.email)}" data-fid="la22:a:${a.email}">${esc(a.email)}</a></td><td data-l="Rolle">${a.email.startsWith('admin') ? 'Server-Admin' : 'Konto'}</td><td data-l="Live-TV"><span class="chip ${r.liveTv ? 'ok' : 'info'}">${r.liveTv ? 'an' : 'aus'}</span></td><td data-l="Aufnahmen">${r.liveTv && r.recordings ? `${fmt.num(a.usedGB)} von ${fmt.num(a.quotaGB || aaDefQ())} GB · ${aaPl(a.count, 'gespeicherte Aufnahme', 'gespeicherte Aufnahmen')}` : '–'}</td></tr>`; }).join('') || '<tr><td colspan="4" class="muted">Kein Konto mit Live-TV.</td></tr>'}</tbody></table></div>`;
}
ACT.aaAccF = (d) => { ui.draft.accf = d.t; render(); };
route('/admin/accounts', (pm, q) => aaView({ area: 'people', page: 'accounts', title: 'Konten', intro: 'Wer deinen Server nutzt, mit welchen Rechten und welchem Kontingent.', actions: `<button type="button" class="btn primary" data-act="aaInviteOpen" data-fid="la22:inv">${icon('mail', 16)}Einladung erstellen</button>`, html: aaGate(null, () => aaAccList(q), { rows: 4, h: 56 }) }));
const aaAccKey = (d) => JSON.stringify([d.liveTv, d.recordings, d.timeshift, d.quota]);
route('/admin/accounts/:id', (pm) => {
  const fresh = parseHash().path !== lastPath; const a = LIVE.accounts.find((x) => x.email === pm.id);
  if (a && (fresh || !ui.draft.acc)) { const r = aaAccR(a); ui.draft.acc = { email: a.email, liveTv: r.liveTv, recordings: r.recordings, timeshift: r.timeshift, quota: a.email === LIVE.account.email || a.quotaGB ? a.quotaGB : '', wasLive: r.liveTv }; AA.accSnap = aaAccKey(ui.draft.acc); }
  let html;
  if (!a) html = notice('warn', 'Dieses Konto gibt es nicht.', '', '<a class="btn secondary" href="#/admin/accounts" data-fid="la22:nf">Zu den Konten</a>');
  else html = aaGate(null, () => {
    const d = ui.draft.acc; const dirty = aaAccKey(d) !== AA.accSnap; const r = aaAccR(a);
    return `<section class="card"><div class="ch"><span class="ik">${icon('user', 18)}</span><h2>Konto</h2></div><dl class="kv"><dt>E-Mail</dt><dd>${esc(a.email)}</dd><dt>Rolle</dt><dd>${a.email.startsWith('admin') ? 'Server-Admin' : 'Konto'}</dd>${r.liveTv && r.recordings ? `<dt>Aufnahmen</dt><dd>${fmt.num(a.usedGB)} von ${fmt.num(a.quotaGB || aaDefQ())} GB · ${aaPl(a.count, 'gespeicherte Aufnahme', 'gespeicherte Aufnahmen')}</dd>` : ''}</dl><p class="muted aa-small">Als Server-Admin siehst du nur Belegung und Anzahl, nie die Titel der Aufnahmen.</p></section>
      ${band('la22:live', 'Live-TV', esc(aaLiveSum(d)), aaLiveBlock('ui.draft.acc', { id: 'la22', used: r.recordings ? a.usedGB : null, wasLive: d.wasLive }), { icon: 'live' })}
      ${band('la22:limits', 'Grenzen', 'gleichzeitige Streams, Bitrate, Auflösung, Umwandlung', '<p class="muted">Diese bestehenden Grenzen gelten auch für Live-TV. Es gibt dafür kein eigenes Feld.</p>', { icon: 'settings' })}
      ${dirty ? `<div class="savebar" role="region" aria-label="Änderungen speichern"><span class="grow">Ungespeicherte Änderungen</span><button type="button" class="btn ghost" data-act="aaAccDiscard" data-fid="la22:disc">Verwerfen</button><button type="button" class="btn primary" data-act="aaAccSave" data-fid="la22:save">Speichern</button></div>` : ''}`;
  }, { rows: 3, h: 90 });
  return aaView({ area: 'people', page: 'accounts', title: a ? a.email : 'Konto', back: ['#/admin/accounts', 'Konten'], intro: 'Zugriff und Rechte dieses Kontos.', html });
});
ACT.aaAccDiscard = () => { ui.draft.acc = null; toast('Änderungen verworfen.', 'ok'); render(); };
ACT.aaAccSave = () => {
  const d = ui.draft.acc; const a = LIVE.accounts.find((x) => x.email === d.email); const q = d.quota === '' || d.quota == null ? aaDefQ() : Number(d.quota);
  if (!(q >= 0)) { toast('Das Kontingent muss eine Zahl sein.', 'bad'); return; }
  a.quotaGB = q; a.rights = { liveTv: d.liveTv, recordings: d.liveTv && d.recordings, timeshift: d.liveTv && d.timeshift };
  if (a.email === LIVE.account.email) { LIVE.account.quotaGB = q; ['liveTv', 'recordings', 'timeshift'].forEach((k) => SIM.setRight('account', k, k === 'liveTv' ? d.liveTv : d.liveTv && d[k])); }
  d.wasLive = d.liveTv; AA.accSnap = aaAccKey(d); toast(`Rechte von ${esc(a.email)} gespeichert.${a.email === LIVE.account.email ? ' Die Zuschauer-App sieht sie sofort.' : ''}`, 'ok'); render();
};
ACT.aaInviteOpen = () => { ui.draft.inv = { email: '', liveTv: false, recordings: false, timeshift: false, quota: '', err: '' }; openDialog('aaInvite', {}); };
DIALOGS.aaInvite = () => { const d = ui.draft.inv; return { title: 'Einladung erstellen', wide: true, body: `<div class="stack">${aaF('inv-m', 'E-Mail-Adresse', aaI('ui.draft.inv.email', 'inv-m', { auto: true, err: d.err, ph: 'name@example.ch' }), { err: d.err })}<p class="muted">Vorgabe für neue Konten: alles aus. Kein Konto sieht Live-TV, bevor du es freischaltest.</p>${aaLiveBlock('ui.draft.inv', { id: 'inv' })}</div>`, actions: `<button type="button" class="btn secondary" data-act="close" data-fid="inv:x">Abbrechen</button><button type="button" class="btn primary" data-act="aaInviteOk" data-fid="inv:ok">Einladung erstellen</button>` }; };
ACT.aaInviteOk = () => { const d = ui.draft.inv; if (!/^\S+@\S+\.\S+$/.test(d.email.trim())) { d.err = 'Trage eine gültige E-Mail-Adresse ein.'; renderOverlays(); return; } AA.invites.push({ email: d.email.trim(), until: LIVE.at(7, '23:59'), live: d.liveTv }); closeDialog(); toast(`Einladung für ${esc(d.email.trim())} erstellt. Live-TV ${d.liveTv ? 'an' : 'aus'}; die Rechte gehen auf das neue Konto über.`, 'ok'); go('#/admin/accounts?tab=invites'); };

/* ---------- LA-23 Hintergrundjobs ---------- */
AA.jobLast = {}; AA.jobRun = {}; AA.jobPause = {};
function aaJobs() {
  const now = SIM.now(); const failed = LIVE.sources.filter((s) => !s.lastOk); const epgLast = Math.max(...LIVE.epgSources.map((e) => e.last)); const grid = Math.ceil((now + 0.01) / 15) * 15;
  const l = (id, d) => AA.jobLast[id] ?? d;
  return [
    { id: 'live.sources.refresh', name: 'Senderlisten aktualisieren', sched: 'alle 15 Min. prüfen, wählt fällige Quellen', last: l('live.sources.refresh', grid - 15), next: grid, res: failed.length ? ['warn', `${aaPl(failed.length, 'Quelle', 'Quellen')} fehlgeschlagen: ${failed.map((s) => esc(s.name)).join(', ')}`] : ['ok', 'in Ordnung'] },
    { id: 'live.epg.import', name: 'Programmdaten laden', sched: 'je Quelle (Intervall bei den EPG-Quellen)', last: l('live.epg.import', epgLast), next: Math.min(...LIVE.epgSources.map((e) => e.last + e.interval * 60)), res: LIVE.epgSources.some((e) => e.warnings.length) ? ['warn', `${LIVE.epgSources.reduce((n, e) => n + e.warnings.length, 0)} Warnung: 312 Sendungen ohne Zeitzone`] : ['ok', 'in Ordnung'] },
    { id: 'live.channels.check', name: 'Verfügbarkeit prüfen', sched: `alle ${LIVE.diagnostics.settings.checkEvery.replace('h', ' Std.')}`, last: l('live.channels.check', now - 140), next: now + 220, res: ['ok', `${admCounts().unavailable} Sender nicht verfügbar gemeldet`] },
    { id: 'live.dvr.reschedule', name: 'Aufnahmen neu planen', sched: 'nach jedem EPG-Import', last: l('live.dvr.reschedule', epgLast + 2), next: epgLast + 12 * 60 + 2, res: ['ok', '2 Aufnahmen verschoben'] },
    { id: 'live.cleanup', name: 'Aufräumen', sched: 'täglich 03:00: abgelaufene Aufnahmen, «behalte N», verwaiste Puffer', last: l('live.cleanup', LIVE.at(0, '03:00')), next: LIVE.at(1, '03:00'), res: ['ok', 'in Ordnung'] },
  ];
}
function aaJobRow(j, live) {
  const run = AA.jobRun[j.id]; const pa = AA.jobPause[j.id];
  return `<tr><td class="main" data-l="Job">${j.name}<small class="mono aa-sm">${j.id || ''}</small></td><td data-l="Zeitplan">${esc(j.sched)}</td><td data-l="Letzter Lauf">${j.last != null ? aaWhen(j.last) : 'nie'}</td><td data-l="Nächster">${pa ? '–' : j.next ? aaWhen(j.next) : '–'}</td>
    <td data-l="Ergebnis">${run ? `<span class="la02-run"><span class="spin"></span>läuft … ${run.p} %</span>` : pa ? '<span class="chip info">Pausiert</span>' : `<span class="la02-st">${aaDot(j.res[0])}${j.res[1]}</span>`}</td>
    <td class="end"><div class="ta"><button type="button" class="btn quiet sm" data-act="aaJobRun" data-id="${j.id}" data-fid="la23:run:${j.id}" ${run ? 'disabled' : ''}>Jetzt ausführen</button><button type="button" class="btn quiet sm" data-act="aaJobPause" data-id="${j.id}" data-fid="la23:pa:${j.id}">${pa ? 'Fortsetzen' : 'Pausieren'}</button></div></td></tr>`;
}
function aaJobsView() {
  const live = aaEmpty() ? [] : aaJobs(); const warn = live.filter((j) => j.res[0] !== 'ok' && !AA.jobPause[j.id]).length; const th = '<thead><tr><th>Job</th><th>Zeitplan</th><th>Letzter Lauf</th><th>Nächster</th><th>Ergebnis</th><th class="end"><span class="vh">Aktionen</span></th></tr></thead>';
  const lib = [{ id: 'library.scan', name: 'Bibliothek scannen', sched: 'alle 6 Std.', last: SIM.now() - 95, next: SIM.now() + 265, res: ['ok', 'in Ordnung'] }, { id: 'metadata.refresh', name: 'Metadaten aktualisieren', sched: 'nachts 02:30', last: LIVE.at(0, '02:30'), next: LIVE.at(1, '02:30'), res: ['ok', 'in Ordnung'] }];
  return `${band('la23:live', 'Live-TV', `${aaPl(live.length, 'Jobart', 'Jobarten')}${warn ? ` · ${warn} mit Warnung` : ''}`, `<div class="tblw aa-tbl">${live.length ? `<table class="tbl">${th}<tbody>${live.map((j) => aaJobRow(j, true)).join('')}</tbody></table>` : '<p class="muted aa-pad">Noch keine Quelle, deshalb laufen keine Live-TV-Jobs.</p>'}</div><p class="muted aa-small">Pausieren einer Jobart beendet nie eine laufende Aufnahme. Laufende Aufnahmen und Empfang sind keine Jobs und erscheinen hier nicht.</p>`, { icon: 'live', open: warn > 0 })}
    ${band('la23:lib', 'Bibliothek', `${lib.length} Jobarten (unverändert)`, `<div class="tblw aa-tbl"><table class="tbl">${th}<tbody>${lib.map((j) => aaJobRow(j, false)).join('')}</tbody></table></div>`, { icon: 'film' })}`;
}
ACT.aaJobRun = (d) => {
  if (AA.jobRun[d.id]) return; AA.jobRun[d.id] = { p: 0 }; render();
  const step = () => { const r = AA.jobRun[d.id]; if (!r) return; r.p = Math.min(100, r.p + 25); if (r.p < 100) { render(); setTimeout(step, 450); return; } delete AA.jobRun[d.id]; AA.jobLast[d.id] = SIM.now(); toast('Job «' + esc(d.id) + '» ist durchgelaufen.', 'ok'); render(); };
  setTimeout(step, 450);
};
ACT.aaJobPause = (d) => { AA.jobPause[d.id] = !AA.jobPause[d.id]; toast(AA.jobPause[d.id] ? `«${esc(d.id)}» pausiert. Laufende Aufnahmen laufen weiter.` : `«${esc(d.id)}» läuft wieder nach Zeitplan.`, 'ok'); render(); };
route('/admin/jobs', () => aaView({ area: 'automation', page: 'jobs', title: 'Hintergrundjobs', intro: 'Aufgaben, die der Server von selbst erledigt. Neu: die Gruppe «Live-TV».', live: aaMode() === 'normal' ? admLiveText() : null, html: aaGate(null, aaJobsView, { rows: 3, h: 56 }) }));

/* ---------- LA-24 Benachrichtigungen der Verwaltung ---------- */
const AA_NT = [
  { id: 'lib', t: 'Bibliothek: Scan fehlgeschlagen', old: true }, { id: 'upd', t: 'Server: Update fehlgeschlagen', old: true },
  { id: 'out', t: 'Live-TV: Sender länger ausgefallen', th: [5, 15, 30, 60], thl: 'Schwelle', unit: 'Min.', def: LIVE.diagnostics.settings.notifyAfter },
  { id: 'imp', t: 'Live-TV: Import einer Senderliste fehlgeschlagen' }, { id: 'epg', t: 'Live-TV: Programmdaten veraltet', th: [24, 48, 72], thl: 'älter als', unit: 'Std.', def: 48 }, { id: 'sto', t: 'Live-TV: Speicherort voll oder nicht eingehängt' },
];
function aaNtInit() { const o = {}; AA_NT.forEach((n) => { o[n.id] = { on: true, th: n.def }; }); return o; }
function aaNtView() {
  const d = ui.draft.nt; const dirty = JSON.stringify(d) !== AA.ntSnap;
  return `<p class="muted">Diese Ereignisse gehen über «Fehler» an alle Administratoren. Abgelehnte Aufnahmen einzelner Konten gehen nicht an dich, sondern an das Profil und das Hauptprofil.</p>
    <section class="card"><div class="ch"><span class="ik">${icon('bell', 18)}</span><h2>Fehlerquellen</h2></div>${AA_NT.map((n) => `<div class="srow${n.old ? '' : ' new'}"><div class="st"><b>${n.t}</b><span>${n.th ? `${n.thl} ${d[n.id].th} ${n.unit}` : n.old ? 'bestehend' : 'neu für Live-TV'}</span></div><div class="sc">${sw(`ui.draft.nt.${n.id}.on`, n.t)}<span>${d[n.id].on ? 'melden' : 'nicht melden'}</span>${n.th && d[n.id].on ? `<label class="vh" for="nt-${n.id}">${n.thl}</label>${selectEl(`ui.draft.nt.${n.id}.th`, n.th.map((v) => ({ v, t: `${n.thl} ${v} ${n.unit}` })), { label: n.thl }).replace('<select ', `<select id="nt-${n.id}" data-num `)}` : ''}</div></div>`).join('')}</section>
    ${dirty ? `<div class="savebar" role="region" aria-label="Änderungen speichern"><span class="grow">Ungespeicherte Änderungen</span><button type="button" class="btn ghost" data-act="aaNtDisc" data-fid="la24:disc">Verwerfen</button><button type="button" class="btn primary" data-act="aaNtSave" data-fid="la24:save">Speichern</button></div>` : ''}`;
}
ACT.aaNtDisc = () => { ui.draft.nt = JSON.parse(AA.ntSnap); toast('Änderungen verworfen.', 'ok'); render(); };
ACT.aaNtSave = () => { AA.ntSnap = JSON.stringify(ui.draft.nt); toast('Einstellungen gespeichert.', 'ok'); render(); };
route('/admin/notifications', () => {
  if (parseHash().path !== lastPath || !ui.draft.nt) { ui.draft.nt = aaNtInit(); AA.ntSnap = JSON.stringify(ui.draft.nt); }
  return aaView({ area: 'automation', page: 'notifications', title: 'Benachrichtigungen', intro: 'Wann du als Administrator eine Meldung bekommst. Neu: vier Fehlerquellen für Live-TV.', html: aaGate(null, aaNtView, { rows: 4, h: 64 }) });
});

/* ---------- LA-26 Meine Medien ---------- */
AA.media = LIBS.map(([l, ic]) => ({ k: l, ic, on: true }));
AA.media.splice(AA.media.findIndex((m) => m.k === 'Radio') + 1, 0, { k: 'Live-TV', ic: 'live', on: true, isNew: true });
function aaMediaView() {
  const m = AA.media;
  return `<p class="muted">Ordne die Einträge der Navigation und blende nicht benötigte aus. Ausblenden entfernt nur den Eintrag aus der Navigation. Wer Live-TV darf, regeln die Konten.</p>
    <section class="card">${m.map((x, i) => `<div class="srow"><div class="st"><b>${icon(x.ic, 16)} ${x.k}${x.isNew ? ' <span class="newtag">NEU</span>' : ''}</b><span>${x.on ? 'sichtbar' : 'ausgeblendet'}</span></div><div class="sc">${aaSw(x.on, 'aaMdVis', { k: x.k }, `${x.k} sichtbar`, 'la26:v:' + x.k)}<button type="button" class="btn quiet icon sm" data-act="aaMdMove" data-k="${x.k}" data-dir="-1" ${i === 0 ? 'disabled' : ''} aria-label="${x.k} nach oben" data-fid="la26:u:${x.k}">${icon('arrowUp', 16)}</button><button type="button" class="btn quiet icon sm" data-act="aaMdMove" data-k="${x.k}" data-dir="1" ${i === m.length - 1 ? 'disabled' : ''} aria-label="${x.k} nach unten" data-fid="la26:d:${x.k}">${icon('arrowDown', 16)}</button></div></div>`).join('')}</section>`;
}
ACT.aaMdVis = (d) => { const x = AA.media.find((m) => m.k === d.k); x.on = !x.on; toast(`«${x.k}» ist ${x.on ? 'sichtbar' : 'in der Navigation ausgeblendet'}.`, 'ok'); render(); };
ACT.aaMdMove = (d) => { const a = AA.media; const i = a.findIndex((m) => m.k === d.k); const j = i + Number(d.dir); if (j < 0 || j >= a.length) return; [a[i], a[j]] = [a[j], a[i]]; render(); setTimeout(() => { const el = document.querySelector(`[data-fid="la26:${d.dir < 0 ? 'u' : 'd'}:${d.k}"]`); if (el && !el.disabled) el.focus(); }, 0); };
route('/admin/my-media', () => aaView({ area: 'home', page: 'my-media', title: 'Meine Medien', intro: 'Die Einträge der Navigation: Reihenfolge und Sichtbarkeit. Neu: «Live-TV».', html: aaGate(null, aaMediaView, { rows: 5, h: 56 }) }));

/* ---------- LA-27 Server › Update ---------- */
AA.ver = { cur: '1.45.0', next: '1.46.0', done: false };
function aaUpdSoon() { return { run: admRecRunning(), soon: admRecSoon(30) }; }
DIALOGS.aaUpdate = () => {
  const u = aaUpdSoon(); const any = u.run + u.soon > 0;
  const w = any ? notice('warn', `${aaPl(u.run, 'Aufnahme läuft', 'Aufnahmen laufen')}${u.soon ? `, ${u.soon === 1 ? '1 beginnt' : u.soon + ' beginnen'} in den nächsten 30 Minuten` : ''}.`, 'Ein Update unterbricht sie (Lücke von etwa einer Minute). Hier stehen nur Zahlen, keine Titel.') : '';
  return { title: `Auf Version ${AA.ver.next} aktualisieren?`, body: `<p class="lead">Der Server startet dabei neu. Aktive Wiedergaben enden kurz.</p>${w}`, actions: any ? `<button type="button" class="btn secondary" data-act="close" data-fid="upd:later" data-autofocus>Später</button><button type="button" class="btn primary" data-act="aaUpdOk" data-fid="upd:ok">Trotzdem aktualisieren</button>` : `<button type="button" class="btn secondary" data-act="close" data-fid="upd:x">Abbrechen</button><button type="button" class="btn primary" data-act="aaUpdOk" data-fid="upd:ok" data-autofocus>Aktualisieren</button>` };
};
ACT.aaUpdOk = () => { closeDialog(); toast('Update läuft (Mockup, nichts wird installiert) …', 'ok', 2500); setTimeout(() => { AA.ver.cur = AA.ver.next; AA.ver.done = true; toast(`Version ${AA.ver.cur} läuft. Das Update ist simuliert.`, 'ok'); render(); }, 2200); };
ACT.aaUpdOpen = () => openDialog('aaUpdate', {});
route('/admin/server', () => aaView({ area: 'system', page: 'server', title: 'Server', intro: 'Version, Update und Betrieb.', html: aaGate(null, () => `<section class="card"><div class="ch"><span class="ik">${icon('server', 18)}</span><h2>Version</h2></div><dl class="kv"><dt>Läuft</dt><dd>${AA.ver.cur}</dd><dt>Verfügbar</dt><dd>${AA.ver.done ? 'nichts Neues' : AA.ver.next}</dd><dt>Betriebszeit</dt><dd>12 Tage 4 Std.</dd></dl>${AA.ver.done ? '<p class="muted">Der Server ist aktuell.</p>' : `<div class="foot"><button type="button" class="btn primary" data-act="aaUpdOpen" data-fid="la27:upd">${icon('dl', 16)}Update installieren</button></div>`}</section><p class="muted aa-small">Vor dem Update warnt der Dialog, wenn Aufnahmen laufen oder gleich beginnen.</p>`, { rows: 2, h: 120 }) }));

/* ---------- LA-28 Sicherung ---------- */
AA.bk = { last: LIVE.at(0, '03:30'), busy: false, hist: [[LIVE.at(0, '03:30'), '38 MB'], [LIVE.at(-1, '03:30'), '38 MB'], [LIVE.at(-2, '03:30'), '37 MB']] };
function aaBackupView() {
  const e = aaEmpty(); const c = admCounts(); const rules = LIVE.profiles.reduce((a, p) => a + SIM.rules(p.id).length, 0);
  const inc = [['Quellen', LIVE.sources.length], ['Sender', c.active], ['Zuordnungen (EPG)', LIVE.mappingCounts.all], ['Planungen (Aufnahmen)', rules]];
  return `${notice('', 'Aufnahmen und Puffer sind nicht Teil der Sicherung.', 'Gesichert werden Quellen, Sender, Zuordnungen und Planungen.')}
    <section class="card"><div class="ch"><span class="ik">${icon('database', 18)}</span><h2>Sicherung</h2><button type="button" class="btn primary sm" data-act="aaBackup" data-fid="la28:go" ${AA.bk.busy ? 'disabled' : ''}>${AA.bk.busy ? '<span class="spin"></span>' : ''}Jetzt sichern</button></div>
      <p>Letzte Sicherung: <b>${aaWhen(AA.bk.last)}</b> · täglich 03:30</p>
      <h3 class="la02-h3">Live-TV in der Sicherung</h3><ul class="aa-inc">${inc.map(([t, n]) => `<li>${icon('check', 14)}<span>${t}</span><b>${e ? '–' : fmt.num(n)}</b></li>`).join('')}</ul>
      <h3 class="la02-h3">Nicht gesichert</h3><ul class="aa-inc">${LIVE.storage.filter((s) => s.id !== 'timeshift').map((s) => `<li>${icon('x', 14)}<span>${s.id === 'recordings' ? 'Aufnahmen' : 'Startpuffer und Timeshift'}</span><b>${e ? '–' : fmt.gb(s.id === 'recordings' ? s.usedGB : s.usedGB + LIVE.storage.find((x) => x.id === 'timeshift').usedGB)}</b></li>`).join('')}</ul></section>
    <section class="card"><div class="ch"><span class="ik">${icon('clock', 18)}</span><h2>Frühere Sicherungen</h2></div><ul class="feed">${AA.bk.hist.map(([t, s]) => `<li><time>${fmt.day(t)} ${fmt.time(t)}</time><span>${s} · in Ordnung</span></li>`).join('')}</ul></section>`;
}
ACT.aaBackup = () => { if (AA.bk.busy) return; AA.bk.busy = true; render(); setTimeout(() => { AA.bk.busy = false; AA.bk.last = SIM.now(); AA.bk.hist.unshift([SIM.now(), '38 MB']); toast('Sicherung abgeschlossen (Mockup).', 'ok'); render(); }, 1500); };
route('/admin/backup', () => aaView({ area: 'system', page: 'backup', title: 'Sicherung', intro: 'Sichert die Einstellungen und Daten deines Servers.', html: aaGate(null, aaBackupView, { rows: 3, h: 110 }) }));
