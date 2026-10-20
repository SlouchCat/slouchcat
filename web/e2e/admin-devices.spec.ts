import type { Page } from '@playwright/test';
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
import { H264_TITLE, itemIdByTitle, resetProgress, startOf, player } from './playback';

// W-7: after a playback the device shows owner, last contact (±1 min), containers, codecs,
// maximum resolution and HDR as the browser reported them. "Vergessen" removes a device; the
// spec forgets a throw-away device of its own, never the shared session's.

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

const deviceRow = (page: Page, name: string) =>
  page.getByRole('rowheader', { name, exact: true }).locator('xpath=..');

test('after a playback the device row shows owner, last contact and the reported profile', async ({
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

  await page.goto('/admin/devices');
  await expect(page.getByRole('heading', { level: 1, name: 'Geräte' })).toBeVisible();
  const row = deviceRow(page, device!.name ?? 'Unbekanntes Gerät').first();
  await expect(row).toContainText(`${OWNER.email} · ${OWNER_PROFILE}`);
  await expect(row).toContainText(upper(reported.containers));
  await expect(row).toContainText(upper(reported.video));
  await expect(row).toContainText(upper(reported.audio));
  await expect(row).toContainText(`${reported.maxWidth} × ${reported.maxHeight}`);
  await expect(row).toContainText(reported.hdr ? 'Ja' : 'Nein');
  const lastSeen = await row.locator('time').getAttribute('datetime');
  expect(Math.abs(Date.parse(lastSeen ?? '') - Date.now())).toBeLessThanOrEqual(60_000);
});

test('forgets a throw-away device by keyboard; its session ends', async ({
  page,
  owner,
  browser,
  baseURL,
  extraHTTPHeaders,
}) => {
  // A browser of its own (a device cookie of its own) that starts one playback.
  const name = `E2E Wegwerf ${test.info().project.name}`;
  const throwaway = await browser.newContext({ baseURL, locale: 'de-CH', extraHTTPHeaders });
  try {
    await loginOwner(throwaway.request);
    const itemId = await itemIdByTitle(owner, H264_TITLE);
    const started = await throwaway.request.post(`/api/v1/items/${itemId}/play`, {
      headers: CSRF,
      data: {
        capabilities: { video: ['h264'], audio: ['aac'], hls: 'mse', name, type: 'other' },
      },
    });
    expect(started.status(), await started.text()).toBe(200);

    await page.goto('/admin/devices');
    const table = page.getByRole('table', { name: 'Geräte' });
    await expect(table).toBeVisible();
    await expect(deviceRow(page, name)).toContainText('Anderes');

    // From the top of the page to the first "Vergessen", then ↓ to the throw-away device.
    await resetFocus(page);
    const forget = page.getByRole('button', { name: `${name} vergessen` });
    await tabTo(page, page.getByRole('button', { name: /vergessen$/ }).first(), 40);
    for (let presses = 0; presses < 20; presses += 1) {
      if (await forget.evaluate((element) => element === document.activeElement)) break;
      await page.keyboard.press('ArrowDown');
    }
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
    await expect(deviceRow(page, name)).toHaveCount(0);
    // The device's session is signed out.
    const session = await throwaway.request.get('/api/v1/auth/session');
    expect(session.status()).toBe(401);
    await expectNoFocusTrap(page);
  } finally {
    await throwaway.close();
  }
});
