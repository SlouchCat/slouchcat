import { type Page } from '@playwright/test';
import { feedKey, PODCAST_TITLE, subscribeFeed, unsubscribe } from './audio';
import { H264_TITLE } from './catalog';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, resetFocus } from './keyboard';
import { arrowTo, INTRO_SHOW_TITLE, itemIdByTitle } from './playback';

// E-8: series and podcasts can be favourites (the button on their detail page); new episodes of a
// favourite notify the profile (notifications.spec.ts).

const favorite = (page: Page) => page.getByRole('button', { name: 'Favorit' });

async function favorites(page: Page): Promise<string[]> {
  const response = await page.request.get('/api/v1/favorites');
  expect(response.status(), await response.text()).toBe(200);
  return ((await response.json()) as { title: string }[]).map((item) => item.title);
}

test('a series and a podcast become favourites; a film cannot (E-8)', async ({
  page,
  owner,
}, testInfo) => {
  const series = await itemIdByTitle(owner, INTRO_SHOW_TITLE);
  await page.goto(`/items/${series}`);
  await expect(favorite(page)).toHaveAttribute('aria-pressed', 'false');
  await favorite(page).click();
  await expect(favorite(page)).toHaveAttribute('aria-pressed', 'true');
  await page.reload();
  await expect(favorite(page)).toHaveAttribute('aria-pressed', 'true');
  expect(await favorites(page)).toEqual([INTRO_SHOW_TITLE]);

  const podcast = await subscribeFeed(owner, feedKey(testInfo), 2);
  try {
    await page.goto(`/items/${podcast.id}`);
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
    await favorite(page).click();
    await expect(favorite(page)).toHaveAttribute('aria-pressed', 'true');
    expect(await favorites(page)).toHaveLength(2);
    expect(await favorites(page)).toContain(podcast.title || PODCAST_TITLE);
  } finally {
    await unsubscribe(owner, podcast.id);
  }

  // Off again.
  await page.goto(`/items/${series}`);
  await favorite(page).click();
  await expect(favorite(page)).toHaveAttribute('aria-pressed', 'false');

  // A film has no favourite button, and the API refuses it (422 not_favoritable).
  const film = await itemIdByTitle(owner, H264_TITLE);
  await page.goto(`/items/${film}`);
  await expect(page.getByRole('button', { name: 'Merken' })).toBeVisible();
  await expect(favorite(page)).toHaveCount(0);
  const refused = await page.request.put(`/api/v1/items/${film}/favorite`, {
    headers: { 'X-Requested-With': 'Slouchcat' },
  });
  expect(refused.status()).toBe(422);
  expect(((await refused.json()) as { code: string }).code).toBe('not_favoritable');
});

test('favourite by keyboard: arrows to the button, Enter toggles; no focus trap', async ({
  page,
  owner,
}) => {
  await page.goto(`/items/${await itemIdByTitle(owner, INTRO_SHOW_TITLE)}`);
  await expect(favorite(page)).toBeVisible();
  await resetFocus(page);
  // ↓ lands on "Zurück" (a series has no play button), ↓ again on the row under the actions:
  // "Merken", then → "Favorit".
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, page.getByRole('button', { name: 'Zurück' }));
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, page.getByRole('button', { name: 'Merken' }));
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, favorite(page));
  await page.keyboard.press('Enter');
  await expect(favorite(page)).toHaveAttribute('aria-pressed', 'true');
  await expectFocusOn(page, favorite(page));
  await page.keyboard.press('Enter');
  await expect(favorite(page)).toHaveAttribute('aria-pressed', 'false');

  // "Merken" sits left of it and toggles the same way.
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, page.getByRole('button', { name: 'Merken' }));
  await page.keyboard.press('Enter');
  await expectFocusOn(page, page.getByRole('button', { name: 'Gemerkt' }));

  // Down leaves the actions for the seasons.
  await arrowTo(
    page,
    page.getByRole('group', { name: 'Staffel wählen' }).getByRole('button').first(),
  );
  await expectNoFocusTrap(page);
});
