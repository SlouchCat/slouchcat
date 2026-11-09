import type { Page, Route } from '@playwright/test';
import { expect, test } from './fixtures';
import { expectVisibleFocus } from './keyboard';

// Concept 18 C-3: no list has «Mehr laden» any more; a long one reads its pages as it scrolls and
// draws only the rows near the view. The test media hold no list this long, so the two lists come
// from routes: an inbox of 450 entries and the station table of the administration with 40'000.
// The keyboard rules are concept 17 6.2 point 2: ↓ and Tab go on past what is drawn.

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
});
