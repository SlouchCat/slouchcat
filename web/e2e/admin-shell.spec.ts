import type { Page } from '@playwright/test';
import { adminArea, adminEntry, adminNav, areaTab, areaTabs } from './admin';
import { H264_TITLE, idOf, itemPath } from './catalog';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// The frame of the administration (redesign, build plan 2; rule 8): the sidebar with back, the
// overview and the five areas; the area's pages as tabs above every page of the area; the
// breadcrumb in the header; the search palette («Bereich suchen», Ctrl+K); on a phone the sheet
// with every page. All by keyboard, with a visible focus and no focus trap.

/** Every area with its pages in the order of their tabs, each with the heading of its page. */
const AREAS: readonly (readonly [string, readonly (readonly [string, string, string])[]])[] = [
  [
    'Medien',
    [
      ['Bibliotheken', '/admin/libraries', 'Bibliotheken'],
      ['Metadaten', '/admin/metadata', 'Metadaten'],
      ['Pflege', '/admin/maintenance', 'Pflege'],
      ['Wünsche', '/admin/wishes', 'Wünsche'],
      ['Podcasts', '/admin/podcasts', 'Podcasts'],
      ['Radio', '/admin/radio', 'Radio'],
      ['Hörbuch-Quellen', '/admin/metadata/audiobooks', 'Hörbücher: Quellen'],
    ],
  ],
  [
    'Nutzer & Geräte',
    [
      ['Konten', '/admin/accounts', 'Konten'],
      ['Geräte', '/admin/devices', 'Geräte'],
      ['Streams', '/admin/streams', 'Streams'],
    ],
  ],
  [
    'Startseite',
    [
      ['Startseite', '/admin/home-rows', 'Startseite'],
      ['Spotlight', '/admin/spotlight', 'Spotlight'],
      ['User-Empfehlungen', '/admin/picks', 'User-Empfehlungen'],
      ['Sammlungen', '/admin/collections', 'Sammlungen'],
      ['Meine Medien', '/admin/my-media', 'Meine Medien'],
      ['Ankündigungen', '/admin/announcements', 'Ankündigungen'],
    ],
  ],
  [
    'Automatisierung',
    [
      ['Hintergrundjobs', '/admin/jobs', 'Hintergrundjobs'],
      ['Integrationen', '/admin/integrations', 'Integrationen'],
      // The account's page inside the administration (concept 15, P-4).
      ['Benachrichtigungen', '/admin/notifications', 'Benachrichtigungskanäle'],
    ],
  ],
  [
    'System',
    [
      ['Statistiken', '/admin/stats', 'Statistiken'],
      ['Sicherung', '/admin/backup', 'Sicherung'],
      ['Server', '/admin/server', 'Server'],
    ],
  ],
];

const PAGES = AREAS.flatMap(([area, pages]) => pages.map((page) => [area, ...page] as const));

const h1 = (page: Page, name: string) => page.getByRole('heading', { level: 1, name, exact: true });
const finder = (page: Page) => page.getByRole('button', { name: 'Bereich suchen' });
const palette = (page: Page) => page.getByRole('dialog', { name: 'Bereich der Verwaltung suchen' });
const paletteField = (page: Page) =>
  palette(page).getByRole('combobox', { name: 'Bereich der Verwaltung suchen' });

test('the sidebar: back, the overview and five areas as a tree; one Tab stop and the arrows', async ({
  page,
}) => {
  await page.goto('/admin/jobs');
  await expect(h1(page, 'Hintergrundjobs')).toBeVisible();
  const nav = adminNav(page);
  // Shown: back, the overview and the three pages of the open area «Automatisierung»; the five
  // areas are buttons that fold their pages in and out.
  await expect(nav.getByRole('link')).toHaveCount(5);
  await expect(nav.getByRole('button')).toHaveCount(5);
  await expect(nav.getByRole('list', { name: 'Verwaltung' }).getByRole('link')).toHaveCount(4);
  const automation = adminArea(page, 'Automatisierung');
  const jobs = adminEntry(page, 'Hintergrundjobs');
  await expect(automation).toHaveAttribute('aria-current', 'true');
  await expect(automation).toHaveAttribute('aria-expanded', 'true');
  await expect(jobs).toHaveAttribute('aria-current', 'page');
  await expect(nav.locator('[aria-current="page"]')).toHaveCount(1);
  // The active profile with its role at the foot of the column, outside the navigation.
  await expect(page.getByText('Server-Admin', { exact: true })).toBeVisible();

  await resetFocus(page);
  await tabTo(page, jobs);
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, automation);
  await page.keyboard.press('Home');
  await expectFocusOn(page, adminEntry(page, 'Zurück zur App'));
  await page.keyboard.press('End');
  const system = adminArea(page, 'System');
  await expectFocusOn(page, system);
  // Enter on an area only folds its pages out; the page stays.
  await page.keyboard.press('Enter');
  await expect(system).toHaveAttribute('aria-expanded', 'true');
  await expect(page).toHaveURL(/\/admin\/jobs$/);
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, adminEntry(page, 'Statistiken'));
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin\/stats$/);
  await expect(h1(page, 'Statistiken')).toBeVisible();
  // The sidebar stays and keeps the focus on the followed entry (focusCarry).
  await expectFocusOn(page, adminEntry(page, 'Statistiken'));
  await expect(adminEntry(page, 'Statistiken')).toHaveAttribute('aria-current', 'page');
  await expect(system).toHaveAttribute('aria-current', 'true');
  await expect(automation).not.toHaveAttribute('aria-current');
  // Tab leaves the column at once and lands on the area's tabs.
  await page.keyboard.press('Tab');
  await expectFocusOn(page, areaTab(page, 'Statistiken'));
  await expectNoFocusTrap(page, 120);
});

test('the area tabs reach every one of the 22 pages by keyboard', async ({ page }) => {
  test.setTimeout(150_000);
  expect(PAGES).toHaveLength(22);
  for (const [area, pages] of AREAS) {
    const [first] = pages;
    if (!first) throw new Error(`no page in ${area}`);
    await page.goto(first[1]);
    await expect(h1(page, first[2])).toBeVisible();
    await expect(areaTabs(page)).toHaveAccessibleName(`Seiten im Bereich ${area}`);
    await expect(areaTabs(page).getByRole('link')).toHaveCount(pages.length);
    await resetFocus(page);
    await tabTo(page, areaTab(page, first[0]), 60);
    for (const [name, path, heading] of pages.slice(1)) {
      // One Tab stop: → moves to the next page's tab, Enter follows it; the next page's tabs
      // take the focus over on the same tab.
      await page.keyboard.press('ArrowRight');
      await expectFocusOn(page, areaTab(page, name));
      await page.keyboard.press('Enter');
      await expect(page).toHaveURL(new RegExp(`${path}$`));
      await expect(h1(page, heading)).toBeVisible();
      await expect(areaTab(page, name)).toHaveAttribute('aria-current', 'page');
      await expect(adminArea(page, area)).toHaveAttribute('aria-current', 'true');
      await expect(adminEntry(page, name)).toHaveAttribute('aria-current', 'page');
      await expectFocusOn(page, areaTab(page, name));
    }
    // The arrows wrap: → from the last page is the first.
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, areaTab(page, first[0]));
    await page.keyboard.press('End');
    await expectFocusOn(page, areaTab(page, pages.at(-1)?.[0] ?? ''));
    await page.keyboard.press('Home');
    await expectFocusOn(page, areaTab(page, first[0]));
  }
});

test('the overview has no area tabs; the line over each title names area and page', async ({
  page,
}) => {
  await page.goto('/admin');
  await expect(h1(page, 'Alles im Blick')).toBeVisible();
  await expect(areaTabs(page)).toHaveCount(0);
  await page.goto('/admin/backup');
  await expect(h1(page, 'Sicherung')).toBeVisible();
  await expect(page.getByText('System / Sicherung', { exact: true })).toBeVisible();
  // The intro stands open below the title; there is no help button in the frame.
  await expect(page.getByRole('button', { name: 'Hilfe zu dieser Seite' })).toHaveCount(0);
});

test('a long intro shows two lines; «Mehr» and «Weniger» by keyboard', async ({ page }) => {
  await page.goto('/admin/radio');
  await expect(h1(page, 'Radio')).toBeVisible();
  const more = page.getByRole('button', { name: 'Mehr', exact: true });
  await expect(more).toHaveAttribute('aria-expanded', 'false');
  const intro = page.locator(`[id="${await more.getAttribute('aria-controls')}"]`);
  const closed = (await intro.boundingBox())?.height ?? 0;
  const lineHeight = await intro.evaluate((node) => parseFloat(getComputedStyle(node).lineHeight));
  expect(closed).toBeLessThanOrEqual(2 * lineHeight + 1);

  await resetFocus(page);
  await tabTo(page, more, 30);
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  const less = page.getByRole('button', { name: 'Weniger', exact: true });
  await expect(less).toHaveAttribute('aria-expanded', 'true');
  await expectFocusOn(page, less);
  expect((await intro.boundingBox())?.height ?? 0).toBeGreaterThan(closed);
  await page.keyboard.press('Enter');
  await expect(more).toHaveAttribute('aria-expanded', 'false');
  expect((await intro.boundingBox())?.height ?? 0).toBeCloseTo(closed, 0);
  await expectNoFocusTrap(page);
});

test('the breadcrumb in the header: Verwaltung › area / page, by keyboard', async ({ page }) => {
  await page.goto('/admin/podcasts');
  await expect(h1(page, 'Podcasts')).toBeVisible();
  const crumbs = page.getByRole('navigation', { name: 'Pfad' });
  await expect(crumbs.getByRole('link', { name: 'Verwaltung' })).toHaveAttribute('href', '/admin');
  const area = crumbs.getByRole('link', { name: 'Medien' });
  await expect(area).toHaveAttribute('href', '/admin/libraries');
  await expect(crumbs.getByText('Podcasts', { exact: true })).toHaveAttribute(
    'aria-current',
    'page',
  );
  await expect(page.getByText('Control Center', { exact: true })).toBeVisible();

  await resetFocus(page);
  await tabTo(page, area, 10);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin\/libraries$/);
  await expect(h1(page, 'Bibliotheken')).toBeVisible();
  await resetFocus(page);
  await tabTo(page, crumbs.getByRole('link', { name: 'Verwaltung' }), 10);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin$/);
  await expect(h1(page, 'Alles im Blick')).toBeVisible();
  // On the overview: «Verwaltung › Übersicht».
  await expect(crumbs.getByText('Übersicht', { exact: true })).toHaveAttribute(
    'aria-current',
    'page',
  );
});

test('the palette: the button and Ctrl+K open it, typing filters, Enter opens, Escape returns', async ({
  page,
}) => {
  await page.goto('/admin/libraries');
  await expect(h1(page, 'Bibliotheken')).toBeVisible();

  // By the button: Tab to it, Enter; the field has the focus.
  await resetFocus(page);
  await tabTo(page, finder(page), 10);
  await expect(finder(page)).toHaveAttribute('aria-keyshortcuts', 'Control+K');
  await page.keyboard.press('Enter');
  await expect(palette(page)).toBeVisible();
  await expectFocusOn(page, paletteField(page));
  // Overview, five areas, 22 pages, back to the app and the media search.
  await expect(palette(page).getByRole('option')).toHaveCount(30);
  // The focus stays inside while it is open.
  for (const key of ['Tab', 'Tab', 'Shift+Tab']) {
    await page.keyboard.press(key);
    await expect
      .poll(() => palette(page).evaluate((dialog) => dialog.contains(document.activeElement)))
      .toBe(true);
  }
  await paletteField(page).focus();
  // Escape closes; the focus is back on the button.
  await page.keyboard.press('Escape');
  await expect(palette(page)).toHaveCount(0);
  await expectFocusOn(page, finder(page));

  // By Ctrl+K: typing filters by title and area, ignoring case; the arrows choose.
  await page.keyboard.press('Control+k');
  await expect(palette(page)).toBeVisible();
  await expectFocusOn(page, paletteField(page));
  await page.keyboard.type('sicher');
  await expect(palette(page).getByRole('option')).toHaveText([/^Sicherung/]);
  await page.keyboard.press('Control+a');
  await page.keyboard.type('automatisierung');
  // The area itself and its three pages.
  await expect(palette(page).getByRole('option')).toHaveCount(4);
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('ArrowDown');
  await expect(palette(page).getByRole('option', { selected: true })).toHaveText(/^Integrationen/);
  await page.keyboard.press('Enter');
  await expect(palette(page)).toHaveCount(0);
  await expect(page).toHaveURL(/\/admin\/integrations$/);
  await expect(h1(page, 'Integrationen')).toBeVisible();

  // Nothing found.
  await page.keyboard.press('Control+k');
  await expectFocusOn(page, paletteField(page));
  await page.keyboard.type('xyz');
  await expect(palette(page).getByRole('option')).toHaveCount(0);
  await expect(palette(page).getByText('Nichts gefunden')).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(palette(page)).toHaveCount(0);

  // The media search is an entry; "/" keeps opening it from an admin page as well.
  await page.keyboard.press('Control+k');
  await page.keyboard.type('mediensuche');
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/search$/);
  await page.goBack();
  await expect(h1(page, 'Integrationen')).toBeVisible();
  await resetFocus(page);
  await page.keyboard.press('/');
  await expect(page).toHaveURL(/\/search$/);
});

test('Ctrl+K does not open the palette while a dialog is open', async ({ page }) => {
  await page.goto('/admin/libraries');
  await expect(h1(page, 'Bibliotheken')).toBeVisible();
  await page.getByRole('button', { name: 'Bibliothek hinzufügen' }).click();
  const dialog = page.getByRole('dialog', { name: 'Bibliothek hinzufügen' });
  await expect(dialog).toBeVisible();
  await page.keyboard.press('Control+k');
  await expect(palette(page)).toHaveCount(0);
  await page.keyboard.press('Escape');
  await expect(dialog).toHaveCount(0);
});

test('deep links of sub-pages show the tab of their page and open its area in the tree', async ({
  page,
}) => {
  await page.goto('/admin/metadata/audiobooks');
  await expect(h1(page, 'Hörbücher: Quellen')).toBeVisible();
  await expect(areaTab(page, 'Hörbuch-Quellen')).toHaveAttribute('aria-current', 'page');
  await expect(areaTab(page, 'Metadaten')).not.toHaveAttribute('aria-current', 'page');
  await expectTreeAt(page, 'Medien', 'Hörbuch-Quellen');

  await page.goto('/admin/podcasts/e2e-unknown-podcast');
  await expect(h1(page, 'Podcast nicht gefunden')).toBeVisible();
  await expect(areaTab(page, 'Podcasts')).toHaveAttribute('aria-current', 'page');
  await expectTreeAt(page, 'Medien', 'Podcasts');

  await page.goto(`/admin/items/${idOf(await itemPath(page, H264_TITLE))}`);
  await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
  await expect(areaTab(page, 'Pflege')).toHaveAttribute('aria-current', 'page');
  await expectTreeAt(page, 'Medien', 'Pflege');
  await resetFocus(page);
  await tabTo(page, areaTab(page, 'Pflege'), 20);
  await expectVisibleFocus(page);
});

// The Explorer tree of the column (from 768 px): each area a button that folds its pages in and
// out (aria-expanded), the current page's link aria-current="page", its area's button
// aria-current="true", open or closed. The column is one Tab stop over the shown items.

/** The list of an area's pages in the tree (aria-label is the area's name). */
const areaList = (page: Page, area: string) =>
  adminNav(page).getByRole('list', { name: area, exact: true });

/**
 * The tree stands at `name` of `area`: its area's button is open and current, the area's list
 * shows its pages in order with their paths, the page's link is the one current page of the nav
 * and shown. `othersClosed`: the other four areas are closed (as on a fresh load); else only not
 * current, open or closed as the viewer left them.
 */
async function expectTreeAt(
  page: Page,
  area: string,
  name: string,
  othersClosed = true,
): Promise<void> {
  const nav = adminNav(page);
  for (const [other] of AREAS) {
    const button = adminArea(page, other);
    if (other === area) {
      await expect(button).toHaveAttribute('aria-expanded', 'true');
      await expect(button).toHaveAttribute('aria-current', 'true');
    } else {
      await expect(button).not.toHaveAttribute('aria-current');
      if (othersClosed) await expect(button).toHaveAttribute('aria-expanded', 'false');
    }
  }
  const pages = AREAS.find(([candidate]) => candidate === area)?.[1] ?? [];
  const links = areaList(page, area).getByRole('link');
  await expect(links).toHaveCount(pages.length);
  for (const [index, [title, path]] of pages.entries()) {
    await expect(links.nth(index)).toHaveAccessibleName(new RegExp(`^${title}(,|$)`));
    await expect(links.nth(index)).toHaveAttribute('href', path);
  }
  const link = areaList(page, area).getByRole('link', { name: new RegExp(`^${name}(,|$)`) });
  await expect(link).toBeVisible();
  await expect(link).toHaveAttribute('aria-current', 'page');
  await expect(nav.locator('[aria-current="page"]')).toHaveCount(1);
}

/** The overview is current: its link, and no area's button. */
async function expectTreeAtOverview(page: Page): Promise<void> {
  await expect(adminEntry(page, 'Übersicht')).toHaveAttribute('aria-current', 'page');
  await expect(adminNav(page).locator('[aria-current="page"]')).toHaveCount(1);
  await expect(adminNav(page).locator('[aria-current="true"]')).toHaveCount(0);
}

test('the tree on the overview: five closed areas, back and the overview as the only links', async ({
  page,
}) => {
  await page.goto('/admin');
  await expect(h1(page, 'Alles im Blick')).toBeVisible();
  const nav = adminNav(page);
  await expect(nav.getByRole('link')).toHaveCount(2);
  await expect(nav.getByRole('link').nth(0)).toHaveAccessibleName('Zurück zur App');
  await expect(nav.getByRole('link').nth(1)).toHaveAccessibleName(/^Übersicht(,|$)/);
  await expectTreeAtOverview(page);
  const buttons = nav.getByRole('button');
  await expect(buttons).toHaveCount(AREAS.length);
  for (const [index, [area]] of AREAS.entries()) {
    await expect(buttons.nth(index)).toHaveAccessibleName(new RegExp(`^${area}(,|$)`));
    await expect(buttons.nth(index)).toHaveAttribute('aria-expanded', 'false');
    // The button names the list it folds; the list stays in the DOM, hidden.
    const list = page.locator(`[id="${await buttons.nth(index).getAttribute('aria-controls')}"]`);
    await expect(list).toHaveAttribute('aria-label', area);
    await expect(list).toBeHidden();
  }
});

test('every one of the 22 pages by deep link opens its area in the tree, the others closed', async ({
  page,
}) => {
  // 22 fresh loads, each checking five buttons and its area's list: about 50 s in Firefox.
  test.setTimeout(150_000);
  expect(PAGES).toHaveLength(22);
  for (const [area, name, path, heading] of PAGES) {
    await page.goto(path);
    await expect(h1(page, heading)).toBeVisible();
    await expectTreeAt(page, area, name);
  }
});

test('deep links with search params and a hash keep the address and mark the tree', async ({
  page,
}) => {
  for (const [path, heading, area, name] of [
    ['/admin/accounts?item=me#meine-profile', 'Konten', 'Nutzer & Geräte', 'Konten'],
    ['/admin/integrations?item=keys', 'Integrationen', 'Automatisierung', 'Integrationen'],
  ] as const) {
    await page.goto(path);
    await expect(h1(page, heading)).toBeVisible();
    await expectTreeAt(page, area, name);
    // Neither the page nor the tree rewrites the address.
    expect(page.url()).toBe(new URL(path, page.url()).href);
  }
});

test('every one of the 22 pages through the tree by keyboard from the overview @keyboard', async ({
  page,
}) => {
  // Five rounds from the overview, each opening an area and following all of its pages: 22 page
  // changes with their checks, about as long as the tabs test above.
  test.setTimeout(150_000);
  let reached = 0;
  for (const [areaIndex, [area, pages]] of AREAS.entries()) {
    await page.goto('/admin');
    await expect(h1(page, 'Alles im Blick')).toBeVisible();
    await resetFocus(page);
    // The overview is the Tab stop; all areas are closed, so each ↓ is the next area's button.
    await tabTo(page, adminEntry(page, 'Übersicht'));
    for (let presses = 0; presses <= areaIndex; presses += 1) {
      await page.keyboard.press('ArrowDown');
    }
    const button = adminArea(page, area);
    await expectFocusOn(page, button);
    // Enter and Space both open an area; opening it leaves the page as it is.
    await page.keyboard.press(areaIndex % 2 === 0 ? 'Enter' : 'Space');
    await expect(button).toHaveAttribute('aria-expanded', 'true');
    await expect(page).toHaveURL(/\/admin$/);
    await expectFocusOn(page, button);
    for (const [name, path, heading] of pages) {
      await page.keyboard.press('ArrowDown');
      const link = adminEntry(page, name);
      await expectFocusOn(page, link);
      await page.keyboard.press('Enter');
      await expect(page).toHaveURL(new RegExp(`${path}$`));
      await expect(h1(page, heading)).toBeVisible();
      await expectTreeAt(page, area, name);
      // The column stays and keeps the focus on the followed link.
      await expectFocusOn(page, link);
      reached += 1;
    }
  }
  expect(reached).toBe(22);
  // Tab leaves the column in one press, to the area's tabs; then on to the end of the page.
  await page.keyboard.press('Tab');
  await expectFocusOn(page, areaTab(page, 'Server'));
  await expectNoFocusTrap(page, 120);
});

test('the tree by keyboard: Space toggles without leaving the page, ↑ ↓ skip closed areas and wrap, Home End @keyboard', async ({
  page,
}) => {
  await page.goto('/admin');
  await expect(h1(page, 'Alles im Blick')).toBeVisible();
  const media = adminArea(page, 'Medien');
  const people = adminArea(page, 'Nutzer & Geräte');
  const system = adminArea(page, 'System');
  const back = adminEntry(page, 'Zurück zur App');
  await resetFocus(page);
  await tabTo(page, adminEntry(page, 'Übersicht'));
  // «Medien» is closed: ↓ goes from its button straight to the next area's.
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, media);
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, people);
  // The arrows wrap: ↑ from the top is the last area, ↓ from there the top again.
  await page.keyboard.press('Home');
  await expectFocusOn(page, back);
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, system);
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, back);
  await page.keyboard.press('End');
  await expectFocusOn(page, system);

  // Space opens «Medien»: the page and the address stay, ↓ now reaches its first page.
  await page.keyboard.press('Home');
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, media);
  await page.keyboard.press('Space');
  await expect(media).toHaveAttribute('aria-expanded', 'true');
  await expect(page).toHaveURL(/\/admin$/);
  await expect(h1(page, 'Alles im Blick')).toBeVisible();
  await expectFocusOn(page, media);
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, adminEntry(page, 'Bibliotheken'));
  // Space again closes it; the focus is on its button, ↓ skips its pages again.
  await page.keyboard.press('ArrowUp');
  await page.keyboard.press('Space');
  await expect(media).toHaveAttribute('aria-expanded', 'false');
  await expect(areaList(page, 'Medien')).toHaveCount(0);
  await expect(page).toHaveURL(/\/admin$/);
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, people);
  // Opened, the last area's last page is the end: End goes there, ↓ wraps to the top.
  await page.keyboard.press('End');
  await page.keyboard.press('Enter');
  await expect(system).toHaveAttribute('aria-expanded', 'true');
  await page.keyboard.press('End');
  await expectFocusOn(page, adminEntry(page, 'Server'));
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, back);
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, adminEntry(page, 'Server'));
  // Nothing of it changed the page.
  await expect(page).toHaveURL(/\/admin$/);
  await expectTreeAtOverview(page);
  // Tab leaves the column in one press: the overview has no area tabs, its header comes next.
  await page.keyboard.press('Tab');
  await expect(adminNav(page).locator(':focus')).toHaveCount(0);
  await expectVisibleFocus(page);
  await expectNoFocusTrap(page, 120);
});

test('an area closes while its page is current, and the tree keeps what the viewer opened', async ({
  page,
}) => {
  await page.goto('/admin/devices');
  await expect(h1(page, 'Geräte')).toBeVisible();
  const people = adminArea(page, 'Nutzer & Geräte');
  const media = adminArea(page, 'Medien');
  const devices = adminNav(page).getByRole('link', { name: /^Geräte(,|$)/, includeHidden: true });
  await expectTreeAt(page, 'Nutzer & Geräte', 'Geräte');

  // Closed, the area stays current; its page's link is folded away and still the current page.
  await people.click();
  await expect(people).toHaveAttribute('aria-expanded', 'false');
  await expect(people).toHaveAttribute('aria-current', 'true');
  await expect(devices).toBeHidden();
  await expect(devices).toHaveAttribute('aria-current', 'page');
  await expect(adminNav(page).locator('[aria-current="page"]')).toHaveCount(1);
  await expect(page).toHaveURL(/\/admin\/devices$/);
  await expect(h1(page, 'Geräte')).toBeVisible();
  // The Tab stop is then the area's button.
  await resetFocus(page);
  await tabTo(page, people);
  await expect(people).toHaveAttribute('aria-expanded', 'false');

  // A second area opened by the viewer stays open; the current area opens again once the page
  // changes, here by the area's tabs above the page.
  await media.click();
  await expect(media).toHaveAttribute('aria-expanded', 'true');
  await areaTab(page, 'Streams').click();
  await expect(page).toHaveURL(/\/admin\/streams$/);
  await expect(h1(page, 'Streams')).toBeVisible();
  await expectTreeAt(page, 'Nutzer & Geräte', 'Streams', false);
  await expect(media).toHaveAttribute('aria-expanded', 'true');

  // Closed by the viewer, «Medien» stays closed on the way to another page of «Nutzer & Geräte»
  // and to a page of another area.
  await media.click();
  await expect(media).toHaveAttribute('aria-expanded', 'false');
  await areaTab(page, 'Konten').click();
  await expect(h1(page, 'Konten')).toBeVisible();
  await expect(media).toHaveAttribute('aria-expanded', 'false');
  await adminArea(page, 'Startseite').click();
  await adminEntry(page, 'Spotlight').click();
  await expect(page).toHaveURL(/\/admin\/spotlight$/);
  await expect(h1(page, 'Spotlight')).toBeVisible();
  await expectTreeAt(page, 'Startseite', 'Spotlight', false);
  await expect(media).toHaveAttribute('aria-expanded', 'false');
  await expect(people).toHaveAttribute('aria-expanded', 'true');
});

test('the browser’s back and forward keep the tree on the page arrived at', async ({ page }) => {
  await page.goto('/admin');
  await expect(h1(page, 'Alles im Blick')).toBeVisible();
  const media = adminArea(page, 'Medien');
  const system = adminArea(page, 'System');
  await media.click();
  await adminEntry(page, 'Pflege').click();
  await expect(page).toHaveURL(/\/admin\/maintenance$/);
  await expect(h1(page, 'Pflege')).toBeVisible();
  await system.click();
  await adminEntry(page, 'Sicherung').click();
  await expect(page).toHaveURL(/\/admin\/backup$/);
  await expect(h1(page, 'Sicherung')).toBeVisible();
  await expectTreeAt(page, 'System', 'Sicherung', false);

  // Closed before going back, «Medien» opens again with its page.
  await media.click();
  await expect(media).toHaveAttribute('aria-expanded', 'false');
  await page.goBack();
  await expect(page).toHaveURL(/\/admin\/maintenance$/);
  await expect(h1(page, 'Pflege')).toBeVisible();
  await expectTreeAt(page, 'Medien', 'Pflege', false);
  await page.goBack();
  await expect(page).toHaveURL(/\/admin$/);
  await expect(h1(page, 'Alles im Blick')).toBeVisible();
  await expectTreeAtOverview(page);

  // Both closed on the overview; forward opens each page's area again.
  await media.click();
  await system.click();
  await expect(media).toHaveAttribute('aria-expanded', 'false');
  await expect(system).toHaveAttribute('aria-expanded', 'false');
  await page.goForward();
  await expect(page).toHaveURL(/\/admin\/maintenance$/);
  await expect(h1(page, 'Pflege')).toBeVisible();
  await expectTreeAt(page, 'Medien', 'Pflege');
  await page.goForward();
  await expect(page).toHaveURL(/\/admin\/backup$/);
  await expect(h1(page, 'Sicherung')).toBeVisible();
  await expectTreeAt(page, 'System', 'Sicherung', false);
  await expect(media).toHaveAttribute('aria-expanded', 'true');
});

test('the tree’s page links carry the same counts and pills as the area tabs', async ({ page }) => {
  /** Spoken names of a page's tab and of its link in the tree, read in one go (same render). */
  const names = (path: string) =>
    page.evaluate((href) => {
      const spoken = (element: Element | null) =>
        element?.getAttribute('aria-label') ??
        element?.textContent?.trim().replace(/\s+/g, ' ') ??
        null;
      const tab = document.querySelector(`nav[aria-label^="Seiten im Bereich "] a[href="${href}"]`);
      const link = document.querySelector(`nav[aria-label="Verwaltung"] a[href="${href}"]`);
      return { tab: spoken(tab), link: spoken(link) };
    }, path);
  // With the server's data these three pages always have a figure: counted titles, the accounts,
  // set-up integrations (or silent webhooks). The others show theirs only when there is one.
  for (const [area, figured, figure] of [
    ['Medien', 'Bibliotheken', /^Bibliotheken, [\d’'.]+ Titel$/],
    ['Nutzer & Geräte', 'Konten', /^Konten, [\d’'.]+ (Konto|Konten)$/],
    [
      'Automatisierung',
      'Integrationen',
      /^Integrationen, (\d+ von \d+ eingerichtet|[\d’'.]+ Webhooks? still)$/,
    ],
  ] as const) {
    const pages = AREAS.find(([candidate]) => candidate === area)?.[1] ?? [];
    const [first] = pages;
    if (!first) throw new Error(`no page in ${area}`);
    await page.goto(first[1]);
    await expect(h1(page, first[2])).toBeVisible();
    // The numbers come with the dashboard, a moment after the page.
    await expect(areaTab(page, figured)).toHaveAccessibleName(figure);
    await expect(adminEntry(page, figured)).toHaveAccessibleName(figure);
    for (const [name, path] of pages) {
      await expect
        .poll(async () => {
          const { tab, link } = await names(path);
          return tab === link ? tab : `tab «${tab}», tree link «${link}»`;
        }, `«${name}»: the tab and the tree link say the same`)
        .toMatch(new RegExp(`^${name}(,|$)`));
    }
  }
});

test.describe('on a tablet', () => {
  test.use({ viewport: { width: 768, height: 1024 } });

  test('the column with the tree stands from 768 px; a deep link opens its area, a click folds it', async ({
    page,
  }) => {
    await page.goto('/admin/streams');
    await expect(h1(page, 'Streams')).toBeVisible();
    await expect(adminNav(page)).toBeVisible();
    await expectTreeAt(page, 'Nutzer & Geräte', 'Streams');
    const media = adminArea(page, 'Medien');
    await media.click();
    await expect(media).toHaveAttribute('aria-expanded', 'true');
    await expect(adminEntry(page, 'Bibliotheken')).toBeVisible();
    await media.click();
    await expect(media).toHaveAttribute('aria-expanded', 'false');
    await expect(areaList(page, 'Medien')).toHaveCount(0);
    await expect(page).toHaveURL(/\/admin\/streams$/);
    // With all five areas open the column scrolls on its own; the page does not widen.
    for (const [area] of AREAS) {
      const button = adminArea(page, area);
      if ((await button.getAttribute('aria-expanded')) !== 'true') await button.click();
      await expect(button).toHaveAttribute('aria-expanded', 'true');
    }
    await expect
      .poll(() => page.evaluate(() => document.documentElement.scrollWidth))
      .toBeLessThanOrEqual(768);
  });
});

test.describe('on a phone', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('below 768 px there is no column and no tree', async ({ page }) => {
    await page.goto('/admin/libraries');
    await expect(h1(page, 'Bibliotheken')).toBeVisible();
    await expect(page.getByRole('button', { name: /^Verwaltung · / })).toBeVisible();
    await expect(adminNav(page)).toHaveCount(0);
    await expect(page.getByRole('button', { name: /^Medien(,|$)/ })).toHaveCount(0);
  });

  test('the sheet reaches every page; the tabs stand above the page and wrap', async ({ page }) => {
    test.setTimeout(150_000);
    await page.goto('/admin');
    await expect(h1(page, 'Alles im Blick')).toBeVisible();
    for (const [area, name, path, heading] of PAGES) {
      const button = page.getByRole('button', { name: /^Verwaltung · / });
      await resetFocus(page);
      await tabTo(page, button, 12);
      await page.keyboard.press('Enter');
      const sheet = page.getByRole('dialog', { name: 'Seiten der Verwaltung' });
      await expect(sheet).toBeVisible();
      await expect(sheet.getByRole('link')).toHaveCount(24);
      const link = sheet
        .getByRole('list', { name: area })
        .getByRole('link', { name: new RegExp(`^${name}(,|$)`) });
      for (let presses = 0; presses < 24; presses += 1) {
        if (await link.evaluate((element) => element === document.activeElement)) break;
        await page.keyboard.press('ArrowDown');
      }
      await expectFocusOn(page, link);
      await page.keyboard.press('Enter');
      await expect(page).toHaveURL(new RegExp(`${path}$`));
      await expect(sheet).toHaveCount(0);
      await expect(h1(page, heading)).toBeVisible();
      await expect(
        page.getByRole('button', { name: `Verwaltung · ${area} / ${name}` }),
      ).toBeVisible();
      await expect(areaTab(page, name)).toHaveAttribute('aria-current', 'page');
    }
    // The tabs of «Medien» wrap onto more lines instead of widening the page.
    await page.goto('/admin/libraries');
    await expect(h1(page, 'Bibliotheken')).toBeVisible();
    await expect
      .poll(() => page.evaluate(() => document.documentElement.scrollWidth))
      .toBeLessThanOrEqual(390);
    const tabsBox = await areaTabs(page).boundingBox();
    expect(tabsBox?.height ?? 0).toBeGreaterThan(50);
    await expectNoFocusTrap(page, 120);
  });
});
