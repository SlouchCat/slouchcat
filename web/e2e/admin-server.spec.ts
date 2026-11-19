import type { Page } from '@playwright/test';
import { adminArea, adminEntry, areaTab } from './admin';
import { refreshSpotlight, traktMockStatus } from './discover';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// System › Server (redesign, build plan 4; rule 8): the server's functions of the former overview
// on a page of their own — its state, the updates with the install and the restart, both storage
// paths, the activity and the server log, polled every 5 s (A-3). By keyboard, with a visible focus
// and no focus trap.

interface TraktData {
  lastError?: string;
}

const card = (page: Page, name: string) => page.getByRole('region', { name, exact: true });
const updates = (page: Page) => card(page, 'Updates & Neustart');
const serverLog = (page: Page) => card(page, 'Server-Protokoll');
const errorList = (page: Page) =>
  serverLog(page).getByRole('list', { name: 'Fehler aus dem Server-Log', includeHidden: true });

async function openServer(page: Page) {
  await page.goto('/admin/server');
  await expect(page.getByRole('heading', { level: 1, name: 'Server' })).toBeVisible();
  await expect(areaTab(page, 'Server')).toHaveAttribute('aria-current', 'page');
}

test('the server page: state, updates with the restart, both storage paths, activity, the log collapsed (A-3)', async ({
  page,
}) => {
  await openServer(page);
  await expect(page.getByText('Live, alle 5 Sekunden')).toBeVisible();
  // The area «System» and its page «Server» are current in the sidebar's tree; «Nach Updates
  // suchen» is the header's action.
  await expect(adminArea(page, 'System')).toHaveAttribute('aria-current', 'true');
  await expect(adminEntry(page, 'Server')).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('button', { name: 'Nach Updates suchen' })).toBeVisible();
  const state = card(page, 'Serverzustand');
  await expect(state.getByText('Version', { exact: true })).toBeVisible();
  await expect(state.getByText('Läuft seit')).toBeVisible();
  await expect(state.getByText('Videokodierung')).toBeVisible();
  await expect(state.getByText(/^(Beobachtete Ordner|inotify-Limit)$/)).toBeVisible();
  await expect(
    state.getByRole('link', {
      name: 'Grafikchip, öffentliche Adresse und Update-Prüfung: Integrationen',
    }),
  ).toHaveAttribute('href', '/admin/integrations?item=server');
  await expect(updates(page).getByText('Updates', { exact: true })).toBeVisible();
  await expect(updates(page).getByRole('button', { name: 'Neu starten' })).toBeVisible();
  const storage = card(page, 'Speicher');
  await expect(storage.getByRole('meter', { name: /^Konfiguration/ })).toBeVisible();
  await expect(storage.getByRole('meter', { name: /^Cache/ })).toBeVisible();
  await expect(storage.getByText(/^Frei: /)).toHaveCount(2);
  await expect(
    page.getByRole('list', { name: 'Letzte Aktivität' }).getByRole('listitem').first(),
  ).toBeVisible();
  // The server log: collapsed with its number; its list is not live itself, a poll with new
  // errors announces only their count.
  await expect(serverLog(page).locator('summary')).toHaveText(/^Server-Protokoll · /);
  await expect(errorList(page)).toBeHidden();
  await expect(errorList(page)).not.toHaveAttribute('aria-live');
  await expect(serverLog(page).getByRole('status')).toHaveCount(1);
});

test('a provoked error shows in the opened server log within 10 s and on the overview (A-3)', async ({
  page,
  owner,
}) => {
  // The refreshes wait at most the suite's 2 s limit (discover.ts refreshSpotlight); the poll for
  // the repaired Trakt state takes up to 60 s.
  test.setTimeout(120_000);
  await openServer(page);
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
    await expect(errorList(page)).toContainText(/trakt/i, {
      timeout: Math.max(1000, 10_000 - (Date.now() - provoked)),
    });
    await expect(errorList(page)).toBeVisible();
    // The poll that brought it announces a short count, not the whole list.
    await expect(serverLog(page).getByRole('status')).toHaveText(/^\d+ neuer? Fehler$/);
    // The overview names the integration with its error among what needs attention.
    await page.goto('/admin');
    await expect(card(page, 'Was benötigt Aufmerksamkeit?').getByText('Trakt: Fehler')).toBeVisible(
      { timeout: 10_000 },
    );
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

test('by keyboard: the area tab, the header, the restart dialog up to «Abbrechen», the log; no focus trap @keyboard', async ({
  page,
}) => {
  // Never a real restart: the suite's server must stay up. Should the confirmation be pressed by
  // mistake, the request goes nowhere and the test fails below.
  const restarts: string[] = [];
  await page.route('**/api/v1/admin/server/restart', async (route) => {
    restarts.push(route.request().method());
    await route.abort();
  });
  // From «Statistiken» over the area's tabs: → reaches «Server», Enter opens it.
  await page.goto('/admin/stats');
  await expect(page.getByRole('heading', { level: 1, name: 'Statistiken' })).toBeVisible();
  await resetFocus(page);
  await tabTo(page, areaTab(page, 'Statistiken'), 60);
  await page.keyboard.press('End');
  await expectFocusOn(page, areaTab(page, 'Server'));
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin\/server$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Server' })).toBeVisible();
  await expect(areaTab(page, 'Server')).toHaveAttribute('aria-current', 'page');

  // One Tab stop for the tabs: the next Tab is the header's «Nach Updates suchen».
  await resetFocus(page);
  await tabTo(page, areaTab(page, 'Server'), 60);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('button', { name: 'Nach Updates suchen' }));

  const restart = updates(page).getByRole('button', { name: 'Neu starten' });
  await tabTo(page, restart, 40);
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
  await expect(updates(page)).toBeVisible();

  // The server log's summary is reached by Tab and opens with Enter.
  await resetFocus(page);
  await tabTo(page, serverLog(page).locator('summary'), 80);
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  await expect(serverLog(page).locator('details')).toHaveAttribute('open', '');
  await resetFocus(page);
  await expectNoFocusTrap(page, 120);
});

test.describe('updates (concept 15 P-8)', () => {
  // Chromium sends the page's GET requests through the service worker, where page.route does not
  // see them; the dashboard is answered here, so the worker stays out.
  test.use({ serviceWorkers: 'block' });

  test('the manual steps without companion; the install dialog and its phases @keyboard', async ({
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
    await openServer(page);
    const card = updates(page);
    await expect(card.getByText('SLOUCHCAT_VERSION=99.0.0')).toBeVisible();
    expect(realUpdater).toBe(false);
    await expect(card.getByText('docker compose pull', { exact: true })).toBeVisible();
    await expect(card.getByText('docker compose up -d', { exact: true })).toBeVisible();
    await expect(card.getByRole('button', { name: /installieren/ })).toHaveCount(0);
    // Tab walks the three copy buttons and the guide, then the card's «Neu starten».
    const copies = card.getByRole('button', { name: 'Kopieren' });
    await resetFocus(page);
    await tabTo(page, copies.nth(0), 60);
    for (let index = 1; index < 3; index += 1) {
      await page.keyboard.press('Tab');
      await expectFocusOn(page, copies.nth(index));
    }
    await page.keyboard.press('Tab');
    await expectFocusOn(page, card.getByRole('link', { name: /Anleitung: Update per Knopf/ }));
    await page.keyboard.press('Tab');
    await expectFocusOn(page, card.getByRole('button', { name: 'Neu starten' }));

    // With the companion: the button, its confirmation, the notes scrolled by keyboard.
    companion = true;
    await page.reload();
    const install = card.getByRole('button', { name: 'Update auf 99.0.0 installieren' });
    await expect(install).toBeVisible();
    await resetFocus(page);
    await tabTo(page, install, 60);
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
    const steps = card.getByRole('list', { name: 'Schritte des Updates' });
    await expect(steps).toBeVisible();
    await expect(steps.locator('[aria-current="step"]')).toHaveText(/^Herunterladen/);
    await expect(card.getByText('Der Updater lädt das neue Image herunter.')).toBeVisible();
    expect(installs).toEqual(['POST']);
    await expect(install).toHaveCount(0);
    // A restart would cut the install off: the card does not offer it meanwhile.
    await expect(card.getByRole('button', { name: 'Neu starten' })).toHaveCount(0);
    await expectFocusOn(
      page,
      card
        .getByRole('status')
        .filter({ has: page.getByRole('list', { name: 'Schritte des Updates' }) }),
    );
    await expectNoFocusTrap(page, 120);
  });
});

test('on a phone the server page fits the screen and reaches every card', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await openServer(page);
  await expect(page.getByRole('button', { name: 'Verwaltung · System / Server' })).toBeVisible();
  await expect(card(page, 'Serverzustand')).toBeVisible();
  await expect(serverLog(page)).toBeAttached();
  await expect
    .poll(() => page.evaluate(() => document.documentElement.scrollWidth))
    .toBeLessThanOrEqual(390);
});
