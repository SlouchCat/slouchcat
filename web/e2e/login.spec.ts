import { expect, test } from '@playwright/test';
import { OWNER } from './env';
import { expectFocusOn, expectNoFocusTrap } from './keyboard';

test.use({ storageState: { cookies: [], origins: [] } });

test.beforeEach(async ({ page }) => {
  await page.goto('/');
  await expect(page).toHaveURL(/\/login$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Anmelden' })).toBeVisible();
});

test('Tab order: e-mail, password, submit; no focus trap', async ({ page }) => {
  await expectFocusOn(page, page.getByLabel('E-Mail-Adresse'));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByLabel('Passwort'));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('button', { name: 'Anmelden' }));
  await expectNoFocusTrap(page);
});

test('wrong password shows invalid_credentials', async ({ page }) => {
  await page.keyboard.type(OWNER.email);
  await page.keyboard.press('Tab');
  await page.keyboard.type('definitely-wrong');
  await page.keyboard.press('Enter');
  await expect(page.getByRole('alert')).toHaveText('E-Mail-Adresse oder Passwort stimmen nicht.');
  await expect(page).toHaveURL(/\/login$/);
});

test('Enter logs in and opens the start page; browser back does not return to the login', async ({
  page,
}) => {
  await page.keyboard.type(OWNER.email);
  await page.keyboard.press('Tab');
  await page.keyboard.type(OWNER.password);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole('navigation', { name: 'Hauptnavigation' })).toBeVisible();
  await page.goBack();
  await expect(page).not.toHaveURL(/\/login$/);
});
