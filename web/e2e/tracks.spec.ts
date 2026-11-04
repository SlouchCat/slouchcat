import { CSRF } from './catalog';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, expectVisibleFocus } from './keyboard';
import { DOWNLOAD_SHOW_TITLE } from './offline';
import {
  arrowTo,
  episodeIds,
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

/** The rows of the sound panel (T) or the subtitle panel (U): a listbox named like the panel. */
const panelList = (page: import('@playwright/test').Page, name: 'Ton' | 'Untertitel') =>
  page.getByRole('dialog', { name }).getByRole('listbox', { name });

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

  // T opens the sound panel on the current audio track; the bar names its language.
  await expect(page.getByRole('button', { name: 'Ton: Deutsch' })).toHaveText('Deutsch');
  await page.keyboard.press('t');
  const menu = panelList(page, 'Ton');
  await expect(menu).toBeVisible();
  await expect(menu.getByRole('option', { selected: true })).toBeFocused();
  await expectVisibleFocus(page);
  await expect(page.getByRole('heading', { name: 'Ton', exact: true })).toBeVisible();
  const target = menu.getByRole('option', { name: /AC3/ });
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

  await page.keyboard.press('u');
  const menu = panelList(page, 'Untertitel');
  await expect(menu.getByRole('option', { name: 'Aus' })).toHaveAttribute('aria-selected', 'true');
  const target = menu.getByRole('option', { name: /SRT/ }).first();
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
  // The track stays hidden: the player draws its cues in a layer of its own (11.10), and the
  // captions of the file show there ("Deutsch 1" … every 4 s).
  await expect
    .poll(() =>
      page
        .locator('video')
        .evaluate((video: HTMLVideoElement) => [...video.textTracks].map((t) => t.mode)),
    )
    .toEqual(['hidden']);
  await expect(player(page).locator('[data-subtitles]')).toContainText(/^Deutsch \d$/, {
    timeout: 10_000,
  });
});

test('the subtitle style by keyboard: size and colour on the layer, the offset shows a cue later', async ({
  page,
  owner,
}) => {
  test.setTimeout(60_000);
  const itemId = await itemIdByTitle(owner, MULTI_AUDIO_TITLE);
  await resetProgress(page.request, itemId);
  await play(page, itemId);
  await page.keyboard.press('u');
  const target = panelList(page, 'Untertitel').getByRole('option', { name: /SRT/ }).first();
  await arrowTo(page, target);
  const second = await startOf(page, () => page.keyboard.press('Enter'));
  await expect(player(page)).toHaveAttribute(
    'data-playback-session-id',
    second.body.playbackSessionId,
  );
  const layer = player(page).locator('[data-subtitles]');
  await expect(layer).toContainText(/^Deutsch \d+$/, { timeout: 10_000 });

  // Paused half a second into a cue: the layer shows that cue.
  const src = (await page.locator('video track').getAttribute('src')) ?? '';
  const vtt = await (await page.request.get(src)).text();
  const starts = [...vtt.matchAll(/(\d+):(\d+)\.(\d+) --> /g)].map(
    ([, minutes, seconds, ms]) => Number(minutes) * 60 + Number(seconds) + Number(ms) / 1000,
  );
  const cueStart = starts.find((start) => start >= 4);
  expect(cueStart, 'a cue from 4 s on').toBeDefined();
  await page.locator('video').evaluate((video: HTMLVideoElement, at: number) => {
    video.pause();
    video.currentTime = at;
  }, cueStart! + 0.5);
  await expect.poll(async () => (await videoState(page)).paused).toBe(true);
  const shown = (await layer.textContent()) ?? '';
  expect(shown).toMatch(/^Deutsch \d+$/);

  // U → ↓ to «Untertitel-Stil»: size XL and yellow by the arrows and Enter.
  await page.keyboard.press('u');
  const subtitles = page.getByRole('dialog', { name: 'Untertitel' });
  await arrowTo(page, subtitles.getByRole('button', { name: 'Untertitel-Stil' }));
  await page.keyboard.press('Enter');
  const style = page.getByRole('dialog', { name: 'Untertitel-Stil' });
  await expectFocusOn(page, style.getByRole('option', { name: 'M', exact: true }));
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, style.getByRole('option', { name: 'XL' }));
  await page.keyboard.press('Enter');
  await expect(layer).toHaveAttribute('data-size', 'xl');
  await arrowTo(page, style.getByRole('option', { name: 'Gelb' }));
  await page.keyboard.press('Enter');
  await expect(layer).toHaveAttribute('data-color', 'yellow');

  // The offset: ten times «später» moves every cue a second later, so the cue on screen at this
  // position is no longer the one that started half a second ago.
  await arrowTo(page, style.getByRole('button', { name: '0,1 s früher' }), 'ArrowDown', 16);
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, style.getByRole('button', { name: '0,1 s später' }));
  for (let presses = 0; presses < 10; presses += 1) await page.keyboard.press('Enter');
  await expect(style.getByText('+1,0 s')).toBeVisible();
  await expect
    .poll(async () => (await layer.count()) === 0 || (await layer.textContent()))
    .not.toBe(shown);
  await page.keyboard.press('Escape');
  await expect(style).toHaveCount(0);
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

  await page.keyboard.press('u');
  const target = panelList(page, 'Untertitel').getByRole('option', { name: /PGS/ });
  // The panel says what the choice costs.
  await expect(target).toHaveAccessibleDescription(/Bild-Untertitel, wird eingebrannt/);
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

test('sound panel by keyboard: → along the row, arrows, Escape closes with focus back; no trap', async ({
  page,
  owner,
}) => {
  const itemId = await itemIdByTitle(owner, MULTI_AUDIO_TITLE);
  await play(page, itemId);
  await page.keyboard.press('Space');
  // The button row is one Tab stop (roving tab index, concept 13 11.2): → walks along it.
  const button = page.getByRole('button', { name: /^Ton: / });
  await arrowTo(page, button, 'ArrowRight');
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  const menu = panelList(page, 'Ton');
  await expect(menu).toBeVisible();
  await expect(button).toHaveAttribute('aria-expanded', 'true');
  // The panel opens on the current audio track; the arrows stay in it and choose nothing.
  const time = await videoState(page);
  await expect(menu.getByRole('option', { selected: true })).toBeFocused();
  await page.keyboard.press('ArrowDown');
  await expectVisibleFocus(page);
  await expect(menu.getByRole('option', { selected: true })).not.toBeFocused();
  await page.keyboard.press('ArrowLeft');
  expect(Math.abs((await videoState(page)).time - time.time)).toBeLessThan(5);
  await page.keyboard.press('Escape');
  await expect(menu).toHaveCount(0);
  await expectFocusOn(page, button);
  // Escape in the panel did not leave the player.
  await expect(page).toHaveURL(/\/play\//);
  await expectNoFocusTrap(page);
});

// Contract 1.11.0 (concept 13 11.3, review m-1): a missing or null subtitleTrack lets the server
// choose, so every switch names audio and subtitles; «Aus» is subtitleOff and survives a switch of
// the audio track.
test('every switch names both tracks: «Aus» is subtitleOff and stays off after an audio switch', async ({
  page,
  owner,
}) => {
  test.setTimeout(90_000);
  const itemId = await itemIdByTitle(owner, MULTI_AUDIO_TITLE);
  await resetProgress(page.request, itemId);
  const first = await play(page, itemId);
  const ac3 = first.tracks.audio.find((track) => track.codec === 'ac3');
  expect(ac3, 'the AC3 track of the test film').toBeDefined();
  expect(first.selection.subtitleTrack).toBeUndefined();

  await page.keyboard.press('u');
  await arrowTo(page, panelList(page, 'Untertitel').getByRole('option', { name: /SRT/ }).first());
  const on = await startOf(page, () => page.keyboard.press('Enter'));
  expect(on.status).toBe(200);
  expect(on.request.audioTrack).toBe(first.selection.audioTrack);
  expect(on.request.subtitleTrack).toBeGreaterThan(0);
  expect(on.request).not.toHaveProperty('subtitleOff');
  await expect(player(page)).toHaveAttribute('data-playback-session-id', on.body.playbackSessionId);

  await page.keyboard.press('u');
  await arrowTo(
    page,
    panelList(page, 'Untertitel').getByRole('option', { name: 'Aus' }),
    'ArrowUp',
  );
  const off = await startOf(page, () => page.keyboard.press('Enter'));
  expect(off.status).toBe(200);
  expect(off.request).toMatchObject({ audioTrack: first.selection.audioTrack, subtitleOff: true });
  expect(off.request).not.toHaveProperty('subtitleTrack');
  expect(off.body.selection.subtitleTrack).toBeUndefined();
  expect(off.body.selection.subtitleReason).toBe('request');
  await expect(player(page)).toHaveAttribute(
    'data-playback-session-id',
    off.body.playbackSessionId,
  );

  await page.keyboard.press('t');
  await arrowTo(page, panelList(page, 'Ton').getByRole('option', { name: /AC3/ }));
  const sound = await startOf(page, () => page.keyboard.press('Enter'));
  expect(sound.status).toBe(200);
  expect(sound.request).toMatchObject({ audioTrack: ac3!.index, subtitleOff: true });
  expect(sound.request).not.toHaveProperty('subtitleTrack');
  expect(sound.body.selection.audioTrack).toBe(ac3!.index);
  expect(sound.body.selection.subtitleTrack).toBeUndefined();
  await expect(player(page)).toHaveAttribute(
    'data-playback-session-id',
    sound.body.playbackSessionId,
  );
  await expect(page.locator('video track')).toHaveCount(0);
});

// Concept 13 11.3 «Je Serie merken», rule 8: the switch below the list of an episode is reached
// with Tab, Space toggles it, Shift+Tab goes back; a choice with it on keeps the language for the
// series, and the next start without tracks takes it (subtitleReason series).
test('«Für diese Serie merken» by keyboard: a subtitle chosen with it on comes back on the next start', async ({
  page,
  owner,
}) => {
  test.setTimeout(90_000);
  const resetSeries = async () => {
    const response = await page.request.delete('/api/v1/me/playback-prefs/series', {
      headers: CSRF,
    });
    expect(response.status(), await response.text()).toBe(204);
  };
  const seriesId = await itemIdByTitle(owner, DOWNLOAD_SHOW_TITLE);
  const [episodeId] = await episodeIds(owner, seriesId);
  expect(episodeId, 'S01E01').toBeDefined();
  await resetSeries();
  await resetProgress(page.request, episodeId);
  const first = await play(page, episodeId);
  const sidecar = first.tracks.subtitles.find((track) => track.source === 'sidecar');
  expect(sidecar, 'the German sidecar of S01E01').toBeDefined();
  expect(first.selection.subtitleTrack).toBeUndefined();

  await page.keyboard.press('u');
  const panel = page.getByRole('dialog', { name: 'Untertitel' });
  const menu = panelList(page, 'Untertitel');
  await expect(menu.getByRole('option', { selected: true })).toBeFocused();
  const remember = panel.getByRole('switch', { name: 'Für diese Serie merken' });
  await expect(remember).toBeChecked();
  await page.keyboard.press('Tab');
  await expect(remember).toBeFocused();
  await expectVisibleFocus(page);
  await page.keyboard.press('Space');
  await expect(remember).not.toBeChecked();
  await page.keyboard.press('Space');
  await expect(remember).toBeChecked();
  await page.keyboard.press('Shift+Tab');
  await expect(menu.getByRole('option', { selected: true })).toBeFocused();
  await arrowTo(page, menu.getByRole('option', { name: /extern/ }));
  const chosen = await startOf(page, () => page.keyboard.press('Enter'));
  expect(chosen.status).toBe(200);
  expect(chosen.request).toMatchObject({ subtitleTrack: sidecar!.id, rememberForSeries: true });
  expect(chosen.request.audioTrack).toBe(first.selection.audioTrack);
  expect(chosen.body.selection.subtitleReason).toBe('request');
  await expect(player(page)).toHaveAttribute(
    'data-playback-session-id',
    chosen.body.playbackSessionId,
  );

  // The next start names no track: the server takes the language remembered for the series.
  const again = await play(page, episodeId);
  expect(again.selection.subtitleTrack).toBe(sidecar!.id);
  expect(again.selection.subtitleReason).toBe('series');
  await expectNoFocusTrap(page);
  await resetSeries();
});
