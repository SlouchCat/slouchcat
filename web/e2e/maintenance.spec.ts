import type { APIRequestContext, Page } from '@playwright/test';
import { BROKEN_TITLE, CSRF, H264_COPY_TITLE, UNRATED_TITLE } from './catalog';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, expectVisibleFocus, tabTo } from './keyboard';

// B-9 maintenance: each prepared case of the test media shows up in its list (unplayable file,
// duplicate by file hash, title without rating), and the actions work by keyboard.

interface IssueData {
  itemId: string;
  title: string;
  mediaFileId?: string;
}

async function issues(owner: APIRequestContext, kind: string): Promise<IssueData[]> {
  const response = await owner.get(`/api/v1/admin/maintenance/issues?kind=${kind}&limit=200`);
  expect(response.status(), await response.text()).toBe(200);
  return ((await response.json()) as { items: IssueData[] }).items;
}

const tab = (page: Page, name: RegExp) => page.getByRole('tab', { name });
const panel = (page: Page) => page.getByRole('tabpanel');

test.beforeEach(async ({ page }) => {
  await page.goto('/admin/maintenance');
  await expect(page.getByRole('heading', { level: 1, name: 'Pflege' })).toBeVisible();
  await expect(page.getByRole('tablist', { name: 'Problemarten' })).toBeVisible();
});

test('every issue kind has a tab with its count; arrows move between the tabs', async ({
  page,
}) => {
  for (const name of [
    'Nicht zugeordnet',
    'Ohne Bilder',
    'Ohne Freigabe',
    'Verwaist',
    'Nicht abspielbar',
    'Freigabe abweichend',
    'Duplikate',
  ]) {
    await expect(tab(page, new RegExp(`^${name} \\(\\d+\\)$`))).toBeVisible();
  }

  await tabTo(page, tab(page, /^Nicht zugeordnet/));
  await expect(tab(page, /^Nicht zugeordnet/)).toHaveAttribute('aria-selected', 'true');
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, tab(page, /^Ohne Bilder/));
  await expect(tab(page, /^Ohne Bilder/)).toHaveAttribute('aria-selected', 'true');
  await expect(panel(page).getByText(/Zugeordnete Titel ohne Poster/)).toBeVisible();
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, tab(page, /^Nicht abspielbar/));
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, tab(page, /^Verwaist/));
  await expectNoFocusTrap(page);
});

test('duplicates list the H264 copy (same file hash)', async ({ page }) => {
  await tab(page, /^Duplikate/).click();
  const group = panel(page)
    .getByRole('article')
    .filter({ has: page.getByRole('link', { name: new RegExp(`^${H264_COPY_TITLE}`) }) });
  await expect(group).toHaveCount(1);
  await expect(group.getByText('Gleicher Dateiinhalt')).toBeVisible();
  await expect(group.getByRole('radio')).toHaveCount(2);
});

test('unrated: select by keyboard, assign a rating, the title leaves the list', async ({
  page,
  owner,
}) => {
  const unrated = (await issues(owner, 'unrated')).find((issue) => issue.title === UNRATED_TITLE);
  expect(unrated, `${UNRATED_TITLE} is unrated`).toBeDefined();
  try {
    await tab(page, /^Ohne Freigabe/).click();
    const box = panel(page).getByRole('checkbox', { name: `${UNRATED_TITLE} auswählen` });
    await expect(box).toBeVisible();
    await box.focus();
    await page.keyboard.press('Space');
    await expect(box).toBeChecked();
    await expectVisibleFocus(page);

    const assign = panel(page).getByRole('button', { name: 'Freigabe zuweisen' });
    await assign.focus();
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', { name: 'Freigabe zuweisen' });
    await expect(dialog).toBeVisible();
    await expectFocusOn(page, dialog.getByRole('radio', { name: 'FSK 0' }));
    await page.keyboard.press('ArrowRight');
    await page.keyboard.press('ArrowRight');
    await expect(dialog.getByRole('radio', { name: 'FSK 12' })).toBeChecked();
    await page.keyboard.press('Tab');
    await page.keyboard.press('Tab');
    await expectFocusOn(page, dialog.getByRole('button', { name: 'Zuweisen' }));
    await page.keyboard.press('Enter');
    await expect(dialog).toHaveCount(0);
    await expect(panel(page).getByText('Freigabe für 1 Titel gesetzt.')).toBeVisible();
    await expect(panel(page).getByRole('link', { name: UNRATED_TITLE })).toHaveCount(0);
  } finally {
    // Back to unrated for the next browser's run.
    if (unrated) {
      await owner.put(`/api/v1/admin/items/${unrated.itemId}/rating`, {
        headers: CSRF,
        data: { rating: null },
      });
    }
  }
});

test('unplayable lists the broken file; "Entfernen" removes it after confirmation', async ({
  page,
  owner,
}) => {
  // The probe of the broken file runs after the scan (long done when this spec runs). The run of
  // the other browser may have removed it already: then only the outcome is checked (the file is
  // on the ignore list, no scan brings it back).
  let before: IssueData | undefined;
  for (let attempt = 0; attempt < 10 && !before; attempt += 1) {
    before = (await issues(owner, 'unplayable')).find((issue) => issue.title === BROKEN_TITLE);
    if (!before) await page.waitForTimeout(1000);
  }

  await tab(page, /^Nicht abspielbar/).click();
  const link = panel(page).getByRole('link', { name: new RegExp(`^${BROKEN_TITLE}`) });
  if (before) {
    await expect(link).toBeVisible();
    await expect(panel(page).getByText(/\.mkv$/)).toBeVisible();
    const box = panel(page).getByRole('checkbox', { name: `${BROKEN_TITLE} auswählen` });
    await box.focus();
    await page.keyboard.press('Space');
    await expect(box).toBeChecked();
    const remove = panel(page).getByRole('button', { name: 'Entfernen' });
    await remove.focus();
    await page.keyboard.press('Enter');
    const confirm = page.getByRole('alertdialog', { name: 'Dateien entfernen?' });
    await expect(confirm).toBeVisible();
    await expectFocusOn(page, confirm.getByRole('button', { name: 'Entfernen' }));
    await page.keyboard.press('Enter');
    await expect(confirm).toHaveCount(0);
    await expect(panel(page).getByText(/^Entfernt: \d+ Titel, 1 Dateien\.$/)).toBeVisible();
  }
  await expect(link).toHaveCount(0);
  expect((await issues(owner, 'unplayable')).some((issue) => issue.title === BROKEN_TITLE)).toBe(
    false,
  );
});

test('a title links to its administration page', async ({ page }) => {
  await tab(page, /^Nicht zugeordnet/).click();
  const first = panel(page).getByRole('link').first();
  await expect(first).toBeVisible();
  const href = await first.getAttribute('href');
  expect(href).toMatch(/^\/admin\/items\//);
  await first.focus();
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(new RegExp(`${href ?? ''}$`));
  await expect(page.getByRole('region', { name: 'Dateien' })).toBeVisible();
});
