import type { components, ItemRef } from '@slouchcat/api-client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, renderHook, waitFor } from '@testing-library/react';
import { createElement, type ReactNode } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { patchWatchlist } from '@/features/discover/queries';
import type { LibraryRequest } from './libraryParams';
import {
  LIBRARY_GRID_PAGE,
  libraryPageQuery,
  useLibraryPages,
  type DrawnRange,
} from './libraryPages';

type LibraryItemsPage = components['schemas']['LibraryItemsPage'];

const item = (index: number): ItemRef => ({
  id: `t${index}`,
  kind: 'movie',
  title: `Titel ${index}`,
  inWatchlist: false,
  images: {},
});

interface Asked {
  offset: number;
  genre: string | null;
  signal: AbortSignal;
}

/**
 * A library of `total` titles behind fetch. Every request is recorded; `hold(offset)` keeps the
 * answer to that offset back until `release(offset)` (or forever).
 */
function library(initial: number) {
  const state = { total: initial };
  const asked: Asked[] = [];
  const held = new Map<number, (() => void)[]>();
  const holding = new Set<number>();
  const answer = (offset: number, genre: string | null): Response => {
    const total = genre ? Math.floor(state.total / 2) : state.total;
    const items = Array.from(
      { length: Math.max(0, Math.min(LIBRARY_GRID_PAGE, total - offset)) },
      (_, index) => item(offset + index),
    );
    const page: LibraryItemsPage = {
      items,
      total,
      offset,
      limit: LIBRARY_GRID_PAGE,
      facets: { letters: [], genres: [], ratings: [] },
    };
    return Response.json(page);
  };
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: Request) => {
      const url = new URL(input.url);
      const offset = Number(url.searchParams.get('offset'));
      const genre = url.searchParams.get('genre');
      asked.push({ offset, genre, signal: input.signal });
      if (holding.has(offset)) {
        await new Promise<void>((resolve) =>
          held.set(offset, [...(held.get(offset) ?? []), resolve]),
        );
      }
      if (input.signal.aborted) throw new DOMException('aborted', 'AbortError');
      return answer(offset, genre);
    }),
  );
  return {
    state,
    asked,
    offsets: (genre: string | null = null) =>
      asked.filter((entry) => entry.genre === genre).map((entry) => entry.offset),
    hold: (offset: number) => holding.add(offset),
    release: (offset: number) => {
      holding.delete(offset);
      for (const resolve of held.get(offset) ?? []) resolve();
      held.delete(offset);
    },
  };
}

function render(range: DrawnRange, query: LibraryRequest = {}) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  const hook = renderHook(
    (props: { range: DrawnRange; query: LibraryRequest }) =>
      useLibraryPages(7, props.query, props.range),
    { wrapper, initialProps: { range, query } },
  );
  return { ...hook, queryClient };
}

const filled = (slots: readonly (ItemRef | undefined)[]) =>
  slots.flatMap((slot, index) => (slot ? [index] : []));

/** The first and last index of each run of loaded places. */
const runs = (slots: readonly (ItemRef | undefined)[]) => {
  const out: [number, number][] = [];
  for (const index of filled(slots)) {
    const last = out.at(-1);
    if (last && last[1] === index - 1) last[1] = index;
    else out.push([index, index]);
  }
  return out;
};

describe('useLibraryPages', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('reads page 0 and leaves the places of the other pages empty', async () => {
    const server = library(450);
    const { result } = render({ start: 0, end: 59 });
    await waitFor(() => expect(result.current.slots).toHaveLength(450));
    expect(runs(result.current.slots)).toEqual([[0, 99]]);
    expect(result.current.slots[42]).toEqual(item(42));
    expect(result.current.first.data?.total).toBe(450);
    // The range lies in page 0: no other page asked for, also after the quiet time.
    await new Promise((resolve) => setTimeout(resolve, 150));
    expect(server.offsets()).toEqual([0]);
  });

  it('a jump into the middle reads only its pages, scrolling up the one above', async () => {
    const server = library(450);
    const { result, rerender } = render({ start: 0, end: 59 });
    await waitFor(() => expect(result.current.slots).toHaveLength(450));
    rerender({ range: { start: 250, end: 320 }, query: {} });
    await waitFor(() =>
      expect(runs(result.current.slots)).toEqual([
        [0, 99],
        [200, 399],
      ]),
    );
    expect(server.offsets()).toEqual([0, 200, 300]);
    // Up: the page above is read; the page below left the range and its places with it.
    rerender({ range: { start: 150, end: 220 }, query: {} });
    await waitFor(() => expect(runs(result.current.slots)).toEqual([[0, 299]]));
    expect(server.offsets()).toEqual([0, 200, 300, 100]);
    // The last page holds the rest of the list.
    rerender({ range: { start: 440, end: 520 }, query: {} });
    await waitFor(() =>
      expect(runs(result.current.slots)).toEqual([
        [0, 99],
        [400, 449],
      ]),
    );
  });

  it('waits for the quiet time: a range passed through quickly asks for nothing', async () => {
    const server = library(900);
    const { result, rerender } = render({ start: 0, end: 59 });
    await waitFor(() => expect(result.current.slots).toHaveLength(900));
    for (const start of [100, 200, 300, 400, 500]) {
      rerender({ range: { start, end: start + 60 }, query: {} });
      await new Promise((resolve) => setTimeout(resolve, 20));
    }
    await waitFor(() => expect(server.offsets()).toEqual([0, 500]));
  });

  it('asks for no other page while page 0 of a changed view is a placeholder', async () => {
    const server = library(450);
    const { result, rerender } = render({ start: 210, end: 290 });
    await waitFor(() =>
      expect(runs(result.current.slots)).toEqual([
        [0, 99],
        [200, 299],
      ]),
    );
    server.hold(0);
    rerender({ range: { start: 210, end: 290 }, query: { genre: 'Krimi' } });
    await new Promise((resolve) => setTimeout(resolve, 200));
    // Only page 0 of the new view is on its way; the previous view's places stay drawn.
    expect(server.offsets('Krimi')).toEqual([0]);
    expect(result.current.first.isPlaceholderData).toBe(true);
    expect(runs(result.current.slots)).toEqual([
      [0, 99],
      [200, 299],
    ]);
    act(() => server.release(0));
    await waitFor(() => expect(result.current.slots).toHaveLength(225));
    await waitFor(() => expect(server.offsets('Krimi')).toEqual([0, 200]));
    await waitFor(() =>
      expect(runs(result.current.slots)).toEqual([
        [0, 99],
        [200, 224],
      ]),
    );
  });

  it('another total on a page reads page 0 again and drops the other pages', async () => {
    const server = library(450);
    const { result, rerender } = render({ start: 0, end: 59 });
    await waitFor(() => expect(result.current.slots).toHaveLength(450));
    // A title was added after page 0 was read.
    server.state.total = 451;
    rerender({ range: { start: 250, end: 280 }, query: {} });
    await waitFor(() => expect(result.current.slots).toHaveLength(451));
    await waitFor(() =>
      expect(runs(result.current.slots)).toEqual([
        [0, 99],
        [200, 299],
      ]),
    );
    expect(server.offsets()).toEqual([0, 200, 0, 200]);
  });

  it('cancels a page that leaves the range on its way', async () => {
    const server = library(900);
    const { result, rerender } = render({ start: 0, end: 59 });
    await waitFor(() => expect(result.current.slots).toHaveLength(900));
    server.hold(500);
    rerender({ range: { start: 510, end: 560 }, query: {} });
    await waitFor(() => expect(server.offsets()).toEqual([0, 500]));
    const pending = server.asked.at(-1)!;
    expect(pending.signal.aborted).toBe(false);
    rerender({ range: { start: 0, end: 59 }, query: {} });
    await waitFor(() => expect(pending.signal.aborted).toBe(true));
    expect(filled(result.current.slots)).toHaveLength(100);
  });

  it('a failed page is reported; drawing it again asks once more', async () => {
    const server = library(450);
    const { result, rerender, queryClient } = render({ start: 0, end: 59 });
    await waitFor(() => expect(result.current.slots).toHaveLength(450));
    const fetchMock = vi.mocked(fetch);
    fetchMock.mockImplementationOnce(() => Promise.resolve(Response.json({}, { status: 500 })));
    rerender({ range: { start: 120, end: 160 }, query: {} });
    await waitFor(() => expect(result.current.failed).toBe(true));
    expect(server.offsets()).toEqual([0]);
    rerender({ range: { start: 0, end: 59 }, query: {} });
    rerender({ range: { start: 120, end: 160 }, query: {} });
    await waitFor(() => expect(runs(result.current.slots)).toEqual([[0, 199]]));
    expect(result.current.failed).toBe(false);
    expect(queryClient.getQueryState(libraryPageQuery(7, {}, 1).queryKey)?.status).toBe('success');
  });

  it('an invalidation of the library reads page 0 and the drawn pages again', async () => {
    const server = library(450);
    const { result, queryClient } = render({ start: 250, end: 280 });
    await waitFor(() =>
      expect(runs(result.current.slots)).toEqual([
        [0, 99],
        [200, 299],
      ]),
    );
    await act(() => queryClient.invalidateQueries({ queryKey: ['library'] }));
    await waitFor(() => expect(server.offsets()).toEqual([0, 200, 0, 200]));
  });

  it('the watchlist patch reaches a place of a later page', async () => {
    library(450);
    const { result, queryClient } = render({ start: 250, end: 280 });
    await waitFor(() => expect(result.current.slots[260]).toBeDefined());
    const key = libraryPageQuery(7, {}, 2).queryKey;
    act(() => {
      queryClient.setQueryData(key, (data) => patchWatchlist(data, 't260', true) as typeof data);
    });
    await waitFor(() => expect(result.current.slots[260]?.inWatchlist).toBe(true));
    expect(result.current.slots[261]?.inWatchlist).toBe(false);
  });
});
