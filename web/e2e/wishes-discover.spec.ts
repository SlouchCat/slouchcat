import type { APIRequestContext, Page } from '@playwright/test';
import { CSRF } from './catalog';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';
import { useFreshProfile } from './playback';
import { closeWishes, openWishes, type WishesSetup } from './wishes';

// Concept 25 stage 5, 5.4 (M-31): «Entdecken» of the wishes area by keyboard (rule 8). The rows
// come from the discover routes of the Seerr mock (seerr-server.mjs) with the titles of the TMDB
// mock (tmdb-server.mjs) and a few TMDB does not know (DISCOVER_UNKNOWN: the placeholder).
// The library's titles have no TMDB id in the e2e run, so no tile carries «In der Bibliothek»
// here; DiscoverPage.test.tsx covers that mark.

const nav = (page: Page) => page.getByRole('navigation', { name: 'Hauptnavigation' });
const area = (page: Page) => page.getByRole('navigation', { name: 'Bereiche unter Wünsche' });
const row = (page: Page, name: string) => page.getByRole('region', { name, exact: true });
const content = (page: Page) => page.locator('#content');

const ROWS = [
  'Im Trend',
  'Beliebte Filme',
  'Film-Genres',
  'Demnächst im Kino',
  'Beliebte Serien',
  'Serien-Genres',
  'Demnächst als Serie',
];

/** Turns the request right of the profile on or off by hand (without an age limit it starts on, ADR 0045). */
async function allowRequests(owner: APIRequestContext, profileId: number, canRequest: boolean) {
  const response = await owner.patch(`/api/v1/profiles/${profileId}`, {
    headers: CSRF,
    data: { canRequest },
  });
  expect(response.status(), await response.text()).toBe(200);
}

test('«Entdecken» by keyboard: the entry leads here, seven rows, arrows, plus, Enter, back @keyboard', async ({
  page,
  owner,
}) => {
  let setup: WishesSetup | undefined;
  try {
    setup = await openWishes(owner);
    await page.goto('/');
    // The entry «Wünsche» leads to «Entdecken» while the pages show.
    const entry = nav(page).getByRole('link', { name: /^Wünsche/ });
    await expect(entry).toHaveAttribute('href', '/discover');
    await resetFocus(page);
    await tabTo(page, entry, 40);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/discover$/);
    await expect(page.getByRole('heading', { level: 1, name: 'Entdecken' })).toBeVisible();
    await expect(area(page).getByRole('link', { name: 'Entdecken' })).toHaveAttribute(
      'aria-current',
      'page',
    );
    for (const name of ROWS) await expect(row(page, name)).toBeVisible();
    await expect(content(page)).not.toContainText('{{');

    // The tiles: a missing film with its plus, a title another Seerr user asked for with the
    // bell, one TMDB does not know with Seerr's text and the placeholder.
    const trending = row(page, 'Im Trend');
    const glass = trending.getByRole('link', { name: /Glasvogel/ });
    await expect(glass).toHaveAttribute('href', '/titles/movie/9001');
    await expect(trending.getByRole('button', { name: 'Glasvogel anfordern' })).toBeAttached();
    await expect(
      trending.getByRole('button', { name: 'Rotes Ufer: Benachrichtige mich' }),
    ).toBeAttached();
    await expect(trending.getByText(/Bereits angefordert/)).toBeVisible();
    const unknown = trending.getByRole('link', { name: /Quiet Harbor/ });
    await expect(unknown).toBeVisible();
    await expect(unknown.locator('img')).toHaveCount(0);
    // Row titles lead to the grids, «Demnächst» newest first; genre tiles to the grid with it.
    await expect(page.getByRole('link', { name: /^Beliebte Filme/ })).toHaveAttribute(
      'href',
      '/discover/movies',
    );
    await expect(page.getByRole('link', { name: /^Demnächst als Serie/ })).toHaveAttribute(
      'href',
      '/discover/series?sort=newest',
    );
    // Two genres have a backdrop (the JPEG of the TMDB mock), «Dokumentarfilm» none.
    const backdrops = row(page, 'Film-Genres').locator('img');
    await expect(backdrops).toHaveCount(2);
    // The tiles load lazily: Firefox fetches an image only near the viewport, Chromium earlier.
    await backdrops.first().scrollIntoViewIfNeeded();
    await expect
      .poll(() =>
        backdrops.evaluateAll((images) =>
          images.map((image) => (image as HTMLImageElement).naturalWidth),
        ),
      )
      .toEqual([64, 64]);
    await expect(row(page, 'Film-Genres').getByRole('link', { name: 'Drama' })).toHaveAttribute(
      'href',
      '/discover/movies?genre=18',
    );

    // Tab after the area's links and the search: the first card; → walks the row.
    const cards = trending.getByRole('link');
    await tabTo(page, cards.first(), 40);
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, cards.nth(1));
    await page.keyboard.press('ArrowLeft');
    await expectFocusOn(page, cards.first());
    // ↓ reaches the next rows by the page's arrow keys: the title and cards of «Beliebte Filme».
    const inRow = (name: string) =>
      row(page, name).evaluate((section) => section.contains(document.activeElement));
    for (let presses = 0; presses < 3 && !(await inRow('Film-Genres')); presses += 1) {
      await page.keyboard.press('ArrowDown');
      await expectVisibleFocus(page);
    }
    expect(await inRow('Film-Genres')).toBe(true);
    await page.keyboard.press('ArrowUp');
    expect(await inRow('Beliebte Filme')).toBe(true);

    // The plus after the card in the Tab order opens the dialog; Escape returns to it.
    await glass.focus();
    await page.keyboard.press('Tab');
    const plus = trending.getByRole('button', { name: 'Glasvogel anfordern' });
    await expectFocusOn(page, plus);
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', { name: 'Film anfordern: Glasvogel' });
    await expect(dialog).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(dialog).toHaveCount(0);
    await expectFocusOn(page, plus);

    // Enter on a card opens the preview page; back returns with the focus on the same card.
    await glass.focus();
    await page.keyboard.press('ArrowRight');
    const second = cards.nth(1);
    await expectFocusOn(page, second);
    const href = await second.getAttribute('href');
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(new RegExp(`${href}$`));
    await page.goBack();
    await expect(page).toHaveURL(/\/discover$/);
    await expectFocusOn(page, second);

    // The search field leads to «Suche» with the text.
    const field = page.getByRole('searchbox', { name: 'Film oder Serie suchen' });
    await field.focus();
    await page.keyboard.type('Glas');
    await resetFocus(page);
    await expectNoFocusTrap(page, 160);
    await field.focus();
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/discover\/search\?q=Glas$/);
  } finally {
    await closeWishes(owner, setup);
  }
});

test.describe('the answers of the server changed on their way', () => {
  // The routes below must see the requests; a service worker would make them on its own.
  test.use({ serviceWorkers: 'block' });

  test('Seerr out of reach: a row of the last state shows the band, nothing kept shows the error', async ({
    page,
    owner,
  }) => {
    let setup: WishesSetup | undefined;
    try {
      setup = await openWishes(owner);
      // The server serves an older row as stale only after an hour (N-54): the answer is changed
      // on its way, the page shows what it says.
      await page.route('**/api/v1/wishes/discover', async (route) => {
        const response = await route.fetch();
        const body = (await response.json()) as { rows: { stale: boolean }[] };
        body.rows[0].stale = true;
        await route.fulfill({ response, json: body });
      });
      await page.goto('/discover');
      await expect(
        page.getByText('Seerr ist nicht erreichbar. Du siehst den Stand vom letzten Abruf.'),
      ).toBeVisible();
      await expect(row(page, 'Im Trend')).toBeVisible();

      await page.unroute('**/api/v1/wishes/discover');
      await page.route('**/api/v1/wishes/discover', (route) =>
        route.fulfill({
          status: 503,
          contentType: 'application/problem+json',
          json: { type: 'about:blank', title: 'seerr', status: 503, code: 'seerr_unreachable' },
        }),
      );
      await page.reload();
      await expect(page.getByText('Seerr ist gerade nicht erreichbar.')).toBeVisible();
      await expect(page.getByRole('button', { name: 'Erneut versuchen' })).toBeVisible();
      await expect(content(page)).not.toContainText('{{');
    } finally {
      await closeWishes(owner, setup);
    }
  });
});

test('without the right: with wishes the entry leads to «Meine Wünsche» and «Entdecken» says so; without any it is not found', async ({
  page,
  owner,
}) => {
  let setup: WishesSetup | undefined;
  try {
    setup = await openWishes(owner);
    const profile = await useFreshProfile(page, owner, 'E2E Entdecken Recht');
    const made = await page.request.post('/api/v1/wishes', {
      headers: CSRF,
      data: { media: 'movie', tmdbId: 9003, source: 'detail' },
    });
    expect(made.status(), await made.text()).toBe(201);
    await allowRequests(owner, profile.id, false);

    await page.goto('/wishes');
    await expect(page.getByRole('heading', { level: 1, name: 'Meine Wünsche' })).toBeVisible();
    await expect(nav(page).getByRole('link', { name: /^Wünsche/ })).toHaveAttribute(
      'href',
      '/wishes',
    );
    // «Meine Wünsche» alone: no row of links.
    await expect(area(page)).toHaveCount(0);
    await page.goto('/discover');
    await expect(page.getByText('Entdecken gibt es für dieses Profil nicht')).toBeVisible();
    const mine = page.getByRole('link', { name: 'Zu «Meine Wünsche»' });
    await tabTo(page, mine, 40);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/wishes$/);
    // The wish goes again, so no other spec meets it in the administration's list.
    await allowRequests(owner, profile.id, true);
    const { id } = (await made.json()) as { id: number };
    const withdrawn = await page.request.delete(`/api/v1/wishes/${id}`, { headers: CSRF });
    expect(withdrawn.ok(), await withdrawn.text()).toBe(true);

    // A fresh profile has the right from the start (ADR 0045): it is turned off by hand here.
    const without = await useFreshProfile(page, owner, 'E2E Entdecken ohne');
    await allowRequests(owner, without.id, false);
    await page.goto('/discover');
    await expect(page.getByText('Seite nicht gefunden')).toBeVisible();
    await expect(nav(page).getByRole('link', { name: /^Wünsche/ })).toHaveCount(0);
    await expect(content(page)).not.toContainText('{{');
  } finally {
    await closeWishes(owner, setup);
  }
});
