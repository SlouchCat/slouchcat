import { onlineManager } from '@tanstack/react-query';
import { act, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi, type MockInstance } from 'vitest';
import { json, mockApi, renderApp, session, type Call } from '@/test/api';
import { resetOfflineFiles, seedFile } from '@/test/offlineFiles';
import {
  downloadIds,
  downloadState,
  manifest,
  offlineProfile,
  rules,
  storedDownload,
  storedProfile,
} from '@/test/offlineFixtures';
import {
  putDownloads,
  putProfiles,
  queuedEvents,
  resetOfflineStore,
  setMeta,
  storedProfiles,
} from '@/test/offlineStore';
import { chooseOfflineProfile, leaveOfflineProfile } from './offlineSession';
import type { OfflineEvent } from './types';
import { remainingMs } from './watchTime';

// O-5 and AN-18 as one flow through the real player, event queue, watch time and engine (only
// IndexedDB, OPFS and hls.js are stand-ins): 10 minutes of offline play on the fake clock, with
// a 10-minute rest of the day's watch time, then the reconnect uploads the queue.

vi.mock('@/features/offline/store', () => import('@/test/offlineStore'));
vi.mock('@/features/offline/opfs', () => import('@/test/offlineFiles'));
vi.mock('@/features/offline/support', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./support')>()),
  offlineSupported: () => true,
}));

// jsdom has no MediaSource: hls.js only has to be there (as in OfflinePlayer.test.tsx).
const hls = vi.hoisted(() => {
  class FakeHls {
    static Events = { ERROR: 'hlsError' };
    static ErrorTypes = { NETWORK_ERROR: 'networkError', MEDIA_ERROR: 'mediaError' };
    static isSupported = () => true;
    on() {}
    loadSource() {}
    attachMedia() {}
    stopLoad() {}
    detachMedia() {}
    recoverMediaError() {}
    destroy() {}
  }
  return { FakeHls };
});
vi.mock('hls.js/light', () => ({ default: hls.FakeHls }));

const MINUTE_MS = 60_000;
const HOUR_MS = 60 * MINUTE_MS;

/** A video element that plays on the fake clock: its time runs while it is not paused. */
function clockMedia() {
  const media = { paused: true, at: 0, since: 0 };
  const time = () =>
    media.paused ? media.at : media.at + (performance.now() - media.since) / 1000;
  vi.spyOn(HTMLMediaElement.prototype, 'play').mockImplementation(function (
    this: HTMLMediaElement,
  ) {
    if (media.paused) {
      media.since = performance.now();
      media.paused = false;
    }
    this.dispatchEvent(new Event('playing'));
    return Promise.resolve();
  });
  vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(function (
    this: HTMLMediaElement,
  ) {
    if (media.paused) return;
    media.at = time();
    media.paused = true;
    this.dispatchEvent(new Event('pause'));
  });
  vi.spyOn(HTMLMediaElement.prototype, 'paused', 'get').mockImplementation(() => media.paused);
  vi.spyOn(HTMLMediaElement.prototype, 'ended', 'get').mockImplementation(() => false);
  vi.spyOn(HTMLMediaElement.prototype, 'duration', 'get').mockImplementation(() => HOUR_MS / 1000);
  vi.spyOn(HTMLMediaElement.prototype, 'currentTime', 'get').mockImplementation(time);
  vi.spyOn(HTMLMediaElement.prototype, 'currentTime', 'set').mockImplementation((value: number) => {
    media.at = value;
    media.since = performance.now();
  });
  return media;
}

let onLine: MockInstance<() => boolean>;

beforeEach(async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  resetOfflineStore();
  resetOfflineFiles();
  onLine = vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(false);
  onlineManager.setOnline(false);
  URL.createObjectURL = vi.fn(() => 'blob:offline-test');
  URL.revokeObjectURL = vi.fn();
  const now = Date.now();
  // A film of an hour, so ten minutes of it never reach the end.
  await putDownloads([storedDownload({ manifest: { ...manifest(), durationMs: HOUR_MS } })]);
  await putProfiles([
    storedProfile({
      snapshot: offlineProfile({
        watchTime: {
          limitMinutes: 60,
          remainingMinutes: 10,
          resetsAt: new Date(now + 6 * HOUR_MS).toISOString(),
        },
      }),
      watch: {
        limitMinutes: 60,
        remainingMinutes: 10,
        resetsAt: now + 6 * HOUR_MS,
        syncedAt: now,
        period: -1,
        playedMs: 0,
      },
    }),
  ]);
  await setMeta('serverOffsetMs', 0);
  await setMeta('accountId', 1);
  seedFile(downloadIds.alien, 'media.mp4', new Uint8Array(9000));
  chooseOfflineProfile({ accountId: 1, profileId: 1 });
});

afterEach(() => {
  leaveOfflineProfile();
  vi.useRealTimers();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  onlineManager.setOnline(true);
});

describe('ten minutes of offline play', () => {
  it('queues ~60 events, stops at the 10-min rest and uploads every event after the reconnect', async () => {
    const media = clockMedia();
    // Flight mode: every request fails at the network until the reconnect.
    const network = { up: false };
    const reachable =
      (reply: (call: Call) => Response) =>
      (call: Call): Response => {
        if (!network.up) throw new TypeError('Failed to fetch');
        return reply(call);
      };
    const { calls } = mockApi({
      'GET /auth/session': reachable(() => json(session)),
      'POST /offline/sync': reachable(() =>
        json({
          serverTime: new Date().toISOString(),
          downloads: [downloadState()],
          revoked: [],
          profiles: [offlineProfile()],
          rules,
        }),
      ),
      'POST /offline/events': reachable((call) => {
        const count = (call.body as { events: unknown[] }).events.length;
        return json({
          accepted: count,
          duplicates: 0,
          rejected: 0,
          results: Array.from({ length: count }, () => 'accepted'),
        });
      }),
    });
    const { container } = renderApp(`/offline/play/${downloadIds.alien}`);
    const region = await screen.findByRole('main', { name: 'Videoplayer' });
    await waitFor(() => expect(region).toHaveAttribute('data-engine', 'mse'));
    await waitFor(() => {
      act(() => {
        container.querySelector('video')!.dispatchEvent(new Event('loadedmetadata'));
      });
      expect(region).toHaveAttribute('data-start-ms');
    });
    expect(media.paused).toBe(false);
    const startedAt = performance.now();

    // Up to 11 minutes on the fake clock, a second at a time, until the limit stops playback.
    let stoppedAfterMs: number | null = null;
    for (let second = 0; second < 660 && stoppedAfterMs === null; second += 1) {
      await act(() => vi.advanceTimersByTimeAsync(1000));
      if (region.getAttribute('data-ended-reason') === 'limit') {
        stoppedAfterMs = performance.now() - startedAt;
      }
    }

    // The watch time stopped playback at 10 min ±30 s, with the hint.
    expect(stoppedAfterMs).not.toBeNull();
    expect(Math.abs(stoppedAfterMs! - 10 * MINUTE_MS)).toBeLessThanOrEqual(30_000);
    expect(media.paused).toBe(true);
    expect(screen.getByRole('alert')).toHaveTextContent('Schauzeit für heute aufgebraucht');
    expect(screen.getByRole('alert')).toHaveTextContent(
      'Dieses Profil darf 60 Minuten pro Tag schauen.',
    );
    expect(screen.getByRole('button', { name: 'Abspielen' })).toBeDisabled();

    // About 60 events, 10 s apart, with seq 1, 2, 3, …; the pause at the limit comes last.
    await waitFor(() => expect(queuedEvents().at(-1)?.event.state).toBe('paused'));
    const events: OfflineEvent[] = queuedEvents().map((entry) => entry.event);
    expect(events.length).toBeGreaterThanOrEqual(59);
    expect(events.length).toBeLessThanOrEqual(62);
    expect(events.map((event) => event.seq)).toEqual(events.map((_, index) => index + 1));
    expect(new Set(events.map((event) => event.playbackId)).size).toBe(1);
    const playing = events.slice(0, -1);
    expect(playing.every((event) => event.state === 'playing')).toBe(true);
    for (let index = 1; index < playing.length; index += 1) {
      const step =
        Date.parse(playing[index]!.clientTime) - Date.parse(playing[index - 1]!.clientTime);
      expect(Math.abs(step - 10_000), `clientTime step ${index}`).toBeLessThanOrEqual(1000);
      const moved = playing[index]!.positionMs - playing[index - 1]!.positionMs;
      expect(Math.abs(moved - 10_000), `position step ${index}`).toBeLessThanOrEqual(1000);
    }
    const playedMs = events.reduce((sum, event) => sum + event.playedMs, 0);
    expect(Math.abs(playedMs - 10 * MINUTE_MS)).toBeLessThanOrEqual(5000);
    expect(Math.abs(events.at(-1)!.positionMs - 10 * MINUTE_MS)).toBeLessThanOrEqual(30_000);
    // The stored watch time booked the same play: nothing is left of the day.
    const watch = storedProfiles()[0]!.watch!;
    expect(watch.playedMs).toBe(playedMs);
    expect(remainingMs(watch, Date.now())).toBe(0);
    // Offline nothing went up.
    expect(calls.filter((call) => call.path === '/offline/events')).toEqual([]);

    // Reconnect: the engine syncs and uploads the whole queue in one batch with sentAt.
    network.up = true;
    onLine.mockReturnValue(true);
    onlineManager.setOnline(true);
    const reconnectedAt = Date.now();
    act(() => {
      window.dispatchEvent(new Event('online'));
    });
    await act(() => vi.advanceTimersByTimeAsync(500));
    await waitFor(() => expect(queuedEvents()).toHaveLength(0));
    const uploads = calls.filter((call) => call.path === '/offline/events');
    expect(uploads).toHaveLength(1);
    const batch = uploads[0]!.body as { sentAt: string; events: OfflineEvent[] };
    expect(batch.events).toEqual(events);
    const sentAt = Date.parse(batch.sentAt);
    expect(Number.isNaN(sentAt)).toBe(false);
    expect(sentAt).toBeGreaterThanOrEqual(reconnectedAt);
    expect(sentAt).toBeGreaterThanOrEqual(Date.parse(events.at(-1)!.clientTime));
  });
});
