import { createProfile } from './accounts';
import { CSRF } from './catalog';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';
import {
  createDownload,
  downloadsTable,
  freshDevice,
  openOfflineLibrary,
  prepareOffline,
  removeDeviceDownloads,
  storedDownloads,
  waitForComplete,
} from './offline';
import { H264_TITLE, itemIdByTitle } from './playback';

// O-7: age ratings and library rights hold offline as online. P18 downloads a film without a
// rating (test media have none: hidden from profiles under 18, AN-9); P6 on the same device does
// not see it, neither on /downloads nor in the offline library, because the server leaves P6 out
// of `allowedProfileIds`. The storage panel shows what estimate() says (±5 %), and the Apple hint
// appears with an iPhone user agent.

const IPHONE =
  'Mozilla/5.0 (iPhone; CPU iPhone OS 19_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/19.0 Mobile/15E148 Safari/604.1';

test('P6 does not see the download of P18, online and offline (O-7)', async ({
  browser,
  baseURL,
  extraHTTPHeaders,
  owner,
}) => {
  test.setTimeout(240_000);
  const p18 = await createProfile(owner, { name: 'E2E P18', maxRating: 18 });
  const p6 = await createProfile(owner, { name: 'E2E P6', maxRating: 6 });
  const itemId = await itemIdByTitle(owner, H264_TITLE);
  const { context, page } = await freshDevice(browser, {
    baseURL,
    extraHTTPHeaders,
    profileId: p18.id,
  });
  try {
    await page.goto('/downloads');
    await createDownload(page, itemId);
    const [download] = await waitForComplete(page);
    expect(download.allowedProfileIds).toContain(p18.id);
    expect(download.allowedProfileIds).not.toContain(p6.id);
    await page.reload();
    await expect(
      downloadsTable(page).getByRole('rowheader', { name: new RegExp(H264_TITLE) }),
    ).toBeVisible();

    // Storage: the panel shows the browser's estimate (±5 %).
    const panel = page.getByRole('region', { name: 'Speicher' });
    await expect(page.getByRole('meter', { name: 'Speicherbelegung' })).toBeVisible();
    const shown = Number(await panel.getAttribute('data-usage-bytes'));
    const estimate = await page.evaluate(
      async () => (await navigator.storage.estimate()).usage ?? 0,
    );
    expect(Math.abs(shown - estimate) / estimate).toBeLessThanOrEqual(0.05);
    expect(Number(await panel.getAttribute('data-download-bytes'))).toBe(
      download.manifest!.sizeBytes,
    );
    await expect(panel.getByText('Hinweis für iPhone, iPad und Safari')).toHaveCount(0);

    // P6 on the same device: nothing to see.
    const selected = await context.request.post(`/api/v1/auth/profiles/${p6.id}/select`, {
      headers: CSRF,
    });
    expect(selected.status()).toBe(200);
    await page.goto('/downloads');
    await expect(page.getByRole('heading', { level: 1, name: 'Downloads' })).toBeVisible();
    await expect(page.getByText(/Noch keine Downloads auf diesem Gerät/)).toBeVisible();
    await expect(downloadsTable(page)).toHaveCount(0);
    // The file stays on the device (it is P18's), and the server still names only P18 and up.
    expect((await storedDownloads(page)).map((entry) => entry.id)).toEqual([download.id]);

    // Offline library: P18 is offered, P6 is not.
    await prepareOffline(page);
    await context.setOffline(true);
    await openOfflineLibrary(page);
    const tiles = page.getByRole('list', { name: 'Profile' });
    await expect(tiles.getByRole('button', { name: /E2E P18/ })).toBeVisible();
    await expect(tiles.getByRole('button', { name: /E2E P6/ })).toHaveCount(0);
    await expectFocusOn(page, tiles.getByRole('button').first());
    await resetFocus(page);
    await tabTo(page, tiles.getByRole('button', { name: /E2E P18/ }), 10);
    await page.keyboard.press('Enter');
    await expect(page.getByRole('heading', { name: 'Downloads von E2E P18' })).toBeVisible();
    await expect(
      page
        .getByRole('list', { name: 'Heruntergeladene Titel' })
        .getByRole('link', { name: new RegExp(H264_TITLE) }),
    ).toBeVisible();
    await expectVisibleFocus(page);
    await expectNoFocusTrap(page);
    await context.setOffline(false);
  } finally {
    await removeDeviceDownloads(context.request).catch(() => undefined);
    await context.close();
  }
});

test('the Apple hint shows on an iPhone, and the Wi-Fi option hides (O-7, AN-13)', async ({
  browser,
  baseURL,
  extraHTTPHeaders,
}) => {
  const { context, page } = await freshDevice(browser, {
    baseURL,
    extraHTTPHeaders,
    userAgent: IPHONE,
  });
  try {
    await page.goto('/profiles');
    await page.getByRole('button', { name: /Hauptprofil/ }).click();
    await page.goto('/downloads');
    const panel = page.getByRole('region', { name: 'Speicher' });
    await expect(panel.getByText('Hinweis für iPhone, iPad und Safari')).toBeVisible();
    const rules = page.getByRole('region', { name: 'Automatische Downloads' });
    await expect(
      rules.getByRole('checkbox', { name: 'Automatische Downloads auf diesem Gerät' }),
    ).toBeVisible();
    await expect(rules.getByRole('checkbox', { name: 'Nur im WLAN' })).toHaveCount(0);
    await expect(rules.getByText(/iPhone und iPad verraten dem Browser nicht/)).toBeVisible();
  } finally {
    await context.close();
  }
});
