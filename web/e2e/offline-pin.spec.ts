import { createProfile } from './accounts';
import { CSRF } from './catalog';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, expectVisibleFocus } from './keyboard';
import {
  createDownload,
  freshDevice,
  openOfflineLibrary,
  prepareOffline,
  removeDeviceDownloads,
  waitForComplete,
} from './offline';
import { H264_TITLE, itemIdByTitle } from './playback';

// AN-19: a protected profile opens offline with its PIN, checked on the device. The verifier of
// the normal PIN comes from an online PIN entry on this device after the sync delivered the salt
// (the downloads page asks for it once); a separate offline PIN replaces it; a profile without a
// verifier stays locked offline. After the third wrong PIN the device pauses 30 s.

test('offline PIN: the verifier of the normal PIN, the separate offline PIN, the brake', async ({
  browser,
  baseURL,
  extraHTTPHeaders,
  owner,
}) => {
  test.setTimeout(300_000);
  const pinned = await createProfile(owner, { name: 'E2E PIN', pin: '4711' });
  const separate = await createProfile(owner, { name: 'E2E Offline-PIN', pin: '1357' });
  await createProfile(owner, { name: 'E2E Gesperrt', pin: '9999' });
  const offlinePin = await owner.put(`/api/v1/profiles/${separate.id}/offline-pin`, {
    headers: CSRF,
    data: { pin: '246813' },
  });
  expect(offlinePin.status(), await offlinePin.text()).toBe(204);

  const { context, page } = await freshDevice(browser, { baseURL, extraHTTPHeaders });
  try {
    // The PIN online, in the picker.
    await page.goto('/profiles');
    await page.getByRole('button', { name: /E2E PIN/ }).click();
    const online = page.getByRole('dialog', { name: 'PIN für E2E PIN' });
    await online.getByLabel('PIN', { exact: true }).fill('4711');
    await online.getByRole('button', { name: 'Öffnen' }).click();
    await expect(page).toHaveURL(/\/$/);

    await createDownload(page, await itemIdByTitle(owner, H264_TITLE));
    const [download] = await waitForComplete(page);
    expect(download.allowedProfileIds).toEqual(expect.arrayContaining([pinned.id, separate.id]));

    // The salt came with the sync after that entry: the downloads page asks for the PIN once.
    await page.goto('/downloads');
    const hint = page.getByRole('status').filter({ hasText: 'PIN für die Offline-Nutzung' });
    await expect(hint).toBeVisible();
    await hint.getByRole('button', { name: 'PIN eingeben' }).focus();
    await page.keyboard.press('Enter');
    const again = page.getByRole('dialog', { name: 'PIN für E2E PIN' });
    await expectFocusOn(page, again.getByLabel('PIN', { exact: true }));
    await page.keyboard.type('4711');
    await page.keyboard.press('Enter');
    await expect(again).toHaveCount(0);
    await expect(hint).toHaveCount(0, { timeout: 15_000 });

    // Offline.
    await prepareOffline(page);
    await context.setOffline(true);
    await openOfflineLibrary(page);
    const tiles = page.getByRole('list', { name: 'Profile' });
    await expect(tiles.getByRole('button')).toHaveCount(4);
    // The arrow keys move between the tiles (rule 8, W11).
    const allTiles = tiles.getByRole('button');
    await expectFocusOn(page, allTiles.first());
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, allTiles.nth(1));
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, allTiles.nth(2));
    await page.keyboard.press('ArrowLeft');
    await expectFocusOn(page, allTiles.nth(1));

    // A profile whose PIN was never entered on this device is locked offline.
    await tiles.getByRole('button', { name: /E2E Gesperrt/ }).focus();
    await page.keyboard.press('Enter');
    const locked = page.getByRole('dialog', { name: 'PIN für E2E Gesperrt' });
    await expect(locked.getByText('Offline gesperrt')).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(locked).toHaveCount(0);

    // The normal PIN through the device's verifier; three wrong ones pause 30 s.
    const tile = tiles.getByRole('button', { name: /E2E PIN/ });
    await tile.focus();
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', { name: 'PIN für E2E PIN' });
    const field = dialog.getByLabel('PIN', { exact: true });
    await expectFocusOn(page, field);
    for (let attempt = 1; attempt <= 3; attempt += 1) {
      await field.fill('0000');
      await page.keyboard.press('Enter');
      if (attempt < 3) {
        await expect(
          dialog.getByText('Die PIN stimmt nicht. Versuche es noch einmal.'),
        ).toBeVisible();
      }
    }
    await expect(dialog.getByText(/Zu viele Fehlversuche\. Neuer Versuch in/)).toBeVisible();
    await expect(dialog.getByRole('button', { name: 'Öffnen' })).toBeDisabled();
    // Escape closes the dialog; the focus returns to the tile; the pause stays on the device.
    await page.keyboard.press('Escape');
    await expectFocusOn(page, tile);
    await page.keyboard.press('Enter');
    await expect(dialog.getByText(/Zu viele Fehlversuche/)).toBeVisible();
    await expect(dialog.getByRole('button', { name: 'Öffnen' })).toBeEnabled({ timeout: 35_000 });
    await field.fill('4711');
    await page.keyboard.press('Enter');
    await expect(page.getByRole('heading', { name: 'Downloads von E2E PIN' })).toBeVisible();
    await expectVisibleFocus(page);
    await expectNoFocusTrap(page);

    // The separate offline PIN replaces the normal one offline.
    await page.getByRole('button', { name: 'Profil wechseln' }).focus();
    await page.keyboard.press('Enter');
    await tiles.getByRole('button', { name: /E2E Offline-PIN/ }).focus();
    await page.keyboard.press('Enter');
    const separateDialog = page.getByRole('dialog', { name: 'PIN für E2E Offline-PIN' });
    await expect(
      separateDialog.getByText('Dieses Profil hat eine eigene Offline-PIN.'),
    ).toBeVisible();
    const offlineField = separateDialog.getByLabel('Offline-PIN', { exact: true });
    await offlineField.fill('135700');
    await page.keyboard.press('Enter');
    await expect(
      separateDialog.getByText('Die PIN stimmt nicht. Versuche es noch einmal.'),
    ).toBeVisible();
    await offlineField.fill('246813');
    await page.keyboard.press('Enter');
    await expect(
      page.getByRole('heading', { name: 'Downloads von E2E Offline-PIN' }),
    ).toBeVisible();
    await context.setOffline(false);
  } finally {
    await removeDeviceDownloads(context.request).catch(() => undefined);
    await context.close();
  }
});
