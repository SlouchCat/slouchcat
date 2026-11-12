import { type APIRequestContext, type Locator, type Page } from '@playwright/test';
import { audioItemId, AUDIO_READY_MS, M4B_TITLE } from './audio';
import { CSRF, EXTRAS_TITLE, H264_TITLE, SHOW_TITLE } from './catalog';
import {
  DEFAULT_HOME_ROWS,
  deletePick,
  listHomeRows,
  recommendItem,
  setHomeRows,
} from './curation';
import { homeRowKeys, refreshSpotlight, ROW_TITLES, traktMockStatus } from './discover';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';
import { episodeIds, itemIdByTitle, useFreshProfile } from './playback';
import { TRAKT_ENTRIES } from './trakt-server.mjs';

/**
 * The stage: the row the server names in `heroRow`, drawn as the carousel (M11). Which row that is
 * is the server's decision; by default it is the spotlight row on top (DEFAULT_HOME_ROWS).
 */
const hero = (page: Page) => page.locator('main section[aria-roledescription]');
/**
 * The band's poster row (concept 08, 7.2): one card per slide, a button named after its title
 * (and the year under it). The administrator's pen beside each poster (concept 15 P-6) and the
 * watchlist button on its cover (concept 17 D-9, a toggle with `aria-pressed`) are buttons of the
 * row too, not posters.
 */
const strip = (page: Page) =>
  hero(page)
    .getByRole('list', { name: 'Spotlight-Titel' })
    .getByRole('button')
    .and(page.locator(':not([aria-label$=": Metadaten bearbeiten"], [aria-pressed])'));
/** The sections of the start page top to bottom: the stage (if any) and the poster rows. */
const sections = (page: Page) =>
  page.locator('main section:is([aria-labelledby], [aria-roledescription])');
const recentRow = (page: Page) => page.getByRole('region', { name: 'Neu hinzugefügt' });

/** A name that begins with `title` (a poster card's name is its title and the year under it). */
const startsWith = (title: string) =>
  new RegExp(`^${title.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}`);

/** The titles of the stage row in their order (GET /home; the e2e server begins at 0). */
async function spotlightTitles(api: APIRequestContext): Promise<string[]> {
  const response = await api.get('/api/v1/home');
  expect(response.status(), await response.text()).toBe(200);
  const { rows } = (await response.json()) as {
    rows: { key: string; items: { title: string }[] }[];
  };
  return (rows.find((row) => row.key === 'spotlight')?.items ?? []).map((item) => item.title);
}

/** Waits until the server has the spotlight row: it asks the Trakt mock right after its start. */
async function waitForSpotlight(api: APIRequestContext): Promise<void> {
  await expect
    .poll(async () => (await homeRowKeys(api)).includes('spotlight'), {
      timeout: 60_000,
      intervals: [1000],
    })
    .toBe(true);
}

/**
 * Distance of `card`'s left edge from the left edge of its row's visible part, less the page
 * gutter (the list's inline padding): 0 when the card stands at the row's left edge (the endless
 * band's leading poster, concept 15, 6.3).
 */
const offsetFromEdge = (card: Locator) =>
  card.evaluate((card) => {
    // The list item: the card itself may be scaled by the focus.
    const element = card.closest('li');
    const list = card.closest('ul');
    const scroller = list?.parentElement;
    if (!element || !scroller || !list) return Number.NaN;
    const gutter = parseFloat(getComputedStyle(list).paddingLeft);
    return element.getBoundingClientRect().left - scroller.getBoundingClientRect().left - gutter;
  });

/** The band's poster row scroller (the list's parent): what scrolls sideways. */
const stripScroller = (page: Page) =>
  hero(page).getByRole('list', { name: 'Spotlight-Titel' }).locator('..');

test.beforeEach(async ({ page }) => {
  await page.goto('/');
  await expect(recentRow(page).getByRole('link').first()).toBeVisible();
  await resetFocus(page);
});

test('rows: never an empty one, cards in a 2:3 frame, at most eight rows (E-1, E-9)', async ({
  page,
}) => {
  // The hero carries `aria-label`, not `aria-labelledby`: it is not counted here, so this checks at
  // most seven list rows plus the hero. Keep it that way, or the heading check below meets the
  // slide's title instead of a row title.
  const rows = page.locator('main section[aria-labelledby]');
  const count = await rows.count();
  expect(count).toBeGreaterThan(0);
  expect(count).toBeLessThanOrEqual(8);
  for (let index = 0; index < count; index += 1) {
    await expect(rows.nth(index).getByRole('link').first()).toBeVisible();
    // Only the eight row keys of E-1 exist (HomeRowKey); each has its title.
    const heading = await rows.nth(index).getByRole('heading', { level: 2 }).textContent();
    expect(Object.values(ROW_TITLES)).toContain(heading);
  }
});

test('a recommended title leads the picks row, once (E-2, concept 18 part B)', async ({
  page,
  owner,
}) => {
  // A second pick next to the baseline pick of the global setup (the H264 film); recommending it
  // twice renews the pick and keeps one card.
  const itemId = await itemIdByTitle(owner, EXTRAS_TITLE);
  await recommendItem(owner, itemId);
  await recommendItem(owner, itemId);
  try {
    await page.reload();
    // The picks are never the stage (plan 3.5): a poster row, newest pick first.
    const picks = page.getByRole('region', { name: ROW_TITLES.picks });
    await expect(picks).not.toHaveAttribute('aria-roledescription');
    const cards = picks.getByRole('list').getByRole('link');
    await expect(cards.first()).toContainText(EXTRAS_TITLE);
    await expect(cards.filter({ hasText: EXTRAS_TITLE })).toHaveCount(1);
  } finally {
    await deletePick(owner, itemId);
  }
});

test('The spotlight band comes from the mock and goes while Trakt is down (E-7, I-7)', async ({
  page,
  owner,
}) => {
  // Up to three polls of 60 s for the spotlight job; the refreshes wait at most the suite's 2 s
  // limit (discover.ts refreshSpotlight).
  test.setTimeout(120_000);
  // The spotlight row is the stage: the carousel named after it.
  const spotlight = hero(page).and(page.getByRole('region', { name: ROW_TITLES.spotlight }));
  const only = ['spotlight'];
  const traktRows = async () => (await homeRowKeys(owner)).filter((key) => only.includes(key));
  // Fails at once with a clear message when the mock does not run (a server of your own).
  await traktMockStatus(200);

  // The server asks the mock right after its start (no run within 24 h yet).
  await expect.poll(traktRows, { timeout: 60_000, intervals: [1000] }).toEqual(only);
  // The carousel stands still (as in admin-picks.spec.ts): no slide change between the checks.
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.reload();
  await expect(spotlight).toBeVisible();
  await expect(sections(page).first()).toHaveAttribute('aria-roledescription');
  // With a stage the server paints no cards ahead (P-10); read from the network, as the service
  // worker answers the navigation from its precache.
  expect(await (await page.request.get('/')).text()).not.toContain('id="poster-prepaint"');
  // The band shows the library titles the list points at (5 films and 2 series), one per slide;
  // the poster row under it has one poster per slide, named after the title it shows. A click
  // shows the slide and stays on the start page (F-5).
  const entries = [...TRAKT_ENTRIES.movies, ...TRAKT_ENTRIES.shows].map((entry) => entry.title);
  const posters = strip(page);
  await expect(posters).toHaveCount(entries.length);
  const shown: string[] = [];
  for (let slide = 0; slide < entries.length; slide += 1) {
    await posters.nth(slide).click();
    await expect(posters.nth(slide)).toHaveAttribute('aria-current', 'true');
    const title = (await spotlight.getByRole('heading', { level: 2 }).textContent()) ?? '';
    await expect(posters.nth(slide)).toHaveAccessibleName(startsWith(title));
    shown.push(title);
  }
  expect(shown.toSorted()).toEqual(entries.toSorted());
  await expect(page).toHaveURL(/\/$/);

  try {
    // Trakt down: the refresh fails, the list is emptied and the row disappears (I-7).
    await traktMockStatus(503);
    await refreshSpotlight(owner);
    await expect.poll(traktRows, { timeout: 60_000, intervals: [1000] }).toEqual([]);
    await page.reload();
    await expect(recentRow(page)).toBeVisible();
    await expect(spotlight).toHaveCount(0);
    await expect(hero(page)).toHaveCount(0);
  } finally {
    await traktMockStatus(200);
    await refreshSpotlight(owner);
    await expect.poll(traktRows, { timeout: 60_000, intervals: [1000] }).toEqual(only);
  }
});

test('the band: arrow keys move in the poster row, Enter selects, the focus stays on the first poster (7b, C-4, F-5, concept 15)', async ({
  page,
  owner,
}) => {
  test.setTimeout(90_000);
  await waitForSpotlight(owner);
  // The carousel stands still (reduced motion, checked in the next test), so only the keys move it.
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.reload();
  await expect(hero(page)).toBeVisible();
  await resetFocus(page);
  const posters = strip(page);
  expect(await posters.count()).toBeGreaterThan(2);
  const heading = hero(page).getByRole('heading', { level: 2 });
  const [first = '', second = ''] = await spotlightTitles(owner);
  await expect(heading).toHaveText(first);
  await expect(posters.nth(0)).toHaveAccessibleName(startsWith(first));

  // Down from the play button lands on the active poster.
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, hero(page).getByRole('link', { name: 'Abspielen' }));
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, posters.nth(0));

  // Arrow keys move the focus only: the slide stays until Enter.
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, posters.nth(1));
  await expect(heading).toHaveText(first);
  await expect(posters.nth(0)).toHaveAttribute('aria-current', 'true');
  await expect(posters.nth(1)).not.toHaveAttribute('aria-current');

  // Enter shows the focused poster's slide without leaving the page; the poster keeps the focus
  // and now leads the row (the endless band, concept 15, 6.3), with the row at its start.
  await page.keyboard.press('Enter');
  await expect(heading).toHaveText(second);
  await expect(posters.nth(1)).toHaveAttribute('aria-current', 'true');
  await expect(posters.nth(0)).not.toHaveAttribute('aria-current');
  await expectFocusOn(page, posters.nth(1));
  await expect(page).toHaveURL(/\/$/);
  await expect.poll(() => offsetFromEdge(posters.nth(1))).toBeCloseTo(0, 0);
  expect(await stripScroller(page).evaluate((element) => element.scrollLeft)).toBe(0);

  // The active poster is the row's one Tab stop (roving tabindex): Tab
  // leaves the row, Shift+Tab returns to it.
  await page.keyboard.press('Tab');
  expect(
    await posters.evaluateAll((buttons) =>
      buttons.includes(document.activeElement as HTMLButtonElement),
    ),
  ).toBe(false);
  await page.keyboard.press('Shift+Tab');
  await expectFocusOn(page, posters.nth(1));

  // Left of the row's first poster (the active one) is the band's way out to the side navigation,
  // on the current page's entry (concept 08, 7.1); right from there returns to the active poster.
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(
    page,
    page
      .getByRole('navigation', { name: 'Hauptnavigation' })
      .getByRole('link', { name: 'Start', exact: true }),
  );
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, posters.nth(1));
  await expect(heading).toHaveText(second);

  // The arrows follow the row as seen: right walks on after the active poster, and the poster
  // that was first before now stands last; right on it holds the focus (the end hold).
  const count = await posters.count();
  for (let position = 2; position < count; position += 1) {
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, posters.nth(position));
  }
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, posters.nth(0));
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, posters.nth(0));
  await expect(heading).toHaveText(second);

  // Up returns to the buttons, down again lands on the active poster, not the last focused one.
  await page.keyboard.press('ArrowUp');
  expect(
    await hero(page)
      .getByRole('link')
      .evaluateAll((links) => links.includes(document.activeElement as HTMLAnchorElement)),
  ).toBe(true);
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, posters.nth(1));
});

test('the trailer button on the stage: keys, dialog, paging (step 8, W-10, C-4)', async ({
  page,
  owner,
}) => {
  test.setTimeout(90_000);
  // The server asks the mock right after its start; the band needs its titles.
  await expect
    .poll(async () => (await homeRowKeys(owner)).includes('spotlight'), {
      timeout: 60_000,
      intervals: [1000],
    })
    .toBe(true);
  type Slide = { id: string; title: string; hasTrailer?: boolean };
  // The poster row comes after the pause button in the Tab order.
  const spotlightItems = async () => {
    const response = await owner.get('/api/v1/home');
    expect(response.status(), await response.text()).toBe(200);
    const { rows } = (await response.json()) as { rows: { key: string; items: Slide[] }[] };
    return rows.find((row) => row.key === 'spotlight')?.items ?? [];
  };
  const items = await spotlightItems();
  expect(items.length).toBeGreaterThan(2);
  const [first] = items;
  if (!first) throw new Error('no spotlight title');
  // The e2e server finds no trailers (no TMDB or YouTube key): a manual one for the first slide
  // only, as trailer.spec.ts does on the detail page.
  const created = await owner.post(`/api/v1/admin/items/${first.id}/trailers`, {
    headers: CSRF,
    data: { lang: 'de', youtubeId: 'dQw4w9WgXcQ' },
  });
  expect(created.status(), await created.text()).toBe(201);
  try {
    // D3: the spotlight row says which titles have a trailer; only the first one has.
    expect((await spotlightItems()).map((item) => item.hasTrailer)).toEqual(
      items.map((_, position) => position === 0),
    );
    await page.route('**://www.youtube-nocookie.com/**', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'text/html',
        body: '<!doctype html><title>yt</title>',
      }),
    );
    // The carousel stands still, so only the keys move it.
    await page.emulateMedia({ reducedMotion: 'reduce' });
    await page.reload();
    await expect(hero(page)).toBeVisible();
    await resetFocus(page);
    const posters = strip(page);
    const last = (await posters.count()) - 1;
    const play = hero(page).getByRole('link', { name: 'Abspielen' });
    const details = hero(page).getByRole('link', { name: 'Details' });
    const trailer = hero(page).getByRole('button', { name: 'Trailer', exact: true });
    await expect(hero(page).getByRole('heading', { level: 2 })).toHaveText(first.title);

    // Abspielen → Details → Trailer by arrows, and the same order by Tab; nothing pages on the way.
    await page.keyboard.press('ArrowDown');
    await expectFocusOn(page, play);
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, details);
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, trailer);
    await expectVisibleFocus(page);
    await expect(posters.nth(0)).toHaveAttribute('aria-current', 'true');
    await page.keyboard.press('Shift+Tab');
    await expectFocusOn(page, details);
    await page.keyboard.press('Tab');
    await expectFocusOn(page, trailer);

    // Enter opens the dialog of the slide's title; Escape closes it, the focus is back on the button.
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', { name: `Trailer: ${first.title}` });
    await expect(dialog).toBeVisible();
    await expect(dialog.locator('iframe')).toHaveAttribute(
      'src',
      /^https:\/\/www\.youtube-nocookie\.com\/embed\/dQw4w9WgXcQ/,
    );
    await expect(
      dialog.getByText('Der Trailer wird von YouTube abgespielt (youtube-nocookie.com).'),
    ).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(dialog).toHaveCount(0);
    await expect(page).toHaveURL(/\/$/);
    await expectFocusOn(page, trailer);
    await expectVisibleFocus(page);

    // The arrow keys still page on the outer buttons: left on Abspielen goes back to the last slide,
    // a title without a trailer, so the stage has no trailer button there.
    await page.keyboard.press('ArrowLeft');
    await expectFocusOn(page, details);
    await page.keyboard.press('ArrowLeft');
    await expectFocusOn(page, play);
    await page.keyboard.press('ArrowLeft');
    await expect(posters.nth(last)).toHaveAttribute('aria-current', 'true');
    await expectFocusOn(page, play);
    await expect(trailer).toHaveCount(0);

    // Without the trailer button Details is the last one: right on it pages forward.
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, details);
    await page.keyboard.press('ArrowRight');
    await expect(posters.nth(0)).toHaveAttribute('aria-current', 'true');
    await expectFocusOn(page, details);
    await expect(trailer).toBeVisible();

    // With it, right on Trailer pages forward; the next title has none, so the focus moves to
    // Details instead of getting lost with the button.
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, trailer);
    await page.keyboard.press('ArrowRight');
    await expect(posters.nth(1)).toHaveAttribute('aria-current', 'true');
    await expect(trailer).toHaveCount(0);
    await expectFocusOn(page, details);
    await expectVisibleFocus(page);
  } finally {
    // Leave the title without trailers for the other specs.
    const item = (await (await owner.get(`/api/v1/admin/items/${first.id}`)).json()) as {
      trailers: { id: number }[];
    };
    for (const entry of item.trailers) {
      await owner.delete(`/api/v1/admin/items/${first.id}/trailers/${entry.id}`, {
        headers: CSRF,
      });
    }
  }
});

test('prefers-reduced-motion stops the rotation (7b)', async ({ page, owner }) => {
  test.setTimeout(90_000);
  await expect
    .poll(async () => (await homeRowKeys(owner)).includes('spotlight'), {
      timeout: 60_000,
      intervals: [1000],
    })
    .toBe(true);
  const heading = hero(page).getByRole('heading', { level: 2 });
  const posters = strip(page);
  // The keyboard focus stays out of the band, which holds it as well.
  await page.clock.install();
  await page.reload();
  await expect(hero(page)).toBeVisible();
  await resetFocus(page);
  const [first = '', second = ''] = await spotlightTitles(owner);

  // Without the preference the band moves on after its interval (5 s, F-10) ...
  await expect(heading).toHaveText(first);
  await expect(hero(page)).not.toHaveAttribute('data-paused');
  await page.clock.runFor(6000);
  await expect(heading).toHaveText(second);

  // ... with it, it stays on its slide.
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.reload();
  await expect(hero(page)).toBeVisible();
  await resetFocus(page);
  await expect(heading).toHaveText(first);
  await page.clock.runFor(30_000);
  await expect(heading).toHaveText(first);
  await expect(posters.nth(0)).toHaveAttribute('aria-current', 'true');
  // No time bar; the band holds, everything else works.
  await expect(hero(page)).toHaveAttribute('data-paused');
  await expect(posters.nth(0).locator('[class*="timer"]')).toHaveCount(0);
  await posters.nth(1).click();
  await expect(heading).toHaveText(second);
});

test('one draw per page view: the shell start, start=K, no jump on a refetch (F-2, 6)', async ({
  page,
  owner,
}) => {
  test.setTimeout(90_000);
  await waitForSpotlight(owner);
  // The shell carries the draw: data-start on #hero-prepaint or, without a backdrop (the test
  // media have none), the start meta; the home preload asks for the same start. Read from the
  // network: the service worker answers navigations from its precache, without either.
  const shell = await page.request.get('/');
  const html = await shell.text();
  const drawn =
    /id="hero-prepaint" data-start="(\d+)"|<meta name="slouchcat-hero-start" content="(\d+)">/.exec(
      html,
    );
  expect(drawn, html).not.toBeNull();
  // SLOUCHCAT_HERO_START=0 in the e2e server (start-server.mjs).
  expect(Number(drawn?.[1] ?? drawn?.[2])).toBe(0);
  const links = shell
    .headersArray()
    .filter(({ name }) => name.toLowerCase() === 'link')
    .map(({ value }) => value)
    .join(', ');
  expect(links).toContain('</api/v1/home?start=0>; rel=preload; as=fetch');

  // The band begins with the drawn title, the first poster of its row.
  const titles = await spotlightTitles(owner);
  expect(titles.length).toBeGreaterThan(3);
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.clock.install();
  await page.reload();
  await expect(hero(page)).toBeVisible();
  const heading = hero(page).getByRole('heading', { level: 2 });
  const posters = strip(page);
  await expect(heading).toHaveText(titles[0] ?? '');
  await expect(posters.nth(0)).toHaveAttribute('aria-current', 'true');
  await expect(posters.nth(0)).toHaveAccessibleName(startsWith(titles[0] ?? ''));

  // start repeats an earlier draw, whatever the fixed start says.
  const repeated = await page.request.get('/api/v1/home?start=3');
  expect(repeated.status(), await repeated.text()).toBe(200);
  expect(((await repeated.json()) as { heroStart?: number }).heroStart).toBe(3);

  // A refetch on window focus brings the server's start again (0); the band stays where it is.
  await posters.nth(3).click();
  await expect(heading).toHaveText(titles[3] ?? '');
  await page.clock.fastForward(31_000);
  const refetch = page.waitForRequest(
    (request) => new URL(request.url()).pathname === '/api/v1/home',
  );
  await page.evaluate(() => window.dispatchEvent(new Event('visibilitychange')));
  const request = await refetch;
  expect(new URL(request.url()).search).toBe('');
  await request.response();
  await expect(heading).toHaveText(titles[3] ?? '');
  await expect(posters.nth(3)).toHaveAttribute('aria-current', 'true');
});

test('the band holds on keyboard focus and the pause button, not on a click (F-3, F-9)', async ({
  page,
  owner,
}) => {
  test.setTimeout(90_000);
  await waitForSpotlight(owner);
  await page.reload();
  await expect(hero(page)).toBeVisible();
  await resetFocus(page);
  const band = hero(page);
  const posters = strip(page);
  const timer = band.locator('[aria-current="true"] [class*="timer"]');
  await expect(band).not.toHaveAttribute('data-paused');
  await expect(timer).toHaveCount(1);

  // A mouse click on a poster shows its slide and does not hold the band: the press takes no focus
  // (PosterCard), so the hold for any focus in the poster row (concept 15, 6.3) does not apply.
  await posters.nth(1).click();
  await expect(posters.nth(1)).toHaveAttribute('aria-current', 'true');
  await expect(band).not.toHaveAttribute('data-paused');
  await expect(timer).toHaveCount(1);

  // Tab into the band holds it (keyboard focus), and leaving it lets it run again. Tab starts at
  // the side navigation's last button, the element before the band.
  await page.getByRole('button', { name: 'Seitenleiste ausblenden' }).focus();
  await tabTo(page, band.getByRole('link', { name: 'Abspielen' }), 3);
  await expect(band).toHaveAttribute('data-paused');
  await expect(timer).toHaveCount(0);
  // Details, Pause, the active poster with its pen, check (concept 20 P-5) and mark, then the row.
  await tabTo(
    page,
    page.locator('main section[aria-labelledby]').first().getByRole('link').first(),
    7,
  );
  await expect(band).not.toHaveAttribute('data-paused');

  // The pause button holds the band for everybody (aria-pressed), the second press resumes.
  await resetFocus(page);
  const pause = band.getByRole('button', { name: 'Pause' });
  await expect(pause).toHaveAttribute('aria-pressed', 'false');
  await pause.click();
  await expect(pause).toHaveAttribute('aria-pressed', 'true');
  await expect(band).toHaveAttribute('data-paused');
  await expect(timer).toHaveCount(0);
  await pause.click();
  await expect(pause).toHaveAttribute('aria-pressed', 'false');
  await expect(band).not.toHaveAttribute('data-paused');
  await expect(timer).toHaveCount(1);
});

test('the spotlight posters carry the pen: Tab poster then pen, the band holds, Escape returns (concept 15 P-6) @keyboard', async ({
  page,
  owner,
}) => {
  test.setTimeout(90_000);
  await waitForSpotlight(owner);
  await page.reload();
  await expect(hero(page)).toBeVisible();
  await resetFocus(page);
  const band = hero(page);
  const posters = strip(page);
  const row = band.getByRole('list', { name: 'Spotlight-Titel' });
  const pens = row.getByRole('button', { name: /: Metadaten bearbeiten$/ });
  // A pen on every poster of the row (the spotlight holds films and series), none on the stage.
  const count = await posters.count();
  expect(count).toBeGreaterThan(1);
  await expect(pens).toHaveCount(count);
  await expect(band.getByRole('button', { name: /: Metadaten bearbeiten$/ })).toHaveCount(count);
  // Each pen, check mark (concept 20 P-5) and watchlist button (concept 17 D-9) sits beside its
  // poster, not in its button, and turns with it (CSS order of the item).
  const marks = row.getByRole('button', { name: / merken$/ });
  await expect(marks).toHaveCount(count);
  const checks = row.getByRole('button', { name: /^Gesehen: / });
  await expect(checks).toHaveCount(count);
  for (let index = 0; index < count; index += 1) {
    const item = row.getByRole('listitem').nth(index);
    await expect(item.getByRole('button')).toHaveCount(4);
    await expect(item.locator('button button')).toHaveCount(0);
  }
  // Only the active poster's pen and mark are Tab stops (the row's roving tabindex).
  const active = posters.and(page.locator('[aria-current="true"]'));
  const activePen = active.locator('xpath=..').getByRole('button', {
    name: /: Metadaten bearbeiten$/,
  });
  const activeMark = active.locator('xpath=..').getByRole('button', { name: / merken$/ });
  const activeCheck = active.locator('xpath=..').getByRole('button', { name: /^Gesehen: / });
  await expect(pens.and(page.locator(':not([tabindex="-1"])'))).toHaveCount(1);
  await expect(marks.and(page.locator(':not([tabindex="-1"])'))).toHaveCount(1);
  await expect(checks.and(page.locator(':not([tabindex="-1"])'))).toHaveCount(1);

  // Tab: the active poster, its pen, its check, its mark; the band holds while the focus is in
  // the row.
  await page.getByRole('button', { name: 'Seitenleiste ausblenden' }).focus();
  await tabTo(page, active, 6);
  const shown = (await active.getAttribute('aria-label')) ?? (await active.textContent()) ?? '';
  await page.keyboard.press('Tab');
  await expectFocusOn(page, activePen);
  await expect(band).toHaveAttribute('data-paused');
  await page.keyboard.press('Tab');
  await expectFocusOn(page, activeCheck);
  await expect(band).toHaveAttribute('data-paused');
  await page.keyboard.press('Tab');
  await expectFocusOn(page, activeMark);
  await expect(band).toHaveAttribute('data-paused');
  await page.keyboard.press('Shift+Tab');
  await page.keyboard.press('Shift+Tab');
  await expectFocusOn(page, activePen);
  const title = ((await activePen.getAttribute('aria-label')) ?? '').replace(
    /: Metadaten bearbeiten$/,
    '',
  );
  expect(shown.startsWith(title)).toBe(true);

  // Enter opens the editor of that title; the band stays on its slide; Escape returns to the pen.
  await page.keyboard.press('Enter');
  const editor = page.getByRole('dialog', { name: title });
  await expect(editor).toBeVisible();
  await expect(band).toHaveAttribute('data-paused');
  await page.keyboard.press('Escape');
  await expect(editor).toHaveCount(0);
  await expectFocusOn(page, activePen);
  await expect(activePen).toHaveAccessibleName(`${title}: Metadaten bearbeiten`);
  await expect(band).toHaveAttribute('data-paused');

  // Shift+Tab back to the poster, Tab past the pen out of the band into the first poster row.
  await page.keyboard.press('Shift+Tab');
  await expectFocusOn(page, active);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, activePen);
  // The arrow keys pass the pen by: right moves on to the next poster as seen.
  await page.keyboard.press('ArrowRight');
  expect(
    await posters.evaluateAll((buttons) =>
      buttons.includes(document.activeElement as HTMLButtonElement),
    ),
  ).toBe(true);
  await expectVisibleFocus(page);
  await tabTo(
    page,
    page.locator('main section[aria-labelledby]').first().getByRole('link').first(),
    4,
  );
  await expect(band).not.toHaveAttribute('data-paused');
  await expectNoFocusTrap(page, 200);
});

test('the poster row is an endless band: the active poster leads, the DOM order stays (concept 15, 6.3)', async ({
  page,
  owner,
}) => {
  test.setTimeout(90_000);
  await waitForSpotlight(owner);
  await page.emulateMedia({ reducedMotion: 'reduce' });
  // A narrow row beside the side navigation (the pager shows from 768 px on).
  await page.setViewportSize({ width: 800, height: 900 });
  await page.reload();
  await expect(hero(page)).toBeVisible();
  await resetFocus(page);
  const titles = await spotlightTitles(owner);
  const posters = strip(page);
  const scroller = stripScroller(page);
  const items = hero(page).getByRole('list', { name: 'Spotlight-Titel' }).locator(':scope > li');
  const count = await posters.count();
  expect(count).toBe(titles.length);
  expect(count).toBeGreaterThan(2);
  const top = await page.evaluate(() => window.scrollY);
  const next = hero(page).getByRole('button', { name: 'Weiterblättern' });
  const previous = hero(page).getByRole('button', { name: 'Zurückblättern' });
  // The DOM keeps the server's order and every poster its fixed position, whatever leads.
  const keepsOrder = async () => {
    for (let position = 0; position < count; position += 1) {
      await expect(posters.nth(position)).toHaveAccessibleName(startsWith(titles[position] ?? ''));
      await expect(items.nth(position)).toHaveAttribute('aria-posinset', String(position + 1));
      await expect(items.nth(position)).toHaveAttribute('aria-setsize', String(count));
    }
  };
  // No room after the last poster: one list item per poster, none hidden.
  await expect(items).toHaveCount(count);
  await expect(items.and(page.locator('[aria-hidden]'))).toHaveCount(0);

  // The first poster leads; every change forward puts the next one first, with the marker.
  await expect.poll(() => offsetFromEdge(posters.nth(0))).toBeCloseTo(0, 0);
  for (let slide = 1; slide < count; slide += 1) {
    await next.click();
    await expect(posters.nth(slide)).toHaveAttribute('aria-current', 'true');
    await expect.poll(() => offsetFromEdge(posters.nth(slide))).toBeCloseTo(0, 0);
    // The one shown before stands last.
    expect(await offsetFromEdge(posters.nth(slide - 1))).toBeGreaterThan(
      await offsetFromEdge(posters.nth((slide + 1) % count)),
    );
  }
  await keepsOrder();
  // After the last title the first one leads again, without a jump back.
  await next.click();
  await expect(posters.nth(0)).toHaveAttribute('aria-current', 'true');
  await expect.poll(() => offsetFromEdge(posters.nth(0))).toBeCloseTo(0, 0);
  // Back: the last title comes to the front.
  await previous.click();
  await expect(posters.nth(count - 1)).toHaveAttribute('aria-current', 'true');
  await expect.poll(() => offsetFromEdge(posters.nth(count - 1))).toBeCloseTo(0, 0);
  await expect.poll(() => offsetFromEdge(posters.nth(0))).toBeGreaterThan(0);

  // A swipe scrolls the row freely; the next change brings it back to its start.
  await scroller.evaluate((element) => element.scrollTo({ left: 250 }));
  await expect.poll(() => scroller.evaluate((element) => element.scrollLeft)).toBeGreaterThan(0);
  await next.click();
  await expect(posters.nth(0)).toHaveAttribute('aria-current', 'true');
  await expect.poll(() => scroller.evaluate((element) => element.scrollLeft)).toBe(0);
  await expect.poll(() => offsetFromEdge(posters.nth(0))).toBeCloseTo(0, 0);
  await keepsOrder();
  expect(await page.evaluate(() => window.scrollY)).toBe(top);

  // With motion (held by the pause button, so only the click moves it), a click on the third
  // poster slides the row by two cards, then it leads.
  await page.emulateMedia({ reducedMotion: 'no-preference' });
  await page.reload();
  await expect(hero(page)).toBeVisible();
  await resetFocus(page);
  await hero(page).getByRole('button', { name: 'Pause' }).click();
  await expect(posters.nth(0)).toHaveAttribute('aria-current', 'true');
  await posters.nth(2).click();
  await expect(posters.nth(2)).toHaveAttribute('aria-current', 'true');
  await expect.poll(() => offsetFromEdge(posters.nth(2))).toBeCloseTo(0, 0);
  await expect.poll(() => offsetFromEdge(posters.nth(0))).toBeGreaterThan(0);
  await keepsOrder();

  // Forward, then back within the glide: the row stays as it was, and the next change forward (a
  // poster click, which sets no direction) glides forward again instead of turning at once.
  await page.evaluate(() => {
    const fills: unknown[] = [];
    (window as unknown as { glideFills: unknown[] }).glideFills = fills;
    const animate = Object.getOwnPropertyDescriptor(Element.prototype, 'animate')
      ?.value as Element['animate'];
    Element.prototype.animate = function (keyframes, options) {
      fills.push(typeof options === 'object' ? options.fill : undefined);
      return animate.call(this, keyframes, options);
    };
  });
  const forward = await next.elementHandle();
  const back = await previous.elementHandle();
  await page.evaluate(
    async ([forward, back]) => {
      (forward as HTMLElement | null)?.click();
      // The forward glide has started (it runs 450 ms); the step back comes one frame later.
      await new Promise((resolve) => requestAnimationFrame(resolve));
      (back as HTMLElement | null)?.click();
    },
    [forward, back] as const,
  );
  await expect(posters.nth(2)).toHaveAttribute('aria-current', 'true');
  await expect.poll(() => offsetFromEdge(posters.nth(2))).toBeCloseTo(0, 0);
  const target = (2 + 2) % count;
  await page.evaluate(() => {
    (window as unknown as { glideFills: unknown[] }).glideFills.length = 0;
  });
  await posters.nth(target).click();
  await expect(posters.nth(target)).toHaveAttribute('aria-current', 'true');
  // The forward glide holds its last frame (fill forwards); the turn at once has no fill.
  expect(
    await page.evaluate(() => (window as unknown as { glideFills: unknown[] }).glideFills),
  ).toContain('forwards');
  await expect.poll(() => offsetFromEdge(posters.nth(target))).toBeCloseTo(0, 0);
  await keepsOrder();
});

test('rows on a phone: three whole covers, a swipe snaps a cover to the left edge (N-3, N-4)', async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.reload();
  const row = recentRow(page);
  await expect(row.getByRole('link').first()).toBeVisible();
  const list = row.getByRole('list');
  const scroller = list.locator('..');
  const cards = list.locator(':scope > li');
  expect(await cards.count()).toBeGreaterThan(3);
  const box = await scroller.boundingBox();
  if (!box) throw new Error('row without box');
  const widths: number[] = [];
  for (let index = 0; index < 3; index += 1) {
    const card = await cards.nth(index).boundingBox();
    if (!card) throw new Error(`card ${index} without box`);
    expect(card.x).toBeGreaterThanOrEqual(box.x - 0.5);
    expect(card.x + card.width).toBeLessThanOrEqual(box.x + box.width + 0.5);
    widths.push(card.width);
  }
  // A third each, the fourth one starts beyond the right edge.
  widths.forEach((width) => expect(width).toBeCloseTo(widths[0] ?? 0, 1));
  const fourth = await cards.nth(3).boundingBox();
  expect(fourth?.x ?? 0).toBeGreaterThanOrEqual(box.x + box.width - 16.5);

  // Scrolled by part of a card, the row snaps a whole cover to the gutter.
  await scroller.evaluate((element) => element.scrollBy({ left: 70 }));
  await expect.poll(() => scroller.evaluate((element) => element.scrollLeft)).toBeGreaterThan(0);
  await expect
    .poll(async () => {
      const offsets = await cards.evaluateAll((items) => {
        const node = items[0]?.parentElement;
        const outer = node?.parentElement;
        if (!node || !outer) return [];
        const gutter = parseFloat(getComputedStyle(node).paddingLeft);
        const left = outer.getBoundingClientRect().left;
        return items.map((item) => item.getBoundingClientRect().left - left - gutter);
      });
      return Math.min(...offsets.map((offset) => Math.abs(offset)));
    })
    .toBeLessThan(1);
});

test('spotlight on a later position: the band stands third, nothing painted ahead (3.5)', async ({
  page,
  owner,
}) => {
  test.setTimeout(90_000);
  // The server asks the mock right after its start; the band needs its titles.
  await expect
    .poll(async () => (await homeRowKeys(owner)).includes('spotlight'), {
      timeout: 60_000,
      intervals: [1000],
    })
    .toBe(true);
  // Two poster rows above the stage: the baseline pick and the recently added titles.
  const order = [
    'picks',
    'recent',
    'spotlight',
    ...DEFAULT_HOME_ROWS.filter((key) => !['picks', 'recent', 'spotlight'].includes(key)),
  ];
  try {
    await setHomeRows(owner, order);
    const shell = await page.reload();
    await expect(hero(page)).toBeVisible();
    await expect(sections(page).nth(0)).toHaveAccessibleName(ROW_TITLES.picks);
    await expect(sections(page).nth(1)).toHaveAccessibleName(ROW_TITLES.recent);
    await expect(sections(page).nth(2)).toHaveAccessibleName(ROW_TITLES.spotlight);
    await expect(sections(page).nth(2)).toHaveAttribute('aria-roledescription');
    // The server paints the stage ahead only when it is on top. The test media have no backdrops,
    // so the shell never holds a prepaint here; the positive case lives in HomePage.test.tsx and
    // in the server's preload tests.
    // The element, not the name: the shell's inline loader script names #hero-prepaint as well.
    expect(await shell?.text()).not.toContain('id="hero-prepaint"');
    await expect(page.locator('#hero-prepaint')).toHaveCount(0);
    // A stage anywhere on the page: the server paints no cards ahead either (P-10).
    expect(await (await page.request.get('/')).text()).not.toContain('id="poster-prepaint"');
  } finally {
    await setHomeRows(owner, DEFAULT_HOME_ROWS);
  }
});

test('no stage: without the spotlight row the page starts with a poster row (3.5)', async ({
  page,
  owner,
}) => {
  test.setTimeout(AUDIO_READY_MS + 30_000);
  // The plan checks the start page without SLOUCHCAT_TRAKT_CLIENT_ID. The e2e server always runs
  // with the mock's client id, so the spotlight row is switched off instead: the server then omits
  // the row and `heroRow` alike, which is the same page as without the key.
  const stored = await owner.get('/api/v1/admin/spotlight');
  expect(stored.status(), await stored.text()).toBe(200);
  const settings = (await stored.json()) as Record<string, unknown>;
  // The catalogue is read only; the update takes the other fields.
  delete settings.listKinds;
  const put = (enabled: boolean) =>
    owner.put('/api/v1/admin/spotlight', { headers: CSRF, data: { ...settings, enabled } });
  // The test films have no artwork; the M4B book has a cover. As the newest pick its card is the
  // first one of the page, the card whose poster the shell preloads (P-10).
  const cover = { itemId: await audioItemId(owner, M4B_TITLE) };
  await recommendItem(owner, cover.itemId);
  try {
    const off = await put(false);
    expect(off.status(), await off.text()).toBe(200);
    expect(await homeRowKeys(owner)).not.toContain('spotlight');

    // P-10: the shell's Link header preloads the first card's poster with high priority. It is
    // read from the network: after the first visit the service worker answers the navigation
    // from its precache, without the per-request header. The cover may still be on its way.
    let link = '';
    await expect
      .poll(
        async () => {
          link = (await page.request.get('/'))
            .headersArray()
            .filter(({ name }) => name.toLowerCase() === 'link')
            .map(({ value }) => value)
            .join(', ');
          return link;
        },
        { timeout: AUDIO_READY_MS, intervals: [1000] },
      )
      .toMatch(/\/images\/poster\/[^>]*>; rel=preload; as=image;.*?fetchpriority=high/);
    expect(/<[^>]*\/items\/([^/]+)\/images\/poster\//.exec(link)?.[1], link).toBe(cover.itemId);

    const shell = await page.reload();
    await expect(recentRow(page)).toBeVisible();
    await expect(hero(page)).toHaveCount(0);
    await expect(page.getByRole('region', { name: ROW_TITLES.spotlight })).toHaveCount(0);
    await expect(sections(page).first()).toHaveAttribute('aria-labelledby');
    await expect(sections(page).first()).not.toHaveAttribute('aria-roledescription');
    // The element, not the name: the shell's inline loader script names #hero-prepaint as well.
    expect(await shell?.text()).not.toContain('id="hero-prepaint"');
    await expect(page.locator('#hero-prepaint')).toHaveCount(0);
    // The card fetches the preloaded title's poster with high priority as well.
    // The first card (the picks row's header links its page as well, concept 08).
    const firstCard = sections(page)
      .first()
      .getByRole('list')
      .getByRole('link')
      .first()
      // The cover, not its blurred ground (a square cover, concept 18 C-1).
      .locator('img:not([aria-hidden])');
    await expect(firstCard).toHaveAttribute('fetchpriority', 'high');
    await expect(firstCard).toHaveAttribute(
      'src',
      new RegExp(`/items/${cover.itemId}/images/poster/`),
    );

    // P-10: the shell paints the first cards ahead, the cover first and with priority. Read from
    // the network like the header above. Attribute values hold ">" (sizes), so no [^>] here.
    const html = await (await page.request.get('/')).text();
    const prepaint = /<div id="poster-prepaint"[^>]*>.*?<\/div>/.exec(html)?.[0] ?? '';
    expect(prepaint, html).toMatch(
      new RegExp(
        `^<div id="poster-prepaint" aria-hidden="true"><img class="square" src="[^"]*/items/${cover.itemId}/images/poster/.*? fetchpriority="high" decoding="sync">`,
      ),
    );
    // No layout shift when the app takes over: the prepainted cover lies exactly on the card's
    // frame (a square cover sits in the middle of both, concept 18 C-1). The markup goes in where
    // the shell has it, in front of #root; the card is measured at rest, without the focus scale.
    await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
    const firstLink = sections(page).first().getByRole('list').getByRole('link').first();
    await expect
      .poll(() => firstLink.evaluate((link) => getComputedStyle(link).transform))
      .toBe('none');
    const [painted, card] = await page.evaluate((markup) => {
      document.getElementById('poster-prepaint')?.remove();
      window.scrollTo(0, 0);
      document.getElementById('root')?.insertAdjacentHTML('beforebegin', markup);
      const box = (element: Element | null | undefined) => {
        const rect = element?.getBoundingClientRect();
        return rect ? [rect.left, rect.top, rect.width, rect.height] : [];
      };
      return [
        box(document.querySelector('#poster-prepaint > img')),
        box(
          document.querySelector('main section[aria-labelledby] a img')?.closest('a')
            ?.firstElementChild,
        ),
      ];
    }, prepaint);
    expect(painted).toHaveLength(4);
    // Under a pixel: the prepainted poster is half a pixel wider than the card on purpose, so it
    // stays the larger LCP candidate (global.css, docs/perf/startseite.md 3).
    painted.forEach((value, index) =>
      expect(Math.abs(value - (card[index] ?? Number.NaN))).toBeLessThan(1),
    );
  } finally {
    const on = await put(true);
    expect(on.status(), await on.text()).toBe(200);
    await deletePick(owner, cover.itemId);
  }
});

/** Reports a position of a title for the profile of `api` (an online progress event). */
async function reportProgress(
  api: APIRequestContext,
  itemId: string,
  positionMs: number,
  state: 'paused' | 'ended' = 'paused',
): Promise<void> {
  const response = await api.post(`/api/v1/items/${itemId}/progress`, {
    headers: CSRF,
    data: {
      positionMs,
      durationMs: 1_800_000,
      playedMs: positionMs,
      seq: 1,
      state,
      clientTime: new Date().toISOString(),
    },
  });
  expect(response.ok(), await response.text()).toBe(true);
}

test('"Weiterschauen" per series: the series card, arrow keys in the row, Enter opens the episode (concept 15 P-1, F-1) @keyboard', async ({
  page,
  owner,
}) => {
  // A profile of its own: its progress is only this test's, the owner's rows stay as they are.
  await useFreshProfile(page, owner, 'E2E Weiterschauen');
  const film = await itemIdByTitle(owner, H264_TITLE);
  const [first, , double] = await episodeIds(owner, await itemIdByTitle(owner, SHOW_TITLE));
  if (!first || !double) throw new Error('season 1 of the test show is incomplete');
  const row = page.getByRole('region', { name: ROW_TITLES.continue });
  const cards = row.getByRole('link');
  const seriesCard = row.getByRole('link', { name: new RegExp(`^${SHOW_TITLE}, `) });

  // The first episode watched to the end: the series stays with the next one, not started yet.
  await reportProgress(page.request, film, 600_000);
  await reportProgress(page.request, first, 1_800_000, 'ended');
  await page.goto('/');
  await expect(seriesCard).toHaveCount(1);
  await expect(seriesCard).toHaveAccessibleName(new RegExp(`^${SHOW_TITLE}, S1 · F2, .+, Neu$`));
  await expect(seriesCard.getByText('S1 · F2 · Neu', { exact: true })).toBeVisible();
  await expect(seriesCard.getByRole('progressbar')).toHaveCount(0);

  // The double episode started last: one card for the series, with the code and the time left.
  await reportProgress(page.request, double, 600_000);
  await page.goto('/');
  await expect(seriesCard).toHaveCount(1);
  await expect(seriesCard).toHaveAccessibleName(
    new RegExp(`^${SHOW_TITLE}, S1 · F3–F4, .+, Noch 20 min$`),
  );
  await expect(seriesCard.getByText(SHOW_TITLE, { exact: true })).toBeVisible();
  await expect(seriesCard.getByText('S1 · F3–F4 · Noch 20 min', { exact: true })).toBeVisible();
  await expect(seriesCard.getByRole('progressbar', { name: '33 % gesehen' })).toBeVisible();
  await expect(seriesCard).toHaveAttribute('href', `/items/${double}`);
  // The film beside it stays a film card.
  await expect(cards).toHaveCount(2);

  // Down from the top into the row, then the arrow keys move along it as in every row.
  await resetFocus(page);
  const inRow = () =>
    cards.evaluateAll((links) => links.indexOf(document.activeElement as HTMLAnchorElement));
  for (let presses = 0; presses < 8 && (await inRow()) < 0; presses += 1) {
    await page.keyboard.press('ArrowDown');
  }
  expect(await inRow()).toBeGreaterThanOrEqual(0);
  await expectVisibleFocus(page);
  const start = await inRow();
  const step = start === 0 ? 'ArrowRight' : 'ArrowLeft';
  await page.keyboard.press(step);
  await expect.poll(inRow).toBe(start === 0 ? 1 : start - 1);
  await expectVisibleFocus(page);
  await page.keyboard.press(step === 'ArrowRight' ? 'ArrowLeft' : 'ArrowRight');
  await expect.poll(inRow).toBe(start);

  // Onto the series card; Enter opens the episode's page (F-1), the next arrow lands on resume.
  const seriesIndex = await seriesCard.evaluate((card) =>
    [...(card.closest('ul')?.querySelectorAll('a') ?? [])].indexOf(card as HTMLAnchorElement),
  );
  if (seriesIndex !== start) {
    await page.keyboard.press(seriesIndex > start ? 'ArrowRight' : 'ArrowLeft');
  }
  await expectFocusOn(page, seriesCard);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(new RegExp(`/items/${double}$`));
  // The big button resumes the episode (concept 17 E-10: «Fortsetzen» with the time left).
  // Once the page stands: an arrow pressed while it still loads would find no start focus.
  await expect(page.getByRole('link', { name: /^Fortsetzen bei / })).toBeVisible();
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, page.getByRole('link', { name: /^Fortsetzen bei / }));
});

/** The libraries the owner's main profile sees, in the order of the side navigation. */
async function visibleLibraries(api: APIRequestContext): Promise<{ id: number; name: string }[]> {
  const response = await api.get('/api/v1/libraries/visible');
  expect(response.status(), await response.text()).toBe(200);
  return (await response.json()) as { id: number; name: string }[];
}

test('«Meine Medien» and «Neu in …»: tiles by arrow keys and Tab, Enter opens the library, back returns (concept 15 P-7) @keyboard', async ({
  page,
  owner,
}) => {
  const libraries = await visibleLibraries(owner);
  const films = libraries.find((library) => library.name === 'Filme');
  if (!films) throw new Error('the e2e library «Filme» is missing');
  const row = (await listHomeRows(owner)).find((entry) => entry.libraryId === films.id);
  if (!row) throw new Error('the row «Neu in Filme» is missing');
  try {
    // «Meine Medien» first, «Neu in Filme» second, the spotlight band further down.
    await setHomeRows(owner, ['libraries', row.id, ...DEFAULT_HOME_ROWS]);
    await page.goto('/');
    const mine = page.getByRole('region', { name: 'Meine Medien' });
    const tiles = mine.getByRole('link');
    await expect(sections(page).nth(0)).toHaveAccessibleName('Meine Medien');
    await expect(sections(page).nth(1)).toHaveAccessibleName('Neu in Filme');
    // A tile per library the profile sees, in the order of the side navigation, with its count.
    await expect(tiles).toHaveCount(libraries.length);
    for (const [index, library] of libraries.entries()) {
      await expect(tiles.nth(index)).toHaveAttribute('href', `/libraries/${library.id}`);
      await expect(tiles.nth(index)).toHaveAccessibleName(new RegExp(`^${library.name}\\s*\\d`));
    }
    // 16:9, two cards wide (Row.module.css .tile).
    const box = await tiles.first().boundingBox();
    expect(box && Math.abs(box.width / box.height - 16 / 9)).toBeLessThan(0.02);

    // The page starts on the first tile (no stage on top); the arrow keys move along the tiles.
    await resetFocus(page);
    const inTiles = () =>
      tiles.evaluateAll((links) => links.indexOf(document.activeElement as HTMLAnchorElement));
    for (let presses = 0; presses < 3 && (await inTiles()) < 0; presses += 1) {
      await page.keyboard.press('ArrowDown');
    }
    expect(await inTiles()).toBe(0);
    await expectVisibleFocus(page);
    if (libraries.length > 1) {
      await page.keyboard.press('ArrowRight');
      await expect.poll(inTiles).toBe(1);
      await expectVisibleFocus(page);
      await expect(tiles.nth(1)).toBeInViewport();
      await page.keyboard.press('ArrowLeft');
      await expect.poll(inTiles).toBe(0);
    }
    // Down into «Neu in Filme», up back to the tiles.
    const filmCards = page.getByRole('region', { name: 'Neu in Filme' }).getByRole('link');
    await page.keyboard.press('ArrowDown');
    await expect
      .poll(() =>
        filmCards.evaluateAll((links) =>
          links.includes(document.activeElement as HTMLAnchorElement),
        ),
      )
      .toBe(true);
    await page.keyboard.press('ArrowUp');
    await expect.poll(inTiles).toBe(0);

    // Enter opens the library page, browser back returns to the start page.
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(new RegExp(`/libraries/${libraries[0].id}$`));
    await expect(page.getByRole('heading', { level: 1, name: libraries[0].name })).toBeVisible();
    await page.goBack();
    await expect(page).toHaveURL(/\/$/);
    await expect(mine).toBeVisible();

    // Tab reaches the first tile after the side navigation, then «Alle anzeigen ›» of «Neu in
    // Filme»: Enter opens the library sorted by «Zuletzt hinzugefügt».
    await resetFocus(page);
    await tabTo(page, page.getByRole('button', { name: 'Seitenleiste ausblenden' }));
    await tabTo(page, tiles.first(), 3);
    const all = page
      .getByRole('region', { name: 'Neu in Filme' })
      .getByRole('link', { name: 'Alle anzeigen ›' });
    await expect(all).toHaveAttribute('href', `/libraries/${films.id}?sort=added`);
    await tabTo(page, all, libraries.length + 2);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(new RegExp(`/libraries/${films.id}\\?sort=added$`));
    await expect(page.getByRole('heading', { level: 1, name: 'Filme' })).toBeVisible();
    await page.goBack();
    await expect(mine).toBeVisible();
    await resetFocus(page);
    await expectNoFocusTrap(page, 160);
  } finally {
    await setHomeRows(owner, DEFAULT_HOME_ROWS);
  }
});

test('arrow keys move hero → rows → header, Enter opens, back returns', async ({ page }) => {
  const hasHero = (await hero(page).count()) > 0;
  const firstCard = recentRow(page).getByRole('link').first();
  const cards = recentRow(page).getByRole('link');

  await page.keyboard.press('ArrowDown');
  if (hasHero) {
    const play = hero(page).getByRole('link', { name: 'Abspielen' });
    const details = hero(page).getByRole('link', { name: 'Details' });
    await expectFocusOn(page, play);
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, details);

    // Right on the last button pages the carousel; focus stays on the button.
    const before = await hero(page).getByRole('heading', { level: 2 }).textContent();
    const slides = await strip(page).count();
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, details);
    if (slides > 1) {
      await expect(hero(page).getByRole('heading', { level: 2 })).not.toHaveText(before ?? '');
    }
    await page.keyboard.press('ArrowLeft');
    await expectFocusOn(page, play);

    // Down to the rows: the poster row, the picks and "Weiterschauen" may stand between the
    // buttons and "Neu hinzugefügt".
    for (let presses = 0; presses < 5; presses += 1) {
      if (
        await recentRow(page)
          .getByRole('link')
          .evaluateAll((links) => links.includes(document.activeElement as HTMLAnchorElement))
      )
        break;
      await page.keyboard.press('ArrowDown');
    }
  } else {
    for (let presses = 0; presses < 2; presses += 1) {
      if (await firstCard.evaluate((element) => element === document.activeElement)) break;
      await page.keyboard.press('ArrowDown');
    }
  }

  const inRecent = () =>
    cards.evaluateAll((links) => links.indexOf(document.activeElement as HTMLAnchorElement));
  expect(await inRecent()).toBeGreaterThanOrEqual(0);
  await expectVisibleFocus(page);

  if ((await cards.count()) > 1) {
    const start = await inRecent();
    await page.keyboard.press('ArrowRight');
    await expect.poll(inRecent).toBe(start + 1);
    await expectVisibleFocus(page);
    // The focused card is scrolled into view.
    await expect(cards.nth(start + 1)).toBeInViewport();
    await page.keyboard.press('ArrowLeft');
    await expect.poll(inRecent).toBe(start);
  }

  // Enter opens the detail page, browser back returns.
  const href = await page.evaluate(() => document.activeElement?.getAttribute('href'));
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(new RegExp(`${href ?? '/items/'}$`));
  await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
  await page.goBack();
  await expect(page).toHaveURL(/\/$/);
  await expect(recentRow(page)).toBeVisible();

  // Up from the top content lands in the header.
  await resetFocus(page);
  await page.keyboard.press('ArrowDown');
  for (let presses = 0; presses < 6; presses += 1) {
    const inHeader = await page.evaluate(
      () =>
        document.activeElement?.closest('header') !== null &&
        document.activeElement !== document.body,
    );
    if (inHeader) break;
    await page.keyboard.press('ArrowUp');
  }
  expect(await page.evaluate(() => document.activeElement?.closest('header') !== null)).toBe(true);
  await expectVisibleFocus(page);
});

// Concept 08: the side navigation stands between the header and the page (DOM order), so Tab goes
// skip link, header, the navigation's column (the profile card's «Wechseln» and Start first,
// "Seitenleiste ausblenden" last), page.
test('Tab: skip link, header, side navigation, hero, then the cards; no focus trap @keyboard', async ({
  page,
}) => {
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('link', { name: 'Zum Inhalt springen' }));
  await tabTo(page, page.locator('header').getByRole('link', { name: /^Profil wechseln/ }), 5);
  // The column's profile card first (concept 17, D-8), then «Start».
  await tabTo(
    page,
    page
      .getByRole('navigation', { name: 'Hauptnavigation' })
      .getByRole('link', { name: 'Start', exact: true }),
    2,
  );
  await tabTo(page, page.getByRole('button', { name: 'Seitenleiste ausblenden' }));
  if ((await hero(page).count()) > 0) {
    await tabTo(page, hero(page).getByRole('link', { name: 'Abspielen' }), 3);
    await tabTo(page, hero(page).getByRole('link', { name: 'Details' }), 2);
    // Then the pause button (F-9) and the poster row as one Tab stop: its active poster.
    if ((await strip(page).count()) > 1) {
      await tabTo(page, hero(page).getByRole('button', { name: 'Pause' }), 2);
      await tabTo(page, strip(page).and(page.locator('[aria-current="true"]')), 1);
    }
  }
  // After the active poster its pen, check (concept 20 P-5) and mark.
  await tabTo(
    page,
    page.locator('main section[aria-labelledby]').first().getByRole('link').first(),
    4,
  );
  await expectNoFocusTrap(page, 120);
});

test('Enter on the skip link moves focus past the header', async ({ page }) => {
  await page.keyboard.press('Tab');
  await page.keyboard.press('Enter');
  await page.keyboard.press('Tab');
  const inHeader = await page.evaluate(() => document.activeElement?.closest('header') !== null);
  expect(inHeader).toBe(false);
  await expectVisibleFocus(page);
});
