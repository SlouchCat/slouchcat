/* Konzept 28 «Live-TV», web mockup: what all administration pages share (core). Counters that LA-01, LA-12,
   LA-16 and LA-21 must show alike, the live indicator «Aktualisiert vor n s», and the bridge from the mockup
   states of the administration (bar «Zustände» → Verwaltung) to the pages. Read by admin-a*.js and admin-b*.js. */
'use strict';

/** shared state of the administration pages: ended viewers (LA-21, LA-12), imported channels added by LA-03 */
const ADM = {
  t0: Date.now(), addedImported: 0, ended: {}, endedS: {},
  sinceMin: { 0: 101, 1: 38, 2: 17, 5: 27 },
  baseActive: LIVE.channels.filter((c) => c.active).length,
  baseDist: LIVE.channels.filter((c) => c.active && c.state !== 'ok').length,
};
const admEmpty = () => SIM.mode() === 'empty';
/** «Aktualisiert vor n s» of live pages (LA-01, LA-12, LA-13) */
const admLiveText = () => `Aktualisiert vor ${Math.floor(((Date.now() - ADM.t0) / 1000) % 5) + 1} s`;
setInterval(() => { const el = document.querySelector('.alive'); if (el && !ui.dialog && /^Aktualisiert/.test(el.textContent.trim())) el.lastChild.textContent = admLiveText(); }, 1000);

/** channel counters of the server: the sample has 16 of 584 active channels in detail; changes on the pages count */
function admCounts() {
  const act = LIVE.channels.filter((c) => c.active).length;
  const dist = LIVE.channels.filter((c) => c.active && SIM.chState(c) !== 'ok').length;
  const active = admEmpty() ? 0 : LIVE.totals.active + act - ADM.baseActive;
  const unavailable = admEmpty() ? 0 : Math.max(0, LIVE.totals.unavailable - ADM.baseDist + dist);
  return { imported: admEmpty() ? 0 : LIVE.totals.imported + ADM.addedImported, active, unavailable, available: active - unavailable };
}
/** viewers with all details; viewers ended on LA-21 or LA-12 are gone */
function admViewers() {
  if (admEmpty()) return [];
  return LIVE.consumers.map((c, i) => ({ c, i })).filter((x) => x.c[1] === 'viewer' && !ADM.ended[x.i]).map(({ c, i }) => {
    const [ch, , acct, prof, dev, mode, off] = c; const inp = LIVE.inputs.find((n) => n.ch === ch); const tr = /umgewandelt/.test(mode); const res = (mode.match(/(\d{3,4})p/) || [])[1];
    return { i, ch, acct, prof, dev, mode, off, tr, res, br: tr ? (res === '720' ? 2.8 : 3.8) : (inp ? inp.bitrate : 8), since: LIVE.NOW - (ADM.sinceMin[i] || 20) };
  });
}
const admViewersOf = (ch) => admViewers().filter((v) => v.ch === ch).length;
/** running recordings: those of the mockup account from the simulation, those of other accounts from the sample */
function admRecRunning() {
  if (admEmpty()) return 0;
  const mine = LIVE.profiles.reduce((n, p) => n + SIM.recordings(p.id).filter((r) => r.state === 'running').length, 0);
  return mine + LIVE.consumers.filter((c) => c[1] === 'recording' && c[2] !== LIVE.account.email).length;
}
/** planned recordings of the mockup account starting within min minutes */
function admRecSoon(min) { return admEmpty() ? 0 : LIVE.profiles.reduce((n, p) => n + SIM.upcoming(p.id).filter((u) => u.start - SIM.now() <= min && u.start >= SIM.now()).length, 0); }
/** active inputs as LA-01, LA-12 and LA-16 show them: recordings need an input (abSyncRecInputs of admin-b-2.js),
    and with the server default «Permanent» (LA-09) every channel on «Standard» is received permanently */
function admInputs() {
  if (admEmpty()) return [];
  if (typeof abSyncRecInputs === 'function') abSyncRecInputs();
  const R = LIVE.resources; const perm = LIVE.settings.reception.default === 'permanent';
  LIVE.channels.filter((c) => c.active && c.reception === 'default').forEach((c) => {
    let n = LIVE.inputs.find((x) => x.ch === c.id);
    if (perm) {
      if (!n) { const ok = SIM.chState(c) === 'ok'; const s = c.sources.find((q) => q.state !== 'fail') || c.sources[0]; n = { ch: c.id, source: s ? s.source : 'src-kabel', backup: 0, state: ok ? 'running' : 'reconnecting', attempt: [1, 5], reasons: ['permanent'], since: SIM.now(), viewers: 0, recordings: 0, bitrate: ok ? c.bitrate : 0, transcode: '–', buffer: '12 MB', timeshift: '–', tsPer: 0, recGB: 0, errors: ok ? null : 'alle Quellen fehlgeschlagen', byDefault: true }; LIVE.inputs.push(n); R.bandIn = Math.round((R.bandIn + n.bitrate) * 10) / 10; }
      else if (!n.reasons.includes('permanent')) { n.reasons.push('permanent'); n.permByDefault = true; }
    } else if (n && n.byDefault) { LIVE.inputs.splice(LIVE.inputs.indexOf(n), 1); R.bandIn = Math.max(0, Math.round((R.bandIn - n.bitrate) * 10) / 10); }
    else if (n && n.permByDefault) { n.reasons = n.reasons.filter((x) => x !== 'permanent'); delete n.permByDefault; }
  });
  return LIVE.inputs;
}
/** «100 GB ≈ 27.8 h HD»: hours of recordings at a bitrate (Mbit/s; 8 Mbit/s = 3.6 GB per hour) */
const admHours = (gb, mbit) => Math.round(gb / ((mbit || 8) * 3600 / 8 / 1000) * 10) / 10;

/* ---------- mockup states of the administration (bar «Zustände» → Verwaltung) ----------
   The pages of «Web Verwaltung B» read ui.abSto (storage) and ui.abEpg (EPG import); both point to LIVE_SIM. */
Object.defineProperty(ui, 'abSto', { configurable: true, get: () => SIM.flags().admStorage || 'normal', set: (v) => SIM.setFlag('admStorage', v === 'normal' ? '' : v) });
Object.defineProperty(ui, 'abEpg', { configurable: true, get: () => (SIM.flags().epgFail ? 'failed' : 'normal'), set: (v) => SIM.setFlag('epgFail', v === 'failed' || v === 'truncated') });
let admApplied = { sto: null, epg: null };
SIM.on((t) => {
  if (t !== 'change') return;
  const f = SIM.flags();
  if (f.admStorage !== admApplied.sto && typeof abStoApply === 'function') { admApplied.sto = f.admStorage; abStoApply(); }
  if (f.epgFail !== admApplied.epg && ACT.abEpgState) { admApplied.epg = f.epgFail; ACT.abEpgState(); }
});
