import { expect, test, type Page } from '@playwright/test';
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

const seasons = (page: Page) => page.getByRole('group', { name: 'Staffel wählen' });
const episodes = (page: Page) => page.getByRole('list', { name: 'Folgen' });

/** Index of the focused card in the episode list, or -1. */
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

test('seasons: Season 1 first, specials last; S01E03–E04 is one episode', async ({ page }) => {
  const chips = seasons(page).getByRole('button');
  await expect(chips).toHaveCount(2);
  await expect(chips.first()).toHaveAttribute('aria-pressed', 'true');
  await expect(chips.last()).toHaveText(/Specials|Season 0|Staffel 0/);
  await expect(episodes(page).getByRole('link')).toHaveCount(3);
  await expect(episodes(page).getByText('S01E01', { exact: true })).toBeVisible();
  await expect(episodes(page).getByText('S01E03–E04', { exact: true })).toBeVisible();
  await expect(page).toHaveTitle(`${SHOW_TITLE} · Slouchcat`);
});

test('arrows: back → seasons → episodes; Enter on a season chip shows the specials', async ({
  page,
}) => {
  // No play button on a series: the first arrow lands on "Zurück".
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, page.getByRole('button', { name: 'Zurück' }));

  for (let presses = 0; presses < 4; presses += 1) {
    if (await seasons(page).locator(':focus').count()) break;
    await page.keyboard.press('ArrowDown');
  }
  await expectFocusOn(page, seasons(page).getByRole('button').first());
  await page.keyboard.press('ArrowRight');
  const specials = seasons(page).getByRole('button').last();
  await expectFocusOn(page, specials);
  await page.keyboard.press('Enter');
  await expect(specials).toHaveAttribute('aria-pressed', 'true');
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
  await page.keyboard.press('ArrowRight');
  await expect.poll(() => focusedEpisode(page)).toBe(start + 1);
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
  await expectFocusOn(
    page,
    page.getByRole('link', { name: /^(Abspielen|Weiterschauen ab \d{2}:\d{2}:\d{2})$/ }),
  );
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

test('Tab: header, back, seasons, episodes; no focus trap', async ({ page }) => {
  await tabTo(page, page.getByRole('button', { name: 'Abmelden' }));
  await tabTo(page, page.getByRole('button', { name: 'Zurück' }), 3);
  // "Verwalten", then "Merken", "Favorit" and "Zu Cat's Empfehlungen" (M7).
  await tabTo(page, seasons(page).getByRole('button').first(), 7);
  await tabTo(page, episodes(page).getByRole('link').first(), 4);
  await expectNoFocusTrap(page);
});
