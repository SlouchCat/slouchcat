import type { components } from '@slouchcat/api-client';
import {
  queryOptions,
  useMutation,
  useQueryClient,
  type InfiniteData,
  type QueryClient,
} from '@tanstack/react-query';
import { api, request } from '@/api/client';
import type { PagedSource } from '@/lib/pagedList';

// Wishes through Seerr (concept 25 stage 2): the titles outside the library, the request dialog's
// seasons and limit, «Meine Wünsche» and its count. The server decides every right and every age
// (rule 12); the client shows what it answers.

export type ExternalMedia = components['schemas']['ExternalMedia'];
export type ExternalTitle = components['schemas']['ExternalTitle'];
export type ExternalTitleRef = components['schemas']['ExternalTitleRef'];
export type ExternalTitlePage = components['schemas']['ExternalTitlePage'];
export type ExternalTitleImages = components['schemas']['ExternalTitleImages'];
export type Wish = components['schemas']['Wish'];
export type WishPage = components['schemas']['WishPage'];
export type WishCreate = components['schemas']['WishCreate'];
export type WishState = components['schemas']['WishState'];
export type WishStateDetail = components['schemas']['WishStateDetail'];
export type WishFailReason = components['schemas']['WishFailReason'];
export type WishRequestReason = components['schemas']['WishRequestReason'];
export type WishProblem = components['schemas']['WishProblem'];
export type WishQuota = components['schemas']['WishQuota'];
export type WishSeasons = components['schemas']['WishSeasons'];
export type WishEvent = components['schemas']['WishEvent'];
export type WishProgress = components['schemas']['WishProgress'];
export type WishStateFilter = 'open' | 'available' | 'closed';
type DiscoverEntry = components['schemas']['DiscoverEntry'];
type DiscoverPage = components['schemas']['DiscoverPage'];
type WishDiscover = components['schemas']['WishDiscover'];

/** Every query of this feature lives under one of these, so a wish refreshes them all. */
export const wishKeys = {
  /** «Meine Wünsche», its count in the side navigation and the limit. */
  wishes: ['wishes'] as const,
  /** The preview pages, the search section and the seasons of the dialog. */
  titles: ['titles'] as const,
  /** The wishes area «Entdecken», «Filme», «Serien», «Suche»: rows and grids of many pages. */
  discover: ['titles', 'discover'] as const,
};

/** The preview page of a title outside the library (getExternalTitle). */
export const externalTitleQuery = (media: ExternalMedia, tmdbId: number) =>
  queryOptions({
    queryKey: [...wishKeys.titles, media, tmdbId],
    queryFn: ({ signal }): Promise<ExternalTitle> =>
      request(api.GET('/titles/{media}/{tmdbId}', { params: { path: { media, tmdbId } }, signal })),
  });

/** The search section «Noch nicht in der Bibliothek» (searchExternalTitles). */
export const externalSearchQuery = (q: string, media?: ExternalMedia) =>
  queryOptions({
    queryKey: [...wishKeys.titles, 'search', q, media ?? 'all'],
    queryFn: ({ signal }) =>
      request(
        api.GET('/titles/search', {
          params: { query: { q, ...(media ? { media } : {}) } },
          signal,
        }),
      ),
    staleTime: 60_000,
  });

/** The seasons the request dialog offers (getWishSeasons). */
export const wishSeasonsQuery = (tmdbId: number) =>
  queryOptions({
    queryKey: [...wishKeys.titles, 'show', tmdbId, 'seasons'],
    queryFn: ({ signal }): Promise<WishSeasons> =>
      request(api.GET('/titles/show/{tmdbId}/seasons', { params: { path: { tmdbId } }, signal })),
  });

/** The limit of the Seerr user the profile sends as (getWishQuota). */
export const wishQuotaQuery = queryOptions({
  queryKey: [...wishKeys.wishes, 'quota'],
  queryFn: ({ signal }): Promise<WishQuota> => request(api.GET('/wishes/quota', { signal })),
});

/** «Meine Wünsche», paged as the list scrolls (concept 18 C-3); page 0 carries the counts. */
export const wishesSource = (state: WishStateFilter | undefined): PagedSource<WishPage, Wish> => ({
  key: [...wishKeys.wishes, 'list', state ?? 'all'],
  fetch: (offset, limit, signal) =>
    request(
      api.GET('/wishes', {
        params: { query: { offset, limit, ...(state ? { state } : {}) } },
        signal,
      }),
    ),
  items: (page) => page.items,
  total: (page) => page.total,
  id: (wish) => wish.id,
});

/**
 * The counts of the profile's wishes (WishPage.counts) for the side navigation's entry: one entry
 * read, every page that shows a count shares it.
 */
export const wishCountsQuery = queryOptions({
  queryKey: [...wishKeys.wishes, 'counts'],
  queryFn: async ({ signal }) =>
    (await request(api.GET('/wishes', { params: { query: { limit: 1 } }, signal }))).counts,
  staleTime: 30_000,
});

/** Changes the entries of the wishes area held in the cache: its rows and every grid page. */
function patchDiscover(queryClient: QueryClient, change: (entry: DiscoverEntry) => DiscoverEntry) {
  const entries = (list: DiscoverEntry[]) => {
    const next = list.map(change);
    return next.some((entry, index) => entry !== list[index]) ? next : list;
  };
  queryClient.setQueriesData<WishDiscover>(
    { queryKey: [...wishKeys.discover, 'rows'] },
    (data) =>
      data && {
        ...data,
        rows: data.rows.map((row) => ({ ...row, entries: entries(row.entries) })),
      },
  );
  for (const kind of ['titles', 'search']) {
    queryClient.setQueriesData<InfiniteData<DiscoverPage>>(
      { queryKey: [...wishKeys.discover, kind] },
      (data) =>
        data && {
          ...data,
          pages: data.pages.map((page) => ({ ...page, entries: entries(page.entries) })),
        },
    );
  }
}

/**
 * After a wish changed: every list, count, preview and search section that shows it. The rows and
 * grids of the wishes area hold many of Seerr's pages: their entry is changed in the cache and they
 * are only marked stale, so a wish does not read every page again (each counts against Seerr's
 * calls per minute); they read again when the page opens anew.
 */
function useWishSettled() {
  const queryClient = useQueryClient();
  return () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: wishKeys.wishes }),
      queryClient.invalidateQueries({
        queryKey: wishKeys.titles,
        predicate: (query) => query.queryKey[1] !== wishKeys.discover[1],
      }),
      queryClient.invalidateQueries({ queryKey: wishKeys.discover, refetchType: 'none' }),
    ]);
}

/**
 * Requests a title, or joins the open wish of another profile (createWish). The callbacks given
 * here belong to the mutation, so they still run when the dialog that started it has closed
 * (DownloadDialog does the same).
 */
export function useCreateWish(callbacks?: {
  onSuccess?: (wish: Wish) => void;
  onError?: (error: Error) => void;
}) {
  const queryClient = useQueryClient();
  const settled = useWishSettled();
  return useMutation({
    mutationFn: (body: WishCreate): Promise<Wish> => request(api.POST('/wishes', { body })),
    onSuccess: (wish) => {
      // The tile of the title shows the new state at once (no plus any more).
      patchDiscover(queryClient, (entry) =>
        entry.item || entry.title.media !== wish.media || entry.title.tmdbId !== wish.tmdbId
          ? entry
          : {
              ...entry,
              title: {
                ...entry.title,
                wish: { id: wish.id, state: wish.state, stateDetail: wish.stateDetail },
              },
            },
      );
      callbacks?.onSuccess?.(wish);
    },
    onError: callbacks?.onError,
    onSettled: settled,
  });
}

/** Withdraws a wish, or only the profile's part of a joined one (withdrawWish). */
export function useWithdrawWish() {
  const queryClient = useQueryClient();
  const settled = useWishSettled();
  return useMutation({
    mutationFn: (id: number) => request(api.DELETE('/wishes/{id}', { params: { path: { id } } })),
    onSuccess: (_, id) =>
      patchDiscover(queryClient, (entry) =>
        entry.title.wish?.id === id
          ? { ...entry, title: { ...entry.title, wish: undefined } }
          : entry,
      ),
    onSettled: settled,
  });
}
