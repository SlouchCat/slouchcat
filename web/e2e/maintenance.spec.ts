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
    'Altersfreigabe fehlt',
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
  await expectFocusOn(page, tab(page, /^Verwaist \d/));
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

test('unrated: select by keyboard, assign a rating, the title leaves the list @keyboard', async ({
  page,
  owner,
}) => {
  const unrated = (await issues(owner, 'unrated')).find((issue) => issue.title === UNRATED_TITLE);
  expect(unrated, `${UNRATED_TITLE} is unrated`).toBeDefined();
  try {
    await openTab(page, /^Altersfreigabe fehlt/);
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
  // Every title of the test media is unmatched and unrated (no TMDB key): «Altersfreigabe fehlt» has rows.
  await openTab(page, /^Altersfreigabe fehlt/);
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
  await expect(tab(page, /^Altersfreigabe fehlt/)).toHaveAttribute('aria-selected', 'true');
  await expect(
    panel(page).getByRole('link', { name: (await first.textContent()) ?? '' }),
  ).toBeVisible();
});

test('orphaned user data: the tab is reached by keyboard and lists what the server finds', async ({
  page,
  owner,
}) => {
  const response = await owner.get('/api/v1/admin/maintenance/relink');
  expect(response.status(), await response.text()).toBe(200);
  const { total } = (await response.json()) as { total: number };

  await openTab(page, /^Verwaiste Benutzerdaten/);
  await expect(page).toHaveURL(/\?tab=relink$/);
  await expect(tab(page, /^Verwaiste Benutzerdaten/)).toHaveAccessibleName(
    `Verwaiste Benutzerdaten ${total} Titel`,
  );
  await expect(panel(page).getByText(/^Gelöschte Titel, die als neue Kopie/)).toBeVisible();
  // The test media carries no provider ids (no TMDB key), so nothing is orphaned normally.
  if (total === 0) {
    await expect(panel(page).getByText('Keine verwaisten Benutzerdaten.')).toBeVisible();
    await expect(panel(page).getByRole('table')).toHaveCount(0);
  } else {
    await expect(panel(page).getByRole('table', { name: 'Verwaiste Benutzerdaten' })).toBeVisible();
  }
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, tab(page, /^Duplikate/));
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, tab(page, /^Verwaiste Benutzerdaten/));
  await expectNoFocusTrap(page, TAB_MAX);
});

test.describe('orphaned user data (requests intercepted)', () => {
  // The test media has no deleted title with a live copy, and making one needs provider ids the
  // run does not have: the list and the relink are answered here (the Go tests of admin and
  // library relink real pairs). The service worker stays out, as below.
  test.use({ serviceWorkers: 'block' });

  test('select by keyboard, «Umhängen» asks, Esc goes back, Enter relinks the selection', async ({
    page,
  }) => {
    const none = {
      progress: 0,
      progressEvents: 0,
      progressDays: 0,
      playbackSessions: 0,
      watchlist: 0,
      collectionItems: 0,
      notifications: 0,
      userPicks: 0,
      downloads: 0,
      activityEvents: 0,
    };
    const pair = (
      n: number,
      deletedTitle: string,
      title: string,
      userData: Partial<typeof none>,
    ) => ({
      deletedId: `01JB8Z6Q4M7N9P2R3S5T7V9X2${n}`,
      liveId: `01JB8Z6Q4M7N9P2R3S5T7V9X3${n}`,
      libraryId: 2,
      libraryName: 'Serien',
      kind: 'series',
      title,
      year: 2003,
      deletedTitle,
      deletedYear: 2003,
      deletedAt: '2026-10-28T09:00:00Z',
      userData: { ...none, ...userData },
    });
    let pairs = [
      pair(1, 'E2E Two and a Half Men', 'E2E Two and a Half Men Copy', {
        progress: 2,
        progressEvents: 100,
      }),
      pair(2, 'E2E Silo', 'E2E Silo', { progress: 1, progressEvents: 40 }),
    ];
    const posted: unknown[] = [];
    await page.route('**/api/v1/admin/maintenance/relink', async (route) => {
      if (route.request().method() === 'POST') {
        posted.push(route.request().postDataJSON());
        const [first] = pairs;
        pairs = pairs.slice(1);
        return route.fulfill({
          json: {
            relinked: [{ keptId: first?.deletedId, removedId: first?.liveId, merged: 3, moved: 0 }],
            skipped: 0,
          },
        });
      }
      return route.fulfill({ json: { total: pairs.length, pairs } });
    });
    await page.goto('/admin/maintenance');
    await expect(h1(page)).toBeVisible();

    await openTab(page, /^Verwaiste Benutzerdaten 2 Titel$/);
    const table = panel(page).getByRole('table', { name: 'Verwaiste Benutzerdaten' });
    // Both titles: the deleted one heads the row, the copy links to its administration page.
    const first = table.getByRole('row', { name: /E2E Two and a Half Men \(2003\)/ });
    await expect(first.getByRole('rowheader')).toHaveText('E2E Two and a Half Men (2003)');
    await expect(
      first.getByRole('link', { name: 'E2E Two and a Half Men Copy (2003)' }),
    ).toBeVisible();
    await expect(table.getByText('2 Fortschritte, 100 Wiedergabe-Ereignisse')).toBeVisible();
    const boxes = table.getByRole('checkbox');
    await expect(boxes).toHaveCount(2);
    await expect(boxes.nth(0)).toBeChecked();
    await expect(boxes.nth(1)).toBeChecked();

    // Tab into the table, ↓ to the second row, Space takes it out.
    await tabTo(page, boxes.first(), TAB_MAX);
    await page.keyboard.press('ArrowDown');
    await expectFocusOn(page, boxes.nth(1));
    await page.keyboard.press('Space');
    await expect(boxes.nth(1)).not.toBeChecked();
    await expect(panel(page).getByText('1 ausgewählt')).toBeVisible();

    // «Umhängen» asks first; Esc closes the question and the focus goes back to the button.
    const relink = panel(page).getByRole('button', { name: 'Umhängen' });
    await tabBackTo(page, relink);
    await page.keyboard.press('Enter');
    const confirm = page.getByRole('alertdialog', { name: 'Benutzerdaten umhängen?' });
    await expect(confirm).toBeVisible();
    await expect(
      confirm.getByText(
        /Es bleiben erhalten: 2 Fortschritte, 100 Wiedergabe-Ereignisse\. Die Kopie wird endgültig entfernt\.$/,
      ),
    ).toBeVisible();
    await expectFocusOn(page, confirm.getByRole('button', { name: 'Umhängen' }));
    await page.keyboard.press('Escape');
    await expect(confirm).toHaveCount(0);
    await expectFocusOn(page, relink);
    expect(posted).toHaveLength(0);

    // Enter twice: open the question, confirm. Only the selected pair goes to the server.
    await page.keyboard.press('Enter');
    await expect(confirm).toBeVisible();
    await page.keyboard.press('Enter');
    await expect(confirm).toHaveCount(0);
    await expect(panel(page).getByText('Umgehängt: 1 Titel.')).toBeVisible();
    expect(posted).toEqual([
      {
        pairs: [{ deletedId: '01JB8Z6Q4M7N9P2R3S5T7V9X21', liveId: '01JB8Z6Q4M7N9P2R3S5T7V9X31' }],
      },
    ]);
    await expect(
      table.getByRole('link', { name: 'E2E Two and a Half Men Copy (2003)' }),
    ).toHaveCount(0);
    // The pair taken out stays out, so «Umhängen» turned off and the h1 takes the focus (ALL-2).
    await expect(
      table.getByRole('checkbox', {
        name: 'E2E Silo (2003) auswählen (Serien, gelöscht am 28.10.2026)',
      }),
    ).not.toBeChecked();
    await expect(relink).toBeDisabled();
    await expect(h1(page)).toBeFocused();
    await expectNoFocusTrap(page, TAB_MAX);
  });
});

test.describe('refresh (requests intercepted)', () => {
  // Chromium sends the page's GET requests through the service worker, where page.route does not
  // see them: the issue lists are answered here, so the worker stays out.
  test.use({ serviceWorkers: 'block' });

  test('PFL-3: «Aktualisieren» without images and «Erneut prüfen» of an unplayable file ask per title', async ({
    page,
    owner,
  }) => {
    // The test media has no title without images (no TMDB key) and the broken file may be gone
    // already: both lists hold one real title here. The refresh itself would queue a metadata
    // run on the shared server; it is answered here and covered by the Go tests of `admin`.
    const response = await owner.get('/api/v1/admin/maintenance/issues?kind=unrated&limit=1');
    const [title] = ((await response.json()) as { items: Record<string, unknown>[] }).items;
    expect(title, 'a title to stand in').toBeDefined();
    const stubs: Record<string, Record<string, unknown>> = {
      no_images: { ...title, kind: 'no_images', title: 'E2E Ohne Bilder' },
      unplayable: {
        ...title,
        kind: 'unplayable',
        title: 'E2E Nicht abspielbar',
        mediaFileId: 'mf-e2e',
        path: 'E2E/kaputt.mkv',
        detail: 'invalid data found when processing input',
      },
    };
    await page.route('**/api/v1/admin/maintenance/issues?*', async (route) => {
      const kind = new URL(route.request().url()).searchParams.get('kind') ?? '';
      const stub = stubs[kind];
      if (!stub) return route.continue();
      return route.fulfill({ json: { total: 1, items: [stub] } });
    });
    const refreshed: string[] = [];
    await page.route('**/api/v1/admin/items/*/refresh', async (route) => {
      refreshed.push(new URL(route.request().url()).pathname);
      await route.fulfill({ status: 202, json: {} });
    });
    const path = `/api/v1/admin/items/${String(title.itemId)}/refresh`;

    for (const [kind, name, label] of [
      ['no_images', /^Ohne Bilder/, 'Aktualisieren'],
      ['unplayable', /^Nicht abspielbar/, 'Erneut prüfen'],
    ] as const) {
      await page.goto(`/admin/maintenance?tab=${kind}`);
      await expect(tab(page, name)).toHaveAttribute('aria-selected', 'true');
      const box = panel(page).getByRole('checkbox', {
        name: `${String(stubs[kind].title)} auswählen`,
      });
      const action = panel(page).getByRole('button', { name: label });
      await expect(action).toBeDisabled();
      await tabTo(page, box, TAB_MAX);
      await page.keyboard.press('Space');
      await expect(box).toBeChecked();
      await tabBackTo(page, action);
      const before = refreshed.length;
      await page.keyboard.press('Enter');
      await expect(panel(page).getByText('1 Titel wird aktualisiert.')).toBeVisible();
      expect(refreshed.slice(before)).toEqual([path]);
      // The button went off with the selection: the h1 holds the focus.
      await expect(h1(page)).toBeFocused();
      await expectNoFocusTrap(page, TAB_MAX);
    }
  });
});
