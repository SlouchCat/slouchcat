import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useEffect, useState, type ComponentProps } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { PlayerChrome, playerEndTime, playerRemainingMs } from './PlayerChrome';

type Props = ComponentProps<typeof PlayerChrome>;

const panel = (label: string, text?: string) => ({
  label,
  text,
  expanded: false,
  onPress: vi.fn(),
});

/** A tab with a row of `count` cards named «<label> <n>», or with text. */
const cardTab = (id: string, label: string, count: number, current?: number) => ({
  id,
  label,
  cards: {
    count,
    current,
    render: (index: number, excluded: boolean) => (
      <button type="button" tabIndex={excluded ? -1 : 0}>
        {`${label} ${index + 1}`}
      </button>
    ),
  },
});
const textTab = (id: string, label: string, text: string) => ({
  id,
  label,
  content: <p>{text}</p>,
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
      headingDetail="2017 · Science-Fiction"
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
      subtitles={panel('Untertitel: Aus', 'Aus')}
      audio={panel('Ton: Deutsch', 'Deutsch')}
      settings={panel('Einstellungen')}
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

  it('shows the panels with the language as a word; leaves out what the page does not pass', () => {
    renderChrome();
    expect(button('Ton: Deutsch')).toHaveTextContent('Deutsch');
    expect(button('Ton: Deutsch')).toHaveAttribute('aria-expanded', 'false');
    expect(button('Untertitel: Aus')).toHaveTextContent('Aus');
    expect(button('Einstellungen')).toHaveTextContent('');
    cleanup();
    // Offline (F-15): no server to switch the tracks through.
    renderChrome({ subtitles: undefined, audio: undefined });
    expect(screen.queryByRole('button', { name: /^Ton: / })).toBeNull();
    expect(screen.queryByRole('button', { name: /^Untertitel: / })).toBeNull();
    expect(button('Einstellungen')).toBeVisible();
  });

  it('reports a press on a panel button and marks it open', async () => {
    const audio = { ...panel('Ton: Deutsch', 'Deutsch'), expanded: true };
    renderChrome({ audio });
    expect(button('Ton: Deutsch')).toHaveAttribute('aria-expanded', 'true');
    await userEvent.click(button('Ton: Deutsch'));
    expect(audio.onPress).toHaveBeenCalledTimes(1);
  });

  it('passes the boost and the mute-only mode to the volume control', () => {
    renderChrome({ volumeBoost: true, volume: 1.2 });
    expect(screen.getByRole('slider', { name: 'Lautstärke' })).toHaveAttribute(
      'aria-valuemax',
      '150',
    );
    cleanup();
    renderChrome({ muteOnly: true });
    expect(screen.queryByRole('slider', { name: 'Lautstärke' })).toBeNull();
    expect(button('Ton aus')).toBeVisible();
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
      expect(button('Untertitel: Aus')).toHaveFocus();
      await userEvent.keyboard('{ArrowRight}');
      expect(button('Ton: Deutsch')).toHaveFocus();
      await userEvent.keyboard('{ArrowRight}');
      expect(button('Einstellungen')).toHaveFocus();
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
      renderChrome({ tabs: [cardTab('chapters', 'Kapitel', 3), textTab('info', 'Info', 'Text')] });
      button('Abspielen').focus();
      await userEvent.keyboard('{ArrowDown}');
      expect(button('Kapitel')).toHaveFocus();
      await userEvent.keyboard('{ArrowUp}');
      expect(button('Abspielen')).toHaveFocus();
    });

    it('leaves the stop on the first button that can take the focus while play is off', () => {
      renderChrome({ transportDisabled: true });
      expect(button('Abspielen')).toBeDisabled();
      expect(button('Ton aus')).not.toHaveAttribute('tabindex', '-1');
      expect(button('Einstellungen')).toHaveAttribute('tabindex', '-1');
    });
  });
});

/** The chrome with the tabs of 11.2 and the open tab in state, like the video player. */
function Tabbed({ initial = null, current = 1 }: { initial?: string | null; current?: number }) {
  const [open, setOpen] = useState<string | null>(initial);
  // Escape closes the open tab, as the player's key map does.
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(null);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);
  return (
    <>
      <PlayerChrome
        title="Silo"
        lineLabel="Silo"
        currentMs={0}
        durationMs={600_000}
        buffered={[]}
        timeDisplay="elapsed"
        onTimeDisplayChange={vi.fn()}
        onSeek={vi.fn()}
        playing={false}
        onTogglePlay={vi.fn()}
        onSkip={vi.fn()}
        volume={1}
        muted={false}
        onVolumeChange={vi.fn()}
        onToggleMute={vi.fn()}
        fullscreen={false}
        onToggleFullscreen={vi.fn()}
        onBack={vi.fn()}
        settings={panel('Einstellungen')}
        tabs={[
          cardTab('chapters', 'Kapitel', 4, current),
          cardTab('next', 'Als Nächstes', 2),
          textTab('info', 'Info', 'Eine Stadt unter der Erde.'),
        ]}
        openTab={open}
        onOpenTab={setOpen}
      />
      <button type="button" onClick={() => setOpen(null)}>
        Reiter schliessen
      </button>
    </>
  );
}

describe('PlayerChrome tabs (11.2, step 5)', () => {
  const strip = () => screen.getByRole('toolbar', { name: 'Mehr zum Titel' });
  const tabButton = (name: string) => within(strip()).getByRole('button', { name });

  it('opens a tab from the strip; Enter on it again closes it', async () => {
    render(<Tabbed />);
    expect(tabButton('Kapitel')).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByRole('region', { name: 'Kapitel' })).toBeNull();
    await userEvent.click(tabButton('Kapitel'));
    const region = screen.getByRole('region', { name: 'Kapitel' });
    expect(tabButton('Kapitel')).toHaveAttribute('aria-expanded', 'true');
    expect(tabButton('Kapitel')).toHaveAttribute('aria-controls', region.id);
    expect(within(region).getAllByRole('button')).toHaveLength(4);
    tabButton('Kapitel').focus();
    await userEvent.keyboard('{Enter}');
    expect(screen.queryByRole('region', { name: 'Kapitel' })).toBeNull();
  });

  it('strip: one Tab stop, ←/→ from tab to tab; ↓ into the cards on the current one', async () => {
    render(<Tabbed initial="chapters" />);
    button('Abspielen').focus();
    await userEvent.keyboard('{ArrowDown}');
    expect(tabButton('Kapitel')).toHaveFocus();
    await userEvent.keyboard('{ArrowRight}');
    expect(tabButton('Als Nächstes')).toHaveFocus();
    expect(tabButton('Kapitel')).toHaveAttribute('tabindex', '-1');
    await userEvent.keyboard('{ArrowRight}{ArrowRight}');
    expect(tabButton('Info')).toHaveFocus();
    await userEvent.keyboard('{Home}');
    expect(tabButton('Kapitel')).toHaveFocus();
    // ↓: the open tab's row of cards, on the card it names as current (the chapter playing).
    await userEvent.keyboard('{ArrowDown}');
    expect(button('Kapitel 2')).toHaveFocus();
    expect(button('Kapitel 1')).toHaveAttribute('tabindex', '-1');
  });

  it('cards: ←/→ from card to card, stop at the ends; ↑ back to the strip, then to the row', async () => {
    const pageKeys = vi.fn();
    window.addEventListener('keydown', pageKeys);
    try {
      render(<Tabbed initial="chapters" />);
      button('Kapitel 2').focus();
      await userEvent.keyboard('{ArrowRight}{ArrowRight}');
      expect(button('Kapitel 4')).toHaveFocus();
      await userEvent.keyboard('{ArrowRight}');
      expect(button('Kapitel 4')).toHaveFocus();
      await userEvent.keyboard('{Home}');
      expect(button('Kapitel 1')).toHaveFocus();
      // The arrows never reach the player (they would seek or change the volume).
      await userEvent.keyboard('{ArrowLeft}{ArrowDown}');
      expect(button('Kapitel 1')).toHaveFocus();
      expect(pageKeys).not.toHaveBeenCalled();
      await userEvent.keyboard('{ArrowUp}');
      expect(tabButton('Kapitel')).toHaveFocus();
      await userEvent.keyboard('{ArrowUp}');
      expect(button('Abspielen')).toHaveFocus();
    } finally {
      window.removeEventListener('keydown', pageKeys);
    }
  });

  it('without the focus in it the row’s stop follows the current card (review step 5)', async () => {
    const view = render(<Tabbed initial="chapters" current={1} />);
    expect(button('Kapitel 2')).toHaveAttribute('tabindex', '0');
    // The next chapter plays: the stop moves with it.
    view.rerender(<Tabbed initial="chapters" current={2} />);
    expect(button('Kapitel 3')).toHaveAttribute('tabindex', '0');
    expect(button('Kapitel 2')).toHaveAttribute('tabindex', '-1');
    // In the row the focused card stays the stop; once the focus left, the current one again.
    button('Kapitel 1').focus();
    view.rerender(<Tabbed initial="chapters" current={3} />);
    expect(button('Kapitel 1')).toHaveAttribute('tabindex', '0');
    await userEvent.keyboard('{ArrowUp}');
    expect(tabButton('Kapitel')).toHaveFocus();
    expect(button('Kapitel 4')).toHaveAttribute('tabindex', '0');
  });

  it('Tab walks row, strip and cards as one stop each; no trap', async () => {
    render(<Tabbed initial="next" />);
    button('Abspielen').focus();
    await userEvent.tab();
    expect(tabButton('Als Nächstes')).toHaveFocus();
    await userEvent.tab();
    expect(button('Als Nächstes 1')).toHaveFocus();
    await userEvent.tab();
    expect(button('Reiter schliessen')).toHaveFocus();
  });

  it('the text of «Info» takes the focus with ↓; the focus goes to its tab when it closes', async () => {
    render(<Tabbed initial="info" />);
    tabButton('Info').focus();
    await userEvent.keyboard('{ArrowDown}');
    const text = screen.getByText('Eine Stadt unter der Erde.').parentElement!;
    expect(text).toHaveFocus();
    // Scrolled down, ↑ scrolls the text back natively; at its top ↑ goes to the strip.
    text.scrollTop = 40;
    const up = fireEvent.keyDown(text, { key: 'ArrowUp' });
    expect(up).toBe(true);
    expect(text).toHaveFocus();
    text.scrollTop = 0;
    await userEvent.keyboard('{ArrowUp}');
    expect(tabButton('Info')).toHaveFocus();
    await userEvent.keyboard('{ArrowDown}');
    expect(text).toHaveFocus();
    // Closed by the page (Escape) with the focus in it: the focus goes to its tab.
    await userEvent.keyboard('{Escape}');
    expect(screen.queryByRole('region', { name: 'Info' })).toBeNull();
    expect(tabButton('Info')).toHaveFocus();
    expect(tabButton('Info')).toHaveAttribute('aria-expanded', 'false');
  });

  it('a phone: options at the top, the transport in the middle (−10, play, +10), no tabs', async () => {
    const { onSkip } = renderChrome({
      layout: 'phone',
      muteOnly: true,
      tabs: [cardTab('chapters', 'Kapitel', 2)],
    });
    expect(screen.queryByRole('toolbar', { name: 'Mehr zum Titel' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Tastenkürzel' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Vorheriges Kapitel' })).toBeNull();
    // iPhone: only the mute button.
    expect(screen.queryByRole('slider', { name: 'Lautstärke' })).toBeNull();
    const options = screen.getByRole('toolbar', { name: 'Optionen' });
    expect(
      within(options)
        .getAllByRole('button')
        .map((b) => b.getAttribute('aria-label')),
    ).toEqual(['Ton aus', 'Untertitel: Aus', 'Ton: Deutsch', 'Einstellungen']);
    // Icons only at the top right.
    expect(within(options).getByRole('button', { name: 'Ton: Deutsch' })).toHaveTextContent('');
    const transport = screen.getByRole('toolbar', { name: 'Wiedergabe' });
    expect(
      within(transport)
        .getAllByRole('button')
        .map((b) => b.getAttribute('aria-label')),
    ).toEqual(['10 Sekunden zurück', 'Abspielen', '10 Sekunden vor']);
    await userEvent.click(button('10 Sekunden vor'));
    expect(onSkip).toHaveBeenCalledWith(10);

    // Focus order follows the layout: options ↓ transport ↓ timeline, ↑ back up.
    button('Abspielen').focus();
    await userEvent.keyboard('{ArrowUp}');
    expect(button('Ton aus')).toHaveFocus();
    await userEvent.keyboard('{ArrowRight}');
    expect(button('Untertitel: Aus')).toHaveFocus();
    await userEvent.keyboard('{ArrowDown}');
    expect(button('Abspielen')).toHaveFocus();
    await userEvent.keyboard('{ArrowDown}');
    expect(seek()).toHaveFocus();
    await userEvent.keyboard('{ArrowUp}');
    expect(button('Abspielen')).toHaveFocus();
    // Tab: head (back, options, full screen), the transport, the time, the timeline.
    const order: string[] = [];
    (document.activeElement as HTMLElement).blur();
    for (let presses = 0; presses < 6; presses += 1) {
      await userEvent.tab();
      order.push((document.activeElement as HTMLElement).getAttribute('aria-label') ?? '');
    }
    expect(order).toEqual([
      'Zurück',
      'Untertitel: Aus',
      'Vollbild',
      'Abspielen',
      '',
      'Wiedergabeposition',
    ]);
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
