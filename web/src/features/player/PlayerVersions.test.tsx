import type { PlaybackInfo } from '@slouchcat/api-client';
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { alienVersions, hlsPlayback, ids, playback } from '@/test/fixtures';
import { json, mockApi, renderApp, signedIn, type Call } from '@/test/api';

/** jsdom has no media pipeline: the element state the player reads (see PlayerPage.test.tsx). */
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

const sd: PlaybackInfo = {
  ...playback,
  mediaFile: alienVersions.mediaFiles[1]!,
  streamUrl: `/items/${ids.alien}/stream?st=ps-sd.token`,
  playbackSessionId: 'ps-sd',
};

function routes(info: PlaybackInfo = playback) {
  return {
    ...signedIn,
    [`POST /items/${ids.alien}/play`]: (call: Call) =>
      json((call.body as { mediaFileId?: string }).mediaFileId === 'mf-2' ? sd : info),
    'DELETE /playback/sessions/*': () => new Response(null, { status: 204 }),
    [`POST /items/${ids.alien}/progress`]: () =>
      json({ positionMs: 0, durationMs: 0, percent: 0, completed: false, updatedAt: '' }),
    [`GET /items/${ids.alien}`]: () => json(alienVersions),
  };
}

const starts = (calls: Call[]) =>
  calls.filter((call) => call.method === 'POST' && call.path.endsWith('/play'));

async function videoOf(container: HTMLElement) {
  return waitFor(() => {
    const element = container.querySelector('video');
    if (!element) throw new Error('no video yet');
    return element;
  });
}

describe('player: versions and stream info (B-5)', () => {
  beforeEach(() => vi.useFakeTimers({ shouldAdvanceTime: true }));
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('plays the version named by ?file and marks the player with its id', async () => {
    stubMedia();
    const { calls } = mockApi(routes());
    const { container } = renderApp(`/play/${ids.alien}?file=mf-2`);
    await videoOf(container);
    expect(starts(calls)[0]?.body).toMatchObject({ mediaFileId: 'mf-2' });
    expect(screen.getByRole('main', { name: 'Videoplayer' })).toHaveAttribute(
      'data-media-file-id',
      'mf-2',
    );
  });

  it('toggles the stream info with "i" and the Info button', async () => {
    stubMedia();
    mockApi(routes(hlsPlayback));
    const { container } = renderApp(`/play/${ids.alien}`);
    await videoOf(container);
    const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });

    fireEvent.keyDown(document.body, { key: 'i' });
    const info = screen.getByRole('region', { name: 'Stream-Info' });
    expect(within(info).getByText('HLS')).toBeVisible();
    expect(within(info).getByText('Remux')).toBeVisible();
    expect(within(info).getByText('container')).toBeVisible();
    expect(within(info).getByText('mf-1')).toBeVisible();
    expect(within(info).getByText('1080p')).toBeVisible();
    expect(within(info).getByText('1920 × 1080 · 1080p')).toBeVisible();

    // Escape closes the overlay first, it does not leave the player.
    fireEvent.keyDown(document.body, { key: 'Escape' });
    expect(screen.queryByRole('region', { name: 'Stream-Info' })).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Stream-Info' }));
    expect(screen.getByRole('region', { name: 'Stream-Info' })).toBeVisible();
  });

  it('switches the version: ends the session, starts the other file at the same position', async () => {
    const media = stubMedia();
    const { calls } = mockApi(routes());
    const { container } = renderApp(`/play/${ids.alien}`);
    const first = await videoOf(container);
    act(() => {
      first.dispatchEvent(new Event('loadedmetadata'));
    });
    expect(media.currentTime).toBeCloseTo(2533);
    media.currentTime = 3100;
    const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });

    await user.click(await screen.findByRole('button', { name: 'Version: 1080p' }));
    await user.click(await screen.findByRole('menuitemradio', { name: '720p' }));

    await waitFor(() => expect(starts(calls)).toHaveLength(2));
    expect(starts(calls)[1]?.body).toMatchObject({ mediaFileId: 'mf-2' });
    expect(
      calls.some((call) => call.method === 'DELETE' && call.path === '/playback/sessions/ps-1'),
    ).toBe(true);

    const region = screen.getByRole('main', { name: 'Videoplayer' });
    await waitFor(() => expect(region).toHaveAttribute('data-media-file-id', 'mf-2'));
    const second = await videoOf(container);
    expect(second).not.toBe(first);
    expect(second).toHaveAttribute('src', `/api/v1/items/${ids.alien}/stream?st=ps-sd.token`);
    act(() => {
      second.dispatchEvent(new Event('loadedmetadata'));
    });
    // The position the viewer was at, not the stored resume position.
    expect(media.currentTime).toBeCloseTo(3100);
  });

  it('has no version menu for a title with one file', async () => {
    stubMedia();
    mockApi({
      ...routes(),
      [`GET /items/${ids.alien}`]: () =>
        json({ ...alienVersions, mediaFiles: alienVersions.mediaFiles.slice(0, 1) }),
    });
    const { container } = renderApp(`/play/${ids.alien}`);
    await videoOf(container);
    await act(() => vi.advanceTimersByTimeAsync(50));
    expect(screen.queryByRole('button', { name: /^Version/ })).not.toBeInTheDocument();
  });
});
