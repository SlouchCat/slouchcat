import type { APIRequestContext, Locator, Page } from '@playwright/test';
import { DEFAULT_HOME_ROWS, setHomeRows } from './curation';
import { homeRowKeys } from './discover';
import { expect, test } from './fixtures';
import { expectFocusOn, expectVisibleFocus, resetFocus, tabTo } from './keyboard';

// Concept 08, 7.1/7.4/8: the side navigation. From 768 px a column at the left (Playwright's
// desktop viewport is 1280 px), below 768 px and wherever the viewer hid the column a drawer behind
// the header's menu button. Both carry the <nav> "Hauptnavigation", never at the same time.
//
// Tab order: skip link, header (menu button where there is one, logo, search, inbox, profile),
// the side navigation, the content.

const nav = (page: Page) => page.getByRole('navigation', { name: 'Hauptnavigation' });
const menuButton = (page: Page) => page.getByRole('button', { name: 'Menü', exact: true });
const drawer = (page: Page) => page.getByRole('dialog', { name: 'Menü' });
const hideButton = (page: Page) =>
  nav(page).getByRole('button', { name: 'Seitenleiste ausblenden' });
const recentRow = (page: Page) => page.getByRole('region', { name: 'Neu hinzugefügt' });
const hero = (page: Page) => page.locator('main section[aria-roledescription]');

interface VisibleLibrary {
  id: number;
  name: string;
  itemCount: number;
}

/** The libraries the owner's primary profile sees, as the server names them (rule 12). */
async function visibleLibraries(owner: APIRequestContext): Promise<VisibleLibrary[]> {
  const response = await owner.get('/api/v1/libraries/visible');
  expect(response.status(), await response.text()).toBe(200);
  return (await response.json()) as VisibleLibrary[];
}

async function picksTotal(owner: APIRequestContext): Promise<number> {
  const response = await owner.get('/api/v1/picks?limit=1');
  expect(response.status(), await response.text()).toBe(200);
  return ((await response.json()) as { total: number }).total;
}

const sideNavState = (page: Page) =>
  page.evaluate(() => ({
    stored: localStorage.getItem('slouchcat.sidenav'),
    attribute: document.documentElement.dataset.sidenav ?? null,
  }));

const isFocused = (locator: Locator) =>
  locator.evaluate((element) => element === document.activeElement);

test('the column at 1280 px: its entries, arrows between them, Enter, into the content and back', async ({
  page,
  owner,
}) => {
  await page.goto('/');
  await expect(recentRow(page).getByRole('link').first()).toBeVisible();
  // One navigation of that name, the column; no menu button beside it.
  await expect(nav(page)).toHaveCount(1);
  await expect(menuButton(page)).toHaveCount(0);

  for (const name of ['Start', 'Suche', 'Merkliste', 'Downloads']) {
    await expect(nav(page).getByRole('link', { name, exact: true })).toBeVisible();
  }
  await expect(nav(page).getByRole('link', { name: 'Start', exact: true })).toHaveAttribute(
    'aria-current',
    'page',
  );
  const toggle = nav(page).getByRole('button', { name: 'Meine Medien ein- oder ausklappen' });
  await expect(toggle).toHaveAttribute('aria-expanded', 'true');
  // One entry per library of the suite (global-setup.ts) the profile sees, with its count.
  const libraries = await visibleLibraries(owner);
  expect(libraries.map((library) => library.name)).toContain('Filme');
  for (const library of libraries) {
    await expect(
      nav(page).getByRole('link', {
        name: `${library.name}, ${library.itemCount} Titel`,
        exact: true,
      }),
    ).toHaveAttribute('href', `/libraries/${library.id}`);
  }
  await expect(
    nav(page).getByRole('link', {
      name: `Cat's Empfehlungen, ${await picksTotal(owner)} Titel`,
      exact: true,
    }),
  ).toHaveAttribute('href', '/picks');
  await expect(nav(page).getByRole('link', { name: 'Verwaltung' })).toHaveAttribute(
    'href',
    '/admin',
  );
  await expect(nav(page).getByRole('link', { name: 'Profile', exact: true })).toBeVisible();
  await expect(nav(page).getByRole('button', { name: 'Abmelden' })).toBeVisible();
  await expect(hideButton(page)).toBeVisible();

  // Arrow down and up move between the entries, with a visible ring.
  const start = nav(page).getByRole('link', { name: 'Start', exact: true });
  await start.focus();
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, nav(page).getByRole('link', { name: 'Suche', exact: true }));
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, nav(page).getByRole('link', { name: 'Merkliste', exact: true }));
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, nav(page).getByRole('link', { name: 'Suche', exact: true }));

  // Arrow left from the first card of a row lands on the current page's entry; arrow right
  // returns to the card the viewer left.
  const firstCard = recentRow(page).getByRole('link').first();
  await firstCard.focus();
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, start);
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, firstCard);
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, start);

  // Down through the group to the film library; Enter opens its page, which is current then.
  const films = libraries.find((library) => library.name === 'Filme')!;
  const filmsEntry = nav(page).getByRole('link', { name: /^Filme, / });
  for (let presses = 0; presses < 12 && !(await isFocused(filmsEntry)); presses += 1) {
    await page.keyboard.press('ArrowDown');
  }
  await expectFocusOn(page, filmsEntry);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(`/libraries/${films.id}`);
  await expect(page.getByRole('heading', { level: 1, name: 'Filme' })).toBeVisible();
  await expect(filmsEntry).toHaveAttribute('aria-current', 'page');
  await expect(start).not.toHaveAttribute('aria-current');

  // The group closes and opens with its button; the state stays on this device.
  await toggle.click();
  await expect(toggle).toHaveAttribute('aria-expanded', 'false');
  await expect(filmsEntry).toHaveCount(0);
  await page.reload();
  await expect(toggle).toHaveAttribute('aria-expanded', 'false');
  await toggle.click();
  await expect(filmsEntry).toBeVisible();
});

test('Tab: skip link, header, the column, then the content', async ({ page }) => {
  await page.goto('/');
  await expect(recentRow(page).getByRole('link').first()).toBeVisible();
  await resetFocus(page);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('link', { name: 'Zum Inhalt springen' }));
  const header = page.locator('header');
  await tabTo(page, header.getByRole('link', { name: 'Suche öffnen' }), 2);
  await tabTo(page, header.getByRole('link', { name: /^Profil wechseln/ }), 2);
  await tabTo(page, nav(page).getByRole('link', { name: 'Start', exact: true }), 1);
  await tabTo(page, nav(page).getByRole('button', { name: 'Abmelden' }), 20);
  await tabTo(page, hideButton(page), 1);
  // The next stop is in the page.
  await page.keyboard.press('Tab');
  expect(await page.evaluate(() => document.activeElement?.closest('#content') !== null)).toBe(
    true,
  );
  await expectVisibleFocus(page);
});

test('hide the column: the menu button opens the drawer, which shows it again', async ({
  page,
}) => {
  await page.goto('/');
  await expect(recentRow(page).getByRole('link').first()).toBeVisible();
  await hideButton(page).focus();
  await page.keyboard.press('Enter');
  await expect(nav(page)).toHaveCount(0);
  // The focus moves to the menu button that replaces the column.
  await expectFocusOn(page, menuButton(page));
  await expect(menuButton(page)).toHaveAttribute('aria-expanded', 'false');
  expect(await sideNavState(page)).toEqual({ stored: 'hidden', attribute: 'hidden' });

  // A reload keeps it: the shell sets the attribute before the app runs.
  await page.reload();
  await expect(menuButton(page)).toBeVisible();
  await expect(nav(page)).toHaveCount(0);
  expect(await sideNavState(page)).toEqual({ stored: 'hidden', attribute: 'hidden' });

  // The drawer: focus on its first entry; its last entry brings the column back.
  await menuButton(page).focus();
  await page.keyboard.press('Enter');
  await expect(drawer(page)).toBeVisible();
  await expect(nav(page)).toHaveCount(1);
  await expectFocusOn(page, drawer(page).getByRole('link', { name: 'Start', exact: true }));
  await expect(drawer(page).getByRole('button', { name: 'Seitenleiste ausblenden' })).toHaveCount(
    0,
  );
  const show = drawer(page).getByRole('button', { name: 'Seitenleiste einblenden' });
  await show.focus();
  await page.keyboard.press('Enter');
  await expect(drawer(page)).toHaveCount(0);
  await expect(nav(page)).toHaveCount(1);
  await expect(menuButton(page)).toHaveCount(0);
  await expectFocusOn(page, hideButton(page));
  expect(await sideNavState(page)).toEqual({ stored: null, attribute: null });
});

test('on a phone the drawer: focus inside, no scrolling behind, Escape back to the button', async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/');
  await expect(recentRow(page).getByRole('link').first()).toBeVisible();
  await expect(nav(page)).toHaveCount(0);
  const menu = menuButton(page);
  await expect(menu).toHaveAttribute('aria-expanded', 'false');
  await expect(menu).toHaveAttribute('aria-controls', 'app-menu');

  await page.evaluate(() => window.scrollTo(0, 300));
  const scrolled = await page.evaluate(() => window.scrollY);
  expect(scrolled).toBeGreaterThan(0);
  await menu.focus();
  await page.keyboard.press('Enter');
  const dialog = drawer(page);
  await expect(dialog).toBeVisible();
  await expect(dialog).toHaveAttribute('id', 'app-menu');
  await expect(nav(page)).toHaveCount(1);
  await expectFocusOn(page, dialog.getByRole('link', { name: 'Start', exact: true }));
  // No column to hide or show on a phone.
  await expect(dialog.getByRole('button', { name: /^Seitenleiste/ })).toHaveCount(0);

  // Tab and the arrow keys stay inside the drawer.
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, dialog.getByRole('link', { name: 'Suche', exact: true }));
  for (let presses = 0; presses < 25; presses += 1) {
    await page.keyboard.press('Tab');
    expect(await dialog.evaluate((element) => element.contains(document.activeElement))).toBe(true);
  }

  // The page behind does not scroll.
  expect(await page.evaluate(() => getComputedStyle(document.documentElement).overflow)).toBe(
    'hidden',
  );
  await page.mouse.move(380, 600);
  await page.mouse.wheel(0, 800);
  await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(resolve)));
  expect(await page.evaluate(() => window.scrollY)).toBe(scrolled);

  // Escape closes it; the focus is back on the menu button, the page where it was.
  await page.keyboard.press('Escape');
  await expect(dialog).toHaveCount(0);
  await expectFocusOn(page, menu);
  await expect(menu).toHaveAttribute('aria-expanded', 'false');
  expect(await page.evaluate(() => window.scrollY)).toBe(scrolled);

  // An entry navigates and closes the drawer.
  await page.keyboard.press('Enter');
  await dialog.getByRole('link', { name: 'Merkliste', exact: true }).click();
  await expect(page).toHaveURL(/\/watchlist$/);
  await expect(dialog).toHaveCount(0);
});

test.describe('without the libraries', () => {
  // The service worker would fetch /api itself, past the page's routes.
  test.use({ serviceWorkers: 'block' });

  test('the group says so, links the downloads, and the rest stands', async ({ page }) => {
    await page.route('**/api/v1/libraries/visible', (route) => route.abort());
    await page.goto('/');
    await expect(recentRow(page).getByRole('link').first()).toBeVisible();
    await expect(nav(page).getByText('Offline: Bibliotheken nicht verfügbar')).toBeVisible();
    await expect(nav(page).getByRole('link', { name: 'Zu den Downloads' })).toHaveAttribute(
      'href',
      '/downloads',
    );
    await expect(nav(page).getByRole('link', { name: /^Filme/ })).toHaveCount(0);
    for (const name of ['Start', 'Suche', 'Merkliste', 'Downloads', 'Profile']) {
      await expect(nav(page).getByRole('link', { name, exact: true })).toBeVisible();
    }
    await expect(nav(page).getByRole('link', { name: 'Verwaltung' })).toBeVisible();
    await expect(nav(page).getByRole('button', { name: 'Abmelden' })).toBeVisible();
  });
});

/** Box of an element as [left, top, width, height]; empty when it is not there. */
const boxOf = (page: Page, selector: string) =>
  page.evaluate((query) => {
    const rect = document.querySelector(query)?.getBoundingClientRect();
    return rect ? [rect.left, rect.top, rect.width, rect.height] : [];
  }, selector);

/**
 * Puts the server's prepaint markup in front of #root, where the shell has it (api/preload.go).
 * The test media have no backdrops, so the server never writes a hero prepaint here, and the
 * suite's first posters are no cover: the geometry is checked with the same markup and a 1 px
 * image. The page is measured at the top, without focus.
 */
async function insertPrepaint(page: Page, markup: string): Promise<void> {
  await page.evaluate((html) => {
    (document.activeElement as HTMLElement | null)?.blur();
    window.scrollTo(0, 0);
    document.getElementById('hero-prepaint')?.remove();
    document.getElementById('poster-prepaint')?.remove();
    document.getElementById('root')?.insertAdjacentHTML('beforebegin', html);
  }, markup);
}

const PIXEL = 'data:image/gif;base64,R0lGODlhAQABAAAAACw=';

const expectSameBox = (painted: number[], target: number[]) => {
  expect(painted).toHaveLength(4);
  expect(target).toHaveLength(4);
  painted.forEach((value, index) =>
    expect(Math.abs(value - (target[index] ?? NaN))).toBeLessThan(1),
  );
};

test('the prepaints lie on the band and on the first card, column shown and hidden (P-10, 8)', async ({
  page,
  owner,
}) => {
  test.setTimeout(120_000);
  // The server asks the mock right after its start; the band needs its titles.
  await expect
    .poll(async () => (await homeRowKeys(owner)).includes('spotlight'), {
      timeout: 60_000,
      intervals: [1000],
    })
    .toBe(true);
  await page.goto('/');
  for (const hidden of [false, true]) {
    await page.evaluate((value) => {
      if (value) localStorage.setItem('slouchcat.sidenav', 'hidden');
      else localStorage.removeItem('slouchcat.sidenav');
    }, hidden);
    await page.reload();
    await expect(hero(page)).toBeVisible();
    await expect(nav(page)).toHaveCount(hidden ? 0 : 1);
    await insertPrepaint(
      page,
      `<div id="hero-prepaint" aria-hidden="true"><img src="${PIXEL}" alt="" width="1280" height="720"></div>`,
    );
    expectSameBox(
      await boxOf(page, '#hero-prepaint'),
      await boxOf(page, 'main section[aria-roledescription] > div'),
    );
  }

  // Without a stage the first row's first card: the baseline pick, a film with a 2:3 poster.
  try {
    await setHomeRows(owner, DEFAULT_HOME_ROWS, ['spotlight']);
    for (const hidden of [false, true]) {
      await page.evaluate((value) => {
        if (value) localStorage.setItem('slouchcat.sidenav', 'hidden');
        else localStorage.removeItem('slouchcat.sidenav');
      }, hidden);
      await page.reload();
      await expect(page.getByRole('region', { name: "Cat's Empfehlungen" })).toBeVisible();
      await expect(hero(page)).toHaveCount(0);
      await insertPrepaint(
        page,
        `<div id="poster-prepaint" aria-hidden="true"><img src="${PIXEL}" alt="" loading="eager"></div>`,
      );
      const frame = 'main section[aria-labelledby] ul a > span';
      // The card at rest, without the focus scale.
      await expect
        .poll(() =>
          page.evaluate(
            (query) => getComputedStyle(document.querySelector(query)!.parentElement!).transform,
            frame,
          ),
        )
        .toBe('none');
      expectSameBox(await boxOf(page, '#poster-prepaint > img'), await boxOf(page, frame));
    }
  } finally {
    await setHomeRows(owner, DEFAULT_HOME_ROWS);
  }
});
