import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { alienDetail, ids, seriesIds, showDetail } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, session, signedIn } from '@/test/api';

// «Löschen» of a title (concept 20 P-9): the round button on the detail page of the
// administrator, the dialog with the preview of the server, the typed title with more than one
// file, the refusal while a profile plays, and the way out to the library afterwards.

const filmPreview = {
  itemId: ids.alien,
  kind: 'movie',
  confirmTitle: 'Alien',
  typeTitle: false,
  files: [
    { rootPath: '/media/filme', path: 'Alien (1979)/Alien (1979).mkv', sizeBytes: 4_000_000_000 },
  ],
  folder: { rootPath: '/media/filme', path: 'Alien (1979)' },
  fileCount: 1,
  totalBytes: 4_000_000_000,
  playingProfiles: [],
  filesDeletable: false,
  filesReason: 'read_only',
};

describe('deleting a title', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('a viewer sees no delete', async () => {
    mockApi({
      'GET /auth/session': () =>
        json({ ...session, account: { ...session.account, role: 'user', isOwner: false } }),
      [`GET /items/${ids.alien}`]: () => json(alienDetail),
    });
    renderApp(`/items/${ids.alien}`);
    expect(await screen.findByRole('heading', { level: 1, name: 'Alien' })).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Löschen' })).toBeNull();
  });

  it('a film: its file with the whole path, the red button, then the library', async () => {
    const { calls } = mockApi({
      ...signedIn,
      [`GET /items/${ids.alien}`]: () => json({ ...alienDetail, libraryId: 1 }),
      [`GET /admin/items/${ids.alien}/deletion`]: () => json(filmPreview),
      [`POST /admin/items/${ids.alien}/deletion`]: () =>
        json({ removedItems: 1, deletedFiles: 0, deletedBytes: 0, leftover: [] }),
    });
    const { router } = renderApp(`/items/${ids.alien}`);
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Löschen' }));
    const dialog = await screen.findByRole('dialog', { name: '«Alien» löschen?' });
    expect(within(dialog).getByText('/media/filme/Alien (1979)/Alien (1979).mkv')).toBeVisible();
    expect(within(dialog).getByText(/Betroffen: 1 Datei/)).toBeVisible();
    expect(
      within(dialog).getByRole('radio', { name: 'Nur aus Slouchcat entfernen' }),
    ).toBeChecked();
    expect(within(dialog).getByText(/Radarr und Sonarr erfahren davon nichts/)).toBeVisible();
    // One file: no title to type.
    expect(within(dialog).queryByLabelText('Zur Bestätigung den Titel eintippen')).toBeNull();
    await user.click(within(dialog).getByRole('button', { name: 'Aus Slouchcat entfernen' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/libraries/1'));
    expect(calls.find((call) => call.method === 'POST')?.body).toEqual({ mode: 'library' });
  });

  it('a series: the folder, the typed title in any case; refused while a profile plays', async () => {
    let playing = ['Anna'];
    const { calls } = mockApi({
      ...signedIn,
      [`GET /items/${seriesIds.show}`]: () => json(showDetail),
      [`GET /admin/items/${seriesIds.show}/deletion`]: () =>
        json({
          itemId: seriesIds.show,
          kind: 'series',
          confirmTitle: 'The Show',
          typeTitle: true,
          files: [
            { rootPath: '/media/serien', path: 'The Show/Season 01/e1.mkv', sizeBytes: 1000 },
            { rootPath: '/media/serien', path: 'The Show/Season 01/e2.mkv', sizeBytes: 1000 },
          ],
          folder: { rootPath: '/media/serien', path: 'The Show' },
          fileCount: 2,
          totalBytes: 2000,
          playingProfiles: playing,
        }),
      [`POST /admin/items/${seriesIds.show}/deletion`]: () =>
        problem(409, 'playing', { playingProfiles: ['Ben'] }),
    });
    renderApp(`/items/${seriesIds.show}`);
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Löschen' }));
    let dialog = await screen.findByRole('dialog', { name: '«The Show» löschen?' });
    expect(within(dialog).getByText('/media/serien/The Show/')).toBeVisible();
    expect(within(dialog).getByText(/Läuft gerade bei Anna/)).toBeVisible();
    const confirm = () => within(dialog).getByRole('button', { name: 'Aus Slouchcat entfernen' });
    expect(confirm()).toBeDisabled();
    await user.click(within(dialog).getByRole('button', { name: 'Abbrechen' }));

    playing = [];
    await user.click(screen.getByRole('button', { name: 'Löschen' }));
    dialog = await screen.findByRole('dialog', { name: '«The Show» löschen?' });
    const field = within(dialog).getByLabelText('Zur Bestätigung den Titel eintippen');
    expect(confirm()).toBeDisabled();
    await user.type(field, ' the show ');
    expect(confirm()).toBeEnabled();
    await user.click(confirm());
    // Someone started meanwhile: the server refuses and names the profile.
    expect(
      await within(dialog).findByText(/Gerade läuft einer dieser Titel bei Ben/),
    ).toBeVisible();
    expect(calls.find((call) => call.method === 'POST')?.body).toEqual({
      mode: 'library',
      confirmTitle: 'the show',
    });
  });

  it('«Auch die Dateien löschen» is grey with the reason on a read-only folder', async () => {
    mockApi({
      ...signedIn,
      [`GET /items/${ids.alien}`]: () => json(alienDetail),
      [`GET /admin/items/${ids.alien}/deletion`]: () => json(filmPreview),
    });
    renderApp(`/items/${ids.alien}`);
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Löschen' }));
    const dialog = await screen.findByRole('dialog', { name: '«Alien» löschen?' });
    expect(within(dialog).getByRole('radio', { name: 'Auch die Dateien löschen' })).toBeDisabled();
    expect(
      within(dialog).getByText(/Geht nicht: der Ordner ist nur lesbar eingebunden/),
    ).toBeVisible();
  });

  it('with the files: «endgültig», the red button; a file that stayed keeps the page', async () => {
    let answer: 'kept' | 'gone' = 'kept';
    const { calls } = mockApi({
      ...signedIn,
      [`GET /items/${ids.alien}`]: () => json({ ...alienDetail, libraryId: 1 }),
      [`GET /admin/items/${ids.alien}/deletion`]: () =>
        json({ ...filmPreview, filesDeletable: true, filesReason: undefined }),
      [`POST /admin/items/${ids.alien}/deletion`]: () =>
        answer === 'kept'
          ? json({
              removedItems: 0,
              deletedFiles: 1,
              deletedBytes: 2e9,
              leftover: [
                { rootPath: '/media/filme', path: 'Alien (1979)/b.mkv', reason: 'failed' },
              ],
              leftInFolder: 3,
              titleDeleted: false,
            })
          : json({
              removedItems: 1,
              deletedFiles: 1,
              deletedBytes: 4e9,
              leftover: [],
              leftInFolder: 0,
              titleDeleted: true,
            }),
    });
    const { router } = renderApp(`/items/${ids.alien}`);
    const user = userEvent.setup();
    const open = async () => {
      await user.click(await screen.findByRole('button', { name: 'Löschen' }));
      return screen.findByRole('dialog', { name: '«Alien» löschen?' });
    };
    let dialog = await open();
    await user.click(within(dialog).getByText('Auch die Dateien löschen'));
    expect(within(dialog).getByText(/endgültig gelöscht, es gibt keinen Papierkorb/)).toBeVisible();
    await user.click(within(dialog).getByRole('button', { name: 'Endgültig löschen' }));
    expect(await screen.findByText(/der Titel bleibt mit den Dateien/)).toBeVisible();
    expect(
      screen.getByText(/3 Dateien, die Slouchcat nicht kennt, bleiben im Ordner/),
    ).toBeVisible();
    expect(router.state.location.pathname).toBe(`/items/${ids.alien}`);
    expect(calls.find((call) => call.method === 'POST')?.body).toEqual({ mode: 'files' });

    answer = 'gone';
    dialog = await open();
    await user.click(within(dialog).getByText('Auch die Dateien löschen'));
    await user.click(within(dialog).getByRole('button', { name: 'Endgültig löschen' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/libraries/1'));
  });
});
