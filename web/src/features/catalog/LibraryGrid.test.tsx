import type { components, ItemRef } from '@slouchcat/api-client';
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { setSpatialFocus } from '@/ui';
import { json, mockApi, renderApp, signedIn } from '@/test/api';
import { virtualLayout } from '@/test/virtual';

type LibraryItemsPage = components['schemas']['LibraryItemsPage'];

// The virtual library grid (concept 17 E-11, E-12): six columns of 100 px, rows of 190 px and a
// gap of 20 px from 300 px down (src/test/virtual.ts), 800 px of window: three rows in view and
// four drawn above and below.

const TOTAL = 1800;

const title = (index: number): ItemRef => ({
  id: `01J9LIBRARYGRID${String(index).padStart(11, '0')}`,
  kind: 'movie',
  title: `Titel ${index}`,
  inWatchlist: false,
  year: 2000,
  images: {},
});

function mockLibrary() {
  const offsets: number[] = [];
  const api = mockApi({
    ...signedIn,
    'GET /libraries/visible': () =>
      json([{ id: 3, name: 'Filme', kind: 'video', contents: ['movie'], itemCount: TOTAL }]),
    'GET /libraries/*/items': () => {
      const url = new URL(api.fetchMock.mock.calls.at(-1)![0].url);
      const offset = Number(url.searchParams.get('offset'));
      offsets.push(offset);
      const limit = Number(url.searchParams.get('limit'));
      const page: LibraryItemsPage = {
        items: Array.from({ length: Math.min(limit, TOTAL - offset) }, (_, index) =>
          title(offset + index),
        ),
        total: TOTAL,
        offset,
        limit,
        facets: { letters: ['T'], genres: [], ratings: [] },
      };
      return json(page);
    },
  });
  return offsets;
}

const grid = () => screen.getByRole('list', { name: 'Filme' });
const place = (index: number) =>
  grid().querySelector<HTMLElement>(`[data-index="${index}"]`) ?? undefined;
const card = (index: number) => place(index)?.querySelector<HTMLAnchorElement>('a[href]');

async function open() {
  const layout = virtualLayout();
  const offsets = mockLibrary();
  renderApp('/libraries/3');
  await waitFor(() => expect(card(0)).toBeDefined());
  return { layout, offsets };
}

describe('library grid', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('draws the rows in view and a screen above and below, every place counted', async () => {
    const { offsets } = await open();
    const places = within(grid()).getAllByRole('listitem');
    // Rows 0 to 2 in view, 4 more below: 7 rows of 6.
    expect(places).toHaveLength(42);
    expect(places[0]).toHaveAttribute('aria-setsize', String(TOTAL));
    expect(places[41]).toHaveAttribute('aria-posinset', '42');
    expect(card(41)).toHaveAttribute('href', `/items/${title(41).id}`);
    expect(place(42)).toBeUndefined();
    expect(offsets).toEqual([0]);
  });

  it('a jump into the middle draws empty places first, then the cards of their page', async () => {
    const { layout, offsets } = await open();
    act(() => layout.scroll(layout.top + 50 * layout.rowSize));
    // Row 50 (place 300) is drawn at once, empty and not focusable until page 3 is there.
    await waitFor(() => expect(place(300)).toBeDefined());
    expect(card(300) ?? null).toBeNull();
    expect(place(300)!.querySelector('[tabindex], a, button')).toBeNull();
    await waitFor(() => expect(card(300)).toHaveAccessibleName(/Titel 300/));
    expect(offsets).toEqual([0, 200, 300]);
  });

  it('ArrowDown from the last drawn row scrolls on and focuses the card below once it is there', async () => {
    const { layout } = await open();
    setSpatialFocus(`library-item-${title(36).id}`);
    await waitFor(() => expect(card(36)).toHaveFocus());
    fireEvent.keyDown(card(36)!, { key: 'ArrowDown' });
    // Place 42 (row 7) was not drawn: the grid scrolled to it and focused it when it came.
    await waitFor(() => expect(card(42)).toHaveFocus());
    expect(layout.scrollY).toBeGreaterThan(0);
  });

  it('Tab past the last drawn card brings the next card and focuses it', async () => {
    await open();
    const last = place(41)!;
    const controls = last.querySelectorAll<HTMLElement>('a[href], button:not([disabled])');
    const end = controls[controls.length - 1]!;
    end.focus();
    const event = fireEvent.keyDown(end, { key: 'Tab' });
    expect(event).toBe(false); // the grid took the Tab
    await waitFor(() => expect(card(42)).toHaveFocus());
    // Shift+Tab from that link: place 41 is drawn, the browser's own Tab order goes back.
    expect(fireEvent.keyDown(card(42)!, { key: 'Tab', shiftKey: true })).toBe(true);
  });

  it('the row with the focus stays drawn when the wheel scrolls it away', async () => {
    const { layout } = await open();
    setSpatialFocus(`library-item-${title(3).id}`);
    await waitFor(() => expect(card(3)).toHaveFocus());
    act(() => layout.scroll(layout.top + 100 * layout.rowSize));
    await waitFor(() => expect(place(600)).toBeDefined());
    expect(card(3)).toHaveFocus();
    expect(place(6)).toBeUndefined();
  });
});
