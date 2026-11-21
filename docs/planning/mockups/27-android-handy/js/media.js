/* Downloads, audiobooks, podcasts, audio player, radio, wishes, notifications.
   All top-level names carry the prefix "md" so they cannot clash with the other page files.
   Own state: S.dlSeeded, S.dlRules, S.prefs.dlQuality, S.bookPos, S.podPos, S.podPlayed, S.podSubs,
   S.radioFav, S.radioRecent, S.wishSeeded, S.annSeen, extra fields on S.playing; ui.dl, ui.dlVar, ui.wr. */
'use strict';

/* ---------- helpers ---------- */
ICONS.heartOn = '<path d="M12 20s-7.500-4.600-7.500-10A4.300 4.300 0 0 1 12 7.400 4.300 4.300 0 0 1 19.500 10c0 5.400-7.500 10-7.500 10z" fill="currentColor"/>';
ICONS.doneAll = '<path d="m2 12.500 4.500 4.500L16 7.500M11.500 17 21 7.500M8 12.500l1.500 1.500"/>';
ICONS.stop = '<rect x="6" y="6" width="12" height="12" rx="2" fill="currentColor"/>';

const mdNum = (x, d) => x.toFixed(d == null ? 1 : d).replace('.', ',');
const mdSize = gb => (gb < 1 ? `${Math.max(1, Math.round(gb * 1000))} MB` : `${mdNum(gb)} GB`);
const mdClock = sec => {
  const s = Math.max(0, Math.round(sec)); const h = Math.floor(s / 3600); const m = Math.floor((s % 3600) / 60); const x = s % 60;
  return (h ? `${h}:${String(m).padStart(2, '0')}` : `${m}`) + ':' + String(x).padStart(2, '0');
};
const mdLeft = sec => dur(Math.max(1, Math.round(sec / 60)));
const mdRand = seed => { let v = seed % 2147483647; if (v <= 0) v += 2147483646; return () => { v = (v * 16807) % 2147483647; return (v - 1) / 2147483646; }; };
const mdIsAudio = i => T(i).kind === 'a' || T(i).kind === 'p';
const mdPath = () => (location.hash.slice(1) || '/').split('?')[0];
const MD_MON = ['Jan.', 'Feb.', 'März', 'Apr.', 'Mai', 'Juni', 'Juli', 'Aug.', 'Sep.', 'Okt.', 'Nov.', 'Dez.'];
const mdArt = (file, alt, shape) => `<span class="md-th ${shape}">${pic(file, alt)}</span>`;
/** re-open a sheet and keep the focus on the control that was used */
function mdSheet(html, fid) { sheet(html); if (fid) { const el = $(`#overlays [data-fid="${fid}"]`); if (el) el.focus(); } }
/** radio or checkbox row inside a sheet */
const mdOpt = (o) => `<button type="button" class="md-opt" role="${o.check ? 'checkbox' : 'radio'}" aria-checked="${!!o.on}" ${o.attrs} data-fid="${o.fid}">
  <span class="${o.check ? 'md-chk' : 'md-rad'}">${o.check ? icon('check', 16) : ''}</span>
  <span class="tx"><b>${esc(o.title)}</b>${o.sub ? `<small>${esc(o.sub)}</small>` : ''}</span>${o.end ? `<span class="sz">${esc(o.end)}</span>` : ''}</button>`;
const mdEq = paused => `<span class="md-eq${paused ? ' paused' : ''}" aria-hidden="true"><i></i><i></i><i></i></span>`;

/* ---------- episodes, chapters ---------- */
/** episodes of season s; season 1 of 2, 37, 38 has real data, the others are counted from D(i).s */
function mdEps(i, s) {
  const d = D(i) || {}; const sn = s || 1;
  if (sn === 1 && d.e) return d.e.map(([id, n, title, min, o], k) => ({ k, n, title, min, o, img: `e-${id}.webp` }));
  const row0 = (d.s || []).find(x => x[0] === sn); const len = row0 ? row0[1] : 8;
  return Array.from({ length: len }, (_, k) => ({ k, n: k + 1, title: '', min: T(i).min, img: `b-${i}.webp` }));
}
const mdChapCache = {};
function mdChapters(i) {
  if (mdChapCache[i]) return mdChapCache[i];
  const t = T(i); const r = mdRand(i * 7919 + 13); const n = 12 + Math.floor(r() * 9);
  const w = Array.from({ length: n }, () => 0.65 + r() * 0.7); const sum = w.reduce((a, b) => a + b, 0);
  const total = t.min * 60; let start = 0;
  mdChapCache[i] = w.map((x, k) => {
    const len = k === n - 1 ? total - start : Math.round((total * x) / sum);
    const c = { k, n: k + 1, title: `Kapitel ${k + 1}`, start, len }; start += len; return c;
  });
  return mdChapCache[i];
}
const mdChapAt = (ch, pos) => { let k = 0; ch.forEach((c, n) => { if (pos >= c.start) k = n; }); return ch[k]; };

/* podcasts: generated episodes that fit the show */
const MD_POD = {
  56: ['Undercover im Paketzentrum', 'Ohne Wohnung in der Grossstadt', 'Warum wir nicht mehr schlafen können', 'Spielsucht mit 19', 'Mein Vater, der Prepper', 'Das Geschäft mit Abnehmspritzen', 'Allein auf dem Land – junge Menschen erzählen', 'Wie heiss wird es in unseren Städten?'],
  57: ['Was die Wahl in den USA für Europa bedeutet', 'Wie gefährlich ist die Vogelgrippe?', 'Die Rente, einfach erklärt', 'Was hilft wirklich gegen Hitze in der Stadt?', 'Wem gehört die Arktis?', 'Künstliche Intelligenz im Klassenzimmer', 'Der Streit ums Bürgergeld', 'Warum die Mieten weiter steigen'],
  58: ['Der Brief ohne Absender', 'Die Nacht am Baggersee', 'Ein Alibi aus Papier', 'Das Erbe der Schwestern', 'Spuren im Schnee', 'Der falsche Arzt', 'Die verschwundene Kassiererin', 'Ein Schuss im Morgengrauen'],
  59: ['Wie stabil ist die Koalition noch?', 'Europas Verteidigung ohne die USA?', 'Was bleibt vom Klimaschutz?', 'Der Streit um die Schuldenbremse', 'Die Lage im Nahen Osten', 'Wer wählt eigentlich populistisch?', 'Migration: Was die Zahlen sagen', 'Ein Jahr nach der Wahl'],
  60: ['Rückenschmerzen – was wirklich hilft', 'Wie viel Schlaf braucht der Mensch?', 'Vitamin D im Winter', 'Wenn das Herz stolpert', 'Allergien bei Kindern', 'Kopfweh und Migräne', 'Gesund älter werden', 'Antibiotika: Wann sie nötig sind'],
  64: ['Die Woche im Rückspiegel', 'Fondue-Streit und andere Katastrophen', 'Warum der Montag nichts dafür kann', 'Ferienpläne, die nie klappen', 'Das grosse Velo-Quiz', 'Zugausfall und gute Laune', 'Wer hat den letzten Kaffee getrunken?', 'Die Weihnachtsmärkte sind eröffnet'],
  66: ['Wie Gletscher die Alpen formen', 'Neue Hoffnung bei Alzheimer?', 'Mikroplastik im Trinkwasser', 'Was Krähen über uns wissen', 'Die Physik des Schnees', 'Bakterien gegen Bakterien', 'Wie alt kann der Mensch werden?', 'Der Mond als Baustelle'],
  97: ['Warum ist der Himmel blau?', 'Was ist ein Schaltjahr?', 'Wie funktioniert ein Kompass?', 'Warum gähnen wir?', 'Was ist Föhn?', 'Woher kommt das Wort «Velo»?', 'Wie entsteht ein Regenbogen?', 'Warum schäumt Bier?'],
  98: ['Brasilien nach dem Klimagipfel', 'Unterwegs in der Ukraine', 'Japans Dörfer ohne Kinder', 'Der Kampf um Lithium in Chile', 'Indiens neue Mittelschicht', 'Leben am Rand der Sahara', 'Grönland zwischen den Grossmächten', 'Taiwan und die Angst vor dem Ernstfall'],
  102: ['Wie teuer wird die Krankenkasse 2027?', 'Der Zürcher Wohnungsmarkt am Limit', 'Was der starke Franken bedeutet', 'Velo gegen Auto in der Stadt', 'Die Schweiz und Europa', 'Was taugt der Bahnausbau?', 'Lehrermangel an den Schulen', 'Gletscherschwund im Wallis'],
};
const MD_POD_ANY = ['Was uns diese Woche beschäftigt', 'Die grosse Frage nach dem Warum', 'Mythen im Faktencheck', 'Hörerfragen, Teil 2', 'Ein Blick hinter die Kulissen', 'Warum wir so entscheiden, wie wir entscheiden', 'Rückblick und Ausblick', 'Die Folge, die wir lange geplant haben', 'Was wirklich zählt', 'Zu Gast: eine Stimme aus der Praxis', 'Wie viel ist genug?', 'Der Streit, den wir nie hatten', 'Die Sache mit der Zeit', 'Kleine Fragen, grosse Antworten', 'Alles auf Anfang', 'Das hätten wir gern früher gewusst'];
const MD_POD_MIN = { 56: 38, 57: 16, 58: 68, 59: 55, 60: 42, 61: 45, 62: 75, 63: 50, 64: 30, 65: 25, 66: 28, 67: 20, 68: 30, 69: 24, 97: 2, 98: 34, 99: 10, 100: 170, 101: 62, 102: 22, 103: 70 };
function mdPodEps(i) {
  const r = mdRand(i * 4421 + 7); const off = i % MD_POD_ANY.length;
  const list = MD_POD[i] || Array.from({ length: 8 }, (_, k) => MD_POD_ANY[(off + k * 3) % MD_POD_ANY.length]);
  const base = MD_POD_MIN[i] || 38;
  return list.map((title, k) => {
    const d = new Date(2026, 10, 17 - k * 7);
    return { k, title, min: Math.max(1, Math.round(base * (0.75 + r() * 0.5))), date: `${d.getDate()}. ${MD_MON[d.getMonth()]}${d.getFullYear() !== 2026 ? ` ${d.getFullYear()}` : ''}` };
  });
}
const mdPlayed = (i, k) => { const m = S.podPlayed || {}; const v = m[`${i}-${k}`]; return v == null ? k >= 3 : v; };
const mdPodPos = (i, k) => { const p = S.playing; if (p && p.kind === 'pod' && p.i === i && p.e === k) return p.pos; return (S.podPos || {})[`${i}-${k}`] || 0; };
const mdSubs = () => S.podSubs || [56, 58, 64, 66, 97];
const MD_BOOK0 = { 32: Math.round(964 * 60 * 0.38) };
const mdBookPos = i => { const p = S.playing; if (p && p.kind === 'book' && p.i === i) return p.pos; const m = S.bookPos || {}; return m[i] != null ? m[i] : (MD_BOOK0[i] || 0); };

/* ---------- downloads: model ---------- */
const MD_Q = [
  { k: 'high', name: 'Hoch', v: '1080p', a: '128 kbit/s', gv: 0.029, ga: 0.00096 },
  { k: 'medium', name: 'Mittel', v: '720p', a: '96 kbit/s', gv: 0.0124, ga: 0.00072 },
  { k: 'low', name: 'Sparsam', v: '480p', a: '64 kbit/s', gv: 0.0048, ga: 0.00048 },
];
const mdQ = k => MD_Q.find(q => q.k === k) || MD_Q[0];
const mdGb = (min, q, audio) => min * (audio ? mdQ(q).ga : mdQ(q).gv);
const mdQLabel = (q, audio) => `${mdQ(q).name} · ${audio ? mdQ(q).a : mdQ(q).v}`;
const MD_DEV_TOTAL = 128; const MD_DEV_OTHER = 86.4; const MD_BUDGET_BASE = 14.2;
const mdHasEp = i => T(i).kind === 's' || T(i).kind === 'p';
const mdFind = (i, e, s) => (S.downloads || []).find(x => x.i === i && (x.e == null ? null : x.e) === (e == null ? null : e) && (x.s || 1) === (s || 1));
const mdUsed = () => (S.downloads || []).reduce((a, x) => a + (x.gb * (x.pct || 0)) / 100, 0);
const mdFree = () => Math.max(0, MD_DEV_TOTAL - MD_DEV_OTHER - mdUsed());

function mdItem(i, e, s) {
  const t = T(i); const sn = s || 1;
  if (t.kind === 's') { const eps = mdEps(i, sn); const ep = eps[e || 0] || eps[0]; return { title: t.title, sub: `S${sn} · F${ep.n}${ep.title ? ` · ${ep.title}` : ''}`, img: ep.img, shape: 'wide', min: ep.min, play: `#/play/${i}?e=${ep.k}${sn > 1 ? `&s=${sn}` : ''}` }; }
  if (t.kind === 'p') { const eps = mdPodEps(i); const ep = eps[e || 0] || eps[0]; return { title: ep.title, sub: t.title, img: `p-${i}.webp`, shape: 'sq', min: ep.min, play: `#/listen/${i}?e=${ep.k}` }; }
  if (t.kind === 'a') return { title: t.title, sub: t.author, img: `p-${i}.webp`, shape: 'sq', min: t.min, play: `#/listen/${i}` };
  return { title: t.title, sub: [t.year || '', KIND[t.kind]].filter(Boolean).join(' · '), img: `p-${i}.webp`, shape: t.kind === 'e' ? 'wide' : 'tall', min: t.min, play: `#/play/${i}` };
}
function mdEntry(i, e, q, state, pct, s) {
  const it = mdItem(i, e, s); const sn = s || 1;
  return { id: e == null ? `${i}` : sn > 1 ? `${i}-s${sn}-${e}` : `${i}-${e}`, i, e: e == null ? null : e, s: sn, q, state, pct: state === 'done' ? 100 : (pct || 0), gb: Math.round(mdGb(it.min, q, mdIsAudio(i)) * 100) / 100 };
}
function mdDlSeed() {
  if (!Array.isArray(S.downloads)) S.downloads = [];
  S.downloads = S.downloads.filter(x => x && x.id && DATA.T[x.i]);
  if (S.dlRules == null) S.dlRules = { s2: true, p58: true };
  if (S.dlSeeded) return;
  S.dlSeeded = true;
  if (!S.downloads.length) {
    S.downloads = [
      mdEntry(0, null, 'high', 'done'),
      mdEntry(37, 0, 'medium', 'done'), mdEntry(37, 1, 'medium', 'done'),
      mdEntry(2, 1, 'high', 'run', 37),
      mdEntry(33, null, 'medium', 'wait'),
    ];
  }
  save();
}
/** start the next waiting download when nothing runs */
function mdNext() {
  if (S.downloads.some(x => x.state === 'run')) return;
  const w = S.downloads.find(x => x.state === 'wait'); if (w) w.state = 'run';
}

/* ---------- download sheet (ACT.dlAsk, used by other pages) ---------- */
ACT.dlAsk = d => {
  const i = Number(d.i); const t = T(i);
  let e = d.e != null && d.e !== '' ? Number(d.e) : null; let s = 1;
  if (t.kind === 's' && d.s != null && d.n != null) { s = Number(d.s) || 1; e = Math.max(0, Number(d.n) - 1); }
  if (t.kind === 's' && e == null) { const n = (D(i) || {}).n; e = n ? Math.max(0, n[1] - 1) : 0; }
  if (t.kind === 'p' && e == null) e = 0;
  if (t.kind === 's') e = Math.min(e, mdEps(i, s).length - 1);
  if (!mdHasEp(i)) e = null;
  ui.dl = { i, e, s, q: (S.prefs || {}).dlQuality || 'high', scope: 'one' };
  mdDlSheet();
};
function mdScopeEps(dl) {
  if (T(dl.i).kind !== 's') return [dl.e];
  const n = mdEps(dl.i, dl.s).length;
  if (dl.scope === 'season') return [...Array(n).keys()];
  if (dl.scope === 'next3') return [dl.e, dl.e + 1, dl.e + 2].filter(k => k < n);
  return [dl.e];
}
function mdDlSheet(fid) {
  const dl = ui.dl; const t = T(dl.i); const audio = mdIsAudio(dl.i); const it = mdItem(dl.i, dl.e, dl.s); const ser = t.kind === 's';
  const todo = mdScopeEps(dl).filter(k => !mdFind(dl.i, k, dl.s));
  const minOf = k => (ser ? mdEps(dl.i, dl.s)[k].min : mdItem(dl.i, k).min);
  const totalMin = todo.reduce((a, k) => a + minOf(k), 0);
  const size = mdGb(totalMin, dl.q, audio); const free = mdFree();
  const eps = ser ? mdEps(dl.i, dl.s) : [];
  const head = ser ? { title: t.title, sub: it.sub } : t.kind === 'p' ? { title: it.title, sub: t.title } : { title: t.title, sub: it.sub };
  const scope = ser ? `<div class="md-ssec" id="md-dls">Umfang</div><div role="radiogroup" aria-labelledby="md-dls">
    ${mdOpt({ on: dl.scope === 'one', title: 'Diese Folge', sub: `F${eps[dl.e].n}${eps[dl.e].title ? ` · ${eps[dl.e].title}` : ''}`, attrs: 'data-act="mdDlScope" data-v="one"', fid: 'dls-one' })}
    ${mdOpt({ on: dl.scope === 'next3', title: 'Nächste 3 Folgen', sub: `F${eps[dl.e].n} bis F${eps[Math.min(eps.length - 1, dl.e + 2)].n}`, attrs: 'data-act="mdDlScope" data-v="next3"', fid: 'dls-next3' })}
    ${mdOpt({ on: dl.scope === 'season', title: 'Ganze Staffel', sub: `Staffel ${dl.s} · ${eps.length} Folgen`, attrs: 'data-act="mdDlScope" data-v="season"', fid: 'dls-season' })}</div>` : '';
  const qual = `<div class="md-ssec" id="md-dlq">Qualität</div><div role="radiogroup" aria-labelledby="md-dlq">${MD_Q.map(q => mdOpt({
    on: dl.q === q.k, title: mdQLabel(q.k, audio), end: todo.length ? mdSize(mdGb(totalMin, q.k, audio)) : '', attrs: `data-act="mdDlQ" data-v="${q.k}" aria-label="${esc(`${mdQLabel(q.k, audio)} · ${mdSize(mdGb(totalMin || it.min, q.k, audio))}`)}"`, fid: `dlq-${q.k}`,
  })).join('')}</div>`;
  const wifi = (S.prefs || {}).wifiOnly;
  const tight = size > free;
  const info = `
    <div class="md-sinfo${tight ? ' warn' : ''}">${icon(tight ? 'alert' : 'storage', 20)}<span>${tight ? `Zu wenig Platz: ${mdSize(size)} nötig, ${mdSize(free)} frei auf dem Gerät.` : `Frei auf dem Gerät: <b>${mdSize(free)}</b>${todo.length ? ` · danach ${mdSize(free - size)}` : ''}`}</span></div>
    <div class="md-sinfo">${icon('wifi', 20)}<span>${wifi ? 'Lädt nur im WLAN. Im Mobilfunk wartet der Download.' : 'Lädt auch über Mobilfunk.'}</span></div>
    ${t.kind === 'a' ? `<div class="md-sinfo">${icon('headphones', 20)}<span>Ein Download über alle Teile des Hörbuchs.</span></div>` : ''}
    ${!todo.length ? `<div class="md-sinfo">${icon('check', 20)}<span>Schon auf dem Gerät. Du findest es unter «Downloads».</span></div>` : ''}`;
  const btn = todo.length
    ? `<button type="button" class="btn accent" data-act="mdDlGo" data-fid="dl-go"${tight ? ' disabled' : ''}>${icon('download', 20)} Herunterladen</button>`
    : '<a class="btn primary" href="#/downloads" data-fid="dl-goto">Zu den Downloads</a>';
  mdSheet(`<h2>Herunterladen</h2>
    <div class="md-dlhead">${mdArt(it.img, head.title, it.shape)}<span class="tx"><b>${esc(head.title)}</b><small>${esc(head.sub)}</small></span></div>
    ${scope}${qual}${info}
    <div class="acts"><button type="button" class="btn ghost" data-act="close" data-fid="dl-x">Abbrechen</button>${btn}</div>`, fid);
}
ACT.mdDlQ = d => { ui.dl.q = d.v; mdDlSheet(`dlq-${d.v}`); };
ACT.mdDlScope = d => { ui.dl.scope = d.v; mdDlSheet(`dls-${d.v}`); };
ACT.mdDlGo = () => {
  const dl = ui.dl; const t = T(dl.i); mdDlSeed();
  const todo = mdScopeEps(dl).filter(k => !mdFind(dl.i, k, dl.s));
  todo.forEach(k => S.downloads.push(mdEntry(dl.i, mdHasEp(dl.i) ? k : null, dl.q, 'wait', 0, dl.s)));
  mdNext(); closeSheet(true); commit();
  toast(todo.length > 1 ? `${todo.length} Folgen werden heruntergeladen` : `«${t.kind === 'p' ? mdItem(dl.i, dl.e).title : t.title}» wird heruntergeladen`);
};

/* ---------- /downloads ---------- */
function mdDlRow(x, off, inGroup) {
  const it = mdItem(x.i, x.e, x.s); const audio = mdIsAudio(x.i); const grey = off && x.state !== 'done';
  const title = inGroup ? it.sub : it.title;
  const sub0 = inGroup ? '' : (T(x.i).kind === 's' ? it.sub : T(x.i).kind === 'p' ? T(x.i).title : '');
  const q = `${mdQLabel(x.q, audio)} · ${mdSize(x.gb)}`;
  const waitN = S.downloads.filter(y => y.state === 'wait').indexOf(x) + 1;
  let line;
  if (x.state === 'run') line = `<span data-dltx="${x.id}">${Math.floor(x.pct)} % · ${mdSize((x.gb * x.pct) / 100)} von ${mdSize(x.gb)}</span>`;
  else if (x.state === 'pause') line = `Pausiert · ${Math.floor(x.pct)} % · ${mdSize(x.gb)}`;
  else if (x.state === 'wait') line = waitN === 1 ? `Wartet · der Server bereitet vor · ${mdSize(x.gb)}` : `Wartet · Platz ${waitN} · ${mdSize(x.gb)}`;
  else line = q;
  const dis = grey ? ' disabled' : '';
  const main = `${mdArt(it.img, title, it.shape)}<span class="tx"><b>${esc(title)}</b>${sub0 ? `<small>${esc(sub0)}</small>` : ''}<small>${line}</small>
    ${x.state === 'run' || x.state === 'pause' ? `<span class="md-prog${x.state === 'pause' ? ' paused' : ''}"><i data-dlbar="${x.id}" style="width:${Math.max(2, x.pct)}%"></i></span>` : ''}</span>`;
  const mainEl = grey ? `<div class="md-dlmain">${main}</div>`
    : `<a class="md-dlmain" href="${x.state === 'done' ? it.play : href(x.i)}" data-fid="dlm-${x.id}">${main}</a>`;
  const ctl = x.state === 'done'
    ? `<a class="ib md-play" href="${it.play}" aria-label="${esc(`${title} abspielen`)}" data-fid="dlp-${x.id}">${icon('play', 22)}</a>`
    : x.state === 'run' || x.state === 'wait'
      ? `<button type="button" class="ib" data-act="mdDlPause" data-id="${x.id}" aria-label="Pausieren" data-fid="dlpa-${x.id}"${dis}>${icon('pause', 20)}</button>`
      : `<button type="button" class="ib" data-act="mdDlResume" data-id="${x.id}" aria-label="Fortsetzen" data-fid="dlre-${x.id}"${dis}>${icon('download', 22)}</button>`;
  return `<div class="md-dl${grey ? ' off' : ''}" data-st="${x.state}">${mainEl}${ctl}
    <button type="button" class="ib" data-act="mdDlDel" data-id="${x.id}" aria-label="${esc(`${title} löschen`)}" data-fid="dld-${x.id}"${dis}>${icon('trash', 20)}</button></div>`;
}
function mdDlSection(key, label, items, off) {
  if (!items.length) return '';
  const blocks = []; const grp = {};
  items.forEach(x => {
    if (x.e == null) { blocks.push({ one: x }); return; }
    if (!grp[x.i]) { grp[x.i] = { i: x.i, items: [] }; blocks.push(grp[x.i]); }
    grp[x.i].items.push(x);
  });
  const total = items.reduce((a, x) => a + x.gb, 0);
  const html = blocks.map(b => {
    if (b.one) return mdDlRow(b.one, off, false);
    const t = T(b.i); const gb = b.items.reduce((a, x) => a + x.gb, 0); const grey = off && key !== 'done';
    const n = b.items.length;
    return `<div class="md-grp"><div class="md-grph">${grey ? '<div class="md-dlmain">' : `<a class="md-dlmain" href="${href(b.i)}" data-fid="dlg-${key}-${b.i}">`}
      ${mdArt(`p-${b.i}.webp`, t.title, t.kind === 'p' ? 'sq' : 'tall')}<span class="tx"><b>${esc(t.title)}</b><small>${n} ${n === 1 ? 'Folge' : 'Folgen'} · ${mdSize(gb)}</small></span>${grey ? '</div>' : '</a>'}
      <button type="button" class="ib" data-act="mdDlDel" data-grp="${b.i}" data-st="${key}" aria-label="${esc(`Alle ${n} ${n === 1 ? 'Folge' : 'Folgen'} von ${t.title} löschen`)}" data-fid="dlgd-${key}-${b.i}"${grey ? ' disabled' : ''}>${icon('trash', 20)}</button></div>
      <div class="md-grpl">${b.items.map(x => mdDlRow(x, off, true)).join('')}</div></div>`;
  }).join('');
  return `<section class="md-dlsec"><div class="md-sech"><h2>${label}</h2><span>${items.length} · ${mdSize(total)}</span></div><div class="md-list">${html}</div></section>`;
}
function mdStorage(list, off) {
  const used = list.reduce((a, x) => a + (x.gb * (x.pct || 0)) / 100, 0);
  const free = Math.max(0, MD_DEV_TOTAL - MD_DEV_OTHER - used);
  const budget = MD_BUDGET_BASE + list.filter(x => x.state !== 'done').reduce((a, x) => a + x.gb, 0);
  const pc = v => `${Math.max(0.6, (v / MD_DEV_TOTAL) * 100)}%`;
  return `<section class="card md-store" aria-label="Speicher">
    <div class="md-storeh"><span><b>${mdSize(used)}</b> Downloads</span><span>${mdSize(free)} frei von ${MD_DEV_TOTAL} GB</span></div>
    <div class="md-bar" role="img" aria-label="${esc(`Downloads ${mdSize(used)}, andere Apps ${mdSize(MD_DEV_OTHER)}, frei ${mdSize(free)}`)}"><i class="a" style="width:${pc(used)}"></i><i class="o" style="width:${pc(MD_DEV_OTHER)}"></i></div>
    <div class="md-leg"><span class="a">Downloads</span><span class="o">Andere Apps</span><span class="f">Frei</span></div>
    <div class="md-budget${off ? ' off' : ''}">${icon('server', 18)}<span>${off ? 'Budget des Servers: unbekannt, kein Kontakt' : `Budget des Servers: <b>${mdSize(budget)}</b> von 100 GB für vorbereitete Kopien`}</span></div>
  </section>`;
}
function mdFold(key, ic, title, sub, body, off) {
  const open = !!ui.open[key];
  return `<section class="md-foldbox${off ? ' off' : ''}">
    <button type="button" class="md-fold" aria-expanded="${open}" data-act="toggle" data-scope="ui" data-path="open.${key}" data-fid="f-${key}"${off ? ' disabled' : ''}>
      <span class="lead">${icon(ic, 22)}</span><span class="tx"><b>${esc(title)}</b><small>${esc(sub)}</small></span><span class="chev">${icon('down', 20)}</span></button>
    ${open && !off ? `<div class="md-foldb">${body}</div>` : ''}</section>`;
}
route('/downloads', () => {
  mdDlSeed();
  const v = ui.dlVar || 'normal'; const off = v === 'offline';
  const list = v === 'empty' ? [] : S.downloads;
  const chip = (k, l) => `<button type="button" class="chip" aria-pressed="${v === k}" data-act="mdDlVar" data-v="${k}" data-fid="dv-${k}">${l}</button>`;
  const variants = `<div class="md-variant" role="group" aria-label="Varianten (nur im Mockup)"><span>Varianten (nur im Mockup)</span>
    <div class="chips">${chip('normal', 'Normal')}${chip('offline', 'Offline')}${chip('empty', 'Leer')}</div></div>`;
  const banner = off ? `<div class="md-offline" role="status">${icon('wifi', 22)}<span><b>Kein Kontakt zum Server</b> · du siehst deine Downloads</span></div>` : '';
  const lists = list.length
    ? mdDlSection('act', 'Wird geladen', list.filter(x => x.state === 'run' || x.state === 'pause'), off)
      + mdDlSection('wait', 'Wartet', list.filter(x => x.state === 'wait'), off)
      + mdDlSection('done', 'Fertig', list.filter(x => x.state === 'done'), off)
    : `<div class="empty">${icon('download', 48)}<b>Noch keine Downloads</b><p>Lade Filme, Folgen und Hörbücher herunter, dann laufen sie auch ohne Netz.</p>
        <a class="btn primary md-emptybtn" href="#/" data-fid="dl-start">${icon('home', 20)} Zum Start</a></div>`;
  const rules = S.dlRules || {};
  const sers = [2, 37, 38, 17, 13]; const pods = mdSubs();
  const nRules = [...sers.map(i => `s${i}`), ...pods.map(i => `p${i}`)].filter(k => rules[k]).length;
  const wifi = (S.prefs || {}).wifiOnly;
  const auto = mdFold('dlAuto', 'refresh', 'Automatische Downloads', nRules ? `${nRules} ${nRules === 1 ? 'Regel' : 'Regeln'} aktiv` : 'Keine Regel aktiv',
    `<div class="md-ssec">Serien</div>${sers.map(i => sw('S', `dlRules.s${i}`, T(i).title, 'Nächste 3 ungesehene Folgen')).join('')}
     <div class="md-ssec">Podcasts im Abo</div>${pods.map(i => sw('S', `dlRules.p${i}`, T(i).title, 'Neue Folgen, höchstens 3')).join('')}
     <p class="hint md-foldhint">Gilt für dieses Gerät und das Profil ${esc(me().name)}${wifi ? ', nur im WLAN' : ''}. Der Server legt die Downloads beim nächsten Abgleich an.</p>`, off);
  const q = (S.prefs || {}).dlQuality || 'high';
  const settings = mdFold('dlSet', 'settings', 'Einstellungen', `${wifi ? 'Nur im WLAN' : 'WLAN und Mobilfunk'} · ${mdQ(q).name}`,
    `${sw('S', 'prefs.wifiOnly', 'Nur im WLAN herunterladen', 'Im Mobilfunk warten die Downloads')}
     ${li('hd', 'Qualität für neue Downloads', `${mdQ(q).name} · ${mdQ(q).v}`, undefined, 'data-act="mdDlQual" data-fid="dl-qual"')}`, off);
  const html = `${bar('Downloads', { back: false })}
    ${variants}${banner}
    <div class="md-dlgrid">
      <div class="md-dlst">${mdStorage(list, off)}</div>
      <div class="md-dlli">${lists}</div>
      <div class="md-dlfo">${auto}${settings}</div>
    </div>`;
  return { nav: 'downloads', html };
});
PAGES.push(['#/downloads', 'Downloads']);
ACT.mdDlVar = d => { ui.dlVar = d.v; render(); };
ACT.mdDlPause = d => {
  const x = S.downloads.find(y => y.id === d.id); if (!x) return;
  const was = x.state; x.state = 'pause'; if (was === 'run') mdNext();
  commit(); toast('Download pausiert');
};
ACT.mdDlResume = d => {
  const x = S.downloads.find(y => y.id === d.id); if (!x) return;
  const busy = S.downloads.some(y => y.state === 'run');
  x.state = busy ? 'wait' : 'run'; commit();
  toast(busy ? 'Wartet, bis der laufende Download fertig ist' : 'Download läuft weiter');
};
ACT.mdDlDel = d => {
  let ids;
  if (d.grp != null) {
    const g = Number(d.grp); const inSec = x => (d.st === 'act' ? x.state === 'run' || x.state === 'pause' : d.st === 'wait' ? x.state === 'wait' : x.state === 'done');
    ids = S.downloads.filter(x => x.i === g && inSec(x)).map(x => x.id);
  } else ids = [d.id];
  const items = S.downloads.filter(x => ids.includes(x.id)); if (!items.length) return;
  const one = items[0]; const it = mdItem(one.i, one.e, one.s); const gb = items.reduce((a, x) => a + (x.gb * x.pct) / 100, 0);
  const name = items.length > 1 ? `${items.length} Folgen von «${T(one.i).title}»` : `«${T(one.i).kind === 's' ? `${T(one.i).title} · ${it.sub}` : it.title}»`;
  sheet(`<h2>${items.length > 1 ? 'Downloads löschen?' : 'Download löschen?'}</h2>
    <p class="md-sp">${esc(name)} ${items.length > 1 ? 'werden' : 'wird'} von diesem Gerät gelöscht${gb > 0.001 ? ` (${mdSize(gb)})` : ''}. Du kannst ${items.length > 1 ? 'sie' : 'ihn'} jederzeit neu herunterladen.</p>
    <div class="acts"><button type="button" class="btn ghost" data-act="close" data-fid="dd-x">Abbrechen</button>
      <button type="button" class="btn danger" data-act="mdDlDelGo" data-ids="${esc(ids.join(','))}" data-fid="dd-go">${icon('trash', 20)} Löschen</button></div>`);
};
ACT.mdDlDelGo = d => {
  const ids = d.ids.split(','); S.downloads = S.downloads.filter(x => !ids.includes(x.id)); mdNext();
  closeSheet(true); commit(); toast(ids.length > 1 ? `${ids.length} Downloads gelöscht` : 'Download gelöscht');
};
ACT.mdDlQual = () => {
  const q = (S.prefs || {}).dlQuality || 'high';
  sheet(`<h2>Qualität für neue Downloads</h2><div role="radiogroup" aria-label="Qualität">${MD_Q.map(x => mdOpt({
    on: q === x.k, title: `${x.name} · ${x.v}`, sub: `Filme rund ${mdSize(mdGb(120, x.k))} · Hörbücher ${x.a}`, attrs: `data-act="set" data-path="S.prefs.dlQuality" data-value="${x.k}"`, fid: `dqs-${x.k}`,
  })).join('')}</div>
    <p class="hint md-sp">Du kannst die Qualität bei jedem Download auch einzeln wählen.</p>
    <div class="acts"><button type="button" class="btn ghost" data-act="close" data-fid="dqs-x">Schliessen</button></div>`);
};

/* ---------- /book/:i ---------- */
function mdTact(attrs, ic, label, on, fid) {
  return `<button type="button" class="md-tact" ${attrs} aria-pressed="${!!on}" data-fid="${fid}"><span class="ic">${icon(ic, 22)}</span>${esc(label)}</button>`;
}
function mdDlTact(i, e) {
  const x = mdFind(i, e); const lab = !x ? 'Herunterladen' : x.state === 'done' ? 'Auf dem Gerät' : 'Wird geladen';
  return mdTact(`data-act="dlAsk" data-i="${i}"${e != null ? ` data-e="${e}"` : ''}`, x && x.state === 'done' ? 'check' : 'download', lab, !!x, `dlt-${i}`);
}
route('/book/:i', p => {
  const i = Number(p.i); const t = T(i); const ch = mdChapters(i); const total = t.min * 60;
  const pos = mdBookPos(i); const pct = Math.round((pos / total) * 100); const cur = mdChapAt(ch, pos);
  const pl = S.playing; const here = pl && pl.kind === 'book' && pl.i === i; const live = here && !pl.paused;
  const started = pos > 0; const w = S.watch.includes(i); const heard = S.seen.includes(i);
  const more = DATA.T.map((_, k) => k).filter(k => k !== i && T(k).kind === 'a' && T(k).author && T(k).author === t.author);
  const chapters = ch.map(c => {
    const done = c.start + c.len <= pos; const isCur = c.k === cur.k && started;
    const lead = `<span class="md-chn">${done ? icon('check', 18) : isCur && live ? mdEq(false) : c.n}</span>`;
    const sub = done ? 'Gehört' : isCur ? `${live ? 'Läuft' : 'Hier weiter'} · noch ${mdLeft(c.start + c.len - pos)}` : '';
    return li(lead, c.title, sub, `<span class="md-chd">${mdClock(c.len)}</span>`, `href="#/listen/${i}?c=${c.n}" data-fid="ch-${c.n}"${isCur ? ' data-cur="1"' : ''}${done ? ' data-done="1"' : ''} aria-label="${esc(`${c.title}, ${mdClock(c.len)}${sub ? `, ${sub}` : ''}`)}"`);
  }).join('');
  const html = `${bar('', { over: true })}
    <div class="md-det">
      <div class="md-blur" aria-hidden="true">${pic(`p-${i}.webp`, '')}</div>
      <div class="md-dgrid">
        <div class="md-dhead">
          <div class="md-cover">${poster(i)}</div>
          <span class="md-kind">${icon('headphones', 16)} Hörbuch</span>
          <h1>${esc(t.title)}</h1>
          <p class="md-by">${esc(t.author)}</p>
          <p class="md-meta">${t.year} · ${dur(t.min)} · ${ch.length} Kapitel</p>
          ${started ? `<div class="md-bprog"><span class="md-prog"><i style="width:${Math.max(2, pct)}%"></i></span><small>${pct} % · noch ${mdLeft(total - pos)}</small></div>` : ''}
          <a class="btn primary block md-cta" href="#/listen/${i}" data-fid="bk-play">${icon(live ? 'headphones' : 'play', 20)} ${live ? 'Zum Player' : started ? 'Weiterhören' : 'Anhören'}</a>
          ${started ? `<small class="md-ctasub">${esc(cur.title)} · ${mdClock(pos - cur.start)} von ${mdClock(cur.len)}</small>` : ''}
          <div class="md-acts">
            ${mdDlTact(i, null)}
            ${mdTact(`data-act="watch" data-i="${i}"`, w ? 'bookmarkOn' : 'bookmark', w ? 'Gemerkt' : 'Merken', w, `bw-${i}`)}
            ${mdTact(`data-act="seen" data-i="${i}"`, 'check', heard ? 'Gehört' : 'Als gehört', heard, `bs-${i}`)}
          </div>
        </div>
        <div class="md-dmain">
          <div class="md-sech"><h2>Kapitel</h2><span>${ch.length} · ${dur(t.min)}</span></div>
          <div class="md-list md-chl">${chapters}</div>
          ${more.length ? `<div class="md-more">${hrow(`Mehr von ${t.author}`, more)}</div>` : ''}
        </div>
      </div>
    </div>`;
  return { nav: 'start', html };
});
PAGES.push(['#/book/32', 'Hörbuch «Silo»']);
PAGES.push(['#/book/88', 'Hörbuch «4000 Wochen»']);

/* ---------- /podcast/:i ---------- */
route('/podcast/:i', p => {
  const i = Number(p.i); const t = T(i); const eps = mdPodEps(i); const sub = mdSubs().includes(i);
  const pl = S.playing;
  const rows = eps.map(ep => {
    const played = mdPlayed(i, ep.k); const pos = mdPodPos(i, ep.k); const isNow = pl && pl.kind === 'pod' && pl.i === i && pl.e === ep.k;
    const x = mdFind(i, ep.k);
    const state = played ? ' · Gespielt' : pos > 0 ? ` · noch ${mdLeft(ep.min * 60 - pos)}` : '';
    return `<div class="md-ep${played ? ' played' : ''}${isNow ? ' now' : ''}">
      <a class="md-epmain" href="#/listen/${i}?e=${ep.k}" data-fid="ep-${ep.k}" aria-label="${esc(`${ep.title}, ${ep.date}, ${dur(ep.min)}${state}, abspielen`)}">
        <span class="md-epplay">${isNow && !pl.paused ? mdEq(false) : icon('play', 18)}</span>
        <span class="tx"><b>${esc(ep.title)}</b><small>${ep.date} · ${dur(ep.min)}${state}${x ? ` · ${x.state === 'done' ? 'auf dem Gerät' : 'wird geladen'}` : ''}</small>
        ${pos > 0 && !played ? `<span class="md-prog"><i style="width:${Math.max(3, (pos / (ep.min * 60)) * 100)}%"></i></span>` : ''}</span></a>
      <button type="button" class="ib${x ? ' on' : ''}" data-act="dlAsk" data-i="${i}" data-e="${ep.k}" aria-label="${x ? 'Auf dem Gerät' : 'Folge herunterladen'}" data-fid="epd-${ep.k}">${icon(x && x.state === 'done' ? 'storage' : 'download', 20)}</button>
      <button type="button" class="ib${played ? ' on' : ''}" data-act="mdPlayed" data-i="${i}" data-e="${ep.k}" aria-pressed="${played}" aria-label="Als gespielt markieren" data-fid="epp-${ep.k}">${icon('check', 20)}</button></div>`;
  }).join('');
  const html = `${bar('', { over: true })}
    <div class="md-det">
      <div class="md-blur" aria-hidden="true">${pic(`p-${i}.webp`, '')}</div>
      <div class="md-dgrid">
        <div class="md-dhead">
          <div class="md-cover">${poster(i)}</div>
          <span class="md-kind">${icon('mic', 16)} Podcast</span>
          <h1>${esc(t.title)}</h1>
          <p class="md-by">${esc(t.author || 'Herausgeber unbekannt')}</p>
          <p class="md-meta">wöchentlich · ${eps.length} Folgen${t.fsk ? ` · ${fsk(t.fsk)}` : ''}</p>
          <a class="btn primary block md-cta" href="#/listen/${i}?e=0" data-fid="pc-play">${icon('play', 20)} Neueste Folge</a>
          <div class="md-acts">
            ${mdTact(`data-act="mdSub" data-i="${i}"`, sub ? 'check' : 'plus', sub ? 'Abonniert' : 'Abonnieren', sub, `pcs-${i}`)}
            ${mdDlTact(i, 0)}
            ${mdTact(`data-act="mdPlayedAll" data-i="${i}"`, 'doneAll', 'Alle gespielt', eps.every(ep => mdPlayed(i, ep.k)), `pca-${i}`)}
          </div>
        </div>
        <div class="md-dmain">
          <div class="md-sech"><h2>Folgen</h2><span>neueste zuerst</span></div>
          <div class="md-list md-eps">${rows}</div>
          <p class="hint md-foot">${sub ? 'Neue Folgen erscheinen auf der Startseite. Automatisch laden stellst du unter «Downloads» ein.' : 'Abonniere den Podcast, um neue Folgen zu sehen.'}</p>
        </div>
      </div>
    </div>`;
  return { nav: 'start', html };
});
PAGES.push(['#/podcast/56', 'Podcast «Y-Kollektiv»']);
PAGES.push(['#/podcast/58', 'Podcast «Verbrechen»']);
ACT.mdSub = d => {
  const i = Number(d.i); const s = mdSubs().slice(); const k = s.indexOf(i);
  if (k < 0) s.push(i); else s.splice(k, 1);
  S.podSubs = s; commit(); toast(k < 0 ? 'Abonniert' : 'Abo beendet');
};
ACT.mdPlayed = d => {
  const key = `${d.i}-${d.e}`; const now = !mdPlayed(Number(d.i), Number(d.e));
  S.podPlayed = Object.assign({}, S.podPlayed, { [key]: now }); commit(); toast(now ? 'Als gespielt markiert' : 'Markierung entfernt');
};
ACT.mdPlayedAll = d => {
  const i = Number(d.i); const eps = mdPodEps(i); const all = eps.every(ep => mdPlayed(i, ep.k));
  S.podPlayed = Object.assign({}, S.podPlayed); eps.forEach(ep => { S.podPlayed[`${i}-${ep.k}`] = !all; });
  commit(); toast(all ? 'Alle Markierungen entfernt' : 'Alle Folgen als gespielt markiert');
};

/* ---------- audio playback ---------- */
function mdPersist(p) {
  if (!p) return;
  if (p.kind === 'book') S.bookPos = Object.assign({}, S.bookPos, { [p.i]: p.pos });
  if (p.kind === 'pod') S.podPos = Object.assign({}, S.podPos, { [`${p.i}-${p.e}`]: p.pos });
}
function mdCur(p) {
  if (p.kind === 'book') return mdChapAt(mdChapters(p.i), p.pos);
  return { k: 0, n: 1, title: '', start: 0, len: p.total };
}
function mdSubLine(p) {
  if (p.kind === 'book') p.sub = `${T(p.i).author} · ${mdCur(p).title}`;
}
function mdStart(i, e, c) {
  mdPersist(S.playing);
  const t = T(i); const speed = S.playing && S.playing.speed ? S.playing.speed : (S.audioSpeed || 1);
  if (t.kind === 'p') {
    const eps = mdPodEps(i); const ep = eps[e] || eps[0];
    S.playing = { kind: 'pod', key: `p${i}-${ep.k}`, i, e: ep.k, pos: mdPodPos(i, ep.k), total: ep.min * 60, speed, sleep: null, title: ep.title, sub: t.title, img: `p-${i}.webp`, href: `#/listen/${i}?e=${ep.k}`, paused: false };
    if (S.playing.pos >= S.playing.total - 2) S.playing.pos = 0;
  } else {
    const ch = mdChapters(i); let pos = mdBookPos(i);
    if (c != null && ch[c - 1]) pos = ch[c - 1].start;
    S.playing = { kind: 'book', key: `a${i}`, i, pos, total: t.min * 60, speed, sleep: null, title: t.title, sub: '', img: `p-${i}.webp`, href: `#/listen/${i}`, paused: false };
  }
  mdSubLine(S.playing); save();
}
function mdSeek(pos) {
  const p = S.playing; if (!p) return;
  p.pos = Math.max(0, Math.min(p.total, pos)); mdSubLine(p); mdPersist(p);
}
route('/listen/:i', (pp, q) => {
  const i = Number(pp.i); const t = T(i); const pod = t.kind === 'p';
  const fresh = lastPath !== `/listen/${pp.i}`;
  let p = S.playing;
  if (pod) {
    const want = q.e != null ? Number(q.e) : (p && p.kind === 'pod' && p.i === i ? p.e : 0);
    if (!p || p.key !== `p${i}-${want}`) mdStart(i, want, null);
  } else if (!p || p.key !== `a${i}`) mdStart(i, null, q.c != null ? Number(q.c) : null);
  else if (fresh && q.c != null) { const c = mdChapters(i)[Number(q.c) - 1]; if (c) { mdSeek(c.start); p.paused = false; save(); } }
  p = S.playing;
  const cur = mdCur(p); const ch = pod ? null : mdChapters(i);
  const eps = pod ? mdPodEps(i) : null;
  const rel = p.pos - cur.start;
  const kind = pod ? `Podcast · ${eps[p.e].date}` : `Hörbuch · Kapitel ${cur.n} von ${ch.length}`;
  const speed = mdSpeedTx(p.speed || 1);
  const sleep = p.sleep == null ? 'Schlaftimer' : p.sleep === 'end' ? (pod ? 'Ende der Folge' : 'Ende des Kapitels') : mdClock(p.sleep);
  const list = pod
    ? eps.map(ep => li(`<span class="md-chn">${ep.k === p.e ? mdEq(p.paused) : ep.k + 1}</span>`, ep.title, `${ep.date} · ${dur(ep.min)}`, '', `href="#/listen/${i}?e=${ep.k}" data-fid="lsl-${ep.k}"${ep.k === p.e ? ' data-cur="1"' : ''}`)).join('')
    : ch.map(c => li(`<span class="md-chn">${c.k === cur.k ? mdEq(p.paused) : c.start + c.len <= p.pos ? icon('check', 18) : c.n}</span>`, c.title, '', `<span class="md-chd">${mdClock(c.len)}</span>`, `data-act="mdChap" data-n="${c.n}" data-fid="lsl-${c.n}"${c.k === cur.k ? ' data-cur="1"' : ''}${c.start + c.len <= p.pos ? ' data-done="1"' : ''}`)).join('');
  const canPrev = pod ? p.e > 0 : true; const canNext = pod ? p.e < eps.length - 1 : cur.k < ch.length - 1;
  const html = `<div class="md-ls" id="mdls">
    <div class="md-lsbg" aria-hidden="true">${pic(p.img, '')}</div>
    <header class="md-lsbar"><button type="button" class="ib" data-act="back" aria-label="Player verkleinern" data-fid="ls-back">${icon('down')}</button>
      <span id="mdlsKind">${esc(kind)}</span>
      <a class="ib" href="${href(i)}" aria-label="${pod ? 'Zum Podcast' : 'Zum Hörbuch'}" data-fid="ls-info">${icon('info')}</a></header>
    <div class="md-lsgrid">
      <div class="md-lscov${p.paused ? ' paused' : ''}">${pic(p.img, t.title)}</div>
      <div class="md-lsctl">
        <h1>${esc(pod ? p.title : t.title)}</h1>
        <p class="md-lsby">${esc(pod ? t.title : t.author)}</p>
        ${pod ? '' : `<p class="md-lschap">${icon('book', 16)} <span>${esc(cur.title)}</span></p>`}
        <input type="range" class="md-seek" id="mdlsSeek" min="0" max="${cur.len}" step="1" value="${Math.round(rel)}" style="--p:${(rel / cur.len) * 100}%" aria-label="${pod ? 'Position in der Folge' : 'Position im Kapitel'}" aria-valuetext="${mdClock(rel)} von ${mdClock(cur.len)}" data-fid="ls-seek">
        <div class="md-lstime"><span id="mdlsPos">${mdClock(rel)}</span><span id="mdlsRem">−${mdClock(cur.len - rel)}</span></div>
        ${pod ? '' : `<p class="md-lstot" id="mdlsTot">noch ${mdLeft(p.total - p.pos)} insgesamt · ${Math.round((p.pos / p.total) * 100)} %</p>`}
        <div class="md-lsmain">
          <button type="button" class="ib" data-act="mdPrev" aria-label="${pod ? 'Vorherige Folge' : 'Kapitelanfang oder vorheriges Kapitel'}" data-fid="ls-prev"${canPrev ? '' : ' disabled'}>${icon('prev', 26)}</button>
          <button type="button" class="ib md-skip" data-act="mdSkip" data-s="-30" aria-label="30 Sekunden zurück" data-fid="ls-rew">${icon('rew', 34)}<b>30</b></button>
          <button type="button" class="md-big" data-act="mdLsPlay" aria-label="${p.paused ? 'Abspielen' : 'Pause'}" data-fid="ls-play">${icon(p.paused ? 'play' : 'pause', 34)}</button>
          <button type="button" class="ib md-skip" data-act="mdSkip" data-s="30" aria-label="30 Sekunden vor" data-fid="ls-fwd">${icon('fwd', 34)}<b>30</b></button>
          <button type="button" class="ib" data-act="mdNextC" aria-label="${pod ? 'Nächste Folge' : 'Nächstes Kapitel'}" data-fid="ls-next"${canNext ? '' : ' disabled'}>${icon('next', 26)}</button>
        </div>
        <div class="md-lsmore">
          <button type="button" class="md-pill" data-act="mdSpeedSheet" aria-label="Tempo ${esc(speed)}" data-fid="ls-speed">${icon('speed', 20)}<span>${esc(speed)}</span></button>
          <button type="button" class="md-pill${p.sleep != null ? ' on' : ''}" data-act="mdSleepSheet" aria-label="Schlaftimer" data-fid="ls-sleep">${icon('moon', 20)}<span id="mdlsSleep">${esc(sleep)}</span></button>
          <button type="button" class="md-pill md-lsonly" data-act="mdListSheet" aria-label="${pod ? 'Folgen' : 'Kapitel'}" data-fid="ls-list">${icon('list', 20)}<span>${pod ? 'Folgen' : 'Kapitel'}</span></button>
          <button type="button" class="md-pill" data-act="dlAsk" data-i="${i}"${pod ? ` data-e="${p.e}"` : ''} aria-label="Herunterladen" data-fid="ls-dl">${icon(mdFind(i, pod ? p.e : null) ? 'check' : 'download', 20)}<span>${mdFind(i, pod ? p.e : null) ? 'Geladen' : 'Laden'}</span></button>
        </div>
        <div class="md-lslist"><div class="md-sech"><h2>${pod ? 'Folgen' : 'Kapitel'}</h2><span>${pod ? eps.length : ch.length}</span></div><div class="md-list">${list}</div></div>
      </div>
    </div></div>`;
  return {
    full: true, html,
    after: root => { const c = $('.md-lslist [data-cur]', root); const box = $('.md-lslist .md-list', root); if (c && box) box.scrollTop = c.offsetTop - box.offsetTop - 60; },
  };
});
PAGES.push(['#/listen/32', 'Tonplayer · Hörbuch «Silo»']);
PAGES.push(['#/listen/58?e=0', 'Tonplayer · Podcast «Verbrechen»']);
ACT.mdLsPlay = () => { const p = S.playing; if (!p) return; p.paused = !p.paused; commit(); };
ACT.mdSkip = d => { const p = S.playing; if (!p) return; mdSeek(p.pos + Number(d.s)); commit(); };
ACT.mdPrev = () => {
  const p = S.playing; if (!p) return;
  if (p.kind === 'pod') { go(`#/listen/${p.i}?e=${p.e - 1}`); return; }
  const ch = mdChapters(p.i); const c = mdCur(p);
  mdSeek(p.pos - c.start > 5 || c.k === 0 ? c.start : ch[c.k - 1].start); commit();
};
ACT.mdNextC = () => {
  const p = S.playing; if (!p) return;
  if (p.kind === 'pod') { go(`#/listen/${p.i}?e=${p.e + 1}`); return; }
  const ch = mdChapters(p.i); const c = mdCur(p); if (ch[c.k + 1]) { mdSeek(ch[c.k + 1].start); commit(); }
};
ACT.mdChap = d => {
  const p = S.playing; if (!p) return; const c = mdChapters(p.i)[Number(d.n) - 1]; if (!c) return;
  mdSeek(c.start); p.paused = false; closeSheet(true); commit(); toast(`${c.title} · ab Anfang`);
};
const MD_SPEEDS = [0.8, 1, 1.2, 1.5, 1.8, 2];
const mdSpeedTx = v => `${mdNum(v)}×`;
ACT.mdSpeedSheet = () => {
  const cur = (S.playing && S.playing.speed) || 1;
  sheet(`<h2>Tempo</h2><div role="radiogroup" aria-label="Tempo" class="md-speeds">${MD_SPEEDS.map(v => `<button type="button" class="md-spd" role="radio" aria-checked="${v === cur}" data-act="mdSpeed" data-v="${v}" data-fid="spd-${v}">${mdSpeedTx(v)}</button>`).join('')}</div>
    <p class="hint md-sp">Die Stimme bleibt auf jeder Stufe gleich hoch.</p>
    <div class="acts"><button type="button" class="btn ghost" data-act="close" data-fid="spd-x">Schliessen</button></div>`);
  const on = $('#overlays [aria-checked="true"]'); if (on) on.focus();
};
ACT.mdSpeed = d => { const p = S.playing; if (!p) return; p.speed = Number(d.v); S.audioSpeed = p.speed; closeSheet(true); commit(); toast(`Tempo ${mdSpeedTx(p.speed)}`); };
ACT.mdSleepSheet = () => {
  const p = S.playing; if (!p) return; const pod = p.kind === 'pod';
  const opts = [[null, 'Aus'], [15 * 60, '15 Minuten'], [30 * 60, '30 Minuten'], [45 * 60, '45 Minuten'], [60 * 60, '1 Stunde'], ['end', pod ? 'Ende der Folge' : 'Ende des Kapitels']];
  const now = p.sleep == null ? null : p.sleep === 'end' ? 'end' : 'run';
  sheet(`<h2>Schlaftimer</h2>${typeof p.sleep === 'number' ? `<p class="md-sp">Läuft noch ${mdClock(p.sleep)}.</p>` : ''}<div role="radiogroup" aria-label="Schlaftimer">${opts.map(([v, l], k) => mdOpt({
    on: (v === null && now === null) || (v === 'end' && now === 'end'), title: l, attrs: `data-act="mdSleep" data-v="${v == null ? '' : v}"`, fid: `slp-${k}`,
  })).join('')}</div>
    <div class="acts"><button type="button" class="btn ghost" data-act="close" data-fid="slp-x">Schliessen</button></div>`);
};
ACT.mdSleep = d => {
  const p = S.playing; if (!p) return;
  p.sleep = d.v === '' ? null : d.v === 'end' ? 'end' : Number(d.v);
  closeSheet(true); commit(); toast(p.sleep == null ? 'Schlaftimer aus' : p.sleep === 'end' ? `Pausiert am ${p.kind === 'pod' ? 'Ende der Folge' : 'Ende des Kapitels'}` : `Pausiert in ${Math.round(p.sleep / 60)} Minuten`);
};
ACT.mdListSheet = () => {
  const p = S.playing; if (!p) return;
  if (p.kind === 'pod') {
    sheet(`<h2>Folgen</h2><div class="md-list md-sheetl">${mdPodEps(p.i).map(ep => li(`<span class="md-chn">${ep.k === p.e ? mdEq(p.paused) : ep.k + 1}</span>`, ep.title, `${ep.date} · ${dur(ep.min)}`, '', `href="#/listen/${p.i}?e=${ep.k}" data-fid="sl-${ep.k}"${ep.k === p.e ? ' data-cur="1"' : ''}`)).join('')}</div>
      <div class="acts"><button type="button" class="btn ghost" data-act="close" data-fid="sl-x">Schliessen</button></div>`);
  } else {
    const cur = mdCur(p);
    sheet(`<h2>Kapitel</h2><div class="md-list md-sheetl">${mdChapters(p.i).map(c => li(`<span class="md-chn">${c.k === cur.k ? mdEq(p.paused) : c.start + c.len <= p.pos ? icon('check', 18) : c.n}</span>`, c.title, c.k === cur.k ? `läuft · noch ${mdLeft(c.start + c.len - p.pos)}` : '', `<span class="md-chd">${mdClock(c.len)}</span>`, `data-act="mdChap" data-n="${c.n}" data-fid="sl-${c.n}"${c.k === cur.k ? ' data-cur="1"' : ''}${c.start + c.len <= p.pos ? ' data-done="1"' : ''}`)).join('')}</div>
      <div class="acts"><button type="button" class="btn ghost" data-act="close" data-fid="sl-x">Schliessen</button></div>`);
  }
  const c = $('#overlays [data-cur]'); if (c) { c.focus(); c.scrollIntoView({ block: 'center' }); }
};
/* update the player in place, so the seek bar moves without redrawing the page */
function mdLsDom(p) {
  const seek = $('#mdlsSeek'); if (!seek || !p || p.kind === 'radio') return;
  const c = mdCur(p); const rel = p.pos - c.start;
  if (!seek.matches(':active')) seek.value = Math.round(rel);
  seek.style.setProperty('--p', `${(rel / c.len) * 100}%`);
  seek.setAttribute('aria-valuetext', `${mdClock(rel)} von ${mdClock(c.len)}`);
  const a = $('#mdlsPos'); if (a) a.textContent = mdClock(rel);
  const b = $('#mdlsRem'); if (b) b.textContent = `−${mdClock(c.len - rel)}`;
  const tt = $('#mdlsTot'); if (tt) tt.textContent = `noch ${mdLeft(p.total - p.pos)} insgesamt · ${Math.round((p.pos / p.total) * 100)} %`;
  const sl = $('#mdlsSleep'); if (sl && typeof p.sleep === 'number') sl.textContent = mdClock(p.sleep);
}
document.addEventListener('input', e => {
  if (e.target.id === 'mdlsSeek' && S.playing) { const c = mdCur(S.playing); mdSeek(c.start + Number(e.target.value)); mdLsDom(S.playing); }
  if (e.target.id === 'mdWq') mdFilter();
});
document.addEventListener('change', e => { if (e.target.id === 'mdlsSeek') save(); });

/* mini player: a radio station without logo shows its name instead of a broken image */
const mdDrawMini0 = drawMini;
drawMini = function () { // eslint-disable-line no-func-assign
  mdDrawMini0();
  const p = S.playing; const im = $('#mini img');
  if (im && p && p.kind === 'radio' && !MD_LOGO.includes(p.id)) { const s = document.createElement('span'); s.className = 'md-miniart'; s.textContent = p.title.replace(/^Radio\s+/, '').slice(0, 2); im.replaceWith(s); }
};

/* ---------- /radio ---------- */
const MD_LOGO = [39211, 38967];
const MD_SONGS = {
  39211: ['Adele – Someone Like You', 'Phil Collins – Another Day in Paradise', 'Lo & Leduc – 079', 'Norah Jones – Don’t Know Why', 'Sting – Fields of Gold', 'Bastian Baker – Lucky'],
  38967: ['Dua Lipa – Houdini', 'Hecht – Kawasaki', 'Billie Eilish – Birds of a Feather', 'Coldplay – Viva la Vida', 'Stephan Eicher – Déjeuner en paix', 'Sophie Hunger – Le vent nous portera'],
  40241: ['Gölä – Schwan', 'ABBA – Dancing Queen', 'Toto – Africa', 'Polo Hofer – Alperose', 'Pegasus – Skyline', 'Whitney Houston – I Wanna Dance with Somebody'],
};
const MD_LAND = { CH: 'Schweiz', DE: 'Deutschland', AT: 'Österreich' };
const mdStation = id => { const r = DATA.radio.find(x => x[0] === id); return r ? { id: r[0], name: r[1], land: MD_LAND[r[2]] || r[2], style: r[3].split(',').map(s => s.trim().replace(/^./, c => c.toUpperCase())).join(', ') } : null; };
const mdRArt = st => (MD_LOGO.includes(st.id) ? pic(`r-${st.id}.webp`, st.name) : `<span class="md-rname">${esc(st.name.replace(/^Radio\s+/, ''))}</span>`);
const mdSong = (id, k) => { const l = MD_SONGS[id] || ['Live']; return l[(k || 0) % l.length]; };
const mdFavs = () => S.radioFav || [39211];
const mdRecent = () => S.radioRecent || [{ id: 38967, when: 'gestern' }, { id: 40241, when: 'Montag' }];
route('/radio', () => {
  const p = S.playing; const live = p && p.kind === 'radio' ? p : null; const favs = mdFavs();
  const tiles = DATA.radio.map(r => {
    const st = mdStation(r[0]); const on = live && live.id === st.id; const fav = favs.includes(st.id);
    return `<div class="md-rtile${on ? ' on' : ''}${on && live.paused ? ' paused' : ''}">
      <button type="button" class="md-rplay" data-act="mdRadio" data-id="${st.id}" aria-pressed="${!!on}" aria-label="${esc(`${st.name}, ${on && !live.paused ? 'pausieren' : 'live hören'}`)}" data-fid="rt-${st.id}">
        <span class="md-rart">${mdRArt(st)}<span class="md-rbadge">${on && !live.paused ? `${mdEq(false)} Live` : on ? 'Pausiert' : `${icon('play', 14)} Live`}</span></span>
        <span class="tx"><b>${esc(st.name)}</b><small>${esc(st.land)} · ${esc(st.style)}</small>${on ? `<small class="md-np" data-np>${esc(mdSong(st.id, live.song))}</small>` : ''}</span></button>
      <button type="button" class="ib md-rfav${fav ? ' on' : ''}" data-act="mdFav" data-id="${st.id}" aria-pressed="${fav}" aria-label="${esc(`${st.name} als Favorit`)}" data-fid="rf-${st.id}">${icon(fav ? 'heartOn' : 'heart', 22)}</button></div>`;
  }).join('');
  const now = live ? (() => {
    const st = mdStation(live.id);
    return `<section class="md-now" aria-label="Läuft gerade"><span class="md-nowart">${mdRArt(st)}</span>
      <span class="tx"><small>${live.paused ? 'Pausiert' : `${mdEq(false)} Läuft gerade`}</small><b>${esc(st.name)}</b><span class="md-np" data-np>${esc(mdSong(st.id, live.song))}</span></span>
      <button type="button" class="ib md-nowbtn" data-act="mdRadio" data-id="${st.id}" aria-label="${live.paused ? 'Weiter' : 'Pause'}" data-fid="rn-play">${icon(live.paused ? 'play' : 'pause', 22)}</button>
      <button type="button" class="ib" data-act="mdRadioStop" aria-label="Beenden" data-fid="rn-stop">${icon('stop', 20)}</button></section>`;
  })() : '';
  const recent = mdRecent().map(r => {
    const st = mdStation(r.id); if (!st) return '';
    const on = live && live.id === st.id;
    return li(`<span class="md-rmini">${mdRArt(st)}</span>`, st.name, `${on ? 'läuft gerade' : r.when} · ${st.style}`, on ? mdEq(live.paused) : icon('play', 20), `data-act="mdRadio" data-id="${st.id}" data-fid="rr-${st.id}"`);
  }).join('');
  const html = `${bar('Webradio')}
    ${now}
    <div class="md-sech"><h2>Sender</h2><span>${DATA.radio.length} in der Bibliothek</span></div>
    <div class="md-rgrid">${tiles}</div>
    <div class="md-sech"><h2>Zuletzt gespielt</h2></div>
    <div class="md-list md-rrec">${recent}</div>
    <p class="hint md-foot">Weitere Sender kommen über die Verwaltung im Browser dazu.</p>`;
  return { nav: 'more', html };
});
PAGES.push(['#/radio', 'Webradio']);
ACT.mdRadio = d => {
  const id = Number(d.id); const p = S.playing;
  if (p && p.kind === 'radio' && p.id === id) { p.paused = !p.paused; commit(); return; }
  const st = mdStation(id); if (!st) return; mdPersist(p);
  const k = Math.floor(Math.random() * 6);
  S.playing = { kind: 'radio', id, song: k, title: st.name, sub: `Live · ${mdSong(id, k)}`, img: `r-${id}.webp`, href: '#/radio', paused: false };
  S.radioRecent = [{ id, when: 'heute' }, ...mdRecent().filter(r => r.id !== id)].slice(0, 5);
  commit(); toast(`${st.name} live`);
};
ACT.mdRadioStop = () => { S.playing = null; commit(); toast('Radio beendet'); };
ACT.mdFav = d => {
  const id = Number(d.id); const f = mdFavs().slice(); const k = f.indexOf(id);
  if (k < 0) f.push(id); else f.splice(k, 1); S.radioFav = f; commit();
  toast(k < 0 ? 'Zu den Favoriten hinzugefügt' : 'Aus den Favoriten entfernt');
};

/* ---------- wishes ---------- */
const MD_WST = {
  req: ['Angefragt', 'wartet auf Freigabe', ''], ok: ['Genehmigt', 'Seerr sucht den Titel', 'ok'],
  load: ['Wird geladen', '', 'acc'], done: ['Verfügbar', 'in der Bibliothek', 'ok'],
};
const MD_WSTEPS = ['req', 'ok', 'load', 'done'];
function mdWishes() {
  if (!Array.isArray(S.wishes)) S.wishes = [];
  S.wishes = S.wishes.map(w => (typeof w === 'number' ? { i: w, state: 'req', when: 'früher' } : w)).filter(w => w && DATA.T[w.i] && MD_WST[w.state]);
  if (!S.wishSeeded) {
    S.wishSeeded = true;
    if (!S.wishes.length) S.wishes = [{ i: 79, state: 'req', when: 'heute' }, { i: 108, state: 'ok', when: 'vorgestern', seasons: [1] }, { i: 75, state: 'load', pct: 64, when: 'vor 3 Tagen' }];
    save();
  }
  return S.wishes;
}
const mdWish = i => mdWishes().find(w => w.i === i);
const mdQuota = () => { const w = mdWishes(); return { films: 1 + w.filter(x => T(x.i).kind === 'm').length, series: w.filter(x => T(x.i).kind === 's').length }; };
const mdSeasons = i => ((D(i) || {}).s ? D(i).s.length : (i % 4) + 1);
const mdWBadge = w => `<span class="badge${MD_WST[w.state][2] ? ` ${MD_WST[w.state][2]}` : ''}">${MD_WST[w.state][0]}${w.state === 'load' ? ` · <span data-wpt="${w.i}">${Math.floor(w.pct || 0)}</span> %` : ''}</span>`;
function mdWRow(w) {
  const t = T(w.i); const open = w.state === 'req' || w.state === 'ok';
  const meta = [KIND[t.kind], t.year, w.seasons ? `Staffel ${w.seasons.join(', ')}` : ''].filter(Boolean).join(' · ');
  return `<article class="md-wi">
    <a class="md-wimain" href="#/wish/${w.i}" data-fid="wl-${w.i}">${mdArt(`p-${w.i}.webp`, t.title, 'tall')}
      <span class="tx"><b>${esc(t.title)}</b><small>${esc(meta)}</small><small>gewünscht ${esc(w.when)}${MD_WST[w.state][1] ? ` · ${MD_WST[w.state][1]}` : ''}</small>
      <span class="md-wst">${mdWBadge(w)}</span>
      ${w.state === 'load' ? `<span class="md-prog"><i data-wpb="${w.i}" style="width:${Math.max(2, w.pct || 0)}%"></i></span>` : ''}</span></a>
    ${open ? `<button type="button" class="btn ghost sm" data-act="mdWDel" data-i="${w.i}" data-fid="wd-${w.i}">Zurückziehen</button>` : ''}
    ${w.state === 'done' ? `<a class="btn secondary sm" href="${href(w.i)}" data-fid="wv-${w.i}">${icon('play', 18)} Ansehen</a>` : ''}</article>`;
}
function mdQuotaCard() {
  const q = mdQuota();
  return `<section class="card md-quota" aria-label="Kontingent">
    <div class="md-qrow"><span><b>${q.films} von 10 Filmen</b> in den letzten 7 Tagen</span></div>
    <span class="md-meter" role="img" aria-label="${q.films} von 10 Filmen"><i style="width:${Math.min(100, q.films * 10)}%"></i></span>
    <div class="md-qrow"><span>${q.series} von 5 Serien in den letzten 7 Tagen</span></div>
    <span class="md-meter sm" role="img" aria-label="${q.series} von 5 Serien"><i style="width:${Math.min(100, q.series * 20)}%"></i></span></section>`;
}
route('/wishes', () => {
  const ws = mdWishes(); const tab = ui.tab.wishes || 'disc';
  const tabs = `<div class="md-tabs" role="tablist" aria-label="Wünsche">
    <button type="button" role="tab" aria-selected="${tab === 'disc'}" aria-controls="md-wp" data-act="mdWTab" data-v="disc" data-fid="wt-disc">Entdecken</button>
    <button type="button" role="tab" aria-selected="${tab === 'mine'}" aria-controls="md-wp" data-act="mdWTab" data-v="mine" data-fid="wt-mine">Meine Wünsche${ws.length ? ` <span class="md-cnt">${ws.length}</span>` : ''}</button></div>`;
  let body;
  if (tab === 'disc') {
    const films = row('films'); const docs = row('docs');
    const opts = i => { const w = mdWish(i); return { href: `#/wish/${i}`, fid: 'w', tag: w ? esc(MD_WST[w.state][0]) : undefined }; };
    const all = [...films, ...docs];
    body = `<div class="md-srch">${icon('search', 20)}<input class="inp" type="search" id="mdWq" data-bind="ui.draft.wq" value="${esc(ui.draft.wq || '')}" placeholder="Film oder Serie suchen" aria-label="Titel ausserhalb der Bibliothek suchen" autocomplete="off" data-fid="wq"></div>
      <p class="hint md-whint">Titel, die noch nicht in der Bibliothek sind. Dein Wunsch geht an Seerr.</p>
      <div id="mdWrows">
        ${hrow('Im Trend', films.filter(i => T(i).year >= 2016), opts)}
        ${hrow('Klassiker', films.filter(i => T(i).year < 2016), opts)}
        ${hrow('Dokus und Reality', docs, opts)}
      </div>
      <div id="mdWres" hidden><div class="md-sech"><h2>Treffer</h2><span id="mdWn"></span></div>
        <div class="grid">${all.map(i => `<div class="md-wres" data-title="${esc(T(i).title.toLowerCase())}">${card(i, Object.assign(opts(i), { fid: 'ws' }))}</div>`).join('')}</div>
        <div class="empty" id="mdWnone" hidden>${icon('search', 40)}<b>Nichts gefunden</b>Versuche einen anderen Titel.</div></div>`;
  } else {
    body = `${mdQuotaCard()}${ws.length ? `<div class="md-wl">${ws.map(mdWRow).join('')}</div>` : `<div class="empty">${icon('wish', 48)}<b>Noch keine Wünsche</b><p>Fehlt dir ein Film oder eine Serie? Wünsch ihn dir.</p>
      <button type="button" class="btn primary md-emptybtn" data-act="mdWTab" data-v="disc" data-fid="w-empty">${icon('search', 20)} Entdecken</button></div>`}`;
  }
  return { nav: 'wishes', html: `${bar('Wünsche', { back: false })}${tabs}<div id="md-wp" role="tabpanel">${body}</div>`, after: () => { if (tab === 'disc') mdFilter(); } };
});
PAGES.push(['#/wishes', 'Wünsche']);
ACT.mdWTab = d => { ui.tab.wishes = d.v; render(); };
function mdFilter() {
  const inp = $('#mdWq'); if (!inp) return;
  const q = inp.value.trim().toLowerCase(); const rows = $('#mdWrows'); const res = $('#mdWres'); if (!rows || !res) return;
  rows.hidden = !!q; res.hidden = !q; if (!q) return;
  let n = 0; $$('.md-wres', res).forEach(el => { const hit = el.dataset.title.includes(q); el.hidden = !hit; if (hit) n += 1; });
  $('#mdWn').textContent = `${n} Titel`; $('#mdWnone').hidden = n > 0;
}
ACT.mdWDel = d => {
  const i = Number(d.i);
  sheet(`<h2>Wunsch zurückziehen?</h2><p class="md-sp">«${esc(T(i).title)}» wird in Seerr gelöscht. Du kannst den Titel später wieder anfordern.</p>
    <div class="acts"><button type="button" class="btn ghost" data-act="close" data-fid="wdx">Behalten</button>
    <button type="button" class="btn danger" data-act="mdWDelGo" data-i="${i}" data-fid="wdg">Zurückziehen</button></div>`);
};
ACT.mdWDelGo = d => { const i = Number(d.i); S.wishes = mdWishes().filter(w => w.i !== i); closeSheet(true); commit(); toast('Wunsch zurückgezogen'); };

route('/wish/:i', p => {
  const i = Number(p.i); const t = T(i); const d = D(i) || {}; const w = mdWish(i); const q = mdQuota();
  const ser = t.kind === 's'; const full = ser ? q.series >= 5 : q.films >= 10;
  const meta = [t.year, KIND[t.kind], ser ? `${mdSeasons(i)} ${mdSeasons(i) === 1 ? 'Staffel' : 'Staffeln'}` : dur(t.min)].filter(Boolean).map(esc).join(' · ');
  const step = w ? MD_WSTEPS.indexOf(w.state) : -1;
  const steps = w ? `<ol class="md-steps" aria-label="Stand des Wunsches">${MD_WSTEPS.map((s, k) => `<li class="${k < step ? 'done' : k === step ? 'cur' : ''}"${k === step ? ' aria-current="step"' : ''}>${MD_WST[s][0]}</li>`).join('')}</ol>` : '';
  const state = w
    ? `<div class="md-wstate">${steps}
        <p class="md-wline">${mdWBadge(w)} <span>gewünscht ${esc(w.when)}${w.seasons ? ` · Staffel ${w.seasons.join(', ')}` : ''}</span></p>
        ${w.state === 'load' ? `<span class="md-prog"><i data-wpb="${i}" style="width:${Math.max(2, w.pct || 0)}%"></i></span>` : ''}
        <div class="md-wbtns">${w.state === 'done' ? `<a class="btn primary" href="${href(i)}" data-fid="wp-open">${icon('play', 20)} Zur Bibliothek</a>` : ''}
          ${w.state === 'req' || w.state === 'ok' ? `<button type="button" class="btn ghost" data-act="mdWDel" data-i="${i}" data-fid="wp-del">Zurückziehen</button>` : ''}
          <button type="button" class="btn ghost" data-act="mdWMine" data-fid="wp-mine">Meine Wünsche</button></div></div>`
    : `<div class="md-wbtns"><button type="button" class="btn accent" data-act="mdWAsk" data-i="${i}" data-fid="wp-ask"${full ? ' disabled' : ''}>${icon('wish', 20)} Anfordern</button>
        <button type="button" class="btn ghost" data-act="watch" data-i="${i}" data-fid="wp-watch" aria-pressed="${S.watch.includes(i)}">${icon(S.watch.includes(i) ? 'bookmarkOn' : 'bookmark', 20)} Merken</button></div>
       <p class="hint">${full ? 'Dein Kontingent ist aufgebraucht. Es erholt sich in den nächsten Tagen.' : ser ? `${q.series} von 5 Serien in den letzten 7 Tagen` : `${q.films} von 10 Filmen in den letzten 7 Tagen`}</p>`;
  const html = `${bar('', { over: true })}
    <div class="md-wban" aria-hidden="true">${backdrop(i)}</div>
    <div class="md-wgrid">
      <div class="md-wpost">${poster(i)}</div>
      <div class="md-whead"><span class="badge">Nicht in der Bibliothek</span><h1>${esc(t.title)}</h1><p class="md-meta">${meta} ${fsk(t.fsk)}</p>
        ${d.g ? `<p class="md-meta">${esc(d.g.join(' · '))}</p>` : ''}</div>
      <div class="md-wbody">
        ${d.t ? `<p class="md-tag">${esc(d.t)}</p>` : ''}
        <p class="md-over">${esc(d.o || 'Zu diesem Titel gibt es noch keine Inhaltsangabe.')}</p>
        ${state}
      </div>
    </div>`;
  return { nav: 'wishes', html };
});
PAGES.push(['#/wish/79', 'Wunsch · Film «1984»']);
PAGES.push(['#/wish/106', 'Wunsch · Serie «Alexander der Grosse»']);
ACT.mdWMine = () => { ui.tab.wishes = 'mine'; go('#/wishes'); };
ACT.mdWAsk = d => { const i = Number(d.i); ui.wr = { i, seasons: T(i).kind === 's' ? [1] : null }; mdWSheet(); };
function mdWSheet(fid) {
  const r = ui.wr; const t = T(r.i); const q = mdQuota(); const ser = t.kind === 's'; const n = ser ? mdSeasons(r.i) : 0;
  const all = ser && r.seasons.length === n;
  const seasons = ser ? `<div class="md-ssec">Staffeln</div>
    ${mdOpt({ check: true, on: all, title: 'Alle Staffeln', sub: `${n} ${n === 1 ? 'Staffel' : 'Staffeln'}`, attrs: 'data-act="mdWSeason" data-v="all"', fid: 'ws-all' })}
    ${Array.from({ length: n }, (_, k) => mdOpt({ check: true, on: r.seasons.includes(k + 1), title: `Staffel ${k + 1}`, attrs: `data-act="mdWSeason" data-v="${k + 1}"`, fid: `ws-${k + 1}` })).join('')}` : '';
  mdSheet(`<h2>Anfordern</h2>
    <div class="md-dlhead">${mdArt(`p-${r.i}.webp`, t.title, 'tall')}<span class="tx"><b>${esc(t.title)}</b><small>${esc([t.year, KIND[t.kind]].filter(Boolean).join(' · '))}</small></span></div>
    ${seasons}
    <div class="md-sinfo">${icon('info', 20)}<span>${ser ? `Danach ${q.series + 1} von 5 Serien` : `Danach ${q.films + 1} von 10 Filmen`} in den letzten 7 Tagen.</span></div>
    <div class="md-sinfo">${icon('bell', 20)}<span>Seerr bekommt den Wunsch. Ist der Titel da, meldet sich die App.</span></div>
    <div class="acts"><button type="button" class="btn ghost" data-act="close" data-fid="wr-x">Abbrechen</button>
      <button type="button" class="btn accent" data-act="mdWGo" data-fid="wr-go"${ser && !r.seasons.length ? ' disabled' : ''}>${icon('wish', 20)} Anfordern</button></div>`, fid);
}
ACT.mdWSeason = d => {
  const r = ui.wr; const n = mdSeasons(r.i);
  if (d.v === 'all') r.seasons = r.seasons.length === n ? [] : Array.from({ length: n }, (_, k) => k + 1);
  else { const s = Number(d.v); r.seasons = r.seasons.includes(s) ? r.seasons.filter(x => x !== s) : [...r.seasons, s].sort((a, b) => a - b); }
  mdWSheet(`ws-${d.v}`);
};
ACT.mdWGo = () => {
  const r = ui.wr; mdWishes();
  S.wishes.unshift({ i: r.i, state: 'req', when: 'gerade eben', seasons: r.seasons || undefined });
  closeSheet(true); commit(); toast(`«${T(r.i).title}» angefordert`);
};

/* ---------- /notifications ---------- */
route('/notifications', () => {
  const notes = S.notes || []; const unread = notes.filter(n => !n.read).length;
  const ann = S.annSeen ? '' : `<section class="md-ann" aria-label="Ankündigung"><span class="k">${icon('info', 16)} Ankündigung</span>
    <h2>Wartung am Sonntag 02:00 bis 03:00</h2><p>Der Server ist in dieser Zeit nicht erreichbar. Deine Downloads kannst du weiter abspielen.</p>
    <div class="acts"><button type="button" class="btn secondary sm" data-act="mdAnn" data-fid="n-ann">Verstanden</button></div></section>`;
  const list = notes.map((n, k) => `<button type="button" class="md-note${n.read ? '' : ' unread'}" data-act="mdNote" data-k="${k}" data-fid="n-${k}" aria-label="${esc(`${n.read ? '' : 'Neu: '}${n.t}. ${n.b}. ${n.when}`)}">
      ${n.i != null ? mdArt(`p-${n.i}.webp`, '', 'tall') : `<span class="md-th tall md-nic">${icon('bell', 22)}</span>`}
      <span class="tx"><b>${esc(n.t)}</b><span>${esc(n.b)}</span><small>${esc(n.when)}</small></span>${n.read ? '' : '<span class="md-dot" aria-hidden="true"></span>'}</button>`).join('');
  const html = `${bar('Benachrichtigungen')}
    <div class="md-narrow">
      ${ann}
      ${notes.length ? `<div class="md-nhead"><span>${unread ? `${unread} ungelesen` : 'Alles gelesen'}</span>
        <button type="button" class="btn ghost sm" data-act="mdReadAll" data-fid="n-all"${unread ? '' : ' disabled'}>${icon('doneAll', 18)} Alle als gelesen markieren</button></div>
        <div class="md-list md-notes">${list}</div>`
    : `<div class="empty">${icon('bell', 48)}<b>Keine Benachrichtigungen</b>Neue Folgen und erfüllte Wünsche erscheinen hier.</div>`}
      <div class="md-list md-nset">${li('settings', 'Einstellungen für Benachrichtigungen', 'Push, E-Mail und Telegram', undefined, 'href="#/settings/notifications" data-fid="n-set"')}</div>
    </div>`;
  return { nav: 'start', html };
});
PAGES.push(['#/notifications', 'Benachrichtigungen']);
ACT.mdAnn = () => { S.annSeen = true; commit(); toast('Ankündigung ausgeblendet'); };
ACT.mdReadAll = () => { (S.notes || []).forEach(n => { n.read = true; }); commit(); toast('Alle als gelesen markiert'); };
ACT.mdNote = d => {
  const n = (S.notes || [])[Number(d.k)]; if (!n) return;
  n.read = true; save();
  if (n.i != null) go(href(n.i)); else render();
};

/* ---------- one clock for playback, downloads and wishes ---------- */
let mdTickN = 0;
function mdAdvance(p) {
  const before = mdCur(p).k;
  p.pos = Math.min(p.total, p.pos + (p.speed || 1));
  let stop = '';
  if (typeof p.sleep === 'number') { p.sleep -= 1; if (p.sleep <= 0) { p.sleep = null; stop = 'Schlaftimer abgelaufen · pausiert'; } }
  const after = mdCur(p).k;
  if (before !== after && p.sleep === 'end') { p.sleep = null; stop = 'Ende des Kapitels · pausiert'; }
  if (p.pos >= p.total) {
    p.sleep = null; stop = p.kind === 'pod' ? 'Folge zu Ende' : 'Hörbuch zu Ende';
    if (p.kind === 'pod') S.podPlayed = Object.assign({}, S.podPlayed, { [`${p.i}-${p.e}`]: true });
  }
  mdPersist(p);
  if (stop) p.paused = true;
  if (before !== after || stop) {
    mdSubLine(p); save();
    if (mdPath().startsWith('/listen/') || mdPath().startsWith('/book/') || mdPath().startsWith('/podcast/')) render(); else drawMini();
    if (stop) toast(stop);
    return;
  }
  mdLsDom(p);
}
function mdDlTick() {
  if ((ui.dlVar || 'normal') !== 'normal' || !Array.isArray(S.downloads)) return;
  const x = S.downloads.find(y => y.state === 'run'); if (!x) return;
  x.pct = Math.min(100, x.pct + Math.max(1.2, Math.min(6, 100 / 15 / Math.max(0.1, x.gb))));
  if (x.pct >= 100) {
    x.pct = 100; x.state = 'done'; mdNext(); commit();
    toast(`«${T(x.i).kind === 's' ? `${T(x.i).title} · ${mdItem(x.i, x.e, x.s).sub.split(' · ').slice(0, 2).join(' · ')}` : mdItem(x.i, x.e).title}» ist auf dem Gerät`);
    return;
  }
  const b = $(`[data-dlbar="${x.id}"]`); if (b) b.style.width = `${x.pct}%`;
  const tx = $(`[data-dltx="${x.id}"]`); if (tx) tx.textContent = `${Math.floor(x.pct)} % · ${mdSize((x.gb * x.pct) / 100)} von ${mdSize(x.gb)}`;
  if (mdTickN % 5 === 0) save();
}
function mdWishTick() {
  if (!Array.isArray(S.wishes)) return;
  S.wishes.filter(w => w.state === 'load').forEach(w => {
    w.pct = Math.min(100, (w.pct || 0) + 0.8);
    if (w.pct >= 100) {
      w.state = 'done';
      S.notes = [{ t: 'Wunsch erfüllt', b: `${T(w.i).title} ist jetzt in der Bibliothek`, i: w.i, when: 'gerade eben', read: false }, ...(S.notes || [])];
      commit(); toast(`«${T(w.i).title}» ist jetzt verfügbar`);
      return;
    }
    $$(`[data-wpb="${w.i}"]`).forEach(el => { el.style.width = `${w.pct}%`; });
    $$(`[data-wpt="${w.i}"]`).forEach(el => { el.textContent = Math.floor(w.pct); });
  });
}
setInterval(() => {
  mdTickN += 1;
  const p = S.playing; const path = mdPath();
  if (p && !p.paused && (p.kind === 'book' || p.kind === 'pod')) mdAdvance(p);
  if (p && !p.paused && p.kind === 'radio' && mdTickN % 15 === 0) {
    p.song = (p.song || 0) + 1; p.sub = `Live · ${mdSong(p.id, p.song)}`; save(); drawMini();
    $$('[data-np]').forEach(el => { el.textContent = mdSong(p.id, p.song); });
  }
  if (path === '/downloads') mdDlTick();
  if (path === '/wishes' || path.startsWith('/wish/')) mdWishTick();
  if (p && mdTickN % 5 === 0) save();
}, 1000);
