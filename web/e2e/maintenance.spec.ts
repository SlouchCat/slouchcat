import type { APIRequestContext, Locator, Page } from '@playwright/test';
import { BROKEN_TITLE, CSRF, H264_COPY_TITLE, UNRATED_TITLE } from './catalog';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// B-9 maintenance (concept 12, variant C): each prepared case of the test media shows up in its
// tab (unplayable file, duplicate by file hash, title without rating), grouped by library, and
// the actions work by keyboard alone.

/** Tab stops from the top of the page into a long list: the sidebar, the tabs, the rows. */
const TAB_MAX = 200;

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

/**
 * Where the broken file stands: listed as unplayable, not probed yet (the title exists, it is
 * unmatched and unrated like every title without TMDB), or gone (removed by the other browser's
 * run, the file is on the ignore list).
 */
async function brokenState(owner: APIRequestContext): Promise<'listed' | 'pending' | 'gone'> {
  if ((await issues(owner, 'unplayable')).some((issue) => issue.title === BROKEN_TITLE)) {
    return 'listed';
  }
  for (const kind of ['unmatched', 'unrated']) {
    if ((await issues(owner, kind)).some((issue) => issue.title === BROKEN_TITLE)) return 'pending';
  }
  return 'gone';
}

/** Presses Shift+Tab until `locator` has focus (the toolbar sits above the tables). */
async function tabBackTo(page: Page, locator: Locator, max = TAB_MAX): Promise<void> {
  for (let presses = 0; presses < max; presses += 1) {
    await page.keyboard.press('Shift+Tab');
    if (await locator.evaluate((element) => element === document.activeElement)) {
      await expectVisibleFocus(page);
      return;
    }
  }
  throw new Error(`Shift+Tab did not reach the element within ${max} presses`);
}

const tab = (page: Page, name: RegExp) => page.getByRole('tab', { name });
const panel = (page: Page) => page.getByRole('tabpanel');
const h1 = (page: Page) => page.getByRole('heading', { level: 1, name: 'Pflege' });

/** Opens a tab by keyboard: Tab into the tab list, then the arrows. */
async function openTab(page: Page, name: RegExp): Promise<void> {
  await tabTo(page, tab(page, /^Nicht zugeordnet/), TAB_MAX);
  for (let presses = 0; presses < 7; presses += 1) {
    if ((await tab(page, name).getAttribute('aria-selected')) === 'true') break;
    await page.keyboard.press('ArrowRight');
  }
  await expectFocusOn(page, tab(page, name));
  await expect(tab(page, name)).toHaveAttribute('aria-selected', 'true');
}

test.beforeEach(async ({ page }) => {
  await page.goto('/admin/maintenance');
  await expect(h1(page)).toBeVisible();
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
  ]) {
    await expect(tab(page, new RegExp(`^${name} \\d+ (Eintrag|Einträge)$`))).toBeVisible();
  }
  await expect(tab(page, /^Duplikate \d+ Gruppen?$/)).toBeVisible();

  await tabTo(page, tab(page, /^Nicht zugeordnet/), TAB_MAX);
  await expect(tab(page, /^Nicht zugeordnet/)).toHaveAttribute('aria-selected', 'true');
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, tab(page, /^Ohne Bilder/));
  await expect(tab(page, /^Ohne Bilder/)).toHaveAttribute('aria-selected', 'true');
  await expect(panel(page).getByText(/Zugeordnete Titel ohne Poster/)).toBeVisible();
  await expect(page).toHaveURL(/\?tab=no_images$/);
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, tab(page, /^Nicht abspielbar/));
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, tab(page, /^Verwaist/));
  await expectNoFocusTrap(page, TAB_MAX);
});

test('a library section folds away with Enter and comes back', async ({ page }) => {
  // Every title of the test media is unmatched (no TMDB key): the first tab has rows.
  const trigger = panel(page).getByRole('heading', { level: 3 }).first().getByRole('button');
  await expect(trigger).toHaveAttribute('aria-expanded', 'true');
  const name = (await trigger.getAttribute('aria-controls')) ?? '';
  const section = page.locator(`[id="${name}"]`);
  await expect(section.getByRole('table')).toBeVisible();

  await tabTo(page, trigger, TAB_MAX);
  await page.keyboard.press('Enter');
  await expect(trigger).toHaveAttribute('aria-expanded', 'false');
  await expect(section.getByRole('table')).toBeHidden();
  await expectFocusOn(page, trigger);
  await page.keyboard.press('Enter');
  await expect(trigger).toHaveAttribute('aria-expanded', 'true');
  await expect(section.getByRole('table')).toBeVisible();

  // Into the table: the checkbox of the first row, ↓ to the next row's checkbox and ↑ back. The
  // first library holds every generated movie, so it has more than one row.
  const boxes = section.getByRole('checkbox');
  expect(await boxes.count()).toBeGreaterThan(1);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, boxes.first());
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, boxes.nth(1));
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, boxes.first());
});

test('duplicates list the H264 copy (same file hash) in a group of its own', async ({
  page,
  owner,
}) => {
  await openTab(page, /^Duplikate/);
  // Only the first group starts open; the others open with Enter on their heading.
  const triggers = panel(page).getByRole('button', { name: /^Gruppe \d+$/ });
  await expect(triggers.first()).toBeVisible();
  const count = await triggers.count();
  for (let index = 0; index < count; index += 1) {
    const trigger = triggers.nth(index);
    if ((await trigger.getAttribute('aria-expanded')) === 'false') {
      await tabTo(page, trigger, TAB_MAX);
      await page.keyboard.press('Enter');
      await expect(trigger).toHaveAttribute('aria-expanded', 'true');
    }
  }

  const group = panel(page)
    .getByRole('group', { name: /^Gruppe \d+$/ })
    .filter({ has: page.getByRole('link', { name: new RegExp(`^${H264_COPY_TITLE}`) }) });
  await expect(group).toHaveCount(1);
  // The reason stands in the heading line, next to the group's button.
  await expect(group.locator('..').getByText(/^Gleicher Dateiinhalt · /)).toBeVisible();
  await expect(group.getByRole('radio')).toHaveCount(2);
  const items = group.getByRole('listitem');
  await expect(items).toHaveCount(2);
  await expect(group.getByText('bleibt')).toHaveCount(1);
  await expect(items.nth(0).getByText('bleibt')).toBeVisible();

  // «Behalten» by keyboard: Tab reaches the chosen radio, ↓ picks the other title, «bleibt»
  // follows the choice.
  const radios = group.getByRole('radio');
  await expect(radios.nth(0)).toBeChecked();
  await resetFocus(page);
  await tabTo(page, radios.nth(0), TAB_MAX);
  await page.keyboard.press('ArrowDown');
  await expect(radios.nth(1)).toBeChecked();
  await expectFocusOn(page, radios.nth(1));
  await expect(items.nth(1).getByText('bleibt')).toBeVisible();
  await expect(items.nth(0).getByText('bleibt')).toHaveCount(0);

  // «Zusammenführen» asks first; Esc closes the question, the focus goes back to the button and
  // nothing is merged.
  const merge = group.getByRole('button', { name: 'Zusammenführen' });
  await expect(merge).toBeEnabled();
  await tabTo(page, merge, 10);
  await page.keyboard.press('Enter');
  const confirm = page.getByRole('alertdialog', { name: 'Titel zusammenführen?' });
  await expect(confirm).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(confirm).toHaveCount(0);
  await expectFocusOn(page, merge);
  await expect(group).toHaveCount(1);
  await expect(radios.nth(1)).toBeChecked();
  await expectNoFocusTrap(page, TAB_MAX);
  const response = await owner.get('/api/v1/admin/maintenance/duplicates');
  expect(response.status(), await response.text()).toBe(200);
  const groups = (await response.json()) as { items: { title: string }[] }[];
  expect(
    groups.filter((entry) => entry.items.some((item) => item.title === H264_COPY_TITLE)),
  ).toHaveLength(1);
});

test('unrated: select by keyboard, assign a rating, the title leaves the list', async ({
  page,
  owner,
}) => {
  const unrated = (await issues(owner, 'unrated')).find((issue) => issue.title === UNRATED_TITLE);
  expect(unrated, `${UNRATED_TITLE} is unrated`).toBeDefined();
  try {
    await openTab(page, /^Ohne Freigabe/);
    const box = panel(page).getByRole('checkbox', { name: `${UNRATED_TITLE} auswählen` });
    await expect(box).toBeVisible();
    await tabTo(page, box, TAB_MAX);
    await page.keyboard.press('Space');
    await expect(box).toBeChecked();
    await expectVisibleFocus(page);

    const assign = panel(page).getByRole('button', { name: 'Freigabe zuweisen' });
    await tabBackTo(page, assign);
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', { name: 'Freigabe zuweisen' });
    await expect(dialog).toBeVisible();
    // No level is chosen up front: FSK 0 would open the title to every profile.
    const submit = dialog.getByRole('button', { name: 'Zuweisen' });
    await expect(dialog.getByRole('radio', { checked: true })).toHaveCount(0);
    await expect(submit).toBeDisabled();
    await expectFocusOn(page, dialog.getByRole('radio', { name: 'FSK 0' }));
    await page.keyboard.press('ArrowRight');
    await page.keyboard.press('ArrowRight');
    await expect(dialog.getByRole('radio', { name: 'FSK 12' })).toBeChecked();
    await expect(dialog.getByText('1 Titel erhält FSK 12.')).toBeVisible();
    await page.keyboard.press('Tab');
    await page.keyboard.press('Tab');
    await expectFocusOn(page, submit);
    await page.keyboard.press('Enter');
    await expect(dialog).toHaveCount(0);
    await expect(panel(page).getByText('Freigabe für 1 Titel gesetzt.')).toBeVisible();
    await expect(panel(page).getByRole('link', { name: UNRATED_TITLE })).toHaveCount(0);
    // The button went off with the selection: the h1 holds the focus.
    await expect(h1(page)).toBeFocused();
    await expectNoFocusTrap(page, TAB_MAX);
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
  await expect.poll(() => brokenState(owner), { timeout: 15_000 }).not.toBe('pending');
  const listed = (await brokenState(owner)) === 'listed';

  await openTab(page, /^Nicht abspielbar/);
  const link = panel(page).getByRole('link', { name: new RegExp(`^${BROKEN_TITLE}`) });
  if (listed) {
    await expect(link).toBeVisible();
    // The path and the finding in full (S-5).
    await expect(panel(page).getByText(/\.mkv$/)).toBeVisible();
    const box = panel(page).getByRole('checkbox', { name: `${BROKEN_TITLE} auswählen` });
    await tabTo(page, box, TAB_MAX);
    await page.keyboard.press('Space');
    await expect(box).toBeChecked();
    const remove = panel(page).getByRole('button', { name: 'Entfernen' });
    await tabBackTo(page, remove);
    await page.keyboard.press('Enter');
    const confirm = page.getByRole('alertdialog', { name: 'Dateien entfernen?' });
    await expect(confirm).toBeVisible();
    await expectFocusOn(page, confirm.getByRole('button', { name: 'Entfernen' }));
    await page.keyboard.press('Enter');
    await expect(confirm).toHaveCount(0);
    await expect(panel(page).getByText(/^Entfernt: \d+ Titel, 1 Datei\.$/)).toBeVisible();
    // The row is gone: the h1 takes the focus (ALL-2).
    await expect(h1(page)).toBeFocused();
  }
  await expect(link).toHaveCount(0);
  await expectNoFocusTrap(page, TAB_MAX);
  expect((await issues(owner, 'unplayable')).some((issue) => issue.title === BROKEN_TITLE)).toBe(
    false,
  );
});

test('a title links to its administration page; Back returns to the tab', async ({ page }) => {
  // Every title of the test media is unmatched and unrated (no TMDB key): «Ohne Freigabe» has rows.
  await openTab(page, /^Ohne Freigabe/);
  await expect(page).toHaveURL(/\/admin\/maintenance\?tab=unrated$/);
  const first = panel(page).getByRole('link').first();
  await expect(first).toBeVisible();
  const href = await first.getAttribute('href');
  expect(href).toMatch(/^\/admin\/items\//);
  await tabTo(page, first, TAB_MAX);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(new RegExp(`${href ?? ''}$`));
  await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
  await expect(h1(page)).toHaveCount(0);

  // The browser's Back returns to «Pflege» with the tab of the address.
  await page.goBack();
  await expect(page).toHaveURL(/\/admin\/maintenance\?tab=unrated$/);
  await expect(h1(page)).toBeVisible();
  await expect(tab(page, /^Ohne Freigabe/)).toHaveAttribute('aria-selected', 'true');
  await expect(
    panel(page).getByRole('link', { name: (await first.textContent()) ?? '' }),
  ).toBeVisible();
});
