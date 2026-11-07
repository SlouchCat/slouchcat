import type { Locator, Page } from '@playwright/test';
import { refreshSpotlight, traktMockStatus } from './discover';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// The administration (concept 11, A-2, A-3): the app column has one entry "Verwaltung"; on every
// /admin page the administration's sidebar stands in its place (19 entries, one Tab stop,
// ↑ ↓ Home End, wrapping), on a phone a button opens it as a sheet. The overview shows eight
// cards of the real server (polled every 5 s) and the server log collapsed below them.

interface TraktData {
  lastError?: string;
}

const adminNav = (page: Page) => page.getByRole('navigation', { name: 'Verwaltung' });
const appNav = (page: Page) => page.getByRole('navigation', { name: 'Hauptnavigation' });
const card = (page: Page, name: string) => page.getByRole('region', { name, exact: true });
const serverLog = (page: Page) => card(page, 'Server-Protokoll');
const errorList = (page: Page) =>
  serverLog(page).getByRole('list', { name: 'Fehler aus dem Server-Log', includeHidden: true });

/** Every entry of the sidebar, in its order, with its path (11.1: 19 entries). */
const NAV_ENTRIES = [
  ['Zurück zur App', '/'],
  ['Übersicht', '/admin'],
  ['Bibliotheken', '/admin/libraries'],
  ['Podcasts', '/admin/podcasts'],
  ['Pflege', '/admin/maintenance'],
  ['Metadaten', '/admin/metadata'],
  ['Konten', '/admin/accounts'],
  ['Geräte', '/admin/devices'],
  ['Streams', '/admin/streams'],
  ['Spotlight', '/admin/spotlight'],
  ["Cat's Empfehlungen", '/admin/picks'],
  ['Sammlungen', '/admin/collections'],
  ['Startseite', '/admin/home-rows'],
  ['Ankündigungen', '/admin/announcements'],
  ['Hintergrundjobs', '/admin/jobs'],
  ['Integrationen', '/admin/integrations'],
  ['Benachrichtigungen', '/admin/notifications'],
  ['Statistiken', '/admin/stats'],
  ['Sicherung', '/admin/backup'],
] as const;

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
  await expect(page.getByRole('heading', { level: 1, name: 'Verwaltung' })).toBeVisible();
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
  await expect(page.getByRole('heading', { level: 1, name: 'Verwaltung' })).toBeVisible();
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

test('admin sidebar: 19 entries, one Tab stop, ↑ ↓ Home End, Enter follows, Back returns', async ({
  page,
}) => {
  await page.goto('/admin/devices');
  await expect(page.getByRole('heading', { level: 1, name: 'Geräte' })).toBeVisible();
  const nav = adminNav(page);
  await expect.poll(() => shownEntries(nav)).toEqual(NAV_ENTRIES.map((pair) => [...pair]));
  const devices = entry(page, 'Geräte');
  await expect(devices).toHaveAttribute('aria-current', 'page');

  // From the top: the header, then the sidebar's current page.
  await resetFocus(page);
  await tabTo(page, devices);
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, entry(page, 'Streams'));
  await page.keyboard.press('ArrowUp');
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, entry(page, 'Konten'));
  await page.keyboard.press('End');
  await expectFocusOn(page, entry(page, 'Sicherung'));
  // The arrows wrap around.
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, entry(page, 'Zurück zur App'));
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, entry(page, 'Sicherung'));
  await page.keyboard.press('Home');
  await expectFocusOn(page, entry(page, 'Zurück zur App'));

  // Tab leaves the sidebar at once (one Tab stop), Shift+Tab comes back to the current page.
  await page.keyboard.press('Tab');
  await expect(nav.locator(':focus')).toHaveCount(0);
  await page.keyboard.press('Shift+Tab');
  await expectFocusOn(page, devices);

  // Enter follows a link; the sidebar stays and keeps the focus; the browser's Back returns.
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin\/streams$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Streams' })).toBeVisible();
  await expect(entry(page, 'Streams')).toHaveAttribute('aria-current', 'page');
  await expectFocusOn(page, entry(page, 'Streams'));
  await page.goBack();
  await expect(page).toHaveURL(/\/admin\/devices$/);
  await expect(devices).toHaveAttribute('aria-current', 'page');
  await expectNoFocusTrap(page);
});

test('every admin page is reached through the sidebar (A-2)', async ({ page }) => {
  test.setTimeout(90_000);
  await openOverview(page);
  await resetFocus(page);
  await tabTo(page, entry(page, 'Übersicht'));
  for (const [name, path] of NAV_ENTRIES.slice(2)) {
    const link = entry(page, name);
    await arrowTo(page, link);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(new RegExp(`${path}$`));
    // Every page has the heading of its entry; the sidebar keeps the focus on the followed link.
    // «Benachrichtigungen» opens the account's page, whose heading is «Benachrichtigungskanäle».
    const heading = name === 'Benachrichtigungen' ? 'Benachrichtigungskanäle' : name;
    await expect(page.getByRole('heading', { level: 1, name: heading, exact: true })).toBeVisible();
    await expect(link).toHaveAttribute('aria-current', 'page');
    await expectFocusOn(page, link);
  }
});

test('the overview: eight equal cards in two columns, the server log collapsed (A-3)', async ({
  page,
}) => {
  await openOverview(page);
  const titles = [
    'Server',
    'Jetzt läuft',
    'Aufgaben',
    'Letzte Aktivität',
    'Integrationen',
    'Webhooks',
    'Speicher',
    'Hinweise',
  ];
  await expect(page.locator('main section h2')).toHaveText(titles);
  // Two columns, each row equally high (P-1), at 1280 px with the sidebar.
  const boxes = await Promise.all(titles.map((name) => card(page, name).boundingBox()));
  for (let row = 0; row < 4; row += 1) {
    const [left, right] = [boxes[row * 2], boxes[row * 2 + 1]];
    expect(left && right).toBeTruthy();
    expect(Math.abs(left!.y - right!.y)).toBeLessThan(1);
    expect(Math.abs(left!.height - right!.height)).toBeLessThan(1);
    expect(right!.x).toBeGreaterThan(left!.x + left!.width);
  }

  const server = card(page, 'Server');
  await expect(server.getByText('Videokodierung')).toBeVisible();
  await expect(server.getByText('Beobachtete Ordner')).toBeVisible();
  await expect(server.getByRole('button', { name: 'Nach Updates suchen' })).toBeVisible();
  await expect(server.getByRole('button', { name: 'Neu starten' })).toBeVisible();
  await expect(
    card(page, 'Jetzt läuft').getByRole('link', { name: 'Alle Streams' }),
  ).toHaveAttribute('href', '/admin/streams');
  const tasks = card(page, 'Aufgaben');
  for (const counter of ['Wartend', 'Laufend', 'Fehlgeschlagen']) {
    await expect(tasks.getByText(counter, { exact: true })).toBeVisible();
  }
  await expect(tasks.getByRole('link', { name: 'Hintergrundjobs' })).toHaveAttribute(
    'href',
    '/admin/jobs',
  );
  // The suite's setup scanned the libraries and the server came up: the activity log has entries.
  await expect(
    page.getByRole('list', { name: 'Letzte Aktivität' }).getByRole('listitem').first(),
  ).toBeVisible();
  const storage = card(page, 'Speicher');
  await expect(storage.getByRole('meter', { name: /^Konfiguration/ })).toBeVisible();
  await expect(storage.getByRole('meter', { name: /^Cache/ })).toBeVisible();
  await expect(storage.getByText(/^Frei: /)).toHaveCount(2);
  // start-server.mjs passes the Trakt mock's client id: the key comes from the environment.
  const integrations = card(page, 'Integrationen');
  await expect(integrations.getByText(/^Trakt: /)).toBeVisible();
  await expect(integrations.getByText(/^TMDB: /)).toBeVisible();
  await expect(
    card(page, 'Webhooks').getByRole('link', { name: 'Webhooks verwalten' }),
  ).toBeVisible();
  await expect(card(page, 'Hinweise').getByRole('status').first()).toHaveText(/\d+ Hinweise?$/);

  // The server log: collapsed with its number; its list is not live itself, a poll with new
  // errors announces only their count.
  await expect(serverLog(page).locator('summary')).toHaveText(/^Server-Protokoll · /);
  await expect(errorList(page)).toBeHidden();
  await expect(errorList(page)).not.toHaveAttribute('aria-live');
  await expect(serverLog(page).getByRole('status')).toHaveCount(1);
});

test('a provoked error shows in the card and the opened server log within 10 s (A-3)', async ({
  page,
  owner,
}) => {
  // The refreshes wait at most the suite's 2 s limit (discover.ts refreshSpotlight); the poll for
  // the repaired Trakt state takes up to 60 s.
  test.setTimeout(120_000);
  await openOverview(page);
  const integrations = card(page, 'Integrationen');
  await expect(integrations.getByText(/^Trakt: /)).toBeVisible();
  // Open the server log by keyboard.
  const summary = serverLog(page).locator('summary');
  await summary.focus();
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  await expect(serverLog(page).locator('details')).toHaveAttribute('open', '');
  try {
    // Trakt is down; the spotlight refresh runs the job at once.
    await traktMockStatus(503);
    const provoked = Date.now();
    await refreshSpotlight(owner);
    await expect(integrations.getByText('Trakt: Fehler')).toBeVisible({ timeout: 10_000 });
    await expect(errorList(page)).toContainText(/trakt/i, {
      timeout: Math.max(1000, 10_000 - (Date.now() - provoked)),
    });
    await expect(errorList(page)).toBeVisible();
    // The poll that brought it announces a short count, not the whole list.
    await expect(serverLog(page).getByRole('status')).toHaveText(/^\d+ neuer? Fehler$/);
  } finally {
    // A failed refresh empties the Trakt lists (I-7): fetch them again for the later specs.
    await traktMockStatus(200);
    await refreshSpotlight(owner);
    await expect
      .poll(
        async () =>
          ((await (await owner.get('/api/v1/admin/trakt')).json()) as TraktData).lastError,
        { timeout: 60_000, intervals: [1000] },
      )
      .toBeFalsy();
  }
});

test('Tab: header, sidebar, cards; the restart dialog up to «Abbrechen»; no focus trap @keyboard', async ({
  page,
}) => {
  // Never a real restart: the suite's server must stay up. Should the confirmation be pressed by
  // mistake, the request goes nowhere and the test fails below.
  const restarts: string[] = [];
  await page.route('**/api/v1/admin/server/restart', async (route) => {
    restarts.push(route.request().method());
    await route.abort();
  });
  await openOverview(page);
  await resetFocus(page);
  await tabTo(page, entry(page, 'Übersicht'));
  // One Tab stop: the next Tab is in the content, at the server card's buttons.
  await page.keyboard.press('Tab');
  const server = card(page, 'Server');
  await expectFocusOn(page, server.getByRole('button', { name: 'Nach Updates suchen' }));
  const restart = server.getByRole('button', { name: 'Neu starten' });
  await page.keyboard.press('Tab');
  await expectFocusOn(page, restart);
  await page.keyboard.press('Enter');
  const dialog = page.getByRole('alertdialog', { name: 'Server neu starten?' });
  await expect(dialog).toContainText('Laufende Downloads für unterwegs werden unterbrochen.');
  await expect(dialog).toContainText(/Gerade läuft nichts\.|laufender? Streams? brechen? ab\./);
  // The focus is inside, on the confirmation; Shift+Tab goes to «Abbrechen».
  await expectFocusOn(page, dialog.getByRole('button', { name: 'Neu starten' }));
  await page.keyboard.press('Shift+Tab');
  await expectFocusOn(page, dialog.getByRole('button', { name: 'Abbrechen' }));
  await page.keyboard.press('Enter');
  await expect(dialog).toHaveCount(0);
  await expectFocusOn(page, restart);
  expect(restarts).toEqual([]);
  await expect(server).toBeVisible();

  // On through the cards: the streams link leads to its page, Back returns.
  const allStreams = card(page, 'Jetzt läuft').getByRole('link', { name: 'Alle Streams' });
  await tabTo(page, allStreams, 5);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin\/streams$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Streams' })).toBeVisible();
  await page.goBack();
  await expect(page).toHaveURL(/\/admin$/);
  await expect(card(page, 'Server')).toBeVisible();
  // The server log's summary is the last stop of the cards' column.
  await resetFocus(page);
  await tabTo(page, serverLog(page).locator('summary'), 80);
  await expectVisibleFocus(page);
  await resetFocus(page);
  await expectNoFocusTrap(page, 120);
});

test.describe('update card (concept 15 P-8)', () => {
  // Chromium sends the page's GET requests through the service worker, where page.route does not
  // see them; the dashboard is answered here, so the worker stays out.
  test.use({ serviceWorkers: 'block' });

  test('update card: the manual steps without companion; the install dialog and its phases @keyboard', async ({
    page,
  }) => {
    // The suite's server has no updater companion and knows no newer release: the dashboard's
    // update state gets a newer version on top of the real answer, and with `companion` the
    // companion as well. The install and its status never reach the server.
    let companion = false;
    let realUpdater: unknown;
    const notes = Array.from({ length: 60 }, (_, line) => `Zeile ${line + 1} der Release-Notizen`);
    await page.route('**/api/v1/admin/dashboard', async (route) => {
      const response = await route.fetch();
      const body = (await response.json()) as { update: Record<string, unknown> };
      realUpdater = body.update.updaterAvailable;
      body.update = {
        ...body.update,
        available: true,
        development: false,
        latestVersion: '99.0.0',
        notes: notes.join('\n'),
        updaterConfigured: companion,
        updaterAvailable: companion,
      };
      await route.fulfill({ response, json: body });
    });
    const installs: string[] = [];
    await page.route('**/api/v1/admin/update/install', async (route) => {
      installs.push(route.request().method());
      await route.fulfill({
        status: 202,
        json: { phase: 'backing_up', from: '1.0.0', to: '99.0.0', migrated: false },
      });
    });
    await page.route('**/api/v1/admin/update/status', (route) =>
      route.fulfill({ json: { phase: 'pulling', from: '1.0.0', to: '99.0.0', migrated: false } }),
    );

    // Fallback C for real: the server says it reaches no companion.
    await openOverview(page);
    const server = card(page, 'Server');
    await expect(server.getByText('SLOUCHCAT_VERSION=99.0.0')).toBeVisible();
    expect(realUpdater).toBe(false);
    await expect(server.getByText('docker compose pull', { exact: true })).toBeVisible();
    await expect(server.getByText('docker compose up -d', { exact: true })).toBeVisible();
    await expect(server.getByRole('button', { name: /installieren/ })).toHaveCount(0);
    await resetFocus(page);
    await tabTo(page, entry(page, 'Übersicht'));
    // Tab walks the three copy buttons and the guide, then the card's own buttons.
    const copies = server.getByRole('button', { name: 'Kopieren' });
    for (let index = 0; index < 3; index += 1) {
      await page.keyboard.press('Tab');
      await expectFocusOn(page, copies.nth(index));
    }
    await page.keyboard.press('Tab');
    await expectFocusOn(page, server.getByRole('link', { name: /Anleitung: Update per Knopf/ }));
    await page.keyboard.press('Tab');
    await expectFocusOn(page, server.getByRole('button', { name: 'Nach Updates suchen' }));

    // With the companion: the button, its confirmation, the notes scrolled by keyboard.
    companion = true;
    await page.reload();
    const install = server.getByRole('button', { name: 'Update auf 99.0.0 installieren' });
    await expect(install).toBeVisible();
    await resetFocus(page);
    await tabTo(page, install);
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('alertdialog', { name: 'Update auf 99.0.0 installieren?' });
    await expect(dialog).toContainText('Der Server startet neu; laufende Wiedergaben brechen ab.');
    await expectFocusOn(page, dialog.getByRole('button', { name: 'Installieren' }));
    await page.keyboard.press('Shift+Tab');
    await expectFocusOn(page, dialog.getByRole('button', { name: 'Abbrechen' }));
    await page.keyboard.press('Shift+Tab');
    const region = dialog.getByRole('region', { name: 'Release-Notizen' });
    await expectFocusOn(page, region);
    await expect(region).toContainText('Zeile 60 der Release-Notizen');
    await page.keyboard.press('ArrowDown');
    await page.keyboard.press('PageDown');
    await expect.poll(() => region.evaluate((element) => element.scrollTop)).toBeGreaterThan(0);
    // Tab stays in the open dialog; Escape closes it and the focus is back on the button.
    for (let presses = 0; presses < 3; presses += 1) await page.keyboard.press('Tab');
    await expect(region).toBeFocused();
    await page.keyboard.press('Escape');
    await expect(dialog).toHaveCount(0);
    await expectFocusOn(page, install);
    expect(installs).toEqual([]);

    // Enter, Enter: the install starts; the card shows its steps and holds the focus.
    await page.keyboard.press('Enter');
    await expectFocusOn(page, dialog.getByRole('button', { name: 'Installieren' }));
    await page.keyboard.press('Enter');
    await expect(dialog).toHaveCount(0);
    const steps = server.getByRole('list', { name: 'Schritte des Updates' });
    await expect(steps).toBeVisible();
    await expect(steps.locator('[aria-current="step"]')).toHaveText(/^Herunterladen/);
    await expect(server.getByText('Der Updater lädt das neue Image herunter.')).toBeVisible();
    expect(installs).toEqual(['POST']);
    await expect(install).toHaveCount(0);
    // A restart would cut the install off: the card does not offer it meanwhile.
    await expect(server.getByRole('button', { name: 'Neu starten' })).toHaveCount(0);
    await expectFocusOn(
      page,
      server
        .getByRole('status')
        .filter({ has: page.getByRole('list', { name: 'Schritte des Updates' }) }),
    );
    await expectNoFocusTrap(page, 120);
  });
});

test('on a phone: «Verwaltung · Übersicht» opens the sidebar as a sheet, Escape returns', async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/admin');
  await expect(page.getByRole('heading', { level: 1, name: 'Verwaltung' })).toBeVisible();
  // Below 768 px the sidebar lives in the sheet only.
  await expect(adminNav(page)).toHaveCount(0);
  await expect(appNav(page)).toHaveCount(0);
  const button = page.getByRole('button', { name: 'Verwaltung · Übersicht' });
  await resetFocus(page);
  await tabTo(page, button, 10);
  await page.keyboard.press('Enter');
  const sheet = page.getByRole('dialog', { name: 'Seiten der Verwaltung' });
  await expect(sheet).toBeVisible();
  await expect(sheet.getByRole('link')).toHaveCount(NAV_ENTRIES.length);
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
  await expect(page.getByRole('button', { name: 'Verwaltung · Bibliotheken' })).toBeVisible();
  await resetFocus(page);
  await expectNoFocusTrap(page, 120);
});
