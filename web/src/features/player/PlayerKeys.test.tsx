import type { PlaybackInfo, Progress } from '@slouchcat/api-client';
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  alienDetail,
  episode2Detail,
  episodes,
  ids,
  playback,
  seriesIds,
  showDetail,
} from '@/test/fixtures';
import { json, mockApi, renderApp, signedIn, type Call } from '@/test/api';
import { formatBitrate } from '@/lib/format';
import { playerDriver } from '@/test/player';
import { CONTROLS_HIDE_MS } from './VideoPlayer';
import { PLAYER_PREFS_KEY } from './usePlayerPrefs';

// Concept 13, step 3: the controls of PlayerChrome in the video player, its focus model (11.2)
// and the key map with M, U, [ ], 0–9 and the time display.

const progress: Progress = {
  positionMs: 0,
  durationMs: 0,
  percent: 0,
  completed: false,
  updatedAt: '',
};

function stubMedia() {
  const state = { paused: true, currentTime: 0, duration: 600 };
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
  vi.spyOn(HTMLMediaElement.prototype, 'duration', 'get').mockImplementation(() => state.duration);
  vi.spyOn(HTMLMediaElement.prototype, 'currentTime', 'get').mockImplementation(
    () => state.currentTime,
  );
  vi.spyOn(HTMLMediaElement.prototype, 'currentTime', 'set').mockImplementation((value: number) => {
    state.currentTime = value;
  });
  return state;
}

// Ten minutes from 1:40 on; chapters at 0, 2:00 and 5:00.
const film: PlaybackInfo = {
  ...playback,
  resumePositionMs: 100_000,
  mediaFile: { ...playback.mediaFile, durationMs: 600_000 },
  chapters: [
    { startMs: 0, endMs: 120_000, title: 'Anfang' },
    { startMs: 120_000, endMs: 300_000, title: 'Mitte' },
    { startMs: 300_000, endMs: 600_000 },
  ],
};

function routes(info: PlaybackInfo, extra: Record<string, (call: Call) => Response> = {}) {
  return {
    ...signedIn,
    [`POST /items/${info.item.id}/play`]: () => json(info),
    'DELETE /playback/sessions/*': () => new Response(null, { status: 204 }),
    'GET /playback/sessions/*': () => json({ state: 'active' }),
    [`POST /items/${info.item.id}/progress`]: () => json(progress),
    [`GET /items/${ids.alien}`]: () => json(alienDetail),
    ...extra,
  };
}

async function start(info: PlaybackInfo = film, extra = {}) {
  const media = stubMedia();
  mockApi(routes(info, extra));
  const { container, router } = renderApp(`/play/${info.item.id}`);
  const video = await waitFor(() => {
    const element = container.querySelector('video');
    if (!element) throw new Error('no video yet');
    return element;
  });
  act(() => {
    video.dispatchEvent(new Event('loadedmetadata'));
  });
  await waitFor(() => expect(screen.getByRole('button', { name: 'Pause' })).toHaveFocus());
  return { media, video, router };
}

const region = () => screen.getByRole('main', { name: 'Videoplayer' });
const button = (name: string | RegExp) => screen.getByRole('button', { name });
const seekBar = () => screen.getByRole('slider', { name: 'Wiedergabeposition' });
const key = (target: Element, name: string, extra: object = {}) =>
  fireEvent.keyDown(target, { key: name, ...extra });

describe('video player controls (concept 13, step 3)', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    localStorage.clear();
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it('gives the first focus to play/pause; ←/→ in the row move the focus and do not seek', async () => {
    const { media } = await start();
    expect(media.currentTime).toBe(100);
    key(button('Pause'), 'ArrowRight');
    expect(button('30 Sekunden vor')).toHaveFocus();
    key(button('30 Sekunden vor'), 'ArrowLeft');
    key(button('Pause'), 'ArrowLeft');
    expect(button('10 Sekunden zurück')).toHaveFocus();
    expect(media.currentTime).toBe(100);
  });

  it('↑ leads from the row to the timeline, where ←/→ seek 10 s and ↓ leads back', async () => {
    const { media } = await start();
    key(button('Pause'), 'ArrowUp');
    expect(seekBar()).toHaveFocus();
    key(seekBar(), 'ArrowRight');
    expect(media.currentTime).toBe(110);
    key(seekBar(), 'ArrowLeft');
    key(seekBar(), 'ArrowLeft');
    expect(media.currentTime).toBe(90);
    key(seekBar(), 'ArrowDown');
    expect(button('Pause')).toHaveFocus();
  });

  it('with the controls faded out the arrows seek and change the volume wherever the focus is', async () => {
    const { media, video } = await start();
    await act(() => vi.advanceTimersByTimeAsync(CONTROLS_HIDE_MS + 100));
    expect(region()).toHaveAttribute('data-controls', 'hidden');
    expect(button('Pause')).toHaveFocus();
    key(button('Pause'), 'ArrowRight');
    expect(media.currentTime).toBe(110);
    expect(button('Pause')).toHaveFocus();
    expect(region()).toHaveAttribute('data-controls', 'visible');

    await act(() => vi.advanceTimersByTimeAsync(CONTROLS_HIDE_MS + 100));
    key(button('Pause'), 'ArrowDown');
    expect(video.volume).toBe(0.95);
    expect(screen.getByRole('status')).toHaveTextContent('Lautstärke 95 %');
  });

  it('with the controls faded out Enter plays and pauses instead of pressing the focused button', async () => {
    const { media } = await start();
    const user = playerDriver().user;
    act(() => button('30 Sekunden vor').focus());
    await act(() => vi.advanceTimersByTimeAsync(CONTROLS_HIDE_MS + 100));
    expect(region()).toHaveAttribute('data-controls', 'hidden');
    const before = media.currentTime;
    await user.keyboard('{Enter}');
    expect(media.paused).toBe(true);
    expect(media.currentTime).toBe(before);
    expect(button('30 Sekunden vor')).toHaveFocus();
  });

  it('↑/↓ change the volume by 5 % without a zone; M mutes, ↑ after it counts from the old level', async () => {
    const { video } = await start();
    act(() => button('Pause').blur());
    key(document.body, 'ArrowDown');
    key(document.body, 'ArrowDown');
    expect(video.volume).toBe(0.9);
    key(document.body, 'm');
    expect(video.muted).toBe(true);
    expect(screen.getByRole('status')).toHaveTextContent('Ton aus');
    expect(button('Ton aus')).toHaveAttribute('aria-pressed', 'true');
    key(document.body, 'ArrowUp');
    expect(video.muted).toBe(false);
    expect(video.volume).toBe(0.95);
    // The level is remembered on this device (F-2).
    expect(JSON.parse(localStorage.getItem(PLAYER_PREFS_KEY) ?? '{}')).toMatchObject({
      volume: 0.95,
    });
  });

  it('the mouse wheel over the picture changes the volume a step per notch, not over the bar', async () => {
    const { video } = await start();
    fireEvent.wheel(video, { deltaY: -100 });
    expect(video.volume).toBe(1);
    fireEvent.wheel(video, { deltaY: 100 });
    fireEvent.wheel(video, { deltaY: 100 });
    expect(video.volume).toBe(0.9);
    expect(screen.getByRole('status')).toHaveTextContent('Lautstärke 90 %');
    // A trackpad's small deltas add up to a notch; Firefox counts lines.
    fireEvent.wheel(video, { deltaY: 40 });
    fireEvent.wheel(video, { deltaY: 40 });
    expect(video.volume).toBe(0.9);
    fireEvent.wheel(video, { deltaY: 40 });
    expect(video.volume).toBe(0.85);
    fireEvent.wheel(video, { deltaY: -3, deltaMode: 1 });
    expect(video.volume).toBe(0.9);
    // Over the bar the wheel is not the volume's.
    fireEvent.wheel(button('Pause'), { deltaY: 100 });
    fireEvent.wheel(seekBar(), { deltaY: 100 });
    expect(video.volume).toBe(0.9);
  });

  it('starts at the volume stored on this device', async () => {
    localStorage.setItem(PLAYER_PREFS_KEY, JSON.stringify({ volume: 0.4 }));
    const { video } = await start();
    expect(video.volume).toBe(0.4);
  });

  it('[ and ] change the speed by 0.25 within 0.5× … 2×', async () => {
    const { video } = await start();
    key(document.body, ']');
    expect(video.playbackRate).toBe(1.25);
    expect(screen.getByRole('status')).toHaveTextContent('Geschwindigkeit 1,25×');
    for (let presses = 0; presses < 6; presses += 1) key(document.body, ']');
    expect(video.playbackRate).toBe(2);
    for (let presses = 0; presses < 10; presses += 1) key(document.body, '[');
    expect(video.playbackRate).toBe(0.5);
  });

  it('[ and ] typed with AltGr (Windows reports Ctrl+Alt) or Option (macOS) change the speed', async () => {
    const { video } = await start();
    // Swiss layout on Windows: AltGr reports Ctrl+Alt and the AltGraph modifier.
    key(document.body, ']', { ctrlKey: true, altKey: true, modifierAltGraph: true });
    expect(video.playbackRate).toBe(1.25);
    // Alt without AltGraph outside macOS is a shortcut.
    key(document.body, ']', { altKey: true });
    expect(video.playbackRate).toBe(1.25);
    vi.spyOn(navigator, 'userAgent', 'get').mockReturnValue(
      'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15',
    );
    key(document.body, '[', { altKey: true });
    expect(video.playbackRate).toBe(1);
    // Ctrl and Meta shortcuts stay with the browser.
    key(document.body, ']', { ctrlKey: true });
    key(document.body, ']', { metaKey: true, modifierAltGraph: true });
    expect(video.playbackRate).toBe(1);
  });

  it('0–9 jump to 0–90 % of the title', async () => {
    const { media } = await start();
    key(document.body, '5');
    expect(media.currentTime).toBe(300);
    key(document.body, '0');
    expect(media.currentTime).toBe(0);
    key(document.body, '9');
    expect(media.currentTime).toBe(540);
  });

  it('0–9 count with the file’s length while the element reports none', async () => {
    const { media } = await start();
    media.duration = Number.POSITIVE_INFINITY;
    key(document.body, '5');
    expect(media.currentTime).toBe(300);
  });

  it('the gear: pages with back and Backspace, the speed page sets the speed, Escape closes', async () => {
    const { video } = await start();
    const user = playerDriver().user;
    await user.click(button('Einstellungen'));
    const gear = await screen.findByRole('dialog', { name: 'Einstellungen' });
    expect(button('Einstellungen')).toHaveAttribute('aria-expanded', 'true');
    const speedRow = within(gear).getByRole('option', { name: 'Geschwindigkeit' });
    expect(speedRow).toHaveAccessibleDescription('1×');
    await waitFor(() =>
      expect(within(gear).getByRole('option', { name: 'Qualität' })).toHaveFocus(),
    );
    await user.keyboard('{ArrowDown}{Enter}');
    const speed = await screen.findByRole('dialog', { name: 'Geschwindigkeit' });
    await waitFor(() => expect(within(speed).getByRole('option', { name: '1×' })).toHaveFocus());
    await user.click(within(speed).getByRole('option', { name: '1,5×' }));
    expect(video.playbackRate).toBe(1.5);
    expect(within(speed).getByRole('option', { name: '1,5×' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    // Backspace goes up a level, the back button as well; Escape closes and gives the focus back.
    await user.keyboard('{Backspace}');
    const main = await screen.findByRole('dialog', { name: 'Einstellungen' });
    expect(
      within(main).getByRole('option', { name: 'Geschwindigkeit' }),
    ).toHaveAccessibleDescription('1,5×');
    await user.click(within(main).getByRole('option', { name: 'Geschwindigkeit' }));
    await user.click(
      within(await screen.findByRole('dialog', { name: 'Geschwindigkeit' })).getByRole('button', {
        name: 'Zurück',
      }),
    );
    await screen.findByRole('dialog', { name: 'Einstellungen' });
    await waitFor(() => expect(document.activeElement?.closest('[role="dialog"]')).not.toBeNull());
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(button('Einstellungen')).toHaveFocus();
  });

  it('Q opens the gear straight on the quality: no way up, Backspace closes', async () => {
    const { router } = await start();
    key(document.body, 'q');
    const quality = await screen.findByRole('dialog', { name: 'Qualität' });
    expect(within(quality).queryByRole('button', { name: 'Zurück' })).toBeNull();
    expect(within(quality).getByRole('option', { name: '1080p' })).toHaveAccessibleDescription(
      formatBitrate(8_000_000, 'de'),
    );
    await waitFor(() =>
      expect(within(quality).getByRole('option', { selected: true })).toHaveFocus(),
    );
    key(document.activeElement!, 'Backspace');
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(router.state.location.pathname).toBe(`/play/${ids.alien}`);
  });

  it('U opens the subtitle panel', async () => {
    await start();
    key(document.body, 'u');
    expect(await playerDriver().findList('subtitle')).toBeVisible();
  });

  it('switches the time to the remaining time and the end time, counted at the speed, and keeps the choice', async () => {
    vi.setSystemTime(new Date(2026, 10, 4, 22, 0, 0));
    await start();
    const { user } = playerDriver();
    await user.click(button('1:40 / 10:00'));
    expect(button(/^−8:20 · endet um 22:08$/)).toBeVisible();
    key(document.body, ']');
    key(document.body, ']');
    // 8:20 at 1.5×: 5:33 in real time.
    expect(button(/^−5:33 · endet um 22:05$/)).toBeVisible();
    expect(JSON.parse(localStorage.getItem(PLAYER_PREFS_KEY) ?? '{}')).toMatchObject({
      timeDisplay: 'remaining',
    });
  });

  it('shows the chapter above the timeline; ⇧→ and the buttons go from chapter to chapter', async () => {
    const { media, video } = await start();
    expect(screen.getByText('Alien · Anfang')).toBeVisible();
    key(document.body, 'ArrowRight', { shiftKey: true });
    expect(media.currentTime).toBe(120);
    act(() => {
      video.dispatchEvent(new Event('timeupdate'));
    });
    expect(screen.getByText('Alien · Mitte')).toBeVisible();
    const { user } = playerDriver();
    await user.click(button('Nächstes Kapitel'));
    expect(media.currentTime).toBe(300);
    act(() => {
      video.dispatchEvent(new Event('timeupdate'));
    });
    expect(screen.getByText('Alien · Kapitel 3')).toBeVisible();
    // In the last chapter there is no forward step (a film has no next episode).
    expect(screen.queryByRole('button', { name: 'Nächstes Kapitel' })).toBeNull();
    await user.click(button('Vorheriges Kapitel'));
    expect(media.currentTime).toBe(120);
  });

  it('shows the year and genres of a film under its title', async () => {
    await start();
    expect(screen.getByRole('heading', { level: 1, name: 'Alien' })).toBeVisible();
    expect(await screen.findByText('1979 · Horror, Science-Fiction')).toBeVisible();
  });

  it('shows the series in the head with the episode under it; forward plays the next episode', async () => {
    const episode: PlaybackInfo = {
      ...playback,
      item: episodes.e2,
      resumePositionMs: 0,
      nextEpisode: episodes.e34,
    };
    const logo = { url: `/items/${seriesIds.show}/images/logo/w400?v=l`, width: 400, height: 155 };
    const { router } = await start(episode, {
      [`GET /items/${episodes.e2.id}`]: () => json(episode2Detail),
      [`GET /items/${seriesIds.show}`]: () =>
        json({ ...showDetail, images: { ...showDetail.images, logo } }),
      [`POST /items/${episodes.e34.id}/play`]: () =>
        json({ ...playback, item: episodes.e34, playbackSessionId: 'ps-34' }),
    });
    // The series' title, as text or as its logo's text.
    const heading = screen.getByRole('heading', { level: 1, name: showDetail.title });
    expect(screen.getByText('S01E02 · Die lange Nacht')).toBeVisible();
    expect(await within(heading).findByRole('img', { name: showDetail.title })).toHaveAttribute(
      'src',
      expect.stringContaining('/images/logo/w400'),
    );
    const { user } = playerDriver();
    await user.click(button('Nächste Folge'));
    await waitFor(() => expect(router.state.location.pathname).toBe(`/play/${episodes.e34.id}`));
  });

  it('? opens the shortcut help beside the video; Escape closes it and stays in the player', async () => {
    const { router } = await start();
    key(document.body, '?');
    const help = screen.getByRole('dialog', { name: 'Tastenkürzel' });
    expect(within(help).getByText('Springt auf 0 bis 90 %')).toBeVisible();
    expect(within(help).getByText('[ ]')).toBeVisible();
    // ↑ before ↓: louder comes first, like the keys.
    expect(within(help).getByText('↑ ↓').closest('div')).toHaveTextContent('Lauter oder leiser');
    await waitFor(() => expect(help.querySelector('dl')).toHaveFocus());
    expect(button('Tastenkürzel')).toHaveAttribute('aria-expanded', 'true');
    // The controls stay while the help is open.
    await act(() => vi.advanceTimersByTimeAsync(CONTROLS_HIDE_MS + 100));
    expect(region()).toHaveAttribute('data-controls', 'visible');
    // The letters still work inside it.
    key(within(help).getByRole('button', { name: 'Schliessen' }), 'm');
    expect(button('Ton aus')).toHaveAttribute('aria-pressed', 'true');
    key(within(help).getByRole('button', { name: 'Schliessen' }), 'Escape');
    expect(screen.queryByRole('dialog', { name: 'Tastenkürzel' })).toBeNull();
    expect(router.state.location.pathname).toBe(`/play/${ids.alien}`);
  });

  it('the «?» button opens the help and gets the focus back; Escape outside it closes it first', async () => {
    const { router } = await start();
    const { user } = playerDriver();
    await user.click(button('Tastenkürzel'));
    const help = screen.getByRole('dialog', { name: 'Tastenkürzel' });
    await waitFor(() => expect(help.querySelector('dl')).toHaveFocus());
    await user.click(within(help).getByRole('button', { name: 'Schliessen' }));
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(button('Tastenkürzel')).toHaveFocus();

    key(document.body, '?');
    expect(screen.getByRole('dialog', { name: 'Tastenkürzel' })).toBeVisible();
    act(() => button('Pause').focus());
    key(button('Pause'), 'Backspace');
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(router.state.location.pathname).toBe(`/play/${ids.alien}`);
  });

  it('during the next-episode countdown an arrow in the row cancels it', async () => {
    const { video } = await start({ ...film, resumePositionMs: 0, nextEpisode: episodes.e2 });
    act(() => {
      video.pause();
      video.dispatchEvent(new Event('ended'));
    });
    expect(screen.getByRole('region', { name: 'Nächste Folge' })).toBeVisible();
    key(button('Abspielen'), 'ArrowRight');
    expect(screen.queryByRole('region', { name: 'Nächste Folge' })).toBeNull();
    // The key did nothing else: the focus stayed.
    expect(button('Abspielen')).toHaveFocus();
  });
});
