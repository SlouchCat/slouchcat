import type { AdminCollection } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ids } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, signedIn } from '@/test/api';

const manualId = '01JB8Z6Q4M7N9P2R3S5T7V9X1A';
const tmdbId = '01JB8Z6Q4M7N9P2R3S5T7V9X2B';

const manual: AdminCollection = {
  id: manualId,
  source: 'manual',
  name: 'Filmabend',
  itemCount: 2,
  items: [],
};
const tmdb: AdminCollection = {
  id: tmdbId,
  source: 'tmdb',
  name: 'Alien Collection',
  nameDe: 'Alien Filmreihe',
  tmdbId: 8091,
  itemCount: 3,
  items: [],
};
const members: AdminCollection = {
  ...manual,
  items: [
    { itemId: ids.heat, kind: 'movie', title: 'Heat', year: 1995, position: 0 },
    { itemId: ids.arrival, kind: 'movie', title: 'Arrival', year: 2016, position: 1 },
  ],
};

describe('admin collections page (M7, E-6)', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('lists the collections with source and count; TMDB ones cannot be changed', async () => {
    mockApi({ ...signedIn, 'GET /admin/collections': () => json([tmdb, manual]) });
    renderApp('/admin/collections');

    const table = await screen.findByRole('table', { name: 'Sammlungen' });
    const rows = within(table).getAllByRole('row');
    expect(rows).toHaveLength(3);
    expect(within(rows[1]!).getByRole('rowheader')).toHaveTextContent('Alien Collection');
    expect(within(rows[1]!).getByText('Alien Filmreihe')).toBeVisible();
    expect(within(rows[1]!).getByText('TMDB')).toBeVisible();
    expect(within(rows[1]!).getByText('3')).toBeVisible();
    for (const name of [
      'Mitglieder von Alien Collection',
      'Alien Collection umbenennen',
      'Sammlung Alien Collection löschen',
    ]) {
      expect(within(rows[1]!).getByRole('button', { name })).toBeDisabled();
    }
    expect(within(rows[2]!).getByText('Manuell')).toBeVisible();
    expect(within(rows[2]!).getByRole('button', { name: 'Filmabend umbenennen' })).toBeEnabled();
    expect(
      screen
        .getByRole('navigation', { name: 'Hauptnavigation' })
        .querySelector('[aria-current="page"]'),
    ).toHaveTextContent('Sammlungen');
  });

  it('creates and renames a manual collection; a server problem shows at the name', async () => {
    let list: AdminCollection[] = [];
    let fail = true;
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/collections': () => json(list),
      'POST /admin/collections': () => {
        if (fail) {
          return problem(422, 'validation_failed', {
            errors: [{ field: 'name', code: 'too_long' }],
          });
        }
        list = [{ ...manual, itemCount: 0 }];
        return json(list[0], 201);
      },
      [`PATCH /admin/collections/${manualId}`]: () => {
        list = [{ ...manual, name: 'Kinoabend' }];
        return json(list[0]);
      },
    });
    renderApp('/admin/collections');
    const user = userEvent.setup();

    expect(await screen.findByText('Noch keine Sammlung.')).toBeVisible();
    await user.click(screen.getByRole('button', { name: 'Sammlung anlegen' }));
    let dialog = await screen.findByRole('dialog', { name: 'Sammlung anlegen' });
    const name = within(dialog).getByRole('textbox', { name: 'Name' });
    await waitFor(() => expect(name).toHaveFocus());
    await user.type(name, 'Filmabend');
    await user.click(within(dialog).getByRole('button', { name: 'Anlegen' }));
    await waitFor(() => expect(name).toHaveAccessibleDescription(/Zu lang/));

    fail = false;
    await user.clear(name);
    await user.type(name, 'Filmabend{Enter}');
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(calls.filter((call) => call.method === 'POST').at(-1)?.body).toEqual({
      name: 'Filmabend',
    });
    expect(screen.getByText('Die Sammlung Filmabend ist angelegt.')).toBeVisible();
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Sammlung anlegen' })).toHaveFocus(),
    );

    await user.click(await screen.findByRole('button', { name: 'Filmabend umbenennen' }));
    dialog = await screen.findByRole('dialog', { name: 'Filmabend umbenennen' });
    const renamed = within(dialog).getByRole('textbox', { name: 'Name' });
    expect(renamed).toHaveValue('Filmabend');
    await user.clear(renamed);
    await user.type(renamed, 'Kinoabend');
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(calls.find((call) => call.method === 'PATCH')?.body).toEqual({ name: 'Kinoabend' });
    expect(screen.getByText('Die Sammlung heisst jetzt Kinoabend.')).toBeVisible();
    expect(await screen.findByRole('rowheader', { name: 'Kinoabend' })).toBeVisible();
  });

  it('a rename marks the viewer pages that show collections as stale', async () => {
    mockApi({
      ...signedIn,
      'GET /admin/collections': () => json([manual]),
      [`PATCH /admin/collections/${manualId}`]: () => json({ ...manual, name: 'Kinoabend' }),
    });
    const { queryClient } = renderApp('/admin/collections');
    const viewer = [
      ['collection', manualId],
      ['item', ids.heat],
      ['search', { query: 'Fi', offset: 0, limit: 50 }],
      ['home'],
    ];
    for (const queryKey of [...viewer, ['genres']]) queryClient.setQueryData(queryKey, {});
    const user = userEvent.setup();

    await user.click(await screen.findByRole('button', { name: 'Filmabend umbenennen' }));
    const dialog = await screen.findByRole('dialog', { name: 'Filmabend umbenennen' });
    const name = within(dialog).getByRole('textbox', { name: 'Name' });
    await user.clear(name);
    await user.type(name, 'Kinoabend{Enter}');
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());

    await waitFor(() => {
      for (const queryKey of viewer) {
        expect(queryClient.getQueryState(queryKey)?.isInvalidated, JSON.stringify(queryKey)).toBe(
          true,
        );
      }
    });
    expect(queryClient.getQueryState(['genres'])?.isInvalidated).toBe(false);
  });

  it('edits the members: add from the search, reorder, remove, save in this order', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/collections': () => json([manual]),
      [`GET /admin/collections/${manualId}`]: () => json(members),
      'GET /admin/items/search': () =>
        json([
          { id: ids.heat, kind: 'movie', title: 'Heat', year: 1995, libraryId: 1 },
          { id: ids.alien, kind: 'movie', title: 'Alien', year: 1979, libraryId: 1 },
          {
            id: '01JB8Z6Q4M7N9P2R3S5T7V9X3C',
            kind: 'episode',
            title: 'Pilot',
            libraryId: 2,
            parentTitle: 'Eine Serie',
          },
        ]),
      [`PUT /admin/collections/${manualId}/items`]: () => json(members),
    });
    renderApp('/admin/collections');
    const user = userEvent.setup();

    await user.click(await screen.findByRole('button', { name: 'Mitglieder von Filmabend' }));
    const dialog = await screen.findByRole('dialog', { name: 'Mitglieder von Filmabend' });
    const list = await within(dialog).findByRole('list', { name: 'Titel der Sammlung' });
    expect(
      within(list)
        .getAllByRole('listitem')
        .map((item) => item.textContent),
    ).toEqual([expect.stringContaining('Heat'), expect.stringContaining('Arrival')]);

    // Members already in the list and episodes are not offered.
    await user.type(within(dialog).getByRole('searchbox', { name: 'Titel suchen' }), 'al');
    const hits = await within(dialog).findByRole('list', { name: 'Gefundene Titel' });
    expect(within(hits).getAllByRole('listitem')).toHaveLength(1);
    await user.click(within(hits).getByRole('button', { name: 'Alien hinzufügen' }));
    await waitFor(() => expect(within(list).getAllByRole('listitem')).toHaveLength(3));

    // Alien to the top: the focus follows the moved entry.
    await user.click(within(list).getByRole('button', { name: 'Alien nach oben' }));
    await user.click(within(list).getByRole('button', { name: 'Alien nach oben' }));
    await waitFor(() =>
      expect(within(list).getByRole('button', { name: 'Alien nach unten' })).toHaveFocus(),
    );
    await user.click(within(list).getByRole('button', { name: 'Heat entfernen' }));
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(calls.find((call) => call.method === 'PUT')?.body).toEqual({
      itemIds: [ids.alien, ids.arrival],
    });
    expect(screen.getByText('Die Mitglieder von Filmabend sind gespeichert.')).toBeVisible();
  });

  it('offers the search only once the stored members are there, so none are lost', async () => {
    let release: () => void = () => undefined;
    const loaded = new Promise<void>((resolve) => {
      release = resolve;
    });
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/collections': () => json([manual]),
      [`GET /admin/collections/${manualId}`]: async () => {
        await loaded;
        return json(members);
      },
      'GET /admin/items/search': () =>
        json([{ id: ids.alien, kind: 'movie', title: 'Alien', year: 1979, libraryId: 1 }]),
      [`PUT /admin/collections/${manualId}/items`]: () => json(members),
    });
    renderApp('/admin/collections');
    const user = userEvent.setup();

    await user.click(await screen.findByRole('button', { name: 'Mitglieder von Filmabend' }));
    const dialog = await screen.findByRole('dialog', { name: 'Mitglieder von Filmabend' });
    // While the members load there is nothing to pick from.
    expect(within(dialog).queryByRole('searchbox')).not.toBeInTheDocument();
    expect(within(dialog).getByRole('button', { name: 'Speichern' })).toBeDisabled();

    release();
    const search = await within(dialog).findByRole('searchbox', { name: 'Titel suchen' });
    await waitFor(() => expect(search).toHaveFocus());
    await user.type(search, 'al');
    await user.click(await within(dialog).findByRole('button', { name: 'Alien hinzufügen' }));
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(calls.find((call) => call.method === 'PUT')?.body).toEqual({
      itemIds: [ids.heat, ids.arrival, ids.alien],
    });
  });

  it('"Löschen" asks first and moves the focus to the heading', async () => {
    let list = [manual];
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/collections': () => json(list),
      [`DELETE /admin/collections/${manualId}`]: () => {
        list = [];
        return new Response(null, { status: 204 });
      },
    });
    renderApp('/admin/collections');
    const user = userEvent.setup();

    await user.click(await screen.findByRole('button', { name: 'Sammlung Filmabend löschen' }));
    const confirm = await screen.findByRole('alertdialog', { name: 'Sammlung Filmabend löschen?' });
    await user.click(within(confirm).getByRole('button', { name: 'Sammlung löschen' }));
    expect(await screen.findByText('Die Sammlung Filmabend ist gelöscht.')).toBeVisible();
    expect(calls.some((call) => call.method === 'DELETE')).toBe(true);
    await waitFor(() =>
      expect(
        screen.getByRole('heading', { level: 1, name: 'Sammlungen' }).parentElement,
      ).toHaveFocus(),
    );
  });
});
