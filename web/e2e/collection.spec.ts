import type { Locator } from '@playwright/test';
import { createCollection, deleteCollection } from './curation';
import { ROW_TITLES } from './discover';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, expectVisibleFocus, resetFocus } from './keyboard';
import { CHAPTERS_TITLE, H264_TITLE, itemIdByTitle } from './playback';

// E-6: a manual collection of the main user (admin API) as a page of its own and as a card in the
// start page's row "Sammlungen" and on its members' detail pages.

const NAME = 'E2E Sammlung Abendprogramm';

test('a manual collection: page, start page row and detail page (E-6)', async ({ page, owner }) => {
  const h264 = await itemIdByTitle(owner, H264_TITLE);
  const chapters = await itemIdByTitle(owner, CHAPTERS_TITLE);
  const { id } = await createCollection(owner, NAME, [h264, chapters]);
  try {
    await page.goto(`/collections/${id}`);
    await expect(page.getByRole('heading', { level: 1, name: NAME })).toBeVisible();
    await expect(page.getByText('2 Titel')).toBeVisible();
    // The head of a detail page (concept 17 E-16): the round back arrow.
    await expect(page.getByRole('button', { name: 'Zurück', exact: true })).toBeVisible();
    const members = page.getByRole('list', { name: 'Titel' }).getByRole('link');
    // The admin's order.
    await expect(members).toHaveCount(2);
    await expect(members.first()).toContainText(H264_TITLE);
    await expect(members.nth(1)).toContainText(CHAPTERS_TITLE);
    await expect(page).toHaveTitle(`${NAME} · Slouchcat`);

    await page.goto('/');
    const row = page.getByRole('region', { name: ROW_TITLES.collections });
    await expect(row.getByRole('link', { name: new RegExp(NAME) })).toHaveAttribute(
      'href',
      `/collections/${id}`,
    );

    // On a member's detail page the collection is a row right after the head (concept 17 E-8):
    // its members in order, this title ringed, "Alle zeigen" to the collection page.
    await page.goto(`/items/${h264}`);
    const section = page.getByRole('region', { name: NAME });
    await expect(section.getByText('2 Titel')).toBeVisible();
    const shelf = section.getByRole('list', { name: NAME }).getByRole('link');
    await expect(shelf).toHaveCount(2);
    await expect(shelf.first()).toContainText(H264_TITLE);
    await expect(shelf.first()).toHaveAttribute('aria-current', 'true');
    await expect(shelf.nth(1)).not.toHaveAttribute('aria-current');
    await section.getByRole('link', { name: `Alle zeigen: ${NAME}` }).click();
    await expect(page).toHaveURL(new RegExp(`/collections/${id}$`));
  } finally {
    await deleteCollection(owner, id);
  }
  await page.goto(`/collections/${id}`);
  await expect(page.getByText('Sammlung nicht gefunden')).toBeVisible();
});

test('collection by keyboard: arrows, Enter, back; no focus trap @keyboard', async ({
  page,
  owner,
}) => {
  const { id } = await createCollection(owner, NAME, [
    await itemIdByTitle(owner, H264_TITLE),
    await itemIdByTitle(owner, CHAPTERS_TITLE),
  ]);
  try {
    await page.goto(`/collections/${id}`);
    const members = page.getByRole('list', { name: 'Titel' }).getByRole('link');
    await expect(members).toHaveCount(2);
    await resetFocus(page);
    await page.keyboard.press('ArrowDown');
    await expectFocusOn(page, members.first());
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, members.nth(1));
    await page.keyboard.press('ArrowLeft');
    await expectFocusOn(page, members.first());
    // Up from the first column leaves the grid for the back arrow over the head.
    await page.keyboard.press('ArrowUp');
    await expectFocusOn(page, page.getByRole('button', { name: 'Zurück', exact: true }));
    await page.keyboard.press('ArrowDown');
    await expectVisibleFocus(page);

    await members.first().focus();
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/items\//);
    await page.goBack();
    await expect(page.getByRole('heading', { level: 1, name: NAME })).toBeVisible();
    await expectNoFocusTrap(page);

    // Backspace goes back; opened directly, that is the start page.
    await resetFocus(page);
    await page.keyboard.press('Backspace');
    await expect(page).toHaveURL((url) => url.pathname === '/');
  } finally {
    await deleteCollection(owner, id);
  }
});

test.describe('the TMDB poster of a collection (concept 17 D-5)', () => {
  // The routes below must see the requests; a service worker would make them on its own.
  test.use({ serviceWorkers: 'block' });

  // 1x1 PNG: a poster the browser decodes.
  const POSTER = Buffer.from(
    'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==',
    'base64',
  );

  test('the poster shows on the card and the head, the neutral tile without the file; keyboard @keyboard', async ({
    page,
    owner,
  }) => {
    const { id } = await createCollection(owner, NAME, [await itemIdByTitle(owner, H264_TITLE)]);
    try {
      // The e2e server has no TMDB key, so a manual collection has no poster: name one in the
      // answers of the start page, the search and the collection page.
      const image = { url: `/collections/${id}/image?v=a&p=1`, width: 400, height: 600 };
      await page.route('**/api/v1/home*', async (route) => {
        const response = await route.fetch();
        const body = (await response.json()) as {
          rows: { key: string; collections?: { id: string; image?: unknown }[] }[];
        };
        for (const row of body.rows) {
          for (const collection of row.collections ?? []) collection.image = image;
        }
        await route.fulfill({ response, json: body });
      });
      await page.route('**/api/v1/search', async (route) => {
        const response = await route.fetch();
        const body = (await response.json()) as { collections: { image?: unknown }[] };
        for (const collection of body.collections) collection.image = image;
        await route.fulfill({ response, json: body });
      });
      await page.route(new RegExp(`/api/v1/collections/${id}(\\?.*)?$`), async (route) => {
        const response = await route.fetch();
        const body = (await response.json()) as Record<string, unknown>;
        await route.fulfill({ response, json: { ...body, image } });
      });
      let file: 'png' | 'gone' = 'png';
      await page.route(`**/api/v1/collections/${id}/image*`, (route) =>
        file === 'png'
          ? route.fulfill({ status: 200, contentType: 'image/png', body: POSTER })
          : route.fulfill({ status: 404, contentType: 'text/plain', body: 'gone' }),
      );
      const loaded = (locator: Locator) =>
        locator.evaluate((element: HTMLImageElement) => element.naturalWidth);

      await page.goto('/');
      const row = page.getByRole('region', { name: ROW_TITLES.collections });
      const homeCard = row.getByRole('link', { name: new RegExp(NAME) });
      await expect(homeCard).toBeVisible();
      // The card is below the fold and its image lazy: Firefox loads it only once it is near.
      await homeCard.scrollIntoViewIfNeeded();
      await expect.poll(() => loaded(homeCard.locator('img'))).toBeGreaterThan(0);

      await page.goto('/search?q=Abendprogramm');
      const found = page.getByRole('region', { name: 'Sammlungen' });
      const card = found.getByRole('link', { name: new RegExp(NAME) });
      await expect(card).toBeVisible();
      await expect.poll(() => loaded(card.locator('img'))).toBeGreaterThan(0);

      // By keyboard: the card, Enter, the head shows the poster, back returns to the search.
      await resetFocus(page);
      await card.focus();
      await expectFocusOn(page, card);
      await page.keyboard.press('Enter');
      await expect(page).toHaveURL(new RegExp(`/collections/${id}$`));
      const cover = page.locator(`main img[src*="/collections/${id}/image"]`);
      await expect.poll(() => loaded(cover)).toBeGreaterThan(0);
      await page.goBack();
      await expect(card).toBeVisible();

      // The file is gone although the answers name it: neutral tile and no broken image.
      file = 'gone';
      await page.goto('/search?q=Abendprogramm');
      await expect(card).toBeVisible();
      await expect(card.locator('img')).toHaveCount(0);
      await page.goto(`/collections/${id}`);
      await expect(page.getByRole('heading', { level: 1, name: NAME })).toBeVisible();
      await expect(cover).toHaveCount(0);
      await page.unrouteAll({ behavior: 'ignoreErrors' });
    } finally {
      await deleteCollection(owner, id);
    }
  });
});
