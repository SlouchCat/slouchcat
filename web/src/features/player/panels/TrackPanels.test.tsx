import type { Track } from '@slouchcat/api-client';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { AudioPanel, SubtitlePanel } from './TrackPanels';

const audio = (index: number, language: string, codec: string, extra: Partial<Track> = {}) =>
  ({
    id: 100 + index,
    index,
    kind: 'audio',
    codec,
    language,
    default: false,
    forced: false,
    source: 'embedded',
    text: false,
    channels: 6,
    ...extra,
  }) satisfies Track;

const subtitle = (id: number, language: string, extra: Partial<Track> = {}) =>
  ({
    id,
    index: id,
    kind: 'subtitle',
    codec: 'subrip',
    language,
    default: false,
    forced: false,
    source: 'embedded',
    text: true,
    ...extra,
  }) satisfies Track;

const rows = (list: HTMLElement) =>
  within(list)
    .getAllByRole('option')
    .map((row) => row.getAttribute('aria-label') ?? row.textContent);
const names = (list: HTMLElement) =>
  within(list)
    .getAllByRole('option')
    .map((row) => {
      const id = row.getAttribute('aria-labelledby');
      return id ? document.getElementById(id)?.textContent : row.textContent;
    });

describe('AudioPanel (V-5)', () => {
  const tracks = [
    audio(1, 'eng', 'eac3', { default: true }),
    audio(2, 'fre', 'ac3'),
    audio(3, 'ger', 'dts'),
    audio(4, 'ita', 'aac'),
  ];

  it('shows the profile language first, then English, the rest folded behind «Weitere Sprachen»', async () => {
    const user = userEvent.setup();
    render(
      <AudioPanel
        tracks={tracks}
        profileLanguage="de"
        selectedIndex={1}
        converts={(track) => track.codec === 'dts'}
        onSelect={vi.fn()}
        onClose={vi.fn()}
      />,
    );
    const list = screen.getByRole('listbox', { name: 'Ton' });
    expect(names(list)).toEqual([
      'Deutsch · DTS · 5.1',
      'Englisch · E-AC3 · 5.1',
      'Weitere Sprachen (2)',
    ]);
    expect(rows(list)).toHaveLength(3);
    await user.click(screen.getByRole('option', { name: 'Weitere Sprachen (2)' }));
    expect(names(list)).toEqual([
      'Deutsch · DTS · 5.1',
      'Englisch · E-AC3 · 5.1',
      'Weitere Sprachen (2)',
      'Französisch · AC3 · 5.1',
      'Italienisch · AAC · 5.1',
    ]);
  });

  it('marks the default and a track that is converted; a pick reports the stream index', async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(
      <AudioPanel
        tracks={tracks}
        profileLanguage="de"
        selectedIndex={1}
        converts={(track) => track.codec === 'dts'}
        onSelect={onSelect}
        onClose={vi.fn()}
      />,
    );
    expect(
      screen.getByRole('option', { name: 'Englisch · E-AC3 · 5.1' }),
    ).toHaveAccessibleDescription('Standard');
    const dts = screen.getByRole('option', { name: 'Deutsch · DTS · 5.1' });
    expect(dts).toHaveAccessibleDescription('wird umgewandelt');
    // The panel opens on the track the server delivers.
    await waitFor(() =>
      expect(screen.getByRole('option', { name: 'Englisch · E-AC3 · 5.1' })).toHaveFocus(),
    );
    await user.click(dts);
    expect(onSelect).toHaveBeenCalledWith(3);
  });

  it('opens unfolded when the chosen track is one of the other languages', () => {
    render(
      <AudioPanel
        tracks={tracks}
        profileLanguage="de"
        selectedIndex={4}
        converts={() => false}
        onSelect={vi.fn()}
        onClose={vi.fn()}
      />,
    );
    expect(screen.getByRole('option', { name: 'Italienisch · AAC · 5.1' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
  });
});

describe('SubtitlePanel (V-5)', () => {
  const tracks = [
    subtitle(1, 'eng', { title: 'English SDH' }),
    subtitle(2, 'ger', { forced: true }),
    subtitle(3, 'ger', { codec: 'hdmv_pgs_subtitle', text: false }),
    subtitle(4, 'jpn'),
  ];

  it('offers «Aus» first; marks forced, SDH and a picture subtitle that is burnt in', async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(
      <SubtitlePanel
        tracks={tracks}
        profileLanguage="de"
        selectedId={undefined}
        onSelect={onSelect}
        onClose={vi.fn()}
      />,
    );
    const list = screen.getByRole('listbox', { name: 'Untertitel' });
    expect(names(list)).toEqual([
      'Aus',
      'Deutsch · SRT',
      'Deutsch · PGS',
      'English SDH · SRT',
      'Weitere Sprachen (1)',
    ]);
    expect(screen.getByRole('option', { name: 'Aus' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('option', { name: 'Deutsch · SRT' })).toHaveAccessibleDescription(
      'erzwungen',
    );
    expect(screen.getByRole('option', { name: 'Deutsch · PGS' })).toHaveAccessibleDescription(
      'Bild-Untertitel, wird eingebrannt',
    );
    expect(screen.getByRole('option', { name: 'English SDH · SRT' })).toHaveAccessibleDescription(
      'SDH',
    );
    await user.click(screen.getByRole('option', { name: 'Deutsch · PGS' }));
    expect(onSelect).toHaveBeenLastCalledWith(3);
  });

  it('reports null for «Aus»', async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(
      <SubtitlePanel
        tracks={tracks}
        profileLanguage="de"
        selectedId={2}
        onSelect={onSelect}
        onClose={vi.fn()}
      />,
    );
    await user.click(screen.getByRole('option', { name: 'Aus' }));
    expect(onSelect).toHaveBeenCalledWith(null);
  });
});
