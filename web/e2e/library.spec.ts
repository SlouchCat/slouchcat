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

test('library by keyboard: Tab order, arrows into the letter bar and back, Enter, back', async ({
  page,
  owner,
}) => {
  const id = await libraryId(owner, LIBRARY);
  await page.goto(`/libraries/${id}`);
  await expect(page.getByRole('heading', { level: 1, name: LIBRARY })).toBeVisible();
  const total = await cards(page).count();
  expect(total).toBeGreaterThan(1);
  await expect(page.getByRole('status').filter({ hasText: `${total} Titel` })).toBeVisible();
  // One content kind: no chips; fewer than 50 titles: no "Mehr laden".
  await expect(page.getByRole('group', { name: 'Inhalt' })).toHaveCount(0);
  await expect(page.getByRole('button', { name: /Mehr laden/ })).toHaveCount(0);

  // Tab: header, sort, direction, "Filter", the letter bar ("•", then the letters with titles),
  // the grid. Disabled letters are skipped.
  await resetFocus(page);
  await tabTo(page, sort(page));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, direction(page, 'aufsteigend'));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, filterButton(page));
  await expect(filterButton(page)).toHaveAttribute('aria-expanded', 'false');
  await page.keyboard.press('Tab');
  await expectFocusOn(page, letter(page, 'Alle Buchstaben'));
  await expect(letter(page, 'Alle Buchstaben')).toHaveAttribute('aria-pressed', 'true');
  await page.keyboard.press('Tab');
  await expectFocusOn(page, letter(page, 'S'));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, cards(page).first());

  // Arrows in the grid: right to the next card, left back.
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

test('the filter panel opens from "Filter", closes with Escape and gives the focus back', async ({
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

test('sort and direction live in the URL; the letter bar filters by title', async ({
  page,
  owner,
}) => {
  const id = await libraryId(owner, LIBRARY);
  await page.goto(`/libraries/${id}`);
  await expect(cards(page).first()).toBeVisible();
  const total = await cards(page).count();

  // The letter bar: every test film starts with "S"; a letter without titles is disabled.
  await expect(letter(page, 'A')).toBeDisabled();
  await expect(letter(page, 'Ziffer oder Zeichen')).toBeDisabled();
  await letter(page, 'S').click();
  await expect(page).toHaveURL(`/libraries/${id}?letter=S`);
  await expect(letter(page, 'S')).toHaveAttribute('aria-pressed', 'true');
  await expect(page.getByRole('status').filter({ hasText: `${total} Titel mit S` })).toBeVisible();
  await expect(cards(page)).toHaveCount(total);
  await expect(
    cards(page).filter({ has: page.getByText(H264_TITLE, { exact: true }) }),
  ).toHaveCount(1);
  for (const name of await cards(page).allTextContents()) expect(name).toMatch(/^Slouchcat/);

  // A new order: into the URL, its own direction (newest first), no letter and no bar.
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
