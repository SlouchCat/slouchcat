import { expect, test, type Page } from '@playwright/test';
import { expectFocusOn, expectNoFocusTrap, expectVisibleFocus, resetFocus } from './keyboard';

// Keyboard test of the folder browser in the library form (CLAUDE.md rule 8, idea 02 step 10):
// arrow keys move, Enter goes down, Backspace comes back up, Escape returns the focus to the
// button, every row shows a ring, and the page keeps no focus trap. The suite's browsable root
// is testdata/media (web/e2e/start-server.mjs), which holds the library folders of the run.

const browse = (page: Page) => page.getByRole('button', { name: 'Durchsuchen …' });
const folder = (page: Page) => page.getByLabel('Ordner', { exact: true });
const rows = (page: Page) => page.getByRole('option');

/** Presses ArrowDown until `row` has the focus; fails after `max` presses. */
async function arrowDownTo(page: Page, row: ReturnType<Page['getByRole']>, max = 12) {
  for (let presses = 0; presses < max; presses += 1) {
    if (await row.evaluate((element) => element === document.activeElement)) return;
    await page.keyboard.press('ArrowDown');
  }
  throw new Error(`ArrowDown did not reach the row within ${max} presses`);
}

/** Opens the dialog from the keyboard and waits until the roots are listed. */
async function openDialog(page: Page) {
  await browse(page).focus();
  await page.keyboard.press('Enter');
  await expect(page.getByRole('dialog')).toBeVisible();
  await expect(rows(page).first()).toBeVisible();
}

test.beforeEach(async ({ page }) => {
  await page.goto('/admin/libraries');
  await expect(page.getByRole('heading', { level: 1, name: 'Bibliotheken' })).toBeVisible();
  await expect(page.getByRole('article', { name: 'Filme' })).toBeVisible();
  await resetFocus(page);
});

test('the dialog opens on the roots, walks the rows and shows a ring on each', async ({ page }) => {
  await openDialog(page);
  // The focus lands in the list, not on the first breadcrumb or button (idea 02, E11).
  await expectFocusOn(page, rows(page).first());
  const count = await rows(page).count();
  expect(count).toBeGreaterThan(0);
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, rows(page).nth(Math.min(1, count - 1)));
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, rows(page).first());
  await page.keyboard.press('End');
  await expectFocusOn(page, rows(page).nth(count - 1));
  await page.keyboard.press('Home');
  await expectFocusOn(page, rows(page).first());
  await expectVisibleFocus(page);
  // The list is one Tab stop: Tab leaves it instead of walking its rows.
  await page.keyboard.press('Tab');
  await expect(rows(page).first()).not.toBeFocused();
});

test('Enter goes into a folder, Backspace comes back, Apply writes the path into the field', async ({
  page,
}) => {
  await openDialog(page);
  // The first root is testdata/media itself; below it lie the folders of the test libraries.
  const root = await rows(page).first().textContent();
  await page.keyboard.press('Enter');
  await expect(page.getByRole('option', { name: /Movies/ })).toBeVisible();

  // Down to "Movies" with the arrow keys alone, then into it.
  const movies = page.getByRole('option', { name: /Movies/ });
  await arrowDownTo(page, movies);
  await expectFocusOn(page, movies);
  await page.keyboard.press('Enter');
  await expect(page.getByRole('option').first()).toBeVisible();
  await expect(page.getByRole('dialog').getByText(/testdata\/media\/Movies$/)).toBeVisible();

  // Backspace goes back up to the root of the browsable root.
  await page.keyboard.press('Backspace');
  await expect(page.getByRole('option', { name: /Movies/ })).toBeVisible();

  // Apply without a marked row takes the folder the dialog stands in.
  await page.getByRole('button', { name: 'Übernehmen' }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(folder(page)).toHaveValue(String(root).trim().split('\n')[0].trim());
  // The focus is in the folder field, so the path can be corrected without tabbing back.
  await expectFocusOn(page, folder(page));
});

test('Escape closes without a change and returns the focus to the button', async ({ page }) => {
  await folder(page).fill('/unveraendert');
  await openDialog(page);
  await page.keyboard.press('Escape');
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(folder(page)).toHaveValue('/unveraendert');
  await expectFocusOn(page, browse(page));
  await folder(page).fill('');
  await expectNoFocusTrap(page);
});

test('a folder that already belongs to a library says so', async ({ page }) => {
  await openDialog(page);
  await page.keyboard.press('Enter');
  await expect(page.getByRole('option', { name: /Movies/ })).toContainText('Gehört zu Bibliothek');
});
