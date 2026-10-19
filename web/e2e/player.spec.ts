import { expect, test } from '@playwright/test';
import { H264_TITLE, itemPath } from './catalog';
import { expectFocusOn, expectNoFocusTrap, expectVisibleFocus } from './keyboard';

/** DoD M1a: direct play starts in under 2 s in Chrome and Firefox. */
const START_BUDGET_MS = 2000;

let detail = '';

test.beforeEach(async ({ page }) => {
  detail ||= await itemPath(page, H264_TITLE);
  await page.goto('/');
  await page.goto(detail);
  await expect(page.getByRole('heading', { level: 1, name: H264_TITLE })).toBeVisible();
});

test('Enter on play starts direct play in under 2 s and reports progress', async ({ page }) => {
  const progress = page.waitForRequest(
    (request) =>
      request.method() === 'POST' && /\/api\/v1\/items\/[^/]+\/progress$/.test(request.url()),
  );
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/play\//);

  const player = page.getByRole('main', { name: 'Videoplayer' });
  await expect(player).toHaveAttribute('data-start-ms', /\d+/);
  const startMs = Number(await player.getAttribute('data-start-ms'));
  test.info().annotations.push({ type: 'start-ms', description: String(startMs) });
  expect(startMs).toBeLessThan(START_BUDGET_MS);

  // Pausing sends a progress event right away.
  await page.keyboard.press('Space');
  const event = (await progress).postDataJSON() as { state: string; seq: number };
  expect(event.state).toBe('paused');
  expect(event.seq).toBeGreaterThanOrEqual(1);
});

test('player keys: Space and arrows; controls are reachable by Tab; Escape returns', async ({
  page,
}) => {
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  const player = page.getByRole('main', { name: 'Videoplayer' });
  await expect(player).toHaveAttribute('data-start-ms', /\d+/);
  const time = () => page.locator('video').evaluate((video: HTMLVideoElement) => video.currentTime);
  const paused = () => page.locator('video').evaluate((video: HTMLVideoElement) => video.paused);

  // The play/pause button has focus; Space toggles.
  await expectFocusOn(page, page.getByRole('button', { name: 'Pause' }));
  await page.keyboard.press('Space');
  await expect.poll(paused).toBe(true);
  await expectFocusOn(page, page.getByRole('button', { name: 'Abspielen' }));

  const duration = await page
    .locator('video')
    .evaluate((video: HTMLVideoElement) => video.duration);
  const before = await time();
  await page.keyboard.press('ArrowRight');
  // +10 s, or the end of the short test clip.
  await expect.poll(time).toBeGreaterThanOrEqual(Math.min(before + 9, duration - 0.1));
  await page.keyboard.press('ArrowLeft');
  await expect.poll(time).toBeLessThan(before + 5);

  // Controls stay visible while paused and every control is reachable by Tab.
  await expect(player).toHaveAttribute('data-controls', 'visible');
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('button', { name: 'Vollbild' }));
  await page.keyboard.press('Shift+Tab');
  await page.keyboard.press('Shift+Tab');
  await expectFocusOn(page, page.getByRole('slider', { name: 'Wiedergabeposition' }));
  await page.keyboard.press('Shift+Tab');
  await expectFocusOn(page, page.getByRole('button', { name: 'Zurück' }));
  await expectNoFocusTrap(page);

  await page.locator('body').focus();
  await page.keyboard.press('Escape');
  await expect(page).toHaveURL(new RegExp(`${detail}$`));
});

test('controls hide after 3 s of playback and come back on a key press', async ({ page }) => {
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  const player = page.getByRole('main', { name: 'Videoplayer' });
  await expect(player).toHaveAttribute('data-start-ms', /\d+/);
  await expect(player).toHaveAttribute('data-controls', 'hidden', { timeout: 6000 });
  await page.keyboard.press('Shift');
  await expect(player).toHaveAttribute('data-controls', 'visible');
  await expectVisibleFocus(page);
});

test('Backspace and browser back return to the detail page', async ({ page }) => {
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/play\//);
  await page.keyboard.press('Backspace');
  await expect(page).toHaveURL(new RegExp(`${detail}$`));
  await page.goForward();
  await expect(page).toHaveURL(/\/play\//);
  await page.goBack();
  await expect(page).toHaveURL(new RegExp(`${detail}$`));
});
