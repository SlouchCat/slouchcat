import { expect, test } from '@playwright/test';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// The real wizard ran in global-setup.ts through the API. To test the page's keyboard behaviour
// the setup state is answered as "required" and the submit is intercepted, so the run's account
// stays untouched.
// The service worker is blocked so that page.route sees every request.
test.use({ storageState: { cookies: [], origins: [] }, serviceWorkers: 'block' });

test.beforeEach(async ({ page }) => {
  await page.route('**/api/v1/setup', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({ json: { setupRequired: true } });
      return;
    }
    await route.fulfill({
      status: 422,
      contentType: 'application/problem+json',
      json: { type: 'about:blank', title: 'invalid', status: 422, code: 'library_path_invalid' },
    });
  });
  await page.goto('/setup');
  await expect(
    page.getByRole('heading', { level: 1, name: 'Willkommen bei Slouchcat' }),
  ).toBeVisible();
});

test('Tab walks through the three steps in order with a visible focus ring', async ({ page }) => {
  // The e-mail field has focus on arrival.
  await expectFocusOn(page, page.getByLabel('E-Mail-Adresse'));
  const order = [
    page.getByLabel('Passwort', { exact: true }),
    page.getByLabel('Passwort wiederholen'),
    page.getByLabel('Name', { exact: true }),
    page.getByRole('radio', { name: 'Filme' }),
    page.getByLabel('Ordner', { exact: true }),
    page.getByRole('link', { name: /Token auf themoviedb.org erstellen/ }),
    page.getByLabel('API-Lesezugriffs-Token'),
    page.getByRole('button', { name: 'Einrichtung abschliessen' }),
  ];
  for (const locator of order) {
    await page.keyboard.press('Tab');
    await expectFocusOn(page, locator);
  }
  await expectNoFocusTrap(page);
});

test('arrow keys switch the library kind', async ({ page }) => {
  await resetFocus(page);
  await tabTo(page, page.getByRole('radio', { name: 'Filme' }));
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, page.getByRole('radio', { name: 'Serien' }));
  await expect(page.getByRole('radio', { name: 'Serien' })).toBeChecked();
  // M6: audiobooks bring the default age rating; podcasts drop the folder.
  const rating = page.getByRole('radiogroup', { name: 'Standard-Altersstufe' });
  await expect(rating).toHaveCount(0);
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, page.getByRole('radio', { name: 'Hörbücher' }));
  await expect(rating).toBeVisible();
  await expect(page.getByLabel('Ordner', { exact: true })).toBeVisible();
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, page.getByRole('radio', { name: 'Podcasts' }));
  await expect(page.getByLabel('Ordner', { exact: true })).toHaveCount(0);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, rating.getByRole('radio', { name: 'FSK 0' }));
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, rating.getByRole('radio', { name: 'FSK 6' }));
  await expectVisibleFocus(page);
  await expectNoFocusTrap(page);
});

test('Enter submits; errors appear inline and the form stays usable', async ({ page }) => {
  await page.getByLabel('E-Mail-Adresse').fill('owner@example.test');
  await page.getByLabel('Passwort', { exact: true }).fill('correct horse battery');
  await page.getByLabel('Passwort wiederholen').fill('correct horse battery');
  await page.getByLabel('Ordner', { exact: true }).fill('/does/not/exist');
  await page.getByLabel('Ordner', { exact: true }).focus();
  await page.keyboard.press('Enter');

  const path = page.getByLabel('Ordner', { exact: true });
  await expect(path).toHaveAttribute('aria-invalid', 'true');
  await expect(page.getByText(/Diesen Ordner gibt es auf dem Server nicht/)).toBeVisible();
  await path.focus();
  await expectVisibleFocus(page);
});

test('client validation catches a short password before anything is sent', async ({ page }) => {
  let posted = false;
  page.on('request', (request) => {
    if (request.method() === 'POST' && request.url().endsWith('/api/v1/setup')) posted = true;
  });
  await page.getByLabel('Passwort', { exact: true }).fill('short');
  await page.getByRole('button', { name: 'Einrichtung abschliessen' }).focus();
  await page.keyboard.press('Enter');
  await expect(page.getByText('Mindestens 10 Zeichen.').last()).toBeVisible();
  expect(posted).toBe(false);
});
