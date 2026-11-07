import type { APIRequestContext, Locator, Page } from '@playwright/test';
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
// detail page's poster and on a library cover (always shown, on phones too) opens the dialog; Tab
// stays inside, Escape closes it and the focus returns to the pen; the tabs Bilder (upload, back to
// automatic), Metadaten (edit, lock, save) and Aktualisieren. The e2e server has no TMDB key: there
// are no candidates to choose (the choice is a Vitest case of MetadataEditor.test.tsx). The film is
// the byte copy of the H264 film, which only the maintenance spec reads (its duplicates); every
// change goes back after the test.

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
const PHONE = { width: 390, height: 844 };

/** Box of `locator`, which must be laid out. */
async function boxOf(locator: Locator) {
  const box = await locator.boundingBox();
  expect(box, 'laid out').not.toBeNull();
  return box!;
}

/** The library grid's cell of the film. */
function filmCell(page: Page) {
  return page
    .getByRole('list', { name: LIBRARY })
    .getByRole('listitem')
    .filter({
      has: page.getByRole('link').filter({ has: page.getByText(TITLE, { exact: true }) }),
    });
}

/**
 * Shows the film as watched in the library grid: its response only, the server keeps the state.
 * The context's route catches the service worker's requests too.
 */
async function showWatched(page: Page): Promise<void> {
  await page.context().route('**/api/v1/libraries/*/items?*', async (route) => {
    const response = await route.fetch();
    const body = (await response.json()) as {
      items: { title: string; progress?: Record<string, unknown> }[];
    };
    for (const item of body.items) {
      if (item.title !== TITLE) continue;
      item.progress = {
        positionMs: 8000,
        durationMs: 8000,
        percent: 100,
        completed: true,
        updatedAt: new Date().toISOString(),
      };
    }
    await route.fulfill({ response, json: body });
  });
}

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

test('the pen sits on the poster, before the actions in the Tab order @keyboard', async ({
  page,
}) => {
  await page.goto(detail);
  await expect(page.getByRole('heading', { level: 1, name: TITLE })).toBeVisible();
  // On the poster's top right, not in the row of actions.
  const poster = pen(page).locator('xpath=../..');
  const picture = await boxOf(poster.locator('xpath=./*[1]'));
  const button = await boxOf(pen(page));
  expect(button.x).toBeGreaterThanOrEqual(picture.x + picture.width / 2);
  expect(button.x + button.width).toBeLessThanOrEqual(picture.x + picture.width);
  expect(button.y).toBeGreaterThanOrEqual(picture.y);
  expect(button.y + button.height).toBeLessThanOrEqual(picture.y + picture.height / 4);
  const play = page.getByRole('link', { name: /^(Abspielen|Weiterschauen ab \d{2}:\d{2}:\d{2})$/ });
  await expect(play.locator('xpath=..').getByRole('button', { name: PEN })).toHaveCount(0);
  // Small, but a touch target.
  expect(button.width).toBeGreaterThanOrEqual(32);
  expect(button.height).toBeGreaterThanOrEqual(32);
  // Tab reaches the pen with a ring, then moves on to play.
  await resetFocus(page);
  await tabTo(page, pen(page), 40);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, play);
  await page.keyboard.press('Shift+Tab');
  await expectFocusOn(page, pen(page));
});

test('the dialog keeps its place and size on every tab', async ({ page }) => {
  await openFromDetail(page);
  const first = await boxOf(dialog(page));
  for (const name of ['Metadaten', 'Zuordnen', 'Aktualisieren']) {
    await selectTab(page, name, 1);
    expect(await boxOf(dialog(page)), name).toEqual(first);
  }
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

  // «Schliessen» closes as well. The no-trap check left the focus on the last element of the page,
  // where headless Firefox does not wrap around: back to the top, as after a page load.
  await resetFocus(page);
  await tabTo(page, pen(page), 40);
  await page.keyboard.press('Enter');
  await expect(dialog(page)).toBeVisible();
  await tabTo(page, close, 10);
  await page.keyboard.press('Enter');
  await expect(dialog(page)).toHaveCount(0);
  await expectFocusOn(page, pen(page));
});

test('unsaved changes: Escape asks first; «Weiter bearbeiten» stays, «Verwerfen» closes @keyboard', async ({
  page,
}) => {
  await openFromDetail(page);
  await selectTab(page, 'Metadaten', 1);
  const panel = dialog(page).getByRole('tabpanel');
  const overview = panel.getByRole('textbox', { name: 'Beschreibung' });
  await tabTo(page, overview, 30);
  await page.keyboard.type('E2E nicht gespeichert');
  const question = page.getByRole('alertdialog', { name: 'Ungespeicherte Änderungen verwerfen?' });
  await page.keyboard.press('Escape');
  await expect(question).toBeVisible();
  // Enter by habit keeps the draft: the focus starts on «Weiter bearbeiten».
  await expectFocusOn(page, question.getByRole('button', { name: 'Weiter bearbeiten' }));
  await page.keyboard.press('Enter');
  await expect(question).toHaveCount(0);
  await expect(dialog(page)).toBeVisible();
  await expectFocusOn(page, overview);
  await expect(overview).toHaveValue(/E2E nicht gespeichert$/);

  // «Schliessen» asks the same; «Verwerfen und schliessen» closes, the focus is back on the pen.
  await tabTo(page, dialog(page).getByRole('button', { name: 'Schliessen' }), 40);
  await page.keyboard.press('Enter');
  await expect(question).toBeVisible();
  await page.keyboard.press('Tab');
  await expectFocusOn(page, question.getByRole('button', { name: 'Verwerfen und schliessen' }));
  await page.keyboard.press('Enter');
  await expect(question).toHaveCount(0);
  await expect(dialog(page)).toHaveCount(0);
  await expectFocusOn(page, pen(page));
  await expect(page.getByText('E2E nicht gespeichert')).toHaveCount(0);
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
  const cell = filmCell(page);
  const card = cell.getByRole('link');
  const cardPen = cell.getByRole('button', { name: `${TITLE}: Metadaten bearbeiten` });
  // The pen shows without hover or focus.
  await page.mouse.move(0, 0);
  await expect(cardPen).toBeVisible();
  await expect(cardPen.locator('xpath=..')).toHaveCSS('opacity', '1');
  await resetFocus(page);
  await tabTo(page, card, 80);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, cardPen);
  await page.keyboard.press('Enter');
  await expect(dialog(page)).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(dialog(page)).toHaveCount(0);
  await expectFocusOn(page, cardPen);
  // The arrow keys pass the pen by: from it they move on from its card to the link of the card
  // beside it, never onto a pen.
  const right = await cell.evaluate((li) => li.nextElementSibling !== null);
  const neighbour = await cell.evaluate(
    (li, next) =>
      (next ? li.nextElementSibling : li.previousElementSibling)
        ?.querySelector('a')
        ?.getAttribute('href') ?? null,
    right,
  );
  expect(neighbour).not.toBeNull();
  await page.keyboard.press(right ? 'ArrowRight' : 'ArrowLeft');
  await expect(grid.locator(`a[href="${neighbour}"]`)).toBeFocused();
  await expectVisibleFocus(page);
  await expectNoFocusTrap(page, 400);
});

test('library grid: a saved title re-sorts the grid; the focus comes back to its pen @keyboard', async ({
  page,
  owner,
}) => {
  const renamed = 'ZZZ E2E Neu sortiert';
  try {
    await page.goto(`/libraries/${await libraryId(owner)}`);
    const grid = page.getByRole('list', { name: LIBRARY });
    const cellOf = (title: string) =>
      grid.getByRole('listitem').filter({
        has: page.getByRole('link').filter({ has: page.getByText(title, { exact: true }) }),
      });
    await resetFocus(page);
    await tabTo(page, cellOf(TITLE).getByRole('link'), 80);
    await page.keyboard.press('Tab');
    await page.keyboard.press('Enter');
    await expect(dialog(page)).toBeVisible();
    await selectTab(page, 'Metadaten', 1);
    const panel = dialog(page).getByRole('tabpanel');
    await tabTo(page, panel.getByRole('textbox', { name: 'Titel', exact: true }), 10);
    await page.keyboard.press('ControlOrMeta+a');
    await page.keyboard.type(renamed);
    await tabTo(page, panel.getByRole('button', { name: 'Speichern' }), 40);
    await page.keyboard.press('Enter');
    // The head keeps the title of the item row; the status says the texts are saved.
    await expect(page.getByRole('dialog').getByRole('status')).toHaveText('Metadaten gespeichert.');
    await page.keyboard.press('Escape');
    await expect(page.getByRole('dialog')).toHaveCount(0);
    // The grid reads the library again and puts the title last; the pen keeps the focus.
    await expect(grid.getByRole('listitem').last()).toContainText(renamed);
    await expectFocusOn(
      page,
      cellOf(renamed).getByRole('button', { name: `${renamed}: Metadaten bearbeiten` }),
    );
  } finally {
    const path = `/api/v1/admin/items/${id}/metadata`;
    await owner.patch(path, { headers: CSRF, data: { title: TITLE } });
    await owner.patch(path, { headers: CSRF, data: { unlock: ['title'] } });
  }
});

test.describe('on a phone', () => {
  test.use({ viewport: PHONE, hasTouch: true });

  test('the grid shows the pen beside the watched check; a tap opens the editor', async ({
    page,
    owner,
  }) => {
    await showWatched(page);
    await page.goto(`/libraries/${await libraryId(owner)}`);
    const cell = filmCell(page);
    const cardPen = cell.getByRole('button', { name: `${TITLE}: Metadaten bearbeiten` });
    await expect(cardPen).toBeVisible();
    await cardPen.scrollIntoViewIfNeeded();
    // Both on the cover: the pen at the top right, the check below it, apart.
    const check = cell.getByRole('img', { name: 'Gesehen' });
    await expect(check).toBeVisible();
    const cover = await boxOf(check.locator('xpath=..'));
    const button = await boxOf(cardPen);
    const mark = await boxOf(check);
    expect(button.width).toBeGreaterThanOrEqual(32);
    expect(button.height).toBeGreaterThanOrEqual(32);
    expect(button.x + button.width).toBeLessThanOrEqual(cover.x + cover.width);
    expect(button.y).toBeGreaterThanOrEqual(cover.y);
    expect(mark.y).toBeGreaterThanOrEqual(button.y + button.height);
    expect(mark.y + mark.height).toBeLessThanOrEqual(cover.y + cover.height);
    await cardPen.tap();
    await expect(dialog(page)).toBeVisible();
  });

  test('the detail page shows the pen; a tap opens the editor', async ({ page }) => {
    await page.goto(detail);
    await expect(page.getByRole('heading', { level: 1, name: TITLE })).toBeVisible();
    await expect(pen(page)).toBeInViewport();
    const button = await boxOf(pen(page));
    expect(button.width).toBeGreaterThanOrEqual(32);
    expect(button.x + button.width).toBeLessThanOrEqual(PHONE.width);
    await pen(page).tap();
    await expect(dialog(page)).toBeVisible();
  });
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
