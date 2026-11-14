import type { Page } from '@playwright/test';
import { listMyMedia, setMyMedia } from './curation';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';
import { arrowTo } from './playback';

// /admin/my-media (concept 21 P-11, variant A; O-20, O-21) by keyboard (rule 8): ↑/↓ on an
// entry's position moves it, Space on «Ausblenden» hides it in the side navigation or on the start
// page alone, and the saved order reaches the side navigation's group (GET /my-media); the tabs
// «Reihenfolge» and «Vorschau» by ←/→; leaving with unsaved changes asks, «Verwerfen» restores;
// the line under the band leads to «Startseite». Tab, Enter and Space, browser back, no trap.

const entriesTable = (page: Page) =>
  page.getByRole('table', { name: 'Einträge von «Meine Medien»' });
const saveBar = (page: Page) => page.getByRole('region', { name: 'Speichern' });
/** The status region of the save area (the column around the save bar). */
const saveStatus = (page: Page) => saveBar(page).locator('xpath=..').getByRole('status');
/** The names in the table, without the kind's chip. */
const names = (page: Page) =>
  entriesTable(page)
    .getByRole('rowheader')
    .evaluateAll((cells) =>
      cells.map((cell) => cell.querySelector('span')?.firstChild?.textContent?.trim() ?? ''),
    );

test('«Meine Medien» by keyboard: placing and hiding per place reach the side navigation @keyboard', async ({
  page,
  owner,
}) => {
  const before = await listMyMedia(owner);
  try {
    await page.goto('/admin/my-media');
    await expect(page.getByRole('heading', { level: 1, name: 'Meine Medien' })).toBeVisible();
    const sidebar = page.getByRole('navigation', { name: 'Verwaltung' });
    await expect(sidebar.getByRole('link', { name: 'Meine Medien' })).toHaveAttribute(
      'aria-current',
      'page',
    );
    const table = entriesTable(page);
    await expect(table.getByRole('rowheader')).toHaveCount(before.length);
    const shown = await names(page);
    expect(shown).toContain('Radio');
    expect(shown).toContain('Sammlungen');
    expect(shown).toContain('User-Empfehlungen');
    const radioAt = shown.indexOf('Radio');
    const below = shown[radioAt + 1] ?? shown[radioAt - 1];

    // ↓ (or ↑ at the end) on the position of «Radio»: it trades places, the select keeps the focus.
    await resetFocus(page);
    const rank = table.getByRole('combobox', { name: 'Platz für Radio' });
    await tabTo(page, rank, 60);
    await expect(rank).toHaveAccessibleDescription('Die anderen Einträge rücken nach.');
    const down = radioAt + 1 < shown.length;
    await page.keyboard.press(down ? 'ArrowDown' : 'ArrowUp');
    const moved = down ? radioAt + 1 : radioAt - 1;
    await expect(table.getByRole('rowheader').nth(moved)).toContainText('Radio');
    await expectFocusOn(page, rank);
    await expect(saveStatus(page)).toHaveText(
      new RegExp(`Radio steht jetzt auf Platz ${moved + 1}`),
    );
    expect((await names(page))[radioAt]).toBe(below);

    // Tab on to the boxes of «Radio»: Space hides it in the side navigation only (O-21).
    const hideSidebar = table.getByRole('checkbox', {
      name: 'Radio in der Seitenleiste ausblenden',
    });
    await tabTo(page, hideSidebar, 6);
    await expectVisibleFocus(page);
    await page.keyboard.press('Space');
    await expect(hideSidebar).toBeChecked();
    const radioRow = table
      .getByRole('row')
      .filter({ has: page.getByRole('combobox', { name: 'Platz für Radio' }) });
    await expect(radioRow).toContainText('nur Startseite');
    await expect(
      table.getByRole('checkbox', { name: 'Radio auf der Startseite ausblenden' }),
    ).not.toBeChecked();

    const save = saveBar(page).getByRole('button', { name: 'Speichern' });
    await tabTo(page, save, 80);
    const stored = page.waitForResponse(
      (response) =>
        response.request().method() === 'PUT' && response.url().endsWith('/admin/my-media'),
    );
    await page.keyboard.press('Enter');
    expect((await stored).status()).toBe(200);
    await expect(page.getByText('«Meine Medien» ist gespeichert.')).toBeVisible();
    await expect(save).toBeFocused();

    // The side navigation's group follows: Radio is gone there, the rest stands in the new order.
    const nav = (await (await owner.get('/api/v1/my-media')).json()) as { id: string }[];
    expect(nav.map((entry) => entry.id)).not.toContain('radio');
    const after = await listMyMedia(owner);
    expect(after.find((entry) => entry.id === 'radio')).toMatchObject({
      position: moved,
      hideSidebar: true,
      hideHome: false,
    });
  } finally {
    await setMyMedia(owner, before);
  }
});

test('«Meine Medien»: the preview by ←/→ shows the draft; leaving asks, «Verwerfen» restores', async ({
  page,
  owner,
}) => {
  const before = await listMyMedia(owner);
  await page.goto('/admin/my-media');
  const table = entriesTable(page);
  await expect(table.getByRole('rowheader')).toHaveCount(before.length);
  const first = (await names(page))[0];

  // Space on «Ausblenden» of the first entry on the start page: unsaved.
  await resetFocus(page);
  const hideHome = table.getByRole('checkbox', {
    name: `${first} auf der Startseite ausblenden`,
  });
  const wasHidden = await hideHome.isChecked();
  await tabTo(page, hideHome, 60);
  await page.keyboard.press('Space');
  await expect(hideHome).toBeChecked({ checked: !wasHidden });
  await expect(saveBar(page).getByText('Ungespeicherte Änderungen')).toBeVisible();

  // ←/→ in the tab list: «Vorschau» shows both places with the draft.
  const order = page.getByRole('tab', { name: /^Reihenfolge/ });
  const preview = page.getByRole('tab', { name: /^Vorschau/ });
  await resetFocus(page);
  await tabTo(page, order, 60);
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, preview);
  await expect(preview).toHaveAttribute('aria-selected', 'true');
  await expect(page).toHaveURL(/[?&]tab=preview/);
  const home = page.getByRole('region', { name: 'Vorschau der Startseite' });
  const homeNames = home.getByRole('listitem');
  if (wasHidden) await expect(homeNames.first()).toContainText(first);
  else await expect(homeNames.filter({ hasText: first })).toHaveCount(0);
  await expect(page.getByRole('region', { name: 'Vorschau der Seitenleiste' })).toBeVisible();
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, order);
  await expect(page).toHaveURL(/\/admin\/my-media$/);

  // The sidebar link waits for the question; Escape stays on the page.
  const sidebar = page.getByRole('navigation', { name: 'Verwaltung' });
  const homeRows = sidebar.getByRole('link', { name: 'Startseite' });
  const toHomeRows = async () => {
    await resetFocus(page);
    await tabTo(page, sidebar.getByRole('link', { name: 'Meine Medien' }), 40);
    await arrowTo(page, homeRows, 'ArrowUp');
    await expectVisibleFocus(page);
  };
  await toHomeRows();
  await page.keyboard.press('Enter');
  const leave = page.getByRole('alertdialog', {
    name: 'Seite mit ungespeicherten Änderungen verlassen?',
  });
  await expectFocusOn(page, leave.getByRole('button', { name: 'Auf der Seite bleiben' }));
  await page.keyboard.press('Escape');
  await expect(leave).toBeHidden();
  await expect(page).toHaveURL(/\/admin\/my-media$/);
  await expectNoFocusTrap(page, 180);

  // «Verwerfen», confirmed: the saved state is back and the h1 takes the focus.
  const discardButton = saveBar(page).getByRole('button', { name: 'Verwerfen' });
  await resetFocus(page);
  await tabTo(page, discardButton, 140);
  await page.keyboard.press('Enter');
  const discard = page.getByRole('alertdialog', { name: 'Änderungen verwerfen?' });
  await expectFocusOn(page, discard.getByRole('button', { name: 'Abbrechen' }));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, discard.getByRole('button', { name: 'Verwerfen' }));
  await page.keyboard.press('Enter');
  await expect(discard).toBeHidden();
  await expect(saveStatus(page)).toHaveText('Die Änderungen sind verworfen.');
  await expect(page.getByRole('heading', { level: 1, name: 'Meine Medien' })).toBeFocused();
  await expect(hideHome).toBeChecked({ checked: wasHidden });

  // The line under the band leads to «Startseite» by Enter; back returns.
  const link = page.getByRole('link', { name: 'Zur Seite «Startseite»' });
  await resetFocus(page);
  await tabTo(page, link, 160);
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin\/home-rows$/);
  await page.goBack();
  await expect(page).toHaveURL(/\/admin\/my-media$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Meine Medien' })).toBeVisible();
  await expectNoFocusTrap(page);
});
