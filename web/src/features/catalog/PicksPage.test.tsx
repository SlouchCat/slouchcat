import type { components, ItemRef } from '@slouchcat/api-client';
import { act, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { alien, heat, ids } from '@/test/fixtures';
import { json, mockApi, renderApp, signedIn } from '@/test/api';
import { virtualLayout } from '@/test/virtual';

type PicksPage = components['schemas']['PicksPage'];

const page = (items: ItemRef[], total = items.length, offset = 0): PicksPage => ({
  items,
  total,
  offset,
  limit: 100,
});

const many = (count: number, from = 0) =>
  Array.from({ length: count }, (_, index) => ({
    ...heat,
    id: `01JB8Z6Q4M7N9P2R3S5T7V${String(from + index).padStart(4, '0')}`,
    title: `Heat ${from + index}`,
  }));

// The page of the user picks (concept 18 part B): newest pick first, without a note, and no
// «Mehr laden»: the next page is read while the grid scrolls (16.4).
describe('User picks page', () => {
  let layout: ReturnType<typeof virtualLayout>;
  beforeEach(() => {
    layout = virtualLayout();
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('lists the picks in the order of the server, with the year under the poster', async () => {
    mockApi({ ...signedIn, 'GET /picks': () => json(page([heat, alien])) });
    renderApp('/picks');
    expect(
      await screen.findByRole('heading', { level: 1, name: 'User-Empfehlungen' }),
    ).toBeVisible();
    expect(await screen.findByText('2 Titel')).toBeVisible();
    expect(
      screen.getByText('Titel, die Profile dieses Haushalts empfehlen, neueste Empfehlung zuerst.'),
    ).toBeVisible();
    const grid = await screen.findByRole('list', { name: 'User-Empfehlungen' });
    const cards = await within(grid).findAllByRole('link');
    expect(cards.map((card) => card.getAttribute('href'))).toEqual([
      `/items/${ids.heat}`,
      `/items/${ids.alien}`,
    ]);
    expect(cards[1]).toHaveTextContent('1979');
    expect(screen.queryByRole('button', { name: /Mehr laden/ })).not.toBeInTheDocument();
  });

  it('reads a long list page by page as the grid scrolls', async () => {
    // The offset of the request being answered (the side navigation asks with limit 1).
    const offsets: number[] = [];
    const { fetchMock } = mockApi({
      ...signedIn,
      'GET /picks': () => {
        const url = new URL(fetchMock.mock.calls.at(-1)![0].url);
        const offset = Number(url.searchParams.get('offset') ?? 0);
        if (url.searchParams.get('limit') === '100') offsets.push(offset);
        return json(page(many(Math.min(100, 250 - offset), offset), 250, offset));
      },
    });
    renderApp('/picks');
    await screen.findByRole('list', { name: 'User-Empfehlungen' });
    expect(await screen.findByText('250 Titel')).toBeVisible();
    // One page only while the top is in view.
    await waitFor(() => expect(screen.getByRole('link', { name: /Heat 0/ })).toBeVisible());
    expect(offsets).toEqual([0]);
    act(() => layout.scroll(layout.top + 17 * layout.rowSize));
    await waitFor(() => expect(screen.getByRole('link', { name: /Heat 105/ })).toBeVisible());
    expect(offsets).toEqual([0, 100]);
    expect(screen.queryByRole('button', { name: /Mehr laden/ })).not.toBeInTheDocument();
  });

  it('a title recommended again beyond the first page stands once, at the front', async () => {
    // Between the reads of page 0 and page 1, «Heat 110» is recommended again: same total, the
    // title moves to the front and page 1 now starts with «Heat 99», which page 0 already holds.
    const before = many(120);
    const after = [before[110]!, ...before.slice(0, 110), ...before.slice(111)];
    let moved = false;
    const { fetchMock } = mockApi({
      ...signedIn,
      'GET /picks': () => {
        const url = new URL(fetchMock.mock.calls.at(-1)![0].url);
        const offset = Number(url.searchParams.get('offset') ?? 0);
        const limit = Number(url.searchParams.get('limit') ?? 100);
        const list = moved ? after : before;
        if (offset === 0 && limit === 100) moved = true;
        return json(page(list.slice(offset, offset + limit), 120, offset));
      },
    });
    renderApp('/picks');
    const grid = await screen.findByRole('list', { name: 'User-Empfehlungen' });
    await waitFor(() => expect(within(grid).getAllByRole('link')[0]).toHaveTextContent('Heat 110'));
    await waitFor(() => expect(within(grid).getAllByRole('link')).toHaveLength(120));
    const hrefs = within(grid)
      .getAllByRole('link')
      .map((card) => card.getAttribute('href'));
    expect(new Set(hrefs).size).toBe(120);
    expect(hrefs).toEqual(after.map((item) => `/items/${item.id}`));
  });
});
