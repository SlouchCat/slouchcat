import type { ProgressEvent } from '@slouchcat/api-client';
import { act, fireEvent, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { alienDetail, ids, playback } from '@/test/fixtures';
import { json, mockApi, renderApp, signedIn, type Call } from '@/test/api';

const progressRoute = `POST /items/${ids.alien}/progress`;

function routes() {
  return {
    ...signedIn,
    [`POST /items/${ids.alien}/play`]: () => json(playback),
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
    expect(video).toHaveAttribute('src', `/api/v1/items/${ids.alien}/stream?file=mf-1`);
    expect(video).toHaveAttribute('preload', 'auto');

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

  it('shows the M1b notice instead of a broken player when the browser cannot play the file', async () => {
    const { canPlayType } = stubMedia('');
    mockApi(routes());
    const { container } = renderApp(`/play/${ids.alien}`);
    expect(await screen.findByText('Format nicht direkt abspielbar')).toBeVisible();
    expect(
      screen.getByText(
        'Dieses Format kann der Browser nicht direkt abspielen; Umwandlung folgt in M1b.',
      ),
    ).toBeVisible();
    expect(container.querySelector('video')).toBeNull();
    expect(canPlayType).toHaveBeenCalledWith('video/mp4; codecs="avc1.640028, mp4a.40.2"');
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
