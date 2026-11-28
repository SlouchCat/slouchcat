/* LA-02 Quellen (administration, sample page of the core): list of sources with state, expandable rows,
   «Jetzt aktualisieren» with progress, «Testen», menu (Bearbeiten, Letzter Import, Entfernen with choice),
   responsibility notes V-1/V-2, every state. 15-administration.md 3.2. Admin pages change LIVE.sources
   directly (shared with LA-01, LA-03, LA-04). */
'use strict';

const LA02 = { busy: {}, tested: {} };
const la02Next = (s) => (s.interval ? fmt.when(s.next).replace(/^Heute/, 'heute').replace(/^Morgen/, 'morgen') : 'nur von Hand');
function la02Last(s) {
  const b = LA02.busy[s.id];
  if (b) return `<span class="la02-run"><span class="spin"></span>läuft … ${fmt.num(b.n)} von ${fmt.num(b.of)} Einträgen</span>`;
  if (!s.last) return '<span class="muted">noch nie</span>';
  return `<span class="la02-st"><span class="dot ${s.lastOk ? 'ok' : 'bad'}"></span>${fmt.dayRel(s.last).replace('Heute', 'heute').replace('Gestern', 'gestern')} ${fmt.time(s.last)} · ${s.lastOk ? 'in Ordnung' : 'fehlgeschlagen'}</span>${!s.lastOk && s.lastError ? `<small class="la02-err">${esc(s.lastError)}. Die bisherige Liste bleibt aktiv.</small>` : ''}`;
}
/* addresses only as scheme · host · port; credentials only «gesetzt» (15 Fassung 2, Grundsatz 5) */
function la02Shp(u) { const m = String(u || '').match(/^([a-z0-9]+):\/\/@?([^/:?]+)(?::(\d+))?/i); if (!m) return esc(u); const port = m[3] || { https: '443', http: '80' }[m[1].toLowerCase()] || ''; return `${esc(m[1].toLowerCase())} · ${esc(m[2])}${port ? ' · ' + port : ''}`; }
const la02Targets = (s) => (s.targets ? s.targets.length : s.fixedTargets || 0);
function la02Address(s) {
  if (s.kind === 'xtream') return `<span class="mono">${la02Shp(s.address)}</span> · Benutzer und Passwort gesetzt`;
  if (s.kind === 'm3u-file') return `Datei <span class="mono">${esc(s.file)}</span> (${esc(s.fileSize)}, hochgeladen ${fmt.day(s.uploaded)})`;
  if (s.kind === 'single') return s.streams.map((u) => `<span class="mono">${la02Shp(u)}</span>`).join(', ');
  return `<span class="mono">${la02Shp(s.address)}</span>`;
}
function la02Detail(s) {
  const runs = LIVE.importRuns[s.id] || [];
  const res = { ok: ['ok', 'in Ordnung'], failed: ['bad', 'fehlgeschlagen'], incomplete: ['warn', 'unvollständig, nicht übernommen'] };
  return `<div class="la02-det">
    <dl class="kv"><dt>Adresse</dt><dd>${la02Address(s)}</dd>
    <dt>Aktualisieren</dt><dd>${s.interval ? `alle ${s.interval} Stunden` : 'nur von Hand'}</dd>
    <dt>Neue Sender sofort anbieten</dt><dd>${s.offerNew ? 'an' : 'aus (Werksvorgabe an; neue Sender kommen inaktiv mit Filter «Neu»)'}</dd>
    <dt>Vorladen für Sender dieser Quelle</dt><dd>${s.moreConn ? 'an' : 'aus'}</dd>
    <dt>Gruppen</dt><dd>${s.groups}</dd><dt>Verschwundene Sender</dt><dd>${s.gone}</dd>
    <dt>Feste Ziele im Heimnetz</dt><dd>${la02Targets(s) ? `${la02Targets(s)} (bestätigt, protokolliert)` : 'keine'}</dd></dl>
    <h3 class="la02-h3">Letzte Läufe</h3>
    ${runs.length ? `<ul class="feed">${runs.map(([id, t, r, n, c, g, note]) => `<li><time>${fmt.day(t)} ${fmt.time(t)}</time><span><span class="dot ${res[r][0]}"></span> ${res[r][1]}${r === 'ok' ? ` · ${n} neu · ${c} geändert · ${g} verschwunden` : ''}${note ? ` · ${esc(note)}` : ''} · <a class="lnk" href="#/admin/live/sources/${s.id}/import/${id}" data-fid="la02:run:${id}">Details</a></span></li>`).join('')}</ul>` : '<p class="muted">Noch keine Läufe. Einzelströme werden nicht importiert.</p>'}
  </div>`;
}
function la02Row(s) {
  const open = isOpen('la02:' + s.id);
  const menu = popMenu('la02:' + s.id, icon('dots', 16), [
    { label: 'Bearbeiten', href: `#/admin/live/sources/${s.id}`, icon: 'pen' },
    { label: 'Letzter Import', href: (LIVE.importRuns[s.id] || [])[0] ? `#/admin/live/sources/${s.id}/import/${LIVE.importRuns[s.id][0][0]}` : `#/admin/live/sources/${s.id}`, icon: 'list' },
    { label: 'Entfernen', act: 'la02Remove', data: { id: s.id }, danger: true, icon: 'trash' },
  ], { aria: `Weitere Aktionen für ${s.name}`, cls: 'quiet icon' });
  return `<tr class="${open ? 'open' : ''}"><td class="main"><span class="la02-name"><button type="button" class="tog" data-act="open" data-id="la02:${s.id}" aria-expanded="${open}" aria-label="${esc(s.name)}: Einzelheiten ${open ? 'zuklappen' : 'aufklappen'}" data-fid="la02:tog:${s.id}">${icon('chevR', 14)}</button><a class="lnk" href="#/admin/live/sources/${s.id}" data-fid="la02:name:${s.id}"><b>${esc(s.name)}</b></a></span></td>
    <td data-l="Art">${LU.SOURCE_KIND[s.kind]}</td>
    <td data-l="Sender">${fmt.num(s.active)} aktiv / ${fmt.num(s.total)}</td>
    <td data-l="Letzte Aktualisierung">${la02Last(s)}</td>
    <td data-l="Nächste">${la02Next(s)}</td>
    <td data-l="Feste Ziele">${la02Targets(s) ? `<span class="chip blue">${icon('home2', 14)}${la02Targets(s)} feste Ziele im Heimnetz</span>` : '<span class="muted">–</span>'}</td>
    <td class="end"><div class="ta">${s.kind === 'single' ? '' : `<button type="button" class="btn quiet" data-act="la02Refresh" data-id="${s.id}" data-fid="la02:ref:${s.id}" ${LA02.busy[s.id] ? 'disabled' : ''}>${icon('refresh', 16)}Jetzt aktualisieren</button>`}<button type="button" class="btn quiet" data-act="la02Test" data-id="${s.id}" data-fid="la02:test:${s.id}">Testen</button>${menu}</div></td></tr>
    ${open ? `<tr class="sub"><td colspan="7">${la02Detail(s)}</td></tr>` : ''}`;
}

route('/admin/live/sources', () => {
  const mode = liveMode(); const list = LIVE.sources;
  const v1 = 'Du bist dafür verantwortlich, dass du die eingetragenen Quellen nutzen und im Haushalt verteilen darfst. Slouchcat prüft keine Nutzungsrechte.';
  const v2 = 'Slouchcat zählt keine Verbindungen je Anbieter und kennt keine Abos. Wie viele gleichzeitige Verbindungen dein Anbieter erlaubt, musst du selbst im Blick behalten.';
  const resp = band('la02:resp', 'Verantwortung für Quellen', 'Nutzungsrechte · keine Verbindungszählung', `<p>${v1}</p><p>${v2}</p>`, { icon: 'info' });
  let html;
  if (mode === 'loading') html = skelRows(4, 56);
  else if (mode === 'error') html = notice('bad', 'Die Quellen konnten nicht geladen werden.', 'Der Server hat nicht geantwortet.', `<button type="button" class="btn secondary" data-act="liveRetry" data-fid="la02:retry">${icon('refresh', 16)}Erneut laden</button>`);
  else if (mode === 'empty' || !list.length) html = `<div class="unchanged la02-empty"><b>Noch keine Quelle</b><p>Füge eine Senderliste (M3U), einen Xtream-Zugang oder einen einzelnen Stream hinzu.</p><div class="row-gap"><a class="btn primary" href="#/admin/live/sources/new" data-fid="la02:add2">${icon('plus', 16)}Quelle hinzufügen</a></div></div>`;
  else {
    const failed = list.filter((s) => !s.lastOk);
    html = (failed.length ? notice('warn', `Import «${esc(failed[0].name)}» fehlgeschlagen (${esc(failed[0].lastError)}).`, 'Die bisherige Liste bleibt aktiv. Nächster Versuch ' + la02Next(failed[0]) + '.', `<a class="btn secondary" href="#/admin/live/sources/${failed[0].id}/import/${LIVE.importRuns[failed[0].id][0][0]}" data-fid="la02:failrun">Letzten Import ansehen</a>`) : '')
      + `<div class="tblw la02"><table class="tbl"><thead><tr><th>Name</th><th>Art</th><th>Sender</th><th>Letzte Aktualisierung</th><th>Nächste</th><th>Feste Ziele</th><th class="end"><span class="vh">Aktionen</span></th></tr></thead><tbody>${list.map(la02Row).join('')}</tbody></table></div>
      <p class="muted la02-foot">${fmt.num(list.reduce((a, s) => a + s.active, 0))} aktive Sender aus ${list.length} Quellen. Eigene Namen, Logos, Gruppen, Nummern, Favoriten, Zuordnungen und geplante Aufnahmen bleiben bei jeder Aktualisierung erhalten.</p>`;
  }
  return { type: 'admin', area: 'livetv', page: 'sources', title: 'Quellen', intro: 'Senderlisten und Streams, aus denen Slouchcat die Sender bildet. Nur der Server-Admin trägt Quellen ein.',
    actions: `<a class="btn primary" href="#/admin/live/sources/new" data-fid="la02:add">${icon('plus', 16)}Quelle hinzufügen</a>`, html: resp + html };
});

ACT.la02Refresh = (d) => {
  const s = LIVE.sources.find((x) => x.id === d.id); if (!s || LA02.busy[s.id]) return;
  const b = LA02.busy[s.id] = { n: 0, of: s.kind === 'xtream' ? 9874 : s.total };
  render();
  const step = () => {
    if (!LA02.busy[s.id]) return;
    b.n = Math.min(b.of, b.n + Math.ceil(b.of / 6));
    if (b.n < b.of) { render(); setTimeout(step, 450); return; }
    delete LA02.busy[s.id];
    const t = SIM.now(); s.last = t; s.next = s.interval ? t + s.interval * 60 : null;
    if (s.kind === 'xtream') { s.lastOk = true; s.lastError = null; s.state = 'ok'; }
    (LIVE.importRuns[s.id] || (LIVE.importRuns[s.id] = [])).unshift([`run-${s.id}-${Math.round(t)}`, t, 'ok', 3, 12, 1, '']);
    LIVE.importRuns[s.id] = LIVE.importRuns[s.id].slice(0, 5);
    render();
    toast(`«${esc(s.name)}» aktualisiert: 3 neu, 12 geändert, 1 verschwunden · <a href="#/admin/live/sources/${s.id}/import/${LIVE.importRuns[s.id][0][0]}">Details</a>`, 'ok', 7000);
  };
  setTimeout(step, 450);
};
ACT.la02Test = (d) => openDialog('la02Test', { id: d.id });
DIALOGS.la02Test = (p) => {
  const s = LIVE.sources.find((x) => x.id === p.id);
  const ok = s.kind !== 'xtream' || s.lastOk;
  const rows = ok
    ? [['ok', 'Adresse erreichbar', `Antwort in ${s.localNet ? 12 : 340} ms`], ['ok', s.kind === 'single' ? 'Stream liefert Bild und Ton' : `Liste gelesen: ${fmt.num(s.kind === 'xtream' ? 9874 : s.total)} Einträge`, s.kind === 'single' ? 'H.264 · 1920 × 1080 · AAC Stereo' : `${s.groups} Gruppen`], ['ok', 'Kein Kopierschutz erkannt', 'Kopiergeschützte Streams spielt Slouchcat nicht ab und umgeht keinen Schutz.']]
    : [['bad', 'Server antwortet 503', `Host ${esc(s.host)} · Dienst nicht verfügbar`], ['wait', 'Liste nicht gelesen', 'Die bisherige Liste bleibt aktiv.']];
  return { title: `Quelle testen: ${esc(s.name)}`, body: `<p class="lead">Slouchcat hat die Quelle eben einmal abgefragt. Adressen erscheinen ohne Zugangsdaten.</p><ul class="checks">${rows.map(([k, t, x]) => `<li class="${k}">${icon(k === 'ok' ? 'check' : k === 'bad' ? 'x' : 'clock', 18)}<span><b>${t}</b><small>${x}</small></span></li>`).join('')}</ul>`,
    actions: `<button type="button" class="btn secondary" data-act="close" data-fid="la02t:x" data-autofocus>Schliessen</button>` };
};
ACT.la02Remove = (d) => { ui.draft.la02rm = 'keep'; openDialog('la02Remove', { id: d.id }); };
DIALOGS.la02Remove = (p) => {
  const s = LIVE.sources.find((x) => x.id === p.id); const only = Math.round(s.active * 0.64);
  const r = (v, t) => `<label class="check"><input type="radio" name="la02rm" value="${v}" data-bind="ui.draft.la02rm" ${ui.draft.la02rm === v ? 'checked' : ''} data-fid="la02rm:${v}">${t}</label>`;
  return { title: `Quelle «${esc(s.name)}» entfernen?`, body: `<p class="lead">${s.active} Sender stammen aus dieser Quelle; ${only} davon haben keine andere Quelle.</p>
    <div class="stack" style="gap:0">${r('keep', `Diese ${only} Sender behalten, als «ohne Quelle» markieren und abschalten`)}${r('delete', `Diese ${only} Sender löschen`)}</div>
    <p class="muted">Favoriten, Zuordnungen und geplante Aufnahmen bleiben bei behaltenen Sendern erhalten. Geplante Aufnahmen gelöschter Sender werden abgesagt; die Profile bekommen eine Warnung.</p>`,
    actions: `<button type="button" class="btn secondary" data-act="close" data-fid="la02rm:x" data-autofocus>Abbrechen</button><button type="button" class="btn danger" data-act="la02RemoveOk" data-id="${s.id}" data-fid="la02rm:ok">${icon('trash', 16)}Entfernen</button>` };
};
ACT.la02RemoveOk = (d) => {
  const k = LIVE.sources.findIndex((x) => x.id === d.id); if (k < 0) return;
  const [s] = LIVE.sources.splice(k, 1); closeDialog(); render();
  toast(`Quelle «${esc(s.name)}» entfernt. ${ui.draft.la02rm === 'delete' ? 'Ihre Sender ohne andere Quelle wurden gelöscht.' : 'Ihre Sender ohne andere Quelle sind abgeschaltet.'}`, 'ok', 6000);
};
