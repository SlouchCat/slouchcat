import { type Page } from '@playwright/test';
import { M4B_TITLE } from './audio';
import { H264_TITLE } from './catalog';
import { createCollection, deleteCollection } from './curation';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, expectVisibleFocus, resetFocus } from './keyboard';
import { arrowTo, itemIdByTitle } from './playback';
import { closeWishes, openWishes, type WishesSetup } from './wishes';

// E-4: search from the second character over titles, people and collections, with filters. The
// e2e server has no TMDB key: titles carry no genres and no cast, so the person section and the
// genre list stay empty here (the contract test covers both, stream A); the people section is
// covered with a mocked answer at the end (concept 17 D-5).

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
      name: 'bis FSK 0',
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

  // Combined (E-4): text, media type, age rating and availability narrow together.
  await page.goto('/search?q=Sl&type=audiobook&rating=0&avail=local');
  await expect(card(page, M4B_TITLE)).toBeVisible();
  await expect(card(page, H264_TITLE)).toHaveCount(0);
  for (const [group, name] of [
    ['Medientyp', 'Hörbücher'],
    ['Altersfreigabe höchstens', 'bis FSK 0'],
    ['Verfügbarkeit', 'Auf dem Server'],
  ] as const) {
    await expect(
      page.getByRole('group', { name: group }).getByRole('button', { name }),
    ).toHaveAttribute('aria-pressed', 'true');
  }
  // Each filter holds: films are unrated, so with FSK 0 no film is left.
  await page.goto('/search?q=Sl&type=movie&rating=0&avail=local');
  await expect(page.getByText('Nichts gefunden')).toBeVisible();
});

test('search by keyboard: "/", arrows, Enter, back; no focus trap @keyboard', async ({ page }) => {
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
  // Tab from a card reaches the pen of the server administrator (concept 20 P-7), its check mark
  // (P-5), then its watchlist button (concept 17 D-9); an arrow moves on from the card.
  await page.keyboard.press('Tab');
  await expectFocusOn(
    page,
    titles(page)
      .getByRole('listitem')
      .first()
      .getByRole('button', { name: /: Metadaten bearbeiten$/ }),
  );
  await page.keyboard.press('Tab');
  await expectFocusOn(
    page,
    titles(page)
      .getByRole('listitem')
      .first()
      .getByRole('button', { name: /^(Gesehen|Gehört): / }),
  );
  await page.keyboard.press('Tab');
  await expectFocusOn(
    page,
    titles(page)
      .getByRole('listitem')
      .first()
      .getByRole('button', { name: / merken$/ }),
  );
  await page.keyboard.press('Shift+Tab');
  await page.keyboard.press('Shift+Tab');
  await page.keyboard.press('Shift+Tab');
  await expectFocusOn(page, cards.first());

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

  // Backspace outside the field goes back: to the start page the search was opened from.
  await resetFocus(page);
  await page.keyboard.press('Backspace');
  await expect(page).toHaveURL((url) => url.pathname === '/' && url.search === '');
});

test.describe('the people of a search as round portrait cards (concept 17 D-5)', () => {
  // The routes below must see the requests; a service worker would make them on its own.
  test.use({ serviceWorkers: 'block' });

  // 1x1 PNG: a portrait the browser decodes.
  const PORTRAIT = Buffer.from(
    'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==',
    'base64',
  );

  test('portrait or initial; arrows, Enter and back by keyboard @keyboard', async ({ page }) => {
    // The e2e server has no TMDB key and so no people: add two to the answer.
    await page.route('**/api/v1/search', async (route) => {
      const response = await route.fetch();
      const body = (await response.json()) as Record<string, unknown>;
      body.people = [
        {
          id: 9001,
          name: 'Anna Beispiel',
          image: { url: '/people/9001/image?v=a&p=1', width: 185, height: 278 },
        },
        { id: 9002, name: 'Bruno Muster' },
      ];
      await route.fulfill({ response, json: body });
    });
    await page.route('**/api/v1/people/9001/image*', (route) =>
      route.fulfill({ status: 200, contentType: 'image/png', body: PORTRAIT }),
    );
    await page.goto('/search?q=H2');
    const people = page.getByRole('region', { name: 'Personen' });
    const anna = people.getByRole('link', { name: 'Anna Beispiel' });
    const bruno = people.getByRole('link', { name: 'Bruno Muster' });
    await expect(anna).toHaveAttribute('href', '/people/9001');
    await expect
      .poll(() => anna.locator('img').evaluate((image: HTMLImageElement) => image.naturalWidth))
      .toBeGreaterThan(0);
    // No portrait named: the initial, no image.
    await expect(bruno.locator('img')).toHaveCount(0);
    await expect(bruno).toContainText('B');

    // ↓ from a title card reaches the people, → walks the row, ← comes back.
    await resetFocus(page);
    await anna.focus();
    await expectFocusOn(page, anna);
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, bruno);
    await page.keyboard.press('ArrowLeft');
    await expectFocusOn(page, anna);
    // ↑ leaves the people for the title cards above.
    await page.keyboard.press('ArrowUp');
    await expect(titles(page).getByRole('link').first()).toBeFocused();
    await page.keyboard.press('ArrowDown');
    await expectFocusOn(page, anna);
    // Tab walks on, no focus trap.
    await page.keyboard.press('Tab');
    await expectFocusOn(page, bruno);
    await expectNoFocusTrap(page, 120);

    // Enter opens the person (the e2e server does not know them: not found); back returns.
    await anna.focus();
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/people\/9001$/);
    await page.goBack();
    await expect(page).toHaveURL(/\/search\?q=H2$/);
    await expect(anna).toBeVisible();
    await page.unrouteAll({ behavior: 'ignoreErrors' });
  });
});

// Concept 25 stage 2, 5.2 (M-07): titles outside the library under the library's hits, from the
// TMDB mock (openWishes stores the key for this test only, e2e/wishes.ts).
test('the section «Noch nicht in der Bibliothek»: cards to the preview, the plus by keyboard, the kind filter maps', async ({
  page,
  owner,
}) => {
  let setup: WishesSetup | undefined;
  try {
    setup = await openWishes(owner);
    await page.goto('/search?q=Glas');
    const missing = page.getByRole('region', { name: 'Noch nicht in der Bibliothek' });
    await expect(missing.getByRole('link')).toHaveCount(3);
    await expect(page.getByText('Nichts gefunden')).toHaveCount(0);
    const bird = missing.getByRole('link', { name: /Glasvogel/ });
    await expect(bird).toHaveAttribute('href', '/titles/movie/9001');
    await expect(missing.getByText('2026 · Noch nicht da')).toBeVisible();
    await expect(page.locator('#content')).not.toContainText('{{');

    // Tab from the card reaches its plus; Enter opens the dialog, Escape returns.
    await bird.focus();
    await page.keyboard.press('Tab');
    const plus = missing.getByRole('button', { name: 'Glasvogel anfordern' });
    await expectFocusOn(page, plus);
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', { name: 'Film anfordern: Glasvogel' });
    await expect(dialog).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(dialog).toHaveCount(0);
    await expectFocusOn(page, plus);
    // Requested from here: the caption names the state, the plus goes.
    await page.keyboard.press('Enter');
    await dialog.getByRole('button', { name: 'Anfordern', exact: true }).press('Enter');
    await expect(dialog).toHaveCount(0);
    await expect(missing.getByText('2026 · Freigabe ausstehend')).toBeVisible();
    await expect(plus).toHaveCount(0);
    await expectVisibleFocus(page);

    // Films only: the series «Glasküste» leaves the section.
    await page.goto('/search?q=Glas&type=movie');
    await expect(missing.getByRole('link')).toHaveCount(2);
    await expect(missing.getByRole('link', { name: /Glasküste/ })).toHaveCount(0);
    await expectNoFocusTrap(page);
  } finally {
    await closeWishes(owner, setup);
  }
});
