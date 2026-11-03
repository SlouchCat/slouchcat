import type { Library, MetadataProviderInfo } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { library } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, signedIn } from '@/test/api';

const second = {
  id: 2,
  path: '/media/Filme-2',
  content: 'movie' as const,
  providers: ['tmdb' as const],
  fsType: 'nfs' as const,
  watch: 'poll' as const,
  warning: 'network_share',
  itemCount: 3,
  fileCount: 4,
};

const withTwoFolders: Library = library({ roots: [library().roots[0]!, second] });

const providers: MetadataProviderInfo[] = [
  {
    id: 'tmdb',
    configured: true,
    roles: ['identify', 'fill'],
    contents: ['movie', 'series'],
    languages: ['de', 'en'],
  },
  {
    id: 'tvdb',
    configured: true,
    roles: ['identify', 'fill'],
    contents: ['series'],
    languages: ['de', 'en'],
  },
];

type User = ReturnType<typeof userEvent.setup>;

/** The row of a library: the toggle's table row. */
const rowOf = (name: string) => screen.getByRole('button', { name }).closest('tr')!;

/** Folds out the row of a library and returns its fold-out. */
async function openRow(user: User, name: string) {
  const toggle = await screen.findByRole('button', { name });
  if (toggle.getAttribute('aria-expanded') !== 'true') await user.click(toggle);
  return screen.findByRole('region', { name });
}

/** Opens "Folders" of the film library from its fold-out and returns the dialog. */
async function openFolders(user: User) {
  const detail = await openRow(user, 'Filme');
  await user.click(within(detail).getByRole('button', { name: 'Ordner von Filme bearbeiten' }));
  return screen.findByRole('dialog', { name: 'Ordner von Filme' });
}

describe('library folders dialog', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('names the number of folders in the row, lists them in the fold-out and the dialog', async () => {
    mockApi({ ...signedIn, 'GET /admin/libraries': () => json([withTwoFolders]) });
    renderApp('/admin/libraries');
    const user = userEvent.setup();

    await screen.findByRole('button', { name: 'Filme' });
    const row = rowOf('Filme');
    // The count and the summary per content (ADR 0015): both folders hold films.
    expect(within(row).getByText('2 Ordner')).toBeVisible();
    expect(within(row).getByText(/2 Filme/)).toBeVisible();
    // The paths themselves are in the fold-out, not in the row (BIB-2a).
    expect(within(row).queryByText('/media/Filme-2')).toBeNull();
    const detail = await openRow(user, 'Filme');
    expect(within(detail).getByText('/media/Filme-2')).toBeVisible();
    expect(within(detail).getByText('Titel: 3 · Dateien: 4')).toBeVisible();

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
    const user = userEvent.setup();

    const detail = await openRow(user, 'Filme');
    // Nothing to walk, so the button cannot promise a scan (idea 02, E13).
    expect(within(detail).getByRole('button', { name: 'Erneut scannen' })).toBeDisabled();
    expect(
      within(detail).getByRole('button', { name: 'Ordner von Filme bearbeiten' }),
    ).toBeEnabled();
  });

  it('shows the single folder in the row and its own sentence without one', async () => {
    mockApi({
      ...signedIn,
      'GET /admin/libraries': () =>
        json([library(), library({ id: 2, name: 'Leer', kind: 'video', roots: [] })]),
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup();

    await screen.findByRole('button', { name: 'Filme' });
    expect(within(rowOf('Filme')).getByText('/media/Filme')).toBeVisible();
    expect(within(rowOf('Leer')).getByText('0 Ordner')).toBeVisible();

    const empty = await openRow(user, 'Leer');
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
    // «Neue Ordner» (F-13): a card per folder, one submit for all of them.
    await user.click(within(dialog).getByRole('button', { name: 'Ordner hinzufügen' }));
    await user.type(within(dialog).getByLabelText('Ordner'), '/media/Filme-2');
    await user.click(within(dialog).getByRole('button', { name: '1 Ordner hinzufügen' }));

    await waitFor(() => {
      expect(calls).toContainEqual(
        expect.objectContaining({
          method: 'POST',
          path: '/admin/libraries/1/roots',
          body: { roots: [{ path: '/media/Filme-2', content: 'movie', providers: ['tmdb'] }] },
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
      'POST /admin/libraries/1/roots': () =>
        problem(422, 'validation_failed', {
          errors: [{ field: 'roots[0].path', code: 'library_path_invalid' }],
        }),
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup();

    const dialog = await openFolders(user);
    await user.click(within(dialog).getByRole('button', { name: 'Ordner hinzufügen' }));
    await user.type(within(dialog).getByLabelText('Ordner'), '/etc');
    await user.click(within(dialog).getByRole('button', { name: '1 Ordner hinzufügen' }));

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
      'POST /admin/libraries/1/roots': () =>
        problem(422, 'validation_failed', {
          errors: [{ field: 'roots[0].path', code: 'library_path_invalid' }],
        }),
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup();

    let dialog = await openFolders(user);
    await user.click(within(dialog).getByRole('button', { name: 'Ordner hinzufügen' }));
    await user.type(within(dialog).getByLabelText('Ordner'), '/etc');
    await user.click(within(dialog).getByRole('button', { name: '1 Ordner hinzufügen' }));
    expect(
      await within(dialog).findByText(/Diesen Ordner gibt es auf dem Server nicht/),
    ).toBeVisible();

    await user.click(within(dialog).getByRole('button', { name: 'Schliessen' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());

    dialog = await openFolders(user);
    expect(within(dialog).queryByText(/Diesen Ordner gibt es auf dem Server nicht/)).toBeNull();
    // The cards of «Neue Ordner» are gone with the dialog.
    expect(within(dialog).queryByLabelText('Ordner')).toBeNull();
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

  it('«Ändern» orders the sources by position and names the new identifier (S-16)', async () => {
    const series = library({
      id: 2,
      name: 'Serien',
      roots: [
        {
          id: 5,
          path: '/media/Serien',
          content: 'series',
          providers: ['tmdb', 'tvdb'],
          fsType: 'ext4',
          watch: 'inotify',
          itemCount: 2,
          fileCount: 6,
        },
      ],
    });
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/libraries': () => json([series]),
      'GET /admin/metadata/providers': () => json(providers),
      'PATCH /admin/libraries/2/roots/5': () => json(series),
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup();

    const detail = await openRow(user, 'Serien');
    await user.click(within(detail).getByRole('button', { name: 'Ordner von Serien bearbeiten' }));
    const dialog = await screen.findByRole('dialog', { name: 'Ordner von Serien' });
    await user.click(within(dialog).getByRole('button', { name: 'Ordner /media/Serien ändern' }));
    const change = await screen.findByRole('dialog', { name: 'Ordner ändern' });
    // No arrows: the position is a choice, its hint says what the others do.
    expect(within(change).queryByRole('button', { name: /nach oben/ })).toBeNull();
    const tvdb = await within(change).findByLabelText('Platz für TVDB');
    expect(tvdb).toHaveAccessibleDescription('Die anderen Einträge rücken nach.');
    // Without a change «Speichern» works but sends nothing; the dialog and the focus stay
    // (decision 18).
    const save = within(change).getByRole('button', { name: 'Speichern' });
    expect(save).toBeEnabled();
    await user.click(save);
    expect(save).toHaveFocus();
    expect(change).toBeInTheDocument();
    expect(calls.some((call) => call.method === 'PATCH')).toBe(false);
    await user.selectOptions(tvdb, 'Platz 1');
    expect(within(change).getByText(/Der Erkenner wechselt/)).toBeVisible();
    await user.click(within(change).getByRole('button', { name: 'Speichern' }));

    await waitFor(() =>
      expect(calls.find((call) => call.method === 'PATCH')?.body).toEqual({
        providers: ['tvdb', 'tmdb'],
      }),
    );
  });

  it('orders the sources of a new folder by position as well', async () => {
    mockApi({
      ...signedIn,
      'GET /admin/libraries': () => json([library()]),
      'GET /admin/metadata/providers': () => json(providers),
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup();

    const dialog = await openFolders(user);
    await user.click(within(dialog).getByRole('button', { name: 'Ordner hinzufügen' }));
    await user.click(within(dialog).getByRole('radio', { name: 'Serien' }));
    expect(await within(dialog).findByLabelText('Platz für TVDB')).toBeVisible();
    expect(within(dialog).queryByRole('button', { name: /nach oben/ })).toBeNull();
  });

  /** The browser of «Pfad ändern»: one root /media with the folders Filme and Filme-neu. */
  const listing = (rel: string) => ({
    rootId: 'r1',
    rel,
    path: rel ? `/media/${rel}` : '/media',
    atRoot: rel === '',
    ...(rel ? { parentRel: '' } : {}),
    fsType: 'ext4',
    watch: 'inotify',
    entries: rel
      ? []
      : [
          { name: 'Filme', rel: 'Filme', path: '/media/Filme', libraryName: 'Filme' },
          { name: 'Filme-neu', rel: 'Filme-neu', path: '/media/Filme-neu' },
        ],
  });
  const browse = {
    'GET /admin/fs/roots': () =>
      json([{ id: 'r1', path: '/media', source: 'configured', browsable: true, fsType: 'ext4' }]),
    'POST /admin/fs/browse': (call: { body: unknown }) =>
      json(listing((call.body as { rel?: string }).rel ?? '')),
  };

  /** Opens «Pfad ändern» of /media/Filme, goes up, enters Filme-neu and applies it. */
  async function pickNewPath(user: User) {
    const dialog = await openFolders(user);
    await user.click(within(dialog).getByRole('button', { name: 'Pfad von /media/Filme ändern' }));
    const browser = await screen.findByRole('dialog', { name: 'Ordner auswählen' });
    // The browser opens in the folder's own path.
    await waitFor(() =>
      expect(within(browser).getByRole('textbox', { name: 'Ordner' })).toHaveValue('/media/Filme'),
    );
    await user.click(within(browser).getByRole('button', { name: 'Eine Ebene höher' }));
    await user.click(await within(browser).findByRole('option', { name: /Filme-neu/ }));
    await waitFor(() =>
      expect(within(browser).getByRole('textbox', { name: 'Ordner' })).toHaveValue(
        '/media/Filme-neu',
      ),
    );
    await user.click(within(browser).getByRole('button', { name: 'OK' }));
    return { dialog, confirm: await screen.findByRole('alertdialog', { name: 'Pfad ändern?' }) };
  }

  it('«Pfad ändern» picks the new folder, names both paths and sends only the path', async () => {
    // The notice names the path the server stored, here written differently than sent.
    const moved = library({ roots: [{ ...library().roots[0]!, path: '/media/filme-neu' }] });
    // The list the server answers with follows the change, like the real one.
    let current = library();
    const { calls } = mockApi({
      ...signedIn,
      ...browse,
      'GET /admin/libraries': () => json([current]),
      'PATCH /admin/libraries/1/roots/1': () => {
        current = moved;
        return json(moved);
      },
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup();

    const { dialog, confirm } = await pickNewPath(user);
    expect(
      within(confirm).getByText(
        'Alter Pfad: /media/Filme · Neuer Pfad: /media/Filme-neu. Titel und Benutzerdaten bleiben erhalten, wenn die Dateien dort unter denselben Namen liegen. Danach wird der Ordner neu eingelesen.',
      ),
    ).toBeVisible();
    // Nothing is sent before the confirmation.
    expect(calls.some((call) => call.method === 'PATCH')).toBe(false);
    await user.click(within(confirm).getByRole('button', { name: 'Ändern' }));

    await waitFor(() =>
      expect(calls.find((call) => call.method === 'PATCH')?.body).toEqual({
        path: '/media/Filme-neu',
      }),
    );
    expect(
      await within(dialog).findByText(
        'Der Ordner liegt jetzt unter /media/filme-neu und wird neu eingelesen.',
      ),
    ).toBeVisible();
    expect(within(dialog).getByText('/media/filme-neu')).toBeVisible();
  });

  it('«Pfad ändern» sends nothing on Abbrechen or for the same folder', async () => {
    const { calls } = mockApi({
      ...signedIn,
      ...browse,
      'GET /admin/libraries': () => json([library()]),
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup();

    const { confirm } = await pickNewPath(user);
    await user.click(within(confirm).getByRole('button', { name: 'Abbrechen' }));
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull());

    // The browser opens in the folder's own path; OK there is no change and asks nothing.
    const dialog = screen.getByRole('dialog', { name: 'Ordner von Filme' });
    await user.click(within(dialog).getByRole('button', { name: 'Pfad von /media/Filme ändern' }));
    const browser = await screen.findByRole('dialog', { name: 'Ordner auswählen' });
    const field = within(browser).getByRole('textbox', { name: 'Ordner' });
    await waitFor(() => expect(field).toHaveValue('/media/Filme'));
    // Written with a trailing slash and a space, it is still the same folder.
    await user.clear(field);
    await user.type(field, '/media/Filme/ ');
    await user.click(within(browser).getByRole('button', { name: 'OK' }));
    await waitFor(() =>
      expect(screen.queryByRole('dialog', { name: 'Ordner auswählen' })).toBeNull(),
    );
    expect(screen.queryByRole('alertdialog')).toBeNull();
    expect(calls.some((call) => call.method === 'PATCH')).toBe(false);
  });

  it.each([
    [problem(409, 'scan_running'), /Ein Scan dieser Bibliothek läuft gerade/],
    [
      problem(422, 'validation_failed', { errors: [{ field: 'path', code: 'root_overlap' }] }),
      'Dieser Ordner überschneidet sich mit einem anderen Ordner.',
    ],
    [
      problem(422, 'validation_failed', {
        errors: [{ field: 'path', code: 'library_path_invalid' }],
      }),
      /Diesen Ordner gibt es auf dem Server nicht/,
    ],
    [
      problem(422, 'validation_failed', {
        errors: [{ field: 'path', code: 'root_files_missing' }],
      }),
      'Unter dem neuen Pfad liegt keine der bisherigen Dateien.',
    ],
  ])('«Pfad ändern» reports what the server refuses (%#)', async (answer, message) => {
    mockApi({
      ...signedIn,
      ...browse,
      'GET /admin/libraries': () => json([library()]),
      'PATCH /admin/libraries/1/roots/1': () => answer,
    });
    renderApp('/admin/libraries');
    const user = userEvent.setup();

    const { dialog, confirm } = await pickNewPath(user);
    await user.click(within(confirm).getByRole('button', { name: 'Ändern' }));
    expect(await within(dialog).findByText(message)).toBeVisible();
    // The folder keeps its path.
    expect(within(dialog).getByText('/media/Filme')).toBeVisible();
    // Another folder action starts: the refusal belonged to «Pfad ändern» only.
    await user.click(within(dialog).getByRole('button', { name: 'Ordner /media/Filme entfernen' }));
    await waitFor(() => expect(within(dialog).queryByText(message)).toBeNull());
  });
});
