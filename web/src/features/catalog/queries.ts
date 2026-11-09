import type {
  components,
  HomePage,
  ItemDetail,
  ItemRef,
  Person,
  PodcastEpisodes,
  Trailer,
} from '@slouchcat/api-client';
import { infiniteQueryOptions, queryOptions } from '@tanstack/react-query';
import { api, request } from '@/api/client';
import type { PagedSource } from '@/lib/pagedList';

type LibraryNav = components['schemas']['LibraryNav'];
type PicksPage = components['schemas']['PicksPage'];

/**
 * The stage's start the app shell drew for this page view (concept 08, 6; api/preload.go):
 * `data-start` of `#hero-prepaint` or, without that image, the `slouchcat-hero-start` meta. Absent
 * when the shell came from the service worker's precache or the page has no stage.
 */
export function shellHeroStart(): number | undefined {
  const value =
    document.getElementById('hero-prepaint')?.dataset.start ??
    document.querySelector<HTMLMetaElement>('meta[name="slouchcat-hero-start"]')?.content;
  const start = value === undefined || value === '' ? NaN : Number(value);
  return Number.isInteger(start) && start >= 0 ? start : undefined;
}

export const homeQuery = queryOptions({
  queryKey: ['home'],
  // The first read of the session repeats the shell's draw (start=K), so it is the URL the shell
  // preloads and the band begins with the painted title; every later read lets the server draw.
  // The stage keeps the start it mounted with, so a refetch never moves the band (5.2).
  queryFn: ({ signal, client, queryKey }): Promise<HomePage> => {
    const start = client.getQueryState(queryKey)?.dataUpdatedAt ? undefined : shellHeroStart();
    return request(
      api.GET('/home', { params: start === undefined ? {} : { query: { start } }, signal }),
    );
  },
  // The router guard prefetches the start page; the page mounting right after must not ask again.
  // The player invalidates it after playback.
  staleTime: 30_000,
});

export const itemQuery = (id: string) =>
  queryOptions({
    queryKey: ['item', id],
    queryFn: ({ signal }): Promise<ItemDetail> =>
      request(api.GET('/items/{id}', { params: { path: { id } }, signal })),
  });

/** Trailers of a title, the profile language first (W-10). */
export const trailersQuery = (id: string) =>
  queryOptions({
    queryKey: ['item', id, 'trailers'],
    queryFn: ({ signal }): Promise<Trailer[]> =>
      request(api.GET('/items/{id}/trailers', { params: { path: { id } }, signal })),
  });

/** A person with the credits the active profile may see; 404 when none is visible. */
export const personQuery = (id: number) =>
  queryOptions({
    queryKey: ['person', id],
    queryFn: ({ signal }): Promise<Person> =>
      request(api.GET('/people/{id}', { params: { path: { id } }, signal })),
  });

/** The episodes of a podcast, newest first (listPodcastEpisodes, concept 18 C-3). */
export const podcastEpisodesSource = (id: string): PagedSource<PodcastEpisodes, ItemRef> => ({
  key: ['item', id, 'episodes'],
  fetch: (offset, limit, signal) =>
    request(
      api.GET('/items/{id}/episodes', {
        params: { path: { id }, query: { offset, limit } },
        signal,
      }),
    ),
  items: (page) => page.episodes,
  total: (page) => page.total,
  id: (episode) => episode.id,
});

/** Titles per page of the user picks page, read while scrolling: the contract's maximum. */
export const PICKS_PAGE = 100;

const nextOffset = (last: { offset: number; total: number; items: unknown[] }) => {
  const next = last.offset + last.items.length;
  return next < last.total && last.items.length > 0 ? next : undefined;
};

/** The libraries with at least one title the active profile may see (concept 08). */
export const visibleLibrariesQuery = queryOptions({
  queryKey: ['libraries', 'visible'],
  queryFn: ({ signal }): Promise<LibraryNav[]> =>
    request(api.GET('/libraries/visible', { signal })),
  staleTime: 60_000,
});

/** How many user picks the active profile sees: the side navigation's count. */
export const picksCountQuery = queryOptions({
  queryKey: ['picks', 'count'],
  queryFn: async ({ signal }): Promise<number> =>
    (await request(api.GET('/picks', { params: { query: { limit: 1 } }, signal }))).total,
  staleTime: 60_000,
});

/** The user picks the active profile sees, newest pick first (E-2, concept 18 part B), paged. */
export const picksPageQuery = infiniteQueryOptions({
  queryKey: ['picks', 'page'],
  queryFn: ({ pageParam, signal }): Promise<PicksPage> =>
    request(
      api.GET('/picks', {
        params: { query: { offset: pageParam, limit: PICKS_PAGE } },
        signal,
      }),
    ),
  initialPageParam: 0,
  getNextPageParam: nextOffset,
});
