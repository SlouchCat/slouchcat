import type { APIRequestContext, Page } from '@playwright/test';
import { CSRF, H264_MKV_LONG_TITLE } from './catalog';
import { STORAGE_STATE } from './env';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, expectVisibleFocus, tabTo } from './keyboard';
import {
  createDownload,
  downloadRow,
  offlinePlayer,
  prepareOffline,
  storedDownloads,
  videoTime,
  waitForComplete,
} from './offline';
import { arrowTo, itemIdByTitle, MULTI_AUDIO_TITLE, resetProgress } from './playback';

// O-4 and O-5: a film downloaded with the English audio track plays in flight mode from the
// device, with its subtitles; the progress of that offline playback reaches the server after the
// reconnect and shows in a second browser of the same account (±10 s). The second test starts
// from a known server position, which the offline player resumes from (W-5), and times the
// reconnect until the second browser has the new one.

/** Queued offline events in the page's IndexedDB. */
const queuedEvents = (page: Page) =>
  page.evaluate(
    () =>
      new Promise<{ event: { positionMs: number; state: string } }[]>((resolve, reject) => {
        const open = indexedDB.open('slouchcat-offline');
        open.onerror = () => reject(open.error ?? new Error('open failed'));
        open.onsuccess = () => {
          const read = open.result.transaction('events').objectStore('events').getAll();
          read.onsuccess = () => {
            open.result.close();
            resolve(read.result as { event: { positionMs: number; state: string } }[]);
          };
          read.onerror = () => reject(read.error ?? new Error('read failed'));
        };
      }),
  );

const textTracks = (page: Page) =>
  page
    .locator('video')
    .evaluate((video: HTMLVideoElement) =>
      [...video.textTracks].map((track) => ({ language: track.language, mode: track.mode })),
    );

test('flight mode: the download plays with subtitles and the chosen audio; progress syncs later', async ({
  page,
  context,
  owner,
  browser,
  baseURL,
  extraHTTPHeaders,
}) => {
  test.setTimeout(240_000);
  const itemId = await itemIdByTitle(owner, MULTI_AUDIO_TITLE);
  await resetProgress(owner, itemId, 20_000);

  // Download with the English track (AC3 5.1 in the source, AAC in the download).
  await page.goto(`/items/${itemId}`);
  await page.getByRole('button', { name: 'Herunterladen', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: `${MULTI_AUDIO_TITLE} herunterladen` });
  const audio = dialog.getByRole('radiogroup', { name: 'Tonspur' });
  await expect(audio.getByRole('radio')).toHaveCount(2);
  await audio
    .locator('label')
    .filter({ hasText: /Englisch|English/ })
    .click();
  await expect(dialog.getByText(/^Untertitel: /)).toBeVisible();
  await dialog.getByRole('button', { name: 'Herunterladen' }).click();
  const [download] = await waitForComplete(page);
  expect(download.manifest!.audio?.language).toMatch(/^en/);
  expect(download.subtitlesStored).toHaveLength(2);

  // Flight mode. The app is already open: the downloads page and the player need no network.
  await page.goto('/downloads');
  await prepareOffline(page);
  const row = downloadRow(page, new RegExp(MULTI_AUDIO_TITLE));
  await expect(row.getByText('Auf diesem Gerät')).toBeVisible();
  await context.setOffline(true);
  await expect(page.getByText(/Keine Verbindung zum Server/)).toBeVisible();
  const play = row.getByRole('link', { name: `${MULTI_AUDIO_TITLE} abspielen` });
  await play.focus();
  await page.keyboard.press('Enter');

  const player = offlinePlayer(page);
  await expect(player).toHaveAttribute('data-engine', 'mse');
  await expect(player).toHaveAttribute('data-start-ms', /\d+/, { timeout: 15_000 });
  await expect.poll(() => videoTime(page)).toBeGreaterThan(1);
  await expectFocusOn(page, player.getByRole('button', { name: 'Pause' }));

  // The controls of the video player (F-15): the row is one Tab stop, ←/→ go from button to
  // button and leave the position alone.
  const bar = player.getByRole('toolbar', { name: 'Wiedergabe' });
  const pause = bar.getByRole('button', { name: 'Pause' });
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, bar.getByRole('button', { name: '30 Sekunden vor' }));
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, pause);
  // No quality, version or stream info offline: the gear names no rung.
  await expect(bar.getByRole('button', { name: 'Einstellungen', exact: true })).toBeVisible();
  await expect(bar.getByRole('button', { name: /^Ton: Englisch/ })).toBeVisible();

  // Subtitles from the stored WebVTT files: U opens the local panel, the arrows pick German, Enter
  // takes it and the focus goes back to play/pause. The track stays hidden: the player's subtitle
  // layer draws its cues (11.10).
  await page.keyboard.press('u');
  const panel = player.getByRole('dialog', { name: 'Untertitel' });
  await expect(panel).toBeVisible();
  await arrowTo(page, panel.getByRole('option', { name: /Deutsch/ }));
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  await expect(panel).toHaveCount(0);
  await expectFocusOn(page, pause);
  await expect(bar.getByRole('button', { name: /^Untertitel: Deutsch/ })).toBeVisible();
  await expect
    .poll(() => textTracks(page))
    .toEqual([
      { language: expect.stringMatching(/^(de|ger)/) as unknown as string, mode: 'hidden' },
    ]);
  // The layer shows the cues of the stored WebVTT («Deutsch 1» … every 4 s, review step 6 M-2).
  // 5 jumps to 10 s on the source timeline, in the third cue (9 s to 11 s); paused there, «Aus»
  // empties the layer, and Space plays on.
  const layer = player.locator('[data-subtitles]');
  await expect(layer).toContainText(/^Deutsch \d$/, { timeout: 10_000 });
  await page.keyboard.press('5');
  await page.keyboard.press(' ');
  const playButton = bar.getByRole('button', { name: 'Abspielen' });
  await expectFocusOn(page, playButton);
  await expect(layer).toHaveText('Deutsch 3');
  await page.keyboard.press('u');
  await arrowTo(page, panel.getByRole('option', { name: 'Aus' }), 'ArrowUp');
  await page.keyboard.press('Enter');
  await expect(panel).toHaveCount(0);
  await expect(bar.getByRole('button', { name: /^Untertitel: Aus/ })).toBeVisible();
  await expect(layer).toHaveCount(0);
  await expectFocusOn(page, playButton);
  await page.keyboard.press(' ');
  await expectFocusOn(page, pause);

  // ↑ leads to the timeline, where ←/→ seek from the device (the fragment index, AN-20); ↓ leads
  // back to the row.
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, player.getByRole('slider', { name: 'Wiedergabeposition' }));
  const before = await videoTime(page);
  await page.keyboard.press('ArrowRight');
  await expect(player).toHaveAttribute('data-seek-count', /\d+/);
  expect(await videoTime(page)).toBeGreaterThan(before + 5);
  await page.keyboard.press('ArrowLeft');
  await expectVisibleFocus(page);
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, pause);

  // ? shows only the keys of this player; Escape closes the help, not the player.
  await page.keyboard.press('?');
  const help = player.getByRole('dialog', { name: 'Tastenkürzel' });
  await expect(help).toBeVisible();
  await expect(help.getByText('Untertitel', { exact: true })).toBeVisible();
  await expect(help.getByText('Qualität', { exact: true })).toHaveCount(0);
  await page.keyboard.press('Escape');
  await expect(help).toHaveCount(0);
  await expectFocusOn(page, pause);

  // Above 100 % and night mode as online (review step 6, m-5): ↑ with the focus outside a zone
  // raises the volume past 100 %; night mode through the gear by the arrows. The download plays
  // on through Web Audio (MSE and createMediaElementSource). From 1 s: the film is 20 s long.
  await page.locator('video').evaluate((video: HTMLVideoElement) => {
    video.currentTime = 1;
  });
  const slider = player.getByRole('slider', { name: 'Lautstärke' });
  await expect(slider).toHaveAttribute('aria-valuemax', '150');
  await page.locator('body').focus();
  for (let presses = 0; presses < 30; presses += 1) {
    if (Number(await slider.getAttribute('aria-valuenow')) > 100) break;
    await page.keyboard.press('ArrowUp');
  }
  await expect(slider).toHaveAttribute('aria-valuenow', '105');
  await expect(player.getByRole('status')).toHaveText(
    'Lautstärke 105 %, verstärkt, kann verzerren',
  );
  await tabTo(page, pause, 10);
  const gear = bar.getByRole('button', { name: 'Einstellungen', exact: true });
  await arrowTo(page, gear, 'ArrowRight');
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  const settings = player.getByRole('dialog', { name: 'Einstellungen' });
  await arrowTo(page, settings.getByRole('option', { name: 'Ton-Optionen' }));
  await page.keyboard.press('Enter');
  const sound = player.getByRole('dialog', { name: 'Ton-Optionen' });
  const night = sound.getByRole('switch', { name: 'Nachtmodus' });
  await expect(night).toBeFocused();
  await page.keyboard.press('Space');
  await expect(night).toBeChecked();
  const boostedAt = await videoTime(page);
  await expect.poll(() => videoTime(page), { timeout: 5000 }).toBeGreaterThan(boostedAt + 0.5);
  expect(await page.locator('video').evaluate((video: HTMLVideoElement) => video.paused)).toBe(
    false,
  );
  await expect(page.getByRole('alert')).toHaveCount(0);
  // Escape closes the gear, the focus goes back to its button; ← leads back to play/pause.
  await page.keyboard.press('Escape');
  await expect(sound).toHaveCount(0);
  await expectFocusOn(page, gear);
  await arrowTo(page, pause, 'ArrowLeft');

  // Pause at a known position, then leave the player with Escape.
  await page.locator('video').evaluate((video: HTMLVideoElement) => {
    video.currentTime = 9;
  });
  await page.keyboard.press(' ');
  await expect
    .poll(() => page.locator('video').evaluate((v: HTMLVideoElement) => v.paused))
    .toBe(true);
  const position = Math.round((await videoTime(page)) * 1000);
  await expectNoFocusTrap(page);
  await page.keyboard.press('Escape');
  await expect(page).toHaveURL(/\/downloads$/);
  await expect.poll(async () => (await queuedEvents(page)).length).toBeGreaterThan(0);

  // Back online: the queue goes up; a second browser of the account resumes there (±10 s).
  await context.setOffline(false);
  await expect.poll(async () => (await queuedEvents(page)).length, { timeout: 30_000 }).toBe(0);
  const second = await browser.newContext({
    baseURL,
    locale: 'de-CH',
    extraHTTPHeaders,
    storageState: STORAGE_STATE,
  });
  try {
    const item = (await (await second.request.get(`/api/v1/items/${itemId}`)).json()) as {
      progress?: { positionMs: number };
    };
    expect(item.progress, 'progress after the reconnect').toBeDefined();
    expect(Math.abs(item.progress!.positionMs - position)).toBeLessThanOrEqual(10_000);
    const view = await second.newPage();
    await view.goto(`/items/${itemId}`);
    await expect(view.getByRole('link', { name: /^Weiterschauen ab 00:00:\d{2}$/ })).toBeVisible();
  } finally {
    await second.close();
  }
  expect((await storedDownloads(page)).map((entry) => entry.id)).toEqual([download.id]);
});

test('without MediaSource the blob fallback shows the stored subtitles on the source timeline', async ({
  page,
  owner,
}) => {
  test.setTimeout(120_000);
  const itemId = await itemIdByTitle(owner, MULTI_AUDIO_TITLE);
  await resetProgress(owner, itemId);
  await page.goto('/downloads');
  const [created] = await createDownload(page, itemId);
  const [download] = await waitForComplete(page, 1, (entry) => entry.id === created.id);
  expect(download.subtitlesStored.length).toBeGreaterThan(0);

  // No MediaSource: hls.js is not supported and the file plays from a blob URL, its timeline
  // ahead of the source's (OfflinePlayer `timelineOffset`). The cues are shifted onto it.
  await page.addInitScript(() => {
    for (const name of ['MediaSource', 'ManagedMediaSource', 'WebKitMediaSource']) {
      Object.defineProperty(window, name, { value: undefined, configurable: true });
    }
  });
  await page.goto('/downloads');
  const row = downloadRow(page, new RegExp(MULTI_AUDIO_TITLE));
  await row.getByRole('link', { name: `${MULTI_AUDIO_TITLE} abspielen` }).focus();
  await page.keyboard.press('Enter');
  const player = offlinePlayer(page);
  await expect(player).toHaveAttribute('data-engine', 'blob');
  await expect(player).toHaveAttribute('data-start-ms', /\d+/, { timeout: 15_000 });
  const pause = player.getByRole('button', { name: 'Pause' });
  await expectFocusOn(page, pause);

  await page.keyboard.press('u');
  const panel = player.getByRole('dialog', { name: 'Untertitel' });
  await arrowTo(page, panel.getByRole('option', { name: /Deutsch/ }));
  await page.keyboard.press('Enter');
  await expect(panel).toHaveCount(0);
  // 5 jumps to 10 s of the source, in the third cue (9 s to 11 s), and Space pauses there.
  await page.keyboard.press('5');
  await page.keyboard.press(' ');
  await expectFocusOn(page, player.getByRole('button', { name: 'Abspielen' }));
  await expect(player.locator('[data-subtitles]')).toHaveText('Deutsch 3');
});

/** Sets the profile's server position of a title (an online progress event). */
async function setProgress(
  api: APIRequestContext,
  itemId: string,
  positionMs: number,
  durationMs: number,
): Promise<void> {
  const response = await api.post(`/api/v1/items/${itemId}/progress`, {
    headers: CSRF,
    data: {
      positionMs,
      durationMs,
      playedMs: 0,
      seq: 0,
      state: 'paused',
      clientTime: new Date().toISOString(),
    },
  });
  expect(response.ok(), await response.text()).toBe(true);
}

test('O-5: from 30 s on the server, 15 s offline; a second browser has the new position within 10 s', async ({
  page,
  context,
  owner,
  browser,
  baseURL,
  extraHTTPHeaders,
}, testInfo) => {
  test.setTimeout(240_000);
  // 90 s of 720p H.264 ("Hoch" copies it): room for a start at 30 s and 15 s of play after it.
  const itemId = await itemIdByTitle(owner, H264_MKV_LONG_TITLE);
  const startMs = 30_000;
  await setProgress(owner, itemId, startMs, 90_000);

  await page.goto('/downloads');
  const [created] = await createDownload(page, itemId, { quality: 'high' });
  await waitForComplete(page, 1, (entry) => entry.id === created.id);
  await page.goto('/downloads');
  await prepareOffline(page);
  const row = downloadRow(page, new RegExp(H264_MKV_LONG_TITLE));
  await expect(row.getByText('Auf diesem Gerät')).toBeVisible();

  await context.setOffline(true);
  await expect(page.getByText(/Keine Verbindung zum Server/)).toBeVisible();
  await row.getByRole('link', { name: `${H264_MKV_LONG_TITLE} abspielen` }).focus();
  await page.keyboard.press('Enter');
  const player = offlinePlayer(page);
  await expect(player).toHaveAttribute('data-start-ms', /\d+/, { timeout: 15_000 });
  // The device has no position of its own yet: the player starts at the server's from the sync.
  const startedAt = await videoTime(page);
  expect(Math.abs(startedAt - startMs / 1000), `offline start ${startedAt} s`).toBeLessThanOrEqual(
    2,
  );
  // At least 10 s of play from there; 15 s, so the ±10 s window cannot reach the start value.
  const playedFrom = Date.now();
  await expect
    .poll(() => videoTime(page), { timeout: 40_000, intervals: [500] })
    .toBeGreaterThanOrEqual(startMs / 1000 + 15);
  const playedForMs = Date.now() - playedFrom;
  expect(playedForMs).toBeGreaterThanOrEqual(10_000);
  await expectFocusOn(page, player.getByRole('button', { name: 'Pause' }));
  await page.keyboard.press(' ');
  await expect
    .poll(() => page.locator('video').evaluate((v: HTMLVideoElement) => v.paused))
    .toBe(true);
  const position = Math.round((await videoTime(page)) * 1000);
  expect(position - 10_000).toBeGreaterThan(startMs);
  await page.keyboard.press('Escape');
  await expect(page).toHaveURL(/\/downloads$/);
  await expect.poll(async () => (await queuedEvents(page)).at(-1)?.event.state).toBe('paused');
  expect((await queuedEvents(page)).at(-1)?.event.positionMs).toBe(position);

  // The second browser exists before the reconnect; the clock runs from setOffline(false).
  const second = await browser.newContext({
    baseURL,
    locale: 'de-CH',
    extraHTTPHeaders,
    storageState: STORAGE_STATE,
  });
  try {
    const serverPosition = async () => {
      const item = (await (await second.request.get(`/api/v1/items/${itemId}`)).json()) as {
        progress?: { positionMs: number };
      };
      return item.progress?.positionMs ?? -1;
    };
    expect(await serverPosition()).toBe(startMs);
    const reconnectAt = Date.now();
    await context.setOffline(false);
    await expect
      .poll(async () => Math.abs((await serverPosition()) - position), {
        timeout: 10_000,
        intervals: [250],
      })
      .toBeLessThanOrEqual(10_000);
    const seenAfterMs = Date.now() - reconnectAt;
    const seen = await serverPosition();
    const measured = { startMs, startedAt, playedForMs, position, seen, seenAfterMs };
    testInfo.annotations.push({ type: 'o5-reconnect', description: JSON.stringify(measured) });
    console.log(`MEASURE ${testInfo.project.name} o5-reconnect ${JSON.stringify(measured)}`);
    expect(Math.abs(seen - startMs)).toBeGreaterThan(10_000);
    const view = await second.newPage();
    await view.goto(`/items/${itemId}`);
    await expect(view.getByRole('link', { name: /^Weiterschauen ab 00:00:\d{2}$/ })).toBeVisible();
  } finally {
    await second.close();
  }
  await expect.poll(async () => (await queuedEvents(page)).length).toBe(0);

  // The file is gone from the device: Enter on the row opens the notice with the focus on
  // «Zurück» at once (focusOnMount, 11.13), and Escape goes back to /downloads.
  await page.evaluate(async (id) => {
    const root = await navigator.storage.getDirectory();
    const dir = await (await root.getDirectoryHandle('downloads')).getDirectoryHandle(id);
    await dir.removeEntry('media.mp4');
  }, created.id);
  await row.getByRole('link', { name: `${H264_MKV_LONG_TITLE} abspielen` }).focus();
  await page.keyboard.press('Enter');
  await expect(page.getByText('Download nicht verfügbar')).toBeVisible();
  await expectFocusOn(page, page.getByRole('button', { name: 'Zurück' }));
  await page.keyboard.press('Escape');
  await expect(page).toHaveURL(/\/downloads$/);
});
