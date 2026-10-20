import { onlineManager } from '@tanstack/react-query';
import { act, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { isUlid } from '@/lib/ulid';
import { mockApi, renderApp } from '@/test/api';
import { ids } from '@/test/fixtures';
import { resetOfflineFiles, seedFile } from '@/test/offlineFiles';
import {
  downloadIds,
  heatDownload,
  offlineProfile,
  storedDownload,
  storedProfile,
} from '@/test/offlineFixtures';
import {
  putDownloads,
  putProfiles,
  queuedEvents,
  resetOfflineStore,
  setMeta,
  storedDownloads,
} from '@/test/offlineStore';
import { chooseOfflineProfile, leaveOfflineProfile } from './offlineSession';
import { timelineOffset } from './OfflinePlayer';

vi.mock('@/features/offline/store', () => import('@/test/offlineStore'));
vi.mock('@/features/offline/opfs', () => import('@/test/offlineFiles'));
const engine = vi.hoisted(() => ({ flush: vi.fn() }));
vi.mock('@/features/offline/engine', () => ({
  startOfflineEngine: () => () => undefined,
  kickOfflineEngine: vi.fn(),
  cancelFetch: vi.fn(),
  flushEventsSoon: engine.flush,
}));
vi.mock('@/features/offline/support', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./support')>()),
  offlineSupported: () => true,
}));

// hls.js is mocked (jsdom has no MSE); `supported` switches to the blob fallback.
const hls = vi.hoisted(() => {
  const state = {
    supported: true,
    instances: [] as { config: Record<string, unknown>; source?: string }[],
  };
  class FakeHls {
    static Events = { ERROR: 'hlsError' };
    static ErrorTypes = { NETWORK_ERROR: 'networkError', MEDIA_ERROR: 'mediaError' };
    static isSupported = () => state.supported;
    readonly record: { config: Record<string, unknown>; source?: string };
    constructor(config: Record<string, unknown>) {
      this.record = { config };
      state.instances.push(this.record);
    }
    on() {}
    loadSource(source: string) {
      this.record.source = source;
    }
    attachMedia() {}
    recoverMediaError() {}
    destroy() {}
  }
  return { state, FakeHls };
});
vi.mock('hls.js/light', () => ({ default: hls.FakeHls }));

function stubMedia(duration = 8) {
  const media = { paused: true, ended: false, currentTime: 0, duration };
  vi.spyOn(HTMLMediaElement.prototype, 'play').mockImplementation(function (
    this: HTMLMediaElement,
  ) {
    media.paused = false;
    this.dispatchEvent(new Event('playing'));
    return Promise.resolve();
  });
  vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(function (
    this: HTMLMediaElement,
  ) {
    media.paused = true;
    this.dispatchEvent(new Event('pause'));
  });
  vi.spyOn(HTMLMediaElement.prototype, 'paused', 'get').mockImplementation(() => media.paused);
  vi.spyOn(HTMLMediaElement.prototype, 'ended', 'get').mockImplementation(() => media.ended);
  vi.spyOn(HTMLMediaElement.prototype, 'duration', 'get').mockImplementation(() => media.duration);
  vi.spyOn(HTMLMediaElement.prototype, 'currentTime', 'get').mockImplementation(
    () => media.currentTime,
  );
  vi.spyOn(HTMLMediaElement.prototype, 'currentTime', 'set').mockImplementation((value: number) => {
    media.currentTime = value;
  });
  return media;
}

beforeEach(async () => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  resetOfflineStore();
  resetOfflineFiles();
  hls.state.supported = true;
  hls.state.instances.length = 0;
  engine.flush.mockClear();
  vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(false);
  onlineManager.setOnline(false);
  URL.createObjectURL = vi.fn(() => 'blob:offline-test');
  URL.revokeObjectURL = vi.fn();
  await putDownloads([storedDownload(), heatDownload()]);
  await putProfiles([storedProfile()]);
  await setMeta('serverOffsetMs', 0);
  seedFile(downloadIds.alien, 'media.mp4', new Uint8Array(9000));
  seedFile(downloadIds.alien, '21.vtt', 'WEBVTT\n');
  chooseOfflineProfile({ accountId: 1, profileId: 1 });
});

afterEach(() => {
  leaveOfflineProfile();
  vi.useRealTimers();
  vi.restoreAllMocks();
  onlineManager.setOnline(true);
});

const player = () => screen.findByRole('main', { name: 'Videoplayer' });

describe('offline player', () => {
  it('plays from OPFS through hls.js with the generated playlist and queues the progress', async () => {
    const media = stubMedia();
    mockApi({});
    const { container } = renderApp(`/offline/play/${downloadIds.alien}`);
    const region = await player();
    await waitFor(() => expect(region).toHaveAttribute('data-engine', 'mse'));
    const instance = hls.state.instances[0]!;
    expect(instance.source).toMatch(
      new RegExp(`/offline/media/${downloadIds.alien}/index\\.m3u8$`),
    );
    expect(typeof instance.config.loader).toBe('function');
    expect(screen.getByRole('heading', { name: 'Alien' })).toBeVisible();
    // The stored subtitle is a <track> of a blob URL, with a local menu.
    await waitFor(() =>
      expect(container.querySelector('track')).toHaveAttribute('src', 'blob:offline-test'),
    );
    expect(screen.getByRole('button', { name: 'Untertitel' })).toBeVisible();

    const video = container.querySelector('video')!;
    act(() => {
      video.dispatchEvent(new Event('loadedmetadata'));
    });
    await waitFor(() => expect(region).toHaveAttribute('data-start-ms'));
    media.currentTime = 4;
    await act(() => vi.advanceTimersByTimeAsync(10_000));
    act(() => video.pause());
    await waitFor(() => expect(queuedEvents()).toHaveLength(2));
    const events = queuedEvents().map((entry) => entry.event);
    expect(events.map((event) => event.state)).toEqual(['playing', 'paused']);
    expect(events.map((event) => event.seq)).toEqual([1, 2]);
    expect(events[0]).toMatchObject({
      profileId: 1,
      itemId: ids.alien,
      downloadId: downloadIds.alien,
      positionMs: 4000,
      durationMs: 8000,
    });
    expect(isUlid(events[0]!.playbackId)).toBe(true);
    expect(events[1]!.playbackId).toBe(events[0]!.playbackId);
    expect(events[0]!.playedMs).toBeGreaterThanOrEqual(9_500);
    // The position stays on the download for this profile; online the queue goes up at once.
    await waitFor(() =>
      expect(
        storedDownloads().find((download) => download.id === downloadIds.alien)?.positions?.['1']
          ?.positionMs,
      ).toBe(4000),
    );
    expect(engine.flush).toHaveBeenCalled();
  });

  it('stops with the hint when the offline watch time is used up (AN-18)', async () => {
    stubMedia();
    const now = Date.now();
    await putProfiles([
      storedProfile({
        snapshot: offlineProfile({
          watchTime: {
            limitMinutes: 1,
            remainingMinutes: 0,
            resetsAt: new Date(now + 3_600_000).toISOString(),
          },
        }),
        watch: {
          limitMinutes: 1,
          remainingMinutes: 0,
          resetsAt: now + 3_600_000,
          syncedAt: now,
          period: -1,
          playedMs: 0,
        },
      }),
    ]);
    mockApi({});
    renderApp(`/offline/play/${downloadIds.alien}`);
    const region = await player();
    expect(region).toHaveAttribute('data-ended-reason', 'limit');
    expect(screen.getByRole('alert')).toHaveTextContent('Schauzeit für heute aufgebraucht');
    expect(screen.getByRole('alert')).toHaveTextContent(
      'Dieses Profil darf 1 Minute pro Tag schauen.',
    );
    expect(screen.getByRole('button', { name: 'Abspielen' })).toBeDisabled();
  });

  it('stops while playing once the rest of the day is played', async () => {
    const media = stubMedia();
    const now = Date.now();
    await putProfiles([
      storedProfile({
        watch: {
          limitMinutes: 60,
          remainingMinutes: 1,
          resetsAt: now + 3_600_000,
          syncedAt: now,
          period: -1,
          playedMs: 30_000,
        },
      }),
    ]);
    mockApi({});
    const { container } = renderApp(`/offline/play/${downloadIds.alien}`);
    const region = await player();
    await waitFor(() => expect(region).toHaveAttribute('data-engine', 'mse'));
    act(() => {
      container.querySelector('video')!.dispatchEvent(new Event('loadedmetadata'));
    });
    await waitFor(() => expect(region).toHaveAttribute('data-start-ms'));
    expect(media.paused).toBe(false);
    await act(() => vi.advanceTimersByTimeAsync(31_000));
    expect(region).toHaveAttribute('data-ended-reason', 'limit');
    expect(media.paused).toBe(true);
  });

  it('plays the whole file from a blob URL without MediaSource, on the source timeline', async () => {
    hls.state.supported = false;
    // The file's timeline starts 10 s late (download fragments keep the HLS timeline).
    const media = stubMedia(18);
    mockApi({});
    const { container } = renderApp(`/offline/play/${downloadIds.alien}`);
    const region = await player();
    await waitFor(() => expect(region).toHaveAttribute('data-engine', 'blob'));
    const video = container.querySelector('video')!;
    await waitFor(() => expect(video).toHaveAttribute('src', 'blob:offline-test'));
    act(() => {
      video.dispatchEvent(new Event('loadedmetadata'));
    });
    expect(media.currentTime).toBe(10);
    media.currentTime = 13;
    await act(() => vi.advanceTimersByTimeAsync(10_000));
    await waitFor(() => expect(queuedEvents()).toHaveLength(1));
    expect(queuedEvents()[0]?.event.positionMs).toBe(3000);
  });

  it('refuses a download the profile may not see (allowedProfileIds)', async () => {
    stubMedia();
    mockApi({});
    renderApp(`/offline/play/${downloadIds.heat}`);
    expect(await screen.findByText('Download nicht verfügbar')).toBeVisible();
  });
});

describe('timelineOffset', () => {
  const ranges = (...list: [number, number][]) => ({
    length: list.length,
    start: (index: number) => list[index]![0],
    end: (index: number) => list[index]![1],
  });

  it('takes the first buffered or seekable time, else the extra duration', () => {
    expect(
      timelineOffset({ buffered: ranges([10, 14]), seekable: ranges(), duration: 18 }, 8000),
    ).toBe(10);
    expect(
      timelineOffset({ buffered: ranges(), seekable: ranges([10, 18]), duration: 18 }, 8000),
    ).toBe(10);
    expect(
      timelineOffset({ buffered: ranges(), seekable: ranges([0, 18]), duration: 18 }, 8000),
    ).toBe(10);
    expect(
      timelineOffset({ buffered: ranges([0, 4]), seekable: ranges([0, 8]), duration: 8 }, 8000),
    ).toBe(0);
  });
});
