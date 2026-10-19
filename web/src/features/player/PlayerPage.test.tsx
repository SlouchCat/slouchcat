import type { ProgressEvent } from '@slouchcat/api-client';
import { act, fireEvent, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { alienDetail, hlsPlayback, ids, playback } from '@/test/fixtures';
import { json, mockApi, renderApp, signedIn, type Call } from '@/test/api';

const progressRoute = `POST /items/${ids.alien}/progress`;

// hls.js is mocked: jsdom has no MSE. The mock records how the player drives it.
const hlsMock = vi.hoisted(() => {
  const instances: { source?: string; media?: unknown; config: unknown; destroyed: boolean }[] = [];
  class FakeHls {
    static Events = { ERROR: 'hlsError' };
    readonly record: (typeof instances)[number];
    constructor(config: unknown) {
      this.record = { config, destroyed: false };
      instances.push(this.record);
    }
    on() {}
    loadSource(source: string) {
      this.record.source = source;
    }
    attachMedia(media: unknown) {
      this.record.media = media;
    }
    destroy() {
      this.record.destroyed = true;
    }
  }
  return { instances, FakeHls };
});
vi.mock('hls.js/light', () => ({ default: hlsMock.FakeHls }));

function routes(info = playback) {
  return {
    ...signedIn,
    [`POST /items/${ids.alien}/play`]: () => json(info),
    'DELETE /playback/sessions/*': () => new Response(null, { status: 204 }),
    [progressRoute]: () =>
      json({ positionMs: 0, durationMs: 0, percent: 0, completed: false, updatedAt: '' }),
    [`GET /items/${ids.alien}`]: () => json(alienDetail),
  };
}

const progressEvents = (calls: Call[]) =>
  calls
    .filter((call) => call.path === `/items/${ids.alien}/progress`)
    .map((call) => call.body as ProgressEvent);

/** jsdom has no media pipeline: model the element state the player reads. */
function stubMedia(canPlay: CanPlayTypeResult = 'probably') {
  const state = { paused: true, ended: false, currentTime: 0, duration: 7020 };
  const canPlayType = vi.spyOn(HTMLMediaElement.prototype, 'canPlayType').mockReturnValue(canPlay);
  vi.spyOn(HTMLMediaElement.prototype, 'play').mockImplementation(function (
    this: HTMLMediaElement,
  ) {
    state.paused = false;
    this.dispatchEvent(new Event('play'));
    this.dispatchEvent(new Event('playing'));
    return Promise.resolve();
  });
  vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(function (
    this: HTMLMediaElement,
  ) {
    state.paused = true;
    this.dispatchEvent(new Event('pause'));
  });
  vi.spyOn(HTMLMediaElement.prototype, 'paused', 'get').mockImplementation(() => state.paused);
  vi.spyOn(HTMLMediaElement.prototype, 'ended', 'get').mockImplementation(() => state.ended);
  vi.spyOn(HTMLMediaElement.prototype, 'duration', 'get').mockImplementation(() => state.duration);
  vi.spyOn(HTMLMediaElement.prototype, 'currentTime', 'get').mockImplementation(
    () => state.currentTime,
  );
  vi.spyOn(HTMLMediaElement.prototype, 'currentTime', 'set').mockImplementation((value: number) => {
    state.currentTime = value;
  });
  return { state, canPlayType };
}

describe('player', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('starts at the resume position, measures the start and reports progress', async () => {
    const { state: media } = stubMedia();
    const { calls } = mockApi(routes());
    const { container } = renderApp(`/play/${ids.alien}`);

    const video = await waitFor(() => {
      const element = container.querySelector('video');
      if (!element) throw new Error('no video yet');
      return element;
    });
    expect(video).toHaveAttribute('src', `/api/v1/items/${ids.alien}/stream?st=ps-1.token`);
    expect(video).toHaveAttribute('preload', 'auto');
    // The client reports what it can play (jsdom: everything the stub allows, no MSE).
    const start = calls.find((call) => call.method === 'POST' && call.path.endsWith('/play'));
    expect(start?.body).toEqual({
      capabilities: { video: ['h264', 'hevc'], audio: ['aac', 'ac3', 'eac3'], hls: 'native' },
    });

    act(() => {
      video.dispatchEvent(new Event('loadedmetadata'));
    });
    expect(media.currentTime).toBeCloseTo(2533);
    const region = screen.getByRole('main', { name: 'Videoplayer' });
    await waitFor(() => expect(region).toHaveAttribute('data-start-ms'));

    // Every 10 s while playing.
    media.currentTime = 2543;
    await act(() => vi.advanceTimersByTimeAsync(10_000));
    media.currentTime = 2553;
    await act(() => vi.advanceTimersByTimeAsync(10_000));

    // Pause, then the end.
    act(() => video.pause());
    media.currentTime = 7020;
    media.ended = true;
    act(() => {
      video.dispatchEvent(new Event('pause'));
      video.dispatchEvent(new Event('ended'));
    });

    await waitFor(() => expect(progressEvents(calls)).toHaveLength(4));
    const events = progressEvents(calls);
    expect(events.map((event) => event.state)).toEqual(['playing', 'playing', 'paused', 'ended']);
    expect(events.map((event) => event.seq)).toEqual([1, 2, 3, 4]);
    expect(events[0]).toMatchObject({
      mediaFileId: 'mf-1',
      positionMs: 2_543_000,
      durationMs: 7_020_000,
    });
    expect(events[0]!.playedMs).toBeGreaterThanOrEqual(9_900);
    expect(events[0]!.playedMs).toBeLessThanOrEqual(10_500);
    expect(events[3]!.positionMs).toBe(7_020_000);
    expect(() => new Date(events[0]!.clientTime).toISOString()).not.toThrow();
  });

  it('reports with keepalive when the page is hidden', async () => {
    stubMedia();
    const { calls } = mockApi(routes());
    const { container } = renderApp(`/play/${ids.alien}`);
    const video = await waitFor(() => {
      const element = container.querySelector('video');
      if (!element) throw new Error('no video yet');
      return element;
    });
    act(() => {
      video.dispatchEvent(new Event('loadedmetadata'));
    });
    vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('hidden');
    act(() => {
      document.dispatchEvent(new Event('visibilitychange'));
    });
    await waitFor(() => expect(progressEvents(calls)).toHaveLength(1));
    const call = calls.find((entry) => entry.path.endsWith('/progress'));
    expect(call?.keepalive).toBe(true);
    expect((call?.body as ProgressEvent).state).toBe('playing');
  });

  it('toggles playback with Space and seeks with the arrow keys', async () => {
    const { state: media } = stubMedia();
    mockApi(routes());
    const { container } = renderApp(`/play/${ids.alien}`);
    const video = await waitFor(() => {
      const element = container.querySelector('video');
      if (!element) throw new Error('no video yet');
      return element;
    });
    act(() => {
      video.dispatchEvent(new Event('loadedmetadata'));
    });
    expect(media.paused).toBe(false);
    fireEvent.keyDown(document.body, { key: ' ' });
    expect(media.paused).toBe(true);
    fireEvent.keyDown(document.body, { key: 'ArrowRight' });
    expect(media.currentTime).toBeCloseTo(2543);
    fireEvent.keyDown(document.body, { key: 'ArrowLeft' });
    fireEvent.keyDown(document.body, { key: 'ArrowLeft' });
    expect(media.currentTime).toBeCloseTo(2523);
  });

  it('shows a notice instead of a broken player when the browser cannot play the file', async () => {
    const { canPlayType } = stubMedia('');
    mockApi(routes());
    const { container } = renderApp(`/play/${ids.alien}`);
    expect(await screen.findByText('Format nicht abspielbar')).toBeVisible();
    expect(screen.getByText('Dieser Browser kann dieses Format nicht abspielen.')).toBeVisible();
    expect(container.querySelector('video')).toBeNull();
    expect(canPlayType).toHaveBeenCalledWith('video/mp4; codecs="avc1.640028, mp4a.40.2"');
  });

  it('plays HLS natively when the element supports it (Safari)', async () => {
    stubMedia();
    mockApi(routes(hlsPlayback));
    const { container } = renderApp(`/play/${ids.alien}`);
    const video = await waitFor(() => {
      const element = container.querySelector('video');
      if (!element) throw new Error('no video yet');
      return element;
    });
    expect(video).toHaveAttribute('src', hlsPlayback.streamUrl);
    expect(screen.getByRole('main', { name: 'Videoplayer' })).toHaveAttribute(
      'data-engine',
      'native',
    );
    expect(hlsMock.instances).toHaveLength(0);
  });

  it('plays HLS through hls.js with MSE, from the resume position', async () => {
    // Chromium/Firefox: no native HLS, but MSE.
    vi.spyOn(HTMLMediaElement.prototype, 'canPlayType').mockImplementation((type) =>
      type === 'application/vnd.apple.mpegurl' ? '' : 'probably',
    );
    vi.stubGlobal('MediaSource', { isTypeSupported: () => true });
    mockApi(routes(hlsPlayback));
    const { container } = renderApp(`/play/${ids.alien}`);
    await waitFor(() => expect(hlsMock.instances).toHaveLength(1));
    const video = container.querySelector('video');
    expect(video).not.toHaveAttribute('src');
    const hls = hlsMock.instances[0]!;
    expect(hls.source).toBe(hlsPlayback.streamUrl);
    expect(hls.media).toBe(video);
    expect(hls.config).toMatchObject({ startPosition: 2533 });
    expect(screen.getByRole('main', { name: 'Videoplayer' })).toHaveAttribute(
      'data-engine',
      'hlsjs',
    );
  });

  it('measures a seek until playback continues', async () => {
    const { state: media } = stubMedia();
    mockApi(routes());
    const { container } = renderApp(`/play/${ids.alien}`);
    const video = await waitFor(() => {
      const element = container.querySelector('video');
      if (!element) throw new Error('no video yet');
      return element;
    });
    act(() => {
      video.dispatchEvent(new Event('loadedmetadata'));
    });
    const region = screen.getByRole('main', { name: 'Videoplayer' });
    expect(region).not.toHaveAttribute('data-seek-ms');
    media.currentTime = 3000;
    act(() => {
      video.dispatchEvent(new Event('seeking'));
    });
    await act(() => vi.advanceTimersByTimeAsync(250));
    act(() => {
      video.dispatchEvent(new Event('seeked'));
      video.dispatchEvent(new Event('playing'));
    });
    expect(Number(region.getAttribute('data-seek-ms'))).toBeGreaterThanOrEqual(200);
    expect(region).toHaveAttribute('data-seek-count', '1');
  });

  it('ends the playback session when the player is left', async () => {
    stubMedia();
    // An id of its own: stops of earlier tests' players may still arrive.
    const { calls } = mockApi(routes({ ...playback, playbackSessionId: 'ps-leave' }));
    const stops = () => calls.filter((call) => call.path === '/playback/sessions/ps-leave');
    const { router, container } = renderApp(`/play/${ids.alien}`);
    await waitFor(() => expect(container.querySelector('video')).not.toBeNull());
    await act(() => vi.advanceTimersByTimeAsync(50));
    expect(stops()).toEqual([]);
    fireEvent.keyDown(document.body, { key: 'Escape' });
    await waitFor(() => expect(router.state.location.pathname).toBe(`/items/${ids.alien}`));
    await waitFor(() =>
      expect(stops()).toEqual([
        { method: 'DELETE', path: '/playback/sessions/ps-leave', body: null, keepalive: true },
      ]),
    );
  });

  it('returns to the detail page with Escape', async () => {
    stubMedia();
    mockApi(routes());
    const { router, container } = renderApp(`/play/${ids.alien}`);
    await waitFor(() => expect(container.querySelector('video')).not.toBeNull());
    fireEvent.keyDown(document.body, { key: 'Escape' });
    await waitFor(() => expect(router.state.location.pathname).toBe(`/items/${ids.alien}`));
  });
});
