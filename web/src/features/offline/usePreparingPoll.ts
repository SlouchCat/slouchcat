import { useEffect } from 'react';
import { useOnline } from '@/lib/useOnline';
import { kickOfflineEngine } from './engine';
import type { StoredDownload } from './types';

/** While a page shows a download the server still prepares, the engine syncs this often. */
export const PREPARING_POLL_MS = 10_000;

/** The server has not finished the download yet (the device cannot fetch it). */
export const isPreparing = (download: StoredDownload) =>
  !download.complete &&
  !download.pendingDelete &&
  (download.state === 'queued' ||
    download.state === 'preparing' ||
    download.state === 'paused' ||
    download.state === 'evicted');

/**
 * Syncs every 10 s instead of every 60 s while the page shows a download in preparation, so its
 * progress moves and the fetch starts soon after the server is ready.
 */
export function usePreparingPoll(downloads: readonly StoredDownload[]): void {
  const online = useOnline();
  const active = online && downloads.some(isPreparing);
  useEffect(() => {
    if (!active) return;
    const timer = window.setInterval(kickOfflineEngine, PREPARING_POLL_MS);
    return () => window.clearInterval(timer);
  }, [active]);
}
