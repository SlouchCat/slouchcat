import type {
  Collection,
  GenreRef,
  ItemPage,
  NotificationPage,
  SearchRequest,
  SearchResult,
} from '@slouchcat/api-client';
import {
  infiniteQueryOptions,
  queryOptions,
  useMutation,
  useQueryClient,
  type QueryKey,
} from '@tanstack/react-query';
import { api, request } from '@/api/client';

/** Page size of "Mehr laden" on the search, watchlist, collection and notification pages. */
export const PAGE_SIZE = 50;

const nextOffset = (last: { offset: number; total: number }, count: number) => {
  const next = last.offset + count;
  return next < last.total && count > 0 ? next : undefined;
};

/** Search (E-4): titles paged; people and collections come with the first page. */
export const searchQuery = (body: Omit<SearchRequest, 'offset' | 'limit'>) =>
  infiniteQueryOptions({
    queryKey: ['search', body],
    queryFn: ({ pageParam, signal }): Promise<SearchResult> =>
      request(
        api.POST('/search', { body: { ...body, offset: pageParam, limit: PAGE_SIZE }, signal }),
      ),
    initialPageParam: 0,
    getNextPageParam: (last) => nextOffset(last, last.items.length),
    // Typing refines the same search: keep the previous results until the new ones arrive.
    placeholderData: (previous) => previous,
    staleTime: 30_000,
  });

/** Genres with visible titles, for the search filter. */
export const genresQuery = queryOptions({
  queryKey: ['genres'],
  queryFn: ({ signal }): Promise<GenreRef[]> => request(api.GET('/genres', { signal })),
  staleTime: 5 * 60_000,
});

/** A collection with its visible members, paged (E-6). */
export const collectionQuery = (id: string) =>
  infiniteQueryOptions({
    queryKey: ['collection', id],
    queryFn: ({ pageParam, signal }): Promise<Collection> =>
      request(
        api.GET('/collections/{id}', {
          params: { path: { id }, query: { offset: pageParam, limit: PAGE_SIZE } },
          signal,
        }),
      ),
    initialPageParam: 0,
    getNextPageParam: (last) => nextOffset(last, last.items.length),
  });

/** The watchlist of the active profile, newest first (E-5). */
export const watchlistQuery = infiniteQueryOptions({
  queryKey: ['watchlist'],
  queryFn: ({ pageParam, signal }): Promise<ItemPage> =>
    request(
      api.GET('/watchlist', { params: { query: { offset: pageParam, limit: PAGE_SIZE } }, signal }),
    ),
  initialPageParam: 0,
  getNextPageParam: (last) => nextOffset(last, last.items.length),
});

/** The inbox page (E-8), newest first. */
export const notificationsPageQuery = infiniteQueryOptions({
  queryKey: ['notifications', 'page'],
  queryFn: ({ pageParam, signal }): Promise<NotificationPage> =>
    request(
      api.GET('/notifications', {
        params: { query: { offset: pageParam, limit: PAGE_SIZE } },
        signal,
      }),
    ),
  initialPageParam: 0,
  getNextPageParam: (last) => nextOffset(last, last.notifications.length),
});

/**
 * The unread count for the header badge: one entry is enough, `unread` counts them all. Asked
 * every 30 s while the app is visible (no push channel in M7, design: no SSE).
 */
export const notificationsQuery = queryOptions({
  queryKey: ['notifications', 'badge'],
  queryFn: ({ signal }): Promise<NotificationPage> =>
    request(api.GET('/notifications', { params: { query: { limit: 1 } }, signal })),
  refetchInterval: 30_000,
  staleTime: 10_000,
});

const isPlainObject = (value: unknown): value is Record<string, unknown> => {
  if (typeof value !== 'object' || value === null) return false;
  const prototype = Object.getPrototypeOf(value) as unknown;
  return prototype === Object.prototype || prototype === null;
};

function walkWatchlist(value: unknown, id: string, on: boolean): unknown {
  if (Array.isArray(value)) {
    let changed = false;
    const next = value.map((entry: unknown) => {
      const patched = walkWatchlist(entry, id, on);
      if (patched !== entry) changed = true;
      return patched;
    });
    return changed ? next : value;
  }
  if (!isPlainObject(value)) return value;
  let next: Record<string, unknown> | undefined;
  for (const [key, entry] of Object.entries(value)) {
    const patched = walkWatchlist(entry, id, on);
    if (patched !== entry) {
      next ??= { ...value };
      next[key] = patched;
    }
  }
  if (value.id === id && typeof value.inWatchlist === 'boolean' && value.inWatchlist !== on) {
    next ??= { ...value };
    next.inWatchlist = on;
  }
  return next ?? value;
}

/**
 * Sets `inWatchlist` of title `id` to `on` wherever it stands in a query's data (concept 17 D-9,
 * plan E-18): a walk over plain objects and arrays only, which keeps every untouched subtree as the
 * same object (structural sharing). Undefined when nothing changed, so `setQueryData` leaves such a
 * query alone instead of marking it fresh.
 */
export function patchWatchlist(data: unknown, id: string, on: boolean): unknown {
  const next = walkWatchlist(data, id, on);
  return next === data ? undefined : next;
}

/**
 * Adds a title to or removes it from the watchlist (E-5). The new state stands at once in every
 * cached list and page that shows the title (only those queries are cancelled and patched); an
 * error turns the same queries back. Afterwards only the watchlist and the start page are read
 * again: the library and the detail keep their patched data (concept 17 8.2).
 */
export function useWatchlistMutation() {
  const queryClient = useQueryClient();
  const patch = (keys: readonly QueryKey[], id: string, on: boolean) => {
    for (const queryKey of keys) {
      queryClient.setQueryData(queryKey, (data: unknown) => patchWatchlist(data, id, on));
    }
  };
  return useMutation({
    mutationFn: ({ id, on }: { id: string; on: boolean }) =>
      request(
        on
          ? api.PUT('/items/{id}/watchlist', { params: { path: { id } } })
          : api.DELETE('/items/{id}/watchlist', { params: { path: { id } } }),
      ),
    onMutate: async ({ id, on }) => {
      const keys = queryClient
        .getQueryCache()
        .findAll()
        .filter((query) => patchWatchlist(query.state.data, id, on) !== undefined)
        .map((query) => query.queryKey);
      await Promise.all(
        keys.map((queryKey) => queryClient.cancelQueries({ queryKey, exact: true })),
      );
      patch(keys, id, on);
      return { keys };
    },
    onError: (_error, { id, on }, context) => {
      if (context) patch(context.keys, id, !on);
    },
    onSettled: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: ['watchlist'] }),
        queryClient.invalidateQueries({ queryKey: ['home'] }),
      ]),
  });
}

/** Marks a series or podcast as favourite or not (E-8: new episodes notify). */
export function useFavoriteMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, on }: { id: string; on: boolean }) =>
      request(
        on
          ? api.PUT('/items/{id}/favorite', { params: { path: { id } } })
          : api.DELETE('/items/{id}/favorite', { params: { path: { id } } }),
      ),
    onSettled: (_data, _error, { id }) =>
      queryClient.invalidateQueries({ queryKey: ['item', id], exact: true }),
  });
}

/** Marks one notification read. */
export function useMarkReadMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: number) =>
      request(api.PUT('/notifications/{id}/read', { params: { path: { id } } })),
    onSettled: () => queryClient.invalidateQueries({ queryKey: ['notifications'] }),
  });
}

/** Marks every notification of the active profile read. */
export function useMarkAllReadMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => request(api.POST('/notifications/read-all')),
    onSettled: () => queryClient.invalidateQueries({ queryKey: ['notifications'] }),
  });
}
