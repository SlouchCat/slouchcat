import { copyFileSync, mkdirSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { expect, test, type APIRequestContext, type Page } from '@playwright/test';
import { MEDIA_DIR, MOVIES_DIR } from './env';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// Keyboard test of the folder list of a library and of the typed confirmation that removes one
// (CLAUDE.md rule 8, idea 02 steps 13 and 14, E16 and E17): the library's row names how many
// folders there are, its fold-out and the dialog list them, "Ordner hinzufügen" puts one in, and removing one stays
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
// «Pfad ändern» (ADR 0023 point 4): one film in two folders under the same name, the old place
// and the new one, so the scan at the new place finds the file again.
const before = join(base, 'fuenf');
const after = join(base, 'sechs');
const MOVED_TITLE = 'E2E Umzug';
const movedFile = `${MOVED_TITLE} (2001)/${MOVED_TITLE} (2001).mp4`;

/** The toggle of the library's row, its row and its fold-out (concept 12, Bib-C). */
const toggle = (page: Page) => page.getByRole('button', { name: NAME, exact: true });
const row = (page: Page) => page.getByRole('row').filter({ has: toggle(page) });
const foldOut = (page: Page) => page.getByRole('region', { name: NAME, exact: true });
const foldersButton = (page: Page) =>
  foldOut(page).getByRole('button', { name: `Ordner von ${NAME} bearbeiten` });
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
  for (const dir of [before, after]) {
    mkdirSync(join(dir, `${MOVED_TITLE} (2001)`), { recursive: true });
    copyFileSync(
      join(MOVIES_DIR, 'Slouchcat Test H264 (2024)', 'Slouchcat Test H264 (2024) - 720p.mp4'),
      join(dir, movedFile),
    );
  }
});

test.beforeEach(async ({ page }) => {
  await page.goto('/admin/libraries');
  await expect(page.getByRole('heading', { level: 1, name: 'Bibliotheken' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Filme', exact: true })).toBeVisible();

  if ((await toggle(page).count()) === 0) {
    await page.getByRole('button', { name: 'Bibliothek hinzufügen' }).click();
    const add = page.getByRole('dialog', { name: 'Bibliothek hinzufügen' });
    await add.getByLabel('Name', { exact: true }).fill(NAME);
    await add.getByLabel('Ordner', { exact: true }).fill(first);
    await add.getByRole('button', { name: 'Bibliothek mit 1 Ordner hinzufügen' }).click();
    await expect(add).toHaveCount(0);
    await expect(toggle(page)).toBeVisible();
  }
  await resetFocus(page);
});

// While a modal is open react-aria hides the rest of the page from the accessibility tree, so
// anything in the row or its fold-out is asserted with the dialog closed.
const closeDialog = async (page: Page) => {
  await dialog(page).getByRole('button', { name: 'Schliessen' }).click();
  await expect(dialog(page)).toHaveCount(0);
};

/** Folds the library's row out (the fold-out carries the button to the dialog). */
const openRow = async (page: Page) => {
  if ((await toggle(page).getAttribute('aria-expanded')) !== 'true') await toggle(page).click();
  await expect(foldOut(page)).toBeVisible();
};

const openDialog = async (page: Page) => {
  await openRow(page);
  await foldersButton(page).click();
  await expect(dialog(page)).toBeVisible();
};

test('the folders dialog opens from the keyboard and lists the folder @keyboard', async ({
  page,
}) => {
  // Tabbed to, not focused by script: a ring only counts when the keyboard put it there.
  // The page carries a row per library, so the budget is generous. Enter folds the row out;
  // its first stops are «Erneut scannen» and «Ordner».
  await tabTo(page, toggle(page), 80);
  await page.keyboard.press('Enter');
  await expect(toggle(page)).toHaveAttribute('aria-expanded', 'true');
  const open = foldersButton(page);
  await page.keyboard.press('Tab');
  await page.keyboard.press('Tab');
  await expectFocusOn(page, open);
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

  // The row counts instead of naming a path (E16); the fold-out lists both.
  await closeDialog(page);
  await expect(row(page).getByText('2 Ordner')).toBeVisible();
  await expect(foldOut(page).getByText(second)).toBeVisible();
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
  await expect(row(page).getByText(first)).toBeVisible();
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

  await expect(row(page).getByText('0 Ordner')).toBeVisible();
  await expect(foldOut(page).getByText(/Kein Ordner: Es wird nichts gescannt/)).toBeVisible();
  await expect(foldOut(page).getByRole('button', { name: 'Erneut scannen' })).toBeDisabled();

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
  await expect(row(page).getByText('3 Ordner')).toBeVisible();
  await expect(row(page).getByText('2 Filme, 1 Serien')).toBeVisible();
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
  await expect(row(page).getByText(first)).toBeVisible();
});

test('«Ändern» edits content and sources by keyboard; a content change needs the typed path', async ({
  page,
}) => {
  await openDialog(page);
  const open = dialog(page).getByRole('button', { name: `Ordner ${first} ändern` });
  /** Opens «Ändern» by keyboard; the dialog starts on the checked content (its first field). */
  const openChange = async () => {
    if (!(await open.evaluate((element) => element === document.activeElement)))
      await tabTo(page, open, 20);
    await page.keyboard.press('Enter');
    await expect(change(page)).toBeVisible();
    await expectFocusOn(page, change(page).getByRole('radio', { checked: true }));
  };
  await openChange();

  // The fillers only: ticking AniList needs no confirmation, the dialog says what happens.
  const sources = change(page).getByRole('group', { name: 'Metadatenquellen' });
  await expect(change(page).getByRole('radio', { name: 'Filme' })).toBeChecked();
  await tabTo(page, sources.getByRole('checkbox', { name: 'AniList' }), 12);
  await expectVisibleFocus(page);
  await page.keyboard.press('Space');
  await expect(change(page).getByText(/Nur die Auffüller ändern sich/)).toBeVisible();
  // The order is a position per source (S-16): ↑ puts AniList first, where it may not stand.
  const rank = sources.getByRole('combobox', { name: 'Platz für AniList' });
  await page.keyboard.press('Tab');
  await expectFocusOn(page, rank);
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, rank);
  await expect(sources.getByRole('alert')).toHaveText(/AniList kann keinen Titel erkennen/);
  await expect(change(page).getByRole('button', { name: 'Speichern' })).toBeDisabled();
  await page.keyboard.press('ArrowDown');
  await expect(sources.getByRole('alert')).toHaveCount(0);
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
  await openChange();
  await expect(change(page).getByRole('radio', { name: 'Filme' })).toBeFocused();
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
  await openChange();
  // The radio inputs are visually hidden (react-aria segments): the keyboard switches them.
  await expect(change(page).getByRole('radio', { name: 'Serien' })).toBeFocused();
  await page.keyboard.press('ArrowLeft');
  await expect(change(page).getByRole('radio', { name: 'Filme' })).toBeChecked();
  await change(page).getByRole('button', { name: 'Speichern' }).click();
  await typed.getByLabel('Tippe den Pfad, um zu bestätigen').fill(first);
  await typed.getByRole('button', { name: 'Inhalt wechseln' }).click();
  await expect(change(page)).toHaveCount(0);
  await closeDialog(page);
  await expectNoFocusTrap(page);
});

/** The library of this spec as the administration lists it. */
async function ownLibrary(api: APIRequestContext) {
  const response = await api.get('/api/v1/admin/libraries');
  expect(response.status()).toBe(200);
  const libraries = (await response.json()) as {
    id: number;
    name: string;
    scan: { state: string; finishedAt?: string };
  }[];
  const library = libraries.find((candidate) => candidate.name === NAME);
  expect(library, NAME).toBeDefined();
  return library!;
}

/** Waits until a scan of the library finished after `since`, and returns the id of the film. */
async function movedTitleAfterScan(api: APIRequestContext, since: number): Promise<string> {
  let libraryId = 0;
  await expect
    .poll(
      async () => {
        const library = await ownLibrary(api);
        libraryId = library.id;
        const finished = library.scan.finishedAt ? Date.parse(library.scan.finishedAt) : 0;
        return library.scan.state === 'done' && finished >= since;
      },
      { timeout: 30_000 },
    )
    .toBe(true);
  const response = await api.get(`/api/v1/libraries/${libraryId}/items?limit=100`);
  expect(response.status()).toBe(200);
  const { items } = (await response.json()) as { items: { id: string; title: string }[] };
  const item = items.find((candidate) => candidate.title === MOVED_TITLE);
  expect(item, MOVED_TITLE).toBeDefined();
  return item!.id;
}

test('«Pfad ändern» picks the new place by keyboard and keeps the title (ADR 0023)', async ({
  page,
}) => {
  const browser = page.getByRole('dialog', { name: 'Ordner auswählen' });
  const confirmMove = page.getByRole('alertdialog', { name: 'Pfad ändern?' });
  const moveButton = (path: string) =>
    dialog(page).getByRole('button', { name: `Pfad von ${path} ändern` });

  await openDialog(page);
  let since = Date.now();
  await addFolders(page, [before]);
  const titleId = await movedTitleAfterScan(page.request, since);

  /**
   * Opens the browser from the row by keyboard and types the new place into its path field: the
   * folders of this spec lie below a hidden folder, which the list does not show, and a library
   * folder opens as a root of its own (PathBrowserDialog.startPosition).
   */
  const pickNewPlace = async () => {
    const open = moveButton(before);
    if (!(await open.evaluate((element) => element === document.activeElement)))
      await tabTo(page, open, 30);
    await expectVisibleFocus(page);
    await page.keyboard.press('Enter');
    await expect(browser).toBeVisible();
    // The browser opens in the folder's own path, the focus on its first row.
    const field = browser.getByLabel('Ordner', { exact: true });
    await expect(field).toHaveValue(before);
    const film = browser.getByRole('option', { name: new RegExp(MOVED_TITLE) });
    await expectFocusOn(page, film);
    await expectVisibleFocus(page);
    await tabTo(page, field, 10);
    await page.keyboard.press('ControlOrMeta+a');
    await page.keyboard.type(after);
    await tabTo(page, browser.getByRole('button', { name: 'OK', exact: true }), 10);
    await page.keyboard.press('Enter');
    await expect(browser).toHaveCount(0);
    await expect(confirmMove).toBeVisible();
    await expect(
      confirmMove.getByText(`Alter Pfad: ${before} · Neuer Pfad: ${after}.`, { exact: false }),
    ).toBeVisible();
    await expectFocusOn(page, confirmMove.getByRole('button', { name: 'Ändern' }));
    await expectVisibleFocus(page);
  };

  // Escape leaves the folder where it was and hands the focus back to its «Pfad ändern».
  let patched = 0;
  page.on('request', (request) => {
    if (request.method() === 'PATCH' && /\/roots\/\d+$/.test(request.url())) patched += 1;
  });
  await pickNewPlace();
  await page.keyboard.press('Escape');
  await expect(confirmMove).toHaveCount(0);
  await expectFocusOn(page, moveButton(before));
  await expect(dialog(page).getByText(before, { exact: true })).toBeVisible();
  expect(patched).toBe(0);

  // Enter confirms: the server takes the new path, the list and the status say so.
  await pickNewPlace();
  const saved = page.waitForResponse(
    (response) => response.request().method() === 'PATCH' && /\/roots\/\d+$/.test(response.url()),
  );
  since = Date.now();
  await page.keyboard.press('Enter');
  const answer = await saved;
  expect(answer.status()).toBe(200);
  expect(answer.request().postDataJSON()).toEqual({ path: after });
  await expect(confirmMove).toHaveCount(0);
  await expect(
    dialog(page).getByText(`Der Ordner liegt jetzt unter ${after} und wird neu eingelesen.`),
  ).toBeVisible();
  await expect(dialog(page).getByText(after, { exact: true })).toBeVisible();
  await expect(dialog(page).getByText(before, { exact: true })).toHaveCount(0);
  await expectFocusOn(page, moveButton(after));

  // The scan at the new place finds the file under the same name: the title keeps its id.
  expect(await movedTitleAfterScan(page.request, since)).toBe(titleId);

  // Out again, so the spec can run again on the same server.
  await dialog(page)
    .getByRole('button', { name: `Ordner ${after} entfernen` })
    .click();
  await confirm(page).getByLabel('Tippe den Pfad, um zu bestätigen').fill(after);
  await confirm(page).getByRole('button', { name: 'Ordner entfernen' }).click();
  await expect(confirm(page)).toHaveCount(0);
  await closeDialog(page);
  await expectNoFocusTrap(page);
});
