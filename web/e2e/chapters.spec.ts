import type { Page } from '@playwright/test';
import { expect, test } from './fixtures';
import { expectFocusOn, expectVisibleFocus } from './keyboard';
import {
  arrowTo,
  CHAPTERS_TITLE,
  episodeIds,
  INTRO_SHOW_TITLE,
  itemIdByTitle,
  play,
  player,
  resetProgress,
  startOf,
  videoState,
} from './playback';

// W-9: a chapter jump lands within ±1 s of the chapter start; "Intro überspringen" shows during
// the detected intro and jumps to its end; from the outro (or the end) a 10-s countdown plays the
// next episode, unless a key cancels it.

const time = async (page: Page) => (await videoState(page)).time;

test('the chapter menu jumps to the chapter start (±1 s)', async ({ page, owner }) => {
  const itemId = await itemIdByTitle(owner, CHAPTERS_TITLE);
  await resetProgress(page.request, itemId, 10_000);
  const info = await play(page, itemId);
  expect(info.chapters.map((chapter) => chapter.title)).toEqual([
    'Kapitel 1: Anfang',
    'Kapitel 2: Mitte',
    'Kapitel 3: Ende',
  ]);
  // Paused, so the position stays where the jump put it.
  await page.keyboard.press('Space');
  await expect.poll(async () => (await videoState(page)).paused).toBe(true);

  await page.keyboard.press('c');
  const menu = page.getByRole('menu', { name: 'Kapitel' });
  await expect(menu).toBeVisible();
  await expectVisibleFocus(page);
  await expect(menu.getByRole('menuitemradio')).toHaveCount(3);
  await arrowTo(page, menu.getByRole('menuitemradio', { name: /^Kapitel 3: Ende/ }));
  await page.keyboard.press('Enter');
  await expect(menu).toHaveCount(0);
  const third = info.chapters[2].startMs / 1000;
  await expect.poll(() => time(page)).toBeGreaterThanOrEqual(third - 1);
  expect(await time(page)).toBeLessThanOrEqual(third + 1);

  // Back to the second chapter from the menu button, by keyboard as well.
  await page.keyboard.press('c');
  await expect(menu.getByRole('menuitemradio', { name: /^Kapitel 3: Ende/ })).toHaveAttribute(
    'aria-checked',
    'true',
  );
  await arrowTo(page, menu.getByRole('menuitemradio', { name: /^Kapitel 2: Mitte/ }), 'ArrowUp');
  await page.keyboard.press('Enter');
  const second = info.chapters[1].startMs / 1000;
  await expect.poll(() => time(page)).toBeLessThanOrEqual(second + 1);
  expect(await time(page)).toBeGreaterThanOrEqual(second - 1);
  // Escape closes the menu only; the player stays.
  await page.keyboard.press('c');
  await page.keyboard.press('Escape');
  await expect(menu).toHaveCount(0);
  await expect(page).toHaveURL(/\/play\//);
});

test.describe('intro show', () => {
  let episodes: string[] = [];

  test.beforeEach(async ({ owner }) => {
    if (episodes.length === 0) {
      const seriesId = await itemIdByTitle(owner, INTRO_SHOW_TITLE);
      episodes = await episodeIds(owner, seriesId);
      expect(episodes.length).toBeGreaterThanOrEqual(2);
    }
  });

  test('"Intro überspringen" appears during the intro and jumps to its end', async ({ page }) => {
    test.setTimeout(60_000);
    await resetProgress(page.request, episodes[0]);
    const info = await play(page, episodes[0]);
    const intro = info.segments.intro;
    expect(intro, 'the detected intro of S01E01').toBeDefined();

    const skip = page.getByRole('button', { name: 'Intro überspringen' });
    // The intro starts a few seconds in; jump close to it instead of waiting.
    await page.locator('video').evaluate(
      (video: HTMLVideoElement, at: number) => {
        video.currentTime = at;
      },
      intro!.startMs / 1000 + 1,
    );
    await expect(skip).toBeVisible({ timeout: 10_000 });
    await page.keyboard.press('s');
    const end = intro!.endMs / 1000;
    await expect.poll(() => time(page)).toBeGreaterThanOrEqual(end - 1);
    expect(await time(page)).toBeLessThanOrEqual(end + 2);
    await expect(skip).toHaveCount(0);
  });

  test('the countdown from the outro plays S01E02 (auto-next)', async ({ page }) => {
    test.setTimeout(60_000);
    await resetProgress(page.request, episodes[0]);
    await resetProgress(page.request, episodes[1]);
    const info = await play(page, episodes[0]);
    expect(info.nextEpisode?.id).toBe(episodes[1]);
    // Binge in full screen: the next episode stays in full screen (W-2).
    const inFullscreen = () => page.evaluate(() => document.fullscreenElement !== null);
    await page.keyboard.press('f');
    await expect.poll(inFullscreen).toBe(true);
    const at = info.segments.outro ? info.segments.outro.startMs / 1000 + 0.5 : undefined;
    await page.locator('video').evaluate((video: HTMLVideoElement, seconds: number | undefined) => {
      video.currentTime = seconds ?? Math.max(0, video.duration - 2);
    }, at);

    const countdown = page.getByRole('region', { name: 'Nächste Folge' });
    await expect(countdown).toBeVisible({ timeout: 10_000 });
    await expect(player(page)).toHaveAttribute('data-autonext', 'countdown');
    await expect(countdown.getByText(/^Startet in \d+ Sekunden?$/)).toBeVisible();
    await expect(countdown.getByText(/^S01E02 · /)).toBeVisible();

    const next = await startOf(page, () =>
      page.waitForURL(`**/play/${episodes[1]}`, { timeout: 20_000 }),
    );
    expect(next.status).toBe(200);
    expect(next.body.item.id).toBe(episodes[1]);
    await expect(player(page)).toHaveAttribute('data-autonext', 'navigated');
    await expect(player(page)).toHaveAttribute('data-start-ms', /\d+/, { timeout: 15_000 });
    await expectFocusOn(page, page.getByRole('button', { name: /^(Pause|Abspielen)$/ }));
    expect(await inFullscreen()).toBe(true);
    await expect(page.getByRole('button', { name: 'Vollbild beenden' })).toBeAttached();
  });

  test('Escape cancels the countdown; N plays the next episode at once', async ({ page }) => {
    test.setTimeout(60_000);
    await resetProgress(page.request, episodes[0]);
    const info = await play(page, episodes[0]);
    await page.locator('video').evaluate((video: HTMLVideoElement) => {
      video.currentTime = Math.max(0, video.duration - 2);
    });
    const countdown = page.getByRole('region', { name: 'Nächste Folge' });
    await expect(countdown).toBeVisible({ timeout: 10_000 });
    await page.keyboard.press('Escape');
    await expect(countdown).toHaveCount(0);
    await expect(player(page)).not.toHaveAttribute('data-autonext', /.+/);
    // Cancelled: no navigation after the ten seconds, and Escape did not leave the player.
    await page.waitForTimeout(11_000);
    await expect(page).toHaveURL(new RegExp(`/play/${episodes[0]}$`));

    const next = await startOf(page, () => page.keyboard.press('n'));
    expect(next.body.item.id).toBe(info.nextEpisode?.id);
    await expect(page).toHaveURL(new RegExp(`/play/${episodes[1]}$`));
  });
});
