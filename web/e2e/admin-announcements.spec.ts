import type { APIRequestContext, Page } from '@playwright/test';
import { CSRF } from './catalog';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// /admin/announcements (A-7) by keyboard (rule 8): write a draft, change it, publish it (the
// active profile then has it among its unread announcements) and delete it. A published
// announcement opens a dialog on every later page load of every profile until acknowledged, so
// each test deletes all of them afterwards.

interface AnnouncementData {
  id: number;
  title: string;
  publishedAt?: string;
}

/** Deletes every announcement (same as `deleteAnnouncements` of lane W3's e2e/mocks.ts). */
async function removeAnnouncements(api: APIRequestContext): Promise<void> {
  const response = await api.get('/api/v1/admin/announcements');
  if (!response.ok()) return;
  for (const announcement of (await response.json()) as AnnouncementData[]) {
    await api.delete(`/api/v1/admin/announcements/${announcement.id}`, { headers: CSRF });
  }
}

test.afterEach(async ({ owner }) => {
  await removeAnnouncements(owner);
});

const TITLE = 'Wartung am Samstag';
const BODY_LINES = ['Der Server ist von 8 bis 10 Uhr nicht erreichbar.', 'Danke für die Geduld!'];

const row = (page: Page, title: string) =>
  page
    .getByRole('table', { name: 'Ankündigungen' })
    .getByRole('row')
    .filter({ has: page.getByRole('rowheader', { name: title, exact: true }) });

/** Waits for the answer of the request `method path` while `action` runs. */
async function answered(page: Page, method: string, path: RegExp, action: () => Promise<void>) {
  const response = page.waitForResponse(
    (r) => r.request().method() === method && path.test(new URL(r.url()).pathname),
  );
  await action();
  return response;
}

test('A-7: draft, change, publish and delete an announcement by keyboard', async ({
  page,
  owner,
}) => {
  await page.goto('/admin/announcements');
  await expect(page.getByRole('heading', { level: 1, name: 'Ankündigungen' })).toBeVisible();
  await expect(page.getByText('Noch keine Ankündigungen.')).toBeVisible();

  // New: title, multi-line text, not published at once, saved as a draft.
  await resetFocus(page);
  const create = page.getByRole('button', { name: 'Neue Ankündigung' });
  await tabTo(page, create, 40);
  await page.keyboard.press('Enter');
  const dialog = page.getByRole('dialog', { name: 'Neue Ankündigung' });
  await expectFocusOn(page, dialog.getByRole('textbox', { name: 'Titel' }));
  await page.keyboard.type(TITLE);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, dialog.getByRole('textbox', { name: 'Text' }));
  await page.keyboard.type(BODY_LINES[0]);
  await page.keyboard.press('Enter');
  await page.keyboard.type(BODY_LINES[1]);
  await page.keyboard.press('Tab');
  const now = dialog.getByRole('checkbox', { name: 'Sofort veröffentlichen' });
  await expectFocusOn(page, now);
  await expect(now).not.toBeChecked();
  await expect(dialog).toContainText('Gäste bekommen sie standardmässig per E-Mail.');
  const save = dialog.getByRole('button', { name: 'Als Entwurf speichern' });
  await tabTo(page, save, 3);
  const created = await answered(page, 'POST', /\/admin\/announcements$/, () =>
    page.keyboard.press('Enter'),
  );
  expect(created.status()).toBe(201);
  const draft = (await created.json()) as AnnouncementData & { body: string };
  expect(draft.body).toBe(BODY_LINES.join('\n'));
  expect(draft.publishedAt).toBeUndefined();
  await expect(dialog).toBeHidden();
  await expectFocusOn(page, create);
  await expect(page.getByText(`„${TITLE}“ ist als Entwurf gespeichert.`)).toBeVisible();
  await expect(row(page, TITLE)).toContainText('Entwurf');
  await expect(row(page, TITLE)).toContainText('Noch nicht verschickt');

  // Change the draft: Enter in the title field saves.
  const edit = row(page, TITLE).getByRole('button', { name: `„${TITLE}“ bearbeiten` });
  await tabTo(page, edit, 20);
  await page.keyboard.press('Enter');
  const editing = page.getByRole('dialog', { name: 'Entwurf bearbeiten' });
  const title = editing.getByRole('textbox', { name: 'Titel' });
  await expectFocusOn(page, title);
  await expect(title).toHaveValue(TITLE);
  await page.keyboard.press('End');
  await page.keyboard.type(' (verschoben)');
  const changed = await answered(page, 'PUT', /\/admin\/announcements\/\d+$/, () =>
    page.keyboard.press('Enter'),
  );
  expect(changed.status()).toBe(200);
  const moved = `${TITLE} (verschoben)`;
  await expect(editing).toBeHidden();
  await expect(row(page, moved)).toContainText('Entwurf');

  // Publish: the confirmation names the channels; afterwards the counter and no more editing.
  const publish = row(page, moved).getByRole('button', { name: `„${moved}“ veröffentlichen` });
  await publish.focus();
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  const confirm = page.getByRole('alertdialog', { name: `„${moved}“ veröffentlichen?` });
  await expect(confirm).toContainText('Gäste bekommen sie standardmässig per E-Mail.');
  await expectFocusOn(page, confirm.getByRole('button', { name: 'Veröffentlichen' }));
  await page.keyboard.press('Escape');
  await expect(confirm).toBeHidden();
  await expectFocusOn(page, publish);
  await page.keyboard.press('Enter');
  const published = await answered(page, 'POST', /\/admin\/announcements\/\d+\/publish$/, () =>
    page.keyboard.press('Enter'),
  );
  expect(published.status()).toBe(200);
  await expect(page.getByText(`„${moved}“ ist veröffentlicht.`)).toBeVisible();
  await expect(row(page, moved)).toContainText(/0 von \d+ Profilen/);
  await expect(row(page, moved).getByRole('button')).toHaveCount(1);
  // The focus left the vanished buttons for the page heading, not for the document body.
  await expect(
    page.getByRole('heading', { level: 1, name: 'Ankündigungen' }).locator('..'),
  ).toBeFocused();
  // A-7: the active profile has it among its unread announcements now.
  const unread = await owner.get('/api/v1/announcements');
  expect(unread.status()).toBe(200);
  expect(((await unread.json()) as AnnouncementData[]).map((entry) => entry.title)).toContain(
    moved,
  );

  // Delete.
  const remove = row(page, moved).getByRole('button', { name: `„${moved}“ löschen` });
  await tabTo(page, remove, 30);
  await page.keyboard.press('Enter');
  const removal = page.getByRole('alertdialog', { name: `„${moved}“ löschen?` });
  await expectFocusOn(page, removal.getByRole('button', { name: 'Ankündigung löschen' }));
  const deleted = await answered(page, 'DELETE', /\/admin\/announcements\/\d+$/, () =>
    page.keyboard.press('Enter'),
  );
  expect(deleted.status()).toBe(204);
  await expect(page.getByText(`„${moved}“ ist gelöscht.`)).toBeVisible();
  await expect(page.getByText('Noch keine Ankündigungen.')).toBeVisible();
  await expectNoFocusTrap(page);
});

test('A-7: a new announcement can go out at once', async ({ page }) => {
  await page.goto('/admin/announcements');
  await resetFocus(page);
  await tabTo(page, page.getByRole('button', { name: 'Neue Ankündigung' }), 40);
  await page.keyboard.press('Enter');
  const dialog = page.getByRole('dialog', { name: 'Neue Ankündigung' });
  await page.keyboard.type('Neue Filme');
  await page.keyboard.press('Tab');
  await page.keyboard.type('Zwanzig neue Filme sind da.');
  await page.keyboard.press('Tab');
  await page.keyboard.press('Space');
  await expect(dialog.getByRole('checkbox', { name: 'Sofort veröffentlichen' })).toBeChecked();
  const submit = dialog.getByRole('button', { name: 'Veröffentlichen' });
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
  await expect(row(page, 'Neue Filme')).toContainText(/0 von \d+ Profilen/);
});
