import type { Chapter, Track } from '@slouchcat/api-client';
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { createRef, type ComponentProps } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { OfflineVideo } from './OfflineVideo';

type Props = ComponentProps<typeof OfflineVideo>;

const audioTrack: Track = {
  id: 11,
  index: 1,
  kind: 'audio',
  codec: 'aac',
  language: 'eng',
  default: true,
  forced: false,
  source: 'embedded',
  text: false,
  channels: 2,
};

const subtitleTrack = (id: number, language: string): Track => ({
  id,
  index: id,
  kind: 'subtitle',
  codec: 'webvtt',
  language,
  default: false,
  forced: false,
  source: 'embedded',
  text: true,
});

const chapters: Chapter[] = [
  { startMs: 0, endMs: 20_000, title: 'Anfang' },
  { startMs: 20_000, endMs: 40_000, title: 'Mitte' },
  { startMs: 40_000, endMs: 60_000, title: 'Ende' },
];

function renderVideo(props: Partial<Props> = {}) {
  const handlers = {
    togglePlay: vi.fn(),
    seekTo: vi.fn(),
    seekBy: vi.fn(),
    onBack: vi.fn(),
  };
  const view = render(
    <OfflineVideo
      media={createRef<HTMLMediaElement>()}
      src={undefined}
      title="Alien"
      durationMs={60_000}
      currentMs={25_000}
      positionOf={() => 25_000}
      buffered={[]}
      playing={false}
      wantsPlaying={() => false}
      chapters={[]}
      audioTrack={audioTrack}
      subtitles={[
        { track: subtitleTrack(21, 'ger'), url: 'blob:de' },
        { track: subtitleTrack(22, 'eng'), url: 'blob:en' },
      ]}
      profileLanguage="de"
      failed={false}
      limitReached={false}
      limitMinutes={undefined}
      data={{ 'data-engine': 'mse' }}
      {...handlers}
      {...props}
    />,
  );
  return { ...view, ...handlers };
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe('offline video player on PlayerChrome (F-15)', () => {
  it('gives the first focus to play/pause and offers only what the download has', async () => {
    renderVideo();
    const region = screen.getByRole('main', { name: 'Videoplayer' });
    expect(region).toHaveAttribute('data-engine', 'mse');
    await waitFor(() => expect(screen.getByRole('button', { name: 'Abspielen' })).toHaveFocus());
    expect(screen.getByRole('toolbar', { name: 'Wiedergabe' })).toBeVisible();
    expect(screen.getByRole('button', { name: 'Ton: Englisch' })).toBeVisible();
    expect(screen.getByRole('button', { name: 'Untertitel: Aus' })).toBeVisible();
    // Without chapters: no chapter steps and no tabs; no picture in picture (W5).
    expect(screen.queryByRole('button', { name: 'Vorheriges Kapitel' })).toBeNull();
    expect(screen.queryByRole('toolbar', { name: 'Reiter' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Bild in Bild' })).toBeNull();

    // The gear: speed and subtitle style, no quality, version or stream info.
    fireEvent.click(screen.getByRole('button', { name: 'Einstellungen' }));
    const gear = screen.getByRole('dialog', { name: 'Einstellungen' });
    expect(within(gear).getByRole('option', { name: /Geschwindigkeit/ })).toBeVisible();
    expect(within(gear).getByRole('option', { name: /Untertitel-Stil/ })).toBeVisible();
    expect(within(gear).queryByRole('option', { name: /Qualität/ })).toBeNull();
    expect(within(gear).queryByRole('option', { name: /Version/ })).toBeNull();
    expect(within(gear).queryByRole('switch', { name: 'Stream-Info' })).toBeNull();
  });

  it('opens the panels with U and T, picks a stored subtitle locally and closes with Escape', () => {
    const { container, onBack } = renderVideo();
    fireEvent.keyDown(window, { key: 'u' });
    const panel = screen.getByRole('dialog', { name: 'Untertitel' });
    // The profile's language first (11.10).
    const rows = within(panel).getAllByRole('option');
    expect(rows.map((row) => row.textContent)).toEqual([
      expect.stringContaining('Aus'),
      expect.stringContaining('Deutsch'),
      expect.stringContaining('Englisch'),
    ]);
    fireEvent.click(within(panel).getByRole('option', { name: /Deutsch/ }));
    expect(screen.queryByRole('dialog', { name: 'Untertitel' })).toBeNull();
    expect(container.querySelector('track')).toHaveAttribute('src', 'blob:de');
    expect(screen.getByRole('button', { name: 'Untertitel: Deutsch' })).toBeVisible();

    fireEvent.keyDown(window, { key: 't' });
    const sound = screen.getByRole('dialog', { name: 'Ton' });
    expect(within(sound).getAllByRole('option')).toHaveLength(1);
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(screen.queryByRole('dialog', { name: 'Ton' })).toBeNull();
    expect(onBack).not.toHaveBeenCalled();
    // Nothing open: Escape leaves the player.
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(onBack).toHaveBeenCalledTimes(1);
  });

  it('plays, seeks and changes the volume through the shared key map', () => {
    const { togglePlay, seekBy, seekTo } = renderVideo();
    const region = screen.getByRole('main', { name: 'Videoplayer' });
    region.focus();
    fireEvent.keyDown(region, { key: 'k' });
    expect(togglePlay).toHaveBeenCalledTimes(1);
    fireEvent.keyDown(region, { key: 'ArrowRight' });
    expect(seekBy).toHaveBeenLastCalledWith(10);
    fireEvent.keyDown(region, { key: '5' });
    expect(seekTo).toHaveBeenLastCalledWith(30_000);
    fireEvent.keyDown(region, { key: 'ArrowUp' });
    expect(screen.getByRole('status')).toHaveTextContent('Lautstärke');
    fireEvent.keyDown(region, { key: ']' });
    expect(screen.getByRole('status')).toHaveTextContent('1,25');
  });

  it('shows the chapters of the download as a tab, C on the chapter playing', async () => {
    const { seekTo } = renderVideo({ chapters });
    expect(screen.getByRole('button', { name: 'Vorheriges Kapitel' })).toBeVisible();
    await waitFor(() => expect(screen.getByRole('button', { name: 'Abspielen' })).toHaveFocus());
    fireEvent.keyDown(window, { key: 'c' });
    const tab = await screen.findByRole('list', { name: 'Kapitel' });
    const current = within(tab).getByRole('button', { current: true });
    expect(current).toHaveAccessibleName(/^Mitte /);
    await waitFor(() => expect(current).toHaveFocus());
    fireEvent.click(within(tab).getByRole('button', { name: /Ende/ }));
    expect(seekTo).toHaveBeenLastCalledWith(40_000);
    fireEvent.keyDown(window, { key: 'ArrowRight', shiftKey: true });
    expect(seekTo).toHaveBeenLastCalledWith(40_000);
  });

  it('lists only the keys this player has in the help', () => {
    renderVideo();
    fireEvent.keyDown(window, { key: '?' });
    const help = screen.getByRole('dialog', { name: 'Tastenkürzel' });
    expect(within(help).getByText('Untertitel')).toBeVisible();
    expect(within(help).queryByText('Qualität')).toBeNull();
    expect(within(help).queryByText('Stream-Info')).toBeNull();
    expect(within(help).queryByText('Nächste Folge')).toBeNull();
    expect(within(help).queryByText('Reiter Kapitel')).toBeNull();
  });

  it('at the limit the hint’s back button takes the focus and nothing opens', async () => {
    const { togglePlay } = renderVideo({ limitReached: true, limitMinutes: 1 });
    const alert = screen.getByRole('alert');
    expect(alert).toHaveTextContent('Schauzeit für heute aufgebraucht');
    await waitFor(() =>
      expect(within(alert).getByRole('button', { name: 'Zurück' })).toHaveFocus(),
    );
    expect(screen.getByRole('button', { name: 'Abspielen' })).toBeDisabled();
    act(() => {
      fireEvent.keyDown(window, { key: 'u' });
    });
    expect(screen.queryByRole('dialog', { name: 'Untertitel' })).toBeNull();
    expect(togglePlay).not.toHaveBeenCalled();
  });
});
