import type { AdminItemHit, AdminPick } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { alienDetail, ids, profiles } from '@/test/fixtures';
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

    await user.click(await screen.findByRole('button', { name: 'Heat nach unten' }));
    expect(calls.find((call) => call.method === 'PUT')?.body).toEqual({ ids: [12, 11] });
    expect(await screen.findByText('Heat steht jetzt an Stelle 2.')).toBeVisible();
    const rows = within(screen.getByRole('table')).getAllByRole('row');
    expect(within(rows[1]!).getByRole('rowheader')).toHaveTextContent('Arrival');
    // "Nach unten" is disabled at the end: the focus moves to "Nach oben" of the same title.
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Heat nach oben' })).toHaveFocus(),
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
        [`GET /items/${ids.alien}`]: () => json(alienDetail),
        'POST /admin/picks': () => json([{ ...heatPick, itemId: ids.alien, title: 'Alien' }], 201),
      }),
      ...renderApp(`/admin/picks?add=${ids.alien}`),
    };
    const user = userEvent.setup();

    const dialog = await screen.findByRole('dialog', { name: 'Empfehlung hinzufügen' });
    expect(await within(dialog).findByText(/^Gewählt: Alien/)).toBeVisible();
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
