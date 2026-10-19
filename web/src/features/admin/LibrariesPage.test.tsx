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
            kind: 'series',
            roots: [
              {
                id: 2,
                path: '/media/Serien',
                fsType: 'nfs',
                watch: 'poll',
                warning: 'network_share',
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
      'POST /admin/libraries': () => problem(422, 'library_path_invalid'),
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup();
    expect(await screen.findByText('Noch keine Bibliothek angelegt.')).toBeVisible();

    await user.type(screen.getByLabelText('Name'), 'Dokus');
    await user.click(screen.getByRole('radio', { name: 'Serien' }));
    await user.type(screen.getByLabelText('Ordner'), '/media/Dokus');
    await user.click(screen.getByRole('button', { name: 'Hinzufügen' }));

    await waitFor(() =>
      expect(screen.getByLabelText('Ordner')).toHaveAccessibleDescription(
        /Diesen Ordner gibt es auf dem Server nicht/,
      ),
    );
    expect(calls.find((call) => call.method === 'POST')?.body).toEqual({
      name: 'Dokus',
      kind: 'series',
      path: '/media/Dokus',
    });
  });
});
