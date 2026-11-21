/* Pages of media.js: audiobooks, podcasts, the audio player, webradio, wishes, notifications, settings.
   Routes: /book/:i, /podcast/:i, /listen/:i, /radio, /wishes, /wish/:i, /notifications, /settings,
   /settings/profiles, /settings/playback. Global names of this file carry the prefix md. */
'use strict';

/* ---------- helpers ---------- */
const MD_MON = ['Januar', 'Februar', 'März', 'April', 'Mai', 'Juni', 'Juli', 'August', 'September', 'Oktober', 'November', 'Dezember'];
const mdRand = seed => { let s = (seed >>> 0) || 1; return () => { s = (Math.imul(s, 1664525) + 1013904223) >>> 0; return s / 4294967296; }; };
const mdPad = n => String(n).padStart(2, '0');
const mdClock = sec => { const v = Math.max(0, Math.round(sec)); const h = Math.floor(v / 3600); const m = Math.floor((v % 3600) / 60); const s = v % 60; return h ? `${h}:${mdPad(m)}:${mdPad(s)}` : `${m}:${mdPad(s)}`; };
const mdLeft = sec => { const m = Math.max(1, Math.round(sec / 60)); return m >= 60 ? `${Math.floor(m / 60)} Std. ${m % 60} Min.` : `${m} Min.`; };
const mdEq = paused => `<span class="md-eq${paused ? ' paused' : ''}" aria-hidden="true"><i></i><i></i><i></i></span>`;
const mdFsk = n => (n ? `ab ${n}` : 'ab 0');
const MD_LOGOS = [39211, 38967]; /* stations with a logo file r-<id>.webp */
const MD_APP_VERSION = '1.0.0';
const mdServerVersion = () => (typeof SU_VERSION !== 'undefined' ? SU_VERSION : '1.41.3');

/* volatile state of this file */
const MDL = { calm: false, side: null };
const MDA = { n: 0, prof: null, sig: '' };
const MDPIN = { v: '', first: null, o: null };
RESETS.push(() => {
  MDL.calm = false; MDA.sig = ''; MDA.prof = null;
  delete ui.tab.wishes; delete ui.mdPE; delete ui.mdReq;
});

/* ---------- audiobooks: chapters ---------- */
const mdChapCache = {};
/** 12 to 20 chapters with believable lengths that add up to the duration of the title */
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
const MD_BOOK0 = { 32: Math.round(964 * 60 * 0.38) };
function mdBookPos(i) {
  const p = mdNow(); if (p && p.kind === 'book' && p.i === i) return p.pos;
  const m = S.mdBookPos || {}; return m[i] != null ? m[i] : (MD_BOOK0[i] || 0);
}

/* ---------- podcasts: generated episodes that fit the show ---------- */
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
const mdPodCache = {};
/** eight weekly episodes, newest first (k = 0 is the newest) */
function mdPodEps(i) {
  if (mdPodCache[i]) return mdPodCache[i];
  const r = mdRand(i * 4421 + 7); const off = i % MD_POD_ANY.length;
  const list = MD_POD[i] || Array.from({ length: 8 }, (_, k) => MD_POD_ANY[(off + k * 3) % MD_POD_ANY.length]);
  const base = MD_POD_MIN[i] || 38;
  mdPodCache[i] = list.map((title, k) => {
    const d = new Date(2026, 10, 17 - k * 7);
    return { k, n: k + 1, title, min: Math.max(1, Math.round(base * (0.75 + r() * 0.5))), date: `${d.getDate()}. ${MD_MON[d.getMonth()]} ${d.getFullYear()}` };
  });
  return mdPodCache[i];
}
const mdPK = (i, k) => `${i}-${k}`;
const MD_POD0 = { '56-1': 0.42, '58-0': 0.18 };
function mdPlayed(i, k) { const m = S.mdPodPlayed || {}; const v = m[mdPK(i, k)]; return v == null ? k >= 3 : v; }
function mdPodPos(i, k) {
  const p = mdNow(); if (p && p.kind === 'pod' && p.i === i && p.e === k) return p.pos;
  const m = S.mdPodPos || {}; const v = m[mdPK(i, k)];
  if (v != null) return v;
  const f = MD_POD0[mdPK(i, k)]; return f ? Math.round(mdPodEps(i)[k].min * 60 * f) : 0;
}
const mdSubs = () => S.mdSubs ?? [56, 58, 64, 66, 97];

/* ---------- playback state: S.playing = { kind: book | pod | radio, i, e, id, pos, paused, song, songAt } ---------- */
const mdNow = () => (S.playing && ['book', 'pod', 'radio'].includes(S.playing.kind) ? S.playing : null);
const mdSpeed = () => S.mdSpeed ?? 1;
const mdSpeedTx = v => `${String(v).replace('.', ',')}×`;
/** remember the position of what plays now before something else starts */
function mdStash() {
  const p = mdNow(); if (!p) return;
  if (p.kind === 'book') S.mdBookPos = Object.assign({}, S.mdBookPos, { [p.i]: p.pos });
  if (p.kind === 'pod') {
    S.mdPodPos = Object.assign({}, S.mdPodPos, { [mdPK(p.i, p.e)]: p.pos });
    const len = (mdPodEps(p.i)[p.e] || { min: 1 }).min * 60;
    if (p.pos >= len * 0.95) S.mdPodPlayed = Object.assign({}, S.mdPodPlayed, { [mdPK(p.i, p.e)]: true });
  }
}
function mdStart(o) { mdStash(); S.playing = Object.assign({ paused: false, pos: 0 }, o); save(); mdSync(); }
function mdBookStart(i, pos) { const total = T(i).min * 60; mdStart({ kind: 'book', i, pos: pos >= total - 5 ? 0 : pos }); }
function mdPodStart(i, k) {
  const len = mdPodEps(i)[k].min * 60; const pos = mdPodPos(i, k);
  mdStart({ kind: 'pod', i, e: k, pos: pos >= len - 5 ? 0 : pos });
}

/* model of the audio player for what plays now */
function mdLsModel() {
  const p = mdNow(); if (!p || p.kind === 'radio') return null;
  const t = T(p.i);
  if (p.kind === 'book') {
    const ch = mdChapters(p.i); const c = mdChapAt(ch, p.pos); const total = t.min * 60;
    return { p, t, pod: false, list: ch, cur: c, rel: p.pos - c.start, len: c.len, kind: `Hörbuch · Kapitel ${c.n} von ${ch.length}`, line: c.title, rest: total - p.pos };
  }
  const eps = mdPodEps(p.i); const e = eps[p.e] || eps[0]; const len = e.min * 60;
  return { p, t, pod: true, list: eps, cur: e, rel: p.pos, len, kind: `Podcast · ${e.date}`, line: e.title, rest: len - p.pos };
}

/* ---------- «Läuft gerade»: a small indicator and an entry in the rail ---------- */
const MD_SONGS = {
  39211: ['Lewis Capaldi – Someone You Loved', 'Adele – Easy On Me', 'Ed Sheeran – Perfect', 'Coldplay – Viva La Vida', 'Bastian Baker – Lucky'],
  38967: ['Hecht – Kawasaki', 'Dua Lipa – Houdini', 'Arctic Monkeys – Do I Wanna Know?', 'Stephan Eicher – Déjeuner en paix', 'Sophie Hunger – Le vent nous portera'],
  40241: ['Bligg – Signal', 'Gölä – Schwan', 'Trauffer – Müeh mit de Chüeh', 'Helene Fischer – Atemlos durch die Nacht', 'Lo & Leduc – 079'],
};
const mdStation = id => DATA.radio.find(r => r[0] === id);
const mdSong = p => { const l = MD_SONGS[p.id] || ['Live']; return l[(p.song || 0) % l.length]; };
/* small spots (indicator, now-playing bar) show a radio symbol where a station has no logo */
const mdLogo = (id, name, small) => (MD_LOGOS.includes(id) ? pic(`r-${id}.webp`, name) : small ? `<span class="md-rname ic" aria-label="${esc(name)}">${icon('radio', 40)}</span>` : `<span class="md-rname">${esc(name)}</span>`);
const mdPill = document.createElement('div');
mdPill.id = 'md-now'; mdPill.hidden = true; mdPill.setAttribute('aria-live', 'polite');
$('#app').appendChild(mdPill);
function mdSync() {
  const p = mdNow(); const path = curPath();
  const hide = !p || path.startsWith('/listen/') || (p.kind === 'radio' && path === '/radio');
  mdPill.hidden = hide;
  if (!hide) {
    let sig; let html;
    if (p.kind === 'radio') {
      const st = mdStation(p.id) || [p.id, 'Webradio'];
      sig = `r|${p.id}|${p.song}`;
      html = `<span class="md-now-im sqr">${mdLogo(p.id, st[1], true)}</span><span class="md-now-tx"><small>${mdEq(false)} Live · ${esc(st[1])}</small><b>${esc(mdSong(p))}</b></span>`;
    } else {
      sig = `${p.kind}|${p.i}|${p.e}|${p.paused}`;
      html = `<span class="md-now-im sqr">${poster(p.i)}</span><span class="md-now-tx"><small>${mdEq(p.paused)} ${p.paused ? 'Pausiert' : 'Läuft gerade'}</small><b>${esc(T(p.i).title)}</b></span>`;
    }
    if (sig !== MDA.sig) { MDA.sig = sig; mdPill.innerHTML = html; }
  } else MDA.sig = '';
  mdRailNow();
}
function mdRailNow() {
  const box = $('#rail .r-items'); if (!box) return;
  const old = $('[data-f="r-now"]', box); const p = mdNow();
  const want = p && !curPath().startsWith('/listen/');
  if (!want) { if (old && F.el !== old) old.remove(); return; }
  const h = p.kind === 'radio' ? '#/radio' : `#/listen/${p.i}`;
  const lab = p.kind === 'radio' ? 'Radio läuft' : p.paused ? 'Pausiert' : 'Läuft gerade';
  const inner = `<span class="r-ic">${mdEq(p.kind !== 'radio' && p.paused)}</span><span class="r-lb">${lab}</span>`;
  if (old) { if (old.getAttribute('href') !== h || old.dataset.lab !== lab) { old.setAttribute('href', h); old.dataset.lab = lab; old.innerHTML = inner; } return; }
  box.insertAdjacentHTML('beforeend', `<a class="r-it md-rnow" href="${h}" data-f="r-now" data-lab="${lab}" aria-label="${lab}: ${esc(p.kind === 'radio' ? (mdStation(p.id) || [0, ''])[1] : T(p.i).title)}">${inner}</a>`);
}
new MutationObserver(mdRailNow).observe($('#rail'), { childList: true });
window.addEventListener('hashchange', () => {
  /* a video starts: the audio pauses; then the indicator follows the new page */
  const p = mdNow();
  if (p && curPath().startsWith('/play/')) { if (p.kind === 'radio') S.playing = null; else p.paused = true; save(); }
  mdSync();
});

/* one tick per second: playback, radio titles, wishes, the calm audio screen */
function mdTick() {
  const p = mdNow();
  if (MDA.prof != null && MDA.prof !== S.profile && p) { mdStash(); S.playing = null; save(); }
  MDA.prof = S.profile;
  if (p && OFF && !p.paused) { if (p.kind === 'radio') S.playing = null; else p.paused = true; save(); }
  const q = mdNow();
  if (q && !q.paused) {
    if (q.kind === 'radio') {
      if (Date.now() - (q.songAt || 0) > 25000) { q.song = (q.song || 0) + 1; q.songAt = Date.now(); mdRadioLive(); }
    } else {
      const m = mdLsModel(); const end = q.kind === 'book' ? T(q.i).min * 60 : m.len;
      q.pos = Math.min(end, q.pos + mdSpeed());
      if (q.pos >= end) {
        q.paused = true;
        if (q.kind === 'pod') S.mdPodPlayed = Object.assign({}, S.mdPodPlayed, { [mdPK(q.i, q.e)]: true });
        save(); toast(q.kind === 'pod' ? 'Folge zu Ende' : 'Hörbuch zu Ende');
      }
    }
    MDA.n += 1; if (MDA.n % 5 === 0) save();
  }
  mdWishTick();
  if (curPath().startsWith('/listen/')) {
    mdLsUpdate();
    if (MDL.calm && Date.now() - lastInput < 1500) mdWake();
    if (!MDL.calm && !OV.length && !OFF && Date.now() - lastInput > 12000) { MDL.calm = true; const b = $('#md-ls'); if (b) b.classList.add('calm'); }
  }
  mdSync();
}
setInterval(mdTick, 1000);

/* ---------- focus guards for the pages with the rail ---------- */
/** Up or Down at the edge of a page bumps instead of jumping diagonally into the rail. */
function mdGuard(name) {
  if ((name !== 'up' && name !== 'down') || OV.length || !F.el || F.el.closest('#rail')) return false;
  const t = pick(F.el, name, focusables(scopeEl()));
  if (t && t.closest('#rail')) { bump(F.el, name); return true; }
  return false;
}
/** detail pages: the left column (cover, buttons) and the list on the right are two zones */
function mdDetKey(name) {
  if (mdGuard(name)) return true;
  const el = F.el; if (!el || OV.length) return false;
  const main = el.closest('.md-main'); const side = el.closest('.md-side');
  if (main && name === 'left' && !pick(el, 'left', focusables(main))) {
    const back = byF(MDL.side); focus(usable(back) && back.closest('.md-side') ? back : defaultIn($('.md-side'))); return true;
  }
  if (side && (name === 'up' || name === 'down') && !pick(el, name, focusables(side))) { bump(el, name); return true; }
  return false;
}
const mdDetFocus = el => { if (el.closest('.md-side')) MDL.side = el.dataset.f; };

/* ---------- /book/:i ---------- */
function mdAmb(i) { return `<div class="md-amb" aria-hidden="true">${pic(`p-${i}.webp`, '')}</div>`; }
function mdWatchBtn(i, f) {
  const on = S.watch.includes(i);
  return `<button type="button" class="tb" data-f="${f}" data-act="watch" data-i="${i}" aria-pressed="${on}">${icon(on ? 'bookmarkOn' : 'bookmark', 34)}<span>${on ? 'Gemerkt' : 'Merken'}</span></button>`;
}
route('/book/:i', p => {
  const i = Number(p.i); if (!DATA.T[i] || T(i).kind !== 'a') return missing(curPath());
  const t = T(i); const ch = mdChapters(i); const total = t.min * 60; const pos = mdBookPos(i);
  const n = mdNow(); const live = n && n.kind === 'book' && n.i === i;
  const cur = mdChapAt(ch, pos); const pct = Math.round((pos / total) * 100); const done = pos >= total - 5;
  const started = pos > 0 && !done;
  const label = live ? 'Zum Player' : done ? 'Nochmals anhören' : started ? 'Weiterhören' : 'Anhören';
  const prog = started || live ? `<div class="md-bprog"><span class="md-bar"><i style="width:${Math.max(2, pct)}%"></i></span><span>${pct} % · noch ${mdLeft(total - pos)}</span></div>`
    : done ? `<div class="md-bprog"><span class="md-bar"><i style="width:100%"></i></span><span>Gehört</span></div>` : '';
  const rows = ch.map(c => {
    const isCur = (started || live) && c.k === cur.k; const heard = c.start + c.len <= pos;
    const st = isCur ? `${live && !n.paused ? 'läuft' : 'angefangen'} · noch ${mdLeft(c.start + c.len - pos)}` : heard ? 'gehört' : '';
    return `<button type="button" class="md-li${isCur ? ' cur' : ''}${heard ? ' heard' : ''}" data-f="ch-${c.n}" data-act="mdChap" data-i="${i}" data-c="${c.n}">
      <span class="md-li-n">${isCur ? mdEq(!live || n.paused) : heard ? icon('check', 30) : c.n}</span>
      <span class="md-li-tx"><b>${esc(c.title)}</b>${st ? `<small>${st}</small>` : ''}</span><span class="md-li-d">${mdClock(c.len)}</span></button>`;
  }).join('');
  const more = t.author ? DATA.T.map((x, j) => j).filter(j => j !== i && DATA.T[j][0] === 'a' && DATA.T[j][5] === t.author) : [];
  const moreHtml = more.length ? `<section class="md-more" data-info-sec><h2 class="md-sech">Mehr von ${esc(t.author)}</h2>
    <div class="md-mrow" data-row="more">${more.map(j => card(j, { sq: true, row: 'more', sub: `${T(j).year} · ${dur(T(j).min)}` })).join('')}</div><div class="rinfo" aria-live="polite"></div></section>` : '';
  return {
    nav: 'libs',
    html: `<div class="page md-det">${mdAmb(i)}
      <aside class="md-side">
        <div class="md-cover">${poster(i)}</div>
        ${prog}
        <div class="md-acts">
          <button type="button" class="tb primary" data-f="bk-play" data-act="mdListen" data-i="${i}" data-default>${icon('play', 34)}<span>${label}</span></button>
          ${mdWatchBtn(i, 'bk-watch')}
        </div>
      </aside>
      <div class="md-main">
        <p class="md-kicker">${icon('headphones', 30)} Hörbuch</p>
        <h1 class="md-h1">${esc(t.title)}</h1>
        <p class="md-by">${esc(t.author)}</p>
        <p class="md-meta">${[t.year, dur(t.min), `${ch.length} Kapitel`].filter(Boolean).join(' · ')}</p>
        <h2 class="md-sech">Kapitel</h2>
        <div class="md-list" data-row="chap">${rows}</div>
        ${moreHtml}
      </div></div>`,
    after: () => {
      const m = MEM[curPath()]; if (m && !m.rows.chap) m.rows.chap = `ch-${started || live ? cur.n : 1}`;
    },
    onFocus: mdDetFocus,
    key: name => { if (name === 'play') { ACT.mdListen({ i }); return true; } return mdDetKey(name); },
  };
});
ACT.mdListen = d => {
  const i = Number(d.i); const n = mdNow();
  if (n && n.kind === 'book' && n.i === i) n.paused = false; else mdBookStart(i, mdBookPos(i));
  save(); go(`#/listen/${i}`);
};
ACT.mdChap = d => {
  const i = Number(d.i); const c = mdChapters(i)[Number(d.c) - 1];
  mdStart({ kind: 'book', i, pos: c.start }); go(`#/listen/${i}?c=${c.n}`);
};

/* ---------- /podcast/:i ---------- */
route('/podcast/:i', p => {
  const i = Number(p.i); if (!DATA.T[i] || T(i).kind !== 'p') return missing(curPath());
  const t = T(i); const eps = mdPodEps(i); const n = mdNow(); const sub = mdSubs().includes(i);
  const live = k => n && n.kind === 'pod' && n.i === i && n.e === k;
  const resume = eps.find(e => !mdPlayed(i, e.k) && mdPodPos(i, e.k) > 0);
  const first = resume || eps.find(e => !mdPlayed(i, e.k)) || eps[0];
  const rows = eps.map(e => {
    const pos = mdPodPos(i, e.k); const len = e.min * 60; const played = mdPlayed(i, e.k); const on = live(e.k);
    const st = on ? (n.paused ? `pausiert · noch ${mdLeft(len - pos)}` : `läuft · noch ${mdLeft(len - pos)}`) : played ? 'gehört' : pos > 0 ? `noch ${mdLeft(len - pos)}` : 'neu';
    return `<button type="button" class="md-li ep${played && !on ? ' heard' : ''}${on ? ' cur' : ''}" data-f="ep-${e.k}" data-act="mdEp" data-i="${i}" data-e="${e.k}">
      <span class="md-li-n">${on ? mdEq(n.paused) : played ? icon('check', 30) : '<i class="md-new" aria-hidden="true"></i>'}</span>
      <span class="md-li-tx"><b>${esc(e.title)}</b><small>${esc(e.date)} · ${st}</small>${pos > 0 && !played ? `<span class="md-bar sm"><i style="width:${Math.max(3, (pos / len) * 100)}%"></i></span>` : ''}</span>
      <span class="md-li-d">${e.min} Min.</span></button>`;
  }).join('');
  return {
    nav: 'libs',
    html: `<div class="page md-det">${mdAmb(i)}
      <aside class="md-side">
        <div class="md-cover">${poster(i)}</div>
        <div class="md-acts">
          <button type="button" class="tb primary" data-f="pd-play" data-act="mdEp" data-i="${i}" data-e="${first.k}" data-default>${icon('play', 34)}<span>${resume ? 'Weiterhören' : 'Neueste Folge'}</span></button>
          <button type="button" class="tb" data-f="pd-sub" data-act="mdSub" data-i="${i}" aria-pressed="${sub}">${icon(sub ? 'check' : 'plus', 34)}<span>${sub ? 'Abonniert' : 'Abonnieren'}</span></button>
        </div>
      </aside>
      <div class="md-main">
        <p class="md-kicker">${icon('mic', 30)} Podcast</p>
        <h1 class="md-h1">${esc(t.title)}</h1>
        ${t.author ? `<p class="md-by">${esc(t.author)}</p>` : ''}
        <p class="md-meta">${eps.length} Folgen · jede Woche neu${t.fsk ? ` · ${mdFsk(t.fsk)}` : ''}</p>
        <h2 class="md-sech">Folgen</h2>
        <div class="md-list" data-row="eps">${rows}</div>
      </div></div>`,
    onFocus: mdDetFocus,
    key: name => {
      if (name !== 'play') return mdDetKey(name);
      const e = F.el && F.el.dataset.e != null ? Number(F.el.dataset.e) : first.k; ACT.mdEp({ i, e }); return true;
    },
  };
});
ACT.mdEp = d => {
  const i = Number(d.i); const k = Number(d.e); const n = mdNow();
  if (n && n.kind === 'pod' && n.i === i && n.e === k) n.paused = false; else mdPodStart(i, k);
  save(); go(`#/listen/${i}?e=${k}`);
};
ACT.mdSub = d => {
  const i = Number(d.i); const l = mdSubs().slice(); const k = l.indexOf(i);
  if (k < 0) l.push(i); else l.splice(k, 1); S.mdSubs = l; commit();
  toast(k < 0 ? 'Abonniert. Neue Folgen erscheinen auf der Startseite.' : 'Abo beendet');
};

/* ---------- /listen/:i  the audio player ---------- */
route('/listen/:i', (p, q) => {
  const i = Number(p.i); if (!DATA.T[i] || !['a', 'p'].includes(T(i).kind)) return missing(curPath());
  if (lastPath !== curPath()) MDL.calm = false;
  const n = mdNow(); const pod = T(i).kind === 'p';
  if (!n || n.kind === 'radio' || n.i !== i) {
    /* opened directly (jump menu): start at the chapter or episode of the address, else where it stopped */
    if (pod) mdPodStart(i, Math.min(7, Math.max(0, Number(q.e) || 0)));
    else { const c = q.c ? mdChapters(i)[Number(q.c) - 1] : null; mdBookStart(i, c ? c.start : mdBookPos(i)); }
  }
  const m = mdLsModel(); const t = m.t; const pct = m.len ? (m.rel / m.len) * 100 : 0;
  const cb = (f, act, ic, lab, extra) => `<button type="button" class="md-cb" data-f="${f}" data-act="${act}" aria-label="${lab}"${extra || ''}>${ic}</button>`;
  return {
    full: true, cls: 'md-lsv',
    html: `<div class="md-ls${MDL.calm ? ' calm' : ''}" id="md-ls">
      <div class="md-ls-bg" aria-hidden="true">${pic(`p-${i}.webp`, '')}</div>
      <div class="md-ls-main">
        <div class="md-ls-cover">${poster(i)}</div>
        <div class="md-ls-info">
          <p class="md-kicker" id="ls-kind">${esc(m.kind)}</p>
          <h1 class="md-ls-title">${esc(t.title)}</h1>
          ${t.author ? `<p class="md-ls-by">${esc(t.author)}</p>` : ''}
          <p class="md-ls-line">${icon(pod ? 'mic' : 'book', 32)}<span id="ls-line">${esc(m.line)}</span></p>
          <button type="button" class="md-seek" data-f="ls-seek" role="slider" aria-label="${pod ? 'Position in der Folge' : 'Position im Kapitel'}" aria-valuetext="${mdClock(m.rel)} von ${mdClock(m.len)}">
            <span class="md-track"><i id="ls-fill" style="width:${pct}%"></i><b id="ls-knob" style="left:${pct}%"></b></span></button>
          <div class="md-times"><span id="ls-pos">${mdClock(m.rel)}</span><span class="md-seekhint">${icon('left', 26)}${icon('right', 26)} 30 Sekunden springen</span><span id="ls-left">−${mdClock(m.len - m.rel)}</span></div>
          <div class="md-ctl">
            ${cb('ls-prev', 'mdPrev', icon('prev', 44), pod ? 'Vorherige Folge' : 'Kapitelanfang oder vorheriges Kapitel')}
            ${cb('ls-rew', 'mdRew', `${icon('rew', 52)}<span class="md-cbn">30</span>`, '30 Sekunden zurück')}
            <button type="button" class="md-cb play" data-f="ls-play" data-act="mdPP" data-default data-state="${m.p.paused ? 'paused' : 'on'}" aria-label="${m.p.paused ? 'Wiedergabe' : 'Pause'}">${icon(m.p.paused ? 'play' : 'pause', 60)}</button>
            ${cb('ls-fwd', 'mdFwd', `${icon('fwd', 52)}<span class="md-cbn">30</span>`, '30 Sekunden vor')}
            ${cb('ls-next', 'mdNext', icon('next', 44), pod ? 'Nächste Folge' : 'Nächstes Kapitel')}
          </div>
          <div class="md-ctl2">
            <button type="button" class="tb sm" data-f="ls-speed" data-act="mdSpeedMenu">${icon('speed', 30)}<span>Tempo <b id="ls-speed">${mdSpeedTx(mdSpeed())}</b></span></button>
            <button type="button" class="tb sm" data-f="ls-list" data-act="mdLsList">${icon('list', 30)}<span>${pod ? 'Folgen' : 'Kapitel'}</span></button>
          </div>
        </div>
      </div>
      <div class="md-calm" aria-hidden="true"><div class="md-calm-in">
        <div class="md-calm-cover">${poster(i)}</div>
        <div class="md-calm-tx"><span class="md-calm-st">${mdEq(m.p.paused)}<span id="lc-st">${m.p.paused ? 'Pausiert' : 'Läuft gerade'}</span></span>
          <b>${esc(t.title)}</b><span id="lc-line">${esc(m.line)} · noch ${mdLeft(m.len - m.rel)}</span>
          <span class="md-bar"><i id="lc-fill" style="width:${pct}%"></i></span></div></div></div>
      <p class="mockhint md-ls-hint"><span class="mk">Nur im Mockup</span> Nach 12 s ohne Taste wird der Bildschirm ruhig. Danach darf der Bildschirmschoner des Fernsehers kommen.</p>
    </div>`,
    key: name => {
      if (MDL.calm) { mdWake(); return true; }
      if (name === 'play') { ACT.mdPP(); return true; }
      if ((name === 'left' || name === 'right') && F.el && F.el.dataset.f === 'ls-seek') { mdSeek(name === 'left' ? -30 : 30); return true; }
      return false;
    },
  };
});
function mdWake() { MDL.calm = false; const b = $('#md-ls'); if (b) b.classList.remove('calm'); }
function mdLsUpdate() {
  const box = $('#md-ls'); const m = mdLsModel(); if (!box || !m) return;
  const pct = m.len ? Math.min(100, (m.rel / m.len) * 100) : 0; const set = (id, v) => { const e = $('#' + id); if (e && e.textContent !== v) e.textContent = v; };
  set('ls-kind', m.kind); set('ls-line', m.line); set('ls-pos', mdClock(m.rel)); set('ls-left', `−${mdClock(m.len - m.rel)}`);
  set('lc-line', `${m.line} · noch ${mdLeft(m.len - m.rel)}`); set('lc-st', m.p.paused ? 'Pausiert' : 'Läuft gerade'); set('ls-speed', mdSpeedTx(mdSpeed()));
  ['ls-fill', 'lc-fill'].forEach(id => { const e = $('#' + id); if (e) e.style.width = `${pct}%`; });
  const k = $('#ls-knob'); if (k) k.style.left = `${pct}%`;
  const s = byF('ls-seek'); if (s) s.setAttribute('aria-valuetext', `${mdClock(m.rel)} von ${mdClock(m.len)}`);
  $$('.md-ls .md-eq').forEach(e => e.classList.toggle('paused', !!m.p.paused));
  const b = byF('ls-play'); const st = m.p.paused ? 'paused' : 'on';
  if (b && b.dataset.state !== st) { b.dataset.state = st; b.innerHTML = icon(m.p.paused ? 'play' : 'pause', 60); b.setAttribute('aria-label', m.p.paused ? 'Wiedergabe' : 'Pause'); }
}
function mdSeek(delta) {
  const m = mdLsModel(); if (!m) return;
  const end = m.pod ? m.len : m.t.min * 60;
  m.p.pos = Math.max(0, Math.min(end - 1, m.p.pos + delta)); save(); mdLsUpdate();
}
/** podcasts: next is the newer episode (k - 1), previous the older one */
function mdPodGo(k) {
  const p = mdNow(); const eps = mdPodEps(p.i);
  if (k < 0) { toast('Das ist die neueste Folge'); return; }
  if (k >= eps.length) { toast('Das ist die älteste Folge'); return; }
  mdPodStart(p.i, k); save(); mdLsUpdate(); toast(eps[k].title);
}
ACT.mdPP = () => { const p = mdNow(); if (!p) return; p.paused = !p.paused; save(); mdLsUpdate(); mdSync(); };
ACT.mdRew = () => mdSeek(-30);
ACT.mdFwd = () => mdSeek(30);
ACT.mdPrev = () => {
  const m = mdLsModel(); if (!m) return;
  if (m.pod) { if (m.rel > 5) { m.p.pos = 0; save(); mdLsUpdate(); } else mdPodGo(m.p.e + 1); return; }
  if (m.rel > 5 || m.cur.k === 0) m.p.pos = m.cur.start; else m.p.pos = m.list[m.cur.k - 1].start;
  save(); mdLsUpdate();
};
ACT.mdNext = () => {
  const m = mdLsModel(); if (!m) return;
  if (m.pod) { mdPodGo(m.p.e - 1); return; }
  if (m.cur.k >= m.list.length - 1) { toast('Das ist das letzte Kapitel'); return; }
  m.p.pos = m.list[m.cur.k + 1].start; save(); mdLsUpdate();
};
ACT.mdSpeedMenu = () => {
  const cur = mdSpeed();
  menu(`<h2>Tempo</h2><div class="mlist">${[0.75, 1, 1.25, 1.5, 1.75, 2].map(v => `<button type="button" class="mi" role="menuitemradio" aria-checked="${v === cur}" data-f="sp-${String(v).replace('.', '')}" data-act="mdSpeed" data-v="${v}"${v === cur ? ' data-default' : ''}>
    <span class="mi-tx"><b>${mdSpeedTx(v)}</b>${v === 1 ? '<small>normal</small>' : ''}</span>${v === cur ? icon('check', 36) : ''}</button>`).join('')}</div>`);
};
ACT.mdSpeed = d => { S.mdSpeed = Number(d.v); save(); closeMenu(); mdLsUpdate(); toast(`Tempo ${mdSpeedTx(S.mdSpeed)}`); };
ACT.mdLsList = () => {
  const m = mdLsModel(); if (!m) return;
  const items = m.list.map(x => {
    const cur = x.k === m.cur.k;
    const sub = m.pod ? `${x.date} · ${x.min} Min.${mdPlayed(m.p.i, x.k) && !cur ? ' · gehört' : ''}` : `${mdClock(x.len)}${cur ? ` · noch ${mdLeft(x.start + x.len - m.p.pos)}` : x.start + x.len <= m.p.pos ? ' · gehört' : ''}`;
    return `<button type="button" class="mi" data-f="ll-${x.k}" data-act="mdLsPick" data-k="${x.k}"${cur ? ' data-default aria-current="true"' : ''}>
      <span class="mi-ic">${cur ? mdEq(m.p.paused) : m.pod ? icon('mic', 32) : `<b class="md-mi-n">${x.n}</b>`}</span>
      <span class="mi-tx"><b>${esc(x.title)}</b><small>${esc(sub)}</small></span></button>`;
  }).join('');
  menu(`<h2>${m.pod ? 'Folgen' : 'Kapitel'}</h2><div class="mlist">${items}</div>`, { cls: 'md-wide' });
};
ACT.mdLsPick = d => {
  const m = mdLsModel(); if (!m) return; const k = Number(d.k);
  closeMenu();
  if (m.pod && k !== m.p.e) mdPodGo(k); else if (!m.pod) m.p.pos = m.list[k].start;
  const n = mdNow(); if (n) n.paused = false;
  save(); mdLsUpdate(); mdSync();
};

/* ---------- /radio ---------- */
const mdFavs = () => S.mdRadioFav ?? [38967];
function mdRadioLive() {
  const p = mdNow(); if (!p || p.kind !== 'radio') return;
  const e = $('#md-rsong'); if (e) { e.textContent = mdSong(p); e.classList.remove('swap'); void e.offsetWidth; e.classList.add('swap'); }
  const s = $(`[data-rsub="${p.id}"]`); if (s) s.textContent = mdSong(p);
}
route('/radio', () => {
  const p = mdNow(); const on = p && p.kind === 'radio' ? p.id : null; const favs = mdFavs();
  const def = on || DATA.radio[0][0];
  const tiles = DATA.radio.map(([id, name, land, style]) => {
    const fav = favs.includes(id); const live = on === id;
    return `<div class="md-rcol">
      <button type="button" class="md-rtile${live ? ' live' : ''}" data-f="rd-${id}" data-act="mdRadio" data-id="${id}"${id === def ? ' data-default' : ''} aria-label="${esc(name)}${live ? ', läuft' : ''}">
        <span class="md-rimg">${mdLogo(id, name)}</span>
        ${live ? `<span class="md-rlive">${mdEq(false)} Live</span>` : ''}${fav ? `<span class="md-rfav">${icon('star', 30)}</span>` : ''}</button>
      <b class="md-rn">${esc(name)}</b>
      <span class="md-rs"${live ? ` data-rsub="${id}"` : ''}>${live ? esc(mdSong(p)) : `${land === 'CH' ? 'Schweiz' : esc(land)} · ${esc(style)}`}</span>
      <button type="button" class="tb sm ghost md-rfb" data-f="rf-${id}" data-act="mdFav" data-id="${id}" aria-pressed="${fav}">${icon(fav ? 'star' : 'heart', 28)}<span>${fav ? 'Favorit' : 'Als Favorit'}</span></button>
    </div>`;
  }).join('');
  const st = on ? mdStation(on) : null;
  const bar = st ? `<div class="md-rbar" role="status">
      <span class="md-rbar-im">${mdLogo(on, st[1], true)}</span>
      <span class="md-rbar-tx"><small><span class="md-onair" aria-hidden="true"></span>Live · ${esc(st[1])}</small><b id="md-rsong">${esc(mdSong(p))}</b></span>
      <button type="button" class="tb" data-f="rd-stop" data-act="mdRadioStop">${icon('pause', 32)}<span>Stopp</span></button></div>` : '';
  return {
    html: `<div class="page md-radio">
      <h1 class="md-h1">Webradio</h1>
      <p class="md-lead">${DATA.radio.length} Sender live. OK startet einen Sender, nochmals OK hält ihn an.</p>
      <div class="md-rrow">${tiles}</div>${bar}</div>`,
    key: name => { if (name === 'play' && F.el && F.el.dataset.id) { ACT.mdRadio({ id: F.el.dataset.id }); return true; } if (name === 'play' && on) { ACT.mdRadioStop(); return true; } return mdGuard(name); },
  };
});
ACT.mdRadio = d => {
  const id = Number(d.id); const p = mdNow();
  if (p && p.kind === 'radio' && p.id === id) { ACT.mdRadioStop(); return; }
  mdStart({ kind: 'radio', id, song: Math.floor(Math.random() * 3), songAt: Date.now() });
  commit(); toast(`${(mdStation(id) || [0, 'Radio'])[1]} läuft`);
};
ACT.mdRadioStop = () => { const p = mdNow(); const id = p && p.id; S.playing = null; commit(); if (id) focusTo(`rd-${id}`); toast('Radio angehalten'); };
ACT.mdFav = d => {
  const id = Number(d.id); const f = mdFavs().slice(); const k = f.indexOf(id);
  if (k < 0) f.push(id); else f.splice(k, 1); S.mdRadioFav = f; commit();
  toast(k < 0 ? 'Zu den Favoriten hinzugefügt' : 'Aus den Favoriten entfernt');
};

/* ---------- wishes ---------- */
const MD_WST = {
  req: ['Angefragt', 'wartet auf Freigabe', ''], ok: ['Genehmigt', 'Seerr sucht den Titel', 'ok'],
  load: ['Wird geladen', 'kommt bald in die Bibliothek', 'acc'], done: ['Verfügbar', 'jetzt in der Bibliothek', 'ok'],
};
function mdWishes() {
  if (!Array.isArray(S.wishes)) S.wishes = [];
  S.wishes = S.wishes.map(w => (typeof w === 'number' ? { i: w, state: 'req', when: 'früher' } : w)).filter(w => w && DATA.T[w.i] && MD_WST[w.state]);
  if (!S.mdWishSeeded) {
    S.mdWishSeeded = true;
    if (!S.wishes.length) S.wishes = [{ i: 79, state: 'req', when: 'heute' }, { i: 108, state: 'ok', when: 'vorgestern', seasons: [1] }, { i: 75, state: 'load', pct: 64, when: 'vor 3 Tagen' }];
    save();
  }
  return S.wishes;
}
const mdWish = i => mdWishes().find(w => w.i === i);
/* one earlier film wish (Resident Evil, fulfilled) counts in the last 7 days */
const mdQuota = () => { const w = mdWishes(); return { films: 1 + w.filter(x => T(x.i).kind === 'm').length, series: w.filter(x => T(x.i).kind === 's').length }; };
const mdSeasons = i => ((D(i) || {}).s ? D(i).s.length : (i % 4) + 1);
const mdWOpen = w => w.state === 'req' || w.state === 'ok';
const mdWBadge = w => `<span class="badge${MD_WST[w.state][2] ? ` ${MD_WST[w.state][2]}` : ''}">${w.state === 'done' ? icon('check', 26) : ''}${MD_WST[w.state][0]}${w.state === 'load' ? ` · <span data-wpt="${w.i}">${Math.floor(w.pct || 0)}</span> %` : ''}</span>`;
function mdQuotaLine(cls) {
  const q = mdQuota();
  return `<div class="md-quota${cls ? ' ' + cls : ''}"><span class="md-qtx"><b>${q.films} von 10 Filmen</b> in den letzten 7 Tagen</span><span class="md-meter" aria-hidden="true"><i style="width:${Math.min(100, q.films * 10)}%"></i></span>
    <span class="md-qtx sm">${q.series} von 5 Serien</span><span class="md-meter sm" aria-hidden="true"><i style="width:${Math.min(100, q.series * 20)}%"></i></span></div>`;
}
function mdWishTick() {
  const ws = Array.isArray(S.wishes) ? S.wishes : []; const now = Date.now(); let moved = false; let changed = false;
  ws.forEach(w => {
    if (!w || typeof w !== 'object' || !MD_WST[w.state]) return;
    if (w.auto && w.state !== 'done' && w.state !== 'load') {
      const s = (now - w.auto) / 1000; const st = s < 8 ? 'req' : s < 16 ? 'ok' : 'load';
      if (st !== w.state) { w.state = st; if (st === 'load') w.pct = 0; changed = true; }
    }
    if (w.state === 'load') {
      w.pct = Math.min(100, (w.pct || 0) + (w.auto ? 2.5 : 0.3)); moved = true;
      if (w.pct >= 100) {
        w.state = 'done'; w.when = 'gerade eben'; changed = true;
        S.notes.unshift({ t: 'Wunsch erfüllt', b: `${T(w.i).title} ist jetzt in der Bibliothek`, i: w.i, when: 'gerade eben', read: false });
        toast(`«${T(w.i).title}» ist jetzt verfügbar`);
      }
    }
  });
  if (moved) {
    ws.forEach(w => {
      if (!w || w.state !== 'load') return;
      $$(`[data-wpt="${w.i}"]`).forEach(e => { e.textContent = Math.floor(w.pct); });
      $$(`[data-wpb="${w.i}"]`).forEach(e => { e.style.width = `${Math.max(2, w.pct)}%`; });
    });
  }
  if (changed) {
    save();
    const path = curPath();
    if ((path === '/wishes' || path.startsWith('/wish/')) && !OV.length) render();
  }
}
function mdWRow(w) {
  const t = T(w.i);
  const meta = [KIND[t.kind], t.year, w.seasons ? `Staffel ${w.seasons.join(', ')}` : ''].filter(Boolean).join(' · ');
  const act = mdWOpen(w) ? `<button type="button" class="tb sm ghost" data-f="wd-${w.i}" data-act="mdWDel" data-i="${w.i}">${icon('close', 28)}<span>Zurückziehen</span></button>`
    : w.state === 'done' ? `<a class="tb sm" href="${href(w.i)}" data-f="wv-${w.i}">${icon('play', 28)}<span>Ansehen</span></a>` : '';
  return `<div class="md-wrow">
    <a class="md-wmain" href="#/wish/${w.i}" data-f="wl-${w.i}">
      <span class="md-wpo">${poster(w.i)}</span>
      <span class="md-wtx"><b>${esc(t.title)}</b><small>${esc(meta)}</small>
        <span class="md-wst">${mdWBadge(w)}<span class="md-wwhen">gewünscht ${esc(w.when)} · ${MD_WST[w.state][1]}</span></span>
        ${w.state === 'load' ? `<span class="md-bar"><i data-wpb="${w.i}" style="width:${Math.max(2, w.pct || 0)}%"></i></span>` : ''}</span></a>
    <div class="md-wact">${act}</div></div>`;
}
route('/wishes', () => {
  const ws = mdWishes(); const tab = ui.tab.wishes || 'disc';
  const tabBtn = (v, label, extra) => `<button type="button" class="md-tab" role="tab" aria-selected="${tab === v}" data-f="wt-${v}" data-act="mdWTab" data-v="${v}"${tab === v ? ' data-default' : ''}>${label}${extra || ''}</button>`;
  const head = `<header class="md-whead" data-top>
    <h1 class="md-h1">Wünsche</h1>
    <div class="md-tabs" role="tablist" aria-label="Wünsche">${tabBtn('disc', 'Entdecken')}${tabBtn('mine', 'Meine Wünsche', ws.length ? `<span class="md-cnt">${ws.length}</span>` : '')}</div>
  </header>`;
  let body;
  if (tab === 'disc') {
    const opts = i => { const w = mdWish(i); return { href: `#/wish/${i}`, tag: w ? esc(MD_WST[w.state][0]) : undefined, sub: [T(i).year, KIND[T(i).kind], w ? MD_WST[w.state][0] : 'nicht in der Bibliothek'].filter(Boolean).join(' · ') }; };
    body = `<p class="md-lead md-wlead">Titel, die noch nicht in der Bibliothek sind. Dein Wunsch geht an Seerr.</p>
      <div class="md-disc">${hrow('Beliebte Filme', row('films'), opts, null, 'wfilms')}${hrow('Dokus und Serien', row('docs'), opts, null, 'wdocs')}</div>`;
  } else {
    body = `<div class="md-wbody">${mdQuotaLine()}
      ${ws.length ? `<div class="md-wlist">${ws.map(mdWRow).join('')}</div>`
    : `<div class="tnote">${icon('info', 36)}<p>Du hast noch nichts gewünscht. Unter «Entdecken» findest du Titel, die noch nicht in der Bibliothek sind.</p></div>`}
      ${ws.some(w => w.state === 'load') ? `<div class="mockvar"><span class="mk">Nur im Mockup</span><button type="button" class="tb sm" data-f="wv-fin" data-act="mdWFinish">Download fertig</button></div>` : ''}</div>`;
  }
  return {
    nav: 'wishes', cls: 'md-wishes', html: head + body,
    onFocus: el => { if (el.closest('.md-disc') && el.dataset.i != null) art(artOf(Number(el.dataset.i)), 'dim'); else if (el.closest('.md-whead')) art(null); },
    key: name => { if (name === 'play' && F.el && F.el.closest('.md-disc')) { toast('Dieser Titel ist noch nicht in der Bibliothek'); return true; } return mdGuard(name); },
  };
});
ACT.mdWTab = d => { ui.tab.wishes = d.v; render(); focusTo(`wt-${d.v}`); };
ACT.mdWFinish = () => { mdWishes().forEach(w => { if (w.state === 'load') w.pct = 99.9; }); mdWishTick(); };
ACT.mdWDel = d => {
  const i = Number(d.i);
  menu(`<h2>Wunsch zurückziehen?</h2><p class="md-dlgp">«${esc(T(i).title)}» wird bei Seerr nicht mehr angefragt.</p>
    <div class="acts"><button type="button" class="tb" data-f="wdx-ok" data-act="mdWDelOk" data-i="${i}">${icon('close', 32)}<span>Zurückziehen</span></button>
    <button type="button" class="tb primary" data-f="wdx-no" data-act="close" data-default><span>Behalten</span></button></div>`, { dialog: true, cls: 'md-dlg' });
};
ACT.mdWDelOk = d => {
  const i = Number(d.i); S.wishes = mdWishes().filter(w => w.i !== i);
  closeAll(); delete ui.mdReq; commit(); toast('Wunsch zurückgezogen');
};

/* ---------- /wish/:i  a title outside the library ---------- */
route('/wish/:i', p => {
  const i = Number(p.i); if (!DATA.T[i]) return missing(curPath());
  const t = T(i); const d = D(i) || {}; const w = mdWish(i); const ser = t.kind === 's';
  const ov = d.o || `${ser ? 'Diese Serie' : 'Dieser Film'} ist noch nicht in deiner Bibliothek. Fordere ${ser ? 'sie' : 'ihn'} an, dann sucht Seerr ${ser ? 'sie' : 'ihn'} für dich.`;
  const q = mdQuota(); const full = ser ? q.series >= 5 : q.films >= 10;
  let acts;
  if (!w) {
    acts = `<button type="button" class="tb primary" data-f="wi-req" data-act="mdReq" data-i="${i}" data-default${full ? ' aria-disabled="true"' : ''}>${icon('plus', 34)}<span>Anfordern</span></button>`;
  } else if (w.state === 'done') {
    acts = `<a class="tb primary" href="${href(i)}" data-f="wi-view" data-default>${icon('play', 34)}<span>Ansehen</span></a>`;
  } else {
    acts = `<a class="tb primary" href="#/wishes" data-f="wi-mine" data-act="mdToMine" data-default>${icon('wish', 34)}<span>Meine Wünsche</span></a>
      ${mdWOpen(w) ? `<button type="button" class="tb" data-f="wi-del" data-act="mdWDel" data-i="${i}">${icon('close', 34)}<span>Zurückziehen</span></button>` : ''}`;
  }
  return {
    nav: 'wishes',
    html: `<div class="page md-wish">
      <div class="md-wpost">${poster(i)}</div>
      <div class="md-winfo">
        <p class="md-kicker">${icon('wish', 30)} Nicht in der Bibliothek</p>
        <h1 class="md-h1 xl">${esc(t.title)}</h1>
        <p class="md-meta">${[t.year, KIND[t.kind], !ser && t.min ? dur(t.min) : '', ser ? `${mdSeasons(i)} ${mdSeasons(i) === 1 ? 'Staffel' : 'Staffeln'}` : ''].filter(Boolean).map(esc).join(' · ')} ${fsk(t.fsk)}</p>
        <p class="md-ov">${esc(ov)}</p>
        ${w ? `<div class="md-wstate">${mdWBadge(w)}<span>${w.seasons ? `Staffel ${w.seasons.join(', ')} · ` : ''}gewünscht ${esc(w.when)} · ${MD_WST[w.state][1]}</span></div>
          ${w.state === 'load' ? `<span class="md-bar md-wbar"><i data-wpb="${i}" style="width:${Math.max(2, w.pct || 0)}%"></i></span>` : ''}` : ''}
        <div class="md-acts">${acts}</div>
        ${!w ? `<p class="md-qline">${full ? 'Dein Kontingent ist aufgebraucht. Es erholt sich in den nächsten Tagen.' : ser ? `${q.series} von 5 Serien in den letzten 7 Tagen` : `${q.films} von 10 Filmen in den letzten 7 Tagen`}</p>` : ''}
      </div></div>`,
    after: () => art(artOf(i), 'dim'),
    key: name => { if (name === 'play') { toast(w && w.state === 'done' ? 'Öffne den Titel mit «Ansehen»' : 'Dieser Titel ist noch nicht in der Bibliothek'); return true; } return mdGuard(name); },
  };
});
ACT.mdToMine = () => { ui.tab.wishes = 'mine'; go('#/wishes'); };
function mdReqHtml() {
  const r = ui.mdReq; const t = T(r.i); const ser = t.kind === 's'; const q = mdQuota(); const n = mdSeasons(r.i);
  const all = r.seasons.length === n;
  const seasons = ser ? `<h3 class="md-ph3">Staffeln</h3><div class="mlist">
    <button type="button" class="mi md-chk" role="checkbox" aria-checked="${all}" data-f="rq-all" data-act="mdReqS" data-s="all"><span class="mi-tx"><b>Alle Staffeln</b><small>auch künftige</small></span><span class="md-box">${all ? icon('check', 30) : ''}</span></button>
    ${Array.from({ length: n }, (_, k) => { const on = r.seasons.includes(k + 1); return `<button type="button" class="mi md-chk" role="checkbox" aria-checked="${on}" data-f="rq-${k + 1}" data-act="mdReqS" data-s="${k + 1}"><span class="mi-tx"><b>Staffel ${k + 1}</b></span><span class="md-box">${on ? icon('check', 30) : ''}</span></button>`; }).join('')}</div>` : '';
  const ok = !ser || r.seasons.length > 0;
  return `<h2>Anfordern</h2>
    <div class="md-preq"><span class="md-preq-po">${poster(r.i)}</span><span><b>${esc(t.title)}</b><small>${[t.year, KIND[t.kind]].filter(Boolean).join(' · ')}</small></span></div>
    ${seasons}
    <div class="md-pinfo">${icon('info', 32)}<span>${ser ? `Danach ${q.series + 1} von 5 Serien` : `Danach ${q.films + 1} von 10 Filmen`} in den letzten 7 Tagen. Dein Wunsch geht an Seerr.</span></div>
    <div class="acts"><button type="button" class="tb primary" data-f="rq-ok" data-act="mdReqOk" data-default${ok ? '' : ' aria-disabled="true"'}>${icon('check', 32)}<span>Anfordern</span></button>
      <button type="button" class="tb ghost" data-f="rq-no" data-act="close"><span>Abbrechen</span></button></div>`;
}
ACT.mdReq = d => {
  const i = Number(d.i); ui.mdReq = { i, seasons: T(i).kind === 's' ? Array.from({ length: mdSeasons(i) }, (_, k) => k + 1) : [] };
  menu(mdReqHtml(), { onClose: () => { delete ui.mdReq; } });
};
ACT.mdReqS = d => {
  const r = ui.mdReq; if (!r) return; const n = mdSeasons(r.i);
  if (d.s === 'all') r.seasons = r.seasons.length === n ? [] : Array.from({ length: n }, (_, k) => k + 1);
  else { const s = Number(d.s); const k = r.seasons.indexOf(s); if (k < 0) r.seasons.push(s); else r.seasons.splice(k, 1); r.seasons.sort((a, b) => a - b); }
  redrawMenu(mdReqHtml());
};
ACT.mdReqOk = () => {
  const r = ui.mdReq; if (!r) return; const ser = T(r.i).kind === 's';
  mdWishes().unshift(Object.assign({ i: r.i, state: 'req', when: 'gerade eben', auto: Date.now() }, ser ? { seasons: r.seasons.slice() } : {}));
  closeMenu(); commit(); toast('Wunsch an Seerr geschickt');
};

/* ---------- /notifications ---------- */
route('/notifications', () => {
  const notes = S.notes || []; const unread = notes.filter(n => !n.read).length; const ann = !S.mdAnnDone;
  const firstUnread = notes.findIndex(n => !n.read);
  const def = ann ? -2 : firstUnread >= 0 ? firstUnread : 0;
  const rows = notes.map((n, k) => `<a class="md-nrow${n.read ? '' : ' unread'}" href="${href(n.i)}" data-f="nt-${k}" data-act="mdNote" data-k="${k}"${k === def ? ' data-default' : ''}>
      <span class="md-npo">${poster(n.i)}</span>
      <span class="md-ntx"><small>${n.read ? '' : '<i class="md-dot" aria-label="ungelesen"></i>'}${esc(n.t)}</small><b>${esc(n.b)}</b></span>
      <span class="md-nwhen">${esc(n.when)}</span></a>`).join('');
  return {
    nav: 'settings',
    html: `<div class="page md-nt">
      <h1 class="md-h1">Benachrichtigungen</h1>
      <p class="md-lead">${unread ? `${unread} ungelesen` : 'Alles gelesen'}</p>
      ${ann ? `<section class="md-ann" aria-label="Ankündigung"><span class="md-ann-ic">${icon('alert', 44)}</span>
        <span class="md-ann-tx"><small>Ankündigung des Servers</small><b>Wartung am Sonntag 02:00 bis 03:00</b><span>Slouchcat ist in dieser Zeit nicht erreichbar.</span></span>
        <button type="button" class="tb primary sm" data-f="nt-ann" data-act="mdAnnOk" data-default>${icon('check', 30)}<span>Verstanden</span></button></section>` : ''}
      <div class="md-nacts"><button type="button" class="tb sm ghost" data-f="nt-all" data-act="mdNotesAll"${unread ? '' : ' aria-disabled="true"'}>${icon('check', 30)}<span>Alle als gelesen markieren</span></button></div>
      <div class="md-nlist">${rows || `<div class="tnote">${icon('bell', 36)}<p>Keine Benachrichtigungen.</p></div>`}</div></div>`,
    key: mdGuard,
  };
});
ACT.mdNote = d => { const n = S.notes[Number(d.k)]; if (!n) return; n.read = true; save(); go(href(n.i)); };
ACT.mdNotesAll = () => { S.notes.forEach(n => { n.read = true; }); commit(); focusTo('nt-0'); toast('Alle als gelesen markiert'); };
ACT.mdAnnOk = () => { S.mdAnnDone = true; commit(); toast('Ankündigung bestätigt'); };

/* ---------- /settings ---------- */
const MD_SET = [
  ['prof', 'Profil wechseln', 'users', '#/profiles'],
  ['manage', 'Profile verwalten', 'edit', '#/settings/profiles'],
  ['play', 'Wiedergabe', 'play', '#/settings/playback'],
  ['notes', 'Benachrichtigungen', 'bell', '#/notifications'],
  ['radio', 'Webradio', 'radio', '#/radio'],
  ['server', 'Server und Konto', 'server', null, 'mdServer'],
  ['about', 'Über', 'info', null, 'mdAbout'],
];
const MD_SOUND = { auto: 'Automatisch', stereo: 'Immer Stereo', manual: 'Formate von Hand' };
const MD_FMT = [['dd', 'Dolby Digital'], ['ddp', 'Dolby Digital Plus'], ['dts', 'DTS'], ['dtshd', 'DTS-HD'], ['truehd', 'Dolby TrueHD']];
const MD_DETECTED = 'Dolby Digital, Dolby Digital Plus, 5.1 Kanäle';
function mdNorm() {
  const pr = S.prefs || (S.prefs = {});
  if (pr.skipIntro == null) pr.skipIntro = true;
  if (!MD_SOUND[pr.mdSound]) pr.mdSound = pr.surround === false ? 'stereo' : 'auto';
  pr.mdFmt = Object.assign({ dd: true, ddp: true, dts: false, dtshd: false, truehd: false }, pr.mdFmt);
  pr.surround = pr.mdSound !== 'stereo';
  return pr;
}
function mdSetSub(k) {
  const pr = mdNorm(); const unread = (S.notes || []).filter(n => !n.read).length;
  return {
    prof: `Aktiv: ${me().name}`, manage: `${S.profiles.length} Profile`, play: `${pr.quality} · Ton ${MD_SOUND[pr.mdSound]}`,
    notes: unread ? `${unread} ungelesen` : 'alles gelesen', radio: `${DATA.radio.length} Sender`,
    server: hostOf(S.server) || 'nicht verbunden', about: `Version ${MD_APP_VERSION}`,
  }[k];
}
function mdServerDl() {
  return `<dl class="md-dl">
    <div><dt>Server</dt><dd>${esc(hostOf(S.server) || 'unbekannt')}</dd></div>
    <div><dt>Verbindung</dt><dd>${S.secure ? `<span class="badge ok">${icon('lock', 26)} verschlüsselt</span>` : `<span class="badge warn">${icon('unlock', 26)} nicht verschlüsselt</span>`}</dd></div>
    <div><dt>Konto</dt><dd>${esc(S.account)}</dd></div>
    <div><dt>Dieser Fernseher</dt><dd>${esc(S.tvName)}</dd></div></dl>
    <p class="md-help">So heisst dieser Fernseher am Handy unter «Meine Geräte». Dort kannst du ihn auch abmelden.</p>`;
}
function mdSetPane(k) {
  const pr = mdNorm();
  if (k === 'prof') return `<div class="md-spav">${avatar(me(), 180)}</div><h2>${esc(me().name)}</h2><p>Wechsle zu einem anderen Profil dieses Kontos. Profile mit PIN fragen danach.</p>`;
  if (k === 'manage') return `<div class="md-spavs">${S.profiles.map(p => avatar(p, 110)).join('')}</div><h2>Profile verwalten</h2><p>${me().main ? 'Profile anlegen, ändern und löschen, PINs und Altersfreigaben festlegen.' : 'Nur mit dem Hauptprofil möglich.'}</p>`;
  if (k === 'play') return `<h2>Wiedergabe</h2><dl class="md-dl sm"><div><dt>Qualität</dt><dd>${esc(pr.quality)}</dd></div><div><dt>Sprache</dt><dd>${esc(pr.audio)}</dd></div><div><dt>Untertitel</dt><dd>${esc(pr.subs)}</dd></div><div><dt>Ton</dt><dd>${MD_SOUND[pr.mdSound]}</dd></div></dl>`;
  if (k === 'notes') { const un = (S.notes || []).filter(n => !n.read); return `<span class="md-spic">${icon('bell', 96)}</span><h2>${un.length ? `${un.length} ungelesen` : 'Alles gelesen'}</h2>${un.slice(0, 2).map(n => `<p>${esc(n.b)}</p>`).join('')}`; }
  if (k === 'radio') return `<div class="md-splogos">${DATA.radio.map(([id, name]) => `<span class="md-splogo">${mdLogo(id, name)}</span>`).join('')}</div><h2>Webradio</h2><p>Sender aus der Schweiz, live.</p>`;
  if (k === 'server') return `<h2>Server und Konto</h2>${mdServerDl()}`;
  return `<span class="md-spic">${wordmark()}</span><h2>Slouchcat für Android TV</h2><dl class="md-dl sm"><div><dt>App</dt><dd>${MD_APP_VERSION}</dd></div><div><dt>Server</dt><dd>Slouchcat ${esc(mdServerVersion())}</dd></div></dl>`;
}
route('/settings', () => {
  const list = MD_SET.map(([k, label, ic, h, act], n) => `<${h ? `a href="${h}"` : `button type="button" data-act="${act}"`} class="mi md-set" data-f="st-${k}" data-k="${k}"${n === 0 ? ' data-default' : ''}>
    <span class="mi-ic">${icon(ic, 36)}</span><span class="mi-tx"><b>${label}</b><small>${esc(mdSetSub(k))}</small></span>${icon('right', 32)}</${h ? 'a' : 'button'}>`).join('');
  return {
    nav: 'settings',
    html: `<div class="page md-settings"><h1 class="md-h1">Einstellungen</h1>
      <div class="md-scols"><div class="mlist md-slist">${list}</div><aside class="md-spane" id="md-spane" data-k="prof" aria-live="polite">${mdSetPane('prof')}</aside></div></div>`,
    onFocus: el => { const k = el.dataset.k; const p = $('#md-spane'); if (k && p && p.dataset.k !== k) { p.dataset.k = k; p.innerHTML = mdSetPane(k); p.classList.remove('swap'); void p.offsetWidth; p.classList.add('swap'); } },
    key: mdGuard,
  };
});
ACT.mdServer = () => menu(`<h2>Server und Konto</h2>${mdServerDl()}
  <div class="acts md-vacts"><button type="button" class="tb primary" data-f="sv-close" data-act="close" data-default><span>Schliessen</span></button>
  <button type="button" class="tb ghost" data-f="sv-out" data-act="mdLogout">${icon('logout', 32)}<span>Diesen Fernseher abmelden</span></button></div>`);
ACT.mdLogout = () => menu(`<h2>Diesen Fernseher abmelden?</h2>
  <p class="md-dlgp">Der Fernseher vergisst Server und Konto. Danach richtest du ihn neu ein, mit dem Handy oder von Hand.</p>
  <div class="acts"><button type="button" class="tb" data-f="lo-ok" data-act="mdLogoutOk">${icon('logout', 32)}<span>Abmelden</span></button>
  <button type="button" class="tb primary" data-f="lo-no" data-act="close" data-default><span>Abbrechen</span></button></div>`, { dialog: true, cls: 'md-dlg' });
ACT.mdLogoutOk = () => {
  closeAll(); mdStash();
  Object.assign(S, { setup: false, server: '', playing: null, profile: 0 }); save();
  history.replaceState(null, '', '#/setup'); NAVSTK.length = 0; NAVSTK.push('#/setup');
  render(); mdSync(); toast('Dieser Fernseher ist abgemeldet');
};
ACT.mdAbout = () => menu(`<h2>Über</h2><div class="md-about">${wordmark()}<p>Slouchcat für Android TV</p></div>
  <dl class="md-dl"><div><dt>App-Version</dt><dd>${MD_APP_VERSION}</dd></div><div><dt>Server</dt><dd>Slouchcat ${esc(mdServerVersion())}</dd></div><div><dt>Adresse</dt><dd>${esc(hostOf(S.server) || 'unbekannt')}</dd></div></dl>
  <div class="acts"><button type="button" class="tb primary" data-f="ab-close" data-act="close" data-default><span>Schliessen</span></button></div>`);

/* ---------- /settings/profiles ---------- */
const MD_FSKS = [0, 6, 12, 16, 18];
route('/settings/profiles', () => {
  const mine = me(); const main = S.profiles.find(p => p.main) || S.profiles[0];
  if (!mine.main) {
    return {
      nav: 'settings',
      html: `<div class="page md-pm"><h1 class="md-h1">Profile verwalten</h1>
        <div class="tnote warn">${icon('lock', 36)}<p>Profile verwaltet nur das Hauptprofil. Wechsle ins Profil «${esc(main.name)}», um Profile anzulegen, zu ändern oder zu löschen.</p></div>
        <div class="md-acts"><a class="tb primary" href="#/profiles" data-f="pm-switch" data-default>${icon('users', 32)}<span>Profil wechseln</span></a>
        <button type="button" class="tb ghost" data-f="pm-back" data-act="back">${icon('back', 32)}<span>Zurück</span></button></div></div>`,
      key: mdGuard,
    };
  }
  const rec = !main.pin && S.profiles.some(p => (p.fsk ?? 18) < 18);
  const rows = S.profiles.map((p, k) => `<button type="button" class="md-prow" data-f="pr-${k}" data-act="mdPEdit" data-k="${k}"${k === 0 ? ' data-default' : ''}>
      ${avatar(p, 96)}<span class="md-prtx"><b>${esc(p.name)}</b><small>${[p.main ? 'Hauptprofil' : '', (p.fsk ?? 18) >= 18 ? 'alle Freigaben' : `Freigabe ${mdFsk(p.fsk)}`, p.pin ? 'mit PIN' : 'ohne PIN'].filter(Boolean).join(' · ')}</small></span>
      ${p.pin ? `<span class="md-prlock">${icon('lock', 32)}</span>` : ''}<span class="md-prgo">${icon('edit', 32)}</span></button>`).join('');
  return {
    nav: 'settings',
    html: `<div class="page md-pm"><h1 class="md-h1">Profile verwalten</h1>
      ${rec ? `<div class="md-rec">${icon('shield', 44)}<p>Empfehlung: Gib dem Hauptprofil eine PIN. Sonst kann jeder ins Hauptprofil wechseln und Altersfreigaben ändern.</p>
        <button type="button" class="tb sm" data-f="pm-pin" data-act="mdPinMain">${icon('lock', 30)}<span>PIN setzen</span></button></div>` : ''}
      <div class="md-plist">${rows}
        <button type="button" class="md-prow add" data-f="pr-add" data-act="mdPEdit" data-k="-1"><span class="md-pradd">${icon('plus', 44)}</span><span class="md-prtx"><b>Profil hinzufügen</b><small>zum Beispiel für Kinder, mit Altersfreigabe</small></span></button></div></div>`,
    key: mdGuard,
  };
});
function mdPEHtml() {
  const d = ui.mdPE; const isNew = d.k < 0; const orig = isNew ? null : S.profiles[d.k];
  const pv = { name: d.name || '?', color: d.color };
  const sw = [1, 2, 3, 4, 5, 6].map(n => `<button type="button" class="md-sw" style="background:var(--av${n})" data-f="pe-c${n}" data-act="mdPECol" data-c="av${n}" aria-pressed="${d.color === `av${n}`}" aria-label="Farbe ${n}">${d.color === `av${n}` ? icon('check', 34) : ''}</button>`).join('');
  const chips = MD_FSKS.map(v => `<button type="button" class="md-chip" data-f="pe-f${v}" data-act="mdPEFsk" data-v="${v}" aria-pressed="${d.fsk === v}">${v}</button>`).join('');
  const ok = d.name.trim().length > 0;
  return `<h2>${isNew ? 'Neues Profil' : 'Profil bearbeiten'}</h2>
    <div class="md-pehead">${avatar(pv, 104)}<span><b>${esc(d.name || 'Ohne Namen')}</b><small>${orig && orig.main ? 'Hauptprofil' : (d.fsk >= 18 ? 'alle Freigaben' : `Freigabe ${mdFsk(d.fsk)}`)}</small></span></div>
    <span class="tf-lab">Name</span>
    <button type="button" class="tf md-petf" data-f="pe-name" data-act="mdPEName" data-default>${d.name ? `<span class="tf-v">${esc(d.name)}</span>` : '<span class="tf-ph">Name eingeben</span>'}<span class="tf-ic">${icon('keyboard', 34)}</span></button>
    <span class="tf-lab md-pelab">Farbe</span><div class="md-sws" data-row="pecol">${sw}</div>
    <span class="tf-lab md-pelab">Altersfreigabe</span><div class="md-chips" data-row="pefsk">${chips}</div>
    <p class="md-help">${d.fsk >= 18 ? 'Dieses Profil sieht alle Titel.' : `Dieses Profil sieht nur Titel bis ${mdFsk(d.fsk)}.`}</p>
    <span class="tf-lab md-pelab">PIN</span>
    <div class="md-pepin">${d.pin ? `<span class="badge ok">${icon('lock', 26)} PIN gesetzt</span>` : '<span class="badge">keine PIN</span>'}</div>
    <div class="md-pebtns">
      <button type="button" class="tb sm" data-f="pe-pin" data-act="mdPEPin">${icon('lock', 30)}<span>${d.pin ? 'PIN ändern' : 'PIN setzen'}</span></button>
      ${d.pin ? `<button type="button" class="tb sm ghost" data-f="pe-pinx" data-act="mdPEPinX">${icon('unlock', 30)}<span>PIN entfernen</span></button>` : ''}</div>
    <div class="acts md-peacts"><button type="button" class="tb primary" data-f="pe-save" data-act="mdPESave"${ok ? '' : ' aria-disabled="true"'}>${icon('check', 32)}<span>${isNew ? 'Anlegen' : 'Speichern'}</span></button>
      ${!isNew && !orig.main ? `<button type="button" class="tb ghost" data-f="pe-del" data-act="mdPEDel">${icon('trash', 32)}<span>Löschen</span></button>` : ''}</div>`;
}
ACT.mdPEdit = d => {
  const k = Number(d.k); const p = k >= 0 ? S.profiles[k] : null;
  const used = S.profiles.map(x => x.color); const free = [1, 2, 3, 4, 5, 6].map(n => `av${n}`).find(c => !used.includes(c)) || 'av4';
  ui.mdPE = p ? { k, name: p.name, color: p.color || 'av1', fsk: p.fsk ?? 18, pin: !!p.pin } : { k: -1, name: '', color: free, fsk: 12, pin: false };
  menu(mdPEHtml(), { cls: 'md-pe', onClose: () => { delete ui.mdPE; } });
};
ACT.mdPEName = () => {
  const d = ui.mdPE; if (!d) return;
  keyboard({ value: d.name, label: 'Name des Profils', type: 'text', placeholder: 'zum Beispiel Lea', onDone: v => { if (!ui.mdPE) return; ui.mdPE.name = v.trim().slice(0, 20); redrawMenu(mdPEHtml()); if (ui.mdPE.name) focusTo('pe-c1'); } });
};
ACT.mdPECol = d => { if (!ui.mdPE) return; ui.mdPE.color = d.c; redrawMenu(mdPEHtml()); };
ACT.mdPEFsk = d => { if (!ui.mdPE) return; ui.mdPE.fsk = Number(d.v); redrawMenu(mdPEHtml()); };
ACT.mdPEPin = () => {
  const d = ui.mdPE; if (!d) return;
  mdPinPad({ name: d.name || 'das neue Profil', onDone: () => { if (!ui.mdPE) return; ui.mdPE.pin = true; redrawMenu(mdPEHtml()); focusTo('pe-save'); toast('PIN bereit. Mit «Speichern» übernimmst du sie.'); } });
};
ACT.mdPEPinX = () => { if (!ui.mdPE) return; ui.mdPE.pin = false; redrawMenu(mdPEHtml()); focusTo('pe-pin'); toast('PIN wird mit «Speichern» entfernt'); };
ACT.mdPESave = () => {
  const d = ui.mdPE; if (!d || !d.name.trim()) return;
  const vals = { name: d.name.trim(), color: d.color, fsk: d.fsk, pin: d.pin };
  if (d.k < 0) S.profiles.push(Object.assign({ main: false }, vals)); else Object.assign(S.profiles[d.k], vals);
  const isNew = d.k < 0; closeMenu(); commit(); if (isNew) focusTo(`pr-${S.profiles.length - 1}`);
  toast(isNew ? `Profil «${vals.name}» angelegt` : 'Profil gespeichert');
};
ACT.mdPEDel = () => {
  const d = ui.mdPE; if (!d || d.k < 0) return; const p = S.profiles[d.k];
  menu(`<h2>Profil «${esc(p.name)}» löschen?</h2><p class="md-dlgp">Merkliste, Fortschritt und Wünsche dieses Profils gehen verloren. Das lässt sich nicht rückgängig machen.</p>
    <div class="acts"><button type="button" class="tb" data-f="pdx-ok" data-act="mdPEDelOk" data-k="${d.k}">${icon('trash', 32)}<span>Löschen</span></button>
    <button type="button" class="tb primary" data-f="pdx-no" data-act="close" data-default><span>Abbrechen</span></button></div>`, { dialog: true, cls: 'md-dlg' });
};
ACT.mdPEDelOk = d => {
  const k = Number(d.k); const p = S.profiles[k]; if (!p || p.main) return;
  closeAll(); delete ui.mdPE; S.profiles.splice(k, 1);
  if (S.profile > k) S.profile -= 1; else if (S.profile === k) S.profile = 0;
  commit(); toast(`Profil «${p.name}» gelöscht`);
};
ACT.mdPinMain = () => {
  const main = S.profiles.find(p => p.main); if (!main) return;
  mdPinPad({ name: main.name, onDone: () => { main.pin = true; commit(); toast(`${main.name} hat jetzt eine PIN`); } });
};

/* number pad for a new PIN: enter, then repeat */
function mdPinPad(o) {
  Object.assign(MDPIN, { v: '', first: null, o });
  const key = (v, label, cls, extra) => `<button type="button" class="pin-k${cls ? ' ' + cls : ''}" data-f="mp-${v}" data-act="mdPinKey" data-d="${v}"${extra || ''}>${label}</button>`;
  menu(`<div class="pin" id="md-pin"><span class="md-pinic">${icon('lock', 64)}</span><h2 id="md-pinh">Neue PIN für ${esc(o.name)}</h2>
      <div class="pin-dots" aria-hidden="true"><i></i><i></i><i></i><i></i></div>
      <p class="pin-msg" role="status" aria-live="polite">Gib vier Ziffern ein.</p>
      <div class="pin-pad">${[1, 2, 3, 4, 5, 6, 7, 8, 9].map(n => key(n, n, '', n === 1 ? ' data-default' : '')).join('')}
        ${key('del', icon('backspace', 40), 'fn', ' aria-label="Löschen"')}${key(0, 0)}${key('x', 'Abbrechen', 'fn')}</div></div>`, {
    dialog: true,
    onKey: (name, e) => {
      if (e && /^[0-9]$/.test(e.key)) { mdPinPress(e.key); return true; }
      if (e && e.key === 'Backspace') { mdPinPress('del'); return true; }
      return false;
    },
    onClose: () => { MDPIN.o = null; },
  });
}
ACT.mdPinKey = d => mdPinPress(String(d.d));
function mdPinDraw(msg, head) {
  const box = $('#md-pin'); if (!box) return;
  $$('.pin-dots i', box).forEach((d, n) => d.classList.toggle('on', n < MDPIN.v.length));
  if (msg) $('.pin-msg', box).innerHTML = msg;
  if (head) $('#md-pinh').textContent = head;
}
function mdPinPress(d) {
  const o = MDPIN.o; if (!o) return;
  if (d === 'x') { closeMenu(); return; }
  if (d === 'del') { MDPIN.v = MDPIN.v.slice(0, -1); mdPinDraw(); return; }
  if (MDPIN.v.length >= 4) return;
  MDPIN.v += d; mdPinDraw();
  if (MDPIN.v.length < 4) return;
  setTimeout(() => {
    if (MDPIN.o !== o) return;
    if (MDPIN.first == null) { MDPIN.first = MDPIN.v; MDPIN.v = ''; mdPinDraw('Gib die PIN zur Bestätigung nochmals ein.', 'PIN wiederholen'); return; }
    if (MDPIN.v === MDPIN.first) { closeMenu(); o.onDone(); return; }
    MDPIN.first = null; MDPIN.v = '';
    const box = $('#md-pin'); box.classList.remove('shake'); void box.offsetWidth; box.classList.add('shake');
    mdPinDraw(`${icon('alert', 30)} Die beiden PINs stimmen nicht überein. Fang nochmals an.`, `Neue PIN für ${o.name}`);
  }, 300);
}

/* ---------- /settings/playback ---------- */
const MD_OPTS = {
  quality: ['Qualität', [['Automatisch', 'passt sich der Verbindung an'], ['Original', 'ohne Umwandlung, wenn möglich'], ['1080p', ''], ['720p', ''], ['480p', 'spart Bandbreite']]],
  audio: ['Bevorzugte Sprache', [['Deutsch', ''], ['Englisch', ''], ['Französisch', ''], ['Italienisch', ''], ['Originalsprache', 'die Sprache, in der gedreht wurde']]],
  subs: ['Untertitel', [['Aus', ''], ['Nur bei fremder Sprache', 'wenn der Ton nicht deine Sprache ist'], ['Deutsch', ''], ['Englisch', '']]],
};
ACT.mdOpt = d => {
  const [label, opts] = MD_OPTS[d.k]; const cur = S.prefs[d.k];
  menu(`<h2>${label}</h2><div class="mlist">${opts.map(([v, sub], n) => `<button type="button" class="mi" role="menuitemradio" aria-checked="${v === cur}" data-f="op-${n}" data-act="set" data-path="S.prefs.${d.k}" data-value="${esc(v)}"${v === cur ? ' data-default' : ''}>
    <span class="mi-tx"><b>${esc(v)}</b>${sub ? `<small>${esc(sub)}</small>` : ''}</span>${v === cur ? icon('check', 36) : ''}</button>`).join('')}</div>`);
};
route('/settings/playback', () => {
  const pr = mdNorm();
  const sel = k => `<button type="button" class="md-srow" data-f="pb-${k}" data-act="mdOpt" data-k="${k}"><span class="md-srow-l">${MD_OPTS[k][0]}</span><span class="md-srow-v">${esc(pr[k])}</span>${icon('right', 32)}</button>`;
  const sw = (f, path, label, on, sub) => `<button type="button" class="tsw md-sw2" role="switch" aria-checked="${on}" data-f="${f}" data-act="toggle" data-scope="S" data-path="${path}"><span class="tsw-tx">${label}${sub ? `<small>${sub}</small>` : ''}</span><span class="sw" aria-hidden="true"></span></button>`;
  const radio = (v, sub) => `<button type="button" class="md-radio" role="radio" aria-checked="${pr.mdSound === v}" data-f="pb-snd-${v}" data-act="set" data-path="S.prefs.mdSound" data-value="${v}"><span class="md-rdot" aria-hidden="true"></span><span class="md-radio-tx"><b>${MD_SOUND[v]}</b><small>${sub}</small></span></button>`;
  const fmts = MD_FMT.map(([k, label]) => sw(`pb-f-${k}`, `prefs.mdFmt.${k}`, label, !!pr.mdFmt[k])).join('');
  const passed = pr.mdSound === 'auto' ? `${MD_DETECTED}. Diese Formate gehen unverändert an die Tonanlage.`
    : pr.mdSound === 'stereo' ? 'Jeder Ton kommt als Stereo, auch Mehrkanalton.'
      : `Gewählt: ${MD_FMT.filter(([k]) => pr.mdFmt[k]).map(x => x[1]).join(', ') || 'keins, also Stereo'}.`;
  return {
    nav: 'settings',
    html: `<div class="page md-pb"><h1 class="md-h1">Wiedergabe</h1>
      <section class="md-sgrp"><h2 class="md-sech">Bild und Sprache</h2>${sel('quality')}${sel('audio')}${sel('subs')}</section>
      <section class="md-sgrp"><h2 class="md-sech">Serien</h2>
        ${sw('pb-auto', 'prefs.autoplay', 'Nächste Folge automatisch', pr.autoplay, 'spielt nach dem Abspann die nächste Folge')}
        ${sw('pb-intro', 'prefs.skipIntro', 'Vorspann überspringen anbieten', pr.skipIntro, 'zeigt im Vorspann einen Knopf zum Überspringen')}</section>
      <section class="md-sgrp"><h2 class="md-sech">Ton</h2>
        <p class="md-help">Tonausgabe: Mehrkanalton geht unverändert an Soundbar oder Receiver, wenn sie ihn können.</p>
        <div class="md-radios" role="radiogroup" aria-label="Tonausgabe">
          ${radio('auto', 'Der Fernseher meldet, was die Tonanlage kann')}
          ${radio('stereo', 'Für Kopfhörer oder die Lautsprecher des Fernsehers')}
          ${radio('manual', 'Wenn die Erkennung falsch liegt')}</div>
        <div class="md-detect">${icon('audio', 36)}<span><b>Erkannt: ${MD_DETECTED}</b><small>${esc(passed)}</small></span></div>
        ${pr.mdSound === 'manual' ? `<div class="md-fmts"><h3 class="md-ph3">Formate, die deine Tonanlage kann</h3>${fmts}</div>` : ''}
      </section></div>`,
    key: mdGuard,
  };
});

PAGES.push(
  ['#/book/32', 'Hörbuch «Silo»'], ['#/book/88', 'Hörbuch «4000 Wochen»'], ['#/podcast/56', 'Podcast «Y-Kollektiv»'], ['#/podcast/64', 'Podcast «Zum Glück ist Freitag»'],
  ['#/listen/32', 'Tonplayer Hörbuch'], ['#/listen/58?e=0', 'Tonplayer Podcast'], ['#/radio', 'Webradio'],
  ['#/wishes', 'Wünsche'], ['#/wish/79', 'Wunsch «1984» (Film)'], ['#/wish/106', 'Wunsch Serie (Staffeln)'],
  ['#/notifications', 'Benachrichtigungen'], ['#/settings', 'Einstellungen'], ['#/settings/profiles', 'Profile verwalten'], ['#/settings/playback', 'Wiedergabe und Ton'],
);
