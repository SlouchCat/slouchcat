import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { adminPodcast, audioIds, library } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, signedIn } from '@/test/api';
import { adminShell, renderAdminPage } from '@/test/admin';

// /admin/podcasts (M6, H-2; concept 12, variant C): tabs «Alle» and one per podcast library,
// each podcast a row that folds out into author, feed address, library, the whole error and
// the actions (POD-1 to POD-3).

const podcastLibrary = library({
  id: 3,
  name: 'Podcasts',
  kind: 'podcasts',
  defaultRating: 0,
  roots: [],
});

const kidsLibrary = library({
  id: 4,
  name: 'Podcasts Kinder',
  kind: 'podcasts',
  defaultRating: 0,
  roots: [],
});

const routes = {
  ...signedIn,
  'GET /admin/libraries': () => json([library(), podcastLibrary]),
};

/** Folds out the row whose title button is named `name` and returns its fold-out. */
async function openRow(user: ReturnType<typeof userEvent.setup>, name: string | RegExp) {
  const toggle = within(screen.getByRole('tabpanel')).getByRole('button', { name });
  if (toggle.getAttribute('aria-expanded') !== 'true') await user.click(toggle);
  const detail = document.getElementById(toggle.getAttribute('aria-controls') ?? '');
  if (!detail) throw new Error(`no fold-out for ${String(name)}`);
  return detail;
}

describe('admin podcasts page (M6, H-2; concept 12)', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('lists the subscriptions with episodes, last fetch and error; the fold-out has the rest', async () => {
    mockApi({
      ...routes,
      ...adminShell,
      'GET /admin/podcasts': () =>
        json([
          adminPodcast,
          {
            ...adminPodcast,
            id: audioIds.ep1,
            title: 'Kaputter Feed',
            author: undefined,
            explicit: true,
            lastCheckedAt: undefined,
            lastError: 'feed_unreachable: HTTP 404',
          },
        ]),
    });
    const { nav } = await renderAdminPage('/admin/podcasts');
    const user = userEvent.setup();

    expect(await screen.findByRole('heading', { level: 1, name: 'Podcasts' })).toBeVisible();
    const table = await screen.findByRole('table', { name: 'Abonnierte Podcasts' });
    const rows = within(table).getAllByRole('row');
    // Header, then each podcast with its (closed) fold-out row.
    expect(within(rows[1]!).getByRole('rowheader')).toHaveTextContent('Slouchcat Test Podcast');
    expect(within(rows[1]!).getByText('2')).toBeVisible();
    expect(within(rows[1]!).getByRole('cell', { name: '–' })).toBeVisible();
    const broken = within(table).getByRole('row', { name: /^Kaputter Feed/ });
    expect(within(broken).getByText('Noch nie')).toBeVisible();
    expect(within(broken).getByText('Explizit')).toBeVisible();
    // The error in one line, the whole text as its tooltip.
    const error = within(broken).getByText(
      'Der Feed ist nicht erreichbar. Prüfe die Adresse und versuche es erneut.',
    );
    expect(error).toBeVisible();
    expect(error.closest('[title]')).toHaveAttribute(
      'title',
      'Der Feed ist nicht erreichbar. Prüfe die Adresse und versuche es erneut.',
    );
    expect(within(broken).queryByText('HTTP 404')).not.toBeInTheDocument();

    const detail = await openRow(user, /^Slouchcat Test Podcast/);
    expect(within(detail).getByText('Slouchcat Test Author')).toBeVisible();
    expect(within(detail).getByText('http://127.0.0.1:8095/feed.xml')).toBeVisible();
    expect(within(detail).getByText('Podcasts')).toBeVisible();

    // The fold-out of the broken feed: the whole message and the server's detail; no author.
    const brokenDetail = await openRow(user, /^Kaputter Feed/);
    expect(
      within(brokenDetail).getByText(
        'Der Feed ist nicht erreichbar. Prüfe die Adresse und versuche es erneut.',
      ),
    ).toBeVisible();
    expect(within(brokenDetail).getByText('HTTP 404')).toBeVisible();
    expect(within(brokenDetail).queryByText('Autor')).not.toBeInTheDocument();
    expect(nav.querySelector('[aria-current="page"]')).toHaveTextContent('Podcasts');
  });

  it('marks podcasts from folders and keeps their feed, which the folder decides', async () => {
    mockApi({
      ...routes,
      'GET /admin/podcasts': () =>
        json([
          { ...adminPodcast, feedOrigin: 'admin' },
          {
            ...adminPodcast,
            id: audioIds.ep1,
            title: 'Ordner-Podcast',
            feedOrigin: 'file',
            rootId: 7,
            localEpisodeCount: 3,
          },
          {
            ...adminPodcast,
            id: audioIds.ep2,
            title: 'Ohne Feed',
            feedUrl: undefined,
            localEpisodeCount: 1,
          },
        ]),
    });
    renderApp('/admin/podcasts');
    const user = userEvent.setup();

    const table = await screen.findByRole('table', { name: 'Abonnierte Podcasts' });
    const subscribed = within(table).getByRole('row', { name: /^Slouchcat Test Podcast/ });
    expect(within(subscribed).queryByText('aus Ordner')).not.toBeInTheDocument();
    expect(within(subscribed).queryByText(/lokal$/)).not.toBeInTheDocument();
    const subscribedDetail = await openRow(user, /^Slouchcat Test Podcast/);
    expect(
      within(subscribedDetail).getByRole('button', { name: 'Abo Slouchcat Test Podcast löschen' }),
    ).toBeVisible();

    const folder = within(table).getByRole('row', { name: /^Ordner-Podcast/ });
    expect(within(folder).getByText('aus Ordner')).toBeVisible();
    expect(within(folder).getByText('3 lokal')).toBeVisible();
    const folderDetail = await openRow(user, /^Ordner-Podcast/);
    expect(within(folderDetail).getByText('http://127.0.0.1:8095/feed.xml')).toBeVisible();
    expect(
      within(folderDetail).getByRole('button', { name: 'Ordner-Podcast jetzt abrufen' }),
    ).toBeVisible();
    expect(within(folderDetail).queryByRole('button', { name: /löschen/ })).not.toBeInTheDocument();
    expect(within(folderDetail).getByText(/^Ein Ordner trägt diesen Podcast/)).toBeVisible();

    // Without a feed there is no address and nothing to fetch; without a folder that owns the
    // podcast (no rootId) it may be removed.
    const noFeed = within(table).getByRole('row', { name: /^Ohne Feed/ });
    expect(within(noFeed).getByText('1 lokal')).toBeVisible();
    const noFeedDetail = await openRow(user, /^Ohne Feed/);
    expect(within(noFeedDetail).queryByText(/^http/)).not.toBeInTheDocument();
    expect(
      within(noFeedDetail).getByText('Kein Feed, die Episoden kommen aus dem Ordner.'),
    ).toBeVisible();
    expect(within(noFeedDetail).queryByRole('button', { name: /abrufen/ })).not.toBeInTheDocument();
    expect(
      within(noFeedDetail).getByRole('button', { name: 'Abo Ohne Feed löschen' }),
    ).toBeVisible();
  });

  it('filters by podcast library in tabs with counts, kept in ?tab=; an open row stays open', async () => {
    mockApi({
      ...routes,
      'GET /admin/libraries': () => json([library(), podcastLibrary, kidsLibrary]),
      'GET /admin/podcasts': () =>
        json([
          adminPodcast,
          { ...adminPodcast, id: audioIds.ep1, title: 'Gutenacht-Geschichten', libraryId: 4 },
          { ...adminPodcast, id: audioIds.ep2, title: 'Echo der Zeit' },
        ]),
    });
    const { router } = renderApp('/admin/podcasts?tab=library-4');
    const user = userEvent.setup();

    const tabs = await screen.findByRole('tablist', { name: 'Podcasts nach Bibliothek' });
    expect(
      within(tabs)
        .getAllByRole('tab')
        .map((tab) => tab.textContent),
    ).toEqual(['Alle 33 Podcasts', 'Podcasts 22 Podcasts', 'Podcasts Kinder 11 Podcast']);
    expect(within(tabs).getByRole('tab', { selected: true })).toHaveAccessibleName(
      'Podcasts Kinder 1 Podcast',
    );
    const kids = screen.getByRole('table', { name: 'Abonnierte Podcasts in Podcasts Kinder' });
    expect(
      within(kids)
        .getAllByRole('rowheader')
        .map((cell) => cell.textContent),
    ).toEqual(['Gutenacht-Geschichten']);
    const detail = await openRow(user, 'Gutenacht-Geschichten');
    expect(within(detail).getByText('Podcasts Kinder')).toBeVisible();

    // «Alle» drops the parameter; the row opened in the other tab is open here too.
    await user.click(within(tabs).getByRole('tab', { name: /^Alle/ }));
    await waitFor(() => expect(router.state.location.search).toEqual({}));
    const all = screen.getByRole('table', { name: 'Abonnierte Podcasts' });
    expect(within(all).getAllByRole('rowheader')).toHaveLength(3);
    expect(within(all).getByRole('button', { name: 'Gutenacht-Geschichten' })).toHaveAttribute(
      'aria-expanded',
      'true',
    );
    await user.click(within(tabs).getByRole('tab', { name: /^Podcasts 2/ }));
    await waitFor(() => expect(router.state.location.search).toEqual({ tab: 'library-3' }));
    // Not quoted, as a bare number would be.
    expect(router.state.location.href).toBe('/admin/podcasts?tab=library-3');
    expect(
      within(screen.getByRole('tabpanel')).queryByText('Gutenacht-Geschichten'),
    ).not.toBeInTheDocument();
  });

  it('an unknown ?tab= shows «Alle» and leaves the address', async () => {
    mockApi({ ...routes, 'GET /admin/podcasts': () => json([adminPodcast]) });
    const { router } = renderApp('/admin/podcasts?tab=99');
    const tab = await screen.findByRole('tab', { selected: true });
    expect(tab).toHaveAccessibleName('Alle 1 Podcast');
    await waitFor(() => expect(router.state.location.search).toEqual({}));
    // Replaced, not pushed: going back does not return to the unknown tab.
    expect(router.history.length).toBe(1);
  });

  it('a library without podcasts says so in its tab', async () => {
    mockApi({
      ...routes,
      'GET /admin/libraries': () => json([podcastLibrary, kidsLibrary]),
      'GET /admin/podcasts': () => json([adminPodcast]),
    });
    renderApp('/admin/podcasts?tab=library-4');
    expect(await screen.findByText('In dieser Bibliothek ist noch kein Podcast.')).toBeVisible();
    expect(screen.queryByRole('table')).not.toBeInTheDocument();
  });

  it('subscribes a feed: a feed problem shows at the URL field, a good one adds the podcast', async () => {
    let fail = true;
    const { calls } = mockApi({
      ...routes,
      'GET /admin/podcasts': () => json(fail ? [] : [adminPodcast]),
      'POST /admin/podcasts': () => {
        if (fail) return problem(422, 'feed_unreachable');
        return json(adminPodcast, 201);
      },
    });
    renderApp('/admin/podcasts');
    const user = userEvent.setup();

    expect(await screen.findByText('Noch kein Podcast abonniert.')).toBeVisible();
    await user.click(screen.getByRole('button', { name: 'Feed abonnieren' }));
    const dialog = await screen.findByRole('dialog', { name: 'Feed abonnieren' });
    const url = within(dialog).getByRole('textbox', { name: 'Feed-Adresse' });
    await waitFor(() => expect(url).toHaveFocus());
    expect(within(dialog).getByRole('radio', { name: 'Podcasts' })).toBeChecked();

    // Checked in the browser first: only http and https.
    await user.type(url, 'ftp://example.org/feed.xml');
    await user.click(within(dialog).getByRole('button', { name: 'Abonnieren' }));
    expect(url).toHaveAccessibleDescription(/http:\/\/ oder https:\/\//);
    expect(calls.some((call) => call.method === 'POST')).toBe(false);

    await user.clear(url);
    await user.type(url, 'http://127.0.0.1:8095/missing.xml');
    await user.click(within(dialog).getByRole('button', { name: 'Abonnieren' }));
    await waitFor(() => expect(url).toHaveAccessibleDescription(/Der Feed ist nicht erreichbar/));
    expect(calls.find((call) => call.method === 'POST')?.body).toEqual({
      libraryId: 3,
      feedUrl: 'http://127.0.0.1:8095/missing.xml',
    });

    fail = false;
    await user.clear(url);
    await user.type(url, 'http://127.0.0.1:8095/feed.xml');
    await user.click(within(dialog).getByRole('button', { name: 'Abonnieren' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(screen.getByText('Slouchcat Test Podcast ist abonniert.')).toBeVisible();
    expect(await screen.findByRole('rowheader', { name: /Slouchcat Test Podcast/ })).toBeVisible();
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Feed abonnieren' })).toHaveFocus(),
    );
  });

  it('"Jetzt abrufen" queues a fetch and polls; "Abo löschen" asks first, pausing the polling', async () => {
    let list = [adminPodcast];
    const { calls } = mockApi({
      ...routes,
      'GET /admin/podcasts': () => json(list),
      [`POST /admin/podcasts/${audioIds.podcast}/refresh`]: () => json(adminPodcast, 202),
      [`DELETE /admin/podcasts/${audioIds.podcast}`]: () => {
        list = [];
        return new Response(null, { status: 204 });
      },
    });
    renderApp('/admin/podcasts');
    const user = userEvent.setup();

    expect(await screen.findByText(/^20\.10\.2026/)).toBeVisible();
    expect(screen.queryByText(/^Live/)).not.toBeInTheDocument();
    const detail = await openRow(user, /^Slouchcat Test Podcast/);
    await user.click(
      within(detail).getByRole('button', { name: 'Slouchcat Test Podcast jetzt abrufen' }),
    );
    expect(await screen.findByText('Slouchcat Test Podcast wird abgerufen.')).toBeVisible();
    expect(calls.some((call) => call.path.endsWith('/refresh'))).toBe(true);
    // Until the last fetch changes, the row says so; the page polls every 2 s meanwhile.
    expect(await screen.findByText('wird abgerufen')).toBeVisible();
    expect(screen.getByText('Live, alle 2 Sekunden')).toBeVisible();

    const removeButton = within(detail).getByRole('button', {
      name: 'Abo Slouchcat Test Podcast löschen',
    });
    await user.click(removeButton);
    const confirm = await screen.findByRole('alertdialog', {
      name: 'Abo Slouchcat Test Podcast löschen?',
    });
    // An open confirmation holds the polling.
    expect(screen.queryByText('Live, alle 2 Sekunden')).not.toBeInTheDocument();
    await user.click(within(confirm).getByRole('button', { name: 'Abo löschen' }));
    expect(await screen.findByText('Das Abo Slouchcat Test Podcast ist gelöscht.')).toBeVisible();
    await waitFor(() => expect(screen.queryByRole('table')).not.toBeInTheDocument());
    expect(calls.some((call) => call.method === 'DELETE')).toBe(true);
    // The row and its buttons are gone: the focus is on the page's heading.
    await waitFor(() =>
      expect(screen.getByRole('heading', { level: 1, name: 'Podcasts' })).toHaveFocus(),
    );
  });

  it('without a podcast library it leads to the libraries first', async () => {
    mockApi({
      ...signedIn,
      'GET /admin/libraries': () => json([library()]),
      'GET /admin/podcasts': () => json([]),
    });
    renderApp('/admin/podcasts');
    expect(
      await screen.findByText('Lege zuerst eine Bibliothek der Art Podcasts an.'),
    ).toBeVisible();
    expect(screen.getByRole('link', { name: 'Zu den Bibliotheken' })).toHaveAttribute(
      'href',
      '/admin/libraries',
    );
    expect(screen.queryByRole('button', { name: 'Feed abonnieren' })).not.toBeInTheDocument();
  });
});
