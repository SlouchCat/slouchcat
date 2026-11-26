import { expect, test } from './fixtures';
import { expectFocusOn } from './keyboard';
import { HDR_TITLE, itemIdByTitle, player, resetProgress, startOf, videoState } from './playback';

// Plan 10.3 row 3: the generated HEVC HDR10 4K clip plays in Chromium and Firefox as a transcode
// with tonemapping, at most 1080p (AN-17), and starts within 5 s on a GPU (W-3).

const START_BUDGET_MS = 5000;
// Without a GPU (the CI runners) the tonemapping runs in software: the plan's documented exception
// from W-3 (section 3.5, AN-17, target below 10 s). The runners measured 5036 to 5321 ms there and
// failed the 5 s now and then; 5.5 s is the client's decision of 26 November 2026.
const SOFTWARE_START_BUDGET_MS = 5500;

test('HDR10 4K: transcode with tonemapping, at most 1080p, starts within 5 s', async ({
  page,
  owner,
  browserName,
}) => {
  test.setTimeout(90_000);
  const itemId = await itemIdByTitle(owner, HDR_TITLE);
  await resetProgress(page.request, itemId, 10_000);
  const start = await startOf(page, () => page.goto(`/play/${itemId}`));
  expect(start.status).toBe(200);
  // The embedded browsers have no HDR display.
  expect(start.request.capabilities.hdr).toBe(false);
  const { body } = start;
  expect(body.mode).toBe('hls');
  expect(body.transcoding?.mode).toBe('transcode');
  expect(body.transcoding?.reason).toBe('hdr');
  expect(body.transcoding?.tonemap).toBe(true);
  expect(body.transcoding?.height).toBeDefined();
  expect(body.transcoding!.height).toBeLessThanOrEqual(1080);

  await expect(player(page)).toHaveAttribute('data-engine', 'hlsjs');
  await expect(player(page)).toHaveAttribute('data-start-ms', /\d+/, { timeout: 30_000 });
  const startMs = Number(await player(page).getAttribute('data-start-ms'));
  expect(startMs).toBeLessThan(
    body.transcoding?.hardware ? START_BUDGET_MS : SOFTWARE_START_BUDGET_MS,
  );
  await expectFocusOn(page, page.getByRole('button', { name: 'Pause' }));
  await expect.poll(async () => (await videoState(page)).time).toBeGreaterThan(0.5);

  // Stream-Info names the tonemapping.
  await page.keyboard.press('i');
  const info = page.getByRole('region', { name: 'Stream-Info' });
  await expect(info.getByText('HDR-Video', { exact: true })).toBeVisible();
  await expect(
    info.getByText('Tonemapping', { exact: true }).locator('xpath=following-sibling::*[1]'),
  ).toHaveText('Ja');
  console.log(
    JSON.stringify({
      measurement: 'hdr-start',
      browser: browserName,
      backend: body.transcoding?.backend ?? null,
      hardware: body.transcoding?.hardware ?? null,
      startMs,
    }),
  );
});
