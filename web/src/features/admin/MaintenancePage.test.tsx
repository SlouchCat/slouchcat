import type { DuplicateGroup, Issue, IssueKind, IssuePage } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, session, signedIn, type Call } from '@/test/api';

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
    issue('unmatched', 2, { itemKind: 'episode', title: 'Episode 1' }),
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

const groups: DuplicateGroup[] = [
  {
    reason: 'file_hash',
    items: [
      {
        itemId: id(8),
        kind: 'movie',
        title: 'Slouchcat Test H264',
        year: 2024,
        files: [{ id: 'mf-a', path: 'H264/a.mp4', sizeBytes: 1_000_000, available: true }],
      },
      {
        itemId: id(9),
        kind: 'movie',
        title: 'Slouchcat Test H264 Copy',
        year: 2024,
        files: [{ id: 'mf-b', path: 'Copy/a.mp4', sizeBytes: 1_000_000, available: true }],
      },
    ],
  },
];

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
    ...extra,
  });
  lastUrl = withQuery(api.fetchMock);
  return api;
}

describe('maintenance page', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    lists = { ...initial };
  });

  it('shows a tab per issue kind with its count and lists the first one', async () => {
    setup();
    renderApp('/admin/maintenance');

    expect(await screen.findByRole('heading', { level: 1, name: 'Pflege' })).toBeVisible();
    const tabs = screen.getByRole('tablist', { name: 'Problemarten' });
    await waitFor(() =>
      expect(within(tabs).getByRole('tab', { name: 'Nicht zugeordnet (2)' })).toBeVisible(),
    );
    expect(within(tabs).getByRole('tab', { name: 'Nicht abspielbar (1)' })).toBeVisible();
    expect(within(tabs).getByRole('tab', { name: 'Ohne Bilder (0)' })).toBeVisible();
    expect(within(tabs).getByRole('tab', { name: 'Duplikate (1)' })).toBeVisible();

    const link = await screen.findByRole('link', { name: 'Slouchcat Test H264 (2024)' });
    expect(link).toHaveAttribute('href', `/admin/items/${id(1)}`);
  });

  it('unplayable: select, remove after confirmation, the list reloads', async () => {
    const user = userEvent.setup();
    const { calls } = setup({
      'POST /admin/maintenance/cleanup': () => {
        lists.unplayable = [];
        return json({ items: 1, files: 1 });
      },
    });
    renderApp('/admin/maintenance');
    await user.click(await screen.findByRole('tab', { name: /Nicht abspielbar/ }));

    const row = await screen.findByRole('link', { name: 'Slouchcat Test Broken' });
    expect(row).toBeVisible();
    expect(screen.getByText('invalid data found when processing input')).toBeVisible();
    const remove = screen.getByRole('button', { name: 'Entfernen' });
    expect(remove).toBeDisabled();
    await user.click(screen.getByRole('checkbox', { name: 'Slouchcat Test Broken auswählen' }));
    expect(screen.getByText('1 ausgewählt')).toBeVisible();
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
  });

  it('unrated: assigns a rating to the selection', async () => {
    const user = userEvent.setup();
    const { calls } = setup({ 'POST /admin/items/rating': () => json({ updated: 2 }) });
    renderApp('/admin/maintenance');
    await user.click(await screen.findByRole('tab', { name: /Ohne Freigabe/ }));
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
    await user.click(await screen.findByRole('tab', { name: /Verwaist/ }));
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
    await user.click(await screen.findByRole('tab', { name: /Freigabe abweichend/ }));
    await screen.findByRole('link', { name: 'Titel 7' });
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
    expect(screen.getByText(/Hier gibt es nichts auszuwählen/)).toBeVisible();
  });

  it('duplicates: pick the title to keep, merge after confirmation', async () => {
    const user = userEvent.setup();
    const { calls } = setup({ 'POST /admin/maintenance/merge': () => json({}) });
    renderApp('/admin/maintenance');
    await user.click(await screen.findByRole('tab', { name: /Duplikate/ }));
    const group = await screen.findByRole('article', { name: 'Gruppe 1' });
    expect(within(group).getByText('Gleicher Dateiinhalt')).toBeVisible();
    expect(within(group).getByText('Copy/a.mp4')).toBeVisible();
    await user.click(within(group).getByRole('radio', { name: '2: Slouchcat Test H264 Copy' }));
    await user.click(within(group).getByRole('button', { name: 'Zusammenführen' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Titel zusammenführen?' });
    await user.click(within(dialog).getByRole('button', { name: 'Zusammenführen' }));
    await waitFor(() =>
      expect(calls.find((call) => call.path === '/admin/maintenance/merge')?.body).toEqual({
        targetId: id(9),
        sourceIds: [id(8)],
      }),
    );
  });

  it('loads further pages with "Mehr laden"', async () => {
    const user = userEvent.setup();
    const many = Array.from({ length: 60 }, (_, index) => issue('orphaned', 10 + index));
    lists.orphaned = many;
    setup();
    renderApp('/admin/maintenance');
    await user.click(await screen.findByRole('tab', { name: /Verwaist/ }));
    expect(await screen.findByText('50 von 60')).toBeVisible();
    await user.click(screen.getByRole('button', { name: 'Mehr laden' }));
    expect(await screen.findByText('60 von 60')).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Mehr laden' })).not.toBeInTheDocument();
  });

  it('is reserved for the primary profile of the main account', async () => {
    const { calls } = mockApi({
      'GET /auth/session': () =>
        json({ ...session, profile: { ...session.profile, id: 2, isPrimary: false } }),
    });
    renderApp('/admin/maintenance');
    expect(await screen.findByText('Nur für das Hauptkonto')).toBeVisible();
    expect(calls.some((call) => call.path.startsWith('/admin/maintenance'))).toBe(false);
  });
});
