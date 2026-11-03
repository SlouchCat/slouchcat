import type { QueryClient } from '@tanstack/react-query';
import type { Backup, RestoreResult } from '@slouchcat/api-client';
import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent, { type UserEvent } from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, session, signedIn, type Routes } from '@/test/api';
import { reloadToLogin } from './restart';

// A-4 (concept 12, variant C): the backups per kind with download, restore and delete in the
// row's menu; a new backup with or without secrets and the restore of an uploaded file in tabs of
// their own; every restore ends in the restart panel. The server checks archives and passphrases.

vi.mock('./restart', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./restart')>()),
  reloadToLogin: vi.fn(),
}));

const plain: Backup = {
  name: 'slouchcat-backup-0.9.0-20261021T120000Z.tar.gz',
  createdAt: '2026-10-21T12:00:00Z',
  sizeBytes: 4_200_000,
  includesSecrets: false,
  encrypted: false,
  version: '0.9.0',
  kind: 'manual',
};

const secret: Backup = {
  name: 'slouchcat-backup-0.9.0-20261020T080000Z.tar.gz.age',
  createdAt: '2026-10-20T08:00:00Z',
  sizeBytes: 4_300_000,
  includesSecrets: true,
  encrypted: true,
  version: '0.9.0',
  kind: 'manual',
};

const auto: Backup = {
  name: 'pre-migration-0.8.0-20261019T070000Z.sqlite',
  createdAt: '2026-10-19T07:00:00Z',
  sizeBytes: 3_900_000,
  includesSecrets: false,
  encrypted: false,
  kind: 'auto',
};

const accepted: RestoreResult = {
  restartAt: '2026-10-21T12:00:02Z',
  version: '0.9.0',
  includesSecrets: false,
};

const PASSPHRASE = 'lange-passphrase-42';

const health = (uptimeSeconds: number) => json({ status: 'ok', version: '0.9.0', uptimeSeconds });

const routes = (list: () => Backup[], extra: Routes = {}): Routes => ({
  ...signedIn,
  'GET /admin/backups': () => json(list()),
  'GET /healthz': () => health(3600),
  ...extra,
});

/** The row of a backup; a new one's header reads «<name> neu». */
const rowOf = (name: string) =>
  screen.getByRole('rowheader', { name: (text) => text.startsWith(name) }).closest('tr')!;
const heading = () => screen.getByRole('heading', { level: 1, name: 'Sicherung' });

/** Picks `item` from «Weitere Aktionen» of a backup's row (S-6). */
async function pick(user: UserEvent, name: string, item: string) {
  await user.click(
    within(rowOf(name)).getByRole('button', { name: `Weitere Aktionen für ${name}` }),
  );
  const menu = await screen.findByRole('menu');
  await user.click(within(menu).getByRole('menuitem', { name: item }));
}

/** Whether any mutation the query client holds carries the passphrase. */
const holdsPassphrase = (queryClient: QueryClient) =>
  queryClient
    .getMutationCache()
    .getAll()
    .some((mutation) => JSON.stringify(mutation.state.variables ?? null).includes(PASSPHRASE));

describe('admin backup (A-4)', () => {
  beforeEach(() => vi.mocked(reloadToLogin).mockClear());
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('lists the backups per kind with keys, encryption and a download link each (SIC-3)', async () => {
    mockApi(routes(() => [plain, secret, auto]));
    renderApp('/admin/backup');
    expect(await screen.findByRole('heading', { level: 1, name: 'Sicherung' })).toBeVisible();
    expect(await screen.findByRole('tab', { selected: true })).toHaveAccessibleName(
      'Sicherungen 3 Sicherungen',
    );
    // One section per kind: «Manuell» with the encrypted count, «Automatisch» with the sentence
    // on when the server makes them.
    expect(screen.getByRole('button', { name: 'Manuell 2 Sicherungen' })).toHaveAttribute(
      'aria-expanded',
      'true',
    );
    expect(screen.getByText(/^1 verschlüsselt · neueste/)).toBeVisible();
    expect(screen.getByRole('button', { name: 'Automatisch 1 Sicherung' })).toBeVisible();
    expect(
      screen.getByText(
        'Automatische Sicherungen legt der Server vor einer Migration der Datenbank (bei einem Update) und vor einer Wiederherstellung an.',
      ),
    ).toBeVisible();

    const manual = screen.getByRole('table', { name: 'Sicherungen: Manuell' });
    expect(
      within(manual)
        .getAllByRole('columnheader')
        .map((cell) => cell.textContent),
    ).toEqual(['Datei', 'Erstellt', 'Grösse', 'Schlüssel', 'Verschlüsselt', 'Aktionen']);
    const cells = (name: string) =>
      within(rowOf(name))
        .getAllByRole('cell')
        .map((cell) => cell.textContent);
    expect(cells(plain.name).slice(0, 4)).toEqual([
      expect.stringMatching(/2026/) as string,
      expect.stringMatching(/^4\sMB$/) as string,
      'Nein',
      'Nein',
    ]);
    expect(cells(secret.name).slice(2, 4)).toEqual(['Ja', 'Ja']);
    expect(
      within(screen.getByRole('table', { name: 'Sicherungen: Automatisch' })).getByRole(
        'rowheader',
        { name: auto.name },
      ),
    ).toBeVisible();
    const download = within(rowOf(plain.name)).getByRole('link', {
      name: `${plain.name} herunterladen`,
    });
    expect(download).toHaveAttribute('href', `/api/v1/admin/backups/${plain.name}/download`);
    expect(download).toHaveAttribute('download', plain.name);
  });

  it('folds a kind away by its heading; empty, the list says so', async () => {
    let list = [plain, auto];
    mockApi(routes(() => list));
    const { queryClient } = renderApp('/admin/backup');
    const user = userEvent.setup();
    const manual = await screen.findByRole('button', { name: 'Manuell 1 Sicherung' });
    manual.focus();
    await user.keyboard('{Enter}');
    expect(manual).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByRole('table', { name: 'Sicherungen: Manuell' })).toBeNull();
    expect(screen.getByRole('table', { name: 'Sicherungen: Automatisch' })).toBeVisible();
    await user.keyboard(' ');
    expect(manual).toHaveAttribute('aria-expanded', 'true');

    list = [];
    await act(() => queryClient.invalidateQueries({ queryKey: ['admin', 'backups'] }));
    expect(await screen.findByText('Noch keine Sicherungen.')).toBeVisible();
    expect(screen.queryByRole('table')).toBeNull();
  });

  it('opens a linked tab; an unknown ?tab= shows «Sicherungen» and leaves the address', async () => {
    mockApi(routes(() => [plain]));
    const linked = renderApp('/admin/backup?tab=upload');
    expect(await screen.findByRole('tab', { selected: true })).toHaveAccessibleName(
      'Aus Datei wiederherstellen',
    );
    linked.unmount();

    const { router } = renderApp('/admin/backup?tab=nonsense');
    expect(await screen.findByRole('tab', { selected: true })).toHaveAccessibleName(/^Sicherungen/);
    await waitFor(() => expect(router.state.location.search).toEqual({}));
    // Replaced, not pushed: going back does not return to the unknown tab.
    expect(router.history.length).toBe(1);
  });

  it('creates a backup without secrets by default; the list shows it as new (SIC-1)', async () => {
    let list = [plain];
    const created: Backup = { ...plain, name: 'slouchcat-backup-0.9.0-20261021T130000Z.tar.gz' };
    const { calls } = mockApi(
      routes(() => list, {
        'POST /admin/backups': () => {
          list = [created, plain];
          return json(created, 201);
        },
      }),
    );
    const { router } = renderApp('/admin/backup');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('tab', { name: 'Neue Sicherung' }));
    expect(router.state.location.search).toEqual({ tab: 'create' });
    const panel = screen.getByRole('tabpanel');
    expect(within(panel).getByRole('radio', { name: 'Ohne Schlüssel (empfohlen)' })).toBeChecked();
    expect(panel).toHaveTextContent('Medien gehören nie dazu.');
    expect(panel).toHaveTextContent('Nach einer Wiederherstellung gibst du sie neu ein.');
    // Private feed addresses stay in (the server keeps podcast subscriptions intact).
    expect(panel).toHaveTextContent(
      'Adressen privater Podcast-Feeds bleiben samt ihrem Zugangscode',
    );
    expect(within(panel).queryByLabelText(/^Passphrase/)).toBeNull();
    await user.click(within(panel).getByRole('button', { name: 'Sicherung erstellen' }));

    expect(await screen.findByText(`Sicherung „${created.name}“ erstellt.`)).toBeVisible();
    expect(calls.find((call) => call.method === 'POST')?.body).toEqual({ includeSecrets: false });
    // «Sicherungen» shows again with the new row; the h1 takes the focus the form's button had.
    await waitFor(() => expect(router.state.location.search).toEqual({}));
    const row = await screen.findByRole('rowheader', { name: `${created.name} neu` });
    expect(row).toBeVisible();
    await waitFor(() => expect(heading()).toHaveFocus());
  });

  it('creates a backup with secrets only with a passphrase of at least 12 characters; it never reaches the mutation cache', async () => {
    const { calls } = mockApi(
      routes(() => [], { 'POST /admin/backups': () => json({ ...secret }, 201) }),
    );
    const { queryClient } = renderApp('/admin/backup?tab=create');
    const user = userEvent.setup();
    const panel = await screen.findByRole('tabpanel');
    await user.click(within(panel).getByRole('radio', { name: 'Mit Schlüsseln' }));
    expect(panel).toHaveTextContent('mit einer Passphrase verschlüsselt (mindestens 12 Zeichen)');
    const passphrase = within(panel).getByLabelText('Passphrase');
    await user.type(passphrase, 'zu-kurz');
    await user.type(within(panel).getByLabelText('Passphrase wiederholen'), 'zu-kurz');
    await user.click(within(panel).getByRole('button', { name: 'Sicherung erstellen' }));
    expect(await within(panel).findByText('Mindestens 12 Zeichen.')).toBeVisible();

    await user.clear(passphrase);
    await user.type(passphrase, PASSPHRASE);
    await user.click(within(panel).getByRole('button', { name: 'Sicherung erstellen' }));
    expect(await within(panel).findByText('Die Passphrasen stimmen nicht überein.')).toBeVisible();
    expect(calls.some((call) => call.method === 'POST')).toBe(false);

    const repeat = within(panel).getByLabelText('Passphrase wiederholen');
    await user.clear(repeat);
    await user.type(repeat, PASSPHRASE);
    await user.click(within(panel).getByRole('button', { name: 'Sicherung erstellen' }));
    expect(await screen.findByText(`Sicherung „${secret.name}“ erstellt.`)).toBeVisible();
    expect(calls.find((call) => call.method === 'POST')?.body).toEqual({
      includeSecrets: true,
      passphrase: PASSPHRASE,
    });
    await waitFor(() => expect(queryClient.getMutationCache().getAll()).toHaveLength(0));
    expect(holdsPassphrase(queryClient)).toBe(false);

    // Back in «Neue Sicherung» the scope stays, the passphrases do not.
    await user.click(screen.getByRole('tab', { name: 'Neue Sicherung' }));
    const again = screen.getByRole('tabpanel');
    expect(within(again).getByRole('radio', { name: 'Mit Schlüsseln' })).toBeChecked();
    expect(within(again).getByLabelText('Passphrase')).toHaveValue('');
  });

  it('deletes a backup from the row menu after the confirmation; the focus goes to the heading', async () => {
    let list = [plain, secret];
    const { calls } = mockApi(
      routes(() => list, {
        // 409 restart_pending: a restart or restore is on its way, the page says so.
        [`POST /admin/backups/${plain.name}/restore`]: () => problem(409, 'restart_pending'),
        [`DELETE /admin/backups/${plain.name}`]: () => {
          list = [secret];
          return new Response(null, { status: 204 });
        },
      }),
    );
    renderApp('/admin/backup');
    const user = userEvent.setup();
    // A refused restore first: its notice goes away with the next outcome.
    await screen.findByRole('table', { name: 'Sicherungen: Manuell' });
    await pick(user, plain.name, 'Wiederherstellen …');
    const confirm = await screen.findByRole('alertdialog', { name: 'Alle Daten ersetzen?' });
    await user.click(within(confirm).getByRole('button', { name: 'Ersetzen und neu starten' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Der Server startet gleich neu. Warte, bis er wieder antwortet.',
    );
    expect(screen.queryByRole('heading', { level: 2, name: 'Server startet neu' })).toBeNull();

    await pick(user, plain.name, 'Löschen …');
    const dialog = await screen.findByRole('alertdialog', {
      name: `Sicherung „${plain.name}“ löschen?`,
    });
    expect(dialog).toHaveTextContent('Heruntergeladene Kopien bleiben.');
    await user.click(within(dialog).getByRole('button', { name: 'Sicherung löschen' }));
    await waitFor(() => expect(calls.some((call) => call.method === 'DELETE')).toBe(true));
    expect(await screen.findByText(`Sicherung „${plain.name}“ gelöscht.`)).toBeVisible();
    expect(screen.getByRole('status')).toHaveTextContent(`Sicherung „${plain.name}“ gelöscht.`);
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    await waitFor(() => expect(screen.queryByRole('rowheader', { name: plain.name })).toBeNull());
    await waitFor(() => expect(heading()).toHaveFocus());
  });

  it('puts «Wiederherstellen» and «Löschen» under «Gefahr»; Escape closes the confirmation without deleting', async () => {
    const { calls } = mockApi(routes(() => [plain]));
    renderApp('/admin/backup');
    const user = userEvent.setup();
    await screen.findByRole('table', { name: 'Sicherungen: Manuell' });
    const more = within(rowOf(plain.name)).getByRole('button', {
      name: `Weitere Aktionen für ${plain.name}`,
    });
    await user.click(more);
    const menu = await screen.findByRole('menu');
    const danger = within(menu).getByRole('group', { name: 'Gefahr' });
    expect(
      within(danger)
        .getAllByRole('menuitem')
        .map((item) => item.textContent),
    ).toEqual(['Wiederherstellen …', 'Löschen …']);
    expect(within(menu).getAllByRole('menuitem')).toHaveLength(2);
    await user.click(within(menu).getByRole('menuitem', { name: 'Löschen …' }));
    expect(await screen.findByRole('alertdialog')).toBeVisible();
    await user.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull());
    await waitFor(() => expect(more).toHaveFocus());
    expect(calls.some((call) => call.method === 'DELETE')).toBe(false);
  });

  it('restores a backup of the list after the warning; the restart panel waits for the new server (SIC-4)', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    let uptime = 3600;
    let down = false;
    const { calls } = mockApi(
      routes(() => [plain], {
        [`POST /admin/backups/${plain.name}/restore`]: () => json(accepted, 202),
        'GET /healthz': () => {
          if (down) throw new TypeError('Failed to fetch');
          return health(uptime);
        },
      }),
    );
    renderApp('/admin/backup');
    const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
    await screen.findByRole('table', { name: 'Sicherungen: Manuell' });
    await pick(user, plain.name, 'Wiederherstellen …');
    const dialog = await screen.findByRole('alertdialog', { name: 'Alle Daten ersetzen?' });
    expect(dialog).toHaveTextContent(`durch den Stand von „${plain.name}“ ersetzt`);
    expect(dialog).toHaveTextContent('startet der Server neu');
    // A restore starts on «Abbrechen»: a second Enter by habit must not replace every piece of data.
    await waitFor(() =>
      expect(within(dialog).getByRole('button', { name: 'Abbrechen' })).toHaveFocus(),
    );
    await user.click(within(dialog).getByRole('button', { name: 'Ersetzen und neu starten' }));

    const restarting = await screen.findByRole('heading', { level: 2, name: 'Server startet neu' });
    await waitFor(() => expect(restarting).toHaveFocus());
    expect(
      calls.find((call) => call.path === `/admin/backups/${plain.name}/restore`)?.body,
    ).toEqual({});
    expect(screen.getByText(/Version 0\.9\.0/)).toBeVisible();
    expect(screen.getByText(/Gib die Schlüssel, den E-Mail-Server/)).toBeVisible();
    expect(
      screen
        .getAllByRole('status')
        .some((region) => region.textContent?.includes('Warte auf den Server …')),
    ).toBe(true);
    expect(screen.queryByRole('table')).toBeNull();
    expect(screen.queryByRole('tablist')).toBeNull();

    // The old process still answers: no reload. Then it is gone, then the new one answers.
    const polls = () => calls.filter((call) => call.path === '/healthz').length;
    await waitFor(() => expect(polls()).toBe(1));
    await act(() => vi.advanceTimersByTimeAsync(2000));
    await waitFor(() => expect(polls()).toBe(2));
    expect(reloadToLogin).not.toHaveBeenCalled();
    down = true;
    await act(() => vi.advanceTimersByTimeAsync(2000));
    await waitFor(() => expect(polls()).toBe(3));
    expect(reloadToLogin).not.toHaveBeenCalled();
    down = false;
    uptime = 1;
    await act(() => vi.advanceTimersByTimeAsync(2000));
    await waitFor(() => expect(reloadToLogin).toHaveBeenCalledTimes(1));
    expect(screen.getByText('Der Server ist zurück.')).toBeVisible();
  });

  it('gives up after 90 s without the new server; «Erneut prüfen» waits again', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    let down = true;
    const { calls } = mockApi(
      routes(() => [plain], {
        [`POST /admin/backups/${plain.name}/restore`]: () => json(accepted, 202),
        'GET /healthz': () => {
          if (down) throw new TypeError('Failed to fetch');
          return health(1);
        },
      }),
    );
    renderApp('/admin/backup');
    const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
    await screen.findByRole('table', { name: 'Sicherungen: Manuell' });
    await pick(user, plain.name, 'Wiederherstellen …');
    const dialog = await screen.findByRole('alertdialog', { name: 'Alle Daten ersetzen?' });
    await user.click(within(dialog).getByRole('button', { name: 'Ersetzen und neu starten' }));
    const restarting = await screen.findByRole('heading', { level: 2, name: 'Server startet neu' });
    const polls = () => calls.filter((call) => call.path === '/healthz').length;

    await act(() => vi.advanceTimersByTimeAsync(88_000));
    expect(screen.queryByText('Der Server kommt nicht zurück')).not.toBeInTheDocument();
    await act(() => vi.advanceTimersByTimeAsync(2_000));
    expect(await screen.findByText('Der Server kommt nicht zurück')).toBeVisible();
    expect(screen.getByText('Seit 90 Sekunden antwortet kein neuer Server.')).toBeVisible();
    expect(screen.getByText(/docker logs slouchcat/)).toBeVisible();
    // No more polls while the notice shows.
    const stopped = polls();
    await act(() => vi.advanceTimersByTimeAsync(10_000));
    expect(polls()).toBe(stopped);
    expect(reloadToLogin).not.toHaveBeenCalled();

    down = false;
    await user.click(screen.getByRole('button', { name: 'Erneut prüfen' }));
    await waitFor(() => expect(reloadToLogin).toHaveBeenCalledTimes(1));
    expect(restarting).toHaveFocus();
    expect(screen.queryByText('Der Server kommt nicht zurück')).not.toBeInTheDocument();
  });

  it('asks for the passphrase of an encrypted backup in a dialog and shows a wrong one at the field', async () => {
    let answer = problem(422, 'passphrase_wrong');
    const { calls } = mockApi(
      routes(() => [secret], {
        [`POST /admin/backups/${secret.name}/restore`]: () => answer,
      }),
    );
    const { queryClient } = renderApp('/admin/backup');
    const user = userEvent.setup();
    await screen.findByRole('table', { name: 'Sicherungen: Manuell' });
    await pick(user, secret.name, 'Wiederherstellen …');
    const dialog = await screen.findByRole('dialog', { name: `„${secret.name}“ wiederherstellen` });
    expect(dialog).toHaveTextContent('ersetzt alle Daten dieses Servers');
    const passphrase = within(dialog).getByLabelText(/^Passphrase/);
    await waitFor(() => expect(passphrase).toHaveFocus());
    const restore = within(dialog).getByRole('button', { name: 'Wiederherstellen' });
    expect(restore).toBeDisabled();
    await user.type(passphrase, 'falsche-passphrase');
    await user.click(restore);
    const confirm = await screen.findByRole('alertdialog', { name: 'Alle Daten ersetzen?' });
    expect(confirm).toHaveTextContent(`durch den Stand von „${secret.name}“ ersetzt`);
    await user.click(within(confirm).getByRole('button', { name: 'Ersetzen und neu starten' }));
    expect(await within(dialog).findByText('Die Passphrase stimmt nicht.')).toBeVisible();
    expect(calls.find((call) => call.method === 'POST')?.body).toEqual({
      passphrase: 'falsche-passphrase',
    });

    answer = json({ ...accepted, includesSecrets: true }, 202);
    await user.clear(passphrase);
    await user.type(passphrase, PASSPHRASE);
    expect(within(dialog).queryByText('Die Passphrase stimmt nicht.')).toBeNull();
    // Enter in the field asks the same confirmation as the button; its focus is on «Abbrechen»,
    // so a second Enter cancels.
    await user.keyboard('{Enter}');
    const second = await screen.findByRole('alertdialog');
    await waitFor(() =>
      expect(within(second).getByRole('button', { name: 'Abbrechen' })).toHaveFocus(),
    );
    await user.keyboard('{Enter}');
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull());
    expect(calls.filter((call) => call.method === 'POST')).toHaveLength(1);
    await user.click(within(dialog).getByRole('button', { name: 'Wiederherstellen' }));
    await user.click(
      within(await screen.findByRole('alertdialog')).getByRole('button', {
        name: 'Ersetzen und neu starten',
      }),
    );
    expect(
      await screen.findByRole('heading', { level: 2, name: 'Server startet neu' }),
    ).toBeVisible();
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(screen.queryByText(/Gib die Schlüssel, den E-Mail-Server/)).toBeNull();
    await waitFor(() => expect(queryClient.getMutationCache().getAll()).toHaveLength(0));
    expect(holdsPassphrase(queryClient)).toBe(false);
  });

  it('cancels the passphrase dialog without a request; the focus returns to the menu', async () => {
    const { calls } = mockApi(routes(() => [secret]));
    renderApp('/admin/backup');
    const user = userEvent.setup();
    await screen.findByRole('table', { name: 'Sicherungen: Manuell' });
    await pick(user, secret.name, 'Wiederherstellen …');
    const dialog = await screen.findByRole('dialog', { name: `„${secret.name}“ wiederherstellen` });
    await user.type(within(dialog).getByLabelText(/^Passphrase/), PASSPHRASE);
    await user.click(within(dialog).getByRole('button', { name: 'Abbrechen' }));
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    await waitFor(() =>
      expect(
        within(rowOf(secret.name)).getByRole('button', {
          name: `Weitere Aktionen für ${secret.name}`,
        }),
      ).toHaveFocus(),
    );
    expect(calls.some((call) => call.method === 'POST')).toBe(false);

    // Opened again, the field is empty.
    await pick(user, secret.name, 'Wiederherstellen …');
    expect(
      within(
        await screen.findByRole('dialog', { name: `„${secret.name}“ wiederherstellen` }),
      ).getByLabelText(/^Passphrase/),
    ).toHaveValue('');
  });

  it('restores an uploaded archive from its tab: the passphrase goes before the file (SIC-2)', async () => {
    let answer: Response | Promise<Response> = problem(422, 'backup_invalid');
    const { fetchMock } = mockApi(routes(() => []));
    // The upload is multipart (mockApi reads JSON bodies): this route keeps the parts' names and
    // texts in order. jsdom's File is not one Node's fetch can serialise, so the archive itself
    // arrives empty here; the e2e spec sends a real one.
    const uploads: { name: string; value: string }[][] = [];
    vi.stubGlobal('fetch', async (input: Request) => {
      if (input.method === 'POST' && input.url.endsWith('/api/v1/admin/backups/restore')) {
        const body = await input.text();
        const parts = [...body.matchAll(/name="([^"]+)"[^\r]*\r\n(?:[^\r]+\r\n)*\r\n([^\r]*)/g)];
        uploads.push(parts.map(([, name = '', value = '']) => ({ name, value })));
        return (await answer).clone();
      }
      return fetchMock(input);
    });
    const { queryClient } = renderApp('/admin/backup');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('tab', { name: 'Aus Datei wiederherstellen' }));
    const panel = screen.getByRole('tabpanel');
    expect(panel).toHaveTextContent('ersetzt alle Daten dieses Servers');
    const restore = within(panel).getByRole('button', { name: 'Wiederherstellen' });
    expect(restore).toBeDisabled();
    const file = new File(['archive'], 'slouchcat-backup.tar.gz.age', {
      type: 'application/octet-stream',
    });
    await user.upload(within(panel).getByLabelText('Sicherungsdatei'), file);
    await user.type(within(panel).getByLabelText(/^Passphrase/), PASSPHRASE);
    await user.click(restore);
    const confirm = await screen.findByRole('alertdialog', { name: 'Alle Daten ersetzen?' });
    // The confirmation names the file and starts on «Abbrechen».
    expect(confirm).toHaveTextContent(`durch den Stand von „${file.name}“ ersetzt`);
    await waitFor(() =>
      expect(within(confirm).getByRole('button', { name: 'Abbrechen' })).toHaveFocus(),
    );
    await user.click(within(confirm).getByRole('button', { name: 'Ersetzen und neu starten' }));
    const invalid =
      'Diese Datei ist keine gültige Sicherung von Slouchcat, oder sie ist beschädigt.';
    expect(await within(panel).findByText(invalid)).toBeVisible();
    expect(uploads[0]?.map((part) => part.name)).toEqual(['passphrase', 'file']);
    expect(uploads[0]?.[0]?.value).toBe(PASSPHRASE);

    // A tab change keeps the file and the answer (the page holds them), not the passphrase.
    await user.click(screen.getByRole('tab', { name: /^Sicherungen/ }));
    await user.click(screen.getByRole('tab', { name: 'Aus Datei wiederherstellen' }));
    const back = screen.getByRole('tabpanel');
    expect(within(back).getByText(invalid)).toBeVisible();
    expect(within(back).getByLabelText(/^Passphrase/)).toHaveValue('');
    const again = within(back).getByRole('button', { name: 'Wiederherstellen' });
    expect(again).toBeEnabled();

    // While the upload runs, the tabs wait.
    let release: (response: Response) => void = () => undefined;
    answer = new Promise<Response>((resolve) => {
      release = resolve;
    });
    await user.click(again);
    await user.click(
      within(await screen.findByRole('alertdialog')).getByRole('button', {
        name: 'Ersetzen und neu starten',
      }),
    );
    await waitFor(() =>
      expect(screen.getByRole('tab', { name: /^Sicherungen/ })).toHaveAttribute(
        'aria-disabled',
        'true',
      ),
    );
    // Without a passphrase this time: that part is left out.
    await waitFor(() => expect(uploads[1]?.map((part) => part.name)).toEqual(['file']));
    release(json(accepted, 202));
    expect(
      await screen.findByRole('heading', { level: 2, name: 'Server startet neu' }),
    ).toBeVisible();
    expect(screen.queryByRole('tabpanel')).toBeNull();
    expect(screen.getByText(/Gib die Schlüssel, den E-Mail-Server/)).toBeVisible();
    await waitFor(() => expect(queryClient.getMutationCache().getAll()).toHaveLength(0));
    expect(holdsPassphrase(queryClient)).toBe(false);
  });

  it('waits while a restore of the list runs: tabs and row menus are disabled', async () => {
    let release: (response: Response) => void = () => undefined;
    const held = new Promise<Response>((resolve) => {
      release = resolve;
    });
    mockApi(
      routes(() => [plain, auto], {
        [`POST /admin/backups/${plain.name}/restore`]: () => held,
      }),
    );
    renderApp('/admin/backup');
    const user = userEvent.setup();
    await screen.findByRole('table', { name: 'Sicherungen: Manuell' });
    await pick(user, plain.name, 'Wiederherstellen …');
    const confirm = await screen.findByRole('alertdialog', { name: 'Alle Daten ersetzen?' });
    await user.click(within(confirm).getByRole('button', { name: 'Ersetzen und neu starten' }));
    const menuOf = (name: string) =>
      within(rowOf(name)).getByRole('button', { name: `Weitere Aktionen für ${name}` });
    await waitFor(() => expect(menuOf(auto.name)).toBeDisabled());
    expect(menuOf(plain.name)).toBeDisabled();
    expect(screen.getByRole('tab', { name: 'Neue Sicherung' })).toHaveAttribute(
      'aria-disabled',
      'true',
    );
    release(json(accepted, 202));
    expect(
      await screen.findByRole('heading', { level: 2, name: 'Server startet neu' }),
    ).toBeVisible();
  });

  it('clears the last outcome when a new action starts; a new scope drops the create error', async () => {
    let list = [plain];
    mockApi(
      routes(() => list, {
        [`DELETE /admin/backups/${plain.name}`]: () => {
          list = [];
          return new Response(null, { status: 204 });
        },
        'POST /admin/backups': () => problem(500, 'internal'),
      }),
    );
    renderApp('/admin/backup');
    const user = userEvent.setup();
    await screen.findByRole('table', { name: 'Sicherungen: Manuell' });
    await pick(user, plain.name, 'Löschen …');
    await user.click(
      within(await screen.findByRole('alertdialog')).getByRole('button', {
        name: 'Sicherung löschen',
      }),
    );
    const done = `Sicherung „${plain.name}“ gelöscht.`;
    expect(await screen.findByText(done)).toBeVisible();
    // A tab change starts something new: the message goes.
    await user.click(screen.getByRole('tab', { name: 'Neue Sicherung' }));
    expect(screen.queryByText(done)).toBeNull();

    const panel = screen.getByRole('tabpanel');
    await user.click(within(panel).getByRole('button', { name: 'Sicherung erstellen' }));
    const error = await within(panel).findByRole('alert');
    expect(error).toBeVisible();
    await user.click(within(panel).getByRole('radio', { name: 'Mit Schlüsseln' }));
    expect(within(panel).queryByRole('alert')).toBeNull();
  });

  it('is for the main account only', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /auth/session': () =>
        json({ ...session, account: { ...session.account, role: 'user', isOwner: false } }),
    });
    renderApp('/admin/backup');
    expect(await screen.findByText('Nur für Administratoren')).toBeVisible();
    expect(calls.some((call) => call.path.startsWith('/admin/'))).toBe(false);
  });

  it('the list fails to load: the error offers «Erneut versuchen»', async () => {
    let fail = true;
    mockApi(
      routes(() => [plain], {
        'GET /admin/backups': () => (fail ? problem(500, 'internal') : json([plain])),
      }),
    );
    renderApp('/admin/backup');
    const user = userEvent.setup();
    const retry = await screen.findByRole('button', { name: 'Erneut versuchen' });
    fail = false;
    await user.click(retry);
    expect(await screen.findByRole('table', { name: 'Sicherungen: Manuell' })).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Erneut versuchen' })).not.toBeInTheDocument();
  });
});
