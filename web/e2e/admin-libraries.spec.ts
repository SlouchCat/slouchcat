import { expect, test, type Page, type Route } from '@playwright/test';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

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
  // The admin navigation is one Tab stop: its current page.
  await tabTo(page, page.getByRole('link', { name: 'Bibliotheken' }));
  await tabTo(page, filme(page).getByRole('button', { name: 'Erneut scannen' }));
  // Other libraries ("Serien", "Hörbücher", "Podcasts", "E2E leer") have their buttons in between,
  // as many as there are libraries: no fixed budget.
  await tabTo(page, page.getByLabel('Name', { exact: true }));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('radio', { name: 'Filme' }));
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, page.getByRole('radio', { name: 'Serien' }));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByLabel('Ordner', { exact: true }));
  // The folder browser of idea 02 sits right after the field, as the wizard's check does.
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('button', { name: 'Durchsuchen …' }));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('button', { name: 'Hinzufügen' }));
  await expectNoFocusTrap(page);
});

test('audio kinds by keyboard: default rating with arrows, no folder for podcasts (M6)', async ({
  page,
}) => {
  const folder = page.getByLabel('Ordner', { exact: true });
  const rating = page.getByRole('radiogroup', { name: 'Standard-Altersstufe' });
  await tabTo(page, page.getByLabel('Name', { exact: true }));
  await page.keyboard.type('E2E Hörbücher');
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('radio', { name: 'Filme' }));
  // Movies and series have no default rating; the third kind is audiobooks.
  await expect(rating).toHaveCount(0);
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, page.getByRole('radio', { name: 'Hörbücher' }));
  await expect(page.getByRole('radio', { name: 'Hörbücher' })).toBeChecked();
  await expect(rating).toBeVisible();
  // Tab: folder, then the rating (FSK 0 by default); the arrows choose FSK 6.
  await page.keyboard.press('Tab');
  await expectFocusOn(page, folder);
  await page.keyboard.type('/does/not/exist/slouchcat-e2e-audio');
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('button', { name: 'Durchsuchen …' }));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, rating.getByRole('radio', { name: 'FSK 0' }));
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, rating.getByRole('radio', { name: 'FSK 6' }));
  await expect(rating.getByRole('radio', { name: 'FSK 6' })).toBeChecked();
  await expectVisibleFocus(page);
  // Enter in the folder sends kind and rating; the missing folder is reported at the field.
  const sent = page.waitForRequest(
    (request) => request.method() === 'POST' && request.url().endsWith('/admin/libraries'),
  );
  await folder.focus();
  await page.keyboard.press('Enter');
  expect((await sent).postDataJSON()).toMatchObject({ kind: 'audiobooks', defaultRating: 6 });
  await expect(folder).toHaveAttribute('aria-invalid', 'true');
  // Podcasts: the folder field goes, the hint says why; the rating stays.
  await page.getByRole('radio', { name: 'Hörbücher' }).focus();
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, page.getByRole('radio', { name: 'Podcasts' }));
  await expect(folder).toHaveCount(0);
  await expect(page.getByText(/Podcasts brauchen keinen Ordner/)).toBeVisible();
  // Without a folder field there is nothing to browse either; the next stop is the rating.
  await expect(page.getByRole('button', { name: 'Durchsuchen …' })).toHaveCount(0);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, rating.getByRole('radio', { name: 'FSK 6' }));
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

test('"Scan abbrechen" shows while a scan is queued or running and cancels it', async ({
  page,
}) => {
  // Hold the list requests back a little, so the queued state the scan request answered with
  // stays on screen long enough to act on (the test library scans in well under a second).
  // The handler may still be waiting when the test unroutes it: then the request is gone already.
  const delay = async (route: Route) => {
    await new Promise((resolve) => setTimeout(resolve, 1500));
    await route.continue().catch(() => undefined);
  };
  await page.route('**/api/v1/admin/libraries', delay);
  await tabTo(page, filme(page).getByRole('button', { name: 'Erneut scannen' }));
  await page.keyboard.press('Enter');
  const cancel = filme(page).getByRole('button', { name: 'Scan abbrechen' });
  await expect(cancel).toBeVisible();
  const cancelled = page.waitForResponse(
    (response) => response.request().method() === 'DELETE' && response.url().endsWith('/scan'),
  );
  await cancel.focus();
  await page.keyboard.press('Enter');
  const response = await cancelled;
  expect(response.status()).toBe(202);
  const library = (await response.json()) as { scan: { state: string; lastError?: string } };
  await page.unroute('**/api/v1/admin/libraries', delay);

  // The cancel either stopped the scan, or the (small) scan finished just before it arrived;
  // anything else, or a finished scan with an error, is a failure.
  expect(['aborted', 'done']).toContain(library.scan.state);
  if (library.scan.state === 'done') expect(library.scan.lastError).toBeUndefined();
  if (library.scan.state === 'aborted') {
    expect(library.scan.lastError).toBe('cancelled');
    await expect(
      filme(page).getByText('Der Scan wurde abgebrochen. Der nächste Scan läuft wieder normal.'),
    ).toBeVisible();
    // A cancelled scan needs no loss acceptance.
    await expect(filme(page).getByRole('button', { name: 'Verlust akzeptieren' })).toHaveCount(0);
  }
  await expect(cancel).toHaveCount(0);

  // Leave the library scanned for the other specs.
  await filme(page).getByRole('button', { name: 'Erneut scannen' }).click();
  await expect(filme(page).getByText('Aktuell')).toBeVisible({ timeout: 60_000 });
});
