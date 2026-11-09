import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, render as draw, renderHook, screen, waitFor } from '@testing-library/react';
import { createElement, type ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import {
  PAGED_LIST_PAGE,
  PAGED_LIST_PARALLEL,
  PAGED_LIST_VIRTUAL_FROM,
  rangeBySize,
  usePagedList,
  wholeRange,
  type PagedSource,
  type PlaceRange,
} from './pagedList';

// The general page model (concept 18 C-3): the library's own tests (catalog/libraryPages.test.ts)
// cover the gaps, the quiet time, the changed total and the shifted pages; these cover what the
// other lists add: the cap on parallel pages (also when read again), the loaded prefix, the range
// by size and the pages kept in place while the list is read again after a change.

interface Page {
  entries: { id: number }[];
  total: number;
}

/** A list of `total` entries; every read is recorded and held until `release` is called. */
function list(total: number, { hold = false } = {}) {
  const asked: number[] = [];
  const running = { now: 0, most: 0 };
  const waiting: (() => void)[] = [];
  const source: PagedSource<Page, { id: number }> = {
    key: ['test', total],
    fetch: async (offset, limit) => {
      asked.push(offset);
      running.now += 1;
      running.most = Math.max(running.most, running.now);
      if (hold) await new Promise<void>((resolve) => waiting.push(resolve));
      running.now -= 1;
      return {
        total,
        entries: Array.from({ length: Math.max(0, Math.min(limit, total - offset)) }, (_, n) => ({
          id: offset + n,
        })),
      };
    },
    items: (page) => page.entries,
    total: (page) => page.total,
    id: (entry) => entry.id,
  };
  return {
    source,
    asked,
    running,
    release: () => waiting.splice(0).forEach((resolve) => resolve()),
  };
}

function render(
  source: PagedSource<Page, { id: number }>,
  range: PlaceRange | ((total: number) => PlaceRange),
) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return { ...renderHook(() => usePagedList(source, range), { wrapper }), queryClient };
}

/**
 * A list whose entries can be taken off (a watchlist): the entries' ids in order; `hold(offset)`
 * keeps the answers to that offset back until `release(offset)`. Every read is recorded with its
 * signal.
 */
function changing(count: number) {
  const ids = Array.from({ length: count }, (_, index) => index);
  const asked: { offset: number; signal: AbortSignal }[] = [];
  const holding = new Set<number>();
  const held = new Map<number, (() => void)[]>();
  const source: PagedSource<Page, { id: number }> = {
    key: ['changing'],
    fetch: async (offset, limit, signal) => {
      asked.push({ offset, signal });
      if (holding.has(offset)) {
        await new Promise<void>((resolve) =>
          held.set(offset, [...(held.get(offset) ?? []), resolve]),
        );
      }
      return {
        total: ids.length,
        entries: ids.slice(offset, offset + limit).map((id) => ({ id })),
      };
    },
    items: (page) => page.entries,
    total: (page) => page.total,
    id: (entry) => entry.id,
  };
  return {
    source,
    asked,
    offsets: () => asked.map((entry) => entry.offset),
    remove: (id: number) => ids.splice(ids.indexOf(id), 1),
    hold: (offset: number) => holding.add(offset),
    release: (offset: number) => {
      holding.delete(offset);
      for (const resolve of held.get(offset) ?? []) resolve();
      held.delete(offset);
    },
  };
}

const ids = (slots: readonly ({ id: number } | undefined)[]) =>
  slots.flatMap((slot) => (slot ? [slot.id] : []));

describe('usePagedList', () => {
  it('reads every page of a short list and counts the loaded places from the first', async () => {
    const server = list(150);
    const { result } = render(server.source, wholeRange(150));
    await waitFor(() => expect(result.current.loaded).toBe(150));
    expect(result.current.total).toBe(150);
    expect(server.asked).toEqual([0, PAGED_LIST_PAGE]);
    expect(result.current.slots.map((entry) => entry?.id)).toEqual(
      Array.from({ length: 150 }, (_, index) => index),
    );
  });

  it(`never has more than ${PAGED_LIST_PARALLEL} pages on their way`, async () => {
    const server = list(1_000, { hold: true });
    const { result } = render(server.source, { start: 0, end: 999 });
    await waitFor(() => expect(server.asked).toEqual([0]));
    server.release();
    await waitFor(() => expect(result.current.total).toBe(1_000));
    // Page 0 first; then the other pages in order, two at a time.
    await waitFor(() => expect(server.asked).toHaveLength(3));
    expect(server.asked).toEqual([0, 100, 200]);
    for (let round = 0; round < 6 && result.current.loaded < 1_000; round += 1) {
      await new Promise((resolve) => setTimeout(resolve, 30));
      expect(server.running.now).toBeLessThanOrEqual(PAGED_LIST_PARALLEL);
      server.release();
    }
    await waitFor(() => {
      server.release();
      expect(result.current.loaded).toBe(1_000);
    });
    expect(server.running.most).toBeLessThanOrEqual(PAGED_LIST_PARALLEL);
    expect([...server.asked].sort((a, b) => a - b)).toEqual(
      Array.from({ length: 10 }, (_, index) => index * 100),
    );
  });

  it('reads a long list only where the drawn rows ask (rangeBySize)', async () => {
    const server = list(PAGED_LIST_VIRTUAL_FROM + 300);
    const { result } = render(server.source, rangeBySize({ start: 320, end: 380 }));
    await waitFor(() => expect(result.current.slots[350]).toEqual({ id: 350 }));
    expect(server.asked).toEqual([0, 300]);
    expect(result.current.loaded).toBe(100);
    expect(result.current.slots[150]).toBeUndefined();
  });

  it('reads a list of up to 200 entries whole, whatever the drawn rows ask', () => {
    expect(rangeBySize({ start: 5, end: 9 })(PAGED_LIST_VIRTUAL_FROM)).toEqual({
      start: 0,
      end: PAGED_LIST_VIRTUAL_FROM - 1,
    });
    expect(rangeBySize({ start: 5, end: 9 })(PAGED_LIST_VIRTUAL_FROM + 1)).toEqual({
      start: 5,
      end: 9,
    });
  });

  it('keeps a page in its place while it is read again after the total changed', async () => {
    const server = changing(150);
    const { result, queryClient } = render(server.source, wholeRange(150));
    await waitFor(() => expect(result.current.loaded).toBe(150));
    // Entry 120 goes; page 0 answers the invalidation at once, page 1 later.
    server.remove(120);
    server.hold(PAGED_LIST_PAGE);
    act(() => void queryClient.invalidateQueries({ queryKey: ['changing'] }));
    await waitFor(() => expect(result.current.total).toBe(149));
    // Page 1 stays drawn, cut to the new total: no place before its end empties meanwhile.
    expect(result.current.slots[121]).toEqual({ id: 121 });
    expect(result.current.loaded).toBe(149);
    expect(new Set(ids(result.current.slots)).size).toBe(149);
    act(() => server.release(PAGED_LIST_PAGE));
    await waitFor(() => expect(result.current.slots[120]).toEqual({ id: 121 }));
    expect(result.current.loaded).toBe(149);
    // Page 1 was read twice, its second read neither cancelled nor asked for again.
    expect(server.offsets()).toEqual([0, 100, 0, 100]);
    expect(server.asked[3]!.signal.aborted).toBe(false);
  });

  it('a page whose entries moved up keeps the others in their places, none twice', async () => {
    const server = changing(250);
    const { result, queryClient } = render(server.source, wholeRange(250));
    await waitFor(() => expect(result.current.loaded).toBe(250));
    // Entry 10 goes: page 0 now ends with entry 100, page 1 (on its way) starts with it.
    server.remove(10);
    server.hold(PAGED_LIST_PAGE);
    server.hold(2 * PAGED_LIST_PAGE);
    act(() => void queryClient.invalidateQueries({ queryKey: ['changing'] }));
    await waitFor(() => expect(result.current.slots[99]).toEqual({ id: 100 }));
    const shown = ids(result.current.slots);
    expect(new Set(shown).size).toBe(shown.length);
    // The rest of page 1 closes up behind entry 100: every entry already where it will be.
    expect(result.current.slots[100]).toEqual({ id: 101 });
    expect(result.current.slots[198]).toEqual({ id: 199 });
    act(() => {
      server.release(PAGED_LIST_PAGE);
      server.release(2 * PAGED_LIST_PAGE);
    });
    await waitFor(() => expect(result.current.loaded).toBe(249));
    expect(ids(result.current.slots)).toEqual(
      Array.from({ length: 250 }, (_, n) => n).filter((n) => n !== 10),
    );
  });

  it(`reads at most ${PAGED_LIST_PARALLEL} pages at once also when the list is read again`, async () => {
    const server = list(1_000);
    const { result, queryClient } = render(server.source, { start: 0, end: 999 });
    await waitFor(() => expect(result.current.loaded).toBe(1_000));
    const held = list(1_000, { hold: true });
    // The same list, now held: every page asks again after the invalidation.
    server.source.fetch = held.source.fetch;
    act(() => void queryClient.invalidateQueries({ queryKey: server.source.key }));
    await waitFor(() => expect(held.asked).toContain(0));
    for (let round = 0; round < 20 && held.asked.length < 10; round += 1) {
      await new Promise((resolve) => setTimeout(resolve, 20));
      expect(held.running.now).toBeLessThanOrEqual(PAGED_LIST_PARALLEL + 1);
      held.release();
    }
    await waitFor(() => {
      held.release();
      expect(held.asked).toHaveLength(10);
    });
    // Page 0 is read on its own; the others never more than two at a time.
    expect(held.running.most).toBeLessThanOrEqual(PAGED_LIST_PARALLEL + 1);
    expect(result.current.loaded).toBe(1_000);
  });

  it('focus stays after removing on page 2', async () => {
    const server = changing(150);
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    // The entries as a list of buttons (as ItemGrid draws a short list).
    function Entries() {
      const paged = usePagedList(server.source, wholeRange);
      return createElement(
        'ul',
        null,
        paged.slots.flatMap((slot) =>
          slot
            ? [
                createElement(
                  'li',
                  { key: slot.id },
                  createElement('button', null, `Eintrag ${slot.id}`),
                ),
              ]
            : [],
        ),
      );
    }
    draw(createElement(QueryClientProvider, { client: queryClient }, createElement(Entries)));
    const neighbour = await screen.findByRole('button', { name: 'Eintrag 121' });
    // «Entfernen» on entry 120 hands the focus to its neighbour, then the list is read again.
    neighbour.focus();
    server.remove(120);
    server.hold(PAGED_LIST_PAGE);
    act(() => void queryClient.invalidateQueries({ queryKey: ['changing'] }));
    await waitFor(() => expect(server.offsets()).toEqual([0, 100, 0, 100]));
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(neighbour).toHaveFocus();
    act(() => server.release(PAGED_LIST_PAGE));
    await waitFor(() => expect(screen.queryByRole('button', { name: 'Eintrag 120' })).toBeNull());
    expect(neighbour).toHaveFocus();
    expect(neighbour).toBeInTheDocument();
  });
});
