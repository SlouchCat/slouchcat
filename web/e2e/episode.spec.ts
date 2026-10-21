import { expect, test, type Page } from '@playwright/test';
import { itemPath, SHOW_TITLE } from './catalog';
import { expectFocusOn, expectNoFocusTrap, resetFocus, tabTo } from './keyboard';

// The episode page (C-2): code and series above the title, play, "Nächste Folge", "Zur Serie".

let series = '';
let firstEpisode = '';

const play = (page: Page) =>
  page.getByRole('link', { name: /^(Abspielen|Weiterschauen ab \d{2}:\d{2}:\d{2})$/ });
const next = (page: Page) => page.getByRole('link', { name: /^Nächste Folge/ });

test.beforeEach(async ({ page }) => {
  if (!firstEpisode) {
    series = await itemPath(page, SHOW_TITLE);
    await page.goto(series);
    const card = page.getByRole('list', { name: 'Folgen' }).getByRole('link').first();
    await expect(card).toBeVisible();
    firstEpisode = (await card.getAttribute('href')) ?? '';
  }
  await page.goto('/');
  await page.goto(firstEpisode);
  await expect(page.getByText(`S01E01 · ${SHOW_TITLE}`)).toBeVisible();
  await resetFocus(page);
});

test('shows code, series and the episode in the tab title', async ({ page }) => {
  const title = await page.getByRole('heading', { level: 1 }).textContent();
  await expect(page).toHaveTitle(`S01E01 ${title ?? ''} – ${SHOW_TITLE} · Slouchcat`);
  await expect(page.getByRole('region', { name: 'Datei' }).getByText('MP4')).toBeVisible();
});

test('"Nächste Folge" leads to S01E02 and then to the double episode', async ({ page }) => {
  await expect(next(page)).toContainText('S01E02');
  await next(page).focus();
  await page.keyboard.press('Enter');
  await expect(page.getByText(`S01E02 · ${SHOW_TITLE}`)).toBeVisible();
  await expect(next(page)).toContainText('S01E03–E04');
  await next(page).focus();
  await page.keyboard.press('Enter');
  await expect(page.getByText(`S01E03–E04 · ${SHOW_TITLE}`)).toBeVisible();
});

test('"Zur Serie" leads to the series; browser back returns', async ({ page }) => {
  const link = page.getByRole('link', { name: 'Zur Serie' });
  await expect(link).toHaveAttribute('href', series);
  await link.focus();
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(new RegExp(`${series}$`));
  await expect(page.getByRole('heading', { level: 1, name: SHOW_TITLE })).toBeVisible();
  await page.goBack();
  await expect(page).toHaveURL(new RegExp(`${firstEpisode}$`));
});

test('arrows: play → back, down to the next episode', async ({ page }) => {
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, play(page));
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, page.getByRole('button', { name: 'Zurück' }));
  await page.keyboard.press('ArrowLeft');
  // The row under the actions (watchlist, M7) comes first.
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, page.getByRole('button', { name: 'Merken' }));
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, next(page));
});

// Tab order from the top of the page, like detail.spec: in Playwright's Firefox a Tab that would
// leave the document after the last link does not wrap to the start, so this does not follow the
// arrow test above.
test('Tab reaches play and the next episode after the header; no focus trap', async ({ page }) => {
  await tabTo(page, page.getByRole('button', { name: 'Abmelden' }));
  await tabTo(page, play(page), 2);
  // Back, the download, "Verwalten", then "Merken" and "Zu Cat's Empfehlungen" (M7).
  await tabTo(page, next(page), 8);
  await expectNoFocusTrap(page);
});
