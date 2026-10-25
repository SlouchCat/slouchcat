import { expect, test, type Locator, type Page } from '@playwright/test';
import { expectFocusOn, expectNoFocusTrap, expectVisibleFocus, resetFocus } from './keyboard';

// Keyboard test of the folder browser in the library form (CLAUDE.md rule 8, idea 02 steps 10
// and 13a): arrow keys move and mark, Enter and the right arrow go down, Backspace comes back
// up, Escape returns the focus to the button, every row shows a ring, and the page keeps no
// focus trap. Since E18 the dialog carries a path field of its own, so the two "Ordner" fields
// are told apart by where they sit. The suite's browsable root is testdata/media
// (web/e2e/start-server.mjs), which holds the library folders of the run.

const browse = (page: Page) => page.getByRole('button', { name: 'Durchsuchen …' }).first();
/** The folder field of the "add library" form, the one the dialog writes into. */
const folder = (page: Page) =>
  page.getByRole('region', { name: 'Bibliothek hinzufügen' }).getByLabel('Ordner', { exact: true });
/** The path field inside the dialog, which "OK" hands on (E18). */
const dialogFolder = (page: Page) => page.getByRole('dialog').getByLabel('Ordner', { exact: true });
const rows = (page: Page) => page.getByRole('option');
const movies = (page: Page) => page.getByRole('option', { name: /Movies/ });

/** Presses ArrowDown until `row` has the focus; fails after `max` presses. */
async function arrowDownTo(page: Page, row: Locator, max = 12) {
  for (let presses = 0; presses < max; presses += 1) {
    if (await row.evaluate((element) => element === document.activeElement)) return;
    await page.keyboard.press('ArrowDown');
  }
  throw new Error(`ArrowDown did not reach the row within ${max} presses`);
}

/**
 * Opens the dialog from the keyboard and waits until the rows are there. With one configured
 * root the dialog opens inside it, so the first rows are the folders of testdata/media.
 */
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

test('the dialog opens inside the root, walks the rows and shows a ring on each', async ({
  page,
}) => {
  await openDialog(page);
  // One configured root, so the dialog stands in it and its folders are the first thing shown
  // instead of a list with a single row to click through.
  await expect(movies(page)).toBeVisible();
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

test('Enter goes into a folder, Backspace comes back, OK writes the path field', async ({
  page,
}) => {
  await openDialog(page);
  await expect(movies(page)).toBeVisible();

  // Down to "Movies" with the arrow keys, then Enter: entering a folder is what writes the path
  // into the dialog's own field, which is what "OK" hands on (E18). The rows carry no selection
  // of their own -- the folder the dialog stands in is the answer.
  await arrowDownTo(page, movies(page));
  await expectFocusOn(page, movies(page));
  await page.keyboard.press('Enter');
  await expect(dialogFolder(page)).toHaveValue(/testdata\/media\/Movies$/);
  await expect(
    page
      .getByRole('dialog')
      .getByText(/testdata\/media\/Movies$/)
      .first(),
  ).toBeVisible();
  await expect(movies(page)).toHaveCount(0);

  // Backspace goes back up to the browsable root, and once more to the list of roots.
  await page.keyboard.press('Backspace');
  await expect(movies(page)).toBeVisible();
  await page.keyboard.press('Backspace');
  await expect(page.getByRole('dialog').getByText('Freigegebene Ordner')).toBeVisible();

  // Back down with the right arrow instead of Enter, then into "Movies" and apply it.
  await page.keyboard.press('ArrowRight');
  await expect(movies(page)).toBeVisible();
  await arrowDownTo(page, movies(page));
  await page.keyboard.press('ArrowRight');
  const chosen = await dialogFolder(page).inputValue();
  expect(chosen).toMatch(/testdata\/media\/Movies$/);
  await page.getByRole('button', { name: 'OK' }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(folder(page)).toHaveValue(String(chosen));
  // The focus is in the folder field, so the path can be corrected without tabbing back.
  await expectFocusOn(page, folder(page));
  await folder(page).fill('');
});

test('a folder already in the field opens the dialog in that folder', async ({ page }) => {
  await openDialog(page);
  await movies(page).click();
  await page.getByRole('button', { name: 'OK' }).click();
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
  await expect(movies(page)).toContainText('Gehört zu Bibliothek');
});

test('one click enters a folder instead of marking it', async ({ page }) => {
  await openDialog(page);
  await expect(movies(page)).toBeVisible();
  // A single click, not a double one: the folder opens and its path lands in the field.
  await movies(page).click();
  await expect(movies(page)).toHaveCount(0);
  await expect(dialogFolder(page)).toHaveValue(/testdata\/media\/Movies$/);
  await page.keyboard.press('Escape');
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await folder(page).fill('');
});

test('the path field takes a typed folder and Backspace there edits instead of going up', async ({
  page,
}) => {
  await openDialog(page);
  await expect(movies(page)).toBeVisible();
  const inside = await dialogFolder(page).inputValue();

  // Tab out of the list and into the dialog's path field, then type a folder below the root.
  await dialogFolder(page).click();
  await dialogFolder(page).fill(`${inside}/Movies`);
  await expectVisibleFocus(page);
  // Backspace in the field edits the text; it must not walk the list one level up (E18).
  await page.keyboard.press('Backspace');
  await expect(dialogFolder(page)).toHaveValue(`${inside}/Movie`);
  await expect(movies(page)).toBeVisible();

  await dialogFolder(page).fill(`${inside}/Movies`);
  await page.getByRole('button', { name: 'Gehe zu' }).click();
  await expect(movies(page)).toHaveCount(0);
  await page.getByRole('button', { name: 'OK' }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(folder(page)).toHaveValue(`${inside}/Movies`);
  await folder(page).fill('');
});

test('a path below no browsable folder stays in the field and is explained', async ({ page }) => {
  await openDialog(page);
  await dialogFolder(page).fill('/etc/geheim');
  await page.getByRole('button', { name: 'Gehe zu' }).click();
  await expect(page.getByText(/liegt unter keinem freigegebenen Ordner/)).toBeVisible();
  // The dialog refuses nothing: the server decides when the form is submitted (rule 12).
  await expect(dialogFolder(page)).toHaveValue('/etc/geheim');
  await page.keyboard.press('Escape');
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(folder(page)).toHaveValue('');
});
