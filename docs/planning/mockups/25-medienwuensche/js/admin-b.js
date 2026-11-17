/* M-23 to M-28: administration pages «Wünsche» (Anfragen, Zuordnung, Einstellungen), «Konten», «Spotlight»,
   «Startseite». Everything here only changes the mockup state in this browser. */
'use strict';

/* ---------- shared helpers ---------- */
function abMini(t, sub) {
  return `<span class="mini"><span class="po">${img(t.poster, t.title)}</span><span class="ab-tx"><b>${esc(t.title)}</b><span>${esc(sub)}</span></span></span>`;
}
function abUserName(id) { const u = seerrUser(id); return u ? u.name : 'unbekannt'; }
/* which account owns a Jellyseerr user (account mapping or a profile mapping of that account) */
function abUserOwner(uid) {
  const n = Number(uid);
  for (const [a, id] of Object.entries(S.map.accounts)) if (Number(id) === n && account(a)) return a;
  for (const [pid, id] of Object.entries(S.map.profiles)) { const p = profile(pid); if (Number(id) === n && p) return p.accountId; }
  return null;
}
function abWishSub(w) {
  const t = T[w.titleKey];
  if (w.media === 'movie') return `Film · ${t.year}`;
  return 'Serie · ' + (w.seasons.length ? (w.seasons.length === 1 ? 'Staffel ' : 'Staffeln ') + w.seasons.join(', ') : 'alle Staffeln');
}
function abCount(n, one, other) { return `${n} ${n === 1 ? one : other}`; }
const AB_RATINGS = [0, 6, 12, 16, 18].map((n) => ({ v: n, t: ageLabel(n) }));

/* sub tabs shared by the three wish pages */
function wishTabs(active) {
  const wc = wishCounts(), mc = mappingCounts();
  const tab = (k, href, label, extra) => `<a href="${href}" ${k === active ? 'aria-current="page"' : ''} data-fid="wt:${k}">${label}${extra || ''}</a>`;
  return `<nav class="subtabs ab-tabs" aria-label="Bereiche der Wünsche">${tab('requests', '#/admin/wishes', 'Anfragen', `<span class="c ${wc.pending ? 'hot' : ''}" title="${wc.pending ? wc.pending + ' warten auf Freigabe' : 'offen'}">${wc.open}</span>`)}${tab('mapping', '#/admin/wishes/mapping', 'Zuordnung', mc.missing ? `<span class="c hot">${mc.missing} fehlen</span>` : '')}${tab('settings', '#/admin/wishes/settings', 'Einstellungen')}</nav>`;
}
function abWishNotices(skipOff) {
  const ss = seerrState();
  let h = '';
  if (ss.k !== 'ok') {
    const txt = ss.k === 'none' || ss.k === 'off' ? 'Ohne Verbindung zu Jellyseerr kann niemand Titel anfordern. Bestehende Wünsche bleiben gespeichert.' : `Stand vom letzten Abgleich um ${fmtTime(S.seerr.lastSync)}. Neue Wünsche werden vorgemerkt und übermittelt, sobald Jellyseerr wieder antwortet.`;
    h += notice(ss.chip === 'bad' ? 'bad' : 'warn', `Jellyseerr: ${ss.label}`, txt, `<a class="btn sm secondary" href="#/admin/integrations/jellyseerr" data-fid="wn:conn">${icon('plug', 16)}Zur Verbindung</a>`);
  }
  if (!S.settings.wishes.enabled && !skipOff) h += notice('warn', 'Wünsche sind ausgeschaltet', 'Kein Profil sieht fehlende Titel oder kann anfordern. Bestehende Wünsche laufen in Jellyseerr weiter.', `<a class="btn sm secondary" href="#/admin/wishes/settings" data-fid="wn:set">${icon('settings', 16)}Zu den Einstellungen</a>`);
  return h;
}

/* ================= M-23 Wünsche › Anfragen ================= */
const AB_WF = [
  ['all', 'Alle', () => true],
  ['open', 'Offen', (w) => OPEN_STATES.includes(w.state)],
  ['queued', STATE_LABEL.queued, (w) => w.state === 'queued'],
  ['pending', STATE_LABEL.pending, (w) => w.state === 'pending'],
  ['processing', STATE_LABEL.processing, (w) => w.state === 'processing' || w.state === 'partial'],
  ['available', STATE_LABEL.available, (w) => w.state === 'available'],
  ['declined', STATE_LABEL.declined, (w) => w.state === 'declined'],
  ['failed', STATE_LABEL.failed, (w) => w.state === 'failed'],
  ['withdrawn', STATE_LABEL.withdrawn, (w) => w.state === 'withdrawn'],
  ['unassigned', 'Nicht zuordenbar', (w) => !w.owner],
];
const AB_WF_EMPTY = { all: 'Noch keine Wünsche.', open: 'Kein offener Wunsch.', unassigned: 'Keine Anfrage ohne Zuordnung. Alle Anfragen gehören zu einem Profil.' };

function abWishRow(w) {
  const t = T[w.titleKey], id = w.id, open = isOpen('aw:' + id);
  const p = w.owner ? profile(w.owner) : null;
  const joined = w.joined.map((j) => profile(j)).filter(Boolean);
  const who = p
    ? `<span class="ab-who"><b>${esc(account(p.accountId).email)} · ${esc(p.name)}</b>${joined.length ? `<span>+ ${joined.map((x) => esc(x.name)).join(', ')}</span>` : ''}</span>`
    : `<span class="chip warn">${icon('alert', 12)}Nicht zuordenbar</span>`;
  const su = `<span class="ab-who"><b>${esc(abUserName(w.seerrUser))}</b>${w.origin === 'jellyseerr' ? '<span class="chip info">In Jellyseerr angelegt</span>' : ''}</span>`;
  const acts = [];
  if (w.state === 'pending') {
    acts.push(`<button type="button" class="btn sm primary" data-act="abApprove" data-id="${id}" data-fid="aw:ap:${id}">${icon('check', 16)}Freigeben</button>`);
    acts.push(`<button type="button" class="btn sm dq" data-act="abDeclineAsk" data-id="${id}" data-fid="aw:de:${id}">${icon('ban', 16)}Ablehnen</button>`);
  }
  if (w.state === 'queued' || w.state === 'failed') acts.push(`<button type="button" class="btn sm secondary" data-act="abRetry" data-id="${id}" data-fid="aw:re:${id}">${icon('refresh', 16)}Erneut übermitteln</button>`);
  acts.push(`<button type="button" class="btn sm quiet" data-act="dialog" data-name="abSeerrOpen" data-id="${id}" data-fid="aw:js:${id}">${icon('external', 16)}In Jellyseerr öffnen</button>`);
  acts.push(`<a class="btn sm quiet" href="${titleHref(t)}" data-fid="aw:ti:${id}">${icon('chevR', 16)}Zum Titel</a>`);
  let sub = '';
  if (open) {
    const steps = w.events.map((e, i) => {
      const last = i === w.events.length - 1;
      const c = last && ['failed', 'declined'].includes(w.state) ? 'bad' : last && OPEN_STATES.includes(w.state) ? 'now' : 'done';
      return `<li class="${c}">${esc(e.t)}<time>${fmtDate(e.at)}, ${fmtTime(e.at)}</time></li>`;
    }).join('');
    const kv = [
      ['Request-ID in Jellyseerr', w.seerrRequestId ? `<span class="mono">#${w.seerrRequestId}</span>` : 'Noch keine: nicht übermittelt'],
      ['Übermittelt als', `${esc(abUserName(w.seerrUser))} <span class="muted">(Header X-API-User)</span>`],
      ['TMDB-ID', `<span class="mono">${mediaOf(t)}/${t.tmdbId}</span>`],
      ['Herkunft', w.origin === 'jellyseerr' ? 'In Jellyseerr angelegt, beim Abgleich übernommen' : 'In Slouchcat angefordert'],
      ['Zustand', wsChip(w.state)],
    ];
    if (w.failReason) kv.push(['Fehlergrund', esc(FAIL_TEXT[w.failReason] || w.failReason)]);
    if (w.detail) kv.push(['Zusatz', esc(w.detail)]);
    if (joined.length) kv.push(['Mitwünscher', joined.map((x) => esc(x.name)).join(', ')]);
    const un = !w.owner ? notice('warn', 'Nicht zuordenbar', `Der Jellyseerr-Benutzer «${esc(abUserName(w.seerrUser))}» gehört zu keinem Konto. Die Anfrage erscheint deshalb bei keinem Profil, und niemand wird benachrichtigt. Ordne den Benutzer einem Konto zu, dann übernimmt der nächste Abgleich die Anfrage.`, `<a class="btn sm secondary" href="#/admin/wishes/mapping" data-fid="aw:map:${id}">${icon('users', 16)}Zuordnung öffnen</a>`) : '';
    sub = `<tr class="sub"><td colspan="6"><div class="ab-subg"><div><h3 class="ab-h3">Verlauf</h3><ol class="steps">${steps}</ol></div><div><h3 class="ab-h3">Diagnose</h3><dl class="kv">${kv.map(([k, v]) => `<dt>${k}</dt><dd>${v}</dd>`).join('')}</dl></div></div>${un}</td></tr>`;
  }
  return `<tr><td class="main"><button type="button" class="tog" data-act="open" data-id="aw:${id}" aria-expanded="${open}" data-fid="aw:tog:${id}"><span class="chev">${icon('chevR', 16)}</span>${abMini(t, abWishSub(w))}</button></td><td>${who}</td><td>${su}</td><td>${wsChip(w.state)}</td><td class="nowrap">${ago(w.created)}</td><td class="end"><span class="ta">${acts.join('')}</span></td></tr>${sub}`;
}

route('/admin/wishes', () => {
  const f = ui.filter.adminWishes || 'all';
  const wc = wishCounts();
  const st = (k, v, l, n, c) => `<button type="button" class="stat ab-stat ${v && c ? c : ''}" data-act="set" data-path="ui.filter.adminWishes" data-value="${k}" aria-pressed="${f === k}" data-fid="aws:${k}"><span class="v">${v}</span><span class="l">${l}</span>${n ? `<span class="n">${n}</span>` : ''}</button>`;
  const stats = `<div class="stats" style="--n:5">${st('open', wc.open, 'Offen', 'alle offenen Zustände')}${st('pending', wc.pending, 'Freigabe ausstehend', wc.pending ? 'wartet auf dich oder Jellyseerr' : '', 'warn')}${st('processing', wc.processing, 'In Beschaffung', '')}${st('available', wc.available30, 'Verfügbar', 'letzte 30 Tage')}${st('failed', wc.problems, 'Fehler', '', 'bad')}</div>`;
  const pills = AB_WF.map(([k, l, fn]) => {
    const n = S.wishes.filter(fn).length;
    if (!n && k !== f && !['all', 'open', 'unassigned'].includes(k)) return '';
    return `<button type="button" class="pill" data-act="set" data-path="ui.filter.adminWishes" data-value="${k}" aria-pressed="${f === k}" data-fid="awf:${k}">${l}<span class="n">${n}</span></button>`;
  }).join('');
  const fn = (AB_WF.find((x) => x[0] === f) || AB_WF[0])[2];
  const list = S.wishes.filter(fn).sort((a, b) => b.created - a.created);
  const label = (AB_WF.find((x) => x[0] === f) || AB_WF[0])[1];
  const empty = AB_WF_EMPTY[f] || `Kein Wunsch im Zustand «${label}».`;
  const table = list.length
    ? `<div class="tblw"><table class="tbl"><thead><tr><th>Titel</th><th>Gewünscht von</th><th>Jellyseerr-Benutzer</th><th>Zustand</th><th>Seit</th><th class="end">Aktionen</th></tr></thead><tbody>${list.map(abWishRow).join('')}</tbody></table></div>`
    : `<div class="empty">${empty}${f !== 'all' ? `<div class="row-gap" style="justify-content:center;margin-top:12px"><button type="button" class="btn sm secondary" data-act="set" data-path="ui.filter.adminWishes" data-value="all" data-fid="aw:all">Alle Wünsche zeigen</button></div>` : ''}</div>`;
  const html = `${wishTabs('requests')}${abWishNotices()}${stats}
    <div class="ab-bar"><div class="pills" role="group" aria-label="Nach Zustand filtern">${pills}</div><span class="muted ab-small">Letzter Abgleich ${ago(S.seerr.lastSync)} · ${abCount(list.length, 'Wunsch', 'Wünsche')}</span></div>
    ${table}`;
  return { type: 'admin', area: 'media', page: 'wishes', title: 'Wünsche', crumb: 'Wünsche › Anfragen', intro: 'Wünsche aller Profile und ihr Zustand in Jellyseerr: freigeben, ablehnen, erneut übermitteln oder nachsehen, was schiefging.', actions: `<button type="button" class="btn primary" data-act="abSync" data-fid="aw:sync">${icon('refresh', 16)}Jetzt abgleichen</button>`, html };
});
ACT.abSync = () => {
  const ok = syncNow();
  if (ok) toast(`Abgleich ausgeführt: ${S.wishes.filter((w) => OPEN_STATES.includes(w.state)).length} offene Anfragen geprüft.`, 'ok');
  else toast(`Abgleich nicht möglich: Jellyseerr ${esc(seerrState().label.toLowerCase())}.`, 'bad');
  render();
};
function abSeerrReady() {
  const ss = seerrState();
  if (ss.k === 'ok') return true;
  toast(`Geht nicht: Jellyseerr ist ${esc(ss.label.toLowerCase())}. Versuche es, sobald die Verbindung steht.`, 'bad');
  return false;
}
ACT.abApprove = (d) => {
  if (!abSeerrReady()) return;
  const w = wishById(d.id);
  approveWish(d.id);
  toast(`«${esc(T[w.titleKey].title)}» ist freigegeben und wird beschafft.`, 'ok');
  render();
};
ACT.abDeclineAsk = (d) => { if (abSeerrReady()) openDialog('abDecline', { id: d.id }); };
DIALOGS.abDecline = ({ id }) => {
  const w = wishById(id), t = T[w.titleKey];
  const people = [w.owner, ...w.joined].map((x) => profile(x)).filter(Boolean).map((p) => p.name);
  return {
    title: `«${esc(t.title)}» ablehnen?`,
    body: `<p class="lead">Jellyseerr kennt keinen Ablehnungsgrund: ${people.length ? `${esc(people.join(' und '))} ${people.length === 1 ? 'sieht' : 'sehen'} nur «Abgelehnt» und ${people.length === 1 ? 'bekommt' : 'bekommen'} eine Benachrichtigung.` : 'Die Anfrage gehört zu keinem Profil, niemand wird benachrichtigt.'}</p>`,
    actions: `<button type="button" class="btn ghost" data-act="close" data-fid="dd:x">Abbrechen</button><button type="button" class="btn danger" data-act="abDeclineOk" data-id="${id}" data-fid="dd:ok" data-autofocus>${icon('ban', 16)}Ablehnen</button>`,
  };
};
ACT.abDeclineOk = (d) => { const w = wishById(d.id); closeDialog(); declineWish(d.id); toast(`«${esc(T[w.titleKey].title)}» ist abgelehnt.`, 'ok'); render(); };
ACT.abRetry = (d) => {
  const w = wishById(d.id), t = T[w.titleKey];
  const was = w.state;
  const res = retryWish(d.id);
  if (was === 'queued') {
    if (w.state === 'queued') toast(`«${esc(t.title)}» bleibt vorgemerkt: ${esc(w.detail || 'Jellyseerr antwortet nicht')}.`, 'warn');
    else toast(`«${esc(t.title)}» ist übermittelt: ${STATE_LABEL[w.state]}.`, w.state === 'failed' ? 'bad' : 'ok');
  } else if (res && res !== w) ui.open['aw:' + res.id] = true;
  render();
};
DIALOGS.abSeerrOpen = ({ id }) => {
  const w = wishById(id);
  return {
    title: 'In Jellyseerr öffnen',
    body: `<p class="lead">Im Produkt öffnet sich hier <span class="mono">${esc(S.seerr.url)}/requests</span> in einem neuen Tab.</p><div class="facts"><div class="f">${icon('info', 16)}<span>Die Anfrage hat dort ${w.seerrRequestId ? `die Nummer <b>#${w.seerrRequestId}</b>` : 'noch keine Nummer, weil sie nicht übermittelt ist'}.</span></div></div>`,
    actions: `<button type="button" class="btn secondary" data-act="close" data-fid="so:x" data-autofocus>Schliessen</button>`,
  };
};

/* ================= M-24 Wünsche › Zuordnung ================= */
function abUserFacts(u) {
  if (!u) return '';
  const no = (ok) => (ok ? 'ok' : 'bad ab-no');
  return `<div class="ab-facts" aria-label="Rechte von ${esc(u.name)} in Jellyseerr"><span class="chip ${no(u.canMovie)}">${icon(u.canMovie ? 'check' : 'ban', 12)}Filme</span><span class="chip ${no(u.canShow)}">${icon(u.canShow ? 'check' : 'ban', 12)}Serien</span><span class="chip ${u.autoApprove ? 'ok' : 'warn'}">${icon(u.autoApprove ? 'truck' : 'hourglass', 12)}${u.autoApprove ? 'Auto-Freigabe' : 'Braucht Freigabe'}</span><span class="chip info">${u.quota ? `Kontingent ${u.quota.n} Titel je ${u.quota.days} Tage` : 'Ohne Kontingent'}</span><span class="muted ab-small">${esc(u.type)} · ${esc(u.email)}</span></div>`;
}
function abUserWarn(u) {
  if (!u) return '';
  if (u.gone) return notice('bad', `Benutzer «${esc(u.name)}» nicht gefunden`, 'Jellyseerr kennt diesen Benutzer nicht mehr. Neue Wünsche scheitern mit «Den zugeordneten Jellyseerr-Benutzer gibt es nicht mehr». Wähle einen anderen Benutzer.');
  let h = '';
  if (!u.canMovie && !u.canShow) h += notice('warn', `«${esc(u.name)}» darf in Jellyseerr nichts anfordern`, 'Jeder Wunsch scheitert. Gib dem Benutzer in Jellyseerr das Recht «Anfragen» oder wähle einen anderen.');
  else if (!u.canShow) h += notice('warn', `«${esc(u.name)}» darf keine Serien anfordern`, 'Serienwünsche scheitern mit «darf diese Art von Titel nicht anfordern». Filme gehen.');
  return h;
}
function abUserOptions(ownerAcc, current, first) {
  return [first, ...S.seerrUsers.map((u) => {
    const own = abUserOwner(u.id);
    const taken = own && own !== ownerAcc;
    const isCur = Number(current) === u.id;
    return { v: u.id, t: u.name + (u.gone ? ' (in Jellyseerr gelöscht)' : '') + (taken ? ` (belegt von ${account(own).email})` : ''), disabled: (taken || u.gone) && !isCur };
  })];
}
function abProfMap(acc, p) {
  const own = S.map.profiles[p.id];
  const accU = S.map.accounts[acc.id];
  const res = seerrUserFor(p);
  const u = own ? seerrUser(own) : null;
  const first = { v: '', t: `Wie Konto (${accU ? abUserName(accU) : 'nicht zugeordnet'})` };
  const mine = acc.id === curAccount().id;
  const state = own
    ? `<span class="chip ok">${icon('user', 12)}Profil zugeordnet: ${esc(abUserName(own))}</span>`
    : res ? `<span class="chip info">Profil nicht zugeordnet · geht als ${esc(abUserName(res.id))}${res.via === 'fallback' ? ' (Sammelbenutzer)' : ''}</span>` : `<span class="chip warn">${icon('alert', 12)}Profil nicht zugeordnet · kein Benutzer</span>`;
  const right = `<span class="chip ${p.canRequest ? 'ok' : 'info'}">${icon(p.canRequest ? 'wish' : 'ban', 12)}${p.canRequest ? 'Darf anfordern' : 'Darf nicht anfordern'}</span>${mine ? `<a class="lnk ab-small" href="#/settings/profiles" data-fid="mp:pl:${p.id}">Im Profil ändern</a>` : '<span class="muted ab-small">setzt das Hauptprofil dieses Kontos</span>'}`;
  return `<li class="ab-prow"><div class="ab-pn"><b>${esc(p.name)}</b><span class="muted ab-small">${p.primary ? 'Hauptprofil' : 'Weiteres Profil'} · ${ageLabel(effMax(p))}${p.pin ? ' · PIN' : ''}</span></div>
    <div class="ab-pc">${selectEl('S.map.profiles.' + p.id, abUserOptions(acc.id, own, first), { label: `Jellyseerr-Benutzer für Profil ${p.name}`, after: 'abMapProf' })}${state}</div>
    <div class="ab-pr">${right}</div>${u ? `<div class="ab-pw">${abUserFacts(u)}${abUserWarn(u)}</div>` : ''}</li>`;
}
function abMapCard(acc) {
  const id = S.map.accounts[acc.id], u = id ? seerrUser(id) : null;
  const sug = suggestionFor(acc);
  const profs = profilesOf(acc.id);
  const ownN = profs.filter((p) => S.map.profiles[p.id]).length;
  const profErr = profs.some((p) => { const x = S.map.profiles[p.id] && seerrUser(S.map.profiles[p.id]); return x && x.gone; });
  const changed = (ui.abChanged || {})[acc.id];
  const head = `<div class="ab-acch"><div class="ab-who"><b>${esc(acc.email)}</b><span class="ab-chips"><span class="chip info">${acc.role === 'admin' ? 'Administrator' : 'Konto'}</span>${acc.owner ? '<span class="chip outline">Besitzer</span>' : ''}${changed ? `<span class="chip ok">${icon('check', 12)}Zuordnung geändert</span>` : ''}</span></div><div class="ab-chips">${profs.map((p) => `<span class="chip info">${esc(p.name)}</span>`).join('')}</div></div>`;
  const sel = `<div class="ab-mapline"><span class="lbl">Jellyseerr-Benutzer</span>${selectEl('S.map.accounts.' + acc.id, abUserOptions(acc.id, id, { v: '', t: 'Nicht zugeordnet' }), { label: `Jellyseerr-Benutzer für ${acc.email}`, after: 'abMapAcc' })}${!u ? '<span class="chip warn">' + icon('alert', 12) + 'Zuordnung fehlt</span>' : ''}</div>`;
  const sugH = sug ? `<div class="ab-sug">${icon('info', 16)}<span>Vorschlag: <b>${esc(sug.name)}</b> (gleiche E-Mail-Adresse)</span><button type="button" class="btn sm secondary" data-act="abSuggest" data-id="${acc.id}" data-fid="mp:sg:${acc.id}">Übernehmen</button></div>` : '';
  const noRight = !acc.canRequest ? notice('', 'Dieses Konto darf laut Slouchcat keine Titel anfordern', 'Die Zuordnung wirkt erst, wenn du dem Konto das Recht gibst.', `<a class="btn sm secondary" href="#/admin/accounts" data-act="abOpenAcc" data-id="${acc.id}" data-fid="mp:acc:${acc.id}">${icon('users', 16)}Zu den Konten</a>`) : '';
  const band_ = band('mp:' + acc.id, 'Profile', `${ownN} von ${profs.length} mit eigenem Benutzer${profErr ? ' · <span class="chip bad">Benutzer nicht gefunden</span>' : ''}`, `<ul class="ab-plist">${profs.map((p) => abProfMap(acc, p)).join('')}</ul>`, { open: profErr, icon: 'users' });
  return `<article class="card ab-acc" aria-label="Zuordnung von ${esc(acc.email)}">${head}${sel}${sugH}${u ? abUserFacts(u) : ''}${abUserWarn(u)}${noRight}${band_}</article>`;
}
route('/admin/wishes/mapping', () => {
  const mc = mappingCounts();
  const sugs = S.accounts.filter((a) => suggestionFor(a));
  const gone = S.seerrUsers.filter((u) => u.gone).length;
  const loaded = S.seerr.usersAt || S.seerr.lastSync;
  const summary = [`<b>${mc.mapped} von ${mc.total} Konten zugeordnet</b>`, sugs.length ? abCount(sugs.length, 'Vorschlag', 'Vorschläge') : '', gone ? `<span class="chip bad">${abCount(gone, 'Benutzer gelöscht', 'Benutzer gelöscht')}</span>` : ''].filter(Boolean).join(' · ');
  const bar = `<div class="ab-bar"><div class="ab-line">${summary}</div><div class="row-gap"><span class="muted ab-small">Benutzerliste geladen ${ago(loaded)}</span><button type="button" class="btn secondary" data-act="abUsersReload" data-fid="mp:reload">${icon('refresh', 16)}Benutzerliste neu laden</button>${sugs.length ? `<button type="button" class="btn primary" data-act="dialog" data-name="abSuggestAll" data-fid="mp:all">${icon('check', 16)}Alle Vorschläge übernehmen</button>` : ''}</div></div>`;
  const free = S.seerrUsers.filter((u) => !u.gone && !abUserOwner(u.id));
  const freeCard = `<section class="card"><div class="ch"><span class="ik">${icon('user', 16)}</span><h2>Jellyseerr-Benutzer ohne Konto</h2><span class="muted ab-small">${abCount(free.length, 'Benutzer', 'Benutzer')}</span></div>
    ${free.length ? `<ul class="ab-free">${free.map((u) => {
      const ws = S.wishes.filter((w) => Number(w.seerrUser) === u.id);
      const un = ws.filter((w) => !w.owner).length;
      const sugFor = S.accounts.find((a) => suggestionFor(a) === u);
      return `<li><div class="ab-who"><b>${esc(u.name)}</b><span>${esc(u.type)} · ${esc(u.email)}</span></div><div class="ab-who"><span>${abCount(ws.length, 'Anfrage', 'Anfragen')} in Jellyseerr${un ? `, davon ${un} nicht zuordenbar` : ''}</span>${un ? `<a class="lnk" href="#/admin/wishes" data-act="set" data-path="ui.filter.adminWishes" data-value="unassigned" data-fid="mp:un:${u.id}">Stehen unter «Anfragen › Nicht zuordenbar»</a>` : ''}${sugFor ? `<span>Vorschlag für ${esc(sugFor.email)}</span>` : ''}</div>${abUserFacts(u)}</li>`;
    }).join('')}</ul>` : '<p class="muted">Jeder Jellyseerr-Benutzer gehört zu einem Konto.</p>'}
    <p class="muted ab-small">Anfragen dieser Benutzer übernimmt Slouchcat erst, wenn du sie einem Konto zuordnest.</p></section>`;
  const sandra = seerrUser(2);
  const help = `<div class="ab-mockhelp"><span class="ab-small"><b>Mockup-Hilfe</b> (gehört nicht zur App)</span><button type="button" class="btn sm secondary" data-act="abMockGone" data-fid="mp:mock">${sandra.gone ? 'Benutzer „sandra“ in Jellyseerr wiederherstellen (Mockup)' : 'Benutzer „sandra“ in Jellyseerr löschen (Mockup)'}</button></div>`;
  const html = `${wishTabs('mapping')}${abWishNotices()}${bar}<div class="stack">${S.accounts.map(abMapCard).join('')}</div>${freeCard}${help}`;
  return { type: 'admin', area: 'media', page: 'wishes', title: 'Wünsche', h1: 'Wünsche', crumb: 'Wünsche › Zuordnung', intro: 'Wer als wer an Jellyseerr geht: Jedes Konto bekommt einen Jellyseerr-Benutzer, einzelne Profile wahlweise einen eigenen. Rechte, Kontingent und Auto-Freigabe kommen von diesem Benutzer.', html };
});
function abMark(accId) { ui.abChanged = ui.abChanged || {}; ui.abChanged[accId] = Date.now(); }
ACT.abMapAcc = (d, el) => {
  const id = d.bind.split('.').pop(), acc = account(id);
  if (el.value === '') delete S.map.accounts[id]; else S.map.accounts[id] = Number(el.value);
  save(); abMark(id);
  toast(`Zuordnung geändert: ${esc(acc.email)} → ${el.value === '' ? 'Nicht zugeordnet' : esc(abUserName(el.value))}`, 'ok');
};
ACT.abMapProf = (d, el) => {
  const pid = d.bind.split('.').pop(), p = profile(pid);
  if (el.value === '') delete S.map.profiles[pid]; else S.map.profiles[pid] = Number(el.value);
  save(); abMark(p.accountId);
  toast(`Zuordnung geändert: Profil ${esc(p.name)} → ${el.value === '' ? 'Wie Konto' : esc(abUserName(el.value))}`, 'ok');
};
ACT.abSuggest = (d) => {
  const acc = account(d.id), u = suggestionFor(acc);
  if (!u) return;
  S.map.accounts[acc.id] = u.id; abMark(acc.id); commit();
  toast(`Zuordnung geändert: ${esc(acc.email)} → ${esc(u.name)}`, 'ok');
};
DIALOGS.abSuggestAll = () => {
  const sugs = S.accounts.map((a) => [a, suggestionFor(a)]).filter((x) => x[1]);
  return {
    title: 'Alle Vorschläge übernehmen?',
    body: `<p class="lead">Slouchcat ordnet diese Konten zu. Ab dann gehen ihre Wünsche als dieser Jellyseerr-Benutzer an Jellyseerr.</p><ul class="checks">${sugs.map(([a, u]) => `<li class="ok">${icon('check', 16)}<span><b>${esc(a.email)}</b> → ${esc(u.name)}<small>gleiche E-Mail-Adresse${u.autoApprove ? ', Auto-Freigabe' : ', braucht Freigabe'}</small></span></li>`).join('')}</ul>`,
    actions: `<button type="button" class="btn ghost" data-act="close" data-fid="sa:x">Abbrechen</button><button type="button" class="btn primary" data-act="abSuggestAllOk" data-fid="sa:ok" data-autofocus>${abCount(sugs.length, 'Vorschlag', 'Vorschläge')} übernehmen</button>`,
  };
};
ACT.abSuggestAllOk = () => {
  const sugs = S.accounts.map((a) => [a, suggestionFor(a)]).filter((x) => x[1]);
  sugs.forEach(([a, u]) => { S.map.accounts[a.id] = u.id; abMark(a.id); });
  closeDialog(); commit();
  toast(`${abCount(sugs.length, 'Zuordnung', 'Zuordnungen')} übernommen.`, 'ok');
};
ACT.abUsersReload = () => {
  if (seerrState().k !== 'ok') { toast(`Benutzerliste nicht geladen: Jellyseerr ${esc(seerrState().label.toLowerCase())}. Gezeigt wird der letzte Stand.`, 'bad'); return; }
  S.seerr.usersAt = Date.now();
  logSeerr('Benutzerliste', 'ok', `${S.seerrUsers.filter((u) => !u.gone).length} Benutzer geladen`);
  commit();
  toast(`${S.seerrUsers.filter((u) => !u.gone).length} Benutzer geladen · ${fmtTime(S.seerr.usersAt)}`, 'ok');
};
ACT.abMockGone = () => {
  const u = seerrUser(2);
  u.gone = !u.gone;
  commit();
  toast(u.gone ? 'Mockup: «sandra» ist in Jellyseerr gelöscht.' : 'Mockup: «sandra» ist wieder da.', 'warn', 3000);
};
ACT.abOpenAcc = (d) => { ui.open['ac:' + d.id] = true; ui.filter.accTab = 'accounts'; };

/* ================= M-25 Wünsche › Einstellungen ================= */
const AB_LABEL = { enabled: 'Wünsche aktiv', movies: 'Filme erlauben', shows: 'Serien erlauben', fromSpotlight: 'Aus dem Spotlight', fromSearch: 'Aus der Suche', fromDetail: 'Auf Detailseiten', join: 'Bereits angeforderte Titel', unmapped: 'Konten ohne Zuordnung', fallbackUser: 'Sammelbenutzer', newAccounts: 'Neue Konten dürfen anfordern', withdraw: 'Zurückziehen erlauben', pinPrivacy: 'Externe Nachrichten bei PIN-Profilen ohne Titel' };
const AB_VAL = { join: 'Mitwünschen', status: 'Nur Zustand zeigen', deny: 'Nicht erlauben', fallback: 'Sammelbenutzer' };
ACT.abSaved = (d) => {
  const path = d.path || d.bind;
  const m = /^S\.accounts\.(\d+)\.canRequest$/.exec(path);
  if (m) { const a = S.accounts[Number(m[1])]; toast(`${esc(a.email)} ${a.canRequest ? 'darf jetzt Titel anfordern' : 'darf keine Titel mehr anfordern'}.`, 'ok'); return; }
  const k = path.split('.').pop(), v = getPath(path), l = AB_LABEL[k] || 'Einstellung';
  if (typeof v === 'boolean') toast(`«${l}» ist ${v ? 'eingeschaltet' : 'ausgeschaltet'}. Gilt sofort.`, 'ok');
  else if (k === 'fallbackUser') toast(v ? `Sammelbenutzer: ${esc(abUserName(v))}. Gilt sofort.` : 'Kein Sammelbenutzer gewählt.', v ? 'ok' : 'warn');
  else toast(`«${l}»: ${esc(AB_VAL[v] || v)}. Gilt sofort.`, 'ok');
};
function abSrow(title, help, ctl, o = {}) {
  return `<div class="${cls('srow', o.isNew !== false && 'new', o.dim && 'ab-dim')}"><div class="st"><b>${title}</b>${help ? `<span>${help}</span>` : ''}</div><div class="sc ${o.col ? 'col' : ''}">${ctl}</div></div>`;
}
function abSeg(path, opts, o = {}) {
  const cur = getPath(path);
  return `<div class="seg" role="group" aria-label="${esc(o.label || '')}">${opts.map(([v, t]) => `<button type="button" data-act="set" data-path="${path}" data-value="${v}" ${o.after ? `data-after="${o.after}"` : ''} aria-pressed="${String(cur) === String(v)}" data-fid="seg:${path}:${v}" ${o.disabled ? 'disabled' : ''}>${t}</button>`).join('')}</div>`;
}
route('/admin/wishes/settings', () => {
  const w = S.settings.wishes, off = !w.enabled, sp = S.settings.spotlight, ss = seerrState();
  const P = 'S.settings.wishes.';
  const s = (k, label, help) => abSrow(label, help, sw(P + k, label, { disabled: off, after: 'abSaved' }), { dim: off });
  const base = `<section class="card"><div class="ch"><span class="ik">${icon('wish', 16)}</span><h2>Grundsatz</h2></div>
    ${abSrow('Wünsche aktiv', 'Profile mit Recht können fehlende Titel anfordern. Aus: Niemand sieht fehlende Titel, offene Wünsche laufen in Jellyseerr weiter.', sw(P + 'enabled', 'Wünsche aktiv', { after: 'abSaved' }) + `<span class="chip ${w.enabled ? 'ok' : 'outline'}">${w.enabled ? 'An' : 'Aus'}</span>`)}
    ${s('movies', 'Filme erlauben', 'Gilt für alle Profile. Aus: kein «Film anfordern».')}
    ${s('shows', 'Serien erlauben', 'Gilt für alle Profile. Aus: kein «Serie anfordern» und keine «Staffeln anfordern».')}</section>`;
  const where = `<section class="card"><div class="ch"><span class="ik">${icon('layers', 16)}</span><h2>Woher angefordert werden darf</h2></div>
    ${s('fromSpotlight', 'Aus dem Spotlight', 'Knopf auf der Bühne und in der Posterreihe. Aus: Spotlight und Spotlight-Liste zeigen keine fehlenden Titel.')}
    ${s('fromSearch', 'Aus der Suche', 'Abschnitt «Noch nicht in der Bibliothek». Aus: Der Abschnitt fehlt.')}
    ${s('fromDetail', 'Auf Detailseiten (fehlende Staffeln)', 'Fehlende Staffeln auf der Seite einer vorhandenen Serie. Aus: Die Serienseite zeigt nur, was vorhanden ist.')}
    ${abSrow('Fehlende Titel im Spotlight', 'Ob das Spotlight fehlende Titel zeigt, stellst du auf der Seite Spotlight ein.', `<span class="chip ${sp.showMissing ? 'ok' : 'outline'}">${sp.showMissing ? `An · höchstens ${sp.maxMissing}` : 'Aus'}</span><a class="lnk" href="#/admin/spotlight" data-fid="ws:spot">Zur Seite Spotlight</a>`, { isNew: false, dim: off })}</section>`;
  const fbOpts = [{ v: '', t: 'Benutzer wählen' }, ...S.seerrUsers.filter((u) => !u.gone).map((u) => ({ v: u.id, t: u.name }))];
  const rules = `<section class="card"><div class="ch"><span class="ik">${icon('settings', 16)}</span><h2>Regeln</h2></div>
    ${abSrow('Bereits angeforderte Titel', w.join === 'join' ? 'Mitwünschen: Ein zweites Profil hängt sich an den offenen Wunsch («Benachrichtige mich») und wird ebenfalls benachrichtigt. Jellyseerr bekommt keine zweite Anfrage.' : 'Nur Zustand zeigen: Ein zweites Profil sieht «Bereits angefordert», kann sich aber nicht dazuschalten und wird nicht benachrichtigt.', abSeg(P + 'join', [['join', 'Mitwünschen'], ['status', 'Nur Zustand zeigen']], { label: 'Bereits angeforderte Titel', after: 'abSaved', disabled: off }), { dim: off, col: false })}
    ${abSrow('Konten ohne Zuordnung', 'Sammelbenutzer: Alle Konten ohne eigenen Benutzer teilen Kontingent und Auto-Freigabe dieses Benutzers.', abSeg(P + 'unmapped', [['deny', 'Nicht erlauben'], ['fallback', 'Sammelbenutzer']], { label: 'Konten ohne Zuordnung', after: 'abSaved', disabled: off }) + (w.unmapped === 'fallback' ? selectEl(P + 'fallbackUser', fbOpts, { label: 'Sammelbenutzer', after: 'abSaved', disabled: off }) + (!w.fallbackUser ? '<span class="chip warn">Ohne Benutzer gilt «Nicht erlauben»</span>' : '') : ''), { dim: off })}
    ${s('newAccounts', 'Neue Konten dürfen anfordern', 'Vorgabe des Schalters im Dialog «Konto einladen». Ändert bestehende Konten nicht.')}
    ${s('withdraw', 'Zurückziehen erlauben', 'Solange ein Wunsch nicht freigegeben ist. Knopf «Zurückziehen» unter «Meine Wünsche».')}
    ${s('pinPrivacy', 'Externe Nachrichten bei PIN-Profilen ohne Titel', 'E-Mail, Web-Push und Telegram nennen dann nur das Profil: «[Sandra] Ein Wunsch ist jetzt verfügbar.»')}</section>`;
  const refs = `<section class="card"><div class="ch"><span class="ik">${icon('link', 16)}</span><h2>Verweise</h2></div>
    ${abSrow('Wer anfordern darf', 'Wer anfordern darf, stellst du je Konto unter Konten ein; je Profil entscheidet das Hauptprofil.', `<a class="lnk" href="#/admin/accounts" data-fid="ws:acc">Zu den Konten</a>`, { isNew: false })}
    ${abSrow('Verbindung zu Jellyseerr', 'Adresse, Schlüssel, Test und Protokoll.', `<span class="chip ${ss.chip}">${esc(ss.label)}</span><a class="lnk" href="#/admin/integrations/jellyseerr" data-fid="ws:conn">Zur Verbindung</a>`, { isNew: false })}
    ${abSrow('Zuordnung', 'Welches Konto als welcher Jellyseerr-Benutzer anfordert.', `<span class="chip ${mappingCounts().missing ? 'warn' : 'ok'}">${mappingCounts().mapped} von ${mappingCounts().total} Konten</span><a class="lnk" href="#/admin/wishes/mapping" data-fid="ws:map">Zur Zuordnung</a>`, { isNew: false })}</section>`;
  const html = `${wishTabs('settings')}${abWishNotices(true)}${off ? notice('warn', 'Wünsche sind ausgeschaltet', 'Die übrigen Einstellungen sind gesperrt, bis du «Wünsche aktiv» einschaltest.') : ''}<p class="muted ab-small">Jede Änderung gilt sofort.</p>${base}${where}${rules}${refs}`;
  return { type: 'admin', area: 'media', page: 'wishes', title: 'Wünsche', crumb: 'Wünsche › Einstellungen', intro: 'Was angefordert werden darf, von wo aus und nach welchen Regeln. Die Regeln gelten für alle Konten.', html };
});

/* ================= M-26 Konten ================= */
function abAccWish(acc) {
  if (!acc.canRequest) return '<span class="chip info">Nicht erlaubt</span>';
  const id = S.map.accounts[acc.id];
  if (!id) {
    if (S.settings.wishes.unmapped === 'fallback' && S.settings.wishes.fallbackUser) return `<span class="chip ok">Erlaubt · ${esc(abUserName(S.settings.wishes.fallbackUser))} (Sammelbenutzer)</span>`;
    return `<span class="chip warn">${icon('alert', 12)}Zuordnung fehlt</span>`;
  }
  const u = seerrUser(id);
  if (!u || u.gone) return `<span class="chip bad">${icon('alert', 12)}Benutzer gelöscht</span>`;
  return `<span class="chip ok">Erlaubt · ${esc(u.name)}</span>`;
}
function abProfSign(p) {
  return `<span class="chip ${p.canRequest ? 'ok' : 'info'}" title="${p.canRequest ? 'darf anfordern' : 'darf nicht anfordern'}">${icon(p.canRequest ? 'wish' : 'ban', 12)}${esc(p.name)}<span class="vh">: ${p.canRequest ? 'darf anfordern' : 'darf nicht anfordern'}</span></span>`;
}
function abWishBlock(acc, path, after) {
  const id = S.map.accounts[acc.id], u = id ? seerrUser(id) : null;
  return `<div class="ab-swl">${sw(path, 'Darf Titel anfordern', after ? { after } : {})}<span class="ab-who"><b>Darf Titel anfordern</b><span>Deckel für alle Profile dieses Kontos. Welches Profil anfordern darf, stellt das Hauptprofil ein.</span></span></div>
    <p class="ab-line">${icon('user', 16)}<span>Jellyseerr-Benutzer: <b>${u ? esc(u.name) + (u.gone ? ' (in Jellyseerr gelöscht)' : '') : 'Nicht zugeordnet'}</b></span><a class="lnk" href="#/admin/wishes/mapping" data-fid="awb:${path}">Zuordnung ändern</a></p>
    <div class="ab-chips" aria-label="Profile und ihr Wunschrecht">${profilesOf(acc.id).map(abProfSign).join('')}</div>`;
}
function abLibsField(base, fp) {
  const d = getPath(base);
  const none = !d.allLibs && !Object.values(d.libs).some(Boolean);
  return `<div class="fld"><span class="lbl">Bibliotheken</span><div class="seg" role="group" aria-label="Bibliotheken">${[[true, 'Alle'], [false, 'Auswahl']].map(([v, l]) => `<button type="button" data-act="set" data-path="${base}.allLibs" data-value="${v}" aria-pressed="${d.allLibs === v}" data-fid="${fp}:lm:${v}">${l}</button>`).join('')}</div>${d.allLibs ? '<span class="hint">Alle Bibliotheken</span>' : `<span class="hint">Bibliotheken wählen</span><div class="ab-libs">${LIBS.map(([l]) => `<label class="check"><input type="checkbox" data-bind="${base}.libs.${l}" data-fid="${fp}:lib:${l}" ${d.libs[l] ? 'checked' : ''}><span>${esc(l)}</span></label>`).join('')}</div>${none ? '<span class="err">Wähle mindestens eine Bibliothek.</span>' : ''}`}</div>`;
}
function abAccVals(acc) {
  const libs = acc.libs || ['Filme', 'Serien'];
  return { rating: Number(acc.maxRating), allLibs: !!acc.allLibraries, libs: Object.fromEntries(LIBS.map(([l]) => [l, libs.includes(l)])), maxHeight: acc.maxHeight ?? '', bitrate: acc.bitrate ?? '', streams: acc.streams ?? '', canRequest: !!acc.canRequest };
}
function abAccDraft(acc) { const k = 'acc_' + acc.id; if (!ui.draft[k]) ui.draft[k] = abAccVals(acc); return ui.draft[k]; }
function abAccDirty(acc) { const norm = (x) => JSON.stringify({ ...x, rating: Number(x.rating), maxHeight: String(x.maxHeight), bitrate: String(x.bitrate), streams: String(x.streams) }); return norm(abAccDraft(acc)) !== norm(abAccVals(acc)); }
function abNumFld(path, label, min, max, fid) {
  const v = getPath(path);
  const bad = v !== '' && v != null && (v < min || v > max);
  return `<div class="fld"><label for="${fid}">${label}</label><input id="${fid}" class="inp num" type="number" min="${min}" max="${max}" data-bind="${path}" data-num data-live data-fid="${fid}" value="${esc(v ?? '')}" ${bad ? 'aria-invalid="true"' : ''}><span class="hint">${min} bis ${max}. Leer heisst ohne Limit.</span>${bad ? `<span class="err">Erlaubt sind ${min} bis ${max}.</span>` : ''}</div>`;
}
function abAccessForm(acc) {
  const d = abAccDraft(acc), base = 'ui.draft.acc_' + acc.id, fp = 'af:' + acc.id;
  const heights = [{ v: '', t: 'Ohne Limit' }, ...[360, 480, 720, 1080, 1440, 2160].map((h) => ({ v: h, t: `bis ${h}p` }))];
  const dirty = abAccDirty(acc);
  return `<section class="card ab-form" aria-label="Zugriff von ${esc(acc.email)}"><div class="ch"><span class="ik">${icon('lock', 16)}</span><h2>Zugriff</h2>${dirty ? '<span class="chip warn">Ungespeicherte Änderungen</span>' : ''}</div>
    <div class="ab-grid2"><div class="fld"><span class="lbl">Höchste Altersfreigabe</span>${selectEl(base + '.rating', AB_RATINGS, { label: 'Höchste Altersfreigabe' })}</div>${abLibsField(base, fp)}</div>
    <fieldset class="ab-fs"><legend>Wiedergabe-Limits</legend><p class="muted ab-small">Gilt für alle Profile dieses Kontos.</p><div class="ab-grid3"><div class="fld"><span class="lbl">Höchste Qualität</span>${selectEl(base + '.maxHeight', heights, { label: 'Höchste Qualität' })}</div>${abNumFld(base + '.bitrate', 'Bandbreite (kbit/s)', 500, 100000, fp + ':br')}${abNumFld(base + '.streams', 'Gleichzeitige Streams', 1, 10, fp + ':st')}</div></fieldset>
    <fieldset class="ab-fs ab-new"><legend>Wünsche <span class="newtag">NEU</span></legend>${abWishBlock(acc, base + '.canRequest')}</fieldset>
    <div class="foot"><button type="button" class="btn quiet" data-act="abAccDiscard" data-id="${acc.id}" data-fid="${fp}:discard" ${dirty ? '' : 'disabled'}>Verwerfen</button><button type="button" class="btn primary" data-act="abAccSave" data-id="${acc.id}" data-fid="${fp}:save">Zugriff speichern</button></div></section>`;
}
function abOwnForm(acc) {
  if (!ui.draft.own) ui.draft.own = { email: acc.email, pw1: '', pw2: '', cur: '' };
  const d = ui.draft.own, e = ui.abOwnErr || {};
  const f = (k, label, type, hint) => `<div class="fld"><label for="ow:${k}">${label}</label><input id="ow:${k}" class="inp" type="${type}" data-bind="ui.draft.own.${k}" data-fid="ow:${k}" value="${esc(d[k])}" autocomplete="${type === 'password' ? (k === 'cur' ? 'current-password' : 'new-password') : 'email'}" ${e[k] ? 'aria-invalid="true"' : ''}>${hint ? `<span class="hint">${hint}</span>` : ''}${e[k] ? `<span class="err">${e[k]}</span>` : ''}</div>`;
  return `<section class="card ab-form" aria-label="Konto ändern"><div class="ch"><span class="ik">${icon('user', 16)}</span><h2>Konto ändern</h2></div><p class="muted ab-small">E-Mail-Adresse und Passwort deines Kontos. Jede Änderung braucht dein aktuelles Passwort.</p>
    ${f('email', 'E-Mail-Adresse', 'email')}${f('pw1', 'Neues Passwort', 'password', 'Leer lassen, um das Passwort zu behalten. Sonst mindestens 10 Zeichen.')}${f('pw2', 'Neues Passwort wiederholen', 'password')}${f('cur', 'Aktuelles Passwort', 'password', 'Bestätigt, dass du es bist.')}
    <div class="foot"><button type="button" class="btn quiet" data-act="abOwnDiscard" data-fid="ow:discard">Verwerfen</button><button type="button" class="btn primary" data-act="abOwnSave" data-fid="ow:save">Speichern</button></div></section>`;
}
function abAccDetail(acc) {
  const idx = S.accounts.indexOf(acc);
  const mine = acc.id === curAccount().id;
  const wishCard = `<section class="card ab-new"><div class="ch"><span class="ik">${icon('wish', 16)}</span><h2>Wünsche</h2><span class="newtag">NEU</span></div>${abWishBlock(acc, `S.accounts.${idx}.canRequest`, 'abSaved')}<p class="muted ab-small">Gilt sofort.</p></section>`;
  if (mine) {
    const profs = profilesOf(acc.id);
    const mp = `<section class="card" id="meine-profile"><div class="ch"><span class="ik">${icon('users', 16)}</span><h2>Meine Profile</h2><a class="btn sm secondary" href="#/settings/profiles" data-fid="ac:prof">Profile verwalten</a></div><ul class="ab-plist">${profs.map((p) => `<li class="ab-prow"><div class="ab-pn"><b>${esc(p.name)}</b><span class="muted ab-small">${p.primary ? 'Hauptprofil' : 'Weiteres Profil'} · ${ageLabel(p.maxRating)}${p.pin ? ' · PIN' : ''}</span></div><div class="ab-pr">${abProfSign(p)}</div></li>`).join('')}</ul></section>`;
    return `<div class="ab-subg">${abOwnForm(acc)}<div class="stack">${wishCard}${mp}</div></div>`;
  }
  const profs = `<section class="card"><div class="ch"><span class="ik">${icon('users', 16)}</span><h2>Profile</h2></div><div class="ab-chips">${profilesOf(acc.id).map((p) => `<span class="chip info">${esc(p.name)}${p.primary ? ' (Hauptprofil)' : ''}</span>`).join('')}</div><p class="muted ab-small">Profile verwaltet jedes Konto selbst.</p></section>`;
  if (acc.role === 'admin') return `<div class="ab-subg"><div class="stack">${notice('', 'Administrator', 'Ein Administrator hat immer die Stufe «ab 18» und alle Bibliotheken; «Zugriff» gibt es nur für Konten ohne Verwaltung.')}${wishCard}</div>${profs}</div>`;
  return `<div class="ab-subg">${abAccessForm(acc)}${profs}</div>`;
}
function abAccRow(acc) {
  const open = isOpen('ac:' + acc.id), id = acc.id;
  const mine = acc.id === curAccount().id;
  const profs = profilesOf(id).map((p) => (p.primary ? `${esc(p.name)} (Hauptprofil)` : esc(p.name))).join(', ');
  const ch = Object.entries(acc.channels).filter(([, c]) => c.enabled).map(([k]) => `<span class="chip info">${CHANNEL_LABEL[k]}</span>`).join('') || 'Keine';
  const libs = acc.role === 'admin' || acc.allLibraries ? 'Alle Bibliotheken' : abCount((acc.libs || ['Filme', 'Serien']).length, 'Bibliothek', 'Bibliotheken');
  let menu = '';
  if (!mine && !acc.owner) {
    const mo = isOpen('am:' + id);
    const items = [];
    if (curAccount().owner) items.push(acc.role === 'admin' ? ['abDemote', 'Rolle entziehen', 'shield', ''] : ['abPromote', 'Zum Administrator ernennen', 'shield', '']);
    items.push(['abRemove', 'Konto entfernen', 'trash', 'dq']);
    menu = `<span class="ab-menuw"><button type="button" class="btn sm quiet icon" data-act="open" data-id="am:${id}" aria-expanded="${mo}" aria-label="Weitere Aktionen für ${esc(acc.email)}" data-fid="ac:menu:${id}">${icon('dots', 16)}</button>${mo ? `<span class="ab-menu" role="menu">${items.map(([n, l, ic, c]) => `<button type="button" role="menuitem" class="btn sm quiet ${c}" data-act="abMenu" data-name="${n}" data-id="${id}" data-fid="ac:mi:${n}:${id}">${icon(ic, 16)}${l}</button>`).join('')}</span>` : ''}</span>`;
  }
  return `<tr><td class="main"><button type="button" class="tog" data-act="open" data-id="ac:${id}" aria-expanded="${open}" data-fid="ac:tog:${id}"><span class="chev">${icon('chevR', 16)}</span>${esc(acc.email)}</button></td>
    <td><span class="ab-chips"><span>${acc.role === 'admin' ? 'Administrator' : 'Konto'}</span>${acc.owner ? '<span class="chip outline">Besitzer</span>' : ''}</span></td>
    <td class="nowrap">${ageLabel(acc.role === 'admin' ? 18 : acc.maxRating)}</td><td class="nowrap">${libs}</td><td>${profs}</td><td>${abAccWish(acc)}</td><td class="nowrap">${acc.lastSeen ? ago(acc.lastSeen) : 'Noch nie'}</td><td><span class="ab-chips">${ch}</span></td><td class="end">${menu}</td></tr>
    ${open ? `<tr class="sub"><td colspan="9">${abAccDetail(acc)}</td></tr>` : ''}`;
}
const AB_INV_STATE = { open: ['Offen', 'ok'], used: ['Benutzt', 'info'], expired: ['Abgelaufen', 'outline'], revoked: ['Widerrufen', 'outline'] };
function abInvRows() {
  if (!S.invitations.length) return '<div class="empty">Noch keine Einladung erstellt.</div>';
  return `<div class="tblw"><table class="tbl" aria-label="Bestehende Einladungen"><thead><tr><th>Notiz</th><th>Zustand</th><th>Altersstufe</th><th>Bibliotheken</th><th>Wünsche <span class="newtag">NEU</span></th><th>Gültig bis / angenommen</th><th class="end">Aktionen</th></tr></thead><tbody>${S.invitations.map((inv) => {
    const [l, c] = AB_INV_STATE[inv.state] || [inv.state, 'info'];
    return `<tr><td class="main">${inv.note ? esc(inv.note) : '<span class="muted">Ohne Notiz</span>'}</td><td><span class="chip ${c}">${l}</span></td><td>${ageLabel(inv.maxRating)}</td><td>${inv.allLibraries === false ? abCount((inv.libs || []).length, 'Bibliothek', 'Bibliotheken') : 'Alle Bibliotheken'}</td><td>${inv.canRequest ? '<span class="chip ok">Darf anfordern</span>' : '<span class="chip info">Nicht erlaubt</span>'}</td><td class="nowrap">${inv.usedBy ? `Angenommen von ${esc(inv.usedBy)}` : `Gültig bis ${fmtDate(inv.until)}`}</td><td class="end">${inv.state === 'open' ? `<button type="button" class="btn sm dq" data-act="dialog" data-name="abRevoke" data-id="${inv.id}" data-fid="inv:rv:${inv.id}">Widerrufen</button>` : ''}</td></tr>`;
  }).join('')}</tbody></table></div>`;
}
route('/admin/accounts', () => {
  const tab = ui.filter.accTab || 'accounts';
  const admins = S.accounts.filter((a) => a.role === 'admin');
  const openInv = S.invitations.filter((i) => i.state === 'open');
  const next = openInv.length ? Math.min(...openInv.map((i) => i.until)) : null;
  const own = profilesOf(curAccount().id).length;
  const stats = `<div class="stats" style="--n:4" aria-label="Zusammenfassung"><div class="stat"><span class="v">${S.accounts.length}</span><span class="l">Konten</span></div><div class="stat"><span class="v">${admins.length}</span><span class="l">Administratoren</span><span class="n">davon ${admins.filter((a) => a.owner).length} Besitzer</span></div><div class="stat"><span class="v">${S.profiles.length}</span><span class="l">Profile</span><span class="n">${own} ${own === 1 ? 'eigenes' : 'eigene'}</span></div><div class="stat"><span class="v">${openInv.length}</span><span class="l">Offene Einladungen</span>${next ? `<span class="n">Nächste gültig bis ${fmtDate(next)}</span>` : ''}</div></div>`;
  const tabs = `<div class="subtabs ab-tabs" role="group" aria-label="Bereiche der Konten"><button type="button" data-act="set" data-path="ui.filter.accTab" data-value="accounts" aria-pressed="${tab === 'accounts'}" data-fid="ac:t:acc">Konten<span class="c">${abCount(S.accounts.length, 'Konto', 'Konten')}</span></button><button type="button" data-act="set" data-path="ui.filter.accTab" data-value="invitations" aria-pressed="${tab === 'invitations'}" data-fid="ac:t:inv">Einladungen<span class="c ${openInv.length ? 'hot' : ''}">${openInv.length} offen</span></button></div>`;
  const body = tab === 'accounts'
    ? `<div class="tblw"><table class="tbl" aria-label="Konten"><thead><tr><th>E-Mail</th><th>Rolle</th><th>Altersstufe</th><th>Bibliotheken</th><th>Profile</th><th>Wünsche <span class="newtag">NEU</span></th><th>Zuletzt gesehen</th><th>Kanäle</th><th class="end">Aktionen</th></tr></thead><tbody>${S.accounts.map(abAccRow).join('')}</tbody></table></div>`
    : abInvRows();
  return { type: 'admin', area: 'people', page: 'accounts', title: 'Konten', intro: 'Alle Konten mit Rolle, Altersstufe, Bibliotheken, Profilen und den Kanälen, über die sie Benachrichtigungen bekommen. Neue Konten entstehen über eine Einladung. Die Profile deines eigenen Kontos verwaltest du hier; die Profile anderer Konten verwaltet jedes Konto selbst.', actions: `<button type="button" class="btn primary" data-act="abInvite" data-fid="ac:invite">${icon('plus', 16)}Konto einladen</button>`, html: `${stats}${tabs}${body}` };
});
ACT.abMenu = (d) => { ui.open['am:' + d.id] = false; render(); openDialog(d.name, { id: d.id }); };
DIALOGS.abPromote = ({ id }) => { const a = account(id); return { title: `${esc(a.email)} zum Administrator ernennen?`, body: '<p class="lead">Das Konto verwaltet danach Server, Bibliotheken und Einstellungen. Seine Altersstufe steigt auf «ab 18» und alle Bibliotheken; die Profile des Kontos behalten ihre eigenen Stufen.</p>', actions: `<button type="button" class="btn ghost" data-act="close" data-fid="rp:x">Abbrechen</button><button type="button" class="btn primary" data-act="abRoleOk" data-id="${id}" data-role="admin" data-fid="rp:ok" data-autofocus>Ernennen</button>` }; };
DIALOGS.abDemote = ({ id }) => { const a = account(id); return { title: `${esc(a.email)} die Administratorrolle entziehen?`, body: '<p class="lead">Das Konto verwaltet danach nur noch seine eigenen Profile und wird überall abgemeldet. Altersstufe und Bibliotheken bleiben, wo sie sind; setze sie danach bewusst neu.</p>', actions: `<button type="button" class="btn ghost" data-act="close" data-fid="rd:x">Abbrechen</button><button type="button" class="btn danger" data-act="abRoleOk" data-id="${id}" data-role="user" data-fid="rd:ok" data-autofocus>Entziehen</button>` }; };
ACT.abRoleOk = (d) => {
  const a = account(d.id);
  a.role = d.role;
  if (d.role === 'admin') { a.maxRating = 18; a.allLibraries = true; }
  delete ui.draft['acc_' + a.id];
  closeDialog(); commit();
  toast(d.role === 'admin' ? `${esc(a.email)} ist jetzt Administrator.` : `${esc(a.email)} ist jetzt ein Konto ohne Verwaltung.`, 'ok');
};
DIALOGS.abRemove = ({ id }) => { const a = account(id); return { title: `Konto ${esc(a.email)} entfernen?`, body: '<p class="lead">Konto, Profile und Fortschritt werden gelöscht. Laufende Sitzungen und Wiedergaben enden sofort. Downloads auf den Geräten des Kontos verschwinden beim nächsten Kontakt mit dem Server. Ein Gerät, das nie mehr online kommt, behält sie.</p><p class="sec">Wünsche bleiben in Jellyseerr bestehen; in Slouchcat gelten sie danach als nicht zuordenbar.</p>', actions: `<button type="button" class="btn ghost" data-act="close" data-fid="rm:x">Abbrechen</button><button type="button" class="btn danger" data-act="abRemoveOk" data-id="${id}" data-fid="rm:ok" data-autofocus>${icon('trash', 16)}Konto entfernen</button>` }; };
ACT.abRemoveOk = (d) => {
  const a = account(d.id);
  const pids = profilesOf(a.id).map((p) => p.id);
  S.wishes.forEach((w) => { if (pids.includes(w.owner)) w.owner = w.joined.find((j) => !pids.includes(j)) || null; w.joined = w.joined.filter((j) => !pids.includes(j) && j !== w.owner); });
  S.notifications = S.notifications.filter((n) => !pids.includes(n.profileId));
  pids.forEach((p) => delete S.map.profiles[p]);
  delete S.map.accounts[a.id];
  S.profiles = S.profiles.filter((p) => p.accountId !== a.id);
  S.deliveries = S.deliveries.filter((x) => x.accountId !== a.id);
  S.accounts = S.accounts.filter((x) => x.id !== a.id);
  closeDialog(); commit();
  toast(`Konto ${esc(a.email)} entfernt.`, 'ok');
};
ACT.abAccDiscard = (d) => { delete ui.draft['acc_' + d.id]; render(); toast('Änderungen verworfen.', 'ok', 2500); };
ACT.abAccSave = (d) => {
  const a = account(d.id), dr = abAccDraft(a);
  if (!dr.allLibs && !Object.values(dr.libs).some(Boolean)) { toast('Wähle mindestens eine Bibliothek.', 'bad'); return; }
  const bad = (v, lo, hi) => v !== '' && v != null && (v < lo || v > hi);
  if (bad(dr.bitrate, 500, 100000) || bad(dr.streams, 1, 10)) { toast('Prüfe die Wiedergabe-Limits.', 'bad'); return; }
  a.maxRating = Number(dr.rating); a.allLibraries = dr.allLibs; a.libs = Object.keys(dr.libs).filter((k) => dr.libs[k]);
  a.maxHeight = dr.maxHeight === '' ? '' : Number(dr.maxHeight); a.bitrate = dr.bitrate; a.streams = dr.streams; a.canRequest = dr.canRequest;
  delete ui.draft['acc_' + a.id];
  commit();
  toast(`Zugriff von ${esc(a.email)} gespeichert.`, 'ok');
};
ACT.abOwnDiscard = () => { delete ui.draft.own; ui.abOwnErr = {}; render(); };
ACT.abOwnSave = () => {
  const a = curAccount(), d = ui.draft.own, e = {};
  if (d.email.trim() === a.email && !d.pw1) { ui.abOwnErr = {}; toast('Ändere die E-Mail-Adresse oder gib ein neues Passwort ein.', 'warn'); render(); return; }
  if (!/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(d.email.trim())) e.email = 'Gib eine gültige E-Mail-Adresse ein.';
  if (d.pw1 && d.pw1.length < 10) e.pw1 = 'Mindestens 10 Zeichen.';
  if (d.pw1 && d.pw1 !== d.pw2) e.pw2 = 'Die beiden Passwörter sind nicht gleich.';
  if (!d.cur) e.cur = 'Das aktuelle Passwort stimmt nicht.';
  ui.abOwnErr = e;
  if (Object.keys(e).length) { render(); return; }
  a.email = d.email.trim();
  delete ui.draft.own;
  commit();
  toast('Dein Konto ist gespeichert.', 'ok');
};
ACT.abInvite = () => {
  ui.draft.invite = { note: '', rating: 16, allLibs: true, libs: Object.fromEntries(LIBS.map(([l]) => [l, false])), canRequest: !!S.settings.wishes.newAccounts, link: '' };
  openDialog('abInvite');
};
DIALOGS.abInvite = () => {
  const d = ui.draft.invite;
  if (d.link) {
    return { title: 'Konto einladen', body: `${notice('warn', 'Kopiere den Link jetzt: Er wird nur dieses eine Mal angezeigt.', '')}<div class="fld"><label for="iv:link">Einladungslink</label><input id="iv:link" class="inp mono" readonly value="${esc(d.link)}" data-fid="iv:link"></div>`, actions: `<button type="button" class="btn secondary" data-act="abInviteCopy" data-fid="iv:copy" data-autofocus>${icon('copy', 16)}Link kopieren</button><button type="button" class="btn primary" data-act="abInviteDone" data-fid="iv:done">Fertig</button>` };
  }
  const none = !d.allLibs && !Object.values(d.libs).some(Boolean);
  return {
    title: 'Konto einladen',
    body: `<p class="lead">Ein Link gilt einmal und sieben Tage. Wer ihn öffnet, legt ein eigenes Konto an.</p>
      <div class="fld"><label for="iv:note">Für wen ist der Link?</label><input id="iv:note" class="inp" data-bind="ui.draft.invite.note" data-fid="iv:note" value="${esc(d.note)}" data-autofocus><span class="hint">Zum Beispiel ein Name. Die Notiz sieht auch die eingeladene Person.</span></div>
      <div class="fld"><span class="lbl">Höchste Altersfreigabe</span>${selectEl('ui.draft.invite.rating', AB_RATINGS, { label: 'Höchste Altersfreigabe' })}</div>
      ${abLibsField('ui.draft.invite', 'iv')}
      <div class="ab-swl ab-new">${sw('ui.draft.invite.canRequest', 'Darf Titel anfordern')}<span class="ab-who"><b>Darf Titel anfordern <span class="newtag">NEU</span></b><span>Vorgabe aus «Neue Konten dürfen anfordern» unter Wünsche › Einstellungen. Die Zuordnung zu Jellyseerr folgt nach dem Anlegen.</span></span></div>`,
    actions: `<button type="button" class="btn ghost" data-act="close" data-fid="iv:x">Abbrechen</button><button type="button" class="btn primary" data-act="abInviteCreate" data-fid="iv:ok" ${none ? 'disabled' : ''}>Einladung erstellen</button>`,
  };
};
ACT.abInviteCreate = () => {
  const d = ui.draft.invite;
  const tok = Math.random().toString(36).slice(2, 10) + Math.random().toString(36).slice(2, 10);
  S.invitations.unshift({ id: 'i' + ++S.seq, note: d.note.trim(), state: 'open', maxRating: Number(d.rating), allLibraries: d.allLibs, libs: Object.keys(d.libs).filter((k) => d.libs[k]), canRequest: d.canRequest, until: Date.now() + 7 * D });
  save();
  d.link = `http://slouchcat.local/invite/${tok}`;
  ui.dialog.fresh = true;
  render();
};
ACT.abInviteCopy = () => {
  const v = ui.draft.invite.link;
  const ok = () => toast('Link kopiert.', 'ok', 2500);
  const no = () => { const el = document.getElementById('iv:link'); if (el) el.select(); toast('Kopieren nicht möglich. Der Link ist markiert, kopiere ihn mit Strg+C.', 'warn'); };
  try { navigator.clipboard.writeText(v).then(ok, no); } catch (e) { no(); }
};
ACT.abInviteDone = () => { closeDialog(); ui.filter.accTab = 'invitations'; render(); };
DIALOGS.abRevoke = ({ id }) => ({ title: 'Einladung widerrufen?', body: '<p class="lead">Der Link funktioniert danach nicht mehr.</p>', actions: `<button type="button" class="btn ghost" data-act="close" data-fid="rv:x">Abbrechen</button><button type="button" class="btn danger" data-act="abRevokeOk" data-id="${id}" data-fid="rv:ok" data-autofocus>Widerrufen</button>` });
ACT.abRevokeOk = (d) => { const inv = S.invitations.find((i) => i.id === d.id); inv.state = 'revoked'; closeDialog(); commit(); toast(`Einladung «${esc(inv.note || 'Ohne Notiz')}» widerrufen.`, 'ok'); };

/* ================= M-27 Spotlight ================= */
const AB_LISTS = [['trending', 'Trending bei Trakt'], ['popular', 'Beliebt bei Trakt'], ['watched_weekly', 'Meistgesehen (letzte Woche)'], ['collected_monthly', 'Meistgesammelt (letzter Monat)'], ['anticipated', 'Am meisten erwartet'], ['boxoffice', 'Kinocharts'], ['url', 'Öffentliche Liste per Adresse']];
const AB_MEDIA = [['movie', 'Nur Filme'], ['show', 'Nur Serien'], ['both', 'Filme und Serien']];
const AB_SPOT_SRC = ['enabled', 'listKey', 'listUrl', 'media', 'itemCount', 'minHits', 'intervalHours', 'rotationSeconds'];
const AB_SPOT_MISS = ['showMissing', 'maxMissing', 'missingOrder'];
const AB_SPOT_NUM = { itemCount: [5, 20], minHits: [0, 10], intervalHours: [6, 48], rotationSeconds: [3, 15], maxMissing: [0, 10] };
function abSpotDraft() {
  if (!ui.draft.spot) { const c = S.settings.spotlight; ui.draft.spot = Object.fromEntries([...AB_SPOT_SRC, ...AB_SPOT_MISS].map((k) => [k, c[k] ?? (k === 'listUrl' ? '' : null)])); }
  return ui.draft.spot;
}
function abSpotDirty(keys) { const d = abSpotDraft(), c = S.settings.spotlight; return keys.some((k) => String(d[k] ?? '') !== String(c[k] ?? '')); }
function abSpotErr(k) {
  const d = abSpotDraft();
  if (AB_SPOT_NUM[k]) { const [lo, hi] = AB_SPOT_NUM[k], v = d[k]; if (v === '' || v == null || !Number.isInteger(Number(v)) || v < lo || v > hi) return `Erlaubt sind ${lo} bis ${hi}.`; }
  if (k === 'listUrl' && d.listKey === 'url' && !/^https:\/\/trakt\.tv\/users\/[^/\s]+\/lists\/[^/\s]+/.test(d.listUrl || '')) return 'Gib die Adresse einer öffentlichen Liste auf trakt.tv ein.';
  return '';
}
function abSpotNum(k, label, hint, extra) {
  const d = abSpotDraft(), [lo, hi] = AB_SPOT_NUM[k], err = ui.abSpotShowErr ? abSpotErr(k) : '';
  return abSrow(label, hint + (extra || ''), `<input class="inp num" type="number" min="${lo}" max="${hi}" aria-label="${esc(label)}" data-bind="ui.draft.spot.${k}" data-num data-live data-fid="sp:${k}" value="${esc(d[k] ?? '')}" ${err ? 'aria-invalid="true"' : ''}>${err ? `<span class="err">${err}</span>` : ''}`, { isNew: false });
}
function abSpotList() { const c = S.settings.spotlight; return SPOT_ORDER.map((k) => T[k]).filter((t) => c.media === 'both' || (c.media === 'movie' ? t.kind === 'movie' : t.kind === 'series')); }
function abSpotFacts() {
  const c = S.settings.spotlight, ov = c.overrides || {};
  const list = abSpotList();
  const hits = list.filter((t) => isPresent(t.key)).length;
  const miss = list.filter((t) => !isPresent(t.key) && !(ov[t.key] && ov[t.key].hidden));
  const req = miss.filter((t) => openWishes(t.key).length);
  return { hits, entries: list.length, miss, req };
}
function abSourceBand() {
  const d = abSpotDraft(), c = S.settings.spotlight, f = abSpotFacts();
  const dirty = abSpotDirty(AB_SPOT_SRC);
  const fixed = d.listKey === 'boxoffice';
  const err = ui.abSpotShowErr ? abSpotErr('listUrl') : '';
  const radios = (name, opts, path, cur, o = {}) => `<div class="ab-radios" role="radiogroup" aria-label="${esc(o.label)}">${opts.map(([v, l]) => `<label class="check"><input type="radio" name="${name}" value="${v}" data-bind="${path}" ${o.after ? `data-after="${o.after}"` : ''} data-fid="sp:${name}:${v}" ${cur === v ? 'checked' : ''} ${o.disabled ? 'disabled' : ''}><span>${l}</span></label>`).join('')}</div>`;
  const shown = c.enabled && f.hits >= c.minHits;
  const why = !c.enabled ? 'Spotlight ist ausgeschaltet: Die Reihe fehlt auf der Startseite.' : f.hits < c.minHits ? `Nur ${f.hits} Treffer in der Bibliothek, weniger als die verlangten ${c.minHits}: Die Reihe fehlt auf der Startseite. Profile mit weniger sichtbaren Titeln können sie auch sonst nicht bekommen.` : '';
  const fetched = S.settings.spotlight.fetchedAt || S.seerr.lastSync - 3 * H;
  const body = `${abSrow('Auf der Startseite zeigen', 'Gilt erst mit «Speichern», zusammen mit der Quelle.', sw('ui.draft.spot.enabled', 'Auf der Startseite zeigen'), { isNew: false })}
    ${abSrow('Trakt-Liste', 'Öffentliche Listen, kein Trakt-Login. <a class="lnk" href="#/admin/integrations" data-fid="sp:keys">Schlüssel unter Integrationen</a>', radios('list', AB_LISTS, 'ui.draft.spot.listKey', d.listKey, { label: 'Trakt-Liste', after: 'abSpotList' }), { isNew: false, col: true })}
    ${d.listKey === 'url' ? abSrow('Adresse der Liste', 'Eine öffentliche Liste auf trakt.tv, etwa https://trakt.tv/users/name/lists/name-der-liste. Merklisten (Watchlists) gehen nicht.', `<input class="inp" type="url" maxlength="500" aria-label="Adresse der Liste" data-bind="ui.draft.spot.listUrl" data-live data-fid="sp:url" value="${esc(d.listUrl || '')}" placeholder="https://trakt.tv/users/…/lists/…" ${err ? 'aria-invalid="true"' : ''}>${err ? `<span class="err">${err}</span>` : ''}`, { isNew: false, col: true }) : ''}
    ${abSrow('Medienart', fixed ? 'Diese Liste gibt die Medienart vor.' : d.listKey === 'url' ? 'Die Liste mischt Filme und Serien: Die Medienart filtert ihre Einträge nach dem Abruf.' : '', radios('media', AB_MEDIA, 'ui.draft.spot.media', fixed ? 'movie' : d.media, { label: 'Medienart', disabled: fixed }), { isNew: false, col: true })}
    ${abSpotNum('itemCount', 'Höchstens', 'Titel in der Reihe, je Profil (5–20)')}
    ${abSpotNum('minHits', 'Ausblenden unter', 'Treffern in der Bibliothek, je Profil (0–10).', ' <b class="ab-newi">Fehlende Titel zählen nicht. <span class="newtag">NEU</span></b>')}
    ${abSpotNum('intervalHours', 'Abruf alle … Stunden', '6 bis 48 Stunden')}
    ${abSpotNum('rotationSeconds', 'Spotlight Wechsel Takt', '3 bis 15 Sekunden')}
    <div class="ab-fetch"><p class="ab-line">${ui.busy.spot ? '<span class="spin"></span>Die Liste wird abgerufen …' : `${icon('clock', 16)}<span>Zuletzt abgerufen ${fmtDate(fetched)}, ${fmtTime(fetched)} · ${f.hits} von ${f.entries} Titeln in der Bibliothek · Schlüssel gesetzt</span>`}</p>${why ? `<p class="sec ab-small">${why}</p>` : ''}</div>
    <div class="foot"><button type="button" class="btn secondary" data-act="abSpotFetch" data-fid="sp:fetch" ${ui.busy.spot ? 'disabled' : ''}>${icon('refresh', 16)}Jetzt abrufen</button><button type="button" class="btn primary" data-act="abSpotSave" data-fid="sp:save1">Speichern</button></div>`;
  const meta = `${dirty ? '<span class="chip warn">Ungespeicherte Änderungen</span>' : ''}<span>${esc((AB_LISTS.find((x) => x[0] === c.listKey) || AB_LISTS[0])[1])} · ${esc((AB_MEDIA.find((x) => x[0] === c.media) || AB_MEDIA[2])[1])}</span><span class="chip ${shown ? 'ok' : 'warn'}">${shown ? 'Auf der Startseite' : 'Fehlt auf der Startseite'}</span>`;
  return band('spot:src', 'Quelle', meta, body, { icon: 'list' });
}
function abMissingBand() {
  const d = abSpotDraft(), c = S.settings.spotlight, w = S.settings.wishes, ss = seerrState(), f = abSpotFacts();
  const dirty = abSpotDirty(AB_SPOT_MISS);
  const p = curProfile(), cr = canRequest(p);
  const seen = spotlightFor(p).filter((t) => !isPresent(t.key)).length;
  let why = '';
  if (!c.showMissing) why = 'Fehlende Titel sind ausgeschaltet.';
  else if (!w.enabled) why = 'Wünsche sind ausgeschaltet.';
  else if (!w.fromSpotlight) why = 'Anfordern aus dem Spotlight ist ausgeschaltet.';
  else if (!cr.ok) why = cr.text;
  const warn = [];
  if (!w.enabled || !w.fromSpotlight) warn.push(notice('warn', !w.enabled ? 'Wünsche sind ausgeschaltet' : 'Anfordern aus dem Spotlight ist aus', 'Solange das so ist, zeigt das Spotlight keine fehlenden Titel, auch wenn der Schalter hier an ist.', `<a class="btn sm secondary" href="#/admin/wishes/settings" data-fid="sp:wset">${icon('settings', 16)}Zu den Einstellungen der Wünsche</a>`));
  if (ss.k === 'none' || ss.k === 'off') warn.push(notice('warn', `Jellyseerr: ${ss.label}`, 'Ohne Jellyseerr erscheinen keine fehlenden Titel.', `<a class="btn sm secondary" href="#/admin/integrations/jellyseerr" data-fid="sp:conn">${icon('plug', 16)}Zur Verbindung</a>`));
  const body = `${warn.join('')}
    ${abSrow('Fehlende Titel zeigen', 'Nur für Profile, die anfordern dürfen. Die erste Folie ist immer ein vorhandener Titel.', sw('ui.draft.spot.showMissing', 'Fehlende Titel zeigen'))}
    ${(() => { const k = 'maxMissing', err = ui.abSpotShowErr ? abSpotErr(k) : ''; return abSrow('Höchstens fehlende Titel', 'Unter den höchstens ' + (d.itemCount || c.itemCount) + ' Titeln der Reihe (0–10).', `<input class="inp num" type="number" min="0" max="10" aria-label="Höchstens fehlende Titel" data-bind="ui.draft.spot.maxMissing" data-num data-live data-fid="sp:maxMissing" value="${esc(d.maxMissing ?? '')}" ${!d.showMissing ? 'disabled' : ''} ${err ? 'aria-invalid="true"' : ''}>${err ? `<span class="err">${err}</span>` : ''}`); })()}
    ${abSrow('Reihenfolge', d.missingOrder === 'after' ? 'Erst alle vorhandenen Titel, dann die fehlenden.' : 'Fehlende Titel stehen an ihrem Platz in der Trakt-Liste.', abSeg('ui.draft.spot.missingOrder', [['trakt', 'Wie bei Trakt'], ['after', 'Nach den vorhandenen']], { label: 'Reihenfolge', disabled: !d.showMissing }))}
    <div class="ab-fetch"><p class="ab-line">${icon('chart', 16)}<span>${abCount(f.miss.length, 'Eintrag', 'Einträge')} der Liste ${f.miss.length === 1 ? 'fehlt' : 'fehlen'} in der Bibliothek, ${f.req.length} davon ${f.req.length === 1 ? 'ist' : 'sind'} angefordert. <b>${esc(p.name)}</b> sähe zurzeit ${seen} fehlende ${seen === 1 ? 'Titel' : 'Titel'} im Spotlight${why ? ` (${esc(why)})` : ''}.</span></p>${dirty ? '<p class="sec ab-small">Gezählt mit dem gespeicherten Stand. Speichere, damit deine Änderung zählt.</p>' : ''}<p class="muted ab-small">Das aktive Profil wechselst du über die Profilwahl.</p></div>
    <div class="foot"><button type="button" class="btn primary" data-act="abSpotSave" data-fid="sp:save2">Speichern</button></div>`;
  const meta = `${dirty ? '<span class="chip warn">Ungespeicherte Änderungen</span>' : ''}<span>${c.showMissing ? `An · höchstens ${c.maxMissing} · ${c.missingOrder === 'after' ? 'nach den vorhandenen' : 'wie bei Trakt'}` : 'Aus'}</span>`;
  return band('spot:miss', 'Fehlende Titel <span class="newtag">NEU</span>', meta, body, { icon: 'wish' });
}
function abOv(key) { const o = S.settings.spotlight.overrides; return (o[key] = o[key] || {}); }
function abOvClean(key) { const o = S.settings.spotlight.overrides; if (o[key] && !o[key].hidden && !o[key].pinned && !o[key].note) delete o[key]; }
function abEntriesBand() {
  const ov = S.settings.spotlight.overrides || {};
  const rows = SPOT_ORDER.map((k, i) => {
    const t = T[k], o = ov[k] || {}, has = isPresent(k), w = openWishes(k)[0];
    const st = o.hidden ? '<span class="chip info">Ausgeblendet</span>' : has ? '<span class="chip ok">Sichtbar</span>' : w ? wsChip(w.state, 'Angefordert · ' + STATE_LABEL[w.state]) : missingChip();
    const acts = [
      o.pinned ? `<button type="button" class="btn sm quiet" data-act="abUnpin" data-key="${k}" aria-label="${esc(t.title)} lösen" data-fid="se:up:${k}">${icon('pin', 16)}Lösen</button>` : `<button type="button" class="btn sm quiet" data-act="abPinAsk" data-key="${k}" aria-label="${esc(t.title)} anheften" data-fid="se:pin:${k}">${icon('pin', 16)}Anheften</button>`,
      `<button type="button" class="btn sm quiet" data-act="abHide" data-key="${k}" aria-label="${esc(t.title)} ${o.hidden ? 'einblenden' : 'ausblenden'}" data-fid="se:hd:${k}">${icon(o.hidden ? 'eye' : 'ban', 16)}${o.hidden ? 'Einblenden' : 'Ausblenden'}</button>`,
    ];
    if (!has && !w) acts.push(`<button type="button" class="btn sm secondary" data-act="askWish" data-key="${k}" aria-label="${esc(t.title)} anfordern" data-fid="se:rq:${k}">${icon('plus', 16)}Anfordern</button>`);
    return `<tr class="${has ? '' : 'ab-missrow'}"><td class="nowrap">${i + 1}</td><td class="main">${abMini(t, `${kindLabel(t)} · ${t.year}`)}</td><td><span class="ab-who">${st}${o.pinned ? `<span>Angeheftet auf Platz ${o.pinned}</span>` : ''}</span></td><td><span class="ab-note">${o.note ? `«${esc(o.note)}»` : '–'}<button type="button" class="btn sm quiet" data-act="abNoteAsk" data-key="${k}" aria-label="Notiz zu ${esc(t.title)}" data-fid="se:nt:${k}">${icon('note', 16)}Notiz</button></span></td><td class="end"><span class="ta">${acts.join('')}</span></td></tr>`;
  }).join('');
  const nMiss = SPOT_ORDER.filter((k) => !isPresent(k)).length;
  const body = `<p class="sec ab-small">Reihenfolge von Trakt; eigene Änderungen bleiben beim nächsten Abruf erhalten. Fehlende Titel haben neu ein Poster, ihren Zustand und die Aktion «Anfordern» <span class="newtag">NEU</span>. «Anfordern» handelt als aktives Profil (${esc(curProfile().name)}).</p>
    <div class="tblw"><table class="tbl" aria-label="Einträge der Liste"><thead><tr><th>Rang</th><th>Titel</th><th>Status</th><th>Eigene Notiz</th><th class="end">Aktionen</th></tr></thead><tbody>${rows}</tbody></table></div>`;
  return band('spot:entries', 'Einträge der Liste', `<span>${abCount(SPOT_ORDER.length, 'Eintrag', 'Einträge')}, ${nMiss} fehlen</span>`, body, { icon: 'film' });
}
const AB_ORPHANS = [{ key: 'wicked-for-good', title: 'Wicked: For Good', meta: 'Film · 2025', what: 'Angeheftet auf Platz 3 · Notiz «Musical-Abend»' }];
function abOrphansBand() {
  const list = S.settings.spotOrphans ?? AB_ORPHANS;
  if (!list.length) return '';
  const body = `<p class="sec ab-small">Anpassungen an Titeln, die nicht mehr in der Liste stehen. Sie wirken erst wieder, wenn der Titel zurückkehrt.</p><div class="tblw"><table class="tbl"><thead><tr><th>Titel</th><th>Anpassung</th><th class="end">Aktionen</th></tr></thead><tbody>${list.map((o) => `<tr><td class="main">${esc(o.title)}<span class="muted ab-small"> · ${esc(o.meta)}</span></td><td>${esc(o.what)}</td><td class="end"><button type="button" class="btn sm dq" data-act="abOrphanRm" data-key="${o.key}" aria-label="Anpassung für ${esc(o.title)} entfernen" data-fid="so:rm:${o.key}">${icon('trash', 16)}Entfernen</button></td></tr>`).join('')}</tbody></table></div>`;
  return band('spot:orph', 'Verwaiste Anpassungen', `<span>${abCount(list.length, 'Anpassung', 'Anpassungen')}</span>`, body, { icon: 'alert' });
}
route('/admin/spotlight', () => {
  abSpotDraft();
  const html = `${notice('', 'Neu: fehlende Titel im Spotlight', 'Bisher galt «Gezeigt werden nur Titel, die in der eigenen Bibliothek stehen»; neu kann das Spotlight auch fehlende Titel zum Anfordern zeigen, wenn du es im Band «Fehlende Titel» einschaltest.')}
    ${abSourceBand()}${abMissingBand()}${abEntriesBand()}${abOrphansBand()}
    <p class="sec ab-small">Wo die Reihe auf der Startseite steht und ob sie erscheint, stellst du unter «Startseite» ein. <a class="lnk" href="#/admin/home-rows" data-fid="sp:rows">Zur Seite «Startseite»</a></p>`;
  return { type: 'admin', area: 'home', page: 'spotlight', title: 'Spotlight', intro: 'Eine öffentliche Trakt-Liste als Reihe der Startseite. Gezeigt werden Titel aus der eigenen Bibliothek und, wenn eingeschaltet, fehlende Titel zum Anfordern; jedes Profil sieht nur, was seine Altersstufe erlaubt.', html };
});
ACT.abSpotList = () => { const d = abSpotDraft(); if (d.listKey === 'boxoffice') d.media = 'movie'; };
ACT.abSpotSave = () => {
  const d = abSpotDraft();
  const errs = [...Object.keys(AB_SPOT_NUM), 'listUrl'].map(abSpotErr).filter(Boolean);
  if (errs.length) { ui.abSpotShowErr = true; ui.open['spot:src'] = true; if (abSpotErr('maxMissing')) ui.open['spot:miss'] = true; render(); toast('Nicht gespeichert: Prüfe die markierten Felder.', 'bad'); return; }
  ui.abSpotShowErr = false;
  if (d.listKey === 'boxoffice') d.media = 'movie';
  const c = S.settings.spotlight;
  [...AB_SPOT_SRC, ...AB_SPOT_MISS].forEach((k) => { c[k] = AB_SPOT_NUM[k] ? Number(d[k]) : d[k]; });
  delete ui.draft.spot;
  ui.hero = { i: 0, paused: false };
  commit();
  toast('Spotlight ist gespeichert.', 'ok');
};
ACT.abSpotFetch = () => {
  ui.busy.spot = true; render();
  toast('Die Liste wird abgerufen.', 'ok', 2500);
  setTimeout(() => { ui.busy.spot = false; S.settings.spotlight.fetchedAt = Date.now(); commit(); toast(`Liste abgerufen: ${SPOT_ORDER.length} Einträge, ${abSpotFacts().hits} in der Bibliothek.`, 'ok'); }, 1600);
};
ACT.abPinAsk = (d) => {
  const n = S.settings.spotlight.itemCount, taken = Object.values(S.settings.spotlight.overrides).map((o) => o.pinned).filter(Boolean);
  let free = 1; while (taken.includes(free) && free <= n) free++;
  ui.draft.pin = { key: d.key, rank: free <= n ? free : '', err: '' };
  openDialog('abPin', { key: d.key });
};
DIALOGS.abPin = ({ key }) => {
  const t = T[key], dr = ui.draft.pin, ov = S.settings.spotlight.overrides, n = S.settings.spotlight.itemCount;
  const taken = Object.entries(ov).filter(([k, o]) => o.pinned && k !== key).sort((a, b) => a[1].pinned - b[1].pinned);
  const all = taken.length >= n;
  const hint = '1 ist der erste Platz. Der Platz gilt für die ganze Liste; ein Profil mit weniger Treffern sieht den Titel früher.';
  return {
    title: `${esc(t.title)} anheften`,
    body: all ? notice('warn', 'Alle Plätze sind belegt; löse zuerst einen.', '') : `<div class="fld"><label for="pn:rank">Platz in der Reihe</label><input id="pn:rank" class="inp num" type="number" min="1" max="${n}" data-bind="ui.draft.pin.rank" data-num data-fid="pn:rank" value="${esc(dr.rank)}" data-autofocus ${dr.err ? 'aria-invalid="true"' : ''}><span class="hint">${hint}${taken.length ? ` Belegt: ${taken.map(([k, o]) => `Platz ${o.pinned} (${esc(T[k] ? T[k].title : k)})`).join(', ')}.` : ''}</span>${dr.err ? `<span class="err">${dr.err}</span>` : ''}</div>`,
    actions: `<button type="button" class="btn ghost" data-act="close" data-fid="pn:x">Abbrechen</button>${all ? '' : `<button type="button" class="btn primary" data-act="abPinOk" data-fid="pn:ok">${icon('pin', 16)}Anheften</button>`}`,
  };
};
ACT.abPinOk = () => {
  const dr = ui.draft.pin, n = S.settings.spotlight.itemCount, r = Number(dr.rank), ov = S.settings.spotlight.overrides;
  const by = Object.entries(ov).find(([k, o]) => o.pinned === r && k !== dr.key);
  if (!Number.isInteger(r) || r < 1 || r > n) dr.err = `Erlaubt sind 1 bis ${n}.`;
  else if (by) dr.err = `Platz ${r} ist belegt (${esc(T[by[0]] ? T[by[0]].title : by[0])}).`;
  else dr.err = '';
  if (dr.err) { renderOverlays(); return; }
  abOv(dr.key).pinned = r;
  closeDialog(); commit();
  toast(`${esc(T[dr.key].title)} ist auf Platz ${r} angeheftet.`, 'ok');
};
ACT.abUnpin = (d) => { delete abOv(d.key).pinned; abOvClean(d.key); commit(); toast(`${esc(T[d.key].title)} ist nicht mehr angeheftet.`, 'ok'); };
ACT.abHide = (d) => { const o = abOv(d.key); o.hidden = !o.hidden; const h = o.hidden; if (!h) delete o.hidden; abOvClean(d.key); commit(); toast(h ? `${esc(T[d.key].title)} ist ausgeblendet.` : `${esc(T[d.key].title)} wird wieder gezeigt.`, 'ok'); };
ACT.abNoteAsk = (d) => { ui.draft.note = { key: d.key, text: (S.settings.spotlight.overrides[d.key] || {}).note || '' }; openDialog('abNote', { key: d.key }); };
DIALOGS.abNote = ({ key }) => ({
  title: `Notiz zu ${esc(T[key].title)}`,
  body: `<div class="fld"><label for="nt:text">Notiz</label><input id="nt:text" class="inp" maxlength="200" data-bind="ui.draft.note.text" data-fid="nt:text" data-enter="abNoteOk" value="${esc(ui.draft.note.text)}" data-autofocus><span class="hint">Höchstens 200 Zeichen. Leer entfernt die Notiz.</span></div>`,
  actions: `<button type="button" class="btn ghost" data-act="close" data-fid="nt:x">Abbrechen</button><button type="button" class="btn primary" data-act="abNoteOk" data-fid="nt:ok">Speichern</button>`,
});
ACT.abNoteOk = () => {
  const dr = ui.draft.note, txt = dr.text.trim(), o = abOv(dr.key);
  if (txt) o.note = txt; else delete o.note;
  abOvClean(dr.key);
  closeDialog(); commit();
  toast(txt ? `Die Notiz zu ${esc(T[dr.key].title)} ist gespeichert.` : `Die Notiz zu ${esc(T[dr.key].title)} ist entfernt.`, 'ok');
};
ACT.abOrphanRm = (d) => {
  const list = S.settings.spotOrphans ?? AB_ORPHANS, o = list.find((x) => x.key === d.key);
  S.settings.spotOrphans = list.filter((x) => x.key !== d.key);
  commit();
  toast(`Die Anpassung für ${esc(o.title)} ist entfernt.`, 'ok');
};

/* ================= M-28 Startseite ================= */
const AB_ROW_DESC = {
  spotlight: ['Die Trakt-Liste, geschnitten mit der Bibliothek. Erscheint als Bühne mit Hintergrundbild an ihrem Platz; unter der Mindestzahl Treffer fehlt sie. Fehlende Titel nur, wenn unter «Spotlight» eingeschaltet.', '#/admin/spotlight', 'Spotlight'],
  picks: ['Titel, die Profile des Besitzerkontos empfehlen; neueste zuerst, mit Ablauf.', '#/admin/x/picks', 'User-Empfehlungen'],
  continue: ['Angefangene Titel des Profils.'],
  recent: ['Die zuletzt in eine Bibliothek gekommenen Titel.'],
  released: ['Nach Erscheinungsdatum der letzten zwölf Monate; eine Serie nach ihrer neusten Folge.'],
  watchlist: ['Die Merkliste des Profils.'],
  collections: ['Eigene Sammlungen und die von TMDB.', '#/admin/x/collections', 'Sammlungen'],
  genres: ['Die Genres der sichtbaren Titel.'],
  libraries: ['Kacheln für die Bibliotheken, die das Profil sieht, für Radio und, wenn eingeblendet, für Sammlungen und User-Empfehlungen; Reihenfolge und Ausblenden regelt die Seite «Meine Medien».', '#/admin/x/my-media', 'Meine Medien'],
  wishes: ['Offene und frisch erfüllte Wünsche des Profils. Erscheint nur bei Profilen, die Wünsche haben.', '#/admin/wishes', 'Wünsche'],
};
const AB_MAX_ROWS = 15;
function abRowsDraft() {
  if (!ui.draft.rows) {
    const r = S.settings.homeRows.map((x) => ({ ...x }));
    if (!r.some((x) => x.key === 'wishes')) { const i = r.findIndex((x) => x.key === 'watchlist'); r.splice(i + 1, 0, { key: 'wishes', hidden: false }); }
    ui.draft.rows = r;
  }
  return ui.draft.rows;
}
function abRowsDirty() { return JSON.stringify(abRowsDraft()) !== JSON.stringify(S.settings.homeRows); }
route('/admin/home-rows', () => {
  const rows = abRowsDraft(), n = rows.length, vis = rows.filter((r) => !r.hidden);
  const tr = rows.map((r, i) => {
    const open = isOpen('hr:' + r.key), desc = AB_ROW_DESC[r.key] || [''];
    const isNew = r.key === 'wishes';
    return `<tr class="${isNew ? 'ab-newrow' : ''}"><td><select class="sel ab-pos" data-bind="ui.abRowPos.${r.key}" data-after="abRowMove" data-rerender aria-label="Platz von ${esc(HOME_TITLES[r.key])}" data-fid="hr:pos:${r.key}">${rows.map((_, j) => `<option value="${j + 1}" ${j === i ? 'selected' : ''}>${j + 1}</option>`).join('')}</select></td>
      <td class="main"><button type="button" class="tog" data-act="open" data-id="hr:${r.key}" aria-expanded="${open}" data-fid="hr:tog:${r.key}"><span class="chev">${icon('chevR', 16)}</span>${esc(HOME_TITLES[r.key])}</button>${isNew ? ' <span class="newtag">NEU</span>' : ''}</td>
      <td>${r.hidden ? '<span class="chip info">ausgeblendet</span>' : '<span class="chip ok">sichtbar</span>'}</td>
      <td><label class="check"><input type="checkbox" data-bind="ui.draft.rows.${i}.hidden" data-after="abRowHide" data-fid="hr:hd:${r.key}" ${r.hidden ? 'checked' : ''}><span>Ausblenden<span class="vh"> ${esc(HOME_TITLES[r.key])}</span></span></label></td></tr>
      ${open ? `<tr class="sub"><td colspan="4"><p class="sec">${esc(desc[0])}</p>${desc[1] ? `<p style="margin-top:8px"><a class="lnk" href="${desc[1]}" data-fid="hr:link:${r.key}">Zur Seite «${desc[2]}»</a></p>` : ''}</td></tr>` : ''}`;
  }).join('');
  const table = `<div class="tblw"><table class="tbl" aria-label="Reihen der Startseite"><thead><tr><th>Platz</th><th>Reihe</th><th>Zustand</th><th>Ausblenden</th></tr></thead><tbody>${tr}</tbody></table></div><p class="muted ab-small">Höchstens ${AB_MAX_ROWS} Reihen sichtbar.</p>`;
  const prev = `<p class="sec ab-small">Schema mit Platzhaltern, in der ungespeicherten Reihenfolge. Auf der echten Startseite fehlen ausgeblendete Reihen, und leere Reihen entfallen ebenfalls (zum Beispiel eine leere Merkliste oder «Deine Wünsche» bei Profilen ohne Wünsche).</p>
    <ol class="ab-prev" aria-label="Vorschau der Startseite">${vis.map((r, i) => `<li class="${r.key === 'spotlight' ? 'stage' : ''}"><b>${i + 1}. ${esc(HOME_TITLES[r.key])}</b><span class="ab-ph" aria-hidden="true">${r.key === 'spotlight' ? '<i class="big"></i>' : '<i></i>'.repeat(6)}</span></li>`).join('')}</ol>`;
  const dirty = abRowsDirty();
  const bar = dirty ? `<div class="savebar"><span class="grow">Ungespeicherte Änderungen</span><button type="button" class="btn quiet" data-act="dialog" data-name="abRowsDiscard" data-fid="hr:discard">Verwerfen</button><button type="button" class="btn primary" data-act="abRowsSave" data-fid="hr:save">Speichern</button></div>` : '';
  const html = `${band('hr:rows', 'Reihen', `<span>${abCount(n, 'Reihe', 'Reihen')}</span>${dirty ? '<span class="chip warn">Ungespeicherte Änderungen</span>' : ''}`, table, { icon: 'list' })}${band('hr:prev', 'Vorschau', `<span>${vis.length} sichtbar</span>`, prev, { icon: 'eye' })}
    <p class="sec ab-small">Welche Trakt-Liste Spotlight zeigt, wann sie zuletzt abgerufen wurde und ob Trakt eingerichtet ist, steht unter «Spotlight». <a class="lnk" href="#/admin/spotlight" data-fid="hr:spot">Zur Seite «Spotlight»</a></p>${bar}`;
  return { type: 'admin', area: 'home', page: 'home-rows', title: 'Startseite', intro: 'Reihenfolge der Reihen auf der Startseite, für alle Profile. Ausgeblendete und leere Reihen erscheinen nicht.', html };
});
ACT.abRowMove = (d, el) => {
  const key = d.bind.split('.').pop(), rows = abRowsDraft(), to = Number(el.value) - 1;
  const from = rows.findIndex((r) => r.key === key);
  const [r] = rows.splice(from, 1);
  rows.splice(to, 0, r);
  toast(`${esc(HOME_TITLES[key])} steht jetzt auf Platz ${to + 1}. Speichere, damit es gilt.`, 'ok');
};
ACT.abRowHide = (d) => {
  const i = Number(d.bind.split('.')[3]), rows = abRowsDraft();
  const vis = rows.filter((r) => !r.hidden).length;
  if (!rows[i].hidden && vis > AB_MAX_ROWS) { rows[i].hidden = true; toast(`Schon ${AB_MAX_ROWS} Reihen sichtbar. Blende zuerst eine andere aus.`, 'warn'); }
};
ACT.abRowsSave = () => { S.settings.homeRows = abRowsDraft().map((r) => ({ ...r })); delete ui.draft.rows; commit(); toast('Die Startseite ist gespeichert.', 'ok'); };
DIALOGS.abRowsDiscard = () => ({ title: 'Änderungen verwerfen?', body: '<p class="lead">Die ungespeicherte Reihenfolge und das Ausblenden gehen verloren; es gilt wieder, was gespeichert ist.</p>', actions: `<button type="button" class="btn ghost" data-act="close" data-fid="rx:x">Abbrechen</button><button type="button" class="btn danger" data-act="abRowsDiscardOk" data-fid="rx:ok" data-autofocus>Verwerfen</button>` });
ACT.abRowsDiscardOk = () => { delete ui.draft.rows; closeDialog(); render(); toast('Die Änderungen sind verworfen.', 'ok'); };
