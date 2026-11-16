import { copyFileSync, mkdirSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { expect, test, type APIRequestContext, type Locator, type Page } from '@playwright/test';
import { MEDIA_DIR, MOVIES_DIR } from './env';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// Keyboard test of the form «Bibliothek bearbeiten» (CLAUDE.md rule 8; concept 24 4.1 to 4.3):
// it opens from the library's fold-out, every folder is a closed card whose head opens and closes
// with Enter and Space, a new card is open with the focus in its path, «Ordner entfernen …» only
// marks the card and hands the focus to «Rückgängig», and «Speichern» saves everything in one go
// after «Änderungen bestätigen», where a removal and another content are confirmed with the typed
// path and a new path names old and new. Escape with changes asks «Änderungen verwerfen?».
//
// It works on a library of its own, in a hidden folder below the media root of the run: hidden
// entries are skipped by the scanner and by the folder browser, so the other specs never see it.
// It stays for the rest of the run, like "E2E leer".

const NAME = 'E2E Ordner';
const base = join(MEDIA_DIR, '.e2e', NAME);
const first = join(base, 'eins');
const second = join(base, 'zwei');
const third = join(base, 'drei');
const fourth = join(base, 'vier');
// A new path (ADR 0023 point 4): one film in two folders under the same name, the old place and
// the new one, so the scan at the new place finds the file again.
const before = join(base, 'fuenf');
const after = join(base, 'sechs');
const MOVED_TITLE = 'E2E Umzug';
const movedFile = `${MOVED_TITLE} (2001)/${MOVED_TITLE} (2001).mp4`;

/** The toggle of the library's row, its row and its fold-out (concept 12, Bib-C). */
const toggle = (page: Page) => page.getByRole('button', { name: NAME, exact: true });
const row = (page: Page) => page.getByRole('row').filter({ has: toggle(page) });
const foldOut = (page: Page) => page.getByRole('region', { name: NAME, exact: true });
const editButton = (page: Page) =>
  foldOut(page).getByRole('button', { name: `${NAME} bearbeiten` });
const form = (page: Page) => page.getByRole('dialog', { name: 'Bibliothek bearbeiten' });
const confirm = (page: Page) => page.getByRole('dialog', { name: 'Änderungen bestätigen' });
const discard = (page: Page) => page.getByRole('alertdialog', { name: 'Änderungen verwerfen?' });
const card = (page: Page, number: number) =>
  form(page).getByRole('listitem', { name: `Ordner ${number}` });
/** Every folder card of the form (the cards hold lists of their own). */
const cards = (page: Page) => form(page).getByRole('listitem', { name: /^Ordner \d+$/ });
/** The head of a card: the first button, the one that folds it. */
const head = (scope: Locator) => scope.getByRole('button').first();
const pathOf = (scope: Locator) => scope.getByLabel('Ordner', { exact: true });
const save = (page: Page) => form(page).getByRole('button', { name: 'Speichern', exact: true });
const typed = (page: Page) => confirm(page).getByLabel('Tippe den Pfad, um zu bestätigen');

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

/** Folds the library's row out (the fold-out carries «Bearbeiten»). */
const openRow = async (page: Page) => {
  if ((await toggle(page).getAttribute('aria-expanded')) !== 'true') await toggle(page).click();
  await expect(foldOut(page)).toBeVisible();
};

const openForm = async (page: Page) => {
  await openRow(page);
  await editButton(page).click();
  await expect(form(page)).toBeVisible();
  await expectFocusOn(page, form(page).getByLabel('Name', { exact: true }));
};

/** «Speichern» by keyboard; with heavy changes the confirmation comes first. */
const pressSave = async (page: Page) => {
  await tabTo(page, save(page), 80);
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
};

/** The confirmation is there; Tab reaches its «Speichern», Enter saves, the form closes. */
const confirmSave = async (page: Page) => {
  const submit = confirm(page).getByRole('button', { name: 'Speichern', exact: true });
  await expect(submit).toBeEnabled();
  await tabTo(page, submit, 20);
  await page.keyboard.press('Enter');
  await expect(confirm(page)).toHaveCount(0);
  await expect(form(page)).toHaveCount(0);
};

/** Removes the folder of `path` through the form, confirmed with its path (cleanup). */
const removeFolder = async (page: Page, path: string) => {
  await openForm(page);
  const target = cards(page).filter({ hasText: path });
  await head(target).click();
  await target.getByRole('button', { name: /^Ordner \d+ entfernen$/ }).click();
  await save(page).click();
  await typed(page).fill(path);
  await confirm(page).getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(form(page)).toHaveCount(0);
};

/**
 * Adds folders through the form; a new folder needs no confirmation. A new card of a video
 * library proposes the content the others lack; `films` keeps every new card at films.
 */
const addFolders = async (page: Page, paths: string[], films = false) => {
  await openForm(page);
  for (const path of paths) {
    // «Ordner hinzufügen» while the library has none, «Weiteren …» after.
    await form(page)
      .getByRole('button', { name: /^(Weiteren )?Ordner hinzufügen$/ })
      .click();
    const last = cards(page).last();
    await expectFocusOn(page, pathOf(last));
    await page.keyboard.type(path);
    if (films) await last.getByText('Filme', { exact: true }).click();
  }
  await save(page).click();
  await expect(form(page)).toHaveCount(0);
};

test('the form opens by keyboard, its cards fold with Enter and Space, Escape asks before discarding @keyboard', async ({
  page,
}) => {
  // Tabbed to, not focused by script: a ring only counts when the keyboard put it there. Enter
  // folds the row out; its stops are «Erneut scannen» and «Bearbeiten».
  await tabTo(page, toggle(page), 80);
  await page.keyboard.press('Enter');
  await expect(toggle(page)).toHaveAttribute('aria-expanded', 'true');
  await page.keyboard.press('Tab');
  await page.keyboard.press('Tab');
  await expectFocusOn(page, editButton(page));
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  await expect(form(page)).toBeVisible();

  // The name first; the kind is locked (no stop); then the head of the one closed card.
  const name = form(page).getByLabel('Name', { exact: true });
  await expectFocusOn(page, name);
  await expect(form(page).getByRole('radio', { name: 'Video' })).toBeDisabled();
  const one = card(page, 1);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, head(one));
  await expectVisibleFocus(page);
  await expect(head(one)).toHaveAttribute('aria-expanded', 'false');
  await expect(head(one)).toContainText(first);
  // Enter opens it, Space closes it; the focus stays on the head (concept 24 P-3).
  await page.keyboard.press('Enter');
  await expect(head(one)).toHaveAttribute('aria-expanded', 'true');
  await expect(pathOf(one)).toHaveValue(first);
  await expectFocusOn(page, head(one));
  await page.keyboard.press('Space');
  await expect(head(one)).toHaveAttribute('aria-expanded', 'false');
  await expect(pathOf(one)).toBeHidden();
  await expectFocusOn(page, head(one));

  // Without a change Escape closes at once and hands the focus back to «Bearbeiten».
  await page.keyboard.press('Escape');
  await expect(form(page)).toHaveCount(0);
  await expectFocusOn(page, editButton(page));

  // With a change it asks; the safe answer has the focus and keeps what was typed.
  await page.keyboard.press('Enter');
  await expectFocusOn(page, name);
  await page.keyboard.press('End');
  await page.keyboard.type(' neu');
  await page.keyboard.press('Escape');
  await expect(discard(page)).toBeVisible();
  const keep = discard(page).getByRole('button', { name: 'Weiter bearbeiten' });
  await expectFocusOn(page, keep);
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  await expect(discard(page)).toHaveCount(0);
  await expect(name).toHaveValue(`${NAME} neu`);
  // «Abbrechen» asks the same; «Verwerfen» closes, nothing is saved.
  await tabTo(page, form(page).getByRole('button', { name: 'Abbrechen' }), 40);
  await page.keyboard.press('Enter');
  await expectFocusOn(page, keep);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, discard(page).getByRole('button', { name: 'Verwerfen' }));
  await page.keyboard.press('Enter');
  await expect(form(page)).toHaveCount(0);
  await expectFocusOn(page, editButton(page));
  await expect(toggle(page)).toBeVisible();
  await expectNoFocusTrap(page);
});

test('a second folder goes in with «Speichern», and comes out only after its path is typed @keyboard', async ({
  page,
}) => {
  await openForm(page);
  // Add: the new card is open and takes the focus in its path; Enter in it saves the form.
  await tabTo(page, form(page).getByRole('button', { name: 'Weiteren Ordner hinzufügen' }), 40);
  await page.keyboard.press('Enter');
  const two = card(page, 2);
  await expect(head(two)).toHaveAttribute('aria-expanded', 'true');
  await expectFocusOn(page, pathOf(two));
  await page.keyboard.type(second);
  await expect(two.getByText('geändert')).toBeVisible();
  await page.keyboard.press('Enter');
  // A new folder needs no confirmation; one band says what changed.
  await expect(form(page)).toHaveCount(0);
  await expect(page.getByText('1 Ordner hinzugefügt; der Scan ist eingereiht.')).toBeVisible();
  await expect(row(page).getByText('2 Ordner')).toBeVisible();
  await expect(foldOut(page).getByText(second)).toBeVisible();

  // Remove: only a mark, with «Rückgängig»; the focus follows.
  await openForm(page);
  await tabTo(page, head(card(page, 2)), 20);
  await page.keyboard.press('Enter');
  const remove = card(page, 2).getByRole('button', { name: 'Ordner 2 entfernen' });
  await tabTo(page, remove, 40);
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  const undo = card(page, 2).getByRole('button', {
    name: 'Entfernen von Ordner 2 rückgängig machen',
  });
  await expectFocusOn(page, undo);
  await expect(card(page, 2)).toContainText('Wird beim Speichern entfernt');
  await page.keyboard.press('Enter');
  await expectFocusOn(page, head(card(page, 2)));
  await page.keyboard.press('Enter');
  await tabTo(page, remove, 40);
  await page.keyboard.press('Enter');
  await expectFocusOn(page, undo);

  // «Speichern» asks first: the consequences and the typed path.
  await pressSave(page);
  await expect(confirm(page)).toBeVisible();
  await expect(confirm(page).getByText(`Ordner entfernen: ${second}`)).toBeVisible();
  await expect(confirm(page).getByText(/weich gelöscht/)).toBeVisible();
  await tabTo(page, typed(page), 20);
  await expectVisibleFocus(page);
  const submit = confirm(page).getByRole('button', { name: 'Speichern', exact: true });
  await expect(submit).toBeDisabled();
  // A near miss is still a miss; surrounding spaces are forgiven.
  await page.keyboard.type(first);
  await expect(submit).toBeDisabled();
  await page.keyboard.press('ControlOrMeta+a');
  await page.keyboard.type(`${second}x`);
  await expect(submit).toBeDisabled();
  await page.keyboard.press('Backspace');
  await page.keyboard.type(' ');
  await expect(submit).toBeEnabled();
  // Escape leaves the confirmation; the form and the mark stay.
  await page.keyboard.press('Escape');
  await expect(confirm(page)).toHaveCount(0);
  await expect(card(page, 2)).toContainText('Wird beim Speichern entfernt');

  // Now for real.
  await pressSave(page);
  await confirmSave(page);
  await expect(page.getByText('1 Ordner entfernt.')).toBeVisible();
  await expect(row(page).getByText(first)).toBeVisible();
  await expectNoFocusTrap(page);
});

test('a library without a folder says so and offers no scan (E13)', async ({ page }) => {
  await openForm(page);
  await head(card(page, 1)).click();
  await card(page, 1).getByRole('button', { name: 'Ordner 1 entfernen' }).click();
  await save(page).click();
  // The text names the last folder and the one-way door.
  await expect(confirm(page).getByText(/Es ist der letzte Ordner/)).toBeVisible();
  await expect(confirm(page).getByText(/SLOUCHCAT_BROWSE_ROOTS/)).toBeVisible();
  await typed(page).fill(first);
  await confirm(page).getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(form(page)).toHaveCount(0);

  await expect(row(page).getByText('0 Ordner')).toBeVisible();
  await expect(
    foldOut(page).getByText(
      'Kein Ordner: Es wird nichts gescannt, bis du unter «Bearbeiten» einen hinzufügst.',
    ),
  ).toBeVisible();
  await expect(foldOut(page).getByRole('button', { name: 'Erneut scannen' })).toBeDisabled();

  // Put it back, so the spec can run again on the same server.
  await addFolders(page, [first]);
  await expect(row(page).getByText(first)).toBeVisible();
});

test('two folders go in with one save, each with its own content (F-13)', async ({ page }) => {
  await addFolders(page, [third, fourth]);
  // The second new card proposed the other content of a video library: a series folder.
  await expect(row(page).getByText('3 Ordner')).toBeVisible();
  await expect(row(page).getByText('2 Filme, 1 Serien')).toBeVisible();
  // Out again in one save, so the other tests find the one folder they expect.
  await openForm(page);
  for (const path of [third, fourth]) {
    const target = cards(page).filter({ hasText: path });
    await head(target).click();
    await target.getByRole('button', { name: /^Ordner \d+ entfernen$/ }).click();
  }
  await save(page).click();
  await expect(confirm(page).getByLabel('Tippe den Pfad, um zu bestätigen')).toHaveCount(2);
  await typed(page).first().fill(third);
  await typed(page).last().fill(fourth);
  await confirm(page).getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(form(page)).toHaveCount(0);
  await expect(page.getByText('2 Ordner entfernt.')).toBeVisible();
  await expect(row(page).getByText(first)).toBeVisible();
});

test('content and sources of a card by keyboard; another content needs the typed path @keyboard', async ({
  page,
}) => {
  await openForm(page);
  const one = card(page, 1);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, head(one));
  await page.keyboard.press('Enter');

  // The fillers only: ticking AniList needs no confirmation, the card says what happens.
  const sources = one.getByRole('group', { name: 'Metadatenquellen' });
  await expect(one.getByRole('radio', { name: 'Filme' })).toBeChecked();
  await tabTo(page, sources.getByRole('checkbox', { name: 'AniList' }), 20);
  await expectVisibleFocus(page);
  await page.keyboard.press('Space');
  await expect(one.getByText(/Nur die Auffüller ändern sich/)).toBeVisible();
  // The order is a position per source (S-16): ↑ puts AniList first, where it may not stand.
  const rank = sources.getByRole('combobox', { name: 'Platz für AniList' });
  await page.keyboard.press('Tab');
  await expectFocusOn(page, rank);
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, rank);
  await expect(sources.getByRole('alert')).toHaveText(/AniList kann keinen Titel erkennen/);
  await expect(save(page)).toBeDisabled();
  await page.keyboard.press('ArrowDown');
  await expect(sources.getByRole('alert')).toHaveCount(0);
  await pressSave(page);
  await expect(form(page)).toHaveCount(0);
  await expect(page.getByText(/^Neue Quellen bei 1 Ordner/)).toBeVisible();
  await expect(foldOut(page).getByText('TMDB › AniList')).toBeVisible();

  // The content: the card warns; the confirmation wants the typed path, Escape leaves it.
  await openForm(page);
  await page.keyboard.press('Tab');
  await page.keyboard.press('Enter');
  const films = one.getByRole('radio', { name: 'Filme' });
  await tabTo(page, films, 20);
  await page.keyboard.press('ArrowRight');
  await expect(one.getByRole('radio', { name: 'Serien' })).toBeChecked();
  await expect(one.getByText(/wirkt wie Ordner entfernen und neu anlegen/)).toBeVisible();
  await pressSave(page);
  await expect(confirm(page).getByText(`Inhalt wechseln: ${first}`)).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(confirm(page)).toHaveCount(0);
  await expect(form(page)).toBeVisible();
  await pressSave(page);
  await tabTo(page, typed(page), 20);
  await page.keyboard.type(first);
  await confirmSave(page);
  await expect(foldOut(page).getByText('TMDB › TVDB')).toBeVisible();

  // Back to films with the old sources, so the spec can run again on the same server.
  await openForm(page);
  await head(one).click();
  await expect(one.getByRole('radio', { name: 'Serien' })).toBeChecked();
  await one.getByRole('radio', { name: 'Serien' }).focus();
  await page.keyboard.press('ArrowLeft');
  await expect(one.getByRole('radio', { name: 'Filme' })).toBeChecked();
  await save(page).click();
  await typed(page).fill(first);
  await confirm(page).getByRole('button', { name: 'Speichern', exact: true }).click();
  await expect(form(page)).toHaveCount(0);
  await expect(foldOut(page).getByText('TMDB', { exact: true })).toBeVisible();
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

/**
 * Waits until a scan of the library finished after `since` and the film is listed, and returns
 * its id. An earlier scan of the library (another test changed a folder) may end after `since`
 * too, before the one that reads the film, so the poll waits for the film itself.
 */
async function movedTitleAfterScan(api: APIRequestContext, since: number): Promise<string> {
  let id = '';
  await expect
    .poll(
      async () => {
        const library = await ownLibrary(api);
        const finished = library.scan.finishedAt ? Date.parse(library.scan.finishedAt) : 0;
        if (library.scan.state !== 'done' || finished < since) return '';
        const response = await api.get(`/api/v1/libraries/${library.id}/items?limit=100`);
        if (response.status() !== 200) return '';
        const { items } = (await response.json()) as { items: { id: string; title: string }[] };
        id = items.find((candidate) => candidate.title === MOVED_TITLE)?.id ?? '';
        return id;
      },
      { timeout: 30_000 },
    )
    .not.toBe('');
  return id;
}

test('a new path by keyboard names old and new path, needs no typed path and keeps the title (ADR 0023)', async ({
  page,
}) => {
  let since = Date.now();
  await addFolders(page, [before], true);
  const titleId = await movedTitleAfterScan(page.request, since);

  await openForm(page);
  // By its number: its head names the new path once it is typed.
  const label = await cards(page).filter({ hasText: before }).getAttribute('aria-label');
  const moved = form(page).getByRole('listitem', { name: label ?? '', exact: true });
  await tabTo(page, head(moved), 20);
  await page.keyboard.press('Enter');
  await tabTo(page, pathOf(moved), 10);
  await page.keyboard.press('ControlOrMeta+a');
  await page.keyboard.type(after);
  await expect(moved.getByText(/Der neue Pfad gilt nach dem Speichern/)).toBeVisible();

  // Escape in the confirmation leaves the folder where it was; nothing is sent.
  let sent = 0;
  page.on('request', (request) => {
    if (request.method() === 'PUT' && /\/admin\/libraries\/\d+$/.test(request.url())) sent += 1;
  });
  await pressSave(page);
  await expect(confirm(page).getByText(`Pfad ändern: ${before}`)).toBeVisible();
  await expect(
    confirm(page).getByText(`Alter Pfad: ${before} · Neuer Pfad: ${after}.`, { exact: false }),
  ).toBeVisible();
  await expect(typed(page)).toHaveCount(0);
  await page.keyboard.press('Escape');
  await expect(confirm(page)).toHaveCount(0);
  expect(sent).toBe(0);

  // Enter confirms: the server takes the new path, the band says so.
  await pressSave(page);
  const saved = page.waitForResponse(
    (response) =>
      response.request().method() === 'PUT' && /\/admin\/libraries\/\d+$/.test(response.url()),
  );
  since = Date.now();
  await confirmSave(page);
  const answer = await saved;
  expect(answer.status()).toBe(200);
  expect(answer.request().postDataJSON()).toMatchObject({ roots: [{}, { path: after }] });
  await expect(page.getByText('1 Ordner mit neuem Pfad wird neu eingelesen.')).toBeVisible();
  await expect(foldOut(page).getByText(after, { exact: true })).toBeVisible();
  await expect(foldOut(page).getByText(before, { exact: true })).toHaveCount(0);

  // The scan at the new place finds the file under the same name: the title keeps its id.
  expect(await movedTitleAfterScan(page.request, since)).toBe(titleId);

  // Out again, so the spec can run again on the same server.
  await removeFolder(page, after);
  await expect(foldOut(page).getByText(after, { exact: true })).toHaveCount(0);
  await expectNoFocusTrap(page);
});
