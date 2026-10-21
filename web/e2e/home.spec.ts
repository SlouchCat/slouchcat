import { type Page } from '@playwright/test';
import { EXTRAS_TITLE } from './catalog';
import { createPick, deletePick } from './curation';
import { homeRowKeys, refreshTrakt, ROW_TITLES, traktMockStatus } from './discover';
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

const hero = (page: Page) => page.getByRole('region', { name: "Cat's Empfehlungen" });
const recentRow = (page: Page) => page.getByRole('region', { name: 'Neu hinzugefügt' });

test.beforeEach(async ({ page }) => {
  await page.goto('/');
  await expect(recentRow(page).getByRole('link').first()).toBeVisible();
  await resetFocus(page);
});

test('rows: never an empty one, cards in a 2:3 frame, at most eight rows (E-1, E-9)', async ({
  page,
}) => {
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

test('a pick with a note shows in the hero (E-2)', async ({ page, owner }) => {
  const note = 'E2E: unbedingt mit Ton schauen.';
  // A second pick next to the baseline pick of the global setup (the H264 film).
  const [pick] = await createPick(owner, {
    itemId: await itemIdByTitle(owner, EXTRAS_TITLE),
    note,
  });
  if (!pick) throw new Error('no pick created');
  try {
    await page.reload();
    await expect(hero(page)).toBeVisible();
    // The pick is the newest; the carousel may start on another slide, so page through it.
    const slides = Math.max(1, await hero(page).locator('button[aria-label]').count());
    for (let slide = 0; slide < slides; slide += 1) {
      if (await hero(page).getByText(note).isVisible()) break;
      await hero(page).locator('button[aria-label]').nth(slide).click();
    }
    await expect(hero(page).getByText(note)).toBeVisible();
  } finally {
    await deletePick(owner, pick.id);
  }
});

test('Trakt rows come from the mock and go while Trakt is down (E-7, I-7)', async ({
  page,
  owner,
}) => {
  test.setTimeout(120_000);
  const trending = page.getByRole('region', { name: ROW_TITLES.traktTrending });
  const popular = page.getByRole('region', { name: ROW_TITLES.traktPopular });
  const both = ['trakt_trending', 'trakt_popular'];
  const traktRows = async () => (await homeRowKeys(owner)).filter((key) => both.includes(key));

  // The server asks the mock right after its start (no run within 24 h yet).
  await expect.poll(traktRows, { timeout: 60_000, intervals: [1000] }).toEqual(both);
  await page.reload();
  await expect(trending).toBeVisible();
  await expect(popular).toBeVisible();
  // Every row shows the library titles the lists point at (5 films and 2 series).
  for (const entry of [...TRAKT_ENTRIES.movies, ...TRAKT_ENTRIES.shows]) {
    await expect(
      trending.getByRole('link').filter({ has: page.getByText(entry.title, { exact: true }) }),
    ).toHaveCount(1);
  }

  try {
    // Trakt down: the refresh fails, the lists are emptied and both rows disappear (I-7).
    await traktMockStatus(503);
    await refreshTrakt(owner);
    await expect.poll(traktRows, { timeout: 60_000, intervals: [1000] }).toEqual([]);
    await page.reload();
    await expect(recentRow(page)).toBeVisible();
    await expect(trending).toHaveCount(0);
    await expect(popular).toHaveCount(0);
  } finally {
    await traktMockStatus(200);
    await refreshTrakt(owner);
    await expect.poll(traktRows, { timeout: 60_000, intervals: [1000] }).toEqual(both);
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
    const slides = await hero(page).locator('button[aria-label]').count();
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, details);
    if (slides > 1) {
      await expect(hero(page).getByRole('heading', { level: 2 })).not.toHaveText(before ?? '');
    }
    await page.keyboard.press('ArrowLeft');
    await expectFocusOn(page, play);

    // Down to the rows.
    for (let presses = 0; presses < 3; presses += 1) {
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
