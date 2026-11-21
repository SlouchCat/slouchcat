/* Start page: full-bleed spotlight and rows. Reference page for the other page files. */
'use strict';

let spotAt = 0; let spotTimer = null; let spotChanged = Date.now();
const spotIds = () => row('spot');
const spotCur = () => spotIds()[spotAt];

function spotMeta(i) {
  const t = T(i); const d = D(i) || {}; const parts = [];
  if (t.year) parts.push(`<span>${t.year}</span>`);
  if (t.fsk) parts.push(fsk(t.fsk));
  parts.push(`<span>${t.kind === 's' ? (d.s ? `${d.s.length} Staffeln` : 'Serie') : dur(t.min)}</span>`);
  const g = (d.g || []).slice(0, 3).join(' · '); if (g) parts.push(`<span>${esc(g)}</span>`);
  return parts.join('<i class="sep" aria-hidden="true"></i>');
}
function spotInfo(i) {
  const t = T(i); const d = D(i) || {};
  /* every spotlight title has a logo file; if one is missing the title shows as text */
  const name = `<img class="sp-logo" src="${IMG}l-${i}.webp" alt="${esc(t.title)}" onerror="this.replaceWith(Object.assign(document.createElement('h1'),{className:'sp-title',textContent:this.alt}))">`;
  return `<div class="sp-kind">${KIND[t.kind]}</div><div class="sp-name">${name}</div><div class="sp-meta">${spotMeta(i)}</div>${d.o ? `<p class="sp-ov">${esc(d.o)}</p>` : ''}`;
}
function watchBtn(i) { const on = S.watch.includes(i); return `${icon(on ? 'bookmarkOn' : 'bookmark', 34)}<span>${on ? 'Gemerkt' : 'Merken'}</span>`; }
function showSpot() {
  const i = spotCur(); const box = $('#sp-info'); if (!box) return;
  box.innerHTML = spotInfo(i); box.classList.remove('swap'); void box.offsetWidth; box.classList.add('swap');
  $$('.sp-th').forEach(b => b.classList.toggle('on', Number(b.dataset.k) === spotAt));
  const w = byF('sp-watch'); if (w) { w.innerHTML = watchBtn(i); w.setAttribute('aria-pressed', String(S.watch.includes(i))); }
  if (!F.el || F.el.closest('.hero') || F.el.closest('#rail')) art(artOf(i), 'hero');
  spotChanged = Date.now();
}
ACT.spotPlay = () => go(`#/play/${spotCur()}`);
ACT.spotWatch = () => ACT.watch({ i: spotCur() });
ACT.spotInfo = () => go(href(spotCur()));
ACT.spotOpen = d => go(href(spotIds()[Number(d.k)]));

function homeFocus(el) {
  if (el.closest('.hero')) {
    if (el.classList.contains('sp-th')) { const k = Number(el.dataset.k); if (k !== spotAt) { spotAt = k; showSpot(); } }
    art(artOf(spotCur()), 'hero'); return;
  }
  if (el.dataset.i != null) art(artOf(Number(el.dataset.i)), 'dim');
}

route('/', () => {
  const ids = spotIds(); if (spotAt >= ids.length) spotAt = 0; const i = spotCur();
  const hero = `<section class="hero" data-top aria-label="Spotlight">
    <div id="sp-info" class="sp-info">${spotInfo(i)}</div>
    <div class="sp-acts">
      <button type="button" class="tb primary" data-f="sp-play" data-act="spotPlay" data-default>${icon('play', 34)}<span>Abspielen</span></button>
      <button type="button" class="tb" data-f="sp-watch" data-act="spotWatch" aria-pressed="${S.watch.includes(i)}">${watchBtn(i)}</button>
      <button type="button" class="tb" data-f="sp-det" data-act="spotInfo">${icon('info', 34)}<span>Details</span></button>
    </div>
    <div class="sp-strip" data-row="spot" role="group" aria-label="Weitere Titel im Spotlight">${ids.map((j, k) => `<button type="button" class="sp-th${k === spotAt ? ' on' : ''}" data-f="sp-${k}" data-k="${k}" data-act="spotOpen" aria-label="${esc(T(j).title)}">${backdrop(j)}<i aria-hidden="true"></i></button>`).join('')}</div>
  </section>`;
  const pct = j => (DATA.R.cont.find(x => x[0] === j) || [0, 0])[1];
  const rows = [
    hrow('Weiterschauen', row('cont'), j => {
      const t = T(j); const p = pct(j);
      return { wide: true, prog: p || 4, href: `#/play/${j}`, sub: t.kind === 'e' ? 'Folge · weiterschauen' : `noch ${Math.max(1, Math.round(t.min * (100 - p) / 100))} Min.` };
    }, null, 'cont'),
    hrow('Neu erschienen', row('rel'), null, '#/list/rel', 'rel'),
    hrow('Zuletzt hinzugefügt', row('rec'), null, '#/list/rec', 'rec'),
    hrow('Empfohlen von anderen Profilen', row('picks'), () => ({ tag: `${icon('heart', 24)} Tipp` }), '#/list/picks', 'picks'),
    hrow('Serien', row('lib6'), null, '#/library/series', 'lib6'),
    hrow('Filme', row('lib1'), null, '#/library/films', 'lib1'),
    S.watch.length ? hrow('Meine Merkliste', S.watch, null, '#/watchlist', 'watch') : '',
    hrow('Podcasts', row('lib4'), { sq: true }, '#/library/podcasts', 'lib4'),
  ].join('');
  return {
    nav: 'start', cls: 'home', html: hero + rows, onFocus: homeFocus,
    after: () => {
      art(artOf(i), 'hero');
      clearInterval(spotTimer);
      /* auto-advance every 8 s, only while the viewer rests on the spotlight buttons or the rail */
      spotTimer = setInterval(() => {
        if (!$('.hero')) { clearInterval(spotTimer); return; }
        if (OV.length || OFF || !F.el || !(F.el.closest('.sp-acts') || F.el.closest('#rail'))) return;
        if (Date.now() - Math.max(lastInput, spotChanged) < 8000) return;
        spotAt = (spotAt + 1) % spotIds().length; showSpot();
      }, 1000);
    },
  };
});
PAGES.push(['#/', 'Start']);
