import type { APIRequestContext, Locator, Page } from '@playwright/test';
import { H264_TITLE } from './catalog';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  focused,
  resetFocus,
  tabTo,
} from './keyboard';

// Concept 08, 7.3/7.4: a library as a grid with sort, direction, filters and the letter bar. The
// film library "Filme" holds the generated test films (testdata/media/Movies); without TMDB they
// keep the names of their files, all starting with "Slouchcat Test", unrated and without genres.

const LIBRARY = 'Filme';

/** Id of a library the owner's primary profile sees (the ids come from the global setup). */
async function libraryId(owner: APIRequestContext, name: string): Promise<number> {
  const response = await owner.get('/api/v1/libraries/visible');
  expect(response.status(), await response.text()).toBe(200);
  const libraries = (await response.json()) as { id: number; name: string }[];
  const library = libraries.find((entry) => entry.name === name);
  expect(library, `library "${name}" visible`).toBeDefined();
  return library!.id;
}

const grid = (page: Page) => page.getByRole('list', { name: LIBRARY });
const cards = (page: Page) => grid(page).getByRole('link');
const letterBar = (page: Page) => page.getByRole('navigation', { name: 'Anfangsbuchstabe' });
const letter = (page: Page, name: string) =>
  letterBar(page).getByRole('button', { name, exact: true });
const sort = (page: Page) => page.getByRole('combobox', { name: 'Sortieren' });
const direction = (page: Page, text: 'aufsteigend' | 'absteigend') =>
  page.getByRole('button', { name: `Reihenfolge: ${text === 'aufsteigend' ? '↑' : '↓'} ${text}` });
const filterButton = (page: Page) => page.getByRole('button', { name: /^Filter/ });

const isInside = (locator: Locator) =>
  locator.evaluate((element) => element.contains(document.activeElement));

test('library by keyboard: Tab order, arrows into the letter bar and back, Enter, back @keyboard', async ({
  page,
  owner,
}) => {
  const id = await libraryId(owner, LIBRARY);
  await page.goto(`/libraries/${id}`);
  await expect(page.getByRole('heading', { level: 1, name: LIBRARY })).toBeVisible();
  const total = await cards(page).count();
  expect(total).toBeGreaterThan(1);
  await expect(page.getByRole('status').filter({ hasText: `${total} Titel` })).toBeVisible();
  // One content kind: no chips; the grid holds every title, no "Mehr laden" (concept 17 D-6).
  await expect(page.getByRole('group', { name: 'Inhalt' })).toHaveCount(0);
  await expect(page.getByRole('button', { name: /Mehr laden/ })).toHaveCount(0);

  // Tab: header, sort, direction, "Filter", the letter bar (the letters with titles), the grid.
  // Disabled letters are skipped.
  await resetFocus(page);
  await tabTo(page, sort(page));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, direction(page, 'aufsteigend'));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, filterButton(page));
  await expect(filterButton(page)).toHaveAttribute('aria-expanded', 'false');
  await page.keyboard.press('Tab');
  await expectFocusOn(page, letter(page, 'S'));
  await expect(letter(page, 'S')).toHaveAttribute('aria-current', 'true');
  await page.keyboard.press('Tab');
  await expectFocusOn(page, cards(page).first());

  // Arrows in the grid: right to the next card, left back.
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, cards(page).nth(1));
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, cards(page).first());

  // Tab from a card: its pen, its check mark (concept 20 P-5), then its watchlist button
  // (concept 17 D-9); the arrows pass all three and move on from the card.
  const firstCell = grid(page).getByRole('listitem').first();
  await page.keyboard.press('Tab');
  await expectFocusOn(page, firstCell.getByRole('button', { name: /: Metadaten bearbeiten$/ }));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, firstCell.getByRole('button', { name: /^(Gesehen|Gehört): / }));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, firstCell.getByRole('button', { name: / merken$/ }));
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, cards(page).nth(1));
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, cards(page).first());

  // Right from the last column reaches the letter bar; left returns to the same card.
  let lastCard = '';
  for (let presses = 0; presses < 12 && !(await isInside(letterBar(page))); presses += 1) {
    lastCard = (await focused(page)).href ?? '';
    await page.keyboard.press('ArrowRight');
  }
  expect(await isInside(letterBar(page)), 'ArrowRight reached the letter bar').toBe(true);
  await expectVisibleFocus(page);
  expect(lastCard).toMatch(/^\/items\//);
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, grid(page).locator(`a[href="${lastCard}"]`));

  // Enter opens the title; the browser's back lands on the same card with the same URL.
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(lastCard);
  await page.goBack();
  await expect(page).toHaveURL(`/libraries/${id}`);
  await expectFocusOn(page, grid(page).locator(`a[href="${lastCard}"]`));

  await expectNoFocusTrap(page, 120);
});

test('the filter panel opens from "Filter", closes with Escape and gives the focus back @keyboard', async ({
  page,
  owner,
}) => {
  const id = await libraryId(owner, LIBRARY);
  await page.goto(`/libraries/${id}`);
  await expect(cards(page).first()).toBeVisible();
  await resetFocus(page);
  await tabTo(page, filterButton(page));
  await page.keyboard.press('Enter');
  await expect(filterButton(page)).toHaveAttribute('aria-expanded', 'true');
  const panel = page.getByRole('region', { name: 'Filter', exact: true });
  await expect(panel).toBeVisible();
  // Without TMDB the films have no genres and no ratings: those groups are not offered.
  await expect(panel.getByRole('group', { name: 'Genre' })).toHaveCount(0);
  await expect(panel.getByRole('group', { name: 'Altersfreigabe' })).toHaveCount(0);

  // Into the panel: the status chips come first.
  await page.keyboard.press('Tab');
  await expectFocusOn(page, panel.getByRole('button', { name: 'Ungesehen' }));
  await page.keyboard.press('Escape');
  await expect(panel).toHaveCount(0);
  await expectFocusOn(page, filterButton(page));
  await expect(filterButton(page)).toHaveAttribute('aria-expanded', 'false');
  // Escape closed the panel only; the page stayed.
  await expect(page).toHaveURL(`/libraries/${id}`);

  // A status filter counts on the button and in the heading's count; "Anwenden" closes the panel.
  await page.keyboard.press('Enter');
  await panel.getByRole('button', { name: 'Ungesehen' }).click();
  await expect(page).toHaveURL(`/libraries/${id}?status=unwatched`);
  await expect(page.getByRole('button', { name: 'Filter (1)' })).toBeVisible();
  await expect(page.getByRole('status').filter({ hasText: '· gefiltert' })).toBeVisible();
  // By pointer: the focus goes back to "Filter" without a ring (Firefox draws :focus-visible after
  // a script focus only when the last interaction was a key; the Escape path above checks the ring).
  await panel.getByRole('button', { name: 'Anwenden' }).click();
  await expect(panel).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Filter (1)' })).toBeFocused();
});

test('sort and direction live in the URL; the letter bar jumps by title', async ({
  page,
  owner,
}) => {
  const id = await libraryId(owner, LIBRARY);
  await page.goto(`/libraries/${id}`);
  await expect(cards(page).first()).toBeVisible();
  const total = await cards(page).count();

  // The letter bar (concept 17 section 6): every test film starts with "S", marked as the letter
  // in view; a letter without titles is disabled. A letter jumps to its first title and focuses
  // it; the URL and the list stay.
  await expect(letter(page, 'A')).toBeDisabled();
  await expect(letter(page, 'Ziffer oder Zeichen')).toBeDisabled();
  await expect(letter(page, 'S')).toHaveAttribute('aria-current', 'true');
  await letter(page, 'S').click();
  await expect(cards(page).first()).toBeFocused();
  await expect(page).toHaveURL(`/libraries/${id}`);
  await expect(cards(page)).toHaveCount(total);
  await expect(
    cards(page).filter({ has: page.getByText(H264_TITLE, { exact: true }) }),
  ).toHaveCount(1);
  for (const name of await cards(page).allTextContents()) expect(name).toMatch(/^Slouchcat/);
  // An old link with a letter (before contract 1.21.0) opens the whole list at its top.
  await page.goto(`/libraries/${id}?letter=S`);
  await expect(page.getByRole('status').filter({ hasText: `${total} Titel` })).toBeVisible();
  await expect(page.getByRole('status').filter({ hasText: 'mit S' })).toHaveCount(0);
  await expect(cards(page)).toHaveCount(total);

  // A new order: into the URL, its own direction (newest first), no letter bar.
  await sort(page).focus();
  await sort(page).selectOption('added');
  await expect(page).toHaveURL(`/libraries/${id}?sort=added`);
  await expect(direction(page, 'absteigend')).toBeVisible();
  await expect(letterBar(page)).toHaveCount(0);
  await expect(cards(page)).toHaveCount(total);
  // The focus stays on the select (only the URL and the grid change).
  await expect(sort(page)).toBeFocused();

  await direction(page, 'absteigend').click();
  await expect(page).toHaveURL(`/libraries/${id}?sort=added&order=asc`);
  await expect(direction(page, 'aufsteigend')).toBeFocused();
  await direction(page, 'aufsteigend').click();
  await expect(page).toHaveURL(`/libraries/${id}?sort=added`);

  // Back to the title order: the bar comes back.
  await sort(page).selectOption('title');
  await expect(page).toHaveURL(`/libraries/${id}`);
  await expect(letterBar(page)).toBeVisible();
});

test('on a phone the letter bar runs above a grid of three columns', async ({ page, owner }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const id = await libraryId(owner, LIBRARY);
  await page.goto(`/libraries/${id}`);
  await expect(cards(page).nth(3)).toBeVisible();
  const bar = await letterBar(page).boundingBox();
  const list = await grid(page).boundingBox();
  expect(bar && list && bar.y + bar.height <= list.y, 'the bar stands above the grid').toBe(true);
  const tops = await Promise.all(
    [0, 1, 2, 3].map(async (index) => (await cards(page).nth(index).boundingBox())?.y ?? -1),
  );
  expect(tops[1]).toBe(tops[0]);
  expect(tops[2]).toBe(tops[0]);
  expect(tops[3]).toBeGreaterThan(tops[0] ?? 0);
});

// Concept 17 section 6 (D-6): a large library in the virtual grid, by keyboard. The library
// "Filme" answers with 1'800 made-up titles (360 rows of five at 1280 px beside the side
// navigation, posters without a file), each page 300 ms late, so the grid meets its empty places
// (plan C-5, M-8).
test.describe('a large library in the virtual grid', () => {
  test.use({ serviceWorkers: 'block', viewport: { width: 1280, height: 800 } });

  const TOTAL = 1800;
  const NUMBERS = 32;
  const PER_LETTER = 68;
  const DELAY_MS = 300;
  /** Budget of one arrow press: the quiet time (100 ms), the late page and drawing it. */
  const PRESS_MS = 2_000;

  const letterOf = (index: number) =>
    index < NUMBERS ? '#' : String.fromCharCode(65 + Math.floor((index - NUMBERS) / PER_LETTER));
  const fake = (index: number) => ({
    id: `01J9E2EB1GL1BRARY${String(index).padStart(9, '0')}`,
    kind: 'movie',
    title: index < NUMBERS ? `${index} Nummer` : `${letterOf(index)} Titel ${index}`,
    inWatchlist: false,
    year: 2000,
    images: {},
  });
  /** Where each letter starts in a list of `total` titles (PER_LETTER for TOTAL). */
  const letterStarts = (total: number) => [
    { letter: '#', offset: 0 },
    ...Array.from({ length: 26 }, (_, n) => ({
      letter: String.fromCharCode(65 + n),
      offset: NUMBERS + n * Math.floor((total - NUMBERS) / 26),
    })),
  ];

  /** `failing` maps an offset to how many of its reads fail (500) before it answers. */
  async function openLarge(
    page: Page,
    owner: APIRequestContext,
    { failing = new Map<number, number>(), total = TOTAL } = {},
  ): Promise<number> {
    const id = await libraryId(owner, LIBRARY);
    await page.route(`**/api/v1/libraries/${id}/items?*`, async (route) => {
      const url = new URL(route.request().url());
      const offset = Number(url.searchParams.get('offset') ?? 0);
      const limit = Number(url.searchParams.get('limit') ?? 50);
      if (offset > 0) await new Promise((resolve) => setTimeout(resolve, DELAY_MS));
      const fails = failing.get(offset) ?? 0;
      if (fails > 0) {
        failing.set(offset, fails - 1);
        await route.fulfill({
          status: 500,
          json: { type: 'about:blank', title: 'internal', status: 500, code: 'internal' },
        });
        return;
      }
      await route.fulfill({
        json: {
          items: Array.from({ length: Math.max(0, Math.min(limit, total - offset)) }, (_, n) =>
            fake(offset + n),
          ),
          total,
          offset,
          limit,
          facets: {
            letters: ['#', ...'ABCDEFGHIJKLMNOPQRSTUVWXYZ'],
            genres: [],
            ratings: [],
            ...(offset === 0 ? { letterStarts: letterStarts(total) } : {}),
          },
        },
      });
    });
    await page.goto(`/libraries/${id}`);
    await expect(page.getByRole('status').filter({ hasText: `${total} Titel` })).toBeVisible();
    await expect(cards(page).first()).toBeVisible();
    return id;
  }

  /** The grid's columns, from its CSS (five at 1280 px). */
  const columnsOf = (page: Page) =>
    grid(page).evaluate((list) => getComputedStyle(list).gridTemplateColumns.split(' ').length);

  /** The place of the focused element in the grid, or -1 outside it. */
  const focusedPlace = (page: Page) =>
    page.evaluate(() => {
      const place = document.activeElement?.closest<HTMLElement>('[data-index]');
      return place ? Number(place.dataset.index) : -1;
    });

  /** Places in view without a card (an empty row the reader would see). */
  const emptyInView = (page: Page) =>
    page.evaluate(() =>
      [...document.querySelectorAll<HTMLElement>('[data-index]')]
        .filter((place) => {
          const box = place.getBoundingClientRect();
          return box.bottom > 0 && box.top < window.innerHeight;
        })
        .filter((place) => place.querySelector('a[href]') === null)
        .map((place) => Number(place.dataset.index)),
    );

  test('ArrowDown walks all 360 rows without losing the focus @keyboard', async ({
    page,
    owner,
  }) => {
    // 360 presses, each within its budget: the test takes longer than one page view.
    test.setTimeout(360 * PRESS_MS);
    await openLarge(page, owner);
    const COLUMNS = await columnsOf(page);
    expect(COLUMNS).toBe(5);
    await resetFocus(page);
    await page.keyboard.press('ArrowDown');
    await expect.poll(() => focusedPlace(page)).toBe(0);
    for (let row = 1; row < TOTAL / COLUMNS; row += 1) {
      await page.keyboard.press('ArrowDown');
      await expect
        .poll(() => focusedPlace(page), { timeout: PRESS_MS, message: `row ${row}` })
        .toBe(row * COLUMNS);
    }
    expect(await focusedPlace(page)).toBe(TOTAL - COLUMNS);
    await expectVisibleFocus(page);
    await expect(page.locator(':focus')).toBeInViewport();
    await expect.poll(() => emptyInView(page)).toEqual([]);
    // The last row: ArrowDown keeps the focus in it.
    await page.keyboard.press('ArrowDown');
    expect(await focusedPlace(page)).toBeGreaterThanOrEqual(TOTAL - COLUMNS);
  });

  test('a letter jumps; Enter opens a card and back returns to it @keyboard', async ({
    page,
    owner,
  }) => {
    const id = await openLarge(page, owner);
    const COLUMNS = await columnsOf(page);
    const m = NUMBERS + 12 * PER_LETTER;
    await letter(page, 'M').focus();
    await page.keyboard.press('Enter');
    await expect.poll(() => focusedPlace(page)).toBe(m);
    await expectVisibleFocus(page);
    await expect(page.locator(':focus')).toBeInViewport();
    await expect(page.locator(':focus')).toContainText('M Titel');
    await expect(letter(page, 'M')).toHaveAttribute('aria-current', 'true');
    await expect.poll(() => emptyInView(page)).toEqual([]);
    // Two rows down, Enter, back: the same card with the focus, in view.
    await page.keyboard.press('ArrowDown');
    await page.keyboard.press('ArrowDown');
    const chosen = m + 2 * COLUMNS;
    await expect.poll(() => focusedPlace(page)).toBe(chosen);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(`/items/${fake(chosen).id}`);
    await page.goBack();
    await expect(page).toHaveURL(`/libraries/${id}`);
    await expect.poll(() => focusedPlace(page)).toBe(chosen);
    await expect(page.locator(':focus')).toBeInViewport();
    await expectVisibleFocus(page);

    // Back at another width (fewer columns, the card in another row): its place comes back into
    // view, then its card with the focus.
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(`/items/${fake(chosen).id}`);
    await page.setViewportSize({ width: 1060, height: 800 });
    await page.goBack();
    await expect.poll(() => columnsOf(page)).toBeLessThan(COLUMNS);
    await expect.poll(() => focusedPlace(page)).toBe(chosen);
    await expect(page.locator(':focus')).toBeInViewport();
  });

  test('a failed page says so; its retry button is reached by Tab and reads it again @keyboard', async ({
    page,
    owner,
  }) => {
    // Page 2 fails twice: its read and the one retry of the app's queries.
    await openLarge(page, owner, { failing: new Map([[200, 2]]) });
    const d = NUMBERS + 3 * PER_LETTER;
    await letter(page, 'D').focus();
    await page.keyboard.press('Enter');
    const alert = page.getByRole('alert').filter({ hasText: 'Ein Teil der Liste' });
    await expect(alert).toBeVisible();
    await expect(alert).toBeInViewport();
    await expect(grid(page).locator(`[data-index="${d}"] a[href]`)).toHaveCount(0);
    // From the letter bar, Tab reaches the retry button; Enter reads the page again.
    const retry = alert.getByRole('button', { name: 'Erneut versuchen' });
    await tabTo(page, retry);
    await page.keyboard.press('Enter');
    await expect(alert).toHaveCount(0);
    await expect(grid(page).locator(`[data-index="${d}"] a[href]`)).toBeInViewport();
    await expect.poll(() => emptyInView(page)).toEqual([]);
    // The focus the button held goes to the first card in view, the list stays where it was.
    const columns = await columnsOf(page);
    await expect.poll(() => focusedPlace(page)).toBe(Math.floor(d / columns) * columns);
    await expectVisibleFocus(page);
    await expect(grid(page).locator(`[data-index="${d}"] a[href]`)).toBeInViewport();
  });

  test('more than 10000 rows: a jump far down draws its cards in view @keyboard', async ({
    page,
    owner,
  }) => {
    // Three columns on a phone: 40'000 titles are 13'334 rows, past the 10'000 grid lines of
    // Firefox (the places are not grid rows).
    await page.setViewportSize({ width: 390, height: 844 });
    const total = 40_000;
    await openLarge(page, owner, { total });
    expect(await columnsOf(page)).toBe(3);
    const z = letterStarts(total).at(-1)!.offset;
    expect(Math.floor(z / 3)).toBeGreaterThan(10_000);
    await letter(page, 'Z').focus();
    await page.keyboard.press('Enter');
    await expect.poll(() => focusedPlace(page)).toBe(z);
    await expect(page.locator(':focus')).toBeInViewport();
    await expectVisibleFocus(page);
    await expect.poll(() => emptyInView(page)).toEqual([]);
    // ArrowDown goes on to the row below, still in view.
    await page.keyboard.press('ArrowDown');
    await expect.poll(() => focusedPlace(page)).toBe(z + 3);
    await expect(page.locator(':focus')).toBeInViewport();
  });

  test('Tab leaves the grid after the last card; the wheel keeps the focus @keyboard', async ({
    page,
    owner,
    browserName,
  }) => {
    await openLarge(page, owner);
    const COLUMNS = await columnsOf(page);
    // The focus on the first card stays while the wheel scrolls far away from it.
    await resetFocus(page);
    await page.keyboard.press('ArrowDown');
    await expect.poll(() => focusedPlace(page)).toBe(0);
    const first = page.locator(':focus');
    const href = await first.getAttribute('href');
    await page.mouse.move(400, 400);
    // Firefox scrolls less per wheel turn than Chromium: turn until far enough.
    const scrolled = () => page.evaluate(() => window.scrollY);
    for (let turn = 0; turn < 40 && (await scrolled()) <= 10_000; turn += 1) {
      await page.mouse.wheel(0, 2_000);
    }
    await expect.poll(scrolled).toBeGreaterThan(10_000);
    await expect.poll(() => emptyInView(page), { timeout: 5_000 }).toEqual([]);
    expect(await focusedPlace(page)).toBe(0);
    await expect(grid(page).locator(`a[href="${href}"]`)).toBeFocused();
    // ArrowDown goes on from it.
    await page.keyboard.press('ArrowDown');
    await expect.poll(() => focusedPlace(page)).toBe(COLUMNS);

    // The last card of the list: Tab through its controls, then out of the grid.
    await letter(page, 'Z').focus();
    await page.keyboard.press('Enter');
    await expect.poll(() => focusedPlace(page)).toBe(NUMBERS + 25 * PER_LETTER);
    for (let place = await focusedPlace(page); place < TOTAL - COLUMNS;) {
      await page.keyboard.press('ArrowDown');
      await expect
        .poll(() => focusedPlace(page), { timeout: PRESS_MS })
        .toBe(Math.min(place + COLUMNS, TOTAL - 1));
      place = await focusedPlace(page);
    }
    for (let place = await focusedPlace(page); place < TOTAL - 1; place += 1) {
      await page.keyboard.press('ArrowRight');
      await expect.poll(() => focusedPlace(page)).toBe(place + 1);
    }
    await expect.poll(() => focusedPlace(page)).toBe(TOTAL - 1);
    const last = grid(page).locator(`[data-index="${TOTAL - 1}"]`);
    const controls = await last.locator('a[href], button:not([disabled])').all();
    // Each Tab waits for its focus (Firefox moves it a moment after the key on a fresh cell).
    for (const control of controls.slice(1)) {
      await page.keyboard.press('Tab');
      await expect(control).toBeFocused();
    }
    // The grid lets this Tab go (it keeps only a Tab towards a place not drawn or still empty).
    // Nothing focusable follows the grid, so the focus leaves the page; Firefox (headless) keeps it
    // on the button at the end of the document, Chromium hands it on.
    await page.evaluate(() => {
      window.addEventListener(
        'keydown',
        (event) => document.body.setAttribute('data-tab-prevented', String(event.defaultPrevented)),
        { once: true },
      );
    });
    await page.keyboard.press('Tab');
    await expect(page.locator('body')).toHaveAttribute('data-tab-prevented', 'false');
    if (browserName === 'chromium') await expect.poll(() => isInside(grid(page))).toBe(false);
  });
});

// Concept 19 part C: the overview of an audio library shows its covers square, without the 2:3
// frame (PC-1, PC-4); a mixed library does so only with the content filter on audio (PC-2).
test.describe('square covers in the overview of audio', () => {
  test.use({ serviceWorkers: 'block', viewport: { width: 1280, height: 800 } });

  const BOOKS = 'Hörbücher';
  const audioGrid = (page: Page, name: string) => page.getByRole('list', { name });

  /** Height per width of the drawn covers' frames, rounded to tenths. */
  const frameRatios = (list: Locator) =>
    list.evaluate((node) =>
      [...node.querySelectorAll('a[href] > span:first-child')].map((frame) => {
        const box = frame.getBoundingClientRect();
        return Math.round((box.height / box.width) * 10) / 10;
      }),
    );

  /**
   * Cards that do not fit their place or overlap the next row: a card's link ends below its
   * place, or a place reaches into the place under it.
   */
  const crowded = (list: Locator) =>
    list.evaluate((node) => {
      const places = [...node.querySelectorAll<HTMLElement>('[data-index]')];
      const found: string[] = [];
      for (const place of places) {
        const box = place.getBoundingClientRect();
        const link = place.querySelector('a[href]')?.getBoundingClientRect();
        if (link && link.bottom > box.bottom + 1) found.push(`card ${place.dataset.index}`);
        for (const other of places) {
          const below = other.getBoundingClientRect();
          if (other !== place && below.top > box.top && below.top < box.bottom - 1) {
            found.push(`place ${place.dataset.index}/${other.dataset.index}`);
          }
        }
      }
      return found;
    });

  const fakeBook = (index: number) => ({
    id: `01J9E2EB1GAUD1OBOOK${String(index).padStart(7, '0')}`,
    kind: 'audiobook',
    title: `Hörbuch ${index}`,
    author: 'Slouchcat Test Author',
    inWatchlist: false,
    images: {},
  });

  test('PC-1: the audiobook library shows its covers square, titles of a row on one line', async ({
    page,
    owner,
  }) => {
    const id = await libraryId(owner, BOOKS);
    await page.goto(`/libraries/${id}`);
    const list = audioGrid(page, BOOKS);
    await expect(list.getByRole('link').first()).toBeVisible();
    expect(new Set(await frameRatios(list))).toEqual(new Set([1]));
    // No blurred copy behind a cover: one image per card.
    for (const card of await list.getByRole('link').all()) {
      expect(await card.locator('img').count()).toBeLessThanOrEqual(1);
    }
    const titleTops = await list.evaluate((node) =>
      [...node.querySelectorAll('a[href] > span:last-child')].map((meta) =>
        Math.round(meta.getBoundingClientRect().top),
      ),
    );
    expect(new Set(titleTops).size).toBe(1);
    expect(await crowded(list)).toEqual([]);
  });

  test('PC-1: 250 audiobooks scroll by without overlap', async ({ page, owner }) => {
    const id = await libraryId(owner, BOOKS);
    const total = 250;
    await page.route(`**/api/v1/libraries/${id}/items?*`, async (route) => {
      const url = new URL(route.request().url());
      const offset = Number(url.searchParams.get('offset') ?? 0);
      const limit = Number(url.searchParams.get('limit') ?? 50);
      await route.fulfill({
        json: {
          items: Array.from({ length: Math.max(0, Math.min(limit, total - offset)) }, (_, n) =>
            fakeBook(offset + n),
          ),
          total,
          offset,
          limit,
          facets: { letters: ['H'], genres: [], ratings: [] },
        },
      });
    });
    await page.goto(`/libraries/${id}`);
    const list = audioGrid(page, BOOKS);
    await expect(page.getByRole('status').filter({ hasText: `${total} Hörbücher` })).toBeVisible();
    const last = list.locator(`[data-index="${total - 1}"] a[href]`);
    for (let step = 0; step < 40 && !(await last.isVisible()); step += 1) {
      await page.mouse.wheel(0, 700);
      await expect.poll(() => crowded(list)).toEqual([]);
      expect(new Set(await frameRatios(list))).toEqual(new Set([1]));
    }
    await expect(last).toBeVisible();
  });

  test('PC-4: the square overview by keyboard: arrows, Tab, Enter, back to the same card @keyboard', async ({
    page,
    owner,
  }) => {
    const id = await libraryId(owner, BOOKS);
    await page.goto(`/libraries/${id}`);
    const list = audioGrid(page, BOOKS);
    const books = list.getByRole('link');
    await expect(books.first()).toBeVisible();
    expect(await books.count()).toBeGreaterThan(1);
    await resetFocus(page);
    await tabTo(page, books.first(), 40);
    await expectVisibleFocus(page);
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, books.nth(1));
    await page.keyboard.press('ArrowLeft');
    await expectFocusOn(page, books.first());
    const href = await books.first().getAttribute('href');
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(href!);
    await page.goBack();
    await expect(page).toHaveURL(`/libraries/${id}`);
    await expectFocusOn(page, list.locator(`a[href="${href}"]`));
    await expectNoFocusTrap(page, 120);
  });

  test('PC-2: a mixed library: «Alle» keeps the 2:3 frame, «Hörbücher» is square', async ({
    page,
    owner,
  }) => {
    const id = await libraryId(owner, LIBRARY);
    // The film library as a mixed one: films and audiobooks.
    await page.route('**/api/v1/libraries/visible', async (route) => {
      const response = await route.fetch();
      const libraries = (await response.json()) as { id: number; contents: string[] }[];
      await route.fulfill({
        response,
        json: libraries.map((library) =>
          library.id === id ? { ...library, contents: ['movie', 'audiobook'] } : library,
        ),
      });
    });
    await page.route(`**/api/v1/libraries/${id}/items?*`, async (route) => {
      const url = new URL(route.request().url());
      if (url.searchParams.get('content') !== 'audiobook') {
        await route.fallback();
        return;
      }
      const offset = Number(url.searchParams.get('offset') ?? 0);
      const limit = Number(url.searchParams.get('limit') ?? 50);
      // A slow first page: the films stay drawn meanwhile.
      if (offset === 0) await new Promise((resolve) => setTimeout(resolve, 1500));
      await route.fulfill({
        json: {
          items: Array.from({ length: Math.max(0, Math.min(limit, 40 - offset)) }, (_, n) =>
            fakeBook(offset + n),
          ),
          total: 40,
          offset,
          limit,
          facets: { letters: ['H'], genres: [], ratings: [] },
        },
      });
    });
    await page.goto(`/libraries/${id}`);
    const list = grid(page);
    await expect(cards(page).first()).toBeVisible();
    expect(new Set(await frameRatios(list))).toEqual(new Set([1.5]));
    const chips = page.getByRole('group', { name: 'Inhalt' });
    await chips.getByRole('button', { name: 'Hörbücher' }).click();
    // While the audiobooks load, the films keep their 2:3 rows: nothing overlaps.
    await page.waitForTimeout(500);
    await expect(list.getByRole('link', { name: /Hörbuch 0/ })).toHaveCount(0);
    expect(new Set(await frameRatios(list))).toEqual(new Set([1.5]));
    expect(await crowded(list)).toEqual([]);
    await expect(list.getByRole('link', { name: /Hörbuch 0/ })).toBeVisible();
    await expect.poll(async () => [...new Set(await frameRatios(list))]).toEqual([1]);
    await expect.poll(() => crowded(list)).toEqual([]);
    await chips.getByRole('button', { name: 'Alle' }).click();
    await expect.poll(async () => [...new Set(await frameRatios(list))]).toEqual([1.5]);
    await expect.poll(() => crowded(list)).toEqual([]);
  });
});
