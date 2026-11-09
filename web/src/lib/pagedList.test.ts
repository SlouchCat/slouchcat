import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { renderHook, waitFor } from '@testing-library/react';
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
// other lists add: the cap on parallel pages, the loaded prefix and the range by size.

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
  return renderHook(() => usePagedList(source, range), { wrapper });
}

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
});
