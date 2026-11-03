import { expect, test, type Page } from '@playwright/test';

// Behaviour of the administration's building blocks (concept 12, 11.2, K12-2b) in a real
// browser, beyond the screenshots of components.spec.ts.
//
// Rows of a DataTable that fold out, checked the way a screen reader meets them: the table keeps
// its column headers, row headers and cells; the first cell holds a button named by the row with
// aria-expanded and aria-controls; the fold-out is a row of its own below, in the tree only while
// it is open. Besides Playwright's aria snapshot the tests read Chromium's own accessibility tree
// over CDP, which is what a screen reader is given; no screen reader ran (a proxy).

const STORY = '/iframe.html?id=ui-datatable--expandable&viewMode=story';

/** Chromium's accessibility node of the button named `name`. */
async function axButton(page: Page, name: string) {
  const client = await page.context().newCDPSession(page);
  const { nodes } = (await client.send('Accessibility.getFullAXTree')) as {
    nodes: {
      role?: { value: string };
      name?: { value: string };
      properties?: { name: string; value: { value: unknown } }[];
    }[];
  };
  const node = nodes.find((n) => n.role?.value === 'button' && n.name?.value === name);
  const expanded = node?.properties?.find((p) => p.name === 'expanded')?.value.value;
  return { role: node?.role?.value, name: node?.name?.value, expanded };
}

test('a closed row: table, row header with a collapsed button, no fold-out', async ({ page }) => {
  await page.goto(STORY);
  const table = page.getByRole('table', { name: 'Laufende Streams' });
  await expect(table.getByRole('row')).toHaveCount(3);
  await expect(table).toMatchAriaSnapshot(`
    - table "Laufende Streams":
      - caption: Laufende Streams
      - rowgroup:
        - row:
          - columnheader "Profil"
          - columnheader "Gerät"
          - columnheader "Titel"
          - columnheader "Qualität"
          - columnheader "Modus"
          - columnheader "Aktionen"
      - rowgroup:
        - row:
          - rowheader "Marco":
            - button "Marco"
          - cell "Chrome 156 · Linux"
          - cell
          - cell
          - cell
          - cell
        - row:
          - rowheader "Lena":
            - button "Lena"
          - cell
          - cell
          - cell
          - cell
          - cell
  `);
  await expect(table.getByRole('button', { name: 'Marco' })).toHaveAttribute(
    'aria-expanded',
    'false',
  );
  expect(await axButton(page, 'Marco')).toEqual({
    role: 'button',
    name: 'Marco',
    expanded: false,
  });
});

test('an open row: the button is expanded and controls the fold-out row', async ({ page }) => {
  await page.goto(STORY);
  const table = page.getByRole('table', { name: 'Laufende Streams' });
  const button = table.getByRole('button', { name: 'Marco' });
  await expect(button).toBeVisible();
  await page.keyboard.press('Tab');
  await page.keyboard.press('Enter');
  await expect(button).toBeFocused();
  await expect(table.getByRole('row')).toHaveCount(4);
  await expect(table).toMatchAriaSnapshot(`
    - table "Laufende Streams":
      - rowgroup:
        - row:
          - rowheader "Marco":
            - button "Marco" [expanded]
          - cell
          - cell
          - cell
          - cell
          - cell
        - row:
          - cell:
            - paragraph: 'Slouchcat Test Multi Audio: 1080p · 8 Mbit/s, Transcode (HW), seit 12 Minuten.'
            - combobox "Qualität begrenzen"
        - row:
          - rowheader "Lena":
            - button "Lena"
  `);
  const controls = await button.getAttribute('aria-controls');
  await expect(page.locator(`[id="${controls}"]`)).toContainText(
    'Slouchcat Test Multi Audio: 1080p · 8 Mbit/s, Transcode (HW), seit 12 Minuten.',
  );
  expect(await axButton(page, 'Marco')).toEqual({ role: 'button', name: 'Marco', expanded: true });
  await page.keyboard.press('Space');
  expect(await axButton(page, 'Marco')).toEqual({
    role: 'button',
    name: 'Marco',
    expanded: false,
  });
  await expect(table.getByRole('row')).toHaveCount(3);
});

test('↑ and ↓ stay with a select inside a fold-out (they change its value)', async ({ page }) => {
  await page.goto(STORY);
  const button = page.getByRole('button', { name: 'Marco' });
  await expect(button).toBeVisible();
  await button.click();
  const select = page.getByRole('combobox', { name: 'Qualität begrenzen' });
  await select.focus();
  await expect(select).toHaveValue('1080p');
  await page.keyboard.press('ArrowDown');
  await expect(select).toBeFocused();
  await expect(select).toHaveValue('Original');
  await page.keyboard.press('ArrowUp');
  await expect(select).toBeFocused();
  await expect(select).toHaveValue('1080p');
});

test('a select of RankSelect keeps the focus when its entry moves down', async ({ page }) => {
  await page.goto('/iframe.html?id=ui-rankselect--move&viewMode=story');
  const deutsch = page.getByRole('combobox', { name: 'Platz für Deutsch' });
  await expect(deutsch).toBeVisible();
  await page.keyboard.press('Tab');
  await expect(deutsch).toBeFocused();
  await page.keyboard.press('ArrowDown');
  await expect(page.getByRole('listitem').first()).toContainText('Englisch');
  await expect(deutsch).toHaveValue('1');
  await expect(deutsch).toBeFocused();
  await page.keyboard.press('ArrowDown');
  await expect(deutsch).toHaveValue('2');
  await expect(deutsch).toBeFocused();
});

/** The value of a component token on the element, as the cascade resolves it. */
const token = (page: Page, selector: string, name: string) =>
  page
    .locator(selector)
    .first()
    .evaluate(
      (element, property) => getComputedStyle(element).getPropertyValue(property).trim(),
      name,
    );

test('the compact density holds only under the admin shell, dialogs included', async ({ page }) => {
  await page.goto('/iframe.html?id=ui-confirmdialog--controlled-compact&viewMode=story');
  await expect(page.getByRole('alertdialog')).toBeVisible();
  const overlay = '[data-density="compact"]:has([role="alertdialog"])';
  // /admin: <html data-shell="admin">, the dialog's portal carries data-density="compact".
  expect(await token(page, overlay, '--sc-control-height')).toBe('2rem');
  expect(await token(page, overlay, '--sc-dialog-gap')).toBe('12px');
  // /settings/*: the same AdminPage and dialog without the admin shell keep today's size.
  await page.evaluate(() => delete document.documentElement.dataset.shell);
  expect(await token(page, overlay, '--sc-control-height')).toBe('2.75rem');
  expect(await token(page, overlay, '--sc-dialog-gap')).toBe('16px');
});
