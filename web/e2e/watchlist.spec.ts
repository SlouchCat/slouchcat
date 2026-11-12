import { type Page } from '@playwright/test';
import { feedKey, PODCAST_TITLE, subscribeFeed, unsubscribe } from './audio';
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
import { INTRO_SHOW_TITLE, itemIdByTitle, useFreshProfile } from './playback';

// E-5: a watchlist per profile; the start page shows it as a row while it holds titles.

const grid = (page: Page) => page.getByRole('list', { name: 'Merkliste' });
const watchlistRow = (page: Page) => page.getByRole('region', { name: ROW_TITLES.watchlist });

test('"Merken" on the detail page fills the list and the row, per profile (E-5)', async ({
  page,
  owner,
}) => {
  const id = await itemIdByTitle(owner, H264_TITLE);
  await page.goto(`/items/${id}`);
  // The round action, not the button on the cover ("… merken", concept 17 D-9).
  await page.getByRole('button', { name: 'Merken', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Gemerkt', exact: true })).toBeVisible();
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

// Concept 20 P-2: the favourite of series and podcasts is gone; «Merken» takes its place and
// notifies new episodes (notifications.spec.ts) and feeds the podcast rule (podcast-auto-download).
test('series and podcasts go on the watchlist; there is no favourite any more (concept 20 P-2)', async ({
  page,
  owner,
}, testInfo) => {
  const merken = page.getByRole('button', { name: 'Merken', exact: true });
  const series = await itemIdByTitle(owner, INTRO_SHOW_TITLE);
  await page.goto(`/items/${series}`);
  await merken.click();
  await expect(page.getByRole('button', { name: 'Gemerkt', exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: /^Favorit/ })).toHaveCount(0);
  await page.reload();
  await expect(page.getByRole('button', { name: 'Gemerkt', exact: true })).toBeVisible();

  const podcast = await subscribeFeed(owner, feedKey(testInfo), 2);
  try {
    await page.goto(`/items/${podcast.id}`);
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
    await expect(page.getByRole('button', { name: /^Favorit/ })).toHaveCount(0);
    await merken.click();
    await expect(page.getByRole('button', { name: 'Gemerkt', exact: true })).toBeVisible();
    const titles = await watchlistTitles(owner);
    expect(titles).toHaveLength(2);
    expect(titles).toContain(INTRO_SHOW_TITLE);
    expect(titles).toContain(podcast.title || PODCAST_TITLE);
  } finally {
    await unsubscribe(owner, podcast.id);
  }
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

test('watchlist by keyboard: arrows, Enter, remove, back; no focus trap @keyboard', async ({
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

  // Escape goes back; opened directly, that is the start page.
  await resetFocus(page);
  await page.keyboard.press('Escape');
  await expect(page).toHaveURL((url) => url.pathname === '/');
});

// Concept 17 D-9: the watchlist button on every cover, shown on hover or focus, filled when marked.

const coverMark = (page: Page, title: string, on: boolean) =>
  page.getByRole('button', {
    name: on ? `${title} von der Merkliste nehmen` : `${title} merken`,
    exact: true,
  });

test('marks a title from its cover in the search; list, row and cover follow (D-9)', async ({
  page,
  owner,
}) => {
  await page.goto('/search?q=H264');
  const titles = page.getByRole('list', { name: 'Titel' });
  const mark = titles.getByRole('button', { name: `${H264_TITLE} merken`, exact: true });
  // Not marked: hidden until the pointer is on the cover.
  await expect(mark).toHaveAttribute('aria-pressed', 'false');
  await expect(mark.locator('xpath=../..')).toHaveCSS('opacity', '0');
  await titles
    .getByRole('listitem')
    .filter({ has: coverMark(page, H264_TITLE, false) })
    .getByRole('link')
    .hover();
  await expect(mark.locator('xpath=../..')).toHaveCSS('opacity', '1');
  await mark.click();

  const marked = titles.getByRole('button', {
    name: `${H264_TITLE} von der Merkliste nehmen`,
    exact: true,
  });
  await expect(marked).toHaveAttribute('aria-pressed', 'true');
  await expect.poll(() => watchlistTitles(owner)).toEqual([H264_TITLE]);
  // Marked: shown without the pointer too.
  await page.mouse.move(0, 0);
  await expect(marked.locator('xpath=../..')).toHaveCSS('opacity', '1');
  // The server knows the mark in every list (ItemRef.inWatchlist).
  await page.reload();
  await expect(marked).toHaveAttribute('aria-pressed', 'true');

  await page.goto('/');
  await expect(coverMark(page, H264_TITLE, true).first()).toBeAttached();
  await expect(
    watchlistRow(page).getByRole('link', { name: new RegExp(H264_TITLE) }),
  ).toBeVisible();

  // Off from the cover on the watchlist page: the title leaves the list.
  await page.goto('/watchlist');
  await grid(page)
    .getByRole('button', { name: `${H264_TITLE} von der Merkliste nehmen`, exact: true })
    .click();
  await expect(page.getByText('Deine Merkliste ist leer')).toBeVisible();
  expect(await watchlistTitles(owner)).toEqual([]);
});

test('the cover button by keyboard: Tab after the card, arrows pass it, focus to the neighbour @keyboard', async ({
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
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, cards.first());

  // Tab: the pen of the server administrator (concept 20 P-7), then the card's cover button, shown
  // with the focus. The arrow keys pass them by and move on from the card.
  const first = coverMark(page, EXTRAS_TITLE, true);
  const pen = grid(page).getByRole('button', { name: `${EXTRAS_TITLE}: Metadaten bearbeiten` });
  await page.keyboard.press('Tab');
  await expectFocusOn(page, pen);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, first);
  await expect(first.locator('xpath=../..')).toHaveCSS('opacity', '1');
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, cards.nth(1));
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, cards.first());

  // Tab order in the cell: card, pen, cover button, "Entfernen".
  await page.keyboard.press('Tab');
  await expectFocusOn(page, pen);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, first);
  await page.keyboard.press('Tab');
  await expectFocusOn(
    page,
    grid(page).getByRole('button', { name: `${EXTRAS_TITLE} von der Merkliste entfernen` }),
  );
  await page.keyboard.press('Shift+Tab');
  await expectFocusOn(page, first);

  // Enter on the button takes the title off; the focus goes to the neighbour's card.
  await page.keyboard.press('Enter');
  await expect(cards).toHaveCount(1);
  await expectFocusOn(page, cards.first());
  await expect(cards.first()).toHaveAccessibleName(new RegExp(H264_TITLE));
  expect(await watchlistTitles(owner)).toEqual([H264_TITLE]);
  await expectNoFocusTrap(page);
});
