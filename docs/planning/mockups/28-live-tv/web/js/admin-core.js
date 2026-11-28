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
/** reason of a deferred permanent channel without the leading «Konnte nicht starten:» (04 T-17: deferred only when it cannot start) */
const admDefWhy = (n) => String((n && n.deferredReason) || '').replace(/^Konnte nicht starten:\s*/i, '');
/** short cause («Obergrenze der Eingangsströme erreicht») and the number of attempts (15 0.1: 2 attempts for 3sat) */
const admDefCause = (n) => admDefWhy(n).replace(/\s*\(.*$/, '').replace(/\.$/, '') || 'Ursache unbekannt';
const admDefTries = (n) => (n && n.attempts) || 2;
/** «zurückgestellt: konnte nicht starten: Obergrenze … erreicht (2 Versuche)», same wording on LA-01, LA-09, LA-12, LA-13 */
const admDefText = (n) => `zurückgestellt: konnte nicht starten: ${admDefCause(n)} (${admDefTries(n)} Versuche)`;
/** permanent and scheduled inputs occupy more than half of the inputs (15 3.9, 04 8.2): warning on LA-01, LA-09, LA-12 */
function admPermLoad() {
  const inp = admInputs().filter((n) => n.state !== 'deferred' && n.reasons.some((r) => r === 'permanent' || r === 'schedule'));
  const max = LIVE.limits.inputs; return inp.length > max / 2 ? { n: inp.length, max } : null;
}
const admPermText = (p) => `Dauerhaft und nach Zeitplan empfangene Sender belegen ${p.n} von ${p.max} Eingängen. Neue Zuschauer können abgewiesen werden, weil ein laufender Dauersender nicht weicht.`;
/* resting precautionary reception per source (04 T-17, 5.3; 15 Grundsatz 3): runtime behaviour, no setting.
   Pattern A: the source refuses a further connection; pattern B: it drops the older one (one-stream mode).
   Periods 30 minutes, 6 hours, 24 hours (n-th time). */
const ADM_REST = [
  { src: 'src-kabel', pattern: 'A', at: LIVE.at(0, '20:39'), until: LIVE.at(0, '21:09'), n: 1 },
  { src: 'src-xtream', pattern: 'B', at: LIVE.at(0, '20:10'), until: LIVE.at(1, '02:10'), n: 2 },
];
const admRest = (id) => { const r = ADM_REST.find((x) => x.src === id); return r && !admEmpty() && SIM.now() < r.until && LIVE.sources.some((s) => s.id === id) ? r : null; };
const admUntil = (t) => (LU.dayOf(t) === LU.dayOf(SIM.now()) ? fmt.time(t) : `morgen ${fmt.time(t)}`);
const admSrcName = (id) => (LIVE.sources.find((s) => s.id === id) || { name: id }).name;
/** «ruht bis 21:09 · Muster A: Quelle lehnte um 20:39 eine weitere Verbindung ab» (LA-09, LA-03) */
const admRestText = (r) => (r.pattern === 'A' ? `ruht bis ${admUntil(r.until)} · Muster A: Quelle lehnte um ${fmt.time(r.at)} eine weitere Verbindung ab` : `ruht bis ${admUntil(r.until)} · Muster B: Quelle trennte die ältere Verbindung; Ein-Strom-Betrieb`);
/** «… ruht bis 21:09 · Muster A (lehnte eine weitere Verbindung ab), 1. Mal» (LA-13) */
const admRestDiag = (r) => `vorsorglicher Empfang ruht bis ${admUntil(r.until)} · ${r.pattern === 'A' ? 'Muster A (lehnte eine weitere Verbindung ab)' : 'Muster B (trennte die ältere Verbindung), Ein-Strom-Betrieb'}, ${r.n}. Mal`;
/** state of the FFmpeg lock as one line (15 3.1: «FFmpeg-Sperre: aktiv» or the warning) */
const admLockLine = () => (SIM.flags().noContain ? admNoContain() : `<p class="muted aa-small">FFmpeg-Sperre: aktiv. Umwandlung und Probe für Live-TV laufen nur in der Sperre.</p>`);
/** recordings on one input only as a number from 2, otherwise «Aufnahmen laufen» (12 SM-45) */
const admRecWord = (n) => (n === 1 ? 'Aufnahmen laufen' : `${fmt.num(n)} Aufnahmen`);
const admRecCell = (n) => (n === 1 ? 'laufen' : String(n));
/** mockup state «Eindämmung nicht verfügbar» (04 T-27, 12 F-1): the same warning on LA-01, LA-09, LA-13, LA-14 and LA-15 */
const ADM_NOCONTAIN = 'Eindämmung nicht verfügbar: Umwandlung für Live-TV aus';
function admNoContain() {
  if (!SIM.flags().noContain) return '';
  const names = LIVE.channels.filter((c) => LU.NEEDS_CONVERT.has(c.id)).map((c) => c.name).join(', ') || 'keine';
  return notice('bad', ADM_NOCONTAIN, `Der Server kann FFmpeg auf diesem Host nicht in eine Sperre setzen (Landlock und Filter für Systemaufrufe). Deshalb laufen für Live-TV keine Umwandlung und keine Probe über FFmpeg. Der Direktweg bleibt: Sender, die ein Gerät im Original abspielt, laufen weiter. Sender, die eine Umwandlung brauchen (hier: ${esc(names)}), spielen nicht. Einen Schalter «Trotzdem erlauben» gibt es nicht; ob es ihn geben soll, ist die offene Frage F-1 in Kapitel 12 (Sicherheit).`);
}

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
