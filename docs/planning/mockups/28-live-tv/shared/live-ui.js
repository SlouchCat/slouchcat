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
    /** «2,9 GB», «1,2 TB» */
    gb: (x) => (x >= 1000 ? `${(x / 1000).toLocaleString('de-CH', { maximumFractionDigits: 1 })} TB` : `${(Math.round(x * 10) / 10).toLocaleString('de-CH')} GB`),
    /** «10'354» (Swiss thousands separator) */
    num: (n) => String(n).replace(/\B(?=(\d{3})+(?!\d))/g, "'"),
    /** «vor 5 Min.», «vor 2 Std.», «gestern» */
    ago: (t) => { const m = SIM.now() - t; if (m < 1) return 'gerade eben'; if (m < 60) return `vor ${Math.round(m)} Min.`; if (m < 1440) return `vor ${Math.round(m / 60)} Std.`; return dayOf(SIM.now()) - dayOf(t) === 1 ? 'gestern' : `am ${fmt.day(t)}`; },
    /** minutes left of a running programme */
    left: (p) => Math.max(0, Math.ceil(p.end - SIM.now())),
  };

  /* ---------- logo with fallback (no logos in the repository) ---------- */
  const initials = (name) => { const w = String(name).replace(/[^\p{L}\p{N} ]/gu, ' ').trim().split(/\s+/); return (w.length > 1 ? w[0][0] + (w[1][0] || '') : w[0].slice(0, 3)).toUpperCase(); };
  /** channel logo on a light neutral plate (logos are dark, black or tall); falls back to initials.
      o: { cls, label } — size comes from the client's CSS (.lv-logo width/height). */
  function logo(c, o) {
    o = o || {}; c = typeof c === 'string' ? SIM.channel(c) : c; if (!c) return '';
    const ini = esc(initials(c.name));
    return `<span class="lv-logo${o.cls ? ' ' + o.cls : ''}" data-ini="${ini}"${o.label ? ` role="img" aria-label="${esc(c.name)}"` : ' aria-hidden="true"'}><img src="${UI.img}logos/${esc(c.slug)}.png" alt="" loading="lazy" onerror="LIVE_UI.logoFail(this)"><b>${ini}</b></span>`;
  }
  function logoFail(im) { const s = im.parentNode; if (s) s.classList.add('no-img'); im.remove(); }
  /** programme picture (16:9) or fallback. Only films and series of the sample have pictures (stand-ins from
      the phone mockup library). o.fallback: 'logo' (channel logo on a calm surface, default), 'title', 'none' */
  function picture(p, o) {
    o = o || {}; const fb = o.fallback || 'logo';
    if (!p) return fb === 'none' ? '' : `<span class="lv-pic lv-pic-empty"></span>`;
    const alt = esc(p.title);
    const fallback = fb === 'none' ? '' : fb === 'title' ? `<span class="lv-pic-fb title"><b>${alt}</b></span>` : `<span class="lv-pic-fb">${logo(p.ch)}</span>`;
    if (!p.img) return fb === 'none' ? '' : `<span class="lv-pic no-img${o.cls ? ' ' + o.cls : ''}">${fallback}</span>`;
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
  const REASON = { unreachable: 'Der Sender war nicht erreichbar.', quota: 'Der Speicher des Kontos war voll.', serverDown: 'Der Server lief zu dieser Zeit nicht.', storage: 'Auf dem Server war kein Speicher frei.', channelGone: 'Den Sender gab es nicht mehr.' };
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
  const sampleNote = (cls) => `<p class="lv-sample${cls ? ' ' + cls : ''}">${esc(TX.sample)} · Konzept 28, Mockup · Uhr des Mockups: ${nowText('time')}</p>`;

  /** simulated picture of a running channel: calm surface, moving shimmer, logo, label «simuliert» */
  function fakeVideo(c, o) {
    o = o || {}; c = typeof c === 'string' ? SIM.channel(c) : c;
    const st = SIM.chState(c); const n = SIM.nowNext(c.id).now;
    const pic = n && n.img ? `<img class="lv-vid-pic" src="${UI.img}lib/${esc(n.img)}" alt="" onerror="this.remove()">` : '';
    return `<div class="lv-vid${o.paused ? ' paused' : ''}${st !== 'ok' ? ' bad' : ''}" data-ch="${esc(c.id)}">${pic}<div class="lv-vid-move" aria-hidden="true"></div>
      <div class="lv-vid-logo">${logo(c)}</div><span class="lv-vid-tag">${esc(TX.sample)}</span></div>`;
  }

  const UI = { img: '../shared/img/', esc, fmt, cal, dayOf, initials, logo, logoFail, picture, picFail, progress, pct, nowText, refresh, TX, REC_STATE, REASON, RULE_KIND, RECEPTION, SOURCE_KIND, badge, programBadges, sampleNote, fakeVideo };
  g.LIVE_UI = UI;
})(typeof window !== 'undefined' ? window : globalThis);
