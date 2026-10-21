import type { SearchResult } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { alien, arrival, heat, ids } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, signedIn, type Call } from '@/test/api';

const collectionId = '01JB8Z6Q4M7N9P2R3S5T7V9C01';

const result = (overrides: Partial<SearchResult> = {}): SearchResult => ({
  items: [heat, alien],
  total: 2,
  offset: 0,
  limit: 50,
  people: [{ id: 3, name: 'Sigourney Weaver' }],
  collections: [{ id: collectionId, name: 'Alien-Reihe', count: 2, cover: alien }],
  ...overrides,
});

const routes = (search: (call: Call) => Response | Promise<Response>) => ({
  ...signedIn,
  'GET /genres': () =>
    json([
      { name: 'Drama', count: 4 },
      { name: 'Horror', count: 2 },
    ]),
  'POST /search': search,
});

const searches = (calls: Call[]) =>
  calls.filter((call) => call.method === 'POST' && call.path === '/search');

describe('search page', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('opens with the field focused and asks nothing for one character', async () => {
    const { calls } = mockApi(routes(() => json(result())));
    renderApp('/search');
    const field = await screen.findByRole('searchbox', { name: 'Suchbegriff' });
    await waitFor(() => expect(field).toHaveFocus());
    expect(screen.getByText(/Suche nach Titeln, Personen und Sammlungen/)).toBeVisible();

    await userEvent.setup().type(field, 'A');
    expect(await screen.findByText('Gib mindestens zwei Zeichen ein.')).toBeVisible();
    await new Promise((resolve) => setTimeout(resolve, 300));
    expect(searches(calls)).toHaveLength(0);
  });

  it('searches from the second character, debounced, and shows three sections', async () => {
    const { calls } = mockApi(routes(() => json(result())));
    const { router } = renderApp('/search');
    const field = await screen.findByRole('searchbox', { name: 'Suchbegriff' });
    await userEvent.setup().type(field, 'Al');

    const titles = await screen.findByRole('region', { name: 'Titel' });
    expect(within(titles).getByRole('link', { name: /Heat/ })).toHaveAttribute(
      'href',
      `/items/${ids.heat}`,
    );
    // One request for the whole word: typing within 200 ms does not ask per key.
    expect(searches(calls)).toHaveLength(1);
    expect(searches(calls)[0]?.body).toEqual({ query: 'Al', offset: 0, limit: 50 });
    expect(router.state.location.search).toEqual({ q: 'Al' });
    expect(screen.getByText('2 Titel gefunden')).toBeVisible();

    const people = screen.getByRole('region', { name: 'Personen' });
    expect(within(people).getByRole('link', { name: 'Sigourney Weaver' })).toHaveAttribute(
      'href',
      '/people/3',
    );
    const collections = screen.getByRole('region', { name: 'Sammlungen' });
    expect(within(collections).getByRole('link', { name: /Alien-Reihe/ })).toHaveAttribute(
      'href',
      `/collections/${collectionId}`,
    );
    expect(within(collections).getByText('2 Titel')).toBeVisible();
  });

  it('browses a genre from a link without text and sends every filter of the URL', async () => {
    const { calls } = mockApi(routes(() => json(result({ people: [], collections: [] }))));
    renderApp('/search?genre=Drama&from=1990&to=2000&rating=12&type=movie,series&avail=local');
    await screen.findByRole('region', { name: 'Titel' });
    expect(searches(calls)[0]?.body).toEqual({
      genre: 'Drama',
      yearFrom: 1990,
      yearTo: 2000,
      maxRating: 12,
      mediaTypes: ['movie', 'series'],
      availability: ['local'],
      offset: 0,
      limit: 50,
    });
    const genre = screen.getByRole('group', { name: 'Genre' });
    expect(within(genre).getByRole('button', { name: 'Drama' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(screen.getByRole('textbox', { name: 'Jahr von' })).toHaveValue('1990');
    expect(
      within(screen.getByRole('group', { name: 'Medientyp' })).getByRole('button', {
        name: 'Serien',
      }),
    ).toHaveAttribute('aria-pressed', 'true');
    expect(screen.queryByRole('region', { name: 'Personen' })).not.toBeInTheDocument();
  });

  it('keeps the filters in the URL and asks again when one changes', async () => {
    const { calls } = mockApi(routes(() => json(result())));
    const { router } = renderApp('/search?q=Al');
    await screen.findByRole('region', { name: 'Titel' });
    const user = userEvent.setup();

    await user.click(
      within(screen.getByRole('group', { name: 'Genre' })).getByRole('button', { name: 'Horror' }),
    );
    await waitFor(() => expect(router.state.location.search).toEqual({ q: 'Al', genre: 'Horror' }));
    await user.click(
      within(screen.getByRole('group', { name: 'Medientyp' })).getByRole('button', {
        name: 'Hörbücher',
      }),
    );
    await user.click(
      within(screen.getByRole('group', { name: 'Verfügbarkeit' })).getByRole('button', {
        name: 'Auf diesem Gerät',
      }),
    );
    await user.click(
      within(screen.getByRole('group', { name: 'Altersfreigabe höchstens' })).getByRole('button', {
        name: 'FSK 6',
      }),
    );
    await user.type(screen.getByRole('textbox', { name: 'Jahr bis' }), '2010');
    await waitFor(() =>
      expect(searches(calls).at(-1)?.body).toEqual({
        query: 'Al',
        genre: 'Horror',
        yearTo: 2010,
        maxRating: 6,
        mediaTypes: ['audiobook'],
        availability: ['download'],
        offset: 0,
        limit: 50,
      }),
    );

    await user.click(screen.getByRole('button', { name: 'Filter zurücksetzen' }));
    await waitFor(() => expect(router.state.location.search).toEqual({ q: 'Al' }));
    // The year field follows the URL.
    expect(screen.getByRole('textbox', { name: 'Jahr bis' })).toHaveValue('');
  });

  it('empties the field when the "Suche" link drops the text from the URL', async () => {
    mockApi(routes(() => json(result())));
    const { router } = renderApp('/search?q=Al');
    await screen.findByRole('region', { name: 'Titel' });
    const field = screen.getByRole('searchbox', { name: 'Suchbegriff' });
    expect(field).toHaveValue('Al');

    await userEvent.setup().click(screen.getByRole('link', { name: 'Suche' }));
    await waitFor(() => expect(field).toHaveValue(''));
    // The debounce must not write the old text back.
    await new Promise((resolve) => setTimeout(resolve, 300));
    expect(router.state.location.search).toEqual({});
    expect(field).toHaveValue('');
    expect(screen.queryByRole('region', { name: 'Titel' })).not.toBeInTheDocument();
  });

  it('keeps typing while the URL follows the field', async () => {
    mockApi(routes(() => json(result())));
    const { router } = renderApp('/search');
    const field = await screen.findByRole('searchbox', { name: 'Suchbegriff' });
    const user = userEvent.setup();
    await user.type(field, 'Al');
    await waitFor(() => expect(router.state.location.search).toEqual({ q: 'Al' }));
    await user.type(field, 'ien');
    await waitFor(() => expect(router.state.location.search).toEqual({ q: 'Alien' }));
    expect(field).toHaveValue('Alien');
  });

  it('Escape in the empty field goes back; with text it stays a key of the field', async () => {
    mockApi(routes(() => json(result())));
    const { router } = renderApp('/search');
    const field = await screen.findByRole('searchbox', { name: 'Suchbegriff' });
    await waitFor(() => expect(field).toHaveFocus());
    const user = userEvent.setup();

    await user.type(field, 'A');
    await user.keyboard('{Backspace}');
    expect(router.state.location.pathname).toBe('/search');
    await user.type(field, 'A{Escape}');
    expect(router.state.location.pathname).toBe('/search');

    await user.clear(field);
    await user.keyboard('{Escape}');
    // Opened directly: back means the start page.
    await waitFor(() => expect(router.state.location.pathname).toBe('/'));
  });

  it('marks the old results busy while a changed search loads', async () => {
    let answer: () => void = () => undefined;
    const { calls } = mockApi(
      routes(async (call) => {
        if ((call.body as { query?: string }).query === 'Ali') {
          await new Promise<void>((resolve) => {
            answer = resolve;
          });
        }
        return json(result());
      }),
    );
    renderApp('/search?q=Al');
    const titles = await screen.findByRole('region', { name: 'Titel' });
    const area = titles.parentElement!;
    expect(area).not.toHaveAttribute('aria-busy');

    await userEvent.setup().type(screen.getByRole('searchbox', { name: 'Suchbegriff' }), 'i');
    await waitFor(() => expect(area).toHaveAttribute('aria-busy', 'true'));
    // The previous titles stay while the new ones load.
    expect(within(titles).getByRole('link', { name: /Heat/ })).toBeVisible();
    expect(searches(calls).at(-1)?.body).toMatchObject({ query: 'Ali' });

    answer();
    await waitFor(() => expect(area).not.toHaveAttribute('aria-busy'));
  });

  it('loads more titles with "Mehr laden"', async () => {
    const many = Array.from({ length: 50 }, (_, index) => ({
      ...heat,
      id: `${ids.heat.slice(0, -2)}${String(index).padStart(2, '0')}`,
      title: `Heat ${index}`,
    }));
    const { calls } = mockApi(
      routes((call) => {
        const offset = (call.body as { offset: number }).offset;
        return json(
          offset === 0
            ? result({ items: many, total: 51 })
            : result({ items: [arrival], total: 51, offset: 50 }),
        );
      }),
    );
    renderApp('/search?q=He');
    const titles = await screen.findByRole('region', { name: 'Titel' });
    expect(within(titles).getAllByRole('link')).toHaveLength(50);
    await userEvent.setup().click(within(titles).getByRole('button', { name: 'Mehr laden' }));
    expect(await within(titles).findByRole('link', { name: /Arrival/ })).toBeVisible();
    expect((searches(calls).at(-1)?.body as { offset: number }).offset).toBe(50);
    expect(within(titles).queryByRole('button', { name: 'Mehr laden' })).not.toBeInTheDocument();
  });

  it('says so when nothing matches, and shows a failure', async () => {
    let fail = false;
    mockApi(
      routes(() =>
        fail
          ? problem(500, 'internal')
          : json(result({ items: [], total: 0, people: [], collections: [] })),
      ),
    );
    renderApp('/search?q=Zz');
    expect(await screen.findByText('Nichts gefunden')).toBeVisible();
    // One message only: the status line does not add "0 Titel gefunden".
    expect(screen.queryByText('0 Titel gefunden')).not.toBeInTheDocument();
    fail = true;
    await userEvent.setup().type(screen.getByRole('searchbox', { name: 'Suchbegriff' }), 'x');
    expect(await screen.findByRole('alert')).toBeVisible();
  });
});
