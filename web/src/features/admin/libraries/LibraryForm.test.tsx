import type { Library, LibrarySave, MetadataProviderInfo } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { library, metadataSettings } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, signedIn, type Call } from '@/test/api';

// «Bibliothek bearbeiten» (concept 24 4.1 to 4.3): the library's values in the form of
// «Hinzufügen», one closed card per folder, and everything saved in one `saveLibrary`. The cases
// of the former folders dialog (LibraryFoldersDialog) are checked here on the form.

const second = {
  id: 2,
  path: '/media/Filme-2',
  content: 'movie' as const,
  providers: ['tmdb' as const],
  fsType: 'nfs' as const,
  watch: 'poll' as const,
  warning: 'network_share',
  readOnly: true,
  itemCount: 3,
  fileCount: 4,
  textLangs: null,
  images: null,
  homeTitleCount: 3,
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
  {
    id: 'anilist',
    configured: true,
    roles: ['fill'],
    contents: ['movie', 'series'],
    languages: ['en'],
  },
];

type User = ReturnType<typeof userEvent.setup>;

/** The routes of the page with the given libraries; `put` answers saveLibrary. */
const routes = (libraries: Library[], extra: Record<string, unknown> = {}) => ({
  ...signedIn,
  'GET /admin/libraries': () => json(libraries),
  'GET /admin/metadata/providers': () => json(providers),
  'GET /admin/settings/metadata': () => json(metadataSettings()),
  ...extra,
});

/** saveLibrary answering with the library as the form sent it (ids of new folders made up). */
const saved =
  (base: Library, metadataChanged: { rootId: number; homeTitleCount: number }[] = []) =>
  (call: Call) => {
    const body = call.body as LibrarySave;
    const roots = body.roots.map((root, index) => ({
      ...(base.roots.find((stored) => stored.id === root.id) ?? second),
      id: root.id ?? 100 + index,
      path: root.path,
      content: root.content,
      providers: root.providers,
      textLangs: root.textLangs ?? null,
      images: root.images ?? null,
    }));
    return json({
      library: { ...base, name: body.name, roots, revision: 'rev-2' },
      metadataChanged,
    });
  };

const puts = (calls: Call[]) => calls.filter((call) => call.method === 'PUT');

/** Opens «Bearbeiten» from the fold-out of the library and returns the dialog. */
async function openEdit(user: User, name = 'Filme') {
  const toggle = await screen.findByRole('button', { name });
  if (toggle.getAttribute('aria-expanded') !== 'true') await user.click(toggle);
  const detail = await screen.findByRole('region', { name });
  await user.click(within(detail).getByRole('button', { name: `${name} bearbeiten` }));
  return screen.findByRole('dialog', { name: 'Bibliothek bearbeiten' });
}

const cardOf = (dialog: HTMLElement, number: number) =>
  within(dialog).getByRole('listitem', { name: `Ordner ${number}` });

/** The head of a card: the button that folds it. */
const headOf = (card: HTMLElement) => within(card).getAllByRole('button')[0]!;

async function openCard(user: User, card: HTMLElement) {
  const head = headOf(card);
  if (head.getAttribute('aria-expanded') !== 'true') await user.click(head);
}

/** «Speichern» of the form (the switch «Speichern» of the trickplay section is no button). */
const saveOf = (dialog: HTMLElement) => within(dialog).getByRole('button', { name: 'Speichern' });

describe('library form «Bearbeiten» (concept 24 4.1 to 4.3)', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('fills the form with the library: kind locked, every card closed to one line, the facts inside', async () => {
    mockApi(routes([withTwoFolders]));
    renderApp('/admin/libraries');
    const user = userEvent.setup();
    const dialog = await openEdit(user);

    expect(within(dialog).getByLabelText('Name')).toHaveValue('Filme');
    expect(within(dialog).getByRole('radio', { name: 'Video' })).toBeChecked();
    expect(within(dialog).getByRole('radio', { name: 'Video' })).toBeDisabled();
    expect(
      within(dialog).getByText('Die Art steht fest, sobald die Bibliothek angelegt ist.'),
    ).toBeVisible();
    const first = cardOf(dialog, 1);
    const card = cardOf(dialog, 2);
    for (const each of [first, card])
      expect(headOf(each)).toHaveAttribute('aria-expanded', 'false');
    // The closed line: path, content, chain, languages and images.
    expect(headOf(card)).toHaveTextContent('/media/Filme-2');
    expect(card).toHaveTextContent(/Filme.*TMDB.*Sprachen: Standard.*Bilder: Standard/);
    expect(within(card).queryByText('geändert')).toBeNull();

    // Open: the facts of the folder below its path (4.2).
    await openCard(user, card);
    expect(within(card).getByLabelText('Ordner')).toHaveValue('/media/Filme-2');
    expect(within(card).getByText('Titel: 3 · Dateien: 4')).toBeVisible();
    expect(within(card).getByText('Dateisystem nfs')).toBeVisible();
    expect(within(card).getByText('Netzlaufwerk')).toBeVisible();
    expect(within(card).getByText('nur lesbar')).toBeVisible();
    expect(within(card).getByText('Änderungen werden regelmässig abgefragt')).toBeVisible();
    expect(within(card).getByRole('button', { name: 'Durchsuchen …' })).toBeVisible();
    expect(within(card).getByRole('button', { name: 'Ordner 2 entfernen' })).toHaveTextContent(
      'Ordner entfernen …',
    );
    // The buttons of 4.1.
    expect(within(dialog).getByRole('button', { name: 'Abbrechen' })).toBeVisible();
    expect(saveOf(dialog)).toBeVisible();
  });

  it('closes without changes at once; with changes «Abbrechen» and Escape ask «Änderungen verwerfen?»', async () => {
    const { calls } = mockApi(routes([library()]));
    renderApp('/admin/libraries');
    const user = userEvent.setup();

    let dialog = await openEdit(user);
    await user.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());

    // «Speichern» without a change sends nothing and closes (decision 18).
    dialog = await openEdit(user);
    await user.click(saveOf(dialog));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expect(puts(calls)).toHaveLength(0);

    dialog = await openEdit(user);
    await user.type(within(dialog).getByLabelText('Name'), ' neu');
    await user.keyboard('{Escape}');
    let question = await screen.findByRole('alertdialog', { name: 'Änderungen verwerfen?' });
    expect(question).toHaveTextContent('Die ungespeicherten Änderungen an Filme gehen verloren.');
    // The safe answer has the focus; it keeps everything typed.
    const keep = within(question).getByRole('button', { name: 'Weiter bearbeiten' });
    await waitFor(() => expect(keep).toHaveFocus());
    await user.click(keep);
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull());
    expect(within(dialog).getByLabelText('Name')).toHaveValue('Filme neu');

    await user.click(within(dialog).getByRole('button', { name: 'Abbrechen' }));
    question = await screen.findByRole('alertdialog', { name: 'Änderungen verwerfen?' });
    await user.click(within(question).getByRole('button', { name: 'Verwerfen' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    // No kept draft of an edit: the next opening starts at the library.
    dialog = await openEdit(user);
    expect(within(dialog).getByLabelText('Name')).toHaveValue('Filme');
    expect(puts(calls)).toHaveLength(0);
  });

  it('removes a folder only when saved and after its path was typed out; «Rückgängig» takes the mark back', async () => {
    const { calls } = mockApi(
      routes([withTwoFolders], { 'PUT /admin/libraries/1': saved(withTwoFolders) }),
    );
    renderApp('/admin/libraries');
    const user = userEvent.setup();
    const dialog = await openEdit(user);

    const card = cardOf(dialog, 2);
    await openCard(user, card);
    await user.click(within(card).getByRole('button', { name: 'Ordner 2 entfernen' }));
    // Only marked: nothing is sent, the card closes and offers the way back.
    expect(cardOf(dialog, 2)).toHaveTextContent('Wird beim Speichern entfernt');
    const undo = within(cardOf(dialog, 2)).getByRole('button', {
      name: 'Entfernen von Ordner 2 rückgängig machen',
    });
    expect(undo).toHaveFocus();
    await user.click(undo);
    expect(cardOf(dialog, 2)).not.toHaveTextContent('Wird beim Speichern entfernt');
    await openCard(user, cardOf(dialog, 2));
    await user.click(within(cardOf(dialog, 2)).getByRole('button', { name: 'Ordner 2 entfernen' }));
    expect(puts(calls)).toHaveLength(0);

    await user.click(saveOf(dialog));
    const confirm = await screen.findByRole('dialog', { name: 'Änderungen bestätigen' });
    // The text says what really happens, including what stays.
    expect(within(confirm).getByText('Ordner entfernen: /media/Filme-2')).toBeVisible();
    expect(within(confirm).getByText(/weich gelöscht/)).toBeVisible();
    expect(within(confirm).getByText(/Offline-Downloads/)).toBeVisible();
    expect(within(confirm).getByText(/Radarr und Sonarr/)).toBeVisible();
    expect(within(confirm).getByText(/über ihre TMDB-, TVDB- oder IMDb-Id wieder/)).toBeVisible();
    expect(within(confirm).getByText(/SLOUCHCAT_BROWSE_ROOTS/)).toBeVisible();
    expect(within(confirm).queryByText(/letzte Ordner/)).toBeNull();
    const submit = within(confirm).getByRole('button', { name: 'Speichern' });
    expect(submit).toBeDisabled();
    // The reason stands beside the disabled button (Z-5).
    expect(within(confirm).getByText(/sobald jeder Pfad genau abgetippt ist/)).toBeVisible();
    const field = within(confirm).getByLabelText('Tippe den Pfad, um zu bestätigen');
    await user.type(field, '/media/Filme');
    expect(submit).toBeDisabled();
    // Surrounding spaces are forgiven; the server compares the path itself.
    await user.type(field, '-2  ');
    await waitFor(() => expect(submit).toBeEnabled());
    await user.click(submit);

    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expect(puts(calls)).toHaveLength(1);
    expect(puts(calls)[0]).toMatchObject({
      path: '/admin/libraries/1',
      body: {
        revision: 'rev-1',
        roots: [{ id: 1, path: '/media/Filme' }],
        removeRoots: [{ id: 2, confirmPath: '/media/Filme-2' }],
      },
    });
    // One band says what changed (4.3).
    expect(screen.getByText('Die Bibliothek Filme ist gespeichert.')).toBeVisible();
    expect(screen.getByText('1 Ordner entfernt.')).toBeVisible();
  });

  it('the confirmation of the last folder names that consequence; a closed one starts the typing again', async () => {
    mockApi(routes([library()]));
    renderApp('/admin/libraries');
    const user = userEvent.setup();
    const dialog = await openEdit(user);
    await openCard(user, cardOf(dialog, 1));
    await user.click(within(cardOf(dialog, 1)).getByRole('button', { name: 'Ordner 1 entfernen' }));
    await user.click(saveOf(dialog));
    let confirm = await screen.findByRole('dialog', { name: 'Änderungen bestätigen' });
    expect(within(confirm).getByText(/Es ist der letzte Ordner/)).toBeVisible();
    await user.type(
      within(confirm).getByLabelText('Tippe den Pfad, um zu bestätigen'),
      '/media/Filme',
    );
    await user.click(within(confirm).getByRole('button', { name: 'Abbrechen' }));
    await waitFor(() =>
      expect(screen.queryByRole('dialog', { name: 'Änderungen bestätigen' })).toBeNull(),
    );
    // The form keeps the typed path while it is open, so «Speichern» again does not ask twice.
    await user.click(saveOf(dialog));
    confirm = await screen.findByRole('dialog', { name: 'Änderungen bestätigen' });
    expect(within(confirm).getByLabelText('Tippe den Pfad, um zu bestätigen')).toHaveValue(
      '/media/Filme',
    );
  });

  it('a content change warns in the card and needs the typed path; path and content together are refused before the save', async () => {
    const { calls } = mockApi(routes([library()], { 'PUT /admin/libraries/1': saved(library()) }));
    renderApp('/admin/libraries');
    const user = userEvent.setup();
    const dialog = await openEdit(user);
    const card = cardOf(dialog, 1);
    await openCard(user, card);

    await user.click(within(card).getByRole('radio', { name: 'Serien' }));
    expect(within(card).getByText(/Der Inhalt wechselt/)).toBeVisible();
    expect(within(card).getByText('geändert')).toBeVisible();
    const path = within(card).getByLabelText('Ordner');
    await user.clear(path);
    await user.type(path, '/media/Serien');
    await user.click(saveOf(dialog));
    expect(
      await within(card).findByText(/Pfad und Inhalt dieses Ordners lassen sich nicht/),
    ).toBeVisible();
    expect(screen.queryByRole('dialog', { name: 'Änderungen bestätigen' })).toBeNull();

    await user.clear(path);
    await user.type(path, '/media/Filme');
    await user.click(saveOf(dialog));
    const confirm = await screen.findByRole('dialog', { name: 'Änderungen bestätigen' });
    expect(within(confirm).getByText('Inhalt wechseln: /media/Filme')).toBeVisible();
    expect(within(confirm).getByText(/Der Inhalt wechselt/)).toBeVisible();
    await user.type(
      within(confirm).getByLabelText('Tippe den Pfad, um zu bestätigen'),
      '/media/Filme',
    );
    await user.click(within(confirm).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(puts(calls)).toHaveLength(1));
    expect(puts(calls)[0]?.body).toMatchObject({
      roots: [
        {
          id: 1,
          path: '/media/Filme',
          content: 'series',
          providers: ['tmdb', 'tvdb'],
          confirmPath: '/media/Filme',
        },
      ],
      removeRoots: [],
    });
    expect(
      await screen.findByText('1 Ordner mit anderem Inhalt wird neu eingelesen.'),
    ).toBeVisible();
  });

  it('a new path names the old and the new one; a refusal lands on the card it belongs to and opens it', async () => {
    const { calls } = mockApi(
      routes([withTwoFolders], {
        'PUT /admin/libraries/1': () =>
          problem(422, 'validation_failed', {
            // The request's first folder is the form's second card: the first is removed.
            errors: [{ field: 'roots[0].path', code: 'root_files_missing' }],
          }),
      }),
    );
    renderApp('/admin/libraries');
    const user = userEvent.setup();
    const dialog = await openEdit(user);
    await openCard(user, cardOf(dialog, 1));
    await user.click(within(cardOf(dialog, 1)).getByRole('button', { name: 'Ordner 1 entfernen' }));
    const card = cardOf(dialog, 2);
    await openCard(user, card);
    const path = within(card).getByLabelText('Ordner');
    await user.clear(path);
    await user.type(path, '/media/Filme-neu/');
    expect(within(card).getByText(/Der neue Pfad gilt nach dem Speichern/)).toBeVisible();
    // Closed again: the message of the server must open it.
    await user.click(headOf(card));
    await user.click(saveOf(dialog));
    const confirm = await screen.findByRole('dialog', { name: 'Änderungen bestätigen' });
    expect(within(confirm).getByText('Pfad ändern: /media/Filme-2')).toBeVisible();
    expect(
      within(confirm).getByText(/^Alter Pfad: \/media\/Filme-2 · Neuer Pfad: \/media\/Filme-neu\./),
    ).toBeVisible();
    // Only the removal is typed; the new path needs no typed path.
    expect(within(confirm).getAllByLabelText('Tippe den Pfad, um zu bestätigen')).toHaveLength(1);
    await user.type(
      within(confirm).getByLabelText('Tippe den Pfad, um zu bestätigen'),
      '/media/Filme',
    );
    await user.click(within(confirm).getByRole('button', { name: 'Speichern' }));

    await waitFor(() =>
      expect(screen.queryByRole('dialog', { name: 'Änderungen bestätigen' })).toBeNull(),
    );
    expect(headOf(cardOf(dialog, 2))).toHaveAttribute('aria-expanded', 'true');
    expect(within(cardOf(dialog, 2)).getByLabelText('Ordner')).toHaveAccessibleDescription(
      'Unter dem neuen Pfad liegt keine der bisherigen Dateien.',
    );
    expect(puts(calls)[0]?.body).toMatchObject({
      roots: [{ id: 2, path: '/media/Filme-neu/' }],
      removeRoots: [{ id: 1, confirmPath: '/media/Filme' }],
    });
  });

  it('409 stale fills the form anew with the library as it is now and says so', async () => {
    let current = library();
    mockApi(
      routes([], {
        'GET /admin/libraries': () => json([current]),
        'PUT /admin/libraries/1': () => {
          current = library({ name: 'Kino', revision: 'rev-9' });
          return problem(409, 'stale');
        },
      }),
    );
    renderApp('/admin/libraries');
    const user = userEvent.setup();
    let dialog = await openEdit(user);
    await user.type(within(dialog).getByLabelText('Name'), ' neu');
    await user.click(saveOf(dialog));
    expect(
      await screen.findByText(/Die Bibliothek wurde inzwischen anderswo geändert/),
    ).toBeVisible();
    dialog = screen.getByRole('dialog', { name: 'Bibliothek bearbeiten' });
    expect(within(dialog).getByLabelText('Name')).toHaveValue('Kino');
  });

  it('409 scan_running keeps the confirmation with every input and offers «Scan abbrechen»', async () => {
    const { calls } = mockApi(
      routes([withTwoFolders], {
        'PUT /admin/libraries/1': () => problem(409, 'scan_running'),
        'DELETE /admin/libraries/1/scan': () => json(withTwoFolders, 202),
      }),
    );
    renderApp('/admin/libraries');
    const user = userEvent.setup();
    const dialog = await openEdit(user);
    await openCard(user, cardOf(dialog, 2));
    await user.click(within(cardOf(dialog, 2)).getByRole('button', { name: 'Ordner 2 entfernen' }));
    await user.click(saveOf(dialog));
    const confirm = await screen.findByRole('dialog', { name: 'Änderungen bestätigen' });
    const field = within(confirm).getByLabelText('Tippe den Pfad, um zu bestätigen');
    await user.type(field, '/media/Filme-2');
    await user.click(within(confirm).getByRole('button', { name: 'Speichern' }));
    expect(
      await within(confirm).findByText(/Ein Scan dieser Bibliothek läuft gerade/),
    ).toBeVisible();
    expect(field).toHaveValue('/media/Filme-2');
    await user.click(within(confirm).getByRole('button', { name: 'Scan abbrechen' }));
    await waitFor(() =>
      expect(
        calls.some((call) => call.method === 'DELETE' && call.path === '/admin/libraries/1/scan'),
      ).toBe(true),
    );
    expect(await within(confirm).findByText(/Der Scan ist abgebrochen/)).toBeVisible();
  });

  it('orders the sources by position; the card names a new identifier or new fillers (S-16, 4.2)', async () => {
    const series = library({
      id: 2,
      name: 'Serien',
      roots: [
        {
          ...library().roots[0]!,
          id: 5,
          path: '/media/Serien',
          content: 'series',
          providers: ['tmdb', 'tvdb'],
        },
      ],
    });
    const { calls } = mockApi(routes([series], { 'PUT /admin/libraries/2': saved(series) }));
    renderApp('/admin/libraries');
    const user = userEvent.setup();
    const dialog = await openEdit(user, 'Serien');
    const card = cardOf(dialog, 1);
    await openCard(user, card);
    expect(within(card).queryByRole('button', { name: /nach oben/ })).toBeNull();
    const tvdb = await within(card).findByLabelText('Platz für TVDB');
    expect(tvdb).toHaveAccessibleDescription('Die anderen Einträge rücken nach.');
    await user.click(within(card).getByRole('checkbox', { name: 'AniList' }));
    expect(within(card).getByText(/Nur die Auffüller ändern sich/)).toBeVisible();
    await user.selectOptions(tvdb, 'Platz 1');
    expect(within(card).getByText(/Der Erkenner wechselt/)).toBeVisible();
    expect(within(card).queryByText(/Nur die Auffüller ändern sich/)).toBeNull();
    // Sources alone confirm nothing (4.3).
    await user.click(saveOf(dialog));
    await waitFor(() => expect(puts(calls)).toHaveLength(1));
    expect(screen.queryByRole('dialog', { name: 'Änderungen bestätigen' })).toBeNull();
    expect(puts(calls)[0]?.body).toMatchObject({
      roots: [{ id: 5, providers: ['tvdb', 'tmdb', 'anilist'] }],
    });
    expect(
      await screen.findByText(
        'Neue Quellen bei 1 Ordner: Die Titel werden neu zugeordnet oder aufgefüllt.',
      ),
    ).toBeVisible();
  });

  it('adds a folder in «Bearbeiten»: the new card is open and goes without id, its sources by position', async () => {
    const { calls } = mockApi(routes([library()], { 'PUT /admin/libraries/1': saved(library()) }));
    renderApp('/admin/libraries');
    const user = userEvent.setup();
    const dialog = await openEdit(user);
    await user.click(within(dialog).getByRole('button', { name: 'Weiteren Ordner hinzufügen' }));
    const card = cardOf(dialog, 2);
    expect(headOf(card)).toHaveAttribute('aria-expanded', 'true');
    const path = within(card).getByLabelText('Ordner');
    expect(path).toHaveFocus();
    expect(within(card).getByRole('radio', { name: 'Serien' })).toBeChecked();
    expect(await within(card).findByLabelText('Platz für TVDB')).toBeVisible();
    await user.type(path, '/media/Serien');
    expect(within(card).getByText('geändert')).toBeVisible();
    // A new folder needs no confirmation.
    await user.click(saveOf(dialog));
    await waitFor(() => expect(puts(calls)).toHaveLength(1));
    expect(puts(calls)[0]?.body).toMatchObject({
      roots: [
        { id: 1, path: '/media/Filme' },
        { path: '/media/Serien', content: 'series', providers: ['tmdb', 'tvdb'] },
      ],
    });
    expect((puts(calls)[0]?.body as LibrarySave).roots[1]).not.toHaveProperty('id');
    expect(await screen.findByText('1 Ordner hinzugefügt; der Scan ist eingereiht.')).toBeVisible();
  });

  it('an empty path of a closed card opens it instead of failing silently', async () => {
    mockApi(routes([library()]));
    renderApp('/admin/libraries');
    const user = userEvent.setup();
    const dialog = await openEdit(user);
    const card = cardOf(dialog, 1);
    await openCard(user, card);
    await user.clear(within(card).getByLabelText('Ordner'));
    await user.click(headOf(card));
    expect(headOf(card)).toHaveAttribute('aria-expanded', 'false');
    await user.click(saveOf(dialog));
    await waitFor(() => expect(headOf(card)).toHaveAttribute('aria-expanded', 'true'));
  });

  describe('languages and images of a folder (concept 24 4.2)', () => {
    const anime = library({
      id: 3,
      name: 'Anime',
      roots: [
        {
          ...library().roots[0]!,
          id: 7,
          path: '/media/Anime',
          content: 'series',
          providers: ['tmdb'],
          textLangs: ['ja', 'en'],
          images: { langs: { poster: ['ja', ''] }, kinds: ['logo'] },
          homeTitleCount: 40,
        },
      ],
    });

    it('«Wie Standard» names the default, «Eigene» starts from it; the band offers «Jetzt anwenden» for that folder', async () => {
      const { calls, fetchMock } = mockApi(
        routes([library()], {
          'PUT /admin/libraries/1': saved(library(), [{ rootId: 1, homeTitleCount: 12 }]),
          'POST /admin/metadata/apply': () => json({ jobId: 41, titles: 12 }, 202),
        }),
      );
      renderApp('/admin/libraries');
      const user = userEvent.setup();
      const dialog = await openEdit(user);
      const card = cardOf(dialog, 1);
      await openCard(user, card);
      const langs = await within(card).findByRole('radiogroup', { name: 'Sprachen' });
      expect(within(langs).getByRole('radio', { name: 'Wie Standard' })).toBeChecked();
      expect(langs).toHaveAccessibleDescription('Standard: Deutsch › Englisch');
      const images = within(card).getByRole('radiogroup', { name: 'Bilder' });
      expect(images).toHaveAccessibleDescription(
        'Standard: Poster, Hintergrund, Logo, Staffelposter und Folgenbild',
      );
      expect(within(card).queryByRole('group', { name: 'Metadaten-Sprachen' })).toBeNull();

      await user.click(within(langs).getByRole('radio', { name: 'Eigene' }));
      expect(langs).toHaveAccessibleDescription('Gilt nur für diesen Ordner.');
      const list = within(card).getByRole('group', { name: 'Metadaten-Sprachen' });
      expect(
        within(list)
          .getAllByRole<HTMLSelectElement>('combobox')
          .map((select) => select.value),
      ).toEqual(['de', 'en']);
      await user.click(within(list).getByRole('button', { name: 'Weitere Sprache' }));
      expect(
        within(card).getByText(/Jede weitere Sprache kostet beim Anwenden eine Anfrage/),
      ).toHaveTextContent(/3 Sprachen: bis zu 33 Titel am Tag/);
      await waitFor(() =>
        expect(
          fetchMock.mock.calls.some(([input]) =>
            input.url.endsWith('/admin/metadata/providers?textLangs=de&textLangs=en&textLangs=fr'),
          ),
        ).toBe(true),
      );
      // The closed line names the own chain.
      expect(headOf(card).parentElement!.parentElement).toHaveTextContent(
        'Sprachen: Deutsch › Englisch › Französisch',
      );
      await user.click(saveOf(dialog));
      await waitFor(() => expect(puts(calls)).toHaveLength(1));
      // The whole state of the folder: the own list, `null` for the images of the default.
      expect(puts(calls)[0]?.body).toMatchObject({
        roots: [{ id: 1, textLangs: ['de', 'en', 'fr'], images: null }],
      });
      expect(
        await screen.findByText(
          'Sprachen oder Bilder geändert bei 1 Ordner. Sie gelten für 12 Titel, sobald du sie anwendest.',
        ),
      ).toBeVisible();
      await user.click(screen.getByRole('button', { name: 'Jetzt anwenden' }));
      await waitFor(() =>
        expect(calls.find((call) => call.path === '/admin/metadata/apply')?.body).toEqual({
          what: ['texts'],
          rootIds: [1],
        }),
      );
      expect(
        await screen.findByText('Lauf 41 eingereiht: 12 Titel. Fortschritt unter Hintergrundjobs.'),
      ).toBeVisible();
    });

    it('back to the default sends null; switching back and forth before the save keeps the own list', async () => {
      const { calls } = mockApi(routes([anime], { 'PUT /admin/libraries/3': saved(anime) }));
      renderApp('/admin/libraries');
      const user = userEvent.setup();
      const dialog = await openEdit(user, 'Anime');
      const card = cardOf(dialog, 1);
      expect(card).toHaveTextContent(/Sprachen: Japanisch › Englisch.*Bilder: eigene/);
      await openCard(user, card);
      const langs = await within(card).findByRole('radiogroup', { name: 'Sprachen' });
      const images = within(card).getByRole('radiogroup', { name: 'Bilder' });
      expect(within(langs).getByRole('radio', { name: 'Eigene' })).toBeChecked();
      expect(within(card).getByRole('checkbox', { name: /Logo/ })).toBeChecked();
      expect(within(card).getByRole('checkbox', { name: /Staffelposter/ })).not.toBeChecked();
      const poster = within(card).getByRole('group', { name: 'Bildsprache Poster' });
      expect(
        within(poster)
          .getAllByRole<HTMLSelectElement>('combobox')
          .map((select) => select.value),
      ).toEqual(['ja', '']);

      await user.click(within(langs).getByRole('radio', { name: 'Wie Standard' }));
      await user.click(within(langs).getByRole('radio', { name: 'Eigene' }));
      expect(
        within(within(card).getByRole('group', { name: 'Metadaten-Sprachen' }))
          .getAllByRole<HTMLSelectElement>('combobox')
          .map((select) => select.value),
      ).toEqual(['ja', 'en']);
      // Back and forth is no change.
      expect(within(card).queryByText('geändert')).toBeNull();

      await user.click(within(langs).getByRole('radio', { name: 'Wie Standard' }));
      await user.click(within(images).getByRole('radio', { name: 'Wie Standard' }));
      await user.click(saveOf(dialog));
      await waitFor(() => expect(puts(calls)).toHaveLength(1));
      expect(puts(calls)[0]?.body).toMatchObject({
        roots: [{ id: 7, textLangs: null, images: null }],
      });
    });

    it('shows the server messages at their fields and opens the card that holds them', async () => {
      mockApi(
        routes([anime], {
          'PUT /admin/libraries/3': () =>
            problem(422, 'validation_failed', {
              errors: [
                { field: 'roots[0].textLangs', code: 'lang_unknown' },
                { field: 'roots[0].images.langs.poster', code: 'chain_duplicate' },
              ],
            }),
        }),
      );
      renderApp('/admin/libraries');
      const user = userEvent.setup();
      const dialog = await openEdit(user, 'Anime');
      const card = cardOf(dialog, 1);
      await openCard(user, card);
      await user.click(
        within(await within(card).findByRole('group', { name: 'Bildsprache Logo' })).getByRole(
          'button',
          { name: 'Weitere Sprache' },
        ),
      );
      await user.click(headOf(card));
      await user.click(saveOf(dialog));
      expect(await within(card).findByText('Jede Sprache nur einmal.')).toBeVisible();
      expect(headOf(card)).toHaveAttribute('aria-expanded', 'true');
      const poster = within(card).getByRole('group', { name: 'Bildsprache Poster' });
      expect(poster.parentElement).toHaveTextContent('Jede Sprache nur einmal.');
      expect(
        within(card).getByRole('group', { name: 'Metadaten-Sprachen' }).parentElement
          ?.parentElement,
      ).toHaveTextContent('Unbekannter Sprachcode.');
    });
  });
});
