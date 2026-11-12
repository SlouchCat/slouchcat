import type { components, ItemRef } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { audioIds, audiobook, audiobookDetail, heat } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, session, signedIn } from '@/test/api';
import { virtualLayout } from '@/test/virtual';

// Concept 19 stage D2: the row «Reihen und Sammlungen» of an audiobook library (PD-5), the page of
// a series (PD-7) and the link and shelf on the detail page of a volume (D4.3).

type AudioGroupList = components['schemas']['AudioGroupList'];
type AudioSeries = components['schemas']['AudioSeries'];
type LibraryNav = components['schemas']['LibraryNav'];

const books: LibraryNav = {
  id: 5,
  name: 'Hörbücher',
  kind: 'audiobooks',
  contents: ['audiobook'],
  itemCount: 3,
};
const films: LibraryNav = {
  id: 3,
  name: 'Filme',
  kind: 'video',
  contents: ['movie'],
  itemCount: 1,
};

const SERIES = '01JB8Z6Q4M7N9P2R3S5T7V9S01';
const OTHER_SERIES = '01JB8Z6Q4M7N9P2R3S5T7V9S02';
const COLLECTION = '01JB8Z6Q4M7N9P2R3S5T7V9C01';

const groups: AudioGroupList = {
  groups: [
    {
      id: SERIES,
      kind: 'series',
      name: 'Bodyguard',
      author: 'Chris Bradford',
      count: 6,
      matching: 6,
      completed: 3,
      cover: audiobook,
    },
    {
      id: COLLECTION,
      kind: 'collection',
      name: 'Hape für unterwegs',
      count: 3,
      matching: 3,
      completed: 0,
    },
    {
      id: OTHER_SERIES,
      kind: 'series',
      name: 'Silo',
      author: 'Hugh Howey',
      count: 6,
      matching: 2,
      completed: 0,
    },
  ],
  seriesCount: 2,
  collectionCount: 1,
};

const itemsPage = (items: ItemRef[], total = items.length) => ({
  items,
  total,
  offset: 0,
  limit: 100,
  facets: { letters: [], genres: [{ name: 'Krimi', count: 1 }], ratings: [] },
});

/** The query strings of the listAudioGroups requests, in order. */
const groupRequests = (fetchMock: ReturnType<typeof mockApi>['fetchMock']) =>
  fetchMock.mock.calls
    .map(([request]) => new URL(request.url))
    .filter((url) => /\/libraries\/\d+\/audio-groups$/.test(url.pathname))
    .map((url) => url.searchParams);

describe('the row «Reihen und Sammlungen» (PD-5)', () => {
  beforeEach(() => virtualLayout());
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  const mockBooks = (answer: AudioGroupList = groups) =>
    mockApi({
      ...signedIn,
      'GET /libraries/visible': () => json([films, books]),
      'GET /libraries/*/items': () => json(itemsPage([audiobook], 89)),
      'GET /libraries/*/audio-groups': () => json(answer),
    });

  it('shows a tile per series and collection with the number of volumes, above the grid', async () => {
    mockBooks();
    renderApp('/libraries/5');
    const row = await screen.findByRole('region', { name: 'Reihen und Sammlungen' });
    expect(await screen.findByText('89 Hörbücher · 2 Reihen · 1 Sammlung')).toBeVisible();

    // The server's order; a series opens its page, a collection the collection page.
    const links = within(row).getAllByRole('link');
    expect(links.map((link) => link.getAttribute('href'))).toEqual([
      `/libraries/5/series/${SERIES}`,
      `/collections/${COLLECTION}`,
      `/libraries/5/series/${OTHER_SERIES}`,
    ]);
    const bodyguard = links[0]!;
    expect(within(bodyguard).getByText('6 Bände')).toBeVisible();
    expect(within(bodyguard).getByText('Bodyguard')).toBeVisible();
    expect(within(bodyguard).getByText('Chris Bradford · 6 Bände')).toBeVisible();
    expect(within(bodyguard).getByRole('progressbar', { name: '3 von 6 gehört' })).toBeVisible();
    // The collection counts its audiobooks; nothing heard, no bar.
    expect(within(links[1]!).getByText('Sammlung · 3 Hörbücher')).toBeVisible();
    expect(within(links[1]!).queryByRole('progressbar')).not.toBeInTheDocument();
    // A filter lets only some volumes pass.
    expect(within(links[2]!).getByText('2 von 6 Bänden')).toBeVisible();

    // Below: every audiobook on its own, as before.
    const grid = screen.getByRole('list', { name: 'Hörbücher' });
    expect(within(grid).getByRole('link', { name: new RegExp(audiobook.title) })).toBeVisible();
    expect(
      row.compareDocumentPosition(grid) & Node.DOCUMENT_POSITION_FOLLOWING,
      'the row stands above the grid',
    ).toBeTruthy();
  });

  it('asks with the grid’s filters, not its order', async () => {
    const { fetchMock } = mockBooks();
    renderApp('/libraries/5?genre=Krimi&status=unwatched&from=2000&to=2020&rating=12&sort=added');
    await screen.findByRole('region', { name: 'Reihen und Sammlungen' });
    const query = groupRequests(fetchMock).at(-1)!;
    expect(query.get('genre')).toBe('Krimi');
    expect(query.getAll('status')).toEqual(['unwatched']);
    expect(query.get('yearFrom')).toBe('2000');
    expect(query.get('yearTo')).toBe('2020');
    expect(query.get('maxRating')).toBe('12');
    expect(query.has('sort')).toBe(false);
    expect(query.has('offset')).toBe(false);
    expect(
      await screen.findByText('89 Hörbücher · 2 Reihen · 1 Sammlung · gefiltert'),
    ).toBeVisible();
  });

  it('is not there without groups, and the head line counts the books only', async () => {
    mockBooks({ groups: [], seriesCount: 0, collectionCount: 0 });
    renderApp('/libraries/5');
    expect(await screen.findByText('89 Hörbücher')).toBeVisible();
    await screen.findByRole('list', { name: 'Hörbücher' });
    expect(screen.queryByRole('region', { name: 'Reihen und Sammlungen' })).not.toBeInTheDocument();
  });

  it('a film library does not ask for it', async () => {
    const { fetchMock } = mockApi({
      ...signedIn,
      'GET /libraries/visible': () => json([films, books]),
      'GET /libraries/*/items': () => json(itemsPage([heat])),
      'GET /libraries/*/audio-groups': () => json(groups),
    });
    renderApp('/libraries/3');
    await screen.findByRole('list', { name: 'Filme' });
    expect(screen.getByText('1 Titel')).toBeVisible();
    expect(groupRequests(fetchMock)).toHaveLength(0);
    expect(screen.queryByRole('region', { name: 'Reihen und Sammlungen' })).not.toBeInTheDocument();
  });
});

const volume = (id: string, title: string, extra: Partial<ItemRef> = {}): ItemRef => ({
  ...audiobook,
  id,
  title,
  progress: undefined,
  durationMs: 40_260_000,
  ...extra,
});

const done = {
  positionMs: 40_260_000,
  durationMs: 40_260_000,
  percent: 100,
  completed: true,
  updatedAt: '2026-11-01T19:00:00Z',
  watchedAt: '2026-11-01T19:00:00Z',
};

const prequel = volume('01JB8Z6Q4M7N9P2R3S5T7V9B05', 'Die Vorgeschichte', { progress: done });
const first = volume('01JB8Z6Q4M7N9P2R3S5T7V9B01', 'Die Geisel', { progress: done });
const second = volume('01JB8Z6Q4M7N9P2R3S5T7V9B02', 'Das Lösegeld', {
  progress: {
    positionMs: 20_130_000,
    durationMs: 40_260_000,
    percent: 50,
    completed: false,
    updatedAt: '2026-11-02T19:00:00Z',
  },
});
const radioPlay = volume('01JB8Z6Q4M7N9P2R3S5T7V9B03', 'Das Lösegeld (Hörspiel)');
const unnumbered = volume('01JB8Z6Q4M7N9P2R3S5T7V9B04', 'Bodyguard: Sonderband');

const bodyguard: AudioSeries = {
  id: SERIES,
  libraryId: 5,
  name: 'Bodyguard',
  author: 'Chris Bradford',
  durationMs: 5 * 40_260_000,
  cover: prequel,
  next: second,
  // The server's order (D4.3): 0.5 before 1, both books with number 2, the one without last.
  volumes: [
    { item: prequel, seriesIndex: 0.5, narrator: 'Philipp Oehme' },
    { item: first, seriesIndex: 1, narrator: 'Philipp Oehme' },
    { item: second, seriesIndex: 2, narrator: 'Philipp Oehme' },
    { item: radioPlay, seriesIndex: 2, narrator: 'Ensemble' },
    { item: unnumbered, narrator: 'Philipp Oehme' },
  ],
  spellings: [
    { name: 'Bodyguard', count: 3 },
    { name: 'Bodyguard - Band', count: 2 },
  ],
};

const userSession = { ...session, account: { ...session.account, role: 'user' as const } };

describe('the page of a series (PD-7)', () => {
  beforeEach(() => window.localStorage.clear());
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  const mockSeries = (answer: AudioSeries = bodyguard, signed = signedIn) =>
    mockApi({
      ...signed,
      [`GET /libraries/5/series/${SERIES}`]: () => json(answer),
    });

  it('shows the head and the volumes in the server’s order, 0,5 first and without a number last', async () => {
    mockSeries();
    renderApp(`/libraries/5/series/${SERIES}`);
    expect(await screen.findByRole('heading', { level: 1, name: 'Bodyguard' })).toBeVisible();
    expect(screen.getByText('Reihe')).toBeVisible();
    // The narrator differs between the volumes: not in the head, but at the volumes.
    expect(screen.getByText('Chris Bradford · 5 Bände · 55 h 55 min')).toBeVisible();
    expect(document.title).toContain('Bodyguard');

    const list = screen.getByRole('list', { name: 'Bände' });
    const rows = within(list).getAllByRole('listitem');
    expect(rows.map((row) => within(row).getAllByRole('link')[0]!.getAttribute('href'))).toEqual(
      [prequel, first, second, radioPlay, unnumbered].map((item) => `/items/${item.id}`),
    );
    expect(
      within(rows[0]!).getByRole('link', { name: 'Band 0,5: Die Vorgeschichte' }),
    ).toBeVisible();
    // Two books with number 2, told apart by the narrator.
    expect(within(rows[2]!).getByText('Philipp Oehme · noch 5 h 36 min')).toBeVisible();
    expect(within(rows[2]!).getByRole('progressbar', { name: '50 % gehört' })).toBeVisible();
    expect(
      within(rows[3]!).getByRole('link', { name: 'Band 2: Das Lösegeld (Hörspiel)' }),
    ).toBeVisible();
    expect(within(rows[3]!).getByText('Ensemble')).toBeVisible();
    // A finished volume: its check mark set at the end of the row (concept 20 P-5), and its
    // button starts it again.
    expect(within(rows[1]!).getByRole('button', { name: 'Gehört: Die Geisel' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(within(rows[2]!).getByRole('button', { name: /^Gehört: / })).toHaveAttribute(
      'aria-pressed',
      'false',
    );
    expect(within(rows[1]!).getByRole('link', { name: 'Abspielen: Die Geisel' })).toHaveAttribute(
      'href',
      `/play/${first.id}?at=0`,
    );
    expect(within(rows[4]!).getByRole('link', { name: /^Bodyguard: Sonderband/ })).toBeVisible();
    expect(
      within(rows[4]!).getByRole('link', { name: 'Abspielen: Bodyguard: Sonderband' }),
    ).toHaveAttribute('href', `/play/${unnumbered.id}`);
  });

  it('«Weiterhören» plays the volume the server names, not the first unheard one it could find', async () => {
    // `next` is the server's choice; the client does not pick it from the list.
    mockSeries({ ...bodyguard, next: radioPlay });
    renderApp(`/libraries/5/series/${SERIES}`);
    expect(await screen.findByRole('link', { name: 'Weiterhören: Band 2' })).toHaveAttribute(
      'href',
      `/play/${radioPlay.id}`,
    );
  });

  it('nothing heard yet: «Abspielen»', async () => {
    const fresh = bodyguard.volumes.map((entry) => ({
      ...entry,
      item: { ...entry.item, progress: undefined },
    }));
    mockSeries({ ...bodyguard, volumes: fresh, next: fresh[0]!.item });
    renderApp(`/libraries/5/series/${SERIES}`);
    expect(await screen.findByRole('link', { name: 'Abspielen' })).toHaveAttribute(
      'href',
      `/play/${prequel.id}`,
    );
  });

  it('the administrator gets the hint on spellings and numbers; «Verstanden» keeps it away', async () => {
    mockSeries();
    const { unmount } = renderApp(`/libraries/5/series/${SERIES}`);
    const hint = await screen.findByText('Nur für Admins');
    const notice = hint.closest('[role="status"]') as HTMLElement;
    expect(
      within(notice).getByText(
        'Diese Reihe steht in den Dateien unter mehreren Namen: «Bodyguard» (3 Bände), «Bodyguard - Band» (2 Bände). Slouchcat zeigt sie als eine Reihe.',
      ),
    ).toBeVisible();
    expect(within(notice).getByText(/Diese Nummern kommen mehrfach vor: 2\./)).toBeVisible();
    expect(screen.getByRole('button', { name: 'Reihe bearbeiten' })).toBeVisible();
    expect(screen.getByRole('button', { name: 'Die Geisel: Metadaten bearbeiten' })).toBeVisible();

    await userEvent.setup().click(within(notice).getByRole('button', { name: 'Verstanden' }));
    expect(screen.queryByText('Nur für Admins')).not.toBeInTheDocument();
    unmount();

    // Dismissed for this set of findings: the next visit shows none.
    mockSeries();
    const again = renderApp(`/libraries/5/series/${SERIES}`);
    await screen.findByRole('heading', { level: 1, name: 'Bodyguard' });
    expect(screen.queryByText('Nur für Admins')).not.toBeInTheDocument();
    again.unmount();

    // A new spelling is a new finding: the hint comes back.
    mockSeries({
      ...bodyguard,
      spellings: [...bodyguard.spellings, { name: 'Bodyguard-Reihe', count: 1 }],
    });
    renderApp(`/libraries/5/series/${SERIES}`);
    expect(await screen.findByText('Nur für Admins')).toBeVisible();
  });

  it('a profile that does not manage sees no hint, no «Reihe bearbeiten» and no pen', async () => {
    mockSeries(bodyguard, { 'GET /auth/session': () => json(userSession) });
    renderApp(`/libraries/5/series/${SERIES}`);
    await screen.findByRole('heading', { level: 1, name: 'Bodyguard' });
    expect(screen.queryByText('Nur für Admins')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Reihe bearbeiten' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Metadaten bearbeiten/ })).not.toBeInTheDocument();
  });

  it('a series with too few visible volumes is not found', async () => {
    mockApi({
      ...signedIn,
      [`GET /libraries/5/series/${SERIES}`]: () => problem(404, 'not_found'),
    });
    renderApp(`/libraries/5/series/${SERIES}`);
    expect(await screen.findByText('Reihe nicht gefunden')).toBeVisible();
    expect(screen.getByRole('link', { name: 'Zur Bibliothek' })).toHaveAttribute(
      'href',
      '/libraries/5',
    );
  });

  it('«Reihe bearbeiten» renames, writes into the volumes and follows a merge to its new id', async () => {
    const { calls } = mockApi({
      ...signedIn,
      [`GET /libraries/5/series/${SERIES}`]: () => json(bodyguard),
      [`GET /libraries/5/series/${OTHER_SERIES}`]: () =>
        json({ ...bodyguard, id: OTHER_SERIES, name: 'Bodyguard (Reihe)' }),
      [`PATCH /admin/audio-series/${SERIES}`]: () =>
        json({
          id: OTHER_SERIES,
          libraryId: 5,
          name: 'Bodyguard (Reihe)',
          nameLocked: true,
          volumeCount: 5,
        }),
    });
    const { router } = renderApp(`/libraries/5/series/${SERIES}`);
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Reihe bearbeiten' }));
    const dialog = await screen.findByRole('dialog', { name: 'Reihe bearbeiten' });
    const name = within(dialog).getByRole('textbox', { name: 'Name der Reihe' });
    expect(name).toHaveValue('Bodyguard');
    await user.clear(name);
    await user.type(name, 'Bodyguard (Reihe)');
    await user.click(within(dialog).getByRole('checkbox', { name: /In alle Bände schreiben/ }));
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));

    await waitFor(() =>
      expect(router.state.location.pathname).toBe(`/libraries/5/series/${OTHER_SERIES}`),
    );
    expect(calls.find((call) => call.method === 'PATCH')?.body).toEqual({
      name: 'Bodyguard (Reihe)',
      writeToVolumes: true,
    });
    expect(
      await screen.findByRole('heading', { level: 1, name: 'Bodyguard (Reihe)' }),
    ).toBeVisible();
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });
});

describe('the detail page of a volume (D4.3)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('links «Reihe · Band n» to the series page and shows its volumes as a shelf', async () => {
    mockApi({
      ...signedIn,
      [`GET /items/${audioIds.book}`]: () =>
        json({ ...audiobookDetail, seriesId: SERIES, libraryId: 5 }),
      [`GET /libraries/5/series/${SERIES}`]: () =>
        json({
          ...bodyguard,
          name: 'Slouchcat Test Audio Series',
          volumes: [{ item: audiobook, seriesIndex: 1 }, ...bodyguard.volumes.slice(2, 3)],
        }),
    });
    renderApp(`/items/${audioIds.book}`);
    expect(
      await screen.findByRole('link', { name: 'Slouchcat Test Audio Series · Band 1' }),
    ).toHaveAttribute('href', `/libraries/5/series/${SERIES}`);
    const shelf = await screen.findByRole('region', { name: 'Slouchcat Test Audio Series' });
    expect(within(shelf).getByText('2 Bände')).toBeVisible();
    const members = within(shelf).getByRole('list', { name: 'Slouchcat Test Audio Series' });
    const cards = within(members).getAllByRole('link');
    expect(cards.map((card) => card.getAttribute('href'))).toEqual([
      `/items/${audioIds.book}`,
      `/items/${second.id}`,
    ]);
    expect(cards[0]).toHaveAttribute('aria-current', 'true');
    expect(
      within(shelf).getByRole('link', { name: 'Alle zeigen: Slouchcat Test Audio Series' }),
    ).toHaveAttribute('href', `/libraries/5/series/${SERIES}`);
  });

  it('without a series id the series line stays text and no shelf is asked for', async () => {
    const { fetchMock } = mockApi({
      ...signedIn,
      [`GET /items/${audioIds.book}`]: () => json({ ...audiobookDetail, libraryId: 5 }),
    });
    renderApp(`/items/${audioIds.book}`);
    expect(await screen.findByText('Slouchcat Test Audio Series · Band 1')).toBeVisible();
    expect(
      screen.queryByRole('link', { name: 'Slouchcat Test Audio Series · Band 1' }),
    ).not.toBeInTheDocument();
    expect(
      fetchMock.mock.calls.some(([request]) => new URL(request.url).pathname.includes('/series/')),
    ).toBe(false);
  });
});
