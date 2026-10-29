import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { library } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, signedIn } from '@/test/api';

describe('admin libraries page', () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('renders scan state, counts, roots and the network share warning', async () => {
    mockApi({
      ...signedIn,
      'GET /admin/libraries': () =>
        json([
          library(),
          library({
            id: 2,
            name: 'Serien',
            kind: 'video',
            roots: [
              {
                id: 2,
                path: '/media/Serien',
                content: 'series',
                providers: ['tmdb', 'tvdb'],
                fsType: 'nfs',
                watch: 'poll',
                warning: 'network_share',
                itemCount: 0,
                fileCount: 0,
              },
            ],
            scan: { state: 'failed', itemCount: 0, fileCount: 0, lastError: 'permission denied' },
          }),
        ]),
    });
    renderApp('/admin/libraries');

    const movies = await screen.findByRole('article', { name: 'Filme' });
    expect(within(movies).getByText('Aktuell')).toBeVisible();
    expect(within(movies).getByText('Titel: 12 · Dateien: 14')).toBeVisible();
    expect(within(movies).getByText('/media/Filme')).toBeVisible();
    expect(within(movies).getByText('Änderungen werden sofort erkannt')).toBeVisible();

    const series = screen.getByRole('article', { name: 'Serien' });
    expect(within(series).getByText('Netzlaufwerk')).toBeVisible();
    expect(within(series).getByText('Dateisystem nfs')).toBeVisible();
    expect(within(series).getByText('Scan fehlgeschlagen')).toBeVisible();
    expect(within(series).getByText('Letzter Fehler: permission denied')).toBeVisible();
  });

  it('explains an aborted scan and accepts the loss after confirmation', async () => {
    const aborted = library({
      scan: { state: 'aborted', itemCount: 12, fileCount: 2, lastError: 'loss_over_threshold' },
    });
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/libraries': () => json([aborted]),
      'POST /admin/libraries/1/accept-loss': () =>
        json({ ...aborted, scan: { ...aborted.scan, state: 'queued' } }, 202),
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup();

    expect(await screen.findByText(/Mehr als 20 % der Dateien fehlen/)).toBeVisible();
    await user.click(screen.getByRole('button', { name: 'Verlust akzeptieren' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Fehlende Dateien entfernen?' });
    await user.click(within(dialog).getByRole('button', { name: 'Entfernen und scannen' }));

    await waitFor(() =>
      expect(calls.some((call) => call.path === '/admin/libraries/1/accept-loss')).toBe(true),
    );
  });

  it('starts a rescan and polls every 5 s while the scan runs', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    let state: 'done' | 'running' = 'done';
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/libraries': () =>
        json([
          library({ scan: { state, itemCount: 3, fileCount: 3, filesSeen: 4, filesTotal: 10 } }),
        ]),
      'POST /admin/libraries/1/scan': () => {
        state = 'running';
        return json(library({ scan: { state, itemCount: 3, fileCount: 3 } }), 202);
      },
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
    await user.click(await screen.findByRole('button', { name: 'Erneut scannen' }));

    expect(await screen.findByText('Scan läuft')).toBeVisible();
    expect(screen.getByRole('progressbar', { name: '4 von 10 Dateien' })).toBeVisible();
    const before = calls.filter((call) => call.path === '/admin/libraries').length;
    await act(() => vi.advanceTimersByTimeAsync(5000));
    await waitFor(() =>
      expect(calls.filter((call) => call.path === '/admin/libraries').length).toBeGreaterThan(
        before,
      ),
    );
  });

  it('adds a library and shows server errors at the path field', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/libraries': () => json([]),
      'POST /admin/libraries': () =>
        problem(422, 'validation_failed', {
          errors: [{ field: 'roots[0].path', code: 'library_path_invalid' }],
        }),
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup();
    expect(await screen.findByText('Noch keine Bibliothek angelegt.')).toBeVisible();

    await user.type(screen.getByLabelText('Name'), 'Dokus');
    await user.click(screen.getByRole('radio', { name: 'Video' }));
    await user.type(screen.getByLabelText('Ordner'), '/media/Dokus');
    await user.click(screen.getByRole('button', { name: 'Hinzufügen' }));

    await waitFor(() =>
      expect(screen.getByLabelText('Ordner')).toHaveAccessibleDescription(
        /Diesen Ordner gibt es auf dem Server nicht/,
      ),
    );
    expect(calls.find((call) => call.method === 'POST')?.body).toEqual({
      name: 'Dokus',
      kind: 'video',
      roots: [{ path: '/media/Dokus', content: 'movie', providers: ['tmdb'] }],
    });
  });
  it('cancels a running scan; a cancelled scan has its own text and no loss button', async () => {
    const running = library({
      scan: { state: 'running', itemCount: 3, fileCount: 4, filesSeen: 40, filesTotal: 100 },
    });
    let current = running;
    const user = userEvent.setup();
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/libraries': () => json([current]),
      'DELETE /admin/libraries/1/scan': () => {
        current = library({
          scan: { state: 'aborted', itemCount: 3, fileCount: 4, lastError: 'cancelled' },
        });
        return json(current, 202);
      },
    });
    renderApp('/admin/libraries');

    const card = await screen.findByRole('article', { name: 'Filme' });
    expect(within(card).getByText('40 von 100 Dateien')).toBeVisible();
    expect(within(card).getByText('Titel: 3 · Dateien: 4')).toBeVisible();
    await user.click(within(card).getByRole('button', { name: 'Scan abbrechen' }));
    await waitFor(() =>
      expect(
        calls.some((call) => call.method === 'DELETE' && call.path === '/admin/libraries/1/scan'),
      ).toBe(true),
    );
    expect(
      await within(card).findByText(
        'Der Scan wurde abgebrochen. Der nächste Scan läuft wieder normal.',
      ),
    ).toBeVisible();
    expect(within(card).queryByRole('button', { name: 'Scan abbrechen' })).not.toBeInTheDocument();
    expect(
      within(card).queryByRole('button', { name: 'Verlust akzeptieren' }),
    ).not.toBeInTheDocument();
  });

  it('audio libraries: default rating on the card, changed through PATCH (AN-2, M6)', async () => {
    const books = library({
      id: 3,
      name: 'Hörbücher',
      kind: 'audiobooks',
      defaultRating: 0,
      roots: [
        {
          id: 3,
          path: '/media/Hoerbuecher',
          content: 'audiobook',
          providers: ['local'],
          fsType: 'ext4',
          watch: 'inotify',
          itemCount: 0,
          fileCount: 0,
        },
      ],
    });
    const podcasts = library({
      id: 4,
      name: 'Podcasts',
      kind: 'podcasts',
      defaultRating: 6,
      roots: [],
    });
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/libraries': () => json([library(), books, podcasts]),
      'PATCH /admin/libraries/3': () => json({ ...books, defaultRating: 12 }),
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup();

    const card = await screen.findByRole('article', { name: 'Hörbücher' });
    expect(within(card).getByText('Standard: FSK 0')).toBeVisible();
    const podcastCard = screen.getByRole('article', { name: 'Podcasts' });
    expect(within(podcastCard).getByText('Standard: FSK 6')).toBeVisible();
    expect(
      within(podcastCard).getByText('Kein Ordner: Die Titel kommen aus abonnierten Feeds.'),
    ).toBeVisible();
    // Video libraries have no default rating.
    const movies = screen.getByRole('article', { name: 'Filme' });
    expect(within(movies).queryByText(/^Standard:/)).not.toBeInTheDocument();
    expect(within(movies).queryByRole('button', { name: /Standardstufe/ })).not.toBeInTheDocument();

    await user.click(
      within(card).getByRole('button', { name: 'Standardstufe von Hörbücher ändern' }),
    );
    const dialog = await screen.findByRole('dialog', { name: 'Standardstufe von Hörbücher' });
    expect(within(dialog).getByRole('radio', { name: 'FSK 0' })).toBeChecked();
    await user.click(within(dialog).getByText('FSK 12'));
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(calls.find((call) => call.method === 'PATCH')?.body).toEqual({ defaultRating: 12 });
    expect(screen.getByText('Die Standardstufe von Hörbücher ist jetzt FSK 12.')).toBeVisible();
  });

  it('adds a podcast library without folder and an audiobook library with its rating', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/libraries': () => json([]),
      'POST /admin/libraries': (call) => json(library(call.body as never), 201),
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup();
    await screen.findByText('Noch keine Bibliothek angelegt.');

    // Video: no default rating to choose.
    expect(screen.queryByRole('radiogroup', { name: 'Standard-Altersstufe' })).toBeNull();
    await user.type(screen.getByLabelText('Name'), 'Podcasts');
    await user.click(screen.getByRole('radio', { name: 'Podcasts' }));
    expect(screen.queryByLabelText('Ordner')).not.toBeInTheDocument();
    expect(screen.getByText(/Ordner optional/)).toBeVisible();
    const rating = screen.getByRole('radiogroup', { name: 'Standard-Altersstufe' });
    expect(within(rating).getByRole('radio', { name: 'FSK 0' })).toBeChecked();
    await user.click(within(rating).getByText('FSK 6'));
    await user.click(screen.getByRole('button', { name: 'Hinzufügen' }));
    await waitFor(() =>
      expect(calls.find((call) => call.method === 'POST')?.body).toEqual({
        name: 'Podcasts',
        kind: 'podcasts',
        roots: [],
        defaultRating: 6,
      }),
    );

    await user.type(await screen.findByLabelText('Name'), 'Hörbücher');
    await user.click(screen.getByRole('radio', { name: 'Hörbücher' }));
    await user.type(screen.getByLabelText('Ordner'), '/media/Hoerbuecher');
    await user.click(screen.getByRole('button', { name: 'Hinzufügen' }));
    await waitFor(() =>
      expect(calls.filter((call) => call.method === 'POST').at(-1)?.body).toEqual({
        name: 'Hörbücher',
        kind: 'audiobooks',
        roots: [{ path: '/media/Hoerbuecher', content: 'audiobook', providers: ['local'] }],
        defaultRating: 0,
      }),
    );
  });
});
