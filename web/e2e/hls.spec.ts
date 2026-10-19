import { expect, test, type Page } from '@playwright/test';
import { CSRF, H264_MKV_LONG_TITLE, HEVC_LONG_TITLE, itemPath } from './catalog';
import { expectFocusOn } from './keyboard';

/**
 * DoD M1b (plan 3.5, W-3): HEVC transcode starts in under 5 s; seeking to 20/50/80 % plays again
 * in under 3 s, for transcode and remux, in Chromium and Firefox (hls.js). The numbers go to
 * stdout as JSON for docs/perf/M1b.md.
 */
const START_BUDGET_MS = 5000;
const SEEK_BUDGET_MS = 3000;
const SEEK_POINTS = [0.2, 0.5, 0.8];

interface PlaybackInfo {
  mode: string;
  playbackSessionId: string;
  transcoding?: { mode: string; backend?: string; reason: string };
}

const isPlay = (url: string, method: string) =>
  method === 'POST' && /\/api\/v1\/items\/[^/]+\/play$/.test(url);

/** Puts the profile's position of the title back to 0, so every run starts at the beginning. */
async function resetProgress(page: Page, itemId: string): Promise<void> {
  const response = await page.request.post(`/api/v1/items/${itemId}/progress`, {
    headers: CSRF,
    data: {
      positionMs: 0,
      durationMs: 90_000,
      playedMs: 0,
      seq: 0,
      state: 'paused',
      clientTime: new Date().toISOString(),
    },
  });
  expect(response.ok()).toBe(true);
}

const videoState = (page: Page) =>
  page.locator('video').evaluate((video: HTMLVideoElement) => ({
    time: video.currentTime,
    duration: video.duration,
    paused: video.paused,
  }));

/**
 * Firefox on Linux decodes HEVC through the system ffmpeg and reports it, so the server would
 * remux. The DoD measures the transcode path (plan 10.3 assumes Firefox without HEVC): `h264Only`
 * reports H.264 as the only video codec for that case.
 */
async function reportH264Only(page: Page): Promise<void> {
  await page.route(/\/api\/v1\/items\/[^/]+\/play$/, async (route) => {
    const body = (route.request().postDataJSON() ?? {}) as {
      capabilities?: { video: string[]; audio: string[]; hls: string };
    };
    if (body.capabilities) body.capabilities.video = ['h264'];
    await route.continue({ postData: JSON.stringify(body) });
  });
}

for (const { title, mode, h264Only } of [
  { title: HEVC_LONG_TITLE, mode: 'transcode', h264Only: true },
  { title: H264_MKV_LONG_TITLE, mode: 'remux', h264Only: false },
]) {
  test(`${title}: HLS ${mode} starts and seeks to 20/50/80 % within budget`, async ({
    page,
    browserName,
  }) => {
    test.setTimeout(90_000);
    const detail = await itemPath(page, title);
    const itemId = detail.split('/').pop() ?? '';
    await resetProgress(page, itemId);
    if (h264Only) await reportH264Only(page);
    await page.goto(detail);
    await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible();

    const started = page.waitForResponse((r) => isPlay(r.url(), r.request().method()));
    await page.keyboard.press('ArrowDown');
    await page.keyboard.press('Enter');
    const info = (await (await started).json()) as PlaybackInfo;
    expect(info.mode).toBe('hls');
    expect(info.transcoding?.mode).toBe(mode);

    const player = page.getByRole('main', { name: 'Videoplayer' });
    await expect(player).toHaveAttribute('data-engine', 'hlsjs');
    await expect(player).toHaveAttribute('data-start-ms', /\d+/, { timeout: 15_000 });
    const startMs = Number(await player.getAttribute('data-start-ms'));
    expect(startMs).toBeLessThan(START_BUDGET_MS);
    await expectFocusOn(page, page.getByRole('button', { name: 'Pause' }));

    const seeks: Record<string, number> = {};
    for (const point of SEEK_POINTS) {
      const before = Number((await player.getAttribute('data-seek-count')) ?? 0);
      const target = await page.locator('video').evaluate((video: HTMLVideoElement, at: number) => {
        video.currentTime = video.duration * at;
        return video.currentTime;
      }, point);
      await expect(player).toHaveAttribute('data-seek-count', String(before + 1), {
        timeout: 15_000,
      });
      const seekMs = Number(await player.getAttribute('data-seek-ms'));
      seeks[`${point * 100}%`] = seekMs;
      // Playing again at the target, not at the start of the buffer.
      await expect.poll(async () => (await videoState(page)).paused).toBe(false);
      const state = await videoState(page);
      expect(state.time).toBeGreaterThanOrEqual(target - 1);
      expect(seekMs).toBeLessThan(SEEK_BUDGET_MS);
    }
    // Measurement output for docs/perf/M1b.md.
    console.log(
      JSON.stringify({
        measurement: 'hls',
        browser: browserName,
        title,
        mode,
        backend: info.transcoding?.backend ?? null,
        startMs,
        seekMs: seeks,
      }),
    );
  });
}
