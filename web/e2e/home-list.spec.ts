import type { Page } from '@playwright/test';
import { H264_TITLE, SHOW_TITLE } from './catalog';
import { ROW_TITLES } from './discover';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// Concept 20 P-7: the list behind a row title (/lists/$key) holds the row's titles without the
// row's cut, with the count, the filter by the kind of the top level and the pen of the server
// administrator on every card.

const grid = (page: Page) => page.getByRole('list', { name: ROW_TITLES.recent });
const cards = (page: Page) => grid(page).getByRole('link');
const chips = (page: Page) => page.getByRole('group', { name: 'Inhalt' });
// A card's name starts with its progress («28 % ») once a spec running alongside played the title.
const card = (title: string, end = '') => new RegExp(`^(\\d+ % )?${title}${end}`);

test('«Neu hinzugefügt» as a list: count, filter, pens, arrows, Enter, back; no focus trap @keyboard', async ({
  page,
}) => {
  await page.goto('/lists/recent');
  await expect(page.getByRole('heading', { level: 1, name: ROW_TITLES.recent })).toBeVisible();
  await expect(page).toHaveTitle(`${ROW_TITLES.recent} · Slouchcat`);
  const count = page.getByRole('status').filter({ hasText: /^\d+ Titel$/ });
  await expect(count).toBeVisible();
  const total = Number((await count.textContent())?.split(' ')[0]);
  await expect(cards(page)).toHaveCount(total);
  await expect(grid(page).getByRole('link', { name: card(H264_TITLE, ' \\d{4}$') })).toBeVisible();
  // The pen on the cards for the server administrator (every kind the editor edits).
  expect(
    await grid(page)
      .getByRole('button', { name: /: Metadaten bearbeiten$/ })
      .count(),
  ).toBeGreaterThan(0);

  // The filter «Serien» keeps the series only.
  await chips(page).getByRole('button', { name: 'Serien' }).click();
  await expect(page).toHaveURL(/\/lists\/recent\?content=series$/);
  await expect(
    grid(page)
      .getByRole('link', { name: card(SHOW_TITLE) })
      .first(),
  ).toBeVisible();
  await expect(grid(page).getByRole('link', { name: card(H264_TITLE) })).toHaveCount(0);
  await chips(page).getByRole('button', { name: 'Alle' }).click();
  await expect(cards(page)).toHaveCount(total);

  // Arrows: the first card, right and left; up leaves the grid for the filter; Enter on a chip
  // filters by keyboard too.
  await page.reload();
  await expect(cards(page)).toHaveCount(total);
  await resetFocus(page);
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, cards(page).first());
  if (total > 1) {
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, cards(page).nth(1));
    await page.keyboard.press('ArrowLeft');
    await expectFocusOn(page, cards(page).first());
  }
  await page.keyboard.press('ArrowUp');
  await expectVisibleFocus(page);
  await expect(chips(page).getByRole('button', { name: 'Alle' })).toBeFocused();
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, chips(page).getByRole('button', { name: 'Serien' }));
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\?content=series$/);
  await expect(chips(page).getByRole('button', { name: 'Serien' })).toHaveAttribute(
    'aria-pressed',
    'true',
  );

  // Enter on a card opens the title; back returns to the list.
  await page.goto('/lists/recent');
  await expect(cards(page)).toHaveCount(total);
  await resetFocus(page);
  await tabTo(page, page.getByRole('button', { name: 'Zurück' }));
  await tabTo(page, cards(page).first(), 8);
  const href = await cards(page).first().getAttribute('href');
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(href ?? /\/items\//);
  await page.goBack();
  await expect(page.getByRole('heading', { level: 1, name: ROW_TITLES.recent })).toBeVisible();
  await resetFocus(page);
  await expectNoFocusTrap(page, 160);

  // Escape goes back; opened directly, that is the start page.
  await resetFocus(page);
  await page.keyboard.press('Escape');
  await expect(page).toHaveURL((url) => url.pathname === '/');
});

test('an unknown list says so and leads to the start page', async ({ page }) => {
  await page.goto('/lists/nope');
  await expect(page.getByText('Liste nicht gefunden')).toBeVisible();
  await page.getByRole('link', { name: 'Zur Startseite' }).click();
  await expect(page).toHaveURL((url) => url.pathname === '/');
});
