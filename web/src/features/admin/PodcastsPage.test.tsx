import type { Library, PodcastFolder } from '@slouchcat/api-client';
import { act, screen, waitFor, within } from '@testing-library/react';
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
  'GET /admin/podcasts/downloads': () =>
    json({ total: 0, running: 0, paused: false, downloads: [] }),
};

/** Folds out the row whose title button is named `name` and returns its fold-out. */
async function openRow(user: ReturnType<typeof userEvent.setup>, name: string | RegExp) {
  const toggle = await within(await screen.findByRole('tabpanel')).findByRole('button', { name });
  if (toggle.getAttribute('aria-expanded') !== 'true') await user.click(toggle);
  const detail = document.getElementById(toggle.getAttribute('aria-controls') ?? '');
  if (!detail) throw new Error(`no fold-out for ${String(name)}`);
  return detail;
}

describe('admin podcasts page (M6, H-2; concept 12)', () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('lists the subscriptions with cover, author, episodes and state; the fold-out has the rest', async () => {
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
    // Concept 19 part A, 12.2 (variant A): title with author and «Nur Link», the episodes with
    // the stored share, the state, and the link to the page of the podcast. Sorted by title.
    const rows = within(table).getAllByRole('row');
    expect(rows[1]).toHaveAccessibleName(/^Kaputter Feed/);
    const first = within(within(table).getByRole('row', { name: /^Slouchcat Test Podcast/ }));
    expect(first.getByRole('rowheader')).toHaveTextContent('Slouchcat Test Podcast');
    expect(first.getByRole('rowheader')).toHaveTextContent('Slouchcat Test Author');
    expect(first.getByText('Nur Link')).toBeVisible();
    expect(first.getByText('2')).toBeVisible();
    expect(first.getByText('keine gespeichert')).toBeVisible();
    expect(first.getByText('Aktuell')).toBeVisible();
    expect(
      first.getByRole('link', { name: 'Folgen von Slouchcat Test Podcast ansehen' }),
    ).toHaveAttribute('href', `/admin/podcasts/${audioIds.podcast}`);
    const broken = within(table).getByRole('row', { name: /^Kaputter Feed/ });
    expect(within(broken).getByText('Explizit')).toBeVisible();
    // The row says «Feed-Fehler»; the whole text is in the fold-out.
    expect(within(broken).getByText('Feed-Fehler')).toBeVisible();
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
    expect(within(brokenDetail).getByText('Noch nie')).toBeVisible();
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
    expect(within(subscribed).getByText('keine gespeichert')).toBeVisible();
    const subscribedDetail = await openRow(user, /^Slouchcat Test Podcast/);
    expect(
      within(subscribedDetail).getByRole('button', { name: 'Abo Slouchcat Test Podcast löschen' }),
    ).toBeVisible();

    const folder = within(table).getByRole('row', { name: /^Ordner-Podcast/ });
    expect(within(folder).getByText('aus Ordner')).toBeVisible();
    expect(within(folder).getByText('alle gespeichert')).toBeVisible();
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
    expect(within(noFeed).getByText('1 gespeichert · 1 ohne Datei')).toBeVisible();
    const noFeedDetail = await openRow(user, /^Ohne Feed/);
    expect(within(noFeedDetail).queryByText(/^http/)).not.toBeInTheDocument();
    expect(within(noFeedDetail).getByText('Kein Feed.')).toBeVisible();
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
    ).toEqual([expect.stringMatching(/^Gutenacht-Geschichten/)]);
    const detail = await openRow(user, /^Gutenacht-Geschichten/);
    expect(within(detail).getByText('Podcasts Kinder')).toBeVisible();

    // «Alle» drops the parameter; the row opened in the other tab is open here too.
    await user.click(within(tabs).getByRole('tab', { name: /^Alle/ }));
    await waitFor(() => expect(router.state.location.search).toEqual({}));
    const all = screen.getByRole('table', { name: 'Abonnierte Podcasts' });
    expect(within(all).getAllByRole('rowheader')).toHaveLength(3);
    expect(
      within(all).getByRole('button', { name: /^Gutenacht-Geschichten Slouchcat/ }),
    ).toHaveAttribute('aria-expanded', 'true');
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

  it('a tab of a library that is gone shows «Alle»; a podcast names a library of another kind', async () => {
    mockApi({
      ...routes,
      'GET /admin/podcasts': () =>
        json([adminPodcast, { ...adminPodcast, id: audioIds.ep1, title: 'Verirrt', libraryId: 1 }]),
    });
    const { router } = renderApp('/admin/podcasts?tab=library-9');
    const tabs = await screen.findByRole('tablist', { name: 'Podcasts nach Bibliothek' });
    expect(within(tabs).getByRole('tab', { selected: true })).toHaveAccessibleName(
      'Alle 2 Podcasts',
    );
    await waitFor(() => expect(router.state.location.search).toEqual({}));
    // The video library «Filme» gets a tab, since a podcast still names it.
    expect(
      within(tabs)
        .getAllByRole('tab')
        .map((tab) => tab.textContent),
    ).toEqual(['Alle 22 Podcasts', 'Filme 11 Podcast', 'Podcasts 11 Podcast']);
    const user = userEvent.setup();
    await user.click(within(tabs).getByRole('tab', { name: /^Filme/ }));
    const films = screen.getByRole('table', { name: 'Abonnierte Podcasts in Filme' });
    expect(
      within(films)
        .getAllByRole('rowheader')
        .map((cell) => cell.textContent),
    ).toEqual([expect.stringMatching(/^Verirrt/)]);
  });

  it('the libraries fail to load: the error offers «Erneut versuchen»', async () => {
    let fail = true;
    mockApi({
      ...routes,
      'GET /admin/libraries': () => (fail ? problem(500, 'internal') : json([podcastLibrary])),
      'GET /admin/podcasts': () => json([adminPodcast]),
    });
    renderApp('/admin/podcasts');
    const user = userEvent.setup();
    const notice = await screen.findByRole('alert');
    expect(notice).toHaveTextContent('Etwas ist schiefgelaufen. Versuche es erneut.');
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
    fail = false;
    await user.click(within(notice).getByRole('button', { name: 'Erneut versuchen' }));
    expect(await screen.findByRole('tablist', { name: 'Podcasts nach Bibliothek' })).toBeVisible();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('polls every 2 s for 30 s after «Jetzt abrufen», held while «Feed abonnieren» is open', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const { calls } = mockApi({
      ...routes,
      'GET /admin/podcasts': () => json([adminPodcast]),
      [`POST /admin/podcasts/${audioIds.podcast}/refresh`]: () => json(adminPodcast, 202),
    });
    renderApp('/admin/podcasts');
    const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
    const lists = () => calls.filter((call) => call.path === '/admin/podcasts').length;

    const detail = await openRow(user, /^Slouchcat Test Podcast/);
    await user.click(
      within(detail).getByRole('button', { name: 'Slouchcat Test Podcast jetzt abrufen' }),
    );
    expect(await screen.findByText('Wird abgerufen')).toBeVisible();
    await waitFor(() => expect(lists()).toBe(2));
    await act(() => vi.advanceTimersByTimeAsync(2000));
    await waitFor(() => expect(lists()).toBe(3));
    await act(() => vi.advanceTimersByTimeAsync(2000));
    await waitFor(() => expect(lists()).toBe(4));

    // The open dialog holds the polling and the live indicator.
    await user.click(screen.getByRole('button', { name: 'Feed abonnieren' }));
    const dialog = await screen.findByRole('dialog', { name: 'Feed abonnieren' });
    expect(screen.queryByText('Live, alle 2 Sekunden')).not.toBeInTheDocument();
    await act(() => vi.advanceTimersByTimeAsync(6000));
    expect(lists()).toBe(4);
    await user.click(within(dialog).getByRole('button', { name: 'Abbrechen' }));
    expect(await screen.findByText('Live, alle 2 Sekunden')).toBeVisible();
    await act(() => vi.advanceTimersByTimeAsync(2000));
    await waitFor(() => expect(lists()).toBe(5));

    // 30 s after the press the polling ends, and «wird abgerufen» with it.
    await act(() => vi.advanceTimersByTimeAsync(20_000));
    await waitFor(() => expect(screen.queryByText('wird abgerufen')).not.toBeInTheDocument());
    expect(screen.queryByText('Live, alle 2 Sekunden')).not.toBeInTheDocument();
    const after = lists();
    await act(() => vi.advanceTimersByTimeAsync(6000));
    expect(lists()).toBe(after);
    expect(within(detail).getByRole('button', { name: /jetzt abrufen/ })).toBeVisible();
  });

  it('a failed «Jetzt abrufen» or «Abo löschen» shows the error and keeps the row', async () => {
    mockApi({
      ...routes,
      'GET /admin/podcasts': () => json([adminPodcast]),
      [`POST /admin/podcasts/${audioIds.podcast}/refresh`]: () => problem(500, 'internal'),
      [`DELETE /admin/podcasts/${audioIds.podcast}`]: () => problem(500, 'internal'),
    });
    renderApp('/admin/podcasts');
    const user = userEvent.setup();
    const detail = await openRow(user, /^Slouchcat Test Podcast/);

    await user.click(
      within(detail).getByRole('button', { name: 'Slouchcat Test Podcast jetzt abrufen' }),
    );
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Etwas ist schiefgelaufen. Versuche es erneut.',
    );
    expect(screen.queryByText('wird abgerufen')).not.toBeInTheDocument();
    expect(screen.queryByText('Live, alle 2 Sekunden')).not.toBeInTheDocument();

    await user.click(
      within(detail).getByRole('button', { name: 'Abo Slouchcat Test Podcast löschen' }),
    );
    const confirm = await screen.findByRole('alertdialog', {
      name: 'Abo Slouchcat Test Podcast löschen?',
    });
    await user.click(within(confirm).getByRole('button', { name: 'Abo löschen' }));
    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument());
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Etwas ist schiefgelaufen. Versuche es erneut.',
    );
    expect(screen.queryByText(/ist gelöscht/)).not.toBeInTheDocument();
    expect(screen.getByRole('rowheader', { name: /Slouchcat Test Podcast/ })).toBeVisible();
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

    expect(await screen.findByText('Aktuell')).toBeVisible();
    expect(screen.queryByText(/^Live/)).not.toBeInTheDocument();
    const detail = await openRow(user, /^Slouchcat Test Podcast/);
    await user.click(
      within(detail).getByRole('button', { name: 'Slouchcat Test Podcast jetzt abrufen' }),
    );
    expect(await screen.findByText('Slouchcat Test Podcast wird abgerufen.')).toBeVisible();
    expect(calls.some((call) => call.path.endsWith('/refresh'))).toBe(true);
    // Until the last fetch changes, the row says so; the page polls every 2 s meanwhile.
    expect(await screen.findByText('Wird abgerufen')).toBeVisible();
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

// Stored podcast episodes (concept 19 part A, stage 1, 12.3): the band «Neue Folgen» of a podcast
// library with the folder check and the confirmation of a change that loads once, the own
// setting of a podcast and «Fehlende nachladen» in its fold-out.
describe('admin podcasts page: stored episodes (concept 19 part A)', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  const savingLibrary = (
    downloads: Partial<NonNullable<Library['podcastDownloads']>> = {},
  ): Library => ({
    ...podcastLibrary,
    roots: [
      {
        id: 7,
        path: '/media/Audiobookshelf/Podcast',
        content: 'podcast',
        providers: ['local'],
        fsType: 'nfs4',
        watch: 'poll',
        itemCount: 1,
        fileCount: 3,
      },
    ],
    podcastDownloads: { mode: 'link', scope: 'new', lastN: 10, parallel: 2, ...downloads },
  });
  const folder = (writable: boolean, problem?: PodcastFolder['problem']): PodcastFolder => ({
    rootId: 7,
    path: '/media/Audiobookshelf/Podcast',
    writable,
    freeBytes: 2_100_000_000_000,
    ...(problem ? { problem } : {}),
  });
  const bandRoutes = (lib: Library, check: PodcastFolder) => ({
    ...routes,
    ...adminShell,
    'GET /admin/libraries': () => json([library(), lib]),
    'GET /admin/libraries/3': () => json({ ...lib, podcastFolder: check }),
    'GET /admin/podcasts': () => json([adminPodcast]),
  });
  /** Opens the band «Neue Folgen» and returns its body. */
  async function openBand(user: ReturnType<typeof userEvent.setup>) {
    const toggle = await screen.findByRole('button', { name: 'Neue Folgen' });
    await user.click(toggle);
    const body = document.getElementById(toggle.getAttribute('aria-controls') ?? '');
    if (!body) throw new Error('no body of the band');
    return body;
  }

  it('P-6: a folder that fails the check locks «Speichern» and says why', async () => {
    mockApi(bandRoutes(savingLibrary(), folder(false, 'read_only')));
    await renderAdminPage('/admin/podcasts');
    const user = userEvent.setup();
    // Folded: one line that sums the setting up.
    expect(await screen.findByText('Nur Link · nichts wird gespeichert')).toBeVisible();
    const band = await openBand(user);
    expect(await within(band).findByText('nur lesbar eingebunden')).toBeVisible();
    expect(within(band).getByText('/media/Audiobookshelf/Podcast')).toBeVisible();
    expect(
      within(band).getByText(/«Speichern» ist gesperrt, bis der Ordner die Prüfung besteht/),
    ).toBeVisible();
    expect(
      within(band).getByRole('link', { name: /Installationsanleitung: Podcast-Folgen speichern/ }),
    ).toHaveAttribute('href', expect.stringContaining('INSTALLATION.md#podcast-folgen-speichern'));
    const group = within(band).getByRole('radiogroup', { name: 'Neue Folgen' });
    expect(within(group).getByRole('radio', { name: 'Speichern' })).toBeDisabled();
    expect(within(group).getByRole('radio', { name: 'Nur Link' })).toBeChecked();
  });

  it('P-6: a folder that passes the check takes «Speichern» at once without asking', async () => {
    const { calls } = mockApi({
      ...bandRoutes(savingLibrary(), folder(true)),
      'PATCH /admin/libraries/3': (call) =>
        json({ ...savingLibrary(), ...(call.body as Partial<Library>) }),
    });
    await renderAdminPage('/admin/podcasts');
    const user = userEvent.setup();
    const band = await openBand(user);
    expect(await within(band).findByText('beschreibbar · 2,1 TB frei')).toBeVisible();
    const save = within(within(band).getByRole('radiogroup', { name: 'Neue Folgen' })).getByRole(
      'radio',
      { name: 'Speichern' },
    );
    expect(save).toBeEnabled();
    await user.click(save);
    expect(await within(band).findByText('Gespeichert.')).toBeVisible();
    expect(calls.filter((call) => call.method === 'PATCH').map((call) => call.body)).toEqual([
      { podcastDownloads: { mode: 'save', scope: 'new', lastN: 10, parallel: 2 } },
    ]);
    // «Nur neue Folgen» loads nothing once: no estimate.
    expect(calls.some((call) => call.path.endsWith('/estimate'))).toBe(false);
  });

  it('P-13: «Alle Folgen» shows the estimate first; «Abbrechen» keeps the scope, «Laden» saves it', async () => {
    const lib = savingLibrary({ mode: 'save' });
    const { calls } = mockApi({
      ...bandRoutes(lib, folder(true)),
      'POST /admin/libraries/3/podcast-downloads/estimate': (call) =>
        json(
          (call.body as { scope: string }).scope === 'all'
            ? { episodes: 1894, bytes: 52_000_000_000, freeBytes: 2_100_000_000_000 }
            : { episodes: 0, bytes: 0, freeBytes: 2_100_000_000_000 },
        ),
      'PATCH /admin/libraries/3': () =>
        json({ ...lib, podcastDownloads: { ...lib.podcastDownloads, scope: 'all' } }),
    });
    await renderAdminPage('/admin/podcasts');
    const user = userEvent.setup();
    expect(await screen.findByText('Speichern · Nur neue Folgen · 2 gleichzeitig')).toBeVisible();
    const band = await openBand(user);
    const scope = within(band).getByRole('radiogroup', { name: 'Umfang' });
    await user.click(within(scope).getByRole('radio', { name: 'Alle Folgen' }));
    const estimate = await within(band).findByText(
      /^Das lädt einmalig rund 1’?\.?894 Folgen, etwa 52 GB\. Frei: 2,1 TB\.$/,
    );
    expect(estimate).toBeVisible();
    expect(
      calls.filter((call) => call.path.endsWith('/estimate')).map((call) => call.body),
    ).toEqual([{ mode: 'save', scope: 'all', lastN: 10 }]);
    expect(calls.some((call) => call.method === 'PATCH')).toBe(false);

    // «Abbrechen»: the old scope applies, nothing was sent.
    await user.click(within(band).getByRole('button', { name: 'Abbrechen' }));
    expect(within(scope).getByRole('radio', { name: 'Nur neue Folgen' })).toBeChecked();
    expect(within(band).queryByText(/Das lädt einmalig/)).not.toBeInTheDocument();
    expect(calls.some((call) => call.method === 'PATCH')).toBe(false);

    // «Laden»: the new scope is saved and the loading begins.
    await user.click(within(scope).getByRole('radio', { name: 'Alle Folgen' }));
    await user.click(await within(band).findByRole('button', { name: 'Laden' }));
    expect(await within(band).findByText('Gespeichert.')).toBeVisible();
    expect(calls.filter((call) => call.method === 'PATCH').map((call) => call.body)).toEqual([
      { podcastDownloads: { mode: 'save', scope: 'all', lastN: 10, parallel: 2 } },
    ]);
  });

  it('a podcast differs from its library after the estimate; «Fehlende nachladen» asks too', async () => {
    const { calls } = mockApi({
      ...bandRoutes(savingLibrary(), folder(true)),
      'POST /admin/libraries/3/podcast-downloads/estimate': (call) => {
        const body = call.body as { mode: string; scope: string };
        return json(
          body.mode === 'save'
            ? {
                episodes: body.scope === 'all' ? 3 : 2,
                bytes: 50_000_000,
                freeBytes: 10_000_000_000,
              }
            : { episodes: 0, bytes: 0, freeBytes: 10_000_000_000 },
        );
      },
      [`PATCH /admin/podcasts/${audioIds.podcast}`]: () =>
        json({ ...adminPodcast, mode: 'save', effectiveMode: 'save' }),
      [`POST /admin/podcasts/${audioIds.podcast}/download`]: () => json({ queued: 3 }, 202),
    });
    await renderAdminPage('/admin/podcasts');
    const user = userEvent.setup();
    const detail = await openRow(user, /^Slouchcat Test Podcast/);
    expect(within(detail).getByText('Gilt: Nur Link')).toBeVisible();
    const mode = within(detail).getByRole('radiogroup', { name: 'Neue Folgen dieses Podcasts' });
    expect(within(mode).getByRole('radio', { name: 'Wie Bibliothek' })).toBeChecked();
    await user.click(within(mode).getByRole('radio', { name: 'Speichern' }));
    expect(
      await within(detail).findByText(/^Das lädt einmalig rund 2 Folgen, etwa 50 MB\./),
    ).toBeVisible();
    expect(calls.some((call) => call.method === 'PATCH')).toBe(false);
    await user.click(within(detail).getByRole('button', { name: 'Laden' }));
    expect(await within(detail).findByText('Gespeichert.')).toBeVisible();
    expect(calls.filter((call) => call.method === 'PATCH').map((call) => call.body)).toEqual([
      { mode: 'save', scope: 'inherit' },
    ]);

    await user.click(
      within(detail).getByRole('button', {
        name: 'Fehlende Folgen von Slouchcat Test Podcast nachladen',
      }),
    );
    expect(await within(detail).findByText(/^Das lädt einmalig rund 3 Folgen/)).toBeVisible();
    expect(calls.some((call) => call.path.endsWith('/download'))).toBe(false);
    await user.click(within(detail).getByRole('button', { name: 'Laden' }));
    expect(await within(detail).findByText('3 Folgen werden geladen.')).toBeVisible();
    expect(
      calls.filter((call) => call.path.endsWith('/estimate')).map((call) => call.body),
    ).toEqual([
      { podcastId: audioIds.podcast, mode: 'save', scope: 'inherit' },
      { podcastId: audioIds.podcast, mode: 'inherit', scope: 'inherit' },
      { podcastId: audioIds.podcast, mode: 'save', scope: 'all' },
    ]);
  });

  it('a podcast the server cannot store has «Speichern» locked', async () => {
    mockApi({
      ...bandRoutes(savingLibrary(), folder(false, 'not_mounted')),
      'GET /admin/podcasts': () => json([{ ...adminPodcast, savable: false }]),
    });
    await renderAdminPage('/admin/podcasts');
    const user = userEvent.setup();
    const detail = await openRow(user, /^Slouchcat Test Podcast/);
    expect(
      within(detail).getByText(
        'Der Ordner dieses Podcasts besteht die Ordnerprüfung nicht; «Speichern» ist gesperrt.',
      ),
    ).toBeVisible();
    const mode = within(detail).getByRole('radiogroup', { name: 'Neue Folgen dieses Podcasts' });
    expect(within(mode).getByRole('radio', { name: 'Speichern' })).toBeDisabled();
    expect(
      within(detail).getByRole('button', {
        name: 'Fehlende Folgen von Slouchcat Test Podcast nachladen',
      }),
    ).toBeDisabled();
  });

  it('«Feed abonnieren» says what a storing library loads after the first fetch', async () => {
    mockApi(bandRoutes(savingLibrary({ mode: 'save', scope: 'last_n', lastN: 5 }), folder(true)));
    await renderAdminPage('/admin/podcasts');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Feed abonnieren' }));
    const dialog = await screen.findByRole('dialog', { name: 'Feed abonnieren' });
    expect(
      within(dialog).getByText(
        'Diese Bibliothek speichert: nach dem Abonnieren lädt Slouchcat die 5 neuesten Folgen als Datei.',
      ),
    ).toBeVisible();
  });
  it('«Feed abonnieren» in a library that loads asks first: «Abonnieren und laden» or «Nur abonnieren»', async () => {
    const { calls } = mockApi({
      ...bandRoutes(savingLibrary({ mode: 'save', scope: 'all' }), folder(true)),
      'POST /admin/podcasts': () => json(adminPodcast, 201),
    });
    await renderAdminPage('/admin/podcasts');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Feed abonnieren' }));
    const dialog = await screen.findByRole('dialog', { name: 'Feed abonnieren' });
    await user.type(
      within(dialog).getByRole('textbox', { name: 'Feed-Adresse' }),
      'http://127.0.0.1:8095/feed.xml',
    );
    const choice = within(dialog).getByRole('radiogroup', { name: 'Folgen laden' });
    expect(within(choice).getByRole('radio', { name: 'Abonnieren und laden' })).not.toBeChecked();
    expect(within(choice).getByRole('radio', { name: 'Nur abonnieren' })).not.toBeChecked();
    // Without a choice nothing is subscribed.
    await user.click(within(dialog).getByRole('button', { name: 'Abonnieren' }));
    expect(calls.some((call) => call.method === 'POST')).toBe(false);
    await user.click(within(choice).getByRole('radio', { name: 'Nur abonnieren' }));
    await user.click(within(dialog).getByRole('button', { name: 'Abonnieren' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(calls.find((call) => call.method === 'POST')?.body).toEqual({
      libraryId: 3,
      feedUrl: 'http://127.0.0.1:8095/feed.xml',
      loadEpisodes: false,
    });
  });
});
