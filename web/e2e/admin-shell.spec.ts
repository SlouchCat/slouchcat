import type { Page } from '@playwright/test';
import { adminEntry, adminNav, areaTab, areaTabs } from './admin';
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

test('the sidebar: back, the overview and five areas; one Tab stop and the arrows', async ({
  page,
}) => {
  await page.goto('/admin/jobs');
  await expect(h1(page, 'Hintergrundjobs')).toBeVisible();
  const nav = adminNav(page);
  await expect(nav.getByRole('link')).toHaveCount(7);
  await expect(nav.getByRole('list', { name: 'Verwaltung' }).getByRole('link')).toHaveCount(6);
  const automation = adminEntry(page, 'Automatisierung');
  await expect(automation).toHaveAttribute('aria-current', 'page');
  await expect(nav.locator('[aria-current="page"]')).toHaveCount(1);
  // The active profile with its role at the foot of the column, outside the navigation.
  await expect(page.getByText('Server-Admin', { exact: true })).toBeVisible();

  await resetFocus(page);
  await tabTo(page, automation);
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, adminEntry(page, 'Startseite'));
  await page.keyboard.press('Home');
  await expectFocusOn(page, adminEntry(page, 'Zurück zur App'));
  await page.keyboard.press('End');
  await expectFocusOn(page, adminEntry(page, 'System'));
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin\/stats$/);
  await expect(h1(page, 'Statistiken')).toBeVisible();
  // The sidebar stays and keeps the focus on the followed entry (focusCarry).
  await expectFocusOn(page, adminEntry(page, 'System'));
  await expect(adminEntry(page, 'System')).toHaveAttribute('aria-current', 'page');
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
      await expect(adminEntry(page, area)).toHaveAttribute('aria-current', 'page');
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

test('deep links of sub-pages show the tab of their page', async ({ page }) => {
  await page.goto('/admin/metadata/audiobooks');
  await expect(h1(page, 'Hörbücher: Quellen')).toBeVisible();
  await expect(areaTab(page, 'Hörbuch-Quellen')).toHaveAttribute('aria-current', 'page');
  await expect(areaTab(page, 'Metadaten')).not.toHaveAttribute('aria-current', 'page');

  await page.goto('/admin/podcasts/e2e-unknown-podcast');
  await expect(h1(page, 'Podcast nicht gefunden')).toBeVisible();
  await expect(areaTab(page, 'Podcasts')).toHaveAttribute('aria-current', 'page');

  await page.goto(`/admin/items/${idOf(await itemPath(page, H264_TITLE))}`);
  await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
  await expect(areaTab(page, 'Pflege')).toHaveAttribute('aria-current', 'page');
  await expect(adminEntry(page, 'Medien')).toHaveAttribute('aria-current', 'page');
  await resetFocus(page);
  await tabTo(page, areaTab(page, 'Pflege'), 20);
  await expectVisibleFocus(page);
});

test.describe('on a phone', () => {
  test.use({ viewport: { width: 390, height: 844 } });

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
