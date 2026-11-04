import { fireEvent, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ComponentProps } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { PlayerChrome, playerEndTime, playerRemainingMs } from './PlayerChrome';

type Props = ComponentProps<typeof PlayerChrome>;

const menu = (label: string) => ({
  label,
  buttonLabel: label,
  options: [
    { id: 'a', label: `${label} A` },
    { id: 'b', label: `${label} B` },
  ],
  selectedKey: 'a',
  onSelect: vi.fn(),
});

function renderChrome(props: Partial<Props> = {}) {
  const handlers = {
    onBack: vi.fn(),
    onSeek: vi.fn(),
    onSkip: vi.fn(),
    onTogglePlay: vi.fn(),
    onVolumeChange: vi.fn(),
    onToggleMute: vi.fn(),
    onToggleFullscreen: vi.fn(),
    onTimeDisplayChange: vi.fn(),
    onToggleHelp: vi.fn(),
  };
  render(
    <PlayerChrome
      title="Blade Runner 2049"
      subtitle="2017 · Science-Fiction"
      lineLabel="Blade Runner 2049 · Kapitel 3"
      currentMs={60_000}
      durationMs={600_000}
      buffered={[]}
      timeDisplay="elapsed"
      playing={false}
      volume={0.5}
      muted={false}
      fullscreen={false}
      previous={{ label: 'Vorheriges Kapitel', onPress: vi.fn() }}
      next={{ label: 'Nächstes Kapitel', onPress: vi.fn() }}
      tracks={menu('Spuren')}
      {...handlers}
      {...props}
    />,
  );
  return handlers;
}

const seek = () => screen.getByRole('slider', { name: 'Wiedergabeposition' });
const button = (name: string | RegExp) => screen.getByRole('button', { name });

describe('playerRemainingMs and playerEndTime (V-4)', () => {
  it('counts the time left in real time at the playback speed', () => {
    expect(playerRemainingMs(60_000, 600_000)).toBe(540_000);
    expect(playerRemainingMs(60_000, 600_000, 2)).toBe(270_000);
    expect(playerRemainingMs(60_000, 600_000, 0.5)).toBe(1_080_000);
    expect(playerRemainingMs(700_000, 600_000)).toBe(0);
  });

  it('ends at now plus the time left', () => {
    const now = new Date(2026, 10, 4, 22, 0, 0).getTime();
    expect(playerEndTime(now, 0, 5_460_000).getTime()).toBe(now + 5_460_000);
    // 1:30 left at 1.5×: one hour.
    expect(playerEndTime(now, 0, 5_400_000, 1.5)).toEqual(new Date(2026, 10, 4, 23, 0, 0));
  });
});

describe('PlayerChrome (concept 13, 11.2)', () => {
  afterEach(() => vi.useRealTimers());

  it('shows the title, the line under it and the line above the timeline', () => {
    renderChrome();
    expect(screen.getByRole('heading', { level: 1, name: 'Blade Runner 2049' })).toBeVisible();
    expect(screen.getByText('2017 · Science-Fiction')).toBeVisible();
    expect(screen.getByText('Blade Runner 2049 · Kapitel 3')).toBeVisible();
    expect(button('1:00 / 10:00')).toBeVisible();
  });

  it('shows the logo in place of the title once there is one', () => {
    renderChrome({ logoUrl: '/logo.webp' });
    const heading = screen.getByRole('heading', { level: 1 });
    expect(within(heading).getByRole('img', { name: 'Blade Runner 2049' })).toHaveAttribute(
      'src',
      '/logo.webp',
    );
  });

  it('asks for the other time display on a click on the time', async () => {
    const { onTimeDisplayChange } = renderChrome();
    await userEvent.click(button('1:00 / 10:00'));
    expect(onTimeDisplayChange).toHaveBeenCalledWith('remaining');
  });

  it('shows «−9:00 · endet um 22:09», and at 1.5× «−6:00 · endet um 22:06»', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 10, 4, 22, 0, 0));
    const { rerender } = render(<RemainingAt rate={1} />);
    expect(screen.getByRole('button', { name: '−9:00 · endet um 22:09' })).toBeVisible();
    rerender(<RemainingAt rate={1.5} />);
    expect(screen.getByRole('button', { name: '−6:00 · endet um 22:06' })).toBeVisible();
  });

  it('offers picture in picture only when the page passes it', () => {
    renderChrome();
    expect(screen.queryByRole('button', { name: 'Bild-in-Bild' })).toBeNull();
  });

  it('hides the choices the player cannot make (offline, F-15)', () => {
    renderChrome({
      quality: menu('Qualität'),
      version: menu('Version'),
      capabilities: { quality: false, version: false, trackSwitch: false },
    });
    expect(screen.queryByRole('button', { name: 'Spuren' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Qualität' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Version' })).toBeNull();
  });

  describe('focus model', () => {
    it('makes the row one Tab stop, on play/pause to begin with', async () => {
      renderChrome();
      const order: string[] = [];
      for (let presses = 0; presses < 6; presses += 1) {
        await userEvent.tab();
        const active = document.activeElement as HTMLElement;
        order.push(active === document.body ? 'body' : (active.getAttribute('aria-label') ?? ''));
      }
      // After the row the focus leaves the player (here: the end of the document).
      expect(order).toEqual(['Zurück', 'Vollbild', '', 'Wiedergabeposition', 'Abspielen', 'body']);
    });

    it('moves from button to button with ←/→, Home and End, and stops at both ends', async () => {
      renderChrome();
      button('Abspielen').focus();
      await userEvent.keyboard('{ArrowRight}');
      expect(button('30 Sekunden vor')).toHaveFocus();
      await userEvent.keyboard('{ArrowRight}{ArrowRight}');
      expect(button('Ton aus')).toHaveFocus();
      await userEvent.keyboard('{ArrowRight}');
      expect(button('Spuren')).toHaveFocus();
      await userEvent.keyboard('{ArrowRight}');
      expect(button('Tastenkürzel')).toHaveFocus();
      await userEvent.keyboard('{ArrowRight}');
      expect(button('Tastenkürzel')).toHaveFocus();
      await userEvent.keyboard('{Home}');
      expect(button('Vorheriges Kapitel')).toHaveFocus();
      await userEvent.keyboard('{ArrowLeft}');
      expect(button('Vorheriges Kapitel')).toHaveFocus();
      await userEvent.keyboard('{End}');
      expect(button('Tastenkürzel')).toHaveFocus();
    });

    it('keeps the last focused button as the row’s Tab stop', async () => {
      renderChrome();
      button('Abspielen').focus();
      await userEvent.keyboard('{ArrowLeft}');
      expect(button('10 Sekunden zurück')).toHaveFocus();
      expect(button('10 Sekunden zurück')).not.toHaveAttribute('tabindex', '-1');
      expect(button('Abspielen')).toHaveAttribute('tabindex', '-1');
      await userEvent.tab({ shift: true });
      expect(seek()).toHaveFocus();
      await userEvent.tab();
      expect(button('10 Sekunden zurück')).toHaveFocus();
    });

    it('↑ goes from the row to the timeline, ↓ from the timeline back to the row', async () => {
      renderChrome();
      button('Abspielen').focus();
      await userEvent.keyboard('{ArrowRight}{ArrowUp}');
      expect(seek()).toHaveFocus();
      await userEvent.keyboard('{ArrowDown}');
      expect(button('30 Sekunden vor')).toHaveFocus();
    });

    it('↑ on a menu button goes to the timeline instead of opening the menu', async () => {
      renderChrome();
      button('Spuren').focus();
      await userEvent.keyboard('{ArrowUp}');
      expect(seek()).toHaveFocus();
      expect(screen.queryByRole('menu')).toBeNull();
    });

    it('keeps the row’s keys from the page; the timeline passes ←/→ and ↑ on', () => {
      const pageKeys = vi.fn();
      window.addEventListener('keydown', pageKeys);
      try {
        renderChrome();
        fireEvent.keyDown(button('Abspielen'), { key: 'ArrowLeft' });
        fireEvent.keyDown(button('Abspielen'), { key: 'ArrowUp' });
        fireEvent.keyDown(button('Ton aus'), { key: 'ArrowDown' });
        expect(pageKeys).not.toHaveBeenCalled();
        fireEvent.keyDown(seek(), { key: 'ArrowRight' });
        fireEvent.keyDown(seek(), { key: 'ArrowUp' });
        expect(pageKeys).toHaveBeenCalledTimes(2);
        fireEvent.keyDown(seek(), { key: 'ArrowDown' });
        expect(pageKeys).toHaveBeenCalledTimes(2);
      } finally {
        window.removeEventListener('keydown', pageKeys);
      }
    });

    it('the volume slider follows its mute button, keeps ←/→ and leaves ↑ to the row', async () => {
      const { onVolumeChange } = renderChrome();
      button('Abspielen').focus();
      await userEvent.keyboard('{ArrowRight}{ArrowRight}{ArrowRight}');
      expect(button('Ton aus')).toHaveFocus();
      await userEvent.tab();
      const slider = screen.getByRole('slider', { name: 'Lautstärke' });
      expect(slider).toHaveFocus();
      await userEvent.keyboard('{ArrowRight}');
      expect(onVolumeChange).toHaveBeenCalledWith(0.55);
      expect(slider).toHaveFocus();
      await userEvent.keyboard('{ArrowUp}');
      expect(seek()).toHaveFocus();
    });

    it('↓ from the row goes to the tabs and ↑ comes back to the row', async () => {
      renderChrome({ tabs: <button type="button">Kapitel-Reiter</button> });
      button('Abspielen').focus();
      await userEvent.keyboard('{ArrowDown}');
      expect(button('Kapitel-Reiter')).toHaveFocus();
      await userEvent.keyboard('{ArrowUp}');
      expect(button('Abspielen')).toHaveFocus();
    });

    it('leaves the stop on the first button that can take the focus while play is off', () => {
      renderChrome({ transportDisabled: true });
      expect(button('Abspielen')).toBeDisabled();
      expect(button('Ton aus')).not.toHaveAttribute('tabindex', '-1');
      expect(button('Spuren')).toHaveAttribute('tabindex', '-1');
    });
  });
});

function RemainingAt({ rate }: { rate: number }) {
  return (
    <PlayerChrome
      title="Film"
      lineLabel="Film"
      currentMs={60_000}
      durationMs={600_000}
      buffered={[]}
      rate={rate}
      timeDisplay="remaining"
      onTimeDisplayChange={() => undefined}
      onSeek={() => undefined}
      playing
      onTogglePlay={() => undefined}
      onSkip={() => undefined}
      volume={1}
      muted={false}
      onVolumeChange={() => undefined}
      onToggleMute={() => undefined}
      fullscreen={false}
      onToggleFullscreen={() => undefined}
      onBack={() => undefined}
    />
  );
}
