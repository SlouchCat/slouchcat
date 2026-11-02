import type { APIRequestContext, Page } from '@playwright/test';
import {
  acceptInvitation,
  accountFoldOut,
  accountRow,
  accountToggle,
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
import { arrowBetweenRows, arrowTo } from './playback';

// /admin/accounts (A-2, concept 12 variant C with KP-1) by keyboard (rule 8): the administrator
// and an account in one table with rating, libraries, profiles and channels; each row folds out
// on its e-mail. The own row changes the e-mail (and back) with the current password and holds
// «Meine Profile»; another account's row holds its access, its profile names and appoint, demote
// and remove. The password itself is never changed: that would sign out the shared session of
// every other spec.

const GUEST = { email: 'konten-e2e@slouchcat.test', password: GUEST_PASSWORD };
const CHANGED_EMAIL = 'e2e-owner-neu@slouchcat.test';
/** Tab presses from the top of the page to a control in the table (header, sidebar, tabs). */
const TAB_MAX = 80;

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

/** Tabs from the top of the page to the e-mail of `email`'s row and folds it out with Enter. */
async function openRowByKeyboard(page: Page, email: string) {
  const toggle = accountToggle(page, email);
  await resetFocus(page);
  await tabTo(page, toggle, TAB_MAX);
  await expect(toggle).toHaveAttribute('aria-expanded', 'false');
  await page.keyboard.press('Enter');
  await expect(toggle).toHaveAttribute('aria-expanded', 'true');
  await expectFocusOn(page, toggle);
  return accountFoldOut(page, email);
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
    const strip = page.getByRole('group', { name: 'Zusammenfassung' });
    await expect(strip.getByRole('term')).toHaveText([
      'Konten',
      'Administratoren',
      'Profile',
      'Offene Einladungen',
    ]);

    const own = accountRow(page, OWNER.email);
    await expect(own).toContainText('Administrator');
    await expect(own).toContainText('FSK 18');
    await expect(own).toContainText('Alle Bibliotheken');
    await expect(own).toContainText(`${OWNER_PROFILE} (Hauptprofil)`);
    await expect(own).toContainText('Besitzer');
    const guest = accountRow(page, GUEST.email);
    await expect(guest).toContainText('Konto');
    await expect(guest).toContainText('FSK 12');
    await expect(guest).toContainText('Gast (Hauptprofil)');
    // Plan 5.2: an invited account gets announcements by e-mail unless it switches them off.
    await expect(guest.getByRole('cell').nth(5)).toHaveText('E-Mail');

    // The own fold-out: «Konto ändern» and «Meine Profile»; neither a role action nor a removal,
    // the owner is protected (ADR 0014).
    await accountToggle(page, OWNER.email).click();
    const ownDetail = accountFoldOut(page, OWNER.email);
    await expect(ownDetail.getByRole('region', { name: 'Konto ändern' })).toBeVisible();
    await expect(ownDetail.getByRole('region', { name: 'Meine Profile' })).toBeVisible();
    await expect(ownDetail.getByRole('article', { name: OWNER_PROFILE })).toBeVisible();
    await expect(
      ownDetail.getByRole('button', { name: /ernennen|entziehen|entfernen/ }),
    ).toHaveCount(0);

    // The guest's fold-out: its profile names to read, nothing to manage there.
    await accountToggle(page, GUEST.email).click();
    const names = accountFoldOut(page, GUEST.email).getByRole('region', {
      name: `Profile von ${GUEST.email}`,
    });
    await expect(names).toContainText('Gast (Hauptprofil)');
    await expect(names).toContainText('Profile verwaltet jedes Konto selbst.');
    await expect(names.getByRole('button')).toHaveCount(0);
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
    const detail = await openRowByKeyboard(page, OWNER.email);
    const toggle = accountToggle(page, OWNER.email);

    // ↓ moves to the e-mail of the next row, ↑ comes back; the fold-out is skipped.
    await arrowBetweenRows(page, accountToggle(page, GUEST.email));
    await page.keyboard.press('ArrowUp');
    await expectFocusOn(page, toggle);

    // Tab goes into the fold-out: «Konto ändern» starts with the e-mail.
    const form = detail.getByRole('region', { name: 'Konto ändern' });
    const email = form.getByRole('textbox', { name: 'E-Mail-Adresse' });
    await page.keyboard.press('Tab');
    await expectFocusOn(page, email);
    await expect(email).toHaveValue(OWNER.email);

    // A wrong current password: the form stays and marks that field.
    await page.keyboard.press('ControlOrMeta+a');
    await page.keyboard.type(CHANGED_EMAIL);
    await page.keyboard.press('Tab');
    await expectFocusOn(page, form.getByLabel('Neues Passwort', { exact: true }));
    await page.keyboard.press('Tab');
    const current = form.getByLabel(/^Aktuelles Passwort/);
    await expectFocusOn(page, current);
    await page.keyboard.type('falsches-passwort-0');
    await page.keyboard.press('Enter');
    await expect(form.getByText('Das aktuelle Passwort stimmt nicht.')).toBeVisible();
    await expect(current).toHaveAttribute('aria-invalid', 'true');

    // The right one: saved, the row and the session carry the new address. Typing clears the
    // mark (the server's verdict was about the old value).
    await page.keyboard.press('ControlOrMeta+a');
    await page.keyboard.type(OWNER.password);
    await expect(current).not.toHaveAttribute('aria-invalid', 'true');
    const saved = page.waitForResponse(
      (r) => r.request().method() === 'PUT' && r.url().endsWith('/api/v1/auth/account'),
    );
    await page.keyboard.press('Enter');
    expect((await saved).status()).toBe(200);
    await expect(page.getByText('Dein Konto ist gespeichert.')).toBeVisible();
    const changed = accountRow(page, CHANGED_EMAIL);
    await expect(changed).toContainText('Administrator');
    const session = (await (await owner.get('/api/v1/auth/session')).json()) as SessionData;
    expect(session.account.email).toBe(CHANGED_EMAIL);
    // The draft is spent: the password field is empty and keeps the focus; the row stays open.
    const changedForm = accountFoldOut(page, CHANGED_EMAIL).getByRole('region', {
      name: 'Konto ändern',
    });
    const changedCurrent = changedForm.getByLabel(/^Aktuelles Passwort/);
    await expect(changedCurrent).toHaveValue('');
    await expectFocusOn(page, changedCurrent);
    await expect(accountToggle(page, CHANGED_EMAIL)).toHaveAttribute('aria-expanded', 'true');

    // And back.
    await page.keyboard.press('Shift+Tab');
    await page.keyboard.press('Shift+Tab');
    await expectFocusOn(page, changedForm.getByRole('textbox', { name: 'E-Mail-Adresse' }));
    await page.keyboard.press('ControlOrMeta+a');
    await page.keyboard.type(OWNER.email);
    await page.keyboard.press('Tab');
    await page.keyboard.press('Tab');
    await expectFocusOn(page, changedCurrent);
    await page.keyboard.type(OWNER.password);
    await page.keyboard.press('Enter');
    await expect(accountRow(page, OWNER.email)).toContainText('Administrator');
    await expectFocusOn(page, current);
    await expectVisibleFocus(page);
    await expectNoFocusTrap(page, 120);
  } finally {
    await restoreOwnerEmail(owner);
    await revokeInvitations(owner);
    await anonymous.dispose();
  }
});

/**
 * Rule 8 for the account table that grew in ADR 0014: the access form, the menu «Weitere
 * Aktionen» with the role and the removal (S-6) and their confirmations are reachable and
 * leaveable by keyboard, the focus stays visible and returns to the menu button, and a refusal
 * from the server is readable instead of silent.
 *
 * The test appoints the invited account and takes the role away again. Both directions run against
 * the real server; the `owner` fixture removes the account afterwards, and its cleanup takes a
 * leftover administrator role off first.
 */
test('ADR 0014: appoint and demote by keyboard, with the confirmation and no focus trap', async ({
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
    const detail = await openRowByKeyboard(page, GUEST.email);

    // «Weitere Aktionen» ends the row; the access form is the first stop in the fold-out.
    const more = row.getByRole('button', { name: `Weitere Aktionen für ${GUEST.email}` });
    const access = detail.getByRole('region', { name: `Zugriff von ${GUEST.email}` });
    await page.keyboard.press('Tab');
    await expectFocusOn(page, more);
    await page.keyboard.press('Tab');
    await expectFocusOn(page, access.getByRole('radio', { name: 'FSK 12' }));
    await page.keyboard.press('Shift+Tab');
    await expectFocusOn(page, more);

    // The menu: Enter opens it on its first item, the arrows move, Escape closes onto its button.
    const menu = page.getByRole('menu');
    const promote = menu.getByRole('menuitem', { name: 'Zum Administrator ernennen' });
    const remove = menu.getByRole('menuitem', { name: 'Konto entfernen' });
    await page.keyboard.press('Enter');
    await expect(promote).toBeFocused();
    await expectVisibleFocus(page);
    await page.keyboard.press('ArrowDown');
    await expect(remove).toBeFocused();
    await expect(menu.getByText('Gefahr')).toBeVisible();
    await page.keyboard.press('ArrowUp');
    await expect(promote).toBeFocused();
    await page.keyboard.press('Escape');
    await expect(menu).toHaveCount(0);
    await expectFocusOn(page, more);

    // The role confirmation: Escape leaves it onto the menu button, Enter appoints.
    await page.keyboard.press('Enter');
    await expect(promote).toBeFocused();
    await page.keyboard.press('Enter');
    const confirm = page.getByRole('alertdialog', {
      name: `${GUEST.email} zum Administrator ernennen?`,
    });
    await expect(confirm).toContainText('FSK 18');
    await expectFocusOn(page, confirm.getByRole('button', { name: 'Ernennen' }));
    await page.keyboard.press('Escape');
    await expect(confirm).toBeHidden();
    await expectFocusOn(page, more);

    await page.keyboard.press('Enter');
    await expect(promote).toBeFocused();
    await page.keyboard.press('Enter');
    await expectFocusOn(page, confirm.getByRole('button', { name: 'Ernennen' }));
    const appointed = page.waitForResponse(
      (r) => r.request().method() === 'PUT' && r.url().includes('/role'),
    );
    await page.keyboard.press('Enter');
    expect((await appointed).status()).toBe(204);
    await expect(row).toContainText('Administrator');
    await expect(row).toContainText('FSK 18');
    await expect(page.getByText(`${GUEST.email} ist jetzt Administrator.`)).toBeVisible();
    // An administrator has no access limits any more, so the form is gone (ADR 0014).
    await expect(detail.getByRole('region', { name: /^Zugriff von/ })).toHaveCount(0);
    await expect(detail).toContainText('hat immer FSK 18 und alle Bibliotheken');

    // And back: the demotion signs the account out everywhere. The confirmation gave the focus
    // back to the menu button, whose first item now takes the role away.
    await expectFocusOn(page, more);
    await page.keyboard.press('Enter');
    await expect(menu.getByRole('menuitem', { name: 'Rolle entziehen' })).toBeFocused();
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
    await expect(access).toBeVisible();

    // The owner is refused by the server, and the page offers it nothing to begin with.
    await expect(
      accountRow(page, OWNER.email).getByRole('button', { name: /Weitere Aktionen/ }),
    ).toHaveCount(0);
    const refused = await page.request.put(
      `/api/v1/admin/accounts/${await ownerAccountId(owner)}/role`,
      { headers: CSRF, data: { role: 'user' } },
    );
    expect(refused.status()).toBe(409);
    expect(await refused.text()).toContain('owner_protected');

    // The removal under «Gefahr»: Escape keeps the account, Enter removes it and the h1 takes
    // the focus.
    await expectFocusOn(page, more);
    await page.keyboard.press('Enter');
    await page.keyboard.press('ArrowDown');
    await expect(remove).toBeFocused();
    await page.keyboard.press('Enter');
    const removal = page.getByRole('alertdialog', { name: `Konto ${GUEST.email} entfernen?` });
    await expectFocusOn(page, removal.getByRole('button', { name: 'Konto entfernen' }));
    await page.keyboard.press('Escape');
    await expect(removal).toBeHidden();
    await expectFocusOn(page, more);
    await page.keyboard.press('Enter');
    await page.keyboard.press('ArrowDown');
    await expect(remove).toBeFocused();
    await page.keyboard.press('Enter');
    await expectFocusOn(page, removal.getByRole('button', { name: 'Konto entfernen' }));
    await page.keyboard.press('Enter');
    await expect(row).toHaveCount(0);
    await expect(page.getByText(`Konto ${GUEST.email} entfernt.`)).toBeVisible();
    // The h1 takes the focus (ALL-2); it draws no ring of its own.
    await expect(page.getByRole('heading', { level: 1, name: 'Konten' })).toBeFocused();
    // The confirmation has left the page (its exit animation), so it is not the last Tab stop.
    await expect(removal).toHaveCount(0);
    await expectNoFocusTrap(page, 120);
  } finally {
    await revokeInvitations(owner);
    await anonymous.dispose();
  }
});

test('K-7: the access inline by keyboard; unsaved, leaving asks, Back stays, Verwerfen goes', async ({
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
    const detail = await openRowByKeyboard(page, GUEST.email);
    const access = detail.getByRole('region', { name: `Zugriff von ${GUEST.email}` });

    // The rating by arrows (past the row's menu button), then Tab to the save button and Enter.
    await page.keyboard.press('Tab');
    await expectFocusOn(
      page,
      row.getByRole('button', { name: `Weitere Aktionen für ${GUEST.email}` }),
    );
    await page.keyboard.press('Tab');
    await expectFocusOn(page, access.getByRole('radio', { name: 'FSK 12' }));
    await page.keyboard.press('ArrowLeft');
    await expect(access.getByRole('radio', { name: 'FSK 6' })).toBeChecked();
    const save = access.getByRole('button', { name: 'Zugriff speichern' });
    await tabTo(page, save, 20);
    const patched = page.waitForResponse(
      (r) => r.request().method() === 'PATCH' && r.url().includes('/admin/accounts/'),
    );
    await page.keyboard.press('Enter');
    expect((await patched).status()).toBe(204);
    await expect(page.getByText(`Zugriff von ${GUEST.email} gespeichert.`)).toBeVisible();
    await expect(row).toContainText('FSK 6');
    await expectFocusOn(page, save);

    // An unsaved change survives folding the row away and out again.
    await resetFocus(page);
    await tabTo(page, access.getByRole('radio', { name: 'FSK 6' }), TAB_MAX);
    await page.keyboard.press('ArrowRight');
    await expect(access.getByRole('radio', { name: 'FSK 12' })).toBeChecked();
    const discard = access.getByRole('button', { name: 'Verwerfen' });
    await expect(discard).toBeVisible();
    const toggle = accountToggle(page, GUEST.email);
    await resetFocus(page);
    await tabTo(page, toggle, TAB_MAX);
    await page.keyboard.press('Enter');
    await expect(toggle).toHaveAttribute('aria-expanded', 'false');
    await page.keyboard.press('Enter');
    await expect(access.getByRole('radio', { name: 'FSK 12' })).toBeChecked();

    // Leaving by the sidebar asks; the default stays, Escape stays as well.
    const nav = page.getByRole('navigation', { name: 'Verwaltung' });
    const devices = nav.getByRole('link', { name: /^Geräte/ });
    await resetFocus(page);
    await tabTo(page, nav.getByRole('link', { name: /^Konten/ }), TAB_MAX);
    await arrowTo(page, devices);
    await page.keyboard.press('Enter');
    const question = page.getByRole('alertdialog', {
      name: 'Seite mit ungespeicherten Änderungen verlassen?',
    });
    await expectFocusOn(page, question.getByRole('button', { name: 'Auf der Seite bleiben' }));
    await page.keyboard.press('Enter');
    await expect(question).toHaveCount(0);
    await expect(page).toHaveURL(/\/admin\/accounts$/);
    await expectFocusOn(page, devices);
    await page.keyboard.press('Enter');
    await expect(question).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(question).toHaveCount(0);
    await expect(access.getByRole('radio', { name: 'FSK 12' })).toBeChecked();

    // «Verwerfen» takes the stored value back; then the sidebar leaves without a question, and
    // the browser's Back returns to the page.
    await resetFocus(page);
    await tabTo(page, discard, TAB_MAX);
    await page.keyboard.press('Enter');
    await expect(access.getByRole('radio', { name: 'FSK 6' })).toBeChecked();
    await expect(discard).toHaveCount(0);
    await expectFocusOn(page, save);
    await resetFocus(page);
    await tabTo(page, nav.getByRole('link', { name: /^Konten/ }), TAB_MAX);
    await arrowTo(page, devices);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/admin\/devices$/);
    await page.goBack();
    await expect(page).toHaveURL(/\/admin\/accounts$/);
    await expect(row).toContainText('FSK 6');
    await expectNoFocusTrap(page, 120);
  } finally {
    await revokeInvitations(owner);
    await anonymous.dispose();
  }
});

test('tabs «Konten» and «Einladungen» by arrows; an unknown tab or item leaves the address', async ({
  page,
  owner,
}) => {
  await createInvitation(owner, {
    note: 'Reiter-E2E',
    maxRating: 18,
    allLibraries: true,
    libraryIds: [],
  });
  try {
    await openAccounts(page);
    const accountsTab = page.getByRole('tab', { name: /^Konten/ });
    const invitationsTab = page.getByRole('tab', { name: /^Einladungen/ });
    await expect(invitationsTab).toContainText('1 offen');
    await resetFocus(page);
    await tabTo(page, accountsTab, TAB_MAX);
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, invitationsTab);
    await expect(invitationsTab).toHaveAttribute('aria-selected', 'true');
    await expect(page).toHaveURL(/\?tab=invitations$/);
    const table = page.getByRole('table', { name: 'Bestehende Einladungen' });
    // Earlier runs leave revoked rows of the same note behind; the open one is this run's.
    await expect(
      table.getByRole('row').filter({ hasText: 'Reiter-E2E' }).filter({ hasText: 'Offen' }),
    ).toHaveCount(1);
    // Tab moves on into the panel.
    await page.keyboard.press('Tab');
    await expectFocusOn(page, table.getByRole('button', { name: 'Widerrufen' }));
    await page.keyboard.press('Shift+Tab');
    await expectFocusOn(page, invitationsTab);
    await page.keyboard.press('ArrowLeft');
    await expect(accountsTab).toHaveAttribute('aria-selected', 'true');
    await expect(page).toHaveURL(/\/admin\/accounts$/);

    await page.goto('/admin/accounts?tab=bogus&item=nobody');
    await expect(page.getByRole('table', { name: 'Konten' })).toBeVisible();
    await expect(page).toHaveURL(/\/admin\/accounts$/);
    await expect(accountsTab).toHaveAttribute('aria-selected', 'true');
    await expectNoFocusTrap(page, 120);
  } finally {
    await revokeInvitations(owner);
  }
});

// KP-1: an administrator manages the own profiles in «Konten»; the sidebar's «Profile» opens the
// own account there. `/settings/profiles` stays for every account (profiles.spec).
test('KP-1: «Profile» opens «Meine Profile»; create, PIN, report and delete by keyboard', async ({
  page,
}) => {
  await page.goto('/admin/devices');
  await expect(page.getByRole('heading', { level: 1, name: 'Geräte' })).toBeVisible();
  const nav = page.getByRole('navigation', { name: 'Verwaltung' });
  const entry = nav.getByRole('link', { name: /^Profile/ });
  await resetFocus(page);
  await tabTo(page, nav.getByRole('link', { name: /^Geräte/ }), TAB_MAX);
  await arrowTo(page, entry, 'ArrowUp');
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin\/accounts\?item=me#meine-profile$/);
  await expect(entry).toHaveAttribute('aria-current', 'page');
  await expect(nav.getByRole('link', { name: /^Konten/ })).not.toHaveAttribute('aria-current');
  await expect(accountToggle(page, OWNER.email)).toHaveAttribute('aria-expanded', 'true');
  const mine = accountFoldOut(page, OWNER.email).getByRole('region', { name: 'Meine Profile' });
  await expect(mine).toBeInViewport();
  await expect(mine.getByRole('article', { name: OWNER_PROFILE })).toBeVisible();

  // Create: Enter on «Profil hinzufügen» opens the dialog on its name; Enter submits.
  const add = mine.getByRole('button', { name: 'Profil hinzufügen' });
  await resetFocus(page);
  await tabTo(page, add, TAB_MAX);
  await page.keyboard.press('Enter');
  const form = page.getByRole('dialog', { name: 'Neues Profil' });
  await expectFocusOn(page, form.getByLabel('Name', { exact: true }));
  await page.keyboard.type('Konto-Kind');
  await page.keyboard.press('Tab');
  await expectFocusOn(page, form.getByRole('radio', { name: 'FSK 18' }));
  await page.keyboard.press('ArrowLeft');
  await page.keyboard.press('ArrowLeft');
  await expect(form.getByRole('radio', { name: 'FSK 12' })).toBeChecked();
  await page.keyboard.press('Shift+Tab');
  await page.keyboard.press('Enter');
  await expect(form).toBeHidden();
  const card = mine.getByRole('article', { name: 'Konto-Kind' });
  await expect(card).toContainText('FSK 12');
  // The table follows: the new name is in the own row.
  await expect(accountRow(page, OWNER.email)).toContainText('Konto-Kind');

  // The menu: Enter opens it on its first item, the arrows move, Escape closes onto its button.
  const more = card.getByRole('button', { name: 'Weitere Aktionen für Konto-Kind' });
  await resetFocus(page);
  await tabTo(page, more, TAB_MAX);
  await page.keyboard.press('Enter');
  const menu = page.getByRole('menu');
  const setPin = menu.getByRole('menuitem', { name: 'PIN setzen' });
  await expect(setPin).toBeFocused();
  await page.keyboard.press('ArrowDown');
  await expect(menu.getByRole('menuitem', { name: 'Löschen' })).toBeFocused();
  await expectVisibleFocus(page);
  await page.keyboard.press('Escape');
  await expect(menu).toHaveCount(0);
  await expectFocusOn(page, more);

  // PIN: set from the menu, shown on the card.
  await page.keyboard.press('Enter');
  await expect(setPin).toBeFocused();
  await page.keyboard.press('Enter');
  const pin = page.getByRole('dialog', { name: 'PIN für Konto-Kind' });
  await expectFocusOn(page, pin.getByLabel('Neue PIN'));
  await page.keyboard.type('2468');
  await page.keyboard.press('Enter');
  await expect(pin).toBeHidden();
  await expect(card.getByText('PIN', { exact: true })).toBeVisible();

  // The report opens and closes by Enter; the button says which.
  const report = card.getByRole('button', { name: 'Schauzeit anzeigen' });
  await resetFocus(page);
  await tabTo(page, report, TAB_MAX);
  await page.keyboard.press('Enter');
  await expect(card.getByRole('region', { name: 'Schauzeit von Konto-Kind' })).toBeVisible();
  await expectFocusOn(page, card.getByRole('button', { name: 'Schauzeit ausblenden' }));
  await page.keyboard.press('Enter');
  await expect(card.getByRole('region', { name: 'Schauzeit von Konto-Kind' })).toHaveCount(0);

  // PIN removal and deletion ask first; after the deletion the h1 takes the focus.
  await page.keyboard.press('Tab');
  await expectFocusOn(page, more);
  await page.keyboard.press('Enter');
  await page.keyboard.press('ArrowDown');
  await expect(menu.getByRole('menuitem', { name: 'PIN entfernen' })).toBeFocused();
  await page.keyboard.press('Enter');
  const removePin = page.getByRole('alertdialog', { name: 'PIN von Konto-Kind entfernen?' });
  await expectFocusOn(page, removePin.getByRole('button', { name: 'PIN entfernen' }));
  await page.keyboard.press('Enter');
  await expect(card.getByText('PIN', { exact: true })).toHaveCount(0);
  await expectFocusOn(page, more);
  await page.keyboard.press('Enter');
  await expect(setPin).toBeFocused();
  await page.keyboard.press('ArrowDown');
  await expect(menu.getByRole('menuitem', { name: 'Löschen' })).toBeFocused();
  await page.keyboard.press('Enter');
  const deletion = page.getByRole('alertdialog', { name: 'Profil „Konto-Kind“ löschen?' });
  await expectFocusOn(page, deletion.getByRole('button', { name: 'Profil löschen' }));
  await page.keyboard.press('Enter');
  await expect(card).toHaveCount(0);
  // The h1 takes the focus (ALL-2); it draws no ring of its own.
  await expect(page.getByRole('heading', { level: 1, name: 'Konten' })).toBeFocused();
  // The confirmation has left the page (its exit animation), so it is not the last Tab stop.
  await expect(deletion).toHaveCount(0);
  await expectNoFocusTrap(page, 120);

  // `/settings/profiles` stays for an administrator too: no redirect, the same list.
  await page.goto('/settings/profiles');
  await expect(page).toHaveURL(/\/settings\/profiles$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Profile' })).toBeVisible();
  await expect(page.getByRole('article', { name: OWNER_PROFILE })).toBeVisible();
});

/** The id of the signed-in owner account. */
async function ownerAccountId(owner: APIRequestContext): Promise<number> {
  const session = (await (await owner.get('/api/v1/auth/session')).json()) as SessionData;
  return session.account.id;
}
