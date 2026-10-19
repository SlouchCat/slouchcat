import { expect, test } from '@playwright/test';
import { expectFocusOn, expectNoFocusTrap, tabTo } from './keyboard';

// People (B-3). The e2e server has no TMDB key, so nobody has a credit: every person page is the
// not-found notice (a person without a visible credit does not exist for the profile, 404). The
// page with credits is covered by PersonPage.test.tsx.

test('a person without visible credits shows the not-found notice', async ({ page }) => {
  const response = page.waitForResponse((candidate) => candidate.url().endsWith('/people/1'));
  await page.goto('/people/1');
  expect((await response).status()).toBe(404);
  await expect(page.getByText('Person nicht gefunden')).toBeVisible();
  await expect(page.getByText('Zu dieser Person gibt es hier keine Titel.')).toBeVisible();
});

test('Tab reaches "Zur Startseite"; Enter goes home; no focus trap', async ({ page }) => {
  await page.goto('/people/abc');
  await expect(page.getByText('Person nicht gefunden')).toBeVisible();
  const home = page.getByRole('link', { name: 'Zur Startseite' });
  await tabTo(page, home);
  await expectFocusOn(page, home);
  await expectNoFocusTrap(page);
  await home.focus();
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/$/);
});

for (const key of ['Escape', 'Backspace'] as const) {
  test(`${key} goes back`, async ({ page }) => {
    await page.goto('/');
    await expect(page.getByRole('heading', { level: 2, name: 'Neu hinzugefügt' })).toBeVisible();
    await page.goto('/people/1');
    await expect(page.getByText('Person nicht gefunden')).toBeVisible();
    await page.keyboard.press(key);
    await expect(page).toHaveURL(/\/$/);
  });
}
