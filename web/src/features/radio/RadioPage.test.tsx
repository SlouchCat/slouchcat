import type { components } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, signedIn } from '@/test/api';

type RadioStation = components['schemas']['RadioStation'];

const station = (id: number, name: string, extra: Partial<RadioStation> = {}): RadioStation => ({
  id,
  name,
  country: 'CH',
  languages: ['de'],
  genres: ['pop'],
  kinds: ['music'],
  moods: [],
  hls: false,
  showsTitles: true,
  favorite: false,
  recommended: false,
  state: 'ok',
  ...extra,
});

const swissPop = station(1, 'Radio Swiss Pop', { region: 'CH-ZH' });
const kanalK = station(2, 'Kanal K', { favorite: true, region: 'CH-AG' });
const gone = station(3, 'Radio Alt', {
  state: 'missing',
  missingSince: Date.UTC(2026, 10, 1),
  removalAt: Date.UTC(2026, 10, 8),
});

const facets = {
  country: [{ value: 'CH', count: 3 }],
  region: [],
  language: [{ value: 'de', count: 3 }],
  genre: [
    { value: 'pop', count: 2 },
    { value: 'jazz', count: 1 },
  ],
  kind: [{ value: 'music', count: 3 }],
  mood: [{ value: 'chill', count: 1 }],
  quality: [{ value: 'good', count: 3 }],
  titles: 2,
};

const page = (items: RadioStation[]) => ({ items, total: items.length, offset: 0 });

function radioApi(overrides: Parameters<typeof mockApi>[0] = {}) {
  return mockApi({
    ...signedIn,
    'GET /radio/preferences': () => json({ defaultCountry: 'CH' }),
    'PUT /radio/preferences': (call) => json(call.body),
    'GET /radio/facets': () => json(facets),
    'GET /radio/stations': () => json(page([swissPop, kanalK, gone])),
    'GET /radio/favorites': () => json([kanalK]),
    ...overrides,
  });
}

describe('radio page', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('shows the browse rows without a filter and only the grid with one', async () => {
    radioApi();
    const { router } = renderApp('/radio');
    expect(await screen.findByRole('heading', { level: 1, name: 'Radio' })).toBeVisible();
    expect(await screen.findByRole('list', { name: 'Meine Sender' })).toBeVisible();
    expect(await screen.findByRole('heading', { name: 'Beliebt in Schweiz' })).toBeVisible();
    expect(screen.getByRole('heading', { name: 'Nach Genre' })).toBeVisible();
    expect(screen.getByRole('heading', { name: 'Nach Stimmung' })).toBeVisible();
    const grid = await screen.findByRole('list', { name: 'Alle Sender' });
    expect(within(grid).getByRole('button', { name: 'Radio Swiss Pop abspielen' })).toBeVisible();
    // A station that left the catalog says until when it stays.
    expect(within(grid).getByText(/Nicht mehr im Katalog, verschwindet am/)).toBeVisible();

    await userEvent.setup().click(screen.getByRole('button', { name: /^Jazz/ }));
    await waitFor(() => expect(router.state.location.search).toEqual({ genre: 'jazz' }));
    expect(await screen.findByRole('list', { name: 'Treffer' })).toBeVisible();
    expect(screen.queryByRole('heading', { name: 'Nach Genre' })).not.toBeInTheDocument();
    expect(screen.queryByRole('list', { name: 'Meine Sender' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Filter Jazz entfernen' })).toBeVisible();
  });

  it('sends the filter of the URL to the server', async () => {
    const { fetchMock } = radioApi();
    renderApp('/radio?country=CH&genre=pop,jazz&titles=true&sort=name');
    await screen.findByRole('list', { name: 'Treffer' });
    const asked = fetchMock.mock.calls
      .map(([request]) => new URL(request.url))
      .filter((url) => url.pathname.endsWith('/radio/stations'))
      .at(-1);
    expect(asked?.searchParams.getAll('country')).toEqual(['CH']);
    expect(asked?.searchParams.getAll('genre')).toEqual(['pop', 'jazz']);
    expect(asked?.searchParams.get('titles')).toBe('true');
    expect(asked?.searchParams.get('sort')).toBe('name');
    expect(screen.getByRole('button', { name: 'Filter Pop entfernen' })).toBeVisible();
    expect(screen.getByRole('button', { name: 'Filter Jazz entfernen' })).toBeVisible();
    expect(
      screen.getByRole('button', { name: 'Filter Nur mit Titelanzeige entfernen' }),
    ).toBeVisible();
    expect(screen.getByRole('combobox', { name: 'Sortieren' })).toHaveValue('name');
  });

  it('says the catalog is loading while the server answers 503', async () => {
    radioApi({
      'GET /radio/stations': () => problem(503, 'radio_catalog_loading', {}),
      'GET /radio/facets': () => problem(503, 'radio_catalog_loading', {}),
    });
    renderApp('/radio');
    expect(await screen.findByText('Der Senderkatalog wird geladen')).toBeVisible();
    expect(screen.queryByRole('heading', { name: 'Nach Genre' })).not.toBeInTheDocument();
  });

  it('brings back the last filter of the profile when it opens without one', async () => {
    radioApi({
      'GET /radio/preferences': () =>
        json({ defaultCountry: 'CH', lastFilter: { genre: ['jazz'] }, lastSort: 'new' }),
    });
    const { router } = renderApp('/radio');
    await waitFor(() =>
      expect(router.state.location.search).toEqual({ genre: 'jazz', sort: 'new' }),
    );
  });

  it('stores the whole preferences after a change, the stored filter not again', async () => {
    const { calls } = radioApi({
      'GET /radio/preferences': () =>
        json({ defaultCountry: 'CH', lastFilter: { genre: ['jazz'] }, lastSort: 'new' }),
    });
    const { router } = renderApp('/radio');
    await waitFor(() =>
      expect(router.state.location.search).toEqual({ genre: 'jazz', sort: 'new' }),
    );
    await userEvent
      .setup()
      .click(await screen.findByRole('button', { name: 'Filter Jazz entfernen' }));
    await waitFor(
      () =>
        expect(calls.filter((call) => call.method === 'PUT').map((call) => call.body)).toEqual([
          { defaultCountry: 'CH', lastFilter: {}, lastSort: 'new' },
        ]),
      { timeout: 3000 },
    );
  });

  it('marks a favourite with the heart', async () => {
    const { calls } = radioApi({
      'PUT /radio/favorites/1': () => new Response(null, { status: 204 }),
    });
    renderApp('/radio');
    const grid = await screen.findByRole('list', { name: 'Alle Sender' });
    const heart = within(grid).getByRole('button', { name: 'Radio Swiss Pop in Meine Sender' });
    expect(heart).toHaveAttribute('aria-pressed', 'false');
    await userEvent.setup().click(heart);
    await waitFor(() =>
      expect(
        calls.some((call) => call.method === 'PUT' && call.path === '/radio/favorites/1'),
      ).toBe(true),
    );
  });
});
