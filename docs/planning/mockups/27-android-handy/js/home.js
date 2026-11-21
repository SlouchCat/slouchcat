/* Start page: spotlight and rows. Reference page for the other page files. */
'use strict';

let heroAt = 0; let heroTimer = null;
function hero() {
  const ids = row('spot');
  const slides = ids.map((i, k) => {
    const t = T(i); const d = D(i) || {}; const g = (d.g || []).slice(0, 2).join(' · ');
    const name = d.l ? `<img class="logo" src="img/l-${i}.webp" alt="${esc(t.title)}">` : `<h2>${esc(t.title)}</h2>`;
    return `<div class="sl${k === heroAt ? ' on' : ''}" data-k="${k}">${backdrop(i)}<div class="sh"></div>
      <div class="in">${name}
        <div class="meta"><span>${t.year}</span>${fsk(t.fsk)}<span>${t.kind === 's' ? 'Serie' : dur(t.min)}</span>${g ? `<span>${esc(g)}</span>` : ''}</div>
        <div class="acts"><a class="btn primary" href="#/play/${i}" data-fid="hp${i}">${icon('play', 20)} Abspielen</a>
          <button type="button" class="btn secondary" data-act="watch" data-i="${i}" data-fid="hw${i}" aria-pressed="${S.watch.includes(i)}">${icon(S.watch.includes(i) ? 'bookmarkOn' : 'bookmark', 20)} Merken</button>
          <a class="ib" href="${href(i)}" aria-label="Details zu ${esc(t.title)}">${icon('info')}</a></div></div></div>`;
  }).join('');
  const dots = ids.map((i, k) => `<button type="button" data-act="hero" data-k="${k}" aria-label="Titel ${k + 1} von ${ids.length}" aria-current="${k === heroAt}"><i></i></button>`).join('');
  return `<section class="hero" aria-label="Spotlight">${slides}<div class="dots">${dots}</div></section>`;
}
ACT.hero = d => { heroAt = Number(d.k); showHero(); };
function showHero() {
  $$('.hero .sl').forEach(s => s.classList.toggle('on', Number(s.dataset.k) === heroAt));
  $$('.hero .dots button').forEach((b, k) => b.setAttribute('aria-current', String(k === heroAt)));
}

/* libraries of the test instance; the lists hold the titles the mockup knows */
const LIBS = [
  { key: 'films', name: 'Filme', icon: 'film', count: 1846, ids: () => [...new Set([...row('films'), ...row('lib1'), ...row('rec').filter(i => T(i).kind === 'm')])] },
  { key: 'series', name: 'Serien', icon: 'tv', count: 505, ids: () => [...new Set([...row('lib6'), ...row('rel').filter(i => T(i).kind === 's')])] },
  { key: 'docs', name: 'Dokus', icon: 'globe', count: 63, ids: () => row('docs') },
  { key: 'books', name: 'Hörbücher', icon: 'headphones', count: 89, ids: () => [...new Set([...row('books'), ...row('watch').filter(i => T(i).kind === 'a')])] },
  { key: 'podcasts', name: 'Podcasts', icon: 'mic', count: 100, ids: () => [...new Set([...row('pods'), ...row('lib4')])] },
];

route('/', () => {
  const p = me(); const unread = S.notes.filter(n => !n.read).length;
  const cont = DATA.R.cont.map(([i, pct]) => card(i, {
    wide: true, prog: pct || 4, href: `#/play/${i}`,
    sub: T(i).kind === 'e' ? 'Folge · weiterschauen' : `noch ${Math.max(1, Math.round(T(i).min * (100 - pct) / 100))} Min.`,
  })).join('');
  const dot = unread ? '<span style="position:absolute;top:10px;right:10px;width:9px;height:9px;border-radius:50%;background:var(--accent)"></span>' : '';
  const html = `
    <header class="bar over">${menuBtn()}<span class="wordmark">Slouch<i>cat</i></span>
      <a class="ib" href="#/notifications" aria-label="Benachrichtigungen${unread ? `, ${unread} neu` : ''}" style="position:relative">${icon('bell')}${dot}</a>
      <a class="ib" href="#/profiles" aria-label="Profil wechseln, aktiv: ${esc(p.name)}">${avatar(p)}</a></header>
    ${hero()}
    <section><div class="rowh"><h2>Weiterschauen</h2></div><div class="hrow">${cont}</div></section>
    ${hrow('Neu erschienen', row('rel'), null, '#/list/rel')}
    ${hrow('Zuletzt hinzugefügt', row('rec'), null, '#/list/rec')}
    ${hrow('Empfohlen von anderen Profilen', row('picks'), () => ({ tag: icon('heart', 12) + ' Tipp' }), '#/list/picks')}
    <section><div class="rowh"><h2>Bibliotheken</h2></div><div class="chips" style="padding-bottom:6px">
      ${LIBS.map(l => `<a class="chip" href="#/library/${l.key}" style="height:48px;border-radius:12px;padding:0 16px">${icon(l.icon, 20)} ${esc(l.name)}</a>`).join('')}</div></section>
    ${hrow('Serien', row('lib6'), null, '#/library/series')}
    ${hrow('Filme', row('lib1'), null, '#/library/films')}
    ${S.watch.length ? hrow('Meine Merkliste', S.watch, null, '#/watchlist') : ''}
    ${hrow('Erfüllte Wünsche', row('wish'), null, '#/wishes')}
    ${hrow('Podcasts', row('lib4'), null, '#/library/podcasts')}`;
  return {
    nav: 'start', html,
    after: () => {
      clearInterval(heroTimer);
      heroTimer = setInterval(() => {
        if (!$('.hero')) { clearInterval(heroTimer); return; }
        heroAt = (heroAt + 1) % row('spot').length; showHero();
      }, 7000);
    },
  };
});
PAGES.push(['#/', 'Start']);
