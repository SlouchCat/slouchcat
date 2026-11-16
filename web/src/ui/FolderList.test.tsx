import type { MetadataProviderInfo } from '@slouchcat/api-client';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { act } from 'react';
import { describe, expect, it } from 'vitest';
import { FolderList, folderListError, newFolderEntry, type FolderEntry } from './FolderList';

const infos: MetadataProviderInfo[] = [
  {
    id: 'tmdb',
    configured: true,
    roles: ['identify', 'fill'],
    contents: ['movie', 'series'],
    languages: ['de', 'en'],
  },
  {
    id: 'tvdb',
    configured: true,
    roles: ['identify', 'fill'],
    contents: ['series'],
    languages: ['de', 'en'],
  },
  {
    id: 'anilist',
    configured: true,
    roles: ['fill'],
    contents: ['movie', 'series'],
    languages: ['en'],
  },
  {
    id: 'local',
    configured: true,
    roles: ['identify'],
    contents: ['audiobook', 'podcast'],
    languages: [],
  },
];

function Harness({
  kind,
  min,
  ordering,
}: {
  kind: 'video' | 'podcasts';
  min: number;
  ordering?: 'arrows' | 'rank';
}) {
  const [entries, setEntries] = useState<FolderEntry[]>(() =>
    min > 0 ? [newFolderEntry(kind, [])] : [],
  );
  return (
    <>
      <FolderList
        kind={kind}
        entries={entries}
        onChange={setEntries}
        infos={infos}
        min={min}
        namePrefix="roots"
        ordering={ordering}
      />
      <output data-testid="value">
        {entries
          .map((entry) => `${entry.path}|${entry.content}|${entry.providers.join(',')}`)
          .join(';')}
      </output>
    </>
  );
}

describe('FolderList', () => {
  it('adds a second card with the other content, focuses its path, and removes it back to the add button', async () => {
    const user = userEvent.setup();
    render(<Harness kind="video" min={1} />);
    const cards = () => screen.getAllByRole('listitem', { name: /^Ordner \d/ });
    expect(cards()).toHaveLength(1);
    // The one card the list keeps has no remove button.
    expect(screen.queryByRole('button', { name: 'Ordner 1 entfernen' })).toBeNull();
    const add = screen.getByRole('button', { name: 'Weiteren Ordner hinzufügen' });
    await user.click(add);
    expect(cards()).toHaveLength(2);
    const second = cards()[1]!;
    expect(within(second).getByLabelText('Ordner')).toHaveFocus();
    // A video library offers series after films (two clicks for «Dokus»).
    expect(within(second).getByRole('radio', { name: 'Serien' })).toBeChecked();
    expect(screen.getByTestId('value')).toHaveTextContent('|movie|tmdb;|series|tmdb,tvdb');

    await user.click(screen.getByRole('button', { name: 'Ordner 2 entfernen' }));
    expect(cards()).toHaveLength(1);
    expect(screen.getByRole('button', { name: 'Weiteren Ordner hinzufügen' })).toHaveFocus();
  });

  it('a content change resets the list of the card; the providers are edited per card', async () => {
    const user = userEvent.setup();
    render(<Harness kind="video" min={1} />);
    await user.click(screen.getByRole('radio', { name: 'Serien' }));
    expect(screen.getByTestId('value')).toHaveTextContent('|series|tmdb,tvdb');
    await user.click(screen.getByRole('checkbox', { name: 'AniList' }));
    expect(screen.getByTestId('value')).toHaveTextContent('|series|tmdb,tvdb,anilist');
    await user.click(screen.getByRole('radio', { name: 'Filme' }));
    expect(screen.getByTestId('value')).toHaveTextContent('|movie|tmdb');
  });

  it('starts empty for podcasts and says so', async () => {
    const user = userEvent.setup();
    render(<Harness kind="podcasts" min={0} />);
    expect(screen.getByText(/Eine Podcast-Bibliothek darf auch ohne Ordner bleiben/)).toBeVisible();
    await user.click(screen.getByRole('button', { name: 'Ordner hinzufügen' }));
    expect(screen.getAllByRole('listitem', { name: /^Ordner \d/ })).toHaveLength(1);
    // One content only: no radio group, the provider stands as text (in the summary and in the
    // fixed list).
    expect(screen.queryByRole('radio')).toBeNull();
    expect(screen.getAllByText('Lokale Dateien').length).toBeGreaterThan(0);
    expect(screen.getByRole('button', { name: 'Ordner 1 entfernen' })).toBeVisible();
  });

  it('names what keeps the list from being sent', () => {
    const t = ((key: string, options?: Record<string, unknown>) =>
      `${key}${options ? JSON.stringify(options) : ''}`) as never;
    const one = { ...newFolderEntry('video', []), path: '/media/a' };
    const inside = { ...newFolderEntry('video', [one]), path: '/media/a/b' };
    const beside = { ...newFolderEntry('video', [one]), path: '/media/ab' };
    const blank = { ...newFolderEntry('video', [one]), path: ' ' };
    expect(folderListError(t, [one, beside], infos)).toBeNull();
    expect(folderListError(t, [one, inside], infos)).toContain('library.roots.overlap');
    expect(folderListError(t, [one, blank], infos)).toBeNull();
    expect(folderListError(t, [{ ...one, providers: ['anilist'] }], infos)).toContain(
      'library.roots.providers',
    );
  });

  it('orders the providers with arrow buttons by default, by position when asked (S-16)', async () => {
    const user = userEvent.setup();
    const { unmount } = render(<Harness kind="video" min={1} />);
    await user.click(screen.getByRole('radio', { name: 'Serien' }));
    expect(screen.getByRole('button', { name: 'TVDB nach oben' })).toBeVisible();
    expect(screen.queryByRole('combobox')).toBeNull();
    unmount();

    render(<Harness kind="video" min={1} ordering="rank" />);
    await user.click(screen.getByRole('radio', { name: 'Serien' }));
    expect(screen.queryByRole('button', { name: /nach (oben|unten)$/ })).toBeNull();
    await user.selectOptions(screen.getByRole('combobox', { name: 'Platz für TVDB' }), 'Platz 1');
    expect(screen.getByTestId('value')).toHaveTextContent('|series|tvdb,tmdb');
  });

  it('a card folds to one line with path, content and sources; Enter opens it (concept 24 4.2)', async () => {
    const user = userEvent.setup();
    render(<Harness kind="video" min={1} />);
    const card = screen.getByRole('listitem', { name: 'Ordner 1' });
    // A new card is open (exception 2); its head names the card while the path is empty.
    const head = within(card).getByRole('button', { name: /Ordner 1/ });
    expect(head).toHaveAttribute('aria-expanded', 'true');
    await user.type(within(card).getByLabelText('Ordner'), '/media/Filme');
    await user.click(head);
    expect(head).toHaveAttribute('aria-expanded', 'false');
    // The fields stay mounted, so nothing typed is lost; they are hidden.
    expect(within(card).getByLabelText('Ordner')).not.toBeVisible();
    expect(within(card).getByRole('button', { name: /\/media\/Filme/ })).toBe(head);
    const line = head.closest('h3')!.parentElement!;
    expect(line).toHaveTextContent('/media/FilmeFilmeTMDB');
    head.focus();
    await user.keyboard('{Enter}');
    expect(head).toHaveAttribute('aria-expanded', 'true');
    expect(head).toHaveFocus();
  });

  it('marks a stored folder instead of removing it; «Rückgängig» brings it back to its head', async () => {
    const user = userEvent.setup();
    const stored = { ...newFolderEntry('video', []), key: 'root-1', path: '/media/Filme' };
    function Marking() {
      const [marked, setMarked] = useState<ReadonlySet<string>>(new Set());
      return (
        <FolderList
          kind="video"
          entries={[stored]}
          onChange={() => undefined}
          infos={infos}
          min={0}
          namePrefix="roots"
          expandedKeys={new Set(['root-1'])}
          marking={{
            isStored: (entry) => entry.key === 'root-1',
            marked,
            onMark: (key) => setMarked(new Set([key])),
            onUndo: () => setMarked(new Set()),
          }}
        />
      );
    }
    render(<Marking />);
    await user.click(screen.getByRole('button', { name: 'Ordner 1 entfernen' }));
    const card = screen.getByRole('listitem', { name: 'Ordner 1' });
    expect(within(card).getByText(/Wird beim Speichern entfernt/)).toBeVisible();
    expect(within(card).queryByLabelText('Ordner')).toBeNull();
    const undo = within(card).getByRole('button', {
      name: 'Entfernen von Ordner 1 rückgängig machen',
    });
    expect(undo).toHaveFocus();
    await user.click(undo);
    await act(async () => {});
    expect(screen.getByRole('button', { name: /\/media\/Filme/ })).toHaveFocus();
    expect(screen.getByLabelText('Ordner')).toHaveValue('/media/Filme');
  });
});
