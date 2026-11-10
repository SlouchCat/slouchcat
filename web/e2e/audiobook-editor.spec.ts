import type { APIRequestContext, Page } from '@playwright/test';
import { audioItemId, M4B_TITLE, MP3_TITLE } from './audio';
import { CSRF } from './catalog';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// The pen and the metadata editor of an audiobook (concept 19 D5, stage D1; rule 8): the pen on a
// book's card in the library «Hörbücher» opens the editor with the tabs Metadaten, Cover, Zuordnen and
// Aktualisieren. PD-1: author, narrator, series and volume change, save and show on the detail
// page. PD-2: the cover candidates of the files (the MP3 book's cover.jpg, the M4B's embedded
// picture) are square; a choice becomes the current cover. PD-14: the pen by Tab, every tab by the
// arrow keys, Escape back to the pen. The books are the generated ones of testdata/generate-media.sh
// (no real cover goes into the repository); every change goes back after the test.

const LIBRARY = 'Hörbücher';
const BOOK_FIELDS = ['author', 'narrator', 'seriesName', 'seriesIndex'] as const;

interface BookMetadata {
  author?: string;
  narrator?: string;
  seriesName?: string;
  seriesIndex?: number;
}

const dialog = (page: Page, title: string) => page.getByRole('dialog', { name: title });
const tabs = (page: Page, title: string) =>
  dialog(page, title).getByRole('tablist', { name: 'Bereiche des Editors' });

async function libraryPath(owner: APIRequestContext): Promise<string> {
  const response = await owner.get('/api/v1/libraries/visible');
  expect(response.status(), await response.text()).toBe(200);
  const libraries = (await response.json()) as { id: number; name: string }[];
  const library = libraries.find((entry) => entry.name === LIBRARY);
  expect(library, `library "${LIBRARY}" visible`).toBeDefined();
  return `/libraries/${library!.id}`;
}

/** The book's card in the library grid and its pen. */
function bookCell(page: Page, title: string) {
  const cell = page
    .getByRole('list', { name: LIBRARY })
    .getByRole('listitem')
    .filter({ has: page.getByRole('link', { name: new RegExp(title) }) });
  return {
    card: cell.getByRole('link'),
    pen: cell.getByRole('button', { name: `${title}: Metadaten bearbeiten` }),
  };
}

/** Opens the editor of `title` from its card in the library, by keyboard. */
async function openFromLibrary(page: Page, owner: APIRequestContext, title: string) {
  await page.goto(await libraryPath(owner));
  const { card, pen } = bookCell(page, title);
  await expect(pen).toBeVisible();
  await resetFocus(page);
  await tabTo(page, card, 80);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, pen);
  await page.keyboard.press('Enter');
  await expect(dialog(page, title)).toBeVisible();
  return pen;
}

async function bookMetadata(owner: APIRequestContext, id: string): Promise<BookMetadata> {
  const response = await owner.get(`/api/v1/admin/items/${id}`);
  expect(response.status(), await response.text()).toBe(200);
  return ((await response.json()) as { metadata: BookMetadata }).metadata;
}

let m4b = '';
let mp3 = '';

test.beforeEach(async ({ owner }) => {
  m4b ||= await audioItemId(owner, M4B_TITLE);
  mp3 ||= await audioItemId(owner, MP3_TITLE);
});

test('the pen on a book card by Tab; the four tabs by the arrow keys; Escape returns @keyboard', async ({
  page,
  owner,
}) => {
  const pen = await openFromLibrary(page, owner, M4B_TITLE);
  await expect(tabs(page, M4B_TITLE).getByRole('tab')).toHaveText([
    /^Metadaten/,
    /^Cover/,
    /^Zuordnen/,
    /^Aktualisieren/,
  ]);
  const selected = tabs(page, M4B_TITLE).getByRole('tab', { selected: true });
  await expect(selected).toHaveText(/^Metadaten/);
  // The head: the link, «Schliessen», then the selected tab; the arrow keys walk the tabs.
  await tabTo(page, selected, 10);
  await expectVisibleFocus(page);
  for (const name of ['Cover', 'Zuordnen', 'Aktualisieren']) {
    await page.keyboard.press('ArrowRight');
    await expect(selected).toHaveText(new RegExp(`^${name}`));
    await expectFocusOn(page, selected);
    await expectVisibleFocus(page);
  }
  await page.keyboard.press('ArrowLeft');
  await page.keyboard.press('ArrowLeft');
  await page.keyboard.press('ArrowLeft');
  await expect(selected).toHaveText(/^Metadaten/);
  // Tab moves into the panel and stays in the dialog.
  for (let presses = 0; presses < 20; presses += 1) {
    await page.keyboard.press('Tab');
    expect(
      await dialog(page, M4B_TITLE).evaluate((element) => element.contains(document.activeElement)),
      'the focus stays in the dialog',
    ).toBe(true);
  }
  await page.keyboard.press('Escape');
  await expect(dialog(page, M4B_TITLE)).toHaveCount(0);
  await expectFocusOn(page, pen);
  await expectVisibleFocus(page);
  await expectNoFocusTrap(page, 400);
});

test('PD-1: author, narrator, series and volume change, save, lock and show on the detail page', async ({
  page,
  owner,
}) => {
  const before = await bookMetadata(owner, m4b);
  // The generated M4B carries all four fields, so the restore below writes each of them back.
  for (const name of BOOK_FIELDS) expect(before[name], name).toBeDefined();
  try {
    await openFromLibrary(page, owner, M4B_TITLE);
    const panel = dialog(page, M4B_TITLE).getByRole('tabpanel');
    // The files' values read «aus den Dateien».
    await expect(panel.getByText('aus den Dateien').first()).toBeVisible();
    const field = (name: string) => panel.getByRole('textbox', { name, exact: true });
    const values: [string, string][] = [
      ['Autor', 'E2E Autorin'],
      ['Sprecher', 'E2E Sprecher'],
      ['Reihe', 'E2E Reihe'],
      ['Band', '2,5'],
    ];
    for (const [name, value] of values) {
      await tabTo(page, field(name), 40);
      await page.keyboard.press('ControlOrMeta+a');
      await page.keyboard.type(value);
      // Typing locks the field.
      await expect(panel.getByRole('button', { name: `Sperre ${name}` })).toHaveAttribute(
        'aria-pressed',
        'true',
      );
    }
    const saved = page.waitForResponse(
      (response) =>
        response.request().method() === 'PATCH' && response.url().endsWith(`/${m4b}/metadata`),
    );
    await tabTo(page, panel.getByRole('button', { name: 'Speichern' }), 40);
    await page.keyboard.press('Enter');
    const response = await saved;
    expect(response.request().postDataJSON()).toEqual({
      author: 'E2E Autorin',
      narrator: 'E2E Sprecher',
      seriesName: 'E2E Reihe',
      seriesIndex: 2.5,
    });
    expect(response.status()).toBe(200);
    await expect(dialog(page, M4B_TITLE).getByRole('status')).toHaveText('Metadaten gespeichert.');
    // The server's values, locked by hand.
    await expect(field('Autor')).toHaveValue('E2E Autorin');
    await expect(field('Band')).toHaveValue('2.5');
    for (const [name] of values) {
      await expect(panel.getByRole('button', { name: `Sperre ${name}` })).toHaveAttribute(
        'aria-pressed',
        'true',
      );
    }
    expect(await bookMetadata(owner, m4b)).toMatchObject({
      author: 'E2E Autorin',
      narrator: 'E2E Sprecher',
      seriesName: 'E2E Reihe',
      seriesIndex: 2.5,
    });
    await page.keyboard.press('Escape');
    await expect(dialog(page, M4B_TITLE)).toHaveCount(0);

    await page.goto(`/items/${m4b}`);
    await expect(page.getByRole('heading', { level: 1, name: M4B_TITLE })).toBeVisible();
    await expect(page.getByText('von E2E Autorin')).toBeVisible();
    await expect(page.getByText('Gelesen von E2E Sprecher')).toBeVisible();
    await expect(page.getByText('E2E Reihe · Band 2,5')).toBeVisible();
  } finally {
    // The files' values again, then the locks go.
    const path = `/api/v1/admin/items/${m4b}/metadata`;
    const restore = await owner.patch(path, {
      headers: CSRF,
      data: Object.fromEntries(BOOK_FIELDS.map((name) => [name, before[name]])),
    });
    expect(restore.status(), await restore.text()).toBe(200);
    const unlock = await owner.patch(path, { headers: CSRF, data: { unlock: [...BOOK_FIELDS] } });
    expect(unlock.status(), await unlock.text()).toBe(200);
  }
});

test('PD-2: the cover candidates are square; a chosen one is the current cover', async ({
  page,
  owner,
}) => {
  try {
    await openFromLibrary(page, owner, MP3_TITLE);
    await tabs(page, MP3_TITLE).getByRole('tab', { name: 'Cover' }).click();
    const panel = dialog(page, MP3_TITLE).getByRole('tabpanel');
    const kind = panel.getByRole('button', { name: /^Cover, / });
    await expect(kind).toHaveAttribute('aria-pressed', 'true');
    const grid = panel.getByRole('listbox', { name: 'Kandidaten für Cover' });
    await expect(grid).toHaveAttribute('data-shape', 'square');
    const cover = grid.getByRole('option', { name: /^cover\.jpg/ });
    await expect(cover).toBeVisible();
    await expect(cover).toContainText('600 × 600');
    // The preview frame is square and its image loads.
    const frame = await cover.locator('span').first().boundingBox();
    expect(frame, 'laid out').not.toBeNull();
    expect(Math.abs(frame!.width - frame!.height)).toBeLessThanOrEqual(1);
    await expect
      .poll(() =>
        cover
          .locator('img')
          .evaluate((image: HTMLImageElement) => image.complete && image.naturalWidth),
      )
      .toBeGreaterThan(0);

    const chosen = page.waitForResponse(
      (response) =>
        response.request().method() === 'PUT' && response.url().endsWith(`/${mp3}/images/poster`),
    );
    await cover.focus();
    await page.keyboard.press('Enter');
    expect((await chosen).status()).toBe(200);
    await expect(dialog(page, MP3_TITLE).getByRole('status')).toHaveText(
      'Cover gewählt und gesperrt; das Bild wird geholt.',
    );
    await expect(kind).toHaveAccessibleName('Cover, gewählt · gesperrt');
    await expect(cover).toContainText('aktuell');
    await expect(panel.getByRole('button', { name: 'Zurück auf automatisch' })).toBeVisible();
  } finally {
    await owner.delete(`/api/v1/admin/items/${mp3}/images/poster`, { headers: CSRF });
  }
});
