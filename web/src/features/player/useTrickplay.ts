import type { Trickplay } from '@slouchcat/api-client';
import { useQuery } from '@tanstack/react-query';
import { useMemo } from 'react';
import { api, apiUrl, isApiError, request } from '@/api/client';
import type { SeekBarTrickplay } from '@/ui/aria';

/** While the set is `queued` or `running`, the player asks again this often, without a limit. */
export const TRICKPLAY_POLL_MS = 10_000;

/** Whether the server still makes the set: the player keeps asking (concept 18, 9). */
export const trickplayPending = (trickplay: Pick<Trickplay, 'state'> | undefined): boolean =>
  trickplay?.state === 'queued' || trickplay?.state === 'running';

/**
 * The contract's `Trickplay` as the seek bar takes it, with every sheet URL resolved. A set that
 * `failed` without a sheet has nothing to show: undefined, so the field shows only time and name
 * (concept 18, 9).
 */
export function seekBarTrickplay(
  trickplay: Trickplay,
  resolve: (url: string, index: number) => string | undefined = apiUrl,
): SeekBarTrickplay | undefined {
  const sheets = trickplay.sheets.flatMap((sheet) => {
    const url = resolve(sheet.url, sheet.index);
    return url ? [{ index: sheet.index, url }] : [];
  });
  if (trickplay.state === 'failed' && sheets.length === 0) return undefined;
  return {
    tileWidth: trickplay.tileWidth,
    tileHeight: trickplay.tileHeight,
    columns: trickplay.columns,
    rows: trickplay.rows,
    pointsMs: trickplay.pointsMs,
    sheets,
  };
}

interface TrickplayOptions {
  itemId: string;
  mediaFileId: string;
  /** Each start has its own answer: a new start may queue the set again. */
  playbackSessionId: string;
  /** `PlaybackInfo.trickplay`; absent: no preview for this title (the server decided). */
  initial: Trickplay | undefined;
}

/**
 * The preview tiles of a playback (concept 18 part A): what the start answered, asked again with
 * `getTrickplay` every TRICKPLAY_POLL_MS while the set is `queued` or `running`; at `complete` or
 * `failed` the player stops asking. A 404 (the setting went to «Aus» meanwhile) stops as well and
 * keeps the sheets it knew.
 */
export function useTrickplay({
  itemId,
  mediaFileId,
  playbackSessionId,
  initial,
}: TrickplayOptions): SeekBarTrickplay | undefined {
  const query = useQuery({
    queryKey: ['trickplay', itemId, mediaFileId, playbackSessionId],
    queryFn: ({ signal }) =>
      request(
        api.GET('/items/{id}/trickplay', {
          params: { path: { id: itemId }, query: { mediaFileId } },
          signal,
        }),
      ),
    enabled: initial !== undefined,
    initialData: initial,
    // The start's answer is fresh; only the interval asks again.
    staleTime: Infinity,
    retry: false,
    refetchOnWindowFocus: false,
    refetchInterval: (current) =>
      isApiError(current.state.error, 404) || !trickplayPending(current.state.data)
        ? false
        : TRICKPLAY_POLL_MS,
  });
  const data = query.data;
  return useMemo(() => (data ? seekBarTrickplay(data) : undefined), [data]);
}
