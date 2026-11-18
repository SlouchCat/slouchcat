import type { APIRequestContext, Locator, Page } from '@playwright/test';
import { CSRF } from './catalog';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';
import { areaTab } from './admin';

// The page "Metadaten" (concept 10, 7.1, ADR 0018; concept 24 3.4) against the real server: the
// settings areas as fold sections (all closed, `?item=` opens one, «geändert» in the header), the
// default with its language places, the sticky save bar, the rating country, the apply run per
// folder and the question before leaving with unsaved changes. Every test leaves the settings as it found them (the suite shares
// one server).

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

const region = (page: Page, name: string) => page.getByRole('region', { name, exact: true });
const saveBar = (page: Page) => region(page, 'Speichern');

/** The fold sections of the page, top to bottom. */
const SECTIONS = [
  'Sprachen',
  'Bilder',
  'Bildqualität',
  'Quellen',
  'Altersfreigabe',
  'Anwenden',
  'Ordner',
] as const;
type SectionName = (typeof SECTIONS)[number];

/** The toggle of a section: the button in its h2. */
const toggle = (page: Page, name: SectionName) =>
  page
    .getByRole('heading', { level: 2, name, exact: true })
    .getByRole('button', { name, exact: true });
/** The header line: title, summary, «geändert». */
const headerOf = (page: Page, name: SectionName) =>
  page.getByRole('heading', { level: 2, name, exact: true }).locator('xpath=..');
/** The content of an open section (react-aria's panel, named by its toggle). */
const section = (page: Page, name: SectionName) => page.getByRole('group', { name, exact: true });

async function openSection(page: Page, name: SectionName): Promise<Locator> {
  const button = toggle(page, name);
  if ((await button.getAttribute('aria-expanded')) !== 'true') await button.click();
  await expect(section(page, name)).toBeVisible();
  return section(page, name);
}

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
  await expect(toggle(page, 'Sprachen')).toBeVisible();
}

/** Moves the focus to `target` with Tab from the top of the page (P-15: no `focus()` by script). */
async function tabToFromTop(page: Page, target: Locator) {
  await resetFocus(page);
  await tabTo(page, target, 200);
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

test('the default shows seven closed sections with their summaries; nothing is sent', async ({
  page,
  owner,
}) => {
  const before = await settingsOf(owner);
  await openMetadata(page);
  await expect(areaTab(page, 'Metadaten')).toHaveAttribute('aria-current', 'page');
  const headings = page.getByRole('main').getByRole('heading', { level: 2 });
  await expect(headings).toHaveText([
    'Sprachen',
    'Bilder',
    'Bildqualität',
    'Quellen',
    'Altersfreigabe',
    'Anwenden',
    'Ordner',
  ]);
  for (const name of SECTIONS)
    await expect(toggle(page, name)).toHaveAttribute('aria-expanded', 'false');
  // The summary of the languages is the default's list in order.
  await expect(headerOf(page, 'Sprachen')).toContainText(before.textLangs.length > 1 ? '›' : '');
  await expect(headerOf(page, 'Ordner')).toContainText(/Video-Ordner (folgt|folgen) dem Standard/);
  const languages = await openSection(page, 'Sprachen');
  const list = languages.getByRole('group', { name: 'Metadaten-Sprachen' });
  await expect(list.getByRole('listitem')).toHaveCount(before.textLangs.length);
  // Fixed places, each a select of the languages (10.2).
  await expect(place(list, 1)).toHaveValue('de');
  // The default needs no Deutsch any more (concept 22 F-7): its «×» is on.
  await expect(list.getByRole('button', { name: '1. Sprache (Deutsch) entfernen' })).toBeEnabled();
  await expect(saveBar(page).getByText('Gespeichert', { exact: true })).toBeVisible();
  // Nothing changed: «Speichern» stays usable and «Verwerfen» is not there (decision 18).
  await expect(saveBar(page).getByRole('button', { name: 'Speichern' })).toBeEnabled();
  await expect(saveBar(page).getByRole('button', { name: 'Verwerfen' })).toHaveCount(0);
});

test('the sections open and close by keyboard, the focus stays on the header @keyboard', async ({
  page,
}) => {
  // The overview first, for the browser back at the end. Both pages must have rendered before
  // going back (Firefox, see admin-libraries.spec).
  await page.goto('/admin');
  await expect(page.getByRole('heading', { level: 1, name: 'Alles im Blick' })).toBeVisible();
  await openMetadata(page);
  // Tab reaches each closed header in turn; Enter opens, Space closes; the focus stays (rule 8).
  await tabToFromTop(page, toggle(page, 'Sprachen'));
  for (const [index, name] of SECTIONS.entries()) {
    if (index > 0) {
      await page.keyboard.press('Tab');
      await expectFocusOn(page, toggle(page, name));
    }
    await page.keyboard.press('Enter');
    await expect(toggle(page, name)).toHaveAttribute('aria-expanded', 'true');
    await expect(section(page, name)).toBeVisible();
    await expectFocusOn(page, toggle(page, name));
    await page.keyboard.press('Space');
    await expect(toggle(page, name)).toHaveAttribute('aria-expanded', 'false');
    await expect(section(page, name)).toBeHidden();
    await expectFocusOn(page, toggle(page, name));
  }
  // An open section takes Tab into its content, then on to the next header.
  await tabToFromTop(page, toggle(page, 'Bildqualität'));
  await page.keyboard.press('Space');
  await page.keyboard.press('Tab');
  await expect(
    section(page, 'Bildqualität').getByRole('radio', { checked: true }).first(),
  ).toBeFocused();
  await tabTo(page, toggle(page, 'Quellen'));
  await expectNoFocusTrap(page, 120);
  // Opening sections changes no address: browser back returns to the overview.
  await expect(page).toHaveURL(/\/admin\/metadata$/);
  await page.goBack();
  await expect(page).toHaveURL(/\/admin$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Alles im Blick' })).toBeVisible();
});

test('a language is added, put on its place with several arrow steps, saved and removed, all by keyboard @keyboard', async ({
  page,
  owner,
}) => {
  // About 355 key presses: seven walks by Tab from the top of the page (130 presses, 13 to 29
  // each) and 225 arrow steps through the language list (105 + 60 + 60). Measured on 18 November
  // 2026 on a developer's machine: 19.5 s in Firefox (8.2 s of Tab walks, 8.2 s of arrow steps),
  // 20 s in Chromium; on the CI runner Chromium took 21.4 s and Firefox ran past 30 s.
  test.setTimeout(60_000);
  const before = await settingsOf(owner);
  try {
    await openMetadata(page, '?item=languages');
    const languages = section(page, 'Sprachen');
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

test('the rating country is chosen in the list with its steps; "Verwerfen" asks and drops the change unsaved', async ({
  page,
  owner,
}) => {
  const before = await settingsOf(owner);
  await openMetadata(page, '?item=rating');
  const rating = section(page, 'Altersfreigabe');
  // Every rating country of the server's table (concept 22 stage 3), named by Intl.
  const country = rating.getByRole('combobox', { name: 'Land' });
  await expect(country).toHaveValue(before.ratingCountry);
  await expect(country.getByRole('option')).toHaveCount(25);
  await tabToFromTop(page, country);
  const other = before.ratingCountry === 'FR' ? 'JP' : 'FR';
  await country.selectOption(other);
  await expect(country).toHaveValue(other);
  await expect(rating.getByText(/^Stufen der Profile: /)).toContainText(
    other === 'FR' ? 'Tous publics · -10 · -12 · -16 · -18' : 'G · PG12 · R15+ · R18+',
  );
  await expect(saveBar(page).getByText('Ungespeicherte Änderungen')).toBeVisible();
  // The header says «geändert», also once the section is folded away; the others do not.
  await tabToFromTop(page, toggle(page, 'Altersfreigabe'));
  await page.keyboard.press('Enter');
  await expect(toggle(page, 'Altersfreigabe')).toHaveAttribute('aria-expanded', 'false');
  await expect(
    headerOf(page, 'Altersfreigabe').getByText('geändert', { exact: true }),
  ).toBeVisible();
  await expect(headerOf(page, 'Sprachen').getByText('geändert', { exact: true })).toHaveCount(0);
  await page.keyboard.press('Enter');
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
  await expect(country).toHaveValue(before.ratingCountry);
  expect((await settingsOf(owner)).ratingCountry).toBe(before.ratingCountry);
});

test('a new rating country with titles asks before saving; Escape keeps the change unsaved @keyboard', async ({
  page,
  owner,
}) => {
  const before = await settingsOf(owner);
  // The question comes only when titles would be read anew (concept 22 F-16): the answer of the
  // settings says there are some, whatever the e2e library holds.
  await page.route('**/api/v1/admin/settings/metadata', async (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    const response = await route.fetch();
    const stored = (await response.json()) as Settings;
    await route.fulfill({ response, json: { ...stored, affectedTitles: 5 } });
  });
  await openMetadata(page, '?item=rating');
  const country = section(page, 'Altersfreigabe').getByRole('combobox', { name: 'Land' });
  await tabToFromTop(page, country);
  await country.selectOption(before.ratingCountry === 'US' ? 'DE' : 'US');
  const save = saveBar(page).getByRole('button', { name: 'Speichern' });
  await tabToFromTop(page, save);
  await page.keyboard.press('Enter');
  const dialog = page.getByRole('alertdialog', { name: 'Freigabeland wechseln?' });
  await expect(dialog).toContainText('Slouchcat liest danach die Freigaben aller Titel neu.');
  await expectFocusOn(page, dialog.getByRole('button', { name: 'Abbrechen' }));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, dialog.getByRole('button', { name: 'Wechseln und neu lesen' }));
  await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();
  await expectFocusOn(page, save);
  await expect(saveBar(page).getByText('Ungespeicherte Änderungen')).toBeVisible();
  expect((await settingsOf(owner)).ratingCountry).toBe(before.ratingCountry);
});

test('"Jetzt anwenden" sends what and where and reports the queued run', async ({ page }) => {
  // The run itself would queue a match per title on the shared server; the request is answered
  // here, the server's own queueing is covered by the Go tests of the package metadata.
  let sent: { what: string[]; rootIds?: number[] } | null = null;
  await page.route('**/api/v1/admin/metadata/apply', async (route) => {
    sent = route.request().postDataJSON() as { what: string[]; rootIds?: number[] };
    await route.fulfill({
      status: 202,
      contentType: 'application/json',
      body: JSON.stringify({ jobId: 4711, titles: 12 }),
    });
  });
  await openMetadata(page, '?item=apply');
  const apply = section(page, 'Anwenden');
  const what = apply.getByRole('radiogroup', { name: 'Was' });
  await tabToFromTop(page, what.getByRole('radio', { name: 'Texte und Bilder' }));
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('ArrowRight');
  await expect(what.getByRole('radio', { name: 'Nur Bilder' })).toBeChecked();
  await expect(
    apply.getByRole('radiogroup', { name: 'Wo' }).getByRole('radio', { name: /Alle Ordner/ }),
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
  await expect(section(page, 'Anwenden')).toBeVisible();
  await expect.poll(polls).toBeGreaterThan(0);
  // Clean: another request once 10 s have passed on the page's clock.
  const clean = await polls();
  await page.clock.runFor(10_500);
  await expect.poll(polls).toBeGreaterThan(clean);

  // A change (the rating country, by keyboard) while «Anwenden» stays open: no request for 12 s.
  await tabToFromTop(page, toggle(page, 'Altersfreigabe'));
  await page.keyboard.press('Enter');
  const country = section(page, 'Altersfreigabe').getByRole('combobox', { name: 'Land' });
  await tabToFromTop(page, country);
  await country.selectOption((await country.inputValue()) === 'FR' ? 'JP' : 'FR');
  await expect(saveBar(page).getByText('Ungespeicherte Änderungen')).toBeVisible();
  await expect(section(page, 'Anwenden')).toBeVisible();
  // At most one request still under way; after that nothing for 12 s on the page's clock while
  // the change waits.
  const mounted = await polls();
  await page.clock.runFor(12_000);
  expect(await polls()).toBeLessThanOrEqual(mounted + 1);
  const paused = await polls();
  await page.clock.runFor(12_000);
  expect(await polls()).toBe(paused);
});

test('?item= opens the named section only; «Anwenden» goes to one folder with its titles', async ({
  page,
  owner,
}) => {
  const libraries = await owner.get('/api/v1/admin/libraries');
  const roots = (
    (await libraries.json()) as {
      name: string;
      kind: string;
      roots: { id: number; path: string; content: string; homeTitleCount: number }[];
    }[]
  )
    .filter((library) => library.kind === 'video')
    .flatMap((library) =>
      library.roots
        .filter((root) => root.content === 'movie' || root.content === 'series')
        .map((root) => ({ ...root, library: library.name })),
    );
  const folder = roots[0];
  expect(folder, 'the e2e server has a video folder').toBeDefined();
  let sent: { what: string[]; rootIds?: number[] } | null = null;
  await page.route('**/api/v1/admin/metadata/apply', async (route) => {
    sent = route.request().postDataJSON() as { what: string[]; rootIds?: number[] };
    await route.fulfill({
      status: 202,
      contentType: 'application/json',
      body: JSON.stringify({ jobId: 4712, titles: folder.homeTitleCount }),
    });
  });

  await openMetadata(page, '?item=folders');
  const folders = section(page, 'Ordner');
  await expect(folders).toBeVisible();
  for (const name of SECTIONS.filter((entry) => entry !== 'Ordner'))
    await expect(toggle(page, name)).toHaveAttribute('aria-expanded', 'false');
  await expect(folders.getByRole('listitem')).toHaveCount(roots.length);
  await expect(folders.getByRole('listitem').first()).toContainText(folder.path);
  await expect(folders.getByRole('listitem').first()).toContainText(/Sprachen: /);
  // The row action is «Bearbeiten» of the folder's library with that folder's card open
  // (concept 24 4.5); Escape without a change returns to it.
  const edit = folders.getByRole('button', {
    name: `${folder.library} bearbeiten, Ordner ${folder.path}`,
  });
  await tabToFromTop(page, edit);
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  const form = page.getByRole('dialog', { name: 'Bibliothek bearbeiten' });
  const card = form
    .getByRole('listitem', { name: /^Ordner \d+$/ })
    .filter({ hasText: folder.path });
  await expect(card.getByRole('button').first()).toHaveAttribute('aria-expanded', 'true');
  await expectFocusOn(page, card.getByLabel('Ordner', { exact: true }));
  await page.keyboard.press('Escape');
  await expect(form).toHaveCount(0);
  await expectFocusOn(page, edit);

  await openMetadata(page, '?item=apply');
  const apply = section(page, 'Anwenden');
  const one = apply.getByRole('radiogroup', { name: 'Wo' }).getByRole('radio', {
    name: `${folder.library}: ${folder.path} (${folder.homeTitleCount} Titel)`,
  });
  await tabToFromTop(page, apply.getByRole('radio', { name: /^Alle Ordner/ }));
  // ↓ moves inside the vertical group to the first folder.
  await page.keyboard.press('ArrowDown');
  await expect(one).toBeChecked();
  await expect(headerOf(page, 'Anwenden')).toContainText(
    `Texte und Bilder · ${folder.library}: ${folder.path}`,
  );
  await tabToFromTop(page, apply.getByRole('button', { name: 'Jetzt anwenden' }));
  await page.keyboard.press('Enter');
  await expect(
    apply.getByText(
      `Lauf 4712 eingereiht: ${folder.homeTitleCount} Titel. Fortschritt unter Hintergrundjobs.`,
    ),
  ).toBeVisible();
  expect(sent).toEqual({ what: ['texts', 'images'], rootIds: [folder.id] });
});
