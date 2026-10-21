import type { APIRequestContext, Page } from '@playwright/test';
import { CSRF } from './catalog';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// /admin/backup (A-4) by keyboard (rule 8): a backup without secrets is created on the real server,
// listed, downloaded (the link's file answers 200 with a gzip archive) and deleted; one with
// secrets needs the passphrase and is listed as encrypted. A restore really replaces the data and
// restarts the server, so the spec goes only up to the confirmation and intercepts the request
// (202) and the health checks: the restart panel shows and the page opens the sign-in once a new
// process answers.

/**
 * The passphrase of the run: the log scan of global-teardown.ts (lane W3, A-9) checks that it
 * never reaches the server log. Same value as `BACKUP_PASSPHRASE` in env.ts there.
 */
const BACKUP_PASSPHRASE = 'e2e-backup-passphrase-7c41d9';

interface BackupData {
  name: string;
  includesSecrets: boolean;
  encrypted: boolean;
  kind: string;
}

/** Deletes the manual backups a spec created (the automatic ones of the server stay). */
async function removeBackups(api: APIRequestContext, names: readonly string[]): Promise<void> {
  for (const name of names) {
    await api.delete(`/api/v1/admin/backups/${encodeURIComponent(name)}`, { headers: CSRF });
  }
}

async function openBackup(page: Page) {
  await page.goto('/admin/backup');
  await expect(page.getByRole('heading', { level: 1, name: 'Sicherung' })).toBeVisible();
}

const backupRow = (page: Page, name: string) => page.locator(`tr[data-row-key="${name}"]`);

/** Waits for the answer of `POST /admin/backups` while `action` runs; returns the new backup. */
async function created(page: Page, action: () => Promise<void>): Promise<BackupData> {
  const response = page.waitForResponse(
    (r) => r.request().method() === 'POST' && r.url().endsWith('/api/v1/admin/backups'),
  );
  await action();
  const answer = await response;
  expect(answer.status(), await answer.text()).toBe(201);
  return (await answer.json()) as BackupData;
}

test('A-4: create a backup without secrets, download it and delete it, by keyboard', async ({
  page,
  owner,
}) => {
  const names: string[] = [];
  try {
    await openBackup(page);
    await resetFocus(page);
    const create = page.getByRole('button', { name: 'Sicherung erstellen' });
    await tabTo(page, create, 40);
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', { name: 'Sicherung erstellen' });
    await expect(dialog).toBeVisible();
    // The recommended choice is preset and has the focus.
    const without = dialog.getByRole('radio', { name: 'Ohne Schlüssel (empfohlen)' });
    await expect(without).toBeChecked();
    await expectFocusOn(page, without);
    await expect(dialog).toContainText('Nach einer Wiederherstellung gibst du sie neu ein.');
    const submit = dialog.getByRole('button', { name: 'Sicherung erstellen' });
    await tabTo(page, submit, 5);
    const backup = await created(page, () => page.keyboard.press('Enter'));
    names.push(backup.name);
    expect(backup).toMatchObject({ includesSecrets: false, encrypted: false, kind: 'manual' });
    expect(backup.name).toMatch(/^slouchcat-backup-.+\.tar\.gz$/);

    await expect(dialog).toBeHidden();
    await expectFocusOn(page, create);
    await expect(page.getByText(`Sicherung „${backup.name}“ erstellt.`)).toBeVisible();
    const row = backupRow(page, backup.name);
    await expect(row.getByRole('rowheader')).toHaveText(backup.name);
    await expect(row).toContainText('Manuell');
    await expect(row.getByRole('cell').nth(3)).toHaveText('Nein');
    await expect(row.getByRole('cell').nth(4)).toHaveText('Nein');

    // The download link: its address answers with the archive, and Enter downloads the file.
    const link = row.getByRole('link', { name: `${backup.name} herunterladen` });
    const href = `/api/v1/admin/backups/${backup.name}/download`;
    await expect(link).toHaveAttribute('href', href);
    const file = await page.request.get(href);
    expect(file.status()).toBe(200);
    expect(file.headers()['content-disposition']).toContain('attachment');
    const bytes = await file.body();
    expect([...bytes.subarray(0, 2)]).toEqual([0x1f, 0x8b]);
    await link.focus();
    await expectVisibleFocus(page);
    const download = page.waitForEvent('download');
    await page.keyboard.press('Enter');
    expect((await download).suggestedFilename()).toBe(backup.name);
    await expect(page).toHaveURL(/\/admin\/backup$/);

    // Delete: Enter opens the confirmation with the focus on its button, Escape returns.
    const remove = row.getByRole('button', { name: `${backup.name} löschen` });
    await remove.focus();
    await page.keyboard.press('Enter');
    const confirm = page.getByRole('alertdialog', {
      name: `Sicherung „${backup.name}“ löschen?`,
    });
    await expectFocusOn(page, confirm.getByRole('button', { name: 'Sicherung löschen' }));
    await page.keyboard.press('Escape');
    await expect(confirm).toBeHidden();
    await expectFocusOn(page, remove);
    await page.keyboard.press('Enter');
    await expectFocusOn(page, confirm.getByRole('button', { name: 'Sicherung löschen' }));
    await page.keyboard.press('Enter');
    await expect(page.getByText(`Sicherung „${backup.name}“ gelöscht.`)).toBeVisible();
    await expect(row).toHaveCount(0);
    expect((await page.request.get(href)).status()).toBe(404);
    await expectNoFocusTrap(page);
  } finally {
    await removeBackups(owner, names);
  }
});

test('A-4: a backup with secrets needs a passphrase of 12 characters and is encrypted', async ({
  page,
  owner,
}) => {
  const names: string[] = [];
  try {
    await openBackup(page);
    await resetFocus(page);
    await tabTo(page, page.getByRole('button', { name: 'Sicherung erstellen' }), 40);
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', { name: 'Sicherung erstellen' });
    const without = dialog.getByRole('radio', { name: 'Ohne Schlüssel (empfohlen)' });
    await expectFocusOn(page, without);
    await page.keyboard.press('ArrowRight');
    await expect(dialog.getByRole('radio', { name: 'Mit Schlüsseln' })).toBeChecked();
    await expect(dialog).toContainText(
      'mit einer Passphrase verschlüsselt (mindestens 12 Zeichen)',
    );
    await page.keyboard.press('Tab');
    await expectFocusOn(page, dialog.getByLabel('Passphrase', { exact: true }));
    await page.keyboard.type('zu-kurz');
    await page.keyboard.press('Tab');
    await page.keyboard.type('zu-kurz');
    await page.keyboard.press('Enter');
    await expect(dialog.getByText('Mindestens 12 Zeichen.', { exact: true })).toBeVisible();

    await dialog.getByLabel('Passphrase', { exact: true }).fill(BACKUP_PASSPHRASE);
    await dialog.getByLabel('Passphrase wiederholen').fill(BACKUP_PASSPHRASE);
    const backup = await created(page, () => page.keyboard.press('Enter'));
    names.push(backup.name);
    expect(backup).toMatchObject({ includesSecrets: true, encrypted: true });
    expect(backup.name).toMatch(/\.tar\.gz\.age$/);
    const row = backupRow(page, backup.name);
    await expect(row.getByRole('cell').nth(3)).toHaveText('Ja');
    await expect(row.getByRole('cell').nth(4)).toHaveText('Ja');

    // Its restore asks for the passphrase first; the spec stops before the confirmation.
    const restore = row.getByRole('button', { name: `${backup.name} wiederherstellen` });
    await restore.focus();
    await page.keyboard.press('Enter');
    const restoreDialog = page.getByRole('dialog', { name: `„${backup.name}“ wiederherstellen` });
    await expect(restoreDialog).toContainText('ersetzt alle Daten dieses Servers');
    await expectFocusOn(page, restoreDialog.getByLabel(/^Passphrase/));
    await expect(restoreDialog.getByRole('button', { name: 'Wiederherstellen' })).toBeDisabled();
    await page.keyboard.press('Escape');
    await expect(restoreDialog).toBeHidden();
    await expectFocusOn(page, restore);
  } finally {
    await removeBackups(owner, names);
  }
});

test.describe('restore', () => {
  // The service worker sends every /api request itself, past page.route: without it the routes
  // below see the restore and the health checks.
  test.use({ serviceWorkers: 'block' });

  test('A-4: restore from an uploaded file up to the restart panel (request intercepted)', async ({
    page,
  }) => {
    // The restore and the health checks never reach the server: it must not restart in the middle
    // of the suite. The page sees an accepted restore and an old process that is still up.
    let restoreBody = '';
    await page.route('**/api/v1/admin/backups/restore', async (route) => {
      restoreBody = route.request().postDataBuffer()?.toString('latin1') ?? '';
      await route.fulfill({
        status: 202,
        json: {
          restartAt: new Date(Date.now() + 2000).toISOString(),
          version: '0.9.0',
          includesSecrets: false,
        },
      });
    });
    let uptime = 3600;
    let polls = 0;
    await page.route('**/api/v1/healthz', async (route) => {
      polls += 1;
      await route.fulfill({ json: { status: 'ok', version: '0.9.0', uptimeSeconds: uptime } });
    });

    await openBackup(page);
    await resetFocus(page);
    const upload = page.getByRole('button', { name: 'Aus Datei wiederherstellen' });
    await tabTo(page, upload, 40);
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', { name: 'Sicherung aus einer Datei wiederherstellen' });
    await expect(dialog).toContainText('ersetzt alle Daten dieses Servers');
    const file = dialog.getByLabel('Sicherungsdatei');
    await expectFocusOn(page, file);
    const restore = dialog.getByRole('button', { name: 'Wiederherstellen' });
    await expect(restore).toBeDisabled();
    await file.setInputFiles({
      name: 'slouchcat-backup-e2e.tar.gz.age',
      mimeType: 'application/octet-stream',
      buffer: Buffer.from('age-encryption.org/v1 e2e archive'),
    });
    await page.keyboard.press('Tab');
    await expectFocusOn(page, dialog.getByLabel(/^Passphrase/));
    await page.keyboard.type(BACKUP_PASSPHRASE);
    await tabTo(page, restore, 4);
    await page.keyboard.press('Enter');
    const confirm = page.getByRole('alertdialog', { name: 'Alle Daten ersetzen?' });
    await expect(confirm).toContainText('startet der Server neu');
    const replace = confirm.getByRole('button', { name: 'Ersetzen und neu starten' });
    await expectFocusOn(page, replace);
    await page.keyboard.press('Enter');

    const heading = page.getByRole('heading', { level: 2, name: 'Server startet neu' });
    await expect(heading).toBeFocused();
    await expect(
      page.getByRole('status').filter({ hasText: 'Warte auf den Server' }),
    ).toBeVisible();
    await expect(page.getByText(/Gib die Schlüssel, den E-Mail-Server/)).toBeVisible();
    // Multipart with the passphrase before the archive (RestoreUpload).
    const passphraseAt = restoreBody.indexOf('name="passphrase"');
    const fileAt = restoreBody.indexOf('name="file"; filename="slouchcat-backup-e2e.tar.gz.age"');
    expect(passphraseAt).toBeGreaterThan(-1);
    expect(fileAt).toBeGreaterThan(passphraseAt);
    expect(restoreBody).toContain('age-encryption.org/v1 e2e archive');

    // Every 2 s a health check; the old process (long uptime) does not end the wait.
    await expect.poll(() => polls, { timeout: 7000 }).toBeGreaterThanOrEqual(2);
    await expect(heading).toBeVisible();
    // A new process answers: the page loads the sign-in (which goes on to the start page while the
    // intercepted restore left the session as it was).
    const login = page.waitForURL(/\/login$/, { waitUntil: 'commit' });
    uptime = 0;
    await login;
  });
});
