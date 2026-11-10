import type { Locator, Page } from '@playwright/test';
import { audioItemId, audioPlayer, M4B_TITLE, playAudio, setAudioProgress } from './audio';
import { H264_TITLE, idOf, itemPath } from './catalog';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, resetFocus, tabTo } from './keyboard';
import {
  audio,
  miniOpen,
  miniPlayer,
  playStation,
  resetRadioPreferences,
  stopRadio,
} from './radio';

// Concept 19 part E (volume). PE-1: with a pointer the audio player's volume slider opens as a
// popover over its neighbours: hovering the mute button at 900 to 1100 px neither changes the row
// of extras nor makes the slider flicker (the old drawer grew inside the row, the row wrapped, the
// control jumped away from the pointer and closed again); 300 ms after the pointer leaves it
// closes. PE-2: on a touch screen at 375 px the slider stays open in the row, covers nothing and
// the page does not scroll sideways, in the audio player and the mini player. PE-3: one volume per
// device for the radio and the audio player, kept across a reload; the video player keeps its
// own. PE-5: the volume by keyboard in the mini player, «Jetzt läuft» and the audio player: Tab,
// arrows, M, Escape closes the popover only. The video player's keyboard tests (player.spec,
// offline-playback.spec) cover it there.

const STATION = 'Testradio Drei';

let m4b = '';

test.beforeEach(async ({ owner }) => {
  m4b ||= await audioItemId(owner, M4B_TITLE);
  await stopRadio(owner);
  await resetRadioPreferences(owner);
});

test.afterEach(async ({ owner }) => {
  await stopRadio(owner);
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

/** The page scrolls sideways. */
const scrollsSideways = (page: Page) =>
  page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth);

/** The names of `others` whose boxes overlap `target`'s box. */
async function overlaps(target: Locator, others: Locator): Promise<string[]> {
  const box = await target.boundingBox();
  if (!box) throw new Error('the target has no box');
  const hits: string[] = [];
  for (const other of await others.all()) {
    const rect = await other.boundingBox();
    if (!rect || rect.width === 0) continue;
    const apart =
      rect.x >= box.x + box.width ||
      rect.x + rect.width <= box.x ||
      rect.y >= box.y + box.height ||
      rect.y + rect.height <= box.y;
    if (!apart) hits.push((await other.getAttribute('aria-label')) ?? (await other.innerText()));
  }
  return hits;
}

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

test('PE-3: one volume for radio and audiobook, kept after a reload; the video player keeps its own', async ({
  page,
}) => {
  test.setTimeout(120_000);
  await playStation(page, STATION);
  // 40 % on the radio's slider, by keyboard: Home, then PageUp four times (10 % each).
  await resetFocus(page);
  await tabTo(page, muteButton(miniPlayer(page)), 80);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, volumeSlider(miniPlayer(page)));
  await page.keyboard.press('Home');
  for (let presses = 0; presses < 4; presses += 1) await page.keyboard.press('PageUp');
  await expect(volumeSlider(miniPlayer(page))).toHaveAttribute('aria-valuenow', '40');
  await expect
    .poll(() => audio(page).evaluate((element: HTMLAudioElement) => element.volume))
    .toBe(0.4);

  // An audiobook starts at 40 % (and stops the radio).
  await setAudioProgress(page.request, m4b, 0, 60_000, 1);
  await playAudio(page, m4b);
  const bookVolume = () =>
    audioPlayer(page)
      .locator('audio')
      .evaluate((element: HTMLAudioElement) => element.volume);
  await expect.poll(bookVolume).toBe(0.4);
  await expect(volumeSlider(audioPlayer(page))).toHaveAttribute('aria-valuenow', '40');
  // After a reload still 40 %.
  await page.reload();
  await expect(audioPlayer(page)).toHaveAttribute('data-start-ms', /\d+/, { timeout: 20_000 });
  await expect.poll(bookVolume).toBe(0.4);
  await expect(volumeSlider(audioPlayer(page))).toHaveAttribute('aria-valuenow', '40');
  await setAudioProgress(page.request, m4b, 0, 60_000, 1);

  // The video player keeps its own value: 100 % in this fresh browser.
  const video = idOf(await itemPath(page, H264_TITLE));
  await page.goto(`/play/${video}`);
  const videoPlayer = page.getByRole('main', { name: 'Videoplayer' });
  await expect(videoPlayer).toHaveAttribute('data-start-ms', /\d+/);
  await expect(volumeSlider(videoPlayer)).toHaveAttribute('aria-valuenow', '100');
  await expect
    .poll(() => page.locator('video').evaluate((element: HTMLVideoElement) => element.volume))
    .toBe(1);
});

test('PE-5: the audio player’s volume by keyboard: Tab opens it, arrows, M, Escape closes only it @keyboard', async ({
  page,
}) => {
  await setAudioProgress(page.request, m4b, 0, 60_000, 1);
  await playAudio(page, m4b);
  const player = audioPlayer(page);
  const mute = muteButton(player);
  const slider = volumeSlider(player);
  const element = () =>
    player.locator('audio').evaluate((node: HTMLAudioElement) => ({
      volume: node.volume,
      muted: node.muted,
    }));
  expect(await sliderOpen(slider)).toBe(false);
  await player.getByRole('button', { name: /^Sleep-Timer/ }).focus();
  await tabTo(page, mute, 2);
  await expect.poll(() => sliderOpen(slider)).toBe(true);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, slider);
  const level = Number(await slider.getAttribute('aria-valuenow'));
  await page.keyboard.press('ArrowLeft');
  await expect(slider).toHaveAttribute('aria-valuenow', String(level - 5));
  expect((await element()).volume).toBeCloseTo((level - 5) / 100, 2);
  // M mutes and unmutes from the slider.
  await page.keyboard.press('m');
  await expect(mute).toHaveAttribute('aria-pressed', 'true');
  expect((await element()).muted).toBe(true);
  await page.keyboard.press('m');
  await expect(mute).toHaveAttribute('aria-pressed', 'false');
  // Escape closes the popover, the focus goes back to the button; the player stays.
  await page.keyboard.press('Escape');
  await expectFocusOn(page, mute);
  await expect.poll(() => sliderOpen(slider)).toBe(false);
  await expect(page).toHaveURL(/\/play\//);
  // Tab opens it again; no focus trap.
  await page.keyboard.press('Tab');
  await expectFocusOn(page, slider);
  await expect.poll(() => sliderOpen(slider)).toBe(true);
  await expectNoFocusTrap(page);
  // With the popover closed, Escape leaves the player as before.
  await mute.focus();
  await page.keyboard.press('Escape');
  await expect.poll(() => sliderOpen(slider)).toBe(false);
  await page.keyboard.press('Escape');
  await expect(page).not.toHaveURL(/\/play\//);
  await setAudioProgress(page.request, m4b, 0, 60_000, 1);
});

test('PE-5: the radio’s volume by keyboard in the mini player and «Jetzt läuft» @keyboard', async ({
  page,
}) => {
  test.setTimeout(120_000);
  await playStation(page, STATION);
  const mini = miniPlayer(page);
  const slider = volumeSlider(mini);
  const radioAudio = () =>
    audio(page).evaluate((node: HTMLAudioElement) => ({ volume: node.volume, muted: node.muted }));
  // Tab: play/pause, then the mute button, which opens the slider above the bar.
  await resetFocus(page);
  await tabTo(page, page.getByTestId('radio-mini-toggle'), 80);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, muteButton(mini));
  await expect.poll(() => sliderOpen(slider)).toBe(true);
  const sliderBox = (await slider.boundingBox())!;
  const barBox = (await mini.boundingBox())!;
  expect(sliderBox.y + sliderBox.height).toBeLessThanOrEqual(barBox.y + 1);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, slider);
  await page.keyboard.press('End');
  await page.keyboard.press('ArrowLeft');
  await expect(slider).toHaveAttribute('aria-valuenow', '95');
  await expect.poll(async () => (await radioAudio()).volume).toBe(0.95);
  // M mutes while the focus is in the mini player.
  await page.keyboard.press('m');
  await expect.poll(async () => (await radioAudio()).muted).toBe(true);
  await expect(muteButton(mini)).toHaveAttribute('aria-pressed', 'true');
  await page.keyboard.press('m');
  await expect.poll(async () => (await radioAudio()).muted).toBe(false);
  // Escape closes the popover only; the radio plays on.
  await page.keyboard.press('Escape');
  await expectFocusOn(page, muteButton(mini));
  await expect.poll(() => sliderOpen(slider)).toBe(false);
  await expect(mini).toHaveAttribute('data-state', 'live');

  // «Jetzt läuft»: the volume under the controls, the same value; M mutes there too.
  await miniOpen(page).focus();
  await page.keyboard.press('Enter');
  const now = page.getByRole('dialog', { name: 'Jetzt läuft' });
  await expect(now).toBeVisible();
  const nowVolume = now.getByTestId('radio-now-volume');
  await tabTo(page, muteButton(nowVolume), 20);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, volumeSlider(nowVolume));
  await expect(volumeSlider(nowVolume)).toHaveAttribute('aria-valuenow', '95');
  await page.keyboard.press('PageDown');
  await expect.poll(async () => (await radioAudio()).volume).toBe(0.85);
  await page.keyboard.press('M');
  await expect.poll(async () => (await radioAudio()).muted).toBe(true);
  await page.keyboard.press('M');
  await expect.poll(async () => (await radioAudio()).muted).toBe(false);
  // The first Escape closes the popover, the second the dialog.
  await page.keyboard.press('Escape');
  await expect(now).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(now).toHaveCount(0);
  await expectFocusOn(page, miniOpen(page));
  await expect(volumeSlider(mini)).toHaveAttribute('aria-valuenow', '85');
});

test.describe('on a touch screen at 375 px', () => {
  test.use({ viewport: { width: 375, height: 812 }, hasTouch: true, isMobile: true });

  test('PE-2: the slider stays in the row, covers nothing, no sideways scrolling', async ({
    page,
  }) => {
    test.setTimeout(120_000);
    await setAudioProgress(page.request, m4b, 0, 60_000, 1);
    await playAudio(page, m4b);
    const player = audioPlayer(page);
    const slider = volumeSlider(player);
    // Open without a touch, in the row: no popover.
    expect(await sliderOpen(slider)).toBe(true);
    await expect(player.locator('[popover]')).toHaveCount(0);
    const row = player.getByRole('button', { name: /^(Sleep-Timer|Kapitel|Geschwindigkeit)/ });
    expect(await overlaps(slider, row)).toEqual([]);
    expect(await scrollsSideways(page)).toBe(false);
    await setAudioProgress(page.request, m4b, 0, 60_000, 1);

    // The mini player: the slider on a line of its own in the dock, over nothing.
    await playStation(page, STATION);
    const mini = miniPlayer(page);
    const miniSlider = volumeSlider(mini);
    await expect.poll(() => sliderOpen(miniSlider)).toBe(true);
    await expect(mini.locator('[popover]')).toHaveCount(0);
    expect(await overlaps(miniSlider, mini.getByRole('button'))).toEqual([]);
    const sliderBox = (await miniSlider.boundingBox())!;
    const barBox = (await mini.boundingBox())!;
    expect(sliderBox.x).toBeGreaterThanOrEqual(barBox.x);
    expect(sliderBox.x + sliderBox.width).toBeLessThanOrEqual(barBox.x + barBox.width);
    expect(sliderBox.y + sliderBox.height).toBeLessThanOrEqual(barBox.y + barBox.height);
    expect(await scrollsSideways(page)).toBe(false);
  });
});
