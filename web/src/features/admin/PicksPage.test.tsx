import type { AdminItem, AdminItemHit, AdminPick } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ids, profiles } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, signedIn } from '@/test/api';

const heatPick: AdminPick = {
  id: 11,
  itemId: ids.heat,
  kind: 'movie',
  title: 'Heat',
  year: 1995,
  position: 0,
  note: 'Ein Klassiker',
  createdAt: '2026-10-01T10:00:00Z',
};
const arrivalPick: AdminPick = {
  id: 12,
  itemId: ids.arrival,
  kind: 'movie',
  title: 'Arrival',
  year: 2016,
  profileId: 2,
  profileName: 'Lena',
  position: 1,
  startsAt: '2020-01-01T10:00:00Z',
  endsAt: '2020-02-01T10:00:00Z',
  createdAt: '2026-10-02T10:00:00Z',
};
const alienPick: AdminPick = {
  id: 13,
  itemId: ids.alien,
  kind: 'movie',
  title: 'Alien',
  year: 1979,
  position: 2,
  createdAt: '2026-10-03T10:00:00Z',
};
const alienAdmin: AdminItem = {
  id: ids.alien,
  kind: 'movie',
  title: 'Alien',
  year: 1979,
  libraryId: 1,
  metadataState: 'matched',
  lockedFields: [],
  trailerState: 'none',
  trailers: [],
  mediaFiles: [],
  children: { seasons: 0, episodes: 0, extras: 0 },
};
const alienHit: AdminItemHit = {
  id: ids.alien,
  kind: 'movie',
  title: 'Alien',
  year: 1979,
  libraryId: 1,
};

const routes = {
  ...signedIn,
  'GET /profiles': () => json(profiles),
};

/** The query string of the last admin search the page sent. */
function lastSearch(fetchMock: ReturnType<typeof mockApi>['fetchMock']): URLSearchParams | null {
  const urls = fetchMock.mock.calls
    .map(([request]) => new URL(request.url))
    .filter((url) => url.pathname.endsWith('/admin/items/search'));
  return urls.at(-1)?.searchParams ?? null;
}

describe('admin picks page (M7, E-2)', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('lists the picks in carousel order with target, note and time window', async () => {
    mockApi({ ...routes, 'GET /admin/picks': () => json([heatPick, arrivalPick]) });
    renderApp('/admin/picks');

    const table = await screen.findByRole('table', {
      name: 'Empfehlungen in der Reihenfolge des Karussells',
    });
    const rows = within(table).getAllByRole('row');
    expect(rows).toHaveLength(3);
    expect(within(rows[1]!).getByRole('rowheader')).toHaveTextContent('Heat');
    expect(within(rows[1]!).getByText('Film · 1995')).toBeVisible();
    expect(within(rows[1]!).getByText('Alle')).toBeVisible();
    expect(within(rows[1]!).getByText('Ein Klassiker')).toBeVisible();
    expect(within(rows[1]!).getByText('Immer')).toBeVisible();
    expect(within(rows[2]!).getByText('Lena')).toBeVisible();
    expect(within(rows[2]!).getByText(/ bis /)).toBeVisible();
    expect(within(rows[2]!).getByText('Abgelaufen')).toBeVisible();
    // The ends of the list cannot move further.
    expect(within(rows[1]!).getByRole('button', { name: 'Heat nach oben' })).toBeDisabled();
    expect(within(rows[2]!).getByRole('button', { name: 'Arrival nach unten' })).toBeDisabled();
    expect(
      screen
        .getByRole('navigation', { name: 'Hauptnavigation' })
        .querySelector('[aria-current="page"]'),
    ).toHaveTextContent("Cat's Empfehlungen");
  });

  it('"Nach unten" sends the whole new order; the focus stays with the moved title', async () => {
    let list = [heatPick, arrivalPick];
    const { calls } = mockApi({
      ...routes,
      'GET /admin/picks': () => json(list),
      'PUT /admin/picks/order': () => {
        list = [arrivalPick, heatPick];
        return new Response(null, { status: 204 });
      },
    });
    renderApp('/admin/picks');
    const user = userEvent.setup();

    const down = await screen.findByRole('button', { name: 'Heat nach unten' });
    // The status region is there before its first message, so that one is announced as well.
    const live = screen.getAllByRole('status').find((element) => element.textContent === '');
    expect(live).toBeDefined();
    await user.click(down);
    expect(calls.find((call) => call.method === 'PUT')?.body).toEqual({ ids: [12, 11] });
    expect(await screen.findByText('Heat steht jetzt an Stelle 2.')).toBeVisible();
    expect(live).toHaveTextContent('Heat steht jetzt an Stelle 2.');
    expect(within(live!).queryByRole('status')).not.toBeInTheDocument();
    const rows = within(screen.getByRole('table')).getAllByRole('row');
    expect(within(rows[1]!).getByRole('rowheader')).toHaveTextContent('Arrival');
    // "Nach unten" is disabled at the end: the focus moves to "Nach oben" of the same title.
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Heat nach oben' })).toHaveFocus(),
    );
  });

  it('queues quick moves: one PUT after the other, the later order stays', async () => {
    let list = [heatPick, arrivalPick, alienPick];
    let answerFirst: () => void = () => undefined;
    const firstAnswered = new Promise<void>((resolve) => {
      answerFirst = resolve;
    });
    const { calls } = mockApi({
      ...routes,
      'GET /admin/picks': () => json(list),
      'PUT /admin/picks/order': async (call) => {
        const order = (call.body as { ids: number[] }).ids;
        if (calls.filter((entry) => entry.method === 'PUT').length === 1) await firstAnswered;
        list = order.map((id) =>
          [heatPick, arrivalPick, alienPick].find((pick) => pick.id === id)!,
        );
        return new Response(null, { status: 204 });
      },
    });
    renderApp('/admin/picks');
    const user = userEvent.setup();
    const titles = () =>
      within(screen.getByRole('table'))
        .getAllByRole('rowheader')
        .map((cell) => cell.textContent);

    await user.click(await screen.findByRole('button', { name: 'Alien nach oben' }));
    await user.click(screen.getByRole('button', { name: 'Alien nach oben' }));
    // Both moves show at once; the second PUT waits for the first.
    expect(titles()).toEqual([
      expect.stringContaining('Alien'),
      expect.stringContaining('Heat'),
      expect.stringContaining('Arrival'),
    ]);
    expect(calls.filter((call) => call.method === 'PUT')).toHaveLength(1);

    answerFirst();
    await waitFor(() => expect(calls.filter((call) => call.method === 'PUT')).toHaveLength(2));
    expect(calls.filter((call) => call.method === 'PUT').map((call) => call.body)).toEqual([
      { ids: [11, 13, 12] },
      { ids: [13, 11, 12] },
    ]);
    await waitFor(() =>
      expect(calls.filter((call) => call.path === '/admin/picks').length).toBeGreaterThan(1),
    );
    expect(titles()).toEqual([
      expect.stringContaining('Alien'),
      expect.stringContaining('Heat'),
      expect.stringContaining('Arrival'),
    ]);
  });

  it('puts the old order back when the server refuses a move', async () => {
    let refuse: () => void = () => undefined;
    const refused = new Promise<void>((resolve) => {
      refuse = resolve;
    });
    mockApi({
      ...routes,
      'GET /admin/picks': () => json([heatPick, arrivalPick]),
      'PUT /admin/picks/order': async () => {
        await refused;
        return problem(500, 'internal');
      },
    });
    renderApp('/admin/picks');
    const user = userEvent.setup();
    const titles = () =>
      within(screen.getByRole('table'))
        .getAllByRole('rowheader')
        .map((cell) => cell.textContent);

    await user.click(await screen.findByRole('button', { name: 'Heat nach unten' }));
    // The new order shows at once; at the end the focus is on "Nach oben".
    await waitFor(() =>
      expect(titles()).toEqual([
        expect.stringContaining('Arrival'),
        expect.stringContaining('Heat'),
      ]),
    );
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Heat nach oben' })).toHaveFocus(),
    );

    refuse();
    expect(await screen.findByRole('alert')).toBeVisible();
    expect(titles()).toEqual([expect.stringContaining('Heat'), expect.stringContaining('Arrival')]);
    // Back in place, the pressed button has the focus again.
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Heat nach unten' })).toHaveFocus(),
    );
  });

  it('adds a title found by the search for a single profile', async () => {
    let list: AdminPick[] = [];
    const { calls, fetchMock } = mockApi({
      ...routes,
      'GET /admin/picks': () => json(list),
      'GET /admin/items/search': () => json([alienHit]),
      'POST /admin/picks': () => {
        list = [{ ...heatPick, id: 13, itemId: ids.alien, title: 'Alien', profileId: 2 }];
        return json(list, 201);
      },
    });
    renderApp('/admin/picks');
    const user = userEvent.setup();

    expect(await screen.findByText(/Noch keine Empfehlung/)).toBeVisible();
    await user.click(screen.getByRole('button', { name: 'Hinzufügen' }));
    const dialog = await screen.findByRole('dialog', { name: 'Empfehlung hinzufügen' });
    const search = within(dialog).getByRole('searchbox', { name: 'Titel suchen' });
    await waitFor(() => expect(search).toHaveFocus());

    // Without a title nothing is sent.
    await user.click(within(dialog).getByRole('button', { name: 'Hinzufügen' }));
    expect(within(dialog).getByText('Wähle zuerst einen Titel.')).toBeVisible();

    // Enter in the search field does not submit the form.
    await user.type(search, 'Al{Enter}');
    await user.click(await within(dialog).findByRole('button', { name: 'Alien wählen' }));
    expect(lastSearch(fetchMock)?.get('q')).toBe('Al');
    expect(within(dialog).getByText(/^Gewählt: Alien/)).toBeVisible();

    // A single profile replaces "Alle Profile".
    const all = within(dialog).getByRole('checkbox', { name: 'Alle Profile' });
    expect(all).toBeChecked();
    await user.click(within(dialog).getByRole('checkbox', { name: 'Lena' }));
    expect(all).not.toBeChecked();
    await user.type(within(dialog).getByRole('textbox', { name: 'Notiz' }), 'Für Lena');
    await user.click(within(dialog).getByRole('button', { name: 'Hinzufügen' }));

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(calls.filter((call) => call.method === 'POST').map((call) => call.body)).toEqual([
      { itemId: ids.alien, profileIds: [2], note: 'Für Lena' },
    ]);
    expect(screen.getByText("Alien ist in Cat's Empfehlungen.")).toBeVisible();
    expect(await screen.findByRole('rowheader', { name: /Alien/ })).toBeVisible();
    await waitFor(() => expect(screen.getByRole('button', { name: 'Hinzufügen' })).toHaveFocus());
  });

  it('?add= opens the dialog with the title of the link; a pick for everyone has no profiles', async () => {
    const { calls, router } = {
      ...mockApi({
        ...routes,
        'GET /admin/picks': () => json([]),
        // The admin view of the title: the active profile's visibility does not count here.
        [`GET /admin/items/${ids.alien}`]: () => json(alienAdmin),
        'POST /admin/picks': () => json([{ ...heatPick, itemId: ids.alien, title: 'Alien' }], 201),
      }),
      ...renderApp(`/admin/picks?add=${ids.alien}`),
    };
    const user = userEvent.setup();

    const dialog = await screen.findByRole('dialog', { name: 'Empfehlung hinzufügen' });
    expect(await within(dialog).findByText(/^Gewählt: Alien/)).toBeVisible();
    expect(calls.some((call) => call.path === `/items/${ids.alien}`)).toBe(false);
    await user.click(within(dialog).getByRole('button', { name: 'Hinzufügen' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(calls.find((call) => call.method === 'POST')?.body).toEqual({ itemId: ids.alien });
    // The link is used once: the parameter is gone.
    await waitFor(() => expect(router.state.location.search).toEqual({}));
  });

  it('checks the time window in the browser and shows the server problem at the field', async () => {
    const { calls } = mockApi({
      ...routes,
      'GET /admin/picks': () => json([heatPick]),
      'PATCH /admin/picks/11': () =>
        problem(422, 'validation_failed', { errors: [{ field: 'note', code: 'too_long' }] }),
    });
    renderApp('/admin/picks');
    const user = userEvent.setup();

    await user.click(await screen.findByRole('button', { name: 'Heat bearbeiten' }));
    const dialog = await screen.findByRole('dialog', { name: 'Heat bearbeiten' });
    const note = within(dialog).getByLabelText('Notiz');
    expect(note).toHaveValue('Ein Klassiker');
    const from = within(dialog).getByLabelText('Sichtbar ab');
    const to = within(dialog).getByLabelText('Sichtbar bis');
    await user.type(from, '2026-11-02T10:00');
    await user.type(to, '2026-11-01T10:00');
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));
    expect(to).toHaveAccessibleDescription(/Das Ende muss nach dem Beginn liegen/);
    expect(calls.some((call) => call.method === 'PATCH')).toBe(false);

    await user.clear(to);
    await user.type(to, '2026-11-03T10:00');
    await user.clear(note);
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(note).toHaveAccessibleDescription(/Zu lang/));
    expect(calls.find((call) => call.method === 'PATCH')?.body).toEqual({
      note: null,
      startsAt: new Date('2026-11-02T10:00').toISOString(),
      endsAt: new Date('2026-11-03T10:00').toISOString(),
    });
  });

  it('"Entfernen" asks first, removes the pick and moves the focus to the heading', async () => {
    let list = [heatPick];
    const { calls } = mockApi({
      ...routes,
      'GET /admin/picks': () => json(list),
      'DELETE /admin/picks/11': () => {
        list = [];
        return new Response(null, { status: 204 });
      },
    });
    renderApp('/admin/picks');
    const user = userEvent.setup();

    await user.click(await screen.findByRole('button', { name: 'Heat entfernen' }));
    const confirm = await screen.findByRole('alertdialog', {
      name: "Heat aus Cat's Empfehlungen entfernen?",
    });
    await user.click(within(confirm).getByRole('button', { name: 'Entfernen' }));
    expect(await screen.findByText('Heat ist nicht mehr empfohlen.')).toBeVisible();
    expect(calls.some((call) => call.method === 'DELETE')).toBe(true);
    await waitFor(() => expect(screen.queryByRole('table')).not.toBeInTheDocument());
    await waitFor(() =>
      expect(
        screen.getByRole('heading', { level: 1, name: "Cat's Empfehlungen" }).parentElement,
      ).toHaveFocus(),
    );
  });
});
