import type { components } from '@slouchcat/api-client';
import { infiniteQueryOptions, queryOptions } from '@tanstack/react-query';
import { api, request } from '@/api/client';
import { wishKeys, type ExternalMedia } from '../queries';

// The wishes area «Entdecken», «Filme», «Serien» and «Suche» (concept 25 stage 5). Every list is
// read through the server, which asks Seerr on behalf of the profile and decides right and age
// (rule 12); the pages show what it answers. The lists live under `wishKeys.discover`: a new wish
// changes the entry in the cache and marks them stale (useWishSettled), without reading every
// page again.

export type WishDiscover = components['schemas']['WishDiscover'];
export type DiscoverRow = components['schemas']['DiscoverRow'];
export type DiscoverRowKey = components['schemas']['DiscoverRowKey'];
export type DiscoverEntry = components['schemas']['DiscoverEntry'];
export type DiscoverGenreTile = components['schemas']['DiscoverGenreTile'];
export type DiscoverPage = components['schemas']['DiscoverPage'];
export type WishTitlesSort = 'popularity' | 'newest' | 'rating';

const discoverKey = wishKeys.discover;

/** «Entdecken»: the seven rows with their first page (getWishDiscover). */
export const wishDiscoverQuery = queryOptions({
  queryKey: [...discoverKey, 'rows'],
  queryFn: ({ signal }): Promise<WishDiscover> => request(api.GET('/wishes/discover', { signal })),
  staleTime: 60_000,
});

/** The next page of a list: Seerr's pages, at most 500; a page may hold fewer entries. */
const nextPage = (last: DiscoverPage) =>
  last.totalPages !== undefined && last.page < last.totalPages ? last.page + 1 : undefined;

export interface WishTitlesRequest {
  sort?: WishTitlesSort;
  genre?: number;
  hideAvailable?: boolean;
}

/** «Filme» and «Serien», page by page as the grid scrolls (listWishTitles). */
export const wishTitlesQuery = (media: ExternalMedia, query: WishTitlesRequest) =>
  infiniteQueryOptions({
    queryKey: [...discoverKey, 'titles', media, query],
    queryFn: ({ pageParam, signal }): Promise<DiscoverPage> =>
      request(
        api.GET('/wishes/titles/{media}', {
          params: { path: { media }, query: { ...query, page: pageParam } },
          signal,
        }),
      ),
    initialPageParam: 1,
    getNextPageParam: nextPage,
    staleTime: 60_000,
    // A refetch reads every page loaded so far: not on a mere return to the window.
    refetchOnWindowFocus: false,
  });

/** «Suche» over Seerr, page by page (searchWishTitles). */
export const wishSearchQuery = (q: string) =>
  infiniteQueryOptions({
    queryKey: [...discoverKey, 'search', q],
    queryFn: ({ pageParam, signal }): Promise<DiscoverPage> =>
      request(api.GET('/wishes/search', { params: { query: { q, page: pageParam } }, signal })),
    initialPageParam: 1,
    getNextPageParam: nextPage,
    staleTime: 60_000,
    // A refetch reads every page loaded so far: not on a mere return to the window.
    refetchOnWindowFocus: false,
  });

/** The genres of the filter of «Filme» and «Serien» (listWishGenres). */
export const wishGenresQuery = (media: ExternalMedia) =>
  queryOptions({
    queryKey: [...discoverKey, 'genres', media],
    queryFn: ({ signal }) =>
      request(api.GET('/wishes/genres/{media}', { params: { path: { media } }, signal })),
    staleTime: 10 * 60_000,
  });

/** An entry's identity in a list: Seerr's pages may repeat a title when the list moved. */
export const entryId = (entry: DiscoverEntry) => `${entry.title.media}-${entry.title.tmdbId}`;

/** The entries of all pages read so far (or of one row), each title once. */
export function entriesOf(
  pages: readonly { entries: readonly DiscoverEntry[] }[] | undefined,
): DiscoverEntry[] {
  const seen = new Set<string>();
  const out: DiscoverEntry[] = [];
  for (const page of pages ?? []) {
    for (const entry of page.entries) {
      const id = entryId(entry);
      if (seen.has(id)) continue;
      seen.add(id);
      out.push(entry);
    }
  }
  return out;
}
