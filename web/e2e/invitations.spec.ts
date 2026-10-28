import type { Page } from '@playwright/test';
import {
  acceptInvitation,
  accountRow,
  anonymousApi,
  createInvitation,
  ensureEmptyLibrary,
  openAccounts,
  revokeInvitation,
  type InvitationData,
} from './accounts';
import { choose } from './controls';
import { GUEST_PASSWORD } from './env';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, tabTo } from './keyboard';

// Invitations (K-6) and the account they create (K-7). Since ADR 0014 both live on one page: the
// "Konto einladen" dialog is the only way an account comes into being, the links it produced are
// listed below the account list, and the account itself is a row of that list. The invited side
// runs in a context of its own, as another person in another browser would.

const GUEST = { email: 'gast-e2e@slouchcat.test', password: GUEST_PASSWORD };

/** The invitations below the account list. */
const invitations = (page: Page) => page.getByRole('region', { name: 'Einladungen' });

const row = (page: Page, text: string) =>
  invitations(page).getByRole('listitem').filter({ hasText: text }).first();

/** Opens the account list and its invitation dialog, the one door for a new account. */
async function openInvite(page: Page) {
  await openAccounts(page);
  await page.getByRole('button', { name: 'Konto einladen' }).click();
  const dialog = page.getByRole('dialog', { name: 'Konto einladen' });
  await expect(dialog).toBeVisible();
  return dialog;
}

test('K-6: the administration invites, the invited person joins, the used link answers 410', async ({
  page,
  browser,
  baseURL,
  extraHTTPHeaders,
  owner,
}) => {
  const dialog = await openInvite(page);
  await dialog.getByLabel(/Für wen ist der Link/).fill('E2E Gast');
  await dialog.getByRole('button', { name: 'Einladung erstellen' }).click();

  // The full link is shown once, with a hint and a copy button.
  const link = dialog.getByRole('textbox', { name: 'Einladungslink' });
  await expect(link).toHaveValue(new RegExp(`^${baseURL}/invite/[A-Za-z0-9_-]{20,}$`));
  await expect(
    dialog.getByText('Kopiere den Link jetzt: Er wird nur dieses eine Mal angezeigt.'),
  ).toBeVisible();
  const inviteUrl = await link.inputValue();
  await dialog.getByRole('button', { name: 'Link kopieren' }).click();
  await expect(
    dialog.getByRole('status').filter({ hasText: /Link kopiert|Kopieren nicht möglich/ }),
  ).toBeVisible();
  await dialog.getByRole('button', { name: 'Fertig' }).click();
  await expect(dialog).toBeHidden();
  await expect(row(page, 'E2E Gast')).toContainText('Offen');

  // The guest, in a browser of their own.
  const guestContext = await browser.newContext({ baseURL, locale: 'de-CH', extraHTTPHeaders });
  try {
    const guest = await guestContext.newPage();
    await guest.goto(inviteUrl);
    await expect(
      guest.getByRole('heading', { level: 1, name: 'Einladung zu Slouchcat' }),
    ).toBeVisible();
    await expect(guest.getByRole('region', { name: 'Für: E2E Gast' })).toContainText('FSK 18');
    await guest.getByLabel('E-Mail-Adresse').fill(GUEST.email);
    await guest.getByLabel(/^Passwort/).fill(GUEST.password);
    await guest.getByLabel(/^Profilname/).fill('Gast');
    await guest.getByRole('button', { name: 'Konto anlegen' }).click();

    // The start page, without any administration.
    await expect(guest).toHaveURL(/\/$/);
    const nav = guest.getByRole('navigation', { name: 'Hauptnavigation' });
    await expect(nav.getByRole('link', { name: 'Start' })).toBeVisible();
    await expect(guest.getByRole('heading', { level: 2, name: 'Neu hinzugefügt' })).toBeVisible();
    await expect(nav.getByRole('link', { name: 'Verwaltung' })).toHaveCount(0);
    expect((await guestContext.request.get('/api/v1/admin/libraries')).status()).toBe(403);
    expect((await guestContext.request.get('/api/v1/admin/accounts')).status()).toBe(403);
    // The old address of the "Zugriff" page leads to the account list, which refuses them.
    await guest.goto('/admin/access');
    await expect(guest).toHaveURL(/\/admin\/accounts$/);
    await expect(guest.getByText('Nur für Administratoren')).toBeVisible();

    // The used link answers 410 for anyone.
    const stranger = await browser.newContext({ baseURL, locale: 'de-CH', extraHTTPHeaders });
    try {
      const strangerPage = await stranger.newPage();
      await strangerPage.goto(inviteUrl);
      await expect(
        strangerPage.getByRole('heading', { level: 1, name: 'Link nicht mehr gültig' }),
      ).toBeVisible();
      await expect(strangerPage.getByLabel('E-Mail-Adresse')).toHaveCount(0);
    } finally {
      await stranger.close();
    }

    // One page: the invitation shows its state, the account it made is a row above it.
    await openAccounts(page);
    await expect(row(page, 'E2E Gast')).toContainText('Benutzt');
    await expect(row(page, 'E2E Gast')).toContainText(`Angenommen von ${GUEST.email}`);
    const entry = accountRow(page, GUEST.email);
    await expect(entry).toContainText('Konto');
    await expect(entry).toContainText('FSK 18');
    await entry.getByRole('button', { name: `Zugriff von ${GUEST.email} ändern` }).click();
    const edit = page.getByRole('dialog', { name: `Zugriff von ${GUEST.email}` });
    await choose(edit, 'radio', 'FSK 12');
    await edit.getByRole('button', { name: 'Speichern' }).click();
    await expect(edit).toBeHidden();
    await expect(entry).toContainText('FSK 12');

    // K-7: removing the account ends its session at once.
    expect((await guestContext.request.get('/api/v1/auth/session')).status()).toBe(200);
    await entry.getByRole('button', { name: `${GUEST.email} entfernen` }).click();
    const confirm = page.getByRole('alertdialog', { name: `Konto ${GUEST.email} entfernen?` });
    await expect(confirm).toContainText('Laufende Sitzungen und Wiedergaben enden sofort');
    await confirm.getByRole('button', { name: 'Konto entfernen' }).click();
    await expect(entry).toHaveCount(0);
    await expect
      .poll(async () => (await guestContext.request.get('/api/v1/auth/session')).status(), {
        timeout: 3000,
      })
      .toBe(401);
  } finally {
    await guestContext.close();
    const all = await owner.get('/api/v1/admin/invitations');
    for (const item of (await all.json()) as InvitationData[]) {
      await revokeInvitation(owner, item.id);
    }
  }
});

test('K-6: an account limited to one library sees only that library and none of the other titles', async ({
  page,
  browser,
  baseURL,
  extraHTTPHeaders,
  owner,
}) => {
  await ensureEmptyLibrary(owner);
  const dialog = await openInvite(page);
  await dialog.getByLabel(/Für wen ist der Link/).fill('Nur leer');
  await choose(dialog, 'radio', 'Auswahl');
  await choose(dialog, 'checkbox', 'E2E leer');
  await dialog.getByRole('button', { name: 'Einladung erstellen' }).click();
  const inviteUrl = await dialog.getByRole('textbox', { name: 'Einladungslink' }).inputValue();
  await dialog.getByRole('button', { name: 'Fertig' }).click();
  await expect(row(page, 'Nur leer')).toContainText('E2E leer');

  const guestContext = await browser.newContext({ baseURL, locale: 'de-CH', extraHTTPHeaders });
  try {
    const guest = await guestContext.newPage();
    await guest.goto(inviteUrl);
    await expect(guest.getByRole('region', { name: 'Für: Nur leer' })).toContainText('E2E leer');
    await guest.getByLabel('E-Mail-Adresse').fill(GUEST.email);
    await guest.getByLabel(/^Passwort/).fill(GUEST.password);
    await guest.getByLabel(/^Profilname/).fill('Gast');
    await guest.getByRole('button', { name: 'Konto anlegen' }).click();
    await expect(guest).toHaveURL(/\/$/);

    // None of the titles of the other library, and only the allowed library is named.
    await expect(
      guest.getByRole('heading', { level: 2, name: 'Hier ist es noch leer' }),
    ).toBeVisible();
    await expect(guest.getByRole('heading', { level: 2, name: 'Neu hinzugefügt' })).toHaveCount(0);
    const libraries = (await (await guestContext.request.get('/api/v1/libraries')).json()) as {
      name: string;
    }[];
    expect(libraries.map((library) => library.name)).toEqual(['E2E leer']);

    // The guest's profile form offers that library by name; the owner's list shows it too.
    await guest.goto('/settings/profiles');
    await guest.getByRole('button', { name: 'Profil hinzufügen' }).click();
    const form = guest.getByRole('dialog', { name: 'Neues Profil' });
    await choose(form, 'radio', 'Auswahl');
    await expect(form.getByRole('checkbox', { name: 'E2E leer' })).toBeVisible();
    await expect(form.getByRole('checkbox', { name: 'Filme' })).toHaveCount(0);
    await openAccounts(page);
    await expect(accountRow(page, GUEST.email)).toContainText('1 Bibliothek');
  } finally {
    await guestContext.close();
    const all = await owner.get('/api/v1/admin/invitations');
    for (const item of (await all.json()) as InvitationData[]) {
      await revokeInvitation(owner, item.id);
    }
  }
});

test('a revoked invitation answers 410', async ({
  page,
  browser,
  baseURL,
  extraHTTPHeaders,
  owner,
}) => {
  const invitation = await createInvitation(owner, {
    note: 'Widerruf-E2E',
    maxRating: 6,
    allLibraries: true,
    libraryIds: [],
  });
  await openAccounts(page);
  const entry = row(page, 'Widerruf-E2E');
  await expect(entry).toContainText('Offen');
  await expect(entry).toContainText('FSK 6');
  await entry.getByRole('button', { name: 'Widerrufen' }).click();
  const confirm = page.getByRole('alertdialog', { name: 'Einladung widerrufen?' });
  await confirm.getByRole('button', { name: 'Abbrechen' }).click();
  await expect(entry).toContainText('Offen');
  await entry.getByRole('button', { name: 'Widerrufen' }).click();
  await confirm.getByRole('button', { name: 'Widerrufen' }).click();
  await expect(entry).toContainText('Widerrufen');
  await expect(entry.getByRole('button', { name: 'Widerrufen' })).toHaveCount(0);

  const visitor = await browser.newContext({ baseURL, locale: 'de-CH', extraHTTPHeaders });
  try {
    const visitorPage = await visitor.newPage();
    await visitorPage.goto(invitation.url);
    await expect(
      visitorPage.getByRole('heading', { level: 1, name: 'Link nicht mehr gültig' }),
    ).toBeVisible();
  } finally {
    await visitor.close();
  }
});

test('keyboard: invitation dialog, link and revoke dialog; no focus trap', async ({
  page,
  playwright,
  baseURL,
  extraHTTPHeaders,
  owner,
}) => {
  const anonymous = await anonymousApi(playwright, baseURL!, extraHTTPHeaders);
  try {
    // One open invitation and one guest, so there are rows to reach.
    await createInvitation(owner, {
      note: 'Offene Einladung',
      maxRating: 18,
      allLibraries: true,
      libraryIds: [],
    });
    const used = await createInvitation(owner, {
      note: 'Benutzte Einladung',
      maxRating: 18,
      allLibraries: true,
      libraryIds: [],
    });
    await acceptInvitation(anonymous, used.token, {
      email: GUEST.email,
      password: GUEST.password,
      profileName: 'Gast',
      language: 'de',
    });

    await openAccounts(page);

    // Enter on the one button that creates an account opens the dialog on its first field.
    const invite = page.getByRole('button', { name: 'Konto einladen' });
    await tabTo(page, invite);
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', { name: 'Konto einladen' });
    await expectFocusOn(page, dialog.getByLabel(/Für wen ist der Link/));
    await page.keyboard.type('Tastatur');
    await page.keyboard.press('Tab');
    await expectFocusOn(page, dialog.getByRole('radio', { name: 'FSK 18' }));
    await page.keyboard.press('ArrowLeft');
    await expect(dialog.getByRole('radio', { name: 'FSK 16' })).toBeChecked();
    await page.keyboard.press('Tab');
    await expectFocusOn(page, dialog.getByRole('radio', { name: 'Alle', exact: true }));
    await page.keyboard.press('ArrowRight');
    await expect(dialog.getByRole('radio', { name: 'Auswahl' })).toBeChecked();
    await page.keyboard.press('Tab');
    // The first checkbox of the list (the libraries are listed by name).
    const first = dialog.getByRole('checkbox').first();
    await expectFocusOn(page, first);
    await page.keyboard.press('Space');
    await expect(first).toBeChecked();
    // One Tab per further library, then the button.
    const others = (await dialog.getByRole('checkbox').count()) - 1;
    for (let index = 0; index < others; index += 1) await page.keyboard.press('Tab');
    await page.keyboard.press('Tab');
    await expectFocusOn(page, dialog.getByRole('button', { name: 'Einladung erstellen' }));
    await page.keyboard.press('Enter');

    // The link takes the focus (selected, ready for Ctrl+C); copy and done are next, and done
    // hands the focus back to the button that opened the dialog.
    const link = dialog.getByRole('textbox', { name: 'Einladungslink' });
    await expect(link).toBeVisible();
    await expectFocusOn(page, link);
    await page.keyboard.press('Tab');
    await expectFocusOn(page, dialog.getByRole('button', { name: 'Link kopieren' }));
    await page.keyboard.press('Tab');
    await expectFocusOn(page, dialog.getByRole('button', { name: 'Fertig' }));
    await page.keyboard.press('Enter');
    await expect(dialog).toBeHidden();
    await expectFocusOn(page, invite);
    const tastatur = row(page, 'Tastatur');
    await expect(tastatur).toContainText('Offen');
    await expect(tastatur).toContainText('FSK 16');

    // Revoke dialog: Enter opens it, the confirm button has the focus, Escape returns to the row.
    const revoke = row(page, 'Offene Einladung').getByRole('button', { name: 'Widerrufen' });
    await tabTo(page, revoke, 80);
    await page.keyboard.press('Enter');
    const confirm = page.getByRole('alertdialog', { name: 'Einladung widerrufen?' });
    await expectFocusOn(page, confirm.getByRole('button', { name: 'Widerrufen' }));
    await page.keyboard.press('Shift+Tab');
    await expectFocusOn(page, confirm.getByRole('button', { name: 'Abbrechen' }));
    await page.keyboard.press('Escape');
    await expect(confirm).toBeHidden();
    await expectFocusOn(page, revoke);

    await expectNoFocusTrap(page);
  } finally {
    const all = await owner.get('/api/v1/admin/invitations');
    for (const item of (await all.json()) as InvitationData[]) {
      await revokeInvitation(owner, item.id);
    }
    await anonymous.dispose();
  }
});
