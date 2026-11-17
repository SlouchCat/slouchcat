/* M-02 Spotlight-Liste, M-03/M-04 Vorschauseite (fehlender Titel), M-05/M-06 Detailseite (vorhandener Titel). */
'use strict';

ui.season = ui.season || {};
const VT_LIST_LABEL = { trending: 'Trending bei Trakt', popular: 'Beliebt bei Trakt', anticipated: 'Erwartet bei Trakt', watched: 'Meistgesehen bei Trakt', custom: 'Eigene Trakt-Liste' };

/* «not visible» is a 404 without any hint at the age rating */
function vtNotFound() {
  return { type: 'viewer', nav: '', title: 'Titel nicht gefunden', html: `<div class="page"><div class="head"><h1>Titel nicht gefunden</h1><p class="sub">Diesen Titel gibt es nicht oder nicht mehr.</p><div class="acts"><a class="btn secondary" href="#/" data-fid="nf:home">Zur Startseite</a></div></div></div>` };
}
/* switch to the other page of the same title without leaving a history entry (back must not loop) */
function vtRedirect(hash) {
  history.replaceState(null, '', hash);
  render();
  return { type: 'viewer', nav: '', title: '', html: '' };
}
function vtEpCount(s) { return s.episodes ? `${s.episodes} ${s.episodes === 1 ? 'Folge' : 'Folgen'}` : ''; }
function vtSeasonLine(s) { return [s.year, vtEpCount(s)].filter(Boolean).join(' · '); }
function vtPlural(ns) { return ns.length === 1 ? `Staffel ${ns[0]}` : `Staffeln ${ns.slice(0, -1).join(', ')} und ${ns[ns.length - 1]}`; }

/* state of one missing season for the active profile: free, own open wish, someone else's wish, announced */
function vtSeasonState(t, n, p) {
  if (upcoming(t.key).includes(n)) return { k: 'upcoming', chip: `<span class="ws queued">${icon('clock', 14)}Angekündigt</span>` };
  const w = openWish(t.key, n);
  if (!w) return { k: 'free', chip: `<span class="ws missing">${icon('plus', 14)}Fehlt</span>` };
  const st = w.state === 'partial' ? (w.seasonState && w.seasonState[n]) || 'processing' : w.state;
  if (isMine(w, p)) return { k: 'mine', w, st, chip: wsChip(st) };
  return { k: 'other', w, st, chip: wsChip('queued', 'Bereits angefordert') };
}
/* ghost card of a season that is not in the library */
function vtGhost(t, s, p, ctx, clickable) {
  const ss = vtSeasonState(t, s.n, p);
  const inner = `<span class="frame">${img(s.poster, s.name)}<span class="ribbon">${ss.chip}</span></span><span class="cap"><b>${esc(s.name)}</b><span>${esc(vtSeasonLine(s))}</span></span>`;
  let el;
  if (ss.k === 'free' && clickable) el = `<button type="button" class="pl" data-act="vtAskSeason" data-key="${t.key}" data-n="${s.n}" aria-label="${esc(s.name)} anfordern" data-fid="${ctx}:s:${s.n}">${inner}</button>`;
  else if (ss.k === 'mine') el = `<a class="pl" href="#/wishes" aria-label="${esc(s.name)}: ${esc(STATE_LABEL[ss.st])}, zu meinen Wünschen" data-fid="${ctx}:s:${s.n}">${inner}</a>`;
  else el = `<div class="pl">${inner}</div>`;
  return `<li class="pc ghost">${el}</li>`;
}
function vtCast(t, ctx) {
  if (!t.cast || !t.cast.length) return '';
  return `<section class="row" aria-label="Besetzung"><div class="rowhead"><h2>Besetzung</h2><div class="paging"><button type="button" class="btn ghost icon" data-act="scrollRow" data-row="${ctx}:cast" data-dir="prev" aria-label="Zurückblättern" data-fid="${ctx}:cast:prev">${icon('chevL')}</button><button type="button" class="btn ghost icon" data-act="scrollRow" data-row="${ctx}:cast" data-dir="next" aria-label="Weiterblättern" data-fid="${ctx}:cast:next">${icon('chevR')}</button></div></div><div class="scroller" data-keep="${ctx}:cast"><ul>${t.cast.map((c) => `<li class="person"><span class="pt">${img(c.img, c.name)}</span><b>${esc(c.name)}</b><span>${esc(c.role || '')}</span></li>`).join('')}</ul></div></section>`;
}
/* backdrop, back button, poster column and the title block shared by both pages */
function vtTop(t, ctx, posterExtra, info) {
  const orig = t.original && t.original !== t.title ? `<p class="orig">Originaltitel: ${esc(t.original)}</p>` : '';
  const genres = (t.genres || []).length ? `<p class="genres">${esc(t.genres.join(', '))}</p>` : '';
  return `<div class="bd">${img(t.backdrop, t.title)}</div>
    <span class="back"><button type="button" class="btn overlay icon" data-act="back" aria-label="Zurück" data-fid="${ctx}:back">${icon('arrowL')}</button></span>
    <div class="lay"><div class="poster">${img(t.poster, t.title)}${posterExtra || ''}</div>
      <div class="info"><div><h1>${esc(t.title)}</h1>${orig}</div>${metaLine(t, { age: true })}${genres}${t.tagline ? `<p class="tagline">${esc(t.tagline)}</p>` : ''}${info}${t.overview ? `<p class="overview">${esc(t.overview)}</p>` : ''}</div></div>`;
}

/* ---------- M-02 Spotlight-Liste ---------- */
route('/lists/spotlight', () => {
  const p = curProfile(), cfg = S.settings.spotlight;
  const ov = cfg.overrides || {};
  let list = SPOT_ORDER.map((k) => T[k]).filter((t) => !(ov[t.key] && ov[t.key].hidden));
  if (cfg.media === 'movie') list = list.filter((t) => t.kind === 'movie');
  if (cfg.media === 'show') list = list.filter((t) => t.kind === 'series');
  /* as today: only titles of the library; what is missing shows up under «Wünsche», never here */
  list = cfg.enabled ? list.filter((t) => isPresent(t.key) && titleVisible(p, t)) : [];
  let body;
  if (!cfg.enabled) body = `<div class="empty">Das Spotlight ist ausgeschaltet.</div>`;
  else if (!list.length) body = `<div class="empty">Von dieser Liste ist noch nichts in der Bibliothek.</div>`;
  else body = `<ul class="grid">${list.map((t) => posterCard(t, { ctx: 'list' })).join('')}</ul>`;
  const sub = `${esc(VT_LIST_LABEL[cfg.listKey] || 'Trakt-Liste')} · ${list.length} Titel`;
  return { type: 'viewer', nav: 'start', title: 'Spotlight', html: `<div class="page"><div class="head"><h1>Spotlight</h1><p class="sub">${sub}</p><div class="acts"><button type="button" class="btn secondary" data-act="back" data-fid="sl:back">${icon('arrowL')}Zurück</button></div></div>${body}</div>` };
});

/* ---------- M-03/M-04 Vorschauseite ---------- */
/* The store hides missing titles from profiles that may not request; the decision sheet (section 3) wants the page
   reachable with a notice instead. Only the age rating and switched-off wishes make the page a 404. */
function vtPreviewVisible(p, t) {
  if (titleVisible(p, t)) return true;
  const cr = canRequest(p, t);
  /* rule F-9: a missing title exists for a profile only if it can request it, or if it has a wish of its own for it */
  return t.age <= effMax(p) && S.wishes.some((w) => w.titleKey === t.key && isMine(w, p));
}
const VT_STEPS = ['Übermittelt', 'Freigegeben', 'In Beschaffung', 'Verfügbar'];
const VT_REACHED = { queued: 0, pending: 1, processing: 3, partial: 3, available: 4 };
function vtWishCard(t, w, p) {
  const last = w.events.length - 1;
  const done = w.events.map((e, i) => `<li class="${i === last && OPEN_STATES.includes(w.state) ? 'now' : ['declined', 'failed'].includes(e.k) ? 'bad' : 'done'}">${esc(e.t)}<time>${fmtDate(e.at)}, ${fmtTime(e.at)}</time></li>`).join('');
  const todo = VT_STEPS.slice(VT_REACHED[w.state] ?? 4).map((s) => `<li class="vt-todo">${s}</li>`).join('');
  const joiner = w.owner !== p.id && w.joined.includes(p.id);
  let acts = `<a class="btn secondary" href="#/wishes" data-fid="pv:mywishes">${icon('wish')}Zu meinen Wünschen</a>`;
  if (joiner) acts += `<button type="button" class="btn ghost" data-act="vtWithdraw" data-id="${w.id}" data-fid="pv:withdraw">${icon('undo')}Nicht mehr mitwünschen</button>`;
  else if (['queued', 'pending'].includes(w.state) && S.settings.wishes.withdraw) acts += `<button type="button" class="btn ghost" data-act="vtWithdraw" data-id="${w.id}" data-fid="pv:withdraw">${icon('undo')}Zurückziehen</button>`;
  const what = w.seasons.length ? vtPlural(w.seasons) : kindLabel(t);
  return `<section class="vt-wish" aria-label="Dein Wunsch"><div class="vt-wh"><h2>Dein Wunsch</h2>${wsChip(w.state)}</div>
    <p class="sec">${esc(what)} · angefordert am ${fmtDate(w.created)}${joiner ? ' · du wünschst mit' : ''}</p>
    ${dlBlock(w)}
    ${w.state === 'queued' && w.detail ? `<p class="vt-detail">${icon('alert', 16)}<span>${esc(w.detail)}. Slouchcat versucht es weiter.</span></p>` : ''}
    <ol class="steps">${done}${todo}</ol><div class="acts">${acts}</div></section>`;
}
function vtStatusBlock(t, p) {
  const a = availability(p, t), cr = canRequest(p, t), lc = lastClosed(t.key, p);
  if (a.kind === 'wished' && a.mine) return vtWishCard(t, a.wish, p);
  if (!cr.ok) {
    const acts = cr.reason === 'unmapped' && isAdmin() ? `<a class="btn secondary sm" href="#/admin/wishes/mapping" data-fid="pv:map">${icon('link', 16)}Seerr-Benutzer öffnen</a>` : '';
    return notice('warn', 'Anfordern ist für dieses Profil nicht möglich', esc(cr.text), acts);
  }
  if (a.kind === 'wished') {
    const tail = S.settings.wishes.join !== 'join' ? 'Sobald der Titel da ist, findest du ihn in der Bibliothek.' : t.kind === 'movie' ? 'Mit «Benachrichtige mich» meldet sich Slouchcat auch bei dir, sobald er abspielbar ist.' : 'Staffeln, die noch frei sind, kannst du selbst anfordern.';
    return notice('', 'Bereits angefordert', `Ein anderes Profil hat ${t.kind === 'movie' ? 'diesen Film' : 'diese Serie'} schon angefordert (${esc(STATE_LABEL[a.wish.state])}). ${tail}`);
  }
  if (lc && lc.state !== 'available') {
    const why = lc.state === 'failed' ? FAIL_TEXT[lc.failReason] || '' : lc.state === 'declined' ? 'Seerr nennt keinen Grund.' : 'Du hast ihn zurückgezogen.';
    return notice(lc.state === 'withdrawn' ? '' : 'warn', `Dein letzter Wunsch: ${STATE_LABEL[lc.state]}`, `${fmtDate(lc.closed || lc.created)}. ${esc(why)} Du kannst den Titel erneut anfordern.`);
  }
  return notice('', 'Noch nicht in der Bibliothek', 'Fordere den Titel an, Slouchcat meldet sich, sobald er abspielbar ist.');
}
route('/titles/:media/:tmdbId', (pm) => {
  const t = TMDB[pm.media + '/' + pm.tmdbId];
  if (!t) return vtNotFound();
  if (isPresent(t.key)) return vtRedirect('#/items/' + t.key);
  const p = curProfile();
  if (!vtPreviewVisible(p, t)) return vtNotFound();
  const cr = canRequest(p, t);
  const info = `<div class="acts">${mainActions(t, { details: false, ctx: 'pv' })}</div>`;
  let body = `<div class="gutter vt-status">${vtStatusBlock(t, p)}</div>`;
  if (t.kind === 'series') {
    const note = SERIES_NOTE[t.key];
    body += `<section class="gutter vt-sec" aria-label="Staffeln"><h2 class="seclabel">Staffeln</h2>${note ? `<p class="vt-note">${icon('clock', 16)}<span>${esc(note)}</span></p>` : ''}<ul class="swall">${(t.seasons || []).map((s) => vtGhost(t, s, p, 'pv', cr.ok)).join('')}</ul></section>`;
  }
  body += vtCast(t, 'pv');
  return { type: 'viewer', nav: '', title: t.title, html: `<div class="det">${vtTop(t, 'pv', '', info)}<div class="body">${body}</div></div>` };
});

/* ---------- M-05/M-06 Detailseite ---------- */
function vtRound(act, ic, label, o = {}) {
  const pressed = o.pressed != null ? `aria-pressed="${!!o.pressed}"` : '';
  return `<button type="button" class="${cls('round', o.pressed && 'on', o.hi && 'hi')}" data-act="${act}" ${o.data || ''} ${pressed} data-fid="det:r:${o.fid}"><span class="c">${icon(ic)}</span><span>${label}</span></button>`;
}
function vtEpisodes(t, s) {
  const n = Math.min(s.episodes || 8, 30);
  const marks = S.marks.eps ?? {};
  const rows = Array.from({ length: n }, (_, i) => {
    const e = i + 1, k = `${t.key}:${s.n}:${e}`, on = !!marks[k];
    const code = `S${String(s.n).padStart(2, '0')}E${String(e).padStart(2, '0')}`;
    return `<li><div class="eprow"><span class="n">${e}</span><button type="button" class="t vt-ept" data-act="play" data-key="${t.key}" aria-label="Abspielen: ${code} Folge ${e}" data-fid="det:ept:${s.n}:${e}">Folge ${e}</button><span class="s">${code}</span><span class="d">${42 + ((e * 7) % 12)} min</span><button type="button" class="vt-epw ${on ? 'on' : ''}" data-act="vtEp" data-k="${k}" aria-pressed="${on}" aria-label="Gesehen: ${code}" data-fid="det:ep:${s.n}:${e}">${icon('check', 18)}</button></div></li>`;
  }).join('');
  const more = s.episodes > 30 ? `<p class="muted vt-note">Im Mockup sind die ersten 30 von ${s.episodes} Folgen gezeigt.</p>` : '';
  return `<ul class="eprows">${rows}</ul>${more}`;
}
function vtMissingLine(t, p, miss) {
  const free = miss.filter((n) => vtSeasonState(t, n, p).k === 'free');
  const parts = [];
  if (free.length) parts.push(`${vtPlural(free)} ${free.length === 1 ? 'fehlt' : 'fehlen'}.`);
  miss.forEach((n) => {
    const ss = vtSeasonState(t, n, p);
    if (ss.k === 'mine') parts.push(`Staffel ${n} ist angefordert: ${STATE_LABEL[ss.st]}.`);
    if (ss.k === 'other') parts.push(`Staffel ${n} ist bereits angefordert.`);
  });
  const link = miss.some((n) => vtSeasonState(t, n, p).k === 'mine') ? ` <a class="lnk" href="#/wishes" data-fid="det:mywishes">Meine Wünsche</a>` : '';
  return `<p class="vt-note">${icon('info', 16)}<span>${parts.join(' ')}${link}</span><span class="newtag">NEU</span></p>`;
}
route('/items/:key', (pm) => {
  const t = T[pm.key];
  if (!t) return vtNotFound();
  if (!isPresent(t.key)) return vtRedirect(titleHref(t));
  const p = curProfile();
  if (!titleVisible(p, t)) return vtNotFound();
  const series = t.kind === 'series';
  const cr = canRequest(p, t);
  const miss = series ? missingSeasons(t) : [];
  const canAsk = series && miss.length > 0 && cr.ok && S.settings.wishes.fromDetail && miss.some((n) => vtSeasonState(t, n, p).k === 'free');
  const wl = !!S.marks.watchlist[t.key], wd = !!S.marks.watched[t.key], pk = !!(S.marks.picks ?? {})[t.key];
  const have = libSeasons(t.key);
  const seasons = series ? (t.seasons || []) : [];
  const cur = have.includes(ui.season[t.key]) ? ui.season[t.key] : have[0];
  const curS = seasons.find((s) => s.n === cur);

  const playSub = series ? (curS ? `S${String(cur).padStart(2, '0')}E01 · Folge 1` : '') : t.runtime;
  let rounds = '';
  if (canAsk) rounds += vtRound('askWish', 'plus', 'Staffeln anfordern', { hi: true, fid: 'ask', data: `data-key="${t.key}"` });
  rounds += vtRound('trailer', 'play', 'Trailer', { fid: 'trailer', data: `data-key="${t.key}"` });
  rounds += vtRound('play', 'undo', 'Von vorne', { fid: 'restart', data: `data-key="${t.key}"` });
  rounds += vtRound('mark', 'bookmark', wl ? 'Gemerkt' : 'Merken', { pressed: wl, fid: 'wl', data: `data-kind="watchlist" data-key="${t.key}"` });
  rounds += vtRound('mark', 'check', 'Gesehen', { pressed: wd, fid: 'wd', data: `data-kind="watched" data-key="${t.key}"` });
  if (curAccount().owner) rounds += vtRound('vtPick', 'star', pk ? 'Empfohlen ✓' : 'Empfehlen', { pressed: pk, fid: 'pick', data: `data-key="${t.key}"` });
  if (isAdmin()) rounds += vtRound('vtDelete', 'trash', 'Löschen', { fid: 'del', data: `data-key="${t.key}"` });
  const info = `<div class="acts"><button type="button" class="btn primary lg playbtn" data-act="play" data-key="${t.key}" data-fid="det:play">${icon('play')}<span class="pt">Abspielen${playSub ? `<small>${esc(playSub)}</small>` : ''}</span></button></div><div class="rounds">${rounds}</div>`;
  const pmark = `<span class="vt-pm"><button type="button" class="vt-rnd ${wl ? 'on' : ''}" data-act="mark" data-kind="watchlist" data-key="${t.key}" aria-pressed="${wl}" aria-label="${esc(t.title)} ${wl ? 'von der Merkliste nehmen' : 'merken'}" data-fid="det:pm">${icon('bookmark', 18)}</button></span>`;

  let body = '';
  if (series) {
    const ghosts = (cr.ok) && S.settings.wishes.fromDetail; /* profiles without the right to request see only what is there, as today */
    const cards = seasons.map((s) => {
      if (have.includes(s.n)) {
        const on = s.n === cur;
        return `<li class="${cls('pc', !on && 'dim', on && 'vt-on')}"><button type="button" class="pl" data-act="vtSeason" data-key="${t.key}" data-n="${s.n}" aria-pressed="${on}" data-fid="det:s:${s.n}"><span class="frame">${img(s.poster, s.name)}</span><span class="cap"><b>${esc(s.name)}</b><span>${esc(vtSeasonLine(s))}</span></span></button></li>`;
      }
      return ghosts ? vtGhost(t, s, p, 'det', S.settings.wishes.fromDetail) : '';
    }).join('');
    const note = SERIES_NOTE[t.key];
    const hasGhost = ghosts && seasons.some((s) => !have.includes(s.n));
    body += `<section class="gutter vt-sec" aria-label="Staffeln"><h2 class="seclabel">Staffeln${hasGhost ? ' <span class="newtag">NEU</span>' : ''}</h2>${note ? `<p class="vt-note">${icon('clock', 16)}<span>${esc(note)}</span></p>` : ''}<ul class="swall">${cards}</ul>${ghosts && miss.length ? vtMissingLine(t, p, miss) : ''}</section>`;
    if (curS) body += `<section class="gutter vt-sec" aria-label="${esc(curS.name)}"><div class="vt-shead"><h2>${esc(curS.name)}</h2><p class="muted">${esc(vtSeasonLine(curS))}</p>${curS.overview ? `<p class="sec">${esc(curS.overview)}</p>` : ''}</div>${vtEpisodes(t, curS)}</section>`;
  }
  body += vtCast(t, 'det');
  return { type: 'viewer', nav: '', title: t.title, html: `<div class="det">${vtTop(t, 'det', pmark, info)}<div class="body">${body}</div></div>` };
});

/* ---------- actions ---------- */
ACT.vtSeason = (d) => { ui.season[d.key] = Number(d.n); render(); };
/* a click on one missing season opens the request dialog with exactly this season chosen */
ACT.vtAskSeason = (d) => {
  const t = T[d.key], n = Number(d.n);
  const miss = isPresent(t.key) ? missingSeasons(t) : airedSeasons(t).map((s) => s.n);
  const free = miss.filter((x) => !openWish(t.key, x));
  ui.draft.wish = { key: d.key, all: false, pick: { ...Object.fromEntries(free.map((x) => [x, false])), [n]: true } };
  openDialog('wish', { key: d.key });
};
ACT.vtEp = (d) => { S.marks.eps = S.marks.eps ?? {}; S.marks.eps[d.k] = !S.marks.eps[d.k]; commit(); };
ACT.vtPick = (d) => {
  S.marks.picks = S.marks.picks ?? {};
  S.marks.picks[d.key] = !S.marks.picks[d.key];
  toast(S.marks.picks[d.key] ? 'Zu den User-Empfehlungen hinzugefügt. Bleibt 30 Tage.' : 'Empfehlung zurückgenommen.', 'ok');
  commit();
};
DIALOGS.vtDelete = ({ key }) => {
  const t = T[key];
  return { title: `«${esc(t.title)}» löschen?`, body: `<p class="lead">Löschen bleibt, wie es ist: Du wählst «Nur aus Slouchcat entfernen» oder «Auch die Dateien löschen» und bestätigst mit dem Titel. Radarr und Sonarr erfahren davon nichts.</p>${notice('', 'Im Mockup', 'Das Mockup löscht nichts. Der Dialog ändert sich durch das Konzept nicht.')}`, actions: `<button type="button" class="btn ghost" data-act="close" data-fid="vtd:x" data-autofocus>Abbrechen</button><button type="button" class="btn secondary" data-act="vtDeleteOk" data-key="${key}" data-fid="vtd:ok">${icon('trash')}Weiter</button>` };
};
ACT.vtDelete = (d) => openDialog('vtDelete', { key: d.key });
ACT.vtDeleteOk = (d) => { closeDialog(); toast(`Löschen ist unverändert. «${esc(T[d.key].title)}» bleibt im Mockup in der Bibliothek.`, 'warn'); };
DIALOGS.vtWithdraw = ({ id }) => {
  const w = wishById(id), t = T[w.titleKey], p = curProfile();
  const joiner = w.owner !== p.id && w.joined.includes(p.id);
  const text = joiner ? `Slouchcat benachrichtigt dich nicht mehr, wenn «${esc(t.title)}» verfügbar ist. Der Wunsch selbst bleibt bestehen.` : `Der Wunsch für «${esc(t.title)}» wird in Seerr zurückgezogen. Du kannst den Titel später wieder anfordern.`;
  return { title: joiner ? 'Nicht mehr mitwünschen?' : 'Wunsch zurückziehen?', body: `<p class="lead">${text}</p>`, actions: `<button type="button" class="btn ghost" data-act="close" data-fid="vtw:x" data-autofocus>Abbrechen</button><button type="button" class="btn primary" data-act="vtWithdrawOk" data-id="${id}" data-fid="vtw:ok">${icon('undo')}${joiner ? 'Nicht mehr mitwünschen' : 'Zurückziehen'}</button>` };
};
ACT.vtWithdraw = (d) => openDialog('vtWithdraw', { id: d.id });
ACT.vtWithdrawOk = (d) => {
  const w = wishById(d.id), t = T[w.titleKey], p = curProfile();
  const joiner = w.owner !== p.id && w.joined.includes(p.id);
  closeDialog();
  withdrawWish(d.id);
  toast(joiner ? `Du wünschst «${esc(t.title)}» nicht mehr mit.` : `Dein Wunsch für «${esc(t.title)}» ist zurückgezogen.`, 'ok');
  commit();
};
