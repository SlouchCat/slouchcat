import type { APIRequestContext, Page } from '@playwright/test';
import { CSRF } from './catalog';
import { choose } from './controls';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// The page "Metadaten" (concept 10, 7.1, ADR 0018) against the real server: the server default
// with its language list, the sticky save bar, a library's deviation, the rating country and the
// apply run. Every test leaves the settings as it found them (the suite shares one server).

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

async function openMetadata(page: Page) {
  await page.goto('/admin/metadata');
  await expect(page.getByRole('heading', { level: 1, name: 'Metadaten' })).toBeVisible();
  await expect(region(page, 'Sprachen')).toBeVisible();
}

/** Picks the scope: the segments up to five libraries, the select above that. */
async function chooseScope(page: Page, name: string) {
  const radios = page.getByRole('radiogroup', { name: 'Geltung' });
  if ((await radios.count()) > 0) {
    await choose(radios, 'radio', name);
  } else {
    await page.getByLabel('Geltung').selectOption({ label: name });
  }
}

test('the default scope lists the languages, the save bar is clean, nothing is sent', async ({
  page,
  owner,
}) => {
  const before = await settingsOf(owner);
  await openMetadata(page);
  await expect(
    page.getByRole('navigation', { name: 'Verwaltung' }).getByRole('link', { name: 'Metadaten' }),
  ).toHaveAttribute('aria-current', 'page');
  const languages = region(page, 'Sprachen');
  const list = languages.getByRole('group', { name: 'Metadaten-Sprachen' });
  await expect(list.getByRole('listitem')).toHaveCount(before.textLangs.length);
  await expect(list.getByRole('listitem').first()).toContainText('Deutsch');
  // Deutsch is the only interface language left once Englisch goes: its remove button is disabled
  // only when it is the last one (E-9); with de and en both present, each may go.
  await expect(list.getByRole('button', { name: 'Deutsch entfernen' })).toBeEnabled();
  await expect(saveBar(page).getByText('Gespeichert', { exact: true })).toBeVisible();
  await expect(saveBar(page).getByRole('button', { name: 'Speichern' })).toBeDisabled();
  await expect(region(page, 'Bilder').getByText('immer')).toHaveCount(2);
  await expect(region(page, 'Güte')).toBeVisible();
  await expect(region(page, 'Quellen')).toBeVisible();
  await expect(region(page, 'Altersfreigabe')).toBeVisible();
  await expect(region(page, 'Auf bestehende Titel anwenden')).toBeVisible();
});

test('a third language is added, saved and removed again, all by keyboard', async ({
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

    // The arrows move the new language forward; the position follows.
    const forward = list.getByRole('button', { name: 'Japanisch nach vorne' });
    await forward.focus();
    await expectVisibleFocus(page);
    await page.keyboard.press('Enter');
    await expect(list.getByRole('listitem').nth(before.textLangs.length - 1)).toContainText(
      'Japanisch',
    );

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

    // Removing it by keyboard and saving puts the list back.
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

test('the rating country changes with the arrow keys; "Verwerfen" drops the change unsaved', async ({
  page,
  owner,
}) => {
  const before = await settingsOf(owner);
  await openMetadata(page);
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
  const discard = saveBar(page).getByRole('button', { name: 'Verwerfen' });
  await discard.focus();
  await page.keyboard.press('Enter');
  await expect(saveBar(page).getByText('Gespeichert', { exact: true })).toBeVisible();
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
    const languages = region(page, 'Sprachen');
    await expect(languages.getByText('wie Server')).toBeVisible();
    const list = languages.getByRole('group', { name: 'Metadaten-Sprachen' });
    await expect(list.getByRole('button', { name: 'Englisch entfernen' })).toBeDisabled();

    // Space switches the deviation on; the chips become editable, "wie Server" goes.
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
    await expect(page.getByText('Gespeichert für Dokus.')).toBeVisible();
    expect((await libraryMetadataOf(owner, id)).textLangs).toEqual(['de']);

    await page.goto('/admin/libraries');
    const card = page.getByRole('article', { name: 'Dokus' });
    await expect(card.getByText('Metadaten: Sprachen abweichend')).toBeVisible();
    const link = card.getByRole('link', { name: 'Metadaten der Bibliothek' });
    await expect(link).toHaveAttribute('href', `/admin/metadata?library=${id}`);
    await link.click();
    await expect(page.getByRole('heading', { level: 1, name: 'Metadaten' })).toBeVisible();
    await expect(page.getByRole('switch', { name: 'Sprachen abweichend' })).toBeChecked();
  } finally {
    const response = await owner.put(`/api/v1/admin/libraries/${id}/metadata`, {
      headers: CSRF,
      data: { textLangs: before.textLangs, images: before.images },
    });
    expect(response.status(), await response.text()).toBe(200);
  }
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
  await openMetadata(page);
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
