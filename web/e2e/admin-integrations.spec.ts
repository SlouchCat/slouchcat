import type { APIRequestContext, Page } from '@playwright/test';
import { CSRF } from './catalog';
import { YOUTUBE_KEY } from './env';
import { expect, test } from './fixtures';
import { setUpdateManifest } from './mocks';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// Integrations (A-2) against the real server: provider keys with their source (the e2e server
// sets the Trakt client id in its environment, no TMDB key), a YouTube key saved and deleted by
// keyboard, the public address and the update check. Every test leaves the settings as it found
// them. The test mail with the SMTP mock is covered by notification-channels.spec.ts.

interface SettingsData {
  publicUrl?: string;
  publicUrlSource: string;
  updateCheckEnabled: boolean;
  keys: Record<string, { source: string; hint?: string }>;
}

async function settingsOf(api: APIRequestContext): Promise<SettingsData> {
  const response = await api.get('/api/v1/admin/settings');
  expect(response.status(), await response.text()).toBe(200);
  return (await response.json()) as SettingsData;
}

async function putSettings(api: APIRequestContext, data: object): Promise<void> {
  const response = await api.put('/api/v1/admin/settings', { headers: CSRF, data });
  expect(response.status(), await response.text()).toBe(200);
}

const region = (page: Page, name: string) => page.getByRole('region', { name, exact: true });

async function openIntegrations(page: Page) {
  await page.goto('/admin/integrations');
  await expect(page.getByRole('heading', { level: 1, name: 'Integrationen' })).toBeVisible();
  await expect(region(page, 'Schlüssel der Anbieter')).toBeVisible();
}

test('keys show their source; the Trakt id from the environment is read-only', async ({ page }) => {
  await openIntegrations(page);
  await expect(
    page
      .getByRole('navigation', { name: 'Verwaltung' })
      .getByRole('link', { name: 'Integrationen' }),
  ).toHaveAttribute('aria-current', 'page');
  const keys = region(page, 'Schlüssel der Anbieter');
  const trakt = keys.getByLabel('Trakt-Client-ID');
  await expect(trakt).toBeDisabled();
  await expect(keys.getByText('In deploy/.env gesetzt', { exact: true })).toBeVisible();
  await expect(keys.getByRole('button', { name: 'Trakt-Client-ID speichern' })).toHaveCount(0);
  // The e2e server runs without a TMDB key.
  await expect(keys.getByLabel('TMDB-Lesezugriffs-Token')).toBeEnabled();
  await expect(keys.getByRole('button', { name: 'TMDB-Token löschen' })).toHaveCount(0);
  await expect(keys.getByRole('link', { name: 'Stand der Trakt-Listen' })).toHaveAttribute(
    'href',
    '/admin/spotlight',
  );
});

test('a YouTube key is saved write-only and deleted again, all by keyboard', async ({
  page,
  owner,
}) => {
  const key = YOUTUBE_KEY;
  try {
    await openIntegrations(page);
    const keys = region(page, 'Schlüssel der Anbieter');
    const field = keys.getByLabel('YouTube-API-Schlüssel');
    await resetFocus(page);
    await tabTo(page, field, 60);
    await page.keyboard.type(key);
    await page.keyboard.press('Tab');
    const save = keys.getByRole('button', { name: 'YouTube-Schlüssel speichern' });
    await expectFocusOn(page, save);
    const stored = page.waitForResponse(
      (response) =>
        response.request().method() === 'PUT' && response.url().endsWith('/admin/settings'),
    );
    await page.keyboard.press('Enter');
    expect((await stored).status()).toBe(200);
    await expect(keys.getByText('YouTube-Schlüssel gespeichert.')).toBeVisible();
    // Write-only: the field is empty again, only the last four characters come back.
    await expect(field).toHaveValue('');
    await expect(keys.getByText('Gespeichert, endet auf wxyz')).toBeVisible();
    expect((await settingsOf(owner)).keys.youtube).toEqual({ source: 'stored', hint: 'wxyz' });

    // "Löschen": the dialog takes the focus, Escape closes it, Enter confirms.
    const remove = keys.getByRole('button', { name: 'YouTube-Schlüssel löschen' });
    await remove.focus();
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('alertdialog', { name: 'YouTube-Schlüssel löschen?' });
    await expect(dialog).toBeVisible();
    await expectVisibleFocus(page);
    await page.keyboard.press('Escape');
    await expect(dialog).toHaveCount(0);
    await expectFocusOn(page, remove);
    await page.keyboard.press('Enter');
    await expect(dialog.getByRole('button', { name: 'Löschen' })).toBeFocused();
    await page.keyboard.press('Enter');
    await expect(keys.getByText('YouTube-Schlüssel gelöscht.')).toBeVisible();
    await expect(remove).toHaveCount(0);
    expect((await settingsOf(owner)).keys.youtube?.source).toBe('none');
    await expectNoFocusTrap(page, 120);
  } finally {
    if ((await settingsOf(owner)).keys.youtube?.source === 'stored') {
      await putSettings(owner, { clear: ['youtube'] });
    }
  }
});

test('the public address and the update check by keyboard', async ({ page, owner }) => {
  const before = await settingsOf(owner);
  try {
    await openIntegrations(page);
    const address = region(page, 'Öffentliche Adresse');
    const field = address.getByLabel('Adresse');
    // Emptied once reached: a field focused before resetFocus would be where Firefox resumes Tab.
    await resetFocus(page);
    await tabTo(page, field, 120);
    await field.fill('');
    // Not an http(s) address: the field says so and nothing is sent.
    await page.keyboard.type('media.e2e.test');
    await page.keyboard.press('Enter');
    await expect(field).toHaveAttribute('aria-invalid', 'true');
    await expect(address.getByText(/Gib eine Adresse mit http:\/\/ oder https:\/\//)).toBeVisible();
    await field.fill('https://media.e2e.test');
    await field.focus();
    await page.keyboard.press('Enter');
    await expect(address.getByText('Öffentliche Adresse gespeichert.')).toBeVisible();
    expect((await settingsOf(owner)).publicUrl).toBe('https://media.e2e.test');

    // The update check: Space switches it off and on; each change is stored at once.
    const update = region(page, 'Update-Prüfung');
    const toggle = update.getByRole('checkbox', { name: 'Täglich nach neuen Versionen suchen' });
    await expect(toggle).toBeChecked({ checked: before.updateCheckEnabled });
    await toggle.focus();
    await expectVisibleFocus(page);
    await page.keyboard.press('Space');
    await expect(update.getByText(/Update-Prüfung (aus|ein)geschaltet\./)).toBeVisible();
    await expect(toggle).toBeChecked({ checked: !before.updateCheckEnabled });
    expect((await settingsOf(owner)).updateCheckEnabled).toBe(!before.updateCheckEnabled);
    await expectFocusOn(page, toggle);
    await page.keyboard.press('Space');
    await expect(toggle).toBeChecked({ checked: before.updateCheckEnabled });
    expect((await settingsOf(owner)).updateCheckEnabled).toBe(before.updateCheckEnabled);
  } finally {
    await putSettings(owner, {
      updateCheckEnabled: before.updateCheckEnabled,
      ...(before.publicUrlSource === 'stored' && before.publicUrl
        ? { publicUrl: before.publicUrl }
        : before.publicUrlSource === 'env'
          ? {}
          : { clear: ['publicUrl'] }),
    });
  }
});

test('"Schlüssel neu erzeugen" asks first; Escape keeps the keys', async ({ page, owner }) => {
  const before = await settingsOf(owner);
  await openIntegrations(page);
  const push = region(page, 'Web-Push');
  const rotate = push.getByRole('button', { name: 'Schlüssel neu erzeugen' });
  await resetFocus(page);
  await tabTo(page, rotate, 120);
  await page.keyboard.press('Enter');
  const dialog = page.getByRole('alertdialog', { name: 'Neue Web-Push-Schlüssel erzeugen?' });
  await expect(dialog).toContainText('Alle Geräte verlieren ihr Abo');
  await expectVisibleFocus(page);
  await page.keyboard.press('Escape');
  await expect(dialog).toHaveCount(0);
  await expectFocusOn(page, rotate);
  expect(await settingsOf(owner)).toEqual(before);
});

test('a newer release in the manifest shows the update badge; switching the check off removes it (A-5)', async ({
  page,
  owner,
}) => {
  const before = await settingsOf(owner);
  try {
    // The manifest mock answers 404 by default; a release far ahead of any build version is newer.
    await setUpdateManifest('99.0.0');
    await putSettings(owner, { updateCheckEnabled: true });
    const checked = await owner.post('/api/v1/admin/update/check', { headers: CSRF });
    expect(checked.status(), await checked.text()).toBe(200);
    expect(((await checked.json()) as { available: boolean }).available).toBe(true);

    await page.goto('/admin/integrations');
    const badge = page.getByRole('link', { name: 'Update auf Version 99.0.0 verfügbar' });
    await expect(badge).toBeVisible();
    await expect(badge).toHaveAttribute('href', '/admin');
    await resetFocus(page);
    await tabTo(page, badge, 20);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/admin$/);
    await expect(page.getByText('Version 99.0.0 ist verfügbar')).toBeVisible();

    // Switching the daily check off forgets the known release: no badge, no notice.
    await putSettings(owner, { updateCheckEnabled: false });
    await page.reload();
    await expect(page.getByRole('heading', { level: 1, name: 'Verwaltung' })).toBeVisible();
    await expect(page.getByRole('link', { name: /^Update auf Version/ })).toHaveCount(0);
  } finally {
    await setUpdateManifest(null);
    await putSettings(owner, { updateCheckEnabled: false });
    await putSettings(owner, { updateCheckEnabled: before.updateCheckEnabled });
  }
});

test('browser back leaves the page for the overview', async ({ page }) => {
  await page.goto('/admin');
  await expect(page.getByRole('heading', { level: 1, name: 'Verwaltung' })).toBeVisible();
  await page
    .getByRole('navigation', { name: 'Verwaltung' })
    .getByRole('link', { name: 'Integrationen' })
    .click();
  await expect(page.getByRole('heading', { level: 1, name: 'Integrationen' })).toBeVisible();
  await page.goBack();
  await expect(page).toHaveURL(/\/admin$/);
});
