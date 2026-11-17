import type { APIRequestContext, Page } from '@playwright/test';
import { arrReset } from './arr';
import { ARR_API_KEY } from './arr-server.mjs';
import { CSRF } from './catalog';
import { BASE_URL, RADARR_BASE, SEERR_BASE, SONARR_BASE } from './env';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';
import { SEERR_API_KEY } from './seerr-server.mjs';

// Integrationen › Seerr (concept 25, M-22) against the real server and the Seerr mock
// (seerr-server.mjs): the seventh tile opens the view; the band «Verbindung» takes switch, address
// and key, tests them and saves them, and the view tests the saved values right after; the check
// list and the state line; a wrong key, a bad address and a new host without the key; another
// instance and the decision about it; the call log. All by keyboard, with a visible focus and no
// focus trap. Every test leaves the connection deleted, as it found it.

const region = (page: Page, name: string) => page.getByRole('region', { name, exact: true });
const tileLink = (page: Page, name: string) =>
  page.getByRole('list', { name: 'Dienste' }).getByRole('link', { name, exact: true });
/** A field of the band «Verbindung»: its setting row is a group with the same name. */
const field = (page: Page, name: string) =>
  region(page, 'Seerr').getByRole('textbox', { name, exact: true });

async function seerrControl(instance: 'same' | 'other'): Promise<void> {
  const response = await fetch(`${SEERR_BASE}/__control`, {
    method: 'POST',
    body: JSON.stringify({ instance }),
  });
  expect(response.status).toBe(204);
}

/** Seerr names its Radarr and Sonarr, or none (`servers: false`). */
async function seerrServers(servers: boolean): Promise<void> {
  const response = await fetch(`${SEERR_BASE}/__control`, {
    method: 'POST',
    body: JSON.stringify({ servers }),
  });
  expect(response.status).toBe(204);
}

async function send(api: APIRequestContext, method: 'put' | 'post', path: string, data: object) {
  const response = await api[method](`/api/v1${path}`, { headers: CSRF, data });
  expect(response.ok(), await response.text()).toBe(true);
  return response;
}

/**
 * The connection the mock answers to, saved and tested through the API. Saving queues the job
 * `seerr.users`; it is waited for, so it never reads the mock while it plays another instance.
 */
async function connect(api: APIRequestContext): Promise<void> {
  await send(api, 'put', '/admin/seerr', { url: SEERR_BASE, apiKey: SEERR_API_KEY, enabled: true });
  await send(api, 'post', '/admin/seerr/test', {});
  await expect
    .poll(
      async () =>
        ((await (await api.get('/api/v1/admin/wishes/mappings/users')).json()) as unknown[]).length,
      {
        timeout: 15_000,
      },
    )
    .toBeGreaterThan(0);
}

/**
 * The mock answers as the first instance again, and the server agrees: a test of the saved
 * connection and, while it still sees another instance, «Zuordnung bestätigen». The job
 * `seerr.users` a decision queues may read the mock between the two, so this repeats until a test
 * finds the same instance.
 */
async function sameInstance(api: APIRequestContext): Promise<void> {
  await seerrControl('same');
  await expect
    .poll(
      async () => {
        await send(api, 'post', '/admin/seerr/test', {});
        const status = (await (await api.get('/api/v1/admin/seerr/status')).json()) as {
          instanceChanged: boolean;
        };
        if (!status.instanceChanged) return true;
        await send(api, 'post', '/admin/seerr/instance', { action: 'confirm' });
        return false;
      },
      { timeout: 15_000 },
    )
    .toBe(true);
}

/** Deletes the connection again, after the server agreed on the first instance. */
async function disconnect(api: APIRequestContext): Promise<void> {
  const settings = (await (await api.get('/api/v1/admin/seerr')).json()) as { urlSource: string };
  if (settings.urlSource === 'stored') await sameInstance(api);
  else await seerrControl('same');
  await send(api, 'put', '/admin/seerr', { clear: true, enabled: false });
}

test.afterEach(async ({ owner }) => {
  await disconnect(owner);
});

test('the tile opens the view; connect, test and save by keyboard @keyboard', async ({ page }) => {
  await page.goto('/admin/integrations');
  await expect(page.getByRole('heading', { level: 1, name: 'Integrationen' })).toBeVisible();
  const links = page.getByRole('list', { name: 'Dienste' }).getByRole('link');
  await expect(links.nth(1)).toHaveText('Webhooks von Radarr und Sonarr');
  await expect(links.nth(2)).toHaveText('Seerr');
  const tile = tileLink(page, 'Seerr');
  const tiles = page.getByRole('list', { name: 'Dienste' });
  await expect(tiles.getByText('Seerr: Nicht eingerichtet')).toBeVisible();

  await resetFocus(page);
  await tabTo(page, tile, 40);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin\/integrations\?item=seerr$/);
  const seerr = region(page, 'Seerr');
  await expectFocusOn(page, seerr.getByRole('heading', { level: 2, name: 'Seerr' }));
  await expect(seerr.getByText('Trage Adresse und Schlüssel ein.')).toBeVisible();

  // Not set up: «Verbindung» is open, «Protokoll» closed (design rule 1.3).
  const connection = seerr.getByRole('button', { name: 'Verbindung', exact: true });
  await page.keyboard.press('Tab');
  await expectFocusOn(page, connection);
  await expect(connection).toHaveAttribute('aria-expanded', 'true');
  const use = seerr.getByRole('switch', { name: 'Seerr verwenden' });
  await page.keyboard.press('Tab');
  await expectFocusOn(page, use);
  await page.keyboard.press('Space');
  await expect(use).toBeChecked();
  await page.keyboard.press('Tab');
  await expectFocusOn(page, field(page, 'Adresse'));
  await page.keyboard.type(SEERR_BASE);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, field(page, 'API-Schlüssel'));
  await page.keyboard.type(SEERR_API_KEY);
  await expect(seerr.getByText('ungespeichert').first()).toBeVisible();

  // «Verbindung testen» checks what was typed: everything passes, one hint (seasons from TMDB).
  const testButton = seerr.getByRole('button', { name: 'Verbindung testen' });
  await page.keyboard.press('Tab');
  await expectFocusOn(page, testButton);
  await page.keyboard.press('Enter');
  const checks = seerr.getByRole('list', { name: 'Ergebnis des Verbindungstests' });
  await expect(checks).toBeVisible();
  await expect(seerr.getByText('Verbunden · 1 Hinweis')).toBeVisible();
  await expect(checks.getByText('Seerr 3.5.1')).toBeVisible();
  await expect(checks.getByText(/Seerr liest Staffeln aus TMDB/)).toBeVisible();

  // «Speichern» stores the three values; the view tests the saved ones right after.
  const save = seerr.getByRole('button', { name: 'Verbindung zu Seerr speichern' });
  await tabTo(page, save, 4);
  await page.keyboard.press('Enter');
  await expect(seerr.getByText('Verbindung zu Seerr gespeichert.')).toBeVisible();
  await expect(seerr.getByText('Seerr: Eingerichtet')).toBeVisible();
  await expect(seerr.getByText('Version 3.5.1')).toBeVisible();
  await expect(field(page, 'API-Schlüssel')).toHaveValue('');
  await expect(field(page, 'API-Schlüssel')).toHaveAttribute(
    'placeholder',
    `Gespeichert, endet auf ${SEERR_API_KEY.slice(-4)}`,
  );
  await expect(seerr.getByText('ungespeichert')).toHaveCount(0);

  // The call log folds out with Enter and away with Space; the focus stays on its toggle.
  // On the way, the band «Radarr und Sonarr» stands open: Radarr and Sonarr from Seerr wait for
  // «Testen» (stage 4).
  const log = seerr.getByRole('button', { name: 'Protokoll', exact: true });
  await tabTo(page, log, 20);
  await expect(log).toHaveAttribute('aria-expanded', 'false');
  await page.keyboard.press('Enter');
  await expect(log).toHaveAttribute('aria-expanded', 'true');
  const calls = seerr.getByRole('table', { name: 'Die letzten Aufrufe an Seerr' });
  await expect(calls.getByText('GET /status').first()).toBeVisible();
  await expect(calls.getByText(/^\d+ ms$/).first()).toBeVisible();
  // No text shows a placeholder its call did not fill.
  await expect(page.locator('#content')).not.toContainText('{{');
  await expectFocusOn(page, log);
  await page.keyboard.press('Space');
  await expect(log).toHaveAttribute('aria-expanded', 'false');
  await expectFocusOn(page, log);
  await expectNoFocusTrap(page, 60);

  // Back to the tiles: the focus returns to the tile, which shows the new state.
  const back = page.getByRole('link', { name: 'Alle Integrationen' });
  await back.focus();
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin\/integrations$/);
  await expectFocusOn(page, tile);
  await expect(tiles.getByText('Seerr: Eingerichtet')).toBeVisible();
});

test('a wrong key, a bad address and a new host without the key are named', async ({
  page,
  owner,
}) => {
  await page.goto('/admin/integrations?item=seerr');
  const seerr = region(page, 'Seerr');
  await field(page, 'Adresse').fill(SEERR_BASE);
  await field(page, 'API-Schlüssel').fill('e2e-wrong-key');
  await seerr.getByRole('button', { name: 'Verbindung testen' }).click();
  await expect(seerr.getByText('Der Test ist gescheitert: Schlüssel gültig.')).toBeVisible();
  await expect(seerr.getByText('Seerr lehnt den Schlüssel ab.')).toBeVisible();

  await field(page, 'Adresse').fill('ftp://seerr.example.org');
  await seerr.getByRole('button', { name: 'Verbindung zu Seerr speichern' }).click();
  await expect(field(page, 'Adresse')).toHaveAttribute('aria-invalid', 'true');
  await expect(field(page, 'Adresse')).toHaveAccessibleDescription(/http:\/\/ oder https:\/\//);

  // A saved connection: another host needs the key again, the stored one never goes there.
  await connect(owner);
  await page.reload();
  await seerr.getByRole('button', { name: 'Verbindung', exact: true }).click();
  await field(page, 'Adresse').fill(SEERR_BASE.replace('127.0.0.1', 'localhost'));
  await seerr.getByRole('button', { name: 'Verbindung zu Seerr speichern' }).click();
  await expect(field(page, 'API-Schlüssel')).toHaveAttribute('aria-invalid', 'true');
  await expect(field(page, 'API-Schlüssel')).toHaveAccessibleDescription(
    /braucht den API-Schlüssel neu/,
  );
  // «Verwerfen» drops the address and the message with it.
  await seerr.getByRole('button', { name: 'Verwerfen' }).click();
  await expect(field(page, 'Adresse')).toHaveValue(SEERR_BASE);
  await expect(field(page, 'API-Schlüssel')).not.toHaveAttribute('aria-invalid', 'true');
});

test('another instance waits for a decision; «Zuordnung löschen» asks first', async ({
  page,
  owner,
}) => {
  await connect(owner);
  await seerrControl('other');
  await send(owner, 'post', '/admin/seerr/test', {});
  await page.goto('/admin/integrations?item=seerr');
  const seerr = region(page, 'Seerr');
  const notice = seerr.getByRole('alert').filter({ hasText: 'Andere Seerr-Instanz erkannt' });
  await expect(notice).toBeVisible();
  await expect(seerr.getByText('Seerr: Fehler')).toBeVisible();

  // «Zuordnung löschen» asks; Escape keeps everything and gives the focus back.
  const reset = notice.getByRole('button', { name: 'Zuordnung löschen' });
  await reset.focus();
  await page.keyboard.press('Enter');
  const question = page.getByRole('alertdialog', { name: 'Zuordnung zu Seerr löschen?' });
  await expect(question).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(question).toHaveCount(0);
  await expectFocusOn(page, reset);

  const confirm = notice.getByRole('button', { name: 'Zuordnung bestätigen' });
  await page.keyboard.press('Shift+Tab');
  await expectFocusOn(page, confirm);
  await page.keyboard.press('Enter');
  await expect(seerr.getByText(/Neue Instanz übernommen/)).toBeVisible();
  await expect(notice).toHaveCount(0);
  // The buttons went with the notice: the focus moves to the page's heading.
  await expect(page.getByRole('heading', { level: 1, name: 'Integrationen' })).toBeFocused();

  // The mock answers as the first instance again: the server takes that one back over.
  await sameInstance(owner);
});

/** Where the coordinator's visual check finds the screenshots (SLOUCHCAT_E2E_SHOTS), if set. */
const SHOTS = process.env.SLOUCHCAT_E2E_SHOTS;
async function shot(page: Page, name: string): Promise<void> {
  if (!SHOTS) return;
  // A dialog that just opened plays its entry animation first; endless ones do not count.
  await page.waitForFunction(() =>
    document
      .getAnimations()
      .every(
        (animation) =>
          animation.playState !== 'running' ||
          animation.effect?.getComputedTiming().endTime === Infinity,
      ),
  );
  await page.screenshot({ path: `${SHOTS}/${name}.png` });
}

/** The view's text without the code to copy: Seerr's JSON template has «{{…}}» of its own. */
const textOutsideCode = (page: Page) =>
  page.locator('#content').evaluate((content) => {
    const copy = content.cloneNode(true) as HTMLElement;
    copy.querySelectorAll('pre').forEach((code) => code.remove());
    return copy.textContent;
  });

/** The webhook integrations of the kind `seerr` the test made, deleted again. */
async function deleteSeerrHooks(api: APIRequestContext): Promise<void> {
  const list = (await (await api.get('/api/v1/admin/integrations')).json()) as {
    id: number;
    kind: string;
  }[];
  for (const integration of list.filter((entry) => entry.kind === 'seerr')) {
    const response = await api.delete(`/api/v1/admin/integrations/${integration.id}`, {
      headers: CSRF,
    });
    expect(response.ok()).toBe(true);
  }
}

// Stage 4 (M-22): the band «Radarr und Sonarr» with «Testen», which confirms the host from Seerr,
// an own address saved with the band's «Speichern» and refused when it is no web address, Seerr
// without a Radarr server; the band «Sofortmeldung» makes the address of the webhook integration
// of the kind `seerr`, shows the template and the types to copy, and its log shows the notice that
// queued the sync. All by keyboard.
test('Radarr and Sonarr: test, own address, not in Seerr; the instant notice by keyboard @keyboard', async ({
  page,
  owner,
}) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await arrReset();
  await connect(owner);
  try {
    await page.goto('/admin/integrations?item=seerr');
    const seerr = region(page, 'Seerr');
    // The address from Seerr waits for «Testen»: the band is open (design rule 1.3).
    const arrBand = seerr.getByRole('button', {
      name: 'Radarr und Sonarr (Profil-Tags und Fortschritt)',
    });
    await expect(arrBand).toHaveAttribute('aria-expanded', 'true');
    const radarr = seerr.getByRole('region', { name: 'Radarr', exact: true });
    const sonarr = seerr.getByRole('region', { name: 'Sonarr', exact: true });
    await expect(radarr.getByText('Radarr wartet auf «Testen»')).toBeVisible();
    await expect(radarr.getByText(`Aus Seerr: ${RADARR_BASE}`)).toBeVisible();
    // Never a key.
    await expect(page.locator('#content')).not.toContainText(ARR_API_KEY);

    // «Testen» by keyboard confirms the host.
    const testRadarr = radarr.getByRole('button', { name: 'Radarr testen' });
    await resetFocus(page);
    await tabTo(page, testRadarr, 60);
    await page.keyboard.press('Enter');
    await expect(
      radarr.getByText(
        `Radarr antwortet unter ${RADARR_BASE}, der Schlüssel aus Seerr gilt. Die Adresse ist bestätigt.`,
      ),
    ).toBeVisible();
    await expect(radarr.getByText('Radarr erreichbar', { exact: true })).toBeVisible();
    await expectFocusOn(page, testRadarr);
    await shot(page, 'seerr-radarr-test-desktop');

    // An own address for Sonarr that does not answer: saved, then tested and named.
    const own = seerr.getByRole('textbox', { name: 'Eigene Adresse für Sonarr' });
    await tabTo(page, own, 20);
    await page.keyboard.type('ftp://sonarr.example.org');
    const save = seerr.getByRole('button', {
      name: 'Eigene Adressen von Radarr und Sonarr speichern',
    });
    await tabTo(page, save, 6);
    await page.keyboard.press('Enter');
    await expect(own).toHaveAttribute('aria-invalid', 'true');
    const dead = 'http://127.0.0.1:9';
    await own.fill(dead);
    await save.focus();
    await page.keyboard.press('Enter');
    await expect(seerr.getByText('Eigene Adressen gespeichert.')).toBeVisible();
    await expect(sonarr.getByText(`Eigene Adresse: ${dead}`)).toBeVisible();
    await sonarr.getByRole('button', { name: 'Sonarr testen' }).focus();
    await page.keyboard.press('Enter');
    await expect(sonarr.getByText(/^Sonarr ist unter .* nicht erreichbar\.$/)).toBeVisible();
    // Empty goes back to the address from Seerr.
    await own.fill('');
    await save.focus();
    await page.keyboard.press('Enter');
    await expect(sonarr.getByText(`Aus Seerr: ${SONARR_BASE}`)).toBeVisible();

    // Seerr without a Radarr server: the test says so.
    await seerrServers(false);
    await testRadarr.focus();
    await page.keyboard.press('Enter');
    // The outcome's title; the state line below names the same reason once the status is read.
    await expect(
      radarr
        .getByRole('paragraph')
        .filter({ hasText: /^Seerr hat keinen Standardserver für Radarr\.$/ }),
    ).toBeVisible();
    await seerrServers(true);
    await page.keyboard.press('Enter');
    await expect(radarr.getByText(/^Radarr antwortet unter /)).toBeVisible();
    expect(await textOutsideCode(page)).not.toContain('{{');

    // The band folds away with Space and opens again with Enter; the focus stays on its toggle.
    await arrBand.focus();
    await page.keyboard.press('Space');
    await expect(arrBand).toHaveAttribute('aria-expanded', 'false');
    await expectFocusOn(page, arrBand);

    // «Sofortmeldung»: closed at first; «Adresse erzeugen» shows the address once.
    const hookBand = seerr.getByRole('button', { name: 'Sofortmeldung', exact: true });
    await expect(hookBand).toHaveAttribute('aria-expanded', 'false');
    await tabTo(page, hookBand, 6);
    await page.keyboard.press('Enter');
    await expect(hookBand).toHaveAttribute('aria-expanded', 'true');
    const create = seerr.getByRole('button', { name: 'Adresse erzeugen' });
    await tabTo(page, create, 6);
    await page.keyboard.press('Enter');
    const address = seerr.getByRole('textbox', { name: 'Vollständige Adresse' });
    await expect(address).toBeFocused();
    const url = await address.inputValue();
    expect(url).toMatch(/\/api\/v1\/webhooks\//);
    await expect(seerr.getByRole('group', { name: 'JSON-Vorlage für Seerr' })).toContainText(
      'notification_type',
    );
    await expect(
      seerr.getByRole('group', { name: 'Benachrichtigungsarten für Seerr' }),
    ).toContainText('MEDIA_PENDING');
    await expect(seerr.getByRole('button', { name: 'Vorlage kopieren' })).toBeVisible();
    await expect(seerr.getByRole('button', { name: 'Arten kopieren' })).toBeVisible();

    // Seerr reports: the sync is queued, the log says so.
    // Without a public address the address is relative to the server.
    const sent = await fetch(new URL(url, BASE_URL), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ notification_type: 'MEDIA_APPROVED' }),
    });
    expect(sent.ok).toBe(true);
    await page.reload();
    await seerr.getByRole('button', { name: 'Sofortmeldung', exact: true }).click();
    // Sonarr still waits for «Testen»: the band opens on its own again.
    await expect(arrBand).toHaveAttribute('aria-expanded', 'true');
    await expect(seerr.getByText('Wünsche werden abgeglichen')).toBeVisible();
    expect(await textOutsideCode(page)).not.toContain('{{');
    await shot(page, 'seerr-bands-open-desktop');
    await page.setViewportSize({ width: 390, height: 844 });
    await shot(page, 'seerr-bands-open-phone');
    await page.setViewportSize({ width: 1280, height: 900 });
    await expectNoFocusTrap(page, 120);
  } finally {
    await seerrServers(true);
    await deleteSeerrHooks(owner);
    await send(owner, 'put', '/admin/seerr/arr', { radarrUrl: '', sonarrUrl: '' });
    await arrReset();
  }
});
