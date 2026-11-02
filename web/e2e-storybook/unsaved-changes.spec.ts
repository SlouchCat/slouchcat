import { expect, test, type Page } from '@playwright/test';

// The question of useUnsavedChanges (concept 12, S-13) starts on «Auf der Seite bleiben», however
// it was opened. Enter on a link is a "virtual" click for react-aria, whose autoFocus then waits
// for the CSS transitions of the page; the cancelled focus ring of the link used to stall it, and
// react-aria focused the dialog itself 500 ms later. That happened in a fraction of the runs only,
// so run this spec with --repeat-each when touching ConfirmDialog. Chromium and Firefox
// (storybook.config.ts).

const STORY = '/iframe.html?id=ui-useunsavedchanges--dirty&viewMode=story';

/** Holds past the 500 ms after which react-aria re-focuses a dialog that has lost its focus. */
async function expectStayFocused(page: Page) {
  await expect(page.getByRole('alertdialog')).toBeVisible();
  const stay = page.getByRole('button', { name: 'Auf der Seite bleiben' });
  await expect(stay).toBeFocused();
  await page.waitForTimeout(700);
  await expect(stay).toBeFocused({ timeout: 0 });
}

test('Enter on a link opens the question with the focus on staying', async ({ page }) => {
  await page.goto(STORY);
  await page.getByRole('link', { name: 'Sicherung' }).focus();
  await page.keyboard.press('Enter');
  await expectStayFocused(page);

  // Enter by habit stays on the page.
  await page.keyboard.press('Enter');
  await expect(page.getByRole('alertdialog')).toBeHidden();
  await expect(page.getByRole('textbox', { name: 'Zeitplan' })).toHaveValue('alle 2 h');
});

test('a mouse click on a link opens the question with the focus on staying', async ({ page }) => {
  await page.goto(STORY);
  await page.getByRole('link', { name: 'Sicherung' }).click();
  await expectStayFocused(page);
});

test('the Back button opens the question with the focus on staying', async ({ page }) => {
  await page.goto(`${STORY}#/`);
  await page.getByRole('link', { name: 'Hintergrundjobs' }).click();
  await expect(page.getByRole('textbox', { name: 'Zeitplan' })).toBeVisible();
  await page.goBack();
  await expectStayFocused(page);
});
