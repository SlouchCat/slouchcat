import type { Page } from '@playwright/test';
import { EXTRAS_TITLE, H264_TITLE } from './catalog';
import { createPick } from './curation';
import { ROW_TITLES } from './discover';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, resetFocus, tabTo } from './keyboard';
import { itemIdByTitle } from './playback';

// Concept 08, 7.3/7.4: every one of Cat's recommendations as a grid with Cat's note; the start
// page row links it with "Alle anzeigen ›". The `owner` fixture leaves only the baseline pick (the
// H264 film without note) and deletes the one created here afterwards.

const NOTE = 'E2E: unbedingt mit Ton schauen.';
const grid = (page: Page) => page.getByRole('list', { name: ROW_TITLES.picks });
const cards = (page: Page) => grid(page).getByRole('link');

test("Cat's recommendations by keyboard: the note, arrows, Enter, back; no focus trap", async ({
  page,
  owner,
}) => {
  await createPick(owner, { itemId: await itemIdByTitle(owner, EXTRAS_TITLE), note: NOTE });

  // The start page row links the page.
  await page.goto('/');
  const row = page.getByRole('region', { name: ROW_TITLES.picks });
  const all = row.getByRole('link', { name: 'Alle anzeigen ›' });
  await expect(all).toHaveAttribute('href', '/picks');
  await all.click();
  await expect(page).toHaveURL('/picks');
  // Fresh load for the keyboard part: after the pointer click Firefox draws no :focus-visible ring
  // on the first script focus, so the arrow steps below would read as rings missing.
  await page.goto('/picks');
  await expect(page.getByRole('heading', { level: 1, name: ROW_TITLES.picks })).toBeVisible();
  await expect(page.getByText('2 Titel', { exact: true })).toBeVisible();
  // Cat's order: the baseline pick first, the new one with its note under the poster.
  await expect(cards(page)).toHaveCount(2);
  await expect(cards(page).first()).toContainText(H264_TITLE);
  await expect(cards(page).nth(1)).toContainText(EXTRAS_TITLE);
  await expect(cards(page).nth(1).getByText(NOTE)).toBeVisible();
  await expect(page.getByRole('button', { name: /Mehr laden/ })).toHaveCount(0);

  // The first arrow lands on the first card; right and left move between the cards.
  await resetFocus(page);
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, cards(page).first());
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, cards(page).nth(1));
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, cards(page).first());
  await page.keyboard.press('ArrowRight');

  // Enter opens the title; the browser's back lands on the same card.
  const href = await cards(page).nth(1).getAttribute('href');
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(href ?? /\/items\//);
  await page.goBack();
  await expect(page).toHaveURL('/picks');
  // The card gets the focus by script after the navigation; Firefox draws no :focus-visible ring
  // for that until the next key, so the ring is checked after an arrow step.
  await expect(cards(page).nth(1)).toBeFocused();
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, cards(page).first());

  // Tab from a fresh load: header, "Zurück", the cards; then the end of the page.
  await page.reload();
  await expect(cards(page)).toHaveCount(2);
  await resetFocus(page);
  await tabTo(page, page.getByRole('button', { name: 'Zurück' }));
  await tabTo(page, cards(page).first(), 3);
  await expectNoFocusTrap(page);

  // Escape goes back; opened directly, that is the start page.
  await resetFocus(page);
  await page.keyboard.press('Escape');
  await expect(page).toHaveURL((url) => url.pathname === '/');
});
