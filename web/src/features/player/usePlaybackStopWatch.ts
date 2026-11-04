import { useEffect, useRef } from 'react';
import { api } from '@/api/client';
import type { EndedReason } from './VideoPlayer';

/**
 * The player asks for the state of its playback session this often while it plays: a stop by the
 * server shows within 5 s (concept 13, 11.13).
 */
export const STOP_WATCH_INTERVAL_MS = 4000;

interface StopWatchOptions {
  playbackSessionId: string;
  /** True while the media really plays: not paused, not ended, not during a switch. */
  active: boolean;
  /**
   * The server stopped the playback. The reason is `limit` for the watch-time limit, `admin` for
   * any other stop with a known reason, undefined for an expired playback (the player then
   * decides as for a 401 of the stream).
   */
  onStopped: (reason?: EndedReason) => void;
}

/**
 * Asks `getPlaybackSessionState` every 4 s while the playback plays and the page is visible, and
 * once more when the page becomes visible again. A player that buffered its whole title makes no
 * further stream request, so the 401 of a stopped stream would never reach it. A 404 (a playback
 * the server does not know, for instance after its restart) and a failed request mean nothing:
 * the next stream request answers 401 if the playback ended.
 */
export function usePlaybackStopWatch({
  playbackSessionId,
  active,
  onStopped,
}: StopWatchOptions): void {
  const stopped = useRef(onStopped);
  useEffect(() => {
    stopped.current = onStopped;
  });

  useEffect(() => {
    if (!active) return;
    let controller: AbortController | null = null;
    const check = async () => {
      if (controller || document.visibilityState === 'hidden') return;
      const own = new AbortController();
      controller = own;
      try {
        const { data } = await api.GET('/playback/sessions/{id}/state', {
          params: { path: { id: playbackSessionId } },
          signal: own.signal,
        });
        if (!own.signal.aborted && data?.state === 'stopped') {
          const by = data.stoppedBy;
          stopped.current(by === 'limit' ? 'limit' : by ? 'admin' : undefined);
        }
      } catch {
        // Aborted or offline: the next tick asks again.
      } finally {
        if (controller === own) controller = null;
      }
    };
    const timer = window.setInterval(() => void check(), STOP_WATCH_INTERVAL_MS);
    const onVisibility = () => {
      if (document.visibilityState === 'visible') void check();
    };
    document.addEventListener('visibilitychange', onVisibility);
    return () => {
      window.clearInterval(timer);
      document.removeEventListener('visibilitychange', onVisibility);
      controller?.abort();
      controller = null;
    };
  }, [active, playbackSessionId]);
}
