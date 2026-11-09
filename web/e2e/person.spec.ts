import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, resetFocus, tabTo } from './keyboard';
import { CHAPTERS_TITLE, H264_TITLE, itemIdByTitle } from './playback';

// People (B-3). The e2e server has no TMDB key, so nobody has a credit: every person page is the
// not-found notice (a person without a visible credit does not exist for the profile, 404). The
// page with credits is covered by PersonPage.test.tsx, and below with a mocked answer that gives the
// person a portrait and two credits (concept 17 D-5).

test('a person without visible credits shows the not-found notice', async ({ page }) => {
  const response = page.waitForResponse((candidate) =>
    candidate.url().endsWith('/api/v1/people/1'),
  );
  await page.goto('/people/1');
  expect((await response).status()).toBe(404);
  await expect(page.getByText('Person nicht gefunden')).toBeVisible();
  await expect(page.getByText('Zu dieser Person gibt es hier keine Titel.')).toBeVisible();
});

test('Tab reaches "Zur Startseite"; Enter goes home; no focus trap @keyboard', async ({ page }) => {
  await page.goto('/people/abc');
  await expect(page.getByText('Person nicht gefunden')).toBeVisible();
  const home = page.getByRole('link', { name: 'Zur Startseite' });
  await tabTo(page, home);
  await expectFocusOn(page, home);
  await expectNoFocusTrap(page);
  await home.focus();
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/$/);
});

for (const key of ['Escape', 'Backspace'] as const) {
  test(`${key} goes back`, async ({ page }) => {
    await page.goto('/');
    await expect(page.getByRole('heading', { level: 2, name: 'Neu hinzugefügt' })).toBeVisible();
    await page.goto('/people/1');
    await expect(page.getByText('Person nicht gefunden')).toBeVisible();
    await page.keyboard.press(key);
    await expect(page).toHaveURL(/\/$/);
  });
}

test.describe('a person with a portrait', () => {
  // The routes below must see the requests; a service worker would make them on its own.
  test.use({ serviceWorkers: 'block' });

  // 1x1 PNG: a portrait the browser decodes.
  const PORTRAIT = Buffer.from(
    'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==',
    'base64',
  );

  test('portrait left of the name; arrows, Tab, Enter and back by keyboard @keyboard', async ({
    page,
    owner,
  }) => {
    const credit = async (title: string) => ({
      item: {
        id: await itemIdByTitle(owner, title),
        kind: 'movie',
        title,
        inWatchlist: false,
        year: 2024,
        images: {},
      },
      role: 'cast',
    });
    const person = {
      id: 9001,
      name: 'Anna Beispiel',
      image: { url: '/people/9001/image?v=a&p=1', width: 185, height: 278 },
      credits: [await credit(H264_TITLE), await credit(CHAPTERS_TITLE)],
    };
    await page.route('**/api/v1/people/9001', (route) => route.fulfill({ json: person }));
    await page.route('**/api/v1/people/9001/image*', (route) =>
      route.fulfill({ status: 200, contentType: 'image/png', body: PORTRAIT }),
    );
    await page.goto('/people/9001');
    const heading = page.getByRole('heading', { level: 1, name: 'Anna Beispiel' });
    await expect(heading).toBeVisible();
    const portrait = page.locator('main img[src*="/people/9001/image"]');
    await expect
      .poll(() => portrait.evaluate((image: HTMLImageElement) => image.naturalWidth))
      .toBeGreaterThan(0);
    // The portrait stands left of the name.
    const [imageBox, nameBox] = await Promise.all([portrait.boundingBox(), heading.boundingBox()]);
    expect((imageBox?.x ?? 0) + (imageBox?.width ?? 0)).toBeLessThanOrEqual(nameBox?.x ?? 0);

    const credits = page.getByRole('region', { name: 'Filme und Serien' }).getByRole('link');
    await expect(credits).toHaveCount(2);
    await resetFocus(page);
    await page.keyboard.press('ArrowDown');
    await expectFocusOn(page, credits.first());
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, credits.nth(1));
    await page.keyboard.press('ArrowLeft');
    await expectFocusOn(page, credits.first());
    // Tab from a card reaches its watchlist button, Shift+Tab comes back.
    await page.keyboard.press('Tab');
    await expectFocusOn(page, page.getByRole('button', { name: `${H264_TITLE} merken` }));
    await page.keyboard.press('Shift+Tab');
    await expectFocusOn(page, credits.first());
    // Up leaves the row for the back arrow.
    await page.keyboard.press('ArrowUp');
    await expectFocusOn(page, page.getByRole('button', { name: 'Zurück', exact: true }));
    await page.keyboard.press('ArrowDown');
    await expectFocusOn(page, credits.first());
    await expectNoFocusTrap(page);

    // Enter opens the title; the browser's back returns to the person, portrait and all.
    await credits.first().focus();
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/items\//);
    await page.goBack();
    await expect(page).toHaveURL(/\/people\/9001$/);
    await expect(heading).toBeVisible();
    await page.unrouteAll({ behavior: 'ignoreErrors' });
  });
});
