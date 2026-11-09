import {
  hashKey,
  queryOptions,
  useQueries,
  useQuery,
  useQueryClient,
  type QueryKey,
  type UseQueryResult,
} from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';

// The page model with gaps (concept 17 section 6, D-6), freed from the library grid for any list
// operation with offset, limit and total (concept 18 C-3): page 0 carries the total (and whatever
// else the operation puts on its first page); any other page is read only while the range of the
// list asks for one of its places, so a long list can be scrolled or jumped into the middle and
// leave the places in between empty. The grids and tables draw the places (VirtualList).

/** Entries per page: the contract's maximum of the list operations (concept 18 C-3). */
export const PAGED_LIST_PAGE = 100;

/** Quiet time after the range changed before its missing pages are asked for. */
export const PAGED_LIST_SETTLE_MS = 100;

/** Pages other than 0 on their way at once, at most (concept 18 C-3). */
export const PAGED_LIST_PARALLEL = 2;

/** How long a page other than page 0 counts as fresh; page 0 checks them (`total`). */
const PAGE_STALE_MS = 30_000;

/** A list operation with offset, limit and total. */
export interface PagedSource<TPage, TItem> {
  /** The list's key; page `n` is read under `[...key, 'page', n]`. */
  key: QueryKey;
  /** Reads `limit` entries from `offset`. */
  fetch: (offset: number, limit: number, signal: AbortSignal) => Promise<TPage>;
  items: (page: TPage) => readonly TItem[];
  total: (page: TPage) => number;
  /** The entry's identity: no entry is drawn twice (concept 18 C-3). */
  id: (item: TItem) => string | number;
  /** Entries per page (PAGED_LIST_PAGE). */
  pageSize?: number;
}

/** Page `page` of a list. */
export const pagedListQuery = <TPage, TItem>(source: PagedSource<TPage, TItem>, page: number) => {
  const size = source.pageSize ?? PAGED_LIST_PAGE;
  return queryOptions({
    queryKey: [...source.key, 'page', page],
    queryFn: ({ signal }): Promise<TPage> => source.fetch(page * size, size, signal),
  });
};

/** The places of the list the range covers: `start` to `end`, both included (indices). */
export interface PlaceRange {
  start: number;
  end: number;
}

/** Every place of a list of `total` entries (a list drawn whole). */
export const wholeRange = (total: number): PlaceRange => ({ start: 0, end: total - 1 });

/** Lists of more entries than this are drawn virtually (concept 18 C-3); others are read whole. */
export const PAGED_LIST_VIRTUAL_FROM = 200;

/**
 * The range of a list that is drawn whole up to PAGED_LIST_VIRTUAL_FROM entries and virtually
 * above: every place, or the places the drawn rows ask for (`span`).
 */
export const rangeBySize =
  (span: PlaceRange) =>
  (total: number): PlaceRange =>
    total > PAGED_LIST_VIRTUAL_FROM ? span : wholeRange(total);

export interface PagedList<TPage, TItem> {
  /** Page 0; while a changed view loads, the previous view's (placeholder). */
  first: UseQueryResult<TPage>;
  /** The number of entries, from page 0 (0 before it arrived). */
  total: number;
  /** One place per entry of the list, undefined while its page is not there. */
  slots: readonly (TItem | undefined)[];
  /** The places from the first on that are all there (a list that grows at its end). */
  loaded: number;
  /**
   * A page of the range failed, also while it is read again (a new read starts without the
   * error); asking for it again reads it once more, so does `retry`.
   */
  failed: boolean;
  /** Reads the failed pages of the range again. */
  retry: () => void;
}

/** The pages other than 0 that hold places of `range`, in order. */
function pagesOf(range: PlaceRange, total: number, size: number): number[] {
  const last = Math.min(range.end, total - 1);
  const out: number[] = [];
  if (last < range.start) return out;
  for (let page = Math.max(1, Math.floor(range.start / size)); page <= Math.floor(last / size);) {
    out.push(page);
    page += 1;
  }
  return out;
}

interface Options {
  /** Read the list at all (a closed dialog, a view without its id yet). */
  enabled?: boolean;
  /** Page 0 asks again when the window gets the focus back (react-query's default: yes). */
  refetchOnWindowFocus?: boolean;
  /** When a failed page is read again by itself (react-query `retry`; the client's default). */
  retry?: (count: number, error: unknown) => boolean;
  retryDelay?: (count: number, error: unknown) => number;
}

/**
 * The entries of a list as places with gaps. Page 0 always; any other page while `range` holds
 * one of its places, asked for once the range rested for PAGED_LIST_SETTLE_MS, only after page 0
 * of this view arrived (its offsets belong to the view) and at most PAGED_LIST_PARALLEL at once,
 * in the order of the range. A page that leaves the range while on its way is cancelled (its
 * signal). Should a page report another total than page 0 or hold an entry of another place, the
 * list changed meanwhile: page 0 is read again and the other pages are dropped. While a changed
 * view loads (another key), the previous view's places stay.
 */
export function usePagedList<TPage, TItem>(
  source: PagedSource<TPage, TItem>,
  range: PlaceRange | ((total: number) => PlaceRange),
  { enabled = true, refetchOnWindowFocus, retry, retryDelay }: Options = {},
): PagedList<TPage, TItem> {
  const queryClient = useQueryClient();
  const size = source.pageSize ?? PAGED_LIST_PAGE;
  const first = useQuery({
    ...pagedListQuery(source, 0),
    enabled,
    refetchOnWindowFocus,
    ...(retry ? { retry } : {}),
    ...(retryDelay ? { retryDelay } : {}),
    // A changed sort or filter keeps the previous list until the new one arrives (no jump).
    placeholderData: (previous) => previous,
  });
  const ready = first.isSuccess && !first.isPlaceholderData;
  // The view whose places are drawn: this one once its page 0 is there, the previous one before.
  const hash = hashKey(source.key);
  const [view, setView] = useState({ hash, source });
  if (ready && view.hash !== hash) setView({ hash, source });
  const shown = ready ? source : view.source;
  const total = first.data === undefined ? 0 : source.total(first.data);

  const drawn = pagesOf(typeof range === 'function' ? range(total) : range, total, size);
  const drawnKey = drawn.join(',');
  const [settled, setSettled] = useState('');
  useEffect(() => {
    const timer = setTimeout(() => setSettled(drawnKey), PAGED_LIST_SETTLE_MS);
    return () => clearTimeout(timer);
  }, [drawnKey]);
  const asked = new Set(settled.split(','));

  // At most PAGED_LIST_PARALLEL pages without data on their way: the ones already running, then
  // the next missing ones in the order of the range. A page with data (or a failed one, which does
  // not read again by itself) always takes part.
  const allowed = new Set<number>();
  if (ready) {
    const states = drawn.map(
      (page) => [page, queryClient.getQueryState(pagedListQuery(shown, page).queryKey)] as const,
    );
    let running = 0;
    for (const [page, state] of states) {
      if (
        state?.data !== undefined ||
        (state?.status === 'error' && state.fetchStatus !== 'fetching')
      ) {
        allowed.add(page);
      } else if (state?.fetchStatus === 'fetching') {
        allowed.add(page);
        running += 1;
      }
    }
    for (const [page] of states) {
      if (running >= PAGED_LIST_PARALLEL) break;
      if (allowed.has(page) || !asked.has(String(page))) continue;
      allowed.add(page);
      running += 1;
    }
  }

  const pages = useQueries({
    queries: drawn.map((page) => ({
      ...pagedListQuery(shown, page),
      enabled: enabled && ready && asked.has(String(page)) && allowed.has(page),
      staleTime: PAGE_STALE_MS,
      refetchOnWindowFocus,
      ...(retry ? { retry } : {}),
      ...(retryDelay ? { retryDelay } : {}),
    })),
    combine: (results) => ({
      data: results.map((result) => result.data),
      updated: results.map((result) => result.dataUpdatedAt),
      failed: results.some(
        (result) =>
          result.data === undefined &&
          (result.isError || (result.isFetching && result.errorUpdateCount > 0)),
      ),
      retry: () => {
        for (const result of results) {
          if (result.isError && result.data === undefined) void result.refetch();
        }
      },
    }),
  });

  // The places: page 0 first, then the other pages, the newest first. A page with another total,
  // or with an entry that already sits in another place (the list shifted between the reads, the
  // total stayed), is left out: no entry is drawn twice, and the page counts as out of date.
  const { items, total: totalOf, id } = source;
  const { slots, stale } = useMemo(() => {
    const out = new Array<TItem | undefined>(total).fill(undefined);
    const placed = new Set<string | number>();
    const place = (page: TPage, offset: number) =>
      items(page).forEach((item, index) => {
        if (offset + index >= total) return;
        out[offset + index] = item;
        placed.add(id(item));
      });
    if (first.data !== undefined && totalOf(first.data) === total) place(first.data, 0);
    const left: number[] = [];
    const order = pages.data
      .map((_, n) => n)
      .sort((a, b) => (pages.updated[b] ?? 0) - (pages.updated[a] ?? 0));
    for (const n of order) {
      const page = pages.data[n];
      if (page === undefined) continue;
      if (totalOf(page) !== total || items(page).some((item) => placed.has(id(item)))) {
        left.push(n);
      } else {
        place(page, (drawn[n] ?? 0) * size);
      }
    }
    return { slots: out, stale: left };
    // `drawn` follows `pages.data` (one result per drawn page); the accessors are the source's.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [first.data, pages.data, pages.updated, total, size]);

  // A page out of date: the list changed meanwhile. A page read after page 0 tells so of page 0:
  // it is read again and the other pages are dropped. A page read before it is out of date itself:
  // only that page is read again. Nothing while page 0 is on its way; its running read is never
  // cancelled, else a faster page would restart it over and over during a scan.
  const staleKey = stale.map((n) => drawn[n]).join(',');
  const newer = stale.some((n) => (pages.updated[n] ?? 0) > first.dataUpdatedAt);
  const sourceKey = source.key;
  useEffect(() => {
    if (!ready || !enabled || first.isFetching || staleKey === '') return;
    const prefix = [...sourceKey, 'page'];
    if (newer) {
      void queryClient.invalidateQueries(
        { queryKey: [...prefix, 0], exact: true },
        { cancelRefetch: false },
      );
    }
    const outdated = new Set(staleKey.split(',').map(Number));
    void queryClient.resetQueries({
      queryKey: prefix,
      predicate: (entry) => {
        const page = entry.queryKey[prefix.length];
        return page !== 0 && (newer || outdated.has(Number(page)));
      },
    });
    // The key's hash stands for the key.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled, first.isFetching, hash, newer, queryClient, ready, staleKey]);

  let loaded = 0;
  while (loaded < slots.length && slots[loaded] !== undefined) loaded += 1;

  return { first, total, slots, loaded, failed: pages.failed, retry: pages.retry };
}
