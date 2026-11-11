import type { APIRequestContext } from '@playwright/test';
import { CSRF } from './catalog';
import { DEFAULT_HOME_ROWS, listHomeRows, setHomeRows } from './curation';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, resetFocus, tabTo } from './keyboard';

// The start page row «Meine Sender» (concept 16, 10.2; plan W2, D-7) by keyboard (rule 8): shown
// through the administration's API at the top, a favourite of the profile as a square tile; Tab
// reaches it, Enter plays it through the app's radio, the tile says so; after a reload the tile
// names the title its running relay plays. The stored order of the rows and the favourite are
// restored afterwards (critic m-18).

const STATION = 'Testradio Drei';
/** The ICY title of the test station, as the mini player shows it (artist – title). */
const ICY_LINE = /^Slouchcat Testton – Titel \d+$/;
/** The same under the tile's name. */
const ICY_CAPTION = /Slouchcat Testton – Titel \d+/;
/** Relay start (ffmpeg, the first segments). */
const LIVE_TIMEOUT_MS = 30_000;

async function stationId(owner: APIRequestContext, name: string): Promise<number> {
  const response = await owner.get(
    `/api/v1/admin/radio/stations?offered=false&limit=50&q=${encodeURIComponent(name)}`,
  );
  expect(response.status(), await response.text()).toBe(200);
  const { items } = (await response.json()) as { items: { id: number; name: string }[] };
  const station = items.find((entry) => entry.name === name);
  expect(station, `station "${name}" in the catalog`).toBeDefined();
  return station!.id;
}

async function stopRadio(owner: APIRequestContext): Promise<void> {
  const response = await owner.get('/api/v1/admin/radio/relays');
  if (!response.ok()) return;
  const { relays } = (await response.json()) as { relays: { listeners: { listenId: string }[] }[] };
  for (const listener of relays.flatMap((relay) => relay.listeners)) {
    await owner.delete(`/api/v1/admin/radio/listeners/${listener.listenId}`, { headers: CSRF });
  }
}

test('«Meine Sender» on the start page: reached by keyboard, plays, names the current title @keyboard', async ({
  page,
  owner,
}) => {
  test.setTimeout(120_000);
  const id = await stationId(owner, STATION);
  const stored = await listHomeRows(owner);
  const favorite = await owner.put(`/api/v1/radio/favorites/${id}`, { headers: CSRF });
  expect(favorite.ok(), await favorite.text()).toBe(true);
  try {
    // The row on top, the default rows after it.
    await setHomeRows(owner, ['radio', ...DEFAULT_HOME_ROWS]);
    await page.goto('/');
    const row = page.getByRole('region', { name: 'Meine Sender' });
    await expect(row).toBeVisible();
    await expect(row.getByRole('link', { name: 'Alle anzeigen ›' })).toHaveAttribute(
      'href',
      '/radio',
    );
    const tile = row.getByRole('button', { name: `${STATION} abspielen` });

    await resetFocus(page);
    await tabTo(page, tile, 40);
    await page.keyboard.press('Enter');
    const mini = page.getByTestId('radio-mini-player');
    await expect(mini).toHaveAttribute('data-state', 'live', { timeout: LIVE_TIMEOUT_MS });
    const playing = row.getByRole('button', { name: `${STATION} pausieren` });
    await expectFocusOn(page, playing);
    await expect(playing).toContainText('Läuft');
    // The heart is the next stop and says the station is a favourite.
    await page.keyboard.press('Tab');
    await expectFocusOn(
      page,
      row.getByRole('button', { name: `${STATION} in Meine Sender`, pressed: true }),
    );
    // The poster rows below give the server administrator three Tab stops per poster since
    // concept 17 D-9 (link, pen, watchlist button), more than the default budget of 80.
    await expectNoFocusTrap(page, 160);

    // The relay runs on after the page went (2 minutes without listeners): the reloaded start
    // page names the title at its live edge.
    await expect(page.getByTestId('radio-mini-line')).toHaveText(ICY_LINE, {
      timeout: LIVE_TIMEOUT_MS,
    });
    await page.reload();
    await expect(row.getByRole('button', { name: `${STATION} abspielen` })).toContainText(
      ICY_CAPTION,
    );
  } finally {
    await stopRadio(owner);
    await owner.delete(`/api/v1/radio/favorites/${id}`, { headers: CSRF });
    const restore = await owner.put('/api/v1/admin/home-rows', {
      headers: CSRF,
      data: stored.map(({ id: rowId, key, hidden }, position) => ({
        id: rowId,
        key,
        hidden,
        position,
      })),
    });
    expect(restore.status(), await restore.text()).toBe(200);
  }
});
