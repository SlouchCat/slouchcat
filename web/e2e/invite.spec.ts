import type { APIRequestContext } from '@playwright/test';
import {
  acceptInvitation,
  anonymousApi,
  createInvitation,
  revokeInvitation,
  type InvitationData,
} from './accounts';
import { OWNER } from './env';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap } from './keyboard';

// The public page of an invitation link (K-6) as a visitor without a session.
test.use({ storageState: { cookies: [], origins: [] } });

const GUEST = { email: 'einlader-e2e@slouchcat.test', password: 'einlader-passwort-0123' };

const created: InvitationData[] = [];

test.afterEach(async ({ owner }) => {
  for (const invitation of created.splice(0)) await revokeInvitation(owner, invitation.id);
});

async function newInvitation(owner: APIRequestContext, note: string): Promise<InvitationData> {
  const invitation = await createInvitation(owner, {
    note,
    maxRating: 12,
    allLibraries: true,
    libraryIds: [],
  });
  created.push(invitation);
  return invitation;
}

test('Tab order: e-mail, password, profile name, language, submit; no focus trap', async ({
  page,
  owner,
}) => {
  const invitation = await newInvitation(owner, 'Tab-Test');
  await page.goto(invitation.url);
  await expect(
    page.getByRole('heading', { level: 1, name: 'Einladung zu Slouchcat' }),
  ).toBeVisible();
  const summary = page.getByRole('region', { name: 'Für: Tab-Test' });
  await expect(summary).toContainText('FSK 12');
  await expect(summary).toContainText('Alle Bibliotheken');

  await expectFocusOn(page, page.getByLabel('E-Mail-Adresse'));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByLabel(/^Passwort/));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByLabel(/^Profilname/));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('radio', { name: 'Deutsch' }));
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, page.getByRole('radio', { name: 'English' }));
  await expect(page.getByRole('radio', { name: 'English' })).toBeChecked();
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('button', { name: 'Konto anlegen' }));
  await expectNoFocusTrap(page);
});

test('validation: short password and a taken e-mail address are shown at the fields', async ({
  page,
  owner,
}) => {
  const invitation = await newInvitation(owner, 'Fehler-Test');
  await page.goto(invitation.url);
  await page.getByLabel('E-Mail-Adresse').fill(OWNER.email);
  await page.getByLabel(/^Passwort/).fill('kurz');
  await page.getByLabel(/^Profilname/).fill('Doppelt');
  await page.keyboard.press('Enter');
  await expect(page.getByText('Mindestens 10 Zeichen.', { exact: true }).last()).toBeVisible();

  await page.getByLabel(/^Passwort/).fill('langes-passwort-123');
  await page.keyboard.press('Enter');
  await expect(page.getByText('Diese E-Mail-Adresse ist schon vergeben.')).toBeVisible();
  await expect(page.getByLabel('E-Mail-Adresse')).toHaveAttribute('aria-invalid', 'true');
  await expect(page).toHaveURL(/\/invite\//);
});

test('Enter in the last field creates the account and opens the start page', async ({
  page,
  owner,
}) => {
  const invitation = await newInvitation(owner, 'Enter-Test');
  await page.goto(invitation.url);
  await page.getByLabel('E-Mail-Adresse').fill(GUEST.email);
  await page.getByLabel(/^Passwort/).fill(GUEST.password);
  await page.getByLabel(/^Profilname/).fill('Einlader');
  await page.getByLabel(/^Profilname/).press('Enter');
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole('link', { name: 'Profil wechseln, aktuell Einlader' })).toBeVisible();
});

test('a used link shows "Link nicht mehr gültig"; the way out is reachable by keyboard', async ({
  page,
  playwright,
  baseURL,
  extraHTTPHeaders,
  owner,
}) => {
  const invitation = await newInvitation(owner, 'Gone-Test');
  const anonymous = await anonymousApi(playwright, baseURL!, extraHTTPHeaders);
  try {
    await acceptInvitation(anonymous, invitation.token, {
      email: GUEST.email,
      password: GUEST.password,
      profileName: 'Erste',
      language: 'de',
    });
  } finally {
    await anonymous.dispose();
  }

  await page.goto(invitation.url);
  await expect(
    page.getByRole('heading', { level: 1, name: 'Link nicht mehr gültig' }),
  ).toBeVisible();
  await expect(page.getByRole('status')).toContainText(
    'schon benutzt, ist abgelaufen oder wurde zurückgezogen',
  );
  await expect(page.getByLabel('E-Mail-Adresse')).toHaveCount(0);

  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('link', { name: 'Zur Anmeldung' }));
  await expectNoFocusTrap(page);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/login$/);
});

test('an unknown link of the right shape shows "Link nicht gefunden"', async ({ page }) => {
  await page.goto('/invite/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA');
  await expect(page.getByRole('heading', { level: 1, name: 'Link nicht gefunden' })).toBeVisible();
  await expect(page.getByLabel('E-Mail-Adresse')).toHaveCount(0);
  await page.keyboard.press('Tab');
  await expectNoFocusTrap(page);
});
