import type { components } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, session } from '@/test/api';

type DiscoverEntry = components['schemas']['DiscoverEntry'];
type DiscoverRow = components['schemas']['DiscoverRow'];
type ItemRef = components['schemas']['ItemRef'];

const rights = { enabled: true, canRequest: true, movies: true, shows: true, discover: true };
const withRights = (wishes: object = rights) => ({
  'GET /auth/session': () => json({ ...session, wishes }),
});
const noWishes = { open: 0, pending: 0, processing: 0, partial: 0, available: 0, closed: 0 };
const wishList = (counts = noWishes) => ({
  'GET /wishes': () => json({ items: [], total: 0, counts }),
  'GET /wishes/quota': () => json({ unlimited: true }),
});

const poster = { url: '/api/v1/titles/movie/9001/images/poster?w=400', width: 400, height: 600 };
const external = (
  media: 'movie' | 'show',
  tmdbId: number,
  title: string,
  extra: Partial<DiscoverEntry['title']> = {},
): DiscoverEntry['title'] => ({
  media,
  tmdbId,
  title,
  year: 2026,
  images: {},
  hasTrailer: false,
  canRequest: true,
  ...extra,
});

const library: ItemRef = {
  id: 'item-dune',
  kind: 'movie',
  title: 'Dune: Part Two',
  year: 2024,
  images: {},
} as ItemRef;

const entries: DiscoverEntry[] = [
  { title: external('movie', 9001, 'Glasvogel', { images: { poster } }), seerrRequested: false },
  {
    title: external('movie', 693134, 'Dune: Part Two', { year: 2024 }),
    item: library,
    seerrRequested: true,
  },
  {
    title: external('movie', 9002, 'Kupfermond', {
      wish: { id: 4, state: 'pending', role: 'owner' } as DiscoverEntry['title']['wish'],
    }),
    seerrRequested: true,
  },
  { title: external('movie', 9005, 'Rotes Ufer'), seerrRequested: true },
];

const row = (key: DiscoverRow['key'], extra: Partial<DiscoverRow> = {}): DiscoverRow => ({
  key,
  entries: [],
  genres: [],
  stale: false,
  ...extra,
});

const rows = (stale = false): DiscoverRow[] => [
  row('trending', { entries, stale }),
  row('popular_movies', { entries: entries.slice(0, 1) }),
  row('movie_genres', {
    genres: [
      { id: 18, name: 'Drama', backdrop: { url: '/api/v1/wishes/genres/movie/18/backdrop' } },
      { id: 99, name: 'Dokumentarfilm' },
    ] as DiscoverRow['genres'],
  }),
  row('upcoming_movies'),
  row('popular_shows', {
    entries: [{ title: external('show', 9100, 'Nebelhafen'), seerrRequested: false }],
  }),
  row('show_genres'),
  row('upcoming_shows'),
];

describe('«Entdecken» (concept 25, 5.4, M-31)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('shows the rows with their marks: library, missing with plus, own wish, requested in Seerr', async () => {
    mockApi({
      ...withRights(),
      ...wishList(),
      'GET /wishes/discover': () => json({ rows: rows() }),
    });
    renderApp('/discover');
    expect(await screen.findByRole('heading', { level: 1, name: 'Entdecken' })).toBeVisible();
    const area = screen.getByRole('navigation', { name: 'Bereiche unter Wünsche' });
    expect(within(area).getByRole('link', { name: 'Entdecken' })).toHaveAttribute(
      'aria-current',
      'page',
    );
    expect(within(area).getByRole('link', { name: 'Meine Wünsche' })).toHaveAttribute(
      'href',
      '/wishes',
    );

    const trending = await screen.findByRole('region', { name: 'Im Trend' });
    // A library title links to its page with the mark «In der Bibliothek».
    const dune = within(trending).getByRole('link', { name: /Dune: Part Two/ });
    expect(dune).toHaveAttribute('href', '/items/item-dune');
    expect(within(trending).getByText('In der Bibliothek')).toBeVisible();
    // A missing title links to the preview page and carries the plus.
    expect(within(trending).getByRole('link', { name: /Glasvogel/ })).toHaveAttribute(
      'href',
      '/titles/movie/9001',
    );
    expect(within(trending).getByRole('button', { name: 'Glasvogel anfordern' })).toBeVisible();
    // The own wish: its state on the poster, no plus.
    expect(within(trending).getByText('Freigabe ausstehend')).toBeVisible();
    expect(within(trending).queryByRole('button', { name: 'Kupfermond anfordern' })).toBeNull();
    // Requested by somebody in Seerr: the bell, «Bereits angefordert».
    expect(
      within(trending).getByRole('button', { name: 'Rotes Ufer: Benachrichtige mich' }),
    ).toBeVisible();
    expect(within(trending).getByText('2026 · Bereits angefordert')).toBeVisible();

    // The row title of popular films leads to «Filme»; the genre tile to the grid with the genre.
    expect(screen.getByRole('link', { name: /^Beliebte Filme/ })).toHaveAttribute(
      'href',
      '/discover/movies',
    );
    expect(screen.getByRole('link', { name: 'Drama' })).toHaveAttribute(
      'href',
      '/discover/movies?genre=18',
    );
    // Empty rows are left out; no stale band.
    expect(screen.queryByRole('region', { name: 'Demnächst im Kino' })).toBeNull();
    expect(screen.queryByText(/Stand vom letzten Abruf/)).toBeNull();
    expect(document.body.textContent).not.toContain('{{');
  });

  it('says when a row is the last state, and leads the search field to «Suche»', async () => {
    mockApi({
      ...withRights(),
      ...wishList(),
      'GET /wishes/discover': () => json({ rows: rows(true) }),
    });
    const { router } = renderApp('/discover');
    expect(
      await screen.findByText('Seerr ist nicht erreichbar. Du siehst den Stand vom letzten Abruf.'),
    ).toBeVisible();
    const user = userEvent.setup();
    await user.type(
      screen.getByRole('searchbox', { name: 'Film oder Serie suchen' }),
      'Dune{Enter}',
    );
    await waitFor(() => expect(router.state.location.pathname).toBe('/discover/search'));
    expect(router.state.location.search).toEqual({ q: 'Dune' });
  });

  it('without the pages: a profile with wishes is led to «Meine Wünsche», another reads not found', async () => {
    mockApi({
      ...withRights({ ...rights, canRequest: false, discover: false }),
      ...wishList({ ...noWishes, closed: 2 }),
      'GET /wishes/discover': () => problem(404, 'not_found'),
    });
    renderApp('/discover');
    expect(await screen.findByText('Entdecken gibt es für dieses Profil nicht')).toBeVisible();
    expect(screen.getByRole('link', { name: 'Zu «Meine Wünsche»' })).toHaveAttribute(
      'href',
      '/wishes',
    );
    // With «Meine Wünsche» alone there is no row of links.
    expect(screen.queryByRole('navigation', { name: 'Bereiche unter Wünsche' })).toBeNull();
  });

  it('without the pages and without wishes: not found', async () => {
    mockApi({
      ...withRights({ ...rights, canRequest: false, discover: false }),
      ...wishList(),
      'GET /wishes/discover': () => problem(404, 'not_found'),
    });
    renderApp('/discover');
    expect(await screen.findByText('Seite nicht gefunden')).toBeVisible();
    expect(screen.queryByText('Entdecken gibt es für dieses Profil nicht')).toBeNull();
    await waitFor(() => expect(document.title).toBe('Seite nicht gefunden · Slouchcat'));
  });

  it('shows a title Seerr repeats within a row once', async () => {
    mockApi({
      ...withRights(),
      ...wishList(),
      'GET /wishes/discover': () =>
        json({ rows: [row('trending', { entries: [entries[0]!, entries[0]!, entries[3]!] })] }),
    });
    renderApp('/discover');
    const trend = await screen.findByRole('region', { name: 'Im Trend' });
    expect(within(trend).getAllByRole('link', { name: /Glasvogel/ })).toHaveLength(1);
    expect(within(trend).getAllByRole('link', { name: /Rotes Ufer/ })).toHaveLength(1);
  });

  it('Seerr out of reach with nothing kept: the error with a retry', async () => {
    let down = true;
    mockApi({
      ...withRights(),
      ...wishList(),
      'GET /wishes/discover': () =>
        down ? problem(503, 'seerr_unreachable') : json({ rows: rows() }),
    });
    renderApp('/discover');
    expect(await screen.findByText('Seerr ist gerade nicht erreichbar.')).toBeVisible();
    // Another problem than 404 keeps the page's tab title (m6).
    expect(document.title).toBe('Entdecken · Slouchcat');
    down = false;
    await userEvent.setup().click(screen.getByRole('button', { name: 'Erneut versuchen' }));
    expect(await screen.findByRole('region', { name: 'Im Trend' })).toBeVisible();
  });
});
