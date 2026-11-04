import type { PlaybackInfo, Progress } from '@slouchcat/api-client';
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  alienDetail,
  episode2Detail,
  episodes,
  ids,
  playback,
  season1Detail,
  seriesIds,
  showDetail,
} from '@/test/fixtures';
import { json, mockApi, renderApp, signedIn, type Call } from '@/test/api';
import { formatBitrate } from '@/lib/format';
import { playerDriver } from '@/test/player';
import { PAUSE_VIEW_MS } from './PauseView';
import { STALL_MS } from './useSlowConnection';
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
  const { calls } = mockApi(routes(info, extra));
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
  return { media, video, router, calls };
}

const region = () => screen.getByRole('main', { name: 'Videoplayer' });
const button = (name: string | RegExp) => screen.getByRole('button', { name });
/** The gear names the rung the server delivers (11.2): «Einstellungen · Automatisch (480p)». */
const gearButton = () => button(/^Einstellungen · /);
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
    // The gear shows the rung it plays and says so in its name (review step 4, m-9).
    const rung = gearButton().textContent ?? '';
    expect(rung).not.toBe('');
    expect(gearButton()).toHaveAccessibleName(`Einstellungen · ${rung}`);
    expect(gearButton()).toHaveAttribute('aria-haspopup', 'dialog');
    await user.click(gearButton());
    const gear = await screen.findByRole('dialog', { name: 'Einstellungen' });
    expect(gearButton()).toHaveAttribute('aria-expanded', 'true');
    expect(gearButton()).toHaveAttribute('aria-controls', gear.id);
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
    expect(gearButton()).toHaveFocus();
  });

  it('the gear’s subtitle style colours the browser’s cues for picture in picture as well', async () => {
    const { video } = await start();
    const user = playerDriver().user;
    expect(video).toHaveAttribute('data-subtitle-color', 'white');
    await user.click(gearButton());
    const gear = await screen.findByRole('dialog', { name: 'Einstellungen' });
    await user.click(within(gear).getByRole('option', { name: 'Untertitel-Stil' }));
    const page = await screen.findByRole('dialog', { name: 'Untertitel-Stil' });
    await user.click(within(page).getByRole('option', { name: 'Gelb' }));
    await user.click(within(page).getByRole('option', { name: 'Box' }));
    expect(video).toHaveAttribute('data-subtitle-color', 'yellow');
    expect(video).toHaveAttribute('data-subtitle-background', 'box');
    expect(JSON.parse(localStorage.getItem('slouchcat.subtitles') ?? '{}')).toMatchObject({
      color: 'yellow',
      background: 'box',
    });
  });

  describe('sound options and 150 % (Web Audio, 11.10)', () => {
    const contexts: { sources: number; compressed: boolean; gain: { value: number } }[] = [];
    class FakeAudioContext {
      state = 'running';
      destination = {};
      record = { sources: 0, compressed: false, gain: { value: 1 } };
      constructor() {
        contexts.push(this.record);
      }
      createMediaElementSource() {
        this.record.sources += 1;
        const record = this.record;
        return {
          connect(target: { kind?: string }) {
            record.compressed = target.kind === 'compressor';
          },
          disconnect() {},
        };
      }
      createDynamicsCompressor() {
        const value = () => ({ value: 0 });
        return {
          kind: 'compressor',
          threshold: value(),
          knee: value(),
          ratio: value(),
          attack: value(),
          release: value(),
          connect() {},
        };
      }
      createGain() {
        return { kind: 'gain', gain: this.record.gain, connect() {} };
      }
      resume() {
        return Promise.resolve();
      }
      close() {
        return Promise.resolve();
      }
    }
    beforeEach(() => {
      contexts.length = 0;
      vi.stubGlobal('AudioContext', FakeAudioContext);
    });
    afterEach(() => vi.unstubAllGlobals());

    it('offers 150 % and night mode; the graph comes with the first one switched on', async () => {
      const { video } = await start();
      const user = playerDriver().user;
      const slider = screen.getByRole('slider', { name: 'Lautstärke' });
      expect(slider).toHaveAttribute('aria-valuemax', '150');
      expect(contexts).toHaveLength(0);
      act(() => slider.focus());
      for (let presses = 0; presses < 4; presses += 1) key(slider, 'ArrowRight');
      expect(slider).toHaveAttribute('aria-valuenow', '120');
      expect(slider).toHaveAttribute(
        'aria-valuetext',
        'Lautstärke 120 %, verstärkt, kann verzerren',
      );
      // The element plays 100 %, the graph adds the rest.
      expect(video.volume).toBe(1);
      expect(contexts).toHaveLength(1);
      expect(contexts[0]!.gain.value).toBeCloseTo(1.2);

      await user.click(gearButton());
      const gear = await screen.findByRole('dialog', { name: 'Einstellungen' });
      await user.click(within(gear).getByRole('option', { name: 'Ton-Optionen' }));
      const sound = await screen.findByRole('dialog', { name: 'Ton-Optionen' });
      await user.click(within(sound).getByRole('switch', { name: 'Nachtmodus' }));
      expect(contexts).toHaveLength(1);
      expect(contexts[0]!.compressed).toBe(true);
      expect(contexts[0]!.sources).toBe(1);
    });

    it('keeps night mode and 150 % into the next episode (review step 4, m-5)', async () => {
      const episode: PlaybackInfo = {
        ...playback,
        item: episodes.e2,
        resumePositionMs: 0,
        nextEpisode: episodes.e34,
      };
      const { router } = await start(episode, {
        [`GET /items/${episodes.e2.id}`]: () => json(episode2Detail),
        [`GET /items/${seriesIds.show}`]: () => json(showDetail),
        [`POST /items/${episodes.e34.id}/play`]: () =>
          json({ ...playback, item: episodes.e34, playbackSessionId: 'ps-34' }),
      });
      const user = playerDriver().user;
      const slider = screen.getByRole('slider', { name: 'Lautstärke' });
      act(() => slider.focus());
      for (let presses = 0; presses < 6; presses += 1) key(slider, 'ArrowRight');
      expect(slider).toHaveAttribute('aria-valuenow', '130');
      await user.click(gearButton());
      await user.click(
        within(await screen.findByRole('dialog', { name: 'Einstellungen' })).getByRole('option', {
          name: 'Ton-Optionen',
        }),
      );
      await user.click(
        within(await screen.findByRole('dialog', { name: 'Ton-Optionen' })).getByRole('switch', {
          name: 'Nachtmodus',
        }),
      );
      key(document.body, 'Escape');
      key(document.body, 'n');
      await waitFor(() => expect(router.state.location.pathname).toBe(`/play/${episodes.e34.id}`));
      await waitFor(() =>
        expect(screen.getByRole('slider', { name: 'Lautstärke' })).toHaveAttribute(
          'aria-valuenow',
          '130',
        ),
      );
      // The next episode's player builds its graph with night mode at once.
      await waitFor(() => expect(contexts.at(-1)?.compressed).toBe(true));
      expect(contexts.at(-1)!.gain.value).toBeCloseTo(1.3);
    });

    it('hides night mode and 150 % in Safari', async () => {
      vi.spyOn(navigator, 'userAgent', 'get').mockReturnValue(
        'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/19.0 Safari/605.1.15',
      );
      await start();
      const user = playerDriver().user;
      expect(screen.getByRole('slider', { name: 'Lautstärke' })).toHaveAttribute(
        'aria-valuemax',
        '100',
      );
      await user.click(gearButton());
      const gear = await screen.findByRole('dialog', { name: 'Einstellungen' });
      expect(within(gear).getByRole('option', { name: 'Geschwindigkeit' })).toBeVisible();
      expect(within(gear).queryByRole('option', { name: 'Ton-Optionen' })).toBeNull();
      expect(contexts).toHaveLength(0);
    });
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

describe('video player tabs, pause view, phone and slow connection (concept 13, step 5)', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    localStorage.clear();
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  // S01E02 of the show with the chapters of the film: the season loads for «Als Nächstes».
  const episode: PlaybackInfo = {
    ...film,
    item: episodes.e2,
    nextEpisode: episodes.e34,
  };
  const episodeRoutes = (info: PlaybackInfo = episode) => ({
    [`GET /items/${episodes.e2.id}`]: () => json(episode2Detail),
    [`GET /items/${seriesIds.show}`]: () => json(showDetail),
    [`GET /items/${season1Detail.id}`]: () => json(season1Detail),
    [`POST /items/${episodes.e1.id}/play`]: () =>
      json({ ...info, item: episodes.e1, playbackSessionId: 'ps-1-1' }),
    [`POST /items/${episodes.e34.id}/play`]: () =>
      json({ ...info, item: episodes.e34, playbackSessionId: 'ps-34' }),
  });
  const strip = () => screen.getByRole('toolbar', { name: 'Mehr zum Titel' });
  const tabButton = (name: string) => within(strip()).getByRole('button', { name });

  it('no pause view before the first playing nor at the end; its facts have the runtime (review step 5)', async () => {
    stubMedia();
    mockApi(routes(film));
    const { container } = renderApp(`/play/${film.item.id}`);
    const video = await waitFor(() => {
      const element = container.querySelector('video');
      if (!element) throw new Error('no video yet');
      return element;
    });
    // The player is starting: nothing has played yet.
    await act(() => vi.advanceTimersByTimeAsync(PAUSE_VIEW_MS + 100));
    expect(screen.queryByRole('region', { name: 'Pause' })).toBeNull();
    act(() => {
      video.dispatchEvent(new Event('loadedmetadata'));
    });
    await waitFor(() => expect(button('Pause')).toHaveFocus());
    key(document.body, ' ');
    await act(() => vi.advanceTimersByTimeAsync(PAUSE_VIEW_MS + 100));
    const view = screen.getByRole('region', { name: 'Pause' });
    // V-10: the year, the runtime and the genres.
    expect(within(view).getByText('1979 · 1 h 57 min · Horror, Science-Fiction')).toBeVisible();
    // The title ended: no pause view over the end.
    act(() => {
      video.dispatchEvent(new Event('ended'));
    });
    expect(screen.queryByRole('region', { name: 'Pause' })).toBeNull();
    await act(() => vi.advanceTimersByTimeAsync(PAUSE_VIEW_MS + 100));
    expect(screen.queryByRole('region', { name: 'Pause' })).toBeNull();
  });

  it('the pause view shows after 5 s of pause; a key hides it, an open panel keeps it away', async () => {
    await start();
    key(document.body, ' ');
    expect(button('Abspielen')).toBeVisible();
    await act(() => vi.advanceTimersByTimeAsync(PAUSE_VIEW_MS - 500));
    expect(screen.queryByRole('region', { name: 'Pause' })).toBeNull();
    await act(() => vi.advanceTimersByTimeAsync(600));
    const view = screen.getByRole('region', { name: 'Pause' });
    expect(within(view).getByText(alienDetail.overview!)).toBeVisible();
    expect(within(view).getByText('Kapitel: Anfang')).toBeVisible();
    // It takes no focus; the controls keep it.
    expect(button('Abspielen')).toHaveFocus();
    key(document.body, 'Shift');
    expect(screen.queryByRole('region', { name: 'Pause' })).toBeNull();
    await act(() => vi.advanceTimersByTimeAsync(PAUSE_VIEW_MS + 100));
    expect(screen.getByRole('region', { name: 'Pause' })).toBeVisible();
    // A panel is open: no pause view.
    key(document.body, '?');
    await act(() => vi.advanceTimersByTimeAsync(PAUSE_VIEW_MS + 100));
    expect(screen.queryByRole('region', { name: 'Pause' })).toBeNull();
    // Playing again hides it for good.
    key(document.body, 'Escape');
    key(document.body, 'k');
    await act(() => vi.advanceTimersByTimeAsync(PAUSE_VIEW_MS + 100));
    expect(screen.queryByRole('region', { name: 'Pause' })).toBeNull();
  });

  it('C opens the chapter tab on the chapter playing; Escape closes it, the focus on its tab', async () => {
    await start();
    expect(tabButton('Kapitel')).toHaveAttribute('aria-expanded', 'false');
    // The chapter menu is gone from the row.
    expect(
      within(screen.getByRole('toolbar', { name: 'Wiedergabe' })).queryByRole('button', {
        name: 'Kapitel',
      }),
    ).toBeNull();
    key(document.body, 'c');
    const cards = await screen.findByRole('list', { name: 'Kapitel' });
    const current = within(cards).getByRole('button', { current: true });
    expect(current).toHaveAccessibleName(/^Anfang 0:00 /);
    await waitFor(() => expect(current).toHaveFocus());
    // The controls stay while a tab is open.
    await act(() => vi.advanceTimersByTimeAsync(CONTROLS_HIDE_MS + 100));
    expect(region()).toHaveAttribute('data-controls', 'visible');
    key(current, 'Escape');
    expect(screen.queryByRole('list', { name: 'Kapitel' })).toBeNull();
    expect(tabButton('Kapitel')).toHaveFocus();
  });

  it('«Als Nächstes» shows the season, starts on the next episode and plays a pick', async () => {
    const { router } = await start(episode, episodeRoutes());
    expect(tabButton('Besetzung')).toBeVisible();
    expect(tabButton('Info')).toBeVisible();
    const { user } = playerDriver();
    await user.click(await within(strip()).findByRole('button', { name: 'Als Nächstes' }));
    const cards = screen.getByRole('list', { name: 'Als Nächstes' });
    const names = within(cards)
      .getAllByRole('button')
      .map((card) => card.textContent);
    expect(names.map((name) => name?.slice(0, 6))).toEqual(['S01E01', 'S01E02', 'S01E03']);
    expect(within(cards).getByRole('button', { current: true })).toHaveTextContent(/^S01E02/);
    // The row's Tab stop is the episode after this one.
    const following = within(cards).getAllByRole('button')[2]!;
    expect(following).not.toHaveAttribute('tabindex', '-1');
    await user.click(following);
    await waitFor(() => expect(router.state.location.pathname).toBe(`/play/${episodes.e34.id}`));
  });

  it('«Besetzung» opens the person; «Info» shows the description', async () => {
    const { router } = await start(episode, episodeRoutes());
    const { user } = playerDriver();
    await user.click(await within(strip()).findByRole('button', { name: 'Info' }));
    expect(screen.getByText(episode2Detail.overview!)).toBeVisible();
    await user.click(tabButton('Besetzung'));
    await user.click(screen.getByRole('button', { name: /^Lena Muster/ }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/people/7'));
  });

  it('another episode asks for the season again: the one just seen shows its progress (review step 6)', async () => {
    const { router, calls } = await start(episode, {
      ...episodeRoutes(),
      [`GET /items/${episodes.e34.id}`]: () =>
        json({ ...episode2Detail, ...episodes.e34, nextEpisode: undefined }),
    });
    const seasonCalls = () =>
      calls.filter((call) => call.method === 'GET' && call.path === `/items/${season1Detail.id}`)
        .length;
    const { user } = playerDriver();
    await user.click(await within(strip()).findByRole('button', { name: 'Als Nächstes' }));
    expect(seasonCalls()).toBe(1);
    await user.click(
      within(screen.getByRole('list', { name: 'Als Nächstes' })).getAllByRole('button')[2]!,
    );
    await waitFor(() => expect(router.state.location.pathname).toBe(`/play/${episodes.e34.id}`));
    // PlayerPage mounts a new view per episode, whose cleanup invalidated the season: the next
    // player does not take it from the 30-s cache.
    await waitFor(() => expect(seasonCalls()).toBeGreaterThan(1));
  });

  it('back is «Vorherige Folge» without chapters, and in the first seconds of the first chapter', async () => {
    const { router } = await start(
      { ...episode, chapters: [], resumePositionMs: 0 },
      episodeRoutes(),
    );
    const { user } = playerDriver();
    await user.click(await screen.findByRole('button', { name: 'Vorherige Folge' }));
    await waitFor(() => expect(router.state.location.pathname).toBe(`/play/${episodes.e1.id}`));
  });

  it('a slow connection suggests the rung below; the switch is not remembered (V-17, V-18)', async () => {
    const { video } = await start();
    const stall = async () => {
      act(() => {
        video.dispatchEvent(new Event('waiting'));
      });
      await act(() => vi.advanceTimersByTimeAsync(STALL_MS + 100));
      act(() => {
        video.dispatchEvent(new Event('playing'));
      });
    };
    await stall();
    expect(screen.queryByRole('region', { name: 'Langsame Verbindung' })).toBeNull();
    await stall();
    const hint = screen.getByRole('region', { name: 'Langsame Verbindung' });
    expect(within(hint).getByRole('status')).toHaveTextContent(
      'Verbindung langsam – auf 720p wechseln?',
    );
    const { user } = playerDriver();
    await user.click(within(hint).getByRole('button', { name: 'Auf 720p wechseln' }));
    await waitFor(() =>
      expect(screen.queryByRole('region', { name: 'Langsame Verbindung' })).toBeNull(),
    );
    expect(JSON.parse(localStorage.getItem(PLAYER_PREFS_KEY) ?? '{}')).not.toHaveProperty(
      'quality',
    );
  });

  it('a choice in the quality panel is remembered on the device (V-17)', async () => {
    await start();
    const player = playerDriver();
    await player.chooseQuality('480p');
    expect(JSON.parse(localStorage.getItem(PLAYER_PREFS_KEY) ?? '{}')).toMatchObject({
      quality: '480p',
    });
  });

  describe('on a phone', () => {
    beforeEach(() => {
      vi.stubGlobal('matchMedia', (query: string) => ({
        matches: query.includes('768px'),
        media: query,
        addEventListener: () => undefined,
        removeEventListener: () => undefined,
      }));
    });

    it('no tabs; C opens the chapters in the gear; two taps on a half seek 10 s', async () => {
      const { media, video } = await start();
      expect(screen.queryByRole('toolbar', { name: 'Mehr zum Titel' })).toBeNull();
      expect(screen.getByRole('toolbar', { name: 'Optionen' })).toBeVisible();
      key(document.body, 'c');
      const chapters = await screen.findByRole('dialog', { name: 'Kapitel' });
      expect(within(chapters).getByRole('option', { selected: true })).toHaveAccessibleName(
        /^Anfang/,
      );
      // A pick jumps and closes the page (review step 5).
      fireEvent.click(within(chapters).getByRole('option', { name: /^Mitte/ }));
      expect(media.currentTime).toBe(120);
      expect(screen.queryByRole('dialog', { name: 'Kapitel' })).toBeNull();

      vi.spyOn(video, 'getBoundingClientRect').mockReturnValue(new DOMRect(0, 0, 400, 800));
      media.currentTime = 100;
      const tap = (x: number) => {
        fireEvent.pointerDown(video, { button: 0, clientX: x, pointerType: 'touch' });
        fireEvent.pointerUp(video, { button: 0, clientX: x, pointerType: 'touch' });
      };
      tap(50);
      tap(50);
      expect(media.currentTime).toBe(90);
      expect(screen.getByRole('status')).toHaveTextContent('−10 s');
      tap(350);
      tap(350);
      expect(media.currentTime).toBe(100);
      // One tap does not play or pause.
      const paused = media.paused;
      tap(200);
      await act(() => vi.advanceTimersByTimeAsync(400));
      expect(media.paused).toBe(paused);
      // A mouse in the phone layout of a narrow window: a click pauses (review step 5).
      fireEvent.pointerDown(video, { button: 0, clientX: 200, pointerType: 'mouse' });
      fireEvent.pointerUp(video, { button: 0, clientX: 200, pointerType: 'mouse' });
      expect(media.paused).toBe(!paused);
    });

    it('a tab open when the window narrows closes, so the bar fades again (review step 5)', async () => {
      let narrow = false;
      const listeners = new Set<() => void>();
      vi.stubGlobal('matchMedia', (query: string) => ({
        get matches() {
          return narrow && query.includes('768px');
        },
        media: query,
        addEventListener: (_type: string, listener: () => void) => listeners.add(listener),
        removeEventListener: (_type: string, listener: () => void) => listeners.delete(listener),
      }));
      await start();
      key(document.body, 'c');
      await screen.findByRole('list', { name: 'Kapitel' });
      narrow = true;
      act(() => listeners.forEach((listener) => listener()));
      expect(screen.queryByRole('list', { name: 'Kapitel' })).toBeNull();
      await act(() => vi.advanceTimersByTimeAsync(CONTROLS_HIDE_MS + 100));
      expect(region()).toHaveAttribute('data-controls', 'hidden');
    });
  });
});
