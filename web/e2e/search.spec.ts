import { type Page } from '@playwright/test';
import { M4B_TITLE } from './audio';
import { H264_TITLE } from './catalog';
import { createCollection, deleteCollection } from './curation';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, expectVisibleFocus, resetFocus } from './keyboard';
import { arrowTo, itemIdByTitle } from './playback';

// E-4: search from the second character over titles, people and collections, with filters. The
// e2e server has no TMDB key: titles carry no genres and no cast, so the person section and the
// genre list stay empty here (the contract test covers both, stream A).

const field = (page: Page) => page.getByRole('searchbox', { name: 'Suchbegriff' });
const titles = (page: Page) => page.getByRole('region', { name: 'Titel' });
const card = (page: Page, title: string) =>
  titles(page)
    .getByRole('link')
    .filter({ has: page.getByText(title, { exact: true }) });

test('two characters find titles; a collection by its name (E-4)', async ({ page, owner }) => {
  await page.goto('/search');
  await expectFocusOn(page, field(page));
  await field(page).pressSequentially('H');
  await expect(page.getByText('Gib mindestens zwei Zeichen ein.')).toBeVisible();
  await field(page).pressSequentially('2');
  await expect(card(page, H264_TITLE)).toBeVisible();
  await expect(page).toHaveURL(/\/search\?q=H2$/);
  await expect(page.getByText(/^\d+ Titel gefunden$/)).toBeVisible();

  const { id: collection } = await createCollection(owner, 'E2E Zugvogel', [
    await itemIdByTitle(owner, H264_TITLE),
  ]);
  try {
    await page.goto('/search?q=Zugv');
    const collections = page.getByRole('region', { name: 'Sammlungen' });
    const entry = collections.getByRole('link', { name: /E2E Zugvogel/ });
    await expect(entry).toHaveAttribute('href', `/collections/${collection}`);
    await expect(entry.getByText('1 Titel')).toBeVisible();
    // The collection matches; its member does not carry the word.
    await expect(titles(page)).toHaveCount(0);
  } finally {
    await deleteCollection(owner, collection);
  }
});

test('every filter narrows the result and lives in the URL', async ({ page }) => {
  // Media type, by click: the URL follows.
  await page.goto('/search?q=Sl');
  await expect(card(page, H264_TITLE)).toBeVisible();
  await page
    .getByRole('group', { name: 'Medientyp' })
    .getByRole('button', { name: 'Hörbücher' })
    .click();
  await expect(page).toHaveURL(/type=audiobook/);
  await expect(card(page, M4B_TITLE)).toBeVisible();
  await expect(card(page, H264_TITLE)).toHaveCount(0);
  await page.getByRole('button', { name: 'Filter zurücksetzen' }).click();
  await expect(page).toHaveURL(/\/search\?q=Sl$/);

  // Year: the test media are from 2024.
  await page.goto('/search?q=H2&from=2025');
  await expect(page.getByText('Nichts gefunden')).toBeVisible();
  await page.goto('/search?q=H2&to=2024');
  await expect(card(page, H264_TITLE)).toBeVisible();
  await expect(page.getByRole('textbox', { name: 'Jahr bis' })).toHaveValue('2024');

  // Age rating: the films are unrated (never pass), the audiobooks are FSK 0 (AN-2).
  await page.goto('/search?q=Sl&rating=0');
  await expect(card(page, M4B_TITLE)).toBeVisible();
  await expect(card(page, H264_TITLE)).toHaveCount(0);
  await expect(
    page.getByRole('group', { name: 'Altersfreigabe höchstens' }).getByRole('button', {
      name: 'FSK 0',
    }),
  ).toHaveAttribute('aria-pressed', 'true');

  // Availability: the file is on the server; nothing is downloaded on this device.
  await page.goto('/search?q=H2&avail=local');
  await expect(card(page, H264_TITLE)).toBeVisible();
  await page.goto('/search?q=H2&avail=download');
  await expect(page.getByText('Nichts gefunden')).toBeVisible();

  // Genre: without TMDB no title has one; a genre from a link stays choosable and finds nothing.
  await page.goto('/search?q=H2&genre=Drama');
  await expect(
    page.getByRole('group', { name: 'Genre' }).getByRole('button', { name: 'Drama' }),
  ).toHaveAttribute('aria-pressed', 'true');
  await expect(page.getByText('Nichts gefunden')).toBeVisible();
  // A genre without text browses by the filter (the genre row of the start page links here).
  await page.goto('/search?genre=Drama');
  await expect(page.getByText('Nichts gefunden')).toBeVisible();
});

test('search by keyboard: "/", arrows, Enter, back; no focus trap', async ({ page }) => {
  // "/" opens the search from the start page.
  await page.goto('/');
  await expect(page.getByRole('region', { name: 'Neu hinzugefügt' })).toBeVisible();
  await resetFocus(page);
  await page.keyboard.press('/');
  await expect(page).toHaveURL(/\/search$/);
  await expectFocusOn(page, field(page));

  await page.keyboard.type('H2');
  await expect(card(page, H264_TITLE)).toBeVisible();
  // Left and right stay in the text.
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, field(page));
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, field(page));

  // Down: first into the filters, then into the results.
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(
    page,
    page.getByRole('group', { name: 'Genre' }).getByRole('button', { name: 'Alle Genres' }),
  );
  const cards = titles(page).getByRole('link');
  await arrowTo(page, cards.first(), 'ArrowDown', 12);
  await expectVisibleFocus(page);
  if ((await cards.count()) > 1) {
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, cards.nth(1));
    await page.keyboard.press('ArrowLeft');
    await expectFocusOn(page, cards.first());
  }

  // Enter opens the title, the browser's back returns to the same search.
  const href = await cards.first().getAttribute('href');
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(new RegExp(`${href ?? '/items/'}$`));
  await page.goBack();
  await expect(page).toHaveURL(/\/search\?q=H2$/);
  await expect(field(page)).toHaveValue('H2');
  await expect(card(page, H264_TITLE)).toBeVisible();

  // "/" focuses the field again; Tab reaches the end of the page.
  await resetFocus(page);
  await page.keyboard.press('/');
  await expectFocusOn(page, field(page));
  await expect(field(page)).toHaveValue('H2');
  await expectNoFocusTrap(page, 120);

  // Backspace outside the field goes back.
  await resetFocus(page);
  await page.keyboard.press('Backspace');
  await expect(page).not.toHaveURL(/\/search/);
});
