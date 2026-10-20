import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, expectVisibleFocus, tabTo } from './keyboard';
import {
  arrowTo,
  itemIdByTitle,
  MULTI_AUDIO_TITLE,
  PGS_TITLE,
  play,
  player,
  recordEndedReason,
  resetProgress,
  startOf,
  videoState,
} from './playback';

// W-4: audio and subtitle tracks. A switch is a new playback session at the current position
// (≤ 3 s interruption, data-switch-ms); text subtitles arrive as WebVTT through <track>, bitmap
// subtitles (PGS) are burned in by a transcode in Chromium and Firefox alike (10.3).

const SWITCH_BUDGET_MS = 3000;

const tracksMenu = (page: import('@playwright/test').Page) =>
  page.getByRole('menu', { name: 'Spuren' });

test('switching the audio track to AC3 English: new session within 3 s, Stream-Info shows it', async ({
  page,
  owner,
  browserName,
}) => {
  test.setTimeout(60_000);
  const itemId = await itemIdByTitle(owner, MULTI_AUDIO_TITLE);
  await resetProgress(page.request, itemId);
  const first = await play(page, itemId);
  const ac3 = first.tracks.audio.find((track) => track.codec === 'ac3');
  expect(ac3, 'the AC3 track of the test film').toBeDefined();

  // T opens the track menu; the current audio track has the tick.
  await page.keyboard.press('t');
  const menu = tracksMenu(page);
  await expect(menu).toBeVisible();
  await expectVisibleFocus(page);
  await expect(menu.getByText('Ton', { exact: true })).toBeVisible();
  await expect(menu.getByText('Untertitel', { exact: true })).toBeVisible();
  const target = menu.getByRole('menuitemradio', { name: /AC3/ });
  await arrowTo(page, target);
  await expectVisibleFocus(page);

  const endedReason = await recordEndedReason(page);
  // The new session starts where the old one was (W-4): the position of the switch request
  // matches the player's time just before the switch.
  const beforeMs = (await videoState(page)).time * 1000;
  const second = await startOf(page, () => page.keyboard.press('Enter'));
  expect(second.status).toBe(200);
  expect(second.request.audioTrack).toBe(ac3!.index);
  expect(second.request.positionMs).toBeDefined();
  expect(Math.abs(second.request.positionMs! - beforeMs)).toBeLessThanOrEqual(2000);
  expect(second.body.playbackSessionId).not.toBe(first.playbackSessionId);
  expect(second.body.selection.audioTrack).toBe(ac3!.index);

  await expect(player(page)).toHaveAttribute(
    'data-playback-session-id',
    second.body.playbackSessionId,
  );
  await expect(player(page)).toHaveAttribute('data-switch-ms', /\d+/, { timeout: 15_000 });
  // The end of the old session was the viewer's own switch, not a stop by the server (W-1).
  await expect(player(page)).not.toHaveAttribute('data-ended-reason', /.+/);
  expect(await endedReason()).toBeNull();
  await expect(page.getByRole('alert')).toHaveCount(0);
  const switchMs = Number(await player(page).getAttribute('data-switch-ms'));
  expect(switchMs).toBeLessThanOrEqual(SWITCH_BUDGET_MS);

  await page.keyboard.press('i');
  const info = page.getByRole('region', { name: 'Stream-Info' });
  await expect(
    info.getByText('Tonspur', { exact: true }).locator('xpath=following-sibling::*[1]'),
  ).toContainText('AC3');
  console.log(JSON.stringify({ measurement: 'track-switch', browser: browserName, switchMs }));
});

test('an SRT track arrives as WebVTT through <track>', async ({ page, owner }) => {
  test.setTimeout(60_000);
  const itemId = await itemIdByTitle(owner, MULTI_AUDIO_TITLE);
  await resetProgress(page.request, itemId);
  const first = await play(page, itemId);
  const srt = first.tracks.subtitles.find((track) => track.codec === 'subrip' && track.text);
  expect(srt, 'an SRT track of the test film').toBeDefined();
  await expect(page.locator('video track')).toHaveCount(0);

  await page.keyboard.press('t');
  const menu = tracksMenu(page);
  await expect(menu.getByRole('menuitemradio', { name: 'Aus' })).toHaveAttribute(
    'aria-checked',
    'true',
  );
  const target = menu.getByRole('menuitemradio', { name: /SRT/ }).first();
  await arrowTo(page, target);
  const second = await startOf(page, () => page.keyboard.press('Enter'));
  expect(second.status).toBe(200);
  expect(second.request.subtitleTrack).toBeGreaterThan(0);
  expect(second.body.selection.subtitleDelivery).toBe('text');
  expect(second.body.subtitleUrl).toBeTruthy();

  await expect(player(page)).toHaveAttribute(
    'data-playback-session-id',
    second.body.playbackSessionId,
  );
  const track = page.locator('video track');
  await expect(track).toHaveCount(1);
  await expect(track).toHaveAttribute('kind', 'subtitles');
  const src = (await track.getAttribute('src')) ?? '';
  expect(src).toContain('/subtitles/');
  const vtt = await page.request.get(src);
  expect(vtt.status()).toBe(200);
  expect(vtt.headers()['content-type']).toMatch(/^text\/vtt/);
  expect(await vtt.text()).toMatch(/^WEBVTT/);
  // The track shows (the `default` attribute turns it on).
  await expect
    .poll(() =>
      page
        .locator('video')
        .evaluate((video: HTMLVideoElement) => [...video.textTracks].map((t) => t.mode)),
    )
    .toContain('showing');
});

test('a PGS track is burned in: transcode with reason subtitle_burn, and it plays', async ({
  page,
  owner,
}) => {
  test.setTimeout(90_000);
  const itemId = await itemIdByTitle(owner, PGS_TITLE);
  await resetProgress(page.request, itemId);
  const first = await play(page, itemId);
  const pgs = first.tracks.subtitles.find((track) => track.codec === 'hdmv_pgs_subtitle');
  expect(pgs, 'the PGS track of the test film').toBeDefined();
  expect(pgs!.text).toBe(false);

  await page.keyboard.press('t');
  const target = tracksMenu(page).getByRole('menuitemradio', { name: /PGS/ });
  await arrowTo(page, target);
  const endedReason = await recordEndedReason(page);
  const second = await startOf(page, () => page.keyboard.press('Enter'));
  expect(second.status).toBe(200);
  expect(second.body.mode).toBe('hls');
  expect(second.body.transcoding?.mode).toBe('transcode');
  expect(second.body.transcoding?.reason).toBe('subtitle_burn');
  expect(second.body.selection.subtitleDelivery).toBe('burn');
  expect(second.body.subtitleUrl).toBeUndefined();

  await expect(player(page)).toHaveAttribute(
    'data-playback-session-id',
    second.body.playbackSessionId,
  );
  await expect(player(page)).toHaveAttribute('data-switch-ms', /\d+/, { timeout: 30_000 });
  await expect(player(page)).not.toHaveAttribute('data-ended-reason', /.+/);
  expect(await endedReason()).toBeNull();
  await expect(page.locator('video track')).toHaveCount(0);
  await expect
    .poll(() => page.locator('video').evaluate((video: HTMLVideoElement) => video.paused))
    .toBe(false);
});

test('track menu by keyboard: Tab to it, arrows, Escape closes with focus back; no trap', async ({
  page,
  owner,
}) => {
  const itemId = await itemIdByTitle(owner, MULTI_AUDIO_TITLE);
  await play(page, itemId);
  await page.keyboard.press('Space');
  const button = page.getByRole('button', { name: 'Spuren' });
  await tabTo(page, button, 10);
  await page.keyboard.press('Enter');
  const menu = tracksMenu(page);
  await expect(menu).toBeVisible();
  // The menu opens on the current audio track.
  await expect(menu.getByRole('menuitemradio', { checked: true }).first()).toBeFocused();
  await page.keyboard.press('ArrowDown');
  await expectVisibleFocus(page);
  await page.keyboard.press('Escape');
  await expect(menu).toHaveCount(0);
  await expectFocusOn(page, button);
  // Escape in the menu did not leave the player.
  await expect(page).toHaveURL(/\/play\//);
  await expectNoFocusTrap(page);
});
