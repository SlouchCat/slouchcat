import {
  expect,
  type APIRequestContext,
  type Browser,
  type BrowserContext,
  type Locator,
  type Page,
} from '@playwright/test';
import { loginOwner } from './accounts';
import { CSRF } from './catalog';

// Helpers of the M5 offline specs (downloads, offline playback, offline library). The device's
// offline data lives in IndexedDB `slouchcat-offline` and in OPFS; the specs read the database
// in the page (never creating it) and wake the app's sync with a focus event.

/** Series of six 8-s episodes (testdata/generate-media.sh, M5): season download, auto-download. */
export const DOWNLOAD_SHOW_TITLE = 'Slouchcat Test Download Show';

/** Preparing and fetching a short title can take a while on the CI runner (software encode). */
export const PREPARE_TIMEOUT_MS = 120_000;

/** What the specs read of a stored download (features/offline/types.ts StoredDownload). */
export interface StoredDownloadData {
  id: string;
  accountId: number;
  state: string;
  quality: string;
  auto: boolean;
  allowedProfileIds: number[];
  estimatedBytes: number;
  bytesWritten: number;
  complete: boolean;
  subtitlesStored: number[];
  item?: { id: string; title: string; parentId?: string; episodeNumber?: number };
  manifest?: {
    sizeBytes: number;
    durationMs: number;
    audio?: { index: number; language?: string };
    subtitles: { track: { id: number; language?: string } }[];
  };
  positions?: Record<string, { positionMs: number; at: number }>;
}

/** The downloads in the page's IndexedDB; empty while the app has not created it yet. */
export function storedDownloads(page: Page): Promise<StoredDownloadData[]> {
  return page.evaluate(async () => {
    const name = 'slouchcat-offline';
    // Opening a missing database would create it without stores and break the app's upgrade.
    const known = await indexedDB.databases();
    if (!known.some((entry) => entry.name === name)) return [];
    return new Promise<StoredDownloadData[]>((resolve, reject) => {
      const open = indexedDB.open(name);
      open.onerror = () => reject(open.error ?? new Error('open failed'));
      open.onsuccess = () => {
        const db = open.result;
        if (!db.objectStoreNames.contains('downloads')) {
          db.close();
          resolve([]);
          return;
        }
        const read = db.transaction('downloads').objectStore('downloads').getAll();
        read.onerror = () => reject(read.error ?? new Error('read failed'));
        read.onsuccess = () => {
          db.close();
          resolve(read.result as StoredDownloadData[]);
        };
      };
    });
  });
}

/** Wakes the app's offline engine (a focus counts as a reason to sync, design 4). */
export async function syncNow(page: Page): Promise<void> {
  await page.evaluate(() => window.dispatchEvent(new Event('focus')));
}

/** `meta.lastSyncAt` of the page's offline database (0 before the first sync). */
export function lastSyncAt(page: Page): Promise<number> {
  return page.evaluate(async () => {
    const name = 'slouchcat-offline';
    const known = await indexedDB.databases();
    if (!known.some((entry) => entry.name === name)) return 0;
    return new Promise<number>((resolve, reject) => {
      const open = indexedDB.open(name);
      open.onerror = () => reject(open.error ?? new Error('open failed'));
      open.onsuccess = () => {
        const db = open.result;
        if (!db.objectStoreNames.contains('meta')) {
          db.close();
          resolve(0);
          return;
        }
        const read = db.transaction('meta').objectStore('meta').get('lastSyncAt');
        read.onerror = () => reject(read.error ?? new Error('read failed'));
        read.onsuccess = () => {
          db.close();
          resolve(typeof read.result === 'number' ? read.result : 0);
        };
      };
    });
  });
}

/** Wakes the engine and waits until its sync went through (K2: assert after, not before). */
export async function syncAndWait(page: Page): Promise<void> {
  const before = await lastSyncAt(page);
  await expect
    .poll(
      async () => {
        await syncNow(page);
        return lastSyncAt(page);
      },
      { timeout: 30_000, intervals: [500] },
    )
    .toBeGreaterThan(before);
}

/**
 * Waits until the device holds `count` complete downloads (matching `filter`), syncing every
 * 2 s so the device learns of ready copies before the 60-s interval.
 */
export async function waitForComplete(
  page: Page,
  count = 1,
  filter: (download: StoredDownloadData) => boolean = () => true,
): Promise<StoredDownloadData[]> {
  let complete: StoredDownloadData[] = [];
  await expect
    .poll(
      async () => {
        await syncNow(page);
        complete = (await storedDownloads(page)).filter(
          (download) => download.complete && filter(download),
        );
        return complete.length;
      },
      { timeout: PREPARE_TIMEOUT_MS, intervals: [2000] },
    )
    .toBeGreaterThanOrEqual(count);
  return complete;
}

/** Creates a download for the page's device through the API (the page's own session). */
export async function createDownload(
  page: Page,
  itemId: string,
  body: Record<string, unknown> = { quality: 'low' },
): Promise<{ id: string; estimatedBytes: number }[]> {
  const response = await page.request.post(`/api/v1/items/${itemId}/downloads`, {
    headers: CSRF,
    data: body,
  });
  expect(response.status(), await response.text()).toBe(201);
  return ((await response.json()) as { downloads: { id: string; estimatedBytes: number }[] })
    .downloads;
}

/**
 * Deletes every download of the request context's device on the server (the sync lists them),
 * so a spec leaves no downloads behind for the next one. Tolerates a server without M5.
 */
export async function removeDeviceDownloads(api: APIRequestContext): Promise<void> {
  const response = await api.post('/api/v1/offline/sync', { headers: CSRF, data: { held: [] } });
  if (!response.ok()) return;
  const { downloads } = (await response.json()) as { downloads: { id: string }[] };
  for (const download of downloads) {
    await api.delete(`/api/v1/downloads/${download.id}`, { headers: CSRF });
  }
}

/** The downloads table of /downloads and the row of a title in it. */
export const downloadsTable = (page: Page): Locator =>
  page.getByRole('table', { name: 'Auf diesem Gerät' });

export const downloadRow = (page: Page, title: string | RegExp): Locator =>
  downloadsTable(page)
    .getByRole('row')
    .filter({ has: page.getByRole('rowheader', { name: title }) });

/** The offline player (same landmark name as the main player). */
export const offlinePlayer = (page: Page): Locator =>
  page.getByRole('main', { name: 'Videoplayer' });

/**
 * A browser of its own with a fresh login of the main account (a device of its own, not the
 * shared storage-state session), optionally with a profile chosen through the API.
 */
export async function freshDevice(
  browser: Browser,
  options: {
    baseURL: string | undefined;
    extraHTTPHeaders?: Record<string, string>;
    userAgent?: string;
    profileId?: number;
  },
): Promise<{ context: BrowserContext; page: Page }> {
  const context = await browser.newContext({
    baseURL: options.baseURL,
    locale: 'de-CH',
    extraHTTPHeaders: options.extraHTTPHeaders,
    storageState: { cookies: [], origins: [] },
    ...(options.userAgent ? { userAgent: options.userAgent } : {}),
  });
  await loginOwner(context.request);
  if (options.profileId !== undefined) {
    const selected = await context.request.post(
      `/api/v1/auth/profiles/${options.profileId}/select`,
      { headers: CSRF },
    );
    expect(selected.status(), await selected.text()).toBe(200);
  }
  return { context, page: await context.newPage() };
}

/**
 * Opens the offline library inside the running app (history push, no document load), as a
 * link of the app does.
 */
export async function openOfflineLibrary(page: Page): Promise<void> {
  await page.evaluate(() => {
    window.history.pushState({}, '', '/offline');
    window.dispatchEvent(new PopStateEvent('popstate'));
  });
  await expect(page).toHaveURL(/\/offline$/);
}

/**
 * Makes sure the offline pages load without network before a spec goes offline: the service
 * worker controls the page (clientsClaim takes over the page it was registered from), so every
 * app chunk comes from its precache, as for a viewer who opened the app before. Fails when no
 * worker takes over (W11: no silent fallback).
 */
export async function prepareOffline(page: Page): Promise<void> {
  await page.waitForFunction(() => navigator.serviceWorker?.controller != null, undefined, {
    timeout: 30_000,
  });
}

/** Time of the video element (seconds). */
export const videoTime = (page: Page) =>
  page.locator('video').evaluate((video: HTMLVideoElement) => video.currentTime);
