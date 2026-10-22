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

  it('[ and ] typed with AltGr (Windows, Linux) or Option (macOS) change the speed; Ctrl and Meta stay free', async () => {
    stubMedia();
    mockApi(routes());
    const { audio } = await start();

    // Swiss layout on Windows: AltGr reports Ctrl+Alt and the AltGraph modifier.
    fireEvent.keyDown(window, { key: ']', ctrlKey: true, altKey: true, modifierAltGraph: true });
    expect(audio.playbackRate).toBe(1.6);
    // Alt without AltGraph outside macOS is a shortcut.
    fireEvent.keyDown(window, { key: ']', altKey: true });
    expect(audio.playbackRate).toBe(1.6);
    // macOS: Option+5 and Option+6 type [ and ].
    vi.spyOn(navigator, 'userAgent', 'get').mockReturnValue(
      'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15',
    );
    fireEvent.keyDown(window, { key: '[', altKey: true });
    expect(audio.playbackRate).toBe(1.5);
    // Ctrl and Meta shortcuts stay with the browser.
    fireEvent.keyDown(window, { key: ']', ctrlKey: true });
    fireEvent.keyDown(window, { key: ']', metaKey: true });
    fireEvent.keyDown(window, { key: ']', metaKey: true, modifierAltGraph: true });
    expect(audio.playbackRate).toBe(1.5);
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

  // hls.js re-attaches the element after a media error (OnMediaSinkAudioError on a machine without
  // an audio output, for example a CI runner). Attaching runs the HTML load algorithm, which sets
  // `paused` without a `pause` event: the element reads as paused while the player still shows
  // "Pause", and the reloaded source reports its metadata again.
  it('a source that loads again keeps the pause, and Pause stops a silently paused element', async () => {
    const media = stubMedia();
    mockApi(routes());
    const { region, audio } = await start();
    const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
    expect(media.paused).toBe(false);

    await user.click(within(region).getByRole('button', { name: 'Pause' }));
    expect(media.paused).toBe(true);
    // The re-attach of the paused player must not play on.
    act(() => {
      audio.dispatchEvent(new Event('loadedmetadata'));
    });
    expect(media.paused).toBe(true);

    // Playing again, then the silent pause of the load algorithm: the button still says "Pause"
    // and has to stop the element for good, not start it.
    await user.click(within(region).getByRole('button', { name: 'Abspielen' }));
    expect(media.paused).toBe(false);
    media.paused = true;
    await user.click(within(region).getByRole('button', { name: 'Pause' }));
    act(() => {
      audio.dispatchEvent(new Event('loadedmetadata'));
    });
    expect(media.paused).toBe(true);
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

  it('the sleep timer ends the chapter on time, follows a jump and waits for a known length', async () => {
    const media = stubMedia();
    mockApi(routes());
    const { region, audio } = await start();
    const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });

    await user.keyboard('t');
    await user.click(
      within(await screen.findByRole('menu', { name: 'Sleep-Timer' })).getByRole('menuitemradio', {
        name: 'Ende des Kapitels',
      }),
    );
    // Shift+→ jumps to chapter 4 (15 … 20 s): the timer follows to its end.
    fireEvent.keyDown(window, { key: 'ArrowRight', shiftKey: true });
    act(() => {
      audio.dispatchEvent(new Event('seeking'));
    });
    media.currentTime = 15.01;
    act(() => {
      audio.dispatchEvent(new Event('timeupdate'));
    });
    expect(media.paused).toBe(false);

    // 0.45 s of the chapter left at 1.5×: a timer pauses after 0.3 s, not at the next update.
    media.currentTime = 19.55;
    act(() => {
      audio.dispatchEvent(new Event('timeupdate'));
    });
    expect(media.paused).toBe(false);
    act(() => {
      vi.advanceTimersByTime(310);
    });
    expect(media.paused).toBe(true);
    expect(await within(region).findByRole('status')).toHaveTextContent('Sleep-Timer abgelaufen');
  });

  it('an episode without a known length offers the end of the title only once it is known', async () => {
    const media = stubMedia();
    media.duration = NaN;
    mockApi({
      ...routes(),
      [`POST /items/${audioIds.ep2}/play`]: () =>
        json({
          ...audioPlayback,
          item: { ...podcastEpisodeDetail, durationMs: undefined },
          mediaFile: { ...audioPlayback.mediaFile, durationMs: undefined },
          chapters: [],
          streamUrl: `/items/${audioIds.ep2}/stream?st=ps-e.token`,
          playbackSessionId: 'ps-e',
          resumePositionMs: 0,
        }),
    });
    const { audio } = await start(`/play/${audioIds.ep2}`);
    const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });

    await user.keyboard('t');
    let menu = await screen.findByRole('menu', { name: 'Sleep-Timer' });
    expect(within(menu).getAllByRole('menuitemradio')).toHaveLength(5);
    await user.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('menu')).not.toBeInTheDocument());

    media.duration = 10;
    act(() => {
      audio.dispatchEvent(new Event('durationchange'));
    });
    await user.keyboard('t');
    menu = await screen.findByRole('menu', { name: 'Sleep-Timer' });
    expect(within(menu).getByRole('menuitemradio', { name: 'Ende des Titels' })).toBeVisible();
  });

  it('a speed that arrives with the detail after the start applies quietly, unless one was picked', async () => {
    stubMedia();
    let release: () => void = () => undefined;
    const detailReady = new Promise<void>((resolve) => {
      release = resolve;
    });
    const withoutRate = {
      ...audioPlayback,
      item: { ...audioPlayback.item, progress: undefined },
    };
    mockApi({
      ...routes(withoutRate),
      [`GET /items/${audioIds.book}`]: async () => {
        await detailReady;
        return json(audiobookDetail);
      },
    });
    const { region, audio } = await start();
    expect(audio.playbackRate).toBe(1);
    act(() => release());
    await waitFor(() => expect(audio.playbackRate).toBe(1.5));
    // Quietly: no speed confirmation for a speed the listener did not pick.
    expect(within(region).queryByRole('status')).not.toBeInTheDocument();
  });

  it('a speed picked before the detail arrives stays', async () => {
    stubMedia();
    let release: () => void = () => undefined;
    const detailReady = new Promise<void>((resolve) => {
      release = resolve;
    });
    const { calls } = mockApi({
      ...routes({ ...audioPlayback, item: { ...audioPlayback.item, progress: undefined } }),
      [`GET /items/${audioIds.book}`]: async () => {
        await detailReady;
        return json(audiobookDetail);
      },
    });
    const { audio } = await start();
    fireEvent.keyDown(window, { key: ']' });
    expect(audio.playbackRate).toBe(1.1);
    act(() => release());
    await waitFor(() =>
      expect(calls.some((call) => call.path === `/items/${audioIds.book}`)).toBe(true),
    );
    await act(() => vi.advanceTimersByTimeAsync(50));
    expect(audio.playbackRate).toBe(1.1);
  });

  it('a chapter link starts at the chapter, also in the last seconds, and leaves the URL', async () => {
    const media = stubMedia();
    mockApi(routes());
    const { router } = await start(`/play/${audioIds.book}?at=55000`);
    // Chapter 12 starts 5 s before the end: no resume rule for an explicit start.
    expect(media.currentTime).toBe(55);
    // A reload resumes from the saved position, not from the chapter.
    await waitFor(() => expect(router.state.location.search).not.toHaveProperty('at'));
    expect(router.state.location.pathname).toBe(`/play/${audioIds.book}`);
  });

  it('the focused seek bar moves like a slider; a drag seeks once, when let go', async () => {
    const media = stubMedia();
    mockApi(routes());
    const { region, audio } = await start();
    const slider = within(region).getByRole('slider', { name: 'Wiedergabeposition' });
    act(() => slider.focus());

    fireEvent.keyDown(slider, { key: 'ArrowUp' });
    expect(media.currentTime).toBeCloseTo(27.34);
    expect(audio.volume).toBe(1);
    fireEvent.keyDown(slider, { key: 'PageUp' });
    expect(media.currentTime).toBe(30);
    fireEvent.keyDown(slider, { key: 'PageDown' });
    expect(media.currentTime).toBe(25);
    fireEvent.keyDown(slider, { key: 'Home' });
    expect(media.currentTime).toBe(0);
    fireEvent.keyDown(slider, { key: 'ArrowDown' });
    expect(media.currentTime).toBe(0);

    vi.spyOn(slider, 'getBoundingClientRect').mockReturnValue(new DOMRect(0, 0, 100, 10));
    slider.setPointerCapture = vi.fn();
    slider.releasePointerCapture = vi.fn();
    fireEvent.pointerDown(slider, { pointerId: 1, clientX: 25 });
    fireEvent.pointerMove(slider, { pointerId: 1, clientX: 50 });
    expect(media.currentTime).toBe(0);
    expect(slider).toHaveAttribute('aria-valuenow', '30');
    fireEvent.pointerUp(slider, { pointerId: 1, clientX: 75 });
    expect(media.currentTime).toBe(45);
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
