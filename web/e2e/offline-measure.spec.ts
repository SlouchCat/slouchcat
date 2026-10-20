import type { Page } from '@playwright/test';
import { CSRF } from './catalog';
import { expect, test } from './fixtures';
import {
  createDownload,
  offlinePlayer,
  PREPARE_TIMEOUT_MS,
  storedDownloads,
  syncNow,
  waitForComplete,
} from './offline';
import { itemIdByTitle } from './playback';

// Measurements of M5 for docs/perf/M5.md, each with its threshold: seeking from OPFS through
// MSE at 25/50/75 % of a downloaded film (< 2 s each), the fetch rate of a download on localhost
// and the prepared size of the three download profiles against the server's estimate (±10 %,
// O-1). The cold start in flight mode is measured in offline-cold-start.spec.ts. Every number goes
// to the test's annotations and to stdout as `MEASURE <project> <name> <json>`.

/** 90 s of 720p H.264/AAC (testdata/generate-media.sh): long enough for several fragments. */
const LONG_TITLE = 'Slouchcat Test H264 MKV Long';
const SEEK_LIMIT_MS = 2000;
const QUALITIES = ['high', 'medium', 'low'] as const;

function report(name: string, value: Record<string, unknown>): void {
  test.info().annotations.push({ type: name, description: JSON.stringify(value) });
  console.log(`MEASURE ${test.info().project.name} ${name} ${JSON.stringify(value)}`);
}

/**
 * Blocks and duration of the file fetch of a download (Resource Timing of the page): from the
 * start of the first block to the end of the last; between the blocks the device writes to OPFS.
 * Firefox reports no body sizes for requests that pass the service worker, so the bytes are the
 * manifest's.
 */
function fetchTiming(page: Page, downloadId: string) {
  return page.evaluate((id) => {
    const blocks = performance
      .getEntriesByType('resource')
      .filter((entry) =>
        entry.name.includes(`/downloads/${id}/file`),
      ) as PerformanceResourceTiming[];
    const start = Math.min(...blocks.map((entry) => entry.startTime));
    const end = Math.max(...blocks.map((entry) => entry.responseEnd));
    return { blocks: blocks.length, ms: end - start };
  }, downloadId);
}

/** Seeks the offline player to `fraction` of the film; resolves with the player's seek time. */
async function seekTo(page: Page, fraction: number): Promise<number> {
  const player = offlinePlayer(page);
  const before = Number((await player.getAttribute('data-seek-count')) ?? 0);
  await page.locator('video').evaluate((video: HTMLVideoElement, at) => {
    video.currentTime = video.duration * at;
  }, fraction);
  await expect(player).toHaveAttribute('data-seek-count', String(before + 1), { timeout: 10_000 });
  return Number(await player.getAttribute('data-seek-ms'));
}

test('download sizes against the estimate, the fetch rate and seeking from OPFS', async ({
  page,
  owner,
}) => {
  test.setTimeout(PREPARE_TIMEOUT_MS * 4);
  const itemId = await itemIdByTitle(owner, LONG_TITLE);
  const options = (await (await owner.get(`/api/v1/items/${itemId}/download-options`)).json()) as {
    qualities: { quality: string; estimatedBytes: number; height: number }[];
  };
  await page.goto(`/items/${itemId}`);
  await expect(page.getByRole('heading', { level: 1, name: LONG_TITLE })).toBeVisible();

  for (const quality of QUALITIES) {
    const [created] = await createDownload(page, itemId, { quality });
    const [download] = await waitForComplete(page, 1, (entry) => entry.id === created.id);
    const option = options.qualities.find((entry) => entry.quality === quality)!;
    const size = download.manifest!.sizeBytes;
    const deviation = (size - option.estimatedBytes) / option.estimatedBytes;
    const timing = await fetchTiming(page, download.id);
    const rate = timing.ms > 0 ? size / 1e6 / (timing.ms / 1000) : 0;
    report(`size-${quality}`, {
      height: option.height,
      sizeBytes: size,
      estimatedBytes: option.estimatedBytes,
      deviationPercent: Math.round(deviation * 1000) / 10,
    });
    report(`fetch-${quality}`, {
      blocks: timing.blocks,
      bytes: size,
      ms: Math.round(timing.ms),
      mbPerSecond: Math.round(rate * 10) / 10,
    });
    expect(Math.abs(deviation), `${quality}: ${size} B vs ${option.estimatedBytes} B`).toBeLessThan(
      0.1,
    );
    // 8-MiB blocks (range.ts): every block of the file came in once.
    expect(timing.blocks).toBe(Math.ceil(size / (8 * 1024 * 1024)));

    if (quality === 'high') {
      // Seeking from the device: hls.js reads the fragments from OPFS (MSE).
      await page.goto(`/offline/play/${download.id}?as=session`);
      const player = offlinePlayer(page);
      await expect(player).toHaveAttribute('data-engine', 'mse');
      await expect(player).toHaveAttribute('data-start-ms', /\d+/, { timeout: 15_000 });
      const seeks: Record<string, number> = {};
      for (const fraction of [0.25, 0.5, 0.75]) {
        seeks[`${fraction * 100} %`] = await seekTo(page, fraction);
      }
      report('seek-ms', { ...seeks, startMs: Number(await player.getAttribute('data-start-ms')) });
      for (const [at, ms] of Object.entries(seeks)) {
        expect(ms, `seek to ${at} took ${ms} ms`).toBeLessThan(SEEK_LIMIT_MS);
      }
    }

    // One download per title and device: delete it before the next profile.
    const removed = await page.request.delete(`/api/v1/downloads/${download.id}`, {
      headers: CSRF,
    });
    expect(removed.status()).toBe(204);
    // The next sync names it as revoked and the device lets it go (a new page: new timings).
    await page.goto(`/items/${itemId}`);
    await expect
      .poll(
        async () => {
          await syncNow(page);
          return (await storedDownloads(page)).some((entry) => entry.id === download.id);
        },
        { timeout: 30_000, intervals: [1000] },
      )
      .toBe(false);
  }
});
