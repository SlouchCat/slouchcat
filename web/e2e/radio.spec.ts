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
import { arrowTo } from './playback';

// /radio (concept 16, 10.1, variant A) by keyboard (rule 8): the search, the quick filters and the
// facet selects in the URL, Back restores the filter, Enter on a tile plays, visible focus, no
// focus trap; on a phone a facet opens as a sheet that Escape closes. Favourites survive a reload;
// a hidden station is never listed (404 for its id). The catalog is the one of the global setup
// (testdata/radio/catalog.json: 68 entries, 58 stations).

const SEARCH = 'Testradio';

interface Station {
  id: number;
  name: string;
}

/** The station with exactly this name from the administration's list (also hidden ones). */
async function adminStation(owner: APIRequestContext, name: string): Promise<Station> {
  const response = await owner.get(
    `/api/v1/admin/radio/stations?exceptions=false&limit=50&q=${encodeURIComponent(name)}`,
  );
  expect(response.status(), await response.text()).toBe(200);
  const { items } = (await response.json()) as { items: Station[] };
  const station = items.find((entry) => entry.name === name);
  expect(station, `station "${name}" in the catalog`).toBeDefined();
  return station!;
}

async function stopRadio(owner: APIRequestContext): Promise<void> {
  const response = await owner.get('/api/v1/admin/radio/relays');
  if (!response.ok()) return;
  const { relays } = (await response.json()) as { relays: { listeners: { listenId: string }[] }[] };
  for (const listener of relays.flatMap((relay) => relay.listeners)) {
    await owner.delete(`/api/v1/admin/radio/listeners/${listener.listenId}`, { headers: CSRF });
  }
}

/** No stored filter: /radio opens as it does for a new profile. */
async function resetRadioPreferences(owner: APIRequestContext): Promise<void> {
  const response = await owner.put('/api/v1/radio/preferences', {
    headers: CSRF,
    data: { defaultCountry: 'CH', lastFilter: {}, lastSort: 'popular' },
  });
  expect(response.status(), await response.text()).toBe(200);
}

const results = (page: Page) => page.getByRole('list', { name: 'Treffer' });
const tileOf = (page: Page, name: string) =>
  results(page).getByRole('button', { name: `${name} abspielen` });

test.beforeEach(async ({ owner }) => {
  await resetRadioPreferences(owner);
});

test.afterEach(async ({ owner }) => {
  await stopRadio(owner);
  await resetRadioPreferences(owner);
});

test('radio page by keyboard: search and filters in the URL, Back restores them, Enter plays @keyboard', async ({
  page,
}) => {
  test.setTimeout(90_000);
  await page.goto('/radio');
  await expect(page.getByRole('heading', { level: 1, name: 'Radio' })).toBeVisible();
  await expect(page.getByRole('list', { name: 'Alle Sender' })).toBeVisible();
  // Without a filter the browse rows show.
  await expect(page.getByRole('heading', { name: 'Nach Genre' })).toBeVisible();

  // The search field by Tab; the URL follows the typed text.
  await resetFocus(page);
  const field = page.getByRole('searchbox', { name: 'Sender suchen' });
  await tabTo(page, field, 60);
  await page.keyboard.type(SEARCH);
  await expect(page).toHaveURL(/[?&]q=Testradio/);
  await expect(tileOf(page, 'Testradio Drei')).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Nach Genre' })).toHaveCount(0);

  // The quick filters: arrows move between them, Enter toggles one.
  const quick = page.getByRole('group', { name: 'Schnellwahl' });
  const myCountry = quick.getByRole('button', { name: 'Mein Land: Schweiz' });
  await tabTo(page, myCountry);
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, quick.getByRole('button', { name: 'Meine Sender' }));
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, myCountry);
  await page.keyboard.press('Enter');
  await expect(myCountry).toHaveAttribute('aria-pressed', 'true');
  await expect(page).toHaveURL(/[?&]country=CH/);

  // A facet select: Enter opens its values, Enter picks one, Escape closes with the focus back.
  const genre = page.getByRole('button', { name: /^Genre/ });
  await tabTo(page, genre);
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, page.getByRole('button', { name: /^Sendeart/ }));
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, genre);
  await page.keyboard.press('Enter');
  const values = page.getByRole('dialog', { name: 'Genre' });
  await expect(values).toBeVisible();
  const jazz = values.getByRole('button', { name: /^Jazz/ });
  await tabTo(page, jazz);
  await page.keyboard.press('Enter');
  await expect(jazz).toHaveAttribute('aria-pressed', 'true');
  await expect(page).toHaveURL(/[?&]genre=jazz/);
  await page.keyboard.press('Escape');
  await expect(values).toHaveCount(0);
  await expectFocusOn(page, genre);
  await expect(page.getByRole('button', { name: 'Filter Jazz entfernen' })).toBeVisible();
  await expect(tileOf(page, 'Testradio Drei')).toBeVisible();
  await expect(tileOf(page, 'Testradio Eins')).toHaveCount(0);

  // Back undoes the last filter: the URL and the chips follow it.
  await page.goBack();
  await expect(page).not.toHaveURL(/genre=/);
  await expect(page).toHaveURL(/[?&]country=CH/);
  await expect(page.getByRole('button', { name: 'Filter Jazz entfernen' })).toHaveCount(0);
  await expect(tileOf(page, 'Testradio Eins')).toBeVisible();

  // Enter on a tile plays the station; the mini player shows it.
  await resetFocus(page);
  await tabTo(page, tileOf(page, 'Testradio Eins'), 80);
  await page.keyboard.press('Enter');
  await expect(page.getByTestId('radio-mini-open')).toHaveAccessibleName(
    'Jetzt läuft öffnen: Testradio Eins',
  );
  await expect(results(page).getByRole('button', { name: 'Testradio Eins pausieren' })).toBeVisible(
    { timeout: 30_000 },
  );

  // Arrows move through the grid.
  const tiles = results(page).getByRole('button', { name: / (abspielen|pausieren)$/ });
  await expect(tiles.nth(1)).toBeVisible();
  await tiles.first().focus();
  await arrowTo(page, tiles.nth(1), 'ArrowRight');
  await expectVisibleFocus(page);

  await expectNoFocusTrap(page);
});

test('a favourite survives a reload; a hidden station is never listed', async ({ page, owner }) => {
  const favourite = await adminStation(owner, 'Testradio Zwei');
  const hidden = await adminStation(owner, 'Radio Swiss Pop');
  try {
    await page.goto(`/radio?q=${SEARCH}`);
    const heart = results(page).getByRole('button', { name: 'Testradio Zwei in Meine Sender' });
    await expect(heart).toHaveAttribute('aria-pressed', 'false');
    await heart.click();
    await expect(heart).toHaveAttribute('aria-pressed', 'true');
    await page.reload();
    await expect(heart).toHaveAttribute('aria-pressed', 'true');
    await page.goto('/radio?fav=true');
    await expect(tileOf(page, 'Testradio Zwei')).toBeVisible();

    // Hidden by the administration: in no list, and its id answers 404.
    const patch = await owner.patch(`/api/v1/admin/radio/stations/${hidden.id}`, {
      headers: CSRF,
      data: { hidden: true },
    });
    expect(patch.status(), await patch.text()).toBe(200);
    await page.goto(`/radio?q=${encodeURIComponent('Swiss Pop')}`);
    await expect(page.getByText('Keine Sender gefunden')).toBeVisible();
    await expect(tileOf(page, 'Radio Swiss Pop')).toHaveCount(0);
    expect((await page.request.get(`/api/v1/radio/stations/${hidden.id}`)).status()).toBe(404);
  } finally {
    await owner.patch(`/api/v1/admin/radio/stations/${hidden.id}`, {
      headers: CSRF,
      data: { hidden: false },
    });
    await owner.delete(`/api/v1/radio/favorites/${favourite.id}`, { headers: CSRF });
  }
});

test('on a phone a facet opens as a sheet that Escape closes @keyboard', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 812 });
  await page.goto('/radio');
  await expect(page.getByRole('list', { name: 'Alle Sender' })).toBeVisible();
  await resetFocus(page);
  const mood = page.getByRole('button', { name: /^Stimmung/ });
  await tabTo(page, mood, 60);
  await page.keyboard.press('Enter');
  const sheet = page.getByRole('dialog', { name: 'Stimmung' });
  await expect(sheet).toBeVisible();
  await expectFocusOn(page, sheet.getByRole('button', { name: 'Filter schliessen' }));
  await tabTo(page, sheet.getByRole('button', { name: /^Chill/ }));
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/[?&]mood=chill/);
  await expect(sheet.getByRole('button', { name: /Sender anzeigen$/ })).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(sheet).toHaveCount(0);
  await expectFocusOn(page, mood);
  await expectNoFocusTrap(page);
});
