import type { Locator, Page } from '@playwright/test';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, resetFocus, tabTo } from './keyboard';
import { deleteAnnouncements } from './mocks';

// /admin/announcements (A-7; concept 12, variant B) by keyboard (rule 8): write a draft in place,
// change it, publish it (the active profile then has it among its unread announcements), move
// through the list with the arrow keys and delete it. A published announcement opens a dialog on
// every later page load of every profile until acknowledged, so each test deletes all of them
// afterwards.

interface AnnouncementData {
  id: number;
  title: string;
  publishedAt?: string;
}

test.afterEach(async ({ owner }) => {
  await deleteAnnouncements(owner);
});

const TITLE = 'Wartung am Samstag';
const BODY_LINES = ['Der Server ist von 8 bis 10 Uhr nicht erreichbar.', 'Danke für die Geduld!'];

const heading = (page: Page) => page.getByRole('heading', { level: 1, name: 'Ankündigungen' });
const list = (page: Page) => page.getByRole('listbox', { name: 'Ankündigungen' });
/** The detail of an announcement: a panel named by its title. */
const region = (page: Page, name: string) => page.getByRole('region', { name, exact: true });

/** Waits for the answer of the request `method path` while `action` runs. */
async function answered(page: Page, method: string, path: RegExp, action: () => Promise<void>) {
  const response = page.waitForResponse(
    (r) => r.request().method() === method && path.test(new URL(r.url()).pathname),
  );
  await action();
  return response;
}

/** Moves the focus into a single-Tab-stop list by Tab, then ↓ or ↑ until `target` has it. */
async function arrowTo(
  page: Page,
  entry: Locator,
  target: Locator,
  key: 'ArrowDown' | 'ArrowUp' = 'ArrowDown',
  max = 10,
) {
  await resetFocus(page);
  await tabTo(page, entry, 200);
  for (let presses = 0; presses < max; presses += 1) {
    if (await target.evaluate((element) => element === document.activeElement)) break;
    await page.keyboard.press(key);
  }
  await expectFocusOn(page, target);
}

test('A-7: draft, change, publish and delete an announcement by keyboard', async ({
  page,
  owner,
}) => {
  await page.goto('/admin/announcements');
  await expect(heading(page)).toBeVisible();
  await expect(page.getByText('Noch keine Ankündigungen.')).toBeVisible();

  // New: in place, title, multi-line text, not published at once, saved as a draft.
  await resetFocus(page);
  const create = page.getByRole('button', { name: 'Neue Ankündigung' });
  await tabTo(page, create, 40);
  await page.keyboard.press('Enter');
  const form = page.getByRole('form', { name: 'Neue Ankündigung' });
  await expectFocusOn(page, form.getByRole('textbox', { name: 'Titel' }));
  await page.keyboard.type(TITLE);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, form.getByRole('textbox', { name: 'Text' }));
  await page.keyboard.type(BODY_LINES[0]);
  await page.keyboard.press('Enter');
  await page.keyboard.type(BODY_LINES[1]);
  // The preview shows the profiles' dialog with what is typed, before anything is saved.
  const newPanel = region(page, 'Neue Ankündigung');
  await expect(newPanel.getByText(TITLE, { exact: true })).toBeVisible();
  await expect(newPanel).toContainText(BODY_LINES[1]);
  await page.keyboard.press('Tab');
  const now = form.getByRole('checkbox', { name: 'Sofort veröffentlichen' });
  await expectFocusOn(page, now);
  await expect(now).not.toBeChecked();
  await expect(form).toContainText('Gäste bekommen sie standardmässig per E-Mail.');
  const save = form.getByRole('button', { name: 'Als Entwurf speichern' });
  await tabTo(page, save, 3);
  const created = await answered(page, 'POST', /\/admin\/announcements$/, () =>
    page.keyboard.press('Enter'),
  );
  expect(created.status()).toBe(201);
  const draft = (await created.json()) as AnnouncementData & { body: string };
  expect(draft.body).toBe(BODY_LINES.join('\n'));
  expect(draft.publishedAt).toBeUndefined();
  await expect(form).toBeHidden();
  // The form went with the save: the same button of the saved draft has the focus.
  await expectFocusOn(
    page,
    region(page, TITLE).getByRole('button', { name: 'Als Entwurf speichern' }),
  );
  await expect(page.getByText(`„${TITLE}“ ist als Entwurf gespeichert.`)).toBeVisible();
  await expect(page).toHaveURL(new RegExp(`[?&]item=announcement-${draft.id}$`));
  await expect(region(page, TITLE)).toContainText('Entwurf');
  await expect(region(page, TITLE)).toContainText('Noch nicht verschickt');

  // Change the draft in place: Enter in the title field saves. Without a change, saving sends
  // nothing and the focus stays.
  const title = region(page, TITLE).getByRole('textbox', { name: 'Titel' });
  const puts: string[] = [];
  page.on('request', (sent) => {
    if (sent.method() === 'PUT') puts.push(sent.url());
  });
  await page.keyboard.press('Enter');
  await expectFocusOn(
    page,
    region(page, TITLE).getByRole('button', { name: 'Als Entwurf speichern' }),
  );
  expect(puts).toEqual([]);
  await page.keyboard.press('Shift+Tab');
  await page.keyboard.press('Shift+Tab');
  await expectFocusOn(page, title);
  await expect(title).toHaveValue(TITLE);
  await page.keyboard.press('End');
  await page.keyboard.type(' (verschoben)');
  const changed = await answered(page, 'PUT', /\/admin\/announcements\/\d+$/, () =>
    page.keyboard.press('Enter'),
  );
  expect(changed.status()).toBe(200);
  const moved = `${TITLE} (verschoben)`;
  await expect(page.getByText(`„${moved}“ ist als Entwurf gespeichert.`)).toBeVisible();
  await expect(region(page, moved)).toContainText('Entwurf');
  await expect(list(page).getByRole('option', { name: moved })).toBeVisible();

  // Publish: the question names the channels; Escape keeps the draft, Enter publishes it.
  const publish = region(page, moved).getByRole('button', { name: `„${moved}“ veröffentlichen` });
  await tabTo(page, publish, 6);
  await page.keyboard.press('Enter');
  const confirm = page.getByRole('alertdialog', { name: `„${moved}“ veröffentlichen?` });
  await expect(confirm).toContainText('Gäste bekommen sie standardmässig per E-Mail.');
  await expectFocusOn(page, confirm.getByRole('button', { name: 'Veröffentlichen' }));
  await page.keyboard.press('Escape');
  await expect(confirm).toBeHidden();
  await expectFocusOn(page, publish);
  await page.keyboard.press('Enter');
  await expectFocusOn(page, confirm.getByRole('button', { name: 'Veröffentlichen' }));
  const published = await answered(page, 'POST', /\/admin\/announcements\/\d+\/publish$/, () =>
    page.keyboard.press('Enter'),
  );
  expect(published.status()).toBe(200);
  await expect(page.getByText(`„${moved}“ ist veröffentlicht.`)).toBeVisible();
  const sent = region(page, moved);
  await expect(sent).toContainText(/0 von \d+ Profilen/);
  await expect(sent.getByRole('progressbar', { name: 'Gelesen' })).toHaveAttribute(
    'aria-valuenow',
    '0',
  );
  // As it was sent: no form any more, only «Löschen».
  await expect(sent.getByRole('textbox')).toHaveCount(0);
  await expect(sent.getByRole('button')).toHaveCount(1);
  // The form and its buttons went: the focus is on the page heading.
  await expect(heading(page)).toBeFocused();
  // A-7: the active profile has it among its unread announcements now.
  const unread = await owner.get('/api/v1/announcements');
  expect(unread.status()).toBe(200);
  expect(((await unread.json()) as AnnouncementData[]).map((entry) => entry.title)).toContain(
    moved,
  );

  // A second, new one: the list moves with ↑ and ↓, Enter selects, the text stays meanwhile.
  await tabTo(page, create, 10);
  await page.keyboard.press('Enter');
  await expectFocusOn(page, form.getByRole('textbox', { name: 'Titel' }));
  await page.keyboard.type('Kinoabend');
  const unsaved = list(page).getByRole('option', { name: /Kinoabend/ });
  const sentEntry = list(page).getByRole('option', { name: moved });
  // The list is one Tab stop, and Tab lands on the selected entry (MasterDetail).
  await resetFocus(page);
  await tabTo(page, unsaved, 200);
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, sentEntry);
  await page.keyboard.press('Enter');
  await expect(region(page, moved)).toBeVisible();
  // The first saved one is the page's default: no `?item=`.
  await expect(page).toHaveURL(/\/admin\/announcements$/);
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, unsaved);
  await page.keyboard.press('Enter');
  await expect(form.getByRole('textbox', { name: 'Titel' })).toHaveValue('Kinoabend');
  // The new one has an address of its own.
  await expect(page).toHaveURL(/[?&]item=new$/);

  // «Abbrechen» asks before the typed text goes; the question starts on «Abbrechen».
  await tabTo(page, form.getByRole('button', { name: 'Abbrechen' }), 10);
  await page.keyboard.press('Enter');
  const discard = page.getByRole('alertdialog', { name: 'Neue Ankündigung verwerfen?' });
  await expectFocusOn(page, discard.getByRole('button', { name: 'Abbrechen' }));
  await tabTo(page, discard.getByRole('button', { name: 'Verwerfen' }), 2);
  await page.keyboard.press('Enter');
  await expect(discard).toBeHidden();
  await expect(form).toBeHidden();
  await expect(unsaved).toHaveCount(0);
  await expect(heading(page)).toBeFocused();
  await expect(region(page, moved)).toBeVisible();

  // Delete.
  const remove = region(page, moved).getByRole('button', { name: `„${moved}“ löschen` });
  await tabTo(page, remove, 30);
  await page.keyboard.press('Enter');
  const removal = page.getByRole('alertdialog', { name: `„${moved}“ löschen?` });
  await expectFocusOn(page, removal.getByRole('button', { name: 'Ankündigung löschen' }));
  const deleted = await answered(page, 'DELETE', /\/admin\/announcements\/\d+$/, () =>
    page.keyboard.press('Enter'),
  );
  expect(deleted.status()).toBe(204);
  await expect(removal).toBeHidden();
  await expect(page.getByText(`„${moved}“ ist gelöscht.`)).toBeVisible();
  await expect(page.getByText('Noch keine Ankündigungen.')).toBeVisible();
  await expect(heading(page)).toBeFocused();
  await expectNoFocusTrap(page);
});

test('A-7: a new announcement can go out at once', async ({ page }) => {
  await page.goto('/admin/announcements');
  await expect(heading(page)).toBeVisible();
  await resetFocus(page);
  await tabTo(page, page.getByRole('button', { name: 'Neue Ankündigung' }), 40);
  await page.keyboard.press('Enter');
  const form = page.getByRole('form', { name: 'Neue Ankündigung' });
  await expectFocusOn(page, form.getByRole('textbox', { name: 'Titel' }));
  await page.keyboard.type('Neue Filme');
  await page.keyboard.press('Tab');
  await page.keyboard.type('Zwanzig neue Filme sind da.');
  await page.keyboard.press('Tab');
  await page.keyboard.press('Space');
  await expect(form.getByRole('checkbox', { name: 'Sofort veröffentlichen' })).toBeChecked();
  const submit = form.getByRole('button', { name: 'Veröffentlichen' });
  await tabTo(page, submit, 3);
  const created = await answered(page, 'POST', /\/admin\/announcements$/, () =>
    page.keyboard.press('Enter'),
  );
  expect(created.status()).toBe(201);
  expect(created.request().postDataJSON()).toEqual({
    title: 'Neue Filme',
    body: 'Zwanzig neue Filme sind da.',
    publish: true,
  });
  expect(((await created.json()) as AnnouncementData).publishedAt).toBeTruthy();
  await expect(page.getByText('„Neue Filme“ ist veröffentlicht.')).toBeVisible();
  await expect(region(page, 'Neue Filme')).toContainText(/0 von \d+ Profilen/);
  // The form and its buttons went: the page puts the focus on the h1 a few frames after the
  // message (focusTitleIfLost). Only then does the keyboard run start from the top: a focus
  // placement that lands between resetFocus and the first Tab would start the Tab sequence at the
  // h1, below the sidebar, and headless Firefox does not wrap around to it.
  await expect(heading(page)).toBeFocused();

  // Back (rule 8): the sidebar leads to «Hintergrundjobs», the browser's Back returns to the
  // announcement in the address.
  const here = page.url();
  const sidebar = page.getByRole('navigation', { name: 'Verwaltung' });
  await arrowTo(
    page,
    sidebar.getByRole('link', { name: 'Ankündigungen' }),
    sidebar.getByRole('link', { name: 'Hintergrundjobs' }),
  );
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin\/jobs$/);
  await page.goBack();
  await expect(page).toHaveURL(here);
  await expect(heading(page)).toBeVisible();
  await expect(region(page, 'Neue Filme')).toBeVisible();
});

test('A-7: leaving with an unsaved text asks first', async ({ page, owner }) => {
  await page.goto('/admin/announcements');
  await expect(heading(page)).toBeVisible();
  await resetFocus(page);
  await tabTo(page, page.getByRole('button', { name: 'Neue Ankündigung' }), 40);
  await page.keyboard.press('Enter');
  const form = page.getByRole('form', { name: 'Neue Ankündigung' });
  await expectFocusOn(page, form.getByRole('textbox', { name: 'Titel' }));
  await page.keyboard.type('Kinoabend');

  // The sidebar link waits for the question; Escape stays on the page with the text.
  const sidebar = page.getByRole('navigation', { name: 'Verwaltung' });
  const current = sidebar.getByRole('link', { name: 'Ankündigungen' });
  const jobs = sidebar.getByRole('link', { name: 'Hintergrundjobs' });
  await arrowTo(page, current, jobs);
  await page.keyboard.press('Enter');
  const leave = page.getByRole('alertdialog', {
    name: 'Seite mit ungespeicherten Änderungen verlassen?',
  });
  const stay = leave.getByRole('button', { name: 'Auf der Seite bleiben' });
  await expectFocusOn(page, stay);
  await page.keyboard.press('Escape');
  await expect(leave).toBeHidden();
  await expect(page).toHaveURL(/\/admin\/announcements/);
  await expect(form.getByRole('textbox', { name: 'Titel' })).toHaveValue('Kinoabend');

  // «Verwerfen und verlassen» goes; nothing was saved.
  await arrowTo(page, current, jobs);
  await page.keyboard.press('Enter');
  await expectFocusOn(page, stay);
  await tabTo(page, leave.getByRole('button', { name: 'Verwerfen und verlassen' }), 3);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin\/jobs/);
  const saved = await owner.get('/api/v1/admin/announcements');
  expect(await saved.json()).toEqual([]);
});
