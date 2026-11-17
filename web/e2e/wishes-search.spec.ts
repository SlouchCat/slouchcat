import type { Page } from '@playwright/test';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';
import { closeWishes, openWishes, type WishesSetup } from './wishes';

// Concept 25 stage 5, 5.4 (M-34): «Suche» of the wishes area by keyboard (rule 8), over /search
// of the Seerr mock; the library search links to it with the same text (N-57).

const field = (page: Page) => page.getByRole('searchbox', { name: 'Film oder Serie suchen' });
const hits = (page: Page) => page.getByRole('list', { name: 'Treffer', exact: true });
const content = (page: Page) => page.locator('#content');

test('«Suche» by keyboard: focus in the field, too short, hits, ids, arrows, Enter, back, nothing found @keyboard', async ({
  page,
  owner,
}) => {
  let setup: WishesSetup | undefined;
  try {
    setup = await openWishes(owner);
    await page.goto('/discover/search');
    await expect(page.getByRole('heading', { level: 1, name: 'Suche' })).toBeVisible();
    await expectFocusOn(page, field(page));
    await expect(page.getByText(/^Gib einen Titel ein\./)).toBeVisible();
    await page.keyboard.type('G');
    await expect(page.getByText('Gib mindestens zwei Zeichen ein.')).toBeVisible();
    await page.keyboard.type('las');
    await expect(page).toHaveURL(/\/discover\/search\?q=Glas$/);
    // The hits: films and series TMDB knows, never the person Seerr lists.
    await expect(hits(page).getByRole('link', { name: /Glasvogel/ })).toBeVisible();
    await expect(hits(page).getByRole('link', { name: /Glasküste/ })).toBeVisible();
    await expect(content(page)).not.toContainText('{{');

    // ↓ from the field into the grid, → walks it, Tab reaches the plus.
    const cards = hits(page).getByRole('link');
    for (let presses = 0; presses < 4; presses += 1) {
      if (await cards.first().evaluate((element) => element === document.activeElement)) break;
      await page.keyboard.press('ArrowDown');
      await expectVisibleFocus(page);
    }
    await expectFocusOn(page, cards.first());
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, cards.nth(1));
    const href = await cards.nth(1).getAttribute('href');
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(new RegExp(`${href}$`));
    await page.goBack();
    await expect(page).toHaveURL(/\/discover\/search\?q=Glas$/);
    await expect(field(page)).toHaveValue('Glas');
    await expectFocusOn(page, cards.nth(1));
    await resetFocus(page);
    await expectNoFocusTrap(page, 120);

    // By id (the server reads the prefix); a person alone: nothing found.
    await page.goto('/discover/search?q=tvdb:79100');
    await expect(hits(page).getByRole('link', { name: /Nebelhafen/ })).toBeVisible();
    await expect(hits(page).getByRole('link')).toHaveCount(1);
    await page.goto('/discover/search?q=Nora');
    await expect(page.getByText('Nichts gefunden für «Nora»')).toBeVisible();
    await expect(page.getByText(/nicht nach Personen/)).toBeVisible();
  } finally {
    await closeWishes(owner, setup);
  }
});

test('the library search leads to «Suche» with the same text', async ({ page, owner }) => {
  let setup: WishesSetup | undefined;
  try {
    setup = await openWishes(owner);
    await page.goto('/search?q=Glas');
    const link = page.getByRole('link', { name: 'Auch in den Wünschen suchen' });
    await expect(link).toHaveAttribute('href', '/discover/search?q=Glas');
    // The section «Noch nicht in der Bibliothek» stays as it is.
    await expect(page.getByRole('heading', { name: 'Noch nicht in der Bibliothek' })).toBeVisible();
    await resetFocus(page);
    await tabTo(page, link, 80);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/discover\/search\?q=Glas$/);
    await expect(hits(page).getByRole('link', { name: /Glasvogel/ })).toBeVisible();
  } finally {
    await closeWishes(owner, setup);
  }
  // Without the wishes the link is gone.
  await page.goto('/search?q=Glas');
  await expect(page.getByRole('heading', { level: 1, name: 'Suche' })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Auch in den Wünschen suchen' })).toHaveCount(0);
});
