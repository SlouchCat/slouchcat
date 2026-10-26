import { type Page } from '@playwright/test';
import { CSRF, EXTRAS_TITLE } from './catalog';
import { createPick, DEFAULT_HOME_ROWS, deletePick, setHomeRows } from './curation';
import { homeRowKeys, refreshSpotlight, ROW_TITLES, traktMockStatus } from './discover';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';
import { itemIdByTitle } from './playback';
import { TRAKT_ENTRIES } from './trakt-server.mjs';

/**
 * The stage: the row the server names in `heroRow`, drawn as the carousel (M11). Which row that is
 * is the server's decision; by default it is the spotlight row on top (DEFAULT_HOME_ROWS).
 */
const hero = (page: Page) => page.locator('main section[aria-roledescription]');
/** The band's poster strip: one button per slide, named after its title (step 7b). */
const strip = (page: Page) => hero(page).getByRole('list').getByRole('button');
/** The sections of the start page top to bottom: the stage (if any) and the poster rows. */
const sections = (page: Page) =>
  page.locator('main section:is([aria-labelledby], [aria-roledescription])');
const recentRow = (page: Page) => page.getByRole('region', { name: 'Neu hinzugefügt' });

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

test('a pick with a note shows in the picks row (E-2)', async ({ page, owner }) => {
  const note = 'E2E: unbedingt mit Ton schauen.';
  // A second pick next to the baseline pick of the global setup (the H264 film).
  const [pick] = await createPick(owner, {
    itemId: await itemIdByTitle(owner, EXTRAS_TITLE),
    note,
  });
  if (!pick) throw new Error('no pick created');
  try {
    await page.reload();
    // The picks are never the stage (plan 3.5): a poster row whose card shows the note.
    const picks = page.getByRole('region', { name: ROW_TITLES.picks });
    await expect(picks).not.toHaveAttribute('aria-roledescription');
    await expect(
      picks.getByRole('link').filter({ hasText: EXTRAS_TITLE }).getByText(note),
    ).toBeVisible();
  } finally {
    await deletePick(owner, pick.id);
  }
});

test('The spotlight band comes from the mock and goes while Trakt is down (E-7, I-7)', async ({
  page,
  owner,
}) => {
  // Two refreshes, each of which may wait for the server's one-a-minute limit (discover.ts).
  test.setTimeout(240_000);
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
  // The band shows the library titles the list points at (5 films and 2 series), one per slide;
  // the poster strip under it has one poster per slide, named after the title it shows.
  const entries = [...TRAKT_ENTRIES.movies, ...TRAKT_ENTRIES.shows].map((entry) => entry.title);
  const posters = strip(page);
  await expect(posters).toHaveCount(entries.length);
  const shown: string[] = [];
  for (let slide = 0; slide < entries.length; slide += 1) {
    await posters.nth(slide).click();
    await expect(posters.nth(slide)).toHaveAttribute('aria-current', 'true');
    const title = (await spotlight.getByRole('heading', { level: 2 }).textContent()) ?? '';
    await expect(posters.nth(slide)).toHaveAccessibleName(title);
    shown.push(title);
  }
  expect(shown.toSorted()).toEqual(entries.toSorted());

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

test('the band: arrow keys move in the poster strip, Enter selects, the focus stays (7b, C-4)', async ({
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
  // The carousel stands still (reduced motion, checked in the next test), so only the keys move it.
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.reload();
  await expect(hero(page)).toBeVisible();
  await resetFocus(page);
  const posters = strip(page);
  expect(await posters.count()).toBeGreaterThan(2);
  const heading = hero(page).getByRole('heading', { level: 2 });
  const first = (await posters.nth(0).getAttribute('aria-label')) ?? '';
  const second = (await posters.nth(1).getAttribute('aria-label')) ?? '';
  await expect(heading).toHaveText(first);

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

  // Enter shows the focused poster's slide; the poster keeps the focus.
  await page.keyboard.press('Enter');
  await expect(heading).toHaveText(second);
  await expect(posters.nth(1)).toHaveAttribute('aria-current', 'true');
  await expect(posters.nth(0)).not.toHaveAttribute('aria-current');
  await expectFocusOn(page, posters.nth(1));

  // The active poster is the strip's one Tab stop (roving tabindex): Tab
  // leaves the strip, Shift+Tab returns to it.
  await page.keyboard.press('Tab');
  expect(
    await posters.evaluateAll((buttons) =>
      buttons.includes(document.activeElement as HTMLButtonElement),
    ),
  ).toBe(false);
  await page.keyboard.press('Shift+Tab');
  await expectFocusOn(page, posters.nth(1));

  // The strip's first poster holds the focus on arrow left: it never leaves the band sideways.
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, posters.nth(0));
  await page.keyboard.press('ArrowLeft');
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
  // Focus and pointer stay out of the band, which pause it as well.
  await page.clock.install();
  await page.reload();
  await expect(hero(page)).toBeVisible();
  await resetFocus(page);
  const first = (await posters.nth(0).getAttribute('aria-label')) ?? '';
  const second = (await posters.nth(1).getAttribute('aria-label')) ?? '';

  // Without the preference the band moves on after its interval (8 s) ...
  await expect(heading).toHaveText(first);
  await page.clock.runFor(9000);
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
  } finally {
    await setHomeRows(owner, DEFAULT_HOME_ROWS);
  }
});

test('no stage: without the spotlight row the page starts with a poster row (3.5)', async ({
  page,
  owner,
}) => {
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
  try {
    const off = await put(false);
    expect(off.status(), await off.text()).toBe(200);
    expect(await homeRowKeys(owner)).not.toContain('spotlight');

    const shell = await page.reload();
    await expect(recentRow(page)).toBeVisible();
    await expect(hero(page)).toHaveCount(0);
    await expect(page.getByRole('region', { name: ROW_TITLES.spotlight })).toHaveCount(0);
    await expect(sections(page).first()).toHaveAttribute('aria-labelledby');
    await expect(sections(page).first()).not.toHaveAttribute('aria-roledescription');
    // The element, not the name: the shell's inline loader script names #hero-prepaint as well.
    expect(await shell?.text()).not.toContain('id="hero-prepaint"');
    await expect(page.locator('#hero-prepaint')).toHaveCount(0);
  } finally {
    const on = await put(true);
    expect(on.status(), await on.text()).toBe(200);
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

    // Down to the rows: the poster strip, the picks and "Weiterschauen" may stand between the
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

test('Tab: skip link, header, hero, then the cards; no focus trap', async ({ page }) => {
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('link', { name: 'Zum Inhalt springen' }));
  await tabTo(
    page,
    page
      .getByRole('navigation', { name: 'Hauptnavigation' })
      .getByRole('link', { name: 'Start', exact: true }),
  );
  await tabTo(page, page.getByRole('button', { name: 'Abmelden' }));
  if ((await hero(page).count()) > 0) {
    await tabTo(page, hero(page).getByRole('link', { name: 'Abspielen' }), 3);
    await tabTo(page, hero(page).getByRole('link', { name: 'Details' }), 2);
    // The poster strip is one Tab stop: its active poster.
    if ((await strip(page).count()) > 1) {
      await tabTo(page, strip(page).and(page.locator('[aria-current="true"]')), 1);
    }
  }
  await tabTo(
    page,
    page.locator('main section[aria-labelledby]').first().getByRole('link').first(),
    3,
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
