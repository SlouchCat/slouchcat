import type {
  Library,
  PodcastDownloadEstimate,
  PodcastDownloadEstimateRequest,
} from '@slouchcat/api-client';
import { queryOptions } from '@tanstack/react-query';
import { useCallback, useRef, useState } from 'react';
import { api, request } from '@/api/client';

/** The guide to the writable podcast folder (docs/INSTALLATION.md, «Podcast-Folgen speichern»). */
export const PODCAST_FOLDER_GUIDE =
  'https://github.com/SlouchCat/slouchcat/blob/main/docs/INSTALLATION.md#podcast-folgen-speichern-freiwillig';

/** The values of «Gleichzeitig laden» (concept 19 part A, 5.1). */
export const PARALLEL_VALUES = ['1', '2', '3', '4', '5'] as const;

/** The highest N of «Neue + letzte N». */
export const LAST_N_MAX = 100;

/**
 * One library with what only `getLibrary` carries: the folder check of a podcast library
 * (`podcastFolder`). The same key as the trickplay dialog's.
 */
export const libraryDetailQuery = (id: number) =>
  queryOptions({
    queryKey: ['admin', 'library', id],
    queryFn: ({ signal }): Promise<Library> =>
      request(api.GET('/admin/libraries/{id}', { params: { path: { id } }, signal })),
  });

/** What an imagined setting would load once (`estimatePodcastDownloads`); changes nothing. */
export const estimatePodcastDownloads = (
  libraryId: number,
  body: PodcastDownloadEstimateRequest,
): Promise<PodcastDownloadEstimate> =>
  request(
    api.POST('/admin/libraries/{id}/podcast-downloads/estimate', {
      params: { path: { id: libraryId } },
      body,
    }),
  );

/** A change that waits for «Laden»: what it loads once more than the setting that applies now. */
export interface PendingLoad<T> {
  value: T;
  episodes: number;
  bytes: number;
  freeBytes: number;
}

/**
 * The confirmation of a change that loads once (12.3): `ask` shows the value at once, asks the
 * server what the new and the current setting would load and applies the change right away when
 * the new one loads nothing more; otherwise it waits in `pending` for `confirm` («Laden») or
 * `cancel` («Abbrechen»), and without «Laden» nothing changes. A later `ask` replaces an earlier
 * one, also while its estimate is on the way. Both buttons go with the question, so the focus
 * returns to the control that asked (rule 8: no focus lost to the page).
 */
export function useLoadConfirmation<T>(apply: (value: T) => void) {
  const [asked, setAsked] = useState<T | null>(null);
  const [pending, setPending] = useState<PendingLoad<T> | null>(null);
  const [failure, setFailure] = useState<unknown>(null);
  const round = useRef(0);
  const origin = useRef<HTMLElement | null>(null);
  const refocus = useCallback(() => {
    const target = origin.current;
    origin.current = null;
    if (target) window.requestAnimationFrame(() => target.isConnected && target.focus());
  }, []);

  const ask = useCallback(
    (
      value: T,
      next: () => Promise<PodcastDownloadEstimate>,
      current: () => Promise<PodcastDownloadEstimate | null>,
    ) => {
      round.current += 1;
      const mine = round.current;
      origin.current =
        document.activeElement instanceof HTMLElement ? document.activeElement : null;
      setAsked(value);
      setPending(null);
      setFailure(null);
      void Promise.all([next(), current()]).then(
        ([after, before]) => {
          if (round.current !== mine) return;
          const episodes = after.episodes - (before?.episodes ?? 0);
          if (episodes <= 0) {
            origin.current = null;
            setAsked(null);
            apply(value);
            return;
          }
          setPending({
            value,
            episodes,
            bytes: Math.max(0, after.bytes - (before?.bytes ?? 0)),
            freeBytes: after.freeBytes,
          });
        },
        (error: unknown) => {
          if (round.current !== mine) return;
          setAsked(null);
          setFailure(error);
        },
      );
    },
    [apply],
  );
  // A change that needs no question drops an open one; the focus stays where it is.
  const reset = useCallback(() => {
    round.current += 1;
    origin.current = null;
    setAsked(null);
    setPending(null);
  }, []);
  const cancel = useCallback(() => {
    round.current += 1;
    setAsked(null);
    setPending(null);
    refocus();
  }, [refocus]);
  const confirm = useCallback(() => {
    if (!pending) return;
    round.current += 1;
    setAsked(null);
    setPending(null);
    apply(pending.value);
    refocus();
  }, [apply, pending, refocus]);
  return {
    /** The value the page shows instead of the stored one while it is asked or pending. */
    shown: pending?.value ?? asked,
    estimating: asked !== null && pending === null,
    pending,
    failure,
    ask,
    reset,
    cancel,
    confirm,
  };
}
