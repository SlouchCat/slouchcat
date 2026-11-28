/* Konzept 28 «Live-TV»: simulation shared by the three mockup clients. Holds every mutable state in
   memory (a reload starts again at Sa 28.11., 20:41): clock, active profile, rights, mockup states,
   favourites, recordings, planning, reminders, the player (channel, timeshift). Nothing is sent anywhere.

   Clients subscribe with LIVE_SIM.on(fn): fn(type) with type 'change' (state changed: redraw),
   'minute' (the mockup minute advanced: progress bars and clocks; LIVE_UI updates them by itself),
   'tick' (every second). Depends on shared/live-data.js. */
'use strict';
(function (g) {
  const L = g.LIVE;
  const clone = (x) => JSON.parse(JSON.stringify(x));
  const listeners = [];
  const emit = (type) => listeners.forEach((fn) => { try { fn(type); } catch (e) { console.error(e); } });

  /* ---------- state ---------- */
  let st;
  function fresh() {
    return {
      clock: { base: L.NOW, t0: Date.now(), running: true },
      profileId: 'anna',
      account: clone(L.account.rights),
      profiles: Object.fromEntries(L.profiles.map((p) => [p.id, clone(p.rights)])),
      mode: 'normal',          // normal | empty | loading | error  (page states of the whole Live-TV area)
      noEpg: false,            // no channel has programme data
      disturbed: false,        // additionally SRF 1 is disturbed (ZDFinfo is disturbed in the data anyway)
      favorites: clone(L.favorites),
      recent: clone(L.recent),
      recordings: clone(L.recordings),
      rules: clone(L.rules),
      reminders: clone(L.reminders),
      dismissed: [],
      player: null,            // { ch, joinedAt, offset (min behind live), paused, pausedAt, list, prev }
      seq: 1,
    };
  }
  st = fresh();

  /* ---------- clock ---------- */
  const now = () => (st.clock.running ? st.clock.base + (Date.now() - st.clock.t0) / 60000 : st.clock.base);
  function setClock(running) { const n = now(); st.clock = { base: n, t0: Date.now(), running }; emit('change'); }
  function advance(min) { const n = now() + min; st.clock = { base: n, t0: Date.now(), running: st.clock.running }; if (st.player && st.player.paused) st.player.offset = Math.min(st.player.offset + min, maxShift()); emit('minute'); emit('change'); }
  let lastMin = Math.floor(now());
  if (typeof setInterval === 'function' && typeof document !== 'undefined') {
    setInterval(() => {
      const m = Math.floor(now());
      if (st.player && st.player.paused) st.player.offset = Math.min(now() - st.player.pausedAt + st.player.pausedOffset, maxShift());
      emit('tick');
      if (m !== lastMin) { lastMin = m; emit('minute'); }
    }, 1000);
  }

  /* ---------- profiles and rights ---------- */
  const profile = (id) => L.profiles.find((p) => p.id === (id || st.profileId));
  function setProfile(id) { if (!profile(id)) return; st.profileId = id; st.player = null; emit('change'); }
  /** effective rights: only when account AND profile have them; recordings and timeshift need live TV */
  function rights(id) {
    const p = st.profiles[id || st.profileId]; const a = st.account;
    const live = a.liveTv && p.liveTv;
    return { liveTv: live, recordings: live && a.recordings && p.recordings, timeshift: live && a.timeshift && p.timeshift };
  }
  const rawRights = (level, id) => (level === 'account' ? st.account : st.profiles[id || st.profileId]);
  function setRight(level, key, on, id) { rawRights(level, id)[key] = !!on; if (!rights().timeshift && st.player) { st.player.offset = 0; st.player.paused = false; } emit('change'); }

  /* ---------- mockup states ---------- */
  function setMode(m) { st.mode = m; emit('change'); }
  function setFlag(k, v) { st[k] = !!v; emit('change'); }

  /* ---------- channels ---------- */
  const allChannels = () => L.channels;
  const channel = (id) => L.channels.find((c) => c.id === id) || L.goneChannels.find((c) => c.id === id) || null;
  /** 'ok' | 'disturbed' | 'gone' */
  function chState(c) { c = typeof c === 'string' ? channel(c) : c; if (!c) return 'gone'; if (c.state === 'gone') return 'gone'; if (c.state === 'disturbed' || (st.disturbed && c.id === 'srf1')) return 'disturbed'; return 'ok'; }
  /** 'ok' | 'none' | 'partial' (as the viewer sees it) */
  function epgOf(c) { c = typeof c === 'string' ? channel(c) : c; if (st.noEpg) return 'none'; return c.epg; }
  const favs = () => st.favorites[st.profileId] || (st.favorites[st.profileId] = []);
  const sortChannels = (a, b) => (a.no == null) - (b.no == null) || (a.no - b.no) || a.name.localeCompare(b.name, 'de');
  /** channels a viewer sees, filtered. f: { q, group ('fav' = Meine Sender), country, lang, category } */
  function channels(f) {
    f = f || {};
    if (st.mode === 'empty') return [];
    let list = allChannels().filter((c) => c.active).slice().sort(sortChannels);
    if (f.group === 'fav') { const fv = favs(); list = fv.map(channel).filter(Boolean); }
    else if (f.group) list = list.filter((c) => c.groups.includes(f.group));
    if (f.country) list = list.filter((c) => c.country === f.country);
    if (f.lang) list = list.filter((c) => c.langs.includes(f.lang));
    if (f.category) list = list.filter((c) => c.category === f.category);
    if (f.q) { const q = f.q.trim().toLowerCase(); list = list.filter((c) => c.name.toLowerCase().includes(q) || String(c.no) === q || String(c.no).startsWith(q) && /^\d+$/.test(q)); }
    return list;
  }
  const channelByNo = (no) => channels().find((c) => c.no === Number(no)) || null;

  /* ---------- programme guide ---------- */
  const byCh = {};
  L.programs.forEach((p) => (byCh[p.ch] || (byCh[p.ch] = [])).push(p));
  const program = (id) => L.programs.find((p) => p.id === id) || null;
  /** programmes of a channel overlapping [from, to); none without programme data */
  function programsOf(chId, from, to) { if (epgOf(chId) === 'none') return []; return (byCh[chId] || []).filter((p) => p.end > from && p.start < to); }
  const programAt = (chId, t) => programsOf(chId, t, t + 0.001)[0] || null;
  /** { now, next } at time t (default now); now null = no data at that time */
  function nowNext(chId, t) {
    t = t == null ? now() : t;
    const list = programsOf(chId, t - 1440, t + 1440);
    const cur = list.find((p) => p.start <= t && p.end > t) || null;
    const nxt = list.find((p) => p.start >= (cur ? cur.end : t)) || null;
    return { now: cur, next: cur && nxt && nxt.start === cur.end ? nxt : (cur ? null : nxt) };
  }
  /** programme-data gaps of a channel in [from, to): [[a, b], …] */
  function gapsOf(chId, from, to) {
    const list = programsOf(chId, from, to); const out = []; let t = from;
    for (const p of list) { if (p.start > t) out.push([t, Math.min(p.start, to)]); t = Math.max(t, p.end); }
    if (t < to) out.push([t, to]);
    return out;
  }
  function genresSoon(hours) {
    const t = now(); const set = new Map();
    channels().forEach((c) => programsOf(c.id, t, t + hours * 60).forEach((p) => set.set(p.genre, (set.get(p.genre) || 0) + 1)));
    return [...set.entries()].filter(([g]) => g !== 'Sonstiges').map(([genre, n]) => ({ genre, n }));
  }
  /** programme search. o: { when: 'now'|'today'|'tomorrow'|'week', genre, group } */
  function search(q, o) {
    o = o || {}; const t = now(); q = (q || '').trim().toLowerCase();
    const range = { now: [t, t + 0.001], today: [t, 1440], tomorrow: [1440, 2880], week: [t, L.EPG_END] }[o.when || 'week'];
    const ids = new Set(channels({ group: o.group }).map((c) => c.id));
    return L.programs.filter((p) => ids.has(p.ch) && epgOf(p.ch) !== 'none' && p.end > range[0] && p.start < range[1]
      && (!q || (p.title + ' ' + p.sub + ' ' + p.desc).toLowerCase().includes(q)) && (!o.genre || p.genre === o.genre)).sort((a, b) => a.start - b.start);
  }

  /* ---------- favourites and history ---------- */
  function toggleFav(id) { const f = favs(); const k = f.indexOf(id); if (k < 0) f.push(id); else f.splice(k, 1); emit('change'); return k < 0; }
  function moveFav(id, dir) { const f = favs(); const k = f.indexOf(id); const j = k + dir; if (k < 0 || j < 0 || j >= f.length) return false; [f[k], f[j]] = [f[j], f[k]]; emit('change'); return true; }
  const isFav = (id) => favs().includes(id);
  const recent = () => (st.recent[st.profileId] || []).map(([id, t]) => ({ ch: channel(id), at: t })).filter((x) => x.ch);

  /* ---------- recordings ---------- */
  function live(r) {
    if (r.state === 'running') {
      const t = now();
      if (t >= r.end) { r.state = 'done'; r.sizeGB = Math.round((r.sizeGB + (r.end - (r.sizedAt || L.NOW)) * (r.gbPerMin || 0.06)) * 10) / 10; }
      else r.liveGB = Math.round((r.sizeGB + Math.max(0, t - (r.sizedAt || L.NOW)) * (r.gbPerMin || 0.06)) * 10) / 10;
    }
    return r;
  }
  const recordings = (pid) => st.recordings.filter((r) => r.profile === (pid || st.profileId)).map(live).sort((a, b) => b.start - a.start);
  const recording = (id) => { const r = st.recordings.find((x) => x.id === id); return r ? live(r) : null; };
  function deleteRecordings(ids) { ids = [].concat(ids); st.recordings = st.recordings.filter((r) => !ids.includes(r.id)); emit('change'); }
  function stopRecording(id) { const r = recording(id); if (!r || r.state !== 'running') return; r.sizeGB = r.liveGB || r.sizeGB; r.end = now(); r.state = 'done'; emit('change'); }
  function setKeep(id, keep) { const r = recording(id); if (r) { r.keep = keep; emit('change'); } }
  function setWatched(id, pct) { const r = recording(id); if (r) { r.watchedPct = pct; r.watched = pct >= 95; emit('change'); } }
  /** the running recording of a channel for the active profile, if any */
  const runningOn = (chId) => recordings().find((r) => r.state === 'running' && r.ch === chId) || null;
  /** profiles with an age limit whose recordings the primary profile may see (LS-14) */
  const childProfiles = () => (profile().primary ? L.profiles.filter((p) => p.age != null && p.age < 18 && p.id !== st.profileId) : []);

  /* ---------- planning ---------- */
  const rules = (pid) => st.rules.filter((r) => r.profile === (pid || st.profileId));
  const rule = (id) => st.rules.find((r) => r.id === id) || null;
  const nid = (p) => `${p}-${st.seq++}`;
  const wd = (t) => (6 + Math.floor(t / 1440)) % 7; // 0 = Sunday; day 0 is a Saturday
  /** concrete upcoming recordings: [{ id, rule, kind, ch, title, sub, start, end, program, skipped? }] */
  function upcoming(pid) {
    const t = now(); const out = []; const seenEp = new Set();
    rules(pid).forEach((r) => {
      if (r.kind === 'single' || r.kind === 'manual') { if (r.end > t) out.push({ id: r.id, rule: r, kind: r.kind, ch: r.ch, title: r.title, sub: (program(r.program) || {}).sub || '', start: r.start, end: r.end, program: r.program || null }); }
      else if (r.kind === 'series') {
        const chans = r.channels === 'all' ? null : r.channels;
        L.programs.filter((p) => p.series === r.series && p.start >= t && (!chans || chans.includes(p.ch)) && (!r.onlyNew || !p.repeat))
          .sort((a, b) => a.start - b.start).forEach((p) => {
            const key = p.sub || p.id;
            const dupe = r.noDup && (seenEp.has(r.id + key) || st.recordings.some((x) => x.rule === r.id && x.sub === p.sub && p.sub));
            if (dupe) return;
            seenEp.add(r.id + key);
            out.push({ id: r.id + ':' + p.id, rule: r, kind: 'series', ch: p.ch, title: p.title, sub: p.sub, start: p.start, end: p.end, program: p.id });
          });
      } else if (r.kind === 'recurring') {
        for (let d = Math.floor(t / 1440); d < L.EPG_END / 1440; d++) {
          const day = d * 1440; if (!r.weekdays.includes(wd(day))) continue;
          const s = day + L.hm(r.from); const e = day + L.hm(r.to);
          if (e > t) out.push({ id: r.id + ':' + d, rule: r, kind: 'recurring', ch: r.ch, title: r.title, sub: '', start: s, end: e, program: (programAt(r.ch, s + 1) || {}).id || null });
        }
      }
    });
    return out.filter((o) => !(o.rule.skip || []).includes(o.id)).sort((a, b) => a.start - b.start);
  }
  /** what a programme has for the active profile: null | 'scheduled' | 'series' | 'recording' */
  function recState(programId) {
    const p = program(programId); if (!p) return null;
    const run = recordings().find((r) => r.state === 'running' && r.ch === p.ch && r.start < p.end && r.end > p.start);
    if (run) return 'recording';
    const up = upcoming().find((u) => u.program === programId || (u.ch === p.ch && u.start < p.end && u.end > p.start + 1));
    if (!up) return null;
    return up.kind === 'series' ? 'series' : 'scheduled';
  }
  const ruleForProgram = (programId) => { const u = upcoming().find((x) => x.program === programId); return u ? u.rule : null; };
  function planRecording(programId, o) {
    o = o || {}; const p = program(programId); if (!p) return null;
    const ex = rules().find((r) => r.kind === 'single' && r.program === programId);
    const base = { padBefore: o.padBefore ?? L.settings.dvr.padBefore, padAfter: o.padAfter ?? L.settings.dvr.padAfter, keep: o.keep || { mode: 'forever' } };
    if (ex) { Object.assign(ex, base); emit('change'); return ex.id; }
    const r = Object.assign({ id: nid('rule'), profile: st.profileId, kind: 'single', program: programId, ch: p.ch, title: p.title, start: p.start, end: p.end, created: now(), estGB: Math.round((p.end - p.start) * 0.06 * 10) / 10 }, base);
    st.rules.push(r); emit('change'); return r.id;
  }
  function planSeries(programId, o) {
    o = o || {}; const p = program(programId); if (!p) return null;
    const ex = rules().find((r) => r.kind === 'series' && r.series === (p.series || p.title));
    const base = Object.assign({ channels: [p.ch], onlyNew: false, noDup: true, keepLast: null, padBefore: L.settings.dvr.padBefore, padAfter: L.settings.dvr.padAfter, keep: { mode: 'forever' }, match: p.series ? 'series' : 'title' }, o);
    if (ex) { Object.assign(ex, base); emit('change'); return ex.id; }
    const r = Object.assign({ id: nid('rule'), profile: st.profileId, kind: 'series', series: p.series || p.title, title: p.seriesName || p.title, created: now() }, base);
    if (!p.series) L.programs.filter((x) => x.title === p.title).forEach((x) => { x.series = x.series || p.title; });
    st.rules.push(r); emit('change'); return r.id;
  }
  /** manual or recurring: o = { ch, title, start, end } (manual) or { ch, title, from, to, weekdays[] } (recurring) */
  function planManual(o) { const r = Object.assign({ id: nid('rule'), profile: st.profileId, kind: o.weekdays ? 'recurring' : 'manual', padBefore: 0, padAfter: 0, keep: { mode: 'forever' }, created: now() }, o); st.rules.push(r); emit('change'); return r.id; }
  function updateRule(id, patch) { const r = rule(id); if (r) { Object.assign(r, patch); emit('change'); } }
  function removeRule(id) { st.rules = st.rules.filter((r) => r.id !== id); emit('change'); }
  /** skip one occurrence of a series or recurring rule */
  function skipOccurrence(occId) { const [rid] = occId.split(':'); const r = rule(rid); if (!r) return; (r.skip || (r.skip = [])).push(occId); emit('change'); }
  /** instant recording in the player. o.until: 'end' | 30 | 60 | 'stop' (max 4 h) */
  function startInstant(chId, o) {
    o = o || {}; const t = now(); const nn = nowNext(chId).now;
    if (quota().freeGB <= 0) return { ok: false, reason: 'quota' };
    const end = o.until === 'end' && nn ? nn.end + L.settings.dvr.padAfter : t + (o.until === 'stop' ? L.settings.dvr.instantMaxH * 60 : Number(o.until) || 60);
    const r = { id: nid('rec'), profile: st.profileId, title: nn ? nn.title : `${channel(chId).name}, ${fmtShort(t)}`, sub: nn ? nn.sub : '', ch: chId, start: t, end, padBefore: 0, padAfter: o.until === 'end' ? L.settings.dvr.padAfter : 0, sizeGB: 0, sizedAt: t, gbPerMin: 0.06, state: 'running', origin: 'instant', keep: o.keep || { mode: 'forever' }, gaps: [], watchedPct: 0, watched: false, tracks: 'Ton: Deutsch', program: nn ? nn.id : null };
    st.recordings.push(r); emit('change'); return { ok: true, id: r.id, end };
  }
  const fmtShort = (t) => { const m = Math.floor(t % 1440); return String(Math.floor(m / 60)).padStart(2, '0') + ':' + String(m % 60).padStart(2, '0'); };

  /* ---------- warnings ---------- */
  function warnings(pid) {
    const list = L.warnings.filter((w) => w.profile === (pid || st.profileId) && !w.hidden && !st.dismissed.includes(w.id) && w.rules.some((id) => rule(id)));
    return list;
  }
  const warningsFor = (ruleId) => warnings().filter((w) => w.rules.includes(ruleId));
  function dismissWarning(id) { st.dismissed.push(id); emit('change'); }

  /* ---------- reminders ---------- */
  const reminders = (pid) => st.reminders.filter((r) => r.profile === (pid || st.profileId) && program(r.program) && program(r.program).start > now() - 1).sort((a, b) => program(a.program).start - program(b.program).start);
  const reminderFor = (programId) => reminders().find((r) => r.program === programId) || null;
  function setReminder(programId, lead) { const r = reminderFor(programId); if (r) r.lead = lead; else st.reminders.push({ id: nid('rem'), profile: st.profileId, program: programId, lead }); emit('change'); }
  function removeReminder(programId) { st.reminders = st.reminders.filter((r) => !(r.program === programId && r.profile === st.profileId)); emit('change'); }

  /* ---------- quota ---------- */
  function quota() {
    const total = L.account.quotaGB; const mine = profile().usedGB || 0; // server figures (14, 0.1)
    const used = L.account.usedGB; const others = Math.round((used - mine) * 10) / 10;
    return { totalGB: total, usedGB: used, mineGB: mine, othersGB: others, freeGB: Math.round((total - used) * 10) / 10, pct: Math.round(used / total * 100) };
  }

  /* ---------- player and timeshift ---------- */
  const maxShift = () => (st.player ? Math.min(L.timeshiftMaxMin, now() - st.player.joinedAt) : 0);
  /** switch on a channel; o.list = zapping list (channel ids). Timeshift starts again on every switch. */
  function tune(chId, o) {
    o = o || {}; const prev = st.player ? st.player.ch : null;
    st.player = { ch: chId, joinedAt: now(), offset: 0, paused: false, pausedAt: 0, pausedOffset: 0, list: o.list || (st.player && st.player.list) || channels().map((c) => c.id), prev };
    const rc = st.recent[st.profileId] || (st.recent[st.profileId] = []);
    const k = rc.findIndex((x) => x[0] === chId); if (k >= 0) rc.splice(k, 1); rc.unshift([chId, now()]);
    emit('change');
  }
  function zap(dir) { const p = st.player; if (!p) return null; const l = p.list; const k = l.indexOf(p.ch); const n = l[(k + dir + l.length) % l.length]; tune(n, { list: l }); return n; }
  function lastChannel() { const p = st.player; if (p && p.prev) tune(p.prev); }
  function pause() { const p = st.player; if (!p || !rights().timeshift || p.paused) return; p.paused = true; p.pausedAt = now(); p.pausedOffset = p.offset; emit('change'); }
  function resume() { const p = st.player; if (!p || !p.paused) return; p.paused = false; emit('change'); }
  /** offset in minutes behind live; clamped to [0, max] */
  function seek(offset) { const p = st.player; if (!p || !rights().timeshift) return; p.offset = Math.max(0, Math.min(offset, maxShift())); if (p.paused) { p.pausedAt = now(); p.pausedOffset = p.offset; } emit('change'); }
  function toLive() { const p = st.player; if (!p) return; p.offset = 0; p.paused = false; emit('change'); }
  function stop() { st.player = null; emit('change'); }
  const player = () => (st.player ? Object.assign({}, st.player, { max: maxShift(), position: now() - st.player.offset, live: st.player.offset < 0.05 && !st.player.paused }) : null);

  function reset() { st = fresh(); lastMin = Math.floor(now()); emit('minute'); emit('change'); }

  g.LIVE_SIM = {
    on: (fn) => { listeners.push(fn); return () => { const k = listeners.indexOf(fn); if (k >= 0) listeners.splice(k, 1); }; },
    emit, now, setClock, advance, clockRunning: () => st.clock.running,
    profile, profiles: () => L.profiles, setProfile, rights, rawRights, setRight,
    mode: () => st.mode, setMode, flags: () => ({ noEpg: st.noEpg, disturbed: st.disturbed }), setFlag,
    channels, channel, channelByNo, chState, epgOf, allChannels,
    program, programsOf, programAt, nowNext, gapsOf, genresSoon, search,
    favorites: () => favs().slice(), isFav, toggleFav, moveFav, recent,
    recordings, recording, deleteRecordings, stopRecording, setKeep, setWatched, runningOn, childProfiles,
    rules, rule, upcoming, recState, ruleForProgram, planRecording, planSeries, planManual, updateRule, removeRule, skipOccurrence, startInstant,
    warnings, warningsFor, dismissWarning,
    reminders, reminderFor, setReminder, removeReminder,
    quota,
    tune, zap, lastChannel, pause, resume, seek, toLive, stop, player,
    reset,
  };
})(typeof window !== 'undefined' ? window : globalThis);
