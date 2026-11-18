import type { Page, Route } from '@playwright/test';
import { idOf, itemPath, SHOW_TITLE } from './catalog';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, resetFocus } from './keyboard';
import { closeWishes, openWishes, seerrRequests, type WishesSetup } from './wishes';

// Concept 25 stage 4 (M-06, plan 4.2): the missing seasons on the page of a series of the library,
// by keyboard (rule 8). The test media's series has no TMDB id in the e2e run (no key, titles from
// the file names), and a manual match would rename it for every other spec, which cannot be
// undone. So the server's answer of getItem gets the catalog of the TMDB mock's series 9100
// («Nebelhafen»: seasons 1 to 3, season 4 announced), as the notifications and start page specs
// feed their data; the request dialog, the wish and Seerr are the real ones (openWishes). The
// season catalog itself (series.catalog, visibility) is covered by the server's tests.

const TMDB_ID = 9100;

// The fed answers must reach the page every time: no service worker answering from its cache.
test.use({ serviceWorkers: 'block' });

type Catalog = {
  inProduction: boolean;
  present: number;
  total: number;
  seasons: { number: number; episodeCount: number; episodes: number; state: string }[];
  nextEpisode?: { season: number; episode: number; airDate?: string };
};

/** The catalog of the answer: season 2 missing until a request holds it. */
const catalogOf = (requested: boolean): Catalog => ({
  inProduction: true,
  present: 1,
  total: 3,
  seasons: [
    { number: 1, episodeCount: 3, episodes: 3, state: 'present' },
    { number: 2, episodeCount: 10, episodes: 0, state: requested ? 'requested' : 'missing' },
    { number: 3, episodeCount: 10, episodes: 0, state: 'requested' },
    { number: 4, episodeCount: 8, episodes: 0, state: 'announced' },
  ],
  nextEpisode: { season: 4, episode: 1, airDate: '2026-12-04' },
});

const wall = (page: Page) => page.getByRole('list', { name: 'Staffeln' });

/** Feeds the catalog into getItem of the series; returns a switch for «season 2 requested». */
async function feedCatalog(page: Page, seriesId: string) {
  let requested = false;
  await page.route(`**/api/v1/items/${seriesId}`, async (route: Route) => {
    const response = await route.fetch();
    const body = (await response.json()) as Record<string, unknown>;
    await route.fulfill({
      response,
      json: { ...body, tmdbId: TMDB_ID, catalog: catalogOf(requested), canRequestSeasons: true },
    });
  });
  // The library has season 1 of the series: Seerr's list says so in the dialog too.
  await page.route(`**/api/v1/titles/show/${TMDB_ID}/seasons`, async (route: Route) => {
    const response = await route.fetch();
    const body = (await response.json()) as { seasons: { number: number; state: string }[] };
    await route.fulfill({
      response,
      json: {
        ...body,
        seasons: body.seasons.map((season) =>
          season.number === 1 ? { ...season, state: 'available' } : season,
        ),
      },
    });
  });
  page.on('request', (request) => {
    if (request.method() === 'POST' && request.url().endsWith('/api/v1/wishes')) requested = true;
  });
}

test('missing seasons: the line, the marked covers, arrows reach them, Enter requests the pressed one, Escape returns @keyboard', async ({
  page,
  owner,
}) => {
  let setup: WishesSetup | undefined;
  try {
    setup = await openWishes(owner);
    const series = await itemPath(page, SHOW_TITLE);
    await feedCatalog(page, idOf(series));
    await page.goto(series);
    await expect(page.getByRole('heading', { level: 1, name: SHOW_TITLE })).toBeVisible();
    await expect(
      page.getByText(/^1 von 3 Staffeln vorhanden · Läuft noch, nächste Folge am /),
    ).toBeVisible();
    const missing = wall(page).getByRole('button', { name: 'Staffel 2 anfordern' });
    await expect(missing).toBeVisible();
    await expect(missing).toContainText('Fehlt');
    await expect(
      wall(page).getByRole('button', { name: 'Staffel 3: bereits angefordert, mitwünschen' }),
    ).toContainText('Angefordert');
    await expect(wall(page).getByRole('button', { name: 'Staffel 4: Angekündigt' })).toBeVisible();
    await expect(page.locator('#content')).not.toContainText('{{');

    // The arrows reach the missing season from the present one, as between the seasons.
    await resetFocus(page);
    const first = wall(page).getByRole('button', { name: /^Season 1 \d/ });
    await first.focus();
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, missing);

    // Enter opens the dialog with the pressed season chosen and focused; arrows stay inside.
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', { name: `Serie anfordern: ${SHOW_TITLE}` });
    await expect(dialog).toBeVisible();
    const two = dialog.getByRole('checkbox', { name: 'Staffel 2' });
    await expect(two).toBeChecked();
    await expectFocusOn(page, two);
    await expect(dialog.getByRole('checkbox', { name: 'Staffel 3' })).not.toBeChecked();
    // The announced season is not offered.
    await expect(dialog.getByRole('checkbox', { name: 'Staffel 4' })).toHaveCount(0);
    await page.keyboard.press('ArrowDown');
    expect(await dialog.evaluate((element) => element.contains(document.activeElement))).toBe(true);
    // Escape returns the focus to the cover.
    await page.keyboard.press('Escape');
    await expect(dialog).toHaveCount(0);
    await expectFocusOn(page, missing);

    // ↓ from the wall reaches «Staffeln anfordern»; its dialog chooses every missing season.
    await page.keyboard.press('ArrowDown');
    const requestAll = page.getByRole('button', { name: 'Staffeln anfordern' });
    await expectFocusOn(page, requestAll);
    await page.keyboard.press('Enter');
    await expect(dialog.getByRole('checkbox', { name: /^Alle fehlenden Staffeln/ })).toBeChecked();
    await page.keyboard.press('Escape');
    await expect(dialog).toHaveCount(0);
    await expectFocusOn(page, requestAll);

    // Request the season: the cover says «Angefordert» and keeps the focus.
    await missing.focus();
    await page.keyboard.press('Enter');
    const submit = dialog.getByRole('button', { name: '1 Staffel anfordern' });
    await expect(submit).toBeEnabled();
    await submit.focus();
    await page.keyboard.press('Enter');
    await expect(dialog).toHaveCount(0);
    const held = wall(page).getByRole('button', {
      name: 'Staffel 2: bereits angefordert, mitwünschen',
    });
    await expect(held).toContainText('Angefordert');
    await expectFocusOn(page, held);
    await expect(page.getByText(/^Angefordert. Den Stand siehst du/)).toBeVisible();
    await expect
      .poll(async () =>
        (await seerrRequests()).some(
          (request) => request.type === 'tv' && request.mediaId === TMDB_ID,
        ),
      )
      .toBe(true);
    await expectNoFocusTrap(page, 80);
  } finally {
    await page.unrouteAll({ behavior: 'ignoreErrors' });
    await closeWishes(owner, setup);
  }
});
