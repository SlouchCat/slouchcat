import type { Track } from '@slouchcat/api-client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { sessionQuery } from '@/app/session';
import { json, mockApi, session } from '@/test/api';
import { DEFAULT_SUBTITLE_STYLE } from '../useSubtitleStyle';
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
    expect(dts).toHaveAccessibleDescription('Stream wird neu aufgebaut');
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

  describe('«Als Standard für dieses Profil» (concept 13 11.4)', () => {
    afterEach(() => vi.unstubAllGlobals());

    const renderWithSession = (selectedIndex: number, profileDefault = true) => {
      const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
      queryClient.setQueryData(sessionQuery.queryKey, session);
      render(
        <QueryClientProvider client={queryClient}>
          <AudioPanel
            tracks={tracks}
            profileLanguage="de"
            selectedIndex={selectedIndex}
            converts={() => false}
            onSelect={vi.fn()}
            onClose={vi.fn()}
            profileDefault={profileDefault}
          />
        </QueryClientProvider>,
      );
      return queryClient;
    };

    it('puts the language of the playing track first in the profile’s audio languages', async () => {
      const user = userEvent.setup();
      const { calls } = mockApi({
        'GET /me/playback-prefs': () =>
          json({
            audioLanguages: ['de', 'fr'],
            subtitleMode: 'foreign',
            subtitleLanguages: ['de'],
          }),
        'PUT /me/playback-prefs': (call) => json(call.body),
      });
      const queryClient = renderWithSession(1);
      const button = screen.getByRole('button', { name: 'Als Standard für dieses Profil' });
      await user.click(button);
      expect(
        await screen.findByText('Englisch ist jetzt Standard für dieses Profil.'),
      ).toBeVisible();
      // The other preferences go back as the server holds them.
      expect(calls.find((call) => call.method === 'PUT')?.body).toEqual({
        audioLanguages: ['en', 'de', 'fr'],
        subtitleMode: 'foreign',
        subtitleLanguages: ['de'],
      });
      expect(queryClient.getQueryData(sessionQuery.queryKey)?.profile?.audioLanguages).toEqual([
        'en',
        'de',
        'fr',
      ]);
      expect(button).toHaveFocus();
    });

    it('says so when the playing language is the first already, and sends nothing', async () => {
      const user = userEvent.setup();
      const { calls } = mockApi({});
      renderWithSession(3);
      expect(screen.getByRole('status')).toHaveTextContent(
        'Deutsch ist Standard für dieses Profil.',
      );
      await user.click(screen.getByRole('button', { name: 'Als Standard für dieses Profil' }));
      expect(calls).toHaveLength(0);
    });

    it('is not offered without the flag (the offline player)', () => {
      renderWithSession(1, false);
      expect(screen.queryByRole('button', { name: 'Als Standard für dieses Profil' })).toBeNull();
    });
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

  it('leads below the list to the subtitle style, a page with a way back', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <SubtitlePanel
        tracks={tracks}
        profileLanguage="de"
        selectedId={undefined}
        onSelect={vi.fn()}
        onClose={vi.fn()}
        subtitleStyle={{ value: DEFAULT_SUBTITLE_STYLE, onChange }}
      />,
    );
    await user.click(screen.getByRole('button', { name: 'Untertitel-Stil' }));
    const page = screen.getByRole('dialog', { name: 'Untertitel-Stil' });
    expect(within(page).getByRole('listbox', { name: 'Grösse' })).toBeInTheDocument();
    await waitFor(() => expect(within(page).getByRole('option', { name: 'M' })).toHaveFocus());
    await user.click(within(page).getByRole('option', { name: 'Gelb' }));
    expect(onChange).toHaveBeenLastCalledWith({ color: 'yellow' });
    await user.click(within(page).getByRole('button', { name: '0,1 s später' }));
    expect(onChange).toHaveBeenLastCalledWith({ offsetMs: 100 });
    expect(within(page).getByText('0,0 s')).toBeVisible();
    await user.keyboard('{Backspace}');
    expect(screen.getByRole('dialog', { name: 'Untertitel' })).toBeInTheDocument();
  });

  it('reaches every part of the panel and of the style page with ↑/↓ alone (review step 4)', async () => {
    const user = userEvent.setup();
    function Styled() {
      const [style, setStyle] = useState(DEFAULT_SUBTITLE_STYLE);
      return (
        <SubtitlePanel
          tracks={[subtitle(2, 'ger')]}
          profileLanguage="de"
          selectedId={undefined}
          onSelect={vi.fn()}
          onClose={vi.fn()}
          subtitleStyle={{
            value: style,
            onChange: (patch) => setStyle((current) => ({ ...current, ...patch })),
          }}
        />
      );
    }
    render(<Styled />);
    const off = screen.getByRole('option', { name: 'Aus' });
    await waitFor(() => expect(off).toHaveFocus());
    // From the last row of the list on to the button below it, and back up into the list.
    await user.keyboard('{ArrowDown}');
    expect(screen.getByRole('option', { name: 'Deutsch · SRT' })).toHaveFocus();
    await user.keyboard('{ArrowDown}');
    expect(screen.getByRole('button', { name: 'Untertitel-Stil' })).toHaveFocus();
    await user.keyboard('{ArrowUp}');
    expect(off).toHaveFocus();
    await user.keyboard('{ArrowDown}{ArrowDown}{Enter}');

    const page = screen.getByRole('dialog', { name: 'Untertitel-Stil' });
    await waitFor(() => expect(within(page).getByRole('option', { name: 'M' })).toHaveFocus());
    // ↓ walks the four lists, each entered on its chosen row, then the offset group.
    await user.keyboard('{ArrowDown}{ArrowDown}{ArrowDown}');
    expect(within(page).getByRole('option', { name: 'Schatten' })).toHaveFocus();
    await user.keyboard('{ArrowDown}{ArrowDown}{ArrowDown}');
    expect(within(page).getByRole('option', { name: 'Weiss' })).toHaveFocus();
    await user.keyboard('{ArrowDown}{ArrowDown}');
    expect(within(page).getByRole('option', { name: 'Unten' })).toHaveFocus();
    await user.keyboard('{ArrowDown}{ArrowDown}');
    const earlier = within(page).getByRole('button', { name: '0,1 s früher' });
    expect(earlier).toHaveFocus();
    // The offset group is one stop: ←/→ between its buttons.
    await user.keyboard('{ArrowRight}');
    const later = within(page).getByRole('button', { name: '0,1 s später' });
    expect(later).toHaveFocus();
    expect(earlier).toHaveAttribute('tabindex', '-1');
    await user.keyboard('{Enter}');
    expect(within(page).getByText('+0,1 s')).toBeVisible();
    // «Zurücksetzen» keeps the focus after it set the offset back to 0 (it never switches off).
    await user.keyboard('{ArrowRight}');
    const reset = within(page).getByRole('button', { name: 'Zurücksetzen' });
    expect(reset).toHaveFocus();
    await user.keyboard('{Enter}');
    expect(within(page).getByText('0,0 s')).toBeVisible();
    expect(reset).toHaveFocus();
    await user.keyboard('{Enter}');
    expect(reset).toHaveFocus();
  });
});
