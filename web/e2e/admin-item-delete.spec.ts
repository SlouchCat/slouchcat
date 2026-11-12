import { copyFileSync, existsSync, mkdirSync } from 'node:fs';
import { join } from 'node:path';
import { expect, test, type APIRequestContext, type Page } from '@playwright/test';
import { CSRF } from './catalog';
import { MEDIA_DIR, MOVIES_DIR } from './env';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// Keyboard test of «Löschen» on the detail page and of «Wartung › Entfernte Titel» (concept 20
// P-9, CLAUDE.md rule 8): the round button opens the dialog, Escape leaves it, «Nur aus Slouchcat
// entfernen» takes the film out and leads to its library, the maintenance page lists it and
// «Wieder aufnehmen» brings it back. The film is the spec's own copy of a test film in a hidden
// folder below the media root (the other libraries and the folder browser skip hidden entries);
// the file stays where it is. One press of the confirming button, by keyboard or by mouse, sends
// exactly one request. The spec deletes its library at the end.

const NAME = 'E2E Titel löschen';
const TITLE = 'E2E Entfernfilm';
const base = join(MEDIA_DIR, '.e2e', NAME);
const film = join(base, `${TITLE} (2003)`, `${TITLE} (2003).mp4`);

// The second test deletes the film the first one took out and back in.
test.describe.configure({ mode: 'serial' });

let libraryId = 0;
let itemId = '';

/** Creates the library (or finds it), waits for its scan and returns the film's id. */
async function prepare(api: APIRequestContext): Promise<string> {
  const list = (await (await api.get('/api/v1/admin/libraries')).json()) as {
    id: number;
    name: string;
  }[];
  libraryId = list.find((library) => library.name === NAME)?.id ?? 0;
  if (libraryId === 0) {
    const created = await api.post('/api/v1/admin/libraries', {
      headers: CSRF,
      data: {
        name: NAME,
        kind: 'video',
        roots: [{ path: base, content: 'movie', providers: ['tmdb'] }],
      },
    });
    expect(created.status()).toBe(201);
    libraryId = ((await created.json()) as { id: number }).id;
  }
  let id = '';
  await expect
    .poll(
      async () => {
        const response = await api.get(`/api/v1/libraries/${libraryId}/items?limit=10`);
        if (response.status() !== 200) return '';
        const { items } = (await response.json()) as { items: { id: string; title: string }[] };
        id = items.find((item) => item.title === TITLE)?.id ?? '';
        return id;
      },
      { timeout: 30_000 },
    )
    .not.toBe('');
  return id;
}

/** The delete requests of the title the page sends from now on: one per press of the button. */
function deletions(page: Page): string[] {
  const sent: string[] = [];
  page.on('request', (request) => {
    if (request.method() === 'POST' && /\/admin\/items\/[^/]+\/deletion$/.test(request.url()))
      sent.push(request.url());
  });
  return sent;
}

test.beforeAll(() => {
  mkdirSync(join(base, `${TITLE} (2003)`), { recursive: true });
  copyFileSync(
    join(MOVIES_DIR, 'Slouchcat Test H264 (2024)', 'Slouchcat Test H264 (2024) - 720p.mp4'),
    film,
  );
});

test.afterAll(async ({ request }) => {
  if (libraryId === 0) return;
  await request.post(`/api/v1/admin/libraries/${libraryId}/deletion`, {
    headers: CSRF,
    data: { confirmName: NAME },
  });
});

test('a title is taken out of Slouchcat and back in, by keyboard @keyboard', async ({ page }) => {
  itemId = await prepare(page.request);
  await page.goto(`/items/${itemId}`);
  await expect(page.getByRole('heading', { level: 1, name: TITLE })).toBeVisible();
  await resetFocus(page);

  // The round button, tabbed to; Enter opens the dialog with the file's whole path.
  const remove = page.getByRole('button', { name: 'Löschen', exact: true });
  await tabTo(page, remove, 40);
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  const dialog = page.getByRole('dialog', { name: `«${TITLE}» löschen?` });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByText(film)).toBeVisible();
  const choice = dialog.getByRole('radio', { name: 'Nur aus Slouchcat entfernen' });
  await expect(choice).toBeChecked();
  await expectFocusOn(page, choice);

  // Escape leaves everything as it was and hands the focus back to the button.
  await page.keyboard.press('Escape');
  await expect(dialog).toHaveCount(0);
  await expectFocusOn(page, remove);

  // One file: no title to type; Tab walks to the red button, Enter takes the film out.
  await page.keyboard.press('Enter');
  await expect(dialog).toBeVisible();
  const confirm = dialog.getByRole('button', { name: 'Aus Slouchcat entfernen' });
  await tabTo(page, confirm, 10);
  await expectVisibleFocus(page);
  const sent = deletions(page);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(new RegExp(`/libraries/${libraryId}$`));
  expect(sent).toHaveLength(1);
  expect((await page.request.get(`/api/v1/items/${itemId}`)).status()).toBe(404);
  expect(existsSync(film)).toBe(true);

  // «Wartung › Entfernte Titel» lists it; «Wieder aufnehmen» brings the same title back.
  await page.goto('/admin/maintenance?tab=removed');
  const table = page.getByRole('table', { name: 'Entfernte Titel' });
  await expect(table.getByText(TITLE, { exact: true })).toBeVisible();
  await resetFocus(page);
  const restore = table.getByRole('button', { name: `${TITLE} wieder aufnehmen` });
  await tabTo(page, restore, 60);
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  await expect(page.getByText(`«${TITLE}» ist wieder da (1 Titel).`)).toBeVisible();
  await expect(table.getByText(TITLE, { exact: true })).toHaveCount(0);
  await expect(page.getByRole('heading', { level: 1, name: 'Pflege' })).toBeFocused();
  await expectNoFocusTrap(page);
  expect((await page.request.get(`/api/v1/items/${itemId}`)).status()).toBe(200);
});

test('one click on the button takes the title out with one request', async ({ page }) => {
  itemId = await prepare(page.request);
  await page.goto(`/items/${itemId}`);
  await expect(page.getByRole('heading', { level: 1, name: TITLE })).toBeVisible();
  await page.getByRole('button', { name: 'Löschen', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: `«${TITLE}» löschen?` });
  await expect(dialog).toBeVisible();
  const sent = deletions(page);
  await dialog.getByRole('button', { name: 'Aus Slouchcat entfernen' }).click();
  await expect(page).toHaveURL(new RegExp(`/libraries/${libraryId}$`));
  expect(sent).toHaveLength(1);
  expect((await page.request.get(`/api/v1/items/${itemId}`)).status()).toBe(404);
  // Back in for the next test, which deletes it with its file.
  const restored = await page.request.post(`/api/v1/admin/maintenance/removed/${itemId}/restore`, {
    headers: CSRF,
  });
  expect(restored.status()).toBe(200);
});

test('the title is deleted with its file, by keyboard, in a writable folder @keyboard', async ({
  page,
}) => {
  itemId = await prepare(page.request);
  await page.goto(`/items/${itemId}`);
  await expect(page.getByRole('heading', { level: 1, name: TITLE })).toBeVisible();
  await resetFocus(page);
  await tabTo(page, page.getByRole('button', { name: 'Löschen', exact: true }), 40);
  await page.keyboard.press('Enter');
  const dialog = page.getByRole('dialog', { name: `«${TITLE}» löschen?` });
  await expect(dialog).toBeVisible();
  // The run's media folder is writable, so the second choice is open; ↓ picks it.
  const files = dialog.getByRole('radio', { name: 'Auch die Dateien löschen' });
  await expect(files).toBeEnabled();
  await page.keyboard.press('ArrowDown');
  await expect(files).toBeChecked();
  await expectVisibleFocus(page);
  await expect(dialog.getByText(/endgültig gelöscht, es gibt keinen Papierkorb/)).toBeVisible();
  const confirm = dialog.getByRole('button', { name: 'Endgültig löschen' });
  await tabTo(page, confirm, 10);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(new RegExp(`/libraries/${libraryId}$`));
  expect(existsSync(film)).toBe(false);
  expect((await page.request.get(`/api/v1/items/${itemId}`)).status()).toBe(404);
  // Gone for good: not among the removed titles.
  const removed = (await (await page.request.get('/api/v1/admin/maintenance/removed')).json()) as {
    items: { id: string }[];
  };
  expect(removed.items.some((item) => item.id === itemId)).toBe(false);
});
