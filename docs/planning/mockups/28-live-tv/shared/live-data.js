/* Konzept 28 «Live-TV»: sample data shared by all three mockup clients (web, TV, phone).
   Everything here is invented sample data (14-oberflaeche.md 0.1, 15-administration.md 0.1). No real
   stream addresses, no credentials: every address uses an example.* domain and is masked.

   Time model: every point in time is a number of MINUTES since Saturday 28 November 2026, 00:00 local
   time (Europe/Zurich). Day 0 = Sa 28.11., day 1 = So 29.11., day -6 = So 22.11. The mockup clock starts
   at 20:41 (= 1241). Format with LIVE_UI.fmt.* (shared/live-ui.js), never with Date in local time.

   Plain script: defines window.LIVE (also loadable with node for checks). Do not change from page files;
   mutable state lives in LIVE_SIM (shared/live-sim.js). */
'use strict';
(function (g) {
  const DAY = 1440;
  const hm = (s) => { const [h, m] = s.split(':').map(Number); return h * 60 + m; };
  const at = (day, s) => day * DAY + hm(s);

  /* ---------- account and profiles ---------- */
  const account = {
    id: 'acc-keller', name: 'Familie Keller', email: 'familie.keller@example.ch',
    quotaGB: 200, usedGB: 142, // start value at 20:41; LIVE_SIM.usage() computes it from the recordings
    rights: { liveTv: true, recordings: true, timeshift: true }, // cap set by the server admin (LA-22)
  };
  const profiles = [
    /* usage is not stored: LIVE_SIM.usage() adds up the recordings (Anna 82 GB, Luca 56.7 GB, Mia 3.3 GB at 20:41) */
    { id: 'anna', name: 'Anna', primary: true, age: null, accent: 'a1', rights: { liveTv: true, recordings: true, timeshift: true } },
    { id: 'luca', name: 'Luca', primary: false, age: 16, accent: 'a2', rights: { liveTv: true, recordings: true, timeshift: true } },
    { id: 'mia', name: 'Mia', primary: false, age: 6, accent: 'a3', rights: { liveTv: true, recordings: true, timeshift: false } },
  ];
  const timeshiftMaxMin = 120;

  /* ---------- groups, countries, languages, categories ---------- */
  const groups = [
    { id: 'ch', name: 'Schweiz', origin: 'source', source: 'src-kabel' },
    { id: 'de', name: 'Deutschland', origin: 'source', source: 'src-kabel' },
    { id: 'at', name: 'Österreich', origin: 'source', source: 'src-xtream' },
    { id: 'news', name: 'Nachrichten', origin: 'own' },
    { id: 'sport', name: 'Sport', origin: 'own' },
    { id: 'kids', name: 'Kinder', origin: 'own' },
    { id: 'culture', name: 'Kultur', origin: 'own' },
    { id: 'regional', name: 'Regional', origin: 'source', source: 'src-region' },
    { id: 'full', name: 'Vollprogramme', origin: 'own' },
  ];
  const countries = { CH: 'Schweiz', DE: 'Deutschland', AT: 'Österreich' };
  const languages = { de: 'Deutsch', fr: 'Französisch', it: 'Italienisch' };
  const categories = ['Vollprogramm', 'Nachrichten', 'Sport', 'Film und Serie', 'Dokumentation', 'Kinder', 'Musik', 'Kultur', 'Regional', 'Unterhaltung', 'Sonstiges'];

  /* ---------- sources (administration) ---------- */
  const sources = [
    { id: 'src-kabel', name: 'Kabelanbieter Schweiz', kind: 'm3u-url', address: 'https://iptv.example.net/playlist.m3u8?token=••••', host: 'iptv.example.net',
      active: 398, total: 412, groups: 14, gone: 1, interval: 24, last: at(0, '06:00'), lastOk: true, next: at(1, '06:00'), state: 'ok', localNet: false, offerNew: false },
    { id: 'src-xtream', name: 'Xtream Beispielanbieter', kind: 'xtream', address: 'http://xtream.example.com:8080', host: 'xtream.example.com', user: 'demo',
      active: 120, total: 9874, groups: 212, gone: 0, interval: 12, last: at(0, '05:00'), lastOk: false, lastError: 'Server antwortet 503', next: at(0, '17:00'), state: 'warn', localNet: false, offerNew: false },
    { id: 'src-region', name: 'Senderliste Region', kind: 'm3u-file', file: 'senderliste-region.m3u', fileSize: '1.2 MB', uploaded: at(-8, '14:20'),
      active: 64, total: 66, groups: 3, gone: 0, interval: 0, last: at(-8, '14:20'), lastOk: true, next: null, state: 'ok', localNet: false, offerNew: false },
    { id: 'src-mc', name: 'Heimnetz Multicast', kind: 'single', streams: ['udp://@239.10.0.1:5000', 'udp://@239.10.0.2:5000'],
      active: 2, total: 2, groups: 0, gone: 0, interval: 0, last: null, lastOk: true, next: null, state: 'ok', localNet: true, fixedTargets: 2, offerNew: false },
  ];
  /* import runs per source: [runId, at, result, new, changed, gone, note] */
  const importRuns = {
    'src-kabel': [
      ['run-k5', at(0, '06:00'), 'ok', 3, 12, 1, ''], ['run-k4', at(-1, '06:00'), 'ok', 0, 4, 0, ''], ['run-k3', at(-2, '06:00'), 'ok', 1, 2, 0, ''],
      ['run-k2', at(-3, '06:00'), 'ok', 0, 0, 0, ''], ['run-k1', at(-4, '06:00'), 'incomplete', 0, 0, 0, 'Die neue Liste hatte 61 % weniger Sender. Nicht übernommen.'],
    ],
    'src-xtream': [
      ['run-x5', at(0, '05:00'), 'failed', 0, 0, 0, 'Server antwortet 503'], ['run-x4', at(-1, '17:00'), 'ok', 12, 40, 3, ''], ['run-x3', at(-1, '05:00'), 'ok', 0, 8, 0, ''],
      ['run-x2', at(-2, '17:00'), 'ok', 2, 5, 1, ''], ['run-x1', at(-2, '05:00'), 'ok', 0, 3, 0, ''],
    ],
    'src-region': [['run-r1', at(-8, '14:20'), 'ok', 66, 0, 0, 'Erster Import']],
    'src-mc': [],
  };
  /* result of the latest import of «Kabelanbieter Schweiz» (LA-04) */
  const importResult = {
    source: 'src-kabel', run: 'run-k5', at: at(0, '06:00'),
    counts: { new: 3, changed: 12, gone: 1, dup: 2, bad: 4, same: 396 },
    new: [['Blue Zoom', 'Schweiz', 41, true, 'BlueZoom.ch'], ['Kanal 9', 'Regional', null, false, ''], ['CNN International', 'Nachrichten', 120, true, 'CNNInt.us']],
    changed: [['SRF 1', 'Name in der Quelle: «SRF1 HD» statt «SRF 1 HD»', 'Dein Name «SRF 1» bleibt'], ['3sat', 'Adresse geändert', 'wirkt sofort'], ['arte', 'Logo der Quelle geändert', 'Dein Logo bleibt']],
    gone: [['TV24', 'seit heute 06:00', 4, 1]],
    dup: [['SRF1 HD CH', 'SRF 1', 'gleiche EPG-Kennung «SRF1.ch»'], ['ZDF HD', 'ZDF', 'Name gleich nach Bereinigung']],
    bad: [[1204, 'Adresse fehlt'], [2210, 'ungültige Angabe «tvg-chno=abc»'], [3012, 'Adresse fehlt'], [4100, 'Zeile unvollständig']],
  };

  /* ---------- channels ----------
     epg: 'ok' | 'none' (no mapping) | 'partial'; reception: 'default' | 'permanent' | 'auto' | 'ondemand' | 'schedule'
     state: 'ok' | 'disturbed' | 'gone'. Sources in rank order (first = main, rest = backups). */
  const ch = (id, no, name, country, langs, grp, category, extra) => Object.assign({
    id, no, name, slug: id, country, langs, groups: grp, category, epg: 'ok', epgKey: name.replace(/\s/g, '') + '.' + country.toLowerCase(),
    reception: 'default', preload: true, state: 'ok', since: null, bitrate: 8.1, video: 'H.264 · 1920 × 1080 · 50 Halbbilder/s', audio: ['Deutsch (AAC Stereo)'], subs: [],
    sources: [{ source: 'src-kabel', name: name + ' HD', url: 'https://iptv.example.net/live/••••/' + id + '.m3u8', state: 'ok' }],
    favCount: 0, active: true,
  }, extra || {});
  const XT = (id) => ({ source: 'src-xtream', name: id.toUpperCase() + ' HD', url: 'http://xtream.example.com:8080/live/••••/••••/' + (1000 + id.length * 37) + '.ts', state: 'ok' });
  const channels = [
    ch('srf1', 1, 'SRF 1', 'CH', ['de'], ['ch', 'full'], 'Vollprogramm', { reception: 'permanent', audio: ['Deutsch (AAC Stereo)', 'Deutsch, Audiodeskription (AAC)'], subs: ['Deutsch (Videotext)'], sources: [{ source: 'src-kabel', name: 'SRF1 HD', url: 'https://iptv.example.net/live/••••/srf1.m3u8', state: 'ok' }, XT('srf1')], favCount: 12 }),
    ch('srfzwei', 2, 'SRF zwei', 'CH', ['de'], ['ch', 'sport'], 'Sport', { audio: ['Deutsch (AAC Stereo)', 'Deutsch (AC-3 5.1)'], sources: [{ source: 'src-kabel', name: 'SRF zwei HD', url: 'https://iptv.example.net/live/••••/srfzwei.m3u8', state: 'ok' }, XT('srfzwei')], favCount: 9, bitrate: 7.4 }),
    ch('srfinfo', 3, 'SRF info', 'CH', ['de'], ['ch', 'news'], 'Nachrichten', { preloaded: true, bitrate: 5.2 }),
    ch('rts1', 4, 'RTS 1', 'CH', ['fr'], ['ch'], 'Vollprogramm', { audio: ['Französisch (AAC Stereo)'], epgKey: 'RTSUn.ch' }),
    ch('rsila1', 5, 'RSI LA 1', 'CH', ['it'], ['ch'], 'Vollprogramm', { audio: ['Italienisch (AAC Stereo)'], epgKey: 'RSILa1.ch' }),
    ch('telezueri', 7, 'TeleZüri', 'CH', ['de'], ['ch', 'regional'], 'Regional', { epg: 'none', epgKey: null, bitrate: 4.0, video: 'H.264 · 1280 × 720 · 50 Bilder/s', sources: [{ source: 'src-region', name: 'TeleZueri', url: 'https://region.example.net/tz/••••.m3u8', state: 'ok' }] }),
    ch('daserste', 10, 'Das Erste', 'DE', ['de'], ['de', 'full'], 'Vollprogramm', { reception: 'schedule', schedule: 'Abends', subs: ['Deutsch (Videotext)'], favCount: 7 }),
    ch('zdf', 11, 'ZDF', 'DE', ['de'], ['de', 'full'], 'Vollprogramm', { subs: ['Deutsch (DVB)'], sources: [{ source: 'src-kabel', name: 'ZDF HD', url: 'https://iptv.example.net/live/••••/zdf.m3u8', state: 'ok' }, XT('zdf')], favCount: 8 }),
    ch('3sat', 12, '3sat', 'DE', ['de'], ['de', 'culture'], 'Kultur', { reception: 'permanent' }),
    ch('arte', 13, 'arte', 'DE', ['de', 'fr'], ['de', 'culture'], 'Kultur', { audio: ['Deutsch (AAC Stereo)', 'Französisch (AAC Stereo)'], favCount: 4 }),
    ch('zdfneo', 14, 'ZDFneo', 'DE', ['de'], ['de'], 'Unterhaltung'),
    ch('zdfinfo', 15, 'ZDFinfo', 'DE', ['de'], ['de', 'news'], 'Dokumentation', { state: 'disturbed', since: at(0, '20:12'), sources: [{ source: 'src-kabel', name: 'ZDFinfo HD', url: 'https://iptv.example.net/live/••••/zdfinfo.m3u8', state: 'fail' }, Object.assign(XT('zdfinfo'), { state: 'fail' })] }),
    ch('tagesschau24', 16, 'tagesschau24', 'DE', ['de'], ['de', 'news'], 'Nachrichten', { bitrate: 5.0 }),
    ch('phoenix', 17, 'phoenix', 'DE', ['de'], ['de', 'news'], 'Nachrichten', { epg: 'partial', epgNote: 'Daten nur bis 20:00' }),
    ch('kika', 18, 'KiKA', 'DE', ['de'], ['de', 'kids'], 'Kinder', { epgNote: 'Sendeschluss 21:00', favCount: 2 }),
    ch('orf2', 30, 'ORF 2', 'AT', ['de'], ['at'], 'Vollprogramm', { sources: [XT('orf2')] }),
  ];
  /* entries only the administration sees: three new channels of the last import (inactive until switched on)
     and three duplicates of existing channels from other sources (inactive, `dupOf`). Viewer lists, the grid
     and every viewer count skip them (LIVE_SIM.channels() takes only active channels without dupOf). */
  const adm = (id, no, name, slug, src, o) => ch(id, no, name, 'CH', ['de'], [], 'Vollprogramm', Object.assign({ epg: 'none', epgKey: null, sources: [src], active: false, nameInSource: name }, o || {}));
  const xsrc = (n, u) => ({ source: 'src-xtream', name: n, url: 'http://xtream.example.com:8080/live/••••/••••/' + u + '.ts', state: 'ok' });
  channels.push(
    adm('x-srf1hd', null, 'SRF1 HD CH (Xtream)', 'srf1', xsrc('SRF1 HD CH', 2201), { epgKey: 'SRF1.ch', dupOf: 'srf1' }),
    adm('x-zdfhd', null, 'ZDF HD (Xtream)', 'zdf', xsrc('ZDF HD', 2202), { country: 'DE', groups: ['de'], dupOf: 'zdf' }),
    adm('x-3sat', null, '3sat HD (Senderliste Region)', '3sat', { source: 'src-region', name: '3sat HD', url: 'https://region.example.net/3sat/••••.m3u8', state: 'ok' }, { country: 'DE', category: 'Kultur', dupOf: '3sat' }),
    adm('bluezoom', 41, 'Blue Zoom', 'bluezoom', { source: 'src-kabel', name: 'Blue Zoom', url: 'https://iptv.example.net/live/••••/bluezoom.m3u8', state: 'ok' }, { isNew: true, groups: ['ch'], epgKey: 'BlueZoom.ch' }),
    adm('kanal9', null, 'Kanal 9', 'kanal9', { source: 'src-kabel', name: 'Kanal 9', url: 'https://iptv.example.net/live/••••/kanal9.m3u8', state: 'ok' }, { isNew: true, groups: ['regional'], category: 'Regional' }),
    adm('cnn', 120, 'CNN International', 'cnn', { source: 'src-kabel', name: 'CNN International', url: 'https://iptv.example.net/live/••••/cnn.m3u8', state: 'ok' }, { isNew: true, country: 'DE', groups: ['news'], category: 'Nachrichten', epgKey: 'CNNInt.us' }),
  );
  /* channel that vanished from «Kabelanbieter Schweiz» this morning: not offered to viewers any more */
  const goneChannels = [ch('tv24', 24, 'TV24', 'CH', ['de'], ['ch'], 'Unterhaltung', { state: 'gone', since: at(0, '06:00'), active: false })];

  /* ---------- programme guide ----------
     A programme: { id, ch, start, end, title, sub, genre, desc, series, season, episode, repeat, firstRun,
                    flags[], img (file in shared/img/lib or null), year, country, director, cast } */
  const L = (n, t) => ({ img: 'b-' + n + '.webp', libTitle: t }); // stand-in picture from the phone mockup library
  const T = (time, title, genre, o) => Object.assign({ time, title, genre }, o || {});
  /* daily template per channel (times of day, repeated every day; explicit end only where a gap follows) */
  const day = {
    srf1: [T('00:30', 'Tagesschau Nacht', 'Nachrichten'), T('00:45', 'Der Bestatter', 'Serie', { repeat: true, series: 'bestatter' }), T('01:40', 'Programmhinweise', 'Sonstiges'), T('06:00', 'Tagesschau', 'Nachrichten'), T('06:15', 'Gesichter & Geschichten', 'Magazin', { repeat: true }), T('09:00', 'SRF bi de Lüt', 'Unterhaltung', { repeat: true }), T('11:00', 'Gesundheit heute', 'Magazin'), T('12:45', 'Tagesschau', 'Nachrichten'), T('13:10', 'Mona Mittendrin', 'Dokumentation'), T('15:00', 'Der Bergdoktor', 'Serie', { repeat: true, series: 'bergdoktor' }), T('16:40', 'Mini Beiz, dini Beiz', 'Unterhaltung'), T('17:45', 'Gesichter & Geschichten', 'Magazin'), T('18:40', 'glanz & gloria', 'Magazin'), T('19:00', 'Schweiz aktuell', 'Nachrichten'), T('19:30', 'Tagesschau', 'Nachrichten', { flags: ['Untertitel'], series: 'tagesschau' }), T('19:55', 'Meteo', 'Nachrichten'), T('20:05', 'DOK', 'Dokumentation'), T('21:00', 'Kassensturz', 'Magazin', { series: 'kassensturz' }), T('21:50', '10 vor 10', 'Nachrichten'), T('22:25', 'Arena', 'Politik', { series: 'arena' }), T('23:45', 'Tagesschau Nacht', 'Nachrichten')],
    srfzwei: [T('00:35', 'Sportlounge', 'Sport', { repeat: true }), T('02:00', 'Programmhinweise', 'Sonstiges'), T('06:00', 'Kinderprogramm', 'Kinder'), T('10:00', 'Ski alpin: Weltcup Abfahrt', 'Sport'), T('12:30', 'Biathlon Weltcup', 'Sport'), T('15:00', 'Fussball: Super League', 'Sport'), T('18:00', 'Sport aktuell', 'Sport'), T('18:30', 'Young Sheldon', 'Serie', L(45, 'Young Sheldon')), T('19:00', 'Young Sheldon', 'Serie', L(45, 'Young Sheldon')), T('19:30', 'Sportpanorama', 'Sport'), T('20:00', 'Wonder Man', 'Serie', L(46, 'Wonder Man')), T('21:00', 'Resident Evil', 'Spielfilm', Object.assign(L(21, 'Resident Evil'), { year: 2026 })), T('22:45', 'Sport aktuell', 'Sport'), T('23:15', 'Cube', 'Spielfilm', Object.assign(L(52, 'Cube'), { year: 1998 }))],
    srfinfo: 'hourly',
    rts1: [T('00:20', 'Le 19h30', 'Nachrichten', { repeat: true }), T('01:00', 'Programmhinweise', 'Sonstiges'), T('06:30', 'RTSinfo', 'Nachrichten'), T('09:00', 'Top Models', 'Serie'), T('12:45', 'Le 12h45', 'Nachrichten'), T('13:25', 'Faut pas croire', 'Magazin'), T('15:00', 'Columbo', 'Serie', { repeat: true }), T('18:00', 'Météo régionale', 'Nachrichten'), T('19:30', 'Le 19h30', 'Nachrichten'), T('20:10', 'Météo', 'Nachrichten'), T('20:20', 'Mise au point', 'Magazin'), T('21:05', 'Tulsa King', 'Serie', L(17, 'Tulsa King')), T('22:40', 'Ted', 'Serie', L(40, 'Ted')), T('23:30', 'Le 19h30', 'Nachrichten', { repeat: true })],
    rsila1: [T('00:40', 'Telegiornale notte', 'Nachrichten'), T('01:10', 'Programmhinweise', 'Sonstiges'), T('06:30', 'Telegiornale', 'Nachrichten'), T('09:00', 'Paganini', 'Dokumentation'), T('12:30', 'Telegiornale', 'Nachrichten'), T('13:00', 'Linea rossa', 'Magazin'), T('16:00', 'Il commissario', 'Serie', { repeat: true }), T('18:00', 'Il Quotidiano', 'Nachrichten'), T('20:00', 'Telegiornale', 'Nachrichten'), T('20:40', 'Yellowstone', 'Serie', L(49, 'Yellowstone')), T('22:20', 'Zero Day', 'Serie', L(42, 'Zero Day')), T('23:30', 'Telegiornale notte', 'Nachrichten')],
    telezueri: [],
    daserste: [T('00:05', 'Tagesthemen', 'Nachrichten', { repeat: true }), T('00:35', 'Nachtkonzert', 'Musik'), T('05:30', 'Morgenmagazin', 'Nachrichten'), T('09:00', 'Tagesschau', 'Nachrichten'), T('09:05', 'Rote Rosen', 'Serie'), T('12:00', 'Tagesschau', 'Nachrichten'), T('12:15', 'Mittagsmagazin', 'Nachrichten'), T('14:00', 'Sportschau', 'Sport'), T('17:50', 'Gefragt – Gejagt', 'Unterhaltung'), T('18:50', 'In aller Freundschaft', 'Serie'), T('19:50', 'Wetter vor acht', 'Nachrichten'), T('20:00', 'Tagesschau', 'Nachrichten', { flags: ['Untertitel'] }), T('20:15', 'Die Bergretter', 'Serie'), T('21:45', 'Tagesthemen', 'Nachrichten'), T('22:15', 'Polizeiruf 110', 'Krimi', { repeat: true }), T('23:45', 'Tagesthemen', 'Nachrichten')],
    zdf: [T('00:55', 'heute', 'Nachrichten'), T('01:05', 'Der Kommissar und das Meer', 'Krimi', { repeat: true }), T('02:35', 'Programmhinweise', 'Sonstiges'), T('05:30', 'ZDF-Morgenmagazin', 'Nachrichten'), T('09:00', 'heute', 'Nachrichten'), T('09:05', 'Volle Kanne', 'Magazin'), T('12:00', 'heute', 'Nachrichten'), T('12:10', 'drehscheibe', 'Magazin'), T('14:00', 'Die Rosenheim-Cops', 'Serie', { repeat: true }), T('17:00', 'heute', 'Nachrichten'), T('17:10', 'hallo deutschland', 'Magazin'), T('18:00', 'SOKO Wismar', 'Krimi'), T('19:00', 'heute', 'Nachrichten'), T('19:25', 'Der Bergdoktor', 'Serie', { repeat: true, series: 'bergdoktor' }), T('20:15', 'Der Staatsanwalt', 'Krimi'), T('21:45', 'heute-journal', 'Nachrichten'), T('22:15', 'Markus Lanz', 'Talk'), T('23:30', 'heute journal update', 'Nachrichten')],
    '3sat': [T('00:30', 'Kulturzeit', 'Kultur', { repeat: true }), T('01:10', 'Programmhinweise', 'Sonstiges'), T('06:00', 'ZIB', 'Nachrichten'), T('07:00', 'Wilde Schweiz', 'Dokumentation', { repeat: true }), T('11:30', 'Alpenpanorama', 'Dokumentation'), T('13:15', 'Länder – Menschen – Abenteuer', 'Dokumentation'), T('17:00', 'Kulturzeit', 'Kultur', { repeat: true }), T('18:30', 'nano', 'Wissen'), T('19:00', 'heute', 'Nachrichten'), T('19:20', 'Kulturzeit', 'Kultur'), T('20:00', 'Tagesschau', 'Nachrichten'), T('20:15', 'Wissenschaftsdoku', 'Dokumentation'), T('21:45', 'ZIB 2', 'Nachrichten'), T('22:25', 'Kabarett', 'Kultur')],
    arte: [T('00:10', 'Square Idée', 'Kultur', { repeat: true }), T('01:00', 'Programmhinweise', 'Sonstiges'), T('06:00', 'Stadt Land Kunst', 'Dokumentation', { repeat: true }), T('09:30', 'GEO Reportage', 'Dokumentation'), T('13:00', 'Stadt Land Kunst', 'Dokumentation'), T('16:00', 'Wildes Europa', 'Dokumentation'), T('18:30', 'Mit offenen Karten', 'Magazin'), T('19:20', 'ARTE Journal', 'Nachrichten'), T('19:40', 'ARTE Reportage', 'Dokumentation'), T('20:15', 'Das Experiment', 'Spielfilm', Object.assign(L(27, 'Das Experiment'), { year: 2001, country: 'Deutschland' })), T('22:15', 'Kurzschluss', 'Kultur'), T('23:15', 'Tracks', 'Musik')],
    zdfneo: [T('00:30', 'Dexter: Wiedererwachen', 'Serie', Object.assign(L(14, 'Dexter: Wiedererwachen'), { repeat: true })), T('02:00', 'Programmhinweise', 'Sonstiges'), T('06:00', 'Bares für Rares', 'Unterhaltung', { repeat: true }), T('10:00', 'Inga Lindström', 'Spielfilm', { repeat: true }), T('13:30', 'Der Bergdoktor', 'Serie', { repeat: true, series: 'bergdoktor' }), T('16:00', 'Bares für Rares', 'Unterhaltung'), T('18:00', 'Young Sherlock', 'Serie', L(44, 'Young Sherlock')), T('19:30', 'Your Friends & Neighbors', 'Serie', L(41, 'Your Friends & Neighbors')), T('20:15', 'Dexter: Wiedererwachen', 'Serie', L(14, 'Dexter: Wiedererwachen')), T('22:00', 'Y: The Last Man', 'Serie', L(48, 'Y: The Last Man')), T('23:30', 'MobLand: Familie bis aufs Blut', 'Serie', L(18, 'MobLand: Familie bis aufs Blut'))],
    zdfinfo: [T('00:15', 'Terra X', 'Dokumentation', { repeat: true }), T('03:00', 'Programmhinweise', 'Sonstiges'), T('06:00', 'ZDFinfo Doku', 'Dokumentation'), T('10:00', 'Alexander der Große: Wie er ein Gott wurde', 'Dokumentation', L(106, 'Alexander der Große: Wie er ein Gott wurde')), T('13:00', 'Border Control: Türkei – Grenzschützer im Einsatz', 'Dokumentation', L(15, 'Border Control: Türkei - Grenzschützer im Einsatz')), T('16:00', 'Goldrausch: Hoffman Family Gold', 'Dokumentation', L(35, 'Goldrausch: Hoffman Family Gold')), T('19:30', 'Terra X', 'Dokumentation'), T('20:15', 'Alone – Überleben in der Wildnis', 'Dokumentation', L(109, 'Alone - Überleben in der Wildnis')), T('22:30', 'aka Charlie Sheen', 'Dokumentation', L(105, 'aka Charlie Sheen'))],
    tagesschau24: 'hourly24',
    phoenix: [T('06:00', 'phoenix vor ort', 'Nachrichten'), T('09:00', 'phoenix plus', 'Dokumentation'), T('12:00', 'phoenix vor ort', 'Nachrichten'), T('14:00', 'phoenix plus', 'Dokumentation'), T('17:30', 'phoenix runde', 'Talk'), T('18:30', 'phoenix plus', 'Dokumentation'), T('19:30', 'phoenix der tag', 'Nachrichten', { end: '20:00' })],
    kika: [T('06:00', 'Sesamstrasse', 'Kinder'), T('07:30', 'Peppa Wutz', 'Kinder'), T('09:00', 'Löwenzahn', 'Kinder', { series: 'loewenzahn' }), T('11:00', 'Schloss Einstein', 'Kinder'), T('13:00', 'Die Pfefferkörner', 'Kinder'), T('15:00', 'Wissen macht Ah!', 'Kinder'), T('17:00', 'Die Schule der magischen Tiere', 'Kinder'), T('18:50', 'Unser Sandmännchen', 'Kinder'), T('19:00', 'Checker Julian', 'Kinder'), T('19:30', 'logo!', 'Nachrichten'), T('19:50', 'Unser Sandmännchen', 'Kinder'), T('20:00', 'Checker Tobi', 'Kinder', { series: 'checker-tobi' }), T('20:30', 'Die Sendung mit der Maus', 'Kinder', { series: 'maus', end: '21:00' })],
    orf2: [T('00:20', 'ZIB 3', 'Nachrichten'), T('00:40', 'Programmhinweise', 'Sonstiges'), T('06:00', 'Guten Morgen Österreich', 'Magazin'), T('09:00', 'ZIB', 'Nachrichten'), T('09:05', 'Klingendes Österreich', 'Musik'), T('12:00', 'ZIB', 'Nachrichten'), T('13:00', 'Mittag in Österreich', 'Magazin'), T('17:00', 'ZIB', 'Nachrichten'), T('18:30', 'Bundesland heute', 'Nachrichten'), T('19:30', 'ZIB 1', 'Nachrichten'), T('20:15', 'Totenfrau', 'Serie', L(50, 'Totenfrau')), T('21:55', 'ZIB 2', 'Nachrichten'), T('22:25', 'Dok 1', 'Dokumentation'), T('23:20', 'Universum', 'Dokumentation')],
  };
  /* evenings and special slots that override the template: [day, channel, from, until, [items]] */
  const tatortStille = { title: 'Tatort', sub: 'Die Stille nach dem Sturm', genre: 'Krimi', series: 'tatort', firstRun: true, flags: ['Untertitel', 'Audiodeskription', '16:9'],
    desc: 'Nach einem Unwetter liegt ein Bergdorf von der Welt abgeschnitten. Als der Strom zurückkehrt, findet man den Gemeindepräsidenten tot im Feuerwehrdepot. Die Ermittler müssen herausfinden, wer die Stille genutzt hat.' };
  const over = [
    [0, 'srf1', '19:00', '24:00', [T('19:00', 'Schweiz aktuell', 'Nachrichten'), T('19:30', 'Tagesschau', 'Nachrichten', { flags: ['Untertitel'], series: 'tagesschau' }), T('19:55', 'Meteo', 'Nachrichten'), T('20:05', 'Wort zum Sonntag', 'Religion'),
      T('20:10', 'Happy Day', 'Show', { end: '22:00', flags: ['Untertitel'], desc: 'Röbi Koller erfüllt Herzenswünsche: Eine Familie aus dem Emmental bekommt nach einem Brand ihr Zuhause zurück, und ein Grossvater trifft nach 40 Jahren seinen Jugendfreund wieder.' }),
      T('22:00', 'Tagesschau Nacht', 'Nachrichten'), T('22:15', 'Sportaktuell', 'Sport'),
      T('22:40', 'Der Bestatter', 'Krimi', { sub: 'Das letzte Hemd', series: 'bestatter', season: 6, episode: 3, end: '23:35', flags: ['Untertitel', '16:9'], year: 2019, country: 'Schweiz', desc: 'Luc Conrad bereitet einen Verstorbenen für die Beerdigung vor und entdeckt am Hemd einen Hinweis, der nicht zur Todesursache passt.' }),
      T('23:35', 'Tagesschau Nacht', 'Nachrichten')]],
    [0, 'srfzwei', '18:30', '24:00', [T('18:30', 'Eishockey: Vorschau', 'Sport'), T('19:00', 'Eishockey National League: ZSC Lions – HC Davos', 'Sport', { live: true, end: '21:45', desc: 'Live aus der Swiss Life Arena in Zürich. Der Leader empfängt den Rekordmeister.' }),
      T('21:45', 'Sport aktuell', 'Sport'), T('22:15', 'Der Marsianer', 'Spielfilm', { end: '00:35', year: 2015, country: 'USA', director: 'Ridley Scott', flags: ['Zweikanalton', '16:9'], desc: 'Ein Astronaut wird nach einem Sturm auf dem Mars für tot gehalten und zurückgelassen. Mit Wissen und Witz versucht er zu überleben, bis Hilfe kommt.' })]],
    [0, 'daserste', '19:50', '24:00', [T('19:50', 'Wetter vor acht', 'Nachrichten'), T('20:00', 'Tagesschau', 'Nachrichten', { flags: ['Untertitel'] }),
      T('20:15', 'Klein gegen Gross', 'Show', { end: '23:15', desc: 'Kinder treten in erstaunlichen Disziplinen gegen Prominente an.' }), T('23:15', 'Tagesthemen', 'Nachrichten')]],
    [0, 'zdf', '19:00', '24:00', [T('19:00', 'heute', 'Nachrichten'), T('19:25', 'Der Bergdoktor', 'Serie', { repeat: true, series: 'bergdoktor', sub: 'Vertrauensfrage' }),
      T('20:15', 'Ein starkes Team', 'Krimi', { sub: 'Tödliche Ernte', series: 'starkes-team', end: '21:45', flags: ['Untertitel', 'Audiodeskription', '16:9'], year: 2026, country: 'Deutschland',
        desc: 'Auf einem Spargelhof im Brandenburger Umland wird ein Erntehelfer tot aufgefunden. Otto Garber und Linett Wachow stossen auf ein Netz aus Abhängigkeit und Schweigen.' }),
      T('21:45', 'heute-journal', 'Nachrichten'), T('22:00', 'Der Kommissar und das Meer', 'Krimi', { sub: 'Nebel über Gotland' }),
      T('23:30', 'das aktuelle sportstudio', 'Sport', { end: '00:55', desc: 'Fussball-Bundesliga: alle Spiele des Tages, Gäste im Studio und die Torwand.' })]],
    [0, '3sat', '19:20', '24:00', [T('19:20', 'Wilde Schweiz', 'Dokumentation', { sub: 'Der Jura' }), T('20:15', 'Kabarett aus der Schweiz', 'Kultur'), T('21:45', 'Kulturzeit extra', 'Kultur'), T('22:30', 'ZIB 2', 'Nachrichten')]],
    [0, 'arte', '19:20', '24:00', [T('19:20', 'ARTE Journal', 'Nachrichten'), T('19:40', 'Die Alpen von oben', 'Dokumentation', { end: '21:15', desc: 'Ein Flug über die Alpen von Slowenien bis nach Frankreich.' }), T('21:15', 'Tracks', 'Musik'), T('22:00', 'Das Experiment', 'Spielfilm', Object.assign(L(27, 'Das Experiment'), { year: 2001, country: 'Deutschland', director: 'Oliver Hirschbiegel' }))]],
    [1, 'srf1', '20:05', '22:00', [Object.assign(T('20:05', 'Tatort', 'Krimi'), tatortStille, { end: '21:35' }), T('21:35', 'Tagesschau Nacht', 'Nachrichten')]],
    [1, 'daserste', '20:15', '21:45', [Object.assign(T('20:15', 'Tatort', 'Krimi'), tatortStille, { end: '21:45' })]],
    [1, 'zdf', '20:15', '21:45', [T('20:15', 'Nur bis Weihnachten', 'Spielfilm', Object.assign(L(20, 'Nur bis Weihnachten'), { end: '21:45', year: 2026 }))]],
    [1, 'orf2', '20:15', '21:55', [Object.assign(T('20:15', 'Tatort', 'Krimi'), tatortStille, { end: '21:55' })]],
    [1, 'srfzwei', '20:00', '22:00', [T('20:00', 'Der Astronaut – Project Hail Mary', 'Spielfilm', Object.assign(L(30, 'Der Astronaut – Project Hail Mary'), { end: '22:00', year: 2026 }))]],
    [4, 'srf1', '22:20', '23:55', [T('22:20', 'Tatort', 'Krimi', { sub: 'Spur im Nebel', series: 'tatort', repeat: true, end: '23:55' })]],
    [5, 'orf2', '20:15', '21:55', [T('20:15', 'Tatort', 'Krimi', { sub: 'Spur im Nebel', series: 'tatort', repeat: true, end: '21:55' })]],
  ];
  const seriesNames = { tatort: 'Tatort', bestatter: 'Der Bestatter', bergdoktor: 'Der Bergdoktor', 'starkes-team': 'Ein starkes Team', maus: 'Die Sendung mit der Maus', 'checker-tobi': 'Checker Tobi', tagesschau: 'Tagesschau', arena: 'Arena', kassensturz: 'Kassensturz', loewenzahn: 'Löwenzahn' };
  const DAYS = 7; // Sa 28.11. to Fr 4.12.: «Programmdaten reichen bis Sa 5.12.» (00:00)
  const EPG_END = DAYS * DAY;

  function build() {
    const out = [];
    for (const c of channels) {
      if (!c.active) continue;
      let items = [];
      const tpl = day[c.id];
      for (let d = 0; d < DAYS; d++) {
        if (tpl === 'hourly' || tpl === 'hourly24') {
          for (let h = 0; h < 24; h++) {
            items.push({ start: d * DAY + h * 60, title: tpl === 'hourly' ? 'Tagesschau' : 'tagesschau', genre: 'Nachrichten', series: tpl === 'hourly' ? 'tagesschau' : null, end: d * DAY + h * 60 + (tpl === 'hourly' ? 10 : 15) });
            const fill = tpl === 'hourly' ? (h % 2 ? '10 vor 10' : 'Kassensturz') : (h % 2 ? 'Bericht aus Berlin' : 'Weltspiegel');
            items.push({ start: d * DAY + h * 60 + (tpl === 'hourly' ? 10 : 15), title: fill, genre: tpl === 'hourly' ? 'Magazin' : 'Nachrichten', repeat: true });
          }
        } else for (const t of tpl) items.push(Object.assign({}, t, { start: d * DAY + hm(t.time), end: t.end ? d * DAY + hm(t.end) + (hm(t.end) < hm(t.time) ? DAY : 0) : undefined }));
      }
      for (const [d, cid, from, until, list] of over) {
        if (cid !== c.id) continue;
        const a = d * DAY + hm(from); const b = d * DAY + (until === '24:00' ? DAY : hm(until));
        items = items.filter((x) => x.start < a || x.start >= b);
        for (const t of list) { const s = d * DAY + hm(t.time); items.push(Object.assign({}, t, { start: s, end: t.end ? d * DAY + hm(t.end) + (hm(t.end) < hm(t.time) ? DAY : 0) : undefined })); }
      }
      items.sort((x, y) => x.start - y.start);
      /* overlaps: a programme that runs past midnight pushes the template aside */
      const clean = [];
      for (const it of items) {
        const prev = clean[clean.length - 1];
        if (prev && prev.end != null && it.start < prev.end) { if (it.end != null && it.end <= prev.end) continue; it.start = prev.end; }
        if (prev && prev.end == null) prev.end = it.start;
        clean.push(it);
      }
      const last = clean[clean.length - 1]; if (last && last.end == null) last.end = Math.min(EPG_END, last.start + 90);
      clean.forEach((it, k) => {
        if (it.end <= it.start) return;
        if (c.id === 'phoenix' && it.start >= hm('20:00')) return; // EPG source delivers only until Sa 20:00
        delete it.time;
        out.push(Object.assign({ id: `p-${c.id}-${it.start}`, ch: c.id, sub: '', desc: '', series: null, season: null, episode: null, repeat: false, firstRun: false, flags: [], img: null, year: null, country: null, director: null, cast: [] }, it, { k }));
      });
    }
    return out;
  }
  const programs = build();
  programs.forEach((p) => { if (!p.desc) p.desc = ''; if (p.series && !p.seriesName) p.seriesName = seriesNames[p.series] || p.title; });

  /* ---------- favourites and history (Anna) ---------- */
  const favorites = { anna: ['srf1', 'srfzwei', 'daserste', 'zdf', 'arte'], luca: ['srfzwei', 'srf1'], mia: ['kika'] };
  const recent = { anna: [['srfzwei', 1236], ['srfinfo', 1150], ['zdf', 1100], ['kika', 1020]], luca: [['srfzwei', 1200]], mia: [['kika', 1180]] };

  /* ---------- recordings ----------
     state: 'running' | 'done' | 'incomplete' | 'failed' | 'rejected'; keep: { mode: 'forever' | 'days', days, until } */
  const R = (o) => Object.assign({ padBefore: 2, padAfter: 10, sizeGB: 0, watchedPct: 0, watched: false, gaps: [], reason: null, keep: { mode: 'forever' }, origin: 'single', rule: null, tracks: 'Ton: Deutsch · Untertitel: Deutsch (Videotext)', fallbackAt: null, img: null, sub: '' }, o);
  const recordings = [
    R({ id: 'rec-hockey', profile: 'anna', title: 'Eishockey: ZSC Lions – HC Davos', ch: 'srfzwei', start: at(0, '20:12'), end: at(0, '21:55'), progStart: at(0, '19:00'), progEnd: at(0, '21:45'), padBefore: 0, sizeGB: 2.9, state: 'running', origin: 'instant', gbPerMin: 0.1, program: 'p-srfzwei-' + at(0, '19:00') }),
    R({ id: 'rec-tatort-6', profile: 'anna', title: 'Tatort', sub: 'Spur im Nebel', ch: 'daserste', start: at(-6, '20:13'), end: at(-6, '21:55'), sizeGB: 5.8, state: 'done', watched: true, watchedPct: 100, origin: 'series', rule: 'rule-tatort' }),
    R({ id: 'rec-tatort-5', profile: 'anna', title: 'Tatort', sub: 'Der Mann am Fenster', ch: 'srf1', start: at(-13, '20:03'), end: at(-13, '21:45'), sizeGB: 5.2, state: 'done', watched: true, watchedPct: 100, origin: 'series', rule: 'rule-tatort' }),
    R({ id: 'rec-tatort-4', profile: 'anna', title: 'Tatort', sub: 'Kalte Asche', ch: 'daserste', start: at(-20, '20:13'), end: at(-20, '21:55'), sizeGB: 5.0, state: 'done', watched: true, watchedPct: 100, origin: 'series', rule: 'rule-tatort' }),
    R({ id: 'rec-tatort-3', profile: 'anna', title: 'Tatort', sub: 'Unter Wasser', ch: 'orf2', start: at(-27, '20:13'), end: at(-27, '21:55'), sizeGB: 5.3, state: 'done', watched: false, watchedPct: 40, origin: 'series', rule: 'rule-tatort' }),
    R({ id: 'rec-tatort-2', profile: 'anna', title: 'Tatort', sub: 'Letzte Ausfahrt', ch: 'srf1', start: at(-34, '20:03'), end: at(-34, '21:45'), sizeGB: 4.9, state: 'done', watched: false, origin: 'series', rule: 'rule-tatort' }),
    R({ id: 'rec-tatort-1', profile: 'anna', title: 'Tatort', sub: 'Das Schweigen der Hirten', ch: 'daserste', start: at(-41, '20:13'), end: at(-41, '21:55'), sizeGB: 5.2, state: 'done', watched: false, origin: 'series', rule: 'rule-tatort' }),
    R({ id: 'rec-arena', profile: 'anna', title: 'Arena', ch: 'srf1', start: at(-1, '22:23'), end: at(-1, '23:50'), sizeGB: 4.1, state: 'done', watchedPct: 35, resumeAt: '35:12', keep: { mode: 'days', days: 30, until: at(29, '00:00') } }),
    R({ id: 'rec-terra', profile: 'anna', title: 'Terra X: Die Alpen', ch: 'zdf', start: at(-3, '19:13'), end: at(-3, '20:10'), sizeGB: 3.2, state: 'incomplete', gaps: [[at(-3, '19:41'), at(-3, '19:44')], [at(-3, '19:58'), at(-3, '20:01')]], fallbackAt: at(-3, '19:44'), reason: 'unreachable', tracks: 'Ton: Deutsch, Deutsch (Audiodeskription) · Untertitel: Deutsch (Videotext)' }),
    R({ id: 'rec-kassensturz', profile: 'anna', title: 'Kassensturz', ch: 'srf1', start: at(-4, '21:03'), end: at(-4, '21:55'), state: 'failed', reason: 'unreachable' }),
    R({ id: 'rec-bergdoktor', profile: 'anna', title: 'Der Bergdoktor', ch: 'zdf', start: at(-2, '20:13'), end: at(-2, '21:55'), state: 'rejected', reason: 'quota' }),
    R({ id: 'rec-mia-checker', profile: 'mia', title: 'Checker Tobi', ch: 'kika', start: at(-7, '19:58'), end: at(-7, '20:40'), sizeGB: 0.9, state: 'done', watched: true, watchedPct: 100 }),
    R({ id: 'rec-mia-maus-3', profile: 'mia', title: 'Die Sendung mit der Maus', sub: 'Wie kommt der Strom in die Steckdose?', ch: 'kika', start: at(-1, '20:28'), end: at(-1, '21:05'), sizeGB: 0.8, state: 'done', origin: 'series', rule: 'rule-mia-maus' }),
    R({ id: 'rec-mia-maus-2', profile: 'mia', title: 'Die Sendung mit der Maus', sub: 'Der Weg der Milch', ch: 'kika', start: at(-8, '20:28'), end: at(-8, '21:05'), sizeGB: 0.8, state: 'done', watched: true, watchedPct: 100, origin: 'series', rule: 'rule-mia-maus' }),
    R({ id: 'rec-mia-maus-1', profile: 'mia', title: 'Die Sendung mit der Maus', sub: 'Lachgeschichte: Der Elefant', ch: 'kika', start: at(-15, '20:28'), end: at(-15, '21:05'), sizeGB: 0.8, state: 'done', watched: true, watchedPct: 100, origin: 'series', rule: 'rule-mia-maus' }),
    R({ id: 'rec-experiment', profile: 'anna', title: 'Das Experiment', ch: 'arte', start: at(-9, '20:13'), end: at(-9, '22:25'), sizeGB: 6.2, state: 'done', watched: true, watchedPct: 100, img: 'b-27.webp' }),
    R({ id: 'rec-kommissar', profile: 'anna', title: 'Der Kommissar und das Meer', sub: 'Tod im Schären', ch: 'zdf', start: at(-7, '21:58'), end: at(-7, '23:40'), sizeGB: 5.8, state: 'done', watchedPct: 60, resumeAt: '58:40' }),
    R({ id: 'rec-polizeiruf', profile: 'anna', title: 'Polizeiruf 110', sub: 'Blinder Fleck', ch: 'daserste', start: at(-12, '20:13'), end: at(-12, '21:55'), sizeGB: 5.6, state: 'done', watched: true, watchedPct: 100 }),
    R({ id: 'rec-bergretter', profile: 'anna', title: 'Die Bergretter', sub: 'Lawinengefahr', ch: 'daserste', start: at(-5, '20:13'), end: at(-5, '21:55'), sizeGB: 6.1, state: 'done' }),
    R({ id: 'rec-alpen', profile: 'anna', title: 'Die Alpen von oben', ch: 'arte', start: at(-14, '19:38'), end: at(-14, '21:25'), sizeGB: 5.4, state: 'done', watched: true, watchedPct: 100 }),
    R({ id: 'rec-dok', profile: 'anna', title: 'DOK', sub: 'Leben am Gletscher', ch: 'srf1', start: at(-8, '20:03'), end: at(-8, '21:10'), sizeGB: 5.9, state: 'done', keep: { mode: 'days', days: 30, until: at(22, '00:00') } }),
    R({ id: 'rec-totenfrau', profile: 'anna', title: 'Totenfrau', sub: 'Folge 4', ch: 'orf2', start: at(-10, '20:13'), end: at(-10, '21:55'), sizeGB: 5.4, state: 'done', watchedPct: 20, img: 'b-50.webp' }),
    ...[['Eishockey: ZSC Lions – SC Bern', -6, 5.1], ['Eishockey: HC Davos – EV Zug', -9, 4.9], ['Fussball: FC Basel – YB', -11, 5.3], ['Eishockey: Lausanne HC – HC Lugano', -13, 4.8], ['Ski alpin: Abfahrt Lake Louise', -14, 5.2],
      ['Eishockey: SC Bern – ZSC Lions', -16, 5.0], ['Fussball: FC Zürich – FC St. Gallen', -18, 4.7], ['Eishockey: EHC Biel – HC Davos', -20, 5.4], ['Biathlon: Sprint Östersund', -21, 5.1], ['Eishockey: EV Zug – Fribourg-Gottéron', -23, 5.1]]
      .map(([t, d, gb], k) => R({ id: 'rec-luca-' + k, profile: 'luca', title: t, ch: 'srfzwei', start: at(d, '19:43'), end: at(d, '22:20'), sizeGB: gb, state: 'done', watched: k % 3 !== 0, watchedPct: k % 3 !== 0 ? 100 : 0 })),
    R({ id: 'rec-luca-scb', profile: 'luca', title: 'Eishockey: SCB – EV Zug', ch: 'srfzwei', start: at(-4, '19:43'), end: at(-4, '22:25'), sizeGB: 6.1, state: 'done', watchedPct: 80 }),
  ];

  /* ---------- planning (rules) ----------
     kind: 'single' | 'series' | 'recurring' | 'manual' */
  const pid = (c, d, t) => `p-${c}-${at(d, t)}`;
  const rules = [
    { id: 'rule-sport', profile: 'anna', kind: 'single', program: pid('zdf', 0, '23:30'), ch: 'zdf', title: 'das aktuelle sportstudio', start: at(0, '23:30'), end: at(1, '00:55'), padBefore: 2, padAfter: 10, keep: { mode: 'forever' }, created: at(-3, '18:00'), estGB: 4.6 },
    { id: 'rule-mars', profile: 'anna', kind: 'single', program: pid('srfzwei', 0, '22:15'), ch: 'srfzwei', title: 'Der Marsianer', start: at(0, '22:15'), end: at(1, '00:35'), padBefore: 2, padAfter: 10, keep: { mode: 'forever' }, created: at(-1, '12:10'), estGB: 8.0 },
    { id: 'rule-tatort', profile: 'anna', kind: 'series', series: 'tatort', title: 'Tatort', match: 'series', channels: 'all', window: null, weekdays: null, onlyNew: true, noDup: true, keepLast: 10, padBefore: 2, padAfter: 10, keep: { mode: 'forever' }, created: at(-60, '20:00'),
      next: pid('srf1', 1, '20:05'), skipped: [[pid('daserste', 1, '20:15'), 'schon geplant'], [pid('orf2', 1, '20:15'), 'schon geplant'], [pid('srf1', 4, '22:20'), 'Wiederholung'], [pid('orf2', 5, '20:15'), 'schon aufgenommen']] },
    { id: 'rule-tagesschau', profile: 'anna', kind: 'recurring', ch: 'srf1', title: 'Tagesschau', from: '19:30', to: '19:55', repeat: 'weekdays', weekdays: [1, 2, 3, 4, 5], endsAfter: null, keepLast: 5, padBefore: 0, padAfter: 2, keep: { mode: 'forever' }, created: at(-10, '19:00') },
    { id: 'rule-tv24', profile: 'anna', kind: 'manual', ch: 'tv24', title: 'TV24, Mo 30.11., 21:00', start: at(2, '21:00'), end: at(2, '22:00'), padBefore: 0, padAfter: 0, keep: { mode: 'forever' }, created: at(-2, '10:00'), warn: 'channelGone' },
    { id: 'rule-mia-maus', profile: 'mia', kind: 'series', series: 'maus', title: 'Die Sendung mit der Maus', match: 'series', channels: ['kika'], onlyNew: false, noDup: true, keepLast: 5, padBefore: 2, padAfter: 5, keep: { mode: 'forever' }, created: at(-20, '09:00') },
    { id: 'rule-mia-checker', profile: 'mia', kind: 'single', program: pid('kika', 1, '20:00'), ch: 'kika', title: 'Checker Tobi', start: at(1, '20:00'), end: at(1, '20:30'), padBefore: 2, padAfter: 10, keep: { mode: 'days', days: 30 }, created: at(-1, '17:00') },
  ];
  /* warnings for planned recordings (LS-12). kind: quota | serverStorage | overlap | duplicate | channelGone | programGone | timeChanged | dst */
  const warnings = [
    { id: 'w-quota', profile: 'anna', kind: 'quota', severity: 'warn', rules: ['rule-mars'], text: 'Für «Der Marsianer» (ca. 8 GB) fehlt voraussichtlich Platz im Speicher des Kontos: frei 58 GB, geplant bis dann 61 GB. Ist beim Start nicht genug frei, wird die Aufnahme nicht gemacht.' },
    { id: 'w-overlap', profile: 'anna', kind: 'overlap', severity: 'info', rules: ['rule-mars', 'rule-sport'], text: '«Der Marsianer» (bis 00:35) und «das aktuelle sportstudio» (ab 23:28) überschneiden sich. Beide werden aufgenommen.' },
    { id: 'w-dup', profile: 'anna', kind: 'duplicate', severity: 'info', rules: ['rule-tatort'], text: '«Tatort: Die Stille nach dem Sturm» ist schon über die Serienaufnahme geplant (So 20:05, SRF 1).', hidden: true },
    { id: 'w-gone', profile: 'anna', kind: 'channelGone', severity: 'bad', rules: ['rule-tv24'], text: 'TV24 ist nicht mehr im Angebot. 1 Aufnahme kann nicht starten.' },
    { id: 'w-time', profile: 'anna', kind: 'timeChanged', severity: 'info', rules: ['rule-tatort'], text: 'Neue Zeit nach Programmänderung: 20:05–21:35 statt 20:05–21:40. Angepasst.' },
  ];
  /* reminders: lead in minutes (0 = at the start) */
  const reminders = [
    { id: 'rem-tatort', profile: 'anna', program: pid('srf1', 1, '20:05'), lead: 5 },
    { id: 'rem-sport', profile: 'anna', program: pid('srfzwei', 0, '21:45'), lead: 0 },
  ];

  /* ---------- administration: EPG, mapping, inputs, diagnostics, storage, accounts ---------- */
  const epgSources = [
    { id: 'epg-xmltv', name: 'XMLTV Schweiz und Deutschland', kind: 'xmltv-url', address: 'https://epg.example.org/ch-de.xml.gz', interval: 24, last: at(0, '04:10'), lastOk: true, channels: 214, programmes: 48312, until: at(7, '00:00'), warnings: [], shift: 0 },
    { id: 'epg-xtream', name: 'Programmdaten Xtream', kind: 'xtream', address: 'vom Xtream-Zugang «Xtream Beispielanbieter»', interval: 24, last: at(-1, '05:05'), lastOk: true, channels: 1840, programmes: 112004, until: at(5, '00:00'), warnings: ['312 Sendungen ohne Zeitzone, als Ortszeit gelesen'], shift: 0 },
  ];
  const epgRuns = { 'epg-xmltv': [[at(0, '04:10'), 'ok', '48 312 Sendungen'], [at(-1, '16:10'), 'ok', '47 980 Sendungen'], [at(-1, '04:10'), 'ok', '47 101 Sendungen'], [at(-2, '16:10'), 'warn', '41 Sendungen mit Ende vor Beginn übersprungen'], [at(-2, '04:10'), 'ok', '46 870 Sendungen']], 'epg-xtream': [[at(-1, '05:05'), 'warn', '312 Sendungen ohne Zeitzone'], [at(-2, '05:05'), 'ok', '111 820 Sendungen']] };
  const mappingCounts = { all: 584, autoSafe: 531, unsure: 22, manual: 19, missing: 12, without: 0 };
  /* [channel, epgSource, epgKey, kind ('auto'|'unsure'|'manual'|'missing'|'without'), reason] */
  const mappings = [
    ['srf1', 'epg-xmltv', 'SRF1.ch', 'auto', 'gleiche Kennung (tvg-id)'], ['srfzwei', 'epg-xmltv', 'SRFzwei.ch', 'auto', 'gleiche Kennung (tvg-id)'], ['srfinfo', 'epg-xmltv', 'SRFinfo.ch', 'auto', 'gleiche Kennung (tvg-id)'],
    ['rts1', 'epg-xmltv', 'RTSUn.ch', 'unsure', 'Name ähnlich (78 %)'], ['rsila1', 'epg-xmltv', 'RSILa1.ch', 'manual', 'von Hand am 20.11.'], ['telezueri', null, null, 'missing', 'kein passender Kanal gefunden'],
    ['daserste', 'epg-xmltv', 'DasErste.de', 'auto', 'gleiche Kennung (tvg-id)'], ['zdf', 'epg-xmltv', 'ZDF.de', 'auto', 'gleiche Kennung (tvg-id)'], ['3sat', 'epg-xmltv', '3sat.de', 'auto', 'Name gleich'],
    ['arte', 'epg-xmltv', 'arte.de', 'unsure', 'Name ähnlich (92 %)'], ['zdfneo', 'epg-xmltv', 'ZDFneo.de', 'auto', 'gleiche Kennung (tvg-id)'], ['zdfinfo', 'epg-xmltv', 'ZDFinfo.de', 'auto', 'gleiche Kennung (tvg-id)'],
    ['tagesschau24', 'epg-xmltv', 'tagesschau24.de', 'auto', 'Name gleich'], ['phoenix', 'epg-xmltv', 'phoenix.de', 'auto', 'gleiche Kennung (tvg-id)'], ['kika', 'epg-xmltv', 'KiKA.de', 'auto', 'gleiche Kennung (tvg-id)'],
    ['orf2', 'epg-xtream', 'ORF2.at', 'manual', 'von Hand am 12.11.'],
    ['bluezoom', 'epg-xmltv', 'BlueZoom.ch', 'auto', 'gleiche Kennung (tvg-id)'], ['cnn', 'epg-xmltv', 'CNNInt.us', 'unsure', 'Name ähnlich (84 %)'],
  ];
  const duplicates = [
    { id: 'dup-1', channels: ['srf1', 'x-srf1hd'], names: ['SRF 1', 'SRF1 HD CH (Xtream)'], reason: 'gleiche EPG-Kennung «SRF1.ch»', keep: 'srf1' },
    { id: 'dup-2', channels: ['zdf', 'x-zdfhd'], names: ['ZDF', 'ZDF HD (Xtream)'], reason: 'Name gleich nach Bereinigung', keep: 'zdf' },
    { id: 'dup-3', channels: ['3sat', 'x-3sat'], names: ['3sat', '3sat HD (Senderliste Region)'], reason: 'gleiche Adresse', keep: '3sat' },
  ];
  const totals = { imported: 10354, active: 584, available: 571, unavailable: 13, duplicates: 3 };
  /* active inputs right now (LA-12): reason viewer | recording | permanent | preload | schedule */
  const inputs = [
    { ch: 'srf1', source: 'src-kabel', backup: 0, state: 'running', reasons: ['permanent', 'viewer'], since: at(0, '00:00') - 3 * DAY, viewers: 2, recordings: 0, bitrate: 8.1, transcode: '1 direkt · 1 umgewandelt (Hardware QSV)', buffer: '12 MB', timeshift: '2.4 GB (2 Puffer)', recGB: 0, errors: null },
    { ch: 'srfzwei', source: 'src-kabel', backup: 0, state: 'running', reasons: ['viewer', 'recording'], since: at(0, '19:00'), viewers: 1, recordings: 2, bitrate: 7.4, transcode: '1 direkt · 0 umgewandelt', buffer: '12 MB', timeshift: '3.1 GB (1 Puffer)', recGB: 2.9, errors: '2 Bildaussetzer in 10 Min.' },
    { ch: 'zdf', source: 'src-kabel', backup: 0, state: 'running', reasons: ['viewer'], since: at(0, '20:14'), viewers: 1, recordings: 0, bitrate: 8.3, transcode: '1 umgewandelt (Hardware QSV)', buffer: '11 MB', timeshift: '0.9 GB (1 Puffer)', recGB: 0, errors: null },
    { ch: 'srfinfo', source: 'src-kabel', backup: 0, state: 'running', reasons: ['preload'], since: at(0, '20:36'), viewers: 0, recordings: 0, bitrate: 5.2, transcode: '–', buffer: '9 MB', timeshift: '–', recGB: 0, errors: null },
    { ch: 'daserste', source: 'src-kabel', backup: 0, state: 'running', reasons: ['schedule'], schedule: 'Abends', since: at(0, '18:00'), viewers: 0, recordings: 0, bitrate: 8.0, transcode: '–', buffer: '12 MB', timeshift: '–', recGB: 0, errors: null },
    /* RG-02 (Vorgabe): a running permanent channel never yields to a viewer. «zurückgestellt» only when it cannot
       start: here the server started at 18:02 with more permanent channels than places (upper limit 8 inputs). */
    { ch: '3sat', source: 'src-kabel', backup: 0, state: 'deferred', reasons: ['permanent'], since: at(0, '18:02'), viewers: 0, recordings: 0, bitrate: 0, transcode: '–', buffer: '0 MB', timeshift: '–', recGB: 0, errors: null, deferredReason: 'Konnte nicht starten: Obergrenze der Eingangsströme erreicht (beim Hochfahren mehr Dauersender eingestellt als Plätze).' },
    { ch: 'zdfinfo', source: 'src-xtream', backup: 2, state: 'reconnecting', attempt: [3, 5], reasons: ['viewer'], since: at(0, '20:12'), viewers: 0, recordings: 0, bitrate: 0, transcode: '–', buffer: '0 MB', timeshift: '–', recGB: 0, errors: 'alle Quellen fehlgeschlagen' },
  ];
  /* consumers of the inputs (LA-12 expanded): no titles, recordings only with the account */
  const consumers = [
    ['srf1', 'viewer', 'familie.keller@example.ch', 'Anna', 'Fernseher Wohnzimmer', 'direkt', 'Live'],
    ['srf1', 'viewer', 'grosseltern@example.ch', 'Grosseltern', 'Chrome · Windows', 'umgewandelt 720p', '−4:10 zeitversetzt'],
    ['srfzwei', 'viewer', 'wg-langstrasse@example.ch', 'Jonas', 'Pixel 8', 'direkt', 'Live'],
    ['srfzwei', 'recording', 'familie.keller@example.ch'], ['srfzwei', 'recording', 'wg-langstrasse@example.ch'],
    ['zdf', 'viewer', 'grosseltern@example.ch', 'Grosseltern', 'Fernseher Stube', 'umgewandelt 1080p', 'Live'],
  ];
  const diagnostics = {
    now: [
      { ch: 'zdfinfo', severity: 'bad', text: 'seit 20:12 · alle Quellen fehlgeschlagen · nächster Versuch 20:43' },
      { ch: 'arte', severity: 'warn', text: 'Bild eingefroren seit 20:38' },
      { ch: '3sat', severity: 'warn', text: 'kein Ton seit 20:30' },
    ],
    week: { outages: 14, reconnects: 38, switches: 6 },
    /* outages per hour (0..23) for the last 7 days, oldest first */
    heat: [[0,0,0,0,0,0,0,0,0,0,0,0,0,1,0,0,0,0,0,0,0,0,0,0],[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,1,0,0,0,0,0],[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,2,1,0,0,0],[0,0,0,1,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,1,2,0,0,0],[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,1,0,0,0,0,0,0,0],[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,3,1,0,0]],
    perChannel: [['zdfinfo', 6, '2 Std. 10 Min.', 21, 4], ['arte', 3, '18 Min.', 7, 0], ['srfzwei', 2, '6 Min.', 5, 1], ['3sat', 2, '35 Min.', 3, 0], ['phoenix', 1, '4 Min.', 2, 1]],
    log: [
      [at(0, '20:12') + 0.07, 'zdfinfo', 'outage', 'Quelle «Kabelanbieter Schweiz» liefert keine Daten (Zeitüberschreitung) · Wiederverbindung 1 von 5'],
      [at(0, '20:13') + 0.5, 'zdfinfo', 'switch', 'Wechsel auf Ersatzquelle 2 «Xtream»'],
      [at(0, '20:14') + 0.03, 'zdfinfo', 'error', 'Ersatzquelle 2: Anmeldung abgelehnt (403) · Host xtream.example.com'],
      [at(0, '20:30'), '3sat', 'warn', 'kein Ton erkannt'],
      [at(0, '20:38'), 'arte', 'warn', 'Bild eingefroren'],
      [at(0, '20:39'), 'zdfinfo', 'reconnect', 'Wiederverbindung 3 von 5'],
      [at(0, '19:02'), 'srfzwei', 'info', 'Eingang gestartet (Zuschauer)'],
      [at(0, '06:00'), null, 'import', 'Import «Kabelanbieter Schweiz»: 3 neu, 12 geändert, 1 verschwunden'],
      [at(0, '05:00'), null, 'error', 'Import «Xtream Beispielanbieter» fehlgeschlagen: Server antwortet 503'],
    ],
    settings: { checkEvery: '6h', parallel: 2, retries: 5, notifyAfter: 15, onlyUsed: true },
  };
  const storage = [
    /* 15 Fassung 2, 0.1: start buffer in the server's memory (factory default, byte budget 512 MB, 96 MB used) */
    { id: 'buffer', name: 'Startpuffer', path: 'Arbeitsspeicher des Servers', kind: 'Arbeitsspeicher (ab Werk)', usedGB: 0.096, totalGB: 0.512, state: 'ok' },
    { id: 'timeshift', name: 'Timeshift', path: '<cache>/live/timeshift', kind: 'SSD (ext4)', usedGB: 18.4, freeGB: 256, limitGB: 50, state: 'ok' },
    { id: 'recordings', name: 'Aufnahmen', path: '/recordings', kind: 'Netzlaufwerk (NFS)', usedGB: 1200, totalGB: 4000, state: 'ok' },
  ];
  /* accounts for the admin (LA-16, LA-22): no titles, no profiles */
  const accounts = [
    { email: 'familie.keller@example.ch', quotaGB: 200, usedGB: 142, count: 31, plannedGB: 21, rights: { liveTv: true, recordings: true, timeshift: true } },
    { email: 'grosseltern@example.ch', quotaGB: 100, usedGB: 12, count: 4, plannedGB: 3, rights: { liveTv: true, recordings: true, timeshift: true } },
    { email: 'wg-langstrasse@example.ch', quotaGB: 100, usedGB: 97, count: 22, plannedGB: 9, rights: { liveTv: true, recordings: true, timeshift: false } },
    { email: 'admin@example.ch', quotaGB: 100, usedGB: 0, count: 0, plannedGB: 0, rights: { liveTv: true, recordings: true, timeshift: true } },
  ];
  /* upper limits of the inputs (15 Fassung 2, `04` 14.1): 8 inputs at once, 4 preloaded */
  const limits = { inputs: 8, preloaded: 4 };
  const settings = {
    dvr: { padBefore: 2, padAfter: 10, keep: 'forever', instantMaxH: 4, followEpg: true, maxParallel: null },
    timeshift: { maxMin: 120, defaultMin: 60, totalGB: 50 }, // factory default 60 min (03-fachkonzept), this sample server: 2 h
    reception: { default: 'ondemand', neighbours: 1, keepHot: 3, favourites: true, stopAfterS: 60, recLeadS: 30 },
    schedules: [{ id: 'sch-abends', name: 'Abends', times: 'Mo–Fr 18:00–23:30, Sa–So 10:00–24:00', channels: ['daserste', 'zdf', 'srf1'] }],
    epg: { previewDays: 7 },
    limits: { recTotal: null, reserveRec: 20, reserveTs: 5, reserveBufPct: 10, defaultQuotaGB: 100 },
  };
  const resources = { cpu: 31, gpu: 12, ramGB: 2.1, bandIn: 38.4, bandOut: 22.1, transcodes: 2, viewers: 4, recRunning: 2, recPlanned24h: 23, recMaxParallel: 4 };

  g.LIVE = {
    DAY, NOW: at(0, '20:41'), EPG_END, timeshiftMaxMin,
    account, profiles, groups, countries, languages, categories,
    sources, importRuns, importResult,
    channels, goneChannels, programs, seriesNames,
    favorites, recent, recordings, rules, warnings, reminders,
    epgSources, epgRuns, mappingCounts, mappings, duplicates, totals,
    inputs, consumers, diagnostics, storage, accounts, settings, resources, limits,
    at, hm,
  };
})(typeof window !== 'undefined' ? window : globalThis);
