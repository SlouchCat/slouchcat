import type { components, ItemRef } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { alien, arrival, audiobook, heat, ids } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, signedIn } from '@/test/api';

type LibraryNav = components['schemas']['LibraryNav'];
type LibraryItemsPage = components['schemas']['LibraryItemsPage'];

const films: LibraryNav = {
  id: 3,
  name: 'Filme',
  kind: 'video',
  contents: ['movie'],
  itemCount: 3,
};
const docs: LibraryNav = {
  id: 4,
  name: 'Dokus',
  kind: 'video',
  contents: ['movie', 'series'],
  itemCount: 3,
};
const books: LibraryNav = {
  id: 5,
  name: 'Hörbücher',
  kind: 'audiobooks',
  contents: ['audiobook'],
  itemCount: 1,
};

const watched: ItemRef = {
  ...arrival,
  progress: {
    positionMs: 6_960_000,
    durationMs: 6_960_000,
    percent: 100,
    completed: true,
    updatedAt: '2026-10-20T19:00:00Z',
  },
};

const page = (items: ItemRef[], overrides: Partial<LibraryItemsPage> = {}): LibraryItemsPage => ({
  items,
  total: items.length,
  offset: 0,
  limit: 50,
  facets: {
    letters: ['A', 'H'],
    genres: [
      { name: 'Drama', count: 2 },
      { name: 'Krimi', count: 1 },
    ],
    ratings: [0, 12, 16],
  },
  ...overrides,
});

/** The query strings of the listLibraryItems requests, in order. */
const itemRequests = (fetchMock: ReturnType<typeof mockApi>['fetchMock']) =>
  fetchMock.mock.calls
    .map(([request]) => new URL(request.url))
    .filter((url) => /\/libraries\/\d+\/items$/.test(url.pathname))
    .map((url) => url.searchParams);

/** The API with the visible libraries; `reply` answers listLibraryItems by its query. */
function mockLibrary(
  reply: (query: URLSearchParams) => Response = () => json(page([alien, watched, heat])),
) {
  const recorded: { fetchMock?: ReturnType<typeof mockApi>['fetchMock'] } = {};
  const api = mockApi({
    ...signedIn,
    'GET /libraries/visible': () => json([docs, films, books]),
    // The call being answered is the last one recorded (a GET is answered synchronously).
    'GET /libraries/*/items': () => {
      const request = recorded.fetchMock?.mock.calls.at(-1)?.[0];
      return reply(new URL(request?.url ?? 'http://test/').searchParams);
    },
  });
  recorded.fetchMock = api.fetchMock;
  return api;
}

describe('library page', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('shows the name, the count, the grid with watch state and rating, and the letter bar', async () => {
    const { fetchMock } = mockLibrary();
    renderApp('/libraries/3');
    expect(await screen.findByRole('heading', { level: 1, name: 'Filme' })).toBeVisible();
    expect(screen.getByText('3 Titel')).toBeVisible();
    const grid = await screen.findByRole('list', { name: 'Filme' });
    expect(within(grid).getByRole('link', { name: /Heat/ })).toHaveAttribute(
      'href',
      `/items/${ids.heat}`,
    );
    // Watched: a check mark; started: the progress; the age rating as a badge.
    const arrivalCard = within(grid).getByRole('link', { name: /Arrival/ });
    expect(within(arrivalCard).getByRole('img', { name: 'Gesehen' })).toBeInTheDocument();
    const alienCard = within(grid).getByRole('link', { name: /Alien/ });
    expect(within(alienCard).getByRole('progressbar', { name: '36 % gesehen' })).toBeVisible();
    expect(within(alienCard).getByText('FSK 16')).toBeVisible();

    // One content kind: no chips.
    expect(screen.queryByRole('group', { name: 'Inhalt' })).not.toBeInTheDocument();
    const sort = screen.getByRole('combobox', { name: 'Sortieren' });
    expect(sort).toHaveValue('title');
    expect(within(sort).getByRole('option', { name: 'Altersfreigabe' })).toBeInTheDocument();
    expect(within(sort).queryByRole('option', { name: 'Autor' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /aufsteigend/ })).toBeVisible();

    const letters = screen.getByRole('navigation', { name: 'Anfangsbuchstabe' });
    expect(within(letters).getByRole('button', { name: 'Alle Buchstaben' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(within(letters).getByRole('button', { name: 'H' })).toBeEnabled();
    expect(within(letters).getByRole('button', { name: 'B' })).toBeDisabled();
    expect(within(letters).getByRole('button', { name: 'Ziffer oder Zeichen' })).toBeDisabled();
    expect(document.title).toContain('Filme');

    const [first] = itemRequests(fetchMock);
    expect(first?.get('offset')).toBe('0');
    expect(first?.get('limit')).toBe('50');
    expect(first?.has('sort')).toBe(false);
  });

  it('a letter filters by the URL and the count names it', async () => {
    const { fetchMock } = mockLibrary((query) =>
      json(query.get('letter') === 'H' ? page([heat]) : page([alien, heat])),
    );
    const { router } = renderApp('/libraries/3');
    const letters = await screen.findByRole('navigation', { name: 'Anfangsbuchstabe' });
    await userEvent.setup().click(within(letters).getByRole('button', { name: 'H' }));
    await waitFor(() => expect(router.state.location.search).toEqual({ letter: 'H' }));
    expect(await screen.findByText('1 Titel mit H')).toBeVisible();
    expect(within(letters).getByRole('button', { name: 'H' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(itemRequests(fetchMock).at(-1)?.get('letter')).toBe('H');
  });

  it('a new sort order goes into the URL, starts in its own direction and hides the letters', async () => {
    const { fetchMock } = mockLibrary();
    const { router } = renderApp('/libraries/3?letter=H');
    const sort = await screen.findByRole('combobox', { name: 'Sortieren' });
    const user = userEvent.setup();
    await user.selectOptions(sort, 'added');
    await waitFor(() => expect(router.state.location.search).toEqual({ sort: 'added' }));
    expect(screen.getByRole('button', { name: /absteigend/ })).toBeVisible();
    expect(screen.queryByRole('navigation', { name: 'Anfangsbuchstabe' })).not.toBeInTheDocument();
    expect(itemRequests(fetchMock).at(-1)?.get('sort')).toBe('added');

    // The direction button flips the order; back to the default it leaves the URL.
    await user.click(screen.getByRole('button', { name: /absteigend/ }));
    await waitFor(() =>
      expect(router.state.location.search).toEqual({ sort: 'added', order: 'asc' }),
    );
    expect(screen.getByRole('button', { name: /aufsteigend/ })).toBeVisible();
    await user.click(screen.getByRole('button', { name: /aufsteigend/ }));
    await waitFor(() => expect(router.state.location.search).toEqual({ sort: 'added' }));
  });

  it('the filter panel offers the facets, counts the filters and closes with Escape', async () => {
    const { fetchMock } = mockLibrary();
    const { router } = renderApp('/libraries/3');
    const filter = await screen.findByRole('button', { name: 'Filter' });
    expect(filter).toHaveAttribute('aria-expanded', 'false');
    const user = userEvent.setup();
    await user.click(filter);
    expect(filter).toHaveAttribute('aria-expanded', 'true');
    const panel = screen.getByRole('region', { name: 'Filter' });

    await user.click(within(panel).getByRole('button', { name: 'Drama' }));
    await user.click(within(panel).getByRole('button', { name: 'Ungesehen' }));
    await user.click(within(panel).getByRole('button', { name: 'bis 12' }));
    await waitFor(() =>
      expect(router.state.location.search).toEqual({
        genre: 'Drama',
        status: 'unwatched',
        rating: 12,
      }),
    );
    expect(screen.getByRole('button', { name: 'Filter (3)' })).toBeVisible();
    expect(await screen.findByText('3 Titel · gefiltert')).toBeVisible();
    const last = itemRequests(fetchMock).at(-1);
    expect(last?.get('genre')).toBe('Drama');
    expect(last?.getAll('status')).toEqual(['unwatched']);
    expect(last?.get('maxRating')).toBe('12');
    expect(within(panel).getByRole('button', { name: 'Profilgrenze' })).toBeVisible();

    await user.click(within(panel).getByRole('button', { name: 'Krimi' }));
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('region', { name: 'Filter' })).not.toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole('button', { name: 'Filter (3)' })).toHaveFocus());
    // Escape closed the panel; the page stayed.
    expect(router.state.location.pathname).toBe('/libraries/3');

    // "Zurücksetzen" lifts every filter.
    await user.click(screen.getByRole('button', { name: 'Filter (3)' }));
    await user.click(screen.getByRole('button', { name: 'Zurücksetzen' }));
    await waitFor(() => expect(router.state.location.search).toEqual({}));
    await user.click(screen.getByRole('button', { name: 'Anwenden' }));
    expect(screen.queryByRole('region', { name: 'Filter' })).not.toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole('button', { name: 'Filter' })).toHaveFocus());
  });

  it('a group without facet values is not offered', async () => {
    mockLibrary(() => json(page([heat], { facets: { letters: ['H'], genres: [], ratings: [] } })));
    renderApp('/libraries/3');
    await userEvent.setup().click(await screen.findByRole('button', { name: 'Filter' }));
    const panel = screen.getByRole('region', { name: 'Filter' });
    expect(within(panel).queryByRole('group', { name: 'Genre' })).not.toBeInTheDocument();
    expect(within(panel).queryByRole('group', { name: 'Altersfreigabe' })).not.toBeInTheDocument();
    expect(within(panel).getByRole('group', { name: 'Status' })).toBeVisible();
    expect(within(panel).getByRole('textbox', { name: 'Jahr von' })).toBeVisible();
  });

  it('a mixed library offers the content chips', async () => {
    const { fetchMock } = mockLibrary();
    const { router } = renderApp('/libraries/4');
    const chips = await screen.findByRole('group', { name: 'Inhalt' });
    expect(within(chips).getByRole('button', { name: 'Alle' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    await userEvent.setup().click(within(chips).getByRole('button', { name: 'Serien' }));
    await waitFor(() => expect(router.state.location.search).toEqual({ content: 'series' }));
    expect(itemRequests(fetchMock).at(-1)?.get('content')).toBe('series');
  });

  it('an audiobook library: author order, listening states, no age rating', async () => {
    mockLibrary(() =>
      json(page([audiobook], { facets: { letters: [], genres: [], ratings: [] } })),
    );
    renderApp('/libraries/5');
    expect(await screen.findByText('1 Hörbuch')).toBeVisible();
    const sort = screen.getByRole('combobox', { name: 'Sortieren' });
    expect(within(sort).getByRole('option', { name: 'Autor' })).toBeInTheDocument();
    expect(within(sort).queryByRole('option', { name: 'Altersfreigabe' })).not.toBeInTheDocument();
    await userEvent.setup().click(screen.getByRole('button', { name: 'Filter' }));
    const status = screen.getByRole('group', { name: 'Status' });
    expect(within(status).getByRole('button', { name: 'Ungehört' })).toBeVisible();
    expect(within(status).getByRole('button', { name: 'Gehört' })).toBeVisible();
  });

  it('nothing matching the filters says so and offers to reset them', async () => {
    mockLibrary(() => json(page([])));
    const { router } = renderApp('/libraries/3?genre=Krimi');
    expect(await screen.findByText('Nichts passt zu diesen Filtern')).toBeVisible();
    await userEvent.setup().click(screen.getByRole('button', { name: 'Zurücksetzen' }));
    await waitFor(() => expect(router.state.location.search).toEqual({}));
  });

  it('a library the profile does not see is not found', async () => {
    mockApi({
      ...signedIn,
      'GET /libraries/visible': () => json([films]),
      'GET /libraries/*/items': () => problem(404, 'not_found'),
    });
    renderApp('/libraries/9');
    expect(await screen.findByText('Bibliothek nicht gefunden')).toBeVisible();
    expect(screen.getByRole('link', { name: 'Zur Startseite' })).toHaveAttribute('href', '/');
  });
});
