import type { components } from '@slouchcat/api-client';
import {
  infiniteQueryOptions,
  queryOptions,
  useMutation,
  useQueryClient,
} from '@tanstack/react-query';
import { api, isApiError, request } from '@/api/client';
import { radioKeys } from '@/app/radio/queries';
import type { RadioQueryParams, RadioSort, RadioFilter } from './filters';

// The radio page's API calls (contract 1.17.0). Before the first catalog sync the list and the
// facets answer 503 `radio_catalog_loading`; the queries then ask again after `Retry-After`
// and the page says the catalog is loading meanwhile.

type RadioStation = components['schemas']['RadioStation'];
type RadioStationPage = components['schemas']['RadioStationPage'];
type RadioFacets = components['schemas']['RadioFacets'];
export type RadioPreferences = components['schemas']['RadioPreferences'];

/** One page of the grid. */
export const RADIO_PAGE_SIZE = 60;
/** The default wait when the server names none. */
const CATALOG_RETRY_MS = 30_000;

/** The catalog is not synced yet (503 `radio_catalog_loading`). */
export const isCatalogLoading = (error: unknown) =>
  isApiError(error, 503) && error.code === 'radio_catalog_loading';

/** Ask again while the catalog loads, after the time the server named; nothing else retries. */
const catalogRetry = {
  retry: (_count: number, error: unknown) => isCatalogLoading(error),
  retryDelay: (_count: number, error: unknown) =>
    (isApiError(error) ? error.retryAfterMs : null) ?? CATALOG_RETRY_MS,
};

export const radioStationsQuery = (params: RadioQueryParams, limit = RADIO_PAGE_SIZE) =>
  infiniteQueryOptions({
    queryKey: [...radioKeys.all, 'stations', params, limit] as const,
    queryFn: ({ pageParam, signal }): Promise<RadioStationPage> =>
      request(
        api.GET('/radio/stations', {
          params: { query: { ...params, offset: pageParam, limit } },
          signal,
        }),
      ),
    initialPageParam: 0,
    getNextPageParam: (last) => {
      const next = last.offset + last.items.length;
      return last.items.length > 0 && next < last.total ? next : undefined;
    },
    ...catalogRetry,
  });

export const radioFacetsQuery = (params: RadioQueryParams) =>
  queryOptions({
    queryKey: [...radioKeys.all, 'facets', params] as const,
    queryFn: ({ signal }): Promise<RadioFacets> =>
      request(api.GET('/radio/facets', { params: { query: params }, signal })),
    ...catalogRetry,
  });

export const radioFavoritesQuery = queryOptions({
  queryKey: [...radioKeys.all, 'favorites'] as const,
  queryFn: ({ signal }): Promise<RadioStation[]> =>
    request(api.GET('/radio/favorites', { signal })),
});

export const radioPreferencesQuery = queryOptions({
  queryKey: [...radioKeys.all, 'preferences'] as const,
  queryFn: ({ signal }): Promise<RadioPreferences> =>
    request(api.GET('/radio/preferences', { signal })),
  staleTime: Infinity,
});

/** Stores default country, last filter and sort of the active profile (F-12). */
export function useSaveRadioPreferences() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: {
      defaultCountry: string;
      lastFilter?: RadioFilter;
      lastSort?: RadioSort;
    }): Promise<RadioPreferences> => request(api.PUT('/radio/preferences', { body })),
    onSuccess: (saved) => queryClient.setQueryData(radioPreferencesQuery.queryKey, saved),
  });
}

/** Puts the favourites in a new order (the complete list of their ids). */
export function useRadioFavoriteOrder() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (stationIds: number[]) =>
      request(api.PUT('/radio/favorites', { body: { stationIds } })),
    onMutate: (stationIds) => {
      const before = queryClient.getQueryData(radioFavoritesQuery.queryKey);
      if (before) {
        const byId = new Map(before.map((station) => [station.id, station]));
        queryClient.setQueryData(
          radioFavoritesQuery.queryKey,
          stationIds.flatMap((id) => byId.get(id) ?? []),
        );
      }
      return { before };
    },
    onError: (_error, _ids, context) => {
      if (context?.before) queryClient.setQueryData(radioFavoritesQuery.queryKey, context.before);
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: radioFavoritesQuery.queryKey }),
  });
}
