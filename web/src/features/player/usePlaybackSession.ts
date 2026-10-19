import { useEffect, useRef } from 'react';
import { api } from '@/api/client';

/**
 * Ends the playback session on the server when the player is left or the page goes away
 * (`DELETE /playback/sessions/{id}` with `keepalive`): its stream token then answers 401 and the
 * server stops the transcode (plan 5.4). The stop on unmount waits one task, so React's
 * development double mount (StrictMode) does not end the session it is about to use; a pending
 * stop is cancelled only when the same session comes back, a replaced session is always stopped.
 */
export function useStopPlaybackOnLeave(playbackSessionId: string | undefined): void {
  const pending = useRef<{ id: string; timer: number } | undefined>(undefined);
  useEffect(() => {
    if (!playbackSessionId) return;
    if (pending.current?.id === playbackSessionId) {
      window.clearTimeout(pending.current.timer);
      pending.current = undefined;
    }
    let stopped = false;
    const stop = () => {
      if (stopped) return;
      stopped = true;
      api
        .DELETE('/playback/sessions/{id}', {
          params: { path: { id: playbackSessionId } },
          keepalive: true,
        })
        .catch(() => undefined);
    };
    const onPageHide = (event: PageTransitionEvent) => {
      if (!event.persisted) stop();
    };
    window.addEventListener('pagehide', onPageHide);
    return () => {
      window.removeEventListener('pagehide', onPageHide);
      pending.current = { id: playbackSessionId, timer: window.setTimeout(stop, 0) };
    };
  }, [playbackSessionId]);
}
