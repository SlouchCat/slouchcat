import type { ProgressEvent } from '@slouchcat/api-client';
import { useEffect, useRef, type RefObject } from 'react';
import { api } from '@/api/client';

/** Interval of `playing` events while the video runs (plan 3.4 step 7). */
export const PROGRESS_INTERVAL_MS = 10_000;

interface ReporterOptions {
  itemId: string;
  mediaFileId: string;
  /** Duration from the media file, used until the element knows its own. */
  fallbackDurationMs: number;
  video: RefObject<HTMLVideoElement | null>;
  /** False until the <video> element is mounted. */
  enabled: boolean;
}

/**
 * Sends progress events for one playback: every 10 s while playing, on pause, on `ended`, when the
 * page is hidden and when the player is left (the last two with `keepalive`, so they survive the
 * page going away). `seq` rises per event; `playedMs` is wall-clock time actually spent playing
 * since the previous event (buffering and seeking do not count).
 */
export function useProgressReporter({
  itemId,
  mediaFileId,
  fallbackDurationMs,
  video,
  enabled,
}: ReporterOptions): void {
  const seq = useRef(1);
  const playedMs = useRef(0);
  const playingSince = useRef<number | null>(null);
  const started = useRef(false);

  useEffect(() => {
    const element = video.current;
    if (!enabled || !element) return;

    const stopClock = () => {
      if (playingSince.current !== null) {
        playedMs.current += performance.now() - playingSince.current;
        playingSince.current = null;
      }
    };
    const startClock = () => {
      stopClock();
      playingSince.current = performance.now();
    };

    const send = (state: ProgressEvent['state'], keepalive = false) => {
      if (!started.current) return;
      const running = playingSince.current !== null;
      stopClock();
      const body: ProgressEvent = {
        mediaFileId,
        positionMs: Math.max(0, Math.round(element.currentTime * 1000)),
        durationMs: Number.isFinite(element.duration)
          ? Math.round(element.duration * 1000)
          : fallbackDurationMs,
        playedMs: Math.round(playedMs.current),
        seq: seq.current,
        state,
        clientTime: new Date().toISOString(),
      };
      seq.current += 1;
      playedMs.current = 0;
      if (running) startClock();
      // Progress is best effort: a lost event is repaired by the next one.
      api
        .POST('/items/{id}/progress', { params: { path: { id: itemId } }, body, keepalive })
        .catch(() => undefined);
    };

    const onPlaying = () => {
      started.current = true;
      startClock();
    };
    const onWaiting = () => stopClock();
    const onPause = () => {
      // When the video reaches its end the browser fires pause before ended; report only ended.
      if (element.ended) return;
      stopClock();
      send('paused');
    };
    const onEnded = () => {
      stopClock();
      send('ended');
    };
    const onHidden = () => {
      if (document.visibilityState === 'hidden') send(element.paused ? 'paused' : 'playing', true);
    };
    const onPageHide = () => send('paused', true);

    const interval = window.setInterval(() => {
      if (!element.paused && !element.ended) send('playing');
    }, PROGRESS_INTERVAL_MS);

    element.addEventListener('playing', onPlaying);
    element.addEventListener('waiting', onWaiting);
    element.addEventListener('pause', onPause);
    element.addEventListener('ended', onEnded);
    document.addEventListener('visibilitychange', onHidden);
    window.addEventListener('pagehide', onPageHide);

    return () => {
      window.clearInterval(interval);
      element.removeEventListener('playing', onPlaying);
      element.removeEventListener('waiting', onWaiting);
      element.removeEventListener('pause', onPause);
      element.removeEventListener('ended', onEnded);
      document.removeEventListener('visibilitychange', onHidden);
      window.removeEventListener('pagehide', onPageHide);
      // Leaving the player: report where the viewer stopped.
      if (!element.ended) send('paused', true);
    };
  }, [enabled, fallbackDurationMs, itemId, mediaFileId, video]);
}
