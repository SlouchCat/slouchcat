import type { components } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, session, signedIn } from '@/test/api';

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
  offerEmpty: false,
};

/** The server's answer to a PUT: the stored country is the effective one, as in the offer. */
const withEffective = (body: unknown) => {
  const prefs = body as { defaultCountry: string };
  return { ...prefs, effectiveCountry: prefs.defaultCountry };
};

const page = (items: RadioStation[]) => ({ items, total: items.length, offset: 0 });

function radioApi(overrides: Parameters<typeof mockApi>[0] = {}) {
  return mockApi({
    ...signedIn,
    // The stored country has stations in the offer: it is the effective one (contract 1.28.0).
    'GET /radio/preferences': () => json({ defaultCountry: 'CH', effectiveCountry: 'CH' }),
    'PUT /radio/preferences': (call) => json(withEffective(call.body)),
    'GET /radio/facets': () => json(facets),
    'GET /radio/stations': () => json(page([swissPop, kanalK, gone])),
    'GET /radio/favorites': () => json([kanalK]),
    ...overrides,
  });
}

/** jsdom has no media playback; the radio calls play() in the click. */
const stubPlayback = () =>
  vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined);

describe('radio page', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

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

  it('opens without a country when the stored one is outside the offer (PB-9)', async () => {
    const { calls, fetchMock } = radioApi({
      // CH is stored, but the offer has no station of it: no effective country.
      'GET /radio/preferences': () => json({ defaultCountry: 'CH', lastFilter: {} }),
      'GET /radio/facets': () => json({ ...facets, country: [{ value: 'DE', count: 3 }] }),
    });
    const { router } = renderApp('/radio');
    // «Beliebt» spans the whole offer; no row and no quick filter of the stored country.
    expect(await screen.findByRole('heading', { name: 'Beliebt' })).toBeVisible();
    expect(screen.queryByRole('heading', { name: 'Beliebt in Schweiz' })).not.toBeInTheDocument();
    expect(screen.queryByRole('checkbox', { name: 'Mein Land: Schweiz' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Mein Land: Schweiz' })).not.toBeInTheDocument();
    expect(router.state.location.search).toEqual({});
    const asked = fetchMock.mock.calls
      .map(([request]) => new URL(request.url))
      .filter((url) => url.pathname.endsWith('/radio/stations'));
    expect(asked.length).toBeGreaterThan(0);
    expect(asked.every((url) => url.searchParams.getAll('country').length === 0)).toBe(true);
    // The stored choice stays: «Mein Land» still shows it, and nothing writes it over.
    expect(await screen.findByRole('combobox', { name: 'Mein Land' })).toHaveValue('CH');
    expect(
      calls
        .filter((call) => call.method === 'PUT')
        .every((call) => (call.body as { defaultCountry: string }).defaultCountry === 'CH'),
    ).toBe(true);
  });

  it('says no station is chosen yet while the offer is empty, with the way there for an admin', async () => {
    radioApi({
      'GET /radio/preferences': () => json({ defaultCountry: 'CH' }),
      'GET /radio/facets': () =>
        json({
          ...facets,
          country: [],
          language: [],
          genre: [],
          kind: [],
          mood: [],
          quality: [],
          titles: 0,
          offerEmpty: true,
        }),
      'GET /radio/stations': () => json(page([])),
      'GET /radio/favorites': () => json([]),
    });
    renderApp('/radio');
    expect(await screen.findByText('Noch keine Sender ausgewählt')).toBeVisible();
    expect(screen.queryByText('Keine Sender gefunden')).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Sender auswählen' })).toHaveAttribute(
      'href',
      '/admin/radio?tab=angebot',
    );
  });

  it('shows no way to the administration to a viewer while the offer is empty', async () => {
    radioApi({
      'GET /auth/session': () =>
        json({ ...session, account: { ...session.account, role: 'user', isOwner: false } }),
      'GET /radio/facets': () => json({ ...facets, offerEmpty: true }),
      'GET /radio/stations': () => json(page([])),
      'GET /radio/favorites': () => json([]),
    });
    renderApp('/radio');
    expect(await screen.findByText('Noch keine Sender ausgewählt')).toBeVisible();
    expect(
      screen.getByText('Die Verwaltung hat noch keine Sender für das Radio ausgewählt.'),
    ).toBeVisible();
    expect(screen.queryByRole('link', { name: 'Sender auswählen' })).not.toBeInTheDocument();
  });

  it('names the provider after the place of a station', async () => {
    radioApi({
      'GET /radio/stations': () =>
        json(page([station(5, 'Radio SRF 3', { region: 'CH-ZH', provider: 'srg-ssr.ch' })])),
    });
    renderApp('/radio');
    const grid = await screen.findByRole('list', { name: 'Alle Sender' });
    expect(within(grid).getByText('Schweiz · Zürich · srg-ssr.ch')).toBeVisible();
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

  it('keeps a new default country while its PUT is slow and the filter is saved', async () => {
    const { calls } = radioApi({
      'GET /radio/facets': () =>
        json({
          ...facets,
          country: [
            { value: 'CH', count: 3 },
            { value: 'DE', count: 1 },
          ],
        }),
      // The country's own PUT answers after the filter's save is due (1 s after the change).
      'PUT /radio/preferences': (call) => {
        const body = call.body as { defaultCountry: string; lastFilter: object };
        const slow = body.defaultCountry === 'DE' && Object.keys(body.lastFilter).length === 0;
        return slow
          ? new Promise<Response>((resolve) =>
              setTimeout(() => resolve(json(withEffective(body))), 1500),
            )
          : json(withEffective(body));
      },
    });
    // «Mein Land» is on: choosing another country moves the filter along.
    const { router } = renderApp('/radio?country=CH');
    const select = await screen.findByRole('combobox', { name: 'Mein Land' });
    await userEvent.setup().selectOptions(select, 'DE');
    await waitFor(() => expect(router.state.location.search).toEqual({ country: 'DE' }));
    const puts = () =>
      calls
        .filter((call) => call.method === 'PUT' && call.path === '/radio/preferences')
        .map((call) => call.body as { defaultCountry: string; lastFilter: object });
    await waitFor(
      () =>
        expect(puts()).toContainEqual({
          defaultCountry: 'DE',
          lastFilter: { country: ['DE'] },
          lastSort: 'popular',
        }),
      { timeout: 4000 },
    );
    // From the choice on, no save sends the old country back.
    const chosen = puts().findIndex((body) => body.defaultCountry === 'DE');
    expect(
      puts()
        .slice(chosen)
        .map((body) => body.defaultCountry),
    ).not.toContain('CH');
  });

  it('keeps the heart of the playing station right after quick presses on two stations', async () => {
    stubPlayback();
    let answerFirst: (response: Response) => void = () => {};
    radioApi({
      // The station stays «starting»: the mini player shows it with its heart.
      'POST /radio/stations/1/listen': () => new Promise<Response>(() => {}),
      'PUT /radio/favorites/1': () =>
        new Promise<Response>((resolve) => {
          answerFirst = resolve;
        }),
      'DELETE /radio/favorites/2': () => new Response(null, { status: 204 }),
    });
    renderApp('/radio');
    const grid = await screen.findByRole('list', { name: 'Alle Sender' });
    const user = userEvent.setup();
    await user.click(within(grid).getByRole('button', { name: 'Radio Swiss Pop abspielen' }));
    const miniHeart = await screen.findByTestId('radio-mini-favorite');
    expect(miniHeart).toHaveAttribute('aria-pressed', 'false');

    // On the page, the heart of the playing station, then at once the one of another station;
    // the first PUT answers last.
    await user.click(within(grid).getByRole('button', { name: 'Radio Swiss Pop in Meine Sender' }));
    await user.click(within(grid).getByRole('button', { name: 'Kanal K in Meine Sender' }));
    answerFirst(new Response(null, { status: 204 }));
    await waitFor(() =>
      expect(screen.getByTestId('radio-mini-favorite')).toHaveAttribute('aria-pressed', 'true'),
    );
  });

  it('says limit and minutes of today when the watch limit refuses the station', async () => {
    stubPlayback();
    radioApi({
      'POST /radio/stations/1/listen': () =>
        problem(403, 'watch_limit_reached', { extensions: { limitMinutes: 60, usedMinutes: 61 } }),
    });
    renderApp('/radio');
    const grid = await screen.findByRole('list', { name: 'Alle Sender' });
    await userEvent
      .setup()
      .click(within(grid).getByRole('button', { name: 'Radio Swiss Pop abspielen' }));
    await waitFor(() =>
      expect(screen.getByTestId('radio-mini-line')).toHaveTextContent(
        'Schauzeit für heute aufgebraucht Dieses Profil darf 60 Minuten pro Tag schauen. ' +
          'Morgen geht es weiter. Heute geschaut: 61 Minuten.',
      ),
    );
  });
});
