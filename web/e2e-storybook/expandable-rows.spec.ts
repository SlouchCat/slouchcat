import { expect, test, type Page } from '@playwright/test';

// Concept 12, 11.2 (K12-2b): the rows of a DataTable that fold out, checked the way a screen
// reader meets them. The table keeps its column headers, row headers and cells; the first cell
// holds a button named by the row with aria-expanded and aria-controls; the fold-out is a row of
// its own below, in the tree only while it is open. The last test reads Chromium's own
// accessibility tree (CDP), which is what a screen reader is given.

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
          - 'cell "Slouchcat Test Multi Audio: 1080p · 8 Mbit/s, Transcode (HW), seit 12 Minuten."'
        - row:
          - rowheader "Lena":
            - button "Lena"
  `);
  const controls = await button.getAttribute('aria-controls');
  await expect(page.locator(`[id="${controls}"]`)).toHaveText(
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
