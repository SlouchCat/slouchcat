import type { APIRequestContext, Page } from '@playwright/test';
import { CSRF } from './catalog';
import { MOVIES_DIR } from './env';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// M9 webhooks (I-8, decision 5.6) against the real server: an integration is created by keyboard,
// its address is shown once with a copy button, a real POST to that address queues a targeted scan
// and lands in the event log, and renewing or deleting the integration asks first. Every test
// removes the integrations it made.

interface IntegrationData {
  id: number;
  name: string;
  kind: string;
  tokenTail: string;
  pathMap: { from: string; to: string }[];
  lastSeenAt?: string;
}

interface CreatedData {
  integration: IntegrationData;
  url: string;
  token: string;
}

interface EventData {
  id: number;
  event: string;
  reportedPath?: string;
  resolvedPath?: string;
  result: string;
}

const region = (page: Page, name: string) => page.getByRole('region', { name, exact: true });

async function listIntegrations(api: APIRequestContext): Promise<IntegrationData[]> {
  const response = await api.get('/api/v1/admin/integrations');
  expect(response.status(), await response.text()).toBe(200);
  return (await response.json()) as IntegrationData[];
}

async function removeIntegrations(api: APIRequestContext, name: string): Promise<void> {
  for (const integration of await listIntegrations(api)) {
    if (integration.name !== name) continue;
    const response = await api.delete(`/api/v1/admin/integrations/${integration.id}`, {
      headers: CSRF,
    });
    expect(response.status(), await response.text()).toBe(204);
  }
}

async function createIntegration(
  api: APIRequestContext,
  name: string,
  kind: string,
): Promise<CreatedData> {
  const response = await api.post('/api/v1/admin/integrations', {
    headers: CSRF,
    data: { name, kind },
  });
  expect(response.status(), await response.text()).toBe(201);
  return (await response.json()) as CreatedData;
}

async function eventsOf(api: APIRequestContext, id: number): Promise<EventData[]> {
  const response = await api.get(`/api/v1/admin/integrations/${id}/events`);
  expect(response.status(), await response.text()).toBe(200);
  return (await response.json()) as EventData[];
}

async function openIntegrations(page: Page) {
  await page.goto('/admin/integrations');
  await expect(page.getByRole('heading', { level: 1, name: 'Integrationen' })).toBeVisible();
  await expect(region(page, 'Webhooks von Radarr und Sonarr')).toBeVisible();
}

test('an integration is created by keyboard and its address is shown exactly once', async ({
  page,
  owner,
}) => {
  const name = 'E2E Radarr';
  try {
    await openIntegrations(page);
    const webhooks = region(page, 'Webhooks von Radarr und Sonarr');

    // Tab reaches the form; the name field, the kind and the button follow one another.
    await resetFocus(page);
    await tabTo(page, webhooks.getByLabel('Name'), 120);
    await expectVisibleFocus(page);
    await page.keyboard.type(name);
    await page.keyboard.press('Tab');
    const radarr = webhooks.getByRole('radio', { name: 'Radarr' });
    const sonarr = webhooks.getByRole('radio', { name: 'Sonarr' });
    await expectFocusOn(page, radarr);
    // The kind is a radio group: the arrow keys move and select inside it, Tab leaves it.
    await page.keyboard.press('ArrowDown');
    await expectFocusOn(page, sonarr);
    await expect(sonarr).toBeChecked();
    await page.keyboard.press('ArrowUp');
    await expectFocusOn(page, radarr);
    await expect(radarr).toBeChecked();
    await page.keyboard.press('Tab');
    const submit = webhooks.getByRole('button', { name: 'Integration anlegen' });
    await expectFocusOn(page, submit);
    await page.keyboard.press('Enter');

    // The address appears once, with the warning; the field has the focus and is selected.
    const created = region(page, `Adresse für «${name}»`);
    await expect(created).toBeVisible();
    await expect(
      created.getByText('Jetzt kopieren — danach ist die Adresse nicht mehr sichtbar.'),
    ).toBeVisible();
    const address = created.getByLabel('Vollständige Adresse');
    await expectFocusOn(page, address);
    const url = await address.inputValue();
    expect(url).toContain('/api/v1/webhooks/');

    // Hiding it is final: only the last four characters stay.
    const tail = url.slice(-4);
    await created.getByRole('button', { name: 'Ausblenden' }).focus();
    await page.keyboard.press('Enter');
    await expect(created).toHaveCount(0);
    await expect(webhooks.getByText(`Adresse endet auf …${tail}`)).toBeVisible();
    await expect(webhooks.getByText('Hat noch nie etwas gemeldet.')).toBeVisible();
    await expect(page.getByText(url)).toHaveCount(0);

    const stored = (await listIntegrations(owner)).find((i) => i.name === name);
    expect(stored?.kind).toBe('radarr');
    expect(stored?.tokenTail).toBe(tail);
    await expectNoFocusTrap(page, 160);
  } finally {
    await removeIntegrations(owner, name);
  }
});

test('a webhook to the address scans the reported directory and lands in the event log', async ({
  page,
  owner,
  request,
}) => {
  const name = 'E2E Import';
  try {
    const created = await createIntegration(owner, name, 'radarr');
    // The reported path is mapped onto the movie library of the suite.
    const mapped = await owner.put(
      `/api/v1/admin/integrations/${created.integration.id}/path-map`,
      {
        headers: CSRF,
        data: { pathMap: [{ from: '/movies', to: MOVIES_DIR }] },
      },
    );
    expect(mapped.status(), await mapped.text()).toBe(200);

    // Radarr sends no CSRF header and no session: the token in the path is the authorisation.
    const hook = await request.post(created.url, {
      data: { eventType: 'Download', movie: { folderPath: '/movies' } },
    });
    expect(hook.status(), await hook.text()).toBe(200);
    expect((await hook.json()) as { result: string }).toMatchObject({ result: 'scan_queued' });

    // Test is acknowledged, an unknown event accepted without an effect.
    for (const [eventType, result] of [
      ['Test', 'acknowledged'],
      ['Health', 'ignored'],
    ] as const) {
      const response = await request.post(created.url, { data: { eventType } });
      expect(response.status(), await response.text()).toBe(200);
      expect((await response.json()) as { result: string }).toMatchObject({ result });
    }

    // A wrong token is refused, a path outside every library root reported as such.
    const wrong = await request.post(
      created.url.slice(0, -1) + (created.url.endsWith('A') ? 'B' : 'A'),
      {
        data: { eventType: 'Test' },
      },
    );
    expect(wrong.status()).toBe(401);
    const outside = await request.post(created.url, {
      data: { eventType: 'Download', movie: { folderPath: '/nirgendwo/Film' } },
    });
    expect(outside.status()).toBe(422);
    expect((await outside.json()) as { code: string }).toMatchObject({
      code: 'path_outside_roots',
    });

    const log = await eventsOf(owner, created.integration.id);
    expect(log.map((entry) => entry.result)).toEqual([
      'outside_roots',
      'ignored',
      'acknowledged',
      'scan_queued',
    ]);
    expect(log[0]?.reportedPath).toBe('/nirgendwo/Film');
    expect(log[0]?.resolvedPath).toBe('/nirgendwo/Film');
    expect(log[3]?.resolvedPath).toBe(MOVIES_DIR);

    // The page shows the same log, and the last contact is no longer empty.
    await openIntegrations(page);
    const webhooks = region(page, 'Webhooks von Radarr und Sonarr');
    await expect(webhooks.getByText(/Letzte Meldung:/)).toBeVisible();
    const table = webhooks.getByRole('table', { name: `Meldungen von ${name}` });
    await expect(table.getByText('Ausserhalb der Bibliotheken')).toBeVisible();
    await expect(table.getByText('Ordner wird gelesen')).toBeVisible();
    await expect(table.getByText('Test quittiert')).toBeVisible();

    // The log is a read-only table: it holds no controls, so Tab passes it by instead of
    // stopping in it, and the next stop is the button below.
    await resetFocus(page);
    await tabTo(page, webhooks.getByRole('button', { name: 'Zeile hinzufügen' }), 160);
    await expectVisibleFocus(page);
    await expect(table.getByRole('button')).toHaveCount(0);
    await expect(table.getByRole('link')).toHaveCount(0);
    await expectNoFocusTrap(page, 200);
  } finally {
    await removeIntegrations(owner, name);
  }
});

test('the path mapping is saved by keyboard and a relative path is refused', async ({
  page,
  owner,
}) => {
  const name = 'E2E Pfade';
  try {
    await createIntegration(owner, name, 'sonarr');
    await openIntegrations(page);
    const webhooks = region(page, 'Webhooks von Radarr und Sonarr');

    await webhooks.getByRole('button', { name: 'Zeile hinzufügen' }).focus();
    await page.keyboard.press('Enter');
    const from = webhooks.getByLabel('Pfad im Dienst');
    const to = webhooks.getByLabel('Pfad auf diesem Server');
    await from.focus();
    await page.keyboard.type('relativ');
    await page.keyboard.press('Tab');
    await expectFocusOn(page, to);
    await page.keyboard.type(MOVIES_DIR);
    await webhooks.getByRole('button', { name: 'Speichern' }).focus();
    await page.keyboard.press('Enter');
    await expect(from).toHaveAttribute('aria-invalid', 'true');
    expect((await listIntegrations(owner)).find((i) => i.name === name)?.pathMap).toEqual([]);

    await from.fill('/tv');
    await webhooks.getByRole('button', { name: 'Speichern' }).focus();
    await page.keyboard.press('Enter');
    await expect(webhooks.getByText('Pfad-Umsetzung gespeichert.')).toBeVisible();
    expect((await listIntegrations(owner)).find((i) => i.name === name)?.pathMap).toEqual([
      { from: '/tv', to: MOVIES_DIR },
    ]);
  } finally {
    await removeIntegrations(owner, name);
  }
});

test('renewing and deleting ask first; Escape keeps everything', async ({ page, owner }) => {
  const name = 'E2E Token';
  try {
    const created = await createIntegration(owner, name, 'radarr');
    await openIntegrations(page);
    const webhooks = region(page, 'Webhooks von Radarr und Sonarr');

    const renew = webhooks.getByRole('button', { name: `Token von ${name} erneuern` });
    await resetFocus(page);
    await tabTo(page, renew, 200);
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('alertdialog', { name: `Token von «${name}» erneuern?` });
    await expect(dialog).toContainText('hört sofort auf zu arbeiten');
    await expectVisibleFocus(page);
    await page.keyboard.press('Escape');
    await expect(dialog).toHaveCount(0);
    await expectFocusOn(page, renew);
    expect((await listIntegrations(owner)).find((i) => i.name === name)?.tokenTail).toBe(
      created.integration.tokenTail,
    );

    // Confirmed, the new address appears once and the old token stops working.
    await page.keyboard.press('Enter');
    await expect(dialog.getByRole('button', { name: 'Erneuern' })).toBeFocused();
    await page.keyboard.press('Enter');
    const panel = region(page, `Adresse für «${name}»`);
    await expect(panel).toBeVisible();
    const fresh = await panel.getByLabel('Vollständige Adresse').inputValue();
    expect(fresh).not.toBe(created.url);
    expect((await listIntegrations(owner)).find((i) => i.name === name)?.tokenTail).toBe(
      fresh.slice(-4),
    );

    // Deleting asks as well; after the confirmation the integration is gone. The revealed
    // address is hidden first, so its heading cannot be mistaken for the integration's own.
    await region(page, `Adresse für «${name}»`).getByRole('button', { name: 'Ausblenden' }).click();
    const remove = webhooks.getByRole('button', { name: `${name} löschen` });
    await remove.focus();
    await page.keyboard.press('Enter');
    const confirm = page.getByRole('alertdialog', { name: `«${name}» löschen?` });
    await expect(confirm.getByRole('button', { name: 'Löschen' })).toBeFocused();
    await page.keyboard.press('Enter');
    await expect(webhooks.getByRole('heading', { level: 3, name, exact: true })).toHaveCount(0);
    expect((await listIntegrations(owner)).find((i) => i.name === name)).toBeUndefined();
  } finally {
    await removeIntegrations(owner, name);
  }
});

test('the dashboard tile counts the reports of the last 24 hours', async ({
  page,
  owner,
  request,
}) => {
  const name = 'E2E Kachel';
  try {
    const created = await createIntegration(owner, name, 'radarr');
    const hook = await request.post(created.url, { data: { eventType: 'Test' } });
    expect(hook.status(), await hook.text()).toBe(200);

    await page.goto('/admin');
    await expect(page.getByRole('heading', { level: 1, name: 'Verwaltung' })).toBeVisible();
    const tile = page.getByRole('region', { name: 'Webhooks', exact: true });
    // One webhook arrived above, so the tile names it and the single integration it came from.
    await expect(tile).toContainText('1 Meldung im Protokoll der letzten 24 Stunden');
    const manage = tile.getByRole('link', { name: 'Webhooks verwalten' });
    await manage.focus();
    await expectVisibleFocus(page);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/admin\/integrations$/);
    await expect(region(page, 'Webhooks von Radarr und Sonarr')).toBeVisible();

    // Browser back returns to the overview the tile sits on.
    await page.goBack();
    await expect(page).toHaveURL(/\/admin$/);
    await expect(page.getByRole('heading', { level: 1, name: 'Verwaltung' })).toBeVisible();
  } finally {
    await removeIntegrations(owner, name);
  }
});
