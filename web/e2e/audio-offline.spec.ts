import type { Page } from '@playwright/test';
import {
  audioDetail,
  audioItemId,
  audioPlayer,
  audioState,
  M4B_TITLE,
  setAudioProgress,
} from './audio';
import { STORAGE_STATE } from './env';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, expectVisibleFocus } from './keyboard';
import {
  createDownload,
  downloadRow,
  prepareOffline,
  storedDownloads,
  waitForComplete,
} from './offline';
import { arrowTo } from './playback';

// O-4/O-5 for audio (M6): a downloaded audiobook plays in flight mode behind the audio player,
// with the chapters of its manifest; a chapter jump works without network; the progress of that
// offline playback reaches the server after the reconnect and shows on a second device (±10 s).
// Runs in Chromium and Firefox (follows offline-playback.spec.ts).

/** Queued offline events in the page's IndexedDB. */
const queuedEvents = (page: Page) =>
  page.evaluate(
    () =>
      new Promise<{ event: { positionMs: number; state: string; playbackRate?: number } }[]>(
        (resolve, reject) => {
          const open = indexedDB.open('slouchcat-offline');
          open.onerror = () => reject(open.error ?? new Error('open failed'));
          open.onsuccess = () => {
            const read = open.result.transaction('events').objectStore('events').getAll();
            read.onsuccess = () => {
              open.result.close();
              resolve(
                read.result as {
                  event: { positionMs: number; state: string; playbackRate?: number };
                }[],
              );
            };
            read.onerror = () => reject(read.error ?? new Error('read failed'));
          };
        },
      ),
  );

const time = async (page: Page) => (await audioState(page)).time;

test('flight mode: the downloaded audiobook plays with its chapters; progress syncs later', async ({
  page,
  context,
  owner,
  browser,
  baseURL,
  extraHTTPHeaders,
}) => {
  test.setTimeout(240_000);
  const book = await audioItemId(owner, M4B_TITLE);
  await audioDetail(owner, book, 12);
  await setAudioProgress(owner, book, 0, 60_000, 1);

  await page.goto('/downloads');
  const [created] = await createDownload(page, book, { quality: 'low' });
  const [download] = await waitForComplete(page, 1, (entry) => entry.id === created.id);
  expect(download.manifest!.durationMs).toBeGreaterThan(55_000);
  expect(download.manifest!.mimeType).toBe('audio/mp4');
  expect(download.manifest!.chapters).toHaveLength(12);

  // Flight mode. The app is open already: the downloads page and the player need no network.
  await page.goto('/downloads');
  await prepareOffline(page);
  const row = downloadRow(page, new RegExp(M4B_TITLE));
  await expect(row.getByText('Auf diesem Gerät')).toBeVisible();
  await context.setOffline(true);
  await expect(page.getByText(/Keine Verbindung zum Server/)).toBeVisible();
  await row.getByRole('link', { name: `${M4B_TITLE} abspielen` }).focus();
  await page.keyboard.press('Enter');

  const player = audioPlayer(page);
  await expect(player).toHaveAttribute('data-start-ms', /\d+/, { timeout: 15_000 });
  await expect(player.getByRole('heading', { name: M4B_TITLE })).toBeVisible();
  await expect.poll(() => time(page)).toBeGreaterThan(0.5);
  await expectFocusOn(page, player.getByRole('button', { name: 'Pause' }));

  // Chapter jump from the device: Shift+→ twice lands on chapter 3 (10 s, ±1 s).
  await page.keyboard.press('Shift+ArrowRight');
  await page.keyboard.press('Shift+ArrowRight');
  await expect.poll(() => time(page)).toBeGreaterThanOrEqual(9.5);
  expect(await time(page)).toBeLessThanOrEqual(12);
  await expect(player.getByText(/^Kapitel 3 · /)).toBeVisible();
  // The chapter menu works offline as well: chapter 7 (30 s).
  await page.keyboard.press('c');
  const menu = page.getByRole('menu', { name: 'Kapitel' });
  await expect(menu).toBeVisible();
  await expectVisibleFocus(page);
  await arrowTo(page, menu.getByRole('menuitemradio', { name: /^Kapitel 7 · / }));
  await page.keyboard.press('Enter');
  await expect.poll(() => time(page)).toBeGreaterThanOrEqual(29.5);
  expect(await time(page)).toBeLessThanOrEqual(32);

  // Pause at a known position, then leave the player with Escape.
  await player.getByRole('button', { name: 'Pause' }).focus();
  await page.keyboard.press('Space');
  await expect.poll(async () => (await audioState(page)).paused).toBe(true);
  const position = Math.round((await time(page)) * 1000);
  await expectNoFocusTrap(page);
  await page.keyboard.press('Escape');
  await expect(page).toHaveURL(/\/downloads$/);
  await expect.poll(async () => (await queuedEvents(page)).at(-1)?.event.state).toBe('paused');
  expect((await queuedEvents(page)).at(-1)?.event).toMatchObject({ playbackRate: 1 });

  // Back online: the queue goes up; a second device of the account has the position (±10 s).
  await context.setOffline(false);
  await expect.poll(async () => (await queuedEvents(page)).length, { timeout: 30_000 }).toBe(0);
  const second = await browser.newContext({
    baseURL,
    locale: 'de-CH',
    extraHTTPHeaders,
    storageState: STORAGE_STATE,
  });
  try {
    const item = (await (await second.request.get(`/api/v1/items/${book}`)).json()) as {
      progress?: { positionMs: number };
    };
    expect(item.progress, 'progress after the reconnect').toBeDefined();
    expect(Math.abs(item.progress!.positionMs - position)).toBeLessThanOrEqual(10_000);
    const view = await second.newPage();
    await view.goto(`/items/${book}`);
    await expect(view.getByRole('link', { name: /^Weiterhören ab 00:00:\d{2}$/ })).toBeVisible();
  } finally {
    await second.close();
  }
  expect((await storedDownloads(page)).map((entry) => entry.id)).toEqual([download.id]);
  await setAudioProgress(owner, book, 0, 60_000, 1);
});
