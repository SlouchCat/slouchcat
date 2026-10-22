import type { APIRequestContext, Page } from '@playwright/test';
import {
  acceptInvitation,
  anonymousApi,
  createInvitation,
  revokeInvitation,
  type InvitationData,
  type SessionData,
} from './accounts';
import { CSRF } from './catalog';
import { GUEST_PASSWORD, OWNER, OWNER_PROFILE } from './env';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';
import { arrowBetweenRows } from './playback';

// /admin/accounts (A-2) by keyboard (rule 8): the main account and a guest in one table with
// rating, libraries, profiles and channels; the guest row leads to the access page, the own row
// changes the e-mail (and back) with the current password. The password itself is never changed:
// that would sign out the shared session of every other spec.

const GUEST = { email: 'konten-e2e@slouchcat.test', password: GUEST_PASSWORD };
const CHANGED_EMAIL = 'e2e-owner-neu@slouchcat.test';

/** A guest account from an invitation; the `owner` fixture removes it after the test. */
async function inviteGuest(owner: APIRequestContext, anonymous: APIRequestContext) {
  const invitation = await createInvitation(owner, {
    note: 'Konten-E2E',
    maxRating: 12,
    allLibraries: true,
    libraryIds: [],
  });
  await acceptInvitation(anonymous, invitation.token, {
    email: GUEST.email,
    password: GUEST.password,
    profileName: 'Gast',
    language: 'de',
  });
}

async function revokeInvitations(owner: APIRequestContext) {
  const all = await owner.get('/api/v1/admin/invitations');
  for (const item of (await all.json()) as InvitationData[]) {
    if (item.state === 'open') await revokeInvitation(owner, item.id);
  }
}

/** Whatever a failed test left behind: the main account ends with its own e-mail again. */
async function restoreOwnerEmail(owner: APIRequestContext) {
  const session = (await (await owner.get('/api/v1/auth/session')).json()) as SessionData;
  if (session.account.email === OWNER.email) return;
  const response = await owner.put('/api/v1/auth/account', {
    headers: CSRF,
    data: { email: OWNER.email, currentPassword: OWNER.password },
  });
  expect(response.status(), await response.text()).toBe(200);
}

async function openAccounts(page: Page) {
  await page.goto('/admin/accounts');
  await expect(page.getByRole('heading', { level: 1, name: 'Konten' })).toBeVisible();
  await expect(page.getByRole('table', { name: 'Konten' })).toBeVisible();
}

const accountRow = (page: Page, email: string) =>
  page
    .getByRole('table', { name: 'Konten' })
    .getByRole('row')
    .filter({ has: page.getByRole('rowheader', { name: email, exact: true }) });

test('A-2: the main account and a guest in one table; the guest row leads to access', async ({
  page,
  owner,
  playwright,
  baseURL,
  extraHTTPHeaders,
}) => {
  const anonymous = await anonymousApi(playwright, baseURL!, extraHTTPHeaders);
  try {
    await inviteGuest(owner, anonymous);
    await openAccounts(page);

    const own = accountRow(page, OWNER.email);
    await expect(own).toContainText('Hauptkonto');
    await expect(own).toContainText('FSK 18');
    await expect(own).toContainText('Alle Bibliotheken');
    await expect(own).toContainText(`${OWNER_PROFILE} (Hauptprofil)`);
    await expect(own.getByRole('link', { name: 'Profile' })).toHaveAttribute(
      'href',
      '/settings/profiles',
    );
    const guest = accountRow(page, GUEST.email);
    await expect(guest).toContainText('Gast');
    await expect(guest).toContainText('FSK 12');
    await expect(guest).toContainText('Gast (Hauptprofil)');
    // Plan 5.2: a guest gets announcements by e-mail unless they switch it off.
    await expect(guest.getByRole('cell').nth(5)).toHaveText('E-Mail');

    // Enter on the guest's link opens the access page; Back returns.
    const access = guest.getByRole('link', { name: `Zugriff von ${GUEST.email} verwalten` });
    await expect(access).toHaveAttribute('href', '/admin/access');
    await resetFocus(page);
    await tabTo(page, access, 60);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/admin\/access$/);
    await expect(page.getByRole('heading', { level: 1, name: 'Zugriff' })).toBeVisible();
    await page.goBack();
    await expect(page).toHaveURL(/\/admin\/accounts$/);
    await expect(accountRow(page, GUEST.email)).toBeVisible();
  } finally {
    await revokeInvitations(owner);
    await anonymous.dispose();
  }
});

test('A-2: change the own e-mail and back by keyboard; a wrong password stays at its field', async ({
  page,
  owner,
  playwright,
  baseURL,
  extraHTTPHeaders,
}) => {
  const anonymous = await anonymousApi(playwright, baseURL!, extraHTTPHeaders);
  try {
    await inviteGuest(owner, anonymous);
    await openAccounts(page);
    await resetFocus(page);
    const change = accountRow(page, OWNER.email).getByRole('button', { name: 'Konto ändern' });
    await tabTo(page, change, 60);

    // ↓ moves to the guest's link in the same column, ↑ comes back.
    const guestLink = accountRow(page, GUEST.email).getByRole('link', {
      name: `Zugriff von ${GUEST.email} verwalten`,
    });
    await arrowBetweenRows(page, guestLink);
    await expectFocusOn(page, guestLink);
    await page.keyboard.press('ArrowUp');
    await expectFocusOn(page, change);

    // Escape closes the dialog and returns the focus.
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', { name: 'Konto ändern' });
    const email = dialog.getByRole('textbox', { name: 'E-Mail-Adresse' });
    await expectFocusOn(page, email);
    await expect(email).toHaveValue(OWNER.email);
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
    await expectFocusOn(page, change);

    // A wrong current password: the dialog stays and marks that field.
    await page.keyboard.press('Enter');
    await expectFocusOn(page, email);
    await page.keyboard.press('ControlOrMeta+a');
    await page.keyboard.type(CHANGED_EMAIL);
    await page.keyboard.press('Tab');
    await expectFocusOn(page, dialog.getByLabel('Neues Passwort', { exact: true }));
    await page.keyboard.press('Tab');
    const current = dialog.getByLabel(/^Aktuelles Passwort/);
    await expectFocusOn(page, current);
    await page.keyboard.type('falsches-passwort-0');
    await page.keyboard.press('Enter');
    await expect(dialog.getByText('Das aktuelle Passwort stimmt nicht.')).toBeVisible();
    await expect(current).toHaveAttribute('aria-invalid', 'true');

    // The right one: saved, the dialog closes, the row and the session carry the new address.
    // Typing clears the mark (the server's verdict was about the old value).
    await page.keyboard.press('ControlOrMeta+a');
    await page.keyboard.type(OWNER.password);
    await expect(current).not.toHaveAttribute('aria-invalid', 'true');
    const saved = page.waitForResponse(
      (r) => r.request().method() === 'PUT' && r.url().endsWith('/api/v1/auth/account'),
    );
    await page.keyboard.press('Enter');
    expect((await saved).status()).toBe(200);
    await expect(dialog).toBeHidden();
    await expect(page.getByText('Dein Konto ist gespeichert.')).toBeVisible();
    const changed = accountRow(page, CHANGED_EMAIL);
    await expect(changed).toContainText('Hauptkonto');
    const session = (await (await owner.get('/api/v1/auth/session')).json()) as SessionData;
    expect(session.account.email).toBe(CHANGED_EMAIL);
    const again = changed.getByRole('button', { name: 'Konto ändern' });
    await expectFocusOn(page, again);
    await expectVisibleFocus(page);

    // And back.
    await page.keyboard.press('Enter');
    await expectFocusOn(page, email);
    await page.keyboard.press('ControlOrMeta+a');
    await page.keyboard.type(OWNER.email);
    await page.keyboard.press('Tab');
    await page.keyboard.press('Tab');
    await expectFocusOn(page, current);
    await page.keyboard.type(OWNER.password);
    await page.keyboard.press('Enter');
    await expect(dialog).toBeHidden();
    await expect(accountRow(page, OWNER.email)).toContainText('Hauptkonto');
    await expectNoFocusTrap(page);
  } finally {
    await restoreOwnerEmail(owner);
    await revokeInvitations(owner);
    await anonymous.dispose();
  }
});
