import type { MetadataProviderInfo } from '@slouchcat/api-client';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
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

function Harness({ kind, min }: { kind: 'video' | 'podcasts'; min: number }) {
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
});
