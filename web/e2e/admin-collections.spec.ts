import type { Locator, Page } from '@playwright/test';
import { createCollection, deleteCollection } from './curation';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';
import { arrowTo } from './playback';

// /admin/collections (M7, E-6; concept 12, variant C) by keyboard (rule 8): create a manual
// collection, switch the source tabs with ←/→, move between the rows with ↑/↓, fold a row out with
// Enter and fill it with two titles from the search, put the second first by its position (↑ on
// the select), get asked when leaving with unsaved members, save, find it in the start page row
// «Sammlungen» and on its own page, rename and delete it; Escape closes a dialog, visible focus
// throughout, no focus trap. Without TMDB the e2e run has no automatic collections.

const SUBTITLES_TITLE = 'Slouchcat Test Subtitles';
const PGS_TITLE = 'Slouchcat Test PGS';

const rowOf = (page: Page, id: string): Locator => page.locator(`tr[data-row-key="${id}"]`);
/** The name button that folds the row of collection `id` out and in. */
const toggleOf = (page: Page, id: string): Locator =>
  rowOf(page, id).getByRole('rowheader').getByRole('button');

test('collections page by keyboard: create, fill, order, show on the start page, rename, delete', async ({
  page,
  owner,
}) => {
  test.setTimeout(90_000);
  // A second collection, so the arrows have a row to move to ("E2E Zweite" sorts after).
  const second = await createCollection(owner, 'E2E Zweite');
  let created: string | undefined;
  try {
    await page.goto('/admin/collections');
    const heading = page.getByRole('heading', { level: 1, name: 'Sammlungen' });
    await expect(heading).toBeVisible();
    await expect(
      page
        .getByRole('navigation', { name: 'Verwaltung' })
        .getByRole('link', { name: 'Sammlungen' }),
    ).toHaveAttribute('aria-current', 'page');

    // «Sammlung anlegen»: name, Enter; the focus returns to the button.
    await resetFocus(page);
    const create = page.getByRole('button', { name: 'Sammlung anlegen' });
    await tabTo(page, create);
    await page.keyboard.press('Enter');
    const createDialog = page.getByRole('dialog', { name: 'Sammlung anlegen' });
    const name = createDialog.getByRole('textbox', { name: 'Name' });
    await expectFocusOn(page, name);
    await page.keyboard.type('E2E Filmabend');
    const posted = page.waitForResponse(
      (response) =>
        response.request().method() === 'POST' && response.url().endsWith('/admin/collections'),
    );
    await page.keyboard.press('Enter');
    const answer = await posted;
    expect(answer.status()).toBe(201);
    created = ((await answer.json()) as { id: string }).id;
    await expect(createDialog).toHaveCount(0);
    await expect(page.getByText('Die Sammlung E2E Filmabend ist angelegt.')).toBeVisible();
    await expectFocusOn(page, create);
    const row = rowOf(page, created);
    await expect(row).toContainText('Manuell');

    // ←/→ switch the source tabs; the tab lives in the address (replaced).
    const tabs = page.getByRole('tablist', { name: 'Sammlungen nach Quelle' });
    const all = tabs.getByRole('tab', { name: /^Alle/ });
    await tabTo(page, all);
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, tabs.getByRole('tab', { name: /^Manuell/ }));
    await expect(page).toHaveURL(/\/admin\/collections\?tab=manual$/);
    await expect(page.getByRole('table', { name: 'Manuelle Sammlungen' })).toBeVisible();
    await page.keyboard.press('ArrowLeft');
    await expectFocusOn(page, all);
    await expect(page).toHaveURL(/\/admin\/collections$/);

    // ↓/↑ between the rows.
    const toggle = toggleOf(page, created);
    await tabTo(page, toggle, 60);
    await arrowTo(page, toggleOf(page, second.id));
    await expectVisibleFocus(page);
    await arrowTo(page, toggle, 'ArrowUp');

    // Enter folds the row out: the search, then the members.
    await page.keyboard.press('Enter');
    await expect(toggle).toHaveAttribute('aria-expanded', 'true');
    const members = page.getByRole('region', { name: 'Mitglieder von E2E Filmabend' });
    await expect(members.getByText('Noch kein Titel in dieser Sammlung.')).toBeVisible();
    const search = members.getByRole('searchbox', { name: 'Titel suchen' });
    await tabTo(page, search, 10);
    const addTitle = async (query: string, title: string) => {
      await page.keyboard.press('ControlOrMeta+a');
      await page.keyboard.type(query);
      const add = members.getByRole('button', { name: `${title} hinzufügen`, exact: true });
      await expect(add).toBeVisible();
      await tabTo(page, add, 10);
      await page.keyboard.press('Enter');
      // The hit leaves the list: the focus is back in the search field.
      await expectFocusOn(page, search);
    };
    await addTitle('Slouchcat Test Subt', SUBTITLES_TITLE);
    await addTitle('Slouchcat Test PGS', PGS_TITLE);
    const list = members.getByRole('list', { name: 'Titel der Sammlung' });
    await expect(list.getByRole('listitem')).toHaveCount(2);
    await expect(row).toContainText('geändert');

    // The position of PGS: ↑ on its select puts it first; the focus stays on the select.
    const place = list.getByRole('combobox', { name: `Platz für ${PGS_TITLE}` });
    await tabTo(page, place, 20);
    await page.keyboard.press('ArrowUp');
    await expect(list.getByRole('listitem').first()).toContainText(PGS_TITLE);
    await expectFocusOn(page, place);
    await expect(place).toHaveValue('0');

    // Leaving with unsaved members asks; «Auf der Seite bleiben» (the default) keeps them.
    // The sidebar is one Tab stop on its current entry; ↑ moves to «Spotlight».
    const sidebar = page.getByRole('navigation', { name: 'Verwaltung' });
    await resetFocus(page);
    await tabTo(page, sidebar.getByRole('link', { name: 'Sammlungen' }));
    await arrowTo(page, sidebar.getByRole('link', { name: 'Spotlight' }), 'ArrowUp');
    await expectVisibleFocus(page);
    await page.keyboard.press('Enter');
    const leave = page.getByRole('alertdialog', {
      name: 'Seite mit ungespeicherten Änderungen verlassen?',
    });
    await expectFocusOn(page, leave.getByRole('button', { name: 'Auf der Seite bleiben' }));
    await page.keyboard.press('Enter');
    await expect(leave).toHaveCount(0);
    await expect(heading).toBeVisible();
    await expect(list.getByRole('listitem').first()).toContainText(PGS_TITLE);

    // «Speichern».
    const saved = page.waitForResponse(
      (response) => response.request().method() === 'PUT' && response.url().endsWith('/items'),
    );
    await tabTo(page, members.getByRole('button', { name: 'Speichern' }), 80);
    await page.keyboard.press('Enter');
    expect((await saved).status()).toBe(200);
    await expect(
      page.getByText('Die Mitglieder von E2E Filmabend sind gespeichert.'),
    ).toBeVisible();
    await expect(row.getByRole('cell').filter({ hasText: /^2$/ })).toBeVisible();
    await expect(row).not.toContainText('geändert');
    await expectVisibleFocus(page);

    // The start page row «Sammlungen» links to the collection's page.
    await page.goto('/');
    const card = page
      .getByRole('region', { name: 'Sammlungen' })
      .getByRole('link', { name: /E2E Filmabend/ });
    await expect(card).toBeVisible();
    await resetFocus(page);
    await tabTo(page, card, 80);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/collections\/[0-9A-Z]{26}$/);
    await expect(page.getByRole('heading', { level: 1, name: 'E2E Filmabend' })).toBeVisible();
    await page.goto('/admin/collections');

    // Escape leaves the rename dialog unchanged; the focus returns to its button.
    await resetFocus(page);
    await tabTo(page, toggle, 60);
    await page.keyboard.press('Enter');
    await expect(toggle).toHaveAttribute('aria-expanded', 'true');
    const rename = members.getByRole('button', { name: 'E2E Filmabend umbenennen' });
    await tabTo(page, rename, 20);
    await page.keyboard.press('Enter');
    const renameDialog = page.getByRole('dialog', { name: 'E2E Filmabend umbenennen' });
    const field = renameDialog.getByRole('textbox', { name: 'Name' });
    await expect(field).toHaveValue('E2E Filmabend');
    await page.keyboard.press('Escape');
    await expect(renameDialog).toHaveCount(0);
    await expectFocusOn(page, rename);

    // «Umbenennen».
    await page.keyboard.press('Enter');
    await expectFocusOn(page, field);
    await page.keyboard.press('ControlOrMeta+a');
    await page.keyboard.type('E2E Kinoabend');
    await page.keyboard.press('Enter');
    await expect(renameDialog).toHaveCount(0);
    await expect(page.getByText('Die Sammlung heisst jetzt E2E Kinoabend.')).toBeVisible();
    await expect(row.getByRole('rowheader')).toHaveText('E2E Kinoabend');

    // «Löschen» with its confirmation; the row goes, the focus to the heading.
    const renamed = page.getByRole('region', { name: 'Mitglieder von E2E Kinoabend' });
    await tabTo(page, renamed.getByRole('button', { name: 'Sammlung E2E Kinoabend löschen' }), 20);
    await page.keyboard.press('Enter');
    const confirm = page.getByRole('alertdialog', { name: 'Sammlung E2E Kinoabend löschen?' });
    await expectFocusOn(page, confirm.getByRole('button', { name: 'Sammlung löschen' }));
    await page.keyboard.press('Enter');
    await expect(confirm).toHaveCount(0);
    await expect(row).toHaveCount(0);
    created = undefined;
    await expect(page.getByText('Die Sammlung E2E Kinoabend ist gelöscht.')).toBeVisible();
    await expect(heading).toBeFocused();
    await expectNoFocusTrap(page);
  } finally {
    if (created) await deleteCollection(owner, created);
    await deleteCollection(owner, second.id);
  }
});
