import type { Backup, RestoreResult } from '@slouchcat/api-client';
import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, session, signedIn, type Routes } from '@/test/api';
import { reloadToLogin } from './restart';

// A-4: backups with and without secrets, download, delete, and the restore from the list or from
// an uploaded file, which ends in the restart panel. The server checks archives and passphrases.

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

const health = (uptimeSeconds: number) => json({ status: 'ok', version: '0.9.0', uptimeSeconds });

const routes = (list: () => Backup[], extra: Routes = {}): Routes => ({
  ...signedIn,
  'GET /admin/backups': () => json(list()),
  'GET /healthz': () => health(3600),
  ...extra,
});

const rowOf = (name: string) => screen.getByRole('rowheader', { name }).closest('tr')!;

describe('admin backup (A-4)', () => {
  beforeEach(() => vi.mocked(reloadToLogin).mockClear());
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('lists the backups with kind, secrets and a download link each', async () => {
    mockApi(routes(() => [plain, secret, auto]));
    renderApp('/admin/backup');
    expect(await screen.findByRole('heading', { level: 1, name: 'Sicherung' })).toBeVisible();
    const table = await screen.findByRole('table', { name: 'Sicherungen' });
    expect(
      within(table)
        .getAllByRole('columnheader')
        .map((cell) => cell.textContent),
    ).toEqual(['Datei', 'Erstellt', 'Grösse', 'Art', 'Schlüssel', 'Verschlüsselt', 'Aktionen']);
    const cells = (name: string) =>
      within(rowOf(name))
        .getAllByRole('cell')
        .map((cell) => cell.textContent);
    expect(cells(plain.name).slice(0, 5)).toEqual([
      expect.stringMatching(/2026/) as string,
      expect.stringMatching(/^4\sMB$/) as string,
      'Manuell',
      'Nein',
      'Nein',
    ]);
    expect(cells(secret.name).slice(2, 5)).toEqual(['Manuell', 'Ja', 'Ja']);
    expect(cells(auto.name)[2]).toBe('Automatisch');
    const download = within(rowOf(plain.name)).getByRole('link', {
      name: `${plain.name} herunterladen`,
    });
    expect(download).toHaveAttribute('href', `/api/v1/admin/backups/${plain.name}/download`);
    expect(download).toHaveAttribute('download', plain.name);
  });

  it('creates a backup without secrets by default', async () => {
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
    renderApp('/admin/backup');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Sicherung erstellen' }));
    const dialog = await screen.findByRole('dialog', { name: 'Sicherung erstellen' });
    expect(within(dialog).getByRole('radio', { name: 'Ohne Schlüssel (empfohlen)' })).toBeChecked();
    expect(dialog).toHaveTextContent('Nach einer Wiederherstellung gibst du sie neu ein.');
    expect(within(dialog).queryByLabelText(/^Passphrase/)).toBeNull();
    await user.click(within(dialog).getByRole('button', { name: 'Sicherung erstellen' }));

    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    expect(calls.find((call) => call.method === 'POST')?.body).toEqual({ includeSecrets: false });
    expect(await screen.findByText(`Sicherung „${created.name}“ erstellt.`)).toBeVisible();
    expect(await screen.findByRole('rowheader', { name: created.name })).toBeVisible();
  });

  it('creates a backup with secrets only with a passphrase of at least 12 characters', async () => {
    const { calls } = mockApi(
      routes(() => [], { 'POST /admin/backups': () => json({ ...secret }, 201) }),
    );
    renderApp('/admin/backup');
    const user = userEvent.setup();
    expect(await screen.findByText('Noch keine Sicherungen.')).toBeVisible();
    await user.click(screen.getByRole('button', { name: 'Sicherung erstellen' }));
    const dialog = await screen.findByRole('dialog', { name: 'Sicherung erstellen' });
    await user.click(within(dialog).getByRole('radio', { name: 'Mit Schlüsseln' }));
    expect(dialog).toHaveTextContent('mit einer Passphrase verschlüsselt (mindestens 12 Zeichen)');
    const passphrase = within(dialog).getByLabelText('Passphrase');
    await user.type(passphrase, 'zu-kurz');
    await user.type(within(dialog).getByLabelText('Passphrase wiederholen'), 'zu-kurz');
    await user.click(within(dialog).getByRole('button', { name: 'Sicherung erstellen' }));
    expect(await within(dialog).findByText('Mindestens 12 Zeichen.')).toBeVisible();

    await user.clear(passphrase);
    await user.type(passphrase, 'lange-passphrase-42');
    await user.click(within(dialog).getByRole('button', { name: 'Sicherung erstellen' }));
    expect(await within(dialog).findByText('Die Passphrasen stimmen nicht überein.')).toBeVisible();
    expect(calls.some((call) => call.method === 'POST')).toBe(false);

    const repeat = within(dialog).getByLabelText('Passphrase wiederholen');
    await user.clear(repeat);
    await user.type(repeat, 'lange-passphrase-42');
    await user.click(within(dialog).getByRole('button', { name: 'Sicherung erstellen' }));
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    expect(calls.find((call) => call.method === 'POST')?.body).toEqual({
      includeSecrets: true,
      passphrase: 'lange-passphrase-42',
    });
  });

  it('deletes a backup after the confirmation; the focus goes to the heading', async () => {
    let list = [plain, secret];
    const { calls } = mockApi(
      routes(() => list, {
        [`DELETE /admin/backups/${plain.name}`]: () => {
          list = [secret];
          return new Response(null, { status: 204 });
        },
      }),
    );
    renderApp('/admin/backup');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: `${plain.name} löschen` }));
    const dialog = await screen.findByRole('alertdialog', {
      name: `Sicherung „${plain.name}“ löschen?`,
    });
    await user.click(within(dialog).getByRole('button', { name: 'Sicherung löschen' }));
    await waitFor(() => expect(calls.some((call) => call.method === 'DELETE')).toBe(true));
    expect(await screen.findByText(`Sicherung „${plain.name}“ gelöscht.`)).toBeVisible();
    await waitFor(() => expect(screen.queryByRole('rowheader', { name: plain.name })).toBeNull());
    await waitFor(() =>
      expect(
        screen.getByRole('heading', { level: 1, name: 'Sicherung' }).parentElement,
      ).toHaveFocus(),
    );
  });

  it('restores a backup of the list after the warning; the restart panel waits for the new server', async () => {
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
    await user.click(await screen.findByRole('button', { name: `${plain.name} wiederherstellen` }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Alle Daten ersetzen?' });
    expect(dialog).toHaveTextContent(`durch den Stand von „${plain.name}“ ersetzt`);
    expect(dialog).toHaveTextContent('startet der Server neu');
    await user.click(within(dialog).getByRole('button', { name: 'Ersetzen und neu starten' }));

    const heading = await screen.findByRole('heading', { level: 2, name: 'Server startet neu' });
    await waitFor(() => expect(heading).toHaveFocus());
    expect(
      calls.find((call) => call.path === `/admin/backups/${plain.name}/restore`)?.body,
    ).toEqual({});
    expect(screen.getByText(/Gib die Schlüssel, den E-Mail-Server/)).toBeVisible();
    expect(screen.getByRole('status')).toHaveTextContent('Warte auf den Server …');
    expect(screen.queryByRole('table')).toBeNull();

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
    expect(screen.getByRole('status')).toHaveTextContent('Der Server ist zurück.');
  });

  it('asks for the passphrase of an encrypted backup and shows a wrong one at the field', async () => {
    let answer = problem(422, 'passphrase_wrong');
    const { calls } = mockApi(
      routes(() => [secret], {
        [`POST /admin/backups/${secret.name}/restore`]: () => answer,
      }),
    );
    renderApp('/admin/backup');
    const user = userEvent.setup();
    await user.click(
      await screen.findByRole('button', { name: `${secret.name} wiederherstellen` }),
    );
    const dialog = await screen.findByRole('dialog', { name: `„${secret.name}“ wiederherstellen` });
    expect(dialog).toHaveTextContent('ersetzt alle Daten dieses Servers');
    const passphrase = within(dialog).getByLabelText(/^Passphrase/);
    expect(passphrase).toHaveFocus();
    const restore = within(dialog).getByRole('button', { name: 'Wiederherstellen' });
    expect(restore).toBeDisabled();
    await user.type(passphrase, 'falsche-passphrase');
    await user.click(restore);
    const confirm = await screen.findByRole('alertdialog', { name: 'Alle Daten ersetzen?' });
    await user.click(within(confirm).getByRole('button', { name: 'Ersetzen und neu starten' }));
    expect(await within(dialog).findByText('Die Passphrase stimmt nicht.')).toBeVisible();
    expect(calls.find((call) => call.method === 'POST')?.body).toEqual({
      passphrase: 'falsche-passphrase',
    });

    answer = json({ ...accepted, includesSecrets: true }, 202);
    await user.clear(passphrase);
    await user.type(passphrase, 'lange-passphrase-42');
    expect(within(dialog).queryByText('Die Passphrase stimmt nicht.')).toBeNull();
    await user.click(restore);
    await user.click(
      within(await screen.findByRole('alertdialog')).getByRole('button', {
        name: 'Ersetzen und neu starten',
      }),
    );
    expect(
      await screen.findByRole('heading', { level: 2, name: 'Server startet neu' }),
    ).toBeVisible();
    expect(screen.queryByText(/Gib die Schlüssel, den E-Mail-Server/)).toBeNull();
  });

  it('restores an uploaded archive: the passphrase goes before the file', async () => {
    let answer = problem(422, 'backup_invalid');
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
        return answer.clone();
      }
      return fetchMock(input);
    });
    renderApp('/admin/backup');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Aus Datei wiederherstellen' }));
    const dialog = await screen.findByRole('dialog', {
      name: 'Sicherung aus einer Datei wiederherstellen',
    });
    const restore = within(dialog).getByRole('button', { name: 'Wiederherstellen' });
    expect(restore).toBeDisabled();
    const file = new File(['archive'], 'slouchcat-backup.tar.gz.age', {
      type: 'application/octet-stream',
    });
    await user.upload(within(dialog).getByLabelText('Sicherungsdatei'), file);
    await user.type(within(dialog).getByLabelText(/^Passphrase/), 'lange-passphrase-42');
    await user.click(restore);
    await user.click(
      within(await screen.findByRole('alertdialog')).getByRole('button', {
        name: 'Ersetzen und neu starten',
      }),
    );
    expect(
      await within(dialog).findByText(
        'Diese Datei ist keine gültige Sicherung von Slouchcat, oder sie ist beschädigt.',
      ),
    ).toBeVisible();
    expect(uploads[0]?.map((part) => part.name)).toEqual(['passphrase', 'file']);
    expect(uploads[0]?.[0]?.value).toBe('lange-passphrase-42');

    answer = json(accepted, 202);
    await user.click(restore);
    await user.click(
      within(await screen.findByRole('alertdialog')).getByRole('button', {
        name: 'Ersetzen und neu starten',
      }),
    );
    expect(
      await screen.findByRole('heading', { level: 2, name: 'Server startet neu' }),
    ).toBeVisible();
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('is for the main account only', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /auth/session': () =>
        json({ ...session, account: { ...session.account, role: 'guest' } }),
    });
    renderApp('/admin/backup');
    expect(await screen.findByText('Nur für das Hauptkonto')).toBeVisible();
    expect(calls.some((call) => call.path.startsWith('/admin/'))).toBe(false);
  });
});
