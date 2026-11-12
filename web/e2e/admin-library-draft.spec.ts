import { expect, test, type Page } from '@playwright/test';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// The kept draft of «Bibliothek hinzufügen» on the page "Bibliotheken" (concept 20, P-11, 13.2;
// CLAUDE.md rule 8). Step 0 retraces Marco's sequence: a name typed in the add dialog, the dialog
// cancelled, then something in «Ordner von …» of another library, then leaving the page. The
// question «bleiben oder verwerfen» came from the add dialog's draft (`useUnsavedChanges(
// draft.dirty)`), which the page showed nowhere. Now the page shows it in a band with
// «Weiter bearbeiten» and «Entwurf verwerfen», and «Abbrechen» asks whether to keep it.
//
// Nothing here adds a library: every draft is kept or discarded, so the run's library list stays
// as the other specs expect it.

// The worker would take the page over in the middle of a test (see admin-libraries.spec.ts).
test.use({ serviceWorkers: 'block' });

const BAND = 'Der Entwurf einer neuen Bibliothek ist noch nicht gespeichert.';

const addDialog = (page: Page) => page.getByRole('dialog', { name: 'Bibliothek hinzufügen' });
const addButton = (page: Page) => page.getByRole('button', { name: 'Bibliothek hinzufügen' });
const toggle = (page: Page, name: string) => page.getByRole('button', { name, exact: true });
const foldOut = (page: Page, name: string) => page.getByRole('region', { name, exact: true });
const keepQuestion = (page: Page) =>
  page.getByRole('alertdialog', { name: 'Entwurf behalten oder verwerfen?' });
const leaveQuestion = (page: Page) =>
  page.getByRole('alertdialog', { name: 'Seite mit ungespeicherten Änderungen verlassen?' });
const band = (page: Page) => page.getByText(BAND);
const continueButton = (page: Page) => page.getByRole('button', { name: 'Weiter bearbeiten' });
const discardButton = (page: Page) => page.getByRole('button', { name: 'Entwurf verwerfen' });
const jobsLink = (page: Page) =>
  page
    .getByRole('navigation', { name: 'Verwaltung' })
    .getByRole('link', { name: /^Hintergrundjobs(,|$)/ });

/** Opens «Ordner von Filme», puts a card into «Neue Ordner», takes it out and closes. */
async function touchFolders(page: Page): Promise<void> {
  if ((await toggle(page, 'Filme').getAttribute('aria-expanded')) !== 'true') {
    await toggle(page, 'Filme').click();
  }
  await foldOut(page, 'Filme').getByRole('button', { name: 'Ordner von Filme bearbeiten' }).click();
  const folders = page.getByRole('dialog', { name: 'Ordner von Filme' });
  await folders.getByRole('button', { name: 'Ordner hinzufügen', exact: true }).click();
  await folders.getByRole('button', { name: 'Ordner 1 entfernen' }).click();
  await folders.getByRole('button', { name: 'Schliessen' }).click();
  await expect(folders).toHaveCount(0);
}

test.beforeEach(async ({ page }) => {
  await page.goto('/admin/libraries');
  await expect(page.getByRole('heading', { level: 1, name: 'Bibliotheken' })).toBeVisible();
  await expect(toggle(page, 'Filme')).toBeVisible();
  await resetFocus(page);
});

test('step 0: a cancelled draft is what asks on leaving, and the band now shows it (P-11)', async ({
  page,
}) => {
  await addButton(page).click();
  await addDialog(page).getByLabel('Name', { exact: true }).fill('test3');
  await addDialog(page).getByRole('button', { name: 'Abbrechen' }).click();
  await keepQuestion(page).getByRole('button', { name: 'Entwurf behalten' }).click();
  await expect(addDialog(page)).toHaveCount(0);
  await expect(band(page)).toBeVisible();

  // The folders dialog saves every step itself; the question on leaving is the draft's.
  await touchFolders(page);
  await jobsLink(page).click();
  await expect(leaveQuestion(page)).toBeVisible();
  await leaveQuestion(page).getByRole('button', { name: 'Auf der Seite bleiben' }).click();
  await expect(leaveQuestion(page)).toHaveCount(0);
  await expect(band(page)).toBeVisible();

  // Discarded from the band, leaving does not ask any more.
  await discardButton(page).click();
  await expect(band(page)).toHaveCount(0);
  await jobsLink(page).click();
  await expect(page).toHaveURL(/\/admin\/jobs$/);
  await expect(leaveQuestion(page)).toHaveCount(0);
});

test('step 0 control: the folders dialog alone leaves no draft (P-11)', async ({ page }) => {
  await touchFolders(page);
  await expect(band(page)).toHaveCount(0);
  await jobsLink(page).click();
  await expect(page).toHaveURL(/\/admin\/jobs$/);
  await expect(leaveQuestion(page)).toHaveCount(0);
});

test('«Abbrechen» asks, the band goes on or discards, all by keyboard (13.2) @keyboard', async ({
  page,
}) => {
  // Open, type, Tab to «Abbrechen», Enter: the question starts on «Entwurf behalten».
  await tabTo(page, addButton(page), 10);
  await page.keyboard.press('Enter');
  const name = addDialog(page).getByLabel('Name', { exact: true });
  await expectFocusOn(page, name);
  await page.keyboard.type('E2E Entwurf');
  await tabTo(page, addDialog(page).getByRole('button', { name: 'Abbrechen' }), 30);
  await page.keyboard.press('Enter');
  const question = keepQuestion(page);
  await expect(question).toBeVisible();
  const keep = question.getByRole('button', { name: 'Entwurf behalten' });
  await expectFocusOn(page, keep);
  await expectVisibleFocus(page);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, question.getByRole('button', { name: 'Entwurf verwerfen' }));
  await page.keyboard.press('Shift+Tab');
  await expectFocusOn(page, keep);

  // Enter keeps it: both dialogs close, the focus is back on the opener, the band is there.
  await page.keyboard.press('Enter');
  await expect(question).toHaveCount(0);
  await expect(addDialog(page)).toHaveCount(0);
  await expectFocusOn(page, addButton(page));
  await expect(band(page)).toBeVisible();

  // The band comes right after the head: Tab, «Weiter bearbeiten», Enter opens the draft.
  await page.keyboard.press('Tab');
  await expectFocusOn(page, continueButton(page));
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  await expect(addDialog(page)).toBeVisible();
  await expect(name).toHaveValue('E2E Entwurf');
  await expect(band(page)).toHaveCount(0);

  // Escape keeps it without asking; the focus goes to the band's button, shown again.
  await page.keyboard.press('Escape');
  await expect(addDialog(page)).toHaveCount(0);
  await expect(keepQuestion(page)).toHaveCount(0);
  await expectFocusOn(page, continueButton(page));

  // Escape on the question keeps it as well.
  await page.keyboard.press('Enter');
  await expect(addDialog(page)).toBeVisible();
  await tabTo(page, addDialog(page).getByRole('button', { name: 'Abbrechen' }), 30);
  await page.keyboard.press('Enter');
  await expect(keepQuestion(page)).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(keepQuestion(page)).toHaveCount(0);
  await expect(addDialog(page)).toHaveCount(0);
  await expect(band(page)).toBeVisible();

  // «Entwurf verwerfen» in the band: the band goes, the h1 takes the focus, the status says so.
  await tabTo(page, discardButton(page), 10);
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  await expect(band(page)).toHaveCount(0);
  await expect(page.getByText('Der Entwurf der neuen Bibliothek ist verworfen.')).toBeVisible();
  await expect(page.getByRole('heading', { level: 1, name: 'Bibliotheken' })).toBeFocused();

  // A new draft discarded in the question: the dialog opens empty again.
  await tabTo(page, addButton(page), 10);
  await page.keyboard.press('Enter');
  await expectFocusOn(page, name);
  await expect(name).toHaveValue('');
  await page.keyboard.type('E2E Entwurf');
  await tabTo(page, addDialog(page).getByRole('button', { name: 'Abbrechen' }), 30);
  await page.keyboard.press('Enter');
  await expectFocusOn(page, keepQuestion(page).getByRole('button', { name: 'Entwurf behalten' }));
  await page.keyboard.press('Tab');
  await page.keyboard.press('Enter');
  await expect(keepQuestion(page)).toHaveCount(0);
  await expect(addDialog(page)).toHaveCount(0);
  await expect(band(page)).toHaveCount(0);
  await expectNoFocusTrap(page);
});
