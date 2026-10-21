import type { Page } from '@playwright/test';
import { createCollection } from './curation';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';
import { arrowTo } from './playback';

// /admin/collections (M7, E-6) by keyboard (rule 8): create a manual collection, fill it with two
// titles from the search and order them, find it in the start page row "Sammlungen" and on its
// own page, rename and delete it; ↑/↓ between the rows, Escape closes a dialog, no focus trap.
// Without TMDB the e2e run has no automatic collections.

const SUBTITLES_TITLE = 'Slouchcat Test Subtitles';
const PGS_TITLE = 'Slouchcat Test PGS';

const table = (page: Page) => page.getByRole('table', { name: 'Sammlungen' });
const row = (page: Page, name: string) =>
  table(page)
    .getByRole('row')
    .filter({ has: page.getByRole('rowheader').getByText(name, { exact: true }) });

test('collections page by keyboard: create, fill, order, show on the start page, rename, delete', async ({
  page,
  owner,
}) => {
  test.setTimeout(90_000);
  // A second collection, so the arrows have a row to move to ("E2E Zweite" sorts after).
  await createCollection(owner, 'E2E Zweite');
  await page.goto('/admin/collections');
  await expect(page.getByRole('heading', { level: 1, name: 'Sammlungen' })).toBeVisible();
  await expect(
    page.getByRole('navigation', { name: 'Verwaltung' }).getByRole('link', { name: 'Sammlungen' }),
  ).toHaveAttribute('aria-current', 'page');

  // "Sammlung anlegen": name, Enter; the focus returns to the button.
  await resetFocus(page);
  const create = page.getByRole('button', { name: 'Sammlung anlegen' });
  await tabTo(page, create);
  await page.keyboard.press('Enter');
  const createDialog = page.getByRole('dialog', { name: 'Sammlung anlegen' });
  const name = createDialog.getByRole('textbox', { name: 'Name' });
  await expectFocusOn(page, name);
  await page.keyboard.type('E2E Filmabend');
  const created = page.waitForResponse(
    (response) =>
      response.request().method() === 'POST' && response.url().endsWith('/admin/collections'),
  );
  await page.keyboard.press('Enter');
  expect((await created).status()).toBe(201);
  await expect(createDialog).toHaveCount(0);
  await expect(page.getByText('Die Sammlung E2E Filmabend ist angelegt.')).toBeVisible();
  await expectFocusOn(page, create);
  await expect(row(page, 'E2E Filmabend')).toContainText('Manuell');

  // ↓/↑ between the rows.
  const members = row(page, 'E2E Filmabend').getByRole('button', {
    name: 'Mitglieder von E2E Filmabend',
  });
  await members.focus();
  await arrowTo(
    page,
    row(page, 'E2E Zweite').getByRole('button', { name: 'Mitglieder von E2E Zweite' }),
  );
  await expectVisibleFocus(page);
  await arrowTo(page, members, 'ArrowUp');

  // "Mitglieder": two titles from the search, PGS moved first, saved.
  await page.keyboard.press('Enter');
  const dialog = page.getByRole('dialog', { name: 'Mitglieder von E2E Filmabend' });
  await expect(dialog.getByText('Noch kein Titel in dieser Sammlung.')).toBeVisible();
  const search = dialog.getByRole('searchbox', { name: 'Titel suchen' });
  await expectFocusOn(page, search);
  const addTitle = async (query: string, title: string) => {
    await search.fill('');
    await page.keyboard.type(query);
    const add = dialog.getByRole('button', { name: `${title} hinzufügen`, exact: true });
    await expect(add).toBeVisible();
    await tabTo(page, add, 10);
    await page.keyboard.press('Enter');
    // The hit leaves the list: the focus is back in the search field.
    await expectFocusOn(page, search);
  };
  await addTitle('Slouchcat Test Subt', SUBTITLES_TITLE);
  await addTitle('Slouchcat Test PGS', PGS_TITLE);
  const list = dialog.getByRole('list', { name: 'Titel der Sammlung' });
  await expect(list.getByRole('listitem')).toHaveCount(2);
  const up = list.getByRole('button', { name: `${PGS_TITLE} nach oben` });
  await tabTo(page, up, 20);
  await page.keyboard.press('Enter');
  await expectFocusOn(page, list.getByRole('button', { name: `${PGS_TITLE} nach unten` }));
  await expect(list.getByRole('listitem').first()).toContainText(PGS_TITLE);
  const saved = page.waitForResponse(
    (response) => response.request().method() === 'PUT' && response.url().endsWith('/items'),
  );
  await tabTo(page, dialog.getByRole('button', { name: 'Speichern' }), 10);
  await page.keyboard.press('Enter');
  expect((await saved).status()).toBe(200);
  await expect(dialog).toHaveCount(0);
  await expect(page.getByText('Die Mitglieder von E2E Filmabend sind gespeichert.')).toBeVisible();
  await expect(
    row(page, 'E2E Filmabend').getByRole('cell').filter({ hasText: /^2$/ }),
  ).toBeVisible();

  // The start page row "Sammlungen" links to the collection's page.
  await page.goto('/');
  const card = page
    .getByRole('region', { name: 'Sammlungen' })
    .getByRole('link', { name: /E2E Filmabend/ });
  await expect(card).toBeVisible();
  await card.focus();
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/collections\/[0-9A-Z]{26}$/);
  await expect(page.getByRole('heading', { level: 1, name: 'E2E Filmabend' })).toBeVisible();
  await page.goBack();
  await page.goto('/admin/collections');

  // Escape leaves the rename dialog unchanged; the focus returns to its button.
  const rename = row(page, 'E2E Filmabend').getByRole('button', {
    name: 'E2E Filmabend umbenennen',
  });
  await rename.focus();
  await page.keyboard.press('Enter');
  const renameDialog = page.getByRole('dialog', { name: 'E2E Filmabend umbenennen' });
  await expect(renameDialog.getByRole('textbox', { name: 'Name' })).toHaveValue('E2E Filmabend');
  await page.keyboard.press('Escape');
  await expect(renameDialog).toHaveCount(0);
  await expectFocusOn(page, rename);

  // "Umbenennen".
  await page.keyboard.press('Enter');
  const field = renameDialog.getByRole('textbox', { name: 'Name' });
  await expectFocusOn(page, field);
  await field.fill('E2E Kinoabend');
  await page.keyboard.press('Enter');
  await expect(renameDialog).toHaveCount(0);
  await expect(page.getByText('Die Sammlung heisst jetzt E2E Kinoabend.')).toBeVisible();
  await expect(row(page, 'E2E Kinoabend')).toHaveCount(1);

  // "Löschen" with its confirmation; the focus goes to the heading.
  const remove = row(page, 'E2E Kinoabend').getByRole('button', {
    name: 'Sammlung E2E Kinoabend löschen',
  });
  await resetFocus(page);
  await tabTo(page, remove, 60);
  await page.keyboard.press('Enter');
  const confirm = page.getByRole('alertdialog', { name: 'Sammlung E2E Kinoabend löschen?' });
  await expectFocusOn(page, confirm.getByRole('button', { name: 'Sammlung löschen' }));
  await page.keyboard.press('Enter');
  await expect(row(page, 'E2E Kinoabend')).toHaveCount(0);
  await expect(page.getByText('Die Sammlung E2E Kinoabend ist gelöscht.')).toBeVisible();
  await expect(
    page.getByRole('heading', { level: 1, name: 'Sammlungen' }).locator('..'),
  ).toBeFocused();
  await expectNoFocusTrap(page);
});
