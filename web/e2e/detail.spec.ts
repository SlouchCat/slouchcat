import { expect, test } from '@playwright/test';
import { H264_TITLE, itemPath } from './catalog';
import { expectFocusOn, expectNoFocusTrap, resetFocus, tabTo } from './keyboard';

let detail = '';

test.beforeEach(async ({ page }) => {
  detail ||= await itemPath(page, H264_TITLE);
  await page.goto('/');
  await page.goto(detail);
  await expect(page.getByRole('heading', { level: 1, name: H264_TITLE })).toBeVisible();
  await resetFocus(page);
});

test('arrow keys land on play, move to back; facts are shown', async ({ page }) => {
  const play = page.getByRole('link', { name: /^(Abspielen|Weiterschauen ab \d{2}:\d{2}:\d{2})$/ });
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, play);
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, page.getByRole('button', { name: 'Zurück' }));
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, play);
  const file = page.getByRole('region', { name: 'Datei' });
  await expect(file.getByText('MP4')).toBeVisible();
});

test('Tab reaches play and back after the header and the side navigation; no focus trap', async ({
  page,
}) => {
  // The side navigation's last stop (concept 08) comes right before the page.
  await tabTo(page, page.getByRole('button', { name: 'Seitenleiste ausblenden' }));
  await page.keyboard.press('Tab');
  await expectFocusOn(
    page,
    page.getByRole('link', { name: /^(Abspielen|Weiterschauen ab \d{2}:\d{2}:\d{2})$/ }),
  );
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('button', { name: 'Zurück' }));
  await expectNoFocusTrap(page);
});

for (const key of ['Escape', 'Backspace'] as const) {
  test(`${key} returns to the start page`, async ({ page }) => {
    await page.keyboard.press(key);
    await expect(page).toHaveURL(/\/$/);
  });
}

test('Enter on "Zurück" returns to where the viewer came from; browser history works', async ({
  page,
}) => {
  // Arrive the way a viewer does: from the start page, card focused, Enter.
  await page.goto('/');
  await page
    .getByRole('region', { name: 'Neu hinzugefügt' })
    .locator(`a[href="${detail}"]`)
    .focus();
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(new RegExp(`${detail}$`));
  await expect(page.getByRole('heading', { level: 1, name: H264_TITLE })).toBeVisible();

  await page.getByRole('button', { name: 'Zurück' }).focus();
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/$/);
  await page.goForward();
  await expect(page).toHaveURL(new RegExp(`${detail}$`));
  await page.goBack();
  await expect(page).toHaveURL(/\/$/);
});

test('an unknown id shows the not-found message', async ({ page }) => {
  await page.goto('/items/01JB8Z6Q4M7N9P2R3S5T7V9W1X');
  await expect(page.getByText('Titel nicht gefunden')).toBeVisible();
  await tabTo(page, page.getByRole('link', { name: 'Zur Startseite' }));
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/$/);
});
