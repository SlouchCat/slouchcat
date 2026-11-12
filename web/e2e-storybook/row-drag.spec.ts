import { expect, test, type Page } from '@playwright/test';

// Dragging a row with fixed paging buttons (concept 20 P-1, RowList arrows="always": the season
// shelf). The test series has two seasons only, which never overflow the shelf, so this runs on
// the story with a long row. A drag over 6 px scrolls the row and hits no card; a smaller move is
// still a click on the card.

const STORY = '/iframe.html?id=ui-row--fixed-arrows&viewMode=story';
// At 1280 px the story's row overflows by a few pixels only; at 800 px by several cards.
const DESKTOP = { width: 800, height: 720 };

/**
 * Counts the clicks that reach a card (a click the row swallows stops before the card) and stops
 * them there, so the story stays where it is.
 */
async function countCardClicks(page: Page) {
  await page.evaluate(() => {
    const state = window as unknown as { cardClicks: number };
    state.cardClicks = 0;
    document.addEventListener('click', (event) => {
      if (!(event.target instanceof Element) || !event.target.closest('li a')) return;
      state.cardClicks += 1;
      event.preventDefault();
    });
  });
}

const cardClicks = (page: Page) =>
  page.evaluate(() => (window as unknown as { cardClicks: number }).cardClicks);

test('a drag scrolls the row and opens no card; a small move still opens it', async ({ page }) => {
  await page.setViewportSize(DESKTOP);
  await page.goto(STORY);
  const row = page.getByRole('list', { name: 'Staffeln' });
  const scroller = row.locator('xpath=..');
  await expect(page.getByRole('button', { name: 'Weiterblättern' })).toBeEnabled();
  await countCardClicks(page);
  const scrollLeft = () => scroller.evaluate((element) => element.scrollLeft);

  const box = await row.getByRole('listitem').nth(1).boundingBox();
  if (!box) throw new Error('no card');
  const x = box.x + box.width / 2;
  const y = box.y + box.height / 2;

  // About 100 px to the left: the row moves, the card under the pointer does not open.
  await page.mouse.move(x, y);
  await page.mouse.down();
  for (let step = 1; step <= 10; step += 1) await page.mouse.move(x - step * 10, y);
  await page.mouse.up();
  await expect.poll(scrollLeft).toBeGreaterThan(50);
  expect(await cardClicks(page)).toBe(0);

  // 3 px and let go: a click on the card.
  const before = await scrollLeft();
  await page.mouse.move(x, y);
  await page.mouse.down();
  await page.mouse.move(x - 3, y);
  await page.mouse.up();
  await expect.poll(() => cardClicks(page)).toBe(1);
  expect(await scrollLeft()).toBe(before);
});

test('let go outside the row before a drag began: hovering later does not scroll it', async ({
  page,
}) => {
  await page.setViewportSize(DESKTOP);
  await page.goto(STORY);
  const row = page.getByRole('list', { name: 'Staffeln' });
  const scroller = row.locator('xpath=..');
  await expect(page.getByRole('button', { name: 'Weiterblättern' })).toBeEnabled();
  const box = await row.getByRole('listitem').nth(1).boundingBox();
  if (!box) throw new Error('no card');
  const x = box.x + box.width / 2;
  const y = box.y + box.height / 2;

  // Pressed on a card, out of the row within the 6 px (no capture yet), released below it.
  const outside = await scroller.boundingBox();
  if (!outside) throw new Error('no row');
  await page.mouse.move(x, y);
  await page.mouse.down();
  await page.mouse.move(x, outside.y + outside.height + 20);
  await page.mouse.up();
  const before = await scroller.evaluate((element) => element.scrollLeft);
  // Back over the row and across it with no button down.
  for (let step = 0; step <= 10; step += 1) await page.mouse.move(x - step * 10, y);
  expect(await scroller.evaluate((element) => element.scrollLeft)).toBe(before);
});

test('below 768 px the cards keep the geometry of a row with a heading; the arrows lie over it', async ({
  page,
}) => {
  await page.setViewportSize({ width: 375, height: 720 });
  const firstCard = () => page.getByRole('listitem').first().boundingBox();

  // A plain row: its heading and first card start at the gutter.
  await page.goto('/iframe.html?id=ui-row--default&viewMode=story');
  const heading = await page.getByRole('heading', { name: 'Neu hinzugefügt' }).boundingBox();
  const plain = await firstCard();

  await page.goto(STORY);
  const next = page.getByRole('button', { name: 'Weiterblättern' });
  await expect(next).toBeEnabled();
  const fixed = await firstCard();
  if (!heading || !plain || !fixed) throw new Error('no layout');
  expect(Math.abs(fixed.x - plain.x)).toBeLessThanOrEqual(1);
  expect(Math.abs(fixed.width - plain.width)).toBeLessThanOrEqual(1);
  expect(Math.abs(fixed.x - heading.x)).toBeLessThanOrEqual(1);

  // Both 44 px buttons inside the row's width, over its ends.
  const previous = await page.getByRole('button', { name: 'Zurückblättern' }).boundingBox();
  const after = await next.boundingBox();
  if (!previous || !after) throw new Error('no buttons');
  expect(previous.width).toBe(44);
  expect(previous.x).toBeGreaterThanOrEqual(0);
  expect(after.x + after.width).toBeLessThanOrEqual(375);
  expect(previous.x).toBeLessThan(fixed.x + fixed.width);
});
