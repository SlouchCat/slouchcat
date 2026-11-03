import type { APIRequestContext, Locator, Page } from '@playwright/test';
import { CSRF } from './catalog';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, resetFocus, tabTo } from './keyboard';

// The page "Metadaten" (concept 10, 7.1, ADR 0018; layout concept 12 Met-B) against the real
// server: the scope select in the header, the list of settings areas (MasterDetail), the server
// default with its language places, the sticky save bar, a library's deviation, the rating
// country, the apply run and the question before leaving with unsaved changes. Every test leaves
// the settings as it found them (the suite shares one server).

interface ImageLangs {
  poster: string[];
  backdrop: string[];
  logo: string[];
}

interface Settings {
  textLangs: string[];
  imageLangs: ImageLangs;
  imageKinds: string[];
  imageMinWidth: { poster: number; backdrop: number };
  imageByVote: boolean;
  localImages: boolean;
  ratingCountry: string;
  tmdbIncludeAdult: boolean;
  fanartEnabled: boolean;
  knownLanguages: string[];
  affectedTitles: number;
  localImageTitles: number;
}

interface LibraryMetadata {
  libraryId: number;
  textLangs: string[] | null;
  images: { langs: ImageLangs; kinds: string[] } | null;
}

async function settingsOf(api: APIRequestContext): Promise<Settings> {
  const response = await api.get('/api/v1/admin/settings/metadata');
  expect(response.status(), await response.text()).toBe(200);
  return (await response.json()) as Settings;
}

/** The writable part of the settings, as `PUT` takes it. */
function updateOf(s: Settings) {
  return {
    textLangs: s.textLangs,
    imageLangs: s.imageLangs,
    imageKinds: s.imageKinds,
    imageMinWidth: s.imageMinWidth,
    imageByVote: s.imageByVote,
    localImages: s.localImages,
    ratingCountry: s.ratingCountry,
    tmdbIncludeAdult: s.tmdbIncludeAdult,
    fanartEnabled: s.fanartEnabled,
  };
}

async function restore(api: APIRequestContext, before: Settings): Promise<void> {
  const response = await api.put('/api/v1/admin/settings/metadata', {
    headers: CSRF,
    data: updateOf(before),
  });
  expect(response.status(), await response.text()).toBe(200);
}

async function libraryId(api: APIRequestContext, name: string): Promise<number> {
  const response = await api.get('/api/v1/admin/libraries');
  expect(response.status()).toBe(200);
  const libraries = (await response.json()) as { id: number; name: string }[];
  const library = libraries.find((entry) => entry.name === name);
  if (!library) throw new Error(`no library ${name}`);
  return library.id;
}

async function libraryMetadataOf(api: APIRequestContext, id: number): Promise<LibraryMetadata> {
  const response = await api.get(`/api/v1/admin/libraries/${id}/metadata`);
  expect(response.status(), await response.text()).toBe(200);
  return (await response.json()) as LibraryMetadata;
}

const region = (page: Page, name: string) => page.getByRole('region', { name, exact: true });
const saveBar = (page: Page) => region(page, 'Speichern');
const areas = (page: Page) => page.getByRole('listbox', { name: 'Bereiche' });
const scopeSelect = (page: Page) => page.getByLabel('Geltung', { exact: true });

/** The select of place `n` (1-based) of a language list. */
const place = (list: Locator, n: number) =>
  list.getByRole('combobox', { name: `${n}. Sprache`, exact: true });

/** The values of the places of a language list, place 1 first ("" = Ohne Text). */
const placesOf = (list: Locator) =>
  list
    .getByRole('combobox')
    .evaluateAll((selects) => selects.map((select) => (select as HTMLSelectElement).value));

async function openMetadata(page: Page, query = '') {
  await page.goto(`/admin/metadata${query}`);
  await expect(page.getByRole('heading', { level: 1, name: 'Metadaten' })).toBeVisible();
  await expect(areas(page)).toBeVisible();
}

/** Moves the focus to `target` with Tab from the top of the page (P-15: no `focus()` by script). */
async function tabToFromTop(page: Page, target: Locator) {
  await resetFocus(page);
  await tabTo(page, target, 200);
}

/**
 * Moves the focus inside a single-Tab-stop list (the admin sidebar, the areas of MasterDetail):
 * Tab to its current entry, then ↓ until `target` has it.
 */
async function arrowTo(
  page: Page,
  entry: Locator,
  target: Locator,
  key: 'ArrowDown' | 'ArrowUp' = 'ArrowDown',
  max = 40,
) {
  await tabToFromTop(page, entry);
  for (let presses = 0; presses < max; presses += 1) {
    if (await target.evaluate((element) => element === document.activeElement)) return;
    await page.keyboard.press(key);
  }
  await expectFocusOn(page, target);
}

/**
 * Picks `value` in the focused closed select with ↑ or ↓ only, one step per enabled option (the
 * browsers skip disabled ones); returns the number of steps.
 */
async function pickByKeys(page: Page, select: Locator, value: string): Promise<number> {
  await expect(select).toBeFocused();
  const { from, to } = await select.evaluate((element, wanted) => {
    const field = element as HTMLSelectElement;
    const open = [...field.options]
      .filter((option) => !option.disabled || option.selected)
      .map((option) => option.value);
    return { from: open.indexOf(field.value), to: open.indexOf(wanted) };
  }, value);
  expect(to, `${value} is an enabled choice`).toBeGreaterThanOrEqual(0);
  const steps = Math.abs(to - from);
  for (let step = 0; step < steps; step += 1)
    await page.keyboard.press(to > from ? 'ArrowDown' : 'ArrowUp');
  await expect(select).toHaveValue(value);
  return steps;
}

/** Picks the scope in the header select by keyboard; a changed scope carries «(geändert)». */
async function chooseScope(page: Page, name: string) {
  const select = scopeSelect(page);
  const value = await select
    .getByRole('option')
    .filter({ hasText: new RegExp(`^${name}( \\(geändert\\))?$`) })
    .getAttribute('value');
  await tabToFromTop(page, select);
  await pickByKeys(page, select, value ?? '');
}

/** The list with `next` on place `index` (0-based); a language on another place swaps (10.2). */
function chooseAt(list: readonly string[], index: number, next: string): string[] {
  const out = [...list];
  const other = list.indexOf(next);
  if (other >= 0 && other !== index) out[other] = list[index]!;
  out[index] = next;
  return out;
}

test('the sticky save bar does not cover the focused element (WCAG 2.4.11)', async ({ page }) => {
  // Low enough that the page scrolls and the bar sticks to the bottom of the viewport.
  await page.setViewportSize({ width: 1280, height: 560 });
  await openMetadata(page);
  // The last two focusable elements above the bar: the links to the libraries and the jobs.
  const before = page.getByRole('link', {
    name: 'Die Reihenfolge der Quellen steht je Ordner unter Bibliotheken.',
  });
  const last = page.getByRole('link', { name: 'Die Zeitpläne stehen unter Hintergrundjobs.' });
  await tabToFromTop(page, before);
  // The page scrolled so the jobs link stands at the very bottom of the viewport, where the sticky
  // bar lies over it. Tab to it: the browser must bring it out from under the bar.
  await page.evaluate(() => {
    const link = [...document.querySelectorAll('a')].find((a) =>
      a.textContent?.includes('Hintergrundjobs.'),
    );
    if (!link) throw new Error('no jobs link');
    window.scrollBy(0, link.getBoundingClientRect().bottom - window.innerHeight + 4);
  });
  const covered = await saveBar(page).boundingBox();
  const under = await last.boundingBox();
  expect(under!.y + under!.height, 'the link starts out under the bar').toBeGreaterThan(covered!.y);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, last);
  const bar = await saveBar(page).boundingBox();
  const focused = await last.boundingBox();
  expect(focused!.y + focused!.height).toBeLessThanOrEqual(bar!.y);
});

test('the default scope lists the areas and the languages, the save bar is clean, nothing is sent', async ({
  page,
  owner,
}) => {
  const before = await settingsOf(owner);
  // The overview first, for the browser back at the end. Both pages must have rendered before
  // going back (Firefox, see admin-libraries.spec).
  await page.goto('/admin');
  await expect(page.getByRole('heading', { level: 1, name: 'Verwaltung' })).toBeVisible();
  await openMetadata(page);
  await expect(
    page.getByRole('navigation', { name: 'Verwaltung' }).getByRole('link', { name: 'Metadaten' }),
  ).toHaveAttribute('aria-current', 'page');
  await expect(scopeSelect(page)).toHaveValue('server');
  await expect(areas(page).getByRole('option')).toHaveCount(6);
  const languages = region(page, 'Sprachen');
  const list = languages.getByRole('group', { name: 'Metadaten-Sprachen' });
  await expect(list.getByRole('listitem')).toHaveCount(before.textLangs.length);
  // Fixed places, each a select of the languages (10.2).
  await expect(place(list, 1)).toHaveValue('de');
  // The default needs Deutsch (MET-2): its «×» is off.
  await expect(list.getByRole('button', { name: '1. Sprache (Deutsch) entfernen' })).toBeDisabled();
  await expect(saveBar(page).getByText('Gespeichert', { exact: true })).toBeVisible();
  // Nothing changed: «Speichern» stays usable and «Verwerfen» is not there (decision 18).
  await expect(saveBar(page).getByRole('button', { name: 'Speichern' })).toBeEnabled();
  await expect(saveBar(page).getByRole('button', { name: 'Verwerfen' })).toHaveCount(0);

  // The list is one Tab stop: ↓ moves, Enter selects; the detail follows.
  await tabToFromTop(page, areas(page).getByRole('option', { name: 'Sprachen' }));
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, areas(page).getByRole('option', { name: 'Bilder' }));
  await page.keyboard.press('Enter');
  await expect(region(page, 'Bilder').getByText('immer')).toHaveCount(2);
  await expect(page).toHaveURL(/[?&]item=images/);
  for (const [option, title] of [
    ['Bildqualität', 'Bildqualität'],
    ['Quellen', 'Quellen'],
    ['Altersfreigabe', 'Altersfreigabe'],
    ['Anwenden', 'Auf bestehende Titel anwenden'],
  ] as const) {
    await page.keyboard.press('ArrowDown');
    await expectFocusOn(page, areas(page).getByRole('option', { name: option }));
    await page.keyboard.press('Enter');
    await expect(region(page, title)).toBeVisible();
  }
  await expect(saveBar(page).getByText('Gespeichert', { exact: true })).toBeVisible();
  await expectNoFocusTrap(page, 120);

  // Browser back returns to the overview; the areas replaced the address, they add no step.
  await page.goBack();
  await expect(page).toHaveURL(/\/admin$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Verwaltung' })).toBeVisible();
});

test('a language is added, put on its place with several arrow steps, saved and removed, all by keyboard @keyboard', async ({
  page,
  owner,
}) => {
  const before = await settingsOf(owner);
  try {
    await openMetadata(page);
    const languages = region(page, 'Sprachen');
    const list = languages.getByRole('group', { name: 'Metadaten-Sprachen' });
    const count = before.textLangs.length;
    expect(before.textLangs).not.toContain('ja');

    // ↓ on the closed select of place 1 picks the next allowed language (a swap), ↑ picks
    // Deutsch again: the list is as before, and the select keeps the focus.
    const first = place(list, 1);
    await tabToFromTop(page, first);
    await page.keyboard.press('ArrowDown');
    await expect(first).not.toHaveValue('de');
    await expect(first).toBeFocused();
    await page.keyboard.press('ArrowUp');
    await expect(first).toHaveValue('de');
    await expectFocusOn(page, first);
    expect(await placesOf(list)).toEqual(before.textLangs);
    await expect(saveBar(page).getByText('Gespeichert', { exact: true })).toBeVisible();

    // «Weitere Sprache» appends a place and focuses it; Japanisch goes there by the arrows.
    const more = list.getByRole('button', { name: 'Weitere Sprache' });
    await tabToFromTop(page, more);
    await page.keyboard.press('Enter');
    await expect(list.getByRole('listitem')).toHaveCount(count + 1);
    await expectFocusOn(page, place(list, count + 1));
    await pickByKeys(page, place(list, count + 1), 'ja');
    await expect(saveBar(page).getByText('Ungespeicherte Änderungen')).toBeVisible();
    const base = await placesOf(list);

    // Japanisch onto place 1 by several ↓ steps: each step counts from the list as it was when
    // the select got the focus, so the end is the direct pick (no chain of swaps).
    await tabToFromTop(page, first);
    const steps = await pickByKeys(page, first, 'ja');
    expect(steps).toBeGreaterThan(1);
    const expected = chooseAt(base, 0, 'ja');
    expect(await placesOf(list)).toEqual(expected);
    await expectFocusOn(page, first);
    await expect(list.getByText('Japanisch und Deutsch haben die Plätze getauscht.')).toBeVisible();

    const save = saveBar(page).getByRole('button', { name: 'Speichern' });
    await tabToFromTop(page, save);
    const stored = page.waitForResponse(
      (response) =>
        response.request().method() === 'PUT' &&
        response.url().endsWith('/admin/settings/metadata'),
    );
    await page.keyboard.press('Enter');
    expect((await stored).status()).toBe(200);
    await expect(
      page.getByText(/^Gespeichert für Standard\. \d+ Titel betroffen – jetzt anwenden\?$/),
    ).toBeVisible();
    await expect(saveBar(page).getByText('Gespeichert', { exact: true })).toBeVisible();
    expect((await settingsOf(owner)).textLangs).toEqual(expected);

    // Deutsch back to place 1, then Japanisch out: the list is as before.
    await tabToFromTop(page, first);
    await pickByKeys(page, first, 'de');
    expect(await placesOf(list)).toEqual(base);
    const remove = list.getByRole('button', {
      name: `${count + 1}. Sprache (Japanisch) entfernen`,
    });
    await tabToFromTop(page, remove);
    await page.keyboard.press('Enter');
    await expect(list.getByRole('listitem')).toHaveCount(count);
    await tabToFromTop(page, save);
    await page.keyboard.press('Enter');
    await expect(saveBar(page).getByText('Gespeichert', { exact: true })).toBeVisible();
    expect((await settingsOf(owner)).textLangs).toEqual(before.textLangs);
    await expectNoFocusTrap(page, 160);
  } finally {
    await restore(owner, before);
  }
});

test('the rating country changes with the arrow keys; "Verwerfen" asks and drops the change unsaved', async ({
  page,
  owner,
}) => {
  const before = await settingsOf(owner);
  await openMetadata(page, '?item=rating');
  const rating = region(page, 'Altersfreigabe');
  const group = rating.getByRole('radiogroup', { name: 'Land' });
  const current = group.getByRole('radio', { checked: true });
  await expect(current).toHaveAccessibleName(new RegExp(`\\(${before.ratingCountry}\\)`));
  await tabToFromTop(page, current);
  await page.keyboard.press(before.ratingCountry === 'US' ? 'ArrowLeft' : 'ArrowRight');
  await expect(group.getByRole('radio', { checked: true })).not.toHaveAccessibleName(
    new RegExp(`\\(${before.ratingCountry}\\)`),
  );
  await expect(saveBar(page).getByText('Ungespeicherte Änderungen')).toBeVisible();
  await expect(areas(page).getByRole('option', { name: 'Altersfreigabe' })).toContainText(
    'Geändert',
  );
  const discard = saveBar(page).getByRole('button', { name: 'Verwerfen' });
  await tabToFromTop(page, discard);
  await page.keyboard.press('Enter');
  // The question names the changed scope; the focus starts on «Abbrechen».
  const dialog = page.getByRole('alertdialog', { name: 'Änderungen verwerfen?' });
  await expect(dialog).toContainText('Die ungespeicherten Änderungen an Standard gehen verloren.');
  await expectFocusOn(page, dialog.getByRole('button', { name: 'Abbrechen' }));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, dialog.getByRole('button', { name: 'Verwerfen' }));
  await page.keyboard.press('Enter');
  await expect(dialog).toBeHidden();
  await expect(saveBar(page).getByText('Gespeichert', { exact: true })).toBeVisible();
  // «Verwerfen» went with the changes: the focus waits on the page head (ALL-2).
  await expect(page.getByRole('heading', { level: 1, name: 'Metadaten' })).toBeFocused();
  await expect(group.getByRole('radio', { checked: true })).toHaveAccessibleName(
    new RegExp(`\\(${before.ratingCountry}\\)`),
  );
  expect((await settingsOf(owner)).ratingCountry).toBe(before.ratingCountry);
});

test('a library deviates in its languages; the libraries page says so', async ({ page, owner }) => {
  const id = await libraryId(owner, 'Dokus');
  const before = await libraryMetadataOf(owner, id);
  try {
    await openMetadata(page);
    await chooseScope(page, 'Dokus');
    await expect(page.getByText(/Dokus übernimmt den Standard/)).toBeVisible();
    await expect(page).toHaveURL(new RegExp(`[?&]library=${id}`));
    // A library has only the two areas it can deviate in.
    await expect(areas(page).getByRole('option')).toHaveCount(2);
    const languages = region(page, 'Sprachen');
    await expect(languages.getByText('wie Server')).toBeVisible();
    const list = languages.getByRole('group', { name: 'Metadaten-Sprachen' });
    // Read-only: disabled places, no «×».
    await expect(place(list, 2)).toBeDisabled();
    await expect(list.getByRole('button', { name: /^2\. Sprache .* entfernen$/ })).toHaveCount(0);

    // Space switches the deviation on; the places become editable, "wie Server" goes.
    const deviate = page.getByRole('switch', { name: 'Sprachen abweichend' });
    await tabToFromTop(page, deviate);
    await page.keyboard.press('Space');
    await expect(deviate).toBeChecked();
    await expect(languages.getByText('wie Server')).toHaveCount(0);
    await expect(place(list, 2)).toHaveValue('en');
    // A library needs Deutsch or Englisch: with both there, Englisch may go.
    const remove = list.getByRole('button', { name: '2. Sprache (Englisch) entfernen' });
    await expect(remove).toBeEnabled();
    await tabToFromTop(page, remove);
    await page.keyboard.press('Enter');
    await expect(list.getByRole('listitem')).toHaveCount(1);
    // The focus moves to the remaining place.
    await expectFocusOn(page, place(list, 1));
    // Deutsch is the last place now: it cannot go (E-9).
    await expect(
      list.getByRole('button', { name: '1. Sprache (Deutsch) entfernen' }),
    ).toBeDisabled();

    const save = saveBar(page).getByRole('button', { name: 'Speichern' });
    await tabToFromTop(page, save);
    const stored = page.waitForResponse(
      (response) =>
        response.request().method() === 'PUT' &&
        response.url().endsWith(`/admin/libraries/${id}/metadata`),
    );
    await page.keyboard.press('Enter');
    expect((await stored).status()).toBe(200);
    await expect(
      page.getByText(/^Gespeichert für Dokus\. \d+ Titel betroffen – jetzt anwenden\?$/),
    ).toBeVisible();
    expect((await libraryMetadataOf(owner, id)).textLangs).toEqual(['de']);

    // The libraries page names the deviation in the row; its fold-out links here (concept 12).
    await page.goto('/admin/libraries');
    const toggle = page.getByRole('button', { name: 'Dokus', exact: true });
    const row = page.getByRole('row').filter({ has: toggle });
    await expect(row.getByText('Sprachen abweichend')).toBeVisible();
    await tabToFromTop(page, toggle);
    await page.keyboard.press('Enter');
    const link = page
      .getByRole('region', { name: 'Dokus', exact: true })
      .getByRole('link', { name: 'Metadaten der Bibliothek' });
    await expect(link).toHaveAttribute('href', `/admin/metadata?library=${id}`);
    await tabTo(page, link, 10);
    await page.keyboard.press('Enter');
    await expect(page.getByRole('heading', { level: 1, name: 'Metadaten' })).toBeVisible();
    await expect(scopeSelect(page)).toHaveValue(String(id));
    await expect(page.getByRole('switch', { name: 'Sprachen abweichend' })).toBeChecked();
  } finally {
    const response = await owner.put(`/api/v1/admin/libraries/${id}/metadata`, {
      headers: CSRF,
      data: { textLangs: before.textLangs, images: before.images },
    });
    expect(response.status(), await response.text()).toBe(200);
  }
});

test('changes survive switching the scope; leaving the page asks first', async ({
  page,
  owner,
}) => {
  const before = await settingsOf(owner);
  const id = await libraryId(owner, 'Dokus');
  const libraryBefore = await libraryMetadataOf(owner, id);
  await openMetadata(page, '?item=images');
  const images = region(page, 'Bilder');
  const poster = images.getByRole('group', { name: 'Bildsprache Poster' });
  // The image places by the arrows: place 1 takes the language of place 2, the two swap.
  const chain = await placesOf(poster);
  expect(chain.length).toBeGreaterThan(1);
  const swapped = chooseAt(chain, 0, chain[1]);
  await tabToFromTop(page, place(poster, 1));
  await pickByKeys(page, place(poster, 1), chain[1]);
  expect(await placesOf(poster)).toEqual(swapped);
  await expectFocusOn(page, place(poster, 1));
  await expect(saveBar(page).getByText('Ungespeicherte Änderungen')).toBeVisible();

  // Another scope by keyboard, and back: the change is still there, nothing was asked.
  await chooseScope(page, 'Dokus');
  await expect(page.getByText(/Dokus übernimmt den Standard/)).toBeVisible();
  await expect(scopeSelect(page).getByRole('option', { name: 'Standard (geändert)' })).toHaveCount(
    1,
  );
  await expect(page.getByRole('alertdialog')).toHaveCount(0);
  await chooseScope(page, 'Standard');
  await expect(region(page, 'Bilder')).toBeVisible();
  expect(
    await placesOf(region(page, 'Bilder').getByRole('group', { name: 'Bildsprache Poster' })),
  ).toEqual(swapped);

  // The sidebar link waits for the question; Escape stays on the page.
  const sidebar = page.getByRole('navigation', { name: 'Verwaltung' });
  const current = sidebar.getByRole('link', { name: 'Metadaten' });
  const jobs = sidebar.getByRole('link', { name: 'Hintergrundjobs' });
  await arrowTo(page, current, jobs);
  await page.keyboard.press('Enter');
  const leave = page.getByRole('alertdialog', {
    name: 'Seite mit ungespeicherten Änderungen verlassen?',
  });
  // The question starts on «Auf der Seite bleiben», in both browsers (ui/Dialog, c61fe4b).
  const stay = leave.getByRole('button', { name: 'Auf der Seite bleiben' });
  await expectFocusOn(page, stay);
  await page.keyboard.press('Escape');
  await expect(leave).toBeHidden();
  await expect(page).toHaveURL(/\/admin\/metadata/);
  await expect(saveBar(page).getByText('Ungespeicherte Änderungen')).toBeVisible();

  // «Verwerfen und verlassen» goes; nothing was saved.
  await arrowTo(page, current, jobs);
  await page.keyboard.press('Enter');
  await expectFocusOn(page, stay);
  await tabTo(page, leave.getByRole('button', { name: 'Verwerfen und verlassen' }), 3);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin\/jobs/);
  expect(await settingsOf(owner)).toEqual(before);
  expect(await libraryMetadataOf(owner, id)).toEqual(libraryBefore);
});

test('"Jetzt anwenden" sends what and where and reports the queued run', async ({ page }) => {
  // The run itself would queue a match per title on the shared server; the request is answered
  // here, the server's own queueing is covered by the Go tests of the package metadata.
  let sent: { what: string[]; libraryIds?: number[] } | null = null;
  await page.route('**/api/v1/admin/metadata/apply', async (route) => {
    sent = route.request().postDataJSON() as { what: string[]; libraryIds?: number[] };
    await route.fulfill({
      status: 202,
      contentType: 'application/json',
      body: JSON.stringify({ jobId: 4711, titles: 12 }),
    });
  });
  await openMetadata(page, '?item=apply');
  const apply = region(page, 'Auf bestehende Titel anwenden');
  const what = apply.getByRole('radiogroup', { name: 'Was' });
  await tabToFromTop(page, what.getByRole('radio', { name: 'Texte und Bilder' }));
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('ArrowRight');
  await expect(what.getByRole('radio', { name: 'Nur Bilder' })).toBeChecked();
  await expect(
    apply.getByRole('radiogroup', { name: 'Wo' }).getByRole('radio', { name: /Alle Bibliotheken/ }),
  ).toBeChecked();
  const run = apply.getByRole('button', { name: 'Jetzt anwenden' });
  await tabToFromTop(page, run);
  await page.keyboard.press('Enter');
  await expect(
    apply.getByText('Lauf 4711 eingereiht: 12 Titel. Fortschritt unter Hintergrundjobs.'),
  ).toBeVisible();
  expect(sent).toEqual({ what: ['images'] });
  await expect(apply.getByRole('link', { name: 'Zu den Hintergrundjobs' })).toHaveAttribute(
    'href',
    '/admin/jobs',
  );
});

test('«Anwenden» polls the jobs every 10 s, and pauses while changes are unsaved', async ({
  page,
}) => {
  // The page's clock instead of the wall clock: runFor fires the 10-s interval at once, so the
  // test proves "every 10 s" and "nothing for 12 s" without waiting 26 s. The clock flows on its
  // own between the steps, so the keyboard part runs as it does without it. The requests are
  // counted in the page, where the interval's fetch starts before runFor returns; the network
  // events of the browser would arrive a moment later and make "none" a race.
  await page.clock.install();
  await page.addInitScript(() => {
    const counter = window as unknown as { jobTypePolls: number };
    counter.jobTypePolls = 0;
    const fetch = window.fetch.bind(window);
    window.fetch = (input, init) => {
      const url = input instanceof Request ? input.url : String(input);
      if (url.endsWith('/api/v1/admin/job-types')) counter.jobTypePolls += 1;
      return fetch(input, init);
    };
  });
  const polls = () =>
    page.evaluate(() => (window as unknown as { jobTypePolls: number }).jobTypePolls);
  await openMetadata(page, '?item=apply');
  await expect(region(page, 'Auf bestehende Titel anwenden')).toBeVisible();
  await expect.poll(polls).toBeGreaterThan(0);
  // Clean: another request once 10 s have passed on the page's clock.
  const clean = await polls();
  await page.clock.runFor(10_500);
  await expect.poll(polls).toBeGreaterThan(clean);

  // A change (the rating country, by keyboard), then back to «Anwenden»: no request for 12 s.
  const rating = areas(page).getByRole('option', { name: 'Altersfreigabe' });
  await arrowTo(page, areas(page).getByRole('option', { name: 'Anwenden' }), rating, 'ArrowUp');
  await page.keyboard.press('Enter');
  const current = region(page, 'Altersfreigabe').getByRole('radio', { checked: true });
  await tabToFromTop(page, current);
  await page.keyboard.press('ArrowRight');
  await expect(saveBar(page).getByText('Ungespeicherte Änderungen')).toBeVisible();
  await tabToFromTop(page, areas(page).getByRole('option', { name: 'Altersfreigabe' }));
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  await expect(region(page, 'Auf bestehende Titel anwenden')).toBeVisible();
  // The area mounts and asks once (the query is stale on mount); after that nothing for 12 s on
  // the page's clock while the change waits.
  const mounted = await polls();
  await page.clock.runFor(12_000);
  expect(await polls()).toBeLessThanOrEqual(mounted + 1);
  const paused = await polls();
  await page.clock.runFor(12_000);
  expect(await polls()).toBe(paused);
});
