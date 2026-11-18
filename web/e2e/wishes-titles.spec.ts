import type { Page } from '@playwright/test';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, resetFocus, tabTo } from './keyboard';
import { closeWishes, openWishes, type WishesSetup } from './wishes';

// Concept 25 stage 5, 5.4 (M-32, M-33): «Filme» and «Serien» of the wishes area by keyboard
// (rule 8). The lists come from /discover/movies and /discover/tv of the Seerr mock, six titles a
// page: the second page of films arrives as the grid scrolls.

const grid = (page: Page, name: string) => page.getByRole('list', { name, exact: true });
const content = (page: Page) => page.locator('#content');

test('«Filme» by keyboard: from the row title, the closed band, genre by arrows, Escape, the grid, Enter, back @keyboard', async ({
  page,
  owner,
}) => {
  let setup: WishesSetup | undefined;
  try {
    setup = await openWishes(owner);
    await page.goto('/discover');
    const head = page.getByRole('link', { name: /^Beliebte Filme/ });
    await resetFocus(page);
    await tabTo(page, head, 60);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/discover\/movies$/);
    await expect(page.getByRole('heading', { level: 1, name: 'Filme' })).toBeVisible();
    await expect(
      page.getByRole('navigation', { name: 'Bereiche unter Wünsche' }).getByRole('link', {
        name: 'Filme',
      }),
    ).toHaveAttribute('aria-current', 'page');
    const films = grid(page, 'Filme');
    await expect(films.getByRole('link', { name: /Glasvogel/ })).toBeVisible();
    // The second page follows as the grid scrolls (no «Mehr laden»).
    await films.getByRole('link').last().scrollIntoViewIfNeeded();
    await expect(films.getByRole('link', { name: /Iron Orchard/ })).toBeVisible();
    await expect(content(page)).not.toContainText('{{');

    // The band starts closed and says its choice on its button.
    const filter = page.getByRole('button', { name: 'Filter: Alle Genres · Vorhandene sichtbar' });
    await expect(filter).toHaveAttribute('aria-expanded', 'false');
    await resetFocus(page);
    await tabTo(page, filter, 40);
    await page.keyboard.press('Enter');
    await expect(filter).toHaveAttribute('aria-expanded', 'true');
    const band = page.getByRole('region', { name: 'Filter' });
    const genres = band.getByRole('group', { name: 'Genre' });
    await tabTo(page, genres.getByRole('button', { name: 'Alle Genres' }), 4);
    // Arrows walk the chips; Enter chooses «Drama».
    const drama = genres.getByRole('button', { name: 'Drama' });
    for (let presses = 0; presses < 6; presses += 1) {
      if (await drama.evaluate((element) => element === document.activeElement)) break;
      await page.keyboard.press('ArrowRight');
    }
    await expectFocusOn(page, drama);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/discover\/movies\?genre=18$/);
    // No focus trap in the band: Tab leaves it to the end of the page.
    await expectNoFocusTrap(page, 120);
    // Escape closes the band and hands the focus back to its button, which says the genre.
    await drama.focus();
    await page.keyboard.press('Escape');
    await expect(band).toHaveCount(0);
    const chosen = page.getByRole('button', { name: 'Filter: Drama · Vorhandene sichtbar' });
    await expectFocusOn(page, chosen);
    await expect(page).toHaveURL(/\/discover\/movies\?genre=18$/);

    // The grid by arrows; Enter opens the preview page, back returns to the same card.
    // The Drama films are there, and the focus stayed on the button meanwhile.
    await expect(films.getByRole('link', { name: /Rotes Ufer/ })).toBeVisible();
    await expect(films.getByRole('link', { name: /Kupfermond/ })).toHaveCount(0);
    await expectFocusOn(page, chosen);
    const cards = films.getByRole('link');
    await tabTo(page, cards.first(), 10);
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, cards.nth(1));
    const href = await cards.nth(1).getAttribute('href');
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(new RegExp(`${href}$`));
    await page.goBack();
    await expect(page).toHaveURL(/\/discover\/movies\?genre=18$/);
    await expectFocusOn(page, cards.nth(1));
  } finally {
    await closeWishes(owner, setup);
  }
});

test('links of «Entdecken» preselect the view; «Serien» requests with seasons; an empty choice says so', async ({
  page,
  owner,
}) => {
  let setup: WishesSetup | undefined;
  try {
    setup = await openWishes(owner);
    await page.goto('/discover');
    // A genre tile opens «Filme» with the genre, «Demnächst als Serie» «Serien» newest first.
    await page
      .getByRole('region', { name: 'Film-Genres' })
      .getByRole('link', { name: 'Abenteuer' })
      .click();
    await expect(page).toHaveURL(/\/discover\/movies\?genre=12$/);
    await expect(page.getByRole('button', { name: /^Filter: Abenteuer/ })).toBeVisible();
    await expect(grid(page, 'Filme').getByRole('link', { name: /Kupfermond/ })).toBeVisible();
    await page.goBack();
    await page.getByRole('link', { name: /^Demnächst als Serie/ }).click();
    await expect(page).toHaveURL(/\/discover\/series\?sort=newest$/);
    await expect(page.getByRole('combobox', { name: 'Sortierung' })).toHaveValue('newest');
    await expect(page.getByRole('heading', { level: 1, name: 'Serien' })).toBeVisible();

    // The plus of a series opens the dialog with its seasons.
    await page.goto('/discover/series');
    const series = grid(page, 'Serien');
    await expect(series.getByRole('link', { name: /Nebelhafen/ })).toBeVisible();
    await series.getByRole('link', { name: /Nebelhafen/ }).focus();
    await page.keyboard.press('Tab');
    await expectFocusOn(page, series.getByRole('button', { name: 'Nebelhafen anfordern' }));
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', { name: 'Serie anfordern: Nebelhafen' });
    await expect(dialog).toBeVisible();
    await expect(dialog.getByText('Staffel 1')).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(dialog).toHaveCount(0);

    // A choice without titles: the note with «Filter zurücksetzen».
    await page.goto('/discover/series?genre=35');
    await expect(page.getByText('Keine Serien für diese Auswahl.')).toBeVisible();
    await page.getByRole('button', { name: 'Filter zurücksetzen' }).click();
    await expect(page).toHaveURL(/\/discover\/series$/);
    await expect(series.getByRole('link', { name: /Nebelhafen/ })).toBeVisible();
    await expect(content(page)).not.toContainText('{{');
  } finally {
    await closeWishes(owner, setup);
  }
});
