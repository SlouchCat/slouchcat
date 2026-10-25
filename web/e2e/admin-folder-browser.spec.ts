import { expect, test, type Locator, type Page } from '@playwright/test';
import { expectFocusOn, expectNoFocusTrap, expectVisibleFocus, resetFocus } from './keyboard';

// Keyboard test of the folder browser in the library form (CLAUDE.md rule 8, idea 02 step 10):
// arrow keys move and mark, Enter and the right arrow go down, Backspace comes back up, Escape
// returns the focus to the button, every row shows a ring, and the page keeps no focus trap. The
// suite's browsable root is testdata/media (web/e2e/start-server.mjs), which holds the library
// folders of the run.

const browse = (page: Page) => page.getByRole('button', { name: 'Durchsuchen …' });
const folder = (page: Page) => page.getByLabel('Ordner', { exact: true });
const rows = (page: Page) => page.getByRole('option');
const movies = (page: Page) => page.getByRole('option', { name: /Movies/ });
/** What "Apply" would write, as the dialog's footer states it. */
const applies = (page: Page) => page.getByText(/^«Übernehmen» schreibt: /);

/** Presses ArrowDown until `row` has the focus; fails after `max` presses. */
async function arrowDownTo(page: Page, row: Locator, max = 12) {
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
  expect(count).toBeGreaterThan(1);
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, rows(page).nth(1));
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, rows(page).first());
  await page.keyboard.press('End');
  await expectFocusOn(page, rows(page).nth(count - 1));
  await page.keyboard.press('Home');
  await expectFocusOn(page, rows(page).first());
  await expectVisibleFocus(page);
  // The list is one Tab stop: Tab leaves it altogether instead of walking to the next row.
  await page.keyboard.press('Tab');
  const insideList = await page.evaluate(
    () => document.activeElement?.closest('[role="listbox"]') !== null,
  );
  expect(insideList, 'Tab left the list').toBe(false);
});

test('Enter goes into a folder, Backspace comes back, Apply writes the marked folder', async ({
  page,
}) => {
  await openDialog(page);
  // The first root is testdata/media itself; below it lie the folders of the test libraries.
  await page.keyboard.press('Enter');
  await expect(movies(page)).toBeVisible();

  // Down to "Movies" with the arrow keys alone: the mark follows the focus, so the footer
  // already says what "Apply" would write.
  await arrowDownTo(page, movies(page));
  await expectFocusOn(page, movies(page));
  await expect(applies(page)).toContainText(/testdata\/media\/Movies$/);
  // Enter still enters the folder after a row was marked.
  await page.keyboard.press('Enter');
  await expect(
    page
      .getByRole('dialog')
      .getByText(/testdata\/media\/Movies$/)
      .first(),
  ).toBeVisible();
  await expect(movies(page)).toHaveCount(0);

  // Backspace goes back up to the root of the browsable root, and once more to the roots list.
  await page.keyboard.press('Backspace');
  await expect(movies(page)).toBeVisible();
  await page.keyboard.press('Backspace');
  await expect(page.getByRole('dialog').getByText('Freigegebene Ordner')).toBeVisible();

  // Back down with the right arrow instead of Enter, then apply the marked folder.
  await page.keyboard.press('ArrowRight');
  await expect(movies(page)).toBeVisible();
  await arrowDownTo(page, movies(page));
  const chosen = (await applies(page).textContent())?.replace('«Übernehmen» schreibt: ', '').trim();
  expect(chosen).toMatch(/testdata\/media\/Movies$/);
  await page.getByRole('button', { name: 'Übernehmen' }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(folder(page)).toHaveValue(String(chosen));
  // The focus is in the folder field, so the path can be corrected without tabbing back.
  await expectFocusOn(page, folder(page));
  await folder(page).fill('');
});

test('a folder already in the field opens the dialog in that folder', async ({ page }) => {
  await openDialog(page);
  await page.keyboard.press('Enter');
  await arrowDownTo(page, movies(page));
  await page.getByRole('button', { name: 'Übernehmen' }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  const chosen = await folder(page).inputValue();
  expect(chosen).toMatch(/testdata\/media\/Movies$/);

  // Reopened, the dialog stands in that folder instead of on the roots.
  await openDialog(page);
  await expect(page.getByRole('dialog').getByText(chosen).first()).toBeVisible();
  await page.keyboard.press('Escape');
  await folder(page).fill('');
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
  await expect(movies(page)).toContainText('Gehört zu Bibliothek');
});
