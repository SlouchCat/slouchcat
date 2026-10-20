import type { PlaybackInfo, PlaybackRequest, Progress, TrackList } from '@slouchcat/api-client';
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { alienDetail, alienTracks, episodes, hlsPlayback, ids, playback } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, signedIn, type Call } from '@/test/api';

// M4 player: tracks (W-4, I-5), quality (W-8), chapters, intro and auto-next (W-9), the limits
// (K-5, W-8) and a stream the admin ended (W-6). The server decides; the player shows its answers.

const hlsMock = vi.hoisted(() => {
  type ErrorData = { type: string; details: string; fatal: boolean; response?: { code: number } };
  const instances: { emitError: (data: ErrorData) => void; stopped: number }[] = [];
  class FakeHls {
    static Events = { ERROR: 'hlsError' };
    static ErrorTypes = { NETWORK_ERROR: 'networkError', MEDIA_ERROR: 'mediaError' };
    private readonly handlers: ((event: string, data: ErrorData) => void)[] = [];
    readonly record = {
      emitError: (data: ErrorData) => this.handlers.forEach((handler) => handler('hlsError', data)),
      stopped: 0,
    };
    constructor() {
      instances.push(this.record);
    }
    on(event: string, handler: (event: string, data: ErrorData) => void) {
      if (event === 'hlsError') this.handlers.push(handler);
    }
    recoverMediaError() {}
    startLoad() {}
    stopLoad() {
      this.record.stopped += 1;
    }
    loadSource() {}
    attachMedia() {}
    destroy() {}
  }
  return { instances, FakeHls };
});
vi.mock('hls.js/light', () => ({ default: hlsMock.FakeHls }));

const playPath = `/items/${ids.alien}/play`;
const tracksPath = `/items/${ids.alien}/tracks`;

const progressAnswer = (extra: Partial<Progress> = {}): Progress => ({
  positionMs: 0,
  durationMs: 0,
  percent: 0,
  completed: false,
  updatedAt: '',
  ...extra,
});

function stubMedia() {
  const state = { paused: true, ended: false, currentTime: 0, duration: 7020 };
  vi.spyOn(HTMLMediaElement.prototype, 'canPlayType').mockReturnValue('probably');
  vi.spyOn(HTMLMediaElement.prototype, 'play').mockImplementation(function (
    this: HTMLMediaElement,
  ) {
    state.paused = false;
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
  return state;
}

/** Answers startPlayback with `first`, every later start with `answer(body)`. */
function routes(
  first: PlaybackInfo,
  answer: (body: PlaybackRequest, count: number) => PlaybackInfo = () => first,
  extra: Record<string, (call: Call) => Response> = {},
) {
  let count = 0;
  return {
    ...signedIn,
    [`POST ${playPath}`]: (call: Call) => {
      count += 1;
      return json(count === 1 ? first : answer(call.body as PlaybackRequest, count));
    },
    'DELETE /playback/sessions/*': () => new Response(null, { status: 204 }),
    [`POST /items/${ids.alien}/progress`]: () => json(progressAnswer()),
    [`GET /items/${ids.alien}`]: () => json(alienDetail),
    ...extra,
  };
}

const starts = (calls: Call[]) =>
  calls.filter((call) => call.method === 'POST' && call.path === playPath);

async function startPlayer(container: HTMLElement) {
  const video = await waitFor(() => {
    const element = container.querySelector('video');
    if (!element) throw new Error('no video yet');
    return element;
  });
  act(() => {
    video.dispatchEvent(new Event('loadedmetadata'));
  });
  return video;
}

const region = () => screen.getByRole('main', { name: 'Videoplayer' });

describe('player M4', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    hlsMock.instances.length = 0;
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('T opens the track menu, which asks for the tracks again; a pick starts a new session', async () => {
    const media = stubMedia();
    const english: PlaybackInfo = {
      ...playback,
      playbackSessionId: 'ps-en',
      streamUrl: `/items/${ids.alien}/stream?st=ps-en.token`,
      selection: { audioTrack: 2 },
    };
    const sidecar: TrackList = {
      ...alienTracks,
      subtitles: [
        ...alienTracks.subtitles,
        {
          id: 30,
          index: -1,
          kind: 'subtitle',
          codec: 'subrip',
          language: 'de',
          default: false,
          forced: false,
          source: 'sidecar',
          text: true,
        },
      ],
    };
    const { calls } = mockApi(
      routes(playback, () => english, { [`GET ${tracksPath}`]: () => json(sidecar) }),
    );
    const { container } = renderApp(`/play/${ids.alien}`);
    await startPlayer(container);
    media.currentTime = 3000;

    fireEvent.keyDown(document.body, { key: 't' });
    const menu = await screen.findByRole('menu', { name: 'Spuren' });
    expect(within(menu).getByText('Ton')).toBeVisible();
    expect(within(menu).getByText('Untertitel')).toBeVisible();
    expect(
      within(menu).getByRole('menuitemradio', { name: 'Deutsch · AAC · 2.0' }),
    ).toHaveAttribute('aria-checked', 'true');
    expect(within(menu).getByRole('menuitemradio', { name: 'Aus' })).toHaveAttribute(
      'aria-checked',
      'true',
    );
    // I-5: the sidecar the server now lists shows up in the open menu.
    await waitFor(() =>
      expect(
        within(menu).getByRole('menuitemradio', { name: 'Deutsch · SRT · extern' }),
      ).toBeVisible(),
    );
    const trackCall = calls.find((call) => call.path === tracksPath);
    expect(trackCall?.method).toBe('GET');

    const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
    await user.click(within(menu).getByRole('menuitemradio', { name: 'Englisch · AC3 · 5.1' }));
    await waitFor(() => expect(starts(calls)).toHaveLength(2));
    expect(starts(calls)[1]?.body).toMatchObject({ audioTrack: 2, positionMs: 3_000_000 });
    // The old session ends before the next one starts.
    const stop = calls.findIndex(
      (call) => call.method === 'DELETE' && call.path === '/playback/sessions/ps-1',
    );
    expect(stop).toBeGreaterThan(-1);
    expect(stop).toBeLessThan(calls.indexOf(starts(calls)[1]!));

    await waitFor(() => expect(region()).toHaveAttribute('data-playback-session-id', 'ps-en'));
    await startPlayer(container);
    expect(Number(region().getAttribute('data-switch-ms'))).toBeGreaterThanOrEqual(0);
    // The new session starts where the viewer was.
    expect(media.currentTime).toBeCloseTo(3000);
  });

  it('asks for new tracks every 30 s while the track menu is open', async () => {
    stubMedia();
    const { calls } = mockApi(
      routes(playback, undefined, { [`GET ${tracksPath}`]: () => json(alienTracks) }),
    );
    const { container } = renderApp(`/play/${ids.alien}`);
    await startPlayer(container);
    const trackCalls = () => calls.filter((call) => call.path === tracksPath);
    expect(trackCalls()).toHaveLength(0);
    fireEvent.keyDown(document.body, { key: 'T' });
    await screen.findByRole('menu', { name: 'Spuren' });
    await waitFor(() => expect(trackCalls()).toHaveLength(1));
    await act(() => vi.advanceTimersByTimeAsync(30_000));
    await waitFor(() => expect(trackCalls()).toHaveLength(2));
  });

  it('switches subtitles off and on: null for off, the track id for a text track', async () => {
    stubMedia();
    const withText: PlaybackInfo = {
      ...playback,
      playbackSessionId: 'ps-sub',
      selection: { audioTrack: 1, subtitleTrack: 21, subtitleDelivery: 'text' },
      subtitleUrl: '/playback/sessions/ps-sub/subtitles/21.vtt?st=ps-sub.token',
    };
    const { calls } = mockApi(routes(playback, () => withText));
    const { container } = renderApp(`/play/${ids.alien}`);
    await startPlayer(container);
    const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
    await user.click(screen.getByRole('button', { name: 'Spuren' }));
    await user.click(await screen.findByRole('menuitemradio', { name: 'Deutsch (SRT)' }));
    await waitFor(() => expect(starts(calls)).toHaveLength(2));
    expect(starts(calls)[1]?.body).toMatchObject({ subtitleTrack: 21, subtitleDelivery: 'auto' });

    // The text track renders through <track>, for direct play as for hls.js.
    await waitFor(() => expect(region()).toHaveAttribute('data-playback-session-id', 'ps-sub'));
    const track = container.querySelector('video track');
    expect(track).toHaveAttribute(
      'src',
      '/api/v1/playback/sessions/ps-sub/subtitles/21.vtt?st=ps-sub.token',
    );
    expect(track).toHaveAttribute('kind', 'subtitles');
    expect(track).toHaveAttribute('srclang', 'ger');
    expect(track).toHaveAttribute('label', 'Deutsch (SRT)');

    fireEvent.keyDown(document.body, { key: 'i' });
    const info = screen.getByRole('region', { name: 'Stream-Info' });
    expect(within(info).getByText('Untertitel als').nextElementSibling).toHaveTextContent(
      'Text (WebVTT)',
    );
  });

  it('Q opens the quality menu: unavailable rungs are disabled, a pick asks for that rung', async () => {
    stubMedia();
    const sd: PlaybackInfo = {
      ...hlsPlayback,
      playbackSessionId: 'ps-480',
      transcoding: {
        mode: 'transcode',
        backend: 'nvenc',
        hardware: true,
        quality: '480p',
        width: 854,
        height: 480,
        bitrate: 2_000_000,
        reason: 'quality',
      },
    };
    const { calls } = mockApi(routes(playback, () => sd));
    const { container } = renderApp(`/play/${ids.alien}`);
    await startPlayer(container);
    expect(screen.getByRole('button', { name: 'Qualität: Automatisch' })).toBeVisible();

    fireEvent.keyDown(document.body, { key: 'q' });
    const menu = await screen.findByRole('menu', { name: /^Qualität/ });
    expect(within(menu).getByRole('menuitemradio', { name: 'Automatisch' })).toHaveAttribute(
      'aria-checked',
      'true',
    );
    expect(within(menu).getByRole('menuitemradio', { name: '360p' })).toHaveAttribute(
      'aria-disabled',
      'true',
    );
    const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
    await user.click(within(menu).getByRole('menuitemradio', { name: '480p' }));
    await waitFor(() => expect(starts(calls)).toHaveLength(2));
    expect(starts(calls)[1]?.body).toMatchObject({ quality: '480p' });
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Qualität: 480p' })).toBeVisible(),
    );

    fireEvent.keyDown(document.body, { key: 'i' });
    const info = screen.getByRole('region', { name: 'Stream-Info' });
    expect(within(info).getByText('Encoder').nextElementSibling).toHaveTextContent(
      'Hardware (nvenc)',
    );
    expect(within(info).getByText('Grund').nextElementSibling).toHaveTextContent(
      'Gewählte Qualität',
    );
    expect(within(info).getByText('Ausgabe').nextElementSibling).toHaveTextContent('854 × 480');
  });

  it('C opens the chapters; a pick jumps to the chapter start', async () => {
    const media = stubMedia();
    const chapters: PlaybackInfo = {
      ...playback,
      resumePositionMs: 0,
      chapters: [
        { startMs: 0, endMs: 3000, title: 'Kapitel 1: Anfang' },
        { startMs: 3000, endMs: 6500, title: 'Kapitel 2: Mitte' },
        { startMs: 6500, endMs: 10_000 },
      ],
    };
    mockApi(routes(chapters));
    const { container } = renderApp(`/play/${ids.alien}`);
    await startPlayer(container);
    fireEvent.keyDown(document.body, { key: 'c' });
    const menu = await screen.findByRole('menu', { name: 'Kapitel' });
    expect(
      within(menu).getByRole('menuitemradio', { name: 'Kapitel 1: Anfang · 0:00:00' }),
    ).toHaveAttribute('aria-checked', 'true');
    const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
    await user.click(within(menu).getByRole('menuitemradio', { name: 'Kapitel 3 · 0:00:06' }));
    expect(media.currentTime).toBeCloseTo(6.5);
  });

  it('shows "Intro überspringen" inside the intro; S jumps to its end', async () => {
    const media = stubMedia();
    const intro: PlaybackInfo = {
      ...playback,
      resumePositionMs: 0,
      segments: { intro: { startMs: 5000, endMs: 25_000, source: 'fingerprint' } },
    };
    mockApi(routes(intro));
    const { container } = renderApp(`/play/${ids.alien}`);
    const video = await startPlayer(container);
    expect(screen.queryByRole('button', { name: 'Intro überspringen' })).toBeNull();
    media.currentTime = 6;
    act(() => {
      video.dispatchEvent(new Event('timeupdate'));
    });
    expect(screen.getByRole('button', { name: 'Intro überspringen' })).toBeVisible();
    fireEvent.keyDown(document.body, { key: 's' });
    expect(media.currentTime).toBeCloseTo(25);
    act(() => {
      video.dispatchEvent(new Event('timeupdate'));
    });
    expect(screen.queryByRole('button', { name: 'Intro überspringen' })).toBeNull();
  });

  it('counts down from the outro and plays the next episode', async () => {
    const media = stubMedia();
    const withNext: PlaybackInfo = {
      ...playback,
      resumePositionMs: 0,
      segments: { outro: { startMs: 6_900_000, endMs: 7_020_000, source: 'fingerprint' } },
      nextEpisode: episodes.e2,
    };
    const { router } = (() => {
      mockApi({
        ...routes(withNext),
        [`POST /items/${episodes.e2.id}/play`]: () =>
          json({ ...playback, item: episodes.e2, playbackSessionId: 'ps-next' }),
      });
      return renderApp(`/play/${ids.alien}`);
    })();
    const video = await startPlayer(document.body);
    media.currentTime = 6901;
    act(() => {
      video.dispatchEvent(new Event('timeupdate'));
    });
    const panel = screen.getByRole('region', { name: 'Nächste Folge' });
    expect(within(panel).getByText('S01E02 · Die lange Nacht')).toBeVisible();
    expect(within(panel).getByText('Startet in 10 Sekunden')).toBeVisible();
    expect(region()).toHaveAttribute('data-autonext', 'countdown');
    expect(region()).toHaveAttribute('data-controls', 'visible');

    await act(() => vi.advanceTimersByTimeAsync(10_500));
    await waitFor(() => expect(router.state.location.pathname).toBe(`/play/${episodes.e2.id}`));
    await waitFor(() => expect(region()).toHaveAttribute('data-autonext', 'navigated'));
    expect(region()).toHaveAttribute('data-playback-session-id', 'ps-next');
  });

  it('cancels the countdown with Escape (or any key) without leaving the player', async () => {
    const media = stubMedia();
    const withNext: PlaybackInfo = { ...playback, resumePositionMs: 0, nextEpisode: episodes.e2 };
    mockApi(routes(withNext));
    const { router, container } = renderApp(`/play/${ids.alien}`);
    const video = await startPlayer(container);
    media.currentTime = 7020;
    media.ended = true;
    act(() => {
      video.dispatchEvent(new Event('ended'));
    });
    expect(screen.getByRole('region', { name: 'Nächste Folge' })).toBeVisible();
    fireEvent.keyDown(document.body, { key: 'Escape' });
    expect(screen.queryByRole('region', { name: 'Nächste Folge' })).toBeNull();
    expect(region()).not.toHaveAttribute('data-autonext');
    await act(() => vi.advanceTimersByTimeAsync(11_000));
    expect(router.state.location.pathname).toBe(`/play/${ids.alien}`);
  });

  it('N plays the next episode at once', async () => {
    stubMedia();
    mockApi(routes({ ...playback, nextEpisode: episodes.e2 }));
    const { router, container } = renderApp(`/play/${ids.alien}`);
    await startPlayer(container);
    fireEvent.keyDown(document.body, { key: 'n' });
    await waitFor(() => expect(router.state.location.pathname).toBe(`/play/${episodes.e2.id}`));
  });

  it('stops with the watch-time notice when the server reports no minutes left (K-5)', async () => {
    const media = stubMedia();
    mockApi(
      routes(playback, undefined, {
        [`POST /items/${ids.alien}/progress`]: () =>
          json(
            progressAnswer({
              watchTime: { limitMinutes: 30, usedMinutes: 30, remainingMinutes: 0 },
            }),
          ),
      }),
    );
    const { container } = renderApp(`/play/${ids.alien}`);
    await startPlayer(container);
    expect(media.paused).toBe(false);
    await act(() => vi.advanceTimersByTimeAsync(10_000));
    await waitFor(() => expect(region()).toHaveAttribute('data-ended-reason', 'limit'));
    expect(media.paused).toBe(true);
    expect(screen.getByRole('alert')).toHaveTextContent('Schauzeit für heute aufgebraucht');
    expect(screen.getByRole('alert')).toHaveTextContent('30 Minuten pro Tag');
    fireEvent.keyDown(document.body, { key: ' ' });
    expect(media.paused).toBe(true);
  });

  it('shows that the admin ended the stream when hls.js gets a 401 (W-6)', async () => {
    stubMedia();
    vi.spyOn(HTMLMediaElement.prototype, 'canPlayType').mockImplementation((type) =>
      type === 'application/vnd.apple.mpegurl' ? '' : 'probably',
    );
    vi.stubGlobal('MediaSource', { isTypeSupported: () => true });
    mockApi(routes(hlsPlayback));
    renderApp(`/play/${ids.alien}`);
    await waitFor(() => expect(hlsMock.instances).toHaveLength(1));
    // A non-fatal fragment error is enough: a 401 never heals.
    act(() =>
      hlsMock.instances[0]!.emitError({
        type: 'networkError',
        details: 'fragLoadError',
        fatal: false,
        response: { code: 401 },
      }),
    );
    expect(region()).toHaveAttribute('data-ended-reason', 'admin');
    expect(screen.getByRole('alert')).toHaveTextContent(
      'Die Verwaltung hat diesen Stream beendet.',
    );
    expect(hlsMock.instances[0]!.stopped).toBe(1);
  });

  it('asks the stream after a direct-play error: 401 means ended, anything else a failure', async () => {
    stubMedia();
    const { calls } = mockApi(
      routes(playback, undefined, {
        [`GET /items/${ids.alien}/stream`]: () => problem(401, 'unauthenticated'),
      }),
    );
    const { container } = renderApp(`/play/${ids.alien}`);
    const video = await startPlayer(container);
    act(() => {
      video.dispatchEvent(new Event('error'));
    });
    await waitFor(() => expect(region()).toHaveAttribute('data-ended-reason', 'admin'));
    expect(calls.find((call) => call.path === `/items/${ids.alien}/stream`)?.method).toBe('GET');
  });

  it('ignores the old session after a switch: its 401 is no stop by the server (W-1)', async () => {
    stubMedia();
    vi.spyOn(HTMLMediaElement.prototype, 'canPlayType').mockImplementation((type) =>
      type === 'application/vnd.apple.mpegurl' ? '' : 'probably',
    );
    vi.stubGlobal('MediaSource', { isTypeSupported: () => true });
    let release = () => {};
    const held = new Promise<void>((resolve) => {
      release = resolve;
    });
    let count = 0;
    const { calls } = mockApi({
      ...routes(hlsPlayback),
      [`POST ${playPath}`]: async () => {
        count += 1;
        if (count === 1) return json(hlsPlayback);
        await held;
        return json({ ...hlsPlayback, playbackSessionId: 'ps-480' });
      },
    });
    renderApp(`/play/${ids.alien}`);
    await waitFor(() => expect(hlsMock.instances).toHaveLength(1));
    fireEvent.keyDown(document.body, { key: 'q' });
    const menu = await screen.findByRole('menu', { name: /^Qualität/ });
    const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
    await user.click(within(menu).getByRole('menuitemradio', { name: '480p' }));
    await waitFor(() => expect(starts(calls)).toHaveLength(2));
    // The old player stopped loading; the server's 401 for the ended session changes nothing.
    expect(hlsMock.instances[0]!.stopped).toBeGreaterThanOrEqual(1);
    act(() => {
      hlsMock.instances[0]!.emitError({
        type: 'networkError',
        details: 'fragLoadError',
        fatal: true,
        response: { code: 401 },
      });
    });
    expect(region()).not.toHaveAttribute('data-ended-reason');
    expect(screen.queryByRole('alert')).toBeNull();

    release();
    await waitFor(() => expect(region()).toHaveAttribute('data-playback-session-id', 'ps-480'));
    expect(region()).not.toHaveAttribute('data-ended-reason');
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('ignores a direct-play error of the old session during a switch (W-1)', async () => {
    stubMedia();
    let release = () => {};
    const held = new Promise<void>((resolve) => {
      release = resolve;
    });
    let count = 0;
    const { calls } = mockApi({
      ...routes(playback),
      [`POST ${playPath}`]: async () => {
        count += 1;
        if (count === 1) return json(playback);
        await held;
        return json({ ...playback, playbackSessionId: 'ps-en', selection: { audioTrack: 2 } });
      },
      [`GET /items/${ids.alien}/stream`]: () => problem(401, 'unauthenticated'),
    });
    const { container } = renderApp(`/play/${ids.alien}`);
    const video = await startPlayer(container);
    const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
    await user.click(screen.getByRole('button', { name: 'Spuren' }));
    await user.click(await screen.findByRole('menuitemradio', { name: 'Englisch · AC3 · 5.1' }));
    await waitFor(() => expect(starts(calls)).toHaveLength(2));
    act(() => {
      video.dispatchEvent(new Event('error'));
    });
    await act(() => vi.advanceTimersByTimeAsync(100));
    expect(calls.some((call) => call.path === `/items/${ids.alien}/stream`)).toBe(false);
    expect(region()).not.toHaveAttribute('data-ended-reason');
    expect(screen.queryByRole('alert')).toBeNull();
    release();
    await waitFor(() => expect(region()).toHaveAttribute('data-playback-session-id', 'ps-en'));
  });

  it('keeps full screen across a switch: the document is in full screen, not the player (W-2)', async () => {
    stubMedia();
    let fullscreenElement: Element | null = null;
    const changed = () => document.dispatchEvent(new Event('fullscreenchange'));
    Object.defineProperty(document, 'fullscreenElement', {
      configurable: true,
      get: () => fullscreenElement,
    });
    const request = vi.fn(() => {
      fullscreenElement = document.documentElement;
      changed();
      return Promise.resolve();
    });
    const exit = vi.fn(() => {
      fullscreenElement = null;
      changed();
      return Promise.resolve();
    });
    document.documentElement.requestFullscreen = request;
    document.exitFullscreen = exit;
    try {
      const { calls } = mockApi(
        routes(playback, () => ({ ...playback, playbackSessionId: 'ps-2' })),
      );
      const { container, unmount } = renderApp(`/play/${ids.alien}`);
      await startPlayer(container);
      fireEvent.keyDown(document.body, { key: 'f' });
      expect(request).toHaveBeenCalledTimes(1);
      // Called on the document element, which no switch replaces.
      expect(request.mock.contexts[0]).toBe(document.documentElement);
      expect(screen.getByRole('button', { name: 'Vollbild beenden' })).toBeVisible();

      const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
      fireEvent.keyDown(document.body, { key: 'q' });
      const menu = await screen.findByRole('menu', { name: /^Qualität/ });
      await user.click(within(menu).getByRole('menuitemradio', { name: '480p' }));
      await waitFor(() => expect(starts(calls)).toHaveLength(2));
      await waitFor(() => expect(region()).toHaveAttribute('data-playback-session-id', 'ps-2'));
      // A new player, still in full screen, and it knows.
      expect(fullscreenElement).toBe(document.documentElement);
      expect(exit).not.toHaveBeenCalled();
      expect(screen.getByRole('button', { name: 'Vollbild beenden' })).toBeVisible();

      // Leaving the player ends full screen.
      unmount();
      expect(exit).toHaveBeenCalledTimes(1);
    } finally {
      Reflect.deleteProperty(document, 'fullscreenElement');
      Reflect.deleteProperty(document.documentElement, 'requestFullscreen');
      Reflect.deleteProperty(document, 'exitFullscreen');
    }
  });

  it('shows the watch-time limit when the start answers 403 watch_limit_reached', async () => {
    stubMedia();
    const { container } = (() => {
      mockApi({
        ...routes(playback),
        [`POST ${playPath}`]: () =>
          problem(403, 'watch_limit_reached', {
            extensions: { limitMinutes: 1, usedMinutes: 2 },
          }),
      });
      return renderApp(`/play/${ids.alien}`);
    })();
    expect(await screen.findByText('Schauzeit für heute aufgebraucht')).toBeVisible();
    expect(
      screen.getByText('Dieses Profil darf 1 Minute pro Tag schauen. Morgen geht es weiter.'),
    ).toBeVisible();
    expect(screen.getByText('Heute geschaut: 2 Minuten.')).toBeVisible();
    expect(container.querySelector('main')).toHaveAttribute(
      'data-error-code',
      'watch_limit_reached',
    );
  });

  it('shows the stream limit when the start answers 409 stream_limit_reached', async () => {
    stubMedia();
    mockApi({
      ...routes(playback),
      [`POST ${playPath}`]: () =>
        problem(409, 'stream_limit_reached', { extensions: { maxStreams: 1, activeStreams: 1 } }),
    });
    renderApp(`/play/${ids.alien}`);
    expect(await screen.findByText('Zu viele gleichzeitige Streams')).toBeVisible();
    expect(screen.getByText(/darf 1 Stream gleichzeitig abspielen/)).toBeVisible();
    expect(screen.getByText('Gerade läuft 1 Stream.')).toBeVisible();
  });
});
