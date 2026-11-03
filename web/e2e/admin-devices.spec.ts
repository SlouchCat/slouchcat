import type { APIRequestContext, Browser, BrowserContext, Page } from '@playwright/test';
import { loginOwner } from './accounts';
import { CSRF } from './catalog';
import { OWNER, OWNER_PROFILE } from './env';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';
import {
  arrowBetweenRows,
  H264_TITLE,
  itemIdByTitle,
  player,
  resetProgress,
  startOf,
} from './playback';

// W-7 (concept 12, variant C): after a playback the device shows owner and last contact (±1 min)
// in the tab «Geräte», containers, codecs, maximum resolution and HDR as the browser reported them
// in the tab «Fähigkeiten»; both tabs group the devices by type. "Vergessen" removes a device; the
// spec forgets throw-away devices of its own, never the shared session's.

interface DeviceData {
  id: string;
  name?: string;
  ownerEmail?: string;
  ownerProfile?: string;
  lastSeenAt: string;
  capabilities?: {
    containers?: string[];
    video: string[];
    audio: string[];
    maxWidth?: number;
    maxHeight?: number;
    hdr?: boolean;
  };
}

const upper = (list: string[] | undefined) => (list ?? []).map((v) => v.toUpperCase()).join(', ');

/** The row of a device by its id (DataTable's data-row-key): names repeat across contexts. */
const deviceRow = (page: Page, id: string) => page.locator(`tr[data-row-key="${id}"]`);

const tabList = (page: Page) => page.getByRole('tablist', { name: 'Ansicht der Geräte' });

/**
 * A browser of its own. `browser.newContext` applies the config's `use` options, so the storage
 * state (and with it the shared session's device cookie) is emptied explicitly: forgetting its
 * device must not sign the shared session out.
 */
function throwawayContext(
  browser: Browser,
  options: { baseURL?: string; extraHTTPHeaders?: Record<string, string> },
): Promise<BrowserContext> {
  return browser.newContext({
    ...options,
    locale: 'de-CH',
    storageState: { cookies: [], origins: [] },
  });
}

/** Signs `context` in and starts one playback there, reported as `type`; its device id. */
async function playOn(
  context: BrowserContext,
  owner: APIRequestContext,
  name: string,
  type = 'other',
): Promise<string> {
  await loginOwner(context.request);
  const itemId = await itemIdByTitle(owner, H264_TITLE);
  const started = await context.request.post(`/api/v1/items/${itemId}/play`, {
    headers: CSRF,
    data: { capabilities: { video: ['h264'], audio: ['aac'], hls: 'mse', name, type } },
  });
  expect(started.status(), await started.text()).toBe(200);
  const id = ((await started.json()) as { device?: { id: string } }).device?.id;
  expect(id, 'PlaybackInfo.device').toBeTruthy();
  return id!;
}

test('after a playback the device rows show owner, last contact and the reported profile', async ({
  page,
  owner,
}) => {
  const itemId = await itemIdByTitle(owner, H264_TITLE);
  await resetProgress(page.request, itemId, 10_000);
  const start = await startOf(page, () => page.goto(`/play/${itemId}`));
  expect(start.status).toBe(200);
  await expect(player(page)).toHaveAttribute('data-start-ms', /\d+/);
  const reported = start.request.capabilities;
  const deviceId = (start.body as { device?: { id: string } }).device?.id;
  expect(deviceId, 'PlaybackInfo.device').toBeTruthy();

  const devices = (await (await owner.get('/api/v1/admin/devices')).json()) as DeviceData[];
  const device = devices.find((candidate) => candidate.id === deviceId);
  expect(device).toBeDefined();
  expect(device!.ownerEmail).toBe(OWNER.email);
  expect(device!.ownerProfile).toBe(OWNER_PROFILE);
  expect(Math.abs(Date.parse(device!.lastSeenAt) - Date.now())).toBeLessThanOrEqual(60_000);
  expect(device!.capabilities?.video).toEqual(reported.video);
  expect(device!.capabilities?.maxHeight).toBe(reported.maxHeight);
  expect(device!.name).toBe(reported.name);

  // The tab «Geräte»: owner and last contact.
  await page.goto('/admin/devices');
  await expect(page.getByRole('heading', { level: 1, name: 'Geräte' })).toBeVisible();
  const row = deviceRow(page, deviceId!);
  await expect(row.getByRole('rowheader')).toHaveText(device!.name ?? 'Unbekanntes Gerät');
  await expect(row).toContainText(`${OWNER.email} · ${OWNER_PROFILE}`);
  const lastSeen = await row.locator('time').getAttribute('datetime');
  expect(Math.abs(Date.parse(lastSeen ?? '') - Date.now())).toBeLessThanOrEqual(60_000);

  // The tab «Fähigkeiten», linked by its address: the profile as the browser reported it.
  await page.goto('/admin/devices?tab=capabilities');
  await expect(tabList(page).getByRole('tab', { selected: true })).toHaveAccessibleName(
    /^Fähigkeiten \d+ Geräte? mit Fähigkeitsprofil$/,
  );
  await expect(row.getByRole('rowheader')).toHaveText(device!.name ?? 'Unbekanntes Gerät');
  await expect(row).toContainText(upper(reported.containers));
  await expect(row).toContainText(upper(reported.video));
  await expect(row).toContainText(upper(reported.audio));
  await expect(row).toContainText(`${reported.maxWidth} × ${reported.maxHeight}`);
  await expect(row).toContainText(reported.hdr ? 'Ja' : 'Nein');
});

test('tabs in the band by arrow keys, the type sections by Enter; forgets a throw-away device by keyboard @keyboard', async ({
  page,
  owner,
  browser,
  baseURL,
  extraHTTPHeaders,
}) => {
  const project = test.info().project.name;
  const name = `E2E Wegwerf ${project}`;
  // Everything made here is cleaned up, however far the test gets.
  const contexts: BrowserContext[] = [];
  const leftover = new Set<string>();
  try {
    // A second device of the same type: ↑/↓ need two rows in the section «Anderes».
    const keepContext = await throwawayContext(browser, { baseURL, extraHTTPHeaders });
    contexts.push(keepContext);
    leftover.add(await playOn(keepContext, owner, `E2E Wegwerf 2 ${project}`));
    const throwaway = await throwawayContext(browser, { baseURL, extraHTTPHeaders });
    contexts.push(throwaway);
    const throwawayId = await playOn(throwaway, owner, name);
    leftover.add(throwawayId);

    await page.goto('/admin/devices');
    const heading = page.getByRole('heading', { level: 1, name: 'Geräte' });
    await expect(heading).toBeVisible();
    // Devices of type `other` are in the section «Anderes».
    const others = page.getByRole('table', { name: 'Geräte: Anderes' });
    const row = others.locator(`tr[data-row-key="${throwawayId}"]`);
    await expect(row.getByRole('rowheader')).toHaveText(name);

    // ←/→ move between the tabs in the band and select them; the tab lives in the address.
    const tabs = tabList(page);
    const list = tabs.getByRole('tab', { name: /^Geräte/ });
    const capabilities = tabs.getByRole('tab', { name: /^Fähigkeiten/ });
    await resetFocus(page);
    await tabTo(page, list);
    await page.keyboard.press('ArrowRight');
    await expect(capabilities).toHaveAttribute('aria-selected', 'true');
    await expectFocusOn(page, capabilities);
    await expect(page).toHaveURL(/\/admin\/devices\?tab=capabilities$/);
    const profile = page
      .getByRole('table', { name: 'Fähigkeiten: Anderes' })
      .locator(`tr[data-row-key="${throwawayId}"]`);
    await expect(profile).toContainText('H264');
    await expect(profile).toContainText('AAC');
    await page.keyboard.press('ArrowLeft');
    await expect(list).toHaveAttribute('aria-selected', 'true');
    await expectFocusOn(page, list);
    await expect(page).toHaveURL(/\/admin\/devices$/);

    // The section «Anderes» folds away and opens again by Enter on its heading.
    const section = page.getByRole('button', { name: /^Anderes \d+ Geräte$/ });
    await tabTo(page, section, 60);
    await page.keyboard.press('Enter');
    await expect(section).toHaveAttribute('aria-expanded', 'false');
    await expect(others).toBeHidden();
    await expectVisibleFocus(page);
    await page.keyboard.press('Enter');
    await expect(section).toHaveAttribute('aria-expanded', 'true');
    await expect(others).toBeVisible();

    // From the section's heading to its first "Vergessen", then ↑/↓ to the throw-away device.
    const forget = row.getByRole('button', { name: `${name} vergessen` });
    await tabTo(page, others.getByRole('button', { name: /vergessen$/ }).first(), 20);
    await arrowBetweenRows(page, forget);
    await expectFocusOn(page, forget);
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('alertdialog', { name: `Gerät „${name}“ vergessen?` });
    await expect(dialog).toBeVisible();
    await expectVisibleFocus(page);
    await page.keyboard.press('Escape');
    await expect(dialog).toHaveCount(0);
    await expectFocusOn(page, forget);
    await page.keyboard.press('Enter');
    await expect(dialog.getByRole('button', { name: 'Gerät vergessen' })).toBeFocused();
    await page.keyboard.press('Enter');

    await expect(page.getByText(`Gerät „${name}“ vergessen.`)).toBeVisible();
    await expect(row).toHaveCount(0);
    leftover.delete(throwawayId);
    // The row and its button are gone: the focus is on the page's heading.
    await expect(heading).toBeFocused();
    // The device's session is signed out.
    const session = await throwaway.request.get('/api/v1/auth/session');
    expect(session.status()).toBe(401);
    await expectNoFocusTrap(page);
  } finally {
    for (const id of leftover) {
      await owner.delete(`/api/v1/admin/devices/${id}`, { headers: CSRF });
    }
    for (const context of contexts) await context.close();
  }
});

/** The left edge of each column header of a table, in its order. */
const headerLefts = (page: Page, name: string) =>
  page
    .getByRole('table', { name })
    .getByRole('columnheader')
    .evaluateAll((cells) => cells.map((cell) => cell.getBoundingClientRect().left));

test('the columns of the type sections line up in both tabs (K12-9)', async ({
  page,
  owner,
  browser,
  baseURL,
  extraHTTPHeaders,
}) => {
  const project = test.info().project.name;
  const contexts: BrowserContext[] = [];
  const leftover = new Set<string>();
  try {
    // Two sections whose rows differ in length: a long name and owner beside a short one.
    const other = await throwawayContext(browser, { baseURL, extraHTTPHeaders });
    contexts.push(other);
    leftover.add(
      await playOn(other, owner, `E2E Spalten mit einem recht langen Gerätenamen ${project}`),
    );
    const tablet = await throwawayContext(browser, { baseURL, extraHTTPHeaders });
    contexts.push(tablet);
    leftover.add(await playOn(tablet, owner, `E2E T ${project}`, 'tablet'));

    for (const [path, view] of [
      ['/admin/devices', 'Geräte'],
      ['/admin/devices?tab=capabilities', 'Fähigkeiten'],
    ] as const) {
      await page.goto(path);
      await expect(page.getByRole('table', { name: `${view}: Tablet` })).toBeVisible();
      await expect(page.getByRole('table', { name: `${view}: Anderes` })).toBeVisible();
      const tablets = await headerLefts(page, `${view}: Tablet`);
      const others = await headerLefts(page, `${view}: Anderes`);
      expect(others).toHaveLength(tablets.length);
      // The same column starts at the same x in both sections (1 px for rounding).
      tablets.forEach((left, column) =>
        expect(Math.abs(left - others[column]), `${view}, column ${column}`).toBeLessThanOrEqual(1),
      );
    }
  } finally {
    for (const id of leftover) {
      await owner.delete(`/api/v1/admin/devices/${id}`, { headers: CSRF });
    }
    for (const context of contexts) await context.close();
  }
});

test('an unknown tab shows «Geräte»; browser back leaves the page', async ({ page }) => {
  // Both pages must have rendered before going back (Firefox, see admin-libraries.spec).
  await page.goto('/');
  await expect(page.getByRole('heading', { level: 2, name: 'Neu hinzugefügt' })).toBeVisible();
  await page.goto('/admin/devices?tab=nonsense');
  const tabs = tabList(page);
  const list = tabs.getByRole('tab', { name: /^Geräte/ });
  await expect(list).toHaveAttribute('aria-selected', 'true');
  await expect(page).toHaveURL(/\/admin\/devices$/);

  // The tab is replaced in the address, not pushed: back leaves the page.
  await resetFocus(page);
  await tabTo(page, list);
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, tabs.getByRole('tab', { name: /^Fähigkeiten/ }));
  await expect(page).toHaveURL(/\/admin\/devices\?tab=capabilities$/);
  await expectNoFocusTrap(page);
  await page.goBack();
  await expect(page).toHaveURL(/\/$/);
});
