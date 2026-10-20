import { onlineManager } from '@tanstack/react-query';
import { act, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { isUlid } from '@/lib/ulid';
import { mockApi, renderApp, signedIn } from '@/test/api';
import { ids } from '@/test/fixtures';
import { resetOfflineFiles, seedFile } from '@/test/offlineFiles';
import {
  downloadIds,
  episodeDownload,
  heatDownload,
  manifest,
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
  cancelFetch: vi.fn(() => Promise.resolve()),
  rememberDeleted: vi.fn(() => Promise.resolve()),
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
    instances: [] as {
      config: Record<string, unknown>;
      source?: string;
      stopped?: boolean;
      detached?: boolean;
    }[],
  };
  class FakeHls {
    static Events = { ERROR: 'hlsError' };
    static ErrorTypes = { NETWORK_ERROR: 'networkError', MEDIA_ERROR: 'mediaError' };
    static isSupported = () => state.supported;
    readonly record: (typeof state.instances)[number];
    constructor(config: Record<string, unknown>) {
      this.record = { config };
      state.instances.push(this.record);
    }
    on() {}
    loadSource(source: string) {
      this.record.source = source;
    }
    attachMedia() {}
    stopLoad() {
      this.record.stopped = true;
    }
    detachMedia() {
      this.record.detached = true;
    }
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
  await setMeta('accountId', 1);
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

/**
 * Starts playback once the player listens: its media effect runs after the commit that shows the
 * engine, so loadedmetadata is sent until the start is recorded.
 */
async function startPlaying(container: HTMLElement, region: HTMLElement) {
  await waitFor(() => expect(region).toHaveAttribute('data-engine'));
  await waitFor(() => {
    act(() => {
      container.querySelector('video')!.dispatchEvent(new Event('loadedmetadata'));
    });
    expect(region).toHaveAttribute('data-start-ms');
  });
}

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

  it('starts at the newer server position the sync stored (W-5, O-5)', async () => {
    stubMedia(90);
    await putDownloads([
      storedDownload({
        manifest: { ...manifest(), durationMs: 90_000 },
        positions: { '1': { positionMs: 12_000, at: 1000 } },
        serverPositions: { '1': { positionMs: 30_000, completed: false, at: 2000 } },
      }),
    ]);
    mockApi({});
    renderApp(`/offline/play/${downloadIds.alien}`);
    const region = await player();
    await waitFor(() => expect(region).toHaveAttribute('data-engine', 'mse'));
    expect(hls.state.instances[0]?.config.startPosition).toBe(30);
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

  it('after the limit nothing plays on: media keys pause again, hls.js lets go (W5)', async () => {
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
          playedMs: 55_000,
        },
      }),
    ]);
    mockApi({});
    const { container } = renderApp(`/offline/play/${downloadIds.alien}`);
    const region = await player();
    await waitFor(() => expect(region).toHaveAttribute('data-engine', 'mse'));
    const video = container.querySelector('video')!;
    expect(video).toHaveAttribute('disablepictureinpicture');
    await startPlaying(container, region);
    media.currentTime = 3;
    await act(() => vi.advanceTimersByTimeAsync(6_000));
    expect(region).toHaveAttribute('data-ended-reason', 'limit');
    expect(media.paused).toBe(true);
    expect(hls.state.instances[0]).toMatchObject({ stopped: true, detached: true });
    // The media notification or a media key plays the element: it pauses at once.
    act(() => {
      media.paused = false;
      video.dispatchEvent(new Event('play'));
    });
    expect(media.paused).toBe(true);
    // The pause at the limit booked the position before hls.js let go.
    const paused = queuedEvents().filter((entry) => entry.event.state === 'paused');
    expect(paused[0]?.event.positionMs).toBe(3000);
  });

  it('leaving the player while it plays books the last stretch (W6)', async () => {
    const media = stubMedia();
    mockApi({});
    const { container, router } = renderApp(`/offline/play/${downloadIds.alien}`);
    const region = await player();
    await startPlaying(container, region);
    media.currentTime = 3;
    // Less than the 10-s interval: only the leave books it.
    await act(() => vi.advanceTimersByTimeAsync(4_000));
    expect(queuedEvents()).toHaveLength(0);
    await act(() => router.navigate({ to: '/offline' }));
    await waitFor(() => expect(queuedEvents()).toHaveLength(1));
    expect(queuedEvents()[0]?.event).toMatchObject({ state: 'paused', positionMs: 3000 });
    expect(queuedEvents()[0]!.event.playedMs).toBeGreaterThanOrEqual(3_500);
  });

  it('another download in the same route is another playback (K10)', async () => {
    const media = stubMedia();
    await putDownloads([episodeDownload('e1')]);
    seedFile(downloadIds.e1, 'media.mp4', new Uint8Array(9000));
    mockApi({});
    const { container, router } = renderApp(`/offline/play/${downloadIds.alien}`);
    let region = await player();
    await waitFor(() => expect(region).toHaveAttribute('data-download-id', downloadIds.alien));
    await startPlaying(container, region);
    await act(() =>
      router.navigate({
        to: '/offline/play/$downloadId',
        params: { downloadId: downloadIds.e1 },
      }),
    );
    region = await player();
    await waitFor(() => expect(region).toHaveAttribute('data-download-id', downloadIds.e1));
    await startPlaying(container, region);
    act(() => container.querySelector('video')!.pause());
    await waitFor(() => expect(queuedEvents()).toHaveLength(2));
    const [first, second] = queuedEvents().map((entry) => entry.event);
    expect(first?.downloadId).toBe(downloadIds.alien);
    expect(second?.downloadId).toBe(downloadIds.e1);
    expect(second?.playbackId).not.toBe(first?.playbackId);
    expect(second?.seq).toBe(1);
    expect(media.paused).toBe(true);
  });

  it('from /downloads it plays for the session’s profile, not the library’s choice (W1)', async () => {
    stubMedia();
    // Profile 2 was opened in the offline library; the session's active profile is 1.
    chooseOfflineProfile({ accountId: 1, profileId: 2 });
    mockApi(signedIn);
    renderApp(`/offline/play/${downloadIds.alien}?as=session`);
    const region = await player();
    expect(region).toHaveAttribute('data-profile-id', '1');
  });

  it('from /downloads a title the session’s profile may not see stays closed (W1)', async () => {
    stubMedia();
    chooseOfflineProfile({ accountId: 1, profileId: 2 });
    mockApi(signedIn);
    renderApp(`/offline/play/${downloadIds.heat}?as=session`);
    expect(await screen.findByText('Download nicht verfügbar')).toBeVisible();
  });

  it('the library’s choice does not open the downloads of a signed-out account (W2)', async () => {
    stubMedia();
    await setMeta('signedOut', true);
    mockApi({});
    renderApp(`/offline/play/${downloadIds.alien}`);
    expect(await screen.findByText('Download nicht verfügbar')).toBeVisible();
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
