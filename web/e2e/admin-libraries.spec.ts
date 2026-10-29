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
  await expectFocusOn(page, page.getByRole('radio', { name: 'Video' }));
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, page.getByRole('radio', { name: 'Hörbücher' }));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByLabel('Ordner', { exact: true }));
  // The folder browser of idea 02 sits right after the field, as the wizard's check does.
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('button', { name: 'Durchsuchen …' }));
  // An audiobook folder has one content and one source: nothing to choose, the next stop is the
  // button for another card (F-13), then the default age rating.
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('button', { name: 'Weiteren Ordner hinzufügen' }));
  await page.keyboard.press('Tab');
  await expectFocusOn(
    page,
    page.getByRole('radiogroup', { name: 'Standard-Altersstufe' }).getByRole('radio', {
      name: 'FSK 0',
    }),
  );
  await page.keyboard.press('Tab');
  await expectFocusOn(
    page,
    page.getByRole('button', { name: 'Bibliothek mit 1 Ordner hinzufügen' }),
  );
  await expectNoFocusTrap(page);
});

test('content and metadata sources by keyboard: Space ticks, arrows sort, cards come and go (ADR 0015)', async ({
  page,
}) => {
  const form = page.getByRole('region', { name: 'Bibliothek hinzufügen' });
  await tabTo(page, page.getByLabel('Name', { exact: true }));
  await page.keyboard.type('E2E Anime');
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('radio', { name: 'Video' }));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByLabel('Ordner', { exact: true }));
  await page.keyboard.type('/does/not/exist/slouchcat-e2e-anime');
  await page.keyboard.press('Tab'); // Durchsuchen …
  // «Inhalt»: films are chosen; the arrow switches to series, whose default list is TMDB › TVDB.
  await page.keyboard.press('Tab');
  await expectFocusOn(page, form.getByRole('radio', { name: 'Filme' }));
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, form.getByRole('radio', { name: 'Serien' }));
  await expect(form.getByRole('radio', { name: 'Serien' })).toBeChecked();
  const sources = form.getByRole('group', { name: 'Metadatenquellen' });
  await expect(sources.getByRole('checkbox', { name: 'TMDB' })).toBeChecked();
  await expect(sources.getByRole('checkbox', { name: 'TVDB' })).toBeChecked();
  await expect(sources.getByRole('checkbox', { name: 'AniList' })).not.toBeChecked();
  // Tab walks the rows: checkbox, up, down; Space ticks AniList.
  await page.keyboard.press('Tab');
  await expectFocusOn(page, sources.getByRole('checkbox', { name: 'TMDB' }));
  await expectVisibleFocus(page);
  await tabTo(page, sources.getByRole('checkbox', { name: 'AniList' }), 8);
  await page.keyboard.press('Space');
  await expect(sources.getByRole('checkbox', { name: 'AniList' })).toBeChecked();
  await expect(sources.getByText('füllt auf')).toHaveCount(2);
  // ArrowUp on the focused sort button moves the row; the focus stays on the button.
  await page.keyboard.press('Tab');
  await expectFocusOn(page, sources.getByRole('button', { name: 'AniList nach oben' }));
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, sources.getByRole('button', { name: 'AniList nach oben' }));
  await expect(sources.getByRole('checkbox').nth(1)).toHaveAccessibleName('AniList');
  // In front it may not stand: the list says so before the server does.
  await page.keyboard.press('ArrowUp');
  await expect(sources.getByRole('alert')).toHaveText(/AniList kann keinen Titel erkennen/);
  await expect(
    page.getByRole('button', { name: 'Bibliothek mit 1 Ordner hinzufügen' }),
  ).toBeDisabled();
  await page.keyboard.press('ArrowDown');
  await expect(sources.getByRole('alert')).toHaveCount(0);
  // «+ Weiteren Ordner hinzufügen» by Enter: the new card takes the focus in its path field and
  // proposes the other content; «Entfernen» hands the focus back to the button.
  await tabTo(page, form.getByRole('button', { name: 'Weiteren Ordner hinzufügen' }), 20);
  await page.keyboard.press('Enter');
  const second = form.getByRole('listitem', { name: 'Ordner 2' });
  await expectFocusOn(page, second.getByLabel('Ordner', { exact: true }));
  await expect(second.getByRole('radio', { name: 'Filme' })).toBeChecked();
  await expect(
    page.getByRole('button', { name: 'Bibliothek mit 2 Ordnern hinzufügen' }),
  ).toBeVisible();
  // «Entfernen» stands in the card's head, right before the path field.
  await page.keyboard.press('Shift+Tab');
  await expectFocusOn(page, second.getByRole('button', { name: 'Ordner 2 entfernen' }));
  await page.keyboard.press('Enter');
  await expect(second).toHaveCount(0);
  await expectFocusOn(page, form.getByRole('button', { name: 'Weiteren Ordner hinzufügen' }));
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
  await expectFocusOn(page, page.getByRole('radio', { name: 'Video' }));
  // A video library has no default rating; the second kind is audiobooks.
  await expect(rating).toHaveCount(0);
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
  await expectFocusOn(page, page.getByRole('button', { name: 'Weiteren Ordner hinzufügen' }));
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
  // Podcasts: the card goes, the hint says why; the add button and the rating stay (F-13).
  await page.getByRole('radio', { name: 'Hörbücher' }).focus();
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, page.getByRole('radio', { name: 'Podcasts' }));
  await expect(folder).toHaveCount(0);
  await expect(page.getByText(/Ordner optional/)).toBeVisible();
  // Without a folder field there is nothing to browse either; the next stop is the add button.
  await expect(page.getByRole('button', { name: 'Durchsuchen …' })).toHaveCount(0);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('button', { name: 'Ordner hinzufügen', exact: true }));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, rating.getByRole('radio', { name: 'FSK 6' }));
  await expect(
    page.getByRole('button', { name: 'Bibliothek ohne Ordner hinzufügen' }),
  ).toBeVisible();
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
