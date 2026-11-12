import { copyFileSync, existsSync, mkdirSync } from 'node:fs';
import { join } from 'node:path';
import { expect, test, type Page } from '@playwright/test';
import { MEDIA_DIR, MOVIES_DIR } from './env';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// Keyboard test of «Bibliothek löschen …» (concept 20 P-8, CLAUDE.md rule 8): the entry in the
// menu of the library's fold-out, the typed confirmation with the name, Escape, and the delete
// itself. The library is the spec's own, on a copy of a test film in a hidden folder below the
// media root of the run (the scanner of the other libraries and the folder browser skip hidden
// entries). The delete never touches a file: the copy is still there afterwards. The server
// answers 202 and deletes in the job `library.delete` (decision of 2026-11-12); one press of the
// button, by keyboard or by mouse, sends exactly one request.

const NAME = 'E2E Löschen';
const base = join(MEDIA_DIR, '.e2e', NAME);
const film = join(base, 'E2E Loeschfilm (2002)', 'E2E Loeschfilm (2002).mp4');

const toggle = (page: Page) => page.getByRole('button', { name: NAME, exact: true });
const foldOut = (page: Page) => page.getByRole('region', { name: NAME, exact: true });
const more = (page: Page) =>
  foldOut(page).getByRole('button', { name: `Weitere Aktionen für ${NAME}` });
const confirm = (page: Page) =>
  page.getByRole('alertdialog', { name: `Bibliothek «${NAME}» löschen?` });
const queued = (page: Page) =>
  page.getByText(`Bibliothek «${NAME}» wird im Hintergrund gelöscht.`, { exact: false });
// The job may wait behind the jobs of the scan the spec's library started.
const done = (page: Page) =>
  expect(page.getByText(`Bibliothek «${NAME}» gelöscht.`, { exact: true })).toBeVisible({
    timeout: 30_000,
  });

/** The delete requests the page sends from now on. */
function deletions(page: Page): string[] {
  const sent: string[] = [];
  page.on('request', (request) => {
    if (request.method() === 'POST' && /\/admin\/libraries\/\d+\/deletion$/.test(request.url()))
      sent.push(request.url());
  });
  return sent;
}

test.beforeAll(() => {
  mkdirSync(join(base, 'E2E Loeschfilm (2002)'), { recursive: true });
  copyFileSync(
    join(MOVIES_DIR, 'Slouchcat Test H264 (2024)', 'Slouchcat Test H264 (2024) - 720p.mp4'),
    film,
  );
});

test.beforeEach(async ({ page }) => {
  await page.goto('/admin/libraries');
  await expect(page.getByRole('heading', { level: 1, name: 'Bibliotheken' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Filme', exact: true })).toBeVisible();
  if ((await toggle(page).count()) === 0) {
    await page.getByRole('button', { name: 'Bibliothek hinzufügen' }).click();
    const add = page.getByRole('dialog', { name: 'Bibliothek hinzufügen' });
    await add.getByLabel('Name', { exact: true }).fill(NAME);
    await add.getByLabel('Ordner', { exact: true }).fill(base);
    await add.getByRole('button', { name: 'Bibliothek mit 1 Ordner hinzufügen' }).click();
    await expect(add).toHaveCount(0);
    await expect(toggle(page)).toBeVisible();
  }
  await resetFocus(page);
});

test('a library is deleted from the keyboard after its name is typed @keyboard', async ({
  page,
}) => {
  // Tabbed to, not focused by script: Enter folds the row out, the menu sits in the fold-out.
  await tabTo(page, toggle(page), 80);
  await page.keyboard.press('Enter');
  await expect(toggle(page)).toHaveAttribute('aria-expanded', 'true');
  await tabTo(page, more(page), 10);
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  const entry = page.getByRole('menuitem', { name: 'Bibliothek löschen …' });
  await expect(entry).toBeVisible();
  // The entry is the last one; arrows reach it, Enter opens the confirmation.
  await page.keyboard.press('End');
  await expectFocusOn(page, entry);
  await page.keyboard.press('Enter');

  await expect(confirm(page)).toBeVisible();
  await expect(
    confirm(page).getByText(/Die Dateien auf der Platte bleiben unberührt/),
  ).toBeVisible();
  await expect(
    confirm(page).getByText(/setzt alles auf den Stand vor dem Löschen zurück/),
  ).toBeVisible();
  const typed = confirm(page).getByLabel('Zur Bestätigung den Namen eintippen');
  const remove = confirm(page).getByRole('button', { name: 'Bibliothek löschen' });
  // The focus starts in the field, with a ring; the button stays refused for a near miss.
  await expectFocusOn(page, typed);
  await expectVisibleFocus(page);
  await page.keyboard.type(NAME.toLowerCase());
  await expect(remove).toBeDisabled();

  // Escape leaves everything as it was and hands the focus back to the menu button.
  await page.keyboard.press('Escape');
  await expect(confirm(page)).toHaveCount(0);
  await expect(toggle(page)).toBeVisible();
  await expectFocusOn(page, more(page));

  // Now for real: the field starts empty again, Tab walks to the button, Enter deletes.
  await page.keyboard.press('Enter');
  await page.keyboard.press('End');
  await page.keyboard.press('Enter');
  await expect(confirm(page)).toBeVisible();
  await expect(typed).toHaveValue('');
  await page.keyboard.type(NAME);
  await expect(remove).toBeEnabled();
  await tabTo(page, remove, 5);
  await expectVisibleFocus(page);
  const sent = deletions(page);
  await page.keyboard.press('Enter');

  // The 202 closes the dialog at once; the row goes and the status says what happens.
  await expect(confirm(page)).toHaveCount(0);
  await expect(queued(page)).toBeVisible();
  await expect(page.getByRole('link', { name: 'Zum Job' })).toHaveAttribute(
    'href',
    '/admin/jobs?item=library.delete',
  );
  await expect(toggle(page)).toHaveCount(0);
  // The row went with the focus: the h1 has it (ALL-2), and the page holds no trap.
  await expect(page.getByRole('heading', { level: 1, name: 'Bibliotheken' })).toBeFocused();
  await expectNoFocusTrap(page);
  // The job ends, the page says so; one Enter sent one request.
  await done(page);
  expect(sent).toHaveLength(1);
  // Not a single file went.
  expect(existsSync(film)).toBe(true);
});

test('one click on the button deletes the library with one request', async ({ page }) => {
  await toggle(page).click();
  await more(page).click();
  await page.getByRole('menuitem', { name: 'Bibliothek löschen …' }).click();
  await expect(confirm(page)).toBeVisible();
  await confirm(page).getByLabel('Zur Bestätigung den Namen eintippen').fill(NAME);
  const sent = deletions(page);
  await confirm(page).getByRole('button', { name: 'Bibliothek löschen' }).click();

  await expect(confirm(page)).toHaveCount(0);
  await expect(queued(page)).toBeVisible();
  await expect(toggle(page)).toHaveCount(0);
  await done(page);
  expect(sent).toHaveLength(1);
  expect(existsSync(film)).toBe(true);
});
