import type { Page } from '@playwright/test';
import {
  acceptInvitation,
  anonymousApi,
  createInvitation,
  createProfile,
  loginOwner,
} from './accounts';
import { OWNER } from './env';
import { expect, test } from './fixtures';
import { expectFocusOn, expectVisibleFocus } from './keyboard';
import { channel, deleteAnnouncements, mailsTo, publishAnnouncement } from './mocks';

// Announcements (A-7, plan 5.2): the main user publishes one, every profile sees it at its next
// start in a dialog until it acknowledges it, and the guest accounts with the default e-mail
// channel get it by mail. The dialog by keyboard (rule 8). Each test logs in with a session of
// its own (a profile switch changes the session; the shared one stays on the primary profile),
// and every announcement is deleted afterwards: an open one would sit over every later spec.
test.use({ storageState: { cookies: [], origins: [] } });

test.afterEach(async ({ owner }) => {
  await deleteAnnouncements(owner);
});

const TITLE = 'Wartung am Samstag';
const BODY =
  'Von 8 bis 10 Uhr ist Slouchcat nicht erreichbar.\nDownloads spielen weiter.\n\nDanach gibt es die neuen Sammlungen.';

const dialog = (page: Page) => page.getByRole('dialog', { name: TITLE });

/** Logs in and picks the profile `name` on the picker by keyboard; the start page follows. */
async function startAs(page: Page, name: string): Promise<void> {
  await loginOwner(page.request);
  await page.goto('/');
  await expect(page).toHaveURL(/\/profiles$/);
  const tile = page
    .getByRole('list', { name: 'Profile' })
    .getByRole('button', { name: new RegExp(`^${name}`) });
  await tile.focus();
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL((url) => url.pathname === '/');
}

/** Reloads and waits for the profile's open announcements (GET /announcements). */
async function restart(page: Page): Promise<void> {
  const answered = page.waitForResponse(
    (response) => new URL(response.url()).pathname === '/api/v1/announcements',
  );
  await page.reload();
  expect((await answered).status()).toBe(200);
}

test('another profile sees it at its next start; after "Verstanden" not again', async ({
  page,
  owner,
}) => {
  await createProfile(owner, { name: 'Zweit', accent: 'sky', language: 'de' });
  await publishAnnouncement(owner, TITLE, BODY);
  await startAs(page, 'Zweit');

  await expect(dialog(page)).toBeVisible();
  await expect(dialog(page)).toContainText('Ankündigung');
  await expect(dialog(page)).toContainText('Von 8 bis 10 Uhr ist Slouchcat nicht erreichbar.');
  await expect(dialog(page)).toContainText('Danach gibt es die neuen Sammlungen.');
  const ok = dialog(page).getByRole('button', { name: 'Verstanden' });
  const later = dialog(page).getByRole('button', { name: 'Später' });

  // The focus starts on "Verstanden" and stays inside: Tab cycles the two buttons, the arrows
  // of the start page behind do not reach out of the dialog.
  await expectFocusOn(page, ok);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, later);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, ok);
  for (const key of ['ArrowDown', 'ArrowRight', 'ArrowUp', 'ArrowLeft']) {
    await page.keyboard.press(key);
    await expect(dialog(page).locator(':focus')).toHaveCount(1);
  }
  await expectVisibleFocus(page);

  // Escape (back) closes it for this visit without acknowledging; the next start shows it again.
  await page.keyboard.press('Escape');
  await expect(dialog(page)).toHaveCount(0);
  await expect(page).toHaveURL((url) => url.pathname === '/');
  // Another page of the same visit (in the app, not a new load): still closed.
  await page
    .getByRole('navigation', { name: 'Hauptnavigation' })
    .getByRole('link', { name: 'Merkliste' })
    .click();
  await expect(page).toHaveURL(/\/watchlist$/);
  await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
  await expect(dialog(page)).toHaveCount(0);
  await restart(page);
  await expect(dialog(page)).toBeVisible();

  // Enter on "Verstanden" acknowledges it: gone, also after the next start.
  await expectFocusOn(page, ok);
  const acknowledged = page.waitForResponse((response) =>
    /\/api\/v1\/announcements\/\d+\/ack$/.test(response.url()),
  );
  await page.keyboard.press('Enter');
  expect((await acknowledged).status()).toBe(204);
  await expect(dialog(page)).toHaveCount(0);
  await restart(page);
  await expect(page.getByRole('link', { name: 'Profil wechseln, aktuell Zweit' })).toBeVisible();
  await expect(dialog(page)).toHaveCount(0);

  // Acknowledged per profile: the primary profile still has it open.
  await page.getByRole('link', { name: 'Profil wechseln, aktuell Zweit' }).click();
  await expect(page).toHaveURL(/\/profiles$/);
  await page
    .getByRole('list', { name: 'Profile' })
    .getByRole('button', { name: /^e2e-owner/ })
    .click();
  await expect(dialog(page)).toBeVisible();
});

test('a guest account gets it by mail on its default e-mail channel (plan 5.2)', async ({
  owner,
  playwright,
  baseURL,
  extraHTTPHeaders,
}) => {
  const guest = { email: 'ankuendigung-gast@slouchcat.test', password: 'ankuendigung-pass-0123' };
  const invitation = await createInvitation(owner);
  const guestApi = await anonymousApi(playwright, baseURL!, extraHTTPHeaders);
  try {
    await acceptInvitation(guestApi, invitation.token, {
      ...guest,
      profileName: 'Gast',
      language: 'de',
    });
    // A new guest account starts with e-mail on for announcements; the main account's is off.
    const mail = await channel(guestApi, 'email');
    expect(mail.enabled).toBe(true);
    expect(mail.events).toContain('announcements');
    expect((await channel(owner, 'email')).enabled).toBe(false);

    await publishAnnouncement(owner, TITLE, BODY);
    await expect
      .poll(async () => (await mailsTo(guest.email)).map((sent) => sent.subject), {
        timeout: 60_000,
      })
      .toContainEqual(expect.stringContaining(TITLE));
    const [sent] = await mailsTo(guest.email);
    expect(sent?.text).toContain('Danach gibt es die neuen Sammlungen.');
    expect(await mailsTo(OWNER.email)).toHaveLength(0);
  } finally {
    await guestApi.dispose();
  }
});
