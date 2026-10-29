import type { MetadataProvider, MetadataProviderInfo } from '@slouchcat/api-client';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it } from 'vitest';
import { ProviderList, providerListError } from './ProviderList';

/** What listMetadataProviders answers on a server without TVDB key. */
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
    configured: false,
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
  content,
  initial,
}: {
  content: 'movie' | 'series' | 'audiobook';
  initial: MetadataProvider[];
}) {
  const [value, setValue] = useState(initial);
  return (
    <>
      <ProviderList content={content} value={value} onChange={setValue} infos={infos} />
      <output data-testid="value">{value.join(',')}</output>
    </>
  );
}

describe('ProviderList', () => {
  it('offers the providers of the content, marks identifier and fillers, and shows the hints', () => {
    render(<Harness content="series" initial={['tvdb', 'tmdb']} />);
    const boxes = screen.getAllByRole('checkbox');
    expect(boxes.map((box) => box.getAttribute('aria-label'))).toEqual(['TVDB', 'TMDB', 'AniList']);
    expect(boxes[0]).toBeChecked();
    expect(boxes[2]).not.toBeChecked();
    expect(screen.getByText('erkennt Titel')).toBeVisible();
    expect(screen.getByText('füllt auf')).toBeVisible();
    expect(screen.getByText(/kein Schlüssel hinterlegt/)).toBeVisible();
    expect(screen.getByText(/liefert kein Deutsch/)).toBeVisible();
    expect(screen.getByText(/nur Auffüller/)).toBeVisible();
  });

  it('ticks keep the shown order, the buttons and arrow keys swap rows, the focus stays', async () => {
    const user = userEvent.setup();
    render(<Harness content="series" initial={['tmdb', 'tvdb']} />);
    await user.click(screen.getByRole('checkbox', { name: 'AniList' }));
    expect(screen.getByTestId('value')).toHaveTextContent('tmdb,tvdb,anilist');

    const down = screen.getByRole('button', { name: 'TMDB nach unten' });
    await user.click(down);
    expect(screen.getByTestId('value')).toHaveTextContent('tvdb,tmdb,anilist');
    // The pressed button keeps the focus after the rows swapped.
    expect(screen.getByRole('button', { name: 'TMDB nach unten' })).toHaveFocus();
    await user.keyboard('{ArrowDown}');
    expect(screen.getByTestId('value')).toHaveTextContent('tvdb,anilist,tmdb');
    // At the end of the list the down button is disabled; the up button takes the focus.
    expect(screen.getByRole('button', { name: 'TMDB nach oben' })).toHaveFocus();
    await user.keyboard('{ArrowUp}{ArrowUp}');
    expect(screen.getByTestId('value')).toHaveTextContent('tmdb,tvdb,anilist');

    await user.click(screen.getByRole('checkbox', { name: 'TVDB' }));
    expect(screen.getByTestId('value')).toHaveTextContent('tmdb,anilist');
  });

  it('refuses a filler in front and an empty list before the server does', async () => {
    const user = userEvent.setup();
    render(<Harness content="movie" initial={['tmdb', 'anilist']} />);
    await user.click(screen.getByRole('button', { name: 'AniList nach oben' }));
    expect(screen.getByRole('alert')).toHaveTextContent(
      'AniList kann keinen Titel erkennen. Setze TMDB an die erste Stelle.',
    );
    expect(screen.getByText('kann nicht erkennen')).toBeVisible();
    await user.click(screen.getByRole('checkbox', { name: 'AniList' }));
    await user.click(screen.getByRole('checkbox', { name: 'TMDB' }));
    expect(screen.getByRole('alert')).toHaveTextContent('Wähle mindestens eine Quelle.');
    expect(providerListError(((key: string) => key) as never, 'movie', ['tmdb'], infos)).toBeNull();
  });

  it('shows a content with one provider, and a list without server answer, as text', () => {
    const { rerender } = render(<Harness content="audiobook" initial={['local']} />);
    expect(screen.queryByRole('checkbox')).toBeNull();
    expect(screen.getByText('Lokale Dateien')).toBeVisible();
    rerender(<ProviderList content="series" value={['tmdb', 'tvdb']} onChange={() => undefined} />);
    expect(screen.queryByRole('checkbox')).toBeNull();
    expect(screen.getByText('TMDB › TVDB')).toBeVisible();
  });
});
