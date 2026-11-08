import { expect, test, type Page } from '@playwright/test';
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

// The big button of a film (concept 17 E-10): without progress "Abspielen" with the runtime; other
// specs may have left progress behind ("Fortsetzen bei …", "Nochmals ansehen …").
const play = (page: Page) =>
  page.getByRole('link', { name: /^(Abspielen|Fortsetzen|Nochmals ansehen)( |$)/ });
const back = (page: Page) => page.getByRole('button', { name: 'Zurück' });

test('arrows: start on play, up to the back arrow, down to the round actions; facts are shown', async ({
  page,
}) => {
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, play(page));
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, back(page));
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, play(page));
  // Under the big button the round actions in their order (E-11): → walks them to "Merken".
  await page.keyboard.press('ArrowDown');
  const merken = page.getByRole('button', { name: /^(Merken|Gemerkt)$/ });
  for (let presses = 0; presses < 3; presses += 1) {
    if (await merken.evaluate((element) => element === document.activeElement)) break;
    await page.keyboard.press('ArrowRight');
  }
  await expectFocusOn(page, merken);
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, page.getByRole('link', { name: 'Zu den User-Empfehlungen' }));
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, play(page));
  const file = page.getByRole('region', { name: 'Datei' });
  await expect(file.getByText('MP4')).toBeVisible();
});

test('Tab: back arrow, pen, watchlist on the cover, play, round actions; no focus trap @keyboard', async ({
  page,
}) => {
  // The side navigation's last stop (concept 08) comes right before the page: the back arrow, for
  // the server administrator the pen on the poster (concept 15 7.4), the watchlist button on the
  // cover (concept 17 D-9), then play and the round actions.
  await tabTo(page, page.getByRole('button', { name: 'Seitenleiste ausblenden' }));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, back(page));
  await page.keyboard.press('Tab');
  await expectFocusOn(
    page,
    page.getByRole('button', { name: 'Metadaten bearbeiten', exact: true }),
  );
  await page.keyboard.press('Tab');
  await expectFocusOn(
    page,
    page.getByRole('button', {
      name: new RegExp(`^${H264_TITLE} (merken|von der Merkliste nehmen)$`),
    }),
  );
  await page.keyboard.press('Tab');
  await expectFocusOn(page, play(page));
  await tabTo(page, page.getByRole('button', { name: /^(Merken|Gemerkt)$/ }), 3);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('link', { name: 'Zu den User-Empfehlungen' }));
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
