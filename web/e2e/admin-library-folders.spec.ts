import { mkdirSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { expect, test, type Page } from '@playwright/test';
import { MEDIA_DIR } from './env';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// Keyboard test of the folder list of a library and of the typed confirmation that removes one
// (CLAUDE.md rule 8, idea 02 steps 13 and 14, E16 and E17): the card names how many folders
// there are, the dialog lists them, "Ordner hinzufügen" puts one in, and removing one stays
// refused until its path has been typed out character for character.
//
// It works on a library of its own, in a hidden folder below the media root of the run: hidden
// entries are skipped by the scanner and by the folder browser, so the other specs never see it.
// There is no API to delete a library, so it stays for the rest of the run, like "E2E leer".

const NAME = 'E2E Ordner';
const base = join(MEDIA_DIR, '.e2e', NAME);
const first = join(base, 'eins');
const second = join(base, 'zwei');
const third = join(base, 'drei');
const fourth = join(base, 'vier');

const card = (page: Page) => page.getByRole('article', { name: NAME });
const dialog = (page: Page) => page.getByRole('dialog', { name: `Ordner von ${NAME}` });
const confirm = (page: Page) =>
  page.getByRole('alertdialog', { name: 'Ordner wirklich entfernen?' });
const change = (page: Page) => page.getByRole('dialog', { name: 'Ordner ändern' });

/** Creates the folders on disk; a library folder has to exist before the server takes it. */
test.beforeAll(() => {
  for (const dir of [first, second, third, fourth]) {
    mkdirSync(dir, { recursive: true });
    writeFileSync(join(dir, 'readme.txt'), 'no media here');
  }
});

test.beforeEach(async ({ page }) => {
  await page.goto('/admin/libraries');
  await expect(page.getByRole('heading', { level: 1, name: 'Bibliotheken' })).toBeVisible();
  await expect(page.getByRole('article', { name: 'Filme' })).toBeVisible();

  if ((await card(page).count()) === 0) {
    await page.getByLabel('Name', { exact: true }).fill(NAME);
    await page.getByLabel('Ordner', { exact: true }).fill(first);
    await page.getByRole('button', { name: 'Bibliothek mit 1 Ordner hinzufügen' }).click();
    await expect(card(page)).toBeVisible();
  }
  await resetFocus(page);
});

// While a modal is open react-aria hides the rest of the page from the accessibility tree, so
// anything on the card is asserted with the dialog closed.
const closeDialog = async (page: Page) => {
  await dialog(page).getByRole('button', { name: 'Schliessen' }).click();
  await expect(dialog(page)).toHaveCount(0);
};

const openDialog = async (page: Page) => {
  await card(page)
    .getByRole('button', { name: `Ordner von ${NAME} bearbeiten` })
    .click();
  await expect(dialog(page)).toBeVisible();
};

test('the folders dialog opens from the keyboard and lists the folder', async ({ page }) => {
  const open = card(page).getByRole('button', { name: `Ordner von ${NAME} bearbeiten` });
  // Tabbed to, not focused by script: a ring only counts when the keyboard put it there.
  // The page carries a button or two per library, so the budget is generous.
  await tabTo(page, open, 80);
  await page.keyboard.press('Enter');

  await expect(dialog(page)).toBeVisible();
  await expect(dialog(page).getByText(first)).toBeVisible();
  // Tab walks the dialog and every stop shows a ring.
  await page.keyboard.press('Tab');
  await expectVisibleFocus(page);

  // Escape closes and hands the focus back to the button that opened it.
  await page.keyboard.press('Escape');
  await expect(dialog(page)).toHaveCount(0);
  await expectFocusOn(page, open);
  await expectNoFocusTrap(page);
});

/** Adds the paths as cards of «Neue Ordner» and submits them in one go (F-13). */
const addFolders = async (page: Page, paths: string[]) => {
  for (const [index, path] of paths.entries()) {
    await dialog(page)
      .getByRole('button', {
        name: index === 0 ? 'Ordner hinzufügen' : 'Weiteren Ordner hinzufügen',
        exact: true,
      })
      .click();
    const cardOf = dialog(page).getByRole('listitem', { name: `Ordner ${index + 1}` });
    await expectFocusOn(page, cardOf.getByLabel('Ordner', { exact: true }));
    await page.keyboard.type(path);
  }
  const submit = dialog(page).getByRole('button', {
    name: `${paths.length} Ordner hinzufügen`,
    exact: true,
  });
  await submit.click();
  for (const path of paths) await expect(dialog(page).getByText(path)).toBeVisible();
};

test('a second folder goes in, and comes out only after its path is typed', async ({ page }) => {
  await openDialog(page);

  // Add: «Ordner hinzufügen» opens a card, its path field takes the path, Enter in it submits.
  await dialog(page).getByRole('button', { name: 'Ordner hinzufügen', exact: true }).click();
  const field = dialog(page).getByLabel('Ordner', { exact: true });
  await expectFocusOn(page, field);
  await page.keyboard.type(second);
  await page.keyboard.press('Enter');
  await expect(dialog(page).getByText(second)).toBeVisible();

  // The card counts instead of naming a path (E16).
  await closeDialog(page);
  await expect(card(page).getByText('2 Ordner')).toBeVisible();
  await openDialog(page);

  // Remove: the confirming button stays refused until the path is repeated exactly.
  const removeRow = dialog(page).getByRole('button', { name: `Ordner ${second} entfernen` });
  await tabTo(page, removeRow);
  await page.keyboard.press('Enter');
  await expect(confirm(page)).toBeVisible();

  const remove = confirm(page).getByRole('button', { name: 'Ordner entfernen' });
  await expect(remove).toBeDisabled();
  const typed = confirm(page).getByLabel('Tippe den Pfad, um zu bestätigen');
  await expectFocusOn(page, typed);
  await expectVisibleFocus(page);

  // A near miss is still a miss; surrounding spaces are forgiven.
  await typed.fill(first);
  await expect(remove).toBeDisabled();
  await typed.fill(`${second}x`);
  await expect(remove).toBeDisabled();
  await typed.fill(`${second} `);
  await expect(remove).toBeEnabled();

  // Escape leaves everything as it was.
  await page.keyboard.press('Escape');
  await expect(confirm(page)).toHaveCount(0);
  await expect(dialog(page).getByText(second)).toBeVisible();

  // Now for real.
  await removeRow.click();
  await confirm(page).getByLabel('Tippe den Pfad, um zu bestätigen').fill(second);
  await confirm(page).getByRole('button', { name: 'Ordner entfernen' }).click();
  await expect(confirm(page)).toHaveCount(0);
  await expect(dialog(page).getByText(second)).toHaveCount(0);
  await closeDialog(page);
  await expect(card(page).getByText(first)).toBeVisible();
  await expectNoFocusTrap(page);
});

test('a library without a folder says so and offers no scan (E13)', async ({ page }) => {
  await openDialog(page);
  await dialog(page)
    .getByRole('button', { name: `Ordner ${first} entfernen` })
    .click();
  // The text names the last folder and the one-way door.
  await expect(confirm(page).getByText(/letzte Ordner/)).toBeVisible();
  await expect(confirm(page).getByText(/SLOUCHCAT_BROWSE_ROOTS/)).toBeVisible();
  await confirm(page).getByLabel('Tippe den Pfad, um zu bestätigen').fill(first);
  await confirm(page).getByRole('button', { name: 'Ordner entfernen' }).click();
  await expect(confirm(page)).toHaveCount(0);

  await expect(dialog(page).getByText(/Diese Bibliothek hat keinen Ordner/)).toBeVisible();
  await closeDialog(page);

  await expect(card(page).getByText(/Kein Ordner: Es wird nichts gescannt/)).toBeVisible();
  await expect(card(page).getByRole('button', { name: 'Erneut scannen' })).toBeDisabled();

  // Put it back, so the spec can run again on the same server.
  await openDialog(page);
  await addFolders(page, [first]);
  await closeDialog(page);
});

test('two folders go in with one submit, each with its own content (F-13)', async ({ page }) => {
  await openDialog(page);
  await addFolders(page, [third, fourth]);
  await closeDialog(page);
  // The second new card proposed the other content of a video library: a series folder.
  await expect(card(page).getByText('3 Ordner')).toBeVisible();
  await expect(card(page).getByText('2 Filme, 1 Serien')).toBeVisible();
  // Out again, so the other tests find the one folder they expect.
  await openDialog(page);
  for (const path of [third, fourth]) {
    await dialog(page)
      .getByRole('button', { name: `Ordner ${path} entfernen` })
      .click();
    await confirm(page).getByLabel('Tippe den Pfad, um zu bestätigen').fill(path);
    await confirm(page).getByRole('button', { name: 'Ordner entfernen' }).click();
    await expect(confirm(page)).toHaveCount(0);
  }
  await closeDialog(page);
  await expect(card(page).getByText(first)).toBeVisible();
});

test('«Ändern» edits content and sources by keyboard; a content change needs the typed path', async ({
  page,
}) => {
  await openDialog(page);
  const open = dialog(page).getByRole('button', { name: `Ordner ${first} ändern` });
  await tabTo(page, open, 20);
  await page.keyboard.press('Enter');
  await expect(change(page)).toBeVisible();

  // The fillers only: ticking AniList needs no confirmation, the dialog says what happens.
  const sources = change(page).getByRole('group', { name: 'Metadatenquellen' });
  await expect(change(page).getByRole('radio', { name: 'Filme' })).toBeChecked();
  await tabTo(page, sources.getByRole('checkbox', { name: 'AniList' }), 12);
  await expectVisibleFocus(page);
  await page.keyboard.press('Space');
  await expect(change(page).getByText(/Nur die Auffüller ändern sich/)).toBeVisible();
  const saved = page.waitForResponse(
    (response) => response.request().method() === 'PATCH' && /\/roots\/\d+$/.test(response.url()),
  );
  await change(page).getByRole('button', { name: 'Speichern' }).click();
  expect((await saved).status()).toBe(200);
  await expect(change(page)).toHaveCount(0);
  await expect(dialog(page).getByText(`Der Ordner ${first} ist geändert.`)).toBeVisible();
  const row = dialog(page).getByRole('listitem').filter({ hasText: first });
  await expect(row.getByText('TMDB › AniList')).toBeVisible();

  // The content: Escape leaves the confirmation, the typed path confirms it.
  await open.click();
  await change(page).getByRole('radio', { name: 'Filme' }).focus();
  await page.keyboard.press('ArrowRight');
  await expect(change(page).getByRole('radio', { name: 'Serien' })).toBeChecked();
  await expect(change(page).getByText(/wirkt wie Ordner entfernen und neu anlegen/)).toBeVisible();
  await change(page).getByRole('button', { name: 'Speichern' }).click();
  const typed = page.getByRole('alertdialog', { name: 'Inhalt wirklich wechseln?' });
  await expect(typed).toBeVisible();
  await expectFocusOn(page, typed.getByLabel('Tippe den Pfad, um zu bestätigen'));
  await expect(typed.getByRole('button', { name: 'Inhalt wechseln' })).toBeDisabled();
  await page.keyboard.press('Escape');
  await expect(typed).toHaveCount(0);
  await expect(change(page)).toBeVisible();
  await change(page).getByRole('button', { name: 'Speichern' }).click();
  await typed.getByLabel('Tippe den Pfad, um zu bestätigen').fill(first);
  await typed.getByRole('button', { name: 'Inhalt wechseln' }).click();
  await expect(typed).toHaveCount(0);
  await expect(change(page)).toHaveCount(0);
  await expect(row.getByText('TMDB › TVDB')).toBeVisible();

  // Back to films, so the spec can run again on the same server.
  await open.click();
  // The radio inputs are visually hidden (react-aria segments): the keyboard switches them.
  await change(page).getByRole('radio', { name: 'Serien' }).focus();
  await page.keyboard.press('ArrowLeft');
  await expect(change(page).getByRole('radio', { name: 'Filme' })).toBeChecked();
  await change(page).getByRole('button', { name: 'Speichern' }).click();
  await typed.getByLabel('Tippe den Pfad, um zu bestätigen').fill(first);
  await typed.getByRole('button', { name: 'Inhalt wechseln' }).click();
  await expect(change(page)).toHaveCount(0);
  await closeDialog(page);
  await expectNoFocusTrap(page);
});
