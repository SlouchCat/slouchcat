import { isProblem } from '@slouchcat/api-client';
import { useEffect, useRef } from 'react';
import { api } from '@/api/client';
import type { EndedReason } from './VideoPlayer';

/**
 * The player asks for the state of its playback session this often while it plays: a stop by the
 * server shows within 5 s (concept 13, 11.13). A request that takes longer is abandoned.
 */
export const STOP_WATCH_INTERVAL_MS = 4000;

interface StopWatchOptions {
  playbackSessionId: string;
  /** True while the media really plays: not paused, not ended, not during a switch. */
  active: boolean;
  /**
   * The server stopped the playback. The reason is `limit` for the watch-time limit, `admin` for
   * any other stop with a known reason, undefined when the server names none (an expired
   * playback, an ended browser session or profile): the player then decides as for a 401 of the
   * stream.
   */
  onStopped: (reason?: EndedReason) => void;
}

/**
 * Asks `getPlaybackSessionState` every 4 s while the playback plays, also with the page hidden (a
 * book plays on with the screen locked), and at once when the page becomes visible again or the
 * playback resumes after 4 s or more without a request. A player that buffered its whole title
 * makes no further stream request, so the 401 of a stopped stream would never reach it.
 *
 * `stopped`, a 401 (the browser session ended: logout, deleted account) and a 403
 * `profile_required` (the profile was deleted) are a stop. A 404 (a playback the server does not
 * know, for instance after its restart) and a failed request mean nothing: the next stream request
 * answers 401 if the playback ended.
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
  // When the last request went out; kept across pauses, so a pause does not push the next one.
  const lastCheckedAt = useRef<number | null>(null);

  useEffect(() => {
    if (!active) return;
    let controller: AbortController | null = null;
    let timer: number | undefined;

    const check = async () => {
      controller?.abort();
      const own = new AbortController();
      controller = own;
      lastCheckedAt.current = Date.now();
      const giveUp = window.setTimeout(() => own.abort(), STOP_WATCH_INTERVAL_MS);
      try {
        const { data, error, response } = await api.GET('/playback/sessions/{id}/state', {
          params: { path: { id: playbackSessionId } },
          signal: own.signal,
        });
        if (own.signal.aborted) return;
        if (
          response.status === 401 ||
          (response.status === 403 && isProblem(error) && error.code === 'profile_required')
        ) {
          stopped.current(undefined);
        } else if (data?.state === 'stopped') {
          const by = data.stoppedBy;
          stopped.current(by === 'limit' ? 'limit' : by ? 'admin' : undefined);
        }
      } catch {
        // Abandoned or offline: the next request asks again.
      } finally {
        window.clearTimeout(giveUp);
        if (controller === own) controller = null;
      }
    };

    // The next request 4 s after the last one; at once when that is already due.
    const schedule = () => {
      const since = lastCheckedAt.current === null ? Infinity : Date.now() - lastCheckedAt.current;
      timer = window.setTimeout(tick, Math.max(0, STOP_WATCH_INTERVAL_MS - since));
    };
    const tick = () => {
      void check();
      schedule();
    };
    const onVisibility = () => {
      if (document.visibilityState !== 'visible') return;
      window.clearTimeout(timer);
      tick();
    };

    schedule();
    document.addEventListener('visibilitychange', onVisibility);
    return () => {
      window.clearTimeout(timer);
      document.removeEventListener('visibilitychange', onVisibility);
      controller?.abort();
      controller = null;
    };
  }, [active, playbackSessionId]);
}
