import type { AdminItem, AdminItemHit, AdminPick } from '@slouchcat/api-client';
import { focusManager } from '@tanstack/react-query';
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ids, profiles } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, session, signedIn } from '@/test/api';
import { adminShell, renderAdminPage } from '@/test/admin';

// /admin/picks (M7, E-2; concept 12, variant C): the picks in carousel order, filtered by tabs
// «Alle», «Aktiv», «Geplant», «Abgelaufen» (time only); each pick folds out into its note and
// time window, saved in place, its place in the row as a choice (S-16) and «Entfernen»
// (PIC-1 to PIC-4).

const heatPick: AdminPick = {
  id: 11,
  itemId: ids.heat,
  kind: 'movie',
  title: 'Heat',
  year: 1995,
  position: 0,
  note: 'Ein Klassiker',
  createdAt: '2026-10-01T10:00:00Z',
  posterUrl: `/api/v1/admin/items/${ids.heat}/images/poster/w200?v=heat`,
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
  providers: ['tmdb'],
  fieldSources: [],
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

/** The fallback mark of a title without a poster (PosterFallback), not the row's chevron. */
const FALLBACK = 'svg[viewBox="0 0 512 512"]';

/** The query string of the last admin search the page sent. */
function lastSearch(fetchMock: ReturnType<typeof mockApi>['fetchMock']): URLSearchParams | null {
  const urls = fetchMock.mock.calls
    .map(([request]) => new URL(request.url))
    .filter((url) => url.pathname.endsWith('/admin/items/search'));
  return urls.at(-1)?.searchParams ?? null;
}

/** The titles of the rows in the open tab, in order. */
const titles = () =>
  within(screen.getByRole('table'))
    .getAllByRole('rowheader')
    .map((cell) => cell.textContent);

/** The button of the row of `title` that folds it out. */
async function toggleOf(title: string): Promise<HTMLElement> {
  const panel = await screen.findByRole('tabpanel');
  return waitFor(() => {
    const toggle = within(panel)
      .getAllByRole('button', { name: new RegExp(`^${title}`) })
      .find((button) => button.hasAttribute('aria-expanded'));
    if (!toggle) throw new Error(`no row for ${title}`);
    return toggle;
  });
}

/** Folds out the pick whose title button starts with `title` and returns its fold-out. */
async function openPick(user: ReturnType<typeof userEvent.setup>, title: string) {
  const toggle = await toggleOf(title);
  if (toggle.getAttribute('aria-expanded') !== 'true') await user.click(toggle);
  const detail = document.getElementById(toggle.getAttribute('aria-controls') ?? '');
  if (!detail) throw new Error(`no fold-out for ${title}`);
  return detail;
}

describe('admin picks page (M7, E-2; concept 12)', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('lists the picks in carousel order with target, note and time window (PIC-2)', async () => {
    mockApi({ ...routes, ...adminShell, 'GET /admin/picks': () => json([heatPick, arrivalPick]) });
    const { nav } = await renderAdminPage('/admin/picks');

    expect(
      await screen.findByRole('heading', { level: 1, name: "Cat's Empfehlungen" }),
    ).toBeVisible();
    // The intro sits behind the help button (S-2).
    expect(screen.getByRole('button', { name: 'Hilfe zu dieser Seite' })).toBeVisible();
    const table = await screen.findByRole('table', {
      name: 'Empfehlungen in der Reihenfolge der Reihe',
    });
    const rows = within(table)
      .getAllByRole('row')
      .filter((row) => !row.hidden);
    expect(rows).toHaveLength(3);
    expect(within(rows[1]!).getByRole('rowheader')).toHaveTextContent('Heat');
    expect(within(rows[1]!).getByText('Film · 1995')).toBeVisible();
    expect(within(rows[1]!).getByText('Alle')).toBeVisible();
    expect(within(rows[1]!).getByText('«Ein Klassiker»')).toBeVisible();
    expect(within(rows[1]!).getByText('Immer')).toBeVisible();
    expect(within(rows[2]!).getByText('Lena')).toBeVisible();
    expect(within(rows[2]!).getByText(/ bis /)).toBeVisible();
    expect(within(rows[2]!).getByText('Abgelaufen')).toBeVisible();
    // Without a note: a dash on screen, words for screen readers.
    expect(within(rows[2]!).getByText('–')).toHaveAttribute('aria-hidden', 'true');
    expect(within(rows[2]!).getByText('keine Notiz')).toBeInTheDocument();
    expect(screen.getByText('2 Einträge')).toBeVisible();
    // No up and down buttons any more: the place is a choice in the fold-out (S-16).
    expect(screen.queryByRole('button', { name: /nach (oben|unten)$/ })).toBeNull();
    expect(nav.querySelector('[aria-current="page"]')).toHaveTextContent("Cat's Empfehlungen");
  });

  it('numbers the picks, shows their posters, counts the planned ones and points to the start page', async () => {
    const planned: AdminPick = { ...alienPick, startsAt: '2099-01-01T10:00:00Z' };
    // Positions with a gap, as after a removal: "Nr." is the place in the row.
    mockApi({
      ...routes,
      'GET /admin/picks': () => json([heatPick, arrivalPick, { ...planned, position: 4 }]),
    });
    renderApp('/admin/picks');

    const table = await screen.findByRole('table', {
      name: 'Empfehlungen in der Reihenfolge der Reihe',
    });
    expect(within(table).getAllByRole('columnheader')[0]).toHaveTextContent('Nr.');
    const rows = within(table)
      .getAllByRole('row')
      .filter((row) => !row.hidden)
      .slice(1);
    expect(rows.map((row) => within(row).getAllByRole('cell')[0]?.textContent)).toEqual([
      '1',
      '2',
      '3',
    ]);
    expect(screen.getByText('3 Einträge · 1 geplant')).toBeVisible();
    expect(within(rows[2]!).getByText('Geplant')).toBeVisible();

    // The poster sits in the row header and adds no text to it (alt="").
    const heat = within(rows[0]!).getByRole('rowheader');
    expect(heat.textContent).toBe('HeatFilm · 1995');
    const poster = heat.querySelector('img');
    expect(poster).toHaveAttribute('alt', '');
    expect(poster).toHaveAttribute('src', heatPick.posterUrl);
    expect(heat.querySelector(FALLBACK)).toBeNull();
    // Without a poster the placeholder, not a broken image.
    const arrival = within(rows[1]!).getByRole('rowheader');
    expect(arrival.textContent).toBe('ArrivalFilm · 2016');
    expect(arrival.querySelector('img')).toBeNull();
    expect(arrival.querySelector(FALLBACK)).not.toBeNull();
    // An image that fails to load gives way to the placeholder as well.
    fireEvent.error(poster!);
    expect(heat.querySelector('img')).toBeNull();
    expect(heat.querySelector(FALLBACK)).not.toBeNull();

    // The order of the rows on the start page lives on its own page: the hint links there (PIC-4).
    const hint = screen.getByText(/^Wo die Reihe auf der Startseite steht/);
    expect(hint).toHaveTextContent(
      'Wo die Reihe auf der Startseite steht und ob sie erscheint, stellst du unter Startseite ein.',
    );
    expect(within(hint).getByRole('link', { name: 'Startseite' })).toHaveAttribute(
      'href',
      '/admin/home-rows',
    );
  });

  it('filters by time in tabs with counts, kept in ?tab=; «Aktiv» is time only', async () => {
    const planned: AdminPick = { ...alienPick, startsAt: '2099-01-01T10:00:00Z' };
    // A pick for one profile whose window runs now: active, whoever may see the title.
    const running: AdminPick = {
      ...heatPick,
      id: 14,
      title: 'Ronin',
      profileId: 2,
      profileName: 'Lena',
      startsAt: '2020-01-01T10:00:00Z',
      endsAt: '2099-01-01T10:00:00Z',
    };
    mockApi({
      ...routes,
      'GET /admin/picks': () => json([heatPick, arrivalPick, planned, running]),
    });
    const { router } = renderApp('/admin/picks?tab=scheduled');
    const user = userEvent.setup();

    const tabs = await screen.findByRole('tablist', { name: 'Empfehlungen nach Zeitraum' });
    expect(
      within(tabs)
        .getAllByRole('tab')
        .map((tab) => tab.textContent),
    ).toEqual([
      'Alle 44 Empfehlungen',
      'Aktiv 22 Empfehlungen',
      'Geplant 11 Empfehlung',
      'Abgelaufen 11 Empfehlung',
    ]);
    expect(within(tabs).getByRole('tab', { selected: true })).toHaveAccessibleName(
      'Geplant 1 Empfehlung',
    );
    expect(titles()).toEqual([expect.stringMatching(/^Alien/)]);
    // «Nr.» is the place in the whole row, also in a filtered tab.
    expect(within(screen.getByRole('table')).getAllByRole('cell')[0]).toHaveTextContent('3');

    await user.click(within(tabs).getByRole('tab', { name: /^Aktiv/ }));
    await waitFor(() => expect(router.state.location.search).toEqual({ tab: 'active' }));
    expect(titles()).toEqual([expect.stringMatching(/^Heat/), expect.stringMatching(/^Ronin/)]);
    await user.click(within(tabs).getByRole('tab', { name: /^Abgelaufen/ }));
    expect(titles()).toEqual([expect.stringMatching(/^Arrival/)]);
    await user.click(within(tabs).getByRole('tab', { name: /^Alle/ }));
    await waitFor(() => expect(router.state.location.search).toEqual({}));
    expect(titles()).toHaveLength(4);
  });

  it('a tab without picks says so; an unknown ?tab= shows «Alle» and leaves the address', async () => {
    mockApi({ ...routes, 'GET /admin/picks': () => json([heatPick]) });
    const { router } = renderApp('/admin/picks?tab=nonsense');
    const tab = await screen.findByRole('tab', { selected: true });
    expect(tab).toHaveAccessibleName('Alle 1 Empfehlung');
    await waitFor(() => expect(router.state.location.search).toEqual({}));
    // Replaced, not pushed: going back does not return to the unknown tab.
    expect(router.history.length).toBe(1);
    await userEvent.setup().click(screen.getByRole('tab', { name: /^Geplant/ }));
    expect(
      within(screen.getByRole('tabpanel')).getByText('Keine Empfehlung in diesem Reiter.'),
    ).toBeVisible();
  });

  it('«Stelle» sends the whole new order; the focus stays on the choice; each move is announced', async () => {
    let list = [heatPick, arrivalPick];
    const { calls } = mockApi({
      ...routes,
      'GET /admin/picks': () => json(list),
      'PUT /admin/picks/order': (call) => {
        const order = (call.body as { ids: number[] }).ids;
        list = order.map((id) => [heatPick, arrivalPick].find((pick) => pick.id === id)!);
        return new Response(null, { status: 204 });
      },
    });
    renderApp('/admin/picks');
    const user = userEvent.setup();

    const detail = await openPick(user, 'Heat');
    // The status region is there before its first message, so that one is announced as well.
    const live = screen.getAllByRole('status').find((element) => element.textContent === '');
    expect(live).toBeDefined();
    const place = within(detail).getByRole('combobox', { name: 'Platz für Heat' });
    expect(place).toHaveValue('0');
    expect(place).toHaveAccessibleDescription('Die anderen Einträge rücken nach.');
    expect(
      within(place)
        .getAllByRole('option')
        .map((option) => option.textContent),
    ).toEqual(['Platz 1', 'Platz 2']);
    await user.selectOptions(place, '1');
    expect(calls.find((call) => call.method === 'PUT')?.body).toEqual({ ids: [12, 11] });
    expect(await screen.findByText('Heat steht jetzt an Stelle 2.')).toBeVisible();
    expect(live).toHaveTextContent('Heat steht jetzt an Stelle 2.');
    expect(within(live!).queryByRole('status')).not.toBeInTheDocument();
    expect(titles()).toEqual([expect.stringMatching(/^Arrival/), expect.stringMatching(/^Heat/)]);
    await waitFor(() =>
      expect(screen.getByRole('combobox', { name: 'Platz für Heat' })).toHaveFocus(),
    );
    expect(screen.getByRole('combobox', { name: 'Platz für Heat' })).toHaveValue('1');

    // Back and to place 2 again: the same words are a new message, announced again.
    const first = within(live!).getByText('Heat steht jetzt an Stelle 2.');
    await user.selectOptions(screen.getByRole('combobox', { name: 'Platz für Heat' }), '0');
    await waitFor(() => expect(titles()[0]).toMatch(/^Heat/));
    await user.selectOptions(screen.getByRole('combobox', { name: 'Platz für Heat' }), '1');
    await waitFor(() => expect(titles()[0]).toMatch(/^Arrival/));
    const again = within(live!).getByText('Heat steht jetzt an Stelle 2.');
    expect(again).not.toBe(first);
    expect(calls.filter((call) => call.method === 'PUT')).toHaveLength(3);
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

    const detail = await openPick(user, 'Alien');
    await user.selectOptions(
      within(detail).getByRole('combobox', { name: 'Platz für Alien' }),
      '1',
    );
    await user.selectOptions(screen.getByRole('combobox', { name: 'Platz für Alien' }), '0');
    // Both moves show at once; the second PUT waits for the first.
    expect(titles()).toEqual([
      expect.stringMatching(/^Alien/),
      expect.stringMatching(/^Heat/),
      expect.stringMatching(/^Arrival/),
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
      expect.stringMatching(/^Alien/),
      expect.stringMatching(/^Heat/),
      expect.stringMatching(/^Arrival/),
    ]);
  });

  it('puts the old order back when the server refuses a move; the choice keeps the focus', async () => {
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

    const detail = await openPick(user, 'Heat');
    await user.selectOptions(within(detail).getByRole('combobox', { name: 'Platz für Heat' }), '1');
    // The new order shows at once.
    await waitFor(() =>
      expect(titles()).toEqual([expect.stringMatching(/^Arrival/), expect.stringMatching(/^Heat/)]),
    );

    refuse();
    expect(await screen.findByRole('alert')).toBeVisible();
    await waitFor(() =>
      expect(titles()).toEqual([expect.stringMatching(/^Heat/), expect.stringMatching(/^Arrival/)]),
    );
    const place = screen.getByRole('combobox', { name: 'Platz für Heat' });
    expect(place).toHaveValue('0');
    await waitFor(() => expect(place).toHaveFocus());
  });

  it('a refused move with later moves queued does not roll back; the last answer puts it right', async () => {
    let list = [heatPick, arrivalPick, alienPick];
    let refuseFirst: () => void = () => undefined;
    const firstRefused = new Promise<void>((resolve) => {
      refuseFirst = resolve;
    });
    const { calls } = mockApi({
      ...routes,
      'GET /admin/picks': () => json(list),
      'PUT /admin/picks/order': async (call) => {
        if (calls.filter((entry) => entry.method === 'PUT').length === 1) {
          await firstRefused;
          return problem(500, 'internal');
        }
        const order = (call.body as { ids: number[] }).ids;
        list = order.map((id) =>
          [heatPick, arrivalPick, alienPick].find((pick) => pick.id === id)!,
        );
        return new Response(null, { status: 204 });
      },
    });
    renderApp('/admin/picks');
    const user = userEvent.setup();

    const detail = await openPick(user, 'Alien');
    await user.selectOptions(
      within(detail).getByRole('combobox', { name: 'Platz für Alien' }),
      '1',
    );
    await user.selectOptions(screen.getByRole('combobox', { name: 'Platz für Alien' }), '0');
    refuseFirst();
    expect(await screen.findByRole('alert')).toBeVisible();
    // No step back to the order before the refused move: the queued one shows on.
    expect(titles()).toEqual([
      expect.stringMatching(/^Alien/),
      expect.stringMatching(/^Heat/),
      expect.stringMatching(/^Arrival/),
    ]);
    await waitFor(() => expect(calls.filter((call) => call.method === 'PUT')).toHaveLength(2));
    // The list asked again after the last answer is what the server holds.
    await waitFor(() =>
      expect(calls.filter((call) => call.path === '/admin/picks').length).toBeGreaterThan(1),
    );
    expect(titles()).toEqual([
      expect.stringMatching(/^Alien/),
      expect.stringMatching(/^Heat/),
      expect.stringMatching(/^Arrival/),
    ]);
  });

  it('adds a title found by the search for a single profile (PIC-1)', async () => {
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

    // Empty: one line with the main action (S-10).
    const empty = await screen.findByText(/Noch keine Empfehlung/);
    expect(empty).toBeVisible();
    const add = screen.getAllByRole('button', { name: 'Hinzufügen' });
    expect(add).toHaveLength(2);
    // Escape returns the focus to the button that opened the dialog: here the empty list's.
    await user.click(add[1]!);
    await screen.findByRole('dialog', { name: 'Empfehlung hinzufügen' });
    await user.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    await waitFor(() => expect(add[1]).toHaveFocus());
    await user.click(add[0]!);
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
    expect(within(dialog).getByText(/^Gewählt: Alien/)).toHaveTextContent(
      'Gewählt: Alien · Film · 1979',
    );

    // A single profile replaces "Alle Profile".
    const all = within(dialog).getByRole('checkbox', { name: 'Alle Profile' });
    expect(all).toBeChecked();
    await user.click(within(dialog).getByRole('checkbox', { name: 'Lena' }));
    expect(all).not.toBeChecked();
    await user.type(within(dialog).getByRole('textbox', { name: 'Notiz' }), 'Für Lena');
    // The time window goes along as instants.
    await user.type(within(dialog).getByLabelText('Sichtbar ab'), '2026-11-02T10:00');
    await user.type(within(dialog).getByLabelText('Sichtbar bis'), '2026-11-09T22:30');
    await user.click(within(dialog).getByRole('button', { name: 'Hinzufügen' }));

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(calls.filter((call) => call.method === 'POST').map((call) => call.body)).toEqual([
      {
        itemId: ids.alien,
        profileIds: [2],
        note: 'Für Lena',
        startsAt: new Date('2026-11-02T10:00').toISOString(),
        endsAt: new Date('2026-11-09T22:30').toISOString(),
      },
    ]);
    expect(screen.getByText("Alien ist in Cat's Empfehlungen.")).toBeVisible();
    expect(await screen.findByRole('rowheader', { name: /Alien/ })).toBeVisible();
    await waitFor(() => expect(screen.getByRole('button', { name: 'Hinzufügen' })).toHaveFocus());
  });

  it('?add= opens the dialog with the title of the link; a pick for everyone has no profiles', async () => {
    const { calls } = mockApi({
      ...routes,
      'GET /admin/picks': () => json([heatPick]),
      // The admin view of the title: the active profile's visibility does not count here.
      [`GET /admin/items/${ids.alien}`]: () => json(alienAdmin),
      'POST /admin/picks': () => json([{ ...heatPick, itemId: ids.alien, title: 'Alien' }], 201),
    });
    const { router } = renderApp(`/admin/picks?add=${ids.alien}&tab=active`);
    const user = userEvent.setup();

    const dialog = await screen.findByRole('dialog', { name: 'Empfehlung hinzufügen' });
    expect(await within(dialog).findByText(/^Gewählt: Alien/)).toBeVisible();
    expect(calls.some((call) => call.path === `/items/${ids.alien}`)).toBe(false);
    await user.click(within(dialog).getByRole('button', { name: 'Hinzufügen' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(calls.find((call) => call.method === 'POST')?.body).toEqual({ itemId: ids.alien });
    // The link is used once: the parameter is gone, the tab stays.
    await waitFor(() => expect(router.state.location.search).toEqual({ tab: 'active' }));
  });

  it('saves note and window in the fold-out; the browser checks the window, the server problem shows at the field', async () => {
    const { calls } = mockApi({
      ...routes,
      'GET /admin/picks': () => json([heatPick]),
      'PATCH /admin/picks/11': () =>
        problem(422, 'validation_failed', { errors: [{ field: 'note', code: 'too_long' }] }),
    });
    renderApp('/admin/picks');
    const user = userEvent.setup();

    const detail = await openPick(user, 'Heat');
    const form = within(detail).getByRole('form', { name: 'Heat bearbeiten' });
    const note = within(form).getByLabelText('Notiz');
    expect(note).toHaveValue('Ein Klassiker');
    // Nothing changed yet: «Speichern» sends nothing, there is nothing to drop.
    const save = within(form).getByRole('button', { name: 'Heat speichern' });
    expect(save).toBeEnabled();
    expect(within(form).queryByRole('button', { name: 'Änderungen an Heat verwerfen' })).toBeNull();
    await user.click(save);
    expect(calls.some((call) => call.method === 'PATCH')).toBe(false);
    expect(save).toHaveFocus();
    const from = within(form).getByLabelText('Sichtbar ab');
    const to = within(form).getByLabelText('Sichtbar bis');
    await user.type(from, '2026-11-02T10:00');
    await user.type(to, '2026-11-01T10:00');
    await user.click(within(form).getByRole('button', { name: 'Heat speichern' }));
    expect(to).toHaveAccessibleDescription(/Das Ende muss nach dem Beginn liegen/);
    expect(calls.some((call) => call.method === 'PATCH')).toBe(false);

    await user.clear(to);
    await user.type(to, '2026-11-03T10:00');
    await user.clear(note);
    await user.click(within(form).getByRole('button', { name: 'Heat speichern' }));
    await waitFor(() => expect(note).toHaveAccessibleDescription(/Zu lang/));
    expect(calls.find((call) => call.method === 'PATCH')?.body).toEqual({
      note: null,
      startsAt: new Date('2026-11-02T10:00').toISOString(),
      endsAt: new Date('2026-11-03T10:00').toISOString(),
    });

    // «Verwerfen» drops the input and the refusal about it; the focus goes to «Speichern».
    await user.click(within(form).getByRole('button', { name: 'Änderungen an Heat verwerfen' }));
    expect(note).toHaveValue('Ein Klassiker');
    expect(note).not.toHaveAccessibleDescription(/Zu lang/);
    expect(within(form).queryByRole('button', { name: 'Änderungen an Heat verwerfen' })).toBeNull();
    await waitFor(() => expect(save).toHaveFocus());
  });

  it('keeps what is typed while a save runs; saving and its error belong to one pick', async () => {
    let answer: () => void = () => undefined;
    const answered = new Promise<void>((resolve) => {
      answer = resolve;
    });
    let list = [heatPick, alienPick];
    const { calls } = mockApi({
      ...routes,
      'GET /admin/picks': () => json(list),
      'PATCH /admin/picks/11': async (call) => {
        await answered;
        const body = call.body as { note: string | null };
        list = [{ ...heatPick, note: body.note ?? undefined }, alienPick];
        return json(list[0]);
      },
      'PATCH /admin/picks/13': () => problem(500, 'internal'),
    });
    renderApp('/admin/picks');
    const user = userEvent.setup();

    const heat = await openPick(user, 'Heat');
    const alien = await openPick(user, 'Alien');
    const note = within(heat).getByLabelText('Notiz');
    await user.type(note, '!');
    await user.click(within(heat).getByRole('button', { name: 'Heat speichern' }));
    // Only Heat saves; Alien's «Speichern» is free, and its refusal stays with Alien.
    expect(within(heat).getByRole('button', { name: 'Heat speichern' })).toHaveTextContent(
      'Wird gespeichert …',
    );
    expect(within(alien).getByRole('button', { name: 'Alien speichern' })).toHaveTextContent(
      'Speichern',
    );
    await user.type(within(alien).getByLabelText('Notiz'), 'Gruselig');
    await user.click(within(alien).getByRole('button', { name: 'Alien speichern' }));
    expect(await within(alien).findByRole('alert')).toBeVisible();
    expect(within(heat).queryByRole('alert')).toBeNull();

    // Typed while Heat's save runs: kept as a draft when the answer comes.
    await user.type(note, '?');
    answer();
    expect(await screen.findByText('Heat ist gespeichert.')).toBeVisible();
    expect(calls.find((call) => call.path === '/admin/picks/11')?.body).toEqual({
      note: 'Ein Klassiker!',
      startsAt: null,
      endsAt: null,
    });
    expect(note).toHaveValue('Ein Klassiker!?');
    const discard = within(heat).getByRole('button', { name: 'Änderungen an Heat verwerfen' });
    expect(discard).toBeVisible();
    // «Verwerfen» now goes back to what the server holds since the save.
    await user.click(discard);
    expect(note).toHaveValue('Ein Klassiker!');
  });

  it('keeps the draft of each pick across tabs and folding (S-17), asks before leaving, saves and discards', async () => {
    let list = [heatPick, alienPick];
    const { calls } = mockApi({
      ...routes,
      ...adminShell,
      'GET /admin/picks': () => json(list),
      'PATCH /admin/picks/11': (call) => {
        const body = call.body as { note: string | null };
        list = [{ ...heatPick, note: body.note ?? undefined }, alienPick];
        return json(list[0]);
      },
    });
    await renderAdminPage('/admin/picks');
    const user = userEvent.setup();

    // Heat gets a new note, Alien one too; a blank after a note is no change.
    let heat = await openPick(user, 'Heat');
    await user.type(within(heat).getByLabelText('Notiz'), ' ');
    expect(within(heat).queryByRole('button', { name: 'Änderungen an Heat verwerfen' })).toBeNull();
    await user.type(within(heat).getByLabelText('Notiz'), 'mit Popcorn');
    expect(
      within(heat).getByRole('button', { name: 'Änderungen an Heat verwerfen' }),
    ).toBeVisible();
    let alien = await openPick(user, 'Alien');
    await user.type(within(alien).getByLabelText('Notiz'), 'Gruselig');
    expect(screen.getAllByText('geändert')).toHaveLength(2);

    // Another tab and back, then folding Heat away and out again: both drafts stay.
    await user.click(screen.getByRole('tab', { name: /^Abgelaufen/ }));
    await user.click(screen.getByRole('tab', { name: /^Aktiv/ }));
    await user.click(await toggleOf('Heat'));
    expect(await toggleOf('Heat')).toHaveAttribute('aria-expanded', 'false');
    heat = await openPick(user, 'Heat');
    expect(within(heat).getByLabelText('Notiz')).toHaveValue('Ein Klassiker mit Popcorn');
    alien = await openPick(user, 'Alien');
    expect(within(alien).getByLabelText('Notiz')).toHaveValue('Gruselig');

    // Leaving the page with a draft asks; staying keeps it.
    await user.click(
      within(screen.getByRole('navigation', { name: 'Verwaltung' })).getByRole('link', {
        name: /^Sammlungen/,
      }),
    );
    const leave = await screen.findByRole('alertdialog', {
      name: 'Seite mit ungespeicherten Änderungen verlassen?',
    });
    await user.click(within(leave).getByRole('button', { name: 'Auf der Seite bleiben' }));
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull());
    expect(screen.getByRole('heading', { level: 1, name: "Cat's Empfehlungen" })).toBeVisible();

    // «Speichern» sends only Heat, trimmed; its draft is spent, «Speichern» keeps the focus.
    await user.click(within(heat).getByRole('button', { name: 'Heat speichern' }));
    expect(await screen.findByText('Heat ist gespeichert.')).toBeVisible();
    expect(calls.filter((call) => call.method === 'PATCH')).toEqual([
      expect.objectContaining({
        path: '/admin/picks/11',
        body: { note: 'Ein Klassiker mit Popcorn', startsAt: null, endsAt: null },
      }),
    ]);
    await waitFor(() => expect(screen.getAllByText('geändert')).toHaveLength(1));
    expect(within(heat).getByRole('button', { name: 'Heat speichern' })).toHaveFocus();
    expect(within(heat).getByLabelText('Notiz')).toHaveValue('Ein Klassiker mit Popcorn');

    // «Verwerfen» drops Alien's draft: the field shows what the server holds again.
    await user.click(within(alien).getByRole('button', { name: 'Änderungen an Alien verwerfen' }));
    expect(within(alien).getByLabelText('Notiz')).toHaveValue('');
    expect(screen.queryByText('geändert')).toBeNull();
    await waitFor(() =>
      expect(within(alien).getByRole('button', { name: 'Alien speichern' })).toHaveFocus(),
    );

    // Without drafts, leaving asks nothing.
    await user.click(
      within(screen.getByRole('navigation', { name: 'Verwaltung' })).getByRole('link', {
        name: /^Sammlungen/,
      }),
    );
    expect(await screen.findByRole('heading', { level: 1, name: 'Sammlungen' })).toBeVisible();
    expect(screen.queryByRole('alertdialog')).toBeNull();
  }, 15_000);

  it('no refetch on window focus while a dialog is open or an open fold-out has a draft; a folded draft does not hold it', async () => {
    const { calls } = mockApi({ ...routes, 'GET /admin/picks': () => json([heatPick]) });
    renderApp('/admin/picks');
    const user = userEvent.setup();
    const lists = () => calls.filter((call) => call.path === '/admin/picks').length;
    const refocus = () =>
      act(() => {
        focusManager.setFocused(false);
        focusManager.setFocused(true);
      });
    /** Gives a refetch the chance it must not take. */
    const settle = () => act(() => new Promise((resolve) => window.setTimeout(resolve, 50)));
    try {
      const detail = await openPick(user, 'Heat');
      // The confirmation holds the list.
      await user.click(within(detail).getByRole('button', { name: 'Heat entfernen' }));
      const dialog = await screen.findByRole('alertdialog');
      let before = lists();
      refocus();
      await settle();
      expect(lists()).toBe(before);
      await user.click(within(dialog).getByRole('button', { name: 'Abbrechen' }));
      await waitFor(() => expect(dialog).not.toBeInTheDocument());

      // «Hinzufügen» holds it too.
      await user.click(screen.getByRole('button', { name: 'Hinzufügen' }));
      const add = await screen.findByRole('dialog', { name: 'Empfehlung hinzufügen' });
      before = lists();
      refocus();
      await settle();
      expect(lists()).toBe(before);
      await user.keyboard('{Escape}');
      await waitFor(() => expect(add).not.toBeInTheDocument());

      // A draft in the open fold-out holds it; folded away, the draft stays and the list is free.
      await user.type(within(detail).getByLabelText('Notiz'), '!');
      before = lists();
      refocus();
      await settle();
      expect(lists()).toBe(before);
      await user.click(await toggleOf('Heat'));
      refocus();
      await waitFor(() => expect(lists()).toBeGreaterThan(before));
      const again = await openPick(user, 'Heat');
      expect(within(again).getByLabelText('Notiz')).toHaveValue('Ein Klassiker!');
    } finally {
      focusManager.setFocused(undefined);
    }
  }, 15_000);

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

    const detail = await openPick(user, 'Heat');
    expect(within(detail).getByText('Für')).toBeVisible();
    expect(within(detail).getByText('Alle')).toBeVisible();
    const remove = within(detail).getByRole('button', { name: 'Heat entfernen' });
    // Escape keeps the pick; the focus goes back to «Entfernen».
    await user.click(remove);
    await screen.findByRole('alertdialog', { name: "Heat aus Cat's Empfehlungen entfernen?" });
    await user.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull());
    await waitFor(() => expect(remove).toHaveFocus());
    expect(calls.some((call) => call.method === 'DELETE')).toBe(false);

    await user.click(remove);
    const confirm = await screen.findByRole('alertdialog', {
      name: "Heat aus Cat's Empfehlungen entfernen?",
    });
    expect(
      within(confirm).getByText(
        'Der Titel bleibt in der Bibliothek, nur die Empfehlung verschwindet.',
      ),
    ).toBeVisible();
    await user.click(within(confirm).getByRole('button', { name: 'Entfernen' }));
    expect(await screen.findByText('Heat ist nicht mehr empfohlen.')).toBeVisible();
    expect(calls.some((call) => call.method === 'DELETE')).toBe(true);
    await waitFor(() => expect(screen.queryByRole('table')).not.toBeInTheDocument());
    await waitFor(() =>
      expect(screen.getByRole('heading', { level: 1, name: "Cat's Empfehlungen" })).toHaveFocus(),
    );
  });

  it('a failed removal shows the problem and keeps the pick', async () => {
    mockApi({
      ...routes,
      'GET /admin/picks': () => json([heatPick]),
      'DELETE /admin/picks/11': () => problem(500, 'internal'),
    });
    renderApp('/admin/picks');
    const user = userEvent.setup();

    const detail = await openPick(user, 'Heat');
    await user.click(within(detail).getByRole('button', { name: 'Heat entfernen' }));
    const confirm = await screen.findByRole('alertdialog');
    await user.click(within(confirm).getByRole('button', { name: 'Entfernen' }));
    expect(await screen.findByRole('alert')).toBeVisible();
    expect(screen.getByRole('rowheader', { name: /Heat/ })).toBeVisible();
  });

  it('refuses anyone but an administrator in the primary profile', async () => {
    const { calls } = mockApi({
      ...routes,
      'GET /auth/session': () =>
        json({ ...session, account: { ...session.account, role: 'user' } }),
    });
    renderApp('/admin/picks');
    expect(await screen.findByText('Nur für Administratoren')).toBeVisible();
    expect(calls.some((call) => call.path === '/admin/picks')).toBe(false);
  });

  it('the list fails to load: the error offers «Erneut versuchen»', async () => {
    let fail = true;
    mockApi({
      ...routes,
      'GET /admin/picks': () => (fail ? problem(500, 'internal') : json([heatPick])),
    });
    renderApp('/admin/picks');
    const user = userEvent.setup();
    const retry = await screen.findByRole('button', { name: 'Erneut versuchen' });
    fail = false;
    await user.click(retry);
    expect(
      await screen.findByRole('table', { name: 'Empfehlungen in der Reihenfolge der Reihe' }),
    ).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Erneut versuchen' })).not.toBeInTheDocument();
  });
});
