import type { Page } from '@playwright/test';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, expectVisibleFocus, tabTo } from './keyboard';
import { closeWishes, openWishes, seerrRequests, type WishesSetup } from './wishes';

// Concept 25 stage 2, 5.1 and 5.5: the preview page of a title outside the library (M-03, M-04)
// and the request dialog (M-13), by keyboard (rule 8). The titles come from the TMDB mock
// (tmdb-server.mjs), the wishes go to the Seerr mock (seerr-server.mjs); openWishes stores the
// TMDB key for the test and closeWishes clears it again (e2e/wishes.ts).

const FILM = '/titles/movie/9001';
const SERIES = '/titles/show/9100';

const content = (page: Page) => page.locator('#content');

test('a film: Tab order, the dialog by keyboard, Escape and «Abbrechen» return, the request turns the button into its state @keyboard', async ({
  page,
  owner,
}) => {
  let setup: WishesSetup | undefined;
  try {
    setup = await openWishes(owner);
    await page.goto(FILM);
    await expect(page.getByRole('heading', { level: 1, name: 'Glasvogel' })).toBeVisible();
    await expect(page.getByText('Noch nicht in der Bibliothek', { exact: true })).toBeVisible();
    // No age rating for viewers (concept 17 F-10), the facts are year and runtime.
    await expect(page.getByText(/2026 · 1 h 41 min/)).toBeVisible();
    await expect(content(page)).not.toContainText('{{');

    // Tab after the side navigation: the back arrow, the big button, the trailer.
    const request = page.getByRole('button', { name: 'Film anfordern' });
    await tabTo(page, page.getByRole('button', { name: 'Seitenleiste ausblenden' }));
    await page.keyboard.press('Tab');
    await expectFocusOn(page, page.getByRole('button', { name: 'Zurück', exact: true }));
    await page.keyboard.press('Tab');
    await expectFocusOn(page, request);
    await page.keyboard.press('Tab');
    await expectFocusOn(page, page.getByRole('button', { name: 'Trailer' }));
    // ↑ from the trailer goes back to the big button (arrow keys of the head).
    await page.keyboard.press('ArrowUp');
    await expectFocusOn(page, request);

    // Enter opens the dialog with the focus inside; arrows stay in it, Escape returns.
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', { name: 'Film anfordern: Glasvogel' });
    await expect(dialog).toBeVisible();
    const submit = dialog.getByRole('button', { name: 'Anfordern', exact: true });
    await expectFocusOn(page, submit);
    await expect(dialog.getByText(/Braucht eine Freigabe/)).toBeVisible();
    await page.keyboard.press('ArrowDown');
    await page.keyboard.press('ArrowLeft');
    await expect(dialog).toBeVisible();
    await expect(page.locator(':focus')).toBeAttached();
    expect(await dialog.evaluate((element) => element.contains(document.activeElement))).toBe(true);
    await page.keyboard.press('Escape');
    await expect(dialog).toHaveCount(0);
    await expectFocusOn(page, request);

    // «Abbrechen» returns as well.
    await page.keyboard.press('Enter');
    await expect(dialog).toBeVisible();
    await page.keyboard.press('Shift+Tab');
    await expectFocusOn(page, dialog.getByRole('button', { name: 'Abbrechen' }));
    await page.keyboard.press('Enter');
    await expect(dialog).toHaveCount(0);
    await expectFocusOn(page, request);

    // Request: the dialog closes, the button becomes «Freigabe ausstehend» and keeps the focus.
    await page.keyboard.press('Enter');
    await expect(submit).toBeFocused();
    await page.keyboard.press('Enter');
    await expect(dialog).toHaveCount(0);
    const state = page.getByRole('link', { name: 'Freigabe ausstehend' });
    await expect(state).toBeVisible();
    await expectVisibleFocus(page);
    await expect(page.getByRole('heading', { name: 'Dein Wunsch' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Zurückziehen' })).toBeVisible();
    expect((await seerrRequests()).map((r) => [r.type, r.mediaId])).toEqual([['movie', 9001]]);

    // Tab leaves the page: no focus trap.
    await expectNoFocusTrap(page, 60);
  } finally {
    await closeWishes(owner, setup);
  }
});

test('a series: the seasons with their states, a refused limit and a request of the seasons that fit', async ({
  page,
  owner,
}) => {
  let setup: WishesSetup | undefined;
  try {
    // One season left of the limit.
    setup = await openWishes(owner, { limit: 1 });
    await page.goto(SERIES);
    await expect(page.getByRole('heading', { level: 1, name: 'Nebelhafen' })).toBeVisible();
    const seasons = page.getByRole('region', { name: 'Staffeln' });
    await expect(seasons.getByRole('listitem')).toHaveCount(3);
    await expect(page.getByText(/Läuft noch, nächste Folge am/)).toBeVisible();

    await page.getByRole('button', { name: 'Serie anfordern' }).press('Enter');
    const dialog = page.getByRole('dialog', { name: 'Serie anfordern: Nebelhafen' });
    const all = dialog.getByRole('checkbox', { name: 'Alle fehlenden Staffeln (3)' });
    await expect(all).toBeChecked();
    await expectFocusOn(page, all);
    await expect(
      dialog.getByText(/Noch 1 von 1 Staffeln frei|Das Limit ist erreicht/),
    ).toBeVisible();
    await expect(dialog.getByRole('button', { name: '3 Staffeln anfordern' })).toBeDisabled();

    // Space takes «Alle» off; season 2 alone fits.
    await page.keyboard.press('Space');
    await tabTo(page, dialog.getByRole('checkbox', { name: 'Staffel 2' }), 5);
    await page.keyboard.press('Space');
    const submit = dialog.getByRole('button', { name: '1 Staffel anfordern' });
    await expect(submit).toBeEnabled();
    await tabTo(page, submit, 10);
    await page.keyboard.press('Enter');
    await expect(dialog).toHaveCount(0);
    await expect(page.getByRole('link', { name: 'Freigabe ausstehend' })).toBeVisible();
    await expect(content(page)).not.toContainText('{{');
  } finally {
    await closeWishes(owner, setup);
  }
});

test('an unknown title says «nicht gefunden»', async ({ page, owner }) => {
  let setup: WishesSetup | undefined;
  try {
    setup = await openWishes(owner);
    await page.goto('/titles/movie/8999');
    await expect(page.getByText('Titel nicht gefunden')).toBeVisible();
    await page.goto('/titles/film/1');
    await expect(page.getByText('Titel nicht gefunden')).toBeVisible();
  } finally {
    await closeWishes(owner, setup);
  }
});
