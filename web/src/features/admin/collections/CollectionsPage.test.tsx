import type { AdminCollection, AdminCollectionItem, AdminItemHit } from '@slouchcat/api-client';
import { focusManager } from '@tanstack/react-query';
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent, { type UserEvent } from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ids } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, session, signedIn } from '@/test/api';
import { adminShell, renderAdminPage } from '@/test/admin';

// Collections (M7, E-6; concept 12, variant C, Sam-C): filter tabs «Alle», «Manuell», «TMDB»;
// each collection a row that folds out into its members. A manual one is filled from the title
// search, ordered by position (RankSelect) and saved in its row; a TMDB one is read only. The
// members being edited are the page's draft per collection (S-17).

const manualId = '01JB8Z6Q4M7N9P2R3S5T7V9X1A';
const tmdbId = '01JB8Z6Q4M7N9P2R3S5T7V9X2B';
const episodeId = '01JB8Z6Q4M7N9P2R3S5T7V9X3C';

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
const heatPoster = `/api/v1/admin/items/${ids.heat}/images/poster/w200?v=heat`;
const heatItem: AdminCollectionItem = {
  itemId: ids.heat,
  kind: 'movie',
  title: 'Heat',
  year: 1995,
  position: 0,
  posterUrl: heatPoster,
};
const arrivalItem: AdminCollectionItem = {
  itemId: ids.arrival,
  kind: 'movie',
  title: 'Arrival',
  year: 2016,
  position: 1,
};
const alienItem: AdminCollectionItem = {
  itemId: ids.alien,
  kind: 'movie',
  title: 'Alien',
  year: 1979,
  position: 0,
};
const members: AdminCollection = { ...manual, items: [heatItem, arrivalItem] };
const tmdbMembers: AdminCollection = {
  ...tmdb,
  items: [
    { ...alienItem, posterUrl: `/api/v1/admin/items/${ids.alien}/images/poster/w200?v=alien` },
    { ...arrivalItem, position: 1 },
  ],
};
const hits: AdminItemHit[] = [
  { id: ids.heat, kind: 'movie', title: 'Heat', year: 1995, libraryId: 1 },
  { id: ids.alien, kind: 'movie', title: 'Alien', year: 1979, libraryId: 1 },
  { id: episodeId, kind: 'episode', title: 'Pilot', libraryId: 2, parentTitle: 'Eine Serie' },
];

/** The title button of a collection's row, which folds it out and in. */
const toggleOf = (name: string) => {
  const header = screen
    .getAllByRole('rowheader')
    .find((cell) => within(cell).queryByText(name, { exact: true }));
  if (!header) throw new Error(`no row ${name}`);
  return within(header).getByRole('button');
};
const rowOf = (name: string) => toggleOf(name).closest('tr')!;

/** Folds the row of `name` out and returns its members region. */
async function openRow(user: UserEvent, name: string) {
  await screen.findAllByRole('rowheader');
  const toggle = toggleOf(name);
  if (toggle.getAttribute('aria-expanded') === 'false') await user.click(toggle);
  return screen.findByRole('region', { name: `Mitglieder von ${name}` });
}

const memberTitles = (region: HTMLElement) =>
  within(within(region).getByRole('list', { name: 'Titel der Sammlung' }))
    .getAllByRole('listitem')
    .map((item) => item.textContent);

const refocus = () =>
  act(() => {
    focusManager.setFocused(false);
    focusManager.setFocused(true);
  });
/** Gives a refetch the chance it must not take. */
const settle = () => act(() => new Promise((resolve) => window.setTimeout(resolve, 50)));

describe('admin collections page (M7, E-6; concept 12, Sam-C)', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('lists the collections with source and count; a TMDB one shows its members only to read', async () => {
    mockApi({
      ...signedIn,
      ...adminShell,
      'GET /admin/collections': () => json([tmdb, manual]),
      [`GET /admin/collections/${tmdbId}`]: () => json(tmdbMembers),
    });
    const { nav } = await renderAdminPage('/admin/collections');
    const user = userEvent.setup();

    const table = await screen.findByRole('table', { name: 'Sammlungen' });
    const rows = within(table).getAllByRole('row');
    expect(rows).toHaveLength(3);
    expect(within(rows[1]!).getByRole('rowheader')).toHaveTextContent('Alien Collection');
    expect(within(rows[1]!).getByText('Alien Filmreihe')).toBeVisible();
    expect(within(rows[1]!).getByText('TMDB')).toBeVisible();
    expect(within(rows[1]!).getByText('3')).toBeVisible();
    expect(within(rows[1]!).getByText('nicht änderbar')).toBeVisible();
    expect(within(rows[2]!).getByText('Manuell')).toBeVisible();
    expect(within(rows[2]!).queryByText('nicht änderbar')).toBeNull();
    expect(nav.querySelector('[aria-current="page"]')).toHaveTextContent('Sammlungen');
    // The intro is behind the help button (S-2).
    expect(screen.getByText(/entstehen beim Abgleich/)).not.toBeVisible();

    const region = await openRow(user, 'Alien Collection');
    expect(toggleOf('Alien Collection')).toHaveAttribute('aria-expanded', 'true');
    expect(
      within(region).getByText(
        'Diese Sammlung kommt von TMDB: Mitglieder, Name und Löschen sind darum gesperrt.',
      ),
    ).toBeVisible();
    await waitFor(() =>
      expect(memberTitles(region)).toEqual(['AlienFilm · 1979', expect.any(String)]),
    );
    // Nothing to change: no search, no positions, no «Speichern», «Umbenennen» or «Löschen».
    expect(within(region).queryByRole('searchbox')).toBeNull();
    expect(within(region).queryByRole('combobox')).toBeNull();
    expect(within(region).queryAllByRole('button')).toEqual([]);
    expect(screen.queryByRole('button', { name: 'Alien Collection umbenennen' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Sammlung Alien Collection löschen' })).toBeNull();

    // The poster is decoration next to the title; without one, or when it fails, the fallback.
    const list = within(region).getByRole('list', { name: 'Titel der Sammlung' });
    const [alien, arrival] = within(list).getAllByRole('listitem');
    const poster = alien!.querySelector('img');
    expect(poster).toHaveAttribute('src', tmdbMembers.items[0]!.posterUrl);
    expect(poster).toHaveAttribute('alt', '');
    expect(arrival!.querySelector('img')).toBeNull();
    expect(within(arrival!).getAllByText('Arrival')).toHaveLength(2);
    fireEvent.error(poster!);
    expect(alien!.querySelector('img')).toBeNull();
    expect(within(alien!).getAllByText('Alien')).toHaveLength(2);
  });

  it('creates and renames a manual collection; a server problem shows at the name', async () => {
    let list: AdminCollection[] = [];
    let fail = true;
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/collections': () => json(list),
      [`GET /admin/collections/${manualId}`]: () => json({ ...list[0], items: [] }),
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
        list = [{ ...manual, name: 'Kinoabend', itemCount: 0 }];
        return json(list[0]);
      },
    });
    renderApp('/admin/collections');
    const user = userEvent.setup();

    expect(await screen.findByText('Noch keine Sammlung.')).toBeVisible();
    await user.click(screen.getByRole('button', { name: 'Sammlung anlegen' }));
    let dialog = await screen.findByRole('dialog', { name: 'Sammlung anlegen' });
    // O-2: no sentence about a «Mitglieder» button that no longer exists.
    expect(dialog).not.toHaveTextContent('Mitglieder');
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

    // «Umbenennen» is in the row's fold-out.
    const region = await openRow(user, 'Filmabend');
    await user.click(within(region).getByRole('button', { name: 'Filmabend umbenennen' }));
    dialog = await screen.findByRole('dialog', { name: 'Filmabend umbenennen' });
    const renamed = within(dialog).getByRole('textbox', { name: 'Name' });
    expect(renamed).toHaveValue('Filmabend');
    await user.clear(renamed);
    await user.type(renamed, 'Kinoabend');
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(calls.find((call) => call.method === 'PATCH')?.body).toEqual({ name: 'Kinoabend' });
    expect(screen.getByText('Die Sammlung heisst jetzt Kinoabend.')).toBeVisible();
    expect(await screen.findByRole('region', { name: 'Mitglieder von Kinoabend' })).toBeVisible();
    expect(toggleOf('Kinoabend')).toHaveAttribute('aria-expanded', 'true');
  });

  it('a rename marks the viewer pages that show collections as stale', async () => {
    mockApi({
      ...signedIn,
      'GET /admin/collections': () => json([manual]),
      [`GET /admin/collections/${manualId}`]: () => json(members),
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

    const region = await openRow(user, 'Filmabend');
    await user.click(within(region).getByRole('button', { name: 'Filmabend umbenennen' }));
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

  it('edits the members in the row: add from the search, choose a position, remove, save', async () => {
    let stored = members;
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/collections': () => json([{ ...manual, itemCount: stored.items.length }]),
      [`GET /admin/collections/${manualId}`]: () => json(stored),
      'GET /admin/items/search': () => json(hits),
      [`PUT /admin/collections/${manualId}/items`]: () => {
        stored = {
          ...manual,
          items: [
            { ...alienItem, position: 0 },
            { ...arrivalItem, position: 1 },
          ],
        };
        return json(stored);
      },
    });
    renderApp('/admin/collections');
    const user = userEvent.setup();

    const region = await openRow(user, 'Filmabend');
    await waitFor(() =>
      expect(memberTitles(region)).toEqual([
        expect.stringContaining('Heat'),
        expect.stringContaining('Arrival'),
      ]),
    );
    expect(within(region).getByText('2 Titel')).toBeVisible();
    // Heat has a poster, Arrival the fallback with its title.
    const list = within(region).getByRole('list', { name: 'Titel der Sammlung' });
    const [heat, arrival] = within(list).getAllByRole('listitem');
    expect(heat!.querySelector('img')).toHaveAttribute('src', heatPoster);
    expect(arrival!.querySelector('img')).toBeNull();

    // Members already in the list and episodes are not offered.
    await user.type(within(region).getByRole('searchbox', { name: 'Titel suchen' }), 'al');
    const found = await within(region).findByRole('list', { name: 'Gefundene Titel' });
    expect(within(found).getAllByRole('listitem')).toHaveLength(1);
    await user.click(within(found).getByRole('button', { name: 'Alien hinzufügen' }));
    await waitFor(() => expect(memberTitles(region)).toHaveLength(3));
    expect(within(region).getByText('Ungespeicherte Änderungen')).toBeVisible();
    expect(within(rowOf('Filmabend')).getByText('geändert')).toBeVisible();

    // Alien to the first place: the others move along, the focus stays on its select (S-16).
    const place = within(region).getByRole('combobox', { name: 'Platz für Alien' });
    expect(place).toHaveValue('2');
    expect(place).toHaveAccessibleDescription('Die anderen Einträge rücken nach.');
    place.focus();
    await user.selectOptions(place, 'Platz 1');
    expect(memberTitles(region)).toEqual([
      expect.stringContaining('Alien'),
      expect.stringContaining('Heat'),
      expect.stringContaining('Arrival'),
    ]);
    expect(within(region).getByRole('combobox', { name: 'Platz für Alien' })).toHaveFocus();

    // The removed entry is gone with its button: the focus goes back to the search field.
    await user.click(within(region).getByRole('button', { name: 'Heat entfernen' }));
    expect(within(region).getByRole('searchbox', { name: 'Titel suchen' })).toHaveFocus();
    await user.click(within(region).getByRole('button', { name: 'Speichern' }));

    expect(await screen.findByText('Die Mitglieder von Filmabend sind gespeichert.')).toBeVisible();
    expect(calls.find((call) => call.method === 'PUT')?.body).toEqual({
      itemIds: [ids.alien, ids.arrival],
    });
    // The draft is spent: «geändert» and «Verwerfen» go, the focus stays on «Speichern».
    await waitFor(() => expect(within(rowOf('Filmabend')).queryByText('geändert')).toBeNull());
    expect(within(region).queryByRole('button', { name: 'Verwerfen' })).toBeNull();
    expect(within(region).getByRole('button', { name: 'Speichern' })).toHaveFocus();
    expect(memberTitles(region)).toEqual([
      expect.stringContaining('Alien'),
      expect.stringContaining('Arrival'),
    ]);
  });

  it('a position by keyboard moves the member; moving it back is no change', async () => {
    mockApi({
      ...signedIn,
      'GET /admin/collections': () => json([manual]),
      [`GET /admin/collections/${manualId}`]: () => json(members),
    });
    renderApp('/admin/collections');
    const user = userEvent.setup();
    const region = await openRow(user, 'Filmabend');
    const place = await within(region).findByRole('combobox', { name: 'Platz für Arrival' });
    place.focus();
    // ↑ on a closed select picks the position above (what the browsers do; jsdom has no keys
    // for a select, so the change comes as the browser sends it).
    fireEvent.change(place, { target: { value: '0' } });
    expect(memberTitles(region)).toEqual([
      expect.stringContaining('Arrival'),
      expect.stringContaining('Heat'),
    ]);
    expect(within(region).getByRole('combobox', { name: 'Platz für Arrival' })).toHaveFocus();
    expect(within(rowOf('Filmabend')).getByText('geändert')).toBeVisible();

    fireEvent.change(within(region).getByRole('combobox', { name: 'Platz für Arrival' }), {
      target: { value: '1' },
    });
    expect(within(region).getByRole('combobox', { name: 'Platz für Arrival' })).toHaveFocus();
    expect(within(rowOf('Filmabend')).queryByText('geändert')).toBeNull();
    expect(within(region).queryByRole('button', { name: 'Verwerfen' })).toBeNull();
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

    const region = await openRow(user, 'Filmabend');
    // While the members load there is nothing to pick from.
    expect(within(region).queryByRole('searchbox')).not.toBeInTheDocument();
    expect(within(region).getByRole('button', { name: 'Speichern' })).toBeDisabled();

    release();
    const search = await within(region).findByRole('searchbox', { name: 'Titel suchen' });
    await user.type(search, 'al');
    await user.click(await within(region).findByRole('button', { name: 'Alien hinzufügen' }));
    await user.click(within(region).getByRole('button', { name: 'Speichern' }));

    await waitFor(() =>
      expect(calls.find((call) => call.method === 'PUT')?.body).toEqual({
        itemIds: [ids.heat, ids.arrival, ids.alien],
      }),
    );
  });

  it('keeps unsaved members across folding and tabs (S-17), asks before leaving, discards', async () => {
    const { calls } = mockApi({
      ...signedIn,
      ...adminShell,
      'GET /admin/collections': () => json([tmdb, manual]),
      [`GET /admin/collections/${manualId}`]: () => json(members),
      [`GET /admin/collections/${tmdbId}`]: () => json(tmdbMembers),
      'GET /admin/items/search': () => json(hits),
    });
    const { router } = renderApp('/admin/collections');
    const user = userEvent.setup();

    let region = await openRow(user, 'Filmabend');
    await user.type(await within(region).findByRole('searchbox', { name: 'Titel suchen' }), 'al');
    await user.click(await within(region).findByRole('button', { name: 'Alien hinzufügen' }));
    await waitFor(() => expect(memberTitles(region)).toHaveLength(3));

    // Folded away: the row still says «geändert».
    await user.click(toggleOf('Filmabend'));
    expect(screen.queryByRole('region', { name: 'Mitglieder von Filmabend' })).toBeNull();
    expect(within(rowOf('Filmabend')).getByText('geändert')).toBeVisible();

    // Another tab and back is no leaving: no question, the row is open again with the draft.
    await user.click(screen.getByRole('tab', { name: /^Manuell/ }));
    await waitFor(() => expect(router.state.location.search).toEqual({ tab: 'manual' }));
    expect(screen.queryByRole('alertdialog')).toBeNull();
    expect(screen.getByRole('table', { name: 'Manuelle Sammlungen' })).toBeVisible();
    region = await openRow(user, 'Filmabend');
    expect(memberTitles(region)).toHaveLength(3);
    await user.click(screen.getByRole('tab', { name: /^Alle/ }));
    await waitFor(() => expect(router.state.location.search).toEqual({}));
    region = await screen.findByRole('region', { name: 'Mitglieder von Filmabend' });
    expect(memberTitles(region)).toEqual([
      expect.stringContaining('Heat'),
      expect.stringContaining('Arrival'),
      expect.stringContaining('Alien'),
    ]);

    // Leaving the page with the draft asks; staying keeps it.
    await user.click(
      within(await screen.findByRole('navigation', { name: 'Verwaltung' })).getByRole('link', {
        name: /^Integrationen/,
      }),
    );
    const leave = await screen.findByRole('alertdialog', {
      name: 'Seite mit ungespeicherten Änderungen verlassen?',
    });
    await user.click(within(leave).getByRole('button', { name: 'Auf der Seite bleiben' }));
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull());
    expect(screen.getByRole('heading', { level: 1, name: 'Sammlungen' })).toBeVisible();

    // «Verwerfen»: what the server holds again, said in the status region; nothing was sent.
    await user.click(within(region).getByRole('button', { name: 'Verwerfen' }));
    expect(memberTitles(region)).toEqual([
      expect.stringContaining('Heat'),
      expect.stringContaining('Arrival'),
    ]);
    const said = await screen.findByText('Die Änderungen an Filmabend sind verworfen.');
    expect(said.closest('[role="status"]')).not.toBeNull();
    expect(within(region).getByRole('button', { name: 'Speichern' })).toHaveFocus();
    expect(within(rowOf('Filmabend')).queryByText('geändert')).toBeNull();
    expect(calls.some((call) => call.method === 'PUT')).toBe(false);
  });

  it('filters by source in tabs with counts, kept in ?tab=; an empty tab says so', async () => {
    mockApi({
      ...signedIn,
      'GET /admin/collections': () =>
        json([tmdb, manual, { ...manual, id: tmdbId.replace('2B', '4D'), name: 'Kinder' }]),
    });
    const { router } = renderApp('/admin/collections');
    const user = userEvent.setup();

    const tabs = await screen.findByRole('tablist', { name: 'Sammlungen nach Quelle' });
    expect(
      within(tabs)
        .getAllByRole('tab')
        .map((tab) => tab.getAttribute('aria-selected') + ' ' + tab.textContent),
    ).toEqual(['true Alle 33 Sammlungen', 'false Manuell 22 Sammlungen', 'false TMDB 11 Sammlung']);
    expect(within(tabs).getByRole('tab', { name: 'Manuell 2 Sammlungen' })).toBeVisible();
    await user.click(within(tabs).getByRole('tab', { name: /^TMDB/ }));
    await waitFor(() => expect(router.state.location.search).toEqual({ tab: 'tmdb' }));
    const table = screen.getByRole('table', { name: 'Sammlungen von TMDB' });
    expect(
      within(table)
        .getAllByRole('rowheader')
        .map((cell) => cell.textContent),
    ).toEqual(['Alien CollectionAlien Filmreihe']);
    expect(within(tabs).getByRole('tab', { selected: true })).toHaveAccessibleName(
      'TMDB 1 Sammlung',
    );
  });

  it('a tab without collections says so; an unknown ?tab= shows «Alle» and leaves the address', async () => {
    mockApi({ ...signedIn, 'GET /admin/collections': () => json([manual]) });
    const linked = renderApp('/admin/collections?tab=tmdb');
    expect(await screen.findByText('Keine Sammlung in diesem Reiter.')).toBeVisible();
    linked.unmount();

    const { router } = renderApp('/admin/collections?tab=nonsense');
    expect(await screen.findByRole('tab', { selected: true })).toHaveAccessibleName(
      'Alle 1 Sammlung',
    );
    await waitFor(() => expect(router.state.location.search).toEqual({}));
    // Replaced, not pushed: going back does not return to the unknown tab.
    expect(router.history.length).toBe(1);
  });

  it('«Löschen» asks first and moves the focus to the heading; cancelling sends nothing', async () => {
    let list = [manual];
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/collections': () => json(list),
      [`GET /admin/collections/${manualId}`]: () => json(members),
      [`DELETE /admin/collections/${manualId}`]: () => {
        list = [];
        return new Response(null, { status: 204 });
      },
    });
    renderApp('/admin/collections');
    const user = userEvent.setup();

    const region = await openRow(user, 'Filmabend');
    const remove = within(region).getByRole('button', { name: 'Sammlung Filmabend löschen' });
    await user.click(remove);
    let confirm = await screen.findByRole('alertdialog', { name: 'Sammlung Filmabend löschen?' });
    await user.click(within(confirm).getByRole('button', { name: 'Abbrechen' }));
    await waitFor(() => expect(confirm).not.toBeInTheDocument());
    expect(calls.some((call) => call.method === 'DELETE')).toBe(false);
    await waitFor(() => expect(remove).toHaveFocus());

    await user.click(remove);
    confirm = await screen.findByRole('alertdialog', { name: 'Sammlung Filmabend löschen?' });
    expect(confirm).toHaveTextContent('Die Titel bleiben in der Bibliothek');
    await user.click(within(confirm).getByRole('button', { name: 'Sammlung löschen' }));
    const done = await screen.findByText('Die Sammlung Filmabend ist gelöscht.');
    expect(done.closest('[role="status"]')).not.toBeNull();
    expect(calls.some((call) => call.method === 'DELETE')).toBe(true);
    expect(await screen.findByText('Noch keine Sammlung.')).toBeVisible();
    await waitFor(() =>
      expect(screen.getByRole('heading', { level: 1, name: 'Sammlungen' })).toHaveFocus(),
    );
  });

  it('says the same result twice: each message is a new node in the status region', async () => {
    mockApi({
      ...signedIn,
      'GET /admin/collections': () => json([manual]),
      [`GET /admin/collections/${manualId}`]: () => json(members),
      [`PUT /admin/collections/${manualId}/items`]: () => json(members),
    });
    renderApp('/admin/collections');
    const user = userEvent.setup();
    const region = await openRow(user, 'Filmabend');
    const save = await within(region).findByRole('button', { name: 'Speichern' });
    await waitFor(() => expect(save).toBeEnabled());
    await user.click(save);
    const first = await screen.findByText('Die Mitglieder von Filmabend sind gespeichert.');
    await user.click(save);
    await waitFor(() =>
      expect(screen.getByText('Die Mitglieder von Filmabend sind gespeichert.')).not.toBe(first),
    );
    expect(first).not.toBeInTheDocument();
  });

  it('pauses the refetch on focus for an open draft or the confirmation, not for a folded draft', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/collections': () => json([manual]),
      [`GET /admin/collections/${manualId}`]: () => json(members),
    });
    renderApp('/admin/collections');
    const user = userEvent.setup();
    const lists = () => calls.filter((call) => call.path === '/admin/collections').length;
    try {
      const region = await openRow(user, 'Filmabend');
      fireEvent.change(await within(region).findByRole('combobox', { name: 'Platz für Arrival' }), {
        target: { value: '0' },
      });
      let before = lists();
      refocus();
      await settle();
      expect(lists()).toBe(before);

      // Folded away, the draft stays but no longer holds the page.
      await user.click(toggleOf('Filmabend'));
      refocus();
      await waitFor(() => expect(lists()).toBeGreaterThan(before));
      expect(within(rowOf('Filmabend')).getByText('geändert')).toBeVisible();

      // Open again without a draft, the confirmation holds the page while it is open.
      const again = await openRow(user, 'Filmabend');
      await user.click(within(again).getByRole('button', { name: 'Verwerfen' }));
      await user.click(within(again).getByRole('button', { name: 'Sammlung Filmabend löschen' }));
      const confirm = await screen.findByRole('alertdialog');
      before = lists();
      refocus();
      await settle();
      expect(lists()).toBe(before);
      await user.click(within(confirm).getByRole('button', { name: 'Abbrechen' }));
      await waitFor(() => expect(confirm).not.toBeInTheDocument());
      refocus();
      await waitFor(() => expect(lists()).toBeGreaterThan(before));
    } finally {
      focusManager.setFocused(undefined);
    }
  });

  it('a refused save shows the problem in the row and keeps the draft; a refused delete keeps the row', async () => {
    mockApi({
      ...signedIn,
      'GET /admin/collections': () => json([manual]),
      [`GET /admin/collections/${manualId}`]: () => json(members),
      [`PUT /admin/collections/${manualId}/items`]: () => problem(500, 'internal'),
      [`DELETE /admin/collections/${manualId}`]: () => problem(500, 'internal'),
    });
    renderApp('/admin/collections');
    const user = userEvent.setup();
    const region = await openRow(user, 'Filmabend');
    fireEvent.change(await within(region).findByRole('combobox', { name: 'Platz für Arrival' }), {
      target: { value: '0' },
    });
    await user.click(within(region).getByRole('button', { name: 'Speichern' }));
    expect(await within(region).findByRole('alert')).toBeVisible();
    expect(within(rowOf('Filmabend')).getByText('geändert')).toBeVisible();
    expect(screen.queryByText('Die Mitglieder von Filmabend sind gespeichert.')).toBeNull();

    await user.click(within(region).getByRole('button', { name: 'Sammlung Filmabend löschen' }));
    const confirm = await screen.findByRole('alertdialog');
    await user.click(within(confirm).getByRole('button', { name: 'Sammlung löschen' }));
    await waitFor(() => expect(confirm).not.toBeInTheDocument());
    await waitFor(() => expect(screen.getAllByRole('alert')).toHaveLength(2));
    expect(toggleOf('Filmabend')).toBeVisible();
    expect(screen.queryByText('Die Sammlung Filmabend ist gelöscht.')).toBeNull();
  });

  it('the list or the members fail to load: the error offers «Erneut versuchen»', async () => {
    let fail = true;
    mockApi({
      ...signedIn,
      'GET /admin/collections': () => (fail ? problem(500, 'internal') : json([manual])),
      [`GET /admin/collections/${manualId}`]: () =>
        fail ? problem(500, 'internal') : json(members),
    });
    renderApp('/admin/collections');
    const user = userEvent.setup();
    const retry = await screen.findByRole('button', { name: 'Erneut versuchen' });
    fail = false;
    await user.click(retry);
    expect(await screen.findByRole('table', { name: 'Sammlungen' })).toBeVisible();

    fail = true;
    const region = await openRow(user, 'Filmabend');
    const again = await within(region).findByRole('button', { name: 'Erneut versuchen' });
    expect(within(region).queryByRole('searchbox')).toBeNull();
    fail = false;
    await user.click(again);
    expect(await within(region).findByRole('searchbox', { name: 'Titel suchen' })).toBeVisible();
  });

  it('is for the main account only', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /auth/session': () =>
        json({ ...session, account: { ...session.account, role: 'user', isOwner: false } }),
    });
    renderApp('/admin/collections');
    expect(await screen.findByText('Nur für Administratoren')).toBeVisible();
    expect(calls.some((call) => call.path.startsWith('/admin/'))).toBe(false);
  });
});
