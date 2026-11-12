import type { APIRequestContext, Locator, Page } from '@playwright/test';
import { audioItemId, M4B_TITLE, MP3_TITLE } from './audio';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// The book sources of audiobooks (concept 19 D3; rule 8, PD-14): the page «Hörbücher: Quellen»
// and the tab «Zuordnen» of the metadata editor, by keyboard, against the real server. The server
// asks the mock of e2e/books-server.mjs instead of the internet: it finds nothing for the test
// books' own titles (the automatic match at start writes no field), Apple Books knows the MP3 book
// without length (so it waits under «Prüfen»), and only the title «Quellentest» finds Audible
// entries. The test leaves every setting as it found it; «n Felder übernehmen» is answered here,
// so the M4B book keeps the values the other specs read.

const SEARCH_TERM = 'Quellentest';

// Chromium sends the page's requests through the service worker, where page.route does not see
// them; the match is answered here, so the worker stays out.
test.use({ serviceWorkers: 'block' });

const heading = (page: Page) => page.getByRole('heading', { level: 1, name: 'Hörbücher: Quellen' });
const saveBar = (page: Page) => page.getByRole('region', { name: 'Speichern', exact: true });

/**
 * Waits until no match of audiobooks runs and the MP3 book waits under «Prüfen»: after the
 * automatic match at start, and after «Jetzt alle …», which takes it off the list for a moment.
 */
async function waitForReview(owner: APIRequestContext): Promise<void> {
  await expect
    .poll(
      async () => {
        const state = await owner.get('/api/v1/admin/metadata/audiobooks');
        if (state.status() !== 200 || ((await state.json()) as { running: boolean }).running)
          return [];
        const response = await owner.get('/api/v1/admin/metadata/audiobooks/review');
        if (response.status() !== 200) return [];
        return ((await response.json()) as { title: string }[]).map((entry) => entry.title);
      },
      { timeout: 60_000, message: 'the MP3 book waits under «Prüfen»' },
    )
    .toContain(MP3_TITLE);
}

/** Moves the focus from the checked radio of a group with the arrow keys and back. */
async function arrowAndBack(page: Page, from: Locator, to: Locator) {
  await page.keyboard.press('ArrowRight');
  await expect(to).toBeChecked();
  await expectFocusOn(page, to);
  await expect(saveBar(page)).toContainText('Ungespeicherte Änderungen');
  await page.keyboard.press('ArrowLeft');
  await expect(from).toBeChecked();
  await expectFocusOn(page, from);
}

test('«Hörbücher: Quellen» by keyboard: folder dialog, settings, «Jetzt alle …», the pen of «Prüfen», Back @keyboard', async ({
  page,
  owner,
}) => {
  // The automatic match of the MP3 book may still run after the server's start.
  test.setTimeout(120_000);
  await waitForReview(owner);
  // In from «Verwaltung › Metadaten» by its link, so the browser's Back has somewhere to go.
  await page.goto('/admin/metadata');
  const metadata = page.getByRole('heading', { level: 1, name: 'Metadaten' });
  await expect(metadata).toBeVisible();
  await resetFocus(page);
  await tabTo(page, page.getByRole('link', { name: 'Quellen öffnen' }), 120);
  await page.keyboard.press('Enter');
  await expect(heading(page)).toBeVisible();

  // The folder's list is shown only; its button opens the folder's dialog, Escape returns.
  const folderButton = page.getByRole('button', { name: /^Quellen von .* ändern$/ }).first();
  await resetFocus(page);
  await tabTo(page, folderButton, 80);
  await page.keyboard.press('Enter');
  const folders = page.getByRole('dialog', { name: 'Ordner von Hörbücher' });
  await expect(folders).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(folders).toHaveCount(0);
  await expectFocusOn(page, folderButton);

  // The region: the arrows pick another marketplace, back again leaves nothing to save.
  const region = page.getByRole('combobox', { name: 'Region von Audible' });
  await tabTo(page, region, 10);
  const marketplace = await region.inputValue();
  await page.keyboard.press('ArrowDown');
  await expect(region).not.toHaveValue(marketplace);
  await expect(saveBar(page)).toContainText('Ungespeicherte Änderungen');
  await page.keyboard.press('ArrowUp');
  await expect(region).toHaveValue(marketplace);
  await expectFocusOn(page, region);
  await expect(saveBar(page)).not.toContainText('Ungespeicherte Änderungen');

  // Audnexus: typed over with its own value, Enter submits no form of the browser (no reload).
  const audnexus = page.getByRole('textbox', { name: 'Adresse von Audnexus' });
  await tabTo(page, audnexus, 4);
  const address = await audnexus.inputValue();
  await page.evaluate(() => {
    (window as unknown as { e2eStay?: boolean }).e2eStay = true;
  });
  await page.keyboard.press('End');
  await page.keyboard.press('Backspace');
  await page.keyboard.type(address.slice(-1));
  await page.keyboard.press('Enter');
  await expect(audnexus).toHaveValue(address);
  await expectFocusOn(page, audnexus);
  expect(
    await page.evaluate(() => (window as unknown as { e2eStay?: boolean }).e2eStay),
    'Enter kept the page (no reload)',
  ).toBe(true);
  await expect(page).toHaveURL(/\/admin\/metadata\/audiobooks$/);

  // The Google Books key lives with the provider keys since concept 20 (O-7): its link is the
  // next stop.
  await tabTo(page, page.getByRole('link', { name: 'Unter «Integrationen» einrichten' }), 4);

  // «Automatisch zuordnen» and the tolerance: one Tab stop each, the arrows pick; nothing saved.
  const all = page.getByRole('radio', { name: 'Alle ohne Zuordnung' });
  await tabTo(page, all, 20);
  await page.keyboard.press('ArrowLeft');
  const fresh = page.getByRole('radio', { name: 'Neue Hörbücher' });
  await expect(fresh).toBeChecked();
  await expectFocusOn(page, fresh);
  await page.keyboard.press('ArrowRight');
  await expect(all).toBeChecked();
  const three = page.getByRole('radio', { name: '3 %' });
  await tabTo(page, three, 4);
  await arrowAndBack(page, three, page.getByRole('radio', { name: '5 %' }));
  await expectVisibleFocus(page);
  // A switch toggles with Space.
  const authors = page.getByRole('switch', { name: 'Autorennamen vereinheitlichen' });
  await tabTo(page, authors, 4);
  await page.keyboard.press('Space');
  await expect(authors).not.toBeChecked();
  await page.keyboard.press('Space');
  await expect(authors).toBeChecked();
  await expect(saveBar(page)).not.toContainText('Ungespeicherte Änderungen');

  // «Jetzt alle ohne Zuordnung zuordnen» says how many it queued.
  const run = page.getByRole('button', { name: 'Jetzt alle ohne Zuordnung zuordnen' });
  await tabTo(page, run, 30);
  await page.keyboard.press('Enter');
  await expect(page.getByText(/^\d+ Hörb(uch|ücher) eingereiht\.$/)).toBeVisible();

  // The run takes the MP3 book off «Prüfen» for a moment; the list is read again while it runs.
  await waitForReview(owner);
  const pen = page.getByRole('button', { name: `${MP3_TITLE} bearbeiten` });
  await expect(pen).toBeVisible({ timeout: 15_000 });

  // The pen of a book under «Prüfen» opens its editor; Escape returns to the pen.
  await tabTo(page, pen, 40);
  await page.keyboard.press('Enter');
  const editor = page.getByRole('dialog', { name: MP3_TITLE });
  await expect(editor).toBeVisible();
  await expect(editor.getByText('Nicht zugeordnet: Angaben nur aus den Dateien')).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(editor).toHaveCount(0);
  await expectFocusOn(page, pen);
  await expectNoFocusTrap(page, 120);

  // The browser's Back returns to «Verwaltung › Metadaten»; Tab shows its ring there.
  await page.goBack();
  await expect(page).toHaveURL(/\/admin\/metadata$/);
  await expect(metadata).toBeVisible();
  await page.keyboard.press('Tab');
  await expectVisibleFocus(page);
});

test('«Zuordnen» of an audiobook by keyboard: search, pick, untick a field, take @keyboard', async ({
  page,
  owner,
}) => {
  const id = await audioItemId(owner, M4B_TITLE);
  const current = await owner.get(`/api/v1/admin/items/${id}`);
  expect(current.status(), await current.text()).toBe(200);
  const item: unknown = await current.json();
  // The match is answered here: the M4B keeps the values the other specs read.
  const sent: unknown[] = [];
  await page.route(`**/api/v1/admin/items/${id}/match`, async (route) => {
    if (route.request().method() !== 'PUT') return route.fallback();
    sent.push(route.request().postDataJSON());
    await route.fulfill({ status: 202, json: item });
  });

  await page.goto(`/items/${id}`);
  const pen = page.getByRole('button', { name: 'Metadaten bearbeiten', exact: true });
  await resetFocus(page);
  await tabTo(page, pen, 60);
  await page.keyboard.press('Enter');
  const editor = page.getByRole('dialog', { name: M4B_TITLE });
  await expect(editor).toBeVisible();
  const selected = editor.getByRole('tab', { selected: true });
  await tabTo(page, selected, 10);
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('ArrowRight');
  await expect(selected).toHaveText(/^Zuordnen/);
  await expectFocusOn(page, selected);

  // The source first, then the title: typed over by keyboard, «Suchen» by Enter.
  const panel = editor.getByRole('tabpanel');
  // The select waits for the configured sources of the folder.
  await expect(panel.getByLabel('Quelle', { exact: true })).toBeVisible();
  await tabTo(page, panel.getByLabel('Quelle', { exact: true }), 4);
  await expect(panel.getByLabel('Quelle', { exact: true })).toHaveValue('audible:de');
  const title = panel.getByRole('textbox', { name: 'Titel', exact: true });
  await tabTo(page, title, 3);
  await page.keyboard.press('ControlOrMeta+A');
  await page.keyboard.type(SEARCH_TERM);
  await tabTo(page, panel.getByRole('button', { name: 'Suchen' }), 6);
  await page.keyboard.press('Enter');

  // Two hits; the length of the first is the files' (1 min), which the server marks.
  const hit = panel.getByRole('button', { name: /^Quellentest – Ungekürzte Lesung wählen/ });
  await expect(hit).toBeVisible();
  await expect(hit).toHaveAccessibleName(/passt\)$/);
  await expect(panel.getByRole('button', { name: /^Quellentest – Hörspiel wählen/ })).toBeVisible();
  await expect(panel.getByText(/^Länge der Dateien: /)).toBeVisible();
  await tabTo(page, hit, 4);
  await page.keyboard.press('Enter');
  await expect(hit).toHaveAttribute('aria-pressed', 'true');

  // «Was übernommen wird»: Space unticks a field, the button counts one less.
  const table = panel.getByRole('table', { name: 'Felder von Audible, heute und neu' });
  await expect(table).toBeVisible();
  const apply = panel.getByRole('button', { name: /^\d+ Felder? übernehmen$/ });
  const before = Number((await apply.textContent())?.match(/\d+/)?.[0]);
  expect(before).toBeGreaterThan(1);
  const publisher = table.getByRole('checkbox', { name: 'Verlag übernehmen' });
  await tabTo(page, publisher, 30);
  await page.keyboard.press('Space');
  await expect(publisher).not.toBeChecked();
  await expect(apply).toHaveText(`${before - 1} Felder übernehmen`);
  await tabTo(page, apply, 30);
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  await expect.poll(() => sent.length).toBe(1);
  const body = sent[0] as { provider: string; providerId: string; fields: string[] };
  expect(body.provider).toBe('audible');
  expect(body.providerId).toBe('B0E2E00001');
  expect(body.fields).toHaveLength(before - 1);
  expect(body.fields).not.toContain('publisher');
  await expect(
    editor.getByText(`${before - 1} Felder von Audible übernommen und gesperrt.`),
  ).toBeAttached();
  // The comparison goes; the focus stays in the dialog on the picked hit.
  await expectFocusOn(page, hit);

  // Escape closes the editor, the focus goes back to the pen.
  await page.keyboard.press('Escape');
  await expect(editor).toHaveCount(0);
  await expectFocusOn(page, pen);
  await expectNoFocusTrap(page);
});
