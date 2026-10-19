import { expect, test, type Page } from '@playwright/test';
import { expectFocusOn, expectNoFocusTrap, resetFocus, tabTo } from './keyboard';

// Other specs add a second library, so the rescan buttons are scoped to the test library.
const filme = (page: Page) => page.getByRole('article', { name: 'Filme' });

test.beforeEach(async ({ page }) => {
  await page.goto('/admin/libraries');
  await expect(page.getByRole('heading', { level: 1, name: 'Bibliotheken' })).toBeVisible();
  await expect(page.getByRole('article', { name: 'Filme' })).toBeVisible();
  await resetFocus(page);
});

test('the scanned test library is listed with its state and counts', async ({ page }) => {
  const library = page.getByRole('article', { name: 'Filme' });
  await expect(library.getByText('Aktuell')).toBeVisible();
  await expect(library.getByText(/Titel: \d+ · Dateien: \d+/)).toBeVisible();
  await expect(library.getByText(/testdata\/media\/Movies$/)).toBeVisible();
});

test('Tab reaches header, rescan and the add form in order; no focus trap', async ({ page }) => {
  await tabTo(page, page.getByRole('link', { name: 'Bibliotheken' }));
  await tabTo(page, filme(page).getByRole('button', { name: 'Erneut scannen' }));
  await tabTo(page, page.getByLabel('Name', { exact: true }), 3);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('radio', { name: 'Filme' }));
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, page.getByRole('radio', { name: 'Serien' }));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByLabel('Ordner', { exact: true }));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('button', { name: 'Hinzufügen' }));
  await expectNoFocusTrap(page);
});

test('Enter on "Erneut scannen" queues a scan that finishes', async ({ page }) => {
  await tabTo(page, filme(page).getByRole('button', { name: 'Erneut scannen' }));
  const scan = page.waitForResponse(
    (response) => response.request().method() === 'POST' && response.url().endsWith('/scan'),
  );
  await page.keyboard.press('Enter');
  expect((await scan).status()).toBe(202);
  await expect(page.getByRole('article', { name: 'Filme' }).getByText('Aktuell')).toBeVisible({
    timeout: 60_000,
  });
});

test('an invalid path is reported at the field; Enter submits from the field', async ({ page }) => {
  await page.getByLabel('Name', { exact: true }).fill('Fehlt');
  await page.getByLabel('Ordner', { exact: true }).fill('/does/not/exist/slouchcat-e2e');
  await page.getByLabel('Ordner', { exact: true }).focus();
  await page.keyboard.press('Enter');
  await expect(page.getByLabel('Ordner', { exact: true })).toHaveAttribute('aria-invalid', 'true');
  await expect(page.getByText(/Diesen Ordner gibt es auf dem Server nicht/)).toBeVisible();
});

test('browser back leaves the page to the start page', async ({ page }) => {
  // Both pages must have rendered before going back: Firefox aborts a history navigation that
  // races the router's own history update on load (page.goBack: NS_ERROR_FAILURE).
  await page.goto('/');
  await expect(page.getByRole('heading', { level: 2, name: 'Neu hinzugefügt' })).toBeVisible();
  await page.goto('/admin/libraries');
  await expect(page.getByRole('article', { name: 'Filme' })).toBeVisible();
  await page.goBack();
  await expect(page).toHaveURL(/\/$/);
});
