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

  it('reads the next page once the grid scrolls to its end', async () => {
    // The offset of the request being answered (the side navigation asks with limit 1).
    const offsets: number[] = [];
    const { fetchMock } = mockApi({
      ...signedIn,
      'GET /picks': () => {
        const url = new URL(fetchMock.mock.calls.at(-1)![0].url);
        const offset = Number(url.searchParams.get('offset') ?? 0);
        if (url.searchParams.get('limit') === '100') offsets.push(offset);
        return json(offset === 100 ? page(many(20, 100), 120, 100) : page(many(100), 120));
      },
    });
    renderApp('/picks');
    await screen.findByRole('list', { name: 'User-Empfehlungen' });
    expect(await screen.findByText('120 Titel')).toBeVisible();
    // One page only while the top is in view.
    expect(offsets).toEqual([0]);
    act(() => layout.scroll(layout.top + 17 * layout.rowSize));
    await waitFor(() => expect(screen.getByRole('link', { name: /Heat 119/ })).toBeVisible());
    expect(offsets).toEqual([0, 100]);
    expect(screen.queryByRole('button', { name: /Mehr laden/ })).not.toBeInTheDocument();
  });
});
