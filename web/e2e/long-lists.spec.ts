import type { Page, Route } from '@playwright/test';
import { expect, test } from './fixtures';
import { expectFocusOn, expectVisibleFocus } from './keyboard';

// Concept 18 C-3: no list has «Mehr laden» any more; a long one reads its pages as it scrolls and
// draws only the rows near the view. The test media hold no list this long, so the lists come from
// routes: an inbox of 450 entries, the station table of the administration with 40'000 and a
// watchlist of 150. The keyboard rules are concept 17 6.2 point 2: ↓ and Tab go on past what is
// drawn.

// The routes answer the page; a service worker would fetch past them.
test.use({ serviceWorkers: 'block' });

/** Answers a list operation from offset and limit; records the offsets asked for. */
async function serve(
  page: Page,
  url: string,
  total: number,
  answer: (offset: number, count: number) => object,
): Promise<number[]> {
  const offsets: number[] = [];
  await page.route(url, async (route: Route) => {
    const query = new URL(route.request().url()).searchParams;
    const offset = Number(query.get('offset') ?? 0);
    const limit = Number(query.get('limit') ?? 100);
    offsets.push(offset);
    await route.fulfill({ json: answer(offset, Math.max(0, Math.min(limit, total - offset))) });
  });
  return offsets;
}

const INBOX = 450;

test('a long inbox reads its pages as ↓ goes on, without «Mehr laden» @keyboard', async ({
  page,
}) => {
  test.setTimeout(120_000);
  const offsets = await serve(page, '**/api/v1/notifications?*', INBOX, (offset, count) => ({
    total: INBOX,
    unread: 0,
    offset,
    limit: 100,
    notifications: Array.from({ length: count }, (_, n) => ({
      id: offset + n + 1,
      kind: 'new_episode',
      createdAt: new Date(Date.UTC(2026, 10, 1) - (offset + n) * 60_000).toISOString(),
      readAt: '2026-11-01T00:00:00Z',
      item: {
        id: `01JLONGLIST${String(offset + n).padStart(15, '0')}`,
        kind: 'episode',
        title: `Folge ${offset + n + 1}`,
        inWatchlist: false,
        images: {},
      },
    })),
  }));
  await page.goto('/notifications');
  const inbox = page.getByRole('list', { name: 'Benachrichtigungen' });
  const entry = (n: number) => inbox.getByRole('link', { name: new RegExp(`Folge ${n},`) });
  await expect(entry(1)).toBeVisible();
  await expect(page.getByRole('button', { name: /Mehr laden/ })).toHaveCount(0);
  await entry(1).focus();
  // Only the rows near the view are in the document.
  expect(await inbox.getByRole('listitem').count()).toBeLessThan(100);

  // ↓ through the first page, one entry per press: the second page is read before the focus
  // gets there.
  for (let n = 2; n <= 106; n += 1) {
    await page.keyboard.press('ArrowDown');
    await expect(entry(n)).toBeFocused();
  }
  await expectVisibleFocus(page);
  expect(offsets).toContain(100);
  expect(await inbox.getByRole('listitem').count()).toBeLessThan(100);

  // The end of the list by scrolling: its last page is read, its last entry drawn.
  await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
  await expect(entry(INBOX)).toBeVisible();
  expect(offsets).toContain(400);
});

const STATIONS = 40_000;

test("the station table of 40'000 draws a few rows and ↓ goes on past them @keyboard", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const offsets = await serve(
    page,
    '**/api/v1/admin/radio/stations?*',
    STATIONS,
    (offset, count) => ({
      total: STATIONS,
      offset,
      items: Array.from({ length: count }, (_, n) => ({
        id: offset + n + 1,
        name: `Sender ${offset + n + 1}`,
        country: 'CH',
        languages: [],
        genres: [],
        kinds: [],
        moods: [],
        hls: false,
        showsTitles: false,
        recommended: false,
        state: 'ok',
        source: 'catalog',
        hidden: false,
        failures: 0,
        tags: [],
        facets: [],
        locks: [],
        logoState: 'none',
        aliasCount: 0,
        streams: [],
        createdAt: 0,
        updatedAt: 0,
      })),
    }),
  );
  await page.goto('/admin/radio?tab=sender');
  const table = page.getByRole('table', { name: 'Sender', exact: true });
  const edit = (n: number) => table.getByRole('button', { name: `Sender ${n} bearbeiten` });
  await expect(edit(1)).toBeVisible();
  await expect(page.getByRole('button', { name: /Weitere Sender|Mehr laden/ })).toHaveCount(0);
  expect(await table.getByRole('row').count()).toBeLessThan(120);

  // ↓ from row to row in the same column, across the first page.
  await edit(1).focus();
  for (let n = 2; n <= 111; n += 1) {
    await page.keyboard.press('ArrowDown');
    await expect(edit(n)).toBeFocused();
  }
  await expectVisibleFocus(page);
  expect(offsets).toContain(100);
  expect(await table.getByRole('row').count()).toBeLessThan(120);

  // A jump into the middle with the scroll bar reads only the pages there.
  await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight / 2));
  await expect(
    table.getByRole('rowheader', { name: /^Sender (19|20)\d{3}$/ }).first(),
  ).toBeVisible();
  expect(offsets.length).toBeLessThan(10);
  // The row with the focus stays drawn far above, with its entry: the focus stays on it.
  await expect(edit(111)).toBeFocused();

  // ↑ onto a row above the drawn ones scrolls it into view below the fixed header, not under it.
  // The top of the rows drawn in the middle (the row with the focus comes before them).
  const places = await table
    .locator('tbody tr[data-place]')
    .evaluateAll((rows) => rows.map((row) => Number(row.getAttribute('data-place'))));
  const top = Math.min(...places.filter((place) => place > 110));
  await edit(top + 2).focus();
  await page.keyboard.press('ArrowUp');
  await page.keyboard.press('ArrowUp');
  await expect(edit(top)).toBeFocused();
  const header = await page.getByRole('banner').boundingBox();
  const row = await edit(top).boundingBox();
  expect(row!.y).toBeGreaterThanOrEqual(header!.y + header!.height - 1);
});

const WATCHLIST = 150;

test('focus stays after removing on page 2 of the watchlist @keyboard', async ({ page }) => {
  // The titles' numbers in order; «Entfernen» takes one off. A second read of page 2 answers
  // late, so page 1 with the new total is there first (the order after an invalidation).
  const titles = Array.from({ length: WATCHLIST }, (_, n) => n + 1);
  const offsets: number[] = [];
  await page.route('**/api/v1/watchlist?*', async (route: Route) => {
    const query = new URL(route.request().url()).searchParams;
    const offset = Number(query.get('offset') ?? 0);
    const limit = Number(query.get('limit') ?? 100);
    offsets.push(offset);
    const answer = {
      total: titles.length,
      offset,
      limit,
      items: titles.slice(offset, offset + limit).map((n) => ({
        id: `01JWATCHLIST${String(n).padStart(14, '0')}`,
        kind: 'movie',
        title: `Titel ${n}`,
        inWatchlist: true,
        images: {},
      })),
    };
    if (offset > 0 && titles.length < WATCHLIST) await new Promise((late) => setTimeout(late, 800));
    await route.fulfill({ json: answer });
  });
  await page.route('**/api/v1/items/*/watchlist', async (route: Route) => {
    titles.splice(titles.indexOf(121), 1);
    await route.fulfill({ status: 204 });
  });
  await page.goto('/watchlist');
  const grid = page.getByRole('list', { name: 'Merkliste' });
  const remove = (n: number) =>
    grid.getByRole('button', { name: `Titel ${n} von der Merkliste entfernen` });
  await expect(remove(150)).toBeAttached();
  await remove(121).focus();
  await page.keyboard.press('Enter');
  // The focus goes to the neighbour and stays there while page 2 is read again and after.
  await expectFocusOn(page, remove(122));
  await expect.poll(() => offsets.filter((offset) => offset === 100).length).toBe(2);
  await expect(grid.getByRole('link', { name: /^Titel 150\b/ })).toBeAttached();
  await expect(remove(121)).toHaveCount(0);
  await expectFocusOn(page, remove(122));
  await expect(grid.getByRole('listitem')).toHaveCount(WATCHLIST - 1);
});
