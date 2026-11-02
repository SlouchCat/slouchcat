import { expect, test, type Locator, type Page } from '@playwright/test';

// The initial focus of the dialogs of web/src/ui, however they were opened. Enter on a link or a
// native button is a "virtual" click for react-aria, whose autoFocus then waits for the CSS
// transitions of the page; with reduced motion every focus ring change is a transition, the modal
// cancelled the one of the opener, and react-aria focused the dialog itself 500 ms later instead
// (focusOnMount.ts). That happened in a fraction of the runs only, so run this spec with
// --repeat-each when touching a dialog. Chromium and Firefox (storybook.config.ts), reduced motion
// like every story run.

const UNSAVED = '/iframe.html?id=ui-useunsavedchanges--dirty&viewMode=story';
const TYPED =
  '/iframe.html?id=ui-confirmdialog--typed-confirmation-opened-by-button&viewMode=story';
const MESSAGE = '/iframe.html?id=ui-messagedialog--opened-by-link&viewMode=story';

/** Holds past the 500 ms after which react-aria re-focuses a dialog that has lost its focus. */
async function expectKeepsFocus(page: Page, dialog: Locator, target: Locator) {
  await expect(dialog).toBeVisible();
  await expect(target).toBeFocused();
  await page.waitForTimeout(700);
  await expect(target).toBeFocused({ timeout: 0 });
}

/** The question of useUnsavedChanges (concept 12, S-13) starts on «Auf der Seite bleiben». */
async function expectStayFocused(page: Page) {
  await expectKeepsFocus(
    page,
    page.getByRole('alertdialog'),
    page.getByRole('button', { name: 'Auf der Seite bleiben' }),
  );
}

test('Enter on a link opens the question with the focus on staying', async ({ page }) => {
  await page.goto(UNSAVED);
  await page.getByRole('link', { name: 'Sicherung' }).focus();
  await page.keyboard.press('Enter');
  await expectStayFocused(page);

  // Enter by habit stays on the page.
  await page.keyboard.press('Enter');
  await expect(page.getByRole('alertdialog')).toBeHidden();
  await expect(page.getByRole('textbox', { name: 'Zeitplan' })).toHaveValue('alle 2 h');
});

test('a mouse click on a link opens the question with the focus on staying', async ({ page }) => {
  await page.goto(UNSAVED);
  await page.getByRole('link', { name: 'Sicherung' }).click();
  await expectStayFocused(page);
});

test('the Back button opens the question with the focus on staying', async ({ page }) => {
  await page.goto(`${UNSAVED}#/`);
  await page.getByRole('link', { name: 'Hintergrundjobs' }).click();
  await expect(page.getByRole('textbox', { name: 'Zeitplan' })).toBeVisible();
  await page.goBack();
  await expectStayFocused(page);
});

test('Enter on a native button opens the typed confirmation with the focus in the field', async ({
  page,
}) => {
  await page.goto(TYPED);
  await page.getByRole('button', { name: 'Ordner entfernen' }).focus();
  await page.keyboard.press('Enter');
  await expectKeepsFocus(
    page,
    page.getByRole('alertdialog'),
    page.getByRole('textbox', { name: 'Tippe den Pfad, um zu bestätigen' }),
  );
});

test('Enter on a link opens the message with the focus on confirming', async ({ page }) => {
  await page.goto(MESSAGE);
  await page.getByRole('link', { name: 'Ankündigung lesen' }).focus();
  await page.keyboard.press('Enter');
  await expectKeepsFocus(
    page,
    page.getByRole('dialog'),
    page.getByRole('button', { name: 'Verstanden' }),
  );
});
