import type { APIRequestContext, Page } from '@playwright/test';
import { CSRF, H264_MKV_LONG_TITLE } from './catalog';
import { STORAGE_STATE } from './env';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, expectVisibleFocus } from './keyboard';
import {
  createDownload,
  downloadRow,
  offlinePlayer,
  prepareOffline,
  storedDownloads,
  videoTime,
  waitForComplete,
} from './offline';
import { itemIdByTitle, MULTI_AUDIO_TITLE, resetProgress } from './playback';

// O-4 and O-5: a film downloaded with the English audio track plays in flight mode from the
// device, with its subtitles; the progress of that offline playback reaches the server after the
// reconnect and shows in a second browser of the same account (±10 s). The second test starts
// from a known server position and times the reconnect until the second browser has the new one.

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

  // Subtitles from the stored WebVTT files: T opens the local menu, the arrows pick German.
  await page.keyboard.press('t');
  const menu = page.getByRole('menu', { name: 'Untertitel' });
  await expect(menu).toBeVisible();
  await page.keyboard.press('ArrowDown');
  const german = menu.getByRole('menuitemradio', { name: /Deutsch/ });
  for (let presses = 0; presses < 3; presses += 1) {
    if (await german.evaluate((item) => item === document.activeElement)) break;
    await page.keyboard.press('ArrowDown');
  }
  await page.keyboard.press('Enter');
  await expect(menu).toHaveCount(0);
  await expect
    .poll(async () => (await textTracks(page)).filter((track) => track.mode === 'showing'))
    .toEqual([
      { language: expect.stringMatching(/^(de|ger)/) as unknown as string, mode: 'showing' },
    ]);

  // Seeking by keyboard works from the device (the fragment index, AN-20).
  const before = await videoTime(page);
  await page.keyboard.press('ArrowRight');
  await expect(player).toHaveAttribute('data-seek-count', /\d+/);
  expect(await videoTime(page)).toBeGreaterThan(before + 5);
  await page.keyboard.press('ArrowLeft');
  await expectVisibleFocus(page);

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
  // The offline player resumes from the device's own position (none yet): go to the server's.
  await page.locator('video').evaluate((video: HTMLVideoElement, at) => {
    video.currentTime = at;
  }, startMs / 1000);
  await expect.poll(() => videoTime(page)).toBeGreaterThanOrEqual(startMs / 1000);
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
    const measured = { startMs, playedForMs, position, seen, seenAfterMs };
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
});
