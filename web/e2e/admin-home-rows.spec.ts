import type { Page } from '@playwright/test';
import { CSRF } from './catalog';
import { DEFAULT_HOME_ROWS, setHomeRows, titleId } from './curation';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';
import { arrowTo, resetProgress } from './playback';

// /admin/home-rows (M7, E-9; concept 12, variant C) by keyboard (rule 8): the position of
// "Neu hinzugefügt" by ↑ on its select changes the start page's order, a hidden row is gone from
// it; a row folds out by Enter with its description; the tabs «Reihenfolge» and «Vorschau» by
// ←/→; leaving with unsaved changes asks; the Trakt line leads to the spotlight page. ↑/↓
// between the rows, Tab, Enter and Space, browser back, no trap.

/** A film the main profile has started, so its "Weiterschauen" row exists. */
const STARTED_TITLE = 'Slouchcat Test Sidecar Subtitles';

/** Headings of the start page's rows (and the hero), top to bottom. */
async function rowOrder(page: Page): Promise<string[]> {
  await page.goto('/');
  await expect(page.getByRole('region', { name: 'Neu hinzugefügt' })).toBeVisible();
  return page
    .locator('main section[aria-labelledby]')
    .evaluateAll((sections) =>
      sections.map(
        (section) =>
          document.getElementById(section.getAttribute('aria-labelledby') ?? '')?.textContent ?? '',
      ),
    );
}

const rowsTable = (page: Page) => page.getByRole('table', { name: 'Reihen der Startseite' });
const saveBar = (page: Page) => page.getByRole('region', { name: 'Speichern' });
/** The status region of the save area (the column around the save bar). */
const saveStatus = (page: Page) => saveBar(page).locator('xpath=..').getByRole('status');

test('start page rows by keyboard: placing and hiding change the start page', async ({
  page,
  owner,
}) => {
  test.setTimeout(90_000);
  // The stored order starts as the default and goes back to it, whatever ran before or fails.
  await setHomeRows(owner, DEFAULT_HOME_ROWS);
  const started = await titleId(owner, STARTED_TITLE);
  const progress = await owner.post(`/api/v1/items/${started}/progress`, {
    headers: CSRF,
    data: {
      positionMs: 30_000,
      durationMs: 60_000,
      playedMs: 30_000,
      seq: 1,
      state: 'paused',
      clientTime: new Date().toISOString(),
    },
  });
  expect(progress.ok(), await progress.text()).toBe(true);
  try {
    const before = await rowOrder(page);
    expect(before.indexOf('Weiterschauen')).toBeGreaterThanOrEqual(0);
    expect(before.indexOf('Weiterschauen')).toBeLessThan(before.indexOf('Neu hinzugefügt'));

    await page.goto('/admin/home-rows');
    await expect(page.getByRole('heading', { level: 1, name: 'Startseite' })).toBeVisible();
    await expect(
      page
        .getByRole('navigation', { name: 'Verwaltung' })
        .getByRole('link', { name: 'Startseite' }),
    ).toHaveAttribute('aria-current', 'page');
    const table = rowsTable(page);
    await expect(table.getByRole('rowheader')).toHaveCount(8);
    // No arrow buttons any more (S-16): a position to choose per row.
    await expect(table.getByRole('button', { name: /nach oben|nach unten/ })).toHaveCount(0);

    // ↑ on the position of "Neu hinzugefügt": it moves one up, the select keeps the focus.
    // Spotlight holds row 0 and picks row 1 (ADR 0011), so recent moves from 3 to 2.
    await resetFocus(page);
    const rank = table.getByRole('combobox', { name: 'Platz für Neu hinzugefügt' });
    await tabTo(page, rank, 60);
    await expect(rank).toHaveAccessibleDescription('Die anderen Einträge rücken nach.');
    await page.keyboard.press('ArrowUp');
    await expect(table.getByRole('rowheader').nth(2)).toHaveText('Neu hinzugefügt');
    await expect(table.getByRole('rowheader').nth(3)).toHaveText('Weiterschauen');
    await expectFocusOn(page, rank);
    await expect(saveStatus(page)).toHaveText(/Neu hinzugefügt steht jetzt auf Platz 3/);
    await expect(saveBar(page).getByText('Ungespeicherte Änderungen')).toBeVisible();

    // Tab to the row's name: Enter folds it out with what the row shows, Enter folds it away.
    const recent = table.getByRole('button', { name: 'Neu hinzugefügt' });
    await page.keyboard.press('Tab');
    await expectFocusOn(page, recent);
    await page.keyboard.press('Enter');
    await expect(recent).toHaveAttribute('aria-expanded', 'true');
    await expect(table.getByText('Die zuletzt in eine Bibliothek gekommenen Titel.')).toBeVisible();
    await page.keyboard.press('Enter');
    await expect(recent).toHaveAttribute('aria-expanded', 'false');
    // ↓ to the next row ("Weiterschauen" now), ↑ back.
    await arrowTo(page, table.getByRole('button', { name: 'Weiterschauen' }));
    await expectVisibleFocus(page);
    await arrowTo(page, recent, 'ArrowUp');

    const save = saveBar(page).getByRole('button', { name: 'Speichern' });
    const stored = () =>
      page.waitForResponse(
        (response) =>
          response.request().method() === 'PUT' && response.url().endsWith('/admin/home-rows'),
      );
    await tabTo(page, save, 40);
    let saved = stored();
    await page.keyboard.press('Enter');
    expect((await saved).status()).toBe(200);
    await expect(page.getByText('Die Startseite ist gespeichert.')).toBeVisible();
    // «Speichern» stays usable (decision 18): the focus stays on it.
    await expect(save).toBeFocused();

    const after = await rowOrder(page);
    expect(after.indexOf('Neu hinzugefügt')).toBeLessThan(after.indexOf('Weiterschauen'));
    await page.goBack();
    await expect(page).toHaveURL(/\/admin\/home-rows$/);

    // "Weiterschauen ausblenden" with Space: gone from the start page.
    const hide = table.getByRole('checkbox', { name: 'Weiterschauen ausblenden' });
    await resetFocus(page);
    await tabTo(page, hide, 60);
    await page.keyboard.press('Space');
    await expect(hide).toBeChecked();
    await tabTo(page, save, 40);
    saved = stored();
    await page.keyboard.press('Enter');
    expect((await saved).status()).toBe(200);
    expect(await rowOrder(page)).not.toContain('Weiterschauen');
    await expect(page.getByRole('region', { name: 'Weiterschauen' })).toHaveCount(0);
  } finally {
    await resetProgress(owner, started);
    await setHomeRows(owner, DEFAULT_HOME_ROWS);
  }
});

test('the preview by ←/→ shows the unsaved order; leaving asks, «Verwerfen» restores', async ({
  page,
}) => {
  await page.goto('/admin/home-rows');
  const table = rowsTable(page);
  await expect(table.getByRole('rowheader')).toHaveCount(8);
  const first = await table.getByRole('rowheader').first().textContent();
  const last = (await table.getByRole('rowheader').last().textContent()) ?? '';

  // ↑ on the last row's position: it moves one up, unsaved.
  await resetFocus(page);
  const rank = table.getByRole('combobox', { name: `Platz für ${last}` });
  await tabTo(page, rank, 80);
  await page.keyboard.press('ArrowUp');
  await expect(table.getByRole('rowheader').nth(6)).toHaveText(last);
  await expectFocusOn(page, rank);

  // ←/→ in the tab list: «Vorschau» shows the order on the page, saved or not.
  const order = page.getByRole('tab', { name: /^Reihenfolge/ });
  const preview = page.getByRole('tab', { name: /^Vorschau/ });
  await resetFocus(page);
  await tabTo(page, order, 60);
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, preview);
  await expect(preview).toHaveAttribute('aria-selected', 'true');
  await expect(page).toHaveURL(/[?&]tab=preview/);
  const sketch = page.getByRole('list', { name: 'Vorschau der Startseite' });
  await expect(sketch.getByRole('listitem').nth(6)).toContainText(`7. ${last}`);
  await expect(page.getByRole('alertdialog')).toHaveCount(0);
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, order);
  await expect(page).toHaveURL(/\/admin\/home-rows$/);
  await expect(table.getByRole('rowheader').nth(6)).toHaveText(last);

  // The sidebar link waits for the question; it starts on «Auf der Seite bleiben», Escape stays.
  // Tab reaches the sidebar at its current entry, ↑ walks it to «Spotlight».
  const sidebar = page.getByRole('navigation', { name: 'Verwaltung' });
  const spotlight = sidebar.getByRole('link', { name: 'Spotlight' });
  const toSpotlight = async () => {
    await resetFocus(page);
    await tabTo(page, sidebar.getByRole('link', { name: 'Startseite' }), 40);
    await arrowTo(page, spotlight, 'ArrowUp');
    await expectVisibleFocus(page);
  };
  await toSpotlight();
  await page.keyboard.press('Enter');
  const leave = page.getByRole('alertdialog', {
    name: 'Seite mit ungespeicherten Änderungen verlassen?',
  });
  await expectFocusOn(page, leave.getByRole('button', { name: 'Auf der Seite bleiben' }));
  await page.keyboard.press('Escape');
  await expect(leave).toBeHidden();
  await expect(page).toHaveURL(/\/admin\/home-rows$/);
  await expect(saveBar(page).getByText('Ungespeicherte Änderungen')).toBeVisible();
  // With unsaved changes the page holds no focus trap either.
  await expectNoFocusTrap(page, 120);

  // «Verwerfen» asks, starting on «Abbrechen»: Enter there keeps the changes and gives the
  // focus back to «Verwerfen»; confirmed, the saved order is back.
  const discardButton = saveBar(page).getByRole('button', { name: 'Verwerfen' });
  await resetFocus(page);
  await tabTo(page, discardButton, 80);
  await page.keyboard.press('Enter');
  const discard = page.getByRole('alertdialog', { name: 'Änderungen verwerfen?' });
  await expectFocusOn(page, discard.getByRole('button', { name: 'Abbrechen' }));
  await page.keyboard.press('Enter');
  await expect(discard).toBeHidden();
  await expectFocusOn(page, discardButton);
  await expect(table.getByRole('rowheader').nth(6)).toHaveText(last);
  // The question let the focus go: Tab runs on to the end of the page; back to «Verwerfen».
  await expectNoFocusTrap(page, 120);
  await resetFocus(page);
  await tabTo(page, discardButton, 80);
  await page.keyboard.press('Enter');
  await expectFocusOn(page, discard.getByRole('button', { name: 'Abbrechen' }));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, discard.getByRole('button', { name: 'Verwerfen' }));
  await page.keyboard.press('Enter');
  await expect(discard).toBeHidden();
  await expect(saveBar(page).getByText('Gespeichert', { exact: true })).toBeVisible();
  await expect(saveStatus(page)).toHaveText('Die Änderungen sind verworfen.');
  await expect(page.getByRole('heading', { level: 1, name: 'Startseite' })).toBeFocused();
  await expect(table.getByRole('rowheader').first()).toHaveText(first ?? '');
  await expect(table.getByRole('rowheader').last()).toHaveText(last);

  // Nothing unsaved: the sidebar link goes at once.
  await toSpotlight();
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin\/spotlight$/);
});

test('the Trakt line links to the spotlight page by keyboard', async ({ page }) => {
  await page.goto('/admin/home-rows');
  await expect(rowsTable(page)).toBeVisible();
  // The Trakt state lives on the spotlight page only (M11 step 6): no section, no refresh here.
  await expect(page.getByRole('region', { name: 'Trakt' })).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Jetzt aktualisieren' })).toHaveCount(0);

  const link = page.getByRole('link', { name: 'Zur Seite «Spotlight»' });
  await resetFocus(page);
  await tabTo(page, link, 80);
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin\/spotlight$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Spotlight' })).toBeVisible();

  await page.goBack();
  await expect(page).toHaveURL(/\/admin\/home-rows$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Startseite' })).toBeVisible();
  await expectNoFocusTrap(page);
});

/** The left edge of each column header of the rows table, in its order. */
const headerLefts = (page: Page) =>
  rowsTable(page)
    .getByRole('columnheader')
    .evaluateAll((cells) => cells.map((cell) => cell.getBoundingClientRect().left));

test('ticking «Ausblenden» leaves the columns where they are (K12-9)', async ({ page, owner }) => {
  await setHomeRows(owner, DEFAULT_HOME_ROWS);
  await page.goto('/admin/home-rows');
  const table = rowsTable(page);
  await expect(table.getByRole('rowheader')).toHaveCount(8);
  const before = await headerLefts(page);

  // The state of the row changes from «sichtbar» to «ausgeblendet», unsaved.
  const hide = table.getByRole('checkbox', { name: 'Neu hinzugefügt ausblenden' });
  await expect(hide).not.toBeChecked();
  await hide.check();
  await expect(saveBar(page).getByText('Ungespeicherte Änderungen')).toBeVisible();
  const after = await headerLefts(page);
  expect(after).toHaveLength(before.length);
  before.forEach((left, column) =>
    expect(Math.abs(left - after[column]), `column ${column}`).toBeLessThanOrEqual(1),
  );

  // Nothing is saved: «Verwerfen», confirmed, brings the stored state back.
  await saveBar(page).getByRole('button', { name: 'Verwerfen' }).click();
  await page
    .getByRole('alertdialog', { name: 'Änderungen verwerfen?' })
    .getByRole('button', { name: 'Verwerfen' })
    .click();
  await expect(saveBar(page).getByText('Gespeichert', { exact: true })).toBeVisible();
  await expect(hide).not.toBeChecked();
});
