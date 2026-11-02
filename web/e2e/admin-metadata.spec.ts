import type { APIRequestContext, Page } from '@playwright/test';
import { CSRF } from './catalog';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

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

async function openMetadata(page: Page, query = '') {
  await page.goto(`/admin/metadata${query}`);
  await expect(page.getByRole('heading', { level: 1, name: 'Metadaten' })).toBeVisible();
  await expect(areas(page)).toBeVisible();
}

/** Picks the scope in the header select; a changed scope carries «(geändert)». */
async function chooseScope(page: Page, name: string) {
  const select = scopeSelect(page);
  const label = await select
    .getByRole('option')
    .filter({ hasText: new RegExp(`^${name}( \\(geändert\\))?$`) })
    .textContent();
  await select.selectOption({ label: label ?? name });
}

test('the default scope lists the areas and the languages, the save bar is clean, nothing is sent', async ({
  page,
  owner,
}) => {
  const before = await settingsOf(owner);
  await openMetadata(page);
  await expect(
    page.getByRole('navigation', { name: 'Verwaltung' }).getByRole('link', { name: 'Metadaten' }),
  ).toHaveAttribute('aria-current', 'page');
  await expect(scopeSelect(page)).toHaveValue('server');
  await expect(areas(page).getByRole('option')).toHaveCount(6);
  const languages = region(page, 'Sprachen');
  const list = languages.getByRole('group', { name: 'Metadaten-Sprachen' });
  await expect(list.getByRole('listitem')).toHaveCount(before.textLangs.length);
  await expect(list.getByRole('listitem').first()).toContainText('Deutsch');
  await expect(list.getByLabel('Platz für Deutsch')).toHaveValue('0');
  // Deutsch is the only interface language left once Englisch goes: its remove button is disabled
  // only when it is the last one (E-9); with de and en both present, each may go.
  await expect(list.getByRole('button', { name: 'Deutsch entfernen' })).toBeEnabled();
  await expect(saveBar(page).getByText('Gespeichert', { exact: true })).toBeVisible();
  await expect(saveBar(page).getByRole('button', { name: 'Speichern' })).toBeDisabled();

  // The list is one Tab stop: ↓ moves, Enter selects; the detail follows.
  await resetFocus(page);
  await tabTo(page, areas(page).getByRole('option', { name: 'Sprachen' }), 80);
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, areas(page).getByRole('option', { name: 'Bilder' }));
  await page.keyboard.press('Enter');
  await expect(region(page, 'Bilder').getByText('immer')).toHaveCount(2);
  await expect(page).toHaveURL(/[?&]item=images/);
  for (const [option, title] of [
    ['Güte', 'Güte'],
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
});

test('a third language is added, put on its place, saved and removed again, all by keyboard', async ({
  page,
  owner,
}) => {
  const before = await settingsOf(owner);
  try {
    await openMetadata(page);
    const languages = region(page, 'Sprachen');
    const picker = languages.getByLabel('Sprache wählen');
    await resetFocus(page);
    await tabTo(page, picker, 80);
    await picker.selectOption({ label: 'Japanisch' });
    await page.keyboard.press('Tab');
    await expectFocusOn(page, languages.getByRole('button', { name: 'Hinzufügen' }));
    await page.keyboard.press('Enter');
    const list = languages.getByRole('group', { name: 'Metadaten-Sprachen' });
    await expect(list.getByRole('listitem')).toHaveCount(before.textLangs.length + 1);
    await expect(list.getByRole('listitem').last()).toContainText('Japanisch');
    await expect(saveBar(page).getByText('Ungespeicherte Änderungen')).toBeVisible();

    // ↑ on its place select moves Japanisch one place forward: it swaps with the one there.
    const place = list.getByLabel('Platz für Japanisch');
    await place.focus();
    await expectVisibleFocus(page);
    await page.keyboard.press('ArrowUp');
    await expect(list.getByRole('listitem').nth(before.textLangs.length - 1)).toContainText(
      'Japanisch',
    );
    await expect(place).toBeFocused();
    await expect(
      languages.getByText(/^Japanisch und .+ haben die Plätze getauscht\.$/),
    ).toBeVisible();

    const save = saveBar(page).getByRole('button', { name: 'Speichern' });
    await save.focus();
    await expectVisibleFocus(page);
    const stored = page.waitForResponse(
      (response) =>
        response.request().method() === 'PUT' &&
        response.url().endsWith('/admin/settings/metadata'),
    );
    await page.keyboard.press('Enter');
    expect((await stored).status()).toBe(200);
    await expect(
      page.getByText(/Gespeichert\. \d+ Titel betroffen – jetzt anwenden\?/),
    ).toBeVisible();
    await expect(saveBar(page).getByText('Gespeichert', { exact: true })).toBeVisible();
    const after = await settingsOf(owner);
    expect(after.textLangs).toContain('ja');
    expect(after.textLangs.indexOf('ja')).toBe(before.textLangs.length - 1);

    // Removing it by keyboard and saving puts the list back (the swapped language stays last).
    const remove = list.getByRole('button', { name: 'Japanisch entfernen' });
    await remove.focus();
    await page.keyboard.press('Enter');
    await expect(list.getByRole('listitem')).toHaveCount(before.textLangs.length);
    await save.focus();
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
  await current.focus();
  await expectVisibleFocus(page);
  await page.keyboard.press(before.ratingCountry === 'US' ? 'ArrowLeft' : 'ArrowRight');
  await expect(group.getByRole('radio', { checked: true })).not.toHaveAccessibleName(
    new RegExp(`\\(${before.ratingCountry}\\)`),
  );
  await expect(saveBar(page).getByText('Ungespeicherte Änderungen')).toBeVisible();
  await expect(areas(page).getByRole('option', { name: 'Altersfreigabe' })).toContainText(
    'Geändert',
  );
  const discard = saveBar(page).getByRole('button', { name: 'Verwerfen' });
  await discard.focus();
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
  // The bar's buttons are disabled now: the focus waits on the page head (ALL-2).
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
    await expect(list.getByRole('button', { name: 'Englisch entfernen' })).toBeDisabled();

    // Space switches the deviation on; the places become editable, "wie Server" goes.
    const deviate = page.getByRole('switch', { name: 'Sprachen abweichend' });
    await deviate.focus();
    await expectVisibleFocus(page);
    await page.keyboard.press('Space');
    await expect(deviate).toBeChecked();
    await expect(languages.getByText('wie Server')).toHaveCount(0);
    const remove = list.getByRole('button', { name: 'Englisch entfernen' });
    await expect(remove).toBeEnabled();
    await remove.focus();
    await page.keyboard.press('Enter');
    await expect(list.getByRole('listitem')).toHaveCount(1);
    // Deutsch is the last interface language of the library now: it cannot go (E-9).
    await expect(list.getByRole('button', { name: 'Deutsch entfernen' })).toBeDisabled();

    const save = saveBar(page).getByRole('button', { name: 'Speichern' });
    await save.focus();
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

    await page.goto('/admin/libraries');
    const card = page.getByRole('article', { name: 'Dokus' });
    await expect(card.getByText('Metadaten: Sprachen abweichend')).toBeVisible();
    const link = card.getByRole('link', { name: 'Metadaten der Bibliothek' });
    await expect(link).toHaveAttribute('href', `/admin/metadata?library=${id}`);
    await link.click();
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
  // The image places by keyboard: ↓ on the first place swaps it with the second.
  const first = poster.getByRole('listitem').first();
  const firstName = ((await first.getByRole('button').getAttribute('aria-label')) ?? '').replace(
    / entfernen$/,
    '',
  );
  const place = first.getByRole('combobox');
  await place.focus();
  await expectVisibleFocus(page);
  await page.keyboard.press('ArrowDown');
  await expect(poster.getByRole('listitem').nth(1)).toContainText(firstName);
  await expect(saveBar(page).getByText('Ungespeicherte Änderungen')).toBeVisible();

  // Another scope by keyboard, and back: the change is still there, nothing was asked.
  await chooseScope(page, 'Dokus');
  await expect(page.getByText(/Dokus übernimmt den Standard/)).toBeVisible();
  await expect(scopeSelect(page).getByRole('option', { name: 'Standard (geändert)' })).toHaveCount(
    1,
  );
  await expect(page.getByRole('alertdialog')).toHaveCount(0);
  await chooseScope(page, 'Standard');
  await expect(
    region(page, 'Bilder')
      .getByRole('group', { name: 'Bildsprache Poster' })
      .getByRole('listitem')
      .nth(1),
  ).toContainText(firstName);

  // The sidebar link waits for the question; Escape stays on the page.
  const jobs = page
    .getByRole('navigation', { name: 'Verwaltung' })
    .getByRole('link', { name: 'Hintergrundjobs' });
  await jobs.focus();
  await page.keyboard.press('Enter');
  const leave = page.getByRole('alertdialog', {
    name: 'Seite mit ungespeicherten Änderungen verlassen?',
  });
  await expectFocusOn(page, leave.getByRole('button', { name: 'Auf der Seite bleiben' }));
  await page.keyboard.press('Escape');
  await expect(leave).toBeHidden();
  await expect(page).toHaveURL(/\/admin\/metadata/);
  await expect(saveBar(page).getByText('Ungespeicherte Änderungen')).toBeVisible();

  // «Verwerfen und verlassen» goes; nothing was saved.
  await jobs.focus();
  await page.keyboard.press('Enter');
  await leave.getByRole('button', { name: 'Verwerfen und verlassen' }).focus();
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
  await what.getByRole('radio', { name: 'Texte und Bilder' }).focus();
  await expectVisibleFocus(page);
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('ArrowRight');
  await expect(what.getByRole('radio', { name: 'Nur Bilder' })).toBeChecked();
  await expect(
    apply.getByRole('radiogroup', { name: 'Wo' }).getByRole('radio', { name: /Alle Bibliotheken/ }),
  ).toBeChecked();
  const run = apply.getByRole('button', { name: 'Jetzt anwenden' });
  await run.focus();
  await expectVisibleFocus(page);
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
