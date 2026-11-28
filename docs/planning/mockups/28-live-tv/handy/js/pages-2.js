/* Konzept 28 «Live-TV», phone and tablet app: LS-07 live player (portrait: picture on top, info and channel
   list below; landscape: full screen with controls, swipe up/down or list to switch channel, timeshift bar,
   record button, sound/subtitles/quality sheet, all error states), LS-08 my recordings, LS-09 recording
   detail, LS-10 recording player. 14-oberflaeche.md 3.7 to 3.10, 4.5 to 4.12. Own globals start with hp / HP. */
'use strict';

/* ============================================================ LS-07 Live-Player */
const hpPlInit = () => ({ key: '', tuneAt: Date.now(), ctl: true, ctlUntil: Date.now() + 4500, infoUntil: Date.now() + 5000, info: false, force: null, stall: 0, listOpen: false, list: null, audio: 0, subs: -1, q: 'auto', reload: 0, reloadTxt: '', swiped: 0, fb: false, dragging: false, pend: null });
const hpPlOn = () => !!$('.hp-pl[data-live]');
const hpList = (pl) => (HP.p && HP.p.list) || (pl && pl.list) || SIM.channels().map((c) => c.id);
const hpListName = (ids) => { const fav = SIM.favorites(); const all = SIM.channels().map((c) => c.id); if (ids.length === fav.length && ids.every((x, i) => x === fav[i])) return 'Meine Sender'; if (ids.length === all.length) return 'Alle Sender'; for (const g of LIVE.groups) { const l = SIM.channels({ group: g.id }).map((c) => c.id); if (l.length === ids.length && l.every((x, i) => x === ids[i])) return 'Gruppe ' + g.name; } return `Auswahl (${ids.length} Sender)`; };

/* phases and faults come from the core (LIVE_UI.playerPhase, the same thresholds in all clients; mockup states in
   «Zustände» → Player): start (black, banner) · loading (from 0.5 s) · slow («Dauert länger») · play · error */
const HP_ERRMAP = { unreachable: 'unreach', notPlayable: 'notplay' };
function hpPhase(c) {
  const P = HP.p; const el = Date.now() - P.tuneAt;
  if (Date.now() < P.reload) return 'loading';
  const ph = LU.playerPhase(el, c, { hot: LIVE.inputs.some((i) => i.ch === c.id && i.state === 'running') });
  P.coreErr = ph.err || null; P.stall = ph.stall ? Date.now() + 60000 : 0;
  if (ph.fallback && !P.fb) { P.fb = true; setTimeout(() => toast(LU.TX.fallback || 'Auf eine Ersatzquelle umgeschaltet.'), 0); } else if (!ph.fallback && ph.k === 'ok') P.fb = false;
  if (ph.k === 'error') return 'error';
  if (ph.k === 'ok') return 'play';
  return ph.spin ? ph.k : 'start';
}
const hpErrKind = (c) => { const e = HP.p.coreErr || (SIM.chState(c) === 'gone' ? 'gone' : 'unreach'); return HP_ERRMAP[e] || e; };
const HP_ERR = {
  unreach: (c) => ({ t: `${c.name} ist gerade nicht erreichbar. Slouchcat versucht es weiter.`, w: true, b: [['hpRetryPl', 'Erneut versuchen'], ['hpOther', 'Anderer Sender']] }),
  gone: () => ({ t: 'Diesen Sender gibt es nicht mehr.', b: [['hpOther', 'Anderer Sender']] }),
  limit: () => ({ t: 'Auf deinem Konto laufen schon 3 Wiedergaben gleichzeitig.', b: [['back', 'Zurück']] }),
  time: () => ({ t: 'Deine Sehzeit für heute ist aufgebraucht.', b: [['back', 'Zurück']] }),
  revoked: () => ({ t: 'Live-TV ist für dieses Profil nicht mehr freigegeben.', b: [['hpHome', 'Zur Startseite']] }),
  notplay: () => ({ t: 'Dieser Sender lässt sich auf diesem Gerät nicht abspielen.', b: [['hpOther', 'Anderer Sender']] }),
  busy: () => ({ t: 'Der Server ist gerade ausgelastet. Versuche es gleich nochmals.', b: [['hpRetryPl', 'Erneut versuchen']] }),
};
const hpRecElapsed = (r) => { const m = Math.max(0, SIM.now() - r.start); return `${Math.floor(m / 60)}:${String(Math.floor(m % 60)).padStart(2, '0')}`; };

function hpPlRecBtn(c, nn) {
  if (!SIM.rights().recordings) return '';
  const run = SIM.runningOn(c.id); const sched = nn.now ? SIM.recState(nn.now.id) : null;
  if (run) return `<button type="button" class="hp-recb run" data-act="hpPlRec" data-fid="pl-rec">${icon('record', 18)}<span>Nimmt auf · <span data-hp-el="${run.id}">${hpRecElapsed(run)}</span></span></button>`;
  if (sched) return `<button type="button" class="hp-recb sched" data-act="hpPlRec" data-fid="pl-rec">${icon('recordHollow', 18)}<span>Geplant</span></button>`;
  return `<button type="button" class="hp-recb" data-act="hpPlRec" data-fid="pl-rec">${icon('recordHollow', 18)}<span>Aufnehmen</span></button>`;
}
ACT.hpPlRec = () => {
  const pl = SIM.player(); if (!pl) return; const c = SIM.channel(pl.ch); const nn = SIM.nowNext(c.id);
  if (SIM.runningOn(c.id)) return hpRunningMenu(c.id);
  if (nn.now && SIM.recState(nn.now.id)) return LiveDlg.planRecording(nn.now.id);
  LiveDlg.instantRecording(c.id);
};

function hpSeekHtml(c, pl, nn) {
  const now = SIM.now(); const max = pl.max; const start = now - max; const pct = max > 0.01 ? Math.max(0, Math.min(100, (1 - pl.offset / max) * 100)) : 100;
  const stub = nn.now && nn.now.start < pl.joinedAt - 0.01 ? `<span class="hp-stub lv-gap" title="${esc(`Vor ${fmt.time(pl.joinedAt)} nicht verfügbar`)}" aria-hidden="true"></span>` : '';
  const marks = SIM.programsOf(c.id, start, now).filter((p) => p.start > start + 0.5 && p.start < now).map((p) => `<i class="hp-mark" style="left:${((p.start - start) / max * 100).toFixed(1)}%" title="ab ${fmt.time(p.start)} ${esc(p.title)}"></i>`).join('');
  const capped = pl.joinedAt < start - 0.01;
  const pill = pl.paused ? `<button type="button" class="hp-pill shift" data-act="hpToLive" data-fid="pl-live">Pausiert · ${fmt.offset(pl.offset)} · ${esc(LU.TX.backToLive)}</button>`
    : pl.offset > 0.05 ? `<button type="button" class="hp-pill shift" data-act="hpToLive" data-fid="pl-live">${fmt.offset(pl.offset)} · ${esc(LU.TX.backToLive)}</button>`
      : `<span class="hp-pill live"><i aria-hidden="true"></i>${esc(LU.TX.live)}</span>`;
  return `<div class="hp-bot"><div class="hp-seekrow">${stub}<div class="hp-trk" data-hp-trk role="slider" tabindex="0" aria-label="Zeitversatz, bis zu ${fmt.dur(max)} zurück" aria-valuemin="0" aria-valuemax="${Math.round(max * 60)}" aria-valuenow="${Math.round((max - pl.offset) * 60)}" aria-valuetext="${pl.offset > 0.05 ? fmt.offset(pl.offset) + ' zeitversetzt' : 'Live'}" data-fid="pl-seek"><i class="hp-fill" style="width:${pct.toFixed(1)}%"></i>${marks}<i class="hp-knob" style="left:${pct.toFixed(1)}%"></i></div></div>
    <div class="hp-lbls"><span>${stub ? `Vor ${fmt.time(pl.joinedAt)} nicht verfügbar · ` : ''}${capped ? `ab ${fmt.time(start)} (höchstens ${fmt.dur(LIVE.timeshiftMaxMin)})` : `seit ${fmt.time(pl.joinedAt)}`}</span><span>${esc(LU.TX.live)} ${fmt.time(now)}</span></div>
    <div class="hp-brow">${pill}<span class="hp-pos">Position <b data-hp-pos>${fmt.time(pl.position)}</b></span></div></div>`;
}

function hpChRow(c, cur, k) {
  const nn = SIM.nowNext(c.id); const st = SIM.chState(c); const rec = SIM.rights().recordings && SIM.runningOn(c.id);
  const mid = st === 'disturbed' ? `<small class="hp-warn">${icon('alert', 14)}${esc(LU.TX.unavailable)}</small>` : nn.now ? `<small>${esc(nn.now.title)} · ${fmt.range(nn.now.start, nn.now.end)}</small>${LU.progress(nn.now.start, nn.now.end)}` : `<small>${esc(LU.TX.epgNone)}</small>`;
  return `<li><button type="button" class="hp-lr${cur ? ' cur' : ''}" data-act="hpPick" data-ch="${c.id}" ${cur ? 'aria-current="true"' : ''} aria-label="${c.no} ${esc(c.name)}${nn.now ? ', ' + esc(nn.now.title) : ''}${cur ? ', läuft gerade' : ''}" data-fid="${k}-${c.id}"><span class="lh-no">${c.no}</span>${LU.logo(c)}<span class="hp-lr-t"><b>${esc(c.name)}</b>${mid}</span>${rec ? LU.badge('recording') : ''}${cur ? `<span class="hp-lr-now">${icon('play', 14)}</span>` : ''}</button></li>`;
}

route('/live/watch/:id', (pm) => {
  const id = pm.id; const c = SIM.channel(id); const r = SIM.rights(); const m = SIM.mode();
  if (!c) return hpNotFound('Diesen Sender gibt es nicht mehr.', '#/live/channels', 'Zu den Sendern');
  if (!HP.p) HP.p = hpPlInit(); const P = HP.p;
  let pl = SIM.player();
  if (!pl || pl.ch !== id) { setTimeout(() => { const q = SIM.player(); if (!q || q.ch !== id) SIM.tune(id, {}); }, 0); pl = { ch: id, joinedAt: SIM.now(), offset: 0, paused: false, list: SIM.channels().map((x) => x.id), prev: null, max: 0, position: SIM.now(), live: true, pending: true }; }
  const key = pl.pending ? 'pending' : `${pl.ch}@${pl.joinedAt}`;
  if (P.key !== key && !pl.pending) { Object.assign(P, { key, tuneAt: Date.now(), ctl: true, ctlUntil: Date.now() + 4500, infoUntil: Date.now() + 5000, force: null, stall: 0, reload: 0 }); }
  const nn = SIM.nowNext(id); const list = hpList(pl).map(SIM.channel).filter(Boolean); const phase = pl.pending ? 'start' : m === 'loading' ? 'loading' : hpPhase(c); const ts = r.timeshift;
  const ek = hpErrKind(c); const eo = (HP_ERR[ek] || HP_ERR.unreach)(c);
  const sub = (ss) => ({ unreach: 'Nächster Versuch in <b data-hp-retry>20</b> s' }[ss] || '');
  const eb = (eo.b || []).map(([a, l], i) => `<button type="button" class="btn ${i ? 'ghost' : 'secondary'} sm" data-act="${a}" data-fid="pl-eb-${a}">${esc(l)}</button>`).join('');
  const errHtml = `<div class="hp-ov hp-err" role="alert">${LU.logo(c)}<b>${esc(c.name)}</b><p>${esc(eo.t)}</p>${eo.w ? `<small>${sub('unreach')}</small>` : ''}<div class="hp-ebtn">${eb}</div></div>`;
  const loadTxt = Date.now() < P.reload ? P.reloadTxt : phase === 'slow' ? 'Dauert länger als üblich …' : `${c.name} wird geladen`;
  const mid = `<div class="hp-mid"><button type="button" class="ib hp-bigb" data-act="hpPrev" aria-label="Vorheriger Sender" data-fid="pl-prev">${icon('prev', 28)}</button>
    ${ts ? `<button type="button" class="ib hp-bigb" data-act="hpSeekRel" data-s="-10" aria-label="10 Sekunden zurück" data-fid="pl-rew">${icon('rew', 28)}</button><button type="button" class="ib hp-bigb hp-pp" data-act="hpPlPP" aria-label="${pl.paused ? 'Weiter' : 'Anhalten'}" data-fid="pl-pp">${icon(pl.paused ? 'play' : 'pause', 32)}</button><button type="button" class="ib hp-bigb" data-act="hpSeekRel" data-s="10" aria-label="10 Sekunden vor" data-fid="pl-fwd">${icon('fwd', 28)}</button>` : ''}
    <button type="button" class="ib hp-bigb" data-act="hpNext" aria-label="Nächster Sender" data-fid="pl-next">${icon('next', 28)}</button></div>`;
  const top = `<div class="hp-top"><button type="button" class="ib back" data-act="back" aria-label="Zurück" data-fid="pl-back">${icon('back')}</button><div class="hp-ttl"><b>${c.no} ${esc(c.name)}</b><small>${nn.now ? esc(nn.now.title) : esc(LU.TX.epgNone)}</small></div>${hpPlRecBtn(c, nn)}
    <button type="button" class="ib hp-land" data-act="hpList" aria-label="Senderliste" data-fid="pl-list">${icon('list')}</button><button type="button" class="ib hp-land" data-act="hpGuide" aria-label="Programm" data-fid="pl-guide">${icon('calendar')}</button>
    <button type="button" class="ib" data-act="hpSettings" aria-label="Ton, Untertitel und Qualität" data-fid="pl-set">${icon('settings')}</button></div>`;
  const bot = ts ? hpSeekHtml(c, pl, nn) : `<div class="hp-bot">${nn.now ? LU.progress(nn.now.start, nn.now.end, { label: 'Fortschritt der Sendung' }) : ''}<div class="hp-brow"><span class="hp-pill live"><i aria-hidden="true"></i>${esc(LU.TX.live)}</span><span class="hp-pos">${nn.now ? LU.nowText('left', nn.now.end) : ''}</span></div></div>`;
  const banner = `<div class="hp-banner${Date.now() < P.infoUntil || P.info ? ' on' : ''}" aria-live="polite"><div class="hp-bn-h"><span class="lh-no">${c.no}</span>${LU.logo(c)}<b>${esc(c.name)}</b>${P.fb ? '<small>Ersatzquelle</small>' : ''}${SIM.rights().recordings && SIM.runningOn(c.id) ? LU.badge('recording', { text: true }) : ''}</div>
    ${nn.now ? `<p><small>Jetzt</small> ${fmt.range(nn.now.start, nn.now.end)} <b>${esc(nn.now.title)}</b></p>${LU.progress(nn.now.start, nn.now.end)}` : `<p>${esc(LU.TX.epgNone)}</p>`}${nn.next ? `<p><small>Danach</small> ${fmt.time(nn.next.start)} <b>${esc(nn.next.title)}</b></p>` : ''}</div>`;
  const side = `<aside class="hp-side${P.listOpen ? ' on' : ''}" aria-label="Senderliste"><div class="hp-side-h"><button type="button" class="btn ghost sm" data-act="hpListSel" data-fid="pl-lsel2">${esc(hpListName(hpList(pl)))} ${icon('down', 14)}</button><button type="button" class="ib" data-act="hpListClose" aria-label="Senderliste schliessen" data-fid="pl-lclose">${icon('close', 22)}</button></div><ul class="hp-list">${list.map((x) => hpChRow(x, x.id === id, 'sl')).join('')}</ul></aside>`;
  const stage = `<section class="hp-stage${P.ctl ? ' ctl-on' : ''}" data-phase="${phase}" data-err="${phase === 'error' ? 1 : 0}" aria-label="Fernsehbild von ${esc(c.name)}">
    ${SIM.chState(c) === 'gone' ? '<div class="lv-vid bad"></div>' : LU.fakeVideo(c, { paused: pl.paused })}
    <button type="button" class="hp-tapzone" data-act="hpStageTap" aria-label="Bedienung ein- oder ausblenden" data-fid="pl-tap"></button>
    <div class="hp-ov hp-load" role="status"><span class="hp-spin">${icon('loadspin', 36)}</span><b data-hp-loadtxt>${esc(loadTxt)}</b></div>
    <div class="hp-ov hp-stall${P.stall > Date.now() ? ' on' : ''}" role="status"><span class="hp-spin">${icon('loadspin', 36)}</span><b>Verbindung wird wiederhergestellt …</b></div>
    ${errHtml}${banner}
    <div class="hp-ctl">${top}${mid}${bot}</div>${side}</section>`;
  const cur = nn.now; const rs = cur && r.recordings ? SIM.recState(cur.id) : null;
  const below = `<section class="hp-below">
    <div class="hp-info"><div class="hp-info-h"><span class="lh-no">${c.no}</span>${LU.logo(c)}<b>${esc(c.name)}</b>${P.fb ? '<small>Ersatzquelle</small>' : ''}${SIM.chState(c) === 'disturbed' ? `<span class="hp-warn">${icon('alert', 14)}${esc(LU.TX.unavailable)}</span>` : ''}</div>
      ${cur ? `<p class="hp-ln"><small>Jetzt</small> <span>${fmt.range(cur.start, cur.end)}</span></p><b class="hp-it">${esc(cur.title)}</b>${cur.sub ? `<small>${esc(cur.sub)}</small>` : ''}${LU.progress(cur.start, cur.end)}${LU.programBadges(cur, { text: true })}` : `<p class="hp-ln">${esc(SIM.epgOf(c) === 'none' ? LU.TX.epgNoneButPlays : LU.TX.epgNone)}</p>`}
      ${nn.next ? `<p class="hp-ln"><small>Danach</small> <span>${fmt.time(nn.next.start)}</span> <b>${esc(nn.next.title)}</b></p>` : ''}
      <div class="hp-btnrow">${hpPlRecBtn(c, nn)}${cur ? `<a class="btn ghost sm" href="#/live/programs/${cur.id}" data-fid="pl-det">${icon('info', 18)}Details</a>` : `<a class="btn ghost sm" href="#/live/guide/channel/${c.id}" data-fid="pl-det">${icon('calendar', 18)}Senderprogramm</a>`}</div></div>
    <div class="hp-lhead"><button type="button" class="btn ghost sm" data-act="hpListSel" data-fid="pl-lsel">Liste: ${esc(hpListName(hpList(pl)))} ${icon('down', 14)}</button><a class="btn ghost sm" href="#/live/guide" data-fid="pl-prog">${icon('calendar', 18)}Programm</a></div>
    <ul class="hp-list" aria-label="Senderliste">${list.map((x) => hpChRow(x, x.id === id, 'lr')).join('')}</ul>
    <p class="lh-sample">${esc(LU.TX.sample)}</p></section>`;
  return { full: true, nav: '', html: `<div class="hp-pl" data-live>${stage}${below}</div>`, after: () => hpPlTick() };
});

/* tick: phases, timers, seek bar labels (no redraw) */
function hpPlTick() {
  if (!hpPlOn()) return; const P = HP.p; const pl = SIM.player(); const st = $('.hp-stage'); if (!pl || !st) return;
  const c = SIM.channel(pl.ch); const ph = SIM.mode() === 'loading' ? 'loading' : hpPhase(c);
  if (st.dataset.phase !== ph) st.dataset.phase = ph;
  const lt = $('[data-hp-loadtxt]', st); if (lt) { const tx = Date.now() < P.reload ? P.reloadTxt : ph === 'slow' ? 'Dauert länger als üblich …' : `${c.name} wird geladen`; if (lt.textContent !== tx) lt.textContent = tx; }
  { const s = $('.hp-stall', st); if (s) s.classList.toggle('on', P.stall > Date.now() && ph === 'play'); }
  if ((ph === 'error') !== (st.dataset.err === '1')) render(); /* a fault state of the core switched */
  const rt = $('[data-hp-retry]', st); if (rt) rt.textContent = String(20 - (Math.floor(Math.max(0, Date.now() - P.tuneAt - 7000) / 1000) % 20));
  if (P.ctl && ph === 'play' && !pl.paused && Date.now() > P.ctlUntil && !$('.hp-ctl :focus-visible')) { P.ctl = false; st.classList.remove('ctl-on'); }
  const bn = $('.hp-banner', st); if (bn) bn.classList.toggle('on', Date.now() < P.infoUntil || P.info);
  if (hpPlTick.drag) return;
  const nn = SIM.nowNext(c.id); const max = pl.max;
  const pos = $('[data-hp-pos]'); if (pos) pos.textContent = fmt.time(pl.position);
  $$('[data-hp-el]').forEach((e) => { const rr = SIM.recording(e.dataset.hpEl); if (rr) e.textContent = hpRecElapsed(rr); });
  const trk = $('[data-hp-trk]'); if (trk && nn) { const pct = max > 0.01 ? Math.max(0, Math.min(100, (1 - pl.offset / max) * 100)) : 100; $('.hp-fill', trk).style.width = pct.toFixed(1) + '%'; $('.hp-knob', trk).style.left = pct.toFixed(1) + '%'; }
  const lb = $('.hp-lbls'); if (lb && lb.lastElementChild) lb.lastElementChild.textContent = `${LU.TX.live} ${fmt.time(SIM.now())}`;
  const pill = $('.hp-pill.shift'); if (pill) pill.textContent = `${pl.paused ? 'Pausiert · ' : ''}${fmt.offset(pl.offset)} · ${LU.TX.backToLive}`;
  if (SIM.rights().timeshift && pl.live !== !!$('.hp-pill.live')) render(); /* flipped between live and shifted without an event */
}
setInterval(hpPlTick, 250);
SIM.on((t) => { if (t === 'tick') hpPlTick(); });

/* switch channel */
function hpDoSwitch(to) {
  const pl = SIM.player(); const l = hpList(pl); const k = l.indexOf(pl.ch); const n = l.indexOf(to);
  history.replaceState(null, '', '#/live/watch/' + to);
  SIM.tune(to, { list: l });
  if (n === 0 && k > 0) toast(`Wieder am Anfang: ${SIM.channel(to).no} ${SIM.channel(to).name}`);
}
function hpSwitch(to) {
  const pl = SIM.player(); if (!pl || to === pl.ch) return;
  if ((pl.offset > 1 || pl.paused) && !HP.noAsk) return LiveDlg.zapConfirm({ to, onConfirm: () => hpDoSwitch(to) });
  hpDoSwitch(to);
}
function hpZap(dir) { const pl = SIM.player(); if (!pl) return; const l = hpList(pl); const k = l.indexOf(pl.ch); hpSwitch(l[(k + dir + l.length) % l.length]); }
ACT.hpPrev = () => hpZap(-1);
ACT.hpNext = () => hpZap(1);
ACT.hpPick = (d) => { closeSheet(true); if (HP.p) HP.p.listOpen = false; hpSwitch(d.ch); };
ACT.hpStageTap = () => { const P = HP.p; if (Date.now() - P.swiped < 400) return; P.ctl = !P.ctl; P.ctlUntil = Date.now() + 4500; const st = $('.hp-stage'); if (st) st.classList.toggle('ctl-on', P.ctl); };
ACT.hpPlPP = () => { const pl = SIM.player(); if (!pl) return; if (!SIM.rights().timeshift) return toast(LU.TX.noTimeshift || 'Anhalten und Zurückspulen sind für dieses Profil nicht freigegeben.'); if (pl.paused) SIM.resume(); else SIM.pause(); };
ACT.hpSeekRel = (d) => { const pl = SIM.player(); if (!pl) return; SIM.seek(pl.offset - Number(d.s) / 60); };
ACT.hpToLive = () => SIM.toLive();
ACT.hpList = () => { HP.p.listOpen = !HP.p.listOpen; render(); };
ACT.hpListClose = () => { HP.p.listOpen = false; render(); };
ACT.hpOther = () => { HP.p.listOpen = true; HP.p.force = null; render(); const b = $('.hp-below'); if (b && b.offsetParent) b.scrollIntoView({ block: 'start' }); };
ACT.hpRetryPl = () => { HP.p.force = null; HP.p.tuneAt = Date.now(); render(); };
ACT.hpHome = () => { SIM.stop(); go('#/'); };
ACT.hpListSel = () => {
  const opts = [['', 'Alle Sender'], ['fav', 'Meine Sender'], ...LIVE.groups.map((g) => [g.id, 'Gruppe ' + g.name])];
  const cur = hpListName(hpList(SIM.player()));
  sheet(`<h2>Liste</h2>${opts.map(([v, t]) => li('list', t, '', t === cur ? icon('check', 20) : '', `data-act="hpListSet" data-v="${v}" data-fid="ls-${v || 'all'}"`)).join('')}<div class="acts"><button type="button" class="btn ghost" data-act="close" data-fid="ls-x">Schliessen</button></div>`);
};
ACT.hpListSet = (d) => { const l = SIM.channels({ group: d.v }).map((c) => c.id); closeSheet(true); if (!l.length) return toast('Diese Liste ist leer'); HP.p.list = l; render(); };
ACT.hpGuide = () => {
  const pl = SIM.player(); const ids = hpList(pl).map(SIM.channel).filter(Boolean);
  sheet(`<h2>Programm</h2><p class="pad hint">Tippe auf eine laufende Sendung, um den Sender zu wechseln.</p><ul class="hp-gl">${ids.map((c) => { const nn = SIM.nowNext(c.id); return `<li><button type="button" class="hp-gsw" data-act="hpPick" data-ch="${c.id}" data-fid="pg-${c.id}"><span class="lh-no">${c.no}</span>${LU.logo(c)}<span class="hp-lr-t"><b>${esc(c.name)}</b><small>${nn.now ? esc(nn.now.title) + ' · ' + fmt.range(nn.now.start, nn.now.end) : esc(LU.TX.epgNone)}</small></span></button>${nn.next ? `<a class="hp-gnx2" href="#/live/programs/${nn.next.id}" data-fid="pg-n-${c.id}"><small>danach ${fmt.time(nn.next.start)}</small> ${esc(nn.next.title)}</a>` : ''}</li>`; }).join('')}</ul><div class="acts"><a class="btn ghost" href="#/live/guide" data-fid="pg-full">Ganze Programmzeitschrift</a><button type="button" class="btn ghost" data-act="close" data-fid="pg-x">Schliessen</button></div>`);
};

/* sound, subtitles, quality (sheet) with the mockup entry for error states */
let HPS = null;
function hpSetHtml() {
  const pl = SIM.player(); const c = SIM.channel(pl ? pl.ch : 'srf1'); const P = HP.p; const mia = SIM.profile().age === 6;
  const chip = (g, i, t, on, dis) => `<button type="button" class="chip${on ? ' on' : ''}" aria-pressed="${on}" ${dis ? 'disabled aria-disabled="true"' : ''} data-act="hpSetPick" data-g="${g}" data-i="${i}" data-fid="set-${g}-${i}">${esc(t)}</button>`;
  return `<h2>Ton, Untertitel, Qualität</h2><div class="sec">Ton</div><div class="chips lh-wrap">${c.audio.map((a, i) => chip('audio', i, a.replace(/ \(.*\)$/, '') + (/Audiodeskription/.test(a) ? ' (Audiodeskription)' : ''), P.audio === i)).join('')}</div>
    <div class="sec">Untertitel</div><div class="chips lh-wrap">${chip('subs', -1, 'Aus', P.subs === -1)}${c.subs.map((s, i) => chip('subs', i, s, P.subs === i)).join('')}</div>${c.subs.length ? '' : `<p class="pad hint">Dieser Sender sendet keine Untertitel.</p>`}
    <div class="sec">Qualität</div><div class="chips lh-wrap">${chip('q', 'auto', 'Automatisch', P.q === 'auto')}${chip('q', 'orig', 'Original (1080i)', P.q === 'orig', mia)}${chip('q', '720', '720p', P.q === '720')}${chip('q', '480', '480p', P.q === '480')}</div>${mia ? '<p class="pad hint">Höchstens 720p für dieses Profil.</p>' : ''}
    <div class="acts"><button type="button" class="btn primary" data-act="close" data-fid="set-ok">Fertig</button></div>`;
}
ACT.hpSettings = () => { HPS = true; sheet(hpSetHtml()); };
ACT.hpSetPick = (d) => {
  const P = HP.p; const k = d.g; const v = d.g === 'q' ? d.i : Number(d.i);
  if (P[k] === v) return; P[k] = v;
  if (k === 'audio') { P.reload = Date.now() + 1300; P.reloadTxt = 'Wechsle Tonspur …'; } else if (k === 'q') { P.reload = Date.now() + 1300; P.reloadTxt = 'Wechsle Qualität …'; }
  const sh = $('#overlays .sheet'); if (sh) { const f = document.activeElement && document.activeElement.dataset.fid; const sc = sh.scrollTop; sh.innerHTML = hpSetHtml(); sh.scrollTop = sc; const e = f && sh.querySelector(`[data-fid="${f}"]`); if (e) e.focus(); }
  const st = $('.hp-stage'); if (st) hpPlTick();
};
/* fault states of the player: mockup bar «Zustände» → Player (LIVE_SIM.flags().fault) */

/* swipe up/down on the picture = next/previous channel; drag on the bar = timeshift position */
(function hpPlGestures() {
  let sw = null; let dr = null;
  const sc = () => $('#device').getBoundingClientRect().width / $('#device').offsetWidth || 1;
  document.addEventListener('pointerdown', (e) => {
    const trk = e.target.closest('[data-hp-trk]');
    if (trk) { const pl = SIM.player(); if (!pl || !SIM.rights().timeshift) return; dr = { trk, pl }; hpPlTick.drag = true; hpPlSeekTo(e, dr); trk.setPointerCapture(e.pointerId); return; }
    const st = e.target.closest('.hp-stage'); if (!st || e.target.closest('button:not(.hp-tapzone), a, .hp-side')) return;
    sw = { x: e.clientX, y: e.clientY };
  });
  document.addEventListener('pointermove', (e) => { if (dr) hpPlSeekTo(e, dr); });
  const up = (e) => {
    if (dr) { const d = dr; dr = null; hpPlTick.drag = false; const off = hpPlSeekTo(e, d); SIM.seek(off); return; }
    if (!sw) return; const dy = (e.clientY - sw.y) / sc(); const dx = (e.clientX - sw.x) / sc(); sw = null;
    if (Math.abs(dy) > 60 && Math.abs(dy) > Math.abs(dx) * 1.5) { HP.p.swiped = Date.now(); hpZap(dy < 0 ? 1 : -1); }
  };
  document.addEventListener('pointerup', up); document.addEventListener('pointercancel', () => { sw = null; if (dr) { dr = null; hpPlTick.drag = false; } });
  document.addEventListener('focusin', (e) => { if (e.target.closest && e.target.closest('.hp-ctl') && HP.p) { HP.p.ctl = true; HP.p.ctlUntil = Date.now() + 4500; const st = $('.hp-stage'); if (st) st.classList.add('ctl-on'); } });
})();
function hpPlSeekTo(e, d) {
  const b = d.trk.getBoundingClientRect(); const x = Math.max(0, Math.min(1, (e.clientX - b.left) / b.width)); const max = d.pl.max; const off = (1 - x) * max;
  $('.hp-fill', d.trk).style.width = (x * 100) + '%'; $('.hp-knob', d.trk).style.left = (x * 100) + '%';
  const pill = $('.hp-pill.shift'); const pos = $('[data-hp-pos]'); if (pos) pos.textContent = fmt.time(SIM.now() - off);
  return off < 0.05 ? 0 : off;
}
/* keyboard of the live player (14, 4.8) */
document.addEventListener('keydown', (e) => {
  if (!hpPlOn() || !e.target.closest || e.target.closest('input, select, textarea') || $('#overlays .scrim') || e.ctrlKey || e.metaKey || e.altKey) return;
  const pl = SIM.player(); if (!pl) return; const k = e.key; let used = true;
  if (k === 'ArrowLeft' || k === 'ArrowRight') { if (!SIM.rights().timeshift) toast('Anhalten und Zurückspulen sind für dieses Profil nicht freigegeben.'); else SIM.seek(pl.offset + (k === 'ArrowLeft' ? 1 : -1) * (e.shiftKey ? 1 : 1 / 6)); }
  else if (k === 'PageUp' || k === 'n' || k === 'N') hpZap(1); else if (k === 'PageDown' || k === 'p' || k === 'P') hpZap(-1);
  else if (k === ' ' || k === 'k' || k === 'K') { if (e.target.closest('button, a')) return; ACT.hpPlPP(); }
  else if (k === 'l' || k === 'L' || k === 'End') SIM.toLive();
  else if (k === 'r' || k === 'R') { if (SIM.rights().recordings) ACT.hpPlRec(); }
  else if (k === 'c' || k === 'C') ACT.hpList(); else if (k === 'g' || k === 'G') ACT.hpGuide();
  else if (k === 'i' || k === 'I') { HP.p.infoUntil = Date.now() + 5000; render(); }
  else used = false;
  if (used) e.preventDefault();
});
/* leaving the player ends the playback (no picture in picture, no mini player) */
window.addEventListener('hashchange', (e) => {
  const w = /#\/live\/watch\//; if (w.test(e.oldURL) && !w.test(e.newURL)) { if (SIM.player()) SIM.stop(); if (HP.p) { HP.p.list = null; HP.p.listOpen = false; HP.p.force = null; HP.p.key = ''; HP.p.fb = false; } }
});

/* ============================================================ LS-08 Meine Aufnahmen */
const HP_SORT = { new: 'Neueste zuerst', old: 'Älteste zuerst', ch: 'Sender', title: 'Titel', size: 'Grösse', exp: 'Läuft bald ab' };
const hpRecDur = (r) => fmt.dur(r.end - r.start);
const hpRecMeta = (r) => `${esc(SIM.channel(r.ch).name)} · ${fmt.day(r.start)} · ${hpRecDur(r)}`;
const hpRecSize = (r) => (r.state === 'running' ? r.liveGB || r.sizeGB : r.sizeGB);
function hpMeter() {
  const q = SIM.quota(); const w = (x) => (x / q.totalGB * 100).toFixed(1) + '%';
  return `<div class="card hp-meter"><p><b>${esc(`Speicher des Kontos: ${fmt.gb(q.usedGB)} von ${fmt.gb(q.totalGB)} belegt`)}</b></p><div class="hp-bar" role="img" aria-label="Du ${fmt.gb(q.mineGB)}, übrige Profile ${fmt.gb(q.othersGB)}, frei ${fmt.gb(q.freeGB)}"><i class="a" style="width:${w(q.mineGB)}"></i><i class="b" style="width:${w(q.othersGB)}"></i></div>
    <p class="hp-legend"><span><i class="a"></i>Du ${fmt.gb(q.mineGB)}</span><span><i class="b"></i>Übrige Profile ${fmt.gb(q.othersGB)}</span><span><i class="c"></i>Frei ${fmt.gb(q.freeGB)}</span></p></div>${q.pct >= 90 ? `<div class="note warn" role="status">${icon('alert', 20)}<div>Der Speicher des Kontos ist fast voll. Neue Aufnahmen können abgelehnt werden.</div></div>` : ''}`;
}
/* «Kontingent voll» is a mockup state of the core (bar «Zustände» → Aufnahmen); SIM.quota() reports it */
const hpFullSwitch = () => '';
document.addEventListener('click', (e) => { if (e.target.closest('[data-tool="reset"]')) setTimeout(() => { HP.noAsk = false; HP.radio = null; SIM.emit('change'); }, 0); }, true);

function hpRecUnits(list) {
  const out = []; const seen = {};
  list.forEach((r) => {
    if (r.origin === 'series' && r.rule && r.state !== 'failed' && r.state !== 'rejected') {
      const grp = list.filter((x) => x.rule === r.rule && x.state !== 'failed' && x.state !== 'rejected');
      if (grp.length > 1) { if (!seen[r.rule]) { seen[r.rule] = true; out.push({ kind: 'stack', rule: r.rule, items: grp, r }); } return; }
    }
    out.push({ kind: 'rec', r });
  });
  return out;
}
const hpUnitState = (u) => (u.kind === 'stack' ? (u.items.some((x) => x.state === 'incomplete') ? 'incomplete' : 'done') : u.r.state);
function hpSortUnits(units) {
  const s = HP.rec.sort; const key = (u) => u.r;
  const f = { new: (a, b) => key(b).start - key(a).start, old: (a, b) => key(a).start - key(b).start, ch: (a, b) => SIM.channel(key(a).ch).no - SIM.channel(key(b).ch).no || key(b).start - key(a).start, title: (a, b) => key(a).title.localeCompare(key(b).title, 'de'), size: (a, b) => (u2s(b) - u2s(a)), exp: (a, b) => (exp(a) - exp(b)) };
  const u2s = (u) => (u.kind === 'stack' ? u.items.reduce((x, y) => x + y.sizeGB, 0) : hpRecSize(u.r)); const exp = (u) => (u.kind === 'rec' && u.r.keep && u.r.keep.mode === 'days' ? (u.r.keep.until != null ? u.r.keep.until : u.r.start + u.r.keep.days * 1440) : 1e9);
  return units.slice().sort(f[s] || f.new);
}
function hpRecRow(r, o) {
  o = o || {}; const nw = !r.watched && !r.watchedPct && r.state !== 'running'; const sel = HP.rec.sel.includes(r.id);
  const badges = `${r.state === 'incomplete' ? LU.badge('incomplete', { text: true }) : ''}${nw ? '<span class="badge acc">Neu</span>' : ''}${r.origin === 'series' ? `<span class="badge">Serie</span>` : ''}`;
  const pic = `<span class="hp-rr-pic">${LU.picture({ ch: r.ch, title: r.title, img: r.img }, { fallback: 'logo' })}${r.watchedPct ? `<span class="hp-ovprog"><span class="lv-prog" role="progressbar" aria-valuenow="${r.watchedPct}" aria-valuemin="0" aria-valuemax="100" aria-label="Angesehen: ${r.watchedPct} %"><i style="width:${r.watchedPct}%"></i></span></span>` : ''}</span>`;
  const body = `<span class="hp-rr-body"><b>${esc(r.title)}</b>${r.sub ? `<small>${esc(r.sub)}</small>` : ''}<small>${hpRecMeta(r)}</small><span class="hp-pr-meta">${badges}<small>${esc(hpKeepText(r.keep, r))}</small></span></span>`;
  if (HP.rec.select) return `<li class="hp-rr"><label class="hp-rr-main hp-selrow"><input type="checkbox" ${sel ? 'checked' : ''} data-act="hpRecSel" data-id="${r.id}" data-fid="rs-${r.id}" aria-label="${esc(r.title)}${r.sub ? ', ' + esc(r.sub) : ''} auswählen">${pic}${body}</label></li>`;
  return `<li class="hp-rr"><a class="hp-rr-main" href="#/live/recordings/${r.id}" data-fid="rr-${r.id}" aria-label="${esc(r.title)}${r.sub ? ', ' + esc(r.sub) : ''}, ${hpRecMeta(r)}">${pic}${body}</a><span class="hp-pr-act"><a class="ib" href="#/live/recordings/${r.id}/play" aria-label="${esc(r.title)} abspielen" data-fid="rp-${r.id}">${icon('play', 22)}</a>${o.del ? `<button type="button" class="ib" data-act="hpKidDel" data-id="${r.id}" aria-label="${esc(r.title)} löschen" data-fid="rk-${r.id}">${icon('trash', 22)}</button>` : ''}</span></li>`;
}
route('/live/recordings/profiles', () => hpProfilesPage());
route('/live/recordings', (p, q) => {
  HP.rec.series = q.series || '';
  const R = HP.rec; const me0 = SIM.profile(); const empty = SIM.mode() === 'empty';
  const all = empty ? [] : SIM.recordings();
  let body = hpState(5, 96);
  const tools = `<div class="hp-tools"><a class="btn ghost sm" href="#/live/scheduled" data-fid="rec-sch">${icon('clock', 18)}Geplant (${SIM.upcoming().length})</a><button type="button" class="btn ghost sm" data-act="hpNewRec" data-fid="rec-new">${icon('plus', 18)}Neue Aufnahme</button><span class="hp-sp"></span>${all.length ? `<button type="button" class="btn ${R.select ? 'primary' : 'ghost'} sm" aria-pressed="${R.select}" data-act="hpRecSelMode" data-fid="rec-selmode">${R.select ? 'Fertig' : 'Auswählen'}</button>` : ''}</div>`;
  if (!body) {
    const kids = me0.primary ? SIM.childProfiles().filter((x) => SIM.recordings(x.id).length) : [];
    const units = hpRecUnits(all.filter((r) => r.state !== 'failed' && r.state !== 'rejected')); const failed = all.filter((r) => r.state === 'failed' || r.state === 'rejected');
    const cnt = (s) => units.filter((u) => hpUnitState(u) === s).length;
    const fl = [['all', 'Alle', units.length], ['running', 'Läuft', cnt('running')], ['done', 'Fertig', cnt('done')], ['incomplete', 'Unvollständig', cnt('incomplete')], ['failed', 'Fehlgeschlagen', failed.length]];
    const head = `${hpMeter()}${kids.length ? `<a class="card hp-card-link" href="#/live/recordings/profiles" data-fid="rec-kids"><span><b>Aufnahmen meiner Profile</b><small>${kids.map((k) => k.name).join(', ')}</small></span>${icon('right', 18)}</a>` : ''}`;
    if (!all.length) body = `${head}${hpEmptyBox('record', 'Noch keine Aufnahmen', 'Plane Aufnahmen in der Programmzeitschrift oder starte eine im Player.', `<a class="btn ghost" href="#/live/guide" data-fid="rec-toguide">Zum Programm</a>`)}`;
    else if (R.series) {
      const rule = SIM.rule(R.series) || { title: 'Serie' }; const eps = all.filter((x) => x.rule === R.series).sort((a, b) => b.start - a.start);
      body = `<div class="pad hp-tools"><a class="btn ghost sm" href="#/live/recordings" data-fid="rec-allser">${icon('back', 18)}Alle Aufnahmen</a></div><div class="sec">${esc(rule.title)} · ${eps.length} Folgen${rule.keepLast ? ` · behalte die letzten ${rule.keepLast}` : ''}</div><ul class="hp-plist">${eps.map((x) => hpRecRow(x)).join('')}</ul>`;
    } else {
      const shown = hpSortUnits(units.filter((u) => R.filter === 'all' || R.filter === 'failed' ? R.filter === 'all' : hpUnitState(u) === R.filter));
      const run = shown.filter((u) => u.kind === 'rec' && u.r.state === 'running'); const rest = shown.filter((u) => !(u.kind === 'rec' && u.r.state === 'running'));
      const sel = HP.rec.sel.map((id) => SIM.recording(id)).filter(Boolean);
      body = `${head}<div class="chips" role="group" aria-label="Zustand">${fl.map(([k, t, n]) => `<button type="button" class="chip${R.filter === k ? ' on' : ''}" aria-pressed="${R.filter === k}" data-act="hpRecFilter" data-k="${k}" data-fid="rf-${k}">${t} ${n}</button>`).join('')}</div>
        <div class="hp-tools"><button type="button" class="btn ghost sm" data-act="hpRecSort" data-fid="rec-sort">${icon('sort', 18)}${esc(HP_SORT[R.sort])}</button></div>
        ${run.length ? `<div class="sec">Läuft</div>${run.map((u) => hpRunCard(u.r)).join('')}` : ''}
        ${rest.length ? `<div class="sec">${R.filter === 'all' ? 'Aufnahmen' : esc((fl.find((x) => x[0] === R.filter) || [])[1] || '')}</div><ul class="hp-plist">${rest.map((u) => (u.kind === 'stack' ? hpStackRow(u) : hpRecRow(u.r))).join('')}</ul>` : ''}
        ${!run.length && !rest.length && R.filter !== 'failed' ? hpEmptyBox('filter', 'Keine Aufnahmen mit diesem Zustand', '') : ''}
        ${failed.length && (R.filter === 'all' || R.filter === 'failed') ? `<div class="hp-fold"><button type="button" class="hp-fold-h" aria-expanded="${R.failOpen || R.filter === 'failed'}" data-act="hpFailToggle" data-fid="rec-fail">${icon('alert', 20)}<span><b>Fehlgeschlagen und nicht aufgenommen</b><small>${failed.length} Aufnahmen</small></span>${icon(R.failOpen || R.filter === 'failed' ? 'up' : 'down', 20)}</button>
          ${R.failOpen || R.filter === 'failed' ? `<ul class="hp-plist">${failed.map((x) => `<li class="hp-rr"><span class="hp-rr-main"><span class="hp-rr-body"><b>${esc(x.title)}</b><small>${hpRecMeta(x)}</small><span class="hp-pr-meta">${LU.badge(x.state === 'failed' ? 'failed' : 'warning', { text: true, label: LU.REC_STATE[x.state] })}</span><small>${esc(LU.REASON[x.reason] || '')}</small></span></span><span class="hp-pr-act"><button type="button" class="btn ghost sm" data-act="hpRecRemove" data-id="${x.id}" aria-label="${esc(x.title)} entfernen" data-fid="rm-${x.id}">Entfernen</button></span></li>`).join('')}</ul>` : ''}</div>` : ''}
        ${R.select ? `<div class="hp-selbar" role="region" aria-label="Auswahl"><span>${sel.length} ausgewählt · ${fmt.gb(sel.reduce((a, x) => a + hpRecSize(x), 0))}</span><button type="button" class="btn danger sm" ${sel.length ? '' : 'disabled'} data-act="hpRecDelSel" data-fid="rec-delsel">${icon('trash', 18)}Löschen</button></div>` : ''}${hpFullSwitch()}`;
    }
  }
  return { nav: '', html: livePage({ tab: 'recordings', title: 'Meine Aufnahmen', html: `${tools}${body}` }) };
});
function hpStackRow(u) {
  const eps = u.items; const r = u.r; const gb = eps.reduce((a, x) => a + x.sizeGB, 0); const rule = SIM.rule(u.rule) || {};
  return `<li class="hp-rr"><a class="hp-rr-main" href="#/live/recordings?series=${u.rule}" data-fid="rst-${u.rule}" aria-label="${esc(r.title)}, ${eps.length} Folgen, Serie öffnen"><span class="hp-rr-pic hp-stack">${LU.picture({ ch: r.ch, title: r.title, img: r.img }, { fallback: 'logo' })}</span><span class="hp-rr-body"><b>${esc(r.title)} · ${eps.length} Folgen</b><small>${fmt.gb(gb)} · letzte ${fmt.day(eps[0].start)}</small><span class="hp-pr-meta"><span class="badge">Serie</span>${eps.some((x) => !x.watched && !x.watchedPct) ? '<span class="badge acc">Neu</span>' : ''}${eps.some((x) => x.state === 'incomplete') ? LU.badge('incomplete', { text: true }) : ''}<small>${rule.keepLast ? `Behalte die letzten ${rule.keepLast} Folgen` : 'Behalten, bis du löschst'}</small></span></span>${icon('right', 18)}</a></li>`;
}
function hpRunCard(r) {
  const c = SIM.channel(r.ch);
  return `<div class="card hp-run"><div class="hp-run-h">${LU.badge('recording', { text: true, label: 'Nimmt auf' })}<b>${esc(r.title)}</b></div><p class="hint">${esc(c.name)} · Nimmt auf seit ${fmt.time(r.start)} · bis ${fmt.time(r.end)} · <span data-hp-gb>${fmt.gb(hpRecSize(r))}</span> bisher</p>${LU.progress(r.start, r.end, { cls: 'rec', label: 'Fortschritt der Aufnahme' })}
    <div class="hp-btnrow"><a class="btn secondary sm" href="#/live/recordings/${r.id}/play" data-fid="run-play-${r.id}">${icon('play', 18)}Ansehen</a><a class="btn ghost sm" href="#/live/recordings/${r.id}" data-fid="run-det-${r.id}">Details</a><button type="button" class="btn ghost sm" data-act="hpStopRec" data-id="${r.id}" data-fid="run-stop-${r.id}">${icon('close', 18)}Beenden</button></div></div>`;
}
ACT.hpNewRec = () => LiveDlg.manualRecording({});
ACT.hpRecFilter = (d) => { HP.rec.filter = d.k; render(); };
ACT.hpRecSort = () => sheet(`<h2>Sortierung</h2>${Object.entries(HP_SORT).map(([k, t]) => li('sort', t, '', HP.rec.sort === k ? icon('check', 20) : '', `data-act="hpRecSortSet" data-k="${k}" data-fid="so-${k}"`)).join('')}<div class="acts"><button type="button" class="btn ghost" data-act="close" data-fid="so-x">Schliessen</button></div>`);
ACT.hpRecSortSet = (d) => { HP.rec.sort = d.k; closeSheet(true); render(); };
ACT.hpFailToggle = () => { HP.rec.failOpen = !HP.rec.failOpen; render(); };
ACT.hpRecRemove = (d) => { SIM.deleteRecordings([d.id]); toast('Eintrag entfernt'); };
ACT.hpRecSelMode = () => { HP.rec.select = !HP.rec.select; HP.rec.sel = []; render(); };
ACT.hpRecSel = (d, el) => { const i = HP.rec.sel.indexOf(d.id); if (el.checked && i < 0) HP.rec.sel.push(d.id); else if (!el.checked && i >= 0) HP.rec.sel.splice(i, 1); const fid = el.dataset.fid; render(); const n = document.querySelector(`[data-fid="${fid}"]`); if (n) n.focus(); };
ACT.hpRecDelSel = () => LiveDlg.confirmDelete({ kind: 'recordings', ids: HP.rec.sel.slice(), onDone: () => { HP.rec.sel = []; HP.rec.select = false; } });

/* ============================================================ LS-09 Aufnahme-Detail */
const hpOwnRec = (id) => { const r = SIM.recording(id); if (!r) return null; const p = SIM.profile(); if (r.profile === p.id) return r; if (p.primary && SIM.childProfiles().some((x) => x.id === r.profile)) return r; return null; };
route('/live/recordings/:id', (pm) => {
  const m = SIM.mode(); const r = hpOwnRec(pm.id);
  if (m === 'loading') return { nav: '', html: `${hpBackBar('Aufnahme')}<div class="pad hp-skel"><span class="lv-skel" style="height:200px"></span><span class="lv-skel" style="height:34px"></span><span class="lv-skel" style="height:90px"></span></div>` };
  if (m === 'error') return { nav: '', html: `${hpBackBar('Aufnahme')}${hpState()}` };
  if (!r) return hpNotFound('Diese Aufnahme gibt es nicht.', '#/live/recordings', 'Zu meinen Aufnahmen');
  const c = SIM.channel(r.ch); const own = r.profile === SIM.profile().id; const bad = r.state === 'failed' || r.state === 'rejected'; const rule = r.rule ? SIM.rule(r.rule) : null;
  const gapMin = r.gaps.reduce((a, g) => a + (g[1] - g[0]), 0); const dur = r.end - r.start;
  const ra = (ic, label, act, extra, fid, cls) => `<button type="button" class="hp-ra${cls ? ' ' + cls : ''}" data-act="${act}" ${extra} data-fid="${fid}"><span class="hp-ra-i">${icon(ic, 24)}</span><span>${esc(label)}</span></button>`;
  const resume = r.watchedPct > 0 && r.watchedPct < 95 && !bad && r.state !== 'running';
  const resTxt = r.resumeAt || hpMMSS(dur * r.watchedPct / 100);
  const kids = !own ? `<div class="note">${icon('info', 20)}<div>Aufnahme von ${esc(hpProfName(r.profile))}. Du siehst sie, weil ${esc(hpProfName(r.profile))} eine Altersgrenze hat.</div></div>` : '';
  const from = `${r.origin === 'series' && rule ? `Aus der Serienaufnahme «${esc(rule.title)}»` : r.origin === 'instant' ? 'Sofortaufnahme' : rule ? `Einzeln geplant am ${fmt.day(rule.created)}` : 'Einzeln geplant'}`;
  const tl = !bad ? `<div class="hp-tl" role="img" aria-label="Zeitleiste der Aufnahme, ${r.gaps.length} Lücken">${r.padBefore ? `<i class="pad" style="left:0;width:${(r.padBefore / dur * 100).toFixed(1)}%"></i>` : ''}${r.padAfter ? `<i class="pad" style="right:0;width:${(r.padAfter / dur * 100).toFixed(1)}%"></i>` : ''}${r.gaps.map((g) => `<i class="lv-gap gp" style="left:${((g[0] - r.start) / dur * 100).toFixed(1)}%;width:${Math.max(1.5, (g[1] - g[0]) / dur * 100).toFixed(1)}%"></i>`).join('')}</div>` : '';
  const html = `${hpBackBar('Aufnahme')}${kids}
    <div class="hp-head"><div class="hp-head-pic">${LU.picture({ ch: r.ch, title: r.title, img: r.img }, { fallback: 'logo' })}</div><div class="pad"><h2 class="hp-h2">${esc(r.title)}</h2>${r.sub ? `<p class="hp-sub">${esc(r.sub)}</p>` : ''}
      <p class="hp-meta">${esc(c.name)} · ${esc(fmt.day(r.start))}, ${fmt.range(r.start, r.end)} · ${hpRecDur(r)}${r.padBefore || r.padAfter ? `, davon Vorlauf ${r.padBefore} Min. und Nachlauf ${r.padAfter} Min.` : ''}</p>
      <div class="hp-chips"><span class="badge ${r.state === 'done' ? 'ok' : ''}">${esc(LU.REC_STATE[r.state])}</span>${bad ? '' : `<span class="badge">${r.state === 'running' ? fmt.gb(hpRecSize(r)) + ' bisher' : fmt.gb(r.sizeGB)}</span>`}</div></div></div>
    <div class="hp-ras" role="toolbar" aria-label="Aktionen">
      ${!bad ? `<a class="hp-ra" href="#/live/recordings/${r.id}/play" data-fid="rd-play"><span class="hp-ra-i">${icon('play', 24)}</span><span>${resume ? 'Weiter ab ' + esc(resTxt) : 'Abspielen'}</span></a>` : ''}
      ${resume ? `<a class="hp-ra" href="#/live/recordings/${r.id}/play?from=start" data-fid="rd-start"><span class="hp-ra-i">${icon('rew', 24)}</span><span>Von vorne</span></a>` : ''}
      ${r.state === 'running' ? ra('close', 'Aufnahme beenden', 'hpStopRec', `data-id="${r.id}"`, 'rd-stop') : ''}
      ${!bad && r.state !== 'running' ? ra('clock', 'Aufbewahrung', 'hpKeep', `data-id="${r.id}"`, 'rd-keep') : ''}
      ${bad ? ra('close', 'Entfernen', 'hpRecRemoveGo', `data-id="${r.id}"`, 'rd-rm') : ra('trash', 'Löschen', 'hpRecDelete', `data-id="${r.id}"`, 'rd-del')}</div>
    <div class="sec">Zustand</div><div class="pad hp-state">
      ${r.state === 'done' ? '<p>Vollständig aufgenommen.</p>' : ''}${r.state === 'running' ? `<p>Nimmt auf seit ${fmt.time(r.start)} · bis ${fmt.time(r.end)}</p>${LU.progress(r.start, r.end, { cls: 'rec' })}` : ''}
      ${bad ? `<div class="note warn" role="status">${icon('alert', 20)}<div><b>${esc(LU.REC_STATE[r.state])}.</b> ${esc(LU.REASON[r.reason] || '')}</div></div>` : ''}
      ${r.state === 'incomplete' ? `<div class="note warn" role="status">${icon('alert', 20)}<div>Unvollständig: ${r.gaps.length} Lücken, zusammen ${Math.round(gapMin)} Minuten. ${r.gaps.map((g, i) => `${i ? 'und ' : 'Der Sender war '}von ${fmt.time(g[0])} bis ${fmt.time(g[1])}`).join(' ')} nicht erreichbar.</div></div>` : ''}
      ${tl}${r.state === 'incomplete' ? '<p class="hint">Schraffiert: Lücken. Heller: Vor- und Nachlauf.</p>' : ''}
      ${r.fallbackAt ? `<div class="note">${icon('info', 20)}<div>Um ${fmt.time(r.fallbackAt)} auf eine Ersatzquelle gewechselt.</div></div>` : ''}
      ${!bad ? `<p class="hp-lnr"><small>Spuren</small><br>${esc(r.tracks)}</p><p class="hp-lnr"><small>Aufbewahrung</small><br>${esc(hpKeepText(r.keep, r))}</p>` : ''}</div>
    <div class="sec">Herkunft</div><div class="pad"><p>${rule && r.origin === 'series' ? `<a class="hp-lnk" href="#/live/scheduled?tab=series" data-fid="rd-from">${from}</a>` : from}</p></div>
    <p class="lh-sample">${esc(LU.TX.sample)}</p>`;
  return { nav: '', html };
});
ACT.hpKeep = (d) => LiveDlg.retention(d.id);
ACT.hpRecDelete = (d) => { const r = SIM.recording(d.id); LiveDlg.confirmDelete({ kind: r.profile !== SIM.profile().id ? 'childRecording' : r.state === 'running' ? 'running' : 'recording', ids: [d.id], onDone: () => history.back() }); };
ACT.hpRecRemoveGo = (d) => { SIM.deleteRecordings([d.id]); toast('Eintrag entfernt'); history.back(); };

/* ============================================================ LS-10 Aufnahme-Player */
const hpRpKey = () => ((location.hash.match(/recordings\/([^/?]+)\/play/) || [])[1] || '');
function hpRpAvail(r) { return r.state === 'running' ? Math.max(0.1, Math.min(SIM.now(), r.end) - r.start) : r.end - r.start; }
const hpMMSS = (min) => { const s = Math.max(0, Math.round(min * 60)); const h = Math.floor(s / 3600); const mm = Math.floor(s / 60) % 60; const ss = s % 60; return (h ? h + ':' + String(mm).padStart(2, '0') : String(mm)) + ':' + String(ss).padStart(2, '0'); };
route('/live/recordings/:id/play', (pm, q) => {
  const r = hpOwnRec(pm.id); const m = SIM.mode();
  if (m === 'loading') return { full: true, nav: '', html: `<div class="hp-pl"><section class="hp-stage ctl-on" data-phase="loading"><div class="hp-ov hp-load"><span class="hp-spin">${icon('loadspin', 36)}</span><b>Aufnahme wird geladen</b></div><div class="hp-ctl"><div class="hp-top"><button type="button" class="ib back" data-act="back" aria-label="Zurück" data-fid="rp-back">${icon('back')}</button></div></div></section></div>` };
  if (m === 'error') return { full: true, nav: '', html: `<div class="hp-pl"><div class="hp-top"><button type="button" class="ib back" data-act="back" aria-label="Zurück" data-fid="rp-back">${icon('back')}</button></div>${hpState()}</div>` };
  if (!r || r.state === 'failed' || r.state === 'rejected') return hpNotFound(r ? 'Diese Aufnahme lässt sich nicht abspielen. ' + (LU.REASON[r.reason] || '') : 'Diese Aufnahme gibt es nicht.', '#/live/recordings', 'Zu meinen Aufnahmen');
  const c = SIM.channel(r.ch); const key = r.id;
  const first = !HP.rp || HP.rp.id !== key;
  if (first) {
    const ra = r.resumeAt ? r.resumeAt.split(':').map(Number) : null; const pctMin = (r.watchedPct > 0 && r.watchedPct < 95) ? (ra ? ra[0] + (ra[1] || 0) / 60 : hpRpAvail(r) * r.watchedPct / 100) : 0;
    HP.rp = { id: key, pos: q.from === 'start' ? r.padBefore : (pctMin ? pctMin : r.padBefore), playing: !pctMin || q.from === 'start', ctl: true, ctlUntil: Date.now() + 4500, asked: q.from === 'start' || !pctMin, resume: pctMin, audio: 0, subs: 0, last: Date.now() };
  }
  const P = HP.rp; const dur = hpRpAvail(r); P.pos = Math.min(P.pos, dur);
  const live = r.state === 'running' && P.pos >= dur - 0.1;
  const trk = `<div class="hp-trk" data-hp-rtrk role="slider" tabindex="0" aria-label="Position in der Aufnahme" aria-valuemin="0" aria-valuemax="${Math.round(dur * 60)}" aria-valuenow="${Math.round(P.pos * 60)}" aria-valuetext="${hpMMSS(P.pos)} von ${hpMMSS(dur)}" data-fid="rp-seek">
    ${r.padBefore ? `<i class="pad" style="left:0;width:${(r.padBefore / dur * 100).toFixed(1)}%"></i>` : ''}${r.state !== 'running' && r.padAfter ? `<i class="pad" style="right:0;width:${(r.padAfter / dur * 100).toFixed(1)}%"></i>` : ''}${r.gaps.map((g) => `<i class="lv-gap gp" style="left:${((g[0] - r.start) / dur * 100).toFixed(1)}%;width:${Math.max(1.5, (g[1] - g[0]) / dur * 100).toFixed(1)}%"></i>`).join('')}<i class="hp-fill" style="width:${(P.pos / dur * 100).toFixed(1)}%"></i><i class="hp-knob" style="left:${(P.pos / dur * 100).toFixed(1)}%"></i></div>`;
  const stage = `<section class="hp-stage${P.ctl ? ' ctl-on' : ''}" data-phase="play" aria-label="Wiedergabe der Aufnahme ${esc(r.title)}">${LU.fakeVideo(c, { paused: !P.playing })}
    <button type="button" class="hp-tapzone" data-act="hpRpTap" aria-label="Bedienung ein- oder ausblenden" data-fid="rp-tap"></button>
    <div class="hp-ctl"><div class="hp-top"><button type="button" class="ib back" data-act="back" aria-label="Zurück" data-fid="rp-back">${icon('back')}</button><div class="hp-ttl"><b>${esc(r.title)}</b><small>${esc(r.sub || c.name)}</small></div><button type="button" class="ib" data-act="hpRpTracks" aria-label="Ton und Untertitel" data-fid="rp-tracks">${icon('settings')}</button></div>
      <div class="hp-mid"><button type="button" class="ib hp-bigb" data-act="hpRpSeek" data-s="-10" aria-label="10 Sekunden zurück" data-fid="rp-rew">${icon('rew', 28)}</button><button type="button" class="ib hp-bigb hp-pp" data-act="hpRpPP" aria-label="${P.playing ? 'Pause' : 'Wiedergabe'}" data-fid="rp-pp">${icon(P.playing ? 'pause' : 'play', 32)}</button><button type="button" class="ib hp-bigb" data-act="hpRpSeek" data-s="10" aria-label="10 Sekunden vor" data-fid="rp-fwd">${icon('fwd', 28)}</button></div>
      <div class="hp-bot"><div class="hp-seekrow">${trk}</div><div class="hp-lbls"><span data-hp-rpos>${hpMMSS(P.pos)}</span><span data-hp-rdur>${hpMMSS(dur)}</span></div>
        <div class="hp-brow">${r.state === 'running' ? (live ? `<span class="hp-pill live"><i aria-hidden="true"></i>Aufnahme läuft</span>` : `<button type="button" class="hp-pill shift" data-act="hpRpLive" data-fid="rp-live">Aufnahme läuft · Zum aktuellen Stand</button>`) : `<span class="hp-pos">Ganze Aufnahme ${hpMMSS(dur)}</span>`}</div></div></div></section>`;
  const html = `<div class="hp-pl" data-rp>${stage}<section class="hp-below"><div class="hp-info"><b class="hp-it">${esc(r.title)}</b>${r.sub ? `<small>${esc(r.sub)}</small>` : ''}<small>${hpRecMeta(r)}</small><div class="hp-btnrow"><a class="btn ghost sm" href="#/live/recordings/${r.id}" data-fid="rp-det">${icon('info', 18)}Details</a><button type="button" class="btn ghost sm" data-act="hpRpTracks" data-fid="rp-tracks2">${icon('subs', 18)}Ton und Untertitel</button></div>
    ${r.state === 'incomplete' ? `<div class="note warn" style="margin:12px 0 0">${icon('alert', 20)}<div>Unvollständig: ${r.gaps.length} Lücken (schraffiert in der Leiste).</div></div>` : ''}</div><p class="lh-sample">${esc(LU.TX.sample)}</p></section></div>`;
  return { full: true, nav: '', html, after: () => { if (!P.asked) { P.asked = true; hpRpAsk(r); } } };
});
function hpRpAsk(r) {
  const P = HP.rp; const t = hpMMSS(P.resume);
  sheet(`<h2>${esc(r.title)}</h2><p class="pad hint">Du hast ${r.watchedPct} % angesehen.</p>${li('play', `Weiter ab ${t}`, '', '', `data-act="hpRpFrom" data-m="${P.resume}" data-fid="rq-resume"`)}${li('rew', 'Von vorne', 'Ab Sendungsbeginn', '', `data-act="hpRpFrom" data-m="${r.padBefore}" data-fid="rq-start"`)}${r.padBefore ? li('clock', 'Ab Aufnahmebeginn (mit Vorlauf)', '', '', `data-act="hpRpFrom" data-m="0" data-fid="rq-full"`) : ''}`);
}
ACT.hpRpFrom = (d) => { HP.rp.pos = Number(d.m); HP.rp.playing = true; closeSheet(true); render(); };
ACT.hpRpTap = () => { const P = HP.rp; P.ctl = !P.ctl; P.ctlUntil = Date.now() + 4500; const st = $('.hp-stage'); if (st) st.classList.toggle('ctl-on', P.ctl); };
ACT.hpRpPP = () => { HP.rp.playing = !HP.rp.playing; render(); };
ACT.hpRpSeek = (d) => { const r = SIM.recording(HP.rp.id); HP.rp.pos = Math.max(0, Math.min(hpRpAvail(r), HP.rp.pos + Number(d.s) / 60)); hpRpUpd(); };
ACT.hpRpLive = () => { const r = SIM.recording(HP.rp.id); HP.rp.pos = hpRpAvail(r); render(); };
ACT.hpRpTracks = () => {
  const r = SIM.recording(HP.rp.id); const c = SIM.channel(r.ch); const P = HP.rp;
  const chip = (g, i, t, on) => `<button type="button" class="chip${on ? ' on' : ''}" aria-pressed="${on}" data-act="hpRpTrk" data-g="${g}" data-i="${i}" data-fid="rt-${g}-${i}">${esc(t)}</button>`;
  sheet(`<h2>Ton und Untertitel</h2><div class="sec">Ton</div><div class="chips lh-wrap">${c.audio.map((a, i) => chip('audio', i, a.replace(/ \(AAC.*\)$/, ''), P.audio === i)).join('')}</div><div class="sec">Untertitel</div><div class="chips lh-wrap">${chip('subs', 0, 'Aus', P.subs === 0)}${c.subs.map((s, i) => chip('subs', i + 1, s, P.subs === i + 1)).join('')}</div><p class="pad hint">${c.subs.length ? 'Alle empfangenen Spuren sind erhalten.' : 'Dieser Sender sendet keine Untertitel.'} Eine Qualitätswahl gibt es für Aufnahmen nicht.</p><div class="acts"><button type="button" class="btn primary" data-act="close" data-fid="rt-ok">Fertig</button></div>`);
};
ACT.hpRpTrk = (d) => { HP.rp[d.g] = Number(d.i); closeSheet(true); toast(d.g === 'audio' ? 'Tonspur gewechselt' : 'Untertitel geändert'); };
function hpRpUpd() {
  const P = HP.rp; const r = P && SIM.recording(P.id); const t = $('[data-hp-rtrk]'); if (!r || !t) return; const dur = hpRpAvail(r); const pct = (P.pos / dur * 100).toFixed(1) + '%';
  $('.hp-fill', t).style.width = pct; $('.hp-knob', t).style.left = pct; const a = $('[data-hp-rpos]'); if (a) a.textContent = hpMMSS(P.pos); const d = $('[data-hp-rdur]'); if (d) d.textContent = hpMMSS(dur);
}
setInterval(() => {
  const P = HP.rp; if (!P || !$('[data-rp]')) return; const r = SIM.recording(P.id); if (!r) return; const now = Date.now(); const dt = (now - P.last) / 1000 / 60; P.last = now;
  if (P.playing && !$('#overlays .scrim')) { const dur = hpRpAvail(r); P.pos = Math.min(dur, P.pos + dt); if (P.pos >= dur && r.state !== 'running') { P.playing = false; render(); return; } }
  hpRpUpd(); if (!hpPlTick.drag) { const dur = hpRpAvail(r); r.watchedPct = Math.min(100, Math.round(P.pos / dur * 100)); }
  if (P.ctl && P.playing && now > P.ctlUntil) { P.ctl = false; const st = $('.hp-stage'); if (st) st.classList.remove('ctl-on'); }
}, 250);
/* drag on the recording bar */
(function hpRpDrag() {
  let dr = null;
  const at = (e, t) => { const b = t.getBoundingClientRect(); const x = Math.max(0, Math.min(1, (e.clientX - b.left) / b.width)); const r = SIM.recording(HP.rp.id); HP.rp.pos = x * hpRpAvail(r); hpRpUpd(); };
  document.addEventListener('pointerdown', (e) => { const t = e.target.closest('[data-hp-rtrk]'); if (!t || !HP.rp) return; dr = t; at(e, t); t.setPointerCapture(e.pointerId); HP.rp.ctlUntil = Date.now() + 4500; });
  document.addEventListener('pointermove', (e) => { if (dr) at(e, dr); });
  ['pointerup', 'pointercancel'].forEach((n) => document.addEventListener(n, () => { dr = null; }));
})();
window.addEventListener('hashchange', (e) => {
  if (/\/play/.test(e.oldURL) && !/\/play/.test(e.newURL) && HP.rp) { const r = SIM.recording(HP.rp.id); if (r) SIM.setWatched(r.id, r.watchedPct || 0); HP.rp = null; }
});
