import type { APIRequestContext } from '@playwright/test';
import { anonymousApi } from './accounts';
import { CSRF } from './catalog';
import { OWNER } from './env';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, resetFocus, tabTo } from './keyboard';

// «Meine Geräte» (concept 27 4.5) by keyboard (rule 8): the sidebar link with Enter, the list with
// «Dieses Gerät», Tab to «Abmelden» of a phone, Escape and Enter in the confirmation, the status
// line, a visible ring on every stop, no focus trap and Back to the page before. The phone is the
// spec's own: an app sign-in of the owner with a name of this run, signed out at the end even when
// the test fails, so a second run on the same server starts the same way. The shared browser
// session of the other specs is never signed out.

interface Device {
  id: string;
  name?: string;
}

async function devicesOf(api: APIRequestContext): Promise<Device[]> {
  const response = await api.get('/api/v1/account/devices');
  expect(response.status(), await response.text()).toBe(200);
  return (await response.json()) as Device[];
}

test('«Meine Geräte» by keyboard: list, sign out a phone with confirmation, Back @keyboard', async ({
  page,
  playwright,
  baseURL,
}) => {
  test.setTimeout(60_000);
  const name = `E2E Telefon ${Date.now().toString(36)}`;
  const app = await anonymousApi(playwright, baseURL!, CSRF);
  const login = await app.post('/api/v1/auth/app-login', {
    data: { email: OWNER.email, password: OWNER.password, deviceName: name, kind: 'phone' },
  });
  expect(login.status(), await login.text()).toBe(200);
  const { deviceId } = (await login.json()) as { deviceId: string };
  try {
    await page.goto('/');
    const link = page.getByRole('link', { name: 'Meine Geräte' });
    await resetFocus(page);
    await tabTo(page, link, 200);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/settings\/devices$/);
    await expect(page.getByRole('heading', { level: 1, name: 'Meine Geräte' })).toBeVisible();

    const list = page.getByRole('list', { name: 'Geräte dieses Kontos' });
    await expect(list.getByText('Dieses Gerät')).toBeVisible();
    const phone = list.getByRole('listitem').filter({ hasText: name });
    await expect(phone.getByText(/^Handy-App, angemeldet am/)).toBeVisible();

    // Tab reaches «Abmelden» of the phone; Escape closes the confirmation and sends nothing.
    const signOut = phone.getByRole('button', { name: `${name} abmelden` });
    await resetFocus(page);
    await tabTo(page, signOut, 200);
    await page.keyboard.press('Enter');
    let dialog = page.getByRole('alertdialog', { name: `„${name}“ abmelden?` });
    await expect(dialog).toBeVisible();
    await expectFocusOn(page, dialog.getByRole('button', { name: 'Gerät abmelden' }));
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
    await expectFocusOn(page, signOut);
    expect((await devicesOf(page.request)).some((device) => device.id === deviceId)).toBe(true);

    // Enter twice: open, confirm. The status names the device; the row is gone, the h1 has the
    // focus, the rest of the page is still reachable.
    await page.keyboard.press('Enter');
    dialog = page.getByRole('alertdialog');
    await expectFocusOn(page, dialog.getByRole('button', { name: 'Gerät abmelden' }));
    const deleted = page.waitForResponse(
      (response) =>
        response.request().method() === 'DELETE' &&
        response.url().endsWith(`/account/devices/${deviceId}`),
    );
    await page.keyboard.press('Enter');
    expect((await deleted).status()).toBe(204);
    await expect(page.getByText(`„${name}“ abgemeldet.`)).toBeVisible();
    await expect(phone).toHaveCount(0);
    // The h1 takes the focus programmatically (no ring, like the other pages after a delete).
    await expect(page.getByRole('heading', { level: 1, name: 'Meine Geräte' })).toBeFocused();
    await expectNoFocusTrap(page, 200);

    // The phone's token is no longer valid.
    const after = await app.get('/api/v1/auth/session', {
      headers: { Authorization: `Bearer ${((await login.json()) as { token: string }).token}` },
    });
    expect(after.status()).toBe(401);

    await page.goBack();
    await expect(page).toHaveURL(/\/$/);
  } finally {
    if ((await devicesOf(page.request)).some((device) => device.id === deviceId)) {
      const response = await page.request.delete(`/api/v1/account/devices/${deviceId}`, {
        headers: CSRF,
      });
      expect(response.status()).toBe(204);
    }
    await app.dispose();
  }
});
