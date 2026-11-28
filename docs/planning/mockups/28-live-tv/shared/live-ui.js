/* Konzept 28 «Live-TV»: what all three mockup clients share for display: formatters for the mockup
   time, channel logo with fallback, programme picture with fallback, progress bars that follow the
   mockup clock, badge texts, the sample-data footnote. Returns HTML strings with classes «lv-…»
   (styles in shared/live-ui.css; each client sets sizes with its own CSS).
   Depends on live-data.js and live-sim.js. Set LIVE_UI.img (path to shared/img/) once per client. */
'use strict';
(function (g) {
  const L = g.LIVE; const SIM = g.LIVE_SIM;
  const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  const pad = (n) => String(n).padStart(2, '0');
  const WD = ['So', 'Mo', 'Di', 'Mi', 'Do', 'Fr', 'Sa'];
  const WDL = ['Sonntag', 'Montag', 'Dienstag', 'Mittwoch', 'Donnerstag', 'Freitag', 'Samstag'];
  /* calendar of a mockup minute (UTC arithmetic, independent of the browser's time zone) */
  const cal = (t) => { const d = new Date(Date.UTC(2026, 10, 28) + Math.floor(t / 1440) * 86400000); return { wd: d.getUTCDay(), d: d.getUTCDate(), m: d.getUTCMonth() + 1, y: d.getUTCFullYear() }; };
  const dayOf = (t) => Math.floor(t / 1440);

  const fmt = {
    /** «20:41» */
    time: (t) => { const m = ((Math.floor(t) % 1440) + 1440) % 1440; return pad(Math.floor(m / 60)) + ':' + pad(m % 60); },
    /** «20:41:07» (player) */
    clock: (t) => { const s = Math.floor(((t % 1440) + 1440) % 1440 * 60); return pad(Math.floor(s / 3600)) + ':' + pad(Math.floor(s / 60) % 60) + ':' + pad(s % 60); },
    /** «Sa 28.11.» */
    day: (t) => { const c = cal(t); return `${WD[c.wd]} ${c.d}.${c.m}.`; },
    /** «Samstag, 28. November» */
    dayLong: (t) => { const c = cal(t); return `${WDL[c.wd]}, ${c.d}. ${['Januar', 'Februar', 'März', 'April', 'Mai', 'Juni', 'Juli', 'August', 'September', 'Oktober', 'November', 'Dezember'][c.m - 1]}`; },
    /** «Heute», «Morgen», «Gestern» or «Mo 30.11.» */
    dayRel: (t) => { const d = dayOf(t) - dayOf(SIM.now()); return d === 0 ? 'Heute' : d === 1 ? 'Morgen' : d === -1 ? 'Gestern' : fmt.day(t); },
    /** «Heute, 20:15» */
    when: (t) => `${fmt.dayRel(t)}, ${fmt.time(t)}`,
    /** «20:10–22:00» */
    range: (a, b) => `${fmt.time(a)}–${fmt.time(b)}`,
    /** «1 Std. 42 Min.» / «35 Min.» */
    dur: (min) => { min = Math.round(min); return min >= 60 ? `${Math.floor(min / 60)} Std.${min % 60 ? ` ${min % 60} Min.` : ''}` : `${min} Min.`; },
    /** «−12:30» for a timeshift offset in minutes */
    offset: (min) => { const s = Math.round(min * 60); return `−${Math.floor(s / 60)}:${pad(s % 60)}`; },
    /** number in de-CH: decimal point, apostrophe for thousands: dec(10354) «10'354», dec(8.1, 1) «8.1», dec(8, 1, 1) «8.0» */
    dec: (n, max, min) => { const v = Number(n).toFixed(max == null ? 0 : max); let [i, f] = v.split('.'); i = i.replace(/\B(?=(\d{3})+(?!\d))/g, "'"); if (f && (min == null || min < f.length)) { f = f.replace(/0+$/, ''); while (min && f.length < min) f += '0'; } return f ? `${i}.${f}` : i; },
    /** «2.9 GB», «1.2 TB» (de-CH: decimal point) */
    gb: (x) => (x >= 1000 ? `${fmt.dec(x / 1000, 1)} TB` : `${fmt.dec(x, 1)} GB`),
    /** «10'354» (Swiss thousands separator) */
    num: (n) => String(n).replace(/\B(?=(\d{3})+(?!\d))/g, "'"),
    /** «vor 5 Min.», «vor 2 Std.», «gestern» */
    ago: (t) => { const m = SIM.now() - t; if (m < 1) return 'gerade eben'; if (m < 60) return `vor ${Math.round(m)} Min.`; if (m < 1440) return `vor ${Math.round(m / 60)} Std.`; return dayOf(SIM.now()) - dayOf(t) === 1 ? 'gestern' : `am ${fmt.day(t)}`; },
    /** minutes left of a running programme */
    left: (p) => Math.max(0, Math.ceil(p.end - SIM.now())),
  };

  /* ---------- logo with fallback (no logos in the repository) ---------- */
  const initials = (name) => { const w = String(name).replace(/[^\p{L}\p{N} ]/gu, ' ').trim().split(/\s+/); return (w.length > 1 ? w[0][0] + (w[1][0] || '') : w[0].slice(0, 3)).toUpperCase(); };
  /* Logos and pictures live only locally in shared/img/ (a link to ~/.local/share/slouchcat-design/media/…,
     never in the repository). Mode «automatisch» (default): once per browser session a single probe
     (new Image() on shared/img/logos/srf1.png) decides; loaded = pictures on, failed = off, and then that one
     failed request is the only one. The switch «Echte Logos und Bilder» (index.html or bar «Zustände»)
     overrides with on/off and is remembered in localStorage (key sc28-img-mode); the probe result is kept in
     sessionStorage (key sc28-img-probe), so no page probes again. While the probe runs, the page stays hidden
     (at most 600 ms), so no initials flash before the logos. */
  const IMGKEY = 'sc28-img-mode'; const PROBEKEY = 'sc28-img-probe';
  const IMGBASE = '../shared/img/';
  let probeMem = null;
  const imageMode = () => { try { const v = localStorage.getItem(IMGKEY); return v === 'on' || v === 'off' ? v : 'auto'; } catch (e) { return 'auto'; } };
  const probeResult = () => { try { return sessionStorage.getItem(PROBEKEY) || probeMem; } catch (e) { return probeMem; } };
  const imagesOn = () => { const m = imageMode(); return m === 'on' || (m === 'auto' && probeResult() === 'ok'); };
  const setImageMode = (m) => { try { localStorage.setItem(IMGKEY, m); } catch (e) { /* private mode */ } if (m === 'auto') probeImages(); if (SIM) SIM.emit('change'); };
  const setImages = (on) => setImageMode(on ? 'on' : 'off');
  function probeImages() {
    if (imageMode() !== 'auto' || probeResult()) return;
    const root = document.documentElement; let done = false;
    const finish = (r) => {
      if (done) return; done = true; probeMem = r;
      try { sessionStorage.setItem(PROBEKEY, r); } catch (e) { /* private mode */ }
      root.style.visibility = '';
      if (r === 'ok' && SIM) SIM.emit('change');
    };
    root.style.visibility = 'hidden';
    const im = new Image();
    im.onload = () => finish('ok'); im.onerror = () => finish('fail');
    setTimeout(() => { if (!done) { root.style.visibility = ''; } }, 600);
    im.src = IMGBASE + 'logos/srf1.png';
  }
  probeImages();
  /* only these 16 logos exist (locally in shared/img/logos/); other channels (TV24, new, test) show initials */
  const LOGOS = new Set(['srf1', 'srfzwei', 'srfinfo', 'rts1', 'rsila1', 'telezueri', 'daserste', 'zdf', '3sat', 'arte', 'zdfneo', 'zdfinfo', 'tagesschau24', 'phoenix', 'kika', 'orf2']);
  /** channel logo on a light neutral plate (logos are dark, black or tall); falls back to initials.
      o: { cls, label } — size comes from the client's CSS (.lv-logo width/height). */
  function logo(c, o) {
    o = o || {}; c = typeof c === 'string' ? SIM.channel(c) : c; if (!c) return '';
    const ini = esc(initials(c.name)); const has = LOGOS.has(c.slug) && imagesOn();
    return `<span class="lv-logo${has ? '' : ' no-img'}${o.cls ? ' ' + o.cls : ''}" data-ini="${ini}"${o.label ? ` role="img" aria-label="${esc(c.name)}"` : ' aria-hidden="true"'}>${has ? `<img src="${UI.img}logos/${esc(c.slug)}.png" alt="" loading="lazy" onerror="LIVE_UI.logoFail(this)">` : ''}<b>${ini}</b></span>`;
  }
  function logoFail(im) { const s = im.parentNode; if (s) s.classList.add('no-img'); im.remove(); }
  /** programme picture (16:9) or fallback. Only films and series of the sample have pictures (stand-ins from
      the phone mockup library). o.fallback: 'logo' (channel logo on a calm surface, default), 'title', 'none' */
  function picture(p, o) {
    o = o || {}; const fb = o.fallback || 'logo';
    if (!p) return fb === 'none' ? '' : `<span class="lv-pic lv-pic-empty"></span>`;
    const alt = esc(p.title);
    const fallback = fb === 'none' ? '' : fb === 'title' ? `<span class="lv-pic-fb title"><b>${alt}</b></span>` : `<span class="lv-pic-fb">${logo(p.ch)}</span>`;
    if (!p.img || !imagesOn()) return fb === 'none' ? '' : `<span class="lv-pic no-img${o.cls ? ' ' + o.cls : ''}">${fallback}</span>`;
    return `<span class="lv-pic${o.cls ? ' ' + o.cls : ''}"><img src="${UI.img}lib/${esc(p.img)}" alt="" loading="lazy" onerror="LIVE_UI.picFail(this)">${fallback}</span>`;
  }
  function picFail(im) { const s = im.parentNode; if (s) s.classList.add('no-img'); im.remove(); }

  /* ---------- progress that follows the mockup clock ---------- */
  const pct = (a, b, t) => Math.max(0, Math.min(100, ((t == null ? SIM.now() : t) - a) / (b - a) * 100));
  /** progress bar of a running programme; updates itself every mockup minute */
  const progress = (a, b, o) => `<span class="lv-prog${o && o.cls ? ' ' + o.cls : ''}" data-s="${a}" data-e="${b}" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow="${Math.round(pct(a, b))}" aria-label="${esc((o && o.label) || 'Fortschritt der Sendung')}"><i style="width:${pct(a, b).toFixed(1)}%"></i></span>`;
  /** text that shows the mockup clock: kind 'time' (20:41), 'left' (noch 64 Min., needs data-e) */
  const nowText = (kind, end) => `<span data-lv-now="${kind}"${end != null ? ` data-e="${end}"` : ''}>${kind === 'left' ? `noch ${Math.max(0, Math.ceil(end - SIM.now()))} Min.` : fmt.time(SIM.now())}</span>`;
  function refresh(root) {
    root = root || (typeof document !== 'undefined' ? document : null); if (!root) return;
    root.querySelectorAll('.lv-prog[data-s]').forEach((el) => { const p = pct(Number(el.dataset.s), Number(el.dataset.e)); const i = el.firstElementChild; if (i) i.style.width = p.toFixed(1) + '%'; el.setAttribute('aria-valuenow', Math.round(p)); });
    root.querySelectorAll('[data-lv-now]').forEach((el) => { const k = el.dataset.lvNow; el.textContent = k === 'left' ? `noch ${Math.max(0, Math.ceil(Number(el.dataset.e) - SIM.now()))} Min.` : k === 'clock' ? fmt.clock(SIM.now()) : fmt.time(SIM.now()); });
  }
  if (SIM && typeof document !== 'undefined') SIM.on((t) => { if (t === 'minute' || t === 'tick') refresh(); });
  /* messages of the player that every client shows the same way (its global toast) */
  const TOAST = { restarted: 'Die Wiedergabe war zu lange angehalten und wurde neu gestartet.', retuned: 'Neu eingeschaltet' };
  if (SIM && typeof document !== 'undefined') SIM.on((t) => { if (TOAST[t] && typeof g.toast === 'function') setTimeout(() => g.toast(TOAST[t]), 0); });

  /* ---------- texts and badges ---------- */
  const TX = {
    sample: 'Beispieldaten, simulierte Wiedergabe',
    epgNone: 'Keine Programmdaten', epgNoneButPlays: 'Keine Programmdaten · Sender läuft trotzdem',
    unavailable: 'Gerade nicht erreichbar', disturbed: 'Gestört', gone: 'Nicht mehr im Angebot',
    now: 'Jetzt', next: 'Danach', live: 'LIVE', backToLive: 'Zurück zu Live',
    record: 'Aufnehmen', recording: 'Nimmt auf', scheduled: 'Aufnahme geplant', series: 'Serie geplant', reminder: 'Erinnerung',
    past: 'Vorbei. Slouchcat bietet keine verpassten Sendungen an.',
    noAgeCheck: 'Live-TV zeigt alles, was der Sender zeigt. Slouchcat prüft keine Altersfreigaben von Sendern oder Sendungen.',
    capped: 'Für dein Konto nicht freigeschaltet. Das kann der Server-Admin ändern.',
    loadingErr: 'Live-TV konnte nicht geladen werden.',
    empty: 'Noch keine Sender. Sobald der Server-Admin Sender einrichtet, erscheinen sie hier.',
  };
  const REC_STATE = { running: 'Läuft', done: 'Fertig', incomplete: 'Unvollständig', failed: 'Fehlgeschlagen', rejected: 'Nicht aufgenommen' };
  const REASON = { unreachable: 'Der Sender war nicht erreichbar.', quota: 'Der Speicher des Kontos war voll.', serverDown: 'Der Server lief zu dieser Zeit nicht.', storage: 'Auf dem Server war kein Speicher frei.', channelGone: 'Den Sender gab es nicht mehr.', channelOff: 'Der Sender wurde deaktiviert.' };
  const RULE_KIND = { single: 'Einzeln', series: 'Serie', recurring: 'Wiederkehrend', manual: 'Manuell' };
  const RECEPTION = { default: 'Standard (bei Bedarf)', permanent: 'Permanent', auto: 'Automatisch', ondemand: 'Bei Bedarf', schedule: 'Nach Zeitplan' };
  const SOURCE_KIND = { 'm3u-url': 'M3U-URL', 'm3u-file': 'M3U-Datei', xtream: 'Xtream', single: 'Einzelstrom' };
  /** recording badge: kind 'scheduled' (○), 'recording' (●), 'series' (●●), 'reminder', 'warning', 'incomplete', 'failed'.
      o.text = true shows the word next to the symbol; without it the word is the accessible name. */
  function badge(kind, o) {
    const sym = { scheduled: '○', recording: '●', series: '●●', reminder: '⏰', warning: '⚠', incomplete: '◐', failed: '✕' }[kind] || '•';
    const word = { scheduled: TX.scheduled, recording: TX.recording, series: TX.series, reminder: TX.reminder, warning: 'Warnung', incomplete: 'Unvollständig', failed: 'Fehlgeschlagen' }[kind] || kind;
    const text = o && o.text ? (o.label || word) : '';
    return `<span class="lv-badge ${kind}"${text ? '' : ` role="img" aria-label="${esc(word)}" title="${esc(word)}"`}><span class="sym" aria-hidden="true">${sym}</span>${text ? `<span>${esc(text)}</span>` : ''}</span>`;
  }
  /** badges of a programme for the active profile (recording state, reminder) */
  function programBadges(p, o) {
    if (!p) return '';
    const r = SIM.rights(); let h = '';
    const rs = r.recordings ? SIM.recState(p.id) : null;
    if (rs) h += badge(rs, o);
    if (SIM.reminderFor(p.id)) h += badge('reminder', o);
    return h;
  }
  /** short line «Beispieldaten, simulierte Wiedergabe» for the foot of every Live-TV page */
  /** «4 Warnungen zu geplanten Aufnahmen» (the same words in every client) */
  const warnText = (n) => `${n} ${n === 1 ? 'Warnung' : 'Warnungen'} zu geplanten Aufnahmen`;
  const sampleNote = (cls) => `<p class="lv-sample${cls ? ' ' + cls : ''}">${esc(TX.sample)} · Konzept 28, Mockup · Uhr des Mockups: ${nowText('time')}</p>`;

  /** simulated picture of a running channel: calm surface, moving shimmer, logo, label «simuliert» */
  function fakeVideo(c, o) {
    o = o || {}; c = typeof c === 'string' ? SIM.channel(c) : c;
    const st = SIM.chState(c); const n = SIM.nowNext(c.id).now;
    const pic = n && n.img && imagesOn() ? `<img class="lv-vid-pic" src="${UI.img}lib/${esc(n.img)}" alt="" onerror="this.remove()">` : '';
    return `<div class="lv-vid${o.paused ? ' paused' : ''}${st !== 'ok' ? ' bad' : ''}" data-ch="${esc(c.id)}">${pic}<div class="lv-vid-move" aria-hidden="true"></div>
      <div class="lv-vid-logo">${logo(c)}</div><span class="lv-vid-tag">${esc(TX.sample)}</span></div>`;
  }

  /* ---------- player: switching phases, the same in all three clients ----------
     The concept proposes 0.5 s (calm loading sign), 3 s («Dauert länger als üblich …») and 15 s (error) (14 Fassung 2, 4.11).
     The mockup shortens 5 s and 15 s so that every state can be shown in a few seconds; LOAD.note says so. */
  const LOAD = { spinnerMs: 500, hotReadyMs: 600, coldReadyMs: 1800, slowAfterMs: 2500, slowReadyMs: 4500, giveUpMs: 6000,
    note: 'Im Mockup verkürzt: Ladeanzeige ab 0.5 s, «Dauert länger» ab 2.5 s, Fehlerbild nach 6 s (Konzept 14 Fassung 2: 0.5 / 3 / 15 s).' };
  /** phase of the player elapsedMs after switching to channel c (o.hot: input already received or preloaded).
      → { k: 'loading'|'slow'|'ok'|'error', spin, why, err ('unreachable'|'limit'|'busy'|'busyPerm'|'notPlayable'|'noConvert'|'revoked'|'gone'), stall, fallback }.
      why: reason of the state «startet» (04 6.3, 14 4.11), shown from «Dauert länger» on.
      Reads the mockup states (LIVE_SIM.flags().fault, .revoked) and the channel state. */
  function playerPhase(elapsedMs, c, o) {
    o = o || {}; c = typeof c === 'string' ? SIM.channel(c) : c; const f = SIM.flags(); const e = elapsedMs;
    if (!c || SIM.chState(c) === 'gone') return { k: 'error', err: 'gone', spin: false };
    if (f.revoked) return { k: 'error', err: 'revoked', spin: false };
    if (f.fault === 'limit' || f.fault === 'busy' || f.fault === 'busyPerm' || f.fault === 'notPlayable') return { k: 'error', err: f.fault, spin: false };
    if (f.noContain && NEEDS_CONVERT.has(c.id)) return { k: 'error', err: 'noConvert', spin: false };
    const why = e < 1200 ? 'Verbindung zur Quelle wird aufgebaut' : NEEDS_CONVERT.has(c.id) ? 'Umwandlung startet' : 'Warte auf den Bildanfang (lange Bildgruppen)';
    const wait = () => ({ k: e < LOAD.slowAfterMs ? 'loading' : 'slow', spin: e > LOAD.spinnerMs, why });
    if (SIM.chState(c) !== 'ok' || f.fault === 'unreachable') return e < LOAD.giveUpMs ? wait() : { k: 'error', err: 'unreachable', spin: false };
    const ready = f.fault === 'slow' ? LOAD.slowReadyMs : o.hot ? LOAD.hotReadyMs : LOAD.coldReadyMs;
    if (e < ready) return wait();
    return { k: 'ok', spin: false, stall: f.fault === 'stall', fallback: f.fault === 'fallback' };
  }
  /** channels whose stream needs a conversion (ZDF: «1 umgewandelt» in LIVE.inputs); without containment they cannot play */
  const NEEDS_CONVERT = new Set(['zdf']);
  /** texts of the player errors (14, 4.12); err from playerPhase */
  const PLAYER_ERR = {
    unreachable: (c) => `${c.name} ist gerade nicht erreichbar. Slouchcat versucht es weiter.`,
    limit: () => 'Auf deinem Konto laufen schon 3 Wiedergaben gleichzeitig.',
    revoked: () => 'Live-TV ist für dieses Profil nicht mehr freigegeben.',
    busy: () => 'Der Server ist gerade ausgelastet. Versuche es gleich nochmals.',
    /* 14 4.12: the player does not name the cause; the mockup state is called «Dauerempfang belegt die Kapazität» */
    busyPerm: () => 'Der Server ist gerade ausgelastet. Versuche es gleich nochmals.',
    noConvert: () => 'Dieser Sender lässt sich auf diesem Gerät nur mit Umwandlung abspielen, und die Umwandlung ist auf diesem Server gerade aus.',
    gone: () => 'Diesen Sender gibt es nicht mehr.',
    notPlayable: () => 'Dieser Sender lässt sich auf diesem Gerät nicht abspielen.',
  };

  /* subtitles (14 4.10): browser none until stage E, Teletext in no client */
  const SUBS_TX = { browser: 'Untertitel gibt es im Browser noch nicht. Die Apps zeigen sie.', later: 'Untertitel folgen in einer späteren Stufe', teletext: 'Dieser Sender sendet Untertitel nur als Videotext. Slouchcat zeigt sie noch nicht an.', none: 'Dieser Sender sendet keine Untertitel.' };
  const isTeletext = (s) => /Videotext/.test(s);
  const UI = { SUBS_TX, isTeletext, NEEDS_CONVERT, warnText, imagesOn, setImages, imageMode, setImageMode, probeResult, LOAD, playerPhase, PLAYER_ERR, img: IMGBASE, esc, fmt, cal, dayOf, initials, logo, logoFail, picture, picFail, progress, pct, nowText, refresh, TX, REC_STATE, REASON, RULE_KIND, RECEPTION, SOURCE_KIND, badge, programBadges, sampleNote, fakeVideo };
  g.LIVE_UI = UI;
})(typeof window !== 'undefined' ? window : globalThis);
