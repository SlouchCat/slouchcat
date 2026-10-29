import { expect, test, type Page } from '@playwright/test';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// The real wizard ran in global-setup.ts through the API. To test the page's keyboard behaviour
// the setup state is answered as "required" and the submit is intercepted, so the run's account
// stays untouched. The checks of folder and TMDB key answer 409 once the setup is done, so they
// are answered here too (design 5, A-1: path warning and invalid key with mocked answers); the
// state of the GPU and the inotify limit is a server without GPU in the container and a tight
// limit, so both commands show.
// The service worker is blocked so that page.route sees every request.
test.use({ storageState: { cookies: [], origins: [] }, serviceWorkers: 'block' });

const system = {
  version: '0.9.0',
  startedAt: '2026-10-21T08:00:00Z',
  gpu: {
    backend: 'software',
    hardwareEncode: false,
    tonemapper: 'cpu',
    hwTonemap: false,
    composeOverride: 'nvidia',
    devices: { dri: false, nvidia: true },
  },
  inotify: { maxUserWatches: 8192, watchedDirs: 0, usagePercent: 0, warn: true },
  configFs: 'ext4',
  cacheFs: 'ext4',
};

/** Answers the folder check with `answer` and counts the requests. */
async function answerPathCheck(page: Page, answer: object): Promise<{ count: number }> {
  const seen = { count: 0 };
  await page.route('**/api/v1/setup/path-check', async (route) => {
    seen.count += 1;
    await route.fulfill({ json: answer });
  });
  return seen;
}

async function answerKeyCheck(page: Page, answer: object): Promise<void> {
  await page.route('**/api/v1/setup/tmdb-check', (route) => route.fulfill({ json: answer }));
}

/**
 * The configured media roots the wizard offers under the folder field (idea 02, E5). The real
 * server answers 409 here, because the run's account exists; the suggestions are mocked so the
 * page can be tested at all.
 */
const SETUP_ROOTS = [
  { id: 'c0', path: '/media', browsable: true, source: 'configured' },
  { id: 'c1', path: '/mnt/nas', browsable: true, source: 'configured' },
];

async function answerSetupRoots(page: Page): Promise<void> {
  await page.route('**/api/v1/setup/roots', (route) => route.fulfill({ json: SETUP_ROOTS }));
}

const step = (page: Page, name: RegExp) => page.getByRole('region', { name });

test.beforeEach(async ({ page }) => {
  await answerPathCheck(page, { ok: true, fsType: 'ext4', watchMode: 'inotify' });
  await answerKeyCheck(page, { ok: true });
  await answerSetupRoots(page);
  await page.route('**/api/v1/setup', async (route) => {
    if (route.request().method() === 'GET') {
      await route.fulfill({ json: { setupRequired: true, system } });
      return;
    }
    await route.fulfill({
      status: 422,
      contentType: 'application/problem+json',
      json: {
        type: 'about:blank',
        title: 'invalid',
        status: 422,
        code: 'validation_failed',
        errors: [{ field: 'library.roots[0].path', code: 'library_path_invalid' }],
      },
    });
  });
  await page.goto('/setup');
  await expect(
    page.getByRole('heading', { level: 1, name: 'Willkommen bei Slouchcat' }),
  ).toBeVisible();
});

test('Tab walks through the four steps in order with a visible focus ring', async ({ page }) => {
  // The e-mail field has focus on arrival.
  await expectFocusOn(page, page.getByLabel('E-Mail-Adresse'));
  const gpu = step(page, /Grafikchip und Dateisystem/);
  const order = [
    page.getByLabel('Passwort', { exact: true }),
    page.getByLabel('Passwort wiederholen'),
    page.getByLabel('Name', { exact: true }),
    page.getByRole('radio', { name: 'Video' }),
    // The folder list (F-13): the one card of a new video library with its path, the configured
    // roots between the field and the check button (idea 02, E5), «Inhalt» (films are chosen),
    // the fixed default list as text, then the button for another card.
    page.getByLabel('Ordner', { exact: true }),
    page.getByRole('button', { name: '/media', exact: true }),
    page.getByRole('button', { name: '/mnt/nas', exact: true }),
    page.getByRole('button', { name: 'Ordner prüfen' }),
    page.getByRole('radio', { name: 'Filme' }),
    page.getByRole('button', { name: 'Weiteren Ordner hinzufügen' }),
    page.getByRole('link', { name: /Token auf themoviedb.org erstellen/ }),
    page.getByLabel('API-Lesezugriffs-Token'),
    page.getByRole('button', { name: 'Schlüssel prüfen' }),
    gpu.getByRole('group', { name: 'Compose-Befehl für NVIDIA' }).getByRole('button'),
    gpu.getByRole('group', { name: 'Befehl für das inotify-Limit' }).getByRole('button'),
    // The step has no inputs: the submit button stays last.
    page.getByRole('button', { name: 'Einrichtung abschliessen' }),
  ];
  for (const locator of order) {
    await page.keyboard.press('Tab');
    await expectFocusOn(page, locator);
  }
  await expectNoFocusTrap(page);
});

test('arrow keys switch the library kind', async ({ page }) => {
  await resetFocus(page);
  await tabTo(page, page.getByRole('radio', { name: 'Video' }));
  await expect(page.getByRole('radio', { name: 'Video' })).toBeChecked();
  // M6: audiobooks bring the default age rating; podcasts drop the folder.
  const rating = page.getByRole('radiogroup', { name: 'Standard-Altersstufe' });
  await expect(rating).toHaveCount(0);
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, page.getByRole('radio', { name: 'Hörbücher' }));
  await expect(page.getByRole('radio', { name: 'Hörbücher' })).toBeChecked();
  await expect(rating).toBeVisible();
  await expect(page.getByLabel('Ordner', { exact: true })).toBeVisible();
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, page.getByRole('radio', { name: 'Podcasts' }));
  // Podcasts start without a card (F-13); the add button stays, the rating follows it.
  await expect(page.getByLabel('Ordner', { exact: true })).toHaveCount(0);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('button', { name: 'Ordner hinzufügen', exact: true }));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, rating.getByRole('radio', { name: 'FSK 0' }));
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, rating.getByRole('radio', { name: 'FSK 6' }));
  await expectVisibleFocus(page);
  await expectNoFocusTrap(page);
});

test('Enter submits; errors appear inline and the form stays usable', async ({ page }) => {
  await page.getByLabel('E-Mail-Adresse').fill('owner@example.test');
  await page.getByLabel('Passwort', { exact: true }).fill('correct horse battery');
  await page.getByLabel('Passwort wiederholen').fill('correct horse battery');
  await page.getByLabel('Ordner', { exact: true }).fill('/does/not/exist');
  await page.getByLabel('Ordner', { exact: true }).focus();
  await page.keyboard.press('Enter');

  const path = page.getByLabel('Ordner', { exact: true });
  await expect(path).toHaveAttribute('aria-invalid', 'true');
  await expect(page.getByText(/Diesen Ordner gibt es auf dem Server nicht/)).toBeVisible();
  await path.focus();
  await expectVisibleFocus(page);
});

test('client validation catches a short password before anything is sent', async ({ page }) => {
  let posted = false;
  page.on('request', (request) => {
    if (request.method() === 'POST' && request.url().endsWith('/api/v1/setup')) posted = true;
  });
  await page.getByLabel('Passwort', { exact: true }).fill('short');
  await page.getByRole('button', { name: 'Einrichtung abschliessen' }).focus();
  await page.keyboard.press('Enter');
  await expect(page.getByText('Mindestens 10 Zeichen.').last()).toBeVisible();
  expect(posted).toBe(false);
});

test('the folder is checked on leaving the field: file system and network share warning', async ({
  page,
}) => {
  await page.unroute('**/api/v1/setup/path-check');
  const checks = await answerPathCheck(page, {
    ok: true,
    fsType: 'cifs',
    watchMode: 'poll',
    warning: 'network_share',
  });
  const library = step(page, /Erste Bibliothek/);
  await tabTo(page, page.getByLabel('Ordner', { exact: true }));
  await page.keyboard.type('/media/nas/Filme');
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('button', { name: '/media', exact: true }));
  await tabTo(page, page.getByRole('button', { name: 'Ordner prüfen' }));
  await expect(
    library.getByText('Dateisystem cifs · Änderungen werden regelmässig abgefragt'),
  ).toBeVisible();
  await expect(library.getByText('Netzfreigabe erkannt')).toBeVisible();
  await expect(library.getByText(/Webhooks aus Radarr und Sonarr/)).toBeVisible();
  // Enter on the button asks again only for a changed folder.
  await page.keyboard.press('Enter');
  expect(checks.count).toBe(1);
  await expectNoFocusTrap(page);
});

// Idea 02, E5: the wizard offers the configured roots, no browsing. Enter on one of them fills
// the folder field, and the field stays the equal way in.
test('the configured roots are suggestions the keyboard can take', async ({ page }) => {
  // Before an account exists the wizard never asks for a listing: no browse button, no request.
  const listings: string[] = [];
  page.on('request', (request) => {
    if (request.url().includes('/api/v1/admin/fs/')) listings.push(request.url());
  });
  const path = page.getByLabel('Ordner', { exact: true });
  await resetFocus(page);
  await tabTo(page, page.getByRole('button', { name: '/mnt/nas', exact: true }));
  await page.keyboard.press('Enter');
  await expect(path).toHaveValue('/mnt/nas');
  // The field stays the equal way in: it can still be typed over.
  await path.fill('/media/Filme');
  await expect(path).toHaveValue('/media/Filme');
  await expect(page.getByRole('button', { name: 'Durchsuchen …' })).toHaveCount(0);
  expect(listings).toEqual([]);
  await expectNoFocusTrap(page);
});

test('"Ordner prüfen" reports a missing folder at the field', async ({ page }) => {
  await page.unroute('**/api/v1/setup/path-check');
  await answerPathCheck(page, { ok: false, problem: 'missing' });
  const path = page.getByLabel('Ordner', { exact: true });
  await path.fill('/does/not/exist');
  await page.getByRole('button', { name: 'Ordner prüfen' }).focus();
  await page.keyboard.press('Enter');
  await expect(path).toHaveAttribute('aria-invalid', 'true');
  await expect(page.getByText(/Diesen Ordner gibt es auf dem Server nicht/)).toBeVisible();
  await expectVisibleFocus(page);
});

test('"Schlüssel prüfen" says clearly when TMDB rejects the key (A-1)', async ({ page }) => {
  await page.unroute('**/api/v1/setup/tmdb-check');
  await answerKeyCheck(page, { ok: false, problem: 'invalid' });
  const key = page.getByLabel('API-Lesezugriffs-Token');
  await tabTo(page, key);
  await page.keyboard.type('not-a-token');
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('button', { name: 'Schlüssel prüfen' }));
  await page.keyboard.press('Enter');
  await expect(key).toHaveAttribute('aria-invalid', 'true');
  await expect(
    page.getByText('Ungültig: TMDB hat den Schlüssel abgelehnt. Prüfe das Lesezugriffs-Token.'),
  ).toBeVisible();

  // A valid key reads as such, and the field is no longer marked.
  await page.unroute('**/api/v1/setup/tmdb-check');
  await answerKeyCheck(page, { ok: true });
  await key.fill('a-good-token');
  await page.getByRole('button', { name: 'Schlüssel prüfen' }).press('Enter');
  await expect(page.getByText('Gültig: TMDB hat den Schlüssel angenommen.')).toBeVisible();
  await expect(key).not.toHaveAttribute('aria-invalid', 'true');
});

test('GPU step: the Compose command and the sysctl command can be copied by keyboard', async ({
  page,
}) => {
  const gpu = step(page, /Grafikchip und Dateisystem/);
  await expect(gpu.getByText('Kein Grafikchip erkannt')).toBeVisible();
  await expect(gpu.getByText('Prozessor (Software)')).toBeVisible();
  const compose = gpu.getByRole('group', { name: 'Compose-Befehl für NVIDIA' });
  await expect(compose).toContainText(
    'docker compose -f compose.yaml -f compose.nvidia.yaml up -d',
  );
  await expect(gpu.getByText('Das inotify-Limit ist knapp')).toBeVisible();
  await expect(gpu.getByRole('group', { name: 'Befehl für das inotify-Limit' })).toContainText(
    'sysctl fs.inotify.max_user_watches=524288',
  );

  await resetFocus(page);
  const copy = compose.getByRole('button', { name: 'Kopieren' });
  await tabTo(page, copy);
  await page.keyboard.press('Enter');
  // With a clipboard the command is copied; without one (the browser may refuse) it is selected.
  await expect(compose.getByRole('status')).toHaveText(
    /^(Befehl kopiert\.|Kopieren nicht möglich.*)$/,
  );
  await expectFocusOn(page, copy);
  await expectNoFocusTrap(page);
});
