import type { Locator, Page } from '@playwright/test';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// The administration (concept 11, A-2, A-3; redesign, build plan 2): the app column has one entry
// "Verwaltung"; on every /admin page the administration's sidebar stands in its place (back, the
// overview and the five areas: one Tab stop, ↑ ↓ Home End, wrapping), on a phone a button opens
// every page as a sheet. The pages of an area are tabs above each of its pages (admin-shell.spec).
// The overview (build plan 3) shows the real server's dashboard, polled every 5 s: the band, four
// key figures, what needs attention, what is active, the areas and the server in short. The
// server's functions in full are on System › Server (admin-server.spec).

const adminNav = (page: Page) => page.getByRole('navigation', { name: 'Verwaltung' });
const appNav = (page: Page) => page.getByRole('navigation', { name: 'Hauptnavigation' });
const card = (page: Page, name: string) => page.getByRole('region', { name, exact: true });

/** The sidebar's entries, in their order, with their paths: back, the overview, the areas. */
const NAV_ENTRIES = [
  ['Zurück zur App', '/'],
  ['Übersicht', '/admin'],
  ['Medien', '/admin/libraries'],
  ['Nutzer & Geräte', '/admin/accounts'],
  ['Startseite', '/admin/home-rows'],
  ['Automatisierung', '/admin/jobs'],
  ['System', '/admin/stats'],
] as const;

/** The first page of every area: the heading its sidebar entry leads to. */
const AREA_HEADINGS: Record<string, string> = {
  Medien: 'Bibliotheken',
  'Nutzer & Geräte': 'Konten',
  Startseite: 'Startseite',
  Automatisierung: 'Hintergrundjobs',
  System: 'Statistiken',
};

/**
 * The sheet on a phone: back, the overview and every page of the five areas (11.1: 19 pages;
 * concept 16 adds «Radio», concept 21 «Meine Medien», the redesign «Hörbuch-Quellen» and
 * «Server»).
 */
const SHEET_LINKS = 23;

/** An entry of the sidebar by its label; numbers and pills may follow in its spoken name. */
const entry = (page: Page, name: string) =>
  adminNav(page).getByRole('link', { name: new RegExp(`^${name}(,|$)`) });

/** The labels and paths of the sidebar's links, as shown (without their numbers). */
const shownEntries = (nav: Locator) =>
  nav
    .getByRole('link')
    .evaluateAll((links) =>
      links.map((link) => [link.querySelector('span')?.textContent, link.getAttribute('href')]),
    );

async function openOverview(page: Page) {
  await page.goto('/admin');
  await expect(page.getByRole('heading', { level: 1, name: 'Alles im Blick' })).toBeVisible();
  await expect(entry(page, 'Übersicht')).toHaveAttribute('aria-current', 'page');
}

/** Presses ↓ in the sidebar until `link` has the focus. */
async function arrowTo(page: Page, link: Locator): Promise<void> {
  for (let presses = 0; presses < NAV_ENTRIES.length; presses += 1) {
    if (await link.evaluate((element) => element === document.activeElement)) break;
    await page.keyboard.press('ArrowDown');
  }
  await expectFocusOn(page, link);
}

test('the app column has one "Verwaltung" entry; Enter opens the overview in the admin sidebar', async ({
  page,
}) => {
  await page.goto('/');
  await expect(page.getByRole('heading', { level: 2, name: 'Neu hinzugefügt' })).toBeVisible();
  const app = appNav(page).getByRole('link', { name: 'Verwaltung' });
  await expect(app).toHaveAttribute('href', '/admin');
  // The admin pages are not in the app column.
  for (const name of ['Bibliotheken', 'Geräte', 'Streams', 'Startseite']) {
    await expect(appNav(page).getByRole('link', { name, exact: true })).toHaveCount(0);
  }
  await resetFocus(page);
  await tabTo(page, app);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Alles im Blick' })).toBeVisible();
  // The sidebar of the administration takes the column's place.
  await expect(adminNav(page)).toBeVisible();
  await expect(appNav(page)).toHaveCount(0);
  await expect(entry(page, 'Übersicht')).toHaveAttribute('aria-current', 'page');
  // The focus went with the app column: the sidebar takes it on «Übersicht».
  await expectFocusOn(page, entry(page, 'Übersicht'));
  // «Zurück zur App» leads back, where the app column stands again with the focus on «Start».
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, entry(page, 'Zurück zur App'));
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/$/);
  await expect(appNav(page)).toBeVisible();
  await expect(adminNav(page)).toHaveCount(0);
  await expectFocusOn(page, appNav(page).getByRole('link', { name: 'Start', exact: true }));
});

test('the focus stays on «Übersicht» after entering the administration with Enter', async ({
  page,
}) => {
  await page.goto('/');
  await expect(page.getByRole('heading', { level: 2, name: 'Neu hinzugefügt' })).toBeVisible();
  await resetFocus(page);
  await tabTo(page, appNav(page).getByRole('link', { name: 'Verwaltung' }));
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin$/);
  await expectFocusOn(page, entry(page, 'Übersicht'));
  // norigin restores the focus of a removed element 300 ms later (its auto-restore); the app
  // column that had the focus is gone by now, and that restore must not pull the focus out of the
  // administration's sidebar into the header. Wait past it, then check where the focus is.
  await page.waitForTimeout(800);
  await expectFocusOn(page, entry(page, 'Übersicht'));
  // A slow machine used to hit this by chance: ↑ then Enter went to the bell, not to the app.
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, entry(page, 'Zurück zur App'));
});

test('admin sidebar: 7 entries, one Tab stop, ↑ ↓ Home End, Enter follows, Back returns', async ({
  page,
}) => {
  await page.goto('/admin/devices');
  await expect(page.getByRole('heading', { level: 1, name: 'Geräte' })).toBeVisible();
  const nav = adminNav(page);
  await expect.poll(() => shownEntries(nav)).toEqual(NAV_ENTRIES.map((pair) => [...pair]));
  // The area of the open page is current.
  const people = entry(page, 'Nutzer & Geräte');
  await expect(people).toHaveAttribute('aria-current', 'page');

  // From the top: the header, then the sidebar's current area.
  await resetFocus(page);
  await tabTo(page, people);
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, entry(page, 'Startseite'));
  await page.keyboard.press('ArrowUp');
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, entry(page, 'Medien'));
  await page.keyboard.press('End');
  await expectFocusOn(page, entry(page, 'System'));
  // The arrows wrap around.
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, entry(page, 'Zurück zur App'));
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, entry(page, 'System'));
  await page.keyboard.press('Home');
  await expectFocusOn(page, entry(page, 'Zurück zur App'));

  // Tab leaves the sidebar at once (one Tab stop), Shift+Tab comes back to the current area.
  await page.keyboard.press('Tab');
  await expect(nav.locator(':focus')).toHaveCount(0);
  await page.keyboard.press('Shift+Tab');
  await expectFocusOn(page, people);

  // Enter follows a link to the area's first page; the sidebar stays and keeps the focus; the
  // browser's Back returns.
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin\/home-rows$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Startseite' })).toBeVisible();
  await expect(entry(page, 'Startseite')).toHaveAttribute('aria-current', 'page');
  await expectFocusOn(page, entry(page, 'Startseite'));
  await page.goBack();
  await expect(page).toHaveURL(/\/admin\/devices$/);
  await expect(people).toHaveAttribute('aria-current', 'page');
  await expectNoFocusTrap(page);
});

test('every area is reached through the sidebar (A-2)', async ({ page }) => {
  test.setTimeout(90_000);
  await openOverview(page);
  await resetFocus(page);
  await tabTo(page, entry(page, 'Übersicht'));
  for (const [name, path] of NAV_ENTRIES.slice(2)) {
    const link = entry(page, name);
    await arrowTo(page, link);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(new RegExp(`${path}$`));
    // Every area opens its first page; the sidebar keeps the focus on the followed link.
    await expect(
      page.getByRole('heading', { level: 1, name: AREA_HEADINGS[name], exact: true }),
    ).toBeVisible();
    await expect(link).toHaveAttribute('aria-current', 'page');
    await expectFocusOn(page, link);
  }
});

test('the overview: band, four key figures, attention, active, areas, the server in short (A-3)', async ({
  page,
}) => {
  await openOverview(page);
  await expect(page.getByText('Live, alle 5 Sekunden')).toBeVisible();
  const metrics = card(page, 'Kennzahlen');
  await expect(metrics.getByRole('link')).toHaveCount(4);
  await expect(metrics.getByRole('link', { name: /^Aktive Streams/ })).toHaveAttribute(
    'href',
    '/admin/streams',
  );
  await expect(metrics.getByRole('link', { name: /^Laufende Jobs/ })).toHaveAttribute(
    'href',
    '/admin/jobs',
  );
  await expect(metrics.getByRole('link', { name: /^Titel zur Pflege/ })).toHaveAttribute(
    'href',
    '/admin/maintenance',
  );
  // F-4: both paths in one figure, linked to the server's page.
  await expect(metrics.getByRole('link', { name: /^Speicherplatz \d+ % · \d+ %/ })).toHaveAttribute(
    'href',
    '/admin/server',
  );
  // Two cards side by side at 1280 px with the sidebar, equally high (P-1).
  const attention = card(page, 'Was benötigt Aufmerksamkeit?');
  const active = card(page, 'Gerade aktiv');
  const [left, right] = await Promise.all([attention.boundingBox(), active.boundingBox()]);
  expect(left && right).toBeTruthy();
  expect(Math.abs(left!.y - right!.y)).toBeLessThan(1);
  expect(Math.abs(left!.height - right!.height)).toBeLessThan(1);
  expect(right!.x).toBeGreaterThan(left!.x + left!.width);
  await expect(attention.getByRole('status').first()).toHaveText(/\d+ Hinweise?$/);
  await expect(active.getByRole('link', { name: 'Alle Streams' })).toHaveAttribute(
    'href',
    '/admin/streams',
  );
  await expect(active.getByRole('link', { name: 'Hintergrundjobs' })).toHaveAttribute(
    'href',
    '/admin/jobs',
  );
  await expect(card(page, 'Direkt zu deinen Aufgaben').getByRole('link')).toHaveCount(5);
  const summary = card(page, 'Server & letzte Aktivitäten');
  await expect(summary.getByRole('link', { name: 'Serverdetails' }).first()).toHaveAttribute(
    'href',
    '/admin/server',
  );
  const state = card(page, 'Serverzustand');
  // start-server.mjs passes the Trakt mock's client id: «x von y eingerichtet».
  await expect(state.getByText(/^\d+ von \d+ eingerichtet$/)).toBeVisible();
  await expect(state.getByText(/^(\d+ Fehler|keine Fehler)$/)).toBeVisible();
  await expect(state.getByRole('link', { name: 'Webhooks verwalten' })).toBeVisible();
  // The suite's setup scanned the libraries and the server came up: the activity log has entries.
  await expect(
    page.getByRole('list', { name: 'Letzte Aktivität' }).getByRole('listitem').first(),
  ).toBeVisible();
  // The server's functions moved to System › Server.
  await expect(page.getByRole('button', { name: 'Neu starten' })).toHaveCount(0);
});

test('Tab: header, sidebar, the overview’s header, key figures; Enter follows, Back returns; no focus trap @keyboard', async ({
  page,
}) => {
  await openOverview(page);
  await resetFocus(page);
  await tabTo(page, entry(page, 'Übersicht'));
  // One Tab stop: the next Tab is in the content, at the header's «Aktualisieren».
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('button', { name: 'Aktualisieren' }));
  await page.keyboard.press('Enter');
  await expect(page.getByRole('button', { name: 'Aktualisieren' })).toBeFocused();
  // On to the key figures: the streams figure leads to its page, Back returns.
  const streams = card(page, 'Kennzahlen').getByRole('link', { name: /^Aktive Streams/ });
  await tabTo(page, streams, 5);
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin\/streams$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Streams' })).toBeVisible();
  await page.goBack();
  await expect(page).toHaveURL(/\/admin$/);
  await expect(card(page, 'Kennzahlen')).toBeVisible();
  // «Serverdetails» leads to System › Server.
  await resetFocus(page);
  const details = card(page, 'Server & letzte Aktivitäten')
    .getByRole('link', { name: 'Serverdetails' })
    .first();
  await tabTo(page, details, 80);
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin\/server$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Server' })).toBeVisible();
  await page.goBack();
  await expect(page).toHaveURL(/\/admin$/);
  await resetFocus(page);
  await expectNoFocusTrap(page, 120);
});

test('on a phone: «Verwaltung · Übersicht» opens every page as a sheet, Escape returns', async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/admin');
  await expect(page.getByRole('heading', { level: 1, name: 'Alles im Blick' })).toBeVisible();
  // Below 768 px the sidebar lives in the sheet only.
  await expect(adminNav(page)).toHaveCount(0);
  await expect(appNav(page)).toHaveCount(0);
  const button = page.getByRole('button', { name: 'Verwaltung · Übersicht' });
  await resetFocus(page);
  await tabTo(page, button, 10);
  await page.keyboard.press('Enter');
  const sheet = page.getByRole('dialog', { name: 'Seiten der Verwaltung' });
  await expect(sheet).toBeVisible();
  await expect(sheet.getByRole('link')).toHaveCount(SHEET_LINKS);
  // The current page has the focus; the arrows move; the focus stays inside while it is open.
  await expectFocusOn(page, entry(page, 'Übersicht'));
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, entry(page, 'Bibliotheken'));
  // Tab and Shift+Tab cycle inside the sheet while it is open.
  for (const key of ['Tab', 'Tab', 'Tab', 'Shift+Tab', 'Shift+Tab', 'Shift+Tab', 'Shift+Tab']) {
    await page.keyboard.press(key);
    await expect
      .poll(() => sheet.evaluate((dialog) => dialog.contains(document.activeElement)))
      .toBe(true);
  }
  await page.keyboard.press('Escape');
  await expect(sheet).toHaveCount(0);
  await expectFocusOn(page, button);

  // Enter on an entry follows it and closes the sheet.
  await page.keyboard.press('Enter');
  await expect(sheet).toBeVisible();
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin\/libraries$/);
  await expect(sheet).toHaveCount(0);
  await expect(page.getByRole('heading', { level: 1, name: 'Bibliotheken' })).toBeVisible();
  await expect(
    page.getByRole('button', { name: 'Verwaltung · Medien / Bibliotheken' }),
  ).toBeVisible();
  await resetFocus(page);
  await expectNoFocusTrap(page, 120);
});
