import type { APIRequestContext, Page } from '@playwright/test';
import { createProfile, loginOwner } from './accounts';
import { CSRF, H264_COPY_TITLE, idOf, itemPath } from './catalog';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// The metadata editor with the pen (concept 15 7.4, variant A «Dialog»; rule 8): the pen on the
// detail page and beside a library cover opens the dialog; Tab stays inside, Escape closes it and
// the focus returns to the pen; the tabs Bilder (upload, back to automatic), Metadaten (edit, lock,
// save) and Aktualisieren. The e2e server has no TMDB key: there are no candidates to choose (the
// choice is a Vitest case of MetadataEditor.test.tsx). The film is the byte copy of the H264 film,
// which only the maintenance spec reads (its duplicates); every change goes back after the test.

const TITLE = H264_COPY_TITLE;
const LIBRARY = 'Filme';
const PEN = 'Metadaten bearbeiten';

/** A 20 × 30 PNG: the smallest image the server takes as a poster. */
const PNG = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAABQAAAAeCAIAAACjcKk8AAAAHklEQVR4nGNocFAgGzGMah7VPKp5VPOo5lHNw1kzAM0hDR92V8fBAAAAAElFTkSuQmCC',
  'base64',
);

let detail = '';
let id = '';

const dialog = (page: Page) => page.getByRole('dialog', { name: TITLE });
const tabs = (page: Page) => dialog(page).getByRole('tablist', { name: 'Bereiche des Editors' });
const pen = (page: Page) => page.getByRole('button', { name: PEN, exact: true });

async function libraryId(owner: APIRequestContext): Promise<number> {
  const response = await owner.get('/api/v1/libraries/visible');
  expect(response.status(), await response.text()).toBe(200);
  const libraries = (await response.json()) as { id: number; name: string }[];
  const library = libraries.find((entry) => entry.name === LIBRARY);
  expect(library, `library "${LIBRARY}" visible`).toBeDefined();
  return library!.id;
}

/** With the arrow keys from the selected tab to `name`. */
async function selectTab(page: Page, name: string, steps: number): Promise<void> {
  const selected = tabs(page).getByRole('tab', { selected: true });
  await selected.focus();
  for (let step = 0; step < steps; step += 1) await page.keyboard.press('ArrowRight');
  await expect(tabs(page).getByRole('tab', { selected: true })).toHaveText(new RegExp(`^${name}`));
}

/** Opens the editor with the pen of the detail page, by keyboard. */
async function openFromDetail(page: Page): Promise<void> {
  await page.goto(detail);
  await expect(page.getByRole('heading', { level: 1, name: TITLE })).toBeVisible();
  await resetFocus(page);
  await tabTo(page, pen(page), 40);
  await page.keyboard.press('Enter');
  await expect(dialog(page)).toBeVisible();
  await expect(tabs(page).getByRole('tab', { selected: true })).toHaveText(/^Bilder/);
}

test.beforeEach(async ({ page }) => {
  if (!id) {
    detail = await itemPath(page, TITLE);
    id = idOf(detail);
  }
});

test.afterEach(async ({ owner }) => {
  // The texts and the poster go back to the providers (none in the e2e server).
  const path = `/api/v1/admin/items/${id}`;
  await owner.patch(`${path}/metadata`, {
    headers: CSRF,
    data: { overview: '', tagline: '' },
  });
  await owner.patch(`${path}/metadata`, {
    headers: CSRF,
    data: { unlock: ['overview', 'tagline'] },
  });
  await owner.delete(`${path}/images/poster`, { headers: CSRF });
});

test('the pen opens the dialog; Tab stays inside, Escape closes, focus back on the pen @keyboard', async ({
  page,
}) => {
  await openFromDetail(page);
  await expect(
    dialog(page).getByRole('link', { name: 'In der Verwaltung öffnen' }),
  ).toHaveAttribute('href', `/admin/items/${encodeURIComponent(id)}`);
  // The tabs in the order of the concept.
  await expect(tabs(page).getByRole('tab')).toHaveText([
    /^Bilder/,
    /^Metadaten/,
    /^Zuordnen/,
    /^Aktualisieren/,
  ]);
  // Tab order of the head: the link, «Schliessen», then the selected tab; the focus never leaves.
  const link = dialog(page).getByRole('link', { name: 'In der Verwaltung öffnen' });
  const close = dialog(page).getByRole('button', { name: 'Schliessen' });
  await tabTo(page, link, 10);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, close);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, tabs(page).getByRole('tab', { selected: true }));
  for (let presses = 0; presses < 30; presses += 1) {
    await page.keyboard.press('Tab');
    expect(
      await dialog(page).evaluate((element) => element.contains(document.activeElement)),
      'the focus stays in the dialog',
    ).toBe(true);
  }
  await page.keyboard.press('Escape');
  await expect(dialog(page)).toHaveCount(0);
  // The detail page stays (Escape belongs to the dialog), the focus is back on the pen.
  await expect(page).toHaveURL(detail);
  await expectFocusOn(page, pen(page));
  await expectNoFocusTrap(page);

  // «Schliessen» closes as well.
  await page.keyboard.press('Shift+Tab');
  await tabTo(page, pen(page), 40);
  await page.keyboard.press('Enter');
  await expect(dialog(page)).toBeVisible();
  await tabTo(page, close, 10);
  await page.keyboard.press('Enter');
  await expect(dialog(page)).toHaveCount(0);
  await expectFocusOn(page, pen(page));
});

test('Metadaten: a change locks the field, «Speichern» writes it, the page shows it', async ({
  page,
}) => {
  await openFromDetail(page);
  await selectTab(page, 'Metadaten', 1);
  const panel = dialog(page).getByRole('tabpanel');
  await expect(panel.getByText(/Texte gelten für die Sprache Deutsch/)).toBeVisible();
  const overview = panel.getByRole('textbox', { name: 'Beschreibung' });
  const lock = panel.getByRole('button', { name: 'Sperre Beschreibung' });
  await expect(lock).toHaveAttribute('aria-pressed', 'false');
  await tabTo(page, overview, 30);
  await page.keyboard.type('E2E Beschreibung vom Editor');
  // Typing locks the field; the tab shows the unsaved changes.
  await expect(lock).toHaveAttribute('aria-pressed', 'true');
  await expect(tabs(page).getByRole('tab', { name: 'Metadaten ungespeichert' })).toBeVisible();
  const saved = page.waitForResponse(
    (response) =>
      response.request().method() === 'PATCH' && response.url().endsWith(`/${id}/metadata`),
  );
  await tabTo(page, panel.getByRole('button', { name: 'Speichern' }), 40);
  await page.keyboard.press('Enter');
  expect((await saved).request().postDataJSON()).toEqual({
    overview: 'E2E Beschreibung vom Editor',
  });
  await expect(dialog(page).getByRole('status')).toHaveText('Metadaten gespeichert.');
  await expect(lock).toHaveAttribute('aria-pressed', 'true');
  await expect(panel.getByText('von Hand').first()).toBeVisible();

  // The lock lifts the lock: «Speichern» sends only that.
  await lock.focus();
  await expectVisibleFocus(page);
  await page.keyboard.press('Space');
  await expect(lock).toHaveAttribute('aria-pressed', 'false');
  const unlocked = page.waitForResponse(
    (response) =>
      response.request().method() === 'PATCH' && response.url().endsWith(`/${id}/metadata`),
  );
  await panel.getByRole('button', { name: 'Speichern' }).focus();
  await page.keyboard.press('Enter');
  expect((await unlocked).request().postDataJSON()).toEqual({ unlock: ['overview'] });
  await expect(lock).toHaveAttribute('aria-pressed', 'false');

  // Closing reads the title again: the detail page shows the new overview at once, and after a
  // reload too.
  await page.keyboard.press('Escape');
  await expect(dialog(page)).toHaveCount(0);
  await expect(page.getByText('E2E Beschreibung vom Editor')).toBeVisible();
  await page.reload();
  await expect(page.getByText('E2E Beschreibung vom Editor')).toBeVisible();
});

test('Bilder: upload an own poster, the kind is locked; back to automatic', async ({ page }) => {
  await openFromDetail(page);
  const panel = dialog(page).getByRole('tabpanel');
  const poster = panel.getByRole('button', { name: /^Poster, / });
  await expect(poster).toHaveAttribute('aria-pressed', 'true');
  await expect(poster).toHaveAccessibleName('Poster, fehlt');

  // A file that is no image: the server's code, in words.
  const upload = panel.locator('input[type="file"]');
  await upload.setInputFiles({
    name: 'kaputt.png',
    mimeType: 'image/png',
    buffer: Buffer.from('nope'),
  });
  await expect(
    panel.getByText('Das ist kein JPEG-, PNG- oder WebP-Bild, oder die Datei ist beschädigt.'),
  ).toBeVisible();

  const uploaded = page.waitForResponse(
    (response) =>
      response.request().method() === 'PUT' &&
      response.url().endsWith(`/${id}/images/poster/upload`),
  );
  await upload.setInputFiles({ name: 'poster.png', mimeType: 'image/png', buffer: PNG });
  expect((await uploaded).status()).toBe(200);
  await expect(dialog(page).getByRole('status')).toHaveText(
    'Poster: eigenes Bild hochgeladen und gesperrt.',
  );
  await expect(poster).toHaveAccessibleName('Poster, gewählt · gesperrt');
  await expect(panel.getByRole('heading', { name: 'Aktuell: eigenes Bild' })).toBeVisible();
  // The tile and the mini poster of the head (the first picture of the dialog) show the upload,
  // served by the administration.
  const shown = [
    panel.getByRole('region', { name: 'Aktuell: eigenes Bild' }).locator('img'),
    dialog(page).locator('img').first(),
  ];
  for (const img of shown) {
    await expect(img).toHaveAttribute(
      'src',
      new RegExp(`/api/v1/admin/items/${id}/images/poster/w400\\?v=[0-9a-f]{64}$`),
    );
    await expect
      .poll(() =>
        img.evaluate((element: HTMLImageElement) => element.complete && element.naturalWidth),
      )
      .toBeGreaterThan(0);
  }

  const automatic = panel.getByRole('button', { name: 'Zurück auf automatisch' });
  await tabTo(page, automatic, 20);
  await page.keyboard.press('Enter');
  await expect(dialog(page).getByRole('status')).toHaveText(/^Sperre für Poster aufgehoben/);
  await expect(automatic).toHaveCount(0);
  // The upload goes with the lock; without a provider (none in the e2e server) the kind is empty.
  await expect(poster).toHaveAccessibleName('Poster, fehlt');
  await expect(panel.getByRole('heading', { name: 'Aktuell: kein Bild' })).toBeVisible();
  await expect(panel.locator('img')).toHaveCount(0);
});

test('Aktualisieren: «Jetzt aktualisieren» with «Vorher alle Sperren aufheben»', async ({
  page,
}) => {
  await openFromDetail(page);
  await selectTab(page, 'Aktualisieren', 3);
  const panel = dialog(page).getByRole('tabpanel');
  const box = panel.getByRole('checkbox', { name: 'Vorher alle Sperren dieses Titels aufheben' });
  await tabTo(page, box, 10);
  await page.keyboard.press('Space');
  await expect(box).toBeChecked();
  const refreshed = page.waitForResponse(
    (response) =>
      response.request().method() === 'POST' && response.url().endsWith(`/${id}/refresh`),
  );
  await tabTo(page, panel.getByRole('button', { name: 'Jetzt aktualisieren' }), 5);
  await page.keyboard.press('Enter');
  const response = await refreshed;
  expect(response.status()).toBe(202);
  expect(response.request().postDataJSON()).toEqual({ unlockAll: true });
  await expect(dialog(page).getByRole('status')).toHaveText('Aktualisierung eingereiht.');
  await expect(box).not.toBeChecked();
});

test('library grid: Tab reaches the card, then its pen; Enter opens, Escape returns @keyboard', async ({
  page,
  owner,
}) => {
  await page.goto(`/libraries/${await libraryId(owner)}`);
  const grid = page.getByRole('list', { name: LIBRARY });
  const cell = grid.getByRole('listitem').filter({
    has: page.getByRole('link').filter({ has: page.getByText(TITLE, { exact: true }) }),
  });
  const card = cell.getByRole('link');
  const cardPen = cell.getByRole('button', { name: `${TITLE}: Metadaten bearbeiten` });
  await resetFocus(page);
  await tabTo(page, card, 80);
  // The pen shows only on hover or focus (its frame over the cover fades in).
  const frame = cardPen.locator('xpath=..');
  await expect(frame).toHaveCSS('opacity', '0');
  await page.keyboard.press('Tab');
  await expectFocusOn(page, cardPen);
  await expect(frame).toHaveCSS('opacity', '1');
  // The arrow keys pass it by: from the pen they move on from its card, never onto a pen.
  await page.keyboard.press('Enter');
  await expect(dialog(page)).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(dialog(page)).toHaveCount(0);
  await expectFocusOn(page, cardPen);
  await card.focus();
  await page.keyboard.press('ArrowRight');
  const active = await page.evaluate(() => document.activeElement?.tagName.toLowerCase());
  expect(active).toBe('a');
});

test('a profile other than the primary one sees no pen', async ({ page, owner }) => {
  const other = await createProfile(owner, { name: 'E2E Ohne Stift', maxRating: 18 });
  // A session of its own for the page: the owner's shared one keeps its primary profile.
  await loginOwner(page.request);
  const selected = await page.request.post(`/api/v1/auth/profiles/${other.id}/select`, {
    headers: CSRF,
  });
  expect(selected.status(), await selected.text()).toBe(200);
  await page.goto(detail);
  await expect(page.getByRole('heading', { level: 1, name: TITLE })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Zurück' })).toBeVisible();
  await expect(pen(page)).toHaveCount(0);
  await page.goto(`/libraries/${await libraryId(owner)}`);
  await expect(page.getByRole('list', { name: LIBRARY }).getByRole('link').first()).toBeVisible();
  await expect(page.getByRole('button', { name: /Metadaten bearbeiten$/ })).toHaveCount(0);
});
