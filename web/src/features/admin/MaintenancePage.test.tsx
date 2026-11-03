import type {
  DuplicateGroup,
  Issue,
  IssueKind,
  IssuePage,
  RelinkPair,
  RelinkPairList,
  RelinkUserData,
} from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, session, signedIn, type Call } from '@/test/api';
import { library } from '@/test/fixtures';

const id = (n: number) => `01JB8Z6Q4M7N9P2R3S5T7V9X${String(n).padStart(2, '0')}`;

const issue = (kind: IssueKind, n: number, extra: Partial<Issue> = {}): Issue => ({
  kind,
  itemId: id(n),
  itemKind: 'movie',
  title: `Titel ${n}`,
  libraryId: 1,
  ...extra,
});

const initial: Record<IssueKind, Issue[]> = {
  unmatched: [
    issue('unmatched', 1, { title: 'Slouchcat Test H264', year: 2024 }),
    issue('unmatched', 2, { itemKind: 'episode', title: 'Episode 1', libraryId: 2 }),
  ],
  no_images: [],
  unrated: [issue('unrated', 3, { title: 'Slouchcat Test Unrated' }), issue('unrated', 4)],
  orphaned: [issue('orphaned', 5)],
  unplayable: [
    issue('unplayable', 6, {
      title: 'Slouchcat Test Broken',
      path: 'Slouchcat Test Broken (2024)/Slouchcat Test Broken.mkv',
      mediaFileId: 'mf-broken',
      detail: 'invalid data found when processing input',
    }),
  ],
  rating_mismatch: [issue('rating_mismatch', 7)],
};

let lists = { ...initial };

const libraries = [library(), library({ id: 2, name: 'Serien' })];

const groups: DuplicateGroup[] = [
  {
    reason: 'file_hash',
    items: [
      {
        itemId: id(8),
        kind: 'movie',
        title: 'Slouchcat Test H264',
        year: 2024,
        libraryId: 1,
        files: [{ id: 'mf-a', path: 'H264/a.mp4', sizeBytes: 1_000_000, available: true }],
      },
      {
        itemId: id(9),
        kind: 'movie',
        title: 'Slouchcat Test H264 Copy',
        year: 2024,
        libraryId: 2,
        files: [{ id: 'mf-b', path: 'Copy/a.mp4', sizeBytes: 1_000_000, available: true }],
      },
    ],
  },
  {
    reason: 'title_year',
    items: [
      { itemId: id(10), kind: 'series', title: 'Das Boot', year: 1985, files: [] },
      { itemId: id(11), kind: 'series', title: 'Das Boot', year: 1985, files: [] },
    ],
  },
];

const noUserData: RelinkUserData = {
  progress: 0,
  progressEvents: 0,
  progressDays: 0,
  playbackSessions: 0,
  watchlist: 0,
  favorites: 0,
  collectionItems: 0,
  notifications: 0,
  topPicks: 0,
  downloads: 0,
  activityEvents: 0,
};

const pair = (n: number, extra: Partial<RelinkPair> = {}): RelinkPair => ({
  deletedId: id(20 + n),
  liveId: id(30 + n),
  libraryId: 2,
  libraryName: 'Serien',
  kind: 'series',
  title: `Serie ${n}`,
  deletedTitle: `Serie ${n}`,
  deletedAt: '2026-10-28T09:00:00Z',
  userData: noUserData,
  ...extra,
});

const initialPairs: RelinkPairList = {
  total: 2,
  pairs: [
    pair(1, {
      title: 'Two and a Half Men',
      year: 2003,
      deletedTitle: 'Two and a Half Men',
      deletedYear: 2003,
      userData: { ...noUserData, progress: 2, progressEvents: 100, playbackSessions: 1 },
    }),
    pair(2, {
      title: 'Silo',
      deletedTitle: 'Silo',
      userData: { ...noUserData, progress: 1, progressEvents: 40 },
    }),
  ],
};

let relinkPairs = initialPairs;

/** The issue endpoint over `lists`, with limit and offset like the server. */
function issuesRoute(url: URL): Response {
  const kind = url.searchParams.get('kind') as IssueKind;
  const limit = Number(url.searchParams.get('limit') ?? 50);
  const offset = Number(url.searchParams.get('offset') ?? 0);
  const all = lists[kind];
  return json({ total: all.length, items: all.slice(offset, offset + limit) } satisfies IssuePage);
}

/** mockApi passes only the path; read the query from the last fetch call. */
function withQuery(fetchMock: { mock: { calls: unknown[][] } }) {
  return () => {
    const request = fetchMock.mock.calls.at(-1)?.[0] as Request;
    return new URL(request.url);
  };
}

function setup(extra: Record<string, (call: Call) => Response> = {}) {
  let lastUrl: () => URL = () => new URL('http://localhost/');
  const api = mockApi({
    ...signedIn,
    'GET /admin/maintenance/issues': () => issuesRoute(lastUrl()),
    'GET /admin/maintenance/duplicates': () => json(groups),
    'GET /admin/maintenance/relink': () => json(relinkPairs),
    'GET /admin/libraries': () => json(libraries),
    ...extra,
  });
  lastUrl = withQuery(api.fetchMock);
  return api;
}

const panel = () => screen.getByRole('tabpanel');
const title = () => screen.getByRole('heading', { level: 1, name: 'Pflege' });

describe('maintenance page', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    lists = { ...initial };
    relinkPairs = initialPairs;
  });

  it('shows a tab per issue kind with its count and lists the first one by library', async () => {
    setup();
    renderApp('/admin/maintenance');

    expect(await screen.findByRole('heading', { level: 1, name: 'Pflege' })).toBeVisible();
    const tabs = screen.getByRole('tablist', { name: 'Problemarten' });
    await waitFor(() =>
      expect(within(tabs).getByRole('tab', { name: 'Nicht zugeordnet 2 Einträge' })).toBeVisible(),
    );
    expect(within(tabs).getByRole('tab', { name: 'Nicht abspielbar 1 Eintrag' })).toBeVisible();
    expect(within(tabs).getByRole('tab', { name: 'Ohne Bilder 0 Einträge' })).toBeVisible();
    expect(
      within(tabs).getByRole('tab', { name: 'Altersfreigabe fehlt 2 Einträge' }),
    ).toBeVisible();
    expect(within(tabs).getByRole('tab', { name: 'Verwaist 1 Eintrag' })).toBeVisible();
    expect(within(tabs).getByRole('tab', { name: 'Freigabe abweichend 1 Eintrag' })).toBeVisible();
    expect(await within(tabs).findByRole('tab', { name: 'Duplikate 2 Gruppen' })).toBeVisible();
    expect(
      await within(tabs).findByRole('tab', { name: 'Verwaiste Benutzerdaten 2 Titel' }),
    ).toBeVisible();
    // The intro sits behind the help button (S-2); the tab's hint stays visible.
    expect(within(panel()).getByText(/^Ohne Zuordnung fehlen Beschreibung/)).toBeVisible();

    const link = await screen.findByRole('link', { name: 'Slouchcat Test H264 (2024)' });
    expect(link).toHaveAttribute('href', `/admin/items/${id(1)}`);
    // One section per library, named from the library list, each with its own table.
    expect(screen.getByRole('button', { name: 'Filme 1 Eintrag' })).toHaveAttribute(
      'aria-expanded',
      'true',
    );
    const movies = screen.getByRole('group', { name: 'Filme 1 Eintrag' });
    expect(within(movies).getByRole('table', { name: 'Nicht zugeordnet in Filme' })).toBeVisible();
    expect(within(movies).getByRole('link', { name: 'Slouchcat Test H264 (2024)' })).toBeVisible();
    const series = screen.getByRole('group', { name: 'Serien 1 Eintrag' });
    expect(within(series).getByRole('link', { name: 'Episode 1' })).toBeVisible();
    expect(within(series).getByText('Folge')).toBeVisible();
    expect(screen.getByText('2 von 2')).toBeVisible();
    // Without path and finding: a dash on screen, «keine Angabe» for screen readers.
    const episodeRow = within(series).getByRole('row', { name: /^Episode 1/ });
    for (const dash of within(episodeRow).getAllByText('–'))
      expect(dash).toHaveAttribute('aria-hidden', 'true');
    expect(within(episodeRow).getAllByText('keine Angabe')).toHaveLength(2);
  });

  it('groups the loaded issues in the order of the libraries; an unknown library comes last', async () => {
    const user = userEvent.setup();
    lists.unrated = [
      issue('unrated', 20, { libraryId: 9 }),
      issue('unrated', 21, { libraryId: 2 }),
      issue('unrated', 22, { libraryId: 1 }),
      issue('unrated', 23, { libraryId: 2 }),
    ];
    setup();
    renderApp('/admin/maintenance?tab=unrated');
    await screen.findByRole('link', { name: 'Titel 20' });

    const headings = within(panel()).getAllByRole('heading', { level: 3 });
    expect(headings).toHaveLength(3);
    expect(headings[0]).toHaveAccessibleName('Filme 1 Eintrag');
    expect(headings[1]).toHaveAccessibleName('Serien 2 Einträge');
    expect(headings[2]).toHaveAccessibleName('Bibliothek 9 1 Eintrag');
    const series = screen.getByRole('group', { name: 'Serien 2 Einträge' });
    expect(
      within(series)
        .getAllByRole('link')
        .map((link) => link.textContent),
    ).toEqual(['Titel 21', 'Titel 23']);

    // A section folds away with its button; the others stay open, the selection stays.
    await user.click(within(series).getByRole('checkbox', { name: 'Titel 21 auswählen' }));
    // The section's heading line counts its selected entries.
    expect(within(panel()).getAllByText('1 ausgewählt')).toHaveLength(2);
    const toggle = screen.getByRole('button', { name: 'Serien 2 Einträge' });
    toggle.focus();
    await user.keyboard('{Enter}');
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByRole('link', { name: 'Titel 21' })).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Titel 22' })).toBeVisible();
    expect(within(panel()).getAllByText('1 ausgewählt')).toHaveLength(2);
    await user.keyboard('{Enter}');
    expect(screen.getByRole('checkbox', { name: 'Titel 21 auswählen' })).toBeChecked();
  });

  it('unplayable: select, remove after confirmation, the list reloads and the h1 takes the focus', async () => {
    const user = userEvent.setup();
    const { calls } = setup({
      'POST /admin/maintenance/cleanup': () => {
        lists.unplayable = [];
        return json({ items: 1, files: 1 });
      },
    });
    renderApp('/admin/maintenance');
    await user.click(await screen.findByRole('tab', { name: /^Nicht abspielbar/ }));

    const row = await screen.findByRole('link', { name: 'Slouchcat Test Broken' });
    expect(row).toBeVisible();
    // Path and finding in full (S-5).
    expect(screen.getByText('invalid data found when processing input')).toBeVisible();
    expect(
      screen.getByText('Slouchcat Test Broken (2024)/Slouchcat Test Broken.mkv'),
    ).toBeVisible();
    const remove = screen.getByRole('button', { name: 'Entfernen' });
    expect(remove).toBeDisabled();
    await user.click(screen.getByRole('checkbox', { name: 'Slouchcat Test Broken auswählen' }));
    expect(within(panel()).getAllByText('1 ausgewählt')[0]).toBeVisible();
    await user.click(remove);
    const dialog = await screen.findByRole('alertdialog', { name: 'Dateien entfernen?' });
    expect(within(dialog).getByText(/^Die ausgewählte Datei wird/)).toBeVisible();
    await user.click(within(dialog).getByRole('button', { name: 'Entfernen' }));

    await waitFor(() =>
      expect(calls.find((call) => call.path === '/admin/maintenance/cleanup')?.body).toEqual({
        mediaFileIds: ['mf-broken'],
      }),
    );
    expect(await screen.findByText('Entfernt: 1 Titel, 1 Datei.')).toBeVisible();
    await waitFor(() =>
      expect(screen.queryByRole('link', { name: 'Slouchcat Test Broken' })).not.toBeInTheDocument(),
    );
    await waitFor(() => expect(title()).toHaveFocus());
  });

  it('a confirmation stays when its rows go meanwhile and still acts on them', async () => {
    const user = userEvent.setup();
    const { calls } = setup({
      'POST /admin/maintenance/cleanup': () => json({ items: 1, files: 0 }),
    });
    const { queryClient } = renderApp('/admin/maintenance?tab=orphaned');
    await user.click(await screen.findByRole('checkbox', { name: 'Titel 5 auswählen' }));
    await user.click(screen.getByRole('button', { name: 'Bereinigen' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Titel bereinigen?' });

    lists.orphaned = [];
    await queryClient.invalidateQueries({ queryKey: ['admin', 'maintenance'] });
    await waitFor(() =>
      expect(screen.queryByRole('link', { name: 'Titel 5' })).not.toBeInTheDocument(),
    );
    expect(dialog).toBeVisible();
    await user.click(within(dialog).getByRole('button', { name: 'Bereinigen' }));
    await waitFor(() =>
      expect(calls.find((call) => call.path === '/admin/maintenance/cleanup')?.body).toEqual({
        itemIds: [id(5)],
      }),
    );
    await waitFor(() => expect(title()).toHaveFocus());
  });

  it('unrated: assigns a rating to the selection', async () => {
    const user = userEvent.setup();
    const { calls } = setup({ 'POST /admin/items/rating': () => json({ updated: 2 }) });
    renderApp('/admin/maintenance');
    await user.click(await screen.findByRole('tab', { name: /^Altersfreigabe fehlt/ }));
    await screen.findByRole('link', { name: 'Slouchcat Test Unrated' });
    await user.click(screen.getByRole('button', { name: 'Alle auswählen' }));
    await user.click(screen.getByRole('button', { name: 'Freigabe zuweisen' }));

    const dialog = await screen.findByRole('dialog', { name: 'Freigabe zuweisen' });
    expect(within(dialog).getByText('Gilt für 2 ausgewählte Titel.')).toBeVisible();
    // Nothing is chosen up front (FSK 0 would open the titles to every profile).
    for (const radio of within(dialog).getAllByRole('radio')) expect(radio).not.toBeChecked();
    const submit = within(dialog).getByRole('button', { name: 'Zuweisen' });
    expect(submit).toBeDisabled();
    await user.click(submit);
    expect(calls.some((call) => call.path === '/admin/items/rating')).toBe(false);

    await user.click(within(dialog).getByRole('radio', { name: 'FSK 12' }));
    expect(within(dialog).getByText('2 Titel erhalten FSK 12.')).toBeVisible();
    expect(submit).toBeEnabled();
    await user.click(submit);

    await waitFor(() =>
      expect(calls.find((call) => call.path === '/admin/items/rating')?.body).toEqual({
        itemIds: [id(3), id(4)],
        rating: 12,
      }),
    );
    expect(await screen.findByText('Freigabe für 2 Titel gesetzt.')).toBeVisible();
  });

  it('unmatched: match opens the search; a missing key reads as the provider error', async () => {
    const user = userEvent.setup();
    const { calls } = setup({
      'GET /admin/metadata/search': () => problem(422, 'tmdb_key_invalid'),
      [`POST /admin/items/${id(1)}/refresh`]: () => json({}, 202),
    });
    renderApp('/admin/maintenance');
    await screen.findByRole('link', { name: 'Slouchcat Test H264 (2024)' });

    const match = screen.getByRole('button', { name: 'Zuordnen' });
    // Episodes are matched through their series: one film or series only.
    await user.click(screen.getByRole('checkbox', { name: 'Episode 1 auswählen' }));
    expect(match).toBeDisabled();
    await user.click(screen.getByRole('checkbox', { name: 'Episode 1 auswählen' }));
    await user.click(screen.getByRole('checkbox', { name: 'Slouchcat Test H264 auswählen' }));
    expect(match).toBeEnabled();

    await user.click(match);
    const dialog = await screen.findByRole('dialog', { name: 'Titel zuordnen' });
    expect(within(dialog).getByLabelText('Suchbegriff')).toHaveValue('Slouchcat Test H264');
    expect(within(dialog).getByLabelText('Jahr (optional)')).toHaveValue('2024');
    await user.click(within(dialog).getByRole('button', { name: 'Suchen' }));
    expect(await within(dialog).findByText(/TMDB hat den Schlüssel abgelehnt/)).toBeVisible();
    await user.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Aktualisieren' }));
    await waitFor(() =>
      expect(calls.some((call) => call.path === `/admin/items/${id(1)}/refresh`)).toBe(true),
    );
    expect(await screen.findByText('1 Titel wird aktualisiert.')).toBeVisible();
  });

  it('refresh: one request per title; a partial failure is reported and the list reloads', async () => {
    const user = userEvent.setup();
    const { calls } = setup({
      [`POST /admin/items/${id(1)}/refresh`]: () => json({}, 202),
      [`POST /admin/items/${id(2)}/refresh`]: () => problem(404, 'not_found'),
    });
    renderApp('/admin/maintenance');
    await screen.findByRole('link', { name: 'Slouchcat Test H264 (2024)' });
    const listed = () =>
      calls.filter((call) => call.method === 'GET' && call.path === '/admin/maintenance/issues')
        .length;
    const before = listed();

    await user.click(screen.getByRole('button', { name: 'Alle auswählen' }));
    await user.click(screen.getByRole('button', { name: 'Aktualisieren' }));
    expect(await screen.findByText(/^1 von 2 Titeln nicht aktualisiert: /)).toBeVisible();
    expect(calls.filter((call) => call.path.endsWith('/refresh'))).toHaveLength(2);
    await waitFor(() => expect(listed()).toBeGreaterThan(before));
    expect(screen.getByText('0 ausgewählt')).toBeVisible();
    // The button went off with the selection: the h1 takes the focus.
    await waitFor(() => expect(title()).toHaveFocus());
  });

  it('PFL-2: «Alle auswählen» turns into «Auswahl aufheben», which clears the selection', async () => {
    const user = userEvent.setup();
    setup();
    renderApp('/admin/maintenance?tab=unrated');
    await screen.findByRole('checkbox', { name: 'Slouchcat Test Unrated auswählen' });
    const boxes = async () => within(panel()).findAllByRole('checkbox');
    expect(await boxes()).toHaveLength(2);
    const assign = within(panel()).getByRole('button', { name: 'Freigabe zuweisen' });
    expect(assign).toBeDisabled();

    await user.click(within(panel()).getByRole('button', { name: 'Alle auswählen' }));
    expect(within(panel()).getAllByText('2 ausgewählt')[0]).toBeVisible();
    for (const box of await boxes()) expect(box).toBeChecked();
    expect(assign).toBeEnabled();

    await user.click(within(panel()).getByRole('button', { name: 'Auswahl aufheben' }));
    expect(within(panel()).getAllByText('0 ausgewählt')[0]).toBeVisible();
    for (const box of await boxes()) expect(box).not.toBeChecked();
    expect(assign).toBeDisabled();
    expect(within(panel()).getByRole('button', { name: 'Alle auswählen' })).toBeVisible();
  });

  it('PFL-3: «Aktualisieren» without images and «Erneut prüfen» of an unplayable file refresh the title', async () => {
    const user = userEvent.setup();
    lists.no_images = [issue('no_images', 12, { title: 'Ohne Poster' })];
    const { calls } = setup({
      [`POST /admin/items/${id(12)}/refresh`]: () => json({}, 202),
      [`POST /admin/items/${id(6)}/refresh`]: () => json({}, 202),
    });
    const refreshed = () =>
      calls.filter((call) => call.method === 'POST' && call.path.endsWith('/refresh'));
    renderApp('/admin/maintenance?tab=no_images');

    await user.click(await screen.findByRole('checkbox', { name: 'Ohne Poster auswählen' }));
    await user.click(within(panel()).getByRole('button', { name: 'Aktualisieren' }));
    expect(await screen.findByText('1 Titel wird aktualisiert.')).toBeVisible();
    expect(refreshed().map((call) => call.path)).toEqual([`/admin/items/${id(12)}/refresh`]);

    await user.click(screen.getByRole('tab', { name: /^Nicht abspielbar/ }));
    const recheck = within(panel()).getByRole('button', { name: 'Erneut prüfen' });
    expect(recheck).toBeDisabled();
    await user.click(
      await screen.findByRole('checkbox', { name: 'Slouchcat Test Broken auswählen' }),
    );
    await user.click(recheck);
    await waitFor(() =>
      expect(refreshed().map((call) => call.path)).toEqual([
        `/admin/items/${id(12)}/refresh`,
        `/admin/items/${id(6)}/refresh`,
      ]),
    );
    expect(await within(panel()).findByText('1 Titel wird aktualisiert.')).toBeVisible();
    // «Erneut prüfen» re-reads the title; the file stays until the probe says otherwise.
    expect(calls.some((call) => call.path === '/admin/maintenance/cleanup')).toBe(false);
  });

  it('orphaned: the confirmation names the count; the trigger is off while it runs', async () => {
    const user = userEvent.setup();
    let finish: (response: Response) => void = () => undefined;
    lists.orphaned = [issue('orphaned', 5), issue('orphaned', 6)];
    setup({
      'POST /admin/maintenance/cleanup': (() =>
        new Promise<Response>((resolve) => {
          finish = resolve;
        })) as unknown as () => Response,
    });
    renderApp('/admin/maintenance');
    await user.click(await screen.findByRole('tab', { name: /^Verwaist \d/ }));
    await screen.findByRole('link', { name: 'Titel 5' });
    await user.click(screen.getByRole('button', { name: 'Alle auswählen' }));
    const trigger = screen.getByRole('button', { name: 'Bereinigen' });
    await user.click(trigger);
    const dialog = await screen.findByRole('alertdialog', { name: 'Titel bereinigen?' });
    expect(within(dialog).getByText(/^Die 2 ausgewählten Titel werden/)).toBeVisible();
    await user.click(within(dialog).getByRole('button', { name: 'Bereinigen' }));
    await waitFor(() => expect(trigger).toBeDisabled());

    lists.orphaned = [];
    finish(json({ items: 2, files: 0 }));
    expect(await screen.findByText('Entfernt: 2 Titel, 0 Dateien.')).toBeVisible();
  });

  it('match: searching again with the same values asks the server again', async () => {
    const user = userEvent.setup();
    let reachable = false;
    const { calls } = setup({
      'GET /admin/metadata/search': () => (reachable ? json([]) : problem(503, 'tmdb_unreachable')),
    });
    renderApp('/admin/maintenance');
    await user.click(
      await screen.findByRole('checkbox', { name: 'Slouchcat Test H264 auswählen' }),
    );
    await user.click(screen.getByRole('button', { name: 'Zuordnen' }));
    const dialog = await screen.findByRole('dialog', { name: 'Titel zuordnen' });
    await user.click(within(dialog).getByRole('button', { name: 'Suchen' }));
    const searches = () => calls.filter((call) => call.path === '/admin/metadata/search').length;
    await waitFor(() => expect(within(dialog).getByRole('alert')).toBeVisible());
    expect(searches()).toBe(1);

    reachable = true;
    await user.click(within(dialog).getByRole('button', { name: 'Suchen' }));
    await waitFor(() => expect(searches()).toBe(2));
    expect(await within(dialog).findByText('Keine Treffer.')).toBeVisible();
    expect(within(dialog).queryByRole('alert')).not.toBeInTheDocument();
  });

  it('match: choosing a candidate stores it', async () => {
    const user = userEvent.setup();
    const { calls } = setup({
      'GET /admin/metadata/search': () =>
        json([
          {
            provider: 'tmdb',
            providerId: '348',
            title: 'Alien',
            year: 1979,
            overview: 'Die Nostromo.',
          },
        ]),
      [`PUT /admin/items/${id(1)}/match`]: () => json({}, 202),
    });
    renderApp('/admin/maintenance');
    await user.click(
      await screen.findByRole('checkbox', { name: 'Slouchcat Test H264 auswählen' }),
    );
    await user.click(screen.getByRole('button', { name: 'Zuordnen' }));
    const dialog = await screen.findByRole('dialog', { name: 'Titel zuordnen' });
    await user.click(within(dialog).getByRole('button', { name: 'Suchen' }));
    const search = calls.find((call) => call.path === '/admin/metadata/search');
    expect(search).toBeDefined();
    await user.click(await within(dialog).findByRole('button', { name: 'Alien (1979) zuordnen' }));
    await waitFor(() =>
      expect(calls.find((call) => call.path === `/admin/items/${id(1)}/match`)?.body).toEqual({
        provider: 'tmdb',
        providerId: '348',
      }),
    );
    expect(
      await screen.findByText('Zuordnung gespeichert, die Metadaten werden geladen.'),
    ).toBeVisible();
  });

  it('rating mismatch: display only, without checkboxes', async () => {
    const user = userEvent.setup();
    setup();
    renderApp('/admin/maintenance');
    await user.click(await screen.findByRole('tab', { name: /^Freigabe abweichend/ }));
    await screen.findByRole('link', { name: 'Titel 7' });
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Alle auswählen' })).not.toBeInTheDocument();
    expect(screen.getByText(/Hier gibt es nichts auszuwählen/)).toBeVisible();
  });

  it('duplicates: one section per group across libraries; pick the title to keep, merge after confirmation', async () => {
    const user = userEvent.setup();
    let finish: (response: Response) => void = () => undefined;
    const { calls } = setup({
      'POST /admin/maintenance/merge': (() =>
        new Promise<Response>((resolve) => {
          finish = resolve;
        })) as unknown as () => Response,
    });
    renderApp('/admin/maintenance');
    await user.click(await screen.findByRole('tab', { name: /^Duplikate/ }));
    const group = await screen.findByRole('group', { name: 'Gruppe 1' });
    expect(screen.getByRole('heading', { level: 3, name: 'Gruppe 1' })).toBeVisible();
    // Titles of two libraries in one group: not split by library.
    expect(
      screen.getByText(/^Gleicher Dateiinhalt · Slouchcat Test H264 \(2024\) · 2 Titel$/),
    ).toBeVisible();
    expect(within(group).getByText('Copy/a.mp4')).toBeVisible();
    expect(within(group).getByText('H264/a.mp4')).toBeVisible();
    // PFL-4: each file with its size.
    expect(within(group).getAllByText('1 MB')).toHaveLength(2);
    // Only the first group starts open (mockup Pfl-C); the second one opens with its button.
    const secondToggle = screen.getByRole('button', { name: 'Gruppe 2' });
    expect(secondToggle).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByRole('group', { name: 'Gruppe 2' })).not.toBeInTheDocument();
    await user.click(secondToggle);
    const second = screen.getByRole('group', { name: 'Gruppe 2' });
    expect(within(second).getAllByText('Keine Datei')).toHaveLength(2);

    // «bleibt» marks the title to keep and follows the choice.
    const items = within(group).getByRole('list', { name: 'Gruppe 1' });
    const copyRow = within(items)
      .getByRole('link', { name: 'Slouchcat Test H264 Copy (2024)' })
      .closest('li') as HTMLElement;
    expect(within(copyRow).queryByText('bleibt')).not.toBeInTheDocument();
    await user.click(within(group).getByRole('radio', { name: '2: Slouchcat Test H264 Copy' }));
    expect(within(copyRow).getByText('bleibt')).toBeVisible();

    await user.click(within(group).getByRole('button', { name: 'Zusammenführen' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Titel zusammenführen?' });
    expect(within(dialog).getByText(/wandern zu «Slouchcat Test H264 Copy»/)).toBeVisible();
    await user.click(within(dialog).getByRole('button', { name: 'Zusammenführen' }));
    await waitFor(() =>
      expect(calls.find((call) => call.path === '/admin/maintenance/merge')?.body).toEqual({
        targetId: id(9),
        sourceIds: [id(8)],
      }),
    );
    // While one group merges, the other groups wait.
    await waitFor(() =>
      expect(within(second).getByRole('button', { name: 'Zusammenführen' })).toBeDisabled(),
    );

    finish(json({}));
    // The outcome lands in the tab's live region, which was there before it.
    const outcome = await within(panel()).findByText('Titel zusammengeführt.');
    expect(outcome.closest('[role="status"]')).not.toBeNull();
    await waitFor(() => expect(title()).toHaveFocus());
  });

  it('orphaned user data: every pair starts selected; «Umhängen» sums the user data, asks and relinks', async () => {
    const user = userEvent.setup();
    let finish: (response: Response) => void = () => undefined;
    const { calls } = setup({
      'POST /admin/maintenance/relink': (() =>
        new Promise<Response>((resolve) => {
          finish = resolve;
        })) as unknown as () => Response,
    });
    renderApp('/admin/maintenance?tab=relink');
    const table = await screen.findByRole('table', { name: 'Verwaiste Benutzerdaten' });
    expect(within(panel()).getByText(/^Gelöschte Titel, die als neue Kopie/)).toBeVisible();
    const row = within(table).getByRole('row', { name: /^Two and a Half Men \(2003\)/ });
    // Both titles side by side: the deleted one, and the live copy with its administration page.
    expect(within(row).getByRole('rowheader')).toHaveTextContent('Two and a Half Men (2003)');
    expect(within(row).getByRole('link', { name: 'Two and a Half Men (2003)' })).toHaveAttribute(
      'href',
      `/admin/items/${id(31)}`,
    );
    expect(within(row).getByText('Serie')).toBeVisible();
    expect(within(row).getByText('Serien')).toBeVisible();
    expect(within(row).getByText('28.10.2026')).toBeVisible();
    expect(
      within(row).getByText('2 Fortschritte, 100 Wiedergabe-Ereignisse, 1 Wiedergabe'),
    ).toBeVisible();
    for (const box of within(table).getAllByRole('checkbox')) expect(box).toBeChecked();
    expect(screen.getByText('2 ausgewählt')).toBeVisible();

    // One pair taken out: the question sums the other's user data only.
    await user.click(
      screen.getByRole('checkbox', { name: 'Silo auswählen (Serien, gelöscht am 28.10.2026)' }),
    );
    expect(screen.getByText('1 ausgewählt')).toBeVisible();
    const trigger = screen.getByRole('button', { name: 'Umhängen' });
    await user.click(trigger);
    const dialog = await screen.findByRole('alertdialog', { name: 'Benutzerdaten umhängen?' });
    expect(
      within(dialog).getByText(
        '1 Kopie geht in ihrem gelöschten Titel auf. Der alte Titel kommt zurück und behält seine Adresse. Es bleiben erhalten: 2 Fortschritte, 100 Wiedergabe-Ereignisse, 1 Wiedergabe. Die Kopie wird endgültig entfernt.',
      ),
    ).toBeVisible();
    await user.click(within(dialog).getByRole('button', { name: 'Umhängen' }));
    await waitFor(() =>
      expect(
        calls.find((call) => call.method === 'POST' && call.path === '/admin/maintenance/relink')
          ?.body,
      ).toEqual({
        pairs: [{ deletedId: id(21), liveId: id(31) }],
      }),
    );
    await waitFor(() => expect(trigger).toHaveAttribute('data-pending'));

    relinkPairs = { total: 1, pairs: [initialPairs.pairs[1] as RelinkPair] };
    finish(
      json({ relinked: [{ keptId: id(21), removedId: id(31), merged: 3, moved: 1 }], skipped: 0 }),
    );
    const outcome = await within(panel()).findByText('Umgehängt: 1 Titel.');
    expect(outcome.closest('[role="status"]')).not.toBeNull();
    // The list reloads; the pair taken out stays out (M3), so the button is off and the h1 takes
    // the focus.
    await waitFor(() =>
      expect(
        screen.queryByRole('link', { name: 'Two and a Half Men (2003)' }),
      ).not.toBeInTheDocument(),
    );
    expect(
      screen.getByRole('checkbox', { name: 'Silo auswählen (Serien, gelöscht am 28.10.2026)' }),
    ).not.toBeChecked();
    expect(screen.getByRole('button', { name: 'Umhängen' })).toBeDisabled();
    await waitFor(() => expect(title()).toHaveFocus());
  });

  it('orphaned user data: «Auswahl aufheben» turns the button off; skipped pairs are explained', async () => {
    const user = userEvent.setup();
    const { calls } = setup({
      'POST /admin/maintenance/relink': () => {
        relinkPairs = { total: 0, pairs: [] };
        return json({
          relinked: [{ keptId: id(21), removedId: id(31), merged: 1, moved: 0 }],
          skipped: 2,
        });
      },
    });
    renderApp('/admin/maintenance?tab=relink');
    await screen.findByRole('table', { name: 'Verwaiste Benutzerdaten' });
    await user.click(screen.getByRole('button', { name: 'Auswahl aufheben' }));
    expect(screen.getByRole('button', { name: 'Umhängen' })).toBeDisabled();
    await user.click(screen.getByRole('button', { name: 'Alle auswählen' }));
    await user.click(screen.getByRole('button', { name: 'Umhängen' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Benutzerdaten umhängen?' });
    expect(
      within(dialog).getByText(
        /^2 Kopien gehen .* 3 Fortschritte, 140 Wiedergabe-Ereignisse, 1 Wiedergabe\. Die Kopien werden endgültig entfernt\.$/,
      ),
    ).toBeVisible();
    await user.click(within(dialog).getByRole('button', { name: 'Umhängen' }));
    expect(
      await screen.findByText(
        'Umgehängt: 1 Titel. 2 übersprungen: Die gelöschten Titel sind schon zurück. Die übrigen Kopien stehen unter «Duplikate».',
      ),
    ).toBeVisible();
    expect(
      calls.find((call) => call.method === 'POST' && call.path === '/admin/maintenance/relink')
        ?.body,
    ).toEqual({
      pairs: [
        { deletedId: id(21), liveId: id(31) },
        { deletedId: id(22), liveId: id(32) },
      ],
    });
    // Nothing left: the tab says so, and the h1 takes the focus.
    expect(await screen.findByText('Keine verwaisten Benutzerdaten.')).toBeVisible();
    expect(screen.queryByRole('table')).not.toBeInTheDocument();
    await waitFor(() => expect(title()).toHaveFocus());
  });

  it('orphaned user data: two copies of one deleted title are marked, counted once and told apart', async () => {
    const user = userEvent.setup();
    const heat = (live: number) =>
      pair(1, {
        liveId: id(30 + live),
        libraryId: 1,
        libraryName: 'Filme',
        kind: 'movie',
        title: 'Heat',
        year: 1995,
        deletedTitle: 'Heat',
        deletedYear: 1995,
        userData: { ...noUserData, progress: 2, topPicks: 1 },
      });
    relinkPairs = {
      total: 3,
      pairs: [heat(1), heat(2), pair(3, { title: 'Dark', deletedTitle: 'Dark' })],
    };
    setup({
      'POST /admin/maintenance/relink': () => {
        relinkPairs = { total: 0, pairs: [] };
        return json({
          relinked: [
            { keptId: id(21), removedId: id(31), merged: 1, moved: 0 },
            { keptId: id(23), removedId: id(33), merged: 1, moved: 0 },
          ],
          skipped: 1,
        });
      },
    });
    renderApp('/admin/maintenance?tab=relink');
    const table = await screen.findByRole('table', { name: 'Verwaiste Benutzerdaten' });
    expect(within(table).getAllByText('2 Kopien')).toHaveLength(2);
    const first = screen.getByRole('checkbox', {
      name: 'Heat (1995) auswählen (Filme, gelöscht am 28.10.2026, Kopie 1 von 2)',
    });
    const second = screen.getByRole('checkbox', {
      name: 'Heat (1995) auswählen (Filme, gelöscht am 28.10.2026, Kopie 2 von 2)',
    });

    // Only the title without user data: the question says so instead of listing nothing.
    await user.click(first);
    await user.click(second);
    await user.click(screen.getByRole('button', { name: 'Umhängen' }));
    let dialog = await screen.findByRole('alertdialog', { name: 'Benutzerdaten umhängen?' });
    expect(
      within(dialog).getByText(
        '1 Kopie geht in ihrem gelöschten Titel auf. Der alte Titel kommt zurück und behält seine Adresse. Am gelöschten Titel hängen keine Benutzerdaten. Die Kopie wird endgültig entfernt.',
      ),
    ).toBeVisible();
    await user.click(within(dialog).getByRole('button', { name: 'Abbrechen' }));
    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument());

    // Both copies: the deleted title and its user data count once, the second copy is skipped.
    await user.click(first);
    await user.click(second);
    await user.click(screen.getByRole('button', { name: 'Umhängen' }));
    dialog = await screen.findByRole('alertdialog', { name: 'Benutzerdaten umhängen?' });
    expect(
      within(dialog).getByText(
        '2 Kopien gehen in ihren gelöschten Titeln auf. Die alten Titel kommen zurück und behalten ihre Adressen. Es bleiben erhalten: 2 Fortschritte, 1 Top-Pick. Die Kopien werden endgültig entfernt. Eine weitere Kopie desselben Titels wird übersprungen und steht danach unter «Duplikate».',
      ),
    ).toBeVisible();
    await user.click(within(dialog).getByRole('button', { name: 'Umhängen' }));
    expect(
      await screen.findByText(
        'Umgehängt: 2 Titel. 1 übersprungen: Der gelöschte Titel ist schon zurück. Die übrige Kopie steht unter «Duplikate».',
      ),
    ).toBeVisible();
  });

  it('orphaned user data: a stale list (422) reloads and says so instead of pointing at fields', async () => {
    const user = userEvent.setup();
    const { calls } = setup({
      'POST /admin/maintenance/relink': () => {
        relinkPairs = { total: 1, pairs: [initialPairs.pairs[1] as RelinkPair] };
        return problem(422, 'validation_failed', {
          errors: [{ field: 'pairs[0]', code: 'invalid_value' }],
        });
      },
    });
    const lists = () =>
      calls.filter((call) => call.method === 'GET' && call.path === '/admin/maintenance/relink')
        .length;
    renderApp('/admin/maintenance?tab=relink');
    await screen.findByRole('table', { name: 'Verwaiste Benutzerdaten' });
    const before = lists();
    await user.click(screen.getByRole('button', { name: 'Umhängen' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Benutzerdaten umhängen?' });
    await user.click(within(dialog).getByRole('button', { name: 'Umhängen' }));
    const outcome = await within(panel()).findByText(
      'Die Liste hat sich geändert und wurde neu geladen.',
    );
    expect(outcome.closest('[role="status"]')).not.toBeNull();
    await waitFor(() => expect(lists()).toBeGreaterThan(before));
    await waitFor(() =>
      expect(
        screen.queryByRole('link', { name: 'Two and a Half Men (2003)' }),
      ).not.toBeInTheDocument(),
    );
    // The reloaded pair is selected and can go again.
    expect(screen.getByRole('button', { name: 'Umhängen' })).toBeEnabled();
  });

  it('orphaned user data: a scan after a merge reports the partial run; the focus stays on the button', async () => {
    const user = userEvent.setup();
    relinkPairs = {
      total: 3,
      pairs: [...initialPairs.pairs, pair(3, { title: 'Dark', deletedTitle: 'Dark' })],
    };
    setup({
      'POST /admin/maintenance/relink': () => {
        relinkPairs = { total: 2, pairs: relinkPairs.pairs.slice(1) };
        return json({
          relinked: [{ keptId: id(21), removedId: id(31), merged: 1, moved: 0 }],
          skipped: 0,
          stoppedBy: 'scan_running',
        });
      },
    });
    renderApp('/admin/maintenance?tab=relink');
    await screen.findByRole('table', { name: 'Verwaiste Benutzerdaten' });
    const trigger = screen.getByRole('button', { name: 'Umhängen' });
    await user.click(trigger);
    const dialog = await screen.findByRole('alertdialog', { name: 'Benutzerdaten umhängen?' });
    await user.click(within(dialog).getByRole('button', { name: 'Umhängen' }));
    expect(
      await within(panel()).findByText(
        '1 umgehängt. Ein Scan läuft gerade, der Rest folgt nach dem Scan.',
      ),
    ).toBeVisible();
    await waitFor(() =>
      expect(
        screen.queryByRole('link', { name: 'Two and a Half Men (2003)' }),
      ).not.toBeInTheDocument(),
    );
    // Rows remain and the button is still on: the focus stays where the dialog left it (m1).
    expect(screen.getByText('2 ausgewählt')).toBeVisible();
    await waitFor(() => expect(trigger).toHaveFocus());
    expect(title()).not.toHaveFocus();
  });

  it('orphaned user data: a refused relink shows the problem and reloads; more pairs than shown are counted', async () => {
    const user = userEvent.setup();
    relinkPairs = { ...initialPairs, total: 250 };
    const { calls } = setup({
      'POST /admin/maintenance/relink': () => problem(409, 'scan_running'),
    });
    renderApp('/admin/maintenance?tab=relink');
    expect(await screen.findByText('2 von 250')).toBeVisible();
    const before = calls.filter(
      (call) => call.method === 'GET' && call.path === '/admin/maintenance/relink',
    ).length;
    await user.click(screen.getByRole('button', { name: 'Umhängen' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Benutzerdaten umhängen?' });
    await user.click(within(dialog).getByRole('button', { name: 'Umhängen' }));
    expect(
      await within(panel()).findByText(/^Ein Scan dieser Bibliothek läuft gerade\./),
    ).toBeVisible();
    await waitFor(() =>
      expect(
        calls.filter((call) => call.method === 'GET' && call.path === '/admin/maintenance/relink')
          .length,
      ).toBeGreaterThan(before),
    );
    // The pairs stay.
    expect(screen.getByRole('link', { name: 'Silo' })).toBeVisible();
  });

  it('loads further pages with "Mehr laden" per tab; the new entries join their sections', async () => {
    const user = userEvent.setup();
    lists.orphaned = Array.from({ length: 60 }, (_, index) =>
      issue('orphaned', 10 + index, { libraryId: index < 55 ? 1 : 2 }),
    );
    setup();
    renderApp('/admin/maintenance');
    await user.click(await screen.findByRole('tab', { name: /^Verwaist \d/ }));
    expect(await screen.findByText('50 von 60')).toBeVisible();
    expect(screen.getByRole('button', { name: 'Filme 50 Einträge' })).toBeVisible();
    expect(screen.queryByRole('button', { name: /^Serien/ })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Mehr laden' }));
    expect(await screen.findByText('60 von 60')).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Mehr laden' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Filme 55 Einträge' })).toBeVisible();
    expect(screen.getByRole('button', { name: 'Serien 5 Einträge' })).toBeVisible();

    // Every tab counts its own: another tab shows its own «X von Y».
    await user.click(screen.getByRole('tab', { name: /^Altersfreigabe fehlt/ }));
    expect(await screen.findByText('2 von 2')).toBeVisible();
    await user.click(screen.getByRole('tab', { name: /^Verwaist \d/ }));
    expect(await screen.findByText('60 von 60')).toBeVisible();
  });

  it('keeps the selection and the outcome of a tab when another tab was open (S-17)', async () => {
    const user = userEvent.setup();
    setup();
    renderApp('/admin/maintenance?tab=unrated');
    await user.click(
      await screen.findByRole('checkbox', { name: 'Slouchcat Test Unrated auswählen' }),
    );
    await user.click(screen.getByRole('tab', { name: /^Verwaist \d/ }));
    await screen.findByRole('link', { name: 'Titel 5' });
    expect(screen.getByText('0 ausgewählt')).toBeVisible();
    await user.click(screen.getByRole('tab', { name: /^Altersfreigabe fehlt/ }));
    expect(
      await screen.findByRole('checkbox', { name: 'Slouchcat Test Unrated auswählen' }),
    ).toBeChecked();
    expect(within(panel()).getAllByText('1 ausgewählt')[0]).toBeVisible();
  });

  it('the tab lives in the address; an unknown one shows the first tab and is dropped', async () => {
    const user = userEvent.setup();
    setup();
    const { router } = renderApp('/admin/maintenance?tab=nonsense');
    const tabs = await screen.findByRole('tablist', { name: 'Problemarten' });
    expect(within(tabs).getByRole('tab', { selected: true })).toHaveAccessibleName(
      /^Nicht zugeordnet/,
    );
    await waitFor(() => expect(router.state.location.search).toEqual({}));

    await user.click(within(tabs).getByRole('tab', { name: /^Duplikate/ }));
    await waitFor(() => expect(router.state.location.search).toEqual({ tab: 'duplicates' }));
    await user.click(within(tabs).getByRole('tab', { name: /^Nicht zugeordnet/ }));
    await waitFor(() => expect(router.state.location.search).toEqual({}));
  });

  it('a tab without issues says so', async () => {
    setup();
    renderApp('/admin/maintenance?tab=no_images');
    expect(await screen.findByText('Nichts zu tun.')).toBeVisible();
    expect(screen.queryByRole('table')).not.toBeInTheDocument();
  });

  it('is reserved for the primary profile of the main account', async () => {
    const { calls } = mockApi({
      'GET /auth/session': () =>
        json({ ...session, profile: { ...session.profile, id: 2, isPrimary: false } }),
    });
    renderApp('/admin/maintenance');
    expect(await screen.findByText('Nur für Administratoren')).toBeVisible();
    expect(calls.some((call) => call.path.startsWith('/admin/maintenance'))).toBe(false);
  });
});
