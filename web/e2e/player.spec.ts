import { expect, test } from '@playwright/test';
import { H264_TITLE, itemPath } from './catalog';
import { expectFocusOn, expectNoFocusTrap, expectVisibleFocus, tabTo } from './keyboard';
import { arrowTo } from './playback';

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

test('player keys: Space, the zones of the bar (11.2), Tab and Escape', async ({ page }) => {
  // Enter on the detail page's play link opens the player with the focus on play/pause.
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  const player = page.getByRole('main', { name: 'Videoplayer' });
  await expect(player).toHaveAttribute('data-start-ms', /\d+/);
  const time = () => page.locator('video').evaluate((video: HTMLVideoElement) => video.currentTime);
  const paused = () => page.locator('video').evaluate((video: HTMLVideoElement) => video.paused);
  const row = page.getByRole('toolbar', { name: 'Wiedergabe' });
  const play = page.getByRole('button', { name: 'Abspielen' });
  const seek = page.getByRole('slider', { name: 'Wiedergabeposition' });

  // The play/pause button has focus; Space toggles.
  await expectFocusOn(page, page.getByRole('button', { name: 'Pause' }));
  await page.keyboard.press('Space');
  await expect.poll(paused).toBe(true);
  await expectFocusOn(page, play);

  // In the button row ←/→ go from button to button and leave the position alone.
  const duration = await page
    .locator('video')
    .evaluate((video: HTMLVideoElement) => video.duration);
  const before = await time();
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, page.getByRole('button', { name: '30 Sekunden vor' }));
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, play);
  expect(await time()).toBe(before);

  // ↑ leads to the timeline, where ←/→ seek 10 s; ↓ leads back to the same button.
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, seek);
  await page.keyboard.press('ArrowRight');
  // +10 s, or the end of the short test clip.
  await expect.poll(time).toBeGreaterThanOrEqual(Math.min(before + 9, duration - 0.1));
  await page.keyboard.press('ArrowLeft');
  await expect.poll(time).toBeLessThan(before + 5);
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, play);

  // Controls stay visible while paused. The row is one Tab stop: Shift+Tab goes back to the
  // timeline, the time display and the head (picture in picture only where the browser has it);
  // Tab from the row leaves it, and the page has no focus trap.
  await expect(player).toHaveAttribute('data-controls', 'visible');
  await page.keyboard.press('Shift+Tab');
  await expectFocusOn(page, seek);
  await page.keyboard.press('Shift+Tab');
  await expectFocusOn(page, page.getByRole('button', { name: /^\d+:\d+ \/ \d+:\d+$/ }));
  await page.keyboard.press('Shift+Tab');
  await expectFocusOn(page, page.getByRole('button', { name: 'Vollbild' }));
  const back = page.getByRole('button', { name: 'Zurück', exact: true });
  for (
    let presses = 0;
    presses < 2 && !(await back.evaluate((b) => b === document.activeElement));
    presses += 1
  ) {
    await page.keyboard.press('Shift+Tab');
  }
  await expectFocusOn(page, back);
  await tabTo(page, play, 5);
  await page.keyboard.press('Tab');
  await expect(row.locator(':focus')).toHaveCount(0);
  await expectNoFocusTrap(page);

  // ? opens the shortcut help with the focus on its list (the video plays on beside it); Escape
  // closes it and stays in the player, the next Escape leaves.
  await page.locator('body').focus();
  await page.keyboard.press('?');
  const help = page.getByRole('dialog', { name: 'Tastenkürzel' });
  await expect(help).toBeVisible();
  await expectFocusOn(page, help.locator('dl'));
  await page.keyboard.press('Escape');
  await expect(help).toHaveCount(0);
  await expect(page).toHaveURL(/\/play\//);

  await page.locator('body').focus();
  await page.keyboard.press('Escape');
  await expect(page).toHaveURL(new RegExp(`${detail}$`));
});

test('panels and the gear by keyboard: open, arrows, a page and back, Escape; no trap', async ({
  page,
}) => {
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  const player = page.getByRole('main', { name: 'Videoplayer' });
  await expect(player).toHaveAttribute('data-start-ms', /\d+/);
  await page.keyboard.press('Space');
  const rate = () =>
    page.locator('video').evaluate((video: HTMLVideoElement) => video.playbackRate);

  // The sound panel from the row: → walks to its button, Enter opens it on the track chosen; the
  // arrows stay in it, Escape gives the focus back to the button.
  const sound = page.getByRole('button', { name: /^Ton: / });
  await arrowTo(page, sound, 'ArrowRight');
  await page.keyboard.press('Enter');
  const soundPanel = page.getByRole('dialog', { name: 'Ton' });
  await expect(sound).toHaveAttribute('aria-expanded', 'true');
  await expectFocusOn(page, soundPanel.getByRole('option', { selected: true }));
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, soundPanel.getByRole('option', { selected: true }));
  await page.keyboard.press('Escape');
  await expect(soundPanel).toHaveCount(0);
  await expectFocusOn(page, sound);

  // U opens the subtitle panel on «Aus».
  await page.keyboard.press('u');
  const subtitles = page.getByRole('dialog', { name: 'Untertitel' });
  await expectFocusOn(page, subtitles.getByRole('option', { name: 'Aus' }));
  await page.keyboard.press('Backspace');
  await expect(subtitles).toHaveCount(0);
  await expect(page).toHaveURL(/\/play\//);

  // The gear: its first page leads to the speed page; Enter picks a speed; Backspace goes up,
  // Escape closes with the focus back on the gear.
  const gear = page.getByRole('button', { name: 'Einstellungen' });
  await arrowTo(page, gear, 'ArrowRight');
  await page.keyboard.press('Enter');
  const settings = page.getByRole('dialog', { name: 'Einstellungen' });
  const speedRow = settings.getByRole('option', { name: 'Geschwindigkeit' });
  await expect(speedRow).toHaveAccessibleDescription('1×');
  await arrowTo(page, speedRow);
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  const speed = page.getByRole('dialog', { name: 'Geschwindigkeit' });
  await expect(speed.getByRole('button', { name: 'Zurück' })).toBeVisible();
  await expectFocusOn(page, speed.getByRole('option', { name: '1×' }));
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, speed.getByRole('option', { name: '1,25×' }));
  await page.keyboard.press('Enter');
  await expect.poll(rate).toBe(1.25);
  await page.keyboard.press('Backspace');
  await expect(speedRow).toHaveAccessibleDescription('1,25×');
  await expectVisibleFocus(page);

  // The sound options: Space switches night mode on (Web Audio, Chromium and Firefox).
  const soundRow = settings.getByRole('option', { name: 'Ton-Optionen' });
  await arrowTo(page, soundRow);
  await page.keyboard.press('Enter');
  const night = page.getByRole('dialog', { name: 'Ton-Optionen' }).getByRole('switch', {
    name: 'Nachtmodus',
  });
  await expect(night).toBeFocused();
  await page.keyboard.press('Space');
  await expect(night).toBeChecked();
  await page.keyboard.press('Backspace');
  await expect(soundRow).toHaveAccessibleDescription('Nachtmodus an');
  await page.keyboard.press('Escape');
  await expect(settings).toHaveCount(0);
  await expectFocusOn(page, gear);

  // A panel is no trap: Tab walks out of it and on to the end of the page.
  await page.keyboard.press('Enter');
  await expect(settings).toBeVisible();
  await expectNoFocusTrap(page);
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

  // Faded out, the bar has no zone: → seeks although play/pause has the focus, which stays.
  const time = () => page.locator('video').evaluate((video: HTMLVideoElement) => video.currentTime);
  const duration = await page
    .locator('video')
    .evaluate((video: HTMLVideoElement) => video.duration);
  await page.keyboard.press('0');
  await expect(player).toHaveAttribute('data-controls', 'hidden', { timeout: 6000 });
  const before = await time();
  await page.keyboard.press('ArrowRight');
  await expect.poll(time).toBeGreaterThanOrEqual(Math.min(before + 9, duration - 0.1));
  await expectFocusOn(page, page.getByRole('button', { name: /^(Pause|Abspielen)$/ }));
  await expect(player).toHaveAttribute('data-controls', 'visible');
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
