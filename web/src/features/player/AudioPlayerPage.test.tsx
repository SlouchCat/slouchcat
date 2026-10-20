import type { ProgressEvent } from '@slouchcat/api-client';
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { audioIds, audioPlayback, audiobookDetail, podcastEpisodeDetail } from '@/test/fixtures';
import { json, mockApi, renderApp, signedIn, type Call } from '@/test/api';

// hls.js is not needed: the M4B book plays directly. jsdom has no media pipeline, so the element
// state the player reads is modelled here.
function stubMedia() {
  const state = { paused: true, ended: false, currentTime: 0, duration: 60 };
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

const progressPath = `/items/${audioIds.book}/progress`;

function routes(info = audioPlayback) {
  return {
    ...signedIn,
    [`POST /items/${audioIds.book}/play`]: () => json(info),
    [`POST /items/${audioIds.ep2}/play`]: () =>
      json({
        ...info,
        item: podcastEpisodeDetail,
        chapters: [],
        streamUrl: `/items/${audioIds.ep2}/stream?st=ps-e.token`,
        playbackSessionId: 'ps-e',
        resumePositionMs: 0,
      }),
    'DELETE /playback/sessions/*': () => new Response(null, { status: 204 }),
    'POST /items/*/progress': () =>
      json({ positionMs: 0, durationMs: 0, percent: 0, completed: false, updatedAt: '' }),
    [`GET /items/${audioIds.book}`]: () => json(audiobookDetail),
    [`GET /items/${audioIds.ep2}`]: () => json(podcastEpisodeDetail),
  };
}

const progressEvents = (calls: Call[]) =>
  calls.filter((call) => call.path === progressPath).map((call) => call.body as ProgressEvent);

/** Mounts the player and plays from the resume position (loadedmetadata → play). */
async function start(path = `/play/${audioIds.book}`) {
  const view = renderApp(path);
  const region = await screen.findByRole('main', { name: 'Audioplayer' });
  const audio = await waitFor(() => {
    const element = view.container.querySelector('audio');
    if (!element) throw new Error('no audio yet');
    return element;
  });
  act(() => {
    audio.dispatchEvent(new Event('loadedmetadata'));
  });
  await waitFor(() => expect(region).toHaveAttribute('data-start-ms'));
  return { ...view, region, audio };
}

describe('audio player (M6)', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('plays an audiobook in an <audio> element from the resume position at the stored speed', async () => {
    const media = stubMedia();
    mockApi(routes());
    const { region, audio } = await start();

    expect(audio).toHaveAttribute('src', `/api/v1/items/${audioIds.book}/stream?st=ps-a.token`);
    expect(screen.queryByRole('main', { name: 'Videoplayer' })).not.toBeInTheDocument();
    expect(within(region).getByRole('heading', { name: 'Slouchcat Test Audiobook M4B' }));
    expect(within(region).getByText('Slouchcat Test Author')).toBeVisible();
    // Resume at 12.34 s: chapter 3 (10 … 15 s), 2 s into it.
    expect(media.currentTime).toBeCloseTo(12.34);
    act(() => {
      audio.dispatchEvent(new Event('timeupdate'));
    });
    expect(within(region).getByText('Kapitel 3 · 0:02 / 0:05')).toBeVisible();
    // H-3: the profile's last speed for this title.
    expect(audio.playbackRate).toBe(1.5);
    expect(region).toHaveAttribute('data-rate', '1.5');
    expect(within(region).getByRole('button', { name: 'Pause' })).toHaveFocus();
  });

  it('keys: speed by [ and ], chapters by Shift+arrows, ±15 s; the speed goes up with progress', async () => {
    const media = stubMedia();
    const { calls } = mockApi(routes());
    const { region, audio } = await start();

    fireEvent.keyDown(window, { key: ']' });
    fireEvent.keyDown(window, { key: ']' });
    expect(audio.playbackRate).toBe(1.7);
    expect(await within(region).findByRole('status')).toHaveTextContent('Geschwindigkeit 1,7×');
    fireEvent.keyDown(window, { key: '[' });
    expect(audio.playbackRate).toBe(1.6);

    // Shift+→: the next chapter (15 s); →: 15 s further; Shift+←: back to that chapter's start.
    fireEvent.keyDown(window, { key: 'ArrowRight', shiftKey: true });
    expect(media.currentTime).toBe(15);
    fireEvent.keyDown(window, { key: 'ArrowRight' });
    expect(media.currentTime).toBe(30);
    fireEvent.keyDown(window, { key: 'ArrowLeft', shiftKey: true });
    expect(media.currentTime).toBe(25);
    fireEvent.keyDown(window, { key: 'ArrowLeft' });
    expect(media.currentTime).toBe(10);

    // The next progress event carries the speed (H-3).
    act(() => audio.pause());
    await waitFor(() => expect(progressEvents(calls).at(-1)?.state).toBe('paused'));
    expect(progressEvents(calls).at(-1)).toMatchObject({ positionMs: 10_000, playbackRate: 1.6 });
  });

  it('C opens the chapter list: a chapter jumps to its start', async () => {
    const media = stubMedia();
    mockApi(routes());
    await start();
    const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });

    await user.keyboard('c');
    const menu = await screen.findByRole('menu', { name: 'Kapitel' });
    expect(within(menu).getAllByRole('menuitemradio')).toHaveLength(12);
    expect(within(menu).getByRole('menuitemradio', { name: 'Kapitel 3 · 0:10' })).toHaveAttribute(
      'aria-checked',
      'true',
    );
    await user.click(within(menu).getByRole('menuitemradio', { name: 'Kapitel 7 · 0:30' }));
    expect(media.currentTime).toBe(30);
    await waitFor(() => expect(screen.queryByRole('menu')).not.toBeInTheDocument());
  });

  it('T sets the sleep timer: at the end of the chapter the player pauses', async () => {
    const media = stubMedia();
    mockApi(routes());
    const { region, audio } = await start();
    const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });

    await user.keyboard('t');
    const menu = await screen.findByRole('menu', { name: 'Sleep-Timer' });
    await user.click(within(menu).getByRole('menuitemradio', { name: 'Ende des Kapitels' }));
    expect(await within(region).findByRole('status')).toHaveTextContent(
      'Sleep-Timer: Ende des Kapitels',
    );
    // 2.66 s of chapter 3 left at 1.5×: the button counts down in wall time.
    expect(
      within(region).getByRole('button', { name: /^Sleep-Timer: noch 0:0[12]$/ }),
    ).toBeVisible();

    media.currentTime = 15.01;
    act(() => {
      audio.dispatchEvent(new Event('timeupdate'));
    });
    expect(media.paused).toBe(true);
    expect(await within(region).findByRole('status')).toHaveTextContent('Sleep-Timer abgelaufen');
    expect(within(region).getByRole('button', { name: 'Sleep-Timer' })).toBeVisible();
  });

  it('an episode: the podcast as byline, no chapter controls; Escape goes back', async () => {
    stubMedia();
    mockApi(routes());
    const { region, router } = await start(`/play/${audioIds.ep2}`);
    expect(within(region).getByRole('heading', { name: 'Slouchcat Test Episode 2' }));
    await waitFor(() => expect(within(region).getByText('Slouchcat Test Podcast')).toBeVisible());
    expect(
      within(region).queryByRole('button', { name: 'Nächstes Kapitel' }),
    ).not.toBeInTheDocument();
    expect(within(region).queryByRole('button', { name: 'Kapitel' })).not.toBeInTheDocument();
    fireEvent.keyDown(window, { key: 'Escape' });
    await waitFor(() => expect(router.state.location.pathname).toBe(`/items/${audioIds.ep2}`));
  });
});
