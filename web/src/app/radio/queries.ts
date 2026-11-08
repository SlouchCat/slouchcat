import type { components } from '@slouchcat/api-client';
import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query';
import { api, request } from '@/api/client';
import { useRadio } from './useRadio';

// The API calls of the radio player (contract 1.17.0). Every radio query lives under the key
// `radio`, so a profile switch (clearProfileData) forgets all of them.

type RadioHeartbeat = components['schemas']['RadioHeartbeat'];
type RadioNow = components['schemas']['RadioNow'];
export type RadioNowEntry = components['schemas']['RadioNowEntry'];

export const radioKeys = {
  all: ['radio'] as const,
  now: (stationId: number) => ['radio', 'now', stationId] as const,
};

/** Opens a listen session for a station (the server starts or joins its relay). */
export const startRadioListen = (stationId: number) =>
  request(api.POST('/radio/stations/{stationId}/listen', { params: { path: { stationId } } }));

export const sendRadioHeartbeat = (listenId: string, playing: boolean): Promise<RadioHeartbeat> =>
  request(
    api.POST('/radio/listen/{listenId}/heartbeat', {
      params: { path: { listenId } },
      body: { playing },
    }),
  );

/**
 * Ends a listen session. `keepalive` lets the request outlive the page (tab closed, provider
 * unmounted); the api client adds the CSRF header. Failures are dropped: the server expires a
 * session nobody asks for after the buffer length.
 */
export const endRadioListen = (listenId: string): Promise<void> =>
  api.DELETE('/radio/listen/{listenId}', { params: { path: { listenId } }, keepalive: true }).then(
    () => undefined,
    () => undefined,
  );

/** How often «Jetzt läuft» asks for the titles of the running station. */
export const RADIO_NOW_INTERVAL_MS = 10_000;

/**
 * The titles the station's stream announced; asked again every 10 s while the tab is visible.
 * With a time shift window (`windowSeconds`, contract 1.19.0) it asks from the start of the window
 * on, so «Zuvor gespielt» holds the whole timeline a listener can play back to (7.6).
 */
export const radioNowQuery = (stationId: number, windowSeconds = 0) =>
  queryOptions({
    queryKey: radioKeys.now(stationId),
    queryFn: ({ signal }): Promise<RadioNow> =>
      request(
        api.GET('/radio/stations/{stationId}/now', {
          params: {
            path: { stationId },
            ...(windowSeconds > 0 ? { query: { since: Date.now() - windowSeconds * 1000 } } : {}),
          },
          signal,
        }),
      ),
    refetchInterval: RADIO_NOW_INTERVAL_MS,
    refetchIntervalInBackground: false,
  });

/** Sizes of a cover or a logo (contract 1.19.0, `RadioImageSize`). */
export type RadioImageSize = 'w96' | 'w256' | 'w512';

/**
 * A cover or logo URL of the server (w256) in another size: the contract names the size in the
 * path, so it is replaced there.
 */
export const radioImageSize = (url: string | undefined, size: RadioImageSize) =>
  url?.replace(/\/w256(?=[/?]|$)/, `/${size}`);

/** The entries before `current`, newest first: «Zuvor gespielt» (7.6). */
export function previousRadioEntries(
  entries: readonly RadioNowEntry[],
  current: RadioNowEntry | null,
): RadioNowEntry[] {
  const index = current ? entries.indexOf(current) : -1;
  return index <= 0 ? [] : entries.slice(0, index).reverse();
}

/**
 * The entry of the title timeline that sounds at `date`: the last one stamped at or before it.
 * Without a playback date the newest entry; null without entries.
 */
export function currentRadioEntry(
  entries: readonly RadioNowEntry[],
  date: Date | null,
): RadioNowEntry | null {
  if (entries.length === 0) return null;
  if (!date) return entries[entries.length - 1] ?? null;
  const at = date.getTime();
  let found: RadioNowEntry | null = null;
  for (const entry of entries) {
    if (entry.at <= at) found = entry;
    else break;
  }
  return found;
}

/**
 * Marks a station as favourite of the active profile or removes the mark. The player's station
 * takes the stored mark here and not in a callback of `mutate`: TanStack runs that one only for
 * the latest call, so a quick second press would leave the heart of the first station stale.
 */
export function useRadioFavorite() {
  const queryClient = useQueryClient();
  const { markFavorite } = useRadio();
  return useMutation({
    mutationFn: ({ stationId, favorite }: { stationId: number; favorite: boolean }) =>
      favorite
        ? request(api.PUT('/radio/favorites/{stationId}', { params: { path: { stationId } } }))
        : request(api.DELETE('/radio/favorites/{stationId}', { params: { path: { stationId } } })),
    onSuccess: (_data, { stationId, favorite }) => markFavorite(stationId, favorite),
    // The mark changes «Meine Sender», the station lists and the counts of their filter only.
    onSettled: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: [...radioKeys.all, 'favorites'] }),
        queryClient.invalidateQueries({ queryKey: [...radioKeys.all, 'stations'] }),
        queryClient.invalidateQueries({ queryKey: [...radioKeys.all, 'facets', { fav: true }] }),
      ]),
  });
}
