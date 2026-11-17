/* M-01 Startseite mit Spotlight, M-13 Dialog «Anfordern», shared viewer building blocks. */
'use strict';

function wsChip(state, label) { return `<span class="ws ${state}">${icon(STATE_ICON[state] || 'info', 14)}${esc(label || STATE_LABEL[state] || state)}</span>`; }
function missingChip() { return `<span class="ws missing">${icon('plus', 14)}Noch nicht in der Bibliothek</span>`; }
/* state text of a title for the active profile, used in captions */
function availText(a) {
  if (a.kind === 'missing') return 'Noch nicht da';
  if (a.kind === 'wished') return a.mine ? STATE_LABEL[a.wish.state] : 'Bereits angefordert';
  if (a.kind === 'partial') return `${a.missing.length} ${a.missing.length === 1 ? 'Staffel fehlt' : 'Staffeln fehlen'}`;
  return '';
}
function fulfilledRecently(p, t) { return S.wishes.some((w) => w.titleKey === t.key && w.state === 'available' && isMine(w, p) && Date.now() - (w.closed || 0) < 14 * D); }

/* poster card: present titles keep «Gesehen» and «Merken»; missing ones get the request button */
function posterCard(t, o = {}) {
  const p = curProfile(), a = availability(p, t);
  const present = a.kind === 'present' || a.kind === 'partial';
  /* o.libMark: on the pages under «Wünsche» a title of the library says so */
  const sub = o.caption != null ? o.caption : [t.year, present ? (a.kind === 'partial' && o.partial ? availText(a) : o.libMark ? 'In der Bibliothek' : '') : availText(a)].filter(Boolean).join(' · ');
  const tag = o.select ? `button type="button" data-act="heroSelect" data-key="${t.key}"` : `a href="${titleHref(t)}"`;
  const end = o.select ? 'button' : 'a';
  let over = '';
  if (present) {
    const wl = S.marks.watchlist[t.key], wd = S.marks.watched[t.key];
    over = `<span class="tr"><button type="button" class="rnd ${wd ? 'on' : ''}" data-act="mark" data-kind="watched" data-key="${t.key}" aria-pressed="${!!wd}" aria-label="Gesehen: ${esc(t.title)}" data-fid="wd:${o.ctx || ''}:${t.key}">${icon('check', 16)}</button></span><span class="br"><button type="button" class="rnd ${wl ? 'on' : ''}" data-act="mark" data-kind="watchlist" data-key="${t.key}" aria-pressed="${!!wl}" aria-label="${esc(t.title)} ${wl ? 'von der Merkliste nehmen' : 'merken'}" data-fid="wl:${o.ctx || ''}:${t.key}">${icon('bookmark', 16)}</button></span>`;
  } else if (a.kind === 'wished' && !a.mine && S.settings.wishes.join !== 'join') {
    over = `<span class="bl">${wsChip('queued', 'Bereits angefordert')}</span>`;
  } else if (a.kind === 'missing' || (a.kind === 'wished' && !a.mine)) {
    over = `<span class="br"><button type="button" class="rnd plus" data-act="askWish" data-key="${t.key}" aria-label="${esc(t.title)} anfordern" data-fid="rq:${o.ctx || ''}:${t.key}">${icon(a.kind === 'wished' ? 'bell' : 'plus', 16)}</button></span>`;
  } else {
    over = `<span class="bl">${wsChip(a.wish.state)}</span>`;
  }
  return `<li class="${cls('pc', o.dim && 'dim', !present && 'missing', present && o.libMark && 'inlib')}"><${tag} class="pl" data-fid="pc:${o.ctx || ''}:${t.key}" ${o.current ? 'aria-current="true"' : ''}><span class="frame">${img(t.poster, t.title)}</span><span class="cap"><b>${esc(t.title)}</b><span>${esc(sub)}</span></span></${end}>${over}</li>`;
}
function rowHtml(key, title, href, items, o = {}) {
  if (!items.length) return '';
  return `<section class="row" aria-label="${esc(title)}"><div class="rowhead"><h2>${href ? `<a href="${href}" data-fid="rh:${key}">${esc(title)}${icon('chevR', 16)}</a>` : esc(title)}</h2><div class="paging"><button type="button" class="btn ghost icon" data-act="scrollRow" data-row="${key}" data-dir="prev" aria-label="Zurückblättern" data-fid="rp:${key}">${icon('chevL')}</button><button type="button" class="btn ghost icon" data-act="scrollRow" data-row="${key}" data-dir="next" aria-label="Weiterblättern" data-fid="rn:${key}">${icon('chevR')}</button></div></div><div class="scroller" data-keep="${key}"><ul>${items.map((t) => posterCard(t, { ctx: key, ...o })).join('')}</ul></div></section>`;
}

/* buttons of the stage and of the preview page for one title and the active profile */
function mainActions(t, o = {}) {
  const p = curProfile(), a = availability(p, t), lg = o.lg === false ? '' : 'lg', ctx = o.ctx || 'st';
  const details = o.details === false ? '' : `<a class="btn secondary ${lg}" href="${titleHref(t)}" data-fid="${ctx}:details">${icon('info')}Details</a>`;
  const trailer = `<button type="button" class="btn secondary ${lg}" data-act="trailer" data-key="${t.key}" data-fid="${ctx}:trailer">${icon('play')}Trailer</button>`;
  if (a.kind === 'present' || a.kind === 'partial') return `<button type="button" class="btn primary ${lg}" data-act="play" data-key="${t.key}" data-fid="${ctx}:play">${icon('play')}Abspielen</button>${details}${trailer}`;
  if (a.kind === 'wished' && a.mine) return `<a class="btn state ${lg}" href="#/wishes" data-fid="${ctx}:state">${icon(STATE_ICON[a.wish.state])}${STATE_LABEL[a.wish.state]}</a>${details}${trailer}`;
  const cr = canRequest(p, t);
  const allowedHere = o.source === 'search' ? S.settings.wishes.fromSearch : true;
  if (!cr.ok || !allowedHere) return `${details}${trailer}`;
  if (a.kind === 'wished' && S.settings.wishes.join !== 'join') return `${details}${trailer}`;
  if (a.kind === 'wished') return `<button type="button" class="btn primary ${lg}" data-act="askWish" data-key="${t.key}" data-fid="${ctx}:join">${icon('bell')}Benachrichtige mich</button>${details}${trailer}`;
  return `<button type="button" class="btn primary ${lg}" data-act="askWish" data-key="${t.key}" data-fid="${ctx}:request">${icon('plus')}${t.kind === 'movie' ? 'Film anfordern' : 'Serie anfordern'}</button>${details}${trailer}`;
}
function metaLine(t, o = {}) {
  const p = curProfile(), a = availability(p, t);
  const parts = [t.year, t.kind === 'movie' ? t.runtime : `${(t.seasons || []).length} Staffeln`].filter(Boolean).map((x, i) => `<span class="${i ? 'sep' : ''}">${esc(x)}</span>`);
  let chip = '';
  if (o.stage) chip = ''; /* the stage stays as it is today: no wish chips */
  else if (a.kind === 'missing') chip = missingChip();
  else if (a.kind === 'wished') chip = a.mine ? wsChip(a.wish.state) : wsChip('queued', 'Bereits angefordert');
  else if (a.kind === 'partial' && canRequest(p, t).ok && S.settings.wishes.fromDetail) chip = `<span class="ws missing">${libSeasons(t.key).length} von ${airedSeasons(t).length} Staffeln vorhanden</span>`;
  else if (fulfilledRecently(p, t)) chip = wsChip('available', 'Dein Wunsch ist da');
  return `<div class="meta">${parts.join('')}${o.age ? `<span class="sep">${ageLabel(t.age)}</span>` : ''}${chip}</div>`;
}

function heroHtml(list) {
  if (!list.length) return '';
  const i = ((ui.hero.i % list.length) + list.length) % list.length;
  const t = list[i];
  const note = (S.settings.spotlight.overrides[t.key] || {}).note;
  return `<section class="hero" aria-label="Spotlight" aria-roledescription="Karussell">
    <div class="rowhead"><h2><a href="#/lists/spotlight" data-fid="rh:spot">Spotlight${icon('chevR', 16)}</a></h2></div>
    <div class="stagebox">
      <div class="art" data-hero-art="${t.key}">${img(t.backdrop, t.title, 'class="fade"')}</div><div class="scr"></div>
      <div class="cnt"><div class="slide" role="group" aria-roledescription="Folie" aria-label="${i + 1} von ${list.length}"><h3><a href="${titleHref(t)}" data-fid="st:title">${esc(t.title)}</a></h3>${metaLine(t, { stage: true })}${note ? `<p class="note">${esc(note)}</p>` : ''}</div>
      <div class="acts">${mainActions(t)}</div></div>
      <div class="corner"><span class="pos">${i + 1} von ${list.length}</span><button type="button" class="btn secondary icon" data-act="heroPause" aria-pressed="${ui.hero.paused}" aria-label="${ui.hero.paused ? 'Weiter' : 'Pause'}" data-fid="st:pause">${icon(ui.hero.paused ? 'play' : 'pause')}</button></div>
      <span class="arrow prev"><button type="button" class="btn secondary icon" data-act="heroStep" data-dir="-1" aria-label="Zurückblättern" data-fid="st:prev">${icon('chevL')}</button></span>
      <span class="arrow next"><button type="button" class="btn secondary icon" data-act="heroStep" data-dir="1" aria-label="Weiterblättern" data-fid="st:next">${icon('chevR')}</button></span>
    </div>
    <div class="scroller" data-keep="spot"><ul aria-label="Spotlight-Titel">${list.map((x, j) => posterCard(x, { ctx: 'spot', select: true, dim: j !== i, current: j === i })).join('')}</ul></div>
  </section>`;
}
const HOME_TITLES = { continue: 'Weiterschauen', recent: 'Neu hinzugefügt', released: 'Kürzlich veröffentlicht', picks: 'User-Empfehlungen', watchlist: 'Merkliste', wishes: 'Erfüllte Wünsche', spotlight: 'Spotlight', collections: 'Sammlungen', genres: 'Genres', libraries: 'Meine Medien' };
function homeRowItems(key, p) {
  const vis = (keys) => keys.map((k) => T[k]).filter((t) => isPresent(t.key) && t.age <= effMax(p));
  if (key === 'continue') return vis(['mobland', 'tenzing', 'dexter-resurrection', 'backrooms']);
  if (key === 'recent') return vis(['godzilla-minus-one', 'resident-evil', 'toy-story-5', 'black-doves', 'odyssey', 'verity', 'masterplan', 'obsession', 'supergirl', 'michael', 'dog-stars', 'tulsa-king']);
  if (key === 'released') return vis(['spider-man-bnd', 'project-hail-mary', 'end-of-oak-street', 'heart-of-the-beast', 'supergirl', 'michael', 'rings-of-power']);
  if (key === 'picks') return vis(['tenzing', 'project-hail-mary', 'dune-2']);
  if (key === 'watchlist') return vis(Object.keys(S.marks.watchlist).filter((k) => S.marks.watchlist[k]));
  /* the start page shows only what the library holds: wishes that came true in the last 14 days; open ones live under «Wünsche» */
  if (key === 'wishes') { const seen = new Set(); return myWishes(p).filter((w) => w.state === 'available' && Date.now() - (w.closed || 0) < 14 * D).map((w) => T[w.titleKey]).filter((t) => isPresent(t.key) && t.age <= effMax(p) && !seen.has(t.key) && seen.add(t.key)); }
  return [];
}
route('/', () => {
  const p = curProfile();
  const spot = spotlightFor(p);
  let html = '<h1 class="vh">Startseite</h1>';
  S.settings.homeRows.filter((r) => !r.hidden).forEach((r) => {
    if (r.key === 'spotlight') html += heroHtml(spot);
    else if (r.key === 'wishes') html += rowHtml('wishes', 'Erfüllte Wünsche', '#/wishes', homeRowItems('wishes', p));
    else html += rowHtml(r.key, HOME_TITLES[r.key], r.key === 'watchlist' ? '#/unchanged/merkliste' : '', homeRowItems(r.key, p));
  });
  return { type: 'viewer', nav: 'start', title: 'Startseite', html: `<div class="home">${html}</div>`, after: heroAfter };
});
function heroAfter(root) {
  const cur = root.querySelector('.hero [aria-current="true"]');
  const sc = root.querySelector('[data-keep="spot"]');
  if (cur && sc && ui.hero.follow) { ui.hero.follow = false; const li = cur.closest('li'); sc.scrollTo({ left: Math.max(0, li.offsetLeft - sc.clientWidth / 2 + li.clientWidth / 2), behavior: 'smooth' }); }
}
ACT.heroStep = (d) => { ui.hero.i += Number(d.dir); ui.hero.follow = true; render(); };
ACT.heroPause = () => { ui.hero.paused = !ui.hero.paused; render(); };
ACT.heroSelect = (d) => { const list = spotlightFor(curProfile()); const i = list.findIndex((t) => t.key === d.key); if (i >= 0) { ui.hero.i = i; ui.hero.paused = true; render(); } };
ACT.mark = (d) => { S.marks[d.kind][d.key] = !S.marks[d.kind][d.key]; commit(); };
/* rotation like the app: stops for keyboard focus inside the stage, an open dialog, a hidden tab */
setInterval(() => {
  if (parseHash().path !== '/' || ui.hero.paused || ui.dialog || ui.drawer || document.hidden || (window.MOCK && MOCK.open)) return;
  const a = document.activeElement;
  if (a && a.closest && a.closest('.hero') && a.matches(':focus-visible')) return;
  if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
  ui.hero.i++;
  render();
}, 6000);

/* ---------- M-13 dialog «Anfordern» ---------- */
ACT.askWish = (d) => {
  const t = T[d.key];
  const miss = t.kind === 'series' ? (isPresent(t.key) ? missingSeasons(t) : airedSeasons(t).map((s) => s.n)) : [];
  const p = curProfile(), canJoin = S.settings.wishes.join === 'join';
  const free = miss.filter((n) => { const w = openWish(t.key, n); return !w || (canJoin && !isMine(w, p)); });
  /* the limit counts every season: preselect only as many new seasons as the limit still allows */
  const lq = t.kind === 'series' ? quotaOfProfile(p) : null, left = lq && lq.tv.limit ? lq.tv.remaining : Infinity;
  let fresh = 0;
  const pick = Object.fromEntries(free.map((n) => { const on = !!openWish(t.key, n) || fresh < left; if (on && !openWish(t.key, n)) fresh++; return [n, on]; }));
  ui.draft.wish = { key: d.key, all: true, pick, capped: left > 0 && free.filter((n) => !openWish(t.key, n)).length > left ? left : 0 };
  openDialog('wish', { key: d.key });
};
function wishChannels(p) { const acc = account(p.accountId); return ['App', ...Object.entries(acc.channels).filter(([, c]) => c.enabled && c.events.includes('wish_available')).map(([k]) => CHANNEL_LABEL[k])]; }
DIALOGS.wish = ({ key }) => {
  const t = T[key], p = curProfile(), a = availability(p, t), cr = canRequest(p, t);
  const head = `<div class="rq"><div class="po">${img(t.poster, t.title)}</div><div class="tx"><b>${esc(t.title)}</b><span class="sec">${[t.year, kindLabel(t), ageLabel(t.age)].join(' · ')}</span><span class="muted" style="font-size:14px;display:-webkit-box;-webkit-line-clamp:3;-webkit-box-orient:vertical;overflow:hidden">${esc(t.overview)}</span></div></div>`;
  const close = `<button type="button" class="btn secondary" data-act="close" data-fid="wd:x">Schliessen</button>`;
  if (!cr.ok) return { title: 'Anfordern nicht möglich', body: head + notice('warn', 'Dieses Profil kann den Titel nicht anfordern', esc(cr.text)), actions: close };
  const su = seerrUserFor(p), u = seerrUser(su.id), ss = seerrState();
  const facts = [];
  const film = t.kind === 'movie';
  if (a.kind === 'wished' && !a.mine && film) {
    return { title: 'Schon angefordert', body: head + `<p class="lead">Dieser Film ist bereits angefordert (${STATE_LABEL[a.wish.state]}). Du kannst dich dazuschalten: Dann benachrichtigt dich Slouchcat ebenfalls, sobald er abspielbar ist.</p><div class="facts"><div class="f">${icon('bell', 16)}<span>Benachrichtigung über: ${wishChannels(p).join(', ')}</span></div></div>`, actions: `<button type="button" class="btn ghost" data-act="close" data-fid="wd:x">Abbrechen</button><button type="button" class="btn primary" data-act="confirmWish" data-fid="wd:ok" data-autofocus>${icon('bell')}Benachrichtige mich</button>` };
  }
  facts.push(`<div class="f">${icon('user', 16)}<span>Wird als <b>${esc(u.name)}</b> an Jellyseerr übermittelt${su.via === 'profile' ? ' (eigener Benutzer des Profils)' : su.via === 'fallback' ? ' (Sammelbenutzer)' : ' (Benutzer des Kontos)'}.</span></div>`);
  facts.push(`<div class="f">${icon(u.autoApprove ? 'truck' : 'hourglass', 16)}<span>${u.autoApprove ? 'Wird sofort beschafft, eine Freigabe braucht es nicht.' : 'Braucht eine Freigabe. Den Stand siehst du unter «Meine Wünsche».'}</span></div>`);
  const lq = quotaFor(u), lk = film ? 'movie' : 'tv', lx = lq[lk];
  facts.push(`<div class="f">${icon('chart', 16)}<span>${lx.limit ? `Limit: <b>${quotaText(lx, lk)}</b> (${lx.limit} in ${lx.days} Tagen${lq.source === 'account' ? ', für alle Profile dieses Kontos gemeinsam' : ''}).` : lq.source === 'admin' ? 'Kein Limit: Administratoren fordern unbegrenzt an.' : 'Kein Limit.'}</span></div>`);
  facts.push(`<div class="f">${icon('bell', 16)}<span>Slouchcat meldet sich, sobald der Titel hier abspielbar ist: ${wishChannels(p).join(', ')}.</span></div>`);
  let body = head + `<div class="facts">${facts.join('')}</div>`;
  if (ss.k !== 'ok') body += notice('warn', 'Jellyseerr ist gerade nicht erreichbar', 'Dein Wunsch wird vorgemerkt. Slouchcat übermittelt ihn, sobald Jellyseerr wieder antwortet.');
  let n = 1, label = 'Anfordern', blocked = false;
  if (film) { const lc = limitCheck(p, t, 1); if (!lc.ok) { blocked = true; body += notice('bad', 'Limit erreicht', esc(lc.text) + ' Wünsche, die schon jemand gestellt hat, kannst du weiter mitverfolgen: Sie zählen nicht.'); } }
  if (!film) {
    const have = libSeasons(t.key), d = ui.draft.wish;
    const rows = (t.seasons || []).map((s) => {
      const up = upcoming(t.key).includes(s.n), has = have.includes(s.n), w = openWish(t.key, s.n);
      const joinable = w && !isMine(w, p) && S.settings.wishes.join === 'join';
      const st = has ? '<span class="chip ok">Vorhanden</span>' : up ? '<span class="chip info">Angekündigt</span>' : joinable ? wsChip('queued', 'Bereits angefordert') : w ? wsChip(w.state) : '';
      const dis = has || up || (!!w && !joinable);
      return `<label class="check"><input type="checkbox" data-bind="ui.draft.wish.pick.${s.n}" data-fid="wd:s${s.n}" ${dis ? 'disabled' : ''} ${!dis && d.pick[s.n] ? 'checked' : ''}><span>${esc(s.name)}<small>${[s.year, s.episodes ? s.episodes + ' Folgen' : ''].filter(Boolean).join(' · ')}</small></span><span class="st">${st}</span></label>`;
    }).join('');
    const free = Object.keys(d.pick).map(Number);
    n = free.filter((x) => d.pick[x]).length;
    body += `<div class="fld"><span class="lbl">Staffeln</span><div class="seasonpick"><label class="check all"><input type="checkbox" data-fid="wd:all" data-act="wishAll" ${n === free.length && n > 0 ? 'checked' : ''} ${free.length ? '' : 'disabled'}><span>Alle fehlenden Staffeln${free.length ? ` (${free.length})` : ''}</span></label>${rows}</div><span class="hint">Einzelne Folgen lassen sich nicht anfordern. Qualität und Zielordner bestimmt Jellyseerr. Bei schon angeforderten Staffeln wirst du mitbenachrichtigt.</span></div>`;
    const fresh = free.filter((x) => d.pick[x] && !openWish(t.key, x)).length;
    label = n === 0 ? 'Staffeln wählen' : fresh === 0 ? 'Benachrichtige mich' : n === 1 ? '1 Staffel anfordern' : `${n} Staffeln anfordern`;
    if (d.capped && fresh <= d.capped) body += notice('', 'Das Limit reicht nicht für alle Staffeln', `Noch ${d.capped} von ${lx.limit} Staffeln frei. Vorgewählt ${d.capped === 1 ? 'ist die erste fehlende Staffel' : 'sind die ersten ' + d.capped + ' fehlenden Staffeln'}; du kannst andere wählen.`);
    if (fresh > 0) { const lc = limitCheck(p, t, fresh); if (!lc.ok) { blocked = true; body += notice('bad', lc.q.tv.remaining ? 'Zu viele Staffeln für das Limit' : 'Limit erreicht', esc(lc.text) + ' Jede Staffel zählt einzeln.'); } }
    if (!free.length) body += notice('', 'Nichts mehr anzufordern', 'Alle ausgestrahlten Staffeln sind vorhanden oder schon angefordert.');
  }
  return { title: film ? 'Film anfordern' : 'Serie anfordern', body, actions: `<button type="button" class="btn ghost" data-act="close" data-fid="wd:x">Abbrechen</button><button type="button" class="btn primary" data-act="confirmWish" data-fid="wd:ok" ${n === 0 || blocked ? 'disabled' : 'data-autofocus'}>${icon('plus')}${label}</button>` };
};
ACT.wishAll = (d, el) => { const dr = ui.draft.wish; Object.keys(dr.pick).forEach((k) => (dr.pick[k] = el.checked)); renderOverlays(); };
ACT.confirmWish = () => {
  const d = ui.draft.wish, t = T[d.key];
  const seasons = t.kind === 'series' ? Object.keys(d.pick).filter((k) => d.pick[k]).map(Number) : [];
  closeDialog();
  requestWish(d.key, seasons);
  render();
};

/* playback and trailers are unchanged; the mockup only confirms the click */
DIALOGS.play = ({ key }) => { const t = T[key]; return { title: esc(t.title), wide: true, body: `<div style="position:relative;aspect-ratio:16/9;border-radius:12px;overflow:hidden;background:#000">${img(t.backdrop, t.title, 'style="width:100%;height:100%;object-fit:cover;opacity:.55"')}<div style="position:absolute;inset:0;display:grid;place-items:center;text-align:center;padding:16px"><div><div style="display:inline-grid;place-items:center;width:72px;height:72px;border-radius:50%;background:var(--tx);color:var(--bg)">${icon('play', 32)}</div><p style="margin-top:12px;font-weight:600">Hier startet die Wiedergabe.</p><p class="sec" style="font-size:14px">Der Player ändert sich durch das Konzept nicht und ist nicht Teil des Mockups.</p></div></div></div>`, actions: `<button type="button" class="btn secondary" data-act="close" data-fid="pl:x" data-autofocus>Schliessen</button>` }; };
ACT.play = (d) => openDialog('play', { key: d.key });
DIALOGS.trailer = ({ key }) => { const t = T[key]; return { title: `Trailer: ${esc(t.title)}`, wide: true, body: `<div style="position:relative;aspect-ratio:16/9;border-radius:12px;overflow:hidden;background:#000">${img(t.backdrop, t.title, 'style="width:100%;height:100%;object-fit:cover;opacity:.55"')}<div style="position:absolute;inset:0;display:grid;place-items:center;text-align:center;padding:16px"><div><div style="display:inline-grid;place-items:center;width:72px;height:72px;border-radius:50%;background:var(--tx);color:var(--bg)">${icon('play', 32)}</div><p style="margin-top:12px;font-weight:600">Trailer von YouTube</p><p class="sec" style="font-size:14px">Für fehlende Titel kommt die Trailer-Kennung aus TMDB. Das Trailer-Fenster selbst bleibt, wie es ist.</p></div></div></div>`, actions: `<button type="button" class="btn secondary" data-act="close" data-fid="tr:x" data-autofocus>Schliessen</button>` }; };
ACT.trailer = (d) => openDialog('trailer', { key: d.key });
