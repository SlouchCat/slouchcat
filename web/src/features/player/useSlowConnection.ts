import type { PlaybackInfo, QualityOption } from '@slouchcat/api-client';
import { useCallback, useEffect, useRef, useState, type RefObject } from 'react';

/** A wait for data counts as a stall from this long on (V-18: «waiting» over 1 s). */
export const STALL_MS = 1000;
/** Two stalls within this window make the connection slow. */
export const STALL_WINDOW_MS = 60_000;

/**
 * The rung one step below the one playing (V-18): the highest rung the server offers that is
 * lower than the picture now (the transcode's height, else the file's). None when there is no
 * lower one.
 */
export function lowerQuality(info: PlaybackInfo): QualityOption | undefined {
  const height = info.transcoding?.height ?? info.mediaFile.height;
  if (height === undefined) return undefined;
  return info.qualities
    .filter(
      (option) =>
        option.quality !== 'auto' &&
        option.available &&
        option.height !== undefined &&
        option.height < height,
    )
    .sort((a, b) => (b.height ?? 0) - (a.height ?? 0))[0];
}

/**
 * Counts the stalls of the media element (V-18, 11.5): a `waiting` after the first `playing` (the
 * start fills the buffer; that is not the connection stalling) that lasts over a second and that
 * no seek caused. Two of them within a minute make it slow; the player then suggests a lower
 * rung and never switches by itself. Dismissed, the hint stays away for this session.
 */
export function useSlowConnection(
  media: RefObject<HTMLMediaElement | null>,
  enabled: boolean,
): { slow: boolean; dismiss: () => void } {
  const [slow, setSlow] = useState(false);
  const [dismissed, setDismissed] = useState(false);
  const stalls = useRef<number[]>([]);
  // The element played once (`playing`); kept while the hook is switched off and on again.
  const played = useRef(false);

  useEffect(() => {
    const element = media.current;
    if (!element || !enabled || dismissed) return;
    let timer: number | undefined;
    const onWaiting = () => {
      // A seek waits for data as well; that is the viewer's doing, not the connection's. Before
      // the first `playing` the player is still starting.
      if (element.seeking || !played.current) return;
      window.clearTimeout(timer);
      timer = window.setTimeout(() => {
        const now = Date.now();
        stalls.current = [...stalls.current.filter((at) => now - at < STALL_WINDOW_MS), now];
        if (stalls.current.length >= 2) setSlow(true);
      }, STALL_MS);
    };
    const onResume = () => window.clearTimeout(timer);
    const onPlaying = () => {
      played.current = true;
      onResume();
    };
    element.addEventListener('waiting', onWaiting);
    element.addEventListener('playing', onPlaying);
    element.addEventListener('seeking', onResume);
    element.addEventListener('pause', onResume);
    return () => {
      window.clearTimeout(timer);
      element.removeEventListener('waiting', onWaiting);
      element.removeEventListener('playing', onPlaying);
      element.removeEventListener('seeking', onResume);
      element.removeEventListener('pause', onResume);
    };
  }, [dismissed, enabled, media]);

  const dismiss = useCallback(() => setDismissed(true), []);
  return { slow: slow && enabled && !dismissed, dismiss };
}
