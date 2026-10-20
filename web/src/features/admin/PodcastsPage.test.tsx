import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { adminPodcast, audioIds, library } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, signedIn } from '@/test/api';

const podcastLibrary = library({
  id: 3,
  name: 'Podcasts',
  kind: 'podcasts',
  defaultRating: 0,
  roots: [],
});

const routes = {
  ...signedIn,
  'GET /admin/libraries': () => json([library(), podcastLibrary]),
};

describe('admin podcasts page (M6, H-2)', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('lists the subscriptions with library, episodes, last fetch and error', async () => {
    mockApi({
      ...routes,
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
    renderApp('/admin/podcasts');

    const table = await screen.findByRole('table', { name: 'Abonnierte Podcasts' });
    const rows = within(table).getAllByRole('row');
    expect(rows).toHaveLength(3);
    expect(within(rows[1]!).getByRole('rowheader')).toHaveTextContent('Slouchcat Test Podcast');
    expect(within(rows[1]!).getByText('Slouchcat Test Author')).toBeVisible();
    expect(within(rows[1]!).getByText('http://127.0.0.1:8095/feed.xml')).toBeVisible();
    expect(within(rows[1]!).getByText('Podcasts')).toBeVisible();
    expect(within(rows[1]!).getByText('2')).toBeVisible();
    expect(within(rows[2]!).getByText('Noch nie')).toBeVisible();
    expect(within(rows[2]!).getByText('Explizit')).toBeVisible();
    expect(within(rows[2]!).getByText('feed_unreachable: HTTP 404')).toBeVisible();
    expect(
      screen
        .getByRole('navigation', { name: 'Hauptnavigation' })
        .querySelector('[aria-current="page"]'),
    ).toHaveTextContent('Podcasts');
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

  it('"Jetzt abrufen" queues a fetch; "Abo löschen" asks first and removes the row', async () => {
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

    await user.click(
      await screen.findByRole('button', { name: 'Slouchcat Test Podcast jetzt abrufen' }),
    );
    expect(await screen.findByText('Slouchcat Test Podcast wird abgerufen.')).toBeVisible();
    expect(calls.some((call) => call.path.endsWith('/refresh'))).toBe(true);

    await user.click(screen.getByRole('button', { name: 'Abo Slouchcat Test Podcast löschen' }));
    const confirm = await screen.findByRole('alertdialog', {
      name: 'Abo Slouchcat Test Podcast löschen?',
    });
    await user.click(within(confirm).getByRole('button', { name: 'Abo löschen' }));
    expect(await screen.findByText('Das Abo Slouchcat Test Podcast ist gelöscht.')).toBeVisible();
    await waitFor(() => expect(screen.queryByRole('table')).not.toBeInTheDocument());
    expect(calls.some((call) => call.method === 'DELETE')).toBe(true);
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
