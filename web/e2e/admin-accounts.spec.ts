import type { APIRequestContext } from '@playwright/test';
import {
  acceptInvitation,
  accountRow,
  anonymousApi,
  createInvitation,
  openAccounts,
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

// /admin/accounts (A-2) by keyboard (rule 8): the administrator and an account in one table with
// rating, libraries, profiles and channels; the account's row leads to the access page, the own row
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

test('A-2: the administrator and an account in one table, with the owner badge', async ({
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
    await expect(own).toContainText('Administrator');
    await expect(own).toContainText('FSK 18');
    await expect(own).toContainText('Alle Bibliotheken');
    await expect(own).toContainText(`${OWNER_PROFILE} (Hauptprofil)`);
    await expect(own.getByRole('link', { name: 'Profile' })).toHaveAttribute(
      'href',
      '/settings/profiles',
    );
    await expect(own).toContainText('Besitzer');
    const guest = accountRow(page, GUEST.email);
    await expect(guest).toContainText('Konto');
    await expect(guest).toContainText('FSK 12');
    await expect(guest).toContainText('Gast (Hauptprofil)');
    // Plan 5.2: an invited account gets announcements by e-mail unless it switches them off.
    await expect(guest.getByRole('cell').nth(5)).toHaveText('E-Mail');

    // The owner's own row carries neither a role action nor a removal: it is protected (ADR 0014).
    await expect(own.getByRole('button', { name: /ernennen|entziehen|entfernen/ })).toHaveCount(0);
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

    // ↓ moves to the first action of the next row in the same column, ↑ comes back.
    const guestAccess = accountRow(page, GUEST.email).getByRole('button', {
      name: `Zugriff von ${GUEST.email} ändern`,
    });
    await arrowBetweenRows(page, guestAccess);
    await expectFocusOn(page, guestAccess);
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
    await expect(changed).toContainText('Administrator');
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
    await expect(accountRow(page, OWNER.email)).toContainText('Administrator');
    await expectNoFocusTrap(page);
  } finally {
    await restoreOwnerEmail(owner);
    await revokeInvitations(owner);
    await anonymous.dispose();
  }
});

/**
 * Rule 8 for the account table that grew in ADR 0014: the access dialog, the role dialog and the
 * removal are reachable and leaveable by keyboard, the focus stays visible and returns to the
 * button it came from, and a refusal from the server is readable instead of silent.
 *
 * The test appoints the invited account and takes the role away again. Both directions run against
 * the real server; the `owner` fixture removes the account afterwards, and its cleanup takes a
 * leftover administrator role off first.
 */
test('ADR 0014: appoint and demote by keyboard, with the dialogs and no focus trap', async ({
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
    const row = accountRow(page, GUEST.email);

    // The access dialog: Enter opens it, Escape closes it and gives the focus back.
    const access = row.getByRole('button', { name: `Zugriff von ${GUEST.email} ändern` });
    await resetFocus(page);
    await tabTo(page, access, 80);
    await page.keyboard.press('Enter');
    const accessDialog = page.getByRole('dialog', { name: `Zugriff von ${GUEST.email}` });
    await expect(accessDialog).toBeVisible();
    await expectFocusOn(page, accessDialog.getByRole('radio', { name: 'FSK 12' }));
    await page.keyboard.press('Escape');
    await expect(accessDialog).toBeHidden();
    await expectFocusOn(page, access);

    // The role dialog: Tab reaches it, Escape leaves it, Enter appoints.
    const promote = row.getByRole('button', { name: `${GUEST.email} zum Administrator ernennen` });
    await page.keyboard.press('Tab');
    await expectFocusOn(page, promote);
    await page.keyboard.press('Enter');
    const confirm = page.getByRole('alertdialog', {
      name: `${GUEST.email} zum Administrator ernennen?`,
    });
    await expect(confirm).toContainText('FSK 18');
    await expectFocusOn(page, confirm.getByRole('button', { name: 'Ernennen' }));
    await page.keyboard.press('Escape');
    await expect(confirm).toBeHidden();
    await expectFocusOn(page, promote);

    await page.keyboard.press('Enter');
    const appointed = page.waitForResponse(
      (r) => r.request().method() === 'PUT' && r.url().includes('/role'),
    );
    await page.keyboard.press('Enter');
    expect((await appointed).status()).toBe(204);
    await expect(row).toContainText('Administrator');
    await expect(row).toContainText('FSK 18');
    // An administrator has no access limits any more, so the button is gone (ADR 0014).
    await expect(row.getByRole('button', { name: /Zugriff von/ })).toHaveCount(0);

    // And back: the demotion signs the account out everywhere. The dialog gave the focus back to
    // the button it came from, and that button now carries the other label - no second Tab walk.
    const demote = row.getByRole('button', {
      name: `${GUEST.email} die Administratorrolle entziehen`,
    });
    await expectFocusOn(page, demote);
    await page.keyboard.press('Enter');
    const back = page.getByRole('alertdialog', {
      name: `${GUEST.email} die Administratorrolle entziehen?`,
    });
    await expect(back).toContainText('abgemeldet');
    const demoted = page.waitForResponse(
      (r) => r.request().method() === 'PUT' && r.url().includes('/role'),
    );
    await page.keyboard.press('Enter');
    expect((await demoted).status()).toBe(204);
    await expect(row).toContainText('Konto');

    // The owner is refused by the server, and the page says so instead of swallowing it.
    const ownRow = accountRow(page, OWNER.email);
    await expect(ownRow.getByRole('button', { name: /ernennen|entziehen|entfernen/ })).toHaveCount(
      0,
    );
    const refused = await page.request.put(
      `/api/v1/admin/accounts/${await ownerAccountId(owner)}/role`,
      { headers: CSRF, data: { role: 'user' } },
    );
    expect(refused.status()).toBe(409);
    expect(await refused.text()).toContain('owner_protected');

    await expectVisibleFocus(page);
    await expectNoFocusTrap(page);
  } finally {
    await revokeInvitations(owner);
    await anonymous.dispose();
  }
});

/** The id of the signed-in owner account. */
async function ownerAccountId(owner: APIRequestContext): Promise<number> {
  const session = (await (await owner.get('/api/v1/auth/session')).json()) as SessionData;
  return session.account.id;
}
