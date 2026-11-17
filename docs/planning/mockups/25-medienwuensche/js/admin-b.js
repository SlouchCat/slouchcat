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
function abWishTabs(active) {
  const wc = wishCounts(), mc = mappingCounts();
  const tab = (k, href, label, extra) => `<a href="${href}" ${k === active ? 'aria-current="page"' : ''} data-fid="awt:${k}">${label}${extra || ''}</a>`;
  return `<nav class="subtabs ab-tabs" aria-label="Bereiche der Wünsche">${tab('requests', '#/admin/wishes', 'Anfragen', `<span class="c ${wc.pending ? 'hot' : ''}" title="${wc.pending ? wc.pending + ' warten auf Freigabe' : 'offen'}">${wc.open}</span>`)}${tab('mapping', '#/admin/wishes/mapping', 'Jellyseerr-Benutzer', mc.missing ? `<span class="c hot">${mc.missing} ohne</span>` : '')}${tab('settings', '#/admin/wishes/settings', 'Einstellungen')}</nav>`;
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
/* «Tag nicht gesetzt»: a tag that could not be set (same rule as the hint on the overview) */
function abUntagged(w) { return wishTags(w).some((x) => x.state === 'fail'); }
function abStalled(w) { const x = dlInfo(w); return !!x && x.k === 'stalled'; }
/* a wish whose account was deleted is closed, not «nicht zuordenbar» */
function abUnassigned(w) { return !w.owner && !w.goneAccount; }
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
  ['unassigned', 'Nicht zuordenbar', abUnassigned],
  ['notag', 'Tag nicht gesetzt', abUntagged],
  ['stalled', 'Download verzögert', abStalled],
];
const AB_WF_EMPTY = { all: 'Noch keine Wünsche.', open: 'Kein offener Wunsch.', unassigned: 'Keine Anfrage ohne Zuordnung. Alle Anfragen gehören zu einem Profil.', notag: 'Jeder Profil-Tag ist gesetzt oder folgt noch.', stalled: 'Kein Download ist verzögert.' };
/* a link may carry the filter (`#/admin/wishes?f=stalled`, `?f=notag`, `?state=pending`): set it once, then drop it from the address */
function abTakeQuery(hash, apply) {
  if (!apply()) return;
  if (typeof history !== 'undefined' && history.replaceState) history.replaceState(null, '', hash);
}

function abWishRow(w) {
  const t = T[w.titleKey], id = w.id, open = isOpen('aw:' + id);
  const p = w.owner ? profile(w.owner) : null;
  const joined = w.joined.map((j) => profile(j)).filter(Boolean);
  const who = p
    ? `<span class="ab-who"><b>${esc(account(p.accountId).email)} · ${esc(p.name)}</b>${joined.length ? `<span>+ ${joined.map((x) => esc(x.name)).join(', ')}</span>` : ''}${wishTags(w).length ? `<span class="ab-chips">${tagChips(w)}</span>` : ''}</span>`
    : w.goneAccount ? `<span class="ab-who"><span class="chip outline">${icon('trash', 12)}Konto gelöscht</span><span>${esc(w.goneAccount)}</span></span>`
    : `<span class="chip warn">${icon('alert', 12)}Nicht zuordenbar</span>`;
  const su = `<span class="ab-who"><b>${esc(abUserName(w.seerrUser))}</b>${w.origin === 'jellyseerr' ? '<span class="chip info">In Jellyseerr angelegt</span>' : ''}</span>`;
  const acts = [];
  const assign = `#/admin/wishes/mapping?user=${w.seerrUser}`;
  if (abUnassigned(w)) {
    /* a request without account: only «In Jellyseerr öffnen» and «Benutzer zuordnen» (07, section 5) */
    acts.push(`<button type="button" class="btn sm quiet" data-act="dialog" data-name="abSeerrOpen" data-id="${id}" data-fid="aw:js:${id}">${icon('external', 16)}In Jellyseerr öffnen</button>`);
    acts.push(`<a class="btn sm secondary" href="${assign}" data-fid="aw:as:${id}">${icon('users', 16)}Benutzer zuordnen</a>`);
  } else {
    if (w.state === 'pending') {
      acts.push(`<button type="button" class="btn sm primary" data-act="abApprove" data-id="${id}" data-fid="aw:ap:${id}">${icon('check', 16)}Freigeben</button>`);
      acts.push(`<button type="button" class="btn sm dq" data-act="abDeclineAsk" data-id="${id}" data-fid="aw:de:${id}">${icon('ban', 16)}Ablehnen</button>`);
    }
    if ((w.state === 'queued' || w.state === 'failed') && !w.goneAccount) acts.push(`<button type="button" class="btn sm secondary" data-act="abRetry" data-id="${id}" data-fid="aw:re:${id}">${icon('refresh', 16)}Erneut übermitteln</button>`);
    acts.push(`<button type="button" class="btn sm quiet" data-act="dialog" data-name="abSeerrOpen" data-id="${id}" data-fid="aw:js:${id}">${icon('external', 16)}In Jellyseerr öffnen</button>`);
    acts.push(`<a class="btn sm quiet" href="${titleHref(t)}" data-fid="aw:ti:${id}">${icon('chevR', 16)}Zum Titel</a>`);
  }
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
    const tagEv = w.events.filter((e) => e.k === 'tagged' || e.k === 'tag_failed');
    kv.push(['Profil-Tags <span class="newtag">NEU</span>', !S.settings.wishes.tags.enabled ? 'Ausgeschaltet' : `<span class="ab-chips">${tagChips(w) || 'Kein Profil'}</span>${tagEv.length ? `<ul class="feed ab-tagev">${tagEv.map((e) => `<li class="${e.k === 'tag_failed' ? 'bad' : ''}"><time>${fmtDate(e.at)}, ${fmtTime(e.at)}</time>${esc(e.t)}</li>`).join('')}</ul>` : `<span class="ab-small">${w.seerrRequestId ? 'Noch nicht gesetzt: Der Tag folgt, sobald der Titel in ' + esc(arrOf(w).name) + ' steht.' : 'Folgt nach dem Übermitteln.'}</span>`}`]);
    if (dlInfo(w)) kv.push(['Download <span class="newtag">NEU</span>', dlBlock(w, true)]);
    const un = abUnassigned(w) ? notice('warn', 'Nicht zuordenbar', `Der Jellyseerr-Benutzer «${esc(abUserName(w.seerrUser))}» gehört zu keinem Konto. Die Anfrage erscheint deshalb bei keinem Profil, und niemand wird benachrichtigt. Ordne den Benutzer unter «Jellyseerr-Benutzer» einem Konto zu, dann übernimmt der nächste Abgleich die Anfrage.`, `<a class="btn sm secondary" href="${assign}" data-fid="aw:map:${id}">${icon('users', 16)}Benutzer zuordnen</a>`)
      : w.goneAccount ? notice('', 'Konto gelöscht', `Das Konto ${esc(w.goneAccount)} gibt es nicht mehr. Slouchcat hat den Wunsch geschlossen; in Jellyseerr bleibt die Anfrage bestehen.`) : '';
    sub = `<tr class="sub"><td colspan="6"><div class="ab-subg"><div><h3 class="ab-h3">Verlauf</h3><ol class="steps">${steps}</ol></div><div><h3 class="ab-h3">Diagnose</h3><dl class="kv">${kv.map(([k, v]) => `<dt>${k}</dt><dd>${v}</dd>`).join('')}</dl></div></div>${un}</td></tr>`;
  }
  return `<tr><td class="main"><button type="button" class="tog" data-act="open" data-id="aw:${id}" aria-expanded="${open}" data-fid="aw:tog:${id}"><span class="chev">${icon('chevR', 16)}</span>${abMini(t, abWishSub(w))}</button></td><td>${who}</td><td>${su}</td><td><span class="ab-st">${wsChip(w.state)}${dlChip(w, true)}</span></td><td class="nowrap">${ago(w.created)}</td><td class="end"><span class="ta">${acts.join('')}</span></td></tr>${sub}`;
}

/* «Limit je Konto»: what Jellyseerr counts per account user */
const AB_LIM_ART = { default: 'Standard', custom: 'Eigenes', unlimited: 'Unbegrenzt' };
/* «5 von 5 in 7 Tagen · nächster Platz am 20.11.» */
function abQuotaCell(x) {
  if (!x.limit) return 'ohne Limit';
  const next = x.restricted && x.nextFree ? ` · nächster Platz am ${new Date(x.nextFree).toLocaleDateString('de-CH', { day: 'numeric', month: 'numeric' })}` : '';
  return `<span class="${x.restricted ? 'ab-warnt' : ''}">${x.used} von ${x.limit} in ${x.days} Tagen${next}</span>`;
}
function abQuotaBand() {
  let full = 0;
  const rows = S.accounts.map((a) => {
    const st = accSeerr(a), u = st.user;
    if (!u || u.gone) return '';
    const q = quotaFor(u), adm = q.source === 'admin';
    if (q.movie.restricted || q.tv.restricted) full++;
    const art = adm ? 'Kein Limit (Administrator)' : AB_LIM_ART[(a.limit || {}).mode || 'default'];
    return `<tr><td class="main"><span class="ab-who"><b>${esc(a.email)}</b><span>als ${esc(u.name)}</span></span></td><td>${adm ? '–' : abQuotaCell(q.movie)}</td><td>${adm ? '–' : abQuotaCell(q.tv)}</td><td>${art}</td><td class="end"><a class="lnk ab-small" href="#/admin/accounts?open=${a.id}" data-fid="aw:qacc:${a.id}">Limit ändern</a></td></tr>`;
  }).filter(Boolean);
  const body = rows.length ? `<div class="tblw"><table class="tbl" aria-label="Limit je Konto"><thead><tr><th>Konto</th><th>Filme</th><th>Staffeln</th><th>Art</th><th class="end"><span class="vh">Aktion</span></th></tr></thead><tbody>${rows.join('')}</tbody></table></div>` : '<p class="muted">Kein Konto hat einen Jellyseerr-Benutzer.</p>';
  return band('aw:quota', 'Limit je Konto <span class="newtag">NEU</span>', `<span class="${full ? 'chip bad' : ''}">${abCount(full, 'Konto', 'Konten')} am Limit</span><span>${abCount(rows.length, 'Konto', 'Konten')} mit Jellyseerr-Benutzer</span>`,
    `${body}<p class="muted ab-small">Gezählt wird wie in Jellyseerr, rollierend und auch mit Wünschen, die direkt dort gestellt werden. Abgelehnte Wünsche und Mitwünsche zählen nicht. Stand des letzten Abgleichs. <a class="lnk" href="#/admin/wishes/settings" data-fid="aw:qset">Limit ab Werk</a></p>`, { icon: 'chart' });
}
route('/admin/wishes', (params, query = {}) => {
  abTakeQuery('#/admin/wishes', () => { const qf = query.f || query.state; if (!qf || !AB_WF.some((x) => x[0] === qf)) return false; ui.filter.adminWishes = qf; return true; });
  const f = ui.filter.adminWishes || 'all';
  const wc = wishCounts();
  const st = (k, v, l, n, c) => `<button type="button" class="stat ab-stat ${v && c ? c : ''}" data-act="set" data-path="ui.filter.adminWishes" data-value="${k}" aria-pressed="${f === k}" data-fid="aws:${k}"><span class="v">${v}</span><span class="l">${l}</span>${n ? `<span class="n">${n}</span>` : ''}</button>`;
  const stats = `<div class="stats" style="--n:5">${st('open', wc.open, 'Offen', 'alle offenen Zustände')}${st('pending', wc.pending, 'Freigabe ausstehend', wc.pending ? 'wartet auf dich oder Jellyseerr' : '', 'warn')}${st('processing', wc.processing, 'In Beschaffung', '')}${st('available', wc.available30, 'Verfügbar', 'letzte 30 Tage')}${st('failed', wc.problems, 'Fehler', '', 'bad')}</div>`;
  const pills = AB_WF.map(([k, l, fn]) => {
    const n = S.wishes.filter(fn).length;
    if ((k === 'notag' && !S.settings.wishes.tags.enabled && k !== f) || (k === 'stalled' && !progressOn() && k !== f)) return '';
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
  const html = `${abWishTabs('requests')}${abWishNotices()}${stats}${abQuotaBand()}
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

/* ================= M-24 Wünsche › Jellyseerr-Benutzer ================= */
/* «10 Filme / 7 Tage · 5 Staffeln / 7 Tage» for { movie: {n, days}, tv: {n, days} } or null */
function abLimitShort(cfg) {
  if (!cfg) return 'Ohne Limit';
  const part = (c, one, many) => (c && Number(c.n) ? `${c.n} ${Number(c.n) === 1 ? one : many} / ${c.days} Tage` : `${many} ohne Limit`);
  return `${part(cfg.movie, 'Film', 'Filme')} · ${part(cfg.tv, 'Staffel', 'Staffeln')}`;
}
/* the same for what quotaFor() reports */
function abQuotaCfg(q) { return q.unlimited ? null : { movie: { n: q.movie.limit, days: q.movie.days }, tv: { n: q.tv.limit, days: q.tv.days } }; }
/* «2 / 10 Filme · 1 / 5 Staffeln» */
function abUse(q) {
  if (q.source === 'admin') return 'kein Limit';
  if (q.unlimited) return 'ohne Limit';
  return [q.movie.limit ? `${q.movie.used} / ${q.movie.limit} Filme` : '', q.tv.limit ? `${q.tv.used} / ${q.tv.limit} Staffeln` : ''].filter(Boolean).join(' · ');
}
function abRights(u) {
  const yn = (ok, l) => `<span class="chip ${ok ? 'ok' : 'bad ab-no'}">${icon(ok ? 'check' : 'ban', 12)}${l}</span>`;
  return `<span class="ab-facts">${yn(u.canMovie, 'Filme')}${yn(u.canShow, 'Serien')}<span class="chip ${u.autoApprove ? 'ok' : 'warn'}">${icon(u.autoApprove ? 'truck' : 'hourglass', 12)}Automatische Freigabe: ${u.autoApprove ? 'ja' : 'nein'}</span></span>`;
}
function abUserFacts(u) {
  if (!u) return '';
  const q = quotaFor(u);
  const lim = q.source === 'admin' ? 'Kein Limit (Administrator)' : q.unlimited ? 'Ohne Limit' : 'Limit ' + abLimitShort(abQuotaCfg(q));
  return `<div class="ab-facts" aria-label="Rechte von ${esc(u.name)} in Jellyseerr">${abRights(u)}<span class="chip info">${lim}</span><span class="muted ab-small">${esc(u.type)} · ${esc(u.email)}</span></div>`;
}
function abUserWarn(u) {
  if (!u) return '';
  if (u.gone) return notice('bad', `Benutzer «${esc(u.name)}» nicht gefunden`, 'Jellyseerr kennt diesen Benutzer nicht mehr. Neue Wünsche scheitern mit «Den zugeordneten Jellyseerr-Benutzer gibt es nicht mehr». Wähle einen anderen Benutzer oder eröffne das Konto neu.');
  /* rights warnings only for users that existed before: an opened user's rights follow the account (07, section 6) */
  if (u.origin === 'slouchcat') return '';
  let h = '';
  if (!u.canMovie && !u.canShow) h += notice('warn', `«${esc(u.name)}» darf in Jellyseerr nichts anfordern`, 'Jeder Wunsch scheitert. Gib dem Benutzer in Jellyseerr das Recht «Anfragen» oder wähle einen anderen.');
  else if (!u.canShow) h += notice('warn', `«${esc(u.name)}» darf keine Serien anfordern`, 'Serienwünsche scheitern mit «darf diese Art von Titel nicht anfordern». Filme gehen.');
  return h;
}
/* limit of a Jellyseerr user with use, «davon … direkt» and «Nächster Platz frei …» */
function abQuotaBlock(u, acc) {
  const q = quotaFor(u);
  const link = acc ? `<a class="lnk ab-small" href="#/admin/accounts" data-act="abOpenAcc" data-id="${acc.id}" data-fid="js:lim:${acc.id}">Limit ändern</a>` : '';
  if (q.source === 'admin') return `<div class="ab-quota"><span class="ab-small">Kein Limit: Jellyseerr gibt Administratoren keines.</span>${link}</div>`;
  const one = (k) => {
    const x = q[k];
    let s = quotaMeter(x, k);
    if (x.direct > 0) s += `<span class="ab-small">davon ${x.direct} direkt in Jellyseerr gestellt</span>`;
    if (x.restricted) s += `<span class="ab-small ab-warnt">Nächster Platz frei ${whenFree(x.nextFree)}</span>`;
    return `<span class="ab-qrow">${s}</span>`;
  };
  return `<div class="ab-quota">${one('movie')}${one('tv')}${q.source === 'seerr' ? '<span class="ab-small">Limit aus Jellyseerr: Dieser Benutzer gehört zu keinem Konto.</span>' : ''}${link}</div>`;
}
function abUserOptions(ownerAcc, current, first) {
  return [first, ...S.seerrUsers.map((u) => {
    const own = abUserOwner(u.id);
    const taken = own && own !== ownerAcc;
    const isCur = Number(current) === u.id;
    return { v: u.id, t: `${u.name} (ID ${u.id})` + (u.gone ? ' (in Jellyseerr gelöscht)' : '') + (taken ? ` · gehört zu ${account(own).email}` : ''), disabled: (taken || u.gone) && !isCur };
  })];
}
/* the account's wishes (owned or joined by one of its profiles) */
function abAccWishes(accId) { const ids = profilesOf(accId).map((p) => p.id); return S.wishes.filter((w) => ids.includes(w.owner) || w.joined.some((j) => ids.includes(j))); }
/* «Neue Konten in Jellyseerr eröffnen» is off: why the buttons are disabled */
function abProvOffLine(fid) { return `<p class="ab-line ab-small">${icon('lock', 16)}<span>Gesperrt: «Neue Konten in Jellyseerr eröffnen» ist unter Wünsche › Einstellungen ausgeschaltet.</span><a class="lnk" href="#/admin/wishes/settings" data-act="abOpenProvSet" data-fid="${fid}">Zur Einstellung</a></p>`; }
ACT.abOpenProvSet = () => { ui.open['ws:prov'] = true; };
/* one profile of an account: age level, wish right, tag, and the collapsed exception «sendet als» */
function abProfRow(acc, p) {
  const i = S.profiles.indexOf(p);
  const auto = tagLabel({ ...p, tagName: '' });
  const tags = S.settings.wishes.tags;
  const bad = p.tagName && !/^[a-z0-9-]+$/.test(p.tagName);
  const own = S.map.profiles[p.id], ou = own ? seerrUser(own) : null;
  const accU = S.map.accounts[acc.id];
  const mine = acc.id === curAccount().id;
  const right = `<span class="chip ${p.canRequest ? 'ok' : 'info'}">${icon(p.canRequest ? 'wish' : 'ban', 12)}${p.canRequest ? 'Darf anfordern' : 'Darf nicht anfordern'}</span>${mine ? `<a class="lnk ab-small" href="#/settings/profiles" data-fid="mp:pl:${p.id}">Im Profil ändern</a>` : '<span class="muted ab-small">setzt das Hauptprofil dieses Kontos</span>'}`;
  const tag = `<div class="fld ab-tagf"><label for="js:tag:${p.id}">Eigener Name <span class="muted ab-small">(Profil-Tag in Radarr und Sonarr)</span></label><div class="ab-chips"><input id="js:tag:${p.id}" class="inp ab-taginp" data-bind="S.profiles.${i}.tagName" data-live data-after="abTagSaved" data-fid="js:tag:${p.id}" value="${esc(p.tagName || '')}" placeholder="${esc(auto)}" maxlength="40" autocomplete="off" ${bad ? 'aria-invalid="true"' : ''}><span class="chip outline" title="Tag, den Slouchcat setzt">${icon('pin', 12)}${esc(tagLabel(p))}</span></div><span class="hint ab-small">Leer lassen für «${esc(auto)}». Nur a–z, 0–9 und Bindestrich.${tags.enabled ? '' : ' Profil-Tags sind zurzeit ausgeschaltet (Wünsche › Einstellungen).'}</span>${bad ? '<span class="err">Erlaubt sind nur Kleinbuchstaben a–z, Ziffern 0–9 und der Bindestrich.</span>' : ''}</div>`;
  const exMeta = ou ? `<span class="chip ${ou.gone ? 'bad' : 'warn'}">Sendet als ${esc(ou.name)}${ou.gone ? ' (gelöscht)' : ''}</span>` : '<span>Keine: sendet als Benutzer des Kontos</span>';
  const exBody = `<p class="ab-small ab-help">Nur für Sonderfälle: Das Profil fordert dann nicht als Benutzer des Kontos an, sondern als ein anderer Jellyseerr-Benutzer, etwa ein Kinder-Benutzer ohne automatische Freigabe. So braucht jeder Wunsch dieses Profils eine Freigabe. Es gelten dann Rechte, Limit und Freigabe jenes Benutzers, nicht die des Kontos. Der Tag bleibt der des Profils.</p>
    <div class="ab-pc"><span class="lbl">Sendet als</span>${selectEl('S.map.profiles.' + p.id, abUserOptions(acc.id, own, { v: '', t: `Benutzer des Kontos (${accU ? abUserName(accU) : 'noch keiner'})` }), { label: `Profil ${p.name} sendet als`, after: 'abMapProf' })}</div>
    ${ou ? `${abUserFacts(ou)}${abUserWarn(ou)}${!ou.gone ? abQuotaBlock(ou) : ''}` : ''}`;
  return `<li class="ab-prow2"><div class="ab-pn"><b>${esc(p.name)}</b><span class="muted ab-small">${p.primary ? 'Hauptprofil' : 'Weiteres Profil'} · ${ageLabel(effMax(p))}${p.pin ? ' · PIN' : ''}</span><span class="ab-chips">${right}</span></div>${tag}<div class="ab-exw">${band('js:ex:' + p.id, 'Ausnahme', exMeta, exBody, { open: !!(ou && ou.gone) })}</div></li>`;
}
/* one collapsed band per account */
function abSeerrBand(acc) {
  const st = accSeerr(acc), u = st.user, id = acc.id;
  const profs = profilesOf(id);
  const gone = !!(u && u.gone);
  const q = u && !gone ? quotaFor(u) : null;
  const profErr = profs.some((p) => { const x = S.map.profiles[p.id] && seerrUser(S.map.profiles[p.id]); return x && x.gone; });
  const changed = (ui.abChanged || {})[id];
  const ss = seerrState(), pwSync = S.settings.wishes.pwSync;
  const nw = abAccWishes(id).length;
  const meta = `<span class="chip info">${acc.role === 'admin' ? 'Administrator' : 'Konto'}</span><span class="chip ${gone ? 'bad' : st.info.chip}">${gone ? 'Benutzer gelöscht' : st.info.label}</span>${u ? `<span>${esc(u.name)} · ID ${u.id}</span>` : ''}<span>${abCount(nw, 'Wunsch', 'Wünsche')}</span>${q ? `<span>${abUse(q)}</span>` : ''}${changed ? `<span class="chip ok">${icon('check', 12)}Geändert</span>` : ''}`;
  const origin = u ? (u.origin === 'slouchcat' ? 'von Slouchcat eröffnet' : 'bestand schon in Jellyseerr') : '';
  const kv = [
    ['Jellyseerr-Benutzer', u ? `<b>${esc(u.name)}</b> <span class="mono ab-small">ID ${u.id}</span> · ${esc(u.email)} · ${esc(u.type)} · ${origin}` : st.suggest ? `<b>${esc(st.suggest.name)}</b> <span class="mono ab-small">ID ${st.suggest.id}</span> · ${esc(st.suggest.email)} · vorgeschlagen` : 'Keiner'],
    ['Wünsche', `${abAccWish(acc)}<span class="ab-small">${abCount(nw, 'Wunsch', 'Wünsche')} der Profile dieses Kontos</span>`],
    ['Zustand', `<span class="chip ${gone ? 'bad' : st.info.chip}">${gone ? 'In Jellyseerr gelöscht' : st.info.label}</span>${st.at ? ` <span class="ab-small">seit ${fmtDate(st.at)}</span>` : ''}`],
    ['Passwort', u ? `<span class="chip ${st.pwInfo.chip}">${st.pwInfo.label}</span>${st.pw === 'synced' && st.pwAt ? ` <span class="ab-small">abgeglichen am ${fmtDate(st.pwAt)}, ${fmtTime(st.pwAt)}</span>` : ''}` : 'Folgt mit dem Benutzer'],
    ['Rechte in Jellyseerr', u ? abRights(u) : `Folgen dem Wunschrecht des Kontos: ${acc.canRequest ? 'darf anfordern' : 'darf nicht anfordern'}`],
    ['Limit', u && !gone ? abQuotaBlock(u, acc) : `Wird beim Eröffnen gesetzt: ${abLimitShort(limitOf(acc))} <a class="lnk ab-small" href="#/admin/accounts" data-act="abOpenAcc" data-id="${id}" data-fid="js:lim:${id}">Limit ändern</a>`],
  ];
  const sug = !u && st.state !== 'suggested' ? suggestionFor(acc) : null;
  const propH = st.state === 'suggested' && st.suggest ? notice('warn', `Verknüpfung vorgeschlagen: «${esc(st.suggest.name)}»`, `In Jellyseerr gibt es schon einen Benutzer mit dieser E-Mail-Adresse. Das Konto hat die Adresse beim Annehmen der Einladung selbst eingegeben; Slouchcat prüft nicht, ob sie ihm gehört. Bestätige nur, wenn Konto und Benutzer dieselbe Person sind. Vorher verknüpft Slouchcat nichts und gibt kein Passwort an Jellyseerr.`, `<button type="button" class="btn sm primary" data-act="abConfirmLink" data-id="${id}" data-fid="js:conf:${id}">${icon('check', 16)}Verknüpfung bestätigen</button>`) : '';
  const sugH = sug ? notice('', `In Jellyseerr gibt es schon «${esc(sug.name)}»`, 'Gleiche E-Mail-Adresse: «In Jellyseerr eröffnen» verknüpft diesen Benutzer, statt einen zweiten anzulegen.') : '';
  const waitH = st.state === 'waiting' ? notice('warn', 'Wartet auf Jellyseerr', `Jellyseerr war beim Eröffnen nicht erreichbar (${fmtDate(st.at)}, ${fmtTime(st.at)}). Der nächste Abgleich versucht es wieder.`) : '';
  const noRight = !acc.canRequest ? notice('', 'Dieses Konto darf laut Slouchcat keine Titel anfordern', 'Der Benutzer in Jellyseerr hat dann auch keine Rechte zum Anfordern. Das ändert sich, sobald du dem Konto das Recht gibst.', `<a class="btn sm secondary" href="#/admin/accounts" data-act="abOpenAcc" data-id="${id}" data-fid="mp:acc:${id}">${icon('users', 16)}Zu den Konten</a>`) : '';
  const pickOpen = isOpen('js:pick:' + id);
  const acts = [];
  const provOff = !S.settings.wishes.provision;
  if (!u && st.state !== 'suggested') acts.push(`<button type="button" class="btn sm primary" data-act="abProvision" data-id="${id}" data-fid="js:prov:${id}" ${provOff ? 'disabled' : ''}>${icon(provOff ? 'lock' : 'plus', 16)}In Jellyseerr eröffnen</button>`);
  acts.push(`<button type="button" class="btn sm secondary" data-act="open" data-id="js:pick:${id}" aria-expanded="${pickOpen}" data-fid="js:pick:${id}">${icon('user', 16)}Anderen Benutzer wählen</button>`);
  if (u) acts.push(`<button type="button" class="btn sm dq" data-act="abUnlinkOk" data-id="${id}" data-fid="js:unl:${id}">${icon('ban', 16)}Verknüpfung lösen</button>`);
  let pwNote = '';
  if (u && !['admin', 'external'].includes(st.pw)) {
    if (st.pw === 'off') acts.push(`<button type="button" class="btn sm quiet" data-act="abPw" data-id="${id}" data-on="1" data-fid="js:pw:${id}" ${pwSync ? '' : 'disabled'}>${icon('lock', 16)}Passwort bei der nächsten Anmeldung neu übernehmen</button>`);
    else acts.push(`<button type="button" class="btn sm quiet" data-act="abPw" data-id="${id}" data-on="0" data-fid="js:pw:${id}">${icon('lock', 16)}Passwort nicht abgleichen</button>`);
    if (!pwSync && st.pw === 'off') pwNote = `Der Passwortabgleich ist ausgeschaltet. <a class="lnk" href="#/admin/wishes/settings" data-fid="js:pwset:${id}">Zu den Einstellungen</a>`;
  } else if (u && st.pw === 'admin') pwNote = `Das Passwort gleicht Slouchcat nicht ab: «${esc(u.name)}» ist in Jellyseerr Administrator, sein Passwort bleibt dort, wie es ist.`;
  else if (u && st.pw === 'external') pwNote = `Das Passwort gleicht Slouchcat nicht ab: «${esc(u.name)}» meldet sich über Jellyfin oder Plex an, nicht mit einem eigenen Passwort.`;
  /* only users this account may take; the others are listed locked with «gehört zu …» (form, not paleness) */
  const pickOpts = [{ v: '', t: 'Benutzer wählen' }, ...S.seerrUsers.filter((x) => !x.gone).map((x) => {
    const cur = u && x.id === u.id, own = abUserOwner(x.id), ok = seerrFreeFor(x, id);
    return { v: x.id, t: `${x.name} (ID ${x.id}) · ${x.email}${cur ? ' (jetzt)' : !ok && own ? ` · gehört zu ${account(own).email}` : ''}`, disabled: !ok && !cur };
  })];
  const pick = pickOpen ? `<div class="ab-pick">${selectEl('ui.draft.jsPick.' + id, pickOpts, { label: `Jellyseerr-Benutzer für ${acc.email}` })}<button type="button" class="btn sm primary" data-act="abPickOk" data-id="${id}" data-fid="js:pok:${id}">Übernehmen</button><button type="button" class="btn sm ghost" data-act="open" data-id="js:pick:${id}" data-fid="js:px:${id}">Abbrechen</button><span class="muted ab-small">Wählbar sind Benutzer, die zu keinem anderen Konto gehören und unter denen kein Profil eines anderen Kontos anfordert.${ss.k !== 'ok' ? ' Gezeigt wird der letzte Stand der Benutzerliste.' : ''}</span></div>` : '';
  const body = `${abUserWarn(u)}${waitH}${propH}${sugH}<dl class="kv ab-kv">${kv.map(([k, v]) => `<dt>${k}</dt><dd>${v}</dd>`).join('')}</dl>
    <div class="row-gap ab-acts">${acts.join('')}</div>${provOff && !u && st.state !== 'suggested' ? abProvOffLine('js:provset:' + id) : ''}${pwNote ? `<p class="ab-small ab-help">${pwNote}</p>` : ''}${pick}${noRight}
    <h3 class="ab-h3">Profile</h3><ul class="ab-plist">${profs.map((p) => abProfRow(acc, p)).join('')}</ul>`;
  return band('js:acc:' + id, esc(acc.email), meta, body, { open: gone || profErr || st.state === 'suggested', icon: 'user' });
}
route('/admin/wishes/mapping', (params, query = {}) => {
  /* «Benutzer zuordnen» from a request: open the band «ohne Konto» with that user preselected */
  abTakeQuery('#/admin/wishes/mapping', () => {
    const uid = Number(query.user);
    if (!uid || !seerrUser(uid)) return false;
    ui.open['js:free'] = true; ui.open['js:as:' + uid] = true; ui.abHlUser = uid; ui.abHlScroll = true;
    return true;
  });
  const ss = seerrState(), w = S.settings.wishes;
  const sts = S.accounts.map((a) => accSeerr(a));
  const withU = sts.filter((s) => s.user).length, without = sts.length - withU;
  const waiting = sts.filter((s) => s.state === 'waiting').length, pwOpen = sts.filter((s) => s.user && s.pw === 'next_login').length;
  const stats = `<div class="stats" style="--n:4" aria-label="Zusammenfassung"><div class="stat"><span class="v">${sts.length}</span><span class="l">Konten</span></div><div class="stat"><span class="v">${withU}</span><span class="l">In Jellyseerr</span><span class="n">eröffnet oder verknüpft</span></div><div class="stat ${without ? 'warn' : ''}"><span class="v">${without}</span><span class="l">Ohne Benutzer</span>${waiting ? `<span class="n">${waiting} ${waiting === 1 ? 'wartet' : 'warten'} auf Jellyseerr</span>` : ''}</div><div class="stat"><span class="v">${pwOpen}</span><span class="l">Passwort noch offen</span><span class="n">folgt bei der nächsten Anmeldung</span></div></div>`;
  const fallback = w.unmapped === 'fallback' && w.fallbackUser ? `fordert ein Konto als Sammelbenutzer «${esc(abUserName(w.fallbackUser))}» an` : 'kann ein Konto nichts anfordern';
  const provOff = !w.provision;
  const warn = without ? notice('warn', `${abCount(without, 'Konto', 'Konten')} ohne Jellyseerr-Benutzer`, `Ohne Benutzer ${fallback} (Regel «Konten ohne Jellyseerr-Benutzer» unter Einstellungen). «Alle eröffnen» verknüpft vorhandene Benutzer mit gleicher E-Mail-Adresse und legt für die übrigen einen neuen an. Vorgeschlagene Verknüpfungen bleiben offen, bis du sie bestätigst.${ss.k !== 'ok' ? ` <b>Jellyseerr ist zurzeit ${esc(ss.label.toLowerCase())}:</b> Die Konten warten dann und werden beim nächsten Abgleich eröffnet.` : ''}`, `<button type="button" class="btn sm primary" data-act="dialog" data-name="abProvisionAll" data-fid="js:all" ${provOff ? 'disabled' : ''}>${icon(provOff ? 'lock' : 'plus', 16)}Alle eröffnen</button><a class="btn sm secondary" href="#/admin/wishes/settings" data-fid="js:rule">${icon('settings', 16)}Zu den Einstellungen</a>`) + (provOff ? abProvOffLine('js:allset') : '') : '';
  const loaded = S.seerr.usersAt || S.seerr.lastSync;
  const bar = `<div class="ab-bar"><div class="ab-line">${icon('info', 16)}<span>Neue Konten: <b>${w.provision ? 'werden in Jellyseerr eröffnet' : 'werden nicht eröffnet'}</b> · Passwort: <b>${w.pwSync ? 'wird abgeglichen' : 'wird nicht abgeglichen'}</b></span><a class="lnk ab-small" href="#/admin/wishes/settings" data-fid="js:set">Ändern</a></div><div class="row-gap"><span class="muted ab-small">Benutzerliste geladen ${ago(loaded)}</span><button type="button" class="btn secondary" data-act="abUsersReload" data-fid="mp:reload">${icon('refresh', 16)}Benutzerliste neu laden</button></div></div>`;
  const free = S.seerrUsers.filter(seerrFree);
  const freeBody = free.length ? `<ul class="ab-free">${free.map((u) => {
    const ws = S.wishes.filter((x) => Number(x.seerrUser) === u.id);
    const un = ws.filter(abUnassigned).length;
    const sugFor = S.accounts.find((a) => suggestionFor(a) === u);
    const exc = S.profiles.filter((p) => Number(S.map.profiles[p.id]) === u.id);
    const asOpen = isOpen('js:as:' + u.id);
    /* accounts without a user (a proposal counts as without, N-2a) */
    const accs = S.accounts.filter((a) => !S.map.accounts[a.id] && seerrFreeFor(u, a.id));
    if (asOpen && (ui.draft.jsAssign || {})[u.id] == null) { ui.draft.jsAssign = ui.draft.jsAssign || {}; ui.draft.jsAssign[u.id] = accs.length === 1 ? accs[0].id : ''; }
    const asPick = asOpen ? `<div class="ab-pick">${accs.length ? `${selectEl('ui.draft.jsAssign.' + u.id, [{ v: '', t: 'Konto wählen' }, ...accs.map((a) => ({ v: a.id, t: a.email }))], { label: `Konto für ${u.name}` })}<button type="button" class="btn sm primary" data-act="abAssignOk" data-id="${u.id}" data-fid="js:aok:${u.id}">Übernehmen</button>` : '<span class="ab-small">Jedes Konto hat schon einen Jellyseerr-Benutzer.</span>'}<button type="button" class="btn sm ghost" data-act="open" data-id="js:as:${u.id}" data-fid="js:ax:${u.id}">Abbrechen</button><span class="muted ab-small">Zur Wahl stehen Konten ohne Jellyseerr-Benutzer.</span></div>` : '';
    return `<li class="${ui.abHlUser === u.id ? 'ab-hl' : ''}"><div class="ab-who"><b>${esc(u.name)} <span class="mono ab-small">ID ${u.id}</span></b><span>${esc(u.type)} · ${esc(u.email)}</span></div><div class="ab-who"><span>${abCount(ws.length, 'Anfrage', 'Anfragen')} in Jellyseerr${un ? `, davon ${un} nicht zuordenbar` : ''}</span>${un ? `<a class="lnk" href="#/admin/wishes?f=unassigned" data-fid="mp:un:${u.id}">Stehen unter «Anfragen › Nicht zuordenbar»</a>` : ''}${exc.length ? `<span>Ausnahme für ${exc.map((p) => 'Profil ' + esc(p.name)).join(', ')}</span>` : ''}${sugFor ? `<span>Gleiche E-Mail wie ${esc(sugFor.email)}: «In Jellyseerr eröffnen» verknüpft ihn</span>` : ''}</div>${abUserFacts(u)}<div class="ab-fullw"><button type="button" class="btn sm secondary" data-act="open" data-id="js:as:${u.id}" aria-expanded="${asOpen}" data-fid="js:as:${u.id}">${icon('users', 16)}Einem Konto zuordnen</button>${asPick}</div></li>`;
  }).join('')}</ul>` : '<p class="muted">Jeder Jellyseerr-Benutzer gehört zu einem Konto.</p>';
  const freeBand = band('js:free', 'Jellyseerr-Benutzer ohne Konto', `<span>${abCount(free.length, 'Benutzer', 'Benutzer')}</span>`, `${freeBody}<p class="muted ab-small">Anfragen dieser Benutzer übernimmt Slouchcat erst, wenn der Benutzer zu einem Konto gehört.</p>`, { icon: 'user' });
  const sandra = seerrUser(2);
  const help = `<div class="ab-mockhelp"><span class="ab-small"><b>Mockup-Hilfe</b> (gehört nicht zur App)</span><button type="button" class="btn sm secondary" data-act="abMockGone" data-fid="mp:mock">${sandra.gone ? 'Benutzer «sandra» in Jellyseerr wiederherstellen (Mockup)' : 'Benutzer «sandra» in Jellyseerr löschen (Mockup)'}</button></div>`;
  const html = `${abWishTabs('mapping')}${abWishNotices()}${stats}${warn}${bar}<div class="stack">${S.accounts.map(abSeerrBand).join('')}</div>${freeBand}${help}`;
  const hl = ui.abHlUser;
  return { type: 'admin', area: 'media', page: 'wishes', title: 'Wünsche', h1: 'Wünsche', crumb: 'Wünsche › Jellyseerr-Benutzer', intro: 'Jedes Konto hat in Jellyseerr einen eigenen Benutzer mit demselben Login. Seine Profile fordern als dieser Benutzer an und tragen einen Tag, damit man in Jellyseerr, Radarr und Sonarr sieht, wer was gewünscht hat. Rechte, Limit und Freigabe kommen von diesem Benutzer.', html,
    after: (root) => { if (!ui.abHlScroll || !hl) return; ui.abHlScroll = false; const el = root.querySelector(`[data-fid="js:as:${hl}"]`); if (el) { el.scrollIntoView({ block: 'center' }); el.focus({ preventScroll: true }); } } };
});
ACT.abAssignOk = (d) => {
  const u = seerrUser(Number(d.id)), accId = (ui.draft.jsAssign || {})[d.id];
  const acc = accId && account(accId);
  if (!acc) { toast('Wähle zuerst ein Konto.', 'warn'); return; }
  if (!seerrFreeFor(u, acc.id)) { toast(`Geht nicht: «${esc(u.name)}» gehört schon zu einem anderen Konto.`, 'bad'); return; }
  linkAccount(acc.id, u.id);
  ui.open['js:as:' + u.id] = false; delete ui.draft.jsAssign[d.id];
  if (ui.abHlUser === u.id) ui.abHlUser = null;
  abMark(acc.id); commit();
  const n = S.wishes.filter((w) => Number(w.seerrUser) === u.id && abUnassigned(w)).length;
  toast(`${esc(acc.email)} → «${esc(u.name)}». ${n ? `${abCount(n, 'Anfrage', 'Anfragen')} übernimmt der nächste Abgleich.` : 'Rechte und Freigabe kommen jetzt von diesem Benutzer; Slouchcat setzt dort das Limit des Kontos.'}`, 'ok', 6500);
};
function abMark(accId) { ui.abChanged = ui.abChanged || {}; ui.abChanged[accId] = Date.now(); }
/* «noah: Tag heisst jetzt sc-noah-k.» (07, section 6) */
ACT.abTagSaved = (d) => {
  const p = S.profiles[Number(d.bind.split('.')[2])];
  if (!p) return;
  if (p.tagName && !/^[a-z0-9-]+$/.test(p.tagName)) { toast('Tag-Name ungültig: nur a–z, 0–9 und Bindestrich.', 'warn'); return; }
  abMark(p.accountId);
  toast(`${esc(p.name)}: Tag heisst jetzt ${esc(tagLabel(p))}.`, 'ok');
};
ACT.abMapProf = (d, el) => {
  const pid = d.bind.split('.').pop(), p = profile(pid);
  if (el.value === '') delete S.map.profiles[pid]; else S.map.profiles[pid] = Number(el.value);
  save(); abMark(p.accountId);
  toast(el.value === '' ? `Ausnahme entfernt: Profil ${esc(p.name)} sendet wieder als Benutzer des Kontos.` : `Ausnahme gesetzt: Profil ${esc(p.name)} sendet als ${esc(abUserName(el.value))}. Es gelten Rechte, Limit und Freigabe dieses Benutzers.`, 'ok');
};
function abPwText(pw) { return pw === 'synced' ? 'Passwort übernommen.' : pw === 'next_login' ? 'Das Passwort folgt bei der nächsten Anmeldung in Slouchcat.' : pw === 'admin' ? 'Das Passwort bleibt in Jellyseerr (Administrator).' : pw === 'external' ? 'Anmeldung dort über Jellyfin oder Plex.' : 'Das Passwort gleicht Slouchcat nicht ab.'; }
function abProvOffToast() { if (S.settings.wishes.provision) return false; toast('«Neue Konten in Jellyseerr eröffnen» ist unter Wünsche › Einstellungen ausgeschaltet. Nichts eröffnet.', 'warn'); return true; }
ACT.abProvision = (d) => {
  if (abProvOffToast()) return;
  const acc = account(d.id);
  const st = provisionAccount(d.id);
  save(); abMark(d.id);
  if (st.state === 'waiting') toast(`Jellyseerr ist ${esc(seerrState().label.toLowerCase())}: ${esc(acc.email)} wird beim nächsten Abgleich eröffnet.`, 'warn');
  else if (st.state === 'linked') toast(`In Jellyseerr gab es «${esc(st.user.name)}» mit gleicher E-Mail schon: verknüpft. ${abPwText(st.pw)}`, 'ok');
  else if (st.user) toast(`${esc(acc.email)} ist in Jellyseerr eröffnet als «${esc(st.user.name)}». ${abPwText(st.pw)}`, 'ok');
  render();
};
ACT.abConfirmLink = (d) => {
  const st = confirmLink(d.id);
  abMark(d.id);
  if (st && st.user) toast(`${esc(account(d.id).email)} ist mit «${esc(st.user.name)}» verknüpft. ${abPwText(st.pw)}`, 'ok');
  render();
};
/* what «Alle eröffnen» would do: link by equal e-mail, create, leave proposals alone */
function abProvisionPlan() {
  const todo = S.accounts.filter((a) => !S.map.accounts[a.id] && accSeerr(a).state !== 'suggested');
  const link = todo.filter((a) => S.seerrUsers.some((u) => seerrFree(u) && u.email.toLowerCase() === a.email.toLowerCase()));
  return { todo, link, create: todo.filter((a) => !link.includes(a)), sug: S.accounts.filter((a) => accSeerr(a).state === 'suggested') };
}
DIALOGS.abProvisionAll = () => {
  const pl = abProvisionPlan(), ss = seerrState();
  const li = (arr, text) => (arr.length ? `<li>${text}: ${arr.map((a) => esc(a.email)).join(', ')}</li>` : '');
  const sum = [pl.link.length ? `${pl.link.length} verknüpfen` : '', pl.create.length ? `${pl.create.length} neu anlegen` : '', pl.sug.length ? `${abCount(pl.sug.length, 'Vorschlag bleibt', 'Vorschläge bleiben')} offen` : ''].filter(Boolean).join(', ');
  return {
    title: 'Alle Konten in Jellyseerr eröffnen?',
    body: pl.todo.length ? `<p class="lead">${sum}.</p><ul class="ab-idl">${li(pl.link, 'Über gleiche E-Mail-Adresse verknüpfen')}${li(pl.create, 'In Jellyseerr neu anlegen')}${li(pl.sug, 'Vorgeschlagene Verknüpfung bleibt offen, bis du sie bestätigst')}</ul>${ss.k !== 'ok' ? `<p class="sec">Jellyseerr ist zurzeit ${esc(ss.label.toLowerCase())}: Die Konten warten dann und werden beim nächsten Abgleich eröffnet.</p>` : ''}`
      : `<p class="lead">Jedes Konto hat schon einen Benutzer${pl.sug.length ? ` oder einen Vorschlag, der auf dich wartet (${pl.sug.map((a) => esc(a.email)).join(', ')})` : ''}.</p>`,
    actions: `<button type="button" class="btn ghost" data-act="close" data-fid="pa:x" ${pl.todo.length ? '' : 'data-autofocus'}>${pl.todo.length ? 'Abbrechen' : 'Schliessen'}</button>${pl.todo.length ? `<button type="button" class="btn primary" data-act="abProvisionAll" data-fid="pa:ok" data-autofocus>${icon('plus', 16)}Alle eröffnen</button>` : ''}`,
  };
};
ACT.abProvisionAll = () => {
  closeDialog();
  if (abProvOffToast()) return;
  const ids = S.accounts.filter((a) => !S.map.accounts[a.id] && accSeerr(a).state !== 'suggested').map((a) => a.id);
  provisionAll();
  ids.forEach(abMark);
  const c = { linked: 0, created: 0, waiting: 0 };
  ids.forEach((id) => { const s = accSeerr(account(id)).state; if (c[s] != null) c[s]++; });
  const parts = [c.linked ? `${c.linked} über gleiche E-Mail verknüpft` : '', c.created ? `${c.created} neu angelegt` : '', c.waiting ? `${c.waiting} ${c.waiting === 1 ? 'wartet' : 'warten'} auf Jellyseerr` : ''].filter(Boolean);
  toast(ids.length ? `${abCount(ids.length, 'Konto', 'Konten')}: ${parts.join(', ')}.` : 'Jedes Konto hat schon einen Benutzer.', c.waiting ? 'warn' : 'ok', 6500);
  render();
};
ACT.abPickOk = (d) => {
  const acc = account(d.id), v = Number(((ui.draft.jsPick || {})[d.id]) || 0);
  if (!v) { toast('Wähle zuerst einen Benutzer.', 'warn'); return; }
  if (v === Number(S.map.accounts[d.id])) { toast('Das ist schon der Benutzer dieses Kontos.', 'warn'); return; }
  if (!seerrFreeFor(seerrUser(v), d.id)) { delete ui.draft.jsPick[d.id]; render(); toast(`Geht nicht: «${esc(abUserName(v))}» gehört zu einem anderen Konto.`, 'bad'); return; }
  linkAccount(d.id, v);
  ui.open['js:pick:' + d.id] = false; delete ui.draft.jsPick[d.id];
  abMark(d.id); commit();
  toast(`${esc(acc.email)} → «${esc(abUserName(v))}». Rechte und Freigabe kommen jetzt von diesem Benutzer; Slouchcat setzt dort das Limit des Kontos.`, 'ok', 6500);
};
/* «Verknüpfung lösen» acts at once, without a dialog (07, section 6) */
ACT.abUnlinkOk = (d) => {
  const acc = account(d.id), u = accSeerr(acc).user;
  unlinkAccount(d.id);
  abMark(d.id); commit();
  toast(`Verknüpfung von ${esc(acc.email)} gelöst. «${esc(u ? u.name : '')}» bleibt in Jellyseerr. Bis du das Konto neu eröffnest oder einen Benutzer wählst, kann es nichts anfordern.`, 'ok', 6500);
};
ACT.abPw = (d) => {
  pwSetMode(d.id, d.on === '1');
  abMark(d.id); commit();
  toast(d.on === '1' ? 'Slouchcat übernimmt das Passwort bei der nächsten Anmeldung des Kontos.' : 'Das Passwort dieses Kontos gleicht Slouchcat nicht mehr ab. In Jellyseerr bleibt das bisherige.', 'ok');
};
ACT.abOpenSeerr = (d) => { ui.open['js:acc:' + d.id] = true; };
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
const AB_LABEL = { enabled: 'Wünsche aktiv', movies: 'Filme erlauben', shows: 'Serien erlauben', fromSearch: 'Anfordern aus der Suche', fromDetail: 'Anfordern auf Detailseiten', join: 'Bereits angeforderte Titel', unmapped: 'Konten ohne Jellyseerr-Benutzer', fallbackUser: 'Sammelbenutzer', newAccounts: 'Neue Konten dürfen anfordern', withdraw: 'Zurückziehen erlauben', pinPrivacy: 'Externe Nachrichten bei PIN-Profilen ohne Titel', fromDiscover: 'Seiten Entdecken, Filme, Serien und Suche', provision: 'Neue Konten in Jellyseerr eröffnen', pwSync: 'Passwort abgleichen' };
const AB_LABEL_PATH = { 'S.settings.wishes.tags.enabled': 'Profil-Tag in Radarr und Sonarr setzen', 'S.settings.wishes.progress': 'Download-Fortschritt zeigen' };
const AB_VAL = { join: 'Mitwünschen', status: 'Nur Zustand zeigen', deny: 'Nicht erlauben', fallback: 'Sammelbenutzer' };
ACT.abSaved = (d) => {
  const path = d.path || d.bind;
  const m = /^S\.accounts\.(\d+)\.canRequest$/.exec(path);
  if (m) { const a = S.accounts[Number(m[1])]; toast(`${esc(a.email)} ${a.canRequest ? 'darf jetzt Titel anfordern' : 'darf keine Titel mehr anfordern'}.`, 'ok'); return; }
  const k = path.split('.').pop(), v = getPath(path), l = AB_LABEL_PATH[path] || AB_LABEL[k] || 'Einstellung';
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
/* limit editor: «Filme [n] in [Tage] Tagen», «Staffeln [n] in [Tage] Tagen»; base = path of { movie: {n, days}, tv: {n, days} } */
const abBadNum = (v, lo, hi) => v === '' || v == null || !Number.isInteger(Number(v)) || Number(v) < lo || Number(v) > hi;
function abLimitErr(base) {
  const bad = ['movie', 'tv'].some((k) => { const c = getPath(base + '.' + k) || {}; return abBadNum(c.n, 0, 100) || abBadNum(c.days, 1, 90); });
  return bad ? 'Anzahl 0 bis 100, Tage 1 bis 90.' : '';
}
function abLimitEditor(base, fp, o = {}) {
  const num = (path, label, lo, hi, fid) => { const v = getPath(path); return `<input class="inp num" type="number" min="${lo}" max="${hi}" aria-label="${esc(label)}" data-bind="${path}" data-num data-live ${o.after ? `data-after="${o.after}"` : ''} data-fid="${fid}" value="${esc(v ?? '')}" ${abBadNum(v, lo, hi) ? 'aria-invalid="true"' : ''} ${o.disabled ? 'disabled' : ''}>`; };
  const row = (k, unit) => `<div class="ab-limrow"><b>${unit}</b>${num(`${base}.${k}.n`, `${unit}: Anzahl`, 0, 100, `${fp}:${k}:n`)}<span>in</span>${num(`${base}.${k}.days`, `${unit}: Tage`, 1, 90, `${fp}:${k}:d`)}<span>Tagen</span></div>`;
  const err = abLimitErr(base);
  return `<div class="ab-lim">${row('movie', 'Filme')}${row('tv', 'Staffeln')}${err ? `<span class="err">${err}</span>` : ''}<span class="muted ab-small">0 heisst ohne Limit.</span></div>`;
}
ACT.abLimDefSaved = () => {
  if (abLimitErr('S.settings.wishes.limit')) { toast('Prüfe das Limit: Anzahl 0 bis 100, Tage 1 bis 90.', 'warn'); return; }
  const n = S.accounts.filter((a) => (a.limit || { mode: 'default' }).mode === 'default' && S.map.accounts[a.id]).length;
  toast(`Limit ab Werk: ${abLimitShort(S.settings.wishes.limit)}. Slouchcat schreibt es in ${abCount(n, 'Jellyseerr-Benutzer', 'Jellyseerr-Benutzer')} der Konten mit «Standard».`, 'ok', 6500);
};
function abLimitDefBand(off) {
  const w = S.settings.wishes;
  const own = S.accounts.filter((a) => (a.limit || {}).mode === 'custom').length, unl = S.accounts.filter((a) => (a.limit || {}).mode === 'unlimited').length;
  const body = `${abSrow('Limit je Konto', 'Gilt für alle Profile eines Kontos zusammen. Jellyseerr zählt rollierend und rechnet auch Wünsche mit, die direkt dort gestellt werden. Abgelehnte Wünsche und Mitwünsche zählen nicht. Administratoren haben kein Limit.', abLimitEditor('S.settings.wishes.limit', 'wl', { after: 'abLimDefSaved', disabled: off }), { dim: off })}
    ${abSrow('Abweichend je Konto', 'Unter «Konten» lässt sich das Limit je Konto abweichend setzen: eigenes Limit oder unbegrenzt.', `<span class="chip info">${own} mit eigenem Limit · ${unl} unbegrenzt</span><a class="lnk" href="#/admin/accounts" data-fid="wl:acc">Zu den Konten</a>`, { isNew: false })}`;
  return band('ws:limit', 'Limit ab Werk <span class="newtag">NEU</span>', `<span>${abLimitShort(w.limit)}</span>`, body, { icon: 'chart', open: !!abLimitErr('S.settings.wishes.limit') });
}
function abProvBand(off) {
  const w = S.settings.wishes, P = 'S.settings.wishes.';
  const local = S.seerr.localLogin !== false;
  const body = `${abSrow('Neue Konten in Jellyseerr eröffnen', 'Wer eine Einladung annimmt, bekommt in Jellyseerr einen Benutzer mit derselben E-Mail-Adresse. Gibt es dort schon einen Benutzer mit dieser Adresse, schlägt Slouchcat die Verknüpfung nur vor: Sie gilt erst, wenn du sie unter «Jellyseerr-Benutzer» bestätigst, denn die Adresse hat das neue Konto selbst eingegeben.', sw(P + 'provision', 'Neue Konten in Jellyseerr eröffnen', { disabled: off, after: 'abSaved' }), { dim: off })}
    ${abSrow('Passwort abgleichen', 'Slouchcat gibt das Passwort an Jellyseerr weiter, wenn es gesetzt oder geändert wird und bei der Anmeldung in Slouchcat. Slouchcat speichert es nie im Klartext. Dafür braucht Jellyseerr die «Lokale Anmeldung». Ein Passwort, das jemand direkt in Jellyseerr ändert, bemerkt Slouchcat nicht. Wer sich in Jellyseerr anmeldet, sieht dort die Wünsche aller Profile des Kontos, auch die eines Profils mit PIN.', sw(P + 'pwSync', 'Passwort abgleichen', { disabled: off, after: 'abSaved' }) + `<span class="chip ${local ? 'ok' : 'warn'}">Lokale Anmeldung in Jellyseerr: ${local ? 'an' : 'aus'}</span>`, { dim: off })}
    ${w.pwSync && !local ? notice('warn', 'In Jellyseerr ist die «Lokale Anmeldung» aus', 'Das übernommene Passwort nützt dort nichts, bis du sie in Jellyseerr einschaltest.') : ''}
    ${abSrow('Bestehende Konten', 'Je Konto eröffnen, verknüpfen oder lösen.', `<a class="lnk" href="#/admin/wishes/mapping" data-fid="wp:map">Zu den Jellyseerr-Benutzern</a>`, { isNew: false })}`;
  return band('ws:prov', 'Konten in Jellyseerr <span class="newtag">NEU</span>', `<span>${w.provision ? 'Neue Konten eröffnen' : 'Neue Konten nicht eröffnen'} · ${w.pwSync ? 'Passwort abgleichen' : 'kein Passwortabgleich'}</span>`, body, { icon: 'users' });
}
/* «Präfix»: 1 to 10 characters, a–z, 0–9 and hyphen; typed into a draft, saved only when valid */
const AB_PREFIX_RE = /^[a-z0-9-]{1,10}$/;
function abPrefixDraft() { if (ui.draft.tagPrefix == null) ui.draft.tagPrefix = S.settings.wishes.tags.prefix || ''; return ui.draft.tagPrefix; }
ACT.abPrefixSaved = () => {
  const v = abPrefixDraft();
  if (!AB_PREFIX_RE.test(v)) { toast('Präfix nicht gespeichert: 1 bis 10 Zeichen, nur Kleinbuchstaben, Ziffern und Bindestrich.', 'warn'); return; }
  if (v === S.settings.wishes.tags.prefix) return;
  S.settings.wishes.tags.prefix = v; save(); delete ui.draft.tagPrefix;
  toast(`Präfix «${esc(v)}»: neue Tags heissen etwa «${esc(tagLabel(S.profiles[0]))}». Gesetzte Tags bleiben, wie sie sind.`, 'ok');
};
function abTagBand(off) {
  const t = S.settings.wishes.tags, P = 'S.settings.wishes.tags.';
  const pv = abPrefixDraft(), bad = !AB_PREFIX_RE.test(pv);
  const ex = ['p1', 'p3', 'p5'].map(profile).filter(Boolean).concat(S.profiles).filter((p, i, a) => a.indexOf(p) === i).slice(0, 3);
  const arr = (a) => `<span class="chip ${a.condition === 'ok' ? 'ok' : 'bad'}">${esc(a.name)}: ${a.condition === 'ok' ? 'erreichbar' : 'nicht erreichbar'}</span>`;
  const body = `${abSrow('Profil-Tag in Radarr und Sonarr setzen', 'Slouchcat legt in Radarr und Sonarr je Profil einen Tag an und hängt ihn an den Titel, sobald Jellyseerr ihn dorthin übergeben hat. Wer mitwünscht, bekommt seinen Tag dazu. Sonst ändert Slouchcat in Radarr und Sonarr nichts.', sw(P + 'enabled', 'Profil-Tag in Radarr und Sonarr setzen', { disabled: off, after: 'abSaved' }), { dim: off })}
    ${abSrow('Download-Fortschritt zeigen', 'Bei Wünschen in Beschaffung zeigt Slouchcat, was Radarr oder Sonarr melden: gesucht, Prozent und Restzeit, eingelesen, verzögert. Nur lesend: Slouchcat ändert dafür nichts in Radarr und Sonarr. Nur der Wunsch zeigt den Fortschritt, nie die Bibliothek.', sw('S.settings.wishes.progress', 'Download-Fortschritt zeigen', { disabled: off, after: 'abSaved' }), { dim: off })}
    ${abSrow('Präfix', 'Kleinbuchstaben, Ziffern und Bindestrich. Beispiel: sc-noah. Gilt für neue Tags; bestehende Tags bleiben, wie sie sind. Den Tag eines einzelnen Profils änderst du unter «Jellyseerr-Benutzer».', `<input class="inp ab-taginp" aria-label="Präfix" data-bind="ui.draft.tagPrefix" data-live data-after="abPrefixSaved" data-fid="wt:prefix" value="${esc(pv)}" maxlength="10" pattern="[a-z0-9-]{1,10}" ${bad ? 'aria-invalid="true"' : ''} ${off || !t.enabled ? 'disabled' : ''}>${bad ? '<span class="err">1 bis 10 Zeichen: nur Kleinbuchstaben a–z, Ziffern 0–9 und Bindestrich. Nicht gespeichert.</span>' : ''}`, { dim: off || !t.enabled })}
    ${abSrow('Beispiele', '', ex.map((p) => `<span class="chip outline">${esc(p.name)} → ${icon('pin', 12)}${esc(tagLabel(p))}</span>`).join(''), { isNew: false, dim: !t.enabled })}
    ${abSrow('Radarr und Sonarr', 'Adresse und Schlüssel kommen aus Jellyseerr.', `${arr(S.arr.radarr)}${arr(S.arr.sonarr)}<a class="lnk" href="#/admin/integrations/jellyseerr" data-fid="wt:conn">Zur Verbindung</a>`, { isNew: false })}`;
  return band('ws:tags', 'Radarr und Sonarr: Profil-Tag und Fortschritt <span class="newtag">NEU</span>', `<span>Tag ${t.enabled ? `an · z. B. ${esc(tagLabel(ex[0]))}` : 'aus'} · Fortschritt ${progressOn() ? 'an' : 'aus'}</span>${bad ? '<span class="chip bad">Präfix ungültig</span>' : ''}`, body, { icon: 'pin', open: bad });
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
    ${s('fromSearch', 'Anfordern aus der Suche', 'Abschnitt «Noch nicht in der Bibliothek». Aus: Der Abschnitt fehlt.')}
    ${s('fromDetail', 'Anfordern auf Detailseiten', 'Fehlende Staffeln auf der Seite einer vorhandenen Serie. Aus: Die Serienseite zeigt nur, was vorhanden ist.')}
    ${s('fromDiscover', 'Seiten Entdecken, Filme, Serien und Suche', 'Eigene Seiten unter «Wünsche» zum Stöbern in allem, was TMDB kennt, nur für Profile, die anfordern dürfen. Aus: Unter «Wünsche» bleibt nur «Meine Wünsche».')}</section>`;
  const fbOpts = [{ v: '', t: 'Benutzer wählen' }, ...S.seerrUsers.filter((u) => !u.gone).map((u) => ({ v: u.id, t: u.name }))];
  const rules = `<section class="card"><div class="ch"><span class="ik">${icon('settings', 16)}</span><h2>Regeln</h2></div>
    ${abSrow('Bereits angeforderte Titel', w.join === 'join' ? 'Mitwünschen: Ein zweites Profil hängt sich an den offenen Wunsch («Benachrichtige mich») und wird ebenfalls benachrichtigt. Jellyseerr bekommt keine zweite Anfrage.' : 'Nur Zustand zeigen: Ein zweites Profil sieht «Bereits angefordert», kann sich aber nicht dazuschalten und wird nicht benachrichtigt.', abSeg(P + 'join', [['join', 'Mitwünschen'], ['status', 'Nur Zustand zeigen']], { label: 'Bereits angeforderte Titel', after: 'abSaved', disabled: off }), { dim: off, col: false })}
    ${abSrow('Konten ohne Jellyseerr-Benutzer', 'Sammelbenutzer: Alle Konten ohne eigenen Benutzer teilen Limit und Auto-Freigabe dieses Benutzers.', abSeg(P + 'unmapped', [['deny', 'Nicht erlauben'], ['fallback', 'Sammelbenutzer']], { label: 'Konten ohne Jellyseerr-Benutzer', after: 'abSaved', disabled: off }) + (w.unmapped === 'fallback' ? selectEl(P + 'fallbackUser', fbOpts, { label: 'Sammelbenutzer', after: 'abSaved', disabled: off }) + (!w.fallbackUser ? '<span class="chip warn">Ohne Benutzer gilt «Nicht erlauben»</span>' : '') : ''), { dim: off })}
    ${s('newAccounts', 'Neue Konten dürfen anfordern', 'Vorgabe des Schalters im Dialog «Konto einladen». Ändert bestehende Konten nicht.')}
    ${s('withdraw', 'Zurückziehen erlauben', 'Solange ein Wunsch nicht freigegeben ist. Knopf «Zurückziehen» unter «Meine Wünsche».')}
    ${s('pinPrivacy', 'Externe Nachrichten bei PIN-Profilen ohne Titel', 'E-Mail, Web-Push und Telegram nennen dann nur das Profil: «[Sandra] Ein Wunsch ist jetzt verfügbar.»')}</section>`;
  const refs = `<section class="card"><div class="ch"><span class="ik">${icon('link', 16)}</span><h2>Verweise</h2></div>
    ${abSrow('Wer anfordern darf', 'Wer anfordern darf, stellst du je Konto unter Konten ein; je Profil entscheidet das Hauptprofil.', `<a class="lnk" href="#/admin/accounts" data-fid="ws:acc">Zu den Konten</a>`, { isNew: false })}
    ${abSrow('Verbindung zu Jellyseerr', 'Adresse, Schlüssel, Test und Protokoll.', `<span class="chip ${ss.chip}">${esc(ss.label)}</span><a class="lnk" href="#/admin/integrations/jellyseerr" data-fid="ws:conn">Zur Verbindung</a>`, { isNew: false })}
    ${abSrow('Jellyseerr-Benutzer', 'Welches Konto als welcher Jellyseerr-Benutzer anfordert, mit Passwort, Limit und Profil-Tags.', `<span class="chip ${mappingCounts().missing ? 'warn' : 'ok'}">${mappingCounts().mapped} von ${mappingCounts().total} Konten</span><a class="lnk" href="#/admin/wishes/mapping" data-fid="ws:map">Zu den Jellyseerr-Benutzern</a>`, { isNew: false })}</section>`;
  const html = `${abWishTabs('settings')}${abWishNotices(true)}${off ? notice('warn', 'Wünsche sind ausgeschaltet', 'Die übrigen Einstellungen sind gesperrt, bis du «Wünsche aktiv» einschaltest.') : ''}<p class="muted ab-small">Jede Änderung gilt sofort.</p>${base}${where}${rules}<div class="stack">${abLimitDefBand(off)}${abProvBand(off)}${abTagBand(off)}</div>${refs}`;
  return { type: 'admin', area: 'media', page: 'wishes', title: 'Wünsche', crumb: 'Wünsche › Einstellungen', intro: 'Was angefordert werden darf, von wo aus und nach welchen Regeln. Die Regeln gelten für alle Konten.', html };
});

/* ================= M-26 Konten ================= */
function abAccWish(acc) {
  if (!acc.canRequest) return '<span class="chip info">Nicht erlaubt</span>';
  const id = S.map.accounts[acc.id];
  if (!id) {
    if (S.settings.wishes.unmapped === 'fallback' && S.settings.wishes.fallbackUser) return `<span class="chip ok">Erlaubt · ${esc(abUserName(S.settings.wishes.fallbackUser))} (Sammelbenutzer)</span>`;
    if (accSeerr(acc).state === 'waiting') return `<span class="chip warn">${icon('hourglass', 12)}Wird in Jellyseerr eröffnet</span>`;
    return `<span class="chip warn">${icon('alert', 12)}Kein Jellyseerr-Benutzer</span>`;
  }
  const u = seerrUser(id);
  if (!u || u.gone) return `<span class="chip bad">${icon('alert', 12)}Benutzer gelöscht</span>`;
  return `<span class="chip ok">Erlaubt · ${esc(u.name)}</span>`;
}
function abProfSign(p) {
  return `<span class="chip ${p.canRequest ? 'ok' : 'info'}" title="${p.canRequest ? 'darf anfordern' : 'darf nicht anfordern'}">${icon(p.canRequest ? 'wish' : 'ban', 12)}${esc(p.name)}<span class="vh">: ${p.canRequest ? 'darf anfordern' : 'darf nicht anfordern'}</span></span>`;
}
/* «Jellyseerr-Benutzer: grosseltern · eröffnet · Passwort abgeglichen · Ändern» (07, section 8) */
const AB_ACC_SHORT = { created: 'eröffnet', linked: 'verknüpft', manual: 'von Hand gewählt', waiting: 'wartet auf Jellyseerr', none: 'nicht eröffnet' };
const AB_PW_SHORT = { synced: 'Passwort abgeglichen', next_login: 'Passwort folgt bei der nächsten Anmeldung', off: 'Passwort nicht abgeglichen', admin: 'Passwort bleibt in Jellyseerr (Administrator)', external: 'Anmeldung über Jellyfin' };
function abSeerrLine(acc) {
  const st = accSeerr(acc), u = st.user;
  const link = (label, fid) => `<a class="lnk" href="#/admin/wishes/mapping" data-act="abOpenSeerr" data-id="${acc.id}" data-fid="${fid}">${label}</a>`;
  if (st.state === 'suggested' && st.suggest) return `<p class="ab-line">${icon('user', 16)}<span>Verknüpfung vorgeschlagen: <b>${esc(st.suggest.name)}</b></span>${link('Bestätigen', 'awb:c:' + acc.id)}</p>`;
  const parts = u ? [`<b>${esc(u.name)}</b>`, u.gone ? 'in Jellyseerr gelöscht' : AB_ACC_SHORT[st.state], AB_PW_SHORT[st.pw || 'off']] : ['<b>Keiner</b>', AB_ACC_SHORT[st.state] || 'nicht eröffnet'];
  return `<p class="ab-line">${icon('user', 16)}<span>Jellyseerr-Benutzer: ${parts.join(' · ')}</span>${link('Ändern', 'awb:' + acc.id)}</p>`;
}
function abWishBlock(acc, path) {
  return `<div class="ab-swl">${sw(path, 'Darf Titel anfordern')}<span class="ab-who"><b>Darf Titel anfordern</b><span>Deckel für alle Profile dieses Kontos. Welches Profil anfordern darf, stellt das Hauptprofil ein. Die Rechte des Jellyseerr-Benutzers folgen diesem Schalter.</span></span></div>
    ${abSeerrLine(acc)}
    <div class="ab-chips" aria-label="Profile und ihr Wunschrecht">${profilesOf(acc.id).map(abProfSign).join('')}</div>`;
}
/* «Limit» in the form «Zugriff»: Standard / Eigenes / Unbegrenzt, saved with the form (07, section 8) */
const AB_LIM_MODES = [['default', 'Standard'], ['custom', 'Eigenes'], ['unlimited', 'Unbegrenzt']];
function abLimitField(acc, base, fp) {
  const d = getPath(base), st = accSeerr(acc), u = st.user && !st.user.gone ? st.user : null;
  if (u && u.admin) return `<div class="fld"><label for="${fp}:lim">Limit</label><select id="${fp}:lim" class="sel" disabled data-fid="${fp}:lim"><option>Kein Limit (Administrator)</option></select><span class="hint">${icon('lock', 12)} «${esc(u.name)}» ist in Jellyseerr Administrator. Jellyseerr gibt Administratoren kein Limit.</span></div>`;
  const hint = d.limitMode === 'custom' ? '' : `<span class="hint">${d.limitMode === 'unlimited' ? 'Ohne Limit.' : `Standard: ${abLimitShort(S.settings.wishes.limit)} (Limit ab Werk unter Wünsche › Einstellungen).`} Gilt für alle Profile dieses Kontos zusammen.</span>`;
  let use;
  if (!u) use = '<p class="muted ab-small">Noch kein Jellyseerr-Benutzer: Slouchcat setzt das Limit, sobald das Konto dort eröffnet ist.</p>';
  else { const q = quotaFor(u); use = `<p class="ab-line">${icon('chart', 16)}<span>Verbrauch in Jellyseerr («${esc(u.name)}»), Stand jetzt</span></p><div class="ab-quota">${quotaMeter(q.movie, 'movie')}${quotaMeter(q.tv, 'tv')}</div>`; }
  return `<div class="fld"><span class="lbl">Limit</span>${abSeg(base + '.limitMode', AB_LIM_MODES, { label: 'Limit' })}${d.limitMode === 'custom' ? abLimitEditor(base + '.limit', fp + ':lim') : ''}${hint}</div>${use}
    <p class="muted ab-small">Jellyseerr zählt rollierend, auch Wünsche, die direkt dort gestellt werden; abgelehnte Wünsche und Mitwünsche zählen nicht.</p>`;
}
function abLibsField(base, fp) {
  const d = getPath(base);
  const none = !d.allLibs && !Object.values(d.libs).some(Boolean);
  return `<div class="fld"><span class="lbl">Bibliotheken</span><div class="seg" role="group" aria-label="Bibliotheken">${[[true, 'Alle'], [false, 'Auswahl']].map(([v, l]) => `<button type="button" data-act="set" data-path="${base}.allLibs" data-value="${v}" aria-pressed="${d.allLibs === v}" data-fid="${fp}:lm:${v}">${l}</button>`).join('')}</div>${d.allLibs ? '<span class="hint">Alle Bibliotheken</span>' : `<span class="hint">Bibliotheken wählen</span><div class="ab-libs">${LIBS.map(([l]) => `<label class="check"><input type="checkbox" data-bind="${base}.libs.${l}" data-fid="${fp}:lib:${l}" ${d.libs[l] ? 'checked' : ''}><span>${esc(l)}</span></label>`).join('')}</div>${none ? '<span class="err">Wähle mindestens eine Bibliothek.</span>' : ''}`}</div>`;
}
function abAccVals(acc) {
  const libs = acc.libs || ['Filme', 'Serien'];
  const l = acc.limit || { mode: 'default' }, w = S.settings.wishes.limit, own = l.mode === 'custom' && l.movie && l.tv;
  return { rating: Number(acc.maxRating), allLibs: !!acc.allLibraries, libs: Object.fromEntries(LIBS.map(([l2]) => [l2, libs.includes(l2)])), maxHeight: acc.maxHeight ?? '', bitrate: acc.bitrate ?? '', streams: acc.streams ?? '', canRequest: !!acc.canRequest,
    limitMode: l.mode, limit: { movie: { ...(own ? l.movie : w.movie) }, tv: { ...(own ? l.tv : w.tv) } } };
}
function abAccDraft(acc) { const k = 'acc_' + acc.id; if (!ui.draft[k]) ui.draft[k] = abAccVals(acc); return ui.draft[k]; }
function abAccDirty(acc) {
  const norm = (x) => JSON.stringify({ ...x, rating: Number(x.rating), maxHeight: String(x.maxHeight), bitrate: String(x.bitrate), streams: String(x.streams), limit: x.limitMode === 'custom' ? JSON.stringify(x.limit) : null });
  return norm(abAccDraft(acc)) !== norm(abAccVals(acc));
}
function abNumFld(path, label, min, max, fid) {
  const v = getPath(path);
  const bad = v !== '' && v != null && (v < min || v > max);
  return `<div class="fld"><label for="${fid}">${label}</label><input id="${fid}" class="inp num" type="number" min="${min}" max="${max}" data-bind="${path}" data-num data-live data-fid="${fid}" value="${esc(v ?? '')}" ${bad ? 'aria-invalid="true"' : ''}><span class="hint">${min} bis ${max}. Leer heisst ohne Limit.</span>${bad ? `<span class="err">Erlaubt sind ${min} bis ${max}.</span>` : ''}</div>`;
}
function abFormFoot(acc, fp, label, only) {
  const dirty = abAccDirty(acc);
  return `<div class="foot"><button type="button" class="btn quiet" data-act="abAccDiscard" data-id="${acc.id}" data-fid="${fp}:discard" ${dirty ? '' : 'disabled'}>Verwerfen</button><button type="button" class="btn primary" data-act="abAccSave" data-id="${acc.id}" ${only ? 'data-only="wish"' : ''} data-fid="${fp}:save">${label}</button></div>`;
}
/* the same form creates nothing and edits everything of «Zugriff», the limit included */
function abAccessForm(acc) {
  abAccDraft(acc);
  const base = 'ui.draft.acc_' + acc.id, fp = 'af:' + acc.id;
  const heights = [{ v: '', t: 'Ohne Limit' }, ...[360, 480, 720, 1080, 1440, 2160].map((h) => ({ v: h, t: `bis ${h}p` }))];
  return `<section class="card ab-form" aria-label="Zugriff von ${esc(acc.email)}"><div class="ch"><span class="ik">${icon('lock', 16)}</span><h2>Zugriff</h2>${abAccDirty(acc) ? '<span class="chip warn">Ungespeicherte Änderungen</span>' : ''}</div>
    <div class="ab-grid2"><div class="fld"><span class="lbl">Höchste Altersfreigabe</span>${selectEl(base + '.rating', AB_RATINGS, { label: 'Höchste Altersfreigabe' })}</div>${abLibsField(base, fp)}</div>
    <fieldset class="ab-fs"><legend>Wiedergabe-Limits</legend><p class="muted ab-small">Gilt für alle Profile dieses Kontos.</p><div class="ab-grid3"><div class="fld"><span class="lbl">Höchste Qualität</span>${selectEl(base + '.maxHeight', heights, { label: 'Höchste Qualität' })}</div>${abNumFld(base + '.bitrate', 'Bandbreite (kbit/s)', 500, 100000, fp + ':br')}${abNumFld(base + '.streams', 'Gleichzeitige Streams', 1, 10, fp + ':st')}</div></fieldset>
    <fieldset class="ab-fs ab-new"><legend>Wünsche <span class="newtag">NEU</span></legend>${abWishBlock(acc, base + '.canRequest')}${abLimitField(acc, base, fp)}</fieldset>
    ${abFormFoot(acc, fp, 'Zugriff speichern')}</section>`;
}
/* administrators have no form «Zugriff»: their block «Wünsche» holds the wish right, the limit and the user line */
function abWishForm(acc) {
  abAccDraft(acc);
  const base = 'ui.draft.acc_' + acc.id, fp = 'af:' + acc.id;
  return `<section class="card ab-form ab-new" aria-label="Wünsche von ${esc(acc.email)}"><div class="ch"><span class="ik">${icon('wish', 16)}</span><h2>Wünsche</h2><span class="newtag">NEU</span>${abAccDirty(acc) ? '<span class="chip warn">Ungespeicherte Änderungen</span>' : ''}</div>
    ${abWishBlock(acc, base + '.canRequest')}${abLimitField(acc, base, fp)}${abFormFoot(acc, fp, 'Speichern', true)}</section>`;
}
function abOwnForm(acc) {
  if (!ui.draft.own) ui.draft.own = { email: acc.email, pw1: '', pw2: '', cur: '' };
  const d = ui.draft.own, e = ui.abOwnErr || {};
  const pwSeerr = ['synced', 'next_login'].includes(accSeerr(acc).pw);
  const f = (k, label, type, hint) => `<div class="fld"><label for="ow:${k}">${label}</label><input id="ow:${k}" class="inp" type="${type}" data-bind="ui.draft.own.${k}" data-fid="ow:${k}" value="${esc(d[k])}" autocomplete="${type === 'password' ? (k === 'cur' ? 'current-password' : 'new-password') : 'email'}" ${e[k] ? 'aria-invalid="true"' : ''}>${hint ? `<span class="hint">${hint}</span>` : ''}${e[k] ? `<span class="err">${e[k]}</span>` : ''}</div>`;
  return `<section class="card ab-form" aria-label="Konto ändern"><div class="ch"><span class="ik">${icon('user', 16)}</span><h2>Konto ändern</h2></div><p class="muted ab-small">E-Mail-Adresse und Passwort deines Kontos. Jede Änderung braucht dein aktuelles Passwort.</p>
    ${f('email', 'E-Mail-Adresse', 'email')}${f('pw1', 'Neues Passwort', 'password', `Leer lassen, um das Passwort zu behalten. Sonst mindestens 10 Zeichen.${pwSeerr ? ' Das neue Passwort gilt auch in Jellyseerr. <span class="newtag">NEU</span>' : ''}`)}${f('pw2', 'Neues Passwort wiederholen', 'password')}${f('cur', 'Aktuelles Passwort', 'password', 'Bestätigt, dass du es bist.')}
    <div class="foot"><button type="button" class="btn quiet" data-act="abOwnDiscard" data-fid="ow:discard">Verwerfen</button><button type="button" class="btn primary" data-act="abOwnSave" data-fid="ow:save">Speichern</button></div></section>`;
}
function abLimitWhere(a) {
  const u = accSeerr(a).user;
  return !u || u.gone ? ' Wird beim Eröffnen in Jellyseerr gesetzt.' : u.admin ? ` In Jellyseerr gilt keines: «${esc(u.name)}» ist dort Administrator.` : ` In Jellyseerr bei «${esc(u.name)}» gesetzt.`;
}
function abAccDetail(acc) {
  const mine = acc.id === curAccount().id;
  if (mine) {
    const profs = profilesOf(acc.id);
    const mp = `<section class="card" id="meine-profile"><div class="ch"><span class="ik">${icon('users', 16)}</span><h2>Meine Profile</h2><a class="btn sm secondary" href="#/settings/profiles" data-fid="ac:prof">Profile verwalten</a></div><ul class="ab-plist">${profs.map((p) => `<li class="ab-prow"><div class="ab-pn"><b>${esc(p.name)}</b><span class="muted ab-small">${p.primary ? 'Hauptprofil' : 'Weiteres Profil'} · ${ageLabel(p.maxRating)}${p.pin ? ' · PIN' : ''}</span></div><div class="ab-pr">${abProfSign(p)}</div></li>`).join('')}</ul></section>`;
    return `<div class="ab-subg">${abOwnForm(acc)}<div class="stack">${abWishForm(acc)}${mp}</div></div>`;
  }
  const profs = `<section class="card"><div class="ch"><span class="ik">${icon('users', 16)}</span><h2>Profile</h2></div><div class="ab-chips">${profilesOf(acc.id).map((p) => `<span class="chip info">${esc(p.name)}${p.primary ? ' (Hauptprofil)' : ''}</span>`).join('')}</div><p class="muted ab-small">Profile verwaltet jedes Konto selbst.</p></section>`;
  if (acc.role === 'admin') return `<div class="ab-subg"><div class="stack">${notice('', 'Administrator', 'Ein Administrator hat immer die Stufe «ab 18» und alle Bibliotheken; «Zugriff» gibt es nur für Konten ohne Verwaltung.')}${abWishForm(acc)}</div>${profs}</div>`;
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
function abInvLimit(inv) { const l = inv.limit || { mode: 'default' }; return l.mode === 'unlimited' ? 'unbegrenzt' : l.mode === 'custom' ? abLimitShort(l) : 'Standard'; }
function abInvRows() {
  if (!S.invitations.length) return '<div class="empty">Noch keine Einladung erstellt.</div>';
  return `<div class="tblw"><table class="tbl" aria-label="Bestehende Einladungen"><thead><tr><th>Notiz</th><th>Zustand</th><th>Altersstufe</th><th>Bibliotheken</th><th>Wünsche <span class="newtag">NEU</span></th><th>Gültig bis / angenommen</th><th class="end">Aktionen</th></tr></thead><tbody>${S.invitations.map((inv) => {
    const [l, c] = AB_INV_STATE[inv.state] || [inv.state, 'info'];
    return `<tr><td class="main">${inv.note ? esc(inv.note) : '<span class="muted">Ohne Notiz</span>'}</td><td><span class="chip ${c}">${l}</span></td><td>${ageLabel(inv.maxRating)}</td><td>${inv.allLibraries === false ? abCount((inv.libs || []).length, 'Bibliothek', 'Bibliotheken') : 'Alle Bibliotheken'}</td><td><span class="ab-who">${inv.canRequest ? '<span class="chip ok">Darf anfordern</span>' : '<span class="chip info">Nicht erlaubt</span>'}<span>Limit: ${abInvLimit(inv)}</span></span></td><td class="nowrap">${inv.usedBy ? `Angenommen von ${esc((account(inv.usedBy) || { email: inv.usedBy }).email)}` : `Gültig bis ${fmtDate(inv.until)}`}</td><td class="end">${inv.state === 'open' ? `<span class="ta"><a class="btn sm secondary" href="#/invite/${inv.id}" data-fid="inv:op:${inv.id}">${icon('external', 16)}Einladungslink öffnen</a><button type="button" class="btn sm dq" data-act="dialog" data-name="abRevoke" data-id="${inv.id}" data-fid="inv:rv:${inv.id}">Widerrufen</button></span>` : ''}</td></tr>`;
  }).join('')}</tbody></table></div>`;
}
route('/admin/accounts', (params, query = {}) => {
  /* `#/admin/accounts?open=<accountId>` (hint «Limit erreicht»): open that account's details */
  abTakeQuery('#/admin/accounts', () => { if (!query.open || !account(query.open)) return false; ui.open['ac:' + query.open] = true; ui.filter.accTab = 'accounts'; ui.abAccJump = query.open; return true; });
  const jump = ui.abAccJump;
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
  return { type: 'admin', area: 'people', page: 'accounts', title: 'Konten', intro: 'Alle Konten mit Rolle, Altersstufe, Bibliotheken, Profilen und den Kanälen, über die sie Benachrichtigungen bekommen. Neue Konten entstehen über eine Einladung. Die Profile deines eigenen Kontos verwaltest du hier; die Profile anderer Konten verwaltet jedes Konto selbst.', actions: `<button type="button" class="btn primary" data-act="abInvite" data-fid="ac:invite">${icon('plus', 16)}Konto einladen</button>`, html: `${stats}${tabs}${body}`,
    after: (root) => { if (!jump) return; ui.abAccJump = null; const el = root.querySelector(`[data-fid="ac:tog:${jump}"]`); if (el) { el.scrollIntoView({ block: 'start' }); el.focus({ preventScroll: true }); } } };
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
/* deleting an account (07, section 8; 04, sections 10 and 15): a wish that a profile of another account joined goes
   to the oldest of these profiles; every other wish of the account is closed, open ones as `failed` / `account_deleted` */
function abRemovePlan(a) {
  const pids = profilesOf(a.id).map((p) => p.id);
  const owned = S.wishes.filter((w) => pids.includes(w.owner));
  const take = owned.filter((w) => w.joined.some((j) => !pids.includes(j) && profile(j)));
  const close = owned.filter((w) => !take.includes(w));
  return { pids, take, close, open: close.filter((w) => OPEN_STATES.includes(w.state)) };
}
function abRemoveSeerrText(a) {
  const st = accSeerr(a), u = st.user;
  if (st.state === 'suggested' && st.suggest) return `Die vorgeschlagene Verknüpfung mit «${esc(st.suggest.name)}» fällt weg. Der Jellyseerr-Benutzer bleibt unverändert.`;
  if (!u) return 'Das Konto hat keinen Jellyseerr-Benutzer.';
  if (u.origin === 'slouchcat' && !u.admin) return 'Wünsche bleiben in Jellyseerr bestehen. Der Jellyseerr-Benutzer bleibt, verliert seine Rechte und bekommt ein neues Zufallspasswort.';
  return 'Wünsche bleiben in Jellyseerr bestehen. Der Jellyseerr-Benutzer bleibt unverändert.';
}
DIALOGS.abRemove = ({ id }) => {
  const a = account(id), pl = abRemovePlan(a);
  const wishes = [
    pl.open.length ? `${pl.open.length === 1 ? 'Ein offener Wunsch des Kontos wird' : `${pl.open.length} offene Wünsche des Kontos werden`} in Slouchcat geschlossen: «${FAIL_TEXT.account_deleted}»` : '',
    pl.close.length - pl.open.length ? `${abCount(pl.close.length - pl.open.length, 'abgeschlossener Wunsch bleibt', 'abgeschlossene Wünsche bleiben')} im Verlauf.` : '',
    pl.take.length ? `${pl.take.length === 1 ? 'Einen Wunsch übernimmt' : `${pl.take.length} Wünsche übernimmt`} das Profil eines anderen Kontos, das mitwünscht (${pl.take.map((w) => `«${esc(T[w.titleKey].title)}» → ${esc(profile(w.joined.find((j) => !pl.pids.includes(j) && profile(j))).name)}`).join(', ')}).` : '',
  ].filter(Boolean);
  return {
    title: `Konto ${esc(a.email)} entfernen?`,
    body: `<p class="lead">Konto, Profile und Fortschritt werden gelöscht. Laufende Sitzungen und Wiedergaben enden sofort. Downloads auf den Geräten des Kontos verschwinden beim nächsten Kontakt mit dem Server. Ein Gerät, das nie mehr online kommt, behält sie.</p>
      <p class="sec">${abRemoveSeerrText(a)} <span class="newtag">NEU</span></p>
      ${wishes.length ? `<ul class="ab-idl">${wishes.map((x) => `<li>${x}</li>`).join('')}</ul>` : ''}`,
    actions: `<button type="button" class="btn ghost" data-act="close" data-fid="rm:x">Abbrechen</button><button type="button" class="btn danger" data-act="abRemoveOk" data-id="${id}" data-fid="rm:ok" data-autofocus>${icon('trash', 16)}Konto entfernen</button>`,
  };
};
ACT.abRemoveOk = (d) => {
  const a = account(d.id), pl = abRemovePlan(a), pids = pl.pids, now = Date.now();
  pl.take.forEach((w) => {
    const next = w.joined.find((j) => !pids.includes(j) && profile(j));
    w.owner = next;
    w.joined = w.joined.filter((j) => j !== next);
    ev(w, 'handover', `Konto ${a.email} gelöscht, ${profile(next).name} übernimmt den Wunsch`);
  });
  pl.close.forEach((w) => {
    if (OPEN_STATES.includes(w.state)) { w.state = 'failed'; w.failReason = 'account_deleted'; w.closed = now; delete w.dl; ev(w, 'failed', FAIL_TEXT.account_deleted); }
    else w.closed = w.closed || now;
    w.owner = null; w.goneAccount = a.email;
  });
  S.wishes.forEach((w) => { w.joined = w.joined.filter((j) => !pids.includes(j)); });
  S.notifications = S.notifications.filter((n) => !pids.includes(n.profileId));
  pids.forEach((p) => delete S.map.profiles[p]);
  const su = seerrUser(S.map.accounts[a.id]);
  const cut = su && !su.admin && su.origin === 'slouchcat';
  if (cut) { su.canMovie = su.canShow = false; logSeerr('Benutzer', 'ok', `«${su.name}»: Recht zum Anfordern entzogen, Zufallspasswort gesetzt, Benutzer bleibt`); }
  else if (su) logSeerr('Benutzer', 'ok', `«${su.name}» bleibt unverändert`);
  delete S.map.accounts[a.id];
  S.profiles = S.profiles.filter((p) => p.accountId !== a.id);
  S.deliveries = S.deliveries.filter((x) => x.accountId !== a.id);
  S.accounts = S.accounts.filter((x) => x.id !== a.id);
  closeDialog(); commit();
  const ws = [pl.open.length ? `${abCount(pl.open.length, 'offener Wunsch', 'offene Wünsche')} geschlossen` : '', pl.take.length ? `${abCount(pl.take.length, 'Wunsch', 'Wünsche')} übernommen` : ''].filter(Boolean).join(', ');
  toast(`Konto ${esc(a.email)} entfernt.${ws ? ` ${ws}.` : ''}${su ? ` «${esc(su.name)}» bleibt in Jellyseerr${cut ? ', ohne Rechte und mit Zufallspasswort' : ' unverändert'}.` : ''}`, 'ok', 6500);
};
ACT.abAccDiscard = (d) => { delete ui.draft['acc_' + d.id]; render(); toast('Änderungen verworfen.', 'ok', 2500); };
/* «Zugriff speichern» (and «Speichern» of the block «Wünsche» of an administrator, data-only="wish") */
ACT.abAccSave = (d) => {
  const a = account(d.id), dr = abAccDraft(a), only = d.only === 'wish';
  if (!only) {
    if (!dr.allLibs && !Object.values(dr.libs).some(Boolean)) { toast('Wähle mindestens eine Bibliothek.', 'bad'); return; }
    const bad = (v, lo, hi) => v !== '' && v != null && (v < lo || v > hi);
    if (bad(dr.bitrate, 500, 100000) || bad(dr.streams, 1, 10)) { toast('Prüfe die Wiedergabe-Limits.', 'bad'); return; }
  }
  if (dr.limitMode === 'custom' && abLimitErr(`ui.draft.acc_${a.id}.limit`)) { toast('Nicht gespeichert: Prüfe das Limit (Anzahl 0 bis 100, Tage 1 bis 90).', 'bad'); return; }
  if (!only) {
    a.maxRating = Number(dr.rating); a.allLibraries = dr.allLibs; a.libs = Object.keys(dr.libs).filter((k) => dr.libs[k]);
    a.maxHeight = dr.maxHeight === '' ? '' : Number(dr.maxHeight); a.bitrate = dr.bitrate; a.streams = dr.streams;
  }
  const rightChanged = a.canRequest !== dr.canRequest;
  a.canRequest = dr.canRequest;
  const before = JSON.stringify(a.limit || { mode: 'default' });
  a.limit = dr.limitMode === 'custom' ? { mode: 'custom', movie: { n: Number(dr.limit.movie.n), days: Number(dr.limit.movie.days) }, tv: { n: Number(dr.limit.tv.n), days: Number(dr.limit.tv.days) } } : { mode: dr.limitMode };
  const limChanged = JSON.stringify(a.limit) !== before;
  if (rightChanged) syncRights(a.id);
  delete ui.draft['acc_' + a.id];
  commit();
  const u = accSeerr(a).user;
  const right = !rightChanged || !u ? '' : !u.admin && u.origin === 'slouchcat' ? ` Die Rechte von «${esc(u.name)}» in Jellyseerr werden angepasst.` : ` «${esc(u.name)}» behält in Jellyseerr seine Rechte, weil Slouchcat ihn nicht eröffnet hat.`;
  const lim = limChanged ? ` Limit: ${a.limit.mode === 'unlimited' ? 'unbegrenzt' : abLimitShort(limitOf(a))}.${abLimitWhere(a)}` : '';
  toast(`${only ? 'Wünsche' : 'Zugriff'} von ${esc(a.email)} gespeichert.${right}${lim}`, 'ok', 6500);
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
  const pwNew = !!d.pw1;
  delete ui.draft.own;
  let msg = 'Dein Konto ist gespeichert.';
  if (pwNew) {
    const r = pwChanged(a.id);
    msg += { synced: ' Das neue Passwort gilt auch in Jellyseerr.', next_login: ' In Jellyseerr folgt das neue Passwort bei deiner nächsten Anmeldung: Jellyseerr ist gerade nicht erreichbar.', admin: ' Dein Passwort in Jellyseerr bleibt, wie es ist: Du bist dort Administrator.', external: ' In Jellyseerr meldest du dich über Jellyfin oder Plex an; dort ändert sich nichts.', off: ' In Jellyseerr bleibt das bisherige Passwort: Der Abgleich ist für dieses Konto aus.' }[r] || '';
  }
  commit();
  toast(msg, 'ok', 6500);
};
ACT.abInvite = () => {
  ui.draft.invite = { note: '', rating: 16, allLibs: true, libs: Object.fromEntries(LIBS.map(([l]) => [l, false])), canRequest: !!S.settings.wishes.newAccounts, link: '', limitMode: 'default', limit: { movie: { ...S.settings.wishes.limit.movie }, tv: { ...S.settings.wishes.limit.tv } } };
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
      <div class="ab-swl ab-new">${sw('ui.draft.invite.canRequest', 'Darf Titel anfordern')}<span class="ab-who"><b>Darf Titel anfordern <span class="newtag">NEU</span></b><span>Vorgabe aus «Neue Konten dürfen anfordern» unter Wünsche › Einstellungen. Die Rechte in Jellyseerr folgen diesem Schalter.</span></span></div>
      <div class="fld ab-new"><span class="lbl">Limit <span class="newtag">NEU</span></span>${abSeg('ui.draft.invite.limitMode', AB_LIM_MODES, { label: 'Limit' })}${d.limitMode === 'custom' ? abLimitEditor('ui.draft.invite.limit', 'iv:lim') : `<span class="hint">${d.limitMode === 'unlimited' ? 'Ohne Limit.' : `Standard: ${abLimitShort(S.settings.wishes.limit)}.`} Gilt für alle Profile des Kontos zusammen.</span>`}</div>
      ${S.settings.wishes.provision && S.seerr.configured && S.seerr.enabled ? `<p class="ab-line ab-new">${icon('info', 16)}<span>Beim Annehmen eröffnet Slouchcat das Konto auch in Jellyseerr: gleiche E-Mail, gleiches Passwort.</span><span class="newtag">NEU</span></p>` : ''}`,
    actions: `<button type="button" class="btn ghost" data-act="close" data-fid="iv:x">Abbrechen</button><button type="button" class="btn primary" data-act="abInviteCreate" data-fid="iv:ok" ${none ? 'disabled' : ''}>Einladung erstellen</button>`,
  };
};
ACT.abInviteCreate = () => {
  const d = ui.draft.invite;
  if (d.limitMode === 'custom' && abLimitErr('ui.draft.invite.limit')) { toast('Prüfe das Limit: Anzahl 0 bis 100, Tage 1 bis 90.', 'bad'); return; }
  const tok = Math.random().toString(36).slice(2, 10) + Math.random().toString(36).slice(2, 10);
  S.invitations.unshift({ id: 'i' + ++S.seq, note: d.note.trim(), state: 'open', maxRating: Number(d.rating), allLibraries: d.allLibs, libs: Object.keys(d.libs).filter((k) => d.libs[k]), canRequest: d.canRequest, limit: d.limitMode === 'custom' ? { mode: 'custom', movie: { ...d.limit.movie }, tv: { ...d.limit.tv } } : { mode: d.limitMode }, until: Date.now() + 7 * D });
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
const AB_SPOT_MISS = [];
const AB_SPOT_NUM = { itemCount: [5, 20], minHits: [0, 10], intervalHours: [6, 48], rotationSeconds: [3, 15] };
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
    ${abSpotNum('minHits', 'Ausblenden unter', 'Treffern in der Bibliothek, je Profil (0–10).')}
    ${abSpotNum('intervalHours', 'Abruf alle … Stunden', '6 bis 48 Stunden')}
    ${abSpotNum('rotationSeconds', 'Spotlight Wechsel Takt', '3 bis 15 Sekunden')}
    <div class="ab-fetch"><p class="ab-line">${ui.busy.spot ? '<span class="spin"></span>Die Liste wird abgerufen …' : `${icon('clock', 16)}<span>Zuletzt abgerufen ${fmtDate(fetched)}, ${fmtTime(fetched)} · ${f.hits} von ${f.entries} Titeln in der Bibliothek · Schlüssel gesetzt</span>`}</p>${why ? `<p class="sec ab-small">${why}</p>` : ''}</div>
    <div class="foot"><button type="button" class="btn secondary" data-act="abSpotFetch" data-fid="sp:fetch" ${ui.busy.spot ? 'disabled' : ''}>${icon('refresh', 16)}Jetzt abrufen</button><button type="button" class="btn primary" data-act="abSpotSave" data-fid="sp:save1">Speichern</button></div>`;
  const meta = `${dirty ? '<span class="chip warn">Ungespeicherte Änderungen</span>' : ''}<span>${esc((AB_LISTS.find((x) => x[0] === c.listKey) || AB_LISTS[0])[1])} · ${esc((AB_MEDIA.find((x) => x[0] === c.media) || AB_MEDIA[2])[1])}</span><span class="chip ${shown ? 'ok' : 'warn'}">${shown ? 'Auf der Startseite' : 'Fehlt auf der Startseite'}</span>`;
  return band('spot:src', 'Quelle', meta, body, { icon: 'list' });
}
/* why «Anfordern» is not offered on this page: Jellyseerr not set up or off, «Wünsche aktiv» off (05, section 6) */
function abSpotNoRequest() {
  if (!S.seerr.configured) return 'Jellyseerr ist nicht eingerichtet.';
  if (!S.seerr.enabled) return '«Jellyseerr verwenden» ist ausgeschaltet.';
  if (!S.settings.wishes.enabled) return '«Wünsche aktiv» ist ausgeschaltet.';
  return '';
}
/* DEMO DATA (mockup only): one Trakt entry that cannot be matched. Trakt gives only a TVDB id, and TMDB «find by
   external id» does not know it (N-62), so it is neither present nor missing. The title is invented. */
const AB_UNMATCHED = [{ key: 'demo-unmatched', title: 'Hafen im Nordlicht', kind: 'series', year: 2026, tmdbId: null, tvdbId: 459812, imdbId: null, rank: SPOT_ORDER.length + 1 }];
const AB_UNMATCHED_TEXT = 'Nicht zuordenbar: nur TVDB-ID bekannt, bei TMDB nicht gefunden';
function abMissingState(k) { const w = openWishes(k)[0]; return w ? wsChip(w.state, 'Angefordert · ' + STATE_LABEL[w.state]) : missingChip(); }
function abMissingBand() {
  const f = abSpotFacts(), why = abSpotNoRequest();
  const rows = f.miss.map((t) => {
    const ask = !why && !openWishes(t.key).length ? `<button type="button" class="btn sm secondary" data-act="askWish" data-key="${t.key}" aria-label="${esc(t.title)} anfordern" data-fid="sm:rq:${t.key}">${icon('plus', 16)}Anfordern</button>` : '';
    return `<li>${abMini(t, `${kindLabel(t)} · ${t.year}`)}<span class="ab-st">${abMissingState(t.key)}</span>${ask ? `<span class="ta">${ask}</span>` : ''}</li>`;
  }).join('');
  const hint = why ? notice('warn', 'Anfordern geht erst, wenn Wünsche eingerichtet sind.', esc(why), `<a class="btn sm secondary" href="#/admin/wishes/settings" data-fid="sm:set">${icon('settings', 16)}Zu den Einstellungen der Wünsche</a>${!S.seerr.configured || !S.seerr.enabled ? `<a class="btn sm secondary" href="#/admin/integrations/jellyseerr" data-fid="sm:conn">${icon('plug', 16)}Zur Verbindung</a>` : ''}`) : '';
  const body = `<p class="ab-line">${icon('info', 16)}<span>Die Startseite und die Spotlight-Liste zeigen nur Titel aus der Bibliothek. Einträge, die fehlen, siehst nur du hier in der Verwaltung. Profile fordern fehlende Titel unter «Wünsche» an.</span></p>
    <p class="ab-line">${icon('chart', 16)}<span>${abCount(f.miss.length, 'Eintrag', 'Einträge')} der Liste ${f.miss.length === 1 ? 'fehlt' : 'fehlen'} in der Bibliothek, ${f.req.length} davon ${f.req.length === 1 ? 'ist' : 'sind'} angefordert.</span></p>
    ${hint}${rows ? `<ul class="ab-missl" aria-label="Fehlende Einträge">${rows}</ul>` : '<p class="muted">Jeder Eintrag der Liste steht in der Bibliothek.</p>'}
    ${!why ? `<p class="muted ab-small">«Anfordern» handelt als aktives Profil (${esc(curProfile().name)}).</p>` : ''}`;
  return band('spot:miss', 'Fehlende Einträge <span class="newtag">NEU</span>', `<span>${f.miss.length} fehlen · ${f.req.length} angefordert</span>${why ? '<span class="chip warn">Anfordern nicht möglich</span>' : ''}`, body, { icon: 'wish' });
}
function abOv(key) { const o = S.settings.spotlight.overrides; return (o[key] = o[key] || {}); }
function abOvClean(key) { const o = S.settings.spotlight.overrides; if (o[key] && !o[key].hidden && !o[key].pinned && !o[key].note) delete o[key]; }
function abEntriesBand() {
  const ov = S.settings.spotlight.overrides || {}, why = abSpotNoRequest();
  const rows = SPOT_ORDER.map((k, i) => {
    const t = T[k], o = ov[k] || {}, has = isPresent(k), w = openWishes(k)[0];
    const st = o.hidden ? '<span class="chip info">Ausgeblendet</span>' : has ? '<span class="chip ok">Sichtbar</span>' : w ? wsChip(w.state, 'Angefordert · ' + STATE_LABEL[w.state]) : missingChip();
    const acts = [
      o.pinned ? `<button type="button" class="btn sm quiet" data-act="abUnpin" data-key="${k}" aria-label="${esc(t.title)} lösen" data-fid="se:up:${k}">${icon('pin', 16)}Lösen</button>` : `<button type="button" class="btn sm quiet" data-act="abPinAsk" data-key="${k}" aria-label="${esc(t.title)} anheften" data-fid="se:pin:${k}">${icon('pin', 16)}Anheften</button>`,
      `<button type="button" class="btn sm quiet" data-act="abHide" data-key="${k}" aria-label="${esc(t.title)} ${o.hidden ? 'einblenden' : 'ausblenden'}" data-fid="se:hd:${k}">${icon(o.hidden ? 'eye' : 'ban', 16)}${o.hidden ? 'Einblenden' : 'Ausblenden'}</button>`,
    ];
    if (!has && !w && !why) acts.push(`<button type="button" class="btn sm secondary" data-act="askWish" data-key="${k}" aria-label="${esc(t.title)} anfordern" data-fid="se:rq:${k}">${icon('plus', 16)}Anfordern</button>`);
    return `<tr class="${has ? '' : 'ab-missrow'}"><td class="nowrap">${i + 1}</td><td class="main">${abMini(t, `${kindLabel(t)} · ${t.year}`)}<span class="ab-ids"><span class="mono">${esc(idLine(t))}</span>${t.kind === 'series' && ID_MATCH[k] && ID_MATCH_TEXT[ID_MATCH[k]] ? `<span class="chip outline">${esc(ID_MATCH_TEXT[ID_MATCH[k]])}</span>` : ''}</span></td><td><span class="ab-who">${st}${o.pinned ? `<span>Angeheftet auf Platz ${o.pinned}</span>` : ''}</span></td><td><span class="ab-note">${o.note ? `«${esc(o.note)}»` : '–'}<button type="button" class="btn sm quiet" data-act="abNoteAsk" data-key="${k}" aria-label="Notiz zu ${esc(t.title)}" data-fid="se:nt:${k}">${icon('note', 16)}Notiz</button></span></td><td class="end"><span class="ta">${acts.join('')}</span></td></tr>`;
  }).join('') + AB_UNMATCHED.map((x) => `<tr class="ab-missrow"><td class="nowrap">${x.rank}</td><td class="main"><span class="ab-who"><b>${esc(x.title)}</b><span>${x.kind === 'series' ? 'Serie' : 'Film'} · ${x.year}</span></span><span class="ab-ids"><span class="mono">${esc(idLine(x))}</span></span></td><td><span class="chip warn">${icon('alert', 12)}${AB_UNMATCHED_TEXT}</span></td><td>–</td><td class="end"><span class="muted ab-small">Keine Aktion: ohne TMDB-ID nicht anforderbar</span></td></tr>`).join('');
  const nMiss = SPOT_ORDER.filter((k) => !isPresent(k)).length;
  const body = `<p class="sec ab-small">Reihenfolge von Trakt; eigene Änderungen bleiben beim nächsten Abruf erhalten. Fehlende Einträge sieht nur die Verwaltung; sie haben neu ein Poster, ihren Zustand und die Aktion «Anfordern» <span class="newtag">NEU</span>. ${why ? `«Anfordern» fehlt: ${esc(why)}` : `«Anfordern» handelt als aktives Profil (${esc(curProfile().name)}).`}</p>
    <div class="tblw"><table class="tbl" aria-label="Einträge der Liste"><thead><tr><th>Rang</th><th>Titel</th><th>Status</th><th>Eigene Notiz</th><th class="end">Aktionen</th></tr></thead><tbody>${rows}</tbody></table></div>`;
  return band('spot:entries', 'Einträge der Liste', `<span>${abCount(SPOT_ORDER.length + AB_UNMATCHED.length, 'Eintrag', 'Einträge')}, ${nMiss} fehlen, ${AB_UNMATCHED.length} nicht zuordenbar</span>`, body, { icon: 'film' });
}
/* «Abgleich über TMDB, TVDB und IMDb» (07, section 9; 05, sections 3.3 and 4) */
function abIdsBand() {
  const f = abSpotFacts(), list = abSpotList();
  const ways = {};
  list.forEach((t) => {
    const w = t.kind === 'movie' ? (isPresent(t.key) ? 'movie' : null) : ID_MATCH[t.key] || (isPresent(t.key) ? 'tvdb' : null);
    if (w) ways[w] = (ways[w] || 0) + 1;
  });
  const WAY = { movie: 'Filme in der Bibliothek über die TMDB-ID erkannt', ...ID_MATCH_TEXT };
  const counts = ['movie', 'tvdb', 'tmdb', 'both', 'resolved'].filter((k) => ways[k]).map((k) => `<li><b>${ways[k]}</b> · ${esc(WAY[k])}</li>`).join('');
  const tmdb = (S.seerr.seasonsFrom || 'tmdb') === 'tmdb';
  const seasons = `<p class="ab-line">${icon('tv', 16)}<span>Jellyseerr liest die Staffeln aus <b>${tmdb ? 'TMDB' : 'TVDB'}</b>.</span><span class="chip ${tmdb ? 'warn' : 'ok'}">Staffeln aus ${tmdb ? 'TMDB' : 'TVDB'}</span></p>${tmdb ? notice('warn', 'Empfehlung: in Jellyseerr den Metadaten-Anbieter für Serien auf TVDB stellen', 'Dann zählen Jellyseerr, Sonarr und Slouchcat Staffeln gleich. Umstellen musst du in Jellyseerr selbst.') : ''}`;
  const un = AB_UNMATCHED.map((x) => `<li><span class="ab-who"><b>${esc(x.title)}</b><span>${x.kind === 'series' ? 'Serie' : 'Film'} · ${x.year} · Rang ${x.rank}</span></span><span class="mono ab-small">${esc(idLine(x))}</span><span class="chip warn">${icon('alert', 12)}${AB_UNMATCHED_TEXT}</span></li>`).join('');
  const body = `<p class="ab-line">${icon('info', 16)}<span>Filme gleicht Slouchcat über TMDB, dann IMDb ab. Serien wie die Bibliothek: TVDB zuerst, dann TMDB, dann IMDb. Jellyseerr führt jeden Titel über die TMDB-ID und gibt bei Serien die TVDB-ID an Sonarr weiter.</span></p>
    <h3 class="ab-h3">Erkannt nach Weg</h3>${counts ? `<ul class="ab-idl">${counts}</ul>` : '<p class="muted">Kein Eintrag in der Bibliothek.</p>'}
    <h3 class="ab-h3">Staffeln in Jellyseerr</h3>${seasons}
    <h3 class="ab-h3">Nicht zuordenbar</h3>${un ? `<ul class="ab-missl" aria-label="Nicht zuordenbare Einträge">${un}</ul><p class="muted ab-small">Ohne TMDB-ID ist ein Eintrag nicht anforderbar; er zählt weder als vorhanden noch als fehlend.</p>` : '<p class="muted">Jeder Eintrag liess sich zuordnen.</p>'}`;
  return band('spot:ids', 'Abgleich über TMDB, TVDB und IMDb <span class="newtag">NEU</span>', `<span>${f.hits} in der Bibliothek · ${f.miss.length} fehlen · ${AB_UNMATCHED.length} nicht zuordenbar</span>`, body, { icon: 'link' });
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
  const html = `${notice('', 'Die Startseite zeigt weiterhin nur Vorhandenes', 'Fehlende Einträge der Liste erscheinen weder auf der Startseite noch in der Spotlight-Liste. Neu siehst du sie hier in der Verwaltung, mit Zustand und der Aktion «Anfordern». Neu erkennt das Spotlight vorhandene Serien auch über die TVDB- und die IMDb-ID.')}
    ${abSourceBand()}${abMissingBand()}${abEntriesBand()}${abIdsBand()}${abOrphansBand()}
    <p class="sec ab-small">Wo die Reihe auf der Startseite steht und ob sie erscheint, stellst du unter «Startseite» ein. <a class="lnk" href="#/admin/home-rows" data-fid="sp:rows">Zur Seite «Startseite»</a></p>`;
  return { type: 'admin', area: 'home', page: 'spotlight', title: 'Spotlight', intro: 'Eine öffentliche Trakt-Liste als Reihe der Startseite. Gezeigt werden nur Titel aus der eigenen Bibliothek; jedes Profil sieht nur, was seine Altersstufe erlaubt.', html };
});
ACT.abSpotList = () => { const d = abSpotDraft(); if (d.listKey === 'boxoffice') d.media = 'movie'; };
ACT.abSpotSave = () => {
  const d = abSpotDraft();
  const errs = [...Object.keys(AB_SPOT_NUM), 'listUrl'].map(abSpotErr).filter(Boolean);
  if (errs.length) { ui.abSpotShowErr = true; ui.open['spot:src'] = true; render(); toast('Nicht gespeichert: Prüfe die markierten Felder.', 'bad'); return; }
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
  spotlight: ['Die Trakt-Liste, geschnitten mit der Bibliothek. Erscheint als Bühne mit Hintergrundbild an ihrem Platz; unter der Mindestzahl Treffer fehlt sie.', '#/admin/spotlight', 'Spotlight'],
  picks: ['Titel, die Profile des Besitzerkontos empfehlen; neueste zuerst, mit Ablauf.', '#/admin/x/picks', 'User-Empfehlungen'],
  continue: ['Angefangene Titel des Profils.'],
  recent: ['Die zuletzt in eine Bibliothek gekommenen Titel.'],
  released: ['Nach Erscheinungsdatum der letzten zwölf Monate; eine Serie nach ihrer neusten Folge.'],
  watchlist: ['Die Merkliste des Profils.'],
  collections: ['Eigene Sammlungen und die von TMDB.', '#/admin/x/collections', 'Sammlungen'],
  genres: ['Die Genres der sichtbaren Titel.'],
  libraries: ['Kacheln für die Bibliotheken, die das Profil sieht, für Radio und, wenn eingeblendet, für Sammlungen und User-Empfehlungen; Reihenfolge und Ausblenden regelt die Seite «Meine Medien».', '#/admin/x/my-media', 'Meine Medien'],
  wishes: ['Wünsche des Profils, die in den letzten 14 Tagen erfüllt wurden, also Titel, die jetzt in der Bibliothek stehen. Offene Wünsche stehen unter «Wünsche», nicht auf der Startseite.', '#/admin/wishes', 'Wünsche'],
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
  const prev = `<p class="sec ab-small">Schema mit Platzhaltern, in der ungespeicherten Reihenfolge. Auf der echten Startseite fehlen ausgeblendete Reihen, und leere Reihen entfallen ebenfalls (zum Beispiel eine leere Merkliste oder «Erfüllte Wünsche» bei Profilen ohne erfüllten Wunsch in den letzten 14 Tagen).</p>
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
