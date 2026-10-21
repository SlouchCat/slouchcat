import type { Page } from '@playwright/test';
import { refreshTrakt, traktMockStatus } from './discover';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// The administration (M8, A-2, A-3): the header has one entry "Verwaltung"; every /admin page has
// the admin navigation (one Tab stop, ← → Home End); the overview shows the dashboard of the real
// server (streams, jobs, storage of /config and /cache, integrations, errors, polled every 5 s)
// and a card for every admin page.

interface TraktData {
  lastError?: string;
}

const adminNav = (page: Page) => page.getByRole('navigation', { name: 'Verwaltung' });
const headerNav = (page: Page) => page.getByRole('navigation', { name: 'Hauptnavigation' });

/** Every page of the admin navigation, in its order, with its path. */
const NAV_PAGES = [
  ['Übersicht', '/admin'],
  ['Bibliotheken', '/admin/libraries'],
  ['Podcasts', '/admin/podcasts'],
  ['Pflege', '/admin/maintenance'],
  ['Konten', '/admin/accounts'],
  ['Zugriff', '/admin/access'],
  ['Geräte', '/admin/devices'],
  ['Streams', '/admin/streams'],
  ["Cat's Empfehlungen", '/admin/picks'],
  ['Sammlungen', '/admin/collections'],
  ['Startseite', '/admin/home-rows'],
  ['Ankündigungen', '/admin/announcements'],
  ['Integrationen', '/admin/integrations'],
  ['Statistiken', '/admin/stats'],
  ['Sicherung', '/admin/backup'],
] as const;

async function openOverview(page: Page) {
  await page.goto('/admin');
  await expect(page.getByRole('heading', { level: 1, name: 'Verwaltung' })).toBeVisible();
  await expect(adminNav(page).getByRole('link', { name: 'Übersicht' })).toHaveAttribute(
    'aria-current',
    'page',
  );
}

test('the header has one "Verwaltung" entry; Enter opens the overview', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('heading', { level: 2, name: 'Neu hinzugefügt' })).toBeVisible();
  const entry = headerNav(page).getByRole('link', { name: 'Verwaltung' });
  await expect(entry).toHaveAttribute('href', '/admin');
  // The nine admin links of M7 are gone from the header.
  for (const name of ['Bibliotheken', 'Geräte', 'Streams', 'Startseite']) {
    await expect(headerNav(page).getByRole('link', { name, exact: true })).toHaveCount(0);
  }
  await resetFocus(page);
  await tabTo(page, entry);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Verwaltung' })).toBeVisible();
  await expect(entry).toHaveAttribute('aria-current', 'page');
});

test('admin navigation: one Tab stop, arrows move, Enter follows, Back returns', async ({
  page,
}) => {
  await page.goto('/admin/devices');
  await expect(page.getByRole('heading', { level: 1, name: 'Geräte' })).toBeVisible();
  const nav = adminNav(page);
  await expect(nav.getByRole('link')).toHaveText(NAV_PAGES.map(([name]) => name));
  const devices = nav.getByRole('link', { name: 'Geräte' });
  await expect(devices).toHaveAttribute('aria-current', 'page');

  // From the top: the header, then the navigation's current page.
  await resetFocus(page);
  await tabTo(page, devices);
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, nav.getByRole('link', { name: 'Streams' }));
  await page.keyboard.press('ArrowLeft');
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, nav.getByRole('link', { name: 'Zugriff' }));
  await page.keyboard.press('End');
  await expectFocusOn(page, nav.getByRole('link', { name: 'Sicherung' }));
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, nav.getByRole('link', { name: 'Übersicht' }));
  await page.keyboard.press('Home');
  await expectFocusOn(page, nav.getByRole('link', { name: 'Übersicht' }));

  // Tab leaves the row at once (one Tab stop), Shift+Tab comes back to the current page.
  await page.keyboard.press('Tab');
  await expect(nav.locator(':focus')).toHaveCount(0);
  await page.keyboard.press('Shift+Tab');
  await expectFocusOn(page, devices);

  // Enter follows a link; the browser's Back returns.
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin\/streams$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Streams' })).toBeVisible();
  await expect(nav.getByRole('link', { name: 'Streams' })).toHaveAttribute('aria-current', 'page');
  await page.goBack();
  await expect(page).toHaveURL(/\/admin\/devices$/);
  await expect(devices).toHaveAttribute('aria-current', 'page');
  await expectNoFocusTrap(page);
});

test('every admin page is reached through the admin navigation (A-2)', async ({ page }) => {
  test.setTimeout(90_000);
  await openOverview(page);
  // The Tab stop is the current page; the arrows walk to the next one.
  const nav = adminNav(page);
  await nav.locator('a[tabindex="0"]').focus();
  for (const [name, path] of NAV_PAGES.slice(1)) {
    await page.keyboard.press('ArrowRight');
    const link = nav.getByRole('link', { name, exact: true });
    await expectFocusOn(page, link);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(new RegExp(`${path}$`));
    // Every page has the heading of its link; its navigation keeps the focus on the followed link.
    await expect(page.getByRole('heading', { level: 1, name, exact: true })).toBeVisible();
    await expect(link).toHaveAttribute('aria-current', 'page');
    await expectFocusOn(page, link);
  }
});

test('the overview shows streams, jobs, storage, integrations and errors (A-3)', async ({
  page,
}) => {
  await openOverview(page);
  const streams = page.getByRole('region', { name: 'Streams', exact: true });
  await expect(streams.getByText('Laufende Streams')).toBeVisible();
  await expect(streams.getByRole('link', { name: 'Alle Streams' })).toHaveAttribute(
    'href',
    '/admin/streams',
  );
  const jobs = page.getByRole('region', { name: 'Hintergrundjobs' });
  for (const counter of ['Wartend', 'Laufend', 'Fehlgeschlagen']) {
    await expect(jobs.getByText(counter, { exact: true })).toBeVisible();
  }
  const storage = page.getByRole('region', { name: 'Speicher', exact: true });
  await expect(storage.getByRole('meter', { name: /^Konfiguration/ })).toBeVisible();
  await expect(storage.getByRole('meter', { name: /^Cache/ })).toBeVisible();
  await expect(storage.getByText(/^Frei: /)).toHaveCount(2);
  // start-server.mjs passes the Trakt mock's client id: the key comes from the environment.
  const integrations = page.getByRole('region', { name: 'Integrationen', exact: true });
  await expect(integrations.getByText(/^Trakt: /)).toBeVisible();
  await expect(integrations.getByText(/^TMDB: /)).toBeVisible();
  const server = page.getByRole('region', { name: 'Server', exact: true });
  await expect(server.getByText('Videokodierung')).toBeVisible();
  await expect(server.getByText('Beobachtete Ordner')).toBeVisible();
  await expect(page.getByRole('list', { name: 'Fehler aus dem Server-Log' })).toHaveAttribute(
    'aria-live',
    'polite',
  );
});

test('a provoked error shows on the dashboard within 10 s (A-3)', async ({ page, owner }) => {
  test.setTimeout(120_000);
  await openOverview(page);
  const integrations = page.getByRole('region', { name: 'Integrationen', exact: true });
  await expect(integrations.getByText(/^Trakt: /)).toBeVisible();
  try {
    // Trakt is down; "Jetzt aktualisieren" runs the job at once.
    await traktMockStatus(503);
    const provoked = Date.now();
    await refreshTrakt(owner);
    await expect(integrations.getByText('Trakt: Fehler')).toBeVisible({ timeout: 10_000 });
    await expect(page.getByRole('list', { name: 'Fehler aus dem Server-Log' })).toContainText(
      /trakt/i,
      { timeout: Math.max(1000, 10_000 - (Date.now() - provoked)) },
    );
  } finally {
    // A failed refresh empties the Trakt lists (I-7): fetch them again for the later specs.
    await traktMockStatus(200);
    await refreshTrakt(owner);
    await expect
      .poll(
        async () =>
          ((await (await owner.get('/api/v1/admin/trakt')).json()) as TraktData).lastError,
        { timeout: 60_000, intervals: [1000] },
      )
      .toBeFalsy();
  }
});

test('Tab: header, navigation, dashboard, cards; Enter on a card; no focus trap', async ({
  page,
}) => {
  await openOverview(page);
  const cards = page.getByRole('region', { name: 'Alle Verwaltungsseiten' });
  await expect(cards.getByRole('link')).toHaveCount(16);
  await expect(cards.getByRole('link', { name: 'Benachrichtigungen' })).toHaveAttribute(
    'href',
    '/settings/notifications',
  );
  await expect(cards.getByRole('link', { name: 'Profile' })).toHaveAttribute(
    'href',
    '/settings/profiles',
  );
  await resetFocus(page);
  await tabTo(page, adminNav(page).getByRole('link', { name: 'Übersicht' }));
  await tabTo(page, page.getByRole('link', { name: 'Integrationen einrichten' }), 20);
  const integrations = cards.getByRole('link', { name: 'Integrationen' });
  await tabTo(page, integrations, 60);
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin\/integrations$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Integrationen' })).toBeVisible();
  await page.goBack();
  await expect(page).toHaveURL(/\/admin$/);
  await expect(cards.getByRole('link', { name: 'Integrationen' })).toBeVisible();
  await resetFocus(page);
  await expectNoFocusTrap(page, 120);
});
