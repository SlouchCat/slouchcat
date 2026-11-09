import { expect, test, type Page } from '@playwright/test';
import { H264_TITLE, itemPath } from './catalog';
import { expectFocusOn, expectNoFocusTrap, resetFocus, tabTo } from './keyboard';

let detail = '';

test.beforeEach(async ({ page }) => {
  detail ||= await itemPath(page, H264_TITLE);
  await page.goto('/');
  await page.goto(detail);
  await expect(page.getByRole('heading', { level: 1, name: H264_TITLE })).toBeVisible();
  await resetFocus(page);
});

// The big button of a film (concept 17 E-10): without progress "Abspielen" with the runtime; other
// specs may have left progress behind ("Fortsetzen bei …", "Nochmals ansehen …").
const play = (page: Page) =>
  page.getByRole('link', { name: /^(Abspielen|Fortsetzen|Nochmals ansehen)( |$)/ });
const back = (page: Page) => page.getByRole('button', { name: 'Zurück' });
/** «Empfehlen» or «Empfohlen ✓» (concept 18 part B): another spec may have taken the pick back. */
const star = (page: Page) => page.getByRole('button', { name: /^(Empfehlen|Empfohlen ✓)$/ });

test('arrows: start on play, up to the back arrow, down to the round actions; facts are shown', async ({
  page,
}) => {
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, play(page));
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, back(page));
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, play(page));
  // Under the big button the round actions in their order (E-11): → walks them to "Merken".
  await page.keyboard.press('ArrowDown');
  const merken = page.getByRole('button', { name: /^(Merken|Gemerkt)$/ });
  for (let presses = 0; presses < 3; presses += 1) {
    if (await merken.evaluate((element) => element === document.activeElement)) break;
    await page.keyboard.press('ArrowRight');
  }
  await expectFocusOn(page, merken);
  await page.keyboard.press('ArrowRight');
  // The star of the user picks (concept 18 part B); the global setup recommended this film.
  await expectFocusOn(page, star(page));
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, play(page));
  const file = page.getByRole('region', { name: 'Datei' });
  await expect(file.getByText('MP4')).toBeVisible();
});

test('Tab: back arrow, pen, watchlist on the cover, play, round actions; no focus trap @keyboard', async ({
  page,
}) => {
  // The side navigation's last stop (concept 08) comes right before the page: the back arrow, for
  // the server administrator the pen on the poster (concept 15 7.4), the watchlist button on the
  // cover (concept 17 D-9), then play and the round actions.
  await tabTo(page, page.getByRole('button', { name: 'Seitenleiste ausblenden' }));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, back(page));
  await page.keyboard.press('Tab');
  await expectFocusOn(
    page,
    page.getByRole('button', { name: 'Metadaten bearbeiten', exact: true }),
  );
  await page.keyboard.press('Tab');
  await expectFocusOn(
    page,
    page.getByRole('button', {
      name: new RegExp(`^${H264_TITLE} (merken|von der Merkliste nehmen)$`),
    }),
  );
  await page.keyboard.press('Tab');
  await expectFocusOn(page, play(page));
  await tabTo(page, page.getByRole('button', { name: /^(Merken|Gemerkt)$/ }), 3);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, star(page));
  await expectNoFocusTrap(page);
});

for (const key of ['Escape', 'Backspace'] as const) {
  test(`${key} returns to the start page`, async ({ page }) => {
    await page.keyboard.press(key);
    await expect(page).toHaveURL(/\/$/);
  });
}

test('Enter on "Zurück" returns to where the viewer came from; browser history works', async ({
  page,
}) => {
  // Arrive the way a viewer does: from the start page, card focused, Enter.
  await page.goto('/');
  await page
    .getByRole('region', { name: 'Neu hinzugefügt' })
    .locator(`a[href="${detail}"]`)
    .focus();
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(new RegExp(`${detail}$`));
  await expect(page.getByRole('heading', { level: 1, name: H264_TITLE })).toBeVisible();

  await page.getByRole('button', { name: 'Zurück' }).focus();
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/$/);
  await page.goForward();
  await expect(page).toHaveURL(new RegExp(`${detail}$`));
  await page.goBack();
  await expect(page).toHaveURL(/\/$/);
});

test('an unknown id shows the not-found message', async ({ page }) => {
  await page.goto('/items/01JB8Z6Q4M7N9P2R3S5T7V9W1X');
  await expect(page.getByText('Titel nicht gefunden')).toBeVisible();
  await tabTo(page, page.getByRole('link', { name: 'Zur Startseite' }));
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/$/);
});

test.describe('the cast row with portraits (concept 17 D-5)', () => {
  // The routes below must see the requests; a service worker would make them on its own.
  test.use({ serviceWorkers: 'block' });

  // 1x1 PNG: a portrait the browser decodes.
  const PORTRAIT = Buffer.from(
    'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==',
    'base64',
  );

  test('a portrait shows on the card, a missing one falls back to the initial', async ({
    page,
  }) => {
    // The e2e server has no TMDB key and so no cast: add three members to the detail answer.
    await page.route(/\/api\/v1\/items\/[^/?]+(\?.*)?$/, async (route) => {
      const response = await route.fetch();
      const body = (await response.json()) as Record<string, unknown>;
      body.cast = [
        {
          personId: 9001,
          name: 'Anna Beispiel',
          role: 'cast',
          character: 'Ellen',
          image: { url: '/people/9001/image?v=a&p=1', width: 185, height: 278 },
        },
        { personId: 9002, name: 'Bruno Muster', role: 'director' },
        {
          personId: 9003,
          name: 'Clara Probe',
          role: 'cast',
          image: { url: '/people/9003/image?v=b&p=1', width: 185, height: 278 },
        },
      ];
      await route.fulfill({ response, json: body });
    });
    await page.route('**/api/v1/people/9001/image*', (route) =>
      route.fulfill({ status: 200, contentType: 'image/png', body: PORTRAIT }),
    );
    // The file is gone although the server named it: the initial takes over.
    await page.route('**/api/v1/people/9003/image*', (route) =>
      route.fulfill({ status: 404, contentType: 'text/plain', body: 'gone' }),
    );
    await page.goto(detail);
    const cast = page.getByRole('region', { name: 'Besetzung' });
    const anna = cast.getByRole('link', { name: /Anna Beispiel/ });
    await expect(anna).toBeVisible();
    await expect(anna.locator('img')).toHaveCount(1);
    await expect
      .poll(() => anna.locator('img').evaluate((image: HTMLImageElement) => image.naturalWidth))
      .toBeGreaterThan(0);
    // No portrait named: the initial, no image.
    const bruno = cast.getByRole('link', { name: /Bruno Muster/ });
    await expect(bruno.locator('img')).toHaveCount(0);
    await expect(bruno).toContainText('B');
    // Named but not loadable: the image goes, the initial stays.
    const clara = cast.getByRole('link', { name: /Clara Probe/ });
    await expect(clara.locator('img')).toHaveCount(0);
    await expect(clara).toContainText('C');
    // The row is reachable by arrows and the focus ring shows.
    await anna.focus();
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, bruno);
    await page.unrouteAll({ behavior: 'ignoreErrors' });
  });
});
