import { expect, test, type Locator, type Page } from '@playwright/test';
import { itemPath, SHOW_TITLE } from './catalog';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// Series, seasons and episodes (B-1, B-2, C-2) by keyboard: start page → series → season →
// episode → player and back. Test media: Season 01 (S01E01, S01E02, S01E03-E04 as one file) and
// Specials (S00E01).

let series = '';

const shelf = (page: Page) => page.getByRole('list', { name: 'Staffeln' });
// The season cards, without the pens on their covers (concept 20 P-3).
const seasonCards = (page: Page) =>
  shelf(page).getByRole('button', { name: /^(?!.*: Metadaten bearbeiten$)/ });
const seasonPens = (page: Page) =>
  shelf(page).getByRole('button', { name: /: Metadaten bearbeiten$/ });
const episodes = (page: Page) => page.getByRole('list', { name: 'Folgen' });
// The big button of a series names its nextUp episode (concept 17 3.3); other specs may have left
// progress behind, so any of the four reasons.
const seriesPlay = (page: Page) =>
  page.getByRole('link', {
    name: /^(Abspielen|Weiterschauen|Nächste Folge|Nochmals ansehen) (S\d\dE\d\d|Alles gesehen)/,
  });
const episodePlay = (page: Page) =>
  page.getByRole('link', { name: /^(Abspielen|Fortsetzen|Nochmals ansehen)( |$)/ });

/**
 * Whether the season card stands wholly inside the shelf's scroll box (concept 20 P-1: the chosen
 * season moves into view by itself), whatever the page's own scroll position.
 */
const inShelf = (card: Locator) =>
  card.evaluate((element) => {
    const box = element.closest('ul')?.parentElement?.getBoundingClientRect();
    const place = element.getBoundingClientRect();
    return box !== undefined && place.left >= box.left - 1 && place.right <= box.right + 1;
  });

/** Index of the focused row in the episode list, or -1. */
const focusedEpisode = (page: Page) =>
  episodes(page)
    .getByRole('link')
    .evaluateAll((links) => links.indexOf(document.activeElement as HTMLAnchorElement));

test.beforeEach(async ({ page }) => {
  series ||= await itemPath(page, SHOW_TITLE);
  await page.goto('/');
  await page.goto(series);
  await expect(page.getByRole('heading', { level: 1, name: SHOW_TITLE })).toBeVisible();
  await expect(episodes(page).getByRole('link').first()).toBeVisible();
  await resetFocus(page);
});

test('the start page lists the series once and never its episodes', async ({ page }) => {
  await page.goto('/');
  const recent = page.getByRole('region', { name: 'Neu hinzugefügt' });
  await expect(recent.getByRole('link').filter({ hasText: SHOW_TITLE })).toHaveCount(1);
  await expect(recent.getByRole('link', { name: /S\d\dE\d\d|^Episode \d/ })).toHaveCount(0);
});

test('seasons: a shelf with Season 1 first, specials last; S01E03–E04 is one episode', async ({
  page,
}) => {
  await expect(seasonCards(page)).toHaveCount(2);
  await expect(seasonCards(page).first()).toHaveAttribute('aria-current', 'true');
  expect(await inShelf(seasonCards(page).first())).toBe(true);
  // Two seasons fit: the shelf's fixed paging buttons are gone (concept 20 P-1).
  const seasonsRegion = page.getByRole('region', { name: 'Staffeln' });
  await expect(
    seasonsRegion.getByRole('button', { name: /^(Zurück|Weiter)blättern$/ }),
  ).toHaveCount(0);
  await expect(seasonCards(page).first()).toContainText('3 Folgen');
  await expect(seasonCards(page).last()).toContainText(/Specials|Season 0|Staffel 0/);
  await expect(episodes(page).getByRole('link')).toHaveCount(3);
  await expect(episodes(page).getByText('S01E01', { exact: true })).toBeVisible();
  await expect(episodes(page).getByText('S01E03–E04', { exact: true })).toBeVisible();
  // The big button plays the nextUp episode of the series (E-10).
  await expect(seriesPlay(page)).toHaveAttribute('href', /^\/play\//);
  await expect(page).toHaveTitle(`${SHOW_TITLE} · Slouchcat`);
});

test('arrows: play → round actions → season shelf → episodes; Enter on a season shows the specials', async ({
  page,
}) => {
  // The start focus is the big button (E-10).
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, seriesPlay(page));
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, page.getByRole('button', { name: 'Zurück' }));
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, seriesPlay(page));

  for (let presses = 0; presses < 4; presses += 1) {
    if (await shelf(page).locator(':focus').count()) break;
    await page.keyboard.press('ArrowDown');
  }
  await expectFocusOn(page, seasonCards(page).first());
  await page.keyboard.press('ArrowRight');
  const specials = seasonCards(page).last();
  await expectFocusOn(page, specials);
  await page.keyboard.press('Enter');
  await expect(specials).toHaveAttribute('aria-current', 'true');
  expect(await inShelf(specials)).toBe(true);
  await expect(episodes(page).getByText('S00E01', { exact: true })).toBeVisible();
  await expect(episodes(page).getByRole('link')).toHaveCount(1);

  // Back to Season 1, then down into the episodes and along them.
  await page.keyboard.press('ArrowLeft');
  await page.keyboard.press('Enter');
  await expect(episodes(page).getByRole('link')).toHaveCount(3);
  await page.keyboard.press('ArrowDown');
  await expect.poll(() => focusedEpisode(page)).toBeGreaterThanOrEqual(0);
  await expectVisibleFocus(page);
  const start = await focusedEpisode(page);
  await page.keyboard.press(start < 2 ? 'ArrowDown' : 'ArrowUp');
  await expect.poll(() => focusedEpisode(page)).toBe(start < 2 ? start + 1 : start - 1);
  await expectVisibleFocus(page);
});

test('Enter opens an episode, play starts it; Backspace and Escape walk back', async ({ page }) => {
  const first = episodes(page).getByRole('link').first();
  const href = await first.getAttribute('href');
  await first.focus();
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(new RegExp(`${href ?? '/items/'}$`));
  await expect(page.getByText(`S01E01 · ${SHOW_TITLE}`)).toBeVisible();

  await resetFocus(page);
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, episodePlay(page));
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/play\//);
  const player = page.getByRole('main', { name: 'Videoplayer' });
  await expect(player).toHaveAttribute('data-start-ms', /\d+/);

  await page.keyboard.press('Backspace');
  await expect(page).toHaveURL(new RegExp(`${href ?? '/items/'}$`));
  await expect(page.getByText(`S01E01 · ${SHOW_TITLE}`)).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page).toHaveURL(new RegExp(`${series}$`));
});

test('Tab: header, back, actions, the chosen season, episodes; no focus trap @keyboard', async ({
  page,
}) => {
  await tabTo(page, page.getByRole('button', { name: 'Abmelden' }));
  await tabTo(page, page.getByRole('button', { name: 'Zurück' }), 3);
  // The pen and the watchlist button on the cover, play, then the round actions: "Merken",
  // "Favorit", "Empfehlen" and "Verwalten".
  await tabTo(page, seriesPlay(page), 4);
  await tabTo(page, page.getByRole('button', { name: 'Favorit' }), 4);
  // Only the chosen season is a Tab stop of the shelf (roving tabindex), then its pen.
  await tabTo(page, seasonCards(page).first(), 4);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, seasonPens(page).first());
  await page.keyboard.press('Tab');
  await expectFocusOn(page, episodes(page).getByRole('link').first());
  await expectNoFocusTrap(page);
});

test('the pen on a season cover opens its editor on «Bilder»; Escape returns to the pen (concept 20 P-3) @keyboard', async ({
  page,
}) => {
  // A pen on every season; only the chosen season's pen is a Tab stop.
  await expect(seasonPens(page)).toHaveCount(2);
  await expect(seasonPens(page).last()).toHaveAttribute('tabindex', '-1');
  const pen = seasonPens(page).first();
  const season = ((await pen.getAttribute('aria-label')) ?? '').replace(
    /: Metadaten bearbeiten$/,
    '',
  );
  await tabTo(page, seasonCards(page).first(), 40);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, pen);

  await page.keyboard.press('Enter');
  const editor = page.getByRole('dialog', { name: season });
  await expect(editor).toBeVisible();
  await expect(
    editor
      .getByRole('tablist', { name: 'Bereiche des Editors' })
      .getByRole('tab', { selected: true }),
  ).toHaveText(/^Bilder/);
  await page.keyboard.press('Escape');
  await expect(editor).toHaveCount(0);
  await expectFocusOn(page, pen);
});

test('season page by keyboard: the series link, play, the episodes; no focus trap @keyboard', async ({
  page,
}) => {
  // The season page of Season 1 (its id from the series' detail).
  const seasonPath = await page.evaluate(async (path) => {
    const response = await fetch(`/api/v1/items/${path.split('/').pop() ?? ''}`);
    const body = (await response.json()) as { seasons?: { id: string }[] };
    return `/items/${body.seasons?.[0]?.id ?? ''}`;
  }, series);
  await page.goto(seasonPath);
  await expect(page.getByRole('heading', { level: 1 })).toHaveText(/Season 1|Staffel 1/);
  const seriesLink = page.getByRole('link', { name: SHOW_TITLE, exact: true });
  await expect(seriesLink).toHaveAttribute('href', series);
  await expect(episodes(page).getByRole('link')).toHaveCount(3);
  // No watchlist and no favourite for a season (concept 17 E-15).
  await expect(page.getByRole('button', { name: /^(Merken|Gemerkt|Favorit)$/ })).toHaveCount(0);

  // Arrows: the start focus on play, ↑ to the series link, ↓ back, ↓ into the episodes.
  await resetFocus(page);
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, seriesPlay(page));
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, seriesLink);
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, seriesPlay(page));
  for (let presses = 0; presses < 4; presses += 1) {
    if ((await focusedEpisode(page)) >= 0) break;
    await page.keyboard.press('ArrowDown');
  }
  await expect.poll(() => focusedEpisode(page)).toBeGreaterThanOrEqual(0);
  await expectVisibleFocus(page);

  // Tab from the top: the back arrow, the series link, play, then the episodes.
  await resetFocus(page);
  await tabTo(page, page.getByRole('button', { name: 'Abmelden' }));
  await tabTo(page, page.getByRole('button', { name: 'Zurück' }), 3);
  await tabTo(page, seriesLink, 3);
  await tabTo(page, seriesPlay(page), 2);
  await tabTo(page, episodes(page).getByRole('link').first(), 6);
  await expectNoFocusTrap(page);

  // Enter on the series link leads to the series.
  await seriesLink.focus();
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(new RegExp(`${series}$`));
});
