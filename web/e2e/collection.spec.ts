import { createCollection, deleteCollection } from './curation';
import { ROW_TITLES } from './discover';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, expectVisibleFocus, resetFocus } from './keyboard';
import { CHAPTERS_TITLE, H264_TITLE, itemIdByTitle } from './playback';

// E-6: a manual collection of the main user (admin API) as a page of its own and as a card in the
// start page's row "Sammlungen" and on its members' detail pages.

const NAME = 'E2E Sammlung Abendprogramm';

test('a manual collection: page, start page row and detail page (E-6)', async ({ page, owner }) => {
  const h264 = await itemIdByTitle(owner, H264_TITLE);
  const chapters = await itemIdByTitle(owner, CHAPTERS_TITLE);
  const { id } = await createCollection(owner, NAME, [h264, chapters]);
  try {
    await page.goto(`/collections/${id}`);
    await expect(page.getByRole('heading', { level: 1, name: NAME })).toBeVisible();
    await expect(page.getByText('2 Titel')).toBeVisible();
    const members = page.getByRole('list', { name: 'Titel' }).getByRole('link');
    // The admin's order.
    await expect(members).toHaveCount(2);
    await expect(members.first()).toContainText(H264_TITLE);
    await expect(members.nth(1)).toContainText(CHAPTERS_TITLE);
    await expect(page).toHaveTitle(`${NAME} · Slouchcat`);

    await page.goto('/');
    const row = page.getByRole('region', { name: ROW_TITLES.collections });
    await expect(row.getByRole('link', { name: new RegExp(NAME) })).toHaveAttribute(
      'href',
      `/collections/${id}`,
    );

    await page.goto(`/items/${h264}`);
    const section = page.getByRole('region', { name: 'Sammlungen' });
    await section.getByRole('link', { name: new RegExp(NAME) }).click();
    await expect(page).toHaveURL(new RegExp(`/collections/${id}$`));
  } finally {
    await deleteCollection(owner, id);
  }
  await page.goto(`/collections/${id}`);
  await expect(page.getByText('Sammlung nicht gefunden')).toBeVisible();
});

test('collection by keyboard: arrows, Enter, back; no focus trap', async ({ page, owner }) => {
  const { id } = await createCollection(owner, NAME, [
    await itemIdByTitle(owner, H264_TITLE),
    await itemIdByTitle(owner, CHAPTERS_TITLE),
  ]);
  try {
    await page.goto(`/collections/${id}`);
    const members = page.getByRole('list', { name: 'Titel' }).getByRole('link');
    await expect(members).toHaveCount(2);
    await resetFocus(page);
    await page.keyboard.press('ArrowDown');
    await expectFocusOn(page, members.first());
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, members.nth(1));
    await page.keyboard.press('ArrowLeft');
    await expectFocusOn(page, members.first());
    // Up from the first column leaves the grid for "Zurück" above it.
    await page.keyboard.press('ArrowUp');
    await expectFocusOn(page, page.getByRole('button', { name: 'Zurück' }));
    await page.keyboard.press('ArrowDown');
    await expectVisibleFocus(page);

    await members.first().focus();
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/items\//);
    await page.goBack();
    await expect(page.getByRole('heading', { level: 1, name: NAME })).toBeVisible();
    await expectNoFocusTrap(page);

    // Backspace goes back; opened directly, that is the start page.
    await resetFocus(page);
    await page.keyboard.press('Backspace');
    await expect(page).toHaveURL((url) => url.pathname === '/');
  } finally {
    await deleteCollection(owner, id);
  }
});
