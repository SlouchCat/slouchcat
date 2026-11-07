import type { components } from '@slouchcat/api-client';
import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query';
import { api, request } from '@/api/client';

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

/** The titles the station's stream announced; asked again every 10 s while the tab is visible. */
export const radioNowQuery = (stationId: number) =>
  queryOptions({
    queryKey: radioKeys.now(stationId),
    queryFn: ({ signal }): Promise<RadioNow> =>
      request(
        api.GET('/radio/stations/{stationId}/now', { params: { path: { stationId } }, signal }),
      ),
    refetchInterval: RADIO_NOW_INTERVAL_MS,
    refetchIntervalInBackground: false,
  });

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

/** Marks a station as favourite of the active profile or removes the mark. */
export function useRadioFavorite() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ stationId, favorite }: { stationId: number; favorite: boolean }) =>
      favorite
        ? request(api.PUT('/radio/favorites/{stationId}', { params: { path: { stationId } } }))
        : request(api.DELETE('/radio/favorites/{stationId}', { params: { path: { stationId } } })),
    onSettled: () => queryClient.invalidateQueries({ queryKey: radioKeys.all }),
  });
}
