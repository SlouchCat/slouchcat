import { useQueryClient } from '@tanstack/react-query';
import { useEffect } from 'react';
import { offlineSupported } from '@/features/offline/support';

/** The engine waits this long for an idle moment, so it never competes with the start (P-10). */
const IDLE_TIMEOUT_MS = 5000;

/**
 * Starts the offline engine (sync, fetch of downloads, upload of offline events) once the page is
 * idle, as its own chunk and only where the browser can keep downloads (features/offline).
 */
export function useOfflineEngine(): void {
  const queryClient = useQueryClient();
  useEffect(() => {
    if (!offlineSupported()) return;
    let cancelled = false;
    let stop: (() => void) | undefined;
    const start = () => {
      import('@/features/offline/engine')
        .then(({ startOfflineEngine }) => {
          if (!cancelled) stop = startOfflineEngine(queryClient);
        })
        .catch(() => undefined);
    };
    // Safari has no requestIdleCallback.
    let idle: number | undefined;
    let timer: number | undefined;
    if (typeof window.requestIdleCallback === 'function') {
      idle = window.requestIdleCallback(start, { timeout: IDLE_TIMEOUT_MS });
    } else {
      timer = window.setTimeout(start, 0);
    }
    return () => {
      cancelled = true;
      if (idle !== undefined) window.cancelIdleCallback(idle);
      if (timer !== undefined) window.clearTimeout(timer);
      stop?.();
    };
  }, [queryClient]);
}
