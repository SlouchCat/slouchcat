import type { HomeRowSetting, TraktStatus } from '@slouchcat/api-client';
import { focusManager } from '@tanstack/react-query';
import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, session, signedIn } from '@/test/api';
import { adminShell, renderAdminPage } from '@/test/admin';

// The start page rows (M7, E-9; concept 12, variant C): tabs «Reihenfolge» and «Vorschau» in the
// band, a position per row (RankSelect, S-16), «Ausblenden», a fold-out with what the row shows,
// and one SaveBar for everything. The server stores and applies the order; the page only edits it.

const keys = [
  'spotlight',
  'picks',
  'continue',
  'recent',
  'released',
  'watchlist',
  'collections',
  'genres',
] as const;
const defaultRows: HomeRowSetting[] = keys.map((key, position) => ({
  key,
  position,
  hidden: false,
}));
const configured: TraktStatus = {
  configured: true,
  lastSuccess: '2026-10-20T06:00:00Z',
  lastAttempt: '2026-10-20T06:00:00Z',
  listKey: 'trending',
  entries: 40,
};
const names = [
  'Spotlight',
  "Cat's Empfehlungen",
  'Weiterschauen',
  'Neu hinzugefügt',
  'Kürzlich veröffentlicht',
  'Merkliste',
  'Sammlungen',
  'Genres',
];

const table = () => screen.getByRole('table', { name: 'Reihen der Startseite' });
const rowNames = () =>
  within(table())
    .getAllByRole('rowheader')
    .map((cell) => cell.textContent);
const rank = (name: string) => screen.getByRole('combobox', { name: `Platz für ${name}` });
const saveBar = () => screen.getByRole('region', { name: 'Speichern' });
const previewNames = () =>
  within(screen.getByRole('list', { name: 'Vorschau der Startseite' }))
    .getAllByRole('listitem')
    .map((item) => item.textContent);

/** A server that keeps what it is sent. */
function storingRoutes() {
  let stored = defaultRows;
  return {
    ...signedIn,
    ...adminShell,
    'GET /admin/home-rows': () => json(stored),
    'PUT /admin/home-rows': (call: { body: unknown }) => {
      stored = call.body as HomeRowSetting[];
      return json(stored);
    },
  };
}

describe('admin start page rows (M7, E-9; concept 12)', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('places and hides rows and saves all eight at once', async () => {
    const { calls } = mockApi(storingRoutes());
    const { nav } = await renderAdminPage('/admin/home-rows');
    const user = userEvent.setup();

    await screen.findByRole('table', { name: 'Reihen der Startseite' });
    expect(screen.getByRole('heading', { level: 1, name: 'Startseite' })).toBeVisible();
    expect(rowNames()).toEqual(names);
    // No arrow buttons any more (S-16): every row has a position to choose.
    expect(screen.queryByRole('button', { name: /nach oben|nach unten/ })).toBeNull();
    expect(rank('Spotlight')).toHaveValue('0');
    expect(rank('Genres')).toHaveValue('7');
    expect(rank('Neu hinzugefügt')).toHaveAccessibleDescription(
      'Die anderen Einträge rücken nach.',
    );
    expect(within(saveBar()).getByText('Gespeichert')).toBeVisible();
    expect(within(saveBar()).getByRole('button', { name: 'Speichern' })).toBeDisabled();
    expect(within(saveBar()).getByRole('button', { name: 'Verwerfen' })).toBeDisabled();

    // One up: «Neu hinzugefügt» and «Weiterschauen» trade places; the select keeps the focus.
    await user.click(rank('Neu hinzugefügt'));
    await user.selectOptions(rank('Neu hinzugefügt'), 'Platz 3');
    expect(rowNames().slice(1, 4)).toEqual([
      "Cat's Empfehlungen",
      'Neu hinzugefügt',
      'Weiterschauen',
    ]);
    await waitFor(() => expect(rank('Neu hinzugefügt')).toHaveFocus());
    expect(screen.getByRole('status')).toHaveTextContent(
      'Neu hinzugefügt steht jetzt auf Platz 3. Speichere, damit es gilt.',
    );
    expect(within(saveBar()).getByText('Ungespeicherte Änderungen')).toBeVisible();

    // Far down: the rows in between move up by one.
    await user.selectOptions(rank('Spotlight'), 'Platz 6');
    expect(rowNames()).toEqual([
      "Cat's Empfehlungen",
      'Neu hinzugefügt',
      'Weiterschauen',
      'Kürzlich veröffentlicht',
      'Merkliste',
      'Spotlight',
      'Sammlungen',
      'Genres',
    ]);
    await waitFor(() => expect(rank('Spotlight')).toHaveFocus());
    await user.selectOptions(rank('Spotlight'), 'Platz 1');
    expect(rowNames()[0]).toBe('Spotlight');

    const genres = within(table()).getByRole('checkbox', { name: 'Genres ausblenden' });
    await user.click(genres);
    expect(genres).toBeChecked();
    const genresRow = within(table()).getByRole('rowheader', { name: 'Genres' }).closest('tr')!;
    expect(genresRow).toHaveTextContent('ausgeblendet');
    // Nothing is stored before «Speichern».
    expect(calls.some((call) => call.method === 'PUT')).toBe(false);

    await user.click(within(saveBar()).getByRole('button', { name: 'Speichern' }));
    expect(await screen.findByText('Die Startseite ist gespeichert.')).toBeVisible();
    const body = calls.find((call) => call.method === 'PUT')?.body as HomeRowSetting[];
    expect(body.map((row) => row.key)).toEqual([
      'spotlight',
      'picks',
      'recent',
      'continue',
      'released',
      'watchlist',
      'collections',
      'genres',
    ]);
    expect(body.map((row) => row.position)).toEqual([0, 1, 2, 3, 4, 5, 6, 7]);
    expect(body.find((row) => row.key === 'genres')?.hidden).toBe(true);
    expect(body.filter((row) => row.hidden)).toHaveLength(1);
    // Saved: the bar says so, and the focus left with the disabled «Speichern» goes to the h1.
    expect(within(saveBar()).getByText('Gespeichert')).toBeVisible();
    await waitFor(() =>
      expect(screen.getByRole('heading', { level: 1, name: 'Startseite' })).toHaveFocus(),
    );
    expect(nav.querySelector('[aria-current="page"]')).toHaveTextContent('Startseite');
  });

  it('a row placed back where it was is no change', async () => {
    mockApi(storingRoutes());
    renderApp('/admin/home-rows');
    const user = userEvent.setup();
    await screen.findByRole('table', { name: 'Reihen der Startseite' });
    await user.selectOptions(rank('Merkliste'), 'Platz 2');
    expect(within(saveBar()).getByText('Ungespeicherte Änderungen')).toBeVisible();
    await user.selectOptions(rank('Merkliste'), 'Platz 6');
    expect(rowNames()).toEqual(names);
    expect(within(saveBar()).getByText('Gespeichert')).toBeVisible();
    await user.click(within(table()).getByRole('checkbox', { name: 'Merkliste ausblenden' }));
    await user.click(within(table()).getByRole('checkbox', { name: 'Merkliste ausblenden' }));
    expect(within(saveBar()).getByText('Gespeichert')).toBeVisible();
  });

  it('«Verwerfen» asks, then brings back what is saved', async () => {
    const { calls } = mockApi(storingRoutes());
    renderApp('/admin/home-rows');
    const user = userEvent.setup();
    await screen.findByRole('table', { name: 'Reihen der Startseite' });
    await user.selectOptions(rank('Genres'), 'Platz 1');
    await user.click(within(table()).getByRole('checkbox', { name: 'Spotlight ausblenden' }));
    expect(rowNames()[0]).toBe('Genres');

    // Cancel keeps the changes.
    await user.click(within(saveBar()).getByRole('button', { name: 'Verwerfen' }));
    let dialog = await screen.findByRole('alertdialog', { name: 'Änderungen verwerfen?' });
    await user.click(within(dialog).getByRole('button', { name: 'Abbrechen' }));
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    expect(rowNames()[0]).toBe('Genres');
    // «Abbrechen» gives the focus back to «Verwerfen», which is still enabled.
    await waitFor(() =>
      expect(within(saveBar()).getByRole('button', { name: 'Verwerfen' })).toHaveFocus(),
    );

    await user.click(within(saveBar()).getByRole('button', { name: 'Verwerfen' }));
    dialog = await screen.findByRole('alertdialog', { name: 'Änderungen verwerfen?' });
    const confirm = within(dialog).getByRole('button', { name: 'Verwerfen' });
    expect(confirm).toHaveClass(/danger/);
    await user.click(confirm);
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    expect(rowNames()).toEqual(names);
    expect(
      within(table()).getByRole('checkbox', { name: 'Spotlight ausblenden' }),
    ).not.toBeChecked();
    expect(within(saveBar()).getByText('Gespeichert')).toBeVisible();
    expect(screen.getByRole('status')).toHaveTextContent('Die Änderungen sind verworfen.');
    // «Verwerfen» went disabled: the h1 takes the focus.
    await waitFor(() =>
      expect(screen.getByRole('heading', { level: 1, name: 'Startseite' })).toHaveFocus(),
    );
    expect(calls.some((call) => call.method === 'PUT')).toBe(false);
  });

  it('announces the same sentence again in a node of its own', async () => {
    mockApi(storingRoutes());
    renderApp('/admin/home-rows');
    const user = userEvent.setup();
    await screen.findByRole('table', { name: 'Reihen der Startseite' });
    const discard = async () => {
      await user.click(within(saveBar()).getByRole('button', { name: 'Verwerfen' }));
      const dialog = await screen.findByRole('alertdialog', { name: 'Änderungen verwerfen?' });
      await user.click(within(dialog).getByRole('button', { name: 'Verwerfen' }));
      await waitFor(() => expect(dialog).not.toBeInTheDocument());
      return within(screen.getByRole('status')).getByText('Die Änderungen sind verworfen.');
    };
    await user.selectOptions(rank('Genres'), 'Platz 1');
    const first = await discard();
    await user.selectOptions(rank('Genres'), 'Platz 2');
    await user.selectOptions(rank('Genres'), 'Platz 1');
    const second = await discard();
    expect(second).not.toBe(first);
  });

  it('the preview shows the unsaved order; the draft stays across tabs (S-17)', async () => {
    mockApi(storingRoutes());
    const { router } = renderApp('/admin/home-rows');
    const user = userEvent.setup();
    await screen.findByRole('table', { name: 'Reihen der Startseite' });
    expect(screen.getByRole('tab', { selected: true })).toHaveAccessibleName(
      'Reihenfolge 8 Reihen',
    );
    await user.selectOptions(rank('Merkliste'), 'Platz 1');
    await user.click(within(table()).getByRole('checkbox', { name: 'Genres ausblenden' }));

    await user.click(screen.getByRole('tab', { name: 'Vorschau 7 sichtbar' }));
    await waitFor(() => expect(router.state.location.search).toEqual({ tab: 'preview' }));
    expect(previewNames()).toEqual([
      '1. Merkliste',
      '2. Spotlight',
      "3. Cat's Empfehlungen",
      '4. Weiterschauen',
      '5. Neu hinzugefügt',
      '6. Kürzlich veröffentlicht',
      '7. Sammlungen',
      '8. Genres ausgeblendet',
    ]);
    expect(screen.getByText(/Schema mit Platzhaltern/)).toBeVisible();
    // Changing the tab is not leaving the page: no question.
    expect(screen.queryByRole('alertdialog')).toBeNull();

    await user.click(screen.getByRole('tab', { name: /^Reihenfolge/ }));
    await waitFor(() => expect(router.state.location.search).toEqual({}));
    expect(rowNames()[0]).toBe('Merkliste');
    expect(within(table()).getByRole('checkbox', { name: 'Genres ausblenden' })).toBeChecked();
    expect(within(saveBar()).getByText('Ungespeicherte Änderungen')).toBeVisible();
  });

  it('opens a linked tab; an unknown ?tab= shows «Reihenfolge» and leaves the address', async () => {
    mockApi(storingRoutes());
    const linked = renderApp('/admin/home-rows?tab=preview');
    expect(await screen.findByRole('tab', { selected: true })).toHaveAccessibleName(
      'Vorschau 8 sichtbar',
    );
    expect(previewNames()[0]).toBe('1. Spotlight');
    linked.unmount();

    const { router } = renderApp('/admin/home-rows?tab=nonsense');
    expect(await screen.findByRole('tab', { selected: true })).toHaveAccessibleName(
      'Reihenfolge 8 Reihen',
    );
    await waitFor(() => expect(router.state.location.search).toEqual({}));
    // Replaced, not pushed: going back does not return to the unknown tab.
    expect(router.history.length).toBe(1);
  });

  it('folds a row out with what it shows and the page that keeps its content', async () => {
    mockApi(storingRoutes());
    renderApp('/admin/home-rows');
    const user = userEvent.setup();
    await screen.findByRole('table', { name: 'Reihen der Startseite' });
    const picks = within(table()).getByRole('button', { name: "Cat's Empfehlungen" });
    expect(picks).toHaveAttribute('aria-expanded', 'false');
    picks.focus();
    await user.keyboard('{Enter}');
    expect(picks).toHaveAttribute('aria-expanded', 'true');
    const detail = document.getElementById(picks.getAttribute('aria-controls') ?? '')!;
    expect(detail).toHaveTextContent('Von Hand gepflegt, mit Notiz, Zielprofil und Zeitfenster.');
    expect(
      within(detail).getByRole('link', { name: "Zur Seite «Cat's Empfehlungen»" }),
    ).toHaveAttribute('href', '/admin/picks');

    // A row without a page of its own has only its description.
    const watchlist = within(table()).getByRole('button', { name: 'Merkliste' });
    await user.click(watchlist);
    const watchDetail = document.getElementById(watchlist.getAttribute('aria-controls') ?? '')!;
    expect(watchDetail).toHaveTextContent('Die Merkliste des Profils.');
    expect(within(watchDetail).queryByRole('link')).toBeNull();

    // The open rows stay open across the preview, and move with their row.
    await user.click(screen.getByRole('tab', { name: /^Vorschau/ }));
    await user.click(screen.getByRole('tab', { name: /^Reihenfolge/ }));
    expect(within(table()).getByRole('button', { name: "Cat's Empfehlungen" })).toHaveAttribute(
      'aria-expanded',
      'true',
    );
    await user.selectOptions(rank("Cat's Empfehlungen"), 'Platz 8');
    expect(within(table()).getByRole('button', { name: "Cat's Empfehlungen" })).toHaveAttribute(
      'aria-expanded',
      'true',
    );

    // Space folds out too.
    const spotlight = within(table()).getByRole('button', { name: 'Spotlight' });
    spotlight.focus();
    await user.keyboard(' ');
    expect(spotlight).toHaveAttribute('aria-expanded', 'true');
  });

  it('asks before leaving the page with unsaved changes (S-13)', async () => {
    mockApi(storingRoutes());
    const { router } = renderApp('/admin/home-rows');
    const user = userEvent.setup();
    await screen.findByRole('table', { name: 'Reihen der Startseite' });
    await user.selectOptions(rank('Genres'), 'Platz 1');

    void router.navigate({ to: '/admin/spotlight' });
    const leave = await screen.findByRole('alertdialog', {
      name: 'Seite mit ungespeicherten Änderungen verlassen?',
    });
    await user.click(within(leave).getByRole('button', { name: 'Auf der Seite bleiben' }));
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull());
    expect(router.state.location.pathname).toBe('/admin/home-rows');
    expect(rowNames()[0]).toBe('Genres');

    void router.navigate({ to: '/admin/spotlight' });
    const again = await screen.findByRole('alertdialog', {
      name: 'Seite mit ungespeicherten Änderungen verlassen?',
    });
    await user.click(within(again).getByRole('button', { name: 'Verwerfen und verlassen' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/admin/spotlight'));
  });

  it('does not refetch on window focus while the list is changed', async () => {
    const { calls } = mockApi(storingRoutes());
    renderApp('/admin/home-rows');
    const user = userEvent.setup();
    const lists = () =>
      calls.filter((call) => call.method === 'GET' && call.path === '/admin/home-rows').length;
    const refocus = () =>
      act(() => {
        focusManager.setFocused(false);
        focusManager.setFocused(true);
      });
    const settle = () => act(() => new Promise((resolve) => window.setTimeout(resolve, 50)));
    try {
      await screen.findByRole('table', { name: 'Reihen der Startseite' });
      await user.selectOptions(rank('Genres'), 'Platz 1');
      const before = lists();
      refocus();
      await settle();
      expect(lists()).toBe(before);

      await user.click(within(saveBar()).getByRole('button', { name: 'Verwerfen' }));
      const dialog = await screen.findByRole('alertdialog', { name: 'Änderungen verwerfen?' });
      await user.click(within(dialog).getByRole('button', { name: 'Verwerfen' }));
      await waitFor(() => expect(dialog).not.toBeInTheDocument());
      refocus();
      await waitFor(() => expect(lists()).toBeGreaterThan(before));
    } finally {
      focusManager.setFocused(undefined);
    }
  });

  it('a refused save shows the problem and keeps the changes', async () => {
    mockApi({
      ...storingRoutes(),
      'PUT /admin/home-rows': () => problem(500, 'internal'),
    });
    renderApp('/admin/home-rows');
    const user = userEvent.setup();
    await screen.findByRole('table', { name: 'Reihen der Startseite' });
    await user.selectOptions(rank('Genres'), 'Platz 1');
    await user.click(within(saveBar()).getByRole('button', { name: 'Speichern' }));
    expect(await screen.findByRole('alert')).toBeVisible();
    expect(rowNames()[0]).toBe('Genres');
    expect(within(saveBar()).getByText('Ungespeicherte Änderungen')).toBeVisible();
    // The next change clears the problem.
    await user.selectOptions(rank('Genres'), 'Platz 2');
    await waitFor(() => expect(screen.queryByRole('alert')).toBeNull());
  });

  it('locks the positions and «Ausblenden» while saving', async () => {
    // The server answers only when the test says so: the save stays pending meanwhile.
    let answer: () => void = () => undefined;
    mockApi({
      ...storingRoutes(),
      'PUT /admin/home-rows': (call) =>
        new Promise<Response>((resolve) => {
          answer = () => resolve(json(call.body));
        }),
    });
    renderApp('/admin/home-rows');
    const user = userEvent.setup();
    await screen.findByRole('table', { name: 'Reihen der Startseite' });
    await user.selectOptions(rank('Genres'), 'Platz 1');
    await user.click(within(saveBar()).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(rank('Genres')).toBeDisabled());
    expect(within(table()).getByRole('checkbox', { name: 'Genres ausblenden' })).toBeDisabled();
    answer();
    expect(await screen.findByText('Die Startseite ist gespeichert.')).toBeVisible();
    await waitFor(() => expect(rank('Genres')).toBeEnabled());
    expect(within(table()).getByRole('checkbox', { name: 'Genres ausblenden' })).toBeEnabled();
  });

  it.each([
    ['an account without administration', { account: { role: 'user', isOwner: false } }],
    ['a profile that is not the primary one', { profile: { isPrimary: false } }],
  ])('tells %s that the page is not theirs, without asking the server', async (_, change) => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /auth/session': () =>
        json({
          ...session,
          account: { ...session.account, ...('account' in change ? change.account : {}) },
          profile: { ...session.profile, ...('profile' in change ? change.profile : {}) },
        }),
    });
    renderApp('/admin/home-rows');
    expect(await screen.findByText('Nur für Administratoren')).toBeVisible();
    expect(screen.getByRole('heading', { level: 1, name: 'Startseite' })).toBeVisible();
    expect(screen.queryByRole('table')).toBeNull();
    expect(calls.some((call) => call.path === '/admin/home-rows')).toBe(false);
  });

  it('leaves the Trakt status to the spotlight page and links there in one line', async () => {
    // The status endpoint still answers, so a status table would have data to show.
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/home-rows': () => json(defaultRows),
      'GET /admin/trakt': () => json(configured),
    });
    renderApp('/admin/home-rows');

    await screen.findByRole('table', { name: 'Reihen der Startseite' });
    const link = screen.getByRole('link', { name: 'Zur Seite «Spotlight»' });
    expect(link).toHaveAttribute('href', '/admin/spotlight');
    expect(link.parentElement).toHaveTextContent(/Welche Trakt-Liste Spotlight zeigt/);
    expect(screen.queryByRole('region', { name: 'Trakt' })).toBeNull();
    expect(screen.queryAllByRole('term')).toHaveLength(0);
    expect(screen.queryByText('Letzter erfolgreicher Abruf')).toBeNull();
    expect(screen.queryByRole('button', { name: 'Jetzt aktualisieren' })).toBeNull();
    expect(calls.some((call) => call.path === '/admin/trakt')).toBe(false);
  });

  it('keeps the intro behind the help button', async () => {
    mockApi(storingRoutes());
    renderApp('/admin/home-rows');
    const user = userEvent.setup();
    await screen.findByRole('table', { name: 'Reihen der Startseite' });
    const intro = screen.getByText(/Ausgeblendete und leere Reihen erscheinen nicht/);
    expect(intro).not.toBeVisible();
    await user.click(screen.getByRole('button', { name: 'Hilfe zu dieser Seite' }));
    expect(intro).toBeVisible();
  });
});
