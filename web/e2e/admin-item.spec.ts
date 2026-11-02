import type { Page } from '@playwright/test';
import { CSRF, H264_TITLE, idOf, itemPath } from './catalog';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, tabTo } from './keyboard';

// Administration view of a title (B-6, B-8, B-9): state, files, the match dialog (the e2e server
// has no TMDB key: the search answers with the provider error), manual trailers.

let detail = '';
let admin = '';

const trailers = (page: Page) => page.getByRole('region', { name: 'Trailer' });

test.beforeEach(async ({ page }) => {
  if (!admin) {
    detail = await itemPath(page, H264_TITLE);
    admin = `/admin/items/${idOf(detail)}`;
  }
  await page.goto(admin);
  await expect(page.getByRole('heading', { level: 1, name: H264_TITLE })).toBeVisible();
});

test.afterEach(async ({ owner }) => {
  // A trailer left by a failed run would make "Löschen" ambiguous here and add a trailer button
  // to the detail page of the other specs.
  if (!admin) return;
  const item = (await (await owner.get(`/api/v1${admin}`)).json()) as {
    trailers: { id: number }[];
  };
  for (const trailer of item.trailers) {
    await owner.delete(`/api/v1${admin}/trailers/${trailer.id}`, { headers: CSRF });
  }
});

test('shows kind, state, both version files and the trailer state', async ({ page }) => {
  const facts = page.getByRole('region', { name: 'Metadaten' });
  await expect(facts.getByRole('term').filter({ hasText: /^Art$/ })).toBeVisible();
  await expect(facts.getByRole('definition').filter({ hasText: /^Film$/ })).toBeVisible();
  const files = page.getByRole('region', { name: 'Dateien' });
  await expect(files.getByRole('listitem')).toHaveCount(2);
  await expect(files.getByText(/Slouchcat Test H264 \(2024\)\.mp4$/)).toBeVisible();
  await expect(files.getByText(/Slouchcat Test H264 \(2024\) - 720p\.mp4$/)).toBeVisible();
  await expect(trailers(page)).toBeVisible();
  await expect(page.getByRole('link', { name: 'Zur Detailseite' })).toHaveAttribute('href', detail);
});

test('the detail page links here with "Verwalten"', async ({ page }) => {
  await page.goto(detail);
  const manage = page.getByRole('link', { name: 'Verwalten' });
  await manage.focus();
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(new RegExp(`${admin}$`));
});

test('match dialog: the search without a key shows the provider error; Escape closes', async ({
  page,
}) => {
  const match = page.getByRole('button', { name: 'Zuordnen' });
  await tabTo(page, match);
  await page.keyboard.press('Enter');
  const dialog = page.getByRole('dialog', { name: 'Titel zuordnen' });
  await expect(dialog).toBeVisible();
  const query = dialog.getByLabel('Suchbegriff');
  await expectFocusOn(page, query);
  await expect(query).toHaveValue(H264_TITLE);
  await page.keyboard.press('Enter');
  await expect(
    dialog.getByText(/TMDB hat den Schlüssel abgelehnt|Dieser Anbieter ist nicht eingerichtet/),
  ).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(dialog).toHaveCount(0);
  await expectFocusOn(page, match);
});

test('add a manual trailer by keyboard, then delete it after confirmation', async ({ page }) => {
  const add = trailers(page).getByRole('button', { name: 'Hinzufügen' });
  await add.focus();
  await page.keyboard.press('Enter');
  const dialog = page.getByRole('dialog', { name: 'Trailer hinzufügen' });
  const source = dialog.getByLabel('YouTube-Link oder Video-ID');
  await expectFocusOn(page, source);
  await page.keyboard.type('https://www.youtube.com/watch?v=dQw4w9WgXcQ');
  await page.keyboard.press('Enter');
  await expect(dialog).toHaveCount(0);
  await expect(trailers(page).getByText('dQw4w9WgXcQ')).toBeVisible();
  await expect(trailers(page).getByText('Manuell')).toBeVisible();

  const remove = trailers(page).getByRole('button', { name: 'Löschen' });
  await remove.focus();
  await page.keyboard.press('Enter');
  const confirm = page.getByRole('alertdialog', { name: 'Trailer löschen?' });
  await expectFocusOn(page, confirm.getByRole('button', { name: 'Löschen' }));
  await page.keyboard.press('Enter');
  await expect(trailers(page).getByText('dQw4w9WgXcQ')).toHaveCount(0);
});

test('an invalid YouTube link is reported at the field', async ({ page }) => {
  await trailers(page).getByRole('button', { name: 'Hinzufügen' }).click();
  const dialog = page.getByRole('dialog', { name: 'Trailer hinzufügen' });
  await dialog.getByLabel('YouTube-Link oder Video-ID').fill('https://example.com/video');
  await dialog.getByRole('button', { name: 'Hinzufügen' }).click();
  await expect(dialog.getByLabel('YouTube-Link oder Video-ID')).toHaveAttribute(
    'aria-invalid',
    'true',
  );
  await page.keyboard.press('Escape');
  await expect(dialog).toHaveCount(0);
});

test('Tab: header, actions, rating, files, trailers; no focus trap', async ({ page }) => {
  // The admin sidebar's Tab stop is its current page: a title's page belongs to "Pflege".
  await tabTo(
    page,
    page
      .getByRole('navigation', { name: 'Verwaltung' })
      .getByRole('link', { name: /^Pflege(,|$)/ }),
  );
  await tabTo(page, page.getByRole('button', { name: 'Zuordnen' }), 12);
  await tabTo(page, page.getByRole('button', { name: 'Aktualisieren' }), 2);
  await tabTo(page, page.getByRole('link', { name: 'Zur Detailseite' }), 2);
  await tabTo(page, trailers(page).getByRole('button', { name: 'Hinzufügen' }), 20);
  await expectNoFocusTrap(page);
});

test.describe('image choice', () => {
  // Chromium sends the page's GET requests through the service worker, where page.route does not
  // see them; the candidates and the reread title are answered here, so the worker stays out.
  test.use({ serviceWorkers: 'block' });

  test('image choice: the grid by arrow keys, Enter chooses, Escape closes, focus returns (concept 10 7.3)', async ({
    page,
    owner,
  }) => {
    // The e2e server has no TMDB key and the test film no image file next to it: the candidates and
    // the choice are answered here; the server side is covered by the Go tests of package metadata.
    const stored = (await (await owner.get(`/api/v1${admin}`)).json()) as Record<string, unknown>;
    const candidates = [
      {
        provider: 'local',
        lang: '',
        width: 600,
        height: 900,
        score: 0,
        voteCount: 0,
        source: 'file:A/poster.jpg?1-1',
        previewUrl: '',
        chosen: false,
      },
      {
        provider: 'tmdb',
        lang: 'de',
        width: 1000,
        height: 1500,
        score: 5.5,
        voteCount: 10,
        source: '/de.jpg',
        previewUrl: '',
        chosen: false,
      },
      {
        provider: 'tmdb',
        lang: 'en',
        width: 2000,
        height: 3000,
        score: 6,
        voteCount: 20,
        source: '/en.jpg',
        previewUrl: '',
        chosen: false,
      },
    ];
    let chosen: unknown = null;
    await page.route(
      (url) => url.pathname === `/api/v1${admin}/image-candidates`,
      (route) =>
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ candidates, unavailable: [] }),
        }),
    );
    await page.route(
      (url) => url.pathname === `/api/v1${admin}/images/poster`,
      async (route) => {
        chosen = route.request().postDataJSON();
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            ...stored,
            images: [
              {
                kind: 'poster',
                provider: 'tmdb',
                lang: 'de',
                source: '/de.jpg',
                chosen: true,
                locked: true,
              },
            ],
          }),
        });
      },
    );

    // After the choice the page reads the title again: that answer carries the choice too.
    await page.route(
      (url) => url.pathname === `/api/v1${admin}`,
      async (route) => {
        if (chosen === null || route.request().method() !== 'GET') return route.continue();
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            ...stored,
            images: [
              {
                kind: 'poster',
                provider: 'tmdb',
                lang: 'de',
                source: '/de.jpg',
                chosen: true,
                locked: true,
              },
            ],
          }),
        });
      },
    );

    const images = page.getByRole('region', { name: 'Bilder' });
    const pick = images
      .getByRole('listitem')
      .filter({ hasText: 'Poster' })
      .getByRole('button', { name: 'Andere wählen' });
    await tabTo(page, pick, 30);
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', { name: 'Poster wählen' });
    const grid = dialog.getByRole('listbox', { name: 'Kandidaten für Poster' });
    const tiles = grid.getByRole('option');
    await expect(tiles).toHaveCount(3);
    await expectFocusOn(page, tiles.nth(0));
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, tiles.nth(1));

    // Escape closes without a choice; the focus is back on the button.
    await page.keyboard.press('Escape');
    await expect(dialog).toHaveCount(0);
    await expectFocusOn(page, pick);
    expect(chosen).toBeNull();

    await page.keyboard.press('Enter');
    await expectFocusOn(page, tiles.nth(0));
    await page.keyboard.press('ArrowRight');
    await page.keyboard.press('Enter');
    await expect(dialog).toHaveCount(0);
    expect(chosen).toEqual({ provider: 'tmdb', source: '/de.jpg' });
    await expect(
      page.getByText('Poster gewählt und gesperrt; das Bild wird geholt.'),
    ).toBeVisible();
    const poster = images.getByRole('listitem').filter({ hasText: 'Poster' });
    await expect(poster.getByText('von Hand gewählt')).toBeVisible();
    await expect(poster.getByRole('button', { name: 'Sperre aufheben' })).toBeVisible();
    await expectNoFocusTrap(page);
  });
});
