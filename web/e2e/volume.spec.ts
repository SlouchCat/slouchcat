import type { Locator, Page } from '@playwright/test';
import { audioItemId, audioPlayer, M4B_TITLE, playAudio, setAudioProgress } from './audio';
import { expect, test } from './fixtures';

// Concept 19 part E (volume). PE-1: with a pointer the audio player's volume slider opens as a
// popover over its neighbours: hovering the mute button at 900 to 1100 px neither changes the row
// of extras nor makes the slider flicker (the old drawer grew inside the row, the row wrapped, the
// control jumped away from the pointer and closed again); 300 ms after the pointer leaves it
// closes.

let m4b = '';

test.beforeEach(async ({ owner }) => {
  m4b ||= await audioItemId(owner, M4B_TITLE);
});

const volumeSlider = (scope: Page | Locator) => scope.getByRole('slider', { name: 'Lautstärke' });
const muteButton = (scope: Page | Locator) => scope.getByRole('button', { name: 'Ton aus' });

/**
 * Whether the slider shows: its popover or drawer is open (closed it is a hidden pixel, the old
 * drawer had no width).
 */
const sliderOpen = (slider: Locator) =>
  slider.evaluate((element) => element.parentElement!.getBoundingClientRect().width > 20);

/**
 * Frames of the extras row while the pointer rests: how many lines it has, where the mute button
 * is and whether the slider shows.
 */
const watchRow = (page: Page, frames: number) =>
  page.evaluate(async (count) => {
    const button = document.querySelector<HTMLElement>('button[aria-label="Ton aus"]')!;
    const row = button.closest('[class*="extras"]')!;
    const seen: { lines: number; at: string; open: boolean }[] = [];
    for (let frame = 0; frame < count; frame += 1) {
      if (frame > 0) await new Promise((resolve) => requestAnimationFrame(resolve));
      const tops = [...row.children].map((child) => Math.round(child.getBoundingClientRect().top));
      const rect = button.getBoundingClientRect();
      const slider = document.querySelector('[role="slider"][aria-label="Lautstärke"]');
      seen.push({
        lines: new Set(tops).size,
        at: `${Math.round(rect.left)},${Math.round(rect.top)}`,
        open: (slider?.parentElement?.getBoundingClientRect().width ?? 0) > 20,
      });
    }
    return seen;
  }, frames);

test('PE-1: the volume opens over its neighbours at 900 to 1100 px and closes 300 ms after', async ({
  page,
}) => {
  await setAudioProgress(page.request, m4b, 0, 60_000, 1);
  for (const width of [900, 1000, 1100]) {
    await page.setViewportSize({ width, height: 800 });
    await playAudio(page, m4b);
    const player = audioPlayer(page);
    const slider = volumeSlider(page);
    const [rest] = await watchRow(page, 1);
    expect(rest.open, `${width} px: closed at rest`).toBe(false);
    await muteButton(player).hover();
    await expect.poll(() => sliderOpen(slider)).toBe(true);
    // Watch the row for 40 frames: it keeps its lines, the button stays under the pointer and the
    // slider stays open.
    const frames = await watchRow(page, 40);
    expect(
      frames.filter((frame) => frame.lines !== rest.lines || frame.at !== rest.at),
      `${width} px: the row does not change`,
    ).toEqual([]);
    expect(
      frames.filter((frame) => !frame.open),
      `${width} px: the slider stays open`,
    ).toEqual([]);
    // The popover lies inside the window.
    const box = (await slider.boundingBox())!;
    expect(box.x + box.width).toBeLessThanOrEqual(width);
    // The pointer leaves: still open a moment later, closed after 300 ms.
    const left = Date.now();
    await page.mouse.move(5, 5);
    await page.waitForTimeout(150);
    expect(await sliderOpen(slider), `${width} px: open 150 ms after`).toBe(true);
    await expect.poll(() => sliderOpen(slider), { timeout: 2000 }).toBe(false);
    expect(Date.now() - left).toBeGreaterThanOrEqual(300);
  }
  await setAudioProgress(page.request, m4b, 0, 60_000, 1);
});
