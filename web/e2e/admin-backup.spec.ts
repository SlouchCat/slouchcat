import type { APIRequestContext, Page } from '@playwright/test';
import { CSRF } from './catalog';
import { BACKUP_PASSPHRASE } from './env';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// /admin/backup (A-4; concept 12, variant C) by keyboard (rule 8): the tabs «Sicherungen», «Neue
// Sicherung» and «Aus Datei wiederherstellen» with the arrow keys; a backup without secrets is
// created on the real server, listed under «Manuell», downloaded (the link's file answers 200 with
// a gzip archive) and deleted from the row's menu; one with secrets needs the passphrase and is
// listed as encrypted. A restore really replaces the data and restarts the server, so the spec
// goes only up to the confirmation and intercepts the request (202) and the health checks: the
// restart panel shows and the page opens the sign-in once a new process answers.

/** Tab presses from the top of the page: the sidebar, the header, the tabs, the sections. */
const TAB_MAX = 80;

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

/** From the top of the page to the selected tab, then with the arrows to the tab `name`. */
async function selectTab(page: Page, name: string, arrows: number) {
  await resetFocus(page);
  await tabTo(page, page.getByRole('tab', { selected: true }), TAB_MAX);
  for (let press = 0; press < arrows; press += 1) await page.keyboard.press('ArrowRight');
  const tab = page.getByRole('tab', { name });
  await expect(tab).toHaveAttribute('aria-selected', 'true');
  await expectFocusOn(page, tab);
}

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
    await selectTab(page, 'Neue Sicherung', 1);
    await expect(page).toHaveURL(/\/admin\/backup\?tab=create$/);
    // ← goes back to «Sicherungen» (the plain address), → again to «Neue Sicherung».
    await page.keyboard.press('ArrowLeft');
    await expect(page.getByRole('tab', { name: /^Sicherungen/ })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    await expect(page).toHaveURL(/\/admin\/backup$/);
    await page.keyboard.press('ArrowRight');
    await expect(page.getByRole('tab', { name: 'Neue Sicherung' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    await expect(page).toHaveURL(/\/admin\/backup\?tab=create$/);
    // The recommended choice is preset and the first stop in the tab.
    const panel = page.getByRole('tabpanel');
    const without = panel.getByRole('radio', { name: 'Ohne Schlüssel (empfohlen)' });
    await expect(without).toBeChecked();
    await page.keyboard.press('Tab');
    await expectFocusOn(page, without);
    await expect(panel).toContainText('Nach einer Wiederherstellung gibst du sie neu ein.');
    const submit = panel.getByRole('button', { name: 'Sicherung erstellen' });
    await tabTo(page, submit, 5);
    const backup = await created(page, () => page.keyboard.press('Enter'));
    names.push(backup.name);
    expect(backup).toMatchObject({ includesSecrets: false, encrypted: false, kind: 'manual' });
    expect(backup.name).toMatch(/^slouchcat-backup-.+\.tar\.gz$/);

    // «Sicherungen» shows again with the new row under «Manuell»; the h1 has the focus.
    await expect(page.getByText(`Sicherung „${backup.name}“ erstellt.`)).toBeVisible();
    await expect(page).toHaveURL(/\/admin\/backup$/);
    const title = page.getByRole('heading', { level: 1, name: 'Sicherung' });
    await expect(title).toBeFocused();
    const manual = page.getByRole('table', { name: 'Sicherungen: Manuell' });
    const row = manual.locator(`tr[data-row-key="${backup.name}"]`);
    await expect(row.getByRole('rowheader')).toHaveText(`${backup.name} neu`);
    await expect(row.getByRole('cell').nth(2)).toHaveText('Nein');
    await expect(row.getByRole('cell').nth(3)).toHaveText('Nein');

    // The kind folds away by its heading with Enter and comes back.
    const group = page.getByRole('button', { name: /^Manuell \d+ Sicherung/ });
    await resetFocus(page);
    await tabTo(page, group, TAB_MAX);
    await page.keyboard.press('Enter');
    await expect(group).toHaveAttribute('aria-expanded', 'false');
    await expect(manual).toBeHidden();
    await page.keyboard.press('Enter');
    await expect(group).toHaveAttribute('aria-expanded', 'true');

    // The download link: its address answers with the archive, and Enter downloads the file.
    const link = row.getByRole('link', { name: `${backup.name} herunterladen` });
    const href = `/api/v1/admin/backups/${backup.name}/download`;
    await expect(link).toHaveAttribute('href', href);
    const file = await page.request.get(href);
    expect(file.status()).toBe(200);
    expect(file.headers()['content-disposition']).toContain('attachment');
    const bytes = await file.body();
    expect([...bytes.subarray(0, 2)]).toEqual([0x1f, 0x8b]);
    await tabTo(page, link, 5);
    const download = page.waitForEvent('download');
    await page.keyboard.press('Enter');
    expect((await download).suggestedFilename()).toBe(backup.name);
    await expect(page).toHaveURL(/\/admin\/backup$/);

    // «Weitere Aktionen»: Enter opens it on «Wiederherstellen …», ↓ goes to «Löschen …», both
    // under «Gefahr»; Escape closes onto its button.
    const more = row.getByRole('button', { name: `Weitere Aktionen für ${backup.name}` });
    await page.keyboard.press('Tab');
    await expectFocusOn(page, more);
    const menu = page.getByRole('menu');
    const restore = menu.getByRole('menuitem', { name: 'Wiederherstellen …' });
    const remove = menu.getByRole('menuitem', { name: 'Löschen …' });
    await page.keyboard.press('Enter');
    await expect(restore).toBeFocused();
    await expectVisibleFocus(page);
    await page.keyboard.press('ArrowDown');
    await expect(remove).toBeFocused();
    const danger = menu.getByRole('group', { name: 'Gefahr' });
    await expect(danger.getByRole('menuitem')).toHaveText(['Wiederherstellen …', 'Löschen …']);
    await expect(menu.getByRole('menuitem')).toHaveCount(2);
    await page.keyboard.press('Escape');
    await expect(menu).toHaveCount(0);
    await expectFocusOn(page, more);

    // Delete: the confirmation has the focus on its button, Escape returns to the menu button.
    const confirm = page.getByRole('alertdialog', {
      name: `Sicherung „${backup.name}“ löschen?`,
    });
    const confirmButton = confirm.getByRole('button', { name: 'Sicherung löschen' });
    await page.keyboard.press('Enter');
    await page.keyboard.press('ArrowDown');
    await expect(remove).toBeFocused();
    await page.keyboard.press('Enter');
    await expectFocusOn(page, confirmButton);
    await page.keyboard.press('Escape');
    await expect(confirm).toBeHidden();
    await expectFocusOn(page, more);
    await page.keyboard.press('Enter');
    await page.keyboard.press('ArrowDown');
    await expect(remove).toBeFocused();
    await page.keyboard.press('Enter');
    await expectFocusOn(page, confirmButton);
    await page.keyboard.press('Enter');
    await expect(confirm).toBeHidden();
    await expect(page.getByText(`Sicherung „${backup.name}“ gelöscht.`)).toBeVisible();
    await expect(row).toHaveCount(0);
    await expect(title).toBeFocused();
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
    await selectTab(page, 'Neue Sicherung', 1);
    const panel = page.getByRole('tabpanel');
    const without = panel.getByRole('radio', { name: 'Ohne Schlüssel (empfohlen)' });
    await page.keyboard.press('Tab');
    await expectFocusOn(page, without);
    await page.keyboard.press('ArrowRight');
    await expect(panel.getByRole('radio', { name: 'Mit Schlüsseln' })).toBeChecked();
    await expect(panel).toContainText('mit einer Passphrase verschlüsselt (mindestens 12 Zeichen)');
    await page.keyboard.press('Tab');
    await expectFocusOn(page, panel.getByLabel('Passphrase', { exact: true }));
    await page.keyboard.type('zu-kurz');
    await page.keyboard.press('Tab');
    await expectFocusOn(page, panel.getByLabel('Passphrase wiederholen'));
    await page.keyboard.type('zu-kurz');
    await page.keyboard.press('Enter');
    await expect(panel.getByText('Mindestens 12 Zeichen.', { exact: true })).toBeVisible();

    await panel.getByLabel('Passphrase', { exact: true }).fill(BACKUP_PASSPHRASE);
    await panel.getByLabel('Passphrase wiederholen').fill(BACKUP_PASSPHRASE);
    const backup = await created(page, () => page.keyboard.press('Enter'));
    names.push(backup.name);
    expect(backup).toMatchObject({ includesSecrets: true, encrypted: true });
    expect(backup.name).toMatch(/\.tar\.gz\.age$/);
    const row = backupRow(page, backup.name);
    await expect(row.getByRole('cell').nth(2)).toHaveText('Ja');
    await expect(row.getByRole('cell').nth(3)).toHaveText('Ja');

    // Its restore asks for the passphrase in a dialog first; the spec stops before the
    // confirmation.
    const more = row.getByRole('button', { name: `Weitere Aktionen für ${backup.name}` });
    await resetFocus(page);
    await tabTo(page, more, TAB_MAX);
    await page.keyboard.press('Enter');
    await expect(page.getByRole('menuitem', { name: 'Wiederherstellen …' })).toBeFocused();
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', { name: `„${backup.name}“ wiederherstellen` });
    await expect(dialog).toContainText('ersetzt alle Daten dieses Servers');
    await expectFocusOn(page, dialog.getByLabel(/^Passphrase/));
    await expect(dialog.getByRole('button', { name: 'Wiederherstellen' })).toBeDisabled();
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
    await expectFocusOn(page, more);
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
    // of the suite. The page sees an accepted restore and an old process that is still up. The
    // clock is the page's own, so the spec can jump over the 90 s the panel waits.
    await page.clock.install();
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
    await selectTab(page, 'Aus Datei wiederherstellen', 2);
    await expect(page).toHaveURL(/\/admin\/backup\?tab=upload$/);
    const panel = page.getByRole('tabpanel');
    await expect(panel).toContainText('ersetzt alle Daten dieses Servers');
    const file = panel.getByLabel('Sicherungsdatei');
    await page.keyboard.press('Tab');
    await expectFocusOn(page, file);
    const restore = panel.getByRole('button', { name: 'Wiederherstellen' });
    await expect(restore).toBeDisabled();
    await file.setInputFiles({
      name: 'slouchcat-backup-e2e.tar.gz.age',
      mimeType: 'application/octet-stream',
      buffer: Buffer.from('age-encryption.org/v1 e2e archive'),
    });
    // The chosen file is the page's: ← and → over the tabs keep it in the field.
    const uploadTab = page.getByRole('tab', { name: 'Aus Datei wiederherstellen' });
    await page.keyboard.press('Shift+Tab');
    await expectFocusOn(page, uploadTab);
    await page.keyboard.press('ArrowLeft');
    await expect(page.getByRole('tab', { name: 'Neue Sicherung' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    await page.keyboard.press('ArrowRight');
    await expect(uploadTab).toHaveAttribute('aria-selected', 'true');
    await page.keyboard.press('Tab');
    await expectFocusOn(page, file);
    expect(
      await file.evaluate((input) => (input as HTMLInputElement).files?.[0]?.name ?? null),
    ).toBe('slouchcat-backup-e2e.tar.gz.age');
    await expect(restore).toBeEnabled();
    await page.keyboard.press('Tab');
    await expectFocusOn(page, panel.getByLabel(/^Passphrase/));
    await page.keyboard.type(BACKUP_PASSPHRASE);
    await tabTo(page, restore, 4);
    await page.keyboard.press('Enter');
    const confirm = page.getByRole('alertdialog', { name: 'Alle Daten ersetzen?' });
    await expect(confirm).toContainText('startet der Server neu');
    await expect(confirm).toContainText('„slouchcat-backup-e2e.tar.gz.age“');
    // The focus starts on «Abbrechen»: a second Enter by habit replaces nothing. Tab reaches
    // «Ersetzen und neu starten».
    await expectFocusOn(page, confirm.getByRole('button', { name: 'Abbrechen' }));
    const replace = confirm.getByRole('button', { name: 'Ersetzen und neu starten' });
    await page.keyboard.press('Tab');
    await expectFocusOn(page, replace);
    await page.keyboard.press('Enter');
    await expect(confirm).toBeHidden();

    const heading = page.getByRole('heading', { level: 2, name: 'Server startet neu' });
    await expect(heading).toBeFocused();
    await expect(
      page.getByRole('status').filter({ hasText: 'Warte auf den Server' }),
    ).toBeVisible();
    await expect(page.getByText(/Gib die Schlüssel, den E-Mail-Server/)).toBeVisible();
    await expect(page.getByRole('tablist')).toHaveCount(0);
    // Multipart with the passphrase before the archive (RestoreUpload).
    const passphraseAt = restoreBody.indexOf('name="passphrase"');
    const fileAt = restoreBody.indexOf('name="file"; filename="slouchcat-backup-e2e.tar.gz.age"');
    expect(passphraseAt).toBeGreaterThan(-1);
    expect(fileAt).toBeGreaterThan(passphraseAt);
    expect(restoreBody).toContain('age-encryption.org/v1 e2e archive');

    // Every 2 s a health check; the old process (long uptime) does not end the wait.
    await expect.poll(() => polls, { timeout: 7000 }).toBeGreaterThanOrEqual(2);
    await expect(heading).toBeVisible();
    // 90 s without a new process: the panel stops asking and says so, with «Erneut prüfen».
    await page.clock.fastForward(90_000);
    await expect(page.getByText('Der Server kommt nicht zurück')).toBeVisible();
    await expect(
      page.getByRole('status').filter({ hasText: 'Seit 90 Sekunden antwortet kein neuer Server.' }),
    ).toBeVisible();
    const stopped = polls;
    await page.clock.runFor(6000);
    expect(polls).toBe(stopped);
    const again = page.getByRole('button', { name: 'Erneut prüfen' });
    await tabTo(page, again, 10);
    await expectVisibleFocus(page);
    // A new process answers: «Erneut prüfen» asks again and the page loads the sign-in (which goes
    // on to the start page while the intercepted restore left the session as it was).
    const login = page.waitForURL(/\/login$/, { waitUntil: 'commit' });
    uptime = 0;
    await page.keyboard.press('Enter');
    await login;
  });
});
