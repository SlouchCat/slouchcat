import { expect, test, type Page } from '@playwright/test';
import { EXTRA_TITLES, EXTRAS_TITLE, itemPath } from './catalog';
import { expectVisibleFocus, resetFocus } from './keyboard';

// B-7: trailers, featurettes, deleted scenes, behind the scenes and "-trailer" files are extras of
// their film: listed on its detail page, playable, never a title of their own on the start page.

let detail = '';

const extras = (page: Page) => page.getByRole('region', { name: 'Extras' });

/** Index of the focused card in the extras row, or -1. */
const focusedExtra = (page: Page) =>
  extras(page)
    .getByRole('link')
    .evaluateAll((links) => links.indexOf(document.activeElement as HTMLAnchorElement));

test.beforeEach(async ({ page }) => {
  detail ||= await itemPath(page, EXTRAS_TITLE);
  await page.goto('/');
  await page.goto(detail);
  await expect(page.getByRole('heading', { level: 1, name: EXTRAS_TITLE })).toBeVisible();
  await resetFocus(page);
});

test('the film lists its five extras with their kind', async ({ page }) => {
  await expect(extras(page).getByRole('link')).toHaveCount(EXTRA_TITLES.length);
  for (const title of EXTRA_TITLES) {
    await expect(
      extras(page)
        .getByRole('link')
        .filter({ has: page.getByText(title, { exact: true }) }),
    ).toHaveCount(1);
  }
  await expect(extras(page).getByText('Trailer', { exact: true })).toHaveCount(2);
  await expect(extras(page).getByText('Featurette', { exact: true })).toHaveCount(1);
  await expect(extras(page).getByText('Entfallene Szene', { exact: true })).toHaveCount(1);
  await expect(extras(page).getByText('Hinter den Kulissen', { exact: true })).toHaveCount(1);
});

test('extras are no titles of their own on the start page', async ({ page }) => {
  await page.goto('/');
  const main = page.getByRole('main');
  await expect(
    page
      .getByRole('region', { name: 'Neu hinzugefügt' })
      .getByRole('link')
      .filter({ has: page.getByText(EXTRAS_TITLE, { exact: true }) }),
  ).toHaveCount(1);
  for (const title of EXTRA_TITLES.filter((name) => !name.startsWith(EXTRAS_TITLE))) {
    await expect(main.getByRole('link').filter({ hasText: title })).toHaveCount(0);
  }
});

test('arrows reach the extras row; Enter plays an extra; back returns', async ({ page }) => {
  await page.keyboard.press('ArrowDown');
  for (let presses = 0; presses < 8; presses += 1) {
    if ((await focusedExtra(page)) >= 0) break;
    await page.keyboard.press('ArrowDown');
  }
  expect(await focusedExtra(page)).toBeGreaterThanOrEqual(0);
  await expectVisibleFocus(page);
  const start = await focusedExtra(page);
  await page.keyboard.press('ArrowRight');
  await expect.poll(() => focusedExtra(page)).toBe(start + 1);

  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/play\//);
  const player = page.getByRole('main', { name: 'Videoplayer' });
  await expect(player).toHaveAttribute('data-start-ms', /\d+/);
  await page.keyboard.press('Escape');
  await expect(page).toHaveURL(new RegExp(`${detail}$`));
});

test('an extra links back to its film', async ({ page }) => {
  // The extra's own detail page (through its id from the play link of a card).
  const card = extras(page).getByRole('link').first();
  const play = (await card.getAttribute('href')) ?? '';
  const id = play.split('/').pop() ?? '';
  await page.goto(`/items/${id}`);
  const parent = page.getByRole('link', { name: 'Zum Haupttitel' });
  await expect(parent).toHaveAttribute('href', detail);
  await parent.focus();
  await page.keyboard.press('Enter');
  await expect(page.getByRole('heading', { level: 1, name: EXTRAS_TITLE })).toBeVisible();
});
