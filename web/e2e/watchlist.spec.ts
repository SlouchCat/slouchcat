import { type Page } from '@playwright/test';
import { EXTRAS_TITLE, H264_TITLE } from './catalog';
import { ROW_TITLES, setWatchlist, watchlistTitles } from './discover';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';
import { itemIdByTitle, useFreshProfile } from './playback';

// E-5: a watchlist per profile; the start page shows it as a row while it holds titles.

const grid = (page: Page) => page.getByRole('list', { name: 'Merkliste' });
const watchlistRow = (page: Page) => page.getByRole('region', { name: ROW_TITLES.watchlist });

test('"Merken" on the detail page fills the list and the row, per profile (E-5)', async ({
  page,
  owner,
}) => {
  const id = await itemIdByTitle(owner, H264_TITLE);
  await page.goto(`/items/${id}`);
  await page.getByRole('button', { name: 'Merken' }).click();
  await expect(page.getByRole('button', { name: 'Gemerkt' })).toBeVisible();
  expect(await watchlistTitles(owner)).toEqual([H264_TITLE]);

  await page.goto('/');
  await expect(
    watchlistRow(page).getByRole('link', { name: new RegExp(H264_TITLE) }),
  ).toBeVisible();

  await page.goto('/watchlist');
  await expect(page.getByRole('heading', { level: 1, name: 'Merkliste' })).toBeVisible();
  await expect(
    page
      .getByRole('navigation', { name: 'Hauptnavigation' })
      .getByRole('link', { name: 'Merkliste' }),
  ).toHaveAttribute('aria-current', 'page');
  await expect(grid(page).getByRole('link')).toHaveCount(1);

  // Another profile of the account has a list of its own.
  await useFreshProfile(page, owner, 'E2E Merkliste');
  await page.goto('/watchlist');
  await expect(page.getByText('Deine Merkliste ist leer')).toBeVisible();
  await page.goto('/');
  await expect(page.getByRole('region', { name: ROW_TITLES.recent })).toBeVisible();
  await expect(watchlistRow(page)).toHaveCount(0);
  expect(await watchlistTitles(owner)).toEqual([H264_TITLE]);
});

test('"Entfernen" takes a title off; the row goes with the last one (E-9)', async ({
  page,
  owner,
}) => {
  await setWatchlist(owner, await itemIdByTitle(owner, H264_TITLE), true);
  await page.goto('/watchlist');
  await grid(page)
    .getByRole('button', { name: `${H264_TITLE} von der Merkliste entfernen` })
    .click();
  await expect(page.getByText('Deine Merkliste ist leer')).toBeVisible();
  await page.goto('/');
  await expect(page.getByRole('region', { name: ROW_TITLES.recent })).toBeVisible();
  await expect(watchlistRow(page)).toHaveCount(0);
});

test('watchlist by keyboard: arrows, Enter, remove, back; no focus trap', async ({
  page,
  owner,
}) => {
  // Newest first: the extras film was added last.
  await setWatchlist(owner, await itemIdByTitle(owner, H264_TITLE), true);
  await setWatchlist(owner, await itemIdByTitle(owner, EXTRAS_TITLE), true);
  await page.goto('/watchlist');
  const cards = grid(page).getByRole('link');
  await expect(cards).toHaveCount(2);
  await resetFocus(page);

  // The first arrow lands on the first card; right moves to the next, down to its button.
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, cards.first());
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, cards.nth(1));
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, cards.first());
  const remove = grid(page).getByRole('button', {
    name: `${EXTRAS_TITLE} von der Merkliste entfernen`,
  });
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, remove);

  // Enter removes; the focus moves to the next title's button.
  await page.keyboard.press('Enter');
  await expect(cards).toHaveCount(1);
  await expectFocusOn(
    page,
    grid(page).getByRole('button', { name: `${H264_TITLE} von der Merkliste entfernen` }),
  );
  expect(await watchlistTitles(owner)).toEqual([H264_TITLE]);

  // Enter on the card opens the title; the browser's back returns.
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, cards.first());
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/items\//);
  await page.goBack();
  await expect(page).toHaveURL(/\/watchlist$/);
  await expect(cards).toHaveCount(1);

  // Tab from a fresh load (headless Firefox does not wrap Tab around once it reached the end):
  // header, back, the card and its button; then the end of the page.
  await page.reload();
  await expect(cards).toHaveCount(1);
  await resetFocus(page);
  await tabTo(page, page.getByRole('button', { name: 'Zurück' }));
  await tabTo(page, cards.first(), 3);
  await expectVisibleFocus(page);
  await expectNoFocusTrap(page);

  // Escape goes back.
  await resetFocus(page);
  await page.keyboard.press('Escape');
  await expect(page).not.toHaveURL(/\/watchlist$/);
});
