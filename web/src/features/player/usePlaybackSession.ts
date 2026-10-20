import { useCallback, useEffect, useRef } from 'react';
import { api } from '@/api/client';

/**
 * Ends the playback session on the server when the player is left or the page goes away
 * (`DELETE /playback/sessions/{id}` with `keepalive`): its stream token then answers 401 and the
 * server stops the transcode (plan 5.4). The stop on unmount waits one task, so React's
 * development double mount (StrictMode) does not end the session it is about to use; a pending
 * stop is cancelled only when the same session comes back, a replaced session is always stopped.
 *
 * Returns a function that stops the current session right away (a version switch) and resolves
 * when the server answered; the later stop on leave then sends nothing a second time.
 */
export function useStopPlaybackOnLeave(playbackSessionId: string | undefined): () => Promise<void> {
  const pending = useRef<{ id: string; timer: number } | undefined>(undefined);
  const stopCurrent = useRef<() => Promise<void>>(() => Promise.resolve());
  useEffect(() => {
    if (!playbackSessionId) return;
    if (pending.current?.id === playbackSessionId) {
      window.clearTimeout(pending.current.timer);
      pending.current = undefined;
    }
    let stopped: Promise<void> | undefined;
    const stop = () => {
      stopped ??= api
        .DELETE('/playback/sessions/{id}', {
          params: { path: { id: playbackSessionId } },
          keepalive: true,
        })
        .then(
          () => undefined,
          () => undefined,
        );
      return stopped;
    };
    stopCurrent.current = stop;
    const onPageHide = (event: PageTransitionEvent) => {
      if (!event.persisted) void stop();
    };
    window.addEventListener('pagehide', onPageHide);
    return () => {
      window.removeEventListener('pagehide', onPageHide);
      pending.current = { id: playbackSessionId, timer: window.setTimeout(() => void stop(), 0) };
    };
  }, [playbackSessionId]);
  return useCallback(() => stopCurrent.current(), []);
}
