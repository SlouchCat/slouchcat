import type { Library } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { library } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, signedIn } from '@/test/api';

const second = {
  id: 2,
  path: '/media/Filme-2',
  fsType: 'nfs' as const,
  watch: 'poll' as const,
  warning: 'network_share',
  itemCount: 3,
  fileCount: 4,
};

const withTwoFolders: Library = library({ roots: [library().roots[0]!, second] });

/** Opens "Folders" of the film library and returns the dialog. */
async function openFolders(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole('button', { name: 'Ordner von Filme bearbeiten' }));
  return screen.findByRole('dialog', { name: 'Ordner von Filme' });
}

describe('library folders dialog', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('names the number of folders on the card and lists them in the dialog', async () => {
    mockApi({ ...signedIn, 'GET /admin/libraries': () => json([withTwoFolders]) });
    renderApp('/admin/libraries');
    const user = userEvent.setup();

    const card = await screen.findByRole('article', { name: 'Filme' });
    expect(within(card).getByText('2 Ordner')).toBeVisible();
    // The paths themselves are in the dialog now, not on the card.
    expect(within(card).queryByText('/media/Filme-2')).toBeNull();

    const dialog = await openFolders(user);
    expect(within(dialog).getByText('/media/Filme')).toBeVisible();
    expect(within(dialog).getByText('/media/Filme-2')).toBeVisible();
    expect(within(dialog).getByText('Titel: 3 · Dateien: 4')).toBeVisible();
  });

  it('leaves no scan to start for a library without a folder', async () => {
    mockApi({
      ...signedIn,
      'GET /admin/libraries': () => json([library({ roots: [] })]),
    });
    renderApp('/admin/libraries');

    const card = await screen.findByRole('article', { name: 'Filme' });
    // Nothing to walk, so the button cannot promise a scan (idea 02, E13).
    expect(within(card).getByRole('button', { name: 'Erneut scannen' })).toBeDisabled();
    expect(within(card).getByRole('button', { name: 'Ordner von Filme bearbeiten' })).toBeEnabled();
  });

  it('shows the single folder on the card and its own sentence without one', async () => {
    mockApi({
      ...signedIn,
      'GET /admin/libraries': () =>
        json([library(), library({ id: 2, name: 'Leer', kind: 'series', roots: [] })]),
    });
    renderApp('/admin/libraries');

    const movies = await screen.findByRole('article', { name: 'Filme' });
    expect(within(movies).getByText('/media/Filme')).toBeVisible();

    const empty = screen.getByRole('article', { name: 'Leer' });
    expect(within(empty).getByText(/Kein Ordner: Es wird nichts gescannt/)).toBeVisible();
    // The podcast sentence would be wrong here: this library has no feeds.
    expect(within(empty).queryByText(/abonnierten Feeds/)).toBeNull();
  });

  it('adds a folder and asks the browse queries again', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/libraries': () => json([library()]),
      'GET /admin/fs/roots': () => json([]),
      'POST /admin/libraries/1/roots': () => json(withTwoFolders, 201),
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup();

    const dialog = await openFolders(user);
    await user.type(within(dialog).getByLabelText('Ordner hinzufügen'), '/media/Filme-2');
    await user.click(within(dialog).getByRole('button', { name: 'Hinzufügen' }));

    await waitFor(() => {
      expect(calls).toContainEqual(
        expect.objectContaining({
          method: 'POST',
          path: '/admin/libraries/1/roots',
          body: { path: '/media/Filme-2' },
        }),
      );
    });
    // The mark "belongs to library X" in the browse dialog is stale now, so its key goes too.
    await waitFor(() => {
      expect(calls.filter((call) => call.path === '/admin/libraries').length).toBeGreaterThan(1);
    });
  });

  it('reports a path the server refuses', async () => {
    mockApi({
      ...signedIn,
      'GET /admin/libraries': () => json([library()]),
      'POST /admin/libraries/1/roots': () => problem(422, 'library_path_invalid'),
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup();

    const dialog = await openFolders(user);
    await user.type(within(dialog).getByLabelText('Ordner hinzufügen'), '/etc');
    await user.click(within(dialog).getByRole('button', { name: 'Hinzufügen' }));

    expect(
      await within(dialog).findByText(/Diesen Ordner gibt es auf dem Server nicht/),
    ).toBeVisible();
  });

  it('removes a folder only after the path has been typed out', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/libraries': () => json([withTwoFolders]),
      'POST /admin/libraries/1/roots/2/removal': () => json(library()),
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup();

    const dialog = await openFolders(user);
    await user.click(
      within(dialog).getByRole('button', { name: 'Ordner /media/Filme-2 entfernen' }),
    );

    const confirm = await screen.findByRole('alertdialog', { name: 'Ordner wirklich entfernen?' });
    // The text says what really happens, including what stays.
    expect(within(confirm).getByText(/weich gelöscht/)).toBeVisible();
    expect(within(confirm).getByText(/Offline-Downloads/)).toBeVisible();
    expect(within(confirm).getByText(/Radarr und Sonarr/)).toBeVisible();
    expect(within(confirm).getByText(/30 Tagen/)).toBeVisible();
    expect(within(confirm).getByText(/SLOUCHCAT_BROWSE_ROOTS/)).toBeVisible();

    const remove = within(confirm).getByRole('button', { name: 'Ordner entfernen' });
    expect(remove).toBeDisabled();

    const field = within(confirm).getByLabelText('Tippe den Pfad, um zu bestätigen');
    await user.type(field, '/media/Filme');
    expect(remove).toBeDisabled();
    await user.type(field, '-2');
    await waitFor(() => expect(remove).toBeEnabled());

    await user.click(remove);
    await waitFor(() => {
      expect(calls).toContainEqual(
        expect.objectContaining({
          method: 'POST',
          path: '/admin/libraries/1/roots/2/removal',
          body: { confirmPath: '/media/Filme-2' },
        }),
      );
    });
  });

  it('starts the confirmation empty again after it was closed unconfirmed', async () => {
    mockApi({ ...signedIn, 'GET /admin/libraries': () => json([withTwoFolders]) });
    renderApp('/admin/libraries');
    const user = userEvent.setup();

    const dialog = await openFolders(user);
    const openConfirm = async () => {
      await user.click(
        within(dialog).getByRole('button', { name: 'Ordner /media/Filme-2 entfernen' }),
      );
      return screen.findByRole('alertdialog', { name: 'Ordner wirklich entfernen?' });
    };

    let confirm = await openConfirm();
    await user.type(
      within(confirm).getByLabelText('Tippe den Pfad, um zu bestätigen'),
      '/media/Filme-2',
    );
    await waitFor(() =>
      expect(within(confirm).getByRole('button', { name: 'Ordner entfernen' })).toBeEnabled(),
    );
    // Second thoughts.
    await user.click(within(confirm).getByRole('button', { name: 'Abbrechen' }));
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull());

    // The same row again: the path has to be typed once more, or the next click would remove the
    // folder without anybody confirming anything.
    confirm = await openConfirm();
    expect(within(confirm).getByLabelText('Tippe den Pfad, um zu bestätigen')).toHaveValue('');
    expect(within(confirm).getByRole('button', { name: 'Ordner entfernen' })).toBeDisabled();
  });

  it('sends the path as it was typed, not as the list knows it', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/libraries': () => json([withTwoFolders]),
      'POST /admin/libraries/1/roots/2/removal': () => json(library()),
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup();

    const dialog = await openFolders(user);
    await user.click(
      within(dialog).getByRole('button', { name: 'Ordner /media/Filme-2 entfernen' }),
    );
    const confirm = await screen.findByRole('alertdialog', { name: 'Ordner wirklich entfernen?' });
    // Surrounding spaces are trimmed before the request leaves.
    await user.type(
      within(confirm).getByLabelText('Tippe den Pfad, um zu bestätigen'),
      '  /media/Filme-2  ',
    );
    await user.click(within(confirm).getByRole('button', { name: 'Ordner entfernen' }));

    await waitFor(() => {
      expect(calls).toContainEqual(
        expect.objectContaining({
          method: 'POST',
          path: '/admin/libraries/1/roots/2/removal',
          body: { confirmPath: '/media/Filme-2' },
        }),
      );
    });
  });

  it('forgets an error when the dialog is closed with the close button', async () => {
    mockApi({
      ...signedIn,
      'GET /admin/libraries': () => json([library()]),
      'POST /admin/libraries/1/roots': () => problem(422, 'library_path_invalid'),
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup();

    let dialog = await openFolders(user);
    await user.type(within(dialog).getByLabelText('Ordner hinzufügen'), '/etc');
    await user.click(within(dialog).getByRole('button', { name: 'Hinzufügen' }));
    expect(
      await within(dialog).findByText(/Diesen Ordner gibt es auf dem Server nicht/),
    ).toBeVisible();

    await user.click(within(dialog).getByRole('button', { name: 'Schliessen' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());

    dialog = await openFolders(user);
    expect(within(dialog).queryByText(/Diesen Ordner gibt es auf dem Server nicht/)).toBeNull();
    expect(within(dialog).getByLabelText('Ordner hinzufügen')).toHaveValue('');
  });

  it('warns that it is the last folder and reports a running scan', async () => {
    mockApi({
      ...signedIn,
      'GET /admin/libraries': () => json([library()]),
      'POST /admin/libraries/1/roots/1/removal': () => problem(409, 'scan_running'),
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup();

    const dialog = await openFolders(user);
    await user.click(within(dialog).getByRole('button', { name: 'Ordner /media/Filme entfernen' }));
    const confirm = await screen.findByRole('alertdialog', { name: 'Ordner wirklich entfernen?' });
    expect(within(confirm).getByText(/letzte Ordner/)).toBeVisible();

    await user.type(
      within(confirm).getByLabelText('Tippe den Pfad, um zu bestätigen'),
      '/media/Filme',
    );
    await user.click(within(confirm).getByRole('button', { name: 'Ordner entfernen' }));
    expect(
      await within(confirm).findByText(/Ein Scan dieser Bibliothek läuft gerade/),
    ).toBeVisible();
  });
});
