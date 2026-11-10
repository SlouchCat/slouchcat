import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, fireEvent, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { initialRadioState, type RadioListen, type RadioStation } from './priority';
import { RadioDock } from './RadioDock';
import type { RadioNowEntry } from './queries';
import { RadioContext, RadioOffsetContext, type RadioPlayer } from './useRadio';

const station: RadioStation = {
  id: 7,
  name: 'Radio Swiss Pop',
  country: 'CH',
  languages: ['de'],
  genres: ['pop'],
  kinds: ['music'],
  moods: [],
  hls: false,
  showsTitles: true,
  favorite: false,
  recommended: false,
  state: 'ok',
  logo: { url: '/radio/images/7/logo/w256?v=l1' },
};

const listen = (timeShiftSeconds: number): RadioListen => ({
  listenId: 'L1',
  playlistUrl: '/radio/live/L1/index.m3u8?rt=token',
  heartbeatSeconds: 30,
  startOffsetSeconds: -8,
  timeShiftSeconds,
  station,
});

const entry = (at: number, artist: string, title: string, cover?: string): RadioNowEntry => ({
  at,
  text: `${artist} - ${title}`,
  artist,
  title,
  ...(cover ? { cover: { url: `/radio/covers/${cover}/w256?v=${cover}` } } : {}),
});

function renderDock({ offset = 0, ...overrides }: Partial<RadioPlayer> & { offset?: number } = {}) {
  const timeShiftSeconds = overrides.timeShiftSeconds ?? 1800;
  const player: RadioPlayer = {
    state: { ...initialRadioState, phase: 'live', station, listen: listen(timeShiftSeconds) },
    playing: true,
    entry: entry(Date.UTC(2026, 10, 8, 13, 30), 'Dua Lipa', 'Houdini', '31'),
    history: [
      entry(Date.UTC(2026, 10, 8, 13, 26), 'Adele', 'Hello', '30'),
      entry(Date.UTC(2026, 10, 8, 13, 22), 'Miley Cyrus', 'Flowers'),
    ],
    timeShiftSeconds,
    longPause: false,
    play: vi.fn(),
    toggle: vi.fn(),
    stop: vi.fn(),
    toLive: vi.fn(),
    seek: vi.fn(),
    hold: () => () => undefined,
    markFavorite: vi.fn(),
    dismiss: vi.fn(),
    volume: 0.8,
    setVolume: vi.fn(),
    muted: false,
    toggleMute: vi.fn(),
    ...overrides,
  };
  const queryClient = new QueryClient();
  render(
    <QueryClientProvider client={queryClient}>
      <RadioContext.Provider value={player}>
        <RadioOffsetContext.Provider value={offset}>
          <RadioDock />
        </RadioOffsetContext.Provider>
      </RadioContext.Provider>
    </QueryClientProvider>,
  );
  return player;
}

describe('RadioDock', () => {
  it('shows «Live» at live and the offset behind it', () => {
    renderDock({ offset: 2 });
    expect(screen.getByTestId('radio-mini-state')).toHaveTextContent('Live');
    expect(screen.getByTestId('radio-mini-player')).toHaveAttribute('data-state', 'live');
  });

  it('shows «−12:30» behind live', () => {
    renderDock({ offset: 750 });
    expect(screen.getByTestId('radio-mini-state')).toHaveTextContent('−12:30');
    expect(screen.getByTestId('radio-mini-player')).toHaveAttribute('data-state', 'shifted');
  });

  it('shows the cover of the title in the mini player, the logo without one', () => {
    renderDock();
    const mini = screen.getByTestId('radio-mini-open');
    expect(mini.querySelector('img')).toHaveAttribute('src', '/api/v1/radio/covers/31/w96?v=31');
  });

  it('names a long pause and goes live from the note', async () => {
    const player = renderDock({ offset: 1796, longPause: true });
    const hint = screen.getByTestId('radio-mini-hint');
    expect(hint).toHaveTextContent(
      'Die Pause war länger als 30 Minuten. Weiter ab dem ältesten Stand.',
    );
    await userEvent.click(within(hint).getByRole('button', { name: 'Live' }));
    expect(player.toLive).toHaveBeenCalled();
  });

  it('opens «Jetzt läuft» with the slider, the large cover and «Zuvor gespielt»', async () => {
    const player = renderDock({ offset: 750 });
    await userEvent.click(screen.getByTestId('radio-mini-open'));
    const dialog = await screen.findByTestId('radio-now-playing');
    const slider = within(dialog).getByRole('slider', {
      name: 'Zeitversatz, bis 30 Minuten zurück',
    });
    expect(slider).toHaveAttribute('aria-valuetext', '12 Minuten 30 Sekunden zurück');
    expect(slider).toHaveAttribute('min', '-1800');
    expect(slider).toHaveValue('-750');
    expect(within(dialog).getByTestId('radio-now-live')).toHaveAttribute('aria-pressed', 'false');
    expect(
      [...document.querySelectorAll('img')].some(
        (image) => image.getAttribute('src') === '/api/v1/radio/covers/31/w512?v=31',
      ),
    ).toBe(true);
    const played = within(dialog).getByTestId('radio-played');
    const items = within(played).getAllByRole('listitem');
    expect(items.map((item) => item.querySelector('span > span')?.textContent)).toEqual([
      'Hello',
      'Flowers',
    ]);
    // An entry without a cover shows the station's logo.
    expect(items[1]?.querySelector('img')).toHaveAttribute(
      'src',
      '/api/v1/radio/images/7/logo/w256?v=l1',
    );
    await userEvent.click(within(dialog).getByTestId('radio-now-live'));
    expect(player.toLive).toHaveBeenCalled();
  });

  it('hides the slider while the server keeps no buffer', async () => {
    renderDock({ timeShiftSeconds: 0 });
    await userEvent.click(screen.getByTestId('radio-mini-open'));
    const dialog = await screen.findByTestId('radio-now-playing');
    expect(within(dialog).queryByRole('slider', { name: /^Zeitversatz/ })).toBeNull();
    expect(within(dialog).getByTestId('radio-now-live')).toHaveAttribute('aria-pressed', 'true');
  });

  describe('volume (concept 19 E3)', () => {
    afterEach(() => {
      vi.restoreAllMocks();
    });

    it('sits beside play/pause in the mini player and under the controls in «Jetzt läuft»', async () => {
      const player = renderDock({ volume: 0.4 });
      const mini = screen.getByTestId('radio-mini-player');
      const slider = await within(mini).findByRole('slider', { name: 'Lautstärke' });
      expect(slider).toHaveAttribute('aria-valuenow', '40');
      // No boost: the radio has no gain node.
      expect(slider).toHaveAttribute('aria-valuemax', '100');
      // ←/→ on the slider set the shared volume.
      act(() => slider.focus());
      fireEvent.keyDown(slider, { key: 'ArrowRight' });
      expect(player.setVolume).toHaveBeenLastCalledWith(0.45);
      await userEvent.click(within(mini).getByRole('button', { name: 'Ton aus' }));
      expect(player.toggleMute).toHaveBeenCalledTimes(1);

      await userEvent.click(screen.getByTestId('radio-mini-open'));
      const dialog = await screen.findByTestId('radio-now-playing');
      const volume = within(dialog).getByTestId('radio-now-volume');
      expect(within(volume).getByRole('slider', { name: 'Lautstärke' })).toHaveAttribute(
        'aria-valuenow',
        '40',
      );
    });

    it('M mutes while the focus is in the mini player or in «Jetzt läuft»', async () => {
      const player = renderDock();
      const toggle = screen.getByTestId('radio-mini-toggle');
      act(() => toggle.focus());
      await userEvent.keyboard('m');
      expect(player.toggleMute).toHaveBeenCalledTimes(1);
      // Not with a modifier (a shortcut of the browser).
      await userEvent.keyboard('{Control>}m{/Control}');
      expect(player.toggleMute).toHaveBeenCalledTimes(1);

      await userEvent.click(screen.getByTestId('radio-mini-open'));
      const dialog = await screen.findByTestId('radio-now-playing');
      act(() => within(dialog).getByTestId('radio-now-live').focus());
      await userEvent.keyboard('M');
      expect(player.toggleMute).toHaveBeenCalledTimes(2);
    });

    it('PE-4: on the iPhone only the mute button, in the mini player and «Jetzt läuft»', async () => {
      vi.spyOn(navigator, 'userAgent', 'get').mockReturnValue(
        'Mozilla/5.0 (iPhone; CPU iPhone OS 19_0 like Mac OS X) AppleWebKit/605.1.15',
      );
      renderDock();
      const mini = screen.getByTestId('radio-mini-player');
      expect(await within(mini).findByRole('button', { name: 'Ton aus' })).toBeVisible();
      expect(within(mini).queryByRole('slider', { name: 'Lautstärke' })).toBeNull();
      await userEvent.click(screen.getByTestId('radio-mini-open'));
      const volume = within(await screen.findByTestId('radio-now-playing')).getByTestId(
        'radio-now-volume',
      );
      expect(within(volume).getByRole('button', { name: 'Ton aus' })).toBeVisible();
      expect(within(volume).queryByRole('slider')).toBeNull();
    });
  });
});
