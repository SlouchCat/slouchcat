import type { components, ItemRef } from '@slouchcat/api-client';
import {
  queryOptions,
  useQueries,
  useQuery,
  useQueryClient,
  type UseQueryResult,
} from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import { api, request } from '@/api/client';
import type { LibraryRequest } from './libraryParams';

// The library grid's pages with gaps (concept 17 section 6, D-6): page 0 carries the total, the
// facets and the letter starts; any other page is read only while its rows are drawn, so the grid
// can jump into the middle of a long list and leave the places in between empty.

type LibraryItemsPage = components['schemas']['LibraryItemsPage'];

/** Titles per page of the library grid: the contract's maximum. */
export const LIBRARY_GRID_PAGE = 100;

/** Quiet time after the drawn range changed before its missing pages are asked for. */
export const LIBRARY_PAGE_SETTLE_MS = 100;

/** How long a page other than page 0 counts as fresh; page 0 checks them (`total`). */
const PAGE_STALE_MS = 30_000;

/** Page `page` of a library view. The prefix `['library']` keeps the invalidations of the app. */
export const libraryPageQuery = (id: number, query: LibraryRequest, page: number) =>
  queryOptions({
    queryKey: ['library', id, query, 'page', page],
    queryFn: ({ signal }): Promise<LibraryItemsPage> =>
      request(
        api.GET('/libraries/{id}/items', {
          params: {
            path: { id },
            query: { ...query, offset: page * LIBRARY_GRID_PAGE, limit: LIBRARY_GRID_PAGE },
          },
          signal,
        }),
      ),
  });

/** The places of the list the grid draws: `start` to `end`, both included (title indices). */
export interface DrawnRange {
  start: number;
  end: number;
}

export interface LibraryPages {
  /** Page 0; while a changed view loads, the previous view's (placeholder). */
  first: UseQueryResult<LibraryItemsPage>;
  /** One place per title of the list, undefined while its page is not there. */
  slots: readonly (ItemRef | undefined)[];
  /** A page of the drawn range failed; drawing it again asks once more, so does `retry`. */
  failed: boolean;
  /** Reads the failed pages of the drawn range again. */
  retry: () => void;
}

/** The pages other than 0 that hold places of `range`, in order. */
function pagesOf(range: DrawnRange, total: number): number[] {
  const last = Math.min(range.end, total - 1);
  const out: number[] = [];
  if (last < range.start) return out;
  for (
    let page = Math.max(1, Math.floor(range.start / LIBRARY_GRID_PAGE));
    page <= Math.floor(last / LIBRARY_GRID_PAGE);
    page += 1
  ) {
    out.push(page);
  }
  return out;
}

/**
 * The titles of a library view as places with gaps. Page 0 always; any other page while `range`
 * holds one of its places, asked for once the range rested for LIBRARY_PAGE_SETTLE_MS and only
 * after page 0 of this view arrived (its offsets belong to the view). A page that leaves the range
 * while on its way is cancelled (its signal). Should a page report another total than page 0 or
 * hold a title of another place, the list changed meanwhile: page 0 is read again and the other
 * pages are dropped. While a changed
 * view loads, the previous view's places stay.
 */
export function useLibraryPages(
  id: number | null,
  query: LibraryRequest,
  range: DrawnRange,
): LibraryPages {
  const queryClient = useQueryClient();
  const first = useQuery({
    ...libraryPageQuery(id ?? 0, query, 0),
    enabled: id !== null,
    // A changed sort or filter keeps the previous grid until the new one arrives (no jump).
    placeholderData: (previous) => previous,
  });
  const ready = first.isSuccess && !first.isPlaceholderData;
  // The view whose places are drawn: this one once its page 0 is there, the previous one before.
  const [view, setView] = useState(query);
  if (ready && view !== query) setView(query);
  const shown = ready ? query : view;
  const total = first.data?.total ?? 0;

  const drawn = pagesOf(range, total);
  const drawnKey = drawn.join(',');
  const [settled, setSettled] = useState('');
  useEffect(() => {
    const timer = setTimeout(() => setSettled(drawnKey), LIBRARY_PAGE_SETTLE_MS);
    return () => clearTimeout(timer);
  }, [drawnKey]);
  const asked = new Set(settled.split(','));

  const pages = useQueries({
    queries: drawn.map((page) => ({
      ...libraryPageQuery(id ?? 0, shown, page),
      enabled: ready && asked.has(String(page)),
      staleTime: PAGE_STALE_MS,
    })),
    combine: (results) => ({
      data: results.map((result) => result.data),
      updated: results.map((result) => result.dataUpdatedAt),
      failed: results.some((result) => result.isError && result.data === undefined),
      retry: () => {
        for (const result of results) {
          if (result.isError && result.data === undefined) void result.refetch();
        }
      },
    }),
  });

  // The places: page 0 first, then the other pages, the newest first. A page with another total,
  // or with a title that already sits in another place (the list shifted between the reads, the
  // total stayed), is left out: no title is drawn twice, and the page counts as out of date.
  const { slots, stale } = useMemo(() => {
    const out = new Array<ItemRef | undefined>(total).fill(undefined);
    const placed = new Set<string>();
    const place = (page: LibraryItemsPage) =>
      page.items.forEach((item, index) => {
        if (page.offset + index >= total) return;
        out[page.offset + index] = item;
        placed.add(item.id);
      });
    if (first.data?.total === total) place(first.data);
    const left: number[] = [];
    const order = pages.data
      .map((_, n) => n)
      .sort((a, b) => (pages.updated[b] ?? 0) - (pages.updated[a] ?? 0));
    for (const n of order) {
      const page = pages.data[n];
      if (page === undefined) continue;
      if (page.total !== total || page.items.some((item) => placed.has(item.id))) {
        left.push(n);
      } else {
        place(page);
      }
    }
    return { slots: out, stale: left };
  }, [first.data, pages.data, pages.updated, total]);

  // A page out of date: the list changed meanwhile. A page read after page 0 tells so of page 0:
  // it is read again and the other pages are dropped. A page read before it is out of date itself:
  // only that page is read again. Nothing while page 0 is on its way; its running read is never
  // cancelled, else a faster page would restart it over and over during a scan.
  const staleKey = stale.map((n) => drawn[n]).join(',');
  const newer = stale.some((n) => (pages.updated[n] ?? 0) > first.dataUpdatedAt);
  useEffect(() => {
    if (!ready || id === null || first.isFetching || staleKey === '') return;
    const base = libraryPageQuery(id, query, 0).queryKey;
    if (newer) {
      void queryClient.invalidateQueries({ queryKey: base, exact: true }, { cancelRefetch: false });
    }
    const outdated = new Set(staleKey.split(',').map(Number));
    void queryClient.resetQueries({
      queryKey: base.slice(0, 4),
      predicate: (entry) =>
        entry.queryKey[4] !== 0 && (newer || outdated.has(Number(entry.queryKey[4]))),
    });
  }, [first.isFetching, id, newer, query, queryClient, ready, staleKey]);

  return { first, slots, failed: pages.failed, retry: pages.retry };
}
